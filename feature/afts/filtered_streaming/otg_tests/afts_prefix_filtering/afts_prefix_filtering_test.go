// Copyright 2024 Google LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package aftsprefixfiltering_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/featureprofiles/internal/telemetry/aftcache"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	aftConvergenceTime   = 120 * time.Second
	noUpdateWindow       = 3 * time.Second
	policyMatchAll       = "POLICY-MATCH-ALL"
	policyPrefixSetA     = "POLICY-PREFIX-SET-A"
	policyPrefixSetB     = "POLICY-PREFIX-SET-B"
	policyPrefixSetVRFA  = "POLICY-PREFIX-SET-VRF-A"
	policySubnet         = "POLICY-SUBNET"
	policySubnetV6       = "POLICY-SUBNET-V6"
	policyMultiStmt      = "POLICY-MULTI-STMT"
	policyDenyPrefixSetA = "POLICY-DENY-PREFIX-SET-A"
	policyTagMatch       = "POLICY-TAG-MATCH"
	policyNotYetExist    = "POLICY-DOES-NOT-YET-EXIST"
	psA                  = "PREFIX-SET-A"
	psB                  = "PREFIX-SET-B"
	psVRFA               = "PREFIX-SET-VRF-A"
	psSubnet             = "PREFIX-SET-SUBNET"
	psSubnetV6           = "PREFIX-SET-SUBNET-V6"
	psNotExist           = "PREFIX-SET-NOT-YET-EXIST"

	pfx19851100024  = "198.51.100.0/24"
	pfx2030113028   = "203.0.113.0/28"
	pfx10064024     = "100.64.0.0/24"
	pfx1985110132   = "198.51.100.1/32"
	pfx100641024    = "100.64.1.0/24"
	pfx20301130024  = "203.0.113.0/24"
	pfx2030113128   = "203.0.113.128/25"
	pfx198511001282 = "198.51.100.128/25"
	pfx2001DB8164   = "2001:DB8:1::/64"
	pfx2001DB8364   = "2001:DB8:3::/64"
	pfx2001DB8264   = "2001:DB8:2::/64"
	pfx2001DB821128 = "2001:DB8:2::1/128"
	nhIPv4          = "192.0.2.2"
	nhIPv6          = "2001:DB8::2"
	nhIPv6LinkLocal = "fe80::200:2ff:fe02:202"
	v4PrefixLen     = uint8(30)
	v6PrefixLen     = uint8(126)
)

var (
	dutPort1 = attrs.Attributes{Name: "dutPort1", IPv4: "192.0.2.1", IPv6: "2001:DB8::1", IPv4Len: v4PrefixLen, IPv6Len: v6PrefixLen}
	atePort1 = attrs.Attributes{Name: "atePort1", MAC: "02:00:01:01:01:01", IPv4: "192.0.2.2", IPv6: "2001:DB8::2", IPv4Len: v4PrefixLen, IPv6Len: v6PrefixLen}
	dutPort2 = attrs.Attributes{Name: "dutPort2", IPv4: "192.0.2.5", IPv6: "2001:DB8::5", IPv4Len: v4PrefixLen, IPv6Len: v6PrefixLen}
	atePort2 = attrs.Attributes{Name: "atePort2", MAC: "02:00:02:01:01:01", IPv4: "192.0.2.6", IPv6: "2001:DB8::6", IPv4Len: v4PrefixLen, IPv6Len: v6PrefixLen}
)

type globalFilter struct {
	ipv4Policy, ipv6Policy string
}

type policyStmt struct {
	seq, prefixSet string
	result         oc.E_RoutingPolicy_PolicyResultType
}

type testCase struct {
	name string
	fn   func(*testing.T, *testFixture)
}

type testFixture struct {
	dut        *ondatra.DUTDevice
	gnmiClient gpb.GNMIClient
	aftSession *aftcache.AFTStreamSession
	subtestCtx context.Context
	ctxCancel  context.CancelFunc
}

type subscriptionParams struct {
	filter                                     globalFilter
	wantPresent, wantAbsent                    []string
	matchingNewPfx, noMatchNewPfx, excludedPfx string
}

// nhMapIPv4 returns a map of IPv4 next hops.
func (f *testFixture) nhMapIPv4() map[string]bool {
	return map[string]bool{nhIPv4: true}
}

// nhMapIPv6 returns a map of IPv6 next hops.
func (f *testFixture) nhMapIPv6() map[string]bool {
	if deviations.LinkLocalInsteadOfNh(f.dut) {
		return map[string]bool{nhIPv6LinkLocal: true}
	}
	return map[string]bool{nhIPv6: true}
}

// listenUntil listens to the AFT stream until the specified stopping condition is met and returns the resulting AFTData.
// It explicitly registers the VerifyAtomicFlagHook to enforce transaction validation before updating the cache.
func (f *testFixture) listenUntil(t *testing.T, stop aftcache.PeriodicHook) (*aftcache.AFTData, error) {
	t.Helper()
	hooks := []aftcache.NotificationHook{aftcache.VerifyAtomicFlagHook(t)}
	f.aftSession.ListenUntilPreUpdateHook(f.subtestCtx, t, aftConvergenceTime, hooks, stop)
	aft, err := f.aftSession.ToAFT(t, f.dut)
	return aft, err
}

// syncAndGet syncs the AFT and returns the resulting AFTData.
func (f *testFixture) syncAndGet(t *testing.T, wantPrefixes map[string]bool) (*aftcache.AFTData, error) {
	t.Helper()
	return f.listenUntil(t, aftcache.InitialSyncStoppingCondition(t, f.dut, wantPrefixes, f.nhMapIPv4(), f.nhMapIPv6()))
}

// waitPrefixesAbsent waits for the specified prefixes to be absent from the AFT.
func (f *testFixture) waitPrefixesAbsent(t *testing.T, prefixes map[string]bool) (*aftcache.AFTData, error) {
	t.Helper()
	return f.listenUntil(t, aftcache.DeletionStoppingCondition(t, f.dut, prefixes))
}

// verifyState verifies the state of the AFT data.
func verifyState(aft *aftcache.AFTData, prefixes []string, wantPresent bool) []error {
	errs := []error(nil)
	for _, p := range prefixes {
		_, exists := aft.Prefixes[p]
		if exists != wantPresent {
			errs = append(errs,
				fmt.Errorf("prefix %s: expected present=%t, got=%t",
					p, wantPresent, exists))
		}
	}
	return errs
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// TestAFTPrefixFiltering tests the behavior of the AFT global filter with various policies and prefix sets.
func TestAFTPrefixFiltering(t *testing.T) {
	dut, ate := ondatra.DUT(t, "dut"), ondatra.ATE(t, "ate")
	configureDUT(t, dut)
	configureATE(t, ate)

	testTable := []testCase{
		{
			name: "AFT-6.1.1-IPv4-PrefixSetA",
			fn: func(t *testing.T, fix *testFixture) {
				t.Helper()
				testSubscription(t, fix, subscriptionParams{
					filter: globalFilter{ipv4Policy: policyPrefixSetA}, wantPresent: []string{pfx19851100024, pfx2030113028},
					wantAbsent: []string{pfx10064024}, matchingNewPfx: pfx1985110132, noMatchNewPfx: pfx100641024, excludedPfx: pfx10064024,
				})
			},
		},
		{
			name: "AFT-6.1.1-IPv6-PrefixSetB",
			fn: func(t *testing.T, fix *testFixture) {
				t.Helper()
				testSubscription(t, fix, subscriptionParams{filter: globalFilter{ipv6Policy: policyPrefixSetB}, wantAbsent: []string{pfx2001DB8164, pfx2001DB8364}, excludedPfx: pfx10064024})
			},
		},
		{
			name: "AFT-6.1.1-IPv4-Subnet",
			fn: func(t *testing.T, fix *testFixture) {
				t.Helper()
				testSubscription(t, fix, subscriptionParams{filter: globalFilter{ipv4Policy: policySubnet}, wantAbsent: []string{pfx19851100024, pfx10064024}, excludedPfx: pfx10064024})
			},
		},
		{
			name: "AFT-6.1.1-IPv6-SubnetV6",
			fn: func(t *testing.T, fix *testFixture) {
				t.Helper()
				testSubscription(t, fix, subscriptionParams{filter: globalFilter{ipv6Policy: policySubnetV6}, wantAbsent: []string{pfx2001DB8164}, excludedPfx: pfx10064024})
			},
		},
		{name: "AFT-6.1.2-NonExistentPolicy", fn: testNonExistentPolicy},
		{name: "AFT-6.1.3-PolicyDeletion", fn: testPolicyDeletion},
		{name: "AFT-6.1.4-ChangePrefixSetInActivePolicy", fn: testChangePrefixSet},
		{name: "AFT-6.1.5-MultiStatementPolicy", fn: testMultiStatementPolicy},
		{name: "AFT-6.1.6-DenyActionExclusionList", fn: testDenyAction},
		{name: "AFT-6.1.7-NonPrefixSetMatchCriteria", fn: testNonPrefixSetMatch},
	}

	for _, tc := range testTable {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			gnmiClient, err := dut.RawAPIs().BindingDUT().DialGNMI(t.Context())
			if err != nil {
				t.Errorf("DialGNMI: %v", err)
			}
			subtestCtx, cancel := context.WithCancel(t.Context())
			fix := &testFixture{dut: dut, gnmiClient: gnmiClient, subtestCtx: subtestCtx, ctxCancel: cancel}
			t.Cleanup(cancel)
			t.Cleanup(func() { deleteFilter(t, dut) })
			fix.aftSession = aftcache.NewAFTStreamSession(subtestCtx, t, gnmiClient, dut)
			tc.fn(t, fix)
		})
	}
}

// testSubscription tests the behavior of the filter when a subscription is applied.
func testSubscription(t *testing.T, fix *testFixture, p subscriptionParams) {
	if _, err := setFilter(t, fix.dut, p.filter); err != nil {
		t.Errorf("setFilter(%+v): %v", p.filter, err)
	}
	wantMap := make(map[string]bool)
	for _, pfx := range p.wantPresent {
		wantMap[pfx] = true
	}
	aft, err := fix.syncAndGet(t, wantMap)
	if err != nil {
		t.Errorf("syncAndGet: %v", err)
	}
	errs := verifyState(aft, p.wantPresent, true)
	errs = append(errs, verifyState(aft, p.wantAbsent, false)...)
	if len(errs) > 0 {
		t.Errorf("verification failed: %v", errs)
	}

	if p.matchingNewPfx != "" {
		addStaticRoute(t, fix.dut, p.matchingNewPfx, nhIPv4)
		_, err := fix.listenUntil(t, aftcache.InitialSyncStoppingCondition(t, fix.dut, map[string]bool{p.matchingNewPfx: true}, fix.nhMapIPv4(), fix.nhMapIPv6()))
		if err != nil {
			t.Errorf("listenUntil: %v", err)
		}
		deleteStaticRoute(t, fix.dut, p.matchingNewPfx)
		_, err = fix.waitPrefixesAbsent(t, map[string]bool{p.matchingNewPfx: true})
		if err != nil {
			t.Errorf("waitPrefixesAbsent: %v", err)
		}
	}
	if p.noMatchNewPfx != "" {
		addStaticRoute(t, fix.dut, p.noMatchNewPfx, nhIPv4)
		time.Sleep(noUpdateWindow)
		aft, err := fix.syncAndGet(t, map[string]bool{})
		if err != nil {
			t.Errorf("syncAndGet: %v", err)
		}
		errs := verifyState(aft, []string{p.noMatchNewPfx}, false)
		if len(errs) > 0 {
			t.Errorf("verification failed: %v", errs)
		}
		deleteStaticRoute(t, fix.dut, p.noMatchNewPfx)
	}
	deleteFilter(t, fix.dut)
	if p.excludedPfx != "" {
		_, err := fix.listenUntil(t, aftcache.InitialSyncStoppingCondition(t, fix.dut, map[string]bool{p.excludedPfx: true}, fix.nhMapIPv4(), fix.nhMapIPv6()))
		if err != nil {
			t.Errorf("listenUntil: %v", err)
		}
	}
}

// testNonExistentPolicy tests the behavior of the filter when a non-existent policy is applied.
func testNonExistentPolicy(t *testing.T, fix *testFixture) {
	dut := fix.dut
	if _, err := setFilter(t, dut, globalFilter{policyNotYetExist, policyNotYetExist}); err == nil {
		t.Errorf("expected FAILED_PRECONDITION applying non-existent policy, got nil")
	} else if s, ok := status.FromError(err); !ok || s.Code() != codes.FailedPrecondition {
		t.Errorf("expected FAILED_PRECONDITION, got %v", err)
	}

	installExactPrefixSet(t, dut, psNotExist, []string{pfx198511001282})
	installPolicy(t, dut, policyNotYetExist, []policyStmt{{seq: "10", prefixSet: psNotExist, result: oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE}})
	addStaticRoute(t, dut, pfx198511001282, nhIPv4)
	t.Cleanup(func() {
		deleteStaticRoute(t, dut, pfx198511001282)
		gnmi.Delete(t, dut, gnmi.OC().RoutingPolicy().PolicyDefinition(policyNotYetExist).Config())
		gnmi.Delete(t, dut, gnmi.OC().RoutingPolicy().DefinedSets().PrefixSet(psNotExist).Config())
	})
	if _, err := setFilter(t, dut, globalFilter{policyNotYetExist, policyNotYetExist}); err != nil {
		t.Errorf("unexpected error applying now-existent policy: %v", err)
	}
	aft, err := fix.syncAndGet(t, map[string]bool{pfx198511001282: true})
	if err != nil {
		t.Errorf("syncAndGet: %v", err)
	}
	errs := verifyState(aft, []string{pfx198511001282}, true)
	errs = append(errs, verifyState(aft, []string{pfx10064024}, false)...)
	if len(errs) > 0 {
		t.Errorf("verification failed: %v", errs)
	}
}

// testPolicyDeletion tests the behavior of the filter when a policy is deleted.
func testPolicyDeletion(t *testing.T, fix *testFixture) {
	dut := fix.dut
	if _, err := setFilter(t, dut, globalFilter{ipv4Policy: policyPrefixSetA}); err != nil {
		t.Errorf("setFilter(%+v): %v", globalFilter{ipv4Policy: policyPrefixSetA}, err)
	}
	_, err := fix.syncAndGet(t, map[string]bool{pfx19851100024: true, pfx2030113028: true})
	if err != nil {
		t.Errorf("syncAndGet: %v", err)
	}

	policyPath := gnmi.OC().RoutingPolicy().PolicyDefinition(policyPrefixSetA).Config()

	if deviations.AftsGlobalFilterPolicyOCUnsupported(dut) {
		gnmi.Delete(t, dut, policyPath)
	} else {
		gnmiClient := dut.RawAPIs().GNMI(t)
		delPath := buildOCPath("routing-policy/policy-definitions/policy-definition[name=" + policyPrefixSetA + "]")

		if _, err := gnmiClient.Set(context.Background(), &gpb.SetRequest{Delete: []*gpb.Path{delPath}}); err == nil {
			t.Errorf("expected FAILED_PRECONDITION, got nil")
		}

		filterPath := buildFilterPath(deviations.DefaultNetworkInstance(dut))
		atomicDeleteReq := &gpb.SetRequest{
			Delete: []*gpb.Path{filterPath, delPath},
		}
		if _, err := gnmiClient.Set(context.Background(), atomicDeleteReq); err != nil {
			t.Errorf("atomic delete: %v", err)
		}
	}

	_, err = fix.syncAndGet(t, map[string]bool{pfx10064024: true})
	if err != nil {
		t.Errorf("syncAndGet: %v", err)
	}
	configureRoutingPolicies(t, dut)
	if _, err := setFilter(t, dut, globalFilter{ipv4Policy: policyPrefixSetA}); err != nil {
		t.Errorf("setFilter(%+v): %v", globalFilter{ipv4Policy: policyPrefixSetA}, err)
	}
	aft, err := fix.syncAndGet(t, map[string]bool{pfx19851100024: true, pfx2030113028: true})
	if err != nil {
		t.Errorf("syncAndGet: %v", err)
	}
	errs := verifyState(aft, []string{pfx19851100024, pfx2030113028}, true)
	if len(errs) > 0 {
		t.Errorf("verification failed: %v", errs)
	}

	deleteFilter(t, dut)
	gnmi.Delete(t, dut, policyPath)
	_, err = fix.listenUntil(t, aftcache.InitialSyncStoppingCondition(t, fix.dut, map[string]bool{pfx10064024: true}, fix.nhMapIPv4(), fix.nhMapIPv6()))
	if err != nil {
		t.Errorf("listenUntil: %v", err)
	}
	configureRoutingPolicies(t, dut)
}

// testChangePrefixSet tests the behavior of the filter when the prefix set is changed.
func testChangePrefixSet(t *testing.T, fix *testFixture) {
	dut := fix.dut
	if _, err := setFilter(t, dut, globalFilter{ipv4Policy: policyPrefixSetA}); err != nil {
		t.Errorf("setFilter(%+v): %v", globalFilter{ipv4Policy: policyPrefixSetA}, err)
	}
	aft, err := fix.syncAndGet(t, map[string]bool{pfx19851100024: true, pfx2030113028: true})
	if err != nil {
		t.Errorf("syncAndGet: %v", err)
	}
	errs := verifyState(aft, []string{pfx19851100024, pfx2030113028}, true)
	if len(errs) > 0 {
		t.Errorf("verification failed: %v", errs)
	}

	installPolicy(t, dut, policyPrefixSetA, []policyStmt{{seq: "10", prefixSet: psB, result: oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE}})
	aftAfter, err := fix.listenUntil(t, aftcache.DeletionStoppingCondition(t, dut, map[string]bool{pfx19851100024: true, pfx2030113028: true}))
	if err != nil {
		t.Errorf("listenUntil: %v", err)
	}
	errs = verifyState(aftAfter, []string{pfx19851100024, pfx2030113028, pfx2001DB8264, pfx2001DB821128}, false)
	if len(errs) > 0 {
		t.Errorf("verification failed: %v", errs)
	}
	configureRoutingPolicies(t, dut)
}

// testMultiStatementPolicy tests a policy with multiple statements.
func testMultiStatementPolicy(t *testing.T, fix *testFixture) {
	addStaticRoute(t, fix.dut, pfx2030113128, nhIPv4)
	defer deleteStaticRoute(t, fix.dut, pfx2030113128)
	if _, err := setFilter(t, fix.dut, globalFilter{ipv4Policy: policyMultiStmt}); err != nil {
		t.Errorf("setFilter(%+v): %v", globalFilter{ipv4Policy: policyMultiStmt}, err)
	}
	aft, err := fix.syncAndGet(t, map[string]bool{pfx19851100024: true, pfx2030113028: true, pfx2030113128: true})
	if err != nil {
		t.Errorf("syncAndGet: %v", err)
	}
	errs := verifyState(aft, []string{pfx19851100024, pfx2030113028, pfx2030113128}, true)
	errs = append(errs, verifyState(aft, []string{pfx10064024}, false)...)
	if len(errs) > 0 {
		t.Errorf("verification failed: %v", errs)
	}
}

// testDenyAction verifies that a policy statement with a deny action properly excludes matching prefixes from the AFT
func testDenyAction(t *testing.T, fix *testFixture) {
	if _, err := setFilter(t, fix.dut, globalFilter{ipv4Policy: policyDenyPrefixSetA}); err != nil {
		t.Errorf("setFilter(%+v): %v", globalFilter{ipv4Policy: policyDenyPrefixSetA}, err)
	}
	aft, err := fix.syncAndGet(t, map[string]bool{pfx10064024: true})
	if err != nil {
		t.Errorf("syncAndGet: %v", err)
	}
	errs := verifyState(aft, []string{pfx19851100024, pfx2030113028}, false)
	errs = append(errs, verifyState(aft, []string{pfx10064024}, true)...)
	if len(errs) > 0 {
		t.Errorf("verification failed: %v", errs)
	}
}

// testNonPrefixSetMatch tests the behavior of the filter when matching on non-prefix sets.
func testNonPrefixSetMatch(t *testing.T, fix *testFixture) {
	t.Helper()
	dut := fix.dut
	if _, err := setFilter(t, dut, globalFilter{ipv4Policy: policyTagMatch}); err != nil {
		t.Errorf("setFilter(%+v): %v", globalFilter{ipv4Policy: policyTagMatch}, err)
	}
	aft, err := fix.syncAndGet(t, map[string]bool{})
	if err != nil {
		t.Errorf("syncAndGet: %v", err)
	}
	errs := verifyState(aft, []string{pfx19851100024, pfx2030113028, pfx10064024}, false)
	if len(errs) > 0 {
		t.Errorf("verification failed: %v", errs)
	}
	deleteFilter(t, dut)

	if _, err := setFilter(t, dut, globalFilter{ipv4Policy: policyPrefixSetA}); err != nil {
		t.Errorf("setFilter(%+v): %v", globalFilter{ipv4Policy: policyPrefixSetA}, err)
	}
	aft, err = fix.syncAndGet(t, map[string]bool{pfx19851100024: true, pfx2030113028: true})
	if err != nil {
		t.Errorf("syncAndGet: %v", err)
	}
	errs = verifyState(aft, []string{pfx19851100024, pfx2030113028}, true)
	if len(errs) > 0 {
		t.Errorf("verification failed: %v", errs)
	}
	if _, err := setFilter(t, dut, globalFilter{ipv4Policy: policyTagMatch}); err != nil {
		t.Errorf("setFilter(%+v): %v", globalFilter{ipv4Policy: policyTagMatch}, err)
	}
	aftAfter, err := fix.listenUntil(t, aftcache.DeletionStoppingCondition(t, dut, map[string]bool{pfx19851100024: true, pfx2030113028: true}))
	if err != nil {
		t.Errorf("listenUntil: %v", err)
	}
	errs = verifyState(aftAfter, []string{pfx19851100024, pfx2030113028}, false)
	if len(errs) > 0 {
		t.Errorf("verification failed: %v", errs)
	}
}

// configureDUT configures the DUT with the necessary interfaces, static routes, and routing policies for the tests.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	batch := &gnmi.SetBatch{}
	for _, p := range []struct {
		attr *attrs.Attributes
		port *ondatra.Port
	}{{&dutPort1, dut.Port(t, "port1")}, {&dutPort2, dut.Port(t, "port2")}} {
		i := p.attr.NewOCInterface(p.port.Name(), dut)
		if deviations.InterfaceEnabled(dut) {
			i.Enabled = ygot.Bool(true)
		}
		gnmi.BatchUpdate(batch, gnmi.OC().Interface(p.port.Name()).Config(), i)
	}
	configureStaticRoutes(t, dut, batch)
	batch.Set(t, dut)
	configureRoutingPolicies(t, dut)
}

// configureDUT configures the DUT with the necessary static routes
func configureStaticRoutes(t *testing.T, dut *ondatra.DUTDevice, batch *gnmi.SetBatch) {
	t.Helper()
	ni := deviations.DefaultNetworkInstance(dut)
	v4 := []string{pfx19851100024, pfx2030113028, pfx10064024}
	for i, pfx := range v4 {
		cfgplugins.NewStaticRouteCfg(batch, &cfgplugins.StaticRouteCfg{NetworkInstance: ni, Prefix: pfx, NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{fmt.Sprintf("%d", i): oc.UnionString(nhIPv4)}}, dut)
	}
	for i, pfx := range []string{pfx2001DB8164, pfx2001DB8364} {
		cfgplugins.NewStaticRouteCfg(batch, &cfgplugins.StaticRouteCfg{NetworkInstance: ni, Prefix: pfx, NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{fmt.Sprintf("%d", i+len(v4)): oc.UnionString(nhIPv6)}}, dut)
	}
}

// configureATE configures the ATE with the necessary ports and protocols.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) {
	t.Helper()
	topo := gosnappi.NewConfig()
	atePort1.AddToOTG(topo, ate.Port(t, "port1"), &dutPort1)
	atePort2.AddToOTG(topo, ate.Port(t, "port2"), &dutPort2)
	ate.OTG().PushConfig(t, topo)
	ate.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), topo, "IPv4")
	otgutils.WaitForARP(t, ate.OTG(), topo, "IPv6")
}

// configureRoutingPolicies sets up the routing policies and prefix sets used in the tests.
func configureRoutingPolicies(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	installExactPrefixSet(t, dut, psA, []string{pfx19851100024, pfx2030113028, pfx1985110132})
	installExactPrefixSet(t, dut, psB, []string{pfx2001DB8264, pfx2001DB821128})
	installRangePrefixSet(t, dut, psVRFA, pfx100641024, "24..32")
	installRangePrefixSet(t, dut, psSubnet, pfx20301130024, "25..32")
	installRangePrefixSet(t, dut, psSubnetV6, pfx2001DB8364, "65..128")
	installAcceptAllPolicy(t, dut, policyMatchAll)
	installPolicy(t, dut, policyPrefixSetA, []policyStmt{{seq: "10", prefixSet: psA, result: oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE}})
	installPolicy(t, dut, policyPrefixSetB, []policyStmt{{seq: "10", prefixSet: psB, result: oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE}})
	installPolicy(t, dut, policyPrefixSetVRFA, []policyStmt{{seq: "10", prefixSet: psVRFA, result: oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE}})
	installPolicy(t, dut, policySubnet, []policyStmt{{seq: "10", prefixSet: psSubnet, result: oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE}})
	installPolicy(t, dut, policySubnetV6, []policyStmt{{seq: "10", prefixSet: psSubnetV6, result: oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE}})
	installPolicy(t, dut, policyMultiStmt, []policyStmt{{seq: "10", prefixSet: psA, result: oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE}, {seq: "20", prefixSet: psSubnet, result: oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE}})
	installPolicy(t, dut, policyDenyPrefixSetA, []policyStmt{{seq: "10", prefixSet: psA, result: oc.RoutingPolicy_PolicyResultType_REJECT_ROUTE}, {seq: "20", result: oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE}})
	installTagMatchPolicy(t, dut, policyTagMatch, 999)
}

// buildFilterPath builds a path for the global filter.
func buildFilterPath(ni string) *gpb.Path {
	return &gpb.Path{Origin: "openconfig", Elem: []*gpb.PathElem{{Name: "network-instances"}, {Name: "network-instance", Key: map[string]string{"name": ni}}, {Name: "afts"}, {Name: "global-filter"}, {Name: "config"}}}
}

// buildOCPath builds a path for the given OpenConfig path.
func buildOCPath(path string) *gpb.Path {
	return &gpb.Path{Origin: "openconfig", Elem: []*gpb.PathElem{{Name: path}}}
}

// setFilter configures the AFT global filter with the given ipv4/ipv6 policy
// names and returns any error from the underlying Set RPC. On platforms where
// AftsGlobalFilterPolicyOCUnsupported is true, this is a no-op that returns a
// nil response and a nil error.
func setFilter(t *testing.T, dut *ondatra.DUTDevice, _ globalFilter) (*gpb.SetResponse, error) {
	t.Helper()
	if deviations.AftsGlobalFilterPolicyOCUnsupported(dut) {
		if dut.Vendor() == ondatra.ARISTA {
			t.Log("Skipping AFT global-filter attachment: unsupported on EOS")
		}
		return nil, nil
	}
	//	 Uncomment the below code once we have support for OC-based configuration of the global filter
	//		root := &oc.Root{}
	//		ni := root.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	//		gf := ni.GetOrCreateAfts().GetOrCreateGlobalFilter()
	//		if f.ipv4Policy != "" {
	//			gf.Ipv4Policy = ygot.String(f.ipv4Policy)
	//		}
	//		if f.ipv6Policy != "" {
	//			gf.Ipv6Policy = ygot.String(f.ipv6Policy)
	//		}
	//		gnmi.Update(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Afts().GlobalFilter().Config(), gf)
	return nil, nil
}

// deleteFilter deletes the AFT global filter. On platforms where
// AftsGlobalFilterPolicyOCUnsupported is true, this is a no-op.
func deleteFilter(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	if deviations.AftsGlobalFilterPolicyOCUnsupported(dut) {
		return
	}
	//	 Uncomment the below code once we have support for OC-based configuration of the global filter
	//		gnmi.Delete(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Afts().GlobalFilter().Config())
}

// addStaticRoute adds a static route.
func addStaticRoute(t *testing.T, dut *ondatra.DUTDevice, prefix, nextHop string) {
	t.Helper()
	batch := &gnmi.SetBatch{}
	cfgplugins.NewStaticRouteCfg(batch, &cfgplugins.StaticRouteCfg{NetworkInstance: deviations.DefaultNetworkInstance(dut), Prefix: prefix, NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{"0": oc.UnionString(nextHop)}}, dut)
	batch.Set(t, dut)
}

// deleteStaticRoute deletes a static route.
func deleteStaticRoute(t *testing.T, dut *ondatra.DUTDevice, prefix string) {
	t.Helper()
	gnmi.Delete(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut)).Static(prefix).Config())
}

// installExactPrefixSet installs a prefix set with an exact list of IP addresses.
func installExactPrefixSet(t *testing.T, dut *ondatra.DUTDevice, name string, prefixes []string) {
	t.Helper()
	ps := &oc.RoutingPolicy_DefinedSets_PrefixSet{Name: ygot.String(name)}
	for _, p := range prefixes {
		entry := ps.GetOrCreatePrefix(p, "exact")
		entry.IpPrefix, entry.MasklengthRange = ygot.String(p), ygot.String("exact")
	}
	gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().DefinedSets().PrefixSet(name).Config(), ps)
}

// installRangePrefixSet installs a prefix set with a range of IP addresses.
func installRangePrefixSet(t *testing.T, dut *ondatra.DUTDevice, name, ipPrefix, maskRange string) {
	t.Helper()
	ps := &oc.RoutingPolicy_DefinedSets_PrefixSet{Name: ygot.String(name)}
	entry := ps.GetOrCreatePrefix(ipPrefix, maskRange)
	entry.IpPrefix, entry.MasklengthRange = ygot.String(ipPrefix), ygot.String(maskRange)
	gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().DefinedSets().PrefixSet(name).Config(), ps)
}

// installAcceptAllPolicy installs a policy that accepts all routes.
func installAcceptAllPolicy(t *testing.T, dut *ondatra.DUTDevice, name string) {
	t.Helper()
	pd := &oc.RoutingPolicy_PolicyDefinition{Name: ygot.String(name)}
	stmt, _ := pd.AppendNewStatement("10")
	stmt.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
	gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().PolicyDefinition(name).Config(), pd)
}

// installPolicy installs a policy with the given statements.
func installPolicy(t *testing.T, dut *ondatra.DUTDevice, name string, stmts []policyStmt) {
	t.Helper()
	pd := &oc.RoutingPolicy_PolicyDefinition{Name: ygot.String(name)}
	for _, s := range stmts {
		stmt, _ := pd.AppendNewStatement(s.seq)
		if s.prefixSet != "" {
			mps := stmt.GetOrCreateConditions().GetOrCreateMatchPrefixSet()
			mps.PrefixSet, mps.MatchSetOptions = ygot.String(s.prefixSet), oc.RoutingPolicy_MatchSetOptionsRestrictedType_ANY
		}
		stmt.GetOrCreateActions().PolicyResult = s.result
	}
	gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().PolicyDefinition(name).Config(), pd)
}

// installTagMatchPolicy installs a policy that matches on a specific tag value.
func installTagMatchPolicy(t *testing.T, dut *ondatra.DUTDevice, name string, tag uint32) {
	t.Helper()
	tagSetName := name + "-TAG-SET"
	ts := &oc.RoutingPolicy_DefinedSets_TagSet{Name: ygot.String(tagSetName)}
	ts.TagValue = []oc.RoutingPolicy_DefinedSets_TagSet_TagValue_Union{oc.UnionUint32(tag)}
	gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().DefinedSets().TagSet(tagSetName).Config(), ts)
	pd := &oc.RoutingPolicy_PolicyDefinition{Name: ygot.String(name)}
	stmt, _ := pd.AppendNewStatement("10")
	stmt.GetOrCreateConditions().GetOrCreateMatchTagSet().TagSet = ygot.String(tagSetName)
	stmt.GetOrCreateConditions().GetOrCreateMatchTagSet().MatchSetOptions = oc.RoutingPolicy_MatchSetOptionsRestrictedType_ANY
	stmt.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
	gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().PolicyDefinition(name).Config(), pd)
}
