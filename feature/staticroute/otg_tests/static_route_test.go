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
		GetOrCreateAddress("192.0.2.11").
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

	Ni1 := &telemetry.NetworkInstance{}

	Ni1.GetOrCreateProtocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, "static").
		GetOrCreateStatic("10.0.0.0/24").
		GetOrCreateNextHop("h1").NextHop = telemetry.UnionString("192.0.2.10")

	dut.Config().NetworkInstance("default").Update(t, Ni1)

	//  Configure an ATE

	// ate := ondatra.ATE(t, "ate")
	// ap1 := ate.Port(t, "port1")
	// ap2 := ate.Port(t, "port2")
	// ap3 := ate.Port(t, "port3")
	// ap4 := ate.Port(t, "port4")

	// top := ate.Topology().New()

	// intf1 := top.AddInterface("intf1").WithPort(ap1)
	// intf1.IPv4().
	// 	WithAddress("192.0.2.12/31").
	// 	WithDefaultGateway("192.0.2.11")

	// intf2 := top.AddInterface("intf2").WithPort(ap2)
	// intf2.IPv4().
	// 	WithAddress("192.0.2.22/31").
	// 	WithDefaultGateway("192.0.2.21")

	// intf3 := top.AddInterface("intf3").WithPort(ap3)
	// intf3.IPv4().
	// 	WithAddress("192.0.2.32/31").
	// 	WithDefaultGateway("192.0.2.31")

	// intf4 := top.AddInterface("intf4").WithPort(ap4)
	// intf4.IPv4().
	// 	WithAddress("192.0.2.42/31").
	// 	WithDefaultGateway("192.0.2.41")

}
