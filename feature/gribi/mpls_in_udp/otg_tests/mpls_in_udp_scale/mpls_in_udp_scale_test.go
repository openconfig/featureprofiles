package mpls_in_udp_scale_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"math/big"
	"math/rand"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/gribigo/client"
	"github.com/openconfig/gribigo/constants"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Constants
const (
	portSpeed                    = oc.IfEthernet_ETHERNET_SPEED_SPEED_100GB
	startVLANPort1               = 100
	startVLANPort2               = 200
	numVRFs                      = 1023
	trafficDuration              = 15 * time.Second
	nhIDStart                    = 201
	mplsNHID                     = uint64(1001)
	mplsNHGID                    = uint64(2001)
	outerIPv6Src                 = "2001:db8::1"
	outerIPv6Dst                 = "2001:db8::100"
	outerDstUDPPort              = 6635
	outerDSCP                    = 26
	outerIPTTL                   = 64
	ttl                          = uint32(100) // Inner packet TTL
	mplsLabel                    = 100         // Each NHG gets a unique label
	ipv4PrefixLen                = 30
	ipv6PrefixLen                = 126
	tolerance                    = 5
	ratePps                      = 10000
	pktSize                      = 256
	policyName                   = "redirect-to-vrf_t"
	ipv4AddrPfx1                 = "192.51.100.%d"
	ipv4AddrPfx2                 = "193.51.100.%d"
	ipv6AddrPfx1                 = "2001:db8:1:5::%d"
	ipv6AddrPfx2                 = "2002:db8:1:6::%d"
	pbfIpv6                      = "2001:db8::%d/128"
	magicIP                      = "192.168.1.1"
	magicMac                     = "02:00:00:00:00:01"
	ipCount                      = 10
	otgMultiPortCaptureSupported = false
	baseIPv6                     = "2001:db8:100::" // starting prefix for IPv6
	baseIPv4                     = "198.51.100.0"   // starting prefix for IPv4
)

// DUT and ATE port attributes
var (
	dutPort1 = attrs.Attributes{Desc: "DUT Ingress Port 1", MAC: "02:01:01:00:00:01", IPv4: "192.51.100.1", IPv4Len: ipv4PrefixLen, IPv6: "2001:db8:1::1", IPv6Len: ipv6PrefixLen}
	dutPort2 = attrs.Attributes{Desc: "DUT Ingress Port 2", MAC: "02:01:01:00:00:02", IPv4: "193.51.100.1", IPv4Len: ipv4PrefixLen, IPv6: "2002:db8:1::1", IPv6Len: ipv6PrefixLen}
	dutPort3 = attrs.Attributes{Desc: "DUT Egress Port 3", MAC: "02:01:01:00:00:03", IPv4: "194.51.100.1", IPv4Len: ipv4PrefixLen, IPv6: "2003:db8:1::1", IPv6Len: ipv6PrefixLen}
	dutPort4 = attrs.Attributes{Desc: "DUT Egress Port 4", MAC: "02:01:01:00:00:04", IPv4: "195.51.100.1", IPv4Len: ipv4PrefixLen, IPv6: "2004:db8:1::1", IPv6Len: ipv6PrefixLen}

	atePort1 = attrs.Attributes{Name: "ATE-Ingress-Port-1", MAC: "02:02:02:00:00:01", IPv4: "192.51.100.2", IPv4Len: ipv4PrefixLen, IPv6: "2001:db8:1::2", IPv6Len: ipv6PrefixLen}
	atePort2 = attrs.Attributes{Name: "ATE-Ingress-Port-2", MAC: "02:02:02:00:00:02", IPv4: "193.51.100.2", IPv4Len: ipv4PrefixLen, IPv6: "2002:db8:1::2", IPv6Len: ipv6PrefixLen}
	atePort3 = attrs.Attributes{Name: "ATE-Egress-Port-3", MAC: "02:02:02:00:00:03", IPv4: "194.51.100.2", IPv4Len: ipv4PrefixLen, IPv6: "2003:db8:1::2", IPv6Len: ipv6PrefixLen}
	atePort4 = attrs.Attributes{Name: "ATE-Egress-Port-4", MAC: "02:02:02:00:00:04", IPv4: "195.51.100.2", IPv4Len: ipv4PrefixLen, IPv6: "2004:db8:1::2", IPv6Len: ipv6PrefixLen}

	dutPort3DummyIP = attrs.Attributes{
		Desc:       "dutPort3",
		IPv4Sec:    "192.0.2.21",
		IPv4LenSec: ipv4PrefixLen,
	}

	otgPort3DummyIP = attrs.Attributes{
		Desc:    "otgPort3",
		IPv4:    "192.0.2.22",
		IPv4Len: ipv4PrefixLen,
	}

	dutPort4DummyIP = attrs.Attributes{
		Desc:       "dutPort4",
		IPv4Sec:    "193.0.2.21",
		IPv4LenSec: ipv4PrefixLen,
	}

	otgPort4DummyIP = attrs.Attributes{
		Desc:    "otgPort4",
		IPv4:    "193.0.2.22",
		IPv4Len: ipv4PrefixLen,
	}
	// IPv6 flow configuration for MPLS-in-UDP testing
	fa6 = flowAttr{
		src:      atePort1.IPv6,
		dst:      strings.Split(baseIPv6, "/")[0], // Extract IPv6 prefix for inner destination
		srcMac:   atePort1.MAC,
		dstMac:   dutPort1.MAC,
		srcPort:  "port1",
		dstPorts: []string{"port3", "port4"},
		topo:     gosnappi.NewConfig(),
	}
	fa4 = flowAttr{
		src:      atePort1.IPv4,
		dst:      strings.Split("198.51.100.0", "/")[0], // Extract IPv6 prefix for inner destination
		srcMac:   atePort1.MAC,
		dstMac:   dutPort1.MAC,
		srcPort:  "port1",
		dstPorts: []string{"port3", "port4"},
		topo:     gosnappi.NewConfig(),
	}
	portsTrafficDistribution = []uint64{50, 50}
	profiles                 = map[int]profileConfig{
		1: {20000, 1}, // 20k NHGs × 1 NH
		2: {20000, 1}, // same as profile 1
		3: {20000, 1}, // same as profile 1
		4: {2500, 8},  // 2.5k NHGs × 8 NHs = 20k NHs
		5: {20000, 1}, // QPS scaling
	}
	totalPrefixes = 20000
)

type profileConfig struct {
	totalNHGs int
	nhsPerNHG int
}

// flowAttr defines traffic flow attributes for test packets
type flowAttr struct {
	src      string   // source IP address
	dst      string   // destination IP address
	srcPort  string   // source OTG port
	dstPorts []string // destination OTG ports
	srcMac   string   // source MAC address
	dstMac   string   // destination MAC address
	topo     gosnappi.Config
}

// testArgs holds the objects needed by a test case
type testArgs struct {
	dut    *ondatra.DUTDevice
	ate    *ondatra.ATEDevice
	topo   gosnappi.Config
	client *gribi.Client
}

type packetResult struct {
	mplsLabel uint64
	// NOTE: Source UDP port is not validated since it is random
	// udpSrcPort uint16
	udpDstPort uint16
	ipTTL      uint8
	srcIP      string
	dstIP      string
}

type testCase struct {
	name                string
	entries             []fluent.GRIBIEntry
	wantAddResults      []*client.OpResult
	wantDelResults      []*client.OpResult
	flows               []gosnappi.Flow
	capturePorts        []string
	wantMPLSLabel       uint64
	wantOuterDstIP      string
	wantOuterSrcIP      string
	wantOuterDstUDPPort uint16
	wantOuterIPTTL      uint8
}

// configureDUT configures all ports with base IPs and subinterfaces with VRF and VLANs.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) []string {
	t.Helper()
	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")
	dp3 := dut.Port(t, "port3")
	dp4 := dut.Port(t, "port4")
	d := gnmi.OC()

	// Create VRFs + PBF (true enables policy-based forwarding rules)
	vrfsList := createVRFsBatched(t, dut, numVRFs, true)

	portList := []*ondatra.Port{dp1, dp2, dp3, dp4}
	dutPortAttrs := []attrs.Attributes{dutPort1, dutPort2, dutPort3, dutPort4}

	for idx, a := range dutPortAttrs {
		p := portList[idx]
		intf := a.NewOCInterface(p.Name(), dut)

		// Vendor/PMD-specific port speed configuration
		if p.PMD() == ondatra.PMD100GBASELR4 && dut.Vendor() != ondatra.CISCO && dut.Vendor() != ondatra.JUNIPER {
			e := intf.GetOrCreateEthernet()
			if !deviations.AutoNegotiateUnsupported(dut) {
				e.AutoNegotiate = ygot.Bool(false)
			}
			if !deviations.DuplexModeUnsupported(dut) {
				e.DuplexMode = oc.Ethernet_DuplexMode_FULL
			}
			if !deviations.PortSpeedUnsupported(dut) {
				e.PortSpeed = oc.IfEthernet_ETHERNET_SPEED_SPEED_100GB
			}
		}
		gnmi.Replace(t, dut, d.Interface(p.Name()).Config(), intf)
		t.Logf("Configured DUT port %s (%s)", p.Name(), a.Desc)
	}

	fptest.ConfigureDefaultNetworkInstance(t, dut)
	// configure 16 L3 subinterfaces under DUT port#1 & 2 and assign them to DEFAULT vrf
	configureDUTSubIfs(t, &oc.Root{}, dut, dp1, ipv4AddrPfx1, ipv6AddrPfx1, startVLANPort1)
	configureDUTSubIfs(t, &oc.Root{}, dut, dp2, ipv4AddrPfx2, ipv6AddrPfx2, startVLANPort2)

	// configure an L3 subinterface without vlan tagging under DUT port#3 & 4
	createSubifDUT(t, &oc.Root{}, dut, dp3, 0, 0, dutPort3.IPv4, dutPort3.IPv6)
	createSubifDUT(t, &oc.Root{}, dut, dp4, 0, 0, dutPort4.IPv4, dutPort4.IPv6)

	applyForwardingPolicy(t, dp1.Name())
	applyForwardingPolicy(t, dp2.Name())
	// Set static ARP for gRIBI NH MAC resolution
	if deviations.GRIBIMACOverrideWithStaticARP(dut) {
		staticARPWithSecondaryIP(t, dut)
	} else if deviations.GRIBIMACOverrideStaticARPStaticRoute(dut) {
		staticARPWithMagicUniversalIP(t, dut)
	}
	return vrfsList
}

// createSubifDUT creates a single L3 subinterface
func createSubifDUT(t *testing.T, d *oc.Root, dut *ondatra.DUTDevice, dutPort *ondatra.Port, index uint32, vlanID uint16, ipv4Addr, ipv6Addr string) {
	t.Helper()
	ifName := dutPort.Name()
	i := d.GetOrCreateInterface(dutPort.Name())
	s := i.GetOrCreateSubinterface(index)
	if vlanID != 0 {
		if deviations.DeprecatedVlanID(dut) {
			s.GetOrCreateVlan().VlanId = oc.UnionUint16(vlanID)
		} else {
			s.GetOrCreateVlan().GetOrCreateMatch().GetOrCreateSingleTagged().VlanId = ygot.Uint16(vlanID)
		}
	}
	s4 := s.GetOrCreateIpv4()
	a4 := s4.GetOrCreateAddress(ipv4Addr)
	a4.PrefixLength = ygot.Uint8(uint8(ipv4PrefixLen))
	if deviations.InterfaceEnabled(dut) {
		s4.Enabled = ygot.Bool(true)
	}
	s6 := s.GetOrCreateIpv6()
	a6 := s6.GetOrCreateAddress(ipv6Addr)
	a6.PrefixLength = ygot.Uint8(uint8(ipv6PrefixLen))
	if deviations.InterfaceEnabled(dut) {
		s6.Enabled = ygot.Bool(true)
	}
	gnmi.Replace(t, dut, gnmi.OC().Interface(ifName).Subinterface(index).Config(), s)
}

// configureDUTSubIfs configures 16 DUT subinterfaces on the target device
func configureDUTSubIfs(t *testing.T, d *oc.Root, dut *ondatra.DUTDevice, dutPort *ondatra.Port, prefixFmtV4, prefixFmtV6 string, startVLANPort int) {
	t.Helper()
	for i := startVLANPort; i < 16; i++ {
		index := uint32(i)
		vlanID := uint16(i)
		if deviations.NoMixOfTaggedAndUntaggedSubinterfaces(dut) {
			vlanID = uint16(i) + 1
		}
		dutIPv4 := fmt.Sprintf(prefixFmtV4, (4*i)+2)
		dutIPv6 := fmt.Sprintf(prefixFmtV6, (5*i)+2)
		createSubifDUT(t, d, dut, dutPort, index, vlanID, dutIPv4, dutIPv6)
	}
}

// applyForwardingPolicy applies the forwarding policy on the interface.
func applyForwardingPolicy(t *testing.T, ingressPort string) {
	t.Helper()
	t.Logf("Applying forwarding policy on interface %v ... ", ingressPort)
	d := &oc.Root{}
	dut := ondatra.DUT(t, "dut")
	interfaceID := ingressPort
	if deviations.InterfaceRefInterfaceIDFormat(dut) {
		interfaceID = ingressPort + ".0"
	}
	pfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding().Interface(interfaceID)
	pfCfg := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut)).GetOrCreatePolicyForwarding().GetOrCreateInterface(interfaceID)
	pfCfg.ApplyVrfSelectionPolicy = ygot.String(policyName)
	pfCfg.GetOrCreateInterfaceRef().Interface = ygot.String(ingressPort)
	pfCfg.GetOrCreateInterfaceRef().Subinterface = ygot.Uint32(0)
	if deviations.InterfaceRefConfigUnsupported(dut) {
		pfCfg.InterfaceRef = nil
	}
	gnmi.Replace(t, dut, pfPath.Config(), pfCfg)
}

func createVRFsBatched(t *testing.T, dut *ondatra.DUTDevice, vrfCount int, enablePBF bool) []string {
	t.Helper()
	droot := &oc.Root{}
	vrfs := []string{deviations.DefaultNetworkInstance(dut)}

	// DEFAULT NI
	ni := droot.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	ni.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE

	sb := &gnmi.SetBatch{}
	gnmi.BatchUpdate(sb, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Config(), ni)

	// VRFs
	for i := 1; i <= vrfCount; i++ {
		name := fmt.Sprintf("VRF_%03d", i)
		vrfs = append(vrfs, name)
		ni := droot.GetOrCreateNetworkInstance(name)
		ni.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF
		gnmi.BatchReplace(sb, gnmi.OC().NetworkInstance(name).Config(), ni)
	}

	// PBF
	if enablePBF {
		pbf := configurePBF(t, dut, vrfCount)
		gnmi.BatchReplace(sb, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding().Config(), pbf)
	}
	sb.Set(t, dut)
	return vrfs
}

func configurePBF(t *testing.T, dut *ondatra.DUTDevice, vrfCount int) *oc.NetworkInstance_PolicyForwarding {
	t.Helper()
	d := &oc.Root{}
	ni := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	pf := ni.GetOrCreatePolicyForwarding()

	// Create one policy to hold multiple rules
	policy := pf.GetOrCreatePolicy(policyName)
	policy.SetType(oc.Policy_Type_VRF_SELECTION_POLICY)

	// Add one rule per VRF up to vrfCount
	for i := 1; i <= vrfCount; i++ {
		ruleID := uint32(i)
		vrfName := fmt.Sprintf("VRF_%03d", i)

		rule := policy.GetOrCreateRule(ruleID)
		rule.GetOrCreateIpv6().SourceAddress = ygot.String(fmt.Sprintf(pbfIpv6, i))
		rule.GetOrCreateIpv6().DscpSet = []uint8{uint8(i % 64)}
		rule.GetOrCreateAction().NetworkInstance = ygot.String(vrfName)
	}

	// Add default catch-all rule (optional but recommended)
	defaultRuleID := uint32(10000)
	defaultRule := policy.GetOrCreateRule(defaultRuleID)
	defaultRule.GetOrCreateIpv6().SourceAddress = ygot.String("::/0")
	defaultRule.GetOrCreateAction().NetworkInstance = ygot.String(deviations.DefaultNetworkInstance(dut))

	return pf
}

func configureATE(t *testing.T, ateConfig gosnappi.Config, atePort *ondatra.Port, vlanID uint16, Name, MAC, dutIPv4, ateIPv4, dutIPv6, ateIPv6 string) {
	t.Helper()
	dev := ateConfig.Devices().Add().SetName(Name + ".Dev")
	eth := dev.Ethernets().Add().SetName(Name + ".Eth").SetMac(MAC)
	eth.Connection().SetPortName(atePort.ID())
	eth.Vlans().Add().SetName(Name).SetId(uint32(vlanID))
	eth.Ipv4Addresses().Add().SetName(Name + ".IPv4").SetAddress(ateIPv4).SetGateway(dutIPv4).SetPrefix(uint32(ipv4PrefixLen))
	eth.Ipv6Addresses().Add().SetName(Name + ".IPv6").SetAddress(ateIPv6).SetGateway(dutIPv6).SetPrefix(uint32(ipv6PrefixLen))
}

// configureATESubIfs configures 16 ATE subinterfaces on the target device
// It returns a slice of the corresponding ATE IPAddresses.
func configureATESubIfs(t *testing.T, ateConfig gosnappi.Config, atePort *ondatra.Port, dut *ondatra.DUTDevice, Name, Mac, prefixFmtV4, prefixFmtV6 string, startVLANPort int) []string {
	t.Helper()
	nextHops := []string{}
	for i := startVLANPort; i < 16; i++ {
		vlanID := uint16(i)
		if deviations.NoMixOfTaggedAndUntaggedSubinterfaces(dut) {
			vlanID = uint16(i) + 1
		}
		dutIPv4 := fmt.Sprintf(prefixFmtV4, (4*i)+2)
		ateIPv4 := fmt.Sprintf(prefixFmtV4, (4*i)+1)
		dutIPv6 := fmt.Sprintf(prefixFmtV6, (5*i)+2)
		ateIPv6 := fmt.Sprintf(prefixFmtV6, (5*i)+1)
		name := fmt.Sprintf("%s%d", Name, i)
		mac, _ := incrementMAC(Mac, i+1)
		configureATE(t, ateConfig, atePort, vlanID, name, mac, dutIPv4, ateIPv4, dutIPv6, ateIPv6)
		nextHops = append(nextHops, ateIPv6)
	}
	return nextHops
}

func configureOTG(t *testing.T, ate *ondatra.ATEDevice, dut *ondatra.DUTDevice, ateConfig gosnappi.Config) gosnappi.Config {
	t.Helper()
	ap1 := ate.Port(t, "port1")
	ap2 := ate.Port(t, "port2")
	ap3 := ate.Port(t, "port3")
	ap4 := ate.Port(t, "port4")

	ateConfig.Ports().Add().SetName(ap1.ID())
	ateConfig.Ports().Add().SetName(ap2.ID())
	ateConfig.Ports().Add().SetName(ap3.ID())
	ateConfig.Ports().Add().SetName(ap4.ID())

	configureATE(t, ateConfig, ap3, 0, atePort3.Name, atePort3.MAC, dutPort3.IPv4, atePort3.IPv4, dutPort3.IPv6, atePort3.IPv6)
	configureATE(t, ateConfig, ap4, 0, atePort4.Name, atePort4.MAC, dutPort4.IPv4, atePort4.IPv4, dutPort4.IPv6, atePort4.IPv6)
	// subIntfIPs is a []string slice with ATE IPv6 addresses for all the subInterfaces
	configureATESubIfs(t, ateConfig, ap1, dut, atePort1.Name, atePort1.MAC, ipv4AddrPfx1, ipv6AddrPfx1, startVLANPort1)
	configureATESubIfs(t, ateConfig, ap2, dut, atePort2.Name, atePort2.MAC, ipv4AddrPfx2, ipv6AddrPfx2, startVLANPort2)
	var pmd100GBASELR4 []string
	for _, p := range ateConfig.Ports().Items() {
		port := ate.Port(t, p.Name())
		if port.PMD() == ondatra.PMD100GBASELR4 {
			pmd100GBASELR4 = append(pmd100GBASELR4, port.ID())
		}
	}
	if len(pmd100GBASELR4) > 0 {
		l1Settings := ateConfig.Layer1().Add().SetName("L1").SetPortNames(pmd100GBASELR4)
		l1Settings.SetAutoNegotiate(true).SetIeeeMediaDefaults(false).SetSpeed("speed_100_gbps")
		autoNegotiate := l1Settings.AutoNegotiation()
		autoNegotiate.SetRsFec(false)
	}
	return ateConfig
}

func programBasicEntries(t *testing.T, dut *ondatra.DUTDevice, c *gribi.Client) {
	t.Helper()
	t.Log("Setting up routing infrastructure for MPLS-in-UDP with ECMP on port3 and port4")

	// Assign unique IDs for NHs and NHG
	nhIDs := []uint64{300, 301} // unique NextHop IDs for port3 and port4
	nhgID := uint64(400)

	var nhEntries []fluent.GRIBIEntry
	var nhOps []*client.OpResult

	// Build next-hops for both ports
	for i, p := range []string{"port3", "port4"} {
		port := dut.Port(t, p)

		switch {
		case deviations.GRIBIMACOverrideWithStaticARP(dut):
			nh, op := gribi.NHEntry(
				nhIDs[i], "MACwithIp", deviations.DefaultNetworkInstance(dut),
				fluent.InstalledInFIB,
				&gribi.NHOptions{Dest: []string{otgPort3DummyIP.IPv4, otgPort4DummyIP.IPv4}[i], Mac: magicMac},
			)
			nhEntries = append(nhEntries, nh)
			nhOps = append(nhOps, op)

		case deviations.GRIBIMACOverrideStaticARPStaticRoute(dut):
			nh, op := gribi.NHEntry(
				nhIDs[i], "MACwithInterface", deviations.DefaultNetworkInstance(dut),
				fluent.InstalledInFIB,
				&gribi.NHOptions{Interface: port.Name(), Mac: magicMac, Dest: magicIP},
			)
			nhEntries = append(nhEntries, nh)
			nhOps = append(nhOps, op)

		default:
			nh, op := gribi.NHEntry(
				nhIDs[i], "MACwithInterface", deviations.DefaultNetworkInstance(dut),
				fluent.InstalledInFIB,
				&gribi.NHOptions{Interface: port.Name(), Mac: magicMac},
			)
			nhEntries = append(nhEntries, nh)
			nhOps = append(nhOps, op)
		}
	}

	// Build NHG with both next-hops (ECMP)
	nhMap := map[uint64]uint64{
		nhIDs[0]: 1, // weight 1
		nhIDs[1]: 1, // weight 1
	}
	nhg, nhgOp := gribi.NHGEntry(nhgID, nhMap, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	nhEntries = append(nhEntries, nhg)
	nhOps = append(nhOps, nhgOp)

	// Install all NH + NHG entries
	c.AddEntries(t, nhEntries, nhOps)

	// Add IPv6 route for outer destination to point to the NHG
	c.AddIPv6(t, outerIPv6Dst+"/128", nhgID,
		deviations.DefaultNetworkInstance(dut),
		deviations.DefaultNetworkInstance(dut),
		fluent.InstalledInFIB)

	t.Logf("Installed ECMP route %s/128 via ports 3 and 4", outerIPv6Dst)
}

// programMPLSinUDPEntries programs gRIBI entries for MPLS-in-UDP encapsulation.
// It installs a single NextHop that performs MPLS-in-UDP encapsulation with the
// provided outer IPv6 and UDP header attributes, associates multiple NextHopGroups
// (NHGs) with that NextHop, and finally installs IPv6 /128 routes pointing to each NHG.
func programMPLSinUDPEntries(
	t *testing.T,
	dut *ondatra.DUTDevice,
	mplsNHID uint64,
	mplsLabelStart uint64,
	numNHGs int,
	outerIPv6Src, outerIPv6Dst string,
	outerDstUDPPort uint16,
	outerIPTTL uint8,
	outerDSCP uint8,
) []fluent.GRIBIEntry {

	entries := []fluent.GRIBIEntry{}

	// Create the single MPLS-in-UDP NextHop
	entries = append(entries,
		fluent.NextHopEntry().
			WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(mplsNHID).
			AddEncapHeader(
				fluent.MPLSEncapHeader().WithLabels(mplsLabelStart),
				fluent.UDPV6EncapHeader().
					WithSrcIP(outerIPv6Src).
					WithDstIP(outerIPv6Dst).
					WithDstUDPPort(uint64(outerDstUDPPort)).
					WithIPTTL(uint64(outerIPTTL)).
					WithDSCP(uint64(outerDSCP)),
			),
	)

	// Create NextHopGroups pointing to the same NH
	for i := 0; i < numNHGs; i++ {
		nhgID := mplsNHGID + uint64(i)
		entries = append(entries,
			fluent.NextHopGroupEntry().
				WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithID(nhgID).
				AddNextHop(mplsNHID, 1),
		)
	}

	// Create IPv6 routes pointing to those NHGs
	baseIP := net.ParseIP(baseIPv6)
	ipInt := big.NewInt(0).SetBytes(baseIP.To16())
	for i := 0; i < numNHGs; i++ {
		ipBytes := ipInt.FillBytes(make([]byte, 16))
		prefix := fmt.Sprintf("%s/128", net.IP(ipBytes).String())
		ipInt.Add(ipInt, big.NewInt(1))

		nhgID := mplsNHGID + uint64(i)
		entries = append(entries,
			fluent.IPv6Entry().
				WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithPrefix(prefix).
				WithNextHopGroup(nhgID).
				WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut)),
		)
	}

	return entries
}

func programMPLSinUDPMultiEntries(
	t *testing.T,
	dut *ondatra.DUTDevice,
	vrfs []string,
	mplsNHBase uint64,
	mplsLabelStart uint64,
	numNHGs int,
	outerIPv6Src, outerIPv6Dst string,
	outerDstUDPPort uint16,
	outerIPTTL uint8,
	outerDSCP uint8,
) []fluent.GRIBIEntry {

	totalEntries := len(vrfs) * (1 + numNHGs + numNHGs) // NH + NHGs + routes
	entries := make([]fluent.GRIBIEntry, 0, totalEntries)

	for vrfIdx, vrfName := range vrfs {
		label := mplsLabelStart + uint64(vrfIdx)
		nhID := mplsNHBase + uint64(vrfIdx)

		// Step 1: NextHop
		entries = append(entries,
			fluent.NextHopEntry().
				WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithIndex(nhID).
				AddEncapHeader(
					fluent.MPLSEncapHeader().WithLabels(label),
					fluent.UDPV6EncapHeader().
						WithSrcIP(outerIPv6Src).
						WithDstIP(outerIPv6Dst).
						WithDstUDPPort(uint64(outerDstUDPPort)).
						WithIPTTL(uint64(outerIPTTL)).
						WithDSCP(uint64(outerDSCP)),
				),
		)

		// Step 2 & 3: NextHopGroups and Routes
		for i := 0; i < numNHGs; i++ {
			nhgID := mplsNHGID + uint64(vrfIdx*numNHGs+i)

			entries = append(entries,
				fluent.NextHopGroupEntry().
					WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
					WithID(nhgID).
					AddNextHop(nhID, 1),
			)

			// Unique IPv6 prefix for this NHG
			offset := int64(vrfIdx*numNHGs + i)
			prefix := generateIPv6Prefix(baseIPv6, offset)

			entries = append(entries,
				fluent.IPv6Entry().
					WithNetworkInstance(vrfName).
					WithPrefix(prefix).
					WithNextHopGroup(nhgID).
					WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut)),
			)
		}
	}

	return entries
}

// awaitTimeout calls a fluent client Await, adding a timeout to the context.
func awaitTimeout(ctx context.Context, c *fluent.GRIBIClient, t testing.TB, timeout time.Duration) error {
	subctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return c.Await(subctx, t)
}

func staticARPWithMagicUniversalIP(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	sb := &gnmi.SetBatch{}

	// Define the static route prefix once
	sp := gnmi.OC().
		NetworkInstance(deviations.DefaultNetworkInstance(dut)).
		Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut)).
		Static(magicIP + "/32")

	// For both ports, add a unique next-hop under the same static prefix
	for i, portName := range []string{"port3", "port4"} {
		nhIndex := fmt.Sprintf("%d", i+1)
		port := dut.Port(t, portName)

		nh := &oc.NetworkInstance_Protocol_Static_NextHop{
			Index: ygot.String(nhIndex),
			InterfaceRef: &oc.NetworkInstance_Protocol_Static_NextHop_InterfaceRef{
				Interface: ygot.String(port.Name()),
			},
		}
		// Push this next-hop under the keyed child
		gnmi.BatchUpdate(sb, sp.NextHop(nhIndex).Config(), nh)

		// Also push a static ARP entry for this interface
		gnmi.BatchUpdate(sb,
			gnmi.OC().Interface(port.Name()).Config(),
			configStaticArp(port.Name(), magicIP, magicMac),
		)
		t.Logf("Added static route %s -> %s (NextHop %s)", magicIP, port.Name(), nhIndex)
	}
	// Commit all changes
	sb.Set(t, dut)
}

// validateMPLSPacketCapture validates MPLS-in-UDP encapsulated packets from capture
func validateMPLSPacketCapture(t *testing.T, ate *ondatra.ATEDevice, otgPortName string, pr *packetResult, labelList []uint64) {
	t.Helper()
	t.Logf("=== PACKET CAPTURE VALIDATION START for port %s ===", otgPortName)

	packetBytes := ate.OTG().GetCapture(t, gosnappi.NewCaptureRequest().SetPortName(otgPortName))
	t.Logf("Captured %d bytes from port %s", len(packetBytes), otgPortName)

	if len(packetBytes) == 0 {
		t.Errorf("No packet data captured on port %s", otgPortName)
		return
	}

	// Write capture to temporary pcap file for analysis
	f, err := os.CreateTemp("", ".pcap")
	if err != nil {
		t.Fatalf("Could not create temporary pcap file: %v", err)
	}
	if _, err := f.Write(packetBytes); err != nil {
		t.Fatalf("Could not write packetBytes to pcap file: %v", err)
	}
	f.Close()

	handle, err := pcap.OpenOffline(f.Name())
	if err != nil {
		t.Fatalf("Could not open pcap file: %v", err)
	}
	defer handle.Close()
	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())

	packetCount := 0
	mplsPacketCount := 0
	validMplsPacketCount := 0
	for packet := range packetSource.Packets() {
		packetCount++
		// Look for UDP-IPv6 packets (MPLS-in-UDP encapsulation)
		udpLayer := packet.Layer(layers.LayerTypeUDP)
		ipv6Layer := packet.Layer(layers.LayerTypeIPv6)
		if udpLayer == nil || ipv6Layer == nil {
			if packetCount < 5 {
				t.Logf("Packet %d: Skipping non-UDP-IPv6 packet", packetCount)
			}
			continue
		}
		mplsPacketCount++
		t.Logf("Packet %d: Found UDP-IPv6 packet for validation", packetCount)
		packetValid := true

		// Validate IPv6 outer header
		v6Packet := ipv6Layer.(*layers.IPv6)
		t.Logf("Packet %d: IPv6 src=%s, dst=%s, hopLimit=%d", packetCount,
			v6Packet.SrcIP.String(), v6Packet.DstIP.String(), v6Packet.HopLimit)

		if v6Packet.DstIP.String() != pr.dstIP {
			t.Errorf("Packet %d: Got outer destination IP %s, want %s", packetCount, v6Packet.DstIP.String(), pr.dstIP)
			packetValid = false
		}
		if v6Packet.SrcIP.String() != pr.srcIP {
			t.Errorf("Packet %d: Got outer source IP %s, want %s", packetCount, v6Packet.SrcIP.String(), pr.srcIP)
			packetValid = false
		}
		if v6Packet.HopLimit != pr.ipTTL {
			t.Errorf("Packet %d: Got outer hop limit %d, want %d", packetCount, v6Packet.HopLimit, pr.ipTTL)
			packetValid = false
		}

		// Validate UDP header - extract raw bytes for robust parsing
		udpHeaderBytes := udpLayer.LayerContents()
		t.Logf("Packet %d: UDP header bytes: %X", packetCount, udpHeaderBytes)

		if len(udpHeaderBytes) < 8 {
			t.Errorf("Packet %d: UDP header too short (len: %d)", packetCount, len(udpHeaderBytes))
			packetValid = false
		}

		// Validate MPLS header inside UDP payload
		payload := udpLayer.LayerPayload()
		if len(payload) < 4 {
			t.Errorf("Packet %d: UDP payload too short for MPLS header, len=%d", packetCount, len(payload))
			packetValid = false
		} else {
			mplsHeaderVal := binary.BigEndian.Uint32(payload[:4])
			label := (mplsHeaderVal >> 12) & 0xFFFFF
			bottomOfStack := (mplsHeaderVal >> 8) & 0x1
			mplsTTL := mplsHeaderVal & 0xFF
			t.Logf("Packet %d: %s", packetCount, formatMPLSHeader(payload[:4]))

			if len(labelList) != 0 {
				// Validate that label is in labelList
				found := false
				for _, l := range labelList {
					if uint64(label) == l {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Packet %d: Got MPLS Label %d, want one of %v", packetCount, label, labelList)
					packetValid = false
				}
			} else {
				// Validate exact match with pr.mplsLabel
				if uint64(label) != pr.mplsLabel {
					t.Errorf("Packet %d: Got MPLS Label %d, want %d", packetCount, label, pr.mplsLabel)
					packetValid = false
				}
			}
			if bottomOfStack != 1 {
				t.Errorf("Packet %d: Got MPLS Bottom of Stack bit %d, want 1", packetCount, bottomOfStack)
				packetValid = false
			}
			expectedMPLSTTL := ttl - 1 // Inner packet TTL decremented by 1
			if uint32(mplsTTL) != expectedMPLSTTL {
				t.Errorf("Packet %d: Got MPLS TTL %d, want %d", packetCount, mplsTTL, expectedMPLSTTL)
				packetValid = false
			}
		}

		if packetValid {
			validMplsPacketCount++
			if validMplsPacketCount <= 2 {
				t.Logf("Packet %d: MPLS validation PASSED", packetCount)
			}
		} else {
			t.Logf("Packet %d: MPLS validation FAILED", packetCount)
		}
	}

	// Summary and validation results
	t.Logf("=== PACKET CAPTURE VALIDATION SUMMARY ===")
	t.Logf("Total packets captured: %d", packetCount)
	t.Logf("UDP-IPv6 packets found: %d", mplsPacketCount)
	t.Logf("Valid MPLS-in-UDP packets: %d", validMplsPacketCount)

	if packetCount == 0 {
		t.Errorf("No packets captured on port %s", otgPortName)
	} else if mplsPacketCount == 0 {
		t.Errorf("No UDP-IPv6 packets found in capture on port %s", otgPortName)
	} else if validMplsPacketCount == 0 {
		t.Errorf("No valid MPLS-in-UDP packets found in capture on port %s", otgPortName)
	} else if validMplsPacketCount < (mplsPacketCount / 2) {
		t.Errorf("Many packets (%d/%d) failed validation", mplsPacketCount-validMplsPacketCount, mplsPacketCount)
	} else {
		t.Logf("Packet capture validation PASSED: Found %d valid MPLS-in-UDP packets", validMplsPacketCount)
	}
}

// Main test entry point.
func TestMPLSinUDPScale(t *testing.T) {
	t.Helper()
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	ateConfig := gosnappi.NewConfig()
	ctx := context.Background()
	vrfsList := configureDUT(t, dut)
	configureOTG(t, ate, dut, ateConfig)
	ate.OTG().PushConfig(t, ateConfig)
	ate.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), ateConfig, "IPv6")

	// Configure gRIBI client
	c := gribi.Client{
		DUT:         dut,
		FIBACK:      true,
		Persistence: true,
	}

	if err := c.Start(t); err != nil {
		t.Fatalf("gRIBI Connection can not be established")
	}
	defer c.Close(t)
	c.BecomeLeader(t)

	// Flush all existing AFT entries and set up basic routing infrastructure
	c.FlushAll(t)
	programBasicEntries(t, dut, &c)
	t.Run("Profile-1-Single VRF", func(t *testing.T) {
		configureVrfProfiles(t, ateConfig, ctx, dut, ate, &c, []string{deviations.DefaultNetworkInstance(dut)}, 1)
	})
	t.Run("Profile-2-Multi VRF", func(t *testing.T) {
		configureVrfProfiles(t, ateConfig, ctx, dut, ate, &c, vrfsList, 2)
	})
	t.Run("Profile-3-Multi VRF with Skew", func(t *testing.T) {
		configureVrfProfiles(t, ateConfig, ctx, dut, ate, &c, vrfsList, 3)
	})
	t.Run("Profile-4-Single VRF", func(t *testing.T) {
		configureVrfProfiles(t, ateConfig, ctx, dut, ate, &c, []string{deviations.DefaultNetworkInstance(dut)}, 4)
	})
	t.Run("Profile-5-Single VRF", func(t *testing.T) {
		configureVrfProfiles(t, ateConfig, ctx, dut, ate, &c, []string{deviations.DefaultNetworkInstance(dut)}, 5)
	})
}

// configureVrfProfiles implements the “Single/Multi VRF Validation” for Profile 1 (baseline) and Profile 4 (ECMP).
// It programs MPLS-in-UDP NHs, NHGs, and 20k prefixes (10k v4 + 10k v6),
// validates FIB/AFT, sends traffic, checks MPLS-over-UDP encapsulation, and deletes entries.
func configureVrfProfiles(
	t *testing.T,
	ateConfig gosnappi.Config,
	ctx context.Context,
	dut *ondatra.DUTDevice,
	ate *ondatra.ATEDevice,
	c *gribi.Client,
	vrfs []string,
	profile int, // 1 = baseline, 4 = ECMP
) {
	t.Helper()
	cfg, ok := profiles[profile]
	if !ok {
		t.Fatalf("Unsupported profile %d", profile)
	}
	totalNHGs := cfg.totalNHGs
	nhsPerNHG := cfg.nhsPerNHG
	totalNHs := totalNHGs * nhsPerNHG
	var entries []fluent.GRIBIEntry
	var wantAdds []*client.OpResult
	var wantDels []*client.OpResult
	var flows []gosnappi.Flow
	var labelList []uint64
	// === Program MPLS-in-UDP NH & NHG entries ===
	if profile == 2 {
		entries = programMPLSinUDPMultiEntries(
			t,
			dut,
			vrfs,
			mplsNHID,
			mplsLabel, // one consistent label across all NHs
			totalNHGs,
			outerIPv6Src, outerIPv6Dst,
			outerDstUDPPort,
			outerIPTTL,
			outerDSCP,
		)
		wantAdds, wantDels = expectedMPLSinUDPMultiOpResults(
			t,
			vrfs,
			mplsNHID,
			totalNHGs,
			mplsNHGID,
		)
		flows = []gosnappi.Flow{
			fa6.getFlow("ipv6", fmt.Sprintf("ip6mpls_p%d", profile), outerDSCP),
		}
	} else if profile == 3 {
		skewPattern := generateSkewPattern(len(vrfs), len(vrfs))
		vrfsSkewList := generateSkewedVRFList(t, vrfs, totalPrefixes, skewPattern)
		entries, labelList = programMPLSinUDPMultiEntriesSkew(
			t,
			dut,
			vrfsSkewList,
			mplsNHID,
			mplsLabel, // one consistent label across all NHs
			totalNHGs,
			outerIPv6Src, outerIPv6Dst,
			outerDstUDPPort,
			outerIPTTL,
			outerDSCP,
		)
		wantAdds, wantDels = expectedMPLSinUDPMultiOpResultsSkewed(
			t,
			vrfsSkewList,
			mplsNHID,
			totalNHGs,
			mplsNHGID,
		)
		flows = []gosnappi.Flow{
			fa6.getFlow("ipv6", fmt.Sprintf("ip6mpls_p%d", profile), outerDSCP),
		}
	} else {
		entries = programMPLSinUDPEntries(
			t,
			dut,
			mplsNHID,
			mplsLabel, // one consistent label across all NHs
			totalNHGs,
			outerIPv6Src, outerIPv6Dst,
			outerDstUDPPort,
			outerIPTTL,
			outerDSCP,
		)
		// === Add IPv4 + IPv6 route entries ===
		ipv4Entries := buildIPv4Routes(t, dut, totalPrefixes/2, baseIPv4, mplsNHGID)
		ipv6Entries := buildIPv6Routes(t, dut, totalPrefixes/2, baseIPv6, mplsNHGID)
		entries = append(entries, ipv4Entries...)
		entries = append(entries, ipv6Entries...)
		// === Expected OpResults ===
		wantAdds, wantDels = expectedMPLSinUDPOpResults(
			t,
			mplsNHID,
			totalNHGs,
			mplsNHGID, // nhgBase
			totalPrefixes,
			baseIPv4, baseIPv6, // used for v6 sample verification
		)
		flows = []gosnappi.Flow{
			fa4.getFlow("ipv4", fmt.Sprintf("ip4mpls_p%d", profile), outerDSCP),
			fa6.getFlow("ipv6", fmt.Sprintf("ip6mpls_p%d", profile), outerDSCP),
		}
	}

	// === Verify infra installed ===
	if err := c.AwaitTimeout(ctx, t, 3*time.Minute); err != nil {
		t.Fatalf("Failed to install infra entries for profile %d: %v", profile, err)
	}

	testCaseArgs := &testCase{
		name: fmt.Sprintf("Profile%d: MPLS-in-UDP Traffic Encap (Single VRF, %d NHGs × %d NHs, %d total prefixes split v4/v6)",
			profile, totalNHGs, nhsPerNHG, totalPrefixes),
		entries:             entries,
		wantAddResults:      wantAdds,
		wantDelResults:      wantDels,
		flows:               flows,
		capturePorts:        []string{"port3", "port4"},
		wantMPLSLabel:       uint64(mplsLabel),
		wantOuterDstIP:      outerIPv6Dst,
		wantOuterSrcIP:      outerIPv6Src,
		wantOuterDstUDPPort: outerDstUDPPort,
		wantOuterIPTTL:      outerIPTTL,
	}

	tcArgs := &testArgs{
		client: c,
		dut:    dut,
		ate:    ate,
		topo:   ateConfig,
	}

	// === Add Entries ===
	t.Logf("Programming Profile %d: %d NHGs × %d NHs/NHG = %d total NHs, %d prefixes (10k v4 + 10k v6)",
		profile, totalNHGs, nhsPerNHG, totalNHs, totalPrefixes)
	c.AddEntries(t, testCaseArgs.entries, testCaseArgs.wantAddResults)

	// === Capture & Send Traffic ===
	expectedPkt := &packetResult{
		mplsLabel:  testCaseArgs.wantMPLSLabel,
		udpDstPort: testCaseArgs.wantOuterDstUDPPort,
		ipTTL:      testCaseArgs.wantOuterIPTTL,
		srcIP:      testCaseArgs.wantOuterSrcIP,
		dstIP:      testCaseArgs.wantOuterDstIP,
	}
	if otgMultiPortCaptureSupported {
		enableCapture(t, ate.OTG(), ateConfig, testCaseArgs.capturePorts)
		sendTraffic(t, tcArgs, testCaseArgs.flows, true)
		validateMPLSPacketCapture(t, ate, testCaseArgs.capturePorts[0], expectedPkt, labelList)
		clearCapture(t, ate.OTG(), ateConfig)
	} else {
		for _, port := range testCaseArgs.capturePorts {
			enableCapture(t, ate.OTG(), ateConfig, []string{port})
			sendTraffic(t, tcArgs, testCaseArgs.flows, true)
			validateMPLSPacketCapture(t, ate, port, expectedPkt, labelList)
			clearCapture(t, ate.OTG(), ateConfig)
		}
	}

	// === Validate Forwarding ===
	validateTrafficFlows(t, ate, ateConfig, tcArgs, testCaseArgs.flows, false, true)
	// === Profile 5 specific QPS scaling ===
	if profile == 5 {
		t.Log("Starting Profile 5 high-rate gRIBI ops at ~1k ops/sec")

		// build 60k ops (20k × 3 per entry)
		ops, _ := buildProfile5Ops(t, dut, totalPrefixes, mplsNHGID, baseIPv4, baseIPv6)

		// stream ops at ~1k ops/sec
		pumpOpsAtRate(t, ctx, c, ops, 1000)
		t.Log("Starting Profile ops/sec")
		// while ops stream, keep sending dataplane traffic
		sendTraffic(t, tcArgs, testCaseArgs.flows, false)
		validateTrafficFlows(t, ate, ateConfig, tcArgs, testCaseArgs.flows, false, true)
		t.Log("Completed Profile 5 QPS scaling phase")
	}
	// === Delete Entries ===
	t.Logf("Deleting MPLS-in-UDP entries for Profile %d", profile)
	var revEntries []fluent.GRIBIEntry
	for i := len(testCaseArgs.entries) - 1; i >= 0; i-- {
		revEntries = append(revEntries, testCaseArgs.entries[i])
	}
	c.DeleteEntries(t, revEntries, testCaseArgs.wantDelResults)

	// === Validate Post-Delete (traffic loss) ===
	t.Logf("Verifying traffic fails after MPLS-in-UDP entries deleted for Profile %d", profile)
	validateTrafficFlows(t, ate, ateConfig, tcArgs, testCaseArgs.flows, false, false)
}

// expectedMPLSinUDPMultiOpResultsSkewed generates expected add/delete operation results
func expectedMPLSinUDPMultiOpResultsSkewed(
	t *testing.T,
	vrfsSkewList []string, // skewed VRF list (duplicates allowed)
	mplsNHBase uint64,
	numNHsPerVRF int,
	nhgBase uint64,
) (adds, dels []*client.OpResult) {
	t.Helper()
	total := len(vrfsSkewList)
	adds = make([]*client.OpResult, 0, total*3)
	dels = make([]*client.OpResult, 0, total*3)

	// Track NH created per VRF
	nhCreated := map[string]uint64{}
	// Track prefix counter per VRF
	prefixCounter := map[string]uint64{}

	baseIP := net.ParseIP(baseIPv6).To16()
	baseIPInt := binary.BigEndian.Uint64(baseIP[8:16]) // use last 64 bits as counter

	for idx, vrf := range vrfsSkewList {
		// NH: create once per VRF
		nhID, exists := nhCreated[vrf]
		if !exists {
			nhID = mplsNHBase + uint64(len(nhCreated))
			adds = append(adds,
				fluent.OperationResult().
					WithNextHopOperation(nhID).
					WithProgrammingResult(fluent.InstalledInFIB).
					WithOperationType(constants.Add).
					AsResult(),
			)
			dels = append(dels,
				fluent.OperationResult().
					WithNextHopOperation(nhID).
					WithProgrammingResult(fluent.InstalledInFIB).
					WithOperationType(constants.Delete).
					AsResult(),
			)
			nhCreated[vrf] = nhID
		}

		// NHG: unique per skewed entry
		nhgID := nhgBase + uint64(idx)
		adds = append(adds,
			fluent.OperationResult().
				WithNextHopGroupOperation(nhgID).
				WithProgrammingResult(fluent.InstalledInFIB).
				WithOperationType(constants.Add).
				AsResult(),
		)
		dels = append(dels,
			fluent.OperationResult().
				WithNextHopGroupOperation(nhgID).
				WithProgrammingResult(fluent.InstalledInFIB).
				WithOperationType(constants.Delete).
				AsResult(),
		)

		// IPv6 prefix: increment last 64 bits per VRF
		prefixCounter[vrf]++
		counter := prefixCounter[vrf]
		ipBytes := make([]byte, 16)
		copy(ipBytes[:8], baseIP[:8])
		binary.BigEndian.PutUint64(ipBytes[8:], baseIPInt+counter-1)
		prefix := fmt.Sprintf("%s/128", net.IP(ipBytes).String())

		adds = append(adds,
			fluent.OperationResult().
				WithIPv6Operation(prefix).
				WithProgrammingResult(fluent.InstalledInFIB).
				WithOperationType(constants.Add).
				AsResult(),
		)
		dels = append(dels,
			fluent.OperationResult().
				WithIPv6Operation(prefix).
				WithProgrammingResult(fluent.InstalledInFIB).
				WithOperationType(constants.Delete).
				AsResult(),
		)
	}

	return adds, dels
}

func programMPLSinUDPMultiEntriesSkew(
	t *testing.T,
	dut *ondatra.DUTDevice,
	vrfsSkewList []string,
	mplsNHBase uint64,
	mplsLabelStart uint64,
	numNHsPerVRF int, // currently one NH per VRF; kept for API compatibility
	outerIPv6Src, outerIPv6Dst string,
	outerDstUDPPort uint16,
	outerIPTTL uint8,
	outerDSCP uint8,
) ([]fluent.GRIBIEntry, []uint64) {
	t.Helper()

	total := len(vrfsSkewList)
	// 3 entries per skew index: NHG + IPv6 route; NH once per VRF (upper bound ~+total/2)
	entries := make([]fluent.GRIBIEntry, 0, total*3)

	// Hoist constants to avoid repeated calls/conversions.
	defaultNI := deviations.DefaultNetworkInstance(dut)
	// NH created once per VRF; map VRF -> NH ID
	nhCreated := make(map[string]uint64, 64) // capacity hint for many VRFs
	// Unique prefix counter per VRF
	prefixCounter := make(map[string]uint64, 64)

	// Fast IPv6 prefix allocator: increment the last 64 bits per VRF
	baseIP := net.ParseIP(baseIPv6).To16()
	if baseIP == nil {
		t.Fatalf("invalid base IPv6 for prefix allocation")
	}
	baseHi := make([]byte, 8)
	copy(baseHi, baseIP[:8]) // top 64 bits fixed
	baseLo := binary.BigEndian.Uint64(baseIP[8:16])
	labelList := []uint64{}
	for idx, vrf := range vrfsSkewList {
		label := mplsLabelStart + uint64(idx)
		labelList = append(labelList, label)
		// --- NH: create exactly once per VRF ---
		nhID, exists := nhCreated[vrf]
		if !exists {
			nhID = mplsNHBase + uint64(len(nhCreated)) // stable and monotonic per new VRF
			nhCreated[vrf] = nhID
			entries = append(entries,
				fluent.NextHopEntry().
					WithNetworkInstance(defaultNI).
					WithIndex(nhID).
					AddEncapHeader(
						fluent.MPLSEncapHeader().WithLabels(label),
						fluent.UDPV6EncapHeader().
							WithSrcIP(outerIPv6Src).
							WithDstIP(outerIPv6Dst).
							WithDstUDPPort(uint64(outerDstUDPPort)).
							WithIPTTL(uint64(outerIPTTL)).
							WithDSCP(uint64(outerDSCP)),
					),
			)
		}

		// --- NHG: unique per skewed occurrence ---
		nhgID := mplsNHGID + uint64(idx)
		entries = append(entries,
			fluent.NextHopGroupEntry().
				WithNetworkInstance(defaultNI).
				WithID(nhgID).
				AddNextHop(nhID, 1),
		)

		// --- IPv6 route: unique per VRF occurrence ---
		prefixCounter[vrf]++
		counter := prefixCounter[vrf] - 1 // start from 0
		ipBytes := make([]byte, 16)
		copy(ipBytes[:8], baseHi)
		binary.BigEndian.PutUint64(ipBytes[8:], baseLo+counter)
		prefix := fmt.Sprintf("%s/128", net.IP(ipBytes).String())

		entries = append(entries,
			fluent.IPv6Entry().
				WithNetworkInstance(vrf).
				WithPrefix(prefix).
				WithNextHopGroup(nhgID).
				WithNextHopGroupNetworkInstance(defaultNI),
		)
	}

	return entries, labelList
}

// buildProfile5Ops generates ADD/DELETE ops mix for Profile 5.
func buildProfile5Ops(t *testing.T, dut *ondatra.DUTDevice, totalPrefixes int, nhgBase uint64, baseIPv4, baseIPv6 string) (adds, dels []fluent.GRIBIEntry) {
	t.Helper()
	ipv6s := buildIPv6Routes(t, dut, totalPrefixes/2, baseIPv6, nhgBase)
	all := append(ipv6s)

	for i, e := range all {
		if i%2 == 0 {
			// ADD this entry
			adds = append(adds, e)
		} else {
			// DELETE version: rebuild with same prefix & NHG, but
			// you don’t mark operation here — caller will use DeleteEntries()
			nhgID := nhgBase + uint64(i%2)
			dels = append(dels,
				fluent.IPv6Entry().
					WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
					WithPrefix(baseIPv6+"/128").
					WithNextHopGroup(nhgID).
					WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut)),
			)

		}
	}
	return adds, dels
}

// pumpOpsAtRate sends ops to gRIBI client at target ops/sec
func pumpOpsAtRate(t *testing.T, ctx context.Context, c *gribi.Client, ops []fluent.GRIBIEntry, targetOps int) {
	t.Helper()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	batchSize := 10
	for start := 0; start < len(ops); start += batchSize {
		end := start + batchSize
		if end > len(ops) {
			end = len(ops)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.AddEntries(t, ops[start:end], nil) // or ModifyEntries if supported
		}
	}
}

// buildIPv4Routes generates num IPv4 /32 routes starting from baseIPv4 mapped to NHGs.
func buildIPv4Routes(t *testing.T, dut *ondatra.DUTDevice, num int, baseIPv4 string, nhgBase uint64) []fluent.GRIBIEntry {
	t.Helper()
	var entries []fluent.GRIBIEntry
	ip := net.ParseIP(baseIPv4).To4()
	ipInt := big.NewInt(0).SetBytes(ip)

	for i := 0; i < num; i++ {
		ipBytes := ipInt.FillBytes(make([]byte, 4))
		prefix := fmt.Sprintf("%s/32", net.IP(ipBytes).String())
		ipInt.Add(ipInt, big.NewInt(1))

		nhgID := nhgBase + uint64(i%num) // cycle across NHGs
		entries = append(entries,
			fluent.IPv4Entry().
				WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithPrefix(prefix).
				WithNextHopGroup(nhgID).
				WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut)),
		)
	}
	return entries
}

// buildIPv6Routes generates num IPv6 /128 routes starting from baseIPv6 mapped to NHGs.
func buildIPv6Routes(t *testing.T, dut *ondatra.DUTDevice, num int, baseIPv6 string, nhgBase uint64) []fluent.GRIBIEntry {
	t.Helper()
	var entries []fluent.GRIBIEntry
	ip := net.ParseIP(baseIPv6).To16()
	ipInt := big.NewInt(0).SetBytes(ip)

	for i := 0; i < num; i++ {
		ipBytes := ipInt.FillBytes(make([]byte, 16))
		prefix := fmt.Sprintf("%s/128", net.IP(ipBytes).String())
		ipInt.Add(ipInt, big.NewInt(1))

		nhgID := nhgBase + uint64(i%num) // cycle across NHGs
		entries = append(entries,
			fluent.IPv6Entry().
				WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithPrefix(prefix).
				WithNextHopGroup(nhgID).
				WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut)),
		)
	}
	return entries
}

// expectedMPLSinUDPOpResults builds expected gRIBI OpResults for Profile 5.
// It models the baseline state of one MPLS-in-UDP NH, all NHGs, and
// 20k route entries (10k IPv4 + 10k IPv6).
func expectedMPLSinUDPOpResults(
	t *testing.T,
	mplsNHID uint64,
	numNHGs int,
	nhgBase uint64,
	totalPrefixes int,
	baseIPv4, baseIPv6 string,
) (adds, dels []*client.OpResult) {
	t.Helper()
	adds = []*client.OpResult{}
	dels = []*client.OpResult{}

	// === Step 1: One NH ===
	adds = append(adds,
		fluent.OperationResult().
			WithNextHopOperation(mplsNHID).
			WithProgrammingResult(fluent.InstalledInFIB).
			WithOperationType(constants.Add).
			AsResult(),
	)
	dels = append(dels,
		fluent.OperationResult().
			WithNextHopOperation(mplsNHID).
			WithProgrammingResult(fluent.InstalledInFIB).
			WithOperationType(constants.Delete).
			AsResult(),
	)

	// === Step 2: NHGs ===
	for i := 0; i < numNHGs; i++ {
		nhgID := nhgBase + uint64(i)
		adds = append(adds,
			fluent.OperationResult().
				WithNextHopGroupOperation(nhgID).
				WithProgrammingResult(fluent.InstalledInFIB).
				WithOperationType(constants.Add).
				AsResult(),
		)
		dels = append(dels,
			fluent.OperationResult().
				WithNextHopGroupOperation(nhgID).
				WithProgrammingResult(fluent.InstalledInFIB).
				WithOperationType(constants.Delete).
				AsResult(),
		)
	}

	// === Step 3: IPv4 routes (/32) ===
	numIPv4 := totalPrefixes / 2
	ip4 := net.ParseIP(baseIPv4).To4()
	ip4Int := big.NewInt(0).SetBytes(ip4)

	for i := 0; i < numIPv4; i++ {
		ipBytes := ip4Int.FillBytes(make([]byte, 4))
		prefix := fmt.Sprintf("%s/32", net.IP(ipBytes).String())
		ip4Int.Add(ip4Int, big.NewInt(1))

		adds = append(adds,
			fluent.OperationResult().
				WithIPv4Operation(prefix).
				WithProgrammingResult(fluent.InstalledInFIB).
				WithOperationType(constants.Add).
				AsResult(),
		)
		dels = append(dels,
			fluent.OperationResult().
				WithIPv4Operation(prefix).
				WithProgrammingResult(fluent.InstalledInFIB).
				WithOperationType(constants.Delete).
				AsResult(),
		)
	}

	// === Step 4: IPv6 routes (/128) ===
	numIPv6 := totalPrefixes / 2
	ip6 := net.ParseIP(baseIPv6).To16()
	ip6Int := big.NewInt(0).SetBytes(ip6)

	for i := 0; i < numIPv6; i++ {
		ipBytes := ip6Int.FillBytes(make([]byte, 16))
		prefix := fmt.Sprintf("%s/128", net.IP(ipBytes).String())
		ip6Int.Add(ip6Int, big.NewInt(1))

		adds = append(adds,
			fluent.OperationResult().
				WithIPv6Operation(prefix).
				WithProgrammingResult(fluent.InstalledInFIB).
				WithOperationType(constants.Add).
				AsResult(),
		)
		dels = append(dels,
			fluent.OperationResult().
				WithIPv6Operation(prefix).
				WithProgrammingResult(fluent.InstalledInFIB).
				WithOperationType(constants.Delete).
				AsResult(),
		)
	}

	return adds, dels
}

func expectedMPLSinUDPMultiOpResults(
	t *testing.T,
	vrfs []string,
	mplsNHBase uint64,
	numNHGs int,
	nhgBase uint64,
) (adds, dels []*client.OpResult) {
	t.Helper()

	// Preallocate: NH + NHGs + Routes
	totalOps := len(vrfs) * (1 + numNHGs + numNHGs)
	adds = make([]*client.OpResult, 0, totalOps)
	dels = make([]*client.OpResult, 0, totalOps)

	// === Step 1: NHs ===
	for vrfIdx := range vrfs {
		nhID := mplsNHBase + uint64(vrfIdx)

		adds = append(adds,
			fluent.OperationResult().
				WithNextHopOperation(nhID).
				WithProgrammingResult(fluent.InstalledInFIB).
				WithOperationType(constants.Add).
				AsResult(),
		)
		dels = append(dels,
			fluent.OperationResult().
				WithNextHopOperation(nhID).
				WithProgrammingResult(fluent.InstalledInFIB).
				WithOperationType(constants.Delete).
				AsResult(),
		)
	}

	// === Step 2 & 3: NHGs and Routes ===
	for vrfIdx := range vrfs {
		for i := 0; i < numNHGs; i++ {
			nhgID := nhgBase + uint64(vrfIdx*numNHGs+i)

			adds = append(adds,
				fluent.OperationResult().
					WithNextHopGroupOperation(nhgID).
					WithProgrammingResult(fluent.InstalledInFIB).
					WithOperationType(constants.Add).
					AsResult(),
			)
			dels = append(dels,
				fluent.OperationResult().
					WithNextHopGroupOperation(nhgID).
					WithProgrammingResult(fluent.InstalledInFIB).
					WithOperationType(constants.Delete).
					AsResult(),
			)

			// Prefix generation (shared helper)
			offset := int64(vrfIdx*numNHGs + i)
			prefix := generateIPv6Prefix(baseIPv6, offset)

			adds = append(adds,
				fluent.OperationResult().
					WithIPv6Operation(prefix).
					WithProgrammingResult(fluent.InstalledInFIB).
					WithOperationType(constants.Add).
					AsResult(),
			)
			dels = append(dels,
				fluent.OperationResult().
					WithIPv6Operation(prefix).
					WithProgrammingResult(fluent.InstalledInFIB).
					WithOperationType(constants.Delete).
					AsResult(),
			)
		}
	}

	return adds, dels
}

func generateIPv6Prefix(baseIPv6 string, offset int64) string {
	baseIP := net.ParseIP(baseIPv6).To16()
	baseInt := big.NewInt(0).SetBytes(baseIP)
	ipInt := big.NewInt(0).Add(baseInt, big.NewInt(offset))
	ipBytes := ipInt.FillBytes(make([]byte, 16))
	return net.IP(ipBytes).String() + "/128"
}

// validateTrafficFlows verifies traffic flow behavior (pass/fail) based on expected outcome
func validateTrafficFlows(t *testing.T, ate *ondatra.ATEDevice, ateConfig gosnappi.Config, args *testArgs, flows []gosnappi.Flow, capture bool, match bool) {
	t.Helper()
	t.Logf("=== TRAFFIC FLOW VALIDATION START (expecting match=%v) ===", match)

	otg := args.ate.OTG()
	sendTraffic(t, args, flows, capture)

	otgutils.LogPortMetrics(t, otg, args.topo)
	otgutils.LogFlowMetrics(t, otg, args.topo)

	for _, flow := range flows {
		outPkts := float32(gnmi.Get(t, otg, gnmi.OTG().Flow(flow.Name()).Counters().OutPkts().State()))
		inPkts := float32(gnmi.Get(t, otg, gnmi.OTG().Flow(flow.Name()).Counters().InPkts().State()))
		lossPct := ((outPkts - inPkts) * 100) / outPkts

		t.Logf("Flow %s: OutPkts=%v, InPkts=%v, LossPct=%v", flow.Name(), outPkts, inPkts, lossPct)

		if outPkts == 0 {
			t.Fatalf("OutPkts for flow %s is 0, want > 0", flow.Name())
		}

		if match {
			// Expecting traffic to pass (0% loss)
			if got := lossPct; got > 0 {
				t.Fatalf("Traffic validation FAILED: Flow %s has %v%% packet loss, want 0%%", flow.Name(), got)
			} else {
				t.Logf("Traffic validation PASSED: Flow %s has 0%% packet loss", flow.Name())
			}
		} else {
			// Expecting traffic to fail (100% loss)
			if got := lossPct; got != 100 {
				t.Fatalf("Traffic validation FAILED: Flow %s has %v%% packet loss, want 100%%", flow.Name(), got)
			} else {
				t.Logf("Traffic validation PASSED: Flow %s has 100%% packet loss", flow.Name())
			}
		}
		if match {
			rxPorts := []string{ateConfig.Ports().Items()[2].Name(), ateConfig.Ports().Items()[3].Name()}
			weights := testLoadBalance(t, ate, rxPorts, flow)
			loadBalVal := true
			for idx, weight := range portsTrafficDistribution {
				if got, want := weights[idx], weight; got < (want-tolerance) || got > (want+tolerance) {
					t.Errorf("ECMP Percentage for Aggregate Index: %d: got %d, want %d", idx+1, got, want)
					loadBalVal = false
				}
			}
			if loadBalVal {
				t.Log("Load balancing has been verified on the Port interfaces.")
			}
		}
	}
}

// sendTraffic sends traffic flows for the specified duration
func sendTraffic(t *testing.T, args *testArgs, flows []gosnappi.Flow, capture bool) {
	t.Helper()
	otg := args.ate.OTG()
	args.topo.Flows().Clear().Items()
	args.topo.Flows().Append(flows...)

	otg.PushConfig(t, args.topo)
	otg.StartProtocols(t)

	otgutils.WaitForARP(t, args.ate.OTG(), args.topo, "IPv4")
	otgutils.WaitForARP(t, args.ate.OTG(), args.topo, "IPv6")

	if capture {
		startCapture(t, args.ate)
		defer stopCapture(t, args.ate)
	}

	otg.StartTraffic(t)
	time.Sleep(trafficDuration)
	otg.StopTraffic(t)
}

// getFlow creates a traffic flow for MPLS-in-UDP testing
func (fa *flowAttr) getFlow(flowType string, name string, dscp uint32) gosnappi.Flow {
	flow := fa.topo.Flows().Add().SetName(name)
	flow.Metrics().SetEnable(true)
	flow.Rate().SetPps(ratePps)
	flow.Size().SetFixed(pktSize)
	flow.TxRx().Port().SetTxName(fa.srcPort).SetRxNames(fa.dstPorts)
	e1 := flow.Packet().Add().Ethernet()
	e1.Src().SetValue(fa.srcMac)
	e1.Dst().SetValue(fa.dstMac)

	// For MPLS-in-UDP testing, we only support IPv6 flows
	if flowType == "ipv6" {
		v6 := flow.Packet().Add().Ipv6()
		v6.Src().SetValue(fa.src)
		v6.Dst().Increment().SetStart(fa.dst).SetCount(ipCount)
		v6.HopLimit().SetValue(ttl)
		v6.TrafficClass().SetValue(dscp << 2)
	} else {
		v4 := flow.Packet().Add().Ipv4()
		v4.Src().SetValue(fa.src)
		v4.Dst().Increment().SetStart(fa.dst).SetCount(ipCount)
		v4.TimeToLive().SetValue(ttl)
		v4.Priority().Dscp().Phb().SetValue(dscp)
	}

	// Add UDP payload to generate traffic
	udp := flow.Packet().Add().Udp()
	// udp.SrcPort().SetValues(randRange(50001, 10000))
	// udp.DstPort().SetValues(randRange(50001, 10000))
	udp.SrcPort().SetValues(randRange(5555, 10000))
	udp.DstPort().SetValues(randRange(5555, 10000))

	return flow
}

func randRange(max int, count int) []uint32 {
	rand.New(rand.NewSource(time.Now().UnixNano()))
	var result []uint32
	for len(result) < count {
		result = append(result, uint32(rand.Intn(max)))
	}
	return result
}

// enableCapture enables packet capture on specified OTG ports
func enableCapture(t *testing.T, otg *otg.OTG, topo gosnappi.Config, otgPortNames []string) {
	t.Helper()
	for _, port := range otgPortNames {
		topo.Captures().Add().SetName(port).SetPortNames([]string{port}).SetFormat(gosnappi.CaptureFormat.PCAP)
	}
	otg.PushConfig(t, topo)
}

// clearCapture clears packet capture from all OTG ports
func clearCapture(t *testing.T, otg *otg.OTG, topo gosnappi.Config) {
	t.Helper()
	topo.Captures().Clear()
	otg.PushConfig(t, topo)
}

// startCapture starts packet capture on OTG ports
func startCapture(t *testing.T, ate *ondatra.ATEDevice) {
	t.Helper()
	otg := ate.OTG()
	cs := gosnappi.NewControlState()
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.START)
	otg.SetControlState(t, cs)
}

// stopCapture stops packet capture on OTG ports
func stopCapture(t *testing.T, ate *ondatra.ATEDevice) {
	t.Helper()
	otg := ate.OTG()
	cs := gosnappi.NewControlState()
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.STOP)
	otg.SetControlState(t, cs)
}

// configStaticArp configures static ARP entries for gRIBI next hop resolution
func configStaticArp(p string, ipv4addr string, macAddr string) *oc.Interface {
	i := &oc.Interface{Name: ygot.String(p)}
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	s := i.GetOrCreateSubinterface(0)
	s4 := s.GetOrCreateIpv4()
	n4 := s4.GetOrCreateNeighbor(ipv4addr)
	n4.LinkLayerAddress = ygot.String(macAddr)
	return i
}

// staticARPWithSecondaryIP configures secondary IPs and static ARP for gRIBI compatibility
func staticARPWithSecondaryIP(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	p3 := dut.Port(t, "port3")
	gnmi.Update(t, dut, gnmi.OC().Interface(p3.Name()).Config(), dutPort3DummyIP.NewOCInterface(p3.Name(), dut))
	gnmi.Update(t, dut, gnmi.OC().Interface(p3.Name()).Config(), configStaticArp(p3.Name(), otgPort3DummyIP.IPv4, magicMac))

	p4 := dut.Port(t, "port4")
	gnmi.Update(t, dut, gnmi.OC().Interface(p4.Name()).Config(), dutPort4DummyIP.NewOCInterface(p4.Name(), dut))
	gnmi.Update(t, dut, gnmi.OC().Interface(p4.Name()).Config(), configStaticArp(p4.Name(), otgPort4DummyIP.IPv4, magicMac))
}

// testLoadBalance to ensure 50:50 Load Balancing
func testLoadBalance(t *testing.T, ate *ondatra.ATEDevice, portNames []string, flow gosnappi.Flow) []uint64 {
	t.Helper()
	var rxs []uint64
	for _, aggName := range portNames {
		metrics := gnmi.Get(t, ate.OTG(), gnmi.OTG().Port(aggName).State())
		rxs = append(rxs, (metrics.GetCounters().GetInFrames()))
	}
	var total uint64
	for _, rx := range rxs {
		total += rx
	}
	for idx, rx := range rxs {
		rxs[idx] = (rx * 100) / total
	}
	return rxs
}

// incrementMAC increments the MAC by i. Returns error if the mac cannot be parsed or overflows the mac address space
func incrementMAC(mac string, i int) (string, error) {
	macAddr, err := net.ParseMAC(mac)
	if err != nil {
		return "", err
	}
	convMac := binary.BigEndian.Uint64(append([]byte{0, 0}, macAddr...))
	convMac = convMac + uint64(i)
	buf := new(bytes.Buffer)
	err = binary.Write(buf, binary.BigEndian, convMac)
	if err != nil {
		return "", err
	}
	newMac := net.HardwareAddr(buf.Bytes()[2:8])
	return newMac.String(), nil
}

// buildVRFIndexMap produces a stable index for each VRF name based on a canonical order.
// canonicalOrder should be the list that determines label/nhID assignment (e.g. ["default","VRF_001",...])
func buildVRFIndexMap(canonicalOrder []string) map[string]int {
	m := make(map[string]int, len(canonicalOrder))
	for i, v := range canonicalOrder {
		m[v] = i
	}
	return m
}

// formatMPLSHeader formats MPLS header bytes for debugging output
func formatMPLSHeader(data []byte) string {
	if len(data) < 4 {
		return "Invalid MPLS header: too short"
	}

	headerValue := binary.BigEndian.Uint32(data[:4])
	label := (headerValue >> 12) & 0xFFFFF
	exp := uint8((headerValue >> 9) & 0x07)
	s := (headerValue >> 8) & 0x01
	ttl := uint8(headerValue & 0xFF)

	return fmt.Sprintf("MPLS Label: %d, EXP: %d, BoS: %t, TTL: %d", label, exp, s == 1, ttl)
}

// generateSkewPattern generates a skewed distribution of prefixes per VRF.
// The returned slice indicates how many prefixes each VRF should have.
func generateSkewPattern(numVRFs, totalPrefixes int) []int {
	rand.Seed(time.Now().UnixNano())

	weights := make([]int, numVRFs)
	sum := 0
	for i := 0; i < numVRFs; i++ {
		weights[i] = rand.Intn(30) + 1 // random weight between 1 and 30
		sum += weights[i]
	}

	skew := make([]int, numVRFs)
	accum := 0
	for i := 0; i < numVRFs; i++ {
		skew[i] = weights[i] * totalPrefixes / sum
		accum += skew[i]
	}

	// Adjust last element so the sum matches exactly
	skew[numVRFs-1] += totalPrefixes - accum
	return skew
}

// generateUniqueVRFs returns the VRFs list unchanged but can shuffle if needed.
// Each VRF is unique to ensure traffic works correctly.
func generateUniqueVRFs(vrfs []string) []string {
	vrfList := make([]string, len(vrfs))
	copy(vrfList, vrfs)

	// optional: shuffle for random order
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(vrfList), func(i, j int) {
		vrfList[i], vrfList[j] = vrfList[j], vrfList[i]
	})

	return vrfList
}

// generateSkewedVRFList creates a skewed VRF assignment list.
// totalPrefixes = total number of prefixes (e.g. 20000)
// vrfs = list of VRF names
// skewPattern = list of prefix counts per VRF (len must == len(vrfs))
func generateSkewedVRFList(t *testing.T, vrfs []string, totalPrefixes int, skewPattern []int) []string {
	t.Helper()
	// sanity check
	sum := 0
	for _, c := range skewPattern {
		sum += c
	}
	if sum != totalPrefixes+1 {
		t.Errorf("skewPattern sum=%d but expected totalPrefixes=%d", sum, totalPrefixes+1)
	}

	vrfList := []string{}
	for i, vrf := range vrfs {
		count := skewPattern[i]
		for j := 0; j < count; j++ {
			vrfList = append(vrfList, vrf)
		}
	}

	// shuffle so distribution looks random
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(vrfList), func(i, j int) {
		vrfList[i], vrfList[j] = vrfList[j], vrfList[i]
	})

	return vrfList
}