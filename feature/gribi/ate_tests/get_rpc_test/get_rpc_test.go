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

package get_rpc_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	spb "github.com/openconfig/gribi/v1/proto/service"
	"github.com/openconfig/gribigo/chk"
	"github.com/openconfig/gribigo/constants"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Settings for configuring the baseline testbed with the test
// topology.
//
// The testbed consists of ate:port1 -> dut:port1,
// dut:port2 -> ate:port2
//
//   * ate:port1 -> dut:port1 subnet 192.0.2.0/30
//   * ate:port2 -> dut:port2 subnet 192.0.2.4/30
//
//   * Destination network: {"198.51.100.0/26", "198.51.100.64/26", "198.51.100.128/26"}

const (
	ipv4PrefixLen = 30
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
		Name:    "atePort1",
		IPv4:    "192.0.2.2",
		IPv4Len: ipv4PrefixLen,
	}

	dutPort2 = attrs.Attributes{
		Desc:    "dutPort2",
		IPv4:    "192.0.2.5",
		IPv4Len: ipv4PrefixLen,
	}

	atePort2 = attrs.Attributes{
		Name:    "atePort2",
		IPv4:    "192.0.2.6",
		IPv4Len: ipv4PrefixLen,
	}
)

var (
	ateDstNetCIDR = []string{"198.51.100.0/26", "198.51.100.64/26", "198.51.100.128/26"}
)

const (
	staticCIDR        = "198.51.100.192/26"
	ipv4Prefix        = "203.0.113.0/24"
	unresolvedNextHop = "192.0.2.254/32"
)

// configInterfaceDUT configures the interface with the Addrs.
func configInterfaceDUT(i *oc.Interface, a *attrs.Attributes) *oc.Interface {
	i.Description = ygot.String(a.Desc)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
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

// configureDUT configures port1, port2 on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	d := gnmi.OC()

	p1 := dut.Port(t, "port1")
	i1 := &oc.Interface{Name: ygot.String(p1.Name())}
	gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), configInterfaceDUT(i1, &dutPort1))

	p2 := dut.Port(t, "port2")
	i2 := &oc.Interface{Name: ygot.String(p2.Name())}
	gnmi.Replace(t, dut, d.Interface(p2.Name()).Config(), configInterfaceDUT(i2, &dutPort2))

}

// configureATE configures port1, port2 on the ATE.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) *ondatra.ATETopology {
	top := ate.Topology().New()

	p1 := ate.Port(t, "port1")
	i1 := top.AddInterface(atePort1.Name).WithPort(p1)
	i1.IPv4().
		WithAddress(atePort1.IPv4CIDR()).
		WithDefaultGateway(dutPort1.IPv4)

	p2 := ate.Port(t, "port2")
	i2 := top.AddInterface(atePort2.Name).WithPort(p2)
	i2.IPv4().
		WithAddress(atePort2.IPv4CIDR()).
		WithDefaultGateway(dutPort2.IPv4)

	return top
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
	top     *ondatra.ATETopology
}

// helperAddEntry configures a sequence of adding the NH, NHG and IPv4Entry by a client.
func helperAddEntry(ctx context.Context, t *testing.T, client *fluent.GRIBIClient, nextHop string, ipPrefix string) {
	t.Helper()
	client.Modify().AddEntry(t,
		fluent.NextHopEntry().
			WithNetworkInstance(*deviations.DefaultNetworkInstance).
			WithIndex(nhIndex).
			WithIPAddress(nextHop),
		fluent.NextHopGroupEntry().
			WithNetworkInstance(*deviations.DefaultNetworkInstance).
			WithID(nhgIndex).
			AddNextHop(nhIndex, 1),
		fluent.IPv4Entry().
			WithNetworkInstance(*deviations.DefaultNetworkInstance).
			WithPrefix(ipPrefix).
			WithNextHopGroup(nhgIndex),
	)

	if err := awaitTimeout(ctx, client, t, time.Minute); err != nil {
		t.Fatalf("Could not program entries via %v, got err: %v", client, err)
	}

}

// configureIPv4ViaClientB configures a IPv4 Entry via ClientB with an Election
// ID of 11 when ClientA is already connected with Election ID of 11. Ensure
// that the entry via ClientB is active through AFT Telemetry.
func configureIPv4ViaClientB(t *testing.T, args *testArgs) {
	for _, cidr := range ateDstNetCIDR {
		t.Logf("Adding an IPv4Entry for %s pointing to ATE port-2 via clientB.", cidr)
		helperAddEntry(args.ctx, t, args.clientB, atePort2.IPv4, cidr)

		// Verify the entry is not installed due to client B having lower election ID.
		chk.HasResult(t, args.clientB.Results(t),
			fluent.OperationResult().
				WithIPv4Operation(cidr).
				WithOperationType(constants.Add).
				WithProgrammingResult(fluent.ProgrammingFailed).
				AsResult(),
			chk.IgnoreOperationID(),
		)
	}
}

// configureIPv4ViaClientAInstalled configures a IPv4 Entry via ClientA with an
// Election ID of 12. Ensure that the entry via ClientA is installed.
func configureIPv4ViaClientAInstalled(t *testing.T, args *testArgs) {
	t.Logf("Adding an IPv4Entry for %s pointing to ATE port-2 via clientA as leader.", ateDstNetCIDR)
	gribi.BecomeLeader(t, args.clientA)
	// TODO (deepgajjar): Remove WithElectionID and reuse helperAddEntry
	// once gribi/gribigo in google3 is updated.
	args.clientA.Modify().AddEntry(t,
		fluent.NextHopEntry().
			WithNetworkInstance(*deviations.DefaultNetworkInstance).
			WithIndex(nhIndex).
			WithIPAddress(atePort2.IPv4))

	args.clientA.Modify().AddEntry(t,
		fluent.NextHopGroupEntry().
			WithNetworkInstance(*deviations.DefaultNetworkInstance).
			WithID(nhgIndex).
			AddNextHop(nhIndex, 1))

	for ip := range ateDstNetCIDR {
		args.clientA.Modify().AddEntry(t,
			fluent.IPv4Entry().
				WithPrefix(ateDstNetCIDR[ip]).
				WithNetworkInstance(*deviations.DefaultNetworkInstance).
				WithNextHopGroup(nhgIndex))
	}

	if err := awaitTimeout(args.ctx, args.clientA, t, time.Minute); err != nil {
		t.Fatalf("Could not program entries via clientA, got err: %v", err)
	}

	chk.HasResult(t, args.clientA.Results(t),
		fluent.OperationResult().
			WithNextHopOperation(nhIndex).
			WithOperationType(constants.Add).
			WithProgrammingResult(fluent.InstalledInFIB).
			AsResult(),
		chk.IgnoreOperationID(),
	)

	chk.HasResult(t, args.clientA.Results(t),
		fluent.OperationResult().
			WithNextHopGroupOperation(nhgIndex).
			WithOperationType(constants.Add).
			WithProgrammingResult(fluent.InstalledInFIB).
			AsResult(),
		chk.IgnoreOperationID(),
	)

	for ip := range ateDstNetCIDR {
		chk.HasResult(t, args.clientA.Results(t),
			fluent.OperationResult().
				WithIPv4Operation(ateDstNetCIDR[ip]).
				WithOperationType(constants.Add).
				WithProgrammingResult(fluent.InstalledInFIB).
				AsResult(),
			chk.IgnoreOperationID(),
		)
	}
}

// validateGetRPC issues GET RPC from clientA and clientB and ensure
// that all entries are returned.
func validateGetRPC(ctx context.Context, t *testing.T, client *fluent.GRIBIClient) {
	t.Helper()
	getResponse, err := client.Get().AllNetworkInstances().WithAFT(fluent.IPv4).Send()
	var prefixes []string
	if err != nil {
		t.Errorf("Cannot Get: %v", err)
	}
	entries := getResponse.GetEntry()
	for _, entry := range entries {
		v := entry.Entry.(*spb.AFTEntry_Ipv4)
		if prefix := v.Ipv4.GetPrefix(); prefix != "" {
			prefixes = append(prefixes, prefix)
		}
	}
	less := func(a, b string) bool { return a < b }
	if diff := cmp.Diff(ateDstNetCIDR, prefixes, cmpopts.SortSlices(less)); diff != "" {
		t.Errorf("Prefixes differed (-want +got):\n%v", diff)
	}
}

// testIPv4LeaderActive specifies gRIBI-A as leader with higher,
// election ID and configures  IPv4 entries through this client.
// Ensures that the enties via ClientA is active through AFT
// Telemetry and getRPC.
func testIPv4LeaderActive(ctx context.Context, t *testing.T, args *testArgs) {
	// Inject IPv4Entry cases for 198.51.100.0, 198.51.100.64, 198.51.100.128
	// to ATE port-2 via gRIBI-A.
	configureIPv4ViaClientAInstalled(t, args)

	// Verify the above entries are active through AFT Telemetry.
	for ip := range ateDstNetCIDR {
		ipv4Path := gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).Afts().Ipv4Entry(ateDstNetCIDR[ip])
		if got, want := gnmi.Get(t, args.dut, ipv4Path.Prefix().State()), ateDstNetCIDR[ip]; got != want {
			t.Errorf("ipv4-entry/state/prefix got %s, want %s", got, want)
		}
	}

	// Configure IPv4 routes pointing to ATE port-2 via clientB.
	// The entry should not be installed due to client B having lower election ID.
	configureIPv4ViaClientB(t, args)

	validateGetRPC(ctx, t, args.clientA)
	validateGetRPC(ctx, t, args.clientB)

	// Configure static route for 198.51.100.192/64, issue Get from gRIBI-A
	// and ensure that only entries for 198.51.100.0/26, 198.51.100.64/26, 198.51.100.128/26
	// are returned, with no entry returned for 198.51.100.192/64.
	dc := gnmi.OC()
	ni := dc.NetworkInstance(*deviations.DefaultNetworkInstance).
		Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, *deviations.StaticProtocolName)
	static := &oc.NetworkInstance_Protocol_Static{
		Prefix: ygot.String(staticCIDR),
	}
	static.GetOrCreateNextHop("0").NextHop = oc.UnionString(atePort2.IPv4)
	gnmi.Replace(t, args.dut, ni.Static(staticCIDR).Config(), static)
	validateGetRPC(ctx, t, args.clientA)
	for ip := range ateDstNetCIDR {
		ipv4Path := gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).Afts().Ipv4Entry(ateDstNetCIDR[ip])
		if got, want := gnmi.Get(t, args.dut, ipv4Path.Prefix().State()), ateDstNetCIDR[ip]; got != want {
			t.Errorf("ipv4-entry/state/prefix got %s, want %s", got, want)
		}
	}

	// Inject an entry that cannot be installed into the FIB due to an unresolved next-hop
	// (203.0.113.0/24 -> unresolved 192.0.2.254/32).  Issue a Get RPC from gRIBI-A and ensure
	// that the entry for 203.0.113.0/24 is not returned.
	args.clientA.Modify().AddEntry(t,
		fluent.NextHopEntry().
			WithNetworkInstance(*deviations.DefaultNetworkInstance).
			WithIndex(1000+nhIndex).
			WithIPAddress(unresolvedNextHop),
		fluent.NextHopGroupEntry().
			WithNetworkInstance(*deviations.DefaultNetworkInstance).
			WithID(1000+nhgIndex).
			AddNextHop(1000+nhIndex, 1),
		fluent.IPv4Entry().
			WithNetworkInstance(*deviations.DefaultNetworkInstance).
			WithPrefix(ipv4Prefix).
			WithNextHopGroup(1000+nhgIndex),
	)

	if err := awaitTimeout(args.ctx, args.clientA, t, time.Minute); err != nil {
		t.Fatalf("Could not program entries via clientA, got err: %v", err)
	}

	validateGetRPC(ctx, t, args.clientA)
}

func TestElectionID(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	// Dial gRIBI
	ctx := context.Background()
	gribic := dut.RawAPIs().GRIBI().Default(t)

	// Configure the DUT
	configureDUT(t, dut)

	// Configure the ATE
	ate := ondatra.ATE(t, "ate")
	top := configureATE(t, ate)
	top.Push(t).StartProtocols(t)

	// Connect gRIBI client to DUT referred to as gRIBI-A - using PRESERVE persistence and
	// SINGLE_PRIMARY mode, with FIB ACK requested. Specify gRIBI-A as the leader.
	clientA := fluent.NewClient()
	clientA.Connection().WithStub(gribic).WithPersistence().WithInitialElectionID(12, 0).
		WithRedundancyMode(fluent.ElectedPrimaryClient).WithFIBACK()

	clientA.Start(ctx, t)
	defer clientA.Stop(t)

	defer func() {
		// Flush all entries after test.
		if err := gribi.FlushAll(clientA); err != nil {
			t.Error(err)
		}
	}()

	clientA.StartSending(ctx, t)
	if err := awaitTimeout(ctx, clientA, t, time.Minute); err != nil {
		t.Fatalf("Await got error during session negotiation for clientA: %v", err)
	}

	// Connect gRIBI client to DUT referred to as gRIBI-B - using PRESERVE persistence and
	// SINGLE_PRIMARY mode, with FIB ACK requested and election ID of 11, which is not the
	// leader.
	clientB := fluent.NewClient()
	clientB.Connection().WithStub(gribic).WithPersistence().WithInitialElectionID(11, 0).
		WithRedundancyMode(fluent.ElectedPrimaryClient).WithFIBACK()

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
	testIPv4LeaderActive(ctx, t, args)
}
