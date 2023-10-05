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

package egress_strict_priority_scheduler_test

import (
	"context"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

type trafficData struct {
	trafficRate           float64
	expectedThroughputPct float32
	frameSize             uint32
	dscp                  uint8
	queue                 string
	inputIntf             *ondatra.Interface
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Test cases:
//  1) Non-oversubscription NC1 and AF4 traffic.
//     - There should be no packet drop for all traffic classes.
//  2) Non-oversubscription NC1 and AF3 traffic.
//     - There should be no packet drop for all traffic classes.
//  3) Non-oversubscription NC1 and AF2 traffic.
//     - There should be no packet drop for all traffic classes.
//  4) Non-oversubscription NC1 and AF1 traffic.
//     - There should be no packet drop for all traffic classes.
//  5) Non-oversubscription NC1 and BE1 traffic.
//     - There should be no packet drop for all traffic classes.
//  6) Oversubscription NC1 and AF4 traffic.
//     - There should be no packet drop for strict priority traffic class.
//  7) Oversubscription NC1 and AF3 traffic.
//     - There should be no packet drop for strict priority traffic class.
//  8) Oversubscription NC1 and AF2 traffic.
//     - There should be no packet drop for strict priority traffic class.
//  9) Oversubscription NC1 and AF1 traffic.
//     - There should be no packet drop for strict priority traffic class.
//  10) Oversubscription NC1 and BE1 traffic.
//     - There should be no packet drop for strict priority traffic class.
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

func TestOneSPQueueTraffic(t *testing.T) {
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
	top := ate.Topology().New()
	intf1 := top.AddInterface("intf1").WithPort(ap1)
	intf1.IPv4().
		WithAddress("198.51.100.1/31").
		WithDefaultGateway("198.51.100.0")
	intf2 := top.AddInterface("intf2").WithPort(ap2)
	intf2.IPv4().
		WithAddress("198.51.100.3/31").
		WithDefaultGateway("198.51.100.2")
	intf3 := top.AddInterface("intf3").WithPort(ap3)
	intf3.IPv4().
		WithAddress("198.51.100.5/31").
		WithDefaultGateway("198.51.100.4")
	top.Push(t).StartProtocols(t)

	var tolerance float32 = 2.0
	type qosVals struct {
		be1, af1, af2, af3, af4, nc1 string
	}

	qos := qosVals{
		be1: "BE1",
		af1: "AF1",
		af2: "AF2",
		af3: "AF3",
		af4: "AF4",
		nc1: "NC1",
	}

	if dut.Vendor() == ondatra.JUNIPER {
		qos = qosVals{
			be1: "0",
			af1: "4",
			af2: "1",
			af3: "5",
			af4: "2",
			nc1: "3",
		}
	}

	// Test case 1: Non-oversubscription NC1 and AF4 traffic.
	//   - There should be no packet drop for all traffic classes.
	nonOversubscribedTrafficFlows1 := map[string]*trafficData{
		"intf1-nc1": {
			frameSize:             1000,
			trafficRate:           0.1,
			expectedThroughputPct: 100.0,
			dscp:                  56,
			queue:                 qos.nc1,
			inputIntf:             intf1,
		},
		"intf1-af4": {
			frameSize:             1000,
			trafficRate:           45.1,
			expectedThroughputPct: 100.0,
			dscp:                  32,
			queue:                 qos.af4,
			inputIntf:             intf1,
		},
		"intf2-nc1": {
			frameSize:             1000,
			trafficRate:           0.7,
			dscp:                  56,
			expectedThroughputPct: 100.0,
			queue:                 qos.nc1,
			inputIntf:             intf2,
		},
		"intf2-af4": {
			frameSize:             1000,
			trafficRate:           54.1,
			dscp:                  32,
			expectedThroughputPct: 100.0,
			queue:                 qos.af4,
			inputIntf:             intf2,
		},
	}

	// Test case 2: Non-oversubscription NC1 and AF3 traffic.
	//   - There should be no packet drop for all traffic classes.
	nonOversubscribedTrafficFlows2 := map[string]*trafficData{
		"intf1-nc1": {
			frameSize:             1000,
			trafficRate:           0.1,
			expectedThroughputPct: 100.0,
			dscp:                  56,
			queue:                 qos.nc1,
			inputIntf:             intf1,
		},
		"intf1-af3": {
			frameSize:             1000,
			trafficRate:           45.1,
			expectedThroughputPct: 100.0,
			dscp:                  24,
			queue:                 qos.af3,
			inputIntf:             intf1,
		},
		"intf2-nc1": {
			frameSize:             1000,
			trafficRate:           0.7,
			dscp:                  56,
			expectedThroughputPct: 100.0,
			queue:                 qos.nc1,
			inputIntf:             intf2,
		},
		"intf2-af3": {
			frameSize:             1000,
			trafficRate:           54.1,
			dscp:                  24,
			expectedThroughputPct: 100.0,
			queue:                 qos.af3,
			inputIntf:             intf2,
		},
	}

	// Test case 3: Non-oversubscription NC1 and AF2 traffic.
	//   - There should be no packet drop for all traffic classes.
	nonOversubscribedTrafficFlows3 := map[string]*trafficData{
		"intf1-nc1": {
			frameSize:             1000,
			trafficRate:           0.1,
			expectedThroughputPct: 100.0,
			dscp:                  56,
			queue:                 qos.nc1,
			inputIntf:             intf1,
		},
		"intf1-af2": {
			frameSize:             1000,
			trafficRate:           45.1,
			expectedThroughputPct: 100.0,
			dscp:                  16,
			queue:                 qos.af2,
			inputIntf:             intf1,
		},
		"intf2-nc1": {
			frameSize:             1000,
			trafficRate:           0.7,
			dscp:                  56,
			expectedThroughputPct: 100.0,
			queue:                 qos.nc1,
			inputIntf:             intf2,
		},
		"intf2-af2": {
			frameSize:             1000,
			trafficRate:           54.1,
			dscp:                  19,
			expectedThroughputPct: 100.0,
			queue:                 qos.af2,
			inputIntf:             intf2,
		},
	}

	// Test case 4: Non-oversubscription NC1 and AF1 traffic.
	//   - There should be no packet drop for all traffic classes.
	nonOversubscribedTrafficFlows4 := map[string]*trafficData{
		"intf1-nc1": {
			frameSize:             1000,
			trafficRate:           0.1,
			expectedThroughputPct: 100.0,
			dscp:                  56,
			queue:                 qos.nc1,
			inputIntf:             intf1,
		},
		"intf1-af1": {
			frameSize:             1000,
			trafficRate:           45.1,
			expectedThroughputPct: 100.0,
			dscp:                  8,
			queue:                 qos.af1,
			inputIntf:             intf1,
		},
		"intf2-nc1": {
			frameSize:             1000,
			trafficRate:           0.7,
			dscp:                  56,
			expectedThroughputPct: 100.0,
			queue:                 qos.nc1,
			inputIntf:             intf2,
		},
		"intf2-af1": {
			frameSize:             1000,
			trafficRate:           54.1,
			dscp:                  8,
			expectedThroughputPct: 100.0,
			queue:                 qos.af1,
			inputIntf:             intf2,
		},
	}

	// Test case 5: Non-oversubscription NC1 and BE1 traffic.
	//   - There should be no packet drop for all traffic classes.
	nonOversubscribedTrafficFlows5 := map[string]*trafficData{
		"intf1-nc1": {
			frameSize:             1000,
			trafficRate:           0.1,
			expectedThroughputPct: 100.0,
			dscp:                  56,
			queue:                 qos.nc1,
			inputIntf:             intf1,
		},
		"intf1-be1": {
			frameSize:             1000,
			trafficRate:           45.1,
			expectedThroughputPct: 100.0,
			dscp:                  0,
			queue:                 qos.be1,
			inputIntf:             intf1,
		},
		"intf2-nc1": {
			frameSize:             1000,
			trafficRate:           0.7,
			dscp:                  56,
			expectedThroughputPct: 100.0,
			queue:                 qos.nc1,
			inputIntf:             intf2,
		},
		"intf2-be1": {
			frameSize:             1000,
			trafficRate:           54.1,
			dscp:                  0,
			expectedThroughputPct: 100.0,
			queue:                 qos.be1,
			inputIntf:             intf2,
		},
	}

	// Test case 6: Oversubscription NC1 and AF4 traffic.
	//   - There should be no packet drop for strict priority traffic class.
	oversubscribedTrafficFlows1 := map[string]*trafficData{
		"intf1-nc1": {
			frameSize:             1000,
			trafficRate:           0.1,
			expectedThroughputPct: 100.0,
			dscp:                  56,
			queue:                 qos.nc1,
			inputIntf:             intf1,
		},
		"intf1-af4": {
			frameSize:             1000,
			trafficRate:           99.9,
			expectedThroughputPct: 49.8,
			dscp:                  32,
			queue:                 qos.af4,
			inputIntf:             intf1,
		},
		"intf2-nc1": {
			frameSize:             1000,
			trafficRate:           0.7,
			dscp:                  56,
			expectedThroughputPct: 100.0,
			queue:                 qos.nc1,
			inputIntf:             intf2,
		},
		"intf2-af4": {
			frameSize:             1000,
			trafficRate:           99.3,
			dscp:                  32,
			expectedThroughputPct: 49.8,
			queue:                 qos.af4,
			inputIntf:             intf2,
		},
	}

	// Test case 7: Oversubscription NC1 and AF3 traffic.
	//   - There should be no packet drop for strict priority traffic class.
	oversubscribedTrafficFlows2 := map[string]*trafficData{
		"intf1-nc1": {
			frameSize:             1000,
			trafficRate:           0.1,
			expectedThroughputPct: 100.0,
			dscp:                  56,
			queue:                 qos.nc1,
			inputIntf:             intf1,
		},
		"intf1-af3": {
			frameSize:             1000,
			trafficRate:           99.9,
			expectedThroughputPct: 49.8,
			dscp:                  24,
			queue:                 qos.af3,
			inputIntf:             intf1,
		},
		"intf2-nc1": {
			frameSize:             1000,
			trafficRate:           0.7,
			dscp:                  56,
			expectedThroughputPct: 100.0,
			queue:                 qos.nc1,
			inputIntf:             intf2,
		},
		"intf2-af3": {
			frameSize:             1000,
			trafficRate:           99.3,
			dscp:                  24,
			expectedThroughputPct: 49.8,
			queue:                 qos.af3,
			inputIntf:             intf2,
		},
	}

	// Test case 8: Oversubscription NC1 and AF2 traffic.
	//   - There should be no packet drop for strict priority traffic class.
	oversubscribedTrafficFlows3 := map[string]*trafficData{
		"intf1-nc1": {
			frameSize:             1000,
			trafficRate:           0.1,
			expectedThroughputPct: 100.0,
			dscp:                  56,
			queue:                 qos.nc1,
			inputIntf:             intf1,
		},
		"intf1-af2": {
			frameSize:             1000,
			trafficRate:           99.9,
			expectedThroughputPct: 49.8,
			dscp:                  16,
			queue:                 qos.af2,
			inputIntf:             intf1,
		},
		"intf2-nc1": {
			frameSize:             1000,
			trafficRate:           0.7,
			dscp:                  56,
			expectedThroughputPct: 100.0,
			queue:                 qos.nc1,
			inputIntf:             intf2,
		},
		"intf2-af2": {
			frameSize:             1000,
			trafficRate:           99.3,
			dscp:                  16,
			expectedThroughputPct: 49.8,
			queue:                 qos.af2,
			inputIntf:             intf2,
		},
	}

	// Test case 9: Oversubscription NC1 and AF1 traffic.
	//   - There should be no packet drop for strict priority traffic class.
	oversubscribedTrafficFlows4 := map[string]*trafficData{
		"intf1-nc1": {
			frameSize:             1000,
			trafficRate:           0.1,
			expectedThroughputPct: 100.0,
			dscp:                  56,
			queue:                 qos.nc1,
			inputIntf:             intf1,
		},
		"intf1-af1": {
			frameSize:             1000,
			trafficRate:           99.9,
			expectedThroughputPct: 49.8,
			dscp:                  8,
			queue:                 qos.af1,
			inputIntf:             intf1,
		},
		"intf2-nc1": {
			frameSize:             1000,
			trafficRate:           0.7,
			dscp:                  56,
			expectedThroughputPct: 100.0,
			queue:                 qos.nc1,
			inputIntf:             intf2,
		},
		"intf2-af1": {
			frameSize:             1000,
			trafficRate:           99.3,
			dscp:                  8,
			expectedThroughputPct: 49.8,
			queue:                 qos.af1,
			inputIntf:             intf2,
		},
	}

	// Test case 10: Oversubscription NC1 and BE1 traffic.
	//   - There should be no packet drop for strict priority traffic class.
	oversubscribedTrafficFlows5 := map[string]*trafficData{
		"intf1-nc1": {
			frameSize:             1000,
			trafficRate:           0.1,
			expectedThroughputPct: 100.0,
			dscp:                  56,
			queue:                 qos.nc1,
			inputIntf:             intf1,
		},
		"intf1-be1": {
			frameSize:             1000,
			trafficRate:           99.9,
			expectedThroughputPct: 49.8,
			dscp:                  0,
			queue:                 qos.be1,
			inputIntf:             intf1,
		},
		"intf2-nc1": {
			frameSize:             1000,
			trafficRate:           0.7,
			dscp:                  56,
			expectedThroughputPct: 100.0,
			queue:                 qos.be1,
			inputIntf:             intf2,
		},
		"intf2-be1": {
			frameSize:             1000,
			trafficRate:           99.3,
			dscp:                  0,
			expectedThroughputPct: 49.8,
			queue:                 qos.be1,
			inputIntf:             intf2,
		},
	}

	cases := []struct {
		desc         string
		trafficFlows map[string]*trafficData
	}{{
		desc:         "Non-oversubscription NC1 and AF4 traffic",
		trafficFlows: nonOversubscribedTrafficFlows1,
	}, {
		desc:         "Non-oversubscription NC1 and AF3 traffic",
		trafficFlows: nonOversubscribedTrafficFlows2,
	}, {
		desc:         "Non-oversubscription NC1 and AF2 traffic",
		trafficFlows: nonOversubscribedTrafficFlows3,
	}, {
		desc:         "Non-oversubscription NC1 and AF1 traffic",
		trafficFlows: nonOversubscribedTrafficFlows4,
	}, {
		desc:         "Non-oversubscription NC1 and BE1 traffic",
		trafficFlows: nonOversubscribedTrafficFlows5,
	}, {
		desc:         "Oversubscription NC1 and AF4 traffic with half AF4 dropped",
		trafficFlows: oversubscribedTrafficFlows1,
	}, {
		desc:         "Oversubscription NC1 and AF3 traffic with half AF3 dropped",
		trafficFlows: oversubscribedTrafficFlows2,
	}, {
		desc:         "Oversubscription NC1 and AF2 traffic with half AF2 dropped",
		trafficFlows: oversubscribedTrafficFlows3,
	}, {
		desc:         "Oversubscription NC1 and AF1 traffic with half AF1 dropped",
		trafficFlows: oversubscribedTrafficFlows4,
	}, {
		desc:         "Oversubscription NC1 and BE1 traffic with half BE1 dropped",
		trafficFlows: oversubscribedTrafficFlows5,
	}}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			trafficFlows := tc.trafficFlows

			var flows []*ondatra.Flow
			for trafficID, data := range trafficFlows {
				t.Logf("Configuring flow %s", trafficID)
				flow := ate.Traffic().NewFlow(trafficID).
					WithSrcEndpoints(data.inputIntf).
					WithDstEndpoints(intf3).
					WithHeaders(ondatra.NewEthernetHeader(), ondatra.NewIPv4Header().WithDSCP(data.dscp)).
					WithFrameRatePct(data.trafficRate).
					WithFrameSize(data.frameSize)
				flows = append(flows, flow)
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

			// Get QoS egress packet counters before the traffic.
			for _, data := range trafficFlows {
				dutQosPktsBeforeTraffic[data.queue] += gnmi.Get(t, dut, gnmi.OC().Qos().Interface(dp3.Name()).Output().Queue(data.queue).TransmitPkts().State())
				dutQosDroppedPktsBeforeTraffic[data.queue] += gnmi.Get(t, dut, gnmi.OC().Qos().Interface(dp3.Name()).Output().Queue(data.queue).DroppedPkts().State())
			}

			t.Logf("Running traffic 1 on DUT interfaces: %s => %s ", dp1.Name(), dp3.Name())
			t.Logf("Running traffic 2 on DUT interfaces: %s => %s ", dp2.Name(), dp3.Name())
			t.Logf("Sending traffic flows: \n%v\n\n", trafficFlows)
			ate.Traffic().Start(t, flows...)
			time.Sleep(30 * time.Second)
			ate.Traffic().Stop(t)
			time.Sleep(30 * time.Second)

			for trafficID, data := range trafficFlows {
				ateOutPkts[data.queue] += gnmi.Get(t, ate, gnmi.OC().Flow(trafficID).Counters().OutPkts().State())
				ateInPkts[data.queue] += gnmi.Get(t, ate, gnmi.OC().Flow(trafficID).Counters().InPkts().State())
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
		if deviations.InterfaceEnabled(dut) && !deviations.IPv4MissingEnabled(dut) {
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

	type qosVals struct {
		be1, af1, af2, af3, af4, nc1 string
	}

	qos := qosVals{
		be1: "BE1",
		af1: "AF1",
		af2: "AF2",
		af3: "AF3",
		af4: "AF4",
		nc1: "NC1",
	}

	if dut.Vendor() == ondatra.JUNIPER {
		qos = qosVals{
			be1: "0",
			af1: "1",
			af2: "4",
			af3: "5",
			af4: "2",
			nc1: "3",
		}
	}
	t.Log("Create qos forwarding groups config")
	forwardingGroups := []struct {
		desc        string
		queueName   string
		targetGroup string
	}{{
		desc:        "forwarding-group-BE1",
		queueName:   qos.be1,
		targetGroup: "target-group-BE1",
	}, {
		desc:        "forwarding-group-AF1",
		queueName:   qos.af1,
		targetGroup: "target-group-AF1",
	}, {
		desc:        "forwarding-group-AF2",
		queueName:   qos.af2,
		targetGroup: "target-group-AF2",
	}, {
		desc:        "forwarding-group-AF3",
		queueName:   qos.af3,
		targetGroup: "target-group-AF3",
	}, {
		desc:        "forwarding-group-AF4",
		queueName:   qos.af4,
		targetGroup: "target-group-AF4",
	}, {
		desc:        "forwarding-group-NC1",
		queueName:   qos.nc1,
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
		i.GetOrCreateInterfaceRef().Interface = ygot.String(tc.intf)
		i.GetOrCreateInterfaceRef().Subinterface = ygot.Uint32(0)
		if deviations.InterfaceRefConfigUnsupported(dut) {
			i.InterfaceRef = nil
		}
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
		queueName:   qos.be1,
		targetGroup: "target-group-BE1",
	}, {
		desc:        "scheduler-policy-AF1",
		sequence:    uint32(1),
		priority:    oc.Scheduler_Priority_UNSET,
		inputID:     "AF1",
		inputType:   oc.Input_InputType_QUEUE,
		weight:      uint64(4),
		queueName:   qos.af1,
		targetGroup: "target-group-AF1",
	}, {
		desc:        "scheduler-policy-AF2",
		sequence:    uint32(1),
		priority:    oc.Scheduler_Priority_UNSET,
		inputID:     "AF2",
		inputType:   oc.Input_InputType_QUEUE,
		weight:      uint64(8),
		queueName:   qos.af2,
		targetGroup: "target-group-AF2",
	}, {
		desc:        "scheduler-policy-AF3",
		sequence:    uint32(1),
		priority:    oc.Scheduler_Priority_UNSET,
		inputID:     "AF3",
		inputType:   oc.Input_InputType_QUEUE,
		weight:      uint64(12),
		queueName:   qos.af3,
		targetGroup: "target-group-AF3",
	}, {
		desc:        "scheduler-policy-AF4",
		sequence:    uint32(1),
		priority:    oc.Scheduler_Priority_UNSET,
		inputID:     "AF4",
		inputType:   oc.Input_InputType_QUEUE,
		weight:      uint64(48),
		queueName:   qos.af4,
		targetGroup: "target-group-AF4",
	}, {
		desc:        "scheduler-policy-NC1",
		sequence:    uint32(0),
		priority:    oc.Scheduler_Priority_STRICT,
		inputID:     "NC1",
		inputType:   oc.Input_InputType_QUEUE,
		weight:      uint64(100),
		queueName:   qos.nc1,
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
		queueName: qos.be1,
		scheduler: "scheduler",
	}, {
		desc:      "output-interface-AF1",
		queueName: qos.af1,
		scheduler: "scheduler",
	}, {
		desc:      "output-interface-AF2",
		queueName: qos.af2,
		scheduler: "scheduler",
	}, {
		desc:      "output-interface-AF3",
		queueName: qos.af3,
		scheduler: "scheduler",
	}, {
		desc:      "output-interface-AF4",
		queueName: qos.af4,
		scheduler: "scheduler",
	}, {
		desc:      "output-interface-NC1",
		queueName: qos.nc1,
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
	gnmiClient := dut.RawAPIs().GNMI(t)
	var config string
	switch dut.Vendor() {
	case ondatra.JUNIPER:
		t.Logf("Push the CLI config:\n%s", dut.Vendor())
		config = juniperCLI()
		gpbSetRequest := buildCLIConfigRequest(config)
		t.Log("gnmiClient Set CLI config")
		if _, err := gnmiClient.Set(context.Background(), gpbSetRequest); err != nil {
			t.Fatalf("gnmiClient.Set() with unexpected error: %v", err)
		}
	default:
		t.Fatalf("Vendor specific config is not supported ")
	}

}
func juniperCLI() string {
	return `
	class-of-service {
		traffic-manager {
			scheduling-mode {
				mode sps;
				strict-queues [ 2 3 4 5 ];
			}
		}
	}
  `
}
func buildCLIConfigRequest(config string) *gpb.SetRequest {
	// Build config with Origin set to cli and Ascii encoded config.
	gpbSetRequest := &gpb.SetRequest{
		Update: []*gpb.Update{{
			Path: &gpb.Path{
				Origin: "cli",
				Elem:   []*gpb.PathElem{},
			},
			Val: &gpb.TypedValue{
				Value: &gpb.TypedValue_AsciiVal{
					AsciiVal: config,
				},
			},
		}},
	}
	return gpbSetRequest
}
