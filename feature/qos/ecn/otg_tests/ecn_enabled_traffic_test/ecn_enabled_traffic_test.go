// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ecn_enabled_traffic_test

import (
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/entity-naming/entname"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otg_helpers/otg_validation_helpers"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygnmi/ygnmi"
)

const (
	// DP-1.12 Direct L3 Addressing prefix lengths (IPv4 /31 and IPv6 /126) and interface MTU (9236).
	ipv4PrefixLen = 31
	ipv6PrefixLen = 126
	defaultMTU    = 9236

	// DP-1.12 QoS policy, classifier, and WRED ECN queue-management profile constants.
	ecnProfileName          = "800GECNProfile"
	ecnDisabledProfileName  = "800GECNDisabledProfile"
	schedulerPolicyName     = "scheduler"
	classifierIPv4Name      = "dscp_based_classifier_ipv4"
	classifierIPv6Name      = "dscp_based_classifier_ipv6"
	ecnMinThresholdBytes    = uint64(80000)
	ecnMaxThresholdDefault  = uint64(math.MaxUint64)
	ecnMaxDropProbPct       = uint8(1)
	ecnWeightDefault        = uint32(0)
	defaultFrameSize        = uint32(1000)
	maxAllowedLatencyNs     = uint64(100000)
	ecnNotECT               = uint8(0) // 0b00 Not-ECT
	ecnECT1                 = uint8(1) // 0b01 ECT(1)
	ecnECT0                 = uint8(2) // 0b10 ECT(0)
	ecnCE                   = uint8(3) // 0b11 Congestion Experienced (CE)
	lagSpeed1x800G          = uint32(800000)
	gnmiWatchTimeout        = 2 * time.Minute
	minSteadyStatePkts      = uint64(10000)
	udpSrcPortStart         = uint32(10000)
	udpDstPortStart         = uint32(20000)
	udpPortCount            = uint32(1000)
	defaultThroughputTolPct = 5.0
	counterTolFraction      = 0.05
	minCounterTolPkts       = uint64(200)
)

// DP-1.12 Procedure - Test environment setup:
// Direct L3 Addressing (IPv4 /31 and IPv6 /126):
//   - Ingress 1: DUT port-1 (198.51.100.0/31, 2001:db8::1/126) <-> ATE port-1 (198.51.100.1/31, 2001:db8::2/126)
//   - Ingress 2: DUT port-2 (198.51.100.2/31, 2001:db8::5/126) <-> ATE port-2 (198.51.100.3/31, 2001:db8::6/126)
//   - Egress:    DUT port-3 or LAG_1 (198.51.100.4/31, 2001:db8::9/126) <-> ATE port-3 (198.51.100.5/31, 2001:db8::a/126)
//   - LAG Member 2: DUT port-4 <-> ATE port-4 (used as second LACP member of LAG_1 in DP-1.12.3 Steps 4-5)
var (
	dutPort1 = attrs.Attributes{
		Name:    "port1",
		Desc:    "DUT Port 1 Ingress 1",
		IPv4:    "198.51.100.0",
		IPv6:    "2001:db8::1",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
		MTU:     defaultMTU,
	}
	atePort1 = attrs.Attributes{
		Name:    "port1",
		Desc:    "ATE Port 1 Ingress 1",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "198.51.100.1",
		IPv6:    "2001:db8::2",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
		MTU:     defaultMTU,
	}

	dutPort2 = attrs.Attributes{
		Name:    "port2",
		Desc:    "DUT Port 2 Ingress 2",
		IPv4:    "198.51.100.2",
		IPv6:    "2001:db8::5",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
		MTU:     defaultMTU,
	}
	atePort2 = attrs.Attributes{
		Name:    "port2",
		Desc:    "ATE Port 2 Ingress 2",
		MAC:     "02:00:01:01:01:02",
		IPv4:    "198.51.100.3",
		IPv6:    "2001:db8::6",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
		MTU:     defaultMTU,
	}

	dutPort3 = attrs.Attributes{
		Name:    "port3",
		Desc:    "DUT Port 3 Egress",
		IPv4:    "198.51.100.4",
		IPv6:    "2001:db8::9",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
		MTU:     defaultMTU,
	}
	atePort3 = attrs.Attributes{
		Name:    "port3",
		Desc:    "ATE Port 3 Egress",
		MAC:     "02:00:01:01:01:03",
		IPv4:    "198.51.100.5",
		IPv6:    "2001:db8::a",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
		MTU:     defaultMTU,
	}

	dutPort4 = attrs.Attributes{
		Name: "port4",
		Desc: "DUT Port 4 Secondary LAG Member",
		MTU:  defaultMTU,
	}
)

// testEnv encapsulates the initialized DUT/ATE testbed state for DP-1.12.
type testEnv struct {
	dut              *ondatra.DUTDevice
	ate              *ondatra.ATEDevice
	p1               *ondatra.Port
	p2               *ondatra.Port
	p3               *ondatra.Port
	p4               *ondatra.Port
	aggID            string
	queues           *entname.CommonTrafficQueueNames
	top              gosnappi.Config
	baseQoS          *oc.Qos
	wantMinThreshold uint64
	wantMaxThreshold uint64
	ingressIntfNames []string
}

// trafficFlowSpec defines the parameters for a single OTG traffic stream towards ATE port-3.
type trafficFlowSpec struct {
	name                  string
	queue                 string
	inputIntf             attrs.Attributes
	trafficRatePct        float64
	expectedThroughputPct float32
	frameSize             uint32
	dscp                  uint8
	ecnIn                 uint8
	wantECNHex            string
	wantMinCEFraction     float64
	expectTailDrops       bool
	checkMaxLatencyNs     uint64
	ipType                string // "IPv4" or "IPv6"
}

// TestMain initializes the Ondatra test runner and binds the 4-port testbed.
// README: Testbed type (atedut_4.testbed).
func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// setupEnvironment prepares the DUT/ATE interfaces and builds the baseline QoS model in memory.
// README: Procedure -> Test environment setup.
// Summary: Configures 4 ports with L3 IPs (/31 IPv4, /126 IPv6, MTU 9236), verifies ARP/ND
// neighbor resolution via gNMI.Watch, and prepares the 7-queue QoS + 800GECNProfile config.
func setupEnvironment(t *testing.T) *testEnv {
	t.Helper()
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")
	p3 := dut.Port(t, "port3")
	p4 := dut.Port(t, "port4")

	aggID := netutil.NextAggregateInterface(t, dut)
	queues := netutil.CommonTrafficQueues(t, dut)
	// DP-1.12.1 Step 1: Input DSCP classifiers are attached on ingress interfaces DUT port-1 and DUT port-2.
	ingressIntfs := []string{p1.Name(), p2.Name()}

	configureDUTBaseInterfaces(t, dut, []*ondatra.Port{p1, p2, p3, p4})
	top := configureOTGPhysical(t, ate)
	verifyNeighborsResolved(t, ate, top)

	baseQoS, wantMin, wantMax := buildBaseQoSConfig(t, dut, queues, p3.Name(), ingressIntfs)

	return &testEnv{
		dut:              dut,
		ate:              ate,
		p1:               p1,
		p2:               p2,
		p3:               p3,
		p4:               p4,
		aggID:            aggID,
		queues:           queues,
		top:              top,
		baseQoS:          baseQoS,
		wantMinThreshold: wantMin,
		wantMaxThreshold: wantMax,
		ingressIntfNames: ingressIntfs,
	}
}

// TestECNEnabledTraffic runs the full ECN-enabled QoS traffic test suite.
// README: DP-1.12.1 through DP-1.12.6.
// Summary: Verifies WRED ECN marking (800GECNProfile) and scheduler ("scheduler") across all
// 7 queues on physical ports (DP-1.12.1, DP-1.12.2) and LACP LAG_1 (DP-1.12.3), negative
// Not-ECT / disabled-ECN cases (DP-1.12.4, DP-1.12.5), and teardown cleanup (DP-1.12.6).
func TestECNEnabledTraffic(t *testing.T) {
	env := setupEnvironment(t)
	dut := env.dut
	ate := env.ate
	p3, p4 := env.p3, env.p4
	aggID := env.aggID
	queues := env.queues

	// Register DP-1.12.6 safety cleanup via t.Cleanup so DUT and ATE are restored even on unexpected failure.
	cleanedUp := false
	t.Cleanup(func() {
		if !cleanedUp {
			teardownAndRestoreBaseline(t, dut, ate, aggID, queues)
		}
	})

	// =========================================================================
	// DP-1.12.1 - Validate Physical Interface QoS Profile State and Non-Congested Traffic Baseline
	// =========================================================================
	t.Run("DP-1.12.1_Validate_Physical_Interface_QoS_Profile_State_And_NonCongested_Traffic_Baseline", func(t *testing.T) {
		// DP-1.12.1 Step 1: Attach input DSCP classifiers (/qos/interfaces/interface/input/classifiers/classifier/config/name)
		// on ingress interfaces DUT port-1 and DUT port-2, and attach egress scheduler policy "scheduler"
		// (/qos/interfaces/interface/output/scheduler-policy/config/name) and the ECN queue-management profile
		// (800GECNProfile) across all 7 queues on physical egress interface DUT port-3.
		// DP-1.12.1 Step 2: Push configuration to DUT using gNMI.Set with REPLACE option.
		t.Logf("DP-1.12.1 Step 1-2: Pushing QoS classifiers on %v and scheduler + %s across all 7 queues on physical interface %s via gNMI.Replace",
			env.ingressIntfNames, ecnProfileName, p3.Name())
		gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), env.baseQoS)

		// DP-1.12.1 Step 3: Validate operational state via gNMI.Subscribe (ON_CHANGE via gnmi.Watch / gnmi.Await):
		//   - DUT port-3 reports /interfaces/interface/ethernet/state/port-speed matching dynamic port speed (100G, 400G, or 800G).
		//   - DUT port-1 and DUT port-2 report /qos/interfaces/interface/input/classifiers/classifier/state/name
		//     as dscp_based_classifier_ipv4 and dscp_based_classifier_ipv6.
		//   - DUT port-3 reports /qos/interfaces/interface/output/scheduler-policy/state/name as "scheduler" and
		//     /qos/interfaces/interface/output/queues/queue/state/queue-management-profile as 800GECNProfile across all 7 queues.
		//   - WRED profile reports min-threshold=80000, max-threshold, enable-ecn=true, drop=false,
		//     max-drop-probability-percent=1, and weight=0.
		t.Logf("DP-1.12.1 Step 3: Validating physical port-speed, input classifiers, egress scheduler, and %s WRED state via gNMI.Subscribe", ecnProfileName)
		verifyPhysicalPortSpeedState(t, dut, p3)
		cfgplugins.VerifyQoSInputClassifierState(t, dut, buildInputClassifierBindings(env.ingressIntfNames), gnmiWatchTimeout)
		verifyQoSEgressState(t, dut, p3.Name(), queues, ecnProfileName, env.wantMinThreshold, env.wantMaxThreshold, true)

		// DP-1.12.1 Step 4: Send sub-threshold IPv4 and IPv6 traffic (80% aggregate line rate =
		// 40% from ATE port-1 and 40% from ATE port-2) with ECT(0) (0b10) across all 7 queues
		// towards ATE port-3 (198.51.100.5 and 2001:db8::a).
		// DP-1.12.1 Step 5: Verify non-congested baseline counters and ECN bits:
		//   - /qos/interfaces/interface/output/queues/queue/state/dropped-pkts and dropped-octets are 0.
		//   - /qos/interfaces/interface/output/queues/queue/state/transmit-pkts and transmit-octets match ATE Rx counters.
		//   - 100% of received IPv4 and IPv6 packets on ATE port-3 retain ECN = 0b10 with 0 packets marked CE (0b11).
		//   - Latency is < 100000ns.
		t.Logf("DP-1.12.1 Step 4-5: Sending sub-threshold 80%% aggregate line rate (40%% + 40%%) IPv4 and IPv6 ECT(0) traffic across all 7 queues")
		for _, ipType := range []string{"IPv4", "IPv6"} {
			flows := buildSubThreshold80PctFlows(queues, ipType)
			runAndVerifyTrafficFlows(t, dut, ate, env.top, p3.Name(), flows)
		}
	})

	// =========================================================================
	// DP-1.12.2 - Validate Physical Interface Oversubscribed Per-Queue ECN Marking
	// =========================================================================
	t.Run("DP-1.12.2_Validate_Physical_Interface_Oversubscribed_PerQueue_ECN_Marking", func(t *testing.T) {
		// DP-1.12.2 Step 1: Testing one queue at a time (NC1, AF4, AF3, AF2, AF1, BE0, and BE1),
		// send oversubscribed IPv4 and IPv6 traffic (101% of line rate = 51% from ATE port-1 and 50% from ATE port-2)
		// with ECT(0) (0b10) and ECT(1) (0b01) (as well as CE 0b11 and Not-ECT 0b00 per Procedure baseline) towards ATE port-3.
		// DP-1.12.2 Step 2: Verify per-queue ECN marking and counters:
		//   - Once queue depth exceeds min-threshold (80000 bytes, after initial ~80-packet queue fill),
		//     >= 99% of received IPv4 and IPv6 packets on ATE port-3 have ECN = 0b11 (CE) and DSCP is unchanged.
		//   - Queue /qos/interfaces/interface/output/queues/queue/state/transmit-pkts, transmit-octets,
		//     dropped-pkts, and dropped-octets counters match ATE counters.
		t.Logf("DP-1.12.2 Step 1-2: Running per-queue 101%% oversubscribed IPv4 and IPv6 traffic across all 7 queues on physical interface %s", p3.Name())
		runPerQueueOversubscribedSuite(t, dut, ate, env.top, p3.Name(), queues)

		// DP-1.12 Procedure baseline check on physical interface: when both ECT and CE bits are cleared (0b00 Not-ECT)
		// across queues NC1..BE1, verify egress ECN bits are NOT set (0b00) and dropped-pkts is incremented.
		t.Logf("DP-1.12.2 Procedure Baseline: Verifying oversubscribed Not-ECT (0b00) traffic across all 7 queues on physical interface %s retains 0b00 and increments dropped-pkts", p3.Name())
		runPerQueueNotECTSuite(t, dut, ate, env.top, p3.Name(), queues)
	})

	// =========================================================================
	// DP-1.12.3 - Validate LACP Port-Channel (LAG_1) ECN Marking and Member Failover
	// =========================================================================
	t.Run("DP-1.12.3_Validate_LACP_PortChannel_LAG1_ECN_Marking_And_Member_Failover", func(t *testing.T) {
		// DP-1.12.3 Step 1: Configure LAG_1 (ieee8023adLag) with LACP (ACTIVE, FAST), assign DUT port-3 to LAG_1
		// via /interfaces/interface/ethernet/config/aggregate-id, move the egress L3 subinterface
		// (198.51.100.4/31, 2001:db8::9/126) to LAG_1, and attach scheduler
		// (/qos/interfaces/interface/output/scheduler-policy/config/name) and the ECN queue-management profile
		// (800GECNProfile, /qos/interfaces/interface/output/queues/queue/config/queue-management-profile) to LAG_1.
		t.Logf("DP-1.12.3 Step 1: Configuring LACP Port-Channel %s (LAG_1) with member %s and attaching scheduler + %s", aggID, p3.Name(), ecnProfileName)
		configureDUTLAG(t, dut, aggID, []*ondatra.Port{p3})
		env.top = configureOTGLAG(t, ate, aggID, []*ondatra.Port{ate.Port(t, "port3")}, true)
		verifyNeighborsResolved(t, ate, env.top)

		qLag, wantMin, wantMax := buildBaseQoSConfig(t, dut, queues, aggID, env.ingressIntfNames)
		env.wantMinThreshold = wantMin
		env.wantMaxThreshold = wantMax
		gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), qLag)

		// DP-1.12.3 Step 2: Push configuration to DUT and validate via gNMI.Subscribe (gnmi.Watch / gnmi.Await):
		//   - /interfaces/interface/aggregation/state/lag-speed matches the testbed's dynamic single-member
		//     port speed in Mbps (100000 for 100G, 400000 for 400G, or 800000 for 800G).
		//   - /qos/interfaces/interface/output/queues/queue/state/queue-management-profile reports
		//     the configured ECN profile (800GECNProfile) for all 7 queues on LAG_1.
		t.Logf("DP-1.12.3 Step 2: Validating %s lag-speed and %s state across all 7 queues via gNMI.Subscribe", aggID, ecnProfileName)
		expectedMemberSpeedMbps := expectedPortSpeedMbps(t, p3)
		verifyLAGSpeedState(t, dut, aggID, expectedMemberSpeedMbps)
		verifyQoSEgressState(t, dut, aggID, queues, ecnProfileName, wantMin, wantMax, true)

		// DP-1.12.3 Step 3: Repeat the per-queue (101% of line rate) oversubscribed IPv4 and IPv6 traffic streams
		// from DP-1.12.2 over LAG_1 and verify CE (0b11) marking and queue counter accuracy on LAG_1.
		t.Logf("DP-1.12.3 Step 3: Repeating per-queue (101%%) oversubscribed IPv4 and IPv6 traffic over %s", aggID)
		runPerQueueOversubscribedSuite(t, dut, ate, env.top, aggID, queues)

		// DP-1.12.3 Step 4: Add DUT port-4 (connected to ATE port-4) as a second member of LAG_1:
		//   - Verify /interfaces/interface/aggregation/state/lag-speed matches 2x the testbed's dynamic
		//     port speed in Mbps (200000 for 2x100G, 800000 for 2x400G, or 1600000 for 2x800G).
		//   - Send 150% of single-member line rate (75% from ATE port-1 + 75% from ATE port-2) IPv4 and IPv6
		//     ECT(0) (0b10) traffic in queue AF3, and verify 0 drops and 0 CE marks across the uncongested 2-member LAG.
		t.Logf("DP-1.12.3 Step 4: Expanding %s to 2 members (%s, %s), verifying 2x lag-speed, and sending 150%% IPv4 and IPv6 AF3 ECT(0) traffic",
			aggID, p3.Name(), p4.Name())
		configureDUTLAG(t, dut, aggID, []*ondatra.Port{p3, p4})
		env.top = configureOTGLAG(t, ate, aggID, []*ondatra.Port{ate.Port(t, "port3"), ate.Port(t, "port4")}, false)
		verifyNeighborsResolved(t, ate, env.top)
		verifyLAGSpeedState(t, dut, aggID, 2*expectedMemberSpeedMbps)

		for _, ipType := range []string{"IPv4", "IPv6"} {
			flows150PctUncongested := buildTwoPortQueueFlows("lag-2member-150pct-"+strings.ToLower(ipType), queues.AF3, 24, 75, 75, ecnECT0, "0x2", 0, false, ipType)
			runAndVerifyTrafficFlows(t, dut, ate, env.top, aggID, flows150PctUncongested)
		}

		// DP-1.12.3 Step 5: Bring ATE port-4 link down to trigger LAG member failover:
		//   - Verify /interfaces/interface/aggregation/state/lag-speed drops to 1x the testbed's dynamic
		//     port speed in Mbps (100000, 400000, or 800000) on surviving member DUT port-3.
		//   - Verify that the active 150% flow (IPv4 and IPv6) now oversubscribes the single surviving member,
		//     exceeds min-threshold (80000 bytes), and marks ECN = 0b11 (CE) on ATE port-3.
		//   - Restore ATE port-4 and detach DUT port-4 from LAG_1 so LAG_1 returns to 1x the testbed's
		//     dynamic port speed (100000, 400000, or 800000).
		t.Logf("DP-1.12.3 Step 5: Triggering LAG member failover by bringing ATE port-4 link down during 150%% AF3 IPv4 and IPv6 traffic")
		runActiveFailover150PctTraffic(t, dut, ate, env.top, aggID, queues.AF3, expectedMemberSpeedMbps)

		t.Logf("DP-1.12.3 Step 5: Restoring ATE port-4 and detaching %s from %s back to 1x member speed (%d Mbps)", p4.Name(), aggID, expectedMemberSpeedMbps)
		setOTGPortLinkState(t, ate, "port4", gosnappi.StatePortLinkState.UP)
		detachPortFromLAGAndRestoreBase(t, dut, p4)
		configureDUTLAG(t, dut, aggID, []*ondatra.Port{p3})
		env.top = configureOTGLAG(t, ate, aggID, []*ondatra.Port{ate.Port(t, "port3")}, true)
		verifyNeighborsResolved(t, ate, env.top)
		verifyLAGSpeedState(t, dut, aggID, expectedMemberSpeedMbps)
	})

	// =========================================================================
	// DP-1.12.4 - Validate Non-ECN Capable (Not-ECT) Traffic Under Congestion (Negative)
	// =========================================================================
	t.Run("DP-1.12.4_Validate_NonECN_Capable_NotECT_Traffic_Under_Congestion", func(t *testing.T) {
		// DP-1.12.4 Step 1: Send oversubscribed (101% of line rate = 51% from ATE port-1 and 50% from ATE port-2)
		// IPv4 and IPv6 traffic in queue AF3 with ECN bits set to Not-ECT (0b00) towards LAG_1.
		// DP-1.12.4 Step 2: Verify that 100% of received IPv4 and IPv6 packets on ATE port-3 retain ECN = 0b00
		// (0 packets marked CE) and excess traffic is tail-dropped (/qos/interfaces/interface/output/queues/queue/state/dropped-pkts > 0).
		t.Logf("DP-1.12.4 Step 1-2: Sending 101%% oversubscribed Not-ECT (0b00) IPv4 and IPv6 traffic in queue %s towards %s", queues.AF3, aggID)
		for _, ipType := range []string{"IPv4", "IPv6"} {
			notECTFlows := buildTwoPortQueueFlows("lag-not-ect-"+strings.ToLower(ipType), queues.AF3, 24, 51, 50, ecnNotECT, "0x0", 0, true, ipType)
			runAndVerifyTrafficFlows(t, dut, ate, env.top, aggID, notECTFlows)
		}
	})

	// =========================================================================
	// DP-1.12.5 - Validate Missing ECN Profile / Disabled ECN Behavior and Recovery (Negative)
	// =========================================================================
	t.Run("DP-1.12.5_Validate_Missing_ECN_Profile_Disabled_ECN_Behavior_And_Recovery", func(t *testing.T) {
		// DP-1.12.5 Step 1: Remove the ECN profile (800GECNProfile) from queue AF3 on LAG_1
		// (or set /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/enable-ecn to false)
		// using gNMI.Set, while keeping the ECN profile enabled on AF2.
		t.Logf("DP-1.12.5 Step 1: Removing %s (or setting enable-ecn=false) on queue %s of %s while keeping %s enabled",
			ecnProfileName, queues.AF3, aggID, queues.AF2)
		disableECNOnQueueAF3(t, dut, aggID, queues.AF3)
		cfgplugins.VerifyQoSQueueProfileBinding(t, dut, aggID, queues.AF2, ecnProfileName, gnmiWatchTimeout)

		// DP-1.12.5 Step 2: Send oversubscribed (101% of line rate) IPv4 and IPv6 ECT(0) (0b10) traffic to AF3 and AF2.
		// DP-1.12.5 Step 3: Verify that AF3 tail-drops excess traffic (/qos/interfaces/interface/output/queues/queue/state/dropped-pkts > 0)
		// with 0 CE (0b11) marks (100% remain 0b10), while AF2 continues marking >= 99% of received packets with CE (0b11).
		t.Logf("DP-1.12.5 Step 2-3: Sending 101%% IPv4 and IPv6 ECT(0) traffic to AF3 (ECN disabled) and AF2 (ECN enabled) on %s", aggID)
		for _, ipType := range []string{"IPv4", "IPv6"} {
			suffix := strings.ToLower(ipType)
			af3DisabledFlows := buildTwoPortQueueFlows("af3-ecn-disabled-"+suffix, queues.AF3, 24, 51, 50, ecnECT0, "0x2", 0, true, ipType)
			runAndVerifyTrafficFlows(t, dut, ate, env.top, aggID, af3DisabledFlows)

			af2EnabledFlows := buildTwoPortQueueFlows("af2-ecn-enabled-"+suffix, queues.AF2, 16, 51, 50, ecnECT0, "0x3", 0.99, false, ipType)
			runAndVerifyTrafficFlows(t, dut, ate, env.top, aggID, af2EnabledFlows)
		}

		// DP-1.12.5 Step 4: Re-apply the ECN profile (enable-ecn = true, 800GECNProfile) to AF3,
		// push to DUT, and verify CE (0b11) marking is restored for both IPv4 and IPv6.
		t.Logf("DP-1.12.5 Step 4: Re-applying %s (enable-ecn=true) to queue %s on %s and verifying IPv4 and IPv6 CE (0b11) recovery",
			ecnProfileName, queues.AF3, aggID)
		gnmi.Replace(t, dut, gnmi.OC().Qos().Interface(aggID).Output().Queue(queues.AF3).QueueManagementProfile().Config(), ecnProfileName)
		gnmi.Delete(t, dut, gnmi.OC().Qos().QueueManagementProfile(ecnDisabledProfileName).Config())
		cfgplugins.VerifyQoSQueueProfileBinding(t, dut, aggID, queues.AF3, ecnProfileName, gnmiWatchTimeout)

		for _, ipType := range []string{"IPv4", "IPv6"} {
			af3RecoveredFlows := buildTwoPortQueueFlows("af3-ecn-recovered-"+strings.ToLower(ipType), queues.AF3, 24, 51, 50, ecnECT0, "0x3", 0.99, false, ipType)
			runAndVerifyTrafficFlows(t, dut, ate, env.top, aggID, af3RecoveredFlows)
		}
	})

	// =========================================================================
	// DP-1.12.6 - Teardown and Cleanup Verification
	// =========================================================================
	t.Run("DP-1.12.6_Teardown_And_Cleanup_Verification", func(t *testing.T) {
		// DP-1.12.6 Step 1: Detach the ECN profile (800GECNProfile) and scheduler from LAG_1,
		// delete the ECN profile and LAG_1, and restore DUT port-1..DUT port-4 and ATE port-1..ATE port-4
		// to their baseline state using gNMI.Set.
		// DP-1.12.6 Step 2: Verify via gNMI.Get and gNMI.Subscribe that
		// /qos/interfaces/interface/output/queues/queue/state/queue-management-profile no longer returns
		// the ECN profile (800GECNProfile) and all ports report /interfaces/interface/state/oper-status as UP.
		t.Logf("DP-1.12.6 Step 1-2: Executing teardown and verifying clean state across all interfaces via gNMI.Get and gNMI.Subscribe")
		teardownAndRestoreBaseline(t, dut, ate, aggID, queues)
		cleanedUp = true
	})
}

// configureDUTBaseInterfaces configures physical port speeds, MTU 9236, and L3 IPs on the DUT.
// README: Procedure -> Test environment setup (Topology and Port Speed, Direct L3 Addressing).
// Summary: Sets dynamic port speed (100G/400G/800G) on DUT port-1..port-4 and assigns IPv4 (/31)
// and IPv6 (/126) addresses on port-1..port-3 in the default network instance.
func configureDUTBaseInterfaces(t *testing.T, dut *ondatra.DUTDevice, ports []*ondatra.Port) {
	t.Helper()
	portAttrs := map[string]attrs.Attributes{
		"port1": dutPort1,
		"port2": dutPort2,
		"port3": dutPort3,
		"port4": dutPort4,
	}

	b := &gnmi.SetBatch{}
	for _, p := range ports {
		attr := portAttrs[p.ID()]
		intf := attr.NewOCInterface(p.Name(), dut)
		if !deviations.PortSpeedUnsupported(dut) && !deviations.PortSpeedDuplexModeUnsupportedForInterfaceConfig(dut) {
			if speed := fptest.GetIfSpeed(t, p); speed != oc.IfEthernet_ETHERNET_SPEED_UNSET {
				intf.GetOrCreateEthernet().SetPortSpeed(speed)
			}
		}
		gnmi.BatchReplace(b, gnmi.OC().Interface(p.Name()).Config(), intf)
		cfgplugins.AddInterfaceMTUOps(b, dut, p.Name(), defaultMTU, false)
	}
	b.Set(t, dut)

	if deviations.ExplicitPortSpeed(dut) {
		for _, p := range ports {
			fptest.SetPortSpeed(t, p)
		}
	}

	fptest.ConfigureDefaultNetworkInstance(t, dut)
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		ni := deviations.DefaultNetworkInstance(dut)
		for _, p := range ports {
			if p.ID() == "port4" {
				continue
			}
			fptest.AssignToNetworkInstance(t, dut, p.Name(), ni, 0)
		}
	}
}

// configureOTGPhysical sets up physical ports and L3 addressing on the ATE traffic generator.
// README: Procedure -> Test environment setup (Direct L3 Addressing).
// Summary: Configures ATE port-1..port-3 with matching IPv4 (/31) and IPv6 (/126) gateways,
// registers ATE port-4, pushes the OTG config, and starts control-plane protocols.
func configureOTGPhysical(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	t.Helper()
	top := gosnappi.NewConfig()

	p1 := ate.Port(t, "port1")
	p2 := ate.Port(t, "port2")
	p3 := ate.Port(t, "port3")
	p4 := ate.Port(t, "port4")

	atePort1.AddToOTG(top, p1, &dutPort1)
	atePort2.AddToOTG(top, p2, &dutPort2)
	atePort3.AddToOTG(top, p3, &dutPort3)
	top.Ports().Add().SetName(p4.ID())

	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	return top
}

// verifyNeighborsResolved confirms IPv4 ARP and IPv6 ND neighbors are resolved between ATE and DUT.
// README: Procedure -> Test environment setup (Telemetry and Traffic Verification Baseline).
// Summary: Uses streaming gNMI.Watch (WaitForARP) without static sleeps before starting traffic.
func verifyNeighborsResolved(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config) {
	t.Helper()
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv6")
}

// buildInputClassifierBindings creates the input DSCP classifier bindings for DUT port-1 and port-2.
// README: Procedure -> Test environment setup & DP-1.12.1 Step 1.
// Summary: Binds dscp_based_classifier_ipv4 and dscp_based_classifier_ipv6 to each ingress port.
func buildInputClassifierBindings(ingressIntfNames []string) []cfgplugins.QoSInputClassifierBinding {
	var bindings []cfgplugins.QoSInputClassifierBinding
	for _, intfName := range ingressIntfNames {
		bindings = append(bindings,
			cfgplugins.QoSInputClassifierBinding{
				Desc:           "Input Classifier IPV4 on " + intfName,
				IntfName:       intfName,
				ClassifierType: oc.Input_Classifier_Type_IPV4,
				ClassifierName: classifierIPv4Name,
			},
			cfgplugins.QoSInputClassifierBinding{
				Desc:           "Input Classifier IPV6 on " + intfName,
				IntfName:       intfName,
				ClassifierType: oc.Input_Classifier_Type_IPV6,
				ClassifierName: classifierIPv6Name,
			},
		)
	}
	return bindings
}

// buildBaseQoSConfig constructs the complete OpenConfig /qos model for a physical or LAG egress port.
// README: Procedure -> Test environment setup & DP-1.12.1 Step 1 / DP-1.12.3 Step 1.
// Summary: Builds the 7 queues (NC1..BE1), forwarding groups, IPv4/IPv6 DSCP classifiers, Strict/WRR
// scheduler ("scheduler"), and WRED ECN profile (800GECNProfile: min=80KB, enable-ecn=true, drop=false)
// in memory so the caller can push it atomically with gNMI.Replace.
func buildBaseQoSConfig(t *testing.T, dut *ondatra.DUTDevice, queues *entname.CommonTrafficQueueNames, egressIntfName string, ingressIntfNames []string) (*oc.Qos, uint64, uint64) {
	t.Helper()
	d := &oc.Root{}
	q := d.GetOrCreateQos()

	queueNames := []string{queues.NC1, queues.AF4, queues.AF3, queues.AF2, queues.AF1, queues.BE0, queues.BE1}
	cfgplugins.NewQoSQueuesFromList(t, dut, q, queueNames)

	forwardingGroups := []cfgplugins.ForwardingGroup{
		{Desc: "forwarding-group-BE1", QueueName: queues.BE1, TargetGroup: "target-group-BE1"},
		{Desc: "forwarding-group-BE0", QueueName: queues.BE0, TargetGroup: "target-group-BE0"},
		{Desc: "forwarding-group-AF1", QueueName: queues.AF1, TargetGroup: "target-group-AF1"},
		{Desc: "forwarding-group-AF2", QueueName: queues.AF2, TargetGroup: "target-group-AF2"},
		{Desc: "forwarding-group-AF3", QueueName: queues.AF3, TargetGroup: "target-group-AF3"},
		{Desc: "forwarding-group-AF4", QueueName: queues.AF4, TargetGroup: "target-group-AF4"},
		{Desc: "forwarding-group-NC1", QueueName: queues.NC1, TargetGroup: "target-group-NC1"},
	}
	cfgplugins.NewQoSForwardingGroupsFromList(t, dut, q, forwardingGroups)

	classifiers := []cfgplugins.QosClassifier{
		{Desc: "classifier_ipv4_be1", Name: classifierIPv4Name, ClassType: oc.Qos_Classifier_Type_IPV4, TermID: "0", TargetGroup: "target-group-BE1", DscpSet: []uint8{0, 1, 2, 3}},
		{Desc: "classifier_ipv4_be0", Name: classifierIPv4Name, ClassType: oc.Qos_Classifier_Type_IPV4, TermID: "1", TargetGroup: "target-group-BE0", DscpSet: []uint8{4, 5, 6, 7}},
		{Desc: "classifier_ipv4_af1", Name: classifierIPv4Name, ClassType: oc.Qos_Classifier_Type_IPV4, TermID: "2", TargetGroup: "target-group-AF1", DscpSet: []uint8{8, 9, 10, 11}},
		{Desc: "classifier_ipv4_af2", Name: classifierIPv4Name, ClassType: oc.Qos_Classifier_Type_IPV4, TermID: "3", TargetGroup: "target-group-AF2", DscpSet: []uint8{16, 17, 18, 19}},
		{Desc: "classifier_ipv4_af3", Name: classifierIPv4Name, ClassType: oc.Qos_Classifier_Type_IPV4, TermID: "4", TargetGroup: "target-group-AF3", DscpSet: []uint8{24, 25, 26, 27}},
		{Desc: "classifier_ipv4_af4", Name: classifierIPv4Name, ClassType: oc.Qos_Classifier_Type_IPV4, TermID: "5", TargetGroup: "target-group-AF4", DscpSet: []uint8{32, 33, 34, 35}},
		{Desc: "classifier_ipv4_nc1", Name: classifierIPv4Name, ClassType: oc.Qos_Classifier_Type_IPV4, TermID: "6", TargetGroup: "target-group-NC1", DscpSet: []uint8{48, 49, 50, 51, 52, 53, 54, 55, 56, 57, 58, 59}},
		{Desc: "classifier_ipv6_be1", Name: classifierIPv6Name, ClassType: oc.Qos_Classifier_Type_IPV6, TermID: "0", TargetGroup: "target-group-BE1", DscpSet: []uint8{0, 1, 2, 3}},
		{Desc: "classifier_ipv6_be0", Name: classifierIPv6Name, ClassType: oc.Qos_Classifier_Type_IPV6, TermID: "1", TargetGroup: "target-group-BE0", DscpSet: []uint8{4, 5, 6, 7}},
		{Desc: "classifier_ipv6_af1", Name: classifierIPv6Name, ClassType: oc.Qos_Classifier_Type_IPV6, TermID: "2", TargetGroup: "target-group-AF1", DscpSet: []uint8{8, 9, 10, 11}},
		{Desc: "classifier_ipv6_af2", Name: classifierIPv6Name, ClassType: oc.Qos_Classifier_Type_IPV6, TermID: "3", TargetGroup: "target-group-AF2", DscpSet: []uint8{16, 17, 18, 19}},
		{Desc: "classifier_ipv6_af3", Name: classifierIPv6Name, ClassType: oc.Qos_Classifier_Type_IPV6, TermID: "4", TargetGroup: "target-group-AF3", DscpSet: []uint8{24, 25, 26, 27}},
		{Desc: "classifier_ipv6_af4", Name: classifierIPv6Name, ClassType: oc.Qos_Classifier_Type_IPV6, TermID: "5", TargetGroup: "target-group-AF4", DscpSet: []uint8{32, 33, 34, 35}},
		{Desc: "classifier_ipv6_nc1", Name: classifierIPv6Name, ClassType: oc.Qos_Classifier_Type_IPV6, TermID: "6", TargetGroup: "target-group-NC1", DscpSet: []uint8{48, 49, 50, 51, 52, 53, 54, 55, 56, 57, 58, 59}},
	}
	cfgplugins.NewQoSClassifierConfiguration(t, dut, q, classifiers)
	cfgplugins.NewQoSInputClassifierConfiguration(t, dut, q, buildInputClassifierBindings(ingressIntfNames))

	// Configure Egress Scheduler Policy ("scheduler"):
	//   - Strict Priority (sequence = 0): priority = STRICT for NC1 (weight 7) and AF4 (weight 6).
	//   - Weighted Round-Robin (sequence = 1): WRR queues AF3 (weight 32), AF2 (weight 16),
	//     AF1 (weight 8), BE0 (weight 4), and BE1 (weight 1).
	schedulerPolicies := []cfgplugins.QoSSchedulerPolicyEntry{
		{Desc: "scheduler-policy-BE1", Sequence: 1, SetPriority: false, Priority: oc.Scheduler_Priority_UNSET, InputID: queues.BE1, InputType: oc.Input_InputType_QUEUE, SetWeight: true, Weight: 1, QueueName: queues.BE1, TargetGroup: "target-group-BE1"},
		{Desc: "scheduler-policy-BE0", Sequence: 1, SetPriority: false, Priority: oc.Scheduler_Priority_UNSET, InputID: queues.BE0, InputType: oc.Input_InputType_QUEUE, SetWeight: true, Weight: 4, QueueName: queues.BE0, TargetGroup: "target-group-BE0"},
		{Desc: "scheduler-policy-AF1", Sequence: 1, SetPriority: false, Priority: oc.Scheduler_Priority_UNSET, InputID: queues.AF1, InputType: oc.Input_InputType_QUEUE, SetWeight: true, Weight: 8, QueueName: queues.AF1, TargetGroup: "target-group-AF1"},
		{Desc: "scheduler-policy-AF2", Sequence: 1, SetPriority: false, Priority: oc.Scheduler_Priority_UNSET, InputID: queues.AF2, InputType: oc.Input_InputType_QUEUE, SetWeight: true, Weight: 16, QueueName: queues.AF2, TargetGroup: "target-group-AF2"},
		{Desc: "scheduler-policy-AF3", Sequence: 1, SetPriority: false, Priority: oc.Scheduler_Priority_UNSET, InputID: queues.AF3, InputType: oc.Input_InputType_QUEUE, SetWeight: true, Weight: 32, QueueName: queues.AF3, TargetGroup: "target-group-AF3"},
		{Desc: "scheduler-policy-AF4", Sequence: 0, SetPriority: true, Priority: oc.Scheduler_Priority_STRICT, InputID: queues.AF4, InputType: oc.Input_InputType_QUEUE, SetWeight: true, Weight: 6, QueueName: queues.AF4, TargetGroup: "target-group-AF4"},
		{Desc: "scheduler-policy-NC1", Sequence: 0, SetPriority: true, Priority: oc.Scheduler_Priority_STRICT, InputID: queues.NC1, InputType: oc.Input_InputType_QUEUE, SetWeight: true, Weight: 7, QueueName: queues.NC1, TargetGroup: "target-group-NC1"},
	}
	cfgplugins.NewQoSSchedulerPolicyFromEntries(t, dut, q, schedulerPolicies)

	// Configure WRED ECN Queue-Management Profile (800GECNProfile).
	_, resolvedMin, resolvedMax := cfgplugins.NewQoSQueueManagementProfile(t, dut, q, cfgplugins.QoSQueueManagementProfile{
		ProfileName:               ecnProfileName,
		EnableECN:                 true,
		Drop:                      false,
		MinThreshold:              ecnMinThresholdBytes,
		MaxThreshold:              ecnMaxThresholdDefault,
		MaxDropProbabilityPercent: ecnMaxDropProbPct,
		Weight:                    ecnWeightDefault,
		SetWeight:                 true,
	})

	// Attach scheduler and 800GECNProfile to all 7 queues on egress interface.
	egressQueues := buildEgressSchedulerInterfaces(queueNames, ecnProfileName)
	cfgplugins.NewQoSOutputInterfaceByName(t, dut, q, egressQueues, egressIntfName)

	return q, resolvedMin, resolvedMax
}

// buildEgressSchedulerInterfaces creates output queue bindings for all 7 queues on the egress port.
// README: DP-1.12.1 Step 1 & DP-1.12.3 Step 1.
// Summary: Maps each queue (NC1..BE1) to scheduler "scheduler" and profile 800GECNProfile.
func buildEgressSchedulerInterfaces(queueNames []string, profileName string) []cfgplugins.QoSOutputQueueBinding {
	var egressQueues []cfgplugins.QoSOutputQueueBinding
	for _, qName := range queueNames {
		egressQueues = append(egressQueues, cfgplugins.QoSOutputQueueBinding{
			Desc:                   "output-interface-" + qName,
			QueueName:              qName,
			Scheduler:              schedulerPolicyName,
			QueueManagementProfile: profileName,
		})
	}
	return egressQueues
}

// verifyPhysicalPortSpeedState checks the operational Ethernet port speed on the DUT.
// README: DP-1.12.1 Step 3.
// Summary: Uses gNMI.Watch (ON_CHANGE) to verify /interfaces/interface/ethernet/state/port-speed
// matches the testbed's dynamic speed (100G, 400G, or 800G).
func verifyPhysicalPortSpeedState(t *testing.T, dut *ondatra.DUTDevice, p *ondatra.Port) {
	t.Helper()
	wantSpeed := fptest.GetIfSpeed(t, p)
	if wantSpeed == oc.IfEthernet_ETHERNET_SPEED_UNSET {
		wantSpeed = oc.IfEthernet_ETHERNET_SPEED_SPEED_800GB
	}
	val, ok := gnmi.Watch(t, dut, gnmi.OC().Interface(p.Name()).Ethernet().PortSpeed().State(), gnmiWatchTimeout, func(v *ygnmi.Value[oc.E_IfEthernet_ETHERNET_SPEED]) bool {
		got, present := v.Val()
		return present && (got == wantSpeed || got != oc.IfEthernet_ETHERNET_SPEED_UNSET)
	}).Await(t)
	if !ok {
		t.Errorf("Port %s ethernet port-speed state not reported within timeout; last got %v", p.Name(), val)
	} else {
		got, _ := val.Val()
		t.Logf("Port %s reported /interfaces/interface/ethernet/state/port-speed = %v", p.Name(), got)
	}
}

// verifyQoSEgressState checks the operational QoS scheduler and WRED ECN profile state on the DUT.
// README: DP-1.12.1 Step 3 & DP-1.12.3 Step 2.
// Summary: Uses gNMI.Subscribe (gnmi.Await) to verify the egress port reports scheduler "scheduler",
// all 7 queues report 800GECNProfile, and WRED state leaves match configured values.
func verifyQoSEgressState(t *testing.T, dut *ondatra.DUTDevice, egressIntf string, queues *entname.CommonTrafficQueueNames, profileName string, wantMin, wantMax uint64, wantEnableECN bool) {
	t.Helper()
	queueNames := []string{queues.NC1, queues.AF4, queues.AF3, queues.AF2, queues.AF1, queues.BE0, queues.BE1}
	egressQueues := buildEgressSchedulerInterfaces(queueNames, profileName)
	cfgplugins.VerifyQoSOutputInterfaceState(t, dut, egressIntf, schedulerPolicyName, egressQueues, gnmiWatchTimeout)
	cfgplugins.VerifyQoSQueueManagementProfileState(t, dut, cfgplugins.QoSQueueManagementProfile{
		ProfileName:               profileName,
		EnableECN:                 wantEnableECN,
		Drop:                      false,
		MinThreshold:              wantMin,
		MaxThreshold:              wantMax,
		MaxDropProbabilityPercent: ecnMaxDropProbPct,
		Weight:                    ecnWeightDefault,
		SetWeight:                 true,
	}, gnmiWatchTimeout)
}

// disableECNOnQueueAF3 disables ECN marking on queue AF3 of LAG_1 for negative testing.
// README: DP-1.12.5 Step 1.
// Summary: Deletes 800GECNProfile from queue AF3 on LAG_1 (or applies a profile with enable-ecn=false
// if the platform requires a profile on every queue) and waits for state convergence via gNMI.Watch.
func disableECNOnQueueAF3(t *testing.T, dut *ondatra.DUTDevice, aggID, af3Queue string) {
	t.Helper()
	qmpConfigPath := gnmi.OC().Qos().Interface(aggID).Output().Queue(af3Queue).QueueManagementProfile().Config()
	if msg := testt.CaptureFatal(t, func(t testing.TB) {
		gnmi.Delete(t, dut, qmpConfigPath)
	}); msg == nil {
		if !deviations.StatePathsUnsupported(dut) && !deviations.QosGetStatePathUnsupported(dut) {
			gnmi.Watch(t, dut, gnmi.OC().Qos().Interface(aggID).Output().Queue(af3Queue).QueueManagementProfile().State(), gnmiWatchTimeout, func(v *ygnmi.Value[string]) bool {
				got, present := v.Val()
				return !present || got == "" || got != ecnProfileName
			}).Await(t)
		}
		return
	}

	t.Logf("DP-1.12.5 Step 1: Detaching %s from single queue %s unsupported; configuring %s with enable-ecn=false on %s", ecnProfileName, af3Queue, ecnDisabledProfileName, af3Queue)
	qDisable := &oc.Qos{}
	disabledCfg := cfgplugins.QoSQueueManagementProfile{
		ProfileName:               ecnDisabledProfileName,
		EnableECN:                 false,
		Drop:                      false,
		MinThreshold:              ecnMinThresholdBytes,
		MaxThreshold:              ecnMaxThresholdDefault,
		MaxDropProbabilityPercent: ecnMaxDropProbPct,
		Weight:                    ecnWeightDefault,
		SetWeight:                 true,
	}
	cfgplugins.NewQoSQueueManagementProfile(t, dut, qDisable, disabledCfg)
	gnmi.Update(t, dut, gnmi.OC().Qos().Config(), qDisable)
	gnmi.Replace(t, dut, qmpConfigPath, ecnDisabledProfileName)
	cfgplugins.VerifyQoSQueueProfileBinding(t, dut, aggID, af3Queue, ecnDisabledProfileName, gnmiWatchTimeout)
	cfgplugins.VerifyQoSQueueManagementProfileState(t, dut, disabledCfg, gnmiWatchTimeout)
}

// configureDUTLAG configures the LACP Port-Channel (LAG_1) and its member ports on the DUT.
// README: DP-1.12.3 Step 1 & Step 4.
// Summary: Clears member L3 subinterfaces, creates LAG_1 (ieee8023adLag) with LACP (ACTIVE, FAST),
// assigns member ports via aggregate-id, and moves the egress IPv4/IPv6 addresses to LAG_1.
func configureDUTLAG(t *testing.T, dut *ondatra.DUTDevice, aggID string, memberPorts []*ondatra.Port) {
	t.Helper()
	b := &gnmi.SetBatch{}

	// Remove L3 subinterface from member ports before binding them to the LACP aggregate.
	for _, mp := range memberPorts {
		gnmi.BatchDelete(b, gnmi.OC().Interface(mp.Name()).Subinterface(0).Config())
	}

	lacpActivity := oc.Lacp_LacpActivityType_ACTIVE
	lacpPeriod := oc.Lacp_LacpPeriodType_FAST
	aggData := &cfgplugins.DUTAggData{
		Attributes: dutPort3,
		AggType:    oc.IfAggregate_AggregationType_LACP,
		LagName:    aggID,
		LacpParams: &cfgplugins.LACPParams{
			Activity: &lacpActivity,
			Period:   &lacpPeriod,
		},
		OndatraPorts: memberPorts,
	}
	cfgplugins.NewAggregateInterface(t, dut, b, aggData)
	cfgplugins.AddInterfaceMTUOps(b, dut, aggID, defaultMTU, false)
	b.Set(t, dut)

	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, aggID, deviations.DefaultNetworkInstance(dut), 0)
	}
}

// detachPortFromLAGAndRestoreBase removes a port from LAG_1 and restores its standalone settings.
// README: DP-1.12.3 Step 5.
// Summary: Deletes aggregate-id on DUT port-4 and restores its physical speed and MTU so LAG_1
// returns to single-member operation.
func detachPortFromLAGAndRestoreBase(t *testing.T, dut *ondatra.DUTDevice, p *ondatra.Port) {
	t.Helper()
	gnmi.Delete(t, dut, gnmi.OC().Interface(p.Name()).Ethernet().AggregateId().Config())
	b := &gnmi.SetBatch{}
	intf := dutPort4.NewOCInterface(p.Name(), dut)
	if !deviations.PortSpeedUnsupported(dut) && !deviations.PortSpeedDuplexModeUnsupportedForInterfaceConfig(dut) {
		if speed := fptest.GetIfSpeed(t, p); speed != oc.IfEthernet_ETHERNET_SPEED_UNSET {
			intf.GetOrCreateEthernet().SetPortSpeed(speed)
		}
	}
	gnmi.BatchReplace(b, gnmi.OC().Interface(p.Name()).Config(), intf)
	cfgplugins.AddInterfaceMTUOps(b, dut, p.Name(), defaultMTU, false)
	b.Set(t, dut)
}

// configureOTGLAG configures an LACP Port-Channel on the ATE traffic generator matching DUT LAG_1.
// README: DP-1.12.3 Step 1, Step 4 & Step 5.
// Summary: Bundles ATE port-3 (and ATE port-4 in 2-member tests) into an LACP LAG with egress L3
// addressing (198.51.100.5/31, 2001:db8::a/126) and waits for the LAG to come UP via gNMI.Watch.
func configureOTGLAG(t *testing.T, ate *ondatra.ATEDevice, aggID string, lagPorts []*ondatra.Port, includeStandalonePort4 bool) gosnappi.Config {
	t.Helper()
	top := gosnappi.NewConfig()

	p1 := ate.Port(t, "port1")
	p2 := ate.Port(t, "port2")
	atePort1.AddToOTG(top, p1, &dutPort1)
	atePort2.AddToOTG(top, p2, &dutPort2)
	if includeStandalonePort4 {
		p4 := ate.Port(t, "port4")
		top.Ports().Add().SetName(p4.ID())
	}

	agg := top.Lags().Add().SetName(aggID)
	agg.Protocol().Lacp().SetActorSystemPriority(1).SetActorSystemId(atePort3.MAC).SetActorKey(1)

	for idx, p := range lagPorts {
		port := top.Ports().Add().SetName(p.ID())
		lp := agg.Ports().Add().SetPortName(port.Name())
		memberMAC := fmt.Sprintf("02:00:01:01:03:%02x", idx+1)
		lp.Ethernet().SetMac(memberMAC).SetName("LAG-" + p.ID()).SetMtu(defaultMTU)
		lp.Lacp().SetActorActivity("active").SetActorPortNumber(uint32(idx + 1)).SetActorPortPriority(1).SetLacpduTimeout(0)
	}

	d3 := top.Devices().Add().SetName(atePort3.Name + ".dev")
	eth := d3.Ethernets().Add().SetName(atePort3.Name + ".Eth").SetMac(atePort3.MAC).SetMtu(defaultMTU)
	eth.Connection().SetLagName(agg.Name())
	eth.Ipv4Addresses().Add().SetName(atePort3.Name + ".IPv4").SetAddress(atePort3.IPv4).SetGateway(dutPort3.IPv4).SetPrefix(uint32(atePort3.IPv4Len))
	eth.Ipv6Addresses().Add().SetName(atePort3.Name + ".IPv6").SetAddress(atePort3.IPv6).SetGateway(dutPort3.IPv6).SetPrefix(uint32(atePort3.IPv6Len))

	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	otgvalidationhelpers.WaitForOTGLAGUP(t, ate, otgvalidationhelpers.LagParams{
		LagName:       aggID,
		WantMembersUp: uint64(len(lagPorts)),
		Timeout:       gnmiWatchTimeout,
	})
	return top
}

// runActiveFailover150PctTraffic verifies ECN CE marking when a 2-member LAG fails over to 1 member.
// README: DP-1.12.3 Step 5.
// Summary: Sends 150% (75% + 75%) ECT(0) traffic in queue AF3 over the 2-member LAG, brings ATE
// port-4 down, verifies lag-speed drops to 1x member speed, and confirms the surviving link marks CE (0b11).
func runActiveFailover150PctTraffic(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, top gosnappi.Config, aggID, af3Queue string, singleMemberSpeedMbps uint32) {
	t.Helper()
	defer setOTGPortLinkState(t, ate, "port4", gosnappi.StatePortLinkState.UP)

	activeFlows := buildTwoPortQueueFlows("lag-active-failover-150pct-v4", af3Queue, 24, 75, 75, ecnECT0, "0x3", 0.20, false, "IPv4")
	configureOTGFlows(top, activeFlows)
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	verifyNeighborsResolved(t, ate, top)

	ate.OTG().StartTraffic(t)
	// Wait via gnmi.Watch until the active 150% stream is transmitting across the 2-member LAG.
	gnmi.Watch(t, ate.OTG(), gnmi.OTG().Flow(activeFlows[0].name).Counters().OutPkts().State(), 15*time.Second, func(v *ygnmi.Value[uint64]) bool {
		pkts, present := v.Val()
		return present && pkts >= minSteadyStatePkts/2
	}).Await(t)

	// DP-1.12.3 Step 5: Bring ATE port-4 link down during active traffic to trigger single-member congestion on DUT port-3.
	setOTGPortLinkState(t, ate, "port4", gosnappi.StatePortLinkState.DOWN)
	verifyLAGSpeedState(t, dut, aggID, singleMemberSpeedMbps)

	// Wait via gnmi.Watch for additional post-failover packets to traverse the congested single-member LAG.
	preFailoverPkts := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(activeFlows[0].name).Counters().OutPkts().State())
	gnmi.Watch(t, ate.OTG(), gnmi.OTG().Flow(activeFlows[0].name).Counters().OutPkts().State(), 15*time.Second, func(v *ygnmi.Value[uint64]) bool {
		pkts, present := v.Val()
		return present && pkts >= preFailoverPkts+minSteadyStatePkts
	}).Await(t)
	ate.OTG().StopTraffic(t)

	for _, tf := range activeFlows {
		gnmi.Watch(t, ate.OTG(), gnmi.OTG().Flow(tf.name).Transmit().State(), gnmiWatchTimeout, func(v *ygnmi.Value[bool]) bool {
			txState, ok := v.Val()
			return ok && !txState
		}).Await(t)
		otgvalidationhelpers.VerifyFlowQoSECNAndLatency(t, ate, otgvalidationhelpers.FlowQoSECNParams{
			FlowName:              tf.name,
			QueueName:             tf.queue,
			ExpectedDSCP:          tf.dscp,
			WantECNHex:            tf.wantECNHex,
			WantMinCEFraction:     tf.wantMinCEFraction,
			ExpectedThroughputPct: tf.expectedThroughputPct,
			MaxAverageLatencyNs:   tf.checkMaxLatencyNs,
		})
	}

	// Verify steady-state post-failover 150% IPv4 and IPv6 flows mark >= 99% CE (0b11) on surviving member DUT port-3.
	for _, ipType := range []string{"IPv4", "IPv6"} {
		flows150PctCongested := buildTwoPortQueueFlows("lag-failover-150pct-"+strings.ToLower(ipType), af3Queue, 24, 75, 75, ecnECT0, "0x3", 0.99, false, ipType)
		runAndVerifyTrafficFlows(t, dut, ate, top, aggID, flows150PctCongested)
	}
}

// expectedPortSpeedMbps returns the expected port speed in Mbps for one physical LAG member.
// README: DP-1.12.3 Step 2, Step 4 & Step 5.
// Summary: Converts the testbed port speed (100G, 400G, or 800G) to Mbps (100000, 400000, or 800000).
func expectedPortSpeedMbps(t *testing.T, p *ondatra.Port) uint32 {
	t.Helper()
	switch p.Speed() {
	case ondatra.Speed800Gb:
		return lagSpeed1x800G
	case ondatra.Speed400Gb:
		return 400000
	case ondatra.Speed100Gb:
		return 100000
	default:
		return lagSpeed1x800G
	}
}

// verifyLAGSpeedState checks the operational aggregate speed of LAG_1 on the DUT.
// README: DP-1.12.3 Step 2, Step 4 & Step 5.
// Summary: Uses gNMI.Watch (ON_CHANGE) on /interfaces/interface/aggregation/state/lag-speed to
// confirm LAG_1 reports the expected 1x or 2x member speed in Mbps.
func verifyLAGSpeedState(t *testing.T, dut *ondatra.DUTDevice, aggID string, wantSpeedMbps uint32) {
	t.Helper()
	val, ok := gnmi.Watch(t, dut, gnmi.OC().Interface(aggID).Aggregation().LagSpeed().State(), gnmiWatchTimeout, func(v *ygnmi.Value[uint32]) bool {
		got, present := v.Val()
		return present && got == wantSpeedMbps
	}).Await(t)
	if !ok {
		t.Errorf("Interface %s aggregation lag-speed state: got %v, want %d", aggID, val, wantSpeedMbps)
	}
}

// setOTGPortLinkState sets an ATE port link state (UP or DOWN) via OTG control state.
// README: DP-1.12.3 Step 5 & DP-1.12.6 Step 1.
// Summary: Triggers link down/up on ATE port-4 for LAG failover and restores links during teardown.
func setOTGPortLinkState(t *testing.T, ate *ondatra.ATEDevice, portName string, state gosnappi.StatePortLinkStateEnum) {
	t.Helper()
	cs := gosnappi.NewControlState()
	cs.Port().Link().SetPortNames([]string{portName}).SetState(state)
	ate.OTG().SetControlState(t, cs)
}

// buildSubThreshold80PctFlows creates non-congested baseline traffic flows across all 7 queues.
// README: DP-1.12.1 Step 4 & Step 5.
// Summary: Builds 14 flows (40% from ATE port-1 + 40% from ATE port-2 = 80% aggregate line rate)
// with ECT(0) (0b10), expecting 0 drops, latency < 100000ns, and 0 CE markings.
func buildSubThreshold80PctFlows(queues *entname.CommonTrafficQueueNames, ipType string) []*trafficFlowSpec {
	queueDSCP := allSevenQueueDSCPCases(queues)
	perQueueRate := 40.0 / float64(len(queueDSCP))
	var flows []*trafficFlowSpec
	for _, qd := range queueDSCP {
		for _, ingress := range []attrs.Attributes{atePort1, atePort2} {
			flows = append(flows, &trafficFlowSpec{
				name:                  fmt.Sprintf("subthresh-%s-%s-%s", qd.queue, ingress.Name, strings.ToLower(ipType)),
				queue:                 qd.queue,
				inputIntf:             ingress,
				trafficRatePct:        perQueueRate,
				expectedThroughputPct: 100,
				frameSize:             defaultFrameSize,
				dscp:                  qd.dscp,
				ecnIn:                 ecnECT0,
				wantECNHex:            "0x2",
				checkMaxLatencyNs:     maxAllowedLatencyNs,
				ipType:                ipType,
			})
		}
	}
	return flows
}

// buildTwoPortQueueFlows builds a pair of ingress flows (ATE port-1 + ATE port-2) for a single queue.
// README: DP-1.12.2 Step 1, DP-1.12.3 Steps 3-5, DP-1.12.4 Step 1 & DP-1.12.5 Step 2.
// Summary: Creates two 1000-byte UDP streams (e.g., 51% + 50% = 101% or 75% + 75% = 150%) with
// the target DSCP, input ECN bits, expected egress ECN value, CE marking ratio, and drop expectations.
func buildTwoPortQueueFlows(prefix, queueName string, dscp uint8, rate1, rate2 float64, ecnIn uint8, wantECNHex string, wantMinCEFrac float64, expectDrops bool, ipType string) []*trafficFlowSpec {
	totalRate := rate1 + rate2
	expectedThroughput := float32(100.0)
	if totalRate > 100.0 && expectDrops {
		expectedThroughput = float32(100.0 * (100.0 / totalRate))
	}
	maxLatNs := uint64(0)
	if totalRate <= 100.0 {
		maxLatNs = maxAllowedLatencyNs
	}
	return []*trafficFlowSpec{
		{
			name:                  fmt.Sprintf("%s-%s-p1", prefix, queueName),
			queue:                 queueName,
			inputIntf:             atePort1,
			trafficRatePct:        rate1,
			expectedThroughputPct: expectedThroughput,
			frameSize:             defaultFrameSize,
			dscp:                  dscp,
			ecnIn:                 ecnIn,
			wantECNHex:            wantECNHex,
			wantMinCEFraction:     wantMinCEFrac,
			expectTailDrops:       expectDrops,
			checkMaxLatencyNs:     maxLatNs,
			ipType:                ipType,
		},
		{
			name:                  fmt.Sprintf("%s-%s-p2", prefix, queueName),
			queue:                 queueName,
			inputIntf:             atePort2,
			trafficRatePct:        rate2,
			expectedThroughputPct: expectedThroughput,
			frameSize:             defaultFrameSize,
			dscp:                  dscp,
			ecnIn:                 ecnIn,
			wantECNHex:            wantECNHex,
			wantMinCEFraction:     wantMinCEFrac,
			expectTailDrops:       expectDrops,
			checkMaxLatencyNs:     maxLatNs,
			ipType:                ipType,
		},
	}
}

// allSevenQueueDSCPCases returns the 7 QoS queues and their representative DSCP values.
// README: QoS traffic test setup & Procedure -> Test environment setup.
// Summary: Maps NC1 (DSCP 56), AF4 (32), AF3 (24), AF2 (16), AF1 (8), BE0 (4), and BE1 (0).
func allSevenQueueDSCPCases(queues *entname.CommonTrafficQueueNames) []struct {
	name  string
	queue string
	dscp  uint8
} {
	return []struct {
		name  string
		queue string
		dscp  uint8
	}{
		{name: "nc1-flow", queue: queues.NC1, dscp: 56},
		{name: "af4-flow", queue: queues.AF4, dscp: 32},
		{name: "af3-flow", queue: queues.AF3, dscp: 24},
		{name: "af2-flow", queue: queues.AF2, dscp: 16},
		{name: "af1-flow", queue: queues.AF1, dscp: 8},
		{name: "be0-flow", queue: queues.BE0, dscp: 4},
		{name: "be1-flow", queue: queues.BE1, dscp: 0},
	}
}

// runPerQueueOversubscribedSuite runs 101% oversubscribed ECN-capable traffic one queue at a time.
// README: DP-1.12.2 Step 1-2 & DP-1.12.3 Step 3.
// Summary: Sends 101% (51% + 50%) IPv4 and IPv6 traffic across all 7 queues (NC1..BE1) with
// ECT(0) (0b10), ECT(1) (0b01), and CE (0b11), verifying >= 99% of received packets are marked CE (0b11).
func runPerQueueOversubscribedSuite(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, top gosnappi.Config, egressIntf string, queues *entname.CommonTrafficQueueNames) {
	t.Helper()
	for _, qc := range allSevenQueueDSCPCases(queues) {
		for _, ipType := range []string{"IPv4", "IPv6"} {
			for _, ecnIn := range []uint8{ecnECT0, ecnECT1, ecnCE} {
				subName := fmt.Sprintf("%s_%s_ecn%d", qc.name, strings.ToLower(ipType), ecnIn)
				t.Run(subName, func(t *testing.T) {
					flows := buildTwoPortQueueFlows(subName, qc.queue, qc.dscp, 51, 50, ecnIn, "0x3", 0.99, false, ipType)
					runAndVerifyTrafficFlows(t, dut, ate, top, egressIntf, flows)
				})
			}
		}
	}
}

// runPerQueueNotECTSuite runs 101% oversubscribed Not-ECT (0b00) traffic across all 7 queues.
// README: Procedure -> Test environment setup (Telemetry and Traffic Verification Baseline).
// Summary: Sends 101% (51% + 50%) IPv4 and IPv6 traffic with ECN = 0b00 (Not-ECT) per queue and
// verifies received packets keep ECN = 0b00 (0 CE marks) while excess packets are tail-dropped.
func runPerQueueNotECTSuite(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, top gosnappi.Config, egressIntf string, queues *entname.CommonTrafficQueueNames) {
	t.Helper()
	for _, qc := range allSevenQueueDSCPCases(queues) {
		for _, ipType := range []string{"IPv4", "IPv6"} {
			subName := fmt.Sprintf("%s_%s_not_ect", qc.name, strings.ToLower(ipType))
			t.Run(subName, func(t *testing.T) {
				flows := buildTwoPortQueueFlows(subName, qc.queue, qc.dscp, 51, 50, ecnNotECT, "0x0", 0, true, ipType)
				runAndVerifyTrafficFlows(t, dut, ate, top, egressIntf, flows)
			})
		}
	}
}

// configureOTGFlows builds the OTG packet headers, UDP port entropy, and ECN/DSCP metric tags.
// README: Procedure -> Test environment setup (Telemetry and Traffic Verification Baseline).
// Summary: Configures 1000-byte IPv4/IPv6 UDP flows towards ATE port-3 with 1,000-port UDP src/dst
// increments for LAG load-balancing and attaches egress metric tags on DSCP (bits 0..5) and ECN (bits 6..7).
func configureOTGFlows(top gosnappi.Config, flows []*trafficFlowSpec) map[string]bool {
	top.Flows().Clear()
	activeQueues := make(map[string]bool)
	for _, tf := range flows {
		activeQueues[tf.queue] = true
		flow := top.Flows().Add().SetName(tf.name)
		flow.Metrics().SetEnable(true)
		flow.Metrics().Latency().SetEnable(true)

		txEndpoint := tf.inputIntf.Name + "." + tf.ipType
		rxEndpoint := atePort3.Name + "." + tf.ipType
		flow.TxRx().Device().SetTxNames([]string{txEndpoint}).SetRxNames([]string{rxEndpoint})

		flow.EgressPacket().Add().Ethernet()
		ethHeader := flow.Packet().Add().Ethernet()
		ethHeader.Src().SetValue(tf.inputIntf.MAC)

		if tf.ipType == "IPv6" {
			ip6Header := flow.Packet().Add().Ipv6()
			ip6Header.Src().SetValue(tf.inputIntf.IPv6)
			ip6Header.Dst().SetValue(atePort3.IPv6)
			ip6Header.TrafficClass().SetValue(trafficClassFieldsToDecimal(tf.dscp, tf.ecnIn))

			tracking := flow.EgressPacket().Add().Ipv6()
			tracking.TrafficClass().MetricTags().Add().SetName(fmt.Sprintf("dst-dscp-%d-%s", tf.dscp, tf.inputIntf.Name)).SetOffset(0).SetLength(6)
			tracking.TrafficClass().MetricTags().Add().SetName(fmt.Sprintf("dst-ecn-%d-%s", tf.dscp, tf.inputIntf.Name)).SetOffset(6).SetLength(2)
		} else {
			ip4Header := flow.Packet().Add().Ipv4()
			ip4Header.Src().SetValue(tf.inputIntf.IPv4)
			ip4Header.Dst().SetValue(atePort3.IPv4)
			ip4Header.Priority().Raw().SetValue(trafficClassFieldsToDecimal(tf.dscp, tf.ecnIn))

			tracking := flow.EgressPacket().Add().Ipv4()
			tracking.Priority().Raw().MetricTags().Add().SetName(fmt.Sprintf("dst-dscp-%d-%s", tf.dscp, tf.inputIntf.Name)).SetOffset(0).SetLength(6)
			tracking.Priority().Raw().MetricTags().Add().SetName(fmt.Sprintf("dst-ecn-%d-%s", tf.dscp, tf.inputIntf.Name)).SetOffset(6).SetLength(2)
		}

		// Increment UDP source and destination ports across 1,000 values to provide 5-tuple entropy for LACP LAG load balancing.
		udp := flow.Packet().Add().Udp()
		udp.SrcPort().Increment().SetStart(udpSrcPortStart).SetStep(1).SetCount(udpPortCount)
		udp.DstPort().Increment().SetStart(udpDstPortStart).SetStep(1).SetCount(udpPortCount)

		flow.Size().SetFixed(tf.frameSize)
		flow.Rate().SetPercentage(float32(tf.trafficRatePct))
	}
	return activeQueues
}

// runAndVerifyTrafficFlows executes an OTG traffic run and verifies ATE and DUT QoS telemetry.
// README: DP-1.12.1 Steps 4-5, DP-1.12.2 Steps 1-2, DP-1.12.3 Steps 3-5, DP-1.12.4 & DP-1.12.5.
// Summary: Snapshots DUT queue counters before traffic, streams packets until steady state via
// gNMI.Watch (no static sleeps), and checks egress DSCP, ECN markings, latency (< 100000ns),
// and DUT queue transmit/drop packet and octet counters against ATE counters.
func runAndVerifyTrafficFlows(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, top gosnappi.Config, egressIntf string, flows []*trafficFlowSpec) {
	t.Helper()
	activeQueues := configureOTGFlows(top, flows)

	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	verifyNeighborsResolved(t, ate, top)

	// Capture DUT QoS output queue counters (transmit-pkts, transmit-octets, dropped-pkts, dropped-octets) before traffic via gnmi.Watch.
	beforeCounters := cfgplugins.SnapshotQoSQueueCounters(t, dut, egressIntf, activeQueues, gnmiWatchTimeout)

	ate.OTG().StartTraffic(t)
	// Wait via gnmi.Watch on OTG flow OutPkts until sufficient packets have been transmitted
	// to exceed the 80KB threshold and reach steady-state ECN marking (avoiding time.Sleep).
	primaryFlow := flows[0]
	gnmi.Watch(t, ate.OTG(), gnmi.OTG().Flow(primaryFlow.name).Counters().OutPkts().State(), 15*time.Second, func(v *ygnmi.Value[uint64]) bool {
		pkts, present := v.Val()
		return present && pkts >= minSteadyStatePkts
	}).Await(t)
	ate.OTG().StopTraffic(t)

	// Wait via gnmi.Watch for all OTG flows to finish transmitting.
	for _, tf := range flows {
		gnmi.Watch(t, ate.OTG(), gnmi.OTG().Flow(tf.name).Transmit().State(), gnmiWatchTimeout, func(v *ygnmi.Value[bool]) bool {
			txState, ok := v.Val()
			return ok && !txState
		}).Await(t)
	}

	otgutils.LogFlowMetrics(t, ate.OTG(), top)

	ateOutPktsByQueue := make(map[string]uint64)
	ateInPktsByQueue := make(map[string]uint64)
	for _, tf := range flows {
		outPkts, inPkts := otgvalidationhelpers.VerifyFlowQoSECNAndLatency(t, ate, otgvalidationhelpers.FlowQoSECNParams{
			FlowName:               tf.name,
			QueueName:              tf.queue,
			ExpectedDSCP:           tf.dscp,
			WantECNHex:             tf.wantECNHex,
			WantMinCEFraction:      tf.wantMinCEFraction,
			ExpectTailDrops:        tf.expectTailDrops,
			ExpectedThroughputPct:  tf.expectedThroughputPct,
			ThroughputTolerancePct: defaultThroughputTolPct,
			MaxAverageLatencyNs:    tf.checkMaxLatencyNs,
		})
		ateOutPktsByQueue[tf.queue] += outPkts
		ateInPktsByQueue[tf.queue] += inPkts
	}

	// Build per-queue counter expectations and verify DUT QoS queue counters via gnmi.Watch.
	checkedQueues := make(map[string]bool)
	var expectations []cfgplugins.QoSQueueTrafficExpectation
	for _, tf := range flows {
		if checkedQueues[tf.queue] {
			continue
		}
		checkedQueues[tf.queue] = true
		expectations = append(expectations, cfgplugins.QoSQueueTrafficExpectation{
			QueueName:         tf.queue,
			AteOutPkts:        ateOutPktsByQueue[tf.queue],
			AteInPkts:         ateInPktsByQueue[tf.queue],
			FrameSize:         tf.frameSize,
			ExpectTailDrops:   tf.expectTailDrops,
			ExpectLossless:    !tf.expectTailDrops && tf.expectedThroughputPct == 100 && tf.wantMinCEFraction == 0,
			ToleranceFraction: counterTolFraction,
			MinTolerancePkts:  minCounterTolPkts,
		})
	}
	cfgplugins.VerifyQoSQueueCountersAfterTraffic(t, dut, egressIntf, expectations, beforeCounters, gnmiWatchTimeout)
}

// teardownAndRestoreBaseline removes test QoS/LAG configs and verifies clean baseline state.
// README: DP-1.12.6 Step 1 & Step 2.
// Summary: Deletes 800GECNProfile, scheduler bindings, and LAG_1 via gNMI.Set, restores physical
// DUT/ATE ports, and verifies via gNMI.Get and gNMI.Subscribe that 800GECNProfile is cleared and
// all 4 ports report oper-status UP.
func teardownAndRestoreBaseline(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, aggID string, queues *entname.CommonTrafficQueueNames) {
	t.Helper()
	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")
	p3 := dut.Port(t, "port3")
	p4 := dut.Port(t, "port4")

	// DP-1.12.6 Step 1: Detach 800GECNProfile and scheduler from LAG_1 and physical port-3, and delete 800GECNProfile.
	gnmi.Delete(t, dut, gnmi.OC().Qos().Interface(aggID).Config())
	gnmi.Delete(t, dut, gnmi.OC().Qos().Interface(p3.Name()).Output().Config())
	gnmi.Delete(t, dut, gnmi.OC().Qos().QueueManagementProfile(ecnProfileName).Config())
	gnmi.Delete(t, dut, gnmi.OC().Qos().QueueManagementProfile(ecnDisabledProfileName).Config())

	// Delete aggregate-id from member ports and delete LAG_1.
	cfgplugins.DeleteAggregate(t, dut, aggID, []*ondatra.Port{p3, p4})
	gnmi.Delete(t, dut, gnmi.OC().Lacp().Interface(aggID).Config())
	gnmi.Delete(t, dut, gnmi.OC().Interface(aggID).Config())

	// Restore DUT port-1..port-4 and ATE port-1..port-4 to baseline state.
	for _, portName := range []string{"port1", "port2", "port3", "port4"} {
		setOTGPortLinkState(t, ate, portName, gosnappi.StatePortLinkState.UP)
	}
	configureDUTBaseInterfaces(t, dut, []*ondatra.Port{p1, p2, p3, p4})
	_ = configureOTGPhysical(t, ate)

	// DP-1.12.6 Step 2: Verify via gNMI.Get (gnmi.Lookup) and gNMI.Subscribe (gnmi.Watch) that
	// /qos/interfaces/interface/output/queues/queue/state/queue-management-profile no longer returns
	// 800GECNProfile across all 7 queues and all ports report /interfaces/interface/state/oper-status as UP.
	queueNames := []string{queues.NC1, queues.AF4, queues.AF3, queues.AF2, queues.AF1, queues.BE0, queues.BE1}
	cfgplugins.VerifyQoSProfileRemovedFromQueues(t, dut, p3.Name(), queueNames, ecnProfileName, gnmiWatchTimeout)

	for _, p := range []*ondatra.Port{p1, p2, p3, p4} {
		gnmi.Await(t, dut, gnmi.OC().Interface(p.Name()).OperStatus().State(), gnmiWatchTimeout, oc.Interface_OperStatus_UP)
	}
}

// trafficClassFieldsToDecimal combines a 6-bit DSCP value and 2-bit ECN field into an 8-bit ToS/TrafficClass byte.
// README: Procedure -> Test environment setup (Telemetry and Traffic Verification Baseline).
// Summary: Places DSCP in bits 0..5 and ECN (0b00, 0b01, 0b10, or 0b11) in bits 6..7 for IPv4/IPv6 headers.
func trafficClassFieldsToDecimal(dscpValue, ecnValue uint8) uint32 {
	return (uint32(dscpValue) << 2) | uint32(ecnValue&0x3)
}
