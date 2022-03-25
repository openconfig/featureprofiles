package cisco_gribi

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/gribigo/chk"
	"github.com/openconfig/gribigo/constants"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
)

const (
	ipv4PrefixLen = 24
	instance      = "DEFAULT"
	vrfName       = "TE"
	ateDstNetCIDR = "198.51.100.0/24"

	vipPrefixLength = 32

	vip1Ip       = "192.0.2.40"
	vip2Ip       = "192.0.2.42"
	vip1NhIndex  = 100
	vip1NhgIndex = 100

	vip2NhIndex  = 200
	vip2NhgIndex = 200

	vrfNhIndex  = 1000
	vrfNhgIndex = 1000
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

func testTraffic(t *testing.T, ate *ondatra.ATEDevice, top *ondatra.ATETopology, srcEndPoint *ondatra.Interface, allPorts map[string]*ondatra.Interface, scale int, hostIP string) {
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
	innerIpv4Header.DstAddressRange().WithMin("201.1.0.2").WithCount(1000).WithStep("0.0.0.1")
	dstEndPoint := []ondatra.Endpoint{}

	for _, v := range allPorts {
		if v != srcEndPoint {
			dstEndPoint = append(dstEndPoint, v)
		}
	}

	flow := ate.Traffic().NewFlow("Flow").
		WithSrcEndpoints(srcEndPoint).
		WithDstEndpoints(dstEndPoint...)

	flow.WithFrameSize(300).WithFrameRateFPS(1000).WithHeaders(ethHeader, ipv4Header, innerIpv4Header)

	ate.Traffic().Start(t, flow)
	time.Sleep(15 * time.Second)
	ate.Traffic().Stop(t)

	time.Sleep(time.Minute)

	flowPath := ate.Telemetry().Flow(flow.Name())
	if got := flowPath.LossPct().Get(t); got > 0 {
		t.Errorf("LossPct for flow %s got %g, want 0", flow.Name(), got)
	}
}

func flushSever(t *testing.T, args *testArgs) {
	c := args.clientA
	if _, err := c.Flush().
		WithElectionOverride().
		WithAllNetworkInstances().
		Send(); err != nil {
		t.Fatalf("could not remove all entries from server, got: %v", err)
	}
}

func configureBaseDoubleRecusionEntry(ctx context.Context, t *testing.T, scale int, hostIp string, args *testArgs) {
	t.Helper()
	c := args.clientA
	// VIP1  Self-Site
	c.Modify().AddEntry(t,
		fluent.IPv4Entry().WithNetworkInstance(instance).WithPrefix(fmt.Sprintf("%s/%d", vip1Ip, vipPrefixLength)).WithNextHopGroup(vip1NhgIndex+1),
		fluent.NextHopGroupEntry().WithNetworkInstance(instance).WithID(vip1NhgIndex+1).
			AddNextHop(vip1NhIndex+1, 10).
			AddNextHop(vip1NhIndex+2, 20).
			AddNextHop(vip1NhIndex+3, 30),
		fluent.NextHopEntry().WithNetworkInstance(instance).WithIndex(vip1NhIndex+1).WithIPAddress(atePort2.IPv4),
		fluent.NextHopEntry().WithNetworkInstance(instance).WithIndex(vip1NhIndex+2).WithIPAddress(atePort3.IPv4),
		fluent.NextHopEntry().WithNetworkInstance(instance).WithIndex(vip1NhIndex+3).WithIPAddress(atePort4.IPv4),
	)
	// VIP2 Next-Site
	c.Modify().AddEntry(t,
		fluent.IPv4Entry().WithNetworkInstance(instance).WithPrefix(fmt.Sprintf("%s/%d", vip2Ip, vipPrefixLength)).WithNextHopGroup(vip2NhgIndex+1),
		fluent.NextHopGroupEntry().WithNetworkInstance(instance).WithID(vip2NhgIndex+1).
			AddNextHop(vip2NhIndex+1, 10).
			AddNextHop(vip2NhIndex+2, 20).
			AddNextHop(vip2NhIndex+3, 30).
			AddNextHop(vip2NhIndex+4, 40),
		fluent.NextHopEntry().WithNetworkInstance(instance).WithIndex(vip2NhIndex+1).WithIPAddress(atePort5.IPv4),
		fluent.NextHopEntry().WithNetworkInstance(instance).WithIndex(vip2NhIndex+2).WithIPAddress(atePort6.IPv4),
		fluent.NextHopEntry().WithNetworkInstance(instance).WithIndex(vip2NhIndex+3).WithIPAddress(atePort7.IPv4),
		fluent.NextHopEntry().WithNetworkInstance(instance).WithIndex(vip2NhIndex+4).WithIPAddress(atePort8.IPv4),
	)
	// VRF Prefix
	entries := []fluent.GRIBIEntry{}
	for i := 0; i < scale; i++ {
		entries = append(entries, fluent.IPv4Entry().WithNetworkInstance(vrfName).WithPrefix(getIPPrefix(hostIp, i, "32")).WithNextHopGroup(vrfNhgIndex+1).WithNextHopGroupNetworkInstance(instance))
	}
	entries = append(entries,
		fluent.NextHopGroupEntry().WithNetworkInstance(instance).WithID(vrfNhgIndex+1).
			AddNextHop(vrfNhIndex+1, 15). // Self Site
			AddNextHop(vrfNhIndex+2, 85), // Next Site
	)
	switch args.usecase {
	case 1:
		entries = append(entries,
			fluent.NextHopEntry().WithNetworkInstance(instance).WithIndex(vrfNhIndex+1).WithIPAddress(vip1Ip).
				WithEncapsulateHeader(fluent.IPinIP).WithIPinIP("20.20.20.1", "10.10.10.1"),
			fluent.NextHopEntry().WithNetworkInstance(instance).WithIndex(vrfNhIndex+2).WithIPAddress(vip2Ip).
				WithEncapsulateHeader(fluent.IPinIP).WithIPinIP("20.20.20.2", "10.10.10.2"),
		)
	default:
		entries = append(entries,
			fluent.NextHopEntry().WithNetworkInstance(instance).WithIndex(vrfNhIndex+1).WithIPAddress(vip1Ip),
			fluent.NextHopEntry().WithNetworkInstance(instance).WithIndex(vrfNhIndex+2).WithIPAddress(vip2Ip),
		)
	}
	c.Modify().AddEntry(t, entries...)

	if err := awaitTimeout(ctx, c, t, time.Minute); err != nil {
		t.Fatalf("Could not program entries via clientA, got err: %v", err)
	}

	for i := uint64(1); i < 15; i++ {
		chk.HasResult(t, args.clientA.Results(t),
			fluent.OperationResult().
				WithOperationID(i).
				WithProgrammingResult(fluent.InstalledInRIB).
				AsResult(),
		)
	}

}

func changeWeight(ctx context.Context, t *testing.T, c *fluent.GRIBIClient, nhgId, nhIndex int, weights ...uint64) {
	t.Helper()
	entry := fluent.NextHopGroupEntry().WithID(uint64(nhgId))
	for i, w := range weights {
		entry.AddNextHop(uint64(nhIndex+i), w)
	}

	c.Modify().AddEntry(t, entry)

	if err := awaitTimeout(ctx, c, t, time.Minute); err != nil {
		t.Fatalf("Could not program entries via %v, got err: %v", c, err)
	}

	chk.HasResult(t, c.Results(t),
		fluent.OperationResult().
			WithNextHopGroupOperation(uint64(nhgId)).
			WithOperationType(constants.Add).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		chk.IgnoreOperationID(),
	)

}

func testDoubleRecursionWithUCMP(ctx context.Context, t *testing.T, args *testArgs) {
	defer flushSever(t, args)
	hostIp := "11.11.11.0"
	scale := 1000

	configureBaseDoubleRecusionEntry(ctx, t, scale, hostIp, args)

	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	// dstEndPoint := []*ondatra.Interface{args.top.Interfaces()[atePort2.Name], args.top.Interfaces()[atePort3.Name]}

	testTraffic(t, args.ate, args.top, srcEndPoint, args.top.Interfaces(), scale, hostIp)
}

func testChangeVip1UCMP(ctx context.Context, t *testing.T, args *testArgs) {
	// defer flushSever(t, args)
	hostIp := "11.11.11.0"
	scale := 1000

	configureBaseDoubleRecusionEntry(ctx, t, scale, hostIp, args)

	newWeights := []uint64{30, 30, 30}
	changeWeight(ctx, t, args.clientA, vip1NhgIndex+1, vip1NhIndex+1, newWeights...)

	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	// dstEndPoint := []*ondatra.Interface{args.top.Interfaces()[atePort2.Name], args.top.Interfaces()[atePort3.Name]}

	testTraffic(t, args.ate, args.top, srcEndPoint, args.top.Interfaces(), scale, hostIp)
}
