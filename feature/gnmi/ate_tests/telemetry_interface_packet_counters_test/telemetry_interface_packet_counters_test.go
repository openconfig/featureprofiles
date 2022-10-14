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

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
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
		counter func(testing.TB) *telemetry.QualifiedUint64
		skip    bool
	}{{
		desc:    "InUnicastPkts",
		path:    intfCounterPath + "in-unicast-pkts",
		counter: intfCounters.InUnicastPkts().Lookup,
	}, {
		desc:    "InUnicastPkts",
		path:    intfCounterPath + "in-unicast-pkts",
		counter: intfCounters.InUnicastPkts().Lookup,
	}, {
		desc:    "InPkts",
		path:    intfCounterPath + "in-pkts",
		counter: intfCounters.InPkts().Lookup,
	}, {
		desc:    "OutPkts",
		path:    intfCounterPath + "out-pkts",
		counter: intfCounters.OutPkts().Lookup,
	}, {
		desc:    "IPv4InPkts",
		path:    ipv4CounterPath + "in-pkts",
		counter: ipv4Counters.InPkts().Lookup,
		skip:    *deviations.SubinterfacePacketCountersMissing,
	}, {
		desc:    "IPv4OutPkts",
		path:    ipv4CounterPath + "out-pkts",
		counter: ipv4Counters.OutPkts().Lookup,
		skip:    *deviations.SubinterfacePacketCountersMissing,
	}, {
		desc:    "IPv6InPkts",
		path:    ipv6CounterPath + "in-pkts",
		counter: ipv6Counters.InPkts().Lookup,
		skip:    *deviations.SubinterfacePacketCountersMissing,
	}, {
		desc:    "IPv6OutPkts",
		path:    ipv6CounterPath + "out-pkts",
		counter: ipv6Counters.OutPkts().Lookup,
		skip:    *deviations.SubinterfacePacketCountersMissing,
	}, {
		desc:    "IPv6InDiscardedPkts",
		path:    ipv6CounterPath + "in-discarded-pkts",
		counter: ipv6Counters.InDiscardedPkts().Lookup,
		skip:    *deviations.SubinterfacePacketCountersMissing,
	}, {
		desc:    "IPv6OutDiscardedPkts",
		path:    ipv6CounterPath + "out-discarded-pkts",
		counter: ipv6Counters.OutDiscardedPkts().Lookup,
		skip:    *deviations.SubinterfacePacketCountersMissing,
	}}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			if tc.skip {
				t.Skipf("Counter %v is not supported.", tc.desc)
			}
			if !tc.counter(t).IsPresent() {
				t.Errorf("Get IsPresent status for path %q: got false, want true", tc.path)
			}
			t.Logf("Got path/value: %s:%d", tc.path, tc.counter(t).Val(t))
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
	ap1 := ate.Port(t, "port1")
	ap2 := ate.Port(t, "port2")
	top := ate.Topology().New()
	intf1 := top.AddInterface("intf1").WithPort(ap1)
	intf1.IPv4().
		WithAddress("198.51.100.1/31").
		WithDefaultGateway("198.51.100.0")
	intf1.IPv6().
		WithAddress("2001:DB8::2/126").
		WithDefaultGateway("2001:DB8::1")
	intf2 := top.AddInterface("intf2").WithPort(ap2)
	intf2.IPv4().
		WithAddress("198.51.100.3/31").
		WithDefaultGateway("198.51.100.2")
	intf2.IPv6().
		WithAddress("2001:DB8::6/126").
		WithDefaultGateway("2001:DB8::5")
	top.Push(t).StartProtocols(t)

	ethHeader := ondatra.NewEthernetHeader()
	ipv4Flow := ate.Traffic().NewFlow("ipv4_test_flow").
		WithSrcEndpoints(intf1).
		WithDstEndpoints(intf2).
		WithHeaders(ethHeader, ondatra.NewIPv4Header()).
		WithFrameRatePct(15)
	ipv6Flow := ate.Traffic().NewFlow("ipv6_test_flow").
		WithSrcEndpoints(intf1).
		WithDstEndpoints(intf2).
		WithHeaders(ethHeader, ondatra.NewIPv6Header()).
		WithFrameRatePct(15)

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
	if *deviations.InterfaceCountersFromContainer {
		dutInPktsBeforeTraffic = map[string]uint64{
			"parent": *i1.Counters().Get(t).InUnicastPkts,
			// "ipv4":   *subintf1.Ipv4().Counters().Get(t).InPkts,
			// "ipv6":   subintf1.Ipv6().Counters().Get(t).InPkts,
		}
		dutOutPktsBeforeTraffic = map[string]uint64{
			"parent": *i2.Counters().Get(t).OutUnicastPkts,
			// "ipv4":   *subintf2.Ipv4().Counters().Get(t).OutPkts,
			// "ipv6":   *subintf2.Ipv6().Counters().Get(t).OutPkts,
		}
	}
	t.Log("Running traffic on DUT interfaces: ", dp1, dp2)
	t.Logf("inPkts: %v and outPkts: %v before traffic: ", dutInPktsBeforeTraffic, dutOutPktsBeforeTraffic)
	ate.Traffic().Start(t, ipv4Flow, ipv6Flow)
	time.Sleep(10 * time.Second)
	ate.Traffic().Stop(t)

	// Check interface status is up.
	ds1 := dut.Telemetry().Interface(dp1.Name()).OperStatus().Get(t)
	if want := telemetry.Interface_OperStatus_UP; ds1 != want {
		t.Errorf("Get(DUT port1 status): got %v, want %v", ds1, want)
	}
	as1 := ate.Telemetry().Interface(ap1.Name()).OperStatus().Get(t)
	if want := telemetry.Interface_OperStatus_UP; as1 != want {
		t.Errorf("Get(ATE port1 status): got %v, want %v", as1, want)
	}
	ds2 := dut.Telemetry().Interface(dp2.Name()).OperStatus().Get(t)
	if want := telemetry.Interface_OperStatus_UP; ds2 != want {
		t.Errorf("Get(DUT port2 status): got %v, want %v", ds2, want)
	}
	as2 := ate.Telemetry().Interface(ap2.Name()).OperStatus().Get(t)
	if want := telemetry.Interface_OperStatus_UP; as2 != want {
		t.Errorf("Get(ATE port2 status): got %v, want %v", as2, want)
	}

	ateInPkts := map[string]uint64{
		"ipv4": ate.Telemetry().Flow(ipv4Flow.Name()).Counters().InPkts().Get(t),
		"ipv6": ate.Telemetry().Flow(ipv6Flow.Name()).Counters().InPkts().Get(t),
	}
	ateInPkts["parent"] = ateInPkts["ipv4"] + ateInPkts["ipv6"]

	ateOutPkts := map[string]uint64{
		"ipv4": ate.Telemetry().Flow(ipv4Flow.Name()).Counters().OutPkts().Get(t),
		"ipv6": ate.Telemetry().Flow(ipv6Flow.Name()).Counters().OutPkts().Get(t),
	}
	ateOutPkts["parent"] = ateOutPkts["ipv4"] + ateOutPkts["ipv6"]

	for k, v := range ateOutPkts {
		if v == 0 {
			t.Errorf("ate.Telemetry().Flow(%v).Counters().OutPkts().Get() = %v, want nonzero", k, v)
		}
	}
	for _, flow := range []string{ipv4Flow.Name(), ipv6Flow.Name()} {
		lossPct := ate.Telemetry().Flow(flow).LossPct().Get(t)
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
	if *deviations.InterfaceCountersFromContainer {
		dutInPktsAfterTraffic = map[string]uint64{
			"parent": *i1.Counters().Get(t).InUnicastPkts,
			// "ipv4":   *subintf1.Ipv4().Counters().Get(t).InPkts,
			// "ipv6":   *subintf1.Ipv6().Counters().Get(t).InPkts,
		}
		dutOutPktsAfterTraffic = map[string]uint64{
			"parent": *i2.Counters().Get(t).OutUnicastPkts,
			// "ipv4":   *subintf2.Ipv4().Counters().Get(t).OutPkts,
			// "ipv6":   *subintf2.Ipv6().Counters().Get(t).OutPkts,
		}
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

	// Configure IPv4 and IPv6 addresses under subinterface.
	for _, intf := range dutIntfs {
		t.Logf("Configure DUT interface %s with attributes %v", intf.intfName, intf)
		i := &telemetry.Interface{
			Name:        ygot.String(intf.intfName),
			Description: ygot.String(intf.desc),
			Type:        telemetry.IETFInterfaces_InterfaceType_ethernetCsmacd,
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
		if !*deviations.IPv4MissingEnabled {
			v4.Enabled = ygot.Bool(true)
		}
		v6.Enabled = ygot.Bool(true)

		dut.Config().Interface(intf.intfName).Replace(t, i)

		t.Logf("Validate that IPv4 and IPv6 addresses are enabled: %s", intf.intfName)
		subint := dut.Telemetry().Interface(intf.intfName).Subinterface(0)

		if !*deviations.IPv4MissingEnabled {
			if !subint.Ipv4().Enabled().Get(t) {
				t.Errorf("Ipv4().Enabled().Get(t) for interface %v: got false, want true", intf.intfName)
			}
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
