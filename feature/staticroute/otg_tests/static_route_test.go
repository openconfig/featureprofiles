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
	// dp3 := dut.Port(t, "port3")
	// dp4 := dut.Port(t, "port4")

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
		GetOrCreateAddress("192.0.2.20").
		PrefixLength = ygot.Uint8(31)

	// ifCfg3 := &telemetry.Interface{
	// 	Name:        ygot.String(dp3.Name()),
	// 	Description: ygot.String("To Ixia"),
	// }

	// ifCfg3.GetOrCreateSubinterface(0).
	// 	GetOrCreateIpv4().
	// 	GetOrCreateAddress("192.0.2.30").
	// 	PrefixLength = ygot.Uint8(31)

	// ifCfg4 := &telemetry.Interface{
	// 	Name:        ygot.String(dp4.Name()),
	// 	Description: ygot.String("To Ixia"),
	// }

	// ifCfg4.GetOrCreateSubinterface(0).
	// 	GetOrCreateIpv4().
	// 	GetOrCreateAddress("192.0.2.40").
	// 	PrefixLength = ygot.Uint8(31)

	dut.Config().Interface(dp1.Name()).Replace(t, ifCfg1)
	dut.Config().Interface(dp2.Name()).Replace(t, ifCfg2)
	// dut.Config().Interface(dp3.Name()).Replace(t, ifCfg3)
	// dut.Config().Interface(dp4.Name()).Replace(t, ifCfg4)

	Ni1 := &telemetry.NetworkInstance{}

	Ni1.GetOrCreateProtocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, "static").
		GetOrCreateStatic("10.0.0.0/24").
		GetOrCreateNextHop("h1").NextHop = telemetry.UnionString("192.0.2.10")

	dut.Config().NetworkInstance("default").Update(t, Ni1)

	//  Configure an ATE

	ate := ondatra.ATE(t, "ate")
	ap1 := ate.Port(t, "port1")
	ap2 := ate.Port(t, "port2")
	// ap3 := ate.Port(t, "port3")
	// ap4 := ate.Port(t, "port4")

	top := ate.OTG().NewConfig(t)

	top.Ports().Add().SetName(ap1.ID())
	i1 := top.Devices().Add().SetName(ap1.ID())
	eth1 := i1.Ethernets().Add().SetName("src.Eth").SetPortName(i1.Name()).SetMac("02:1a:c0:00:02:01")
	eth1.Ipv4Addresses().Add().SetName("src.IPv4").SetAddress("192.0.2.11").SetGateway("192.0.2.10").SetPrefix(int32(31))

	top.Ports().Add().SetName(ap2.ID())
	i2 := top.Devices().Add().SetName(ap2.ID())
	eth2 := i2.Ethernets().Add().SetName("dst1.Eth").SetPortName(i2.Name()).SetMac("02:1a:c0:00:02:02")
	eth2.Ipv4Addresses().Add().SetName("dst1.IPv4").SetAddress("192.0.2.21").SetGateway("192.0.2.20").SetPrefix(int32(31))

	// top.Ports().Add().SetName(ap3.ID())
	// i3 := top.Devices().Add().SetName(ap3.ID())
	// eth3 := i3.Ethernets().Add().SetName("dst2.Eth").SetPortName(i3.Name()).SetMac("02:1a:c0:00:02:05")
	// eth3.Ipv4Addresses().Add().SetName("dst2.IPv4").SetAddress("192.0.2.31").SetGateway("192.0.2.30").SetPrefix(int32(31))

	// top.Ports().Add().SetName(ap4.ID())
	// i4 := top.Devices().Add().SetName(ap4.ID())
	// eth4 := i4.Ethernets().Add().SetName("dst3.Eth").SetPortName(i4.Name()).SetMac("02:1a:c0:00:02:06")
	// eth4.Ipv4Addresses().Add().SetName("dst3.IPv4").SetAddress("192.0.2.41").SetGateway("192.0.2.40").SetPrefix(int32(31))

	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)

}
