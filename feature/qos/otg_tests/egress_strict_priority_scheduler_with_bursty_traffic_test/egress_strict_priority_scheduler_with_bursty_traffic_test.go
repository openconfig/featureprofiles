// Copyright 2025 Google LLC
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

package egress_strict_priority_scheduler_with_bursty_traffic_test

import (
	"context"
	"fmt"
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
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

var (
	dutIngressPort1AteP1 = attrs.Attributes{IPv4: "198.51.100.1", IPv4Len: 30, IPv6: "2001:db8::1", IPv6Len: 126}
	dutIngressPort2AteP2 = attrs.Attributes{IPv4: "198.51.100.5", IPv4Len: 30, IPv6: "2001:db8::5", IPv6Len: 126}
	dutEgressPort3AteP3  = attrs.Attributes{IPv4: "198.51.100.9", IPv4Len: 30, IPv6: "2001:db8::9", IPv6Len: 126}

	ateTxP1 = attrs.Attributes{Name: "ate1", MAC: "00:01:01:01:01:01", IPv4: "198.51.100.2", IPv4Len: 30, IPv6: "2001:db8::2", IPv6Len: 126}
	ateTxP2 = attrs.Attributes{Name: "ate2", MAC: "00:01:01:01:01:02", IPv4: "198.51.100.6", IPv4Len: 30, IPv6: "2001:db8::6", IPv6Len: 126}
	ateRxP3 = attrs.Attributes{Name: "ate3", MAC: "00:01:01:01:01:03", IPv4: "198.51.100.10", IPv4Len: 30, IPv6: "2001:db8::a", IPv6Len: 126}

	mplsLabel uint32  = 1001
	tolerance float32 = 5.0
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestEgressStrictPrioritySchedulerBurstTraffic(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	t.Logf("Configuring TCAM Profile")
	configureTcamProfile(t, dut)

	t.Logf("Configuring QoS Global parameters")
	configureQoSGlobalParams(t, dut)

	verifyEgressStrictPrioritySchedulerBurstTrafficIPv4(t, dut)
	verifyEgressStrictPrioritySchedulerBurstTrafficIPv6(t, dut)
	verifyEgressStrictPrioritySchedulerBurstTrafficMPLS(t, dut)
}

func verifyEgressStrictPrioritySchedulerBurstTrafficIPv4(t *testing.T, dut *ondatra.DUTDevice) {

	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")
	dp3 := dut.Port(t, "port3")

	// Configure DUT interfaces and QoS.
	ConfigureDUTIntfIPv4(t, dut)
	ConfigureDUTQoSIPv4(t, dut)

	// Configure ATE interfaces.
	ate := ondatra.ATE(t, "ate")
	ap1 := ate.Port(t, "port1")
	ap2 := ate.Port(t, "port2")
	ap3 := ate.Port(t, "port3")

	top := gosnappi.NewConfig()

	ateTxP1.AddToOTG(top, ap1, &dutIngressPort1AteP1)
	ateTxP2.AddToOTG(top, ap2, &dutIngressPort2AteP2)
	ateRxP3.AddToOTG(top, ap3, &dutEgressPort3AteP3)
	ate.OTG().PushConfig(t, top)

	createTrafficFlowsIPv4(t, ate, top, dut)
	t.Run("\n*** Running test for IPv4 ***\n", func(t *testing.T) {

		ate.OTG().StartProtocols(t)
		otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")

		queues := netutil.CommonTrafficQueues(t, dut)

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

		trafficFlows := map[string]*trafficData{
			"ateTxP1-regular-nc1": {
				frameSize:             512,
				trafficRate:           1,
				expectedThroughputPct: 100.0,
				dscp:                  6,
				queue:                 queues.NC1,
				inputIntf:             ateTxP1,
			},
			"ateTxP2-burst-nc1": {
				frameSize:             256,
				trafficRate:           10,
				dscp:                  7,
				expectedThroughputPct: 100.0,
				queue:                 queues.NC1,
				inputIntf:             ateTxP2,
				burstPackets:          50000,
				burstMinGap:           12,
				burstGap:              100,
			},
			"ateTxP1-regular-af4": {
				frameSize:             512,
				trafficRate:           30,
				expectedThroughputPct: 100.0,
				dscp:                  4,
				queue:                 queues.AF4,
				inputIntf:             ateTxP1,
			},
			"ateTxP2-burst-af4": {
				frameSize:             256,
				trafficRate:           20,
				dscp:                  5,
				expectedThroughputPct: 100.0,
				queue:                 queues.AF4,
				inputIntf:             ateTxP2,
				burstPackets:          50000,
				burstMinGap:           12,
				burstGap:              100,
			},
			"ateTxP1-regular-af3": {
				frameSize:             512,
				trafficRate:           12,
				expectedThroughputPct: 100.0,
				dscp:                  3,
				queue:                 queues.AF3,
				inputIntf:             ateTxP1,
			},
			"ateTxP2-burst-af3": {
				frameSize:             256,
				trafficRate:           10,
				dscp:                  3,
				expectedThroughputPct: 100.0,
				queue:                 queues.AF3,
				inputIntf:             ateTxP2,
				burstPackets:          50000,
				burstMinGap:           12,
				burstGap:              100,
			},
			"ateTxP1-regular-af2": {
				frameSize:             512,
				trafficRate:           15,
				expectedThroughputPct: 50.0,
				dscp:                  2,
				queue:                 queues.AF2,
				inputIntf:             ateTxP1,
			},
			"ateTxP2-burst-af2": {
				frameSize:             256,
				trafficRate:           17,
				dscp:                  2,
				expectedThroughputPct: 50.0,
				queue:                 queues.AF2,
				inputIntf:             ateTxP2,
				burstPackets:          50000,
				burstMinGap:           12,
				burstGap:              100,
			},
			"ateTxP1-regular-af1": {
				frameSize:             512,
				trafficRate:           12,
				expectedThroughputPct: 0.0,
				dscp:                  1,
				queue:                 queues.AF1,
				inputIntf:             ateTxP1,
			},
			"ateTxP2-burst-af1": {
				frameSize:             256,
				trafficRate:           13,
				dscp:                  1,
				expectedThroughputPct: 0.0,
				queue:                 queues.AF1,
				inputIntf:             ateTxP2,
				burstPackets:          50000,
				burstMinGap:           12,
				burstGap:              100,
			},
			"ateTxP1-regular-be1": {
				frameSize:             512,
				trafficRate:           12,
				expectedThroughputPct: 0.0,
				dscp:                  0,
				queue:                 queues.BE1,
				inputIntf:             ateTxP1,
			},
			"ateTxP2-burst-be1": {
				frameSize:             256,
				trafficRate:           20,
				expectedThroughputPct: 0.0,
				dscp:                  0,
				queue:                 queues.BE1,
				inputIntf:             ateTxP2,
				burstPackets:          50000,
				burstMinGap:           12,
				burstGap:              100,
			},
		}

		if deviations.InterfaceOutputQueueNonStandardName(dut) {

			// Configuring the non-standard queue names.
			for flowName, data := range trafficFlows {
				if strings.Contains(strings.ToUpper(flowName), "BE1") {
					data.queue = dp3.Name() + "-" + strconv.Itoa(0)
				}
				if strings.Contains(strings.ToUpper(flowName), "AF1") {
					data.queue = dp3.Name() + "-" + strconv.Itoa(1)
				}
				if strings.Contains(strings.ToUpper(flowName), "AF2") {
					data.queue = dp3.Name() + "-" + strconv.Itoa(2)
				}
				if strings.Contains(strings.ToUpper(flowName), "AF3") {
					data.queue = dp3.Name() + "-" + strconv.Itoa(3)
				}
				if strings.Contains(strings.ToUpper(flowName), "AF4") {
					data.queue = dp3.Name() + "-" + strconv.Itoa(4)
				}
				if strings.Contains(strings.ToUpper(flowName), "NC1") {
					data.queue = dp3.Name() + "-" + strconv.Itoa(5)
				}
			}

		}

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

		uniqueQueues := make(map[string]bool)
		for _, data := range trafficFlows {
			uniqueQueues[data.queue] = true
		}

		// Get QoS egress packet counters before the traffic.
		const timeout = 10 * time.Second
		isPresent := func(val *ygnmi.Value[uint64]) bool { return val.IsPresent() }

		for queue := range uniqueQueues {
			count, ok := gnmi.Watch(t, dut, gnmi.OC().Qos().Interface(dp3.Name()).Output().Queue(queue).TransmitPkts().State(), timeout, isPresent).Await(t)
			if !ok {
				t.Errorf("TransmitPkts count for queue %s on interface %q not available within %v", queue, dp3.Name(), timeout)
			}
			dutQosPktsBeforeTraffic[queue], _ = count.Val()

			count, ok = gnmi.Watch(t, dut, gnmi.OC().Qos().Interface(dp3.Name()).Output().Queue(queue).DroppedPkts().State(), timeout, isPresent).Await(t)
			if !ok {
				t.Errorf("DroppedPkts count for queue %s on interface %q not available within %v", dp3.Name(), queue, timeout)
			}
			dutQosDroppedPktsBeforeTraffic[queue], _ = count.Val()
		}

		t.Logf("Before TX values map: %v\n", dutQosPktsBeforeTraffic)
		t.Logf("Before DROP values map: %v\n", dutQosDroppedPktsBeforeTraffic)

		t.Logf("Running regular traffic on DUT interfaces: %s => %s \n", dp1.Name(), dp3.Name())
		t.Logf("Running bursty traffic on DUT interfaces: %s => %s \n", dp2.Name(), dp3.Name())
		t.Logf("Sending traffic flows:\n")

		ate.OTG().StartTraffic(t)

		time.Sleep(15 * time.Second)

		ate.OTG().StopTraffic(t)

		for flowName := range trafficFlows {
			waitForTraffic(t, ate.OTG(), flowName, 10)
		}

		t.Logf("Printing aggregated flow metrics from OTG: \n")
		otgutils.LogFlowMetrics(t, ate.OTG(), top)

		for trafficID, data := range trafficFlows {

			t.Logf("Retrieving statistics for %s\n", trafficID)
			ateTxPkts := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(trafficID).Counters().OutPkts().State())
			ateRxPkts := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(trafficID).Counters().InPkts().State())
			if ateTxPkts == 0 {
				t.Fatalf("Flow %s did not send any packets, please check tester configuration.\n", trafficID)
			}

			lossPct := (float32)((float64(ateTxPkts-ateRxPkts) * 100.0) / float64(ateTxPkts))

			t.Logf("Flow: %s\t\t| EgressQueue:%s\t\t| Loss%%: %.2f%%\t\t| Rx%%:%.2f%%\t\t| Tolerance%%: %.2f%%", trafficID, data.queue, lossPct, 100-lossPct, tolerance)

			if got, want := 100.0-lossPct, data.expectedThroughputPct; got < want-tolerance || got > want+tolerance {
				t.Errorf("Expected throughput for queue %q should be within [%.2f%%, %.2f%%]: got %.2f%%\n", data.queue, want-tolerance, want+tolerance, got)
			}

			ateOutPkts[data.queue] += ateTxPkts
			ateInPkts[data.queue] += ateRxPkts

		}

		header := "| %-20s | %-20s | %-18s | %-18s | %-24s | %-24s |"
		row := "| %-20s | %-20s | %-18d | %-18d | %-24d | %-24d |"
		t.Logf(header, "Intf", "Queue", "ATE Tx frames", "DUT Tx frames", "ATE Dropped frames", "DUT Dropped frames")

		for queue := range uniqueQueues {
			dutQueueTransmitPktsTotal := gnmi.Get(t, dut, gnmi.OC().Qos().Interface(dp3.Name()).Output().Queue(queue).TransmitPkts().State())
			dutQueueDroppedPktsTotal := gnmi.Get(t, dut, gnmi.OC().Qos().Interface(dp3.Name()).Output().Queue(queue).DroppedPkts().State())

			dutQueueTransmitPkts := dutQueueTransmitPktsTotal - dutQosPktsBeforeTraffic[queue]
			dutQueueDroppedPkts := dutQueueDroppedPktsTotal - dutQosDroppedPktsBeforeTraffic[queue]

			ateQueueTransmitPkts := ateOutPkts[queue]
			var ateQueueDroppedPkts uint64 = 0
			if ateOutPkts[queue] >= ateInPkts[queue] {
				ateQueueDroppedPkts = ateOutPkts[queue] - ateInPkts[queue]
			} else {
				t.Fatalf("ATE reports more received pkts than sent pkts on interface %s and egress queue %s", dp3.Name(), queue)
			}
			t.Logf(row, dp3.Name(), queue, ateQueueTransmitPkts, dutQueueTransmitPkts, ateQueueDroppedPkts, dutQueueDroppedPkts)
		}

	})

}

func verifyEgressStrictPrioritySchedulerBurstTrafficIPv6(t *testing.T, dut *ondatra.DUTDevice) {

	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")
	dp3 := dut.Port(t, "port3")

	// Configure DUT interfaces and QoS.
	ConfigureDUTIntfIPv6(t, dut)
	ConfigureDUTQoSIPv6(t, dut)

	// Configure ATE interfaces.
	ate := ondatra.ATE(t, "ate")
	ap1 := ate.Port(t, "port1")
	ap2 := ate.Port(t, "port2")
	ap3 := ate.Port(t, "port3")

	top := gosnappi.NewConfig()

	ateTxP1.AddToOTG(top, ap1, &dutIngressPort1AteP1)
	ateTxP2.AddToOTG(top, ap2, &dutIngressPort2AteP2)
	ateRxP3.AddToOTG(top, ap3, &dutEgressPort3AteP3)
	ate.OTG().PushConfig(t, top)

	createTrafficFlowsIPv6(t, ate, top, dut)
	t.Run("\n*** Running test for IPv6 ***\n", func(t *testing.T) {

		ate.OTG().StartProtocols(t)
		otgutils.WaitForARP(t, ate.OTG(), top, "IPv6")

		queues := netutil.CommonTrafficQueues(t, dut)

		type trafficData struct {
			trafficRate           float64
			expectedThroughputPct float32
			frameSize             uint32
			dscp                  []uint32
			queue                 string
			inputIntf             attrs.Attributes
			burstPackets          uint32
			burstMinGap           uint32
			burstGap              uint32
		}

		trafficFlows := map[string]*trafficData{
			"ateTxP1-regular-nc1": {
				frameSize:             512,
				trafficRate:           1,
				expectedThroughputPct: 100.0,
				dscp:                  []uint32{48, 49, 50, 51, 52, 53, 54, 55},
				queue:                 queues.NC1,
				inputIntf:             ateTxP1,
			},
			"ateTxP2-burst-nc1": {
				frameSize:             256,
				trafficRate:           10,
				dscp:                  []uint32{56, 57, 58, 59, 60, 61, 62, 63},
				expectedThroughputPct: 100.0,
				queue:                 queues.NC1,
				inputIntf:             ateTxP2,
				burstPackets:          50000,
				burstMinGap:           12,
				burstGap:              100,
			},
			"ateTxP1-regular-af4": {
				frameSize:             512,
				trafficRate:           30,
				expectedThroughputPct: 100.0,
				dscp:                  []uint32{32, 33, 34, 35, 36, 37, 38, 39},
				queue:                 queues.AF4,
				inputIntf:             ateTxP1,
			},
			"ateTxP2-burst-af4": {
				frameSize:             256,
				trafficRate:           20,
				dscp:                  []uint32{40, 41, 42, 43, 44, 45, 46, 47},
				expectedThroughputPct: 100.0,
				queue:                 queues.AF4,
				inputIntf:             ateTxP2,
				burstPackets:          50000,
				burstMinGap:           12,
				burstGap:              100,
			},
			"ateTxP1-regular-af3": {
				frameSize:             512,
				trafficRate:           12,
				expectedThroughputPct: 100.0,
				dscp:                  []uint32{24, 25, 26, 27},
				queue:                 queues.AF3,
				inputIntf:             ateTxP1,
			},
			"ateTxP2-burst-af3": {
				frameSize:             256,
				trafficRate:           10,
				dscp:                  []uint32{28, 29, 30, 31},
				expectedThroughputPct: 100.0,
				queue:                 queues.AF3,
				inputIntf:             ateTxP2,
				burstPackets:          50000,
				burstMinGap:           12,
				burstGap:              100,
			},
			"ateTxP1-regular-af2": {
				frameSize:             512,
				trafficRate:           15,
				expectedThroughputPct: 50.0,
				dscp:                  []uint32{16, 17, 18, 19},
				queue:                 queues.AF2,
				inputIntf:             ateTxP1,
			},
			"ateTxP2-burst-af2": {
				frameSize:             256,
				trafficRate:           17,
				dscp:                  []uint32{20, 21, 22, 23},
				expectedThroughputPct: 50.0,
				queue:                 queues.AF2,
				inputIntf:             ateTxP2,
				burstPackets:          50000,
				burstMinGap:           12,
				burstGap:              100,
			},
			"ateTxP1-regular-af1": {
				frameSize:             512,
				trafficRate:           12,
				expectedThroughputPct: 0.0,
				dscp:                  []uint32{8, 9, 10, 11},
				queue:                 queues.AF1,
				inputIntf:             ateTxP1,
			},
			"ateTxP2-burst-af1": {
				frameSize:             256,
				trafficRate:           13,
				dscp:                  []uint32{12, 13, 14, 15},
				expectedThroughputPct: 0.0,
				queue:                 queues.AF1,
				inputIntf:             ateTxP2,
				burstPackets:          50000,
				burstMinGap:           12,
				burstGap:              100,
			},
			"ateTxP1-regular-be1": {
				frameSize:             512,
				trafficRate:           12,
				expectedThroughputPct: 0.0,
				dscp:                  []uint32{0, 1, 2, 3},
				queue:                 queues.BE1,
				inputIntf:             ateTxP1,
			},
			"ateTxP2-burst-be1": {
				frameSize:             512,
				trafficRate:           20,
				expectedThroughputPct: 0.0,
				dscp:                  []uint32{4, 5, 6, 7},
				queue:                 queues.BE1,
				inputIntf:             ateTxP2,
				burstPackets:          50000,
				burstMinGap:           12,
				burstGap:              100,
			},
		}

		if deviations.InterfaceOutputQueueNonStandardName(dut) {

			// Configuring the non-standard queue names.
			for flowName, data := range trafficFlows {
				if strings.Contains(strings.ToUpper(flowName), "BE1") {
					data.queue = dp3.Name() + "-" + strconv.Itoa(0)
				}
				if strings.Contains(strings.ToUpper(flowName), "AF1") {
					data.queue = dp3.Name() + "-" + strconv.Itoa(1)
				}
				if strings.Contains(strings.ToUpper(flowName), "AF2") {
					data.queue = dp3.Name() + "-" + strconv.Itoa(2)
				}
				if strings.Contains(strings.ToUpper(flowName), "AF3") {
					data.queue = dp3.Name() + "-" + strconv.Itoa(3)
				}
				if strings.Contains(strings.ToUpper(flowName), "AF4") {
					data.queue = dp3.Name() + "-" + strconv.Itoa(4)
				}
				if strings.Contains(strings.ToUpper(flowName), "NC1") {
					data.queue = dp3.Name() + "-" + strconv.Itoa(5)
				}
			}
		}

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

		uniqueQueues := make(map[string]bool)
		for _, data := range trafficFlows {
			uniqueQueues[data.queue] = true
		}
		// Get QoS egress packet counters before the traffic.
		const timeout = 10 * time.Second
		isPresent := func(val *ygnmi.Value[uint64]) bool { return val.IsPresent() }

		for queue := range uniqueQueues {
			count, ok := gnmi.Watch(t, dut, gnmi.OC().Qos().Interface(dp3.Name()).Output().Queue(queue).TransmitPkts().State(), timeout, isPresent).Await(t)
			if !ok {
				t.Errorf("TransmitPkts count for queue %s on interface %q not available within %v", queue, dp3.Name(), timeout)
			}
			dutQosPktsBeforeTraffic[queue], _ = count.Val()

			count, ok = gnmi.Watch(t, dut, gnmi.OC().Qos().Interface(dp3.Name()).Output().Queue(queue).DroppedPkts().State(), timeout, isPresent).Await(t)
			if !ok {
				t.Errorf("DroppedPkts count for queue %s on interface %q not available within %v", dp3.Name(), queue, timeout)
			}
			dutQosDroppedPktsBeforeTraffic[queue], _ = count.Val()
		}

		t.Logf("Before TX values map: %v\n", dutQosPktsBeforeTraffic)
		t.Logf("Before DROP values map: %v\n", dutQosDroppedPktsBeforeTraffic)
		t.Logf("Running regular traffic on DUT interfaces: %s => %s \n", dp1.Name(), dp3.Name())
		t.Logf("Running bursty traffic on DUT interfaces: %s => %s \n", dp2.Name(), dp3.Name())
		t.Logf("Sending traffic flows:\n")
		ate.OTG().StartTraffic(t)
		time.Sleep(30 * time.Second)
		ate.OTG().StopTraffic(t)

		time.Sleep(5 * time.Second)

		t.Logf("Printing aggregated flow metrics from OTG: \n")
		otgutils.LogFlowMetrics(t, ate.OTG(), top)

		for trafficID, data := range trafficFlows {

			t.Logf("Retrieving statistics for %s\n", trafficID)
			ateTxPkts := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(trafficID).Counters().OutPkts().State())
			ateRxPkts := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(trafficID).Counters().InPkts().State())
			if ateTxPkts == 0 {
				t.Fatalf("Flow %s did not send any packets, please check tester configuration.\n", trafficID)
			}

			lossPct := (float32)((float64(ateTxPkts-ateRxPkts) * 100.0) / float64(ateTxPkts))

			t.Logf("Flow: %s\t\t|EgressQueue:%s\t\t|Loss%%: %.2f%%\t\t|ExpectedRx%%:%.2f%%\t\t|Tolerance%%: %.2f%%", trafficID, data.queue, lossPct, 100-lossPct, tolerance)

			if got, want := 100.0-lossPct, data.expectedThroughputPct; got < want-tolerance || got > want+tolerance {
				t.Errorf("Expected throughput for queue %q should be within [%.2f%%, %.2f%%]: got %.2f%%\n", data.queue, want-tolerance, want+tolerance, got)
			}

			ateOutPkts[data.queue] += ateTxPkts
			ateInPkts[data.queue] += ateRxPkts

		}

		header := "| %-20s | %-20s | %-18s | %-18s | %-24s | %-24s |"
		row := "| %-20s | %-20s | %-18d | %-18d | %-24d | %-24d |"
		t.Logf(header, "Intf", "Queue", "ATE Tx frames", "DUT Tx frames", "ATE Dropped frames", "DUT Dropped frames")

		for queue := range uniqueQueues {
			dutQueueTransmitPktsTotal := gnmi.Get(t, dut, gnmi.OC().Qos().Interface(dp3.Name()).Output().Queue(queue).TransmitPkts().State())
			dutQueueDroppedPktsTotal := gnmi.Get(t, dut, gnmi.OC().Qos().Interface(dp3.Name()).Output().Queue(queue).DroppedPkts().State())

			dutQueueTransmitPkts := dutQueueTransmitPktsTotal - dutQosPktsBeforeTraffic[queue]
			dutQueueDroppedPkts := dutQueueDroppedPktsTotal - dutQosDroppedPktsBeforeTraffic[queue]

			ateQueueTransmitPkts := ateOutPkts[queue]
			var ateQueueDroppedPkts uint64 = 0
			if ateOutPkts[queue] >= ateInPkts[queue] {
				ateQueueDroppedPkts = ateOutPkts[queue] - ateInPkts[queue]
			} else {
				t.Fatalf("ATE reports more received pkts than sent pkts on interface %s and egress queue %s", dp3.Name(), queue)
			}
			t.Logf(row, dp3.Name(), queue, ateQueueTransmitPkts, dutQueueTransmitPkts, ateQueueDroppedPkts, dutQueueDroppedPkts)
		}

	})

}

func verifyEgressStrictPrioritySchedulerBurstTrafficMPLS(t *testing.T, dut *ondatra.DUTDevice) {

	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")
	dp3 := dut.Port(t, "port3")

	// Configure DUT interfaces and QoS.
	ConfigureDUTIntfIPv4(t, dut)
	ConfigureDUTQoSMPLS(t, dut)

	// Configure ATE interfaces.
	ate := ondatra.ATE(t, "ate")
	ap1 := ate.Port(t, "port1")
	ap2 := ate.Port(t, "port2")
	ap3 := ate.Port(t, "port3")

	top := gosnappi.NewConfig()

	ateTxP1.AddToOTG(top, ap1, &dutIngressPort1AteP1)
	ateTxP2.AddToOTG(top, ap2, &dutIngressPort2AteP2)
	ateRxP3.AddToOTG(top, ap3, &dutEgressPort3AteP3)
	ate.OTG().PushConfig(t, top)

	t.Run("\n*** Running test for MPLS ***\n", func(t *testing.T) {

		ate.OTG().StartProtocols(t)

		otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")

		createTrafficFlowsMPLS(t, ate, top, dut)

		queues := netutil.CommonTrafficQueues(t, dut)

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

		trafficFlows := map[string]*trafficData{
			"ateTxP1-regular-nc1": {
				frameSize:             512,
				trafficRate:           1,
				expectedThroughputPct: 100.0,
				dscp:                  6,
				queue:                 queues.NC1,
				inputIntf:             ateTxP1,
			},
			"ateTxP2-burst-nc1": {
				frameSize:             256,
				trafficRate:           10,
				dscp:                  7,
				expectedThroughputPct: 100.0,
				queue:                 queues.NC1,
				inputIntf:             ateTxP2,
				burstPackets:          50000,
				burstMinGap:           12,
				burstGap:              100,
			},
			"ateTxP1-regular-af4": {
				frameSize:             512,
				trafficRate:           30,
				expectedThroughputPct: 100.0,
				dscp:                  4,
				queue:                 queues.AF4,
				inputIntf:             ateTxP1,
			},
			"ateTxP2-burst-af4": {
				frameSize:             256,
				trafficRate:           20,
				dscp:                  5,
				expectedThroughputPct: 100.0,
				queue:                 queues.AF4,
				inputIntf:             ateTxP2,
				burstPackets:          50000,
				burstMinGap:           12,
				burstGap:              100,
			},
			"ateTxP1-regular-af3": {
				frameSize:             512,
				trafficRate:           12,
				expectedThroughputPct: 100.0,
				dscp:                  3,
				queue:                 queues.AF3,
				inputIntf:             ateTxP1,
			},
			"ateTxP2-burst-af3": {
				frameSize:             256,
				trafficRate:           10,
				dscp:                  3,
				expectedThroughputPct: 100.0,
				queue:                 queues.AF3,
				inputIntf:             ateTxP2,
				burstPackets:          50000,
				burstMinGap:           12,
				burstGap:              100,
			},
			"ateTxP1-regular-af2": {
				frameSize:             512,
				trafficRate:           15,
				expectedThroughputPct: 50.0,
				dscp:                  2,
				queue:                 queues.AF2,
				inputIntf:             ateTxP1,
			},
			"ateTxP2-burst-af2": {
				frameSize:             256,
				trafficRate:           17,
				dscp:                  2,
				expectedThroughputPct: 50.0,
				queue:                 queues.AF2,
				inputIntf:             ateTxP2,
				burstPackets:          50000,
				burstMinGap:           12,
				burstGap:              100,
			},
			"ateTxP1-regular-af1": {
				frameSize:             512,
				trafficRate:           12,
				expectedThroughputPct: 0.0,
				dscp:                  1,
				queue:                 queues.AF1,
				inputIntf:             ateTxP1,
			},
			"ateTxP2-burst-af1": {
				frameSize:             256,
				trafficRate:           13,
				dscp:                  1,
				expectedThroughputPct: 0.0,
				queue:                 queues.AF1,
				inputIntf:             ateTxP2,
				burstPackets:          50000,
				burstMinGap:           12,
				burstGap:              100,
			},
			"ateTxP1-regular-be1": {
				frameSize:             512,
				trafficRate:           12,
				expectedThroughputPct: 0.0,
				dscp:                  0,
				queue:                 queues.BE1,
				inputIntf:             ateTxP1,
			},
			"ateTxP2-burst-be1": {
				frameSize:             256,
				trafficRate:           20,
				expectedThroughputPct: 0.0,
				dscp:                  0,
				queue:                 queues.BE1,
				inputIntf:             ateTxP2,
				burstPackets:          50000,
				burstMinGap:           12,
				burstGap:              100,
			},
		}

		if deviations.InterfaceOutputQueueNonStandardName(dut) {

			// Configuring the non-standard queue names.
			for flowName, data := range trafficFlows {
				if strings.Contains(strings.ToUpper(flowName), "BE1") {
					data.queue = dp3.Name() + "-" + strconv.Itoa(0)
				}
				if strings.Contains(strings.ToUpper(flowName), "AF1") {
					data.queue = dp3.Name() + "-" + strconv.Itoa(1)
				}
				if strings.Contains(strings.ToUpper(flowName), "AF2") {
					data.queue = dp3.Name() + "-" + strconv.Itoa(2)
				}
				if strings.Contains(strings.ToUpper(flowName), "AF3") {
					data.queue = dp3.Name() + "-" + strconv.Itoa(3)
				}
				if strings.Contains(strings.ToUpper(flowName), "AF4") {
					data.queue = dp3.Name() + "-" + strconv.Itoa(4)
				}
				if strings.Contains(strings.ToUpper(flowName), "NC1") {
					data.queue = dp3.Name() + "-" + strconv.Itoa(5)
				}
			}

		}
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

		uniqueQueues := make(map[string]bool)
		for _, data := range trafficFlows {
			uniqueQueues[data.queue] = true
		}

		// Get QoS egress packet counters before the traffic.
		const timeout = 10 * time.Second
		isPresent := func(val *ygnmi.Value[uint64]) bool { return val.IsPresent() }

		for queue := range uniqueQueues {
			count, ok := gnmi.Watch(t, dut, gnmi.OC().Qos().Interface(dp3.Name()).Output().Queue(queue).TransmitPkts().State(), timeout, isPresent).Await(t)
			if !ok {
				t.Errorf("TransmitPkts count for queue %s on interface %q not available within %v", queue, dp3.Name(), timeout)
			}
			dutQosPktsBeforeTraffic[queue], _ = count.Val()

			count, ok = gnmi.Watch(t, dut, gnmi.OC().Qos().Interface(dp3.Name()).Output().Queue(queue).DroppedPkts().State(), timeout, isPresent).Await(t)
			if !ok {
				t.Errorf("DroppedPkts count for queue %s on interface %q not available within %v", dp3.Name(), queue, timeout)
			}
			dutQosDroppedPktsBeforeTraffic[queue], _ = count.Val()
		}

		t.Logf("Running regular traffic on DUT interfaces: %s => %s \n", dp1.Name(), dp3.Name())
		t.Logf("Running bursty traffic on DUT interfaces: %s => %s \n", dp2.Name(), dp3.Name())
		t.Logf("Sending traffic flows:\n")
		ate.OTG().StartTraffic(t)

		time.Sleep(15 * time.Second)

		ate.OTG().StopTraffic(t)

		time.Sleep(5 * time.Second)

		t.Logf("Printing aggregated flow metrics from OTG: \n")
		otgutils.LogFlowMetrics(t, ate.OTG(), top)

		for trafficID, data := range trafficFlows {

			t.Logf("Retrieving statistics for %s\n", trafficID)
			ateTxPkts := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(trafficID).Counters().OutPkts().State())
			ateRxPkts := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(trafficID).Counters().InPkts().State())
			if ateTxPkts == 0 {
				t.Fatalf("Flow %s did not send any packets, please check tester configuration.\n", trafficID)
			}

			lossPct := (float32)((float64(ateTxPkts-ateRxPkts) * 100.0) / float64(ateTxPkts))

			t.Logf("Flow: %s\t\t|EgressQueue:%s\t\t|Loss%%: %.2f%%\t\t|ExpectedRx%%:%.2f%%\t\t|Tolerance%%: %.2f%%", trafficID, data.queue, lossPct, 100-lossPct, tolerance)

			if got, want := 100.0-lossPct, data.expectedThroughputPct; got < want-tolerance || got > want+tolerance {
				t.Errorf("Expected throughput for queue %q should be within [%.2f%%, %.2f%%]: got %.2f%%\n", data.queue, want-tolerance, want+tolerance, got)
			}

			ateOutPkts[data.queue] += ateTxPkts
			ateInPkts[data.queue] += ateRxPkts

		}

		header := "| %-20s | %-20s | %-18s | %-18s | %-24s | %-24s |"
		row := "| %-20s | %-20s | %-18d | %-18d | %-24d | %-24d |"
		t.Logf(header, "Intf", "Queue", "ATE Tx frames", "DUT Tx frames", "ATE Dropped frames", "DUT Dropped frames")

		for queue := range uniqueQueues {
			dutQueueTransmitPktsTotal := gnmi.Get(t, dut, gnmi.OC().Qos().Interface(dp3.Name()).Output().Queue(queue).TransmitPkts().State())
			dutQueueDroppedPktsTotal := gnmi.Get(t, dut, gnmi.OC().Qos().Interface(dp3.Name()).Output().Queue(queue).DroppedPkts().State())

			dutQueueTransmitPkts := dutQueueTransmitPktsTotal - dutQosPktsBeforeTraffic[queue]
			dutQueueDroppedPkts := dutQueueDroppedPktsTotal - dutQosDroppedPktsBeforeTraffic[queue]

			ateQueueTransmitPkts := ateOutPkts[queue]
			var ateQueueDroppedPkts uint64 = 0
			if ateOutPkts[queue] >= ateInPkts[queue] {
				ateQueueDroppedPkts = ateOutPkts[queue] - ateInPkts[queue]
			} else {
				t.Fatalf("ATE reports more received pkts than sent pkts on interface %s and egress queue %s", dp3.Name(), queue)
			}
			t.Logf(row, dp3.Name(), queue, ateQueueTransmitPkts, dutQueueTransmitPkts, ateQueueDroppedPkts, dutQueueDroppedPkts)
		}

	})

}

func createTrafficFlowsIPv4(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config, dut *ondatra.DUTDevice) {
	t.Helper()
	// configuration of regular and burst flows on the ATE
	/*
		Non-burst flows on ateTxP1:

		Forwarding Group	Traffic linerate (%)	Frame size	Expected Loss %
		be1					12						512			100
		af1					12						512			100
		af2					15						512			50
		af3					12						512			0
		af4					30						512			0
		nc1					1						512			0

		Burst flows on ateTxP2:

		Fwd Grp    | Traffic linerate (%)   | FS         | Burst         | IPG           | IBG             | Expected loss (%)
		be1        | 20                     | 256        | 50000         | 12            | 100             | 100
		af1        | 13                     | 256        | 50000         | 12            | 100             | 100
		af2        | 17                     | 256        | 50000         | 12            | 100             | 50
		af3        | 10                     | 256        | 50000         | 12            | 100             | 0
		af4        | 20                     | 256        | 50000         | 12            | 100             | 0
		nc1        | 10                     | 256        | 50000         | 12            | 100             | 0
	*/

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

	queues := netutil.CommonTrafficQueues(t, dut)
	trafficFlows := map[string]*trafficData{
		"ateTxP1-regular-nc1": {
			frameSize:             512,
			trafficRate:           1,
			expectedThroughputPct: 100.0,
			dscp:                  6,
			queue:                 queues.NC1,
			inputIntf:             ateTxP1,
		},
		"ateTxP2-burst-nc1": {
			frameSize:             256,
			trafficRate:           10,
			dscp:                  7,
			expectedThroughputPct: 100.0,
			queue:                 queues.NC1,
			inputIntf:             ateTxP2,
			burstPackets:          50000,
			burstMinGap:           12,
			burstGap:              100,
		},
		"ateTxP1-regular-af4": {
			frameSize:             512,
			trafficRate:           30,
			expectedThroughputPct: 100.0,
			dscp:                  4,
			queue:                 queues.AF4,
			inputIntf:             ateTxP1,
		},
		"ateTxP2-burst-af4": {
			frameSize:             256,
			trafficRate:           20,
			dscp:                  5,
			expectedThroughputPct: 100.0,
			queue:                 queues.AF4,
			inputIntf:             ateTxP2,
			burstPackets:          50000,
			burstMinGap:           12,
			burstGap:              100,
		},
		"ateTxP1-regular-af3": {
			frameSize:             512,
			trafficRate:           12,
			expectedThroughputPct: 100.0,
			dscp:                  3,
			queue:                 queues.AF3,
			inputIntf:             ateTxP1,
		},
		"ateTxP2-burst-af3": {
			frameSize:             256,
			trafficRate:           10,
			dscp:                  3,
			expectedThroughputPct: 100.0,
			queue:                 queues.AF3,
			inputIntf:             ateTxP2,
			burstPackets:          50000,
			burstMinGap:           12,
			burstGap:              100,
		},
		"ateTxP1-regular-af2": {
			frameSize:             512,
			trafficRate:           15,
			expectedThroughputPct: 50.0,
			dscp:                  2,
			queue:                 queues.AF2,
			inputIntf:             ateTxP1,
		},
		"ateTxP2-burst-af2": {
			frameSize:             256,
			trafficRate:           17,
			dscp:                  2,
			expectedThroughputPct: 50.0,
			queue:                 queues.AF2,
			inputIntf:             ateTxP2,
			burstPackets:          50000,
			burstMinGap:           12,
			burstGap:              100,
		},
		"ateTxP1-regular-af1": {
			frameSize:             512,
			trafficRate:           12,
			expectedThroughputPct: 0.0,
			dscp:                  1,
			queue:                 queues.AF1,
			inputIntf:             ateTxP1,
		},
		"ateTxP2-burst-af1": {
			frameSize:             256,
			trafficRate:           13,
			dscp:                  1,
			expectedThroughputPct: 0.0,
			queue:                 queues.AF1,
			inputIntf:             ateTxP2,
			burstPackets:          50000,
			burstMinGap:           12,
			burstGap:              100,
		},
		"ateTxP1-regular-be1": {
			frameSize:             512,
			trafficRate:           12,
			expectedThroughputPct: 0.0,
			dscp:                  0,
			queue:                 queues.BE1,
			inputIntf:             ateTxP1,
		},
		"ateTxP2-burst-be1": {
			frameSize:             256,
			trafficRate:           20,
			expectedThroughputPct: 0.0,
			dscp:                  0,
			queue:                 queues.BE1,
			inputIntf:             ateTxP2,
			burstPackets:          50000,
			burstMinGap:           12,
			burstGap:              100,
		},
	}
	top.Flows().Clear()

	for trafficID, data := range trafficFlows {
		t.Logf("Configuring flow %s\n", trafficID)
		flow := top.Flows().Add().SetName(trafficID)
		flow.Metrics().SetEnable(true)
		flow.TxRx().Device().SetTxNames([]string{data.inputIntf.Name + ".IPv4"}).SetRxNames([]string{ateRxP3.Name + ".IPv4"})
		ethHeader := flow.Packet().Add().Ethernet()
		ethHeader.Src().SetValue(data.inputIntf.MAC)

		ipHeader := flow.Packet().Add().Ipv4()
		ipHeader.Src().SetValue(data.inputIntf.IPv4)
		ipHeader.Dst().SetValue(ateRxP3.IPv4)
		ipHeader.Priority().Dscp().Phb().SetValue(uint32(data.dscp))

		flow.Size().SetFixed(uint32(data.frameSize))
		flow.Rate().SetPercentage(float32(data.trafficRate))
		if data.burstMinGap > 0 {
			flow.Duration().Burst().SetPackets(uint32(data.burstPackets)).SetGap(uint32(data.burstMinGap))
		}
		if data.burstGap > 0 {
			flow.Duration().Burst().InterBurstGap().SetBytes(float64(data.burstGap))
		}

	}
	ate.OTG().PushConfig(t, top)
}

func createTrafficFlowsIPv6(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config, dut *ondatra.DUTDevice) {
	t.Helper()
	// configuration of regular and burst flows on the ATE
	/*
		Non-burst flows on ateTxP1:

		Forwarding Group	Traffic linerate (%)	Frame size	Expected Loss %
		be1					12						512			100
		af1					12						512			100
		af2					15						512			50
		af3					12						512			0
		af4					30						512			0
		nc1					1						512			0

		Burst flows on ateTxP2:

		Fwd Grp    | Traffic linerate (%)   | FS         | Burst         | IPG           | IBG             | Expected loss (%)
		be1        | 20                     | 256        | 50000         | 12            | 100             | 100
		af1        | 13                     | 256        | 50000         | 12            | 100             | 100
		af2        | 17                     | 256        | 50000         | 12            | 100             | 50
		af3        | 10                     | 256        | 50000         | 12            | 100             | 0
		af4        | 20                     | 256        | 50000         | 12            | 100             | 0
		nc1        | 10                     | 256        | 50000         | 12            | 100             | 0
	*/

	type trafficData struct {
		trafficRate           float64
		expectedThroughputPct float32
		frameSize             uint32
		dscp                  []uint32
		queue                 string
		inputIntf             attrs.Attributes
		burstPackets          uint32
		burstMinGap           uint32
		burstGap              uint32
	}

	queues := netutil.CommonTrafficQueues(t, dut)
	trafficFlows := map[string]*trafficData{
		"ateTxP1-regular-nc1": {
			frameSize:             512,
			trafficRate:           1,
			expectedThroughputPct: 100.0,
			dscp:                  []uint32{48, 49, 50, 51, 52, 53, 54, 55},
			queue:                 queues.NC1,
			inputIntf:             ateTxP1,
		},
		"ateTxP2-burst-nc1": {
			frameSize:             256,
			trafficRate:           10,
			dscp:                  []uint32{56, 57, 58, 59, 60, 61, 62, 63},
			expectedThroughputPct: 100.0,
			queue:                 queues.NC1,
			inputIntf:             ateTxP2,
			burstPackets:          50000,
			burstMinGap:           12,
			burstGap:              100,
		},
		"ateTxP1-regular-af4": {
			frameSize:             512,
			trafficRate:           30,
			expectedThroughputPct: 100.0,
			dscp:                  []uint32{32, 33, 34, 35, 36, 37, 38, 39},
			queue:                 queues.AF4,
			inputIntf:             ateTxP1,
		},
		"ateTxP2-burst-af4": {
			frameSize:             256,
			trafficRate:           20,
			dscp:                  []uint32{40, 41, 42, 43, 44, 45, 46, 47},
			expectedThroughputPct: 100.0,
			queue:                 queues.AF4,
			inputIntf:             ateTxP2,
			burstPackets:          50000,
			burstMinGap:           12,
			burstGap:              100,
		},
		"ateTxP1-regular-af3": {
			frameSize:             512,
			trafficRate:           12,
			expectedThroughputPct: 100.0,
			dscp:                  []uint32{24, 25, 26, 27},
			queue:                 queues.AF3,
			inputIntf:             ateTxP1,
		},
		"ateTxP2-burst-af3": {
			frameSize:             256,
			trafficRate:           10,
			dscp:                  []uint32{28, 29, 30, 31},
			expectedThroughputPct: 100.0,
			queue:                 queues.AF3,
			inputIntf:             ateTxP2,
			burstPackets:          50000,
			burstMinGap:           12,
			burstGap:              100,
		},
		"ateTxP1-regular-af2": {
			frameSize:             512,
			trafficRate:           15,
			expectedThroughputPct: 50.0,
			dscp:                  []uint32{16, 17, 18, 19},
			queue:                 queues.AF2,
			inputIntf:             ateTxP1,
		},
		"ateTxP2-burst-af2": {
			frameSize:             256,
			trafficRate:           17,
			dscp:                  []uint32{20, 21, 22, 23},
			expectedThroughputPct: 50.0,
			queue:                 queues.AF2,
			inputIntf:             ateTxP2,
			burstPackets:          50000,
			burstMinGap:           12,
			burstGap:              100,
		},
		"ateTxP1-regular-af1": {
			frameSize:             512,
			trafficRate:           12,
			expectedThroughputPct: 0.0,
			dscp:                  []uint32{8, 9, 10, 11},
			queue:                 queues.AF1,
			inputIntf:             ateTxP1,
		},
		"ateTxP2-burst-af1": {
			frameSize:             256,
			trafficRate:           13,
			dscp:                  []uint32{12, 13, 14, 15},
			expectedThroughputPct: 0.0,
			queue:                 queues.AF1,
			inputIntf:             ateTxP2,
			burstPackets:          50000,
			burstMinGap:           12,
			burstGap:              100,
		},
		"ateTxP1-regular-be1": {
			frameSize:             512,
			trafficRate:           12,
			expectedThroughputPct: 0.0,
			dscp:                  []uint32{0, 1, 2, 3},
			queue:                 queues.BE1,
			inputIntf:             ateTxP1,
		},
		"ateTxP2-burst-be1": {
			frameSize:             512,
			trafficRate:           20,
			expectedThroughputPct: 0.0,
			dscp:                  []uint32{4, 5, 6, 7},
			queue:                 queues.BE1,
			inputIntf:             ateTxP2,
			burstPackets:          50000,
			burstMinGap:           12,
			burstGap:              100,
		},
	}
	top.Flows().Clear()

	for trafficID, data := range trafficFlows {
		t.Logf("Configuring flow %s\n", trafficID)
		flow := top.Flows().Add().SetName(trafficID)
		flow.Metrics().SetEnable(true)
		flow.TxRx().Device().SetTxNames([]string{data.inputIntf.Name + ".IPv6"}).SetRxNames([]string{ateRxP3.Name + ".IPv6"})
		ethHeader := flow.Packet().Add().Ethernet()
		ethHeader.Src().SetValue(data.inputIntf.MAC)

		ipHeader := flow.Packet().Add().Ipv6()
		ipHeader.Src().SetValue(data.inputIntf.IPv6)
		ipHeader.Dst().SetValue(ateRxP3.IPv6)
		ipHeader.TrafficClass().SetValues([]uint32(data.dscp))

		flow.Size().SetFixed(uint32(data.frameSize))
		flow.Rate().SetPercentage(float32(data.trafficRate))
		if data.burstMinGap > 0 {
			flow.Duration().Burst().SetPackets(uint32(data.burstPackets)).SetGap(uint32(data.burstMinGap))
		}
		if data.burstGap > 0 {
			flow.Duration().Burst().InterBurstGap().SetBytes(float64(data.burstGap))
		}

	}
	ate.OTG().PushConfig(t, top)
}

func createTrafficFlowsMPLS(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config, dut *ondatra.DUTDevice) {
	t.Helper()
	// configuration of regular and burst flows on the ATE
	/*
		Non-burst flows on ateTxP1:

		Forwarding Group	Traffic linerate (%)	Frame size	Expected Loss %
		be1					12						512			100
		af1					12						512			100
		af2					15						512			50
		af3					12						512			0
		af4					30						512			0
		nc1					1						512			0

		Burst flows on ateTxP2:

		Fwd Grp    | Traffic linerate (%)   | FS         | Burst         | IPG           | IBG             | Expected loss (%)
		be1        | 20                     | 256        | 50000         | 12            | 100             | 100
		af1        | 13                     | 256        | 50000         | 12            | 100             | 100
		af2        | 17                     | 256        | 50000         | 12            | 100             | 50
		af3        | 10                     | 256        | 50000         | 12            | 100             | 0
		af4        | 20                     | 256        | 50000         | 12            | 100             | 0
		nc1        | 10                     | 256        | 50000         | 12            | 100             | 0
	*/

	type trafficData struct {
		trafficRate           float64
		expectedThroughputPct float32
		frameSize             uint32
		exp                   uint8
		queue                 string
		inputIntf             attrs.Attributes
		burstPackets          uint32
		burstMinGap           uint32
		burstGap              uint32
	}

	queues := netutil.CommonTrafficQueues(t, dut)
	trafficFlows := map[string]*trafficData{
		"ateTxP1-regular-nc1": {
			frameSize:             512,
			trafficRate:           1,
			expectedThroughputPct: 100.0,
			exp:                   6,
			queue:                 queues.NC1,
			inputIntf:             ateTxP1,
		},
		"ateTxP2-burst-nc1": {
			frameSize:             256,
			trafficRate:           10,
			exp:                   7,
			expectedThroughputPct: 100.0,
			queue:                 queues.NC1,
			inputIntf:             ateTxP2,
			burstPackets:          50000,
			burstMinGap:           12,
			burstGap:              100,
		},
		"ateTxP1-regular-af4": {
			frameSize:             512,
			trafficRate:           30,
			expectedThroughputPct: 100.0,
			exp:                   4,
			queue:                 queues.AF4,
			inputIntf:             ateTxP1,
		},
		"ateTxP2-burst-af4": {
			frameSize:             256,
			trafficRate:           20,
			exp:                   5,
			expectedThroughputPct: 100.0,
			queue:                 queues.AF4,
			inputIntf:             ateTxP2,
			burstPackets:          50000,
			burstMinGap:           12,
			burstGap:              100,
		},
		"ateTxP1-regular-af3": {
			frameSize:             512,
			trafficRate:           12,
			expectedThroughputPct: 100.0,
			exp:                   3,
			queue:                 queues.AF3,
			inputIntf:             ateTxP1,
		},
		"ateTxP2-burst-af3": {
			frameSize:             256,
			trafficRate:           10,
			exp:                   3,
			expectedThroughputPct: 100.0,
			queue:                 queues.AF3,
			inputIntf:             ateTxP2,
			burstPackets:          50000,
			burstMinGap:           12,
			burstGap:              100,
		},
		"ateTxP1-regular-af2": {
			frameSize:             512,
			trafficRate:           15,
			expectedThroughputPct: 50.0,
			exp:                   2,
			queue:                 queues.AF2,
			inputIntf:             ateTxP1,
		},
		"ateTxP2-burst-af2": {
			frameSize:             256,
			trafficRate:           17,
			exp:                   2,
			expectedThroughputPct: 50.0,
			queue:                 queues.AF2,
			inputIntf:             ateTxP2,
			burstPackets:          50000,
			burstMinGap:           12,
			burstGap:              100,
		},
		"ateTxP1-regular-af1": {
			frameSize:             512,
			trafficRate:           12,
			expectedThroughputPct: 0.0,
			exp:                   1,
			queue:                 queues.AF1,
			inputIntf:             ateTxP1,
		},
		"ateTxP2-burst-af1": {
			frameSize:             256,
			trafficRate:           13,
			exp:                   1,
			expectedThroughputPct: 0.0,
			queue:                 queues.AF1,
			inputIntf:             ateTxP2,
			burstPackets:          50000,
			burstMinGap:           12,
			burstGap:              100,
		},
		"ateTxP1-regular-be1": {
			frameSize:             512,
			trafficRate:           12,
			expectedThroughputPct: 0.0,
			exp:                   0,
			queue:                 queues.BE1,
			inputIntf:             ateTxP1,
		},
		"ateTxP2-burst-be1": {
			frameSize:             256,
			trafficRate:           20,
			expectedThroughputPct: 0.0,
			exp:                   0,
			queue:                 queues.BE1,
			inputIntf:             ateTxP2,
			burstPackets:          50000,
			burstMinGap:           12,
			burstGap:              100,
		},
	}
	top.Flows().Clear()

	dstMac1 := gnmi.Get(t, dut, gnmi.OC().Interface(dut.Port(t, "port1").Name()).Ethernet().MacAddress().State())
	dstMac2 := gnmi.Get(t, dut, gnmi.OC().Interface(dut.Port(t, "port2").Name()).Ethernet().MacAddress().State())

	for trafficID, data := range trafficFlows {
		t.Logf("Configuring flow %s\n", trafficID)

		dstMac := ""
		srcPort := ""

		if strings.Contains(strings.ToUpper(trafficID), "REGULAR") {
			dstMac = dstMac1
			srcPort = "port1"
		} else if strings.Contains(strings.ToUpper(trafficID), "BURST") {
			dstMac = dstMac2
			srcPort = "port2"
		}

		flow := top.Flows().Add().SetName(trafficID)
		flow.Metrics().SetEnable(true)
		flow.TxRx().Port().SetTxName(srcPort).SetRxNames([]string{"port3"})

		ethHeader := flow.Packet().Add().Ethernet()
		ethHeader.Src().SetValue(data.inputIntf.MAC)
		ethHeader.Dst().SetValue(dstMac)

		mplsHeader := flow.Packet().Add().Mpls()
		mplsHeader.Label().SetValue(mplsLabel)
		mplsHeader.TrafficClass().SetValue(uint32(data.exp))

		ipHeader := flow.Packet().Add().Ipv4()
		ipHeader.Src().SetValue(data.inputIntf.IPv4)
		ipHeader.Dst().SetValue(ateRxP3.IPv4)

		flow.Size().SetFixed(uint32(data.frameSize))
		flow.Rate().SetPercentage(float32(data.trafficRate))
		if data.burstMinGap > 0 {
			flow.Duration().Burst().SetPackets(uint32(data.burstPackets)).SetGap(uint32(data.burstMinGap))
		}
		if data.burstGap > 0 {
			flow.Duration().Burst().InterBurstGap().SetBytes(float64(data.burstGap))
		}

	}

	ate.OTG().PushConfig(t, top)
}

func ConfigureDUTIntfIPv4(t *testing.T, dut *ondatra.DUTDevice) {
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
		desc:      "DUT input intf port1",
		intfName:  dp1.Name(),
		ipAddr:    dutIngressPort1AteP1.IPv4,
		prefixLen: dutIngressPort1AteP1.IPv4Len,
	}, {
		desc:      "DUT input intf port2",
		intfName:  dp2.Name(),
		ipAddr:    dutIngressPort2AteP2.IPv4,
		prefixLen: dutIngressPort2AteP2.IPv4Len,
	}, {
		desc:      "DUT output intf port3",
		intfName:  dp3.Name(),
		ipAddr:    dutEgressPort3AteP3.IPv4,
		prefixLen: dutEgressPort3AteP3.IPv4Len,
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
		if deviations.InterfaceEnabled(dut) && !deviations.IPv4MissingEnabled(dut) {
			s.Enabled = ygot.Bool(true)
			t.Logf("DUT %s %s %s requires interface enable deviation ", dut.Vendor(), dut.Model(), dut.Version())
		}
		a := s.GetOrCreateAddress(intf.ipAddr)
		a.PrefixLength = ygot.Uint8(intf.prefixLen)
		gnmi.Replace(t, dut, gnmi.OC().Interface(intf.intfName).Config(), i)
		if deviations.ExplicitInterfaceInDefaultVRF(dut) {
			fptest.AssignToNetworkInstance(t, dut, intf.intfName, deviations.DefaultNetworkInstance(dut), 0)
			t.Logf("DUT %s %s %s requires explicit interface in default VRF deviation ", dut.Vendor(), dut.Model(), dut.Version())
		}
	}
	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, dp1)
		fptest.SetPortSpeed(t, dp2)
		fptest.SetPortSpeed(t, dp3)
		t.Logf("DUT %s %s %s requires explicit port speed set deviation ", dut.Vendor(), dut.Model(), dut.Version())
	}
}

func ConfigureDUTIntfIPv6(t *testing.T, dut *ondatra.DUTDevice) {
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
		desc:      "DUT input intf port1",
		intfName:  dp1.Name(),
		ipAddr:    dutIngressPort1AteP1.IPv6,
		prefixLen: dutIngressPort1AteP1.IPv6Len,
	}, {
		desc:      "DUT input intf port2",
		intfName:  dp2.Name(),
		ipAddr:    dutIngressPort2AteP2.IPv6,
		prefixLen: dutIngressPort2AteP2.IPv6Len,
	}, {
		desc:      "DUT output intf port3",
		intfName:  dp3.Name(),
		ipAddr:    dutEgressPort3AteP3.IPv6,
		prefixLen: dutEgressPort3AteP3.IPv6Len,
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
		s4 := i.GetOrCreateSubinterface(0).GetOrCreateIpv4()
		s := i.GetOrCreateSubinterface(0).GetOrCreateIpv6()
		if deviations.InterfaceEnabled(dut) && !deviations.IPv4MissingEnabled(dut) {
			s4.Enabled = ygot.Bool(true)
		}
		s.Enabled = ygot.Bool(true)
		a := s.GetOrCreateAddress(intf.ipAddr)
		a.PrefixLength = ygot.Uint8(intf.prefixLen)
		gnmi.Replace(t, dut, gnmi.OC().Interface(intf.intfName).Config(), i)
		if deviations.ExplicitInterfaceInDefaultVRF(dut) {
			fptest.AssignToNetworkInstance(t, dut, intf.intfName, deviations.DefaultNetworkInstance(dut), 0)
			t.Logf("DUT %s %s %s requires explicit interface in default VRF deviation ", dut.Vendor(), dut.Model(), dut.Version())
		}
	}
	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, dp1)
		fptest.SetPortSpeed(t, dp2)
		fptest.SetPortSpeed(t, dp3)
		t.Logf("DUT %s %s %s requires explicit port speed set deviation ", dut.Vendor(), dut.Model(), dut.Version())
	}
}

func ConfigureDUTQoSIPv4(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")
	dp3 := dut.Port(t, "port3")
	d := &oc.Root{}
	q := d.GetOrCreateQos()
	queues := netutil.CommonTrafficQueues(t, dut)

	if deviations.QOSQueueRequiresID(dut) {
		queueNames := []string{queues.NC1, queues.AF4, queues.AF3, queues.AF2, queues.AF1, queues.BE1}
		for i, queue := range queueNames {
			q1 := q.GetOrCreateQueue(queue)
			q1.Name = ygot.String(queue)
			queueid := len(queueNames) - i
			q1.QueueId = ygot.Uint8(uint8(queueid))
		}
		t.Logf("\nDUT %s %s %s requires QoS queue requires ID deviation \n\n", dut.Vendor(), dut.Model(), dut.Version())
	}

	t.Logf("Create QoS forwarding groups and queue names configuration")
	forwardingGroups := []struct {
		desc        string
		queueName   string
		targetGroup string
		priority    uint8
	}{{
		desc:        "forwarding-group-BE1",
		queueName:   queues.BE1,
		targetGroup: "target-group-BE1",
		priority:    0,
	}, {
		desc:        "forwarding-group-AF1",
		queueName:   queues.AF1,
		targetGroup: "target-group-AF1",
		priority:    1,
	}, {
		desc:        "forwarding-group-AF2",
		queueName:   queues.AF2,
		targetGroup: "target-group-AF2",
		priority:    2,
	}, {
		desc:        "forwarding-group-AF3",
		queueName:   queues.AF3,
		targetGroup: "target-group-AF3",
		priority:    3,
	}, {
		desc:        "forwarding-group-AF4",
		queueName:   queues.AF4,
		targetGroup: "target-group-AF4",
		priority:    4,
	}, {
		desc:        "forwarding-group-NC1",
		queueName:   queues.NC1,
		targetGroup: "target-group-NC1",
		priority:    5,
	}}

	t.Logf("QoS forwarding groups config: %v", forwardingGroups)
	for _, tc := range forwardingGroups {
		qoscfg.SetForwardingGroup(t, dut, q, tc.targetGroup, tc.queueName)

		t.Logf("QoS forwarding groups config: %v", forwardingGroups)
	}

	t.Logf("Create QoS Classifiers config")
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
		dscpSet:     []uint8{0},
	}, {
		desc:        "classifier_ipv4_af1",
		name:        "dscp_based_classifier_ipv4",
		classType:   oc.Qos_Classifier_Type_IPV4,
		termID:      "1",
		targetGroup: "target-group-AF1",
		dscpSet:     []uint8{1},
	}, {
		desc:        "classifier_ipv4_af2",
		name:        "dscp_based_classifier_ipv4",
		classType:   oc.Qos_Classifier_Type_IPV4,
		termID:      "2",
		targetGroup: "target-group-AF2",
		dscpSet:     []uint8{2},
	}, {
		desc:        "classifier_ipv4_af3",
		name:        "dscp_based_classifier_ipv4",
		classType:   oc.Qos_Classifier_Type_IPV4,
		termID:      "3",
		targetGroup: "target-group-AF3",
		dscpSet:     []uint8{3},
	}, {
		desc:        "classifier_ipv4_af4",
		name:        "dscp_based_classifier_ipv4",
		classType:   oc.Qos_Classifier_Type_IPV4,
		termID:      "4",
		targetGroup: "target-group-AF4",
		dscpSet:     []uint8{4, 5},
	}, {
		desc:        "classifier_ipv4_nc1",
		name:        "dscp_based_classifier_ipv4",
		classType:   oc.Qos_Classifier_Type_IPV4,
		termID:      "5",
		targetGroup: "target-group-NC1",
		dscpSet:     []uint8{6, 7},
	}}

	t.Logf("QoS classifiers config: %v", classifiers)
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
		condition.GetOrCreateIpv4().SetDscpSet(tc.dscpSet)

		gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), q)
	}

	t.Logf("Create QoS input classifier config")
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
		desc:                "Input Classifier Type IPV4",
		intf:                dp2.Name(),
		inputClassifierType: oc.Input_Classifier_Type_IPV4,
		classifier:          "dscp_based_classifier_ipv4",
	}}

	t.Logf("QoS input classifier config: %v", classifierIntfs)
	for _, tc := range classifierIntfs {
		qoscfg.SetInputClassifier(t, dut, q, tc.intf, tc.inputClassifierType, tc.classifier)
	}

	t.Logf("Create QoS scheduler policies config")
	schedulerPolicies := []struct {
		desc        string
		sequence    uint32
		setPriority bool
		priority    oc.E_Scheduler_Priority
		inputID     string
		inputType   oc.E_Input_InputType
		setWeight   bool
		queueName   string
		targetGroup string
	}{{
		desc:        "scheduler-policy-BE1",
		sequence:    uint32(0),
		setPriority: true,
		setWeight:   false,
		priority:    oc.Scheduler_Priority_STRICT,
		inputID:     "BE1",
		inputType:   oc.Input_InputType_QUEUE,
		queueName:   queues.BE1,
		targetGroup: "target-group-BE1",
	}, {
		desc:        "scheduler-policy-AF1",
		sequence:    uint32(0),
		setPriority: true,
		setWeight:   false,
		priority:    oc.Scheduler_Priority_STRICT,
		inputID:     "AF1",
		inputType:   oc.Input_InputType_QUEUE,
		queueName:   queues.AF1,
		targetGroup: "target-group-AF1",
	}, {
		desc:        "scheduler-policy-AF2",
		sequence:    uint32(0),
		setPriority: true,
		setWeight:   false,
		priority:    oc.Scheduler_Priority_STRICT,
		inputID:     "AF2",
		inputType:   oc.Input_InputType_QUEUE,
		queueName:   queues.AF2,
		targetGroup: "target-group-AF2",
	}, {
		desc:        "scheduler-policy-AF3",
		sequence:    uint32(0),
		setPriority: true,
		setWeight:   false,
		priority:    oc.Scheduler_Priority_STRICT,
		inputID:     "AF3",
		inputType:   oc.Input_InputType_QUEUE,
		queueName:   queues.AF3,
		targetGroup: "target-group-AF3",
	}, {
		desc:        "scheduler-policy-AF4",
		sequence:    uint32(0),
		setPriority: true,
		setWeight:   false,
		priority:    oc.Scheduler_Priority_STRICT,
		inputID:     "AF4",
		inputType:   oc.Input_InputType_QUEUE,
		queueName:   queues.AF4,
		targetGroup: "target-group-AF4",
	}, {
		desc:        "scheduler-policy-NC1",
		sequence:    uint32(0),
		setPriority: true,
		setWeight:   false,
		priority:    oc.Scheduler_Priority_STRICT,
		inputID:     "NC1",
		inputType:   oc.Input_InputType_QUEUE,
		queueName:   queues.NC1,
		targetGroup: "target-group-NC1",
	}}

	schedulerPolicy := q.GetOrCreateSchedulerPolicy("scheduler")
	schedulerPolicy.SetName("scheduler")
	t.Logf("QoS scheduler policies config: %v", schedulerPolicies)
	for _, tc := range schedulerPolicies {
		s := schedulerPolicy.GetOrCreateScheduler(tc.sequence)
		s.SetSequence(tc.sequence)
		if tc.setPriority {
			s.SetPriority(tc.priority)
		}
		input := s.GetOrCreateInput(tc.inputID)
		input.SetId(tc.inputID)
		input.SetInputType(tc.inputType)
		input.SetQueue(tc.queueName)
		gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), q)
	}

	t.Logf("Create QoS output interface config")
	schedulerIntfs := []struct {
		desc      string
		queueName string
		scheduler string
	}{{
		desc:      "output-interface-BE1",
		queueName: queues.BE1,
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

	t.Logf("QoS output interface config: %v", schedulerIntfs)
	for _, tc := range schedulerIntfs {
		i := q.GetOrCreateInterface(dp3.Name())
		i.SetInterfaceId(dp3.Name())
		i.GetOrCreateInterfaceRef().Interface = ygot.String(dp3.Name())
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

func ConfigureDUTQoSIPv6(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")
	dp3 := dut.Port(t, "port3")
	d := &oc.Root{}
	q := d.GetOrCreateQos()
	queues := netutil.CommonTrafficQueues(t, dut)

	if deviations.QOSQueueRequiresID(dut) {
		queueNames := []string{queues.NC1, queues.AF4, queues.AF3, queues.AF2, queues.AF1, queues.BE1}
		for i, queue := range queueNames {
			q1 := q.GetOrCreateQueue(queue)
			q1.Name = ygot.String(queue)
			queueid := len(queueNames) - i
			q1.QueueId = ygot.Uint8(uint8(queueid))
		}
		t.Logf("\nDUT %s %s %s requires QoS queue requires ID deviation \n\n", dut.Vendor(), dut.Model(), dut.Version())
	}

	t.Logf("Create QoS forwarding groups and queue names configuration")
	forwardingGroups := []struct {
		desc        string
		queueName   string
		targetGroup string
		priority    uint8
	}{{
		desc:        "forwarding-group-BE1",
		queueName:   queues.BE1,
		targetGroup: "target-group-BE1",
		priority:    0,
	}, {
		desc:        "forwarding-group-AF1",
		queueName:   queues.AF1,
		targetGroup: "target-group-AF1",
		priority:    1,
	}, {
		desc:        "forwarding-group-AF2",
		queueName:   queues.AF2,
		targetGroup: "target-group-AF2",
		priority:    2,
	}, {
		desc:        "forwarding-group-AF3",
		queueName:   queues.AF3,
		targetGroup: "target-group-AF3",
		priority:    3,
	}, {
		desc:        "forwarding-group-AF4",
		queueName:   queues.AF4,
		targetGroup: "target-group-AF4",
		priority:    4,
	}, {
		desc:        "forwarding-group-NC1",
		queueName:   queues.NC1,
		targetGroup: "target-group-NC1",
		priority:    5,
	}}

	t.Logf("QoS forwarding groups config: %v", forwardingGroups)
	for _, tc := range forwardingGroups {
		qoscfg.SetForwardingGroup(t, dut, q, tc.targetGroup, tc.queueName)
		t.Logf("QoS forwarding groups config: %v", forwardingGroups)
	}

	t.Logf("Create QoS Classifiers config")
	classifiers := []struct {
		desc        string
		name        string
		classType   oc.E_Qos_Classifier_Type
		termID      string
		targetGroup string
		dscpSet     []uint8
	}{{
		desc:        "classifier_ipv6_be1",
		name:        "dscp_based_classifier_ipv6",
		classType:   oc.Qos_Classifier_Type_IPV6,
		termID:      "0",
		targetGroup: "target-group-BE1",
		dscpSet:     []uint8{0, 1, 2, 3, 4, 5, 6, 7},
	}, {
		desc:        "classifier_ipv6_af1",
		name:        "dscp_based_classifier_ipv6",
		classType:   oc.Qos_Classifier_Type_IPV6,
		termID:      "1",
		targetGroup: "target-group-AF1",
		dscpSet:     []uint8{8, 9, 10, 11, 12, 13, 14, 15},
	}, {
		desc:        "classifier_ipv6_af2",
		name:        "dscp_based_classifier_ipv6",
		classType:   oc.Qos_Classifier_Type_IPV6,
		termID:      "2",
		targetGroup: "target-group-AF2",
		dscpSet:     []uint8{16, 17, 18, 19, 20, 21, 22, 23},
	}, {
		desc:        "classifier_ipv6_af3",
		name:        "dscp_based_classifier_ipv6",
		classType:   oc.Qos_Classifier_Type_IPV6,
		termID:      "3",
		targetGroup: "target-group-AF3",
		dscpSet:     []uint8{24, 25, 26, 27, 28, 29, 30, 31},
	}, {
		desc:        "classifier_ipv6_af4",
		name:        "dscp_based_classifier_ipv6",
		classType:   oc.Qos_Classifier_Type_IPV6,
		termID:      "4",
		targetGroup: "target-group-AF4",
		dscpSet:     []uint8{32, 33, 34, 35, 36, 37, 38, 39, 40, 41, 42, 43, 44, 45, 46, 47},
	}, {
		desc:        "classifier_ipv6_nc1",
		name:        "dscp_based_classifier_ipv6",
		classType:   oc.Qos_Classifier_Type_IPV6,
		termID:      "5",
		targetGroup: "target-group-NC1",
		dscpSet:     []uint8{48, 49, 50, 51, 52, 53, 54, 55, 56, 57, 58, 59, 60, 61, 62, 63},
	}}

	t.Logf("QoS classifiers config: %v", classifiers)
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
		condition.GetOrCreateIpv6().SetDscpSet(tc.dscpSet)
		gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), q)
	}

	t.Logf("Create QoS input classifier config")
	classifierIntfs := []struct {
		desc                string
		intf                string
		inputClassifierType oc.E_Input_Classifier_Type
		classifier          string
	}{{
		desc:                "Input Classifier Type IPV6",
		intf:                dp1.Name(),
		inputClassifierType: oc.Input_Classifier_Type_IPV6,
		classifier:          "dscp_based_classifier_ipv6",
	}, {
		desc:                "Input Classifier Type IPV6",
		intf:                dp2.Name(),
		inputClassifierType: oc.Input_Classifier_Type_IPV6,
		classifier:          "dscp_based_classifier_ipv6",
	}}

	t.Logf("QoS input classifier config: %v", classifierIntfs)
	for _, tc := range classifierIntfs {
		qoscfg.SetInputClassifier(t, dut, q, tc.intf, tc.inputClassifierType, tc.classifier)
	}

	t.Logf("Create QoS scheduler policies config")
	schedulerPolicies := []struct {
		desc        string
		sequence    uint32
		setPriority bool
		priority    oc.E_Scheduler_Priority
		inputID     string
		inputType   oc.E_Input_InputType
		setWeight   bool
		queueName   string
		targetGroup string
	}{{
		desc:        "scheduler-policy-BE1",
		sequence:    uint32(0),
		setPriority: true,
		setWeight:   false,
		priority:    oc.Scheduler_Priority_STRICT,
		inputID:     "BE1",
		inputType:   oc.Input_InputType_QUEUE,
		queueName:   queues.BE1,
		targetGroup: "target-group-BE1",
	}, {
		desc:        "scheduler-policy-AF1",
		sequence:    uint32(0),
		setPriority: true,
		setWeight:   false,
		priority:    oc.Scheduler_Priority_STRICT,
		inputID:     "AF1",
		inputType:   oc.Input_InputType_QUEUE,
		queueName:   queues.AF1,
		targetGroup: "target-group-AF1",
	}, {
		desc:        "scheduler-policy-AF2",
		sequence:    uint32(0),
		setPriority: true,
		setWeight:   false,
		priority:    oc.Scheduler_Priority_STRICT,
		inputID:     "AF2",
		inputType:   oc.Input_InputType_QUEUE,
		queueName:   queues.AF2,
		targetGroup: "target-group-AF2",
	}, {
		desc:        "scheduler-policy-AF3",
		sequence:    uint32(0),
		setPriority: true,
		setWeight:   false,
		priority:    oc.Scheduler_Priority_STRICT,
		inputID:     "AF3",
		inputType:   oc.Input_InputType_QUEUE,
		queueName:   queues.AF3,
		targetGroup: "target-group-AF3",
	}, {
		desc:        "scheduler-policy-AF4",
		sequence:    uint32(0),
		setPriority: true,
		setWeight:   false,
		priority:    oc.Scheduler_Priority_STRICT,
		inputID:     "AF4",
		inputType:   oc.Input_InputType_QUEUE,
		queueName:   queues.AF4,
		targetGroup: "target-group-AF4",
	}, {
		desc:        "scheduler-policy-NC1",
		sequence:    uint32(0),
		setPriority: true,
		setWeight:   false,
		priority:    oc.Scheduler_Priority_STRICT,
		inputID:     "NC1",
		inputType:   oc.Input_InputType_QUEUE,
		queueName:   queues.NC1,
		targetGroup: "target-group-NC1",
	}}

	schedulerPolicy := q.GetOrCreateSchedulerPolicy("scheduler")
	schedulerPolicy.SetName("scheduler")
	t.Logf("QoS scheduler policies config: %v", schedulerPolicies)
	for _, tc := range schedulerPolicies {
		s := schedulerPolicy.GetOrCreateScheduler(tc.sequence)
		s.SetSequence(tc.sequence)
		if tc.setPriority {
			s.SetPriority(tc.priority)
		}
		input := s.GetOrCreateInput(tc.inputID)
		input.SetId(tc.inputID)
		input.SetInputType(tc.inputType)
		input.SetQueue(tc.queueName)
		gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), q)
	}

	t.Logf("Create QoS output interface config")
	schedulerIntfs := []struct {
		desc      string
		queueName string
		scheduler string
	}{{
		desc:      "output-interface-BE1",
		queueName: queues.BE1,
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

	t.Logf("QoS output interface config: %v", schedulerIntfs)
	for _, tc := range schedulerIntfs {
		i := q.GetOrCreateInterface(dp3.Name())
		i.SetInterfaceId(dp3.Name())
		i.GetOrCreateInterfaceRef().Interface = ygot.String(dp3.Name())
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

func ConfigureDUTQoSMPLS(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")
	dp3 := dut.Port(t, "port3")
	d := &oc.Root{}
	q := d.GetOrCreateQos()
	queues := netutil.CommonTrafficQueues(t, dut)

	if deviations.QOSQueueRequiresID(dut) {
		queueNames := []string{queues.NC1, queues.AF4, queues.AF3, queues.AF2, queues.AF1, queues.BE1}
		for i, queue := range queueNames {
			q1 := q.GetOrCreateQueue(queue)
			q1.Name = ygot.String(queue)
			queueid := len(queueNames) - i
			q1.QueueId = ygot.Uint8(uint8(queueid))
		}
		t.Logf("\nDUT %s %s %s requires QoS queue requires ID deviation \n\n", dut.Vendor(), dut.Model(), dut.Version())
	}

	t.Logf("Create QoS forwarding groups and queue names configuration")
	forwardingGroups := []struct {
		desc        string
		queueName   string
		targetGroup string
		priority    uint8
	}{{
		desc:        "forwarding-group-BE1",
		queueName:   queues.BE1,
		targetGroup: "target-group-BE1",
		priority:    0,
	}, {
		desc:        "forwarding-group-AF1",
		queueName:   queues.AF1,
		targetGroup: "target-group-AF1",
		priority:    1,
	}, {
		desc:        "forwarding-group-AF2",
		queueName:   queues.AF2,
		targetGroup: "target-group-AF2",
		priority:    2,
	}, {
		desc:        "forwarding-group-AF3",
		queueName:   queues.AF3,
		targetGroup: "target-group-AF3",
		priority:    3,
	}, {
		desc:        "forwarding-group-AF4",
		queueName:   queues.AF4,
		targetGroup: "target-group-AF4",
		priority:    4,
	}, {
		desc:        "forwarding-group-NC1",
		queueName:   queues.NC1,
		targetGroup: "target-group-NC1",
		priority:    5,
	}}

	t.Logf("QoS forwarding groups config: %v", forwardingGroups)
	for _, tc := range forwardingGroups {
		qoscfg.SetForwardingGroup(t, dut, q, tc.targetGroup, tc.queueName)

		t.Logf("QoS forwarding groups config: %v", forwardingGroups)
	}

	t.Logf("Create QoS Classifiers config")
	classifiers := []struct {
		desc        string
		name        string
		classType   oc.E_Qos_Classifier_Type
		termID      string
		targetGroup string
		dscpSet     []uint8
	}{{
		desc:        "classifier_mpls_be1",
		name:        "dscp_based_classifier_mpls",
		classType:   oc.Qos_Classifier_Type_MPLS,
		termID:      "0",
		targetGroup: "target-group-BE1",
		dscpSet:     []uint8{0},
	}, {
		desc:        "classifier_mpls_af1",
		name:        "dscp_based_classifier_mpls",
		classType:   oc.Qos_Classifier_Type_MPLS,
		termID:      "1",
		targetGroup: "target-group-AF1",
		dscpSet:     []uint8{1},
	}, {
		desc:        "classifier_mpls_af2",
		name:        "dscp_based_classifier_mpls",
		classType:   oc.Qos_Classifier_Type_MPLS,
		termID:      "2",
		targetGroup: "target-group-AF2",
		dscpSet:     []uint8{2},
	}, {
		desc:        "classifier_mpls_af3",
		name:        "dscp_based_classifier_mpls",
		classType:   oc.Qos_Classifier_Type_MPLS,
		termID:      "3",
		targetGroup: "target-group-AF3",
		dscpSet:     []uint8{3},
	}, {
		desc:        "classifier_mpls_af4",
		name:        "dscp_based_classifier_mpls",
		classType:   oc.Qos_Classifier_Type_MPLS,
		termID:      "4",
		targetGroup: "target-group-AF4",
		dscpSet:     []uint8{4, 5},
	}, {
		desc:        "classifier_mpls_nc1",
		name:        "dscp_based_classifier_mpls",
		classType:   oc.Qos_Classifier_Type_MPLS,
		termID:      "5",
		targetGroup: "target-group-NC1",
		dscpSet:     []uint8{6, 7},
	}}

	t.Logf("QoS classifiers config: %v", classifiers)
	if deviations.MplsExpIngressClassifierOcUnsupported(dut) {
		configureMplsExpClassifierCLI(t, dut, classifiers)
	} else {
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
			condition.GetOrCreateMpls().SetTrafficClass(tc.dscpSet[0])
			gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), q)
		}

		t.Logf("Create QoS input classifier config")
		classifierIntfs := []struct {
			desc                string
			intf                string
			inputClassifierType oc.E_Input_Classifier_Type
			classifier          string
		}{{
			desc:                "Input Classifier Type MPLS",
			intf:                dp1.Name(),
			inputClassifierType: oc.Input_Classifier_Type_MPLS,
			classifier:          "dscp_based_classifier_mpls",
		}, {
			desc:                "Input Classifier Type MPLS",
			intf:                dp2.Name(),
			inputClassifierType: oc.Input_Classifier_Type_MPLS,
			classifier:          "dscp_based_classifier_mpls",
		}}

		t.Logf("QoS input classifier config: %v", classifierIntfs)
		for _, tc := range classifierIntfs {
			qoscfg.SetInputClassifier(t, dut, q, tc.intf, tc.inputClassifierType, tc.classifier)
		}
	}

	t.Logf("Create QoS scheduler policies config")
	schedulerPolicies := []struct {
		desc        string
		sequence    uint32
		setPriority bool
		priority    oc.E_Scheduler_Priority
		inputID     string
		inputType   oc.E_Input_InputType
		setWeight   bool
		queueName   string
		targetGroup string
	}{{
		desc:        "scheduler-policy-BE1",
		sequence:    uint32(0),
		setPriority: true,
		setWeight:   false,
		priority:    oc.Scheduler_Priority_STRICT,
		inputID:     "BE1",
		inputType:   oc.Input_InputType_QUEUE,
		queueName:   queues.BE1,
		targetGroup: "target-group-BE1",
	}, {
		desc:        "scheduler-policy-AF1",
		sequence:    uint32(0),
		setPriority: true,
		setWeight:   false,
		priority:    oc.Scheduler_Priority_STRICT,
		inputID:     "AF1",
		inputType:   oc.Input_InputType_QUEUE,
		queueName:   queues.AF1,
		targetGroup: "target-group-AF1",
	}, {
		desc:        "scheduler-policy-AF2",
		sequence:    uint32(0),
		setPriority: true,
		setWeight:   false,
		priority:    oc.Scheduler_Priority_STRICT,
		inputID:     "AF2",
		inputType:   oc.Input_InputType_QUEUE,
		queueName:   queues.AF2,
		targetGroup: "target-group-AF2",
	}, {
		desc:        "scheduler-policy-AF3",
		sequence:    uint32(0),
		setPriority: true,
		setWeight:   false,
		priority:    oc.Scheduler_Priority_STRICT,
		inputID:     "AF3",
		inputType:   oc.Input_InputType_QUEUE,
		queueName:   queues.AF3,
		targetGroup: "target-group-AF3",
	}, {
		desc:        "scheduler-policy-AF4",
		sequence:    uint32(0),
		setPriority: true,
		setWeight:   false,
		priority:    oc.Scheduler_Priority_STRICT,
		inputID:     "AF4",
		inputType:   oc.Input_InputType_QUEUE,
		queueName:   queues.AF4,
		targetGroup: "target-group-AF4",
	}, {
		desc:        "scheduler-policy-NC1",
		sequence:    uint32(0),
		setPriority: true,
		setWeight:   false,
		priority:    oc.Scheduler_Priority_STRICT,
		inputID:     "NC1",
		inputType:   oc.Input_InputType_QUEUE,
		queueName:   queues.NC1,
		targetGroup: "target-group-NC1",
	}}

	schedulerPolicy := q.GetOrCreateSchedulerPolicy("scheduler")
	schedulerPolicy.SetName("scheduler")
	t.Logf("QoS scheduler policies config: %v", schedulerPolicies)
	for _, tc := range schedulerPolicies {
		s := schedulerPolicy.GetOrCreateScheduler(tc.sequence)
		s.SetSequence(tc.sequence)
		if tc.setPriority {
			s.SetPriority(tc.priority)
		}
		input := s.GetOrCreateInput(tc.inputID)
		input.SetId(tc.inputID)
		input.SetInputType(tc.inputType)
		input.SetQueue(tc.queueName)
		gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), q)
	}

	t.Logf("Create QoS output interface config")
	schedulerIntfs := []struct {
		desc      string
		queueName string
		scheduler string
	}{{
		desc:      "output-interface-BE1",
		queueName: queues.BE1,
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

	t.Logf("QoS output interface config: %v", schedulerIntfs)
	for _, tc := range schedulerIntfs {
		i := q.GetOrCreateInterface(dp3.Name())
		i.SetInterfaceId(dp3.Name())
		i.GetOrCreateInterfaceRef().Interface = ygot.String(dp3.Name())
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

func runCliCommand(t *testing.T, dut *ondatra.DUTDevice, cliCommand string) string {
	cliClient := dut.RawAPIs().CLI(t)
	output, err := cliClient.RunCommand(context.Background(), cliCommand)
	if err != nil {
		t.Fatalf("Failed to execute CLI command '%s': %v", cliCommand, err)
	}
	t.Logf("Received from cli: %s", output.Output())
	return output.Output()
}

func configureQoSGlobalParams(t *testing.T, dut *ondatra.DUTDevice) {

	queues := netutil.CommonTrafficQueues(t, dut)
	qosQNameSet := `
	configure terminal
	!
	qos tx-queue %d name %s
	!
	`
	qosMapTC := `
	configure terminal
	!
	qos map traffic-class %d to tx-queue %d
	!
	`

	qosCfgTargetGroup := `
	configure terminal
	!
	qos traffic-class %d name %s
	!
	`

	runCliCommand(t, dut, "show version")

	qList := []string{queues.BE1, queues.AF1, queues.AF2, queues.AF3, queues.AF4, queues.NC1}
	for index, queue := range qList {

		runCliCommand(t, dut, fmt.Sprintf(qosQNameSet, index, queue))
		time.Sleep(time.Second)
		runCliCommand(t, dut, fmt.Sprintf(qosMapTC, index, index))
		time.Sleep(time.Second)
		runCliCommand(t, dut, fmt.Sprintf(qosCfgTargetGroup, index, fmt.Sprintf("target-group-%s", queue)))
		time.Sleep(time.Second)
	}

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

func configureTcamProfile(t *testing.T, dut *ondatra.DUTDevice) {

	tcamProfileConfig := `
hardware counter feature traffic-policy in
!
hardware tcam
  profile ancx
    feature acl port ip
        sequence 45
        key size limit 160
        key field dscp dst-ip ip-frag ip-protocol l4-dst-port l4-ops l4-src-port src-ip tcp-control ttl
        action count drop mirror
        packet ipv4 forwarding bridged
        packet ipv4 forwarding routed
        packet ipv4 forwarding routed multicast
        packet ipv4 mpls ipv4 forwarding mpls decap
        packet ipv4 mpls ipv6 forwarding mpls decap
        packet ipv4 non-vxlan forwarding routed decap
        packet ipv4 vxlan eth ipv4 forwarding routed decap
        packet ipv4 vxlan forwarding bridged decap
    feature acl port ip egress mpls-tunnelled-match
        sequence 95
    feature acl port ipv6
        sequence 25
        key field dst-ipv6 ipv6-next-header ipv6-traffic-class l4-dst-port l4-ops-3b l4-src-port src-ipv6-high src-ipv6-low tcp-control
        action count drop mirror
        packet ipv6 forwarding bridged
        packet ipv6 forwarding routed
        packet ipv6 forwarding routed multicast
        packet ipv6 ipv6 forwarding routed decap
    feature acl port ipv6 egress
        sequence 105
        key field dst-ipv6 ipv6-next-header ipv6-traffic-class l4-dst-port l4-src-port src-ipv6-high src-ipv6-low tcp-control
        action count drop mirror
        packet ipv6 forwarding bridged
        packet ipv6 forwarding routed
    feature acl port mac
        sequence 55
        key size limit 160
        key field dst-mac ether-type src-mac
        action count drop mirror
        packet ipv4 forwarding bridged
        packet ipv4 forwarding routed
        packet ipv4 forwarding routed multicast
        packet ipv4 mpls ipv4 forwarding mpls decap
        packet ipv4 mpls ipv6 forwarding mpls decap
        packet ipv4 non-vxlan forwarding routed decap
        packet ipv4 vxlan forwarding bridged decap
        packet ipv6 forwarding bridged
        packet ipv6 forwarding routed
        packet ipv6 forwarding routed decap
        packet ipv6 forwarding routed multicast
        packet ipv6 ipv6 forwarding routed decap
        packet mpls forwarding bridged decap
        packet mpls ipv4 forwarding mpls
        packet mpls ipv6 forwarding mpls
        packet mpls non-ip forwarding mpls
        packet non-ip forwarding bridged
    feature acl vlan ipv6 egress
        sequence 20
        key field dst-ipv6 ipv6-next-header ipv6-traffic-class l4-dst-port l4-src-port src-ipv6-high src-ipv6-low tcp-control
        action count drop mirror
        packet ipv6 forwarding bridged
        packet ipv6 forwarding routed
    feature counter lfib
        sequence 85
    feature forwarding-destination mpls
        sequence 100
    feature mirror ip
        sequence 80
        key size limit 160
        key field dscp dst-ip ip-frag ip-protocol l4-dst-port l4-ops l4-src-port src-ip tcp-control
        action count mirror set-policer
        packet ipv4 forwarding bridged
        packet ipv4 forwarding routed
        packet ipv4 forwarding routed multicast
        packet ipv4 non-vxlan forwarding routed decap
    feature mpls
        sequence 5
        key size limit 160
        action drop redirect set-ecn
        packet ipv4 mpls ipv4 forwarding mpls decap
        packet ipv4 mpls ipv6 forwarding mpls decap
        packet mpls ipv4 forwarding mpls
        packet mpls ipv6 forwarding mpls
        packet mpls non-ip forwarding mpls
    feature mpls pop ingress
        sequence 90
    feature pbr mpls
        sequence 65
        key size limit 160
        key field mpls-inner-ip-tos
        action count drop redirect
        packet mpls ipv4 forwarding mpls
        packet mpls ipv6 forwarding mpls
        packet mpls non-ip forwarding mpls
    feature qos ip
        sequence 75
        key size limit 160
        key field dscp dst-ip ip-frag ip-protocol l4-dst-port l4-ops l4-src-port src-ip tcp-control
        action set-dscp set-policer set-tc
        packet ipv4 forwarding routed
        packet ipv4 forwarding routed multicast
        packet ipv4 mpls ipv4 forwarding mpls decap
        packet ipv4 mpls ipv6 forwarding mpls decap
        packet ipv4 non-vxlan forwarding routed decap
    feature qos ipv6
        sequence 70
        key field dst-ipv6 ipv6-next-header ipv6-traffic-class l4-dst-port l4-src-port src-ipv6-high src-ipv6-low
        action set-dscp set-policer set-tc
        packet ipv6 forwarding routed
    feature traffic-policy port ipv4
        sequence 45
        key size limit 160
        key field dscp dst-ip-label ip-frag ip-fragment-offset ip-length ip-protocol l4-dst-port-label l4-src-port-label src-ip-label tcp-control ttl
        action count drop redirect set-dscp set-tc
        packet ipv4 forwarding routed
    feature traffic-policy port ipv4 egress
        key size limit 160
        key field dscp dst-ip-label ip-frag ip-protocol l4-dst-port-label l4-src-port-label src-ip-label
        action count drop
        packet ipv4 forwarding routed
    feature traffic-policy port ipv6
        sequence 25
        key size limit 160
        key field dst-ipv6-label hop-limit ipv6-length ipv6-next-header ipv6-traffic-class l4-dst-port-label l4-src-port-label src-ipv6-label tcp-control
        action count drop redirect set-dscp set-tc
        packet ipv6 forwarding routed
    feature traffic-policy port ipv6 egress
        key size limit 160
        key field dscp dst-ipv6-label ipv6-next-header l4-dst-port-label l4-src-port-label src-ipv6-label
        action count drop
        packet ipv6 forwarding routed
    feature tunnel vxlan
        sequence 50
        key size limit 160
        packet ipv4 vxlan eth ipv4 forwarding routed decap
        packet ipv4 vxlan forwarding bridged decap
  system profile ancx
!
    `

	if dut.Vendor() != ondatra.ARISTA || strings.ToLower(dut.Model()) == "ceos" {
		t.Logf("TCAM profile not supported on %s %s", dut.Name(), dut.Model())
		return
	}

	gnmiClient := dut.RawAPIs().GNMI(t)
	t.Logf("Push the Tcam profile:%s", dut.Vendor())
	gpbSetRequest := buildCliSetRequest(tcamProfileConfig)
	if _, err := gnmiClient.Set(context.Background(), gpbSetRequest); err != nil {
		t.Fatalf("Failed to set TCAM profile from CLI: %v", err)
	}
}

func configureMplsExpClassifierCLI(t *testing.T, dut *ondatra.DUTDevice, classifiers []struct {
	desc        string
	name        string
	classType   oc.E_Qos_Classifier_Type
	termID      string
	targetGroup string
	dscpSet     []uint8
}) {

	qosMapCmd := fmt.Sprintf(`
	mpls ip
	!
	mpls static top-label %d %s pop payload-type ipv4
	!
	`, mplsLabel, ateRxP3.IPv4)
	qosMapExp := `
	qos map exp %d to traffic-class %s
	!
	`

	for _, classifier := range classifiers {
		tc := classifier.termID
		for _, exp := range classifier.dscpSet {
			qosMapCmd += fmt.Sprintf(qosMapExp, exp, tc)
			t.Logf(qosMapExp, exp, tc)

		}
	}

	gnmiClient := dut.RawAPIs().GNMI(t)
	t.Logf("Push the CLI QoS config:%s", dut.Vendor())
	gpbSetRequest := buildCliSetRequest(qosMapCmd)
	if _, err := gnmiClient.Set(context.Background(), gpbSetRequest); err != nil {
		t.Fatalf("Failed to set QoS Exp mappings from CLI: %v", err)
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
