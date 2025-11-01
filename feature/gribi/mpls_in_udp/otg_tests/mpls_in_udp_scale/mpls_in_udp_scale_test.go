package mpls_in_udp_scale_test

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"math/rand"
	"net"
	"os"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/featureprofiles/internal/iputil"
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
	portSpeed       = oc.IfEthernet_ETHERNET_SPEED_SPEED_100GB
	startVLANPort1  = 100
	startVLANPort2  = 200
	numVRFs         = 10
	vlanCount       = 16
	trafficDuration = 15 * time.Second
	nextHopID       = uint64(1001)
	nextHopGroupID  = uint64(2001)
	outerIPv6Src    = "2001:db8:300::1"
	outerIPv6Dst    = "3001:db8:300::1"
	tunnelDst       = "3001:db8:300::%d%s"
	outerDstUDPPort = 6635
	outerDSCP       = 26
	outerTTL        = 64
	innerTTL        = uint32(100)
	mplsLabel       = 100
	ipv4PrefixLen   = 30
	ipv6PrefixLen   = 126
	routeV4PrfLen   = "/32"
	routeV6PrfLen   = "/128"
	tolerance       = 5
	ratePPS         = 10000
	pktSize         = 256
	mtu             = 9126
	policyName      = "redirect-to-vrf_t"
	ipv4AddrPfx1    = "192.51.100.%d"
	ipv4AddrPfx2    = "193.51.100.%d"
	ipv6AddrPfx1    = "4001:db8:1:5::%d"
	ipv6AddrPfx2    = "5001:db8:1:6::%d"
	pbfIPv6         = "6001:db8:1:5::%d/128"
	dstIP           = "192.168.1.1"
	dstMac          = "02:00:00:00:00:01"
	incCount        = 10
	baseIPv6        = "3001:db8:100::" // starting prefix for IPv6
	baseIPv4        = "198.51.100.0"   // starting prefix for IPv4
)

// DUT and ATE port attributes
var (
	dutPort1 = attrs.Attributes{Desc: "DUT Ingress Port 1", MAC: "02:01:01:00:00:01", IPv4: "192.51.100.1", IPv4Len: ipv4PrefixLen, IPv6: "2001:db8:1::1", IPv6Len: ipv6PrefixLen}
	dutPort2 = attrs.Attributes{Desc: "DUT Ingress Port 2", MAC: "02:03:03:00:00:01", IPv4: "193.51.100.1", IPv4Len: ipv4PrefixLen, IPv6: "2002:db8:1::1", IPv6Len: ipv6PrefixLen}
	dutPort3 = attrs.Attributes{Desc: "DUT Egress Port 3", MAC: "02:05:05:00:00:01", IPv4: "194.51.100.1", IPv4Len: ipv4PrefixLen, IPv6: "2003:db8:1::1", IPv6Len: ipv6PrefixLen, MTU: mtu}
	dutPort4 = attrs.Attributes{Desc: "DUT Egress Port 4", MAC: "02:07:07:00:00:01", IPv4: "195.51.100.1", IPv4Len: ipv4PrefixLen, IPv6: "2004:db8:1::1", IPv6Len: ipv6PrefixLen, MTU: mtu}

	atePort1 = attrs.Attributes{Name: "ATE-Ingress-Port-1", MAC: "02:02:02:00:00:01", IPv4: "192.51.100.2", IPv4Len: ipv4PrefixLen, IPv6: "2001:db8:1::2", IPv6Len: ipv6PrefixLen}
	atePort2 = attrs.Attributes{Name: "ATE-Ingress-Port-2", MAC: "02:04:04:00:00:01", IPv4: "193.51.100.2", IPv4Len: ipv4PrefixLen, IPv6: "2002:db8:1::2", IPv6Len: ipv6PrefixLen}
	atePort3 = attrs.Attributes{Name: "ATE-Egress-Port-3", MAC: "02:06:06:00:00:01", IPv4: "194.51.100.2", IPv4Len: ipv4PrefixLen, IPv6: "2003:db8:1::2", IPv6Len: ipv6PrefixLen, MTU: mtu}
	atePort4 = attrs.Attributes{Name: "ATE-Egress-Port-4", MAC: "02:08:08:00:00:01", IPv4: "195.51.100.2", IPv4Len: ipv4PrefixLen, IPv6: "2004:db8:1::2", IPv6Len: ipv6PrefixLen, MTU: mtu}

	dutPort3DummyIP = attrs.Attributes{Desc: "dutPort3", IPv4Sec: "192.0.2.21", IPv4LenSec: ipv4PrefixLen}

	otgPort3DummyIP = attrs.Attributes{Desc: "otgPort3", IPv4: "192.0.2.22", IPv4Len: ipv4PrefixLen}

	dutPort4DummyIP = attrs.Attributes{Desc: "dutPort4", IPv4Sec: "193.0.2.21", IPv4LenSec: ipv4PrefixLen}

	otgPort4DummyIP = attrs.Attributes{Desc: "otgPort4", IPv4: "193.0.2.22", IPv4Len: ipv4PrefixLen}
	// IPv6 flow configuration for MPLS-in-UDP testing
	fa6 = flowAttr{
		src:      atePort1.IPv6,
		dst:      baseIPv6, // IPv6 prefix for inner destination
		srcMac:   atePort1.MAC,
		dstMac:   dutPort1.MAC,
		srcPort:  "port1",
		dstPorts: []string{"port3", "port4"},
		topo:     gosnappi.NewConfig(),
	}
	fa4 = flowAttr{
		src:      atePort1.IPv4,
		dst:      baseIPv4, // IPv4 prefix for inner destination
		srcMac:   atePort1.MAC,
		dstMac:   dutPort1.MAC,
		srcPort:  "port1",
		dstPorts: []string{"port3", "port4"},
		topo:     gosnappi.NewConfig(),
	}
	portsTrafficDistribution = []uint64{50, 50}
	profiles                 = map[vrfProfile]profileConfig{
		profileSingleVRF:      {20, 1}, // 20k NHGs × 1 NH
		profileMultiVRF:       {20, 1}, // same as profile 1
		profileMultiVRFSkew:   {20, 1}, // same as profile 1
		profileSingleVRFECMP:  {20, 8}, // 2.5k NHGs × 8 NHs = 20k NHs
		profileSingleVRFgRIBI: {20, 1}, // QPS scaling
	}
	totalPrefixes = 20
)

type profileConfig struct {
	totalNHGs int
	nhsPerNHG int
}

// flowAttr defines traffic flow attributes for test packets.
type flowAttr struct {
	src      string   // source IP address
	dst      string   // destination IP address
	srcPort  string   // source OTG port
	dstPorts []string // destination OTG ports
	srcMac   string   // source MAC address
	dstMac   string   // destination MAC address
	topo     gosnappi.Config
}

// testArgs holds the objects needed by a test case.
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
	wantOuterTTL        uint8
}

type vrfProfile int

const (
	profileUnknown vrfProfile = iota
	profileSingleVRF
	profileMultiVRF
	profileMultiVRFSkew
	profileSingleVRFECMP
	profileSingleVRFgRIBI
)

// configureDUT configures all ports with base IPs and subinterfaces with VRF and VLANs.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) []string {
	t.Helper()
	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")
	dp3 := dut.Port(t, "port3")
	dp4 := dut.Port(t, "port4")
	d := gnmi.OC()

	vrfBatch := new(gnmi.SetBatch)
	configureHardwareInit(t, dut)
	// Create VRFs + PBF (true enables policy-based forwarding rules)
	vrfsList := cfgplugins.CreateVRFs(t, dut, vrfBatch, cfgplugins.VRFConfig{VRFCount: numVRFs, EnablePBF: true, VRFPolicyName: policyName, VRFIPv6: pbfIPv6})
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
		gnmi.BatchUpdate(vrfBatch, d.Interface(p.Name()).Config(), intf)
		t.Logf("Configured DUT port %s (%s)", p.Name(), a.Desc)
	}

	fptest.ConfigureDefaultNetworkInstance(t, dut)
	// configure 16 L3 subinterfaces under DUT port#1 & 2 and assign them to DEFAULT vrf
	configureDUTSubinterfaces(t, vrfBatch, new(oc.Root), dut, dp1, ipv4AddrPfx1, ipv6AddrPfx1, startVLANPort1)
	configureDUTSubinterfaces(t, vrfBatch, new(oc.Root), dut, dp2, ipv4AddrPfx2, ipv6AddrPfx2, startVLANPort2)

	// configure an L3 subinterface without vlan tagging under DUT port#3 & 4
	createDUTSubinterface(t, vrfBatch, new(oc.Root), dut, dp3, 0, 0, dutPort3.IPv4, dutPort3.IPv6)
	createDUTSubinterface(t, vrfBatch, new(oc.Root), dut, dp4, 0, 0, dutPort4.IPv4, dutPort4.IPv6)

	cfgplugins.VRFPolicy(t, vrfBatch, cfgplugins.VRFPolicyConfig{IngressPort: dp1.Name(), PolicyName: policyName})
	cfgplugins.VRFPolicy(t, vrfBatch, cfgplugins.VRFPolicyConfig{IngressPort: dp2.Name(), PolicyName: policyName})
	vrfBatch.Set(t, dut)
	// Set static ARP for gRIBI NH MAC resolution
	switch {
	case deviations.GRIBIMACOverrideWithStaticARP(dut):
		staticARPWithSecondaryIP(t, dut)
	case deviations.GRIBIMACOverrideStaticARPStaticRoute(dut):
		staticARPWithUniversalIP(t, dut, vrfsList, []string{"port3", "port4"}, dstIP, routeV4PrfLen, dstMac, "ipv4", 0)
	}
	return vrfsList
}

// createDUTSubinterface creates a single subinterface on the DUT port with optional VLAN, IPv4, and IPv6 configuration, and stages it into the provided GNMI SetBatch.
func createDUTSubinterface(t *testing.T, vrfBatch *gnmi.SetBatch, d *oc.Root, dut *ondatra.DUTDevice, dutPort *ondatra.Port, index uint32, vlanID uint16, ipv4Addr, ipv6Addr string) {
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
	gnmi.BatchUpdate(vrfBatch, gnmi.OC().Interface(ifName).Subinterface(index).Config(), s)
}

// configureDUTSubinterfaces creates and configures multiple subinterfaces (up to 16) on the given DUT port. Each subinterface is assigned a VLAN ID, IPv4, and IPv6 address, and all configurations are staged into the provided GNMI SetBatch.
func configureDUTSubinterfaces(t *testing.T, vrfBatch *gnmi.SetBatch, d *oc.Root, dut *ondatra.DUTDevice, dutPort *ondatra.Port, prefixFmtV4, prefixFmtV6 string, startVLANPort int) {
	t.Helper()
	// The 32 logical ingress interfaces (16 VLANs × 2 ports) are mapped to VRFs as per scale profiles for traffic classification and forwarding validation.
	// Each VLAN subinterface is configured with both IPv4 and IPv6 addresses derived from prefixFmtV4 and prefixFmtV6 patterns.
	for i := 0; i < vlanCount; i++ {
		index := uint32(i + 1)
		vlanID := uint16(startVLANPort + i)
		if deviations.NoMixOfTaggedAndUntaggedSubinterfaces(dut) {
			vlanID++
		}
		dutIPv4 := fmt.Sprintf(prefixFmtV4, (4*index)+2)
		dutIPv6 := fmt.Sprintf(prefixFmtV6, (5*index)+2)
		createDUTSubinterface(t, vrfBatch, d, dut, dutPort, index, vlanID, dutIPv4, dutIPv6)
		cfgplugins.VRFPolicy(t, vrfBatch, cfgplugins.VRFPolicyConfig{IngressPort: fmt.Sprintf("%s.%d", dutPort.Name(), index), PolicyName: policyName})
	}
}

// configureHardwareInit sets up the initial hardware configuration on the DUT. It pushes hardware initialization configs for VRF Selection Extended feature and Policy Forwarding feature.
func configureHardwareInit(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	hardwareVrfCfg := cfgplugins.NewDUTHardwareInit(t, dut, cfgplugins.FeatureVrfSelectionExtended)
	hardwarePfCfg := cfgplugins.NewDUTHardwareInit(t, dut, cfgplugins.FeaturePolicyForwarding)
	if hardwareVrfCfg == "" || hardwarePfCfg == "" {
		return
	}
	cfgplugins.PushDUTHardwareInitConfig(t, dut, hardwareVrfCfg)
	cfgplugins.PushDUTHardwareInitConfig(t, dut, hardwarePfCfg)
}

// createATEDevice creates a single ATE device with Ethernet, VLAN, IPv4, and IPv6 configuration, and attaches it to the given ATE port.
func createATEDevice(t *testing.T, ateConfig gosnappi.Config, atePort *ondatra.Port, vlanID uint16, Name, MAC, dutIPv4, ateIPv4, dutIPv6, ateIPv6 string) {
	t.Helper()
	dev := ateConfig.Devices().Add().SetName(Name + ".Dev")
	eth := dev.Ethernets().Add().SetName(Name + ".Eth").SetMac(MAC)
	eth.Connection().SetPortName(atePort.ID())
	eth.Vlans().Add().SetName(Name).SetId(uint32(vlanID))
	eth.Ipv4Addresses().Add().SetName(Name + ".IPv4").SetAddress(ateIPv4).SetGateway(dutIPv4).SetPrefix(uint32(ipv4PrefixLen))
	eth.Ipv6Addresses().Add().SetName(Name + ".IPv6").SetAddress(ateIPv6).SetGateway(dutIPv6).SetPrefix(uint32(ipv6PrefixLen))
}

// mustConfigureATESubinterfaces configures 16 ATE subinterfaces on the target device It returns a slice of the corresponding ATE IPAddresses.
func mustConfigureATESubinterfaces(t *testing.T, ateConfig gosnappi.Config, atePort *ondatra.Port, dut *ondatra.DUTDevice, Name, Mac, prefixFmtV4, prefixFmtV6 string, startVLANPort int) {
	t.Helper()
	for i := range vlanCount {
		vlanID := uint16(startVLANPort + i)
		if deviations.NoMixOfTaggedAndUntaggedSubinterfaces(dut) {
			vlanID++
		}
		dutIPv4 := fmt.Sprintf(prefixFmtV4, (4*(i+1))+2)
		ateIPv4 := fmt.Sprintf(prefixFmtV4, (4*(i+1))+1)
		dutIPv6 := fmt.Sprintf(prefixFmtV6, (5*(i+1))+2)
		ateIPv6 := fmt.Sprintf(prefixFmtV6, (5*(i+1))+1)
		name := fmt.Sprintf("%s%d", Name, i)
		mac, err := iputil.IncrementMAC(Mac, i+1)
		if err != nil {
			t.Errorf("%s", err)
		}
		createATEDevice(t, ateConfig, atePort, vlanID, name, mac, dutIPv4, ateIPv4, dutIPv6, ateIPv6)
	}
}

// configureOTG sets up the ATE topology across 4 physical ports, including VLAN subinterfaces, IP addressing, and device-level configs. It also applies Layer1 link settings for 100GBASE-LR4 PMD ports with auto-negotiation and disables RS-FEC if required.
func configureOTG(t *testing.T, ate *ondatra.ATEDevice, dut *ondatra.DUTDevice) gosnappi.Config {
	t.Helper()
	ateConfig := gosnappi.NewConfig()
	ap1 := ate.Port(t, "port1")
	ap2 := ate.Port(t, "port2")
	ap3 := ate.Port(t, "port3")
	ap4 := ate.Port(t, "port4")

	ateConfig.Ports().Add().SetName(ap1.ID())
	ateConfig.Ports().Add().SetName(ap2.ID())
	ateConfig.Ports().Add().SetName(ap3.ID())
	ateConfig.Ports().Add().SetName(ap4.ID())

	createATEDevice(t, ateConfig, ap3, 0, atePort3.Name, atePort3.MAC, dutPort3.IPv4, atePort3.IPv4, dutPort3.IPv6, atePort3.IPv6)
	createATEDevice(t, ateConfig, ap4, 0, atePort4.Name, atePort4.MAC, dutPort4.IPv4, atePort4.IPv4, dutPort4.IPv6, atePort4.IPv6)
	// subIntfIPs is a []string slice with ATE IPv6 addresses for all the subInterfaces
	mustConfigureATESubinterfaces(t, ateConfig, ap1, dut, atePort1.Name, atePort1.MAC, ipv4AddrPfx1, ipv6AddrPfx1, startVLANPort1)
	mustConfigureATESubinterfaces(t, ateConfig, ap2, dut, atePort2.Name, atePort2.MAC, ipv4AddrPfx2, ipv6AddrPfx2, startVLANPort2)
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

// programBasicEntries installs basic NextHop and NextHopGroup entries to set up ECMP forwarding across port3 and port4, along with an IPv6 route to test MPLS-in-UDP tunnels.
func programBasicEntries(t *testing.T, dut *ondatra.DUTDevice, c *gribi.Client, vrfs []string) {
	t.Helper()
	t.Log("Setting up routing infrastructure for MPLS-in-UDP with unique NH IDs and NH/NHG in default NI")

	// base IDs (choose ranges that won't collide with other tests)
	nhBase := uint64(300)  // will be offset per VRF & per port
	nhgBase := uint64(400) // will be offset per VRF

	otgDummyIPs := map[string]string{
		dut.Port(t, "port3").Name(): otgPort3DummyIP.IPv4,
		dut.Port(t, "port4").Name(): otgPort4DummyIP.IPv4,
	}

	defaultNI := deviations.DefaultNetworkInstance(dut)

	// iterate VRFs and program unique NH, NHG in default NI; program IPv6 prefix in the VRF
	for vrfIdx, vrf := range vrfs {
		// generate unique NH IDs for the two egress ports for this VRF
		nhIDs := []uint64{
			nhBase + uint64(vrfIdx*10) + 0, // port3
			nhBase + uint64(vrfIdx*10) + 1, // port4
		}
		nhgID := nhgBase + uint64(vrfIdx) // unique NHG per VRF

		// build NH entries (all in default NI)
		var entries []fluent.GRIBIEntry
		var ops []*client.OpResult

		for i, portName := range []string{"port3", "port4"} {
			port := dut.Port(t, portName)
			otgDummyIP, ok := otgDummyIPs[port.Name()]
			if !ok {
				t.Fatalf("No dummy IP defined for DUT port %s", port.Name())
			}

			// Use MACwithInterface or MACwithIp depending on deviations; important: network instance = defaultNI
			switch {
			case deviations.GRIBIMACOverrideWithStaticARP(dut):
				nh, op := gribi.NHEntry(nhIDs[i], "MACwithIp", defaultNI, fluent.InstalledInFIB, &gribi.NHOptions{Dest: otgDummyIP, Mac: dstMac})
				entries = append(entries, nh)
				ops = append(ops, op)

			case deviations.GRIBIMACOverrideStaticARPStaticRoute(dut):
				nh, op := gribi.NHEntry(nhIDs[i], "MACwithInterface", defaultNI, fluent.InstalledInFIB, &gribi.NHOptions{Interface: port.Name(), Mac: dstMac, Dest: dstIP})
				entries = append(entries, nh)
				ops = append(ops, op)

			default:
				nh, op := gribi.NHEntry(nhIDs[i], "MACwithInterface", defaultNI, fluent.InstalledInFIB, &gribi.NHOptions{Interface: port.Name(), Mac: dstMac})
				entries = append(entries, nh)
				ops = append(ops, op)
			}
		}

		// Build NHG in default NI (pointing to the two NHs)
		nhMap := map[uint64]uint64{nhIDs[0]: 1, nhIDs[1]: 1}
		nhg, nhgOp := gribi.NHGEntry(nhgID, nhMap, defaultNI, fluent.InstalledInFIB)
		entries = append(entries, nhg)
		ops = append(ops, nhgOp)

		// Install NH + NHG in DEFAULT NI
		c.AddEntries(t, entries, ops)

		// Now add IPv6 route in the VRF NI that references the NHG in default NI with unique prefix per VRF
		prefix := fmt.Sprintf(tunnelDst, vrfIdx+1, routeV6PrfLen)
		c.AddIPv6(t, prefix, nhgID, vrf, defaultNI, fluent.InstalledInFIB)

		t.Logf("Programmed VRF %s: NHs %v NHG %d Prefix %s", vrf, nhIDs, nhgID, prefix)
	}

	t.Log("programBasicEntries completed")
}

// programMPLSinUDPEntries programs gRIBI entries for MPLS-in-UDP encapsulation. It installs a single NextHop that performs MPLS-in-UDP encapsulation with the provided outer IPv6 and UDP header attributes, associates multiple NextHopGroups (NHGs) with that NextHop, and finally installs IPv6 /128 routes pointing to each NHG.
func programMPLSinUDPEntries(t *testing.T, dut *ondatra.DUTDevice, nextHopID, nhgBase, mplsLabelStart uint64, numNHGs int, vrfs []string, outerIPv6Src, outerIPv6Dst string, outerDstUDPPort uint16, outerTTL, outerDSCP uint8) []fluent.GRIBIEntry {
	t.Helper()
	entries := make([]fluent.GRIBIEntry, 0, 1+2*numNHGs*len(vrfs))

	// Single NextHop shared by all VRFs:
	entries = append(entries,
		fluent.NextHopEntry().
			WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(nextHopID).
			AddEncapHeader(
				fluent.MPLSEncapHeader().WithLabels(mplsLabelStart),
				fluent.UDPV6EncapHeader().
					WithSrcIP(outerIPv6Src).
					WithDstIP(outerIPv6Dst).
					WithDstUDPPort(uint64(outerDstUDPPort)).
					WithIPTTL(uint64(outerTTL)).
					WithDSCP(uint64(outerDSCP)),
			),
	)

	baseIP := net.ParseIP(baseIPv6).To16()
	if baseIP == nil {
		t.Fatalf("invalid base IPv6: %q", baseIPv6)
	}

	// For each VRF produce NHGs and routes.
	for vrfIdx, vrf := range vrfs {
		for i := 0; i < numNHGs; i++ {
			// Unique NHG per (vrf, i):
			nhgID := nhgBase + uint64(vrfIdx*numNHGs+i)

			// Add NHG referencing the single nextHopID
			entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).WithID(nhgID).AddNextHop(nextHopID, 1))

			// Compute IPv6 prefix for this (vrf, i)
			prefixIP, perr := iputil.IncrementIPv6(baseIP, uint64(vrfIdx*numNHGs+i))
			if perr != nil {
				t.Fatalf("failed to generate IPv6 prefix: %v", perr)
			}
			prefix := fmt.Sprintf("%s/%s", prefixIP.String(), routeV6PrfLen)

			entries = append(entries, fluent.IPv6Entry().WithNetworkInstance(vrf).WithPrefix(prefix).WithNextHopGroup(nhgID).WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut)))
		}
	}

	return entries
}

// programMPLSinUDPMultiEntries programs MPLS-in-UDP encapsulated NextHops, NextHopGroups, and IPv6 routes across multiple VRFs. It returns a slice of gRIBI entries that can be pushed to the DUT using the fluent gRIBI client.
func programMPLSinUDPMultiEntries(t *testing.T, dut *ondatra.DUTDevice, vrfs []string, mplsNHBase, nhgBase, mplsLabelStart uint64, numNHGs int, outerIPv6Src, outerIPv6Dst string, outerDstUDPPort uint16, outerTTL, outerDSCP uint8) []fluent.GRIBIEntry {
	t.Helper()

	// Preallocate: 1 NextHop per VRF + (numNHGs * (1 NHG + 1 Route)) per VRF
	totalEntries := len(vrfs) * (1 + numNHGs*2)
	entries := make([]fluent.GRIBIEntry, 0, totalEntries)

	defaultNI := deviations.DefaultNetworkInstance(dut)

	for vrfIdx, vrfName := range vrfs {
		// one unique MPLS label and one NH per VRF
		label := mplsLabelStart + uint64(vrfIdx)
		nhID := mplsNHBase + uint64(vrfIdx) // unique NH per VRF

		// NextHop (programmed into default network-instance by convention)
		entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(defaultNI).WithIndex(nhID).
			AddEncapHeader(fluent.MPLSEncapHeader().WithLabels(label),
				fluent.UDPV6EncapHeader().
					WithSrcIP(outerIPv6Src).
					WithDstIP(outerIPv6Dst).
					WithDstUDPPort(uint64(outerDstUDPPort)).
					WithIPTTL(uint64(outerTTL)).
					WithDSCP(uint64(outerDSCP)),
			),
		)

		// For each NHG in this VRF, create NHG and a unique route pointing to it
		for i := range numNHGs {
			nhgID := nhgBase + uint64(vrfIdx*numNHGs+i)

			// NextHopGroup (referring to the NH in defaultNI)
			entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(defaultNI).WithID(nhgID).AddNextHop(nhID, 1))

			// Generate unique IPv6 prefix for this NHG
			offset := int64(vrfIdx*numNHGs + i)
			prefixIP := iputil.GenerateIPv6WithOffset(net.ParseIP(baseIPv6), offset) // returns net.IP or IPNet depending on your util
			prefixStr := fmt.Sprintf("%s%s", prefixIP.String(), routeV6PrfLen)

			entries = append(entries, fluent.IPv6Entry().WithNetworkInstance(vrfName).WithPrefix(prefixStr).WithNextHopGroup(nhgID).WithNextHopGroupNetworkInstance(defaultNI))
		}
	}

	return entries
}

func staticARPWithUniversalIP(t *testing.T, dut *ondatra.DUTDevice, vrfsList, portList []string, sDstIP, sRoutePrfLen, sDstMac, protoType string, indx int) {
	t.Helper()
	sb := new(gnmi.SetBatch)
	for v, vrfName := range vrfsList {
		protoPath := gnmi.OC().NetworkInstance(vrfName).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
		proto := &oc.NetworkInstance_Protocol{Identifier: oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, Name: ygot.String(deviations.StaticProtocolName(dut))}
		gnmi.BatchUpdate(sb, protoPath.Config(), proto)

		sp := protoPath.Static(sDstIP + sRoutePrfLen)

		for i, portName := range portList {
			nhIndex := fmt.Sprintf("%d", indx+v*len(portList)+i)
			port := dut.Port(t, portName)

			// Create next-hop entry
			nh := &oc.NetworkInstance_Protocol_Static_NextHop{Index: ygot.String(nhIndex), InterfaceRef: &oc.NetworkInstance_Protocol_Static_NextHop_InterfaceRef{Interface: ygot.String(port.Name())}}

			// Add next-hop config under the static route
			gnmi.BatchUpdate(sb, sp.NextHop(nhIndex).Config(), nh)

			// Add static ARP entry for that interface
			gnmi.BatchUpdate(sb, gnmi.OC().Interface(port.Name()).Config(), configStaticArp(port.Name(), sDstIP, sDstMac, protoType))

			t.Logf("Added static route in VRF %s: %s -> %s (NextHop %s)", vrfName, sDstIP, port.Name(), nhIndex)
		}
	}
	sb.Set(t, dut)
}

// validateMPLSPacketCapture analyzes a packet capture on the given OTG port and validates MPLS-in-UDP encapsulation against the expected parameters. Returns an error if validation fails.
func validateMPLSPacketCapture(t *testing.T, ate *ondatra.ATEDevice, otgPortName string, pr *packetResult, labelList []uint64) error {
	t.Helper()
	t.Logf("=== PACKET CAPTURE VALIDATION START for port %s ===", otgPortName)

	// Get raw packet bytes
	packetBytes := ate.OTG().GetCapture(t, gosnappi.NewCaptureRequest().SetPortName(otgPortName))
	if len(packetBytes) == 0 {
		return fmt.Errorf("no packet data captured on port %s", otgPortName)
	}
	t.Logf("Captured %d bytes from port %s", len(packetBytes), otgPortName)

	// Write capture to temporary pcap file
	tmpFile, err := os.CreateTemp("", "*.pcap")
	if err != nil {
		return fmt.Errorf("could not create temporary pcap file: %v", err)
	}
	if _, err := tmpFile.Write(packetBytes); err != nil {
		return fmt.Errorf("could not write packet data: %v", err)
	}
	tmpFile.Close()

	handle, err := pcap.OpenOffline(tmpFile.Name())
	if err != nil {
		return fmt.Errorf("could not open pcap file: %v", err)
	}
	defer handle.Close()

	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())

	// Optimize label lookups by using a map
	labelMap := make(map[uint64]struct{}, len(labelList))
	for _, l := range labelList {
		labelMap[l] = struct{}{}
	}

	var packetCount, mplsPacketCount, validMplsPacketCount int
	for packet := range packetSource.Packets() {
		packetCount++
		udpLayer := packet.Layer(layers.LayerTypeUDP)
		ipv6Layer := packet.Layer(layers.LayerTypeIPv6)
		// Skip packets that are not UDP-over-IPv6. For debugging, we log only the first 5 skipped packets to avoid flooding the test logs if the capture contains many irrelevant packets (e.g., ARP, ICMP).
		if udpLayer == nil || ipv6Layer == nil {
			if packetCount <= 5 {
				t.Logf("Packet %d: Skipping non-UDP-IPv6 packet", packetCount)
			}
			continue
		}
		mplsPacketCount++

		// Validate IPv6 header
		v6 := ipv6Layer.(*layers.IPv6)
		if v6.DstIP.String() != pr.dstIP {
			return fmt.Errorf("packet %d: got dstIP %s, want %s", packetCount, v6.DstIP, pr.dstIP)
		}
		if v6.SrcIP.String() != pr.srcIP {
			return fmt.Errorf("packet %d: got srcIP %s, want %s", packetCount, v6.SrcIP, pr.srcIP)
		}
		if v6.HopLimit != pr.ipTTL {
			return fmt.Errorf("packet %d: got hopLimit %d, want %d", packetCount, v6.HopLimit, pr.ipTTL)
		}

		// Extract UDP payload (MPLS header)
		payload := udpLayer.LayerPayload()
		if len(payload) < 4 {
			return fmt.Errorf("packet %d: UDP payload too short (len=%d)", packetCount, len(payload))
		}
		mplsHeader := binary.BigEndian.Uint32(payload[:4])
		label := (mplsHeader >> 12) & 0xFFFFF
		bos := (mplsHeader >> 8) & 0x1
		mplsTTL := mplsHeader & 0xFF

		// Label validation
		if len(labelMap) > 0 {
			if _, ok := labelMap[uint64(label)]; !ok {
				return fmt.Errorf("packet %d: got MPLS label %d, not in %v", packetCount, label, labelList)
			}
		} else if uint64(label) != pr.mplsLabel {
			return fmt.Errorf("packet %d: got MPLS label %d, want %d", packetCount, label, pr.mplsLabel)
		}
		if bos != 1 {
			return fmt.Errorf("packet %d: BOS bit = %d, want 1", packetCount, bos)
		}
		expectedMPLSTTL := innerTTL - 1
		if uint32(mplsTTL) != expectedMPLSTTL {
			return fmt.Errorf("packet %d: got MPLS TTL %d, want %d", packetCount, mplsTTL, expectedMPLSTTL)
		}

		validMplsPacketCount++
		if validMplsPacketCount <= 2 {
			// Validate the first two packets only and limit logging to reduce log storage.
			t.Logf("Packet %d: MPLS validation PASSED", packetCount)
		}
	}

	// Summary
	t.Logf("=== PACKET CAPTURE VALIDATION SUMMARY ===")
	t.Logf("Total packets: %d, UDP-IPv6: %d, Valid MPLS-in-UDP: %d",
		packetCount, mplsPacketCount, validMplsPacketCount)

	// Validation checks
	switch {
	case packetCount == 0:
		return fmt.Errorf("no packets captured on port %s", otgPortName)

	case mplsPacketCount == 0:
		return fmt.Errorf("no UDP-IPv6 packets found on port %s", otgPortName)

	case validMplsPacketCount == 0:
		return fmt.Errorf("no valid MPLS-in-UDP packets found on port %s", otgPortName)
	default:
		// Strict tolerance aligned with README: target minimal loss (e.g., <=1%).
		const allowedLossPct = 1.0
		allowedFailures := int(math.Ceil(float64(mplsPacketCount) * allowedLossPct / 100.0))
		invalid := mplsPacketCount - validMplsPacketCount

		if invalid > allowedFailures {
			return fmt.Errorf("too many invalid packets: %d/%d (allowed %d failures = %.2f%%)",
				invalid, mplsPacketCount, allowedFailures, allowedLossPct)
		}
	}
	t.Logf("Validation PASSED: %d valid MPLS-in-UDP packets", validMplsPacketCount)
	return nil
}

// Main test entry point.
func TestMPLSinUDPScale(t *testing.T) {
	t.Helper()
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	ctx := t.Context()
	vrfsList := configureDUT(t, dut)
	ateConfig := configureOTG(t, ate, dut)
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
	programBasicEntries(t, dut, &c, vrfsList)
	t.Run("Profile-1-Single VRF", func(t *testing.T) {
		if err := configureVRFProfiles(ctx, t, ateConfig, dut, ate, &c, []string{vrfsList[0], vrfsList[1]}, profileSingleVRF, false); err != nil {
			t.Errorf("configureVrfProfiles failed: %v", err)
		}
	})
	t.Run("Profile-2-Multi VRF", func(t *testing.T) {
		if err := configureVRFProfiles(ctx, t, ateConfig, dut, ate, &c, vrfsList, profileMultiVRF, true); err != nil {
			t.Errorf("configureVrfProfiles failed: %v", err)
		}
	})
	t.Run("Profile-3-Multi VRF with Skew", func(t *testing.T) {
		if err := configureVRFProfiles(ctx, t, ateConfig, dut, ate, &c, vrfsList, profileMultiVRFSkew, true); err != nil {
			t.Errorf("configureVrfProfiles failed: %v", err)
		}
	})
	t.Run("Profile-4-Single VRF", func(t *testing.T) {
		if err := configureVRFProfiles(ctx, t, ateConfig, dut, ate, &c, []string{vrfsList[0], vrfsList[1]}, profileSingleVRFECMP, false); err != nil {
			t.Errorf("configureVrfProfiles failed: %v", err)
		}
	})
	t.Run("Profile-5-Single VRF", func(t *testing.T) {
		if err := configureVRFProfiles(ctx, t, ateConfig, dut, ate, &c, []string{vrfsList[0], vrfsList[1]}, profileSingleVRFgRIBI, false); err != nil {
			t.Errorf("configureVrfProfiles failed: %v", err)
		}
	})
}

// configureVRFProfiles implements the “Single/Multi VRF Validation” for Profile 1 (baseline) and Profile 4 (ECMP). It programs MPLS-in-UDP NHs, NHGs, and 20k prefixes (10k v4 + 10k v6), validates FIB/AFT, sends traffic, checks MPLS-over-UDP encapsulation, and deletes entries.
func configureVRFProfiles(ctx context.Context, t *testing.T, ateConfig gosnappi.Config, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, c *gribi.Client, vrfs []string, profile vrfProfile, otgMultiPortCaptureSupported bool) error {
	t.Helper()
	cfg, ok := profiles[profile]
	if !ok {
		return fmt.Errorf("Unsupported profile %d", profile)
	}
	totalNHGs := cfg.totalNHGs
	nhsPerNHG := cfg.nhsPerNHG
	totalNHs := totalNHGs * nhsPerNHG
	var (
		entries   []fluent.GRIBIEntry
		wantAdds  []*client.OpResult
		wantDels  []*client.OpResult
		flows     []gosnappi.Flow
		labelList []uint64
	)
	// === Program MPLS-in-UDP NH & NHG entries ===
	switch profile {
	case profileMultiVRF:
		entries = programMPLSinUDPMultiEntries(t, dut, vrfs, nextHopID, nextHopGroupID, mplsLabel, totalNHGs, outerIPv6Src, outerIPv6Dst, outerDstUDPPort, outerTTL, outerDSCP)
		wantAdds, wantDels = expectedMPLSinUDPMultiOpResults(t, vrfs, nextHopID, nextHopGroupID, totalNHGs)
		flows = []gosnappi.Flow{fa6.createFlow("ipv6", fmt.Sprintf("ip6mpls_p%d", profile), outerDSCP)}
	case profileMultiVRFSkew:
		entries = programMPLSinUDPSkewedEntries(t, dut, vrfs, nextHopID, nextHopGroupID, mplsLabel, totalNHGs, outerIPv6Src, outerIPv6Dst, outerDstUDPPort, outerTTL, outerDSCP)
		wantAdds, wantDels = expectedMPLSinUDPSkewedResults(t, vrfs, nextHopID, nextHopGroupID, totalNHGs)
		flows = []gosnappi.Flow{fa6.createFlow("ipv6", fmt.Sprintf("ip6mpls_p%d", profile), outerDSCP)}
	default:
		// Profile 1 (single VRF), Profile 4 (ECMP) and Profile 5 fall here
		entries = programMPLSinUDPEntries(t, dut, nextHopID, nextHopGroupID, mplsLabel, totalNHGs, vrfs, outerIPv6Src, outerIPv6Dst, outerDstUDPPort, outerTTL, outerDSCP)
		// === Add IPv4 + IPv6 route entries ===
		ipv4Entries := buildIPv4Routes(t, dut, totalPrefixes/2, baseIPv4, vrfs, nextHopGroupID)
		ipv6Entries := buildIPv6Routes(t, dut, totalPrefixes/2, baseIPv6, vrfs, nextHopGroupID)
		entries = append(entries, ipv4Entries...)
		entries = append(entries, ipv6Entries...)
		// === Expected OpResults ===
		wantAdds, wantDels = expectedMPLSinUDPOpResults(t, nextHopID, nextHopGroupID, totalNHGs, totalPrefixes, baseIPv4, baseIPv6, vrfs)
		flows = []gosnappi.Flow{fa4.createFlow("ipv4", fmt.Sprintf("ip4mpls_p%d", profile), outerDSCP), fa6.createFlow("ipv6", fmt.Sprintf("ip6mpls_p%d", profile), outerDSCP)}
	}

	// === Verify infra installed ===
	if err := c.AwaitTimeout(ctx, t, 3*time.Minute); err != nil {
		return fmt.Errorf("Failed to install infra entries for profile %d: %v", profile, err)
	}

	testCaseArgs := &testCase{
		name:                fmt.Sprintf("Profile%d: MPLS-in-UDP Traffic Encap (Single VRF, %d NHGs × %d NHs, %d total prefixes split v4/v6)", profile, totalNHGs, nhsPerNHG, totalPrefixes),
		entries:             entries,
		wantAddResults:      wantAdds,
		wantDelResults:      wantDels,
		flows:               flows,
		capturePorts:        []string{"port3", "port4"},
		wantMPLSLabel:       uint64(mplsLabel),
		wantOuterDstIP:      outerIPv6Dst,
		wantOuterSrcIP:      outerIPv6Src,
		wantOuterDstUDPPort: outerDstUDPPort,
		wantOuterTTL:        outerTTL,
	}

	tArgs := &testArgs{
		client: c,
		dut:    dut,
		ate:    ate,
		topo:   ateConfig,
	}

	// === Add Entries ===
	t.Logf("Programming Profile %d: %d NHGs × %d NHs/NHG = %d total NHs, %d prefixes", profile, totalNHGs, nhsPerNHG, totalNHs, totalPrefixes)
	c.AddEntries(t, testCaseArgs.entries, testCaseArgs.wantAddResults)

	// === Capture & Send Traffic ===
	expectedPkt := &packetResult{
		mplsLabel:  testCaseArgs.wantMPLSLabel,
		udpDstPort: testCaseArgs.wantOuterDstUDPPort,
		ipTTL:      testCaseArgs.wantOuterTTL,
		srcIP:      testCaseArgs.wantOuterSrcIP,
		dstIP:      testCaseArgs.wantOuterDstIP,
	}
	if otgMultiPortCaptureSupported {
		enableCapture(t, ate.OTG(), ateConfig, testCaseArgs.capturePorts)
		sendTraffic(t, tArgs, testCaseArgs.flows, true)
		err := validateMPLSPacketCapture(t, ate, testCaseArgs.capturePorts[0], expectedPkt, labelList)
		if err != nil {
			return fmt.Errorf("profile %d capture validation failed: %v", profile, err)
		}
		clearCapture(t, ate.OTG(), ateConfig)
	} else {
		for _, port := range testCaseArgs.capturePorts {
			enableCapture(t, ate.OTG(), ateConfig, []string{port})
			sendTraffic(t, tArgs, testCaseArgs.flows, true)
			err := validateMPLSPacketCapture(t, ate, port, expectedPkt, labelList)
			if err != nil {
				return fmt.Errorf("profile %d capture validation failed: %v", profile, err)
			}
			clearCapture(t, ate.OTG(), ateConfig)
		}
	}

	// Validate forwarding (allow the helper to return an error for test assertions)
	if err := validateTrafficFlows(t, ate, ateConfig, tArgs, testCaseArgs.flows, false, true); err != nil {
		return fmt.Errorf("profile %d traffic validation failed: %v", profile, err)
	}
	// === Profile 5 specific QPS scaling ===
	if profile == 5 {
		t.Log("Starting Profile 5 high-rate gRIBI ops at ~1k ops/sec")

		// build 60k ops (20k × 3 per entry)
		ops, _ := buildProfile5Ops(t, dut, totalPrefixes, nextHopGroupID, baseIPv6, vrfs)

		// pump ops at rate in a goroutine while sending dataplane traffic
		var pumpWg sync.WaitGroup
		pumpWg.Add(1)
		go func() {
			defer pumpWg.Done()
			// pumpOpsAtRate blocks while streaming; capture returned errors via t.Fatal inside helper if required
			pumpOpsAtRate(ctx, t, c, ops, 1000)
		}()

		// keep sending dataplane traffic while ops are streaming
		sendTraffic(t, tArgs, testCaseArgs.flows, false)
		// Wait for pump to finish before validating flows (if pumpOpsAtRate is synchronous this returns promptly)
		pumpWg.Wait()

		if err := validateTrafficFlows(t, ate, ateConfig, tArgs, testCaseArgs.flows, false, true); err != nil {
			return fmt.Errorf("profile %d traffic validation failed: %v", profile, err)
		}
		t.Log("Completed Profile 5 QPS scaling phase")
	}
	// === Delete Entries ===
	t.Logf("Deleting MPLS-in-UDP entries for Profile %d", profile)
	n := len(testCaseArgs.entries)
	revEntries := make([]fluent.GRIBIEntry, n)
	for i := 0; i < n; i++ {
		revEntries[i] = testCaseArgs.entries[n-1-i]
	}
	c.DeleteEntries(t, revEntries, testCaseArgs.wantDelResults)

	// Validate that traffic fails after deletion (expect loss)
	t.Logf("Verifying traffic fails after MPLS-in-UDP entries deleted for Profile %d", profile)
	if perr := validateTrafficFlows(t, ate, ateConfig, tArgs, testCaseArgs.flows, false, false); perr != nil {
		return fmt.Errorf("profile %d post-delete traffic validation failed: %v", profile, perr)
	}

	t.Logf("Profile %d finished: %d NHGs × %d NHs", profile, totalNHGs, nhsPerNHG)
	return nil
}

// buildProfile5Ops generates ADD/DELETE ops mix for Profile 5.
func buildProfile5Ops(t *testing.T, dut *ondatra.DUTDevice, totalPrefixes int, nhgBase uint64, baseIPv6 string, vrfs []string) (adds, dels []fluent.GRIBIEntry) {
	t.Helper()

	// Generate IPv6 routes for all VRFs
	for _, vrf := range vrfs {
		ipv6s := buildIPv6Routes(t, dut, totalPrefixes/2, baseIPv6, []string{vrf}, nhgBase)
		for i, e := range ipv6s {
			if i%2 == 0 {
				// ADD this entry
				adds = append(adds, e)
			} else {
				// DELETE version: rebuild with same prefix & NHG
				nhgID := nhgBase + uint64(i%2)
				dels = append(dels, fluent.IPv6Entry().WithNetworkInstance(vrf).WithPrefix(baseIPv6+routeV6PrfLen).WithNextHopGroup(nhgID).WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut)))
			}
		}
	}

	return adds, dels
}

// pumpOpsAtRate sends ops to gRIBI client at target ops/sec.
func pumpOpsAtRate(ctx context.Context, t *testing.T, c *gribi.Client, ops []fluent.GRIBIEntry, targetOps int) {
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
func buildIPv4Routes(t *testing.T, dut *ondatra.DUTDevice, num int, baseIPv4 string, vrfs []string, nhgBase uint64) []fluent.GRIBIEntry {
	t.Helper()
	ip := net.ParseIP(baseIPv4).To4()
	if ip == nil {
		t.Fatalf("invalid baseIPv4: %s", baseIPv4)
	}

	totalEntries := num * len(vrfs)
	entries := make([]fluent.GRIBIEntry, 0, totalEntries)

	for vrfIdx, vrf := range vrfs {
		for i := 0; i < num; i++ {
			// compute unique prefix per vrf+i
			prefixIP, perr := iputil.IncrementIPv4(ip, uint32(vrfIdx*num+i))
			if perr != nil {
				t.Fatalf("failed to increment IPv4: %v", perr)
			}
			prefix := fmt.Sprintf("%s%s", prefixIP.String(), routeV4PrfLen)
			nhgID := nhgBase + uint64(vrfIdx*num+i)

			entries = append(entries, fluent.IPv4Entry().WithNetworkInstance(vrf).WithPrefix(prefix).WithNextHopGroup(nhgID).WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut)))
		}
	}
	return entries
}

// buildIPv6Routes generates num IPv6 prefixes per VRF and maps to nhgBase + vrfIdx*num + i.
func buildIPv6Routes(t *testing.T, dut *ondatra.DUTDevice, num int, baseIPv6 string, vrfs []string, nhgBase uint64) []fluent.GRIBIEntry {
	t.Helper()
	ip := net.ParseIP(baseIPv6).To16()
	if ip == nil {
		t.Fatalf("invalid baseIPv6: %s", baseIPv6)
	}

	totalEntries := num * len(vrfs)
	entries := make([]fluent.GRIBIEntry, 0, totalEntries)

	for vrfIdx, vrf := range vrfs {
		for i := 0; i < num; i++ {
			prefixIP, perr := iputil.IncrementIPv6(ip, uint64(vrfIdx*num+i))
			if perr != nil {
				t.Fatalf("failed to increment IPv6: %v", perr)
			}
			prefix := fmt.Sprintf("%s%s", prefixIP.String(), routeV6PrfLen)
			nhgID := nhgBase + uint64(vrfIdx*num+i)

			entries = append(entries, fluent.IPv6Entry().WithNetworkInstance(vrf).WithPrefix(prefix).WithNextHopGroup(nhgID).WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut)))
		}
	}
	return entries
}

// expectedMPLSinUDPOpResults builds expected gRIBI OpResults for Profile 5. It models the baseline state of one MPLS-in-UDP NH, all NHGs, and 20k route entries (10k IPv4 + 10k IPv6).
func expectedMPLSinUDPOpResults(t *testing.T, nextHopID, nhgBase uint64, numNHGs, totalPrefixes int, baseIPv4, baseIPv6 string, vrfs []string) (adds, dels []*client.OpResult) {
	t.Helper()

	// Conservative capacity: 1 NH + (NHGs per VRF) + totalPrefixes (routes)
	totalNHGs := numNHGs * len(vrfs)
	adds = make([]*client.OpResult, 0, 1+totalNHGs+totalPrefixes)
	dels = make([]*client.OpResult, 0, 1+totalNHGs+totalPrefixes)

	// Step 1: One NH (shared)
	adds = append(adds, fluent.OperationResult().WithNextHopOperation(nextHopID).WithProgrammingResult(fluent.InstalledInFIB).WithOperationType(constants.Add).AsResult())
	dels = append(dels, fluent.OperationResult().WithNextHopOperation(nextHopID).WithProgrammingResult(fluent.InstalledInFIB).WithOperationType(constants.Delete).AsResult())

	// Step 2: NHGs (unique per VRF+i)
	for vrfIdx := range vrfs {
		for i := 0; i < numNHGs; i++ {
			nhgID := nhgBase + uint64(vrfIdx*numNHGs+i)
			adds = append(adds, fluent.OperationResult().WithNextHopGroupOperation(nhgID).WithProgrammingResult(fluent.InstalledInFIB).WithOperationType(constants.Add).AsResult())
			dels = append(dels, fluent.OperationResult().WithNextHopGroupOperation(nhgID).WithProgrammingResult(fluent.InstalledInFIB).WithOperationType(constants.Delete).AsResult())
		}
	}

	// --- Steps 3 & 4: IPv4 & IPv6 routes across VRFs ---
	// We'll generate (totalPrefixes/2) IPv4 and the rest IPv6. For per-VRF distribution
	// compute per-VRF counts so prefixes are unique and deterministic.
	v4Count := totalPrefixes / 2
	v6Count := totalPrefixes - v4Count

	// sanity checks for bases
	base4 := net.ParseIP(baseIPv4).To4()
	base6 := net.ParseIP(baseIPv6).To16()
	if base4 == nil {
		t.Fatalf("invalid baseIPv4: %s", baseIPv4)
	}
	if base6 == nil {
		t.Fatalf("invalid baseIPv6: %s", baseIPv6)
	}

	// Function to append prefix op results (concurrent-safe with mutex)
	var mu sync.Mutex
	appendOps := func(isV6 bool, prefix string) {
		mu.Lock()
		defer mu.Unlock()
		if isV6 {
			adds = append(adds, fluent.OperationResult().WithIPv6Operation(prefix).WithProgrammingResult(fluent.InstalledInFIB).WithOperationType(constants.Add).AsResult())
			dels = append(dels, fluent.OperationResult().WithIPv6Operation(prefix).WithProgrammingResult(fluent.InstalledInFIB).WithOperationType(constants.Delete).AsResult())
		} else {
			adds = append(adds, fluent.OperationResult().WithIPv4Operation(prefix).WithProgrammingResult(fluent.InstalledInFIB).WithOperationType(constants.Add).AsResult())
			dels = append(dels, fluent.OperationResult().WithIPv4Operation(prefix).WithProgrammingResult(fluent.InstalledInFIB).WithOperationType(constants.Delete).AsResult())
		}
	}

	workers := runtime.NumCPU()
	var wg sync.WaitGroup

	// Helper to generate prefixes deterministically. For each VRF, produce perVRFCount prefixes.
	generatePerVRF := func(isV6 bool, perVRFCount int, base net.IP) {
		if perVRFCount == 0 {
			return
		}
		batch := (perVRFCount + workers - 1) / workers
		for vrfIdx := range vrfs {
			// For each VRF we generate perVRFCount prefixes, index offset = vrfIdx*perVRFCount + i
			for w := 0; w < workers; w++ {
				start := w * batch
				end := start + batch
				if end > perVRFCount {
					end = perVRFCount
				}
				if start >= end {
					continue
				}
				wg.Add(1)
				go func(vrfIdx, start, end int) {
					defer wg.Done()
					for i := start; i < end; i++ {
						globalIndex := vrfIdx*perVRFCount + i
						var prefixIP net.IP
						var perr error
						if isV6 {
							prefixIP, perr = iputil.IncrementIPv6(base, uint64(globalIndex))
						} else {
							prefixIP, perr = iputil.IncrementIPv4(base, uint32(globalIndex))
						}
						if perr != nil {
							t.Errorf("prefix generation error: %v", perr)
							continue
						}
						var prefix string
						if isV6 {
							prefix = fmt.Sprintf("%s%s", prefixIP.String(), routeV6PrfLen)
						} else {
							prefix = fmt.Sprintf("%s%s", prefixIP.String(), routeV4PrfLen)
						}
						appendOps(isV6, prefix)
					}
				}(vrfIdx, start, end)
			}
		}
	}

	// per-VRF counts: distribute evenly across VRFs
	perVrfV4 := 0
	perVrfV6 := 0
	if len(vrfs) > 0 {
		perVrfV4 = (v4Count + len(vrfs) - 1) / len(vrfs) // ceil-ish distribution
		perVrfV6 = (v6Count + len(vrfs) - 1) / len(vrfs)
	}

	// Generate v4 & v6 prefixes across VRFs
	generatePerVRF(false, perVrfV4, base4)
	generatePerVRF(true, perVrfV6, base6)

	wg.Wait()
	return adds, dels
}

// expectedMPLSinUDPMultiOpResults builds the expected set of Add/Delete OperationResults for MPLS-in-UDP multi-VRF programming.
func expectedMPLSinUDPMultiOpResults(t *testing.T, vrfs []string, mplsNHBase, nhgBase uint64, numNHGs int) (adds, dels []*client.OpResult) {
	t.Helper()
	totalOps := len(vrfs) * (1 + numNHGs*2) // 1 NH + 2*numNHGs per VRF
	adds = make([]*client.OpResult, 0, totalOps)
	dels = make([]*client.OpResult, 0, totalOps)

	// === Step 1: NHs (cheap, sequential) ===
	for vrfIdx := range vrfs {
		nhID := mplsNHBase + uint64(vrfIdx)
		adds = append(adds, fluent.OperationResult().WithNextHopOperation(nhID).WithProgrammingResult(fluent.InstalledInFIB).WithOperationType(constants.Add).AsResult())
		dels = append(dels, fluent.OperationResult().WithNextHopOperation(nhID).WithProgrammingResult(fluent.InstalledInFIB).WithOperationType(constants.Delete).AsResult())
	}

	// === Step 2 & 3: NHGs and Routes (parallelized) ===
	type result struct {
		add []*client.OpResult
		del []*client.OpResult
	}

	totalJobs := len(vrfs) * numNHGs
	results := make([]result, totalJobs)

	jobs := make(chan int, totalJobs)
	var wg sync.WaitGroup

	workers := runtime.NumCPU() * 2 // tunable
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				vrfIdx := idx / numNHGs
				i := idx % numNHGs

				nhgID := nhgBase + uint64(vrfIdx*numNHGs+i)

				// NHG add/del
				addOps := []*client.OpResult{fluent.OperationResult().WithNextHopGroupOperation(nhgID).WithProgrammingResult(fluent.InstalledInFIB).WithOperationType(constants.Add).AsResult()}
				delOps := []*client.OpResult{fluent.OperationResult().WithNextHopGroupOperation(nhgID).WithProgrammingResult(fluent.InstalledInFIB).WithOperationType(constants.Delete).AsResult()}

				// Prefix add/del
				offset := int64(vrfIdx*numNHGs + i)
				prefixIP := iputil.GenerateIPv6WithOffset(net.ParseIP(baseIPv6), offset)
				prefixStr := fmt.Sprintf("%s%s", prefixIP.String(), routeV6PrfLen)

				addOps = append(addOps, fluent.OperationResult().WithIPv6Operation(prefixStr).WithProgrammingResult(fluent.InstalledInFIB).WithOperationType(constants.Add).AsResult())
				delOps = append(delOps, fluent.OperationResult().WithIPv6Operation(prefixStr).WithProgrammingResult(fluent.InstalledInFIB).WithOperationType(constants.Delete).AsResult())

				results[idx] = result{add: addOps, del: delOps}
			}
		}()
	}

	// Feed jobs
	for i := 0; i < totalJobs; i++ {
		jobs <- i
	}
	close(jobs)

	wg.Wait()

	// === Deterministic gather ===
	for _, r := range results {
		adds = append(adds, r.add...)
		dels = append(dels, r.del...)
	}

	return adds, dels
}

// generateSkewPattern returns a slice of prefix counts per VRF, summing to totalNHGs.
func generateSkewPattern(numVRFs, totalNHGs int) []int {
	pattern := make([]int, numVRFs)

	if numVRFs == 0 || totalNHGs == 0 {
		return pattern
	}

	// Define heavier VRFs: e.g., top 3 get more
	heavyCount := int(math.Max(1, float64(numVRFs)/4)) // first 25% VRFs heavier
	lightCount := numVRFs - heavyCount

	// Calculate total weight: heavy VRFs 2x, light VRFs 1x
	totalWeight := heavyCount*2 + lightCount*1

	assigned := 0
	for i := 0; i < numVRFs; i++ {
		if i < heavyCount {
			pattern[i] = totalNHGs * 2 / totalWeight
		} else {
			pattern[i] = totalNHGs * 1 / totalWeight
		}
		assigned += pattern[i]
	}

	// Fix rounding drift
	leftover := totalNHGs - assigned
	for i := 0; leftover > 0; i = (i + 1) % numVRFs {
		pattern[i]++
		leftover--
	}

	return pattern
}

// programMPLSinUDPSkewedEntries programs skewed entries for Profile 3.
func programMPLSinUDPSkewedEntries(t *testing.T, dut *ondatra.DUTDevice, vrfs []string, mplsNHBase, nhgBase, mplsLabelStart uint64, totalNHGs int, outerIPv6Src, outerIPv6Dst string, outerDstUDPPort uint16, outerTTL, outerDSCP uint8) []fluent.GRIBIEntry {
	t.Helper()
	entries := make([]fluent.GRIBIEntry, 0, totalNHGs*3)

	// defaultNI still used for other things; but NH/NHG should be in the VRF NI
	defaultNI := deviations.DefaultNetworkInstance(dut)

	skewPattern := generateSkewPattern(len(vrfs), totalNHGs)
	nhIndex := 0

	for vrfIdx, vrfName := range vrfs {
		label := mplsLabelStart + uint64(vrfIdx)
		vrfNHID := mplsNHBase + uint64(vrfIdx)
		// Increment outer tunnel destination per VRF
		vrfOuterDst := iputil.GenerateIPv6WithOffset(net.ParseIP(outerIPv6Dst), int64(vrfIdx))
		// Create NH in the same network-instance as the route (vrfName)
		entries = append(entries,
			fluent.NextHopEntry().
				WithNetworkInstance(defaultNI). // create NH in VRF NI
				WithIndex(vrfNHID).
				AddEncapHeader(
					fluent.MPLSEncapHeader().WithLabels(label),
					fluent.UDPV6EncapHeader().
						WithSrcIP(outerIPv6Src).
						WithDstIP(vrfOuterDst.String()).
						WithDstUDPPort(uint64(outerDstUDPPort)).
						WithIPTTL(uint64(outerTTL)).
						WithDSCP(uint64(outerDSCP)),
				).WithNextHopNetworkInstance(vrfName),
		)

		// create NHGs also in the same VRF NI
		numNHGsVRF := skewPattern[vrfIdx]
		for i := 0; i < numNHGsVRF; i++ {
			nhgID := nhgBase + uint64(nhIndex)

			entries = append(entries,
				fluent.NextHopGroupEntry().
					WithNetworkInstance(defaultNI). // NHG in VRF NI
					WithID(nhgID).
					AddNextHop(vrfNHID, 1),
			)

			// Generate a prefix unique to this VRF
			prefixIP := iputil.GenerateIPv6WithOffset(net.ParseIP(baseIPv6), int64(nhIndex))
			prefixStr := fmt.Sprintf("%s%s", prefixIP.String(), routeV6PrfLen)

			entries = append(entries,
				fluent.IPv6Entry().
					WithNetworkInstance(vrfName). // install route in this VRF
					WithPrefix(prefixStr).
					WithNextHopGroup(nhgID).WithNextHopGroupNetworkInstance(defaultNI), // NHG is in same VRF
			)
			nhIndex++
		}
	}

	return entries
}

// expectedMPLSinUDPSkewedResults generates expected add/delete operation results.
func expectedMPLSinUDPSkewedResults(t *testing.T, vrfs []string, mplsNHBase, nhgBase uint64, totalNHGs int) (adds, dels []*client.OpResult) {
	t.Helper()
	adds = []*client.OpResult{}
	dels = []*client.OpResult{}

	skewPattern := generateSkewPattern(len(vrfs), totalNHGs)
	nhIndex := 0
	for vrfIdx := range vrfs {
		vrfNHID := mplsNHBase + uint64(vrfIdx)

		// NH add/del
		adds = append(adds, fluent.OperationResult().WithNextHopOperation(vrfNHID).WithProgrammingResult(fluent.InstalledInFIB).WithOperationType(constants.Add).AsResult())
		dels = append(dels, fluent.OperationResult().WithNextHopOperation(vrfNHID).WithProgrammingResult(fluent.InstalledInFIB).WithOperationType(constants.Delete).AsResult())

		numNHGsVRF := skewPattern[vrfIdx]
		for i := 0; i < numNHGsVRF; i++ {
			nhgID := nhgBase + uint64(nhIndex)

			addOps := []*client.OpResult{fluent.OperationResult().WithNextHopGroupOperation(nhgID).WithProgrammingResult(fluent.InstalledInFIB).WithOperationType(constants.Add).AsResult()}
			delOps := []*client.OpResult{fluent.OperationResult().WithNextHopGroupOperation(nhgID).WithProgrammingResult(fluent.InstalledInFIB).WithOperationType(constants.Delete).AsResult()}

			prefixIP := iputil.GenerateIPv6WithOffset(net.ParseIP(baseIPv6), int64(nhIndex))
			prefixStr := fmt.Sprintf("%s%s", prefixIP.String(), routeV6PrfLen)

			addOps = append(addOps, fluent.OperationResult().WithIPv6Operation(prefixStr).WithProgrammingResult(fluent.InstalledInFIB).WithOperationType(constants.Add).AsResult())
			delOps = append(delOps, fluent.OperationResult().WithIPv6Operation(prefixStr).WithProgrammingResult(fluent.InstalledInFIB).WithOperationType(constants.Delete).AsResult())

			adds = append(adds, addOps...)
			dels = append(dels, delOps...)
			nhIndex++
		}
	}

	return adds, dels
}

// validateTrafficFlows verifies traffic flow behavior (pass/fail) based on expected outcome.
func validateTrafficFlows(t *testing.T, ate *ondatra.ATEDevice, ateConfig gosnappi.Config, args *testArgs, flows []gosnappi.Flow, capture bool, match bool) error {
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
			return fmt.Errorf("OutPkts for flow %s is 0, want > 0", flow.Name())
		}

		if match {
			// Expecting traffic to pass (0% loss)
			if got := lossPct; got > tolerance {
				return fmt.Errorf("Traffic validation FAILED: Flow %s has %v%% packet loss, want 0%%", flow.Name(), got)
			}
		} else {
			// Expecting traffic to fail (100% loss)
			if got := lossPct; got != 100 {
				return fmt.Errorf("Traffic validation FAILED: Flow %s has %v%% packet loss, want 100%%", flow.Name(), got)
			}
		}
		if match {
			rxPorts := []string{ateConfig.Ports().Items()[2].Name(), ateConfig.Ports().Items()[3].Name()}
			weights := testLoadBalance(t, ate, rxPorts)
			for idx, weight := range portsTrafficDistribution {
				if got, want := weights[idx], weight; got < (want-tolerance) || got > (want+tolerance) {
					return fmt.Errorf("ECMP percentage mismatch on Aggregate : %d: got %d, want %d", idx+1, got, want)
				}
			}
		}
	}
	return nil
}

// sendTraffic sends traffic flows for the specified duration.
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

// createFlow creates a traffic flow for MPLS-in-UDP testing.
func (fa *flowAttr) createFlow(flowType string, name string, dscp uint32) gosnappi.Flow {
	flow := fa.topo.Flows().Add().SetName(name)
	flow.Metrics().SetEnable(true)
	flow.Rate().SetPps(ratePPS)
	flow.Size().SetFixed(pktSize)
	flow.TxRx().Port().SetTxName(fa.srcPort).SetRxNames(fa.dstPorts)
	e1 := flow.Packet().Add().Ethernet()
	e1.Src().SetValue(fa.srcMac)
	e1.Dst().SetValue(fa.dstMac)

	if flowType == "ipv6" {
		v6 := flow.Packet().Add().Ipv6()
		v6.Src().SetValue(fa.src)
		v6.Dst().Increment().SetStart(fa.dst).SetCount(incCount)
		v6.HopLimit().SetValue(innerTTL)
		v6.TrafficClass().SetValue(dscp << 2)
	} else {
		v4 := flow.Packet().Add().Ipv4()
		v4.Src().SetValue(fa.src)
		v4.Dst().Increment().SetStart(fa.dst).SetCount(incCount)
		v4.TimeToLive().SetValue(innerTTL)
		v4.Priority().Dscp().Phb().SetValue(dscp)
	}

	// Add UDP payload to generate traffic
	udp := flow.Packet().Add().Udp()
	udp.SrcPort().SetValues(randRange(5555, 10000))
	udp.DstPort().SetValues(randRange(5555, 10000))

	return flow
}

// randRange generates a slice of random uint32 values in the range [0, max).
func randRange(rmax int, rcount int) []uint32 {
	// #nosec G404 -- math/rand is fine for non-crypto randomness
	rand.New(rand.NewSource(time.Now().UnixNano()))
	var result []uint32
	for len(result) < rcount {
		result = append(result, uint32(rand.Intn(rmax)))
	}
	return result
}

// enableCapture enables packet capture on specified OTG ports.
func enableCapture(t *testing.T, otg *otg.OTG, topo gosnappi.Config, otgPortNames []string) {
	t.Helper()
	for _, port := range otgPortNames {
		topo.Captures().Add().SetName(port).SetPortNames([]string{port}).SetFormat(gosnappi.CaptureFormat.PCAP)
	}
	otg.PushConfig(t, topo)
}

// clearCapture clears packet capture from all OTG ports.
func clearCapture(t *testing.T, otg *otg.OTG, topo gosnappi.Config) {
	t.Helper()
	topo.Captures().Clear()
	otg.PushConfig(t, topo)
}

// startCapture starts packet capture on OTG ports.
func startCapture(t *testing.T, ate *ondatra.ATEDevice) {
	t.Helper()
	otg := ate.OTG()
	cs := gosnappi.NewControlState()
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.START)
	otg.SetControlState(t, cs)
}

// stopCapture stops packet capture on OTG ports.
func stopCapture(t *testing.T, ate *ondatra.ATEDevice) {
	t.Helper()
	otg := ate.OTG()
	cs := gosnappi.NewControlState()
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.STOP)
	otg.SetControlState(t, cs)
}

// configStaticArp configures static ARP entries for gRIBI next hop resolution.
func configStaticArp(p, ipaddr, macAddr, protoType string) *oc.Interface {
	i := &oc.Interface{Name: ygot.String(p)}
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	s := i.GetOrCreateSubinterface(0)
	if protoType == "ipv4" {
		s4 := s.GetOrCreateIpv4()
		n4 := s4.GetOrCreateNeighbor(ipaddr)
		n4.LinkLayerAddress = ygot.String(macAddr)
	} else {
		s6 := s.GetOrCreateIpv6()
		n6 := s6.GetOrCreateNeighbor(ipaddr)
		n6.LinkLayerAddress = ygot.String(macAddr)
	}
	return i
}

// staticARPWithSecondaryIP configures secondary IPs and static ARP for gRIBI compatibility.
func staticARPWithSecondaryIP(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	p3 := dut.Port(t, "port3")
	gnmi.Update(t, dut, gnmi.OC().Interface(p3.Name()).Config(), dutPort3DummyIP.NewOCInterface(p3.Name(), dut))
	gnmi.Update(t, dut, gnmi.OC().Interface(p3.Name()).Config(), configStaticArp(p3.Name(), otgPort3DummyIP.IPv4, dstMac, "ipv4"))

	p4 := dut.Port(t, "port4")
	gnmi.Update(t, dut, gnmi.OC().Interface(p4.Name()).Config(), dutPort4DummyIP.NewOCInterface(p4.Name(), dut))
	gnmi.Update(t, dut, gnmi.OC().Interface(p4.Name()).Config(), configStaticArp(p4.Name(), otgPort4DummyIP.IPv4, dstMac, "ipv4"))
}

// testLoadBalance to ensure 50:50 Load Balancing.
func testLoadBalance(t *testing.T, ate *ondatra.ATEDevice, portNames []string) []uint64 {
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
