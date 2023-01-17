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

package mixed_sp_wrr_traffic_test

import (
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

var (
	ate1 = attrs.Attributes{
		Desc:    "ate1",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "198.51.100.1",
		IPv4Len: 31,
	}

	ate2 = attrs.Attributes{
		Name:    "ate2",
		MAC:     "02:00:01:01:01:02",
		IPv4:    "198.51.100.3",
		IPv4Len: 31,
	}

	ate3 = attrs.Attributes{
		Desc:    "ate3",
		MAC:     "02:00:01:01:01:03",
		IPv4:    "198.51.100.5",
		IPv4Len: 31,
	}
)

type trafficData struct {
	trafficRate float64
	frameSize   uint32
	dscp        uint8
	queue       string
	ateAttrs    attrs.Attributes
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Test cases:
//  - https://github.com/openconfig/featureprofiles/blob/main/feature/qos/ate_tests/mixed_sp_wrr_traffic_test/README.md
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

func TestQoSCounters(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")
	dp3 := dut.Port(t, "port3")

	// Configure DUT interfaces.
	ConfigureDUTIntf(t, dut)

	// Configure ATE interfaces.
	ate := ondatra.ATE(t, "ate")
	ap1 := ate.Port(t, "port1")
	ap2 := ate.Port(t, "port2")
	ap3 := ate.Port(t, "port3")
	top := ate.OTG().NewConfig(t)

	top.Ports().Add().SetName(ap1.ID())
	top.Ports().Add().SetName(ap2.ID())
	top.Ports().Add().SetName(ap3.ID())

	dev1 := top.Devices().Add().SetName(ate1.Name)
	eth1 := dev1.Ethernets().Add().SetName(dev1.Name() + ".eth")
	eth1.SetPortName(ap1.ID()).SetMac(ate1.MAC)
	eth1.Ipv4Addresses().Add().SetName(dev1.Name() + ".ipv4").SetAddress(ate1.IPv4).SetGateway("198.51.100.0").SetPrefix(int32(ate1.IPv4Len))

	dev2 := top.Devices().Add().SetName(ate2.Name)
	eth2 := dev2.Ethernets().Add().SetName(dev2.Name() + ".eth")
	eth2.SetPortName(ap2.ID()).SetMac(ate2.MAC)
	eth2.Ipv4Addresses().Add().SetName(dev2.Name() + ".ipv4").SetAddress(ate2.IPv4).SetGateway("198.51.100.2").SetPrefix(int32(ate2.IPv4Len))

	dev3 := top.Devices().Add().SetName(ate3.Name)
	eth3 := dev3.Ethernets().Add().SetName(dev3.Name() + ".eth")
	eth3.SetPortName(ap2.ID()).SetMac(ate3.MAC)
	eth3.Ipv4Addresses().Add().SetName(dev3.Name() + ".ipv4").SetAddress(ate3.IPv4).SetGateway("198.51.100.4").SetPrefix(int32(ate3.IPv4Len))

	// intf1 := top.AddInterface("intf1").WithPort(ap1)
	// intf1.IPv4().
	// 	WithAddress("198.51.100.1/31").
	// 	WithDefaultGateway("198.51.100.0")
	// intf2 := top.AddInterface("intf2").WithPort(ap2)
	// intf2.IPv4().
	// 	WithAddress("198.51.100.3/31").
	// 	WithDefaultGateway("198.51.100.2")
	// intf3 := top.AddInterface("intf3").WithPort(ap3)
	// intf3.IPv4().
	// 	WithAddress("198.51.100.5/31").
	// 	WithDefaultGateway("198.51.100.4")
	// top.Push(t).StartProtocols(t)

	var trafficFlows map[string]*trafficData

	switch dut.Vendor() {
	case ondatra.JUNIPER:
		trafficFlows = map[string]*trafficData{
			"intf1-nc1": {frameSize: 1000, trafficRate: 1, dscp: 56, queue: "3", ateAttrs: ate1},
			"intf1-af4": {frameSize: 400, trafficRate: 4, dscp: 32, queue: "2", ateAttrs: ate1},
			"intf1-af3": {frameSize: 300, trafficRate: 3, dscp: 24, queue: "5", ateAttrs: ate1},
			"intf1-af2": {frameSize: 200, trafficRate: 2, dscp: 16, queue: "1", ateAttrs: ate1},
			"intf1-af1": {frameSize: 1100, trafficRate: 1, dscp: 8, queue: "4", ateAttrs: ate1},
			"intf1-be1": {frameSize: 1200, trafficRate: 1, dscp: 0, queue: "0", ateAttrs: ate1},
			"intf2-nc1": {frameSize: 1000, trafficRate: 2, dscp: 56, queue: "3", ateAttrs: ate2},
			"intf2-af4": {frameSize: 400, trafficRate: 8, dscp: 32, queue: "2", ateAttrs: ate2},
			"intf2-af3": {frameSize: 300, trafficRate: 6, dscp: 24, queue: "5", ateAttrs: ate2},
			"intf2-af2": {frameSize: 200, trafficRate: 3, dscp: 16, queue: "1", ateAttrs: ate2},
			"intf2-af1": {frameSize: 1100, trafficRate: 2, dscp: 8, queue: "4", ateAttrs: ate2},
			"intf2-be1": {frameSize: 1200, trafficRate: 2, dscp: 0, queue: "0", ateAttrs: ate2},
		}
	case ondatra.ARISTA:
		trafficFlows = map[string]*trafficData{
			"intf1-nc1": {frameSize: 700, trafficRate: 0.1, dscp: 56, queue: dp3.Name() + "-7", ateAttrs: ate1},
			"intf1-af4": {frameSize: 400, trafficRate: 18, dscp: 32, queue: dp3.Name() + "-4", ateAttrs: ate1},
			"intf1-af3": {frameSize: 1300, trafficRate: 16, dscp: 24, queue: dp3.Name() + "-3", ateAttrs: ate1},
			"intf1-af2": {frameSize: 1200, trafficRate: 8, dscp: 16, queue: dp3.Name() + "-2", ateAttrs: ate1},
			"intf1-af1": {frameSize: 1000, trafficRate: 4, dscp: 8, queue: dp3.Name() + "-0", ateAttrs: ate1},
			"intf1-be1": {frameSize: 1111, trafficRate: 2, dscp: 0, queue: dp3.Name() + "-1", ateAttrs: ate1},
			"intf1-be0": {frameSize: 1110, trafficRate: 0.5, dscp: 4, queue: dp3.Name() + "-1", ateAttrs: ate1},
			"intf2-nc1": {frameSize: 700, trafficRate: 0.9, dscp: 56, queue: dp3.Name() + "-7", ateAttrs: ate2},
			"intf2-af4": {frameSize: 400, trafficRate: 20, dscp: 32, queue: dp3.Name() + "-4", ateAttrs: ate2},
			"intf2-af3": {frameSize: 1300, trafficRate: 16, dscp: 24, queue: dp3.Name() + "-3", ateAttrs: ate2},
			"intf2-af2": {frameSize: 1200, trafficRate: 8, dscp: 16, queue: dp3.Name() + "-2", ateAttrs: ate2},
			"intf2-af1": {frameSize: 1000, trafficRate: 4, dscp: 8, queue: dp3.Name() + "-0", ateAttrs: ate2},
			"intf2-be1": {frameSize: 1111, trafficRate: 2, dscp: 0, queue: dp3.Name() + "-1", ateAttrs: ate2},
			"intf2-be0": {frameSize: 1112, trafficRate: 0.5, dscp: 5, queue: dp3.Name() + "-1", ateAttrs: ate2},
		}
	case ondatra.CISCO:
		trafficFlows = map[string]*trafficData{
			"intf1-nc1": {frameSize: 1000, trafficRate: 1, dscp: 56, queue: "7", ateAttrs: ate1},
			"intf1-af4": {frameSize: 400, trafficRate: 3, dscp: 32, queue: "4", ateAttrs: ate1},
			"intf1-af3": {frameSize: 300, trafficRate: 2, dscp: 24, queue: "3", ateAttrs: ate1},
			"intf1-af2": {frameSize: 200, trafficRate: 2, dscp: 16, queue: "2", ateAttrs: ate1},
			"intf1-af1": {frameSize: 1100, trafficRate: 1, dscp: 8, queue: "0", ateAttrs: ate1},
			"intf1-be1": {frameSize: 1200, trafficRate: 1, dscp: 0, queue: "1", ateAttrs: ate1},
			"intf2-nc1": {frameSize: 1000, trafficRate: 2, dscp: 56, queue: "7", ateAttrs: ate2},
			"intf2-af4": {frameSize: 400, trafficRate: 6, dscp: 32, queue: "4", ateAttrs: ate2},
			"intf2-af3": {frameSize: 300, trafficRate: 4, dscp: 24, queue: "3", ateAttrs: ate2},
			"intf2-af2": {frameSize: 200, trafficRate: 4, dscp: 16, queue: "2", ateAttrs: ate2},
			"intf2-af1": {frameSize: 1100, trafficRate: 2, dscp: 8, queue: "0", ateAttrs: ate2},
			"intf2-be1": {frameSize: 1200, trafficRate: 2, dscp: 0, queue: "1", ateAttrs: ate2},
		}
	default:
		t.Fatalf("Output queue mapping is missing for %v", dut.Vendor().String())
	}

	for trafficID, data := range trafficFlows {
		t.Logf("Configuring flow %s", trafficID)

		flow := top.Flows().Add().SetName(trafficID)
		flow.Metrics().SetEnable(true)
		flow.TxRx().Device().SetTxNames([]string{data.ateAttrs.Name + ".ipv4"}).SetRxNames([]string{dev3.Name() + ".ipv4"})
		ethHeader := flow.Packet().Add().Ethernet()
		ethHeader.Src().SetValue(data.ateAttrs.MAC)

		ipHeader := flow.Packet().Add().Ipv4()
		ipHeader.Src().SetValue(data.ateAttrs.IPv4)
		ipHeader.Dst().SetValue(ate3.IPv4)
		ipHeader.Priority().Dscp().Phb().SetValue(int32(data.dscp))

		flow.Size().SetFixed(int32(data.frameSize))
		flow.Rate().SetPercentage(float32(data.trafficRate))
		flow.Duration().FixedPackets().SetPackets(10000)

		// flow := ate.Traffic().NewFlow(trafficID).
		// 	WithSrcEndpoints(data.inputIntf).
		// 	WithDstEndpoints(intf3).
		// 	WithHeaders(ondatra.NewEthernetHeader(), ondatra.NewIPv4Header().WithDSCP(data.dscp)).
		// 	WithFrameRatePct(data.trafficRate).
		// 	WithFrameSize(data.frameSize)
		// flows = append(flows, flow)
	}

	ateOutPkts := make(map[string]uint64)
	dutQosPktsBeforeTraffic := make(map[string]uint64)
	dutQosPktsAfterTraffic := make(map[string]uint64)

	// Get QoS egress packet counters before the traffic.
	for _, data := range trafficFlows {
		dutQosPktsBeforeTraffic[data.queue] = gnmi.Get(t, dut, gnmi.OC().Qos().Interface(dp3.Name()).Output().Queue(data.queue).TransmitPkts().State())
	}

	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)

	time.Sleep(30 * time.Second)
	t.Logf("Running traffic 1 on DUT interfaces: %s => %s ", dp1.Name(), dp3.Name())
	t.Logf("Running traffic 2 on DUT interfaces: %s => %s ", dp2.Name(), dp3.Name())
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
		dutQosPktsAfterTraffic[data.queue] = gnmi.Get(t, dut, gnmi.OC().Qos().Interface(dp3.Name()).Output().Queue(data.queue).TransmitPkts().State())
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
