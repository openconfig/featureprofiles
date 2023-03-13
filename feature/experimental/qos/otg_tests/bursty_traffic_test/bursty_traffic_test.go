// Copyright 2023 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package bursty_traffic_test

import (
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

var (
	intf1 = attrs.Attributes{
		Name:    "ate1",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "198.51.100.1",
		IPv4Len: 31,
	}

	intf2 = attrs.Attributes{
		Name:    "ate2",
		MAC:     "02:00:01:02:01:01",
		IPv4:    "198.51.100.3",
		IPv4Len: 31,
	}

	intf3 = attrs.Attributes{
		Name:    "ate3",
		MAC:     "02:00:01:03:01:01",
		IPv4:    "198.51.100.5",
		IPv4Len: 31,
	}

	dutPort1 = attrs.Attributes{
		IPv4: "198.51.100.0",
	}
	dutPort2 = attrs.Attributes{
		IPv4: "198.51.100.2",
	}
	dutPort3 = attrs.Attributes{
		IPv4: "198.51.100.4",
	}
)

type trafficData struct {
	trafficRate           float64
	expectedThroughputPct float32
	frameSize             uint32
	dscp                  uint8
	queue                 string
	inputIntf             attrs.Attributes
	burstPackets          uint32
	burstMinGap           uint32
	burstGap              uint32
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Test cases:
//  - Verify that there is no traffic loss with bursty traffic cases:
//    1) Bursty NC1 traffic.
//    2) Bursty AF4 traffic.
//    3) Bursty AF3 traffic.
//    4) Bursty AF2 traffic.
//    5) Bursty AF1 traffic.
//    6) Bursty BE0 traffic.
//    7) Bursty BE1 traffic.
//
// Topology:
//       ATE port 1
//        |
//       DUT--------ATE port 3
//        |
//       ATE port 2
//
//  Sample CLI command to get telemetry using gmic:
//   - gnmic -a ipaddr:10162 -u username -p password --skip-verify get \
//      --path /components/component --format flat
//   - gnmic tool info:
//     - https://github.com/karimra/gnmic/blob/main/README.md
//

func TestBurstyTraffic(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")
	dp3 := dut.Port(t, "port3")

	// Configure DUT interfaces and QoS.
	ConfigureDUTIntf(t, dut)
	ConfigureQoS(t, dut)

	// Configure ATE interfaces.
	ate := ondatra.ATE(t, "ate")
	ap1 := ate.Port(t, "port1")
	ap2 := ate.Port(t, "port2")
	ap3 := ate.Port(t, "port3")
	top := ate.OTG().NewConfig(t)

	intf1.AddToOTG(top, ap1, &dutPort1)
	intf2.AddToOTG(top, ap2, &dutPort2)
	intf3.AddToOTG(top, ap3, &dutPort3)
	ate.OTG().PushConfig(t, top)

	var tolerance float32 = 2.0

	queueMap := map[ondatra.Vendor]map[string]string{
		ondatra.JUNIPER: {
			"NC1": "3",
			"AF4": "2",
			"AF3": "5",
			"AF2": "1",
			"AF1": "4",
			"BE1": "0",
			"BE0": "6",
		},
		ondatra.ARISTA: {
			"NC1": "NC1",
			"AF4": "AF4",
			"AF3": "AF3",
			"AF2": "AF2",
			"AF1": "AF1",
			"BE1": "BE1",
			"BE0": "BE0",
		},
		ondatra.CISCO: {
			"NC1": "7",
			"AF4": "4",
			"AF3": "3",
			"AF2": "2",
			"AF1": "0",
			"BE1": "1",
			"BE0": "1",
		},
		ondatra.NOKIA: {
			"NC1": "7",
			"AF4": "4",
			"AF3": "3",
			"AF2": "2",
			"AF1": "0",
			"BE1": "1",
			"BE0": "1",
		},
	}

	// Test case 1: Bursty NC1 traffic.
	nc1TrafficFlows := map[string]*trafficData{
		"intf1-nc1": {
			frameSize:             512,
			trafficRate:           45,
			expectedThroughputPct: 100.0,
			dscp:                  56,
			queue:                 queueMap[dut.Vendor()]["NC1"],
			inputIntf:             intf1,
			burstPackets:          1200,
			burstMinGap:           12,
			burstGap:              48000,
		},
		"intf2-nc1": {
			frameSize:             512,
			trafficRate:           50,
			dscp:                  56,
			expectedThroughputPct: 100.0,
			queue:                 queueMap[dut.Vendor()]["NC1"],
			inputIntf:             intf2,
			burstPackets:          1200,
			burstMinGap:           12,
			burstGap:              96000,
		},
	}

	// Test case 2: Bursty AF4 traffic.
	af4TrafficFlows := map[string]*trafficData{
		"intf1-af4": {
			frameSize:             512,
			trafficRate:           45,
			expectedThroughputPct: 100.0,
			dscp:                  32,
			queue:                 queueMap[dut.Vendor()]["AF4"],
			inputIntf:             intf1,
			burstPackets:          1200,
			burstMinGap:           12,
			burstGap:              48000,
		},
		"intf2-af4": {
			frameSize:             512,
			trafficRate:           50,
			dscp:                  32,
			expectedThroughputPct: 100.0,
			queue:                 queueMap[dut.Vendor()]["AF4"],
			inputIntf:             intf2,
			burstPackets:          1200,
			burstMinGap:           12,
			burstGap:              96000,
		},
	}

	// Test case 3: Bursty AF3 traffic.
	af3TrafficFlows := map[string]*trafficData{
		"intf1-af3": {
			frameSize:             512,
			trafficRate:           45,
			expectedThroughputPct: 100.0,
			dscp:                  24,
			queue:                 queueMap[dut.Vendor()]["AF3"],
			inputIntf:             intf1,
			burstPackets:          1200,
			burstMinGap:           12,
			burstGap:              48000,
		},
		"intf2-af3": {
			frameSize:             512,
			trafficRate:           50,
			dscp:                  24,
			expectedThroughputPct: 100.0,
			queue:                 queueMap[dut.Vendor()]["AF3"],
			inputIntf:             intf2,
			burstPackets:          1200,
			burstMinGap:           12,
			burstGap:              96000,
		},
	}

	// Test case 4: Bursty AF2 traffic.
	af2TrafficFlows := map[string]*trafficData{
		"intf1-af2": {
			frameSize:             512,
			trafficRate:           45,
			expectedThroughputPct: 100.0,
			dscp:                  16,
			queue:                 queueMap[dut.Vendor()]["AF2"],
			inputIntf:             intf1,
			burstPackets:          1200,
			burstMinGap:           12,
			burstGap:              48000,
		},
		"intf2-af2": {
			frameSize:             512,
			trafficRate:           50,
			dscp:                  16,
			expectedThroughputPct: 100.0,
			queue:                 queueMap[dut.Vendor()]["AF2"],
			inputIntf:             intf2,
			burstPackets:          1200,
			burstMinGap:           12,
			burstGap:              96000,
		},
	}

	// Test case 5: Bursty AF1 traffic.
	af1TrafficFlows := map[string]*trafficData{
		"intf1-af1": {
			frameSize:             512,
			trafficRate:           45,
			expectedThroughputPct: 100.0,
			dscp:                  8,
			queue:                 queueMap[dut.Vendor()]["AF1"],
			inputIntf:             intf1,
			burstPackets:          1200,
			burstMinGap:           12,
			burstGap:              48000,
		},
		"intf2-af1": {
			frameSize:             512,
			trafficRate:           50,
			dscp:                  8,
			expectedThroughputPct: 100.0,
			queue:                 queueMap[dut.Vendor()]["AF1"],
			inputIntf:             intf2,
			burstPackets:          1200,
			burstMinGap:           12,
			burstGap:              96000,
		},
	}

	// Test case 6: Bursty BE0 traffic.
	be0TrafficFlows := map[string]*trafficData{
		"intf1-be0": {
			frameSize:             512,
			trafficRate:           45,
			expectedThroughputPct: 100.0,
			dscp:                  4,
			queue:                 queueMap[dut.Vendor()]["BE0"],
			inputIntf:             intf1,
			burstPackets:          1200,
			burstMinGap:           12,
			burstGap:              48000,
		},
		"intf2-be0": {
			frameSize:             512,
			trafficRate:           50,
			dscp:                  4,
			expectedThroughputPct: 100.0,
			queue:                 queueMap[dut.Vendor()]["BE0"],
			inputIntf:             intf2,
			burstPackets:          1200,
			burstMinGap:           12,
			burstGap:              96000,
		},
	}

	// Test case 7: Bursty BE1 traffic.
	be1TrafficFlows := map[string]*trafficData{
		"intf1-be1": {
			frameSize:             512,
			trafficRate:           45,
			expectedThroughputPct: 100.0,
			dscp:                  0,
			queue:                 queueMap[dut.Vendor()]["BE1"],
			inputIntf:             intf1,
			burstPackets:          1200,
			burstMinGap:           12,
			burstGap:              48000,
		},
		"intf2-be1": {
			frameSize:             512,
			trafficRate:           50,
			dscp:                  0,
			expectedThroughputPct: 100.0,
			queue:                 queueMap[dut.Vendor()]["BE1"],
			inputIntf:             intf2,
			burstPackets:          1200,
			burstMinGap:           12,
			burstGap:              96000,
		},
	}

	cases := []struct {
		desc         string
		trafficFlows map[string]*trafficData
	}{{
		desc:         "Bursty NC1 traffic",
		trafficFlows: nc1TrafficFlows,
	}, {
		desc:         "Bursty AF4 traffic",
		trafficFlows: af4TrafficFlows,
	}, {
		desc:         "Bursty AF3 traffic",
		trafficFlows: af3TrafficFlows,
	}, {
		desc:         "Bursty AF2 traffic",
		trafficFlows: af2TrafficFlows,
	}, {
		desc:         "Bursty AF1 traffic",
		trafficFlows: af1TrafficFlows,
	}, {
		desc:         "Bursty BE0 traffic",
		trafficFlows: be0TrafficFlows,
	}, {
		desc:         "Bursty BE1 traffic",
		trafficFlows: be1TrafficFlows,
	}}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			trafficFlows := tc.trafficFlows
			top.Flows().Clear()

			for trafficID, data := range trafficFlows {
				t.Logf("Configuring flow %s", trafficID)
				flow := top.Flows().Add().SetName(trafficID)
				flow.Metrics().SetEnable(true)
				flow.TxRx().Device().SetTxNames([]string{data.inputIntf.Name + ".IPv4"}).SetRxNames([]string{intf3.Name + ".IPv4"})
				ethHeader := flow.Packet().Add().Ethernet()
				ethHeader.Src().SetValue(data.inputIntf.MAC)

				ipHeader := flow.Packet().Add().Ipv4()
				ipHeader.Src().SetValue(data.inputIntf.IPv4)
				ipHeader.Dst().SetValue(intf3.IPv4)
				ipHeader.Priority().Dscp().Phb().SetValue(int32(data.dscp))

				flow.Size().SetFixed(int32(data.frameSize))
				flow.Rate().SetPercentage(float32(data.trafficRate))
				flow.Duration().SetChoice("burst")
				flow.Duration().Burst().SetPackets(int32(data.burstPackets)).SetGap(int32(data.burstMinGap))
				flow.Duration().Burst().InterBurstGap().SetBytes(float64(data.burstGap))

			}
			ate.OTG().PushConfig(t, top)
			ate.OTG().StartProtocols(t)

			ateOutPkts := make(map[string]uint64)
			ateInPkts := make(map[string]uint64)
			dutQosPktsBeforeTraffic := make(map[string]uint64)
			dutQosPktsAfterTraffic := make(map[string]uint64)
			dutQosDroppedPktsBeforeTraffic := make(map[string]uint64)
			dutQosDroppedPktsAfterTraffic := make(map[string]uint64)

			// Set the initial counters to 0.
			for _, data := range trafficFlows {
				ateOutPkts[data.queue] = 0
				ateInPkts[data.queue] = 0
				dutQosPktsBeforeTraffic[data.queue] = 0
				dutQosPktsAfterTraffic[data.queue] = 0
				dutQosDroppedPktsBeforeTraffic[data.queue] = 0
				dutQosDroppedPktsAfterTraffic[data.queue] = 0
			}

			// Get QoS egress packet counters before the traffic.
			for _, data := range trafficFlows {
				dutQosPktsBeforeTraffic[data.queue] += gnmi.Get(t, dut, gnmi.OC().Qos().Interface(dp3.Name()).Output().Queue(data.queue).TransmitPkts().State())
				dutQosDroppedPktsBeforeTraffic[data.queue] += gnmi.Get(t, dut, gnmi.OC().Qos().Interface(dp3.Name()).Output().Queue(data.queue).DroppedPkts().State())
			}

			t.Logf("Running traffic 1 on DUT interfaces: %s => %s ", dp1.Name(), dp3.Name())
			t.Logf("Running traffic 2 on DUT interfaces: %s => %s ", dp2.Name(), dp3.Name())
			t.Logf("Sending traffic flows: \n%v\n\n", trafficFlows)
			ate.OTG().StartTraffic(t)
			time.Sleep(30 * time.Second)
			ate.OTG().StopTraffic(t)
			time.Sleep(30 * time.Second)

			for trafficID, data := range trafficFlows {
				ateOutPkts[data.queue] += gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(trafficID).Counters().OutPkts().State())
				ateInPkts[data.queue] += gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(trafficID).Counters().InPkts().State())
				dutQosPktsAfterTraffic[data.queue] += gnmi.Get(t, dut, gnmi.OC().Qos().Interface(dp3.Name()).Output().Queue(data.queue).TransmitPkts().State())
				dutQosDroppedPktsAfterTraffic[data.queue] += gnmi.Get(t, dut, gnmi.OC().Qos().Interface(dp3.Name()).Output().Queue(data.queue).DroppedPkts().State())
				t.Logf("ateInPkts: %v, txPkts %v, Queue: %v", ateInPkts[data.queue], dutQosPktsAfterTraffic[data.queue], data.queue)

				lossPct := gnmi.Get(t, ate, gnmi.OC().Flow(trafficID).LossPct().State())
				t.Logf("Get flow %q: lossPct: %.2f%% or rxPct: %.2f%%, want: %.2f%%\n\n", data.queue, lossPct, 100.0-lossPct, data.expectedThroughputPct)
				if got, want := 100.0-lossPct, data.expectedThroughputPct; got < want-tolerance || got > want+tolerance {
					t.Errorf("Get(throughput for queue %q): got %.2f%%, want within [%.2f%%, %.2f%%]", data.queue, got, want-tolerance, want+tolerance)
				}
			}

			// Check QoS egress packet counters are updated correctly.
			t.Logf("QoS dutQosPktsBeforeTraffic: %v", dutQosPktsBeforeTraffic)
			t.Logf("QoS dutQosPktsAfterTraffic: %v", dutQosPktsAfterTraffic)
			t.Logf("QoS dutQosDroppedPktsBeforeTraffic: %v", dutQosDroppedPktsBeforeTraffic)
			t.Logf("QoS dutQosDroppedPktsAfterTraffic: %v", dutQosDroppedPktsAfterTraffic)
			t.Logf("QoS ateOutPkts: %v", ateOutPkts)
			t.Logf("QoS ateInPkts: %v", ateInPkts)
			for _, data := range trafficFlows {
				qosCounterDiff := dutQosPktsAfterTraffic[data.queue] - dutQosPktsBeforeTraffic[data.queue]
				ateCounterDiff := ateInPkts[data.queue]
				ateDropCounterDiff := ateOutPkts[data.queue] - ateInPkts[data.queue]
				dutDropCounterDiff := dutQosDroppedPktsAfterTraffic[data.queue] - dutQosDroppedPktsBeforeTraffic[data.queue]
				t.Logf("QoS queue %q: ateDropCounterDiff: %v dutDropCounterDiff: %v", data.queue, ateDropCounterDiff, dutDropCounterDiff)
				if qosCounterDiff < ateCounterDiff {
					t.Errorf("Get telemetry packet update for queue %q: got %v, want >= %v", data.queue, qosCounterDiff, ateCounterDiff)
				}
			}
		})
	}
}

func ConfigureDUTIntf(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")
	dp3 := dut.Port(t, "port3")

	dutIntfs := []struct {
		desc      string
		intfName  string
		ipAddr    string
		prefixLen uint8
	}{{
		desc:      "Input interface port1",
		intfName:  dp1.Name(),
		ipAddr:    "198.51.100.0",
		prefixLen: 31,
	}, {
		desc:      "Input interface port2",
		intfName:  dp2.Name(),
		ipAddr:    "198.51.100.2",
		prefixLen: 31,
	}, {
		desc:      "Output interface port3",
		intfName:  dp3.Name(),
		ipAddr:    "198.51.100.4",
		prefixLen: 31,
	}}

	// Configure the interfaces.
	for _, intf := range dutIntfs {
		t.Logf("Configure DUT interface %s with attributes %v", intf.intfName, intf)
		i := &oc.Interface{
			Name:        ygot.String(intf.intfName),
			Description: ygot.String(intf.desc),
			Type:        oc.IETFInterfaces_InterfaceType_ethernetCsmacd,
			Enabled:     ygot.Bool(true),
		}
		i.GetOrCreateEthernet()
		s := i.GetOrCreateSubinterface(0).GetOrCreateIpv4()
		if *deviations.InterfaceEnabled && !*deviations.IPv4MissingEnabled {
			s.Enabled = ygot.Bool(true)
		}
		a := s.GetOrCreateAddress(intf.ipAddr)
		a.PrefixLength = ygot.Uint8(intf.prefixLen)
		gnmi.Replace(t, dut, gnmi.OC().Interface(intf.intfName).Config(), i)
	}
}

func ConfigureQoS(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")
	dp3 := dut.Port(t, "port3")
	d := &oc.Root{}
	q := d.GetOrCreateQos()

	t.Logf("Create qos forwarding groups config")
	forwardingGroups := []struct {
		desc        string
		queueName   string
		targetGroup string
	}{{
		desc:        "forwarding-group-BE1",
		queueName:   "BE1",
		targetGroup: "target-group-BE1",
	}, {
		desc:        "forwarding-group-BE0",
		queueName:   "BE0",
		targetGroup: "target-group-BE0",
	}, {
		desc:        "forwarding-group-AF1",
		queueName:   "AF1",
		targetGroup: "target-group-AF1",
	}, {
		desc:        "forwarding-group-AF2",
		queueName:   "AF2",
		targetGroup: "target-group-AF2",
	}, {
		desc:        "forwarding-group-AF3",
		queueName:   "AF3",
		targetGroup: "target-group-AF3",
	}, {
		desc:        "forwarding-group-AF4",
		queueName:   "AF4",
		targetGroup: "target-group-AF4",
	}, {
		desc:        "forwarding-group-NC1",
		queueName:   "NC1",
		targetGroup: "target-group-NC1",
	}}

	t.Logf("qos forwarding groups config: %v", forwardingGroups)
	for _, tc := range forwardingGroups {
		fwdGroup := q.GetOrCreateForwardingGroup(tc.targetGroup)
		fwdGroup.SetName(tc.targetGroup)
		fwdGroup.SetOutputQueue(tc.queueName)
		queue := q.GetOrCreateQueue(tc.queueName)
		queue.SetName(tc.queueName)
		gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), q)
	}

	t.Logf("Create qos Classifiers config")
	classifiers := []struct {
		desc        string
		name        string
		classType   oc.E_Qos_Classifier_Type
		termID      string
		targetGroup string
		dscpSet     []uint8
	}{{
		desc:        "classifier_ipv4_be1",
		name:        "dscp_based_classifier_ipv4",
		classType:   oc.Qos_Classifier_Type_IPV4,
		termID:      "0",
		targetGroup: "target-group-BE1",
		dscpSet:     []uint8{0, 1, 2, 3},
	}, {
		desc:        "classifier_ipv4_be0",
		name:        "dscp_based_classifier_ipv4",
		classType:   oc.Qos_Classifier_Type_IPV4,
		termID:      "1",
		targetGroup: "target-group-BE0",
		dscpSet:     []uint8{4, 5, 6, 7},
	}, {
		desc:        "classifier_ipv4_af1",
		name:        "dscp_based_classifier_ipv4",
		classType:   oc.Qos_Classifier_Type_IPV4,
		termID:      "2",
		targetGroup: "target-group-AF1",
		dscpSet:     []uint8{8, 9, 10, 11},
	}, {
		desc:        "classifier_ipv4_af2",
		name:        "dscp_based_classifier_ipv4",
		classType:   oc.Qos_Classifier_Type_IPV4,
		termID:      "3",
		targetGroup: "target-group-AF2",
		dscpSet:     []uint8{16, 17, 18, 19},
	}, {
		desc:        "classifier_ipv4_af3",
		name:        "dscp_based_classifier_ipv4",
		classType:   oc.Qos_Classifier_Type_IPV4,
		termID:      "4",
		targetGroup: "target-group-AF3",
		dscpSet:     []uint8{24, 25, 26, 27},
	}, {
		desc:        "classifier_ipv4_af4",
		name:        "dscp_based_classifier_ipv4",
		classType:   oc.Qos_Classifier_Type_IPV4,
		termID:      "5",
		targetGroup: "target-group-AF4",
		dscpSet:     []uint8{32, 33, 34, 35},
	}, {
		desc:        "classifier_ipv4_nc1",
		name:        "dscp_based_classifier_ipv4",
		classType:   oc.Qos_Classifier_Type_IPV4,
		termID:      "6",
		targetGroup: "target-group-NC1",
		dscpSet:     []uint8{48, 49, 50, 51, 52, 53, 54, 55, 56, 57, 58, 59},
	}, {
		desc:        "classifier_ipv6_be1",
		name:        "dscp_based_classifier_ipv6",
		classType:   oc.Qos_Classifier_Type_IPV6,
		termID:      "0",
		targetGroup: "target-group-BE1",
		dscpSet:     []uint8{0, 1, 2, 3},
	}, {
		desc:        "classifier_ipv6_be0",
		name:        "dscp_based_classifier_ipv6",
		classType:   oc.Qos_Classifier_Type_IPV6,
		termID:      "1",
		targetGroup: "target-group-BE0",
		dscpSet:     []uint8{4, 5, 6, 7},
	}, {
		desc:        "classifier_ipv6_af1",
		name:        "dscp_based_classifier_ipv6",
		classType:   oc.Qos_Classifier_Type_IPV6,
		termID:      "2",
		targetGroup: "target-group-AF1",
		dscpSet:     []uint8{8, 9, 10, 11},
	}, {
		desc:        "classifier_ipv6_af2",
		name:        "dscp_based_classifier_ipv6",
		classType:   oc.Qos_Classifier_Type_IPV6,
		termID:      "3",
		targetGroup: "target-group-AF2",
		dscpSet:     []uint8{16, 17, 18, 19},
	}, {
		desc:        "classifier_ipv6_af3",
		name:        "dscp_based_classifier_ipv6",
		classType:   oc.Qos_Classifier_Type_IPV6,
		termID:      "4",
		targetGroup: "target-group-AF3",
		dscpSet:     []uint8{24, 25, 26, 27},
	}, {
		desc:        "classifier_ipv6_af4",
		name:        "dscp_based_classifier_ipv6",
		classType:   oc.Qos_Classifier_Type_IPV6,
		termID:      "5",
		targetGroup: "target-group-AF4",
		dscpSet:     []uint8{32, 33, 34, 35},
	}, {
		desc:        "classifier_ipv6_nc1",
		name:        "dscp_based_classifier_ipv6",
		classType:   oc.Qos_Classifier_Type_IPV6,
		termID:      "6",
		targetGroup: "target-group-NC1",
		dscpSet:     []uint8{48, 49, 50, 51, 52, 53, 54, 55, 56, 57, 58, 59},
	}}

	t.Logf("qos Classifiers config: %v", classifiers)
	for _, tc := range classifiers {
		classifier := q.GetOrCreateClassifier(tc.name)
		classifier.SetName(tc.name)
		classifier.SetType(tc.classType)
		term, err := classifier.NewTerm(tc.termID)
		if err != nil {
			t.Fatalf("Failed to create classifier.NewTerm(): %v", err)
		}

		term.SetId(tc.termID)
		action := term.GetOrCreateActions()
		action.SetTargetGroup(tc.targetGroup)
		condition := term.GetOrCreateConditions()
		if tc.name == "dscp_based_classifier_ipv4" {
			condition.GetOrCreateIpv4().SetDscpSet(tc.dscpSet)
		} else if tc.name == "dscp_based_classifier_ipv6" {
			condition.GetOrCreateIpv6().SetDscpSet(tc.dscpSet)
		}
		gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), q)
	}

	t.Logf("Create qos input classifier config")
	classifierIntfs := []struct {
		desc                string
		intf                string
		inputClassifierType oc.E_Input_Classifier_Type
		classifier          string
	}{{
		desc:                "Input Classifier Type IPV4",
		intf:                dp1.Name(),
		inputClassifierType: oc.Input_Classifier_Type_IPV4,
		classifier:          "dscp_based_classifier_ipv4",
	}, {
		desc:                "Input Classifier Type IPV6",
		intf:                dp1.Name(),
		inputClassifierType: oc.Input_Classifier_Type_IPV6,
		classifier:          "dscp_based_classifier_ipv6",
	}, {
		desc:                "Input Classifier Type IPV4",
		intf:                dp2.Name(),
		inputClassifierType: oc.Input_Classifier_Type_IPV4,
		classifier:          "dscp_based_classifier_ipv4",
	}, {
		desc:                "Input Classifier Type IPV6",
		intf:                dp2.Name(),
		inputClassifierType: oc.Input_Classifier_Type_IPV6,
		classifier:          "dscp_based_classifier_ipv6",
	}}

	t.Logf("qos input classifier config: %v", classifierIntfs)
	for _, tc := range classifierIntfs {
		i := q.GetOrCreateInterface(tc.intf)
		i.SetInterfaceId(tc.intf)
		c := i.GetOrCreateInput().GetOrCreateClassifier(tc.inputClassifierType)
		c.SetType(tc.inputClassifierType)
		c.SetName(tc.classifier)
		gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), q)
	}

	t.Logf("Create qos scheduler policies config")
	schedulerPolicies := []struct {
		desc        string
		sequence    uint32
		priority    oc.E_Scheduler_Priority
		inputID     string
		inputType   oc.E_Input_InputType
		weight      uint64
		queueName   string
		targetGroup string
	}{{
		desc:        "scheduler-policy-BE1",
		sequence:    uint32(1),
		priority:    oc.Scheduler_Priority_UNSET,
		inputID:     "BE1",
		inputType:   oc.Input_InputType_QUEUE,
		weight:      uint64(1),
		queueName:   "BE1",
		targetGroup: "target-group-BE1",
	}, {
		desc:        "scheduler-policy-BE0",
		sequence:    uint32(1),
		priority:    oc.Scheduler_Priority_UNSET,
		inputID:     "BE0",
		inputType:   oc.Input_InputType_QUEUE,
		weight:      uint64(4),
		queueName:   "BE0",
		targetGroup: "target-group-BE0",
	}, {
		desc:        "scheduler-policy-AF1",
		sequence:    uint32(1),
		priority:    oc.Scheduler_Priority_UNSET,
		inputID:     "AF1",
		inputType:   oc.Input_InputType_QUEUE,
		weight:      uint64(8),
		queueName:   "AF1",
		targetGroup: "target-group-AF1",
	}, {
		desc:        "scheduler-policy-AF2",
		sequence:    uint32(1),
		priority:    oc.Scheduler_Priority_UNSET,
		inputID:     "AF2",
		inputType:   oc.Input_InputType_QUEUE,
		weight:      uint64(16),
		queueName:   "AF2",
		targetGroup: "target-group-AF2",
	}, {
		desc:        "scheduler-policy-AF3",
		sequence:    uint32(1),
		priority:    oc.Scheduler_Priority_UNSET,
		inputID:     "AF3",
		inputType:   oc.Input_InputType_QUEUE,
		weight:      uint64(32),
		queueName:   "AF3",
		targetGroup: "target-group-AF3",
	}, {
		desc:        "scheduler-policy-AF4",
		sequence:    uint32(0),
		priority:    oc.Scheduler_Priority_STRICT,
		inputID:     "AF4",
		inputType:   oc.Input_InputType_QUEUE,
		weight:      uint64(100),
		queueName:   "AF4",
		targetGroup: "target-group-AF4",
	}, {
		desc:        "scheduler-policy-NC1",
		sequence:    uint32(0),
		priority:    oc.Scheduler_Priority_STRICT,
		inputID:     "NC1",
		inputType:   oc.Input_InputType_QUEUE,
		weight:      uint64(200),
		queueName:   "NC1",
		targetGroup: "target-group-NC1",
	}}

	schedulerPolicy := q.GetOrCreateSchedulerPolicy("scheduler")
	schedulerPolicy.SetName("scheduler")
	t.Logf("qos scheduler policies config: %v", schedulerPolicies)
	for _, tc := range schedulerPolicies {
		s := schedulerPolicy.GetOrCreateScheduler(tc.sequence)
		s.SetSequence(tc.sequence)
		s.SetPriority(tc.priority)
		input := s.GetOrCreateInput(tc.inputID)
		input.SetId(tc.inputID)
		input.SetInputType(tc.inputType)
		input.SetQueue(tc.queueName)
		input.SetWeight(tc.weight)
		gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), q)
	}

	t.Logf("Create qos output interface config")
	schedulerIntfs := []struct {
		desc      string
		queueName string
		scheduler string
	}{{
		desc:      "output-interface-BE1",
		queueName: "BE1",
		scheduler: "scheduler",
	}, {
		desc:      "output-interface-BE0",
		queueName: "BE0",
		scheduler: "scheduler",
	}, {
		desc:      "output-interface-AF1",
		queueName: "AF1",
		scheduler: "scheduler",
	}, {
		desc:      "output-interface-AF2",
		queueName: "AF2",
		scheduler: "scheduler",
	}, {
		desc:      "output-interface-AF3",
		queueName: "AF3",
		scheduler: "scheduler",
	}, {
		desc:      "output-interface-AF4",
		queueName: "AF4",
		scheduler: "scheduler",
	}, {
		desc:      "output-interface-NC1",
		queueName: "NC1",
		scheduler: "scheduler",
	}}

	t.Logf("qos output interface config: %v", schedulerIntfs)
	for _, tc := range schedulerIntfs {
		i := q.GetOrCreateInterface(dp3.Name())
		i.SetInterfaceId(dp3.Name())
		output := i.GetOrCreateOutput()
		schedulerPolicy := output.GetOrCreateSchedulerPolicy()
		schedulerPolicy.SetName(tc.scheduler)
		queue := output.GetOrCreateQueue(tc.queueName)
		queue.SetName(tc.queueName)
		gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), q)
	}
}
