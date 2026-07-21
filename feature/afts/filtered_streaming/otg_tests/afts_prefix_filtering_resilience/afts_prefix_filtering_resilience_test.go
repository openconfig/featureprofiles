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
	aftpf "github.com/openconfig/featureprofiles/internal/aft_prefix_filtering"
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
	v4PfxSet           = "PREFIX-SET-A"
	ipv4Policy         = "POLICY-PREFIX-SET-A"
	ipv6Policy         = "POLICY-PREFIX-SET-A"
	matchAllPolicy     = "POLICY-MATCH-ALL"
	vrfAPolicy         = "POLICY-PREFIX-SET-VRF-A"
	vrfPrefixName      = "PREFIX-SET-VRF-A"
	scaleIPv4Routes    = 5000
	scaleIPv6Routes    = 2000
	scaleSyncDeadline  = 5 * time.Minute
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
	scaleVrfV4Pfx        = "198.19.0.0"
	scaleVrfV6Pfx        = "2001:db8:1000::"
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
	pfxCount             = 1
	aftFilterDUTAS       = 65001
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
	if deviations.GetRetainGnmiCfgAfterReboot(dut) {
		if err := aftpf.ConfigureToStoreRunningGNMIConfig(t, dut); err != nil {
			t.Fatalf("failed to configure DUT to store running gNMI config: %v", err)
		}
		defer aftpf.UnconfigureToStoreRunningGNMIConfig(t, dut)
	}
	batch := configureDUT(t, dut)
	configurePolicies(t, dut, batch)
	aftpf.ConfigureNetworkInstanceStaticRoute(t, dut, batch, aftpf.NetworkInstanceStaticRouteParams{
		DefaultPrefixes:     defaultIPv4Prefixes,
		VRFPrefixes:         vrfV4Prefixes,
		DefaultNextHop:      atePort1.IPv4,
		VRFNextHop:          atePort2.IPv4,
		StartIndex:          staticRouteIndex,
		DefaultInstanceName: deviations.DefaultNetworkInstance(dut),
		VRFInstanceName:     vrfName})
	aftpf.ConfigureNetworkInstanceStaticRoute(t, dut, batch, aftpf.NetworkInstanceStaticRouteParams{
		DefaultPrefixes:     defaultIPv6Prefixes,
		VRFPrefixes:         vrfV6Prefixes,
		DefaultNextHop:      atePort1.IPv6,
		VRFNextHop:          atePort2.IPv6,
		StartIndex:          staticRouteIndex + 100,
		DefaultInstanceName: deviations.DefaultNetworkInstance(dut),
		VRFInstanceName:     vrfName})
	topo, interfaceNamesList := configureATE(t, ate)
	ate.OTG().PushConfig(t, topo)
	ate.OTG().StartProtocols(t)
	cfgplugins.IsIPv4InterfaceARPresolved(t, ate, cfgplugins.AddressFamilyParams{InterfaceNames: interfaceNamesList})
	cfgplugins.IsIPv6InterfaceARPresolved(t, ate, cfgplugins.AddressFamilyParams{InterfaceNames: interfaceNamesList})
	aftpf.AFTAwaitScaleBGPConvergence(t, dut, aftpf.AFTBGPConvergenceParams{
		NetworkInstance: deviations.DefaultNetworkInstance(dut),
		V4Neighbor:      atePort1.IPv4, V6Neighbor: atePort1.IPv6,
		V4RouteCount: scaleIPv4Routes, V6RouteCount: scaleIPv6Routes,
	})
	aftpf.AFTAwaitScaleBGPConvergence(t, dut, aftpf.AFTBGPConvergenceParams{
		NetworkInstance: vrfName,
		V4Neighbor:      atePort2.IPv4, V6Neighbor: atePort2.IPv6,
		V4RouteCount: scaleIPv4Routes, V6RouteCount: scaleIPv6Routes,
	})
	tests := []struct {
		name string
		test func(t *testing.T, dut *ondatra.DUTDevice)
	}{
		{
			name: "AFT-6.3.1-ValidationAfterDeviceReboot",
			test: testAfterReboot,
		},
		{
			name: "AFT-6.3.2-ScaleTest",
			test: testScaleFiltering,
		},
		{
			name: "AFT-6.3.3-PerNetworkInstanceFiltering",
			test: testPerNIFiltering,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Log("Starting test")
			tc.test(t, dut)
			t.Log("Finished test")
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
	configureDUTInterface(t, dut, batch, &dutPort1, p1)
	configureDUTInterface(t, dut, batch, &dutPort2, p2)
	cfgplugins.EnableDefaultNetworkInstanceBgp(t, dut, aftFilterDUTAS)
	defaultNI := cfgplugins.ConfigureNetworkInstance(t, dut, deviations.DefaultNetworkInstance(dut), true)
	nonDefaultNI := cfgplugins.ConfigureNetworkInstance(t, dut, vrfName, false)
	configureScaleBGP(t, dut, defaultNI, nonDefaultNI)
	cfgplugins.UpdateNetworkInstanceOnDut(t, dut, defaultNI.GetName(), defaultNI)
	cfgplugins.UpdateNetworkInstanceOnDut(t, dut, vrfName, nonDefaultNI)
	configureNetworkInstanceOnDUTPort(t, dut, batch, &dutPort2, p2, vrfName)
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

// configureNetworkInstanceOnDUTPort configure DUT ports.
func configureNetworkInstanceOnDUTPort(t *testing.T, dut *ondatra.DUTDevice, batch *gnmi.SetBatch, attrs *attrs.Attributes, p *ondatra.Port, niName string) {
	t.Helper()
	d := gnmi.OC()
	cfgplugins.AssignToNetworkInstance(t, dut, p.Name(), niName, 0)
	i := attrs.NewOCInterface(p.Name(), dut)
	gnmi.BatchUpdate(batch, d.Interface(p.Name()).Config(), i)
}

// configureHardwareInit sets up the initial hardware configuration on the DUT. It pushes hardware initialization configs for VRF Selection Extended feature and Policy Forwarding feature.
// TODO: The TCAM profile needs to be updated for the VRF configuration. I will remove it if its no longer needed after the global filter validation is complete.
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
	dev1 := atePort1.AddToOTG(topo, p1, &dutPort1)
	dev2 := atePort2.AddToOTG(topo, p2, &dutPort2)
	// Advertise the scaled routes from the ATE instead of installing them as static routes: DEFAULT scale over port1 and VRF-A scale over port2.
	aftpf.AFTConfigureATEScaleBGP(t, dev1, aftpf.AFTATEBGPParams{
		DUTPort: dutPort1, ATEPort: atePort1, NamePrefix: "default-scale",
		V4RouteCount: scaleIPv4Routes, V4BaseAddr: scaleV4Pfx, V4PrefixLen: scaleV4PfxLen,
		V6RouteCount: scaleIPv6Routes, V6BaseAddr: scaleV6Pfx, V6PrefixLen: scaleV6PfxLen,
	})
	aftpf.AFTConfigureATEScaleBGP(t, dev2, aftpf.AFTATEBGPParams{
		DUTPort: dutPort2, ATEPort: atePort2, NamePrefix: "vrfa-scale",
		V4RouteCount: scaleIPv4Routes, V4BaseAddr: scaleVrfV4Pfx, V4PrefixLen: scaleV4PfxLen,
		V6RouteCount: scaleIPv6Routes, V6BaseAddr: scaleVrfV6Pfx, V6PrefixLen: scaleV6PfxLen,
	})
	// Collect interface/device names
	for _, dev := range topo.Devices().Items() {
		interfaceNamesList = append(
			interfaceNamesList,
			dev.Name(),
		)
	}
	return topo, interfaceNamesList
}

// configureScaleBGP configures dual-AFI eBGP peerings in the DEFAULT (port1)
// and VRF-A (port2) network instances so that the ATE-advertised scale routes
// are learned and installed in both instances.
func configureScaleBGP(t *testing.T, dut *ondatra.DUTDevice, defaultNI, nonDefaultNI *oc.NetworkInstance) {
	t.Helper()
	aftpf.AFTConfigureScaleBGP(t, dut, aftpf.AFTBGPParams{
		NetworkInstance: defaultNI,
		RouterID:        dutPort1.IPv4,
		V4Neighbor:      atePort1.IPv4,
		V6Neighbor:      atePort1.IPv6,
	})
	aftpf.AFTConfigureScaleBGP(t, dut, aftpf.AFTBGPParams{
		NetworkInstance: nonDefaultNI,
		RouterID:        dutPort2.IPv4,
		V4Neighbor:      atePort2.IPv4,
		V6Neighbor:      atePort2.IPv6,
	})
}

// fetchAFT collects AFT telemetry from two independent sessions and validates consistency between them.
func fetchAFT(t *testing.T, dut *ondatra.DUTDevice, aftSession1, aftSession2 *aftcache.AFTStreamSession, stoppingCondition aftcache.PeriodicHook, wantPrefixes map[string]bool) (*aftcache.AFTData, error) {
	t.Helper()
	aftpf.RunCollector(t, aftpf.RunCollectorParams{Ctx: context.Background(), Collector: aftSession1, Stop: stoppingCondition, Timeout: aftConvergenceTime})
	aftpf.RunCollector(t, aftpf.RunCollectorParams{Ctx: context.Background(), Collector: aftSession2, Stop: stoppingCondition, Timeout: aftConvergenceTime})
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

// testAfterReboot validates that AFT filtering policies and filtered entries
// are preserved across a DUT reboot.
func testAfterReboot(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wantPrefixes := aftpf.GeneratePrefixes(t, aftpf.GeneratePrefixesParams{V4Prefixes: []string{matchPrefixAft1, matchPrefixAft2}, V6Prefixes: nil, PfxCount: pfxCount})
	// Prefixes advertised by the ATE but expected to be filtered out.
	unexpectedPrefixes := generateUnexpectedPrefixes([]string{matchPrefixAft1, matchPrefixAft2}, []string{matchPrefixAft1, matchPrefixAft2})
	// Verify configured policies before reboot.
	verifyGlobalFilterPolicies(t, dut, ipv4Policy, ipv6Policy)
	// Establish initial gNMI subscriptions.
	aftSession1 := aftcache.NewAFTStreamSession(ctx, t, aftpf.GnmiClientSession(t, dut, aftpf.PrefixesParams{Ctx: ctx}), dut)
	aftSession2 := aftcache.NewAFTStreamSession(ctx, t, aftpf.GnmiClientSession(t, dut, aftpf.PrefixesParams{Ctx: ctx}), dut)
	t.Log("Collecting initial filtered AFT entries")
	stoppingCondition := aftcache.InitialSyncStoppingCondition(t, dut, wantPrefixes, map[string]bool{atePort1.IPv4: true}, nil)
	aftBefore, err := fetchAFT(t, dut, aftSession1, aftSession2, stoppingCondition, wantPrefixes)
	if err != nil {
		t.Fatalf("Failed to fetch initial AFT: %v", err)
	}
	verifyFilteredPrefixes(t, aftBefore, wantPrefixes, unexpectedPrefixes, true)
	// Get initial boot time.
	lastBootTime, err := bootTime(t, dut)
	if err != nil {
		t.Fatalf("Failed to get boot time: %v", err)
	}
	t.Log("Rebooting DUT")
	rebootDUT(t, dut)
	// Wait for DUT recovery.
	waitForReboot(t, dut, lastBootTime)
	// Verify policy persistence after reboot.
	verifyGlobalFilterPolicies(t, dut, ipv4Policy, ipv6Policy)
	// Re-establish subscriptions.
	t.Log("Re-establishing AFT subscriptions")
	aftSession3 := aftcache.NewAFTStreamSession(ctx, t, aftpf.GnmiClientSession(t, dut, aftpf.PrefixesParams{Ctx: ctx}), dut)
	aftSession4 := aftcache.NewAFTStreamSession(ctx, t, aftpf.GnmiClientSession(t, dut, aftpf.PrefixesParams{Ctx: ctx}), dut)
	stoppingCondition2 := aftcache.InitialSyncStoppingCondition(t, dut, wantPrefixes, map[string]bool{atePort1.IPv4: true}, nil)
	aftAfter, err := fetchAFT(t, dut, aftSession3, aftSession4, stoppingCondition2, wantPrefixes)
	if err != nil {
		t.Fatalf("Failed to fetch AFT after reboot: %v", err)
	}
	// Verify filtered entries after reboot.
	verifyFilteredPrefixes(t, aftAfter, wantPrefixes, unexpectedPrefixes, true)
	t.Log("AFT reboot validation completed successfully")
}

// verifyGlobalFilterPolicies verifies global-filter IPv4/IPv6 policies persisted.
func verifyGlobalFilterPolicies(t *testing.T, dut *ondatra.DUTDevice, wantIPv4Policy, wantIPv6Policy string) {
	t.Helper()
	if deviations.AftsGlobalFilterPolicyOCUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			aftpf.VerifyGlobalFilterPoliciesCLI(t, dut, aftpf.ConfigureGlobalFilterPoliciesParams{V4Policy: wantIPv4Policy, V6Policy: wantIPv6Policy})
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
				gnmiPath(t, ocPolicyPathV4),
				gnmiPath(t, ocPolicyPathV6),
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

// gnmiPath converts a string-based gNMI path into a structured gNMI Path.
func gnmiPath(t *testing.T, path string) *gpb.Path {
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

// configurePolicies configures all routing policies required for AFT filtering
// validation, including base policies and scale filtering policies. It also
// applies the default global filter policies and pushes the routing policy
// configuration to the DUT.
func configurePolicies(t *testing.T, dut *ondatra.DUTDevice, batch *gnmi.SetBatch) {
	t.Helper()
	root := &oc.Root{}
	rp := root.GetOrCreateRoutingPolicy()
	ipv4ScalePrefixes := generateScaleIPv4Prefixes(t)
	ipv6ScalePrefixes := generateScaleIPv6Prefixes(t)
	configureBasePolicies(t, rp)
	configureScalePolicies(t, rp, ipv4ScalePrefixes, ipv6ScalePrefixes)
	aftpf.ConfigureGlobalFilterPolicies(t, dut,
		aftpf.ConfigureGlobalFilterPoliciesParams{
			V4Policy: ipv4Policy,
			V6Policy: ipv6Policy,
			VRFName:  deviations.DefaultNetworkInstance(dut),
		})
	gnmi.BatchReplace(batch, gnmi.OC().RoutingPolicy().Config(), rp)
	batch.Set(t, dut)
}

// configureBasePolicies configures the common routing policies used by the
// filtering tests. This includes match-all policies, IPv4/IPv6 prefix matching
// policies, subnet policies, multi-statement policies, VRF policies, and other
// policy scenarios validated by the test suite.
func configureBasePolicies(t *testing.T, rp *oc.RoutingPolicy) {
	t.Helper()
	policies := []aftpf.PrefixSetPolicyParams{
		{
			PolicyName:     matchAllPolicy,
			StatementNames: []string{defaultStatementName},
			PolicyResult:   oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
		},
		{
			PolicyName:     ipv4Policy,
			StatementNames: []string{defaultStatementName},
			PrefixSetNames: []string{v4PfxSet},
			PrefixList:     policyIPv4Prefixes,
			PrefixMode:     pfxMode,
			PolicyResult:   oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
		},
		{
			PolicyName:     ipv6Policy,
			StatementNames: []string{"20"},
			PrefixSetNames: []string{v4PfxSet},
			PrefixList:     policyIPv6Prefixes,
			PrefixMode:     pfxMode,
			PolicyResult:   oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
		},
		{
			PolicyName:     "POLICY-SUBNET-V4",
			StatementNames: []string{defaultStatementName},
			PrefixSetNames: []string{"PREFIX-SET-SUBNET-V4"},
			MatchPrefixSet: true,
			PolicyResult:   oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
		},
		{
			PolicyName:     "POLICY-SUBNET-V6",
			StatementNames: []string{defaultStatementName},
			PrefixSetNames: []string{"PREFIX-SET-SUBNET-V6"},
			MatchPrefixSet: true,
			PolicyResult:   oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
		},
		{
			PolicyName:     "POLICY-MULTI-STMT",
			StatementNames: []string{defaultStatementName, "20"},
			PrefixSetNames: []string{"PREFIX-SET-A", "PREFIX-SET-SUBNET"},
			MatchPrefixSet: true,
			PolicyResult:   oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
		},
		{
			PolicyName:     "POLICY-DENY-PREFIX-SET-A",
			StatementNames: []string{defaultStatementName, "20"},
			PrefixSetNames: []string{"PREFIX-SET-A", ""},
			MatchPrefixSet: true,
			PrefixDeny:     true,
			PolicyResult:   oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
		},
		{
			PolicyName:     "POLICY-TAG-MATCH",
			StatementNames: []string{defaultStatementName},
			MatchPrefixSet: true,
			SetTag:         true,
			PolicyResult:   oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
		},
	}
	for _, policy := range policies {
		aftpf.AddPrefixSetPolicy(t, rp, policy)
	}
	// VRF policies require separate IPv4/IPv6 prefix modes.
	aftpf.AddPrefixSetPolicy(t, rp,
		aftpf.PrefixSetPolicyParams{
			PolicyName:     "POLICY-PREFIX-SET-VRF-A",
			StatementNames: []string{defaultStatementName},
			PrefixSetNames: []string{vrfPrefixName},
			PrefixList:     []string{vrfV4Pfx},
			PrefixMode:     pfxV4MaskRange,
			PolicyResult:   oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
		})
	aftpf.AddPrefixSetPolicy(t, rp,
		aftpf.PrefixSetPolicyParams{
			PolicyName:     "POLICY-PREFIX-SET-VRF-A",
			StatementNames: []string{"20"},
			PrefixSetNames: []string{vrfPrefixName},
			PrefixList:     []string{vrfV6Pfx},
			PrefixMode:     pfxV6MaskRange,
			PolicyResult:   oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
		})
}

// configureScalePolicies configures routing policies used for scale filtering
// validation. It creates IPv4 and IPv6 policies for different prefix match
// percentages and associates each policy with its corresponding prefix-set.
func configureScalePolicies(t *testing.T, rp *oc.RoutingPolicy, ipv4Prefixes, ipv6Prefixes []string) {
	t.Helper()
	for _, percent := range []int{1, 5, 20} {
		scalePolicies := []aftpf.PrefixSetPolicyParams{
			{
				PolicyName:     fmt.Sprintf("POLICY-SCALE-IPV4-%d", percent),
				StatementNames: []string{defaultStatementName},
				PrefixSetNames: []string{fmt.Sprintf("PREFIX-SCALE-IPV4-%d", percent)},
				PrefixList:     selectPercentagePrefixes(ipv4Prefixes, percent),
				PrefixMode:     pfxMode,
				MatchPrefixSet: true,
				PolicyResult:   oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
			},
			{
				PolicyName:     fmt.Sprintf("POLICY-SCALE-IPV6-%d", percent),
				StatementNames: []string{defaultStatementName},
				PrefixSetNames: []string{fmt.Sprintf("PREFIX-SCALE-IPV6-%d", percent)},
				PrefixList:     selectPercentagePrefixes(ipv6Prefixes, percent),
				PrefixMode:     pfxMode,
				MatchPrefixSet: true,
				PolicyResult:   oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
			},
		}
		for _, policy := range scalePolicies {
			aftpf.AddPrefixSetPolicy(t, rp, policy)
		}
	}
}

// generateScaleIPv4Prefixes generates the IPv4 prefix list used by scale
// filtering policies. The prefixes are generated using the configured scale
// route count, prefix length, and address step values.
func generateScaleIPv4Prefixes(t *testing.T) []string {
	t.Helper()
	ips, err := iputil.GenerateIPsWithStep(scaleV4Pfx, scaleIPv4Routes, intStepV4)
	if err != nil {
		t.Fatalf("failed generating IPv4 prefixes: %v", err)
	}
	prefixes := make([]string, 0, len(ips))
	for _, ip := range ips {
		prefixes = append(prefixes, fmt.Sprintf("%s/%d", ip, scaleV4PfxLen))
	}
	return prefixes
}

// generateScaleIPv6Prefixes generates the IPv6 prefix list used by scale
// filtering policies. The prefixes are generated using the configured scale
// route count, prefix length, and IPv6 address step values.
func generateScaleIPv6Prefixes(t *testing.T) []string {
	t.Helper()
	ips, err := iputil.GenerateIPv6sWithStep(scaleV6Pfx, scaleIPv6Routes, intStepV6)
	if err != nil {
		t.Fatalf("failed generating IPv6 prefixes: %v", err)
	}
	prefixes := make([]string, 0, len(ips))
	for _, ip := range ips {
		prefixes = append(prefixes,
			fmt.Sprintf("%s/%d", ip, scaleV6PfxLen))
	}
	return prefixes
}

// bootTime returns the system boot time reported by the DUT.
func bootTime(t *testing.T, dut *ondatra.DUTDevice) (uint64, error) {
	t.Helper()
	var bootTime uint64
	_, ok := gnmi.Watch(t, dut, gnmi.OC().System().BootTime().State(), scaleSyncDeadline, func(val *ygnmi.Value[uint64]) bool {
		var ok bool
		bootTime, ok = val.Val()
		return ok
	}).Await(t)
	if !ok {
		return 0, fmt.Errorf("failed to get boot time")
	}
	return bootTime, nil
}

// waitForReboot waits for the DUT to become unreachable, come back online, and report a boot time newer than the previous boot time.
func waitForReboot(t *testing.T, dut *ondatra.DUTDevice, lastBootTime uint64) {
	t.Helper()
	startReboot := time.Now()
	t.Logf("Wait for DUT to boot up by polling the telemetry output.")
	{
		ticker := time.NewTicker(rebootPollInterval)
		defer ticker.Stop()
		timeout := time.After(maxRebootTime)
		var deviceWentDown bool
	rebootLoop:
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
						t.Logf("Device is now unreachable. Waiting for it to come back up.")
						deviceWentDown = true
					}
					t.Logf("Time elapsed %.2f seconds, DUT not reachable yet: %s.", time.Since(startReboot).Seconds(), *errMsg)
				} else {
					if deviceWentDown {
						t.Logf("Device rebooted successfully with received time: %v.", currentTime)
						break rebootLoop
					}
					t.Logf("Device is still reachable; reboot hasn't started yet.")
				}
			}
		}
	}
	t.Logf("Device boot time: %.2f seconds.", time.Since(startReboot).Seconds())
	t.Logf("Wait for DUT to boot up by polling the telemetry output.")
	_, err := dut.RawAPIs().BindingDUT().DialGNMI(t.Context())
	if err != nil {
		t.Fatalf("Failed to dial GNMI after reboot: %v", err)
	}
	// Wait for boot time to change.
	_, ok := gnmi.Watch(t, dut, gnmi.OC().System().BootTime().State(), maxRebootTime, bootTimePredicate(lastBootTime)).Await(t)
	if !ok {
		currentBootTime, _ := bootTime(t, dut)
		t.Fatalf("Boot time did not update after reboot. Current: %d, Last: %d", currentBootTime, lastBootTime)
	}
	t.Logf("Device boot time: %.2f seconds.", time.Since(startReboot).Seconds())
	currentBootTime, _ := bootTime(t, dut)
	t.Logf("Boot time successfully changed from %d to %d", lastBootTime, currentBootTime)
}

// bootTimePredicate returns a predicate that evaluates to true when the DUT reports a boot time greater than the provided previous boot time.
func bootTimePredicate(lastBootTime uint64) func(val *ygnmi.Value[uint64]) bool {
	return func(val *ygnmi.Value[uint64]) bool {
		currentBootTime, ok := val.Val()
		return ok && currentBootTime > lastBootTime
	}
}

// rebootDUT performs gNOI reboot.
func rebootDUT(t *testing.T, dut *ondatra.DUTDevice) {
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

// testScaleFiltering validates AFT filtering behavior under scale.
//
// Test flow:
//  1. Populate AFT with large-scale IPv4/IPv6 routes advertised via BGP.
//  2. Configure policies matching approximately 1%, 5%, and 20%.
//  3. Establish dual AFT subscriptions.
//  4. Measure synchronization time.
//  5. Verify synchronization completes within deadline.
//  6. Verify only expected filtered prefixes are streamed.
func testScaleFiltering(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var ipv4Prefixes, ipv6Prefixes []string
	// Enumerate the scale routes advertised via BGP from the ATE
	t.Logf("Using %d IPv4 and %d IPv6 BGP-advertised scale routes", scaleIPv4Routes, scaleIPv6Routes)
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
	// Policy scenarios
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
			// Select expected prefixes
			var selectedPrefixes, allPrefixes []string
			var stoppingCondition aftcache.PeriodicHook
			if tc.ipv4 {
				allPrefixes = ipv4Prefixes
				selectedPrefixes = selectPercentagePrefixes(ipv4Prefixes, tc.matchPercent)
			} else {
				allPrefixes = ipv6Prefixes
				selectedPrefixes = selectPercentagePrefixes(ipv6Prefixes, tc.matchPercent)
			}
			wantPrefixes := aftpf.GeneratePrefixes(t, aftpf.GeneratePrefixesParams{V4Prefixes: selectedPrefixes, V6Prefixes: nil, PfxCount: pfxCount})
			unexpectedPrefixes := generateUnexpectedPrefixes(allPrefixes, selectedPrefixes)
			// Create subscriptions
			aftSession1 := aftcache.NewAFTStreamSession(ctx, t, aftpf.GnmiClientSession(t, dut, aftpf.PrefixesParams{Ctx: ctx}), dut)
			aftSession2 := aftcache.NewAFTStreamSession(ctx, t, aftpf.GnmiClientSession(t, dut, aftpf.PrefixesParams{Ctx: ctx}), dut)
			if tc.ipv4 {
				aftpf.ConfigureGlobalFilterPolicies(t, dut, aftpf.ConfigureGlobalFilterPoliciesParams{V4Policy: tc.policyName, V6Policy: "", VRFName: deviations.DefaultNetworkInstance(dut)})
				stoppingCondition = aftcache.InitialSyncStoppingCondition(t, dut, wantPrefixes, map[string]bool{atePort1.IPv4: true}, nil)
			} else {
				aftpf.ConfigureGlobalFilterPolicies(t, dut, aftpf.ConfigureGlobalFilterPoliciesParams{V4Policy: "", V6Policy: tc.policyName, VRFName: deviations.DefaultNetworkInstance(dut)})
				stoppingCondition = aftcache.InitialSyncStoppingCondition(t, dut, wantPrefixes, nil, map[string]bool{atePort1.IPv6: true})
			}
			// Measure synchronization time
			start := time.Now()
			aftData, err := fetchAFT(t, dut, aftSession1, aftSession2, stoppingCondition, wantPrefixes)
			if err != nil {
				t.Fatalf("Failed to fetch scaled AFT: %v", err)
			}
			syncDuration := time.Since(start)
			t.Logf("Synchronization completed in %v", syncDuration)
			// Verify synchronization time
			if syncDuration > scaleSyncDeadline {
				t.Fatalf("Synchronization exceeded limit got=%v want<=%v", syncDuration, scaleSyncDeadline)
			}
			// Verify filtering correctness
			verifyFilteredPrefixes(t, aftData, wantPrefixes, unexpectedPrefixes, tc.ipv4)
			t.Logf("Verified scale filtering for %s", tc.name)
		})
	}
}

// // verifyFilteredPrefixes validates that all expected prefixes are present in the received AFT data and that unexpected prefixes are absent.
//
//	func verifyFilteredPrefixes(t *testing.T, aftPrefixes *aftcache.AFTData, wantPrefixes map[string]bool, ipv4 bool) {
//		t.Helper()
//		// Validate IPv4 entries
//		if ipv4 {
//			for pfx := range wantPrefixes {
//				if _, ok := aftPrefixes.Prefixes[pfx]; !ok {
//					t.Fatalf("Expected IPv4 prefix missing from filtered AFT: %s", pfx)
//				}
//			}
//			for _, pfx := range unexpectedPrefixes {
//				if _, ok := aftPrefixes.Prefixes[pfx]; ok {
//					t.Fatalf("Unexpected IPv4 prefix present after filtering: %s", pfx)
//				}
//			}
//			t.Log("Verified IPv4 filtered prefixes")
//			return
//		}
//		// Validate IPv6 entries
//		for pfx := range wantPrefixes {
//			if _, ok := aftPrefixes.Prefixes[pfx]; !ok {
//				t.Fatalf("Expected IPv6 prefix missing from filtered AFT: %s", pfx)
//			}
//		}
//		for _, pfx := range unexpectedPrefixes {
//			if _, ok := aftPrefixes.Prefixes[pfx]; ok {
//				t.Fatalf("Unexpected IPv6 prefix present after filtering: %s", pfx)
//			}
//		}
//		t.Log("Verified IPv6 filtered prefixes")
//	}
//

// verifyFilteredPrefixes validates that the AFT contains all expected prefixes
// after applying the filter policy and does not contain prefixes that should
// have been filtered out.
func verifyFilteredPrefixes(t *testing.T, aftPrefixes *aftcache.AFTData, wantPrefixes map[string]bool, unexpectedPrefixes []string, ipv4 bool) {
	t.Helper()
	addressFamily := "IPv6"
	if ipv4 {
		addressFamily = "IPv4"
	}
	// Verify prefixes expected to pass the filter are present.
	for pfx := range wantPrefixes {
		if _, ok := aftPrefixes.Prefixes[pfx]; !ok {
			t.Fatalf("Expected %s prefix missing from filtered AFT: %s", addressFamily, pfx)
		}
	}
	// Verify prefixes expected to be filtered out are absent.
	for _, pfx := range unexpectedPrefixes {
		if _, ok := aftPrefixes.Prefixes[pfx]; ok {
			t.Fatalf("Unexpected %s prefix present after filtering: %s", addressFamily, pfx)
		}
	}
	t.Logf("Verified %s filtered prefixes", addressFamily)
}

// generateUnexpectedPrefixes returns prefixes that should not appear in the
// filtered AFT. These are prefixes advertised by the ATE but not selected by
// the filtering policy.
func generateUnexpectedPrefixes(allPrefixes, expectedPrefixes []string) []string {
	expected := make(map[string]bool)
	for _, pfx := range expectedPrefixes {
		expected[pfx] = true
	}
	var unexpected []string
	for _, pfx := range allPrefixes {
		if !expected[pfx] {
			unexpected = append(unexpected, pfx)
		}
	}
	return unexpected
}

// selectPercentagePrefixes selects percentage-based subset.
func selectPercentagePrefixes(prefixes []string, percent int) []string {
	count := (len(prefixes) * percent) / 100
	if count == 0 {
		count = 1
	}
	return prefixes[:count]
}

// testPerNIFiltering validates:
//
// 1. Independent AFT filtering per network-instance.
// 2. Multiple collectors subscribing simultaneously.
// 3. Correct filtered prefix visibility.
// 4. Dynamic route update propagation.
// 5. No leakage between collectors.
// 6. Policy-change behavior.
// 7. Collector stability during filter updates.
func testPerNIFiltering(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Configure NI-specific AFT filters
	t.Log("Configuring per-network-instance AFT filters")
	aftpf.ConfigureGlobalFilterPolicies(t, dut, aftpf.ConfigureGlobalFilterPoliciesParams{V4Policy: ipv4Policy, V6Policy: ipv6Policy, VRFName: deviations.DefaultNetworkInstance(dut)})
	aftpf.ConfigureGlobalFilterPolicies(t, dut, aftpf.ConfigureGlobalFilterPoliciesParams{V4Policy: vrfAPolicy, V6Policy: vrfAPolicy, VRFName: vrfName})
	// Expected prefixes
	defaultWant := aftpf.GeneratePrefixes(t, aftpf.GeneratePrefixesParams{V4Prefixes: []string{matchPrefixAft1, matchPrefixAft2}, V6Prefixes: nil, PfxCount: pfxCount})
	vrfWant := aftpf.GeneratePrefixes(t, aftpf.GeneratePrefixesParams{V4Prefixes: []string{matchPrefixAft1}, V6Prefixes: nil, PfxCount: pfxCount})
	// Collector-1 (DEFAULT)
	collector1 := aftcache.NewAFTStreamSession(ctx, t, aftpf.GnmiClientSession(t, dut, aftpf.PrefixesParams{Ctx: ctx}), dut)
	// Collector-2 (VRF-A)
	collector2 := aftcache.NewAFTStreamSession(ctx, t, aftpf.GnmiClientSession(t, dut, aftpf.PrefixesParams{Ctx: ctx}), dut)
	// Initial sync validation
	t.Log("Validating initial filtered AFT state")
	defaultStop := aftcache.InitialSyncStoppingCondition(t, dut, defaultWant, map[string]bool{atePort1.IPv4: true}, nil)
	vrfStop := aftcache.InitialSyncStoppingCondition(t, dut, vrfWant, map[string]bool{atePort2.IPv4: true}, nil)
	aftpf.RunCollector(t, aftpf.RunCollectorParams{Ctx: context.Background(), Collector: collector1, Stop: defaultStop, Timeout: subscriptionWait})
	aftpf.RunCollector(t, aftpf.RunCollectorParams{Ctx: context.Background(), Collector: collector2, Stop: vrfStop, Timeout: subscriptionWait})
	defaultAFT, err := collector1.ToAFT(t, dut)
	if err != nil {
		t.Fatalf("Collector1 ToAFT failed: %v", err)
	}
	vrfAFT, err := collector2.ToAFT(t, dut)
	if err != nil {
		t.Fatalf("Collector2 ToAFT failed: %v", err)
	}
	// Validate Collector-1
	aftpf.VerifyPrefixesPresent(t, aftpf.PrefixesParams{InfoAFT: defaultAFT, Prefixes: []string{matchPrefixAft1, matchPrefixAft2}})
	aftpf.VerifyPrefixesAbsent(t, aftpf.PrefixesParams{InfoAFT: defaultAFT, Prefixes: []string{matchPrefixAbsent}})
	// Validate Collector-2
	aftpf.VerifyPrefixesPresent(t, aftpf.PrefixesParams{InfoAFT: vrfAFT, Prefixes: []string{matchVrfPfx1}})
	aftpf.VerifyPrefixesAbsent(t, aftpf.PrefixesParams{InfoAFT: vrfAFT, Prefixes: []string{matchPrefixAft1, matchPrefixAft2}})
	// Add unmatched route to DEFAULT
	// Neither collector should receive it
	t.Log("Adding unmatched route to DEFAULT")
	aftpf.MustAddSingleStaticRoute(t, dut, aftpf.AddStaticRouteParams{NetworkInstanceName: deviations.DefaultNetworkInstance(dut), Prefix: v4AbsentPfx1, Index: fmt.Sprintf("%d", staticRouteIndex+900), NextHop: atePort1.IPv4})
	mustVerifyPrefixAbsent(t, dut, collector1, v4AbsentPfx1)
	mustVerifyPrefixAbsent(t, dut, collector2, v4AbsentPfx1)
	// Add unmatched route
	t.Log("Adding unmatched route to DEFAULT")
	aftpf.MustAddSingleStaticRoute(t, dut, aftpf.AddStaticRouteParams{NetworkInstanceName: deviations.DefaultNetworkInstance(dut), Prefix: v4AbsentPfx2, Index: fmt.Sprintf("%d", staticRouteIndex+901), NextHop: atePort1.IPv4})
	mustVerifyPrefixAbsent(t, dut, collector1, v4AbsentPfx2)
	mustVerifyPrefixAbsent(t, dut, collector2, v4AbsentPfx2)
	// Add matched exact route to DEFAULT
	// Collector1 should receive it
	t.Log("Adding matched route to DEFAULT")
	aftpf.MustAddSingleStaticRoute(t, dut, aftpf.AddStaticRouteParams{NetworkInstanceName: deviations.DefaultNetworkInstance(dut), Prefix: matchVrfPfx4, Index: fmt.Sprintf("%d", staticRouteIndex+902), NextHop: atePort1.IPv4})
	waitForPrefixesPresent(t, dut, collector1, []string{matchVrfPfx4}, subscriptionWait, atePort1.IPv4)
	mustVerifyPrefixAbsent(t, dut, collector2, matchVrfPfx4)
	// Add matched subnet route to VRF-A
	// Collector2 should receive it
	t.Log("Adding matched subnet route to VRF-A")
	aftpf.MustAddSingleStaticRoute(t, dut, aftpf.AddStaticRouteParams{NetworkInstanceName: vrfName, Prefix: matchVrfPfx2, Index: fmt.Sprintf("%d", staticRouteIndex+903), NextHop: atePort2.IPv4})
	waitForPrefixesPresent(t, dut, collector2, []string{matchVrfPfx2}, subscriptionWait, atePort2.IPv4)
	mustVerifyPrefixAbsent(t, dut, collector1, matchVrfPfx2)
	// Change VRF-A policy to MATCH-ALL
	t.Log("Changing VRF-A policy to POLICY-MATCH-ALL")
	aftpf.ConfigureGlobalFilterPolicies(t, dut, aftpf.ConfigureGlobalFilterPoliciesParams{V4Policy: matchAllPolicy, V6Policy: matchAllPolicy, VRFName: vrfName})
	// Collector1 should remain stable
	defaultAFTAfter, err := collector1.ToAFT(t, dut)
	if err != nil {
		t.Fatalf("Collector1 unexpectedly failed after policy change: %v", err)
	}
	aftpf.VerifyPrefixesPresent(t, aftpf.PrefixesParams{InfoAFT: defaultAFTAfter, Prefixes: policyIPv4Prefixes})
	aftpf.VerifyPrefixesAbsent(t, aftpf.PrefixesParams{InfoAFT: defaultAFTAfter, Prefixes: []string{v4AbsentPfx1, v4AbsentPfx2}})
	// Collector2 should now receive all VRF-A routes
	t.Log("Waiting for Collector2 to receive all VRF-A routes")
	wantAllVRF := []string{matchPrefixAft1, matchVrfPfx1, matchVrfPfx3, matchVrfPfx2}
	waitForPrefixesPresent(t, dut, collector2, wantAllVRF, subscriptionWait, atePort2.IPv4)
	t.Log("Per-network-instance filtering validation completed successfully")
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

// waitForPrefixesPresent validates all prefixes appear.
func waitForPrefixesPresent(t *testing.T, dut *ondatra.DUTDevice, session *aftcache.AFTStreamSession, prefixes []string, timeout time.Duration, nextHop string) {
	t.Helper()
	wantPrefixes := aftpf.GeneratePrefixes(t, aftpf.GeneratePrefixesParams{V4Prefixes: prefixes, V6Prefixes: nil, PfxCount: pfxCount})
	stoppingCondition := aftcache.InitialSyncStoppingCondition(t, dut, wantPrefixes, map[string]bool{nextHop: true}, nil)
	session.ListenUntil(context.Background(), t, timeout, stoppingCondition)
}
