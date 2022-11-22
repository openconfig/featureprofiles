// Copyright 2022 Google LLC
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

package qos_output_queue_counters_test

import (
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

type trafficData struct {
	trafficRate float64
	frameSize   uint32
	dscp        uint8
	queue       string
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Test cases:
//
// Verify the presence of the telemetry paths of the following features:
//  - Configure the interfaces connectd to ATE ports.
//  - Send the traffic with all forwarding class NC1, AF4, AF3, AF2, AF1 and BE1 over the DUT.
//  - Check the QoS queue counters exist and are updated correctly
//    - /qos/interfaces/interface/output/queues/queue/state/transmit-pkts
//    - /qos/interfaces/interface/output/queues/queue/state/transmit-octets
//    - /qos/interfaces/interface/output/queues/queue/state/dropped-pkts
//
// Topology:
//   ate:port1 <--> port1:dut:port2 <--> ate:port2
//
// Test notes:
//   - We may need to update the queue mapping after QoS feature implementation is finalized.
//
//  Sample CLI command to get telemetry using gmic:
//   - gnmic -a ipaddr:10162 -u username -p password --skip-verify get \
//      --path /components/component --format flat
//   - gnmic tool info:
//     - https://github.com/karimra/gnmic/blob/main/README.md
//

func TestQoSCounters(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")

	// Configure DUT interfaces.
	ConfigureDUTIntf(t, dut)

	// Configure ATE interfaces.
	ate := ondatra.ATE(t, "ate")
	ap1 := ate.Port(t, "port1")
	ap2 := ate.Port(t, "port2")
	top := ate.Topology().New()
	intf1 := top.AddInterface("intf1").WithPort(ap1)
	intf1.IPv4().
		WithAddress("198.51.100.1/31").
		WithDefaultGateway("198.51.100.0")
	intf2 := top.AddInterface("intf2").WithPort(ap2)
	intf2.IPv4().
		WithAddress("198.51.100.3/31").
		WithDefaultGateway("198.51.100.2")
	top.Push(t).StartProtocols(t)

	var trafficFlows map[string]*trafficData

	switch dut.Vendor() {
	case ondatra.JUNIPER:
		trafficFlows = map[string]*trafficData{
			"flow-nc1": {frameSize: 1000, trafficRate: 1, dscp: 56, queue: "3"},
			"flow-af4": {frameSize: 400, trafficRate: 4, dscp: 32, queue: "2"},
			"flow-af3": {frameSize: 300, trafficRate: 3, dscp: 24, queue: "5"},
			"flow-af2": {frameSize: 200, trafficRate: 2, dscp: 16, queue: "1"},
			"flow-af1": {frameSize: 1100, trafficRate: 1, dscp: 8, queue: "4"},
			"flow-be1": {frameSize: 1200, trafficRate: 1, dscp: 0, queue: "0"},
		}
	case ondatra.ARISTA:
		trafficFlows = map[string]*trafficData{
			"flow-nc1": {frameSize: 700, trafficRate: 7, dscp: 56, queue: dp2.Name() + "-7"},
			"flow-af4": {frameSize: 400, trafficRate: 4, dscp: 32, queue: dp2.Name() + "-4"},
			"flow-af3": {frameSize: 1300, trafficRate: 3, dscp: 24, queue: dp2.Name() + "-3"},
			"flow-af2": {frameSize: 1200, trafficRate: 2, dscp: 16, queue: dp2.Name() + "-2"},
			"flow-af1": {frameSize: 1000, trafficRate: 10, dscp: 8, queue: dp2.Name() + "-0"},
			"flow-be1": {frameSize: 1111, trafficRate: 1, dscp: 0, queue: dp2.Name() + "-1"},
		}
	case ondatra.CISCO:
		trafficFlows = map[string]*trafficData{
			"flow-nc1": {frameSize: 1000, trafficRate: 1, dscp: 56, queue: "7"},
			"flow-af4": {frameSize: 400, trafficRate: 3, dscp: 32, queue: "4"},
			"flow-af3": {frameSize: 300, trafficRate: 2, dscp: 24, queue: "3"},
			"flow-af2": {frameSize: 200, trafficRate: 2, dscp: 16, queue: "2"},
			"flow-af1": {frameSize: 1100, trafficRate: 1, dscp: 8, queue: "0"},
			"flow-be1": {frameSize: 1200, trafficRate: 1, dscp: 0, queue: "1"},
		}
	default:
		t.Fatalf("Output queue mapping is missing for %v", dut.Vendor().String())
	}

	var flows []*ondatra.Flow
	for trafficID, data := range trafficFlows {
		t.Logf("Configuring flow %s", trafficID)
		flow := ate.Traffic().NewFlow(trafficID).
			WithSrcEndpoints(intf1).
			WithDstEndpoints(intf2).
			WithHeaders(ondatra.NewEthernetHeader(), ondatra.NewIPv4Header().WithDSCP(data.dscp)).
			WithFrameRatePct(data.trafficRate).
			WithFrameSize(data.frameSize)
		flows = append(flows, flow)
	}

	ateOutPkts := make(map[string]uint64)
	dutQosPktsBeforeTraffic := make(map[string]uint64)
	dutQosPktsAfterTraffic := make(map[string]uint64)

	// Get QoS egress packet counters before the traffic.
	for _, data := range trafficFlows {
		dutQosPktsBeforeTraffic[data.queue] = gnmi.Get(t, dut, gnmi.OC().Qos().Interface(dp2.Name()).Output().Queue(data.queue).TransmitPkts().State())
	}

	t.Logf("Running traffic on DUT interfaces: %s and %s ", dp1.Name(), dp2.Name())
	ate.Traffic().Start(t, flows...)
	time.Sleep(10 * time.Second)
	ate.Traffic().Stop(t)
	time.Sleep(30 * time.Second)

	for trafficID, data := range trafficFlows {
		ateOutPkts[data.queue] = gnmi.Get(t, ate, gnmi.OC().Flow(trafficID).Counters().OutPkts().State())
		t.Logf("ateOutPkts: %v, txPkts %v, Queue: %v", ateOutPkts[data.queue], dutQosPktsAfterTraffic[data.queue], data.queue)
		t.Logf("Get(out packets for queue %q): got %v", data.queue, ateOutPkts[data.queue])

		lossPct := gnmi.Get(t, ate, gnmi.OC().Flow(trafficID).LossPct().State())
		if lossPct >= 1 {
			t.Errorf("Get(traffic loss for queue %q): got %v, want < 1", data.queue, lossPct)
		}
	}

	for trafficID, data := range trafficFlows {
		ateOutPkts[data.queue] = gnmi.Get(t, ate, gnmi.OC().Flow(trafficID).Counters().OutPkts().State())
		dutQosPktsAfterTraffic[data.queue] = gnmi.Get(t, dut, gnmi.OC().Qos().Interface(dp2.Name()).Output().Queue(data.queue).TransmitPkts().State())
		t.Logf("ateOutPkts: %v, txPkts %v, Queue: %v", ateOutPkts[data.queue], dutQosPktsAfterTraffic[data.queue], data.queue)
		t.Logf("Get(out packets for flow %q): got %v, want nonzero", trafficID, ateOutPkts)

		lossPct := gnmi.Get(t, ate, gnmi.OC().Flow(trafficID).LossPct().State())
		if lossPct >= 1 {
			t.Errorf("Get(traffic loss for queue %q: got %v, want < 1", data.queue, lossPct)
		}
	}

	// Check QoS egress packet counters are updated correctly.
	t.Logf("QoS egress packet counters before traffic: %v", dutQosPktsBeforeTraffic)
	t.Logf("QoS egress packet counters after traffic: %v", dutQosPktsAfterTraffic)
	t.Logf("QoS packet counters from ATE: %v", ateOutPkts)
	for _, data := range trafficFlows {
		qosCounterDiff := dutQosPktsAfterTraffic[data.queue] - dutQosPktsBeforeTraffic[data.queue]
		if qosCounterDiff < ateOutPkts[data.queue] {
			t.Errorf("Get(telemetry packet update for queue %q): got %v, want >= %v", data.queue, qosCounterDiff, ateOutPkts[data.queue])
		}
	}
}

func ConfigureDUTIntf(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")

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
		desc:      "Output interface port2",
		intfName:  dp2.Name(),
		ipAddr:    "198.51.100.2",
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
		if *deviations.InterfaceEnabled {
			s.Enabled = ygot.Bool(true)
		}
		a := s.GetOrCreateAddress(intf.ipAddr)
		a.PrefixLength = ygot.Uint8(intf.prefixLen)
		gnmi.Replace(t, dut, gnmi.OC().Interface(intf.intfName).Config(), i)
	}
}
