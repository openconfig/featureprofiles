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
	"github.com/openconfig/featureprofiles/helpers"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"

	"github.com/openconfig/gribigo/chk"
	"github.com/openconfig/gribigo/constants"
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
	trafficDuration = 30 * time.Second
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

type trafficEndpoint struct {
	endpointName string
	mac          string
	ip           string
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

	inputMap := map[attrs.Attributes]attrs.Attributes{
		atePort1: dutPort1,
		atePort2: dutPort2,
		atePort3: dutPort3,
	}
	for ateInput, dutInput := range inputMap {
		t.Logf("OTG AddInterface: %v", ateInput)
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
// testTraffic generates traffic flow from source network to
// destination network via srcEndPoint to dstEndPoint and checks for
// packet loss. This is built for otg

func testTraffic(t *testing.T, ate *ondatra.ATEDevice, config gosnappi.Config, srcEndPoint, dstEndPoint trafficEndpoint) {

	otg := ate.OTG()
	config.Flows().Clear().Items()
	flowipv4 := config.Flows().Add().SetName("Flow")
	flowipv4.Metrics().SetEnable(true)
	flowipv4.TxRx().Port().
		SetTxName(srcEndPoint.endpointName).
		SetRxName(dstEndPoint.endpointName)
	flowipv4.Size().SetFixed(512)
	flowipv4.Rate().SetPps(2)
	flowipv4.Duration().SetChoice("continuous")
	e1 := flowipv4.Packet().Add().Ethernet()
	e1.Src().SetValue(srcEndPoint.mac)
	v4 := flowipv4.Packet().Add().Ipv4()
	v4.Src().SetValue(srcEndPoint.ip)
	v4.Dst().Increment().SetStart(strings.Split(ateDstNetCIDR, "/")[0]).SetCount(250)
	otg.PushConfig(t, config)

	t.Logf("Starting traffic")
	otg.StartTraffic(t)
	err := helpers.WatchFlowMetrics(t, otg, config, &helpers.WaitForOpts{Interval: 2 * time.Second, Timeout: trafficDuration})
	if err != nil {
		log.Println(err)
	}
	t.Logf("Stop traffic")
	otg.StopTraffic(t)

	fMetrics, err := helpers.GetFlowMetrics(t, otg, config)
	if err != nil {
		t.Fatal("Error while getting the flow metrics")
	}

	helpers.PrintMetricsTable(&helpers.MetricsTableOpts{
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

// awaitTimeout calls a fluent client Await, adding a timeout to the context.
func awaitTimeout(ctx context.Context, c *fluent.GRIBIClient, t testing.TB, timeout time.Duration) error {
	subctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return c.Await(subctx, t)
}

// testArgs holds the objects needed by a test case.
type testArgs struct {
	ctx     context.Context
	clientA *fluent.GRIBIClient
	clientB *fluent.GRIBIClient
	dut     *ondatra.DUTDevice
	ate     *ondatra.ATEDevice
	top     gosnappi.Config
}

// helperAddEntry configures a sequence of adding the NH, NHG and IPv4Entry by a client.
func helperAddEntry(ctx context.Context, t *testing.T, client *fluent.GRIBIClient, nextHop string) {
	t.Helper()
	client.Modify().AddEntry(t,
		fluent.NextHopEntry().
			WithNetworkInstance(instance).
			WithIndex(nhIndex).
			WithIPAddress(nextHop),
		fluent.NextHopGroupEntry().
			WithNetworkInstance(instance).
			WithID(nhgIndex).
			AddNextHop(nhIndex, 1),
		fluent.IPv4Entry().
			WithNetworkInstance(instance).
			WithPrefix(ateDstNetCIDR).
			WithNextHopGroup(nhgIndex),
	)

	if err := awaitTimeout(ctx, client, t, time.Minute); err != nil {
		t.Fatalf("Could not program entries via %v, got err: %v", client, err)
	}

}

// configureIPv4ViaClientB configures a IPv4 Entry via ClientB with an Election
// ID of 11 when ClientA is already connected with Election ID of 10. Ensure
// that the entry via ClientB is active through AFT Telemetry.
func configureIPv4ViaClientB(t *testing.T, args *testArgs) {
	t.Logf("Adding an IPv4Entry for %s pointing to ATE port-3 via clientB.", ateDstNetCIDR)

	helperAddEntry(args.ctx, t, args.clientB, "192.0.2.10")

	chk.HasResult(t, args.clientB.Results(t),
		fluent.OperationResult().
			WithNextHopOperation(nhIndex).
			WithOperationType(constants.Add).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		chk.IgnoreOperationID(),
	)

	chk.HasResult(t, args.clientB.Results(t),
		fluent.OperationResult().
			WithNextHopGroupOperation(nhgIndex).
			WithOperationType(constants.Add).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		chk.IgnoreOperationID(),
	)

	chk.HasResult(t, args.clientB.Results(t),
		fluent.OperationResult().
			WithIPv4Operation(ateDstNetCIDR).
			WithOperationType(constants.Add).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		chk.IgnoreOperationID(),
	)
}

// configureIPv4ViaClientA configures a IPv4 Entry via ClientA with an Election
// ID of 10 when ClientB is already primary and connected with Election ID of
// 11. Ensure that the entry via ClientA is ignored and not installed.
func configureIPv4ViaClientA(t *testing.T, args *testArgs) {
	t.Logf("Adding an IPv4Entry for %s pointing to ATE port-2 via clientA.", ateDstNetCIDR)

	helperAddEntry(args.ctx, t, args.clientA, "192.0.2.6")

	// Verify the entry is not installed due to client A having lower election ID.
	chk.HasResult(t, args.clientA.Results(t),
		fluent.OperationResult().
			WithIPv4Operation(ateDstNetCIDR).
			WithOperationType(constants.Add).
			WithProgrammingResult(fluent.ProgrammingFailed).
			AsResult(),
		chk.IgnoreOperationID(),
	)
}

// configureIPv4ViaClientAInstalled configures a IPv4 Entry via ClientA with an
// Election ID of 12. Ensure that the entry via ClientA is installed.
func configureIPv4ViaClientAInstalled(t *testing.T, args *testArgs) {
	t.Logf("Adding an IPv4Entry for %s pointing to ATE port-2 via clientA with election ID of 12.", ateDstNetCIDR)

	// TODO: Remove WithElectionID and reuse helperAddEntry
	args.clientA.Modify().AddEntry(t,
		fluent.NextHopEntry().
			WithNetworkInstance(instance).
			WithIndex(nhIndex).
			WithIPAddress("192.0.2.6").
			WithElectionID(12, 0))

	args.clientA.Modify().AddEntry(t,
		fluent.NextHopGroupEntry().
			WithNetworkInstance(instance).
			WithID(nhgIndex).
			AddNextHop(nhIndex, 1).
			WithElectionID(12, 0))

	args.clientA.Modify().AddEntry(t,
		fluent.IPv4Entry().
			WithPrefix(ateDstNetCIDR).
			WithNetworkInstance(instance).
			WithNextHopGroup(nhgIndex).
			WithElectionID(12, 0))

	if err := awaitTimeout(args.ctx, args.clientA, t, time.Minute); err != nil {
		t.Fatalf("Could not program entries via clientA, got err: %v", err)
	}

	chk.HasResult(t, args.clientA.Results(t),
		fluent.OperationResult().
			WithNextHopOperation(nhIndex).
			WithOperationType(constants.Add).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		chk.IgnoreOperationID(),
	)

	chk.HasResult(t, args.clientA.Results(t),
		fluent.OperationResult().
			WithNextHopGroupOperation(nhgIndex).
			WithOperationType(constants.Add).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		chk.IgnoreOperationID(),
	)

	chk.HasResult(t, args.clientA.Results(t),
		fluent.OperationResult().
			WithIPv4Operation(ateDstNetCIDR).
			WithOperationType(constants.Add).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		chk.IgnoreOperationID(),
	)
}

// testIPv4LeaderActiveChange modifies election ID of ClientA with an Election ID of 12
// and configures a IPv4 entry through this client. Ensure that the entry via ClientA
// is active through AFT Telemetry.
func testIPv4LeaderActiveChange(ctx context.Context, t *testing.T, args *testArgs) {
	// Configure IPv4 route for 198.51.100.0/24 pointing to ATE port-3 via clientB.
	configureIPv4ViaClientB(t, args)

	// Verify the entry for 198.51.100.0/24 is active through AFT Telemetry.
	ipv4Path := args.dut.Telemetry().NetworkInstance(instance).Afts().Ipv4Entry(ateDstNetCIDR)
	if got, want := ipv4Path.Prefix().Get(t), ateDstNetCIDR; got != want {
		t.Errorf("ipv4-entry/state/prefix got %s, want %s", got, want)
	}

	srcEndPoint := trafficEndpoint{atePort1.Name, atePort1.MAC, atePort1.IPv4}
	dstEndPoint := trafficEndpoint{atePort3.Name, atePort3.MAC, atePort3.IPv4}
	testTraffic(t, args.ate, args.top, srcEndPoint, dstEndPoint)

	// Configure IPv4 route for 198.51.100.0/24 pointing to ATE port-3 via clientB.
	// The entry should not be installed due to client A having lower election ID.
	configureIPv4ViaClientA(t, args)

	// Modify the election ID of client A to 12 so clientA becomes the active Leader.
	args.clientA.Modify().UpdateElectionID(t, 12, 0)

	if err := awaitTimeout(ctx, args.clientA, t, time.Minute); err != nil {
		t.Fatalf("could not update election ID via clientA, got err: %v", err)
	}

	chk.HasResult(t, args.clientA.Results(t),
		fluent.OperationResult().
			WithCurrentServerElectionID(12, 0).
			AsResult(),
		chk.IgnoreOperationID(),
	)

	// Configure IPv4 route for 198.51.100.0/24 pointing to ATE port-2 via clientA with election ID of 12.
	configureIPv4ViaClientAInstalled(t, args)

	ipv4Path = args.dut.Telemetry().NetworkInstance(instance).Afts().Ipv4Entry(ateDstNetCIDR)
	if got, want := ipv4Path.Prefix().Get(t), ateDstNetCIDR; got != want {
		t.Errorf("ipv4-entry/state/prefix got %s, want %s", got, want)
	}

	// Verify with traffic that the entry is installed through the ATE port-2.

	srcEndPoint = trafficEndpoint{atePort1.Name, atePort1.MAC, atePort1.IPv4}
	dstEndPoint = trafficEndpoint{atePort2.Name, atePort2.MAC, atePort2.IPv4}
	testTraffic(t, args.ate, args.top, srcEndPoint, dstEndPoint)
}
func TestElectionIDChange(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	// Dial gRIBI
	ctx := context.Background()
	gribic := dut.RawAPIs().GRIBI().Default(t)

	// Configure the DUT
	configureDUT(t, dut)

	// Configure the ATE
	ate := ondatra.ATE(t, "ate")

	top := configureATE(t, ate)

	tt := struct {
		name string
		desc string
		fn   func(ctx context.Context, t *testing.T, args *testArgs)
	}{
		name: "IPv4EntryWithLeaderChange",
		desc: "Connect gRIBI-A to DUT specifying SINGLE_PRIMARY client redundancy with election_id 12.",
		fn:   testIPv4LeaderActiveChange,
	}

	// Each case will run with its own gRIBI fluent client.
	t.Run(tt.name, func(t *testing.T) {
		t.Logf("Name: %s", tt.name)
		t.Logf("Description: %s", tt.desc)

		// Configure the gRIBI client clientA with election ID of 10.
		clientA := fluent.NewClient()

		clientA.Connection().WithStub(gribic).WithInitialElectionID(10, 0).
			WithRedundancyMode(fluent.ElectedPrimaryClient)

		clientA.Start(ctx, t)
		defer clientA.Stop(t)
		clientA.StartSending(ctx, t)
		if err := awaitTimeout(ctx, clientA, t, time.Minute); err != nil {
			t.Fatalf("Await got error during session negotiation for clientA: %v", err)
		}
		// Configure the gRIBI client clientB with election ID of 11.
		clientB := fluent.NewClient()

		clientB.Connection().WithStub(gribic).WithInitialElectionID(11, 0).
			WithRedundancyMode(fluent.ElectedPrimaryClient)

		clientB.Start(context.Background(), t)
		defer clientB.Stop(t)
		clientB.StartSending(context.Background(), t)
		if err := awaitTimeout(ctx, clientB, t, time.Minute); err != nil {
			t.Fatalf("Await got error during session negotiation for clientB: %v", err)
		}
		args := &testArgs{
			ctx:     ctx,
			clientA: clientA,
			clientB: clientB,
			dut:     dut,
			ate:     ate,
			top:     top,
		}

		tt.fn(ctx, t, args)
	})
}

func TestUnsetDut(t *testing.T) {
	t.Logf("Start Unsetting DUT Config")
	dut := ondatra.DUT(t, "dut")
	dut.Config().New().WithAristaFile("unset_dut.txt").Push(t)
}
