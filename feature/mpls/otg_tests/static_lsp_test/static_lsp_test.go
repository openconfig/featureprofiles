package static_lsp_test

import (
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	otgconfighelpers "github.com/openconfig/featureprofiles/internal/otg_helpers/otg_config_helpers"
	otgvalidationhelpers "github.com/openconfig/featureprofiles/internal/otg_helpers/otg_validation_helpers"
	"github.com/openconfig/featureprofiles/internal/otg_helpers/packetvalidationhelpers"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	ipv4PrefixLen   = 30
	ipv6PrefixLen   = 126
	lspNextHopIndex = 0
	tolerance       = 0.01 // 1% Traffic Tolerance

	// MPLSoGUE tunnel (outer) parameters.
	gueUDPPort  = 6635         // Well-known UDP port for MPLSoGUE.
	outerSrcIP  = "100.64.0.1" // Outer IPv4 source of the encapsulated traffic.
	outerDstIP  = "11.1.1.1"   // Outer IPv4 destination, within the DUT decap prefix (11.0.0.0/8).
	outerUDPSrc = 49152

	// Static LSP labels bound to each egress next-hop.
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
	innerDSCP = 32
	innerTTL  = 64

	trafficPPS     = 1000
	trafficPackets = 10000

	// Egress VLAN ID used by PF-1.25.3 to move the untagged interface onto a
	// VLAN-tagged subinterface. The same interface carries both IPv4 and IPv6.
	vlanID = 10
)

var (
	ateSrc = attrs.Attributes{
		Name:    "ateSrc",
		MAC:     "02:11:01:00:00:01",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::1",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}

	dutSrc = attrs.Attributes{
		Desc:    "DUT to ATE source",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::2",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}

	dutDst = attrs.Attributes{
		Desc:    "DUT to ATE destination",
		IPv4:    "192.0.2.5",
		IPv6:    "2001:db8::5",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}

	ateDst = attrs.Attributes{
		Name:    "ateDst",
		MAC:     "02:12:01:00:00:01",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::6",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func configInterfaceDUT(i *oc.Interface, a *attrs.Attributes, dut *ondatra.DUTDevice) *oc.Interface {
	i.Description = ygot.String(a.Desc)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd

	if deviations.InterfaceEnabled(dut) {
		i.Enabled = ygot.Bool(true)
	}
	s := i.GetOrCreateSubinterface(0)

	s4 := s.GetOrCreateIpv4()
	if deviations.InterfaceEnabled(dut) && !deviations.IPv4MissingEnabled(dut) {
		s4.Enabled = ygot.Bool(true)
	}
	s4a := s4.GetOrCreateAddress(a.IPv4)
	s4a.PrefixLength = ygot.Uint8(ipv4PrefixLen)

	s6 := s.GetOrCreateIpv6()
	if deviations.InterfaceEnabled(dut) {
		s6.Enabled = ygot.Bool(true)
	}
	s6a := s6.GetOrCreateAddress(a.IPv6)
	s6a.PrefixLength = ygot.Uint8(ipv6PrefixLen)

	return i
}

// configureHardwareInit applies the TCAM profile
func configureHardwareInit(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	hardwareInitCfg := cfgplugins.NewDUTHardwareInit(t, dut, cfgplugins.FeaturePolicyForwarding)
	cfgplugins.PushDUTHardwareInitConfig(t, dut, hardwareInitCfg)
}

// configureDUT configures port1, port2 on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	d := gnmi.OC()

	configureHardwareInit(t, dut)

	p1 := dut.Port(t, "port1")
	i1 := &oc.Interface{Name: ygot.String(p1.Name())}
	i1.Enabled = ygot.Bool(true)
	gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), configInterfaceDUT(i1, &dutSrc, dut))

	p2 := dut.Port(t, "port2")
	i2 := &oc.Interface{Name: ygot.String(p2.Name())}
	gnmi.Replace(t, dut, d.Interface(p2.Name()).Config(), configInterfaceDUT(i2, &dutDst, dut))

	t.Cleanup(func() {
		gnmi.Delete(t, dut, d.Interface(p1.Name()).Subinterface(0).Config())
		gnmi.Delete(t, dut, d.Interface(p2.Name()).Subinterface(0).Config())
	})

	// Enable MPLS forwarding, MPLSoGUE decapsulation, and the static LSP label bindings.
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	cfgplugins.MplsConfig(t, dut)
	configureGueDecap(t, dut)
	configureStaticLSPs(t, dut)
}

// configureGueDecap configures MPLSoGUE decapsulation on the DUT.
func configureGueDecap(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	_, ni, pf := cfgplugins.SetupPolicyForwardingInfraOC(deviations.DefaultNetworkInstance(dut))
	ocPFParams := cfgplugins.OcPolicyForwardingParams{
		NetworkInstanceName: deviations.DefaultNetworkInstance(dut),
		InnerDstIPv4:        innerIPv4Dst + "/32",
	}
	cfgplugins.DecapGroupConfigGue(t, dut, pf, ocPFParams)
	if !deviations.GueGreDecapUnsupported(dut) {
		cfgplugins.PushPolicyForwardingConfig(t, dut, ni)
	}
	t.Cleanup(func() {
		cfgplugins.RemoveDecapGroupGue(t, dut, "customer10")
	})
}

// removeInterfaceAddress deletes the IPv4 or IPv6 address of the given family from a DUT subinterface.
func removeInterfaceAddress(t *testing.T, dut *ondatra.DUTDevice, port string, index uint32, a *attrs.Attributes, family string) {
	t.Helper()
	if a == nil {
		t.Fatal("Attributes 'a' cannot be nil")
	}
	sub := gnmi.OC().Interface(dut.Port(t, port).Name()).Subinterface(index)
	switch family {
	case "IPv4":
		gnmi.Delete(t, dut, sub.Ipv4().Address(a.IPv4).Config())
	case "IPv6":
		gnmi.Delete(t, dut, sub.Ipv6().Address(a.IPv6).Config())
	}
}

// addInterfaceAddress re-adds the IPv4 or IPv6 address of the given family to a DUT subinterface.
func addInterfaceAddress(t *testing.T, dut *ondatra.DUTDevice, port string, index uint32, a *attrs.Attributes, family string) {
	t.Helper()
	if a == nil {
		t.Fatal("Attributes 'a' cannot be nil")
	}
	sub := gnmi.OC().Interface(dut.Port(t, port).Name()).Subinterface(index)
	switch family {
	case "IPv4":
		addr := &oc.Interface_Subinterface_Ipv4_Address{Ip: ygot.String(a.IPv4), PrefixLength: ygot.Uint8(ipv4PrefixLen)}
		gnmi.Update(t, dut, sub.Ipv4().Address(a.IPv4).Config(), addr)
	case "IPv6":
		addr := &oc.Interface_Subinterface_Ipv6_Address{Ip: ygot.String(a.IPv6), PrefixLength: ygot.Uint8(ipv6PrefixLen)}
		gnmi.Update(t, dut, sub.Ipv6().Address(a.IPv6).Config(), addr)
	}
}

// addVLANSubinterface configures a VLAN-tagged subinterface on a DUT port carrying both
// IPv4 and IPv6 addresses. PF-1.25.3 uses it to move an egress interface from an untagged
// interface to a VLAN interface.
func addVLANSubinterface(t *testing.T, dut *ondatra.DUTDevice, port string, index uint32, vlanID uint16, a *attrs.Attributes) {
	t.Helper()
	if a == nil {
		t.Fatal("Attributes 'a' cannot be nil")
	}
	p := dut.Port(t, port)
	i := &oc.Interface{Name: ygot.String(p.Name())}
	if deviations.InterfaceEnabled(dut) {
		i.Enabled = ygot.Bool(true)
	}
	s := i.GetOrCreateSubinterface(index)
	s.Enabled = ygot.Bool(true)
	if deviations.DeprecatedVlanID(dut) {
		s.GetOrCreateVlan().VlanId = oc.UnionUint16(vlanID)
	} else {
		s.GetOrCreateVlan().GetOrCreateMatch().GetOrCreateSingleTagged().VlanId = ygot.Uint16(vlanID)
	}
	if a.IPv4 != "" {
		s4 := s.GetOrCreateIpv4()
		if deviations.InterfaceEnabled(dut) && !deviations.IPv4MissingEnabled(dut) {
			s4.Enabled = ygot.Bool(true)
		}
		s4.GetOrCreateAddress(a.IPv4).PrefixLength = ygot.Uint8(ipv4PrefixLen)
	}
	if a.IPv6 != "" {
		s6 := s.GetOrCreateIpv6()
		if deviations.InterfaceEnabled(dut) {
			s6.Enabled = ygot.Bool(true)
		}
		s6.GetOrCreateAddress(a.IPv6).PrefixLength = ygot.Uint8(ipv6PrefixLen)
	}
	gnmi.Update(t, dut, gnmi.OC().Interface(p.Name()).Config(), i)
}

// removeVLANSubinterface deletes a VLAN-tagged subinterface from a DUT port.
func removeVLANSubinterface(t *testing.T, dut *ondatra.DUTDevice, port string, index uint32) {
	t.Helper()
	gnmi.Delete(t, dut, gnmi.OC().Interface(dut.Port(t, port).Name()).Subinterface(index).Config())
}

// churnOTG builds the OTG config used during PF-1.25.3 interface VLAN churn. The egress
// (port2) destination is a single VLAN-tagged interface carrying both IPv4 and IPv6, so a
// single family's address can be churned on the DUT while the other keeps flowing.
func churnOTG(vlan uint32) gosnappi.Config {
	top := gosnappi.NewConfig()
	port1 := top.Ports().Add().SetName("port1")
	port2 := top.Ports().Add().SetName("port2")

	// Port1: ingress source of MPLSoGUE traffic (untagged).
	srcDev := top.Devices().Add().SetName(ateSrc.Name)
	srcEth := srcDev.Ethernets().Add().SetName(ateSrc.Name + ".Eth").SetMac(ateSrc.MAC)
	srcEth.Connection().SetPortName(port1.Name())
	srcEth.Ipv4Addresses().Add().SetName(ateSrc.Name + ".IPv4").SetAddress(ateSrc.IPv4).SetGateway(dutSrc.IPv4).SetPrefix(uint32(ateSrc.IPv4Len))
	srcEth.Ipv6Addresses().Add().SetName(ateSrc.Name + ".IPv6").SetAddress(ateSrc.IPv6).SetGateway(dutSrc.IPv6).SetPrefix(uint32(ateSrc.IPv6Len))

	// Port2: single VLAN-tagged egress interface carrying both IPv4 and IPv6.
	dstDev := top.Devices().Add().SetName(ateDst.Name)
	dstEth := dstDev.Ethernets().Add().SetName(ateDst.Name + ".Eth").SetMac(ateDst.MAC)
	dstEth.Connection().SetPortName(port2.Name())
	dstEth.Vlans().Add().SetName(ateDst.Name + ".vlan").SetId(vlan)
	dstEth.Ipv4Addresses().Add().SetName(ateDst.Name + ".IPv4").SetAddress(ateDst.IPv4).SetGateway(dutDst.IPv4).SetPrefix(uint32(ateDst.IPv4Len))
	dstEth.Ipv6Addresses().Add().SetName(ateDst.Name + ".IPv6").SetAddress(ateDst.IPv6).SetGateway(dutDst.IPv6).SetPrefix(uint32(ateDst.IPv6Len))

	return top
}

// configureATE configures port1 and port2 on the ATE.
func configureOTG(t *testing.T) gosnappi.Config {
	t.Helper()
	top := gosnappi.NewConfig()
	port1 := top.Ports().Add().SetName("port1")
	port2 := top.Ports().Add().SetName("port2")

	// Port1 Configuration.
	iDut1Dev := top.Devices().Add().SetName(ateSrc.Name)
	iDut1Eth := iDut1Dev.Ethernets().Add().SetName(ateSrc.Name + ".Eth").SetMac(ateSrc.MAC)
	iDut1Eth.Connection().SetPortName(port1.Name())
	iDut1Ipv4 := iDut1Eth.Ipv4Addresses().Add().SetName(ateSrc.Name + ".IPv4")
	iDut1Ipv4.SetAddress(ateSrc.IPv4).SetGateway(dutSrc.IPv4).SetPrefix(uint32(ateSrc.IPv4Len))
	iDut1Ipv6 := iDut1Eth.Ipv6Addresses().Add().SetName(ateSrc.Name + ".IPv6")
	iDut1Ipv6.SetAddress(ateSrc.IPv6).SetGateway(dutSrc.IPv6).SetPrefix(uint32(ateSrc.IPv6Len))

	// Port2 Configuration.
	iDut2Dev := top.Devices().Add().SetName(ateDst.Name)
	iDut2Eth := iDut2Dev.Ethernets().Add().SetName(ateDst.Name + ".Eth").SetMac(ateDst.MAC)
	iDut2Eth.Connection().SetPortName(port2.Name())
	iDut2Ipv4 := iDut2Eth.Ipv4Addresses().Add().SetName(ateDst.Name + ".IPv4")
	iDut2Ipv4.SetAddress(ateDst.IPv4).SetGateway(dutDst.IPv4).SetPrefix(uint32(ateDst.IPv4Len))
	iDut2Ipv6 := iDut2Eth.Ipv6Addresses().Add().SetName(ateDst.Name + ".IPv6")
	iDut2Ipv6.SetAddress(ateDst.IPv6).SetGateway(dutDst.IPv6).SetPrefix(uint32(ateDst.IPv6Len))

	return top

}

// staticLSP describes a static MPLS label binding to an egress next-hop.
type staticLSP struct {
	name         string
	label        uint32
	nextHopIP    string
	protocolType string
}

// staticLSPList returns the static MPLS label to egress next-hop bindings for IPv4 and IPv6.
func staticLSPList() []staticLSP {
	return []staticLSP{
		{name: "lsp-v4", label: mplsLabelIPv4, nextHopIP: ateDst.IPv4, protocolType: "ipv4"},
		{name: "lsp-v6", label: mplsLabelIPv6, nextHopIP: ateDst.IPv6, protocolType: "ipv6"},
	}
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
	for _, l := range staticLSPList() {
		l := l
		setStaticLSP(t, dut, l)
		t.Cleanup(func() { deleteStaticLSP(t, dut, l) })
	}
}

// innerIPv4Flow returns the inner IPv4 payload parameters used to validate DSCP/TTL preservation.
func innerIPv4Flow() *otgconfighelpers.Flow {
	return &otgconfighelpers.Flow{
		IPv4Flow: &otgconfighelpers.IPv4FlowParams{IPv4Src: innerIPv4Src, IPv4Dst: innerIPv4Dst, TTL: innerTTL, DSCP: innerDSCP},
	}
}

// innerIPv6Flow returns the inner IPv6 payload parameters used to validate DSCP/TTL preservation.
func innerIPv6Flow() *otgconfighelpers.Flow {
	return &otgconfighelpers.Flow{
		IPv6Flow: &otgconfighelpers.IPv6FlowParams{IPv6Src: innerIPv6Src, IPv6Dst: innerIPv6Dst, HopLimit: innerTTL, TrafficClass: innerDSCP << 2},
	}
}

// createMPLSoGUEFlow builds an MPLSoGUE-encapsulated flow from ATE port1 towards the DUT.
// The DUT is expected to decapsulate GUE, pop the MPLS label, and forward the inner payload
// to the egress next-hop bound to the label (ATE port2).
func createMPLSoGUEFlow(top gosnappi.Config, name string, label uint32, inner *otgconfighelpers.Flow, rxName string) {
	f := &otgconfighelpers.Flow{
		FlowName:      name,
		TxNames:       []string{ateSrc.Name + ".IPv4"},
		RxNames:       []string{rxName},
		FrameSize:     512,
		PpsRate:       trafficPPS,
		PacketsToSend: trafficPackets,
		EthFlow:       &otgconfighelpers.EthFlowParams{SrcMAC: ateSrc.MAC},
		IPv4Flow:      &otgconfighelpers.IPv4FlowParams{IPv4Src: outerSrcIP, IPv4Dst: outerDstIP},
		UDPFlow:       &otgconfighelpers.UDPFlowParams{UDPSrcPort: outerUDPSrc, UDPDstPort: gueUDPPort},
		MPLSFlow:      &otgconfighelpers.MPLSFlowParams{MPLSLabel: label, MPLSExp: 7},
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
	for _, name := range flowNames {
		gnmi.Watch(t, otgObj, gnmi.OTG().Flow(name).Counters().OutPkts().State(), time.Minute, func(v *ygnmi.Value[uint64]) bool {
			pkts, ok := v.Val()
			return ok && pkts >= uint64(trafficPackets)
		}).Await(t)
	}
	otgObj.StopTraffic(t)
}

// TestStaticLSP verifies MPLSoGUE decapsulation and static MPLS label to egress next-hop forwarding.
func TestStaticLSP(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	configureDUT(t, dut)

	var top gosnappi.Config
	t.Run("PF-1.25.1: Generate config for MPLS in GRE decap and push to DUT", func(subT *testing.T) {
		top = configureOTG(subT)
	})

	v4Val := &otgvalidationhelpers.OTGValidation{Flow: &otgvalidationhelpers.FlowParams{Name: "mplsogue-ipv4", TolerancePct: tolerance * 100}}
	v6Val := &otgvalidationhelpers.OTGValidation{Flow: &otgvalidationhelpers.FlowParams{Name: "mplsogue-ipv6", TolerancePct: tolerance * 100}}

	t.Run("PF-1.25.2: Verify MPLSoGUE decapsulate action for IPv4 and IPv6 payload", func(t *testing.T) {
		top.Flows().Clear()
		createMPLSoGUEFlow(top, "mplsogue-ipv4", mplsLabelIPv4, innerIPv4Flow(), ateDst.Name+".IPv4")
		createMPLSoGUEFlow(top, "mplsogue-ipv6", mplsLabelIPv6, innerIPv6Flow(), ateDst.Name+".IPv6")
		pushAndResolve(t, ate, top)
		runFlows(t, ate, []string{"mplsogue-ipv4", "mplsogue-ipv6"})
		otgutils.LogFlowMetrics(t, ate.OTG(), top)
		if err := v4Val.ValidateLossOnFlows(t, ate); err != nil {
			t.Errorf("IPv4 decap flow: %v", err)
		}
		if err := v6Val.ValidateLossOnFlows(t, ate); err != nil {
			t.Errorf("IPv6 decap flow: %v", err)
		}
	})

	t.Run("PF-1.25.3: Verify decap traffic is unaffected by IPv4/IPv6 VLAN config changes", func(t *testing.T) {
		t.Run("VLAN config churn", func(t *testing.T) {
			// Restore the base (untagged) config on the DUT and ATE even if this subtest
			// fails or panics early.
			t.Cleanup(func() {
				removeVLANSubinterface(t, dut, "port2", vlanID)
				addInterfaceAddress(t, dut, "port2", 0, &dutDst, "IPv4")
				addInterfaceAddress(t, dut, "port2", 0, &dutDst, "IPv6")
				top = configureOTG(t)
				pushAndResolve(t, ate, top)
			})

			// Move the egress interface from the untagged interface onto a single VLAN-tagged
			// subinterface carrying both IPv4 and IPv6 (mirrored on the ATE).
			removeInterfaceAddress(t, dut, "port2", 0, &dutDst, "IPv4")
			removeInterfaceAddress(t, dut, "port2", 0, &dutDst, "IPv6")
			addVLANSubinterface(t, dut, "port2", vlanID, vlanID, &dutDst)

			// Remove and re-add the IPv4 address on the VLAN subinterface. IPv6 decap traffic
			// on the same VLAN interface must be unaffected.
			top = churnOTG(vlanID)
			createMPLSoGUEFlow(top, "mplsogue-ipv6", mplsLabelIPv6, innerIPv6Flow(), ateDst.Name+".IPv6")
			pushAndResolve(t, ate, top)
			removeInterfaceAddress(t, dut, "port2", vlanID, &dutDst, "IPv4")
			addInterfaceAddress(t, dut, "port2", vlanID, &dutDst, "IPv4")
			runFlows(t, ate, []string{"mplsogue-ipv6"})
			otgutils.LogFlowMetrics(t, ate.OTG(), top)
			if err := v6Val.ValidateLossOnFlows(t, ate); err != nil {
				t.Errorf("IPv6 decap flow during IPv4 VLAN config churn: %v", err)
			}

			// Remove and re-add the IPv6 address on the VLAN subinterface. IPv4 decap traffic
			// on the same VLAN interface must be unaffected.
			top = churnOTG(vlanID)
			createMPLSoGUEFlow(top, "mplsogue-ipv4", mplsLabelIPv4, innerIPv4Flow(), ateDst.Name+".IPv4")
			pushAndResolve(t, ate, top)
			removeInterfaceAddress(t, dut, "port2", vlanID, &dutDst, "IPv6")
			addInterfaceAddress(t, dut, "port2", vlanID, &dutDst, "IPv6")
			runFlows(t, ate, []string{"mplsogue-ipv4"})
			otgutils.LogFlowMetrics(t, ate.OTG(), top)
			if err := v4Val.ValidateLossOnFlows(t, ate); err != nil {
				t.Errorf("IPv4 decap flow during IPv6 VLAN config churn: %v", err)
			}
		})

		t.Run("Decap config churn", func(t *testing.T) {
			// The per-family MPLSoGUE decap config in this 2-port setup is the per-family
			// static MPLS label binding that pops the label and forwards the inner payload.
			var v4LSP, v6LSP staticLSP
			for _, l := range staticLSPList() {
				switch l.protocolType {
				case "ipv4":
					v4LSP = l
				case "ipv6":
					v6LSP = l
				}
			}

			// Remove and re-add the IPv4 MPLSoGUE decap config. IPv6 decap traffic must be unaffected.
			createMPLSoGUEFlow(top, "mplsogue-ipv6", mplsLabelIPv6, innerIPv6Flow(), ateDst.Name+".IPv6")
			pushAndResolve(t, ate, top)
			deleteStaticLSP(t, dut, v4LSP)
			setStaticLSP(t, dut, v4LSP)
			runFlows(t, ate, []string{"mplsogue-ipv6"})
			otgutils.LogFlowMetrics(t, ate.OTG(), top)
			if err := v6Val.ValidateLossOnFlows(t, ate); err != nil {
				t.Errorf("IPv6 decap flow during IPv4 decap config churn: %v", err)
			}

			// Remove and re-add the IPv6 MPLSoGUE decap config. IPv4 decap traffic must be unaffected.
			top = configureOTG(t)
			createMPLSoGUEFlow(top, "mplsogue-ipv4", mplsLabelIPv4, innerIPv4Flow(), ateDst.Name+".IPv4")
			pushAndResolve(t, ate, top)
			deleteStaticLSP(t, dut, v6LSP)
			setStaticLSP(t, dut, v6LSP)
			runFlows(t, ate, []string{"mplsogue-ipv4"})
			otgutils.LogFlowMetrics(t, ate.OTG(), top)
			if err := v4Val.ValidateLossOnFlows(t, ate); err != nil {
				t.Errorf("IPv4 decap flow during IPv6 decap config churn: %v", err)
			}
		})
	})

	t.Run("PF-1.25.4: Verify MPLSoGUE DSCP/TTL preserve operation", func(t *testing.T) {
		packetvalidationhelpers.ClearCapture(t, top, ate)
		top.Flows().Clear()
		createMPLSoGUEFlow(top, "mplsogue-ipv4", mplsLabelIPv4, innerIPv4Flow(), ateDst.Name+".IPv4")
		pv := &packetvalidationhelpers.PacketValidation{
			PortName:    "port2",
			CaptureName: "ipv4-decap",
			Validations: []packetvalidationhelpers.ValidationType{packetvalidationhelpers.ValidateIPv4Header},
			// Decapsulated inner IPv4 must retain its DSCP (TOS) and TTL. TOS byte = DSCP << 2.
			IPv4Layer: &packetvalidationhelpers.IPv4Layer{DstIP: innerIPv4Dst, Tos: innerDSCP << 2, TTL: innerTTL, SkipProtocolCheck: true},
		}
		packetvalidationhelpers.ConfigurePacketCapture(t, top, pv)
		pushAndResolve(t, ate, top)
		cs := packetvalidationhelpers.StartCapture(t, ate)
		runFlows(t, ate, []string{"mplsogue-ipv4"})
		packetvalidationhelpers.StopCapture(t, ate, cs)
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
		createMPLSoGUEFlow(top, "mplsogue-ipv4", mplsLabelIPv4, innerIPv4Flow(), ateDst.Name+".IPv4")
		createMPLSoGUEFlow(top, "mplsogue-ipv6", mplsLabelIPv6, innerIPv6Flow(), ateDst.Name+".IPv6")
		pushAndResolve(t, ate, top)
		nhVal := &otgvalidationhelpers.OTGValidation{Interface: &otgvalidationhelpers.InterfaceParams{Names: []string{ateDst.Name}}}
		if err := nhVal.IsIPv4Interfaceresolved(t, ate); err != nil {
			t.Errorf("IPv4 next-hop resolution: %v", err)
		}
		if err := nhVal.IsIPv6Interfaceresolved(t, ate); err != nil {
			t.Errorf("IPv6 next-hop resolution: %v", err)
		}
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
		p1 := dut.Port(t, "port1")
		top.Flows().Clear()

		// Plain IPv6 flow from ATE port1 destined to the FEC subnet the DUT matches for the
		// push LSP. The DUT pushes the MPLS label and forwards to the ATE port2 next-hop.
		f := &otgconfighelpers.Flow{
			FlowName:      "v6-push",
			TxNames:       []string{ateSrc.Name + ".IPv6"},
			RxNames:       []string{ateDst.Name + ".IPv6"},
			FrameSize:     512,
			PpsRate:       trafficPPS,
			PacketsToSend: trafficPackets,
			EthFlow:       &otgconfighelpers.EthFlowParams{SrcMAC: ateSrc.MAC},
			IPv6Flow:      &otgconfighelpers.IPv6FlowParams{IPv6Src: ateSrc.IPv6, IPv6Dst: pushFECDstIP},
		}
		f.CreateFlow(top)
		f.AddEthHeader()
		f.AddIPv6Header()

		pv := &packetvalidationhelpers.PacketValidation{
			PortName:    "port2",
			CaptureName: "v6-mpls-push",
			Validations: []packetvalidationhelpers.ValidationType{packetvalidationhelpers.ValidateMPLSLayer},
			MPLSLayer:   &packetvalidationhelpers.MPLSLayer{Label: mplsPushLabelV6},
		}
		packetvalidationhelpers.ConfigurePacketCapture(t, top, pv)
		pushAndResolve(t, ate, top)

		cfgplugins.NewStaticMplsLspPushLabel(t, dut, v6LSPName, p1.Name(), ateDst.IPv6, pushFECDstCIDR, mplsPushLabelV6, lspNextHopIndex, "ipv6")
		t.Cleanup(func() { cfgplugins.RemoveStaticMplsLspPushLabel(t, dut, v6LSPName, p1.Name()) })

		cs := packetvalidationhelpers.StartCapture(t, ate)
		runFlows(t, ate, []string{"v6-push"})
		packetvalidationhelpers.StopCapture(t, ate, cs)
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
