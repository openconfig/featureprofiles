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

package dscp_trust_classification_test

import (
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/featureprofiles/internal/qoscfg"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ygot/ygot"
)

type trafficData struct {
	trafficRate float64
	frameSize   uint32
	codePoint   uint8
	queue       string
}

const (
	ateSrcName    = "dev1"
	ateDstName    = "dev2"
	ateSrcMac     = "02:00:01:01:01:01"
	ateDstMac     = "02:00:01:01:01:02"
	ateSrcIp      = "198.51.100.1"
	ateDstIp      = "198.51.100.3"
	ateSrcGateway = "198.51.100.0"
	ateDstGateway = "198.51.100.2"
	prefixLen     = 31
)

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
	top := gosnappi.NewConfig()

	top.Ports().Add().SetName(ap1.ID())
	top.Ports().Add().SetName(ap2.ID())

	dev1 := top.Devices().Add().SetName("dev1")
	eth1 := dev1.Ethernets().Add().SetName(dev1.Name() + ".eth").SetMac("02:00:01:01:01:01")
	eth1.Connection().SetPortName(ap1.ID())
	eth1.Ipv4Addresses().Add().SetName(dev1.Name() + ".ipv4").SetAddress("198.51.100.1").SetGateway("198.51.100.0").SetPrefix(uint32(31))
	eth1.Ipv6Addresses().Add().SetName(dev1.Name() + ".ipv6").SetAddress("2001:db8::2").SetGateway("2001:db8::1").SetPrefix(uint32(31))

	dev2 := top.Devices().Add().SetName("dev2")
	eth2 := dev2.Ethernets().Add().SetName(dev1.Name() + ".eth").SetMac("02:00:01:01:01:01")
	eth2.Connection().SetPortName(ap1.ID())
	eth2.Ipv4Addresses().Add().SetName(dev1.Name() + ".ipv4").SetAddress("198.51.100.3").SetGateway("198.51.100.2").SetPrefix(uint32(31))
	eth2.Ipv6Addresses().Add().SetName(dev1.Name() + ".ipv6").SetAddress("2001:db8::6").SetGateway("2001:db8::5").SetPrefix(uint32(126))

	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")

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
	}}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			trafficFlows := tc.trafficFlows
			for trafficID, data := range trafficFlows {
				t.Logf("Configuring flow %s", trafficID)
				if strings.Contains(trafficID, "ipv4") {
					flow := top.Flows().Add().SetName(trafficID)
					flow.Metrics().SetEnable(true)
					flow.TxRx().Device().SetTxNames([]string{dev1.Name() + ".ipv4"}).SetRxNames([]string{dev2.Name() + ".ipv4"})
					ethHeader := flow.Packet().Add().Ethernet()
					ethHeader.Src().SetValue(ateSrcMac)
					ipHeader := flow.Packet().Add().Ipv4()
					ipHeader.Src().SetValue(ateSrcIp)
					ipHeader.Dst().SetValue(ateDstIp)
					ipHeader.Priority().Dscp().Phb().SetValue(uint32(data.codePoint))
					flow.Size().SetFixed(uint32(data.frameSize))
					flow.Rate().SetPercentage(float32(data.trafficRate))
					flow.Duration().FixedPackets().SetPackets(10000)

				} else if strings.Contains(trafficID, "ipv6") {
					flow := top.Flows().Add().SetName(trafficID)
					flow.Metrics().SetEnable(true)
					flow.TxRx().Device().SetTxNames([]string{dev1.Name() + ".ipv6"}).SetRxNames([]string{dev2.Name() + ".ipv6"})
					ethHeader := flow.Packet().Add().Ethernet()
					ethHeader.Src().SetValue(ateSrcMac)
					ipHeader := flow.Packet().Add().Ipv6()
					ipHeader.Src().SetValue(ateSrcIp)
					ipHeader.Dst().SetValue(ateDstIp)
					ipHeader.TrafficClass().SetValue(uint32(data.codePoint))
					flow.Size().SetFixed(uint32(data.frameSize))
					flow.Rate().SetPercentage(float32(data.trafficRate))
					flow.Duration().FixedPackets().SetPackets(10000)
				}
			}

			var counterNames []string
			counters := make(map[string]map[string]uint64)

			counterNames = []string{
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
				counters["dutQosVoQDroppedPktsBeforeTraffic"][data.queue] = gnmi.Get(t, dut, gnmi.OC().Qos().Interface(dp2.Name()).Input().VoqInterface(data.queue).Queue(data.queue).DroppedPkts().State())
			}
			t.Logf("Running traffic 1 on DUT interfaces: %s => %s ", dp1.Name(), dp2.Name())
			t.Logf("Sending traffic flows: \n%v\n\n", trafficFlows)
			ate.OTG().StartTraffic(t)
			time.Sleep(120 * time.Second)
			ate.OTG().StopTraffic(t)
			time.Sleep(30 * time.Second)

			otgutils.LogFlowMetrics(t, ate.OTG(), top)
			for trafficID, data := range trafficFlows {
				counters["ateOutPkts"][data.queue] += gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(trafficID).Counters().OutPkts().State())
				counters["ateInPkts"][data.queue] += gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(trafficID).Counters().InPkts().State())
				counters["dutQosPktsAfterTraffic"][data.queue] = gnmi.Get(t, dut, gnmi.OC().Qos().Interface(dp2.Name()).Output().Queue(data.queue).TransmitPkts().State())
				counters["dutQosOctetsAfterTraffic"][data.queue] = gnmi.Get(t, dut, gnmi.OC().Qos().Interface(dp2.Name()).Output().Queue(data.queue).TransmitOctets().State())
				counters["dutQosDroppedPktsAfterTraffic"][data.queue] = gnmi.Get(t, dut, gnmi.OC().Qos().Interface(dp2.Name()).Output().Queue(data.queue).DroppedPkts().State())
				counters["dutQosDroppedOctetsAfterTraffic"][data.queue] = gnmi.Get(t, dut, gnmi.OC().Qos().Interface(dp2.Name()).Output().Queue(data.queue).DroppedOctets().State())
				t.Logf("ateInPkts: %v, txPkts %v, Queue: %v", counters["ateInPkts"][data.queue], counters["dutQosPktsAfterTraffic"][data.queue], data.queue)

				lossPct := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(trafficID).LossPct().State())
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
			for _, data := range trafficFlows {
				counters["dutQosVoQDroppedPktsAfterTraffic"][data.queue] = gnmi.Get(t, dut, gnmi.OC().Qos().Interface(dp2.Name()).Input().VoqInterface(data.queue).Queue(data.queue).DroppedPkts().State())
				dutVoQDroppedPktDiff := counters["dutQosVoQDroppedPktsAfterTraffic"][data.queue] - counters["dutQosVoQDroppedPktsBeforeTraffic"][data.queue]
				if dutVoQDroppedPktDiff == 0 {
					t.Errorf("Get dutVoQDroppedPktDiff for queue %q: got %v, want 0", data.queue, dutVoQDroppedPktDiff)
				}
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
	}
}

func configureQoS(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")
	d := &oc.Root{}
	q := d.GetOrCreateQos()

	queues := netutil.CommonTrafficQueues(t, dut)

	t.Log("Create qos forwarding groups config")
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
		t.Run(tc.desc, func(t *testing.T) {
			qoscfg.SetForwardingGroup(t, dut, q, tc.targetGroup, tc.queueName)
		})
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

	// TO DO - Configure DSCP Trust for other vendors

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

	schedulers := []struct {
		desc        string
		sequence    uint32
		priority    oc.E_Scheduler_Priority
		inputID     string
		weight      uint64
		queueName   string
		targetGroup string
	}{{
		desc:        "scheduler-policy-BE1",
		sequence:    uint32(1),
		priority:    oc.Scheduler_Priority_UNSET,
		inputID:     "BE1",
		weight:      uint64(1),
		queueName:   queues.BE1,
		targetGroup: "BE1",
	}, {
		desc:        "scheduler-policy-BE0",
		sequence:    uint32(1),
		priority:    oc.Scheduler_Priority_UNSET,
		inputID:     "BE0",
		weight:      uint64(2),
		queueName:   queues.BE0,
		targetGroup: "BE0",
	}, {
		desc:        "scheduler-policy-AF1",
		sequence:    uint32(1),
		priority:    oc.Scheduler_Priority_UNSET,
		inputID:     "AF1",
		weight:      uint64(4),
		queueName:   queues.AF1,
		targetGroup: "AF1",
	}, {
		desc:        "scheduler-policy-AF2",
		sequence:    uint32(1),
		priority:    oc.Scheduler_Priority_UNSET,
		inputID:     "AF2",
		weight:      uint64(8),
		queueName:   queues.AF2,
		targetGroup: "AF2",
	}, {
		desc:        "scheduler-policy-AF3",
		sequence:    uint32(1),
		priority:    oc.Scheduler_Priority_UNSET,
		inputID:     "AF3",
		weight:      uint64(16),
		queueName:   queues.AF3,
		targetGroup: "AF3",
	}, {
		desc:        "scheduler-policy-AF4",
		sequence:    uint32(0),
		priority:    oc.Scheduler_Priority_STRICT,
		inputID:     queues.AF4,
		weight:      uint64(99),
		queueName:   queues.AF4,
		targetGroup: "AF4",
	}, {
		desc:        "scheduler-policy-NC1",
		sequence:    uint32(0),
		priority:    oc.Scheduler_Priority_STRICT,
		inputID:     "NC1",
		weight:      uint64(100),
		queueName:   queues.NC1,
		targetGroup: "NC1",
	}}

	schedulerPolicy := q.GetOrCreateSchedulerPolicy("scheduler")
	schedulerPolicy.SetName("scheduler")
	t.Logf("qos scheduler policies config cases: %v", schedulers)
	for _, tc := range schedulers {
		t.Run(tc.desc, func(t *testing.T) {
			qoscfg.SetForwardingGroup(t, dut, q, tc.targetGroup, tc.queueName)
			s := schedulerPolicy.GetOrCreateScheduler(tc.sequence)
			s.SetSequence(tc.sequence)
			s.SetPriority(tc.priority)
			input := s.GetOrCreateInput(tc.inputID)
			input.SetId(tc.inputID)
			input.SetInputType(oc.Input_InputType_QUEUE)
			input.SetQueue(tc.queueName)
			input.SetWeight(tc.weight)
			gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), q)
		})
	}
	t.Logf("Create qos output interface config")
	schedulerIntfs := []struct {
		desc      string
		queueName string
		scheduler string
	}{{
		desc:      "output-interface-NC1",
		queueName: queues.NC1,
		scheduler: "scheduler",
	}, {
		desc:      "output-interface-AF4",
		queueName: queues.AF4,
		scheduler: "scheduler",
	}, {
		desc:      "output-interface-AF3",
		queueName: queues.AF3,
		scheduler: "scheduler",
	}, {
		desc:      "output-interface-AF2",
		queueName: queues.AF2,
		scheduler: "scheduler",
	}, {
		desc:      "output-interface-AF1",
		queueName: queues.AF1,
		scheduler: "scheduler",
	}, {
		desc:      "output-interface-BE1",
		queueName: queues.BE1,
		scheduler: "scheduler",
	}}

	t.Logf("qos output interface config: %v", schedulerIntfs)
	for _, tc := range schedulerIntfs {
		i := q.GetOrCreateInterface(dp2.Name())
		i.SetInterfaceId(dp2.Name())
		i.GetOrCreateInterfaceRef().Interface = ygot.String(dp2.Name())
		output := i.GetOrCreateOutput()
		schedulerPolicy := output.GetOrCreateSchedulerPolicy()
		schedulerPolicy.SetName(tc.scheduler)
		queue := output.GetOrCreateQueue(tc.queueName)
		queue.SetName(tc.queueName)
	}
	gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), q)
}
