// Copyright 2024 Google LLC
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
	"math"
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
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

var (
	intf1 = attrs.Attributes{
		Name:    "ateSrc1",
		IPv4:    "198.51.100.1",
		IPv6:    "2001:db8::1",
		MAC:     "02:00:01:01:01:01",
		IPv4Len: 31,
		IPv6Len: 126,
	}
	intf2 = attrs.Attributes{
		Name:    "ateSrc2",
		IPv4:    "198.51.100.3",
		IPv6:    "2001:db8::3",
		MAC:     "02:00:01:01:01:02",
		IPv4Len: 31,
		IPv6Len: 126,
	}
	intf3 = attrs.Attributes{
		Name:    "ateDst1",
		IPv4:    "198.51.100.5",
		IPv6:    "2001:db8::5",
		MAC:     "02:00:01:01:01:03",
		IPv4Len: 31,
		IPv6Len: 126,
	}
	dutPort1 = attrs.Attributes{
		Desc:    "DUT to ATE source",
		IPv4:    "198.51.100.2",
		IPv6:    "2001:db8::2",
		IPv4Len: 31,
		IPv6Len: 126,
	}
	dutPort2 = attrs.Attributes{
		Desc:    "DUT to ATE source",
		IPv4:    "198.51.100.4",
		IPv6:    "2001:db8::4",
		IPv4Len: 31,
		IPv6Len: 126,
	}
	dutPort3 = attrs.Attributes{
		Desc:    "DUT to ATE destination",
		IPv4:    "198.51.100.6",
		IPv6:    "2001:db8::8",
		IPv4Len: 31,
		IPv6Len: 126,
	}
)

const (
	mplsLabel1 = 1000001
	mplsLabel2 = 1000002
	mplsLabel3 = 1000003
	// tolerance = 0.01 // 1% Traffic Tolerance
)

type trafficData struct {
	trafficRate           float64
	expectedThroughputPct float32
	frameSize             uint32
	dscp                  uint8
	queue                 string
	inputIntf             attrs.Attributes
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Test cases:
//  1) Egress Strict Priority scheduler for IPv4 Traffic.
//     - Loss Rate: Capture packet loss for every generated flow and verify that loss for each flow does not exceed expected loss.
//	   - Telemetry: Utilize OpenConfig telemetry parameters to validate that per queue dropped packets statistics corresponds (with error margin) to the packet loss reported for every flow matching that particular queue.
//  2) Egress Strict Priority scheduler for IPv6 Traffic.
//     - Loss Rate: Capture packet loss for every generated flow and verify that loss for each flow does not exceed expected loss.
//	   - Telemetry: Utilize OpenConfig telemetry parameters to validate that per queue dropped packets statistics corresponds (with error margin) to the packet loss reported for every flow matching that particular queue.
//  3) Egress Strict Priority scheduler for MPLS Traffic.
//     - Loss Rate: Capture packet loss for every generated flow and verify that loss for each flow does not exceed expected loss.
//	   - Telemetry: Utilize OpenConfig telemetry parameters to validate that per queue dropped packets statistics corresponds (with error margin) to the packet loss reported for every flow matching that particular queue.
//
//  Details: https://github.com/openconfig/featureprofiles/blob/main/feature/qos/ate_tests/egress_strict_priority_scheduler_test/README.md
//
// Topology:
//       ATE port 3
//        |
//       DUT--------ATE port 1
//        |
//       ATE port 2
//
//  Sample CLI command to get telemetry using gmic:
//   - gnmic -a ipaddr:10162 -u username -p password --skip-verify get \
//      --path /components/component --format flat
//   - gnmic tool info:
//     - https://github.com/karimra/gnmic/blob/main/README.md
//

func TestEgressStrictPriorityScheduler(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")
	dp3 := dut.Port(t, "port3")

	// Configure DUT interfaces.
	ConfigureDUTIntf(t, dut)
	configureStaticLSP(t, dut, "lsp1", mplsLabel1, intf1.IPv4)
	configureStaticLSP(t, dut, "lsp2", mplsLabel2, intf2.IPv4)
	configureStaticLSP(t, dut, "lsp3", mplsLabel3, intf3.IPv4)
	if dut.Vendor() == ondatra.CISCO {
		ConfigureCiscoQos(t, dut)
	} else {
		ConfigureQoS(t, dut)
	}

	// Configure ATE interfaces.
	ate := ondatra.ATE(t, "ate")
	ap1 := ate.Port(t, "port1")
	ap2 := ate.Port(t, "port2")
	ap3 := ate.Port(t, "port3")
	top := gosnappi.NewConfig()

	intf1.AddToOTG(top, ap1, &dutPort1)
	intf2.AddToOTG(top, ap2, &dutPort2)
	intf3.AddToOTG(top, ap3, &dutPort3)
	ate.OTG().PushConfig(t, top)

	var tolerance float32 = 2.0 // Suppose you expect the packet loss to be exactly 2.0% based on flow data. However, OpenConfig telemetry reports 2.1%. This 0.1% difference could be due to system noise or measurement delays. Setting a tolerance of 0.5% would accept both values as "correct" since the difference is within the acceptable range.
	queues := netutil.CommonTrafficQueues(t, dut)

	// Test case 1: Egress Strict Priority scheduler for IPv4 Traffic.
	//   - Loss Rate: Capture packet loss for every generated flow and verify that loss for each flow does not exceed expected loss.
	//	 - Telemetry: Utilize OpenConfig telemetry parameters to validate that per queue dropped packets statistics corresponds (with error margin) to the packet loss reported for every flow matching that particular queue.

	EgressIPv4TrafficFlows := map[string]*trafficData{
		"intf1-nc1": {
			frameSize:             512,
			trafficRate:           1,
			expectedThroughputPct: 0,
			dscp:                  1,
			queue:                 queues.NC1,
			inputIntf:             intf1,
		},
		"intf1-af4": {
			frameSize:             512,
			trafficRate:           30,
			expectedThroughputPct: 0,
			dscp:                  2,
			queue:                 queues.AF4,
			inputIntf:             intf1,
		},
		"intf1-af3": {
			frameSize:             512,
			trafficRate:           12,
			expectedThroughputPct: 0,
			dscp:                  3,
			queue:                 queues.AF3,
			inputIntf:             intf1,
		},
		"intf1-af2": {
			frameSize:             512,
			trafficRate:           10,
			expectedThroughputPct: 50.0,
			dscp:                  4,
			queue:                 queues.AF2,
			inputIntf:             intf1,
		},
		"intf1-af1": {
			frameSize:             512,
			trafficRate:           12,
			expectedThroughputPct: 100.0,
			dscp:                  5,
			queue:                 queues.AF1,
			inputIntf:             intf1,
		},
		"intf1-be1": {
			frameSize:             512,
			trafficRate:           12,
			expectedThroughputPct: 100.0,
			dscp:                  6,
			queue:                 queues.BE1,
			inputIntf:             intf1,
		},
		"intf2-nc1": {
			frameSize:             512,
			trafficRate:           1,
			dscp:                  1,
			expectedThroughputPct: 0,
			queue:                 queues.NC1,
			inputIntf:             intf2,
		},
		"intf2-af4": {
			frameSize:             512,
			trafficRate:           30,
			dscp:                  2,
			expectedThroughputPct: 0,
			queue:                 queues.AF4,
			inputIntf:             intf2,
		},
		"intf2-af3": {
			frameSize:             512,
			trafficRate:           12,
			expectedThroughputPct: 0,
			dscp:                  3,
			queue:                 queues.AF3,
			inputIntf:             intf2,
		},
		"intf2-af2": {
			frameSize:             512,
			trafficRate:           10,
			expectedThroughputPct: 50.0,
			dscp:                  4,
			queue:                 queues.AF2,
			inputIntf:             intf2,
		},
		"intf2-af1": {
			frameSize:             512,
			trafficRate:           12,
			expectedThroughputPct: 100.0,
			dscp:                  5,
			queue:                 queues.AF1,
			inputIntf:             intf2,
		},
		"intf2-be1": {
			frameSize:             512,
			trafficRate:           12,
			dscp:                  6,
			expectedThroughputPct: 100.0,
			queue:                 queues.BE1,
			inputIntf:             intf2,
		},
	}

	// Test case 2: Egress Strict Priority scheduler for IPv6 Traffic.
	//   - Loss Rate: Capture packet loss for every generated flow and verify that loss for each flow does not exceed expected loss.
	//	 - Telemetry: Utilize OpenConfig telemetry parameters to validate that per queue dropped packets statistics corresponds (with error margin) to the packet loss reported for every flow matching that particular queue.

	EgressIPv6TrafficFlows := map[string]*trafficData{
		"intf1-nc1": {
			frameSize:             512,
			trafficRate:           1,
			expectedThroughputPct: 0,
			dscp:                  1,
			queue:                 queues.NC1,
			inputIntf:             intf1,
		},
		"intf1-af4": {
			frameSize:             512,
			trafficRate:           30,
			expectedThroughputPct: 0,
			dscp:                  2,
			queue:                 queues.AF4,
			inputIntf:             intf1,
		},
		"intf1-af3": {
			frameSize:             512,
			trafficRate:           12,
			expectedThroughputPct: 0,
			dscp:                  3,
			queue:                 queues.AF3,
			inputIntf:             intf1,
		},
		"intf1-af2": {
			frameSize:             512,
			trafficRate:           10,
			expectedThroughputPct: 50.0,
			dscp:                  4,
			queue:                 queues.AF2,
			inputIntf:             intf1,
		},
		"intf1-af1": {
			frameSize:             512,
			trafficRate:           12,
			expectedThroughputPct: 100.0,
			dscp:                  5,
			queue:                 queues.AF1,
			inputIntf:             intf1,
		},
		"intf1-be1": {
			frameSize:             512,
			trafficRate:           12,
			expectedThroughputPct: 100.0,
			dscp:                  6,
			queue:                 queues.BE1,
			inputIntf:             intf1,
		},
		"intf2-nc1": {
			frameSize:             512,
			trafficRate:           1,
			dscp:                  1,
			expectedThroughputPct: 0,
			queue:                 queues.NC1,
			inputIntf:             intf2,
		},
		"intf2-af4": {
			frameSize:             512,
			trafficRate:           30,
			dscp:                  2,
			expectedThroughputPct: 0,
			queue:                 queues.AF4,
			inputIntf:             intf2,
		},
		"intf2-af3": {
			frameSize:             512,
			trafficRate:           12,
			expectedThroughputPct: 0,
			dscp:                  3,
			queue:                 queues.AF3,
			inputIntf:             intf2,
		},
		"intf2-af2": {
			frameSize:             512,
			trafficRate:           10,
			expectedThroughputPct: 50.0,
			dscp:                  4,
			queue:                 queues.AF2,
			inputIntf:             intf2,
		},
		"intf2-af1": {
			frameSize:             512,
			trafficRate:           12,
			expectedThroughputPct: 100.0,
			dscp:                  5,
			queue:                 queues.AF1,
			inputIntf:             intf2,
		},
		"intf2-be1": {
			frameSize:             512,
			trafficRate:           12,
			dscp:                  6,
			expectedThroughputPct: 100.0,
			queue:                 queues.BE1,
			inputIntf:             intf2,
		},
	}

	// Test case 3: Egress Strict Priority scheduler for MPLS Traffic.
	//   - Loss Rate: Capture packet loss for every generated flow and verify that loss for each flow does not exceed expected loss.
	//	 - Telemetry: Utilize OpenConfig telemetry parameters to validate that per queue dropped packets statistics corresponds (with error margin) to the packet loss reported for every flow matching that particular queue.

	EgressMplsTrafficFlows := map[string]*trafficData{
		"intf1-nc1": {
			frameSize:             512,
			trafficRate:           1,
			expectedThroughputPct: 0,
			dscp:                  1,
			queue:                 queues.NC1,
			inputIntf:             intf1,
		},
		"intf1-af4": {
			frameSize:             512,
			trafficRate:           30,
			expectedThroughputPct: 0,
			dscp:                  2,
			queue:                 queues.AF4,
			inputIntf:             intf1,
		},
		"intf1-af3": {
			frameSize:             512,
			trafficRate:           12,
			expectedThroughputPct: 0,
			dscp:                  3,
			queue:                 queues.AF3,
			inputIntf:             intf1,
		},
		"intf1-af2": {
			frameSize:             512,
			trafficRate:           10,
			expectedThroughputPct: 50.0,
			dscp:                  4,
			queue:                 queues.AF2,
			inputIntf:             intf1,
		},
		"intf1-af1": {
			frameSize:             512,
			trafficRate:           12,
			expectedThroughputPct: 100.0,
			dscp:                  5,
			queue:                 queues.AF1,
			inputIntf:             intf1,
		},
		"intf1-be1": {
			frameSize:             512,
			trafficRate:           12,
			expectedThroughputPct: 100.0,
			dscp:                  6,
			queue:                 queues.BE1,
			inputIntf:             intf1,
		},
		"intf2-nc1": {
			frameSize:             512,
			trafficRate:           1,
			dscp:                  1,
			expectedThroughputPct: 0,
			queue:                 queues.NC1,
			inputIntf:             intf2,
		},
		"intf2-af4": {
			frameSize:             512,
			trafficRate:           30,
			dscp:                  2,
			expectedThroughputPct: 0,
			queue:                 queues.AF4,
			inputIntf:             intf2,
		},
		"intf2-af3": {
			frameSize:             512,
			trafficRate:           12,
			expectedThroughputPct: 0,
			dscp:                  3,
			queue:                 queues.AF3,
			inputIntf:             intf2,
		},
		"intf2-af2": {
			frameSize:             512,
			trafficRate:           10,
			expectedThroughputPct: 50.0,
			dscp:                  4,
			queue:                 queues.AF2,
			inputIntf:             intf2,
		},
		"intf2-af1": {
			frameSize:             512,
			trafficRate:           12,
			expectedThroughputPct: 100.0,
			dscp:                  5,
			queue:                 queues.AF1,
			inputIntf:             intf2,
		},
		"intf2-be1": {
			frameSize:             512,
			trafficRate:           12,
			dscp:                  6,
			expectedThroughputPct: 100.0,
			queue:                 queues.BE1,
			inputIntf:             intf2,
		},
	}

	cases := []struct {
		desc         string
		trafficFlows map[string]*trafficData
		trafficType  string
	}{{
		desc:         "Egress Strict Priority scheduler for IPv4 Traffic",
		trafficFlows: EgressIPv4TrafficFlows,
		trafficType:  "ipv4",
	}, {
		desc:         "Egress Strict Priority scheduler for IPv6 Traffic",
		trafficFlows: EgressIPv6TrafficFlows,
		trafficType:  "ipv6",
	}, {
		desc:         "Egress Strict Priority scheduler for MPLS Traffic",
		trafficFlows: EgressMplsTrafficFlows,
		trafficType:  "mpls",
	}}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			trafficFlows := tc.trafficFlows
			top.Flows().Clear()
			if tc.trafficType == "ipv4" {
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
					ipHeader.Priority().Dscp().Phb().SetValue(uint32(data.dscp))
					flow.Size().SetFixed(uint32(data.frameSize))
					flow.Rate().SetPercentage(float32(data.trafficRate))

				}
				ate.OTG().PushConfig(t, top)
				ate.OTG().StartProtocols(t)
				otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")
			} else if tc.trafficType == "ipv6" {
				for trafficID, data := range trafficFlows {
					t.Logf("Configuring flow %s", trafficID)
					flow := top.Flows().Add().SetName(trafficID)
					flow.Metrics().SetEnable(true)
					flow.TxRx().Device().SetTxNames([]string{data.inputIntf.Name + ".IPv6"}).SetRxNames([]string{intf3.Name + ".IPv6"})
					ethHeader := flow.Packet().Add().Ethernet()
					ethHeader.Src().SetValue(data.inputIntf.MAC)

					ipHeader := flow.Packet().Add().Ipv6()
					ipHeader.Src().SetValue(data.inputIntf.IPv6)
					ipHeader.Dst().SetValue(intf3.IPv6)
					ipHeader.TrafficClass().SetValue(uint32(data.dscp))
					flow.Size().SetFixed(uint32(data.frameSize))
					flow.Rate().SetPercentage(float32(data.trafficRate))

				}
				ate.OTG().PushConfig(t, top)
				ate.OTG().StartProtocols(t)
				otgutils.WaitForARP(t, ate.OTG(), top, "IPv6")
			} else {
				// get dut mac interface for traffic mpls flow
				dutDstInterface := dut.Port(t, "port3").Name()
				dstMac := gnmi.Get(t, dut, gnmi.OC().Interface(dutDstInterface).Ethernet().MacAddress().State())
				t.Logf("DUT remote mac address is %s", dstMac)
				for trafficID, data := range trafficFlows {
					// t.Logf("Configuring flow %s", trafficID)
					// flow := top.Flows().Add().SetName(trafficID)
					// flow.Metrics().SetEnable(true)
					// flow.TxRx().Port().SetTxName("port3").SetRxNames([]string{"port1", "port2"})
					// ethHeader := flow.Packet().Add().Ethernet()
					// ethHeader.Src().SetValue(data.inputIntf.MAC)
					// ethHeader.Dst().SetValue(intf3.MAC)
					// ipHeader := flow.Packet().Add().Mpls()
					// ipHeader.TrafficClass().SetValue(uint32(data.dscp))
					// flow.Size().SetFixed(uint32(data.frameSize))
					// flow.Rate().SetPercentage(float32(data.trafficRate))
					t.Logf("Configuring flow %s", trafficID)
					flow := top.Flows().Add().SetName(trafficID)
					flow.Metrics().SetEnable(true)
					flow.TxRx().Port().SetTxName("port3").SetRxNames([]string{"port1", "port2"})
					// Set up ethernet layer.
					ethHeader := flow.Packet().Add().Ethernet()
					ethHeader.Src().SetValue(data.inputIntf.MAC)
					ethHeader.Dst().SetValue(dstMac)
					// Set up mpls layer.
					ipHeader := flow.Packet().Add().Mpls()
					ipHeader.TrafficClass().SetValue(uint32(data.dscp))
					flow.Size().SetFixed(uint32(data.frameSize))
					flow.Rate().SetPercentage(float32(data.trafficRate))
					ip4 := flow.Packet().Add().Ipv4()
					ip4.Src().SetValue(data.inputIntf.IPv4)
					ip4.Dst().SetValue(intf3.IPv4)
					ip4.Version().SetValue(4)
				}
				ate.OTG().PushConfig(t, top)
				ate.OTG().StartProtocols(t)
				otgutils.WaitForARP(t, ate.OTG(), top, "Mpls")
			}
			trafficInputRate := make(map[string]float64)
			trafficOutputRate := make(map[string]float64)
			counters := make(map[string]map[string]uint64)

			var counterNames = []string{
				"ateOutPkts", "ateInPkts", "dutQosPktsBeforeTraffic", "dutQosOctetsBeforeTraffic",
				"dutQosPktsAfterTraffic", "dutQosOctetsAfterTraffic", "dutQosDroppedPktsBeforeTraffic",
				"dutQosDroppedOctetsBeforeTraffic", "dutQosDroppedPktsAfterTraffic",
				"dutQosDroppedOctetsAfterTraffic",
			}

			for _, name := range counterNames {
				counters[name] = make(map[string]uint64)

				// Set the initial counters to 0.
				for _, data := range trafficFlows {
					counters[name][data.queue] = 0
				}
			}

			// Get QoS egress packet counters before the traffic.
			const timeout = time.Minute
			isPresent := func(val *ygnmi.Value[uint64]) bool { return val.IsPresent() }
			for _, data := range trafficFlows {
				count, ok := gnmi.Watch(t, dut, gnmi.OC().Qos().Interface(dp3.Name()).Output().Queue(data.queue).TransmitPkts().State(), timeout, isPresent).Await(t)
				if !ok {
					t.Errorf("TransmitPkts count for queue %q on interface %q not available within %v", dp3.Name(), data.queue, timeout)
				}
				counters["dutQosPktsBeforeTraffic"][data.queue], _ = count.Val()

				count, ok = gnmi.Watch(t, dut, gnmi.OC().Qos().Interface(dp3.Name()).Output().Queue(data.queue).TransmitOctets().State(), timeout, isPresent).Await(t)
				if !ok {
					t.Errorf("TransmitOctets count for queue %q on interface %q not available within %v", dp3.Name(), data.queue, timeout)
				}
				counters["dutQosOctetsBeforeTraffic"][data.queue], _ = count.Val()

				count, ok = gnmi.Watch(t, dut, gnmi.OC().Qos().Interface(dp3.Name()).Output().Queue(data.queue).DroppedPkts().State(), timeout, isPresent).Await(t)
				if !ok {
					t.Errorf("DroppedPkts count for queue %q on interface %q not available within %v", dp3.Name(), data.queue, timeout)
				}
				counters["dutQosDroppedPktsBeforeTraffic"][data.queue], _ = count.Val()

				count, ok = gnmi.Watch(t, dut, gnmi.OC().Qos().Interface(dp3.Name()).Output().Queue(data.queue).DroppedOctets().State(), timeout, isPresent).Await(t)
				if !ok {
					t.Errorf("DroppedOctets count for queue %q on interface %q not available within %v", dp3.Name(), data.queue, timeout)
				}
				counters["dutQosDroppedOctetsBeforeTraffic"][data.queue], _ = count.Val()
			}

			t.Logf("Running traffic 1 on DUT interfaces: %s => %s ", dp1.Name(), dp3.Name())
			t.Logf("Running traffic 2 on DUT interfaces: %s => %s ", dp2.Name(), dp3.Name())
			t.Logf("Sending traffic flows: \n%v\n\n", trafficFlows)
			ate.OTG().StartTraffic(t)
			time.Sleep(30 * time.Second)
			ate.OTG().StopTraffic(t)
			time.Sleep(30 * time.Second)

			otgutils.LogFlowMetrics(t, ate.OTG(), top)
			for trafficID, data := range trafficFlows {
				ateTxPkts := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(trafficID).Counters().OutPkts().State()) //Number of transmitted packets.
				ateRxPkts := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(trafficID).Counters().InPkts().State())  //Number of received packets.
				counters["ateOutPkts"][data.queue] += ateTxPkts
				counters["ateInPkts"][data.queue] += ateRxPkts

				counters["dutQosPktsAfterTraffic"][data.queue] = gnmi.Get(t, dut, gnmi.OC().Qos().Interface(dp3.Name()).Output().Queue(data.queue).TransmitPkts().State())
				counters["dutQosOctetsAfterTraffic"][data.queue] = gnmi.Get(t, dut, gnmi.OC().Qos().Interface(dp3.Name()).Output().Queue(data.queue).TransmitOctets().State())
				counters["dutQosDroppedPktsAfterTraffic"][data.queue] = gnmi.Get(t, dut, gnmi.OC().Qos().Interface(dp3.Name()).Output().Queue(data.queue).DroppedPkts().State())
				counters["dutQosDroppedOctetsAfterTraffic"][data.queue] = gnmi.Get(t, dut, gnmi.OC().Qos().Interface(dp3.Name()).Output().Queue(data.queue).DroppedOctets().State())
				t.Logf("ateInPkts: %v, txPkts %v, Queue: %v", counters["ateInPkts"][data.queue], counters["dutQosPktsAfterTraffic"][data.queue], data.queue)
				// Calculate aggregated throughput:
				_, ok := trafficInputRate[data.queue]
				if !ok {
					trafficInputRate[data.queue] = data.trafficRate
				} else {
					trafficInputRate[data.queue] += data.trafficRate
				}
				// Verify packet loss with tolerance
				if otgutils.GetFlowLossPct(t, ate.OTG(), trafficID, 30) > float64(tolerance) {
					t.Errorf("Packet loss (%.2f%%) exceeded tolerance (%.2f%%) for flow %s", otgutils.GetFlowLossPct(t, ate.OTG(), trafficID, 30), tolerance, trafficID)
				}
				// Verify the loss packet as per the test scenario
				lossPct := (float32)((float64(ateTxPkts-ateRxPkts) * 100.0) / float64(ateTxPkts)) //Loss Percentage= Tx - Rx / Tx * 100
				t.Logf("Get flow %q: lossPct: %.2f%% or rxPct: %.2f%%, want: %.2f%%\n\n", data.queue, lossPct, 100.0-lossPct, data.expectedThroughputPct)
				if got, want := 100.0-lossPct, data.expectedThroughputPct; got != want {
					t.Errorf("Get(throughput for queue %q): got %.2f%%, want %.2f%%", data.queue, got, want)
				}
				queueDropPath := gnmi.OC().Qos().Interface(dp3.Name()).Output().Queue(queues.NC1).DroppedPkts().State()
				queueDrops := gnmi.Get(t, dut, queueDropPath)
				if math.Abs(float64(lossPct)-float64(queueDrops)) > float64(tolerance) {
					t.Errorf("Mismatch: Flow loss %.2f%%, but queue drops %d for queue %s", lossPct, queueDrops, queues.NC1)
				}
				_, ok = trafficOutputRate[data.queue]
				if !ok {
					trafficOutputRate[data.queue] = data.trafficRate * float64(100.0-lossPct)
				} else {
					trafficOutputRate[data.queue] += data.trafficRate * float64(100.0-lossPct)
					got := trafficOutputRate[data.queue] / trafficInputRate[data.queue]
					want := float64(data.expectedThroughputPct)
					if got < want-float64(tolerance) || got > want+float64(tolerance) {
						t.Errorf("Get(throughput for queue %q): got %.2f%%, want within [%.2f%%, %.2f%%]", data.queue, got, want-float64(tolerance), want+float64(tolerance))
					}
				}
			}

			// Check QoS egress packet counters are updated correctly.
			for _, name := range counterNames {
				t.Logf("QoS %s: %v", name, counters[name])
			}

			for _, data := range trafficFlows {
				// These counters record QoS metrics related to transmitted and dropped packets at the DUT
				dutPktCounterDiff := counters["dutQosPktsAfterTraffic"][data.queue] - counters["dutQosPktsBeforeTraffic"][data.queue]
				atePktCounterDiff := counters["ateInPkts"][data.queue]
				t.Logf("Queue %q: atePktCounterDiff: %v dutPktCounterDiff: %v", data.queue, atePktCounterDiff, dutPktCounterDiff)
				if dutPktCounterDiff < atePktCounterDiff {
					t.Errorf("Get dutPktCounterDiff for queue %q: got %v, want >= %v", data.queue, dutPktCounterDiff, atePktCounterDiff)
				}

				ateDropPktCounterDiff := counters["ateOutPkts"][data.queue] - counters["ateInPkts"][data.queue]
				dutDropPktCounterDiff := counters["dutQosDroppedPktsAfterTraffic"][data.queue] - counters["dutQosDroppedPktsBeforeTraffic"][data.queue]
				t.Logf("Queue %q: ateDropPktCounterDiff: %v dutDropPktCounterDiff: %v", data.queue, ateDropPktCounterDiff, dutDropPktCounterDiff)
				if dutDropPktCounterDiff < ateDropPktCounterDiff {
					if !deviations.DequeueDeleteNotCountedAsDrops(dut) {
						t.Errorf("Get dutDropPktCounterDiff for queue %q: got %v, want >= %v", data.queue, dutDropPktCounterDiff, ateDropPktCounterDiff)
					}
				}

				dutOctetCounterDiff := counters["dutQosOctetsAfterTraffic"][data.queue] - counters["dutQosOctetsBeforeTraffic"][data.queue]
				ateOctetCounterDiff := counters["ateInPkts"][data.queue] * uint64(data.frameSize)
				t.Logf("Queue %q: ateOctetCounterDiff: %v dutOctetCounterDiff: %v", data.queue, ateOctetCounterDiff, dutOctetCounterDiff)
				if !deviations.QOSOctets(dut) {
					if dutOctetCounterDiff < ateOctetCounterDiff {
						t.Errorf("Get dutOctetCounterDiff for queue %q: got %v, want >= %v", data.queue, dutOctetCounterDiff, ateOctetCounterDiff)
					}
				}

				ateDropOctetCounterDiff := (counters["ateOutPkts"][data.queue] - counters["ateInPkts"][data.queue]) * uint64(data.frameSize)
				dutDropOctetCounterDiff := counters["dutQosDroppedOctetsAfterTraffic"][data.queue] - counters["dutQosDroppedOctetsBeforeTraffic"][data.queue]
				t.Logf("Queue %q: ateDropOctetCounterDiff: %v dutDropOctetCounterDiff: %v", data.queue, ateDropOctetCounterDiff, dutDropOctetCounterDiff)
				if dutDropOctetCounterDiff < ateDropOctetCounterDiff {
					if !deviations.DequeueDeleteNotCountedAsDrops(dut) {
						t.Errorf("Get dutDropOctetCounterDiff for queue %q: got %v, want >= %v", data.queue, dutDropOctetCounterDiff, ateDropOctetCounterDiff)
					}
				}

			}
		})
	}
}

func ConfigureDUTIntf(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	dc := gnmi.OC()
	t.Log("Configure interfaces on dut1.")
	i1 := dutPort1.NewOCInterface(dut.Port(t, "port1").Name(), dut)
	gnmi.Replace(t, dut, dc.Interface(i1.GetName()).Config(), i1)
	i2 := dutPort2.NewOCInterface(dut.Port(t, "port2").Name(), dut)
	gnmi.Replace(t, dut, dc.Interface(i2.GetName()).Config(), i2)
	i3 := dutPort3.NewOCInterface(dut.Port(t, "port3").Name(), dut)
	gnmi.Replace(t, dut, dc.Interface(i3.GetName()).Config(), i3)

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, dut.Port(t, "port1"))
		fptest.SetPortSpeed(t, dut.Port(t, "port2"))
		fptest.SetPortSpeed(t, dut.Port(t, "port3"))
	}
}

// Custom type that includes both the string and the enum value
type CustomNetworkInstanceType struct {
	Prefix string
	Type   oc.E_NetworkInstanceTypes_NETWORK_INSTANCE_TYPE
}

// Method to convert the custom type to the required enum type
func (c CustomNetworkInstanceType) String() string {
	return c.Prefix + c.Type.String()
}

// configureStaticLSP configures a static MPLS LSP with the provided parameters.
func configureStaticLSP(t *testing.T, dut *ondatra.DUTDevice, lspName string, incomingLabel uint32, nextHopIP string) {
	d := &oc.Root{}
	dni := deviations.DefaultNetworkInstance(dut)
	defPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut))
	// Create an instance of the custom type
	// failed to apply: Failed must constraint "(type='oc-ni-types:DEFAULT_INSTANCE' and name='default') or (type='oc-ni-types:L3VRF' and name!='default') or (type='oc-ni-types:L2VSI' and name!='default')" of node /network-instances/network-instance[name='DEFAULT']/config, found bad element: name
	mplsType := CustomNetworkInstanceType{
		Prefix: "oc-ni-types:",
		Type:   oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE,
	}
	mplsName := "default" //  failed to process /network-instances/network-instance[name=DEFAULT]: cannot replace existing key "name" with default, already set to DEFAULT
	t.Log("mohan type:", mplsType)
	// ygot.String(deviations.DefaultNetworkInstance(dut))
	gnmi.Update(t, dut, defPath.Config(), &oc.NetworkInstance{
		Name: &mplsName,
		Type: mplsType.Type, // Use the enum value directly
	})
	// Check if the NetworkInstance already exists
	// existingNI := gnmi.Get(t, dut, defPath.Config()) // Get(t) on dut(dut) at /network-instances/network-instance[name=DEFAULT]: path origin:"openconfig" elem:{name:"network-instances"} elem:{name:"network-instance" key:{key:"name" value:"DEFAULT"}}: value not present
	// if existingNI == nil {
	// 	// If it doesn't exist, create a new NetworkInstance
	// 	t.Log("mohan type1:", mplsType)
	// 	gnmi.Update(t, dut, defPath.Config(), &oc.NetworkInstance{
	// 		Name: &mplsName,
	// 		Type: mplsType.Type, // Use the enum value directly
	// 	})
	// } else {
	// 	// If it exists, update only the necessary fields
	// 	gnmi.Update(t, dut, defPath.Type().Config(), mplsType.Type)
	// }
	mplsCfg := d.GetOrCreateNetworkInstance(dni).GetOrCreateMpls()
	staticMplsCfg := mplsCfg.GetOrCreateLsps().GetOrCreateStaticLsp(lspName)
	staticMplsCfg.GetOrCreateEgress().SetIncomingLabel(oc.UnionUint32(incomingLabel))
	staticMplsCfg.GetOrCreateEgress().SetNextHop(nextHopIP)
	staticMplsCfg.GetOrCreateEgress().SetPushLabel(oc.Egress_PushLabel_IMPLICIT_NULL)
	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Mpls().Config(), mplsCfg)
}

func ConfigureQoS(t *testing.T, dut *ondatra.DUTDevice) {
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

	t.Logf("Create qos Classifiers config")
	classifiers := []struct {
		desc        string
		name        string
		classType   oc.E_Qos_Classifier_Type
		termID      string
		targetGroup string
		dscpSet     []uint8
		expSet      []uint8 // MPLS EXP values
	}{{
		desc:        "classifier_ipv4_be1",
		name:        "dscp_based_classifier_ipv4",
		classType:   oc.Qos_Classifier_Type_IPV4,
		termID:      "0",
		targetGroup: "target-group-BE1",
		dscpSet:     []uint8{0, 1, 2, 3},
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
	}, {
		desc:        "classifier_mpls_be1",
		name:        "dscp_based_classifier_mpls",
		classType:   oc.Qos_Classifier_Type_MPLS,
		termID:      "0",
		targetGroup: "target-group-BE1",
		expSet:      []uint8{0, 1, 2, 3},
	}, {
		desc:        "classifier_mpls_af1",
		name:        "dscp_based_classifier_mpls",
		classType:   oc.Qos_Classifier_Type_MPLS,
		termID:      "2",
		targetGroup: "target-group-AF1",
		expSet:      []uint8{8, 9, 10, 11},
	}, {
		desc:        "classifier_mpls_af2",
		name:        "dscp_based_classifier_mpls",
		classType:   oc.Qos_Classifier_Type_MPLS,
		termID:      "3",
		targetGroup: "target-group-AF2",
		expSet:      []uint8{16, 17, 18, 19},
	}, {
		desc:        "classifier_mpls_af3",
		name:        "dscp_based_classifier_mpls",
		classType:   oc.Qos_Classifier_Type_MPLS,
		termID:      "4",
		targetGroup: "target-group-AF3",
		expSet:      []uint8{24, 25, 26, 27},
	}, {
		desc:        "classifier_mpls_af4",
		name:        "dscp_based_classifier_mpls",
		classType:   oc.Qos_Classifier_Type_MPLS,
		termID:      "5",
		targetGroup: "target-group-AF4",
		expSet:      []uint8{32, 33, 34, 35},
	}, {
		desc:        "classifier_mpls_nc1",
		name:        "dscp_based_classifier_mpls",
		classType:   oc.Qos_Classifier_Type_MPLS,
		termID:      "6",
		targetGroup: "target-group-NC1",
		expSet:      []uint8{48, 49, 50, 51, 52, 53, 54, 55, 56, 57, 58, 59},
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
		if len(tc.dscpSet) > 0 {
			condition.GetOrCreateIpv4().SetDscpSet(tc.dscpSet)
			condition.GetOrCreateIpv6().SetDscpSet(tc.dscpSet)
		} else if len(tc.expSet) > 0 {
			condition.GetOrCreateMpls().SetTrafficClass(tc.expSet[0])
		}
		// if tc.name == "dscp_based_classifier_ipv4" {
		// 	condition.GetOrCreateIpv4().SetDscpSet(tc.dscpSet)
		// } else if tc.name == "dscp_based_classifier_ipv6" {
		// 	condition.GetOrCreateIpv6().SetDscpSet(tc.dscpSet)
		// }
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
		desc:                "Input Classifier Type MPLS",
		intf:                dp1.Name(),
		inputClassifierType: oc.Input_Classifier_Type_MPLS,
		classifier:          "exp_based_classifier_mpls",
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
	}, {
		desc:                "Input Classifier Type MPLS",
		intf:                dp2.Name(),
		inputClassifierType: oc.Input_Classifier_Type_MPLS,
		classifier:          "exp_based_classifier_mpls",
	}}

	t.Logf("qos input classifier config: %v", classifierIntfs)
	for _, tc := range classifierIntfs {
		qoscfg.SetInputClassifier(t, dut, q, tc.intf, tc.inputClassifierType, tc.classifier)
	}

	nc1InputWeight := uint64(200)
	af4InputWeight := uint64(100)
	if deviations.SchedulerInputWeightLimit(dut) {
		nc1InputWeight = uint64(100)
		af4InputWeight = uint64(99)
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
		desc:        "scheduler-policy-AF1",
		sequence:    uint32(1),
		priority:    oc.Scheduler_Priority_UNSET,
		inputID:     "AF1",
		inputType:   oc.Input_InputType_QUEUE,
		weight:      uint64(8),
		queueName:   queues.AF1,
		targetGroup: "target-group-AF1",
	}, {
		desc:        "scheduler-policy-AF2",
		sequence:    uint32(1),
		priority:    oc.Scheduler_Priority_UNSET,
		inputID:     "AF2",
		inputType:   oc.Input_InputType_QUEUE,
		weight:      uint64(16),
		queueName:   queues.AF2,
		targetGroup: "target-group-AF2",
	}, {
		desc:        "scheduler-policy-AF3",
		sequence:    uint32(1),
		priority:    oc.Scheduler_Priority_UNSET,
		inputID:     "AF3",
		inputType:   oc.Input_InputType_QUEUE,
		weight:      uint64(32),
		queueName:   queues.AF3,
		targetGroup: "target-group-AF3",
	}, {
		desc:        "scheduler-policy-AF4",
		sequence:    uint32(0),
		priority:    oc.Scheduler_Priority_STRICT,
		inputID:     "AF4",
		inputType:   oc.Input_InputType_QUEUE,
		weight:      af4InputWeight,
		queueName:   queues.AF4,
		targetGroup: "target-group-AF4",
	}, {
		desc:        "scheduler-policy-NC1",
		sequence:    uint32(0),
		priority:    oc.Scheduler_Priority_STRICT,
		inputID:     "NC1",
		inputType:   oc.Input_InputType_QUEUE,
		weight:      nc1InputWeight,
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

func ConfigureCiscoQos(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")
	dp3 := dut.Port(t, "port3")
	d := &oc.Root{}
	q := d.GetOrCreateQos()
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
	queueName := []string{"NC1", "AF4", "AF3", "AF2", "AF1", "BE1"}

	for i, queue := range queueName {
		q1 := q.GetOrCreateQueue(queue)
		q1.Name = ygot.String(queue)
		queueid := len(queueName) - i
		q1.QueueId = ygot.Uint8(uint8(queueid))

	}
	gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), q)
	casesfwdgrp := []struct {
		desc        string
		queueName   string
		targetGroup string
	}{{
		desc:        "forwarding-group-BE1",
		queueName:   "BE1",
		targetGroup: "target-group-BE1",
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
	t.Logf("qos forwarding groups config cases: %v", casesfwdgrp)
	for _, tc := range casesfwdgrp {
		t.Run(tc.desc, func(t *testing.T) {
			qoscfg.SetForwardingGroup(t, dut, q, tc.targetGroup, tc.queueName)
		})
	}
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
		if tc.classType == oc.Qos_Classifier_Type_IPV4 {
			condition.GetOrCreateIpv4().SetDscpSet(tc.dscpSet)
		} else if tc.classType == oc.Qos_Classifier_Type_IPV6 {
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
		queueName:   "BE1",
		targetGroup: "target-group-BE1",
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
		weight:      uint64(6),
		queueName:   "AF4",
		targetGroup: "target-group-AF4",
	}, {
		desc:        "scheduler-policy-NC1",
		sequence:    uint32(0),
		priority:    oc.Scheduler_Priority_STRICT,
		inputID:     "NC1",
		inputType:   oc.Input_InputType_QUEUE,
		weight:      uint64(7),
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
		desc:      "output-interface-NC1",
		queueName: "NC1",
		scheduler: "scheduler",
	}, {
		desc:      "output-interface-AF4",
		queueName: "AF4",
		scheduler: "scheduler",
	}, {
		desc:      "output-interface-AF3",
		queueName: "AF3",
		scheduler: "scheduler",
	}, {
		desc:      "output-interface-AF2",
		queueName: "AF2",
		scheduler: "scheduler",
	}, {
		desc:      "output-interface-AF1",
		queueName: "AF1",
		scheduler: "scheduler",
	}, {
		desc:      "output-interface-BE1",
		queueName: "BE1",
		scheduler: "scheduler",
	}}

	t.Logf("qos output interface config: %v", schedulerIntfs)
	for _, tc := range schedulerIntfs {
		i := q.GetOrCreateInterface(dp3.Name())
		i.SetInterfaceId(dp3.Name())
		i.GetOrCreateInterfaceRef().Interface = ygot.String(dp3.Name())
		output := i.GetOrCreateOutput()
		schedulerPolicy := output.GetOrCreateSchedulerPolicy()
		schedulerPolicy.SetName(tc.scheduler)
		queue := output.GetOrCreateQueue(tc.queueName)
		queue.SetName(tc.queueName)

	}
	gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), q)
}
