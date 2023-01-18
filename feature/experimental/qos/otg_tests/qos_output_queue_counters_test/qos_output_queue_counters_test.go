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
	"github.com/openconfig/featureprofiles/internal/otgutils"
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
	top := ate.OTG().NewConfig(t)

	top.Ports().Add().SetName(ap1.ID())
	top.Ports().Add().SetName(ap2.ID())

	dev1 := top.Devices().Add().SetName(ateSrcName)
	eth1 := dev1.Ethernets().Add().SetName(dev1.Name() + ".eth")
	eth1.SetPortName(ap1.ID()).SetMac(ateSrcMac)
	eth1.Ipv4Addresses().Add().SetName(dev1.Name() + ".ipv4").SetAddress(ateSrcIp).SetGateway(ateSrcGateway).SetPrefix(int32(prefixLen))

	dev2 := top.Devices().Add().SetName(ateDstName)
	eth2 := dev2.Ethernets().Add().SetName(dev2.Name() + ".eth")
	eth2.SetPortName(ap2.ID()).SetMac(ateDstMac)
	eth2.Ipv4Addresses().Add().SetName(dev2.Name() + ".ipv4").SetAddress(ateDstIp).SetGateway(ateDstGateway).SetPrefix(int32(prefixLen))

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

	for trafficID, data := range trafficFlows {
		t.Logf("Configuring flow %s", trafficID)

		flow := top.Flows().Add().SetName(trafficID)
		flow.Metrics().SetEnable(true)
		flow.TxRx().Device().SetTxNames([]string{dev1.Name() + ".ipv4"}).SetRxNames([]string{dev2.Name() + ".ipv4"})
		ethHeader := flow.Packet().Add().Ethernet()
		ethHeader.Src().SetValue(ateSrcMac)

		ipHeader := flow.Packet().Add().Ipv4()
		ipHeader.Src().SetValue(ateSrcIp)
		ipHeader.Dst().SetValue(ateDstIp)
		ipHeader.Priority().Dscp().Phb().SetValue(int32(data.dscp))

		flow.Size().SetFixed(int32(data.frameSize))
		flow.Rate().SetPercentage(float32(data.trafficRate))
		flow.Duration().FixedPackets().SetPackets(10000)
	}

	ateOutPkts := make(map[string]uint64)
	dutQosPktsBeforeTraffic := make(map[string]uint64)
	dutQosPktsAfterTraffic := make(map[string]uint64)

	// Get QoS egress packet counters before the traffic.
	for _, data := range trafficFlows {
		dutQosPktsBeforeTraffic[data.queue] = gnmi.Get(t, dut, gnmi.OC().Qos().Interface(dp2.Name()).Output().Queue(data.queue).TransmitPkts().State())
	}

	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)

	time.Sleep(30 * time.Second)
	t.Logf("Running traffic on DUT interfaces: %s and %s ", dp1.Name(), dp2.Name())
	ate.OTG().StartTraffic(t)
	time.Sleep(10 * time.Second)
	ate.OTG().StopTraffic(t)
	time.Sleep(30 * time.Second)

	otgutils.LogFlowMetrics(t, ate.OTG(), top)
	for trafficID, data := range trafficFlows {
		ateOutPkts[data.queue] = gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(trafficID).Counters().OutPkts().State())
		t.Logf("ateOutPkts: %v, txPkts %v, Queue: %v", ateOutPkts[data.queue], dutQosPktsAfterTraffic[data.queue], data.queue)
		t.Logf("Get(out packets for queue %q): got %v", data.queue, ateOutPkts[data.queue])

		ateTxPkts := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(trafficID).Counters().OutPkts().State())
		ateRxPkts := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(trafficID).Counters().InPkts().State())
		lossPct := ((ateTxPkts - ateRxPkts) * 100) / ateTxPkts

		if lossPct >= 1 {
			t.Errorf("Get(traffic loss for queue %q): got %v, want < 1", data.queue, lossPct)
		}
	}

	for trafficID, data := range trafficFlows {
		ateOutPkts[data.queue] = gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(trafficID).Counters().OutPkts().State())
		dutQosPktsAfterTraffic[data.queue] = gnmi.Get(t, dut, gnmi.OC().Qos().Interface(dp2.Name()).Output().Queue(data.queue).TransmitPkts().State())
		t.Logf("ateOutPkts: %v, txPkts %v, Queue: %v", ateOutPkts[data.queue], dutQosPktsAfterTraffic[data.queue], data.queue)
		t.Logf("Get(out packets for flow %q): got %v, want nonzero", trafficID, ateOutPkts)

		ateTxPkts := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(trafficID).Counters().OutPkts().State())
		ateRxPkts := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(trafficID).Counters().InPkts().State())
		lossPct := ((ateTxPkts - ateRxPkts) * 100) / ateTxPkts

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
		if *deviations.InterfaceEnabled && !*deviations.IPv4MissingEnabled {
			s.Enabled = ygot.Bool(true)
		}
		a := s.GetOrCreateAddress(intf.ipAddr)
		a.PrefixLength = ygot.Uint8(intf.prefixLen)
		gnmi.Replace(t, dut, gnmi.OC().Interface(intf.intfName).Config(), i)
	}
}
