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
// dut:port2 -> ate:port2 and dut:port3 -> ate:port3.
//
//   * ate:port1 -> dut:port1 subnet 192.0.2.0/30
//   * ate:port2 -> dut:port2 subnet 192.0.2.4/30
//   * ate:port3 -> dut:port3 subnet 192.0.2.8/30
//
//   * Destination network: 198.51.100.0/24

const (
	ipv4PrefixLen = 30
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

// configureDUT configures port1, port2 and port3 on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	d := gnmi.OC()

	p1 := dut.Port(t, "port1")
	i1 := &oc.Interface{Name: ygot.String(p1.Name())}
	gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), configInterfaceDUT(i1, &dutPort1))
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, p1.Name(), *deviations.DefaultNetworkInstance, 0)
	}

	p2 := dut.Port(t, "port2")
	i2 := &oc.Interface{Name: ygot.String(p2.Name())}
	gnmi.Replace(t, dut, d.Interface(p2.Name()).Config(), configInterfaceDUT(i2, &dutPort2))
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, p2.Name(), *deviations.DefaultNetworkInstance, 0)
	}

	p3 := dut.Port(t, "port3")
	i3 := &oc.Interface{Name: ygot.String(p3.Name())}
	gnmi.Replace(t, dut, d.Interface(p3.Name()).Config(), configInterfaceDUT(i3, &dutPort3))
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, p3.Name(), *deviations.DefaultNetworkInstance, 0)
	}
	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, p1)
		fptest.SetPortSpeed(t, p2)
		fptest.SetPortSpeed(t, p3)
	}
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

	flowPath := gnmi.OC().Flow(flow.Name())
	if got := gnmi.Get(t, ate, flowPath.LossPct().State()); got > 0 {
		t.Errorf("LossPct for flow %s got %g, want 0", flow.Name(), got)
	}
}

// testArgs holds the objects needed by a test case.
type testArgs struct {
	ctx     context.Context
	clientA *gribi.Client
	clientB *gribi.Client
	dut     *ondatra.DUTDevice
	ate     *ondatra.ATEDevice
	top     *ondatra.ATETopology
}

// testIPv4LeaderActiveChange first configures an IPV4 Entry through clientB
// and ensures that the entry is active by checking AFT Telemetry and traffic.
// It then configures an IPv4 entry through clientA without updating the election
// and ensures that the installation fails. Finally, it updated the ClientA election
// id to 12, configures an IPV4 through clinetA and ensures that the entry is active
// by checking AFT Telemetry and traffic.
func testIPv4LeaderActiveChange(ctx context.Context, t *testing.T, args *testArgs) {
	// Add an IPv4Entry for 198.51.100.0/24 pointing to ATE port-3 via gRIBI-B,
	// ensure that the entry is active through AFT telemetry and traffic.
	t.Logf("an IPv4Entry for %s pointing to ATE port-3 via gRIBI-B", ateDstNetCIDR)
	args.clientB.BecomeLeader(t)
	args.clientB.AddNH(t, nhIndex, atePort3.IPv4, *deviations.DefaultNetworkInstance, fluent.InstalledInRIB)
	args.clientB.AddNHG(t, nhgIndex, map[uint64]uint64{nhIndex: 1}, *deviations.DefaultNetworkInstance, fluent.InstalledInRIB)
	args.clientB.AddIPv4(t, ateDstNetCIDR, nhgIndex, *deviations.DefaultNetworkInstance, "", fluent.InstalledInRIB)

	// Verify the entry for 198.51.100.0/24 is active through AFT Telemetry.
	t.Logf("Verify the entry for %s is active through AFT Telemetry.", ateDstNetCIDR)
	ipv4Path := gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).Afts().Ipv4Entry(ateDstNetCIDR)
	gnmi.Await(t, args.dut, ipv4Path.Prefix().State(), time.Minute, ateDstNetCIDR)

	// Verify the entry for 198.51.100.0/24 is active through Traffic.
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	dstEndPoint := args.top.Interfaces()[atePort3.Name]
	testTraffic(t, args.ate, args.top, srcEndPoint, dstEndPoint)

	// Add an IPv4Entry for 198.51.100.0/24 pointing to ATE port-2 via gRIBI-A,
	// ensure that the entry is ignored by the DUT.
	t.Logf("Adding an IPv4Entry for %s pointing to ATE port-2 via gRIBI-A", ateDstNetCIDR)
	args.clientA.AddNH(t, nhIndex+1, atePort2.IPv4, *deviations.DefaultNetworkInstance, fluent.ProgrammingFailed)
	args.clientA.AddNHG(t, nhgIndex+1, map[uint64]uint64{nhIndex + 1: 1}, *deviations.DefaultNetworkInstance, fluent.ProgrammingFailed)
	args.clientA.AddIPv4(t, ateDstNetCIDR, nhgIndex+1, *deviations.DefaultNetworkInstance, "", fluent.ProgrammingFailed)

	// Send a ModifyRequest from gRIBI-A updating its election_id to make it leader,
	// followed by a ModifyRequest updating 198.51.100.0/24 pointing to ATE port-2,
	// ensure that routing is updated to receive packets for 198.51.100.0/24 at ATE port-2.
	args.clientA.BecomeLeader(t)
	t.Logf("Adding an IPv4Entry for %s pointing to ATE port-2 via client gRIBI-A", ateDstNetCIDR)
	args.clientA.AddNH(t, nhIndex+2, atePort2.IPv4, *deviations.DefaultNetworkInstance, fluent.InstalledInRIB)
	args.clientA.AddNHG(t, nhgIndex+2, map[uint64]uint64{nhIndex + 2: 1}, *deviations.DefaultNetworkInstance, fluent.InstalledInRIB)
	args.clientA.AddIPv4(t, ateDstNetCIDR, nhgIndex+2, *deviations.DefaultNetworkInstance, "", fluent.InstalledInRIB)

	// Verify the entry for 198.51.100.0/24 is active through AFT Telemetry.
	t.Logf("Verify the entry for %s is active through AFT Telemetry.", ateDstNetCIDR)
	ipv4Path = gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).Afts().Ipv4Entry(ateDstNetCIDR)
	gnmi.Await(t, args.dut, ipv4Path.Prefix().State(), time.Minute, ateDstNetCIDR)

	// Verify with traffic that the entry for 198.51.100.0/24 is installed through the ATE port-2.
	srcEndPoint = args.top.Interfaces()[atePort1.Name]
	dstEndPoint = args.top.Interfaces()[atePort2.Name]
	testTraffic(t, args.ate, args.top, srcEndPoint, dstEndPoint)
}

func TestElectionIDChange(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ctx := context.Background()

	// Configure the DUT
	configureDUT(t, dut)

	// Configure the ATE
	ate := ondatra.ATE(t, "ate")
	top := configureATE(t, ate)
	top.Push(t).StartProtocols(t)

	// Configure the gRIBI client clientA
	clientA := gribi.Client{
		DUT:         dut,
		FIBACK:      false,
		Persistence: true,
	}
	defer clientA.Close(t)

	// Flush all entries after test. ClientA or ClientB doesn't matter since we use Election Override on FlushAll.
	defer clientA.FlushAll(t)

	if err := clientA.Start(t); err != nil {
		t.Fatalf("gRIBI Connection can not be established")
	}

	// Configure the gRIBI client clientB
	clientB := gribi.Client{
		DUT:         dut,
		FIBACK:      false,
		Persistence: true,
	}
	defer clientB.Close(t)
	if err := clientB.Start(t); err != nil {
		t.Fatalf("gRIBI Connection can not be established")
	}

	args := &testArgs{
		ctx:     ctx,
		clientA: &clientA,
		clientB: &clientB,
		dut:     dut,
		ate:     ate,
		top:     top,
	}

	testIPv4LeaderActiveChange(ctx, t, args)
}
