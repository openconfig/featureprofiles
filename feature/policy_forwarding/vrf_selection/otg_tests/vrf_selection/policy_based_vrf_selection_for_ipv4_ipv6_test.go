package vrf_selection_test

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
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygnmi/ygnmi"
)

const (
	dutDefaultAS           = 65000
	dutNonDefaultAS        = 65001
	ateAS1                 = 65002
	ateAS2                 = 65003
	vrfSelectionPolicyName = "pf16-vrf-selection"
	IPv4                   = "IPv4"
	IPv6                   = "IPv6"
	trafficFrameSize       = 256
	trafficRatePps         = 100
	noOfPackets            = 1000
	ipv4PrefixLen          = 24
	ipv6PrefixLen          = 64
	ipv4Prefix1            = "50.1.1.0"
	ipv4Prefix2            = "60.1.1.0"
	ipv6Prefix3            = "2050:db8:1::0"
	ipv6Prefix4            = "2060:db8:2::0"
	bgpWaitTime            = 1 * time.Minute
	routeTimeout           = 15 * time.Second
	trafficTimeout         = 30 * time.Second
	routeCount             = 1
	ingress                = false
	egress                 = true
	pktCountErrorPct       = 0.03 // 3% error tolerance for packet counts
)

var (
	dutPort1 = attrs.Attributes{
		Name:    "port1",
		Desc:    "DUT Port1",
		IPv4:    "192.0.2.1",
		IPv4Len: 30,
		IPv6:    "2001:db8:1::1",
		IPv6Len: 64,
	}
	dutPort2 = attrs.Attributes{
		Name:    "port2",
		Desc:    "DUT Port2",
		IPv4:    "192.0.2.5",
		IPv4Len: 30,
		IPv6:    "2001:db8:2::1",
		IPv6Len: 64,
	}
	dutPort3 = attrs.Attributes{
		Name:    "port3",
		Desc:    "DUT Port3",
		IPv4:    "192.0.2.9",
		IPv4Len: 30,
		IPv6:    "2001:db8:3::1",
		IPv6Len: 64,
	}

	ate1Port1 = attrs.Attributes{
		Name:    "port1",
		Desc:    "ATE1 Port1",
		IPv4:    "192.0.2.2",
		IPv4Len: 30,
		IPv6:    "2001:db8:1::2",
		IPv6Len: 64,
		MAC:     "00:01:12:00:00:01",
	}
	ate2Port1 = attrs.Attributes{
		Name:    "port2",
		Desc:    "ATE2 Port1",
		IPv4:    "192.0.2.6",
		IPv4Len: 30,
		IPv6:    "2001:db8:2::2",
		IPv6Len: 64,
		MAC:     "00:01:12:00:00:02",
	}
	ate2Port2 = attrs.Attributes{
		Name:    "port3",
		Desc:    "ATE2 Port2",
		IPv4:    "192.0.2.10",
		IPv4Len: 30,
		IPv6:    "2001:db8:3::2",
		IPv6Len: 64,
		MAC:     "00:01:12:00:00:03",
	}

	routesToAdvertise = map[string]string{
		fmt.Sprintf("%s/%d", ipv4Prefix1, ipv4PrefixLen): IPv4,
		fmt.Sprintf("%s/%d", ipv4Prefix2, ipv4PrefixLen): IPv4,
		fmt.Sprintf("%s/%d", ipv6Prefix3, ipv6PrefixLen): IPv6,
		fmt.Sprintf("%s/%d", ipv6Prefix4, ipv6PrefixLen): IPv6,
	}

	dutInterfaceMap = map[string]bool{}

	interfaceCounterPackets = map[string]uint64{}
	niToInterfaceMap        = map[string]string{}

	pktCountError = uint64(pktCountErrorPct * noOfPackets)

	sourcePort                = ate1Port1
	defaultDestinationPort    = ate2Port1
	nonDefaultDestinationPort = ate2Port2
	nonDefaultNIName          = "NonDefaultNI"
	defaultNIName             = ""
)

type testCase struct {
	name     string
	vrfRules []cfgplugins.VrfRule
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestPolicyBasedVRFSelection(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	otg := ate.OTG()
	defaultNIName = deviations.DefaultNetworkInstance(dut)

	configureDUT(t, dut)
	config := configureATE(t, ate)

	otg.PushConfig(t, config)
	otg.StartProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), config, IPv4)
	otgutils.WaitForARP(t, ate.OTG(), config, IPv6)

	verifyBGP(t, dut, ate1Port1, defaultNIName)
	verifyBGP(t, dut, ate2Port1, defaultNIName)
	verifyBGP(t, dut, ate2Port2, nonDefaultNIName)

	dp2 := dut.Port(t, "port2")
	dp3 := dut.Port(t, "port3")

	niToInterfaceMap[defaultNIName] = dp2.Name()
	niToInterfaceMap[nonDefaultNIName] = dp3.Name()

	verifyRoutes(t, dut, defaultNIName)
	verifyRoutes(t, dut, nonDefaultNIName)

	testCases := []testCase{
		{
			name: "PF-1.6.1: Default VRF for all flows with regular traffic profile",
			vrfRules: []cfgplugins.VrfRule{
				{Index: 1, IpType: IPv4, SourcePrefix: ipv4Prefix1, PrefixLength: ipv4PrefixLen, NetInstName: defaultNIName},
				{Index: 2, IpType: IPv4, SourcePrefix: ipv4Prefix2, PrefixLength: ipv4PrefixLen, NetInstName: defaultNIName},
				{Index: 3, IpType: IPv6, SourcePrefix: ipv6Prefix3, PrefixLength: ipv6PrefixLen, NetInstName: defaultNIName},
				{Index: 4, IpType: IPv6, SourcePrefix: ipv6Prefix4, PrefixLength: ipv6PrefixLen, NetInstName: defaultNIName},
			},
		},
		{
			name: "PF-1.6.2: Traffic from ATE1 to ATE2, 1 Prefix migrated to Non-Default VRF using the VRF selection policy",
			vrfRules: []cfgplugins.VrfRule{
				{Index: 1, IpType: IPv4, SourcePrefix: ipv4Prefix1, PrefixLength: ipv4PrefixLen, NetInstName: nonDefaultNIName},
				{Index: 2, IpType: IPv4, SourcePrefix: ipv4Prefix2, PrefixLength: ipv4PrefixLen, NetInstName: defaultNIName},
				{Index: 3, IpType: IPv6, SourcePrefix: ipv6Prefix3, PrefixLength: ipv6PrefixLen, NetInstName: defaultNIName},
				{Index: 4, IpType: IPv6, SourcePrefix: ipv6Prefix4, PrefixLength: ipv6PrefixLen, NetInstName: defaultNIName},
			},
		},
		{
			name: "PF-1.6.3: Traffic from ATE1 to ATE2, 2 Prefixes migrated to Non-Default VRF using the VRF selection policy",
			vrfRules: []cfgplugins.VrfRule{
				{Index: 1, IpType: IPv4, SourcePrefix: ipv4Prefix1, PrefixLength: ipv4PrefixLen, NetInstName: nonDefaultNIName},
				{Index: 2, IpType: IPv4, SourcePrefix: ipv4Prefix2, PrefixLength: ipv4PrefixLen, NetInstName: nonDefaultNIName},
				{Index: 3, IpType: IPv6, SourcePrefix: ipv6Prefix3, PrefixLength: ipv6PrefixLen, NetInstName: defaultNIName},
				{Index: 4, IpType: IPv6, SourcePrefix: ipv6Prefix4, PrefixLength: ipv6PrefixLen, NetInstName: defaultNIName},
			},
		},
		{
			name: "PF-1.6.4: Traffic from ATE1 to ATE2, 3 Prefixes migrated to Non-Default VRF using the VRF selection policy",
			vrfRules: []cfgplugins.VrfRule{
				{Index: 1, IpType: IPv4, SourcePrefix: ipv4Prefix1, PrefixLength: ipv4PrefixLen, NetInstName: nonDefaultNIName},
				{Index: 2, IpType: IPv4, SourcePrefix: ipv4Prefix2, PrefixLength: ipv4PrefixLen, NetInstName: nonDefaultNIName},
				{Index: 3, IpType: IPv6, SourcePrefix: ipv6Prefix3, PrefixLength: ipv6PrefixLen, NetInstName: nonDefaultNIName},
				{Index: 4, IpType: IPv6, SourcePrefix: ipv6Prefix4, PrefixLength: ipv6PrefixLen, NetInstName: defaultNIName},
			},
		},
		{
			name: "PF-1.6.5: Traffic from ATE1 to ATE2, 4 Prefixes migrated to Non-Default VRF using the VRF selection policy",
			vrfRules: []cfgplugins.VrfRule{
				{Index: 1, IpType: IPv4, SourcePrefix: ipv4Prefix1, PrefixLength: ipv4PrefixLen, NetInstName: nonDefaultNIName},
				{Index: 2, IpType: IPv4, SourcePrefix: ipv4Prefix2, PrefixLength: ipv4PrefixLen, NetInstName: nonDefaultNIName},
				{Index: 3, IpType: IPv6, SourcePrefix: ipv6Prefix3, PrefixLength: ipv6PrefixLen, NetInstName: nonDefaultNIName},
				{Index: 4, IpType: IPv6, SourcePrefix: ipv6Prefix4, PrefixLength: ipv6PrefixLen, NetInstName: nonDefaultNIName},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			runTest(t, dut, tc, ate, config)
		})
	}
}

func configureFlows(t *testing.T, config *gosnappi.Config, tc testCase) {
	(*config).Flows().Clear()
	for _, rule := range tc.vrfRules {
		var destinationPort attrs.Attributes
		switch rule.NetInstName {
		case defaultNIName:
			destinationPort = defaultDestinationPort
		case nonDefaultNIName:
			destinationPort = nonDefaultDestinationPort
		default:
			t.Fatalf("Invalid VRF name %s in rule %d", rule.NetInstName, rule.Index)
		}
		flowName := fmt.Sprintf("%sPrefix%d", rule.IpType, rule.Index)
		flow := (*config).Flows().Add().SetName(flowName)
		flow.Metrics().SetEnable(true)
		flow.TxRx().Device().SetTxNames([]string{fmt.Sprintf("%s.%s", sourcePort.Name, rule.IpType)}).SetRxNames([]string{fmt.Sprintf("%s.%s", destinationPort.Name, rule.IpType)})
		flow.Size().SetFixed(trafficFrameSize)
		flow.Rate().SetPps(trafficRatePps)
		flow.Duration().SetFixedPackets(gosnappi.NewFlowFixedPackets().SetPackets(noOfPackets))

		eth := flow.Packet().Add().Ethernet()
		eth.Src().SetValue(sourcePort.MAC)

		switch rule.IpType {
		case IPv4:
			ipv4 := flow.Packet().Add().Ipv4()
			ipv4.Src().SetValue(rule.SourcePrefix)
			ipv4.Dst().SetValue(destinationPort.IPv4)
		case IPv6:
			ipv6 := flow.Packet().Add().Ipv6()
			ipv6.Src().SetValue(rule.SourcePrefix)
			ipv6.Dst().SetValue(destinationPort.IPv6)

		default:
			t.Errorf("Invalid traffic type %s", rule.IpType)
		}
	}
}

func runTest(t *testing.T, dut *ondatra.DUTDevice, tc testCase, ate *ondatra.ATEDevice, config gosnappi.Config) {
	otg := ate.OTG()
	t.Logf("Configuring Policy Forwarding for testcase %s", tc.name)
	interfaceName := dut.Port(t, "port1").Name()
	gnmi.Delete(t, dut, gnmi.OC().NetworkInstance(defaultNIName).PolicyForwarding().Config())
	configureVrfSelectionPolicy(t, dut, defaultNIName, vrfSelectionPolicyName, interfaceName, tc.vrfRules)

	configureFlows(t, &config, tc)

	otg.PushConfig(t, config)
	otg.StartProtocols(t)
	getInterfaceCounterValues(t, dut)

	otg.StartTraffic(t)
	for _, fc := range config.Flows().Items() {
		waitForTraffic(t, otg, fc.Name(), trafficTimeout)
	}
	verifyDutInterfaceCounters(t, dut, tc)
	verifyFlowStatistics(t, ate, config)
}

func configureVrfSelectionPolicy(t *testing.T, dut *ondatra.DUTDevice, niName, policyName, interfaceName string, vrfRules []cfgplugins.VrfRule) {
	_, ni, pf := cfgplugins.SetupPolicyForwardingInfraOC(niName)
	cfgplugins.ConfigureVrfSelectionPolicy(t, dut, pf, policyName, vrfRules)
	cfgplugins.ApplyVrfSelectionPolicyToInterfaceOC(t, pf, interfaceName, policyName)
	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(niName).Config(), ni)
}

func verifyFlowStatistics(t *testing.T, ate *ondatra.ATEDevice, config gosnappi.Config) {
	otg := ate.OTG()

	otgutils.LogFlowMetrics(t, otg, config)
	otgutils.LogPortMetrics(t, otg, config)

	for _, fc := range config.Flows().Items() {
		flowMetrics := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(fc.Name()).State())
		if *flowMetrics.Counters.OutPkts == 0 {
			t.Errorf("No packets transmitted")
		}
		if *flowMetrics.Counters.InPkts != noOfPackets {
			t.Errorf("Flow %s received %d packets, expected %d packets", fc.Name(), *flowMetrics.Counters.InPkts, noOfPackets)
		}
	}
}

func waitForTraffic(t *testing.T, otg *otg.OTG, flowName string, timeout time.Duration) {
	transmitPath := gnmi.OTG().Flow(flowName).Transmit().State()
	_, ok := gnmi.Watch(t, otg, transmitPath, timeout, func(val *ygnmi.Value[bool]) bool {
		transmitState, present := val.Val()
		return present && !transmitState
	}).Await(t)

	if !ok {
		t.Errorf("Traffic for flow %s did not stop within the timeout of %d", flowName, timeout)
	} else {
		t.Logf("Traffic for flow %s has stopped", flowName)
	}
}

func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")
	dp3 := dut.Port(t, "port3")

	dutInterfaceMap[dp1.Name()] = ingress
	dutInterfaceMap[dp2.Name()] = egress
	dutInterfaceMap[dp3.Name()] = egress

	t.Logf("Configuring Hardware Init")
	configureHardwareInit(t, dut)

	cfgplugins.EnableDefaultNetworkInstanceBgp(t, dut, dutDefaultAS)
	isDefaultVrf := true
	t.Logf("Configuring Network Instances")
	defaultNI := cfgplugins.ConfigureNetworkInstance(t, dut, defaultNIName, isDefaultVrf)
	nonDefaultNI := cfgplugins.ConfigureNetworkInstance(t, dut, nonDefaultNIName, !isDefaultVrf)

	t.Logf("Configuring BGP")
	cfgplugins.ConfigureBGPNeighbor(t, dut, defaultNI, dutPort1.IPv4, ate1Port1.IPv4, dutDefaultAS, dutDefaultAS, IPv4, true)
	cfgplugins.ConfigureBGPNeighbor(t, dut, defaultNI, dutPort2.IPv4, ate2Port1.IPv4, dutDefaultAS, ateAS1, IPv4, true)
	cfgplugins.ConfigureBGPNeighbor(t, dut, defaultNI, dutPort2.IPv4, ate2Port1.IPv6, dutDefaultAS, ateAS1, IPv6, true)
	cfgplugins.ConfigureBGPNeighbor(t, dut, nonDefaultNI, dutPort3.IPv4, ate2Port2.IPv4, dutNonDefaultAS, ateAS2, IPv4, true)
	cfgplugins.ConfigureBGPNeighbor(t, dut, nonDefaultNI, dutPort3.IPv4, ate2Port2.IPv6, dutNonDefaultAS, ateAS2, IPv6, true)

	cfgplugins.UpdateNetworkInstanceOnDut(t, dut, defaultNIName, defaultNI)
	cfgplugins.UpdateNetworkInstanceOnDut(t, dut, nonDefaultNIName, nonDefaultNI)

	t.Logf("Configuring Interfaces")
	configureDUTPort(t, dut, &dutPort1, dp1, defaultNIName)
	configureDUTPort(t, dut, &dutPort2, dp2, defaultNIName)
	configureDUTPort(t, dut, &dutPort3, dp3, nonDefaultNIName)
}

func configureHardwareInit(t *testing.T, dut *ondatra.DUTDevice) {
	hardwareInitCfg := cfgplugins.NewDUTHardwareInit(t, dut, cfgplugins.FeatureVrfSelectionExtended)
	if hardwareInitCfg == "" {
		return
	}
	cfgplugins.PushDUTHardwareInitConfig(t, dut, hardwareInitCfg)
}

func configureDUTPort(t *testing.T, dut *ondatra.DUTDevice, attrs *attrs.Attributes, p *ondatra.Port, niName string) {
	d := gnmi.OC()
	cfgplugins.AssignToNetworkInstance(t, dut, p.Name(), niName, 0)
	i := attrs.NewOCInterface(p.Name(), dut)
	gnmi.Replace(t, dut, d.Interface(p.Name()).Config(), i)
}

func verifyBGP(t *testing.T, dut *ondatra.DUTDevice, otgPort attrs.Attributes, vrfName string) {
	bgpPath := gnmi.OC().NetworkInstance(vrfName).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()

	t.Logf("Waiting for BGP session to be ESTABLISHED in %s", vrfName)
	status, ok := gnmi.Watch(t, dut, bgpPath.Neighbor(otgPort.IPv4).SessionState().State(), bgpWaitTime, func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
		state, present := val.Val()
		return present && state == oc.Bgp_Neighbor_SessionState_ESTABLISHED
	}).Await(t)
	if !ok {
		t.Fatalf("BGP in %s did not establish: %v", vrfName, status)
	}
}

func getInterfaceCounterValues(t *testing.T, dut *ondatra.DUTDevice) {
	for intfName, direction := range dutInterfaceMap {
		counterPath := gnmi.OC().Interface(intfName).Counters().State()
		counterValues := gnmi.Get(t, dut, counterPath)
		switch direction {
		case ingress:
			t.Logf("Interface %s in unicast packets: %d", intfName, counterValues.GetInUnicastPkts())
			interfaceCounterPackets[intfName] = counterValues.GetInUnicastPkts()
		case egress:
			t.Logf("Interface %s out unicast packets: %d", intfName, counterValues.GetOutUnicastPkts())
			interfaceCounterPackets[intfName] = counterValues.GetOutUnicastPkts()
		}
	}
}

func verifyDutInterfaceCounters(t *testing.T, dut *ondatra.DUTDevice, tc testCase) {
	totalFlowCount := len(tc.vrfRules)
	var expectedPkts uint64
	expectedIngressPkts := uint64(totalFlowCount * noOfPackets)
	expectedEgressPacketsPerVRF := make(map[string]uint64)
	for _, rule := range tc.vrfRules {
		expectedEgressPacketsPerVRF[niToInterfaceMap[rule.NetInstName]] += uint64(noOfPackets)
	}

	for intfName, direction := range dutInterfaceMap {
		counterPath := gnmi.OC().Interface(intfName).Counters().State()
		counterValues := gnmi.Get(t, dut, counterPath)
		var packetType string
		var total uint64
		switch direction {
		case ingress:
			t.Logf("Verifying ingress counters for interface %s", intfName)
			total = counterValues.GetInUnicastPkts()
			packetType = "in-unicast-pkts"
			expectedPkts = expectedIngressPkts
		case egress:
			t.Logf("Verifying egress counters for interface %s", intfName)
			total = counterValues.GetOutUnicastPkts()
			packetType = "out-unicast-pkts"
			expectedPkts = expectedEgressPacketsPerVRF[intfName]
		}

		got := total - interfaceCounterPackets[intfName]
		message := fmt.Sprintf("Interface %s %s: got %d, want %d ± %d", intfName, packetType, got, expectedPkts, pktCountError)
		if -pktCountError < got-expectedPkts || got-expectedPkts > pktCountError {
			t.Errorf("Error: %s", message)
		} else {
			t.Log(message)
		}
		interfaceCounterPackets[intfName] = total
	}
}

func verifyRoutes(t *testing.T, dut *ondatra.DUTDevice, vrfName string) {
	for route, ipType := range routesToAdvertise {
		var ok bool
		switch ipType {
		case IPv4:
			aftVrf1 := gnmi.OC().NetworkInstance(vrfName).Afts().Ipv4Entry(route)
			_, ok = gnmi.Watch(t, dut, aftVrf1.State(), routeTimeout, func(val *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv4Entry]) bool {
				return val.IsPresent()
			}).Await(t)
		case IPv6:
			route = strings.Replace(route, "::0/", "::/", 1)
			aftVrf1 := gnmi.OC().NetworkInstance(vrfName).Afts().Ipv6Entry(route)
			_, ok = gnmi.Watch(t, dut, aftVrf1.State(), routeTimeout, func(val *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv6Entry]) bool {
				return val.IsPresent()
			}).Await(t)
		}
		if !ok {
			t.Errorf("Route %s is not installed in AFT for %s", route, vrfName)
		} else {
			t.Logf("Route %s successfully installed in AFT for %s", route, vrfName)
		}
	}
}

func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	config := gosnappi.NewConfig()

	p1 := ate.Port(t, "port1")
	p2 := ate.Port(t, "port2")
	p3 := ate.Port(t, "port3")

	d1 := ate1Port1.AddToOTG(config, p1, &dutPort1)
	d2 := ate2Port1.AddToOTG(config, p2, &dutPort2)
	d3 := ate2Port2.AddToOTG(config, p3, &dutPort3)

	ip1 := d1.Ethernets().Items()[0].Ipv4Addresses().Items()[0]
	ip2 := d2.Ethernets().Items()[0].Ipv4Addresses().Items()[0]
	ip2v6 := d2.Ethernets().Items()[0].Ipv6Addresses().Items()[0]
	ip3 := d3.Ethernets().Items()[0].Ipv4Addresses().Items()[0]
	ip3v6 := d3.Ethernets().Items()[0].Ipv6Addresses().Items()[0]

	bgp1 := d1.Bgp().SetRouterId(ate1Port1.IPv4)
	bgp1Peer := bgp1.Ipv4Interfaces().Add().SetIpv4Name(ip1.Name()).Peers().Add().SetName(fmt.Sprintf("%s.BGP.peer", d1.Name()))
	bgp1Peer.SetPeerAddress(dutPort1.IPv4).SetAsNumber(uint32(dutDefaultAS)).SetAsType(gosnappi.BgpV4PeerAsType.IBGP)

	bgp2 := d2.Bgp().SetRouterId(ate2Port1.IPv4)
	bgp2Peer := bgp2.Ipv4Interfaces().Add().SetIpv4Name(ip2.Name()).Peers().Add().SetName(fmt.Sprintf("%s.v4.BGP.peer", d2.Name()))
	bgp2Peer.SetPeerAddress(dutPort2.IPv4).SetAsNumber(uint32(ateAS1)).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)

	bgp2Netv4 := bgp2Peer.V4Routes().Add().SetName("vrf-p2-v4-routes")
	bgp2Netv4.SetNextHopIpv4Address(ip2.Address())
	bgp2Netv4.Addresses().Add().SetAddress(ipv4Prefix1).SetPrefix(ipv4PrefixLen).SetCount(routeCount)
	bgp2Netv4.Addresses().Add().SetAddress(ipv4Prefix2).SetPrefix(ipv4PrefixLen).SetCount(routeCount)

	bgp2Peerv6 := bgp2.Ipv6Interfaces().Add().SetIpv6Name(ip2v6.Name()).Peers().Add().SetName(fmt.Sprintf("%s.v6.BGP.peer", d2.Name()))
	bgp2Peerv6.SetPeerAddress(dutPort2.IPv6).SetAsNumber(uint32(ateAS1)).SetAsType(gosnappi.BgpV6PeerAsType.EBGP)

	bgp2Netv6 := bgp2Peerv6.V6Routes().Add().SetName("vrf-p2-v6-routes")
	bgp2Netv6.SetNextHopIpv6Address(ip2v6.Address())
	bgp2Netv6.Addresses().Add().SetAddress(ipv6Prefix3).SetPrefix(ipv6PrefixLen).SetCount(routeCount)
	bgp2Netv6.Addresses().Add().SetAddress(ipv6Prefix4).SetPrefix(ipv6PrefixLen).SetCount(routeCount)

	bgp3 := d3.Bgp().SetRouterId(ate2Port2.IPv4)
	bgp3Peer := bgp3.Ipv4Interfaces().Add().SetIpv4Name(ip3.Name()).Peers().Add().SetName(fmt.Sprintf("%s.v4.BGP.peer", d3.Name()))
	bgp3Peer.SetPeerAddress(dutPort3.IPv4).SetAsNumber(uint32(ateAS2)).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)

	bgp3Netv4 := bgp3Peer.V4Routes().Add().SetName("vrf-p3-v4-routes")
	bgp3Netv4.SetNextHopIpv4Address(ip3.Address())
	bgp3Netv4.Addresses().Add().SetAddress(ipv4Prefix1).SetPrefix(ipv4PrefixLen).SetCount(routeCount)
	bgp3Netv4.Addresses().Add().SetAddress(ipv4Prefix2).SetPrefix(ipv4PrefixLen).SetCount(routeCount)

	bgp3Peerv6 := bgp3.Ipv6Interfaces().Add().SetIpv6Name(ip3v6.Name()).Peers().Add().SetName(fmt.Sprintf("%s.v6.BGP.peer", d3.Name()))
	bgp3Peerv6.SetPeerAddress(dutPort3.IPv6).SetAsNumber(uint32(ateAS2)).SetAsType(gosnappi.BgpV6PeerAsType.EBGP)

	bgp3Netv6 := bgp3Peerv6.V6Routes().Add().SetName("vrf-p3-v6-routes")
	bgp3Netv6.SetNextHopIpv6Address(ip3v6.Address())
	bgp3Netv6.Addresses().Add().SetAddress(ipv6Prefix3).SetPrefix(ipv6PrefixLen).SetCount(routeCount)
	bgp3Netv6.Addresses().Add().SetAddress(ipv6Prefix4).SetPrefix(ipv6PrefixLen).SetCount(routeCount)

	return config
}
