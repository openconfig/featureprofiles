// Copyright 2026 Google LLC
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

// Package bgp_confederation_test implements RT-1.111: BGP Autonomous System
// Confederations (RFC 5065). The procedure is described in README.md in this
// directory.
package bgp_confederation_test

import (
	"fmt"
	"net/netip"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	otgconfighelpers "github.com/openconfig/featureprofiles/internal/otg_helpers/otg_config_helpers"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/gnmi/oc/netinstbgp"
	otgtelemetry "github.com/openconfig/ondatra/gnmi/otg"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygnmi/ygnmi"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// AS numbers and identifiers from the README "DUT and OTG properties" table.
const (
	dutAS        = 64501 // DUT global AS, which is its confederation Member-AS.
	confedID     = 64500 // Confederation identifier.
	confedPeerAS = 64502 // Member-AS of ATE R3, configured in the DUT member-as.
	extPeerAS    = 64510 // AS of ATE R1, outside the confederation.

	dutRouterID = "192.0.2.254"
	bgpInstance = "BGP"   // BGP protocol name from the README Canonical OC.
	allowPolicy = "ALLOW" // Accept-all import and export policy for every peer.

	plenIPv4 = 30
	plenIPv6 = 126
)

// Traffic profile from the README "Testing steps" and "Test pass fail
// criteria": every flow runs for 30 seconds and may lose at most 1% of its
// packets.
const (
	trafficSeconds = 30
	trafficPPS     = 1000
	trafficPackets = trafficSeconds * trafficPPS
	frameSize      = 256
	maxLossPct     = 1.0
)

// Upper bounds for the gNMI watches.
const (
	stateTimeout   = time.Minute
	sessionTimeout = 3 * time.Minute
	routeTimeout   = 2 * time.Minute
	trafficTimeout = trafficSeconds*time.Second + time.Minute
)

// family is an address family exercised by the test.
type family string

const (
	ipv4 family = "IPv4"
	ipv6 family = "IPv6"
)

var families = []family{ipv4, ipv6}

// afiSafi returns the BGP unicast AFI-SAFI of the address family.
func (f family) afiSafi() oc.E_BgpTypes_AFI_SAFI_TYPE {
	if f == ipv6 {
		return oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST
	}
	return oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST
}

// ateRouter is one emulated ATE BGP router of the README topology together
// with the DUT port it connects to.
type ateRouter struct {
	name    string // README name: R1, R2, or R3.
	port    string // Testbed port ID, the same on the DUT and the ATE.
	asn     uint32
	ibgp    bool
	dutAttr attrs.Attributes
	ateAttr attrs.Attributes
}

// addr returns the ATE router address, which is the DUT BGP neighbor address.
func (r *ateRouter) addr(f family) string {
	if f == ipv6 {
		return r.ateAttr.IPv6
	}
	return r.ateAttr.IPv4
}

// otgIntf returns the name of the OTG IP interface that attrs.AddToOTG creates.
func (r *ateRouter) otgIntf(f family) string {
	return r.ateAttr.Name + "." + string(f)
}

// peerName returns the name of the OTG BGP peer of the address family.
func (r *ateRouter) peerName(f family) string {
	return r.ateAttr.Name + ".BGP." + string(f)
}

// peerGroup returns the DUT peer-group of the address family. Each peer group
// carries a single AFI-SAFI so that every session negotiates one family.
func (r *ateRouter) peerGroup(f family) string {
	return r.name + "-" + string(f)
}

func (r *ateRouter) String() string {
	return fmt.Sprintf("ATE %s (%s)", r.name, r.port)
}

var (
	r1 = &ateRouter{
		name: "R1",
		port: "port1",
		asn:  extPeerAS,
		dutAttr: attrs.Attributes{
			Desc:    "DUT to ATE R1 (external eBGP)",
			IPv4:    "192.0.2.1",
			IPv4Len: plenIPv4,
			IPv6:    "2001:db8::1",
			IPv6Len: plenIPv6,
		},
		ateAttr: attrs.Attributes{
			Name:    "R1",
			MAC:     "02:00:01:01:01:01",
			IPv4:    "192.0.2.2",
			IPv4Len: plenIPv4,
			IPv6:    "2001:db8::2",
			IPv6Len: plenIPv6,
		},
	}
	r2 = &ateRouter{
		name: "R2",
		port: "port2",
		asn:  dutAS,
		ibgp: true,
		dutAttr: attrs.Attributes{
			Desc:    "DUT to ATE R2 (iBGP)",
			IPv4:    "192.0.2.5",
			IPv4Len: plenIPv4,
			IPv6:    "2001:db8::5",
			IPv6Len: plenIPv6,
		},
		ateAttr: attrs.Attributes{
			Name:    "R2",
			MAC:     "02:00:02:01:01:01",
			IPv4:    "192.0.2.6",
			IPv4Len: plenIPv4,
			IPv6:    "2001:db8::6",
			IPv6Len: plenIPv6,
		},
	}
	r3 = &ateRouter{
		name: "R3",
		port: "port3",
		asn:  confedPeerAS,
		dutAttr: attrs.Attributes{
			Desc:    "DUT to ATE R3 (confederation eBGP)",
			IPv4:    "192.0.2.9",
			IPv4Len: plenIPv4,
			IPv6:    "2001:db8::9",
			IPv6Len: plenIPv6,
		},
		ateAttr: attrs.Attributes{
			Name:    "R3",
			MAC:     "02:00:03:01:01:01",
			IPv4:    "192.0.2.10",
			IPv4Len: plenIPv4,
			IPv6:    "2001:db8::a",
			IPv6Len: plenIPv6,
		},
	}
	routers = []*ateRouter{r1, r2, r3}
)

// advertisedRoute is one row of the README route table.
type advertisedRoute struct {
	name     string     // README name: EXT, IBGP, or CONFED.
	router   *ateRouter // ATE router that advertises the prefixes.
	v4Prefix string
	v6Prefix string
	// asSetMode is how the ATE router adds its own AS to the AS_PATH that it
	// advertises, which gives the README AS_PATH: an AS_SEQUENCE for the
	// external peer, an AS_CONFED_SEQUENCE for the confederation peer, and
	// nothing for the iBGP peer.
	asSetMode gosnappi.BgpAsPathAsSetModeEnum
}

// prefix returns the advertised prefix of the address family.
func (a *advertisedRoute) prefix(f family) string {
	if f == ipv6 {
		return a.v6Prefix
	}
	return a.v4Prefix
}

// otgName returns the name of the OTG route range of the address family.
func (a *advertisedRoute) otgName(f family) string {
	return a.name + "." + string(f)
}

// routeOptions returns the OTG route range options: the ATE address as the
// next hop and the AS_PATH from the README route table.
func (a *advertisedRoute) routeOptions(f family) []otgconfighelpers.BGPRouteOption {
	opts := []otgconfighelpers.BGPRouteOption{
		otgconfighelpers.WithBGPRouteNextHopIPv4(a.router.ateAttr.IPv4),
	}
	if f == ipv6 {
		opts = []otgconfighelpers.BGPRouteOption{
			otgconfighelpers.WithBGPRouteNextHopIPv6(a.router.ateAttr.IPv6),
		}
	}
	// The README AS_PATH is built from the ATE AS only, without extra
	// segments: on an eBGP session the ATE adds its own AS even when
	// DO_NOT_INCLUDE_LOCAL_AS is set, so an extra segment would be advertised
	// after it.
	asPath := gosnappi.NewBgpAsPath().SetAsSetMode(a.asSetMode)
	return append(opts, otgconfighelpers.WithBGPRouteASPath(asPath))
}

var (
	extRoute = &advertisedRoute{
		name:      "EXT",
		router:    r1,
		v4Prefix:  "203.0.113.0/26",
		v6Prefix:  "2001:db8:100::/48",
		asSetMode: gosnappi.BgpAsPathAsSetMode.INCLUDE_AS_SEQ, // AS_SEQ [64510]
	}
	ibgpRoute = &advertisedRoute{
		name:      "IBGP",
		router:    r2,
		v4Prefix:  "198.51.100.0/26",
		v6Prefix:  "2001:db8:200::/48",
		asSetMode: gosnappi.BgpAsPathAsSetMode.DO_NOT_INCLUDE_LOCAL_AS, // Empty AS_PATH.
	}
	confedRoute = &advertisedRoute{
		name:      "CONFED",
		router:    r3,
		v4Prefix:  "198.51.100.64/26",
		v6Prefix:  "2001:db8:300::/48",
		asSetMode: gosnappi.BgpAsPathAsSetMode.INCLUDE_AS_CONFED_SEQ, // AS_CONFED_SEQUENCE [64502]
	}
	advertisedRoutes = []*advertisedRoute{extRoute, ibgpRoute, confedRoute}
)

// asPathCheck is one README verification of a received AS_PATH. A nil at
// checks the DUT loc-rib; otherwise it checks the routes learned by that ATE
// router.
type asPathCheck struct {
	route *advertisedRoute
	at    *ateRouter
	want  string // Expected AS_PATH in the README notation.
}

// flowPath is one README traffic direction, from an ATE router towards a route
// advertised by another ATE router. It is run for IPv4 and IPv6.
type flowPath struct {
	src *ateRouter
	dst *advertisedRoute
}

// trafficFlow is a flowPath in one address family.
type trafficFlow struct {
	flowPath
	fam family
}

// name returns the OTG flow name.
func (f trafficFlow) name() string {
	return fmt.Sprintf("%s_%s_to_%s", f.fam, f.src.name, f.dst.name)
}

// propagationCase is one of the README subtests RT-1.111.2 to RT-1.111.5.
type propagationCase struct {
	name      string
	advertise []*advertisedRoute
	checks    []asPathCheck
	traffic   []flowPath
}

// propagationCases lists RT-1.111.2 to RT-1.111.5 in README order. The
// subtests share one environment and run sequentially: RT-1.111.4 has no
// advertisement step and uses the routes advertised by RT-1.111.2 and
// RT-1.111.3.
var propagationCases = []propagationCase{
	{
		name:      "RT-1.111.2: Confederation peer to iBGP peer route propagation",
		advertise: []*advertisedRoute{confedRoute},
		checks: []asPathCheck{
			{route: confedRoute, want: "AS_CONFED_SEQUENCE [64502]"},
			{route: confedRoute, at: r2, want: "AS_CONFED_SEQUENCE [64502]"},
		},
		traffic: []flowPath{{src: r2, dst: confedRoute}},
	},
	{
		name:      "RT-1.111.3: iBGP peer to confederation peer route propagation",
		advertise: []*advertisedRoute{ibgpRoute},
		checks: []asPathCheck{
			{route: ibgpRoute, at: r3, want: "AS_CONFED_SEQUENCE [64501]"},
		},
		traffic: []flowPath{{src: r3, dst: ibgpRoute}},
	},
	{
		name: "RT-1.111.4: Confederation routes to external eBGP peer",
		checks: []asPathCheck{
			{route: confedRoute, at: r1, want: "AS_SEQ [64500]"},
			{route: ibgpRoute, at: r1, want: "AS_SEQ [64500]"},
		},
		traffic: []flowPath{{src: r1, dst: ibgpRoute}, {src: r1, dst: confedRoute}},
	},
	{
		name:      "RT-1.111.5: External route to confederation and iBGP peers",
		advertise: []*advertisedRoute{extRoute},
		checks: []asPathCheck{
			{route: extRoute, at: r3, want: "AS_CONFED_SEQUENCE [64501], AS_SEQ [64510]"},
			{route: extRoute, at: r2, want: "AS_SEQ [64510]"},
		},
		traffic: []flowPath{{src: r2, dst: extRoute}, {src: r3, dst: extRoute}},
	},
}

// flows returns the IPv4 and IPv6 traffic flows of the subtest.
func (tc propagationCase) flows() []trafficFlow {
	var flows []trafficFlow
	for _, p := range tc.traffic {
		for _, f := range families {
			flows = append(flows, trafficFlow{flowPath: p, fam: f})
		}
	}
	return flows
}

// trafficDescription returns the README wording of the traffic direction, for
// example "from port2 to port3".
func (tc propagationCase) trafficDescription() string {
	var srcs, dsts []string
	for _, p := range tc.traffic {
		if !slices.Contains(srcs, p.src.port) {
			srcs = append(srcs, p.src.port)
		}
		if d := p.dst.router.port; !slices.Contains(dsts, d) {
			dsts = append(dsts, d)
		}
	}
	return fmt.Sprintf("from %s to %s", strings.Join(srcs, " and "), strings.Join(dsts, " and "))
}

// testEnv is the environment shared by all subtests.
type testEnv struct {
	dut *ondatra.DUTDevice
	ate *ondatra.ATEDevice
	top gosnappi.Config
}

func TestBGPConfederation(t *testing.T) {
	env := setupEnvironment(t)

	allEstablished := false
	t.Run("RT-1.111.1: Confederation configuration, state and session establishment", func(t *testing.T) {
		allEstablished = verifyStateAndSessions(t, env.dut)
	})
	if !allEstablished {
		t.Fatal("Not all BGP sessions are ESTABLISHED; RT-1.111.2 to RT-1.111.5 cannot run")
	}

	for _, tc := range propagationCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.run(t, env)
		})
	}
}

// setupEnvironment applies the README Initial Setup. Each README Cleanup step
// is registered with t.Cleanup before the configuration that it reverts is
// pushed, so that it also runs when a push fails part-way.
func setupEnvironment(t *testing.T) *testEnv {
	t.Helper()
	env := &testEnv{
		dut: ondatra.DUT(t, "dut"),
		ate: ondatra.ATE(t, "ate"),
	}

	t.Log("Initial Setup: connect DUT port-1, port-2, and port-3 to OTG port-1, port-2, and port-3")
	for _, r := range routers {
		t.Logf("%v: DUT port %s, ATE port %s", r, env.dut.Port(t, r.port).Name(), env.ate.Port(t, r.port).ID())
	}

	t.Log("Initial Setup: configure OTG interfaces, BGP peers, route ranges, and IPv4/IPv6 traffic flows, and start OTG protocols")
	env.top = configureATE(t, env.ate)
	otg := env.ate.OTG()
	t.Cleanup(func() {
		t.Log("Cleanup: stop OTG traffic and protocols")
		otg.StopTraffic(t)
		otg.StopProtocols(t)
	})
	otg.PushConfig(t, env.top)
	otg.StartProtocols(t)
	// Each subtest advertises its own route prefixes, so none is advertised
	// before RT-1.111.2. Stopping the OTG protocols in Cleanup withdraws them.
	controlRoutes(t, otg, gosnappi.StateProtocolRouteState.WITHDRAW, advertisedRoutes...)

	t.Log("Initial Setup: configure DUT interfaces and BGP in the DEFAULT network instance")
	configureDUTInterfaces(t, env.dut)
	configureDUTBGP(t, env.dut)

	otgutils.WaitForARP(t, otg, env.top, "IPv4")
	otgutils.WaitForARP(t, otg, env.top, "IPv6")
	return env
}

// configureATE builds the OTG configuration: one device per ATE router with an
// IPv4 and an IPv6 BGP peer, the README route ranges, and all traffic flows.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	t.Helper()
	top := gosnappi.NewConfig()
	for _, r := range routers {
		dev := r.ateAttr.AddToOTG(top, ate.Port(t, r.port), &r.dutAttr)
		sessionType := otgconfighelpers.WithBGPEBGP()
		if r.ibgp {
			sessionType = otgconfighelpers.WithBGPIBGP()
		}
		peer4 := otgconfighelpers.AddBGPV4Peer(dev, r.otgIntf(ipv4),
			otgconfighelpers.WithBGPName(r.peerName(ipv4)),
			otgconfighelpers.WithBGPPeerAddress(r.dutAttr.IPv4),
			otgconfighelpers.WithBGPASNumber(r.asn),
			otgconfighelpers.WithBGPRouterID(r.ateAttr.IPv4),
			otgconfighelpers.WithoutBGPGR(),
			otgconfighelpers.WithBGPLearnedV4Pfx(true),
			sessionType,
		)
		peer6 := otgconfighelpers.AddBGPV6Peer(dev, r.otgIntf(ipv6),
			otgconfighelpers.WithBGPName(r.peerName(ipv6)),
			otgconfighelpers.WithBGPPeerAddress(r.dutAttr.IPv6),
			otgconfighelpers.WithBGPASNumber(r.asn),
			otgconfighelpers.WithBGPRouterID(r.ateAttr.IPv4),
			otgconfighelpers.WithoutBGPGR(),
			otgconfighelpers.WithBGPLearnedV6Pfx(true),
			sessionType,
		)
		for _, a := range advertisedRoutes {
			if a.router != r {
				continue
			}
			otgconfighelpers.AddBGPV4Routes(peer4, a.otgName(ipv4), []string{a.v4Prefix}, a.routeOptions(ipv4)...)
			otgconfighelpers.AddBGPV6Routes(peer6, a.otgName(ipv6), []string{a.v6Prefix}, a.routeOptions(ipv6)...)
		}
	}
	for _, tc := range propagationCases {
		for _, f := range tc.flows() {
			addFlow(t, top, f)
		}
	}
	return top
}

// addFlow adds a flow that sends trafficPackets packets at trafficPPS, which
// takes trafficSeconds, to the first host address of the destination route.
func addFlow(t *testing.T, top gosnappi.Config, f trafficFlow) {
	t.Helper()
	flow := &otgconfighelpers.Flow{
		TxNames:       []string{f.src.otgIntf(f.fam)},
		RxNames:       []string{f.dst.router.otgIntf(f.fam)},
		FrameSize:     frameSize,
		FlowName:      f.name(),
		PpsRate:       trafficPPS,
		PacketsToSend: trafficPackets,
		EthFlow:       &otgconfighelpers.EthFlowParams{SrcMAC: f.src.ateAttr.MAC},
	}
	flow.CreateFlow(top)
	flow.AddEthHeader()
	dst := firstHost(t, f.dst.prefix(f.fam))
	if f.fam == ipv6 {
		flow.IPv6Flow = &otgconfighelpers.IPv6FlowParams{IPv6Src: f.src.ateAttr.IPv6, IPv6Dst: dst}
		flow.AddIPv6Header()
		return
	}
	flow.IPv4Flow = &otgconfighelpers.IPv4FlowParams{IPv4Src: f.src.ateAttr.IPv4, IPv4Dst: dst}
	flow.AddIPv4Header()
}

// configureDUTInterfaces configures the DUT ports from the README table and
// waits until their state matches the configuration. The pre-test interface
// configuration is captured first and restored in Cleanup.
func configureDUTInterfaces(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	ni := deviations.DefaultNetworkInstance(dut)
	restore := &gnmi.SetBatch{}
	batch := &gnmi.SetBatch{}
	for _, r := range routers {
		name := dut.Port(t, r.port).Name()
		intf := gnmi.OC().Interface(name)
		if orig, ok := gnmi.Lookup(t, dut, intf.Config()).Val(); ok {
			gnmi.BatchReplace(restore, intf.Config(), orig)
		} else {
			gnmi.BatchDelete(restore, intf.Config())
		}
		if deviations.ExplicitInterfaceInDefaultVRF(dut) {
			// fptest.AssignToNetworkInstance uses the ID "<interface>.<subinterface>".
			niIntf := gnmi.OC().NetworkInstance(ni).Interface(name + ".0")
			if _, ok := gnmi.Lookup(t, dut, niIntf.Config()).Val(); !ok {
				gnmi.BatchDelete(restore, niIntf.Config())
			}
		}
		gnmi.BatchReplace(batch, intf.Config(), r.dutAttr.NewOCInterface(name, dut))
	}
	t.Cleanup(func() {
		t.Log("Cleanup: restore the pre-test DUT interface configuration")
		restore.Set(t, dut)
	})
	batch.Set(t, dut)

	// The DEFAULT network instance always exists; this idempotent update only
	// sets its type, so it is not reverted.
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	for _, r := range routers {
		p := dut.Port(t, r.port)
		if deviations.ExplicitPortSpeed(dut) {
			fptest.SetPortSpeed(t, p)
		}
		if deviations.ExplicitInterfaceInDefaultVRF(dut) {
			fptest.AssignToNetworkInstance(t, dut, p.Name(), ni, 0)
		}
	}
	for _, r := range routers {
		awaitInterfaceState(t, dut, dut.Port(t, r.port).Name(), &r.dutAttr)
	}
}

// awaitInterfaceState waits until the DUT port is up and reports the configured
// IPv4 and IPv6 addresses.
func awaitInterfaceState(t *testing.T, dut *ondatra.DUTDevice, name string, a *attrs.Attributes) {
	t.Helper()
	intf := gnmi.OC().Interface(name)
	gnmi.Await(t, dut, intf.OperStatus().State(), stateTimeout, oc.Interface_OperStatus_UP)
	sub := intf.Subinterface(0)
	gnmi.Await(t, dut, sub.Ipv4().Address(a.IPv4).PrefixLength().State(), stateTimeout, a.IPv4Len)
	gnmi.Await(t, dut, sub.Ipv6().Address(a.IPv6).PrefixLength().State(), stateTimeout, a.IPv6Len)
}

// configureDUTBGP configures the ALLOW policy and BGP with global AS 64501,
// router ID 192.0.2.254, confederation identifier 64500, member-as [64502],
// and the IPv4 and IPv6 unicast neighbors of R1, R2, and R3.
func configureDUTBGP(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	batch := &gnmi.SetBatch{}
	if _, err := cfgplugins.ConfigureBGPRoutePolicy(t, batch, cfgplugins.BGPPolicyConfig{PolicyName: allowPolicy, StatementID: "accept-all"}); err != nil {
		t.Fatalf("ConfigureBGPRoutePolicy(%q) failed: %v", allowPolicy, err)
	}
	proto := cfgplugins.ConfigureDUTBGP(t, dut, batch, cfgplugins.BGPConfig{DutAS: dutAS, RouterID: dutRouterID})
	bgp := proto.GetOrCreateBgp()
	// cfgplugins has no confederation helper, so the confederation container
	// is set directly.
	confed := bgp.GetOrCreateGlobal().GetOrCreateConfederation()
	confed.SetIdentifier(confedID)
	confed.SetMemberAs([]uint32{confedPeerAS})
	for _, r := range routers {
		for _, f := range families {
			pg := r.peerGroup(f)
			cfgplugins.ConfigurePeerGroup(bgp.GetOrCreatePeerGroup(pg), dut,
				cfgplugins.WithPeerAS(r.asn),
				cfgplugins.WithPGAfiSafiEnabled(f.afiSafi(), true, false),
				cfgplugins.ApplyPGRoutingPolicy(allowPolicy, allowPolicy, false),
			)
			cfgplugins.ConfigurePeer(bgp.GetOrCreateNeighbor(r.addr(f)), dut,
				cfgplugins.WithPeerGroup(pg, r.asn, allowPolicy, allowPolicy, false),
				cfgplugins.WithPeerAfiSafiEnabled(f == ipv4, allowPolicy, allowPolicy, false),
				cfgplugins.ApplyPeerPerAfiSafiRoutingPolicy(f == ipv4, allowPolicy, allowPolicy, false),
			)
		}
	}
	// Cleanups run in reverse order: BGP is deleted before the policy it uses.
	t.Cleanup(func() {
		t.Logf("Cleanup: delete the DUT routing policy %s", allowPolicy)
		gnmi.Delete(t, dut, gnmi.OC().RoutingPolicy().PolicyDefinition(allowPolicy).Config())
	})
	t.Cleanup(func() {
		t.Log("Cleanup: delete the DUT BGP configuration")
		cfgplugins.BGPClearConfig(t, dut)
	})
	batch.Set(t, dut)
	gnmi.Await(t, dut, gnmi.OC().RoutingPolicy().PolicyDefinition(allowPolicy).Name().State(), stateTimeout, allowPolicy)

	// Some devices stream the BGP RIB paths of the README only after a native
	// telemetry setting. The setting only enables telemetry and is often part
	// of the device baseline, so it is not reverted.
	if deviations.BgpRibStreamingConfigRequired(dut) {
		cfgplugins.DeviationBgpRibStreamingConfigRequired(t, dut)
	}
}

// onChange returns DUT gNMI options for ON_CHANGE subscriptions, which the
// README lists in its RPC coverage.
func onChange(dut *ondatra.DUTDevice) *gnmi.Opts {
	return dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_ON_CHANGE))
}

// bgpPath returns the path of the DUT BGP instance in the DEFAULT network
// instance.
func bgpPath(dut *ondatra.DUTDevice) *netinstbgp.NetworkInstance_Protocol_BgpPath {
	return gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInstance).Bgp()
}

// verifyStateAndSessions implements RT-1.111.1. It reports whether all IPv4 and
// IPv6 BGP sessions reached ESTABLISHED.
func verifyStateAndSessions(t *testing.T, dut *ondatra.DUTDevice) bool {
	t.Helper()
	wantMembers := []uint32{confedPeerAS}
	t.Logf("Verify DUT BGP global AS (%d), confederation identifier (%d), and member-as (%v)", dutAS, confedID, wantMembers)
	// The as leaf and the confederation container are watched instead of the
	// whole global container, which also holds counters that some devices do
	// not stream ON_CHANGE.
	global := bgpPath(dut).Global()
	asVal, ok := gnmi.Watch(t, onChange(dut), global.As().State(), stateTimeout, func(v *ygnmi.Value[uint32]) bool {
		as, present := v.Val()
		return present && as == dutAS
	}).Await(t)
	if !ok {
		var got uint32
		if asVal != nil {
			got, _ = asVal.Val()
		}
		t.Errorf("BGP global state as got %d, want %d", got, dutAS)
	}
	confedVal, ok := gnmi.Watch(t, onChange(dut), global.Confederation().State(), stateTimeout, func(v *ygnmi.Value[*oc.NetworkInstance_Protocol_Bgp_Global_Confederation]) bool {
		c, present := v.Val()
		return present && c.GetIdentifier() == confedID && sameASNs(c.GetMemberAs(), wantMembers)
	}).Await(t)
	if !ok {
		var got *oc.NetworkInstance_Protocol_Bgp_Global_Confederation
		if confedVal != nil {
			got, _ = confedVal.Val()
		}
		t.Errorf("BGP confederation state got identifier %d, member-as %v; want identifier %d, member-as %v",
			got.GetIdentifier(), got.GetMemberAs(), confedID, wantMembers)
	}

	wantSessions := len(routers) * len(families)
	t.Logf("Verify all %d IPv4 and IPv6 BGP sessions are ESTABLISHED", wantSessions)
	type sessionWatch struct {
		desc    string
		watcher *gnmi.Watcher[oc.E_Bgp_Neighbor_SessionState]
	}
	var watches []sessionWatch
	for _, r := range routers {
		for _, f := range families {
			nbr := r.addr(f)
			w := gnmi.Watch(t, onChange(dut), bgpPath(dut).Neighbor(nbr).SessionState().State(), sessionTimeout, func(v *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
				s, present := v.Val()
				return present && s == oc.Bgp_Neighbor_SessionState_ESTABLISHED
			})
			watches = append(watches, sessionWatch{desc: fmt.Sprintf("neighbor %s (%v, %s)", nbr, r, f), watcher: w})
		}
	}
	established := 0
	for _, w := range watches {
		v, ok := w.watcher.Await(t)
		if ok {
			established++
			continue
		}
		got := "no value"
		if v != nil {
			if s, present := v.Val(); present {
				got = s.String()
			}
		}
		t.Errorf("DUT %s session-state got %s, want ESTABLISHED", w.desc, got)
	}
	if established < wantSessions {
		t.Errorf("ESTABLISHED BGP sessions got %d, want %d", established, wantSessions)
	}
	return established == wantSessions
}

// run implements one of the subtests RT-1.111.2 to RT-1.111.5.
func (tc propagationCase) run(t *testing.T, env *testEnv) {
	otg := env.ate.OTG()
	if len(tc.advertise) > 0 {
		for _, a := range tc.advertise {
			t.Logf("Advertise the %s route prefixes (%s, %s) from %v", a.name, a.v4Prefix, a.v6Prefix, a.router)
		}
		controlRoutes(t, otg, gosnappi.StateProtocolRouteState.ADVERTISE, tc.advertise...)
	}

	for _, c := range tc.checks {
		if c.at == nil && deviations.BGPRibOcPathUnsupported(env.dut) {
			// Without the OpenConfig BGP RIB telemetry, the ATE AS_PATH checks
			// and the traffic of the subtest still verify that the DUT
			// propagates the routes.
			t.Logf("Skip the DUT loc-rib check of the %s routes: the device does not support the OpenConfig BGP RIB telemetry paths", c.route.name)
			continue
		}
		if c.at == nil {
			t.Logf("Verify in the DUT loc-rib that the %s routes are received with %s", c.route.name, c.want)
		} else {
			t.Logf("Verify %v receives the %s routes with %s", c.at, c.route.name, c.want)
		}
		for _, f := range families {
			if c.at == nil {
				verifyLocRibASPath(t, env.dut, f, c.route.prefix(f), c.want)
			} else {
				verifyATEASPath(t, otg, c.at, f, c.route.prefix(f), c.want)
			}
		}
	}

	t.Logf("Verify IPv4 and IPv6 traffic %s passes without loss", tc.trafficDescription())
	var names []string
	for _, f := range tc.flows() {
		names = append(names, f.name())
	}
	runTraffic(t, otg, names)
}

// controlRoutes advertises or withdraws the IPv4 and IPv6 route ranges of the
// given routes on the ATE.
func controlRoutes(t *testing.T, otg *otg.OTG, state gosnappi.StateProtocolRouteStateEnum, routes ...*advertisedRoute) {
	t.Helper()
	var names []string
	for _, a := range routes {
		for _, f := range families {
			names = append(names, a.otgName(f))
		}
	}
	cs := gosnappi.NewControlState()
	cs.Protocol().Route().SetNames(names).SetState(state)
	otg.SetControlState(t, cs)
}

// verifyLocRibASPath waits until prefix is in the DUT loc-rib with the AS_PATH
// want, in README notation, and reports an error otherwise.
func verifyLocRibASPath(t *testing.T, dut *ondatra.DUTDevice, f family, prefix, want string) {
	t.Helper()
	rib := bgpPath(dut).Rib()
	got := "prefix not in loc-rib"
	// matches reports whether the attr-set of a loc-rib route has the wanted
	// AS_PATH.
	matches := func(attrIndex uint64) bool {
		if deviations.SkipCheckingAttributeIndex(dut) {
			// The device does not return the attr-set by the route attr-index, so
			// every attr-set is searched for the wanted AS_PATH.
			for _, v := range gnmi.LookupAll(t, dut, rib.AttrSetAny().State()) {
				if as, ok := v.Val(); ok && asPathFromRIB(as) == want {
					got = want
					return true
				}
			}
			got = "no attr-set with the wanted AS_PATH"
			return false
		}
		as, ok := gnmi.Lookup(t, dut, rib.AttrSet(attrIndex).State()).Val()
		if !ok {
			got = fmt.Sprintf("attr-set %d not found", attrIndex)
			return false
		}
		got = asPathFromRIB(as)
		return got == want
	}

	var ok bool
	if f == ipv6 {
		_, ok = gnmi.Watch(t, onChange(dut), rib.AfiSafi(f.afiSafi()).Ipv6Unicast().LocRib().State(), routeTimeout,
			func(v *ygnmi.Value[*oc.NetworkInstance_Protocol_Bgp_Rib_AfiSafi_Ipv6Unicast_LocRib]) bool {
				locRib, present := v.Val()
				if !present {
					return false
				}
				for _, r := range locRib.Route {
					if samePrefix(r.GetPrefix(), prefix) {
						return matches(r.GetAttrIndex())
					}
				}
				return false
			}).Await(t)
	} else {
		_, ok = gnmi.Watch(t, onChange(dut), rib.AfiSafi(f.afiSafi()).Ipv4Unicast().LocRib().State(), routeTimeout,
			func(v *ygnmi.Value[*oc.NetworkInstance_Protocol_Bgp_Rib_AfiSafi_Ipv4Unicast_LocRib]) bool {
				locRib, present := v.Val()
				if !present {
					return false
				}
				for _, r := range locRib.Route {
					if samePrefix(r.GetPrefix(), prefix) {
						return matches(r.GetAttrIndex())
					}
				}
				return false
			}).Await(t)
	}
	if !ok {
		t.Errorf("DUT loc-rib %s: AS_PATH got %q, want %q", prefix, got, want)
	}
}

// verifyATEASPath waits until the ATE router learns prefix and reports an
// error unless it has the AS_PATH want, in README notation.
func verifyATEASPath(t *testing.T, otg *otg.OTG, r *ateRouter, f family, prefix, want string) {
	t.Helper()
	peer := gnmi.OTG().BgpPeer(r.peerName(f))
	isPrefix := func(addr string, length uint32) bool {
		return samePrefix(fmt.Sprintf("%s/%d", addr, length), prefix)
	}
	// The OTG reports the AS_PATH as an unkeyed list, to which a streaming
	// subscription appends the segments of every update that it receives.
	// The watch therefore only waits for the prefix, and the AS_PATH is
	// read afterwards with a new request.
	got := "prefix not received"
	var ok bool
	if f == ipv6 {
		q := peer.UnicastIpv6PrefixAny().State()
		_, ok = gnmi.WatchAll(t, otg, q, routeTimeout, func(v *ygnmi.Value[*otgtelemetry.BgpPeer_UnicastIpv6Prefix]) bool {
			p, present := v.Val()
			return present && isPrefix(p.GetAddress(), p.GetPrefixLength())
		}).Await(t)
		for _, v := range gnmi.LookupAll(t, otg, q) {
			if p, present := v.Val(); present && isPrefix(p.GetAddress(), p.GetPrefixLength()) {
				got = asPathFromOTG(p.AsPath)
			}
		}
	} else {
		q := peer.UnicastIpv4PrefixAny().State()
		_, ok = gnmi.WatchAll(t, otg, q, routeTimeout, func(v *ygnmi.Value[*otgtelemetry.BgpPeer_UnicastIpv4Prefix]) bool {
			p, present := v.Val()
			return present && isPrefix(p.GetAddress(), p.GetPrefixLength())
		}).Await(t)
		for _, v := range gnmi.LookupAll(t, otg, q) {
			if p, present := v.Val(); present && isPrefix(p.GetAddress(), p.GetPrefixLength()) {
				got = asPathFromOTG(p.AsPath)
			}
		}
	}
	if !ok || got != want {
		t.Errorf("%v %s: AS_PATH got %q, want %q", r, prefix, got, want)
	}
}

// runTraffic implements the README traffic steps for the given flows: start
// them, wait until each has sent its packets (trafficSeconds), stop them, and
// check that each flow lost at most maxLossPct of its packets.
func runTraffic(t *testing.T, otg *otg.OTG, flows []string) {
	t.Helper()
	t.Logf("Start traffic flows %v", flows)
	setFlowState(t, otg, gosnappi.StateTrafficFlowTransmitState.START, flows)

	t.Logf("Wait %d seconds for the traffic flows to send %d packets each", trafficSeconds, trafficPackets)
	for _, name := range flows {
		_, ok := gnmi.Watch(t, otg, gnmi.OTG().Flow(name).Counters().OutPkts().State(), trafficTimeout, func(v *ygnmi.Value[uint64]) bool {
			sent, present := v.Val()
			return present && sent >= trafficPackets
		}).Await(t)
		if !ok {
			t.Errorf("Flow %s: sent fewer than %d packets within %v", name, trafficPackets, trafficTimeout)
		}
	}

	t.Log("Stop traffic flows")
	setFlowState(t, otg, gosnappi.StateTrafficFlowTransmitState.STOP, flows)

	t.Log("Collect RX/TX packet counts")
	logCfg := gosnappi.NewConfig()
	for _, name := range flows {
		logCfg.Flows().Add().SetName(name)
	}
	otgutils.LogFlowMetrics(t, otg, logCfg)
	for _, name := range flows {
		t.Run(name, func(t *testing.T) {
			otgutils.ExpectedTrafficLoss(t, otg, name, 0, maxLossPct)
		})
	}
}

// setFlowState starts or stops the given flows only, leaving the flows of the
// other subtests idle.
func setFlowState(t *testing.T, otg *otg.OTG, state gosnappi.StateTrafficFlowTransmitStateEnum, flows []string) {
	t.Helper()
	cs := gosnappi.NewControlState()
	cs.Traffic().FlowTransmit().SetState(state).SetFlowNames(flows)
	otg.SetControlState(t, cs)
}

// ribSegmentNames maps the DUT RIB AS_PATH segment types to the README notation.
var ribSegmentNames = map[oc.E_RibBgp_AsPathSegmentType]string{
	oc.RibBgp_AsPathSegmentType_AS_SEQ:             "AS_SEQ",
	oc.RibBgp_AsPathSegmentType_AS_SET:             "AS_SET",
	oc.RibBgp_AsPathSegmentType_AS_CONFED_SEQUENCE: "AS_CONFED_SEQUENCE",
	oc.RibBgp_AsPathSegmentType_AS_CONFED_SET:      "AS_CONFED_SET",
}

// otgSegmentNames maps the OTG AS_PATH segment types to the README notation.
var otgSegmentNames = map[otgtelemetry.E_State_SegmentType]string{
	otgtelemetry.State_SegmentType_AS_SEQUENCE:        "AS_SEQ",
	otgtelemetry.State_SegmentType_AS_SET:             "AS_SET",
	otgtelemetry.State_SegmentType_AS_CONFED_SEQUENCE: "AS_CONFED_SEQUENCE",
	otgtelemetry.State_SegmentType_AS_CONFED_SET:      "AS_CONFED_SET",
}

// segmentText renders one AS_PATH segment in the README notation, for example
// "AS_CONFED_SEQUENCE [64501]".
func segmentText[K comparable](names map[K]string, typ K, asns []uint32) string {
	name, ok := names[typ]
	if !ok {
		name = fmt.Sprintf("UNKNOWN(%v)", typ)
	}
	return fmt.Sprintf("%s %v", name, asns)
}

// asPathFromRIB renders the AS_PATH of a DUT RIB attr-set in the README
// notation. Segments are ordered by their index, which starts from 0, and
// empty segments are skipped.
func asPathFromRIB(as *oc.NetworkInstance_Protocol_Bgp_Rib_AttrSet) string {
	indexes := make([]uint32, 0, len(as.AsSegment))
	for i := range as.AsSegment {
		indexes = append(indexes, i)
	}
	slices.Sort(indexes)
	var segs []string
	for _, i := range indexes {
		s := as.AsSegment[i]
		if len(s.GetMember()) == 0 {
			continue
		}
		segs = append(segs, segmentText(ribSegmentNames, s.GetType(), s.GetMember()))
	}
	return strings.Join(segs, ", ")
}

// otgSegment is an AS_PATH segment learned by an OTG IPv4 or IPv6 BGP peer.
type otgSegment interface {
	GetSegmentType() otgtelemetry.E_State_SegmentType
	GetAsNumbers() []uint32
}

// asPathFromOTG renders an AS_PATH learned by the ATE in the README notation,
// skipping empty segments.
func asPathFromOTG[S otgSegment](path []S) string {
	var segs []string
	for _, s := range path {
		if len(s.GetAsNumbers()) == 0 {
			continue
		}
		segs = append(segs, segmentText(otgSegmentNames, s.GetSegmentType(), s.GetAsNumbers()))
	}
	return strings.Join(segs, ", ")
}

// sameASNs reports whether got and want contain the same AS numbers in any
// order.
func sameASNs(got, want []uint32) bool {
	g, w := slices.Clone(got), slices.Clone(want)
	slices.Sort(g)
	slices.Sort(w)
	return slices.Equal(g, w)
}

// samePrefix reports whether two CIDR strings denote the same prefix,
// independent of how the address is written.
func samePrefix(a, b string) bool {
	pa, errA := netip.ParsePrefix(a)
	pb, errB := netip.ParsePrefix(b)
	return errA == nil && errB == nil && pa.Masked() == pb.Masked()
}

// firstHost returns the first address after the network address of prefix.
func firstHost(t *testing.T, prefix string) string {
	t.Helper()
	p, err := netip.ParsePrefix(prefix)
	if err != nil {
		t.Fatalf("netip.ParsePrefix(%q) failed: %v", prefix, err)
	}
	return p.Masked().Addr().Next().String()
}
