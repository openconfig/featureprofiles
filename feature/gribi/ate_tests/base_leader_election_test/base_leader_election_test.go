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
// The testbed consists of ate:port1 -> dut:port1,
// dut:port2 -> ate:port2 and dut:port3 -> ate:port3.
//
//   * ate:port1 -> dut:port1 subnet 192.0.2.0/30
//   * ate:port2 -> dut:port2 subnet 192.0.2.4/30
//   * ate:port3 -> dut:port3 subnet 192.0.2.8/30
//
//   * Destination network: 198.51.100.0/24

const (
	ipv4PrefixLen = 30
	instance      = "default"
	ateDstNetCIDR = "198.51.100.0/24"
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

	dutPort3 = attrs.Attributes{
		Desc:    "dutPort3",
		IPv4:    "192.0.2.9",
		IPv4Len: ipv4PrefixLen,
	}

	atePort3 = attrs.Attributes{
		Name:    "atePort3",
		IPv4:    "192.0.2.10",
		IPv4Len: ipv4PrefixLen,
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

	p3 := ate.Port(t, "port3")
	i3 := top.AddInterface(atePort3.Name).WithPort(p3)
	i3.IPv4().
		WithAddress(atePort3.IPv4CIDR()).
		WithDefaultGateway(dutPort3.IPv4)

	return top
}

// testTraffic generates traffic flow from source network to
// destination network via srcEndPoint to dstEndPoint and checks for
// packet loss.
func testTraffic(t *testing.T, ate *ondatra.ATEDevice, top *ondatra.ATETopology, srcEndPoint, dstEndPoint *ondatra.Interface) {
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

	flowPath := ate.Telemetry().Flow(flow.Name())
	if got := flowPath.LossPct().Get(t); got > 0 {
		t.Errorf("LossPct for flow %s got %g, want 0", flow.Name(), got)
	}
}

// testArgs holds the objects needed by a test case.
type testArgs struct {
	ctx        context.Context
	clientA    *gribi.GRIBIHandler
	clientB    *gribi.GRIBIHandler
	dut        *ondatra.DUTDevice
	ate        *ondatra.ATEDevice
	top        *ondatra.ATETopology
	wantFibACK bool
}

// configureIPv4ViaClientA configures a IPv4 Entry via ClientA with an Election
// ID of 10 when ClientB is already primary and connected with Election ID of
// 11. Ensure that the entry via ClientA is ignored and not installed.
func configureIPv4ViaNonLeaderClient(t *testing.T, args *testArgs, nonleader *gribi.GRIBIHandler) {
	t.Logf("Adding an IPv4Entry for %s pointing to ATE port-2 via clientA that is not leader.", ateDstNetCIDR)
	nonleader.AddNH(t, nhIndex, "192.0.2.10", instance, fluent.ProgrammingFailed)
	nonleader.AddNHG(t, nhgIndex, map[uint64]uint64{nhIndex: 1}, instance, fluent.ProgrammingFailed)
	nonleader.AddIPV4Entry(t, nhgIndex, instance, ateDstNetCIDR, instance, fluent.ProgrammingFailed)
}

// configureIPv4ViaClientAInstalled configures a IPv4 Entry via ClientA with an
// Election ID of 12. Ensure that the entry via ClientA is installed.
func configureIPv4ViaLeaderClientInstalled(t *testing.T, args *testArgs, leader *gribi.GRIBIHandler, nh string) {
	t.Logf("Adding an IPv4Entry for %s pointing to ATE port-2 via clientA as leader.", ateDstNetCIDR)
	progResult := fluent.InstalledInRIB
	if args.wantFibACK {
		progResult = fluent.InstalledInFIB
	}
	leader.AddNH(t, nhIndex, nh, instance, progResult)
	leader.AddNHG(t, nhgIndex, map[uint64]uint64{nhIndex: 1}, instance, progResult)
	leader.AddIPV4Entry(t, nhgIndex, instance, ateDstNetCIDR, instance, progResult)
}

// testIPv4LeaderActiveChange modifies election ID of ClientA with an Election ID of 12
// and configures a IPv4 entry through this client. Ensure that the entry via ClientA
// is active through AFT Telemetry.
func testIPv4LeaderActiveChange(ctx context.Context, t *testing.T, args *testArgs) {
	// Configure IPv4 route for 198.51.100.0/24 pointing to ATE port-3 via clientB as the leader.
	args.clientB.BecomeLeader(t)
	configureIPv4ViaLeaderClientInstalled(t, args, args.clientB, atePort3.IPv4)

	// Verify the entry for 198.51.100.0/24 is active through AFT Telemetry.
	ipv4Path := args.dut.Telemetry().NetworkInstance(instance).Afts().Ipv4Entry(ateDstNetCIDR)
	if got, want := ipv4Path.Prefix().Get(t), ateDstNetCIDR; got != want {
		t.Errorf("ipv4-entry/state/prefix got %s, want %s", got, want)
	}

	// Verify the entry for 198.51.100.0/24 is active through Traffic.
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	dstEndPoint := args.top.Interfaces()[atePort3.Name]
	testTraffic(t, args.ate, args.top, srcEndPoint, dstEndPoint)

	// Configure IPv4 route for 198.51.100.0/24 pointing to ATE port-3 via clientA without beaing leader.
	// The entry should not be installed due to client is not the leader.
	configureIPv4ViaNonLeaderClient(t, args, args.clientA)

	// Modify  client A to becomes the active Leader.
	args.clientA.BecomeLeader(t)

	// Configure IPv4 route for 198.51.100.0/24 pointing to ATE port-2 via clientA with election ID of 12.
	configureIPv4ViaLeaderClientInstalled(t, args, args.clientA, atePort2.IPv4)

	// Verify the entry for 198.51.100.0/24 is active through AFT Telemetry.
	ipv4Path = args.dut.Telemetry().NetworkInstance(instance).Afts().Ipv4Entry(ateDstNetCIDR)
	if got, want := ipv4Path.Prefix().Get(t), ateDstNetCIDR; got != want {
		t.Errorf("ipv4-entry/state/prefix got %s, want %s", got, want)
	}

	// Verify with traffic that the entry is installed through the ATE port-2.
	srcEndPoint = args.top.Interfaces()[atePort1.Name]
	dstEndPoint = args.top.Interfaces()[atePort2.Name]
	testTraffic(t, args.ate, args.top, srcEndPoint, dstEndPoint)
}

func TestElectionIDChange(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	// Dial gRIBI
	ctx := context.Background()

	// Configure the DUT
	configureDUT(t, dut)

	// Configure the ATE
	ate := ondatra.ATE(t, "ate")
	top := configureATE(t, ate)
	top.Push(t).StartProtocols(t)

	tests := []struct {
		name        string
		desc        string
		fn          func(ctx context.Context, t *testing.T, args *testArgs)
		wantFibACK  bool
		persistance bool
	}{
		{
			name:        "IPv4EntryWithLeaderChange",
			desc:        "Connect gRIBI-A and B to DUT specifying SINGLE_PRIMARY client redundancy without persistance and FibACK",
			fn:          testIPv4LeaderActiveChange,
			wantFibACK:  false,
			persistance: false,
		},
		{
			name:        "IPv4EntryWithLeaderChangeWithPersistance",
			desc:        "Connect gRIBI-A and B to DUT specifying SINGLE_PRIMARY client redundancy with persistance and RibACK",
			fn:          testIPv4LeaderActiveChange,
			wantFibACK:  false,
			persistance: true,
		},
		{
			name:        "IPv4EntryWithLeaderChangeWithPersistanceandFiback",
			desc:        "Connect gRIBI-A and B to DUT specifying SINGLE_PRIMARY redundancy mode with persistance and FibACK",
			fn:          testIPv4LeaderActiveChange,
			wantFibACK:  true,
			persistance: true,
		},
		{
			name:        "IPv4EntryWithLeaderChangeandFibackWithoutPersistance",
			desc:        "Connect gRIBI-A and B to DUT specifying SINGLE_PRIMARY client redundancy with persistance and RibACK",
			fn:          testIPv4LeaderActiveChange,
			wantFibACK:  true,
			persistance: false,
		},
	}

	// Each case will run with its own gRIBI fluent client.
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Name: %s", tt.name)
			t.Logf("Description: %s", tt.desc)

			// Configure the gRIBI client clientA
			clientA := gribi.NewGRIBIFluent(t, dut, tt.persistance, tt.wantFibACK)
			defer clientA.Close(t)

			// Configure the gRIBI client clientB
			clientB := gribi.NewGRIBIFluent(t, dut, tt.persistance, tt.wantFibACK)
			defer clientB.Close(t)

			args := &testArgs{
				ctx:        ctx,
				clientA:    clientA,
				clientB:    clientB,
				dut:        dut,
				ate:        ate,
				top:        top,
				wantFibACK: tt.wantFibACK,
			}

			tt.fn(ctx, t, args)
		})
	}
}
