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

package telemetry_interface_packet_counters_test

import (
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/gnmi/oc/interfaces"
	otgtelemetry "github.com/openconfig/ondatra/gnmi/otg"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestEthernetCounters(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	dp := dut.Port(t, "port2")
	counters := gnmi.OC().Interface(dp.Name()).Ethernet().Counters()
	ethCounterPath := "/interfaces/interface/ethernet/state/counters/"

	cases := []struct {
		desc    string
		path    string
		counter *ygnmi.Value[uint64]
	}{{
		desc:    "InMacPauseFrames",
		path:    ethCounterPath + "out-mac-pause-frames",
		counter: gnmi.Lookup(t, dut, counters.InMacPauseFrames().State()),
	}, {
		desc:    "OutMacPauseFrames",
		path:    ethCounterPath + "in-mac-pause-frames",
		counter: gnmi.Lookup(t, dut, counters.OutMacPauseFrames().State()),
	}, {
		desc: "InMaxsizeExceeded",
		path: ethCounterPath + "in-maxsize-exceeded",
		// TODO: Uncomment counter in-maxsize-exceeded after the issue fixed.
		// counter: counters.InMaxsizeExceeded().Lookup(t),
	}, {
		desc:    "InFragmentFrames",
		path:    ethCounterPath + "in-fragment-frames",
		counter: gnmi.Lookup(t, dut, counters.InFragmentFrames().State()),
	}, {
		desc:    "InCrcErrors",
		path:    ethCounterPath + "in-crc-errors",
		counter: gnmi.Lookup(t, dut, counters.InCrcErrors().State()),
	}, {
		desc:    "InJabberFrames",
		path:    ethCounterPath + "in-jabber-frames",
		counter: gnmi.Lookup(t, dut, counters.InJabberFrames().State()),
	}}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			// TODO: Enable the test for in-maxsize-exceeded after the issue fixed.
			if tc.desc == "InMaxsizeExceeded" {
				t.Skipf("Counter in-maxsize-exceeded is not supported yet.")
			}
			val, present := tc.counter.Val()
			if !present {
				t.Errorf("Get IsPresent status for path %q: got false, want true", tc.path)
			}
			t.Logf("Got path/value: %s:%d", tc.path, val)
		})
	}
}

func TestInterfaceCounters(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	dp := dut.Port(t, "port1")

	// Configure DUT interfaces.
	ConfigureDUTIntf(t, dut)

	// TODO: Uncomment the code which is commented out after the issue fixed.
	intfCounters := gnmi.OC().Interface(dp.Name()).Counters()
	subint := gnmi.OC().Interface(dp.Name()).Subinterface(0)
	ipv4Counters := subint.Ipv4().Counters()
	ipv6Counters := subint.Ipv6().Counters()
	intfCounterPath := "/interfaces/interface/state/counters/"
	ipv4CounterPath := "/interfaces/interface/subinterfaces/subinterface/ipv4/state/counters/"
	ipv6CounterPath := "/interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/"

	skipSubinterfacePacketCountersMissing := deviations.SubinterfacePacketCountersMissing(dut)
	skipIpv6DiscardedPkts := skipSubinterfacePacketCountersMissing || deviations.Ipv6DiscardedPktsUnsupported(dut)

	cases := []struct {
		desc    string
		path    string
		counter ygnmi.SingletonQuery[uint64]
		skip    bool
	}{{
		desc:    "InUnicastPkts",
		path:    intfCounterPath + "in-unicast-pkts",
		counter: intfCounters.InUnicastPkts().State(),
	}, {
		desc:    "InUnicastPkts",
		path:    intfCounterPath + "in-unicast-pkts",
		counter: intfCounters.InUnicastPkts().State(),
	}, {
		desc:    "InPkts",
		path:    intfCounterPath + "in-pkts",
		counter: intfCounters.InPkts().State(),
	}, {
		desc:    "OutPkts",
		path:    intfCounterPath + "out-pkts",
		counter: intfCounters.OutPkts().State(),
	}, {
		desc:    "IPv4InPkts",
		path:    ipv4CounterPath + "in-pkts",
		counter: ipv4Counters.InPkts().State(),
		skip:    skipSubinterfacePacketCountersMissing,
	}, {
		desc:    "IPv4OutPkts",
		path:    ipv4CounterPath + "out-pkts",
		counter: ipv4Counters.OutPkts().State(),
		skip:    skipSubinterfacePacketCountersMissing,
	}, {
		desc:    "IPv6InPkts",
		path:    ipv6CounterPath + "in-pkts",
		counter: ipv6Counters.InPkts().State(),
		skip:    skipSubinterfacePacketCountersMissing,
	}, {
		desc:    "IPv6OutPkts",
		path:    ipv6CounterPath + "out-pkts",
		counter: ipv6Counters.OutPkts().State(),
		skip:    skipSubinterfacePacketCountersMissing,
	}, {
		desc:    "IPv6InDiscardedPkts",
		path:    ipv6CounterPath + "in-discarded-pkts",
		counter: ipv6Counters.InDiscardedPkts().State(),
		skip:    skipIpv6DiscardedPkts,
	}, {
		desc:    "IPv6OutDiscardedPkts",
		path:    ipv6CounterPath + "out-discarded-pkts",
		counter: ipv6Counters.OutDiscardedPkts().State(),
		skip:    skipIpv6DiscardedPkts,
	}}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			if tc.skip {
				t.Skipf("Counter %v is not supported.", tc.desc)
			}
			val, present := gnmi.Lookup(t, dut, tc.counter).Val()
			if !present {
				t.Errorf("Get IsPresent status for path %q: got false, want true", tc.path)
			}
			t.Logf("Got path/value: %s:%d", tc.path, val)
		})
	}
}

func validateInAndOutPktsPerSecond(t *testing.T, dut *ondatra.DUTDevice, i1, i2 *interfaces.InterfacePath) bool {
	if deviations.InterfaceCountersFromContainer(dut) {
		time.Sleep(10 * time.Second)
		return true
	}
	inSamples := gnmi.Collect(t, dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_SAMPLE), ygnmi.WithSampleInterval(30*time.Second)), i1.Counters().InUnicastPkts().State(), 90*time.Second)
	outSamples := gnmi.Collect(t, dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_SAMPLE), ygnmi.WithSampleInterval(30*time.Second)), i2.Counters().OutUnicastPkts().State(), 90*time.Second)

	inPkts := inSamples.Await(t)
	outPkts := outSamples.Await(t)

	if len(inPkts) < 2 || len(outPkts) < 2 {
		t.Fatalf("did not get enough samples: in counters: %s out counters %s",
			inPkts, outPkts)
	}
	t.Logf("Sample Size Incoming Packets: %d, Sample Size Outgoing Packets: %d", len(inPkts), len(outPkts))
	var pktCounterOK = true
	// check counters for first and last sample interval, they shouldn't be equal
	inValFirst, _ := inPkts[0].Val()
	outValFirst, _ := outPkts[0].Val()
	inValFinal, _ := inPkts[len(inPkts)-1].Val()
	outValFinal, _ := outPkts[len(inPkts)-1].Val()

	if inValFinal == inValFirst || outValFinal == outValFirst {
		t.Logf("Counters not incremented: Initial Incoming Packets: %d, Final Incoming Packets: %d", inValFirst, inValFinal)
		t.Logf("Counters not incremented: Initial Outgoing Packets: %d,  Final Outgoing Packets: %d", outValFirst, outValFinal)
		pktCounterOK = false
		return pktCounterOK
	}

	for i := 1; i < len(inPkts); i++ {
		inValOld, _ := inPkts[i-1].Val()
		outValOld, _ := outPkts[i-1].Val()
		inValLatest, _ := inPkts[i].Val()
		outValLatest, _ := outPkts[i].Val()
		t.Logf("Incoming Packets: %d, Outgoing Packets: %d", inValLatest, outValLatest)
		if inValLatest == inValOld || outValLatest == outValOld || (inValLatest-inValOld != outValLatest-outValOld) {
			t.Logf("Comparison with previous iteration: Incoming Packets Delta : %d, Outgoing Packets Delta: %d", inValLatest-inValOld, outValLatest-outValOld)
			pktCounterOK = false
			break
		}
	}
	return pktCounterOK
}

func fetchInAndOutPkts(t *testing.T, dut *ondatra.DUTDevice, i1, i2 *interfaces.InterfacePath) (map[string]uint64, map[string]uint64) {
	// TODO: Replace InUnicastPkts with InPkts and OutUnicastPkts with OutPkts.
	if deviations.InterfaceCountersFromContainer(dut) {
		inPkts := map[string]uint64{
			"parent": *gnmi.Get(t, dut, i1.Counters().State()).InUnicastPkts,
		}
		outPkts := map[string]uint64{
			"parent": *gnmi.Get(t, dut, i2.Counters().State()).OutUnicastPkts,
		}
		return inPkts, outPkts
	}

	inPkts := map[string]uint64{
		"parent": gnmi.Get(t, dut, i1.Counters().InUnicastPkts().State()),
	}
	outPkts := map[string]uint64{
		"parent": gnmi.Get(t, dut, i2.Counters().OutUnicastPkts().State()),
	}
	return inPkts, outPkts
}

func TestIntfCounterUpdate(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")

	// TODO: Uncomment the code which is commented out after the issue fixed.
	// Configure DUT interfaces.
	ConfigureDUTIntf(t, dut)

	// Configure ATE interfaces.
	ate := ondatra.ATE(t, "ate")
	otg := ate.OTG()
	ap1 := ate.Port(t, "port1")
	ap2 := ate.Port(t, "port2")
	config := gosnappi.NewConfig()
	config.Ports().Add().SetName(ap1.ID())
	intf1 := config.Devices().Add().SetName(ap1.Name())
	eth1 := intf1.Ethernets().Add().SetName(ap1.Name() + ".Eth").SetMac("02:00:01:01:01:01")
	eth1.Connection().SetChoice(gosnappi.EthernetConnectionChoice.PORT_NAME).SetPortName(ap1.ID())
	ip4_1 := eth1.Ipv4Addresses().Add().SetName(intf1.Name() + ".IPv4").
		SetAddress("198.51.100.1").SetGateway("198.51.100.0").
		SetPrefix(31)
	ip6_1 := eth1.Ipv6Addresses().Add().SetName(intf1.Name() + ".IPv6").
		SetAddress("2001:DB8::2").SetGateway("2001:DB8::1").
		SetPrefix(126)
	config.Ports().Add().SetName(ap2.ID())
	intf2 := config.Devices().Add().SetName(ap2.Name())
	eth2 := intf2.Ethernets().Add().SetName(ap2.Name() + ".Eth").SetMac("02:00:01:02:01:01")
	eth2.Connection().SetChoice(gosnappi.EthernetConnectionChoice.PORT_NAME).SetPortName(ap2.ID())
	ip4_2 := eth2.Ipv4Addresses().Add().SetName(intf2.Name() + ".IPv4").
		SetAddress("198.51.100.3").SetGateway("198.51.100.2").
		SetPrefix(31)
	ip6_2 := eth2.Ipv6Addresses().Add().SetName(intf2.Name() + ".IPv6").
		SetAddress("2001:DB8::6").SetGateway("2001:DB8::5").
		SetPrefix(126)

	flowipv4 := config.Flows().Add().SetName("IPv4_test_flow")
	flowipv4.Metrics().SetEnable(true)
	flowipv4.TxRx().Device().
		SetTxNames([]string{intf1.Name() + ".IPv4"}).
		SetRxNames([]string{intf2.Name() + ".IPv4"})
	flowipv4.Size().SetFixed(100)
	flowipv4.Rate().SetPps(15)
	e1 := flowipv4.Packet().Add().Ethernet()
	e1.Src().SetValue(eth1.Mac())
	v4 := flowipv4.Packet().Add().Ipv4()
	v4.Src().SetValue(ip4_1.Address())
	v4.Dst().SetValue(ip4_2.Address())

	flowipv6 := config.Flows().Add().SetName("IPv6_test_flow")
	flowipv6.Metrics().SetEnable(true)
	flowipv6.TxRx().Device().
		SetTxNames([]string{intf1.Name() + ".IPv6"}).
		SetRxNames([]string{intf2.Name() + ".IPv6"})
	flowipv6.Size().SetFixed(100)
	flowipv6.Rate().SetPps(15)
	e2 := flowipv6.Packet().Add().Ethernet()
	e2.Src().SetValue(eth1.Mac())
	v6 := flowipv6.Packet().Add().Ipv6()
	v6.Src().SetValue(ip6_1.Address())
	v6.Dst().SetValue(ip6_2.Address())
	otg.PushConfig(t, config)
	otg.StartProtocols(t)

	i1 := gnmi.OC().Interface(dp1.Name())
	i2 := gnmi.OC().Interface(dp2.Name())

	t.Log("Running traffic on DUT interfaces: ", dp1, dp2)
	dutInPktsBeforeTraffic, dutOutPktsBeforeTraffic := fetchInAndOutPkts(t, dut, i1, i2)

	t.Logf("inPkts: %v and outPkts: %v before traffic: ", dutInPktsBeforeTraffic, dutOutPktsBeforeTraffic)
	waitOTGARPEntry(t)

	otg.StartTraffic(t)
	time.Sleep(2 * time.Second)
	// Check incoming and outgoing interface counters updated per second
	inAndOutPktsPerSecoundCounterOK := validateInAndOutPktsPerSecond(t, dut, i1, i2)
	otg.StopTraffic(t)

	// Check interface status is up.
	ds1 := gnmi.Get(t, dut, gnmi.OC().Interface(dp1.Name()).OperStatus().State())
	if want := oc.Interface_OperStatus_UP; ds1 != want {
		t.Errorf("Get(DUT port1 status): got %v, want %v", ds1, want)
	}
	ds2 := gnmi.Get(t, dut, gnmi.OC().Interface(dp2.Name()).OperStatus().State())
	if want := oc.Interface_OperStatus_UP; ds2 != want {
		t.Errorf("Get(DUT port2 status): got %v, want %v", ds2, want)
	}

	// Verifying the ate port link state
	for _, p := range config.Ports().Items() {
		portMetrics := gnmi.Get(t, otg, gnmi.OTG().Port(p.Name()).State())
		if portMetrics.GetLink() != otgtelemetry.Port_Link_UP {
			t.Errorf("Get(ATE %v status): got %v, want %v", p.Name(), portMetrics.GetLink(), otgtelemetry.Port_Link_UP)
		}
	}

	// Getting the otg flow metrics
	otgutils.LogFlowMetrics(t, otg, config)
	ateInPkts := map[string]uint64{}
	ateOutPkts := map[string]uint64{}
	for _, f := range config.Flows().Items() {
		recvMetric := gnmi.Get(t, otg, gnmi.OTG().Flow(f.Name()).State())
		if f.Name() == "IPv4_test_flow" {
			ateInPkts["IPv4"] = recvMetric.GetCounters().GetInPkts()
			ateOutPkts["IPv4"] = recvMetric.GetCounters().GetOutPkts()
		}
		if f.Name() == "IPv6_test_flow" {
			ateInPkts["IPv6"] = recvMetric.GetCounters().GetInPkts()
			ateOutPkts["IPv6"] = recvMetric.GetCounters().GetOutPkts()
		}
	}
	ateInPkts["parent"] = ateInPkts["IPv4"] + ateInPkts["IPv6"]
	ateOutPkts["parent"] = ateOutPkts["IPv4"] + ateOutPkts["IPv6"]

	for k, v := range ateOutPkts {
		if v == 0 {
			t.Errorf("gnmi.Get(t, ate.OTG(), gnmi.OC().Flow(%v).Counters().OutPkts().State()) = %v, want nonzero", k, v)
		}
	}
	for _, flow := range []string{flowipv4.Name(), flowipv6.Name()} {
		var lossPct float32
		if flow == "IPv4_test_flow" {
			lostPackets := float32(ateOutPkts["IPv4"] - ateInPkts["IPv4"])
			lossPct = lostPackets * 100 / float32(ateOutPkts["IPv4"])
		} else {
			lostPackets := float32(ateOutPkts["IPv6"] - ateInPkts["IPv6"])
			lossPct = lostPackets * 100 / float32(ateOutPkts["IPv6"])
		}
		if lossPct >= 1 {
			t.Errorf("LossPct per Flow(%v) = %f, want < 1", flow, lossPct)
		}
	}

	dutInPktsAfterTraffic, dutOutPktsAfterTraffic := fetchInAndOutPkts(t, dut, i1, i2)

	t.Logf("inPkts: %v and outPkts: %v after traffic: ", dutInPktsAfterTraffic, dutOutPktsAfterTraffic)
	for k := range dutInPktsAfterTraffic {
		if got, want := dutInPktsAfterTraffic[k]-dutInPktsBeforeTraffic[k], ateInPkts[k]; got < want {
			t.Errorf("Get less inPkts from telemetry: got %v, want >= %v", got, want)
		}
		if got, want := dutOutPktsAfterTraffic[k]-dutOutPktsBeforeTraffic[k], ateOutPkts[k]; got < want {
			t.Errorf("Get less outPkts from telemetry: got %v, want >= %v", got, want)
		}
	}
	// Validate per second interface counters are updated
	if !inAndOutPktsPerSecoundCounterOK {
		t.Error("Interface Packet Counters are not updated per second")
	}
}

func ConfigureDUTIntf(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")

	dutIntfs := []struct {
		desc          string
		intfName      string
		ipv4Addr      string
		ipv4PrefixLen uint8
		ipv6Addr      string
		ipv6PrefixLen uint8
	}{{
		desc:          "Input interface port1",
		intfName:      dp1.Name(),
		ipv4Addr:      "198.51.100.0",
		ipv4PrefixLen: 31,
		ipv6Addr:      "2001:DB8::1",
		ipv6PrefixLen: 126,
	}, {
		desc:          "Output interface port2",
		intfName:      dp2.Name(),
		ipv4Addr:      "198.51.100.2",
		ipv4PrefixLen: 31,
		ipv6Addr:      "2001:DB8::5",
		ipv6PrefixLen: 126,
	}}

	// Configure IPv4 and IPv6 addresses under subinterface.
	for _, intf := range dutIntfs {
		t.Logf("Configure DUT interface %s with attributes %v", intf.intfName, intf)
		i := &oc.Interface{
			Name:        ygot.String(intf.intfName),
			Description: ygot.String(intf.desc),
			Type:        oc.IETFInterfaces_InterfaceType_ethernetCsmacd,
		}
		i.GetOrCreateEthernet()
		s := i.GetOrCreateSubinterface(0)
		v4 := s.GetOrCreateIpv4()
		a4 := v4.GetOrCreateAddress(intf.ipv4Addr)
		a4.PrefixLength = ygot.Uint8(intf.ipv4PrefixLen)
		v6 := s.GetOrCreateIpv6()
		a6 := v6.GetOrCreateAddress(intf.ipv6Addr)
		a6.PrefixLength = ygot.Uint8(intf.ipv6PrefixLen)

		// We are testing that "enabled" is accepted by device when explicitly set to true,
		// per: https://github.com/openconfig/featureprofiles/issues/253
		i.Enabled = ygot.Bool(true)
		s.Enabled = ygot.Bool(true)
		if !deviations.IPv4MissingEnabled(dut) {
			v4.Enabled = ygot.Bool(true)
		}
		v6.Enabled = ygot.Bool(true)

		gnmi.Replace(t, dut, gnmi.OC().Interface(intf.intfName).Config(), i)

		t.Logf("Validate that IPv4 and IPv6 addresses are enabled: %s", intf.intfName)
		subint := gnmi.OC().Interface(intf.intfName).Subinterface(0)

		if !deviations.IPv4MissingEnabled(dut) {
			if !gnmi.Get(t, dut, subint.Ipv4().Enabled().State()) {
				t.Errorf("Ipv4().Enabled().Get(t) for interface %v: got false, want true", intf.intfName)
			}
		}
		if !gnmi.Get(t, dut, subint.Ipv6().Enabled().State()) {
			t.Errorf("Ipv6().Enabled().Get(t) for interface %v: got false, want true", intf.intfName)
		}
	}
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, dp1.Name(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, dp2.Name(), deviations.DefaultNetworkInstance(dut), 0)
	}
	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, dp1)
		fptest.SetPortSpeed(t, dp2)
	}
}

func TestInterfaceCPU(t *testing.T) {
	// TODO: Enable interface CPU test case here after bug is fixed.
	t.Skipf("Telemetry path /interfaces/interface/state/cpu is not supported.")

	dut := ondatra.DUT(t, "dut")
	dp := dut.Port(t, "port2")
	path := "/interfaces/interface/state/cpu"
	cpu, present := gnmi.Lookup(t, dut, gnmi.OC().Interface(dp.Name()).Cpu().State()).Val()
	if !present {
		t.Errorf("cpu.IsPresent() for path: %q: got false, want true", path)
	} else {
		t.Logf("Got path/value: %s:%v", path, cpu)
	}
}

func TestInterfaceMgmt(t *testing.T) {
	// TODO: Enable interface management test case here after bug is fixed.
	t.Skipf("Telemetry path /interfaces/interface/state/management is not supported.")

	dut := ondatra.DUT(t, "dut")
	dp := dut.Port(t, "port2")
	path := "/interfaces/interface/state/management"
	mgmt, present := gnmi.Lookup(t, dut, gnmi.OC().Interface(dp.Name()).Management().State()).Val()
	if !present {
		t.Errorf("mgmt.IsPresent() for path: %q: got false, want true", path)
	} else {
		t.Logf("Got path/value: %s:%v", path, mgmt)
	}
}

// waitOtgArpEntry waits until ARP entries are present on OTG interfaces
func waitOTGARPEntry(t *testing.T) {
	ate := ondatra.ATE(t, "ate")
	otg := ate.OTG()

	gnmi.WatchAll(t, otg, gnmi.OTG().InterfaceAny().Ipv4NeighborAny().LinkLayerAddress().State(), time.Minute, func(val *ygnmi.Value[string]) bool {
		return val.IsPresent()
	}).Await(t)
	gnmi.WatchAll(t, otg, gnmi.OTG().InterfaceAny().Ipv6NeighborAny().LinkLayerAddress().State(), time.Minute, func(val *ygnmi.Value[string]) bool {
		return val.IsPresent()
	}).Await(t)

}
