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

// Package pipeline_counters_drop_test implements gNMI-1.29: Pipeline Counters
// Drops Test, as described in README.md.
package pipeline_counters_drop_test

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/telemetry/pipelinecounters"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
)

var hcFlow = flag.Bool("pipeline_hc_flow", false, "Run the optional 100,000 packet high-count flow of gNMI-1.29.7 Step 6.")

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	ipv4PrefixLen  = 30
	ipv6PrefixLen  = 126
	mtu            = 1500
	sampleInterval = 10 * time.Second
	awaitTimeout   = 2 * time.Minute
	roundTimeout   = time.Minute
	settleTimeout  = 2 * time.Minute
	// resubscribeInterval bounds each streaming subscription of
	// awaitResubscribe before the path is subscribed again.
	resubscribeInterval = 10 * time.Second
	quietRetries        = 3
	pktCount            = 1000
	// maxListedLeaves bounds how many leaves a failure message lists.
	maxListedLeaves = 10

	// Prefix plan (README "Prefix plan").
	legacyDiscardV4 = "203.0.113.2/32"
	controlV4       = "203.0.113.128/25"
	controlV6       = "2001:db8:200::/64"
	discardV4       = "198.51.100.0/24"
	discardV6       = "2001:db8:100::/64"
	edgeV4          = "100.64.0.0/24"
	edgeV6          = "2001:db8:300::/64"
	stabilityV4     = "192.0.2.128/25"

	// Pre-defined destinations.
	legacyDstV4  = "203.0.113.2"
	controlDstV4 = "203.0.113.129"
	controlDstV6 = "2001:db8:200::1"
	discardDstV4 = "198.51.100.1"
	discardDstV6 = "2001:db8:100::1"
	edgeDstV4    = "100.64.0.1"
	edgeDstV6    = "2001:db8:300::1"
)

// ciscoLeaf identifies a Cisco XR packet-processing drop counter of
// gNMI-1.29.1 to gNMI-1.29.3 by case-insensitive leaf name prefixes: the name
// produced by the ciscoxr-vendordrops-ft functional translator
// (.../CiscoXR/spitfire/packet-processing/state/<NAME>, for example
// L3_NULL_ADJ(D*)) and the name streamed natively by IOS-XR 25.4.15 and later
// (.../<module>:cisco/<asic>/packet-processing/state/<name>).
type ciscoLeaf struct {
	desc  string
	names []string
}

var (
	ciscoRouteLookupFailed = ciscoLeaf{"L3 route lookup failed", []string{"L3_ROUTE_LOOKUP_FAILED", "l3_drop_adj_non_inject"}}
	ciscoNullAdj           = ciscoLeaf{"L3 null adjacency", []string{"L3_NULL_ADJ", "l3_null_adj"}}
	ciscoMPLSLabelMiss     = ciscoLeaf{"MPLS label miss", []string{"MPLS_TE_MIDPOINT_LDP_LABELS_MISS", "mpls_te_midpoint_ldp_labels_miss"}}
)

// matches reports whether k is a packet-processing leaf whose name starts
// with one of the names of c, compared case-insensitively.
func (c ciscoLeaf) matches(k pipelinecounters.LeafKey) bool {
	if k.Class != pipelinecounters.ClassPacketProcessing {
		return false
	}
	name := strings.ToLower(k.RelPath[strings.LastIndexByte(k.RelPath, '/')+1:])
	for _, n := range c.names {
		if strings.HasPrefix(name, strings.ToLower(n)) {
			return true
		}
	}
	return false
}

// portPair holds the DUT and ATE attributes of one link (README "Interface
// addressing").
type portPair struct {
	id  string
	dut attrs.Attributes
	ate attrs.Attributes
}

var portPairs = []portPair{
	{
		id:  "port1",
		dut: attrs.Attributes{Desc: "DUT port1 (ingress A)", IPv4: "192.0.2.1", IPv4Len: ipv4PrefixLen, IPv6: "2001:db8::1", IPv6Len: ipv6PrefixLen, MTU: mtu},
		ate: attrs.Attributes{Name: "atePort1", MAC: "02:00:01:01:01:01", IPv4: "192.0.2.2", IPv4Len: ipv4PrefixLen, IPv6: "2001:db8::2", IPv6Len: ipv6PrefixLen, MTU: mtu},
	},
	{
		id:  "port2",
		dut: attrs.Attributes{Desc: "DUT port2 (ingress B)", IPv4: "192.0.2.5", IPv4Len: ipv4PrefixLen, IPv6: "2001:db8::5", IPv6Len: ipv6PrefixLen, MTU: mtu},
		ate: attrs.Attributes{Name: "atePort2", MAC: "02:00:01:01:01:02", IPv4: "192.0.2.6", IPv4Len: ipv4PrefixLen, IPv6: "2001:db8::6", IPv6Len: ipv6PrefixLen, MTU: mtu},
	},
	{
		id:  "port3",
		dut: attrs.Attributes{Desc: "DUT port3 (ingress C)", IPv4: "192.0.2.9", IPv4Len: ipv4PrefixLen, IPv6: "2001:db8::9", IPv6Len: ipv6PrefixLen, MTU: mtu},
		ate: attrs.Attributes{Name: "atePort3", MAC: "02:00:01:01:01:03", IPv4: "192.0.2.10", IPv4Len: ipv4PrefixLen, IPv6: "2001:db8::a", IPv6Len: ipv6PrefixLen, MTU: mtu},
	},
	{
		id:  "port4",
		dut: attrs.Attributes{Desc: "DUT port4 (egress / edge)", IPv4: "192.0.2.13", IPv4Len: ipv4PrefixLen, IPv6: "2001:db8::d", IPv6Len: ipv6PrefixLen, MTU: mtu},
		ate: attrs.Attributes{Name: "atePort4", MAC: "02:00:01:01:01:04", IPv4: "192.0.2.14", IPv4Len: ipv4PrefixLen, IPv6: "2001:db8::e", IPv6Len: ipv6PrefixLen, MTU: mtu},
	},
}

// flowSpec describes one fixed-count flow of a test step.
type flowSpec struct {
	name    string
	ingress int // index into portPairs
	egress  int // index into portPairs
	dsts    []string
	ipv6    bool
	ttl     uint32
	count   uint32
	pps     uint64
	dropped bool // true if the DUT is expected to drop every packet
}

// testEnv holds the state shared by the sequential subtests.
type testEnv struct {
	dut           *ondatra.DUTDevice
	ate           *ondatra.ATEDevice
	top           gosnappi.Config
	ni            string
	dutMAC        map[string]string // port ID -> DUT interface MAC
	ics           []string          // all INTEGRATED_CIRCUIT components
	icByPort      map[string]string // port ID -> INTEGRATED_CIRCUIT
	icMapComplete bool
	stream        *pipelinecounters.Stream
	streamOpts    pipelinecounters.Options
	routes        map[string]*cfgplugins.StaticRouteCfg // installed static routes by prefix
	discardLeaves map[bool][]string                     // ipv6 -> leaves with ingress sum N in 1.29.6
	// nonIngressErrs collects gNMI-1.29.9.4 violations seen during
	// single-flow steps of gNMI-1.29.6 and gNMI-1.29.8, and nonIngressSteps
	// the steps that were evaluated.
	nonIngressErrs  []string
	nonIngressSteps []string
	// defaultRoute describes the default route(s) found in setup, or is empty
	// if there is none. With a default route, traffic that should be dropped
	// as no-route is forwarded instead, so the no-route steps cannot be
	// validated.
	defaultRoute string
}

func (e *testEnv) dutPort(t *testing.T, i int) *ondatra.Port {
	return e.dut.Port(t, portPairs[i].id)
}

func (e *testEnv) atePort(t *testing.T, i int) *ondatra.Port {
	return e.ate.Port(t, portPairs[i].id)
}

// ingressICs returns the integrated circuits hosting the given ports, or nil
// (meaning all integrated circuits) if the ingress map is not complete.
func (e *testEnv) ingressICs(ports ...int) []string {
	if !e.icMapComplete {
		return nil
	}
	set := map[string]bool{}
	for _, i := range ports {
		set[e.icByPort[portPairs[i].id]] = true
	}
	var out []string
	for ic := range set {
		out = append(out, ic)
	}
	sort.Strings(out)
	return out
}

// TestPipelineCountersDrops runs the README subtests in order. Later subtests
// build on the configuration, subscription and recorded leaves of earlier
// ones.
func TestPipelineCountersDrops(t *testing.T) {
	env := setupEnvironment(t)

	t.Run("gNMI-1.29.1 Packet Processing: L3 Route Lookup Failed", func(t *testing.T) { testCiscoLeafStreamed(t, env, ciscoRouteLookupFailed) })
	t.Run("gNMI-1.29.2 Packet Processing: L3 Null Adjacency", func(t *testing.T) { testLegacyNullAdj(t, env) })
	t.Run("gNMI-1.29.3 Packet Processing: MPLS Label Miss", func(t *testing.T) { testCiscoLeafStreamed(t, env, ciscoMPLSLabelMiss) })
	t.Run("gNMI-1.29.4 Vendor Drop Counter Discovery and Baseline", func(t *testing.T) { testDiscovery(t, env) })
	t.Run("gNMI-1.29.5 Forwarded Traffic Does Not Increment Drop Counters", func(t *testing.T) { testForwardedTraffic(t, env) })
	t.Run("gNMI-1.29.6 IPv4 and IPv6 Discard Route Exact Drop Count", func(t *testing.T) { testDiscardRoutes(t, env) })
	t.Run("gNMI-1.29.7 Dual-Stack Multi-Port Multi-Prefix Drop Accounting", func(t *testing.T) { testScale(t, env) })
	t.Run("gNMI-1.29.8 Edge Link Cut Drop Accounting", func(t *testing.T) { testEdgeCut(t, env) })
	t.Run("gNMI-1.29.9 Negative Tests", func(t *testing.T) { testNegative(t, env) })
}

// setupEnvironment implements README "Test environment setup".
func setupEnvironment(t *testing.T) *testEnv {
	t.Helper()
	env := &testEnv{
		dut:           ondatra.DUT(t, "dut"),
		ate:           ondatra.ATE(t, "ate"),
		dutMAC:        map[string]string{},
		routes:        map[string]*cfgplugins.StaticRouteCfg{},
		discardLeaves: map[bool][]string{},
	}
	env.ni = deviations.DefaultNetworkInstance(env.dut)
	env.streamOpts = pipelinecounters.Options{
		SampleInterval:       sampleInterval,
		FunctionalTranslator: deviations.CiscoxrVendordropFt(env.dut),
	}
	registerCleanup(t, env)

	t.Log("Setup: configure the default network instance (name and type DEFAULT_INSTANCE).")
	fptest.ConfigureDefaultNetworkInstance(t, env.dut)

	t.Log("Setup: configure 4 DUT ports (type ethernetCsmacd, enabled, L2 MTU 1514, IPv4/IPv6 MTU 1500) in the default network instance.")
	for i, pp := range portPairs {
		dp := env.dutPort(t, i)
		gnmi.Replace(t, env.dut, gnmi.OC().Interface(dp.Name()).Config(), pp.dut.NewOCInterface(dp.Name(), env.dut))
		if deviations.ExplicitPortSpeed(env.dut) {
			fptest.SetPortSpeed(t, dp)
		}
		if deviations.ExplicitInterfaceInDefaultVRF(env.dut) {
			fptest.AssignToNetworkInstance(t, env.dut, dp.Name(), env.ni, 0)
		}
	}

	t.Log("Setup: configure the ATE ports and push the OTG configuration.")
	env.top = gosnappi.NewConfig()
	for i := range portPairs {
		portPairs[i].ate.AddToOTG(env.top, env.atePort(t, i), &portPairs[i].dut)
	}
	env.ate.OTG().PushConfig(t, env.top)
	env.ate.OTG().StartProtocols(t)

	t.Log("Readiness gate: oper-status UP on all 4 DUT ports.")
	upDeadline := time.Now().Add(awaitTimeout)
	for i := range portPairs {
		dp := env.dutPort(t, i)
		gnmi.Await(t, env.dut, gnmi.OC().Interface(dp.Name()).OperStatus().State(), remaining(upDeadline), oc.Interface_OperStatus_UP)
		// Read one leaf per port: a batch query asks for
		// .../ethernet/*/mac-address, which some devices (Cisco XR) reject.
		env.dutMAC[portPairs[i].id] = gnmi.Get(t, env.dut, gnmi.OC().Interface(dp.Name()).Ethernet().MacAddress().State())
	}
	awaitNeighbors(t, env, 0, 1, 2, 3)

	t.Log("Readiness gate: every INTEGRATED_CIRCUIT component is ACTIVE.")
	env.ics = components.FindComponentsByType(t, env.dut, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_INTEGRATED_CIRCUIT)
	sort.Strings(env.ics)
	awaitICsActive(t, env)

	t.Log("Prefix plan: verify that no default route (0.0.0.0/0 or ::/0) is present on the DUT.")
	afts := gnmi.OC().NetworkInstance(env.ni).Afts()
	var defaults []string
	checkDefault := func(prefix string, origin oc.E_PolicyTypes_INSTALL_PROTOCOL_TYPE, nhg uint64) {
		desc, forwards := describeAFTEntry(t, env, origin, nhg)
		if !forwards {
			// An entry without any forwarding next hop drops the traffic, like
			// having no route (for example the implicit "default reject" route
			// of Junos or the implicit "0.0.0.0 via Null0" / ":: via Null0"
			// entry of IOS-XR).
			t.Logf("Default route %s has no forwarding next hop (%s); treated as an implicit discard route.", prefix, desc)
			return
		}
		defaults = append(defaults, prefix+": "+desc)
	}
	if e, ok := gnmi.Lookup(t, env.dut, afts.Ipv4Entry("0.0.0.0/0").State()).Val(); ok {
		checkDefault("0.0.0.0/0", e.GetOriginProtocol(), e.GetNextHopGroup())
	}
	if e, ok := gnmi.Lookup(t, env.dut, afts.Ipv6Entry("::/0").State()).Val(); ok {
		checkDefault("::/0", e.GetOriginProtocol(), e.GetNextHopGroup())
	}
	if len(defaults) > 0 {
		env.defaultRoute = strings.Join(defaults, "; ")
		t.Errorf("Default route present in the AFT of network instance %s (%s); the test requires no default route. If it is a management route, place management in a separate VRF. The no-route steps (gNMI-1.29.8 Step 5, gNMI-1.29.9.2, gNMI-1.29.9.3) will fail; all other subtests still run.", env.ni, env.defaultRoute)
	}
	return env
}

// awaitNeighbors waits for ARP and IPv6 neighbor discovery between the given
// ATE ports and their DUT ports, on both the DUT (gNMI) and the ATE.
func awaitNeighbors(t *testing.T, env *testEnv, ports ...int) {
	t.Helper()
	t.Log("Readiness gate: IPv4 link-layer-address and IPv6 neighbor-state on the DUT; ARP/ND on the ATE.")
	v4Deadline := time.Now().Add(awaitTimeout)
	v6Deadline := time.Now().Add(awaitTimeout)
	var ateIfs []string
	for _, i := range ports {
		dp := env.dutPort(t, i)
		pp := portPairs[i]
		ateIfs = append(ateIfs, pp.ate.Name+".Eth")
		sub := gnmi.OC().Interface(dp.Name()).Subinterface(0)
		if !awaitResubscribe(t, env.dut, sub.Ipv4().Neighbor(pp.ate.IPv4).LinkLayerAddress().State(), v4Deadline, func(v *ygnmi.Value[string]) bool {
			mac, present := v.Val()
			return present && mac != ""
		}) {
			t.Fatalf("IPv4 neighbor %s not resolved on %s within %v", pp.ate.IPv4, dp.Name(), awaitTimeout)
		}
		if !awaitResubscribe(t, env.dut, sub.Ipv6().Neighbor(pp.ate.IPv6).NeighborState().State(), v6Deadline, func(v *ygnmi.Value[oc.E_Neighbor_NeighborState]) bool {
			s, present := v.Val()
			return present && (s == oc.Neighbor_NeighborState_REACHABLE || s == oc.Neighbor_NeighborState_STALE)
		}) {
			t.Fatalf("IPv6 neighbor %s not REACHABLE/STALE on %s within %v", pp.ate.IPv6, dp.Name(), awaitTimeout)
		}
	}
	awaitOTGNeighbors(t, env, ateIfs)
}

// remaining returns the time left until deadline, but at least one second.
// Readiness gates (README "Readiness gates") create one deadline of
// awaitTimeout per condition and pass remaining(deadline) to every wait of
// that condition, so the timeout applies to the condition as a whole and not
// to each port, neighbor, component, route or flow.
func remaining(deadline time.Time) time.Duration {
	if r := time.Until(deadline); r > time.Second {
		return r
	}
	return time.Second
}

// awaitResubscribe waits until match accepts the value of q or deadline
// passes, and reports whether match succeeded. Some devices send only the
// initial state of a keyed path (an AFT entry, the AFT container or an
// interface counter) and no later updates, so a single streaming watch can
// miss the change. The path is therefore watched for at most
// resubscribeInterval at a time and subscribed again, which re-reads its
// current state, until the deadline.
func awaitResubscribe[T any](t *testing.T, dut *ondatra.DUTDevice, q ygnmi.SingletonQuery[T], deadline time.Time, match func(*ygnmi.Value[T]) bool) bool {
	t.Helper()
	for time.Now().Before(deadline) {
		budget := remaining(deadline)
		if budget > resubscribeInterval {
			budget = resubscribeInterval
		}
		if _, ok := gnmi.Watch(t, dut, q, budget, match).Await(t); ok {
			return true
		}
	}
	return false
}

// awaitICsActive verifies that every INTEGRATED_CIRCUIT component that
// reports /components/component/state/oper-status is ACTIVE. The oper-status
// of all components is read with a single wildcard query, because some
// platforms answer a query for a component without oper-status only after a
// long delay, so one query per component can take tens of minutes. Components
// that are not ACTIVE yet share a single awaitTimeout deadline. Components
// that do not report oper-status, or that report DISABLED (administratively
// out of service, for example an empty fabric card slot), are logged and
// skipped.
func awaitICsActive(t *testing.T, env *testEnv) {
	t.Helper()
	if len(env.ics) == 0 {
		t.Fatalf("No INTEGRATED_CIRCUIT component found on %s", env.dut.Name())
	}
	active := oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE
	status := map[string]oc.E_PlatformTypes_COMPONENT_OPER_STATUS{}
	for _, v := range gnmi.LookupAll(t, env.dut, gnmi.OC().ComponentAny().OperStatus().State()) {
		s, ok := v.Val()
		if !ok {
			continue
		}
		if name := componentName(v.Path); name != "" {
			status[name] = s
		}
	}
	var pending, notReported, disabled []string
	for _, ic := range env.ics {
		s, ok := status[ic]
		switch {
		case !ok:
			notReported = append(notReported, ic)
		case s == oc.PlatformTypes_COMPONENT_OPER_STATUS_DISABLED:
			disabled = append(disabled, ic)
		case s != active:
			pending = append(pending, ic)
		}
	}
	if len(notReported) > 0 {
		t.Logf("Integrated circuits without oper-status (skipped): %v", notReported)
	}
	if len(disabled) > 0 {
		t.Logf("Integrated circuits with oper-status DISABLED (skipped): %v", disabled)
	}
	deadline := time.Now().Add(awaitTimeout)
	for _, ic := range pending {
		got, ok := gnmi.Watch(t, env.dut, gnmi.OC().Component(ic).OperStatus().State(), remaining(deadline), func(v *ygnmi.Value[oc.E_PlatformTypes_COMPONENT_OPER_STATUS]) bool {
			s, present := v.Val()
			return present && s == active
		}).Await(t)
		if !ok {
			state := "unknown"
			if got != nil {
				if s, present := got.Val(); present {
					state = s.String()
				}
			}
			t.Errorf("Integrated circuit %s is not ACTIVE (oper-status %s)", ic, state)
		}
	}
	t.Logf("Integrated circuits checked: %d total, %d without oper-status, %d DISABLED, %d were not ACTIVE at first read.", len(env.ics), len(notReported), len(disabled), len(pending))
}

// componentName returns the name key of the component element of a gNMI path
// under /components/component, or "" if the path has none.
func componentName(p *gpb.Path) string {
	for _, e := range p.GetElem() {
		if e.GetName() == "component" {
			return e.GetKey()["name"]
		}
	}
	return ""
}

// describeAFTEntry returns the origin protocol and the next hops (IP address
// and interface) of an AFT entry, for diagnosing an unexpected route.
// forwards is false only if at least one next hop of the next-hop group is
// read and none of them forwards traffic (empty IP and interface, or an
// implicit entry with unset origin protocol pointing to Null0/discard with an
// unspecified IP). Forwarding is assumed if the next-hop group or its next
// hops cannot be read.
func describeAFTEntry(t *testing.T, env *testEnv, origin oc.E_PolicyTypes_INSTALL_PROTOCOL_TYPE, nhgID uint64) (desc string, forwards bool) {
	t.Helper()
	orNone := func(s string) string {
		if s == "" {
			return "<none>"
		}
		return s
	}
	isImplicitDiscard := func(ip, intf string) bool {
		if ip == "" && intf == "" {
			return true
		}
		if origin != oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_UNSET {
			return false
		}
		if ip != "" && ip != "0.0.0.0" && ip != "::" {
			return false
		}
		return strings.EqualFold(intf, "Null0") || strings.EqualFold(intf, "discard")
	}
	afts := gnmi.OC().NetworkInstance(env.ni).Afts()
	var nhs []string
	read, anyForwarding := 0, false
	if nhg, ok := gnmi.Lookup(t, env.dut, afts.NextHopGroup(nhgID).State()).Val(); ok {
		for idx := range nhg.NextHop {
			if nh, ok := gnmi.Lookup(t, env.dut, afts.NextHop(idx).State()).Val(); ok {
				read++
				ip, intf := nh.GetIpAddress(), nh.GetInterfaceRef().GetInterface()
				if !isImplicitDiscard(ip, intf) {
					anyForwarding = true
				}
				nhs = append(nhs, fmt.Sprintf("%s via %s", orNone(ip), orNone(intf)))
			}
		}
	}
	forwards = read == 0 || anyForwarding
	sort.Strings(nhs)
	originStr := "not reported"
	if origin != oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_UNSET {
		originStr = origin.String()
	}
	return fmt.Sprintf("origin-protocol %s, next-hop-group %d, next hops %v", originStr, nhgID, nhs), forwards
}

// ensureStream opens the drop subscription (README "Counter collection and
// settling") if it is not open yet.
func ensureStream(t *testing.T, env *testEnv) {
	t.Helper()
	if env.stream != nil {
		return
	}
	s := pipelinecounters.NewStream(t, env.dut, env.streamOpts)
	if ok, err := s.AwaitSync(awaitTimeout); err != nil || !ok {
		s.Close()
		t.Fatalf("Drop subscription did not sync within %v: err=%v", awaitTimeout, err)
	}
	// Only a synced subscription is shared with later subtests.
	env.stream = s
}

// installRoutes configures static routes in a single gNMI Set and awaits them
// in the static route state and the AFT.
func installRoutes(t *testing.T, env *testEnv, routes map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union) {
	t.Helper()
	b := &gnmi.SetBatch{}
	var prefixes []string
	cfgs := map[string]*cfgplugins.StaticRouteCfg{}
	for prefix, nh := range routes {
		cfg := &cfgplugins.StaticRouteCfg{
			NetworkInstance: env.ni,
			Prefix:          prefix,
			NextHops:        map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{"0": nh},
		}
		if _, err := cfgplugins.NewStaticRouteCfg(b, cfg, env.dut); err != nil {
			t.Fatalf("Failed to build static route %s: %v", prefix, err)
		}
		cfgs[prefix] = cfg
		prefixes = append(prefixes, prefix)
	}
	sort.Strings(prefixes)
	t.Logf("Push static routes %v using gNMI Set.", prefixes)
	b.Set(t, env.dut)
	// Record the routes for cleanup only once the Set succeeded.
	for p, cfg := range cfgs {
		env.routes[p] = cfg
	}
	awaitStaticRouteState(t, env, prefixes)
	awaitStaticRoutesInAFT(t, env, prefixes, true)
}

// awaitStaticRouteState waits until every prefix is reported under
// .../static-routes/static/state/prefix within one shared awaitTimeout
// deadline, using awaitResubscribe so platforms that only emit initial state
// on keyed paths do not stall.
func awaitStaticRouteState(t *testing.T, env *testEnv, prefixes []string) {
	t.Helper()
	sp := gnmi.OC().NetworkInstance(env.ni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(env.dut))
	deadline := time.Now().Add(awaitTimeout)
	for _, p := range prefixes {
		if !awaitResubscribe(t, env.dut, sp.Static(p).Prefix().State(), deadline, func(v *ygnmi.Value[string]) bool {
			got, present := v.Val()
			return present && got == p
		}) {
			t.Fatalf("Static route state %s in network instance %s not reported within %v", p, env.ni, awaitTimeout)
		}
	}
}

// removeRoutes deletes static routes and awaits their removal from the AFT.
func removeRoutes(t *testing.T, env *testEnv, prefixes ...string) {
	t.Helper()
	b := &gnmi.SetBatch{}
	var removed []string
	for _, p := range prefixes {
		if _, ok := env.routes[p]; !ok {
			continue
		}
		if _, err := cfgplugins.NewStaticRouteCfg(b, &cfgplugins.StaticRouteCfg{NetworkInstance: env.ni, Prefix: p, RemoveStaticRoute: true}, env.dut); err != nil {
			t.Fatalf("Failed to build static route deletion %s: %v", p, err)
		}
		removed = append(removed, p)
	}
	if len(removed) == 0 {
		return
	}
	t.Logf("Delete static routes %v using gNMI Set.", removed)
	b.Set(t, env.dut)
	// Stop tracking the routes for cleanup only once the Set succeeded.
	for _, p := range removed {
		delete(env.routes, p)
	}
	awaitStaticRoutesInAFT(t, env, removed, false)
}

func dropUnion() oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union {
	return oc.LocalRouting_LOCAL_DEFINED_NEXT_HOP_DROP
}

// awaitStaticRoutesInAFT waits until every prefix is present in (present ==
// true) or absent from (present == false) the AFT of the test network
// instance as a static route. A present entry must report origin-protocol
// STATIC; when waiting for absence, an entry of another origin (a route the
// test did not configure) also counts as absent. Prefixes containing ":" are
// IPv6.
// All prefixes share one awaitTimeout deadline, and each prefix is waited
// for with awaitResubscribe.
func awaitStaticRoutesInAFT(t *testing.T, env *testEnv, prefixes []string, present bool) {
	t.Helper()
	static := oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC
	afts := gnmi.OC().NetworkInstance(env.ni).Afts()
	deadline := time.Now().Add(awaitTimeout)
	for _, prefix := range prefixes {
		var ok bool
		if strings.Contains(prefix, ":") {
			ok = awaitResubscribe(t, env.dut, afts.Ipv6Entry(prefix).State(), deadline, func(v *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv6Entry]) bool {
				e, isPresent := v.Val()
				return (isPresent && e.GetOriginProtocol() == static) == present
			})
		} else {
			ok = awaitResubscribe(t, env.dut, afts.Ipv4Entry(prefix).State(), deadline, func(v *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv4Entry]) bool {
				e, isPresent := v.Val()
				return (isPresent && e.GetOriginProtocol() == static) == present
			})
		}
		if !ok {
			t.Fatalf("AFT entry %s in network instance %s: want present=%t (origin-protocol STATIC) within %v", prefix, env.ni, present, awaitTimeout)
		}
	}
}

// maxParentDepth bounds the walk up the component tree in
// integratedCircuitForPort, protecting against malformed parent loops.
const maxParentDepth = 8

// integratedCircuitForPort returns the INTEGRATED_CIRCUIT component that hosts
// a DUT port: it reads /interfaces/interface/state/hardware-port and follows
// /components/component/state/parent upwards. ok is false if any step is not
// reported by the device.
func integratedCircuitForPort(t *testing.T, dut *ondatra.DUTDevice, p *ondatra.Port) (string, bool) {
	t.Helper()
	name, ok := gnmi.Lookup(t, dut, gnmi.OC().Interface(p.Name()).HardwarePort().State()).Val()
	if !ok || name == "" {
		return "", false
	}
	for depth := 0; depth < maxParentDepth; depth++ {
		parent, ok := gnmi.Lookup(t, dut, gnmi.OC().Component(name).Parent().State()).Val()
		if !ok || parent == "" || parent == name {
			return "", false
		}
		if cType, ok := gnmi.Lookup(t, dut, gnmi.OC().Component(parent).Type().State()).Val(); ok {
			if v, isHW := cType.(oc.E_PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT); isHW && v == oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_INTEGRATED_CIRCUIT {
				return parent, true
			}
		}
		name = parent
	}
	return "", false
}

// integratedCircuitMap maps ondatra port ID to the INTEGRATED_CIRCUIT hosting
// that port. complete is false if any port could not be mapped; the map then
// only contains the mapped ports.
func integratedCircuitMap(t *testing.T, dut *ondatra.DUTDevice, ports []*ondatra.Port) (map[string]string, bool) {
	t.Helper()
	m := make(map[string]string, len(ports))
	complete := true
	for _, p := range ports {
		ic, ok := integratedCircuitForPort(t, dut, p)
		if !ok {
			complete = false
			continue
		}
		m[p.ID()] = ic
	}
	return m, complete
}

// runFlows replaces the OTG flows with specs, starts them and waits until all
// fixed-count flows have stopped. It returns tx/rx per flow.
//
// ARP/ND is awaited only on the ATE interfaces that the flows depend on: the
// ingress interface of every flow and the egress interface of every forwarded
// flow. The egress interface of a dropped flow may be down on purpose (for
// example dut:port4 during the gNMI-1.29.8 edge link cut).
func runFlows(t *testing.T, env *testEnv, specs []flowSpec) map[string][2]uint64 {
	t.Helper()
	env.top.Flows().Clear()
	var names []string
	arpIfs := map[string]bool{}
	for _, f := range specs {
		in, out := portPairs[f.ingress], portPairs[f.egress]
		arpIfs[in.ate.Name+".Eth"] = true
		if !f.dropped {
			arpIfs[out.ate.Name+".Eth"] = true
		}
		count := f.count
		if count == 0 {
			count = pktCount
		}
		addFixedFlow(t, env, f, count)
		names = append(names, f.name)
		t.Logf("Flow %s: %s -> %s, dst %v, %d packets.", f.name, in.id, out.id, f.dsts, count)
	}
	otg := env.ate.OTG()
	otg.PushConfig(t, env.top)
	otg.StartProtocols(t)
	var ifNames []string
	for n := range arpIfs {
		ifNames = append(ifNames, n)
	}
	sort.Strings(ifNames)
	awaitOTGNeighbors(t, env, ifNames)
	t.Logf("Start traffic and await the stopped transmit state of %v.", names)
	otg.StartTraffic(t)
	deadline := time.Now().Add(awaitTimeout)
	for _, n := range names {
		if _, ok := gnmi.Watch(t, otg, gnmi.OTG().Flow(n).Transmit().State(), remaining(deadline), func(v *ygnmi.Value[bool]) bool {
			transmit, present := v.Val()
			return present && !transmit
		}).Await(t); !ok {
			otg.StopTraffic(t)
			t.Fatalf("Flow %s did not stop transmitting within %v", n, awaitTimeout)
		}
	}
	// OTG flow counters can lag the stopped transmit state. For a forwarded
	// flow, wait until every transmitted packet is counted as received (until
	// the shared deadline; the caller reports any loss). A dropped flow is
	// expected to receive nothing; the callers re-read its received count
	// with flowRx after the drop counters settled.
	res := map[string][2]uint64{}
	for _, f := range specs {
		q := gnmi.OTG().Flow(f.name).Counters()
		tx, _ := gnmi.Lookup(t, otg, q.OutPkts().State()).Val()
		if !f.dropped {
			gnmi.Watch(t, otg, q.InPkts().State(), remaining(deadline), func(v *ygnmi.Value[uint64]) bool {
				rx, present := v.Val()
				return present && rx >= tx
			}).Await(t)
		}
		rx, _ := gnmi.Lookup(t, otg, q.InPkts().State()).Val()
		res[f.name] = [2]uint64{tx, rx}
	}
	return res
}

// flowRx re-reads the received packet count of an OTG flow. It is used for
// dropped flows after the drop counters settled, so that packets counted late
// by the ATE are not missed.
func flowRx(t *testing.T, env *testEnv, name string) uint64 {
	t.Helper()
	rx, _ := gnmi.Lookup(t, env.ate.OTG(), gnmi.OTG().Flow(name).Counters().InPkts().State()).Val()
	return rx
}

// Frame size, rate, TTL and UDP ports of every flow (README "Standard flow
// template").
const (
	flowFrameSize  = 512
	flowDefaultPPS = 100
	flowDefaultTTL = 64
	flowUDPSrcPort = 49152
	flowUDPDstPort = 49153
)

// addFixedFlow adds a port-based OTG flow for f that transmits exactly count
// packets from the ATE ingress port to the DUT ingress port MAC, with flow
// metrics enabled. Multiple destination addresses are cycled.
func addFixedFlow(t *testing.T, env *testEnv, f flowSpec, count uint32) {
	t.Helper()
	in := portPairs[f.ingress]
	ttl := f.ttl
	if ttl == 0 {
		ttl = flowDefaultTTL
	}
	pps := f.pps
	if pps == 0 {
		pps = flowDefaultPPS
	}
	flow := env.top.Flows().Add().SetName(f.name)
	flow.Metrics().SetEnable(true)
	flow.TxRx().Port().SetTxName(env.atePort(t, f.ingress).ID()).SetRxNames([]string{env.atePort(t, f.egress).ID()})
	flow.Size().SetFixed(flowFrameSize)
	flow.Rate().SetPps(pps)
	flow.Duration().FixedPackets().SetPackets(count)
	eth := flow.Packet().Add().Ethernet()
	eth.Src().SetValue(in.ate.MAC)
	eth.Dst().SetValue(env.dutMAC[in.id])
	if f.ipv6 {
		v6 := flow.Packet().Add().Ipv6()
		v6.Src().SetValue(in.ate.IPv6)
		if len(f.dsts) == 1 {
			v6.Dst().SetValue(f.dsts[0])
		} else {
			v6.Dst().SetValues(f.dsts)
		}
		v6.HopLimit().SetValue(ttl)
	} else {
		v4 := flow.Packet().Add().Ipv4()
		v4.Src().SetValue(in.ate.IPv4)
		if len(f.dsts) == 1 {
			v4.Dst().SetValue(f.dsts[0])
		} else {
			v4.Dst().SetValues(f.dsts)
		}
		v4.TimeToLive().SetValue(ttl)
	}
	udp := flow.Packet().Add().Udp()
	udp.SrcPort().SetValue(flowUDPSrcPort)
	udp.DstPort().SetValue(flowUDPDstPort)
}

// awaitOTGNeighbors waits until IPv4 ARP and IPv6 neighbor discovery are
// resolved on the given OTG Ethernet interfaces (for example "atePort1.Eth").
// Other OTG interfaces are not checked, so that an interface whose DUT port is
// disabled on purpose does not block the wait.
func awaitOTGNeighbors(t *testing.T, env *testEnv, ifNames []string) {
	t.Helper()
	otg := env.ate.OTG()
	want := map[string]bool{}
	for _, n := range ifNames {
		want[n] = true
	}
	present := func(v *ygnmi.Value[string]) bool {
		mac, ok := v.Val()
		return ok && mac != ""
	}
	deadline := time.Now().Add(awaitTimeout)
	for _, d := range env.top.Devices().Items() {
		eth := d.Ethernets().Items()[0]
		if !want[eth.Name()] {
			continue
		}
		if len(eth.Ipv4Addresses().Items()) > 0 {
			if _, ok := gnmi.WatchAll(t, otg, gnmi.OTG().Interface(eth.Name()).Ipv4NeighborAny().LinkLayerAddress().State(), remaining(deadline), present).Await(t); !ok {
				t.Fatalf("Did not receive OTG IPv4 neighbor entry for interface %s within %v", eth.Name(), awaitTimeout)
			}
		}
		for _, ip := range eth.Ipv6Addresses().Items() {
			if gw := ip.Gateway(); gw != "" {
				if _, ok := gnmi.Watch(t, otg, gnmi.OTG().Interface(eth.Name()).Ipv6Neighbor(gw).LinkLayerAddress().State(), remaining(deadline), present).Await(t); !ok {
					t.Fatalf("Did not receive OTG IPv6 neighbor entry for gateway %s on interface %s within %v", gw, eth.Name(), awaitTimeout)
				}
			}
		}
	}
}

// ifCounters returns in-unicast-pkts, in-errors and in-discards of a DUT port.
func ifCounters(t *testing.T, env *testEnv, i int) (unicast, errs, discards uint64) {
	t.Helper()
	c := gnmi.Get(t, env.dut, gnmi.OC().Interface(env.dutPort(t, i).Name()).Counters().State())
	return c.GetInUnicastPkts(), c.GetInErrors(), c.GetInDiscards()
}

type aggField struct {
	name string
	get  func(pipelinecounters.Aggregates) *uint64
}

var (
	ppAgg   = aggField{"packet-processing-aggregate", func(a pipelinecounters.Aggregates) *uint64 { return a.PacketProcessing }}
	advAgg  = aggField{"adverse-aggregate", func(a pipelinecounters.Aggregates) *uint64 { return a.Adverse }}
	congAgg = aggField{"congestion-aggregate", func(a pipelinecounters.Aggregates) *uint64 { return a.Congestion }}
	nrAgg   = aggField{"lookup-block/no-route", func(a pipelinecounters.Aggregates) *uint64 { return a.NoRoute }}
	nnhAgg  = aggField{"lookup-block/no-nexthop", func(a pipelinecounters.Aggregates) *uint64 { return a.NoNexthop }}
)

// aggDeltas returns the delta of an OC aggregate on every integrated circuit
// of ics that reports it. A decrease is only logged here: it is a
// monotonicity failure, reported by the monotonicity check (the aggregates are
// part of the drop subscription).
func aggDeltas(t *testing.T, base, end map[string]pipelinecounters.Aggregates, ics []string, f aggField) map[string]uint64 {
	t.Helper()
	out := map[string]uint64{}
	for _, ic := range ics {
		d, err := pipelinecounters.AggregateDelta(base, end, []string{ic}, f.get)
		switch {
		case errors.Is(err, pipelinecounters.ErrNotReported):
			continue
		case err != nil:
			t.Logf("%s on %s: %v (reported by the monotonicity check)", f.name, ic, err)
			continue
		}
		out[ic] = d
	}
	return out
}

func sumValues(m map[string]uint64) uint64 {
	var s uint64
	for _, v := range m {
		s += v
	}
	return s
}

// logP3P7 implements the log-only checks P3 and P7. expected maps an
// integrated circuit to its expected drop count; "" is used for a total.
func logP3P7(t *testing.T, d pipelinecounters.Snapshot, aggBase, aggEnd map[string]pipelinecounters.Aggregates, p3ICs, p7ICs []string, expected map[string]uint64) {
	t.Helper()
	pp := aggDeltas(t, aggBase, aggEnd, p3ICs, ppAgg)
	if len(pp) == 0 {
		t.Log("P3 (log only): packet-processing-aggregate is not reported.")
	}
	for key, want := range expected {
		label := key
		got, reported := sumValues(pp), len(pp) > 0
		if key != "" {
			got, reported = pp[key]
		} else {
			label = "ingress ICs"
		}
		switch {
		case !reported:
			t.Logf("P3 (log only): %q packet-processing-aggregate not reported.", label)
		case got != want:
			t.Logf("P3 (log only) WARNING: %q packet-processing-aggregate delta = %d, want %d", label, got, want)
		default:
			t.Logf("P3 (log only): %q packet-processing-aggregate delta = %d as expected.", label, got)
		}
	}
	t.Log("P7 (log only): compare the sum of vendor packet-processing leaves with packet-processing-aggregate per integrated circuit.")
	ppAll := aggDeltas(t, aggBase, aggEnd, p7ICs, ppAgg)
	notReported := 0
	for _, ic := range p7ICs {
		var vendorPP uint64
		for k, v := range d {
			if k.Component == ic && k.Class == pipelinecounters.ClassPacketProcessing {
				vendorPP += v
			}
		}
		agg, ok := ppAll[ic]
		switch {
		case !ok:
			// Many platforms type non-forwarding chips (for example power
			// regulators) as INTEGRATED_CIRCUIT; only count them.
			notReported++
		case agg != vendorPP:
			t.Logf("P7 (log only) WARNING: %s vendor packet-processing sum %d != aggregate delta %d", ic, vendorPP, agg)
		default:
			t.Logf("P7 (log only): %s vendor packet-processing sum matches the aggregate (%d).", ic, agg)
		}
	}
	if notReported > 0 {
		t.Logf("P7 (log only): %d of %d integrated circuits do not report packet-processing-aggregate.", notReported, len(p7ICs))
	}
}

// ifBaseline holds in-unicast-pkts, in-errors and in-discards of a DUT port.
type ifBaseline [3]uint64

// checkP1 implements P1 for one DUT port: in-unicast-pkts delta >= want,
// in-errors delta == 0, in-discards delta logged.
func checkP1(t *testing.T, env *testEnv, i int, base ifBaseline, want uint64) {
	t.Helper()
	dp := env.dutPort(t, i)
	target := base[0] + want
	if ok := awaitResubscribe(t, env.dut, gnmi.OC().Interface(dp.Name()).Counters().InUnicastPkts().State(), time.Now().Add(awaitTimeout), func(v *ygnmi.Value[uint64]) bool {
		got, present := v.Val()
		return present && got >= target
	}); !ok {
		t.Errorf("P1: %s in-unicast-pkts did not increase by >= %d", dp.Name(), want)
	}
	_, e, disc := ifCounters(t, env, i)
	switch {
	case e < base[1]:
		t.Logf("P1: %s in-errors decreased from %d to %d (logged only).", dp.Name(), base[1], e)
	case e > base[1]:
		t.Errorf("P1: %s in-errors increased by %d, want 0", dp.Name(), e-base[1])
	}
	if disc < base[2] {
		t.Logf("P1: %s in-discards decreased from %d to %d (logged only).", dp.Name(), base[2], disc)
	} else {
		t.Logf("P1: %s in-discards delta = %d (logged only).", dp.Name(), disc-base[2])
	}
}

func ifBaselineOf(t *testing.T, env *testEnv, i int) ifBaseline {
	t.Helper()
	u, e, d := ifCounters(t, env, i)
	return ifBaseline{u, e, d}
}

// dropResult is returned by validateDrops.
type dropResult struct {
	deltas  pipelinecounters.Snapshot
	matches []string // leaves whose ingress sum is exactly N
	aggBase map[string]pipelinecounters.Aggregates
	aggEnd  map[string]pipelinecounters.Aggregates
}

// validateDrops implements README "Standard drop validation" (P0 to P7 and
// logging) for flows that are all expected to be dropped.
func validateDrops(t *testing.T, env *testEnv, specs []flowSpec) dropResult {
	t.Helper()
	return validateDropsOpt(t, env, specs, false)
}

// validateEdgeCutDrops is validateDrops for the gNMI-1.29.8 edge link cut. If
// no vendor drop leaf increments at all, P4 falls back to the control plane
// dropped-aggregate: traffic that matches no route may be punted to the CPU
// and dropped by control plane policing (CoPP) instead of being counted as a
// pipeline drop.
func validateEdgeCutDrops(t *testing.T, env *testEnv, specs []flowSpec) dropResult {
	t.Helper()
	return validateDropsOpt(t, env, specs, true)
}

// readControlPlaneDrops returns the sum of
// /components/component/integrated-circuit/pipeline-counters/control-plane-traffic/state/dropped-aggregate
// over all integrated circuits that report it.
func readControlPlaneDrops(t *testing.T, dut *ondatra.DUTDevice) uint64 {
	t.Helper()
	var sum uint64
	reported := 0
	for _, v := range gnmi.LookupAll(t, dut, gnmi.OC().ComponentAny().IntegratedCircuit().PipelineCounters().ControlPlaneTraffic().DroppedAggregate().State()) {
		if d, ok := v.Val(); ok {
			sum += d
			reported++
		}
	}
	t.Logf("Control plane dropped-aggregate: %d (summed over %d integrated circuits).", sum, reported)
	return sum
}

// readControlPlanePunted returns the sum of
// /components/component/integrated-circuit/pipeline-counters/control-plane-traffic/state/queued-aggregate
// and .../dropped-aggregate over all integrated circuits that report them:
// every packet punted to the control plane is either enqueued to the CPU or
// dropped by control plane policing.
func readControlPlanePunted(t *testing.T, dut *ondatra.DUTDevice) uint64 {
	t.Helper()
	cp := gnmi.OC().ComponentAny().IntegratedCircuit().PipelineCounters().ControlPlaneTraffic()
	var queued uint64
	reported := 0
	for _, v := range gnmi.LookupAll(t, dut, cp.QueuedAggregate().State()) {
		if q, ok := v.Val(); ok {
			queued += q
			reported++
		}
	}
	t.Logf("Control plane queued-aggregate: %d (summed over %d integrated circuits).", queued, reported)
	return queued + readControlPlaneDrops(t, dut)
}

// validateDropsOpt implements validateDrops. If cpFallback is true, P4 also
// passes when no vendor drop leaf increments but the control plane
// dropped-aggregate delta is at least N.
func validateDropsOpt(t *testing.T, env *testEnv, specs []flowSpec, cpFallback bool) dropResult {
	t.Helper()
	var n uint64
	ingressPorts := map[int]uint64{}
	for _, f := range specs {
		c := uint64(f.count)
		if c == 0 {
			c = pktCount
		}
		n += c
		ingressPorts[f.ingress] += c
	}
	var portIdx []int
	for i := range ingressPorts {
		portIdx = append(portIdx, i)
	}
	sort.Ints(portIdx)
	ics := env.ingressICs(portIdx...)

	t.Logf("Take a quiet baseline (expected drops N = %d).", n)
	base := env.stream.AwaitQuietBaseline(t, quietRetries, roundTimeout)
	aggBase := pipelinecounters.AggregatesFrom(base, env.ics)
	ifBase := map[int]ifBaseline{}
	for _, i := range portIdx {
		ifBase[i] = ifBaselineOf(t, env, i)
	}
	var cpBase uint64
	if cpFallback {
		t.Log("Record the control plane dropped-aggregate baseline.")
		cpBase = readControlPlaneDrops(t, env.dut)
	}

	counts := runFlows(t, env, specs)

	t.Log("P0: the ATE transmitted exactly the configured number of packets.")
	for _, f := range specs {
		want := uint64(f.count)
		if want == 0 {
			want = pktCount
		}
		if got := counts[f.name][0]; got != want {
			t.Fatalf("P0: traffic generator error: flow %s transmitted %d packets, want %d", f.name, got, want)
		}
	}

	t.Log("Record the settled final value.")
	final, ok := env.stream.AwaitIngressSum(t, base, ics, n, settleTimeout)
	if !ok && !cpFallback {
		t.Errorf("No vendor drop leaf reached an ingress sum of %d within %v", n, settleTimeout)
	}
	d := pipelinecounters.Deltas(base, final)
	aggEnd := pipelinecounters.AggregatesFrom(final, env.ics)
	t.Log("Log: drop counters that incremented:")
	pipelinecounters.LogIncremented(t, d)

	t.Log("P1: DUT ingress in-unicast-pkts delta >= N per port, in-errors delta == 0; in-discards logged.")
	for _, i := range portIdx {
		checkP1(t, env, i, ifBase[i], ingressPorts[i])
	}

	t.Log("P2: the ATE egress port received 0 packets of each dropped flow.")
	for _, f := range specs {
		if rx := flowRx(t, env, f.name); rx != 0 {
			t.Errorf("P2: flow %s received %d packets, want 0", f.name, rx)
		}
	}

	aggICs := ics
	if aggICs == nil {
		aggICs = env.ics
	}

	t.Logf("P4: at least one vendor-specific leaf has an ingress sum of exactly %d.", n)
	sums := pipelinecounters.IngressSum(d, ics)
	matches := pipelinecounters.LeavesWithSum(sums, n)
	if len(matches) == 0 {
		if cpFallback {
			checkControlPlaneFallback(t, env, d, sums, n, cpBase)
		} else {
			t.Errorf("P4: no vendor-specific leaf under .../drop/vendor has an ingress sum of exactly %d; sums: %v", n, sums)
		}
	}
	for _, m := range matches {
		t.Logf("P4: leaf .../drop/vendor/%s has an ingress sum of exactly %d.", m, n)
	}

	for _, m := range matches {
		for k := range d {
			if k.RelPath == m {
				class := k.Class
				if class == "" {
					class = "ungrouped"
				}
				t.Logf("P4: matching leaf %s is in class %s.", k.FullPath(), class)
			}
		}
	}

	checkNotAdverseOrCongestion(t, env, d, aggBase, aggEnd)
	logP3P7(t, d, aggBase, aggEnd, aggICs, env.ics, map[string]uint64{"": n})
	return dropResult{deltas: d, matches: matches, aggBase: aggBase, aggEnd: aggEnd}
}

// checkControlPlaneFallback implements P4 for the gNMI-1.29.8 edge link cut
// when no vendor leaf has an ingress sum of exactly n. If no vendor drop leaf
// incremented at all, the drops must be accounted for by the control plane
// dropped-aggregate (delta >= n). Partial vendor counts still fail P4.
func checkControlPlaneFallback(t *testing.T, env *testEnv, d pipelinecounters.Snapshot, sums map[string]uint64, n, cpBase uint64) {
	t.Helper()
	var totalVendorDrops uint64
	for _, v := range pipelinecounters.VendorOnly(d) {
		totalVendorDrops += v
	}
	if totalVendorDrops != 0 {
		t.Errorf("P4: no vendor-specific leaf under .../drop/vendor has an ingress sum of exactly %d (total vendor drops %d); sums: %v", n, totalVendorDrops, sums)
		return
	}
	t.Log("P4: no vendor drop leaf incremented; check the control plane dropped-aggregate.")
	cpEnd := readControlPlaneDrops(t, env.dut)
	var cpDelta uint64
	if cpEnd < cpBase {
		t.Logf("Control plane dropped-aggregate decreased from %d to %d (logged only); delta treated as 0.", cpBase, cpEnd)
	} else {
		cpDelta = cpEnd - cpBase
	}
	if cpDelta >= n {
		t.Logf("0 pipeline vendor drops recorded, but %d control plane drops recorded. Traffic matching a no-route condition was punted to the CPU and dropped by CoPP.", cpDelta)
		return
	}
	t.Errorf("Neither the vendor drop leaves nor the control plane dropped-aggregate account for %d drops: 0 pipeline vendor drops, control plane dropped-aggregate delta %d (summed over all integrated circuits)", n, cpDelta)
}

// checkNotAdverseOrCongestion implements P5 and P6. OC aggregates are
// checked on every integrated circuit that reports them.
func checkNotAdverseOrCongestion(t *testing.T, env *testEnv, d pipelinecounters.Snapshot, aggBase, aggEnd map[string]pipelinecounters.Aggregates) {
	t.Helper()
	t.Log("P5: vendor adverse leaves (and adverse-aggregate, if reported) do not increment.")
	if got := pipelinecounters.ClassDelta(d, pipelinecounters.ClassAdverse); got != 0 {
		t.Errorf("P5: vendor adverse leaves incremented by %d, want 0", got)
	}
	for ic, got := range aggDeltas(t, aggBase, aggEnd, env.ics, advAgg) {
		if got != 0 {
			t.Errorf("P5: %s adverse-aggregate incremented by %d, want 0", ic, got)
		}
	}
	t.Log("P6: vendor congestion leaves (and congestion-aggregate, if reported) do not increment.")
	if got := pipelinecounters.ClassDelta(d, pipelinecounters.ClassCongestion); got != 0 {
		t.Errorf("P6: vendor congestion leaves incremented by %d, want 0", got)
	}
	for ic, got := range aggDeltas(t, aggBase, aggEnd, env.ics, congAgg) {
		if got != 0 {
			t.Errorf("P6: %s congestion-aggregate incremented by %d, want 0", ic, got)
		}
	}
}

// validateNoDrops sends forwarded flows and verifies that they are received
// without loss and that no drop counter changes (zero-drop settling).
func validateNoDrops(t *testing.T, env *testEnv, specs []flowSpec) {
	t.Helper()
	t.Log("Take a quiet baseline (expected drops = 0).")
	base := env.stream.AwaitQuietBaseline(t, quietRetries, roundTimeout)
	aggBase := pipelinecounters.AggregatesFrom(base, env.ics)
	counts := runFlows(t, env, specs)
	t.Log("Record the settled final value (second sample after the flows stopped).")
	final := env.stream.AwaitRounds(t, 2, roundTimeout)
	aggEnd := pipelinecounters.AggregatesFrom(final, env.ics)
	t.Log("Verify every flow is received without loss and no drop counter changed.")
	for _, f := range specs {
		want := uint64(f.count)
		if want == 0 {
			want = pktCount
		}
		if tx, rx := counts[f.name][0], counts[f.name][1]; tx != want || rx != want {
			t.Errorf("Flow %s: tx=%d rx=%d, want %d received without loss", f.name, tx, rx, want)
		}
	}
	if d := pipelinecounters.Deltas(base, final); len(d) != 0 {
		t.Errorf("Drop counters incremented for forwarded traffic:")
		pipelinecounters.LogIncremented(t, d)
	}
	for _, f := range []aggField{ppAgg, advAgg, congAgg, nrAgg, nnhAgg} {
		for ic, got := range aggDeltas(t, aggBase, aggEnd, env.ics, f) {
			if got != 0 {
				t.Errorf("%s %s incremented by %d for forwarded traffic, want 0", ic, f.name, got)
			}
		}
	}
}

// recordNonIngressDrops collects gNMI-1.29.9.4 violations for a single-flow
// step from ATE port 1.
func recordNonIngressDrops(env *testEnv, step string, r dropResult) {
	if !env.icMapComplete || len(env.ics) < 2 {
		return
	}
	env.nonIngressSteps = append(env.nonIngressSteps, step)
	ingress := env.icByPort[portPairs[0].id]
	for k, v := range r.deltas {
		if k.Component != ingress && k.Class != pipelinecounters.ClassOC {
			env.nonIngressErrs = append(env.nonIngressErrs, fmt.Sprintf("%s: %s incremented by %d", step, k, v))
		}
	}
	for _, ic := range env.ics {
		if ic == ingress {
			continue
		}
		if got, err := pipelinecounters.AggregateDelta(r.aggBase, r.aggEnd, []string{ic}, ppAgg.get); err == nil && got != 0 {
			env.nonIngressErrs = append(env.nonIngressErrs, fmt.Sprintf("%s: %s packet-processing-aggregate incremented by %d", step, ic, got))
		}
	}
}

// ciscoLeavesAvailable reports whether the DUT exposes the Cisco XR drop
// leaves of gNMI-1.29.1 to gNMI-1.29.3, either through the functional
// translator or natively (any streamed leaf matches one of their names). The
// drop subscription must be open.
func ciscoLeavesAvailable(env *testEnv) bool {
	if env.streamOpts.FunctionalTranslator != "" {
		return true
	}
	for _, k := range env.stream.Leaves() {
		for _, c := range []ciscoLeaf{ciscoRouteLookupFailed, ciscoNullAdj, ciscoMPLSLabelMiss} {
			if c.matches(k) {
				return true
			}
		}
	}
	return false
}

// testCiscoLeafStreamed implements gNMI-1.29.1 and gNMI-1.29.3.
func testCiscoLeafStreamed(t *testing.T, env *testEnv, c ciscoLeaf) {
	ensureStream(t, env)
	if !ciscoLeavesAvailable(env) {
		t.Skipf("The %s leaf %v* is only exposed by Cisco XR, through the ciscoxr_vendordrop_ft functional translator or natively; no such leaf is streamed", c.desc, c.names)
	}
	t.Logf("Verify that the %s leaf %v* is present and streamed at the %v sample interval.", c.desc, c.names, sampleInterval)
	snap := env.stream.AwaitRounds(t, 3, roundTimeout)
	matched := map[string]bool{}
	for k := range snap {
		if !c.matches(k) {
			continue
		}
		matched[k.FullPath()] = true
		t.Logf("Leaf %s present (%d updates).", k.FullPath(), env.stream.UpdateCount(k))
		if env.stream.UpdateCount(k) < 2 {
			t.Errorf("Leaf %s received in fewer than 2 samples", k.FullPath())
		}
	}
	if len(matched) == 0 {
		t.Errorf("No %s leaf %v* streamed by %s", c.desc, c.names, env.dut.Name())
	}
	for _, g := range env.stream.LeavesExceedingGap(2 * sampleInterval) {
		if matched[strings.SplitN(g, " ", 2)[0]] {
			t.Errorf("Leaf not streamed at the %v sample interval: %s", sampleInterval, g)
		}
	}
}

// testLegacyNullAdj implements gNMI-1.29.2.
func testLegacyNullAdj(t *testing.T, env *testEnv) {
	ensureStream(t, env)
	if !ciscoLeavesAvailable(env) {
		t.Skipf("The %s leaf %v* is only exposed by Cisco XR, through the ciscoxr_vendordrop_ft functional translator or natively; no such leaf is streamed", ciscoNullAdj.desc, ciscoNullAdj.names)
	}
	t.Logf("Configure static route %s with next hop DROP.", legacyDiscardV4)
	installRoutes(t, env, map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{legacyDiscardV4: dropUnion()})
	defer removeRoutes(t, env, legacyDiscardV4)

	base := env.stream.AwaitQuietBaseline(t, quietRetries, roundTimeout)
	t.Logf("From ATE port 1, send traffic destined to %s.", legacyDstV4)
	runFlows(t, env, []flowSpec{{name: "v4-legacy-null", ingress: 0, egress: 1, dsts: []string{legacyDstV4}, dropped: true}})
	final, _ := env.stream.AwaitIngressSum(t, base, env.ingressICs(0), pktCount, settleTimeout)
	d := pipelinecounters.Deltas(base, final)
	pipelinecounters.LogIncremented(t, d)
	for k, v := range d {
		if ciscoNullAdj.matches(k) && v > 0 {
			t.Logf("Verified %s incremented by %d.", k, v)
			return
		}
	}
	t.Errorf("No %s leaf %v* incremented", ciscoNullAdj.desc, ciscoNullAdj.names)
}

// testDiscovery implements gNMI-1.29.4.
func testDiscovery(t *testing.T, env *testEnv) {
	t.Log("Step 1 - Discover the integrated circuits.")
	awaitICsActive(t, env)
	t.Logf("Integrated circuits: %v", env.ics)

	t.Log("Step 2 - Map ingress ports to integrated circuits (best effort).")
	env.icByPort, env.icMapComplete = integratedCircuitMap(t, env.dut, []*ondatra.Port{env.dutPort(t, 0), env.dutPort(t, 1), env.dutPort(t, 2)})
	if env.icMapComplete {
		t.Logf("Ingress map: %v", env.icByPort)
	} else {
		t.Logf("Ingress map not available (%v); per-integrated-circuit checks use the sum of all integrated circuits.", env.icByPort)
	}

	t.Log("Step 3 - Open the drop subscription.")
	ensureStream(t, env)

	t.Log("Step 4 - Validate the per-vendor leaf list.")
	leaves := env.stream.Leaves()
	vendor := env.dut.Vendor().String()
	t.Logf("dut.Vendor()=%s reports %d vendor drop leaves:", vendor, len(leaves))
	for _, k := range leaves {
		t.Logf("  %s", k.FullPath())
	}
	if len(leaves) == 0 {
		t.Errorf("No vendor drop leaf is reported under .../pipeline-counters/drop/vendor for dut.Vendor()=%s", vendor)
	}
	vendorElems := map[string]bool{}
	matchesVendor := false
	for _, k := range leaves {
		elem := strings.SplitN(k.RelPath, "/", 2)[0]
		vendorElems[elem] = true
		// The container may be module-qualified, for example
		// Cisco-IOS-XR-openconfig-platform-pipeline-counters-ext:cisco.
		if _, local, qualified := strings.Cut(elem, ":"); qualified {
			elem = local
		}
		if strings.HasPrefix(strings.ToLower(elem), strings.ToLower(vendor)) {
			matchesVendor = true
		}
	}
	t.Logf("Vendor containers below .../drop/vendor: %v", vendorElems)
	if len(leaves) > 0 && !matchesVendor {
		t.Errorf("No vendor drop leaf is reported under a .../drop/vendor/<VENDOR> container matching dut.Vendor()=%s (got %v)", vendor, vendorElems)
	}
	if nc := env.stream.NonCounterLeaves(); len(nc) != 0 {
		t.Errorf("Vendor drop leaves that are not non-negative integer counters: %v", nc)
	}
	if r := env.stream.RateLeaves(); len(r) != 0 {
		t.Logf("Vendor rate leaves, excluded from all counter checks (%d): %v", len(r), r)
	}
	for ic, a := range pipelinecounters.AggregatesFrom(env.stream.Latest(), env.ics) {
		t.Logf("OC drop aggregates on %s: packet-processing=%t adverse=%t congestion=%t (absence is a warning only).", ic, a.PacketProcessing != nil, a.Adverse != nil, a.Congestion != nil)
	}

	t.Logf("Step 5 - Verify streaming: every leaf in at least two samples at the %v interval, with no gap longer than %v.", sampleInterval, 2*sampleInterval)
	env.stream.AwaitRounds(t, 2, roundTimeout)
	if len(leaves) > 0 && env.stream.MinUpdates() < 2 {
		t.Errorf("Some vendor drop leaves were received in fewer than 2 samples")
	}
	if gaps := env.stream.LeavesExceedingGap(2 * sampleInterval); len(gaps) != 0 {
		examples := gaps
		if len(examples) > maxListedLeaves {
			examples = examples[:maxListedLeaves]
		}
		t.Errorf("%d of %d vendor drop leaves were not streamed at the %v sample interval (gap longer than %v); first %d: %v", len(gaps), len(leaves), sampleInterval, 2*sampleInterval, len(examples), examples)
	}
	if ft := env.streamOpts.FunctionalTranslator; ft != "" {
		if n := env.stream.TranslationErrors(); n != 0 {
			t.Errorf("Functional translator %s failed to translate %d notifications; vendor drop leaves may be missing", ft, n)
		}
	}

	t.Log("Step 6 - Take the first quiet baseline and start the monotonicity check.")
	env.stream.AwaitQuietBaseline(t, quietRetries, roundTimeout)
}

// testForwardedTraffic implements gNMI-1.29.5.
func testForwardedTraffic(t *testing.T, env *testEnv) {
	ensureStream(t, env)
	nh4 := oc.UnionString(portPairs[3].ate.IPv4)
	nh6 := oc.UnionString(portPairs[3].ate.IPv6)
	t.Log("Step 1/2/3 - Configure, push and await the control routes.")
	installRoutes(t, env, map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{controlV4: nh4, controlV6: nh6})
	t.Log("Step 4-7 - Send forwarded flows and verify no drop counter changes.")
	validateNoDrops(t, env, []flowSpec{
		{name: "v4-ctrl", ingress: 0, egress: 3, dsts: []string{controlDstV4}},
		{name: "v6-ctrl", ingress: 0, egress: 3, dsts: []string{controlDstV6}, ipv6: true},
	})
}

// testDiscardRoutes implements gNMI-1.29.6.
func testDiscardRoutes(t *testing.T, env *testEnv) {
	ensureStream(t, env)
	t.Log("Step 1/2 - Configure and push the IPv4 and IPv6 discard routes.")
	installRoutes(t, env, map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{discardV4: dropUnion(), discardV6: dropUnion()})

	t.Log("Step 3 - Verify that no more-specific AFT entry exists for the test destinations.")
	afts := gnmi.OC().NetworkInstance(env.ni).Afts()
	if gnmi.Lookup(t, env.dut, afts.Ipv4Entry(discardDstV4+"/32").State()).IsPresent() {
		t.Errorf("Unexpected AFT entry %s/32", discardDstV4)
	}
	if gnmi.Lookup(t, env.dut, afts.Ipv6Entry(discardDstV6+"/128").State()).IsPresent() {
		t.Errorf("Unexpected AFT entry %s/128", discardDstV6)
	}

	for _, tc := range []struct {
		step string
		flow flowSpec
	}{
		{"Step 4 - Measure IPv4 drops", flowSpec{name: "v4-drop", ingress: 0, egress: 1, dsts: []string{discardDstV4}, dropped: true}},
		{"Step 5 - Measure IPv6 drops", flowSpec{name: "v6-drop", ingress: 0, egress: 1, dsts: []string{discardDstV6}, ipv6: true, dropped: true}},
	} {
		t.Run(tc.step, func(t *testing.T) {
			r := validateDrops(t, env, []flowSpec{tc.flow})
			env.discardLeaves[tc.flow.ipv6] = r.matches
			t.Logf("Discard reason leaves (ipv6=%t): %v", tc.flow.ipv6, r.matches)
			recordNonIngressDrops(env, "gNMI-1.29.6 "+tc.flow.name, r)
		})
	}

	t.Log("Step 6 - Verify that both discard routes are still present in the AFT.")
	awaitStaticRoutesInAFT(t, env, []string{discardV4, discardV6}, true)
}

// scalePrefixes returns the 16 IPv4 and 16 IPv6 scale discard prefixes and one
// destination per prefix.
func scalePrefixes() (v4, v6, dst4, dst6 []string) {
	for i := 0; i < 16; i++ {
		v4 = append(v4, fmt.Sprintf("198.18.%d.0/24", i))
		dst4 = append(dst4, fmt.Sprintf("198.18.%d.1", i))
		v6 = append(v6, fmt.Sprintf("2001:db8:100:%x::/64", i+1))
		dst6 = append(dst6, fmt.Sprintf("2001:db8:100:%x::1", i+1))
	}
	return v4, v6, dst4, dst6
}

// testScale implements gNMI-1.29.7.
func testScale(t *testing.T, env *testEnv) {
	ensureStream(t, env)
	v4, v6, dst4, dst6 := scalePrefixes()
	t.Log("Step 1/2 - Configure the 32 scale discard routes in a single gNMI Set.")
	routes := map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{}
	for _, p := range append(append([]string{}, v4...), v6...) {
		routes[p] = dropUnion()
	}
	installRoutes(t, env, routes)
	t.Log("Step 3 - Await the scale, discard and control routes in the static route state and the AFT.")
	awaitStaticRouteState(t, env, []string{discardV4, discardV6, controlV4, controlV6})
	awaitStaticRoutesInAFT(t, env, []string{discardV4, discardV6, controlV4, controlV6}, true)

	t.Log("Step 4 - Configure 7 flows on the ATE.")
	flows := []flowSpec{
		{name: "v4-drop-A", ingress: 0, egress: 3, dsts: dst4, count: 1000, dropped: true},
		{name: "v6-drop-A", ingress: 0, egress: 3, dsts: dst6, ipv6: true, count: 1000, dropped: true},
		{name: "v4-drop-B", ingress: 1, egress: 3, dsts: dst4, count: 2000, dropped: true},
		{name: "v6-drop-B", ingress: 1, egress: 3, dsts: dst6, ipv6: true, count: 2000, dropped: true},
		{name: "v4-drop-C", ingress: 2, egress: 3, dsts: dst4, count: 3000, dropped: true},
		{name: "v6-drop-C", ingress: 2, egress: 3, dsts: dst6, ipv6: true, count: 3000, dropped: true},
		{name: "v4-ctrl", ingress: 0, egress: 3, dsts: []string{controlDstV4}, count: 1000},
	}
	perPort := map[int]uint64{0: 2000, 1: 4000, 2: 6000}
	const total = 12000

	var first pipelinecounters.Snapshot
	for iter := 1; iter <= 3; iter++ {
		t.Run(fmt.Sprintf("Step 5 - Iteration %d", iter), func(t *testing.T) {
			ics := env.ingressICs(0, 1, 2)
			t.Log("Take a quiet baseline.")
			base := env.stream.AwaitQuietBaseline(t, quietRetries, roundTimeout)
			aggBase := pipelinecounters.AggregatesFrom(base, env.ics)
			ifBase := map[int]ifBaseline{}
			for i := range perPort {
				ifBase[i] = ifBaselineOf(t, env, i)
			}
			t.Log("Start all 7 flows at the same time and await the stopped transmit state.")
			counts := runFlows(t, env, flows)

			t.Log("P0: each flow transmitted exactly its packet count.")
			for _, f := range flows {
				if got := counts[f.name][0]; got != uint64(f.count) {
					t.Fatalf("P0: traffic generator error: flow %s transmitted %d, want %d", f.name, got, f.count)
				}
			}
			final, ok := env.stream.AwaitIngressSum(t, base, ics, total, settleTimeout)
			if !ok {
				t.Errorf("No vendor drop leaf reached an ingress sum of %d within %v", total, settleTimeout)
			}
			d := pipelinecounters.Deltas(base, final)
			aggEnd := pipelinecounters.AggregatesFrom(final, env.ics)
			t.Log("Log: drop counters that incremented:")
			pipelinecounters.LogIncremented(t, d)

			t.Log("P1: in-unicast-pkts delta >= 3,000 / 4,000 / 6,000 on dut:port1/2/3, in-errors delta == 0, in-discards logged.")
			for i, extra := range map[int]uint64{0: 3000, 1: 4000, 2: 6000} {
				checkP1(t, env, i, ifBase[i], extra)
			}

			t.Log("P2: ATE port 4 received 0 packets of the drop flows and 1,000 of v4-ctrl.")
			for _, f := range flows {
				want := uint64(0)
				if !f.dropped {
					want = uint64(f.count)
				}
				got := counts[f.name][1]
				if f.dropped {
					got = flowRx(t, env, f.name)
				}
				if got != want {
					t.Errorf("P2: flow %s received %d, want %d", f.name, got, want)
				}
			}

			expected := map[string]uint64{"": total}
			if env.icMapComplete {
				expected = map[string]uint64{}
				for i, v := range perPort {
					expected[env.icByPort[portPairs[i].id]] += v
				}
			}
			t.Log("P3 (log only): packet-processing-aggregate compared with the expected values.")
			logP3P7(t, d, aggBase, aggEnd, env.ics, env.ics, expected)

			t.Log("P4: exact per-integrated-circuit (or total) vendor leaf sums.")
			if env.icMapComplete {
				for ic, want := range expected {
					if m := pipelinecounters.LeavesWithSum(pipelinecounters.IngressSum(d, []string{ic}), want); len(m) == 0 {
						t.Errorf("P4: no vendor leaf on %s has a delta of exactly %d", ic, want)
					} else {
						t.Logf("P4: %s leaves with delta %d: %v", ic, want, m)
					}
				}
			} else if m := pipelinecounters.LeavesWithSum(pipelinecounters.IngressSum(d, nil), total); len(m) == 0 {
				t.Errorf("P4: no vendor leaf summed over all integrated circuits has a delta of exactly %d", total)
			}
			checkNotAdverseOrCongestion(t, env, d, aggBase, aggEnd)

			t.Log("Verify that this iteration produces exactly the same per-integrated-circuit deltas as the first one.")
			vd := pipelinecounters.VendorOnly(d)
			if first == nil {
				first = vd
				return
			}
			if diff := append(pipelinecounters.Diff(first, vd), pipelinecounters.Diff(vd, first)...); len(diff) != 0 {
				t.Errorf("Iteration %d deltas differ from the first completed iteration: %v", iter, diff)
			}
		})
	}

	if *hcFlow {
		t.Run("Step 6 - High-count flow", func(t *testing.T) {
			validateDrops(t, env, []flowSpec{{name: "v4-drop-hc", ingress: 0, egress: 3, dsts: []string{dst4[0]}, count: 100000, pps: 10000, dropped: true}})
		})
	} else {
		t.Log("Step 6 (optional) skipped; enable with --pipeline_hc_flow.")
	}
}

// awaitEdgeUnreachable waits for the edge routes to be withdrawn from the AFT
// or, on platforms that keep the entries, for the ATE port 4 next hops to be
// removed. The AFT is waited for with awaitResubscribe.
func awaitEdgeUnreachable(t *testing.T, env *testEnv) {
	t.Helper()
	ateV4, ateV6 := portPairs[3].ate.IPv4, portPairs[3].ate.IPv6
	ok := awaitResubscribe(t, env.dut, gnmi.OC().NetworkInstance(env.ni).Afts().State(), time.Now().Add(awaitTimeout), func(v *ygnmi.Value[*oc.NetworkInstance_Afts]) bool {
		a, present := v.Val()
		if !present {
			return false
		}
		if a.GetIpv4Entry(edgeV4) == nil && a.GetIpv6Entry(edgeV6) == nil {
			return true
		}
		for _, nh := range a.NextHop {
			if ip := nh.GetIpAddress(); ip == ateV4 || ip == ateV6 {
				return false
			}
		}
		return true
	})
	if !ok {
		t.Fatalf("Edge routes %s/%s and next hops %s/%s still present in the AFT %v after the edge link cut", edgeV4, edgeV6, ateV4, ateV6, awaitTimeout)
	}
}

// testEdgeCut implements gNMI-1.29.8.
func testEdgeCut(t *testing.T, env *testEnv) {
	ensureStream(t, env)
	edgeFlows := []flowSpec{
		{name: "v4-edge", ingress: 0, egress: 3, dsts: []string{edgeDstV4}},
		{name: "v6-edge", ingress: 0, egress: 3, dsts: []string{edgeDstV6}, ipv6: true},
	}
	t.Log("Step 1/2 - Configure and await the edge routes via ATE port 4.")
	installRoutes(t, env, map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
		edgeV4: oc.UnionString(portPairs[3].ate.IPv4),
		edgeV6: oc.UnionString(portPairs[3].ate.IPv6),
	})

	t.Log("Step 3 - Pre-cut verification.")
	validateNoDrops(t, env, edgeFlows)

	t.Log("Step 4 - Cut the edge link (dut:port4 enabled=false).")
	p4 := env.dutPort(t, 3).Name()
	gnmi.Replace(t, env.dut, gnmi.OC().Interface(p4).Enabled().Config(), false)
	restored := false
	t.Cleanup(func() {
		if !restored {
			// Restore the edge link fully, so that later subtests that send
			// traffic towards ATE port 4 (for example gNMI-1.29.9.5) start
			// from a working link.
			gnmi.Replace(t, env.dut, gnmi.OC().Interface(p4).Enabled().Config(), true)
			gnmi.Await(t, env.dut, gnmi.OC().Interface(p4).OperStatus().State(), awaitTimeout, oc.Interface_OperStatus_UP)
			awaitNeighbors(t, env, 3)
		}
	})
	gnmi.Await(t, env.dut, gnmi.OC().Interface(p4).OperStatus().State(), awaitTimeout, oc.Interface_OperStatus_DOWN)
	awaitEdgeUnreachable(t, env)

	t.Log("Step 5 - Measure drops during the cut (IPv4, then IPv6).")
	for _, f := range edgeFlows {
		f := f
		f.dropped = true
		t.Run("cut "+f.name, func(t *testing.T) {
			if env.defaultRoute != "" {
				t.Errorf("Cannot validate no-route drops: a default route is present (%s); no traffic is sent", env.defaultRoute)
				return
			}
			r := validateEdgeCutDrops(t, env, []flowSpec{f})
			recordNonIngressDrops(env, "gNMI-1.29.8 "+f.name, r)
			t.Log("Lookup block counters (log only): no-route and no-nexthop deltas on the ingress integrated circuit.")
			nr := aggDeltas(t, r.aggBase, r.aggEnd, env.ingressICsOrAll(0), nrAgg)
			nn := aggDeltas(t, r.aggBase, r.aggEnd, env.ingressICsOrAll(0), nnhAgg)
			if len(nr) == 0 && len(nn) == 0 {
				t.Log("Lookup block counters not reported.")
			} else {
				t.Logf("Lookup block (log only): no-route delta %d, no-nexthop delta %d (expected drops %d).", sumValues(nr), sumValues(nn), pktCount)
			}
		})
	}

	t.Log("Step 6 - Restore the edge link.")
	gnmi.Replace(t, env.dut, gnmi.OC().Interface(p4).Enabled().Config(), true)
	gnmi.Await(t, env.dut, gnmi.OC().Interface(p4).OperStatus().State(), awaitTimeout, oc.Interface_OperStatus_UP)
	awaitNeighbors(t, env, 3)
	restored = true
	awaitStaticRoutesInAFT(t, env, []string{edgeV4, edgeV6}, true)

	t.Log("Step 7 - Post-restore verification.")
	validateNoDrops(t, env, edgeFlows)
}

// ingressICsOrAll returns ingressICs or, when the map is incomplete, all ICs.
func (e *testEnv) ingressICsOrAll(ports ...int) []string {
	if ics := e.ingressICs(ports...); ics != nil {
		return ics
	}
	return e.ics
}

// testNegative implements gNMI-1.29.9.
func testNegative(t *testing.T, env *testEnv) {
	ensureStream(t, env)
	t.Log("Built-in negative checks: gNMI-1.29.5 / v4-ctrl in gNMI-1.29.7 (no drops for forwarded traffic) and P5 in gNMI-1.29.6 to gNMI-1.29.8 (no adverse drops).")

	t.Run("gNMI-1.29.9.1 Counters Do Not Reset", func(t *testing.T) { testCountersDoNotReset(t, env) })
	for _, tc := range []struct {
		name   string
		prefix string
		dst    string
		ipv6   bool
	}{
		{"gNMI-1.29.9.2 IPv4 Drop Reason Changes With The Route", discardV4, discardDstV4, false},
		{"gNMI-1.29.9.3 IPv6 Drop Reason Changes With The Route", discardV6, discardDstV6, true},
	} {
		t.Run(tc.name, func(t *testing.T) { testDropReasonChange(t, env, tc.prefix, tc.dst, tc.ipv6) })
	}
	t.Run("gNMI-1.29.9.4 No Drops On Integrated Circuits Without Ingress Traffic", func(t *testing.T) {
		if !env.icMapComplete || len(env.ics) < 2 {
			t.Skipf("Ingress map available=%t, integrated circuits=%d; subtest requires both", env.icMapComplete, len(env.ics))
		}
		if len(env.nonIngressSteps) == 0 {
			t.Fatal("No single-flow step of gNMI-1.29.6 or gNMI-1.29.8 recorded drop counter results; nothing to verify")
		}
		t.Logf("Verified steps (%d): %v", len(env.nonIngressSteps), env.nonIngressSteps)
		for _, e := range env.nonIngressErrs {
			t.Error(e)
		}
	})
	t.Run("gNMI-1.29.9.5 TTL Expiry Drops", func(t *testing.T) { testTTLExpiry(t, env) })
	t.Run("gNMI-1.29.9.6 Subscription To A Non-Existent Vendor Container", func(t *testing.T) { testNonExistentVendor(t, env) })
}

// testCountersDoNotReset implements gNMI-1.29.9.1.
func testCountersDoNotReset(t *testing.T, env *testEnv) {
	// onceFailed records whether Subscribe ONCE returned an error, so that it
	// is reported once and the remaining reads and checks still run.
	onceFailed := false
	// read reads the vendor drop leaves twice with Subscribe ONCE and once
	// from the SAMPLE subscription within one quiet period. ok is false if a
	// background drop changed a counter during the reads, detected with the
	// next SAMPLE round; the reads are then discarded.
	read := func(t *testing.T) (snap pipelinecounters.Snapshot, ok bool) {
		quiet := pipelinecounters.VendorOnly(env.stream.AwaitQuietBaseline(t, quietRetries, roundTimeout))
		t.Log("Read the vendor drop leaves twice with Subscribe ONCE and once from the SAMPLE subscription.")
		var mismatch []string
		for i := 1; i <= 2; i++ {
			if onceFailed {
				t.Logf("Subscribe ONCE read %d/2 is skipped; an earlier read failed.", i)
				break
			}
			got, err := pipelinecounters.ReadOnce(t, env.dut, env.streamOpts)
			if err != nil {
				t.Errorf("gNMI Subscribe ONCE read %d/2 of the vendor drop leaves failed: %v", i, err)
				onceFailed = true
				break
			}
			if diff := symmetricDiff(quiet, got); len(diff) != 0 {
				mismatch = append(mismatch, fmt.Sprintf("Subscribe ONCE %d/2: %v", i, diff))
			}
		}
		t.Log("Await the next SAMPLE round to detect background drops during the reads.")
		if diff := symmetricDiff(quiet, pipelinecounters.VendorOnly(env.stream.AwaitRounds(t, 1, roundTimeout))); len(diff) != 0 {
			t.Logf("Background drops changed counters during the reads; the reads are discarded: %v", diff)
			return nil, false
		}
		if len(mismatch) != 0 {
			t.Errorf("Subscribe ONCE values differ from the SAMPLE subscription while the counters were unchanged: %v", mismatch)
		} else {
			t.Log("All successful reads returned identical values.")
		}
		return quiet, true
	}
	for attempt := 1; attempt <= quietRetries; attempt++ {
		t.Logf("Attempt %d/%d: read, change an unrelated route, read again.", attempt, quietRetries)
		before, ok := read(t)
		if !ok {
			continue
		}
		t.Logf("Configure and remove static route %s -> DROP.", stabilityV4)
		installRoutes(t, env, map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{stabilityV4: dropUnion()})
		removeRoutes(t, env, stabilityV4)
		after, ok := read(t)
		if !ok {
			continue
		}
		if diff := symmetricDiff(before, after); len(diff) != 0 {
			if attempt < quietRetries {
				t.Logf("Counters changed between the reads before and after the configuration change (a background drop between the reads also does this); retrying: %v", diff)
				continue
			}
			t.Errorf("Vendor drop counters changed after an unrelated configuration change in all %d attempts: %v", quietRetries, diff)
		}
		t.Log("Verify that the monotonicity check recorded no decrease.")
		if v := env.stream.MonotonicViolations(); len(v) != 0 {
			t.Errorf("Monotonicity violations during the test: %v", v)
		}
		return
	}
	t.Fatalf("Could not read the counters within a quiet period in %d attempts", quietRetries)
}

// symmetricDiff describes every leaf whose value differs between want and
// got, is missing from got, or appears only in got.
func symmetricDiff(want, got pipelinecounters.Snapshot) []string {
	diff := pipelinecounters.Diff(want, got)
	for k := range got {
		if _, ok := want[k]; !ok {
			diff = append(diff, fmt.Sprintf("%s new", k))
		}
	}
	sort.Strings(diff)
	return diff
}

// testDropReasonChange implements gNMI-1.29.9.2 and gNMI-1.29.9.3.
func testDropReasonChange(t *testing.T, env *testEnv, prefix, dst string, ipv6 bool) {
	if env.defaultRoute != "" {
		t.Errorf("Cannot validate no-route drops: a default route is present (%s); the discard route is kept and no traffic is sent", env.defaultRoute)
		return
	}
	discard := env.discardLeaves[ipv6]
	t.Logf("Discard reason leaves recorded in gNMI-1.29.6: %v", discard)
	if len(discard) == 0 {
		t.Error("No discard reason leaf was recorded in gNMI-1.29.6 (its P4 failed or it did not run), so the drop reason change cannot be verified; the no-route drops are still validated")
	}
	removeRoutes(t, env, prefix)
	name := "v4-drop"
	if ipv6 {
		name = "v6-drop"
	}
	r := validateDrops(t, env, []flowSpec{{name: name, ingress: 0, egress: 1, dsts: []string{dst}, ipv6: ipv6, dropped: true}})
	isDiscard := map[string]bool{}
	for _, l := range discard {
		isDiscard[l] = true
	}
	isMatch := map[string]bool{}
	var different, shared []string
	for _, m := range r.matches {
		isMatch[m] = true
		if isDiscard[m] {
			shared = append(shared, m)
		} else {
			different = append(different, m)
		}
	}
	if len(shared) > 0 {
		t.Logf("INFO: leaves that count both discard and no-route drops: %v", shared)
	}
	if len(different) > 0 {
		t.Logf("The drop is also counted by leaves that did not count the discard drops: %v; verify every discard-only leaf has a delta of 0 on every integrated circuit.", different)
		sums := pipelinecounters.IngressSum(r.deltas, nil)
		for _, l := range discard {
			if !isMatch[l] && sums[l] != 0 {
				t.Errorf("Discard reason leaf %s incremented by %d after the route was removed, want 0", l, sums[l])
			}
		}
	}
	if nr := aggDeltas(t, r.aggBase, r.aggEnd, env.ingressICsOrAll(0), nrAgg); len(nr) > 0 {
		t.Logf("Lookup block (log only): no-route delta = %d (expected drops %d).", sumValues(nr), pktCount)
	}
}

// testTTLExpiry implements gNMI-1.29.9.5.
func testTTLExpiry(t *testing.T, env *testEnv) {
	punted := deviations.PipelineDropTTLExpiredPunted(env.dut)
	for _, f := range []flowSpec{
		{name: "v4-ttl", ingress: 0, egress: 3, dsts: []string{controlDstV4}, ttl: 1, dropped: true},
		{name: "v6-ttl", ingress: 0, egress: 3, dsts: []string{controlDstV6}, ipv6: true, ttl: 1, dropped: true},
	} {
		t.Run(f.name, func(t *testing.T) {
			base := env.stream.AwaitQuietBaseline(t, quietRetries, roundTimeout)
			aggBase := pipelinecounters.AggregatesFrom(base, env.ics)
			var cpBase uint64
			if punted {
				t.Log("Deviation pipeline_drop_ttl_expired_punted: record the control plane queued-aggregate + dropped-aggregate baseline.")
				cpBase = readControlPlanePunted(t, env.dut)
			}
			counts := runFlows(t, env, []flowSpec{f})
			if counts[f.name][0] != pktCount {
				t.Fatalf("P0: traffic generator error: flow %s transmitted %d, want %d", f.name, counts[f.name][0], pktCount)
			}
			var final pipelinecounters.Snapshot
			if punted {
				t.Logf("Deviation pipeline_drop_ttl_expired_punted: TTL expired packets are punted to the CPU; verify the control plane queued-aggregate + dropped-aggregate delta is at least %d.", pktCount)
				final = env.stream.AwaitRounds(t, 2, roundTimeout)
				cpEnd := readControlPlanePunted(t, env.dut)
				if cpEnd < cpBase || cpEnd-cpBase < pktCount {
					t.Errorf("Control plane queued-aggregate + dropped-aggregate went from %d to %d, want a delta of at least %d", cpBase, cpEnd, pktCount)
				}
			} else {
				t.Logf("Verify that at least one vendor drop leaf has a delta of at least %d.", pktCount)
				var ok bool
				if final, ok = env.stream.AwaitIngressSum(t, base, env.ingressICs(0), pktCount, settleTimeout); !ok {
					t.Errorf("No vendor drop leaf reached a delta of at least %d", pktCount)
				}
			}
			t.Logf("Verify that ATE port 4 receives 0 packets of %s.", f.name)
			if rx := flowRx(t, env, f.name); rx != 0 {
				t.Errorf("ATE port 4 received %d packets of %s, want 0", rx, f.name)
			}
			d := pipelinecounters.Deltas(base, final)
			aggEnd := pipelinecounters.AggregatesFrom(final, env.ics)
			pipelinecounters.LogIncremented(t, d)
			for ic, got := range aggDeltas(t, aggBase, aggEnd, env.ics, ppAgg) {
				t.Logf("%s packet-processing-aggregate delta = %d (logged only).", ic, got)
			}
			t.Log("Verify P5: the adverse counters do not increment.")
			if got := pipelinecounters.ClassDelta(d, pipelinecounters.ClassAdverse); got != 0 {
				t.Errorf("P5: vendor adverse leaves incremented by %d, want 0", got)
			}
			for ic, got := range aggDeltas(t, aggBase, aggEnd, env.ics, advAgg) {
				if got != 0 {
					t.Errorf("P5: %s adverse-aggregate incremented by %d, want 0", ic, got)
				}
			}
		})
	}
}

// testNonExistentVendor implements gNMI-1.29.9.6.
func testNonExistentVendor(t *testing.T, env *testEnv) {
	p := pipelinecounters.VendorDropPath()
	p.Elem = append(p.Elem, &gpb.PathElem{Name: "NonExistentVendor"})
	t.Log("Send a gNMI Subscribe ONCE for .../drop/vendor/NonExistentVendor.")
	ctx, cancel := context.WithTimeout(context.Background(), awaitTimeout)
	defer cancel()
	sub, err := env.dut.RawAPIs().GNMI(t).Subscribe(ctx)
	if err != nil {
		t.Fatalf("Failed to open gNMI Subscribe: %v", err)
	}
	if err := sub.Send(&gpb.SubscribeRequest{Request: &gpb.SubscribeRequest_Subscribe{Subscribe: &gpb.SubscriptionList{
		Prefix:       &gpb.Path{Target: env.dut.Name()},
		Mode:         gpb.SubscriptionList_ONCE,
		Encoding:     gpb.Encoding_PROTO,
		Subscription: []*gpb.Subscription{{Path: p}},
	}}}); err != nil && !errors.Is(err, io.EOF) {
		// io.EOF means the server already closed the stream; its status is
		// returned by Recv below.
		t.Fatalf("Failed to send SubscribeRequest: %v", err)
	}
	for {
		resp, err := sub.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				t.Log("Subscribe ONCE closed with EOF.")
			} else if c := status.Code(err); c == codes.NotFound || c == codes.InvalidArgument {
				t.Logf("Subscribe ONCE returned %v: %v", c, err)
			} else {
				t.Errorf("Subscribe ONCE to a non-existent vendor container returned %v, want sync_response or EOF with no updates, NOT_FOUND or INVALID_ARGUMENT", err)
			}
			break
		}
		if resp.GetSyncResponse() {
			t.Log("Subscribe ONCE completed with a sync_response.")
			break
		}
		if len(resp.GetUpdate().GetUpdate()) > 0 {
			t.Errorf("Unexpected updates for a non-existent vendor container: %v", resp.GetUpdate())
		}
	}
	t.Log("Verify every integrated circuit remains ACTIVE and the drop subscription is still streaming.")
	awaitICsActive(t, env)
	if err := env.stream.Err(); err != nil {
		t.Errorf("Drop subscription failed: %v", err)
	}
	env.stream.AwaitRounds(t, 1, roundTimeout)
}

// registerCleanup registers the README "Cleanup" steps. Each step is a
// separate t.Cleanup function, registered in reverse order because t.Cleanup
// is last-in first-out, so the steps run in README order. The testing package
// runs every remaining cleanup function even if one of them fails fatally, so
// a failing step (for example a failed route deletion) does not skip the
// remaining steps. Steps also run when setup failed part-way.
func registerCleanup(t *testing.T, env *testEnv) {
	steps := []func(){
		func() {
			t.Log("Cleanup: stop and remove all ATE flows and close the drop subscription.")
			// StopTraffic fails on OTG when no flow was ever configured (for
			// example when setup failed before the first traffic step), so
			// only stop and remove flows if there are any.
			if env.top != nil && len(env.top.Flows().Items()) > 0 {
				env.ate.OTG().StopTraffic(t)
				env.top.Flows().Clear()
				env.ate.OTG().PushConfig(t, env.top)
			}
		},
		func() {
			if env.stream == nil {
				return
			}
			if v := env.stream.MonotonicViolations(); len(v) != 0 {
				t.Errorf("Monotonicity: drop counters decreased during the test: %v", v)
			}
			env.stream.Close()
		},
		func() {
			t.Log("Cleanup: make sure all 4 DUT ports are enabled.")
			for i := range portPairs {
				dp := env.dutPort(t, i)
				if enabled, ok := gnmi.Lookup(t, env.dut, gnmi.OC().Interface(dp.Name()).Enabled().Config()).Val(); ok && !enabled {
					gnmi.Replace(t, env.dut, gnmi.OC().Interface(dp.Name()).Enabled().Config(), true)
				}
			}
		},
		func() {
			if len(env.routes) == 0 {
				return
			}
			t.Log("Cleanup: delete all static routes configured by the test and await their removal from the AFT.")
			var prefixes []string
			for p := range env.routes {
				prefixes = append(prefixes, p)
			}
			sort.Strings(prefixes)
			removeRoutes(t, env, prefixes...)
		},
		func() {
			t.Log("Cleanup: verify all 4 DUT ports are UP.")
			deadline := time.Now().Add(awaitTimeout)
			for i := range portPairs {
				dp := env.dutPort(t, i)
				if _, ok := gnmi.Watch(t, env.dut, gnmi.OC().Interface(dp.Name()).OperStatus().State(), remaining(deadline), func(v *ygnmi.Value[oc.E_Interface_OperStatus]) bool {
					s, present := v.Val()
					return present && s == oc.Interface_OperStatus_UP
				}).Await(t); !ok {
					t.Errorf("Cleanup: %s (%s) is not UP after %v", dp.Name(), portPairs[i].id, awaitTimeout)
				}
			}
		},
		func() {
			if len(env.ics) == 0 {
				t.Log("Cleanup: integrated circuits were not discovered; ACTIVE check skipped.")
				return
			}
			t.Log("Cleanup: verify every integrated circuit is ACTIVE.")
			awaitICsActive(t, env)
		},
	}
	for i := len(steps) - 1; i >= 0; i-- {
		t.Cleanup(steps[i])
	}
}
