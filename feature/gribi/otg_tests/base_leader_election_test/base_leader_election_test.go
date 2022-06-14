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

package base_leader_election_test

import (
	"context"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/telemetry"
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
//   * Destination network: 198.51.100.0/24

const (
	ipv4PrefixLen   = 30
	instance        = "default"
	ateDstNetCIDR   = "198.51.100.0/24"
	nhIndex         = 1
	nhgIndex        = 42
	trafficDuration = 10 * time.Second
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "dutPort1",
		IPv4:    "192.0.2.1",
		IPv4Len: ipv4PrefixLen,
	}

	atePort1 = attrs.Attributes{
		Name:    "port1",
		MAC:     "00:00:01:01:01:01",
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
		MAC:     "00:00:02:01:01:01",
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
		MAC:     "00:00:03:01:01:01",
		IPv4:    "192.0.2.10",
		IPv4Len: ipv4PrefixLen,
	}
)

var inputMap = map[attrs.Attributes]attrs.Attributes{
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

	for ateInput, dutInput := range inputMap {
		t.Logf("OTG Add Interface: %v with IPv4 %v", ateInput.Name, ateInput.IPv4)
		top.Ports().Add().SetName(ateInput.Name)
		dev := top.Devices().Add().SetName(ateInput.Name)
		eth := dev.Ethernets().Add().SetName(ateInput.Name + ".eth").
			SetPortName(dev.Name()).SetMac(ateInput.MAC)
		eth.Ipv4Addresses().Add().SetName(dev.Name() + ".ipv4").
			SetAddress(ateInput.IPv4).SetGateway(dutInput.IPv4).
			SetPrefix(int32(ateInput.IPv4Len))
	}
	otg.PushConfig(t, top)
	otg.StartProtocols(t)
	return top
}

// testTraffic generates traffic flow from source network to
// destination network via srcEndPoint to dstEndPoint and checks for
// packet loss.
func testTraffic(t *testing.T, ate *ondatra.ATEDevice, config gosnappi.Config, srcEndPoint, dstEndPoint attrs.Attributes) {

	// srcEndPoint is atePort1
	otg := ate.OTG()
	gwIp := inputMap[srcEndPoint].IPv4
	dstMac, _ := otgutils.GetIPv4NeighborMacEntry(t, srcEndPoint.Name+".eth", gwIp, otg)
	config.Flows().Clear().Items()
	flowipv4 := config.Flows().Add().SetName("Flow")
	flowipv4.Metrics().SetEnable(true)
	flowipv4.TxRx().Port().
		SetTxName(srcEndPoint.Name).
		SetRxName(dstEndPoint.Name)
	flowipv4.Size().SetFixed(512)
	flowipv4.Rate().SetPps(10)
	flowipv4.Duration().SetChoice("continuous")
	e1 := flowipv4.Packet().Add().Ethernet()
	e1.Src().SetValue(srcEndPoint.MAC)
	e1.Dst().SetChoice("value").SetValue(dstMac)
	v4 := flowipv4.Packet().Add().Ipv4()
	v4.Src().SetValue(srcEndPoint.IPv4)
	v4.Dst().Increment().SetStart(strings.Split(ateDstNetCIDR, "/")[0]).SetCount(250)
	otg.PushConfig(t, config)

	t.Logf("Starting traffic")
	otg.StartTraffic(t)
	err := otgutils.WatchFlowMetrics(t, otg, config, &otgutils.WaitForOpts{Interval: 2 * time.Second, Timeout: trafficDuration})
	if err != nil {
		log.Println(err)
	}
	t.Logf("Stop traffic")
	otg.StopTraffic(t)

	pMetrics, err := otgutils.GetAllPortMetrics(t, otg, config)
	if err != nil {
		t.Fatal("Error while getting the port metrics")
	}
	otgutils.PrintMetricsTable(&otgutils.MetricsTableOpts{
		ClearPrevious:  false,
		AllPortMetrics: pMetrics,
	})

	fMetrics, err := otgutils.GetFlowMetrics(t, otg, config)
	if err != nil {
		t.Fatal("Error while getting the flow metrics")
	}
	otgutils.PrintMetricsTable(&otgutils.MetricsTableOpts{
		ClearPrevious: false,
		FlowMetrics:   fMetrics,
	})

	for _, f := range fMetrics.Items() {
		lostPackets := f.FramesTx() - f.FramesRx()
		lossPct := lostPackets * 100 / f.FramesTx()
		if lossPct > 0 && f.FramesTx() > 0 {
			t.Errorf("Loss Pct for Flow: %s got %v, want 0", f.Name(), lossPct)
		}
	}
}

// testArgs holds the objects needed by a test case.
type testArgs struct {
	ctx     context.Context
	clientA *gribi.Client
	clientB *gribi.Client
	dut     *ondatra.DUTDevice
	ate     *ondatra.ATEDevice
	top     gosnappi.Config
}

// testIPv4LeaderActiveChange first configures an IPV4 Entry through clientB
// and ensures that the entry is active by checking AFT Telemetry and traffic.
// It then configures an IPv4 entry through clientA without updating the election
// and ensures that the installation fails. Finally, it updated the ClientA election
// id to 12, configures an IPV4 through clinetA and ensures that the entry is active
// by checking AFT Telemetry and traffic.
func testIPv4LeaderActiveChange(ctx context.Context, t *testing.T, args *testArgs) {
	// Add an IPv4Entry for 198.51.100.0/24 pointing to ATE port-3 via gRIBI-B,
	// ensure that the entry is active through AFT telemetry and traffic.
	t.Logf("an IPv4Entry for %s pointing to ATE port-3 via gRIBI-B", ateDstNetCIDR)
	args.clientB.AddNH(t, nhIndex, atePort3.IPv4, instance, fluent.InstalledInRIB)
	args.clientB.AddNHG(t, nhgIndex, map[uint64]uint64{nhIndex: 1}, instance, fluent.InstalledInRIB)
	args.clientB.AddIPv4(t, ateDstNetCIDR, nhgIndex, instance, "", fluent.InstalledInRIB)

	// Verify the entry for 198.51.100.0/24 is active through AFT Telemetry.
	ipv4Path := args.dut.Telemetry().NetworkInstance(instance).Afts().Ipv4Entry(ateDstNetCIDR)
	if got, want := ipv4Path.Prefix().Get(t), ateDstNetCIDR; got != want {
		t.Errorf("ipv4-entry/state/prefix got %s, want %s", got, want)
	}

	// Verify the entry for 198.51.100.0/24 is active through Traffic.
	testTraffic(t, args.ate, args.top, atePort1, atePort3)

	// Add an IPv4Entry for 198.51.100.0/24 pointing to ATE port-2 via gRIBI-A,
	// ensure that the entry is ignored by the DUT.
	t.Logf("Adding an IPv4Entry for %s pointing to ATE port-2 via gRIBI-A", ateDstNetCIDR)
	args.clientA.AddNH(t, nhIndex, atePort2.IPv4, instance, fluent.ProgrammingFailed)
	args.clientA.AddNHG(t, nhgIndex, map[uint64]uint64{nhIndex: 1}, instance, fluent.ProgrammingFailed)
	args.clientA.AddIPv4(t, ateDstNetCIDR, nhgIndex, instance, "", fluent.ProgrammingFailed)

	// Send a ModifyRequest from gRIBI-A specifying election_id 12,
	// followed by a ModifyRequest updating 198.51.100.0/24 pointing to ATE port-2,
	// ensure that routing is updated to receive packets for 198.51.100.0/24 at ATE port-2.
	args.clientA.UpdateElectionID(t, 12, 0)
	t.Logf("Adding an IPv4Entry for %s pointing to ATE port-2 via client gRIBI-A", ateDstNetCIDR)
	args.clientA.AddNH(t, nhIndex, atePort2.IPv4, instance, fluent.InstalledInRIB)
	args.clientA.AddNHG(t, nhgIndex, map[uint64]uint64{nhIndex: 1}, instance, fluent.InstalledInRIB)
	args.clientA.AddIPv4(t, ateDstNetCIDR, nhgIndex, instance, "", fluent.InstalledInRIB)

	// Verify the entry for 198.51.100.0/24 is active through AFT Telemetry.
	ipv4Path = args.dut.Telemetry().NetworkInstance(instance).Afts().Ipv4Entry(ateDstNetCIDR)
	if got, want := ipv4Path.Prefix().Get(t), ateDstNetCIDR; got != want {
		t.Errorf("ipv4-entry/state/prefix got %s, want %s", got, want)
	}

	// Verify with traffic that the entry for 198.51.100.0/24 is installed through the ATE port-2.
	testTraffic(t, args.ate, args.top, atePort1, atePort2)
}

func TestElectionIDChange(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ctx := context.Background()

	// Configure the DUT
	configureDUT(t, dut)

	// Configure the ATE
	ate := ondatra.ATE(t, "ate")
	top := configureATE(t, ate)

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

	// Configure the gRIBI client clientB
	clientB := gribi.Client{
		DUT:                  dut,
		FibACK:               false,
		Persistence:          true,
		InitialElectionIDLow: 11,
	}
	defer clientB.Close(t)
	if err := clientB.Start(t); err != nil {
		t.Fatalf("gRIBI Connection can not be established")
	}

	args := &testArgs{
		ctx:     ctx,
		clientA: &clientA,
		clientB: &clientB,
		dut:     dut,
		ate:     ate,
		top:     top,
	}

	testIPv4LeaderActiveChange(ctx, t, args)
}
