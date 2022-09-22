package gnmi_test

import (
	"testing"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestGNMI(t *testing.T) {

	// Configure a DUT

	dut := ondatra.DUT(t, "dut")
	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")
	dp3 := dut.Port(t, "port3")
	dp4 := dut.Port(t, "port4")

	ifCfg1 := &telemetry.Interface{
		Name:        ygot.String(dp1.Name()),
		Description: ygot.String("From Ixia"),
	}

	ifCfg1.GetOrCreateSubinterface(0).
		GetOrCreateIpv4().
		GetOrCreateAddress("192.0.2.10").
		PrefixLength = ygot.Uint8(31)

	ifCfg2 := &telemetry.Interface{
		Name:        ygot.String(dp2.Name()),
		Description: ygot.String("To Ixia"),
	}

	ifCfg2.GetOrCreateSubinterface(0).
		GetOrCreateIpv4().
		GetOrCreateAddress("192.0.2.21").
		PrefixLength = ygot.Uint8(31)

	ifCfg3 := &telemetry.Interface{
		Name:        ygot.String(dp3.Name()),
		Description: ygot.String("To Ixia"),
	}

	ifCfg3.GetOrCreateSubinterface(0).
		GetOrCreateIpv4().
		GetOrCreateAddress("192.0.2.31").
		PrefixLength = ygot.Uint8(31)

	ifCfg4 := &telemetry.Interface{
		Name:        ygot.String(dp4.Name()),
		Description: ygot.String("To Ixia"),
	}

	ifCfg4.GetOrCreateSubinterface(0).
		GetOrCreateIpv4().
		GetOrCreateAddress("192.0.2.41").
		PrefixLength = ygot.Uint8(31)

	dut.Config().Interface(dp1.Name()).Replace(t, ifCfg1)
	dut.Config().Interface(dp2.Name()).Replace(t, ifCfg2)
	dut.Config().Interface(dp3.Name()).Replace(t, ifCfg3)
	dut.Config().Interface(dp4.Name()).Replace(t, ifCfg4)

	// Configure an ATE

	/*ate := ondatra.ATE(t, "otg")
	ap1 := ate.Port(t, "port1")

	top := ate.Topology().New()
	intf1 := top.AddInterface("intf1").WithPort(ap1)
	intf1.IPv4().
		WithAddress("192.0.2.11/31").
		WithDefaultGateway("192.0.2.10")

	top.Push(t).StartProtocols(t) */

	// Exercise 5: Query Telemetry

	// ds1 := dut.Telemetry().Interface(dp1.Name()).OperStatus().Get(t)
	// if want := telemetry.Interface_OperStatus_UP; ds1 != want {
	// 	t.Errorf("Get(DUT port1 status): got %v, want %v", ds1, want)
	// }
	// ds2 := dut.Telemetry().Interface(dp2.Name()).OperStatus().Get(t)
	// if want := telemetry.Interface_OperStatus_UP; ds2 != want {
	// 	t.Errorf("Get(DUT port2 status): got %v, want %v", ds2, want)
	// }
	// ds3 := dut.Telemetry().Interface(dp3.Name()).OperStatus().Get(t)
	// if want := telemetry.Interface_OperStatus_UP; ds3 != want {
	// 	t.Errorf("Get(DUT port3 status): got %v, want %v", ds3, want)
	// }

}

// ap4 := ate.Port(t, "port4")

// ap2 := ate.Port(t, "port2")
// ap3 := ate.Port(t, "port3")

// intf2 := top.AddInterface("intf2").WithPort(ap2)
// intf2.IPv4().
// 	WithAddress("192.0.2.5/31").
// 	WithDefaultGateway("192.0.2.1")
// top.Push(t).StartProtocols(t)
// intf3 := top.AddInterface("intf3").WithPort(ap3)
// intf3.IPv4().
// 	WithAddress("192.0.2.6/31").
// 	WithDefaultGateway("192.0.2.2")

// intf4 := top.AddInterface("intf4").WithPort(ap4)
// intf4.IPv4().
// 	WithAddress("192.0.2.7/31").
// 	WithDefaultGateway("192.0.2.3")

// ds4 := dut.Telemetry().Interface(dp4.Name()).OperStatus().Get(t)
// if want := telemetry.Interface_OperStatus_UP; ds4 != want {
// 	t.Errorf("Get(DUT port4 status): got %v, want %v", ds4, want)
// }
// Generate Traffic at Port 2

// ethHeader := ondatra.NewEthernetHeader()
// ipv4Header := ondatra.NewIPv4Header()
// mplsHeader := ondatra.NewMPLSHeader().WithLabel(20)
// flow2 := ate.Traffic().NewFlow("codelab_flow").
// 	WithSrcEndpoints(intf1).
// 	WithDstEndpoints(intf2).
// 	WithHeaders(ethHeader, ipv4Header, mplsHeader).
// 	WithFrameRatePct(15)
// ate.Traffic().Start(t, flow2)
// time.Sleep(5 * time.Second)
// ate.Traffic().Stop(t)

// Generate Traffic at Port 3

// flow3 := ate.Traffic().NewFlow("codelab_flow").
// 	WithSrcEndpoints(intf1).
// 	WithDstEndpoints(intf3).
// 	WithHeaders(ethHeader, ipv4Header, mplsHeader).
// 	WithFrameRatePct(15)
// ate.Traffic().Start(t, flow3)
// time.Sleep(5 * time.Second)
// ate.Traffic().Stop(t)

// Generate Traffic at Port 4

// flow4 := ate.Traffic().NewFlow("codelab_flow").
// 	WithSrcEndpoints(intf1).
// 	WithDstEndpoints(intf4).
// 	WithHeaders(ethHeader, ipv4Header, mplsHeader).
// 	WithFrameRatePct(15)
// ate.Traffic().Start(t, flow4)
// time.Sleep(5 * time.Second)
// ate.Traffic().Stop(t)

// as1 := ate.Telemetry().Interface(ap1.Name()).OperStatus().Get(t)
// if want := telemetry.Interface_OperStatus_UP; as1 != want {
// 	t.Errorf("Get(ATE port1 status): got %v, want %v", as1, want)
// }
// as2 := ate.Telemetry().Interface(ap2.Name()).OperStatus().Get(t)
// if want := telemetry.Interface_OperStatus_UP; as2 != want {
// 	t.Errorf("Get(ATE port2 status): got %v, want %v", as2, want)
// }
// as3 := ate.Telemetry().Interface(ap3.Name()).OperStatus().Get(t)
// if want := telemetry.Interface_OperStatus_UP; as3 != want {
// 	t.Errorf("Get(ATE port3 status): got %v, want %v", as3, want)
// }
// as4 := ate.Telemetry().Interface(ap4.Name()).OperStatus().Get(t)
// if want := telemetry.Interface_OperStatus_UP; as4 != want {
// 	t.Errorf("Get(ATE port4 status): got %v, want %v", as4, want)
// }
// outPkts2 := ate.Telemetry().Flow(flow2.Name()).Counters().OutPkts().Get(t)
// if outPkts2 == 0 {
// 	t.Errorf("Get(out packets for flow2 %q): got %v, want nonzero", flow2.Name(), outPkts2)
// }
// outPkts3 := ate.Telemetry().Flow(flow3.Name()).Counters().OutPkts().Get(t)
// if outPkts3 == 0 {
// 	t.Errorf("Get(out packets for flow2 %q): got %v, want nonzero", flow3.Name(), outPkts3)
// }
// outPkts4 := ate.Telemetry().Flow(flow4.Name()).Counters().OutPkts().Get(t)
// if outPkts4 == 0 {
// 	t.Errorf("Get(out packets for flow2 %q): got %v, want nonzero", flow4.Name(), outPkts4)
// }

// dut1 := ondatra.DUT(t, "dut1")
// dut2 := ondatra.DUT(t, "dut2")
// sys1 := dut1.Telemetry().System().Lookup(t)
// sys2 := dut2.Telemetry().System().Lookup(t)
// t.Logf("dut1 system: %v", sys1)
// t.Logf("dut2 system: %v", sys2)
