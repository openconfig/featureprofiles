package static_route_resiliency_test

import (
	"fmt"
	"math/rand"
	"net"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	otgtelemetry "github.com/openconfig/ondatra/gnmi/otg"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	ipv4PrefixLen   = 24
	ipv6PrefixLen   = 64
	vlanID          = uint16(10)
	vlanIntfName    = "Vlan10"
	trafficWaitTime = 60 * time.Second
	frameSize       = 512
	flowPPS         = 1000             // packets per second of each RT-1.73.1-RT-1.73.4 flow
	scaleFlowPPS    = 200              // packets per second of each of the 20 RT-1.73.5 flow groups
	numScaleRoutes  = 100              // RT-1.73.5 static routes per address family
	routeWaitTime   = 60 * time.Second // max time for static route state to converge
	verifyWindow    = 10 * time.Second // traffic verification window, measured in packets sent at the flow rate
	minLBShare      = 0.10             // min share of sent (or received) traffic on each load-sharing ATE port
	maxIdleShare    = 0.01             // max share of sent (or received) traffic on an ATE port that must stay idle
	numSrcAddrs     = 253              // source addresses of every flow: 198.51.108.2-198.51.108.254 and 2001:db8:108::2-2001:db8:108::fe (ATE Port 8 subnet)
	numSrcPorts     = 1009             // UDP source ports of every flow (udpSrcPorts)
	numDstPorts     = 1013             // UDP destination ports of every flow (udpDstPorts)
)

var (
	atePorts = []attrs.Attributes{
		{Name: "port1", MAC: "02:00:01:00:00:01", IPv4: "198.51.100.2", IPv4Len: ipv4PrefixLen, IPv6: "2001:db8:100::2", IPv6Len: ipv6PrefixLen},
		{Name: "port2", MAC: "02:00:01:00:00:02", IPv4: "198.51.100.3", IPv4Len: ipv4PrefixLen, IPv6: "2001:db8:100::3", IPv6Len: ipv6PrefixLen},
		{Name: "port3", MAC: "02:00:01:00:00:03"}, // LAG 1 Member
		{Name: "port4", MAC: "02:00:01:00:00:04"}, // LAG 1 Member
		{Name: "port5", MAC: "02:00:01:00:00:05"}, // LAG 2 Member
		{Name: "port6", MAC: "02:00:01:00:00:06"}, // LAG 2 Member
		{Name: "port7", MAC: "02:00:01:00:00:07", IPv4: "198.51.102.2", IPv4Len: ipv4PrefixLen, IPv6: "2001:db8:102::2", IPv6Len: ipv6PrefixLen},
		{Name: "port8", MAC: "02:00:01:00:00:08", IPv4: "198.51.108.2", IPv4Len: ipv4PrefixLen, IPv6: "2001:db8:108::2", IPv6Len: ipv6PrefixLen},
	}

	dutPorts = []attrs.Attributes{
		{Name: "port1", IPv4: "198.51.100.1", IPv4Len: ipv4PrefixLen, IPv6: "2001:db8:100::1", IPv6Len: ipv6PrefixLen},
		{Name: "port2", IPv4: "198.51.100.1", IPv4Len: ipv4PrefixLen, IPv6: "2001:db8:100::1", IPv6Len: ipv6PrefixLen},
		{Name: "port3"},
		{Name: "port4"},
		{Name: "port5"},
		{Name: "port6"},
		{Name: "port7", IPv4: "198.51.102.1", IPv4Len: ipv4PrefixLen, IPv6: "2001:db8:102::1", IPv6Len: ipv6PrefixLen},
		{Name: "port8", IPv4: "198.51.108.1", IPv4Len: ipv4PrefixLen, IPv6: "2001:db8:108::1", IPv6Len: ipv6PrefixLen},
	}

	sviParams = cfgplugins.SVIParams{
		IntfName: vlanIntfName,
		IPv4:     "198.51.100.1",
		IPv4Len:  ipv4PrefixLen,
		IPv6:     "2001:db8:100::1",
		IPv6Len:  ipv6PrefixLen,
	}

	lag1DutAttrs = attrs.Attributes{
		Name:    "lag1",
		IPv4:    "198.51.101.1",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:db8:101::1",
		IPv6Len: ipv6PrefixLen,
	}

	lag1AteAttrs = attrs.Attributes{
		Name:    "lag1Ate",
		MAC:     "02:00:10:01:01:01",
		IPv4:    "198.51.101.2",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:db8:101::2",
		IPv6Len: ipv6PrefixLen,
	}

	// udpSrcPorts and udpDstPorts are the UDP source and destination ports of every flow (see createTraffic).
	udpSrcPorts = randomPorts(1, numSrcPorts)
	udpDstPorts = randomPorts(2, numDstPorts)
)

// unusedAggregateInterfaces returns the vendor-specific names of the n lowest-indexed aggregate
// interfaces that are not present in the DUT's /interfaces/interface/aggregation state, so that the
// test does not reuse, and merge its configuration into, an aggregate interface that already exists
// on the DUT (e.g. the in-band management LAGs of the DUT's baseline configuration).
func unusedAggregateInterfaces(t *testing.T, dut *ondatra.DUTDevice, n int) []string {
	t.Helper()
	batch := gnmi.OCBatch()
	batch.AddPaths(gnmi.OC().InterfaceAny().Aggregation())
	var inUse []string
	if root, ok := gnmi.Lookup(t, dut, batch.State()).Val(); ok {
		for name := range root.Interface {
			inUse = append(inUse, name)
		}
	}
	slices.Sort(inUse)
	t.Logf("Aggregate interfaces already present on the DUT: %v", inUse)
	var names []string
	// netutil.AggregateInterface fails the test beyond the vendor's maximum index, so the loop ends.
	for i := 0; len(names) < n; i++ {
		if name := netutil.AggregateInterface(t, dut, i); !slices.Contains(inUse, name) {
			names = append(names, name)
		}
	}
	return names
}

// configureDUT configures all DUT interfaces required by the RT-1.73 Test Environment Setup:
//   - VLAN Interface: VLAN 10 with DUT Ports 1 and 2 as access ports, and SVI Vlan10
//     configured with IPv4 198.51.100.1/24 and IPv6 2001:db8:100::1/64.
//   - LAG 1 (Resilience): Layer 3 Static LAG across DUT Ports 3 and 4 with IPv4 198.51.101.1/24
//     and IPv6 2001:db8:101::1/64.
//   - LAG 2 (Scale & FIB): Layer 3 Static LAG across DUT Ports 5 and 6 with 10 VLAN-tagged
//     subinterfaces (VLANs 101-110) using IPv4 198.51.111.1/24..198.51.120.1/24 and
//     IPv6 2001:db8:111::1/64..2001:db8:120::1/64.
//   - Standalone Port: DUT Port 7 with IPv4 198.51.102.1/24 and IPv6 2001:db8:102::1/64.
//   - Traffic Source Port: DUT Port 8 with IPv4 10.0.0.1/24 and IPv6 2001:db8:a::1/64.
//
// It returns the name of the DUT LAG 1 interface.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) string {
	t.Helper()
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	// Determine platform-appropriate aggregate interface names for LAG 1 and LAG 2, skipping
	// aggregate interfaces that already exist on the DUT.
	lagNames := unusedAggregateInterfaces(t, dut, 2)
	lag1Name, lag2Name := lagNames[0], lagNames[1]
	t.Logf("Using LAG names: %s, %s", lag1Name, lag2Name)

	// Test Environment Setup - Step 1: Initialize DUT Ports 1 and 2 via BatchReplace with an
	// unaddressed Subinterface(0) (IPv4/IPv6 disabled) when OpenConfig SwitchedVlan is supported.
	// On platforms where the baseline config sets "switchport default mode routed" and "no switchport"
	// on all ports (e.g. Arista EOS), replacing the interface with Subinterface(0) having
	// ipv4.enabled=false clears "no switchport" and transitions the port from L3 routed mode into
	// L2 bridged (switchport) mode so that subsequent SwitchedVlan access-vlan configuration takes effect.
	if !deviations.SwitchedVlanUnsupported(dut) {
		initBatch := &gnmi.SetBatch{}
		for i := 0; i < 2; i++ {
			portName := dut.Port(t, dutPorts[i].Name).Name()
			initAttr := &attrs.Attributes{Name: dutPorts[i].Name}
			initIntf := initAttr.NewOCInterface(portName, dut)
			gnmi.BatchReplace(initBatch, gnmi.OC().Interface(portName).Config(), initIntf)
		}
		initBatch.Set(t, dut)
	}

	// Test Environment Setup - Step 2: Create VLAN 10 on the DUT in both OpenConfig
	// (/network-instances/network-instance[name=default]/vlans/vlan[vlan-id=10]) and via
	// cfgplugins.ConfigureVlan (which also disables spanning-tree on VLAN 10 so access ports
	// immediately enter forwarding state without STP learning delay).
	vlanSubData := cfgplugins.DUTSubInterfaceData{
		VlanID:        int(vlanID),
		IPv4Address:   net.ParseIP(sviParams.IPv4),
		IPv6Address:   net.ParseIP(sviParams.IPv6),
		IPv4PrefixLen: int(sviParams.IPv4Len),
		IPv6PrefixLen: int(sviParams.IPv6Len),
	}
	if !deviations.SwitchedVlanUnsupported(dut) {
		vlanBatch := &gnmi.SetBatch{}
		cfgplugins.CreateVlanFromOC(t, dut, vlanBatch, deviations.DefaultNetworkInstance(dut), vlanSubData)
		vlanBatch.Set(t, dut)
		cfgplugins.ConfigureVlan(t, dut, cfgplugins.VlanParams{
			VlanID: vlanID,
		})
	}

	// Test Environment Setup - Step 3: Configure physical DUT Ports 1..8.
	//   - Ports 1 & 2: Configured as L2 Access Ports in VLAN 10 (bridged to SVI Vlan10).
	//   - Ports 3 & 4: Enabled physical member ports for Static LAG 1.
	//   - Ports 5 & 6: Enabled physical member ports for Static LAG 2.
	//   - Port 7: Standalone routed L3 port (198.51.102.1/24, 2001:db8:102::1/64).
	//   - Port 8: Traffic source routed L3 port (10.0.0.1/24, 2001:db8:a::1/64).
	portBatch := &gnmi.SetBatch{}
	for i, a := range dutPorts {
		iObj := &oc.Interface{Name: ygot.String(dut.Port(t, a.Name).Name())}
		iObj.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
		if deviations.InterfaceEnabled(dut) {
			iObj.Enabled = ygot.Bool(true)
		}

		if i < 2 { // DUT Ports 1 and 2: Assign as access ports to VLAN 10 when SwitchedVlan OC is supported.
			if !deviations.SwitchedVlanUnsupported(dut) {
				cfgplugins.ConfigureAccessVlan(cfgplugins.AccessVlanParams{
					Intf:   iObj,
					VlanID: vlanID,
				})
			}
		} else if i == 6 || i == 7 { // DUT Port 7 (Standalone Port) and DUT Port 8 (Traffic Source Port).
			s := iObj.GetOrCreateSubinterface(0)
			cfgplugins.ConfigureSubinterfaceIPs(s, dut, a.IPv4, a.IPv4Len, a.IPv6, a.IPv6Len)
			// Without this, platforms with deviation InterfaceEnabled (e.g. Nokia SR Linux) leave
			// IPv4/IPv6 admin-disabled on the subinterface and never answer the ATE's ARP/ND, so the
			// setup WaitForARP fails for ATE Port 7 / Port 8 before RT-1.73.1 starts.
			enableSubinterfaceIP(dut, s)
		}
		// Member port assignment for LAG 1 (Ports 3, 4) and LAG 2 (Ports 5, 6) is handled by NewAggregateInterface.
		gnmi.BatchUpdate(portBatch, gnmi.OC().Interface(dut.Port(t, a.Name).Name()).Config(), iObj)
	}
	portBatch.Set(t, dut)

	// Test Environment Setup - Step 4: Configure Routed VLAN Interface (SVI / IRB) for VLAN 10.
	// Use deviation RoutedVlanInterfaceName if specified by the platform metadata, and if
	// VlanSubinterfaceOCUnsupported is set, configure the VLAN SVI / IRB addressing via CLI helper.
	// A platform-specific name (e.g. Junos "irb.10", where the ".10" suffix is the IRB unit) does
	// not imply the VLAN the way "Vlan10" does, so the SVI is also bound to VLAN 10 explicitly via
	// /interfaces/interface/routed-vlan/config/vlan (Junos: "set vlans vlan10 l3-interface irb.10").
	sviCfg := sviParams
	if routedVlanName := deviations.RoutedVlanInterfaceName(dut); routedVlanName != "" {
		sviCfg.IntfName = routedVlanName
		sviCfg.Vlan = oc.UnionUint16(vlanID)
	}
	if !deviations.SwitchedVlanUnsupported(dut) {
		cfgplugins.ConfigureSVI(t, dut, sviCfg)
	}
	if deviations.VlanSubinterfaceOCUnsupported(dut) {
		cfgplugins.ConfigureVlanInterfaceFromCLI(t, dut, vlanSubData)
	}
	if deviations.LoadBalancePolicyOCUnsupported(dut) {
		// When the DUT baseline configuration has "rib ucmp eligibility mode link-bandwidth all"
		// (e.g. Arista EOS edge router profiles), static routes across mixed aggregate/subinterface
		// egress interfaces are weighted 10:1 by interface link bandwidth rather than equal cost (ECMP).
		// Disable link-bandwidth UCMP for the duration of this ECMP resilience test so next-hops are
		// load-balanced equally as specified by RT-1.73.4, and restore baseline on cleanup.
		helpers.GnmiCLIConfig(t, dut, "router general\n   no rib ucmp eligibility mode link-bandwidth all\n")
		t.Cleanup(func() {
			helpers.GnmiCLIConfig(t, dut, "router general\n   rib ucmp eligibility mode link-bandwidth all\n")
		})
	}

	lagBatch := &gnmi.SetBatch{}
	// Test Environment Setup - LAG 1 (Resilience): DUT Ports 3 and 4 as Layer 3 Static LAG 1
	// with IPv4 198.51.101.1/24 and IPv6 2001:db8:101::1/64.
	lag1 := &cfgplugins.DUTAggData{
		Attributes:      lag1DutAttrs,
		OndatraPortsIdx: []int{2, 3}, // Port 3 and 4
		LagName:         lag1Name,
		AggType:         oc.IfAggregate_AggregationType_STATIC,
	}
	cfgplugins.NewAggregateInterface(t, dut, lagBatch, lag1)

	// Test Environment Setup - LAG 2 (Scale & FIB): DUT Ports 5 and 6 as Layer 3 Static LAG 2
	// with 10 subinterfaces (VLAN tags 101-110) and IPv4 198.51.111.0/24..198.51.120.0/24,
	// IPv6 2001:db8:111::/64..2001:db8:120::/64.
	var lag2Subs []*cfgplugins.DUTSubInterfaceData
	for i := 1; i <= 10; i++ {
		tEnable := true
		lag2Subs = append(lag2Subs, &cfgplugins.DUTSubInterfaceData{
			VlanID:        100 + i,
			VlanEnable:    &tEnable,
			IPv4Address:   net.ParseIP(fmt.Sprintf("198.51.%d.1", 110+i)),
			IPv6Address:   net.ParseIP(fmt.Sprintf("2001:db8:%d::1", 110+i)),
			IPv4PrefixLen: 24,
			IPv6PrefixLen: 64,
		})
	}
	lag2 := &cfgplugins.DUTAggData{
		SubInterfaces:   lag2Subs,
		OndatraPortsIdx: []int{4, 5}, // Port 5 and 6
		LagName:         lag2Name,
		AggType:         oc.IfAggregate_AggregationType_STATIC,
	}
	agg2 := cfgplugins.NewAggregateInterface(t, dut, lagBatch, lag2)
	// cfgplugins.AddSubInterface (called by NewAggregateInterface for VLANs 101-110) has the same
	// limitation as ConfigureSubinterfaceIPs, so enable IPv4/IPv6 on each LAG 2 VLAN subinterface for
	// InterfaceEnabled platforms. lagBatch holds pointers to these subinterfaces and marshals them at
	// lagBatch.Set, so the change is part of the same SetRequest.
	for _, sd := range lag2Subs {
		s := agg2.GetSubinterface(uint32(sd.VlanID))
		if s == nil {
			t.Fatalf("LAG 2 %s has no subinterface for VLAN %d", lag2Name, sd.VlanID)
		}
		enableSubinterfaceIP(dut, s)
	}
	// Remove untagged Subinterface(0) from LAG 2 so platforms that disallow mixing untagged
	// Subinterface(0) with 802.1Q VLAN-tagged subinterfaces (101..110) accept the LAG 2 config.
	// Platforms that need a routed Subinterface(0) for non-zero subinterfaces to be operational
	// (RequireRoutedSubinterface0, e.g. Arista EOS, where removing it turns the LAG into an L2
	// switchport and the VLAN subinterfaces never come up) keep the routed Subinterface(0)
	// created by NewAggregateInterface.
	if !deviations.RequireRoutedSubinterface0(dut) {
		agg2.DeleteSubinterface(0)
		gnmi.BatchDelete(lagBatch, gnmi.OC().Interface(lag2Name).Subinterface(0).Config())
	}

	lagBatch.Set(t, dut)

	// Explicitly bind routed L3 interfaces and VLAN subinterfaces to the default network-instance
	// when required by the platform (ExplicitInterfaceInDefaultVRF).
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, dut.Port(t, "port7").Name(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, dut.Port(t, "port8").Name(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, lag1Name, deviations.DefaultNetworkInstance(dut), 0)
		for i := 1; i <= 10; i++ {
			fptest.AssignToNetworkInstance(t, dut, lag2Name, deviations.DefaultNetworkInstance(dut), uint32(100+i))
		}
	}

	return lag1Name
}

// enableSubinterfaceIP sets /interfaces/interface/subinterfaces/subinterface/ipv4/config/enabled
// and .../ipv6/config/enabled to true on s for platforms that require interface enabled leaves to
// be set explicitly (deviation InterfaceEnabled). On such platforms (e.g. Nokia SR Linux) IPv4 and
// IPv6 on a subinterface stay admin-disabled unless enabled=true is configured, so the DUT does not
// answer ARP/ND and the Test Environment Setup's WaitForARP never resolves.
//
// cfgplugins.ConfigureSubinterfaceIPs and cfgplugins.AddSubInterface only set these leaves under
// deviation IPv4MissingEnabled, so this follows attrs.ConfigOCInterface instead: ipv4 enabled under
// InterfaceEnabled unless the platform does not support ipv4/config/enabled (IPv4MissingEnabled),
// and ipv6 enabled under InterfaceEnabled. It only adds leaves for address families configured on s.
func enableSubinterfaceIP(dut *ondatra.DUTDevice, s *oc.Interface_Subinterface) {
	if !deviations.InterfaceEnabled(dut) {
		return
	}
	if s4 := s.GetIpv4(); s4 != nil && !deviations.IPv4MissingEnabled(dut) {
		s4.Enabled = ygot.Bool(true)
	}
	if s6 := s.GetIpv6(); s6 != nil {
		s6.Enabled = ygot.Bool(true)
	}
}

// configureOTG configures the ATE (OTG) ports, LAGs, VLAN subinterfaces, and IPv4/IPv6 addresses
// matching the RT-1.73 Test Environment Setup in the README:
//   - ATE Ports 1 & 2: Connected to DUT VLAN 10 access ports (198.51.100.2, 198.51.100.3 / 2001:db8:100::2, ::3).
//   - ATE Ports 3 & 4: Static LAG 1 with IPv4 198.51.101.2/24 and IPv6 2001:db8:101::2/64.
//   - ATE Ports 5 & 6: Static LAG 2 with 10 VLAN subinterfaces (VLANs 101-110) and IPs 198.51.111.2..198.51.120.2.
//   - ATE Port 7: Standalone routed interface (198.51.102.2/24, 2001:db8:102::2/64).
//   - ATE Port 8: Traffic source interface (198.51.108.2/24, 2001:db8:108::2/64).
func configureOTG(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config) {
	t.Helper()

	// Configure standalone ATE Ports 1, 2, 7, and 8.
	for _, i := range []int{0, 1, 6, 7} {
		a := atePorts[i]
		p := ate.Port(t, a.Name)
		a.AddToOTG(top, p, &dutPorts[i])
	}

	// Configure ATE LAG 1 on ATE Ports 3 and 4.
	p3 := ate.Port(t, atePorts[2].Name)
	p4 := ate.Port(t, atePorts[3].Name)
	top.Ports().Add().SetName(p3.ID())
	top.Ports().Add().SetName(p4.ID())

	lag1 := top.Lags().Add().SetName("ATE_LAG1")
	lag1.Protocol().Static().SetLagId(1)
	lag1.Ports().Add().SetPortName(p3.ID()).Ethernet().SetMac(atePorts[2].MAC).SetName("LAG1_Rx1")
	lag1.Ports().Add().SetPortName(p4.ID()).Ethernet().SetMac(atePorts[3].MAC).SetName("LAG1_Rx2")

	lag1Dev := top.Devices().Add().SetName("LAG1_Dev")
	lag1Eth := lag1Dev.Ethernets().Add().SetName("LAG1_Eth").SetMac(lag1AteAttrs.MAC)
	lag1Eth.Connection().SetLagName("ATE_LAG1")
	lag1Eth.Ipv4Addresses().Add().SetName("LAG1_IPv4").SetAddress(lag1AteAttrs.IPv4).SetGateway(lag1DutAttrs.IPv4).SetPrefix(uint32(lag1AteAttrs.IPv4Len))
	lag1Eth.Ipv6Addresses().Add().SetName("LAG1_IPv6").SetAddress(lag1AteAttrs.IPv6).SetGateway(lag1DutAttrs.IPv6).SetPrefix(uint32(lag1AteAttrs.IPv6Len))

	// Configure ATE LAG 2 on ATE Ports 5 and 6 with 10 VLAN subinterfaces (VLAN tags 101-110).
	p5 := ate.Port(t, atePorts[4].Name)
	p6 := ate.Port(t, atePorts[5].Name)
	top.Ports().Add().SetName(p5.ID())
	top.Ports().Add().SetName(p6.ID())

	lag2 := top.Lags().Add().SetName("ATE_LAG2")
	lag2.Protocol().Static().SetLagId(2)
	lag2.Ports().Add().SetPortName(p5.ID()).Ethernet().SetMac(atePorts[4].MAC).SetName("LAG2_Rx1")
	lag2.Ports().Add().SetPortName(p6.ID()).Ethernet().SetMac(atePorts[5].MAC).SetName("LAG2_Rx2")

	for i := 1; i <= 10; i++ {
		vlanId := 100 + i
		vlanName := fmt.Sprintf("LAG2_Vlan%d", vlanId)
		lag2Dev := top.Devices().Add().SetName(fmt.Sprintf("LAG2_Dev%d", vlanId))
		lag2Eth := lag2Dev.Ethernets().Add().SetName(fmt.Sprintf("LAG2_Eth%d", vlanId)).SetMac(fmt.Sprintf("02:00:20:%02x:01:01", i))
		lag2Eth.Connection().SetLagName("ATE_LAG2")
		lag2Eth.Vlans().Add().SetName(vlanName).SetId(uint32(vlanId))

		lag2Ipv4 := lag2Eth.Ipv4Addresses().Add().SetName(fmt.Sprintf("LAG2_IPv4_%d", vlanId))
		lag2Ipv4.SetAddress(fmt.Sprintf("198.51.%d.2", 110+i)).SetGateway(fmt.Sprintf("198.51.%d.1", 110+i)).SetPrefix(24)

		lag2Ipv6 := lag2Eth.Ipv6Addresses().Add().SetName(fmt.Sprintf("LAG2_IPv6_%d", vlanId))
		lag2Ipv6.SetAddress(fmt.Sprintf("2001:db8:%d::2", 110+i)).SetGateway(fmt.Sprintf("2001:db8:%d::1", 110+i)).SetPrefix(64)
	}
}

// createTraffic appends an IPv4 flow and an IPv6 flow originating from ATE Port 8 (Traffic Source Port)
// destined to dstV4Net and dstV6Net, tracking reception on the specified rxV4Names and rxV6Names OTG device endpoints.
// A destination with a prefix length (e.g. "203.0.115.1/24") is incremented over 254 addresses; a plain address is
// the only destination.
//
// RT-1.73.2 Step 4, RT-1.73.3 Step 4 and RT-1.73.4 Steps 3 and 5 verify load-sharing across ECMP next-hops and LAG
// member links, which the DUT does by hashing packet header fields. With few distinct header values, the share of
// each next-hop depends on where the DUT hash happens to map those few values and can be far from even. So that
// these checks exercise the DUT load-sharing rather than a pattern in the traffic, every flow varies each 5-tuple
// field over many values: numSrcAddrs source addresses, up to 254 destination addresses, and the pseudo-random
// udpSrcPorts and udpDstPorts lists. The value counts (253, 254, 1009, 1013) are pairwise coprime, so the fields
// do not repeat in lockstep.
func createTraffic(top gosnappi.Config, flowName, dstV4Net, dstV6Net string, rxV4Names, rxV6Names []string) {
	srcV4Addr := atePorts[7].IPv4
	srcV6Addr := atePorts[7].IPv6

	v4F := top.Flows().Add().SetName(flowName + "_v4")
	v4F.Metrics().SetEnable(true)
	v4F.TxRx().Device().SetTxNames([]string{"port8.IPv4"}).SetRxNames(rxV4Names)
	v4F.Size().SetFixed(frameSize)
	v4F.Rate().SetPps(flowPPS)
	eth := v4F.Packet().Add().Ethernet()
	eth.Src().SetValue(atePorts[7].MAC)
	eth.Dst().Auto()
	v4 := v4F.Packet().Add().Ipv4()
	v4.Src().Increment().SetStart(srcV4Addr).SetStep("0.0.0.1").SetCount(numSrcAddrs)

	if strings.Contains(dstV4Net, "/") {
		parts := strings.Split(dstV4Net, "/")
		v4.Dst().Increment().SetStart(parts[0]).SetStep("0.0.0.1").SetCount(254)
	} else {
		v4.Dst().SetValue(dstV4Net)
	}
	udp4 := v4F.Packet().Add().Udp()
	udp4.SrcPort().SetValues(udpSrcPorts)
	udp4.DstPort().SetValues(udpDstPorts)

	v6F := top.Flows().Add().SetName(flowName + "_v6")
	v6F.Metrics().SetEnable(true)
	v6F.TxRx().Device().SetTxNames([]string{"port8.IPv6"}).SetRxNames(rxV6Names)
	v6F.Size().SetFixed(frameSize)
	v6F.Rate().SetPps(flowPPS)
	eth = v6F.Packet().Add().Ethernet()
	eth.Src().SetValue(atePorts[7].MAC)
	eth.Dst().Auto()
	v6 := v6F.Packet().Add().Ipv6()
	v6.Src().Increment().SetStart(srcV6Addr).SetStep("::1").SetCount(numSrcAddrs)

	if strings.Contains(dstV6Net, "/") {
		parts := strings.Split(dstV6Net, "/")
		v6.Dst().Increment().SetStart(parts[0]).SetStep("::1").SetCount(254)
	} else {
		v6.Dst().SetValue(dstV6Net)
	}
	udp6 := v6F.Packet().Add().Udp()
	udp6.SrcPort().SetValues(udpSrcPorts)
	udp6.DstPort().SetValues(udpDstPorts)
}

// createScaleTraffic creates 10 IPv4 and 10 IPv6 OTG flow groups (20 Flow Groups total, well within the 64 Flow Group
// hardware limit on ATE Port 8) that exercise all 100 IPv4 static routes (10.1.0.0/24..10.1.99.0/24) and all 100 IPv6
// static routes (2001:db8:1000::/64..2001:db8:1099::/64) across the 10 ATE LAG 2 VLAN subinterfaces (VLAN 101..110) and ATE Port 7
// (RT-1.73.5 Step 2 onwards). As in createTraffic, every flow varies each 5-tuple field over many values
// (17 sources, 10 destinations, 31 UDP source ports, 29 UDP destination ports; pairwise coprime counts)
// so that the RT-1.73.5 Step 2 and Step 4 load-sharing checks exercise the DUT load-sharing without exceeding
// the ATE hardware table memory limit for value lists across 20 concurrent flow groups.
func createScaleTraffic(top gosnappi.Config) {
	srcV4Addr := atePorts[7].IPv4
	srcV6Addr := atePorts[7].IPv6

	for k := 0; k < 10; k++ {
		nhIndex := k + 1
		vlanID := 100 + nhIndex
		rxV4 := []string{fmt.Sprintf("LAG2_IPv4_%d", vlanID), "port7.IPv4"}
		rxV6 := []string{fmt.Sprintf("LAG2_IPv6_%d", vlanID), "port7.IPv6"}

		// IPv4 flow group for nhIndex covering the 10 routes: 10.1.(k).1, 10.1.(k+10).1, ..., 10.1.(k+90).1
		v4F := top.Flows().Add().SetName(fmt.Sprintf("scale_nh%d_v4", nhIndex))
		v4F.Metrics().SetEnable(true)
		v4F.TxRx().Device().SetTxNames([]string{"port8.IPv4"}).SetRxNames(rxV4)
		v4F.Size().SetFixed(frameSize)
		v4F.Rate().SetPps(scaleFlowPPS)
		eth := v4F.Packet().Add().Ethernet()
		eth.Src().SetValue(atePorts[7].MAC)
		eth.Dst().Auto()
		v4 := v4F.Packet().Add().Ipv4()
		v4.Src().Increment().SetStart(srcV4Addr).SetStep("0.0.0.1").SetCount(17)
		v4.Dst().Increment().SetStart(fmt.Sprintf("10.1.%d.1", k)).SetStep("0.0.10.0").SetCount(10)
		udp4 := v4F.Packet().Add().Udp()
		udp4.SrcPort().Increment().SetStart(10000).SetStep(7).SetCount(31)
		udp4.DstPort().Increment().SetStart(20000).SetStep(13).SetCount(29)

		// IPv6 flow group for nhIndex covering the 10 routes: 2001:db8:100k::1, 2001:db8:101k::1, ..., 2001:db8:109k::1.
		// Scale route k+10j is 2001:db8:10{j}{k}::/64 (see scalePrefixes), so a step of 0:0:10:: advances j by one.
		v6F := top.Flows().Add().SetName(fmt.Sprintf("scale_nh%d_v6", nhIndex))
		v6F.Metrics().SetEnable(true)
		v6F.TxRx().Device().SetTxNames([]string{"port8.IPv6"}).SetRxNames(rxV6)
		v6F.Size().SetFixed(frameSize)
		v6F.Rate().SetPps(scaleFlowPPS)
		eth = v6F.Packet().Add().Ethernet()
		eth.Src().SetValue(atePorts[7].MAC)
		eth.Dst().Auto()
		v6 := v6F.Packet().Add().Ipv6()
		v6.Src().Increment().SetStart(srcV6Addr).SetStep("::1").SetCount(17)
		v6.Dst().Increment().SetStart(fmt.Sprintf("2001:db8:10%02d::1", k)).SetStep("0:0:10::").SetCount(10)
		udp6 := v6F.Packet().Add().Udp()
		udp6.SrcPort().Increment().SetStart(10000).SetStep(7).SetCount(31)
		udp6.DstPort().Increment().SetStart(20000).SetStep(13).SetCount(29)
	}
}

// randomPorts returns n pseudo-random UDP ports in [1024, 65535]. The fixed seed makes every run send the same
// packets, so that load-sharing results are reproducible.
func randomPorts(seed int64, n int) []uint32 {
	r := rand.New(rand.NewSource(seed))
	ports := make([]uint32, n)
	for i := range ports {
		ports[i] = uint32(1024 + r.Intn(65536-1024))
	}
	return ports
}

// configureRoute configures IPv4 and IPv6 static routes with the specified next-hops on the DUT
// via gNMI Set Replace and waits until the DUT reports exactly those next-hops in state.
func configureRoute(t *testing.T, dut *ondatra.DUTDevice, v4Prefix, v6Prefix string, v4Nh, v6Nh []string) {
	t.Helper()
	cfgplugins.ConfigureStaticRouteWithMultipleNextHops(t, dut, v4Prefix, v6Prefix, v4Nh, v6Nh)
	awaitStaticNextHops(t, dut, map[string][]string{v4Prefix: v4Nh, v6Prefix: v6Nh})
}

// awaitInterfaceDown waits for the specified interface on the DUT to transition to either
// OperStatus DOWN or LOWER_LAYER_DOWN (standard OpenConfig state for an admin-UP LAG whose member ports are DOWN).
func awaitInterfaceDown(t *testing.T, dut *ondatra.DUTDevice, intfName string, timeout time.Duration) {
	t.Helper()
	last, ok := gnmi.Watch(t, dut, gnmi.OC().Interface(intfName).OperStatus().State(), timeout, func(val *ygnmi.Value[oc.E_Interface_OperStatus]) bool {
		status, present := val.Val()
		return present && (status == oc.Interface_OperStatus_DOWN || status == oc.Interface_OperStatus_LOWER_LAYER_DOWN)
	}).Await(t)
	if !ok {
		got := gnmi.Get(t, dut, gnmi.OC().Interface(intfName).OperStatus().State())
		t.Fatalf("Interface %s did not transition to DOWN or LOWER_LAYER_DOWN within %v, last got: %v", intfName, timeout, got)
	}
	status, _ := last.Val()
	t.Logf("Interface %s oper-status: %v", intfName, status)
}

// trafficCounters is a snapshot of OTG flow and ATE port counters. Deltas between two snapshots
// verify traffic over a window while it keeps running, as required by RT-1.73.3 (which builds on
// the RT-1.73.2 traffic), RT-1.73.4 Step 4 and RT-1.73.5 Steps 2-8.
type trafficCounters struct {
	flowTx, flowRx map[string]uint64 // packets by flow name
	portRx         map[string]uint64 // InFrames by ATE port name
}

// readCounters returns the current tx/rx packet counters of every flow in top and the InFrames
// counter of every ATE port. The flow counters and the port counters are each read with a single
// wildcard gNMI request (FlowAny, PortAny) so that all counters of a kind are sampled together.
// Reading them one path at a time took about 1s per path (about 24s for the 20 RT-1.73.5 flows
// and 8 ports), which shifted the port counter windows against the flow counter windows by
// several seconds and skewed the per-port shares of the sent traffic (up to 128% in total).
// Every flow of top and every ATE port must be present: a missing counter would read as zero
// and could let an idle-port check pass vacuously.
func readCounters(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config) trafficCounters {
	t.Helper()
	c := trafficCounters{flowTx: map[string]uint64{}, flowRx: map[string]uint64{}, portRx: map[string]uint64{}}
	for _, f := range gnmi.GetAll(t, ate.OTG(), gnmi.OTG().FlowAny().State()) {
		c.flowTx[f.GetName()], c.flowRx[f.GetName()] = f.GetCounters().GetOutPkts(), f.GetCounters().GetInPkts()
	}
	for _, v := range gnmi.LookupAll(t, ate.OTG(), gnmi.OTG().PortAny().Counters().InFrames().State()) {
		inFrames, ok := v.Val()
		if !ok {
			continue
		}
		for _, e := range v.Path.GetElem() {
			if e.GetName() == "port" { // /ports/port[name=<port>]/state/counters/in-frames
				c.portRx[e.GetKey()["name"]] = inFrames
			}
		}
	}
	for _, f := range top.Flows().Items() {
		if _, ok := c.flowTx[f.Name()]; !ok {
			t.Fatalf("OTG flow %s is missing from the OTG flow state", f.Name())
		}
	}
	for _, p := range atePorts {
		if _, ok := c.portRx[p.Name]; !ok {
			t.Fatalf("ATE port %s is missing from the OTG port counters", p.Name)
		}
	}
	return c
}

// verifyTrafficSince fails the test unless, between before and now:
//   - every flow sent packets and lost between minLoss and maxLoss percent of them,
//   - every port in lbPorts received at least minLBShare of all sent packets, and
//   - every port in idlePorts received at most maxIdleShare of all sent packets.
func verifyTrafficSince(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config, before trafficCounters, minLoss, maxLoss float64, lbPorts, idlePorts []string) {
	t.Helper()
	now := readCounters(t, ate, top)
	delta := func(what string, b, n uint64) uint64 {
		if n < b {
			t.Fatalf("%s counter went backwards (%d -> %d): traffic restarted inside a verification window?", what, b, n)
		}
		return n - b
	}
	var sent uint64
	for _, f := range top.Flows().Items() {
		name := f.Name()
		tx := delta(name+" tx", before.flowTx[name], now.flowTx[name])
		rx := delta(name+" rx", before.flowRx[name], now.flowRx[name])
		if tx == 0 {
			t.Fatalf("Flow %s sent no packets in the verification window", name)
		}
		sent += tx
		loss := 0.0 // rx may exceed tx by the packets in flight at the first snapshot.
		if rx < tx {
			loss = float64(tx-rx) * 100 / float64(tx)
		}
		t.Logf("Flow %s: window tx=%d rx=%d loss=%.3f%%", name, tx, rx, loss)
		if loss < minLoss-0.01 || loss > maxLoss+0.01 {
			t.Fatalf("Flow %s: window loss %.3f%% (tx=%d rx=%d), want %v-%v%%", name, loss, tx, rx, minLoss, maxLoss)
		}
	}
	var portsRx uint64
	for _, p := range lbPorts {
		rx := delta(p, before.portRx[p], now.portRx[p])
		portsRx += rx
		share := float64(rx) / float64(sent)
		t.Logf("ATE %s received %d of %d sent packets (%.1f%%)", p, rx, sent, 100*share)
		if share < minLBShare {
			t.Errorf("ATE %s received %.1f%% of the traffic, want >= %.0f%% (load-sharing)", p, 100*share, 100*minLBShare)
		}
	}
	for _, p := range idlePorts {
		rx := delta(p, before.portRx[p], now.portRx[p])
		portsRx += rx
		share := float64(rx) / float64(sent)
		t.Logf("ATE %s received %d of %d sent packets (%.2f%%), expected idle", p, rx, sent, 100*share)
		if share > maxIdleShare {
			t.Errorf("ATE %s received %d of %d packets (%.2f%%), want <= %.0f%% (no traffic expected)", p, rx, sent, 100*share, 100*maxIdleShare)
		}
	}
	if len(lbPorts)+len(idlePorts) > 0 {
		// Diagnostic only: flow and port counters are separate OTG statistics sampled one request apart,
		// so this can differ from 100% by the packets sent in between.
		t.Logf("ATE ports %v received %d packets in total for %d sent (%.1f%%)", append(slices.Clone(lbPorts), idlePorts...), portsRx, sent, 100*float64(portsRx)/float64(sent))
	}
}

// verifyTrafficWindow lets traffic run for verifyWindow worth of packets (see awaitFlowsTransmitted),
// then applies verifyTrafficSince to that window.
func verifyTrafficWindow(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config, minLoss, maxLoss float64, lbPorts, idlePorts []string) {
	t.Helper()
	before := readCounters(t, ate, top)
	awaitFlowsTransmitted(t, ate, top, before, verifyWindow)
	verifyTrafficSince(t, ate, top, before, minLoss, maxLoss, lbPorts, idlePorts)
}

// windowPkts returns the number of packets flow f sends in d at its configured rate.
func windowPkts(f gosnappi.Flow, d time.Duration) uint64 {
	return uint64(float64(f.Rate().Pps()) * d.Seconds())
}

// awaitFlowsTransmitted uses gnmi.Watch on /flows/flow/state/counters/out-pkts and returns once
// every flow in top has sent at least window worth of packets (at its configured rate) since
// before. This bounds a traffic verification window by the packets actually sent instead of
// sleeping for a fixed time. If the ATE resets a flow counter because traffic was (re)started
// (RT-1.73.5 Step 10), the window of that flow is measured from the reset.
func awaitFlowsTransmitted(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config, before trafficCounters, window time.Duration) {
	t.Helper()
	timeout := window + trafficWaitTime
	for _, f := range top.Flows().Items() {
		name := f.Name()
		pkts := windowPkts(f, window)
		last := before.flowTx[name]
		want := last + pkts
		_, ok := gnmi.Watch(t, ate.OTG(), gnmi.OTG().Flow(name).State(), timeout, func(v *ygnmi.Value[*otgtelemetry.Flow]) bool {
			fs, present := v.Val()
			if !present {
				return false
			}
			tx := fs.GetCounters().GetOutPkts()
			if tx < last { // Counter reset: traffic was (re)started.
				want = tx + pkts
			}
			last = tx
			return tx >= want
		}).Await(t)
		if !ok {
			t.Fatalf("Flow %s did not send %d packets within %v (last out-pkts %d)", name, pkts, timeout, last)
		}
	}
}

// awaitTrafficFlowing uses gnmi.Watch on /flows/flow/state/counters/in-pkts and returns once every
// flow in top has received at least one second worth of packets (at its configured rate) since the
// call, i.e. the DUT forwards all flows. It is used after StartTraffic (RT-1.73.4 Step 3, RT-1.73.5
// Step 2) and to observe automatic recovery without restarting traffic (RT-1.73.3 Step 4, RT-1.73.5 Step 8).
func awaitTrafficFlowing(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config, timeout time.Duration) {
	t.Helper()
	start := time.Now()
	before := readCounters(t, ate, top)
	for _, f := range top.Flows().Items() {
		name := f.Name()
		want := before.flowRx[name] + windowPkts(f, time.Second)
		_, ok := gnmi.Watch(t, ate.OTG(), gnmi.OTG().Flow(name).State(), timeout, func(v *ygnmi.Value[*otgtelemetry.Flow]) bool {
			fs, present := v.Val()
			return present && fs.GetCounters().GetInPkts() >= want
		}).Await(t)
		if !ok {
			t.Fatalf("Flow %s is not forwarded: it did not receive %d packets within %v", name, want-before.flowRx[name], timeout)
		}
	}
	t.Logf("All flows are forwarded (waited %v)", time.Since(start).Round(time.Second))
}

// awaitFlowsDropped uses gnmi.Watch on /flows/flow/state/in-frame-rate and returns once every flow
// in top is received at less than 1 frame per second, i.e. the DUT drops all traffic. It is used
// after a failure (RT-1.73.3 Step 2, RT-1.73.5 Step 7) so that packets received before the failure,
// which OTG counters may report late, stay out of the following drop verification window.
func awaitFlowsDropped(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config, timeout time.Duration) {
	t.Helper()
	for _, f := range top.Flows().Items() {
		name := f.Name()
		last := "no flow state received"
		_, ok := gnmi.Watch(t, ate.OTG(), gnmi.OTG().Flow(name).State(), timeout, func(v *ygnmi.Value[*otgtelemetry.Flow]) bool {
			fs, present := v.Val()
			if !present {
				return false
			}
			rate := fs.GetInFrameRate() // IEEE 754 float32; must be 4 bytes long to decode.
			if len(rate) != 4 {
				last = fmt.Sprintf("in-frame-rate of %d bytes, want 4", len(rate))
				return false
			}
			fps := ygot.BinaryToFloat32(rate)
			last = fmt.Sprintf("in-frame-rate %.1f fps", fps)
			return fps < 1
		}).Await(t)
		if !ok {
			t.Fatalf("Flow %s is still received %v after the failure (last %s), want all traffic dropped", name, timeout, last)
		}
	}
	t.Logf("All flows are dropped by the DUT")
}

// awaitRxShare uses gnmi.WatchAll on /ports/port/state/in-rate of all ATE ports and returns once
// cond holds for each port in ports, where share is the port's receive rate divided by the total
// receive rate of rxPorts. Every port in ports must also be in rxPorts. It is used after a FIB
// change so the next verification window starts only once the DUT forwards according to the new
// next-hops (RT-1.73.4 Step 4, RT-1.73.5 Steps 3 and 5). Comparing shares of rates keeps the check
// independent of the unit of in-rate.
func awaitRxShare(t *testing.T, ate *ondatra.ATEDevice, ports, rxPorts []string, desc string, cond func(share float64) bool) {
	t.Helper()
	rates := map[string]float64{}
	summary := "no ATE port in-rate received"
	_, ok := gnmi.WatchAll(t, ate.OTG(), gnmi.OTG().PortAny().State(), trafficWaitTime, func(v *ygnmi.Value[*otgtelemetry.Port]) bool {
		ps, present := v.Val()
		if !present {
			return false
		}
		name := ps.GetName()
		if elems := v.Path.GetElem(); name == "" && len(elems) > 0 {
			name = elems[len(elems)-1].GetKey()["name"] // /ports/port[name=<port>]
		}
		if rate := ps.GetInRate(); len(rate) == 4 { // IEEE 754 float32.
			rates[name] = float64(ygot.BinaryToFloat32(rate))
		}
		var total float64
		for _, p := range rxPorts {
			r, seen := rates[p]
			if !seen {
				summary = fmt.Sprintf("no in-rate received yet for ATE %s", p)
				return false
			}
			total += r
		}
		if total <= 0 {
			summary = fmt.Sprintf("no traffic received on %v", rxPorts)
			return false
		}
		met := true
		var parts []string
		for _, p := range ports {
			share := rates[p] / total
			parts = append(parts, fmt.Sprintf("%s=%.2f%%", p, 100*share))
			met = met && cond(share)
		}
		summary = fmt.Sprintf("share of the rate received on %v: %s", rxPorts, strings.Join(parts, ", "))
		return met
	}).Await(t)
	if !ok {
		t.Fatalf("ATE ports %v did not become %s within %v (%s)", ports, desc, trafficWaitTime, summary)
	}
	t.Logf("ATE ports %v are %s (%s)", ports, desc, summary)
}

// awaitPortsIdle waits until each ATE port in idlePorts receives at most maxIdleShare of the
// traffic received on rxPorts (see awaitRxShare).
func awaitPortsIdle(t *testing.T, ate *ondatra.ATEDevice, idlePorts, rxPorts []string) {
	t.Helper()
	awaitRxShare(t, ate, idlePorts, rxPorts, "idle", func(share float64) bool { return share <= maxIdleShare })
}

// awaitPortsLoadSharing waits until each ATE port in lbPorts receives at least minLBShare of the
// traffic received on rxPorts (see awaitRxShare).
func awaitPortsLoadSharing(t *testing.T, ate *ondatra.ATEDevice, lbPorts, rxPorts []string) {
	t.Helper()
	awaitRxShare(t, ate, lbPorts, rxPorts, "load-sharing", func(share float64) bool { return share >= minLBShare })
}

// staticNextHops returns the static routes in the static protocol state p as canonical prefix ->
// sorted canonical next-hops, read from .../static/state/prefix and
// .../next-hops/next-hop/state/next-hop. Routes whose prefix leaf is absent are skipped.
func staticNextHops(p *oc.NetworkInstance_Protocol) map[string][]string {
	routes := make(map[string][]string)
	if p == nil {
		return routes
	}
	for _, s := range p.Static {
		if s.GetPrefix() == "" {
			continue
		}
		var nhs []string
		for _, nh := range s.NextHop {
			if addr, ok := nh.GetNextHop().(oc.UnionString); ok {
				nhs = append(nhs, canonicalIP(string(addr)))
			}
		}
		slices.Sort(nhs)
		routes[canonicalPrefix(s.GetPrefix())] = nhs
	}
	return routes
}

// staticNextHopsMismatch returns "" if the static protocol state p has exactly the next-hops in
// want (canonical prefix -> sorted canonical next-hops, as returned by staticNextHops) for every
// prefix in want, and otherwise describes a prefix whose next-hops differ.
func staticNextHopsMismatch(p *oc.NetworkInstance_Protocol, want map[string][]string) string {
	if p == nil {
		return "static protocol is not present in state"
	}
	got := staticNextHops(p)
	for prefix, w := range want {
		if g := got[prefix]; !slices.Equal(g, w) {
			return fmt.Sprintf("%s: got next-hops %v, want %v", prefix, g, w)
		}
	}
	return ""
}

// staticRoutesLeft returns the prefixes that are still present in the static protocol state p
// (.../static-routes/static/state/prefix). An absent static protocol (nil p) has no routes left.
func staticRoutesLeft(p *oc.NetworkInstance_Protocol, prefixes []string) []string {
	got := staticNextHops(p)
	var left []string
	for _, prefix := range prefixes {
		if _, ok := got[canonicalPrefix(prefix)]; ok {
			left = append(left, prefix)
		}
	}
	return left
}

// awaitStaticNextHops uses gnmi.Watch on the DUT's static protocol state
// (/network-instances/network-instance/protocols/protocol[identifier=STATIC]) and returns once the
// DUT reports exactly the wanted next-hops for every prefix in want. Unlike waiting for
// state/prefix, this proves that a Replace adding or removing next-hops of an existing route took
// effect. It backs every configureRoute and replaceStaticRoutes call (RT-1.73.1-RT-1.73.4 Steps 1-2,
// RT-1.73.4 Step 4, RT-1.73.5 Steps 1, 3 and 5) and the RT-1.73.5 Step 8 route persistence check.
//
// The Watch accumulates the streamed state, and a target that serves the default TARGET_DEFINED
// subscription as SAMPLE sends no deletes, so a removed next-hop can stay in the watched state. If
// the Watch times out, a fresh ONCE snapshot (gnmi.Lookup) decides instead.
func awaitStaticNextHops(t *testing.T, dut *ondatra.DUTDevice, want map[string][]string) {
	t.Helper()
	wantNHs := make(map[string][]string, len(want))
	for prefix, nhs := range want {
		w := make([]string, 0, len(nhs))
		for _, nh := range nhs {
			w = append(w, canonicalIP(nh))
		}
		slices.Sort(w)
		wantNHs[canonicalPrefix(prefix)] = w
	}
	sp := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
	_, ok := gnmi.Watch(t, dut, sp.State(), routeWaitTime, func(v *ygnmi.Value[*oc.NetworkInstance_Protocol]) bool {
		p, _ := v.Val() // nil if the static protocol is not present in state.
		return staticNextHopsMismatch(p, wantNHs) == ""
	}).Await(t)
	if !ok {
		p, _ := gnmi.Lookup(t, dut, sp.State()).Val()
		if mismatch := staticNextHopsMismatch(p, wantNHs); mismatch != "" {
			t.Fatalf("Static route state did not converge within %v: %s", routeWaitTime, mismatch)
		}
		t.Logf("Static route next-hops converged in a fresh snapshot only: the watched state did not report the removed next-hops")
	}
	t.Logf("Static route next-hops converged in state for %d prefixes", len(want))
}

// awaitStaticRoutesRemoved uses gnmi.Watch on the DUT's static protocol state and returns once none
// of prefixes is present in .../static-routes/static/state/prefix (RT-1.73.5 Step 9). As in
// awaitStaticNextHops, a fresh ONCE snapshot decides if the Watch times out, because a SAMPLE
// stream does not report the deleted routes.
func awaitStaticRoutesRemoved(t *testing.T, dut *ondatra.DUTDevice, prefixes []string) {
	t.Helper()
	sp := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
	_, ok := gnmi.Watch(t, dut, sp.State(), routeWaitTime, func(v *ygnmi.Value[*oc.NetworkInstance_Protocol]) bool {
		p, _ := v.Val() // nil if the static protocol, and with it every static route, is gone from state.
		return len(staticRoutesLeft(p, prefixes)) == 0
	}).Await(t)
	if !ok {
		p, _ := gnmi.Lookup(t, dut, sp.State()).Val()
		if left := staticRoutesLeft(p, prefixes); len(left) > 0 {
			t.Fatalf("%d of %d static routes still in state %v after Delete, e.g. %s", len(left), len(prefixes), routeWaitTime, left[0])
		}
		t.Logf("Static routes are removed in a fresh snapshot only: the watched state did not report the deletes")
	}
	t.Logf("All %d static routes are removed from state", len(prefixes))
}

// canonicalIP returns s in canonical IP address form, or s unchanged if it is not an IP address.
func canonicalIP(s string) string {
	if ip := net.ParseIP(s); ip != nil {
		return ip.String()
	}
	return s
}

// canonicalPrefix returns s in canonical CIDR form, or s unchanged if it is not a CIDR prefix.
func canonicalPrefix(s string) string {
	if _, n, err := net.ParseCIDR(s); err == nil {
		return n.String()
	}
	return s
}

// scalePrefixes returns the IPv4 and IPv6 prefix of RT-1.73.5 scale route i (0-99):
// 10.1.0.0/24..10.1.99.0/24 and 2001:db8:1000::/64..2001:db8:1099::/64.
func scalePrefixes(i int) (string, string) {
	return fmt.Sprintf("10.1.%d.0/24", i), fmt.Sprintf("2001:db8:10%02d::/64", i)
}

// scaleRoutes returns prefix -> next-hops for the 200 RT-1.73.5 scale routes, where nextHops(i)
// gives the IPv4 and IPv6 next-hops of scale route i.
func scaleRoutes(nextHops func(i int) (v4, v6 []string)) map[string][]string {
	routes := make(map[string][]string)
	for i := 0; i < numScaleRoutes; i++ {
		v4Prefix, v6Prefix := scalePrefixes(i)
		routes[v4Prefix], routes[v6Prefix] = nextHops(i)
	}
	return routes
}

// replaceStaticRoutes replaces the given static routes in a single gNMI SetRequest (one Replace
// per prefix) and waits until the DUT reports exactly those next-hops in state.
func replaceStaticRoutes(t *testing.T, dut *ondatra.DUTDevice, routes map[string][]string) {
	t.Helper()
	b := &gnmi.SetBatch{}
	for prefix, nhs := range routes {
		cfg := &cfgplugins.StaticRouteCfg{
			NetworkInstance: deviations.DefaultNetworkInstance(dut),
			Prefix:          prefix,
			NextHops:        map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{},
		}
		for idx, nh := range nhs {
			cfg.NextHops[strconv.Itoa(idx)] = oc.UnionString(nh)
		}
		if _, err := cfgplugins.NewStaticRouteCfg(b, cfg, dut); err != nil {
			t.Fatalf("NewStaticRouteCfg(%s): %v", prefix, err)
		}
	}
	b.Set(t, dut)
	awaitStaticNextHops(t, dut, routes)
}

// TestStaticRouteResiliency is the main test entry point that orchestrates the execution of
// RT-1.73: Static Route Resilience Test (subtests RT-1.73.1 through RT-1.73.5).
func TestStaticRouteResiliency(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	top := gosnappi.NewConfig()

	// Test Environment Setup: Configure DUT and ATE (OTG) interfaces, VLAN 10 SVI, LAG 1, and LAG 2.
	lag1Name := configureDUT(t, dut)
	configureOTG(t, ate, top)

	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	for _, p := range []string{"port7", "port8"} {
		dutPortName := dut.Port(t, p).Name()
		oper := gnmi.Get(t, dut, gnmi.OC().Interface(dutPortName).OperStatus().State())
		t.Logf("DUT %s (%s) oper status: %v", p, dutPortName, oper)
	}
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv6")

	// RT-1.73.1 - Validate Static Route with VLAN Interface (SVI)
	t.Run("RT-1.73.1: Validate Static Route with VLAN Interface (SVI)", func(t *testing.T) {
		// RT-1.73.1 Step 1 & Step 2: Configure static routes for 203.0.113.0/24 and 2001:db8:213::/64
		// pointing to next-hops 198.51.100.2 and 2001:db8:100::2 (ATE Port 1 via VLAN 10 SVI) and push via gNMI Set Replace.
		configureRoute(t, dut, "203.0.113.0/24", "2001:db8:213::/64", []string{"198.51.100.2"}, []string{"2001:db8:100::2"})

		// RT-1.73.1 Step 3: Start IPv4 and IPv6 traffic from ATE Port 8 destined to 203.0.113.1 and 2001:db8:213::1.
		top.Flows().Clear()
		createTraffic(top, "traffic_svi", "203.0.113.1", "2001:db8:213::1", []string{"port1.IPv4"}, []string{"port1.IPv6"})
		ate.OTG().PushConfig(t, top)
		ate.OTG().StartProtocols(t)
		otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")
		otgutils.WaitForARP(t, ate.OTG(), top, "IPv6")
		ate.OTG().StartTraffic(t)
		trafficRunning := true
		defer func() {
			if trafficRunning {
				ate.OTG().StopTraffic(t)
			}
		}()

		// RT-1.73.1 Step 4: Verify the DUT routes the traffic out of DUT Port 1 towards ATE Port 1 (the next-hops
		// 198.51.100.2 and 2001:db8:100::2 are ATE Port 1, and flow rx is only counted at the ATE Port 1 endpoints).
		// ExpectedTrafficLoss waits until every flow is received with at most 1% loss; verifyTrafficWindow then
		// verifies at most 1% loss and reception on ATE Port 1 over a verifyWindow of traffic.
		for _, flow := range top.Flows().Items() {
			otgutils.ExpectedTrafficLoss(t, ate.OTG(), flow.Name(), 0, 1)
		}
		verifyTrafficWindow(t, ate, top, 0, 1, []string{"port1"}, nil)

		// RT-1.73.1 Step 5: Stop traffic.
		ate.OTG().StopTraffic(t)
		trafficRunning = false
	})

	// RT-1.73.2 - Validate Static Route over LAG Interface
	t.Run("RT-1.73.2: Validate Static Route over LAG Interface", func(t *testing.T) {
		// RT-1.73.2 Step 1 & Step 2: Configure static routes for 203.0.114.0/24 and 2001:db8:214::/64
		// with next-hops 198.51.101.2 and 2001:db8:101::2 (ATE LAG 1) and push configuration to DUT.
		configureRoute(t, dut, "203.0.114.0/24", "2001:db8:214::/64", []string{"198.51.101.2"}, []string{"2001:db8:101::2"})

		// RT-1.73.2 Step 3: Start IPv4 and IPv6 traffic from ATE Port 8 destined to 203.0.114.1/24 and 2001:db8:214::1/64.
		top.Flows().Clear()
		createTraffic(top, "traffic_lag1", "203.0.114.1/24", "2001:db8:214::1/64", []string{"LAG1_IPv4"}, []string{"LAG1_IPv6"})
		ate.OTG().PushConfig(t, top)
		ate.OTG().StartProtocols(t)
		otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")
		otgutils.WaitForARP(t, ate.OTG(), top, "IPv6")
		ate.OTG().StartTraffic(t)

		// RT-1.73.2 Step 4: Verify the DUT routes traffic out of DUT Ports 3 and 4, load-balancing across LAG 1 member links.
		for _, flow := range top.Flows().Items() {
			otgutils.ExpectedTrafficLoss(t, ate.OTG(), flow.Name(), 0, 1)
		}
		verifyTrafficWindow(t, ate, top, 0, 1, []string{"port3", "port4"}, nil)
		// Do not stop traffic: RT-1.73.3 builds directly on this subtest with traffic still flowing.
	})

	// RT-1.73.3 - Control Plane Resilience on LAG Failure
	t.Run("RT-1.73.3: Control Plane Resilience on LAG Failure", func(t *testing.T) {
		// Builds directly on RT-1.73.2 with its traffic still flowing; traffic is stopped in Step 5.
		trafficRunning := true
		defer func() {
			if trafficRunning {
				ate.OTG().StopTraffic(t)
			}
		}()
		p3 := ate.Port(t, atePorts[2].Name)
		p4 := ate.Port(t, atePorts[3].Name)

		// RT-1.73.3 Step 1: Simulate a LAG failure by disabling ATE Port 3 and ATE Port 4.
		portStateAction := gosnappi.NewControlState()
		portStateAction.Port().Link().SetPortNames([]string{p3.ID(), p4.ID()}).SetState(gosnappi.StatePortLinkState.DOWN)
		ate.OTG().SetControlState(t, portStateAction)
		portsRestored := false
		defer func() {
			if !portsRestored {
				portStateAction.Port().Link().SetPortNames([]string{p3.ID(), p4.ID()}).SetState(gosnappi.StatePortLinkState.UP)
				ate.OTG().SetControlState(t, portStateAction)
			}
		}()

		// RT-1.73.3 Step 2: Verify via gNMI state (/interfaces/interface/state/oper-status) that DUT LAG 1 transitions
		// to DOWN / LOWER_LAYER_DOWN, and that the still-running traffic is dropped. awaitFlowsDropped watches the
		// flow receive rate fall below 1 frame per second so that packets received before the failure, which OTG
		// counters may report late, stay out of the drop verification window.
		awaitInterfaceDown(t, dut, lag1Name, trafficWaitTime)
		awaitFlowsDropped(t, ate, top, trafficWaitTime)
		verifyTrafficWindow(t, ate, top, 99.9, 100, nil, nil)

		// RT-1.73.3 Step 3: Perform an unrelated gNMI Set operation (updating description on DUT Port 7)
		// and verify it succeeds without throwing an "unreachable next-hop" error and is applied in state.
		dutP7 := dut.Port(t, "port7").Name()
		gnmi.Update(t, dut, gnmi.OC().Interface(dutP7).Description().Config(), "test_description")
		gnmi.Await(t, dut, gnmi.OC().Interface(dutP7).Description().State(), trafficWaitTime, "test_description")

		// RT-1.73.3 Step 4: Re-enable ATE Ports 3 and 4 and verify DUT LAG 1 and both of its member ports transition back to UP.
		portStateAction.Port().Link().SetPortNames([]string{p3.ID(), p4.ID()}).SetState(gosnappi.StatePortLinkState.UP)
		ate.OTG().SetControlState(t, portStateAction)
		portsRestored = true

		gnmi.Await(t, dut, gnmi.OC().Interface(lag1Name).OperStatus().State(), trafficWaitTime, oc.Interface_OperStatus_UP)
		for _, p := range []string{"port3", "port4"} {
			gnmi.Await(t, dut, gnmi.OC().Interface(dut.Port(t, p).Name()).OperStatus().State(), trafficWaitTime, oc.Interface_OperStatus_UP)
		}
		otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")
		otgutils.WaitForARP(t, ate.OTG(), top, "IPv6")

		// RT-1.73.3 Step 4: Verify traffic forwarding resumes automatically (traffic is not restarted) and is
		// load-balanced across LAG 1 (DUT Ports 3 and 4).
		awaitTrafficFlowing(t, ate, top, trafficWaitTime)
		verifyTrafficWindow(t, ate, top, 0, 1, []string{"port3", "port4"}, nil)

		// RT-1.73.3 Step 5: Stop traffic.
		ate.OTG().StopTraffic(t)
		trafficRunning = false
	})

	// RT-1.73.4 - Validate ECMP and FIB Reprogramming Across Multiple LAGs
	t.Run("RT-1.73.4: Validate ECMP and FIB Reprogramming Across Multiple LAGs", func(t *testing.T) {
		// RT-1.73.4 Step 1 & Step 2: Configure static routes for 203.0.115.0/24 and 2001:db8:215::/64
		// with two next-hops: ATE LAG 1 (198.51.101.2 / 2001:db8:101::2) and ATE LAG 2 first subinterface (198.51.111.2 / 2001:db8:111::2).
		configureRoute(t, dut, "203.0.115.0/24", "2001:db8:215::/64", []string{"198.51.101.2", "198.51.111.2"}, []string{"2001:db8:101::2", "2001:db8:111::2"})

		// RT-1.73.4 Step 2 & Step 3: Start IPv4/IPv6 traffic from ATE Port 8 and verify load-balancing across all 4 LAG member ports (Ports 3, 4, 5, and 6).
		top.Flows().Clear()
		createTraffic(top, "traffic_ecmp", "203.0.115.1/24", "2001:db8:215::1/64", []string{"LAG1_IPv4", "LAG2_IPv4_101"}, []string{"LAG1_IPv6", "LAG2_IPv6_101"})
		ate.OTG().PushConfig(t, top)
		ate.OTG().StartProtocols(t)
		otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")
		otgutils.WaitForARP(t, ate.OTG(), top, "IPv6")
		ate.OTG().StartTraffic(t)
		trafficRunning := true
		defer func() {
			if trafficRunning {
				ate.OTG().StopTraffic(t)
			}
		}()
		awaitTrafficFlowing(t, ate, top, trafficWaitTime)
		verifyTrafficWindow(t, ate, top, 0, 1, []string{"port3", "port4", "port5", "port6"}, nil)

		// RT-1.73.4 Step 4: While traffic is flowing, update the static route via gNMI Set Replace to remove LAG 2 next-hop,
		// leaving only LAG 1 (198.51.101.2 and 2001:db8:101::2). configureRoute verifies the removal in next-hop state, and
		// awaitPortsIdle watches ATE Ports 5 and 6 (LAG 2) go idle, i.e. the DUT FIB no longer forwards to LAG 2.
		before := readCounters(t, ate, top)
		configureRoute(t, dut, "203.0.115.0/24", "2001:db8:215::/64", []string{"198.51.101.2"}, []string{"2001:db8:101::2"})
		awaitPortsIdle(t, ate, []string{"port5", "port6"}, []string{"port3", "port4", "port5", "port6"})

		// RT-1.73.4 Step 5: Verify the FIB is reprogrammed without traffic loss, and that traffic now flows exclusively
		// out of DUT Ports 3 and 4 (LAG 1) with no traffic egressing DUT Ports 5 and 6 (LAG 2).
		verifyTrafficSince(t, ate, top, before, 0, 1, nil, nil)
		verifyTrafficWindow(t, ate, top, 0, 1, []string{"port3", "port4"}, []string{"port5", "port6"})

		// RT-1.73.4 Step 6: Stop traffic and verify the loss over the whole run on the final counters.
		ate.OTG().StopTraffic(t)
		trafficRunning = false
		for _, flow := range top.Flows().Items() {
			otgutils.ExpectedTrafficLoss(t, ate.OTG(), flow.Name(), 0, 1)
		}
	})

	// RT-1.73.5 - Scale, Dynamic FIB Re-programming, and Route Persistence
	t.Run("RT-1.73.5: Scale, Dynamic FIB Re-programming, and Route Persistence", func(t *testing.T) {
		sp := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))

		// Next-hops of scale route i: the ATE LAG 2 subinterface IPs for VLAN 101+i%10
		// (198.51.111.2..198.51.120.2 / 2001:db8:111::2..2001:db8:120::2) and/or ATE Port 7.
		lag2NHs := func(i int) ([]string, []string) {
			n := 111 + i%10
			return []string{fmt.Sprintf("198.51.%d.2", n)}, []string{fmt.Sprintf("2001:db8:%d::2", n)}
		}
		lag2AndP7NHs := func(i int) ([]string, []string) {
			v4, v6 := lag2NHs(i)
			return append(v4, atePorts[6].IPv4), append(v6, atePorts[6].IPv6)
		}
		p7NHs := func(int) ([]string, []string) {
			return []string{atePorts[6].IPv4}, []string{atePorts[6].IPv6}
		}

		// RT-1.73.5 Step 1 (Scale Routes): Configure 100 IPv4 static routes (10.1.0.0/24..10.1.99.0/24)
		// and 100 IPv6 static routes (2001:db8:1000::/64..2001:db8:1099::/64) distributed evenly across the 10 ATE LAG 2 subinterface IPs.
		replaceStaticRoutes(t, dut, scaleRoutes(lag2NHs))

		// RT-1.73.5 Step 2 (Verify Scale): Start traffic from ATE Port 8 to all 200 route destinations
		// (grouped into 20 Flow Groups to stay within ATE Port 8's 64 Flow Group hardware limit)
		// and verify traffic load-balances out of DUT Ports 5 and 6 (LAG 2) and not DUT Port 7.
		// Traffic keeps running until Step 9.
		top.Flows().Clear()
		createScaleTraffic(top)
		ate.OTG().PushConfig(t, top)
		ate.OTG().StartProtocols(t)
		otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")
		otgutils.WaitForARP(t, ate.OTG(), top, "IPv6")
		ate.OTG().StartTraffic(t)
		trafficRunning := true
		defer func() {
			if trafficRunning {
				ate.OTG().StopTraffic(t)
			}
		}()
		awaitTrafficFlowing(t, ate, top, trafficWaitTime)
		verifyTrafficWindow(t, ate, top, 0, 1, []string{"port5", "port6"}, []string{"port7"})

		// RT-1.73.5 Step 3 (FIB Update - Add Next-Hop): While traffic is flowing, update all 200 static routes via a single gNMI Set Replace
		// to add an additional next-hop pointing to ATE Port 7 (198.51.102.2 and 2001:db8:102::2). replaceStaticRoutes verifies the
		// next-hops in state, and awaitPortsLoadSharing watches ATE Port 7 start receiving its share, i.e. the DUT FIB uses the new next-hop.
		before := readCounters(t, ate, top)
		replaceStaticRoutes(t, dut, scaleRoutes(lag2AndP7NHs))
		awaitPortsLoadSharing(t, ate, []string{"port7"}, []string{"port5", "port6", "port7"})

		// RT-1.73.5 Step 4 (Verify Add): Verify the DUT seamlessly load-balances traffic across DUT Ports 5, 6 (LAG 2) and DUT Port 7 without traffic loss.
		verifyTrafficSince(t, ate, top, before, 0, 1, nil, nil)
		verifyTrafficWindow(t, ate, top, 0, 1, []string{"port5", "port6", "port7"}, nil)

		// RT-1.73.5 Step 5 (FIB Update - Remove Next-Hop): Update all 200 static routes via gNMI Set Replace
		// to remove the LAG 2 subinterface next-hops, leaving only ATE Port 7 (198.51.102.2 and 2001:db8:102::2).
		// awaitPortsIdle watches ATE Ports 5 and 6 (LAG 2) go idle, i.e. the DUT FIB no longer forwards to LAG 2.
		before = readCounters(t, ate, top)
		replaceStaticRoutes(t, dut, scaleRoutes(p7NHs))
		awaitPortsIdle(t, ate, []string{"port5", "port6"}, []string{"port5", "port6", "port7"})

		// RT-1.73.5 Step 6 (Verify Remove): Verify a seamless transition and that traffic now flows strictly out of
		// DUT Port 7 (at most maxIdleShare on each of DUT Ports 5 and 6, and at most 1% flow loss).
		verifyTrafficSince(t, ate, top, before, 0, 1, nil, nil)
		verifyTrafficWindow(t, ate, top, 0, 1, []string{"port7"}, []string{"port5", "port6"})

		// RT-1.73.5 Step 7 (Linecard OIR / Disable): With traffic still flowing, admin-disable DUT Port 7 via
		// /interfaces/interface/config/enabled and verify its oper-status transitions to DOWN and traffic drops.
		// Linecard OIR is not used: on the reference testbed DUT Port 7 shares a linecard with the traffic source
		// port (DUT Port 8) and the LAG member ports, so OIR would take those down too. The README allows
		// admin-disabling DUT Port 7 instead.
		t.Log("RT-1.73.5 Step 7: admin-disabling DUT Port 7 (README fallback for linecard OIR)")
		dutP7 := dut.Port(t, "port7").Name()
		gnmi.Update(t, dut, gnmi.OC().Interface(dutP7).Enabled().Config(), false)
		p7Restored := false
		defer func() {
			if !p7Restored {
				gnmi.Update(t, dut, gnmi.OC().Interface(dutP7).Enabled().Config(), true)
			}
		}()
		awaitInterfaceDown(t, dut, dutP7, trafficWaitTime)
		// Keep packets received before the disable, which OTG counters may report late, out of the drop window.
		awaitFlowsDropped(t, ate, top, trafficWaitTime)
		verifyTrafficWindow(t, ate, top, 99.9, 100, nil, nil)

		// RT-1.73.5 Step 8 (Verify Persistence): Re-enable DUT Port 7 and verify the static routes persist
		// and traffic forwarding resumes automatically over DUT Port 7, without restarting traffic.
		gnmi.Update(t, dut, gnmi.OC().Interface(dutP7).Enabled().Config(), true)
		p7Restored = true
		gnmi.Await(t, dut, gnmi.OC().Interface(dutP7).OperStatus().State(), trafficWaitTime, oc.Interface_OperStatus_UP)
		awaitStaticNextHops(t, dut, scaleRoutes(p7NHs))
		otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")
		otgutils.WaitForARP(t, ate.OTG(), top, "IPv6")
		awaitTrafficFlowing(t, ate, top, trafficWaitTime)
		verifyTrafficWindow(t, ate, top, 0, 1, []string{"port7"}, []string{"port5", "port6"})

		// RT-1.73.5 Step 9 (Stop & Delete): Stop traffic, delete all 200 static routes using a gNMI Delete
		// operation, and verify they are removed from state.
		ate.OTG().StopTraffic(t)
		trafficRunning = false
		b := &gnmi.SetBatch{}
		var prefixes []string
		for i := 0; i < numScaleRoutes; i++ {
			v4Prefix, v6Prefix := scalePrefixes(i)
			gnmi.BatchDelete(b, sp.Static(v4Prefix).Config())
			gnmi.BatchDelete(b, sp.Static(v6Prefix).Config())
			prefixes = append(prefixes, v4Prefix, v6Prefix)
		}
		b.Set(t, dut)
		awaitStaticRoutesRemoved(t, dut, prefixes)

		// RT-1.73.5 Step 10 (Verify Flush): Start traffic briefly, stop it, and verify on the final counters
		// that all traffic was dropped by the DUT, confirming the FIB was fully flushed. "Briefly" is
		// verifyWindow worth of packets. OTG resets the flow counters on StartTraffic, which the flow state
		// may only show after the baseline is read; awaitFlowsTransmitted then measures from the reset, so
		// counters left over from the traffic stopped in Step 9 cannot end the window early.
		ate.OTG().StartTraffic(t)
		trafficRunning = true // Let the deferred StopTraffic clean up if the flush window fails.
		awaitFlowsTransmitted(t, ate, top, readCounters(t, ate, top), verifyWindow)
		ate.OTG().StopTraffic(t)
		trafficRunning = false
		for _, flow := range top.Flows().Items() {
			otgutils.ExpectedTrafficLoss(t, ate.OTG(), flow.Name(), 99.9, 100)
		}
	})
}
