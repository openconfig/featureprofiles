package static_route_resiliency_test

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"      // Changed from ondatra/ondatra
	"github.com/openconfig/ondatra/gnmi" // Changed from gnmi/gnmi
	"github.com/openconfig/ondatra/gnmi/oc"
	otgtelemetry "github.com/openconfig/ondatra/gnmi/otg"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	ipv4PrefixLen = 24
	ipv6PrefixLen = 64
	vlanID        = uint16(10)
	vlanIntfName  = "Vlan10"

	lag1Name    = "Port-Channel1"
	lag2Name    = "Port-Channel2"
	ateLag1Name = "lag1"
	ateLag2Name = "lag2"

	trafficDuration = 10 * time.Second
	packetPerSecond = 100
)

var (
	dutPort1 = attrs.Attributes{Name: "port1"}
	dutPort2 = attrs.Attributes{Name: "port2"}
	dutPort3 = attrs.Attributes{Name: "port3"}
	dutPort4 = attrs.Attributes{Name: "port4"}
	dutPort5 = attrs.Attributes{Name: "port5"}
	dutPort6 = attrs.Attributes{Name: "port6"}
	dutPort7 = attrs.Attributes{Name: "port7", IPv4: "198.51.102.1", IPv4Len: 24, IPv6: "2001:db8:102::1", IPv6Len: 64}
	dutPort8 = attrs.Attributes{Name: "port8", IPv4: "192.0.2.1", IPv4Len: 24, IPv6: "2001:db8:192::1", IPv6Len: 64}

	sviIP = attrs.Attributes{IPv4: "198.51.100.1", IPv4Len: 24, IPv6: "2001:db8:100::1", IPv6Len: 64}

	atePort1 = attrs.Attributes{Name: "port1", MAC: "02:00:01:01:01:01", IPv4: "198.51.100.2", IPv4Len: 24, IPv6: "2001:db8:100::2", IPv6Len: 64}
	atePort2 = attrs.Attributes{Name: "port2", MAC: "02:00:01:01:01:02", IPv4: "198.51.100.3", IPv4Len: 24, IPv6: "2001:db8:100::3", IPv6Len: 64}
	atePort7 = attrs.Attributes{Name: "port7", MAC: "02:00:02:01:01:07", IPv4: "198.51.102.2", IPv4Len: 24, IPv6: "2001:db8:102::2", IPv6Len: 64}
	atePort8 = attrs.Attributes{Name: "port8", MAC: "02:00:02:01:01:08", IPv4: "192.0.2.2", IPv4Len: 24, IPv6: "2001:db8:192::2", IPv6Len: 64}

	dutLag1 = attrs.Attributes{Name: lag1Name, IPv4: "198.51.101.1", IPv4Len: 24, IPv6: "2001:db8:101::1", IPv6Len: 64}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func lacpActivityType(v oc.E_Lacp_LacpActivityType) *oc.E_Lacp_LacpActivityType { return &v }
func lacpPeriodType(v oc.E_Lacp_LacpPeriodType) *oc.E_Lacp_LacpPeriodType       { return &v }

func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()

	cfgplugins.ConfigureVlan(t, dut, cfgplugins.VlanParams{VlanID: vlanID})

	portBatch := &gnmi.SetBatch{}

	dutPortsGroup1 := []attrs.Attributes{dutPort1, dutPort2}
	for _, p := range dutPortsGroup1 {
		iObj := &oc.Interface{Name: ygot.String(dut.Port(t, p.Name).Name())}
		iObj.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
		if deviations.InterfaceEnabled(dut) {
			iObj.Enabled = ygot.Bool(true)
		}
		cfgplugins.ConfigureAccessVlan(cfgplugins.AccessVlanParams{
			Intf:   iObj,
			VlanID: vlanID,
		})
		gnmi.BatchReplace(portBatch, gnmi.OC().Interface(dut.Port(t, p.Name).Name()).Config(), iObj)
	}

	dutPortsGroup2 := []attrs.Attributes{dutPort7, dutPort8}
	for _, p := range dutPortsGroup2 {
		iObj := &oc.Interface{Name: ygot.String(dut.Port(t, p.Name).Name())}
		iObj.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
		if deviations.InterfaceEnabled(dut) {
			iObj.Enabled = ygot.Bool(true)
		}
		s := iObj.GetOrCreateSubinterface(0)
		cfgplugins.ConfigureSubinterfaceIPs(s, dut, p.IPv4, p.IPv4Len, p.IPv6, p.IPv6Len)
		gnmi.BatchReplace(portBatch, gnmi.OC().Interface(dut.Port(t, p.Name).Name()).Config(), iObj)
	}
	portBatch.Set(t, dut)

	cfgplugins.ConfigureSVI(t, dut, cfgplugins.SVIParams{
		IntfName: vlanIntfName,
		IPv4:     sviIP.IPv4,
		IPv4Len:  sviIP.IPv4Len,
		IPv6:     sviIP.IPv6,
		IPv6Len:  sviIP.IPv6Len,
	})

	lag1Batch := &gnmi.SetBatch{}
	cfgplugins.NewAggregateInterface(t, dut, lag1Batch, &cfgplugins.DUTAggData{
		LagName:         lag1Name,
		OndatraPortsIdx: []int{2, 3}, // ports 3 and 4 (0-indexed)
		AggType:         oc.IfAggregate_AggregationType_LACP,
		Attributes:      dutLag1,
		LacpParams: &cfgplugins.LACPParams{
			Activity: lacpActivityType(oc.Lacp_LacpActivityType_ACTIVE),
			Period:   lacpPeriodType(oc.Lacp_LacpPeriodType_FAST),
		},
	})

	tTrue := true
	var lag2Subs []*cfgplugins.DUTSubInterfaceData
	for i := uint16(101); i <= 110; i++ {
		lag2Subs = append(lag2Subs, &cfgplugins.DUTSubInterfaceData{
			VlanID:        int(i),
			VlanEnable:    &tTrue,
			IPv4Address:   net.ParseIP(fmt.Sprintf("198.51.%d.1", i+10)),
			IPv4PrefixLen: 24,
			IPv6Address:   net.ParseIP(fmt.Sprintf("2001:db8:%d::1", i+10)),
			IPv6PrefixLen: 64,
		})
	}
	cfgplugins.NewAggregateInterface(t, dut, lag1Batch, &cfgplugins.DUTAggData{
		LagName:         lag2Name,
		OndatraPortsIdx: []int{4, 5},
		AggType:         oc.IfAggregate_AggregationType_LACP,
		SubInterfaces:   lag2Subs,
		LacpParams: &cfgplugins.LACPParams{
			Activity: lacpActivityType(oc.Lacp_LacpActivityType_ACTIVE),
			Period:   lacpPeriodType(oc.Lacp_LacpPeriodType_FAST),
		},
	})
	lag1Batch.Set(t, dut)
}

func configureATE(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config) {
	t.Helper()

	atePort1.AddToOTG(top, ate.Port(t, atePort1.Name), &sviIP)
	atePort2.AddToOTG(top, ate.Port(t, atePort2.Name), &sviIP)

	atePort7.AddToOTG(top, ate.Port(t, atePort7.Name), &dutPort7)
	atePort8.AddToOTG(top, ate.Port(t, atePort8.Name), &dutPort8)

	addLAGPorts := func(lag gosnappi.Lag, members []string, systemMAC string) {
		lag.Protocol().Lacp().SetActorKey(1).SetActorSystemPriority(1).SetActorSystemId(systemMAC)
		for i, portName := range members {
			p := ate.Port(t, portName)
			top.Ports().Add().SetName(p.ID())
			lagPort := lag.Ports().Add().SetPortName(p.ID())
			lagPort.Ethernet().SetMac(systemMAC).SetName(lag.Name() + "-" + p.ID()).SetMtu(1500)
			lagPort.Lacp().SetActorActivity("passive").SetActorPortNumber(uint32(i) + 1).SetActorPortPriority(1).SetLacpduTimeout(0)
		}
	}

	lag1 := top.Lags().Add().SetName(ateLag1Name)
	addLAGPorts(lag1, []string{"port3", "port4"}, "02:00:03:01:01:01")

	dev1 := top.Devices().Add().SetName("ateLag1Dev")
	eth1 := dev1.Ethernets().Add().SetName("ateLag1Eth").SetMac("02:00:03:01:01:01")
	eth1.Connection().SetLagName(lag1.Name())
	eth1.Ipv4Addresses().Add().SetName("ateLag1IPv4").SetAddress("198.51.101.2").SetGateway("198.51.101.1").SetPrefix(24)
	eth1.Ipv6Addresses().Add().SetName("ateLag1IPv6").SetAddress("2001:db8:101::2").SetGateway("2001:db8:101::1").SetPrefix(64)

	lag2 := top.Lags().Add().SetName(ateLag2Name)
	addLAGPorts(lag2, []string{"port5", "port6"}, "02:00:05:01:01:01")

	for i := uint16(101); i <= 110; i++ {
		dev2 := top.Devices().Add().SetName(fmt.Sprintf("ateLag2Dev_vlan%d", i))
		eth2 := dev2.Ethernets().Add().SetName(fmt.Sprintf("ateLag2Eth_vlan%d", i)).SetMac(fmt.Sprintf("02:00:05:%02x:01:01", i))
		eth2.Connection().SetLagName(lag2.Name())
		eth2.Vlans().Add().SetName(fmt.Sprintf("ateLag2Vlan%d", i)).SetId(uint32(i))
		eth2.Ipv4Addresses().Add().SetName(fmt.Sprintf("ateLag2IPv4_%d", i)).SetAddress(fmt.Sprintf("198.51.%d.2", i+10)).SetGateway(fmt.Sprintf("198.51.%d.1", i+10)).SetPrefix(24)
		eth2.Ipv6Addresses().Add().SetName(fmt.Sprintf("ateLag2IPv6_%d", i)).SetAddress(fmt.Sprintf("2001:db8:%d::2", i+10)).SetGateway(fmt.Sprintf("2001:db8:%d::1", i+10)).SetPrefix(64)
	}
}

func fetchDUTCounters(t *testing.T, dut *ondatra.DUTDevice) map[string]uint64 {
	t.Helper()

	portNames := make([]string, 0, 8)
	batch := gnmi.OCBatch()
	for i := 1; i <= 8; i++ {
		portName := dut.Port(t, fmt.Sprintf("port%d", i)).Name()
		portNames = append(portNames, portName)
		batch.AddPaths(gnmi.OC().Interface(portName).Counters().OutUnicastPkts().State().PathStruct())
	}

	root := gnmi.Get(t, dut, batch.State())
	counters := make(map[string]uint64, len(portNames))
	for _, portName := range portNames {
		counters[portName] = root.GetInterface(portName).GetCounters().GetOutUnicastPkts()
	}
	return counters
}

func TestStaticRouteResiliency(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	configureDUT(t, dut)
	t.Cleanup(func() {
		// Attempt to remove configured VLAN, SVIs and aggregated interfaces to restore DUT state.
		// Remove SVI/interface for VLAN
		_ = gnmi.Delete(t, dut, gnmi.OC().Interface(vlanIntfName).Config())

		// Remove aggregate interfaces (LAGs)
		_ = gnmi.Delete(t, dut, gnmi.OC().Interface(lag1Name).Config())
		_ = gnmi.Delete(t, dut, gnmi.OC().Interface(lag2Name).Config())

		// Remove per-port interface config that was altered by the test (ports 1-8)
		for i := 1; i <= 8; i++ {
			ifName := dut.Port(t, fmt.Sprintf("port%d", i)).Name()
			_ = gnmi.Delete(t, dut, gnmi.OC().Interface(ifName).Config())
		}
	})

	top := gosnappi.NewConfig()
	configureATE(t, ate, top)

	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)

	waitForOTGARP(t, ate, top)

	t.Run("RT-1.73.1 Validate Static Route with VLAN Interface", func(t *testing.T) {
		testRouteWithVLAN(t, dut, ate, top)
	})

	t.Run("RT-1.73.2 Validate Static Route over LAG Interface", func(t *testing.T) {
		testRouteWithLAG(t, dut, ate, top)
	})

	t.Run("RT-1.73.3 Control Plane Resilience on LAG Failure", func(t *testing.T) {
		testLAGFailure(t, dut, ate, top)
	})

	t.Run("RT-1.73.4 Validate ECMP and FIB Reprogramming Across Multiple LAGs", func(t *testing.T) {
		testECMPAndFIB(t, dut, ate, top)
	})

	t.Run("RT-1.73.5 Scale, Dynamic FIB Re-programming, and Route Persistence", func(t *testing.T) {
		testScaleAndPersistence(t, dut, ate, top)
	})
}

func testScaleAndPersistence(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, top gosnappi.Config) {
	applyStaticRoutes(t, dut, []string{"198.51.101.2"}, "203.0.115.0/24", "2001:db8:215::/64")

	top.Flows().Clear()
	for i := 0; i < 100; i++ {
		v4Dst := fmt.Sprintf("10.1.%d.1", i)
		v6Dst := fmt.Sprintf("2001:db8:10%02x::1", i)

		createFlow(t, top, fmt.Sprintf("flow_scale_v4_%d", i), atePort8.Name+".IPv4", []string{"ateLag2IPv4_101", "ateLag2IPv4_110"}, atePort8.IPv4, v4Dst, atePort8.IPv6, v6Dst, false)
		createFlow(t, top, fmt.Sprintf("flow_scale_v6_%d", i), atePort8.Name+".IPv6", []string{"ateLag2IPv6_101", "ateLag2IPv6_110"}, atePort8.IPv4, v4Dst, atePort8.IPv6, v6Dst, true)
	}
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	waitForOTGARP(t, ate, top)

	beforeScale := fetchDUTCounters(t, dut)
	ate.OTG().StartTraffic(t)
	waitForFlowTraffic(t, ate.OTG(), "flow_scale_v4_0", 60*time.Second)
	ate.OTG().StopTraffic(t)
	lag2Ports := []string{dut.Port(t, "port5").Name(), dut.Port(t, "port6").Name()}

	var afterScale map[string]uint64
	var portStateReached bool
	var attempts int
	for attempts = 0; attempts < 60; attempts++ { // Retry for up to 60 iterations
		afterScale = fetchDUTCounters(t, dut)
		if portCountersIncreased(beforeScale, afterScale, lag2Ports) {
			portStateReached = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !portStateReached && attempts >= 60 {
		t.Logf("Warning: timeout waiting for port counters to increase")
	}

	assertFlowLossBelow(t, ate, "flow_scale_v4_0", 60*time.Second, 10)
	if !portCountersIncreased(beforeScale, afterScale, lag2Ports) {
		t.Fatalf("expected scaled flows to forward traffic through LAG2 ports %v", lag2Ports)
	}

	// Step 3 (FIB Update - Add Next-Hop) - Add ATE Port 7
	beforeAdd := fetchDUTCounters(t, dut)
	bAdd := &gnmi.SetBatch{}
	for i := 0; i < 100; i++ {
		v4Prefix := fmt.Sprintf("10.1.%d.0/24", i)
		v6Prefix := fmt.Sprintf("2001:db8:10%02x::/64", i)
		vlanIndex := (i % 10) + 111
		nhV4 := fmt.Sprintf("198.51.%d.2", vlanIndex)
		nhV6 := fmt.Sprintf("2001:db8:%d::2", vlanIndex)

		cfgplugins.NewStaticRouteCfg(bAdd, &cfgplugins.StaticRouteCfg{
			NetworkInstance: deviations.DefaultNetworkInstance(dut),
			Prefix:          v4Prefix,
			NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
				"0": oc.UnionString(nhV4),
				"1": oc.UnionString("198.51.102.2"),
			},
		}, dut)
		cfgplugins.NewStaticRouteCfg(bAdd, &cfgplugins.StaticRouteCfg{
			NetworkInstance: deviations.DefaultNetworkInstance(dut),
			Prefix:          v6Prefix,
			NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
				"0": oc.UnionString(nhV6),
				"1": oc.UnionString("2001:db8:102::2"),
			},
		}, dut)
	}
	bAdd.Set(t, dut)
	// Step 4 (Verify Add)
	ate.OTG().StartTraffic(t)
	waitForFlowTraffic(t, ate.OTG(), "flow_scale_v4_0", 60*time.Second)
	ate.OTG().StopTraffic(t)

	var afterAdd map[string]uint64
	gnmi.Watch(t, dut, func() any {
		afterAdd = fetchDUTCounters(t, dut)
		if portCountersIncreased(beforeAdd, afterAdd, lag2Ports) {
			return true
		}
		return nil
	}, time.Minute, 100*time.Millisecond)

	assertFlowLossBelow(t, ate, "flow_scale_v4_0", 60*time.Second, 10)
	if !portCountersIncreased(beforeAdd, afterAdd, lag2Ports) {
		t.Fatalf("expected FIB add to preserve traffic through LAG2 ports %v", lag2Ports)
	}

	// Step 5 (FIB Update - Remove Next-Hop)
	beforeRemove := fetchDUTCounters(t, dut)
	bRem := &gnmi.SetBatch{}
	for i := 0; i < 100; i++ {
		v4Prefix := fmt.Sprintf("10.1.%d.0/24", i)
		v6Prefix := fmt.Sprintf("2001:db8:10%02x::/64", i)

		cfgplugins.NewStaticRouteCfg(bRem, &cfgplugins.StaticRouteCfg{
			NetworkInstance: deviations.DefaultNetworkInstance(dut),
			Prefix:          v4Prefix,
			NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
				"0": oc.UnionString("198.51.102.2"),
			},
		}, dut)
		cfgplugins.NewStaticRouteCfg(bRem, &cfgplugins.StaticRouteCfg{
			NetworkInstance: deviations.DefaultNetworkInstance(dut),
			Prefix:          v6Prefix,
			NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
				"0": oc.UnionString("2001:db8:102::2"),
			},
		}, dut)
	}
	bRem.Set(t, dut)
	// Step 6 (Verify Remove)
	ate.OTG().StartTraffic(t)
	waitForFlowTraffic(t, ate.OTG(), "flow_scale_v4_0", 60*time.Second)
	ate.OTG().StopTraffic(t)

	var afterRemove map[string]uint64
	gnmi.Watch(t, dut, func() any {
		afterRemove = fetchDUTCounters(t, dut)
		if portCountersIncreased(beforeRemove, afterRemove, lag2Ports) {
			return true
		}
		return nil
	}, time.Minute, 100*time.Millisecond)

	assertFlowLossBelow(t, ate, "flow_scale_v4_0", 60*time.Second, 10)
	if !portCountersIncreased(beforeRemove, afterRemove, lag2Ports) {
		t.Fatalf("expected FIB remove to preserve traffic through LAG2 ports %v", lag2Ports)
	}

	// Step 7 (Simulate Linecard OIR by Admin-disable)
	port7Name := dut.Port(t, "port7").Name()
	origEnabled := gnmi.Get(t, dut, gnmi.OC().Interface(port7Name).Enabled().State())
	t.Cleanup(func() {
		gnmi.Replace(t, dut, gnmi.OC().Interface(port7Name).Enabled().Config(), origEnabled)
	})
	gnmi.Replace(t, dut, gnmi.OC().Interface(port7Name).Enabled().Config(), false)
	gnmi.Await(t, dut, gnmi.OC().Interface(port7Name).Enabled().State(), time.Minute, false)

	// Step 8 (Verify Persistence)
	gnmi.Replace(t, dut, gnmi.OC().Interface(dut.Port(t, "port7").Name()).Enabled().Config(), true)
	gnmi.Await(t, dut, gnmi.OC().Interface(dut.Port(t, "port7").Name()).Enabled().State(), time.Minute, true)

	ate.OTG().StartTraffic(t)
	waitForFlowTraffic(t, ate.OTG(), "flow_scale_v4_0", 60*time.Second)
	assertFlowLossBelow(t, ate, "flow_scale_v4_0", 60*time.Second, 10)
	ate.OTG().StopTraffic(t)

	// Step 9 & 10 (Stop & Delete)
	ate.OTG().StopTraffic(t)

	ni := deviations.DefaultNetworkInstance(dut)
	sp := gnmi.OC().NetworkInstance(ni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
	for i := 0; i < 100; i++ {
		gnmi.Delete(t, dut, sp.Static(fmt.Sprintf("10.1.%d.0/24", i)).Config())
		gnmi.Delete(t, dut, sp.Static(fmt.Sprintf("2001:db8:10%02x::/64", i)).Config())
	}

	ate.OTG().StartTraffic(t)
	waitForFlowTraffic(t, ate.OTG(), "flow_scale_v4_0", 30*time.Second)
	assertFlowLossAbove(t, ate, "flow_scale_v4_0", 30*time.Second, 90)
	ate.OTG().StopTraffic(t)
}

func createFlow(t *testing.T, top gosnappi.Config, name string, srcRx string, dstRx []string, srcIPv4, dstIPv4 string, srcIPv6, dstIPv6 string, isIPv6 bool) gosnappi.Flow {
	flow := top.Flows().Add().SetName(name)
	flow.Metrics().SetEnable(true)
	flow.TxRx().Device().SetTxNames([]string{srcRx}).SetRxNames(dstRx)
	eth := flow.Packet().Add().Ethernet()
	eth.Src().SetValue(atePort8.MAC)
	eth.Dst().Auto()

	if isIPv6 {
		ip := flow.Packet().Add().Ipv6()
		ip.Src().SetValue(srcIPv6)
		ip.Dst().SetValue(dstIPv6)
	} else {
		ip := flow.Packet().Add().Ipv4()
		ip.Src().SetValue(srcIPv4)
		ip.Dst().SetValue(dstIPv4)
	}

	flow.Size().SetFixed(512)
	flow.Rate().SetPps(packetPerSecond)
	return flow
}

func applyStaticRoutes(t *testing.T, dut *ondatra.DUTDevice, nexthops []string, dstIPv4 string, dstIPv6 string) {
	t.Helper()
	b := &gnmi.SetBatch{}
	for i, nh := range nexthops {
		cfgplugins.NewStaticRouteCfg(b, &cfgplugins.StaticRouteCfg{
			NetworkInstance: deviations.DefaultNetworkInstance(dut),
			Prefix:          dstIPv4,
			NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
				fmt.Sprintf("%d", i): oc.UnionString(nh),
			},
		}, dut)
	}
	for i, nh := range nexthops {
		cfgplugins.NewStaticRouteCfg(b, &cfgplugins.StaticRouteCfg{
			NetworkInstance: deviations.DefaultNetworkInstance(dut),
			Prefix:          dstIPv6,
			NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
				fmt.Sprintf("%d", i): oc.UnionString(nh),
			},
		}, dut)
	}
	b.Set(t, dut)
}

func testRouteWithVLAN(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, top gosnappi.Config) {
	applyStaticRoutes(t, dut, []string{"198.51.100.2"}, "203.0.113.0/24", "2001:db8:213::/64")

	top.Flows().Clear()
	rxNames := []string{atePort1.Name + ".IPv4"}
	createFlow(t, top, "flow_vlan_v4", atePort8.Name+".IPv4", rxNames, atePort8.IPv4, "203.0.113.1", atePort8.IPv6, "2001:db8:213::1", false)
	rxNamesV6 := []string{atePort1.Name + ".IPv6"}
	createFlow(t, top, "flow_vlan_v6", atePort8.Name+".IPv6", rxNamesV6, atePort8.IPv4, "203.0.113.1", atePort8.IPv6, "2001:db8:213::1", true)
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	waitForOTGARP(t, ate, top)

	before := fetchDUTCounters(t, dut)
	ate.OTG().StartTraffic(t)
	waitForFlowTraffic(t, ate.OTG(), "flow_vlan_v4", 30*time.Second)
	ate.OTG().StopTraffic(t)

	port1 := dut.Port(t, "port1").Name()
	port2 := dut.Port(t, "port2").Name()

	var after map[string]uint64
	gnmi.Watch(t, dut, func() any {
		after = fetchDUTCounters(t, dut)
		if (after[port1]-before[port1]) >= 100 || (after[port2]-before[port2]) >= 100 {
			return true
		}
		return nil
	}, time.Minute, 100*time.Millisecond)

	if (after[port1]-before[port1]) < 100 && (after[port2]-before[port2]) < 100 {
		t.Errorf("Traffic not routed out of Port 1 or 2 properly")
	}
}

func testRouteWithLAG(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, top gosnappi.Config) {
	applyStaticRoutes(t, dut, []string{"198.51.101.2"}, "203.0.114.0/24", "2001:db8:214::/64")

	top.Flows().Clear()
	createFlow(t, top, "flow_lag_v4", atePort8.Name+".IPv4", []string{"ateLag1IPv4"}, atePort8.IPv4, "203.0.114.1", atePort8.IPv6, "2001:db8:214::1", false)
	createFlow(t, top, "flow_lag_v6", atePort8.Name+".IPv6", []string{"ateLag1IPv6"}, atePort8.IPv4, "203.0.114.1", atePort8.IPv6, "2001:db8:214::1", true)
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	waitForOTGARP(t, ate, top)

	before := fetchDUTCounters(t, dut)
	ate.OTG().StartTraffic(t)
	waitForFlowTraffic(t, ate.OTG(), "flow_lag_v4", 30*time.Second)
	ate.OTG().StopTraffic(t)

	port3 := dut.Port(t, "port3").Name()
	port4 := dut.Port(t, "port4").Name()

	var after map[string]uint64
	gnmi.Watch(t, dut, func() any {
		after = fetchDUTCounters(t, dut)
		if (after[port3]-before[port3]) >= 100 || (after[port4]-before[port4]) >= 100 {
			return true
		}
		return nil
	}, time.Minute, 100*time.Millisecond)

	if (after[port3]-before[port3]) < 100 && (after[port4]-before[port4]) < 100 {
		t.Errorf("Traffic not routed out of LAG ports 3 or 4 properly")
	}
}

func testLAGFailure(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, top gosnappi.Config) {
	applyStaticRoutes(t, dut, []string{"198.51.101.2"}, "203.0.114.0/24", "2001:db8:214::/64")

	top.Flows().Clear()
	createFlow(t, top, "flow_lag_v4", atePort8.Name+".IPv4", []string{"ateLag1IPv4"}, atePort8.IPv4, "203.0.114.1", atePort8.IPv6, "2001:db8:214::1", false)
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	waitForOTGARP(t, ate, top)

	ate.OTG().StartTraffic(t)
	t.Cleanup(func() {
		ate.OTG().StopTraffic(t)
	})
	waitForFlowTraffic(t, ate.OTG(), "flow_lag_v4", 30*time.Second)

	portStateAction := gosnappi.NewControlState()
	portStateAction.Port().Link().SetPortNames([]string{ate.Port(t, "port3").ID(), ate.Port(t, "port4").ID()}).SetState(gosnappi.StatePortLinkState.DOWN)
	ate.OTG().SetControlState(t, portStateAction)

	gnmi.Await(t, dut, gnmi.OC().Interface(lag1Name).OperStatus().State(), time.Minute, oc.Interface_OperStatus_DOWN)

	gnmi.Replace(t, dut, gnmi.OC().Interface(dut.Port(t, "port7").Name()).Description().Config(), "test-lag-failure")

	portStateActionUp := gosnappi.NewControlState()
	portStateActionUp.Port().Link().SetPortNames([]string{ate.Port(t, "port3").ID(), ate.Port(t, "port4").ID()}).SetState(gosnappi.StatePortLinkState.UP)
	ate.OTG().SetControlState(t, portStateActionUp)

	gnmi.Await(t, dut, gnmi.OC().Interface(lag1Name).OperStatus().State(), time.Minute, oc.Interface_OperStatus_UP)
	waitForFlowTraffic(t, ate.OTG(), "flow_lag_v4", 30*time.Second)
	assertFlowLossBelow(t, ate, "flow_lag_v4", 60*time.Second, 5)
	ate.OTG().StopTraffic(t)
}

func testECMPAndFIB(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, top gosnappi.Config) {
	applyStaticRoutes(t, dut, []string{"198.51.101.2", "198.51.111.2"}, "203.0.115.0/24", "2001:db8:215::/64")

	top.Flows().Clear()
	createFlow(t, top, "flow_ecmp_v4", atePort8.Name+".IPv4", []string{"ateLag1IPv4"}, atePort8.IPv4, "203.0.115.1", atePort8.IPv6, "2001:db8:215::1", false)
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	waitForOTGARP(t, ate, top)

	beforeECMP := fetchDUTCounters(t, dut)
	ate.OTG().StartTraffic(t)
	t.Cleanup(func() {
		ate.OTG().StopTraffic(t)
	})
	waitForFlowTraffic(t, ate.OTG(), "flow_ecmp_v4", 30*time.Second)
	ate.OTG().StopTraffic(t)

	lag1Ports := []string{dut.Port(t, "port3").Name(), dut.Port(t, "port4").Name()}
	lag2Ports := []string{dut.Port(t, "port5").Name(), dut.Port(t, "port6").Name()}

	var afterECMP map[string]uint64
	gnmi.Watch(t, dut, func() any {
		afterECMP = fetchDUTCounters(t, dut)
		if portCountersIncreased(beforeECMP, afterECMP, lag1Ports) && portCountersIncreased(beforeECMP, afterECMP, lag2Ports) {
			return true
		}
		return nil
	}, time.Minute, 100*time.Millisecond)

	assertFlowLossBelow(t, ate, "flow_ecmp_v4", 60*time.Second, 5)
	if !portCountersIncreased(beforeECMP, afterECMP, lag1Ports) {
		t.Fatalf("expected ECMP traffic to use LAG1 ports %v", lag1Ports)
	}
	if !portCountersIncreased(beforeECMP, afterECMP, lag2Ports) {
		t.Fatalf("expected ECMP traffic to use LAG2 ports %v", lag2Ports)
	}

	applyStaticRoutes(t, dut, []string{"198.51.101.2"}, "203.0.115.0/24", "2001:db8:215::/64")

	top.Flows().Clear()
	for i := 0; i < 100; i++ {
		v4Dst := fmt.Sprintf("10.1.%d.1", i)
		v6Dst := fmt.Sprintf("2001:db8:10%02x::1", i)

		createFlow(t, top, fmt.Sprintf("flow_scale_v4_%d", i), atePort8.Name+".IPv4", []string{"ateLag2IPv4_101", "ateLag2IPv4_110"}, atePort8.IPv4, v4Dst, atePort8.IPv6, v6Dst, false)
		createFlow(t, top, fmt.Sprintf("flow_scale_v6_%d", i), atePort8.Name+".IPv6", []string{"ateLag2IPv6_101", "ateLag2IPv6_110"}, atePort8.IPv4, v4Dst, atePort8.IPv6, v6Dst, true)
	}
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	waitForOTGARP(t, ate, top)

	beforeScale := fetchDUTCounters(t, dut)
	ate.OTG().StartTraffic(t)
	waitForFlowTraffic(t, ate.OTG(), "flow_scale_v4_0", 60*time.Second)
	ate.OTG().StopTraffic(t)

	var afterScale map[string]uint64
	gnmi.Watch(t, dut, func() any {
		afterScale = fetchDUTCounters(t, dut)
		if portCountersIncreased(beforeScale, afterScale, lag2Ports) {
			return true
		}
		return nil
	}, time.Minute, 100*time.Millisecond)

	assertFlowLossBelow(t, ate, "flow_scale_v4_0", 60*time.Second, 10)
	if !portCountersIncreased(beforeScale, afterScale, lag2Ports) {
		t.Fatalf("expected scaled flows to forward traffic through LAG2 ports %v", lag2Ports)
	}

	// Step 3 (FIB Update - Add Next-Hop) - Add ATE Port 7
	beforeAdd := fetchDUTCounters(t, dut)
	bAdd := &gnmi.SetBatch{}
	for i := 0; i < 100; i++ {
		v4Prefix := fmt.Sprintf("10.1.%d.0/24", i)
		v6Prefix := fmt.Sprintf("2001:db8:10%02x::/64", i)
		vlanIndex := (i % 10) + 111
		nhV4 := fmt.Sprintf("198.51.%d.2", vlanIndex)
		nhV6 := fmt.Sprintf("2001:db8:%d::2", vlanIndex)

		cfgplugins.NewStaticRouteCfg(bAdd, &cfgplugins.StaticRouteCfg{
			NetworkInstance: deviations.DefaultNetworkInstance(dut),
			Prefix:          v4Prefix,
			NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
				"0": oc.UnionString(nhV4),
				"1": oc.UnionString("198.51.102.2"),
			},
		}, dut)
		cfgplugins.NewStaticRouteCfg(bAdd, &cfgplugins.StaticRouteCfg{
			NetworkInstance: deviations.DefaultNetworkInstance(dut),
			Prefix:          v6Prefix,
			NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
				"0": oc.UnionString(nhV6),
				"1": oc.UnionString("2001:db8:102::2"),
			},
		}, dut)
	}
	bAdd.Set(t, dut)
	// Step 4 (Verify Add)
	ate.OTG().StartTraffic(t)
	waitForFlowTraffic(t, ate.OTG(), "flow_scale_v4_0", 60*time.Second)
	ate.OTG().StopTraffic(t)

	var afterAdd map[string]uint64
	gnmi.Watch(t, dut, func() any {
		afterAdd = fetchDUTCounters(t, dut)
		if portCountersIncreased(beforeAdd, afterAdd, lag2Ports) {
			return true
		}
		return nil
	}, time.Minute, 100*time.Millisecond)

	assertFlowLossBelow(t, ate, "flow_scale_v4_0", 60*time.Second, 10)
	if !portCountersIncreased(beforeAdd, afterAdd, lag2Ports) {
		t.Fatalf("expected FIB add to preserve traffic through LAG2 ports %v", lag2Ports)
	}

	// Step 5 (FIB Update - Remove Next-Hop)
	beforeRemove := fetchDUTCounters(t, dut)
	bRem := &gnmi.SetBatch{}
	for i := 0; i < 100; i++ {
		v4Prefix := fmt.Sprintf("10.1.%d.0/24", i)
		v6Prefix := fmt.Sprintf("2001:db8:10%02x::/64", i)

		cfgplugins.NewStaticRouteCfg(bRem, &cfgplugins.StaticRouteCfg{
			NetworkInstance: deviations.DefaultNetworkInstance(dut),
			Prefix:          v4Prefix,
			NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
				"0": oc.UnionString("198.51.102.2"),
			},
		}, dut)
		cfgplugins.NewStaticRouteCfg(bRem, &cfgplugins.StaticRouteCfg{
			NetworkInstance: deviations.DefaultNetworkInstance(dut),
			Prefix:          v6Prefix,
			NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
				"0": oc.UnionString("2001:db8:102::2"),
			},
		}, dut)
	}
	bRem.Set(t, dut)
	// Step 6 (Verify Remove)
	ate.OTG().StartTraffic(t)
	waitForFlowTraffic(t, ate.OTG(), "flow_scale_v4_0", 60*time.Second)
	ate.OTG().StopTraffic(t)

	var afterRemove map[string]uint64
	gnmi.Watch(t, dut, func() any {
		afterRemove = fetchDUTCounters(t, dut)
		if portCountersIncreased(beforeRemove, afterRemove, lag2Ports) {
			return true
		}
		return nil
	}, time.Minute, 100*time.Millisecond)

	assertFlowLossBelow(t, ate, "flow_scale_v4_0", 60*time.Second, 10)
	if !portCountersIncreased(beforeRemove, afterRemove, lag2Ports) {
		t.Fatalf("expected FIB remove to preserve traffic through LAG2 ports %v", lag2Ports)
	}

	// Step 7 (Simulate Linecard OIR by Admin-disable)
	port7Name := dut.Port(t, "port7").Name()
	origEnabled := gnmi.Get(t, dut, gnmi.OC().Interface(port7Name).Enabled().State())
	t.Cleanup(func() {
		gnmi.Replace(t, dut, gnmi.OC().Interface(port7Name).Enabled().Config(), origEnabled)
	})
	gnmi.Replace(t, dut, gnmi.OC().Interface(port7Name).Enabled().Config(), false)
	gnmi.Await(t, dut, gnmi.OC().Interface(port7Name).Enabled().State(), time.Minute, false)

	// Step 8 (Verify Persistence)
	gnmi.Replace(t, dut, gnmi.OC().Interface(dut.Port(t, "port7").Name()).Enabled().Config(), true)
	gnmi.Await(t, dut, gnmi.OC().Interface(dut.Port(t, "port7").Name()).Enabled().State(), time.Minute, true)

	ate.OTG().StartTraffic(t)
	waitForFlowTraffic(t, ate.OTG(), "flow_scale_v4_0", 60*time.Second)
	assertFlowLossBelow(t, ate, "flow_scale_v4_0", 60*time.Second, 10)
	ate.OTG().StopTraffic(t)

	// Step 9 & 10 (Stop & Delete)
	ate.OTG().StopTraffic(t)

	ni := deviations.DefaultNetworkInstance(dut)
	sp := gnmi.OC().NetworkInstance(ni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
	for i := 0; i < 100; i++ {
		gnmi.Delete(t, dut, sp.Static(fmt.Sprintf("10.1.%d.0/24", i)).Config())
		gnmi.Delete(t, dut, sp.Static(fmt.Sprintf("2001:db8:10%02x::/64", i)).Config())
	}

	ate.OTG().StartTraffic(t)
	waitForFlowTraffic(t, ate.OTG(), "flow_scale_v4_0", 30*time.Second)
	assertFlowLossAbove(t, ate, "flow_scale_v4_0", 30*time.Second, 90)
	ate.OTG().StopTraffic(t)
}

func assertFlowLossBelow(t *testing.T, ate *ondatra.ATEDevice, flowName string, timeout time.Duration, maxLossPct float32) {
	t.Helper()
	otg := ate.OTG()
	waitForFlowTraffic(t, otg, flowName, timeout)

	var outPkts uint64
	if ok := gnmi.Watch(t, otg, gnmi.OTG().Flow(flowName).Counters().OutPkts().State(), timeout, func(val *ygnmi.Value[uint64]) bool {
		v, present := val.Val()
		if !present || v == 0 {
			return false
		}
		outPkts = v
		return true
	}).Await(t); !ok {
		t.Fatalf("timeout waiting for flow %s OutPkts counter to populate", flowName)
	}

	inPkts := uint64(gnmi.Get(t, otg, gnmi.OTG().Flow(flowName).Counters().InPkts().State()))
	lossPct := (float32(outPkts-inPkts) * 100) / float32(outPkts)
	if lossPct > maxLossPct {
		t.Fatalf("flow %s loss %.2f%% exceeds max allowed %.2f%%", flowName, lossPct, maxLossPct)
	}
}

func assertFlowLossAbove(t *testing.T, ate *ondatra.ATEDevice, flowName string, timeout time.Duration, minLossPct float32) {
	t.Helper()
	otg := ate.OTG()
	waitForFlowTraffic(t, otg, flowName, timeout)

	var outPkts uint64
	if ok := gnmi.Watch(t, otg, gnmi.OTG().Flow(flowName).Counters().OutPkts().State(), timeout, func(val *ygnmi.Value[uint64]) bool {
		v, present := val.Val()
		if !present || v == 0 {
			return false
		}
		outPkts = v
		return true
	}).Await(t); !ok {
		t.Fatalf("timeout waiting for flow %s OutPkts counter to populate", flowName)
	}

	inPkts := uint64(gnmi.Get(t, otg, gnmi.OTG().Flow(flowName).Counters().InPkts().State()))
	lossPct := (float32(outPkts-inPkts) * 100) / float32(outPkts)
	if lossPct < minLossPct {
		t.Fatalf("flow %s loss %.2f%% is below expected minimum %.2f%%", flowName, lossPct, minLossPct)
	}
}

func portCountersIncreased(before, after map[string]uint64, ports []string) bool {
	for _, port := range ports {
		if after[port] > before[port] {
			return true
		}
	}
	return false
}

func waitForOTGARP(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config) {
	for _, d := range top.Devices().Items() {
		eth := d.Ethernets().Items()[0]
		if _, ok := gnmi.Watch(t, ate.OTG(), gnmi.OTG().Interface(eth.Name()+".Eth").Ipv4NeighborAny().LinkLayerAddress().State(), 2*time.Minute, func(val *ygnmi.Value[string]) bool {
			return val.IsPresent()
		}).Await(t); !ok {
			t.Fatalf("Did not receive OTG IPv4 neighbor entry for interface %s", eth.Name())
		}
		if _, ok := gnmi.Watch(t, ate.OTG(), gnmi.OTG().Interface(eth.Name()+".Eth").Ipv6NeighborAny().LinkLayerAddress().State(), 2*time.Minute, func(val *ygnmi.Value[string]) bool {
			return val.IsPresent()
		}).Await(t); !ok {
			t.Fatalf("Did not receive OTG IPv6 neighbor entry for interface %s", eth.Name())
		}
	}
}

func waitForFlowTraffic(t *testing.T, otgDev *otg.OTG, flowName string, timeout time.Duration) {
	_, ok := gnmi.Watch(t, otgDev, gnmi.OTG().Flow(flowName).State(), timeout, func(val *ygnmi.Value[*otgtelemetry.Flow]) bool {
		f, present := val.Val()
		return present && f.GetCounters() != nil && f.GetCounters().GetOutPkts() > 0
	}).Await(t)
	if !ok {
		t.Fatalf("timeout waiting for flow %s to transmit packets", flowName)
	}
}
