package static_route_resiliency_test

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"unicode"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
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
	packetPerSecond = 100
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
		{Name: "port8", MAC: "02:00:01:00:00:08", IPv4: "10.0.0.2", IPv4Len: ipv4PrefixLen, IPv6: "2001:db8:a::2", IPv6Len: ipv6PrefixLen},
	}

	dutPorts = []attrs.Attributes{
		{Name: "port1"},
		{Name: "port2"},
		{Name: "port3"},
		{Name: "port4"},
		{Name: "port5"},
		{Name: "port6"},
		{Name: "port7", IPv4: "198.51.102.1", IPv4Len: ipv4PrefixLen, IPv6: "2001:db8:102::1", IPv6Len: ipv6PrefixLen},
		{Name: "port8", IPv4: "10.0.0.1", IPv4Len: ipv4PrefixLen, IPv6: "2001:db8:a::1", IPv6Len: ipv6PrefixLen},
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
)

func configureDUT(t *testing.T, dut *ondatra.DUTDevice) (string, string) {
	t.Helper()

	lag1Name := netutil.NextAggregateInterface(t, dut)
	numRE := strings.TrimLeftFunc(lag1Name, func(r rune) bool { return !unicode.IsDigit(r) })
	start, err := strconv.Atoi(numRE)
	if err != nil {
		t.Fatalf("Failed to parse LAG number from %s: %v", lag1Name, err)
	}
	prefix := strings.TrimRight(lag1Name, "0123456789")
	lag2Name := fmt.Sprintf("%s%d", prefix, start+1)
	t.Logf("Using LAG names: %s, %s", lag1Name, lag2Name)

	cfgplugins.ConfigureVlan(t, dut, cfgplugins.VlanParams{
		VlanID: vlanID,
	})

	portBatch := &gnmi.SetBatch{}
	for i, a := range dutPorts {
		iObj := &oc.Interface{Name: ygot.String(dut.Port(t, a.Name).Name())}
		iObj.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
		if deviations.InterfaceEnabled(dut) {
			iObj.Enabled = ygot.Bool(true)
		}

		if i < 2 { // Ports 1, 2: Access with VLAN 10
			cfgplugins.ConfigureAccessVlan(cfgplugins.AccessVlanParams{
				Intf:   iObj,
				VlanID: vlanID,
			})
		} else if i == 6 || i == 7 { // Port 7, 8: Standalone Port
			s := iObj.GetOrCreateSubinterface(0)
			cfgplugins.ConfigureSubinterfaceIPs(s, dut, a.IPv4, a.IPv4Len, a.IPv6, a.IPv6Len)
		}
		// Lag member assignment will be handled by NewAggregateInterface
		gnmi.BatchUpdate(portBatch, gnmi.OC().Interface(dut.Port(t, a.Name).Name()).Config(), iObj)
	}
	portBatch.Set(t, dut)

	cfgplugins.ConfigureSVI(t, dut, sviParams)

	lagBatch := &gnmi.SetBatch{}
	// Setup LAG 1
	lag1 := &cfgplugins.DUTAggData{
		Attributes:      lag1DutAttrs,
		OndatraPortsIdx: []int{2, 3}, // Port 3 and 4
		LagName:         lag1Name,
		AggType:         oc.IfAggregate_AggregationType_STATIC,
	}
	cfgplugins.NewAggregateInterface(t, dut, lagBatch, lag1)

	// Setup LAG 2
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
	// For no IPv4 config, attributes is omitted
	cfgplugins.NewAggregateInterface(t, dut, lagBatch, lag2)

	lagBatch.Set(t, dut)
	return lag1Name, lag2Name
}

func configureOTG(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config, lag1Name, lag2Name string) {
	t.Helper()

	// Configure Port 1, 2, 7, 8 normally
	for _, i := range []int{0, 1, 6, 7} {
		a := atePorts[i]
		p := ate.Port(t, a.Name)
		a.AddToOTG(top, p, &dutPorts[i])
	}

	// Configure LAG 1
	p3 := ate.Port(t, atePorts[2].Name)
	p4 := ate.Port(t, atePorts[3].Name)
	top.Ports().Add().SetName(p3.ID())
	top.Ports().Add().SetName(p4.ID())

	lag1 := top.Lags().Add().SetName("ATE_LAG1")
	// Using static LAG
	lag1.Protocol().Static().SetLagId(1)
	lag1.Ports().Add().SetPortName(p3.ID()).Ethernet().SetMac(atePorts[2].MAC).SetName("LAG1_Rx1")
	lag1.Ports().Add().SetPortName(p4.ID()).Ethernet().SetMac(atePorts[3].MAC).SetName("LAG1_Rx2")

	lag1Dev := top.Devices().Add().SetName("LAG1_Dev")
	lag1Eth := lag1Dev.Ethernets().Add().SetName("LAG1_Eth").SetMac(lag1AteAttrs.MAC)
	lag1Eth.Connection().SetLagName("ATE_LAG1")
	lag1Eth.Ipv4Addresses().Add().SetName("LAG1_IPv4").SetAddress(lag1AteAttrs.IPv4).SetGateway(lag1DutAttrs.IPv4).SetPrefix(uint32(lag1AteAttrs.IPv4Len))
	lag1Eth.Ipv6Addresses().Add().SetName("LAG1_IPv6").SetAddress(lag1AteAttrs.IPv6).SetGateway(lag1DutAttrs.IPv6).SetPrefix(uint32(lag1AteAttrs.IPv6Len))

	// Configure LAG 2
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

func createTraffic(top gosnappi.Config, flowName, dstV4Net, dstV6Net string) {
	top.Flows().Clear()

	srcV4Addr := atePorts[7].IPv4
	srcV6Addr := atePorts[7].IPv6

	v4F := top.Flows().Add().SetName(flowName + "_v4")
	v4F.Metrics().SetEnable(true)
	v4F.TxRx().Device().SetTxNames([]string{"port8.IPv4"})
	eth := v4F.Packet().Add().Ethernet()
	eth.Src().SetValue(atePorts[7].MAC)
	v4 := v4F.Packet().Add().Ipv4()
	v4.Src().SetValue(srcV4Addr)

	if strings.Contains(dstV4Net, "/") {
		parts := strings.Split(dstV4Net, "/")
		v4.Dst().Increment().SetStart(parts[0]).SetStep("0.0.0.1").SetCount(254)
	} else {
		v4.Dst().SetValue(dstV4Net)
	}

	v6F := top.Flows().Add().SetName(flowName + "_v6")
	v6F.Metrics().SetEnable(true)
	v6F.TxRx().Device().SetTxNames([]string{"port8.IPv6"})
	eth = v6F.Packet().Add().Ethernet()
	eth.Src().SetValue(atePorts[7].MAC)
	v6 := v6F.Packet().Add().Ipv6()
	v6.Src().SetValue(srcV6Addr)

	if strings.Contains(dstV6Net, "/") {
		parts := strings.Split(dstV6Net, "/")
		v6.Dst().Increment().SetStart(parts[0]).SetStep("::1").SetCount(254)
	} else {
		v6.Dst().SetValue(dstV6Net)
	}
}

func configureRoute(t *testing.T, dut *ondatra.DUTDevice, v4Prefix, v6Prefix string, v4Nh, v6Nh []string) {
	b := &gnmi.SetBatch{}
	// IPv4 route
	v4Map := make(map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union)
	for i, nh := range v4Nh {
		v4Map[fmt.Sprintf("%d", i)] = oc.UnionString(nh)
	}
	sV4 := &cfgplugins.StaticRouteCfg{
		NetworkInstance: deviations.DefaultNetworkInstance(dut),
		Prefix:          v4Prefix,
		NextHops:        v4Map,
	}
	if _, err := cfgplugins.NewStaticRouteCfg(b, sV4, dut); err != nil {
		t.Fatalf("Failed to configure IPv4 static route: %v", err)
	}

	// IPv6 route
	v6Map := make(map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union)
	for i, nh := range v6Nh {
		v6Map[fmt.Sprintf("%d", i)] = oc.UnionString(nh)
	}
	sV6 := &cfgplugins.StaticRouteCfg{
		NetworkInstance: deviations.DefaultNetworkInstance(dut),
		Prefix:          v6Prefix,
		NextHops:        v6Map,
	}
	if _, err := cfgplugins.NewStaticRouteCfg(b, sV6, dut); err != nil {
		t.Fatalf("Failed to configure IPv6 static route: %v", err)
	}
	b.Set(t, dut)

	sp := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
	gnmi.Await(t, dut, sp.Static(v4Prefix).Prefix().State(), 30*time.Second, v4Prefix)
	gnmi.Await(t, dut, sp.Static(v6Prefix).Prefix().State(), 30*time.Second, v6Prefix)
}

// TestStaticRouteResiliency is the main test entry point that orchestrates the execution of all RT-1.73 subtests.
func TestStaticRouteResiliency(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	top := gosnappi.NewConfig()

	lag1Name, lag2Name := configureDUT(t, dut)
	configureOTG(t, ate, top, lag1Name, lag2Name)

	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv6")

	t.Run("RT-1.73.1: Validate Static Route with VLAN Interface (SVI)", func(t *testing.T) {
		configureRoute(t, dut, "203.0.113.0/24", "2001:db8:213::/64", []string{"198.51.100.2"}, []string{"2001:db8:100::2"})
		createTraffic(top, "traffic_svi", "203.0.113.1", "2001:db8:213::1")
		ate.OTG().PushConfig(t, top)
		ate.OTG().StartProtocols(t)
		ate.OTG().StartTraffic(t)
		for _, flow := range top.Flows().Items() {
			otgutils.ExpectedTrafficLoss(t, ate.OTG(), flow.Name(), 0, 1)
		}
		ate.OTG().StopTraffic(t)
		otgutils.VerifyPortTraffic(t, ate.OTG(), []string{"port1", "port2"}, 100, nil, 0, 5*time.Second)
	})

	t.Run("RT-1.73.2: Validate Static Route over LAG Interface", func(t *testing.T) {
		configureRoute(t, dut, "203.0.114.0/24", "2001:db8:214::/64", []string{"198.51.101.2"}, []string{"2001:db8:101::2"})
		createTraffic(top, "traffic_lag1", "203.0.114.1", "2001:db8:214::1")
		ate.OTG().PushConfig(t, top)
		ate.OTG().StartProtocols(t)
		ate.OTG().StartTraffic(t)
		for _, flow := range top.Flows().Items() {
			otgutils.ExpectedTrafficLoss(t, ate.OTG(), flow.Name(), 0, 1)
		}
		otgutils.VerifyPortTraffic(t, ate.OTG(), []string{"port3", "port4"}, 100, nil, 0, 5*time.Second)
		// Leave traffic flowing for next test
	})

	t.Run("RT-1.73.3: Control Plane Resilience on LAG Failure", func(t *testing.T) {
		p3 := ate.Port(t, atePorts[2].Name)
		p4 := ate.Port(t, atePorts[3].Name)

		// Simulate LAG failure by bringing down ATE ports
		portStateAction := gosnappi.NewControlState()
		portStateAction.Port().Link().SetPortNames([]string{p3.ID(), p4.ID()}).SetState(gosnappi.StatePortLinkState.DOWN)
		ate.OTG().SetControlState(t, portStateAction)

		gnmi.Await(t, dut, gnmi.OC().Interface(lag1Name).OperStatus().State(), trafficWaitTime, oc.Interface_OperStatus_DOWN)

		// Unrelated gNMI Set operation should succeed
		gnmi.Update(t, dut, gnmi.OC().Interface(dut.Port(t, "port7").Name()).Description().Config(), "test_description")

		// Verify traffic drops
		ate.OTG().StartTraffic(t)
		for _, flow := range top.Flows().Items() {
			otgutils.ExpectedTrafficLoss(t, ate.OTG(), flow.Name(), 99.9, 100)
		}
		ate.OTG().StopTraffic(t)
		otgutils.VerifyPortTraffic(t, ate.OTG(), nil, 0, []string{"port3", "port4"}, 10, 5*time.Second)

		// Re-enable
		portStateAction.Port().Link().SetPortNames([]string{p3.ID(), p4.ID()}).SetState(gosnappi.StatePortLinkState.UP)
		ate.OTG().SetControlState(t, portStateAction)

		gnmi.Await(t, dut, gnmi.OC().Interface(lag1Name).OperStatus().State(), trafficWaitTime, oc.Interface_OperStatus_UP)

		// Verify traffic resumes
		ate.OTG().StartTraffic(t)
		otgutils.VerifyPortTraffic(t, ate.OTG(), []string{"port3", "port4"}, 100, nil, 0, 5*time.Second)
		ate.OTG().StopTraffic(t)
	})

	t.Run("RT-1.73.4: Validate ECMP and FIB Reprogramming Across Multiple LAGs", func(t *testing.T) {
		configureRoute(t, dut, "203.0.115.0/24", "2001:db8:215::/64", []string{"198.51.101.2", "198.51.111.2"}, []string{"2001:db8:101::2", "2001:db8:111::2"})
		createTraffic(top, "traffic_ecmp", "203.0.115.1", "2001:db8:215::1")
		ate.OTG().PushConfig(t, top)
		ate.OTG().StartProtocols(t)
		ate.OTG().StartTraffic(t)
		otgutils.VerifyPortTraffic(t, ate.OTG(), []string{"port3", "port4", "port5", "port6"}, 100, nil, 0, 5*time.Second)

		// Update route to remove lag2 next-hop
		configureRoute(t, dut, "203.0.115.0/24", "2001:db8:215::/64", []string{"198.51.101.2"}, []string{"2001:db8:101::2"})

		// Verify traffic loss is still low (Seamless)
		for _, flow := range top.Flows().Items() {
			otgutils.ExpectedTrafficLoss(t, ate.OTG(), flow.Name(), 0, 1)
		}

		// Verify forwarding now only on port3, port4
		otgutils.VerifyPortTraffic(t, ate.OTG(), []string{"port3", "port4"}, 100, []string{"port5", "port6"}, 10, 5*time.Second)

		ate.OTG().StopTraffic(t)
	})

	t.Run("RT-1.73.5: Scale, Dynamic FIB Re-programming, and Route Persistence", func(t *testing.T) {
		b := &gnmi.SetBatch{}
		sp := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))

		// Map routes across subinterfaces
		for i := 0; i < 100; i++ {
			nhIndex := (i % 10) + 1
			v4Prefix := fmt.Sprintf("10.1.%d.0/24", i)
			v6Prefix := fmt.Sprintf("2001:db8:10%02x::/64", i)

			sV4 := &cfgplugins.StaticRouteCfg{
				NetworkInstance: deviations.DefaultNetworkInstance(dut),
				Prefix:          v4Prefix,
				NextHops:        map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{"0": oc.UnionString(fmt.Sprintf("198.51.%d.2", 110+nhIndex))},
			}
			cfgplugins.NewStaticRouteCfg(b, sV4, dut)

			sV6 := &cfgplugins.StaticRouteCfg{
				NetworkInstance: deviations.DefaultNetworkInstance(dut),
				Prefix:          v6Prefix,
				NextHops:        map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{"0": oc.UnionString(fmt.Sprintf("2001:db8:%d::2", 110+nhIndex))},
			}
			cfgplugins.NewStaticRouteCfg(b, sV6, dut)
		}
		b.Set(t, dut)
		gnmi.Await(t, dut, sp.Static("10.1.99.0/24").Prefix().State(), 30*time.Second, "10.1.99.0/24")

		top.Flows().Clear()
		for i := 0; i < 100; i++ {
			createTraffic(top, fmt.Sprintf("scale_%d", i), fmt.Sprintf("10.1.%d.1", i), fmt.Sprintf("2001:db8:10%02x::1", i))
		}
		ate.OTG().PushConfig(t, top)
		ate.OTG().StartProtocols(t)
		ate.OTG().StartTraffic(t)
		otgutils.VerifyPortTraffic(t, ate.OTG(), []string{"port5", "port6"}, 100, nil, 0, 5*time.Second)

		// Update to add port7
		for i := 0; i < 100; i++ {
			nhIndex := (i % 10) + 1
			v4Prefix := fmt.Sprintf("10.1.%d.0/24", i)
			v6Prefix := fmt.Sprintf("2001:db8:10%02x::/64", i)

			sV4 := &cfgplugins.StaticRouteCfg{
				NetworkInstance: deviations.DefaultNetworkInstance(dut),
				Prefix:          v4Prefix,
				NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
					"0": oc.UnionString(fmt.Sprintf("198.51.%d.2", 110+nhIndex)),
					"1": oc.UnionString("198.51.102.2"),
				},
			}
			cfgplugins.NewStaticRouteCfg(b, sV4, dut)

			sV6 := &cfgplugins.StaticRouteCfg{
				NetworkInstance: deviations.DefaultNetworkInstance(dut),
				Prefix:          v6Prefix,
				NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
					"0": oc.UnionString(fmt.Sprintf("2001:db8:%d::2", 110+nhIndex)),
					"1": oc.UnionString("2001:db8:102::2"),
				},
			}
			cfgplugins.NewStaticRouteCfg(b, sV6, dut)
		}
		b.Set(t, dut)

		// Verify traffic loss is still low (Seamless)
		for _, flow := range top.Flows().Items() {
			otgutils.ExpectedTrafficLoss(t, ate.OTG(), flow.Name(), 0, 1)
		}

		// Verify forwarding on all next-hops
		otgutils.VerifyPortTraffic(t, ate.OTG(), []string{"port5", "port6", "port7"}, 100, nil, 0, 5*time.Second)

		// Remove LAG2
		for i := 0; i < 100; i++ {
			v4Prefix := fmt.Sprintf("10.1.%d.0/24", i)
			v6Prefix := fmt.Sprintf("2001:db8:10%02x::/64", i)

			sV4 := &cfgplugins.StaticRouteCfg{
				NetworkInstance: deviations.DefaultNetworkInstance(dut),
				Prefix:          v4Prefix,
				NextHops:        map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{"0": oc.UnionString("198.51.102.2")},
			}
			cfgplugins.NewStaticRouteCfg(b, sV4, dut)

			sV6 := &cfgplugins.StaticRouteCfg{
				NetworkInstance: deviations.DefaultNetworkInstance(dut),
				Prefix:          v6Prefix,
				NextHops:        map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{"0": oc.UnionString("2001:db8:102::2")},
			}
			cfgplugins.NewStaticRouteCfg(b, sV6, dut)
		}
		b.Set(t, dut)
		// Verify traffic loss is still low (Seamless)
		for _, flow := range top.Flows().Items() {
			otgutils.ExpectedTrafficLoss(t, ate.OTG(), flow.Name(), 0, 1)
		}

		// Verify forwarding strictly on port7
		otgutils.VerifyPortTraffic(t, ate.OTG(), []string{"port7"}, 100, []string{"port5", "port6"}, 10, 5*time.Second)

		// Stop traffic to prepare for drop verification
		ate.OTG().StopTraffic(t)

		// Simulate Linecard OIR / Disable
		dutP7 := dut.Port(t, "port7").Name()
		gnmi.Update(t, dut, gnmi.OC().Interface(dutP7).Enabled().Config(), false)
		gnmi.Await(t, dut, gnmi.OC().Interface(dutP7).OperStatus().State(), trafficWaitTime, oc.Interface_OperStatus_DOWN)

		// Verify traffic drops
		ate.OTG().StartTraffic(t)
		for _, flow := range top.Flows().Items() {
			otgutils.ExpectedTrafficLoss(t, ate.OTG(), flow.Name(), 99.9, 100)
		}
		ate.OTG().StopTraffic(t)
		otgutils.VerifyPortTraffic(t, ate.OTG(), nil, 0, []string{"port7"}, 10, 5*time.Second)

		gnmi.Update(t, dut, gnmi.OC().Interface(dutP7).Enabled().Config(), true)
		gnmi.Await(t, dut, gnmi.OC().Interface(dutP7).OperStatus().State(), trafficWaitTime, oc.Interface_OperStatus_UP)
		ate.OTG().StartTraffic(t)
		otgutils.VerifyPortTraffic(t, ate.OTG(), []string{"port7"}, 100, nil, 0, 5*time.Second)
		ate.OTG().StopTraffic(t)

		// Stop and Delete
		for i := 0; i < 100; i++ {
			v4Prefix := fmt.Sprintf("10.1.%d.0/24", i)
			v6Prefix := fmt.Sprintf("2001:db8:10%02x::/64", i)
			gnmi.BatchDelete(b, sp.Static(v4Prefix).Config())
			gnmi.BatchDelete(b, sp.Static(v6Prefix).Config())
		}
		b.Set(t, dut)

		// Wait for deletion propagating
		gnmi.Watch(t, dut, sp.Static("10.1.99.0/24").Prefix().State(), 30*time.Second, func(val *ygnmi.Value[string]) bool {
			return !val.IsPresent()
		}).Await(t)
		// Verify traffic drops after route deletion
		ate.OTG().StartTraffic(t)
		for _, flow := range top.Flows().Items() {
			otgutils.ExpectedTrafficLoss(t, ate.OTG(), flow.Name(), 99.9, 100)
		}
		ate.OTG().StopTraffic(t)
	})
}
