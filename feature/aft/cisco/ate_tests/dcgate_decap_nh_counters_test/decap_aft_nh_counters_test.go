package dcgate_decap_aft_nh_counters

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"testing"
	"time"

	aftUtil "github.com/openconfig/featureprofiles/feature/aft/cisco/aftUtils"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
)

const (
	nh1ID                             = 120
	nhg1ID                            = 20
	ipv4OuterDest                     = "192.51.100.65"
	transitVrfIP                      = "203.0.113.1"
	repairedVrfIP                     = "203.0.113.100"
	noMatchSrcIP                      = "198.100.200.123"
	decapMixPrefix1                   = "192.51.128.0/22"
	decapMixPrefix2                   = "192.55.200.3/32"
	src111TeDstFlowFilter             = "4043" // Egress tracking flow filter decimal value for first 4 bits of last octet of SA 198.51.100.111 + First 8 bits of first octet of TE DA 203.0.113.1
	src222TeDstFlowFilter             = "3787" // Egress tracking flow filter decimal value for first 4 bits of last octet of SA 198.51.100.222 + First 8 bits of first octet of TE DA 203.0.113.100
	noMatchSrcEncapDstFilter          = "2954" // Egress tracking flow filter decimal value for first 4 bits of last octet of SA 198.100.200.123 + First 8 bits of first octet of TE DA 138.0.11.8
	IPinIPProtocolFieldOffset         = 184
	IPinIPProtocolFieldWidth          = 8
	IPinIPpSrcDstIPOffset             = 236
	IPinIPpSrcDstIPWidth              = 12
	IPinIPpDscpOffset                 = 120
	IPinIPpDscpWidth                  = 8
	sampleInterval                    = 5 * time.Second
	collectTime                       = 90 * time.Second
	aftCountertolerance       float64 = 1.0
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
	fn                func(t *testing.T, args *testArgs)
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
			aftValidationType: "exact",
		},
		{
			name:              "Decap with DSCP match",
			desc:              "match on source, protocol and DSCP, VRF_DECAP hit -> VRF_ENCAP_A miss -> Fallback to DEFAULT",
			fn:                testBaseDecapDscpMatch,
			aftValidationType: "exact",
		},
		{
			name:              "Decap with NO DSCP match & Mixed Prefix Length Decap gRIBI Entries",
			desc:              "match on source and protocol, no match on DSCP; flow VRF_DECAP hit -> DEFAULT",
			fn:                testMixDecapNoDscpMatch,
			aftValidationType: "transit",
		},
		{
			name:              "Tunneled traffic with NO Decap",
			desc:              "IPinIP tunneled traffic recived on cluster interfaces are sent to TE VRF when no match in DECAP VRF",
			fn:                testTunnelWithNoDecap,
			aftValidationType: "transit",
		},
		{
			name:              "TE Disabled Default class match",
			desc:              "TE disabled IPinIP/IP cluster traffic arriving on WAN facing ports > Send to Default class",
			fn:                testTEDisabledTraffic,
			aftValidationType: "transit",
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
				NextHopTypes:      make(map[uint64]string),
			}
			t.Logf("Reset to Base gRIBI programming")
			baseGribiProgramming(t, dut)
			tt.fn(t, tcArgs)
		})
	}
}

func testBaseDecapNoDscpMatch(t *testing.T, args *testArgs) {
	// Graceful close & flush of the gRIBI client
	defer args.gribiClient.Close(t)
	defer args.gribiClient.FlushAll(t)
	gnmiClient := args.dut.RawAPIs().GNMI(t)

	// Start the gRIBI client
	if err := args.gribiClient.Start(t); err != nil {
		t.Fatalf("gRIBI Connection can not be established")
	}

	// Iterate over each prefix length variation
	for _, tt := range prefixLengthVariation {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Name: %s", tt.name)

			// Standard housekeeping
			electionID := args.gribiClient.LearnElectionID(t)
			args.gribiClient.Flush(t, electionID, vrfDecap)
			args.gribiClient.BecomeLeader(t)

			// 1) Program a decap next-hop
			//    We tell gRIBI "Decap" as a local handle, but we won't rely on ephemeral IDs.
			args.gribiClient.AddNH(
				t, nh1ID, "Decap", // local handle says decap
				deviations.DefaultNetworkInstance(args.dut),
				fluent.InstalledInFIB,
			)
			// Then create the next-hop group referencing that local ID
			args.gribiClient.AddNHG(
				t, nhg1ID,
				map[uint64]uint64{nh1ID: 1},
				deviations.DefaultNetworkInstance(args.dut),
				fluent.InstalledInFIB,
			)
			// Program the prefix in the DECAP VRF
			args.gribiClient.AddIPv4(
				t, tt.prefix, nhg1ID,
				vrfDecap,
				deviations.DefaultNetworkInstance(args.dut),
				fluent.InstalledInFIB,
			)

			t.Run("DECAP & forward with Match in Decap VRF", func(t *testing.T) {
				t.Log("Generating Traffic flows")

				flow1, details1 := flow1V4.createTrafficFlow(
					t, args.ate, "ipInIPFlowDecap", "IPv4",
					ipv4OuterDest, ipv4OuterSrc111,
					dscpEncapNoMatch, defaultDstPort,
				)
				flow2, details2 := flow1V6.createTrafficFlow(
					t, args.ate, "ipv6InIPFlowDecap", "IPv6",
					ipv4OuterDest, ipv4OuterSrc111,
					dscpEncapNoMatch, defaultDstPort,
				)
				flowDecapMatch := []*ondatra.Flow{flow1, flow2}

				flowDetails := map[string]aftUtil.FlowDetails{
					flow1.Name(): details1,
					flow2.Name(): details2,
				}

				// 1) Get pre-traffic counters
				//preCounters, err := aftUtil.GetAftCountersModePoll(t, gnmiClient)
				preCounters, err := aftUtil.GetAftCountersSample(t, gnmiClient, sampleInterval, time.Second*60)
				if err != nil {
					t.Fatalf("Failed to get pre-counters via poll: %v", err)
				}

				// 3) Send traffic referencing statsMapping
				sendTraffic(t, args.ate, flowDecapMatch, flowDetails)

				// 3) Get post-traffic counters
				postCounters, err := aftUtil.GetAftCountersSample(t, gnmiClient, sampleInterval, collectTime)
				if err != nil {
					t.Fatalf("Failed to get post-counters via poll: %v", err)
				}
				t.Logf("Post-counters: %v", postCounters)

				results := aftUtil.BuildAftPrefixChain(t, args.dut, preCounters, postCounters)
				aftUtil.AftCounterResults(t, flowDetails, results, args.aftValidationType, len(postCounters),
					aftCountertolerance, "Decap")

				// 4) Validate traffic result
				t.Logf("Validate Rx Traffic on Dest Port %v & Packet is Decap", defaultDstPort)
				args.validateTrafficFlows(t, flowDecapMatch, []string{strconv.Itoa(nhUdpProtocol)}, false)
			})

			t.Run("No DECAP with No Match in Decap VRF", func(t *testing.T) {
				t.Log("Generating Traffic flows")
				dstPorts := []string{atePort2.Name, atePort3.Name, atePort4.Name}

				flow1, details1 := flow1V4.createTrafficFlow(
					t, args.ate, "ipInIPFlowDecapNoMatch", "IPv4",
					transitVrfIP, ipv4OuterSrc111,
					dscpEncapNoMatch, dstPorts,
				)
				flow2, details2 := flow1V6.createTrafficFlow(
					t, args.ate, "ipv6InIPFlowDecapNoMatch", "IPv6",
					transitVrfIP, ipv4OuterSrc111,
					dscpEncapNoMatch, dstPorts,
				)
				flowDecapNoMatch := []*ondatra.Flow{flow1, flow2}
				flowDetails := map[string]aftUtil.FlowDetails{
					flow1.Name(): details1,
					flow2.Name(): details2,
				}

				// 1) Get pre-traffic counters
				preCounters, err := aftUtil.GetAftCountersSample(t, gnmiClient, sampleInterval, collectTime)
				if err != nil {
					t.Fatalf("Failed to get pre-counters via poll: %v", err)
				}

				// 3) Send traffic with same statsMapping
				sendTraffic(t, args.ate, flowDecapNoMatch, flowDetails)

				// 3) Get post-traffic counters
				//postCounters, err := aftUtil.GetAftCountersModeOnce(t, gnmiClient)
				postCounters, err := aftUtil.GetAftCountersSample(t, gnmiClient, sampleInterval, collectTime)
				if err != nil {
					t.Fatalf("GetAftCountersSample error: %v", err)
				}
				t.Logf("Post-counters: %v", postCounters)

				results := aftUtil.BuildAftPrefixChain(t, args.dut, preCounters, postCounters)
				aftUtil.AftCounterResults(t, flowDetails, results, "transit", len(postCounters),
					aftCountertolerance, "Transit")

				t.Logf("Validate Rx Traffic on Dest Ports %v & packet is IPinIP", dstPorts)
				args.validateTrafficFlows(
					t, flowDecapNoMatch,
					[]string{strconv.Itoa(ipipProtocol), strconv.Itoa(ipv6ipProtocol)},
					false,
				)
			})
		})
	}
}

func testBaseDecapDscpMatch(t *testing.T, args *testArgs) {
	// Graceful close & flush
	defer args.gribiClient.Close(t)
	defer args.gribiClient.FlushAll(t)
	gnmiClient := args.dut.RawAPIs().GNMI(t)

	if err := args.gribiClient.Start(t); err != nil {
		t.Fatalf("gRIBI Connection can not be established")
	}

	for _, tt := range prefixLengthVariation {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Name: %s", tt.name)

			// Standard housekeeping
			electionID := args.gribiClient.LearnElectionID(t)
			args.gribiClient.Flush(t, electionID, vrfDecap)
			args.gribiClient.Flush(t, electionID, vrfEncapA)
			args.gribiClient.Flush(t, electionID, vrfEncapB)
			args.gribiClient.BecomeLeader(t)

			// Program a decap next-hop
			args.gribiClient.AddNH(
				t, nh1ID, "Decap",
				deviations.DefaultNetworkInstance(args.dut),
				fluent.InstalledInFIB,
			)
			args.gribiClient.AddNHG(
				t, nhg1ID,
				map[uint64]uint64{nh1ID: 1},
				deviations.DefaultNetworkInstance(args.dut),
				fluent.InstalledInFIB,
			)
			args.gribiClient.AddIPv4(
				t, tt.prefix, nhg1ID,
				vrfDecap,
				deviations.DefaultNetworkInstance(args.dut),
				fluent.InstalledInFIB,
			)

			t.Run("DECAP & forward with Match in Decap VRF", func(t *testing.T) {
				t.Log("Generating Traffic flows")

				flow1, details1 := flow1V4.createTrafficFlow(
					t, args.ate, "ipInIPFlowDecap", "IPv4",
					ipv4OuterDest, ipv4OuterSrc111,
					dscpEncapA1, defaultDstPort,
				)
				flow2, details2 := flow1V6.createTrafficFlow(
					t, args.ate, "ipv6InIPFlowDecap", "IPv6",
					ipv4OuterDest, ipv4OuterSrc111,
					dscpEncapA1, defaultDstPort,
				)

				trafficFlow := []*ondatra.Flow{flow1, flow2}
				flowDetails := map[string]aftUtil.FlowDetails{
					flow1.Name(): details1,
					flow2.Name(): details2,
				}

				// 1) Get pre-traffic counters
				preCounters, err := aftUtil.GetAftCountersSample(t, gnmiClient, sampleInterval, collectTime)
				if err != nil {
					t.Fatalf("Failed to get pre-counters via poll: %v", err)
				}

				// 2) Send Traffic
				sendTraffic(t, args.ate, trafficFlow, flowDetails)

				// 3) Get post-traffic counters
				postCounters, err := aftUtil.GetAftCountersSample(t, gnmiClient, sampleInterval, collectTime)
				if err != nil {
					t.Fatalf("Failed to get post-counters via poll: %v", err)
				}
				t.Logf("Post-counters: %v", postCounters)

				results := aftUtil.BuildAftPrefixChain(t, args.dut, preCounters, postCounters)
				aftUtil.AftCounterResults(t, flowDetails, results, args.aftValidationType, len(postCounters),
					aftCountertolerance, "Decap")

				t.Logf("Validate Rx Traffic on Dest Port %v & Packet is Decap", defaultDstPort)
				args.validateTrafficFlows(t, trafficFlow, []string{strconv.Itoa(nhUdpProtocol)}, false)
			})
		})
	}
}

func testMixDecapNoDscpMatch(t *testing.T, args *testArgs) {
	defer args.gribiClient.Close(t)
	defer args.gribiClient.FlushAll(t)
	gnmiClient := args.dut.RawAPIs().GNMI(t)

	if err := args.gribiClient.Start(t); err != nil {
		t.Fatalf("gRIBI Connection can not be established")
	}

	// Standard housekeeping
	electionID := args.gribiClient.LearnElectionID(t)
	args.gribiClient.Flush(t, electionID, vrfDecap)
	args.gribiClient.BecomeLeader(t)

	// Program decap next-hop
	args.gribiClient.AddNH(
		t, nh1ID, "Decap",
		deviations.DefaultNetworkInstance(args.dut),
		fluent.InstalledInFIB,
	)
	args.gribiClient.AddNHG(
		t, nhg1ID,
		map[uint64]uint64{nh1ID: 1},
		deviations.DefaultNetworkInstance(args.dut),
		fluent.InstalledInFIB,
	)
	args.gribiClient.AddIPv4(
		t, decapMixPrefix1, nhg1ID, vrfDecap,
		deviations.DefaultNetworkInstance(args.dut),
		fluent.InstalledInFIB,
	)
	args.gribiClient.AddIPv4(
		t, decapMixPrefix2, nhg1ID, vrfDecap,
		deviations.DefaultNetworkInstance(args.dut),
		fluent.InstalledInFIB,
	)

	t.Run("DECAP & forward with Match in Decap VRF", func(t *testing.T) {
		t.Log("Generating Traffic flows")
		flow1, details1 := flow1V4.createTrafficFlow(t, args.ate,
			"ipInIPFlowDecapMix1", "IPv4",
			"192.51.130.64", ipv4OuterSrc111,
			dscpEncapNoMatch, defaultDstPort,
		)
		flow2, details2 := flow1V4.createTrafficFlow(t, args.ate,
			"ipInIPFlowDecapMix2", "IPv4",
			"192.51.128.5", ipv4OuterSrc111,
			dscpEncapNoMatch, defaultDstPort,
		)
		flow3, details3 := flow1V6.createTrafficFlow(t, args.ate,
			"ipv6InIPFlowDecap", "IPv6",
			"192.55.200.3", ipv4OuterSrc111,
			dscpEncapNoMatch, defaultDstPort,
		)

		flowDecapMatch := []*ondatra.Flow{flow1, flow2, flow3}
		flowDetails := map[string]aftUtil.FlowDetails{
			flow1.Name(): details1,
			flow2.Name(): details2,
			flow3.Name(): details3,
		}

		// 1) Get pre-traffic counters
		preCounters, err := aftUtil.GetAftCountersSample(t, gnmiClient, sampleInterval, collectTime)
		if err != nil {
			t.Fatalf("Failed to get pre-counters via poll: %v", err)
		}

		// 2) send traffic
		sendTraffic(t, args.ate, flowDecapMatch, flowDetails)

		// 3) Get post-traffic counters
		postCounters, err := aftUtil.GetAftCountersSample(t, gnmiClient, sampleInterval, collectTime)
		if err != nil {
			t.Fatalf("GetAftCountersSample error: %v", err)
		}

		results := aftUtil.BuildAftPrefixChain(t, args.dut, preCounters, postCounters)
		aftUtil.AftCounterResults(t, flowDetails, results, "increment", len(postCounters),
			aftCountertolerance, "Decap")

		t.Logf("Validate Rx Traffic on Dest Port %v & Packet is Decap", defaultDstPort)
		args.validateTrafficFlows(t, flowDecapMatch, []string{strconv.Itoa(nhUdpProtocol)}, false)
	})

	t.Run("NO DECAP with NO Match in Decap VRF", func(t *testing.T) {
		dstPorts := []string{atePort2.Name, atePort3.Name, atePort4.Name}
		t.Log("Generating Traffic flows")
		flow1, details1 := flow1V4.createTrafficFlow(t, args.ate,
			"ipInIPFlowDecapNoMatch", "IPv4",
			transitVrfIP, ipv4OuterSrc111,
			dscpEncapNoMatch, dstPorts,
		)
		flow2, details2 := flow1V6.createTrafficFlow(t, args.ate,
			"ipv6InIPFlowDecapNoMatch", "IPv6",
			transitVrfIP, ipv4OuterSrc111,
			dscpEncapNoMatch, dstPorts,
		)

		flowDecapNoMatch := []*ondatra.Flow{flow1, flow2}
		flowDetails := map[string]aftUtil.FlowDetails{
			flow1.Name(): details1,
			flow2.Name(): details2,
		}

		// 1) Get pre-traffic counters
		preCounters, err := aftUtil.GetAftCountersSample(t, gnmiClient, sampleInterval, collectTime)
		if err != nil {
			t.Fatalf("Failed to get pre-counters via poll: %v", err)
		}

		t.Log("Validate flows without match in decap VRF recieved on Port2,3,4 is IPinIP")

		sendTraffic(t, args.ate, flowDecapNoMatch, flowDetails)

		// 3) Get post-traffic counters
		postCounters, err := aftUtil.GetAftCountersSample(t, gnmiClient, sampleInterval, collectTime)
		if err != nil {
			t.Fatalf("GetAftCountersSample error: %v", err)
		}

		results := aftUtil.BuildAftPrefixChain(t, args.dut, preCounters, postCounters)
		aftUtil.AftCounterResults(t, flowDetails, results, args.aftValidationType, len(postCounters),
			aftCountertolerance, "Decap")

		t.Logf("Validate Rx Traffic on Dest Ports %v & packet is IPinIP", dstPorts)
		args.validateTrafficFlows(t, flowDecapNoMatch,
			[]string{strconv.Itoa(ipipProtocol), strconv.Itoa(ipv6ipProtocol)},
			false,
		)
	})
}

func testTunnelWithNoDecap(t *testing.T, args *testArgs) {
	gnmiClient := args.dut.RawAPIs().GNMI(t)
	t.Log("Apply cluster facing PBR policy on Ingress port")
	sp := args.dut.Port(t, dutPort1.Name)
	applyForwardingPolicy(t, sp.Name(), clusterPolicy, false)

	t.Run("Verify Tunneled traffic with no decap with SRC for Transit VRF", func(t *testing.T) {
		dstPorts := []string{atePort2.Name, atePort3.Name, atePort4.Name}
		flow1, details1 := flow2V4.createTrafficFlow(t, args.ate, "ipInIPFlowDscpNoMatch", "IPv4",
			transitVrfIP, ipv4OuterSrc111, dscpEncapNoMatch, dstPorts)
		flow2, details2 := flow2V6.createTrafficFlow(t, args.ate, "ipv6InIPFlowDscpNoMatch", "IPv6",
			transitVrfIP, ipv4OuterSrc111, dscpEncapNoMatch, dstPorts)

		trafficFlow := []*ondatra.Flow{flow1, flow2}
		flowDetails := map[string]aftUtil.FlowDetails{
			flow1.Name(): details1,
			flow2.Name(): details2,
		}

		// 1) Get pre-traffic counters
		preCounters, err := aftUtil.GetAftCountersSample(t, gnmiClient, sampleInterval, collectTime)
		if err != nil {
			t.Fatalf("Failed to get pre-counters via poll: %v", err)
		}

		sendTraffic(t, args.ate, trafficFlow, flowDetails)
		t.Logf("Validate Rx Traffic on Dest Ports %v & packet is IPinIP & with DA 138.x.x.x", dstPorts)

		// 3) Get post-traffic counters
		postCounters, err := aftUtil.GetAftCountersSample(t, gnmiClient, sampleInterval, collectTime)
		if err != nil {
			t.Fatalf("GetAftCountersSample error: %v", err)
		}

		results := aftUtil.BuildAftPrefixChain(t, args.dut, preCounters, postCounters)
		aftUtil.AftCounterResults(t, flowDetails, results, args.aftValidationType, len(postCounters),
			aftCountertolerance, "Transit")

		args.validateTrafficFlows(t, trafficFlow, []string{src111TeDstFlowFilter}, false)
		weights := []float64{0.0625, 0.1875, 0.75}
		validateTrafficDistribution(t, args.ate, weights, dstPorts)
	})

	t.Run("Verify Tunneled traffic with no decap with SRC for Repaired VRF", func(t *testing.T) {
		dstPorts := []string{atePort5.Name}
		flow1, details1 := flow2V4.createTrafficFlow(t, args.ate, "ipInIPFlowDscpNoMatch", "IPv4",
			repairedVrfIP, ipv4OuterSrc222, dscpEncapNoMatch, dstPorts)
		flow2, details2 := flow2V6.createTrafficFlow(t, args.ate, "ipv6InIPFlowDscpNoMatch", "IPv6",
			repairedVrfIP, ipv4OuterSrc222, dscpEncapNoMatch, dstPorts)

		trafficFlow := []*ondatra.Flow{flow1, flow2}
		flowDetails := map[string]aftUtil.FlowDetails{
			flow1.Name(): details1,
			flow2.Name(): details2,
		}

		// 1) Get pre-traffic counters
		preCounters, err := aftUtil.GetAftCountersSample(t, gnmiClient, sampleInterval, collectTime)
		if err != nil {
			t.Fatalf("Failed to get pre-counters via poll: %v", err)
		}

		sendTraffic(t, args.ate, trafficFlow, flowDetails)

		// 3) Get post-traffic counters
		postCounters, err := aftUtil.GetAftCountersSample(t, gnmiClient, sampleInterval, collectTime)
		if err != nil {
			t.Fatalf("GetAftCountersSample error: %v", err)
		}

		results := aftUtil.BuildAftPrefixChain(t, args.dut, preCounters, postCounters)
		aftUtil.AftCounterResults(t, flowDetails, results, args.aftValidationType, len(postCounters),
			aftCountertolerance, "Transit")

		t.Logf("Validate Rx Traffic on Dest Ports %v & packet is IPinIP & with DA 138.x.x.x", dstPorts)
		args.validateTrafficFlows(t, trafficFlow, []string{src222TeDstFlowFilter}, false)
	})
}

func testTEDisabledTraffic(t *testing.T, args *testArgs) {
	t.Logf("Configure static route for outer header IP prefix %v in Default VRF with NH to Port8", encapVrfIPv4Prefix)
	configStaticRoute(t, args.dut, encapVrfIPv4Prefix, AtePorts["port8"].IPv4, "", "", false)
	gnmiClient := args.dut.RawAPIs().GNMI(t)

	t.Run("Verify TE disabled IPinIP traffic with No Match SRC IP", func(t *testing.T) {
		dstPorts := []string{atePort8.Name}
		flow1, details1 := flow2V4.createTrafficFlow(t, args.ate, "ipInIPFlowSrcNoMatch", "IPv4",
			encapIPv4FlowIP, noMatchSrcIP, dscpEncapNoMatch, dstPorts)
		flow2, details2 := flow2V6.createTrafficFlow(t, args.ate, "ipv6InIPFlowSrcNoMatch", "IPv6",
			encapIPv4FlowIP, noMatchSrcIP, dscpEncapNoMatch, dstPorts)

		trafficFlow := []*ondatra.Flow{flow1, flow2}
		flowDetails := map[string]aftUtil.FlowDetails{
			flow1.Name(): details1,
			flow2.Name(): details2,
		}

		// 1) Get pre-traffic counters
		preCounters, err := aftUtil.GetAftCountersSample(t, gnmiClient, sampleInterval, collectTime)
		if err != nil {
			t.Fatalf("Failed to get pre-counters via poll: %v", err)
		}

		sendTraffic(t, args.ate, trafficFlow, flowDetails)

		// 3) Get post-traffic counters
		postCounters, err := aftUtil.GetAftCountersSample(t, gnmiClient, sampleInterval, collectTime)
		if err != nil {
			t.Fatalf("GetAftCountersSample error: %v", err)
		}

		results := aftUtil.BuildAftPrefixChain(t, args.dut, preCounters, postCounters)
		aftUtil.AftCounterResults(t, flowDetails, results, args.aftValidationType, len(postCounters),
			aftCountertolerance, "Transit")

		t.Logf("Validate Rx Traffic on Dest Ports %v & packet is IPinIP", dstPorts)
		args.validateTrafficFlows(t, trafficFlow, []string{noMatchSrcEncapDstFilter}, false)
	})

	t.Run("Verify v4 traffic with Transit_Reapired SRC IP", func(t *testing.T) {
		dstPorts := []string{atePort8.Name}
		flow1, details1 := flow1V4.createTrafficFlow(t, args.ate, "ipv4FlowSrc111", "",
			encapIPv4FlowIP, ipv4OuterSrc111, dscpEncapNoMatch, dstPorts)
		flow2, details2 := flow1V6.createTrafficFlow(t, args.ate, "ipv4FlowSrc222", "",
			encapIPv4FlowIP, ipv4OuterSrc222, dscpEncapNoMatch, dstPorts)

		trafficFlow := []*ondatra.Flow{flow1, flow2}
		flowDetails := map[string]aftUtil.FlowDetails{
			flow1.Name(): details1,
			flow2.Name(): details2,
		}

		// 1) Get pre-traffic counters
		preCounters, err := aftUtil.GetAftCountersSample(t, gnmiClient, sampleInterval, collectTime)
		if err != nil {
			t.Fatalf("Failed to get pre-counters via poll: %v", err)
		}

		sendTraffic(t, args.ate, trafficFlow, flowDetails)

		// 3) Get post-traffic counters
		postCounters, err := aftUtil.GetAftCountersSample(t, gnmiClient, sampleInterval, collectTime)
		if err != nil {
			t.Fatalf("GetAftCountersSample error: %v", err)
		}

		results := aftUtil.BuildAftPrefixChain(t, args.dut, preCounters, postCounters)
		aftUtil.AftCounterResults(t, flowDetails, results, args.aftValidationType, len(postCounters),
			aftCountertolerance, "Repaired")

		t.Logf("Validate Rx Traffic on Dest Ports %v & packet is v4 packet with Next header UDP", dstPorts)
		args.validateTrafficFlows(t, trafficFlow, []string{strconv.Itoa(nhUdpProtocol)}, false)

	})

	t.Run("Remove Default route & verify traffic is Dropped", func(t *testing.T) {
		dstPorts := []string{atePort8.Name}
		flow1, details1 := flow1V4.createTrafficFlow(t, args.ate, "ipv4FlowSrc111", "",
			encapIPv4FlowIP, ipv4OuterSrc111, dscpEncapNoMatch, dstPorts)
		flow2, details2 := flow1V6.createTrafficFlow(t, args.ate, "ipv4FlowSrc222", "",
			encapIPv4FlowIP, ipv4OuterSrc222, dscpEncapNoMatch, dstPorts)

		trafficFlow := []*ondatra.Flow{flow1, flow2}
		flowDetails := map[string]aftUtil.FlowDetails{
			flow1.Name(): details1,
			flow2.Name(): details2,
		}

		t.Logf("Delete Static route for outer header IP prefix %v", encapVrfIPv4Prefix)
		configStaticRoute(t, args.dut, encapVrfIPv4Prefix, AtePorts["port8"].IPv4, "", "", true)

		// 1) Get pre-traffic counters
		preCounters, err := aftUtil.GetAftCountersSample(t, gnmiClient, sampleInterval, collectTime)
		if err != nil {
			t.Fatalf("Failed to get pre-counters via poll: %v", err)
		}

		sendTraffic(t, args.ate, trafficFlow, flowDetails)

		// 3) Get post-traffic counters
		postCounters, err := aftUtil.GetAftCountersSample(t, gnmiClient, sampleInterval, collectTime)
		if err != nil {
			t.Fatalf("GetAftCountersSample error: %v", err)
		}

		results := aftUtil.BuildAftPrefixChain(t, args.dut, preCounters, postCounters)
		aftUtil.AftCounterResults(t, flowDetails, results, args.aftValidationType, len(postCounters),
			aftCountertolerance, "Dropped")

		t.Log("Validate Traffic is dropped with No route in Default VRF")
		args.validateTrafficFlows(t, trafficFlow, []string{}, true)
	})
}

func testDecapEncap(t *testing.T, args *testArgs) {
	// Graceful close & flush
	defer args.gribiClient.Close(t)
	defer args.gribiClient.FlushAll(t)
	gnmiClient := args.dut.RawAPIs().GNMI(t)

	if err := args.gribiClient.Start(t); err != nil {
		t.Fatalf("gRIBI Connection can not be established")
	}

	electionID := args.gribiClient.LearnElectionID(t)
	args.gribiClient.Flush(t, electionID, vrfDecap)
	args.gribiClient.BecomeLeader(t)

	t.Logf("Program Decap entries for prefixes %v & %v in vrf %v",
		decapMixPrefix1, decapMixPrefix2, vrfDecap,
	)

	// 1) Program a decap next‐hop in the default NI
	args.gribiClient.AddNH(
		t, nh1ID, "Decap",
		deviations.DefaultNetworkInstance(args.dut),
		fluent.InstalledInFIB,
	)
	args.gribiClient.AddNHG(
		t, nhg1ID,
		map[uint64]uint64{nh1ID: 1},
		deviations.DefaultNetworkInstance(args.dut),
		fluent.InstalledInFIB,
	)

	// 2) Add IPv4 routes for decap
	args.gribiClient.AddIPv4(
		t, decapMixPrefix1, nhg1ID,
		vrfDecap,
		deviations.DefaultNetworkInstance(args.dut),
		fluent.InstalledInFIB,
	)
	args.gribiClient.AddIPv4(
		t, decapMixPrefix2, nhg1ID,
		vrfDecap,
		deviations.DefaultNetworkInstance(args.dut),
		fluent.InstalledInFIB,
	)

	t.Logf("Program Inner IPv6 entries for prefix %v in Encap VRFs %v & %v",
		encapVrfIPv6Prefix, vrfEncapA, vrfEncapB,
	)
	// TODO: Add IPv6 entries in Encap VRFs once that PR is merged

	//
	// TEST SCENARIO #1: "Verify Decap & Encap with DSCP_A"
	//
	t.Run("Verify Decap & Encap with DSCP_A", func(t *testing.T) {
		dstPorts := []string{atePort2.Name, atePort3.Name, atePort4.Name, atePort6.Name}
		// (TODO: Add IPv6inIP Flow after the additional entries in Encap VRF.)

		// Build IPv4 flow referencing "192.51.130.64" and outer src = 198.51.100.222
		flow1, details1 := flow2V4.createTrafficFlow(
			t, args.ate,
			"ipInIPFDecapEncap", // flow name
			"IPv4",
			"192.51.130.64", // outer IP DST
			ipv4OuterSrc222, // outer IP SRC
			dscpEncapA1,     // DSCP
			dstPorts,
		)

		trafficFlow := []*ondatra.Flow{flow1}
		flowDetails := map[string]aftUtil.FlowDetails{
			flow1.Name(): details1,
		}
		// 1) Get pre-traffic counters
		preCounters, err := aftUtil.GetAftCountersSample(t, gnmiClient, sampleInterval, collectTime)
		if err != nil {
			t.Fatalf("Failed to get pre-counters via poll: %v", err)
		}

		sendTraffic(t, args.ate, trafficFlow, flowDetails)

		// 3) Get post-traffic counters
		postCounters, err := aftUtil.GetAftCountersSample(t, gnmiClient, sampleInterval, collectTime)
		if err != nil {
			t.Fatalf("GetAftCountersSample error: %v", err)
		}

		results := aftUtil.BuildAftPrefixChain(t, args.dut, preCounters, postCounters)
		aftUtil.AftCounterResults(t, flowDetails, results, args.aftValidationType, len(postCounters),
			aftCountertolerance, "Decap")

		t.Logf("Validate Rx Traffic on Dest Ports %v & packet is IPinIP", dstPorts)
		args.validateTrafficFlows(t, trafficFlow, []string{src111TeDstFlowFilter}, false)

		t.Logf("Validate Hierarchical Traffic on Dest Ports %v", dstPorts)
		weights := []float64{0.015625, 0.046875, 0.1875, 0.75}
		validateTrafficDistribution(t, args.ate, weights, dstPorts)

		// 3) Check DSCP
		t.Logf("Validate DSCP value %v for egress IPinIP traffic", dscpEncapA1)
		flow2, details2 := flow3V4.createTrafficFlow(
			t, args.ate,
			"ipInIPFDecapEncap", "IPv4",
			"192.51.130.64", ipv4OuterSrc222,
			dscpEncapA1, dstPorts,
		)

		trafficFlow = []*ondatra.Flow{flow2}
		flowDetails = map[string]aftUtil.FlowDetails{
			flow2.Name(): details2,
		}

		sendTraffic(t, args.ate, trafficFlow, flowDetails)

		// DSCP decimal is 5 bits in binary vs. 7 bits in the ATE
		// we shift left by 2 bits to match
		args.validateTrafficFlows(
			t, trafficFlow,
			[]string{strconv.Itoa(dscpEncapA1 << 2)},
			false,
		)
	})

	//
	// TEST SCENARIO #2: "Verify Decap & Encap with DSCP_B"
	//
	t.Run("Verify Decap & Encap with DSCP_B", func(t *testing.T) {
		dstPorts := []string{atePort2.Name, atePort3.Name, atePort4.Name, atePort6.Name}
		// (TODO: IPv6inIP Flow after additional entries in Encap VRF.)

		flow1, details1 := flow2V4.createTrafficFlow(
			t, args.ate,
			"ipInIPFDecapEncap",
			"IPv4",
			"192.51.130.64", ipv4OuterSrc222,
			dscpEncapB1, dstPorts,
		)

		trafficFlow := []*ondatra.Flow{flow1}
		flowDetails := map[string]aftUtil.FlowDetails{
			flow1.Name(): details1,
		}

		// 1) Get pre-traffic counters
		preCounters, err := aftUtil.GetAftCountersSample(t, gnmiClient, sampleInterval, collectTime)
		if err != nil {
			t.Fatalf("Failed to get pre-counters via poll: %v", err)
		}

		sendTraffic(t, args.ate, trafficFlow, flowDetails)

		// 3) Get post-traffic counters
		postCounters, err := aftUtil.GetAftCountersSample(t, gnmiClient, sampleInterval, collectTime)
		if err != nil {
			t.Fatalf("GetAftCountersSample error: %v", err)
		}

		results := aftUtil.BuildAftPrefixChain(t, args.dut, preCounters, postCounters)
		aftUtil.AftCounterResults(t, flowDetails, results, args.aftValidationType, len(postCounters),
			aftCountertolerance, "Decap")

		t.Logf("Validate Rx Traffic on Dest Ports %v & packet is IPinIP", dstPorts)
		args.validateTrafficFlows(t, trafficFlow, []string{src111TeDstFlowFilter}, false)

		t.Logf("Validate Hierarchical Traffic on Dest Ports %v", dstPorts)
		weights := []float64{0.046875, 0.140625, 0.5625, 0.25}
		validateTrafficDistribution(t, args.ate, weights, dstPorts)

		t.Logf("Validate DSCP value %v for egress IPinIP traffic", dscpEncapB1)
		flow2, details2 := flow3V4.createTrafficFlow(
			t, args.ate,
			"ipInIPFDecapEncap", "IPv4",
			"192.51.130.64", ipv4OuterSrc222,
			dscpEncapB1, dstPorts,
		)

		trafficFlow = []*ondatra.Flow{flow2}
		flowDetails = map[string]aftUtil.FlowDetails{
			flow2.Name(): details2,
		}

		// 1) Get pre-traffic counters
		preCounters1, err := aftUtil.GetAftCountersSample(t, gnmiClient, sampleInterval, collectTime)
		if err != nil {
			t.Fatalf("Failed to get pre-counters via poll: %v", err)
		}

		sendTraffic(t, args.ate, trafficFlow, flowDetails)

		// 3) Get post-traffic counters
		postCounters1, err := aftUtil.GetAftCountersSample(t, gnmiClient, sampleInterval, collectTime)
		if err != nil {
			t.Fatalf("GetAftCountersSample error: %v", err)
		}

		results1 := aftUtil.BuildAftPrefixChain(t, args.dut, preCounters1, postCounters1)
		aftUtil.AftCounterResults(t, flowDetails, results1, args.aftValidationType, len(postCounters),
			aftCountertolerance, "Decap")

		// shift left by 2 bits again
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

func (args *testArgs) validateTrafficFlows(
	t *testing.T,
	flows []*ondatra.Flow,
	reqFilterList []string,
	wantLoss bool,
) {
	// We'll store the results here.
	type flowResult struct {
		FlowName   string
		WantedLoss string
		ActualLoss string
		Result     string
	}

	var results []flowResult

	// Provide an overview of what scenario we are testing.
	if wantLoss {
		t.Log("Scenario: expecting FULL (100%) traffic loss.")
	} else {
		t.Log("Scenario: expecting NO traffic loss (0%).")
	}

	for _, flow := range flows {
		t.Logf("Validating flow %q ...", flow.Name())

		flowPath := gnmi.OC().Flow(flow.Name())
		lossPct := gnmi.Get(t, args.ate, flowPath.LossPct().State())

		var result flowResult
		result.FlowName = flow.Name()

		// Evaluate the "WantedLoss" and "ActualLoss" fields (just for logging/table).
		if wantLoss {
			result.WantedLoss = "100%"
		} else {
			result.WantedLoss = "0%"
		}
		result.ActualLoss = fmt.Sprintf("%.2f%%", lossPct)

		if wantLoss {
			t.Logf("Checking if flow %q has 100%% loss ...", flow.Name())
			if lossPct < 100 {
				msg := fmt.Sprintf("FAIL: Flow %q LossPct got %g, want 100", flow.Name(), lossPct)
				t.Error(msg)
				result.Result = "FAIL"
			} else {
				t.Logf("PASS: Flow %q is at 100%% loss as expected.", flow.Name())
				result.Result = "PASS"
			}
		} else {
			t.Logf("Checking if flow %q has 0%% loss ...", flow.Name())
			if lossPct > 0 {
				t.Errorf("WARNING: Flow %q LossPct got %g, want 0", flow.Name(), lossPct)
				result.Result = "FAIL"
			} else {
				t.Logf("PASS: Flow %q has 0%% loss as expected.", flow.Name())
				result.Result = "PASS"
			}
		}

		// Only check additional counters/filters if no loss expected
		if flow.Name() != "ipv6InIPFlowDecap" && !wantLoss {
			t.Log("Verifying protocol and counters for packets received on ATE...")
			egressTrackPath := flowPath.EgressTrackingAny()
			egressTrackState := gnmi.GetAll(t, args.ate, egressTrackPath.State())

			if len(egressTrackState) == 0 {
				t.Logf("No egress tracking found for flow %q; skipping filter checks.", flow.Name())
			} else {
				getFlowFilter := egressTrackState[0].GetFilter()
				if slices.Contains(reqFilterList, getFlowFilter) {
					t.Log("PASS: Egress tracking filter matches expected Rx packet filter on ATE.")
				} else {
					t.Logf("WARNING: EgressTracking filter got %q, want one of %q", getFlowFilter, reqFilterList)
				}
			}

			inPkts := gnmi.Get(t, args.ate, flowPath.Counters().InPkts().State())
			ingressTrackPath := flowPath.IngressTrackingAny()
			ingressTrackCounters := gnmi.GetAll(t, args.ate, ingressTrackPath.Counters().InPkts().State())

			var ingressTrackPackets uint64
			for _, pktCount := range ingressTrackCounters {
				if pktCount == 0 {
					t.Log("Encountered 0 in ingress tracking; skipping accumulation.")
					break
				}
				ingressTrackPackets += pktCount
			}

			if ingressTrackPackets != inPkts {
				t.Logf("WARNING: IngressTracking counter in-pkts got %d, want %d", ingressTrackPackets, inPkts)
			} else {
				t.Logf("PASS: IngressTracking counter in-pkts matches expected: %d", inPkts)
			}
		}

		results = append(results, result)
	}

	// Finally, print a summary table.
	t.Log("------------------------------------------------------------------------------")
	t.Logf("%-30s %-15s %-15s %-10s", "FLOW NAME", "WANTED LOSS", "ACTUAL LOSS", "RESULT")
	t.Log("------------------------------------------------------------------------------")
	for _, r := range results {
		t.Logf("%-30s %-15s %-15s %-10s", r.FlowName, r.WantedLoss, r.ActualLoss, r.Result)
	}
	t.Log("------------------------------------------------------------------------------")
}
