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

package leader_failover_test

import (
	"context"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
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
// The testbed consists of:
//   ate:port1 -> dut:port1 and
//   dut:port2 -> ate:port2
//
//   * ate:port1 -> dut:port1 subnet 192.0.2.0/30
//   * ate:port2 -> dut:port2 subnet 192.0.2.4/30
//
//   * Destination network: -> 203.0.113.0/24

const (
	ipv4PrefixLen = 30
	instance      = "DEFAULT"
	ateDstNetCIDR = "203.0.113.0/24"
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

// configInterfaceDUT configures the DUT interfaces.
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

// configureDUT configures port1 and port2 on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	d := dut.Config()

	p1 := dut.Port(t, "port1")
	i1 := &telemetry.Interface{Name: ygot.String(p1.Name())}
	d.Interface(p1.Name()).Replace(t, configInterfaceDUT(i1, &dutPort1))

	p2 := dut.Port(t, "port2")
	i2 := &telemetry.Interface{Name: ygot.String(p2.Name())}
	d.Interface(p2.Name()).Replace(t, configInterfaceDUT(i2, &dutPort2))
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
// packet loss. The boolean flag wantLoss could be used to check
// either for 100% loss (when set to true) or 0% loss (when set to false).
func testTraffic(t *testing.T, ate *ondatra.ATEDevice, top *ondatra.ATETopology, srcEndPoint, dstEndPoint *ondatra.Interface, wantLoss bool) {
	ethHeader := ondatra.NewEthernetHeader()
	ipv4Header := ondatra.NewIPv4Header()
	ipv4Header.DstAddressRange().
		WithMin("203.0.113.1").
		WithMax("203.0.113.250").
		WithCount(250)

	flow := ate.Traffic().NewFlow("Flow").
		WithSrcEndpoints(srcEndPoint).
		WithDstEndpoints(dstEndPoint).
		WithHeaders(ethHeader, ipv4Header)

	ate.Traffic().Start(t, flow)
	time.Sleep(15 * time.Second)
	ate.Traffic().Stop(t)

	time.Sleep(time.Minute)

	flowPath := ate.Telemetry().Flow(flow.Name())

	if !wantLoss {
		if got := flowPath.LossPct().Get(t); got != 0 {
			t.Errorf("FAIL: LossPct for flow named %s got %g, want 0", flow.Name(), got)
		} else {
			t.Logf("LossPct for flow named %s got %g, want 0", flow.Name(), got)
		}
	} else {
		if got := flowPath.LossPct().Get(t); got != 100 {
			t.Errorf("FAIL: LossPct for flow named %s got %g, want 100", flow.Name(), got)
		} else {
			t.Logf("LossPct for flow named %s got %g, want 100", flow.Name(), got)
		}
	}
}

// testArgs holds the objects needed by the test case.
type testArgs struct {
	ctx context.Context
	dut *ondatra.DUTDevice
	ate *ondatra.ATEDevice
	top *ondatra.ATETopology
}

// testIPv4RouteAdd configures an IPV4 Entry through the given gRIBI client
// and ensures that the entry is active by checking AFT Telemetry and Traffic.
func testIPv4RouteAdd(ctx context.Context, t *testing.T, args *testArgs, clientA *gribi.Client) {

	// Add an IPv4Entry for 203.0.113.0/24 pointing to ATE port-2 via gRIBI.
	t.Logf("Add an IPv4Entry for %s pointing to ATE port-2 via clientA", ateDstNetCIDR)
	clientA.AddNH(t, nhIndex, atePort2.IPv4, instance, fluent.InstalledInRIB)
	clientA.AddNHG(t, nhgIndex, map[uint64]uint64{nhIndex: 1}, instance, fluent.InstalledInRIB)
	clientA.AddIPv4(t, ateDstNetCIDR, nhgIndex, instance, "", fluent.InstalledInRIB)

	// Verify the entry for 203.0.113.0/24 is active through AFT Telemetry.
	t.Logf("Verify through AFT Telemetry that 203.0.113.0/24 is active")
	ipv4Path := args.dut.Telemetry().NetworkInstance(instance).Afts().Ipv4Entry(ateDstNetCIDR)
	if got, want := ipv4Path.Prefix().Get(t), ateDstNetCIDR; got != want {
		t.Errorf("ipv4-entry/state/prefix got %s, want %s", got, want)
	}

	// Verify by running traffic that the route entry for 203.0.113.0/24 is installed and active.
	t.Logf("Verify by running traffic that 203.0.113.0/24 is active")
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	dstEndPoint := args.top.Interfaces()[atePort2.Name]

	testTraffic(t, args.ate, args.top, srcEndPoint, dstEndPoint, false)
}

func TestLeaderFailover(t *testing.T) {

	start := time.Now()
	dut := ondatra.DUT(t, "dut")
	ctx := context.Background()

	// Configure the DUT.
	t.Logf("Configure DUT")
	configureDUT(t, dut)

	// Configure the ATE.
	t.Logf("Configure ATE")
	ate := ondatra.ATE(t, "ate")
	top := configureATE(t, ate)
	top.Push(t).StartProtocols(t)

	t.Logf("Time check: %s", time.Since(start))

	args := &testArgs{
		ctx: ctx,
		dut: dut,
		ate: ate,
		top: top,
	}

	// Set parameters for gRIBI client clientA.
	clientA := &gribi.Client{
		DUT:                  dut,
		FibACK:               false,
		Persistence:          false,
		InitialElectionIDLow: 10,
	}

	t.Run("SINGLE_PRIMARY/PERSISTENCE=DELETE", func(t *testing.T) {

		defer clientA.Close(t)

		// Establish gRIBI client connection.
		t.Logf("Establish gRIBI client connection with PERSISTENCE set to FALSE/DELETE")
		if err := clientA.Start(t); err != nil {
			t.Fatalf("gRIBI Connection for clientA could not be established")
		}

		// Add a route to 203.0.113.0/24 through gRIBI and verify through telemetry and traffic.
		t.Logf("Add gRIBI route to 203.0.113.0/24 and verify through Telemetry and Traffic")
		testIPv4RouteAdd(ctx, t, args, clientA)

		t.Logf("Time check: %s", time.Since(start))

		// Close gRIBI client connection.
		t.Logf("Close gRIBI client connection")
		clientA.Close(t)
	})

	t.Run("Verify Route Is Deleted When Client Disconnects", func(t *testing.T) {

		// Verify through AFT Telemetry that the entry for 203.0.113.0/24 has been deleted.
		t.Logf("Verify through Telemetry that the route to 203.0.113.0/24 has been deleted after gRIBI client disconnected")
		ipv4Path := args.dut.Telemetry().NetworkInstance(instance).Afts().Ipv4Entry(ateDstNetCIDR)
		got2 := ipv4Path.Prefix().Lookup(t)
		t.Logf("After gRIBI client disconnect, telemetry got is %s, want is <nil>", got2)
		if got2 != nil {
			t.Errorf("After gRIBI client disconnect, lookup of ipv4-entry/state/prefix got %s, want nil", got2)
		}

		t.Logf("Time check: %s", time.Since(start))

		// Verify that traffic incurs 100% loss since the route to 203.0.113.0/24 was deleted.
		t.Logf("Verify traffic to 203.0.113.0/24 is dropped")
		srcEndPoint := args.top.Interfaces()[atePort1.Name]
		dstEndPoint := args.top.Interfaces()[atePort2.Name]
		testTraffic(t, args.ate, args.top, srcEndPoint, dstEndPoint, true)

		t.Logf("Time check: %s", time.Since(start))

	})

	// Change parameters for gRIBI client clientA.
	// Set Persistence to true.
	clientA = &gribi.Client{
		DUT:                  args.dut,
		FibACK:               false,
		Persistence:          true,
		InitialElectionIDLow: 10,
	}

	t.Run("SINGLE_PRIMARY/PERSISTENCE=PRESERVE", func(t *testing.T) {

		// Reconnect gRIBI client with persistence set to preserve.
		t.Logf("Reconnect clientA, with PERSISTENCE set to TRUE/PRESERVE")

		// Reconnect gRIBI client.
		if err := clientA.Start(t); err != nil {
			t.Fatalf("gRIBI Connection for clientA could not be re-established")
		}

		defer clientA.Close(t)

		// Add the route to 203.0.113.0/24 back and verify through telemetry and traffic.
		t.Logf("Add gRIBI route again to 203.0.113.0/24 and verify through Telemetry and Traffic")
		testIPv4RouteAdd(ctx, t, args, clientA)

		t.Logf("Time check: %s", time.Since(start))

		// Close the gRIBI client connection again.
		t.Logf("Close gRIBI client connection again")
		clientA.Close(t)

	})

	t.Run("Verify Route Is Preserved When Client Disconnects", func(t *testing.T) {

		// Verify the entry for 203.0.113.0/24 is active through Traffic
		// since the route will be left intact after gRIBI client disconnects
		// due to PERSISTENCE being set to PRESERVE mode.
		t.Logf("Verify with traffic that the route to 203.0.113.0/24 is present")
		srcEndPoint := args.top.Interfaces()[atePort1.Name]
		dstEndPoint := args.top.Interfaces()[atePort2.Name]
		testTraffic(t, args.ate, args.top, srcEndPoint, dstEndPoint, false)

		t.Logf("Time check: %s", time.Since(start))

		// Verify the entry for 203.0.113.0/24 is active through Telemetry.
		t.Logf("Verify through telemetry that the route to 203.0.113.0/24 is present")
		ipv4Path := args.dut.Telemetry().NetworkInstance(instance).Afts().Ipv4Entry(ateDstNetCIDR)
		if got3, want3 := ipv4Path.Prefix().Get(t), ateDstNetCIDR; got3 != want3 {
			t.Errorf("ipv4-entry/state/prefix got %s, want %s", got3, want3)
		}

	})

	t.Run("Reconnect Client, Delete Route and Verify No Traffic", func(t *testing.T) {

		// Reconnect client.
		t.Logf("Reconnect clientA again")
		if err := clientA.Start(t); err != nil {
			t.Fatalf("gRIBI Connection for clientA could not be re-established")
		}

		defer clientA.Close(t)

		// Delete the route to 203.0.113.0/24 that was added through the previous session.
		t.Logf("Delete gRIBI route 203.0.113.0/24 and verify through Telemetry and Traffic")
		ipv4Entry := fluent.IPv4Entry().WithPrefix(ateDstNetCIDR).WithNetworkInstance(instance)
		clientA.DeleteIpv4Entry(t, ipv4Entry)

		// Verify the entry for 203.0.113.0/24 is inactive through AFT Telemetry.
		ipv4Path := args.dut.Telemetry().NetworkInstance(instance).Afts().Ipv4Entry(ateDstNetCIDR)
		got4 := ipv4Path.Prefix().Lookup(t)
		t.Logf("After delete, telemetry got is %s, want is <nil>", got4)
		if got4 != nil {
			t.Errorf("After delete, lookup of ipv4-entry/state/prefix got %s, want nil", got4)
		}

		t.Logf("Time check: %s", time.Since(start))

		// Verify that traffic to 203.0.113.0/24 gets dropped since the route has been deleted.
		t.Logf("Verify traffic to 203.0.113.0/24 is dropped")
		srcEndPoint := args.top.Interfaces()[atePort1.Name]
		dstEndPoint := args.top.Interfaces()[atePort2.Name]
		testTraffic(t, args.ate, args.top, srcEndPoint, dstEndPoint, true)

	})

	t.Logf("Test run time: %s", time.Since(start))
}
