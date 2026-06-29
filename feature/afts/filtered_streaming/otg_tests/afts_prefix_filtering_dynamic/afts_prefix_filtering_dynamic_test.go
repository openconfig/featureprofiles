// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0

// Package aftsprefixfilteringdynamic implements AFT-6.4:
// AFT Prefix Filtering Dynamic Updates.
package afts_prefix_filtering_dynamic_test

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
	"github.com/openconfig/featureprofiles/internal/telemetry/aftcache"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

const (
	vrfName              = "VRF-A"
	v4PfxSet             = "PREFIX-SET-A"
	v6PfxSet             = "PREFIX-SET-B"
	v4Policy             = "POLICY-PREFIX-SET-A"
	v6Policy             = "POLICY-PREFIX-SET-B"
	matchAllPolicy       = "POLICY-MATCH-ALL"
	subscriptionWait     = 3 * time.Minute
	prefixAFT1V4         = "198.51.100.0/24"
	prefixAFT2V4         = "203.0.113.0/28"
	prefixAFT3V4         = "192.0.2.0/24"
	prefixAFT1V6         = "2001:db8:2::/64"
	prefixAFT2V6         = "2001:db8:2::1/128"
	prefixAFT3V6         = "2001:db8:2::2/128"
	vrfV4Pfx             = "100.64.1.0/24"
	vrfV6Pfx             = "2001:db8:3::/64"
	maskRange            = "exact"
	notificationWaitTime = 30 * time.Second
	staticRouteIndex     = 100
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "DUT to ATE Port 1",
		MAC:     "02:00:02:02:02:02",
		IPv4:    "192.0.2.1",
		IPv4Len: 30,
		IPv6:    "2001:db8:0:1::1",
		IPv6Len: 64,
	}
	atePort1 = attrs.Attributes{
		Name:    "atePort1",
		Desc:    "ATE to DUT Port 1",
		MAC:     "02:00:02:01:01:01",
		IPv4:    "192.0.2.2",
		IPv4Len: 30,
		IPv6:    "2001:db8:0:1::2",
		IPv6Len: 64,
	}
	dutPort2 = attrs.Attributes{
		Desc:    "DUT to ATE Port 2",
		MAC:     "02:00:04:02:02:02",
		IPv4:    "192.0.3.1",
		IPv4Len: 30,
		IPv6:    "2001:db8:0:2::1",
		IPv6Len: 64,
	}
	atePort2 = attrs.Attributes{
		Name:    "atePort2",
		Desc:    "ATE to DUT Port 2",
		MAC:     "02:00:04:01:01:01",
		IPv4:    "192.0.3.2",
		IPv4Len: 30,
		IPv6:    "2001:db8:0:2::2",
		IPv6Len: 64,
	}

	defaultIPv4Prefixes = []string{
		"198.51.100.0/24",
		"203.0.113.0/28",
		"100.64.0.0/24",
	}

	policyIPv4Prefixes = []string{
		"198.51.100.0/24",
		"203.0.113.0/28",
		"198.51.100.1/32",
		"192.0.2.0/24",
	}

	defaultIPv6Prefixes = []string{
		"2001:db8:2::/64",
		"2001:db8:2::1/128",
		"2001:db8:2::2/128",
	}

	policyIPv6Prefixes = []string{
		"2001:db8:2::/64",
		"2001:db8:2::1/128",
	}

	vrfV4Prefixes = []string{
		"198.51.100.0/24",
		"100.64.1.0/24",
		"203.0.113.128/28",
	}
	vrfV6Prefixes = []string{
		"2001:db8:2::/64",
		"2001:db8:2::1/128",
		"2001:db8:2::2/128",
	}
)

type dynamicUpdateTestParams struct {
	testID     string
	prefixSet  string
	prefix1    string
	prefix2    string
	prefix3    string
	nhIP       string
	maskRange  string
	policyName string
	indx       string
}

// TestMain runs featureprofile tests.
func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// TestAFTPrefixFilteringDynamicUpdates implements AFT-6.4.
func TestAFTPrefixFilteringDynamicUpdates(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	batch := configureDUT(t, dut)
	configurePolicies(t, dut, batch)
	mustConfigureStaticRoute(t, dut, batch, defaultIPv4Prefixes, vrfV4Prefixes, atePort1.IPv4, atePort2.IPv4, staticRouteIndex)
	mustConfigureStaticRoute(t, dut, batch, defaultIPv6Prefixes, vrfV6Prefixes, atePort1.IPv6, atePort2.IPv6, staticRouteIndex+100)
	topo, interfaceNamesList := configureATE(t, ate)
	ate.OTG().PushConfig(t, topo)
	ate.OTG().StartProtocols(t)
	cfgplugins.IsIPv4InterfaceARPresolved(t, ate, cfgplugins.AddressFamilyParams{InterfaceNames: interfaceNamesList})
	cfgplugins.IsIPv6InterfaceARPresolved(t, ate, cfgplugins.AddressFamilyParams{InterfaceNames: interfaceNamesList})

	tests := []struct {
		name string
		test func(t *testing.T, dut *ondatra.DUTDevice)
	}{
		{
			name: "AFT-6.4.1-DynamicIPv4PrefixSetUpdates",
			test: validateIPv4DynamicUpdates,
		},
		{
			name: "AFT-6.4.2-DynamicIPv6PrefixSetUpdates",
			test: validateIPv6DynamicUpdates,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.test(t, dut)
		})
	}
}

// configureDUT configures the DUT with the necessary VRF, interfaces, BGP, and redistribution policies.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) *gnmi.SetBatch {
	t.Helper()
	batch := &gnmi.SetBatch{}
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	configureHardwareInit(t, dut)
	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")
	nonDefaultNI := cfgplugins.ConfigureNetworkInstance(t, dut, vrfName, false)
	configureDUTInterface(t, dut, batch, &dutPort1, p1)
	configureDUTInterface(t, dut, batch, &dutPort2, p2)
	cfgplugins.UpdateNetworkInstanceOnDut(t, dut, vrfName, nonDefaultNI)
	configureDUTPort(t, dut, batch, &dutPort2, p2, vrfName)
	batch.Set(t, dut)
	return batch
}

// configureDUTInterface configure interfaces on DUT.
func configureDUTInterface(t *testing.T, dut *ondatra.DUTDevice, batch *gnmi.SetBatch, attrs *attrs.Attributes, p *ondatra.Port) {
	t.Helper()
	ocPath := gnmi.OC()
	i := attrs.NewOCInterface(p.Name(), dut)
	i.Description = ygot.String(attrs.Desc)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	if deviations.InterfaceEnabled(dut) {
		i.Enabled = ygot.Bool(true)
	}

	i.GetOrCreateEthernet()
	i4 := i.GetOrCreateSubinterface(0).GetOrCreateIpv4()
	i4.Enabled = ygot.Bool(true)
	av4 := i4.GetOrCreateAddress(attrs.IPv4)
	av4.PrefixLength = ygot.Uint8(attrs.IPv4Len)

	i6 := i.GetOrCreateSubinterface(0).GetOrCreateIpv6()
	i6.Enabled = ygot.Bool(true)
	av6 := i6.GetOrCreateAddress(attrs.IPv6)
	av6.PrefixLength = ygot.Uint8(attrs.IPv6Len)

	gnmi.BatchUpdate(batch, ocPath.Interface(p.Name()).Config(), i)
}

// configureDUTPort configure DUT ports.
func configureDUTPort(t *testing.T, dut *ondatra.DUTDevice, batch *gnmi.SetBatch, attrs *attrs.Attributes, p *ondatra.Port, niName string) {
	t.Helper()
	ocPath := gnmi.OC()
	cfgplugins.AssignToNetworkInstance(t, dut, p.Name(), niName, 0)
	i := attrs.NewOCInterface(p.Name(), dut)
	gnmi.BatchUpdate(batch, ocPath.Interface(p.Name()).Config(), i)
}

// configureHardwareInit sets up the initial hardware configuration on the DUT. It pushes hardware initialization configs for VRF Selection Extended feature and Policy Forwarding feature.
func configureHardwareInit(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	features := []cfgplugins.FeatureType{
		cfgplugins.FeatureVrfSelectionExtended,
		cfgplugins.FeaturePolicyForwarding,
	}
	for _, feature := range features {
		hardwareInitCfg := cfgplugins.NewDUTHardwareInit(t, dut, feature)
		if hardwareInitCfg != "" {
			cfgplugins.PushDUTHardwareInitConfig(t, dut, hardwareInitCfg)
		}
	}
}

// configureATE configures the ATE ports and BGP neighbor.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) (gosnappi.Config, []string) {
	t.Helper()
	var interfaceNamesList []string
	topo := gosnappi.NewConfig()
	p1 := ate.Port(t, "port1")
	p2 := ate.Port(t, "port2")

	atePort1.AddToOTG(topo, p1, &dutPort1)
	atePort2.AddToOTG(topo, p2, &dutPort2)
	// Collect interface/device names
	for _, dev := range topo.Devices().Items() {
		interfaceNamesList = append(interfaceNamesList, dev.Name())
	}
	return topo, interfaceNamesList
}

// validateIPv4DynamicUpdates validates:
//
//  1. Initial filtered subscription.
//  2. Add matching IPv4 route -> visible.
//  3. Add non-matching IPv4 route -> not visible.
//  4. Delete matching route -> removed.
//  5. Dynamic policy update -> newly matched route becomes visible.
func validateIPv4DynamicUpdates(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	validateDynamicUpdates(t, dut,
		dynamicUpdateTestParams{
			testID:     "AFT-6.4.1",
			prefixSet:  v4PfxSet,
			prefix1:    prefixAFT1V4,
			prefix2:    prefixAFT2V4,
			prefix3:    prefixAFT3V4,
			nhIP:       atePort1.IPv4,
			maskRange:  maskRange,
			policyName: v4Policy,
			indx:       "5001",
		},
	)
}

// validateIPv6DynamicUpdates validates:
//
//  1. Initial filtered IPv6 subscription.
//  2. Add matching IPv6 route.
//  3. Add non-matching IPv6 route.
//  4. Delete matching IPv6 route.
//  5. Dynamic policy update.
func validateIPv6DynamicUpdates(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	validateDynamicUpdates(t, dut,
		dynamicUpdateTestParams{
			testID:     "AFT-6.4.2",
			prefixSet:  v6PfxSet,
			prefix1:    prefixAFT1V6,
			prefix2:    prefixAFT2V6,
			prefix3:    prefixAFT3V6,
			nhIP:       atePort1.IPv6,
			maskRange:  maskRange,
			policyName: v6Policy,
			indx:       "6001",
		},
	)
}

// validateDynamicUpdates validates dynamic AFT prefix filtering behavior for both IPv4 and IPv6 route policies.
func validateDynamicUpdates(t *testing.T, dut *ondatra.DUTDevice, pArgs dynamicUpdateTestParams) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cfgplugins.ConfigureGlobalFilterPolicies(t, dut, cfgplugins.ConfigureGlobalFilterPoliciesParams{V4Policy: pArgs.policyName, V6Policy: "", VRFName: deviations.DefaultNetworkInstance(dut)})

	wantPrefixes := map[string]bool{
		pArgs.prefix1: true,
		pArgs.prefix2: true,
	}

	gnmiClient, err := dut.RawAPIs().BindingDUT().DialGNMI(ctx)
	if err != nil {
		t.Fatalf("Failed to dial GNMI: %v", err)
	}

	// ------------------------------------------------------------
	// Initial Sync
	// ------------------------------------------------------------

	t.Logf("%s - Initial Synchronization", pArgs.testID)

	initialCollector := cfgplugins.NewCollector(t, dut, cfgplugins.NewCollectorParams{Context: ctx, Client: gnmiClient})
	cfgplugins.RunCollector(t, cfgplugins.RunCollectorParams{Ctx: context.Background(), Collector: initialCollector, Stop: aftcache.InitialSyncStoppingCondition(t, dut, wantPrefixes, map[string]bool{atePort1.IPv4: true}, map[string]bool{atePort1.IPv6: true}), Timeout: subscriptionWait})
	initialAFT, err := initialCollector.ToAFT(t, dut)
	if err != nil {
		t.Fatalf("ToAFT failed: %v", err)
	}

	cfgplugins.VerifyPrefixesPresent(t, cfgplugins.PrefixesParams{InfoAFT: initialAFT, Prefixes: []string{pArgs.prefix1, pArgs.prefix2}})
	// ------------------------------------------------------------
	// AFT-6.4.X.1 Add Prefix
	// ------------------------------------------------------------

	t.Logf("%s.1 - Addition of Prefix to Active Set", pArgs.testID)

	addCollector := cfgplugins.NewCollector(t, dut, cfgplugins.NewCollectorParams{Context: ctx, Client: gnmiClient})
	mustAddSingleStaticRoute(t, dut, deviations.DefaultNetworkInstance(dut), pArgs.prefix3, pArgs.indx, pArgs.nhIP)
	cfgplugins.RunCollector(t, cfgplugins.RunCollectorParams{Ctx: context.Background(), Collector: addCollector, Stop: aftcache.InitialSyncStoppingCondition(t, dut, map[string]bool{pArgs.prefix3: true}, map[string]bool{atePort1.IPv4: true}, map[string]bool{atePort1.IPv6: true}), Timeout: subscriptionWait})
	addAFT, err := addCollector.ToAFT(t, dut)
	if err != nil {
		t.Fatalf("ToAFT failed: %v", err)
	}
	cfgplugins.VerifyPrefixesPresent(t, cfgplugins.PrefixesParams{InfoAFT: addAFT, Prefixes: []string{pArgs.prefix3}})
	// Wait until notification received or timeout
	cfgplugins.RunCollector(t, cfgplugins.RunCollectorParams{Ctx: context.Background(), Collector: addCollector, Stop: aftcache.WaitForUpdateNotification(t, aftcache.NotificationExpectation{AddPrefix: pArgs.prefix3, NotificationWait: notificationWaitTime}), Timeout: subscriptionWait})
	// ------------------------------------------------------------
	// AFT-6.4.X.2 Delete Prefix
	// ------------------------------------------------------------

	t.Logf("%s.2 - Deletion of Prefix from Active Set", pArgs.testID)

	deleteCollector := cfgplugins.NewCollector(t, dut, cfgplugins.NewCollectorParams{Context: ctx, Client: gnmiClient})
	cfgplugins.RemovePrefixFromPrefixSet(t, dut, cfgplugins.RemovePrefixFromPrefixSetParams{PrefixSetName: pArgs.prefixSet, Prefix: pArgs.prefix1, MaskRange: pArgs.maskRange})
	cfgplugins.RunCollector(t, cfgplugins.RunCollectorParams{Ctx: context.Background(), Collector: deleteCollector, Stop: aftcache.InitialSyncStoppingCondition(t, dut, map[string]bool{pArgs.prefix1: true}, map[string]bool{atePort1.IPv4: true}, map[string]bool{atePort1.IPv6: true}), Timeout: subscriptionWait})
	delAFT, delerr := deleteCollector.ToAFT(t, dut)
	if delerr != nil {
		t.Fatalf("ToAFT failed: %v", delerr)
	}
	verifyPrefixRemovedFromPrefixSet(t, dut, pArgs.prefixSet, pArgs.prefix1, pArgs.maskRange)
	cfgplugins.VerifyPrefixesAbsent(t, cfgplugins.PrefixesParams{InfoAFT: delAFT, Prefixes: []string{pArgs.prefix1}})
	// Wait until notification received or timeout
	cfgplugins.RunCollector(t, cfgplugins.RunCollectorParams{Ctx: context.Background(), Collector: deleteCollector, Stop: aftcache.WaitForDeleteNotification(t, aftcache.NotificationExpectation{DeletePrefix: pArgs.prefix1, NotificationWait: notificationWaitTime}), Timeout: subscriptionWait})
	// ------------------------------------------------------------
	// AFT-6.4.X.3 Atomic Add/Delete
	// ------------------------------------------------------------

	t.Logf("%s.3 - Simultaneous Addition and Deletion", pArgs.testID)

	swapCollector := cfgplugins.NewCollector(t, dut, cfgplugins.NewCollectorParams{Context: ctx, Client: gnmiClient})
	atomicPrefixSetSwap(t, dut, pArgs.prefixSet, pArgs.prefix1, pArgs.prefix2, pArgs.maskRange)
	verifyPrefixRemovedFromPrefixSet(t, dut, pArgs.prefixSet, pArgs.prefix2, pArgs.maskRange)
	cfgplugins.RunCollector(t, cfgplugins.RunCollectorParams{Ctx: context.Background(), Collector: swapCollector, Stop: aftcache.InitialSyncStoppingCondition(t, dut, map[string]bool{pArgs.prefix3: true}, map[string]bool{atePort1.IPv4: true}, map[string]bool{atePort1.IPv6: true}), Timeout: subscriptionWait})
	swapAFT, swaperr := swapCollector.ToAFT(t, dut)
	if swaperr != nil {
		t.Fatalf("ToAFT failed: %v", swaperr)
	}
	cfgplugins.VerifyPrefixesPresent(t, cfgplugins.PrefixesParams{InfoAFT: swapAFT, Prefixes: []string{pArgs.prefix1}})
	cfgplugins.VerifyPrefixesAbsent(t, cfgplugins.PrefixesParams{InfoAFT: swapAFT, Prefixes: []string{pArgs.prefix2}})
	// Wait until notification received or timeout
	cfgplugins.RunCollector(t, cfgplugins.RunCollectorParams{Ctx: context.Background(), Collector: swapCollector, Stop: aftcache.WaitForUpdateDeleteNotification(t, aftcache.NotificationExpectation{AddPrefix: pArgs.prefix1, DeletePrefix: pArgs.prefix2, NotificationWait: notificationWaitTime}), Timeout: subscriptionWait})
}

// verifyPrefixRemovedFromPrefixSet verifies that the specified prefix no longer exists in the given prefix-set configuration.
func verifyPrefixRemovedFromPrefixSet(t *testing.T, dut *ondatra.DUTDevice, prefixSetName, prefix, maskRange string) {
	t.Helper()
	path := gnmi.OC().RoutingPolicy().DefinedSets().PrefixSet(prefixSetName).Prefix(prefix, maskRange).Config()

	if val, present := gnmi.Lookup(t, dut, path).Val(); present {
		t.Fatalf("Prefix %s mask-range %s still exists in prefix-set %s: %+v", prefix, maskRange, prefixSetName, val)
	}
	t.Logf("Verified prefix %s mask-range %s removed from prefix-set %s", prefix, maskRange, prefixSetName)
}

// atomicPrefixSetSwap atomically adds one prefix and removes another from the specified prefix-set using a single gNMI Set transaction.
func atomicPrefixSetSwap(t *testing.T, dut *ondatra.DUTDevice, prefixName, addPrefixVal, delPrefixVal, prefixMode string) {
	t.Helper()
	batch := &gnmi.SetBatch{}
	addPath := gnmi.OC().RoutingPolicy().DefinedSets().PrefixSet(prefixName).Prefix(addPrefixVal, prefixMode)
	addEntry := &oc.RoutingPolicy_DefinedSets_PrefixSet_Prefix{
		IpPrefix:        ygot.String(addPrefixVal),
		MasklengthRange: ygot.String(prefixMode),
	}
	gnmi.BatchReplace(batch, addPath.Config(), addEntry)
	delPath := gnmi.OC().RoutingPolicy().DefinedSets().PrefixSet(prefixName).Prefix(delPrefixVal, prefixMode)
	gnmi.BatchDelete(batch, delPath.Config())

	batch.Set(t, dut)
}

// configurePolicies configures routing policies.
func configurePolicies(t *testing.T, dut *ondatra.DUTDevice, batch *gnmi.SetBatch) {
	t.Helper()
	root := &oc.Root{}
	rp := root.GetOrCreateRoutingPolicy()
	// POLICY-MATCH-ALL
	cfgplugins.AddPrefixSetPolicy(t, rp, cfgplugins.PrefixSetPolicyParams{PolicyName: matchAllPolicy, StatementNames: []string{"10"}, PolicyResult: oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE})

	cfgplugins.AddPrefixSetPolicyWithMatch(t, rp, cfgplugins.PrefixSetPolicyParams{PolicyName: "POLICY-PREFIX-SET-A", StatementNames: []string{"10"}, PrefixSetNames: []string{"PREFIX-SET-A"}, PrefixList: policyIPv4Prefixes, PrefixMode: "exact", PolicyResult: oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE})
	cfgplugins.AddPrefixSetPolicyWithMatch(t, rp, cfgplugins.PrefixSetPolicyParams{PolicyName: "POLICY-PREFIX-SET-B", StatementNames: []string{"10"}, PrefixSetNames: []string{"PREFIX-SET-B"}, PrefixList: policyIPv6Prefixes, PrefixMode: "exact", PolicyResult: oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE})
	cfgplugins.AddPrefixSetPolicy(t, rp, cfgplugins.PrefixSetPolicyParams{PolicyName: "POLICY-SUBNET-V4", StatementNames: []string{"10"}, PrefixSetNames: []string{"PREFIX-SET-SUBNET-V4"}, MatchPrefixSet: true, PolicyResult: oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE})
	cfgplugins.AddPrefixSetPolicy(t, rp, cfgplugins.PrefixSetPolicyParams{PolicyName: "POLICY-SUBNET-V6", StatementNames: []string{"10"}, PrefixSetNames: []string{"PREFIX-SET-SUBNET-V6"}, MatchPrefixSet: true, PolicyResult: oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE})
	cfgplugins.AddPrefixSetPolicy(t, rp, cfgplugins.PrefixSetPolicyParams{PolicyName: "POLICY-MULTI-STMT", StatementNames: []string{"10", "20"}, PrefixSetNames: []string{"PREFIX-SET-A", "PREFIX-SET-SUBNET"}, MatchPrefixSet: true, PolicyResult: oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE})
	cfgplugins.AddPrefixSetPolicy(t, rp, cfgplugins.PrefixSetPolicyParams{PolicyName: "POLICY-DENY-PREFIX-SET-A", StatementNames: []string{"10", "20"}, PrefixSetNames: []string{"PREFIX-SET-A", ""}, MatchPrefixSet: true, PrefixDeny: true, PolicyResult: oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE})
	cfgplugins.AddPrefixSetPolicy(t, rp, cfgplugins.PrefixSetPolicyParams{PolicyName: "POLICY-TAG-MATCH", StatementNames: []string{"10"}, MatchPrefixSet: true, SetTag: true, PolicyResult: oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE})
	cfgplugins.AddPrefixSetPolicyWithMatch(t, rp, cfgplugins.PrefixSetPolicyParams{PolicyName: "POLICY-PREFIX-SET-VRF-A", StatementNames: []string{"10"}, PrefixSetNames: []string{"PREFIX-SET-VRF-A"}, PrefixList: []string{vrfV4Pfx}, PrefixMode: "24..32", PolicyResult: oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE})
	cfgplugins.AddPrefixSetPolicyWithMatch(t, rp, cfgplugins.PrefixSetPolicyParams{PolicyName: "POLICY-PREFIX-SET-VRF-B", StatementNames: []string{"20"}, PrefixSetNames: []string{"PREFIX-SET-VRF-B"}, PrefixList: []string{vrfV6Pfx}, PrefixMode: "65..128", PolicyResult: oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE})

	cfgplugins.ConfigureGlobalFilterPolicies(t, dut, cfgplugins.ConfigureGlobalFilterPoliciesParams{V4Policy: v4Policy, V6Policy: v6Policy, VRFName: deviations.DefaultNetworkInstance(dut)})
	gnmi.BatchReplace(batch, gnmi.OC().RoutingPolicy().Config(), rp)
	batch.Set(t, dut)
}

// mustConfigureStaticRoute installs a static route into the default NI.
func mustConfigureStaticRoute(t *testing.T, dut *ondatra.DUTDevice, batch *gnmi.SetBatch, defaultPrefixes, vrfPrefixes []string, nhIP, vrfNhIP string, indx int) {
	t.Helper()
	for idx, prefix := range defaultPrefixes {
		cfgplugins.ConfigureStaticRoute(t, dut, batch, cfgplugins.ConfigureStaticRouteParams{NetworkInstance: deviations.DefaultNetworkInstance(dut), Prefix: prefix, Index: fmt.Sprintf("%d", idx+indx), NextHop: nhIP})
	}
	// ------------------------------------------------------------
	// VRF-A IPv4 and IPv6 routes
	// ------------------------------------------------------------
	for idx, prefix := range vrfPrefixes {
		cfgplugins.ConfigureStaticRoute(t, dut, batch, cfgplugins.ConfigureStaticRouteParams{NetworkInstance: vrfName, Prefix: prefix, Index: fmt.Sprintf("%d", idx+indx+200), NextHop: vrfNhIP})
	}
	batch.Set(t, dut)
}

// mustAddSingleStaticRoute adds one static route.
func mustAddSingleStaticRoute(t *testing.T, dut *ondatra.DUTDevice, niName, prefix, index, nextHop string) {
	t.Helper()
	batch := &gnmi.SetBatch{}
	cfgplugins.ConfigureStaticRoute(t, dut, batch, cfgplugins.ConfigureStaticRouteParams{NetworkInstance: niName, Prefix: prefix, Index: index, NextHop: nextHop})
	batch.Set(t, dut)
}
