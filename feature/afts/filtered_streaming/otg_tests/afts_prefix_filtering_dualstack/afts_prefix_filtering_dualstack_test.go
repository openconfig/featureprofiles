// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0

// Package aftsprefixfilteringdualstack implements AFT-6.2:
// AFT Prefix Filtering Dual-Stack.
//
// The shared two-port DUT/ATE BGP topology, bulk-route scale, and raw
// afts/global-filter gNMI helpers live in the cfgplugins package
// (aft_prefix_filtering.go) and are reused across the AFT-6.x tests.
package aftsprefixfilteringdualstack_test

import (
	"context"
	"testing"

	aftpf "github.com/openconfig/featureprofiles/internal/aft_prefix_filtering"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/telemetry/aftcache"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
)

const (
	v4PfxSetA = "PREFIX-SET-A"
	v6PfxSetB = "PREFIX-SET-B"

	policyPfxSetA = "POLICY-PREFIX-SET-A"
	policyPfxSetB = "POLICY-PREFIX-SET-B"
)

var (
	baseIPv4Prefixes = []string{
		"198.51.100.0/24",
		"203.0.113.0/28",
		"100.64.0.0/24",
		"198.51.100.1/32",
	}
	baseIPv6Prefixes = []string{
		"2001:db8:1::/64",
		"2001:db8:2::/64",
		"2001:db8:3::/64",
		"2001:db8:2::1/128",
	}

	pfxSetAMembers = []string{
		"198.51.100.0/24",
		"203.0.113.0/28",
		"198.51.100.1/32",
	}
	pfxSetBMembers = []string{
		"2001:db8:2::/64",
		"2001:db8:2::1/128",
	}

	v4MatchPrefixes  = []string{"198.51.100.0/24", "203.0.113.0/28", "198.51.100.1/32"}
	v6MatchPrefixes  = []string{"2001:db8:2::/64", "2001:db8:2::1/128"}
	nonMatchPrefixes = []string{"100.64.0.0/24", "2001:db8:1::/64", "2001:db8:3::/64"}
)

// configurePolicies configures the routing policies and prefix-sets required
// by the AFT-6.2 test procedures.
func configurePolicies(t *testing.T, dut *ondatra.DUTDevice, batch *gnmi.SetBatch) {
	t.Helper()
	root := &oc.Root{}
	rp := root.GetOrCreateRoutingPolicy()
	cfgplugins.AddPrefixSetPolicy(t, rp, cfgplugins.PrefixSetPolicyParams{PolicyName: aftpf.AFTFilterPolicyMatchAll, StatementNames: []string{aftpf.AFTFilterDefaultStatementName}, PolicyResult: oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE})
	cfgplugins.AddPrefixSetPolicyWithMatch(t, rp, cfgplugins.PrefixSetPolicyParams{PolicyName: policyPfxSetA, StatementNames: []string{aftpf.AFTFilterDefaultStatementName}, PrefixSetNames: []string{v4PfxSetA}, PrefixList: pfxSetAMembers, PrefixMode: aftpf.AFTFilterPfxMode, PolicyResult: oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE})
	cfgplugins.AddPrefixSetPolicyWithMatch(t, rp, cfgplugins.PrefixSetPolicyParams{PolicyName: policyPfxSetB, StatementNames: []string{aftpf.AFTFilterDefaultStatementName}, PrefixSetNames: []string{v6PfxSetB}, PrefixList: pfxSetBMembers, PrefixMode: aftpf.AFTFilterPfxMode, PolicyResult: oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE})
	gnmi.BatchReplace(batch, gnmi.OC().RoutingPolicy().Config(), rp)
	batch.Set(t, dut)
}

// testSimultaneousDualStackPolicy implements AFT-6.2.1.
func testSimultaneousDualStackPolicy(t *testing.T, dut *ondatra.DUTDevice) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ni := deviations.DefaultNetworkInstance(dut)
	allMatch := append(append([]string{}, v4MatchPrefixes...), v6MatchPrefixes...)

	t.Logf("AFT-6.2.1 - Apply IPv4 policy %s and IPv6 policy %s simultaneously", policyPfxSetA, policyPfxSetB)
	cfgplugins.ConfigureGlobalFilterPolicies(t, dut, cfgplugins.ConfigureGlobalFilterPoliciesParams{V4Policy: policyPfxSetA, V6Policy: policyPfxSetB, VRFName: ni})

	wantPrefixes := map[string]bool{}
	for _, p := range allMatch {
		wantPrefixes[p] = true
	}
	collector := aftcache.NewAFTStreamSession(ctx, t, aftpf.GnmiClientSession(t, dut, aftpf.PrefixesParams{Ctx: ctx}), dut)
	aftpf.RunCollector(t, aftpf.RunCollectorParams{Ctx: context.Background(), Collector: collector, Stop: aftcache.InitialSyncStoppingCondition(t, dut, wantPrefixes, map[string]bool{aftpf.AFTFilterATEPort1.IPv4: true}, map[string]bool{aftpf.AFTFilterATEPort1.IPv6: true}), Timeout: aftpf.AFTFilterSubscriptionWait})
	aft, err := collector.ToAFT(t, dut)
	if err != nil {
		t.Errorf("ToAFT failed: %v", err)
	} else {
		aftpf.VerifyPrefixesPresent(t, aftpf.PrefixesParams{InfoAFT: aft, Prefixes: allMatch})
		aftpf.VerifyPrefixesAbsent(t, aftpf.PrefixesParams{InfoAFT: aft, Prefixes: nonMatchPrefixes})
	}

	t.Logf("AFT-6.2.1 - Swap policies: IPv4 policy %s and IPv6 policy %s match nothing", policyPfxSetB, policyPfxSetA)
	cfgplugins.ConfigureGlobalFilterPolicies(t, dut, cfgplugins.ConfigureGlobalFilterPoliciesParams{V4Policy: policyPfxSetB, V6Policy: policyPfxSetA, VRFName: ni})

	// Reuse the same streaming session so the policy swap is observed as
	// streamed deletions rather than a fresh re-subscription.
	aftpf.RunCollector(t, aftpf.RunCollectorParams{Ctx: context.Background(), Collector: collector, Stop: aftcache.DeletionStoppingCondition(t, dut, wantPrefixes), Timeout: aftpf.AFTFilterSubscriptionWait})
	swapAFT, err := collector.ToAFT(t, dut)
	if err != nil {
		t.Errorf("ToAFT failed after policy swap: %v", err)
	} else {
		aftpf.VerifyPrefixesAbsent(t, aftpf.PrefixesParams{InfoAFT: swapAFT, Prefixes: allMatch})
	}

	if err := aftpf.AFTFilterDeleteGlobalFilter(t, dut, ni); err != nil {
		t.Errorf("Cleanup: failed to delete global-filter: %v", err)
	}
}

// TestMain runs featureprofile tests.
func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// TestAFTPrefixFilteringDualStack implements AFT-6.2.
func TestAFTPrefixFilteringDualStack(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	ni := deviations.DefaultNetworkInstance(dut)

	if _, err := aftpf.AFTFilterDialGNMI(t, dut); err != nil {
		t.Fatalf("%v", err)
	}

	batch := aftpf.AFTFilterConfigureDUT(t, dut)
	configurePolicies(t, dut, batch)
	prefixes := aftpf.AFTFilterConfigureBaseRoutesParams{V4Prefixes: baseIPv4Prefixes, V6Prefixes: baseIPv6Prefixes}
	aftpf.AFTFilterConfigureBaseRoutes(t, dut, batch, prefixes)
	d := &oc.Root{}
	defNI := d.GetOrCreateNetworkInstance(ni)
	aftpf.AFTFilterConfigureBGP(t, dut, batch, defNI)
	batch.Set(t, dut)
	aftpf.AFTFilterApplyBGPMaxPrefixes(t, dut)
	topo, interfaceNamesList := aftpf.AFTFilterConfigureATE(t, ate)
	aftpf.AFTFilterConfigureATEBGP(t, topo)
	ate.OTG().PushConfig(t, topo)
	ate.OTG().StartProtocols(t)
	cfgplugins.IsIPv4InterfaceARPresolved(t, ate, cfgplugins.AddressFamilyParams{InterfaceNames: interfaceNamesList})
	cfgplugins.IsIPv6InterfaceARPresolved(t, ate, cfgplugins.AddressFamilyParams{InterfaceNames: interfaceNamesList})

	aftpf.AFTFilterAwaitBGPConvergence(t, dut, ni)

	t.Run("aft-6.2.1-testSimultaneousDualStackPolicy", func(t *testing.T) {
		testSimultaneousDualStackPolicy(t, dut)
	})
}
