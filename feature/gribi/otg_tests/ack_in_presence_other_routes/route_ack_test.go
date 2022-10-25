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

package route_ack_test

import (
	"context"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/featureprofiles/yang/fpoc"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/telemetry"
	otgtelemetry "github.com/openconfig/ondatra/telemetry/otg"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Settings for configuring the baseline testbed with the test
// topology.
//
// The testbed consists of ate:port1 -> dut:port1,
// dut:port2 -> ate:port2 and dut:port3 -> ate:port3.
//
//   * ate:port1 -> dut:port1 subnet 192.0.2.0/30
//   * ate:port2 -> dut:port2 subnet 192.0.2.4/30
//   * ate:port3 -> dut:port3 subnet 192.0.2.8/30
//
//   * Destination network: 203.0.113.0/24

const (
	ipv4PrefixLen = 30
	ateDstNetCIDR = "203.0.113.0/24"
	staticNH      = "192.0.2.6"
	nhIndex       = 1
	nhgIndex      = 42
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "dutPort1",
		IPv4:    "192.0.2.1",
		IPv4Len: ipv4PrefixLen,
	}

	atePort1 = attrs.Attributes{
		Name:    "port1",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.0.2.2",
		IPv4Len: ipv4PrefixLen,
	}

	dutPort2 = attrs.Attributes{
		Desc:    "dutPort2",
		IPv4:    "192.0.2.5",
		IPv4Len: ipv4PrefixLen,
	}

	atePort2 = attrs.Attributes{
		Name:    "port2",
		MAC:     "02:00:02:01:01:01",
		IPv4:    "192.0.2.6",
		IPv4Len: ipv4PrefixLen,
	}

	dutPort3 = attrs.Attributes{
		Desc:    "dutPort3",
		IPv4:    "192.0.2.9",
		IPv4Len: ipv4PrefixLen,
	}

	atePort3 = attrs.Attributes{
		Name:    "port3",
		MAC:     "02:00:03:01:01:01",
		IPv4:    "192.0.2.10",
		IPv4Len: ipv4PrefixLen,
	}
)

var gatewayMap = map[attrs.Attributes]attrs.Attributes{
	atePort1: dutPort1,
	atePort2: dutPort2,
	atePort3: dutPort3,
}

// configInterfaceDUT configures the interface with the Addrs.
func configInterfaceDUT(i *telemetry.Interface, a *attrs.Attributes) *telemetry.Interface {
	i.Description = ygot.String(a.Desc)
	i.Type = telemetry.IETFInterfaces_InterfaceType_ethernetCsmacd
	if *deviations.InterfaceEnabled {
		i.Enabled = ygot.Bool(true)
	}

	s := i.GetOrCreateSubinterface(0)
	s4 := s.GetOrCreateIpv4()
	if *deviations.InterfaceEnabled {
		s4.Enabled = ygot.Bool(true)
	}
	s4a := s4.GetOrCreateAddress(a.IPv4)
	s4a.PrefixLength = ygot.Uint8(ipv4PrefixLen)

	return i
}

// configureDUT configures port1, port2 and port3 on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	d := dut.Config()

	p1 := dut.Port(t, "port1")
	i1 := &telemetry.Interface{Name: ygot.String(p1.Name())}
	d.Interface(p1.Name()).Replace(t, configInterfaceDUT(i1, &dutPort1))

	p2 := dut.Port(t, "port2")
	i2 := &telemetry.Interface{Name: ygot.String(p2.Name())}
	d.Interface(p2.Name()).Replace(t, configInterfaceDUT(i2, &dutPort2))

	p3 := dut.Port(t, "port3")
	i3 := &telemetry.Interface{Name: ygot.String(p3.Name())}
	d.Interface(p3.Name()).Replace(t, configInterfaceDUT(i3, &dutPort3))

}

// configureATE configures port1, port2 and port3 on the ATE.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	otg := ate.OTG()
	top := otg.NewConfig(t)

	top.Ports().Add().SetName(atePort1.Name)
	i1 := top.Devices().Add().SetName(atePort1.Name)
	eth1 := i1.Ethernets().Add().SetName(atePort1.Name + ".Eth").SetPortName(i1.Name()).SetMac(atePort1.MAC)
	eth1.Ipv4Addresses().Add().SetName(i1.Name() + ".IPv4").SetAddress(atePort1.IPv4).SetGateway(dutPort1.IPv4).SetPrefix(int32(atePort1.IPv4Len))

	top.Ports().Add().SetName(atePort2.Name)
	i2 := top.Devices().Add().SetName(atePort2.Name)
	eth2 := i2.Ethernets().Add().SetName(atePort2.Name + ".Eth").SetPortName(i2.Name()).SetMac(atePort2.MAC)
	eth2.Ipv4Addresses().Add().SetName(i2.Name() + ".IPv4").SetAddress(atePort2.IPv4).SetGateway(dutPort2.IPv4).SetPrefix(int32(atePort2.IPv4Len))

	top.Ports().Add().SetName(atePort3.Name)
	i3 := top.Devices().Add().SetName(atePort3.Name)
	eth3 := i3.Ethernets().Add().SetName(atePort3.Name + ".Eth").SetPortName(i3.Name()).SetMac(atePort3.MAC)
	eth3.Ipv4Addresses().Add().SetName(i3.Name() + ".IPv4").SetAddress(atePort3.IPv4).SetGateway(dutPort3.IPv4).SetPrefix(int32(atePort3.IPv4Len))
	return top
}

// Waits for an ARP entry to be present for ATE Port1
func waitOTGARPEntry(t *testing.T) {
	ate := ondatra.ATE(t, "ate")
	ate.OTG().Telemetry().Interface(atePort1.Name+".Eth").Ipv4NeighborAny().LinkLayerAddress().Watch(
		t, time.Minute, func(val *otgtelemetry.QualifiedString) bool {
			return val.IsPresent()
		}).Await(t)
}

// testTraffic generates traffic flow from source network to
// destination network via srcEndPoint to dstEndPoint and checks for
// packet loss.
func testTraffic(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config, srcEndPoint, dstEndPoint attrs.Attributes) {
	otg := ate.OTG()
	gwIp := gatewayMap[srcEndPoint].IPv4
	waitOTGARPEntry(t)
	dstMac := otg.Telemetry().Interface(srcEndPoint.Name + ".Eth").Ipv4Neighbor(gwIp).LinkLayerAddress().Get(t)
	top.Flows().Clear().Items()
	flowipv4 := top.Flows().Add().SetName("Flow")
	flowipv4.Metrics().SetEnable(true)
	flowipv4.TxRx().Port().
		SetTxName(srcEndPoint.Name).
		SetRxName(dstEndPoint.Name)
	flowipv4.Duration().SetChoice("continuous")
	e1 := flowipv4.Packet().Add().Ethernet()
	e1.Src().SetValue(srcEndPoint.MAC)
	e1.Dst().SetChoice("value").SetValue(dstMac)
	v4 := flowipv4.Packet().Add().Ipv4()
	srcIpv4 := srcEndPoint.IPv4
	v4.Src().SetValue(srcIpv4)
	v4.Dst().Increment().SetStart("203.0.113.0").SetCount(250)
	otg.PushConfig(t, top)

	otg.StartTraffic(t)
	time.Sleep(15 * time.Second)
	t.Logf("Stop traffic")
	otg.StopTraffic(t)

	otgutils.LogFlowMetrics(t, otg, top)
	txPkts := otg.Telemetry().Flow("Flow").Counters().OutPkts().Get(t)
	rxPkts := otg.Telemetry().Flow("Flow").Counters().InPkts().Get(t)
	if got := (txPkts - rxPkts) * 100 / txPkts; got > 0 {
		t.Errorf("LossPct for flow %s got %v, want 0", flowipv4.Name(), got)
	}
}

// testArgs holds the objects needed by a test case.
type testArgs struct {
	ctx     context.Context
	clientA *gribi.Client
	dut     *ondatra.DUTDevice
	ate     *ondatra.ATEDevice
	top     gosnappi.Config
}

// Configure network instance
func configureNetworkInstance(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	dutConfPath := dut.Config().NetworkInstance(*deviations.DefaultNetworkInstance)
	dutConfPath.Type().Replace(t, telemetry.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)
}

// configStaticRoute configures a static route.
func configStaticRoute(t *testing.T, dut *ondatra.DUTDevice, prefix string, nexthop string) {
	ni1 := dut.Config().NetworkInstance(*deviations.DefaultNetworkInstance).Get(t)
	static := ni1.GetOrCreateProtocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, *deviations.StaticProtocolName)
	static.Enabled = ygot.Bool(true)
	sr := static.GetOrCreateStatic(prefix)
	nh := sr.GetOrCreateNextHop("nhg1")
	nh.NextHop = fpoc.UnionString(nexthop)
	dut.Config().NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, *deviations.StaticProtocolName).Update(t, static)
}

// routeAck configures a IPv4 entry through clientA. Ensure that the entry via ClientA
// is active through AFT Telemetry.
func routeAck(ctx context.Context, t *testing.T, args *testArgs) {
	// Add an IPv4Entry for 203.0.113.0/24 pointing to ATE port-3 via gRIBI-A,
	// ensure that the entry is active through AFT telemetry
	t.Logf("Add an IPv4Entry for %s pointing to ATE port-3 via gRIBI-A", ateDstNetCIDR)
	args.clientA.AddNH(t, nhIndex, atePort3.IPv4, *deviations.DefaultNetworkInstance, fluent.InstalledInRIB)
	args.clientA.AddNHG(t, nhgIndex, map[uint64]uint64{nhIndex: 1}, *deviations.DefaultNetworkInstance, fluent.InstalledInRIB)
	args.clientA.AddIPv4(t, ateDstNetCIDR, nhgIndex, *deviations.DefaultNetworkInstance, "", fluent.InstalledInRIB)

	// Verify the entry for 203.0.113.0/24 is active through AFT Telemetry.
	ipv4Path := args.dut.Telemetry().NetworkInstance(*deviations.DefaultNetworkInstance).Afts().Ipv4Entry(ateDstNetCIDR)
	if got, ok := ipv4Path.Prefix().Watch(t, time.Minute, func(val *telemetry.QualifiedString) bool {
		return val.IsPresent() && val.Val(t) == ateDstNetCIDR
	}).Await(t); !ok {
		t.Errorf("ipv4-entry/state/prefix got %s, want %s", got.Val(t), ateDstNetCIDR)
	}
	// Verify that static route(203.0.113.0/24) to ATE port-2 is preferred by the traffic.`
	srcEndPoint := atePort1
	dstEndPoint := atePort2
	testTraffic(t, args.ate, args.top, srcEndPoint, dstEndPoint)

}

func TestRouteAck(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ctx := context.Background()

	// Configure the DUT
	configureDUT(t, dut)

	// Configure the ATE
	ate := ondatra.ATE(t, "ate")
	top := configureATE(t, ate)
	otg := ate.OTG()
	otg.PushConfig(t, top)
	otg.StartProtocols(t)

	// Configure the DUT with static route 203.0.113.0/24
	configureNetworkInstance(t)
	t.Logf("Configure the DUT with static route 203.0.113.0/24...")
	configStaticRoute(t, dut, ateDstNetCIDR, staticNH)
	// Verify the entry for 203.0.113.0/24 is active through AFT Telemetry.
	ipv4Path := dut.Telemetry().NetworkInstance(*deviations.DefaultNetworkInstance).Afts().Ipv4Entry(ateDstNetCIDR)
	if got, ok := ipv4Path.Prefix().Watch(t, time.Minute, func(val *telemetry.QualifiedString) bool {
		return val.IsPresent() && val.Val(t) == ateDstNetCIDR
	}).Await(t); !ok {
		t.Errorf("ipv4-entry/state/prefix got %s, want %s", got.Val(t), ateDstNetCIDR)
	} else {
		t.Logf("Prefix %s installed in DUT as static...", got.Val(t))
	}

	// Configure the gRIBI client clientA
	clientA := gribi.Client{
		DUT:                  dut,
		FibACK:               false,
		Persistence:          true,
		InitialElectionIDLow: 10,
	}
	defer clientA.Close(t)
	if err := clientA.Start(t); err != nil {
		t.Fatalf("gRIBI Connection can not be established")
	}

	args := &testArgs{
		ctx:     ctx,
		clientA: &clientA,
		dut:     dut,
		ate:     ate,
		top:     top,
	}

	routeAck(ctx, t, args)
	otg.StopProtocols(t)
}
