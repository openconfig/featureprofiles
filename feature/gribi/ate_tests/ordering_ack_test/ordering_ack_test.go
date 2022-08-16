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

package ordering_ack_test

import (
	"context"
	"flag"
	"testing"
	"time"

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

var (
	// TE-3.5 specific deviation flags that are currently set to reduced compliance checking
	// in order to establish a baseline to highlight the non-compliant behavior.  They
	// should be set to the more strict setting.
	fibAck         = flag.Bool("fib_ack", true, "Request FIB_PROGRAMMED from the device.")
	flush          = flag.Bool("flush", false, "Flush gRIBI entries before setup.  Should normally be false unless the device erroneously persists entries after the client disconnected.")
	checkTelemetry = flag.Bool("telemetry", false /* TODO: set to true */, "Check AFT telemetry.")

	wantInstalled = fluent.InstalledInRIB
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Settings for configuring the baseline testbed with the test
// topology.  IxNetwork flow requires both source and destination
// networks be configured on the ATE.  It is not possible to send
// packets to the ether.
//
// The testbed consists of ate:port1 -> dut:port1 and
// dut:port2 -> ate:port2.  The first pair is called the "source"
// pair, and the second the "destination" pair.
//
//   - Source: ate:port1 -> dut:port1 subnet 192.0.2.0/30
//   - Destination: dut:port2 -> ate:port2 subnet 192.0.2.4/30
//
// A traffic flow from a source network is configured to be sent from
// ate:port1, with a destination network expected to be received at
// ate:port{2-9}.
//
//   - Source network: 198.51.100.0/24 (TEST-NET-2)
//   - Destination network: 203.0.113.0/24 (TEST-NET-3)
const (
	plen4 = 30

	ateDstNetName = "dstnet"
	ateDstNetCIDR = "203.0.113.0/24"

	nhIndex  = 42
	nhWeight = 1
	nhgIndex = 10

	awaitDuration = 2 * time.Minute
)

var (
	ateSrc = attrs.Attributes{
		Name:    "ateSrc",
		IPv4:    "192.0.2.1",
		IPv4Len: plen4,
	}

	dutSrc = attrs.Attributes{
		Desc:    "DUT to ATE source",
		IPv4:    "192.0.2.2",
		IPv4Len: plen4,
	}

	dutDst = attrs.Attributes{
		Desc:    "DUT to ATE destination",
		IPv4:    "192.0.2.5",
		IPv4Len: plen4,
	}

	ateDst = attrs.Attributes{
		Name:    "dst",
		IPv4:    "192.0.2.6",
		IPv4Len: plen4,
	}
)

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
	s4a.PrefixLength = ygot.Uint8(plen4)

	return i
}

// configureDUT configures port1 and port2 on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	d := dut.Config()

	p1 := dut.Port(t, "port1")
	i1 := &telemetry.Interface{Name: ygot.String(p1.Name())}
	d.Interface(p1.Name()).Replace(t, configInterfaceDUT(i1, &dutSrc))

	p2 := dut.Port(t, "port2")
	i2 := &telemetry.Interface{Name: ygot.String(p2.Name())}
	d.Interface(p2.Name()).Replace(t, configInterfaceDUT(i2, &dutDst))
}

// configureATE configures port1 and port2 on the ATE.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) *ondatra.ATETopology {
	top := ate.Topology().New()

	p1 := ate.Port(t, "port1")
	i1 := top.AddInterface(ateSrc.Name).WithPort(p1)
	i1.IPv4().
		WithAddress(ateSrc.IPv4CIDR()).
		WithDefaultGateway(dutSrc.IPv4)

	p2 := ate.Port(t, "port2")
	i2 := top.AddInterface(ateDst.Name).WithPort(p2)
	i2.IPv4().
		WithAddress(ateDst.IPv4CIDR()).
		WithDefaultGateway(dutDst.IPv4)
	i2.AddNetwork(ateDstNetName).IPv4().WithAddress(ateDstNetCIDR)

	return top
}

// testTraffic generates traffic flow from source network to
// destination network via ate:port1 to ate:port2 and checks for
// packet loss.
func testTraffic(
	t *testing.T,
	ate *ondatra.ATEDevice,
	top *ondatra.ATETopology,
) {
	i1 := top.Interfaces()[ateSrc.Name]
	i2 := top.Interfaces()[ateDst.Name]
	n2 := i2.Networks()[ateDstNetName]

	ethHeader := ondatra.NewEthernetHeader()
	ipv4Header := ondatra.NewIPv4Header()
	flow := ate.Traffic().NewFlow("Flow").
		WithSrcEndpoints(i1).
		WithDstEndpoints(n2).
		WithHeaders(ethHeader, ipv4Header)

	ate.Traffic().Start(t, flow)
	time.Sleep(15 * time.Second)
	ate.Traffic().Stop(t)

	flowPath := ate.Telemetry().Flow(flow.Name())
	if got := flowPath.LossPct().Get(t); got > 0 {
		t.Errorf("LossPct for flow %s got %g, want 0", flow.Name(), got)
	}
}

// awaitTimeout calls a fluent client Await, adding a timeout to the context.
func awaitTimeout(ctx context.Context, c *fluent.GRIBIClient, t testing.TB) error {
	subctx, cancel := context.WithTimeout(ctx, awaitDuration)
	defer cancel()
	return c.Await(subctx, t)
}

// testArgs holds the objects needed by a test case.
type testArgs struct {
	ctx context.Context
	c   *fluent.GRIBIClient
	dut *ondatra.DUTDevice
	ate *ondatra.ATEDevice
	top *ondatra.ATETopology
}

// testCaseFunc describes a test case function.
type testCaseFunc func(t *testing.T, args *testArgs)

// testModifyNHG configures a NextHopGroup referencing a NextHop.
func testModifyNHG(t *testing.T, args *testArgs) {
	args.c.Modify().AddEntry(t,
		fluent.NextHopEntry().
			WithNetworkInstance(*deviations.DefaultNetworkInstance).
			WithIndex(nhIndex).
			WithIPAddress(ateDst.IPv4),
		fluent.NextHopGroupEntry().
			WithNetworkInstance(*deviations.DefaultNetworkInstance).
			WithID(nhgIndex).
			AddNextHop(nhIndex, nhWeight),
	)
	if err := awaitTimeout(args.ctx, args.c, t); err != nil {
		t.Errorf("Await got error for ModifyRequest: %v", err)
	}

	res := args.c.Results(t)
	chk.HasResult(t, res,
		fluent.OperationResult().
			WithOperationID(1).
			WithOperationType(constants.Add).
			WithNextHopOperation(nhIndex).
			WithProgrammingResult(wantInstalled).
			AsResult(),
	)
	chk.HasResult(t, res,
		fluent.OperationResult().
			WithOperationID(2).
			WithOperationType(constants.Add).
			WithNextHopGroupOperation(nhgIndex).
			WithProgrammingResult(wantInstalled).
			AsResult(),
	)

	t.Run("Telemetry", func(t *testing.T) {
		if !*checkTelemetry {
			t.Skip()
		}
		nhgNhPath := args.dut.Telemetry().NetworkInstance(*deviations.DefaultNetworkInstance).Afts().NextHopGroup(nhgIndex).NextHop(nhIndex)
		if got, want := nhgNhPath.Index().Get(t), uint64(nhIndex); got != want {
			t.Errorf("next-hop-group/next-hop/state/index got %d, want %d", got, want)
		}
		if got, want := nhgNhPath.Weight().Get(t), uint64(nhWeight); got != want {
			t.Errorf("next-hop-group/next-hop/state/weight got %d, want %d", got, want)
		}
	})
}

// testModifyIPv4NHG configures a ModifyRequest with a NextHop and an IPv4Entry before a
// NextHopGroup which is invalid due to the forward reference.
func testModifyIPv4NHG(t *testing.T, args *testArgs) {
	args.c.Modify().AddEntry(t,
		fluent.NextHopEntry().
			WithNetworkInstance(*deviations.DefaultNetworkInstance).
			WithIndex(nhIndex).
			WithIPAddress(ateDst.IPv4),
		fluent.IPv4Entry().
			WithNetworkInstance(*deviations.DefaultNetworkInstance).
			WithPrefix(ateDstNetCIDR).
			WithNextHopGroup(nhgIndex),
		fluent.NextHopGroupEntry().
			WithNetworkInstance(*deviations.DefaultNetworkInstance).
			WithID(nhgIndex).
			AddNextHop(nhIndex, nhWeight),
	)
	if err := awaitTimeout(args.ctx, args.c, t); err != nil {
		t.Fatalf("Await got error for ModifyRequest: %v", err)
	}

	res := args.c.Results(t)
	chk.HasResult(t, res,
		fluent.OperationResult().
			WithOperationID(2).
			WithOperationType(constants.Add).
			WithIPv4Operation(ateDstNetCIDR).
			WithProgrammingResult(fluent.ProgrammingFailed).
			AsResult(),
	)
}

// testModifyNHGIPv4 configures a ModifyRequest with a NextHopGroup and IPv4Entry.
func testModifyNHGIPv4(t *testing.T, args *testArgs) {
	args.c.Modify().AddEntry(t,
		fluent.NextHopEntry().
			WithNetworkInstance(*deviations.DefaultNetworkInstance).
			WithIndex(nhIndex).
			WithIPAddress(ateDst.IPv4),
		fluent.NextHopGroupEntry().
			WithNetworkInstance(*deviations.DefaultNetworkInstance).
			WithID(nhgIndex).
			AddNextHop(nhIndex, nhWeight),
		fluent.IPv4Entry().
			WithNetworkInstance(*deviations.DefaultNetworkInstance).
			WithPrefix(ateDstNetCIDR).
			WithNextHopGroup(nhgIndex),
	)
	if err := awaitTimeout(args.ctx, args.c, t); err != nil {
		t.Fatalf("Await got error for ModifyRequest: %v", err)
	}

	res := args.c.Results(t)
	chk.HasResult(t, res,
		fluent.OperationResult().
			WithOperationID(1).
			WithOperationType(constants.Add).
			WithNextHopOperation(nhIndex).
			WithProgrammingResult(wantInstalled).
			AsResult(),
	)
	chk.HasResult(t, res,
		fluent.OperationResult().
			WithOperationID(2).
			WithOperationType(constants.Add).
			WithNextHopGroupOperation(nhgIndex).
			WithProgrammingResult(wantInstalled).
			AsResult(),
	)
	chk.HasResult(t, res,
		fluent.OperationResult().
			WithOperationID(3).
			WithOperationType(constants.Add).
			WithIPv4Operation(ateDstNetCIDR).
			WithProgrammingResult(wantInstalled).
			AsResult(),
	)

	t.Run("Telemetry", func(t *testing.T) {
		if !*checkTelemetry {
			t.Skip()
		}
		nhgNhPath := args.dut.Telemetry().NetworkInstance(*deviations.DefaultNetworkInstance).Afts().NextHopGroup(nhgIndex).NextHop(nhIndex)
		if got, want := nhgNhPath.Index().Get(t), uint64(nhIndex); got != want {
			t.Errorf("next-hop-group/next-hop/state/index got %d, want %d", got, want)
		}
		if got, want := nhgNhPath.Weight().Get(t), uint64(nhWeight); got != want {
			t.Errorf("next-hop-group/next-hop/state/weight got %d, want %d", got, want)
		}

		ipv4Path := args.dut.Telemetry().NetworkInstance(*deviations.DefaultNetworkInstance).Afts().Ipv4Entry(ateDstNetCIDR)
		if got, want := ipv4Path.NextHopGroup().Get(t), uint64(nhgIndex); got != want {
			t.Errorf("ipv4-entry/state/next-hop-group got %d, want %d", got, want)
		}
		if got, want := ipv4Path.Prefix().Get(t), ateDstNetCIDR; got != want {
			t.Errorf("ipv4-entry/state/prefix got %s, want %s", got, want)
		}
	})

	t.Run("Traffic", func(t *testing.T) {
		testTraffic(t, args.ate, args.top)
	})
}

// testModifyIPv4AddDelAdd configures a ModifyRequest with AFT operations to add, delete,
// and add IPv4Entry.
func testModifyIPv4AddDelAdd(t *testing.T, args *testArgs) {
	testModifyNHG(t, args) // Uses operation IDs 1 and 2.

	ent := fluent.IPv4Entry().
		WithNetworkInstance(*deviations.DefaultNetworkInstance).
		WithPrefix(ateDstNetCIDR).
		WithNextHopGroup(nhgIndex)

	args.c.Modify().
		AddEntry(t, ent).
		DeleteEntry(t, ent).
		AddEntry(t, ent)
	if err := awaitTimeout(args.ctx, args.c, t); err != nil {
		t.Fatalf("Await got error for ModifyRequest: %v", err)
	}

	res := args.c.Results(t)
	chk.HasResult(t, res,
		fluent.OperationResult().
			WithOperationID(3).
			WithOperationType(constants.Add).
			WithIPv4Operation(ateDstNetCIDR).
			WithProgrammingResult(wantInstalled).
			AsResult(),
	)
	chk.HasResult(t, res,
		fluent.OperationResult().
			WithOperationID(4).
			WithOperationType(constants.Delete).
			WithIPv4Operation(ateDstNetCIDR).
			WithProgrammingResult(wantInstalled).
			AsResult(),
	)
	chk.HasResult(t, res,
		fluent.OperationResult().
			WithOperationID(5).
			WithOperationType(constants.Add).
			WithIPv4Operation(ateDstNetCIDR).
			WithProgrammingResult(wantInstalled).
			AsResult(),
	)

	t.Run("Telemetry", func(t *testing.T) {
		if !*checkTelemetry {
			t.Skip()
		}
		ipv4Path := args.dut.Telemetry().NetworkInstance(*deviations.DefaultNetworkInstance).Afts().Ipv4Entry(ateDstNetCIDR)
		if got, want := ipv4Path.NextHopGroup().Get(t), uint64(nhgIndex); got != want {
			t.Errorf("ipv4-entry/state/next-hop-group got %d, want %d", got, want)
		}
		if got, want := ipv4Path.Prefix().Get(t), ateDstNetCIDR; got != want {
			t.Errorf("ipv4-entry/state/prefix got %s, want %s", got, want)
		}
	})

	t.Run("Traffic", func(t *testing.T) {
		testTraffic(t, args.ate, args.top)
	})
}

var cases = []struct {
	name string
	desc string
	fn   testCaseFunc
}{
	{
		name: "Modify NHG",
		desc: "A NextHopGroup referencing a NextHop is responded to with RIB+FIB ACK, and is reported through the AFT telemetry.",
		fn:   testModifyNHG,
	},
	{
		name: "Modify IPv4 and NHG",
		desc: "A single ModifyRequest with the following ordered operations is responded to with an error: (1) An AFTOperation containing an IPv4Entry referencing NextHopGroup 10. (2) An AFTOperation containing a NextHopGroup id=10.",
		fn:   testModifyIPv4NHG,
	},
	{
		name: "Modify NHG and IPv4",
		desc: "A single ModifyRequest with the following ordered operations is installed (verified through telemetry and traffic): (1) An AFTOperation containing a NextHopGroup 10 pointing to a NextHop to ATE port-2. (2) An AFTOperation containing a IPv4Entry referencing NextHopGroup 10.",
		fn:   testModifyNHGIPv4,
	},
	{
		name: "Modify IPv4 Add Del Add",
		desc: "A single ModifyRequest with the following ordered operations is installed (verified through telemetry and traffic): (1) An AFT entry adding IPv4Entry 203.0.113.0/24. (2) An AFT entry deleting IPv4Entry 203.0.113.0/24. (3) An AFT entry adding IPv4Entry 203.0.113.0/24.",
		fn:   testModifyIPv4AddDelAdd,
	},
}

func TestOrderingACK(t *testing.T) {
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

	// Each case will run with its own gRIBI fluent client.
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Name: %s", tc.name)
			t.Logf("Description: %s", tc.desc)

			// Configure the gRIBI client.
			c := fluent.NewClient()
			n := c.Connection().
				WithStub(gribic).
				WithRedundancyMode(fluent.ElectedPrimaryClient).
				WithInitialElectionID(1 /* low */, 0 /* hi */) // ID must be > 0.
			if *fibAck {
				// The main difference WithFIBACK() made was that we are now expecting
				// fluent.InstalledInFIB in []*client.OpResult, as opposed to
				// fluent.InstalledInRIB.
				n.WithFIBACK()
				wantInstalled = fluent.InstalledInFIB
			}

			c.Start(ctx, t)
			defer c.Stop(t)
			c.StartSending(ctx, t)
			if err := awaitTimeout(ctx, c, t); err != nil {
				t.Fatalf("Await got error during session negotiation: %v", err)
			}

			if *flush {
				_, err := c.Flush().
					WithElectionOverride().
					WithAllNetworkInstances().
					Send()
				if err != nil {
					t.Errorf("Cannot flush: %v", err)
				}
			}

			args := &testArgs{ctx: ctx, c: c, dut: dut, ate: ate, top: top}
			tc.fn(t, args)
		})
	}
}
