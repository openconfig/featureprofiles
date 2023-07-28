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

package ingress_traffic_classification_test

import (
	"context"
	"fmt"
	"strings"
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
	trafficRate float64
	frameSize   uint32
	codePoint   uint8
	queue       string
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Test cases:
//
// Verify the presence of the telemetry paths of the following features:
//  -  Configure DUT with ingress and egress routed interfaces.
//  -  Configure QOS classifiers to match packets arriving on DUT ingress interface to corresponding forwarding class according to
//     classification table.
//  -  Configure packet re-marking for configured classes according to the marking table.
//  -  One-by-one send flows containing every TOS/TC/EXP value in the classification table.
//  -  For every flow sent, verify match-packets counters on the DUT ingress interface
//  -  Verify packet markings on ATE ingress interface.
//  -  Verify that no traffic drops in all flows
//  - Check the QoS queue counters exist and are updated correctly
//    - /qos/interfaces/interface/output/queues/queue/state/transmit-pkts
//    - /qos/interfaces/interface/output/queues/queue/state/transmit-octets
//    - /qos/interfaces/interface/output/queues/queue/state/dropped-pkts
//    - /qos/interfaces/interface/output/queues/queue/state/dropped-octets
//    - /qos/interfaces/interface/input/classifiers/classifier/terms/term/state/matched-packets
//    - /qos/interfaces/interface/input/classifiers/classifier/terms/term/state/matched-octets
//
// Topology:
//   ate:port1 <--> port1:dut:port2 <--> ate:port2
//
//  Sample CLI command to get telemetry using gnmic:
//   - gnmic -a ipaddr:50051 -u username -p password --skip-verify get \
//      --path /components/component --format flat
//   - gnmic tool info:
//     - https://github.com/karimra/gnmic/blob/main/README.md
//

func TestQoSClassifier(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")

	// Configure DUT interfaces and QoS.
	configureDUTIntf(t, dut)
	configureQoS(t, dut)

	// Configure ATE interfaces.
	ate := ondatra.ATE(t, "ate")
	ap1 := ate.Port(t, "port1")
	ap2 := ate.Port(t, "port2")
	top := ate.Topology().New()
	intf1 := top.AddInterface("intf1").WithPort(ap1)
	intf1.IPv4().
		WithAddress("198.51.100.1/31").
		WithDefaultGateway("198.51.100.0")
	intf1.IPv6().
		WithAddress("2001:db8::2/126").
		WithDefaultGateway("2001:db8::1")
	intf2 := top.AddInterface("intf2").WithPort(ap2)
	intf2.IPv4().
		WithAddress("198.51.100.3/31").
		WithDefaultGateway("198.51.100.2")
	intf2.IPv6().
		WithAddress("2001:db8::6/126").
		WithDefaultGateway("2001:db8::5")
	top.Push(t).StartProtocols(t)

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

	ipv4TrafficFlow1 := map[string]*trafficData{
		"flow-ipv4-nc1": {frameSize: 1000, trafficRate: 1, codePoint: 6, queue: qos.nc1},
		"flow-ipv4-af4": {frameSize: 400, trafficRate: 4, codePoint: 4, queue: qos.af4},
		"flow-ipv4-af3": {frameSize: 300, trafficRate: 3, codePoint: 3, queue: qos.af3},
		"flow-ipv4-af2": {frameSize: 200, trafficRate: 2, codePoint: 2, queue: qos.af2},
		"flow-ipv4-af1": {frameSize: 1100, trafficRate: 1, codePoint: 1, queue: qos.af1},
		"flow-ipv4-be1": {frameSize: 1200, trafficRate: 1, codePoint: 0, queue: qos.be1},
	}
	ipv4TrafficFlow2 := map[string]*trafficData{
		"flow-ipv4-nc1": {frameSize: 1000, trafficRate: 1, codePoint: 7, queue: qos.nc1},
		"flow-ipv4-af4": {frameSize: 400, trafficRate: 4, codePoint: 5, queue: qos.af4},
		"flow-ipv4-af3": {frameSize: 300, trafficRate: 3, codePoint: 3, queue: qos.af3},
		"flow-ipv4-af2": {frameSize: 200, trafficRate: 2, codePoint: 2, queue: qos.af2},
		"flow-ipv4-af1": {frameSize: 1100, trafficRate: 1, codePoint: 1, queue: qos.af1},
		"flow-ipv4-be1": {frameSize: 1200, trafficRate: 1, codePoint: 0, queue: qos.be1},
	}
	ipv6TrafficFlow1 := map[string]*trafficData{
		"flow-ipv6-nc1": {frameSize: 1000, trafficRate: 1, codePoint: 48, queue: qos.nc1},
		"flow-ipv6-af4": {frameSize: 400, trafficRate: 4, codePoint: 32, queue: qos.af4},
		"flow-ipv6-af3": {frameSize: 300, trafficRate: 3, codePoint: 24, queue: qos.af3},
		"flow-ipv6-af2": {frameSize: 200, trafficRate: 2, codePoint: 16, queue: qos.af2},
		"flow-ipv6-af1": {frameSize: 1100, trafficRate: 1, codePoint: 8, queue: qos.af1},
		"flow-ipv6-be1": {frameSize: 1200, trafficRate: 1, codePoint: 0, queue: qos.be1},
	}
	ipv6TrafficFlow2 := map[string]*trafficData{
		"flow-ipv6-nc1": {frameSize: 1000, trafficRate: 1, codePoint: 49, queue: qos.nc1},
		"flow-ipv6-af4": {frameSize: 400, trafficRate: 4, codePoint: 33, queue: qos.af4},
		"flow-ipv6-af3": {frameSize: 300, trafficRate: 3, codePoint: 25, queue: qos.af3},
		"flow-ipv6-af2": {frameSize: 200, trafficRate: 2, codePoint: 17, queue: qos.af2},
		"flow-ipv6-af1": {frameSize: 1100, trafficRate: 1, codePoint: 9, queue: qos.af1},
		"flow-ipv6-be1": {frameSize: 1200, trafficRate: 1, codePoint: 1, queue: qos.be1},
	}
	ipv6TrafficFlow3 := map[string]*trafficData{
		"flow-ipv6-nc1": {frameSize: 1000, trafficRate: 1, codePoint: 50, queue: qos.nc1},
		"flow-ipv6-af4": {frameSize: 400, trafficRate: 4, codePoint: 34, queue: qos.af4},
		"flow-ipv6-af3": {frameSize: 300, trafficRate: 3, codePoint: 26, queue: qos.af3},
		"flow-ipv6-af2": {frameSize: 200, trafficRate: 2, codePoint: 18, queue: qos.af2},
		"flow-ipv6-af1": {frameSize: 1100, trafficRate: 1, codePoint: 10, queue: qos.af1},
		"flow-ipv6-be1": {frameSize: 1200, trafficRate: 1, codePoint: 2, queue: qos.be1},
	}
	ipv6TrafficFlow4 := map[string]*trafficData{
		"flow-ipv6-nc1": {frameSize: 1000, trafficRate: 1, codePoint: 51, queue: qos.nc1},
		"flow-ipv6-af4": {frameSize: 400, trafficRate: 4, codePoint: 35, queue: qos.af4},
		"flow-ipv6-af3": {frameSize: 300, trafficRate: 3, codePoint: 27, queue: qos.af3},
		"flow-ipv6-af2": {frameSize: 200, trafficRate: 2, codePoint: 19, queue: qos.af2},
		"flow-ipv6-af1": {frameSize: 1100, trafficRate: 1, codePoint: 11, queue: qos.af1},
		"flow-ipv6-be1": {frameSize: 1200, trafficRate: 1, codePoint: 3, queue: qos.be1},
	}
	ipv6TrafficFlow5 := map[string]*trafficData{
		"flow-ipv6-nc1": {frameSize: 1000, trafficRate: 1, codePoint: 52, queue: qos.nc1},
		"flow-ipv6-af4": {frameSize: 400, trafficRate: 4, codePoint: 36, queue: qos.af4},
		"flow-ipv6-af3": {frameSize: 300, trafficRate: 3, codePoint: 28, queue: qos.af3},
		"flow-ipv6-af2": {frameSize: 200, trafficRate: 2, codePoint: 20, queue: qos.af2},
		"flow-ipv6-af1": {frameSize: 1100, trafficRate: 1, codePoint: 12, queue: qos.af1},
		"flow-ipv6-be1": {frameSize: 1200, trafficRate: 1, codePoint: 4, queue: qos.be1},
	}
	ipv6TrafficFlow6 := map[string]*trafficData{
		"flow-ipv6-nc1": {frameSize: 1000, trafficRate: 1, codePoint: 53, queue: qos.nc1},
		"flow-ipv6-af4": {frameSize: 400, trafficRate: 4, codePoint: 37, queue: qos.af4},
		"flow-ipv6-af3": {frameSize: 300, trafficRate: 3, codePoint: 29, queue: qos.af3},
		"flow-ipv6-af2": {frameSize: 200, trafficRate: 2, codePoint: 21, queue: qos.af2},
		"flow-ipv6-af1": {frameSize: 1100, trafficRate: 1, codePoint: 13, queue: qos.af1},
		"flow-ipv6-be1": {frameSize: 1200, trafficRate: 1, codePoint: 5, queue: qos.be1},
	}
	ipv6TrafficFlow7 := map[string]*trafficData{
		"flow-ipv6-nc1": {frameSize: 1000, trafficRate: 1, codePoint: 54, queue: qos.nc1},
		"flow-ipv6-af4": {frameSize: 400, trafficRate: 4, codePoint: 38, queue: qos.af4},
		"flow-ipv6-af3": {frameSize: 300, trafficRate: 3, codePoint: 30, queue: qos.af3},
		"flow-ipv6-af2": {frameSize: 200, trafficRate: 2, codePoint: 22, queue: qos.af2},
		"flow-ipv6-af1": {frameSize: 1100, trafficRate: 1, codePoint: 14, queue: qos.af1},
		"flow-ipv6-be1": {frameSize: 1200, trafficRate: 1, codePoint: 6, queue: qos.be1},
	}
	ipv6TrafficFlow8 := map[string]*trafficData{
		"flow-ipv6-nc1": {frameSize: 1000, trafficRate: 1, codePoint: 55, queue: qos.nc1},
		"flow-ipv6-af4": {frameSize: 400, trafficRate: 4, codePoint: 39, queue: qos.af4},
		"flow-ipv6-af3": {frameSize: 300, trafficRate: 3, codePoint: 31, queue: qos.af3},
		"flow-ipv6-af2": {frameSize: 200, trafficRate: 2, codePoint: 23, queue: qos.af2},
		"flow-ipv6-af1": {frameSize: 1100, trafficRate: 1, codePoint: 15, queue: qos.af1},
		"flow-ipv6-be1": {frameSize: 1200, trafficRate: 1, codePoint: 7, queue: qos.be1},
	}
	ipv6TrafficFlow9 := map[string]*trafficData{
		"flow-ipv6-nc1": {frameSize: 1000, trafficRate: 1, codePoint: 56, queue: qos.nc1},
		"flow-ipv6-af4": {frameSize: 400, trafficRate: 4, codePoint: 40, queue: qos.af4},
		"flow-ipv6-af3": {frameSize: 300, trafficRate: 3, codePoint: 31, queue: qos.af3},
		"flow-ipv6-af2": {frameSize: 200, trafficRate: 2, codePoint: 23, queue: qos.af2},
		"flow-ipv6-af1": {frameSize: 1100, trafficRate: 1, codePoint: 15, queue: qos.af1},
		"flow-ipv6-be1": {frameSize: 1200, trafficRate: 1, codePoint: 7, queue: qos.be1},
	}
	ipv6TrafficFlow10 := map[string]*trafficData{
		"flow-ipv6-nc1": {frameSize: 1000, trafficRate: 1, codePoint: 57, queue: qos.nc1},
		"flow-ipv6-af4": {frameSize: 400, trafficRate: 4, codePoint: 41, queue: qos.af4},
		"flow-ipv6-af3": {frameSize: 300, trafficRate: 3, codePoint: 31, queue: qos.af3},
		"flow-ipv6-af2": {frameSize: 200, trafficRate: 2, codePoint: 23, queue: qos.af2},
		"flow-ipv6-af1": {frameSize: 1100, trafficRate: 1, codePoint: 15, queue: qos.af1},
		"flow-ipv6-be1": {frameSize: 1200, trafficRate: 1, codePoint: 7, queue: qos.be1},
	}
	ipv6TrafficFlow11 := map[string]*trafficData{
		"flow-ipv6-nc1": {frameSize: 1000, trafficRate: 1, codePoint: 58, queue: qos.nc1},
		"flow-ipv6-af4": {frameSize: 400, trafficRate: 4, codePoint: 42, queue: qos.af4},
		"flow-ipv6-af3": {frameSize: 300, trafficRate: 3, codePoint: 31, queue: qos.af3},
		"flow-ipv6-af2": {frameSize: 200, trafficRate: 2, codePoint: 23, queue: qos.af2},
		"flow-ipv6-af1": {frameSize: 1100, trafficRate: 1, codePoint: 15, queue: qos.af1},
		"flow-ipv6-be1": {frameSize: 1200, trafficRate: 1, codePoint: 7, queue: qos.be1},
	}
	ipv6TrafficFlow12 := map[string]*trafficData{
		"flow-ipv6-nc1": {frameSize: 1000, trafficRate: 1, codePoint: 59, queue: qos.nc1},
		"flow-ipv6-af4": {frameSize: 400, trafficRate: 4, codePoint: 43, queue: qos.af4},
		"flow-ipv6-af3": {frameSize: 300, trafficRate: 3, codePoint: 31, queue: qos.af3},
		"flow-ipv6-af2": {frameSize: 200, trafficRate: 2, codePoint: 23, queue: qos.af2},
		"flow-ipv6-af1": {frameSize: 1100, trafficRate: 1, codePoint: 15, queue: qos.af1},
		"flow-ipv6-be1": {frameSize: 1200, trafficRate: 1, codePoint: 7, queue: qos.be1},
	}
	ipv6TrafficFlow13 := map[string]*trafficData{
		"flow-ipv6-nc1": {frameSize: 1000, trafficRate: 1, codePoint: 60, queue: qos.nc1},
		"flow-ipv6-af4": {frameSize: 400, trafficRate: 4, codePoint: 44, queue: qos.af4},
		"flow-ipv6-af3": {frameSize: 300, trafficRate: 3, codePoint: 31, queue: qos.af3},
		"flow-ipv6-af2": {frameSize: 200, trafficRate: 2, codePoint: 23, queue: qos.af2},
		"flow-ipv6-af1": {frameSize: 1100, trafficRate: 1, codePoint: 15, queue: qos.af1},
		"flow-ipv6-be1": {frameSize: 1200, trafficRate: 1, codePoint: 7, queue: qos.be1},
	}
	ipv6TrafficFlow14 := map[string]*trafficData{
		"flow-ipv6-nc1": {frameSize: 1000, trafficRate: 1, codePoint: 61, queue: qos.nc1},
		"flow-ipv6-af4": {frameSize: 400, trafficRate: 4, codePoint: 45, queue: qos.af4},
		"flow-ipv6-af3": {frameSize: 300, trafficRate: 3, codePoint: 31, queue: qos.af3},
		"flow-ipv6-af2": {frameSize: 200, trafficRate: 2, codePoint: 23, queue: qos.af2},
		"flow-ipv6-af1": {frameSize: 1100, trafficRate: 1, codePoint: 15, queue: qos.af1},
		"flow-ipv6-be1": {frameSize: 1200, trafficRate: 1, codePoint: 7, queue: qos.be1},
	}
	ipv6TrafficFlow15 := map[string]*trafficData{
		"flow-ipv6-nc1": {frameSize: 1000, trafficRate: 1, codePoint: 62, queue: qos.nc1},
		"flow-ipv6-af4": {frameSize: 400, trafficRate: 4, codePoint: 46, queue: qos.af4},
		"flow-ipv6-af3": {frameSize: 300, trafficRate: 3, codePoint: 31, queue: qos.af3},
		"flow-ipv6-af2": {frameSize: 200, trafficRate: 2, codePoint: 23, queue: qos.af2},
		"flow-ipv6-af1": {frameSize: 1100, trafficRate: 1, codePoint: 15, queue: qos.af1},
		"flow-ipv6-be1": {frameSize: 1200, trafficRate: 1, codePoint: 7, queue: qos.be1},
	}
	ipv6TrafficFlow16 := map[string]*trafficData{
		"flow-ipv6-nc1": {frameSize: 1000, trafficRate: 1, codePoint: 63, queue: qos.nc1},
		"flow-ipv6-af4": {frameSize: 400, trafficRate: 4, codePoint: 47, queue: qos.af4},
		"flow-ipv6-af3": {frameSize: 300, trafficRate: 3, codePoint: 31, queue: qos.af3},
		"flow-ipv6-af2": {frameSize: 200, trafficRate: 2, codePoint: 23, queue: qos.af2},
		"flow-ipv6-af1": {frameSize: 1100, trafficRate: 1, codePoint: 15, queue: qos.af1},
		"flow-ipv6-be1": {frameSize: 1200, trafficRate: 1, codePoint: 7, queue: qos.be1},
	}
	mplsTrafficFlow1 := map[string]*trafficData{
		"flow-mpls-nc1": {frameSize: 1000, trafficRate: 1, codePoint: 6, queue: qos.nc1},
		"flow-mpls-af4": {frameSize: 400, trafficRate: 4, codePoint: 4, queue: qos.af4},
		"flow-mpls-af3": {frameSize: 300, trafficRate: 3, codePoint: 3, queue: qos.af3},
		"flow-mpls-af2": {frameSize: 200, trafficRate: 2, codePoint: 2, queue: qos.af2},
		"flow-mpls-af1": {frameSize: 1100, trafficRate: 1, codePoint: 1, queue: qos.af1},
		"flow-mpls-be1": {frameSize: 1200, trafficRate: 1, codePoint: 0, queue: qos.be1},
	}

	cases := []struct {
		desc         string
		trafficFlows map[string]*trafficData
	}{{
		desc:         "IPv4 Traffic flow 1",
		trafficFlows: ipv4TrafficFlow1,
	}, {
		desc:         "IPv4 Traffic flow 2",
		trafficFlows: ipv4TrafficFlow2,
	}, {
		desc:         "IPv6 Traffic flow 1",
		trafficFlows: ipv6TrafficFlow1,
	}, {
		desc:         "IPv6 Traffic flow 2",
		trafficFlows: ipv6TrafficFlow2,
	}, {
		desc:         "IPv6 Traffic flow 3",
		trafficFlows: ipv6TrafficFlow3,
	}, {
		desc:         "IPv6 Traffic flow 4",
		trafficFlows: ipv6TrafficFlow4,
	}, {
		desc:         "IPv6 Traffic flow 5",
		trafficFlows: ipv6TrafficFlow5,
	}, {
		desc:         "IPv6 Traffic flow 6",
		trafficFlows: ipv6TrafficFlow6,
	}, {
		desc:         "IPv6 Traffic flow 7",
		trafficFlows: ipv6TrafficFlow7,
	}, {
		desc:         "IPv6 Traffic flow 8",
		trafficFlows: ipv6TrafficFlow8,
	}, {
		desc:         "IPv6 Traffic flow 9",
		trafficFlows: ipv6TrafficFlow9,
	}, {
		desc:         "IPv6 Traffic flow 10",
		trafficFlows: ipv6TrafficFlow10,
	}, {
		desc:         "IPv6 Traffic flow 11",
		trafficFlows: ipv6TrafficFlow11,
	}, {
		desc:         "IPv6 Traffic flow 12",
		trafficFlows: ipv6TrafficFlow12,
	}, {
		desc:         "IPv6 Traffic flow 13",
		trafficFlows: ipv6TrafficFlow13,
	}, {
		desc:         "IPv6 Traffic flow 14",
		trafficFlows: ipv6TrafficFlow14,
	}, {
		desc:         "IPv6 Traffic flow 15",
		trafficFlows: ipv6TrafficFlow15,
	}, {
		desc:         "IPv6 Traffic flow 16",
		trafficFlows: ipv6TrafficFlow16,
	}, {
		desc:         "MPLS Traffic flow 1",
		trafficFlows: mplsTrafficFlow1,
	}}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			trafficFlows := tc.trafficFlows
			var flows []*ondatra.Flow
			for trafficID, data := range trafficFlows {
				t.Logf("Configuring flow %s", trafficID)
				if strings.Contains(trafficID, "ipv4") {
					flow := ate.Traffic().NewFlow(trafficID).
						WithSrcEndpoints(intf1).
						WithDstEndpoints(intf2).
						WithHeaders(ondatra.NewEthernetHeader(), ondatra.NewIPv4Header().WithDSCP(data.codePoint)).
						WithFrameRatePct(data.trafficRate).
						WithFrameSize(data.frameSize)
					flows = append(flows, flow)
				} else if strings.Contains(trafficID, "ipv6") {
					flow := ate.Traffic().NewFlow(trafficID).
						WithSrcEndpoints(intf1).
						WithDstEndpoints(intf2).
						WithHeaders(ondatra.NewEthernetHeader(), ondatra.NewIPv6Header().WithDSCP(data.codePoint)).
						WithFrameRatePct(data.trafficRate).
						WithFrameSize(data.frameSize)
					flows = append(flows, flow)
				} else if strings.Contains(trafficID, "mpls") {
					flow := ate.Traffic().NewFlow(trafficID).
						WithSrcEndpoints(intf1).
						WithDstEndpoints(intf2).
						WithHeaders(ondatra.NewEthernetHeader(), ondatra.NewIPv4Header().WithDSCP(data.codePoint)).
						WithFrameRatePct(data.trafficRate).
						WithFrameSize(data.frameSize)
					flows = append(flows, flow)
				}
			}

			counters := make(map[string]map[string]uint64)

			counterNames := []string{
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
			for _, data := range trafficFlows {
				counters["dutQosPktsBeforeTraffic"][data.queue] = gnmi.Get(t, dut, gnmi.OC().Qos().Interface(dp2.Name()).Output().Queue(data.queue).TransmitPkts().State())
				counters["dutQosPktsBeforeTraffic"][data.queue] = gnmi.Get(t, dut, gnmi.OC().Qos().Interface(dp2.Name()).Output().Queue(data.queue).TransmitPkts().State())
				counters["dutQosDroppedPktsBeforeTraffic"][data.queue] = gnmi.Get(t, dut, gnmi.OC().Qos().Interface(dp2.Name()).Output().Queue(data.queue).DroppedPkts().State())
				counters["dutQosDroppedOctetsBeforeTraffic"][data.queue] = gnmi.Get(t, dut, gnmi.OC().Qos().Interface(dp2.Name()).Output().Queue(data.queue).DroppedOctets().State())
			}
			t.Logf("Running traffic 1 on DUT interfaces: %s => %s ", dp1.Name(), dp2.Name())
			t.Logf("Sending traffic flows: \n%v\n\n", trafficFlows)
			ate.Traffic().Start(t, flows...)
			time.Sleep(120 * time.Second)
			ate.Traffic().Stop(t)
			time.Sleep(30 * time.Second)

			for trafficID, data := range trafficFlows {
				counters["ateOutPkts"][data.queue] += gnmi.Get(t, ate, gnmi.OC().Flow(trafficID).Counters().OutPkts().State())
				counters["ateInPkts"][data.queue] += gnmi.Get(t, ate, gnmi.OC().Flow(trafficID).Counters().InPkts().State())
				counters["dutQosPktsAfterTraffic"][data.queue] = gnmi.Get(t, dut, gnmi.OC().Qos().Interface(dp2.Name()).Output().Queue(data.queue).TransmitPkts().State())
				counters["dutQosOctetsAfterTraffic"][data.queue] = gnmi.Get(t, dut, gnmi.OC().Qos().Interface(dp2.Name()).Output().Queue(data.queue).TransmitOctets().State())
				counters["dutQosDroppedPktsAfterTraffic"][data.queue] = gnmi.Get(t, dut, gnmi.OC().Qos().Interface(dp2.Name()).Output().Queue(data.queue).DroppedPkts().State())
				counters["dutQosDroppedOctetsAfterTraffic"][data.queue] = gnmi.Get(t, dut, gnmi.OC().Qos().Interface(dp2.Name()).Output().Queue(data.queue).DroppedOctets().State())
				t.Logf("ateInPkts: %v, txPkts %v, Queue: %v", counters["ateInPkts"][data.queue], counters["dutQosPktsAfterTraffic"][data.queue], data.queue)

				lossPct := gnmi.Get(t, ate, gnmi.OC().Flow(trafficID).LossPct().State())
				t.Logf("Get flow %q: lossPct: %.2f%% or rxPct: %.2f%%, want: %.2f%%\n\n", data.queue, lossPct, 100.0-lossPct, 100.0)
				if got, want := 100.0-lossPct, float32(100.0); got != want {
					t.Errorf("Get(throughput for queue %q): got %.2f%%, want %.2f%%", data.queue, got, want)
				}
			}

			// Check QoS egress packet counters are updated correctly.
			for _, name := range counterNames {
				t.Logf("QoS %s: %v", name, counters[name])
			}

			for _, data := range trafficFlows {
				dutPktCounterDiff := counters["dutQosPktsAfterTraffic"][data.queue] - counters["dutQosPktsBeforeTraffic"][data.queue]
				atePktCounterDiff := counters["ateInPkts"][data.queue]
				t.Logf("Queue %q: atePktCounterDiff: %v dutPktCounterDiff: %v", data.queue, atePktCounterDiff, dutPktCounterDiff)
				if dutPktCounterDiff < atePktCounterDiff {
					t.Errorf("Get dutPktCounterDiff for queue %q: got %v, want >= %v", data.queue, dutPktCounterDiff, atePktCounterDiff)
				}

				dutDropPktCounterDiff := counters["dutQosDroppedPktsAfterTraffic"][data.queue] - counters["dutQosDroppedPktsBeforeTraffic"][data.queue]
				t.Logf("Queue %q: dutDropPktCounterDiff: %v", data.queue, dutDropPktCounterDiff)
				if dutDropPktCounterDiff != 0 {
					t.Errorf("Get dutDropPktCounterDiff for queue %q: got %v, want 0", data.queue, dutDropPktCounterDiff)
				}

				dutOctetCounterDiff := counters["dutQosOctetsAfterTraffic"][data.queue] - counters["dutQosOctetsBeforeTraffic"][data.queue]
				ateOctetCounterDiff := counters["ateInPkts"][data.queue] * uint64(data.frameSize)
				t.Logf("Queue %q: ateOctetCounterDiff: %v dutOctetCounterDiff: %v", data.queue, ateOctetCounterDiff, dutOctetCounterDiff)
				if dutOctetCounterDiff < ateOctetCounterDiff {
					t.Errorf("Get dutOctetCounterDiff for queue %q: got %v, want >= %v", data.queue, dutOctetCounterDiff, ateOctetCounterDiff)
				}

				dutDropOctetCounterDiff := counters["dutQosDroppedOctetsAfterTraffic"][data.queue] - counters["dutQosDroppedOctetsBeforeTraffic"][data.queue]
				t.Logf("Queue %q: dutDropOctetCounterDiff: %v", data.queue, dutDropOctetCounterDiff)
				if dutDropOctetCounterDiff != 0 {
					t.Errorf("Get dutDropOctetCounterDiff for queue %q: got %v, want 0", data.queue, dutDropOctetCounterDiff)
				}

			}
			if deviations.MatchedPacketsOctetsUnsupported(dut) {
				t.Log("Validate Matched Packets and Octets")
				validateMatchedStats(t, dut, trafficFlows)
			}
		})
	}
}

func configureDUTIntf(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")

	dutIntfs := []struct {
		desc      string
		intfName  string
		ipAddr    string
		prefixLen uint8
		IPv6      string
		IPv6Len   uint8
	}{{
		desc:      "Input interface port1",
		intfName:  dp1.Name(),
		ipAddr:    "198.51.100.0",
		prefixLen: 31,
		IPv6:      "2001:db8::1",
		IPv6Len:   126,
	}, {
		desc:      "Output interface port2",
		intfName:  dp2.Name(),
		ipAddr:    "198.51.100.2",
		prefixLen: 31,
		IPv6:      "2001:db8::5",
		IPv6Len:   126,
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
		i4 := i.GetOrCreateSubinterface(0).GetOrCreateIpv4()
		a := i4.GetOrCreateAddress(intf.ipAddr)
		a.PrefixLength = ygot.Uint8(intf.prefixLen)
		i6 := i.GetOrCreateSubinterface(0).GetOrCreateIpv6()
		b := i6.GetOrCreateAddress(intf.IPv6)
		b.PrefixLength = ygot.Uint8(intf.IPv6Len)
		gnmi.Replace(t, dut, gnmi.OC().Interface(intf.intfName).Config(), i)

		t.Logf("Configure family mpls on interface %s", intf.intfName)

		gnmiClient := dut.RawAPIs().GNMI().Default(t)
		var config string
		t.Logf("Push the CLI config:\n%s", dut.Vendor())
		config = juniperCLI(intf.intfName)
		switch dut.Vendor() {
		case ondatra.JUNIPER:
			config = juniperCLI(intf.intfName)
			t.Logf("Push the CLI config:\n%s", dut.Vendor())
		}
		gpbSetRequest, err := buildCliConfigRequest(config)
		if err != nil {
			t.Fatalf("Cannot build a gNMI SetRequest: %v", err)
		}
		t.Log("gnmiClient Set CLI config")
		if _, err = gnmiClient.Get(context.Background(), gpbSetRequest); err != nil {
			t.Fatalf("gnmiClient.Set() with unexpected error: %v", err)
		}
	}
}

func configureQoS(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")
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
			af1: "4",
			af2: "1",
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
		fwdGroup.SetOutputQueue(tc.queueName)
		gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), q)
	}

	t.Log("Create qos Classifiers config")
	classifiers := []struct {
		desc        string
		name        string
		classType   oc.E_Qos_Classifier_Type
		termID      string
		targetGroup string
		dscpSet     []uint8
		exp         uint8
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
	}, {
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
	}, {
		desc:        "classifier_mpls_be1",
		name:        "exp_based_classifier_mpls",
		classType:   oc.Qos_Classifier_Type_MPLS,
		termID:      "0",
		targetGroup: "target-group-BE1",
		exp:         0,
	}, {
		desc:        "classifier_mpls_af1",
		name:        "exp_based_classifier_mpls",
		classType:   oc.Qos_Classifier_Type_MPLS,
		termID:      "1",
		targetGroup: "target-group-AF1",
		exp:         1,
	}, {
		desc:        "classifier_mpls_af2",
		name:        "exp_based_classifier_mpls",
		classType:   oc.Qos_Classifier_Type_MPLS,
		termID:      "2",
		targetGroup: "target-group-AF2",
		exp:         2,
	}, {
		desc:        "classifier_mpls_af3",
		name:        "exp_based_classifier_mpls",
		classType:   oc.Qos_Classifier_Type_MPLS,
		termID:      "3",
		targetGroup: "target-group-AF3",
		exp:         3,
	}, {
		desc:        "classifier_mpls_af4",
		name:        "exp_based_classifier_mpls",
		classType:   oc.Qos_Classifier_Type_MPLS,
		termID:      "4",
		targetGroup: "target-group-AF4",
		exp:         4,
	}, {
		desc:        "classifier_mpls_af4",
		name:        "exp_based_classifier_mpls",
		classType:   oc.Qos_Classifier_Type_MPLS,
		termID:      "5",
		targetGroup: "target-group-AF4",
		exp:         5,
	}, {
		desc:        "classifier_mpls_nc1",
		name:        "exp_based_classifier_mpls",
		classType:   oc.Qos_Classifier_Type_MPLS,
		termID:      "6",
		targetGroup: "target-group-NC1",
		exp:         6,
	}, {
		desc:        "classifier_mpls_nc1",
		name:        "exp_based_classifier_mpls",
		classType:   oc.Qos_Classifier_Type_MPLS,
		termID:      "7",
		targetGroup: "target-group-NC1",
		exp:         7,
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
		} else if tc.name == "exp_based_classifier_mpls" {
			condition.GetOrCreateMpls().SetTrafficClass(tc.exp)
		}
		gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), q)
	}

	t.Log("Create qos input classifier config")
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
	}}

	t.Logf("Qos input classifier config: %v", classifierIntfs)
	for _, tc := range classifierIntfs {
		i := q.GetOrCreateInterface(tc.intf)
		i.SetInterfaceId(tc.intf)
		i.GetOrCreateInterfaceRef().Interface = ygot.String(dp1.Name())
		i.GetOrCreateInterfaceRef().Subinterface = ygot.Uint32(0)
		c := i.GetOrCreateInput().GetOrCreateClassifier(tc.inputClassifierType)
		c.SetType(tc.inputClassifierType)
		c.SetName(tc.classifier)
		gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), q)
		// Verify the Classifier is applied on interface by checking the telemetry path state values.
		classifier := gnmi.OC().Qos().Interface(dp1.Name()).Input().Classifier(tc.inputClassifierType)
		if got, want := gnmi.Get(t, dut, classifier.Name().State()), tc.classifier; got != want {
			t.Errorf("Classifier name attached to interface %v is got %v, want %v", dp1.Name(), got, want)
		}
		if got, want := gnmi.Get(t, dut, classifier.Type().State()), tc.inputClassifierType; got != want {
			t.Errorf("Classifier type attached to interface %v is got %v, want %v", dp1.Name(), got, want)
		}
	}

	t.Log("Create qos classifier remark config")
	classifierRemarks := []struct {
		desc        string
		name        string
		classType   oc.E_Qos_Classifier_Type
		termID      string
		targetGroup string
		setDSCP     uint8
		setMPLSTc   uint8
	}{{
		desc:        "classifier_ipv4_be1",
		name:        "dscp_based_classifier_ipv4_remark",
		classType:   oc.Qos_Classifier_Type_IPV4,
		termID:      "0",
		targetGroup: "target-group-BE1",
		setDSCP:     0,
	}, {
		desc:        "classifier_ipv4_af1",
		name:        "dscp_based_classifier_ipv4_remark",
		classType:   oc.Qos_Classifier_Type_IPV4,
		termID:      "1",
		targetGroup: "target-group-AF1",
		setDSCP:     1,
	}, {
		desc:        "classifier_ipv4_af2",
		name:        "dscp_based_classifier_ipv4_remark",
		classType:   oc.Qos_Classifier_Type_IPV4,
		termID:      "2",
		targetGroup: "target-group-AF2",
		setDSCP:     2,
	}, {
		desc:        "classifier_ipv4_af3",
		name:        "dscp_based_classifier_ipv4_remark",
		classType:   oc.Qos_Classifier_Type_IPV4,
		termID:      "3",
		targetGroup: "target-group-AF3",
		setDSCP:     3,
	}, {
		desc:        "classifier_ipv4_af4",
		name:        "dscp_based_classifier_ipv4_remark",
		classType:   oc.Qos_Classifier_Type_IPV4,
		termID:      "4",
		targetGroup: "target-group-AF4",
		setDSCP:     4,
	}, {
		desc:        "classifier_ipv4_nc1",
		name:        "dscp_based_classifier_ipv4_remark",
		classType:   oc.Qos_Classifier_Type_IPV4,
		termID:      "5",
		targetGroup: "target-group-NC1",
		setDSCP:     6,
	}, {
		desc:        "classifier_ipv6_be1",
		name:        "dscp_based_classifier_ipv6_remark",
		classType:   oc.Qos_Classifier_Type_IPV6,
		termID:      "0",
		targetGroup: "target-group-BE1",
		setDSCP:     0,
	}, {
		desc:        "classifier_ipv6_af1",
		name:        "dscp_based_classifier_ipv6_remark",
		classType:   oc.Qos_Classifier_Type_IPV6,
		termID:      "1",
		targetGroup: "target-group-AF1",
		setDSCP:     8,
	}, {
		desc:        "classifier_ipv6_af2",
		name:        "dscp_based_classifier_ipv6_remark",
		classType:   oc.Qos_Classifier_Type_IPV6,
		termID:      "2",
		targetGroup: "target-group-AF2",
		setDSCP:     16,
	}, {
		desc:        "classifier_ipv6_af3",
		name:        "dscp_based_classifier_ipv6_remark",
		classType:   oc.Qos_Classifier_Type_IPV6,
		termID:      "3",
		targetGroup: "target-group-AF3",
		setDSCP:     24,
	}, {
		desc:        "classifier_ipv6_af4",
		name:        "dscp_based_classifier_ipv6_remark",
		classType:   oc.Qos_Classifier_Type_IPV6,
		termID:      "4",
		targetGroup: "target-group-AF4",
		setDSCP:     32,
	}, {
		desc:        "classifier_ipv6_nc1",
		name:        "dscp_based_classifier_ipv6_remark",
		classType:   oc.Qos_Classifier_Type_IPV6,
		termID:      "5",
		targetGroup: "target-group-NC1",
		setDSCP:     48,
	}, {
		desc:        "classifier_mpls_be1",
		name:        "exp_based_classifier_mpls_remark",
		classType:   oc.Qos_Classifier_Type_MPLS,
		termID:      "0",
		targetGroup: "target-group-BE1",
		setMPLSTc:   0,
	}, {
		desc:        "classifier_mpls_af1",
		name:        "exp_based_classifier_mpls_remark",
		classType:   oc.Qos_Classifier_Type_MPLS,
		termID:      "1",
		targetGroup: "target-group-AF1",
		setMPLSTc:   1,
	}, {
		desc:        "classifier_mpls_af2",
		name:        "exp_based_classifier_mpls_remark",
		classType:   oc.Qos_Classifier_Type_MPLS,
		termID:      "2",
		targetGroup: "target-group-AF2",
		setMPLSTc:   2,
	}, {
		desc:        "classifier_mpls_af3",
		name:        "exp_based_classifier_mpls_remark",
		classType:   oc.Qos_Classifier_Type_MPLS,
		termID:      "3",
		targetGroup: "target-group-AF3",
		setMPLSTc:   3,
	}, {
		desc:        "classifier_mpls_af4",
		name:        "exp_based_classifier_mpls_remark",
		classType:   oc.Qos_Classifier_Type_MPLS,
		termID:      "4",
		targetGroup: "target-group-AF4",
		setMPLSTc:   4,
	}, {
		desc:        "classifier_mpls_nc1",
		name:        "exp_based_classifier_mpls_remark",
		classType:   oc.Qos_Classifier_Type_MPLS,
		termID:      "5",
		targetGroup: "target-group-NC1",
		setMPLSTc:   6,
	}}
	t.Logf("Qos Classifiers remark config: %v", classifiers)
	for _, tc := range classifierRemarks {
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
		if tc.name == "dscp_based_classifier_ipv4_remark" || tc.name == "dscp_based_classifier_ipv6_remark" {
			action.GetOrCreateRemark().SetSetDscp(tc.setDSCP)
		} else if tc.name == "exp_based_classifier_mpls_remark" {
			action.GetOrCreateRemark().SetSetMplsTc(tc.setMPLSTc)
		}
		gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), q)
	}

	t.Log("Create qos classifier remark on egress interface config")
	classifierRemarkIntfs := []struct {
		desc                string
		intf                string
		inputClassifierType oc.E_Input_Classifier_Type
		classifier          string
	}{{
		desc:                "Input Classifier Type IPV4",
		intf:                dp2.Name(),
		inputClassifierType: oc.Input_Classifier_Type_IPV4,
		classifier:          "dscp_based_classifier_ipv4_remark",
	}, {
		desc:                "Input Classifier Type IPV6",
		intf:                dp2.Name(),
		inputClassifierType: oc.Input_Classifier_Type_IPV6,
		classifier:          "dscp_based_classifier_ipv6_remark",
	}, {
		desc:                "Input Classifier Type MPLS",
		intf:                dp2.Name(),
		inputClassifierType: oc.Input_Classifier_Type_MPLS,
		classifier:          "exp_based_classifier_mpls_remark",
	}}
	t.Logf("Qos classifier remark and egress interface binding config: %v", classifierIntfs)
	for _, tc := range classifierRemarkIntfs {
		e := q.GetOrCreateInterface(tc.intf)
		e.SetInterfaceId(tc.intf)
		e.GetOrCreateInterfaceRef().Interface = ygot.String(dp2.Name())
		e.GetOrCreateInterfaceRef().Subinterface = ygot.Uint32(0)
		c := e.GetOrCreateOutput().GetOrCreateClassifier(tc.inputClassifierType)
		c.SetType(tc.inputClassifierType)
		c.SetName(tc.classifier)
		gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), q)
		// Verify the Classifier  remark is applied on interface by checking the telemetry path state values.
		classifier := gnmi.OC().Qos().Interface(dp2.Name()).Output().Classifier(tc.inputClassifierType)
		if got, want := gnmi.Get(t, dut, classifier.Name().State()), tc.classifier; got != want {
			t.Errorf("Classifier name attached to interface %v is got %v, want %v", dp2.Name(), got, want)
		}
		if got, want := gnmi.Get(t, dut, classifier.Type().State()), tc.inputClassifierType; got != want {
			t.Errorf("Classifier type attached to interface %v is got %v, want %v", dp2.Name(), got, want)
		}
	}
}
func juniperCLI(intf string) string {
	return fmt.Sprintf(`
	interfaces {
		%s {
			unit 0 {
                  family mpls
				}
			}
		}
  `, intf)
}

func buildCliConfigRequest(config string) (*gpb.SetRequest, error) {
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
	return gpbSetRequest, nil
}

func validateMatchedStats(t *testing.T, dut *ondatra.DUTDevice, trafficFlows map[string]*trafficData) {
	t.Helper()
	dp1 := dut.Port(t, "port1")
	ipv4TermRange := [6]string{"0", "1", "2", "3", "4", "5"}
	ipv6TermRange := [6]string{"0", "1", "2", "3", "4", "5"}
	mplsTermRange := [8]string{"0", "1", "2", "3", "4", "5", "6", "7"}
	for trafficID, data := range trafficFlows {
		MatchedPkts := gnmi.Get(t, dut, gnmi.OC().Qos().Interface(dp1.Name()).Input().Queue(data.queue).TransmitPkts().State())
		MatchedOctets := gnmi.Get(t, dut, gnmi.OC().Qos().Interface(dp1.Name()).Output().Queue(data.queue).TransmitOctets().State())
		if strings.Contains(trafficID, "ipv4") {
			for _, i := range ipv4TermRange {
				classifier := gnmi.OC().Qos().Interface(dp1.Name()).Input().Classifier(oc.Input_Classifier_Type_IPV4)
				if got, want := gnmi.Get(t, dut, classifier.Term(i).MatchedPackets().State()), MatchedPkts; got != want {
					t.Errorf("Matched packets for ipv4 classifier of term %v is got %v, want %v", i, got, want)
				}
				if got, want := gnmi.Get(t, dut, classifier.Term(i).MatchedOctets().State()), MatchedOctets; got != want {
					t.Errorf("Matched octets for ipv4 classifier of term %v is got %v, want %v", i, got, want)
				}
			}
		}
		if strings.Contains(trafficID, "ipv6") {
			for _, i := range ipv6TermRange {
				classifier := gnmi.OC().Qos().Interface(dp1.Name()).Input().Classifier(oc.Input_Classifier_Type_IPV6)
				if got, want := gnmi.Get(t, dut, classifier.Term(i).MatchedPackets().State()), MatchedPkts; got != want {
					t.Errorf("Matched packets for ipv6 classifier of term %v is got %v, want %v", i, got, want)
				}
				if got, want := gnmi.Get(t, dut, classifier.Term(i).MatchedOctets().State()), MatchedOctets; got != want {
					t.Errorf("Matched octets for ipv6 classifier of term %v is got %v, want %v", i, got, want)
				}
			}
			if strings.Contains(trafficID, "mpls") {
				for _, i := range mplsTermRange {
					classifier := gnmi.OC().Qos().Interface(dp1.Name()).Input().Classifier(oc.Input_Classifier_Type_MPLS)
					if got, want := gnmi.Get(t, dut, classifier.Term(i).MatchedPackets().State()), MatchedPkts; got != want {
						t.Errorf("Matched packets for mpls classifier of term %v is got %v, want %v", i, got, want)
					}
					if got, want := gnmi.Get(t, dut, classifier.Term(i).MatchedOctets().State()), MatchedOctets; got != want {
						t.Errorf("Matched octets for mpls classifier of term %v is got %v, want %v", i, got, want)
					}
				}
			}
		}
	}
}
