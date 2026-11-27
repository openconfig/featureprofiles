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

// Package vrf_selection_multi_linecard_test implements RT-3.6: VRF Selection Policy Hardware
// Programming Across Multiple Linecards.
package vrf_selection_multi_linecard_test

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/gnmi/otg"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	policyName = "CLUSTER_FACING_POLICY"
	vrfName    = "VRF-1"

	ruleSeqIPv4 = 10
	ruleSeqIPv6 = 20

	matchIPv4Prefix   = "198.51.100.0/24"
	matchIPv6Prefix   = "2001:db8:100::/48"
	nomatchIPv4Prefix = "203.0.113.0/24"
	nomatchIPv6Prefix = "2001:db8:200::/48"
	matchIPv4Dst      = "198.51.100.1"
	matchIPv6Dst      = "2001:db8:100::1"
	nomatchIPv4Dst    = "203.0.113.1"
	nomatchIPv6Dst    = "2001:db8:200::1"

	// RT-3.6.1: 4 flows of 10,000 packets of 512 bytes at 1,000 pps per ingress port.
	packetsPerFlow = 10000
	flowRatePps    = 1000
	frameSize      = 512
	// RT-3.6.1: the ATE egress port totals may exceed the flow totals by up to 1% (10 per mille)
	// because the port counters also count ARP/ND/LLDP packets, and may fall short by up to 0.1%
	// (1 per mille) because the ATE port counters can undercount by a few frames. Loss is checked
	// exactly per flow (Rx == Tx); the upper bound is what detects misdelivery to the wrong port.
	egressOverPermille  = 10
	egressUnderPermille = 1

	// Traffic window names. Each subtest sends its own disjoint flow set, so no OTG config is
	// pushed after a reboot and every window starts from fresh counters.
	windowRT361 = "RT361"
	windowRT362 = "RT362"
	windowRT363 = "RT363"

	// The timeouts below are sized so that the worst case of every wait, plus setup, traffic and
	// cleanup, fits within a 75m test run timeout; each wait still ends as soon as its condition
	// is met.
	convergenceTimeout     = 2 * time.Minute
	trafficTimeout         = time.Minute
	rxSettleTimeout        = time.Minute
	baselineCounterTimeout = 15 * time.Second

	// retryInterval bounds each components.WatchWithRetry attempt, so a DUT that is unreachable
	// after a reboot is retried until the timeout.
	retryInterval = 30 * time.Second

	// RT-3.6.2 timeouts.
	chassisRebootTimeout  = 20 * time.Minute
	linecardActiveTimeout = 10 * time.Minute
	portUpTimeout         = 10 * time.Minute

	// RT-3.6.3 timeouts.
	lcLeaveActiveTimeout = 5 * time.Minute
	lcRebootTimeout      = 10 * time.Minute
	lcPortUpTimeout      = 3 * time.Minute
)

// Test environment setup: DUT port1..port6 are ingress ports in DEFAULT; port7 is the DEFAULT
// egress and port8 the VRF-1 egress.
var (
	ingressPorts  = []string{"port1", "port2", "port3", "port4", "port5", "port6"}
	defaultEgress = "port7"
	vrfEgress     = "port8"
	allPorts      = append(append([]string{}, ingressPorts...), defaultEgress, vrfEgress)
)

// testEnv holds the state shared by the RT-3.6 subtests.
type testEnv struct {
	dut       *ondatra.DUTDevice
	ate       *ondatra.ATEDevice
	otgCfg    gosnappi.Config
	defaultNI string
	// portLC maps each of port1..port8 to the LINECARD component that hosts it.
	portLC map[string]string
	// lastEgressInFrames records the last observed ATE InFrames count per egress port across
	// traffic windows so that a transient zero/missing lookup after StartProtocols is not
	// mistaken for the pre-traffic baseline.
	lastEgressInFrames map[string]uint64
}

// portAttrs returns the DUT and ATE attributes of port N (1..8) as defined in the README test
// environment setup: DUT 192.0.2.(4N-3)/30 and 2001:db8:0:N::1/126, ATE 192.0.2.(4N-2) and
// 2001:db8:0:N::2.
func portAttrs(n int) (dut, ate attrs.Attributes) {
	dut = attrs.Attributes{
		Desc:    fmt.Sprintf("DUT port%d", n),
		IPv4:    fmt.Sprintf("192.0.2.%d", 4*n-3),
		IPv4Len: 30,
		IPv6:    fmt.Sprintf("2001:db8:0:%x::1", n),
		IPv6Len: 126,
	}
	ate = attrs.Attributes{
		Name:    fmt.Sprintf("atePort%d", n),
		MAC:     fmt.Sprintf("02:00:%02x:01:01:01", n),
		IPv4:    fmt.Sprintf("192.0.2.%d", 4*n-2),
		IPv4Len: 30,
		IPv6:    fmt.Sprintf("2001:db8:0:%x::2", n),
		IPv6Len: 126,
	}
	return dut, ate
}

// portNum returns N for "portN". It panics on any other name, which is a programming error.
func portNum(name string) int {
	n, err := strconv.Atoi(strings.TrimPrefix(name, "port"))
	if err != nil {
		panic(fmt.Sprintf("portNum(%q): not a portN name", name))
	}
	return n
}

// ipv6RuleSupported reports whether rule 20 (IPv6) can share CLUSTER_FACING_POLICY with rule 10
// (IPv4). Devices with the MixedAFPolicyForwardingUnsupported deviation reject IPv4 and IPv6
// rules in one VRF_SELECTION_POLICY; on them the policy has rule 10 only, and the matched IPv6
// flows are expected to follow the DEFAULT route to port7 (see the README).
func ipv6RuleSupported(dut *ondatra.DUTDevice) bool {
	return !deviations.MixedAFPolicyForwardingUnsupported(dut)
}

// TestVRFSelectionMultiLinecard verifies that a VRF selection policy bound to ingress ports on
// at least two linecards is programmed on every linecard (RT-3.6.1), is still programmed after a
// chassis cold reboot without any configuration being re-pushed (RT-3.6.2), and is re-programmed
// on a single linecard that reboots while the rest of the chassis stays up (RT-3.6.3).
func TestVRFSelectionMultiLinecard(t *testing.T) {
	env := setupEnvironment(t)
	dut, ate := env.dut, env.ate
	// The reboot subtests are skipped when a prerequisite failed fatally, so that a broken baseline
	// or a DUT that did not recover does not cost a chassis or linecard reboot: RT-3.6.2 needs the
	// RT-3.6.1 bindings to have converged, and RT-3.6.3 needs the chassis to have recovered in
	// RT-3.6.2. Traffic failures do not block later subtests.
	var baselineBound, chassisRecovered bool

	t.Run("RT-3.6.1 - Policy programmed on every ingress linecard", func(t *testing.T) {
		t.Log("RT-3.6.1: wait for apply-vrf-selection-policy == CLUSTER_FACING_POLICY in state on all ingress ports")
		awaitIngressBindings(t, env, "RT-3.6.1", convergenceTimeout)
		baselineBound = true
		t.Log("RT-3.6.1: wait for ARP/ND on all ATE ports")
		otgutils.WaitForARP(t, ate.OTG(), env.otgCfg, "IPv4")
		otgutils.WaitForARP(t, ate.OTG(), env.otgCfg, "IPv6")
		t.Log("RT-3.6.1: send the 24 flows concurrently and verify every flow reaches its expected egress port")
		verifyTraffic(t, env, windowRT361)
	})

	t.Run("RT-3.6.2 - Policy programmed after chassis reboot", func(t *testing.T) {
		if !baselineBound {
			t.Skip("RT-3.6.1 bindings did not converge; skipping the chassis reboot")
		}
		t.Log("RT-3.6.2: issue a gNOI System.Reboot COLD for the chassis and wait for boot-time to advance")
		components.ColdRebootChassis(t, dut, "RT-3.6.2 chassis reboot", chassisRebootTimeout)

		t.Log("RT-3.6.2: wait for every linecard hosting port1..port8 (ingress and egress) to be ACTIVE")
		awaitLinecardsActive(t, dut, env.linecardsHosting(allPorts), linecardActiveTimeout)
		t.Log("RT-3.6.2: wait for all ports to be UP")
		awaitPortsUp(t, dut, allPorts, portUpTimeout)
		chassisRecovered = true

		// RT-3.6.2: the configuration set with gNMI must persist across the reboot without any
		// explicit save operation. A missing binding or rule is a configuration persistence
		// failure, so it is reported before, and instead of, a traffic failure.
		t.Log("RT-3.6.2: verify the bindings persisted in state")
		awaitIngressBindings(t, env, "RT-3.6.2", convergenceTimeout)
		t.Log("RT-3.6.2: verify the policy body (rules 10 and 20) persisted in state")
		awaitPolicyRules(t, env, "RT-3.6.2", convergenceTimeout)

		// RT-3.6.2: push no DUT config and no ATE config. Re-pushing ATE config bounces the links,
		// which can re-program the policy and hide a linecard that missed it during boot.
		t.Log("RT-3.6.2: restart ATE protocols without pushing any config and wait for ARP/ND")
		ate.OTG().StartProtocols(t)
		otgutils.WaitForARP(t, ate.OTG(), env.otgCfg, "IPv4")
		otgutils.WaitForARP(t, ate.OTG(), env.otgCfg, "IPv6")
		t.Log("RT-3.6.2: repeat the traffic checks of RT-3.6.1")
		verifyTraffic(t, env, windowRT362)
	})

	t.Run("RT-3.6.3 - Policy re-programmed after a single linecard reboot", func(t *testing.T) {
		if !chassisRecovered {
			t.Skip("The DUT did not recover from the RT-3.6.2 chassis reboot; skipping the linecard reboot")
		}
		// RT-3.6.3: select one ingress linecard, preferring one that hosts neither port7 nor
		// port8 so that the egress ports stay up.
		lc := pickRebootLinecard(t, env)
		lcPorts := env.portsOn(lc)
		t.Logf("RT-3.6.3: rebooting linecard %s, which hosts %v", lc, lcPorts)

		t.Log("RT-3.6.3: record last-reboot-time and issue gNOI System.Reboot COLD with subcomponents = [LC]")
		lastReboot, _ := gnmi.Lookup(t, dut, gnmi.OC().Component(lc).LastRebootTime().State()).Val()
		components.RebootSubcomponents(t, dut, "RT-3.6.3 linecard reboot", lc)

		t.Log("RT-3.6.3: wait for LC to leave ACTIVE, return to ACTIVE and report a newer last-reboot-time")
		components.AwaitLinecardRebootRecovery(t, dut, lc, lastReboot, lcLeaveActiveTimeout, lcRebootTimeout)
		t.Log("RT-3.6.3: wait for every port hosted on LC to be UP")
		awaitPortsUp(t, dut, lcPorts, lcPortUpTimeout)

		t.Log("RT-3.6.3: verify the bindings and rules 10 and 20 are present in state")
		awaitIngressBindings(t, env, "RT-3.6.3", convergenceTimeout)
		awaitPolicyRules(t, env, "RT-3.6.3", convergenceTimeout)

		t.Log("RT-3.6.3: restart ATE protocols without pushing any config and wait for ARP/ND")
		ate.OTG().StartProtocols(t)
		otgutils.WaitForARP(t, ate.OTG(), env.otgCfg, "IPv4")
		otgutils.WaitForARP(t, ate.OTG(), env.otgCfg, "IPv6")
		t.Log("RT-3.6.3: repeat the traffic checks of RT-3.6.1")
		verifyTraffic(t, env, windowRT363)
	})
}

// setupEnvironment implements the README "Test environment setup": it configures the DUT and
// the ATE, maps port1..port8 to their linecards and skips the test unless the ingress ports span
// at least 2 linecards. The README "Cleanup" is registered with t.Cleanup before any
// configuration is pushed, so it runs whether the test passes or fails.
//
// setupEnvironment is called at the start of the test function rather than from TestMain:
// fptest.RunTests owns the testbed reservation and TestMain has no *testing.T, so devices can only
// be reserved and configured from within a test.
func setupEnvironment(t *testing.T) *testEnv {
	t.Helper()
	env := &testEnv{
		dut:                ondatra.DUT(t, "dut"),
		ate:                ondatra.ATE(t, "ate"),
		lastEgressInFrames: make(map[string]uint64),
	}
	env.defaultNI = deviations.DefaultNetworkInstance(env.dut)

	// Cleanup: snapshot the config the test modifies so that it can be restored exactly, rather
	// than deleting config that existed before the test.
	snap := snapshotDUT(t, env.dut, env.defaultNI)
	t.Cleanup(func() { cleanupDUT(t, env.dut, env.defaultNI, snap) })

	configureDUT(t, env.dut, env.defaultNI)
	// Some platforms report hardware-port only for configured interfaces, so the ports are mapped
	// to linecards after configureDUT.
	env.portLC = mapLinecards(t, env.dut)

	env.otgCfg = configureATE(t, env.ate, env.dut)
	// Cleanup: stop ATE traffic. Registered after the DUT cleanup, so it runs first.
	t.Cleanup(func() { env.ate.OTG().StopTraffic(t) })
	return env
}

// mapLinecards implements the README linecard resolution step: it maps each of port1..port8 to
// its parent LINECARD (hardware-port, then /components/component/state/parent up to a LINECARD)
// and skips the test unless the ingress ports span at least 2 linecards.
func mapLinecards(t *testing.T, dut *ondatra.DUTDevice) map[string]string {
	t.Helper()
	ports := make([]*ondatra.Port, 0, len(allPorts))
	for _, p := range allPorts {
		ports = append(ports, dut.Port(t, p))
	}
	byName, err := components.LinecardsFromPorts(t, dut, ports)
	switch {
	case errors.Is(err, components.ErrNoLinecardComponents):
		t.Skip("DUT has no LINECARD components; RT-3.6 requires a modular DUT")
	case err != nil:
		t.Fatalf("Could not map port1..port8 to linecards: %v", err)
	}
	portLC := make(map[string]string, len(ports))
	for _, p := range ports {
		portLC[p.ID()] = byName[p.Name()]
		t.Logf("%s (%s) is on linecard %s", p.ID(), p.Name(), portLC[p.ID()])
	}
	ingressLCs := make(map[string]bool)
	for _, p := range ingressPorts {
		ingressLCs[portLC[p]] = true
	}
	if len(ingressLCs) < 2 {
		t.Skipf("Ingress ports %v span %d linecard(s); RT-3.6 needs at least 2", ingressPorts, len(ingressLCs))
	}
	return portLC
}

// linecardsHosting returns, without duplicates and in port order, the linecards that host ports.
func (e *testEnv) linecardsHosting(ports []string) []string {
	var lcs []string
	seen := make(map[string]bool)
	for _, p := range ports {
		if lc := e.portLC[p]; !seen[lc] {
			seen[lc] = true
			lcs = append(lcs, lc)
		}
	}
	return lcs
}

// portsOn returns the ports among port1..port8 that are hosted on linecard lc.
func (e *testEnv) portsOn(lc string) []string {
	var ports []string
	for _, p := range allPorts {
		if e.portLC[p] == lc {
			ports = append(ports, p)
		}
	}
	return ports
}

// pickRebootLinecard implements the RT-3.6.3 linecard selection: the first ingress linecard
// that hosts neither port7 nor port8 or, if every ingress linecard hosts an egress port, the
// first ingress linecard.
func pickRebootLinecard(t *testing.T, env *testEnv) string {
	t.Helper()
	egressLCs := map[string]bool{env.portLC[defaultEgress]: true, env.portLC[vrfEgress]: true}
	lcs := env.linecardsHosting(ingressPorts)
	for _, lc := range lcs {
		if !egressLCs[lc] {
			return lc
		}
	}
	t.Logf("RT-3.6.3: every ingress linecard %v hosts an egress port; using %s", lcs, lcs[0])
	return lcs[0]
}

// configureDUT implements the README test environment setup on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice, defaultNI string) {
	t.Helper()
	// Baseline prerequisite, intentionally not reverted by Cleanup: the platform hardware-init
	// config that VRF selection needs (e.g. an Arista TCAM profile). It is idempotent, and changing
	// it back requires a forwarding-agent restart, so it is left in place as in the other
	// featureprofiles VRF selection tests.
	if hwInit := cfgplugins.NewDUTHardwareInit(t, dut, cfgplugins.FeatureVrfSelectionExtended); hwInit != "" {
		cfgplugins.PushDUTHardwareInitConfig(t, dut, hwInit)
	}
	// Setup: create network instance VRF-1 (L3VRF).
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	cfgplugins.UpdateNetworkInstanceOnDut(t, dut, vrfName, cfgplugins.ConfigureNetworkInstance(t, dut, vrfName, false))

	// Setup: address port8 and assign its subinterface 0 to VRF-1.
	configureVRFEgress(t, dut)

	// Setup, in one gNMI Set: address port1..port7 (subinterface 0 stays in DEFAULT), the static
	// routes in VRF-1 and DEFAULT, and CLUSTER_FACING_POLICY with rule 10 (IPv4 198.51.100.0/24 ->
	// VRF-1) and rule 20 (IPv6 2001:db8:100::/48 -> VRF-1) bound to subinterface 0 of port1..port6.
	b := &gnmi.SetBatch{}
	for _, p := range allPorts {
		if p != vrfEgress {
			addDefaultPort(t, dut, b, p, defaultNI)
		}
	}
	addStaticRoutes(t, dut, b, defaultNI)
	var ingressNames []string
	for _, p := range ingressPorts {
		ingressNames = append(ingressNames, dut.Port(t, p).Name())
	}
	rules := []cfgplugins.VrfSelectionDstRule{
		{SeqID: ruleSeqIPv4, DestinationAddress: matchIPv4Prefix, NetworkInstance: vrfName},
	}
	if ipv6RuleSupported(dut) {
		rules = append(rules, cfgplugins.VrfSelectionDstRule{SeqID: ruleSeqIPv6, DestinationAddress: matchIPv6Prefix, NetworkInstance: vrfName})
	}
	cfgplugins.NewVrfSelectionPolicy(t, dut, b, cfgplugins.VrfSelectionPolicyConfig{
		NetworkInstance: defaultNI,
		PolicyName:      policyName,
		Rules:           rules,
		Interfaces:      ingressNames,
	})
	b.Set(t, dut)
}

// dutSnapshot holds the DUT config that the test modifies, captured before the test.
type dutSnapshot struct {
	// interfaces is the config of port1..port8.
	interfaces cfgplugins.InterfaceConfigSnapshot
	// defaultNIIntfs records, for the subinterface 0 of port1..port7, whether it was already
	// assigned to the default network instance. Only used with ExplicitInterfaceInDefaultVRF.
	defaultNIIntfs map[string]bool
}

// defaultNIInterfaceIDs returns the default network-instance interface ids that the test
// assigns: subinterface 0 of port1..port7, when the DUT needs explicit assignment.
func defaultNIInterfaceIDs(t *testing.T, dut *ondatra.DUTDevice) []string {
	t.Helper()
	if !deviations.ExplicitInterfaceInDefaultVRF(dut) {
		return nil
	}
	var ids []string
	for _, p := range allPorts {
		if p != vrfEgress {
			ids = append(ids, dut.Port(t, p).Name()+".0")
		}
	}
	return ids
}

// snapshotDUT captures the DUT config that the test modifies. The default network-instance
// interfaces are looked up one by one, because a multi-path Get can fail as a whole when any
// path is absent.
func snapshotDUT(t *testing.T, dut *ondatra.DUTDevice, defaultNI string) *dutSnapshot {
	t.Helper()
	var names []string
	for _, p := range allPorts {
		names = append(names, dut.Port(t, p).Name())
	}
	s := &dutSnapshot{
		interfaces:     cfgplugins.SnapshotInterfacesConfig(t, dut, names),
		defaultNIIntfs: make(map[string]bool),
	}
	for _, id := range defaultNIInterfaceIDs(t, dut) {
		s.defaultNIIntfs[id] = gnmi.LookupConfig(t, dut, gnmi.OC().NetworkInstance(defaultNI).Interface(id).Config()).IsPresent()
	}
	return s
}

// cleanupStep runs one cleanup step and reports its failure without skipping the next steps.
func cleanupStep(t *testing.T, desc string, step func(t testing.TB)) {
	t.Helper()
	if errMsg := testt.CaptureFatal(t, step); errMsg != nil {
		t.Errorf("Cleanup: %s failed: %s", desc, *errMsg)
	}
}

// cleanupDUT implements the README Cleanup on the DUT. Each step is a separate gNMI Set, so a
// step that fails (e.g. because the test failed before that config was pushed) does not prevent
// the others. The steps run in an order that never leaves a dangling reference: the policy and
// its bindings, so that no rule references VRF-1; the static routes; the default network-instance
// assignments added by the test and VRF-1 (which removes the port8 membership); then the
// pre-test interface config of port1..port8, which removes the addressing added by the test.
func cleanupDUT(t *testing.T, dut *ondatra.DUTDevice, defaultNI string, snap *dutSnapshot) {
	t.Helper()
	t.Log("Cleanup: removing the test configuration and restoring the original DUT configuration")
	cleanupStep(t, "deleting CLUSTER_FACING_POLICY and its bindings", func(t testing.TB) {
		b := &gnmi.SetBatch{}
		pf := gnmi.OC().NetworkInstance(defaultNI).PolicyForwarding()
		for _, p := range ingressPorts {
			gnmi.BatchDelete(b, pf.Interface(cfgplugins.PolicyForwardingInterfaceID(dut, dut.Port(t, p).Name(), 0)).Config())
		}
		gnmi.BatchDelete(b, pf.Policy(policyName).Config())
		b.Set(t, dut)
	})
	cleanupStep(t, "deleting the DEFAULT static routes", func(t testing.TB) {
		b := &gnmi.SetBatch{}
		for _, r := range staticRoutes(defaultNI) {
			if r.ni == vrfName {
				continue // Removed with VRF-1 below.
			}
			gnmi.BatchDelete(b, gnmi.OC().NetworkInstance(r.ni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut)).Static(r.prefix).Config())
		}
		b.Set(t, dut)
	})
	cleanupStep(t, "removing the test network-instance assignments and VRF-1", func(t testing.TB) {
		b := &gnmi.SetBatch{}
		for id, existed := range snap.defaultNIIntfs {
			if !existed {
				gnmi.BatchDelete(b, gnmi.OC().NetworkInstance(defaultNI).Interface(id).Config())
			}
		}
		gnmi.BatchDelete(b, gnmi.OC().NetworkInstance(vrfName).Config())
		b.Set(t, dut)
	})
	restore := cfgplugins.RestoreInterfacesConfig(t, dut, &gnmi.SetBatch{}, snap.interfaces)
	cleanupStep(t, "restoring the port1..port8 interface config", func(t testing.TB) {
		restore.Set(t, dut)
	})
}

// interfaceConfig returns the README setup interface config of port p: subinterface 0 with
// DUT 192.0.2.(4N-3)/30 and 2001:db8:0:N::1/126.
func interfaceConfig(t *testing.T, dut *ondatra.DUTDevice, p string) *oc.Interface {
	t.Helper()
	port := dut.Port(t, p)
	dutAttrs, _ := portAttrs(portNum(p))
	intf := dutAttrs.NewOCInterface(port.Name(), dut)
	if deviations.ExplicitPortSpeed(dut) {
		intf.GetOrCreateEthernet().PortSpeed = fptest.GetIfSpeed(t, port)
	}
	return intf
}

// addDefaultPort appends to b the config of port p, whose subinterface 0 stays in DEFAULT, and,
// when the DUT needs it, the explicit assignment of subinterface 0 to the default network
// instance. The assignment follows the interface config in the same SetRequest, so subinterface 0
// exists when it is assigned.
func addDefaultPort(t *testing.T, dut *ondatra.DUTDevice, b *gnmi.SetBatch, p, defaultNI string) {
	t.Helper()
	intf := interfaceConfig(t, dut, p)
	gnmi.BatchUpdate(b, gnmi.OC().Interface(intf.GetName()).Config(), intf)
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		id := intf.GetName() + ".0"
		gnmi.BatchUpdate(b, gnmi.OC().NetworkInstance(defaultNI).Interface(id).Config(), &oc.NetworkInstance_Interface{
			Id:           ygot.String(id),
			Interface:    intf.Name,
			Subinterface: ygot.Uint32(0),
		})
	}
}

// configureVRFEgress addresses port8 and assigns its subinterface 0 to VRF-1
// (/network-instances/network-instance/interfaces/interface/config/{interface,subinterface}).
// It is configured on its own because, with the InterfaceConfigVRFBeforeAddress deviation, the
// VRF assignment must be committed before the addresses are added.
func configureVRFEgress(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	intf := interfaceConfig(t, dut, vrfEgress)
	intfPath := gnmi.OC().Interface(intf.GetName()).Config()
	if deviations.InterfaceConfigVRFBeforeAddress(dut) {
		// Create the interface and subinterface 0 without addresses, assign it to VRF-1, then add
		// the addresses.
		base := &oc.Interface{Name: intf.Name, Type: intf.Type, Enabled: intf.Enabled}
		base.GetOrCreateSubinterface(0)
		gnmi.Update(t, dut, intfPath, base)
		fptest.AssignToNetworkInstance(t, dut, intf.GetName(), vrfName, 0)
		gnmi.Update(t, dut, intfPath, intf)
		return
	}
	// Subinterface 0 must exist before it is assigned.
	gnmi.Update(t, dut, intfPath, intf)
	fptest.AssignToNetworkInstance(t, dut, intf.GetName(), vrfName, 0)
}

// staticRoute is one static route: prefix in network instance ni via next hop nh.
type staticRoute struct{ ni, prefix, nh string }

// staticRoutes returns the README setup static routes: the matched prefixes routed in both
// VRF-1 (via ATE port8, 192.0.2.30 / 2001:db8:0:8::2) and DEFAULT (via ATE port7, 192.0.2.26 /
// 2001:db8:0:7::2), so traffic that bypasses the policy is delivered to port7 instead of port8,
// and the unmatched prefixes routed in DEFAULT only.
func staticRoutes(defaultNI string) []staticRoute {
	_, defaultATE := portAttrs(portNum(defaultEgress))
	_, vrfATE := portAttrs(portNum(vrfEgress))
	return []staticRoute{
		{vrfName, matchIPv4Prefix, vrfATE.IPv4},
		{vrfName, matchIPv6Prefix, vrfATE.IPv6},
		{defaultNI, matchIPv4Prefix, defaultATE.IPv4},
		{defaultNI, matchIPv6Prefix, defaultATE.IPv6},
		{defaultNI, nomatchIPv4Prefix, defaultATE.IPv4},
		{defaultNI, nomatchIPv6Prefix, defaultATE.IPv6},
	}
}

// addStaticRoutes appends the README setup static routes (see staticRoutes) to b.
func addStaticRoutes(t *testing.T, dut *ondatra.DUTDevice, b *gnmi.SetBatch, defaultNI string) {
	t.Helper()
	for _, r := range staticRoutes(defaultNI) {
		cfg := &cfgplugins.StaticRouteCfg{
			NetworkInstance: r.ni,
			Prefix:          r.prefix,
			NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
				"0": oc.UnionString(r.nh),
			},
		}
		if _, err := cfgplugins.NewStaticRouteCfg(b, cfg, dut); err != nil {
			t.Fatalf("NewStaticRouteCfg(%s %s): %v", r.ni, r.prefix, err)
		}
	}
}

// flowSpec describes one ingress flow and the only egress port allowed to receive it.
type flowSpec struct {
	name, ipType, dst, egress string
}

// ingressFlows returns the 4 RT-3.6.1 flows sent from ingress port p in traffic window window:
// IPV4-MATCH and IPV6-MATCH expected on port8, IPV4-NOMATCH and IPV6-NOMATCH expected on port7.
// Without rule 20 (MixedAFPolicyForwardingUnsupported) IPV6-MATCH is expected on port7.
func ingressFlows(dut *ondatra.DUTDevice, window, p string) []flowSpec {
	ipv6MatchEgress := vrfEgress
	if !ipv6RuleSupported(dut) {
		ipv6MatchEgress = defaultEgress
	}
	return []flowSpec{
		{fmt.Sprintf("%s-%s-IPV4-MATCH", window, p), "IPv4", matchIPv4Dst, vrfEgress},
		{fmt.Sprintf("%s-%s-IPV6-MATCH", window, p), "IPv6", matchIPv6Dst, ipv6MatchEgress},
		{fmt.Sprintf("%s-%s-IPV4-NOMATCH", window, p), "IPv4", nomatchIPv4Dst, defaultEgress},
		{fmt.Sprintf("%s-%s-IPV6-NOMATCH", window, p), "IPv6", nomatchIPv6Dst, defaultEgress},
	}
}

// configureATE pushes one OTG config with three disjoint flow sets, one per subtest (RT-3.6.1,
// RT-3.6.2 and RT-3.6.3), so the post-reboot traffic needs no OTG config push and starts from
// fresh counters.
func configureATE(t *testing.T, ate *ondatra.ATEDevice, dut *ondatra.DUTDevice) gosnappi.Config {
	t.Helper()
	cfg := gosnappi.NewConfig()
	for _, p := range allPorts {
		dutAttrs, ateAttrs := portAttrs(portNum(p))
		ateAttrs.AddToOTG(cfg, ate.Port(t, p), &dutAttrs)
	}
	for _, window := range []string{windowRT361, windowRT362, windowRT363} {
		for _, p := range ingressPorts {
			_, src := portAttrs(portNum(p))
			for _, f := range ingressFlows(dut, window, p) {
				_, dst := portAttrs(portNum(f.egress))
				addFlow(cfg, f, src, dst)
			}
		}
	}
	ate.OTG().PushConfig(t, cfg)
	ate.OTG().StartProtocols(t)
	return cfg
}

// addFlow adds flow f (512-byte frames, 1,000 pps, 10,000 packets) from src to dst to cfg.
func addFlow(cfg gosnappi.Config, f flowSpec, src, dst attrs.Attributes) {
	flow := cfg.Flows().Add().SetName(f.name)
	flow.Metrics().SetEnable(true)
	// The only Rx endpoint is the expected egress device, so traffic delivered to the other egress
	// port is counted as loss on this flow.
	flow.TxRx().Device().SetTxNames([]string{src.Name + "." + f.ipType}).SetRxNames([]string{dst.Name + "." + f.ipType})
	flow.Size().SetFixed(frameSize)
	flow.Rate().SetPps(flowRatePps)
	flow.Duration().FixedPackets().SetPackets(packetsPerFlow)
	flow.Packet().Add().Ethernet().Src().SetValue(src.MAC)
	if f.ipType == "IPv4" {
		ip := flow.Packet().Add().Ipv4()
		ip.Src().SetValue(src.IPv4)
		ip.Dst().SetValue(f.dst)
		return
	}
	ip := flow.Packet().Add().Ipv6()
	ip.Src().SetValue(src.IPv6)
	ip.Dst().SetValue(f.dst)
}

// stateBatch returns one query for the given state leaves. The leaves are added as explicit
// .../state/... queries: a gnmi.OCBatch path for a leaf that exists in both config and state is
// subscribed as .../*/leaf, which some devices reject (InvalidArgument) or answer with no data.
func stateBatch(t *testing.T, queries ...ygnmi.UntypedQuery) ygnmi.SingletonQuery[*oc.Root] {
	t.Helper()
	b := ygnmi.NewBatch[*oc.Root](gnmi.OC().State())
	if err := b.AddPaths(queries...); err != nil {
		t.Fatalf("stateBatch: AddPaths: %v", err)
	}
	return b.Query()
}

// awaitAll watches query (a batch of state paths) until notReady reports nothing, and returns
// what is still not ready when timeout passes. It uses components.WatchWithRetry, so a DUT
// that is still unreachable after a reboot is retried until the timeout instead of failing.
func awaitAll(t *testing.T, dut *ondatra.DUTDevice, query ygnmi.SingletonQuery[*oc.Root], timeout time.Duration, notReady func(*oc.Root) []string) []string {
	t.Helper()
	var last *oc.Root
	// Read the current state once first: some devices answer a one-time read of a leaf (e.g.
	// policy-forwarding interface state) without sending it in the initial sync of a streaming
	// subscription. The lookup is captured so that a DUT that is not reachable yet falls through
	// to the retrying watch below instead of failing the test.
	if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
		last, _ = gnmi.Lookup(t, dut, query).Val()
	}); errMsg != nil {
		t.Logf("Initial state lookup failed, watching instead: %s", *errMsg)
	} else if len(notReady(last)) == 0 {
		return nil
	}
	components.WatchWithRetry(t, dut, query, timeout, retryInterval, func(v *ygnmi.Value[*oc.Root]) bool {
		last, _ = v.Val()
		return len(notReady(last)) == 0
	})
	return notReady(last)
}

// awaitIngressBindings waits for every ingress port to report
// apply-vrf-selection-policy == CLUSTER_FACING_POLICY in state, and fails the test fatally,
// labelled with step, if any does not within timeout.
func awaitIngressBindings(t *testing.T, env *testEnv, step string, timeout time.Duration) {
	t.Helper()
	dut := env.dut
	pfPath := gnmi.OC().NetworkInstance(env.defaultNI).PolicyForwarding()
	ids := make(map[string]string, len(ingressPorts))
	var queries []ygnmi.UntypedQuery
	for _, p := range ingressPorts {
		ids[p] = cfgplugins.PolicyForwardingInterfaceID(dut, dut.Port(t, p).Name(), 0)
		queries = append(queries, pfPath.Interface(ids[p]).ApplyVrfSelectionPolicy().State())
	}
	unbound := awaitAll(t, dut, stateBatch(t, queries...), timeout, func(root *oc.Root) []string {
		var out []string
		for _, p := range ingressPorts {
			if root.GetNetworkInstance(env.defaultNI).GetPolicyForwarding().GetInterface(ids[p]).GetApplyVrfSelectionPolicy() != policyName {
				out = append(out, fmt.Sprintf("%s (%s on %s)", p, ids[p], env.portLC[p]))
			}
		}
		return out
	})
	if len(unbound) > 0 {
		t.Fatalf("%s: configuration failure: ingress ports %v do not report apply-vrf-selection-policy %q in state within %v", step, unbound, policyName, timeout)
	}
}

// awaitPolicyRules implements the RT-3.6.2 policy body persistence check: rules 10 and 20 of
// CLUSTER_FACING_POLICY must be present in state with their sequence-id, destination-address and
// network-instance (VRF-1). It fails the test fatally, labelled with step, if they are not within
// timeout.
func awaitPolicyRules(t *testing.T, env *testEnv, step string, timeout time.Duration) {
	t.Helper()
	dut := env.dut
	pol := gnmi.OC().NetworkInstance(env.defaultNI).PolicyForwarding().Policy(policyName)
	type wantRule struct {
		seq    uint32
		prefix string
		ipv6   bool
	}
	want := []wantRule{{seq: ruleSeqIPv4, prefix: matchIPv4Prefix}}
	if ipv6RuleSupported(dut) {
		want = append(want, wantRule{seq: ruleSeqIPv6, prefix: matchIPv6Prefix, ipv6: true})
	}
	var queries []ygnmi.UntypedQuery
	for _, w := range want {
		r := pol.Rule(w.seq)
		queries = append(queries, r.SequenceId().State(), r.Action().NetworkInstance().State())
		if w.ipv6 {
			queries = append(queries, r.Ipv6().DestinationAddress().State())
		} else {
			queries = append(queries, r.Ipv4().DestinationAddress().State())
		}
	}
	missing := awaitAll(t, dut, stateBatch(t, queries...), timeout, func(root *oc.Root) []string {
		var out []string
		p := root.GetNetworkInstance(env.defaultNI).GetPolicyForwarding().GetPolicy(policyName)
		for _, w := range want {
			r := p.GetRule(w.seq)
			dst := r.GetIpv4().GetDestinationAddress()
			if w.ipv6 {
				dst = r.GetIpv6().GetDestinationAddress()
			}
			if r.GetSequenceId() != w.seq || dst != w.prefix || r.GetAction().GetNetworkInstance() != vrfName {
				out = append(out, fmt.Sprintf("rule %d (got destination-address %q, network-instance %q)", w.seq, dst, r.GetAction().GetNetworkInstance()))
			}
		}
		return out
	})
	if len(missing) > 0 {
		t.Fatalf("%s: configuration persistence failure: %s %v not in state as configured within %v", step, policyName, missing, timeout)
	}
}

// awaitLinecardsActive waits for every linecard in lcs to report oper-status ACTIVE.
func awaitLinecardsActive(t *testing.T, dut *ondatra.DUTDevice, lcs []string, timeout time.Duration) {
	t.Helper()
	var queries []ygnmi.UntypedQuery
	for _, lc := range lcs {
		queries = append(queries, gnmi.OC().Component(lc).OperStatus().State())
	}
	notActive := awaitAll(t, dut, stateBatch(t, queries...), timeout, func(root *oc.Root) []string {
		var out []string
		for _, lc := range lcs {
			if root.GetComponent(lc).GetOperStatus() != oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE {
				out = append(out, lc)
			}
		}
		return out
	})
	if len(notActive) > 0 {
		t.Fatalf("Linecards %v are not ACTIVE within %v", notActive, timeout)
	}
}

// awaitPortsUp waits for every DUT port in ports to be operationally UP.
func awaitPortsUp(t *testing.T, dut *ondatra.DUTDevice, ports []string, timeout time.Duration) {
	t.Helper()
	var queries []ygnmi.UntypedQuery
	names := make(map[string]string, len(ports))
	for _, p := range ports {
		names[p] = dut.Port(t, p).Name()
		queries = append(queries, gnmi.OC().Interface(names[p]).OperStatus().State())
	}
	down := awaitAll(t, dut, stateBatch(t, queries...), timeout, func(root *oc.Root) []string {
		var out []string
		for _, p := range ports {
			if root.GetInterface(names[p]).GetOperStatus() != oc.Interface_OperStatus_UP {
				out = append(out, fmt.Sprintf("%s (%s)", p, names[p]))
			}
		}
		return out
	})
	if len(down) > 0 {
		t.Fatalf("Ports %v are not UP within %v", down, timeout)
	}
}

// verifyTraffic implements the RT-3.6.1 traffic checks for traffic window window: it sends the 4
// flows of every ingress port concurrently (24 flows) and checks that every flow transmitted
// 10,000 packets and that all of them reached its expected egress port (Rx == Tx), and that ATE
// port7 and port8 each received the expected total, allowing +1% for ARP/ND/LLDP and -0.1% for
// ATE port counter undercounting.
func verifyTraffic(t *testing.T, env *testEnv, window string) {
	t.Helper()
	dut, ate := env.dut, env.ate
	var flows []flowSpec
	var names []string
	wantRx := map[string]uint64{}
	for _, p := range ingressPorts {
		for _, f := range ingressFlows(dut, window, p) {
			flows = append(flows, f)
			names = append(names, f.name)
			wantRx[f.egress] += packetsPerFlow
		}
	}
	egressIDs := map[string]string{defaultEgress: ate.Port(t, defaultEgress).ID(), vrfEgress: ate.Port(t, vrfEgress).ID()}
	before := baselineEgressInFrames(t, ate, egressIDs, env.lastEgressInFrames)
	t.Logf("%s: baseline ATE egress InFrames: %s=%d, %s=%d", window, defaultEgress, before[defaultEgress], vrfEgress, before[vrfEgress])

	b := gnmi.OTGBatch()
	for _, n := range names {
		b.AddPaths(gnmi.OTG().Flow(n).Counters())
	}
	for _, id := range egressIDs {
		b.AddPaths(gnmi.OTG().Port(id).Counters())
	}
	counters := b.State()
	egressDelta := func(root *otg.Root, e string) uint64 {
		got := root.GetPort(egressIDs[e]).GetCounters().GetInFrames()
		return got - min(got, before[e])
	}

	cs := gosnappi.NewControlState()
	cs.Traffic().FlowTransmit().SetFlowNames(names).SetState(gosnappi.StateTrafficFlowTransmitState.START)
	ate.OTG().SetControlState(t, cs)
	// Wait until every flow has sent all of its packets, then for in-flight packets to be counted.
	gnmi.Watch(t, ate.OTG(), counters, trafficTimeout, func(v *ygnmi.Value[*otg.Root]) bool {
		root, _ := v.Val()
		for _, n := range names {
			if root.GetFlow(n).GetCounters().GetOutPkts() < packetsPerFlow {
				return false
			}
		}
		return true
	}).Await(t)
	gnmi.Watch(t, ate.OTG(), counters, rxSettleTimeout, func(v *ygnmi.Value[*otg.Root]) bool {
		root, _ := v.Val()
		for _, n := range names {
			if c := root.GetFlow(n).GetCounters(); c.GetInPkts() < c.GetOutPkts() {
				return false
			}
		}
		return egressDelta(root, defaultEgress) >= egressMin(wantRx[defaultEgress]) && egressDelta(root, vrfEgress) >= egressMin(wantRx[vrfEgress])
	}).Await(t)

	after, _ := gnmi.Lookup(t, ate.OTG(), counters).Val()
	// RT-3.6.1: for every flow Tx == 10,000 and Rx == Tx. A failure names the ingress port and its
	// linecard.
	for _, f := range flows {
		c := after.GetFlow(f.name).GetCounters()
		tx, rx := c.GetOutPkts(), c.GetInPkts()
		p := ingressPortOf(f.name)
		switch {
		case tx != packetsPerFlow:
			t.Errorf("%s (ingress %s on %s): transmitted %d packets, want %d", f.name, p, env.portLC[p], tx, packetsPerFlow)
		case rx != tx:
			t.Errorf("%s (ingress %s on %s): %d/%d packets reached %s; the policy is not programmed correctly on %s", f.name, p, env.portLC[p], rx, tx, f.egress, env.portLC[p])
		}
	}
	// RT-3.6.1: ATE port7 and port8 each received the expected total, allowing +1% for ARP/ND/LLDP
	// and -0.1% for ATE port counter undercounting. Since each flow's only receiver is its
	// expected egress port, the upper bound of these totals detects misdelivery.
	for _, e := range []string{defaultEgress, vrfEgress} {
		afterFrames := gnmi.Get(t, ate.OTG(), gnmi.OTG().Port(egressIDs[e]).State()).GetCounters().GetInFrames()
		env.lastEgressInFrames[e] = afterFrames
		if got, lo, hi := afterFrames-min(afterFrames, before[e]), egressMin(wantRx[e]), egressMax(wantRx[e]); got < lo || got > hi {
			t.Errorf("ATE %s InFrames delta got %d (before=%d, after=%d), want [%d, %d] (expected %d)", e, got, before[e], afterFrames, lo, hi, wantRx[e])
		}
	}
}

// baselineEgressInFrames reads each egress port's pre-traffic InFrames count via a single-port
// state query (as in otgutils.LogPortMetrics), avoiding mixed unstarted-flow lookups and
// port-only OTGBatch STREAM subscriptions that do not sync while traffic is idle. If a port
// transiently reports a count below the previous traffic window's count (prev), it waits up to
// baselineCounterTimeout via a single-port gnmi.Watch and re-reads once, while still accepting a
// counter that genuinely reset on a link bounce.
func baselineEgressInFrames(t *testing.T, ate *ondatra.ATEDevice, egressIDs map[string]string, prev map[string]uint64) map[string]uint64 {
	t.Helper()
	got := make(map[string]uint64, len(egressIDs))
	for _, e := range []string{defaultEgress, vrfEgress} {
		q := gnmi.OTG().Port(egressIDs[e]).State()
		p, ok := gnmi.Lookup(t, ate.OTG(), q).Val()
		if ok && p != nil {
			got[e] = p.GetCounters().GetInFrames()
		}
		if !ok || got[e] < prev[e] {
			gnmi.Watch(t, ate.OTG(), q, baselineCounterTimeout, func(v *ygnmi.Value[*otg.Port]) bool {
				if p, present := v.Val(); present && p != nil {
					got[e] = p.GetCounters().GetInFrames()
					return got[e] >= prev[e]
				}
				return false
			}).Await(t)
			if p, ok := gnmi.Lookup(t, ate.OTG(), q).Val(); ok && p != nil {
				got[e] = p.GetCounters().GetInFrames()
			}
		}
	}
	return got
}

// egressMin returns the lowest accepted ATE egress port InFrames delta for want flow packets.
func egressMin(want uint64) uint64 { return want - want*egressUnderPermille/1000 }

// egressMax returns the highest accepted ATE egress port InFrames delta for want flow packets.
func egressMax(want uint64) uint64 { return want + want*egressOverPermille/1000 }

// ingressPortOf returns the ingress port encoded in a flow name ("<window>-<port>-...").
func ingressPortOf(flowName string) string {
	parts := strings.SplitN(flowName, "-", 3)
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}
