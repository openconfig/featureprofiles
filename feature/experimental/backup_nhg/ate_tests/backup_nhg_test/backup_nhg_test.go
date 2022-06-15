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

// next hop groups are honored for next hop groups containing a single next hop.
package backup_nhg_test

import (
	"context"
	"testing"
	"time"

	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/telemetry"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/fptest"
)

type testArgs struct {
	ate    *ondatra.ATEDevice
	ateTop *ondatra.ATETopology
	ctx    context.Context
	dut    *ondatra.DUTDevice
	gribic *fluent.GRIBIClient
}

const (
	defaultNetworkInstance = "default"

	// Destination prefix for DUT to ATE traffic.
	dstPfx      = "198.51.100.0/24"
	dstPfxMin   = "198.51.100.0"
	dstPfxMax   = "198.51.100.255"
	dstPfxCount = 256
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "DUT Port 1",
		IPv4:    "192.0.2.1",
		IPv4Len: 30,
	}
	dutPort2 = attrs.Attributes{
		Desc:    "DUT Port 2",
		IPv4:    "192.0.2.5",
		IPv4Len: 30,
	}
	dutPort3 = attrs.Attributes{
		Desc:    "DUT Port 3",
		IPv4:    "192.0.2.9",
		IPv4Len: 30,
	}

	atePort1 = attrs.Attributes{
		Name:    "atePort1",
		Desc:    "ATE Port 1",
		IPv4:    "192.0.2.2",
		IPv4Len: 30,
	}
	atePort2 = attrs.Attributes{
		Name:    "atePort2",
		Desc:    "ATE Port 2",
		IPv4:    "192.0.2.6",
		IPv4Len: 30,
	}
	atePort3 = attrs.Attributes{
		Name:    "atePort3",
		Desc:    "ATE Port 3",
		IPv4:    "192.0.2.10",
		IPv4Len: 30,
	}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// TestDirectBackupNexthopGroup validates a prefix directly linked to a next hop
// group with a backup uses the backup when the primary next hop becomes
// impaired.
//
// Setup Steps
//   * Connect DUT port 1 <> ATE port 1.
//   * Connect DUT port 2 <> ATE port 2.
//   * Connect DUT port 3 <> ATE port 3.
//   * Create prefix 198.51.100.0/24, next hop group, and next hop to forward to
//     ATE port 2 with GRIBI.
//   * Assign backup next hop group to forward to ATE port 3 with GRIBI.
//
// Validation Steps
//   * Verify AFT telemetry shows ATE port 2 selected.
//   * Verify traffic flows to ATE port 2 and not ATE port 3.
//   * After each impairment, verify traffic flows to ATE port 3 and not ATE
//     port 2.
//
// Impairments
//   * Interface ATE port-2 is disabled.
//   * Interface DUT port-2 is disabled.
//   * TODO: Static ARP entry for ATE port 2 is removed from DUT via configuration, with no dynamic ARP enabled.
func TestDirectBackupNexthopGroup(t *testing.T) {
	ctx := context.Background()

	dut := ondatra.DUT(t, "dut")
	configureDUT(t, dut)

	ate := ondatra.ATE(t, "ate")
	ateTop := configureATE(t, ate)

	gribic := dut.RawAPIs().GRIBI().Default(t)
	c := fluent.NewClient()
	c.Connection().WithStub(gribic).
		WithPersistence().
		WithRedundancyMode(fluent.ElectedPrimaryClient).
		WithInitialElectionID(1 /* low */, 0 /* hi */) // ID must be > 0.
	c.Start(ctx, t)
	defer c.Stop(t)
	c.StartSending(ctx, t)
	if err := awaitTimeout(ctx, c, t, time.Minute); err != nil {
		t.Fatalf("Await got error during session negotiation: %v", err)
	}

	tcArgs := &testArgs{
		ate:    ate,
		ateTop: ateTop,
		ctx:    ctx,
		dut:    dut,
		gribic: c,
	}

	tcArgs.configureBackupNextHopGroup(t, false)

	baselineFlow := tcArgs.createFlow("Baseline Path Flow", &atePort2)
	backupFlow := tcArgs.createFlow("Backup Path Flow", &atePort3)

	cases := []struct {
		desc               string
		applyImpairmentFn  func()
		removeImpairmentFn func()
	}{
		{
			desc: "Disable ATE port-2",
			applyImpairmentFn: func() {
				ateP2 := ate.Port(t, "port2")
				dutP2 := dut.Port(t, "port2")
				ate.Actions().NewSetPortState().WithPort(ateP2).WithEnabled(false).Send(t)
				dut.Telemetry().Interface(dutP2.Name()).OperStatus().Await(t, time.Minute, telemetry.Interface_OperStatus_DOWN)
			},
			removeImpairmentFn: func() {
				ateP2 := ate.Port(t, "port2")
				dutP2 := dut.Port(t, "port2")
				ate.Actions().NewSetPortState().WithPort(ateP2).WithEnabled(true).Send(t)
				dut.Telemetry().Interface(dutP2.Name()).OperStatus().Await(t, time.Minute, telemetry.Interface_OperStatus_UP)
			},
		},
		{
			desc: "Disable DUT port-2",
			applyImpairmentFn: func() {
				dutP2 := dut.Port(t, "port2")
				dut.Config().Interface(dutP2.Name()).Enabled().Replace(t, false)
				dut.Telemetry().Interface(dutP2.Name()).OperStatus().Await(t, time.Minute, telemetry.Interface_OperStatus_DOWN)
			},
			removeImpairmentFn: func() {
				dutP2 := dut.Port(t, "port2")
				dut.Config().Interface(dutP2.Name()).Enabled().Replace(t, true)
				dut.Telemetry().Interface(dutP2.Name()).OperStatus().Await(t, time.Minute, telemetry.Interface_OperStatus_UP)
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			t.Run("Validate Baseline AFT Telemetry", func(t *testing.T) {
				tcArgs.validateAftTelemetry(t)
			})

			t.Run("Validate Baseline Traffic Delivery", func(t *testing.T) {
				tcArgs.validateTrafficFlows(t, baselineFlow, backupFlow)
			})

			tc.applyImpairmentFn()
			defer tc.removeImpairmentFn()

			t.Run("Validate Backup Path Traffic Delivery", func(t *testing.T) {
				tcArgs.validateTrafficFlows(t, backupFlow, baselineFlow)
			})
		})
	}

	tcArgs.configureBackupNextHopGroup(t, true)
}

// TestIndirectBackupNexthopGroup validates the backup next hop group is utilized
// during the failure of a single next hop that is resolved recursively.
//
// Setup Steps
//   * Connect DUT port 1 <> ATE port 1.
//   * Connect DUT port 2 <> ATE port 2.
//   * Connect DUT port 3 <> ATE port 3.
//   * Create prefix 198.51.100.0/24, next hop group, and next hop to forward to
//     192.0.2.254.  Create backup next hop group to forward to DUT port 3.
//   * Create prefix 192.0.2.254/32, next hop group, and next hop to forward to
//     DUT port 2.
//
// Validation Steps
//   * Verify traffic flows to ATE port 2 and not ATE port 3.
//   * Delete prefix 192.0.2.254/32.
//   * Verify traffic flows to ATE port 3 and not ATE port 2.
func TestIndirectBackupNexthopGroup(t *testing.T) {
	ctx := context.Background()

	dut := ondatra.DUT(t, "dut")
	configureDUT(t, dut)

	ate := ondatra.ATE(t, "ate")
	ateTop := configureATE(t, ate)

	gribic := dut.RawAPIs().GRIBI().Default(t)
	c := fluent.NewClient()
	c.Connection().WithStub(gribic).
		WithPersistence().
		WithRedundancyMode(fluent.ElectedPrimaryClient).
		WithInitialElectionID(1 /* low */, 0 /* hi */) // ID must be > 0.
	c.Start(ctx, t)
	defer c.Stop(t)
	c.StartSending(ctx, t)
	if err := awaitTimeout(ctx, c, t, time.Minute); err != nil {
		t.Fatalf("Await got error during session negotiation: %v", err)
	}

	tcArgs := &testArgs{
		ate:    ate,
		ateTop: ateTop,
		ctx:    ctx,
		dut:    dut,
		gribic: c,
	}

	tcArgs.configureIndirBackupNextHopGroup(t, false)

	baselineFlow := tcArgs.createFlow("Baseline Path Flow", &atePort2)
	backupFlow := tcArgs.createFlow("Backup Path Flow", &atePort3)

	t.Run("Validate Baseline Traffic Delivery", func(t *testing.T) {
		tcArgs.validateTrafficFlows(t, baselineFlow, backupFlow)
	})

	// Delete indirect(recursive) next hop prefix entry to activate the backup
	// next hop path.
	c.Modify().DeleteEntry(t, fluent.IPv4Entry().WithNetworkInstance(defaultNetworkInstance).
		WithPrefix("192.0.2.254/32").WithNextHopGroup(10000))

	t.Run("Validate Backup Path Traffic Delivery", func(t *testing.T) {
		tcArgs.validateTrafficFlows(t, backupFlow, baselineFlow)
	})

	tcArgs.configureIndirBackupNextHopGroup(t, true)
}

// awaitTimeout calls a fluent client Await, adding a timeout to the context.
func awaitTimeout(ctx context.Context, c *fluent.GRIBIClient, t testing.TB, timeout time.Duration) error {
	subctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return c.Await(subctx, t)
}

// configreATE configures port1-3 on the ATE.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) *ondatra.ATETopology {
	top := ate.Topology().New()

	p1 := ate.Port(t, "port1")
	p2 := ate.Port(t, "port2")
	p3 := ate.Port(t, "port3")

	atePort1.AddToATE(top, p1, &dutPort1)
	atePort2.AddToATE(top, p2, &dutPort2)
	atePort3.AddToATE(top, p3, &dutPort3)

	top.Push(t).StartProtocols(t)

	return top
}

// configureDUT configures port1-3 on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	d := dut.Config()

	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")
	p3 := dut.Port(t, "port3")

	d.Interface(p1.Name()).Replace(t, dutPort1.NewInterface(p1.Name()))
	d.Interface(p2.Name()).Replace(t, dutPort2.NewInterface(p2.Name()))
	d.Interface(p3.Name()).Replace(t, dutPort3.NewInterface(p3.Name()))
}

// configureBackupNextHopGroup creates and deletes the gribi nexthops, nexthop
// groups, and prefixes for evaluating a single backup next hop forwarding
// entry.
func (a *testArgs) configureBackupNextHopGroup(t *testing.T, del bool) {
	const (
		// Next hop group ID that the dstPfx will forward to.
		dstNHGID = 101
		// Backup next hop group ID that the dstPfx will forward to.
		dstBackupNHGID = 102

		dutPort2ID, dutPort3ID = 10002, 10003
	)

	nh1 := fluent.NextHopEntry().WithNetworkInstance(defaultNetworkInstance).
		WithIndex(dutPort2ID).WithIPAddress(atePort2.IPv4)
	nh2 := fluent.NextHopEntry().WithNetworkInstance(defaultNetworkInstance).
		WithIndex(dutPort3ID).WithIPAddress(atePort3.IPv4)

	nhg := fluent.NextHopGroupEntry().WithNetworkInstance(defaultNetworkInstance).
		WithID(dstNHGID).AddNextHop(dutPort2ID, 1).WithBackupNHG(dstBackupNHGID)
	bnhg := fluent.NextHopGroupEntry().WithNetworkInstance(defaultNetworkInstance).
		WithID(dstBackupNHGID).AddNextHop(dutPort3ID, 1)

	pfx := fluent.IPv4Entry().WithNetworkInstance(defaultNetworkInstance).
		WithPrefix(dstPfx).WithNextHopGroup(dstNHGID)

	if del {
		a.gribic.Modify().DeleteEntry(t, pfx, nhg, bnhg, nh2, nh1)
	} else {
		a.gribic.Modify().AddEntry(t, nh1, nh2, bnhg, nhg, pfx)
	}
	if err := awaitTimeout(a.ctx, a.gribic, t, time.Minute); err != nil {
		t.Fatalf("Await got error for entries: %v", err)
	}
}

// configureIndirBackupNextHopGroup creates and deletes the gribi nexthops,
// nexthop groups, and prefixes for evaluating an indirect next hop forwarding
// entry.
func (a *testArgs) configureIndirBackupNextHopGroup(t *testing.T, del bool) {
	const (
		// Prefix for recursive nexthop resolution.
		recurPfx = "192.0.2.254/32"
		// Next hop adjacency identifier that the recursive next hop group will point to.
		recurNHID = 10000
		// Recursive next hop address for dstPfx to forward to.
		recurNH = "192.0.2.254"
		// Next hop group adjacency identifier that the recurPfx will forward to.
		recurNHGID = 103
		// Next hop group ID that the dstPfx will forward to.
		dstNHGID = 101
		// Backup next hop group ID that the dstPfx will forward to.
		dstBackupNHGID = 102

		dutPort2ID, dutPort3ID = 10002, 10003
	)

	rnh := fluent.NextHopEntry().WithNetworkInstance(defaultNetworkInstance).
		WithIndex(recurNHID).WithIPAddress(recurNH)
	nhg := fluent.NextHopGroupEntry().WithNetworkInstance(defaultNetworkInstance).
		WithID(dstNHGID).AddNextHop(recurNHID, 1).WithBackupNHG(dstBackupNHGID)
	pfx := fluent.IPv4Entry().WithNetworkInstance(defaultNetworkInstance).
		WithPrefix(dstPfx).WithNextHopGroup(dstNHGID)

	nh1 := fluent.NextHopEntry().WithNetworkInstance(defaultNetworkInstance).
		WithIndex(dutPort2ID).WithIPAddress(atePort2.IPv4)
	rnhg := fluent.NextHopGroupEntry().WithNetworkInstance(defaultNetworkInstance).
		WithID(recurNHGID).AddNextHop(dutPort2ID, 1)
	rpfx := fluent.IPv4Entry().WithNetworkInstance(defaultNetworkInstance).
		WithPrefix(recurPfx).WithNextHopGroup(recurNHGID)

	nh2 := fluent.NextHopEntry().WithNetworkInstance(defaultNetworkInstance).
		WithIndex(dutPort3ID).WithIPAddress(atePort3.IPv4)
	bnhg := fluent.NextHopGroupEntry().WithNetworkInstance(defaultNetworkInstance).
		WithID(dstBackupNHGID).AddNextHop(dutPort3ID, 1)

	if del {
		a.gribic.Modify().DeleteEntry(t, pfx, nhg, bnhg, rnhg, rnh, nh2, nh1)
	} else {
		a.gribic.Modify().AddEntry(t, nh1, nh2, rnh, rnhg, rpfx, bnhg, nhg, pfx)
	}
	if err := awaitTimeout(a.ctx, a.gribic, t, time.Minute); err != nil {
		t.Fatalf("Await got error for entries: %v", err)
	}
}

// createFlow returns a flow from atePort1 to the dstPfx, expected to arrive on ATE interface dst.
func (a *testArgs) createFlow(name string, dst *attrs.Attributes) *ondatra.Flow {
	hdr := ondatra.NewIPv4Header()
	hdr.WithSrcAddress(dutPort1.IPv4).DstAddressRange().WithMin(dstPfxMin).WithMax(dstPfxMax).WithCount(dstPfxCount)

	flow := a.ate.Traffic().NewFlow(name).
		WithSrcEndpoints(a.ateTop.Interfaces()[atePort1.Name]).
		WithDstEndpoints(a.ateTop.Interfaces()[dst.Name]).
		WithHeaders(ondatra.NewEthernetHeader(), hdr)

	return flow
}

func (a *testArgs) validateAftTelemetry(t *testing.T) {
	aftPfxNHG := a.dut.Telemetry().NetworkInstance(defaultNetworkInstance).Afts().Ipv4Entry(dstPfx).NextHopGroup()
	aftPfxNHGVal, found := aftPfxNHG.Watch(t, 10*time.Second, func(val *telemetry.QualifiedUint64) bool {
		// Do nothing in this matching function, as we already filter on the prefix.
		return true
	}).Await(t)
	if !found {
		t.Fatalf("Could not find prefix %s in telemetry AFT", dstPfx)
	}

	aftNHG := a.dut.Telemetry().NetworkInstance(defaultNetworkInstance).Afts().NextHopGroup(aftPfxNHGVal.Val(t)).Get(t)
	if got := len(aftNHG.NextHop); got != 1 {
		t.Fatalf("Prefix %s next-hop entry count: got %d, want 1", dstPfx, got)
	}

	for k := range aftNHG.NextHop {
		aftnh := a.dut.Telemetry().NetworkInstance(defaultNetworkInstance).Afts().NextHop(k).Get(t)
		if got, want := aftnh.GetIpAddress(), atePort2.IPv4; got != want {
			t.Fatalf("Prefix %s next-hop IP: got %s, want %s", dstPfx, got, want)
		}
	}
}

// validateTrafficFlows verifies that the good flow delivers traffic and the
// bad flow does not deliver traffic.
func (a *testArgs) validateTrafficFlows(t *testing.T, good *ondatra.Flow, bad *ondatra.Flow) {
	a.ate.Traffic().Start(t, good, bad)
	time.Sleep(15 * time.Second)
	a.ate.Traffic().Stop(t)

	if got := a.ate.Telemetry().Flow(good.Name()).LossPct().Get(t); got > 0 {
		t.Fatalf("LossPct for flow %s: got %g, want 0", good.Name(), got)
	}
	if got := a.ate.Telemetry().Flow(bad.Name()).LossPct().Get(t); got < 100 {
		t.Fatalf("LossPct for flow %s: got %g, want 100", bad.Name(), got)
	}
}
