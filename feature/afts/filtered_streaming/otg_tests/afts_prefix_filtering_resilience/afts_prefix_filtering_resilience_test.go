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
	"errors"
	"flag"
	"fmt"
	"strings"
	"sync"
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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	vrfName = "VRF-A"
	// v4PfxSet and v6PfxSet are kept as separate, address-family specific
	// prefix-sets. Mixing IPv4 and IPv6 prefixes in a single prefix-set is not
	// supported by all vendors.
	v4PfxSet           = "PREFIX-SET-A-V4"
	v6PfxSet           = "PREFIX-SET-A-V6"
	ipv4Policy         = "POLICY-PREFIX-SET-A"
	ipv6Policy         = "POLICY-PREFIX-SET-A"
	matchAllPolicy     = "POLICY-MATCH-ALL"
	vrfAPolicy         = "POLICY-PREFIX-SET-VRF-A"
	vrfPrefixNameV4    = "PREFIX-SET-VRF-A-V4"
	vrfPrefixNameV6    = "PREFIX-SET-VRF-A-V6"
	subscriptionWait   = 2 * time.Minute
	aftConvergenceTime = 10 * time.Minute
	// policyChangeWait is the maximum time allowed, per AFT-6.3.3, for a
	// collector to observe prefixes newly admitted by a global-filter change.
	policyChangeWait = 60 * time.Second
	// streamTerminationWait is the maximum time to wait for an active gNMI AFT
	// stream to terminate after the DUT reboot is issued.
	streamTerminationWait = 2 * time.Minute
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
	// unmatchedPrefixOffset is the offset, past the first un-matched prefix of a
	// scale pool, at which an additional un-matched prefix is verified to be
	// absent from the filtered AFT.
	unmatchedPrefixOffset = 10
	// aftsPath is the AFT subtree used for the raw gNMI subscription checks.
	aftsPath = "/network-instances/network-instance/afts"
)

var (
	// AFT-6.3.2 user adjustable values: X (IPv4 routes), Y (IPv6 routes) and
	// K (maximum allowed initial synchronization time).
	scaleIPv4Routes = flag.Int("scale_ipv4_routes", 5000, "Number of IPv4 routes (X) advertised from the ATE for the AFT-6.3.2 scale test.")
	scaleIPv6Routes = flag.Int("scale_ipv6_routes", 2000, "Number of IPv6 routes (Y) advertised from the ATE for the AFT-6.3.2 scale test.")
	// Synchronization exceeded limit with 120 seconds so increase to 180 seconds to avoid test failure. The actual convergence time is expected to be much lower than this limit.
	scaleSyncDeadline = flag.Duration("scale_sync_deadline", 180*time.Second, "Maximum allowed AFT initial synchronization time (K) for the AFT-6.3.2 scale test.")

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
	// rebootUnmatchedPrefixes are routes that ARE installed in the DUT's DEFAULT
	// network instance (see defaultIPv4Prefixes / defaultIPv6Prefixes) but are
	// deliberately NOT part of POLICY-PREFIX-SET-A (see policyIPv4Prefixes /
	// policyIPv6Prefixes). Their absence from the streamed AFT is what proves
	// the global filter is actually dropping un-matched routes.
	rebootUnmatchedPrefixes = []string{
		"100.64.0.0/24",
		"2001:db8:2::2/128",
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
	awaitScaleBGPConvergence(t, dut)
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

// configureHardwareInit sets up the initial hardware configuration on the DUT.
// It pushes the hardware initialization configuration for the VRF Selection
// Extended and Policy Forwarding features.
//
// TODO: The TCAM profile is currently required for the VRF configuration.
// Remove it if it is no longer needed after the global filter validation is
// complete.
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
	aftpf.ConfigureATEScaleBGP(t, dev1, aftpf.ATEBGPParams{
		DUTPort: dutPort1, ATEPort: atePort1, NamePrefix: "default-scale",
		V4RouteCount: uint32(*scaleIPv4Routes), V4BaseAddr: scaleV4Pfx, V4PrefixLen: scaleV4PfxLen,
		V6RouteCount: uint32(*scaleIPv6Routes), V6BaseAddr: scaleV6Pfx, V6PrefixLen: scaleV6PfxLen,
	})
	aftpf.ConfigureATEScaleBGP(t, dev2, aftpf.ATEBGPParams{
		DUTPort: dutPort2, ATEPort: atePort2, NamePrefix: "vrfa-scale",
		V4RouteCount: uint32(*scaleIPv4Routes), V4BaseAddr: scaleVrfV4Pfx, V4PrefixLen: scaleV4PfxLen,
		V6RouteCount: uint32(*scaleIPv6Routes), V6BaseAddr: scaleVrfV6Pfx, V6PrefixLen: scaleV6PfxLen,
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
	aftpf.ConfigureScaleBGP(t, dut, aftpf.BGPParams{
		NetworkInstance: defaultNI,
		RouterID:        dutPort1.IPv4,
		V4Neighbor:      atePort1.IPv4,
		V6Neighbor:      atePort1.IPv6,
	})
	aftpf.ConfigureScaleBGP(t, dut, aftpf.BGPParams{
		NetworkInstance: nonDefaultNI,
		RouterID:        dutPort2.IPv4,
		V4Neighbor:      atePort2.IPv4,
		V6Neighbor:      atePort2.IPv6,
	})
}

// fetchAFT collects AFT telemetry from two independent sessions and validates consistency between them.
// Per AFT-6.3.2 the two subscriptions must stream simultaneously, so both
// collectors are run concurrently and the caller's elapsed time therefore
// reflects the true concurrent synchronization time rather than the sum of two
// sequential collections.
func fetchAFT(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, aftSession1, aftSession2 *aftcache.AFTStreamSession, stoppingCondition aftcache.PeriodicHook, wantPrefixes map[string]bool, timeout time.Duration) (*aftcache.AFTData, error) {
	t.Helper()
	runCollectorsConcurrently(t, aftpf.RunCollectorParams{Ctx: ctx, Collector: aftSession1, Stop: stoppingCondition, Timeout: timeout},
		aftpf.RunCollectorParams{Ctx: ctx, Collector: aftSession2, Stop: stoppingCondition, Timeout: timeout})
	aft1, err := aftSession1.ToAFT(t, dut)
	if err != nil {
		return nil, fmt.Errorf("error getting AFT from session1: %v", err)
	}
	aft2, err := aftSession2.ToAFT(t, dut)
	if err != nil {
		return nil, fmt.Errorf("error getting AFT from session2: %v", err)
	}
	filteredAFT1 := aft1.FilterByPrefixes(wantPrefixes)
	filteredAFT2 := aft2.FilterByPrefixes(wantPrefixes)
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

// runCollectorsConcurrently starts every supplied AFT collector at the same
// time and returns once all of them have satisfied their stopping condition.
// AFT-6.3.2 requires the subscriptions to stream simultaneously; running the
// collectors back to back would instead measure the sum of two sequential
// synchronizations and would never exercise the DUT's ability to serve
// concurrent AFT subscribers.
func runCollectorsConcurrently(t *testing.T, cfgs ...aftpf.RunCollectorParams) {
	t.Helper()
	var wg sync.WaitGroup
	for _, cfg := range cfgs {
		wg.Add(1)
		go func(cfg aftpf.RunCollectorParams) {
			defer wg.Done()
			aftpf.RunCollector(t, cfg)
		}(cfg)
	}
	wg.Wait()
	// A collector failing inside its goroutine marks the test as failed but
	// cannot abort it from there, so stop the test here on the main goroutine.
	if t.Failed() {
		t.FailNow()
	}
}

// awaitScaleBGPConvergence waits for the eBGP sessions in the default network
// instance and in the non-default VRF to establish and for the expected scale
// IPv4/IPv6 route counts to be installed.
func awaitScaleBGPConvergence(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	aftpf.AwaitScaleBGPConvergence(t, dut, aftpf.BGPConvergenceParams{
		NetworkInstance: deviations.DefaultNetworkInstance(dut),
		V4Neighbor:      atePort1.IPv4, V6Neighbor: atePort1.IPv6,
		V4RouteCount: uint32(*scaleIPv4Routes), V6RouteCount: uint32(*scaleIPv6Routes),
	})
	aftpf.AwaitScaleBGPConvergence(t, dut, aftpf.BGPConvergenceParams{
		NetworkInstance: vrfName,
		V4Neighbor:      atePort2.IPv4, V6Neighbor: atePort2.IPv6,
		V4RouteCount: uint32(*scaleIPv4Routes), V6RouteCount: uint32(*scaleIPv6Routes),
	})
}

// testAfterReboot validates that AFT filtering policies and filtered entries
// are preserved across a DUT reboot.
func testAfterReboot(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wantPrefixes := aftpf.GeneratePrefixes(t, aftpf.GeneratePrefixesParams{V4Prefixes: []string{matchPrefixAft1, matchPrefixAft2}, V6Prefixes: policyIPv6Prefixes, PfxCount: pfxCount})
	// Verify configured policies before reboot.
	verifyGlobalFilterPolicies(t, dut, ipv4Policy, ipv6Policy)
	// Establish initial gNMI subscriptions.
	aftClient1 := aftpf.GnmiClientSession(t, dut, aftpf.PrefixesParams{Ctx: ctx})
	aftSession1 := aftcache.NewAFTStreamSession(ctx, t, aftClient1, dut)
	aftSession2 := aftcache.NewAFTStreamSession(ctx, t, aftpf.GnmiClientSession(t, dut, aftpf.PrefixesParams{Ctx: ctx}), dut)
	t.Log("Collecting initial filtered AFT entries")
	stoppingCondition := aftcache.InitialSyncStoppingCondition(t, dut, wantPrefixes, map[string]bool{atePort1.IPv4: true}, map[string]bool{atePort1.IPv6: true})
	aftBefore, err := fetchAFT(ctx, t, dut, aftSession1, aftSession2, stoppingCondition, wantPrefixes, aftConvergenceTime)
	if err != nil {
		t.Fatalf("Failed to fetch initial AFT: %v", err)
	}
	verifyFilteredPrefixes(t, aftBefore, wantPrefixes, rebootUnmatchedPrefixes, true)
	// Get initial boot time.
	lastBootTime, err := bootTime(t, dut)
	if err != nil {
		t.Fatalf("Failed to get boot time: %v", err)
	}
	// Open an additional AFT subscription on the same client used before the
	// reboot and keep it active across the reboot so that its termination can be
	// asserted on the pre-existing stream (rather than on a new subscription
	// created after the DUT is already down).
	monitoredStream := startAFTStream(ctx, t, aftClient1)
	t.Log("Rebooting DUT")
	rebootDUT(t, dut)
	// The subscription is active while the DUT reboots; verify the stream is
	// terminated by the DUT going down.
	verifyStreamTerminated(t, monitoredStream)
	// Wait for DUT recovery.
	waitForReboot(t, dut, lastBootTime)
	// Verify policy persistence after reboot.
	verifyGlobalFilterPolicies(t, dut, ipv4Policy, ipv6Policy)
	// The reboot tears down the eBGP sessions, so wait for them to re-establish
	// and for the scale routes to be re-learned before continuing. Otherwise the
	// subsequent subtests would start against a partially converged RIB/AFT.
	t.Log("Waiting for BGP sessions and scale routes to re-converge after reboot")
	awaitScaleBGPConvergence(t, dut)
	// Re-establish subscriptions.
	t.Log("Re-establishing AFT subscriptions")
	aftSession3 := aftcache.NewAFTStreamSession(ctx, t, aftpf.GnmiClientSession(t, dut, aftpf.PrefixesParams{Ctx: ctx}), dut)
	aftSession4 := aftcache.NewAFTStreamSession(ctx, t, aftpf.GnmiClientSession(t, dut, aftpf.PrefixesParams{Ctx: ctx}), dut)
	stoppingCondition2 := aftcache.InitialSyncStoppingCondition(t, dut, wantPrefixes, map[string]bool{atePort1.IPv4: true}, map[string]bool{atePort1.IPv6: true})
	aftAfter, err := fetchAFT(ctx, t, dut, aftSession3, aftSession4, stoppingCondition2, wantPrefixes, aftConvergenceTime)
	if err != nil {
		t.Fatalf("Failed to fetch AFT after reboot: %v", err)
	}
	// Verify filtered entries after reboot.
	verifyFilteredPrefixes(t, aftAfter, wantPrefixes, rebootUnmatchedPrefixes, true)
	t.Log("AFT reboot validation completed successfully")
}

// startAFTStream establishes an AFT ON_CHANGE subscription on the given client
// and returns the active stream. The stream is intentionally left open so that
// its termination can be observed later.
func startAFTStream(ctx context.Context, t *testing.T, client gpb.GNMIClient) gpb.GNMI_SubscribeClient {
	t.Helper()
	stream, err := client.Subscribe(ctx)
	if err != nil {
		t.Fatalf("Failed to establish gNMI AFT subscription before reboot: %v", err)
	}
	req := &gpb.SubscribeRequest{
		Request: &gpb.SubscribeRequest_Subscribe{
			Subscribe: &gpb.SubscriptionList{
				Mode:     gpb.SubscriptionList_STREAM,
				Encoding: gpb.Encoding_PROTO,
				Subscription: []*gpb.Subscription{{
					Path: gnmiPath(t, aftsPath),
					Mode: gpb.SubscriptionMode_ON_CHANGE,
				}},
			},
		},
	}
	if err := stream.Send(req); err != nil {
		t.Fatalf("Failed to send gNMI AFT subscribe request before reboot: %v", err)
	}
	return stream
}

// verifyStreamTerminated verifies that the gNMI AFT subscription that was
// already active before the reboot is torn down once the DUT goes down. Only
// transport level errors are expected during the reboot window; if no error is
// observed within streamTerminationWait the test fails, since a client side
// timeout is not a valid proof of stream termination.
func verifyStreamTerminated(t *testing.T, stream gpb.GNMI_SubscribeClient) {
	t.Helper()
	errCh := make(chan error, 1)
	go func() {
		for {
			if _, err := stream.Recv(); err != nil {
				errCh <- err
				return
			}
		}
	}()
	select {
	case err := <-errCh:
		// A deadline/cancellation raised by the test's own context means the
		// client gave up, not that the DUT terminated the stream.
		if isContextError(err) {
			t.Fatalf("gNMI AFT stream ended due to a client side context error, not DUT stream termination: %v", err)
		}
		if !isEndpointUnreachableError(err) {
			t.Fatalf("gNMI AFT stream ended with non-endpoint error during reboot window: %v", err)
		}
		t.Logf("gNMI AFT stream terminated as expected after reboot: %v", err)
	case <-time.After(streamTerminationWait):
		t.Fatalf("gNMI AFT stream did not terminate within %v after the reboot was issued", streamTerminationWait)
	}
}

// isContextError reports whether err was caused by the client context being
// cancelled or exceeding its deadline, rather than by the DUT closing the RPC.
func isContextError(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}
	switch status.Code(err) {
	case codes.DeadlineExceeded, codes.Canceled:
		return true
	}
	return false
}

// isEndpointUnreachableError reports whether err indicates the DUT endpoint
// became unreachable while rebooting.
func isEndpointUnreachableError(err error) bool {
	if status.Code(err) == codes.Unavailable {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "no route to host") ||
		strings.Contains(msg, "network is unreachable") ||
		strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "transport is closing") ||
		strings.Contains(msg, "eof")
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
		// network-instance is a keyed list, so the query must be scoped to a
		// single instance. Without the [name=...] key the DUT would either
		// reject the request or return the global-filter policies of every
		// network instance, and the last notification received would
		// non-deterministically overwrite the values checked below.
		niName := deviations.DefaultNetworkInstance(dut)
		ocPolicyConfigPathV4 := fmt.Sprintf("/network-instances/network-instance[name=%s]/afts/global-filter/config/ipv4-policy", niName)
		ocPolicyConfigPathV6 := fmt.Sprintf("/network-instances/network-instance[name=%s]/afts/global-filter/config/ipv6-policy", niName)
		ocPolicyStatePathV4 := fmt.Sprintf("/network-instances/network-instance[name=%s]/afts/global-filter/state/ipv4-policy", niName)
		ocPolicyStatePathV6 := fmt.Sprintf("/network-instances/network-instance[name=%s]/afts/global-filter/state/ipv6-policy", niName)
		gnmiClient, err := dut.RawAPIs().BindingDUT().DialGNMI(context.Background())
		if err != nil {
			t.Fatalf("Failed to dial GNMI: %v", err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		configReq := &gpb.GetRequest{
			Path: []*gpb.Path{
				gnmiPath(t, ocPolicyConfigPathV4),
				gnmiPath(t, ocPolicyConfigPathV6),
			},
			Type: gpb.GetRequest_CONFIG,
		}
		configResp, err := gnmiClient.Get(ctx, configReq)
		if err != nil {
			t.Fatalf("GNMI Get for global-filter config failed: %v", err)
		}
		stateReq := &gpb.GetRequest{
			Path: []*gpb.Path{
				gnmiPath(t, ocPolicyStatePathV4),
				gnmiPath(t, ocPolicyStatePathV6),
			},
			Type: gpb.GetRequest_STATE,
		}
		stateResp, err := gnmiClient.Get(ctx, stateReq)
		if err != nil {
			t.Fatalf("GNMI Get for global-filter state failed: %v", err)
		}
		var gotConfigIPv4Policy string
		var gotConfigIPv6Policy string
		for _, notif := range configResp.GetNotification() {
			for _, upd := range notif.GetUpdate() {
				path, err := ygot.PathToString(upd.GetPath())
				if err != nil {
					t.Fatalf("PathToString failed: %v", err)
				}
				val := upd.GetVal().GetStringVal()
				switch {
				case strings.Contains(path, "ipv4-policy"):
					gotConfigIPv4Policy = val
				case strings.Contains(path, "ipv6-policy"):
					gotConfigIPv6Policy = val
				}
			}
		}
		var gotStateIPv4Policy string
		var gotStateIPv6Policy string
		for _, notif := range stateResp.GetNotification() {
			for _, upd := range notif.GetUpdate() {
				path, err := ygot.PathToString(upd.GetPath())
				if err != nil {
					t.Fatalf("PathToString failed: %v", err)
				}
				val := upd.GetVal().GetStringVal()
				switch {
				case strings.Contains(path, "ipv4-policy"):
					gotStateIPv4Policy = val
				case strings.Contains(path, "ipv6-policy"):
					gotStateIPv6Policy = val
				}
			}
		}
		if gotConfigIPv4Policy != wantIPv4Policy {
			t.Fatalf("IPv4 config policy mismatch got=%s want=%s", gotConfigIPv4Policy, wantIPv4Policy)
		}
		if gotConfigIPv6Policy != wantIPv6Policy {
			t.Fatalf("IPv6 config policy mismatch got=%s want=%s", gotConfigIPv6Policy, wantIPv6Policy)
		}
		if gotStateIPv4Policy != wantIPv4Policy {
			t.Fatalf("IPv4 state policy mismatch got=%s want=%s", gotStateIPv4Policy, wantIPv4Policy)
		}
		if gotStateIPv6Policy != wantIPv6Policy {
			t.Fatalf("IPv6 state policy mismatch got=%s want=%s", gotStateIPv6Policy, wantIPv6Policy)
		}
		t.Logf("Verified persisted global-filter config/state policies for %s: ipv4=%s ipv6=%s", niName, gotConfigIPv4Policy, gotConfigIPv6Policy)
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
			MatchPrefixSet: true,
			MatchSetOption: oc.RoutingPolicy_MatchSetOptionsRestrictedType_ANY,
			PolicyResult:   oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
		},
		{
			PolicyName:     ipv6Policy,
			StatementNames: []string{"20"},
			PrefixSetNames: []string{v6PfxSet},
			PrefixList:     policyIPv6Prefixes,
			PrefixMode:     pfxMode,
			MatchPrefixSet: true,
			MatchSetOption: oc.RoutingPolicy_MatchSetOptionsRestrictedType_ANY,
			PolicyResult:   oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
		},
	}
	for _, policy := range policies {
		aftpf.AddPrefixSetPolicy(t, rp, policy)
	}
	// VRF policies require separate IPv4/IPv6 prefix modes.
	aftpf.AddPrefixSetPolicy(t, rp,
		aftpf.PrefixSetPolicyParams{
			PolicyName:     vrfAPolicy,
			StatementNames: []string{defaultStatementName},
			PrefixSetNames: []string{vrfPrefixNameV4},
			PrefixList:     []string{vrfV4Pfx},
			PrefixMode:     pfxV4MaskRange,
			MatchPrefixSet: true,
			MatchSetOption: oc.RoutingPolicy_MatchSetOptionsRestrictedType_ANY,
			PolicyResult:   oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
		})
	aftpf.AddPrefixSetPolicy(t, rp,
		aftpf.PrefixSetPolicyParams{
			PolicyName:     vrfAPolicy,
			StatementNames: []string{"20"},
			PrefixSetNames: []string{vrfPrefixNameV6},
			PrefixList:     []string{vrfV6Pfx},
			PrefixMode:     pfxV6MaskRange,
			MatchPrefixSet: true,
			MatchSetOption: oc.RoutingPolicy_MatchSetOptionsRestrictedType_ANY,
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
				MatchSetOption: oc.RoutingPolicy_MatchSetOptionsRestrictedType_ANY,
				PolicyResult:   oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
			},
			{
				PolicyName:     fmt.Sprintf("POLICY-SCALE-IPV6-%d", percent),
				StatementNames: []string{defaultStatementName},
				PrefixSetNames: []string{fmt.Sprintf("PREFIX-SCALE-IPV6-%d", percent)},
				PrefixList:     selectPercentagePrefixes(ipv6Prefixes, percent),
				PrefixMode:     pfxMode,
				MatchPrefixSet: true,
				MatchSetOption: oc.RoutingPolicy_MatchSetOptionsRestrictedType_ANY,
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
	ips, err := iputil.GenerateIPsWithStep(scaleV4Pfx, *scaleIPv4Routes, intStepV4)
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
	ips, err := iputil.GenerateIPv6sWithStep(scaleV6Pfx, *scaleIPv6Routes, intStepV6)
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
	_, ok := gnmi.Watch(t, dut, gnmi.OC().System().BootTime().State(), maxRebootTime, func(val *ygnmi.Value[uint64]) bool {
		var ok bool
		bootTime, ok = val.Val()
		return ok
	}).Await(t)
	if !ok {
		return 0, fmt.Errorf("failed to get boot time")
	}
	return bootTime, nil
}

// waitForReboot waits for the DUT to become unreachable, come back online,
// and report a boot time newer than the previous boot time.
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
	t.Logf("DUT became reachable again after %.2f seconds; waiting for the boot time telemetry to update.", time.Since(startReboot).Seconds())
	// No explicit gNMI dial is performed here: gRPC dialing in Go is
	// non-blocking, so a successful Dial would not prove the gNMI server is
	// serving again. The Watch below issues real RPCs and is therefore the
	// actual readiness check.
	// Wait for boot time to change. The Watch below already returns the boot
	// time it last observed, so the value is reused instead of starting another
	// watch just to read it back.
	val, ok := gnmi.Watch(t, dut, gnmi.OC().System().BootTime().State(), maxRebootTime, bootTimePredicate(lastBootTime)).Await(t)
	if !ok {
		lastObserved, present := val.Val()
		if !present {
			t.Fatalf("Boot time did not update after reboot; no boot time reported. Last: %d", lastBootTime)
		}
		t.Fatalf("Boot time did not update after reboot. Current: %d, Last: %d", lastObserved, lastBootTime)
	}
	currentBootTime, _ := val.Val()
	t.Logf("Reboot completed in %.2f seconds; boot time successfully changed from %d to %d.", time.Since(startReboot).Seconds(), lastBootTime, currentBootTime)
}

// bootTimePredicate returns a predicate that evaluates to true when the DUT
// reports a boot time greater than the provided previous boot time.
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
	var ipv4Prefixes, ipv6Prefixes []string
	// Enumerate the scale routes advertised via BGP from the ATE
	t.Logf("Using %d IPv4 and %d IPv6 BGP-advertised scale routes", *scaleIPv4Routes, *scaleIPv6Routes)
	ipv4Pfs, err := iputil.GenerateIPsWithStep(scaleV4Pfx, *scaleIPv4Routes, intStepV4)
	if err != nil {
		t.Fatalf("failed to generate DUT IPs: %v", err)
	}
	for _, ip := range ipv4Pfs {
		ipv4Prefixes = append(ipv4Prefixes, fmt.Sprintf("%s/%d", ip, scaleV4PfxLen))
	}
	ipv6Pfs, err := iputil.GenerateIPv6sWithStep(scaleV6Pfx, *scaleIPv6Routes, intStepV6)
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
			// Scope the streaming context to this scenario. Each scenario opens two
			// AFT stream sessions, and every session starts a background reader
			// goroutine that lives until its context is cancelled. Cancelling here
			// tears the sessions down at the end of the scenario so that the DUT
			// never has to serve the subscriptions of all scenarios at once.
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			// Select expected prefixes
			var selectedPrefixes []string
			var unmatchedPrefixes []string
			var wantPrefixes map[string]bool
			var stoppingCondition aftcache.PeriodicHook
			if tc.ipv4 {
				selectedPrefixes = selectPercentagePrefixes(ipv4Prefixes, tc.matchPercent)
				unmatchedPrefixes = unmatchedScalePrefixes(ipv4Prefixes, tc.matchPercent)
				wantPrefixes = aftpf.GeneratePrefixes(t, aftpf.GeneratePrefixesParams{V4Prefixes: selectedPrefixes, V6Prefixes: nil, PfxCount: pfxCount})
			} else {
				selectedPrefixes = selectPercentagePrefixes(ipv6Prefixes, tc.matchPercent)
				unmatchedPrefixes = unmatchedScalePrefixes(ipv6Prefixes, tc.matchPercent)
				wantPrefixes = aftpf.GeneratePrefixes(t, aftpf.GeneratePrefixesParams{V4Prefixes: nil, V6Prefixes: selectedPrefixes, PfxCount: pfxCount})
			}
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
			aftData, err := fetchAFT(ctx, t, dut, aftSession1, aftSession2, stoppingCondition, wantPrefixes, *scaleSyncDeadline)
			if err != nil {
				t.Fatalf("Failed to fetch scaled AFT: %v", err)
			}
			syncDuration := time.Since(start)
			t.Logf("Synchronization completed in %v", syncDuration)
			// Verify synchronization time
			if syncDuration > *scaleSyncDeadline {
				t.Fatalf("Synchronization exceeded limit got=%v want<=%v", syncDuration, *scaleSyncDeadline)
			}
			// Verify filtering correctness
			verifyFilteredPrefixes(t, aftData, wantPrefixes, unmatchedPrefixes, tc.ipv4)
			t.Logf("Verified scale filtering for %s: %d matched prefixes present, %d unmatched prefixes absent", tc.name, len(selectedPrefixes), len(unmatchedPrefixes))
		})
	}
}

// verifyFilteredPrefixes validates that the AFT contains all expected prefixes
// after applying the filter policy and does not contain prefixes that should
// have been filtered out. unmatchedPrefixes must contain routes that are
// actually installed on the DUT but excluded by the active filter policy;
// prefixes that were never advertised would make the negative check vacuous.
func verifyFilteredPrefixes(t *testing.T, aftPrefixes *aftcache.AFTData, wantPrefixes map[string]bool, unmatchedPrefixes []string, ipv4 bool) {
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
	// Verify installed-but-unmatched prefixes were filtered out.
	for _, pfx := range unmatchedPrefixes {
		if _, ok := aftPrefixes.Prefixes[pfx]; ok {
			t.Fatalf("Unmatched %s prefix present after filtering: %s", addressFamily, pfx)
		}
	}
	t.Logf("Verified %s filtered prefixes", addressFamily)
}

// selectPercentagePrefixes selects percentage-based subset.
func selectPercentagePrefixes(prefixes []string, percent int) []string {
	count := (len(prefixes) * percent) / 100
	if count == 0 {
		count = 1
	}
	return prefixes[:count]
}

// unmatchedScalePrefixes returns representative prefixes from the portion of
// the scale pool that the active policy does NOT match, i.e. everything after
// the percentage-based subset returned by selectPercentagePrefixes. These
// prefixes are advertised by the ATE and are therefore present in the DUT's
// unfiltered AFT, so asserting their absence from the streamed AFT validates
// that the global filter drops un-matched routes. Representative prefixes are
// taken at the filter boundary (first unmatched prefix), at a fixed offset
// beyond the boundary, and at the end of the pool, instead of every remaining
// prefix, to keep the verification inexpensive.
func unmatchedScalePrefixes(prefixes []string, matchPercent int) []string {
	matchedCount := len(selectPercentagePrefixes(prefixes, matchPercent))
	if matchedCount >= len(prefixes) {
		return nil
	}
	unmatched := prefixes[matchedCount:]
	boundaryOffsets := []int{0, unmatchedPrefixOffset, len(unmatched) - 1}
	selected := make(map[string]bool)
	var result []string
	for _, offset := range boundaryOffsets {
		if offset < 0 || offset >= len(unmatched) {
			continue
		}
		prefix := unmatched[offset]
		if selected[prefix] {
			continue
		}
		selected[prefix] = true
		result = append(result, prefix)
	}
	return result
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
	aftpf.RunCollector(t, aftpf.RunCollectorParams{Ctx: ctx, Collector: collector1, Stop: defaultStop, Timeout: subscriptionWait})
	aftpf.RunCollector(t, aftpf.RunCollectorParams{Ctx: ctx, Collector: collector2, Stop: vrfStop, Timeout: subscriptionWait})
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
	aftpf.VerifyPrefixesAbsent(t, aftpf.PrefixesParams{InfoAFT: vrfAFT, Prefixes: []string{matchPrefixAft1, matchPrefixAft2, matchVrfPfx3}})
	// Install the routes that must be filtered out together with a canary route
	// that each collector IS expected to receive. The canary is matched by the
	// collector's own policy, so its arrival proves the stream was live and the
	// DUT had time to propagate updates. Only once the canary has been observed
	// can the absence of the unmatched routes be treated as evidence of
	// filtering rather than of the test checking too early.
	t.Log("Adding routes that must be filtered out, plus per-collector canary routes")
	aftpf.AddSingleStaticRoute(t, dut, aftpf.AddStaticRouteParams{NetworkInstanceName: deviations.DefaultNetworkInstance(dut), Prefix: v4AbsentPfx1, Index: fmt.Sprintf("%d", staticRouteIndex+900), NextHop: atePort1.IPv4})
	aftpf.AddSingleStaticRoute(t, dut, aftpf.AddStaticRouteParams{NetworkInstanceName: deviations.DefaultNetworkInstance(dut), Prefix: v4AbsentPfx2, Index: fmt.Sprintf("%d", staticRouteIndex+901), NextHop: atePort1.IPv4})
	// Canary for Collector-1: matched by POLICY-PREFIX-SET-A in DEFAULT.
	aftpf.AddSingleStaticRoute(t, dut, aftpf.AddStaticRouteParams{NetworkInstanceName: deviations.DefaultNetworkInstance(dut), Prefix: matchVrfPfx4, Index: fmt.Sprintf("%d", staticRouteIndex+902), NextHop: atePort1.IPv4})
	// Canary for Collector-2: matched by POLICY-PREFIX-SET-VRF-A in VRF-A.
	aftpf.AddSingleStaticRoute(t, dut, aftpf.AddStaticRouteParams{NetworkInstanceName: vrfName, Prefix: matchVrfPfx2, Index: fmt.Sprintf("%d", staticRouteIndex+903), NextHop: atePort2.IPv4})
	// Collector-1 must receive its own canary and must never receive the routes
	// excluded by its filter, nor the VRF-A canary.
	t.Log("Observing Collector-1 stream for leaked prefixes")
	verifyPrefixesFilteredDuringStream(ctx, t, dut, collector1, prefixLeakCheckParams{
		CollectorName:     "Collector1",
		CanaryPrefix:      matchVrfPfx4,
		ForbiddenPrefixes: []string{v4AbsentPfx1, v4AbsentPfx2, matchVrfPfx2},
		Timeout:           subscriptionWait,
	})
	// Collector-2 must receive its own canary and must never receive the DEFAULT
	// routes, nor the DEFAULT canary.
	t.Log("Observing Collector-2 stream for leaked prefixes")
	verifyPrefixesFilteredDuringStream(ctx, t, dut, collector2, prefixLeakCheckParams{
		CollectorName:     "Collector2",
		CanaryPrefix:      matchVrfPfx2,
		ForbiddenPrefixes: []string{v4AbsentPfx1, v4AbsentPfx2, matchVrfPfx4},
		Timeout:           subscriptionWait,
	})
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
	// The filter-policy reconfiguration may cause the DUT to terminate
	// Collector 2's stream, so re-establish the subscription and validate the
	// full set of VRF-A prefixes after SYNC on the new stream.
	t.Log("Re-establishing Collector2 subscription after the VRF-A filter policy change")
	collector2 = reestablishAFTSession(ctx, t, dut)
	// Collector2 should now receive all VRF-A routes within 60 seconds.
	t.Log("Waiting for Collector2 to receive all VRF-A routes")
	wantAllVRF := []string{matchPrefixAft1, matchVrfPfx1, matchVrfPfx3, matchVrfPfx2}
	waitForPrefixesPresent(ctx, t, dut, collector2, wantAllVRF, policyChangeWait, atePort2.IPv4)
	t.Log("Per-network-instance filtering validation completed successfully")
}

// prefixLeakCheckParams holds the parameters used to verify, on a live AFT
// stream, that filtered-out prefixes never reach a collector.
type prefixLeakCheckParams struct {
	// CollectorName identifies the collector in log and failure messages.
	CollectorName string
	// CanaryPrefix is a prefix the collector IS expected to receive. Observing
	// it proves the stream is live and that the DUT has propagated the routes
	// installed alongside it.
	CanaryPrefix string
	// ForbiddenPrefixes lists prefixes the collector must never receive.
	ForbiddenPrefixes []string
	// Timeout bounds how long the stream is observed while waiting for the
	// canary prefix.
	Timeout time.Duration
}

// verifyPrefixesFilteredDuringStream listens on the collector's live AFT stream
// until the canary prefix is received, failing as soon as any forbidden prefix
// appears. Listening on the stream, rather than inspecting the cached snapshot
// synchronously right after installing a route, ensures the DUT has been given
// time to propagate updates: a snapshot read taken immediately after a route is
// added would pass even if the DUT leaked that route moments later.
func verifyPrefixesFilteredDuringStream(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, session *aftcache.AFTStreamSession, cfg prefixLeakCheckParams) {
	t.Helper()
	stoppingCondition := aftcache.PeriodicHook{
		Description: fmt.Sprintf("%s: await canary %s while checking for leaked prefixes", cfg.CollectorName, cfg.CanaryPrefix),
		PeriodicFunc: func(ss *aftcache.AFTStreamSession) (bool, error) {
			aft, err := ss.ToAFT(t, dut)
			if err != nil {
				return false, err
			}
			for _, prefix := range cfg.ForbiddenPrefixes {
				if _, ok := aft.Prefixes[prefix]; ok {
					return false, fmt.Errorf("%s received filtered-out prefix %s", cfg.CollectorName, prefix)
				}
			}
			if _, ok := aft.Prefixes[cfg.CanaryPrefix]; !ok {
				return false, nil
			}
			t.Logf("%s received canary prefix %s; no filtered-out prefixes leaked", cfg.CollectorName, cfg.CanaryPrefix)
			return true, nil
		},
	}
	session.ListenUntil(ctx, t, cfg.Timeout, stoppingCondition)
}

// waitForPrefixesPresent validates that all prefixes appear on the collector's
// stream after SYNC, using the existing aftcache streaming API.
func waitForPrefixesPresent(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, session *aftcache.AFTStreamSession, prefixes []string, timeout time.Duration, nextHop string) {
	t.Helper()
	wantPrefixes := aftpf.GeneratePrefixes(t, aftpf.GeneratePrefixesParams{V4Prefixes: prefixes, V6Prefixes: nil, PfxCount: pfxCount})
	stoppingCondition := aftcache.InitialSyncStoppingCondition(t, dut, wantPrefixes, map[string]bool{nextHop: true}, nil)
	session.ListenUntil(ctx, t, timeout, stoppingCondition)
}

// reestablishAFTSession dials a new gNMI client and returns a fresh AFT stream
// session. It is used after a global-filter policy reconfiguration, which the
// DUT is allowed to answer by terminating the existing AFT subscription. The
// previous session's stream may therefore already be closed, and continuing to
// listen on it would fail instead of validating the new filter behaviour, so
// the verification is always performed on a re-established subscription.
func reestablishAFTSession(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice) *aftcache.AFTStreamSession {
	t.Helper()
	return aftcache.NewAFTStreamSession(ctx, t, aftpf.GnmiClientSession(t, dut, aftpf.PrefixesParams{Ctx: ctx}), dut)
}
