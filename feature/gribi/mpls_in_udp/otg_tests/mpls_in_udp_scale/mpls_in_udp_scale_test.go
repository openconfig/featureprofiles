package mpls_in_udp_scale_test

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"math/rand"
	"os"
	"slices"
	"sort"
	"strings"
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
	"github.com/openconfig/featureprofiles/internal/helpers"
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
	numVRFs         = 1023
	vlanCount       = numVRFs // As per the README, the VLAN count is 16. However, since we do not support configuring multiple VRFs on the same VLAN or interface, the count has been set to match the number of VRFs instead.
	trafficDuration = 15 * time.Second
	nextHopID       = uint64(10001)
	nextHopGroupID  = uint64(20001)
	outerIPv6Src    = "2001:db8:300::1"
	outerIPv6Dst    = "6001:0:0:1::1"
	outerDstUDPPort = 6635
	outerDSCP       = 26
	outerTTL        = 64
	innerTTL        = uint32(100)
	mplsLabel       = 100
	v4PrefixLen     = 30
	v6PrefixLen     = 126
	routeV4PrfLen   = "/32"
	routeV6PrfLen   = "/128"
	tolerance       = 5
	ratePPS         = 10000
	pktSize         = 256
	mtu             = 9126
	dutV4AddrPfx1   = "192.51.0.1"
	ateV4AddrPfx1   = "192.51.0.2"
	dutV4AddrPfx2   = "193.51.0.1"
	ateV4AddrPfx2   = "193.51.0.2"
	dutV6AddrPfx1   = "4001:0:0:1::1"
	ateV6AddrPfx1   = "4001:0:0:1::2"
	dutV6AddrPfx2   = "5001:0:0:1::1"
	ateV6AddrPfx2   = "5001:0:0:1::2"
	dstIP           = "192.168.1.1"
	dstMac          = "02:00:00:00:00:01"
	incCount        = 16
	baseIPv6        = "3001:0:0:1::1" // starting prefix for IPv6
	baseIPv4        = "198.51.0.1"    // starting prefix for IPv4
	intStepV6       = "0:0:0:1::"
	intStepV4       = "0.0.0.4"
	totalPrefixes   = 20000
	nhBaseValue     = uint64(1000) // will be offset per VRF & per port
	nhgBaseValue    = uint64(5000)
)

// DUT and ATE port attributes
var (
	dutPort1 = attrs.Attributes{Desc: "DUT Ingress Port1", IPv4: "192.51.100.1", IPv4Len: v4PrefixLen, IPv6: "4001:db8:1::1", IPv6Len: v6PrefixLen}
	dutPort2 = attrs.Attributes{Desc: "DUT Ingress Port2", IPv4: "193.51.100.1", IPv4Len: v4PrefixLen, IPv6: "5001:db8:1::1", IPv6Len: v6PrefixLen}
	dutPort3 = attrs.Attributes{Desc: "DUT Egress Port3", IPv4: "194.51.100.1", IPv4Len: v4PrefixLen, IPv6: "4002:db8:1::1", IPv6Len: v6PrefixLen, MTU: mtu}
	dutPort4 = attrs.Attributes{Desc: "DUT Egress Port4", IPv4: "195.51.100.1", IPv4Len: v4PrefixLen, IPv6: "5002:db8:1::1", IPv6Len: v6PrefixLen, MTU: mtu}

	atePort1 = attrs.Attributes{Name: "ATE-Ingress-Port1", MAC: "02:02:02:00:00:01", IPv4: "192.51.100.2", IPv4Len: v4PrefixLen, IPv6: "2001:db8:1::2", IPv6Len: v6PrefixLen}
	atePort2 = attrs.Attributes{Name: "ATE-Ingress-Port2", MAC: "02:04:04:00:00:01", IPv4: "193.51.100.2", IPv4Len: v4PrefixLen, IPv6: "2002:db8:1::2", IPv6Len: v6PrefixLen}
	atePort3 = attrs.Attributes{Name: "ATE-Egress-Port3", MAC: "02:06:06:00:00:01", IPv4: "194.51.100.2", IPv4Len: v4PrefixLen, IPv6: "2003:db8:1::2", IPv6Len: v6PrefixLen, MTU: mtu}
	atePort4 = attrs.Attributes{Name: "ATE-Egress-Port4", MAC: "02:08:08:00:00:01", IPv4: "195.51.100.2", IPv4Len: v4PrefixLen, IPv6: "2004:db8:1::2", IPv6Len: v6PrefixLen, MTU: mtu}

	dutPort3DummyIP = attrs.Attributes{Desc: "dutPort3", IPv4Sec: "192.0.2.21", IPv4LenSec: v4PrefixLen}

	otgPort3DummyIP = attrs.Attributes{Desc: "otgPort3", IPv4: "192.0.2.22", IPv4Len: v4PrefixLen}

	dutPort4DummyIP = attrs.Attributes{Desc: "dutPort4", IPv4Sec: "193.0.2.21", IPv4LenSec: v4PrefixLen}

	otgPort4DummyIP = attrs.Attributes{Desc: "otgPort4", IPv4: "193.0.2.22", IPv4Len: v4PrefixLen}
	// IPv6 flow configuration for MPLS-in-UDP testing
	fa6 = flowAttr{
		src:      atePort1.IPv6,
		srcMac:   atePort1.MAC,
		srcPort:  []string{"port1", "port1"},
		dstPorts: []string{"port3", "port4"},
		topo:     gosnappi.NewConfig(),
	}
	fa4 = flowAttr{
		src:      atePort1.IPv4,
		srcMac:   atePort1.MAC,
		srcPort:  []string{"port1", "port1"},
		dstPorts: []string{"port3", "port4"},
		topo:     gosnappi.NewConfig(),
	}
	portsTrafficDistribution = []uint64{50, 50}
	profiles                 = map[vrfProfile]profileConfig{
		profileSingleVRF:      {20000, 1}, // 20k NHGs × 1 NH
		profileMultiVRF:       {20000, 1}, // same as profile 1
		profileMultiVRFSkew:   {20000, 1}, // same as profile 1
		profileSingleVRFECMP:  {2500, 8},  // 2.5k NHGs × 8 NHs = 20k NHs
		profileSingleVRFgRIBI: {20000, 1}, // QPS scaling
	}
	tDstLists   = []string{}
	labelLists  = []int{}
	pfx1V6Lists = []string{}
)

type profileConfig struct {
	totalNHGs int
	nhsPerNHG int
}

// flowAttr defines traffic flow attributes for test packets.
type flowAttr struct {
	src      string   // source IP address
	srcPort  []string // source OTG ports
	dstPorts []string // destination OTG ports
	srcMac   string   // source MAC address
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
	vrfsList := cfgplugins.CreateVRFs(t, dut, vrfBatch, cfgplugins.VRFConfig{VRFCount: numVRFs})
	portList := []*ondatra.Port{dp1, dp2, dp3, dp4}
	dutPortAttrs := []attrs.Attributes{dutPort1, dutPort2, dutPort3, dutPort4}

	for idx, a := range dutPortAttrs {
		p := portList[idx]
		intf := a.NewOCInterface(p.Name(), dut)
		// Vendors for which the LR4 PMD workaround/logic MUST apply
		applyPMDVendors := map[ondatra.Vendor]bool{
			ondatra.ARISTA:  true,
			ondatra.CISCO:   false,
			ondatra.JUNIPER: false,
			// Add more if needed
		}
		// Vendor/PMD-specific port speed configuration
		if p.PMD() == ondatra.PMD100GBASELR4 && applyPMDVendors[dut.Vendor()] {
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
	if deviations.InterfaceConfigVRFBeforeAddress(dut) {
		t.Log("Configure/update Network Instance")
		dutConfNIPath := d.NetworkInstance(deviations.DefaultNetworkInstance(dut))
		gnmi.Replace(t, dut, dutConfNIPath.Type().Config(), oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)
	}
	// configure 16 L3 subinterfaces under DUT port#1 & 2 and assign them to DEFAULT vrf
	configureDUTSubinterfaces(t, vrfBatch, new(oc.Root), dut, dp1, dutV4AddrPfx1, dutV6AddrPfx1, startVLANPort1, true)
	configureDUTSubinterfaces(t, vrfBatch, new(oc.Root), dut, dp2, dutV4AddrPfx2, dutV6AddrPfx2, startVLANPort2, false)

	// configure an L3 subinterface without vlan tagging under DUT port#3 & 4
	createDUTSubinterface(t, vrfBatch, new(oc.Root), dut, dp3, 0, 0, dutPort3.IPv4, dutPort3.IPv6)
	createDUTSubinterface(t, vrfBatch, new(oc.Root), dut, dp4, 0, 0, dutPort4.IPv4, dutPort4.IPv6)
	for indx := 0; indx < numVRFs+1; indx++ {
		if indx != 0 {
			cfgplugins.AssignInterfaceToNetworkInstance(t, vrfBatch, dut, dp1.Name(), &cfgplugins.NetworkInstanceParams{Name: fmt.Sprintf("VRF_%04d", indx), Default: false}, uint32(indx))
			cfgplugins.AssignInterfaceToNetworkInstance(t, vrfBatch, dut, dp2.Name(), &cfgplugins.NetworkInstanceParams{Name: fmt.Sprintf("VRF_%04d", indx), Default: false}, uint32(indx))
		}
	}
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
	a4.PrefixLength = ygot.Uint8(uint8(v4PrefixLen))
	if deviations.InterfaceEnabled(dut) {
		s4.Enabled = ygot.Bool(true)
	}
	s6 := s.GetOrCreateIpv6()
	a6 := s6.GetOrCreateAddress(ipv6Addr)
	a6.PrefixLength = ygot.Uint8(uint8(v6PrefixLen))
	if deviations.InterfaceEnabled(dut) {
		s6.Enabled = ygot.Bool(true)
	}
	gnmi.BatchUpdate(vrfBatch, gnmi.OC().Interface(ifName).Subinterface(index).Config(), s)
}

// configureDUTSubinterfaces creates and configures multiple subinterfaces (up to 16) on the given DUT port. Each subinterface is assigned a VLAN ID, IPv4, and IPv6 address, and all configurations are staged into the provided GNMI SetBatch.
func configureDUTSubinterfaces(t *testing.T, vrfBatch *gnmi.SetBatch, d *oc.Root, dut *ondatra.DUTDevice, dutPort *ondatra.Port, prefixFmtV4, prefixFmtV6 string, startVLANPort int, pfx bool) {
	t.Helper()
	// The 32 logical ingress interfaces (16 VLANs × 2 ports) are mapped to VRFs as per scale profiles for traffic classification and forwarding validation.
	// Each VLAN subinterface is configured with both IPv4 and IPv6 addresses derived from prefixFmtV4 and prefixFmtV6 patterns.
	dutIPs, err := iputil.GenerateIPsWithStep(prefixFmtV4, vlanCount, intStepV4)
	if err != nil {
		t.Fatalf("failed to generate DUT IPs: %v", err)
	}
	dutIPsV6, err := iputil.GenerateIPv6sWithStep(prefixFmtV6, vlanCount, intStepV6)
	if err != nil {
		t.Fatalf("failed to generate DUT IPv6s: %v", err)
	}
	if pfx {
		pfx1V6Lists = dutIPsV6
	}
	for i := range vlanCount {
		index := uint32(i + 1)
		vlanID := uint16(startVLANPort + i)
		if deviations.NoMixOfTaggedAndUntaggedSubinterfaces(dut) {
			vlanID++
		}
		createDUTSubinterface(t, vrfBatch, d, dut, dutPort, index, vlanID, dutIPs[i], dutIPsV6[i])
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
	// Only add VLAN if vlanID > 0 (untagged physical interface otherwise)
	if vlanID > 0 {
		eth.Vlans().Add().SetName(Name).SetId(uint32(vlanID))
	}
	eth.Ipv4Addresses().Add().SetName(Name + ".IPv4").SetAddress(ateIPv4).SetGateway(dutIPv4).SetPrefix(uint32(v4PrefixLen))
	eth.Ipv6Addresses().Add().SetName(Name + ".IPv6").SetAddress(ateIPv6).SetGateway(dutIPv6).SetPrefix(uint32(v6PrefixLen))
}

// mustConfigureATESubinterfaces configures 16 ATE subinterfaces on the target device It returns a slice of the corresponding ATE IPAddresses.
func mustConfigureATESubinterfaces(t *testing.T, ateConfig gosnappi.Config, atePort *ondatra.Port, dut *ondatra.DUTDevice, Name, Mac, dutPfxFmtV4, atePfxFmtV4, dutPfxFmtV6, atePfxFmtV6 string, startVLANPort int) []string {
	t.Helper()
	interfaceNamesList := []string{}
	dutIPsV6, err := iputil.GenerateIPv6sWithStep(dutPfxFmtV6, vlanCount, intStepV6)
	if err != nil {
		t.Fatalf("failed to generate DUT IPv6s: %v", err)
	}
	otgIPsV6, err := iputil.GenerateIPv6sWithStep(atePfxFmtV6, vlanCount, intStepV6)
	if err != nil {
		t.Fatalf("failed to generate OTG IPv6s: %v", err)
	}
	dutIPsV4, err := iputil.GenerateIPsWithStep(dutPfxFmtV4, vlanCount, intStepV4)
	if err != nil {
		t.Fatalf("failed to generate DUT IPv4s: %v", err)
	}
	otgIPsV4, err := iputil.GenerateIPsWithStep(atePfxFmtV4, vlanCount, intStepV4)
	if err != nil {
		t.Fatalf("failed to generate OTG IPv4s: %v", err)
	}
	for i := range vlanCount {
		vlanID := uint16(startVLANPort + i)
		if deviations.NoMixOfTaggedAndUntaggedSubinterfaces(dut) {
			vlanID++
		}

		name := fmt.Sprintf("%s-%d", Name, i)
		mac, err := iputil.IncrementMAC(Mac, i+1)
		if err != nil {
			t.Errorf("%s", err)
		}
		interfaceNamesList = append(interfaceNamesList, name)
		createATEDevice(t, ateConfig, atePort, vlanID, name, mac, dutIPsV4[i], otgIPsV4[i], dutIPsV6[i], otgIPsV6[i])
	}

	return interfaceNamesList
}

// configureOTG sets up the ATE topology across 4 physical ports, including VLAN subinterfaces, IP addressing, and device-level configs. It also applies Layer1 link settings for 100GBASE-LR4 PMD ports with auto-negotiation and disables RS-FEC if required.
func configureOTG(t *testing.T, ate *ondatra.ATEDevice, dut *ondatra.DUTDevice) (gosnappi.Config, []string) {
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

	createATEDevice(t, ateConfig, ap1, 0, atePort1.Name, atePort1.MAC, dutPort1.IPv4, atePort1.IPv4, dutPort1.IPv6, atePort1.IPv6)
	createATEDevice(t, ateConfig, ap2, 0, atePort2.Name, atePort2.MAC, dutPort2.IPv4, atePort2.IPv4, dutPort2.IPv6, atePort2.IPv6)
	createATEDevice(t, ateConfig, ap3, 0, atePort3.Name, atePort3.MAC, dutPort3.IPv4, atePort3.IPv4, dutPort3.IPv6, atePort3.IPv6)
	createATEDevice(t, ateConfig, ap4, 0, atePort4.Name, atePort4.MAC, dutPort4.IPv4, atePort4.IPv4, dutPort4.IPv6, atePort4.IPv6)
	// subIntfIPs is a []string slice with ATE IPv6 addresses for all the subInterfaces
	interfaceNamesList := mustConfigureATESubinterfaces(t, ateConfig, ap1, dut, atePort1.Name, atePort1.MAC, dutV4AddrPfx1, ateV4AddrPfx1, dutV6AddrPfx1, ateV6AddrPfx1, startVLANPort1)
	mustConfigureATESubinterfaces(t, ateConfig, ap2, dut, atePort2.Name, atePort2.MAC, dutV4AddrPfx2, ateV4AddrPfx2, dutV6AddrPfx2, ateV6AddrPfx2, startVLANPort2)
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
	return ateConfig, interfaceNamesList
}

// programBasicEntries installs basic NextHop and NextHopGroup entries to set up ECMP forwarding across port3 and port4, along with an IPv6 route to test MPLS-in-UDP tunnels.
func programBasicEntries(t *testing.T, dut *ondatra.DUTDevice, c *gribi.Client, vrfs []string) {
	t.Helper()
	t.Log("Setting up routing infrastructure for MPLS-in-UDP with unique NH IDs and NH/NHG in default NI")

	otgDummyIPs := map[string]string{
		dut.Port(t, "port3").Name(): otgPort3DummyIP.IPv4,
		dut.Port(t, "port4").Name(): otgPort4DummyIP.IPv4,
	}

	defaultNI := deviations.DefaultNetworkInstance(dut)
	tunnelPrefix, err := iputil.GenerateIPv6sWithStep(outerIPv6Dst, len(vrfs), intStepV6)
	if err != nil {
		t.Fatalf("failed to generate DUT IPv6s: %v", err)
	}
	tDstLists = tunnelPrefix
	// iterate VRFs and program unique NH, NHG in default NI; program IPv6 prefix in the VRF
	for vrfIdx := range vrfs {
		// generate unique NH IDs for the two egress ports for this VRF
		nhIDs := []uint64{
			nhBaseValue + uint64(vrfIdx*5) + 0, // port3
			nhBaseValue + uint64(vrfIdx*5) + 1, // port4
		}
		nhgID := nhgBaseValue + uint64(vrfIdx) // unique NHG per VRF

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
		labelLists = append(labelLists, mplsLabel+vrfIdx)
		c.AddIPv6(t, tunnelPrefix[vrfIdx]+routeV6PrfLen, nhgID, defaultNI, defaultNI, fluent.InstalledInFIB)
	}

	t.Log("programBasicEntries completed")
}

// programMPLSinUDPEntries programs gRIBI entries for MPLS-in-UDP encapsulation. It installs a single NextHop that performs MPLS-in-UDP encapsulation with the provided outer IPv6 and UDP header attributes, associates multiple NextHopGroups (NHGs) with that NextHop, and finally installs IPv6 /128 routes pointing to each NHG.
func programMPLSinUDPEntries(t *testing.T, dut *ondatra.DUTDevice, nextHopID, nhgBase, mplsLabelStart uint64, numNHGs int, vrfs []string, outerIPv6Src, outerIPv6Dst string, outerDstUDPPort uint16, outerTTL, outerDSCP uint8) ([]fluent.GRIBIEntry, map[string][]string) {
	t.Helper()
	entries := make([]fluent.GRIBIEntry, 0, 1+2*numNHGs*len(vrfs))
	vrfV6PfxMap := make(map[string][]string)
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
	// nhIndex := 0
	for vrfIdx := range vrfs {
		for i := 0; i < numNHGs; i++ {
			// Unique NHG per (vrf, i):
			nhgID := nhgBase + uint64(vrfIdx*numNHGs+i)

			// Add NHG referencing the single nextHopID
			entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).WithID(nhgID).AddNextHop(nextHopID, 1))
		}
	}

	return entries, vrfV6PfxMap
}

// expectedMPLSinUDPOpResults builds expected gRIBI OpResults for Profile 5. It models the baseline state of one MPLS-in-UDP NH, all NHGs, and 20k route entries (10k IPv4 + 10k IPv6).
func expectedMPLSinUDPOpResults(t *testing.T, nextHopID, nhgBase uint64, numNHGs, totalPrefixes int, baseIPv4, baseIPv6 string, vrfs []string) (adds, dels []*client.OpResult) {
	t.Helper()

	// totalNHGs := numNHGs * len(vrfs)

	adds = make([]*client.OpResult, 0)
	dels = make([]*client.OpResult, 0)

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

	// Step 3: IPv4 routes (num per VRF)
	numV4 := totalPrefixes / 2
	if numV4 > 0 {
		totalV4 := numV4 * len(vrfs)
		ipv4List, err := iputil.GenerateIPsWithStep(baseIPv4, totalV4, intStepV4)
		if err != nil {
			t.Fatalf("GenerateIPsWithStep(v4) failed: %v", err)
		}

		for vrfIdx := range vrfs {
			for i := 0; i < numV4; i++ {
				idx := vrfIdx*numV4 + i
				prefix := ipv4List[idx] + routeV4PrfLen

				adds = append(adds, fluent.OperationResult().WithIPv4Operation(prefix).WithProgrammingResult(fluent.InstalledInFIB).WithOperationType(constants.Add).AsResult())
				dels = append(dels, fluent.OperationResult().WithIPv4Operation(prefix).WithProgrammingResult(fluent.InstalledInFIB).WithOperationType(constants.Delete).AsResult())
			}
		}
	}

	// Step 4: IPv6 routes (num per VRF)
	numV6 := totalPrefixes - numV4
	if numV6 > 0 {
		totalV6 := numV6 * len(vrfs)
		ipv6List, err := iputil.GenerateIPv6sWithStep(baseIPv6, totalV6, intStepV6)
		if err != nil {
			t.Fatalf("GenerateIPv6sWithStep(v6) failed: %v", err)
		}

		for vrfIdx := range vrfs {
			for i := 0; i < numV6; i++ {
				idx := vrfIdx*numV6 + i
				prefix := ipv6List[idx] + routeV6PrfLen

				adds = append(adds, fluent.OperationResult().WithIPv6Operation(prefix).WithProgrammingResult(fluent.InstalledInFIB).WithOperationType(constants.Add).AsResult())
				dels = append(dels, fluent.OperationResult().WithIPv6Operation(prefix).WithProgrammingResult(fluent.InstalledInFIB).WithOperationType(constants.Delete).AsResult())
			}
		}
	}

	// Step 5: Reverse deletes (routes → NHGs → NH)
	slices.Reverse(dels)

	return adds, dels
}

// programMPLSinUDPMultiEntries programs MPLS-in-UDP encapsulated NextHops, NextHopGroups, and IPv6 routes across multiple VRFs. It returns a slice of gRIBI entries that can be pushed to the DUT using the fluent gRIBI client.
func programMPLSinUDPMultiEntries(t *testing.T, dut *ondatra.DUTDevice, vrfs []string, mplsNHBase, nhgBase, mplsLabelStart uint64, numNHGs int, outerIPv6Src, outerIPv6Dst string, outerDstUDPPort uint16, outerTTL, outerDSCP uint8) ([]fluent.GRIBIEntry, map[string][]string) {
	t.Helper()
	vrfMultiPfxMap := make(map[string][]string)
	defaultNI := deviations.DefaultNetworkInstance(dut)

	totalPrefixes := len(vrfs) * numNHGs

	// Pre-generate all prefixes
	prefixIPv6s, err := iputil.GenerateIPv6sWithStep(baseIPv6, totalPrefixes, intStepV6)
	if err != nil {
		t.Fatalf("failed to generate prefixIPv6s: %v", err)
	}

	// Per-VRF outer destination IP
	vrfOuterDst, err := iputil.GenerateIPv6sWithStep(outerIPv6Dst, len(vrfs), intStepV6)
	if err != nil {
		t.Fatalf("failed to generate vrfOuterDst IPv6s: %v", err)
	}

	// Pre-size entries slice: (1 NH + numNHGs*2) per VRF
	totalEntries := len(vrfs) * (1 + numNHGs*2)
	entries := make([]fluent.GRIBIEntry, 0, totalEntries)

	idx := 0 // prefix index

	for vrfIdx, vrfName := range vrfs {

		// === 1) One NH per VRF ===
		nhID := mplsNHBase + uint64(vrfIdx)
		label := mplsLabelStart + uint64(vrfIdx)

		entries = append(entries,
			fluent.
				NextHopEntry().
				WithNetworkInstance(defaultNI).
				WithIndex(nhID).
				AddEncapHeader(
					fluent.MPLSEncapHeader().WithLabels(label),
					fluent.UDPV6EncapHeader().
						WithSrcIP(outerIPv6Src).
						WithDstIP(vrfOuterDst[vrfIdx]).
						WithDstUDPPort(uint64(outerDstUDPPort)).
						WithIPTTL(uint64(outerTTL)).
						WithDSCP(uint64(outerDSCP)),
				),
		)

		// === 2) For each NHG in this VRF ===
		for i := 0; i < numNHGs; i++ {
			nhgID := nhgBase + uint64(vrfIdx*numNHGs+i)

			// NHG
			entries = append(entries,
				fluent.NextHopGroupEntry().
					WithNetworkInstance(defaultNI).
					WithID(nhgID).
					AddNextHop(nhID, 1),
			)

			// Route
			pfx := prefixIPv6s[idx]
			vrfMultiPfxMap[vrfName] = append(vrfMultiPfxMap[vrfName], pfx)
			prefixStr := pfx + routeV6PrfLen

			entries = append(entries,
				fluent.IPv6Entry().
					WithNetworkInstance(vrfName).
					WithPrefix(prefixStr).
					WithNextHopGroup(nhgID).
					WithNextHopGroupNetworkInstance(defaultNI),
			)

			idx++
		}
	}

	return entries, vrfMultiPfxMap
}

// expectedMPLSinUDPMultiOpResults builds the expected set of Add/Delete OperationResults for MPLS-in-UDP multi-VRF programming.
func expectedMPLSinUDPMultiOpResults(t *testing.T, vrfs []string, mplsNHBase, nhgBase uint64, numNHGs int) (adds, dels []*client.OpResult) {
	t.Helper()

	// Total prefixes = VRFs × NHGs per VRF
	totalPrefixes := len(vrfs) * numNHGs

	prefixIPv6s, err := iputil.GenerateIPv6sWithStep(baseIPv6, totalPrefixes, intStepV6)
	if err != nil {
		t.Fatalf("failed to generate IPv6s: %v", err)
	}

	// Pre-size: 1 NH + 2 per NHG (NHG add + route add)
	totalOps := len(vrfs) * (1 + 2*numNHGs)
	adds = make([]*client.OpResult, 0, totalOps)
	dels = make([]*client.OpResult, 0, totalOps)

	idx := 0

	// === 1) NH results ===
	for vrfIdx := range vrfs {
		nhID := mplsNHBase + uint64(vrfIdx)

		adds = append(adds, fluent.OperationResult().WithNextHopOperation(nhID).WithProgrammingResult(fluent.InstalledInFIB).WithOperationType(constants.Add).AsResult())
		dels = append(dels, fluent.OperationResult().WithNextHopOperation(nhID).WithProgrammingResult(fluent.InstalledInFIB).WithOperationType(constants.Delete).AsResult())
	}

	// === 2) NHG + IPv6 route results ===
	for vrfIdx := range vrfs {
		for i := 0; i < numNHGs; i++ {

			nhgID := nhgBase + uint64(vrfIdx*numNHGs+i)

			// NHG add/del
			adds = append(adds, fluent.OperationResult().WithNextHopGroupOperation(nhgID).WithProgrammingResult(fluent.InstalledInFIB).WithOperationType(constants.Add).AsResult())
			dels = append(dels, fluent.OperationResult().WithNextHopGroupOperation(nhgID).WithProgrammingResult(fluent.InstalledInFIB).WithOperationType(constants.Delete).AsResult())

			// Route add/del
			pfx := prefixIPv6s[idx]
			prefixStr := pfx + routeV6PrfLen

			adds = append(adds, fluent.OperationResult().WithIPv6Operation(prefixStr).WithProgrammingResult(fluent.InstalledInFIB).WithOperationType(constants.Add).AsResult())
			dels = append(dels, fluent.OperationResult().WithIPv6Operation(prefixStr).WithProgrammingResult(fluent.InstalledInFIB).WithOperationType(constants.Delete).AsResult())

			idx++
		}
	}

	// Reverse delete operations to match LIFO order (Route -> NHG -> NH)
	slices.Reverse(dels)
	return adds, dels
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
	defer tmpFile.Close()
	if _, err := tmpFile.Write(packetBytes); err != nil {
		return fmt.Errorf("could not write packet data: %v", err)
	}

	handle, err := pcap.OpenOffline(tmpFile.Name())
	if err != nil {
		return fmt.Errorf("could not open pcap file: %v", err)
	}
	defer handle.Close()

	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())

	seenLabels := make(map[uint64]bool, len(labelList))
	for _, l := range labelList {
		seenLabels[l] = true
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
		if !slices.Contains(tDstLists, v6.DstIP.String()) {
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
		if len(seenLabels) > 0 {
			if !seenLabels[uint64(label)] {
				return fmt.Errorf("packet %d: got MPLS label %d, not in %v", packetCount, label, labelList)
			}
		} else if !slices.Contains(labelLists, int(label)) {
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
	t.Logf("Total packets: %d, UDP-IPv6: %d, Valid MPLS-in-UDP: %d", packetCount, mplsPacketCount, validMplsPacketCount)

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
			return fmt.Errorf("too many invalid packets: %d/%d (allowed %d failures = %.2f%%)", invalid, mplsPacketCount, allowedFailures, allowedLossPct)
		}
	}
	t.Logf("Validation PASSED: %d valid MPLS-in-UDP packets", validMplsPacketCount)
	return nil
}

// newGRIBIClient create gRIBI session.
func mustNewGRIBIClient(t *testing.T, dut *ondatra.DUTDevice) *gribi.Client {
	t.Helper()

	c := &gribi.Client{
		DUT:         dut,
		FIBACK:      true,
		Persistence: true,
	}

	if err := c.Start(t); err != nil {
		t.Fatalf("gRIBI connection failed: %v", err)
	}

	c.BecomeLeader(t)
	return c
}

// Main test entry point.
func TestMPLSinUDPScale(t *testing.T) {
	t.Helper()
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	ctx := context.Background()
	vrfsList := configureDUT(t, dut)
	ateConfig, interfaceNamesList := configureOTG(t, ate, dut)
	ate.OTG().PushConfig(t, ateConfig)
	ate.OTG().StartProtocols(t)
	// Limiting it to 50 since checking ARP for 1024 interfaces takes long time
	ifs := interfaceNamesList
	if len(ifs) >= 50 {
		ifs = ifs[:50]
	}
	cfgplugins.IsIPv4InterfaceARPresolved(t, ate, cfgplugins.AddressFamilyParams{InterfaceNames: ifs})
	cfgplugins.IsIPv6InterfaceARPresolved(t, ate, cfgplugins.AddressFamilyParams{InterfaceNames: ifs})

	dstMac := gnmi.Get(t, dut, gnmi.OC().Interface(dut.Port(t, "port1").Name()).Ethernet().MacAddress().State())
	// Disable hardware nexthop proxying for Arista devices to ensure FIB-ACK works correctly.
	// See: https://partnerissuetracker.corp.google.com/issues/422275961
	if deviations.DisableHardwareNexthopProxy(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			const aristaDisableNHGProxyCLI = "ip hardware fib next-hop proxy disabled"
			helpers.GnmiCLIConfig(t, dut, aristaDisableNHGProxyCLI)
		default:
			t.Errorf("Deviation DisableHardwareNexthopProxy is not handled for the dut: %v", dut.Vendor())
		}
	}
	c := mustNewGRIBIClient(t, dut)
	t.Cleanup(func() {
		c.FlushAll(t)
		c.Close(t)
	})
	programBasicEntries(t, dut, c, vrfsList)
	t.Run("Profile-1-Single VRF", func(t *testing.T) {
		if err := configureVRFProfiles(ctx, t, ateConfig, dut, ate, c, []string{deviations.DefaultNetworkInstance(dut), vrfsList[1]}, profileSingleVRF, false, dstMac); err != nil {
			t.Errorf("configureVrfProfiles failed: %v", err)
		}
	})
	t.Run("Profile-2-Multi VRF", func(t *testing.T) {
		if err := configureVRFProfiles(ctx, t, ateConfig, dut, ate, c, vrfsList, profileMultiVRF, true, dstMac); err != nil {
			t.Errorf("configureVrfProfiles failed: %v", err)
		}
	})
	t.Run("Profile-3-Multi VRF with Skew", func(t *testing.T) {
		if err := configureVRFProfiles(ctx, t, ateConfig, dut, ate, c, vrfsList, profileMultiVRFSkew, true, dstMac); err != nil {
			t.Errorf("configureVrfProfiles failed: %v", err)
		}
	})
	t.Run("Profile-4-Single VRF", func(t *testing.T) {
		if err := configureVRFProfiles(ctx, t, ateConfig, dut, ate, c, []string{deviations.DefaultNetworkInstance(dut), vrfsList[1]}, profileSingleVRFECMP, false, dstMac); err != nil {
			t.Errorf("configureVrfProfiles failed: %v", err)
		}
	})
	t.Run("Profile-5-Single VRF", func(t *testing.T) {
		if err := configureVRFProfiles(ctx, t, ateConfig, dut, ate, c, []string{deviations.DefaultNetworkInstance(dut), vrfsList[1]}, profileSingleVRFgRIBI, false, dstMac); err != nil {
			t.Errorf("configureVrfProfiles failed: %v", err)
		}
	})
}

// configureVRFProfiles implements the “Single/Multi VRF Validation” for Profile 1 (baseline) and Profile 4 (ECMP). It programs MPLS-in-UDP NHs, NHGs, and 20k prefixes (10k v4 + 10k v6), validates FIB/AFT, sends traffic, checks MPLS-over-UDP encapsulation, and deletes entries.
func configureVRFProfiles(ctx context.Context, t *testing.T, ateConfig gosnappi.Config, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, c *gribi.Client, vrfs []string, profile vrfProfile, otgMultiPortCaptureSupported bool, dstmac string) error {
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
		vrfPfxMap map[string][]string
	)
	// === Program MPLS-in-UDP NH & NHG entries ===
	switch profile {
	case profileMultiVRF:
		entries, vrfPfxMap = programMPLSinUDPMultiEntries(t, dut, vrfs, nextHopID, nextHopGroupID, mplsLabel, totalNHGs, outerIPv6Src, outerIPv6Dst, outerDstUDPPort, outerTTL, outerDSCP)
		wantAdds, wantDels = expectedMPLSinUDPMultiOpResults(t, vrfs, nextHopID, nextHopGroupID, totalNHGs)
		flows = append(flows, fa6.createFlow("ipv6", fmt.Sprintf("ip6mpls_p%d", profile), dstmac, fa6.srcPort[0], outerDSCP, startVLANPort1, vrfPfxMap)...)
	case profileMultiVRFSkew:
		entries, vrfPfxMap = programMPLSinUDPSkewedEntries(t, dut, vrfs, nextHopID, nextHopGroupID, mplsLabel, totalNHGs, outerIPv6Src, outerIPv6Dst, outerDstUDPPort, outerTTL, outerDSCP)
		wantAdds, wantDels = expectedMPLSinUDPSkewedResults(t, vrfs, nextHopID, nextHopGroupID, totalNHGs)
		flows = append(flows, fa6.createFlow("ipv6", fmt.Sprintf("ip6mpls_p%d", profile), dstmac, fa6.srcPort[0], outerDSCP, startVLANPort1, vrfPfxMap)...)
	default:
		// Profile 1 (single VRF), Profile 4 (ECMP) and Profile 5 fall here
		entries, vrfPfxMap = programMPLSinUDPEntries(t, dut, nextHopID, nextHopGroupID, mplsLabel, totalNHGs, vrfs, outerIPv6Src, outerIPv6Dst, outerDstUDPPort, outerTTL, outerDSCP)
		// === Add IPv4 + IPv6 route entries ===
		ipv4Entries, vrfV4PfxMap := buildIPv4Routes(t, dut, totalPrefixes/2, baseIPv4, vrfs, nextHopGroupID)
		ipv6Entries := buildIPv6Routes(t, dut, totalPrefixes/2, baseIPv6, vrfs, nextHopGroupID)
		entries = append(entries, ipv4Entries...)
		entries = append(entries, ipv6Entries...)
		// === Expected OpResults ===
		wantAdds, wantDels = expectedMPLSinUDPOpResults(t, nextHopID, nextHopGroupID, totalNHGs, totalPrefixes, baseIPv4, baseIPv6, vrfs)
		flows = append(flows, append(fa4.createFlow("ipv4", fmt.Sprintf("ip4mpls_p%d", profile), dstmac, fa4.srcPort[0], outerDSCP, startVLANPort1, vrfV4PfxMap), fa6.createFlow("ipv6", fmt.Sprintf("ip6mpls_p%d", profile), dstmac, fa6.srcPort[0], outerDSCP, startVLANPort1, vrfPfxMap)...)...)
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
	// === Verify infra installed ===
	if err := c.AwaitTimeout(ctx, t, 10*time.Minute); err != nil {
		return fmt.Errorf("Failed to install infra entries for profile %d: %v", profile, err)
	}
	// === Capture & Send Traffic ===
	expectedPkt := &packetResult{
		mplsLabel:  testCaseArgs.wantMPLSLabel,
		udpDstPort: testCaseArgs.wantOuterDstUDPPort,
		ipTTL:      testCaseArgs.wantOuterTTL,
		srcIP:      testCaseArgs.wantOuterSrcIP,
		dstIP:      testCaseArgs.wantOuterDstIP,
	}
	// clearCapture(t, ate.OTG(), ateConfig)
	if otgMultiPortCaptureSupported {
		clearCapture(t, ate.OTG(), ateConfig)
		enableCapture(t, ate.OTG(), ateConfig, testCaseArgs.capturePorts)
		sendTraffic(t, tArgs, testCaseArgs.flows, true)
		err := validateMPLSPacketCapture(t, ate, testCaseArgs.capturePorts[0], expectedPkt, labelList)
		if err != nil {
			return fmt.Errorf("profile %d capture validation failed: %v", profile, err)
		}
	} else {
		for _, port := range testCaseArgs.capturePorts {
			clearCapture(t, ate.OTG(), ateConfig)
			enableCapture(t, ate.OTG(), ateConfig, []string{port})
			sendTraffic(t, tArgs, testCaseArgs.flows, true)
			err := validateMPLSPacketCapture(t, ate, port, expectedPkt, labelList)
			if err != nil {
				return fmt.Errorf("profile %d capture validation failed: %v", profile, err)
			}
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
	// Delete entries in reverse order (Routes -> NHGs -> NHs) to satisfy dependencies.
	slices.Reverse(testCaseArgs.entries)
	c.DeleteEntries(t, testCaseArgs.entries, nil)
	// Validate that traffic fails after deletion (expect loss)
	t.Logf("Verifying traffic fails after MPLS-in-UDP entries deleted for Profile %d", profile)
	if perr := validateTrafficFlows(t, ate, ateConfig, tArgs, testCaseArgs.flows, false, false); perr != nil {
		return fmt.Errorf("profile %d post-delete traffic validation failed: %v", profile, perr)
	}

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

	// batchSize := 10
	for start := 0; start < len(ops); start += targetOps {
		end := start + targetOps
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
func buildIPv4Routes(t *testing.T, dut *ondatra.DUTDevice, num int, baseIPv4 string, vrfs []string, nhgBase uint64) ([]fluent.GRIBIEntry, map[string][]string) {
	t.Helper()
	totalEntries := num * len(vrfs)
	vrfV4PfxMap := make(map[string][]string)
	// Generate `totalEntries` sequential IPv4 addresses
	ipv4List, err := iputil.GenerateIPsWithStep(baseIPv4, totalEntries, intStepV4)
	if err != nil {
		t.Fatalf("failed to generate IPv4 prefixes: %v", err)
	}
	entries := make([]fluent.GRIBIEntry, 0, totalEntries)
	for vrfIdx, vrf := range vrfs {
		for i := 0; i < num; i++ {
			// Correct unique index
			idx := vrfIdx*num + i
			ip := ipv4List[idx]
			prefix := fmt.Sprintf("%s%s", ip, routeV4PrfLen)
			// Store prefix per VRF
			vrfV4PfxMap[vrf] = append(vrfV4PfxMap[vrf], ip)
			nhgID := nhgBase + uint64(idx)
			entries = append(entries, fluent.IPv4Entry().WithNetworkInstance(vrf).WithPrefix(prefix).WithNextHopGroup(nhgID).WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut)))
		}
	}

	return entries, vrfV4PfxMap
}

// buildIPv6Routes generates num IPv6 prefixes per VRF and maps to nhgBase + vrfIdx*num + i.
func buildIPv6Routes(t *testing.T, dut *ondatra.DUTDevice, num int, baseIPv6 string, vrfs []string, nhgBase uint64) []fluent.GRIBIEntry {
	t.Helper()
	totalEntries := num * len(vrfs)
	ipv6List, err := iputil.GenerateIPv6sWithStep(baseIPv6, totalEntries, intStepV6)
	if err != nil {
		t.Fatalf("failed to generate IPv6 prefixes: %v", err)
	}
	entries := make([]fluent.GRIBIEntry, 0, totalEntries)
	for vrfIdx, vrf := range vrfs {
		for i := 0; i < num; i++ {
			idx := vrfIdx*num + i
			ip := ipv6List[idx]
			prefix := fmt.Sprintf("%s%s", ip, routeV6PrfLen)
			nhgID := nhgBase + uint64(idx)
			entries = append(entries, fluent.IPv6Entry().WithNetworkInstance(vrf).WithPrefix(prefix).WithNextHopGroup(nhgID).WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut)))
		}
	}

	return entries
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
func programMPLSinUDPSkewedEntries(t *testing.T, dut *ondatra.DUTDevice, vrfs []string, mplsNHBase, nhgBase, mplsLabelStart uint64, totalNHGs int, outerIPv6Src, outerIPv6Dst string, outerDstUDPPort uint16, outerTTL, outerDSCP uint8) ([]fluent.GRIBIEntry, map[string][]string) {
	t.Helper()
	entries := make([]fluent.GRIBIEntry, 0, totalNHGs*3)
	vrfSkewPfxMap := make(map[string][]string)
	// defaultNI still used for other things; but NH/NHG should be in the VRF NI
	defaultNI := deviations.DefaultNetworkInstance(dut)

	skewPattern := generateSkewPattern(len(vrfs), totalNHGs)
	totalRoutes := 0
	for _, v := range skewPattern {
		totalRoutes += v
	}
	nhIndex := 0
	prefixIPv6s, err := iputil.GenerateIPv6sWithStep(baseIPv6, totalRoutes, intStepV6)
	if err != nil {
		t.Errorf("failed to generate prefixIPv6s: %v", err)
	}
	vrfOuterDst, err := iputil.GenerateIPv6sWithStep(outerIPv6Dst, len(vrfs), intStepV6)
	if err != nil {
		t.Errorf("failed to generate vrfOuterDst IPv6s: %v", err)
	}
	for vrfIdx, vrfName := range vrfs {
		label := mplsLabelStart + uint64(vrfIdx)
		vrfNHID := mplsNHBase + uint64(vrfIdx)
		// Create NH in the same network-instance as the route (vrfName)
		entries = append(entries,
			fluent.NextHopEntry().
				WithNetworkInstance(defaultNI). // create NH in VRF NI
				WithIndex(vrfNHID).
				AddEncapHeader(
					fluent.MPLSEncapHeader().WithLabels(label),
					fluent.UDPV6EncapHeader().
						WithSrcIP(outerIPv6Src).
						WithDstIP(vrfOuterDst[vrfIdx]).
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
			prefixStr := fmt.Sprintf("%s%s", prefixIPv6s[nhIndex], routeV6PrfLen)
			// append prefixIPv6s[index] to the list for this VRF
			vrfSkewPfxMap[vrfName] = append(vrfSkewPfxMap[vrfName], prefixIPv6s[nhIndex])
			entries = append(entries,
				fluent.IPv6Entry().
					WithNetworkInstance(vrfName). // install route in this VRF
					WithPrefix(prefixStr).
					WithNextHopGroup(nhgID).WithNextHopGroupNetworkInstance(defaultNI), // NHG is in same VRF
			)
			nhIndex++
		}
	}

	return entries, vrfSkewPfxMap
}

// expectedMPLSinUDPSkewedResults generates expected add/delete operation results.
func expectedMPLSinUDPSkewedResults(t *testing.T, vrfs []string, mplsNHBase, nhgBase uint64, totalNHGs int) (adds, dels []*client.OpResult) {
	t.Helper()
	adds = []*client.OpResult{}
	dels = []*client.OpResult{}

	skewPattern := generateSkewPattern(len(vrfs), totalNHGs)
	totalRoutes := 0
	for _, v := range skewPattern {
		totalRoutes += v
	}
	t.Logf("skewPattern %d, vrfs %d, totalRoutes %d", len(skewPattern), len(vrfs), totalRoutes)
	nhIndex := 0
	prefixIPv6s, err := iputil.GenerateIPv6sWithStep(baseIPv6, totalRoutes, intStepV6)
	if err != nil {
		t.Errorf("failed to generate IPv6s: %v", err)
	}
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

			// prefixIP := iputil.GenerateIPv6WithOffset(net.ParseIP(baseIPv6), int64(nhIndex))
			prefixStr := fmt.Sprintf("%s%s", prefixIPv6s[nhIndex], routeV6PrfLen)

			addOps = append(addOps, fluent.OperationResult().WithIPv6Operation(prefixStr).WithProgrammingResult(fluent.InstalledInFIB).WithOperationType(constants.Add).AsResult())
			delOps = append(delOps, fluent.OperationResult().WithIPv6Operation(prefixStr).WithProgrammingResult(fluent.InstalledInFIB).WithOperationType(constants.Delete).AsResult())

			adds = append(adds, addOps...)
			dels = append(dels, delOps...)
			nhIndex++
		}
	}

	// Reverse delete operations to match LIFO order (Route -> NHG -> NH)
	slices.Reverse(dels)
	return adds, dels
}

type VRFEntry struct {
	VRF      string
	Prefixes []string
}

func sortedVRFSkewList(vrfSkewList map[string][]string) []VRFEntry {
	vrfNames := make([]string, 0, len(vrfSkewList))

	// collect keys
	for vrf := range vrfSkewList {
		vrfNames = append(vrfNames, vrf)
	}

	// sort VRF names
	sort.Strings(vrfNames)

	// build ordered list
	ordered := make([]VRFEntry, 0, len(vrfNames))
	for _, vrf := range vrfNames {
		prefixes := make([]string, len(vrfSkewList[vrf]))
		copy(prefixes, vrfSkewList[vrf])

		ordered = append(ordered, VRFEntry{
			VRF:      vrf,
			Prefixes: prefixes,
		})
	}

	return ordered
}

// validateTrafficFlows verifies traffic flow behavior (pass/fail) based on expected outcome.
func validateTrafficFlows(t *testing.T, ate *ondatra.ATEDevice, ateConfig gosnappi.Config, args *testArgs, flows []gosnappi.Flow, capture bool, match bool) error {
	t.Helper()
	t.Logf("=== TRAFFIC FLOW VALIDATION START (expecting match=%v) ===", match)

	otg := args.ate.OTG()
	otg.StartTraffic(t)
	time.Sleep(trafficDuration)
	otg.StopTraffic(t)
	otgutils.LogPortMetrics(t, otg, args.topo)

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
	otgutils.LogPortMetrics(t, otg, args.topo)
}

// createFlow creates a traffic flow for MPLS-in-UDP testing.
func (fa *flowAttr) createFlow(flowType, name, dstmac, srcPort string, dscp, vlanID uint32, vrfDataLists map[string][]string) []gosnappi.Flow {
	orderedVRFs := sortedVRFSkewList(vrfDataLists)
	flows := []gosnappi.Flow{}
	for indx, entry := range orderedVRFs {
		flow := fa.topo.Flows().Add().SetName(fmt.Sprintf("%s_%s_%d", name, entry.VRF, indx))
		flow.Metrics().SetEnable(true)
		flow.Rate().SetPps(ratePPS)
		flow.Size().SetFixed(pktSize)
		flow.Duration().FixedPackets().SetPackets(10000)
		flow.TxRx().Port().SetTxName(srcPort).SetRxNames(fa.dstPorts)
		e1 := flow.Packet().Add().Ethernet()
		e1.Src().SetValue(fa.srcMac)
		e1.Dst().SetValue(dstmac)
		if strings.ToUpper(entry.VRF) != "DEFAULT" {
			flow.Packet().Add().Vlan().Id().SetValue(vlanID)
			vlanID++
		}
		if flowType == "ipv6" {
			v6 := flow.Packet().Add().Ipv6()
			v6.Src().SetValues(pfx1V6Lists)
			v6.Dst().SetValues(entry.Prefixes)
			v6.HopLimit().SetValue(innerTTL)
			v6.TrafficClass().SetValue(dscp << 2)
		} else {
			v4 := flow.Packet().Add().Ipv4()
			v4.Src().SetValue(fa.src)
			v4.Dst().SetValues(entry.Prefixes)
			v4.TimeToLive().SetValue(innerTTL)
			v4.Priority().Dscp().Phb().SetValue(dscp)
		}
		// Add UDP payload to generate traffic
		udp := flow.Packet().Add().Udp()
		udp.SrcPort().SetValues(randRange(5555, 6000))
		udp.DstPort().SetValues(randRange(5555, 6000))

		flows = append(flows, flow)
	}

	return flows
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
