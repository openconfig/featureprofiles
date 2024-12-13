package dcgate_decap_aft_nh_counters

import (
	"context"
	"slices"
	"strconv"
	"testing"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
)

const (
	nh1ID                     = 120
	nhg1ID                    = 20
	ipv4OuterDest             = "192.51.100.65"
	innerV4DstIP              = "198.18.1.1"
	innerV4SrcIP              = "198.18.0.255"
	innerV6SrcIP              = "2001:DB8::198:1"
	innerV6DstIP              = "2001:DB8:2:0:192::10"
	transitVrfIP              = "203.0.113.1"
	repairedVrfIP             = "203.0.113.100"
	noMatchSrcIP              = "198.100.200.123"
	decapMixPrefix1           = "192.51.128.0/22"
	decapMixPrefix2           = "192.55.200.3/32"
	src111TeDstFlowFilter     = "4043" // Egress tracking flow filter decimal value for first 4 bits of last octet of SA 198.51.100.111 + First 8 bits of first octet of TE DA 203.0.113.1
	src222TeDstFlowFilter     = "3787" // Egress tracking flow filter decimal value for first 4 bits of last octet of SA 198.51.100.222 + First 8 bits of first octet of TE DA 203.0.113.100
	noMatchSrcEncapDstFilter  = "2954" // Egress tracking flow filter decimal value for first 4 bits of last octet of SA 198.100.200.123 + First 8 bits of first octet of TE DA 138.0.11.8
	IPinIPProtocolFieldOffset = 184
	IPinIPProtocolFieldWidth  = 8
	IPinIPpSrcDstIPOffset     = 236
	IPinIPpSrcDstIPWidth      = 12
	IPinIPpDscpOffset         = 120
	IPinIPpDscpWidth          = 8
)

var prefixLengthVariation = []struct {
	name   string
	prefix string
}{
	{
		name:   "24 prefix length",
		prefix: "192.51.100.0/24",
	},
	{
		name:   "22 prefix length",
		prefix: "192.51.100.0/22",
	},
	{
		name:   "28 prefix length",
		prefix: "192.51.100.64/28",
	},
	{
		name:   "32 prefix length",
		prefix: "192.51.100.65/32",
	},
}

var defaultDstPort = []string{atePort8.Name}

var (
	//For IPinIP traffic flows with egress tracking IPv4 protocol next header field
	flow1V4 = trafficflowAttr{
		innerSrcStart:        innerSrcIPv4Start,
		innerdstStart:        innerDstIPv4Start,
		innerFlowCount:       flowCount,
		egressTrackingOffset: IPinIPProtocolFieldOffset,
		egressTrackingWidth:  IPinIPProtocolFieldWidth,
	}
	//For IPv6inIP traffic flows with egress tracking IPv4 protocol next header field
	flow1V6 = trafficflowAttr{
		innerSrcStart:        innerSrcIPv6Start,
		innerdstStart:        innerDstIPv6Start,
		innerFlowCount:       flowCount,
		egressTrackingOffset: IPinIPProtocolFieldOffset,
		egressTrackingWidth:  IPinIPProtocolFieldWidth,
	}
	//For IPinIP traffic flows with egress tracking 1st 4 bits of last octet of SA + 1st 8 bits of 1st octet of DA
	flow2V4 = trafficflowAttr{
		innerSrcStart:        innerSrcIPv4Start,
		innerdstStart:        innerDstIPv4Start,
		innerFlowCount:       flowCount,
		egressTrackingOffset: IPinIPpSrcDstIPOffset,
		egressTrackingWidth:  IPinIPpSrcDstIPWidth,
	}
	//For IPinIP traffic flows with egress tracking 1st 4 bits of last octet of SA + 1st 8 bits of 1st octet of DA
	flow2V6 = trafficflowAttr{
		innerSrcStart:        innerSrcIPv6Start,
		innerdstStart:        innerDstIPv6Start,
		innerFlowCount:       flowCount,
		egressTrackingOffset: IPinIPpSrcDstIPOffset,
		egressTrackingWidth:  IPinIPpSrcDstIPWidth,
	}
	//For DSCP field value in the outer header IPinIP traffic
	flow3V4 = trafficflowAttr{
		innerSrcStart:        innerSrcIPv4Start,
		innerdstStart:        innerDstIPv4Start,
		innerFlowCount:       flowCount,
		egressTrackingOffset: IPinIPpDscpOffset,
		egressTrackingWidth:  IPinIPpDscpWidth,
	}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

type testCase struct {
	desc              string
	name              string
	fn                func(ctx context.Context, t *testing.T, args *testArgs)
	aftValidationType string
}

func TestVrfPolicyDrivenTE(t *testing.T) {
	ctx := context.Background()
	dut := ondatra.DUT(t, "dut")
	t.Logf("Config DUT")
	configureDUT(t, dut)
	t.Logf("Creating ATE")
	ate := ondatra.ATE(t, "ate")
	topo := configureATE(t, ate)
	cases := []testCase{
		{
			name:              "Decap with NO DSCP match",
			desc:              "match on source and protocol, no match on DSCP; flow VRF_DECAP hit -> DEFAULT",
			fn:                testBaseDecapNoDscpMatch,
			aftValidationType: "increment",
		},
		{
			name:              "Decap with DSCP match",
			desc:              "match on source, protocol and DSCP, VRF_DECAP hit -> VRF_ENCAP_A miss -> Fallback to DEFAULT",
			fn:                testBaseDecapDscpMatch,
			aftValidationType: "increment",
		},
		{
			name:              "Decap with NO DSCP match & Mixed Prefix Length Decap gRIBI Entries",
			desc:              "match on source and protocol, no match on DSCP; flow VRF_DECAP hit -> DEFAULT",
			fn:                testMixDecapNoDscpMatch,
			aftValidationType: "increment",
		},
		{
			name:              "Tunneled traffic with NO Decap",
			desc:              "IPinIP tunneled traffic recived on cluster interfaces are sent to TE VRF when no match in DECAP VRF",
			fn:                testTunnelWithNoDecap,
			aftValidationType: "increment",
		},
		{
			name:              "TE Disabled Default class match",
			desc:              "TE disabled IPinIP/IP cluster traffic arriving on WAN facing ports > Send to Default class",
			fn:                testTEDisabledTraffic,
			aftValidationType: "increment",
		},
		{
			name:              "Decap and encap",
			desc:              "Decap and then match route in post-Decap Encap VRF >> Traffic forwarded out with Encap header",
			fn:                testDecapEncap,
			aftValidationType: "increment",
		},
	}
	client := gribi.Client{
		DUT:         dut,
		FIBACK:      true,
		Persistence: true,
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Name: %s", tt.name)
			t.Logf("Description: %s", tt.desc)
			tcArgs := &testArgs{
				ctx:               ctx,
				gribiClient:       &client,
				dut:               dut,
				ate:               ate,
				topo:              topo,
				aftValidationType: tt.aftValidationType,
			}
			t.Logf("Reset to Base gRIBI programming")
			baseGribiProgramming(t, dut)
			tt.fn(ctx, t, tcArgs)
		})
	}
}

func testBaseDecapNoDscpMatch(ctx context.Context, t *testing.T, args *testArgs) {
	defer args.gribiClient.Close(t)
	defer args.gribiClient.FlushAll(t)
	if err := args.gribiClient.Start(t); err != nil {
		t.Fatalf("gRIBI Connection can not be established")
	}
	for _, tt := range prefixLengthVariation {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Name: %s", tt.name)
			electionID := args.gribiClient.LearnElectionID(t)
			// Flush entries in decap VRF before running the tc
			args.gribiClient.Flush(t, electionID, vrfDecap)
			args.gribiClient.BecomeLeader(t)
			args.gribiClient.AddNH(t, nh1ID, "Decap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
			args.gribiClient.AddNHG(t, nhg1ID, map[uint64]uint64{nh1ID: 1}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
			args.gribiClient.AddIPv4(t, tt.prefix, nhg1ID, vrfDecap, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

			t.Run("DECAP & forward with Match in Decap VRF", func(t *testing.T) {
				t.Log("Generating Traffic flows")
				flowDecapMatch := []*ondatra.Flow{flow1V4.createTrafficFlow(t, args.ate, "ipInIPFlowDecap", "IPv4", ipv4OuterDest, ipv4OuterSrc111, dscpEncapNoMatch, defaultDstPort), flow1V6.createTrafficFlow(t, args.ate, "ipv6InIPFlowDecap", "IPv6", ipv4OuterDest, ipv4OuterSrc111, dscpEncapNoMatch, defaultDstPort)}
				t.Log("Validate AFT Telemetry")
				args.validateAftTelemetry(t, vrfDecap, tt.prefix, 1)
				sendTraffic(t, args.ate, flowDecapMatch, args.aftValidationType)
				t.Logf("Validate Rx Traffic on Dest Port %v & Packet is Decap", defaultDstPort)
				args.validateTrafficFlows(t, flowDecapMatch, []string{strconv.Itoa(nhUdpProtocol)}, false)
			})
			t.Run("No DECAP with No Match in Decap VRF", func(t *testing.T) {
				t.Log("Generating Traffic flows")
				dstPorts := []string{atePort2.Name, atePort3.Name, atePort4.Name}
				flowDecapNoMatch := []*ondatra.Flow{flow1V4.createTrafficFlow(t, args.ate, "ipInIPFlowDecapNoMatch", "IPv4", transitVrfIP, ipv4OuterSrc111, dscpEncapNoMatch, dstPorts), flow1V6.createTrafficFlow(t, args.ate, "ipv6InIPFlowDecapNoMatch", "IPv6", transitVrfIP, ipv4OuterSrc111, dscpEncapNoMatch, dstPorts)}
				t.Log("Validate AFT Telemetry")
				args.validateAftTelemetry(t, vrfDecap, tt.prefix, 1)
				sendTraffic(t, args.ate, flowDecapNoMatch, args.aftValidationType)
				t.Logf("Validate Rx Traffic on Dest Ports %v & packet is IPinIP", dstPorts)
				args.validateTrafficFlows(t, flowDecapNoMatch, []string{strconv.Itoa(ipipProtocol), strconv.Itoa(ipv6ipProtocol)}, false)
			})
		})
	}
}

func testBaseDecapDscpMatch(ctx context.Context, t *testing.T, args *testArgs) {
	defer args.gribiClient.Close(t)
	defer args.gribiClient.FlushAll(t)
	if err := args.gribiClient.Start(t); err != nil {
		t.Fatalf("gRIBI Connection can not be established")
	}
	for _, tt := range prefixLengthVariation {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Name: %s", tt.name)
			electionID := args.gribiClient.LearnElectionID(t)
			// Flush entries in decap VRF before running the tc
			args.gribiClient.Flush(t, electionID, vrfDecap)
			// Flush entries in Encap VRFs for Prefix match miss scenario
			args.gribiClient.Flush(t, electionID, vrfEncapA)
			args.gribiClient.Flush(t, electionID, vrfEncapB)
			args.gribiClient.BecomeLeader(t)
			args.gribiClient.AddNH(t, nh1ID, "Decap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
			args.gribiClient.AddNHG(t, nhg1ID, map[uint64]uint64{nh1ID: 1}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
			args.gribiClient.AddIPv4(t, tt.prefix, nhg1ID, vrfDecap, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
			t.Run("DECAP & forward with Match in Decap VRF", func(t *testing.T) {
				t.Log("Generating Traffic flows")
				trafficFlow := []*ondatra.Flow{flow1V4.createTrafficFlow(t, args.ate, "ipInIPFlowDecap", "IPv4", ipv4OuterDest, ipv4OuterSrc111, dscpEncapA1, defaultDstPort), flow1V6.createTrafficFlow(t, args.ate, "ipv6InIPFlowDecap", "IPv6", ipv4OuterDest, ipv4OuterSrc111, dscpEncapA1, defaultDstPort)}
				t.Log("Validate AFT Telemetry")
				args.validateAftTelemetry(t, vrfDecap, tt.prefix, 1)
				sendTraffic(t, args.ate, trafficFlow, args.aftValidationType)
				t.Logf("Validate Rx Traffic on Dest Port %v & Packet is Decap", defaultDstPort)
				args.validateTrafficFlows(t, trafficFlow, []string{strconv.Itoa(nhUdpProtocol)}, false)
			})
		})
	}
}

func testMixDecapNoDscpMatch(ctx context.Context, t *testing.T, args *testArgs) {
	defer args.gribiClient.Close(t)
	defer args.gribiClient.FlushAll(t)
	if err := args.gribiClient.Start(t); err != nil {
		t.Fatalf("gRIBI Connection can not be established")
	}
	electionID := args.gribiClient.LearnElectionID(t)
	// Flush entries in decap VRF before running the tc
	args.gribiClient.Flush(t, electionID, vrfDecap)
	args.gribiClient.BecomeLeader(t)
	args.gribiClient.AddNH(t, nh1ID, "Decap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.gribiClient.AddNHG(t, nhg1ID, map[uint64]uint64{nh1ID: 1}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.gribiClient.AddIPv4(t, decapMixPrefix1, nhg1ID, vrfDecap, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.gribiClient.AddIPv4(t, decapMixPrefix2, nhg1ID, vrfDecap, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	t.Logf("Validate AFT Telemetry for prefixes in %v VRF", vrfDecap)
	args.validateAftTelemetry(t, vrfDecap, decapMixPrefix1, 1)
	args.validateAftTelemetry(t, vrfDecap, decapMixPrefix2, 1)
	t.Run("DECAP & forward with Match in Decap VRF", func(t *testing.T) {
		t.Log("Generating Traffic flows")
		flowDecapMatch := []*ondatra.Flow{flow1V4.createTrafficFlow(t, args.ate, "ipInIPFlowDecapMix1", "IPv4", "192.51.130.64", ipv4OuterSrc111, dscpEncapNoMatch, defaultDstPort), flow1V4.createTrafficFlow(t, args.ate, "ipInIPFlowDecapMix2", "IPv4", "192.51.128.5", ipv4OuterSrc111, dscpEncapNoMatch, defaultDstPort), flow1V6.createTrafficFlow(t, args.ate, "ipv6InIPFlowDecap", "IPv6", "192.55.200.3", ipv4OuterSrc111, dscpEncapNoMatch, defaultDstPort)}
		sendTraffic(t, args.ate, flowDecapMatch, args.aftValidationType)
		t.Logf("Validate Rx Traffic on Dest Port %v & Packet is Decap", defaultDstPort)
		args.validateTrafficFlows(t, flowDecapMatch, []string{strconv.Itoa(nhUdpProtocol)}, false)
	})
	t.Run("NO DECAP with NO Match in Decap VRF", func(t *testing.T) {
		dstPorts := []string{atePort2.Name, atePort3.Name, atePort4.Name}
		t.Log("Generating Traffic flows")
		flowDecapNoMatch := []*ondatra.Flow{flow1V4.createTrafficFlow(t, args.ate, "ipInIPFlowDecapNoMatch", "IPv4", transitVrfIP, ipv4OuterSrc111, dscpEncapNoMatch, dstPorts), flow1V6.createTrafficFlow(t, args.ate, "ipv6InIPFlowDecapNoMatch", "IPv6", transitVrfIP, ipv4OuterSrc111, dscpEncapNoMatch, dstPorts)}
		t.Log("Validate flows without match in decap VRF recieved on Port2,3,4 is IPinIP")
		sendTraffic(t, args.ate, flowDecapNoMatch, args.aftValidationType)
		t.Logf("Validate Rx Traffic on Dest Ports %v & packet is IPinIP", dstPorts)
		args.validateTrafficFlows(t, flowDecapNoMatch, []string{strconv.Itoa(ipipProtocol), strconv.Itoa(ipv6ipProtocol)}, false)
	})
}

func testTunnelWithNoDecap(ctx context.Context, t *testing.T, args *testArgs) {
	t.Log("Apply cluster facing PBR policy on Ingress port")
	sp := args.dut.Port(t, dutPort1.Name)
	applyForwardingPolicy(t, sp.Name(), clusterPolicy, false)
	t.Run("Verify Tunneled traffic with no decap with SRC for Transit VRF", func(t *testing.T) {
		dstPorts := []string{atePort2.Name, atePort3.Name, atePort4.Name}
		trafficFlow := []*ondatra.Flow{flow2V4.createTrafficFlow(t, args.ate, "ipInIPFlowDscpNoMatch", "IPv4", transitVrfIP, ipv4OuterSrc111, dscpEncapNoMatch, dstPorts), flow2V6.createTrafficFlow(t, args.ate, "ipv6InIPFlowDscpNoMatch", "IPv6", transitVrfIP, ipv4OuterSrc111, dscpEncapNoMatch, dstPorts)}
		sendTraffic(t, args.ate, trafficFlow, args.aftValidationType)
		t.Logf("Validate Rx Traffic on Dest Ports %v & packet is IPinIP & with DA 138.x.x.x", dstPorts)
		args.validateTrafficFlows(t, trafficFlow, []string{src111TeDstFlowFilter}, false)
		weights := []float64{0.0625, 0.1875, 0.75}
		validateTrafficDistribution(t, args.ate, weights, dstPorts)
	})
	t.Run("Verify Tunneled traffic with no decap with SRC for Repaired VRF", func(t *testing.T) {
		dstPorts := []string{atePort5.Name}
		trafficFlow := []*ondatra.Flow{flow2V4.createTrafficFlow(t, args.ate, "ipInIPFlowDscpNoMatch", "IPv4", repairedVrfIP, ipv4OuterSrc222, dscpEncapNoMatch, dstPorts), flow2V6.createTrafficFlow(t, args.ate, "ipv6InIPFlowDscpNoMatch", "IPv6", repairedVrfIP, ipv4OuterSrc222, dscpEncapNoMatch, dstPorts)}
		sendTraffic(t, args.ate, trafficFlow, args.aftValidationType)
		t.Logf("Validate Rx Traffic on Dest Ports %v & packet is IPinIP & with DA 138.x.x.x", dstPorts)
		args.validateTrafficFlows(t, trafficFlow, []string{src222TeDstFlowFilter}, false)
	})
}

func testTEDisabledTraffic(ctx context.Context, t *testing.T, args *testArgs) {
	t.Logf("Configure static route for outer header IP prefix %v in Default VRF with NH to Port8", encapVrfIPv4Prefix)
	configStaticRoute(t, args.dut, encapVrfIPv4Prefix, AtePorts["port8"].IPv4, "", "", false)
	t.Run("Verify TE disabled IPinIP traffic with No Match SRC IP", func(t *testing.T) {
		dstPorts := []string{atePort8.Name}
		trafficFlow := []*ondatra.Flow{flow2V4.createTrafficFlow(t, args.ate, "ipInIPFlowSrcNoMatch", "IPv4", encapIPv4FlowIP, noMatchSrcIP, dscpEncapNoMatch, dstPorts), flow2V6.createTrafficFlow(t, args.ate, "ipv6InIPFlowSrcNoMatch", "IPv6", encapIPv4FlowIP, noMatchSrcIP, dscpEncapNoMatch, dstPorts)}
		sendTraffic(t, args.ate, trafficFlow, args.aftValidationType)
		t.Logf("Validate Rx Traffic on Dest Ports %v & packet is IPinIP", dstPorts)
		args.validateTrafficFlows(t, trafficFlow, []string{noMatchSrcEncapDstFilter}, false)
	})
	t.Run("Verify v4 traffic with Transit_Reapired SRC IP", func(t *testing.T) {
		dstPorts := []string{atePort8.Name}
		trafficFlow := []*ondatra.Flow{flow1V4.createTrafficFlow(t, args.ate, "ipv4FlowSrc111", "", encapIPv4FlowIP, ipv4OuterSrc111, dscpEncapNoMatch, dstPorts), flow1V6.createTrafficFlow(t, args.ate, "ipv4FlowSrc222", "", encapIPv4FlowIP, ipv4OuterSrc222, dscpEncapNoMatch, dstPorts)}
		sendTraffic(t, args.ate, trafficFlow, args.aftValidationType)
		t.Logf("Validate Rx Traffic on Dest Ports %v & packet is v4 packet with Next header UDP", dstPorts)
		args.validateTrafficFlows(t, trafficFlow, []string{strconv.Itoa(nhUdpProtocol)}, false)
	})
	t.Run("Remove Default route & verify traffic is Dropped", func(t *testing.T) {
		dstPorts := []string{atePort8.Name}
		trafficFlow := []*ondatra.Flow{flow1V4.createTrafficFlow(t, args.ate, "ipv4FlowSrc111", "", encapIPv4FlowIP, ipv4OuterSrc111, dscpEncapNoMatch, dstPorts), flow1V6.createTrafficFlow(t, args.ate, "ipv4FlowSrc222", "", encapIPv4FlowIP, ipv4OuterSrc222, dscpEncapNoMatch, dstPorts)}
		t.Logf("Delete Static route for outer header IP prefix %v", encapVrfIPv4Prefix)
		configStaticRoute(t, args.dut, encapVrfIPv4Prefix, AtePorts["port8"].IPv4, "", "", true)
		sendTraffic(t, args.ate, trafficFlow, args.aftValidationType)
		t.Log("Validate Traffic is dropped with No route in Default VRF")
		args.validateTrafficFlows(t, trafficFlow, []string{}, true)
	})
}

func testDecapEncap(ctx context.Context, t *testing.T, args *testArgs) {
	defer args.gribiClient.Close(t)
	defer args.gribiClient.FlushAll(t)
	if err := args.gribiClient.Start(t); err != nil {
		t.Fatalf("gRIBI Connection can not be established")
	}
	electionID := args.gribiClient.LearnElectionID(t)
	// Flush entries in decap VRF before running the tc
	args.gribiClient.Flush(t, electionID, vrfDecap)
	args.gribiClient.BecomeLeader(t)
	t.Logf("Program Decap entries for prefixes %v & %v in vrf %v", decapMixPrefix1, decapMixPrefix1, vrfDecap)
	args.gribiClient.AddNH(t, nh1ID, "Decap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.gribiClient.AddNHG(t, nhg1ID, map[uint64]uint64{nh1ID: 1}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.gribiClient.AddIPv4(t, decapMixPrefix1, nhg1ID, vrfDecap, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.gribiClient.AddIPv4(t, decapMixPrefix2, nhg1ID, vrfDecap, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	t.Logf("Program Inner IPv6 entries for prefixe %v in Encap VRFs %v & %v", encapVrfIPv6Prefix, vrfEncapA, vrfEncapB)

	//TODO Add IPv6 entries in Encap VRF once https://github.com/openconfig/featureprofiles/pull/2457 is merged

	t.Logf("Validate AFT Telemetry for prefixes in %v VRF", vrfDecap)
	args.validateAftTelemetry(t, vrfDecap, decapMixPrefix1, 1)
	args.validateAftTelemetry(t, vrfDecap, decapMixPrefix2, 1)
	t.Run("Verify Decap & Encap with DSCP_A", func(t *testing.T) {
		dstPorts := []string{atePort2.Name, atePort3.Name, atePort4.Name, atePort6.Name}
		//TODO Add IPv6inIP Flow after AddIPv6 entries in Encap VRF
		trafficFlow := []*ondatra.Flow{flow2V4.createTrafficFlow(t, args.ate, "ipInIPFDecapEncap", "IPv4", "192.51.130.64", ipv4OuterSrc222, dscpEncapA1, dstPorts)}
		sendTraffic(t, args.ate, trafficFlow, args.aftValidationType)
		t.Logf("Validate Rx Traffic on Dest Ports %v & packet is IPinIP", dstPorts)
		args.validateTrafficFlows(t, trafficFlow, []string{src111TeDstFlowFilter}, false)
		t.Logf("Validate Hierarchical Traffic on Dest Ports %v", dstPorts)
		weights := []float64{0.015625, 0.046875, 0.1875, 0.75}
		validateTrafficDistribution(t, args.ate, weights, dstPorts)
		t.Logf("Validate DSCP value %v for egress IPinIP traffic", dscpEncapA1)
		trafficFlow = []*ondatra.Flow{flow3V4.createTrafficFlow(t, args.ate, "ipInIPFDecapEncap", "IPv4", "192.51.130.64", ipv4OuterSrc222, dscpEncapA1, dstPorts)}
		sendTraffic(t, args.ate, trafficFlow, args.aftValidationType)
		//dscpEncap decimal val is 5 bits in binar vs. 7 bit ATE val, so left shift with 2 bits to match
		args.validateTrafficFlows(t, trafficFlow, []string{strconv.Itoa(dscpEncapA1 << 2)}, false)
	})
	t.Run("Verify Decap & Encap with DSCP_B", func(t *testing.T) {
		dstPorts := []string{atePort2.Name, atePort3.Name, atePort4.Name, atePort6.Name}
		//TODO Add IPv6inIP Flow after AddIPv6 entries in Encap VRF
		trafficFlow := []*ondatra.Flow{flow2V4.createTrafficFlow(t, args.ate, "ipInIPFDecapEncap", "IPv4", "192.51.130.64", ipv4OuterSrc222, dscpEncapB1, dstPorts)}
		sendTraffic(t, args.ate, trafficFlow, args.aftValidationType)
		t.Logf("Validate Rx Traffic on Dest Ports %v & packet is IPinIP", dstPorts)
		args.validateTrafficFlows(t, trafficFlow, []string{src111TeDstFlowFilter}, false)
		t.Logf("Validate Hierarchical Traffic on Dest Ports %v", dstPorts)
		weights := []float64{0.046875, 0.140625, 0.5625, 0.25}
		validateTrafficDistribution(t, args.ate, weights, dstPorts)
		t.Logf("Validate DSCP value %v for egress IPinIP traffic", dscpEncapB1)
		trafficFlow = []*ondatra.Flow{flow3V4.createTrafficFlow(t, args.ate, "ipInIPFDecapEncap", "IPv4", "192.51.130.64", ipv4OuterSrc222, dscpEncapB1, dstPorts)}
		sendTraffic(t, args.ate, trafficFlow, args.aftValidationType)
		//dscpEncap decimal val is 5 bits in binar vs. 7 bit ATE val, so left shift with 2 bits to match
		args.validateTrafficFlows(t, trafficFlow, []string{strconv.Itoa(dscpEncapB1 << 2)}, false)
	})
}

func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	configureBaseconfig(t, dut)
	// apply PBF to src interface.
	sp := dut.Port(t, dutPort1.Name)
	applyForwardingPolicy(t, sp.Name(), wanPolicy, false)

	//Configure default static route
	configStaticRoute(t, dut, encapVrfIPv4Prefix, AtePorts["port8"].IPv4, encapVrfIPv6Prefix, AtePorts["port8"].IPv6, false)
}

// validateTrafficFlows verifies no trafic loss for the flows on ATE & verifies Packet Egress tracking fields based on reqFilterList
func (args *testArgs) validateTrafficFlows(t *testing.T, flows []*ondatra.Flow, reqFilterList []string, wantLoss bool) {

	for _, flow := range flows {
		flowPath := gnmi.OC().Flow(flow.Name())
		t.Log("Verify no traffic loss")
		got := gnmi.Get(t, args.ate, flowPath.LossPct().State())
		if wantLoss {
			if got < 100 {
				t.Fatalf("LossPct for flow %s: got %g, want 100", flow.Name(), got)
			}
		} else {
			if got > 0 {
				t.Logf("LossPct for flow %s: got %g, want 0", flow.Name(), got)

			}
		}
		if flow.Name() != "ipv6InIPFlowDecap" && !wantLoss {
			t.Log("Verify Protocol field for packets recived on ATE")
			egressTrackPath := flowPath.EgressTrackingAny()
			egressTrackState := gnmi.GetAll(t, args.ate, egressTrackPath.State())
			getFlowFilter := egressTrackState[0].GetFilter()
			if slices.Contains(reqFilterList, getFlowFilter) {
				t.Log("Egress tracking filter matches for the Rx packet on ATE")
			} else {
				t.Errorf("EgressTracking filter got %q, want %q", getFlowFilter, reqFilterList)
			}
			inPkts := gnmi.Get(t, args.ate, flowPath.Counters().InPkts().State())
			ingressTrack := flowPath.IngressTrackingAny()
			ingressTrackCounters := gnmi.GetAll(t, args.ate, ingressTrack.Counters().InPkts().State())
			var ingressTrackPackets uint64
			for _, v := range ingressTrackCounters {
				if slices.Contains(ingressTrackCounters, 0) {
					break
				}
				ingressTrackPackets = ingressTrackPackets + v
			}
			if got := ingressTrackPackets; got != inPkts {
				t.Errorf("IngressTracking counter in-pkts got %d, want %d", got, inPkts)
			}
		}
	}
}
