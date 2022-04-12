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

package gribi_ipv4_entry

import (
	"context"
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
//   * Destination network: 1.0.0.0/8

const (
	ipv4PrefixLen = 30
	instance      = "inet.0"
	ateDstNetCIDR = "1.0.0.0/8"
	nhIndexA      = 1
	nhgIndexA     = 42
	nhIndexB      = 2
	nhgIndexB     = 44
	tolerance     = 50
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

func captureTrafficStats(t *testing.T, ate *ondatra.ATEDevice) {
	ap := ate.Port(t, "port1")
	aic := ate.Telemetry().Interface(ap.Name()).Counters()
	outPkts := aic.OutPkts().Get(t)
	fptest.LogYgot(t, "ate:port1 counters", aic, aic.Get(t))

	op1 := ate.Port(t, "port2")
	aic1 := ate.Telemetry().Interface(op1.Name()).Counters()
	inPkts1 := aic1.InPkts().Get(t)
	fptest.LogYgot(t, "ate:port2 counters", aic1, aic1.Get(t))

	op2 := ate.Port(t, "port3")
	aic2 := ate.Telemetry().Interface(op2.Name()).Counters()
	inPkts2 := aic2.InPkts().Get(t)
	fptest.LogYgot(t, "ate:port3 counters", aic2, aic2.Get(t))

	t.Logf("Sent Packets: %d, received Packets on Port2: %d, port3: %d", outPkts, inPkts1, inPkts2)
	diffPerLB := diffPercentage(int(outPkts), int(inPkts1), int(inPkts2))
	if diffPerLB > 5 {
		t.Errorf("LoadBalance Fail: Packets are not load balanced on multiple NHs. Sent: %v, Received: %d, %d", outPkts, inPkts1, inPkts2)
	} else {
		t.Logf("LoadBalance Pass: Packets are load balanced on multiple NHs: %d, Received: %d, %d", outPkts, inPkts1, inPkts2)
	}

}
func diffPercentage(outPkts, inPkts1, inPkts2 int) int {
	diffPkt := 0
	if inPkts1 < inPkts2 {
		diffPkt = inPkts2 - inPkts1
	} else {
		diffPkt = inPkts1 - inPkts2
	}
	Percentage := (diffPkt / outPkts) * 100
	return Percentage
}
func sendTraffic(t *testing.T, ate *ondatra.ATEDevice, flow *ondatra.Flow) {
	t.Logf("Starting traffic")
	ate.Traffic().Start(t, flow)
	time.Sleep(15 * time.Second)
	ate.Traffic().Stop(t)
	time.Sleep(time.Minute)
	t.Logf("Stop traffic")
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
	dut     *ondatra.DUTDevice
	ate     *ondatra.ATEDevice
	top     *ondatra.ATETopology
}

func ConfigureIPv4Entry(ctx context.Context, t *testing.T, client *fluent.GRIBIClient, nextHop string, nhIndex uint64, nhgIndex uint64) {
	t.Helper()
	client.Modify().AddEntry(t,
		fluent.NextHopEntry().WithNetworkInstance(instance).WithIndex(nhIndex).WithIPAddress(nextHop),
		fluent.NextHopGroupEntry().WithNetworkInstance(instance).WithID(nhgIndex).AddNextHop(nhIndex, 1),
		fluent.IPv4Entry().WithNetworkInstance(instance).WithPrefix(ateDstNetCIDR).WithNextHopGroup(nhgIndex),
	)

	if err := awaitTimeout(ctx, client, t, time.Minute); err != nil {
		t.Fatalf("Could not program entries via %v, got err: %v", client, err)
	}

	chk.HasResult(t, client.Results(t),
		fluent.OperationResult().
			WithNextHopOperation(nhIndex).
			WithOperationType(constants.Add).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		chk.IgnoreOperationID(),
	)

	chk.HasResult(t, client.Results(t),
		fluent.OperationResult().
			WithNextHopGroupOperation(nhgIndex).
			WithOperationType(constants.Add).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		chk.IgnoreOperationID(),
	)

	chk.HasResult(t, client.Results(t),
		fluent.OperationResult().
			WithIPv4Operation(ateDstNetCIDR).
			WithOperationType(constants.Add).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		chk.IgnoreOperationID(),
	)

}

func ConfigureIPv4EntryMultiNH(ctx context.Context, t *testing.T, client *fluent.GRIBIClient) {
	t.Helper()

	client.Modify().AddEntry(t,
		fluent.NextHopEntry().WithNetworkInstance(instance).WithIndex(nhIndexA).WithIPAddress("192.0.2.6"),
		fluent.NextHopEntry().WithNetworkInstance(instance).WithIndex(nhIndexB).WithIPAddress("192.0.2.10"),
		fluent.NextHopGroupEntry().WithNetworkInstance(instance).WithID(nhgIndexA).AddNextHop(nhIndexA, 1).AddNextHop(nhIndexB, 1),
		fluent.IPv4Entry().WithNetworkInstance(instance).WithPrefix(ateDstNetCIDR).WithNextHopGroup(nhgIndexA),
	)

	if err := awaitTimeout(ctx, client, t, time.Minute); err != nil {
		t.Fatalf("Could not program entries via %v, got err: %v", client, err)
	}

	chk.HasResult(t, client.Results(t),
		fluent.OperationResult().
			WithNextHopOperation(nhIndexA).
			WithNextHopOperation(nhIndexB).
			WithOperationType(constants.Add).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		chk.IgnoreOperationID(),
	)

	chk.HasResult(t, client.Results(t),
		fluent.OperationResult().
			WithNextHopGroupOperation(nhgIndexA).
			WithOperationType(constants.Add).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		chk.IgnoreOperationID(),
	)

	chk.HasResult(t, client.Results(t),
		fluent.OperationResult().
			WithIPv4Operation(ateDstNetCIDR).
			WithOperationType(constants.Add).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		chk.IgnoreOperationID(),
	)

}

func testSingleIPv4EntrySingleNHG(ctx context.Context, t *testing.T, args *testArgs) {
	// Configure IPv4 route for 1.0.0.0/8 pointing to ATE port-2 via clientA.
	ConfigureIPv4Entry(ctx, t, args.clientA, "192.0.2.6", nhIndexA, nhgIndexA)

	ipv4Path := args.dut.Telemetry().NetworkInstance("default").Afts().Ipv4Entry(ateDstNetCIDR)
	if got, want := ipv4Path.Prefix().Get(t), ateDstNetCIDR; got != want {
		t.Errorf("TestFAIL: ipv4-entry/state/prefix got %s, want %s", got, want)
	} else {
		t.Logf("PASS: ipv4-entry/state/prefix got %s, want %s", got, want)
	}

	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	dstEndPoint := args.top.Interfaces()[atePort2.Name]
	ethHeader := ondatra.NewEthernetHeader()
	ipv4Header := ondatra.NewIPv4Header()
	ipv4Header.DstAddressRange().WithMin("1.0.0.0").WithMax("1.0.0.254").WithCount(250)

	flow := args.ate.Traffic().NewFlow("Flow").WithSrcEndpoints(srcEndPoint).
		WithDstEndpoints(dstEndPoint).WithHeaders(ethHeader, ipv4Header).WithFrameRateFPS(50)

	sendTraffic(t, args.ate, flow)

	flowPath := args.ate.Telemetry().Flow(flow.Name())
	if got := flowPath.LossPct().Get(t); got != 0 {
		t.Errorf("FAIL:LossPct for flow %s got %g, want 0", flow.Name(), got)
	} else {
		t.Logf("PASS: LossPct for flow %s got %g, want 0", flow.Name(), got)
	}
}

//Single IPV4 Entry Multiple NHs. Client A and Client B
func testSingleIPv4EntryMultipleNHs(ctx context.Context, t *testing.T, args *testArgs) {
	t.Logf("Configure IPv4 route for 1.0.0.0/24 pointing to ATE port-2 clientA")
	ConfigureIPv4EntryMultiNH(ctx, t, args.clientA)

	//ipv4Path = args.dut.Telemetry().NetworkInstance(instance).Afts().Ipv4Entry(ateDstNetCIDR)
	ipv4Path := args.dut.Telemetry().NetworkInstance("default").Afts().Ipv4Entry(ateDstNetCIDR)
	if got, want := ipv4Path.Prefix().Get(t), ateDstNetCIDR; got != want {
		t.Errorf("ipv4-entry/state/prefix got %s, want %s", got, want)
	}

	// Verify with traffic that the entry is installed through the ATE port-2.
	t.Logf("Verify traffic load balance with Client A/Client B")
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	dstEndPoint1 := args.top.Interfaces()[atePort2.Name]
	dstEndPoint2 := args.top.Interfaces()[atePort3.Name]
	ethHeader := ondatra.NewEthernetHeader()
	ipv4Header := ondatra.NewIPv4Header()
	ipv4Header.DstAddressRange().WithMin("1.0.0.0").WithMax("1.0.0.254").WithCount(250)

	flow := args.ate.Traffic().NewFlow("Flow").WithSrcEndpoints(srcEndPoint).
		WithDstEndpoints(dstEndPoint1, dstEndPoint2).WithHeaders(ethHeader, ipv4Header).WithFrameRateFPS(50)

	sendTraffic(t, args.ate, flow)

	flowPath := args.ate.Telemetry().Flow(flow.Name())
	if got := flowPath.LossPct().Get(t); got != 0 {
		t.Errorf("LossPct for flow %s got %g, want 0", flow.Name(), got)
	} else {
		t.Logf("PASS: LossPct for flow %s got %g, want 0", flow.Name(), got)
	}

	captureTrafficStats(t, args.ate)
}

//Single IPV4 Entry Invalid NH. Client A
func testSingleIPv4EntryInvalidNH(ctx context.Context, t *testing.T, args *testArgs) {
	t.Logf("Configure IPv4 route for 1.0.0.0/8 pointing to invalid NH 192.0.2.60")
	ConfigureIPv4Entry(ctx, t, args.clientA, "192.0.2.60", nhIndexA, nhgIndexA)

	//ipv4Path := args.dut.Telemetry().NetworkInstance("default").Afts().Ipv4Entry(ateDstNetCIDR)
	/*if got, want := ipv4Path.Prefix().Get(t), ateDstNetCIDR; got != want {
		t.Logf("1.0.0.0/8 with invalid nexthop 192.0.2.60 is not active state")
	}*/

	t.Logf("Verify with traffic that the entry is installed through the ATE port-2.")
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	dstEndPoint := args.top.Interfaces()[atePort2.Name]
	ethHeader := ondatra.NewEthernetHeader()
	ipv4Header := ondatra.NewIPv4Header()
	ipv4Header.DstAddressRange().WithMin("1.0.0.0").WithMax("1.0.0.254").WithCount(250)

	flow := args.ate.Traffic().NewFlow("Flow").WithSrcEndpoints(srcEndPoint).
		WithDstEndpoints(dstEndPoint).WithHeaders(ethHeader, ipv4Header).WithFrameRateFPS(50)

	sendTraffic(t, args.ate, flow)

	flowPath := args.ate.Telemetry().Flow(flow.Name())
	if got := flowPath.LossPct().Get(t); got != 100.0 {
		t.Errorf("FAIL: LossPct for flow %s got %g, want 0", flow.Name(), got)
	} else {
		t.Logf("PASS: LossPct for flow %s got %g, want 100", flow.Name(), got)
	}
	//Update nexthop to down interface
	//Bringdown interface dut - ate port3
	//portJuniper := args.dut.Port(t, "port3")
	//fmt.Printf("args.dut.Vendor(): %v\n", args.dut.Vendor())
	//op := args.dut.Operations().NewSetInterfaceState().WithPhysicalInterface(portJuniper).WithStateEnabled(false)
	//op.Operate(t)
	/*t.Logf("Make dutAte Port-3 down")
	portAte := args.ate.Port(t, "port3")
	op := args.ate.Operations().NewSetInterfaceState().WithPhysicalInterface(portAte).WithStateEnabled(false)
	op.Operate(t)*/
	interfaceStateChange(t, args.ate, false)
	t.Logf("Add IPv4 entry with down interface as nexthop")
	ConfigureIPv4Entry(ctx, t, args.clientA, "192.0.2.10", nhIndexA, nhgIndexA)

	/*ipv4Path = args.dut.Telemetry().NetworkInstance("default").Afts().Ipv4Entry(ateDstNetCIDR)
	if got, want := ipv4Path.Prefix().Get(t), ateDstNetCIDR; got != want {
		t.Logf("1.0.0.0/8 with invalid nexthop 192.0.2.10 is not in active state")
	}*/

	t.Logf("Verify with traffic to destination added when down interface as nexthop")
	srcEndPoint = args.top.Interfaces()[atePort1.Name]
	dstEndPoint = args.top.Interfaces()[atePort3.Name]
	ipv4Header.DstAddressRange().WithMin("1.0.0.0").WithMax("1.0.0.254").WithCount(250)

	flow = args.ate.Traffic().NewFlow("Flow").WithSrcEndpoints(srcEndPoint).
		WithDstEndpoints(dstEndPoint).WithHeaders(ethHeader, ipv4Header).WithFrameRateFPS(50)

	sendTraffic(t, args.ate, flow)

	flowPath = args.ate.Telemetry().Flow(flow.Name())
	if got := flowPath.LossPct().Get(t); got != 100.0 {
		t.Errorf("FAIL: LossPct for flow %s got %g, want 0", flow.Name(), got)
	} else {
		t.Logf("PASS: LossPct for flow %s got %g, want 100", flow.Name(), got)
	}

	interfaceStateChange(t, args.ate, true)
	//op = args.dut.Operations().NewSetInterfaceState().WithPhysicalInterface(portJuniper).WithStateEnabled(true)
	//op.Operate(t)
	//t.Logf("Bringup dutAte port-3 back online ")
	//op = args.ate.Operations().NewSetInterfaceState().WithPhysicalInterface(portAte).WithStateEnabled(true)
	//op.Operate(t)

}
func interfaceStateChange(t *testing.T, ate *ondatra.ATEDevice, intfstate bool) {
	if intfstate {
		t.Logf("Make dutAte Port-3 UP")
	} else {
		t.Logf("Make dutAte Port-3 DOWN")
	}

	portAte := ate.Port(t, "port3")
	op := ate.Operations().NewSetInterfaceState().WithPhysicalInterface(portAte).WithStateEnabled(intfstate)
	op.Operate(t)
}

//Testcase to execute TE2.1
func TestSingleIPv4EntrySingleNH(t *testing.T) {
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

	tt := []struct {
		name string
		desc string
		fn   func(ctx context.Context, t *testing.T, args *testArgs)
	}{{
		name: "Single IPV4 Entry with one nexthop",
		desc: "Connect gRIBI-A to DUT",
		fn:   testSingleIPv4EntrySingleNHG,
	}, {
		name: "Single IPV4 Entry with multiple nexthops",
		desc: "Connect gRIBI-A to DUT",
		fn:   testSingleIPv4EntryMultipleNHs,
	}, {
		name: "Single IPV4 Entry with invalid nexthop",
		desc: "Connect gRIBI-A to DUT",
		fn:   testSingleIPv4EntryInvalidNH,
	}}
	// Each case will run with its own gRIBI fluent client.
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Name: %s", tc.name)
			t.Logf("Description: %s", tc.desc)

			// Configure the gRIBI client clientA with election ID of 10.
			clientA := fluent.NewClient()

			clientA.Connection().WithStub(gribic).WithInitialElectionID(10, 0).
				WithRedundancyMode(fluent.ElectedPrimaryClient)

			clientA.Start(ctx, t)
			defer clientA.Stop(t)
			clientA.StartSending(ctx, t)
			if err := awaitTimeout(ctx, clientA, t, time.Minute); err != nil {
				t.Fatalf("Await got error during session negotiation for clientA: %v", err)
			}

			chk.HasResult(t, clientA.Results(t),
				fluent.OperationResult().
					WithCurrentServerElectionID(10, 0).
					AsResult(),
				chk.IgnoreOperationID(),
			)

			args := &testArgs{
				ctx:     ctx,
				clientA: clientA,
				dut:     dut,
				ate:     ate,
				top:     top,
			}
			tc.fn(ctx, t, args)
		})
	}
}

func tTestSingleIPv4EntryMultiNHs(t *testing.T) {
	t.Logf("Test Single IPv4 Entry with Multiple  NextHops")
	dut := ondatra.DUT(t, "dut")

	// Dial gRIBI
	ctx := context.Background()
	gribic := dut.RawAPIs().GRIBI().Default(t)

	// Configure the DUT
	t.Logf("Configure DUT")
	configureDUT(t, dut)

	// Configure the ATE
	ate := ondatra.ATE(t, "ate")
	top := configureATE(t, ate)
	t.Logf("Configure ATE, startprotocols")
	top.Push(t).StartProtocols(t)

	// Configure the gRIBI client clientA .
	t.Logf("Configure the gRIBI clientA with SINGLE_PRIMARY, Election id 10")
	clientA := fluent.NewClient()
	clientA.Connection().WithStub(gribic).WithInitialElectionID(10, 0).
		WithRedundancyMode(fluent.ElectedPrimaryClient)

	clientA.Start(ctx, t)
	defer clientA.Stop(t)
	clientA.StartSending(ctx, t)
	if err := awaitTimeout(ctx, clientA, t, time.Minute); err != nil {
		t.Fatalf("Await got error during session negotiation for clientA: %v", err)
	}

	args := &testArgs{
		ctx:     ctx,
		clientA: clientA,
		dut:     dut,
		ate:     ate,
		top:     top,
	}
	clientA.Flush().WithAllNetworkInstances().WithElectionOverride()

	testSingleIPv4EntryMultipleNHs(ctx, t, args)
}

func tTestSingleIPv4EntryInvalidNHs(t *testing.T) {
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

	// Configure the gRIBI client clientA with election ID of 10.
	clientA := fluent.NewClient()
	clientA.Connection().WithStub(gribic).WithInitialElectionID(10, 0).
		WithRedundancyMode(fluent.ElectedPrimaryClient)
	clientA.Start(ctx, t)
	defer clientA.Stop(t)
	clientA.StartSending(ctx, t)
	if err := awaitTimeout(ctx, clientA, t, time.Minute); err != nil {
		t.Fatalf("Await got error during session negotiation for clientA: %v", err)
	}

	args := &testArgs{
		ctx:     ctx,
		clientA: clientA,
		dut:     dut,
		ate:     ate,
		top:     top,
	}
	clientA.Flush().WithAllNetworkInstances()

	testSingleIPv4EntryInvalidNH(ctx, t, args)
}
