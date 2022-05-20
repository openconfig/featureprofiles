package cisco_gribi_test

import (
	"context"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/gribi/util"
	"github.com/openconfig/gribigo/chk"
	"github.com/openconfig/gribigo/constants"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
)

const (
	ipv4PrefixLen = 24
	ipv6PrefixLen = 126
	instance      = "default"
	vlanMTU       = 1518
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "dutPort1",
		IPv4:    "100.120.1.1",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2000::100:120:1:1",
		IPv6Len: ipv6PrefixLen,
	}

	atePort1 = attrs.Attributes{
		Name:    "atePort1",
		IPv4:    "100.120.1.2",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2000::100:120:1:2",
		IPv6Len: ipv6PrefixLen,
	}

	dutPort2 = attrs.Attributes{
		Desc:    "dutPort2",
		IPv4:    "100.121.1.1",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2000::100:121:1:1",
		IPv6Len: ipv6PrefixLen,
	}

	atePort2 = attrs.Attributes{
		Name:    "atePort2",
		IPv4:    "100.121.1.2",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2000::100:121:1:2",
		IPv6Len: ipv6PrefixLen,
	}

	dutPort3 = attrs.Attributes{
		Desc:    "dutPort3",
		IPv4:    "100.122.1.1",
		IPv4Len: ipv4PrefixLen,
	}

	atePort3 = attrs.Attributes{
		Name:    "atePort3",
		IPv4:    "100.122.1.2",
		IPv4Len: ipv4PrefixLen,
	}

	dutPort4 = attrs.Attributes{
		Desc:    "dutPort4",
		IPv4:    "100.123.1.1",
		IPv4Len: ipv4PrefixLen,
	}

	atePort4 = attrs.Attributes{
		Name:    "atePort4",
		IPv4:    "100.123.1.2",
		IPv4Len: ipv4PrefixLen,
	}
	dutPort5 = attrs.Attributes{
		Desc:    "dutPort5",
		IPv4:    "100.124.1.1",
		IPv4Len: ipv4PrefixLen,
	}

	atePort5 = attrs.Attributes{
		Name:    "atePort5",
		IPv4:    "100.124.1.2",
		IPv4Len: ipv4PrefixLen,
	}
	dutPort6 = attrs.Attributes{
		Desc:    "dutPort6",
		IPv4:    "100.125.1.1",
		IPv4Len: ipv4PrefixLen,
	}

	atePort6 = attrs.Attributes{
		Name:    "atePort6",
		IPv4:    "100.125.1.2",
		IPv4Len: ipv4PrefixLen,
	}
	dutPort7 = attrs.Attributes{
		Desc:    "dutPort7",
		IPv4:    "100.126.1.1",
		IPv4Len: ipv4PrefixLen,
	}

	atePort7 = attrs.Attributes{
		Name:    "atePort7",
		IPv4:    "100.126.1.2",
		IPv4Len: ipv4PrefixLen,
	}
	dutPort8 = attrs.Attributes{
		Desc:    "dutPort8",
		IPv4:    "100.127.1.1",
		IPv4Len: ipv4PrefixLen,
	}
	atePort8 = attrs.Attributes{
		Name:    "atePort8",
		IPv4:    "100.127.1.2",
		IPv4Len: ipv4PrefixLen,
	}

	dutPort2Vlan10 = attrs.Attributes{
		Desc:    "dutPort2Vlan10",
		IPv4:    "100.121.10.1",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2000::100:121:10:1",
		IPv6Len: ipv6PrefixLen,
		MTU:     vlanMTU,
	}

	atePort2Vlan10 = attrs.Attributes{
		Name:    "atePort2Vlan10",
		IPv4:    "100.121.10.2",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2000::100:121:10:2",
		IPv6Len: ipv6PrefixLen,
		MTU:     vlanMTU,
	}

	dutPort2Vlan20 = attrs.Attributes{
		Desc:    "dutPort2Vlan20",
		IPv4:    "100.121.20.1",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2000::100:121:20:1",
		IPv6Len: ipv6PrefixLen,
		MTU:     vlanMTU,
	}

	atePort2Vlan20 = attrs.Attributes{
		Name:    "atePort2Vlan20",
		IPv4:    "100.121.20.2",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2000::100:121:20:2",
		IPv6Len: ipv6PrefixLen,
		MTU:     vlanMTU,
	}

	dutPort2Vlan30 = attrs.Attributes{
		Desc:    "dutPort2Vlan30",
		IPv4:    "100.121.30.1",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2000::100:121:30:1",
		IPv6Len: ipv6PrefixLen,
		MTU:     vlanMTU,
	}

	atePort2Vlan30 = attrs.Attributes{
		Name:    "atePort2Vlan20",
		IPv4:    "100.121.30.2",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2000::100:121:30:2",
		IPv6Len: ipv6PrefixLen,
		MTU:     vlanMTU,
	}
)

// configureATE configures port1, port2 and port3 on the ATE.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) *ondatra.ATETopology {
	top := ate.Topology().New()

	p1 := ate.Port(t, "port1")
	i1 := top.AddInterface(atePort1.Name).WithPort(p1)
	i1.IPv4().
		WithAddress(atePort1.IPv4CIDR()).
		WithDefaultGateway(dutPort1.IPv4)
	i1.IPv6().
		WithAddress(atePort1.IPv6CIDR()).
		WithDefaultGateway(dutPort1.IPv6)

	p2 := ate.Port(t, "port2")
	i2 := top.AddInterface(atePort2.Name).WithPort(p2)
	i2.IPv4().
		WithAddress(atePort2.IPv4CIDR()).
		WithDefaultGateway(dutPort2.IPv4)
	i2.IPv6().
		WithAddress(atePort2.IPv6CIDR()).
		WithDefaultGateway(dutPort2.IPv6)

	p3 := ate.Port(t, "port3")
	i3 := top.AddInterface(atePort3.Name).WithPort(p3)
	i3.IPv4().
		WithAddress(atePort3.IPv4CIDR()).
		WithDefaultGateway(dutPort3.IPv4)

	p4 := ate.Port(t, "port4")
	i4 := top.AddInterface(atePort4.Name).WithPort(p4)
	i4.IPv4().
		WithAddress(atePort4.IPv4CIDR()).
		WithDefaultGateway(dutPort4.IPv4)

	p5 := ate.Port(t, "port5")
	i5 := top.AddInterface(atePort5.Name).WithPort(p5)
	i5.IPv4().
		WithAddress(atePort5.IPv4CIDR()).
		WithDefaultGateway(dutPort5.IPv4)

	p6 := ate.Port(t, "port6")
	i6 := top.AddInterface(atePort6.Name).WithPort(p6)
	i6.IPv4().
		WithAddress(atePort6.IPv4CIDR()).
		WithDefaultGateway(dutPort6.IPv4)

	p7 := ate.Port(t, "port7")
	i7 := top.AddInterface(atePort7.Name).WithPort(p7)
	i7.IPv4().
		WithAddress(atePort7.IPv4CIDR()).
		WithDefaultGateway(dutPort7.IPv4)

	p8 := ate.Port(t, "port8")
	i8 := top.AddInterface(atePort8.Name).WithPort(p8)
	i8.IPv4().
		WithAddress(atePort8.IPv4CIDR()).
		WithDefaultGateway(dutPort8.IPv4)

	//Configure vlans on ATE port2
	i2v10 := top.AddInterface("atePort2Vlan10").WithPort(p2)
	i2v10.Ethernet().WithMTU(1518).WithVLANID(10)
	i2v10.IPv4().
		WithAddress(atePort2Vlan10.IPv4CIDR()).
		WithDefaultGateway(dutPort2Vlan10.IPv4)
	i2v10.IPv6().
		WithAddress(atePort2Vlan10.IPv6CIDR()).
		WithDefaultGateway(dutPort2Vlan10.IPv6)

	i2v20 := top.AddInterface("atePort2Vlan20").WithPort(p2)
	i2v20.Ethernet().WithMTU(1518).WithVLANID(20)
	i2v20.IPv4().
		WithAddress(atePort2Vlan20.IPv4CIDR()).
		WithDefaultGateway(dutPort2Vlan20.IPv4)
	i2v20.IPv6().
		WithAddress(atePort2Vlan20.IPv6CIDR()).
		WithDefaultGateway(dutPort2Vlan20.IPv6)

	i2v30 := top.AddInterface("atePort2Vlan30").WithPort(p2)
	i2v30.Ethernet().WithMTU(1518).WithVLANID(30)
	i2v30.IPv4().
		WithAddress(atePort2Vlan30.IPv4CIDR()).
		WithDefaultGateway(dutPort2Vlan30.IPv4)
	i2v30.IPv6().
		WithAddress(atePort2Vlan30.IPv6CIDR()).
		WithDefaultGateway(dutPort2Vlan30.IPv6)

	return top
}

func testTraffic(t *testing.T, expectPass bool, ate *ondatra.ATEDevice, top *ondatra.ATETopology, srcEndPoint *ondatra.Interface, allPorts map[string]*ondatra.Interface, scale int, hostIP string, args *testArgs, dscp uint8, weights ...float64) {
	ethHeader := ondatra.NewEthernetHeader()
	ethHeader.WithSrcAddress("00:11:01:00:00:01")
	ethHeader.WithDstAddress("00:01:00:02:00:00")

	ipv4Header := ondatra.NewIPv4Header()
	ipv4Header.SrcAddressRange().
		WithMin("198.51.100.0").
		WithMax("198.51.100.254").
		WithCount(250)
	ipv4Header.WithDSCP(dscp)
	ipv4Header.DstAddressRange().WithMin(hostIP).WithCount(uint32(scale)).WithStep("0.0.0.1")

	innerIpv4Header := ondatra.NewIPv4Header()
	innerIpv4Header.WithSrcAddress("200.1.0.2")
	innerIpv4Header.DstAddressRange().WithMin("201.1.0.2").WithCount(10000).WithStep("0.0.0.1")
	dstEndPoint := []ondatra.Endpoint{}

	for _, v := range allPorts {
		if *v != *srcEndPoint {
			dstEndPoint = append(dstEndPoint, v)
		}
	}

	flow := ate.Traffic().NewFlow("Flow").
		WithSrcEndpoints(srcEndPoint).
		WithDstEndpoints(dstEndPoint...)

	flow.WithFrameSize(300).WithFrameRateFPS(1000).WithHeaders(ethHeader, ipv4Header, innerIpv4Header)

	ate.Traffic().Start(t, flow)
	time.Sleep(15 * time.Second)

	stats := ate.Telemetry().InterfaceAny().Counters().Get(t)
	if got := util.CheckTrafficPassViaPortPktCounter(stats); got != expectPass {
		t.Fatalf("Flow %s is not working as expected", flow.Name())
	}

	// if expectPass {
	// 	tolerance := float64(0.03)
	// 	interval := 45 * time.Second
	// 	if len(weights) > 0 {
	// 		CheckDUTTrafficViaInterfaceTelemetry(t, args.dut, args.interfaces.in, args.interfaces.out[:len(weights)], weights, interval, tolerance)
	// 	}
	// }
	ate.Traffic().Stop(t)

	time.Sleep(time.Minute)

	// flowPath := ate.Telemetry().Flow(flow.Name())
	// if got := flowPath.LossPct().Get(t); got > 0 {
	// 	t.Errorf("LossPct for flow %s got %g, want 0", flow.Name(), got)
	// }
}

func flushSever(t *testing.T, args *testArgs) {
	c := args.clientA.Fluent(t)
	if _, err := c.Flush().
		WithElectionOverride().
		WithAllNetworkInstances().
		Send(); err != nil {
		t.Fatalf("could not remove all entries from server, got: %v", err)
	}
}

func configureBaseDoubleRecusionVip1Entry(ctx context.Context, t *testing.T, args *testArgs) {
	t.Helper()
	c := args.clientA.Fluent(t)
	c.Modify().AddEntry(t,
		fluent.NextHopEntry().WithNetworkInstance(instance).WithIndex(args.prefix.vip1NhIndex+2).WithIPAddress(atePort2.IPv4),
		fluent.NextHopEntry().WithNetworkInstance(instance).WithIndex(args.prefix.vip1NhIndex+3).WithIPAddress(atePort3.IPv4),
		fluent.NextHopEntry().WithNetworkInstance(instance).WithIndex(args.prefix.vip1NhIndex+4).WithIPAddress(atePort4.IPv4),
		fluent.NextHopGroupEntry().WithNetworkInstance(instance).WithID(args.prefix.vip1NhgIndex+1).
			AddNextHop(args.prefix.vip1NhIndex+2, 10).
			AddNextHop(args.prefix.vip1NhIndex+3, 20).
			AddNextHop(args.prefix.vip1NhIndex+4, 30),
		fluent.IPv4Entry().WithNetworkInstance(instance).WithPrefix(util.GetIPPrefix(args.prefix.vip1Ip, 0, args.prefix.vipPrefixLength)).WithNextHopGroup(args.prefix.vip1NhgIndex+1),
	)

	if err := args.clientA.AwaitTimeout(ctx, t, time.Minute); err != nil {
		t.Fatalf("Could not program entries via %v, got err: %v", c, err)
	}

	// Verification part
	// NH Verification
	for _, nhIndex := range []uint64{args.prefix.vip1NhIndex + 2, args.prefix.vip1NhIndex + 3, args.prefix.vip1NhIndex + 4} {
		chk.HasResult(t, c.Results(t),
			fluent.OperationResult().
				WithNextHopOperation(nhIndex).
				WithOperationType(constants.Add).
				WithProgrammingResult(fluent.InstalledInRIB).
				AsResult(),
			chk.IgnoreOperationID(),
		)
	}

	// NHG Verification
	chk.HasResult(t, c.Results(t),
		fluent.OperationResult().
			WithNextHopGroupOperation(args.prefix.vip1NhgIndex+1).
			WithOperationType(constants.Add).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		chk.IgnoreOperationID(),
	)

	// IPv4
	chk.HasResult(t, c.Results(t),
		fluent.OperationResult().WithIPv4Operation(util.GetIPPrefix(args.prefix.vip1Ip, 0, args.prefix.vipPrefixLength)).
			WithOperationType(constants.Add).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		chk.IgnoreOperationID(),
	)

}

func configureBaseDoubleRecusionVip2Entry(ctx context.Context, t *testing.T, args *testArgs) {
	t.Helper()
	c := args.clientA.Fluent(t)
	c.Modify().AddEntry(t,
		fluent.NextHopEntry().WithNetworkInstance(instance).WithIndex(args.prefix.vip2NhIndex+5).WithIPAddress(atePort5.IPv4),
		fluent.NextHopEntry().WithNetworkInstance(instance).WithIndex(args.prefix.vip2NhIndex+6).WithIPAddress(atePort6.IPv4),
		fluent.NextHopEntry().WithNetworkInstance(instance).WithIndex(args.prefix.vip2NhIndex+7).WithIPAddress(atePort7.IPv4),
		fluent.NextHopEntry().WithNetworkInstance(instance).WithIndex(args.prefix.vip2NhIndex+8).WithIPAddress(atePort8.IPv4),
		fluent.NextHopGroupEntry().WithNetworkInstance(instance).WithID(args.prefix.vip2NhgIndex+1).
			AddNextHop(args.prefix.vip2NhIndex+5, 10).
			AddNextHop(args.prefix.vip2NhIndex+6, 20).
			AddNextHop(args.prefix.vip2NhIndex+7, 30).
			AddNextHop(args.prefix.vip2NhIndex+8, 40),
		fluent.IPv4Entry().WithNetworkInstance(instance).WithPrefix(util.GetIPPrefix(args.prefix.vip2Ip, 0, args.prefix.vipPrefixLength)).WithNextHopGroup(args.prefix.vip2NhgIndex+1),
	)

	if err := args.clientA.AwaitTimeout(ctx, t, time.Minute); err != nil {
		t.Fatalf("Could not program entries via %v, got err: %v", c, err)
	}

	// Verification part
	// NH Verification
	for _, nhIndex := range []uint64{args.prefix.vip2NhIndex + 5, args.prefix.vip2NhIndex + 6, args.prefix.vip2NhIndex + 7, args.prefix.vip2NhIndex + 8} {
		chk.HasResult(t, c.Results(t),
			fluent.OperationResult().
				WithNextHopOperation(nhIndex).
				WithOperationType(constants.Add).
				WithProgrammingResult(fluent.InstalledInRIB).
				AsResult(),
			chk.IgnoreOperationID(),
		)
	}

	// NHG Verification
	chk.HasResult(t, c.Results(t),
		fluent.OperationResult().
			WithNextHopGroupOperation(args.prefix.vip2NhgIndex+1).
			WithOperationType(constants.Add).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		chk.IgnoreOperationID(),
	)

	// IPv4
	chk.HasResult(t, c.Results(t),
		fluent.OperationResult().WithIPv4Operation(util.GetIPPrefix(args.prefix.vip2Ip, 0, args.prefix.vipPrefixLength)).
			WithOperationType(constants.Add).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		chk.IgnoreOperationID(),
	)
}

func configureBaseDoubleRecusionVrfEntry(ctx context.Context, t *testing.T, scale int, hostIP, prefixLength string, args *testArgs) {
	t.Helper()
	c := args.clientA.Fluent(t)
	c.Modify().AddEntry(t,
		fluent.NextHopEntry().WithNetworkInstance(instance).WithIndex(args.prefix.vrfNhIndex+1).WithIPAddress(args.prefix.vip1Ip),
		fluent.NextHopEntry().WithNetworkInstance(instance).WithIndex(args.prefix.vrfNhIndex+2).WithIPAddress(args.prefix.vip2Ip),
		fluent.NextHopGroupEntry().WithNetworkInstance(instance).WithID(args.prefix.vrfNhgIndex+1).
			AddNextHop(args.prefix.vrfNhIndex+1, 15).
			AddNextHop(args.prefix.vrfNhIndex+2, 85),
	)
	entries := []fluent.GRIBIEntry{}
	for i := 0; i < scale; i++ {
		entries = append(entries,
			fluent.IPv4Entry().
				WithNetworkInstance(args.prefix.vrfName).
				WithPrefix(util.GetIPPrefix(hostIP, i, prefixLength)).
				WithNextHopGroup(args.prefix.vrfNhgIndex+1).
				WithNextHopGroupNetworkInstance(instance))
	}
	c.Modify().AddEntry(t, entries...)

	if err := args.clientA.AwaitTimeout(ctx, t, time.Minute); err != nil {
		t.Fatalf("Could not program entries via %v, got err: %v", c, err)
	}

	// Verification part
	// NH Verification
	for _, nhIndex := range []uint64{args.prefix.vrfNhIndex + 1, args.prefix.vrfNhIndex + 2} {
		chk.HasResult(t, c.Results(t),
			fluent.OperationResult().
				WithNextHopOperation(nhIndex).
				WithOperationType(constants.Add).
				WithProgrammingResult(fluent.InstalledInRIB).
				AsResult(),
			chk.IgnoreOperationID(),
		)
	}

	// NHG Verification
	chk.HasResult(t, c.Results(t),
		fluent.OperationResult().
			WithNextHopGroupOperation(args.prefix.vrfNhgIndex+1).
			WithOperationType(constants.Add).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		chk.IgnoreOperationID(),
	)

	// IPv4
	for i := 0; i < scale; i++ {
		chk.HasResult(t, c.Results(t),
			fluent.OperationResult().WithIPv4Operation(util.GetIPPrefix(hostIP, i, prefixLength)).
				WithOperationType(constants.Add).
				WithProgrammingResult(fluent.InstalledInRIB).
				AsResult(),
			chk.IgnoreOperationID(),
		)
	}
}

func testDoubleRecursionWithUCMP(ctx context.Context, t *testing.T, args *testArgs) {
	defer flushSever(t, args)

	weights := []float64{10 * 15, 20 * 15, 30 * 15, 10 * 85, 20 * 85, 30 * 85, 40 * 85}

	configureBaseDoubleRecusionVip1Entry(ctx, t, args)
	configureBaseDoubleRecusionVip2Entry(ctx, t, args)
	configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)

	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	// dstEndPoint := []*ondatra.Interface{args.top.Interfaces()[atePort2.Name], args.top.Interfaces()[atePort3.Name]}

	testTraffic(t, true, args.ate, args.top, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, args, 0, weights...)
}

func testDeleteAndAddUCMP(ctx context.Context, t *testing.T, args *testArgs) {
	defer flushSever(t, args)

	// Programm the base double recursion entry
	configureBaseDoubleRecusionVip1Entry(ctx, t, args)
	configureBaseDoubleRecusionVip2Entry(ctx, t, args)
	configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)

	srcEndPoint := args.top.Interfaces()[atePort1.Name]

	// Delete UCMP at VRF Level by changing current NHG to single PATH
	args.clientA.AddNHG(t, args.prefix.vrfNhgIndex+1, map[uint64]uint64{args.prefix.vrfNhIndex + 1: 1}, instance, fluent.InstalledInRIB)
	weights := []float64{10, 20, 30}
	testTraffic(t, true, args.ate, args.top, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, args, 0, weights...)

	// Add back UCMP at VRF Level by changing NHG back to UCMP
	args.clientA.AddNHG(t, args.prefix.vrfNhgIndex+1, map[uint64]uint64{args.prefix.vrfNhIndex + 1: 15, args.prefix.vrfNhIndex + 2: 85}, instance, fluent.InstalledInRIB)
	weights = []float64{10 * 15, 20 * 15, 30 * 15, 10 * 85, 20 * 85, 30 * 85, 40 * 85}
	testTraffic(t, true, args.ate, args.top, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, args, 0, weights...)
}

func testVRFnonRecursion(ctx context.Context, t *testing.T, args *testArgs) {
	defer flushSever(t, args)

	// Programm the base double recursion entry
	configureBaseDoubleRecusionVip1Entry(ctx, t, args)
	configureBaseDoubleRecusionVip2Entry(ctx, t, args)
	configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)

	srcEndPoint := args.top.Interfaces()[atePort1.Name]

	// Change VRF Level NHG to single recursion which is same as the VIP1
	args.clientA.AddNHG(t, args.prefix.vrfNhgIndex+1, map[uint64]uint64{args.prefix.vip1NhIndex + 2: 10, args.prefix.vip1NhIndex + 3: 20, args.prefix.vip1NhIndex + 4: 30}, instance, fluent.InstalledInRIB)
	weights := []float64{10, 20, 30}
	testTraffic(t, true, args.ate, args.top, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, args, 0, weights...)

	// Change VRF Level NHG to back to double recursion
	args.clientA.AddNHG(t, args.prefix.vrfNhgIndex+1, map[uint64]uint64{args.prefix.vrfNhIndex + 1: 15, args.prefix.vrfNhIndex + 2: 85}, instance, fluent.InstalledInRIB)
	weights = []float64{10 * 15, 20 * 15, 30 * 15, 10 * 85, 20 * 85, 30 * 85, 40 * 85}
	testTraffic(t, true, args.ate, args.top, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, args, 0, weights...)
}
