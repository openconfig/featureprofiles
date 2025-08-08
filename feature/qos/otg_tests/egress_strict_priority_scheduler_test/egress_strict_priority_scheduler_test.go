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

package egress_strict_priority_scheduler_test

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
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

func TestEgressStrictPrioritySchedulerTraffic(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	t.Logf("Configuring TCAM Profile")
	cfgplugins.ConfigureTcamProfile(t, dut)

	t.Logf("Configuring QoS Global parameters")
	cfgplugins.NewQosInitialize(t, dut)

	verifyEgressStrictPrioritySchedulerTrafficIPv4(t, dut)
	verifyEgressStrictPrioritySchedulerTrafficIPv6(t, dut)
	verifyEgressStrictPrioritySchedulerTrafficMPLS(t, dut)
}

func verifyEgressStrictPrioritySchedulerTrafficIPv4(t *testing.T, dut *ondatra.DUTDevice) {

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
			"ateTxP2-regular-nc1": {
				frameSize:             512,
				trafficRate:           1,
				dscp:                  7,
				expectedThroughputPct: 100.0,
				queue:                 queues.NC1,
				inputIntf:             ateTxP2,
			},
			"ateTxP1-regular-af4": {
				frameSize:             512,
				trafficRate:           30,
				expectedThroughputPct: 100.0,
				dscp:                  4,
				queue:                 queues.AF4,
				inputIntf:             ateTxP1,
			},
			"ateTxP2-regular-af4": {
				frameSize:             512,
				trafficRate:           30,
				dscp:                  5,
				expectedThroughputPct: 100.0,
				queue:                 queues.AF4,
				inputIntf:             ateTxP2,
			},
			"ateTxP1-regular-af3": {
				frameSize:             512,
				trafficRate:           12,
				expectedThroughputPct: 100.0,
				dscp:                  3,
				queue:                 queues.AF3,
				inputIntf:             ateTxP1,
			},
			"ateTxP2-regular-af3": {
				frameSize:             512,
				trafficRate:           12,
				dscp:                  3,
				expectedThroughputPct: 100.0,
				queue:                 queues.AF3,
				inputIntf:             ateTxP2,
			},
			"ateTxP1-regular-af2": {
				frameSize:             512,
				trafficRate:           10,
				expectedThroughputPct: 50.0,
				dscp:                  2,
				queue:                 queues.AF2,
				inputIntf:             ateTxP1,
			},
			"ateTxP2-regular-af2": {
				frameSize:             512,
				trafficRate:           10,
				dscp:                  2,
				expectedThroughputPct: 50.0,
				queue:                 queues.AF2,
				inputIntf:             ateTxP2,
			},
			"ateTxP1-regular-af1": {
				frameSize:             512,
				trafficRate:           12,
				expectedThroughputPct: 0.0,
				dscp:                  1,
				queue:                 queues.AF1,
				inputIntf:             ateTxP1,
			},
			"ateTxP2-regular-af1": {
				frameSize:             512,
				trafficRate:           12,
				dscp:                  1,
				expectedThroughputPct: 0.0,
				queue:                 queues.AF1,
				inputIntf:             ateTxP2,
			},
			"ateTxP1-regular-be1": {
				frameSize:             512,
				trafficRate:           12,
				expectedThroughputPct: 0.0,
				dscp:                  0,
				queue:                 queues.BE1,
				inputIntf:             ateTxP1,
			},
			"ateTxP2-regular-be1": {
				frameSize:             512,
				trafficRate:           12,
				expectedThroughputPct: 0.0,
				dscp:                  0,
				queue:                 queues.BE1,
				inputIntf:             ateTxP2,
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
		t.Logf("Running regular traffic on DUT interfaces: %s => %s \n", dp2.Name(), dp3.Name())
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

func verifyEgressStrictPrioritySchedulerTrafficIPv6(t *testing.T, dut *ondatra.DUTDevice) {

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
			"ateTxP2-regular-nc1": {
				frameSize:             512,
				trafficRate:           1,
				dscp:                  []uint32{56, 57, 58, 59, 60, 61, 62, 63},
				expectedThroughputPct: 100.0,
				queue:                 queues.NC1,
				inputIntf:             ateTxP2,
			},
			"ateTxP1-regular-af4": {
				frameSize:             512,
				trafficRate:           30,
				expectedThroughputPct: 100.0,
				dscp:                  []uint32{32, 33, 34, 35, 36, 37, 38, 39},
				queue:                 queues.AF4,
				inputIntf:             ateTxP1,
			},
			"ateTxP2-regular-af4": {
				frameSize:             512,
				trafficRate:           30,
				dscp:                  []uint32{40, 41, 42, 43, 44, 45, 46, 47},
				expectedThroughputPct: 100.0,
				queue:                 queues.AF4,
				inputIntf:             ateTxP2,
			},
			"ateTxP1-regular-af3": {
				frameSize:             512,
				trafficRate:           12,
				expectedThroughputPct: 100.0,
				dscp:                  []uint32{24, 25, 26, 27},
				queue:                 queues.AF3,
				inputIntf:             ateTxP1,
			},
			"ateTxP2-regular-af3": {
				frameSize:             512,
				trafficRate:           12,
				dscp:                  []uint32{28, 29, 30, 31},
				expectedThroughputPct: 100.0,
				queue:                 queues.AF3,
				inputIntf:             ateTxP2,
			},
			"ateTxP1-regular-af2": {
				frameSize:             512,
				trafficRate:           10,
				expectedThroughputPct: 50.0,
				dscp:                  []uint32{16, 17, 18, 19},
				queue:                 queues.AF2,
				inputIntf:             ateTxP1,
			},
			"ateTxP2-regular-af2": {
				frameSize:             512,
				trafficRate:           10,
				dscp:                  []uint32{20, 21, 22, 23},
				expectedThroughputPct: 50.0,
				queue:                 queues.AF2,
				inputIntf:             ateTxP2,
			},
			"ateTxP1-regular-af1": {
				frameSize:             512,
				trafficRate:           12,
				expectedThroughputPct: 0.0,
				dscp:                  []uint32{8, 9, 10, 11},
				queue:                 queues.AF1,
				inputIntf:             ateTxP1,
			},
			"ateTxP2-regular-af1": {
				frameSize:             512,
				trafficRate:           12,
				dscp:                  []uint32{12, 13, 14, 15},
				expectedThroughputPct: 0.0,
				queue:                 queues.AF1,
				inputIntf:             ateTxP2,
			},
			"ateTxP1-regular-be1": {
				frameSize:             512,
				trafficRate:           12,
				expectedThroughputPct: 0.0,
				dscp:                  []uint32{0, 1, 2, 3},
				queue:                 queues.BE1,
				inputIntf:             ateTxP1,
			},
			"ateTxP2-regular-be1": {
				frameSize:             512,
				trafficRate:           12,
				expectedThroughputPct: 0.0,
				dscp:                  []uint32{4, 5, 6, 7},
				queue:                 queues.BE1,
				inputIntf:             ateTxP2,
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
		t.Logf("Running regular traffic on DUT interfaces: %s => %s \n", dp2.Name(), dp3.Name())
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

func verifyEgressStrictPrioritySchedulerTrafficMPLS(t *testing.T, dut *ondatra.DUTDevice) {

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
			"ateTxP2-regular-nc1": {
				frameSize:             512,
				trafficRate:           1,
				dscp:                  7,
				expectedThroughputPct: 100.0,
				queue:                 queues.NC1,
				inputIntf:             ateTxP2,
			},
			"ateTxP1-regular-af4": {
				frameSize:             512,
				trafficRate:           30,
				expectedThroughputPct: 100.0,
				dscp:                  4,
				queue:                 queues.AF4,
				inputIntf:             ateTxP1,
			},
			"ateTxP2-regular-af4": {
				frameSize:             512,
				trafficRate:           30,
				dscp:                  5,
				expectedThroughputPct: 100.0,
				queue:                 queues.AF4,
				inputIntf:             ateTxP2,
			},
			"ateTxP1-regular-af3": {
				frameSize:             512,
				trafficRate:           12,
				expectedThroughputPct: 100.0,
				dscp:                  3,
				queue:                 queues.AF3,
				inputIntf:             ateTxP1,
			},
			"ateTxP2-regular-af3": {
				frameSize:             512,
				trafficRate:           12,
				dscp:                  3,
				expectedThroughputPct: 100.0,
				queue:                 queues.AF3,
				inputIntf:             ateTxP2,
			},
			"ateTxP1-regular-af2": {
				frameSize:             512,
				trafficRate:           10,
				expectedThroughputPct: 50.0,
				dscp:                  2,
				queue:                 queues.AF2,
				inputIntf:             ateTxP1,
			},
			"ateTxP2-regular-af2": {
				frameSize:             512,
				trafficRate:           10,
				dscp:                  2,
				expectedThroughputPct: 50.0,
				queue:                 queues.AF2,
				inputIntf:             ateTxP2,
			},
			"ateTxP1-regular-af1": {
				frameSize:             512,
				trafficRate:           12,
				expectedThroughputPct: 0.0,
				dscp:                  1,
				queue:                 queues.AF1,
				inputIntf:             ateTxP1,
			},
			"ateTxP2-regular-af1": {
				frameSize:             512,
				trafficRate:           12,
				dscp:                  1,
				expectedThroughputPct: 0.0,
				queue:                 queues.AF1,
				inputIntf:             ateTxP2,
			},
			"ateTxP1-regular-be1": {
				frameSize:             512,
				trafficRate:           12,
				expectedThroughputPct: 0.0,
				dscp:                  0,
				queue:                 queues.BE1,
				inputIntf:             ateTxP1,
			},
			"ateTxP2-regular-be1": {
				frameSize:             512,
				trafficRate:           12,
				expectedThroughputPct: 0.0,
				dscp:                  0,
				queue:                 queues.BE1,
				inputIntf:             ateTxP2,
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
		t.Logf("Running regular traffic on DUT interfaces: %s => %s \n", dp2.Name(), dp3.Name())
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

	type trafficData struct {
		trafficRate           float64
		expectedThroughputPct float32
		frameSize             uint32
		dscp                  uint8
		queue                 string
		inputIntf             attrs.Attributes
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
		"ateTxP2-regular-nc1": {
			frameSize:             512,
			trafficRate:           1,
			dscp:                  7,
			expectedThroughputPct: 100.0,
			queue:                 queues.NC1,
			inputIntf:             ateTxP2,
		},
		"ateTxP1-regular-af4": {
			frameSize:             512,
			trafficRate:           30,
			expectedThroughputPct: 100.0,
			dscp:                  4,
			queue:                 queues.AF4,
			inputIntf:             ateTxP1,
		},
		"ateTxP2-regular-af4": {
			frameSize:             512,
			trafficRate:           30,
			dscp:                  5,
			expectedThroughputPct: 100.0,
			queue:                 queues.AF4,
			inputIntf:             ateTxP2,
		},
		"ateTxP1-regular-af3": {
			frameSize:             512,
			trafficRate:           12,
			expectedThroughputPct: 100.0,
			dscp:                  3,
			queue:                 queues.AF3,
			inputIntf:             ateTxP1,
		},
		"ateTxP2-regular-af3": {
			frameSize:             512,
			trafficRate:           12,
			dscp:                  3,
			expectedThroughputPct: 100.0,
			queue:                 queues.AF3,
			inputIntf:             ateTxP2,
		},
		"ateTxP1-regular-af2": {
			frameSize:             512,
			trafficRate:           10,
			expectedThroughputPct: 50.0,
			dscp:                  2,
			queue:                 queues.AF2,
			inputIntf:             ateTxP1,
		},
		"ateTxP2-regular-af2": {
			frameSize:             512,
			trafficRate:           10,
			dscp:                  2,
			expectedThroughputPct: 50.0,
			queue:                 queues.AF2,
			inputIntf:             ateTxP2,
		},
		"ateTxP1-regular-af1": {
			frameSize:             512,
			trafficRate:           12,
			expectedThroughputPct: 0.0,
			dscp:                  1,
			queue:                 queues.AF1,
			inputIntf:             ateTxP1,
		},
		"ateTxP2-regular-af1": {
			frameSize:             512,
			trafficRate:           12,
			dscp:                  1,
			expectedThroughputPct: 0.0,
			queue:                 queues.AF1,
			inputIntf:             ateTxP2,
		},
		"ateTxP1-regular-be1": {
			frameSize:             512,
			trafficRate:           12,
			expectedThroughputPct: 0.0,
			dscp:                  0,
			queue:                 queues.BE1,
			inputIntf:             ateTxP1,
		},
		"ateTxP2-regular-be1": {
			frameSize:             512,
			trafficRate:           12,
			expectedThroughputPct: 0.0,
			dscp:                  0,
			queue:                 queues.BE1,
			inputIntf:             ateTxP2,
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

	}
	ate.OTG().PushConfig(t, top)
}

func createTrafficFlowsIPv6(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config, dut *ondatra.DUTDevice) {
	t.Helper()

	type trafficData struct {
		trafficRate           float64
		expectedThroughputPct float32
		frameSize             uint32
		dscp                  []uint32
		queue                 string
		inputIntf             attrs.Attributes
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
		"ateTxP2-regular-nc1": {
			frameSize:             512,
			trafficRate:           1,
			dscp:                  []uint32{56, 57, 58, 59, 60, 61, 62, 63},
			expectedThroughputPct: 100.0,
			queue:                 queues.NC1,
			inputIntf:             ateTxP2,
		},
		"ateTxP1-regular-af4": {
			frameSize:             512,
			trafficRate:           30,
			expectedThroughputPct: 100.0,
			dscp:                  []uint32{32, 33, 34, 35, 36, 37, 38, 39},
			queue:                 queues.AF4,
			inputIntf:             ateTxP1,
		},
		"ateTxP2-regular-af4": {
			frameSize:             512,
			trafficRate:           30,
			dscp:                  []uint32{40, 41, 42, 43, 44, 45, 46, 47},
			expectedThroughputPct: 100.0,
			queue:                 queues.AF4,
			inputIntf:             ateTxP2,
		},
		"ateTxP1-regular-af3": {
			frameSize:             512,
			trafficRate:           12,
			expectedThroughputPct: 100.0,
			dscp:                  []uint32{24, 25, 26, 27},
			queue:                 queues.AF3,
			inputIntf:             ateTxP1,
		},
		"ateTxP2-regular-af3": {
			frameSize:             512,
			trafficRate:           12,
			dscp:                  []uint32{28, 29, 30, 31},
			expectedThroughputPct: 100.0,
			queue:                 queues.AF3,
			inputIntf:             ateTxP2,
		},
		"ateTxP1-regular-af2": {
			frameSize:             512,
			trafficRate:           10,
			expectedThroughputPct: 50.0,
			dscp:                  []uint32{16, 17, 18, 19},
			queue:                 queues.AF2,
			inputIntf:             ateTxP1,
		},
		"ateTxP2-regular-af2": {
			frameSize:             512,
			trafficRate:           10,
			dscp:                  []uint32{20, 21, 22, 23},
			expectedThroughputPct: 50.0,
			queue:                 queues.AF2,
			inputIntf:             ateTxP2,
		},
		"ateTxP1-regular-af1": {
			frameSize:             512,
			trafficRate:           12,
			expectedThroughputPct: 0.0,
			dscp:                  []uint32{8, 9, 10, 11},
			queue:                 queues.AF1,
			inputIntf:             ateTxP1,
		},
		"ateTxP2-regular-af1": {
			frameSize:             512,
			trafficRate:           12,
			dscp:                  []uint32{12, 13, 14, 15},
			expectedThroughputPct: 0.0,
			queue:                 queues.AF1,
			inputIntf:             ateTxP2,
		},
		"ateTxP1-regular-be1": {
			frameSize:             512,
			trafficRate:           12,
			expectedThroughputPct: 0.0,
			dscp:                  []uint32{0, 1, 2, 3},
			queue:                 queues.BE1,
			inputIntf:             ateTxP1,
		},
		"ateTxP2-regular-be1": {
			frameSize:             512,
			trafficRate:           12,
			expectedThroughputPct: 0.0,
			dscp:                  []uint32{4, 5, 6, 7},
			queue:                 queues.BE1,
			inputIntf:             ateTxP2,
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

	}
	ate.OTG().PushConfig(t, top)
}

func createTrafficFlowsMPLS(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config, dut *ondatra.DUTDevice) {
	t.Helper()

	type trafficData struct {
		trafficRate           float64
		expectedThroughputPct float32
		frameSize             uint32
		exp                   uint8
		queue                 string
		inputIntf             attrs.Attributes
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
		"ateTxP2-regular-nc1": {
			frameSize:             512,
			trafficRate:           1,
			exp:                   7,
			expectedThroughputPct: 100.0,
			queue:                 queues.NC1,
			inputIntf:             ateTxP2,
		},
		"ateTxP1-regular-af4": {
			frameSize:             512,
			trafficRate:           30,
			expectedThroughputPct: 100.0,
			exp:                   4,
			queue:                 queues.AF4,
			inputIntf:             ateTxP1,
		},
		"ateTxP2-regular-af4": {
			frameSize:             512,
			trafficRate:           30,
			exp:                   5,
			expectedThroughputPct: 100.0,
			queue:                 queues.AF4,
			inputIntf:             ateTxP2,
		},
		"ateTxP1-regular-af3": {
			frameSize:             512,
			trafficRate:           12,
			expectedThroughputPct: 100.0,
			exp:                   3,
			queue:                 queues.AF3,
			inputIntf:             ateTxP1,
		},
		"ateTxP2-regular-af3": {
			frameSize:             512,
			trafficRate:           12,
			exp:                   3,
			expectedThroughputPct: 100.0,
			queue:                 queues.AF3,
			inputIntf:             ateTxP2,
		},
		"ateTxP1-regular-af2": {
			frameSize:             512,
			trafficRate:           10,
			expectedThroughputPct: 50.0,
			exp:                   2,
			queue:                 queues.AF2,
			inputIntf:             ateTxP1,
		},
		"ateTxP2-regular-af2": {
			frameSize:             512,
			trafficRate:           10,
			exp:                   2,
			expectedThroughputPct: 50.0,
			queue:                 queues.AF2,
			inputIntf:             ateTxP2,
		},
		"ateTxP1-regular-af1": {
			frameSize:             512,
			trafficRate:           12,
			expectedThroughputPct: 0.0,
			exp:                   1,
			queue:                 queues.AF1,
			inputIntf:             ateTxP1,
		},
		"ateTxP2-regular-af1": {
			frameSize:             512,
			trafficRate:           12,
			exp:                   1,
			expectedThroughputPct: 0.0,
			queue:                 queues.AF1,
			inputIntf:             ateTxP2,
		},
		"ateTxP1-regular-be1": {
			frameSize:             512,
			trafficRate:           12,
			expectedThroughputPct: 0.0,
			exp:                   0,
			queue:                 queues.BE1,
			inputIntf:             ateTxP1,
		},
		"ateTxP2-regular-be1": {
			frameSize:             512,
			trafficRate:           12,
			expectedThroughputPct: 0.0,
			exp:                   0,
			queue:                 queues.BE1,
			inputIntf:             ateTxP2,
		},
	}
	top.Flows().Clear()

	dstMac1 := gnmi.Get(t, dut, gnmi.OC().Interface(dut.Port(t, "port1").Name()).Ethernet().MacAddress().State())
	dstMac2 := gnmi.Get(t, dut, gnmi.OC().Interface(dut.Port(t, "port2").Name()).Ethernet().MacAddress().State())

	for trafficID, data := range trafficFlows {
		t.Logf("Configuring flow %s\n", trafficID)

		dstMac := ""
		srcPort := ""

		if strings.Contains(strings.ToUpper(trafficID), "ATETXP1") {
			dstMac = dstMac1
			srcPort = "port1"
		} else if strings.Contains(strings.ToUpper(trafficID), "ATETXP2") {
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
	d := &oc.Root{}
	q := d.GetOrCreateQos()
	queues := netutil.CommonTrafficQueues(t, dut)

	cfgplugins.NewQoSQueue(t, dut, q)

	t.Logf("Create QoS forwarding groups and queue names configuration")
	forwardingGroups := []cfgplugins.ForwardingGroup{
		{
			Desc:        "forwarding-group-BE1",
			QueueName:   queues.BE1,
			TargetGroup: "target-group-BE1",
			Priority:    0,
		}, {
			Desc:        "forwarding-group-AF1",
			QueueName:   queues.AF1,
			TargetGroup: "target-group-AF1",
			Priority:    1,
		}, {
			Desc:        "forwarding-group-AF2",
			QueueName:   queues.AF2,
			TargetGroup: "target-group-AF2",
			Priority:    2,
		}, {
			Desc:        "forwarding-group-AF3",
			QueueName:   queues.AF3,
			TargetGroup: "target-group-AF3",
			Priority:    3,
		}, {
			Desc:        "forwarding-group-AF4",
			QueueName:   queues.AF4,
			TargetGroup: "target-group-AF4",
			Priority:    4,
		}, {
			Desc:        "forwarding-group-NC1",
			QueueName:   queues.NC1,
			TargetGroup: "target-group-NC1",
			Priority:    5,
		}}

	cfgplugins.NewQoSForwardingGroup(t, dut, q, forwardingGroups)

	t.Logf("Create QoS Classifiers config")
	classifiers := []cfgplugins.QosClassifier{
		{
			Desc:        "classifier_ipv4_be1",
			Name:        "dscp_based_classifier_ipv4",
			ClassType:   oc.Qos_Classifier_Type_IPV4,
			TermID:      "0",
			TargetGroup: "target-group-BE1",
			DscpSet:     []uint8{0},
		}, {
			Desc:        "classifier_ipv4_af1",
			Name:        "dscp_based_classifier_ipv4",
			ClassType:   oc.Qos_Classifier_Type_IPV4,
			TermID:      "1",
			TargetGroup: "target-group-AF1",
			DscpSet:     []uint8{1},
		}, {
			Desc:        "classifier_ipv4_af2",
			Name:        "dscp_based_classifier_ipv4",
			ClassType:   oc.Qos_Classifier_Type_IPV4,
			TermID:      "2",
			TargetGroup: "target-group-AF2",
			DscpSet:     []uint8{2},
		}, {
			Desc:        "classifier_ipv4_af3",
			Name:        "dscp_based_classifier_ipv4",
			ClassType:   oc.Qos_Classifier_Type_IPV4,
			TermID:      "3",
			TargetGroup: "target-group-AF3",
			DscpSet:     []uint8{3},
		}, {
			Desc:        "classifier_ipv4_af4",
			Name:        "dscp_based_classifier_ipv4",
			ClassType:   oc.Qos_Classifier_Type_IPV4,
			TermID:      "4",
			TargetGroup: "target-group-AF4",
			DscpSet:     []uint8{4, 5},
		}, {
			Desc:        "classifier_ipv4_nc1",
			Name:        "dscp_based_classifier_ipv4",
			ClassType:   oc.Qos_Classifier_Type_IPV4,
			TermID:      "5",
			TargetGroup: "target-group-NC1",
			DscpSet:     []uint8{6, 7},
		}}

	q = cfgplugins.NewQoSClassifierConfiguration(t, dut, q, classifiers)

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
	schedulerPolicies := []cfgplugins.SchedulerPolicy{
		{
			Desc:        "scheduler-policy-BE1",
			Sequence:    uint32(6),
			SetPriority: true,
			SetWeight:   false,
			Priority:    oc.Scheduler_Priority_STRICT,
			InputID:     "BE1",
			InputType:   oc.Input_InputType_QUEUE,
			QueueName:   queues.BE1,
			TargetGroup: "target-group-BE1",
		}, {
			Desc:        "scheduler-policy-AF1",
			Sequence:    uint32(5),
			SetPriority: true,
			SetWeight:   false,
			Priority:    oc.Scheduler_Priority_STRICT,
			InputID:     "AF1",
			InputType:   oc.Input_InputType_QUEUE,
			QueueName:   queues.AF1,
			TargetGroup: "target-group-AF1",
		}, {
			Desc:        "scheduler-policy-AF2",
			Sequence:    uint32(4),
			SetPriority: true,
			SetWeight:   false,
			Priority:    oc.Scheduler_Priority_STRICT,
			InputID:     "AF2",
			InputType:   oc.Input_InputType_QUEUE,
			QueueName:   queues.AF2,
			TargetGroup: "target-group-AF2",
		}, {
			Desc:        "scheduler-policy-AF3",
			Sequence:    uint32(3),
			SetPriority: true,
			SetWeight:   false,
			Priority:    oc.Scheduler_Priority_STRICT,
			InputID:     "AF3",
			InputType:   oc.Input_InputType_QUEUE,
			QueueName:   queues.AF3,
			TargetGroup: "target-group-AF3",
		}, {
			Desc:        "scheduler-policy-AF4",
			Sequence:    uint32(2),
			SetPriority: true,
			SetWeight:   false,
			Priority:    oc.Scheduler_Priority_STRICT,
			InputID:     "AF4",
			InputType:   oc.Input_InputType_QUEUE,
			QueueName:   queues.AF4,
			TargetGroup: "target-group-AF4",
		}, {
			Desc:        "scheduler-policy-NC1",
			Sequence:    uint32(1),
			SetPriority: true,
			SetWeight:   false,
			Priority:    oc.Scheduler_Priority_STRICT,
			InputID:     "NC1",
			InputType:   oc.Input_InputType_QUEUE,
			QueueName:   queues.NC1,
			TargetGroup: "target-group-NC1",
		}}

	q = cfgplugins.NewQoSSchedulerPolicy(t, dut, q, schedulerPolicies)

	t.Logf("Create QoS output interface config")
	schedulerIntfs := []cfgplugins.QoSSchedulerInterface{
		{
			Desc:      "output-interface-BE1",
			QueueName: queues.BE1,
			Scheduler: "scheduler",
		}, {
			Desc:      "output-interface-AF1",
			QueueName: queues.AF1,
			Scheduler: "scheduler",
		}, {
			Desc:      "output-interface-AF2",
			QueueName: queues.AF2,
			Scheduler: "scheduler",
		}, {
			Desc:      "output-interface-AF3",
			QueueName: queues.AF3,
			Scheduler: "scheduler",
		}, {
			Desc:      "output-interface-AF4",
			QueueName: queues.AF4,
			Scheduler: "scheduler",
		}, {
			Desc:      "output-interface-NC1",
			QueueName: queues.NC1,
			Scheduler: "scheduler",
		}}

	q = cfgplugins.NewQoSSchedulerInterface(t, dut, q, schedulerIntfs, "port3")
	gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), q)

}

func ConfigureDUTQoSIPv6(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")
	d := &oc.Root{}
	q := d.GetOrCreateQos()
	queues := netutil.CommonTrafficQueues(t, dut)

	cfgplugins.NewQoSQueue(t, dut, q)

	t.Logf("Create QoS forwarding groups and queue names configuration")
	forwardingGroups := []cfgplugins.ForwardingGroup{
		{
			Desc:        "forwarding-group-BE1",
			QueueName:   queues.BE1,
			TargetGroup: "target-group-BE1",
			Priority:    0,
		}, {
			Desc:        "forwarding-group-AF1",
			QueueName:   queues.AF1,
			TargetGroup: "target-group-AF1",
			Priority:    1,
		}, {
			Desc:        "forwarding-group-AF2",
			QueueName:   queues.AF2,
			TargetGroup: "target-group-AF2",
			Priority:    2,
		}, {
			Desc:        "forwarding-group-AF3",
			QueueName:   queues.AF3,
			TargetGroup: "target-group-AF3",
			Priority:    3,
		}, {
			Desc:        "forwarding-group-AF4",
			QueueName:   queues.AF4,
			TargetGroup: "target-group-AF4",
			Priority:    4,
		}, {
			Desc:        "forwarding-group-NC1",
			QueueName:   queues.NC1,
			TargetGroup: "target-group-NC1",
			Priority:    5,
		}}

	cfgplugins.NewQoSForwardingGroup(t, dut, q, forwardingGroups)

	t.Logf("Create QoS Classifiers config")
	classifiers := []cfgplugins.QosClassifier{
		{
			Desc:        "classifier_ipv6_be1",
			Name:        "dscp_based_classifier_ipv6",
			ClassType:   oc.Qos_Classifier_Type_IPV6,
			TermID:      "0",
			TargetGroup: "target-group-BE1",
			DscpSet:     []uint8{0, 1, 2, 3, 4, 5, 6, 7},
		}, {
			Desc:        "classifier_ipv6_af1",
			Name:        "dscp_based_classifier_ipv6",
			ClassType:   oc.Qos_Classifier_Type_IPV6,
			TermID:      "1",
			TargetGroup: "target-group-AF1",
			DscpSet:     []uint8{8, 9, 10, 11, 12, 13, 14, 15},
		}, {
			Desc:        "classifier_ipv6_af2",
			Name:        "dscp_based_classifier_ipv6",
			ClassType:   oc.Qos_Classifier_Type_IPV6,
			TermID:      "2",
			TargetGroup: "target-group-AF2",
			DscpSet:     []uint8{16, 17, 18, 19, 20, 21, 22, 23},
		}, {
			Desc:        "classifier_ipv6_af3",
			Name:        "dscp_based_classifier_ipv6",
			ClassType:   oc.Qos_Classifier_Type_IPV6,
			TermID:      "3",
			TargetGroup: "target-group-AF3",
			DscpSet:     []uint8{24, 25, 26, 27, 28, 29, 30, 31},
		}, {
			Desc:        "classifier_ipv6_af4",
			Name:        "dscp_based_classifier_ipv6",
			ClassType:   oc.Qos_Classifier_Type_IPV6,
			TermID:      "4",
			TargetGroup: "target-group-AF4",
			DscpSet:     []uint8{32, 33, 34, 35, 36, 37, 38, 39, 40, 41, 42, 43, 44, 45, 46, 47},
		}, {
			Desc:        "classifier_ipv6_nc1",
			Name:        "dscp_based_classifier_ipv6",
			ClassType:   oc.Qos_Classifier_Type_IPV6,
			TermID:      "5",
			TargetGroup: "target-group-NC1",
			DscpSet:     []uint8{48, 49, 50, 51, 52, 53, 54, 55, 56, 57, 58, 59, 60, 61, 62, 63},
		}}

	q = cfgplugins.NewQoSClassifierConfiguration(t, dut, q, classifiers)

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
	schedulerPolicies := []cfgplugins.SchedulerPolicy{
		{
			Desc:        "scheduler-policy-BE1",
			Sequence:    uint32(0),
			SetPriority: true,
			SetWeight:   false,
			Priority:    oc.Scheduler_Priority_STRICT,
			InputID:     "BE1",
			InputType:   oc.Input_InputType_QUEUE,
			QueueName:   queues.BE1,
			TargetGroup: "target-group-BE1",
		}, {
			Desc:        "scheduler-policy-AF1",
			Sequence:    uint32(0),
			SetPriority: true,
			SetWeight:   false,
			Priority:    oc.Scheduler_Priority_STRICT,
			InputID:     "AF1",
			InputType:   oc.Input_InputType_QUEUE,
			QueueName:   queues.AF1,
			TargetGroup: "target-group-AF1",
		}, {
			Desc:        "scheduler-policy-AF2",
			Sequence:    uint32(0),
			SetPriority: true,
			SetWeight:   false,
			Priority:    oc.Scheduler_Priority_STRICT,
			InputID:     "AF2",
			InputType:   oc.Input_InputType_QUEUE,
			QueueName:   queues.AF2,
			TargetGroup: "target-group-AF2",
		}, {
			Desc:        "scheduler-policy-AF3",
			Sequence:    uint32(0),
			SetPriority: true,
			SetWeight:   false,
			Priority:    oc.Scheduler_Priority_STRICT,
			InputID:     "AF3",
			InputType:   oc.Input_InputType_QUEUE,
			QueueName:   queues.AF3,
			TargetGroup: "target-group-AF3",
		}, {
			Desc:        "scheduler-policy-AF4",
			Sequence:    uint32(0),
			SetPriority: true,
			SetWeight:   false,
			Priority:    oc.Scheduler_Priority_STRICT,
			InputID:     "AF4",
			InputType:   oc.Input_InputType_QUEUE,
			QueueName:   queues.AF4,
			TargetGroup: "target-group-AF4",
		}, {
			Desc:        "scheduler-policy-NC1",
			Sequence:    uint32(0),
			SetPriority: true,
			SetWeight:   false,
			Priority:    oc.Scheduler_Priority_STRICT,
			InputID:     "NC1",
			InputType:   oc.Input_InputType_QUEUE,
			QueueName:   queues.NC1,
			TargetGroup: "target-group-NC1",
		}}

	q = cfgplugins.NewQoSSchedulerPolicy(t, dut, q, schedulerPolicies)

	t.Logf("Create QoS output interface config")
	schedulerIntfs := []cfgplugins.QoSSchedulerInterface{
		{
			Desc:      "output-interface-BE1",
			QueueName: queues.BE1,
			Scheduler: "scheduler",
		}, {
			Desc:      "output-interface-AF1",
			QueueName: queues.AF1,
			Scheduler: "scheduler",
		}, {
			Desc:      "output-interface-AF2",
			QueueName: queues.AF2,
			Scheduler: "scheduler",
		}, {
			Desc:      "output-interface-AF3",
			QueueName: queues.AF3,
			Scheduler: "scheduler",
		}, {
			Desc:      "output-interface-AF4",
			QueueName: queues.AF4,
			Scheduler: "scheduler",
		}, {
			Desc:      "output-interface-NC1",
			QueueName: queues.NC1,
			Scheduler: "scheduler",
		}}

	q = cfgplugins.NewQoSSchedulerInterface(t, dut, q, schedulerIntfs, "port3")
	gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), q)

}

func ConfigureDUTQoSMPLS(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")
	d := &oc.Root{}
	q := d.GetOrCreateQos()
	queues := netutil.CommonTrafficQueues(t, dut)

	cfgplugins.NewQoSQueue(t, dut, q)

	t.Logf("Create QoS forwarding groups and queue names configuration")
	forwardingGroups := []cfgplugins.ForwardingGroup{
		{
			Desc:        "forwarding-group-BE1",
			QueueName:   queues.BE1,
			TargetGroup: "target-group-BE1",
			Priority:    0,
		}, {
			Desc:        "forwarding-group-AF1",
			QueueName:   queues.AF1,
			TargetGroup: "target-group-AF1",
			Priority:    1,
		}, {
			Desc:        "forwarding-group-AF2",
			QueueName:   queues.AF2,
			TargetGroup: "target-group-AF2",
			Priority:    2,
		}, {
			Desc:        "forwarding-group-AF3",
			QueueName:   queues.AF3,
			TargetGroup: "target-group-AF3",
			Priority:    3,
		}, {
			Desc:        "forwarding-group-AF4",
			QueueName:   queues.AF4,
			TargetGroup: "target-group-AF4",
			Priority:    4,
		}, {
			Desc:        "forwarding-group-NC1",
			QueueName:   queues.NC1,
			TargetGroup: "target-group-NC1",
			Priority:    5,
		}}

	cfgplugins.NewQoSForwardingGroup(t, dut, q, forwardingGroups)

	t.Logf("Create QoS Classifiers config")
	classifiers := []cfgplugins.QosClassifier{
		{
			Desc:        "classifier_mpls_be1",
			Name:        "dscp_based_classifier_mpls",
			ClassType:   oc.Qos_Classifier_Type_MPLS,
			TermID:      "0",
			TargetGroup: "target-group-BE1",
			DscpSet:     []uint8{0},
		}, {
			Desc:        "classifier_mpls_af1",
			Name:        "dscp_based_classifier_mpls",
			ClassType:   oc.Qos_Classifier_Type_MPLS,
			TermID:      "1",
			TargetGroup: "target-group-AF1",
			DscpSet:     []uint8{1},
		}, {
			Desc:        "classifier_mpls_af2",
			Name:        "dscp_based_classifier_mpls",
			ClassType:   oc.Qos_Classifier_Type_MPLS,
			TermID:      "2",
			TargetGroup: "target-group-AF2",
			DscpSet:     []uint8{2},
		}, {
			Desc:        "classifier_mpls_af3",
			Name:        "dscp_based_classifier_mpls",
			ClassType:   oc.Qos_Classifier_Type_MPLS,
			TermID:      "3",
			TargetGroup: "target-group-AF3",
			DscpSet:     []uint8{3},
		}, {
			Desc:        "classifier_mpls_af4",
			Name:        "dscp_based_classifier_mpls",
			ClassType:   oc.Qos_Classifier_Type_MPLS,
			TermID:      "4",
			TargetGroup: "target-group-AF4",
			DscpSet:     []uint8{4, 5},
		}, {
			Desc:        "classifier_mpls_nc1",
			Name:        "dscp_based_classifier_mpls",
			ClassType:   oc.Qos_Classifier_Type_MPLS,
			TermID:      "5",
			TargetGroup: "target-group-NC1",
			DscpSet:     []uint8{6, 7},
		}}

	t.Logf("QoS classifiers config: %v", classifiers)
	if deviations.MplsExpIngressClassifierOcUnsupported(dut) {
		configureMplsExpClassifierCLI(t, dut, classifiers)
	} else {

		q = cfgplugins.NewQoSClassifierConfiguration(t, dut, q, classifiers)

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
	schedulerPolicies := []cfgplugins.SchedulerPolicy{
		{
			Desc:        "scheduler-policy-BE1",
			Sequence:    uint32(0),
			SetPriority: true,
			SetWeight:   false,
			Priority:    oc.Scheduler_Priority_STRICT,
			InputID:     "BE1",
			InputType:   oc.Input_InputType_QUEUE,
			QueueName:   queues.BE1,
			TargetGroup: "target-group-BE1",
		}, {
			Desc:        "scheduler-policy-AF1",
			Sequence:    uint32(0),
			SetPriority: true,
			SetWeight:   false,
			Priority:    oc.Scheduler_Priority_STRICT,
			InputID:     "AF1",
			InputType:   oc.Input_InputType_QUEUE,
			QueueName:   queues.AF1,
			TargetGroup: "target-group-AF1",
		}, {
			Desc:        "scheduler-policy-AF2",
			Sequence:    uint32(0),
			SetPriority: true,
			SetWeight:   false,
			Priority:    oc.Scheduler_Priority_STRICT,
			InputID:     "AF2",
			InputType:   oc.Input_InputType_QUEUE,
			QueueName:   queues.AF2,
			TargetGroup: "target-group-AF2",
		}, {
			Desc:        "scheduler-policy-AF3",
			Sequence:    uint32(0),
			SetPriority: true,
			SetWeight:   false,
			Priority:    oc.Scheduler_Priority_STRICT,
			InputID:     "AF3",
			InputType:   oc.Input_InputType_QUEUE,
			QueueName:   queues.AF3,
			TargetGroup: "target-group-AF3",
		}, {
			Desc:        "scheduler-policy-AF4",
			Sequence:    uint32(0),
			SetPriority: true,
			SetWeight:   false,
			Priority:    oc.Scheduler_Priority_STRICT,
			InputID:     "AF4",
			InputType:   oc.Input_InputType_QUEUE,
			QueueName:   queues.AF4,
			TargetGroup: "target-group-AF4",
		}, {
			Desc:        "scheduler-policy-NC1",
			Sequence:    uint32(0),
			SetPriority: true,
			SetWeight:   false,
			Priority:    oc.Scheduler_Priority_STRICT,
			InputID:     "NC1",
			InputType:   oc.Input_InputType_QUEUE,
			QueueName:   queues.NC1,
			TargetGroup: "target-group-NC1",
		}}

	q = cfgplugins.NewQoSSchedulerPolicy(t, dut, q, schedulerPolicies)

	t.Logf("Create QoS output interface config")
	schedulerIntfs := []cfgplugins.QoSSchedulerInterface{
		{
			Desc:      "output-interface-BE1",
			QueueName: queues.BE1,
			Scheduler: "scheduler",
		}, {
			Desc:      "output-interface-AF1",
			QueueName: queues.AF1,
			Scheduler: "scheduler",
		}, {
			Desc:      "output-interface-AF2",
			QueueName: queues.AF2,
			Scheduler: "scheduler",
		}, {
			Desc:      "output-interface-AF3",
			QueueName: queues.AF3,
			Scheduler: "scheduler",
		}, {
			Desc:      "output-interface-AF4",
			QueueName: queues.AF4,
			Scheduler: "scheduler",
		}, {
			Desc:      "output-interface-NC1",
			QueueName: queues.NC1,
			Scheduler: "scheduler",
		}}

	q = cfgplugins.NewQoSSchedulerInterface(t, dut, q, schedulerIntfs, "port3")
	gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), q)

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

func configureMplsExpClassifierCLI(t *testing.T, dut *ondatra.DUTDevice, classifiers []cfgplugins.QosClassifier) {

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
		tc := classifier.TermID
		for _, exp := range classifier.DscpSet {
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
