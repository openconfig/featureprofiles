// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0

// Package aftprefixfilteringresilience implements AFT-6.3:
// AFT Prefix Filtering Resilience.
package afts_prefix_filtering_resilience_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/iputil"
	"github.com/openconfig/featureprofiles/internal/telemetry/aftcache"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	spb "github.com/openconfig/gnoi/system"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygot/ygot"
)

const (
	vrfName            = "VRF-A"
	v4PfxSet           = "PREFIX-SET-A"
	ipv4Policy         = "POLICY-PREFIX-SET-A"
	ipv6Policy         = "POLICY-PREFIX-SET-B"
	matchAllPolicy     = "POLICY-MATCH-ALL"
	vrfAPolicy         = "POLICY-PREFIX-SET-VRF-A"
	vrfPrefixName      = "PREFIX-SET-VRF-A"
	scaleIPv4Routes    = 5000
	scaleIPv6Routes    = 2000
	scaleSyncDeadline  = 4 * time.Minute
	subscriptionWait   = 2 * time.Minute
	aftConvergenceTime = 10 * time.Minute
	// maxRebootTime is the maximum time allowed for the DUT to complete the reboot.
	maxRebootTime = 5 * time.Minute
	// rebootPollInterval is the interval at which the DUT's reachability is polled during reboot.
	rebootPollInterval   = 30 * time.Second
	matchPrefixAft1      = "198.51.100.0/24"
	matchPrefixAft2      = "203.0.113.0/28"
	matchVrfPfx1         = "100.64.1.0/24"
	matchVrfPfx2         = "100.64.1.128/25"
	matchVrfPfx3         = "203.0.113.128/28"
	matchVrfPfx4         = "198.51.100.1/32"
	matchPrefixAbsent    = "100.64.0.0/24"
	intStepV6            = "::1"
	intStepV4            = "0.0.0.1"
	scaleV4Pfx           = "198.18.0.0"
	scaleV6Pfx           = "2001:db8:0::"
	vrfV4Pfx             = "100.64.1.0/24"
	vrfV6Pfx             = "2001:db8:3::/64"
	v4AbsentPfx1         = "100.64.2.0/24"
	v4AbsentPfx2         = "203.0.113.64/28"
	scaleV4PfxLen        = 32
	scaleV6PfxLen        = 128
	defaultStatementName = "10"
	pfxV4MaskRange       = "24..32"
	pfxV6MaskRange       = "65..128"
	pfxMode              = "exact"
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
	}
	policyIPv6Prefixes = []string{
		"2001:db8:2::/64",
		"2001:db8:2::1/128",
	}
	defaultIPv6Prefixes = []string{
		"2001:db8:2::/64",
		"2001:db8:2::1/128",
		"2001:db8:2::2/128",
	}
	vrfV4Prefixes = []string{
		"198.51.100.0/24",
		"100.64.1.0/24",
		"203.0.113.128/28",
	}
	unexpectedPrefixes = []string{
		"10.0.0.0/8",
		"172.16.0.0/16",
		"192.168.0.0/16",
	}
	vrfV6Prefixes = []string{
		"2001:db8:2::/64",
		"2001:db8:2::1/128",
		"2001:db8:2::2/128",
	}
)

// TestMain runs featureprofile tests.
func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// TestAFTPrefixFilteringResilience implements AFT-6.3.
func TestAFTPrefixFilteringResilience(t *testing.T) {
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
			name: "AFT-6.3.1-ValidationAfterDeviceReboot",
			test: validateAfterReboot,
		},
		{
			name: "AFT-6.3.2-ScaleTest",
			test: validateScaleFiltering,
		},
		{
			name: "AFT-6.3.3-PerNetworkInstanceFiltering",
			test: validatePerNIFiltering,
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
		interfaceNamesList = append(
			interfaceNamesList,
			dev.Name(),
		)
	}
	return topo, interfaceNamesList
}

// fetchAFT collects AFT telemetry from two independent sessions and validates consistency between them.
func fetchAFT(t *testing.T, dut *ondatra.DUTDevice, aftSession1, aftSession2 *aftcache.AFTStreamSession, stoppingCondition aftcache.PeriodicHook, wantPrefixes map[string]bool) (*aftcache.AFTData, error) {
	t.Helper()
	cfgplugins.RunCollector(t, cfgplugins.RunCollectorParams{Ctx: context.Background(), Collector: aftSession1, Stop: stoppingCondition, Timeout: aftConvergenceTime})
	cfgplugins.RunCollector(t, cfgplugins.RunCollectorParams{Ctx: context.Background(), Collector: aftSession2, Stop: stoppingCondition, Timeout: aftConvergenceTime})

	aft1, err := aftSession1.ToAFT(t, dut)
	if err != nil {
		return nil, fmt.Errorf("error getting AFT from session1: %v", err)
	}

	aft2, err := aftSession2.ToAFT(t, dut)
	if err != nil {
		return nil, fmt.Errorf("error getting AFT from session2: %v", err)
	}

	filteredAFT1 := filterAFTByPrefixes(aft1, wantPrefixes)
	filteredAFT2 := filterAFTByPrefixes(aft2, wantPrefixes)

	sortSlices := cmpopts.SortSlices(
		func(a, b uint64) bool {
			return a < b
		},
	)

	if diff := cmp.Diff(filteredAFT1, filteredAFT2, sortSlices); diff != "" {
		return nil, fmt.Errorf("aft inconsistency detected: %s", diff)
	}
	return aft1, nil
}

// filterAFTByPrefixes filters only required prefixes and associated NHGs and NHs from full AFT data.
func filterAFTByPrefixes(aft *aftcache.AFTData, wantPrefixes map[string]bool) *aftcache.AFTData {
	return aft.FilterByPrefixes(wantPrefixes)
}

// validateAfterReboot validates:
// 1. AFT filtered subscription establishment
// 2. Stream termination during reboot
// 3. DUT recovery
// 4. Subscription re-establishment
// 5. Policy persistence after reboot
// 6. Correct filtered AFT entries after reboot
func validateAfterReboot(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wantPrefixes := map[string]bool{
		matchPrefixAft1: true,
		matchPrefixAft2: true,
	}

	// ------------------------------------------------------------
	// Verify configured policies before reboot
	// ------------------------------------------------------------

	verifyGlobalFilterPolicies(t, dut, ipv4Policy, ipv6Policy)

	// ------------------------------------------------------------
	// Establish initial gNMI subscriptions
	// ------------------------------------------------------------

	aftSession1 := aftcache.NewAFTStreamSession(ctx, t, cfgplugins.GnmiClientSession(t, dut, cfgplugins.PrefixesParams{Ctx: ctx}), dut)
	aftSession2 := aftcache.NewAFTStreamSession(ctx, t, cfgplugins.GnmiClientSession(t, dut, cfgplugins.PrefixesParams{Ctx: ctx}), dut)

	t.Log("Collecting initial filtered AFT entries")

	stoppingCondition := aftcache.InitialSyncStoppingCondition(t, dut, wantPrefixes, map[string]bool{atePort1.IPv4: true}, nil)

	aftBefore, err := fetchAFT(t, dut, aftSession1, aftSession2, stoppingCondition, wantPrefixes)
	if err != nil {
		t.Fatalf("Failed to fetch initial AFT: %v", err)
	}

	verifyFilteredPrefixes(t, aftBefore, wantPrefixes, true)

	// ------------------------------------------------------------
	// Verify stream terminates during reboot
	// ------------------------------------------------------------

	streamErrCh := make(chan error, 2)

	go func() {
		_, err := aftSession1.ToAFT(t, dut)
		streamErrCh <- err
	}()

	go func() {
		_, err := aftSession2.ToAFT(t, dut)
		streamErrCh <- err
	}()
	cfgplugins.BackUpConfig(t, dut, "my-config")
	// ------------------------------------------------------------
	// Reboot DUT while subscription is active
	// ------------------------------------------------------------
	t.Log("Rebooting DUT")
	mustRebootDUT(t, dut)

	// ------------------------------------------------------------
	// Verify stream terminates correctly
	// ------------------------------------------------------------
	t.Log("Verifying gNMI stream termination")
	for i := 0; i < 2; i++ {
		select {
		case err := <-streamErrCh:
			if err == nil || isExpectedReconnectError(err) {
				t.Logf("Observed expected stream termination error: %v", err)
			} else {
				t.Fatalf("Unexpected stream error during reboot: %v", err)
			}
		case <-time.After(subscriptionWait):
			t.Fatalf("Timed out waiting for stream termination")
		}
	}

	// ------------------------------------------------------------
	// Wait for DUT recovery
	// ------------------------------------------------------------

	t.Log("Waiting for DUT recovery")

	waitForGNMI(t, dut)

	t.Log("DUT recovered successfully")
	cfgplugins.RestoreConfig(t, dut, "my-config")
	// ------------------------------------------------------------
	// Verify policy persistence after reboot
	// ------------------------------------------------------------

	verifyGlobalFilterPolicies(t, dut, ipv4Policy, ipv6Policy)

	// ------------------------------------------------------------
	// Re-establish subscriptions
	// ------------------------------------------------------------

	t.Log("Re-establishing AFT subscriptions")

	aftSession3 := aftcache.NewAFTStreamSession(ctx, t, cfgplugins.GnmiClientSession(t, dut, cfgplugins.PrefixesParams{Ctx: ctx}), dut)
	aftSession4 := aftcache.NewAFTStreamSession(ctx, t, cfgplugins.GnmiClientSession(t, dut, cfgplugins.PrefixesParams{Ctx: ctx}), dut)

	stoppingCondition2 := aftcache.InitialSyncStoppingCondition(t, dut, wantPrefixes, map[string]bool{atePort1.IPv4: true}, nil)

	aftAfter, err := fetchAFT(t, dut, aftSession3, aftSession4, stoppingCondition2, wantPrefixes)
	if err != nil {
		t.Fatalf("Failed to fetch AFT after reboot: %v", err)
	}

	// ------------------------------------------------------------
	// Verify filtered entries after reboot
	// ------------------------------------------------------------

	verifyFilteredPrefixes(t, aftAfter, wantPrefixes, true)

	t.Log("AFT reboot validation completed successfully")
}

// verifyGlobalFilterPolicies verifies global-filter IPv4/IPv6 policies persisted.
func verifyGlobalFilterPolicies(t *testing.T, dut *ondatra.DUTDevice, wantIPv4Policy, wantIPv6Policy string) {
	t.Helper()
	if deviations.AftsGlobalFilterPolicyOCUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			cfgplugins.VerifyGlobalFilterPoliciesCLI(t, dut, cfgplugins.GlobalFilterParams{PrefixName: wantIPv4Policy})
			cfgplugins.VerifyGlobalFilterPoliciesCLI(t, dut, cfgplugins.GlobalFilterParams{PrefixName: wantIPv6Policy})
		}
	} else {
		const (
			ocPolicyPathV4 = "/network-instances/network-instance/afts/global-filter/config/ipv4-policy"
			ocPolicyPathV6 = "/network-instances/network-instance/afts/global-filter/config/ipv6-policy"
		)
		gnmiClient, err := dut.RawAPIs().BindingDUT().DialGNMI(context.Background())
		if err != nil {
			t.Fatalf("Failed to dial GNMI: %v", err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		req := &gpb.GetRequest{
			Path: []*gpb.Path{
				mustPath(t, ocPolicyPathV4),
				mustPath(t, ocPolicyPathV6),
			},
			Type: gpb.GetRequest_CONFIG,
		}

		resp, err := gnmiClient.Get(ctx, req)
		if err != nil {
			t.Fatalf("GNMI Get failed: %v", err)
		}

		var gotIPv4Policy string
		var gotIPv6Policy string

		for _, notif := range resp.GetNotification() {
			for _, upd := range notif.GetUpdate() {

				path, err := ygot.PathToString(upd.GetPath())
				if err != nil {
					t.Fatalf("PathToString failed: %v", err)
				}

				val := upd.GetVal().GetStringVal()

				switch {
				case strings.Contains(path, "ipv4-policy"):
					gotIPv4Policy = val

				case strings.Contains(path, "ipv6-policy"):
					gotIPv6Policy = val
				}
			}
		}

		if gotIPv4Policy != wantIPv4Policy {
			t.Fatalf("IPv4 policy mismatch got=%s want=%s", gotIPv4Policy, wantIPv4Policy)
		}

		if gotIPv6Policy != wantIPv6Policy {
			t.Fatalf("IPv6 policy mismatch got=%s want=%s", gotIPv6Policy, wantIPv6Policy)
		}

		t.Logf("Verified persisted global-filter policies IPv4=%s IPv6=%s", gotIPv4Policy, gotIPv6Policy)
	}
}

// mustPath converts a string-based gNMI path into a structured gNMI Path.
func mustPath(t *testing.T, path string) *gpb.Path {
	t.Helper()
	p, err := ygot.StringToPath(path, ygot.StructuredPath)
	if err != nil {
		t.Fatalf("Failed to parse path %s: %v", path, err)
	}
	return p
}

// isExpectedReconnectError validates acceptable reboot-window errors.
func isExpectedReconnectError(err error) bool {
	if err == nil {
		return false
	}

	s := strings.ToLower(err.Error())

	return strings.Contains(s, "unavailable") ||
		strings.Contains(s, "transport is closing") ||
		strings.Contains(s, "connection refused") ||
		strings.Contains(s, "eof") ||
		strings.Contains(s, "context canceled") ||
		strings.Contains(s, "deadline exceeded") ||
		strings.Contains(s, "connection reset")
}

// configurePolicies configures routing policies.
func configurePolicies(t *testing.T, dut *ondatra.DUTDevice, batch *gnmi.SetBatch) {
	t.Helper()
	root := &oc.Root{}
	rp := root.GetOrCreateRoutingPolicy()
	cfgplugins.AddPrefixSetPolicy(t, rp, cfgplugins.PrefixSetPolicyParams{PolicyName: matchAllPolicy, StatementNames: []string{defaultStatementName}, PolicyResult: oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE})
	cfgplugins.AddPrefixSetPolicyWithMatch(t, rp, cfgplugins.PrefixSetPolicyParams{PolicyName: ipv4Policy, StatementNames: []string{defaultStatementName}, PrefixSetNames: []string{v4PfxSet}, PrefixList: policyIPv4Prefixes, PrefixMode: pfxMode, PolicyResult: oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE})
	cfgplugins.AddPrefixSetPolicyWithMatch(t, rp, cfgplugins.PrefixSetPolicyParams{PolicyName: ipv6Policy, StatementNames: []string{"20"}, PrefixSetNames: []string{v4PfxSet}, PrefixList: policyIPv6Prefixes, PrefixMode: pfxMode, PolicyResult: oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE})
	cfgplugins.AddPrefixSetPolicy(t, rp, cfgplugins.PrefixSetPolicyParams{PolicyName: "POLICY-SUBNET-V4", StatementNames: []string{defaultStatementName}, PrefixSetNames: []string{"PREFIX-SET-SUBNET-V4"}, MatchPrefixSet: true, PolicyResult: oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE})
	cfgplugins.AddPrefixSetPolicy(t, rp, cfgplugins.PrefixSetPolicyParams{PolicyName: "POLICY-SUBNET-V6", StatementNames: []string{defaultStatementName}, PrefixSetNames: []string{"PREFIX-SET-SUBNET-V6"}, MatchPrefixSet: true, PolicyResult: oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE})
	cfgplugins.AddPrefixSetPolicy(t, rp, cfgplugins.PrefixSetPolicyParams{PolicyName: "POLICY-MULTI-STMT", StatementNames: []string{defaultStatementName, "20"}, PrefixSetNames: []string{"PREFIX-SET-A", "PREFIX-SET-SUBNET"}, MatchPrefixSet: true, PolicyResult: oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE})
	cfgplugins.AddPrefixSetPolicy(t, rp, cfgplugins.PrefixSetPolicyParams{PolicyName: "POLICY-DENY-PREFIX-SET-A", StatementNames: []string{defaultStatementName, "20"}, PrefixSetNames: []string{"PREFIX-SET-A", ""}, MatchPrefixSet: true, PrefixDeny: true, PolicyResult: oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE})
	cfgplugins.AddPrefixSetPolicy(t, rp, cfgplugins.PrefixSetPolicyParams{PolicyName: "POLICY-TAG-MATCH", StatementNames: []string{defaultStatementName}, MatchPrefixSet: true, SetTag: true, PolicyResult: oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE})
	cfgplugins.AddPrefixSetPolicyWithMatch(t, rp, cfgplugins.PrefixSetPolicyParams{PolicyName: "POLICY-PREFIX-SET-VRF-A", StatementNames: []string{defaultStatementName}, PrefixSetNames: []string{vrfPrefixName}, PrefixList: []string{vrfV4Pfx}, PrefixMode: pfxV4MaskRange, PolicyResult: oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE})
	cfgplugins.AddPrefixSetPolicyWithMatch(t, rp, cfgplugins.PrefixSetPolicyParams{PolicyName: "POLICY-PREFIX-SET-VRF-A", StatementNames: []string{"20"}, PrefixSetNames: []string{vrfPrefixName}, PrefixList: []string{vrfV6Pfx}, PrefixMode: pfxV6MaskRange, PolicyResult: oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE})

	cfgplugins.ConfigureGlobalFilterPolicies(t, dut, cfgplugins.ConfigureGlobalFilterPoliciesParams{V4Policy: ipv4Policy, V6Policy: ipv6Policy, VRFName: deviations.DefaultNetworkInstance(dut)})
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

// mustRebootDUT performs gNOI reboot.
func mustRebootDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	rebootRequest := &spb.RebootRequest{
		Method:  spb.RebootMethod_COLD,
		Delay:   0,
		Message: "Reboot without delay",
		Force:   true,
	}
	gnoiClient, err := dut.RawAPIs().BindingDUT().DialGNOI(context.Background())
	if err != nil {
		t.Fatalf("Error dialing gNOI: %v", err)
	}
	bootTimeBeforeReboot := gnmi.Get(t, dut, gnmi.OC().System().BootTime().State())
	t.Logf("DUT boot time before reboot: %v", bootTimeBeforeReboot)
	t.Log("Sending reboot request to DUT")
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), maxRebootTime)
	defer cancel()
	if _, err = gnoiClient.System().Reboot(ctxWithTimeout, rebootRequest); err != nil {
		t.Fatalf("Failed to reboot DUT with unexpected err: %v", err)
	}
	t.Log("Reboot request sent to DUT, waiting for DUT to reboot")
}

// waitForGNMI waits for DUT recovery.
func waitForGNMI(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	startReboot := time.Now()
	t.Log("Wait for DUT to boot up by polling the telemetry output.")
	ticker := time.NewTicker(rebootPollInterval)
	defer ticker.Stop()
	timeout := time.After(maxRebootTime)
	var deviceWentDown bool
	for {
		select {
		case <-timeout:
			t.Fatalf("Timeout exceeded: DUT did not reboot within maximum boot time(%v).", maxRebootTime)
		case <-ticker.C:
			var currentTime string
			errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				currentTime = gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State())
			})
			if errMsg != nil {
				if !deviceWentDown {
					t.Log("Device is now unreachable. Waiting for it to come back up.")
					deviceWentDown = true
				}
				t.Logf("Time elapsed %.2f seconds, DUT not reachable yet: %s.", time.Since(startReboot).Seconds(), *errMsg)
				continue
			}
			// DUT reachable again after reboot.
			if deviceWentDown {
				t.Logf("Device rebooted successfully with received time: %v", currentTime)
				t.Logf("Total reboot time: %.2f seconds", time.Since(startReboot).Seconds())
				return
			}
		}
	}
}

// validateScaleFiltering validates AFT filtering behavior under scale.
//
// Test flow:
//  1. Populate AFT with large-scale IPv4/IPv6 static routes.
//  2. Configure policies matching approximately 1%, 5%, and 20%.
//  3. Establish dual AFT subscriptions.
//  4. Measure synchronization time.
//  5. Verify synchronization completes within deadline.
//  6. Verify only expected filtered prefixes are streamed.
func validateScaleFiltering(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var ipv4Prefixes, ipv6Prefixes []string
	// ------------------------------------------------------------
	// Install scale routes
	// ------------------------------------------------------------

	t.Logf("Installing %d IPv4 and %d IPv6 static routes", scaleIPv4Routes, scaleIPv6Routes)
	ipv4Pfs, err := iputil.GenerateIPsWithStep(scaleV4Pfx, scaleIPv4Routes, intStepV4)
	if err != nil {
		t.Fatalf("failed to generate DUT IPs: %v", err)
	}
	for _, ip := range ipv4Pfs {
		ipv4Prefixes = append(ipv4Prefixes, fmt.Sprintf("%s/%d", ip, scaleV4PfxLen))
	}
	ipv6Pfs, err := iputil.GenerateIPv6sWithStep(scaleV6Pfx, scaleIPv6Routes, intStepV6)
	if err != nil {
		t.Fatalf("failed to generate DUT IPv6s: %v", err)
	}
	for _, ip := range ipv6Pfs {
		ipv6Prefixes = append(ipv6Prefixes, fmt.Sprintf("%s/%d", ip, scaleV6PfxLen))
	}
	configureScaleStaticRoutes(t, dut, ipv4Prefixes, ipv6Prefixes)

	// ------------------------------------------------------------
	// Policy scenarios
	// ------------------------------------------------------------

	testCases := []struct {
		name         string
		policyName   string
		prefixSet    string
		matchPercent int
		ipv4         bool
	}{
		{
			name:         "IPv4-1Percent",
			policyName:   "POLICY-SCALE-IPV4-1",
			prefixSet:    "PREFIX-SCALE-IPV4-1",
			matchPercent: 1,
			ipv4:         true,
		},
		{
			name:         "IPv4-5Percent",
			policyName:   "POLICY-SCALE-IPV4-5",
			prefixSet:    "PREFIX-SCALE-IPV4-5",
			matchPercent: 5,
			ipv4:         true,
		},
		{
			name:         "IPv4-20Percent",
			policyName:   "POLICY-SCALE-IPV4-20",
			prefixSet:    "PREFIX-SCALE-IPV4-20",
			matchPercent: 20,
			ipv4:         true,
		},
		{
			name:         "IPv6-1Percent",
			policyName:   "POLICY-SCALE-IPV6-1",
			prefixSet:    "PREFIX-SCALE-IPV6-1",
			matchPercent: 1,
			ipv4:         false,
		},
		{
			name:         "IPv6-5Percent",
			policyName:   "POLICY-SCALE-IPV6-5",
			prefixSet:    "PREFIX-SCALE-IPV6-5",
			matchPercent: 5,
			ipv4:         false,
		},
		{
			name:         "IPv6-20Percent",
			policyName:   "POLICY-SCALE-IPV6-20",
			prefixSet:    "PREFIX-SCALE-IPV6-20",
			matchPercent: 20,
			ipv4:         false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			// ------------------------------------------------------------
			// Select expected prefixes
			// ------------------------------------------------------------

			var selectedPrefixes []string
			var stoppingCondition aftcache.PeriodicHook

			if tc.ipv4 {
				selectedPrefixes = selectPercentagePrefixes(ipv4Prefixes, tc.matchPercent)
			} else {
				selectedPrefixes = selectPercentagePrefixes(ipv6Prefixes, tc.matchPercent)
			}

			wantPrefixes := make(map[string]bool)
			for _, pfx := range selectedPrefixes {
				wantPrefixes[pfx] = true
			}
			// ------------------------------------------------------------
			// Create subscriptions
			// ------------------------------------------------------------

			aftSession1 := aftcache.NewAFTStreamSession(ctx, t, cfgplugins.GnmiClientSession(t, dut, cfgplugins.PrefixesParams{Ctx: ctx}), dut)
			aftSession2 := aftcache.NewAFTStreamSession(ctx, t, cfgplugins.GnmiClientSession(t, dut, cfgplugins.PrefixesParams{Ctx: ctx}), dut)

			if tc.ipv4 {
				cfgplugins.ConfigureGlobalFilterPolicies(t, dut, cfgplugins.ConfigureGlobalFilterPoliciesParams{V4Policy: tc.policyName, V6Policy: "", VRFName: deviations.DefaultNetworkInstance(dut)})
				stoppingCondition = aftcache.InitialSyncStoppingCondition(t, dut, wantPrefixes, map[string]bool{atePort1.IPv4: true}, nil)

			} else {
				cfgplugins.ConfigureGlobalFilterPolicies(t, dut, cfgplugins.ConfigureGlobalFilterPoliciesParams{V4Policy: "", V6Policy: tc.policyName, VRFName: deviations.DefaultNetworkInstance(dut)})
				stoppingCondition = aftcache.InitialSyncStoppingCondition(t, dut, wantPrefixes, nil, map[string]bool{atePort1.IPv6: true})
			}

			// ------------------------------------------------------------
			// Measure synchronization time
			// ------------------------------------------------------------

			start := time.Now()
			aftData, err := fetchAFT(t, dut, aftSession1, aftSession2, stoppingCondition, wantPrefixes)
			if err != nil {
				t.Fatalf("Failed to fetch scaled AFT: %v", err)
			}
			syncDuration := time.Since(start)
			t.Logf("Synchronization completed in %v", syncDuration)

			// ------------------------------------------------------------
			// Verify synchronization time
			// ------------------------------------------------------------

			if syncDuration > scaleSyncDeadline {
				t.Fatalf("Synchronization exceeded limit got=%v want<=%v", syncDuration, scaleSyncDeadline)
			}

			// ------------------------------------------------------------
			// Verify filtering correctness
			// ------------------------------------------------------------
			verifyFilteredPrefixes(t, aftData, wantPrefixes, tc.ipv4)
			t.Logf("Verified scale filtering for %s", tc.name)
		})
	}
}

// verifyFilteredPrefixes validates that all expected prefixes are present in the received AFT data and that unexpected prefixes are absent.
func verifyFilteredPrefixes(t *testing.T, gotPrefixes *aftcache.AFTData, wantPrefixes map[string]bool, ipv4 bool) {
	t.Helper()
	// ------------------------------------------------------------
	// Validate IPv4 entries
	// ------------------------------------------------------------
	if ipv4 {
		for pfx := range wantPrefixes {
			if _, ok := gotPrefixes.Prefixes[pfx]; !ok {
				t.Fatalf("Expected IPv4 prefix missing from filtered AFT: %s", pfx)
			}
		}
		for _, pfx := range unexpectedPrefixes {
			if _, ok := gotPrefixes.Prefixes[pfx]; ok {
				t.Fatalf("Unexpected IPv4 prefix present after filtering: %s", pfx)
			}
		}
		t.Log("Verified IPv4 filtered prefixes")
		return
	}
	// ------------------------------------------------------------
	// Validate IPv6 entries
	// ------------------------------------------------------------
	for pfx := range wantPrefixes {
		if _, ok := gotPrefixes.Prefixes[pfx]; !ok {
			t.Fatalf("Expected IPv6 prefix missing from filtered AFT: %s", pfx)
		}
	}
	for _, pfx := range unexpectedPrefixes {
		if _, ok := gotPrefixes.Prefixes[pfx]; ok {
			t.Fatalf("Unexpected IPv6 prefix present after filtering: %s", pfx)
		}
	}
	t.Log("Verified IPv6 filtered prefixes")
}

// configureScaleStaticRoutes installs scaled IPv4 and IPv6 routes.
func configureScaleStaticRoutes(t *testing.T, dut *ondatra.DUTDevice, ipv4Prefixes, ipv6Prefixes []string) {
	t.Helper()
	batch := &gnmi.SetBatch{}
	for idx, prefix := range ipv4Prefixes {
		cfgplugins.ConfigureStaticRoute(t, dut, batch, cfgplugins.ConfigureStaticRouteParams{NetworkInstance: deviations.DefaultNetworkInstance(dut), Prefix: prefix, Index: fmt.Sprintf("v4-%d", idx), NextHop: atePort1.IPv4})
	}

	for idx, prefix := range ipv6Prefixes {
		cfgplugins.ConfigureStaticRoute(t, dut, batch, cfgplugins.ConfigureStaticRouteParams{NetworkInstance: deviations.DefaultNetworkInstance(dut), Prefix: prefix, Index: fmt.Sprintf("v6-%d", idx), NextHop: atePort1.IPv6})
	}
	batch.Set(t, dut)
	t.Log("Scale static routes installed successfully")
}

// selectPercentagePrefixes selects percentage-based subset.
func selectPercentagePrefixes(prefixes []string, percent int) []string {
	count := (len(prefixes) * percent) / 100
	if count == 0 {
		count = 1
	}
	return prefixes[:count]
}

// validatePerNIFiltering validates:
//
// 1. Independent AFT filtering per network-instance.
// 2. Multiple collectors subscribing simultaneously.
// 3. Correct filtered prefix visibility.
// 4. Dynamic route update propagation.
// 5. No leakage between collectors.
// 6. Policy-change behavior.
// 7. Collector stability during filter updates.
func validatePerNIFiltering(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ------------------------------------------------------------
	// Configure NI-specific AFT filters
	// ------------------------------------------------------------

	t.Log("Configuring per-network-instance AFT filters")

	cfgplugins.ConfigureGlobalFilterPolicies(t, dut, cfgplugins.ConfigureGlobalFilterPoliciesParams{V4Policy: ipv4Policy, V6Policy: ipv6Policy, VRFName: deviations.DefaultNetworkInstance(dut)})
	cfgplugins.ConfigureGlobalFilterPolicies(t, dut, cfgplugins.ConfigureGlobalFilterPoliciesParams{V4Policy: vrfAPolicy, V6Policy: vrfAPolicy, VRFName: vrfName})

	// ------------------------------------------------------------
	// Expected prefixes
	// ------------------------------------------------------------

	defaultWant := map[string]bool{
		matchPrefixAft1: true,
		matchPrefixAft2: true,
	}

	vrfWant := map[string]bool{
		matchVrfPfx1: true,
	}

	// ------------------------------------------------------------
	// Collector-1 (DEFAULT)
	// ------------------------------------------------------------

	collector1 := aftcache.NewAFTStreamSession(ctx, t, cfgplugins.GnmiClientSession(t, dut, cfgplugins.PrefixesParams{Ctx: ctx}), dut)

	// ------------------------------------------------------------
	// Collector-2 (VRF-A)
	// ------------------------------------------------------------

	collector2 := aftcache.NewAFTStreamSession(ctx, t, cfgplugins.GnmiClientSession(t, dut, cfgplugins.PrefixesParams{Ctx: ctx}), dut)

	// ------------------------------------------------------------
	// Initial sync validation
	// ------------------------------------------------------------

	t.Log("Validating initial filtered AFT state")

	defaultStop := aftcache.InitialSyncStoppingCondition(t, dut, defaultWant, map[string]bool{atePort1.IPv4: true}, nil)
	vrfStop := aftcache.InitialSyncStoppingCondition(t, dut, vrfWant, map[string]bool{atePort2.IPv4: true}, nil)

	cfgplugins.RunCollector(t, cfgplugins.RunCollectorParams{Ctx: context.Background(), Collector: collector1, Stop: defaultStop, Timeout: subscriptionWait})
	cfgplugins.RunCollector(t, cfgplugins.RunCollectorParams{Ctx: context.Background(), Collector: collector2, Stop: vrfStop, Timeout: subscriptionWait})

	defaultAFT, err := collector1.ToAFT(t, dut)
	if err != nil {
		t.Fatalf("Collector1 ToAFT failed: %v", err)
	}

	vrfAFT, err := collector2.ToAFT(t, dut)
	if err != nil {
		t.Fatalf("Collector2 ToAFT failed: %v", err)
	}

	// ------------------------------------------------------------
	// Validate Collector-1
	// ------------------------------------------------------------

	cfgplugins.VerifyPrefixesPresent(t, cfgplugins.PrefixesParams{InfoAFT: defaultAFT, Prefixes: []string{matchPrefixAft1, matchPrefixAft2}})
	cfgplugins.VerifyPrefixesAbsent(t, cfgplugins.PrefixesParams{InfoAFT: defaultAFT, Prefixes: []string{matchPrefixAbsent}})

	// ------------------------------------------------------------
	// Validate Collector-2
	// ------------------------------------------------------------

	cfgplugins.VerifyPrefixesPresent(t, cfgplugins.PrefixesParams{InfoAFT: vrfAFT, Prefixes: []string{matchVrfPfx1}})
	cfgplugins.VerifyPrefixesAbsent(t, cfgplugins.PrefixesParams{InfoAFT: vrfAFT, Prefixes: []string{matchPrefixAft1, matchPrefixAft2}})

	// ------------------------------------------------------------
	// Add unmatched route to DEFAULT
	// Neither collector should receive it
	// ------------------------------------------------------------

	t.Log("Adding unmatched route to DEFAULT")
	mustAddSingleStaticRoute(t, dut, deviations.DefaultNetworkInstance(dut), v4AbsentPfx1, fmt.Sprintf("%d", staticRouteIndex+900), atePort1.IPv4)

	mustValidatePrefixAbsentFromCollectors(t, dut, collector1, collector2, v4AbsentPfx1)

	// ------------------------------------------------------------
	// Add unmatched route
	// ------------------------------------------------------------

	t.Log("Adding unmatched route to DEFAULT")
	mustAddSingleStaticRoute(t, dut, deviations.DefaultNetworkInstance(dut), v4AbsentPfx2, fmt.Sprintf("%d", staticRouteIndex+901), atePort1.IPv4)

	mustValidatePrefixAbsentFromCollectors(t, dut, collector1, collector2, v4AbsentPfx2)

	// ------------------------------------------------------------
	// Add matched exact route to DEFAULT
	// Collector1 should receive it
	// ------------------------------------------------------------

	t.Log("Adding matched route to DEFAULT")
	mustAddSingleStaticRoute(t, dut, deviations.DefaultNetworkInstance(dut), matchVrfPfx4, fmt.Sprintf("%d", staticRouteIndex+902), atePort1.IPv4)

	mustVerifyPrefixesEventuallyPresent(t, dut, collector1, []string{matchVrfPfx4}, subscriptionWait)

	mustVerifyPrefixAbsent(t, dut, collector2, matchVrfPfx4)

	// ------------------------------------------------------------
	// Add matched subnet route to VRF-A
	// Collector2 should receive it
	// ------------------------------------------------------------

	t.Log("Adding matched subnet route to VRF-A")
	mustAddSingleStaticRoute(t, dut, vrfName, matchVrfPfx2, fmt.Sprintf("%d", staticRouteIndex+903), atePort1.IPv4)

	mustVerifyPrefixesEventuallyPresent(t, dut, collector2, []string{matchVrfPfx2}, subscriptionWait)

	mustVerifyPrefixAbsent(t, dut, collector1, matchVrfPfx2)

	// ------------------------------------------------------------
	// Change VRF-A policy to MATCH-ALL
	// ------------------------------------------------------------

	t.Log("Changing VRF-A policy to POLICY-MATCH-ALL")

	cfgplugins.ConfigureGlobalFilterPolicies(t, dut, cfgplugins.ConfigureGlobalFilterPoliciesParams{V4Policy: matchAllPolicy, V6Policy: matchAllPolicy, VRFName: vrfName})
	// ------------------------------------------------------------
	// Collector1 should remain stable
	// ------------------------------------------------------------

	defaultAFTAfter, err := collector1.ToAFT(t, dut)
	if err != nil {
		t.Fatalf("Collector1 unexpectedly failed after policy change: %v", err)
	}

	cfgplugins.VerifyPrefixesPresent(t, cfgplugins.PrefixesParams{InfoAFT: defaultAFTAfter, Prefixes: policyIPv4Prefixes})
	cfgplugins.VerifyPrefixesAbsent(t, cfgplugins.PrefixesParams{InfoAFT: defaultAFTAfter, Prefixes: []string{v4AbsentPfx1, v4AbsentPfx2}})

	// ------------------------------------------------------------
	// Collector2 should now receive all VRF-A routes
	// ------------------------------------------------------------

	t.Log("Waiting for Collector2 to receive all VRF-A routes")

	wantAllVRF := []string{matchPrefixAft1, matchVrfPfx1, matchVrfPfx3, matchVrfPfx2}

	mustVerifyPrefixesEventuallyPresent(t, dut, collector2, wantAllVRF, subscriptionWait)

	t.Log("Per-network-instance filtering validation completed successfully")
}

// mustAddSingleStaticRoute adds one static route.
func mustAddSingleStaticRoute(t *testing.T, dut *ondatra.DUTDevice, niName, prefix, index, nextHop string) {
	t.Helper()
	batch := &gnmi.SetBatch{}
	cfgplugins.ConfigureStaticRoute(t, dut, batch, cfgplugins.ConfigureStaticRouteParams{NetworkInstance: niName, Prefix: prefix, Index: index, NextHop: nextHop})
	batch.Set(t, dut)
}

// mustValidatePrefixAbsentFromCollectors validates prefix absent from both collectors.
func mustValidatePrefixAbsentFromCollectors(t *testing.T, dut *ondatra.DUTDevice, c1, c2 *aftcache.AFTStreamSession, prefix string) {
	t.Helper()
	aft1, err := c1.ToAFT(t, dut)
	if err != nil {
		t.Fatalf("Collector1 ToAFT failed: %v", err)
	}

	aft2, err := c2.ToAFT(t, dut)
	if err != nil {
		t.Fatalf("Collector2 ToAFT failed: %v", err)
	}

	if _, ok := aft1.Prefixes[prefix]; ok {
		t.Fatalf("Collector1 unexpectedly received %s", prefix)
	}

	if _, ok := aft2.Prefixes[prefix]; ok {
		t.Fatalf("Collector2 unexpectedly received %s", prefix)
	}
}

// mustVerifyPrefixAbsent validates prefix absent.
func mustVerifyPrefixAbsent(t *testing.T, dut *ondatra.DUTDevice, session *aftcache.AFTStreamSession, prefix string) {
	t.Helper()
	aft, err := session.ToAFT(t, dut)
	if err != nil {
		t.Fatalf("ToAFT failed: %v", err)
	}

	if _, ok := aft.Prefixes[prefix]; ok {
		t.Fatalf("Unexpected prefix received: %s", prefix)
	}
}

// mustVerifyPrefixesEventuallyPresent validates all prefixes appear.
func mustVerifyPrefixesEventuallyPresent(t *testing.T, dut *ondatra.DUTDevice, session *aftcache.AFTStreamSession, prefixes []string, timeout time.Duration) {
	t.Helper()
	wantPrefixes := make(map[string]bool)
	for _, pfx := range prefixes {
		wantPrefixes[pfx] = true
	}
	stoppingCondition := aftcache.InitialSyncStoppingCondition(t, dut, wantPrefixes, map[string]bool{atePort1.IPv4: true}, nil)
	session.ListenUntil(context.Background(), t, timeout, stoppingCondition)
}
