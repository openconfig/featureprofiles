package encap_gre_ipv4_test

import (
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/featureprofiles/internal/qoscfg"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
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

	IPv4 = "IPv4"
	IPv6 = "IPv6"
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
	applyCustomFlow        func(t *testing.T, top gosnappi.Config, tc testCase, flow *gosnappi.Flow, packet *ipFlow)
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
			ipType:         IPv4,
			capturePort:    "port2",
			srcDstPortPair: []attrs.Attributes{otgPort1, otgPort2},
			applyCustomFlow: func(t *testing.T, top gosnappi.Config, tc testCase, flow *gosnappi.Flow, packet *ipFlow) {
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
			ipType:         IPv6,
			capturePort:    "port2",
			srcDstPortPair: []attrs.Attributes{otgPort1, otgPort2},
			applyCustomFlow: func(t *testing.T, top gosnappi.Config, tc testCase, flow *gosnappi.Flow, packet *ipFlow) {
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
			ipType:          IPv4,
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
			ipType:          IPv6,
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
			ipType:         IPv4,
			capturePort:    "port2",
			srcDstPortPair: []attrs.Attributes{otgPort1, otgPort2},
			applyCustomFlow: func(t *testing.T, top gosnappi.Config, tc testCase, flow *gosnappi.Flow, packet *ipFlow) {
				foundIpv4, ok := (*packet).(gosnappi.FlowIpv4)
				if !ok || foundIpv4 == nil {
					return
				}
				foundIpv4.Priority().Dscp().Phb().Increment().SetStart(0).SetStep(8).SetCount(8)
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
			ipType:         IPv6,
			capturePort:    "port2",
			srcDstPortPair: []attrs.Attributes{otgPort1, otgPort2},
			applyCustomFlow: func(t *testing.T, top gosnappi.Config, tc testCase, flow *gosnappi.Flow, packet *ipFlow) {
				foundIpv6, ok := (*packet).(gosnappi.FlowIpv6)
				if !ok || foundIpv6 == nil {
					return
				}
				foundIpv6.TrafficClass().Increment().SetStart(0).SetStep(32).SetCount(8)
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
		{
			name:           "PF-1.1.7.1: Verify MTU handling during GRE encap for IPv4 traffic",
			ipType:         IPv4,
			capturePort:    "port1",
			srcDstPortPair: []attrs.Attributes{otgPort1, otgPort2},
			applyCustomFlow: func(t *testing.T, top gosnappi.Config, tc testCase, flow *gosnappi.Flow, packet *ipFlow) {
				(*flow).Size().SetFixed(4000)
				foundIpv4, ok := (*packet).(gosnappi.FlowIpv4)
				if !ok || foundIpv4 == nil {
					return
				}
				foundIpv4.DontFragment().SetValue(1)
			},
			verifyOutput: func(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, tc testCase) {
				checkFragmentationNeeded(t, tc)
				checkPolicyStatistics(t, dut, tc)
			},
			checkEncapDscp:         false,
			checkEncapLoadBalanced: true,
			flowName:               "FlowTC71",
		},
		{
			name:           "PF-1.1.7.2: Verify MTU handling during GRE encap for IPv6 traffic",
			ipType:         IPv6,
			capturePort:    "port1",
			srcDstPortPair: []attrs.Attributes{otgPort1, otgPort2},
			applyCustomFlow: func(t *testing.T, top gosnappi.Config, tc testCase, flow *gosnappi.Flow, packet *ipFlow) {
				(*flow).Size().SetFixed(4000)
			},
			verifyOutput: func(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, tc testCase) {
				checkFragmentationNeeded(t, tc)
				checkPolicyStatistics(t, dut, tc)
			},
			checkEncapDscp:         false,
			checkEncapLoadBalanced: true,
			flowName:               "FlowTC72",
		},
	}

	//for _, tc := range testCases {
	for _, tc := range []testCase{testCases[6]} {
		t.Run(tc.name, func(t *testing.T) {
			runTest(t, tc, dut, ate, top)
		})
	}
}

func runTest(t *testing.T, tc testCase, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, config gosnappi.Config) {
	var captureCS gosnappi.ControlState
	otg := ate.OTG()
	config.Flows().Clear()
	if tc.capturePort != "" {
		enableCapture(t, ate, config, []string{tc.capturePort})
		defer clearCapture(t, ate, config)
	}
	flow := config.Flows().Add().SetName(tc.flowName)
	flow.Metrics().SetEnable(true)
	flow.TxRx().Device().SetTxNames([]string{fmt.Sprintf("%s.%s", tc.srcDstPortPair[0].Name, tc.ipType)}).SetRxNames([]string{fmt.Sprintf("%s.%s", tc.srcDstPortPair[1].Name, tc.ipType)})
	flow.Size().SetFixed(trafficFrameSize)
	flow.Rate().SetPps(trafficRatePps)
	flow.Duration().SetFixedPackets(gosnappi.NewFlowFixedPackets().SetPackets(noOfPackets))

	eth := flow.Packet().Add().Ethernet()
	eth.Src().SetValue(tc.srcDstPortPair[0].MAC)
	var ipPacket ipFlow
	switch tc.ipType {
	case IPv4:
		ipv4 := flow.Packet().Add().Ipv4()
		ipv4.Src().SetValue(tc.srcDstPortPair[0].IPv4)
		ipv4.Dst().SetValue(tc.srcDstPortPair[1].IPv4)
		ipPacket = ipv4
	case IPv6:
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

	otg.PushConfig(t, config)
	otg.StartProtocols(t)

	if tc.capturePort != "" {
		captureCS = startCapture(t, ate)
	}

	otg.StartTraffic(t)
	time.Sleep(5 * time.Second)
	otg.StopTraffic(t)
	time.Sleep(5 * time.Second)

	otg.StopProtocols(t)

	if captureCS != nil {
		stopCapture(t, ate, captureCS)
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

	t.Logf("Configuring Interfaces")
	configureDUTPort(t, dut, &dutPort1, dp1)
	configureDUTPort(t, dut, &dutPort2, dp2)
	configureDUTPort(t, dut, &dutPort3, dp3)

	configureLoopbackInterface(t, dut)

	t.Logf("Configuring TCAM Profile")
	configureTcamProfile(t, dut)

	t.Logf("Configuring Policy Forwarding")
	configurePolicyForwarding(t, dut)

	t.Logf("Configuring QOS")
	configureQoSClassifier(t, dut)

	t.Logf("Configuring Static Routes")
	configureStaticRoutes(t, dut)
}

func configureTunnelDestinations() {
	for index := range tunnelCount {
		tunnelDestinations = append(tunnelDestinations, fmt.Sprintf("203.0.113.%d", index+10))
	}
}

func configureDUTPort(t *testing.T, dut *ondatra.DUTDevice, attrs *attrs.Attributes, p *ondatra.Port) {
	t.Helper()
	d := gnmi.OC()
	i := attrs.NewOCInterface(p.Name(), dut)
	i.Description = ygot.String(attrs.Desc)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	if deviations.InterfaceEnabled(dut) {
		i.Enabled = ygot.Bool(true)
	}

	i.GetOrCreateEthernet()
	i4 := i.GetOrCreateSubinterface(0).GetOrCreateIpv4()
	i4.Enabled = ygot.Bool(true)
	a := i4.GetOrCreateAddress(attrs.IPv4)
	a.PrefixLength = ygot.Uint8(attrs.IPv4Len)

	i6 := i.GetOrCreateSubinterface(0).GetOrCreateIpv6()
	i6.Enabled = ygot.Bool(true)
	a6 := i6.GetOrCreateAddress(attrs.IPv6)
	a6.PrefixLength = ygot.Uint8(attrs.IPv6Len)

	gnmi.Replace(t, dut, d.Interface(p.Name()).Config(), i)
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, p.Name(), deviations.DefaultNetworkInstance(dut), 0)
		t.Logf("DUT %s %s %s requires explicit interface in default VRF deviation ", dut.Vendor(), dut.Model(), dut.Version())
	}

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, p)
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

func configureTcamProfile(t *testing.T, dut *ondatra.DUTDevice) {
	tcamProfileConfig := `
    hardware tcam
   profile tcam-test
      feature traffic-policy port ipv4
         sequence 45
         key size limit 160
         key field dscp dst-ip-label ip-frag ip-fragment-offset ip-length ip-protocol l4-dst-port-label l4-src-port-label src-ip-label tcp-control ttl
         action count drop redirect set-dscp set-tc
         packet ipv4 forwarding routed
      !
      feature traffic-policy port ipv6
         sequence 25
         key size limit 160
         key field dst-ipv6-label hop-limit ipv6-length ipv6-next-header ipv6-traffic-class l4-dst-port-label l4-src-port-label src-ipv6-label tcp-control
         action count drop redirect set-dscp set-tc
         packet ipv6 forwarding routed
      !
   system profile tcam-test
    !
    hardware counter feature gre tunnel interface out
    !
    hardware counter feature traffic-policy in
    !
    hardware counter feature traffic-policy out
    !
    hardware counter feature route ipv4
    !
    hardware counter feature nexthop
    !
    `
	gnmiClient := dut.RawAPIs().GNMI(t)
	t.Logf("Push the CLI Qos config:%s", dut.Vendor())
	gpbSetRequest := buildCliSetRequest(tcamProfileConfig)
	if _, err := gnmiClient.Set(context.Background(), gpbSetRequest); err != nil {
		t.Fatalf("gnmiClient.Set() with unexpected error: %v", err)
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
	ni := oc.NetworkInstance{Name: ygot.String(deviations.DefaultNetworkInstance(dut))}
	static := ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
	static.SetEnabled(true)
	sr := static.GetOrCreateStatic(prefix)
	nh := sr.GetOrCreateNextHop(index)
	nh.NextHop = oc.UnionString(nexthop)
	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut)).Config(), static)
}

func buildCliSetRequest(config string) *gpb.SetRequest {
	gpbSetRequest := &gpb.SetRequest{
		Update: []*gpb.Update{
			{
				Path: &gpb.Path{
					Origin: "cli",
					Elem:   []*gpb.PathElem{},
				},
				Val: &gpb.TypedValue{
					Value: &gpb.TypedValue_AsciiVal{
						AsciiVal: config,
					},
				},
			},
		},
	}
	return gpbSetRequest
}

func buildCliGetRequest(cliCommand string) *gpb.GetRequest {
	// Build the gNMI GetRequest for the CLI command
	gpbGetRequest := &gpb.GetRequest{
		Path: []*gpb.Path{
			{
				Origin: "cli",
				Elem: []*gpb.PathElem{
					{
						Name: cliCommand,
					},
				},
			},
		},
	}

	return gpbGetRequest
}

func configurePolicyForwarding(t *testing.T, dut *ondatra.DUTDevice) {
	interfaceName := dut.Port(t, "port1").Name()
	if deviations.PolicyForwardingToNextHopUnsupported(dut) || deviations.PolicyForwardingGREEncapsulationUnsupported(dut) {
		t.Logf("Configuring pf through CLI")
		configurePolicyForwardingFromCLI(t, dut, trafficPolicyName, interfaceName)
	} else {
		t.Logf("Configuring pf through OC")
		configurePolicyForwardingFromOC(t, dut, trafficPolicyName, interfaceName)
	}
}

func configurePolicyForwardingFromCLI(t *testing.T, dut *ondatra.DUTDevice, policyName string, interfaceName string) {
	gnmiClient := dut.RawAPIs().GNMI(t)
	tpConfig := trafficPolicyCliConfig(dut, policyName, interfaceName)
	t.Logf("Push the CLI Policy config:%s", dut.Vendor())
	gpbSetRequest := buildCliSetRequest(tpConfig)
	if _, err := gnmiClient.Set(context.Background(), gpbSetRequest); err != nil {
		t.Fatalf("gnmiClient.Set() with unexpected error: %v", err)
	}
}

func configurePolicyForwardingFromOC(t *testing.T, dut *ondatra.DUTDevice, policyName string, interfaceName string) {
	pf := &oc.Root{}
	ni := pf.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	policy := ni.GetOrCreatePolicyForwarding().GetOrCreatePolicy(policyName)
	policy.Type = oc.Policy_Type_PBR_POLICY
	// Rule 1: Match IPV4-SRC1 and accept/forward
	rule1 := policy.GetOrCreateRule(1)
	rule1.GetOrCreateTransport()
	rule1.GetOrCreateIpv4().DestinationAddress = ygot.String("192.0.2.4/30")
	rule1.GetOrCreateAction().SetNextHop(dutPort3.IPv4)

	// Rule 2: Match IPV6-SRC1 and accept/forward
	rule2 := policy.GetOrCreateRule(2)
	rule2.GetOrCreateIpv6().DestinationAddress = ygot.String("2001:db8::6/128")
	rule2.GetOrCreateAction().SetNextHop(dutPort3.IPv6)

	// Rule 3: Match IPV4-SRC2 and encapsulate to 32 IPv4 GRE destinations
	rule3 := policy.GetOrCreateRule(3)
	rule3.GetOrCreateIpv4().DestinationAddress = ygot.String("192.0.2.8/30")
	encapGre3 := rule3.GetOrCreateAction().GetOrCreateEncapsulateGre()
	for i, dest := range tunnelDestinations {
		targetName := fmt.Sprintf("gre%d", i)
		encapGre3.GetOrCreateTarget(targetName).Source = ygot.String(dutlo0Attrs.IPv4)
		encapGre3.GetOrCreateTarget(targetName).Destination = ygot.String(dest)
	}

	// Rule 4: Match IPV6-SRC2 and encapsulate to 32 IPv4 GRE destinations
	rule4 := policy.GetOrCreateRule(4)
	rule4.GetOrCreateIpv6().DestinationAddress = ygot.String("2001:db8::a/128")
	encapGre4 := rule4.GetOrCreateAction().GetOrCreateEncapsulateGre()
	for i, dest := range tunnelDestinations {
		targetName := fmt.Sprintf("gre%d", i)
		encapGre4.GetOrCreateTarget(targetName).Source = ygot.String(dutlo0Attrs.IPv4)
		encapGre4.GetOrCreateTarget(targetName).Destination = ygot.String(dest)
	}

	// Apply the policy to DUT Port 1
	ni.GetOrCreatePolicyForwarding().GetOrCreateInterface(interfaceName).ApplyForwardingPolicy = ygot.String(policyName)
	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding().Config(), ni.PolicyForwarding)
}

// ConfigureQoSClassifier configures QoS classifier for incoming traffic on ATE Port1.
func configureQoSClassifier(t *testing.T, dut *ondatra.DUTDevice) {
	interfaceName := dut.Port(t, "port1").Name()
	classifierName := "qos-classifier-1"
	ipv4DscpValues := []uint8{0, 8, 16, 24, 32, 40, 48, 56}
	ipv6DscpValues := []uint8{0, 32, 64, 96, 128, 160, 192, 224}
	if deviations.QosClassifierDscpRemarkUnsupported(dut) {
		t.Logf("Configuring qos through CLI")
		configureQoSClassifierFromCLI(t, dut, classifierName, interfaceName, ipv4DscpValues, ipv6DscpValues)
	} else {
		t.Logf("Configuring qos through OC")
		configureQoSClassifierFromOC(t, dut, classifierName, interfaceName, ipv4DscpValues, ipv6DscpValues)
	}
}

func configureQoSClassifierFromCLI(t *testing.T, dut *ondatra.DUTDevice, classifierName string, interfaceName string, ipv4DscpValues []uint8, ipv6DscpValues []uint8) {
	gnmiClient := dut.RawAPIs().GNMI(t)
	qosConfig := qosClassifierCliConfig(dut, classifierName, interfaceName, ipv4DscpValues, ipv6DscpValues)
	t.Logf("Push the CLI Qos config:%s", dut.Vendor())
	gpbSetRequest := buildCliSetRequest(qosConfig)
	if _, err := gnmiClient.Set(context.Background(), gpbSetRequest); err != nil {
		t.Fatalf("gnmiClient.Set() with unexpected error: %v", err)
	}
}

func configureQoSClassifierFromOC(t *testing.T, dut *ondatra.DUTDevice, classifierName string, interfaceName string, ipv4DscpValues []uint8, ipv6DscpValues []uint8) {
	qos := &oc.Qos{}
	classifier := qos.GetOrCreateClassifier(classifierName)
	classifier.SetType(oc.Qos_Classifier_Type_IPV4)
	classifier.SetName(classifierName)

	// Create terms for IPv4 DSCP values
	for i, dscp := range ipv4DscpValues {
		term := classifier.GetOrCreateTerm(fmt.Sprintf("termV4-%d", i+1))
		term.Id = ygot.String(fmt.Sprintf("termV4-%d", i+1))
		term.GetOrCreateConditions().GetOrCreateIpv4().DscpSet = []uint8{dscp}
		term.GetOrCreateActions().GetOrCreateRemark().SetSetDscp(5)
		term.GetOrCreateActions().GetOrCreateRemark().SetDscp = ygot.Uint8(dscp)
	}

	// Create terms for IPv6 DSCP values
	for i, dscp := range ipv6DscpValues {
		term := classifier.GetOrCreateTerm(fmt.Sprintf("termV6-%d", i+1))
		term.Id = ygot.String(fmt.Sprintf("termV6-%d", i+1))
		term.GetOrCreateConditions().GetOrCreateIpv6().DscpSet = []uint8{dscp}
		term.GetOrCreateActions().GetOrCreateRemark().SetDscp = ygot.Uint8(dscp)
	}

	gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), qos)
	qoscfg.SetInputClassifier(t, dut, qos, interfaceName, oc.Input_Classifier_Type_IPV4, classifier.GetName())
}

func qosClassifierCliConfig(dut *ondatra.DUTDevice, classifierName string, interfaceName string, ipv4DscpValues []uint8, ipv6DscpValues []uint8) string {
	switch dut.Vendor() {
	case ondatra.ARISTA:
		return `
        qos rewrite dscp
        !
        `
		// cliConfig := fmt.Sprintf("qos map dscp-classifier %s\n", classifierName)
		// for i, dscp := range ipv4DscpValues {
		//  cliConfig += fmt.Sprintf(`
		//  dscp %d class termV4-%d
		//  qos map dscp-remark remarkV4-%d
		//  match dscp %d
		//  set dscp %d
		//  !
		//  `, dscp, i+1, i+1, dscp, dscp)
		// }

		// // Add terms for IPv6 DSCP values
		// for i, dscp := range ipv6DscpValues {
		//  cliConfig += fmt.Sprintf(`
		//  dscp %d class termV6-%d
		//  qos map dscp-remark remarkV6-%d
		//  match dscp %d
		//  set dscp %d
		//  !
		//  `, dscp, i+1, i+1, dscp, dscp)
		// }

		// // Apply the classifier to the interface
		// cliConfig += fmt.Sprintf(`
		// interface %s
		// service-policy input %s
		// !
		// `, interfaceName, classifierName)

		//return cliConfig
	default:
		return ""
	}
}

func trafficPolicyCliConfig(dut *ondatra.DUTDevice, policyName string, interfaceName string) string {
	switch dut.Vendor() {
	case ondatra.ARISTA:
		var v4MatchRules, v6MatchRules string
		v4MatchRules += fmt.Sprintf(`
        match rule-src1-v4 ipv4
        destination prefix %s
        actions
        count
        redirect next-hop group SRC1_NH
        !
        `, "192.0.2.4 255.255.255.252")

		v4MatchRules += fmt.Sprintf(`
        match rule-src2-v4 ipv4
        destination prefix %s
        actions
        count
        !
        `, "192.0.2.8 255.255.255.252")

		v6MatchRules += fmt.Sprintf(`
        match rule-src1-v6 ipv6
        destination prefix %s/128
        actions
        count
        redirect next-hop group SRC1_NH
        !
        `, otgPort2.IPv6)

		v6MatchRules += fmt.Sprintf(`
        match rule-src2-v6 ipv6
        destination prefix %s/128
        actions
        count
        !
        `, otgPort3.IPv6)

		ipv4GreNH := fmt.Sprintf(`
        nexthop-group SRC1_NH type gre
        tunnel-source intf %s
        `, dutlo0Attrs.Name)

		for index, dest := range tunnelDestinations {
			ipv4GreNH += fmt.Sprintf(`
            entry %d tunnel-destination %s
            `, index, dest)
		}

		// Apply Policy on the interface
		trafficPolicyConfig := fmt.Sprintf(`
            traffic-policies
            traffic-policy %s
            %s
            %s
            %s
            !
            interface %s
            traffic-policy input %s
            `, policyName, v4MatchRules, v6MatchRules, ipv4GreNH, interfaceName, trafficPolicyName)
		return trafficPolicyConfig
	default:
		return ""
	}
}

func checkPolicyStatistics(t *testing.T, dut *ondatra.DUTDevice, tc testCase) {
	if deviations.PolicyForwardingGREEncapsulationUnsupported(dut) {
		checkPolicyStatisticsFromCLI(t, dut, tc)
	} else {
		checkPolicyStatisticsFromOC(t, dut, tc)
	}
}

func checkPolicyStatisticsFromCLI(t *testing.T, dut *ondatra.DUTDevice, tc testCase) {
	//TODO get output from cli command
	return
	// t.Logf("Checking policy statistics for flow %s", tc.flowName)
	// gnmiClient := dut.RawAPIs().GNMI(t)
	// //| %s| sed -e 's/.*%s:\(.*\)packets.*/\1/' | xargs
	// policyCountersCommand := fmt.Sprintf("show traffic-policy %s interface counters", trafficPolicyName)
	// gpbGetRequest := buildCliGetRequest(policyCountersCommand)
	// response, err := gnmiClient.Get(context.Background(), gpbGetRequest)
	// if err != nil {
	// 	t.Fatalf("gnmiClient.Set() with unexpected error: %v", err)
	// }
	// if len(response.Notification) == 0 || len(response.Notification[0].Update) == 0 {
	// 	t.Fatalf("No output received for CLI command '%s'", policyCountersCommand)
	// }
	// cliOutput := response.Notification[0].Update[0].Val.GetAsciiVal()
	// t.Logf("Received from cli %s", cliOutput)
	// totalMatched, err := strconv.ParseUint(cliOutput, 10, 64)
	// if err != nil {
	// 	t.Fatalf("Invalid response for CLI command '%s': %v", policyCountersCommand, err)
	// }
	// previouslyMatched := ruleMatchedPackets[tc.policyRule]
	// if totalMatched != previouslyMatched+noOfPackets {
	// 	t.Errorf("Expected %d packets matched by policy %s for flow %s, but got %d", noOfPackets, tc.policyRule, tc.flowName, totalMatched-previouslyMatched)
	// } else {
	// 	t.Logf("%d packets matched by policy %s for flow %s", totalMatched-previouslyMatched, trafficPolicyName, tc.flowName)
	// }
	// ruleMatchedPackets[tc.policyRule] = totalMatched
}

func checkPolicyStatisticsFromOC(t *testing.T, dut *ondatra.DUTDevice, tc testCase) {
	///network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/state/matched-pkts:
	totalMatched := gnmi.Get(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding().Policy(trafficPolicyName).Rule(uint32(ruleSequenceMap[tc.policyRule])).MatchedPkts().State())
	previouslyMatched := ruleMatchedPackets[tc.policyRule]
	if totalMatched != previouslyMatched+noOfPackets {
		t.Errorf("Expected %d packets matched by policy %s for flow %s, but got %d", noOfPackets, tc.policyRule, tc.flowName, totalMatched-previouslyMatched)
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
	f, err := os.Create(captureFilePath)
	if err != nil {
		t.Fatalf("ERROR: Could not create temporary pcap file: %v\n", err)
	}
	if _, err := f.Write(bytes); err != nil {
		t.Fatalf("ERROR: Could not write bytes to pcap file: %v\n", err)
	}
	f.Close()
	return f.Name()
}

func stopCapture(t *testing.T, ate *ondatra.ATEDevice, cs gosnappi.ControlState) {
	t.Helper()
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.STOP)
	ate.OTG().SetControlState(t, cs)
	time.Sleep(5 * time.Second)
}

func checkGreCapture(t *testing.T, tc testCase) {
	handle, err := pcap.OpenOffline(tc.captureFilename)
	if err != nil {
		t.Error(err)
	}
	defer handle.Close()

	var innerLayerType gopacket.LayerType
	switch tc.ipType {
	case IPv4:
		innerLayerType = layers.LayerTypeIPv4
	case IPv6:
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
		packetCount += 1
		ipOuterLayer, ok := ipLayer.(*layers.IPv4)
		if !ok || ipOuterLayer == nil {
			t.Errorf("Outer IP layer not found %d", ipLayer)
		}
		greLayer := packet.Layer(layers.LayerTypeGRE)
		grePacket, ok := greLayer.(*layers.GRE)
		if !ok || grePacket == nil {
			t.Errorf("GRE layer not found %d", greLayer)
		}
		if ipOuterLayer.Protocol != greProtocol {
			t.Errorf("Packet is not encapslated properly. Encapsulated protocol is: %d", ipOuterLayer.Protocol)
		}
		innerPacket := gopacket.NewPacket(grePacket.Payload, grePacket.NextLayerType(), gopacket.Default)
		ipInnerLayer := innerPacket.Layer(innerLayerType)
		if ipInnerLayer == nil {
			t.Error("Inner IP layer not found")
		}
		var innerPacketTOS uint8
		switch tc.ipType {
		case IPv4:
			ipInnerPacket, ok := ipInnerLayer.(*layers.IPv4)
			if !ok || ipInnerPacket == nil {
				t.Errorf("Inner layer of type %s not found", innerLayerType.String())
			}
			innerPacketTOS = ipInnerPacket.TOS
		case IPv6:
			ipInnerPacket, ok := ipInnerLayer.(*layers.IPv6)
			if !ok || ipInnerPacket == nil {
				t.Errorf("Inner layer of type %s not found", innerLayerType.String())
			}
			innerPacketTOS = ipInnerPacket.TrafficClass
		}

		tunnelPackets[ipOuterLayer.DstIP.String()] += 1
		if tc.checkEncapDscp {
			if ipOuterLayer.TOS != innerPacketTOS {
				t.Errorf("DSCP mismatch: outer DSCP %d, inner DSCP %d", ipOuterLayer.TOS, innerPacketTOS)
			}
		}
		dscpPackets[innerPacketTOS] += 1
	}
	if packetCount < noOfPackets {
		t.Errorf("Received %d, expecting more than %d packets", packetCount, noOfPackets)
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

func checkFragmentationNeeded(t *testing.T, tc testCase) {
	handle, err := pcap.OpenOffline(tc.captureFilename)
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close()

	var notFoundMessage string
	var icmpType gopacket.LayerType
	switch tc.ipType {
	case IPv4:
		icmpType = layers.LayerTypeICMPv4
		notFoundMessage = "No ICMP Type 3 Code 4 packets found"
	case IPv6:
		icmpType = layers.LayerTypeICMPv6
		notFoundMessage = "No ICMPv6 Packet Too Big messages found"
	}
	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())

	found := false
	for packet := range packetSource.Packets() {
		icmpLayer := packet.Layer(icmpType)
		if icmpLayer != nil {
			switch tc.ipType {
			case IPv4:
				icmpPacket, ok := icmpLayer.(*layers.ICMPv4)
				if !ok || icmpPacket == nil {
					continue
				}
				if icmpPacket.TypeCode == layers.CreateICMPv4TypeCode(3, 4) {
					found = true
					mtu := binary.BigEndian.Uint16(icmpPacket.Payload[:2])
					t.Logf("Found ICMP Type 3 Code 4 (Fragmentation Needed) message with MTU: %d", mtu)
					return
				}
			case IPv6:
				icmpv6Packet, ok := icmpLayer.(*layers.ICMPv6)
				if !ok || icmpv6Packet == nil {
					continue
				}

				if icmpv6Packet.TypeCode.Type() == layers.ICMPv6TypePacketTooBig {
					found = true
					mtu := binary.BigEndian.Uint32(icmpv6Packet.Payload[:4])
					t.Logf("Found ICMPv6 Packet Too Big message with MTU: %d", mtu)
					return
				}
			}

		}
	}

	if !found {
		t.Error(notFoundMessage)
	}
}
