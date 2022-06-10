package cisco_gribi_test

import (
	"context"
	"testing"

	"github.com/openconfig/featureprofiles/internal/attrs"

	"github.com/openconfig/gribigo/fluent"
	
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


func testDoubleRecursionWithUCMP(ctx context.Context, t *testing.T, args *testArgs) {
	defer flushServer(t, args)

	weights := []float64{10 * 15, 20 * 15, 30 * 15, 10 * 85, 20 * 85, 30 * 85, 40 * 85}

	configureBaseDoubleRecusionVip1Entry(ctx, t, args)
	configureBaseDoubleRecusionVip2Entry(ctx, t, args)
	configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)

	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	// dstEndPoint := []*ondatra.Interface{args.top.Interfaces()[atePort2.Name], args.top.Interfaces()[atePort3.Name]}

	testTraffic(t, true, args.ate, args.top, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, args, 0, weights...)
}

func testDeleteAndAddUCMP(ctx context.Context, t *testing.T, args *testArgs) {
	defer flushServer(t, args)

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
	defer flushServer(t, args)

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
