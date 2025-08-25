package encap_gre_ipv4_test

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
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
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	trafficFrameSize  = 512
	trafficRatePps    = 1000
	noOfPackets       = 1024
	greProtocol       = 47
	trafficPolicyName = "IP_MATCH_TRAFFIC_POLICY"
	tunnelCount       = 32
	captureFilePath   = "/tmp/capture.pcap"
	testTimeout       = 10 * time.Second
	ipv4              = "IPv4"
	ipv6              = "IPv6"
	ipv4PrefixLen     = 32
	ipv6PrefixLen     = 128
	nhGroupName       = "SRC_NH"
)

var (
	dutlo0Attrs = attrs.Attributes{
		Name:    "Loopback0",
		IPv4:    "192.0.20.2",
		IPv6:    "2001:DB8:0::10",
		IPv4Len: 32,
		IPv6Len: 128,
	}

	ruleSequenceMap = map[string]uint8{
		"rule-src1-v4": 1,
		"rule-src1-v6": 2,
		"rule-src2-v4": 3,
		"rule-src2-v6": 4,
	}

	ruleMatchedPackets = map[string]uint64{
		"rule-src1-v4": 0,
		"rule-src1-v6": 0,
		"rule-src2-v4": 0,
		"rule-src2-v6": 0,
	}

	dutPort1 = attrs.Attributes{Desc: "Dut port 1", IPv4: "192.0.2.1", IPv4Len: 30, IPv6: "2001:DB8:0::1", IPv6Len: 126}
	dutPort2 = attrs.Attributes{Desc: "Dut port 2", IPv4: "192.0.2.5", IPv4Len: 30, IPv6: "2001:DB8:0::5", IPv6Len: 126}
	dutPort3 = attrs.Attributes{Desc: "Dut port 3", IPv4: "192.0.2.9", IPv4Len: 30, IPv6: "2001:DB8:0::9", IPv6Len: 126}

	otgPort1 = attrs.Attributes{Desc: "OTG port 1", Name: "port1", MAC: "00:01:12:00:00:01", IPv4: "192.0.2.2", IPv4Len: 30, IPv6: "2001:DB8:0::2", IPv6Len: 126, MTU: 9216}
	otgPort2 = attrs.Attributes{Desc: "OTG port 1", Name: "port2", MAC: "00:01:12:00:00:02", IPv4: "192.0.2.6", IPv4Len: 30, IPv6: "2001:DB8:0::6", IPv6Len: 126, MTU: 2000}
	otgPort3 = attrs.Attributes{Desc: "OTG port 1", Name: "port3", MAC: "00:01:12:00:00:03", IPv4: "192.0.2.10", IPv4Len: 30, IPv6: "2001:DB8:0::A", IPv6Len: 126, MTU: 2000}

	tunnelDestinations = []string{}
	dscpValues         = []uint8{0, 8, 16, 24, 32, 40, 48, 56}
	classifierName     = "qos-classifier-1"
)

type ipFlow interface {
	HasSrc() bool
	HasDst() bool
}

type testCase struct {
	name                   string
	ipType                 string
	capturePort            string
	captureFilename        string
	srcDstPortPair         []attrs.Attributes
	applyCustomFlow        func(t *testing.T, top *gosnappi.Config, tc testCase, flow *gosnappi.Flow, packet *ipFlow)
	verifyOutput           func(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, tc testCase)
	checkEncapDscp         bool
	checkEncapLoadBalanced bool
	flowName               string
	policyRule             string
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestEncapGREIPv4(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	top := gosnappi.NewConfig()
	configureTunnelDestinations()
	configureDUT(t, dut)

	ap1 := ate.Port(t, "port1")
	ap2 := ate.Port(t, "port2")
	ap3 := ate.Port(t, "port3")

	otgPort1.AddToOTG(top, ap1, &dutPort1)
	otgPort2.AddToOTG(top, ap2, &dutPort2)
	otgPort3.AddToOTG(top, ap3, &dutPort3)

	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv6")

	testCases := []testCase{
		{
			name:           "PF-1.1.1: Verify PF GRE encapsulate action for IPv4 traffic",
			ipType:         ipv4,
			capturePort:    "port2",
			srcDstPortPair: []attrs.Attributes{otgPort1, otgPort2},
			applyCustomFlow: func(t *testing.T, top *gosnappi.Config, tc testCase, flow *gosnappi.Flow, packet *ipFlow) {
				foundIpv4, ok := (*packet).(gosnappi.FlowIpv4)
				if !ok || foundIpv4 == nil {
					return
				}
				foundIpv4.Src().Increment().SetStart(tc.srcDstPortPair[0].IPv4).SetStep("0.0.0.1").SetCount(1000)
			},
			verifyOutput: func(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, tc testCase) {
				checkGreCapture(t, tc)
				checkPolicyStatistics(t, dut, tc)
			},
			checkEncapDscp:         false,
			checkEncapLoadBalanced: true,
			policyRule:             "rule-src1-v4",
			flowName:               "FlowTC1",
		},
		{
			name:           "PF-1.1.2: Verify PF GRE encapsulate action for IPv6 traffic",
			ipType:         ipv6,
			capturePort:    "port2",
			srcDstPortPair: []attrs.Attributes{otgPort1, otgPort2},
			applyCustomFlow: func(t *testing.T, top *gosnappi.Config, tc testCase, flow *gosnappi.Flow, packet *ipFlow) {
				foundIpv6, ok := (*packet).(gosnappi.FlowIpv6)
				if !ok || foundIpv6 == nil {
					return
				}
				foundIpv6.Src().Increment().SetStart(tc.srcDstPortPair[0].IPv6).SetStep("::1").SetCount(1000)
			},
			verifyOutput: func(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, tc testCase) {
				checkGreCapture(t, tc)
				checkPolicyStatistics(t, dut, tc)
			},
			checkEncapDscp:         false,
			checkEncapLoadBalanced: true,
			policyRule:             "rule-src1-v6",
			flowName:               "FlowTC2",
		},
		{
			name:            "PF-1.1.3: Verify PF IPV4 forward action",
			ipType:          ipv4,
			srcDstPortPair:  []attrs.Attributes{otgPort1, otgPort3},
			applyCustomFlow: nil,
			verifyOutput: func(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, tc testCase) {
				checkFlowStats(t, ate, tc)
				checkPolicyStatistics(t, dut, tc)
			},
			checkEncapDscp:         false,
			checkEncapLoadBalanced: false,
			policyRule:             "rule-src2-v4",
			flowName:               "FlowTC3",
		},
		{
			name:            "PF-1.1.4: Verify PF IPV6 forward action",
			ipType:          ipv6,
			srcDstPortPair:  []attrs.Attributes{otgPort1, otgPort3},
			applyCustomFlow: nil,
			verifyOutput: func(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, tc testCase) {
				checkFlowStats(t, ate, tc)
				checkPolicyStatistics(t, dut, tc)
			},
			checkEncapDscp:         false,
			checkEncapLoadBalanced: false,
			policyRule:             "rule-src2-v6",
			flowName:               "FlowTC4",
		},
		{
			name:           "PF-1.1.5: Verify PF GRE DSCP copy to outer header for IPv4 traffic",
			ipType:         ipv4,
			capturePort:    "port2",
			srcDstPortPair: []attrs.Attributes{otgPort1, otgPort2},
			applyCustomFlow: func(t *testing.T, top *gosnappi.Config, tc testCase, flow *gosnappi.Flow, packet *ipFlow) {
				foundIpv4, ok := (*packet).(gosnappi.FlowIpv4)
				if !ok || foundIpv4 == nil {
					return
				}

				var dscpValues32 []uint32
				for _, v := range dscpValues {
					dscpValues32 = append(dscpValues32, uint32(v))
				}
				foundIpv4.Priority().Dscp().Phb().SetValues(dscpValues32)
			},
			verifyOutput: func(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, tc testCase) {
				checkGreCapture(t, tc)
				checkPolicyStatistics(t, dut, tc)
			},
			checkEncapDscp:         true,
			checkEncapLoadBalanced: false,
			policyRule:             "rule-src1-v4",
			flowName:               "FlowTC5",
		},
		{
			name:           "PF-1.1.6: Verify PF GRE DSCP copy to outer header for IPv6 traffic",
			ipType:         ipv6,
			capturePort:    "port2",
			srcDstPortPair: []attrs.Attributes{otgPort1, otgPort2},
			applyCustomFlow: func(t *testing.T, top *gosnappi.Config, tc testCase, flow *gosnappi.Flow, packet *ipFlow) {
				foundIpv6, ok := (*packet).(gosnappi.FlowIpv6)
				if !ok || foundIpv6 == nil {
					return
				}

				var tcValues []uint32
				for _, dscp := range dscpValues {
					tcValues = append(tcValues, uint32(dscp<<2))
				}
				foundIpv6.TrafficClass().SetValues(tcValues)
			},
			verifyOutput: func(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, tc testCase) {
				checkGreCapture(t, tc)
				checkPolicyStatistics(t, dut, tc)
			},
			checkEncapDscp:         true,
			checkEncapLoadBalanced: false,
			policyRule:             "rule-src1-v6",
			flowName:               "FlowTC6",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			runTest(t, tc, dut, ate, top)
		})
	}
}

func configureFlows(t *testing.T, config *gosnappi.Config, tc testCase) {
	(*config).Flows().Clear()
	flow := (*config).Flows().Add().SetName(tc.flowName)
	flow.Metrics().SetEnable(true)
	flow.TxRx().Device().SetTxNames([]string{fmt.Sprintf("%s.%s", tc.srcDstPortPair[0].Name, tc.ipType)}).SetRxNames([]string{fmt.Sprintf("%s.%s", tc.srcDstPortPair[1].Name, tc.ipType)})
	flow.Size().SetFixed(trafficFrameSize)
	flow.Rate().SetPps(trafficRatePps)
	flow.Duration().SetFixedPackets(gosnappi.NewFlowFixedPackets().SetPackets(noOfPackets))

	eth := flow.Packet().Add().Ethernet()
	eth.Src().SetValue(tc.srcDstPortPair[0].MAC)
	var ipPacket ipFlow
	switch tc.ipType {
	case ipv4:
		ipv4 := flow.Packet().Add().Ipv4()
		ipv4.Src().SetValue(tc.srcDstPortPair[0].IPv4)
		ipv4.Dst().SetValue(tc.srcDstPortPair[1].IPv4)
		ipPacket = ipv4
	case ipv6:
		ipv6 := flow.Packet().Add().Ipv6()
		ipv6.Src().SetValue(tc.srcDstPortPair[0].IPv6)
		ipv6.Dst().SetValue(tc.srcDstPortPair[1].IPv6)
		ipPacket = ipv6
	default:
		t.Errorf("Invalid traffic type %s", tc.ipType)
	}

	if tc.applyCustomFlow != nil {
		tc.applyCustomFlow(t, config, tc, &flow, &ipPacket)
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

func runTest(t *testing.T, tc testCase, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, config gosnappi.Config) {
	var captureState gosnappi.ControlState
	configureFlows(t, &config, tc)

	if tc.capturePort != "" {
		enableCapture(t, ate, config, []string{tc.capturePort})
		defer clearCapture(t, ate, config)
	}

	otg := ate.OTG()
	otg.PushConfig(t, config)
	otg.StartProtocols(t)

	if tc.capturePort != "" {
		captureState = startCapture(t, ate)
	}

	otg.StartTraffic(t)
	waitForTraffic(t, otg, tc.flowName, testTimeout)
	otg.StopProtocols(t)

	if captureState != nil {
		stopCapture(t, ate, captureState)
		tc.captureFilename = getCapture(t, ate, tc)
	}

	otgutils.LogFlowMetrics(t, otg, config)
	otgutils.LogPortMetrics(t, otg, config)

	if tc.verifyOutput != nil {
		tc.verifyOutput(t, dut, ate, tc)
	}
}

func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")
	dp3 := dut.Port(t, "port3")

	interfaceName := dut.Port(t, "port1").Name()

	t.Logf("Configuring Interfaces")
	configureDUTPort(t, dut, &dutPort1, dp1)
	configureDUTPort(t, dut, &dutPort2, dp2)
	configureDUTPort(t, dut, &dutPort3, dp3)

	configureLoopbackInterface(t, dut)

	t.Logf("Configuring Hardware Init")
	configureHardwareInit(t, dut)

	encapTarget := make(map[string]*oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action_EncapsulateGre_Target)
	width := len(fmt.Sprintf("%d", tunnelCount))
	for index, dest := range tunnelDestinations {
		key := fmt.Sprintf("gre%0*d", width, index+1)
		encapTarget[key] = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action_EncapsulateGre_Target{
			Source:      ygot.String(dutlo0Attrs.IPv4),
			Destination: ygot.String(dest),
		}
	}

	policyRules := []cfgplugins.PolicyForwardingRule{
		{
			Id:                 1,
			Name:               "rule_ipv4_pass",
			IpType:             ipv4,
			DestinationAddress: fmt.Sprintf("%s/%d", otgPort3.IPv4, ipv4PrefixLen),
			Action: &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{
				NextHop: ygot.String(dutPort3.IPv4),
			},
		},
		{
			Id:                 2,
			Name:               "rule_ipv6_pass",
			IpType:             ipv6,
			DestinationAddress: fmt.Sprintf("%s/%d", otgPort3.IPv6, ipv6PrefixLen),
			Action: &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{
				NextHop: ygot.String(dutPort3.IPv6),
			},
		},
		{
			Id:                 3,
			Name:               "rule_ipv4_encap",
			IpType:             ipv4,
			DestinationAddress: fmt.Sprintf("%s/%d", otgPort2.IPv4, ipv4PrefixLen),
			Action: &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{
				EncapsulateGre: &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action_EncapsulateGre{
					Target: encapTarget,
				},
			},
		},
		{
			Id:                 4,
			Name:               "rule_ipv6_encap",
			IpType:             ipv6,
			DestinationAddress: fmt.Sprintf("%s/%d", otgPort2.IPv6, ipv6PrefixLen),
			Action: &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{
				EncapsulateGre: &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action_EncapsulateGre{
					Target: encapTarget,
				},
			},
		},
	}

	t.Logf("Configuring Policy Forwarding")
	_, ni, pf := cfgplugins.SetupPolicyForwardingInfraOC(deviations.DefaultNetworkInstance(dut))
	cfgplugins.NewPolicyForwardingEncapGre(t, dut, pf, trafficPolicyName, interfaceName, nhGroupName, policyRules)
	cfgplugins.ApplyPolicyToInterfaceOC(t, pf, interfaceName, trafficPolicyName)
	if !deviations.PolicyForwardingOCUnsupported(dut) {
		cfgplugins.PushPolicyForwardingConfig(t, dut, ni)
	}

	t.Logf("Configuring QOS")
	ipv6DscpValues := []uint8{}
	for _, dscp := range dscpValues {
		ipv6DscpValues = append(ipv6DscpValues, dscp<<2)
	}
	qos := &oc.Qos{}
	cfgplugins.ConfigureQosClassifierDscpRemark(t, dut, qos, classifierName, interfaceName, dscpValues, ipv6DscpValues)
	cfgplugins.PushQosClassifierToDUT(t, dut, qos, interfaceName, classifierName, true)

	t.Logf("Configuring Static Routes")
	configureStaticRoutes(t, dut)
}

func configureTunnelDestinations() {
	for index := range tunnelCount {
		tunnelDestinations = append(tunnelDestinations, fmt.Sprintf("203.0.113.%d", index+10))
	}
}

func configureHardwareInit(t *testing.T, dut *ondatra.DUTDevice) {
	hardwareInitCfg := cfgplugins.NewDUTHardwareInit(t, dut, cfgplugins.FeaturePolicyForwarding)
	if hardwareInitCfg == "" {
		return
	}
	cfgplugins.PushDUTHardwareInitConfig(t, dut, hardwareInitCfg)
}

func configureDUTPort(t *testing.T, dut *ondatra.DUTDevice, attrs *attrs.Attributes, p *ondatra.Port) {
	t.Helper()
	d := gnmi.OC()
	i := attrs.NewOCInterface(p.Name(), dut)
	gnmi.Replace(t, dut, d.Interface(p.Name()).Config(), i)
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		cfgplugins.AssignToNetworkInstance(t, dut, p.Name(), deviations.DefaultNetworkInstance(dut), 0)
	}
}

func configureLoopbackInterface(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	dc := gnmi.OC()
	loopbackIntfName := netutil.LoopbackInterface(t, dut, 0)
	dutlo0Attrs.Name = loopbackIntfName
	lo0 := gnmi.OC().Interface(loopbackIntfName).Subinterface(0)
	ipv4Addrs := gnmi.LookupAll(t, dut, lo0.Ipv4().AddressAny().State())
	ipv6Addrs := gnmi.LookupAll(t, dut, lo0.Ipv6().AddressAny().State())
	if len(ipv4Addrs) == 0 && len(ipv6Addrs) == 0 {
		loop1 := dutlo0Attrs.NewOCInterface(loopbackIntfName, dut)
		loop1.Type = oc.IETFInterfaces_InterfaceType_softwareLoopback
		gnmi.Update(t, dut, dc.Interface(loopbackIntfName).Config(), loop1)
	} else {
		v4, ok := ipv4Addrs[0].Val()
		if ok {
			dutlo0Attrs.IPv4 = v4.GetIp()
		}
		v6, ok := ipv6Addrs[0].Val()
		if ok {
			dutlo0Attrs.IPv6 = v6.GetIp()
		}
		t.Logf("Got DUT IPv4 loopback address: %v", dutlo0Attrs.IPv4)
		t.Logf("Got DUT IPv6 loopback address: %v", dutlo0Attrs.IPv6)
	}
}

func configureStaticRoutes(t *testing.T, dut *ondatra.DUTDevice) {

	configStaticRoute(t, dut, "192.0.2.0/30", "192.0.2.10", "0")
	for index, dest := range tunnelDestinations {
		configStaticRoute(t, dut, fmt.Sprintf("%s/32", dest), otgPort2.IPv4, strconv.Itoa(index+1))
	}
	configStaticRoute(t, dut, "2001:DB8:0::0/126", "2001:DB8:0::10", "0")
}

func configStaticRoute(t *testing.T, dut *ondatra.DUTDevice, prefix string, nexthop string, index string) {
	b := &gnmi.SetBatch{}
	if nexthop == "Null0" {
		nexthop = "DROP"
	}
	routeCfg := &cfgplugins.StaticRouteCfg{
		NetworkInstance: deviations.DefaultNetworkInstance(dut),
		Prefix:          prefix,
		NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
			index: oc.UnionString(nexthop),
		},
	}
	if _, err := cfgplugins.NewStaticRouteCfg(b, routeCfg, dut); err != nil {
		t.Fatalf("Failed to configure static route: %v", err)
	}
	b.Set(t, dut)
}

func runCliCommand(t *testing.T, dut *ondatra.DUTDevice, cliCommand string) string {
	cliClient := dut.RawAPIs().CLI(t)
	output, err := cliClient.RunCommand(context.Background(), cliCommand)
	if err != nil {
		t.Errorf("Failed to execute CLI command '%s': %v", cliCommand, err)
	}
	t.Logf("Received from cli: %s", output.Output())
	return output.Output()
}

func checkPolicyStatistics(t *testing.T, dut *ondatra.DUTDevice, tc testCase) {
	if deviations.PolicyForwardingGreEncapsulationOcUnsupported(dut) {
		checkPolicyStatisticsFromCLI(t, dut, tc)
	} else {
		checkPolicyStatisticsFromOC(t, dut, tc)
	}
}

func checkPolicyStatisticsFromCLI(t *testing.T, dut *ondatra.DUTDevice, tc testCase) {
	t.Logf("Checking policy statistics for flow %s", tc.flowName)
	switch dut.Vendor() {
	case ondatra.ARISTA:
		//extract text from CLI output between rule name and packets
		policyCountersCommand := fmt.Sprintf(`show traffic-policy %s interface counters | grep %s | sed -e 's/.*%s:\(.*\)packets.*/\1/'`, trafficPolicyName, tc.policyRule, tc.policyRule)
		cliOutput := runCliCommand(t, dut, policyCountersCommand)
		cliOutput = strings.TrimSpace(cliOutput)
		if cliOutput == "" {
			t.Errorf("No output for CLI command '%s'", policyCountersCommand)
			return
		}
		totalMatched, err := strconv.ParseUint(cliOutput, 10, 64)
		if err != nil {
			t.Errorf("Invalid response for CLI command '%s': %v", cliOutput, err)
			return
		}
		previouslyMatched := ruleMatchedPackets[tc.policyRule]
		if totalMatched != previouslyMatched+noOfPackets {
			t.Errorf("Expected %d packets matched by policy %s rule %s for flow %s, but got %d", noOfPackets, trafficPolicyName, tc.policyRule, tc.flowName, totalMatched-previouslyMatched)
		} else {
			t.Logf("%d packets matched by policy %s rule %s for flow %s", totalMatched-previouslyMatched, trafficPolicyName, tc.policyRule, tc.flowName)
		}
		ruleMatchedPackets[tc.policyRule] = totalMatched
	default:
		t.Errorf("Vendor %s is not supported for policy statistics check through CLI", dut.Vendor())
	}
}

func checkPolicyStatisticsFromOC(t *testing.T, dut *ondatra.DUTDevice, tc testCase) {
	totalMatched := gnmi.Get(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding().Policy(trafficPolicyName).Rule(uint32(ruleSequenceMap[tc.policyRule])).MatchedPkts().State())
	previouslyMatched := ruleMatchedPackets[tc.policyRule]
	if totalMatched != previouslyMatched+noOfPackets {
		t.Errorf("Expected %d packets matched by policy %s rule %s for flow %s, but got %d", noOfPackets, trafficPolicyName, tc.policyRule, tc.flowName, totalMatched-previouslyMatched)
	}
	ruleMatchedPackets[tc.policyRule] = totalMatched
}

func enableCapture(t *testing.T, ate *ondatra.ATEDevice, topo gosnappi.Config, otgPortNames []string) {
	t.Helper()
	for _, port := range otgPortNames {
		t.Log("Enabling capture on ", port)
		topo.Captures().Add().SetName(port).SetPortNames([]string{port}).SetFormat(gosnappi.CaptureFormat.PCAP)
	}

	pb, _ := topo.Marshal().ToProto()
	t.Log(pb.GetCaptures())
	ate.OTG().PushConfig(t, topo)
}

func clearCapture(t *testing.T, ate *ondatra.ATEDevice, topo gosnappi.Config) {
	t.Helper()
	topo.Captures().Clear()
	ate.OTG().PushConfig(t, topo)
}

func startCapture(t *testing.T, ate *ondatra.ATEDevice) gosnappi.ControlState {
	t.Helper()
	cs := gosnappi.NewControlState()
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.START)
	ate.OTG().SetControlState(t, cs)
	return cs
}

func getCapture(t *testing.T, ate *ondatra.ATEDevice, tc testCase) string {
	otg := ate.OTG()
	bytes := otg.GetCapture(t, gosnappi.NewCaptureRequest().SetPortName(tc.capturePort))
	if len(bytes) == 0 {
		t.Errorf("Empty capture received for flow %s on port %s", tc.flowName, tc.capturePort)
		return ""
	}
	f, err := os.Create(captureFilePath)
	if err != nil {
		t.Errorf("Could not create temporary pcap file: %v\n", err)
		return ""
	}
	defer f.Close()
	if _, err := f.Write(bytes); err != nil {
		t.Errorf("Could not write bytes to pcap file: %v\n", err)
		return ""
	}

	return f.Name()
}

func stopCapture(t *testing.T, ate *ondatra.ATEDevice, cs gosnappi.ControlState) {
	t.Helper()
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.STOP)
	ate.OTG().SetControlState(t, cs)
	time.Sleep(5 * time.Second)
}

func checkGreCapture(t *testing.T, tc testCase) {
	if tc.captureFilename == "" {
		t.Errorf("Capture file not found for flow %s", tc.flowName)
		return
	}
	handle, err := pcap.OpenOffline(tc.captureFilename)
	if err != nil {
		t.Error(err)
		return
	}
	defer handle.Close()

	var innerLayerType gopacket.LayerType
	switch tc.ipType {
	case ipv4:
		innerLayerType = layers.LayerTypeIPv4
	case ipv6:
		innerLayerType = layers.LayerTypeIPv6
	}
	var packetCount uint32 = 0
	var variation uint32 = tunnelCount / 10
	tunnelPackets := make(map[string]uint32)
	dscpPackets := make(map[uint8]uint32)
	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	for packet := range packetSource.Packets() {
		ipLayer := packet.Layer(layers.LayerTypeIPv4)
		if ipLayer == nil {
			continue
		}
		sourceDscp := (uint8)(dscpValues[int(packetCount)%len(dscpValues)])
		packetCount += 1
		ipOuterLayer, ok := ipLayer.(*layers.IPv4)
		if !ok || ipOuterLayer == nil {
			t.Errorf("Outer IP layer not found %d", ipLayer)
			return
		}
		greLayer := packet.Layer(layers.LayerTypeGRE)
		grePacket, ok := greLayer.(*layers.GRE)
		if !ok || grePacket == nil {
			t.Error("GRE layer not found")
			return
		}
		if ipOuterLayer.Protocol != greProtocol {
			t.Errorf("Packet is not encapslated properly. Encapsulated protocol is: %d", ipOuterLayer.Protocol)
			return
		}
		innerPacket := gopacket.NewPacket(grePacket.Payload, grePacket.NextLayerType(), gopacket.Default)
		ipInnerLayer := innerPacket.Layer(innerLayerType)
		if ipInnerLayer == nil {
			t.Error("Inner IP layer not found")
			return
		}
		var innerPacketTOS, dscp uint8
		switch tc.ipType {
		case ipv4:
			ipInnerPacket, ok := ipInnerLayer.(*layers.IPv4)
			if !ok || ipInnerPacket == nil {
				t.Errorf("Inner layer of type %s not found", innerLayerType.String())
				return
			}
			innerPacketTOS = ipInnerPacket.TOS
			dscp = innerPacketTOS >> 2
		case ipv6:
			ipInnerPacket, ok := ipInnerLayer.(*layers.IPv6)
			if !ok || ipInnerPacket == nil {
				t.Errorf("Inner layer of type %s not found", innerLayerType.String())
				return
			}
			innerPacketTOS = ipInnerPacket.TrafficClass
			dscp = innerPacketTOS
			sourceDscp = sourceDscp << 2
		}

		tunnelPackets[ipOuterLayer.DstIP.String()] += 1
		if tc.checkEncapDscp {
			if ipOuterLayer.TOS != innerPacketTOS {
				t.Errorf("DSCP mismatch: outer DSCP %d, inner DSCP %d", ipOuterLayer.TOS, innerPacketTOS)
			}
			if dscp != sourceDscp {
				t.Errorf("DSCP mismatch: source DSCP %d, received DSCP %d", sourceDscp, dscp)
			}
			dscpPackets[dscp] += 1
		}
	}
	if packetCount < noOfPackets {
		t.Errorf("Received %d gre packets, expecting more than %d gre packets", packetCount, noOfPackets)
	}
	if tc.checkEncapLoadBalanced {
		if len(tunnelPackets) != tunnelCount {
			t.Errorf("Expected %d, tunnels, actually %d tunnels", tunnelCount, len(tunnelPackets))
		}
		var aproxPacketsPerTunnel uint32 = noOfPackets / tunnelCount
		for dest, count := range tunnelPackets {
			t.Logf("Destination %s, count %d", dest, count)
			if count < aproxPacketsPerTunnel-variation || count > aproxPacketsPerTunnel+variation {
				t.Errorf("Expected aprox %d packets for tunnel %s, received %d", aproxPacketsPerTunnel, dest, count)
			}
		}
	}
	for dscp, count := range dscpPackets {
		t.Logf("Packets with dscp %d: %d", dscp, count)
	}
}

func checkFlowStats(t *testing.T, ate *ondatra.ATEDevice, tc testCase) {
	otg := ate.OTG()
	flowMetrics := gnmi.Get(t, otg, gnmi.OTG().Flow(tc.flowName).State())
	if *flowMetrics.Counters.OutPkts != uint64(noOfPackets) {
		t.Errorf("Expected %d frames to be transmitted, but got %d", noOfPackets, flowMetrics.Counters.OutPkts)
	}
	if *flowMetrics.Counters.InPkts != uint64(noOfPackets) {
		t.Errorf("Expected %x frames to be received, but got %d", noOfPackets, flowMetrics.Counters.InPkts)
	}
}
