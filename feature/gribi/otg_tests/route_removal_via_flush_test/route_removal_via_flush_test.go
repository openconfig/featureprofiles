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

package route_removal_via_flush_test

import (
	"context"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	gpb "github.com/openconfig/gribi/v1/proto/service"
	"github.com/openconfig/gribigo/chk"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	otgtelemetry "github.com/openconfig/ondatra/telemetry/otg"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Settings for configuring the baseline testbed with the test
// topology.
//
// The testbed consists of ate:port1 -> dut:port1,
// dut:port2 -> ate:port2.
//
//   * ate:port1 -> dut:port1 subnet 192.0.2.0/30
//   * ate:port2 -> dut:port2 subnet 192.0.2.4/30
//
//   * Destination network: 198.51.100.0/24

const (
	ateDstNetCIDR            = "198.51.100.0/24"
	clientAOriginElectionID  = 10
	clientBOriginElectionID  = 9
	clientAUpdatedElectionID = 12
	clientBUpdatedElectionID = 11
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "dutPort1",
		IPv4:    "192.0.2.1",
		IPv4Len: 30,
	}

	atePort1 = attrs.Attributes{
		Name:    "atePort1",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.0.2.2",
		IPv4Len: 30,
	}

	dutPort2 = attrs.Attributes{
		Desc:    "dutPort2",
		IPv4:    "192.0.2.5",
		IPv4Len: 30,
	}

	atePort2 = attrs.Attributes{
		Name:    "atePort2",
		MAC:     "02:00:02:01:01:01",
		IPv4:    "192.0.2.6",
		IPv4Len: 30,
	}
)

// TestRouteRemovelViaFlush test flush with the following operations
// 1. Flush request from clientA (the primary client) should succeed.
// 2. Flush request from clientB (not a primary client) should fail.
// 3. Failover the primary role from clientA to clientB.
// 4. Flush from clientB should succeed.
func TestRouteRemovelViaFlush(t *testing.T) {
	ctx := context.Background()

	dut := ondatra.DUT(t, "dut")
	configureDUT(t, dut)

	ate := ondatra.ATE(t, "ate")
	ateTop := configureATE(t, ate)
	ate.OTG().PushConfig(t, ateTop)
	ate.OTG().StartProtocols(t)

	gribic := dut.RawAPIs().GRIBI().Default(t)

	// Configure the gRIBI client clientA with election ID of 10.
	clientA := fluent.NewClient()
	clientA.Connection().WithStub(gribic).
		WithPersistence().
		WithRedundancyMode(fluent.ElectedPrimaryClient).
		WithInitialElectionID(clientAOriginElectionID /* low */, 0 /* hi */) // ID must be > 0.

	clientB := fluent.NewClient()
	clientB.Connection().WithStub(gribic).
		WithPersistence().
		WithInitialElectionID(clientBOriginElectionID, 0).
		WithRedundancyMode(fluent.ElectedPrimaryClient)

	clientA.Start(ctx, t)
	defer clientA.Stop(t)
	clientA.StartSending(ctx, t)
	if err := awaitTimeout(ctx, clientA, t, time.Minute); err != nil {
		t.Fatalf("Await got error during session negotiation: %v", err)
	}

	clientB.Start(ctx, t)
	defer clientB.Stop(t)
	clientB.StartSending(ctx, t)
	if err := awaitTimeout(ctx, clientB, t, time.Minute); err != nil {
		t.Fatalf("Await got error during session negotiation: %v", err)
	}

	testFlushWithDefaultNetworkInstance(ctx, t, clientA, clientB, ate, ateTop)

}

// testFlushWithDefaultNetWorkInstance tests flush with default network instance
func testFlushWithDefaultNetworkInstance(ctx context.Context, t *testing.T, clientA, clientB *fluent.GRIBIClient, ate *ondatra.ATEDevice, ateTop gosnappi.Config) {
	// Inject an entry into the default network instance pointing to ATE port-2.
	// clientA is primary client
	injectEntry(ctx, t, clientA, *deviations.DefaultNetworkInstance)
	// Test traffic between ATE port-1 and ATE port-2.
	lossPct := testTraffic(t, ate, ateTop)
	if got := lossPct; got > 0 {
		t.Errorf("LossPct for flow got %v, want 0", got)
	} else {
		t.Log("Traffic can be forwarded between ATE port-1 and ATE port-2")
	}

	_, err := flush(ctx, t, clientA, clientAOriginElectionID, *deviations.DefaultNetworkInstance)
	if err != nil {
		t.Errorf("Unexpected error from flush, got: %v", err)
	}
	// After flush, left entry should be 0, and packets can no longer be forwarded.
	lossPct = testTraffic(t, ate, ateTop)
	if got := lossPct; got == 0 {
		t.Error("Traffic can still be forwarded between ATE port-1 and ATE port-2")
	} else {
		t.Log("Traffic can not be forwarded between ATE port-1 and ATE port-2")
	}
	leftEntries := checkNIHasNEntries(ctx, clientA, *deviations.DefaultNetworkInstance, t)
	if leftEntries != 0 {
		t.Errorf("Network instance has %d entry/entries, wanted: %d", leftEntries, 0)
	}

	// clientA is primary client
	injectEntry(ctx, t, clientA, *deviations.DefaultNetworkInstance)

	// flush should be failed, and remains 3 entries.
	flushRes, err := flush(ctx, t, clientB, clientBOriginElectionID, *deviations.DefaultNetworkInstance)
	if err == nil {
		t.Errorf("Flush should return an error, got response: %v", flushRes)
	}
	leftEntries = checkNIHasNEntries(ctx, clientB, *deviations.DefaultNetworkInstance, t)
	if leftEntries != 3 {
		t.Errorf("Network instance has %d entry/entries, wanted: %d", leftEntries, 3)
	}

	// Increases clientB's election ID to makes it be the primary client.
	clientB.Modify().UpdateElectionID(t, clientBUpdatedElectionID, 0)

	// Flush should be succeed and 0 entry left.
	_, err = flush(ctx, t, clientB, clientBUpdatedElectionID, *deviations.DefaultNetworkInstance)
	if err != nil {
		t.Fatalf("Unexpected error from flush, got: %v", err)
	}
	leftEntries = checkNIHasNEntries(ctx, clientB, *deviations.DefaultNetworkInstance, t)
	if leftEntries != 0 {
		t.Errorf("Network instance has %d entry/entries, wanted: %d", leftEntries, 0)
	}
}

// configureDUT configures port1-2 on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	d := dut.Config()

	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")

	d.Interface(p1.Name()).Replace(t, dutPort1.NewInterface(p1.Name()))
	d.Interface(p2.Name()).Replace(t, dutPort2.NewInterface(p2.Name()))

}

// configureATE configures port1, port2 on the ATE.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	t.Helper()
	otg := ate.OTG()
	top := otg.NewConfig(t)

	top.Ports().Add().SetName(ate.Port(t, "port1").ID())
	i1 := top.Devices().Add().SetName(ate.Port(t, "port1").ID())
	eth1 := i1.Ethernets().Add().SetName(atePort1.Name + ".Eth").
		SetPortName(i1.Name()).SetMac(atePort1.MAC)
	eth1.Ipv4Addresses().Add().SetName(atePort1.Name + ".IPv4").
		SetAddress(atePort1.IPv4).SetGateway(dutPort1.IPv4).
		SetPrefix(int32(atePort1.IPv4Len))

	top.Ports().Add().SetName(ate.Port(t, "port2").ID())
	i2 := top.Devices().Add().SetName(ate.Port(t, "port2").ID())
	eth2 := i2.Ethernets().Add().SetName(atePort2.Name + ".Eth").
		SetPortName(i2.Name()).SetMac(atePort2.MAC)
	eth2.Ipv4Addresses().Add().SetName(atePort2.Name + ".IPv4").
		SetAddress(atePort2.IPv4).SetGateway(dutPort2.IPv4).
		SetPrefix(int32(atePort2.IPv4Len))

	return top
}

// awaitTimeout calls a fluent client Await, adding a timeout to the context.
func awaitTimeout(ctx context.Context, c *fluent.GRIBIClient, t testing.TB, timeout time.Duration) error {
	subctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return c.Await(subctx, t)
}

// injectEntry adds a fully referenced IPv4Entry.
func injectEntry(ctx context.Context, t *testing.T, client *fluent.GRIBIClient, networkInstanceName string) {
	t.Helper()
	client.Modify().AddEntry(t,
		fluent.NextHopEntry().
			WithNetworkInstance(networkInstanceName).
			WithIndex(1).
			WithIPAddress("192.0.2.6"),
		fluent.NextHopGroupEntry().
			WithNetworkInstance(networkInstanceName).
			WithID(42).
			AddNextHop(1, 1),
		fluent.IPv4Entry().
			WithNetworkInstance(networkInstanceName).
			WithPrefix(ateDstNetCIDR).
			WithNextHopGroupNetworkInstance(networkInstanceName).
			WithNextHopGroup(42),
	)

	if err := awaitTimeout(ctx, client, t, time.Minute); err != nil {
		t.Fatalf("Unexpected error from server - entries, got: %v, want: nil", err)
	}
	res := client.Results(t)

	// Check the three entries in order.
	chk.HasResult(t, res,
		fluent.OperationResult().
			WithOperationID(1).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
	)
	chk.HasResult(t, res,
		fluent.OperationResult().
			WithOperationID(2).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
	)
	chk.HasResult(t, res,
		fluent.OperationResult().
			WithOperationID(3).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
	)
}

// Waits for at least one ARP entry on any OTG interface
func waitOTGARPEntry(t *testing.T) {
	ate := ondatra.ATE(t, "ate")
	ate.OTG().Telemetry().InterfaceAny().Ipv4NeighborAny().LinkLayerAddress().Watch(
		t, time.Minute, func(val *otgtelemetry.QualifiedString) bool {
			return val.IsPresent()
		}).Await(t)
}

// testTraffic generates traffic flow from source network to
// destination network via srcEndPoint to dstEndPoint and checks for
// packet loss.
func testTraffic(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config) int {
	// Ensure that traffic can be forwarded between ATE port-1 and ATE port-2.
	t.Helper()
	otg := ate.OTG()
	waitOTGARPEntry(t)
	dstMac := otg.Telemetry().Interface(atePort1.Name + ".Eth").Ipv4Neighbor(dutPort1.IPv4).LinkLayerAddress().Get(t)
	top.Flows().Clear().Items()
	flowipv4 := top.Flows().Add().SetName("Flow")
	flowipv4.Metrics().SetEnable(true)
	flowipv4.TxRx().Port().
		SetTxName(ate.Port(t, "port1").ID()).
		SetRxName(ate.Port(t, "port2").ID())
	flowipv4.Duration().SetChoice("continuous")
	e1 := flowipv4.Packet().Add().Ethernet()
	e1.Src().SetValue(atePort1.MAC)
	e1.Dst().SetChoice("value").SetValue(dstMac)
	v4 := flowipv4.Packet().Add().Ipv4()
	v4.Src().SetValue(atePort1.IPv4)
	v4.Dst().Increment().SetStart("198.51.100.1").SetCount(250)
	otg.PushConfig(t, top)
	otg.StartProtocols(t)

	otg.StartTraffic(t)
	time.Sleep(15 * time.Second)
	t.Logf("Stop traffic")
	otg.StopTraffic(t)

	otgutils.LogFlowMetrics(t, otg, top)
	time.Sleep(time.Minute)
	txPkts := int(otg.Telemetry().Flow("Flow").Counters().OutPkts().Get(t))
	rxPkts := int(otg.Telemetry().Flow("Flow").Counters().InPkts().Get(t))
	lossPct := (txPkts - rxPkts) * 100 / txPkts
	return lossPct
}

// flush flushes all the state on the server, but does not validate it specifically.
func flush(ctx context.Context, t *testing.T, client *fluent.GRIBIClient, electionID uint64, networkInstanceName string) (*gpb.FlushResponse, error) {
	t.Helper()
	res, err := client.Flush().
		WithElectionID(electionID, 0).
		WithNetworkInstance(networkInstanceName).
		Send()
	return res, err
}

// checkNIHasNEntries uses the Get RPC to validate that the network instance named ni
// contains want (an integer) entries.
func checkNIHasNEntries(ctx context.Context, c *fluent.GRIBIClient, ni string, t testing.TB) int {
	t.Helper()
	gr, err := c.Get().
		WithNetworkInstance(ni).
		WithAFT(fluent.AllAFTs).
		Send()

	if err != nil {
		t.Fatalf("Unexpected error from get, got: %v", err)
	}
	return len(gr.GetEntry())
}
