package cisco_gribi

import (
	"context"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
)

const (
	ipv4PrefixLen = 24
	instance      = "DEFAULT"
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "dutPort1",
		IPv4:    "100.120.1.1",
		IPv4Len: ipv4PrefixLen,
	}

	atePort1 = attrs.Attributes{
		Name:    "atePort1",
		IPv4:    "100.120.1.2",
		IPv4Len: ipv4PrefixLen,
	}

	dutPort2 = attrs.Attributes{
		Desc:    "dutPort2",
		IPv4:    "100.121.1.1",
		IPv4Len: ipv4PrefixLen,
	}

	atePort2 = attrs.Attributes{
		Name:    "atePort2",
		IPv4:    "100.121.1.2",
		IPv4Len: ipv4PrefixLen,
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
)

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
	return top
}

func testTraffic(t *testing.T, ate *ondatra.ATEDevice, top *ondatra.ATETopology, srcEndPoint *ondatra.Interface, allPorts map[string]*ondatra.Interface, scale int, hostIP string, args *testArgs, weights ...float64) {
	ethHeader := ondatra.NewEthernetHeader()
	ethHeader.WithSrcAddress("00:11:01:00:00:01")
	ethHeader.WithDstAddress("00:01:00:02:00:00")

	ipv4Header := ondatra.NewIPv4Header()
	ipv4Header.SrcAddressRange().
		WithMin("198.51.100.0").
		WithMax("198.51.100.254").
		WithCount(250)
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
	if got := CheckTrafficPassViaPortPktCounter(stats); !got {
		t.Errorf("LossPct for flow %s", flow.Name())
	}

	//

	tolerance := float64(0.02)
	interval := 45 * time.Second
	if len(weights) > 0 {
		CheckDUTTrafficViaInterfaceTelemetry(t, args.dut, args.interfaces.in, args.interfaces.out[:len(weights)], weights, interval, tolerance)
	}
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

func configureBaseDoubleRecusionVip1Entry(t *testing.T, args *testArgs) {
	t.Helper()
	// Vip1
	weights := map[uint64]uint64{
		args.prefix.vip1NhIndex + 2: 10,
		args.prefix.vip1NhIndex + 3: 20,
		args.prefix.vip1NhIndex + 4: 30,
	}
	args.clientA.AddNH(t, args.prefix.vip1NhIndex+2, atePort2.IPv4, instance, fluent.InstalledInRIB)
	args.clientA.AddNH(t, args.prefix.vip1NhIndex+3, atePort3.IPv4, instance, fluent.InstalledInRIB)
	args.clientA.AddNH(t, args.prefix.vip1NhIndex+4, atePort4.IPv4, instance, fluent.InstalledInRIB)
	args.clientA.AddNHG(t, args.prefix.vip1NhgIndex+1, weights, instance, fluent.InstalledInRIB)
	args.clientA.AddIPV4Entry(t, args.prefix.vip1NhgIndex+1, instance, getIPPrefix(args.prefix.vip1Ip, 0, args.prefix.vipPrefixLength), instance, fluent.InstalledInRIB)
}

func configureBaseDoubleRecusionVip2Entry(t *testing.T, args *testArgs) {
	t.Helper()
	// Vip2
	weights := map[uint64]uint64{
		args.prefix.vip2NhIndex + 5: 10,
		args.prefix.vip2NhIndex + 6: 20,
		args.prefix.vip2NhIndex + 7: 30,
		args.prefix.vip2NhIndex + 8: 40,
	}
	args.clientA.AddNH(t, args.prefix.vip2NhIndex+5, atePort5.IPv4, instance, fluent.InstalledInRIB)
	args.clientA.AddNH(t, args.prefix.vip2NhIndex+6, atePort6.IPv4, instance, fluent.InstalledInRIB)
	args.clientA.AddNH(t, args.prefix.vip2NhIndex+7, atePort7.IPv4, instance, fluent.InstalledInRIB)
	args.clientA.AddNH(t, args.prefix.vip2NhIndex+8, atePort8.IPv4, instance, fluent.InstalledInRIB)

	args.clientA.AddNHG(t, args.prefix.vip2NhgIndex+1, weights, instance, fluent.InstalledInRIB)
	args.clientA.AddIPV4Entry(t, args.prefix.vip2NhgIndex+1, instance, getIPPrefix(args.prefix.vip2Ip, 0, args.prefix.vipPrefixLength), instance, fluent.InstalledInRIB)

}

func configureBaseDoubleRecusionVrfEntry(t *testing.T, scale int, hostIp, prefixLength string, args *testArgs) {
	t.Helper()
	// VRF
	weights := map[uint64]uint64{
		args.prefix.vrfNhIndex + 1: 15,
		args.prefix.vrfNhIndex + 2: 85,
	}
	args.clientA.AddNH(t, args.prefix.vrfNhIndex+1, args.prefix.vip1Ip, instance, fluent.InstalledInRIB)
	args.clientA.AddNH(t, args.prefix.vrfNhIndex+2, args.prefix.vip2Ip, instance, fluent.InstalledInRIB)
	args.clientA.AddNHG(t, args.prefix.vrfNhgIndex+1, weights, instance, fluent.InstalledInRIB)
	for i := 0; i < scale; i++ {
		args.clientA.AddIPV4Entry(t, args.prefix.vrfNhgIndex+1, instance, getIPPrefix(hostIp, i, prefixLength), args.prefix.vrfName, fluent.InstalledInRIB)
	}
}

func testDoubleRecursionWithUCMP(ctx context.Context, t *testing.T, args *testArgs) {
	defer flushSever(t, args)

	weights := []float64{10 * 15, 20 * 15, 30 * 15, 10 * 85, 20 * 85, 30 * 85, 40 * 85}

	configureBaseDoubleRecusionVip1Entry(t, args)
	configureBaseDoubleRecusionVip2Entry(t, args)
	configureBaseDoubleRecusionVrfEntry(t, args.prefix.scale, args.prefix.host, "32", args)

	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	// dstEndPoint := []*ondatra.Interface{args.top.Interfaces()[atePort2.Name], args.top.Interfaces()[atePort3.Name]}

	testTraffic(t, args.ate, args.top, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, args, weights...)
}

func testDeleteAndAddUCMP(ctx context.Context, t *testing.T, args *testArgs) {
	defer flushSever(t, args)

	// Programm the base double recursion entry
	configureBaseDoubleRecusionVip1Entry(t, args)
	configureBaseDoubleRecusionVip2Entry(t, args)
	configureBaseDoubleRecusionVrfEntry(t, args.prefix.scale, args.prefix.host, "32", args)

	srcEndPoint := args.top.Interfaces()[atePort1.Name]

	// Delete UCMP at VRF Level by changing current NHG to single PATH
	args.clientA.AddNHG(t, args.prefix.vrfNhgIndex+1, map[uint64]uint64{args.prefix.vrfNhIndex + 1: 1}, instance, fluent.InstalledInRIB)
	weights := []float64{10, 20, 30}
	testTraffic(t, args.ate, args.top, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, args, weights...)

	// Add back UCMP at VRF Level by changing NHG back to UCMP
	args.clientA.AddNHG(t, args.prefix.vrfNhgIndex+1, map[uint64]uint64{args.prefix.vrfNhIndex + 1: 15, args.prefix.vrfNhIndex + 2: 85}, instance, fluent.InstalledInRIB)
	weights = []float64{10 * 15, 20 * 15, 30 * 15, 10 * 85, 20 * 85, 30 * 85, 40 * 85}
	testTraffic(t, args.ate, args.top, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, args, weights...)
}
