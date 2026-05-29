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
	"sync"
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
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	vrfName            = "VRF-A"
	ipv4Policy         = "POLICY-PREFIX-SET-A"
	ipv6Policy         = "POLICY-PREFIX-SET-B"
	ipv4Pfx            = "POLICY-PREFIX-SET-A"
	ipv6Pfx            = "POLICY-PREFIX-SET-B"
	matchAllPolicy     = "POLICY-MATCH-ALL"
	vrfAPolicy         = "POLICY-PREFIX-SET-VRF-A"
	scaleIPv4Routes    = 5000
	scaleIPv6Routes    = 2000
	scaleSyncDeadline  = 4 * time.Minute
	rebootTimeout      = 10 * time.Minute
	subscriptionWait   = 2 * time.Minute
	aftConvergenceTime = 30 * time.Minute
	// maxRebootTime is the maximum time allowed for the DUT to complete the reboot.
	maxRebootTime = 5 * time.Minute
	// rebootPollInterval is the interval at which the DUT's reachability is polled during reboot.
	rebootPollInterval = 30 * time.Second
	prefixAft1         = "198.51.100.0/24"
	prefixAft2         = "203.0.113.0/28"
	vrfPfx1            = "100.64.1.0/24"
	pfxAbsent          = "100.64.0.0/24"
	intStepV6          = "::1"
	intStepV4          = "0.0.0.1"
	scaleV4Pfx         = "198.18.0.0"
	scaleV6Pfx         = "2001:db8:0::"
	scaleV4PfxLen      = 32
	scaleV6PfxLen      = 128
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

	defaultIPv6Prefixes = []string{
		"2001:DB8:1::/64",
		"2001:DB8:3::/64",
	}

	vrfAPrefixes = []string{
		"198.51.100.0/24",
		"100.64.1.0/24",
		"203.0.113.128/28",
	}
	unexpectedPrefixes = []string{
		"10.0.0.0/8",
		"172.16.0.0/16",
		"192.168.0.0/16",
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
	configurePolicies(t, dut)
	configureStaticRoutes(t, dut, batch)
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
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		aftSession1.ListenUntil(t.Context(), t, aftConvergenceTime, stoppingCondition)
	}()

	go func() {
		defer wg.Done()
		aftSession2.ListenUntil(t.Context(), t, aftConvergenceTime, stoppingCondition)
	}()

	wg.Wait()
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
	ctx := t.Context()
	wantPrefixes := map[string]bool{
		prefixAft1: true,
		prefixAft2: true,
	}

	// ------------------------------------------------------------
	// Verify configured policies before reboot
	// ------------------------------------------------------------

	verifyGlobalFilterPolicies(t, dut, ipv4Policy, ipv6Policy)

	// ------------------------------------------------------------
	// Establish initial gNMI subscriptions
	// ------------------------------------------------------------

	gnmiClient1, err := dut.RawAPIs().BindingDUT().DialGNMI(ctx)
	if err != nil {
		t.Fatalf("Failed to dial GNMI client1: %v", err)
	}

	gnmiClient2, err := dut.RawAPIs().BindingDUT().DialGNMI(ctx)
	if err != nil {
		t.Fatalf("Failed to dial GNMI client2: %v", err)
	}

	aftSession1 := aftcache.NewAFTStreamSession(ctx, t, gnmiClient1, dut)

	aftSession2 := aftcache.NewAFTStreamSession(ctx, t, gnmiClient2, dut)

	t.Log("Collecting initial filtered AFT entries")

	stoppingCondition := aftcache.InitialSyncStoppingCondition(t, dut, wantPrefixes, nil, nil)

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
			if err == nil || !isExpectedReconnectError(err) {
				t.Logf("Observed expected stream termination error: %v", err)
			} else {
				t.Fatalf("Unexpected stream error during reboot: %v", err)
			}
		case <-time.After(2 * time.Minute):
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
	// Create NEW GNMI clients after reboot
	// ------------------------------------------------------------

	gnmiClient3, err := dut.RawAPIs().BindingDUT().DialGNMI(ctx)
	if err != nil {
		t.Fatalf("Failed to redial GNMI client3: %v", err)
	}

	gnmiClient4, err := dut.RawAPIs().BindingDUT().DialGNMI(ctx)
	if err != nil {
		t.Fatalf("Failed to redial GNMI client4: %v", err)
	}

	// ------------------------------------------------------------
	// Re-establish subscriptions
	// ------------------------------------------------------------

	t.Log("Re-establishing AFT subscriptions")

	aftSession3 := aftcache.NewAFTStreamSession(ctx, t, gnmiClient3, dut)

	aftSession4 := aftcache.NewAFTStreamSession(ctx, t, gnmiClient4, dut)

	stoppingCondition2 := aftcache.InitialSyncStoppingCondition(t, dut, wantPrefixes, nil, nil)

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
			mustVerifyGlobalFilterPoliciesCLI(t, dut)
		}
	} else {
		const (
			ocPolicyPathV4 = "/network-instances/network-instance/afts/global-filter/config/ipv4-policy"
			ocPolicyPathV6 = "/network-instances/network-instance/afts/global-filter/config/ipv6-policy"
		)
		gnmiClient, err := dut.RawAPIs().BindingDUT().DialGNMI(t.Context())
		if err != nil {
			t.Fatalf("Failed to dial GNMI: %v", err)
		}

		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
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

// mustVerifyGlobalFilterPoliciesCLI verifies that the expected routing policy configuration is present in the DUT running configuration after reboot.
func mustVerifyGlobalFilterPoliciesCLI(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	cmd := fmt.Sprintf("sh running-config all | include %s", vrfAPolicy)
	runningConfig, err := dut.RawAPIs().CLI(t).RunCommand(context.Background(), cmd)
	if err != nil {
		t.Fatalf("'show running-config' failed: %v", err)
	}
	t.Logf("CLI output for %q:\n%s", cmd, runningConfig.Output())
	if !strings.Contains(runningConfig.Output(), vrfAPolicy) {
		t.Fatalf("Policy %s not found after reboot", vrfAPolicy)
	}
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
func configurePolicies(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	root := &oc.Root{}
	rp := root.GetOrCreateRoutingPolicy()
	createMatchAllPolicy(rp)
	createPrefixSetPolicy(rp, ipv4Pfx, ipv4Policy, "exact", policyIPv4Prefixes)
	createVRFAPolicy(rp)
	createPrefixSetPolicy(rp, ipv6Pfx, ipv6Policy, "exact", defaultIPv6Prefixes)
	configureGlobalFilterPolicies(t, dut, ipv4Policy, ipv6Policy, deviations.DefaultNetworkInstance(dut))
	gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)
}

// createMatchAllPolicy creates POLICY-MATCH-ALL.
func createMatchAllPolicy(rp *oc.RoutingPolicy) {
	pd := rp.GetOrCreatePolicyDefinition(matchAllPolicy)

	stmt, _ := pd.AppendNewStatement("10")

	stmt.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
}

// createPrefixSetPolicy creates a routing policy that matches a prefix-set and accepts routes matching the configured prefixes.
func createPrefixSetPolicy(rp *oc.RoutingPolicy, prefixSetName, policyName, matchMode string, prefixes []string) {
	ps := rp.GetOrCreateDefinedSets().GetOrCreatePrefixSet(prefixSetName)

	for _, prefix := range prefixes {
		addPrefix(ps, prefix, matchMode)
	}

	pd := rp.GetOrCreatePolicyDefinition(policyName)

	stmt, err := pd.AppendNewStatement("10")
	if err != nil {
		return
	}
	match := stmt.GetOrCreateConditions().GetOrCreateMatchPrefixSet()

	match.PrefixSet = ygot.String(prefixSetName)
	stmt.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
}

// createVRFAPolicy creates POLICY-PREFIX-SET-VRF-A.
func createVRFAPolicy(rp *oc.RoutingPolicy) {
	ps := rp.GetOrCreateDefinedSets().
		GetOrCreatePrefixSet("PREFIX-SET-VRF-A")

	addPrefix(ps, vrfPfx1, "24..32")

	pd := rp.GetOrCreatePolicyDefinition(vrfAPolicy)

	stmt, _ := pd.AppendNewStatement("10")

	match := stmt.GetOrCreateConditions().GetOrCreateMatchPrefixSet()

	match.PrefixSet = ygot.String("PREFIX-SET-VRF-A")

	stmt.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
}

// addPrefix adds prefix-set entry.
func addPrefix(ps *oc.RoutingPolicy_DefinedSets_PrefixSet, prefix, maskRange string) {
	p := ps.GetOrCreatePrefix(prefix, maskRange)

	p.IpPrefix = ygot.String(prefix)
	p.MasklengthRange = ygot.String(maskRange)
}

// configureStaticRoutes installs a static route into the default NI and non default NI.
func configureStaticRoutes(t *testing.T, dut *ondatra.DUTDevice, batch *gnmi.SetBatch) {
	t.Helper()
	// DEFAULT network-instance IPv4 routes
	for idx, prefix := range defaultIPv4Prefixes {
		mustConfigureStaticRoute(t, dut, batch, deviations.DefaultNetworkInstance(dut), prefix, fmt.Sprintf("%d", idx+1), atePort1.IPv4)
	}

	// DEFAULT network-instance IPv6 routes
	for idx, prefix := range defaultIPv6Prefixes {
		mustConfigureStaticRoute(t, dut, batch, deviations.DefaultNetworkInstance(dut), prefix, fmt.Sprintf("%d", idx+100), atePort1.IPv6)
	}
	// ------------------------------------------------------------
	// VRF-A IPv4 routes
	// ------------------------------------------------------------

	for idx, prefix := range vrfAPrefixes {
		mustConfigureStaticRoute(t, dut, batch, vrfName, prefix, fmt.Sprintf("%d", idx+200), atePort2.IPv4)
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

// mustRebootDUT performs gNOI reboot.
func mustRebootDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	rebootRequest := &spb.RebootRequest{
		Method:  spb.RebootMethod_COLD,
		Delay:   0,
		Message: "Reboot without delay",
		Force:   true,
	}
	gnoiClient, err := dut.RawAPIs().BindingDUT().DialGNOI(t.Context())
	if err != nil {
		t.Fatalf("Error dialing gNOI: %v", err)
	}
	bootTimeBeforeReboot := gnmi.Get(t, dut, gnmi.OC().System().BootTime().State())
	t.Logf("DUT boot time before reboot: %v", bootTimeBeforeReboot)
	t.Log("Sending reboot request to DUT")
	ctxWithTimeout, cancel := context.WithTimeout(t.Context(), rebootTimeout)
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
	ctx := t.Context()
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

			if tc.ipv4 {
				selectedPrefixes = selectPercentagePrefixes(ipv4Prefixes, tc.matchPercent)
			} else {
				selectedPrefixes = selectPercentagePrefixes(ipv6Prefixes, tc.matchPercent)
			}

			wantPrefixes := make(map[string]bool)
			for _, pfx := range selectedPrefixes {
				wantPrefixes[pfx] = true
			}

			if tc.ipv4 {
				configureGlobalFilterPolicies(t, dut, tc.policyName, tc.policyName, deviations.DefaultNetworkInstance(dut))
			} else {
				configureGlobalFilterPolicies(t, dut, tc.policyName, tc.policyName, deviations.DefaultNetworkInstance(dut))
			}

			// ------------------------------------------------------------
			// Create subscriptions
			// ------------------------------------------------------------

			gnmiClient1, err := dut.RawAPIs().BindingDUT().DialGNMI(ctx)
			if err != nil {
				t.Fatalf("Failed to dial GNMI client1: %v", err)
			}

			gnmiClient2, err := dut.RawAPIs().BindingDUT().DialGNMI(ctx)
			if err != nil {
				t.Fatalf("Failed to dial GNMI client2: %v", err)
			}

			aftSession1 := aftcache.NewAFTStreamSession(ctx, t, gnmiClient1, dut)
			aftSession2 := aftcache.NewAFTStreamSession(ctx, t, gnmiClient2, dut)
			stoppingCondition := aftcache.InitialSyncStoppingCondition(t, dut, wantPrefixes, nil, nil)

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
		mustConfigureStaticRoute(t, dut, batch, deviations.DefaultNetworkInstance(dut), prefix, fmt.Sprintf("v4-%d", idx), atePort1.IPv4)
	}

	for idx, prefix := range ipv6Prefixes {
		mustConfigureStaticRoute(t, dut, batch, deviations.DefaultNetworkInstance(dut), prefix, fmt.Sprintf("v6-%d", idx), atePort1.IPv6)
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

	ctx := t.Context()

	// ------------------------------------------------------------
	// Configure NI-specific AFT filters
	// ------------------------------------------------------------

	t.Log("Configuring per-network-instance AFT filters")

	configureGlobalFilterPolicies(t, dut, ipv4Policy, ipv4Policy, deviations.DefaultNetworkInstance(dut))

	configureGlobalFilterPolicies(t, dut, vrfAPolicy, vrfAPolicy, vrfName)

	// ------------------------------------------------------------
	// Expected prefixes
	// ------------------------------------------------------------

	defaultWant := map[string]bool{
		prefixAft1: true,
		prefixAft2: true,
	}

	vrfWant := map[string]bool{
		vrfPfx1: true,
	}

	// ------------------------------------------------------------
	// Collector-1 (DEFAULT)
	// ------------------------------------------------------------

	gnmiClient1, err := dut.RawAPIs().BindingDUT().DialGNMI(ctx)
	if err != nil {
		t.Fatalf("Failed to dial GNMI client1: %v", err)
	}

	collector1 := aftcache.NewAFTStreamSession(ctx, t, gnmiClient1, dut)

	// ------------------------------------------------------------
	// Collector-2 (VRF-A)
	// ------------------------------------------------------------

	gnmiClient2, err := dut.RawAPIs().BindingDUT().DialGNMI(ctx)
	if err != nil {
		t.Fatalf("Failed to dial GNMI client2: %v", err)
	}

	collector2 := aftcache.NewAFTStreamSession(ctx, t, gnmiClient2, dut)

	// ------------------------------------------------------------
	// Initial sync validation
	// ------------------------------------------------------------

	t.Log("Validating initial filtered AFT state")

	defaultStop := aftcache.InitialSyncStoppingCondition(t, dut, defaultWant, nil, nil)
	vrfStop := aftcache.InitialSyncStoppingCondition(t, dut, vrfWant, nil, nil)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		collector1.ListenUntil(ctx, t, subscriptionWait, defaultStop)
	}()

	go func() {
		defer wg.Done()
		collector2.ListenUntil(ctx, t, subscriptionWait, vrfStop)
	}()

	wg.Wait()

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

	verifyPrefixesPresent(t, defaultAFT, []string{prefixAft1, prefixAft2})

	verifyPrefixesAbsent(t, defaultAFT, []string{pfxAbsent})

	// ------------------------------------------------------------
	// Validate Collector-2
	// ------------------------------------------------------------

	verifyPrefixesPresent(t, vrfAFT, []string{vrfPfx1})

	verifyPrefixesAbsent(t, vrfAFT, []string{prefixAft1, prefixAft2})

	// ------------------------------------------------------------
	// Add unmatched route to DEFAULT
	// Neither collector should receive it
	// ------------------------------------------------------------

	t.Log("Adding unmatched route to DEFAULT")
	mustAddSingleStaticRoute(t, dut, deviations.DefaultNetworkInstance(dut), "100.64.2.0/24", "1000", atePort1.IPv4)

	mustValidatePrefixAbsentFromCollectors(t, dut, collector1, collector2, "100.64.2.0/24")

	// ------------------------------------------------------------
	// Add unmatched route
	// ------------------------------------------------------------

	t.Log("Adding unmatched route to DEFAULT")
	mustAddSingleStaticRoute(t, dut, deviations.DefaultNetworkInstance(dut), "203.0.113.64/28", "1001", atePort1.IPv4)

	mustValidatePrefixAbsentFromCollectors(t, dut, collector1, collector2, "203.0.113.64/28")

	// ------------------------------------------------------------
	// Add matched exact route to DEFAULT
	// Collector1 should receive it
	// ------------------------------------------------------------

	t.Log("Adding matched route to DEFAULT")
	mustAddSingleStaticRoute(t, dut, deviations.DefaultNetworkInstance(dut), "198.51.100.1/32", "1002", atePort1.IPv4)

	mustVerifyPrefixEventuallyPresent(t, dut, collector1, "198.51.100.1/32")

	mustVerifyPrefixAbsent(t, dut, collector2, "198.51.100.1/32")

	// ------------------------------------------------------------
	// Add matched subnet route to VRF-A
	// Collector2 should receive it
	// ------------------------------------------------------------

	t.Log("Adding matched subnet route to VRF-A")

	mustAddSingleStaticRoute(t, dut, vrfName, "100.64.1.128/25", "1003", atePort1.IPv4)

	mustVerifyPrefixEventuallyPresent(t, dut, collector2, "100.64.1.128/25")

	mustVerifyPrefixAbsent(t, dut, collector1, "100.64.1.128/25")

	// ------------------------------------------------------------
	// Change VRF-A policy to MATCH-ALL
	// ------------------------------------------------------------

	t.Log("Changing VRF-A policy to POLICY-MATCH-ALL")

	configureGlobalFilterPolicies(t, dut, matchAllPolicy, matchAllPolicy, vrfName)
	// ------------------------------------------------------------
	// Collector1 should remain stable
	// ------------------------------------------------------------

	defaultAFTAfter, err := collector1.ToAFT(t, dut)
	if err != nil {
		t.Fatalf("Collector1 unexpectedly failed after policy change: %v", err)
	}

	verifyPrefixesPresent(t, defaultAFTAfter,
		[]string{
			"198.51.100.0/24",
			"203.0.113.0/28",
			"198.51.100.1/32",
		},
	)

	verifyPrefixesAbsent(t, defaultAFTAfter,
		[]string{
			"100.64.2.0/24",
			"203.0.113.64/28",
		},
	)

	// ------------------------------------------------------------
	// Collector2 should now receive all VRF-A routes
	// ------------------------------------------------------------

	t.Log("Waiting for Collector2 to receive all VRF-A routes")

	wantAllVRF := []string{
		"198.51.100.0/24",
		"100.64.1.0/24",
		"203.0.113.128/28",
		"100.64.1.128/25",
	}

	mustVerifyPrefixesEventuallyPresent(t, dut, collector2, wantAllVRF, 60*time.Second)

	t.Log("Per-network-instance filtering validation completed successfully")
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
			t.Fatalf("Expected prefix missing: %s", pfx)
		}
	}
}

// verifyPrefixesAbsent validates prefixes do not exist.
func verifyPrefixesAbsent(t *testing.T, aft *aftcache.AFTData, prefixes []string) {
	t.Helper()
	for _, pfx := range prefixes {
		if _, ok := aft.Prefixes[pfx]; ok {
			t.Fatalf("Unexpected prefix present: %s", pfx)
		}
	}
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

// mustVerifyPrefixEventuallyPresent waits until prefix appears.
func mustVerifyPrefixEventuallyPresent(t *testing.T, dut *ondatra.DUTDevice, session *aftcache.AFTStreamSession, prefix string) {
	t.Helper()
	_, ok := gnmi.Watch(t, dut, gnmi.OC().System().State(), 60*time.Second,
		func(val *ygnmi.Value[*oc.System]) bool {
			aft, err := session.ToAFT(t, dut)
			if err != nil {
				return false
			}

			_, present := aft.Prefixes[prefix]
			return present
		},
	).Await(t)

	if !ok {
		t.Fatalf("Timed out waiting for prefix %s", prefix)
	}

	t.Logf("Observed expected prefix %s", prefix)
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
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		aft, err := session.ToAFT(t, dut)
		if err != nil {

			// acceptable if stream restarted after policy change
			if isExpectedReconnectError(err) {
				t.Logf("Collector stream restarted after policy change: %v", err)
				continue
			}
			t.Fatalf("Unexpected ToAFT error: %v", err)
		}

		allPresent := true
		for _, pfx := range prefixes {
			if _, ok := aft.Prefixes[pfx]; !ok {
				allPresent = false
				break
			}
		}

		if allPresent {
			t.Log("All expected prefixes received")
			return
		}
	}
	t.Fatalf("Timed out waiting for expected prefixes")
}
