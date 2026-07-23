// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0

// Package aftsprefixfiltering implements AFT-6.1:
// AFT Prefix Filtering.
package aftsprefixfiltering_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	aftpf "github.com/openconfig/featureprofiles/internal/aft_prefix_filtering"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/telemetry/aftcache"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	v4PfxSetA            = "PREFIX-SET-A"
	v6PfxSetB            = "PREFIX-SET-B"
	vrfPfxSetA           = "PREFIX-SET-VRF-A"
	subnetPfxSetV4       = "PREFIX-SET-SUBNET"
	subnetPfxSetV6       = "PREFIX-SET-SUBNET-V6"
	policyPfxSetA        = "POLICY-PREFIX-SET-A"
	policyPfxSetB        = "POLICY-PREFIX-SET-B"
	policyVrfA           = "POLICY-PREFIX-SET-VRF-A"
	policySubnet         = "POLICY-SUBNET"
	policySubnetV6       = "POLICY-SUBNET-V6"
	policyMultiStmt      = "POLICY-MULTI-STMT"
	policyDenyPfxSetA    = "POLICY-DENY-PREFIX-SET-A"
	policyTagMatch       = "POLICY-TAG-MATCH"
	policyNotYetExist    = "POLICY-DOES-NOT-YET-EXIST"
	pfxSetNotYetExist    = "PREFIX-SET-DOES-NOT-YET-EXIST"
	secondStatementName  = "20"
	tagStatementName     = "100"
	notificationWaitTime = 30 * time.Second
	policyMatchAll       = aftpf.PolicyMatchAll
	defaultStatementName = aftpf.DefaultStatementName
	pfxMode              = aftpf.PfxMode
	subscriptionWait     = aftpf.AFTSubscriptionWait
	staticRouteIndex     = aftpf.StaticRouteIndex
)

var (
	atePort1           = aftpf.ATEPort1
	dialGNMI           = aftpf.DialGNMI
	deleteGlobalFilter = aftpf.AFTDeleteGlobalFilter
	baseIPv4Prefixes   = []string{
		"198.51.100.0/24",
		"203.0.113.0/28",
		"100.64.0.0/24",
	}
	baseIPv6Prefixes = []string{
		"2001:db8:1::/64",
		"2001:db8:2::/64",
		"2001:db8:3::/64",
	}
	pfxSetAMembers = []string{
		"198.51.100.0/24",
		"203.0.113.0/28",
		"198.51.100.1/32",
		"0.0.0.0/0",
	}
	pfxSetBMembers = []string{
		"2001:db8:2::/64",
		"2001:db8:2::1/128",
		"0::0/0",
	}
)

// configurePolicies configures the routing policies and prefix-sets required by the AFT-6.1 test procedures
func configurePolicies(t *testing.T, dut *ondatra.DUTDevice, batch *gnmi.SetBatch) {
	t.Helper()
	root := &oc.Root{}
	rp := root.GetOrCreateRoutingPolicy()
	cfgplugins.AddPrefixSetPolicy(t, rp, cfgplugins.PrefixSetPolicyParams{
		PolicyName:     policyMatchAll,
		StatementNames: []string{defaultStatementName},
		PolicyResult:   oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
	})
	cfgplugins.AddPrefixSetPolicyWithMatch(t, rp, cfgplugins.PrefixSetPolicyParams{
		PolicyName:     policyPfxSetA,
		StatementNames: []string{defaultStatementName},
		PrefixSetNames: []string{v4PfxSetA},
		PrefixList:     pfxSetAMembers,
		PrefixMode:     pfxMode,
		PolicyResult:   oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
	})
	cfgplugins.AddPrefixSetPolicyWithMatch(t, rp, cfgplugins.PrefixSetPolicyParams{
		PolicyName:     policyPfxSetB,
		StatementNames: []string{defaultStatementName},
		PrefixSetNames: []string{v6PfxSetB},
		PrefixList:     pfxSetBMembers,
		PrefixMode:     pfxMode,
		PolicyResult:   oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
	})
	cfgplugins.AddPrefixSetPolicyWithMatch(t, rp, cfgplugins.PrefixSetPolicyParams{
		PolicyName:     policyVrfA,
		StatementNames: []string{defaultStatementName},
		PrefixSetNames: []string{vrfPfxSetA},
		PrefixList:     []string{"100.64.1.0/24"},
		PrefixMode:     "24..32",
		PolicyResult:   oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
	})
	cfgplugins.AddPrefixSetPolicyWithMatch(t, rp, cfgplugins.PrefixSetPolicyParams{
		PolicyName:     policySubnet,
		StatementNames: []string{defaultStatementName},
		PrefixSetNames: []string{subnetPfxSetV4},
		PrefixList:     []string{"203.0.113.0/24"},
		PrefixMode:     "25..32",
		PolicyResult:   oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
	})
	cfgplugins.AddPrefixSetPolicyWithMatch(t, rp, cfgplugins.PrefixSetPolicyParams{
		PolicyName:     policySubnetV6,
		StatementNames: []string{defaultStatementName},
		PrefixSetNames: []string{subnetPfxSetV6},
		PrefixList:     []string{"2001:db8:3::/64"},
		PrefixMode:     "65..128",
		PolicyResult:   oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
	})
	cfgplugins.AddPrefixSetPolicy(t, rp, cfgplugins.PrefixSetPolicyParams{
		PolicyName:     policyMultiStmt,
		StatementNames: []string{defaultStatementName, secondStatementName},
		PrefixSetNames: []string{v4PfxSetA, subnetPfxSetV4},
		MatchPrefixSet: true,
		PolicyResult:   oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
	})
	cfgplugins.AddPrefixSetPolicy(t, rp, cfgplugins.PrefixSetPolicyParams{
		PolicyName:     policyDenyPfxSetA,
		StatementNames: []string{defaultStatementName, secondStatementName},
		PrefixSetNames: []string{v4PfxSetA, ""},
		MatchPrefixSet: true,
		PrefixDeny:     true,
		PolicyResult:   oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
	})
	cfgplugins.AddPrefixSetPolicy(t, rp, cfgplugins.PrefixSetPolicyParams{
		PolicyName:     policyTagMatch,
		StatementNames: []string{tagStatementName},
		MatchPrefixSet: true,
		SetTag:         true,
		PolicyResult:   oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
	})
	gnmi.BatchReplace(batch, gnmi.OC().RoutingPolicy().Config(), rp)
	batch.Set(t, dut)
}

// addSingleStaticRoute adds one static route to the given network instance.
// batch.Set fails the test directly on error, so there is no error to
// propagate to the caller.
func addSingleStaticRoute(t *testing.T, dut *ondatra.DUTDevice, niName, prefix, index, nextHop string) {
	t.Helper()
	batch := &gnmi.SetBatch{}
	cfgplugins.ConfigureStaticRoute(t, dut, batch, cfgplugins.ConfigureStaticRouteParams{
		NetworkInstance: niName,
		Prefix:          prefix,
		Index:           index,
		NextHop:         nextHop,
	})
	batch.Set(t, dut)
}

// removeStaticRoute deletes a static route (keyed by prefix) from the given
// network instance. gnmi.Delete fails the test directly on error, so there
// is no error to propagate to the caller.
func removeStaticRoute(t *testing.T, dut *ondatra.DUTDevice, niName, prefix string) {
	t.Helper()
	gnmi.Delete(t, dut, gnmi.OC().NetworkInstance(niName).
		Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut)).
		Static(prefix).Config())
}

// policyDefinitionPath builds the gNMI path for a policy-definition list entry.
// TODO: replace this raw gNMI path with the ygot-generated OC path once the
// OC path is supported.
func policyDefinitionPath(policyName string) *gpb.Path {
	return &gpb.Path{
		Elem: []*gpb.PathElem{
			{Name: "routing-policy"},
			{Name: "policy-definitions"},
			{Name: "policy-definition", Key: map[string]string{"name": policyName}},
		},
	}
}

// statementPrefixSetConfigPath builds the gNMI path for the match-prefix-set
// prefix-set leaf of a policy statement.
// TODO: replace this raw gNMI path with the ygot-generated OC path once the
// OC path is supported.
func statementPrefixSetConfigPath(policyName, statementName string) *gpb.Path {
	return &gpb.Path{
		Elem: []*gpb.PathElem{
			{Name: "routing-policy"},
			{Name: "policy-definitions"},
			{Name: "policy-definition", Key: map[string]string{"name": policyName}},
			{Name: "statements"},
			{Name: "statement", Key: map[string]string{"name": statementName}},
			{Name: "conditions"},
			{Name: "match-prefix-set"},
			{Name: "config"},
			{Name: "prefix-set"},
		},
	}
}

// setGlobalFilterExpectCode sets the global-filter policy leaves and returns an
// error unless the gNMI Set fails with exactly the given status code.
func setGlobalFilterExpectCode(t *testing.T, dut *ondatra.DUTDevice, niName, v4Policy, v6Policy string,
	wantCode codes.Code) error {
	t.Helper()
	if deviations.AftsGlobalFilterPolicyOCUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			t.Log("Skipping AFT global-filter negative check: unsupported on EOS")
			return nil
		}
	}

	// For vendors that support the OpenConfig afts/global-filter augment
	// (openconfig-aft-global-filter.yang, models 3.3.0), set the filter leaves
	// through the typed OC path API and assert the Set fails with wantCode. The
	// generated ondatra `oc` bindings do not yet contain the GlobalFilter
	// container, so the calls are commented out; uncomment them once the
	// bindings are regenerated against openconfig/public >= 3.3.0.
	// gf := gnmi.OC().NetworkInstance(niName).Afts().GlobalFilter()
	// batch := &gnmi.SetBatch{}
	// if v4Policy != "" {
	// 	gnmi.BatchUpdate(batch, gf.Ipv4Policy().Config(), v4Policy)
	// }
	// if v6Policy != "" {
	// 	gnmi.BatchUpdate(batch, gf.Ipv6Policy().Config(), v6Policy)
	// }
	// // Requires a non-fatal Set variant to inspect the returned status code
	// // against wantCode instead of failing the test on RPC error.
	// return nil
	return fmt.Errorf("aft global filter policy is expected to be supported on %s, "+
		"but no OpenConfig implementation is available", dut.Vendor())
}

// deletePolicyExpectCode attempts to delete a policy-definition and returns
// an error unless the gNMI Set RPC fails with exactly the given status code.
func deletePolicyExpectCode(t *testing.T, dut *ondatra.DUTDevice, policyName string, wantCode codes.Code) error {
	t.Helper()
	client, err := dialGNMI(t, dut)
	if err != nil {
		return err
	}
	req := &gpb.SetRequest{Delete: []*gpb.Path{policyDefinitionPath(policyName)}}
	_, setErr := client.Set(context.Background(), req)
	if setErr == nil {
		return fmt.Errorf("gNMI Set deleting policy %s succeeded, want error with code %v", policyName, wantCode)
	}
	if got := status.Code(setErr); got != wantCode {
		return fmt.Errorf("unexpected gNMI Set error code: got %v, want %v (err: %v)", got, wantCode, setErr)
	}
	t.Logf("Received expected %v error deleting policy %s: %v", wantCode, policyName, setErr)
	return nil
}

// deletePolicy deletes a policy-definition using the ygot-generated OC path.
func deletePolicy(t *testing.T, dut *ondatra.DUTDevice, policyName string) error {
	t.Helper()
	gnmi.Delete(t, dut, gnmi.OC().RoutingPolicy().PolicyDefinition(policyName).Config())
	return nil
}

// deleteGlobalFilterAndPolicyAtomic deletes the global-filter and a
// policy-definition in a single atomic gNMI Set request.
func deleteGlobalFilterAndPolicyAtomic(t *testing.T, dut *ondatra.DUTDevice, niName, policyName string) error {
	t.Helper()
	if deviations.AftsGlobalFilterPolicyOCUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			t.Log("Skipping atomic AFT global-filter + policy delete: unsupported on EOS")
			return nil
		}
	}

	// For vendors that support the OpenConfig afts/global-filter augment
	// (openconfig-aft-global-filter.yang, models 3.3.0), delete the filter
	// leaves and the policy-definition atomically through the typed OC path
	// API. The generated ondatra `oc` bindings do not yet contain the
	// GlobalFilter container, so the calls are commented out; uncomment them
	// once the bindings are regenerated against openconfig/public >= 3.3.0.
	// batch := &gnmi.SetBatch{}
	// gnmi.BatchDelete(batch, gnmi.OC().NetworkInstance(niName).Afts().GlobalFilter().Ipv4Policy().Config())
	// gnmi.BatchDelete(batch, gnmi.OC().NetworkInstance(niName).Afts().GlobalFilter().Ipv6Policy().Config())
	// gnmi.BatchDelete(batch, gnmi.OC().RoutingPolicy().PolicyDefinition(policyName).Config())
	// batch.Set(t, dut)
	// return nil
	return fmt.Errorf("aft global filter deletion is expected to be supported on %s, "+
		"but no OpenConfig implementation is available", dut.Vendor())
}

// updatePolicyStatementPrefixSet replaces the prefix-set referenced by a
// policy statement's match-prefix-set condition.
func updatePolicyStatementPrefixSet(t *testing.T, dut *ondatra.DUTDevice, policyName, statementName,
	newPrefixSet string) error {
	t.Helper()
	client, err := dialGNMI(t, dut)
	if err != nil {
		return err
	}
	req := &gpb.SetRequest{Update: []*gpb.Update{
		{
			Path: statementPrefixSetConfigPath(policyName, statementName),
			Val:  &gpb.TypedValue{Value: &gpb.TypedValue_StringVal{StringVal: newPrefixSet}},
		},
	}}
	if _, err := client.Set(context.Background(), req); err != nil {
		return fmt.Errorf("failed to update policy %s statement %s prefix-set to %s: %w",
			policyName, statementName, newPrefixSet, err)
	}
	return nil
}

// rawAFTStream is a raw gNMI STREAM subscription to afts/global-filter/state,
// used to observe the delete notification emitted when the filter is removed.
type rawAFTStream struct {
	mu     sync.Mutex
	notifs []*gpb.Notification
}

// newRawAFTStream opens a raw STREAM ON_CHANGE subscription on
// afts/global-filter/state, reading notifications in the background.
// TODO: replace this raw gNMI subscription with the ygot-generated OC path
// once the afts/global-filter schema is supported in featureprofiles.
func newRawAFTStream(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, niName string) (*rawAFTStream, error) {
	t.Helper()
	client, err := dialGNMI(t, dut)
	if err != nil {
		return nil, err
	}
	sub, err := client.Subscribe(ctx)
	if err != nil {
		return nil, fmt.Errorf("subscribe() failed: %w", err)
	}
	req := &gpb.SubscribeRequest{Request: &gpb.SubscribeRequest_Subscribe{Subscribe: &gpb.SubscriptionList{
		Mode:     gpb.SubscriptionList_STREAM,
		Encoding: gpb.Encoding_PROTO,
		Prefix: &gpb.Path{
			Origin: "openconfig",
			Target: dut.Name(),
			Elem: []*gpb.PathElem{
				{Name: "network-instances"},
				{Name: "network-instance", Key: map[string]string{"name": niName}},
				{Name: "afts"},
			},
		},
		Subscription: []*gpb.Subscription{{
			Path: &gpb.Path{Elem: []*gpb.PathElem{{Name: "global-filter"}, {Name: "state"}}},
			Mode: gpb.SubscriptionMode_ON_CHANGE,
		}},
	}}}
	if err := sub.Send(req); err != nil {
		return nil, fmt.Errorf("sending subscribe request failed: %w", err)
	}
	rs := &rawAFTStream{}
	go func() {
		for {
			resp, err := sub.Recv()
			if err != nil {
				return
			}
			if n := resp.GetUpdate(); n != nil {
				rs.mu.Lock()
				rs.notifs = append(rs.notifs, n)
				rs.mu.Unlock()
			}
		}
	}()
	return rs, nil
}

// awaitStateDelete blocks until a delete notification is observed for the
// given global-filter/state policy leaf, or until timeout.
func (rs *rawAFTStream) awaitStateDelete(timeout time.Duration, policyLeaf string) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if rs.hasStateDelete(policyLeaf) {
			return true
		}
		time.Sleep(time.Second)
	}
	return false
}

// hasStateDelete reports whether a delete notification referencing the
// global-filter subtree deleted the named policy leaf.
func (rs *rawAFTStream) hasStateDelete(policyLeaf string) bool {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	for _, n := range rs.notifs {
		if !notificationHasName(n, "global-filter") {
			continue
		}
		for _, d := range n.GetDelete() {
			for _, e := range d.GetElem() {
				if e.GetName() == policyLeaf {
					return true
				}
			}
		}
	}
	return false
}

// notificationHasName reports whether the given path element name appears
// anywhere in the notification's prefix, update paths, or delete paths.
func notificationHasName(n *gpb.Notification, name string) bool {
	for _, e := range n.GetPrefix().GetElem() {
		if e.GetName() == name {
			return true
		}
	}
	for _, u := range n.GetUpdate() {
		for _, e := range u.GetPath().GetElem() {
			if e.GetName() == name {
				return true
			}
		}
	}
	for _, d := range n.GetDelete() {
		for _, e := range d.GetElem() {
			if e.GetName() == name {
				return true
			}
		}
	}
	return false
}

// verifyNexthopGroupsAndNexthopsRetained confirms the next-hop-group and
// next-hops backing a prefix are still present in the filtered AFT.
func verifyNexthopGroupsAndNexthopsRetained(aft *aftcache.AFTData, prefix string) error {
	nhgID, ok := aft.Prefixes[prefix]
	if !ok {
		return fmt.Errorf("retained prefix %s missing from filtered AFT after dynamic delete", prefix)
	}
	nhg, ok := aft.NextHopGroups[nhgID]
	if !ok {
		return fmt.Errorf("next-hop-group %d for retained prefix %s was deleted, want retained", nhgID, prefix)
	}
	if len(nhg.NHIDs) == 0 {
		return fmt.Errorf("next-hop-group %d for retained prefix %s has no next-hops after dynamic delete", nhgID, prefix)
	}
	for _, nhID := range nhg.NHIDs {
		if _, ok := aft.NextHops[nhID]; !ok {
			return fmt.Errorf("next-hop %d shared by retained prefix %s was deleted, want retained", nhID, prefix)
		}
	}
	return nil
}

// runPrefixSetIteration runs a single AFT-6.1.1 iteration for one address
// family: validate the filtered view, dynamic updates, then filter removal.
func runPrefixSetIteration(t *testing.T, dut *ondatra.DUTDevice, tc prefixSetIterationCase, iterIdx int) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ni := deviations.DefaultNetworkInstance(dut)

	if tc.extraRoute != "" {
		idx := fmt.Sprintf("%d", staticRouteIndex+500+iterIdx)
		addSingleStaticRoute(t, dut, ni, tc.extraRoute, idx, tc.nextHop)
	}

	v4Policy, v6Policy := "", ""
	if tc.ipv4 {
		v4Policy = tc.policyName
	} else {
		v6Policy = tc.policyName
	}
	aftpf.ConfigureGlobalFilterPolicies(t, dut, aftpf.ConfigureGlobalFilterPoliciesParams{
		V4Policy: v4Policy,
		V6Policy: v6Policy,
		VRFName:  ni,
	})

	rawStream, rawErr := newRawAFTStream(ctx, t, dut, ni)
	if rawErr != nil {
		t.Errorf("%s: failed to open raw AFT subscription: %v", tc.name, rawErr)
	}

	wantPrefixes := make(map[string]bool)
	for _, p := range tc.matchPrefixes {
		wantPrefixes[p] = true
	}

	t.Logf("%s - Initial Synchronization", tc.name)

	collector := aftcache.NewAFTStreamSession(ctx, t, aftpf.GnmiClientSession(t, dut, aftpf.PrefixesParams{Ctx: ctx}), dut)
	collector.ListenUntilPreUpdateHook(context.Background(), t, subscriptionWait,
		[]aftcache.NotificationHook{aftcache.VerifyAtomicFlagHook(t)},
		aftcache.InitialSyncStoppingCondition(t, dut, wantPrefixes,
			map[string]bool{atePort1.IPv4: true}, map[string]bool{atePort1.IPv6: true}))
	initialAFT, err := collector.ToAFT(t, dut)
	if err != nil {
		t.Errorf("%s: ToAFT failed: %v", tc.name, err)
	} else {
		aftpf.VerifyPrefixesPresent(t, aftpf.PrefixesParams{InfoAFT: initialAFT, Prefixes: tc.matchPrefixes})
		aftpf.VerifyPrefixesAbsent(t, aftpf.PrefixesParams{InfoAFT: initialAFT, Prefixes: []string{tc.nonMatchPrefix}})
	}

	if tc.dynamicAddPrefix != "" {
		t.Logf("%s - Validate Dynamic Updates", tc.name)

		dynV4NH, dynV6NH := map[string]bool{}, map[string]bool{}
		if tc.ipv4 {
			dynV4NH = map[string]bool{atePort1.IPv4: true}
		} else {
			dynV6NH = map[string]bool{atePort1.IPv6: true}
		}

		dynIdx := fmt.Sprintf("%d", staticRouteIndex+600+iterIdx)
		addSingleStaticRoute(t, dut, ni, tc.dynamicAddPrefix, dynIdx, tc.nextHop)

		// Keep watching the same streaming session so the dynamic add is
		// observed as an incremental notification on the existing stream.
		aftpf.CollectAndVerify(t, dut, aftpf.RunCollectorParams{
			Ctx:       context.Background(),
			Collector: collector,
			Stop: aftcache.InitialSyncStoppingCondition(t, dut, map[string]bool{tc.dynamicAddPrefix: true},
				dynV4NH, dynV6NH),
			Timeout: subscriptionWait,
		}, []string{tc.dynamicAddPrefix}, nil)

		removeStaticRoute(t, dut, ni, tc.dynamicAddPrefix)

		// Same session again: the delete must be seen as a streamed
		// deletion notification, not by re-subscribing after the fact.
		delAFT := aftpf.CollectAndVerify(t, dut, aftpf.RunCollectorParams{
			Ctx:       context.Background(),
			Collector: collector,
			Stop:      aftcache.DeletionStoppingCondition(t, dut, map[string]bool{tc.dynamicAddPrefix: true}),
			Timeout:   subscriptionWait,
		}, nil, []string{tc.dynamicAddPrefix})
		if delAFT != nil && len(tc.matchPrefixes) > 0 {
			if err := verifyNexthopGroupsAndNexthopsRetained(delAFT, tc.matchPrefixes[0]); err != nil {
				t.Errorf("%s: %v", tc.name, err)
			}
		}

		if tc.dynamicNonMatchPrefix != "" {
			nmIdx := fmt.Sprintf("%d", staticRouteIndex+650+iterIdx)
			addSingleStaticRoute(t, dut, ni, tc.dynamicNonMatchPrefix, nmIdx, tc.nextHop)

			// Drain the existing stream and confirm the non-matching prefix
			// never surfaces; it must stay absent from the filtered view.
			aftpf.CollectAndVerify(t, dut, aftpf.RunCollectorParams{
				Ctx:       context.Background(),
				Collector: collector,
				Stop:      aftcache.DeletionStoppingCondition(t, dut, map[string]bool{tc.dynamicNonMatchPrefix: true}),
				Timeout:   notificationWaitTime,
			}, nil, []string{tc.dynamicNonMatchPrefix})

			removeStaticRoute(t, dut, ni, tc.dynamicNonMatchPrefix)
		}
	}

	t.Logf("%s - Remove the Filtered View", tc.name)

	if err := deleteGlobalFilter(t, dut, ni); err != nil {
		t.Errorf("%s: failed to delete global-filter: %v", tc.name, err)
		return
	}

	if rawStream != nil {
		wantDeleteLeaf := "ipv4-policy"
		if !tc.ipv4 {
			wantDeleteLeaf = "ipv6-policy"
		}
		if !rawStream.awaitStateDelete(subscriptionWait, wantDeleteLeaf) {
			t.Errorf("%s: did not receive global-filter/state/%s delete notification after removing the filter",
				tc.name, wantDeleteLeaf)
		}
	}

	liftWant := map[string]bool{tc.nonMatchPrefix: true}
	// The filter removal is validated on the same stream: the previously
	// filtered-out prefix must now appear as a streamed update.
	aftpf.CollectAndVerify(t, dut, aftpf.RunCollectorParams{
		Ctx:       context.Background(),
		Collector: collector,
		Stop: aftcache.InitialSyncStoppingCondition(t, dut, liftWant,
			map[string]bool{atePort1.IPv4: true}, map[string]bool{atePort1.IPv6: true}),
		Timeout: subscriptionWait,
	}, []string{tc.nonMatchPrefix}, nil)

	if tc.extraRoute != "" {
		removeStaticRoute(t, dut, ni, tc.extraRoute)
	}
}

// prefixSetIterationCase parameterizes a single AFT-6.1.1 iteration.
type prefixSetIterationCase struct {
	name                  string
	policyName            string
	ipv4                  bool
	nextHop               string
	matchPrefixes         []string
	nonMatchPrefix        string
	extraRoute            string // installed before subscribing, if not already part of the base set
	dynamicAddPrefix      string // added mid-subscription to validate dynamic updates
	dynamicNonMatchPrefix string // added mid-subscription; must never surface
}

// testPrefixSetPolicySubscription implements AFT-6.1.1, iterated across
// address families and prefix-set/subnet policies.
func testPrefixSetPolicySubscription(t *testing.T, dut *ondatra.DUTDevice) {
	cases := []prefixSetIterationCase{
		{
			name:                  "IPv4-POLICY-PREFIX-SET-A",
			policyName:            policyPfxSetA,
			ipv4:                  true,
			nextHop:               atePort1.IPv4,
			matchPrefixes:         []string{"198.51.100.0/24", "203.0.113.0/28"},
			nonMatchPrefix:        "100.64.0.0/24",
			dynamicAddPrefix:      "198.51.100.1/32",
			dynamicNonMatchPrefix: "100.64.1.0/24",
		},
		{
			name:                  "IPv6-POLICY-PREFIX-SET-B",
			policyName:            policyPfxSetB,
			ipv4:                  false,
			nextHop:               atePort1.IPv6,
			matchPrefixes:         []string{"2001:db8:2::/64"},
			nonMatchPrefix:        "2001:db8:1::/64",
			dynamicAddPrefix:      "2001:db8:2::1/128",
			dynamicNonMatchPrefix: "2001:db8:4::/64",
		},
		{
			name:           "IPv4-POLICY-SUBNET",
			policyName:     policySubnet,
			ipv4:           true,
			nextHop:        atePort1.IPv4,
			matchPrefixes:  []string{"203.0.113.0/28"},
			nonMatchPrefix: "100.64.0.0/24",
		},
		{
			name:           "IPv6-POLICY-SUBNET-V6",
			policyName:     policySubnetV6,
			ipv4:           false,
			nextHop:        atePort1.IPv6,
			matchPrefixes:  []string{"2001:db8:3::/65"},
			nonMatchPrefix: "2001:db8:1::/64",
			extraRoute:     "2001:db8:3::/65",
		},
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runPrefixSetIteration(t, dut, tc, i)
		})
	}
}

// testNonExistentPolicy implements AFT-6.1.2.
func testNonExistentPolicy(t *testing.T, dut *ondatra.DUTDevice) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ni := deviations.DefaultNetworkInstance(dut)
	matchPrefix := "198.51.100.128/25"

	if err := setGlobalFilterExpectCode(t, dut, ni, policyNotYetExist, policyNotYetExist,
		codes.FailedPrecondition); err != nil {
		t.Errorf("Non-existent policy check: %v", err)
	}

	root := &oc.Root{}
	rp := root.GetOrCreateRoutingPolicy()
	cfgplugins.AddPrefixSetPolicyWithMatch(t, rp, cfgplugins.PrefixSetPolicyParams{
		PolicyName:     policyNotYetExist,
		StatementNames: []string{defaultStatementName},
		PrefixSetNames: []string{pfxSetNotYetExist},
		PrefixList:     []string{matchPrefix},
		PrefixMode:     pfxMode,
		PolicyResult:   oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
	})
	batch := &gnmi.SetBatch{}
	gnmi.BatchUpdate(batch, gnmi.OC().RoutingPolicy().Config(), rp)
	batch.Set(t, dut)

	mpIdx := fmt.Sprintf("%d", staticRouteIndex+700)
	addSingleStaticRoute(t, dut, ni, matchPrefix, mpIdx, atePort1.IPv4)

	aftpf.ConfigureGlobalFilterPolicies(t, dut, aftpf.ConfigureGlobalFilterPoliciesParams{
		V4Policy: policyNotYetExist,
		V6Policy: "",
		VRFName:  ni,
	})

	wantPrefixes := map[string]bool{matchPrefix: true}
	collector := aftcache.NewAFTStreamSession(ctx, t, aftpf.GnmiClientSession(t, dut, aftpf.PrefixesParams{Ctx: ctx}), dut)
	aftpf.CollectAndVerify(t, dut, aftpf.RunCollectorParams{
		Ctx:       context.Background(),
		Collector: collector,
		Stop: aftcache.InitialSyncStoppingCondition(t, dut, wantPrefixes,
			map[string]bool{atePort1.IPv4: true}, map[string]bool{atePort1.IPv6: true}),
		Timeout: subscriptionWait,
	}, []string{matchPrefix}, []string{"198.51.100.0/24", "100.64.0.0/24"})

	if err := deleteGlobalFilter(t, dut, ni); err != nil {
		t.Errorf("Cleanup: failed to delete global-filter: %v", err)
	}
	removeStaticRoute(t, dut, ni, matchPrefix)
	if err := deletePolicy(t, dut, policyNotYetExist); err != nil {
		t.Errorf("Cleanup: failed to delete policy %s: %v", policyNotYetExist, err)
	}
}

// testPolicyDeletion implements AFT-6.1.3.
func testPolicyDeletion(t *testing.T, dut *ondatra.DUTDevice) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ni := deviations.DefaultNetworkInstance(dut)
	wantPrefixes := map[string]bool{"198.51.100.0/24": true, "203.0.113.0/28": true}
	liftWant := map[string]bool{"100.64.0.0/24": true}

	aftpf.ConfigureGlobalFilterPolicies(t, dut, aftpf.ConfigureGlobalFilterPoliciesParams{
		V4Policy: policyPfxSetA,
		V6Policy: "",
		VRFName:  ni,
	})

	collector := aftcache.NewAFTStreamSession(ctx, t, aftpf.GnmiClientSession(t, dut, aftpf.PrefixesParams{Ctx: ctx}), dut)
	aftpf.CollectAndVerify(t, dut, aftpf.RunCollectorParams{
		Ctx:       context.Background(),
		Collector: collector,
		Stop: aftcache.InitialSyncStoppingCondition(t, dut, wantPrefixes,
			map[string]bool{atePort1.IPv4: true}, map[string]bool{atePort1.IPv6: true}),
		Timeout: subscriptionWait,
	}, nil, nil)

	if err := deletePolicyExpectCode(t, dut, policyPfxSetA, codes.FailedPrecondition); err != nil {
		t.Errorf("Policy-still-referenced check: %v", err)
	}

	if err := deleteGlobalFilterAndPolicyAtomic(t, dut, ni, policyPfxSetA); err != nil {
		t.Errorf("Atomic delete failed: %v", err)
		return
	}

	// Same session observes the atomic filter+policy delete as a streamed change.
	aftpf.CollectAndVerify(t, dut, aftpf.RunCollectorParams{
		Ctx:       context.Background(),
		Collector: collector,
		Stop: aftcache.InitialSyncStoppingCondition(t, dut, liftWant,
			map[string]bool{atePort1.IPv4: true}, map[string]bool{atePort1.IPv6: true}),
		Timeout: subscriptionWait,
	}, []string{"100.64.0.0/24"}, nil)

	root := &oc.Root{}
	rp := root.GetOrCreateRoutingPolicy()
	cfgplugins.AddPrefixSetPolicyWithMatch(t, rp, cfgplugins.PrefixSetPolicyParams{
		PolicyName:     policyPfxSetA,
		StatementNames: []string{defaultStatementName},
		PrefixSetNames: []string{v4PfxSetA},
		PrefixList:     pfxSetAMembers,
		PrefixMode:     pfxMode,
		PolicyResult:   oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
	})
	batch := &gnmi.SetBatch{}
	gnmi.BatchUpdate(batch, gnmi.OC().RoutingPolicy().Config(), rp)
	batch.Set(t, dut)
	aftpf.ConfigureGlobalFilterPolicies(t, dut, aftpf.ConfigureGlobalFilterPoliciesParams{
		V4Policy: policyPfxSetA,
		V6Policy: "",
		VRFName:  ni,
	})

	// Same session observes the policy re-configuration as streamed updates.
	aftpf.CollectAndVerify(t, dut, aftpf.RunCollectorParams{
		Ctx:       context.Background(),
		Collector: collector,
		Stop: aftcache.InitialSyncStoppingCondition(t, dut, wantPrefixes,
			map[string]bool{atePort1.IPv4: true}, map[string]bool{atePort1.IPv6: true}),
		Timeout: subscriptionWait,
	}, []string{"198.51.100.0/24", "203.0.113.0/28"}, []string{"100.64.0.0/24"})

	if err := deleteGlobalFilter(t, dut, ni); err != nil {
		t.Errorf("Multi-step delete: failed to delete global-filter: %v", err)
	}
	if err := deletePolicy(t, dut, policyPfxSetA); err != nil {
		t.Errorf("Multi-step delete: failed to delete policy: %v", err)
	}

	// Same session observes the multi-step filter+policy delete as streamed deletions.
	aftpf.CollectAndVerify(t, dut, aftpf.RunCollectorParams{
		Ctx:       context.Background(),
		Collector: collector,
		Stop: aftcache.InitialSyncStoppingCondition(t, dut, liftWant,
			map[string]bool{atePort1.IPv4: true}, map[string]bool{atePort1.IPv6: true}),
		Timeout: subscriptionWait,
	}, []string{"100.64.0.0/24"}, nil)

	batch2 := &gnmi.SetBatch{}
	gnmi.BatchUpdate(batch2, gnmi.OC().RoutingPolicy().Config(), rp)
	batch2.Set(t, dut)
}

// testChangeReferencedPrefixSet implements AFT-6.1.4.
func testChangeReferencedPrefixSet(t *testing.T, dut *ondatra.DUTDevice) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ni := deviations.DefaultNetworkInstance(dut)

	aftpf.ConfigureGlobalFilterPolicies(t, dut, aftpf.ConfigureGlobalFilterPoliciesParams{
		V4Policy: policyPfxSetA,
		V6Policy: "",
		VRFName:  ni,
	})

	wantPrefixes := map[string]bool{"198.51.100.0/24": true, "203.0.113.0/28": true}
	collector := aftcache.NewAFTStreamSession(ctx, t, aftpf.GnmiClientSession(t, dut, aftpf.PrefixesParams{Ctx: ctx}), dut)
	aftpf.CollectAndVerify(t, dut, aftpf.RunCollectorParams{
		Ctx:       context.Background(),
		Collector: collector,
		Stop: aftcache.InitialSyncStoppingCondition(t, dut, wantPrefixes,
			map[string]bool{atePort1.IPv4: true}, map[string]bool{atePort1.IPv6: true}),
		Timeout: subscriptionWait,
	}, []string{"198.51.100.0/24", "203.0.113.0/28"}, nil)

	if err := updatePolicyStatementPrefixSet(t, dut, policyPfxSetA, defaultStatementName, v6PfxSetB); err != nil {
		t.Errorf("Failed to swap prefix-set reference: %v", err)
		return
	}

	// Same session observes the prefix-set swap as streamed deletions.
	aftpf.CollectAndVerify(t, dut, aftpf.RunCollectorParams{
		Ctx:       context.Background(),
		Collector: collector,
		Stop:      aftcache.DeletionStoppingCondition(t, dut, map[string]bool{"198.51.100.0/24": true}),
		Timeout:   subscriptionWait,
	}, nil, []string{"198.51.100.0/24", "203.0.113.0/28"})

	// The swap only produces delete notifications for PREFIX-SET-A's members;
	// PREFIX-SET-B's members were never installed in the AFT, so there is no
	// further positive event to wait on. Wait out a full notification window
	// and re-check the snapshot to catch any spurious re-add that a bug might
	// otherwise introduce after the deletion settles.
	time.Sleep(notificationWaitTime)
	finalAFT, err := collector.ToAFT(t, dut)
	if err != nil {
		t.Errorf("ToAFT failed on stability check: %v", err)
	} else {
		aftpf.VerifyPrefixesAbsent(t, aftpf.PrefixesParams{
			InfoAFT: finalAFT, Prefixes: []string{"198.51.100.0/24", "203.0.113.0/28"},
		})
	}

	if err := updatePolicyStatementPrefixSet(t, dut, policyPfxSetA, defaultStatementName, v4PfxSetA); err != nil {
		t.Errorf("Cleanup: failed to restore prefix-set reference: %v", err)
	}
	if err := deleteGlobalFilter(t, dut, ni); err != nil {
		t.Errorf("Cleanup: failed to delete global-filter: %v", err)
	}
}

// testMultiStatementPolicy implements AFT-6.1.5.
func testMultiStatementPolicy(t *testing.T, dut *ondatra.DUTDevice) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ni := deviations.DefaultNetworkInstance(dut)
	extraPrefix := "203.0.113.128/25"

	extraIdx := fmt.Sprintf("%d", staticRouteIndex+800)
	addSingleStaticRoute(t, dut, ni, extraPrefix, extraIdx, atePort1.IPv4)

	aftpf.ConfigureGlobalFilterPolicies(t, dut, aftpf.ConfigureGlobalFilterPoliciesParams{
		V4Policy: policyMultiStmt,
		V6Policy: "",
		VRFName:  ni,
	})

	wantPrefixes := map[string]bool{"198.51.100.0/24": true, "203.0.113.0/28": true, extraPrefix: true}
	collector := aftcache.NewAFTStreamSession(ctx, t, aftpf.GnmiClientSession(t, dut, aftpf.PrefixesParams{Ctx: ctx}), dut)
	aftpf.CollectAndVerify(t, dut, aftpf.RunCollectorParams{
		Ctx:       context.Background(),
		Collector: collector,
		Stop: aftcache.InitialSyncStoppingCondition(t, dut, wantPrefixes,
			map[string]bool{atePort1.IPv4: true}, map[string]bool{atePort1.IPv6: true}),
		Timeout: subscriptionWait,
	}, []string{"198.51.100.0/24", "203.0.113.0/28", extraPrefix}, []string{"100.64.0.0/24"})

	if err := deleteGlobalFilter(t, dut, ni); err != nil {
		t.Errorf("Cleanup: failed to delete global-filter: %v", err)
	}
	removeStaticRoute(t, dut, ni, extraPrefix)
}

// testDenyActionPolicy implements AFT-6.1.6.
func testDenyActionPolicy(t *testing.T, dut *ondatra.DUTDevice) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ni := deviations.DefaultNetworkInstance(dut)

	aftpf.ConfigureGlobalFilterPolicies(t, dut, aftpf.ConfigureGlobalFilterPoliciesParams{
		V4Policy: policyDenyPfxSetA,
		V6Policy: "",
		VRFName:  ni,
	})

	wantPrefixes := map[string]bool{"100.64.0.0/24": true}
	collector := aftcache.NewAFTStreamSession(ctx, t, aftpf.GnmiClientSession(t, dut, aftpf.PrefixesParams{Ctx: ctx}), dut)
	aftpf.CollectAndVerify(t, dut, aftpf.RunCollectorParams{
		Ctx:       context.Background(),
		Collector: collector,
		Stop: aftcache.InitialSyncStoppingCondition(t, dut, wantPrefixes,
			map[string]bool{atePort1.IPv4: true}, map[string]bool{atePort1.IPv6: true}),
		Timeout: subscriptionWait,
	}, []string{"100.64.0.0/24"}, []string{"198.51.100.0/24", "203.0.113.0/28"})

	if err := deleteGlobalFilter(t, dut, ni); err != nil {
		t.Errorf("Cleanup: failed to delete global-filter: %v", err)
	}
}

// testNonPrefixSetMatchCriteria implements AFT-6.1.7.
func testNonPrefixSetMatchCriteria(t *testing.T, dut *ondatra.DUTDevice) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ni := deviations.DefaultNetworkInstance(dut)

	aftpf.ConfigureGlobalFilterPolicies(t, dut, aftpf.ConfigureGlobalFilterPoliciesParams{
		V4Policy: policyTagMatch,
		V6Policy: "",
		VRFName:  ni,
	})

	collector := aftcache.NewAFTStreamSession(ctx, t, aftpf.GnmiClientSession(t, dut, aftpf.PrefixesParams{Ctx: ctx}), dut)
	aftpf.CollectAndVerify(t, dut, aftpf.RunCollectorParams{
		Ctx:       context.Background(),
		Collector: collector,
		Stop: aftcache.InitialSyncStoppingCondition(t, dut, map[string]bool{},
			map[string]bool{atePort1.IPv4: true}, map[string]bool{atePort1.IPv6: true}),
		Timeout: subscriptionWait,
	}, nil, []string{"198.51.100.0/24", "203.0.113.0/28", "100.64.0.0/24"})

	if err := deleteGlobalFilter(t, dut, ni); err != nil {
		t.Errorf("Failed to delete global-filter: %v", err)
		return
	}

	aftpf.ConfigureGlobalFilterPolicies(t, dut, aftpf.ConfigureGlobalFilterPoliciesParams{
		V4Policy: policyPfxSetA,
		V6Policy: "",
		VRFName:  ni,
	})

	// Same session observes the transition to the prefix-set policy as streamed updates.
	wantPrefixes := map[string]bool{"198.51.100.0/24": true, "203.0.113.0/28": true}
	aftpf.CollectAndVerify(t, dut, aftpf.RunCollectorParams{
		Ctx:       context.Background(),
		Collector: collector,
		Stop: aftcache.InitialSyncStoppingCondition(t, dut, wantPrefixes,
			map[string]bool{atePort1.IPv4: true}, map[string]bool{atePort1.IPv6: true}),
		Timeout: subscriptionWait,
	}, nil, nil)

	aftpf.ConfigureGlobalFilterPolicies(t, dut, aftpf.ConfigureGlobalFilterPoliciesParams{
		V4Policy: policyTagMatch,
		V6Policy: "",
		VRFName:  ni,
	})

	// Same session observes the transition back to tag-match as streamed deletions.
	aftpf.CollectAndVerify(t, dut, aftpf.RunCollectorParams{
		Ctx:       context.Background(),
		Collector: collector,
		Stop:      aftcache.DeletionStoppingCondition(t, dut, map[string]bool{"198.51.100.0/24": true}),
		Timeout:   subscriptionWait,
	}, nil, []string{"198.51.100.0/24", "203.0.113.0/28"})

	if err := deleteGlobalFilter(t, dut, ni); err != nil {
		t.Errorf("Cleanup: failed to delete global-filter: %v", err)
	}
}

// TestMain runs featureprofile tests.
func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// TestAFTPrefixFiltering implements AFT-6.1.
func TestAFTPrefixFiltering(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	if deviations.AftsGlobalFilterPolicyOCUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			t.Skipf("Skipping AFT-6.1 test validation: AFT global-filter policy is not supported on %s", dut.Vendor())
		}
	}

	ate := ondatra.ATE(t, "ate")
	ni := deviations.DefaultNetworkInstance(dut)

	if _, err := aftpf.DialGNMI(t, dut); err != nil {
		t.Fatalf("%v", err)
	}

	batch := aftpf.ConfigureDUT(t, dut)
	configurePolicies(t, dut, batch)
	prefixes := aftpf.ConfigureBaseRoutesParams{V4Prefixes: baseIPv4Prefixes, V6Prefixes: baseIPv6Prefixes}
	aftpf.ConfigureBaseRoutes(t, dut, batch, prefixes)
	d := &oc.Root{}
	defNI := d.GetOrCreateNetworkInstance(ni)
	aftpf.ConfigureBGP(t, dut, batch, defNI)
	batch.Set(t, dut)
	batch.Set(t, dut)
	aftpf.ApplyBGPMaxPrefixes(t, dut, aftpf.BGPPrefixParams{V4Prefix: aftpf.ATEPort1.IPv4,
		V6Prefix: aftpf.ATEPort2.IPv6, NetworkInstance: defNI})
	topo, interfaceNamesList := aftpf.ConfigureATE(t, ate)
	aftpf.ConfigureATEBGP(t, topo)
	ate.OTG().PushConfig(t, topo)
	ate.OTG().StartProtocols(t)
	cfgplugins.IsIPv4InterfaceARPresolved(t, ate, cfgplugins.AddressFamilyParams{InterfaceNames: interfaceNamesList})
	cfgplugins.IsIPv6InterfaceARPresolved(t, ate, cfgplugins.AddressFamilyParams{InterfaceNames: interfaceNamesList})

	aftpf.AwaitBGPConvergence(t, dut, ni)

	// AFT-6.1.1 is the only subtest iterated across both IPv4 and IPv6: the
	// README's Test Case Iteration section requires that subscribe/validate
	// cycle to be repeated per address family. AFT-6.1.2 through AFT-6.1.7
	// validate policy semantics (non-existent policy, deletion, prefix-set
	// swap, multi-statement, deny action, non-prefix-set match) that are
	// address-family agnostic and whose README procedures reference only
	// ipv4-policy, so they are exercised with IPv4 only; the per-address-family
	// path is already covered by AFT-6.1.1.
	tests := []struct {
		name string
		test func(t *testing.T, dut *ondatra.DUTDevice)
	}{
		{
			name: "aft-6.1.1-testPrefixSetPolicySubscription",
			test: testPrefixSetPolicySubscription,
		},
		{
			name: "aft-6.1.2-testNonExistentPolicy",
			test: testNonExistentPolicy,
		},
		{
			name: "aft-6.1.3-testPolicyDeletion",
			test: testPolicyDeletion,
		},
		{
			name: "aft-6.1.4-testChangeReferencedPrefixSet",
			test: testChangeReferencedPrefixSet,
		},
		{
			name: "aft-6.1.5-testMultiStatementPolicy",
			test: testMultiStatementPolicy,
		},
		{
			name: "aft-6.1.6-testDenyActionPolicy",
			test: testDenyActionPolicy,
		},
		{
			name: "aft-6.1.7-testNonPrefixSetMatchCriteria",
			test: testNonPrefixSetMatchCriteria,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.test(t, dut)
		})
	}
}
