package static_lsp_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/helpers"
	otgconfighelpers "github.com/openconfig/featureprofiles/internal/otg_helpers/otg_config_helpers"
	otgvalidationhelpers "github.com/openconfig/featureprofiles/internal/otg_helpers/otg_validation_helpers"
	"github.com/openconfig/featureprofiles/internal/otg_helpers/packetvalidationhelpers"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	ipv4PrefixLen   = 30
	ipv6PrefixLen   = 126
	lspNextHopIndex = 0
	tolerance       = 0.01 // 1% Traffic Tolerance

	// MPLSoGUE tunnel (outer) parameters.
	gueUDPPort    = 6635         // Well-known UDP port for MPLSoGUE.
	guePolicyName = "customer10" // Policy forwarding name for MPLSoGUE decap.
	outerSrcIP    = "100.64.0.1" // Outer IPv4 source of the encapsulated traffic (from 100.64.0.0/22).
	outerDstIP    = "11.1.1.1"   // Outer IPv4 destination, within the DUT decap prefix (11.0.0.0/8).
	outerUDPSrc   = 49152

	// Base Static LSP labels bound to default untagged egress next-hop.
	mplsLabelIPv4 = 99991
	mplsLabelIPv6 = 99992

	// PF-1.25.v6 push (ingress) LSP parameters: match an IPv6 FEC subnet and push a
	// single label towards the ATE IPv6 next-hop on the egress port.
	v6LSPName       = "v6-static-lsp"
	mplsPushLabelV6 = 100100
	pushFECDstIP    = "2001:db8:cafe::1"
	pushFECDstCIDR  = "2001:db8:cafe::/64"

	// Inner payload addressing.
	innerIPv4Src = "198.51.100.1"
	innerIPv4Dst = "203.0.113.1"
	innerIPv6Src = "2001:db8:2::1"
	innerIPv6Dst = "2001:db8:2::2"

	// Inner payload DSCP/TTL used to validate preservation across decap.
	innerTTL = 64

	trafficPPS     = 1000
	trafficPackets = 10000
)

// customerSubinterface represents one of the 10 customer VLAN subinterfaces defined in README.md:
// - Two VLANs with IPv4 link local address only (/29): VLAN 20, VLAN 21
// - Two VLANs with IPv4 global /30 address: VLAN 22, VLAN 23
// - Two VLANs with IPv6 address /125 only: VLAN 24, VLAN 25
// - Four VLANs with IPv4 and IPv6 address: VLAN 26 (Jumbo MTU 9080), VLAN 27, VLAN 28, VLAN 29
type customerSubinterface struct {
	vlanID      uint32
	dutIPv4     string
	ateIPv4     string
	ipv4Len     uint8
	dutIPv6     string
	ateIPv6     string
	ipv6Len     uint8
	mtu         uint32
	mplsV4Label uint32
	mplsV6Label uint32
	mplsV4Name  string
	mplsV6Name  string
}

var (
	// Ingress and Egress LAG port assignments. Modify here to change port mappings.
	ingressPorts   = []string{"port2", "port3"}
	egressLagPorts = []string{"port1", "port4"}

	// 10 Customer Subinterfaces on Egress LAG as specified in README.md PF-1.25.1.
	custSubinterfaces = []customerSubinterface{
		// Two VLANs with IPv4 link local address only, /29 address
		{vlanID: 20, dutIPv4: "169.254.0.11", ateIPv4: "169.254.0.12", ipv4Len: 29, mplsV4Label: 99920, mplsV4Name: "lsp-v4-vlan20"},
		{vlanID: 21, dutIPv4: "169.254.0.19", ateIPv4: "169.254.0.20", ipv4Len: 29, mplsV4Label: 99921, mplsV4Name: "lsp-v4-vlan21"},

		// Two VLANs with IPv4 global /30 address
		{vlanID: 22, dutIPv4: "192.0.2.29", ateIPv4: "192.0.2.30", ipv4Len: 30, mplsV4Label: 99922, mplsV4Name: "lsp-v4-vlan22"},
		{vlanID: 23, dutIPv4: "192.0.2.33", ateIPv4: "192.0.2.34", ipv4Len: 30, mplsV4Label: 99923, mplsV4Name: "lsp-v4-vlan23"},

		// Two VLANs with IPv6 address /125 only
		{vlanID: 24, dutIPv6: "2001:db8:1:1::1", ateIPv6: "2001:db8:1:1::2", ipv6Len: 125, mplsV6Label: 99924, mplsV6Name: "lsp-v6-vlan24"},
		{vlanID: 25, dutIPv6: "2001:db8:1:2::1", ateIPv6: "2001:db8:1:2::2", ipv6Len: 125, mplsV6Label: 99925, mplsV6Name: "lsp-v6-vlan25"},

		// Four VLANs with IPv4 and IPv6 address (including One VLAN with MTU 9080)
		{vlanID: 26, dutIPv4: "192.0.2.9", ateIPv4: "192.0.2.10", ipv4Len: 30, dutIPv6: "2001:db8:1:3::1", ateIPv6: "2001:db8:1:3::2", ipv6Len: 125, mtu: 9080, mplsV4Label: 99926, mplsV6Label: 99936, mplsV4Name: "lsp-v4-vlan26", mplsV6Name: "lsp-v6-vlan26"},
		{vlanID: 27, dutIPv4: "192.0.2.13", ateIPv4: "192.0.2.14", ipv4Len: 30, dutIPv6: "2001:db8:1:4::1", ateIPv6: "2001:db8:1:4::2", ipv6Len: 125, mplsV4Label: 99927, mplsV6Label: 99937, mplsV4Name: "lsp-v4-vlan27", mplsV6Name: "lsp-v6-vlan27"},
		{vlanID: 28, dutIPv4: "192.0.2.17", ateIPv4: "192.0.2.18", ipv4Len: 30, dutIPv6: "2001:db8:1:5::1", ateIPv6: "2001:db8:1:5::2", ipv6Len: 125, mplsV4Label: 99928, mplsV6Label: 99938, mplsV4Name: "lsp-v4-vlan28", mplsV6Name: "lsp-v6-vlan28"},
		{vlanID: 29, dutIPv4: "192.0.2.21", ateIPv4: "192.0.2.22", ipv4Len: 30, dutIPv6: "2001:db8:1:6::1", ateIPv6: "2001:db8:1:6::2", ipv6Len: 125, mplsV4Label: 99929, mplsV6Label: 99939, mplsV4Name: "lsp-v4-vlan29", mplsV6Name: "lsp-v6-vlan29"},
	}

	// IMIX frame size profile (64, 128, 256, 512, 1024, 1500, 9000 MTU bytes).
	sizeWeightProfile = []otgconfighelpers.SizeWeightPair{
		{Size: 64, Weight: 20},
		{Size: 128, Weight: 20},
		{Size: 256, Weight: 20},
		{Size: 512, Weight: 10},
		{Size: 1024, Weight: 10},
		{Size: 1500, Weight: 18},
		{Size: 9000, Weight: 2},
	}

	// Ingress Port 1 (corresponding to ingressPorts[0])
	ateIngress1 = attrs.Attributes{
		Name:    "ateIngress1",
		MAC:     "02:11:02:00:00:01",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8:2::2",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
		MTU:     9216,
	}

	dutIngress1 = attrs.Attributes{
		Desc:    "DUT to ATE Ingress Port 1",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8:2::1",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
		MTU:     9216,
	}

	// Ingress Port 2 (corresponding to ingressPorts[1])
	ateIngress2 = attrs.Attributes{
		Name:    "ateIngress2",
		MAC:     "02:11:03:00:00:01",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8:3::2",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
		MTU:     9216,
	}

	dutIngress2 = attrs.Attributes{
		Desc:    "DUT to ATE Ingress Port 2",
		IPv4:    "192.0.2.5",
		IPv6:    "2001:db8:3::1",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
		MTU:     9216,
	}

	// Egress LAG base interface (subinterface 0)
	dutDst = attrs.Attributes{
		Desc:    "DUT to ATE Egress LAG Base",
		IPv4:    "192.0.2.25",
		IPv6:    "2001:db8:1::1",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
		MTU:     9216,
	}

	ateDst = attrs.Attributes{
		Name:    "ateDst",
		MAC:     "02:12:01:00:00:01",
		IPv4:    "192.0.2.26",
		IPv6:    "2001:db8:1::2",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
		MTU:     9216,
	}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func configInterfaceDUT(i *oc.Interface, a *attrs.Attributes, dut *ondatra.DUTDevice) *oc.Interface {
	i.Description = ygot.String(a.Desc)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	if !deviations.OmitL2MTU(dut) {
		i.Mtu = ygot.Uint16(a.MTU + 14)
	}

	if deviations.InterfaceEnabled(dut) {
		i.Enabled = ygot.Bool(true)
	}
	s := i.GetOrCreateSubinterface(0)

	s4 := s.GetOrCreateIpv4()
	if deviations.InterfaceEnabled(dut) && !deviations.IPv4MissingEnabled(dut) {
		s4.Enabled = ygot.Bool(true)
	}
	s4.Mtu = ygot.Uint16(a.MTU)
	s4a := s4.GetOrCreateAddress(a.IPv4)
	s4a.PrefixLength = ygot.Uint8(ipv4PrefixLen)

	s6 := s.GetOrCreateIpv6()
	if deviations.InterfaceEnabled(dut) {
		s6.Enabled = ygot.Bool(true)
	}
	s6.Mtu = ygot.Uint32(uint32(a.MTU))
	s6a := s6.GetOrCreateAddress(a.IPv6)
	s6a.PrefixLength = ygot.Uint8(ipv6PrefixLen)

	return i
}

// configureCustomerSubinterfacesDUT configures the 10 customer VLAN subinterfaces on the DUT Egress LAG.
func configureCustomerSubinterfacesDUT(dut *ondatra.DUTDevice, agg *oc.Interface) {
	for _, cs := range custSubinterfaces {
		s := agg.GetOrCreateSubinterface(cs.vlanID)
		if deviations.InterfaceEnabled(dut) {
			s.Enabled = ygot.Bool(true)
		}
		if deviations.DeprecatedVlanID(dut) {
			s.GetOrCreateVlan().VlanId = oc.UnionUint16(uint16(cs.vlanID))
		} else {
			s.GetOrCreateVlan().GetOrCreateMatch().GetOrCreateSingleTagged().VlanId = ygot.Uint16(uint16(cs.vlanID))
		}

		subMtu := uint32(9216)
		if cs.mtu != 0 {
			subMtu = cs.mtu
		}

		// IPv4 on subinterface (always enabled and MTU set on routed subinterface for EOS)
		s4 := s.GetOrCreateIpv4()
		if deviations.InterfaceEnabled(dut) && !deviations.IPv4MissingEnabled(dut) {
			s4.Enabled = ygot.Bool(true)
		}
		s4.Mtu = ygot.Uint16(uint16(subMtu))
		if cs.dutIPv4 != "" {
			s4a := s4.GetOrCreateAddress(cs.dutIPv4)
			s4a.PrefixLength = ygot.Uint8(cs.ipv4Len)
		}

		// IPv6 on subinterface (always set matching MTU to satisfy equal MTU requirement on EOS)
		s6 := s.GetOrCreateIpv6()
		if deviations.InterfaceEnabled(dut) {
			s6.Enabled = ygot.Bool(true)
		}
		s6.Mtu = ygot.Uint32(subMtu)
		if cs.dutIPv6 != "" {
			s6a := s6.GetOrCreateAddress(cs.dutIPv6)
			s6a.PrefixLength = ygot.Uint8(cs.ipv6Len)
		}
	}
}

// configureHardwareInit applies the TCAM profile
func configureHardwareInit(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	hardwareInitCfg := cfgplugins.NewDUTHardwareInit(t, dut, cfgplugins.FeaturePolicyForwarding)
	cfgplugins.PushDUTHardwareInitConfig(t, dut, hardwareInitCfg)
}

// configureDUT configures Ingress Ports and Egress LAG (with 10 customer VLAN subinterfaces) on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) string {
	d := gnmi.OC()

	configureHardwareInit(t, dut)

	// Ingress ports
	pIngress1 := dut.Port(t, ingressPorts[0])
	i1 := &oc.Interface{Name: ygot.String(pIngress1.Name())}
	gnmi.Replace(t, dut, d.Interface(pIngress1.Name()).Config(), configInterfaceDUT(i1, &dutIngress1, dut))

	pIngress2 := dut.Port(t, ingressPorts[1])
	i2 := &oc.Interface{Name: ygot.String(pIngress2.Name())}
	gnmi.Replace(t, dut, d.Interface(pIngress2.Name()).Config(), configInterfaceDUT(i2, &dutIngress2, dut))

	// Egress LAG
	var aggPorts []*ondatra.Port
	for _, portName := range egressLagPorts {
		aggPorts = append(aggPorts, dut.Port(t, portName))
	}
	aggID := netutil.NextAggregateInterface(t, dut)

	if deviations.AggregateAtomicUpdate(dut) {
		cfgplugins.DeleteAggregate(t, dut, aggID, aggPorts)
		cfgplugins.SetupAggregateAtomically(t, dut, aggID, aggPorts)
	}

	lacp := &oc.Lacp_Interface{Name: ygot.String(aggID)}
	lacp.LacpMode = oc.Lacp_LacpActivityType_ACTIVE
	lacp.SetInterval(oc.Lacp_LacpPeriodType_FAST)
	gnmi.Replace(t, dut, d.Lacp().Interface(aggID).Config(), lacp)

	agg := &oc.Interface{Name: ygot.String(aggID)}
	configInterfaceDUT(agg, &dutDst, dut)
	configureCustomerSubinterfacesDUT(dut, agg)
	agg.Type = oc.IETFInterfaces_InterfaceType_ieee8023adLag
	agg.GetOrCreateAggregation().LagType = oc.IfAggregate_AggregationType_LACP
	gnmi.Replace(t, dut, d.Interface(aggID).Config(), agg)

	if !deviations.AggregateAtomicUpdate(dut) {
		for _, port := range aggPorts {
			i := &oc.Interface{Name: ygot.String(port.Name())}
			i.Description = ygot.String("Egress LAG member " + port.Name())
			i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
			// Members inherit the aggregate MTU. IOS XR rejects an explicit
			// member MTU in the same configuration as bundle membership.
			if deviations.InterfaceEnabled(dut) {
				i.Enabled = ygot.Bool(true)
			}
			i.GetOrCreateEthernet().AggregateId = ygot.String(aggID)
			gnmi.Replace(t, dut, d.Interface(port.Name()).Config(), i)
		}
	}

	t.Cleanup(func() {
		for _, portName := range ingressPorts {
			gnmi.Delete(t, dut, d.Interface(dut.Port(t, portName).Name()).Subinterface(0).Config())
		}
		for _, cs := range custSubinterfaces {
			gnmi.Delete(t, dut, d.Interface(aggID).Subinterface(cs.vlanID).Config())
		}
		cfgplugins.DeleteAggregate(t, dut, aggID, aggPorts)
		gnmi.Delete(t, dut, d.Interface(aggID).Config())
		gnmi.Delete(t, dut, d.Lacp().Interface(aggID).Config())
	})

	// Enable MPLS forwarding, MPLSoGUE decapsulation, and the static LSP label bindings.
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	cfgplugins.MplsConfig(t, dut)
	configureGueDecap(t, dut)
	configureStaticLSPs(t, dut)

	return aggID
}

// configureGueDecap configures MPLSoGUE decapsulation on the DUT.
func configureGueDecap(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	if dut.Vendor() == ondatra.CISCO && deviations.InterfacePolicyForwardingOCUnsupported(dut) {
		// IOS XR terminates MPLS-over-UDP on an NVE local tunnel endpoint;
		// apply-forwarding-policy is not supported on this platform.
		configureCiscoMPLSUDPDecap(t, dut)
		return
	}
	_, _, pf := cfgplugins.SetupPolicyForwardingInfraOC(deviations.DefaultNetworkInstance(dut))
	ocPFParams := cfgplugins.OcPolicyForwardingParams{
		NetworkInstanceName: deviations.DefaultNetworkInstance(dut),
		AppliedPolicyName:   guePolicyName,
		InnerDstIPv4:        outerDstIP + "/32",
		DecapPolicy: cfgplugins.DecapPolicyParams{
			IPv4DestAddress: outerDstIP + "/32",
		},
	}
	if deviations.GueGreDecapUnsupported(dut) {
		cfgplugins.DecapGroupConfigGue(t, dut, pf, ocPFParams)
		t.Cleanup(func() { cfgplugins.RemoveDecapGroupGue(t, dut, ocPFParams) })
		return
	}
	// The packet is IPv4/UDP/MPLS, so match UDP and request MPLS-in-UDP
	// decapsulation rather than the IP-in-IP match used by the generic helper.
	rule := pf.GetOrCreatePolicy(guePolicyName).GetOrCreateRule(10)
	rule.GetOrCreateIpv4().DestinationAddress = ygot.String(outerDstIP + "/32")
	rule.GetOrCreateIpv4().Protocol = oc.UnionUint8(17)
	rule.GetOrCreateTransport().DestinationPort = oc.UnionUint16(gueUDPPort)
	rule.GetOrCreateAction().DecapsulateMplsInUdp = ygot.Bool(true)
	for _, pName := range ingressPorts {
		p := dut.Port(t, pName)
		cfgplugins.ApplyPolicyToInterfaceOC(t, pf, p.Name(), guePolicyName)
	}
	pfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding()
	gnmi.Update(t, dut, pfPath.Config(), pf)
	t.Cleanup(func() {
		for _, pName := range ingressPorts {
			gnmi.Delete(t, dut, pfPath.Interface(dut.Port(t, pName).Name()).Config())
		}
		gnmi.Delete(t, dut, pfPath.Policy(guePolicyName).Config())
	})
}

// configureCiscoMPLSUDPDecap creates an NVE endpoint for the outer destination.
func configureCiscoMPLSUDPDecap(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	const nveName = "nve1"
	loopback := netutil.LoopbackInterface(t, dut, 125)
	for _, name := range []string{loopback, nveName} {
		if gnmi.Lookup(t, dut, gnmi.OC().Interface(name).Config()).IsPresent() {
			t.Fatalf("MPLS-over-UDP test requires unused interface %s", name)
		}
	}
	i := &oc.Interface{
		Name:    ygot.String(loopback),
		Type:    oc.IETFInterfaces_InterfaceType_softwareLoopback,
		Enabled: ygot.Bool(true),
	}
	i.GetOrCreateSubinterface(0).GetOrCreateIpv4().GetOrCreateAddress(outerDstIP).PrefixLength = ygot.Uint8(32)
	gnmi.Replace(t, dut, gnmi.OC().Interface(loopback).Config(), i)
	t.Cleanup(func() { gnmi.Delete(t, dut, gnmi.OC().Interface(loopback).Config()) })
	// Cleanup runs in reverse order so the NVE is removed before its source.
	t.Cleanup(func() { helpers.GnmiCLIConfig(t, dut, "no interface "+nveName+"\n") })
	helpers.GnmiCLIConfig(t, dut, fmt.Sprintf("interface %s\n overlay-encapsulation mpls-udp\n source-interface %s\n!\n", nveName, loopback))
}

// clearNeighbors flushes ARP entries and IPv6 neighbors on the DUT.
func clearNeighbors(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	var commands []string
	switch dut.Vendor() {
	case ondatra.ARISTA:
		commands = []string{"clear arp-cache", "clear ipv6 neighbors"}
	case ondatra.CISCO:
		commands = []string{"clear arp-cache", "clear ipv6 neighbors"}
	case ondatra.JUNIPER:
		commands = []string{"clear arp", "clear ipv6 neighbors"}
	default:
		t.Fatalf("Neighbor clearing is not implemented for %s", dut.Vendor())
	}
	cli := dut.RawAPIs().CLI(t)
	for _, command := range commands {
		result, err := cli.RunCommand(context.Background(), command)
		if err != nil {
			t.Fatalf("%s: %v", command, err)
		}
		if result.Error() != "" || strings.Contains(result.Output(), "% Invalid") {
			t.Fatalf("%s failed: %s %s", command, result.Error(), result.Output())
		}
	}
}

// configureIPv6PushLSP binds the IPv6 FEC to the label-imposing next-hop.
func configureIPv6PushLSP(t *testing.T, dut *ondatra.DUTDevice, ingressName string) {
	t.Helper()
	if dut.Vendor() != ondatra.CISCO {
		cfgplugins.NewStaticMplsLspPushLabel(t, dut, v6LSPName, ingressName, ateDst.IPv6, pushFECDstCIDR, mplsPushLabelV6, lspNextHopIndex, "ipv6")
		t.Cleanup(func() { cfgplugins.RemoveStaticMplsLspPushLabel(t, dut, v6LSPName, ingressName) })
		return
	}
	// The OC ingress next-hop alone does not associate an IP FEC with an
	// LSP on IOS XR. Bind the prefix to a native static LSP and its route.
	const localLabel = 100101
	helpers.GnmiCLIConfig(t, dut, fmt.Sprintf("router static\n address-family ipv6 unicast\n  %s %s\n !\n!\nmpls static\n lsp %s\n  in-label %d allocate per-prefix %s\n  forward\n   path 1 resolve-nexthop %s out-label %d\n  !\n !\n!\n", pushFECDstCIDR, ateDst.IPv6, v6LSPName, localLabel, pushFECDstCIDR, ateDst.IPv6, mplsPushLabelV6))
	t.Cleanup(func() {
		helpers.GnmiCLIConfig(t, dut, fmt.Sprintf("mpls static\n no lsp %s\n!\nrouter static\n address-family ipv6 unicast\n  no %s %s\n !\n!\n", v6LSPName, pushFECDstCIDR, ateDst.IPv6))
	})
}

// verifyIPv6NeighborTelemetry validates that the DUT has resolved the IPv6 neighbor LinkLayerAddress.
func verifyIPv6NeighborTelemetry(t *testing.T, dut *ondatra.DUTDevice, aggID string, expectedMAC string) {
	t.Helper()
	llAddr, ok := gnmi.Await(t, dut, gnmi.OC().Interface(aggID).Subinterface(0).Ipv6().Neighbor(ateDst.IPv6).LinkLayerAddress().State(), time.Minute, expectedMAC).Val()
	if !ok || llAddr == "" {
		t.Errorf("IPv6 neighbor link-layer-address is empty for next-hop %s", ateDst.IPv6)
	}
}

// verifyLagLoadBalancing verifies that packets are distributed across all member links of the Egress LAG.
func verifyLagLoadBalancing(t *testing.T, ate *ondatra.ATEDevice) {
	t.Helper()
	var total uint64
	counts := make([]uint64, len(egressLagPorts))

	for i, portName := range egressLagPorts {
		p := ate.Port(t, portName)
		counts[i] = gnmi.Get(t, ate.OTG(), gnmi.OTG().Port(p.ID()).Counters().InFrames().State())
		total += counts[i]
		if counts[i] == 0 {
			t.Errorf("Egress LAG member port %s did not receive any packets", p.ID())
		}
	}
	if total == 0 {
		t.Fatal("No packets were received on the Egress LAG")
	}

	expected := float64(total) / float64(len(egressLagPorts))
	const tolerancePct = 0.10
	for i, portName := range egressLagPorts {
		p := ate.Port(t, portName)
		low := expected * (1 - tolerancePct)
		high := expected * (1 + tolerancePct)
		if float64(counts[i]) < low || float64(counts[i]) > high {
			t.Errorf("Egress LAG member port %s received %d packets; expected range [%.0f, %.0f], total=%d", p.ID(), counts[i], low, high, total)
		}
		t.Logf("Egress LAG member port %s received %d packets; expected range [%.0f, %.0f]", p.ID(), counts[i], low, high)
	}
}

// removeSubinterfaceAddress deletes the IPv4 or IPv6 address of the given family from a DUT subinterface.
func removeSubinterfaceAddress(t *testing.T, dut *ondatra.DUTDevice, intfName string, index uint32, ip string, family string) {
	t.Helper()
	sub := gnmi.OC().Interface(intfName).Subinterface(index)
	switch family {
	case "IPv4":
		gnmi.Delete(t, dut, sub.Ipv4().Address(ip).Config())
	case "IPv6":
		gnmi.Delete(t, dut, sub.Ipv6().Address(ip).Config())
	}
}

// addSubinterfaceAddress re-adds the IPv4 or IPv6 address of the given family to a DUT subinterface.
func addSubinterfaceAddress(t *testing.T, dut *ondatra.DUTDevice, intfName string, index uint32, ip string, prefixLen uint8, family string) {
	t.Helper()
	sub := gnmi.OC().Interface(intfName).Subinterface(index)
	switch family {
	case "IPv4":
		addr := &oc.Interface_Subinterface_Ipv4_Address{Ip: ygot.String(ip), PrefixLength: ygot.Uint8(prefixLen)}
		gnmi.Update(t, dut, sub.Ipv4().Address(ip).Config(), addr)
	case "IPv6":
		addr := &oc.Interface_Subinterface_Ipv6_Address{Ip: ygot.String(ip), PrefixLength: ygot.Uint8(prefixLen)}
		gnmi.Update(t, dut, sub.Ipv6().Address(ip).Config(), addr)
	}
}

// configureOTG configures Ingress Ports and Egress LAG (with all 10 customer VLAN subinterfaces) on the ATE.
func configureOTG(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	t.Helper()
	top := gosnappi.NewConfig()

	// Ingress Port 1
	pIngress1 := ate.Port(t, ingressPorts[0])
	top.Ports().Add().SetName(pIngress1.ID())
	src1Dev := top.Devices().Add().SetName(ateIngress1.Name)
	src1Eth := src1Dev.Ethernets().Add().SetName(ateIngress1.Name + ".Eth").SetMac(ateIngress1.MAC).SetMtu(uint32(ateIngress1.MTU))
	src1Eth.Connection().SetPortName(pIngress1.ID())
	src1Eth.Ipv4Addresses().Add().SetName(ateIngress1.Name + ".IPv4").SetAddress(ateIngress1.IPv4).SetGateway(dutIngress1.IPv4).SetPrefix(uint32(ateIngress1.IPv4Len))
	src1Eth.Ipv6Addresses().Add().SetName(ateIngress1.Name + ".IPv6").SetAddress(ateIngress1.IPv6).SetGateway(dutIngress1.IPv6).SetPrefix(uint32(ateIngress1.IPv6Len))

	// Ingress Port 2
	pIngress2 := ate.Port(t, ingressPorts[1])
	top.Ports().Add().SetName(pIngress2.ID())
	src2Dev := top.Devices().Add().SetName(ateIngress2.Name)
	src2Eth := src2Dev.Ethernets().Add().SetName(ateIngress2.Name + ".Eth").SetMac(ateIngress2.MAC).SetMtu(uint32(ateIngress2.MTU))
	src2Eth.Connection().SetPortName(pIngress2.ID())
	src2Eth.Ipv4Addresses().Add().SetName(ateIngress2.Name + ".IPv4").SetAddress(ateIngress2.IPv4).SetGateway(dutIngress2.IPv4).SetPrefix(uint32(ateIngress2.IPv4Len))
	src2Eth.Ipv6Addresses().Add().SetName(ateIngress2.Name + ".IPv6").SetAddress(ateIngress2.IPv6).SetGateway(dutIngress2.IPv6).SetPrefix(uint32(ateIngress2.IPv6Len))

	// Egress LAG Configuration
	lag := top.Lags().Add().SetName("ateDstLag")
	lag.Protocol().Lacp().SetActorKey(1).SetActorSystemPriority(1).SetActorSystemId(ateDst.MAC)
	for i, portName := range egressLagPorts {
		p := ate.Port(t, portName)
		top.Ports().Add().SetName(p.ID())
		lagPort := lag.Ports().Add().SetPortName(p.ID())
		lagPort.Ethernet().SetMac(ateDst.MAC).SetName("ateDstLag-" + p.ID()).SetMtu(uint32(ateDst.MTU))
		lagPort.Lacp().SetActorActivity("active").SetActorPortNumber(uint32(i + 1)).SetActorPortPriority(1).SetLacpduTimeout(0)
	}

	// Base untagged egress destination (subinterface 0)
	dstDev := top.Devices().Add().SetName(ateDst.Name)
	dstEth := dstDev.Ethernets().Add().SetName(ateDst.Name + ".Eth").SetMac(ateDst.MAC).SetMtu(uint32(ateDst.MTU))
	dstEth.Connection().SetLagName(lag.Name())
	dstEth.Ipv4Addresses().Add().SetName(ateDst.Name + ".IPv4").SetAddress(ateDst.IPv4).SetGateway(dutDst.IPv4).SetPrefix(uint32(ateDst.IPv4Len))
	dstEth.Ipv6Addresses().Add().SetName(ateDst.Name + ".IPv6").SetAddress(ateDst.IPv6).SetGateway(dutDst.IPv6).SetPrefix(uint32(ateDst.IPv6Len))

	// 10 Customer VLAN subinterfaces on Egress LAG
	for _, cs := range custSubinterfaces {
		cDev := top.Devices().Add().SetName(fmt.Sprintf("custDev-vlan%d", cs.vlanID))
		cEth := cDev.Ethernets().Add().SetName(fmt.Sprintf("custEth-vlan%d", cs.vlanID)).SetMac(ateDst.MAC)
		mtu := uint32(ateDst.MTU)
		if cs.mtu != 0 {
			mtu = cs.mtu
		}
		cEth.SetMtu(mtu)
		cEth.Connection().SetLagName(lag.Name())
		cEth.Vlans().Add().SetName(fmt.Sprintf("custVlan-%d", cs.vlanID)).SetId(cs.vlanID)

		if cs.ateIPv4 != "" {
			cEth.Ipv4Addresses().Add().SetName(fmt.Sprintf("cust-vlan%d.IPv4", cs.vlanID)).SetAddress(cs.ateIPv4).SetGateway(cs.dutIPv4).SetPrefix(uint32(cs.ipv4Len))
		}
		if cs.ateIPv6 != "" {
			cEth.Ipv6Addresses().Add().SetName(fmt.Sprintf("cust-vlan%d.IPv6", cs.vlanID)).SetAddress(cs.ateIPv6).SetGateway(cs.dutIPv6).SetPrefix(uint32(cs.ipv6Len))
		}
	}

	return top
}

// staticLSP describes a static MPLS label binding to an egress next-hop.
type staticLSP struct {
	name         string
	label        uint32
	nextHopIP    string
	protocolType string
}

// staticLSPList returns all static MPLS label bindings across base and customer VLAN interfaces.
func staticLSPList() []staticLSP {
	lsps := []staticLSP{
		{name: "lsp-base-v4", label: mplsLabelIPv4, nextHopIP: ateDst.IPv4, protocolType: "ipv4"},
		{name: "lsp-base-v6", label: mplsLabelIPv6, nextHopIP: ateDst.IPv6, protocolType: "ipv6"},
	}
	for _, cs := range custSubinterfaces {
		if cs.mplsV4Label != 0 && cs.ateIPv4 != "" {
			lsps = append(lsps, staticLSP{name: cs.mplsV4Name, label: cs.mplsV4Label, nextHopIP: cs.ateIPv4, protocolType: "ipv4"})
		}
		if cs.mplsV6Label != 0 && cs.ateIPv6 != "" {
			lsps = append(lsps, staticLSP{name: cs.mplsV6Name, label: cs.mplsV6Label, nextHopIP: cs.ateIPv6, protocolType: "ipv6"})
		}
	}
	return lsps
}

// setStaticLSP configures a single static MPLS label binding via cfgplugins.
func setStaticLSP(t *testing.T, dut *ondatra.DUTDevice, l staticLSP) {
	t.Helper()
	batch := &gnmi.SetBatch{}
	cfgplugins.MPLSStaticLSPByPass(t, batch, dut, l.name, l.label, l.nextHopIP, l.protocolType, true)
	batch.Set(t, dut)
}

// deleteStaticLSP removes a single static MPLS label binding via cfgplugins.
func deleteStaticLSP(t *testing.T, dut *ondatra.DUTDevice, l staticLSP) {
	t.Helper()
	batch := &gnmi.SetBatch{}
	cfgplugins.RemoveMPLSStaticLSP(t, batch, dut, l.name, l.label, l.nextHopIP, l.protocolType, true)
	batch.Set(t, dut)
}

// configureStaticLSPs configures all static MPLS label bindings and registers cleanup.
func configureStaticLSPs(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	lsps := staticLSPList()
	if deviations.StaticMplsLspOCUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			var cfg strings.Builder
			cfg.WriteString("mpls ip\n")
			for _, l := range lsps {
				fmt.Fprintf(&cfg, "mpls static top-label %d %s pop payload-type %s access-list bypass\n", l.label, l.nextHopIP, l.protocolType)
			}
			helpers.GnmiCLIConfig(t, dut, cfg.String())
			t.Cleanup(func() {
				var del strings.Builder
				for _, l := range lsps {
					fmt.Fprintf(&del, "no mpls static top-label %d %s pop payload-type %s access-list bypass\n", l.label, l.nextHopIP, l.protocolType)
				}
				helpers.GnmiCLIConfig(t, dut, del.String())
			})
		default:
			t.Errorf("Deviation StaticMplsLspOCUnsupported is not handled for the dut: %v", dut.Vendor())
		}
		return
	}
	for _, l := range lsps {
		setStaticLSP(t, dut, l)
		t.Cleanup(func() { deleteStaticLSP(t, dut, l) })
	}
}

// innerIPv4Flow returns inner IPv4 payload with random/varying source/dest IPs, DSCP range 0-56,
// and TCP/UDP source and destination port variation.
func innerIPv4Flow() *otgconfighelpers.Flow {
	return &otgconfighelpers.Flow{
		IPv4Flow: &otgconfighelpers.IPv4FlowParams{
			IPv4Src:      innerIPv4Src,
			IPv4SrcCount: 1000,
			IPv4Dst:      innerIPv4Dst,
			IPv4DstCount: 1000,
			TTL:          innerTTL,
			DSCP:         0,
			DSCPCount:    57,
		},
		TCPFlow: &otgconfighelpers.TCPFlowParams{
			TCPSrcPort:  49152,
			TCPSrcCount: 1000,
			TCPDstPort:  80,
			TCPDstCount: 1000,
		},
	}
}

// innerIPv6Flow returns inner IPv6 payload with random/varying source/dest IPs, DSCP/TC range,
// and TCP/UDP source and destination port variation.
func innerIPv6Flow() *otgconfighelpers.Flow {
	return &otgconfighelpers.Flow{
		IPv6Flow: &otgconfighelpers.IPv6FlowParams{
			IPv6Src:           innerIPv6Src,
			IPv6SrcCount:      1000,
			IPv6Dst:           innerIPv6Dst,
			IPv6DstCount:      1000,
			HopLimit:          innerTTL,
			TrafficClass:      0,
			TrafficClassCount: 255,
		},
		TCPFlow: &otgconfighelpers.TCPFlowParams{
			TCPSrcPort:  49152,
			TCPSrcCount: 1000,
			TCPDstPort:  80,
			TCPDstCount: 1000,
		},
	}
}

// allIPv4TOSValuesForDSCPRange returns the IPv4 ToS byte values that correspond to
// the DSCP range [start, end]
func allIPv4TOSValuesForDSCPRange(start, end uint32) []uint8 {
	vals := make([]uint8, 0, end-start+1)
	for dscp := start; dscp <= end; dscp++ {
		vals = append(vals, uint8(dscp<<2))
	}
	return vals
}

// innerIPv4PreserveSweepFlow sends one flow that cycles through DSCP values 0..56.
// This validates the full range without 57 independent OTG runs/captures.
func innerIPv4PreserveSweepFlow() *otgconfighelpers.Flow {
	return &otgconfighelpers.Flow{
		IPv4Flow: &otgconfighelpers.IPv4FlowParams{
			IPv4Src:   innerIPv4Src,
			IPv4Dst:   innerIPv4Dst,
			TTL:       innerTTL,
			DSCP:      0,
			DSCPCount: 57,
		},
		TCPFlow: &otgconfighelpers.TCPFlowParams{
			TCPSrcPort:  49152,
			TCPSrcCount: 1000,
			TCPDstPort:  80,
			TCPDstCount: 1000,
		},
	}
}

// createMPLSoGUEFlow builds an MPLSoGUE-encapsulated flow from an Ingress ATE port towards the DUT.
func createMPLSoGUEFlow(top gosnappi.Config, name string, txName string, txMAC string, label uint32, inner *otgconfighelpers.Flow, rxName string) {
	f := &otgconfighelpers.Flow{
		FlowName:          name,
		TxNames:           []string{txName},
		RxNames:           []string{rxName},
		SizeWeightProfile: &sizeWeightProfile,
		PpsRate:           trafficPPS,
		PacketsToSend:     trafficPackets,
		EthFlow:           &otgconfighelpers.EthFlowParams{SrcMAC: txMAC},
		IPv4Flow:          &otgconfighelpers.IPv4FlowParams{IPv4Src: outerSrcIP, IPv4SrcCount: 1000, IPv4Dst: outerDstIP},
		UDPFlow:           &otgconfighelpers.UDPFlowParams{UDPSrcPort: outerUDPSrc, UDPSrcCount: 1000, UDPDstPort: gueUDPPort},
		MPLSFlow:          &otgconfighelpers.MPLSFlowParams{MPLSLabel: label, MPLSExp: 7},
	}
	f.CreateFlow(top)
	f.AddEthHeader()
	f.AddIPv4Header()
	f.AddUDPHeader()
	f.AddMPLSHeader()
	if inner.IPv4Flow != nil {
		f.IPv4Flow = inner.IPv4Flow
		f.AddIPv4Header()
	}
	if inner.IPv6Flow != nil {
		f.IPv6Flow = inner.IPv6Flow
		f.AddIPv6Header()
	}
	// The outer GUE encapsulation uses UDP, while the inner payload TCP
	if inner.TCPFlow != nil {
		f.TCPFlow = inner.TCPFlow
		f.AddTCPHeader()
	}
}

// pushAndResolve pushes the OTG config, starts protocols, and waits for neighbor resolution.
func pushAndResolve(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config) {
	t.Helper()
	otgObj := ate.OTG()
	otgObj.PushConfig(t, top)
	otgObj.StartProtocols(t)
	otgutils.WaitForARP(t, otgObj, top, "IPv4")
	otgutils.WaitForARP(t, otgObj, top, "IPv6")
}

// runFlows starts traffic, waits for all packets to be transmitted, and stops traffic.
func runFlows(t *testing.T, ate *ondatra.ATEDevice, flowNames []string) {
	t.Helper()
	otgObj := ate.OTG()
	otgObj.StartTraffic(t)
	defer otgObj.StopTraffic(t)
	for _, name := range flowNames {
		gnmi.Watch(t, otgObj, gnmi.OTG().Flow(name).Counters().OutPkts().State(), time.Minute, func(v *ygnmi.Value[uint64]) bool {
			pkts, ok := v.Val()
			return ok && pkts >= uint64(trafficPackets)
		}).Await(t)
	}
}

// TestStaticLSP verifies MPLSoGUE decapsulation and static MPLS label to egress next-hop forwarding.
func TestStaticLSP(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	aggID := configureDUT(t, dut)

	var top gosnappi.Config
	t.Run("PF-1.25.1: Generate config for MPLS in GRE decap and push to DUT", func(t *testing.T) {
		top = configureOTG(t, ate)
	})

	v4Val := &otgvalidationhelpers.OTGValidation{Flow: &otgvalidationhelpers.FlowParams{Name: "mplsogue-ipv4", TolerancePct: tolerance * 100}}
	v6Val := &otgvalidationhelpers.OTGValidation{Flow: &otgvalidationhelpers.FlowParams{Name: "mplsogue-ipv6", TolerancePct: tolerance * 100}}

	t.Run("PF-1.25.2: Verify MPLSoGUE decapsulate action for IPv4 and IPv6 payload", func(t *testing.T) {
		top.Flows().Clear()
		// Base flows to default egress LAG subinterface 0
		createMPLSoGUEFlow(top, "mplsogue-ipv4", ateIngress1.Name+".IPv4", ateIngress1.MAC, mplsLabelIPv4, innerIPv4Flow(), ateDst.Name+".IPv4")
		createMPLSoGUEFlow(top, "mplsogue-ipv6", ateIngress2.Name+".IPv4", ateIngress2.MAC, mplsLabelIPv6, innerIPv6Flow(), ateDst.Name+".IPv6")

		// Customer VLAN flows to customer subinterfaces (e.g. VLAN 20 IPv4, VLAN 24 IPv6, VLAN 26 Dual-stack)
		createMPLSoGUEFlow(top, "mplsogue-cust-v4-vlan20", ateIngress1.Name+".IPv4", ateIngress1.MAC, 99920, innerIPv4Flow(), "cust-vlan20.IPv4")
		createMPLSoGUEFlow(top, "mplsogue-cust-v6-vlan24", ateIngress2.Name+".IPv4", ateIngress2.MAC, 99924, innerIPv6Flow(), "cust-vlan24.IPv6")
		createMPLSoGUEFlow(top, "mplsogue-cust-v4-vlan26", ateIngress1.Name+".IPv4", ateIngress1.MAC, 99926, innerIPv4Flow(), "cust-vlan26.IPv4")
		createMPLSoGUEFlow(top, "mplsogue-cust-v6-vlan26", ateIngress2.Name+".IPv4", ateIngress2.MAC, 99936, innerIPv6Flow(), "cust-vlan26.IPv6")

		pushAndResolve(t, ate, top)
		flowList := []string{
			"mplsogue-ipv4",
			"mplsogue-ipv6",
			"mplsogue-cust-v4-vlan20",
			"mplsogue-cust-v6-vlan24",
			"mplsogue-cust-v4-vlan26",
			"mplsogue-cust-v6-vlan26",
		}
		runFlows(t, ate, flowList)
		otgutils.LogFlowMetrics(t, ate.OTG(), top)

		for _, flowName := range flowList {
			val := &otgvalidationhelpers.OTGValidation{Flow: &otgvalidationhelpers.FlowParams{Name: flowName, TolerancePct: tolerance * 100}}
			if err := val.ValidateLossOnFlows(t, ate); err != nil {
				t.Errorf("Flow %s decap validation failed: %v", flowName, err)
			}
		}
		verifyLagLoadBalancing(t, ate)
	})

	t.Run("PF-1.25.3: Verify decap traffic is unaffected by IPv4/IPv6 VLAN config changes", func(t *testing.T) {
		t.Run("VLAN config churn", func(t *testing.T) {
			// Test VLAN config churn on Dual-stack VLAN 26:
			// Churn IPv4 address on VLAN 26 while IPv6 decap traffic flows on VLAN 26.
			top.Flows().Clear()
			createMPLSoGUEFlow(top, "mplsogue-cust-v6-vlan26", ateIngress2.Name+".IPv4", ateIngress2.MAC, 99936, innerIPv6Flow(), "cust-vlan26.IPv6")
			createMPLSoGUEFlow(top, "mplsogue-cust-v4-vlan26", ateIngress1.Name+".IPv4", ateIngress1.MAC, 99926, innerIPv4Flow(), "cust-vlan26.IPv4")
			pushAndResolve(t, ate, top)

			cs26 := custSubinterfaces[6] // VLAN 26
			removeSubinterfaceAddress(t, dut, aggID, cs26.vlanID, cs26.dutIPv4, "IPv4")
			addSubinterfaceAddress(t, dut, aggID, cs26.vlanID, cs26.dutIPv4, cs26.ipv4Len, "IPv4")
			runFlows(t, ate, []string{"mplsogue-cust-v6-vlan26"})
			otgutils.LogFlowMetrics(t, ate.OTG(), top)
			v6Val26 := &otgvalidationhelpers.OTGValidation{Flow: &otgvalidationhelpers.FlowParams{Name: "mplsogue-cust-v6-vlan26", TolerancePct: tolerance * 100}}
			if err := v6Val26.ValidateLossOnFlows(t, ate); err != nil {
				t.Errorf("IPv6 decap flow during IPv4 VLAN config churn: %v", err)
			}

			// Churn IPv6 address on VLAN 26 while IPv4 decap traffic flows on VLAN 26.
			removeSubinterfaceAddress(t, dut, aggID, cs26.vlanID, cs26.dutIPv6, "IPv6")
			addSubinterfaceAddress(t, dut, aggID, cs26.vlanID, cs26.dutIPv6, cs26.ipv6Len, "IPv6")
			runFlows(t, ate, []string{"mplsogue-cust-v4-vlan26"})
			otgutils.LogFlowMetrics(t, ate.OTG(), top)
			v4Val26 := &otgvalidationhelpers.OTGValidation{Flow: &otgvalidationhelpers.FlowParams{Name: "mplsogue-cust-v4-vlan26", TolerancePct: tolerance * 100}}
			if err := v4Val26.ValidateLossOnFlows(t, ate); err != nil {
				t.Errorf("IPv4 decap flow during IPv6 VLAN config churn: %v", err)
			}
		})

		t.Run("Decap config churn", func(t *testing.T) {
			var v4LSP, v6LSP staticLSP
			for _, l := range staticLSPList() {
				if l.name == "lsp-v4-vlan26" {
					v4LSP = l
				}
				if l.name == "lsp-v6-vlan26" {
					v6LSP = l
				}
			}

			// Remove and re-add IPv4 static LSP decap config; verify IPv6 traffic unaffected.
			top.Flows().Clear()
			createMPLSoGUEFlow(top, "mplsogue-cust-v6-vlan26", ateIngress2.Name+".IPv4", ateIngress2.MAC, 99936, innerIPv6Flow(), "cust-vlan26.IPv6")
			pushAndResolve(t, ate, top)
			deleteStaticLSP(t, dut, v4LSP)
			setStaticLSP(t, dut, v4LSP)
			runFlows(t, ate, []string{"mplsogue-cust-v6-vlan26"})
			otgutils.LogFlowMetrics(t, ate.OTG(), top)
			v6Val26 := &otgvalidationhelpers.OTGValidation{Flow: &otgvalidationhelpers.FlowParams{Name: "mplsogue-cust-v6-vlan26", TolerancePct: tolerance * 100}}
			if err := v6Val26.ValidateLossOnFlows(t, ate); err != nil {
				t.Errorf("IPv6 decap flow during IPv4 decap config churn: %v", err)
			}

			// Remove and re-add IPv6 static LSP decap config; verify IPv4 traffic unaffected.
			top.Flows().Clear()
			createMPLSoGUEFlow(top, "mplsogue-cust-v4-vlan26", ateIngress1.Name+".IPv4", ateIngress1.MAC, 99926, innerIPv4Flow(), "cust-vlan26.IPv4")
			pushAndResolve(t, ate, top)
			deleteStaticLSP(t, dut, v6LSP)
			setStaticLSP(t, dut, v6LSP)
			runFlows(t, ate, []string{"mplsogue-cust-v4-vlan26"})
			otgutils.LogFlowMetrics(t, ate.OTG(), top)
			v4Val26 := &otgvalidationhelpers.OTGValidation{Flow: &otgvalidationhelpers.FlowParams{Name: "mplsogue-cust-v4-vlan26", TolerancePct: tolerance * 100}}
			if err := v4Val26.ValidateLossOnFlows(t, ate); err != nil {
				t.Errorf("IPv4 decap flow during IPv6 decap config churn: %v", err)
			}
		})
	})

	t.Run("PF-1.25.4: Verify MPLSoGUE DSCP/TTL preserve operation", func(t *testing.T) {
		// Preserve the inner TTL across MPLS disposition rather than copying
		// the decremented outer label TTL into the IP header.
		if !deviations.MplsOCUnsupported(dut) {
			ttlPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Mpls().Global().TtlPropagation()
			previous := gnmi.Lookup(t, dut, ttlPath.Config())
			gnmi.Update(t, dut, ttlPath.Config(), false)
			t.Cleanup(func() {
				if value, present := previous.Val(); present {
					gnmi.Update(t, dut, ttlPath.Config(), value)
				} else {
					gnmi.Delete(t, dut, ttlPath.Config())
				}
			})
		}
		packetvalidationhelpers.ClearCapture(t, top, ate)
		top.Flows().Clear()
		createMPLSoGUEFlow(top, "mplsogue-ipv4", ateIngress1.Name+".IPv4", ateIngress1.MAC, mplsLabelIPv4, innerIPv4PreserveSweepFlow(), ateDst.Name+".IPv4")
		pCapture := ate.Port(t, egressLagPorts[0])
		pv := &packetvalidationhelpers.PacketValidation{
			PortName:    pCapture.ID(),
			CaptureName: "ipv4-decap",
			Validations: []packetvalidationhelpers.ValidationType{packetvalidationhelpers.ValidateIPv4Header},
			// Decapsulated inner IPv4 must retain the DSCP/TOS set across the sweep. TOS = DSCP << 2.
			IPv4Layer: &packetvalidationhelpers.IPv4Layer{DstIP: innerIPv4Dst, AllowedTOSValues: allIPv4TOSValuesForDSCPRange(0, 56), TTL: innerTTL, SkipProtocolCheck: true},
		}
		packetvalidationhelpers.ConfigurePacketCapture(t, top, pv)
		pushAndResolve(t, ate, top)
		cs := packetvalidationhelpers.StartCapture(t, ate)
		defer packetvalidationhelpers.StopCapture(t, ate, cs)
		runFlows(t, ate, []string{"mplsogue-ipv4"})
		otgutils.LogFlowMetrics(t, ate.OTG(), top)
		if err := v4Val.ValidateLossOnFlows(t, ate); err != nil {
			t.Errorf("IPv4 decap flow: %v", err)
		}
		if err := packetvalidationhelpers.CaptureAndValidatePackets(t, ate, pv); err != nil {
			t.Errorf("DSCP/TTL preserve validation: %v", err)
		}
	})

	t.Run("PF-1.25.5: Verify IPv4/IPv6 nexthop resolution of decap traffic", func(t *testing.T) {
		packetvalidationhelpers.ClearCapture(t, top, ate)
		top.Flows().Clear()
		createMPLSoGUEFlow(top, "mplsogue-ipv4", ateIngress1.Name+".IPv4", ateIngress1.MAC, mplsLabelIPv4, innerIPv4Flow(), ateDst.Name+".IPv4")
		createMPLSoGUEFlow(top, "mplsogue-ipv6", ateIngress2.Name+".IPv4", ateIngress2.MAC, mplsLabelIPv6, innerIPv6Flow(), ateDst.Name+".IPv6")
		pushAndResolve(t, ate, top)
		nhVal := &otgvalidationhelpers.InterfaceParams{Names: []string{ateDst.Name}}
		nhValObj := &otgvalidationhelpers.OTGValidation{Interface: nhVal}
		if err := nhValObj.IsIPv4Interfaceresolved(t, ate); err != nil {
			t.Errorf("IPv4 next-hop resolution: %v", err)
		}
		if err := nhValObj.IsIPv6Interfaceresolved(t, ate); err != nil {
			t.Errorf("IPv6 next-hop resolution: %v", err)
		}

		// Clear ARP and IPv6 neighbors on DUT to verify dynamic resolution during traffic.
		clearNeighbors(t, dut)

		runFlows(t, ate, []string{"mplsogue-ipv4", "mplsogue-ipv6"})
		otgutils.LogFlowMetrics(t, ate.OTG(), top)
		if err := v4Val.ValidateLossOnFlows(t, ate); err != nil {
			t.Errorf("IPv4 decap flow: %v", err)
		}
		if err := v6Val.ValidateLossOnFlows(t, ate); err != nil {
			t.Errorf("IPv6 decap flow: %v", err)
		}
	})

	t.Run("PF-1.25.v6: Validate static MPLS LSP push using an IPv6 next-hop", func(t *testing.T) {
		pIngressPush := dut.Port(t, ingressPorts[0])
		pCapture := ate.Port(t, egressLagPorts[0])
		top.Flows().Clear()

		// Plain IPv6 flow from Ingress Port 1 destined to the FEC subnet the DUT matches for the
		// push LSP. The DUT pushes the MPLS label and forwards to the ATE Egress LAG next-hop.
		f := &otgconfighelpers.Flow{
			FlowName:      "v6-push",
			TxNames:       []string{ateIngress1.Name + ".IPv6"},
			RxNames:       []string{ateDst.Name + ".IPv6"},
			FrameSize:     512,
			PpsRate:       trafficPPS,
			PacketsToSend: trafficPackets,
			EthFlow:       &otgconfighelpers.EthFlowParams{SrcMAC: ateIngress1.MAC},
			IPv6Flow:      &otgconfighelpers.IPv6FlowParams{IPv6Src: ateIngress1.IPv6, IPv6SrcCount: 1000, IPv6Dst: pushFECDstIP},
			UDPFlow:       &otgconfighelpers.UDPFlowParams{UDPSrcPort: 10000, UDPSrcCount: 1000, UDPDstPort: 5000},
		}
		f.CreateFlow(top)
		f.AddEthHeader()
		f.AddIPv6Header()
		f.AddUDPHeader()

		pv := &packetvalidationhelpers.PacketValidation{
			PortName:    pCapture.ID(),
			CaptureName: "v6-mpls-push",
			Validations: []packetvalidationhelpers.ValidationType{packetvalidationhelpers.ValidateMPLSLayer},
			MPLSLayer:   &packetvalidationhelpers.MPLSLayer{Label: mplsPushLabelV6},
		}
		packetvalidationhelpers.ConfigurePacketCapture(t, top, pv)
		pushAndResolve(t, ate, top)

		// Validate IPv6 Neighbor Discovery (ND) resolution on DUT telemetry leaf.
		verifyIPv6NeighborTelemetry(t, dut, aggID, ateDst.MAC)

		configureIPv6PushLSP(t, dut, pIngressPush.Name())

		cs := packetvalidationhelpers.StartCapture(t, ate)
		defer packetvalidationhelpers.StopCapture(t, ate, cs)
		runFlows(t, ate, []string{"v6-push"})
		otgutils.LogFlowMetrics(t, ate.OTG(), top)

		pushVal := &otgvalidationhelpers.OTGValidation{Flow: &otgvalidationhelpers.FlowParams{Name: "v6-push", TolerancePct: tolerance * 100}}
		if err := pushVal.ValidateLossOnFlows(t, ate); err != nil {
			t.Errorf("IPv6 push flow: %v", err)
		}
		if err := packetvalidationhelpers.CaptureAndValidatePackets(t, ate, pv); err != nil {
			t.Errorf("MPLS label push validation: %v", err)
		}
	})
}
