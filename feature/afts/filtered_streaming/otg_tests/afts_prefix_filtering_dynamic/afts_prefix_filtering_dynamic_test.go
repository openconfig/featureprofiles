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
	"flag"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/telemetry/aftcache"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

const (
	vrfName          = "VRF-A"
	v4PfxSet         = "PREFIX-SET-A"
	v6PfxSet         = "PREFIX-SET-B"
	v4Policy         = "POLICY-PREFIX-SET-A"
	v6Policy         = "POLICY-PREFIX-SET-B"
	matchAllPolicy   = "POLICY-MATCH-ALL"
	subscriptionWait = 3 * time.Minute
	prefixAft1V4     = "198.51.100.0/24"
	prefixAft2V4     = "203.0.113.0/28"
	prefixAft3V4     = "192.0.2.0/24"
	prefixAft1V6     = "2001:db8:2::/64"
	prefixAft2V6     = "2001:db8:2::1/128"
	prefixAft3V6     = "2001:db8:2::2/128"
	vrfV4Pfx         = "100.64.1.0/24"
	vrfV6Pfx         = "2001:db8:3::/64"
	maskRange        = "exact"
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
	debugNotifications = flag.Bool("debug_notifications", true, "Enable full AFT notification recording")
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
	configureStaticRoutes(t, dut, batch, defaultIPv4Prefixes, vrfV4Prefixes, atePort1.IPv4, atePort2.IPv4, 100)
	configureStaticRoutes(t, dut, batch, defaultIPv6Prefixes, vrfV6Prefixes, atePort1.IPv6, atePort2.IPv6, 200)
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
	d := gnmi.OC()
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

	gnmi.BatchUpdate(batch, d.Interface(p.Name()).Config(), i)
}

// configureDUTPort configure DUT ports.
func configureDUTPort(t *testing.T, dut *ondatra.DUTDevice, batch *gnmi.SetBatch, attrs *attrs.Attributes, p *ondatra.Port, niName string) {
	t.Helper()
	d := gnmi.OC()
	cfgplugins.AssignToNetworkInstance(t, dut, p.Name(), niName, 0)
	i := attrs.NewOCInterface(p.Name(), dut)
	gnmi.BatchUpdate(batch, d.Interface(p.Name()).Config(), i)
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
			prefix1:    prefixAft1V4,
			prefix2:    prefixAft2V4,
			prefix3:    prefixAft3V4,
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
			prefix1:    prefixAft1V6,
			prefix2:    prefixAft2V6,
			prefix3:    prefixAft3V6,
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
	ctx := context.Background()
	configureGlobalFilterPolicies(t, dut, pArgs.policyName, "", deviations.DefaultNetworkInstance(dut))

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

	initialCollector := newCollector(ctx, t, dut, gnmiClient)

	runCollector(ctx, t, initialCollector, aftcache.InitialSyncStoppingCondition(t, dut, wantPrefixes, nil, nil))
	aft, err := initialCollector.ToAFT(t, dut)
	if err != nil {
		t.Fatalf("ToAFT failed: %v", err)
	}

	verifyPrefixesPresent(t, aft, []string{pArgs.prefix1, pArgs.prefix2})

	// ------------------------------------------------------------
	// AFT-6.4.X.1 Add Prefix
	// ------------------------------------------------------------

	t.Logf("%s.1 - Addition of Prefix to Active Set", pArgs.testID)

	addCollector := newCollector(ctx, t, dut, gnmiClient)
	mustAddSingleStaticRoute(t, dut, deviations.DefaultNetworkInstance(dut), pArgs.prefix3, pArgs.indx, pArgs.nhIP)
	runCollector(ctx, t, addCollector, aftcache.InitialSyncStoppingCondition(t, dut, map[string]bool{pArgs.prefix3: true}, nil, nil))
	addAFT, err := addCollector.ToAFT(t, dut)
	if err != nil {
		t.Fatalf("ToAFT failed: %v", err)
	}
	verifyPrefixesPresent(t, addAFT, []string{pArgs.prefix3})
	// Wait until notification received or timeout
	runCollector(ctx, t, addCollector, aftcache.WaitForUpdateNotification(t, aftcache.NotificationExpectation{AddPrefix: pArgs.prefix3}))

	// ------------------------------------------------------------
	// AFT-6.4.X.2 Delete Prefix
	// ------------------------------------------------------------

	t.Logf("%s.2 - Deletion of Prefix from Active Set", pArgs.testID)

	deleteCollector := newCollector(ctx, t, dut, gnmiClient)
	removePrefixFromPrefixSet(t, dut, pArgs.prefixSet, pArgs.prefix1, pArgs.maskRange)
	runCollector(ctx, t, deleteCollector, aftcache.InitialSyncStoppingCondition(t, dut, map[string]bool{pArgs.prefix1: true}, nil, nil))
	delaft, delerr := deleteCollector.ToAFT(t, dut)
	if delerr != nil {
		t.Fatalf("ToAFT failed: %v", delerr)
	}
	verifyPrefixRemovedFromPrefixSet(t, dut, pArgs.prefixSet, pArgs.prefix1, pArgs.maskRange)
	verifyPrefixesAbsent(t, delaft, []string{pArgs.prefix1})
	// Wait until notification received or timeout
	runCollector(ctx, t, deleteCollector, aftcache.WaitForDeleteNotification(t, aftcache.NotificationExpectation{DeletePrefix: pArgs.prefix1}))

	// ------------------------------------------------------------
	// AFT-6.4.X.3 Atomic Add/Delete
	// ------------------------------------------------------------

	t.Logf("%s.3 - Simultaneous Addition and Deletion", pArgs.testID)

	swapCollector := newCollector(ctx, t, dut, gnmiClient)
	atomicPrefixSetSwap(t, dut, pArgs.prefixSet, pArgs.prefix1, pArgs.prefix2, pArgs.maskRange)
	verifyPrefixRemovedFromPrefixSet(t, dut, pArgs.prefixSet, pArgs.prefix2, pArgs.maskRange)
	runCollector(ctx, t, swapCollector, aftcache.InitialSyncStoppingCondition(t, dut, map[string]bool{pArgs.prefix3: true}, nil, nil))
	swapaft, swaperr := swapCollector.ToAFT(t, dut)
	if swaperr != nil {
		t.Fatalf("ToAFT failed: %v", swaperr)
	}
	verifyPrefixesPresent(t, swapaft, []string{pArgs.prefix1})
	verifyPrefixesAbsent(t, swapaft, []string{pArgs.prefix2})
	// Wait until notification received or timeout
	runCollector(ctx, t, swapCollector, aftcache.WaitForUpdateDeleteNotification(t, aftcache.NotificationExpectation{AddPrefix: pArgs.prefix1, DeletePrefix: pArgs.prefix2}))
}

// newCollector creates and returns a new AFT stream session. If debug_notifications is enabled, all received gNMI notifications are recorded in memory for later inspection and troubleshooting.
func newCollector(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, client gpb.GNMIClient) *aftcache.AFTStreamSession {
	t.Helper()
	c := aftcache.NewAFTStreamSession(ctx, t, client, dut)
	if *debugNotifications {
		c.WithDebug()
		t.Log("DEBUG MODE ENABLED: Recording all gNMI notifications to memory.")
	}
	return c
}

// runCollector starts the AFT stream collector and blocks until the supplied stopping condition is satisfied or the collector times out.
func runCollector(ctx context.Context, t *testing.T, collector *aftcache.AFTStreamSession, stop aftcache.PeriodicHook) {
	t.Helper()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		collector.ListenUntil(ctx, t, subscriptionWait, stop)
	}()
	wg.Wait()
}

// removePrefixFromPrefixSet removes the specified prefix entry from the given routing-policy prefix-set on the DUT.
func removePrefixFromPrefixSet(t *testing.T, dut *ondatra.DUTDevice, prefixSetName, prefix, maskRange string) {
	t.Helper()
	batch := &gnmi.SetBatch{}
	gnmi.BatchDelete(batch, gnmi.OC().RoutingPolicy().DefinedSets().PrefixSet(prefixSetName).Prefix(prefix, maskRange).Config())
	batch.Set(t, dut)
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

// configureGlobalFilterPolicies configures AFT global-filter policies for the specified network-instance.
func configureGlobalFilterPolicies(t *testing.T, dut *ondatra.DUTDevice, ipv4Policy, ipv6Policy, vrfName string) {
	t.Helper()
	if deviations.AftsGlobalFilterPolicyOCUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			t.Log("Skipping AFT global-filter attachment: unsupported on EOS")
		}
	} else {
		// TODO: Enable the following code once OC supports AFTs global filter configuration.
		// root := &oc.Root{}
		// ni := root.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
		// afts := ni.GetOrCreateAfts()
		// gf := afts.GetOrCreateGlobalFilter()
		// gf.Ipv4Policy = ygot.String(ipv4Policy)
		// gf.Ipv6Policy = ygot.String(ipv6Policy)
		// gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Afts().GlobalFilter().Config(), gf)
	}
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

	configureGlobalFilterPolicies(t, dut, v4Policy, v6Policy, deviations.DefaultNetworkInstance(dut))
	gnmi.BatchReplace(batch, gnmi.OC().RoutingPolicy().Config(), rp)
	batch.Set(t, dut)
}

// configureStaticRoutes installs a static route into the default NI and non default NI.
func configureStaticRoutes(t *testing.T, dut *ondatra.DUTDevice, batch *gnmi.SetBatch, defaultPrefixes, vrfPrefixes []string, nhIP, vrfNhIP string, indx int) {
	t.Helper()
	for idx, prefix := range defaultPrefixes {
		mustConfigureStaticRoute(t, dut, batch, deviations.DefaultNetworkInstance(dut), prefix, fmt.Sprintf("%d", idx+indx), nhIP)
	}
	// ------------------------------------------------------------
	// VRF-A IPv4 and IPv6 routes
	// ------------------------------------------------------------
	for idx, prefix := range vrfPrefixes {
		mustConfigureStaticRoute(t, dut, batch, vrfName, prefix, fmt.Sprintf("%d", idx+indx+200), vrfNhIP)
	}
	batch.Set(t, dut)
}

// mustConfigureStaticRoute installs a static route into the default NI.
func mustConfigureStaticRoute(t *testing.T, dut *ondatra.DUTDevice, batch *gnmi.SetBatch, niName, ipRoutePfx, indx, nxtIP string) {
	t.Helper()
	staticRoute := &cfgplugins.StaticRouteCfg{
		NetworkInstance: niName,
		Prefix:          ipRoutePfx,
		NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
			indx: oc.UnionString(nxtIP),
		},
	}

	if _, err := cfgplugins.NewStaticRouteCfg(batch, staticRoute, dut); err != nil {
		t.Fatalf("Failed to configure static route %s: %v", ipRoutePfx, err)
	}
}

// mustAddSingleStaticRoute adds one static route.
func mustAddSingleStaticRoute(t *testing.T, dut *ondatra.DUTDevice, niName, prefix, index, nextHop string) {
	t.Helper()
	batch := &gnmi.SetBatch{}
	staticRoute := &cfgplugins.StaticRouteCfg{
		NetworkInstance: niName,
		Prefix:          prefix,
		NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
			index: oc.UnionString(nextHop),
		},
	}
	if _, err := cfgplugins.NewStaticRouteCfg(batch, staticRoute, dut); err != nil {
		t.Fatalf("Failed creating static route %s: %v", prefix, err)
	}

	batch.Set(t, dut)
}

// verifyPrefixesPresent validates expected prefixes exist.
func verifyPrefixesPresent(t *testing.T, aft *aftcache.AFTData, prefixes []string) {
	t.Helper()

	for _, pfx := range prefixes {
		if _, ok := aft.Prefixes[pfx]; !ok {
			t.Errorf("expected prefix missing: %s", pfx)
		}
	}
}

// verifyPrefixesAbsent validates prefixes do not exist.
func verifyPrefixesAbsent(t *testing.T, aft *aftcache.AFTData, prefixes []string) {
	t.Helper()
	for _, pfx := range prefixes {
		if _, ok := aft.Prefixes[pfx]; ok {
			t.Errorf("unexpected prefix present: %s", pfx)
		}
	}
}
