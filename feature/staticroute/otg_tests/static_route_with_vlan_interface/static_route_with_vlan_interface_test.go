package staticroutewithvlaninterface_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	ipv4PrefixLen            = 28
	ipv6PrefixLen            = 64
	vlanID                   = uint16(10)
	vlanIntfName             = "Vlan10"
	frameSize                = 512
	packetPerSecond          = 100
	flowTypeForwarding       = "forwarding"
	flowTypeStatic           = "static"
	expectedPktsPerPort      = 2 * packetPerSecond
	trafficWaitTime          = 60 * time.Second
	expectedFlowsPerPort     = 3
	expectedTotalPktsPerPort = expectedPktsPerPort * expectedFlowsPerPort
	numOfVlanAccessPorts     = 3
)

var (
	atePorts = []attrs.Attributes{
		{
			Name:    "atePort1",
			MAC:     "02:00:01:01:01:01",
			IPv4:    "198.51.100.161",
			IPv4Len: ipv4PrefixLen,
			IPv6:    "2001:db8:2:1::161",
			IPv6Len: ipv6PrefixLen,
		},
		{
			Name:    "atePort2",
			MAC:     "02:00:01:01:01:02",
			IPv4:    "198.51.100.193",
			IPv4Len: ipv4PrefixLen,
			IPv6:    "2001:db8:2:2::193",
			IPv6Len: ipv6PrefixLen,
		},
		{
			Name:    "atePort3",
			MAC:     "02:00:01:01:01:03",
			IPv4:    "198.51.100.225",
			IPv4Len: ipv4PrefixLen,
			IPv6:    "2001:db8:2:3::225",
			IPv6Len: ipv6PrefixLen,
		},
		{
			Name:    "atePort4",
			MAC:     "02:00:01:01:01:04",
			IPv4:    "198.51.100.1",
			IPv4Len: 31,
			IPv6:    "2001:db8:1::1",
			IPv6Len: 127,
		},
	}

	dutPorts = []attrs.Attributes{
		{
			Name: "port1",
			Desc: "dutPort1",
		},
		{
			Name: "port2",
			Desc: "dutPort2",
		},
		{
			Name: "port3",
			Desc: "dutPort3",
		},
		{
			Name:    "port4",
			Desc:    "dutPort4",
			IPv4:    "198.51.100.0",
			IPv4Len: 31,
			IPv6:    "2001:db8:1::",
			IPv6Len: 127,
		},
	}

	staticRoutes = []struct {
		prefix  string
		nextHop string
		intf    string
	}{
		// Resolution routes
		{"198.51.100.161/32", "", vlanIntfName},
		{"198.51.100.193/32", "", vlanIntfName},
		{"198.51.100.225/32", "", vlanIntfName},
		{"2001:db8:2:1::161/128", "", vlanIntfName},
		{"2001:db8:2:2::193/128", "", vlanIntfName},
		{"2001:db8:2:3::225/128", "", vlanIntfName},

		// Recursive routes
		{"198.51.100.160/28", "198.51.100.161", ""},
		{"198.51.100.192/28", "198.51.100.193", ""},
		{"198.51.100.224/28", "198.51.100.225", ""},
		{"2001:db8:2:1::/64", "2001:db8:2:1::161", ""},
		{"2001:db8:2:2::/64", "2001:db8:2:2::193", ""},
		{"2001:db8:2:3::/64", "2001:db8:2:3::225", ""},
	}

	sviParams = cfgplugins.SVIParams{
		IntfName: vlanIntfName,
		IPv4:     "198.51.100.129",
		IPv4Len:  25,
		IPv6:     "2001:db8:2::129",
		IPv6Len:  48,
	}

	peerPort     *attrs.Attributes
	flowPatterns []FlowPattern
)

type testCase struct {
	name     string
	flowType string
}

type dutCounters struct {
	inPkts  uint64
	outPkts uint64
}

type FlowPattern struct {
	srcIndex int
	dstIndex int
	nameFmt  string
}

// TestMain initializes the FeatureProfiles test framework and runs all tests in this package.
func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// TestStaticRouteWithVlanInterface validates IPv4/IPv6 forwarding using a VLAN SVI and recursive static routes.
func TestStaticRouteWithVlanInterface(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	configureDUT(t, dut)

	ate := ondatra.ATE(t, "ate")
	top := gosnappi.NewConfig()
	configureOTG(t, ate, top)

	tests := []testCase{
		{
			name:     "RT-1.67.1: Validate Layer 3 Forwarding over VLAN interface",
			flowType: flowTypeForwarding,
		},
		{
			name:     "RT-1.67.2: Validate IP forwarding over static route",
			flowType: flowTypeStatic,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mustTestStaticRoute(t, tc, dut, top, ate)
		})
	}

}

// mustTestStaticRoute validates static routes, runs OTG traffic, and verifies DUT counters and flow behavior end-to-end.
func mustTestStaticRoute(t *testing.T, tc testCase, dut *ondatra.DUTDevice, top gosnappi.Config, ate *ondatra.ATEDevice) {
	t.Helper()

	// Validate static routes if this is the IP forwarding over static route test
	if tc.flowType == flowTypeStatic {
		t.Log("Validating all static routes on DUT")
		if err := validateAllStaticRoutes(t, dut); err != nil {
			t.Fatalf("Static route validation failed: %v", err)
		}
	}

	// Clear existing flows
	top.Flows().Clear()

	// Create traffic flows
	createTrafficFlows(t, top, tc.flowType)

	// Push config and start protocols
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)

	otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv6")

	// Capture DUT counters BEFORE traffic
	t.Log("Reading DUT counters before traffic")
	beforeCounters := fetchDUTCounters(t, dut)

	// Start traffic
	t.Log("Starting traffic")
	ate.OTG().StartTraffic(t)

	// Wait for at least 100 packets to be transmitted on atleast one flow to ensure traffic is flowing before we check counters
	t.Log("Waiting for traffic to be transmitted")
	flows := top.Flows().Items()
	flowName := flows[len(flows)-1].Name()
	gnmi.Watch(t, ate.OTG(), gnmi.OTG().Flow(flowName).Counters().OutPkts().State(), trafficWaitTime, func(val *ygnmi.Value[uint64]) bool {
		v, present := val.Val()
		if !present {
			return false
		}
		return v >= packetPerSecond
	}).Await(t)

	// Stop traffic
	t.Log("Stopping traffic")
	ate.OTG().StopTraffic(t)

	// Capture DUT counters AFTER traffic
	t.Log("Reading DUT counters after traffic")
	afterCounters := fetchDUTCounters(t, dut)

	// Validate DUT counters
	if err := validateDUTCounters(t, beforeCounters, afterCounters, tc.flowType); err != nil {
		t.Fatalf("DUT counter validation failed: %v", err)
	}

	// Log OTG metrics
	otgutils.LogFlowMetrics(t, ate.OTG(), top)

	// Verify OTG flows
	for _, flow := range top.Flows().Items() {
		if err := verifyTrafficFlow(t, ate, flow.Name()); err != nil {
			t.Fatalf("Traffic flow validation failed: %v", err)
		}
	}
}

// configureDUT configures the DUT with VLAN, access ports, SVI interface, and recursive static routes.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()

	// VLAN Global Configuration
	t.Log("Configuring Global VLAN")
	cfgplugins.ConfigureVlan(t, dut, cfgplugins.VlanParams{
		VlanID: vlanID,
	})

	// Physical Ports (Access for Ports 1-3, Routed for Port 4) Configuration
	t.Log("Configuring switched vlan ports and routed port")
	portBatch := &gnmi.SetBatch{}
	for i, a := range dutPorts {
		iObj := &oc.Interface{Name: ygot.String(dut.Port(t, a.Name).Name()), Description: ygot.String(a.Desc)}
		iObj.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
		if deviations.InterfaceEnabled(dut) {
			iObj.Enabled = ygot.Bool(true)
		}

		if i < numOfVlanAccessPorts { // Ports 1, 2, 3: Access with VLAN 10
			cfgplugins.ConfigureAccessVlan(dut, iObj, vlanID)
		} else { // Port 4: Routed
			s := iObj.GetOrCreateSubinterface(0)
			cfgplugins.ConfigureSubinterfaceIPs(s, dut, a.IPv4, a.IPv4Len, a.IPv6, a.IPv6Len)
		}
		gnmi.BatchUpdate(portBatch, gnmi.OC().Interface(dut.Port(t, a.Name).Name()).Config(), iObj)
	}
	portBatch.Set(t, dut)

	// SVI Configuration
	t.Log("Configuring SVI")
	cfgplugins.ConfigureSVI(t, dut, sviParams)

	// Static Routes Configuration
	t.Log("Configuring Recursive Static Routes")
	mustConfigureRecursiveStaticRoutes(t, dut)
}

// mustConfigureRecursiveStaticRoutes installs recursive and resolution static routes required for next-hop resolution.
func mustConfigureRecursiveStaticRoutes(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()

	staticRouteBatch := &gnmi.SetBatch{}

	dutOcRoot := &oc.Root{}
	ni := dutOcRoot.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	proto := ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
	proto.SetEnabled(true)

	for _, r := range staticRoutes {
		routeCfg := &cfgplugins.StaticRouteCfg{
			NetworkInstance: deviations.DefaultNetworkInstance(dut),
			Prefix:          r.prefix,
		}

		if r.nextHop != "" {
			// Recursive IP path
			routeCfg.NextHops = map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
				"0": oc.UnionString(r.nextHop),
			}
		} else if r.intf != "" {
			// Interface path
			routeCfg.NextHopIntf = r.intf
		}

		if _, err := cfgplugins.NewStaticRouteCfg(staticRouteBatch, routeCfg, dut); err != nil {
			t.Fatalf("Failed to configure route %s: %v", r.prefix, err)
		}
	}

	staticRouteBatch.Set(t, dut)
	t.Logf("Successfully pushed %d static routes", len(staticRoutes))
}

// configureOTG builds the OTG topology and assigns IPv4/IPv6 addressing to ATE ports
func configureOTG(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config) {
	t.Helper()

	peerPort = &attrs.Attributes{
		IPv4: sviParams.IPv4,
		IPv6: sviParams.IPv6,
	}

	for i, a := range atePorts {
		p := ate.Port(t, fmt.Sprintf("port%d", i+1))
		t.Logf("Configuring OTG Port %s with MAC %s, IPv4 %s/%d and IPv6 %s/%d", p.Name(), a.MAC, a.IPv4, a.IPv4Len, a.IPv6, a.IPv6Len)
		if i == numOfVlanAccessPorts {
			peerPort = &dutPorts[3]
		}
		a.AddToOTG(top, p, peerPort)
	}
}

// createTrafficFlows generates IPv4 and IPv6 traffic flows based on the test scenario.
func createTrafficFlows(t *testing.T, top gosnappi.Config, flowType string) {
	t.Helper()

	switch flowType {

	case flowTypeForwarding:
		// RT-1.67.1: ATE Port 1,2,3 -> Port 4
		flowPatterns = []FlowPattern{
			{srcIndex: 0, dstIndex: 3, nameFmt: "%s_to_P4_%s"},
			{srcIndex: 1, dstIndex: 3, nameFmt: "%s_to_P4_%s"},
			{srcIndex: 2, dstIndex: 3, nameFmt: "%s_to_P4_%s"},
		}

	case flowTypeStatic:
		// RT-1.67.2: Port 4 -> Ports 1,2,3
		flowPatterns = []FlowPattern{
			{srcIndex: 3, dstIndex: 0, nameFmt: "P4_to_%s_%s"},
			{srcIndex: 3, dstIndex: 1, nameFmt: "P4_to_%s_%s"},
			{srcIndex: 3, dstIndex: 2, nameFmt: "P4_to_%s_%s"},
		}
	}

	ipVersions := []string{"v4", "v6"}

	for _, p := range flowPatterns {
		src := atePorts[p.srcIndex]
		dst := atePorts[p.dstIndex]

		for _, ipVer := range ipVersions {

			var flowName string
			if flowType == flowTypeForwarding {
				flowName = fmt.Sprintf(p.nameFmt, src.Name, ipVer)
			} else {
				flowName = fmt.Sprintf(p.nameFmt, dst.Name, ipVer)
			}

			flow := top.Flows().Add().SetName(flowName)
			flow.Metrics().SetEnable(true)

			proto := "IPv4"
			if ipVer == "v6" {
				proto = "IPv6"
			}

			flow.TxRx().Device().SetTxNames([]string{src.Name + "." + proto}).SetRxNames([]string{dst.Name + "." + proto})

			// Ethernet
			eth := flow.Packet().Add().Ethernet()
			eth.Src().SetValue(src.MAC)
			eth.Dst().Auto()

			// IP Header
			switch ipVer {
			case "v4":
				ipv4 := flow.Packet().Add().Ipv4()
				ipv4.Src().SetValue(src.IPv4)
				ipv4.Dst().SetValue(dst.IPv4)
			case "v6":
				ipv6 := flow.Packet().Add().Ipv6()
				ipv6.Src().SetValue(src.IPv6)
				ipv6.Dst().SetValue(dst.IPv6)
			}

			// Common settings
			flow.Size().SetFixed(uint32(frameSize))
			flow.Rate().SetPps(packetPerSecond)
			flow.Duration().FixedPackets().SetPackets(packetPerSecond)
		}
	}
}

// validateAllStaticRoutes verifies that all recursive and resolution static routes are installed on the DUT.
func validateAllStaticRoutes(t *testing.T, dut *ondatra.DUTDevice) error {
	t.Helper()

	sp := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))

	for _, r := range staticRoutes {

		t.Logf("Validating route %s", r.prefix)

		gnmi.Await(t, dut, sp.Static(r.prefix).Prefix().State(), 120*time.Second, r.prefix)

		switch {

		// Recursive route validation
		case r.nextHop != "":
			gotNH := gnmi.Get(t, dut, sp.Static(r.prefix).NextHop("0").NextHop().State())

			if fmt.Sprint(gotNH) != r.nextHop {
				return fmt.Errorf("route %s next-hop mismatch: got %v, want %v", r.prefix, gotNH, r.nextHop)
			}

		// Resolution route validation
		case r.intf != "":
			gotIntf := gnmi.Get(t, dut, sp.Static(r.prefix).NextHop("0").InterfaceRef().Interface().Config())

			if gotIntf != r.intf {
				return fmt.Errorf("route %s interface mismatch: got %v, want %v", r.prefix, gotIntf, r.intf)
			}
		}
	}

	t.Logf("Validated %d static routes", len(staticRoutes))
	return nil
}

// verifyTrafficFlow validates OTG flow metrics to ensure packets are transmitted and received without loss.
func verifyTrafficFlow(t *testing.T, ate *ondatra.ATEDevice, flowName string) error {
	t.Helper()

	recvMetric := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(flowName).State())

	tx := recvMetric.GetCounters().GetOutPkts()
	rx := recvMetric.GetCounters().GetInPkts()

	t.Logf("Validating flow %s: TX=%d, RX=%d", flowName, tx, rx)

	if tx < packetPerSecond {
		return fmt.Errorf("flow %s: expected traffic not transmitted", flowName)
	} else if rx > packetPerSecond && rx != tx {
		return fmt.Errorf("flow %s: packet loss detected! TX: %d, RX: %d", flowName, tx, rx)
	} else {
		t.Logf("Flow %s: Success", flowName)
	}
	return nil
}

// fetchDUTCounters retrieves ingress and egress packet counters from all DUT interfaces.
func fetchDUTCounters(t *testing.T, dut *ondatra.DUTDevice) map[string]dutCounters {
	t.Helper()

	counters := make(map[string]dutCounters)

	for _, p := range dutPorts {

		intf := dut.Port(t, p.Name).Name()

		inPkts := gnmi.Get(t, dut, gnmi.OC().Interface(intf).Counters().InUnicastPkts().State())
		outPkts := gnmi.Get(t, dut, gnmi.OC().Interface(intf).Counters().OutUnicastPkts().State())

		t.Logf("DUT %s counters: IN=%d OUT=%d", intf, inPkts, outPkts)

		counters[intf] = dutCounters{
			inPkts:  inPkts,
			outPkts: outPkts,
		}
	}

	return counters
}

// validateDUTCounters compares DUT counter deltas before and after traffic to verify forwarding behavior.
func validateDUTCounters(t *testing.T, before, after map[string]dutCounters, flowType string) error {
	t.Helper()

	for intf, beforeCtr := range before {

		afterCtr := after[intf]

		inDelta := afterCtr.inPkts - beforeCtr.inPkts
		outDelta := afterCtr.outPkts - beforeCtr.outPkts

		t.Logf("DUT %s delta: IN=%d OUT=%d", intf, inDelta, outDelta)

		switch flowType {

		case flowTypeForwarding:
			if intf == "port1" || intf == "port2" || intf == "port3" {
				if inDelta >= expectedPktsPerPort {
					return fmt.Errorf("%s ingress packets mismatch: got %d want %d", strings.ToLower(intf), inDelta, expectedPktsPerPort)
				}
			}

			if intf == "port4" {
				if outDelta >= expectedTotalPktsPerPort {
					return fmt.Errorf("port4 egress packets mismatch: got %d want %d", outDelta, expectedTotalPktsPerPort)
				}
			}

		case flowTypeStatic:
			if intf == "port4" {
				if inDelta >= expectedTotalPktsPerPort {
					return fmt.Errorf("port4 ingress packets mismatch: got %d want %d", inDelta, expectedTotalPktsPerPort)
				}
			}

			if intf == "port1" || intf == "port2" || intf == "port3" {
				if outDelta >= expectedPktsPerPort {
					return fmt.Errorf("%s egress packets mismatch: got %d want %d", strings.ToLower(intf), outDelta, expectedPktsPerPort)
				}
			}
		}
	}
	return nil
}
