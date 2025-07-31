// Copyright 2025 Google LLC
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
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/featureprofiles/internal/qoscfg"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	otgtelemetry "github.com/openconfig/ondatra/gnmi/otg"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	v4PrefixLen = 31
)

var (
	dutPort1 = attrs.Attributes{
		IPv4:    "198.51.100.0",
		IPv4Len: v4PrefixLen,
	}

	dutPort2 = attrs.Attributes{
		IPv4:    "198.51.100.2",
		IPv4Len: v4PrefixLen,
	}

	dutPort3 = attrs.Attributes{
		IPv4:    "198.51.100.4",
		IPv4Len: v4PrefixLen,
	}

	atePort1 = attrs.Attributes{
		Name:    "port1",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "198.51.100.1",
		IPv4Len: v4PrefixLen,
	}

	atePort2 = attrs.Attributes{
		Name:    "port2",
		MAC:     "02:00:01:01:01:02",
		IPv4:    "198.51.100.3",
		IPv4Len: v4PrefixLen,
	}

	atePort3 = attrs.Attributes{
		Name:    "port3",
		MAC:     "02:00:01:01:01:03",
		IPv4:    "198.51.100.5",
		IPv4Len: v4PrefixLen,
	}
)

type trafficData struct {
	trafficRate           float64
	expectedThroughputPct float32
	frameSize             uint32
	dscp                  uint8
	queue                 string
	inputIntf             attrs.Attributes
	ecnValue              uint8
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestECNEnabledTraffic(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")
	p3 := dut.Port(t, "port3")
	configureDUT(t, dut)

	ate := ondatra.ATE(t, "ate")
	top, _ := configureOTG(t, ate)

	configureQoS(t, dut)
	queues := netutil.CommonTrafficQueues(t, dut)

	var tolerance float32 = 2.0

	oversubscribedTrafficFlows := map[string][]*trafficData{
		"nc1-flow": {
			{
				queue:                 queues.NC1,
				inputIntf:             atePort1,
				trafficRate:           51,
				expectedThroughputPct: 51,
				frameSize:             1000,
				dscp:                  56,
				ecnValue:              3,
			},
			{
				queue:                 queues.NC1,
				inputIntf:             atePort2,
				trafficRate:           50,
				expectedThroughputPct: 50,
				frameSize:             1000,
				dscp:                  56,
				ecnValue:              3,
			},
		},
		"af4-flow": {
			{
				queue:                 queues.AF4,
				inputIntf:             atePort1,
				trafficRate:           51,
				expectedThroughputPct: 51,
				frameSize:             1000,
				dscp:                  32,
				ecnValue:              2,
			},
			{
				queue:                 queues.AF4,
				inputIntf:             atePort2,
				trafficRate:           50,
				expectedThroughputPct: 50,
				frameSize:             1000,
				dscp:                  32,
				ecnValue:              2,
			},
		},
		"af3-flow": {
			{
				queue:                 queues.AF3,
				inputIntf:             atePort1,
				trafficRate:           51,
				expectedThroughputPct: 51,
				frameSize:             1000,
				dscp:                  24,
				ecnValue:              2,
			},
			{
				queue:                 queues.AF3,
				inputIntf:             atePort2,
				trafficRate:           50,
				expectedThroughputPct: 50,
				frameSize:             1000,
				dscp:                  24,
				ecnValue:              2,
			},
		},
		"af2-flow": {
			{
				queue:                 queues.AF2,
				inputIntf:             atePort1,
				trafficRate:           51,
				expectedThroughputPct: 51,
				frameSize:             1000,
				dscp:                  16,
				ecnValue:              2,
			},
			{
				queue:                 queues.AF2,
				inputIntf:             atePort2,
				trafficRate:           50,
				expectedThroughputPct: 50,
				frameSize:             1000,
				dscp:                  16,
				ecnValue:              2,
			},
		},
		"af1-flow": {
			{
				queue:                 queues.AF1,
				inputIntf:             atePort1,
				trafficRate:           51,
				expectedThroughputPct: 51,
				frameSize:             1000,
				dscp:                  8,
				ecnValue:              2,
			},
			{
				queue:                 queues.AF1,
				inputIntf:             atePort2,
				trafficRate:           50,
				expectedThroughputPct: 50,
				frameSize:             1000,
				dscp:                  8,
				ecnValue:              2,
			},
		},
		"be0-flow": {
			{
				queue:                 queues.BE0,
				inputIntf:             atePort1,
				trafficRate:           51,
				expectedThroughputPct: 51,
				frameSize:             1000,
				dscp:                  4,
				ecnValue:              2,
			},
			{
				queue:                 queues.BE0,
				inputIntf:             atePort2,
				trafficRate:           50,
				expectedThroughputPct: 50,
				frameSize:             1000,
				dscp:                  4,
				ecnValue:              2,
			},
		},
		"be1-flow": {
			{
				queue:                 queues.BE1,
				inputIntf:             atePort1,
				trafficRate:           51,
				expectedThroughputPct: 51,
				frameSize:             1000,
				dscp:                  0,
				ecnValue:              2,
			},
			{
				queue:                 queues.BE1,
				inputIntf:             atePort2,
				trafficRate:           50,
				expectedThroughputPct: 50,
				frameSize:             1000,
				dscp:                  0,
				ecnValue:              2,
			},
		},
	}

	for fn, tfs := range oversubscribedTrafficFlows {
		t.Run(fn, func(t *testing.T) {
			top.Flows().Clear()
			for _, tf := range tfs {
				name := fn + "-" + tf.inputIntf.Name
				t.Logf("Configuring flow %s", name)
				flow := top.Flows().Add().SetName(name)
				flow.Metrics().SetEnable(true)
				flow.TxRx().Device().SetTxNames([]string{tf.inputIntf.Name + ".IPv4"}).SetRxNames([]string{atePort3.Name + ".IPv4"})
				ethHeader := flow.Packet().Add().Ethernet()
				ethHeader.Src().SetValue(tf.inputIntf.MAC)

				ipHeader := flow.Packet().Add().Ipv4()
				ipHeader.Src().SetValue(tf.inputIntf.IPv4)
				ipHeader.Dst().SetValue(atePort3.IPv4)
				ipHeader.Priority().Dscp().Phb().SetValue(uint32(tf.dscp))
				ipHeader.Priority().Raw().SetValue(trafficClassFieldsToDecimal(tf.dscp, tf.ecnValue))

				tracking := flow.EgressPacket().Add().Ipv4()
				tracking.Priority().Raw().MetricTags().Add().SetName(fmt.Sprintf("dst-dscp-%d-%s", tf.dscp, tf.inputIntf.Name)).SetOffset(0).SetLength(6)
				tracking.Priority().Raw().MetricTags().Add().SetName(fmt.Sprintf("dst-ecn-%d-%s", tf.dscp, tf.inputIntf.Name)).SetOffset(6).SetLength(2)

				flow.Size().SetFixed(uint32(tf.frameSize))
				flow.Rate().SetPercentage(float32(tf.trafficRate))
			}

			ate.OTG().PushConfig(t, top)
			ate.OTG().StartProtocols(t)
			otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")

			ateOutPkts := make(map[string]uint64)
			ateInPkts := make(map[string]uint64)
			dutQosPktsBeforeTraffic := make(map[string]uint64)
			dutQosPktsAfterTraffic := make(map[string]uint64)
			dutQosDroppedPktsBeforeTraffic := make(map[string]uint64)
			dutQosDroppedPktsAfterTraffic := make(map[string]uint64)

			// Set the initial counters to 0.
			for _, data := range tfs {
				ateOutPkts[data.queue] = 0
				ateInPkts[data.queue] = 0
				dutQosPktsBeforeTraffic[data.queue] = 0
				dutQosPktsAfterTraffic[data.queue] = 0
				dutQosDroppedPktsBeforeTraffic[data.queue] = 0
				dutQosDroppedPktsAfterTraffic[data.queue] = 0
			}

			// Get QoS egress packet counters before the traffic.
			const timeout = time.Minute
			isPresent := func(val *ygnmi.Value[uint64]) bool { return val.IsPresent() }
			for _, data := range tfs {
				count, ok := gnmi.Watch(t, dut, gnmi.OC().Qos().Interface(p3.Name()).Output().Queue(data.queue).TransmitPkts().State(), timeout, isPresent).Await(t)
				if !ok {
					t.Errorf("TransmitPkts count for queue %q on interface %q not available within %v", p3.Name(), data.queue, timeout)
				}
				dutQosPktsBeforeTraffic[data.queue], _ = count.Val()

				count, ok = gnmi.Watch(t, dut, gnmi.OC().Qos().Interface(p3.Name()).Output().Queue(data.queue).DroppedPkts().State(), timeout, isPresent).Await(t)
				if !ok {
					t.Errorf("DroppedPkts count for queue %q on interface %q not available within %v", p3.Name(), data.queue, timeout)
				}
				dutQosDroppedPktsBeforeTraffic[data.queue], _ = count.Val()
			}

			t.Logf("Running traffic 1 on DUT interfaces: %s => %s ", p1.Name(), p3.Name())
			t.Logf("Running traffic 2 on DUT interfaces: %s => %s ", p2.Name(), p3.Name())
			t.Logf("Sending traffic flows: \n%v\n\n", tfs)
			ate.OTG().StartTraffic(t)
			time.Sleep(30 * time.Second)
			ate.OTG().StopTraffic(t)
			time.Sleep(30 * time.Second)

			otgutils.LogFlowMetrics(t, ate.OTG(), top)
			for _, data := range tfs {
				name := fn + "-" + data.inputIntf.Name
				flowData := gnmi.Get[*otgtelemetry.Flow](t, ate.OTG(), gnmi.OTG().Flow(name).State())

				ateOutPkts[data.queue] += flowData.GetCounters().GetOutPkts()
				ateInPkts[data.queue] += flowData.GetCounters().GetOutPkts()
				dutQosPktsAfterTraffic[data.queue] += gnmi.Get(t, dut, gnmi.OC().Qos().Interface(p3.Name()).Output().Queue(data.queue).TransmitPkts().State())
				dutQosDroppedPktsAfterTraffic[data.queue] += gnmi.Get(t, dut, gnmi.OC().Qos().Interface(p3.Name()).Output().Queue(data.queue).DroppedPkts().State())
				t.Logf("ateInPkts: %v, txPkts %v, Queue: %v", ateInPkts[data.queue], dutQosPktsAfterTraffic[data.queue], data.queue)

				ateTxPkts := float32(flowData.GetCounters().GetOutPkts())
				ateRxPkts := float32(flowData.GetCounters().GetInPkts())
				if ateTxPkts == 0 {
					t.Fatalf("TxPkts == 0, want >0.")
				}
				lossPct := (ateTxPkts - ateRxPkts) * 100 / ateTxPkts
				t.Logf("Get flow %q: lossPct: %.2f%% or rxPct: %.2f%%, want: %.2f%%\n\n", data.queue, lossPct, 100.0-lossPct, data.expectedThroughputPct)
				if got, want := 100.0-lossPct, data.expectedThroughputPct; got < want-tolerance || got > want+tolerance {
					t.Errorf("Get(throughput for queue %q): got %.2f%%, want within [%.2f%%, %.2f%%]", data.queue, got, want-tolerance, want+tolerance)
				}

				ets := flowData.TaggedMetric
				dscpAsHex := fmt.Sprintf("0x%02x", data.dscp)
				if len(ets) != 1 {
					t.Logf("got %d flows, but expected one, this probably indicates that the flow has"+
						" some packets tagged 01 and some tagged 11 (congestion experienced) -- "+
						"this should not happen in this test case, will continue validation...", len(ets))
				}

				for _, et := range ets {
					if len(et.Tags) != 2 {
						t.Errorf("expected two metric tags (dscp/ecn) but got %d", len(et.Tags))
					}

					for _, tag := range et.Tags {
						tagName := tag.GetTagName()
						valueAsHex := tag.GetTagValue().GetValueAsHex()
						t.Logf("flow with dscp value %d, tag name %q, got value %s", data.dscp, tagName, valueAsHex)
						if strings.Contains(tagName, "dscp") {
							if valueAsHex != dscpAsHex {
								t.Errorf("expected dscp bit to be %x, but got %s", dscpAsHex, valueAsHex)
							}
						} else {
							if data.queue == queues.NC1 && valueAsHex != "0x3" {
								// NC1 should be 11 -- ecn capable and congestion experienced.
								t.Errorf("expected ecn bit to be 0x3, but got %s", valueAsHex)
							} else if valueAsHex != "0x2" {
								// ECN should be 10 -- ecn capable but no congestion experienced.
								t.Errorf("expected ecn bit to be 0x2, but got %s", valueAsHex)
							}
						}
					}
				}
			}

			// Check QoS egress packet counters are updated correctly.
			t.Logf("QoS dutQosPktsBeforeTraffic: %v", dutQosPktsBeforeTraffic)
			t.Logf("QoS dutQosPktsAfterTraffic: %v", dutQosPktsAfterTraffic)
			t.Logf("QoS dutQosDroppedPktsBeforeTraffic: %v", dutQosDroppedPktsBeforeTraffic)
			t.Logf("QoS dutQosDroppedPktsAfterTraffic: %v", dutQosDroppedPktsAfterTraffic)
			t.Logf("QoS ateOutPkts: %v", ateOutPkts)
			t.Logf("QoS ateInPkts: %v", ateInPkts)
			for _, data := range tfs {
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

func trafficClassFieldsToDecimal(dscpValue, ecnValue uint8) uint32 {
	dscpByte := byte(dscpValue)
	ecnByte := byte(ecnValue)
	tosStr := fmt.Sprintf("%06b%02b", dscpByte, ecnByte)
	tosDec, _ := strconv.ParseInt(tosStr, 2, 64)
	return uint32(tosDec)
}

func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	b := &gnmi.SetBatch{}
	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")
	p3 := dut.Port(t, "port3")
	gnmi.BatchReplace(b, gnmi.OC().Interface(p1.Name()).Config(), dutPort1.NewOCInterface(p1.Name(), dut))
	gnmi.BatchReplace(b, gnmi.OC().Interface(p2.Name()).Config(), dutPort2.NewOCInterface(p2.Name(), dut))
	gnmi.BatchReplace(b, gnmi.OC().Interface(p3.Name()).Config(), dutPort3.NewOCInterface(p3.Name(), dut))
	b.Set(t, dut)

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, p1)
		fptest.SetPortSpeed(t, p2)
		fptest.SetPortSpeed(t, p3)
	}

	fptest.ConfigureDefaultNetworkInstance(t, dut)

	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, p1.Name(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, p2.Name(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, p3.Name(), deviations.DefaultNetworkInstance(dut), 0)
	}
}

func configureOTG(t *testing.T, ate *ondatra.ATEDevice) (gosnappi.Config, []gosnappi.Device) {
	t.Helper()

	top := gosnappi.NewConfig()
	p1 := ate.Port(t, "port1")
	p2 := ate.Port(t, "port2")
	p3 := ate.Port(t, "port3")

	d1 := atePort1.AddToOTG(top, p1, &dutPort1)
	d2 := atePort2.AddToOTG(top, p2, &dutPort2)
	d3 := atePort3.AddToOTG(top, p3, &dutPort3)
	return top, []gosnappi.Device{d1, d2, d3}
}

func configureQoS(t *testing.T, dut *ondatra.DUTDevice) {
	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")
	p3 := dut.Port(t, "port3")

	d := &oc.Root{}
	q := d.GetOrCreateQos()
	queues := netutil.CommonTrafficQueues(t, dut)

	if deviations.QOSQueueRequiresID(dut) {
		queueNames := []string{queues.NC1, queues.AF4, queues.AF3, queues.AF2, queues.AF1, queues.BE0, queues.BE1}
		for i, queue := range queueNames {
			q1 := q.GetOrCreateQueue(queue)
			q1.Name = ygot.String(queue)
			queueid := len(queueNames) - i
			q1.QueueId = ygot.Uint8(uint8(queueid))
		}
	}
	if dut.Vendor() == ondatra.JUNIPER {
		queues.AF4 = "5"
	}

	t.Logf("Create qos forwarding groups config")
	forwardingGroups := []struct {
		desc        string
		queueName   string
		targetGroup string
	}{{
		desc:        "forwarding-group-BE1",
		queueName:   queues.BE1,
		targetGroup: "target-group-BE1",
	}, {
		desc:        "forwarding-group-BE0",
		queueName:   queues.BE0,
		targetGroup: "target-group-BE0",
	}, {
		desc:        "forwarding-group-AF1",
		queueName:   queues.AF1,
		targetGroup: "target-group-AF1",
	}, {
		desc:        "forwarding-group-AF2",
		queueName:   queues.AF2,
		targetGroup: "target-group-AF2",
	}, {
		desc:        "forwarding-group-AF3",
		queueName:   queues.AF3,
		targetGroup: "target-group-AF3",
	}, {
		desc:        "forwarding-group-AF4",
		queueName:   queues.AF4,
		targetGroup: "target-group-AF4",
	}, {
		desc:        "forwarding-group-NC1",
		queueName:   queues.NC1,
		targetGroup: "target-group-NC1",
	}}

	t.Logf("qos forwarding groups config: %v", forwardingGroups)
	for _, tc := range forwardingGroups {
		qoscfg.SetForwardingGroup(t, dut, q, tc.targetGroup, tc.queueName)
	}

	t.Logf("Create qos queue management profile config")
	qmp := q.GetOrCreateQueueManagementProfile("queueManagementProfile")
	wup := qmp.GetOrCreateWred().GetOrCreateUniform()
	wup.SetEnableEcn(true)
	wup.SetMinThreshold(uint64(80_000))
	wup.SetMaxThreshold(math.MaxUint32)
	wup.SetDrop(false)
	wup.SetMaxDropProbabilityPercent(uint8(1))

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
		intf:                p1.Name(),
		inputClassifierType: oc.Input_Classifier_Type_IPV4,
		classifier:          "dscp_based_classifier_ipv4",
	}, {
		desc:                "Input Classifier Type IPV6",
		intf:                p1.Name(),
		inputClassifierType: oc.Input_Classifier_Type_IPV6,
		classifier:          "dscp_based_classifier_ipv6",
	}, {
		desc:                "Input Classifier Type IPV4",
		intf:                p2.Name(),
		inputClassifierType: oc.Input_Classifier_Type_IPV4,
		classifier:          "dscp_based_classifier_ipv4",
	}, {
		desc:                "Input Classifier Type IPV6",
		intf:                p2.Name(),
		inputClassifierType: oc.Input_Classifier_Type_IPV6,
		classifier:          "dscp_based_classifier_ipv6",
	}}

	t.Logf("qos input classifier config: %v", classifierIntfs)
	for _, tc := range classifierIntfs {
		qoscfg.SetInputClassifier(t, dut, q, tc.intf, tc.inputClassifierType, tc.classifier)
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
		queueName:   queues.BE1,
		targetGroup: "target-group-BE1",
	}, {
		desc:        "scheduler-policy-BE0",
		sequence:    uint32(1),
		priority:    oc.Scheduler_Priority_UNSET,
		inputID:     "BE0",
		inputType:   oc.Input_InputType_QUEUE,
		weight:      uint64(2),
		queueName:   queues.BE0,
		targetGroup: "target-group-BE0",
	}, {
		desc:        "scheduler-policy-AF1",
		sequence:    uint32(1),
		priority:    oc.Scheduler_Priority_UNSET,
		inputID:     "AF1",
		inputType:   oc.Input_InputType_QUEUE,
		weight:      uint64(4),
		queueName:   queues.AF1,
		targetGroup: "target-group-AF1",
	}, {
		desc:        "scheduler-policy-AF2",
		sequence:    uint32(1),
		priority:    oc.Scheduler_Priority_UNSET,
		inputID:     "AF2",
		inputType:   oc.Input_InputType_QUEUE,
		weight:      uint64(8),
		queueName:   queues.AF2,
		targetGroup: "target-group-AF2",
	}, {
		desc:        "scheduler-policy-AF3",
		sequence:    uint32(1),
		priority:    oc.Scheduler_Priority_UNSET,
		inputID:     "AF3",
		inputType:   oc.Input_InputType_QUEUE,
		weight:      uint64(12),
		queueName:   queues.AF3,
		targetGroup: "target-group-AF3",
	}, {
		desc:        "scheduler-policy-AF4",
		sequence:    uint32(1),
		priority:    oc.Scheduler_Priority_UNSET,
		inputID:     "AF4",
		inputType:   oc.Input_InputType_QUEUE,
		weight:      uint64(48),
		queueName:   queues.AF4,
		targetGroup: "target-group-AF4",
	}, {
		desc:        "scheduler-policy-NC1",
		sequence:    uint32(0),
		priority:    oc.Scheduler_Priority_STRICT,
		inputID:     "NC1",
		inputType:   oc.Input_InputType_QUEUE,
		weight:      uint64(100),
		queueName:   queues.NC1,
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
		queueName: queues.BE1,
		scheduler: "scheduler",
	}, {
		desc:      "output-interface-BE0",
		queueName: queues.BE0,
		scheduler: "scheduler",
	}, {
		desc:      "output-interface-AF1",
		queueName: queues.AF1,
		scheduler: "scheduler",
	}, {
		desc:      "output-interface-AF2",
		queueName: queues.AF2,
		scheduler: "scheduler",
	}, {
		desc:      "output-interface-AF3",
		queueName: queues.AF3,
		scheduler: "scheduler",
	}, {
		desc:      "output-interface-AF4",
		queueName: queues.AF4,
		scheduler: "scheduler",
	}, {
		desc:      "output-interface-NC1",
		queueName: queues.NC1,
		scheduler: "scheduler",
	}}

	t.Logf("qos output interface config: %v", schedulerIntfs)
	for _, tc := range schedulerIntfs {
		i := q.GetOrCreateInterface(p3.Name())
		i.SetInterfaceId(p3.Name())
		i.GetOrCreateInterfaceRef().Interface = ygot.String(p3.Name())
		if deviations.InterfaceRefConfigUnsupported(dut) {
			i.InterfaceRef = nil
		}
		output := i.GetOrCreateOutput()
		schedulerPolicy := output.GetOrCreateSchedulerPolicy()
		schedulerPolicy.SetName(tc.scheduler)
		queue := output.GetOrCreateQueue(tc.queueName)
		queue.SetName(tc.queueName)
		gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), q)
	}
}
