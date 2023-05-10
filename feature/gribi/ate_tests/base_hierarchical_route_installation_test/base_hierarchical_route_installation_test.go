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

package base_hierarchical_route_installation_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/gribigo/chk"
	"github.com/openconfig/gribigo/constants"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Settings for configuring the baseline testbed with the test
// topology.
//
// The testbed consists of ate:port1 -> dut:port1
// and dut:port2 -> ate:port2.
//
//   - ate:port1 -> dut:port1 subnet 192.0.2.0/30
//   - ate:port2 -> dut:port2 subnet 192.0.2.4/30
const (
	ipv4PrefixLen     = 30
	ateDstNetCIDR     = "198.51.100.0/24"
	ateIndirectNH     = "203.0.113.1"
	ateIndirectNHCIDR = ateIndirectNH + "/32"
	nhIndex           = 1
	nhgIndex          = 42
	nhIndex2          = 2
	nhgIndex2         = 52
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

// configInterfaceDUT configures the interface with the Addrs.
func configInterfaceDUT(i *oc.Interface, a *attrs.Attributes) *oc.Interface {
	i.Description = ygot.String(a.Desc)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	if *deviations.InterfaceEnabled {
		i.Enabled = ygot.Bool(true)
	}

	s := i.GetOrCreateSubinterface(0)
	s4 := s.GetOrCreateIpv4()
	if *deviations.InterfaceEnabled && !*deviations.IPv4MissingEnabled {
		s4.Enabled = ygot.Bool(true)
	}
	s4a := s4.GetOrCreateAddress(a.IPv4)
	s4a.PrefixLength = ygot.Uint8(ipv4PrefixLen)

	return i
}

// configureDUT configures port1 and port2 on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	d := gnmi.OC()

	p1 := dut.Port(t, "port1")
	i1 := &oc.Interface{Name: ygot.String(p1.Name())}
	gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), configInterfaceDUT(i1, &dutPort1))

	p2 := dut.Port(t, "port2")
	i2 := &oc.Interface{Name: ygot.String(p2.Name())}
	gnmi.Replace(t, dut, d.Interface(p2.Name()).Config(), configInterfaceDUT(i2, &dutPort2))
	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, p1)
		fptest.SetPortSpeed(t, p2)
	}
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, p1.Name(), *deviations.DefaultNetworkInstance, 0)
		fptest.AssignToNetworkInstance(t, dut, p2.Name(), *deviations.DefaultNetworkInstance, 0)
	}
}

// configureATE configures port1 and port2 on the ATE.
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

// testTraffic generates traffic flow from source network to
// destination network via srcEndPoint to dstEndPoint and checks for
// packet loss and returns loss percentage as float.
func testTraffic(t *testing.T, ate *ondatra.ATEDevice, top *ondatra.ATETopology, srcEndPoint *ondatra.Interface, dstEndPoint *ondatra.Interface) float32 {
	ethHeader := ondatra.NewEthernetHeader()
	ipv4Header := ondatra.NewIPv4Header()
	ipv4Header.DstAddressRange().
		WithMin("198.51.100.0").
		WithMax("198.51.100.254").
		WithCount(250)

	flow := ate.Traffic().NewFlow("Flow").
		WithSrcEndpoints(srcEndPoint).
		WithDstEndpoints(dstEndPoint).
		WithHeaders(ethHeader, ipv4Header)

	ate.Traffic().Start(t, flow)
	time.Sleep(15 * time.Second)
	ate.Traffic().Stop(t)

	time.Sleep(time.Minute)

	flowPath := gnmi.OC().Flow(flow.Name())
	val, _ := gnmi.Watch(t, ate, flowPath.LossPct().State(), time.Minute, func(val *ygnmi.Value[float32]) bool {
		return val.IsPresent()
	}).Await(t)
	lossPct, present := val.Val()
	if !present {
		t.Fatalf("Could not read loss percentage for flow %q from ATE.", flow.Name())
	}
	return lossPct
}

// awaitTimeout calls a fluent client Await, adding a timeout to the context.
func awaitTimeout(ctx context.Context, c *fluent.GRIBIClient, t testing.TB, timeout time.Duration) error {
	subctx, cancel := context.WithTimeout(ctx, timeout)
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

// setupRecursiveIPv4Entry configures a recursive IPv4 Entry for 198.51.100.0/24
// to NextHopGroup containing one NextHop 203.0.113.1/32. 203.0.113.1/32 is configured to point
// to NextHopGroup containing one NextHop specified to address of ATE port-2.
func setupRecursiveIPv4Entry(t *testing.T, args *testArgs) {
	t.Helper()

	// Add an IPv4Entry for 198.51.100.0/24 pointing to 203.0.113.1/32.
	args.c.Modify().AddEntry(t,
		fluent.NextHopEntry().
			WithNetworkInstance(*deviations.DefaultNetworkInstance).
			WithIndex(nhIndex).
			WithIPAddress(ateIndirectNH))

	args.c.Modify().AddEntry(t,
		fluent.NextHopGroupEntry().
			WithNetworkInstance(*deviations.DefaultNetworkInstance).
			WithID(nhgIndex).
			AddNextHop(nhIndex, 1))

	args.c.Modify().AddEntry(t,
		fluent.IPv4Entry().
			WithPrefix(ateDstNetCIDR).
			WithNetworkInstance(*deviations.DefaultNetworkInstance).
			WithNextHopGroup(nhgIndex))

	if err := awaitTimeout(args.ctx, args.c, t, time.Minute); err != nil {
		t.Fatalf("Could not program entries via c, got err: %v", err)
	}

	// Add an IPv4Entry for 203.0.113.1/32 pointing to 192.0.2.6.
	args.c.Modify().AddEntry(t,
		fluent.NextHopEntry().
			WithNetworkInstance(*deviations.DefaultNetworkInstance).
			WithIndex(nhIndex2).
			WithIPAddress(atePort2.IPv4))

	args.c.Modify().AddEntry(t,
		fluent.NextHopGroupEntry().
			WithNetworkInstance(*deviations.DefaultNetworkInstance).
			WithID(nhgIndex2).
			AddNextHop(nhIndex2, 1))

	args.c.Modify().AddEntry(t,
		fluent.IPv4Entry().
			WithPrefix(ateIndirectNHCIDR).
			WithNetworkInstance(*deviations.DefaultNetworkInstance).
			WithNextHopGroup(nhgIndex2))

	if err := awaitTimeout(args.ctx, args.c, t, time.Minute); err != nil {
		t.Fatalf("Could not program entries via c, got err: %v", err)
	}

	chk.HasResult(t, args.c.Results(t),
		fluent.OperationResult().
			WithIPv4Operation(ateDstNetCIDR).
			WithOperationType(constants.Add).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		chk.IgnoreOperationID(),
	)

	chk.HasResult(t, args.c.Results(t),
		fluent.OperationResult().
			WithIPv4Operation(ateIndirectNHCIDR).
			WithOperationType(constants.Add).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		chk.IgnoreOperationID(),
	)
}

// deleteRecursiveIPv4Entry verifies recursive IPv4 Entry for 198.51.100.0/24 -> 203.0.113.1/32 -> 192.0.2.6.
// The entry for 203.0.113.1/32 is deleted. Verify that the traffic results in loss and removal from AFT.
func deleteRecursiveIPv4Entry(t *testing.T, args *testArgs) {
	args.c.Modify().DeleteEntry(t,
		fluent.IPv4Entry().
			WithPrefix(ateIndirectNHCIDR).
			WithNetworkInstance(*deviations.DefaultNetworkInstance).
			WithNextHopGroup(nhgIndex2))

	if err := awaitTimeout(args.ctx, args.c, t, time.Minute); err != nil {
		t.Fatalf("Could not program entries via c, got err: %v", err)
	}

	chk.HasResult(t, args.c.Results(t),
		fluent.OperationResult().
			WithIPv4Operation(ateIndirectNHCIDR).
			WithOperationType(constants.Delete).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		chk.IgnoreOperationID(),
	)
}

// testRecursiveIPv4Entry verifies recursive IPv4 Entry for 198.51.100.0/24 (a) -> 203.0.113.1/32 (b) -> 192.0.2.6 (c).
// The IPv4 Entry is verified through AFT Telemetry and Traffic.
// TODO: the below code checks entries for each level of the hierarchy statically. We need to create a helper function that does the check recursively.
func testRecursiveIPv4Entry(t *testing.T, args *testArgs) {
	setupRecursiveIPv4Entry(t, args)

	aftsPath := gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).Afts()
	fptest.LogQuery(t, "AFTs", aftsPath.State(), gnmi.Get(t, args.dut, aftsPath.State()))

	// Verify that the entry for 198.51.100.0/24 (a) is installed through AFT Telemetry. a->c or a->b are the expected results.
	ipv4Entry := gnmi.Get(t, args.dut, gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).Afts().Ipv4Entry(ateDstNetCIDR).State())
	if got, want := ipv4Entry.GetPrefix(), ateDstNetCIDR; got != want {
		t.Errorf("TestRecursiveIPv4Entry: ipv4-entry/state/prefix = %v, want %v", got, want)
	}
	if got, want := ipv4Entry.GetOriginProtocol(), oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_GRIBI; got != want {
		t.Errorf("TestRecursiveIPv4Entry: ipv4-entry/state/origin-protocol = %v, want %v", got, want)
	}
	if got, want := ipv4Entry.GetNextHopGroupNetworkInstance(), *deviations.DefaultNetworkInstance; got != want {
		t.Errorf("TestRecursiveIPv4Entry: ipv4-entry/state/next-hop-group-network-instance = %v, want %v", got, want)
	}
	nhgIndexInst := ipv4Entry.GetNextHopGroup()
	if nhgIndexInst == 0 {
		t.Errorf("TestRecursiveIPv4Entry: ipv4-entry/state/next-hop-group is not present")
	}
	nhg := gnmi.Get(t, args.dut, gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).Afts().NextHopGroup(nhgIndexInst).State())
	if got, want := nhg.GetProgrammedId(), uint64(nhgIndex); got != want {
		t.Errorf("TestRecursiveIPv4Entry: next-hop-group/state/programmed-id = %v, want %v", got, want)
	}

	for nhIndexInst, nhgNH := range nhg.NextHop {
		if got, want := nhgNH.GetIndex(), uint64(nhIndexInst); got != want {
			t.Errorf("next-hop index is incorrect: got %v, want %v", got, want)
		}
		nh := gnmi.Get(t, args.dut, gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).Afts().NextHop(nhIndexInst).State())
		// for devices that return  the nexthop with resolving it recursively. For a->b->c the device returns c
		if got := nh.GetIpAddress(); got != atePort2.IPv4 && got != ateIndirectNH {
			t.Errorf("next-hop is incorrect: got %v, want %v or %v ", got, ateIndirectNH, atePort2.IPv4)
		}
	}

	// Verify that the entry for 203.0.113.1/32 (b) is installed through AFT Telemetry. b->c is the expected result.
	ipv4Entry = gnmi.Get(t, args.dut, gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).Afts().Ipv4Entry(ateIndirectNHCIDR).State())
	if got, want := ipv4Entry.GetPrefix(), ateIndirectNHCIDR; got != want {
		t.Errorf("TestRecursiveIPv4Entry = %v: ipv4-entry/state/prefix, want %v", got, want)
	}
	if got, want := ipv4Entry.GetOriginProtocol(), oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_GRIBI; got != want {
		t.Errorf("TestRecursiveIPv4Entry: ipv4-entry/state/origin-protocol = %v, want %v", got, want)
	}
	if got, want := ipv4Entry.GetNextHopGroupNetworkInstance(), *deviations.DefaultNetworkInstance; got != want {
		t.Errorf("TestRecursiveIPv4Entry: ipv4-entry/state/next-hop-group-network-instance = %v, want %v", got, want)
	}
	nhgIndexInst = ipv4Entry.GetNextHopGroup()
	if nhgIndexInst == 0 {
		t.Errorf("TestRecursiveIPv4Entry: ipv4-entry/state/next-hop-group is not present")
	}
	nhg = gnmi.Get(t, args.dut, gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).Afts().NextHopGroup(nhgIndexInst).State())
	if got, want := nhg.GetProgrammedId(), uint64(nhgIndex2); got != want {
		t.Errorf("TestRecursiveIPv4Entry: next-hop-group/state/programmed-id = %v, want %v", got, want)
	}

	for nhIndexInst, nhgNH := range nhg.NextHop {
		if got, want := nhgNH.GetIndex(), uint64(nhIndexInst); got != want {
			t.Errorf("next-hop index is incorrect: got %v, want %v", got, want)
		}
		nh := gnmi.Get(t, args.dut, gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).Afts().NextHop(nhIndexInst).State())
		if got, want := nh.GetIpAddress(), atePort2.IPv4; got != want {
			t.Errorf("next-hop is incorrect: got %v, want %v", got, want)
		}
	}

	// Verify with traffic that the entry is installed through the ATE port-2.
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	dstEndPoint := args.top.Interfaces()[atePort2.Name]

	// Verify that there should be no traffic loss
	loss := testTraffic(t, args.ate, args.top, srcEndPoint, dstEndPoint)

	if loss > 0.5 {
		t.Errorf("Loss: got %g, want < 0.5", loss)
	}

	time.Sleep(time.Minute)

	// Delete the next hop entry for 203.0.113.1/32
	deleteRecursiveIPv4Entry(t, args)

	time.Sleep(30 * time.Second)

	// Verify that the entry for 198.51.100.0/24 is not installed through AFT Telemetry.
	ipv4Path := gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).Afts().Ipv4Entry(ateIndirectNHCIDR)
	if gnmi.Lookup(t, args.dut, ipv4Path.State()).IsPresent() {
		t.Errorf("TestRecursiveIPv4Entry: ipv4-entry/state/prefix: Found route %s that should not exist", ateIndirectNHCIDR)
	}

	// Verify with the deletion of the next hop, traffic loss should be observed.
	loss = testTraffic(t, args.ate, args.top, srcEndPoint, dstEndPoint)

	if loss != 100 {
		t.Errorf("Loss: got %g, want 100", loss)
	}
}

func TestRecursiveIPv4Entries(t *testing.T) {
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

	tests := []struct {
		name string
		desc string
		fn   func(t *testing.T, args *testArgs)
	}{
		{
			name: "TestInvalidIPv4Entry",
			desc: "Program invalid IPv4 prefix in the IPv4Entry and verify that error is being returned.",
			fn:   nil,
		},
		{
			name: "TestMissingNextHopGroupEntry",
			desc: "Verify that missing Next hop group within an IPv4Entry results in error being returned.",
			fn:   nil,
		},
		{
			name: "TestInvalidIPv4NextHop",
			desc: "Verify that invalid IPv4 address in Next hop results in error being returned.",
			fn:   nil,
		},
		{
			name: "TestRecursiveInterfaceEntry",
			desc: "Program 203.0.113.1/32 to NextHopGroup containing one Next hop specified to be egress interface DUT port-2.",
			fn:   nil,
		},
		{
			name: "TestRecursiveMACEntry",
			desc: "Program 203.0.113.1/32 to NextHopGroup containing one Next hop specified to be MAC address of ATE port-2.",
			fn:   nil,
		},
		{
			name: "TestRecursiveNetworkInstanceEntry",
			desc: "Verify that invalid IPv4 address in Next hop results in error being returned.",
			fn:   nil,
		},
		{
			name: "TestRecursiveIPv4Entry",
			desc: "Program 198.51.100.0/24 recursively to ATE Port2 and verify with Telemetry and Traffic.",
			fn:   testRecursiveIPv4Entry,
		},
	}

	const usePreserve = "PRESERVE"

	t.Run(fmt.Sprintf("Persistence=%s", usePreserve), func(t *testing.T) {
		// Each case will run with its own gRIBI fluent client.
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				t.Logf("Name: %s", tc.name)
				t.Logf("Description: %s", tc.desc)

				if tc.fn == nil {
					t.Skip("Test case not implemented.")
				}

				// Configure the gRIBI client c with election ID of 1 and make it leader.
				c := fluent.NewClient()
				c.Connection().
					WithStub(gribic).
					WithInitialElectionID(1, 0).
					WithRedundancyMode(fluent.ElectedPrimaryClient).WithPersistence()

				c.Start(context.Background(), t)
				defer c.Stop(t)
				c.StartSending(context.Background(), t)
				if err := awaitTimeout(ctx, c, t, time.Minute); err != nil {
					t.Fatalf("Await got error during session negotiation for c: %v", err)
				}
				gribi.BecomeLeader(t, c)

				defer func() {
					if err := gribi.FlushAll(c); err != nil {
						t.Errorf("Cannot flush: %v", err)
					}
				}()

				args := &testArgs{
					ctx: ctx,
					c:   c,
					dut: dut,
					ate: ate,
					top: top,
				}

				tc.fn(t, args)
			})
		}
	})
}
