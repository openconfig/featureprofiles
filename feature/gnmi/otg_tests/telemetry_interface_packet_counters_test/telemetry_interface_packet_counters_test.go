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
	"log"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestEthernetCounters(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	dp := dut.Port(t, "port2")
	counters := dut.Telemetry().Interface(dp.Name()).Ethernet().Counters()
	ethCounterPath := "/interfaces/interface/ethernet/state/counters/"

	cases := []struct {
		desc    string
		path    string
		counter *telemetry.QualifiedUint64
	}{{
		desc:    "InMacPauseFrames",
		path:    ethCounterPath + "out-mac-pause-frames",
		counter: counters.InMacPauseFrames().Lookup(t),
	}, {
		desc:    "OutMacPauseFrames",
		path:    ethCounterPath + "in-mac-pause-frames",
		counter: counters.OutMacPauseFrames().Lookup(t),
	}, {
		desc: "InMaxsizeExceeded",
		path: ethCounterPath + "in-maxsize-exceeded",
		// TODO: Uncomment counter in-maxsize-exceeded after the issue fixed.
		// counter: counters.InMaxsizeExceeded().Lookup(t),
	}, {
		desc:    "InFragmentFrames",
		path:    ethCounterPath + "in-fragment-frames",
		counter: counters.InFragmentFrames().Lookup(t),
	}, {
		desc:    "InCrcErrors",
		path:    ethCounterPath + "in-crc-errors",
		counter: counters.InCrcErrors().Lookup(t),
	}, {
		desc:    "InJabberFrames",
		path:    ethCounterPath + "in-jabber-frames",
		counter: counters.InJabberFrames().Lookup(t),
	}}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			// TODO: Enable the test for in-maxsize-exceeded after the issue fixed.
			if tc.desc == "InMaxsizeExceeded" {
				t.Skipf("Counter in-maxsize-exceeded is not supported yet.")
			}

			if !tc.counter.IsPresent() {
				t.Errorf("Get IsPresent status for path %q: got false, want true", tc.path)
			}
			t.Logf("Got path/value: %s:%d", tc.path, tc.counter.Val(t))
		})
	}
}

func TestInterfaceCounters(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	dp := dut.Port(t, "port1")

	// Configure DUT interfaces.
	ConfigureDUTIntf(t, dut)

	// TODO: Uncomment the code which is commented out after the issue fixed.
	intfCounters := dut.Telemetry().Interface(dp.Name()).Counters()
	subint := dut.Telemetry().Interface(dp.Name()).Subinterface(0)
	ipv4Counters := subint.Ipv4().Counters()
	ipv6Counters := subint.Ipv6().Counters()
	intfCounterPath := "/interfaces/interface/state/counters/"
	ipv4CounterPath := "/interfaces/interface/subinterfaces/subinterface/ipv4/state/counters/"
	ipv6CounterPath := "/interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/"

	cases := []struct {
		desc    string
		path    string
		counter *telemetry.QualifiedUint64
	}{{
		desc:    "InUnicastPkts",
		path:    intfCounterPath + "in-unicast-pkts",
		counter: intfCounters.InUnicastPkts().Lookup(t),
	}, {
		desc:    "InPkts",
		path:    intfCounterPath + "in-pkts",
		counter: intfCounters.InPkts().Lookup(t),
	}, {
		desc:    "OutPkts",
		path:    intfCounterPath + "out-pkts",
		counter: intfCounters.OutPkts().Lookup(t),
	}, {
		// desc: "IPv4InPkts",
		path:    ipv4CounterPath + "in-pkts",
		counter: ipv4Counters.InPkts().Lookup(t),
	}, {
		// desc: "IPv4OutPkts",
		path:    ipv4CounterPath + "out-pkts",
		counter: ipv4Counters.OutPkts().Lookup(t),
	}, {
		// desc: "IPv6InPkts",
		path:    ipv6CounterPath + "in-pkts",
		counter: ipv6Counters.InPkts().Lookup(t),
	}, {
		// desc: "IPv6OutPkts",
		path:    ipv6CounterPath + "out-pkts",
		counter: ipv6Counters.OutPkts().Lookup(t),
	}, {
		// desc: "IPv6InDiscardedPkts",
		path:    ipv6CounterPath + "in-discarded-pkts",
		counter: ipv6Counters.InDiscardedPkts().Lookup(t),
	}, {
		// desc: "IPv6OutDiscardedPkts",
		path:    ipv6CounterPath + "out-discarded-pkts",
		counter: ipv6Counters.OutDiscardedPkts().Lookup(t),
	}}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			// TODO: Enable the test for in-maxsize-exceeded after the issue fixed.
			if len(tc.desc) == 0 {
				t.Skipf("Counter %v is not supported.", tc.desc)
			}

			if !tc.counter.IsPresent() {
				t.Errorf("Get IsPresent status for path %q: got false, want true", tc.path)
			}
			t.Logf("Got path/value: %s:%d", tc.path, tc.counter.Val(t))
		})
	}
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
	config := otg.NewConfig(t)
	config.Ports().Add().SetName(ap1.ID())
	intf1 := config.Devices().Add().SetName(ap1.Name())
	eth1 := intf1.Ethernets().Add().SetName(ap1.Name() + ".eth").
		SetPortName(ap1.ID()).SetMac("00:00:01:01:01:01")
	ip4_1 := eth1.Ipv4Addresses().Add().SetName(intf1.Name() + ".ipv4").
		SetAddress("198.51.100.1").SetGateway("198.51.100.0").
		SetPrefix(31)
	ip6_1 := eth1.Ipv6Addresses().Add().SetName(intf1.Name() + ".ipv6").
		SetAddress("2001:DB8::2").SetGateway("2001:DB8::1").
		SetPrefix(126)
	config.Ports().Add().SetName(ap2.ID())
	intf2 := config.Devices().Add().SetName(ap2.Name())
	eth2 := intf2.Ethernets().Add().SetName(ap2.Name() + ".eth").
		SetPortName(ap2.ID()).SetMac("00:00:01:01:01:01")
	ip4_2 := eth2.Ipv4Addresses().Add().SetName(intf2.Name() + ".ipv4").
		SetAddress("198.51.100.3").SetGateway("198.51.100.2").
		SetPrefix(31)
	ip6_2 := eth2.Ipv6Addresses().Add().SetName(intf2.Name() + ".ipv6").
		SetAddress("2001:DB8::6").SetGateway("2001:DB8::5").
		SetPrefix(126)

	flowipv4 := config.Flows().Add().SetName("ipv4_test_flow")
	flowipv4.Metrics().SetEnable(true)
	flowipv4.TxRx().Device().
		SetTxNames([]string{intf1.Name() + ".ipv4"}).
		SetRxNames([]string{intf2.Name() + ".ipv4"})
	flowipv4.Size().SetFixed(100)
	flowipv4.Rate().SetPps(15)
	e1 := flowipv4.Packet().Add().Ethernet()
	e1.Src().SetValue(eth1.Mac())
	v4 := flowipv4.Packet().Add().Ipv4()
	v4.Src().SetValue(ip4_1.Address())
	v4.Dst().SetValue(ip4_2.Address())

	flowipv6 := config.Flows().Add().SetName("ipv6_test_flow")
	flowipv6.Metrics().SetEnable(true)
	flowipv6.TxRx().Device().
		SetTxNames([]string{intf1.Name() + ".ipv6"}).
		SetRxNames([]string{intf2.Name() + ".ipv6"})
	flowipv6.Size().SetFixed(100)
	flowipv6.Rate().SetPps(15)
	e2 := flowipv6.Packet().Add().Ethernet()
	e2.Src().SetValue(eth1.Mac())
	v6 := flowipv6.Packet().Add().Ipv6()
	v6.Src().SetValue(ip6_1.Address())
	v6.Dst().SetValue(ip6_2.Address())
	otg.PushConfig(t, config)
	otg.StartProtocols(t)

	// TODO: Replace InUnicastPkts with InPkts and OutUnicastPkts with OutPkts.
	i1 := dut.Telemetry().Interface(dp1.Name())
	// subintf1 := i1.Subinterface(0)
	dutInPktsBeforeTraffic := map[string]uint64{
		"parent": i1.Counters().InUnicastPkts().Get(t),
		// "ipv4":   subintf1.Ipv4().Counters().InPkts().Get(t),
		// "ipv6":   subintf1.Ipv6().Counters().InPkts().Get(t),
	}
	i2 := dut.Telemetry().Interface(dp2.Name())
	// subintf2 := i2.Subinterface(0)
	dutOutPktsBeforeTraffic := map[string]uint64{
		"parent": i2.Counters().OutUnicastPkts().Get(t),
		// "ipv4":   subintf2.Ipv4().Counters().OutPkts().Get(t),
		// "ipv6":   subintf2.Ipv6().Counters().OutPkts().Get(t),
	}

	t.Log("Running traffic on DUT interfaces: ", dp1, dp2)
	t.Logf("inPkts: %v and outPkts: %v before traffic: ", dutInPktsBeforeTraffic, dutOutPktsBeforeTraffic)

	otgutils.WaitForArpEntries(t, otg, "ipv4", &otgutils.WaitForOpts{Interval: 1 * time.Second, Timeout: 10 * time.Second})
	otgutils.WaitForArpEntries(t, otg, "ipv6", &otgutils.WaitForOpts{Interval: 1 * time.Second, Timeout: 10 * time.Second})
	otg.StartTraffic(t)
	err := otgutils.WatchFlowMetrics(t, otg, config, &otgutils.WaitForOpts{Interval: 2 * time.Second, Timeout: 10 * time.Second})
	if err != nil {
		log.Println(err)
	}
	otg.StopTraffic(t)

	// Check interface status is up.
	ds1 := dut.Telemetry().Interface(dp1.Name()).OperStatus().Get(t)
	if want := telemetry.Interface_OperStatus_UP; ds1 != want {
		t.Errorf("Get(DUT port1 status): got %v, want %v", ds1, want)
	}
	ds2 := dut.Telemetry().Interface(dp2.Name()).OperStatus().Get(t)
	if want := telemetry.Interface_OperStatus_UP; ds2 != want {
		t.Errorf("Get(DUT port2 status): got %v, want %v", ds2, want)
	}

	// Verifying the ate port link state
	pMetrics, err := otgutils.GetAllPortMetrics(t, ate.OTG(), config)
	if err != nil {
		t.Fatal("Error while getting the port metrics")
	}
	otgutils.PrintMetricsTable(&otgutils.MetricsTableOpts{
		ClearPrevious:  false,
		AllPortMetrics: pMetrics,
	})
	for _, port := range pMetrics.Items() {
		if port.Link() != gosnappi.PortMetricLinkEnum("up") {
			t.Errorf("Get(ATE %v status): got %v, want %v", port.Name(), port.Link(), "up")
		}
	}

	// Getting the otg flow metrics
	fMetrics, err := otgutils.GetFlowMetrics(t, otg, config)
	if err != nil {
		t.Fatal("Error while getting the flow metrics")
	}

	otgutils.PrintMetricsTable(&otgutils.MetricsTableOpts{
		ClearPrevious: false,
		FlowMetrics:   fMetrics,
	})

	ateInPkts := map[string]uint64{}
	ateOutPkts := map[string]uint64{}
	for _, f := range fMetrics.Items() {
		if f.Name() == "ipv4_test_flow" {
			ateInPkts["ipv4"] = uint64(f.FramesRx())
			ateOutPkts["ipv4"] = uint64(f.FramesTx())
		}
		if f.Name() == "ipv6_test_flow" {
			ateInPkts["ipv6"] = uint64(f.FramesRx())
			ateOutPkts["ipv6"] = uint64(f.FramesTx())
		}
	}
	ateInPkts["parent"] = ateInPkts["ipv4"] + ateInPkts["ipv6"]
	ateOutPkts["parent"] = ateOutPkts["ipv4"] + ateOutPkts["ipv6"]

	for k, v := range ateOutPkts {
		if v == 0 {
			t.Errorf("ate.Telemetry().Flow(%v).Counters().OutPkts().Get() = %v, want nonzero", k, v)
		}
	}
	for _, flow := range []string{flowipv4.Name(), flowipv6.Name()} {
		lossPct := 0
		if flowipv4.Name() == "ipv4_test_flow" {
			lostPackets := int(ateOutPkts["ipv4"] - ateInPkts["ipv4"])
			lossPct = lostPackets * 100 / int(ateOutPkts["ipv4"])
		} else {
			lostPackets := int(ateOutPkts["ipv4"] - ateInPkts["ipv4"])
			lossPct = lostPackets * 100 / int(ateOutPkts["ipv4"])
		}
		if lossPct >= 1 {
			t.Errorf("ate.Telemetry().Flow(%v).LossPct().Get() = %v, want < 1", flow, lossPct)
		}
	}

	// TODO: Replace InUnicastPkts with InPkts and OutUnicastPkts with OutPkts.
	dutInPktsAfterTraffic := map[string]uint64{
		"parent": i1.Counters().InUnicastPkts().Get(t),
		// "ipv4":   subintf1.Ipv4().Counters().InPkts().Get(t),
		// "ipv6":   subintf1.Ipv6().Counters().InPkts().Get(t),
	}
	dutOutPktsAfterTraffic := map[string]uint64{
		"parent": i2.Counters().OutUnicastPkts().Get(t),
		// "ipv4":   subintf2.Ipv4().Counters().OutPkts().Get(t),
		// "ipv6":   subintf2.Ipv6().Counters().OutPkts().Get(t),
	}

	t.Logf("inPkts: %v and outPkts: %v after traffic: ", dutInPktsAfterTraffic, dutOutPktsAfterTraffic)
	for k := range dutInPktsAfterTraffic {
		if got, want := dutInPktsAfterTraffic[k]-dutInPktsBeforeTraffic[k], ateInPkts[k]; got < want {
			t.Errorf("Get less inPkts from telemetry: got %v, want >= %v", got, want)
		}
		if got, want := dutOutPktsAfterTraffic[k]-dutOutPktsBeforeTraffic[k], ateOutPkts[k]; got < want {
			t.Errorf("Get less outPkts from telemetry: got %v, want >= %v", got, want)
		}
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

	// Configure the interfaces.
	for _, intf := range dutIntfs {
		t.Logf("Configure DUT interface %s with attributes %v", intf.intfName, intf)
		i := &telemetry.Interface{
			Name:        ygot.String(intf.intfName),
			Description: ygot.String(intf.desc),
			Type:        telemetry.IETFInterfaces_InterfaceType_ethernetCsmacd,
			Enabled:     ygot.Bool(true),
		}
		i.GetOrCreateEthernet()
		s := i.GetOrCreateSubinterface(0).GetOrCreateIpv4()
		s.Enabled = ygot.Bool(true)
		a := s.GetOrCreateAddress(intf.ipv4Addr)
		a.PrefixLength = ygot.Uint8(intf.ipv4PrefixLen)
		s6 := i.GetOrCreateSubinterface(0).GetOrCreateIpv6()
		s6.Enabled = ygot.Bool(true)
		a6 := s6.GetOrCreateAddress(intf.ipv6Addr)
		a6.PrefixLength = ygot.Uint8(intf.ipv6PrefixLen)
		dut.Config().Interface(intf.intfName).Replace(t, i)

		t.Logf("Validate DUT IPv4 and IPv6 subinterface %s are enabled.", intf.intfName)
		subint := dut.Telemetry().Interface(intf.intfName).Subinterface(0)
		if !subint.Ipv4().Enabled().Get(t) {
			t.Errorf("Ipv4().Enabled().Get(t) for interface %v: got false, want true", intf.intfName)
		}
		if !subint.Ipv6().Enabled().Get(t) {
			t.Errorf("Ipv6().Enabled().Get(t) for interface %v: got false, want true", intf.intfName)
		}
	}
}

func TestInterfaceCPU(t *testing.T) {
	// TODO: Enable interface CPU test case here after bug is fixed.
	t.Skipf("Telemetry path /interfaces/interface/state/cpu is not supported.")

	dut := ondatra.DUT(t, "dut")
	dp := dut.Port(t, "port2")
	path := "/interfaces/interface/state/cpu"
	cpu := dut.Telemetry().Interface(dp.Name()).Cpu().Lookup(t)
	if !cpu.IsPresent() {
		t.Errorf("cpu.IsPresent() for path: %q: got false, want true", path)
	} else {
		t.Logf("Got path/value: %s:%v", path, cpu.Val(t))
	}
}

func TestInterfaceMgmt(t *testing.T) {
	// TODO: Enable interface management test case here after bug is fixed.
	t.Skipf("Telemetry path /interfaces/interface/state/management is not supported.")

	dut := ondatra.DUT(t, "dut")
	dp := dut.Port(t, "port2")
	path := "/interfaces/interface/state/management"
	mgmt := dut.Telemetry().Interface(dp.Name()).Management().Lookup(t)
	if !mgmt.IsPresent() {
		t.Errorf("mgmt.IsPresent() for path: %q: got false, want true", path)
	} else {
		t.Logf("Got path/value: %s:%v", path, mgmt.Val(t))
	}
}
