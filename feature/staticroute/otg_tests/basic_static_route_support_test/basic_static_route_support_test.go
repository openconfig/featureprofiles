package basic_static_route_support_test

import (
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	ipv4PrefixLen           = 30
	ipv6PrefixLen           = 126
	isisName                = "DEFAULT"
	dutAreaAddr             = "49.0001"
	ateAreaAddr             = "49.0002"
	dutSysID                = "1920.0000.2001"
	ate1SysID               = "640000000001"
	ate2SysID               = "640000000002"
	v4Route                 = "203.0.113.0"
	v4TrafficStart          = "203.0.113.1"
	v4RoutePrefix           = uint32(24)
	v6Route                 = "2001:db8:128:128::0"
	v6TrafficStart          = "2001:db8:128:128::1"
	v6RoutePrefix           = uint32(64)
	v4LoopbackRoute         = "198.51.100.100"
	v4LoopbackRoutePrefix   = uint32(32)
	v6LoopbackRoute         = "2001:db8:64:64::1"
	v6LoopbackRoutePrefix   = uint32(128)
	v4Flow                  = "v4Flow"
	v6Flow                  = "v6Flow"
	trafficDuration         = 2 * time.Minute
	lossTolerance           = float64(1)
	ecmpTolerance           = uint64(2)
	port1Tag                = "0x01"
	port2Tag                = "0x02"
	dummyV6                 = "2001:db8::192:0:2:d"
	dummyMAC                = "00:1A:11:00:0A:BC"
	explicitMetricTolerance = float64(2)
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "dutPort1",
		Name:    "port1",
		IPv4:    "192.0.2.1",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:db8::192:0:2:1",
		IPv6Len: ipv6PrefixLen,
	}

	atePort1 = attrs.Attributes{
		Name:    "atePort1",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.0.2.2",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:db8::192:0:2:2",
		IPv6Len: ipv6PrefixLen,
	}

	dutPort2 = attrs.Attributes{
		Desc:    "dutPort2",
		Name:    "port2",
		IPv4:    "192.0.2.5",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:db8::192:0:2:5",
		IPv6Len: ipv6PrefixLen,
	}

	atePort2 = attrs.Attributes{
		Name:    "atePort2",
		MAC:     "02:00:01:01:01:02",
		IPv4:    "192.0.2.6",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:db8::192:0:2:6",
		IPv6Len: ipv6PrefixLen,
	}

	dutPort3 = attrs.Attributes{
		Desc:    "dutPort3",
		Name:    "port3",
		IPv4:    "192.0.2.9",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:db8::192:0:2:9",
		IPv6Len: ipv6PrefixLen,
	}

	atePort3 = attrs.Attributes{
		Name:    "atePort3",
		MAC:     "02:00:01:01:01:03",
		IPv4:    "192.0.2.10",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:db8::192:0:2:a",
		IPv6Len: ipv6PrefixLen,
	}

	dutPort4 = attrs.Attributes{
		Desc:    "dutPort4",
		Name:    "port4",
		IPv4:    "192.0.2.13",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:db8::192:0:2:d",
		IPv6Len: ipv6PrefixLen,
	}

	atePort4 = attrs.Attributes{
		Name:    "atePort4",
		MAC:     "02:00:01:01:01:04",
		IPv4:    "192.0.2.14",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:db8::192:0:2:e",
		IPv6Len: ipv6PrefixLen,
	}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

type ipAddr struct {
	address string
	prefix  uint32
}

func (ip *ipAddr) cidr(t *testing.T) string {
	_, net, err := net.ParseCIDR(fmt.Sprintf("%s/%d", ip.address, ip.prefix))
	if err != nil {
		t.Fatal(err)
	}
	return net.String()
}

type testData struct {
	dut            *ondatra.DUTDevice
	ate            *ondatra.ATEDevice
	top            gosnappi.Config
	otgP1          gosnappi.Device
	otgP2          gosnappi.Device
	otgP3          gosnappi.Device
	staticIPv4     ipAddr
	staticIPv6     ipAddr
	advertisedIPv4 ipAddr
	advertisedIPv6 ipAddr
}

func TestBasicStaticRouteSupport(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	configureDUT(t, dut)

	ate := ondatra.ATE(t, "ate")
	top := gosnappi.NewConfig()
	devs := configureOTG(t, ate, top)

	td := testData{
		dut:            dut,
		ate:            ate,
		top:            top,
		otgP1:          devs[0],
		otgP2:          devs[1],
		otgP3:          devs[2],
		staticIPv4:     ipAddr{address: v4Route, prefix: v4RoutePrefix},
		staticIPv6:     ipAddr{address: v6Route, prefix: v6RoutePrefix},
		advertisedIPv4: ipAddr{address: v4Route, prefix: v4RoutePrefix},
		advertisedIPv6: ipAddr{address: v6Route, prefix: v6RoutePrefix},
	}
	td.configureOTGFlows(t)
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	defer ate.OTG().StopProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv6")

	tcs := []struct {
		desc string
		fn   func(t *testing.T)
	}{
		{
			desc: "RT-1.26.1: Static Route ECMP",
			fn:   td.testStaticRouteECMP,
		},
		{
			desc: "RT-1.26.2: Static Route With Metric",
			fn:   td.testStaticRouteWithMetric,
		},
		{
			desc: "RT-1.26.3: Static Route With Preference",
			fn:   td.testStaticRouteWithPreference,
		},
		{
			desc: "RT-1.26.4: Static Route SetTag",
			fn:   td.testStaticRouteSetTag,
		},
		{
			desc: "RT-1.26.5 - Validate Cross-Address Family (XAF) Next-Hops",
			fn:   td.testCrossAddressFamilyNextHops,
		},
		{
			desc: "RT-1.26.6: Validate Static Route with DROP Next-Hop",
			fn:   td.testStaticRouteWithDropNextHop,
		},
		{
			desc: "RT-1.26.9 - Direct Interface IP Deletion (Negative)",
			fn:   td.testDirectInterfaceIPDeletion,
		},
		{
			desc: "RT-1.26.10 - Overlapping Prefixes / LPM (Corner)",
			fn:   td.testOverlappingPrefixesLPM,
		},
		{
			desc: "RT-1.26.11 - Route Resolution Loop (Negative)",
			fn:   td.testRouteResolutionLoop,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.desc, func(t *testing.T) {
			tc.fn(t)
		})
	}
}

// TestStaticRouteAddRemove validates dynamic hop expansions.
// Relevant README subtest: RT-1.26.8 - Validate Dynamic Add and Remove of Next-Hops
// Technical Explanation: Expands an existing static prefix dynamically up to 4 nexthops,
// then scales back down to 2, ensuring proper configuration bounds and system reactions.
func TestStaticRouteAddRemove(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	configureDUT(t, dut)

	ate := ondatra.ATE(t, "ate")
	top := gosnappi.NewConfig()
	configureOTG(t, ate, top)

	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	defer ate.OTG().StopProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")

	t.Run("RT-1.26.8 - Validate Dynamic Add and Remove of Next-Hops", func(t *testing.T) {
		// Step 1 - Configure one IPv4 static route with next-hops set to the IPv4
		// address of ATE port-2 (index 0) and port-3 (index 1).
		prefix := ipAddr{address: v4Route, prefix: v4RoutePrefix}
		t.Cleanup(func() {
			sp := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
			gnmi.Delete(t, dut, sp.Static(prefix.cidr(t)).Config())
		})
		b := &gnmi.SetBatch{}
		sV4 := &cfgplugins.StaticRouteCfg{
			NetworkInstance: deviations.DefaultNetworkInstance(dut),
			Prefix:          prefix.cidr(t),
			NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
				"0": oc.UnionString(atePort2.IPv4),
				"1": oc.UnionString(atePort3.IPv4),
			},
		}
		if _, err := cfgplugins.NewStaticRouteCfg(b, sV4, dut); err != nil {
			t.Fatalf("Failed to configure IPv4 static route: %v", err)
		}

		// Step 2 - Push configuration to DUT.
		b.Set(t, dut)
		validateStaticRoute(t, dut, prefix.cidr(t), sV4)

		// Step 3 - Update the static route by adding next-hops for ATE port-1 (index 2)
		// and port-4 (index 3).
		b = &gnmi.SetBatch{}
		sV4 = &cfgplugins.StaticRouteCfg{
			NetworkInstance: deviations.DefaultNetworkInstance(dut),
			Prefix:          prefix.cidr(t),
			NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
				"0": oc.UnionString(atePort2.IPv4),
				"1": oc.UnionString(atePort3.IPv4),
				"2": oc.UnionString(atePort1.IPv4),
				"3": oc.UnionString(atePort4.IPv4),
			},
		}
		if _, err := cfgplugins.NewStaticRouteCfg(b, sV4, dut); err != nil {
			t.Fatalf("Failed to configure IPv4 static route: %v", err)
		}

		// Step 4 - Push configuration to DUT.
		b.Set(t, dut)

		// Step 5 - Validate all four next-hops and indexes are reported correctly
		validateStaticRoute(t, dut, prefix.cidr(t), sV4)

		// Step 6 - Remove two next-hops (e.g., indexes 0 and 3).
		cfgplugins.DeleteStaticRouteNextHops(t, dut, deviations.DefaultNetworkInstance(dut), prefix.cidr(t), "0", "3")

		// Step 7 - Push configuration to DUT and validate that only two next-hops remain.
		sV4Removed := &cfgplugins.StaticRouteCfg{
			NetworkInstance: deviations.DefaultNetworkInstance(dut),
			Prefix:          prefix.cidr(t),
			NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
				"1": oc.UnionString(atePort3.IPv4),
				"2": oc.UnionString(atePort1.IPv4),
			},
		}
		validateStaticRoute(t, dut, prefix.cidr(t), sV4Removed)
	})
}

func validateStaticRoute(t *testing.T, dut *ondatra.DUTDevice, prefix string, sV4 *cfgplugins.StaticRouteCfg) {
	sp := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
	gnmi.Await(t, dut, sp.Static(prefix).Prefix().State(), 120*time.Second, prefix)

	if deviations.SkipStaticNexthopCheck(dut) {
		nexthops := gnmi.LookupAll(t, dut, sp.Static(prefix).NextHopAny().NextHop().State())
		if got, want := len(nexthops), len(sV4.NextHops); got != want {
			t.Errorf("Static route next hop count - %s: got: %v, want: %v", prefix, got, want)
		}
	} else {
		// Validate both the routes i.e. ipv4-route-[a|b] are configured and reported
		// correctly
		gotStatic := gnmi.Get(t, dut, sp.Static(prefix).State())
		if got, want := len(gotStatic.NextHop), len(sV4.NextHops); got != want {
			t.Errorf("Static route %s next hop count: got %d, want %d", prefix, got, want)
		}
		t.Logf("Static route %s: got: %v, want: %v", prefix, len(gotStatic.NextHop), len(sV4.NextHops))
		for index, nextHop := range gotStatic.NextHop {
			if got, want := nextHop.GetNextHop(), sV4.NextHops[index]; got != want {
				t.Errorf("Static route %s: got: %v, want: %v", prefix, got, want)
			}
		}
	}
}

func TestDisableRecursiveNextHopResolution(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	if deviations.UnsupportedStaticRouteNextHopRecurse(dut) {
		t.Skip("Skipping Disable Recursive Next Hop Resolution Test. Deviation UnsupportedStaticRouteNextHopRecurse enabled.")
	}
	configureDUT(t, dut)

	ate := ondatra.ATE(t, "ate")
	top := gosnappi.NewConfig()
	devs := configureOTG(t, ate, top)

	td := testData{
		dut:            dut,
		ate:            ate,
		top:            top,
		otgP1:          devs[0],
		otgP2:          devs[1],
		otgP3:          devs[2],
		staticIPv4:     ipAddr{address: v4Route, prefix: v4RoutePrefix},
		staticIPv6:     ipAddr{address: v6Route, prefix: v6RoutePrefix},
		advertisedIPv4: ipAddr{address: v4LoopbackRoute, prefix: v4LoopbackRoutePrefix},
		advertisedIPv6: ipAddr{address: v6LoopbackRoute, prefix: v6LoopbackRoutePrefix},
	}

	// Configure ipv4 and ipv6 ISIS between ATE port-1 <-> DUT port-1 and ATE
	// port-2 <-> DUT port2.
	// Configure one IPv4 /32 host route i.e. `ipv4-loopback = 198.51.100.100/32`
	// connected to ATE and advertised to DUT through both the IPv4 ISIS
	// adjacencies.
	// Configure one IPv6 /128 host route i.e. `ipv6-loopback =
	// 2001:db8::64:64::1/128` connected to ATE and advertised to DUT through both
	// the IPv6 ISIS adjacencies.
	td.advertiseRoutesWithISIS(t)
	td.configureOTGFlows(t)
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	defer ate.OTG().StopProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv6")

	if err := td.awaitISISAdjacency(t, dut.Port(t, "port1"), isisName); err != nil {
		t.Fatal(err)
	}
	if err := td.awaitISISAdjacency(t, dut.Port(t, "port2"), isisName); err != nil {
		t.Fatal(err)
	}

	t.Run("RT-1.26.7 - Validate Disabling Recursive Next-Hop Resolution", func(t *testing.T) {
		td.testRecursiveNextHopResolution(t)
		td.testRecursiveNextHopResolutionDisabled(t)
	})
}

// testRecursiveNextHopResolution tests basic recursive lookup.
// Relevant README subtest: RT-1.26.7 - Validate Disabling Recursive Next-Hop Resolution (Setup phase)
// Technical Explanation: Targets an indirect route advertised distantly (via IS-IS),
// validating whether the static table correctly recurses intermediate IPs prior to disabling it.
func (td *testData) testRecursiveNextHopResolution(t *testing.T) {
	b := &gnmi.SetBatch{}
	// Configure one IPv4 static route i.e. ipv4-route on the DUT for destination
	// `ipv4-network 203.0.113.0/24` with the next hop of `ipv4-loopback
	// 198.51.100.100/32`. Remove all other existing next hops for the route.
	sV4 := &cfgplugins.StaticRouteCfg{
		NetworkInstance: deviations.DefaultNetworkInstance(td.dut),
		Prefix:          td.staticIPv4.cidr(t),
		NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
			"0": oc.UnionString(td.advertisedIPv4.address),
		},
	}
	spV4, err := cfgplugins.NewStaticRouteCfg(b, sV4, td.dut)
	if err != nil {
		t.Fatal(err)
	}
	spV4.GetOrCreateNextHop("0").SetRecurse(true)
	// Configure one IPv6 static route i.e. ipv6-route on the DUT for destination
	// `ipv6-network 2001:db8:128:128::/64` with the next hop of `ipv6-loopback =
	// 2001:db8::64:64::1/128`. Remove all other existing next hops for the route.
	sV6 := &cfgplugins.StaticRouteCfg{
		NetworkInstance: deviations.DefaultNetworkInstance(td.dut),
		Prefix:          td.staticIPv6.cidr(t),
		NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
			"0": oc.UnionString(td.advertisedIPv6.address),
		},
	}
	spV6, err := cfgplugins.NewStaticRouteCfg(b, sV6, td.dut)
	if err != nil {
		t.Fatal(err)
	}
	spV6.GetOrCreateNextHop("0").SetRecurse(true)

	b.Set(t, td.dut)

	t.Run("Telemetry", func(t *testing.T) {
		sp := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(td.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(td.dut))

		_, ok := gnmi.Watch(t, td.dut, sp.Static(td.staticIPv4.cidr(t)).State(), time.Second*60, func(v *ygnmi.Value[*oc.NetworkInstance_Protocol_Static]) bool {
			val, present := v.Val()
			return present && val.GetPrefix() == td.staticIPv4.cidr(t)
		}).Await(t)
		if !ok {
			t.Errorf("IPv4 Static Route telemetry failed ")
		}
		_, ok = gnmi.Watch(t, td.dut, sp.Static(td.staticIPv6.cidr(t)).State(), time.Second*60, func(v *ygnmi.Value[*oc.NetworkInstance_Protocol_Static]) bool {
			val, present := v.Val()
			return present && val.GetPrefix() == td.staticIPv6.cidr(t)
		}).Await(t)
		if !ok {
			t.Errorf("IPv6 Static Route telemetry failed ")
		}

		gotStatic := gnmi.Get(t, td.dut, sp.Static(td.staticIPv4.cidr(t)).State())
		if got, want := gotStatic.GetNextHop("0").GetNextHop(), oc.UnionString(td.advertisedIPv4.address); got != want {
			t.Errorf("IPv4 Static Route next hop: got: %s, want: %s", got, want)
		}
		gotStatic = gnmi.Get(t, td.dut, sp.Static(td.staticIPv6.cidr(t)).State())
		if got, want := gotStatic.GetNextHop("0").GetNextHop(), oc.UnionString(td.advertisedIPv6.address); got != want {
			t.Errorf("IPv6 Static Route next hop: got: %s, want: %s", got, want)
		}
	})
	t.Run("Traffic", func(t *testing.T) {
		// Initiate traffic from ATE port-3 towards destination `ipv4-network
		// 203.0.113.0/24` and `ipv6-network 2024:db8:128:128::/64`
		td.ate.OTG().StartTraffic(t)
		time.Sleep(trafficDuration)
		td.ate.OTG().StopTraffic(t)

		lossV4 := otgutils.GetFlowLossPct(t, td.ate.OTG(), v4Flow, 20*time.Second)
		lossV6 := otgutils.GetFlowLossPct(t, td.ate.OTG(), v6Flow, 20*time.Second)

		// Validate that traffic is received from DUT (doesn't matter which port)
		otgutils.LogFlowMetrics(t, td.ate.OTG(), td.top)
		if lossV4 > lossTolerance {
			t.Errorf("Loss percent for IPv4 Traffic: got: %f, want 0%%", lossV4)
		}
		if lossV6 > lossTolerance {
			t.Errorf("Loss percent for IPv6 Traffic: got: %f, want 0%%", lossV6)
		}
	})
}

// testRecursiveNextHopResolutionDisabled tests disabling recursion.
// Relevant README subtest: RT-1.26.7 - Validate Disabling Recursive Next-Hop Resolution (Execution phase)
// Technical Explanation: Expliclity disables recursive resolution flags, proving
// the device immediately orphans distant IPs and halts traffic to intermediate hops.
func (td *testData) testRecursiveNextHopResolutionDisabled(t *testing.T) {
	sp := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(td.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(td.dut))
	// Disable static route next-hop recursive lookup (set to false)
	batch := &gnmi.SetBatch{}
	gnmi.BatchReplace(batch, sp.Static(td.staticIPv4.cidr(t)).NextHop("0").Recurse().Config(), false)
	gnmi.BatchReplace(batch, sp.Static(td.staticIPv6.cidr(t)).NextHop("0").Recurse().Config(), false)
	batch.Set(t, td.dut)

	t.Run("Telemetry", func(t *testing.T) {

		_, ok := gnmi.Watch(t, td.dut, sp.Static(td.staticIPv4.cidr(t)).State(), time.Second*30, func(v *ygnmi.Value[*oc.NetworkInstance_Protocol_Static]) bool {
			val, present := v.Val()
			return !present || (present && !val.GetNextHop("0").GetRecurse())
		}).Await(t)
		if !ok {
			t.Errorf("Unable to set recurse to false for v4 prefix")
		}

		_, ok = gnmi.Watch(t, td.dut, sp.Static(td.staticIPv6.cidr(t)).State(), time.Second*30, func(v *ygnmi.Value[*oc.NetworkInstance_Protocol_Static]) bool {
			val, present := v.Val()
			return !present || (present && !val.GetNextHop("0").GetRecurse())
		}).Await(t)
		if !ok {
			t.Errorf("Unable to set recurse to false for v6 prefix")
		}
	})
	t.Run("Traffic", func(t *testing.T) {
		// Initiate traffic from ATE port-3 towards destination `ipv4-network
		// 203.0.113.0/24` and `ipv6-network 2001:db8:128:128::/64`
		td.ate.OTG().StartTraffic(t)
		time.Sleep(trafficDuration)
		td.ate.OTG().StopTraffic(t)

		lossV4 := otgutils.GetFlowLossPct(t, td.ate.OTG(), v4Flow, 20*time.Second)
		lossV6 := otgutils.GetFlowLossPct(t, td.ate.OTG(), v6Flow, 20*time.Second)

		// Validate that traffic is NOT received from DUT
		otgutils.LogFlowMetrics(t, td.ate.OTG(), td.top)
		if got, want := lossV4, float64(100); got != want {
			t.Errorf("Loss percent for IPv4 Traffic: got: %f, want %f", got, want)
		}
		if got, want := lossV6, float64(100); got != want {
			t.Errorf("Loss percent for IPv6 Traffic: got: %f, want %f", got, want)
		}
	})
}

func (td *testData) configureStaticRouteToATEP1AndP2(t *testing.T) {
	b := &gnmi.SetBatch{}
	// Configure IPv4 static routes:
	//   *   Configure one IPv4 static route i.e. ipv4-route-a on the DUT for
	//       destination `ipv4-network 203.0.113.0/24` with the next hop set to the
	//       IPv4 address of ATE port-1
	//   *   Configure another IPv4 static route i.e. ipv4-route-b on the DUT for
	//       destination `ipv4-network 203.0.113.0/24` with the next hop set to the
	//       IPv4 address of ATE port-2
	sV4 := &cfgplugins.StaticRouteCfg{
		NetworkInstance: deviations.DefaultNetworkInstance(td.dut),
		Prefix:          td.staticIPv4.cidr(t),
		NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
			"0": oc.UnionString(atePort1.IPv4),
			"1": oc.UnionString(atePort2.IPv4),
		},
	}
	if _, err := cfgplugins.NewStaticRouteCfg(b, sV4, td.dut); err != nil {
		t.Fatalf("Failed to configure IPv4 static route: %v", err)
	}

	// Configure IPv6 static routes:
	//   *   Configure one IPv6 static route i.e. ipv6-route-a on the DUT for
	//       destination `ipv6-network 2001:db8:128:128::/64` with the next hop set
	//       to the IPv6 address of ATE port-1
	//   *   Configure another IPv6 static route i.e. ipv6-route-b on the DUT for
	//       destination `ipv6-network 2001:db8:128:128::/64` with the next hop set
	//       to the IPv6 address of ATE port-2
	sV6 := &cfgplugins.StaticRouteCfg{
		NetworkInstance: deviations.DefaultNetworkInstance(td.dut),
		Prefix:          td.staticIPv6.cidr(t),
		NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
			"0": oc.UnionString(atePort1.IPv6),
			"1": oc.UnionString(atePort2.IPv6),
		},
	}
	if _, err := cfgplugins.NewStaticRouteCfg(b, sV6, td.dut); err != nil {
		t.Fatalf("Failed to configure IPv6 static route: %v", err)
	}
	b.Set(t, td.dut)
}

func (td *testData) deleteStaticRoutes(t *testing.T) {
	b := &gnmi.SetBatch{}
	sp := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(td.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(td.dut))
	gnmi.BatchDelete(b, sp.Static(td.staticIPv4.cidr(t)).Config())
	gnmi.BatchDelete(b, sp.Static(td.staticIPv6.cidr(t)).Config())
	b.Set(t, td.dut)
}

// testStaticRouteECMP validates foundational ECMP behavior.
// Relevant README subtest: RT-1.26.1 - Validate Static Route ECMP
// Technical Explanation: Tests whether independent static route segments properly
// leverage multi-path execution by splitting packet transmissions evenly
// across connected egress endpoints (dutPort1, dutPort2).
func (td *testData) testStaticRouteECMP(t *testing.T) {
	td.configureStaticRouteToATEP1AndP2(t)
	defer td.deleteStaticRoutes(t)

	t.Run("Telemetry", func(t *testing.T) {

		sp := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(td.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(td.dut))
		gnmi.Await(t, td.dut, sp.Static(td.staticIPv4.cidr(t)).Prefix().State(), 120*time.Second, td.staticIPv4.cidr(t))
		gnmi.Await(t, td.dut, sp.Static(td.staticIPv6.cidr(t)).Prefix().State(), 120*time.Second, td.staticIPv6.cidr(t))

		if deviations.SkipStaticNexthopCheck(td.dut) {
			nexthops := gnmi.LookupAll(t, td.dut, sp.Static(td.staticIPv4.cidr(t)).NextHopAny().NextHop().State())
			if len(nexthops) != 2 {
				t.Errorf("IPv4 Static Route next hop: want %d nexthops,got %d nexthops", 2, len(nexthops))
			}
			for _, nexthop := range nexthops {
				if got, ok := nexthop.Val(); !ok || (got != oc.UnionString(atePort1.IPv4) && got != oc.UnionString(atePort2.IPv4)) {
					t.Errorf("IPv4 Static Route next hop:got %s,want %s or %s", got, oc.UnionString(atePort1.IPv4), oc.UnionString(atePort2.IPv4))
				}
			}
			nexthops = gnmi.LookupAll(t, td.dut, sp.Static(td.staticIPv6.cidr(t)).NextHopAny().NextHop().State())
			if len(nexthops) != 2 {
				t.Errorf("IPv6 Static Route next hop: want %d nexthops,got %d nexthops", 2, len(nexthops))
			}
			for _, nexthop := range nexthops {
				if got, ok := nexthop.Val(); !ok || (got != oc.UnionString(atePort1.IPv6) && got != oc.UnionString(atePort2.IPv6)) {
					t.Errorf("IPv6 Static Route next hop: got %s,want %s or %s", got, oc.UnionString(atePort1.IPv6), oc.UnionString(atePort2.IPv6))
				}
			}
		} else {
			// Validate both the routes i.e. ipv4-route-[a|b] are configured and reported
			// correctly
			gotStatic := gnmi.Get(t, td.dut, sp.Static(td.staticIPv4.cidr(t)).State())
			if got, want := gotStatic.GetNextHop("0").GetNextHop(), oc.UnionString(atePort1.IPv4); got != want {
				t.Errorf("IPv4 Static Route next hop: got: %s, want: %s", got, want)
			}
			if got, want := gotStatic.GetNextHop("1").GetNextHop(), oc.UnionString(atePort2.IPv4); got != want {
				t.Errorf("IPv4 Static Route next hop: got: %s, want: %s", got, want)
			}
			// Validate both the routes i.e. ipv6-route-[a|b] are configured and reported
			// correctly
			gotStatic = gnmi.Get(t, td.dut, sp.Static(td.staticIPv6.cidr(t)).State())
			if got, want := gotStatic.GetNextHop("0").GetNextHop(), oc.UnionString(atePort1.IPv6); got != want {
				t.Errorf("IPv6 Static Route next hop: got: %s, want: %s", got, want)
			}
			if got, want := gotStatic.GetNextHop("1").GetNextHop(), oc.UnionString(atePort2.IPv6); got != want {
				t.Errorf("IPv6 Static Route next hop: got: %s, want: %s", got, want)
			}
		}
	})

	t.Run("Traffic", func(t *testing.T) {
		// Initiate traffic from ATE port-3 towards destination `ipv4-network
		// 203.0.113.0/24` and `ipv6-network 2001:db8:128:128::/64`
		td.ate.OTG().StartTraffic(t)
		time.Sleep(trafficDuration)
		td.ate.OTG().StopTraffic(t)

		lossV4 := otgutils.GetFlowLossPct(t, td.ate.OTG(), v4Flow, 20*time.Second)
		lossV6 := otgutils.GetFlowLossPct(t, td.ate.OTG(), v6Flow, 20*time.Second)

		otgutils.LogFlowMetrics(t, td.ate.OTG(), td.top)
		if lossV4 > lossTolerance {
			t.Errorf("Loss percent for IPv4 Traffic: got: %f, want 0%%", lossV4)
		}
		if lossV6 > lossTolerance {
			t.Errorf("Loss percent for IPv6 Traffic: got: %f, want 0%%", lossV6)
		}

		portCounters := egressTrackingCounters(t, td.ate, v4Flow)
		if len(portCounters) != 2 {
			t.Errorf("IPv4 egress tracking counters: got: %v, want: 2", len(portCounters))
		}
		p1Counter, ok := portCounters[port1Tag]
		if !ok {
			t.Errorf("Port1 IPv4 egress tracking counter not found: %v", portCounters)
		}
		p2Counter, ok := portCounters[port2Tag]
		if !ok {
			t.Errorf("Port2 IPv4 egress tracking counter not found: %v", portCounters)
		}
		// Validate that traffic is received from DUT on both port-1 and port-2 and
		// ECMP works
		if got, want := p1Counter*100/(p1Counter+p2Counter), uint64(50); got < want-ecmpTolerance || got > want+ecmpTolerance {
			t.Errorf("ECMP IPv4 load balance error for port1, got: %v, want: %v", got, want)
		}
		if got, want := p2Counter*100/(p1Counter+p2Counter), uint64(50); got < want-ecmpTolerance || got > want+ecmpTolerance {
			t.Errorf("ECMP IPv4 load balance error for port2, got: %v, want: %v", got, want)
		}

		portCounters = egressTrackingCounters(t, td.ate, v6Flow)
		if len(portCounters) != 2 {
			t.Errorf("IPv6 egress tracking counters: got: %v, want: 2", len(portCounters))
		}
		p1Counter, ok = portCounters[port1Tag]
		if !ok {
			t.Errorf("Port1 IPv6 egress tracking counter not found: %v", portCounters)
		}
		p2Counter, ok = portCounters[port2Tag]
		if !ok {
			t.Errorf("Port2 IPv6 egress tracking counter not found: %v", portCounters)
		}
		// Validate that traffic is received from DUT on both port-1 and port-2 and
		// ECMP works
		if got, want := p1Counter*100/(p1Counter+p2Counter), uint64(50); got < want-ecmpTolerance || got > want+ecmpTolerance {
			t.Errorf("ECMP IPv6 load balance error for port1, got: %v, want: %v", got, want)
		}
		if got, want := p2Counter*100/(p1Counter+p2Counter), uint64(50); got < want-ecmpTolerance || got > want+ecmpTolerance {
			t.Errorf("ECMP IPv6 load balance error for port2, got: %v, want: %v", got, want)
		}
	})
}

// testStaticRouteWithMetric validates route selection based on metric.
// Relevant README subtest: RT-1.26.2 - Validate Static Route Metric
// Technical Explanation: Modifies the static route's metric, asserting that
// the control plane prefers the next hop with the lower metric (higher priority)
// during forwarding resolution.
func (td *testData) testStaticRouteWithMetric(t *testing.T) {
	td.configureStaticRouteToATEP1AndP2(t)
	defer td.deleteStaticRoutes(t)

	var port2Metric = uint32(100)
	sp := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(td.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(td.dut))

	// Configure metric of ipv4-route-b and ipv6-route-b to 100
	batch := &gnmi.SetBatch{}
	if deviations.StaticRouteWithExplicitMetric(td.dut) {
		// per the cisco specifications setting the metric is equivlent to setting the weight, so in this case
		// we want the majority of the traffic to go over port 1 so setting the metric to 100 and port 2 as 1
		var port1Metric = uint32(100)
		port2Metric = uint32(1)
		gnmi.BatchReplace(batch, sp.Static(td.staticIPv4.cidr(t)).NextHop("0").Metric().Config(), port1Metric)
		gnmi.BatchReplace(batch, sp.Static(td.staticIPv6.cidr(t)).NextHop("0").Metric().Config(), port1Metric)

	}

	gnmi.BatchReplace(batch, sp.Static(td.staticIPv4.cidr(t)).NextHop("1").Metric().Config(), port2Metric)
	gnmi.BatchReplace(batch, sp.Static(td.staticIPv6.cidr(t)).NextHop("1").Metric().Config(), port2Metric)
	batch.Set(t, td.dut)

	t.Run("Telemetry", func(t *testing.T) {
		if deviations.TelemetryNotSupportedForLowPriorityNh(td.dut) {
			t.Skip("Skipping Telemetry check for Metric, since deviation MissingStaticRouteNextHopMetricTelemetry is enabled.")
		}
		gnmi.Await(t, td.dut, sp.Static(td.staticIPv4.cidr(t)).Prefix().State(), 30*time.Second, td.staticIPv4.cidr(t))
		gnmi.Await(t, td.dut, sp.Static(td.staticIPv6.cidr(t)).Prefix().State(), 30*time.Second, td.staticIPv6.cidr(t))
		// Validate that the metric is set correctly
		if got, want := gnmi.Get(t, td.dut, sp.Static(td.staticIPv4.cidr(t)).NextHop("1").Metric().State()), port2Metric; got != want {
			t.Errorf("IPv4 Static Route metric for NextHop 1, got: %d, want: %d", got, want)
		}
		if got, want := gnmi.Get(t, td.dut, sp.Static(td.staticIPv6.cidr(t)).NextHop("1").Metric().State()), port2Metric; got != want {
			t.Errorf("IPv6 Static Route metric for NextHop 1, got: %d, want: %d", got, want)
		}
	})

	t.Run("Traffic", func(t *testing.T) {
		// Initiate traffic from ATE port-3 towards destination `ipv4-network
		// 203.0.113.0/24` and `ipv6-network 2001:db8:128:128::/64`
		td.ate.OTG().StartTraffic(t)
		time.Sleep(trafficDuration)
		td.ate.OTG().StopTraffic(t)

		lossV4 := otgutils.GetFlowLossPct(t, td.ate.OTG(), v4Flow, 20*time.Second)
		lossV6 := otgutils.GetFlowLossPct(t, td.ate.OTG(), v6Flow, 20*time.Second)

		otgutils.LogFlowMetrics(t, td.ate.OTG(), td.top)
		if lossV4 > lossTolerance {
			t.Errorf("Loss percent for IPv4 Traffic: got: %f, want 0%%", lossV4)
		}
		if lossV6 > lossTolerance {
			t.Errorf("Loss percent for IPv6 Traffic: got: %f, want 0%%", lossV6)
		}
		// Validate that traffic is received from DUT on port-1 and not on port-2
		portCounters := egressTrackingCounters(t, td.ate, v4Flow)
		_, rxV4 := otgutils.GetFlowStats(t, td.ate.OTG(), v4Flow, 20*time.Second)
		port1Counter, ok := portCounters[port1Tag]
		if !ok {
			t.Errorf("Port1 IPv4 egress tracking counter not found: %v", portCounters)
		}

		if deviations.StaticRouteWithExplicitMetric(td.dut) {
			// validate traffic
			got, want := float64(port1Counter)*100/float64(rxV4), float64(100)
			expectedMinTraffic := want * (1 - explicitMetricTolerance/100)
			if got < expectedMinTraffic {
				t.Errorf("IPv4 traffic on port1, got: %v%%, expected to be at least %v%%", got, expectedMinTraffic)
			}
		} else {
			// validate traffic default behavior
			if got, want := float64(port1Counter)*100/float64(rxV4), float64(100); got+lossTolerance < want {
				t.Errorf("IPv4 traffic on port1, got: %v, want: %v", got, want)
			}
		}

		// Validate that traffic is received from DUT on port-1 and not on port-2
		portCounters = egressTrackingCounters(t, td.ate, v6Flow)
		_, rxV6 := otgutils.GetFlowStats(t, td.ate.OTG(), v6Flow, 20*time.Second)
		port1Counter, ok = portCounters[port1Tag]
		if !ok {
			t.Errorf("Port1 IPv6 egress tracking counter not found: %v", portCounters)
		}
		if deviations.StaticRouteWithExplicitMetric(td.dut) {
			// validate traffic
			got, want := float64(port1Counter)*100/float64(rxV6), float64(100)
			expectedMinTraffic := want * (1 - explicitMetricTolerance/100)
			if got < expectedMinTraffic {
				t.Errorf("IPv6 traffic on port1, got: %v%%, expected to be at least %v%%", got, expectedMinTraffic)
			}

		} else {
			// validate traffic default behavior
			if got, want := float64(port1Counter)*100/float64(rxV6), float64(100); got+lossTolerance < want {
				t.Errorf("IPv6 traffic on port1, got: %v, want: %v", got, want)
			}
		}

	})
}

// testStaticRouteWithPreference validates route administrative distance.
// Relevant README subtest: RT-1.26.3 - Validate Static Route Preference
// Technical Explanation: Adjusts the AD/preference of a route next hop,
// validating that lower preference values correctly override identical prefixes
// with higher values during LPM routing tiebreakers.
func (td *testData) testStaticRouteWithPreference(t *testing.T) {
	td.configureStaticRouteToATEP1AndP2(t)
	defer td.deleteStaticRoutes(t)

	const port1Preference = uint32(50)
	const port2Metric = uint32(100)

	sp := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(td.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(td.dut))

	// Configure metric of ipv4-route-b and ipv6-route-b to 100
	batch := &gnmi.SetBatch{}
	gnmi.BatchReplace(batch, sp.Static(td.staticIPv4.cidr(t)).NextHop("1").Metric().Config(), port2Metric)
	gnmi.BatchReplace(batch, sp.Static(td.staticIPv6.cidr(t)).NextHop("1").Metric().Config(), port2Metric)

	// Configure preference of ipv4-route-a and ipv6-route-a to 50
	if deviations.SetMetricAsPreference(td.dut) {
		// Lower metric indicate more favourable path.
		// If we use Metric instead of Preference, we would need to have a port1Metric
		// larger than port2Metric for traffic to pass through port 2
		port1Metric := port2Metric + port1Preference
		gnmi.BatchReplace(batch, sp.Static(td.staticIPv4.cidr(t)).NextHop("0").Metric().Config(), port1Metric)
		gnmi.BatchReplace(batch, sp.Static(td.staticIPv6.cidr(t)).NextHop("0").Metric().Config(), port1Metric)
	} else {
		gnmi.BatchReplace(batch, sp.Static(td.staticIPv4.cidr(t)).NextHop("0").Preference().Config(), port1Preference)
		gnmi.BatchReplace(batch, sp.Static(td.staticIPv6.cidr(t)).NextHop("0").Preference().Config(), port1Preference)
	}
	batch.Set(t, td.dut)

	t.Run("Telemetry", func(t *testing.T) {
		if deviations.SetMetricAsPreference(td.dut) || deviations.TelemetryNotSupportedForLowPriorityNh(td.dut) {
			t.Skip("Skipping Preference telemetry check since deviation SetMetricAsPreference is enabled")
		}
		gnmi.Await(t, td.dut, sp.Static(td.staticIPv4.cidr(t)).Prefix().State(), 30*time.Second, td.staticIPv4.cidr(t))
		gnmi.Await(t, td.dut, sp.Static(td.staticIPv6.cidr(t)).Prefix().State(), 30*time.Second, td.staticIPv6.cidr(t))
		// Validate that the preference is set correctly
		if deviations.SkipStaticNexthopCheck(td.dut) {
			gotStatic := gnmi.Get(t, td.dut, sp.Static(td.staticIPv4.cidr(t)).State())
			indexes := gnmi.LookupAll(t, td.dut, sp.Static(td.staticIPv4.cidr(t)).NextHopAny().Index().State())
			for _, index := range indexes {
				if val, ok := index.Val(); ok {
					if gotStatic.GetNextHop(val).GetNextHop() == oc.UnionString(atePort1.IPv4) {
						if got, want := gotStatic.GetNextHop(val).GetPreference(), port1Preference; got != want {
							t.Errorf("IPv4 Static Route preference for port1: got: %d, want: %d", got, want)
						}
					}
				} else {
					t.Errorf("Unable to fetch nexthop index")
				}
			}
			gotStatic = gnmi.Get(t, td.dut, sp.Static(td.staticIPv6.cidr(t)).State())
			indexes = gnmi.LookupAll(t, td.dut, sp.Static(td.staticIPv6.cidr(t)).NextHopAny().Index().State())
			for _, index := range indexes {
				if val, ok := index.Val(); ok {
					if gotStatic.GetNextHop(val).GetNextHop() == oc.UnionString(atePort1.IPv6) {
						if got, want := gotStatic.GetNextHop(val).GetPreference(), port1Preference; got != want {
							t.Errorf("IPv6 Static Route preference for port1: got: %d, want: %d", got, want)
						}
					}
				} else {
					t.Errorf("Unable to fetch nexthop index")
				}
			}
		} else {
			if got, want := gnmi.Get(t, td.dut, sp.Static(td.staticIPv4.cidr(t)).NextHop("0").Preference().State()), port1Preference; got != want {
				t.Errorf("IPv4 Static Route preference for NextHop 0, got: %d, want: %d", got, want)
			}
			if got, want := gnmi.Get(t, td.dut, sp.Static(td.staticIPv6.cidr(t)).NextHop("0").Preference().State()), port1Preference; got != want {
				t.Errorf("IPv6 Static Route preference for NextHop 0, got: %d, want: %d", got, want)
			}
		}
	})

	t.Run("Traffic", func(t *testing.T) {
		// Initiate traffic from ATE port-3 towards destination `ipv4-network
		// 203.0.113.0/24` and `ipv6-network 2001:db8:128:128::/64`
		td.ate.OTG().StartTraffic(t)
		time.Sleep(trafficDuration)
		td.ate.OTG().StopTraffic(t)

		lossV4 := otgutils.GetFlowLossPct(t, td.ate.OTG(), v4Flow, 20*time.Second)
		lossV6 := otgutils.GetFlowLossPct(t, td.ate.OTG(), v6Flow, 20*time.Second)

		otgutils.LogFlowMetrics(t, td.ate.OTG(), td.top)
		if lossV4 > lossTolerance {
			t.Errorf("Loss percent for IPv4 Traffic: got: %f, want 0%%", lossV4)
		}
		if lossV6 > lossTolerance {
			t.Errorf("Loss percent for IPv6 Traffic: got: %f, want 0%%", lossV6)
		}
		// Validate that traffic is now received from DUT on port-2 and not on port-1
		portCounters := egressTrackingCounters(t, td.ate, v4Flow)
		_, rxV4 := otgutils.GetFlowStats(t, td.ate.OTG(), v4Flow, 20*time.Second)
		port2Counter, ok := portCounters[port2Tag]
		if !ok {
			t.Errorf("Port2 IPv4 egress tracking counter not found: %v", portCounters)
		}
		if got, want := float64(port2Counter)*100/float64(rxV4), float64(100); got+lossTolerance < want {
			t.Errorf("IPv4 traffic on port2, got: %v, want: %v", got, want)
		}
		// Validate that traffic is now received from DUT on port-2 and not on port-1
		portCounters = egressTrackingCounters(t, td.ate, v6Flow)
		_, rxV6 := otgutils.GetFlowStats(t, td.ate.OTG(), v6Flow, 20*time.Second)
		port2Counter, ok = portCounters[port2Tag]
		if !ok {
			t.Errorf("Port2 IPv6 egress tracking counter not found: %v", portCounters)
		}
		if got, want := float64(port2Counter)*100/float64(rxV6), float64(100); got+lossTolerance < want {
			t.Errorf("IPv6 traffic on port2, got: %v, want: %v", got, want)
		}
	})
}

// testStaticRouteSetTag validates static route tagging features.
// Relevant README subtest: RT-1.26.4 - Validate Static Route Tag
// Technical Explanation: Pushes an arbitrary route tag to the control plane,
// retrieving it via gNMI state paths to confirm attributes are retained.
func (td *testData) testStaticRouteSetTag(t *testing.T) {
	const tag = uint32(10)

	b := &gnmi.SetBatch{}
	// Configure a tag of value 10 on ipv4 and ipv6 static routes
	v4Cfg := &cfgplugins.StaticRouteCfg{
		NetworkInstance: deviations.DefaultNetworkInstance(td.dut),
		Prefix:          td.staticIPv4.cidr(t),
		NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
			"0": oc.UnionString(atePort1.IPv4),
			"1": oc.UnionString(atePort2.IPv4),
		},
	}
	sV4, err := cfgplugins.NewStaticRouteCfg(b, v4Cfg, td.dut)
	if err != nil {
		t.Fatalf("Failed to configure IPv4 static route: %v", err)
	}
	sV4.SetTag, _ = sV4.To_NetworkInstance_Protocol_Static_SetTag_Union(tag)

	v6Cfg := &cfgplugins.StaticRouteCfg{
		NetworkInstance: deviations.DefaultNetworkInstance(td.dut),
		Prefix:          td.staticIPv6.cidr(t),
		NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
			"0": oc.UnionString(atePort1.IPv6),
			"1": oc.UnionString(atePort2.IPv6),
		},
	}
	sV6, err := cfgplugins.NewStaticRouteCfg(b, v6Cfg, td.dut)
	if err != nil {
		t.Fatalf("Failed to configure IPv6 static route: %v", err)
	}
	sV6.SetTag, _ = sV6.To_NetworkInstance_Protocol_Static_SetTag_Union(tag)

	b.Set(t, td.dut)

	defer td.deleteStaticRoutes(t)

	// Validate the tag is set
	t.Run("Telemetry", func(t *testing.T) {
		sp := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(td.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(td.dut))
		gnmi.Await(t, td.dut, sp.Static(td.staticIPv4.cidr(t)).Prefix().State(), 30*time.Second, td.staticIPv4.cidr(t))
		gnmi.Await(t, td.dut, sp.Static(td.staticIPv6.cidr(t)).Prefix().State(), 30*time.Second, td.staticIPv6.cidr(t))
		if got, want := gnmi.Get(t, td.dut, sp.Static(td.staticIPv4.cidr(t)).SetTag().State()), oc.UnionUint32(tag); got != want {
			t.Errorf("IPv4 Static Route SetTag, got: %d, want: %d", got, want)
		}
		if got, want := gnmi.Get(t, td.dut, sp.Static(td.staticIPv6.cidr(t)).SetTag().State()), oc.UnionUint32(tag); got != want {
			t.Errorf("IPv6 Static Route SetTag, got: %d, want: %d", got, want)
		}
	})
}

func staticARPWithMagicUniversalIP(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")
	dummyIPCIDR := dummyV6 + "/128"
	s2 := &oc.NetworkInstance_Protocol_Static{
		Prefix: ygot.String(dummyIPCIDR),
		NextHop: map[string]*oc.NetworkInstance_Protocol_Static_NextHop{
			"0": {
				Index: ygot.String("0"),
				InterfaceRef: &oc.NetworkInstance_Protocol_Static_NextHop_InterfaceRef{
					Interface: ygot.String(p1.Name()),
				},
			},
			"1": {
				Index: ygot.String("1"),
				InterfaceRef: &oc.NetworkInstance_Protocol_Static_NextHop_InterfaceRef{
					Interface: ygot.String(p2.Name()),
				},
			},
		},
	}
	sp := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
	static, ok := gnmi.LookupConfig(t, dut, sp.Config()).Val()
	if !ok || static == nil {
		static = &oc.NetworkInstance_Protocol{
			Identifier: oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC,
			Name:       ygot.String(deviations.StaticProtocolName(dut)),
			Static: map[string]*oc.NetworkInstance_Protocol_Static{
				dummyIPCIDR: s2,
			},
		}
		gnmi.Replace(t, dut, sp.Config(), static)
	} else {
		gnmi.Replace(t, dut, sp.Static(dummyIPCIDR).Config(), s2)
	}
}

// testStaticRouteWithDropNextHop validates blackholing invalid routes.
// Relevant README subtest: RT-1.26.6 - Validate Static Route with DROP Next-Hop
// Technical Explanation: Maps generic subnets targeting an explicit DROP local-defined
// next-hop, measuring line loss rates to confirm packets are deliberately blackholed.
func (td *testData) testStaticRouteWithDropNextHop(t *testing.T) {
	if deviations.StaticRouteWithDropNhUnsupported(td.dut) {
		t.Skip("Skipping test static route with drop nexthop. Deviation StaticRouteWithDropNhUnsupported enabled.")
	}
	b := &gnmi.SetBatch{}
	// Configure IPv4 static routes:
	//   *   Configure one IPv4 static route i.e. ipv4-route-a on the DUT for
	//       destination `ipv4-network 203.0.113.0/24` with the next hop set to DROP
	//       local-defined next hop
	sV4 := &cfgplugins.StaticRouteCfg{
		NetworkInstance: deviations.DefaultNetworkInstance(td.dut),
		Prefix:          td.staticIPv4.cidr(t),
		NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
			"0": oc.LocalRouting_LOCAL_DEFINED_NEXT_HOP_DROP,
		},
	}
	if _, err := cfgplugins.NewStaticRouteCfg(b, sV4, td.dut); err != nil {
		t.Fatalf("Failed to configure IPv4 static route: %v", err)
	}

	// Configure IPv6 static routes:
	//   *   Configure one IPv6 static route i.e. ipv6-route-a on the DUT for
	//       destination `ipv6-network 2001:db8:128:128::/64` with the next hop set
	//       to DROP local-defined next hop
	sV6 := &cfgplugins.StaticRouteCfg{
		NetworkInstance: deviations.DefaultNetworkInstance(td.dut),
		Prefix:          td.staticIPv6.cidr(t),
		NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
			"0": oc.LocalRouting_LOCAL_DEFINED_NEXT_HOP_DROP,
		},
	}
	if _, err := cfgplugins.NewStaticRouteCfg(b, sV6, td.dut); err != nil {
		t.Fatalf("Failed to configure IPv6 static route: %v", err)
	}
	b.Set(t, td.dut)

	defer td.deleteStaticRoutes(t)

	t.Run("Telemetry", func(t *testing.T) {
		if deviations.MissingStaticRouteDropNextHopTelemetry(td.dut) {
			t.Skip("Skipping telemetry check for DROP next hop. Deviation MissingStaticRouteDropNextHopTelemetryenabled.")
		}
		sp := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(td.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(td.dut))
		gnmi.Await(t, td.dut, sp.Static(td.staticIPv4.cidr(t)).Prefix().State(), 30*time.Second, td.staticIPv4.cidr(t))
		gnmi.Await(t, td.dut, sp.Static(td.staticIPv6.cidr(t)).Prefix().State(), 30*time.Second, td.staticIPv6.cidr(t))

		// Validate the route is configured and reported correctly
		gotStatic := gnmi.Get(t, td.dut, sp.Static(td.staticIPv4.cidr(t)).State())
		if got, want := gotStatic.GetNextHop("0").GetNextHop(), oc.LocalRouting_LOCAL_DEFINED_NEXT_HOP_DROP; got != want {
			t.Errorf("IPv4 Static Route next hop: got: %s, want: %s", got, want)
		}
		// Validate the route is configured and reported correctly
		gotStatic = gnmi.Get(t, td.dut, sp.Static(td.staticIPv6.cidr(t)).State())
		if got, want := gotStatic.GetNextHop("0").GetNextHop(), oc.LocalRouting_LOCAL_DEFINED_NEXT_HOP_DROP; got != want {
			t.Errorf("IPv6 Static Route next hop: got: %s, want: %s", got, want)
		}
	})

	t.Run("Traffic", func(t *testing.T) {
		// Initiate traffic from ATE port-3 towards destination `ipv4-network
		// 203.0.113.0/24` and `ipv6-network 2001:db8:128:128::/64`
		td.ate.OTG().StartTraffic(t)
		time.Sleep(trafficDuration)
		td.ate.OTG().StopTraffic(t)

		lossV4 := otgutils.GetFlowLossPct(t, td.ate.OTG(), v4Flow, 20*time.Second)
		lossV6 := otgutils.GetFlowLossPct(t, td.ate.OTG(), v6Flow, 20*time.Second)

		// Validate that traffic is dropped on DUT and not received on port-1 and
		// port-2
		otgutils.LogFlowMetrics(t, td.ate.OTG(), td.top)
		if lossV4 != 100 {
			t.Errorf("Loss percent for IPv4 Traffic: got: %f, want 100%%", lossV4)
		}
		if lossV6 != 100 {
			t.Errorf("Loss percent for IPv6 Traffic: got: %f, want 100%%", lossV6)
		}
	})
}

func egressTrackingCounters(t *testing.T, ate *ondatra.ATEDevice, flow string) map[string]uint64 {
	t.Helper()
	etTags := gnmi.GetAll(t, ate.OTG(), gnmi.OTG().Flow(flow).TaggedMetricAny().State())
	inPkts := map[string]uint64{}
	for _, tags := range etTags {
		for _, tag := range tags.Tags {
			inPkts[tag.GetTagValue().GetValueAsHex()] = tags.GetCounters().GetInPkts()
		}
	}
	return inPkts
}

func (td *testData) configureOTGFlows(t *testing.T) {
	t.Helper()

	srcV4 := td.otgP3.Ethernets().Items()[0].Ipv4Addresses().Items()[0]
	srcV6 := td.otgP3.Ethernets().Items()[0].Ipv6Addresses().Items()[0]

	dst1V4 := td.otgP1.Ethernets().Items()[0].Ipv4Addresses().Items()[0]
	dst1V6 := td.otgP1.Ethernets().Items()[0].Ipv6Addresses().Items()[0]
	dst2V4 := td.otgP2.Ethernets().Items()[0].Ipv4Addresses().Items()[0]
	dst2V6 := td.otgP2.Ethernets().Items()[0].Ipv6Addresses().Items()[0]

	v4F := td.top.Flows().Add()
	v4F.SetName(v4Flow).Metrics().SetEnable(true)
	v4F.TxRx().Device().SetTxNames([]string{srcV4.Name()}).SetRxNames([]string{dst1V4.Name(), dst2V4.Name()})

	v4FEth := v4F.Packet().Add().Ethernet()
	v4FEth.Src().SetValue(atePort3.MAC)

	v4FIp := v4F.Packet().Add().Ipv4()
	v4FIp.Src().SetValue(srcV4.Address())
	v4FIp.Dst().Increment().SetStart(v4TrafficStart).SetCount(254)

	udp := v4F.Packet().Add().Udp()
	udp.DstPort().Increment().SetStart(1).SetCount(500).SetStep(1)
	udp.SrcPort().Increment().SetStart(1).SetCount(500).SetStep(1)

	eth := v4F.EgressPacket().Add().Ethernet()
	ethTag := eth.Dst().MetricTags().Add()
	ethTag.SetName("MACTrackingv4").SetOffset(40).SetLength(8)

	v6F := td.top.Flows().Add()
	v6F.SetName(v6Flow).Metrics().SetEnable(true)
	v6F.TxRx().Device().SetTxNames([]string{srcV6.Name()}).SetRxNames([]string{dst1V6.Name(), dst2V6.Name()})

	v6FEth := v6F.Packet().Add().Ethernet()
	v6FEth.Src().SetValue(atePort3.MAC)

	v6FIP := v6F.Packet().Add().Ipv6()
	v6FIP.Src().SetValue(srcV6.Address())
	v6FIP.Dst().Increment().SetStart(v6TrafficStart).SetCount(254)

	udp = v6F.Packet().Add().Udp()
	udp.DstPort().Increment().SetStart(1).SetCount(500).SetStep(1)
	udp.SrcPort().Increment().SetStart(1).SetCount(500).SetStep(1)

	eth = v6F.EgressPacket().Add().Ethernet()
	ethTag = eth.Dst().MetricTags().Add()
	ethTag.SetName("MACTrackingv6").SetOffset(40).SetLength(8)
}

func (td *testData) awaitISISAdjacency(t *testing.T, p *ondatra.Port, isisName string) error {
	t.Helper()
	isis := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(td.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisName).Isis()
	intf := isis.Interface(p.Name())
	if deviations.ExplicitInterfaceInDefaultVRF(td.dut) || deviations.InterfaceRefInterfaceIDFormat(td.dut) {
		intf = isis.Interface(p.Name() + ".0")
	}
	query := intf.Level(2).AdjacencyAny().AdjacencyState().State()
	_, ok := gnmi.WatchAll(t, td.dut, query, time.Minute, func(v *ygnmi.Value[oc.E_Isis_IsisInterfaceAdjState]) bool {
		state, _ := v.Val()
		return v.IsPresent() && state == oc.Isis_IsisInterfaceAdjState_UP
	}).Await(t)

	if !ok {
		return fmt.Errorf("timeout - waiting for adjacency state")
	}
	return nil
}

func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	for _, dutPorts := range []*attrs.Attributes{&dutPort1, &dutPort2, &dutPort3, &dutPort4} {
		dutPort := dut.Port(t, dutPorts.Name)
		dutInt := dutPorts.NewOCInterface(dutPort.Name(), dut)
		if deviations.FrBreakoutFix(dut) && dutPort.PMD() == ondatra.PMD100GBASEFR {
			ethPort := dutInt.GetOrCreateEthernet()
			ethPort.SetAutoNegotiate(false)
			ethPort.SetDuplexMode(oc.Ethernet_DuplexMode_FULL)
			ethPort.SetPortSpeed(oc.IfEthernet_ETHERNET_SPEED_SPEED_100GB)
		}
		if deviations.IPv6StaticRouteWithIPv4NextHopRequiresStaticARP(dut) {
			dutInt.GetOrCreateSubinterface(0).GetOrCreateIpv6().GetOrCreateNeighbor(dummyV6).LinkLayerAddress = ygot.String(dummyMAC)
		}
		gnmi.Replace(t, dut, gnmi.OC().Interface(dutPort.Name()).Config(), dutInt)
		if deviations.ExplicitInterfaceInDefaultVRF(dut) {
			fptest.AssignToNetworkInstance(t, dut, dutPort.Name(), deviations.DefaultNetworkInstance(dut), 0)
		}
	}
}

func configureOTG(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config) []gosnappi.Device {
	t.Helper()
	p1 := ate.Port(t, "port1")
	p2 := ate.Port(t, "port2")
	p3 := ate.Port(t, "port3")
	p4 := ate.Port(t, "port4")

	d1 := atePort1.AddToOTG(top, p1, &dutPort1)
	d2 := atePort2.AddToOTG(top, p2, &dutPort2)
	d3 := atePort3.AddToOTG(top, p3, &dutPort3)
	d4 := atePort4.AddToOTG(top, p4, &dutPort4)
	return []gosnappi.Device{d1, d2, d3, d4}
}

func (td *testData) advertiseRoutesWithISIS(t *testing.T) {
	t.Helper()

	root := &oc.Root{}
	ni := root.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(td.dut))
	isisP := ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisName)
	isisP.SetEnabled(true)
	isis := isisP.GetOrCreateIsis()

	g := isis.GetOrCreateGlobal()
	if deviations.ISISInstanceEnabledRequired(td.dut) {
		g.Instance = ygot.String(isisName)
	}
	g.LevelCapability = oc.Isis_LevelType_LEVEL_2
	g.Net = []string{fmt.Sprintf("%v.%v.00", dutAreaAddr, dutSysID)}
	g.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	g.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)

	isisLevel2 := isis.GetOrCreateLevel(2)
	isisLevel2.MetricStyle = oc.Isis_MetricStyle_WIDE_METRIC
	if deviations.ISISLevelEnabled(td.dut) {
		isisLevel2.Enabled = ygot.Bool(true)
	}

	p1Name := td.dut.Port(t, "port1").Name()
	p2Name := td.dut.Port(t, "port2").Name()
	if deviations.InterfaceRefInterfaceIDFormat(td.dut) {
		for _, intfName := range []string{p1Name, p2Name} {
			isisIntf := isis.GetOrCreateInterface(intfName + ".0")
			isisIntf.GetOrCreateInterfaceRef().Interface = ygot.String(intfName)
			isisIntf.GetOrCreateInterfaceRef().Subinterface = ygot.Uint32(0)
		}
	}
	if deviations.ExplicitInterfaceInDefaultVRF(td.dut) || deviations.InterfaceRefInterfaceIDFormat(td.dut) {
		p1Name += ".0"
		p2Name += ".0"
	}
	for _, intfName := range []string{p1Name, p2Name} {
		isisIntf := isis.GetOrCreateInterface(intfName)
		if !deviations.InterfaceRefInterfaceIDFormat(td.dut) {
			isisIntf.GetOrCreateInterfaceRef().Interface = ygot.String(intfName)
			isisIntf.GetOrCreateInterfaceRef().Subinterface = ygot.Uint32(0)
		}
		if deviations.InterfaceRefConfigUnsupported(td.dut) {
			isisIntf.InterfaceRef = nil
		}
		isisIntf.Enabled = ygot.Bool(true)
		isisIntf.CircuitType = oc.Isis_CircuitType_POINT_TO_POINT
		isisIntf.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
		isisIntf.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
		if deviations.ISISInterfaceAfiUnsupported(td.dut) {
			isisIntf.Af = nil
		}

		isisIntfLevel := isisIntf.GetOrCreateLevel(2)
		isisIntfLevel.Enabled = ygot.Bool(true)

		isisIntfLevelAfiv4 := isisIntfLevel.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST)
		isisIntfLevelAfiv4.Metric = ygot.Uint32(10)
		isisIntfLevelAfiv4.Enabled = ygot.Bool(true)
		isisIntfLevelAfiv6 := isisIntfLevel.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST)
		isisIntfLevelAfiv6.Metric = ygot.Uint32(10)
		isisIntfLevelAfiv6.Enabled = ygot.Bool(true)
		if deviations.MissingIsisInterfaceAfiSafiEnable(td.dut) {
			isisIntfLevelAfiv4.Enabled = nil
			isisIntfLevelAfiv6.Enabled = nil
		}
	}
	gnmi.Update(t, td.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(td.dut)).Config(), ni)

	dev1ISIS := td.otgP1.Isis().SetSystemId(ate1SysID).SetName(td.otgP1.Name() + ".ISIS")
	dev1ISIS.Basic().SetHostname(dev1ISIS.Name()).SetLearnedLspFilter(true)
	dev1ISIS.Advanced().SetAreaAddresses([]string{strings.Replace(ateAreaAddr, ".", "", -1)})
	dev1IsisInt := dev1ISIS.Interfaces().Add().
		SetEthName(td.otgP1.Ethernets().Items()[0].Name()).SetName("dev1IsisInt").
		SetNetworkType(gosnappi.IsisInterfaceNetworkType.POINT_TO_POINT).
		SetLevelType(gosnappi.IsisInterfaceLevelType.LEVEL_2).
		SetMetric(10)
	dev1IsisInt.Advanced().SetAutoAdjustMtu(true).SetAutoAdjustArea(true).SetAutoAdjustSupportedProtocols(true)

	dev2ISIS := td.otgP2.Isis().SetSystemId(ate2SysID).SetName(td.otgP2.Name() + ".ISIS")
	dev2ISIS.Basic().SetHostname(dev2ISIS.Name()).SetLearnedLspFilter(true)
	dev2ISIS.Advanced().SetAreaAddresses([]string{strings.Replace(ateAreaAddr, ".", "", -1)})
	dev2IsisInt := dev2ISIS.Interfaces().Add().
		SetEthName(td.otgP2.Ethernets().Items()[0].Name()).SetName("dev2IsisInt").
		SetNetworkType(gosnappi.IsisInterfaceNetworkType.POINT_TO_POINT).
		SetLevelType(gosnappi.IsisInterfaceLevelType.LEVEL_2).
		SetMetric(10)
	dev2IsisInt.Advanced().SetAutoAdjustMtu(true).SetAutoAdjustArea(true).SetAutoAdjustSupportedProtocols(true)

	// configure emulated network params
	net2v4 := td.otgP1.Isis().V4Routes().Add().SetName("v4-isisNet-dev1").SetLinkMetric(10)
	net2v4.Addresses().Add().SetAddress(td.advertisedIPv4.address).SetPrefix(td.advertisedIPv4.prefix)
	net2v6 := td.otgP1.Isis().V6Routes().Add().SetName("v6-isisNet-dev1").SetLinkMetric(10)
	net2v6.Addresses().Add().SetAddress(td.advertisedIPv6.address).SetPrefix(td.advertisedIPv6.prefix)

	net3v4 := td.otgP2.Isis().V4Routes().Add().SetName("v4-isisNet-dev2").SetLinkMetric(10)
	net3v4.Addresses().Add().SetAddress(td.advertisedIPv4.address).SetPrefix(td.advertisedIPv4.prefix)
	net3v6 := td.otgP2.Isis().V6Routes().Add().SetName("v6-isisNet-dev2").SetLinkMetric(10)
	net3v6.Addresses().Add().SetAddress(td.advertisedIPv6.address).SetPrefix(td.advertisedIPv6.prefix)
}

// testCrossAddressFamilyNextHops validates XAF configurations.
// Relevant README subtest: RT-1.26.5 - Validate Cross-Address Family (XAF) Next-Hops
// Technical Explanation: Overrides IPv4 target destinations to route exclusively
// via IPv6 mapped interfaces, and vice versa. It validates whether the protocol stack
// can span address families without disruption.
func (td *testData) testCrossAddressFamilyNextHops(t *testing.T) {
	if deviations.IPv6StaticRouteWithIPv4NextHopUnsupported(td.dut) || deviations.IPv4StaticRouteWithIPv6NextHopUnsupported(td.dut) {
		t.Skip("Skipping XAF route test. Deviations unsupported.")
	}

	b := &gnmi.SetBatch{}
	// Step 2 - Configure IPv6 static route `2001:db8:128:128::/64` with next-hops
	// set to the IPv4 address of ATE port-1 and ATE port-2.
	var v6Cfg *cfgplugins.StaticRouteCfg
	if deviations.IPv6StaticRouteWithIPv4NextHopRequiresStaticARP(td.dut) {
		staticARPWithMagicUniversalIP(t, td.dut)
		v6Cfg = &cfgplugins.StaticRouteCfg{
			NetworkInstance: deviations.DefaultNetworkInstance(td.dut),
			Prefix:          td.staticIPv6.cidr(t),
			NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
				"0": oc.UnionString(dummyV6),
			},
		}
	} else {
		v6Cfg = &cfgplugins.StaticRouteCfg{
			NetworkInstance: deviations.DefaultNetworkInstance(td.dut),
			Prefix:          td.staticIPv6.cidr(t),
			NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
				"0": oc.UnionString(atePort1.IPv4),
				"1": oc.UnionString(atePort2.IPv4),
			},
		}
	}
	if _, err := cfgplugins.NewStaticRouteCfg(b, v6Cfg, td.dut); err != nil {
		t.Fatalf("Failed to configure IPv6 static route: %v", err)
	}

	// Step 3 - Configure IPv4 static route `203.0.113.0/24` with next-hops set to
	// the IPv6 address of ATE port-1 and ATE port-2.
	v4Cfg := &cfgplugins.StaticRouteCfg{
		NetworkInstance: deviations.DefaultNetworkInstance(td.dut),
		Prefix:          td.staticIPv4.cidr(t),
		NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
			"0": oc.UnionString(atePort1.IPv6),
			"1": oc.UnionString(atePort2.IPv6),
		},
	}
	if _, err := cfgplugins.NewStaticRouteCfg(b, v4Cfg, td.dut); err != nil {
		t.Fatalf("Failed to configure IPv4 static route: %v", err)
	}

	t.Cleanup(func() {
		td.deleteStaticRoutes(t)
	})

	// Step 4 - Push configuration to DUT.
	b.Set(t, td.dut)

	// Step 5 - Validate the routes are configured and the cross-family next-hops are reported correctly
	// Validations
	t.Run("Telemetry", func(t *testing.T) {
		sp := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(td.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(td.dut))
		gnmi.Await(t, td.dut, sp.Static(td.staticIPv4.cidr(t)).Prefix().State(), 30*time.Second, td.staticIPv4.cidr(t))
		gnmi.Await(t, td.dut, sp.Static(td.staticIPv6.cidr(t)).Prefix().State(), 30*time.Second, td.staticIPv6.cidr(t))

		if deviations.SkipStaticNexthopCheck(td.dut) {
			nexthops := gnmi.LookupAll(t, td.dut, sp.Static(td.staticIPv4.cidr(t)).NextHopAny().NextHop().State())
			if len(nexthops) != 2 {
				t.Errorf("IPv4 Static Route next hop: want 2 nexthops,got %d nexthops", len(nexthops))
			}
			for _, nexthop := range nexthops {
				if got, ok := nexthop.Val(); !ok || (got != oc.UnionString(atePort1.IPv6) && got != oc.UnionString(atePort2.IPv6)) {
					t.Errorf("IPv4 Static Route next hop: got %s", got)
				}
			}
		} else {
			gotStatic := gnmi.Get(t, td.dut, sp.Static(td.staticIPv4.cidr(t)).State())
			if got, want := gotStatic.GetNextHop("0").GetNextHop(), oc.UnionString(atePort1.IPv6); got != want {
				t.Errorf("IPv4 Static Route next hop: got: %s, want: %s", got, want)
			}
			if got, want := gotStatic.GetNextHop("1").GetNextHop(), oc.UnionString(atePort2.IPv6); got != want {
				t.Errorf("IPv4 Static Route next hop: got: %s, want: %s", got, want)
			}
		}

		if !deviations.IPv6StaticRouteWithIPv4NextHopRequiresStaticARP(td.dut) {
			gotStatic := gnmi.Get(t, td.dut, sp.Static(td.staticIPv6.cidr(t)).State())
			if got, want := gotStatic.GetNextHop("0").GetNextHop(), oc.UnionString(atePort1.IPv4); got != want {
				t.Errorf("IPv6 Static Route next hop: got: %s, want: %s", got, want)
			}
			if got, want := gotStatic.GetNextHop("1").GetNextHop(), oc.UnionString(atePort2.IPv4); got != want {
				t.Errorf("IPv6 Static Route next hop: got: %s, want: %s", got, want)
			}
		}
	})

	t.Run("Traffic", func(t *testing.T) {
		td.ate.OTG().StartTraffic(t)
		time.Sleep(trafficDuration)
		td.ate.OTG().StopTraffic(t)

		lossV4 := otgutils.GetFlowLossPct(t, td.ate.OTG(), v4Flow, 20*time.Second)
		lossV6 := otgutils.GetFlowLossPct(t, td.ate.OTG(), v6Flow, 20*time.Second)
		otgutils.LogFlowMetrics(t, td.ate.OTG(), td.top)

		if lossV4 > lossTolerance {
			t.Errorf("Loss percent for IPv4 Traffic: got: %f, want 0%%", lossV4)
		}
		if lossV6 > lossTolerance {
			t.Errorf("Loss percent for IPv6 Traffic: got: %f, want 0%%", lossV6)
		}

		portCountersV4 := egressTrackingCounters(t, td.ate, v4Flow)
		p1CounterV4, ok4_1 := portCountersV4[port1Tag]
		p2CounterV4, ok4_2 := portCountersV4[port2Tag]

		if ok4_1 && ok4_2 {
			if total := p1CounterV4 + p2CounterV4; total == 0 {
				t.Errorf("ECMP IPv4 load balance error: total packets received is zero")
			} else if got, want := p1CounterV4*100/total, uint64(50); got < want-ecmpTolerance || got > want+ecmpTolerance {
				t.Errorf("ECMP IPv4 load balance error for port1, got: %v, want: %v", got, want)
			}
		}

		portCountersV6 := egressTrackingCounters(t, td.ate, v6Flow)
		p1CounterV6, ok6_1 := portCountersV6[port1Tag]
		p2CounterV6, ok6_2 := portCountersV6[port2Tag]
		if ok6_1 && ok6_2 {
			if total := p1CounterV6 + p2CounterV6; total == 0 {
				t.Errorf("ECMP IPv6 load balance error: total packets received is zero")
			} else if got, want := p1CounterV6*100/total, uint64(50); got < want-ecmpTolerance || got > want+ecmpTolerance {
				t.Errorf("ECMP IPv6 load balance error for port1, got: %v, want: %v", got, want)
			}
		}
	})
}

// testDirectInterfaceIPDeletion validates static route withdrawal.
// Relevant README subtest: RT-1.26.9 - Direct Interface IP Deletion (Negative)
// Technical Explanation: Drops an interface IP address dynamically during route
// propagation to ensure the next hop transitions to inactive without crashing the control plane.
func (td *testData) testDirectInterfaceIPDeletion(t *testing.T) {
	// Step 1: Configure a static route that resolves via a directly connected interface IP.
	b := &gnmi.SetBatch{}
	sV4 := &cfgplugins.StaticRouteCfg{
		NetworkInstance: deviations.DefaultNetworkInstance(td.dut),
		Prefix:          td.staticIPv4.cidr(t),
		NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
			"0": oc.UnionString(atePort2.IPv4),
		},
		// FIX: Add this block to ensure the route cannot failover to IS-IS
		// explicitly enforcing "Direct Interface IP" requirement from RT-1.26.9.
		Metric: uint32(100),
	}
	if _, err := cfgplugins.NewStaticRouteCfg(b, sV4, td.dut); err != nil {
		t.Fatalf("Failed to configure IPv4 static route: %v", err)
	}

	sp := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(td.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(td.dut))
	// explicitly enforcing "Direct Interface IP" requirement from RT-1.26.9 by disabling recurse over ISIS backdoors.
	gnmi.BatchReplace(b, sp.Static(td.staticIPv4.cidr(t)).NextHop("0").Recurse().Config(), false)
	b.Set(t, td.dut)

	gnmi.Await(t, td.dut, sp.Static(td.staticIPv4.cidr(t)).Prefix().State(), 30*time.Second, td.staticIPv4.cidr(t))

	port2 := td.dut.Port(t, "port2").Name()
	if deviations.ExplicitInterfaceInDefaultVRF(td.dut) || deviations.InterfaceRefInterfaceIDFormat(td.dut) {
		port2 = port2 + ".0"
	}

	t.Cleanup(func() {
		ipConf := &oc.Interface_Subinterface_Ipv4_Address{
			Ip:           ygot.String(dutPort2.IPv4),
			PrefixLength: ygot.Uint8(uint8(dutPort2.IPv4Len)),
		}
		gnmi.Replace(t, td.dut, gnmi.OC().Interface(port2).Subinterface(0).Ipv4().Address(dutPort2.IPv4).Config(), ipConf)

		// FIX: Wait for restored State
		gnmi.Await(t, td.dut, gnmi.OC().Interface(port2).Subinterface(0).Ipv4().Address(dutPort2.IPv4).Ip().State(), 30*time.Second, dutPort2.IPv4)
		td.deleteStaticRoutes(t)
	})

	// Step 2: Delete the IP address of that direct interface using a gNMI Set DELETE
	gnmi.Delete(t, td.dut, gnmi.OC().Interface(port2).Subinterface(0).Ipv4().Address(dutPort2.IPv4).Config())

	// FIX: Add gnmi Wait block to verify the IP address has actually been removed from State!
	t.Log("Awaiting state convergence for Interface IP deletion...")
	gnmi.Watch(t, td.dut, gnmi.OC().Interface(port2).Subinterface(0).Ipv4().Address(dutPort2.IPv4).State(), 30*time.Second, func(val *ygnmi.Value[*oc.Interface_Subinterface_Ipv4_Address]) bool {
		return !val.IsPresent()
	}).Await(t)

	// Step 3: Send Traffic
	td.ate.OTG().StartTraffic(t)
	time.Sleep(trafficDuration)
	td.ate.OTG().StopTraffic(t)

	// Step 4: Verify the traffic drops, and the static route becomes inactive without crashing the device.
	lossV4 := otgutils.GetFlowLossPct(t, td.ate.OTG(), v4Flow, 20*time.Second)
	if got, want := lossV4, float64(100); got != want {
		t.Errorf("Loss percent for IPv4 Traffic after direct IP deletion: got: %v, want %v", got, want)
	}
}

// testOverlappingPrefixesLPM validates Longest Prefix Match sorting.
// Relevant README subtest: RT-1.26.10 - Overlapping Prefixes / LPM (Corner)
// Technical Explanation: Maps overlapping /8 and /24 topologies simultaneously.
// Uses targeted synthetic traffic directed perfectly within the /24 envelope to prove priority.
func (td *testData) testOverlappingPrefixesLPM(t *testing.T) {
	// Step 1: Configure overlapping static routes: 10.0.0.0/8 pointing to ATE port-1, and 10.1.1.0/24 pointing to ATE port-2.
	b := &gnmi.SetBatch{}
	prefix8 := "10.0.0.0/8"
	prefix24 := "10.1.1.0/24"

	sV4_8 := &cfgplugins.StaticRouteCfg{
		NetworkInstance: deviations.DefaultNetworkInstance(td.dut),
		Prefix:          prefix8,
		NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
			"0": oc.UnionString(atePort1.IPv4),
		},
	}
	if _, err := cfgplugins.NewStaticRouteCfg(b, sV4_8, td.dut); err != nil {
		t.Fatalf("Failed to configure %s static route: %v", prefix8, err)
	}

	sV4_24 := &cfgplugins.StaticRouteCfg{
		NetworkInstance: deviations.DefaultNetworkInstance(td.dut),
		Prefix:          prefix24,
		NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
			"0": oc.UnionString(atePort2.IPv4),
		},
	}
	if _, err := cfgplugins.NewStaticRouteCfg(b, sV4_24, td.dut); err != nil {
		t.Fatalf("Failed to configure %s static route: %v", prefix24, err)
	}

	t.Cleanup(func() {
		bDel := &gnmi.SetBatch{}
		sp := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(td.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(td.dut))
		gnmi.BatchDelete(bDel, sp.Static(prefix8).Config())
		gnmi.BatchDelete(bDel, sp.Static(prefix24).Config())
		bDel.Set(t, td.dut)
	})

	// Step 2: Push configuration to DUT.
	b.Set(t, td.dut)

	sp := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(td.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(td.dut))
	gnmi.Await(t, td.dut, sp.Static(prefix8).Prefix().State(), 30*time.Second, prefix8)
	gnmi.Await(t, td.dut, sp.Static(prefix24).Prefix().State(), 30*time.Second, prefix24)

	// RT-1.26.10 Step 3 - Send traffic to destination 10.1.1.1 to verify LPM routing
	lpmFlowName := "lpmV4"
	lpmF := td.top.Flows().Add()
	lpmF.SetName(lpmFlowName).Metrics().SetEnable(true)

	srcV4 := td.otgP3.Ethernets().Items()[0].Ipv4Addresses().Items()[0]
	dst1V4 := td.otgP1.Ethernets().Items()[0].Ipv4Addresses().Items()[0]
	dst2V4 := td.otgP2.Ethernets().Items()[0].Ipv4Addresses().Items()[0]

	lpmF.TxRx().Device().SetTxNames([]string{srcV4.Name()}).SetRxNames([]string{dst1V4.Name(), dst2V4.Name()})
	eth := lpmF.Packet().Add().Ethernet()
	eth.Src().SetValue(atePort3.MAC)
	lpmIP := lpmF.Packet().Add().Ipv4()
	lpmIP.Src().SetValue(srcV4.Address())
	lpmIP.Dst().SetValue("10.1.1.1") // Directly hit the /24 envelope

	egressEth := lpmF.EgressPacket().Add().Ethernet()
	ethTag := egressEth.Dst().MetricTags().Add()
	ethTag.SetName("MACTrackingLPM").SetOffset(40).SetLength(8)

	td.ate.OTG().PushConfig(t, td.top)
	td.ate.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, td.ate.OTG(), td.top, "IPv4")

	td.ate.OTG().StartTraffic(t)
	time.Sleep(trafficDuration)
	td.ate.OTG().StopTraffic(t)

	// RT-1.26.10 Step 4 - Verify that traffic strictly adheres to Longest Prefix Match (LPM)
	lossLPM := otgutils.GetFlowLossPct(t, td.ate.OTG(), lpmFlowName, 20*time.Second)
	if lossLPM > lossTolerance {
		t.Errorf("Loss percent for LPM Traffic: got: %f, want 0%%", lossLPM)
	}

	portCounters := egressTrackingCounters(t, td.ate, lpmFlowName)
	p2Counter, ok := portCounters[port2Tag]
	if !ok {
		t.Logf("LPM tracking counters: %v", portCounters)
	}
	_, rxLPM := otgutils.GetFlowStats(t, td.ate.OTG(), lpmFlowName, 20*time.Second)
	if rxLPM == 0 {
		t.Errorf("LPM IPv4 traffic: received 0 packets, expected traffic to flow")
	} else if got, want := float64(p2Counter)*100/float64(rxLPM), float64(100); got+lossTolerance < want {
		t.Errorf("LPM IPv4 traffic on port2, got: %v%%, want: %v%%", got, want)
	}
}

// testRouteResolutionLoop asserts recursive safeties.
// Relevant README subtest: RT-1.26.11 - Route Resolution Loop (Negative)
// Technical Explanation: Submits cross configurations (A pointing to B, B pointing to A).
// Asserts that bounding checks block infinite loops without OS crashing.
func (td *testData) testRouteResolutionLoop(t *testing.T) {
	// Step 1: Configure Static Route A pointing to Next-Hop IP B
	// Step 2: Configure Static Route B pointing to Next-Hop IP A
	b := &gnmi.SetBatch{}
	prefixA := "192.168.1.0/24"
	nhB := "192.168.2.1"

	prefixB := "192.168.2.0/24"
	nhA := "192.168.1.1"

	sV4A := &cfgplugins.StaticRouteCfg{
		NetworkInstance: deviations.DefaultNetworkInstance(td.dut),
		Prefix:          prefixA,
		NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
			"0": oc.UnionString(nhB),
		},
	}
	if _, err := cfgplugins.NewStaticRouteCfg(b, sV4A, td.dut); err != nil {
		t.Fatalf("Failed to configure %s static route: %v", prefixA, err)
	}

	sV4B := &cfgplugins.StaticRouteCfg{
		NetworkInstance: deviations.DefaultNetworkInstance(td.dut),
		Prefix:          prefixB,
		NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
			"0": oc.UnionString(nhA),
		},
	}
	if _, err := cfgplugins.NewStaticRouteCfg(b, sV4B, td.dut); err != nil {
		t.Fatalf("Failed to configure %s static route: %v", prefixB, err)
	}

	t.Cleanup(func() {
		bDel := &gnmi.SetBatch{}
		sp := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(td.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(td.dut))
		gnmi.BatchDelete(bDel, sp.Static(prefixA).Config())
		gnmi.BatchDelete(bDel, sp.Static(prefixB).Config())
		bDel.Set(t, td.dut)
	})

	// Step 3: Push configuration to DUT.
	b.Set(t, td.dut)

	// RT-1.26.11 Step 4: Verify the device's control plane detects or breaks the recursion loop safely without hanging or crashing
	// If the operating system stays resilient, gnmi will eventually respond after internal reconvergence bounds.
	// We run gnmi.Await checking config state within a reasonable timeout.
	sp := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(td.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(td.dut))
	gnmi.Await(t, td.dut, sp.Static(prefixA).Prefix().Config(), 45*time.Second, prefixA)
	gnmi.Await(t, td.dut, sp.Static(prefixB).Prefix().Config(), 45*time.Second, prefixB)
}
