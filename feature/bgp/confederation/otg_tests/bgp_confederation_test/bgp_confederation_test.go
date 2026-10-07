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
// Confederations (RFC 5065). See README.md in this directory for the test plan.
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
	"github.com/openconfig/featureprofiles/internal/otgutils"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/gnmi/oc/netinstbgp"
	otgtelemetry "github.com/openconfig/ondatra/gnmi/otg"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// AS numbers (README Test environment setup).
const (
	dutMemberAS = 64501 // DUT Member-AS.
	confedID    = 64500 // Confederation Identifier.
	ateExtAS    = 64510 // ATE:port1, external AS.
	ateIBGPAS   = 64501 // ATE:port2, same Member-AS as the DUT.
	ateConfedAS = 64502 // ATE:port3, neighboring Member-AS.
	otherMember = 64503 // Member-AS behind the iBGP peer, only in AS_PATHs.
	extTransit  = 64511 // External AS behind Member-AS 64502, only in AS_PATHs.
)

// DUT configuration names and values (README Canonical OC).
const (
	bgpName         = "BGP"
	routerID        = "192.0.2.254"
	policyAllow     = "ALLOW"
	policyStatement = "accept-all"
	pgExt           = "EBGP-EXT"
	pgIBGP          = "IBGP-MEMBER"
	pgConfed        = "CONFED-EBGP"
	plenIPv4        = 30
	plenIPv6        = 126
)

// Timers. None of them is a fixed sleep: every wait is a gNMI watch that
// returns as soon as its condition is met, or at the timeout.
const (
	// sessionTimeout bounds the wait for a BGP session to become ESTABLISHED.
	sessionTimeout = 3 * time.Minute
	// routeTimeout bounds the wait for an expected route to appear.
	routeTimeout = time.Minute
	// positiveControlTimeout bounds the wait for the OTG routes_advertised
	// counter to increase (README absence check, positive control).
	positiveControlTimeout = time.Minute
	// holdWindow is the absence check hold window (README Checks shared by
	// the subtests).
	holdWindow = 30 * time.Second
	// trafficStatsTimeout bounds the wait for a 30 second flow to stop and
	// its counters to settle.
	trafficStatsTimeout  = 2 * time.Minute
	trafficSettleTimeout = 10 * time.Second
)

// Traffic parameters (README flows).
const (
	frameSize      = 512
	trafficPPS     = 1000
	trafficPackets = 30 * trafficPPS // 30 seconds at 1000 pps.
)

// addrFamily identifies IPv4 unicast or IPv6 unicast.
type addrFamily int

const (
	ipv4 addrFamily = iota
	ipv6
)

var addrFamilies = []addrFamily{ipv4, ipv6}

func (a addrFamily) String() string {
	if a == ipv6 {
		return "IPv6"
	}
	return "IPv4"
}

func (a addrFamily) afiSafi() oc.E_BgpTypes_AFI_SAFI_TYPE {
	if a == ipv6 {
		return oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST
	}
	return oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST
}

func (a addrFamily) other() addrFamily {
	if a == ipv6 {
		return ipv4
	}
	return ipv6
}

// bgpLink describes one DUT-ATE link and the BGP sessions on it (README
// Test environment setup).
type bgpLink struct {
	port      string
	role      string
	dut       *attrs.Attributes
	ate       *attrs.Attributes
	ateAS     uint32
	ibgp      bool
	peerGroup string
}

// peerName returns the name of the OTG BGP peer of the link.
func (l *bgpLink) peerName(af addrFamily) string {
	if af == ipv6 {
		return l.ate.Name + ".BGP6.peer"
	}
	return l.ate.Name + ".BGP4.peer"
}

// neighbor returns the DUT BGP neighbor address (the ATE address) of the link.
func (l *bgpLink) neighbor(af addrFamily) string {
	if af == ipv6 {
		return l.ate.IPv6
	}
	return l.ate.IPv4
}

// dutAddr returns the DUT interface address of the link.
func (l *bgpLink) dutAddr(af addrFamily) string {
	if af == ipv6 {
		return l.dut.IPv6
	}
	return l.dut.IPv4
}

func (l *bgpLink) String() string {
	return fmt.Sprintf("ATE:%s (%s)", l.port, l.role)
}

var (
	linkExt = &bgpLink{
		port: "port1",
		role: "external eBGP",
		dut: &attrs.Attributes{
			Desc:    "DUT port1 to ATE port1 (external eBGP)",
			IPv4:    "192.0.2.1",
			IPv4Len: plenIPv4,
			IPv6:    "2001:db8::1",
			IPv6Len: plenIPv6,
		},
		ate: &attrs.Attributes{
			Name:    "atePort1",
			MAC:     "02:00:01:01:01:01",
			IPv4:    "192.0.2.2",
			IPv4Len: plenIPv4,
			IPv6:    "2001:db8::2",
			IPv6Len: plenIPv6,
		},
		ateAS:     ateExtAS,
		peerGroup: pgExt,
	}
	linkIBGP = &bgpLink{
		port: "port2",
		role: "iBGP",
		dut: &attrs.Attributes{
			Desc:    "DUT port2 to ATE port2 (iBGP)",
			IPv4:    "192.0.2.5",
			IPv4Len: plenIPv4,
			IPv6:    "2001:db8::5",
			IPv6Len: plenIPv6,
		},
		ate: &attrs.Attributes{
			Name:    "atePort2",
			MAC:     "02:00:02:01:01:01",
			IPv4:    "192.0.2.6",
			IPv4Len: plenIPv4,
			IPv6:    "2001:db8::6",
			IPv6Len: plenIPv6,
		},
		ateAS:     ateIBGPAS,
		ibgp:      true,
		peerGroup: pgIBGP,
	}
	linkConfed = &bgpLink{
		port: "port3",
		role: "confederation eBGP",
		dut: &attrs.Attributes{
			Desc:    "DUT port3 to ATE port3 (confederation eBGP)",
			IPv4:    "192.0.2.9",
			IPv4Len: plenIPv4,
			IPv6:    "2001:db8::9",
			IPv6Len: plenIPv6,
		},
		ate: &attrs.Attributes{
			Name:    "atePort3",
			MAC:     "02:00:03:01:01:01",
			IPv4:    "192.0.2.10",
			IPv4Len: plenIPv4,
			IPv6:    "2001:db8::a",
			IPv6Len: plenIPv6,
		},
		ateAS:     ateConfedAS,
		peerGroup: pgConfed,
	}
	links = []*bgpLink{linkExt, linkIBGP, linkConfed}
)

// segmentType is an AS_PATH segment type, independent of the OC, OTG
// telemetry, and gosnappi representations.
type segmentType int

const (
	segUnknown segmentType = iota
	segSeq
	segSet
	segConfedSeq
	segConfedSet
)

func (s segmentType) String() string {
	switch s {
	case segSeq:
		return "AS_SEQ"
	case segSet:
		return "AS_SET"
	case segConfedSeq:
		return "AS_CONFED_SEQUENCE"
	case segConfedSet:
		return "AS_CONFED_SET"
	default:
		return "UNKNOWN"
	}
}

func (s segmentType) isConfed() bool {
	return s == segConfedSeq || s == segConfedSet
}

func (s segmentType) gosnappi(t *testing.T) gosnappi.BgpAsPathSegmentTypeEnum {
	t.Helper()
	switch s {
	case segSeq:
		return gosnappi.BgpAsPathSegmentType.AS_SEQ
	case segConfedSeq:
		return gosnappi.BgpAsPathSegmentType.AS_CONFED_SEQ
	default:
		t.Fatalf("segment type %v is not used in the ATE configuration of this test", s)
		return ""
	}
}

func segmentTypeFromOC(s oc.E_RibBgp_AsPathSegmentType) segmentType {
	switch s {
	case oc.RibBgp_AsPathSegmentType_AS_SEQ:
		return segSeq
	case oc.RibBgp_AsPathSegmentType_AS_SET:
		return segSet
	case oc.RibBgp_AsPathSegmentType_AS_CONFED_SEQUENCE:
		return segConfedSeq
	case oc.RibBgp_AsPathSegmentType_AS_CONFED_SET:
		return segConfedSet
	default:
		return segUnknown
	}
}

func segmentTypeFromOTG(s otgtelemetry.E_State_SegmentType) segmentType {
	switch s {
	case otgtelemetry.State_SegmentType_AS_SEQUENCE:
		return segSeq
	case otgtelemetry.State_SegmentType_AS_SET:
		return segSet
	case otgtelemetry.State_SegmentType_AS_CONFED_SEQUENCE:
		return segConfedSeq
	case otgtelemetry.State_SegmentType_AS_CONFED_SET:
		return segConfedSet
	default:
		return segUnknown
	}
}

// asSegment is one AS_PATH segment. Members are in AS_PATH order: the
// leftmost (most recently prepended) AS number is first (README Checks
// shared by the subtests, DUT telemetry).
type asSegment struct {
	typ  segmentType
	asns []uint32
}

// asPath is an AS_PATH. Index 0 is the leftmost segment.
type asPath []asSegment

func seq(asns ...uint32) asSegment       { return asSegment{typ: segSeq, asns: asns} }
func confedSeq(asns ...uint32) asSegment { return asSegment{typ: segConfedSeq, asns: asns} }

func (p asPath) String() string {
	if len(p) == 0 {
		return "(empty)"
	}
	parts := make([]string, 0, len(p))
	for _, s := range p {
		parts = append(parts, fmt.Sprintf("%v %v", s.typ, s.asns))
	}
	return strings.Join(parts, ", ")
}

// equal compares two AS_PATHs segment by segment (README OTG route range
// settings). Adjacent segments of the same type are not merged: RFC 5065
// Section 4.1 rules b.1 and c.2 require the prepended AS number to be the
// last element of the existing sequence, so AS_SEQ [64500], AS_SEQ [64511]
// is not the same as the expected AS_SEQ [64500, 64511].
func (p asPath) equal(o asPath) bool {
	return slices.EqualFunc(p, o, func(a, b asSegment) bool {
		return a.typ == b.typ && slices.Equal(a.asns, b.asns)
	})
}

// routeRange is one advertised route range (README route range table and
// OTG route range settings).
type routeRange struct {
	name string
	link *bgpLink
	v4   string
	v6   string
	// wire is the exact AS_PATH that the DUT must receive ("AS_PATH sent by
	// ATE" column).
	wire asPath
	// includeLocalAS selects the OTG as_set_mode include_as_seq, which
	// prepends the ATE AS in an AS_SEQ. Otherwise do_not_include_local_as.
	includeLocalAS bool
	// includeConfedLocalAS selects the OTG as_set_mode include_as_confed_seq,
	// which prepends the ATE AS in an AS_CONFED_SEQUENCE. It takes precedence
	// over includeLocalAS.
	includeConfedLocalAS bool
	// otgSegments are the OTG as_path segments.
	otgSegments asPath
	// localPref and med are sent only when not nil.
	localPref *uint32
	med       *uint32
}

func (r *routeRange) otgName(af addrFamily) string {
	if af == ipv6 {
		return r.name + ".v6"
	}
	return r.name + ".v4"
}

func (r *routeRange) prefix(af addrFamily) string {
	if af == ipv6 {
		return r.v6
	}
	return r.v4
}

func (r *routeRange) String() string { return r.name }

var (
	rrExt = &routeRange{
		name: "EXT", link: linkExt, v4: "203.0.113.0/26", v6: "2001:db8:100::/48",
		wire:           asPath{seq(ateExtAS)},
		includeLocalAS: true,
	}
	rrIBGP = &routeRange{
		name: "IBGP", link: linkIBGP, v4: "198.51.100.0/26", v6: "2001:db8:200::/48",
		wire:      nil,
		localPref: ygot.Uint32(200),
		med:       ygot.Uint32(50),
	}
	rrIBGPConfed = &routeRange{
		name: "IBGP-CONFED", link: linkIBGP, v4: "198.51.100.192/26", v6: "2001:db8:201::/48",
		wire:        asPath{confedSeq(otherMember)},
		otgSegments: asPath{confedSeq(otherMember)},
		localPref:   ygot.Uint32(100),
	}
	rrConfed = &routeRange{
		name: "CONFED", link: linkConfed, v4: "198.51.100.64/26", v6: "2001:db8:300::/48",
		wire:                 asPath{confedSeq(ateConfedAS)},
		includeConfedLocalAS: true,
		localPref:            ygot.Uint32(150),
	}
	rrConfedTransit = &routeRange{
		name: "CONFED-TRANSIT", link: linkConfed, v4: "203.0.113.128/26", v6: "2001:db8:302::/48",
		wire:                 asPath{confedSeq(ateConfedAS), seq(extTransit)},
		includeConfedLocalAS: true,
		otgSegments:          asPath{seq(extTransit)},
	}
	// The negative ranges are never accepted, so their wire AS_PATH is only
	// documentation. On ebgp peers the ATE always prepends its own AS (the
	// as_set_mode), and the remaining segments are added with otgSegments.
	// The looped paths may therefore be sent as one or as two adjacent
	// segments of the same type (README OTG route range settings); both are a
	// loop under RFC 5065 Section 4 and RFC 4271 Section 9.1.2.
	rrConfedLoop = &routeRange{
		name: "CONFED-LOOP", link: linkConfed, v4: "198.51.100.128/26", v6: "2001:db8:301::/48",
		wire:                 asPath{confedSeq(ateConfedAS, dutMemberAS)},
		includeConfedLocalAS: true,
		otgSegments:          asPath{confedSeq(dutMemberAS)},
	}
	rrExtLoop = &routeRange{
		name: "EXT-LOOP", link: linkExt, v4: "203.0.113.64/26", v6: "2001:db8:101::/48",
		wire:           asPath{seq(ateExtAS, confedID)},
		includeLocalAS: true,
		otgSegments:    asPath{seq(confedID)},
	}
	// EXT-MALFORMED carries an AS_CONFED_SEQUENCE from a peer outside the
	// confederation (RFC 5065 Section 5), CONFED-MALFORMED comes from the
	// confederation peer without a leading AS_CONFED_SEQUENCE.
	rrExtMalformed = &routeRange{
		name: "EXT-MALFORMED", link: linkExt, v4: "203.0.113.192/27", v6: "2001:db8:102::/48",
		wire:           asPath{seq(ateExtAS), confedSeq(ateExtAS)},
		includeLocalAS: true,
		otgSegments:    asPath{confedSeq(ateExtAS)},
	}
	rrConfedMalformed = &routeRange{
		name: "CONFED-MALFORMED", link: linkConfed, v4: "203.0.113.224/27", v6: "2001:db8:303::/48",
		wire:           asPath{seq(ateConfedAS)},
		includeLocalAS: true,
	}

	baseRoutes     = []*routeRange{rrExt, rrIBGP, rrIBGPConfed, rrConfed, rrConfedTransit}
	loopRoutes     = []*routeRange{rrConfedLoop, rrExtLoop}
	malformedRoute = []*routeRange{rrExtMalformed, rrConfedMalformed}
	negativeRoutes = append(slices.Clone(loopRoutes), malformedRoute...)
	allRoutes      = append(slices.Clone(baseRoutes), negativeRoutes...)

	// routeSetExpected is the expected set of base routes per ATE peer
	// (README route set check). Base routes advertised by the same
	// ATE port are the echo set and are ignored if received.
	routeSetExpected = map[*bgpLink][]*routeRange{
		linkExt:    {rrConfed, rrIBGP, rrIBGPConfed, rrConfedTransit},
		linkIBGP:   {rrExt, rrConfed, rrConfedTransit},
		linkConfed: {rrExt, rrIBGP, rrIBGPConfed},
	}
)

// flowSpec is one traffic flow (README flows).
type flowSpec struct {
	name    string
	af      addrFamily
	tx, rx  *routeRange
	src     string
	dst     string
	subtest string
}

var flows = []flowSpec{
	{"v4-p2-to-p3", ipv4, rrIBGP, rrConfed, "198.51.100.1", "198.51.100.65", "RT-1.111.2"},
	{"v6-p2-to-p3", ipv6, rrIBGP, rrConfed, "2001:db8:200::1", "2001:db8:300::1", "RT-1.111.2"},
	{"v4-p3-to-p2", ipv4, rrConfed, rrIBGP, "198.51.100.65", "198.51.100.1", "RT-1.111.3"},
	{"v6-p3-to-p2", ipv6, rrConfed, rrIBGP, "2001:db8:300::1", "2001:db8:200::1", "RT-1.111.3"},
	{"v4-p1-to-p3", ipv4, rrExt, rrConfed, "203.0.113.1", "198.51.100.65", "RT-1.111.4"},
	{"v6-p1-to-p3", ipv6, rrExt, rrConfed, "2001:db8:100::1", "2001:db8:300::1", "RT-1.111.4"},
	{"v4-p1-to-p2", ipv4, rrExt, rrIBGP, "203.0.113.1", "198.51.100.1", "RT-1.111.4"},
	{"v6-p1-to-p2", ipv6, rrExt, rrIBGP, "2001:db8:100::1", "2001:db8:200::1", "RT-1.111.4"},
	{"v4-p3-to-p1", ipv4, rrConfed, rrExt, "198.51.100.65", "203.0.113.1", "RT-1.111.5"},
	{"v6-p3-to-p1", ipv6, rrConfed, rrExt, "2001:db8:300::1", "2001:db8:100::1", "RT-1.111.5"},
	{"v4-p2-to-p1", ipv4, rrIBGP, rrExt, "198.51.100.1", "203.0.113.1", "RT-1.111.5"},
	{"v6-p2-to-p1", ipv6, rrIBGP, rrExt, "2001:db8:200::1", "2001:db8:100::1", "RT-1.111.5"},
}

// stabilityCounters holds the session stability counters (README session
// stability check). A missing key means the counter was not reported.
type stabilityCounters struct {
	dutTransitions map[string]uint64 // Keyed by DUT neighbor address.
	otgFlaps       map[string]uint64 // Keyed by OTG BGP peer name.
}

// testEnv is the shared state of all subtests.
type testEnv struct {
	dut    *ondatra.DUTDevice
	otg    *otg.OTG
	otgCfg gosnappi.Config
	ni     string
	// baseline is the session stability baseline.
	baseline stabilityCounters
	// rebaseline is set when a subtest reported a session stability change,
	// so that the next negative subtest records a new baseline first.
	rebaseline bool
	// sessionsUp is set by RT-1.111.1 when all six sessions are up on the
	// DUT and on the ATE.
	sessionsUp bool
}

func TestBGPConfederation(t *testing.T) {
	env := setupEnvironment(t)

	tests := []struct {
		name string
		fn   func(t *testing.T, env *testEnv)
	}{
		{"RT-1.111.1: Confederation configuration, state and session establishment", testConfigStateAndSessions},
		{"RT-1.111.2: Confederation peer to iBGP peer (AS_PATH unchanged, rule a)", testConfedToIBGP},
		{"RT-1.111.3: iBGP peer to confederation peer (AS_CONFED_SEQUENCE prepend, rule b)", testIBGPToConfed},
		{"RT-1.111.4: Confederation routes to the external peer (segments removed, rule c)", testToExternal},
		{"RT-1.111.5: External route to confederation and iBGP peers", testExternalToConfedAndIBGP},
		{"RT-1.111.6: AS loop detection (Member-AS and confederation identifier)", testASLoopDetection},
		{"RT-1.111.7: Malformed AS_PATH from external and confederation peers", testMalformedASPath},
	}
	for i, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if t.Failed() {
					dumpDiagnostics(t, env)
				}
			}()
			tc.fn(t, env)
		})
		// README Checks shared by the subtests: a session that does not reach ESTABLISHED
		// in RT-1.111.1 stops the test, because no later result would be
		// meaningful.
		if i == 0 && !env.sessionsUp {
			t.Fatalf("BGP sessions did not come up in RT-1.111.1; skipping the remaining subtests")
		}
	}
}

// setupEnvironment applies the DUT and ATE base configuration (README Test
// environment setup) and registers the cleanup.
func setupEnvironment(t *testing.T) *testEnv {
	t.Helper()
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	env := &testEnv{
		dut: dut,
		otg: ate.OTG(),
		ni:  deviations.DefaultNetworkInstance(dut),
	}

	t.Log("Test environment setup: configure the DUT")
	// Registered before the DUT configuration so that the DUT is restored
	// even if the configuration push fails part-way. Cleanups run in LIFO
	// order, so the ATE cleanup registered below runs first (README Cleanup).
	t.Cleanup(func() { cleanupDUT(t, env) })
	configureDUT(t, env)

	t.Log("Test environment setup: build the ATE configuration")
	env.otgCfg = configureOTG(t, ate)

	t.Log("Test environment setup: push the ATE configuration and start protocols")
	t.Cleanup(func() {
		t.Log("Cleanup: stop traffic and protocols on the ATE")
		env.otg.StopTraffic(t)
		env.otg.StopProtocols(t)
	})
	env.otg.PushConfig(t, env.otgCfg)
	env.otg.StartProtocols(t)

	t.Log("Test environment setup: withdraw the negative route ranges")
	setRouteState(t, env.otg, negativeRoutes, gosnappi.StateProtocolRouteState.WITHDRAW)

	t.Log("Test environment setup: wait for ARP and IPv6 neighbor discovery")
	otgutils.WaitForARP(t, env.otg, env.otgCfg, "IPv4")
	otgutils.WaitForARP(t, env.otg, env.otgCfg, "IPv6")
	return env
}

func (env *testEnv) bgpPath() *netinstbgp.NetworkInstance_Protocol_BgpPath {
	return gnmi.OC().NetworkInstance(env.ni).Protocol(cfgplugins.PTBGP, bgpName).Bgp()
}

// dutStream returns the DUT with the gNMI subscription mode ON_CHANGE. It is
// used for every STREAM subscription to the DUT (README Checks shared by the
// subtests, DUT telemetry); ONCE lookups use env.dut directly.
func (env *testEnv) dutStream() gnmi.DeviceOrOpts {
	return env.dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_ON_CHANGE))
}

// configureDUT applies the DUT configuration of README Canonical OC in the
// order interfaces, network instance, then routing policy and BGP.
func configureDUT(t *testing.T, env *testEnv) {
	t.Helper()
	dut := env.dut

	// Interfaces.
	sb := &gnmi.SetBatch{}
	for _, l := range links {
		p := dut.Port(t, l.port)
		if deviations.ExplicitPortSpeed(dut) {
			fptest.SetPortSpeed(t, p)
		}
		intf := l.dut.NewOCInterface(p.Name(), dut)
		if deviations.InterfaceEnabled(dut) {
			intf.GetOrCreateSubinterface(0).Enabled = ygot.Bool(true)
		}
		gnmi.BatchReplace(sb, gnmi.OC().Interface(p.Name()).Config(), intf)
	}
	sb.Set(t, dut)

	// The DEFAULT network instance and, where required, the explicit
	// assignment of the interfaces to it.
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		for _, l := range links {
			fptest.AssignToNetworkInstance(t, dut, dut.Port(t, l.port).Name(), env.ni, 0)
		}
	}

	// Routing policy ALLOW.
	sb = &gnmi.SetBatch{}
	rp := &oc.RoutingPolicy{}
	pd := rp.GetOrCreatePolicyDefinition(policyAllow)
	st, err := pd.AppendNewStatement(policyStatement)
	if err != nil {
		t.Fatalf("AppendNewStatement(%q) failed: %v", policyStatement, err)
	}
	st.GetOrCreateActions().SetPolicyResult(oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)
	gnmi.BatchReplace(sb, gnmi.OC().RoutingPolicy().PolicyDefinition(policyAllow).Config(), pd)

	// BGP global (including the confederation), peer-groups, and neighbors,
	// in a single replace of the BGP protocol.
	protoPath := gnmi.OC().NetworkInstance(env.ni).Protocol(cfgplugins.PTBGP, bgpName)
	gnmi.BatchReplace(sb, protoPath.Config(), buildBGP(dut))

	// Push the routing policy and BGP configuration with gNMI Set (REPLACE).
	sb.Set(t, dut)

	// Some implementations stream the BGP RIB (loc-rib, adj-rib-in-post, and
	// attr-sets) only after an additional vendor configuration.
	if deviations.BgpRibStreamingConfigRequired(dut) {
		cfgplugins.DeviationBgpRibStreamingConfigRequired(t, dut)
	}
}

// buildBGP returns the complete BGP protocol configuration of README
// Canonical OC.
func buildBGP(dut *ondatra.DUTDevice) *oc.NetworkInstance_Protocol {
	proto := &oc.NetworkInstance_Protocol{
		Identifier: cfgplugins.PTBGP,
		Name:       ygot.String(bgpName),
	}
	bgp := proto.GetOrCreateBgp()
	global := bgp.GetOrCreateGlobal()
	global.SetAs(dutMemberAS)
	global.SetRouterId(routerID)
	for _, af := range addrFamilies {
		global.GetOrCreateAfiSafi(af.afiSafi()).SetEnabled(true)
	}
	// Confederation identifier and the neighboring Member-AS. The
	// local Member-AS is global/config/as and is not listed in member-as.
	// TODO(b/525237517): add the deviation bgp_confederation_oc_unsupported
	// with a vendor CLI fallback for DUTs that do not support the OC
	// confederation container.
	confed := global.GetOrCreateConfederation()
	confed.SetIdentifier(confedID)
	confed.SetMemberAs([]uint32{ateConfedAS})

	for _, l := range links {
		// Each peer-group enables both address families with import
		// and export policy ALLOW.
		pg := bgp.GetOrCreatePeerGroup(l.peerGroup)
		pg.SetPeerAs(l.ateAS)
		for _, af := range addrFamilies {
			pgAF := pg.GetOrCreateAfiSafi(af.afiSafi())
			pgAF.SetEnabled(true)
			if !deviations.RoutePolicyUnderAFIUnsupported(dut) {
				ap := pgAF.GetOrCreateApplyPolicy()
				ap.SetImportPolicy([]string{policyAllow})
				ap.SetExportPolicy([]string{policyAllow})
			}
		}
		if deviations.RoutePolicyUnderAFIUnsupported(dut) {
			ap := pg.GetOrCreateApplyPolicy()
			ap.SetImportPolicy([]string{policyAllow})
			ap.SetExportPolicy([]string{policyAllow})
		}

		// Each neighbor enables its own address family and disables
		// the other one, so that it does not inherit it from the peer-group.
		for _, af := range addrFamilies {
			nbr := bgp.GetOrCreateNeighbor(l.neighbor(af))
			nbr.SetEnabled(true)
			nbr.SetPeerAs(l.ateAS)
			nbr.SetPeerGroup(l.peerGroup)
			nbr.GetOrCreateAfiSafi(af.afiSafi()).SetEnabled(true)
			nbr.GetOrCreateAfiSafi(af.other().afiSafi()).SetEnabled(false)
		}
	}
	return proto
}

// cleanupDUT removes the DUT configuration of this test in the order given
// in README Cleanup (the DUT part, after the ATE has stopped).
func cleanupDUT(t *testing.T, env *testEnv) {
	t.Helper()
	dut := env.dut
	bgp := env.bgpPath()

	t.Log("Cleanup: remove the BGP neighbors, peer-groups, and global BGP configuration")
	sb := &gnmi.SetBatch{}
	for _, l := range links {
		for _, af := range addrFamilies {
			gnmi.BatchDelete(sb, bgp.Neighbor(l.neighbor(af)).Config())
		}
	}
	sb.Set(t, dut)
	sb = &gnmi.SetBatch{}
	for _, l := range links {
		gnmi.BatchDelete(sb, bgp.PeerGroup(l.peerGroup).Config())
	}
	sb.Set(t, dut)
	// Deleting the protocol removes the confederation identifier, member-as,
	// and the global BGP configuration.
	gnmi.Delete(t, dut, gnmi.OC().NetworkInstance(env.ni).Protocol(cfgplugins.PTBGP, bgpName).Config())

	t.Log("Cleanup: remove the routing policy ALLOW")
	gnmi.Delete(t, dut, gnmi.OC().RoutingPolicy().PolicyDefinition(policyAllow).Config())

	// The last README Cleanup step restores the baseline by removing the addresses
	// added by this test. The interface description, enabled state, and the
	// DEFAULT network instance assignment are part of the DUT baseline
	// configuration and are intentionally left in place.
	t.Log("Cleanup: remove the interface addresses")
	sb = &gnmi.SetBatch{}
	for _, l := range links {
		sub := gnmi.OC().Interface(dut.Port(t, l.port).Name()).Subinterface(0)
		gnmi.BatchDelete(sb, sub.Ipv4().Address(l.dut.IPv4).Config())
		gnmi.BatchDelete(sb, sub.Ipv6().Address(l.dut.IPv6).Config())
	}
	sb.Set(t, dut)
}

// configureOTG builds the single ATE configuration of this test: interfaces,
// one IPv4 and one IPv6 BGP peer per port, all nine route ranges, and the
// twelve flows (README Test environment setup).
func configureOTG(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	t.Helper()
	cfg := gosnappi.NewConfig()
	for _, l := range links {
		dev := l.ate.AddToOTG(cfg, ate.Port(t, l.port), l.dut)
		eth := dev.Ethernets().Items()[0]
		ip4 := eth.Ipv4Addresses().Items()[0]
		ip6 := eth.Ipv6Addresses().Items()[0]
		bgp := dev.Bgp().SetRouterId(l.ate.IPv4)

		v4Type, v6Type := gosnappi.BgpV4PeerAsType.EBGP, gosnappi.BgpV6PeerAsType.EBGP
		if l.ibgp {
			v4Type, v6Type = gosnappi.BgpV4PeerAsType.IBGP, gosnappi.BgpV6PeerAsType.IBGP
		}

		// OTG enables both capabilities by default; restrict each peer to
		// its own address family. The learned information filter keeps both
		// address families so that the route set check can detect a prefix
		// of the other address family.
		p4 := bgp.Ipv4Interfaces().Add().SetIpv4Name(ip4.Name()).Peers().Add().
			SetName(l.peerName(ipv4)).
			SetPeerAddress(l.dut.IPv4).
			SetAsNumber(l.ateAS).
			SetAsType(v4Type)
		p4.Capability().SetIpv4Unicast(true).SetIpv6Unicast(false)
		p4.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)

		p6 := bgp.Ipv6Interfaces().Add().SetIpv6Name(ip6.Name()).Peers().Add().
			SetName(l.peerName(ipv6)).
			SetPeerAddress(l.dut.IPv6).
			SetAsNumber(l.ateAS).
			SetAsType(v6Type)
		p6.Capability().SetIpv4Unicast(false).SetIpv6Unicast(true)
		p6.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)

		for _, r := range allRoutes {
			if r.link != l {
				continue
			}
			addr, plen := splitPrefix(t, r.v4)
			rr4 := p4.V4Routes().Add().SetName(r.otgName(ipv4)).
				SetNextHopIpv4Address(l.ate.IPv4).
				SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).
				SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)
			rr4.Addresses().Add().SetAddress(addr).SetPrefix(plen).SetCount(1)
			configureASPath(t, rr4.AsPath(), r)
			configureAdvanced(rr4.Advanced(), r)

			addr, plen = splitPrefix(t, r.v6)
			rr6 := p6.V6Routes().Add().SetName(r.otgName(ipv6)).
				SetNextHopIpv6Address(l.ate.IPv6).
				SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).
				SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL)
			rr6.Addresses().Add().SetAddress(addr).SetPrefix(plen).SetCount(1)
			configureASPath(t, rr6.AsPath(), r)
			configureAdvanced(rr6.Advanced(), r)
		}
	}

	for _, f := range flows {
		flow := cfg.Flows().Add().SetName(f.name)
		flow.Metrics().SetEnable(true)
		flow.TxRx().Device().
			SetTxNames([]string{f.tx.otgName(f.af)}).
			SetRxNames([]string{f.rx.otgName(f.af)})
		flow.Size().SetFixed(frameSize)
		flow.Rate().SetPps(trafficPPS)
		flow.Duration().FixedPackets().SetPackets(trafficPackets)
		flow.Packet().Add().Ethernet().Src().SetValue(f.tx.link.ate.MAC)
		if f.af == ipv6 {
			ip := flow.Packet().Add().Ipv6()
			ip.Src().SetValue(f.src)
			ip.Dst().SetValue(f.dst)
		} else {
			ip := flow.Packet().Add().Ipv4()
			ip.Src().SetValue(f.src)
			ip.Dst().SetValue(f.dst)
		}
	}
	return cfg
}

// configureASPath sets the OTG as_path of a route range.
func configureASPath(t *testing.T, p gosnappi.BgpAsPath, r *routeRange) {
	t.Helper()
	mode := gosnappi.BgpAsPathAsSetMode.DO_NOT_INCLUDE_LOCAL_AS
	switch {
	case r.includeConfedLocalAS:
		mode = gosnappi.BgpAsPathAsSetMode.INCLUDE_AS_CONFED_SEQ
	case r.includeLocalAS:
		mode = gosnappi.BgpAsPathAsSetMode.INCLUDE_AS_SEQ
	}
	p.SetAsSetMode(mode)
	for _, s := range r.otgSegments {
		p.Segments().Add().SetType(s.typ.gosnappi(t)).SetAsNumbers(s.asns)
	}
}

// configureAdvanced sets LOCAL_PREF and MED explicitly, because OTG sends
// both by default.
func configureAdvanced(a gosnappi.BgpRouteAdvanced, r *routeRange) {
	a.SetIncludeLocalPreference(r.localPref != nil)
	if r.localPref != nil {
		a.SetLocalPreference(*r.localPref)
	}
	a.SetIncludeMultiExitDiscriminator(r.med != nil)
	if r.med != nil {
		a.SetMultiExitDiscriminator(*r.med)
	}
}

func splitPrefix(t *testing.T, s string) (string, uint32) {
	t.Helper()
	p, err := netip.ParsePrefix(s)
	if err != nil {
		t.Fatalf("netip.ParsePrefix(%q) failed: %v", s, err)
	}
	return p.Addr().String(), uint32(p.Bits())
}

// canonicalPrefix returns the canonical text form of addr/plen, so that
// prefixes reported in different textual forms compare equal.
func canonicalPrefix(addr string, plen uint32) string {
	a, err := netip.ParseAddr(addr)
	if err != nil {
		return fmt.Sprintf("%s/%d", addr, plen)
	}
	p, err := a.Prefix(int(plen))
	if err != nil {
		return fmt.Sprintf("%s/%d", addr, plen)
	}
	return p.String()
}

// canonicalAddr returns the canonical text form of an IP address.
func canonicalAddr(s string) string {
	a, err := netip.ParseAddr(s)
	if err != nil {
		return s
	}
	return a.String()
}

// setRouteState advertises or withdraws route ranges at runtime with OTG
// route state control, without pushing a new ATE configuration.
func setRouteState(t *testing.T, o *otg.OTG, ranges []*routeRange, state gosnappi.StateProtocolRouteStateEnum) {
	t.Helper()
	var names []string
	for _, r := range ranges {
		for _, af := range addrFamilies {
			names = append(names, r.otgName(af))
		}
	}
	t.Logf("Setting route state %v for %v", state, names)
	cs := gosnappi.NewControlState()
	cs.Protocol().Route().SetNames(names).SetState(state)
	o.SetControlState(t, cs)
}

// advertiseWithPositiveControl advertises negative route ranges, registers
// their withdraw as a cleanup, and waits until the routes_advertised counter
// of every sending ATE peer has increased (README absence check, positive
// control). It returns false if the positive control failed, in which case
// the absence check is inconclusive and has already been reported as a
// failure.
func advertiseWithPositiveControl(t *testing.T, env *testEnv, ranges []*routeRange) bool {
	t.Helper()
	var peers []string
	for _, r := range ranges {
		for _, af := range addrFamilies {
			if p := r.link.peerName(af); !slices.Contains(peers, p) {
				peers = append(peers, p)
			}
		}
	}
	before := map[string]uint64{}
	for _, p := range peers {
		v, _ := gnmi.Lookup(t, env.otg, gnmi.OTG().BgpPeer(p).Counters().OutRoutes().State()).Val()
		before[p] = v
	}

	// The withdraw is always performed, also when the subtest fails or the
	// advertise call itself fails; registered before the advertise.
	t.Cleanup(func() {
		setRouteState(t, env.otg, ranges, gosnappi.StateProtocolRouteState.WITHDRAW)
	})
	setRouteState(t, env.otg, ranges, gosnappi.StateProtocolRouteState.ADVERTISE)

	watchers := map[string]*gnmi.Watcher[uint64]{}
	for _, p := range peers {
		b := before[p]
		watchers[p] = gnmi.Watch(t, env.otg, gnmi.OTG().BgpPeer(p).Counters().OutRoutes().State(), positiveControlTimeout, func(v *ygnmi.Value[uint64]) bool {
			got, ok := v.Val()
			return ok && got > b
		})
	}
	ok := true
	for _, p := range peers {
		if _, increased := watchers[p].Await(t); !increased {
			t.Errorf("Positive control for %v: OTG peer %s routes_advertised did not increase from %d within %v; the absence check is inconclusive", ranges, p, before[p], positiveControlTimeout)
			ok = false
		}
	}
	return ok
}

// awaitPresent waits until q reports a value and returns the first one.
func awaitPresent[T any](t *testing.T, dev gnmi.DeviceOrOpts, q ygnmi.WildcardQuery[T], timeout time.Duration) (T, bool) {
	t.Helper()
	var zero T
	v, ok := gnmi.WatchAll(t, dev, q, timeout, func(v *ygnmi.Value[T]) bool {
		return v.IsPresent()
	}).Await(t)
	if !ok || v == nil {
		return zero, false
	}
	val, _ := v.Val()
	return val, true
}

// locRibPrefixQuery returns the DUT loc-rib prefix leaf of all paths for
// prefix.
func (env *testEnv) locRibPrefixQuery(af addrFamily, prefix string) ygnmi.WildcardQuery[string] {
	rib := env.bgpPath().Rib().AfiSafi(af.afiSafi())
	if af == ipv6 {
		return rib.Ipv6Unicast().LocRib().RouteAny().WithPrefix(prefix).Prefix().State()
	}
	return rib.Ipv4Unicast().LocRib().RouteAny().WithPrefix(prefix).Prefix().State()
}

// locRibAttrIndexQuery returns the DUT loc-rib attr-index leaf of all paths
// for prefix.
func (env *testEnv) locRibAttrIndexQuery(af addrFamily, prefix string) ygnmi.WildcardQuery[uint64] {
	rib := env.bgpPath().Rib().AfiSafi(af.afiSafi())
	if af == ipv6 {
		return rib.Ipv6Unicast().LocRib().RouteAny().WithPrefix(prefix).AttrIndex().State()
	}
	return rib.Ipv4Unicast().LocRib().RouteAny().WithPrefix(prefix).AttrIndex().State()
}

// adjRibInPostPrefixQuery returns the adj-rib-in-post prefix leaf of prefix
// from neighbor nbr.
func (env *testEnv) adjRibInPostPrefixQuery(af addrFamily, nbr, prefix string) ygnmi.WildcardQuery[string] {
	rib := env.bgpPath().Rib().AfiSafi(af.afiSafi())
	if af == ipv6 {
		return rib.Ipv6Unicast().Neighbor(nbr).AdjRibInPost().RouteAny().WithPrefix(prefix).Prefix().State()
	}
	return rib.Ipv4Unicast().Neighbor(nbr).AdjRibInPost().RouteAny().WithPrefix(prefix).Prefix().State()
}

// adjRibInPostAttrIndexQuery returns the adj-rib-in-post attr-index leaf of
// prefix from neighbor nbr.
func (env *testEnv) adjRibInPostAttrIndexQuery(af addrFamily, nbr, prefix string) ygnmi.WildcardQuery[uint64] {
	rib := env.bgpPath().Rib().AfiSafi(af.afiSafi())
	if af == ipv6 {
		return rib.Ipv6Unicast().Neighbor(nbr).AdjRibInPost().RouteAny().WithPrefix(prefix).AttrIndex().State()
	}
	return rib.Ipv4Unicast().Neighbor(nbr).AdjRibInPost().RouteAny().WithPrefix(prefix).AttrIndex().State()
}

// adjRibInPreAttrIndexQuery returns the adj-rib-in-pre attr-index leaf of
// prefix from neighbor nbr.
func (env *testEnv) adjRibInPreAttrIndexQuery(af addrFamily, nbr, prefix string) ygnmi.WildcardQuery[uint64] {
	rib := env.bgpPath().Rib().AfiSafi(af.afiSafi())
	if af == ipv6 {
		return rib.Ipv6Unicast().Neighbor(nbr).AdjRibInPre().RouteAny().WithPrefix(prefix).AttrIndex().State()
	}
	return rib.Ipv4Unicast().Neighbor(nbr).AdjRibInPre().RouteAny().WithPrefix(prefix).AttrIndex().State()
}

// adjRibOutPostAttrIndexQuery returns the adj-rib-out-post attr-index leaf
// of prefix sent to neighbor nbr.
func (env *testEnv) adjRibOutPostAttrIndexQuery(af addrFamily, nbr, prefix string) ygnmi.WildcardQuery[uint64] {
	rib := env.bgpPath().Rib().AfiSafi(af.afiSafi())
	if af == ipv6 {
		return rib.Ipv6Unicast().Neighbor(nbr).AdjRibOutPost().RouteAny().WithPrefix(prefix).AttrIndex().State()
	}
	return rib.Ipv4Unicast().Neighbor(nbr).AdjRibOutPost().RouteAny().WithPrefix(prefix).AttrIndex().State()
}

// attrSet reads the DUT attr-set referenced by idx.
func (env *testEnv) attrSet(t *testing.T, idx uint64) (*oc.NetworkInstance_Protocol_Bgp_Rib_AttrSet, bool) {
	t.Helper()
	return gnmi.Lookup(t, env.dut, env.bgpPath().Rib().AttrSet(idx).State()).Val()
}

// ribAttrSet waits for the attr-index of a RIB route selected by q and
// returns the referenced attr-set. desc names the route and table in error
// messages.
func (env *testEnv) ribAttrSet(t *testing.T, q ygnmi.WildcardQuery[uint64], desc string) (*oc.NetworkInstance_Protocol_Bgp_Rib_AttrSet, bool) {
	t.Helper()
	idx, ok := awaitPresent(t, env.dutStream(), q, routeTimeout)
	if !ok {
		t.Errorf("%s: not found within %v", desc, routeTimeout)
		return nil, false
	}
	attr, ok := env.attrSet(t, idx)
	if !ok {
		t.Errorf("%s: attr-set %d not found", desc, idx)
	}
	return attr, ok
}

// dutAdjRibInAttrSet waits for prefix in the adj-rib-in-post of neighbor nbr
// and returns the referenced attr-set.
func (env *testEnv) dutAdjRibInAttrSet(t *testing.T, af addrFamily, nbr, prefix string) (*oc.NetworkInstance_Protocol_Bgp_Rib_AttrSet, bool) {
	t.Helper()
	if deviations.BgpAdjRibOcUnsupported(env.dut) {
		return nil, false
	}
	return env.ribAttrSet(t, env.adjRibInPostAttrIndexQuery(af, nbr, prefix), fmt.Sprintf("DUT adj-rib-in-post %s from neighbor %s", prefix, nbr))
}

// dutLocRibAttrSet waits for prefix in the DUT loc-rib and returns the
// referenced attr-set.
func (env *testEnv) dutLocRibAttrSet(t *testing.T, af addrFamily, prefix string) (*oc.NetworkInstance_Protocol_Bgp_Rib_AttrSet, bool) {
	t.Helper()
	if deviations.BGPRibOcPathUnsupported(env.dut) {
		t.Logf("DUT loc-rib %s: attr-set not read (bgp_rib_oc_path_unsupported)", prefix)
		return nil, false
	}
	return env.ribAttrSet(t, env.locRibAttrIndexQuery(af, prefix), fmt.Sprintf("DUT loc-rib %s", prefix))
}

// dutAttrSet returns the attr-set of route range r as received from neighbor
// nbr, read from the adj-rib-in-post of that neighbor. On a DUT without
// adj-rib-in OC telemetry it falls back to the loc-rib attr-set, which
// carries the same attributes because every base prefix of this test is
// advertised by exactly one ATE peer. The returned string describes the
// location that was read, for error messages.
func (env *testEnv) dutAttrSet(t *testing.T, af addrFamily, nbr string, r *routeRange) (*oc.NetworkInstance_Protocol_Bgp_Rib_AttrSet, string, bool) {
	t.Helper()
	prefix := r.prefix(af)
	if deviations.BgpAdjRibOcUnsupported(env.dut) {
		attr, ok := env.dutLocRibAttrSet(t, af, prefix)
		return attr, fmt.Sprintf("DUT loc-rib %s (%s)", prefix, r), ok
	}
	attr, ok := env.dutAdjRibInAttrSet(t, af, nbr, prefix)
	return attr, fmt.Sprintf("DUT adj-rib-in-post %s (%s) from %s", prefix, r, nbr), ok
}

// checkLocRibPresent checks that prefix is in the DUT loc-rib.
func (env *testEnv) checkLocRibPresent(t *testing.T, af addrFamily, prefix string) {
	t.Helper()
	if deviations.BGPRibOcPathUnsupported(env.dut) {
		return
	}
	if _, ok := awaitPresent(t, env.dutStream(), env.locRibPrefixQuery(af, prefix), routeTimeout); !ok {
		t.Errorf("DUT loc-rib: prefix %s not found within %v", prefix, routeTimeout)
	}
}

// dutASPath converts the as-segment list of a DUT attr-set to an asPath,
// using index 0 as the leftmost segment.
func dutASPath(attr *oc.NetworkInstance_Protocol_Bgp_Rib_AttrSet) (asPath, error) {
	var p asPath
	for i := 0; i < len(attr.AsSegment); i++ {
		s, ok := attr.AsSegment[uint32(i)]
		if !ok {
			return nil, fmt.Errorf("as-segment indexes are not contiguous from 0 (have %d segments, index %d missing)", len(attr.AsSegment), i)
		}
		p = append(p, asSegment{typ: segmentTypeFromOC(s.GetType()), asns: s.GetMember()})
	}
	return p, nil
}

func checkDUTASPath(t *testing.T, desc string, attr *oc.NetworkInstance_Protocol_Bgp_Rib_AttrSet, want asPath) {
	t.Helper()
	got, err := dutASPath(attr)
	if err != nil {
		t.Errorf("%s: %v", desc, err)
		return
	}
	if !got.equal(want) {
		t.Errorf("%s: AS_PATH got %v, want %v", desc, got, want)
	}
}

func checkUint32(t *testing.T, desc, attrName string, got *uint32, want uint32) {
	t.Helper()
	switch {
	case got == nil:
		t.Errorf("%s: %s got not present, want %d", desc, attrName, want)
	case *got != want:
		t.Errorf("%s: %s got %d, want %d", desc, attrName, *got, want)
	}
}

// optUint32 formats an optional attribute value for a log message.
func optUint32(v *uint32) string {
	if v == nil {
		return "not present"
	}
	return fmt.Sprint(*v)
}

// otgRoute is a route received by an ATE BGP peer, independent of the
// address family.
type otgRoute struct {
	prefix    string
	asPath    asPath
	localPref *uint32
	med       *uint32
	nextHopV4 string
	nextHopV6 string
}

func (r otgRoute) nextHop(af addrFamily) string {
	if af == ipv6 {
		return r.nextHopV6
	}
	return r.nextHopV4
}

// otgLocalPref returns nil when the ATE reports LOCAL_PREF as 0. The ATE
// reports 0 when the attribute is absent from the UPDATE, and the test never
// uses a LOCAL_PREF of 0.
func otgLocalPref(lp *uint32) *uint32 {
	if lp == nil || *lp == 0 {
		return nil
	}
	return lp
}

func otgRouteV4(p *otgtelemetry.BgpPeer_UnicastIpv4Prefix) otgRoute {
	r := otgRoute{
		prefix:    canonicalPrefix(p.GetAddress(), p.GetPrefixLength()),
		localPref: otgLocalPref(p.LocalPreference),
		med:       p.MultiExitDiscriminator,
		nextHopV4: p.GetNextHopIpv4Address(),
		nextHopV6: p.GetNextHopIpv6Address(),
	}
	for _, s := range p.AsPath {
		r.asPath = append(r.asPath, asSegment{typ: segmentTypeFromOTG(s.GetSegmentType()), asns: s.GetAsNumbers()})
	}
	return r
}

func otgRouteV6(p *otgtelemetry.BgpPeer_UnicastIpv6Prefix) otgRoute {
	r := otgRoute{
		prefix:    canonicalPrefix(p.GetAddress(), p.GetPrefixLength()),
		localPref: otgLocalPref(p.LocalPreference),
		med:       p.MultiExitDiscriminator,
		nextHopV4: p.GetNextHopIpv4Address(),
		nextHopV6: p.GetNextHopIpv6Address(),
	}
	for _, s := range p.AsPath {
		r.asPath = append(r.asPath, asSegment{typ: segmentTypeFromOTG(s.GetSegmentType()), asns: s.GetAsNumbers()})
	}
	return r
}

func otgPrefixV4Query(peer string) ygnmi.WildcardQuery[*otgtelemetry.BgpPeer_UnicastIpv4Prefix] {
	return gnmi.OTG().BgpPeer(peer).UnicastIpv4PrefixAny().State()
}

func otgPrefixV6Query(peer string) ygnmi.WildcardQuery[*otgtelemetry.BgpPeer_UnicastIpv6Prefix] {
	return gnmi.OTG().BgpPeer(peer).UnicastIpv6PrefixAny().State()
}

// awaitOTGRouteOf waits until the OTG peer reports prefix and returns it.
func awaitOTGRouteOf[T any](t *testing.T, o *otg.OTG, q ygnmi.WildcardQuery[T], conv func(T) otgRoute, prefix string) (otgRoute, bool) {
	t.Helper()
	var got otgRoute
	_, ok := gnmi.WatchAll(t, o, q, routeTimeout, func(v *ygnmi.Value[T]) bool {
		val, present := v.Val()
		if !present {
			return false
		}
		r := conv(val)
		if r.prefix != prefix {
			return false
		}
		got = r
		return true
	}).Await(t)
	return got, ok
}

// awaitOTGRoute waits until the ATE peer of link l for address family af
// receives the prefix of r from the DUT, and reports an error if it does
// not.
func awaitOTGRoute(t *testing.T, env *testEnv, l *bgpLink, af addrFamily, r *routeRange) (otgRoute, bool) {
	t.Helper()
	peer, prefix := l.peerName(af), r.prefix(af)
	var got otgRoute
	var ok bool
	if af == ipv6 {
		got, ok = awaitOTGRouteOf(t, env.otg, otgPrefixV6Query(peer), otgRouteV6, prefix)
	} else {
		got, ok = awaitOTGRouteOf(t, env.otg, otgPrefixV4Query(peer), otgRouteV4, prefix)
	}
	if !ok {
		t.Errorf("%v %v peer %s: %s (%s) not received within %v", l, af, peer, prefix, r, routeTimeout)
		return got, false
	}
	// The first update for a prefix may not carry all of its attributes
	// yet; re-read the current state before the attributes are compared.
	for _, cur := range lookupOTGRoutes(t, env.otg, peer)[af] {
		if cur.prefix == prefix {
			return cur, true
		}
	}
	t.Errorf("%v %v peer %s: %s (%s) was received but is no longer present", l, af, peer, prefix, r)
	return got, false
}

// lookupOTGRoutes returns all IPv4 and IPv6 prefixes the OTG peer received.
func lookupOTGRoutes(t *testing.T, o *otg.OTG, peer string) map[addrFamily][]otgRoute {
	t.Helper()
	out := map[addrFamily][]otgRoute{}
	for _, v := range gnmi.LookupAll(t, o, otgPrefixV4Query(peer)) {
		if p, ok := v.Val(); ok {
			out[ipv4] = append(out[ipv4], otgRouteV4(p))
		}
	}
	for _, v := range gnmi.LookupAll(t, o, otgPrefixV6Query(peer)) {
		if p, ok := v.Val(); ok {
			out[ipv6] = append(out[ipv6], otgRouteV6(p))
		}
	}
	return out
}

func checkOTGASPath(t *testing.T, desc string, got otgRoute, want asPath) {
	t.Helper()
	if !got.asPath.equal(want) {
		t.Errorf("%s: AS_PATH got %v, want %v", desc, got.asPath, want)
	}
}

// ---------------------------------------------------------------------------
// Session and stability checks.
// ---------------------------------------------------------------------------

// awaitAllSessions waits for each of the six DUT neighbors and each of the
// six ATE BGP peers separately to be ESTABLISHED. A check that returns as
// soon as any one session is up is not sufficient (README RT-1.111.1).
func awaitAllSessions(t *testing.T, env *testEnv) bool {
	t.Helper()
	type pending struct {
		desc  string
		await func() bool
	}
	var checks []pending
	for _, l := range links {
		for _, af := range addrFamilies {
			nbr := l.neighbor(af)
			dw := gnmi.Watch(t, env.dutStream(), env.bgpPath().Neighbor(nbr).SessionState().State(), sessionTimeout, func(v *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
				s, ok := v.Val()
				return ok && s == oc.Bgp_Neighbor_SessionState_ESTABLISHED
			})
			checks = append(checks, pending{
				desc:  fmt.Sprintf("DUT neighbor %s (%v)", nbr, l),
				await: func() bool { _, ok := dw.Await(t); return ok },
			})
			peer := l.peerName(af)
			ow := gnmi.Watch(t, env.otg, gnmi.OTG().BgpPeer(peer).SessionState().State(), sessionTimeout, func(v *ygnmi.Value[otgtelemetry.E_BgpPeer_SessionState]) bool {
				s, ok := v.Val()
				return ok && s == otgtelemetry.BgpPeer_SessionState_ESTABLISHED
			})
			checks = append(checks, pending{
				desc:  fmt.Sprintf("ATE BGP peer %s", peer),
				await: func() bool { _, ok := ow.Await(t); return ok },
			})
		}
	}
	allUp := true
	for _, c := range checks {
		if !c.await() {
			t.Errorf("%s: session state did not become ESTABLISHED within %v", c.desc, sessionTimeout)
			allUp = false
		}
	}
	return allUp
}

// readStability reads the session stability counters. A counter that is not
// reported is reported as an error, because both checks are mandatory.
func readStability(t *testing.T, env *testEnv) stabilityCounters {
	t.Helper()
	c := stabilityCounters{dutTransitions: map[string]uint64{}, otgFlaps: map[string]uint64{}}
	for _, l := range links {
		for _, af := range addrFamilies {
			nbr := l.neighbor(af)
			if v, ok := gnmi.Lookup(t, env.dut, env.bgpPath().Neighbor(nbr).EstablishedTransitions().State()).Val(); ok {
				c.dutTransitions[nbr] = v
			} else {
				// TODO(b/525237517): add the deviation
				// bgp_established_transitions_unsupported, which skips this
				// check and logs it; until it exists, a missing counter is a
				// failure.
				t.Errorf("DUT neighbor %s: established-transitions is not reported", nbr)
			}
			peer := l.peerName(af)
			if v, ok := gnmi.Lookup(t, env.otg, gnmi.OTG().BgpPeer(peer).Counters().Flaps().State()).Val(); ok {
				c.otgFlaps[peer] = v
			} else {
				t.Errorf("ATE BGP peer %s: session_flap_count is not reported", peer)
			}
		}
	}
	return c
}

// recordBaseline records the session stability baseline.
func recordBaseline(t *testing.T, env *testEnv) {
	t.Helper()
	env.baseline = readStability(t, env)
	env.rebaseline = false
	t.Logf("Session stability baseline: DUT established-transitions %v, OTG session_flap_count %v", env.baseline.dutTransitions, env.baseline.otgFlaps)
}

// compareStability compares the session stability counters with the
// baseline. If any counter changed, the next negative subtest records a new
// baseline, so that one reset is reported only once.
func compareStability(t *testing.T, env *testEnv) {
	t.Helper()
	now := readStability(t, env)
	changed := false
	for nbr, base := range env.baseline.dutTransitions {
		if got, ok := now.dutTransitions[nbr]; ok && got != base {
			t.Errorf("DUT neighbor %s: established-transitions got %d, want %d (unchanged from baseline)", nbr, got, base)
			changed = true
		}
	}
	for peer, base := range env.baseline.otgFlaps {
		if got, ok := now.otgFlaps[peer]; ok && got != base {
			t.Errorf("ATE BGP peer %s: session_flap_count got %d, want %d (unchanged from baseline)", peer, got, base)
			changed = true
		}
	}
	if changed {
		env.rebaseline = true
	}
}

// readNotifications reads the OTG notifications_received counter of every
// ATE BGP peer.
func readNotifications(t *testing.T, env *testEnv) map[string]uint64 {
	t.Helper()
	out := map[string]uint64{}
	for _, l := range links {
		for _, af := range addrFamilies {
			peer := l.peerName(af)
			if v, ok := gnmi.Lookup(t, env.otg, gnmi.OTG().BgpPeer(peer).Counters().InNotifications().State()).Val(); ok {
				out[peer] = v
			} else {
				t.Errorf("ATE BGP peer %s: notifications_received is not reported", peer)
			}
		}
	}
	return out
}

func compareNotifications(t *testing.T, env *testEnv, base map[string]uint64) {
	t.Helper()
	now := readNotifications(t, env)
	for peer, b := range base {
		if got, ok := now[peer]; ok && got != b {
			t.Errorf("ATE BGP peer %s: notifications_received got %d, want %d (the DUT sent a NOTIFICATION)", peer, got, b)
			env.rebaseline = true
		}
	}
}

// prepareNegativeSubtest implements the first bullet of RT-1.111.6 and RT-1.111.7:
// wait for every session, re-baseline if a previous subtest reported a
// change, and record the notifications_received counters.
func prepareNegativeSubtest(t *testing.T, env *testEnv) map[string]uint64 {
	t.Helper()
	if !awaitAllSessions(t, env) {
		t.Fatalf("Not all BGP sessions are ESTABLISHED; the negative checks cannot run")
	}
	if env.rebaseline {
		t.Log("A previous subtest reported a session stability change; recording a new baseline")
		recordBaseline(t, env)
	}
	return readNotifications(t, env)
}

// ---------------------------------------------------------------------------
// Hold window.
// ---------------------------------------------------------------------------

// holdCheck is a watcher that runs for the hold window and reports true if
// a violation was observed.
type holdCheck struct {
	desc  string
	await func(t *testing.T) bool
}

func watchAllFor[T any](t *testing.T, dev gnmi.DeviceOrOpts, q ygnmi.WildcardQuery[T], desc string, violation func(T) bool) holdCheck {
	t.Helper()
	w := gnmi.WatchAll(t, dev, q, holdWindow, func(v *ygnmi.Value[T]) bool {
		val, ok := v.Val()
		return ok && violation(val)
	})
	return holdCheck{desc: desc, await: func(t *testing.T) bool { _, ok := w.Await(t); return ok }}
}

func watchFor[T any](t *testing.T, dev gnmi.DeviceOrOpts, q ygnmi.SingletonQuery[T], desc string, violation func(T) bool) holdCheck {
	t.Helper()
	w := gnmi.Watch(t, dev, q, holdWindow, func(v *ygnmi.Value[T]) bool {
		val, ok := v.Val()
		return ok && violation(val)
	})
	return holdCheck{desc: desc, await: func(t *testing.T) bool { _, ok := w.Await(t); return ok }}
}

// watchDeletedFor reports a violation when a path matching q is deleted
// during the hold window.
func watchDeletedFor[T any](t *testing.T, dev gnmi.DeviceOrOpts, q ygnmi.WildcardQuery[T], desc string) holdCheck {
	t.Helper()
	w := gnmi.WatchAll(t, dev, q, holdWindow, func(v *ygnmi.Value[T]) bool { return !v.IsPresent() })
	return holdCheck{desc: desc, await: func(t *testing.T) bool { _, ok := w.Await(t); return ok }}
}

// holdOptions selects the locations checked during a hold window.
type holdOptions struct {
	// adjRibInPost also checks the adj-rib-in-post of the sending neighbor.
	adjRibInPost bool
	// otgPeers also checks the prefixes received by all six ATE BGP peers.
	otgPeers bool
	// sessions also checks that all six sessions stay ESTABLISHED on the DUT
	// and on the ATE.
	sessions bool
	// baseRoutes also checks that no base prefix is deleted from the DUT
	// loc-rib. Their presence after the window is checked separately by
	// checkBaseRoutesInLocRib.
	baseRoutes bool
}

// runHoldWindow checks that none of the prefixes of ranges appears in the DUT
// loc-rib (and the other selected locations) for the whole hold window
// (README absence check, hold window). All watchers start together, so
// the window runs once for all checks.
//
// Each check is a gNMI STREAM subscription that must reach its deadline
// without a matching update. gnmi.Watcher.Await returns false on the
// deadline, but calls t.Fatalf on any other subscription error (for example,
// a path the DUT rejects). If that turns out to be a problem on some
// implementation, the fallback is to sample the same paths with
// gnmi.LookupAll during the window. At most 42 subscriptions run
// concurrently (RT-1.111.7: 8 DUT RIB, 10 base prefix, 12 OTG prefix, and 12
// session watchers), split between the DUT and the OTG gNMI servers.
func runHoldWindow(t *testing.T, env *testEnv, ranges []*routeRange, opts holdOptions) {
	t.Helper()
	t.Logf("Hold window of %v for %v", holdWindow, ranges)
	present := func(string) bool { return true }
	var checks []holdCheck
	if deviations.BGPRibOcPathUnsupported(env.dut) {
		t.Log("DUT loc-rib absence check skipped: deviation bgp_rib_oc_path_unsupported")
	}
	for _, r := range ranges {
		for _, af := range addrFamilies {
			prefix := r.prefix(af)
			if !deviations.BGPRibOcPathUnsupported(env.dut) {
				checks = append(checks, watchAllFor(t, env.dutStream(), env.locRibPrefixQuery(af, prefix),
					fmt.Sprintf("%s (%s) is present in the DUT loc-rib", prefix, r), present))
			}
			if opts.adjRibInPost && !deviations.BgpAdjRibOcUnsupported(env.dut) {
				nbr := r.link.neighbor(af)
				checks = append(checks, watchAllFor(t, env.dutStream(), env.adjRibInPostPrefixQuery(af, nbr, prefix),
					fmt.Sprintf("%s (%s) is present in the DUT adj-rib-in-post of neighbor %s", prefix, r, nbr), present))
			}
		}
	}
	if opts.otgPeers {
		forbidden := map[addrFamily][]string{}
		for _, r := range ranges {
			for _, af := range addrFamilies {
				forbidden[af] = append(forbidden[af], r.prefix(af))
			}
		}
		for _, l := range links {
			for _, af := range addrFamilies {
				peer := l.peerName(af)
				checks = append(checks,
					watchAllFor(t, env.otg, otgPrefixV4Query(peer),
						fmt.Sprintf("ATE BGP peer %s received a forbidden IPv4 prefix from %v", peer, forbidden[ipv4]),
						func(p *otgtelemetry.BgpPeer_UnicastIpv4Prefix) bool {
							return slices.Contains(forbidden[ipv4], otgRouteV4(p).prefix)
						}),
					watchAllFor(t, env.otg, otgPrefixV6Query(peer),
						fmt.Sprintf("ATE BGP peer %s received a forbidden IPv6 prefix from %v", peer, forbidden[ipv6]),
						func(p *otgtelemetry.BgpPeer_UnicastIpv6Prefix) bool {
							return slices.Contains(forbidden[ipv6], otgRouteV6(p).prefix)
						}))
			}
		}
	}
	if opts.baseRoutes && !deviations.BGPRibOcPathUnsupported(env.dut) {
		for _, r := range baseRoutes {
			for _, af := range addrFamilies {
				checks = append(checks, watchDeletedFor(t, env.dutStream(), env.locRibPrefixQuery(af, r.prefix(af)),
					fmt.Sprintf("base prefix %s (%s) was deleted from the DUT loc-rib", r.prefix(af), r)))
			}
		}
	}
	if opts.sessions {
		for _, l := range links {
			for _, af := range addrFamilies {
				nbr := l.neighbor(af)
				checks = append(checks, watchFor(t, env.dutStream(), env.bgpPath().Neighbor(nbr).SessionState().State(),
					fmt.Sprintf("DUT neighbor %s left ESTABLISHED", nbr),
					func(s oc.E_Bgp_Neighbor_SessionState) bool { return s != oc.Bgp_Neighbor_SessionState_ESTABLISHED }))
				peer := l.peerName(af)
				checks = append(checks, watchFor(t, env.otg, gnmi.OTG().BgpPeer(peer).SessionState().State(),
					fmt.Sprintf("ATE BGP peer %s left ESTABLISHED", peer),
					func(s otgtelemetry.E_BgpPeer_SessionState) bool {
						return s != otgtelemetry.BgpPeer_SessionState_ESTABLISHED
					}))
			}
		}
	}
	for _, c := range checks {
		if c.await(t) {
			t.Errorf("Hold window: %s", c.desc)
		}
	}
}

// checkBaseRoutesInLocRib checks that all base prefixes are still in the DUT
// loc-rib.
func checkBaseRoutesInLocRib(t *testing.T, env *testEnv) {
	t.Helper()
	if deviations.BGPRibOcPathUnsupported(env.dut) {
		t.Log("DUT loc-rib base route check skipped: deviation bgp_rib_oc_path_unsupported")
		return
	}
	for _, r := range baseRoutes {
		for _, af := range addrFamilies {
			if len(gnmi.LookupAll(t, env.dut, env.locRibPrefixQuery(af, r.prefix(af)))) == 0 {
				t.Errorf("DUT loc-rib: base prefix %s (%s) is missing", r.prefix(af), r)
			}
		}
	}
}

// checkRouteSet runs the README route set check for all six ATE BGP
// peers.
func checkRouteSet(t *testing.T, env *testEnv) {
	t.Helper()
	var negatives []string
	for _, r := range negativeRoutes {
		for _, af := range addrFamilies {
			negatives = append(negatives, r.prefix(af))
		}
	}
	for _, l := range links {
		var echo []string
		for _, r := range baseRoutes {
			if r.link == l {
				echo = append(echo, r.prefix(ipv4), r.prefix(ipv6))
			}
		}
		for _, af := range addrFamilies {
			peer := l.peerName(af)
			received := lookupOTGRoutes(t, env.otg, peer)
			// Forbidden set first, over all received prefixes before any
			// echo exemption.
			for _, r := range received[af.other()] {
				t.Errorf("Route set %s: received %v prefix %s on an %v session", peer, af.other(), r.prefix, af)
			}
			got := map[string]bool{}
			for _, r := range received[af] {
				got[r.prefix] = true
				if slices.Contains(negatives, r.prefix) {
					t.Errorf("Route set %s: received forbidden negative prefix %s", peer, r.prefix)
				}
			}
			// Expected set.
			want := map[string]bool{}
			for _, r := range routeSetExpected[l] {
				want[r.prefix(af)] = true
				if !got[r.prefix(af)] {
					t.Errorf("Route set %s: expected prefix %s (%s) not received", peer, r.prefix(af), r)
				}
			}
			// Echo exemption, then anything else is unexpected.
			for p := range got {
				if want[p] || slices.Contains(echo, p) || slices.Contains(negatives, p) {
					continue
				}
				t.Errorf("Route set %s: received unexpected prefix %s", peer, p)
			}
		}
	}
}

// dumpDiagnostics logs the state that helps to tell a DUT problem from an ATE
// or test problem (README Checks shared by the subtests, on failure).
func dumpDiagnostics(t *testing.T, env *testEnv) {
	t.Helper()
	t.Log("Collecting diagnostics after failure")
	bgp := env.bgpPath()
	for _, l := range links {
		for _, af := range addrFamilies {
			nbr := bgp.Neighbor(l.neighbor(af))
			state, _ := gnmi.Lookup(t, env.dut, nbr.SessionState().State()).Val()
			t.Logf("DUT neighbor %s: session-state %v", l.neighbor(af), state)
			if !deviations.MissingBgpLastNotificationErrorCode(env.dut) {
				if sent, ok := gnmi.Lookup(t, env.dut, nbr.Messages().Sent().LastNotificationErrorCode().State()).Val(); ok {
					t.Logf("DUT neighbor %s: last-notification-error-code sent %v", l.neighbor(af), sent)
				}
				if rcvd, ok := gnmi.Lookup(t, env.dut, nbr.Messages().Received().LastNotificationErrorCode().State()).Val(); ok {
					t.Logf("DUT neighbor %s: last-notification-error-code received %v", l.neighbor(af), rcvd)
				}
			}
			pfx := nbr.AfiSafi(af.afiSafi()).Prefixes()
			rx, _ := gnmi.Lookup(t, env.dut, pfx.Received().State()).Val()
			inst, _ := gnmi.Lookup(t, env.dut, pfx.Installed().State()).Val()
			sentPfx, _ := gnmi.Lookup(t, env.dut, pfx.Sent().State()).Val()
			t.Logf("DUT neighbor %s: prefixes received %d, installed %d, sent %d", l.neighbor(af), rx, inst, sentPfx)
			peerType, _ := gnmi.Lookup(t, env.dut, nbr.PeerType().State()).Val()
			t.Logf("DUT neighbor %s: peer-type %v", l.neighbor(af), peerType)
			peer := l.peerName(af)
			if c, ok := gnmi.Lookup(t, env.otg, gnmi.OTG().BgpPeer(peer).Counters().State()).Val(); ok {
				t.Logf("ATE BGP peer %s: routes_advertised %d, routes_received %d, session_flap_count %d, notifications_sent %d, notifications_received %d",
					peer, c.GetOutRoutes(), c.GetInRoutes(), c.GetFlaps(), c.GetOutNotifications(), c.GetInNotifications())
			}
		}
	}
	for _, af := range addrFamilies {
		rib := bgp.Rib().AfiSafi(af.afiSafi())
		var q ygnmi.WildcardQuery[string]
		if af == ipv6 {
			q = rib.Ipv6Unicast().LocRib().RouteAny().Prefix().State()
		} else {
			q = rib.Ipv4Unicast().LocRib().RouteAny().Prefix().State()
		}
		var prefixes []string
		for _, v := range gnmi.LookupAll(t, env.dut, q) {
			if p, ok := v.Val(); ok {
				prefixes = append(prefixes, p)
			}
		}
		t.Logf("DUT %v loc-rib prefixes: %v", af, prefixes)
	}
}

// valueString formats the last value observed by a watcher for an error
// message.
func valueString[T any](v *ygnmi.Value[T]) string {
	if v == nil {
		return "no value"
	}
	val, ok := v.Val()
	if !ok {
		return "not present"
	}
	return fmt.Sprint(val)
}

// ---------------------------------------------------------------------------
// Subtests.
// ---------------------------------------------------------------------------

// testConfigStateAndSessions implements RT-1.111.1.
func testConfigStateAndSessions(t *testing.T, env *testEnv) {
	global := env.bgpPath().Global()

	t.Log("Step 1: verify the confederation state telemetry")
	if v, ok := gnmi.Watch(t, env.dutStream(), global.As().State(), routeTimeout, func(v *ygnmi.Value[uint32]) bool {
		got, ok := v.Val()
		return ok && got == dutMemberAS
	}).Await(t); !ok {
		t.Errorf("global/state/as got %s, want %d", valueString(v), dutMemberAS)
	}
	if v, ok := gnmi.Watch(t, env.dutStream(), global.Confederation().Identifier().State(), routeTimeout, func(v *ygnmi.Value[uint32]) bool {
		got, ok := v.Val()
		return ok && got == confedID
	}).Await(t); !ok {
		t.Errorf("global/confederation/state/identifier got %s, want %d", valueString(v), confedID)
	}
	wantMembers := []uint32{ateConfedAS}
	if v, ok := gnmi.Watch(t, env.dutStream(), global.Confederation().MemberAs().State(), routeTimeout, func(v *ygnmi.Value[[]uint32]) bool {
		got, ok := v.Val()
		return ok && slices.Equal(got, wantMembers)
	}).Await(t); !ok {
		// TODO(b/525237517): add the deviation
		// bgp_confederation_member_as_includes_local for implementations
		// that also report the local Member-AS ([64501, 64502]) (README
		// RT-1.111.1).
		t.Errorf("global/confederation/state/member-as got %s, want %v", valueString(v), wantMembers)
	}

	t.Log("Step 2: verify that each of the six BGP sessions is ESTABLISHED on the DUT and on the ATE")
	env.sessionsUp = awaitAllSessions(t, env)
	if !env.sessionsUp {
		return
	}

	t.Log("Step 3: record the session stability baseline")
	recordBaseline(t, env)

	t.Log("Step 4: verify the negative prefixes are absent from the DUT loc-rib at startup")
	runHoldWindow(t, env, negativeRoutes, holdOptions{})
	compareStability(t, env)
}

// testConfedToIBGP implements RT-1.111.2 (RFC 5065 Section 4.1 rule a and
// RFC 4271 Section 5.1.5 LOCAL_PREF exception for confederations).
func testConfedToIBGP(t *testing.T, env *testEnv) {
	routes := []*routeRange{rrConfed, rrConfedTransit}
	// localPrefReceived records per address family whether LOCAL_PREF 150 of
	// CONFED reached the DUT, which gates the LOCAL_PREF checks.
	localPrefReceived := map[addrFamily]bool{}

	t.Log("Step 1: verify the DUT RIB")
	for _, af := range addrFamilies {
		nbr := linkConfed.neighbor(af)
		for _, r := range routes {
			attr, desc, ok := env.dutAttrSet(t, af, nbr, r)
			if ok {
				checkDUTASPath(t, desc, attr, r.wire)
			}
			if r == rrConfed && env.confedLocalPrefReceived(t, af, attr) {
				localPrefReceived[af] = true
				if ok {
					checkUint32(t, desc, "local-pref", attr.LocalPref, *rrConfed.localPref)
				}
			}
			env.checkLocRibPresent(t, af, r.prefix(af))
		}
	}

	t.Log("Step 2: verify the routes received by ATE:port2 (AS_PATH not modified)")
	for _, af := range addrFamilies {
		for _, r := range routes {
			got, ok := awaitOTGRoute(t, env, linkIBGP, af, r)
			if !ok {
				continue
			}
			desc := fmt.Sprintf("ATE:port2 %s (%s)", r.prefix(af), r)
			checkOTGASPath(t, desc, got, r.wire)
			if r == rrConfed && localPrefReceived[af] {
				checkUint32(t, desc, "local_preference", got.localPref, *rrConfed.localPref)
			}
		}
	}
	t.Log("Flows v4-p2-to-p3 and v6-p2-to-p3 are checked in the traffic phase of RT-1.111.5")
}

// confedLocalPrefReceived reports whether LOCAL_PREF 150 of CONFED, sent on
// the OTG ebgp peer of ATE:port3, reached the DUT. OTG guarantees
// include_local_preference only on ibgp peers (README OTG route range
// settings), so the LOCAL_PREF checks of RT-1.111.2 run only when it did. The
// adj-rib-in-pre attr-set shows the attribute as received; on a DUT without
// adj-rib OC telemetry the attr-set already read from the loc-rib (attr) is
// used instead, which cannot tell an attribute that was not sent from one
// that was ignored.
func (env *testEnv) confedLocalPrefReceived(t *testing.T, af addrFamily, attr *oc.NetworkInstance_Protocol_Bgp_Rib_AttrSet) bool {
	t.Helper()
	want := *rrConfed.localPref
	prefix, nbr := rrConfed.prefix(af), linkConfed.neighbor(af)
	table := "loc-rib"
	if !deviations.BgpAdjRibOcUnsupported(env.dut) {
		// The adj-rib-in-pre entry is only a gate, so it is read once
		// without failing: step 1 has already waited for the route.
		table, attr = "adj-rib-in-pre", nil
		for _, v := range gnmi.LookupAll(t, env.dut, env.adjRibInPreAttrIndexQuery(af, nbr, prefix)) {
			if idx, ok := v.Val(); ok {
				attr, _ = env.attrSet(t, idx)
				break
			}
		}
	}
	switch {
	case attr == nil:
		t.Logf("DUT %s %s (%s) from %s: attr-set not read; LOCAL_PREF checks skipped for %v", table, prefix, rrConfed, nbr, af)
	case attr.LocalPref != nil && attr.GetLocalPref() == want:
		return true
	default:
		t.Logf("DUT %s %s (%s): local-pref %s, want %d: the ATE did not send LOCAL_PREF on the ebgp peer %s (OTG guarantees include_local_preference only on ibgp peers); LOCAL_PREF checks skipped for %v", table, prefix, rrConfed, optUint32(attr.LocalPref), want, nbr, af)
	}
	return false
}

// testIBGPToConfed implements RT-1.111.3 (RFC 5065 Section 4.1 rules b.1 and
// b.3, and Section 5.2 attributes).
func testIBGPToConfed(t *testing.T, env *testEnv) {
	t.Log("Step 1: verify the DUT RIB")
	for _, af := range addrFamilies {
		nbr := linkIBGP.neighbor(af)

		if attr, desc, ok := env.dutAttrSet(t, af, nbr, rrIBGP); ok {
			checkUint32(t, desc, "local-pref", attr.LocalPref, *rrIBGP.localPref)
			checkUint32(t, desc, "med", attr.Med, *rrIBGP.med)
			if got, want := canonicalAddr(attr.GetNextHop()), canonicalAddr(linkIBGP.neighbor(af)); got != want {
				t.Errorf("%s: next-hop got %q, want %q", desc, got, want)
			}
			checkDUTASPath(t, desc, attr, rrIBGP.wire)
		}

		if attr, desc, ok := env.dutAttrSet(t, af, nbr, rrIBGPConfed); ok {
			checkDUTASPath(t, desc, attr, rrIBGPConfed.wire)
		}
	}

	t.Log("Step 2: verify the routes received by ATE:port3")
	for _, af := range addrFamilies {
		// Rule b.3: empty AS_PATH, the DUT creates AS_CONFED_SEQUENCE [64501].
		if got, ok := awaitOTGRoute(t, env, linkConfed, af, rrIBGP); ok {
			desc := fmt.Sprintf("ATE:port3 %s (%s)", rrIBGP.prefix(af), rrIBGP)
			checkOTGASPath(t, desc, got, asPath{confedSeq(dutMemberAS)})
			checkUint32(t, desc, "multi_exit_discriminator", got.med, *rrIBGP.med)
			checkConfedNextHop(t, desc, got, af)
			// An OTG ebgp peer does not report a received LOCAL_PREF (RFC
			// 4271 Section 5.1.5); the value is logged and the LOCAL_PREF
			// sent by the DUT is verified in step 3.
			t.Logf("%s: local_preference reported by the ATE: %s", desc, optUint32(got.localPref))
		}
		// Rule b.1: 64501 is prepended to the existing AS_CONFED_SEQUENCE.
		if got, ok := awaitOTGRoute(t, env, linkConfed, af, rrIBGPConfed); ok {
			desc := fmt.Sprintf("ATE:port3 %s (%s)", rrIBGPConfed.prefix(af), rrIBGPConfed)
			checkOTGASPath(t, desc, got, asPath{confedSeq(dutMemberAS, otherMember)})
			checkConfedNextHop(t, desc, got, af)
		}
	}

	t.Log("Step 3: verify the LOCAL_PREF sent to ATE:port3 in the DUT adj-rib-out-post")
	for _, af := range addrFamilies {
		env.checkAdjRibOutLocalPref(t, af, linkConfed.neighbor(af), rrIBGP)
	}
	t.Log("Flows v4-p3-to-p2 and v6-p3-to-p2 are checked in the traffic phase of RT-1.111.5")
}

// checkAdjRibOutLocalPref verifies that the DUT sends route range r with its
// LOCAL_PREF unchanged to neighbor nbr, read from the adj-rib-out-post of
// that neighbor (README RT-1.111.3). The check is skipped on a DUT without
// adj-rib OC telemetry.
func (env *testEnv) checkAdjRibOutLocalPref(t *testing.T, af addrFamily, nbr string, r *routeRange) {
	t.Helper()
	prefix := r.prefix(af)
	desc := fmt.Sprintf("DUT adj-rib-out-post %s (%s) to %s", prefix, r, nbr)
	if deviations.BgpAdjRibOcUnsupported(env.dut) {
		t.Logf("%s: LOCAL_PREF %d not verified (bgp_adj_rib_oc_unsupported)", desc, *r.localPref)
		return
	}
	if attr, ok := env.ribAttrSet(t, env.adjRibOutPostAttrIndexQuery(af, nbr, prefix), desc); ok {
		checkUint32(t, desc, "local-pref", attr.LocalPref, *r.localPref)
	}
}

// checkConfedNextHop checks the NEXT_HOP of a route from the iBGP peer as
// received by ATE:port3. RFC 5065 Section 5.1/5.2: the NEXT_HOP may be
// unchanged (the default) or the DUT's own address on Link 3; which one is
// logged (README RT-1.111.3).
func checkConfedNextHop(t *testing.T, desc string, got otgRoute, af addrFamily) {
	t.Helper()
	unchanged, self := canonicalAddr(linkIBGP.neighbor(af)), canonicalAddr(linkConfed.dutAddr(af))
	switch nh := canonicalAddr(got.nextHop(af)); nh {
	case unchanged:
		t.Logf("%s: NEXT_HOP %s is unchanged (default behavior)", desc, nh)
	case self:
		t.Logf("%s: NEXT_HOP %s is the DUT address (next-hop-self)", desc, nh)
	default:
		t.Errorf("%s: NEXT_HOP got %q, want %q (unchanged) or %q (next-hop-self)", desc, nh, unchanged, self)
	}
}

// testToExternal implements RT-1.111.4 (RFC 5065 Section 4.1 rules c.1, c.2,
// and c.4, Section 5, and RFC 4271 Section 5.1.5).
func testToExternal(t *testing.T, env *testEnv) {
	want := map[*routeRange]asPath{
		rrConfed:        {seq(confedID)},
		rrIBGP:          {seq(confedID)},
		rrIBGPConfed:    {seq(confedID)},
		rrConfedTransit: {seq(confedID, extTransit)},
	}
	memberASNs := []uint32{dutMemberAS, ateConfedAS, otherMember}

	t.Log("Step 1: verify the routes received by ATE:port1")
	for _, af := range addrFamilies {
		for _, r := range []*routeRange{rrConfed, rrIBGP, rrIBGPConfed, rrConfedTransit} {
			got, ok := awaitOTGRoute(t, env, linkExt, af, r)
			if !ok {
				continue
			}
			desc := fmt.Sprintf("ATE:port1 %s (%s)", r.prefix(af), r)
			checkOTGASPath(t, desc, got, want[r])
			for _, s := range got.asPath {
				if s.typ.isConfed() {
					t.Errorf("%s: received confederation segment %v %v", desc, s.typ, s.asns)
				}
				for _, as := range s.asns {
					if slices.Contains(memberASNs, as) {
						t.Errorf("%s: received Member-AS %d in the AS_PATH", desc, as)
					}
				}
			}
			if got.localPref != nil {
				t.Errorf("%s: local_preference got %d, want not present", desc, *got.localPref)
			}
			// Only the global IPv6 next hop is checked; a link-local next
			// hop, if also sent, is not checked.
			if nh, want := canonicalAddr(got.nextHop(af)), canonicalAddr(linkExt.dutAddr(af)); nh != want {
				t.Errorf("%s: next hop got %q, want %q", desc, nh, want)
			}
		}
	}
	t.Log("Flows v4-p1-to-p3, v6-p1-to-p3, v4-p1-to-p2, and v6-p1-to-p2 are checked in the traffic phase of RT-1.111.5")
}

// testExternalToConfedAndIBGP implements RT-1.111.5 (RFC 5065 Section 4.1
// rules a and b.2), the route set check, and the single traffic phase.
func testExternalToConfedAndIBGP(t *testing.T, env *testEnv) {
	t.Log("Step 1: verify the DUT RIB")
	for _, af := range addrFamilies {
		desc := fmt.Sprintf("DUT loc-rib %s (%s)", rrExt.prefix(af), rrExt)
		if attr, ok := env.dutLocRibAttrSet(t, af, rrExt.prefix(af)); ok {
			checkDUTASPath(t, desc, attr, rrExt.wire)
		}
	}

	t.Log("Step 2: verify the routes received by ATE:port3 and ATE:port2")
	for _, af := range addrFamilies {
		// Rule b.2: a new AS_CONFED_SEQUENCE is prepended.
		if got, ok := awaitOTGRoute(t, env, linkConfed, af, rrExt); ok {
			checkOTGASPath(t, fmt.Sprintf("ATE:port3 %s (%s)", rrExt.prefix(af), rrExt), got, asPath{confedSeq(dutMemberAS), seq(ateExtAS)})
		}
		// Rule a: the AS_PATH is not modified.
		if got, ok := awaitOTGRoute(t, env, linkIBGP, af, rrExt); ok {
			checkOTGASPath(t, fmt.Sprintf("ATE:port2 %s (%s)", rrExt.prefix(af), rrExt), got, rrExt.wire)
		}
	}

	t.Log("Step 3: route set check")
	checkRouteSet(t, env)

	t.Log("Step 4: verify traffic for all flows")
	runTraffic(t, env)
}

// runTraffic runs all twelve flows in a single traffic phase and evaluates
// each flow separately (README flows and RT-1.111.5).
//
// The flow counters are watched in two phases: first until every flow has
// transmitted all of its packets, then, after traffic is stopped, until the
// Rx counter of each flow has settled at its Tx counter. This avoids reading
// the counters before transmission has started. The timing of the OTG flow
// counters should be verified on ixia-c/KNE and hardware ATEs.
func runTraffic(t *testing.T, env *testEnv) {
	t.Helper()
	env.otg.StartTraffic(t)

	txWatchers := map[string]*gnmi.Watcher[uint64]{}
	for _, f := range flows {
		txWatchers[f.name] = gnmi.Watch(t, env.otg, gnmi.OTG().Flow(f.name).Counters().OutPkts().State(), trafficStatsTimeout, func(v *ygnmi.Value[uint64]) bool {
			tx, ok := v.Val()
			return ok && tx >= trafficPackets
		})
	}
	for _, f := range flows {
		if _, ok := txWatchers[f.name].Await(t); !ok {
			t.Errorf("[%s] flow %s: did not transmit %d packets within %v", f.subtest, f.name, trafficPackets, trafficStatsTimeout)
		}
	}
	env.otg.StopTraffic(t)

	for _, f := range flows {
		tx, rx := otgutils.GetFlowStats(t, env.otg, f.name, trafficSettleTimeout)
		switch {
		case tx == 0:
			t.Errorf("[%s] flow %s: transmitted packets got 0, want > 0", f.subtest, f.name)
		case rx != tx:
			lossPct := (float64(tx) - float64(rx)) * 100 / float64(tx)
			t.Errorf("[%s] flow %s: received %d of %d packets (loss %.2f%%), want 0%% loss", f.subtest, f.name, rx, tx, lossPct)
		default:
			t.Logf("[%s] flow %s: %d packets transmitted and received", f.subtest, f.name, tx)
		}
	}
	otgutils.LogFlowMetrics(t, env.otg, env.otgCfg)
}

// testASLoopDetection implements RT-1.111.6 (RFC 5065 Section 4).
func testASLoopDetection(t *testing.T, env *testEnv) {
	t.Log("Step 1: advertise the looped routes")
	notifications := prepareNegativeSubtest(t, env)
	// After a failed positive control the absence check is inconclusive
	// (already reported), but the session and route set checks still run.
	if advertiseWithPositiveControl(t, env, loopRoutes) {
		t.Log("Step 2: verify the looped routes are rejected")
		runHoldWindow(t, env, loopRoutes, holdOptions{otgPeers: true, sessions: true, baseRoutes: true})
	}
	checkBaseRoutesInLocRib(t, env)
	compareStability(t, env)
	compareNotifications(t, env, notifications)
	checkRouteSet(t, env)
}

// testMalformedASPath implements RT-1.111.7 (RFC 5065 Section 5 and RFC 7606
// treat-as-withdraw).
func testMalformedASPath(t *testing.T, env *testEnv) {
	t.Log("Step 1: advertise the malformed routes")
	notifications := prepareNegativeSubtest(t, env)
	// A session reset instead of treat-as-withdraw is a failure of the
	// RFC 7606 requirement and is reported by the session and counter checks,
	// which also run after a failed positive control.
	if advertiseWithPositiveControl(t, env, malformedRoute) {
		t.Log("Step 2: verify the malformed routes are not accepted")
		runHoldWindow(t, env, malformedRoute, holdOptions{adjRibInPost: true, otgPeers: true, sessions: true, baseRoutes: true})
	}
	checkBaseRoutesInLocRib(t, env)
	compareStability(t, env)
	compareNotifications(t, env, notifications)
	checkRouteSet(t, env)
}
