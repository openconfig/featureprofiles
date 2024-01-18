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
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/gribigo/chk"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
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
	ateDstNetCIDR = "198.51.100.0/24"
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

type testArgs struct {
	dut        *ondatra.DUTDevice
	ate        *ondatra.ATEDevice
	ateTop     gosnappi.Config
	clientA    *fluent.GRIBIClient
	clientB    *fluent.GRIBIClient
	electionID gribi.Uint128
}

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

	gribic := dut.RawAPIs().GRIBI(t)

	// Configure the gRIBI client clientA with election ID of 10.
	clientA := fluent.NewClient()
	clientA.Connection().WithStub(gribic).
		WithPersistence().
		WithRedundancyMode(fluent.ElectedPrimaryClient).
		WithInitialElectionID(1 /* low */, 0 /* hi */) // ID must be > 0.

	clientB := fluent.NewClient()
	clientB.Connection().WithStub(gribic).
		WithPersistence().
		WithInitialElectionID(1, 0).
		WithRedundancyMode(fluent.ElectedPrimaryClient)

	clientA.Start(ctx, t)
	defer clientA.Stop(t)
	clientA.StartSending(ctx, t)
	if err := awaitTimeout(ctx, clientA, t, time.Minute); err != nil {
		t.Fatalf("Await got error during session negotiation: %v", err)
	}

	clientB.Start(ctx, t)
	defer clientB.Stop(t)

	defer func() {
		// Flush all entries after test.
		if err := gribi.FlushAll(clientB); err != nil {
			t.Error(err)
		}
	}()

	clientB.StartSending(ctx, t)
	if err := awaitTimeout(ctx, clientB, t, time.Minute); err != nil {
		t.Fatalf("Await got error during session negotiation: %v", err)
	}

	// Make clientA the leader
	eID := gribi.BecomeLeader(t, clientA)

	// gRIBI-B electionID = leaderElectionID-1
	gribi.UpdateElectionID(t, clientB, eID.Decrement())

	args := &testArgs{
		dut:        dut,
		ate:        ate,
		ateTop:     ateTop,
		clientA:    clientA,
		clientB:    clientB,
		electionID: eID,
	}

	testFlushWithDefaultNetworkInstance(ctx, t, args)
}

// testFlushWithDefaultNetWorkInstance tests flush with default network instance
func testFlushWithDefaultNetworkInstance(ctx context.Context, t *testing.T, args *testArgs) {
	// Inject an entry into the default network instance pointing to ATE port-2.
	// clientA is primary client
	injectEntry(ctx, t, args.clientA, deviations.DefaultNetworkInstance(args.dut))
	otgutils.WaitForARP(t, args.ate.OTG(), args.ateTop, "IPv4")
	// Test traffic between ATE port-1 and ATE port-2.
	lossPct := testTraffic(t, args.ate, args.ateTop)
	if got := lossPct; got > 0 {
		t.Errorf("LossPct for flow got %v, want 0", got)
	} else {
		t.Log("Traffic is forwarded between ATE port-1 and ATE port-2")
	}

	// Flush should delete the entries
	if _, err := gribi.Flush(args.clientA, args.electionID, deviations.DefaultNetworkInstance(args.dut)); err != nil {
		t.Errorf("Unexpected error from flush, got: %v", err)
	}
	// After flush, left entry should be 0, and packets can no longer be forwarded.
	lossPct = testTraffic(t, args.ate, args.ateTop)
	if got := lossPct; got == 0 {
		t.Error("Traffic can still be forwarded between ATE port-1 and ATE port-2")
	} else {
		t.Log("Traffic is not forwarded between ATE port-1 and ATE port-2")
	}
	if got, want := checkNIHasNEntries(ctx, args.clientA, deviations.DefaultNetworkInstance(args.dut), t), 0; got != want {
		t.Errorf("Network instance has %d entry/entries, wanted: %d", got, want)
	}

	// clientA is primary client
	injectEntry(ctx, t, args.clientA, deviations.DefaultNetworkInstance(args.dut))

	// flush should fail, and preserve 3 entries.
	if res, err := gribi.Flush(args.clientB, args.electionID.Decrement(), deviations.DefaultNetworkInstance(args.dut)); err == nil {
		t.Errorf("Flush should return an error, got response: %v", res)
	}

	if got, want := checkNIHasNEntries(ctx, args.clientB, deviations.DefaultNetworkInstance(args.dut), t), 3; got != want {
		t.Errorf("Network instance has %d entry/entries, wanted: %d", got, want)
	}

	// Increases clientB's election ID to makes it be the primary client.
	eID := gribi.BecomeLeader(t, args.clientB)

	// Flush should be succeed and 0 entry left.
	if _, err := gribi.Flush(args.clientB, eID, deviations.DefaultNetworkInstance(args.dut)); err != nil {
		t.Fatalf("Unexpected error from flush, got: %v", err)
	}

	if got, want := checkNIHasNEntries(ctx, args.clientB, deviations.DefaultNetworkInstance(args.dut), t), 0; got != want {
		t.Errorf("Network instance has %d entry/entries, wanted: %d", got, want)
	}
}

// configureDUT configures port1-2 on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	d := gnmi.OC()

	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")

	gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), dutPort1.NewOCInterface(p1.Name(), dut))
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, p1.Name(), deviations.DefaultNetworkInstance(dut), 0)
	}
	gnmi.Replace(t, dut, d.Interface(p2.Name()).Config(), dutPort2.NewOCInterface(p2.Name(), dut))
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, p2.Name(), deviations.DefaultNetworkInstance(dut), 0)
	}
	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, p1)
		fptest.SetPortSpeed(t, p2)
	}

}

// configureATE configures port1, port2 on the ATE.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	t.Helper()
	top := gosnappi.NewConfig()

	p1 := ate.Port(t, "port1")
	p2 := ate.Port(t, "port2")

	atePort1.AddToOTG(top, p1, &dutPort1)
	atePort2.AddToOTG(top, p2, &dutPort2)

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

// testTraffic generates traffic flow from source network to
// destination network via srcEndPoint to dstEndPoint and checks for
// packet loss.
func testTraffic(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config) float32 {
	// Ensure that traffic can be forwarded between ATE port-1 and ATE port-2.
	t.Helper()
	otg := ate.OTG()
	top.Flows().Clear().Items()
	flowipv4 := top.Flows().Add().SetName("Flow")
	flowipv4.Metrics().SetEnable(true)
	flowipv4.TxRx().Device().
		SetTxNames([]string{atePort1.Name + ".IPv4"}).
		SetRxNames([]string{atePort2.Name + ".IPv4"})

	flowipv4.Duration().Continuous()
	e1 := flowipv4.Packet().Add().Ethernet()
	e1.Src().SetValue(atePort1.MAC)
	v4 := flowipv4.Packet().Add().Ipv4()
	v4.Src().SetValue(atePort1.IPv4)
	v4.Dst().Increment().SetStart("198.51.100.1").SetCount(250)
	otg.PushConfig(t, top)
	otg.StartProtocols(t)

	otg.StartTraffic(t)
	time.Sleep(15 * time.Second)
	t.Logf("Stop traffic")
	otg.StopTraffic(t)

	txPkts, rxPkts := otgutils.GetFlowStats(t, otg, "Flow", 5*time.Second)
	otgutils.LogFlowMetrics(t, otg, top)
	lossPct := float32(txPkts-rxPkts) * 100 / float32(txPkts)
	return lossPct
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
