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

// Ensure backup next hop groups are honored for next hop groups containing a single next hop.
package backup_nhg_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ygot/ygot"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
)

type testArgs struct {
	ate    *ondatra.ATEDevice
	ateTop *ondatra.ATETopology
	ctx    context.Context
	dut    *ondatra.DUTDevice
	gribic *fluent.GRIBIClient
}

const (
	// Destination prefix for DUT to ATE traffic.
	dstPfx      = "198.51.100.0/24"
	dstPfxMin   = "198.51.100.0"
	dstPfxMax   = "198.51.100.255"
	dstPfxCount = 256

	vrfName = "VRF-1"
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
				gnmi.Await(t, dut, gnmi.OC().Interface(dutP2.Name()).OperStatus().State(), time.Minute, oc.Interface_OperStatus_DOWN)
			},
			removeImpairmentFn: func() {
				ateP2 := ate.Port(t, "port2")
				dutP2 := dut.Port(t, "port2")
				ate.Actions().NewSetPortState().WithPort(ateP2).WithEnabled(true).Send(t)
				gnmi.Await(t, dut, gnmi.OC().Interface(dutP2.Name()).OperStatus().State(), time.Minute, oc.Interface_OperStatus_UP)
			},
		},
		{
			desc: "Disable DUT port-2",
			applyImpairmentFn: func() {
				dutP2 := dut.Port(t, "port2")
				gnmi.Replace(t, dut, gnmi.OC().Interface(dutP2.Name()).Enabled().Config(), false)
				gnmi.Await(t, dut, gnmi.OC().Interface(dutP2.Name()).OperStatus().State(), time.Minute, oc.Interface_OperStatus_DOWN)
			},
			removeImpairmentFn: func() {
				dutP2 := dut.Port(t, "port2")
				gnmi.Replace(t, dut, gnmi.OC().Interface(dutP2.Name()).Enabled().Config(), true)
				gnmi.Await(t, dut, gnmi.OC().Interface(dutP2.Name()).OperStatus().State(), time.Minute, oc.Interface_OperStatus_UP)
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			t.Run("Validate Baseline AFT Telemetry", func(t *testing.T) {
				var err error
				// AFT telemetry may take time to correctly reflect FIB.
				for i := 0; i < 10; i++ {
					err = tcArgs.validateAftTelemetry(t)
					if err == nil {
						break
					}
					time.Sleep(2 * time.Second)
				}
				if err != nil {
					t.Fatal(err)
				}
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
	gribi.BecomeLeader(t, c)

	// Flush all entries before test.
	if err := gribi.FlushAll(c); err != nil {
		t.Errorf("Cannot flush: %v", err)
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
	c.Modify().DeleteEntry(t, fluent.IPv4Entry().WithNetworkInstance(*deviations.DefaultNetworkInstance).
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
	d := gnmi.OC()

	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")
	p3 := dut.Port(t, "port3")

	p1c := dutPort1.NewOCInterface(p1.Name())
	p1c.GetSubinterface(0).Ipv4 = nil
	gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), p1c)
	gnmi.Replace(t, dut, d.Interface(p2.Name()).Config(), dutPort2.NewOCInterface(p2.Name()))
	gnmi.Replace(t, dut, d.Interface(p3.Name()).Config(), dutPort3.NewOCInterface(p3.Name()))

	if *deviations.ExplicitPortSpeed {
		fptest.SetPortSpeed(t, p1)
		fptest.SetPortSpeed(t, p2)
		fptest.SetPortSpeed(t, p3)
	}
	if *deviations.ExplicitInterfaceInDefaultVRF {
		fptest.AssignToNetworkInstance(t, dut, p1.Name(), *deviations.DefaultNetworkInstance, 0)
		fptest.AssignToNetworkInstance(t, dut, p2.Name(), *deviations.DefaultNetworkInstance, 0)
		fptest.AssignToNetworkInstance(t, dut, p3.Name(), *deviations.DefaultNetworkInstance, 0)
	}

	vrf := &oc.NetworkInstance{
		Name: ygot.String(vrfName),
		Type: oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF,
	}
	if !*deviations.VRFSelectionPolicyRequired {
		vrfIntf := vrf.GetOrCreateInterface(p1.Name())
		vrfIntf.Interface = ygot.String(p1.Name())
		vrfIntf.Subinterface = ygot.Uint32(0)
	}
	gnmi.Replace(t, dut, d.NetworkInstance(vrfName).Config(), vrf)
	p1c = dutPort1.NewOCInterface(p1.Name())
	gnmi.Replace(t, dut, d.Interface(p1.Name()).Subinterface(0).Ipv4().Config(), p1c.GetSubinterface(0).GetIpv4())

	if *deviations.VRFSelectionPolicyRequired {
		policy := &oc.NetworkInstance_PolicyForwarding_Policy{
			PolicyId: ygot.String("VRF-SELECTION-POLICY"),
			Type:     oc.Policy_Type_VRF_SELECTION_POLICY,
		}
		rule := policy.GetOrCreateRule(1)
		rule.GetOrCreateIpv4().SetProtocol(oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP)
		rule.GetOrCreateAction().SetNetworkInstance(vrfName)
		gnmi.Replace(t, dut, d.NetworkInstance(*deviations.DefaultNetworkInstance).PolicyForwarding().Policy("VRF-SELECTION-POLICY").Config(), policy)

		pfi := &oc.NetworkInstance_PolicyForwarding_Interface{
			InterfaceId:             ygot.String(p1.Name()),
			ApplyVrfSelectionPolicy: ygot.String("VRF-SELECTION-POLICY"),
		}
		gnmi.Replace(t, dut, d.NetworkInstance(*deviations.DefaultNetworkInstance).PolicyForwarding().Interface(p1.Name()).Config(), pfi)
	}
}

// configureBackupNextHopGroup creates and deletes the gribi nexthops, nexthop
// groups, and prefixes for evaluating a single backup next hop forwarding
// entry.
func (a *testArgs) configureBackupNextHopGroup(t *testing.T, del bool) {
	const (
		// Next hop group ID that the dstPfx will forward to.
		dstNHGID = 101
		// Backup next hop ID
		dstBackupNHID = 104
		// Backup next hop group ID that the dstPfx will forward to.
		dstBackupNHGID = 102
		// Destination prefix next hop group ID
		dNHGID = 105

		dutPort2ID, dutPort3ID = 10002, 10003
	)

	nh1 := fluent.NextHopEntry().WithNetworkInstance(*deviations.DefaultNetworkInstance).
		WithIndex(dutPort2ID).WithIPAddress(atePort2.IPv4).WithInterfaceRef(a.dut.Port(t, "port2").Name())
	nh2 := fluent.NextHopEntry().WithNetworkInstance(*deviations.DefaultNetworkInstance).
		WithIndex(dutPort3ID).WithIPAddress(atePort3.IPv4).WithInterfaceRef(a.dut.Port(t, "port3").Name())
	bnh := fluent.NextHopEntry().WithNetworkInstance(*deviations.DefaultNetworkInstance).
		WithIndex(dstBackupNHID).WithNextHopNetworkInstance(*deviations.DefaultNetworkInstance)

	nhg := fluent.NextHopGroupEntry().WithNetworkInstance(*deviations.DefaultNetworkInstance).
		WithID(dNHGID).AddNextHop(dutPort3ID, 1)
	bnhg := fluent.NextHopGroupEntry().WithNetworkInstance(*deviations.DefaultNetworkInstance).
		WithID(dstBackupNHGID).AddNextHop(dstBackupNHID, 1)
	vrfNHG := fluent.NextHopGroupEntry().WithNetworkInstance(*deviations.DefaultNetworkInstance).
		WithID(dstNHGID).AddNextHop(dutPort2ID, 1).WithBackupNHG(dstBackupNHGID)

	vrfPfx := fluent.IPv4Entry().WithNetworkInstance(vrfName).WithPrefix(dstPfx).
		WithNextHopGroup(dstNHGID).WithNextHopGroupNetworkInstance(*deviations.DefaultNetworkInstance)
	pfx := fluent.IPv4Entry().WithNetworkInstance(*deviations.DefaultNetworkInstance).WithPrefix(dstPfx).
		WithNextHopGroup(dNHGID).WithNextHopGroupNetworkInstance(*deviations.DefaultNetworkInstance)

	if del {
		a.gribic.Modify().DeleteEntry(t, pfx, vrfPfx, vrfNHG, bnhg, nhg, bnh, nh2, nh1)
	} else {
		a.gribic.Modify().AddEntry(t, nh1, nh2, bnh, nhg, bnhg, vrfNHG, vrfPfx, pfx)
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
		// Backup next hop ID
		dstBackupNHID = 104
		// Backup next hop group ID that the dstPfx will forward to.
		dstBackupNHGID = 102
		// Destination prefix next hop group ID in default VRF
		dNHGID = 105

		dutPort2ID, dutPort3ID = 10002, 10003
	)

	dnh := fluent.NextHopEntry().WithNetworkInstance(*deviations.DefaultNetworkInstance).
		WithIndex(recurNHID).WithIPAddress(recurNH)
	dnhg := fluent.NextHopGroupEntry().WithNetworkInstance(*deviations.DefaultNetworkInstance).
		WithID(dstNHGID).AddNextHop(recurNHID, 1).WithBackupNHG(dstBackupNHGID)
	dpfx := fluent.IPv4Entry().WithNetworkInstance(vrfName).WithPrefix(dstPfx).
		WithNextHopGroup(dstNHGID).WithNextHopGroupNetworkInstance(*deviations.DefaultNetworkInstance)

	bnh := fluent.NextHopEntry().WithNetworkInstance(*deviations.DefaultNetworkInstance).
		WithIndex(dstBackupNHID).WithNextHopNetworkInstance(*deviations.DefaultNetworkInstance)
	bnhg := fluent.NextHopGroupEntry().WithNetworkInstance(*deviations.DefaultNetworkInstance).
		WithID(dstBackupNHGID).AddNextHop(dstBackupNHID, 1)

	nh1 := fluent.NextHopEntry().WithNetworkInstance(*deviations.DefaultNetworkInstance).
		WithIndex(dutPort2ID).WithIPAddress(atePort2.IPv4).WithInterfaceRef(a.dut.Port(t, "port2").Name())
	rnhg := fluent.NextHopGroupEntry().WithNetworkInstance(*deviations.DefaultNetworkInstance).
		WithID(recurNHGID).AddNextHop(dutPort2ID, 1)
	rpfx := fluent.IPv4Entry().WithNetworkInstance(*deviations.DefaultNetworkInstance).WithPrefix(recurPfx).
		WithNextHopGroup(recurNHGID)

	nh2 := fluent.NextHopEntry().WithNetworkInstance(*deviations.DefaultNetworkInstance).
		WithIndex(dutPort3ID).WithIPAddress(atePort3.IPv4).WithInterfaceRef(a.dut.Port(t, "port3").Name())
	nhg := fluent.NextHopGroupEntry().WithNetworkInstance(*deviations.DefaultNetworkInstance).
		WithID(dNHGID).AddNextHop(dutPort3ID, 1)
	pfx := fluent.IPv4Entry().WithNetworkInstance(*deviations.DefaultNetworkInstance).
		WithPrefix(dstPfx).WithNextHopGroup(dNHGID)

	if del {
		a.gribic.Modify().DeleteEntry(t, dpfx, rpfx, pfx, dnhg, bnhg, rnhg, nhg, nh2, nh1, bnh, dnh)
	} else {
		a.gribic.Modify().AddEntry(t, dnh, bnh, nh1, nh2, nhg, rnhg, bnhg, dnhg, pfx, rpfx, dpfx)
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
		WithHeaders(ondatra.NewEthernetHeader(), hdr, hdr, ondatra.NewTCPHeader())

	return flow
}

func (a *testArgs) validateAftTelemetry(t *testing.T) error {
	aftPfxNHG := gnmi.OC().NetworkInstance(vrfName).Afts().Ipv4Entry(dstPfx).NextHopGroup()
	aftPfxNHGVal, found := gnmi.Watch(t, a.dut, aftPfxNHG.State(), 10*time.Second, func(val *ygnmi.Value[uint64]) bool {
		return true
	}).Await(t)
	if !found {
		return fmt.Errorf("Could not find prefix %s in telemetry AFT", dstPfx)
	}
	nhg, _ := aftPfxNHGVal.Val()

	aftNHG := gnmi.Get(t, a.dut, gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).Afts().NextHopGroup(nhg).State())
	if got := len(aftNHG.NextHop); got != 1 {
		return fmt.Errorf("Prefix %s next-hop entry count: got %d, want 1", dstPfx, got)
	}

	for k := range aftNHG.NextHop {
		aftnh := gnmi.Get(t, a.dut, gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).Afts().NextHop(k).State())
		if got, want := aftnh.GetIpAddress(), atePort2.IPv4; got != want {
			return fmt.Errorf("Prefix %s next-hop IP: got %s, want %s", dstPfx, got, want)
		}
	}

	return nil
}

// validateTrafficFlows verifies that the good flow delivers traffic and the
// bad flow does not deliver traffic.
func (a *testArgs) validateTrafficFlows(t *testing.T, good *ondatra.Flow, bad *ondatra.Flow) {
	a.ate.Traffic().Start(t, good, bad)
	time.Sleep(15 * time.Second)
	a.ate.Traffic().Stop(t)

	if got := gnmi.Get(t, a.ate, gnmi.OC().Flow(good.Name()).LossPct().State()); got > 0 {
		t.Fatalf("LossPct for flow %s: got %g, want 0", good.Name(), got)
	}
	if got := gnmi.Get(t, a.ate, gnmi.OC().Flow(bad.Name()).LossPct().State()); got < 100 {
		t.Fatalf("LossPct for flow %s: got %g, want 100", bad.Name(), got)
	}
}
