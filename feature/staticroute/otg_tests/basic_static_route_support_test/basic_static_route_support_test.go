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

// cidr
// Objective: Helper function to generate a CIDR string from an IP address and prefix length.
// Traceability: Setup/Helper
// Technical Summary: Combines the IP address string and prefix into CIDR notation and validates it using `net.ParseCIDR`.
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
	otgP4          gosnappi.Device
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
		otgP4:          devs[3],
		staticIPv4:     ipAddr{address: v4Route, prefix: v4RoutePrefix},
		staticIPv6:     ipAddr{address: v6Route, prefix: v6RoutePrefix},
		advertisedIPv4: ipAddr{address: v4Route, prefix: v4RoutePrefix},
		advertisedIPv6: ipAddr{address: v6Route, prefix: v6RoutePrefix},
	}
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
			desc: "RT-1.26.5: Cross-Address Family (XAF) Next-Hops",
			fn:   td.testStaticRouteXAFNextHops,
		},
		{
			desc: "RT-1.26.6: Static Route With Drop Next Hop",
			fn:   td.testStaticRouteWithDropNextHop,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.desc, func(t *testing.T) {
			tc.fn(t)
		})
	}
}

// TestRT1268_StaticRouteAddRemove
// Objective: Validates the dynamic addition and removal of static route next-hops on the DUT.
// Traceability: RT-1.26.8 - Validate Dynamic Add and Remove of Next-Hops
// Technical Summary: Dynamically configures new static route next-hops, verifies state propagation, and subsequently deletes a subset of next-hops, ensuring the device control plane maintains correct state without traffic interruption.
func TestRT1268_StaticRouteAddRemove(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	configureDUT(t, dut)

	ate := ondatra.ATE(t, "ate")
	top := gosnappi.NewConfig()
	devs := configureOTG(t, ate, top)

	td := testData{
		dut:   dut,
		ate:   ate,
		top:   top,
		otgP1: devs[0],
		otgP2: devs[1],
		otgP3: devs[2],
		otgP4: devs[3],
	}

	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	defer ate.OTG().StopProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")
	td.configureOTGFlows(t)

	// Subtest ID: RT-1.26.8 - Validate Dynamic Add and Remove of Next-Hops
	// Step 1 - Configure one IPv4 static route with next-hops set to the IPv4 address of ATE port-2 (index 0) and port-3 (index 1).
	prefix := ipAddr{address: v4Route, prefix: v4RoutePrefix}
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
	cfgplugins.ValidateStaticRouteConfigured(t, dut, deviations.DefaultNetworkInstance(dut), prefix.cidr(t), sV4)

	// Step 3 - Update the static route by adding next-hops for ATE port-1 (index 2) and port-4 (index 3).
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
	cfgplugins.ValidateStaticRouteConfigured(t, dut, deviations.DefaultNetworkInstance(dut), prefix.cidr(t), sV4)
	expectedNh := map[string]string{"0": atePort2.IPv4, "1": atePort3.IPv4, "2": atePort1.IPv4, "3": atePort4.IPv4}
	cfgplugins.ValidateStaticRouteNextHopIndex(t, dut, deviations.DefaultNetworkInstance(dut), prefix.cidr(t), expectedNh)

	// Step 6 - Remove two next-hops (e.g., indexes 0 and 3).
	cfgplugins.DeleteStaticRouteNextHops(t, dut, deviations.DefaultNetworkInstance(dut), prefix.cidr(t), "0", "3")

	// Step 7 - Push configuration to DUT and validate that only two next-hops remain.
	// (Note: DeleteStaticRouteNextHops inherently updates the DUT)
	sV4 = &cfgplugins.StaticRouteCfg{
		NetworkInstance: deviations.DefaultNetworkInstance(dut),
		Prefix:          prefix.cidr(t),
		NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
			"1": oc.UnionString(atePort3.IPv4),
			"2": oc.UnionString(atePort1.IPv4),
		},
	}
	cfgplugins.ValidateStaticRouteConfigured(t, dut, deviations.DefaultNetworkInstance(dut), prefix.cidr(t), sV4)
	expectedNh = map[string]string{"1": atePort3.IPv4, "2": atePort1.IPv4}
	cfgplugins.ValidateStaticRouteNextHopIndex(t, dut, deviations.DefaultNetworkInstance(dut), prefix.cidr(t), expectedNh)

	// Step 8 - Validate traffic converges correctly after the dynamic route updates on active ports.
	td.ate.OTG().StartTraffic(t)
	time.Sleep(trafficDuration)
	td.ate.OTG().StopTraffic(t)
	lossV4 := otgutils.GetFlowLossPct(t, td.ate.OTG(), v4Flow, 20*time.Second)
	if lossV4 > lossTolerance {
		t.Errorf("Loss percent for IPv4 Traffic after dynamic update: got: %f, want <= %f%%", lossV4, lossTolerance)
	}
}

// TestDisableRecursiveNextHopResolution
// Objective: Validates that the device properly halts static route resolution when recursive lookup is disabled.
// Traceability: RT-1.26.7 - Validate Disabling Recursive Next-Hop Resolution
// Technical Summary: Pre-provisions standard routes resolved recursively via IS-IS. Then delegates to subtests to verify traffic flows normally, followed by a disabled recurse state which should immediately halt traffic matching that route.
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
		otgP4:          devs[3],
		staticIPv4:     ipAddr{address: v4Route, prefix: v4RoutePrefix},
		staticIPv6:     ipAddr{address: v6Route, prefix: v6RoutePrefix},
		advertisedIPv4: ipAddr{address: v4LoopbackRoute, prefix: v4LoopbackRoutePrefix},
		advertisedIPv6: ipAddr{address: v6LoopbackRoute, prefix: v6LoopbackRoutePrefix},
	}

	// Step 1 - Configure IPv4 and IPv6 IS-IS between ATE port-1 <-> DUT port-1 and ATE port-2 <-> DUT port-2.
	// Step 2 - Configure one IPv4 /32 host route (198.51.100.100/32) and one IPv6 /128 host route (2001:db8::64:64::1/128) connected to ATE and advertised to DUT through both IS-IS adjacencies.
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

	t.Run("RT-1.26.7: Disable Recursive Next Hop Resolution", func(t *testing.T) {
		// Subtest ID: RT-1.26.7 - Validate Disabling Recursive Next-Hop Resolution
		td.testRecursiveNextHopResolution(t)
		td.testRecursiveNextHopResolutionDisabled(t)
	})
}

// testRecursiveNextHopResolution
// Objective: Validates successful recursive next-hop resolution for IPv4 and IPv6 static routes as a baseline.
// Traceability: RT-1.26.7 (Step 3-5)
// Technical Summary: Sets `recurse` to true on static route next-hops resolving to routes advertised over IS-IS, awaits state convergence using `gnmi.Watch`, and verifies 0% traffic loss via ATE.
func (td *testData) testRecursiveNextHopResolution(t *testing.T) {
	b := &gnmi.SetBatch{}
	// Step 3 - Configure an IPv4 static route for 203.0.113.0/24 with next-hop 198.51.100.100. Configure an IPv6 static route for 2001:db8:128:128::/64 with next-hop 2001:db8::64:64::1.
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

	// Step 4 - Push configuration to DUT.
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
		// Step 5 - Send Traffic and Verify that traffic is received from DUT.
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

// testRecursiveNextHopResolutionDisabled
// Objective: Validates that traffic is dropped when recursive next-hop resolution is explicitly disabled.
// Traceability: RT-1.26.7 (Step 6-9)
// Technical Summary: Overwrites the static route next-hop `recurse` config to false. Validates its propagation into state using `gnmi.Watch`, then measures traffic, asserting 100% packet loss as the route becomes unresolved.
func (td *testData) testRecursiveNextHopResolutionDisabled(t *testing.T) {
	sp := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(td.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(td.dut))
	// Step 6 - Disable static route next-hop recursive lookup by setting recurse to false.
	batch := &gnmi.SetBatch{}
	gnmi.BatchReplace(batch, sp.Static(td.staticIPv4.cidr(t)).NextHop("0").Recurse().Config(), false)
	gnmi.BatchReplace(batch, sp.Static(td.staticIPv6.cidr(t)).NextHop("0").Recurse().Config(), false)
	// Step 7 - Push configuration to DUT.
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
		// Step 9 - Send Traffic and Verify that traffic is NOT received from DUT, as the recursive next-hop resolution is disabled.
		td.ate.OTG().StartTraffic(t)
		time.Sleep(trafficDuration)
		td.ate.OTG().StopTraffic(t)

		lossV4 := otgutils.GetFlowLossPct(t, td.ate.OTG(), v4Flow, 20*time.Second)
		lossV6 := otgutils.GetFlowLossPct(t, td.ate.OTG(), v6Flow, 20*time.Second)

		otgutils.LogFlowMetrics(t, td.ate.OTG(), td.top)
		if got, want := lossV4, float64(100); got != want {
			t.Errorf("Loss percent for IPv4 Traffic: got: %f, want %f", got, want)
		}
		if got, want := lossV6, float64(100); got != want {
			t.Errorf("Loss percent for IPv6 Traffic: got: %f, want %f", got, want)
		}
	})
}

// configureStaticRouteToATEP1AndP2
// Objective: Helper function to configure standard IPv4 and IPv6 static routes pointing towards ATE port-1 and port-2.
// Traceability: Setup/Helper (Used in RT-1.26.1, RT-1.26.2, RT-1.26.3)
// Technical Summary: Deploys static route next-hops using `cfgplugins` mapped to destination network IPs. Pushes changes via gNMI Replace to ensure clean environment for ECMP/Metric comparisons.
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
	// Step 3 - Push configuration to DUT.
	b.Set(t, td.dut)
}

// deleteStaticRoutes
// Objective: Helper function to clean up IPv4 and IPv6 static routes post-validation.
// Traceability: Teardown/Helper
// Technical Summary: Invokes gNMI BatchDelete on the specific IPv4 and IPv6 prefixes under the static routing protocol.
func (td *testData) deleteStaticRoutes(t *testing.T) {
	b := &gnmi.SetBatch{}
	sp := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(td.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(td.dut))
	gnmi.BatchDelete(b, sp.Static(td.staticIPv4.cidr(t)).Config())
	gnmi.BatchDelete(b, sp.Static(td.staticIPv6.cidr(t)).Config())
	b.Set(t, td.dut)
}

// testStaticRouteECMP
// Objective: Validates Equal-Cost Multi-Path (ECMP) routing for static routes over IPv4 and IPv6.
// Traceability: RT-1.26.1 - Validate Static Route ECMP
// Technical Summary: Configures identical static routes pointing to different next-hops (ATE port 1 and 2). Monitors state convergence via `gnmi.Await`, sends traffic, and verifies an approximate 50/50 packet distribution across both egress ports.
func (td *testData) testStaticRouteECMP(t *testing.T) {
	// Step 1 - Configure IPv4 and IPv6 static routes for ECMP
	// Step 2 - Push configuration to DUT using gnmi.Set with REPLACE option.
	td.configureStaticRouteToATEP1AndP2(t)
	defer td.deleteStaticRoutes(t)

	t.Run("Telemetry", func(t *testing.T) {
		// Step 3 - Validate both routes are configured and reported correctly by checking that the state prefix matches.
		sp := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(td.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(td.dut))
		gnmi.Await(t, td.dut, sp.Static(td.staticIPv4.cidr(t)).Prefix().State(), 120*time.Second, td.staticIPv4.cidr(t))
		gnmi.Await(t, td.dut, sp.Static(td.staticIPv6.cidr(t)).Prefix().State(), 120*time.Second, td.staticIPv6.cidr(t))

		if deviations.SkipStaticNexthopCheck(td.dut) {
			nexthops := gnmi.LookupAll(t, td.dut, sp.Static(td.staticIPv4.cidr(t)).NextHopAny().NextHop().State())
			if len(nexthops) != 2 {
				t.Errorf("IPv4 Static Route next hop: want %d nexthops,got %d nexthops", 2, len(nexthops))
			}
			for _, nexthop := range nexthops {
				if got, ok := nexthop.Val(); !ok || !(got == oc.UnionString(atePort1.IPv4) || got == oc.UnionString(atePort2.IPv4)) {
					t.Errorf("IPv4 Static Route next hop:got %s,want %s or %s", got, oc.UnionString(atePort1.IPv4), oc.UnionString(atePort2.IPv4))
				}
			}
			nexthops = gnmi.LookupAll(t, td.dut, sp.Static(td.staticIPv6.cidr(t)).NextHopAny().NextHop().State())
			if len(nexthops) != 2 {
				t.Errorf("IPv6 Static Route next hop: want %d nexthops,got %d nexthops", 2, len(nexthops))
			}
			for _, nexthop := range nexthops {
				if got, ok := nexthop.Val(); !ok || !(got == oc.UnionString(atePort1.IPv6) || got == oc.UnionString(atePort2.IPv6)) {
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
		// Step 4 - Send IPv4 and IPv6 Traffic from ATE port-3 towards destination `203.0.113.0/24` and `2001:db8:128:128::/64`.
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
		// Step 5 - Verify that traffic is received from DUT on both port-1 and port-2, confirming ECMP works.
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
		// Step 5 - Verify that traffic is received from DUT on both port-1 and port-2, confirming ECMP works.
		if got, want := p1Counter*100/(p1Counter+p2Counter), uint64(50); got < want-ecmpTolerance || got > want+ecmpTolerance {
			t.Errorf("ECMP IPv6 load balance error for port1, got: %v, want: %v", got, want)
		}
		if got, want := p2Counter*100/(p1Counter+p2Counter), uint64(50); got < want-ecmpTolerance || got > want+ecmpTolerance {
			t.Errorf("ECMP IPv6 load balance error for port2, got: %v, want: %v", got, want)
		}
	})
}

// testStaticRouteWithMetric
// Objective: Validates that static route metrics influence path selection correctly.
// Traceability: RT-1.26.2 - Validate Static Route Metric
// Technical Summary: Modifies the metric of one path to a higher score (or lower priority). Awaits state update via `gnmi.Await` and confirms via egress packet counters that 100% of traffic flows strictly down the more favorable path, accommodating vendor deviations.
func (td *testData) testStaticRouteWithMetric(t *testing.T) {
	td.configureStaticRouteToATEP1AndP2(t)
	defer td.deleteStaticRoutes(t)

	port2Metric := uint32(100)
	sp := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(td.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(td.dut))

	// Step 1 - Set metric to 100 for both ipv4-route-b and ipv6-route-b
	batch := &gnmi.SetBatch{}
	if deviations.StaticRouteWithExplicitMetric(td.dut) {
		// per the cisco specifications setting the metric is equivlent to setting the weight, so in this case
		// we want the majority of the traffic to go over port 1 so setting the metric to 100 and port 2 as 1
		port1Metric := uint32(100)
		port2Metric = uint32(1)
		gnmi.BatchReplace(batch, sp.Static(td.staticIPv4.cidr(t)).NextHop("0").Metric().Config(), port1Metric)
		gnmi.BatchReplace(batch, sp.Static(td.staticIPv6.cidr(t)).NextHop("0").Metric().Config(), port1Metric)

	}

	gnmi.BatchReplace(batch, sp.Static(td.staticIPv4.cidr(t)).NextHop("1").Metric().Config(), port2Metric)
	gnmi.BatchReplace(batch, sp.Static(td.staticIPv6.cidr(t)).NextHop("1").Metric().Config(), port2Metric)
	// Step 2 - Push configuration to DUT using gnmi.Set with REPLACE option.
	batch.Set(t, td.dut)

	t.Run("Telemetry", func(t *testing.T) {
		if deviations.TelemetryNotSupportedForLowPriorityNh(td.dut) {
			t.Skip("Skipping Telemetry check for Metric, since deviation MissingStaticRouteNextHopMetricTelemetry is enabled.")
		}
		gnmi.Await(t, td.dut, sp.Static(td.staticIPv4.cidr(t)).Prefix().State(), 30*time.Second, td.staticIPv4.cidr(t))
		gnmi.Await(t, td.dut, sp.Static(td.staticIPv6.cidr(t)).Prefix().State(), 30*time.Second, td.staticIPv6.cidr(t))
		// Step 3 - Validate that the metric is set correctly by checking the state.
		if got, want := gnmi.Get(t, td.dut, sp.Static(td.staticIPv4.cidr(t)).NextHop("1").Metric().State()), port2Metric; got != want {
			t.Errorf("IPv4 Static Route metric for NextHop 1, got: %d, want: %d", got, want)
		}
		if got, want := gnmi.Get(t, td.dut, sp.Static(td.staticIPv6.cidr(t)).NextHop("1").Metric().State()), port2Metric; got != want {
			t.Errorf("IPv6 Static Route metric for NextHop 1, got: %d, want: %d", got, want)
		}
	})

	t.Run("Traffic", func(t *testing.T) {
		// Step 4 - Send IPv4 and IPv6 Traffic from ATE port-3 towards destination `203.0.113.0/24` and `2001:db8:128:128::/64`.
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
		// Step 5 - Verify that traffic is received from DUT on port-1 and NOT on port-2
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

		// Step 5 - Verify that traffic is received from DUT on port-1 and NOT on port-2
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

// testStaticRouteWithPreference
// Objective: Validates that static route preference (administrative distance) influences path selection.
// Traceability: RT-1.26.3 - Validate Static Route Preference
// Technical Summary: Edits the preference (or metric dependent on vendor deviation) for next-hops. Awaits the state update via `gnmi.Await`, then generates traffic assuring it routes strictly down the path with the superior preference value.
func (td *testData) testStaticRouteWithPreference(t *testing.T) {
	td.configureStaticRouteToATEP1AndP2(t)
	defer td.deleteStaticRoutes(t)

	const port1Preference = uint32(50)

	sp := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(td.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(td.dut))

	// Subtest ID: RT-1.26.3 - Validate Static Route Preference
	batch := &gnmi.SetBatch{}

	// Step 1 - Set the value of /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config/preference to 50 for both ipv4-route-a and ipv6-route-a.
	if deviations.SetMetricAsPreference(td.dut) {
		const port2Metric = uint32(100)
		gnmi.BatchReplace(batch, sp.Static(td.staticIPv4.cidr(t)).NextHop("1").Metric().Config(), port2Metric)
		gnmi.BatchReplace(batch, sp.Static(td.staticIPv6.cidr(t)).NextHop("1").Metric().Config(), port2Metric)

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
	// Step 2 - Push configuration to DUT.
	batch.Set(t, td.dut)

	t.Run("Telemetry", func(t *testing.T) {
		if deviations.SetMetricAsPreference(td.dut) || deviations.TelemetryNotSupportedForLowPriorityNh(td.dut) {
			t.Skip("Skipping Preference telemetry check since deviation SetMetricAsPreference is enabled")
		}
		gnmi.Await(t, td.dut, sp.Static(td.staticIPv4.cidr(t)).Prefix().State(), 30*time.Second, td.staticIPv4.cidr(t))
		gnmi.Await(t, td.dut, sp.Static(td.staticIPv6.cidr(t)).Prefix().State(), 30*time.Second, td.staticIPv6.cidr(t))
		// Step 3 - Validate that the preference is set correctly by checking the state.
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
		// Step 4 - Send IPv4 and IPv6 Traffic from ATE port-3 towards destination `203.0.113.0/24` and `2001:db8:128:128::/64`.
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
		// Step 5 - Verify that traffic is now received from DUT on port-2 and NOT on port-1
		portCounters := egressTrackingCounters(t, td.ate, v4Flow)
		_, rxV4 := otgutils.GetFlowStats(t, td.ate.OTG(), v4Flow, 20*time.Second)
		port2Counter, ok := portCounters[port2Tag]
		if !ok {
			t.Errorf("Port2 IPv4 egress tracking counter not found: %v", portCounters)
		}
		if got, want := float64(port2Counter)*100/float64(rxV4), float64(100); got+lossTolerance < want {
			t.Errorf("IPv4 traffic on port2, got: %v, want: %v", got, want)
		}
		// Step 5 - Verify that traffic is now received from DUT on port-2 and NOT on port-1
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

// testStaticRouteSetTag
// Objective: Validates that static routes can accept and retain custom tags.
// Traceability: RT-1.26.4 - Validate Static Route Tag
// Technical Summary: Assigns a `set-tag` value of 10 to standard static routes and validates via `gnmi.Await` that the assigned value matches the device's operational state.
func (td *testData) testStaticRouteSetTag(t *testing.T) {
	const tag = uint32(10)

	b := &gnmi.SetBatch{}
	// Step 1 - Configure a tag of value 10 on the IPv4 and IPv6 static routes
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

	// Step 2 - Push configuration to DUT.
	b.Set(t, td.dut)

	defer td.deleteStaticRoutes(t)

	// Step 3 - Validate the tag is set correctly by checking the state.
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

// testStaticRouteXAFNextHops
// Objective: Validates Cross-Address Family (XAF) next-hop resolution (e.g., IPv6 route over IPv4 next-hop).
// Traceability: RT-1.26.5 - Validate Cross-Address Family (XAF) Next-Hops
// Technical Summary: Removes specific metrics/preferences to level the ECMP plane. Configures IPv6 destinations with IPv4 next-hops and vice versa. Evaluates State paths for correctness via `gnmi.Await`, factoring in static ARP requirements per vendor, and verifies 0% packet loss and 50/50 egress split.
func (td *testData) testStaticRouteXAFNextHops(t *testing.T) {
	// Subtest ID: RT-1.26.5 - Validate Cross-Address Family (XAF) Next-Hops
	// Step 1 - Delete the configuration using a gNMI Set DELETE on the specific metric and preference paths for ipv4-route-b, ipv6-route-b, ipv4-route-a, and ipv6-route-a.
	// In the real setup, you would just delete the existing routes or specifically configure the target routes.
	// But according to the test procedure, we should delete the specific leaves.
	netInst := deviations.DefaultNetworkInstance(td.dut)
	sp := gnmi.OC().NetworkInstance(netInst).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(td.dut))
	cfgplugins.DeleteStaticRouteNextHopLeaves(t, td.dut, netInst, td.staticIPv4.cidr(t), "0", "Metric", "Preference")
	cfgplugins.DeleteStaticRouteNextHopLeaves(t, td.dut, netInst, td.staticIPv4.cidr(t), "1", "Metric", "Preference")
	cfgplugins.DeleteStaticRouteNextHopLeaves(t, td.dut, netInst, td.staticIPv6.cidr(t), "0", "Metric", "Preference")
	cfgplugins.DeleteStaticRouteNextHopLeaves(t, td.dut, netInst, td.staticIPv6.cidr(t), "1", "Metric", "Preference")

	// Step 2 - Configure IPv6 static route with next-hops set to the IPv4 address of ATE port-1 and ATE port-2.
	// Step 3 - Configure IPv4 static route with next-hops set to the IPv6 address of ATE port-1 and ATE port-2.
	b := &gnmi.SetBatch{}
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
	} else if !deviations.IPv6StaticRouteWithIPv4NextHopUnsupported(td.dut) {
		v6Cfg = &cfgplugins.StaticRouteCfg{
			NetworkInstance: deviations.DefaultNetworkInstance(td.dut),
			Prefix:          td.staticIPv6.cidr(t),
			NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
				"0": oc.UnionString(atePort1.IPv4),
				"1": oc.UnionString(atePort2.IPv4),
			},
		}
	}
	if v6Cfg != nil {
		if _, err := cfgplugins.NewStaticRouteCfg(b, v6Cfg, td.dut); err != nil {
			t.Fatalf("Failed to configure IPv6 static route: %v", err)
		}
	}

	var v4Cfg *cfgplugins.StaticRouteCfg
	if !deviations.IPv4StaticRouteWithIPv6NextHopUnsupported(td.dut) {
		v4Cfg = &cfgplugins.StaticRouteCfg{
			NetworkInstance: deviations.DefaultNetworkInstance(td.dut),
			Prefix:          td.staticIPv4.cidr(t),
			NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
				"0": oc.UnionString(atePort1.IPv6),
				"1": oc.UnionString(atePort2.IPv6),
			},
		}
	}
	if v4Cfg != nil {
		if _, err := cfgplugins.NewStaticRouteCfg(b, v4Cfg, td.dut); err != nil {
			t.Fatalf("Failed to configure IPv4 static route: %v", err)
		}
	}
	// Step 4 - Push configuration
	b.Set(t, td.dut)

	defer td.deleteStaticRoutes(t)

	// Step 5 - Validate the routes are configured and the cross-family next-hops are reported correctly.
	t.Run("Telemetry", func(t *testing.T) {
		if !deviations.IPv6StaticRouteWithIPv4NextHopUnsupported(td.dut) {
			if deviations.IPv6StaticRouteWithIPv4NextHopRequiresStaticARP(td.dut) {
				t.Logf("Telemetry for v6 not validated due to use of deviation: IPv6StaticRouteWithIPv4NextHopRequiresStaticARP.")
			} else {
				gnmi.Await(t, td.dut, sp.Static(td.staticIPv6.cidr(t)).Prefix().State(), 30*time.Second, td.staticIPv6.cidr(t))
				gotStatic := gnmi.Get(t, td.dut, sp.Static(td.staticIPv6.cidr(t)).State())
				if got, want := gotStatic.GetNextHop("0").GetNextHop(), oc.UnionString(atePort1.IPv4); got != want {
					t.Errorf("IPv6 Static Route next hop: got: %s, want: %s", got, want)
				}
				if got, want := gotStatic.GetNextHop("1").GetNextHop(), oc.UnionString(atePort2.IPv4); got != want {
					t.Errorf("Static Route next hop: got: %s, want: %s", got, want)
				}
			}
		}

		if !deviations.IPv4StaticRouteWithIPv6NextHopUnsupported(td.dut) {
			gnmi.Await(t, td.dut, sp.Static(td.staticIPv4.cidr(t)).Prefix().State(), 30*time.Second, td.staticIPv4.cidr(t))
			if deviations.SkipStaticNexthopCheck(td.dut) {
				nexthops := gnmi.LookupAll(t, td.dut, sp.Static(td.staticIPv4.cidr(t)).NextHopAny().NextHop().State())
				if len(nexthops) != 2 {
					t.Errorf("IPv4 Static Route next hop: want %d nexthops,got %d nexthops", 2, len(nexthops))
				}
				for _, nexthop := range nexthops {
					if got, ok := nexthop.Val(); !ok || !(got == oc.UnionString(atePort1.IPv6) || got == oc.UnionString(atePort2.IPv6)) {
						t.Errorf("IPv4 Static Route next hop: got %s,want %s or %s", got, oc.UnionString(atePort1.IPv6), oc.UnionString(atePort2.IPv6))
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
		}
	})

	t.Run("Traffic", func(t *testing.T) {
		// Step 6 - Send Traffic from ATE port-3 towards both destinations.
		// Step 7 - Verify that traffic is received from DUT on both port-1 and port-2 and ECMP works for both XAF routes.
		td.ate.OTG().StartTraffic(t)
		time.Sleep(trafficDuration)
		td.ate.OTG().StopTraffic(t)

		otgutils.LogFlowMetrics(t, td.ate.OTG(), td.top)

		if !deviations.IPv4StaticRouteWithIPv6NextHopUnsupported(td.dut) {
			lossV4 := otgutils.GetFlowLossPct(t, td.ate.OTG(), v4Flow, 20*time.Second)
			if lossV4 > lossTolerance {
				t.Errorf("Loss percent for IPv4 Traffic: got: %f, want 0%%", lossV4)
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
			if got, want := p1Counter*100/(p1Counter+p2Counter), uint64(50); got < want-ecmpTolerance || got > want+ecmpTolerance {
				t.Errorf("ECMP IPv4 load balance error for port1, got: %v, want: %v", got, want)
			}
			if got, want := p2Counter*100/(p1Counter+p2Counter), uint64(50); got < want-ecmpTolerance || got > want+ecmpTolerance {
				t.Errorf("ECMP IPv4 load balance error for port2, got: %v, want: %v", got, want)
			}
		}

		if !deviations.IPv6StaticRouteWithIPv4NextHopUnsupported(td.dut) {
			lossV6 := otgutils.GetFlowLossPct(t, td.ate.OTG(), v6Flow, 20*time.Second)
			if lossV6 > lossTolerance {
				t.Errorf("Loss percent for IPv6 Traffic: got: %f, want 0%%", lossV6)
			}
			portCounters := egressTrackingCounters(t, td.ate, v6Flow)
			if len(portCounters) != 2 {
				t.Errorf("IPv6 egress tracking counters: got: %v, want: 2", len(portCounters))
			}
			p1Counter, ok := portCounters[port1Tag]
			if !ok {
				t.Errorf("Port1 IPv6 egress tracking counter not found: %v", portCounters)
			}
			p2Counter, ok := portCounters[port2Tag]
			if !ok {
				t.Errorf("Port2 IPv6 egress tracking counter not found: %v", portCounters)
			}
			if got, want := p1Counter*100/(p1Counter+p2Counter), uint64(50); got < want-ecmpTolerance || got > want+ecmpTolerance {
				t.Errorf("ECMP IPv6 load balance error for port1, got: %v, want: %v", got, want)
			}
			if got, want := p2Counter*100/(p1Counter+p2Counter), uint64(50); got < want-ecmpTolerance || got > want+ecmpTolerance {
				t.Errorf("ECMP IPv6 load balance error for port2, got: %v, want: %v", got, want)
			}
		}
	})
}

// staticARPWithMagicUniversalIP
// Objective: Helper function to populate static ARP entries for XAF cases requiring manual neighbor resolution.
// Traceability: RT-1.26.5 (Deviation helper)
// Technical Summary: Overrides static configuration to force a neighbor entry binding a dummy IPv6 address to a dummy MAC on the physical interface to allow routing across families on specific OSes.
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

// testStaticRouteWithDropNextHop
// Objective: Validates that configuring a static route to DROP successfully discards matching traffic.
// Traceability: RT-1.26.6 - Validate Static Route with DROP Next-Hop
// Technical Summary: Replaces next-hops with `LOCAL_DEFINED_NEXT_HOP_DROP`. Awaits convergence via `gnmi.Await` and validates that test traffic targeted at those destinations experiences 100% packet loss.
func (td *testData) testStaticRouteWithDropNextHop(t *testing.T) {
	if deviations.StaticRouteWithDropNhUnsupported(td.dut) {
		t.Skip("Skipping test static route with drop nexthop. Deviation StaticRouteWithDropNhUnsupported enabled.")
	}
	b := &gnmi.SetBatch{}
	// Step 1 - Configure an IPv4 static route for 203.0.113.0/24 by setting next-hop to DROP
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

	// Step 2 - Configure an IPv6 static route for 2001:db8:128:128::/64 by setting next-hop to DROP
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
	// Step 3 - Push configuration to DUT.
	b.Set(t, td.dut)

	defer td.deleteStaticRoutes(t)

	t.Run("Telemetry", func(t *testing.T) {
		if deviations.MissingStaticRouteDropNextHopTelemetry(td.dut) {
			t.Skip("Skipping telemetry check for DROP next hop. Deviation MissingStaticRouteDropNextHopTelemetryenabled.")
		}
		sp := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(td.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(td.dut))
		gnmi.Await(t, td.dut, sp.Static(td.staticIPv4.cidr(t)).Prefix().State(), 30*time.Second, td.staticIPv4.cidr(t))
		gnmi.Await(t, td.dut, sp.Static(td.staticIPv6.cidr(t)).Prefix().State(), 30*time.Second, td.staticIPv6.cidr(t))

		// Step 4 - Validate the route is configured and reported correctly.
		gotStatic := gnmi.Get(t, td.dut, sp.Static(td.staticIPv4.cidr(t)).State())
		if got, want := gotStatic.GetNextHop("0").GetNextHop(), oc.LocalRouting_LOCAL_DEFINED_NEXT_HOP_DROP; got != want {
			t.Errorf("IPv4 Static Route next hop: got: %s, want: %s", got, want)
		}
		// Step 4 - Validate the route is configured and reported correctly.
		gotStatic = gnmi.Get(t, td.dut, sp.Static(td.staticIPv6.cidr(t)).State())
		if got, want := gotStatic.GetNextHop("0").GetNextHop(), oc.LocalRouting_LOCAL_DEFINED_NEXT_HOP_DROP; got != want {
			t.Errorf("IPv6 Static Route next hop: got: %s, want: %s", got, want)
		}
	})

	t.Run("Traffic", func(t *testing.T) {
		// Step 4 - Send IPv4 and IPv6 Traffic from ATE port-3 towards destination `203.0.113.0/24` and `2001:db8:128:128::/64`.
		td.ate.OTG().StartTraffic(t)
		time.Sleep(trafficDuration)
		td.ate.OTG().StopTraffic(t)

		lossV4 := otgutils.GetFlowLossPct(t, td.ate.OTG(), v4Flow, 20*time.Second)
		lossV6 := otgutils.GetFlowLossPct(t, td.ate.OTG(), v6Flow, 20*time.Second)

		// Step 6 - Verify that traffic is dropped on DUT and not received on port-1 and port-2.
		otgutils.LogFlowMetrics(t, td.ate.OTG(), td.top)
		if lossV4 != 100 {
			t.Errorf("Loss percent for IPv4 Traffic: got: %f, want 100%%", lossV4)
		}
		if lossV6 != 100 {
			t.Errorf("Loss percent for IPv6 Traffic: got: %f, want 100%%", lossV6)
		}
	})
}

// egressTrackingCounters
// Objective: Helper function to retrieve flow egress tracking counters indexed by offset tag.
// Traceability: Validation/Helper
// Technical Summary: Queries OTG for metrics categorized by specific tagging schemas appended during flow transmission, allowing exact calculation of packet balancing on a per-port basis.
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

// configureOTGFlows
// Objective: Helper function to declare packet formats, flows, and metric tracking definitions on the OTG.
// Traceability: Setup/Helper
// Technical Summary: Assembles IPv4 and IPv6 traffic definitions bound to their respective source and destination endpoints, and appends Ethernet trailer tags to facilitate per-port egress metrics tracking.
func (td *testData) configureOTGFlows(t *testing.T) {
	t.Helper()

	srcV4 := td.otgP3.Ethernets().Items()[0].Ipv4Addresses().Items()[0]
	srcV6 := td.otgP3.Ethernets().Items()[0].Ipv6Addresses().Items()[0]

	dst1V4 := td.otgP1.Ethernets().Items()[0].Ipv4Addresses().Items()[0]
	dst1V6 := td.otgP1.Ethernets().Items()[0].Ipv6Addresses().Items()[0]
	dst2V4 := td.otgP2.Ethernets().Items()[0].Ipv4Addresses().Items()[0]
	dst2V6 := td.otgP2.Ethernets().Items()[0].Ipv6Addresses().Items()[0]
	dst3V4 := td.otgP3.Ethernets().Items()[0].Ipv4Addresses().Items()[0]
	dst4V4 := td.otgP4.Ethernets().Items()[0].Ipv4Addresses().Items()[0]

	v4F := td.top.Flows().Add()
	v4F.SetName(v4Flow).Metrics().SetEnable(true)
	v4F.TxRx().Device().SetTxNames([]string{srcV4.Name()}).SetRxNames([]string{dst1V4.Name(), dst2V4.Name(), dst3V4.Name(), dst4V4.Name()})

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

// awaitISISAdjacency
// Objective: Helper function to verify ISIS adjacency status between DUT and ATE.
// Traceability: Setup/Helper
// Technical Summary: Queries the IS-IS network instance adjacency state utilizing `gnmi.WatchAll` to await UP state, blocking test execution until routing domains converge.
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

// configureDUT
// Objective: Configures baseline DUT interfaces and physical attributes.
// Traceability: Setup/Helper
// Technical Summary: Provisions interface mappings, sets port speed configurations (handling FR breakout deviations), and establishes basic network-instance assignments for all participating ports.
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

// configureOTG
// Objective: Configures baseline ATE interfaces and physical attributes.
// Traceability: Setup/Helper
// Technical Summary: Instantiates gosnappi device topology definitions covering IPv4 and IPv6 properties mapped to the physical testbed links.
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

// advertiseRoutesWithISIS
// Objective: Establishes IS-IS peering and advertises emulated routes from the ATE to the DUT.
// Traceability: RT-1.26.7 (Step 1-2)
// Technical Summary: Configures ISIS instances, interfaces, metrics, and network types on the DUT. Mirrored settings are deployed on the OTG with configured background IPv4/IPv6 networks to guarantee recursive next-hop routes successfully map in the routing table.
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

// TestRT1269_DirectInterfaceIPDeletion
// Objective: Validates device resilience and route invalidation when a directly connected interface IP is deleted.
// Traceability: RT-1.26.9 - Direct Interface IP Deletion (Negative)
// Technical Summary: Deploys a static route using a direct interface reference. The interface IP is deleted, triggering `gnmi.Watch` validation for the removal event. It asserts 100% traffic loss and the inactive state of the static route without device failure.
func TestRT1269_DirectInterfaceIPDeletion(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	configureDUT(t, dut)

	ate := ondatra.ATE(t, "ate")
	top := gosnappi.NewConfig()
	devs := configureOTG(t, ate, top)

	td := testData{
		dut:   dut,
		ate:   ate,
		top:   top,
		otgP1: devs[0],
		otgP2: devs[1],
		otgP3: devs[2],
	}

	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	defer ate.OTG().StopProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")

	td.configureOTGFlows(t)

	// Subtest ID: RT-1.26.9 - Direct Interface IP Deletion (Negative)
	// Step 1 - Configure a static route that resolves via a directly connected interface IP.
	prefixV4 := ipAddr{address: v4Route, prefix: v4RoutePrefix}
	prefixV6 := ipAddr{address: v6Route, prefix: v6RoutePrefix}
	b := &gnmi.SetBatch{}
	sV4 := &cfgplugins.StaticRouteCfg{
		NetworkInstance: deviations.DefaultNetworkInstance(dut),
		Prefix:          prefixV4.cidr(t),
		NextHopIntf:     dut.Port(t, "port1").Name(),
	}
	if _, err := cfgplugins.NewStaticRouteCfg(b, sV4, dut); err != nil {
		t.Fatalf("Failed to configure IPv4 static route: %v", err)
	}
	b.Set(t, dut)
	cfgplugins.ValidateStaticRouteConfigured(t, dut, deviations.DefaultNetworkInstance(dut), prefixV4.cidr(t), sV4)

	sV6 := &cfgplugins.StaticRouteCfg{
		NetworkInstance: deviations.DefaultNetworkInstance(dut),
		Prefix:          prefixV6.cidr(t),
		NextHopIntf:     dut.Port(t, "port1").Name(),
	}
	if _, err := cfgplugins.NewStaticRouteCfg(b, sV6, dut); err != nil {
		t.Fatalf("Failed to configure IPv6 static route: %v", err)
	}
	b.Set(t, dut)
	cfgplugins.ValidateStaticRouteConfigured(t, dut, deviations.DefaultNetworkInstance(dut), prefixV6.cidr(t), sV6)

	// Step 2 - Delete the IP address of that direct interface using a gNMI Set DELETE.
	intfPath := gnmi.OC().Interface(dut.Port(t, "port1").Name())
	gnmi.Delete(t, dut, intfPath.Subinterface(0).Ipv4().Address(dutPort1.IPv4).Config())
	gnmi.Delete(t, dut, intfPath.Subinterface(0).Ipv6().Address(dutPort1.IPv6).Config())

	// Wait for the IPv4 and IPv6 addresses to be deleted from state
	_, ok := gnmi.Watch(t, dut, intfPath.Subinterface(0).Ipv4().Address(dutPort1.IPv4).State(), 30*time.Second, func(v *ygnmi.Value[*oc.Interface_Subinterface_Ipv4_Address]) bool {
		return !v.IsPresent()
	}).Await(t)
	if !ok {
		t.Errorf("Timeout waiting for interface IP %s to be deleted", dutPort1.IPv4)
	}
	_, ok = gnmi.Watch(t, dut, intfPath.Subinterface(0).Ipv6().Address(dutPort1.IPv6).State(), 30*time.Second, func(v *ygnmi.Value[*oc.Interface_Subinterface_Ipv6_Address]) bool {
		return !v.IsPresent()
	}).Await(t)
	if !ok {
		t.Errorf("Timeout waiting for interface IP %s to be deleted", dutPort1.IPv6)
	}

	// Step 3 - Send Traffic.
	td.ate.OTG().StartTraffic(t)
	time.Sleep(trafficDuration)
	td.ate.OTG().StopTraffic(t)

	// Step 4 - Verify the traffic drops, and the static route becomes inactive without crashing the device.
	lossV4 := otgutils.GetFlowLossPct(t, td.ate.OTG(), v4Flow, 20*time.Second)
	if lossV4 < 100 {
		t.Errorf("Loss percent for IPv4 Traffic after Intf IP deletion: got: %f, want 100%%", lossV4)
	}
	lossV6 := otgutils.GetFlowLossPct(t, td.ate.OTG(), v6Flow, 20*time.Second)
	if lossV6 < 100 {
		t.Errorf("Loss percent for IPv6 Traffic after Intf IP deletion: got: %f, want 100%%", lossV6)
	}

	sp := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
	_, ok = gnmi.Watch(t, dut, sp.Static(prefixV4.cidr(t)).State(), 30*time.Second, func(v *ygnmi.Value[*oc.NetworkInstance_Protocol_Static]) bool {
		return !v.IsPresent()
	}).Await(t)
	if !ok {
		t.Errorf("Timeout waiting for static route %s to become inactive after deleting Intf IP", prefixV4.cidr(t))
	}
	_, ok = gnmi.Watch(t, dut, sp.Static(prefixV6.cidr(t)).State(), 30*time.Second, func(v *ygnmi.Value[*oc.NetworkInstance_Protocol_Static]) bool {
		return !v.IsPresent()
	}).Await(t)
	if !ok {
		t.Errorf("Timeout waiting for static route %s to become inactive after deleting Intf IP", prefixV6.cidr(t))
	}
}

// TestRT12610_OverlappingPrefixesLPM
// Objective: Validates Longest Prefix Match (LPM) logic handles overlapping static routes accurately.
// Traceability: RT-1.26.10 - Overlapping Prefixes / LPM (Corner)
// Technical Summary: Configures two overlapping prefixes (a /8 and a /24) to distinct egress ports. Awaits telemetry via `gnmi.Await` and transmits traffic falling in the overlapped region, ensuring 100% traffic paths down the more specific /24 route.
func TestRT12610_OverlappingPrefixesLPM(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	configureDUT(t, dut)

	ate := ondatra.ATE(t, "ate")
	top := gosnappi.NewConfig()
	devs := configureOTG(t, ate, top)

	td := testData{
		dut:   dut,
		ate:   ate,
		top:   top,
		otgP1: devs[0],
		otgP2: devs[1],
		otgP3: devs[2],
	}

	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	defer ate.OTG().StopProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")

	// Special Flow Configuration pointing to 10.1.1.1
	srcV4 := td.otgP3.Ethernets().Items()[0].Ipv4Addresses().Items()[0]
	dst1V4 := td.otgP1.Ethernets().Items()[0].Ipv4Addresses().Items()[0]
	dst2V4 := td.otgP2.Ethernets().Items()[0].Ipv4Addresses().Items()[0]

	v4F := td.top.Flows().Add()
	v4F.SetName("LPMFlow").Metrics().SetEnable(true)
	v4F.TxRx().Device().SetTxNames([]string{srcV4.Name()}).SetRxNames([]string{dst1V4.Name(), dst2V4.Name()})

	v4FEth := v4F.Packet().Add().Ethernet()
	v4FEth.Src().SetValue(atePort3.MAC)

	v4FIp := v4F.Packet().Add().Ipv4()
	v4FIp.Src().SetValue(srcV4.Address())
	v4FIp.Dst().SetValue("10.1.1.1") // Target specific IP in overlapping region

	eth := v4F.EgressPacket().Add().Ethernet()
	ethTag := eth.Dst().MetricTags().Add()
	ethTag.SetName("MACTrackingv4").SetOffset(40).SetLength(8)

	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	defer ate.OTG().StopProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")

	// Subtest ID: RT-1.26.10 - Overlapping Prefixes / LPM (Corner)
	// Step 1 - Configure overlapping static routes: 10.0.0.0/8 pointing to ATE port-1, and 10.1.1.0/24 pointing to ATE port-2.
	b := &gnmi.SetBatch{}
	sV4_8 := &cfgplugins.StaticRouteCfg{
		NetworkInstance: deviations.DefaultNetworkInstance(dut),
		Prefix:          "10.0.0.0/8",
		NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
			"0": oc.UnionString(atePort1.IPv4),
		},
	}
	if _, err := cfgplugins.NewStaticRouteCfg(b, sV4_8, dut); err != nil {
		t.Fatalf("Failed to configure 10.0.0.0/8 route: %v", err)
	}
	sV4_24 := &cfgplugins.StaticRouteCfg{
		NetworkInstance: deviations.DefaultNetworkInstance(dut),
		Prefix:          "10.1.1.0/24",
		NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
			"0": oc.UnionString(atePort2.IPv4),
		},
	}
	if _, err := cfgplugins.NewStaticRouteCfg(b, sV4_24, dut); err != nil {
		t.Fatalf("Failed to configure 10.1.1.0/24 route: %v", err)
	}

	// Step 2 - Push configuration to DUT.
	b.Set(t, dut)

	// Validate telemetry before proceeding
	sp := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
	gnmi.Await(t, dut, sp.Static("10.0.0.0/8").Prefix().State(), 120*time.Second, "10.0.0.0/8")
	gnmi.Await(t, dut, sp.Static("10.1.1.0/24").Prefix().State(), 120*time.Second, "10.1.1.0/24")

	// Step 3 - Send traffic to destination 10.1.1.1.
	td.ate.OTG().StartTraffic(t)
	time.Sleep(30 * time.Second)
	td.ate.OTG().StopTraffic(t)

	// Step 4 - Verify that traffic strictly adheres to Longest Prefix Match (LPM) routing and flows exclusively to ATE port-2.
	lossV4 := otgutils.GetFlowLossPct(t, td.ate.OTG(), "LPMFlow", 20*time.Second)
	if lossV4 > lossTolerance {
		t.Errorf("Loss percent for IPv4 Traffic: got: %f, want 0%%", lossV4)
	}

	portCounters := egressTrackingCounters(t, td.ate, "LPMFlow")
	p2Counter := portCounters[port2Tag]
	p1Counter := portCounters[port1Tag]

	if p1Counter > 0 {
		t.Errorf("Traffic leaked to port 1 (10.0.0.0/8 route) - LPM failure: got %v packets", p1Counter)
	}
	if p2Counter == 0 {
		t.Errorf("No traffic received on port 2 (10.1.1.0/24 route) expected for LPM resolution.")
	}
}

// TestRT12611_RouteResolutionLoop
// Objective: Validates the DUT prevention or mitigation of recursive route resolution loops.
// Traceability: RT-1.26.11 - Route Resolution Loop (Negative)
// Technical Summary: Statically routes Prefix A to Next-hop B, and Prefix B to Next-hop A. Pauses for potential loops, and asserts control plane health by fetching fundamental interface operative states and validating gNMI responsivity.
func TestRT12611_RouteResolutionLoop(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	configureDUT(t, dut)

	ate := ondatra.ATE(t, "ate")
	top := gosnappi.NewConfig()
	configureOTG(t, ate, top)

	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	defer ate.OTG().StopProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")

	// Subtest ID: RT-1.26.11 - Route Resolution Loop (Negative)
	// Step 1 - Configure Static Route A pointing to Next-Hop IP B.
	b := &gnmi.SetBatch{}
	routeA := "198.51.100.201/32"
	nextHopB := "198.51.100.202"
	sA := &cfgplugins.StaticRouteCfg{
		NetworkInstance: deviations.DefaultNetworkInstance(dut),
		Prefix:          routeA,
		NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
			"0": oc.UnionString(nextHopB),
		},
	}
	if _, err := cfgplugins.NewStaticRouteCfg(b, sA, dut); err != nil {
		t.Fatalf("Failed to configure IPv4 static route A: %v", err)
	}

	routeAv6 := "2001:db8::201/128"
	nextHopBv6 := "2001:db8::202"
	sAv6 := &cfgplugins.StaticRouteCfg{
		NetworkInstance: deviations.DefaultNetworkInstance(dut),
		Prefix:          routeAv6,
		NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
			"0": oc.UnionString(nextHopBv6),
		},
	}
	if _, err := cfgplugins.NewStaticRouteCfg(b, sAv6, dut); err != nil {
		t.Fatalf("Failed to configure IPv6 static route A: %v", err)
	}

	// Step 2 - Configure Static Route B pointing to Next-Hop IP A.
	routeB := "198.51.100.202/32"
	nextHopA := "198.51.100.201"
	sB := &cfgplugins.StaticRouteCfg{
		NetworkInstance: deviations.DefaultNetworkInstance(dut),
		Prefix:          routeB,
		NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
			"0": oc.UnionString(nextHopA),
		},
	}
	if _, err := cfgplugins.NewStaticRouteCfg(b, sB, dut); err != nil {
		t.Fatalf("Failed to configure IPv4 static route B: %v", err)
	}

	routeBv6 := "2001:db8::202/128"
	nextHopAv6 := "2001:db8::201"
	sBv6 := &cfgplugins.StaticRouteCfg{
		NetworkInstance: deviations.DefaultNetworkInstance(dut),
		Prefix:          routeBv6,
		NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
			"0": oc.UnionString(nextHopAv6),
		},
	}
	if _, err := cfgplugins.NewStaticRouteCfg(b, sBv6, dut); err != nil {
		t.Fatalf("Failed to configure IPv6 static route B: %v", err)
	}

	// Step 3 - Push configuration to DUT.
	b.Set(t, dut)

	// Step 4 - Verify the device's control plane detects or breaks the recursion loop safely without hanging or crashing.
	// Allow time for device control plane to potentially loop
	time.Sleep(30 * time.Second)

	// Query device interface state to ensure control plane is responsive
	operStatus := gnmi.Get(t, dut, gnmi.OC().Interface(dut.Port(t, "port1").Name()).OperStatus().State())
	if operStatus == oc.Interface_OperStatus_UNSET {
		t.Errorf("Device interface operational status unretrievable - possible control plane hang")
	}

	// Make sure we can retrieve the static routes
	gotStaticA := gnmi.Get(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut)).Static(routeA).State())
	if gotStaticA == nil || gotStaticA.GetPrefix() != routeA {
		t.Errorf("Failed to retrieve State for Route A, control plane may be unresponsive")
	}

	gotStaticAv6 := gnmi.Get(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut)).Static(routeAv6).State())
	if gotStaticAv6 == nil || gotStaticAv6.GetPrefix() != routeAv6 {
		t.Errorf("Failed to retrieve State for IPv6 Route A, control plane may be unresponsive")
	}
}
