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
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	aftpf "github.com/openconfig/featureprofiles/internal/aft_prefix_filtering"
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
	subscriptionWait     = 5 * time.Minute
	prefixAFT1V4         = "198.51.100.0/24"
	prefixAFT2V4         = "203.0.113.0/28"
	prefixAFT3V4         = "192.0.2.0/24"
	prefixAFT1V6         = "2001:db8:2::/64"
	prefixAFT2V6         = "2001:db8:2::1/128"
	prefixAFT3V6         = "2001:db8:2::2/128"
	vrfV4Pfx             = "100.64.1.0/24"
	vrfV6Pfx             = "2001:db8:3::/64"
	vrfRouteV4Pfx        = "90.0.0.1"
	vrfRouteV6Pfx        = "4000::1"
	maskRange            = "exact"
	notificationWaitTime = 30 * time.Second
	staticRouteIndex     = 100
	pfxCount             = 1
	aftFilterDUTAS       = 65001
	vrfRoutes            = 5000
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
		"192.0.2.0/24",
	}

	policyIPv4Prefixes = []string{
		"198.51.100.0/24",
		"203.0.113.0/28",
		"198.51.100.1/32",
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
		"198.18.1.0/24",
		"100.64.1.0/24",
		"203.0.113.128/28",
	}
	vrfV6Prefixes = []string{
		"2001:db8:3::/64",
		"2001:db8:3::1/128",
		"2001:db8:3::2/128",
	}
)

type dynamicUpdateTestParams struct {
	testID                 string
	prefixSet              string
	initialAllowedPrefixes []string
	dynamicPrefix          string
	nhIP                   string
	maskRange              string
	policyName             string
	nhIndex                string
	isIPv6                 bool
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
	aftpf.AwaitScaleBGPConvergence(t, dut, aftpf.BGPConvergenceParams{
		NetworkInstance: deviations.DefaultNetworkInstance(dut),
		V4Neighbor:      atePort1.IPv4,
		V6Neighbor:      atePort1.IPv6,
		V4RouteCount:    aftpf.BulkV4RouteCount,
		V6RouteCount:    aftpf.BulkV6RouteCount,
	})
	aftpf.AwaitScaleBGPConvergence(t, dut, aftpf.BGPConvergenceParams{
		NetworkInstance: vrfName,
		V4Neighbor:      atePort2.IPv4,
		V6Neighbor:      atePort2.IPv6,
		V4RouteCount:    vrfRoutes,
		V6RouteCount:    vrfRoutes,
	})
	tests := []struct {
		name string
		test func(t *testing.T, dut *ondatra.DUTDevice)
	}{
		{
			name: "AFT-6.4.1-DynamicIPv4PrefixSetUpdates",
			test: testIPv4DynamicUpdates,
		},
		{
			name: "AFT-6.4.2-DynamicIPv6PrefixSetUpdates",
			test: testIPv6DynamicUpdates,
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
	configureDUTInterface(t, dut, batch, &dutPort1, p1)
	configureDUTInterface(t, dut, batch, &dutPort2, p2)
	cfgplugins.EnableDefaultNetworkInstanceBgp(t, dut, aftFilterDUTAS)
	defaultNI := cfgplugins.ConfigureNetworkInstance(t, dut, deviations.DefaultNetworkInstance(dut), true)
	nonDefaultNI := cfgplugins.ConfigureNetworkInstance(t, dut, vrfName, false)
	configureBGP(t, dut, defaultNI, nonDefaultNI)
	cfgplugins.UpdateNetworkInstanceOnDut(t, dut, defaultNI.GetName(), defaultNI)
	cfgplugins.UpdateNetworkInstanceOnDut(t, dut, vrfName, nonDefaultNI)
	configureDUTPort(t, dut, batch, &dutPort2, p2, vrfName)
	batch.Set(t, dut)
	aftpf.ApplyBGPMaxPrefixes(t, dut, aftpf.BGPPrefixParams{V4Prefix: atePort1.IPv4, V6Prefix: atePort1.IPv6, NetworkInstance: defaultNI})
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

// configureHardwareInit sets up the initial hardware configuration on the DUT.
// It pushes hardware initialization configurations for the VRF Selection Extended
// and Policy Forwarding features.
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
	// Advertise scaled background routes from the ATE: DEFAULT over port1.
	aftpf.ConfigureATEScaleBGP(t, dev1,
		aftpf.ATEBGPParams{
			DUTPort:      dutPort1,
			ATEPort:      atePort1,
			NamePrefix:   "default-bulk",
			V4RouteCount: aftpf.BulkV4RouteCount,
			V4BaseAddr:   aftpf.BulkV4BaseAddr,
			V4PrefixLen:  aftpf.BulkV4PrefixLen,
			V6RouteCount: aftpf.BulkV6RouteCount,
			V6BaseAddr:   aftpf.BulkV6BaseAddr,
			V6PrefixLen:  aftpf.BulkV6PrefixLen,
		})
	aftpf.ConfigureATEScaleBGP(t, dev2,
		aftpf.ATEBGPParams{
			DUTPort:      dutPort2,
			ATEPort:      atePort2,
			NamePrefix:   "vrf-bulk",
			V4RouteCount: vrfRoutes,
			V4BaseAddr:   vrfRouteV4Pfx,
			V4PrefixLen:  aftpf.BulkV4PrefixLen,
			V6RouteCount: vrfRoutes,
			V6BaseAddr:   vrfRouteV6Pfx,
			V6PrefixLen:  aftpf.BulkV6PrefixLen,
		})
	// Collect interface/device names
	for _, dev := range topo.Devices().Items() {
		interfaceNamesList = append(interfaceNamesList, dev.Name())
	}
	return topo, interfaceNamesList
}

// configureBGP configures dual-AFI eBGP peerings in the DEFAULT network instance.
// It enables the DUT to learn and install ATE-advertised background routes.
func configureBGP(t *testing.T, dut *ondatra.DUTDevice, defaultNI, nonDefaultNI *oc.NetworkInstance) {
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

// testIPv4DynamicUpdates validates:
//
//  1. Initial filtered subscription.
//  2. Add prefix to prefix-set -> matching AFT entry becomes visible.
//  3. Remove prefix from prefix-set -> AFT entry is removed.
//  4. Atomic prefix-set update -> add and delete notifications are received.
func testIPv4DynamicUpdates(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	testDynamicUpdates(t, dut,
		dynamicUpdateTestParams{
			testID:                 "AFT-6.4.1",
			prefixSet:              v4PfxSet,
			initialAllowedPrefixes: []string{prefixAFT1V4, prefixAFT2V4},
			dynamicPrefix:          prefixAFT3V4,
			nhIP:                   atePort1.IPv4,
			maskRange:              maskRange,
			policyName:             v4Policy,
			nhIndex:                "5001",
			isIPv6:                 false,
		},
	)
}

// testIPv6DynamicUpdates validates:
//
//  1. Initial filtered IPv6 subscription.
//  2. Add IPv6 prefix to prefix-set -> matching AFT entry becomes visible.
//  3. Remove IPv6 prefix from prefix-set -> AFT entry is removed.
//  4. Atomic prefix-set update -> add and delete notifications are received.
func testIPv6DynamicUpdates(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	testDynamicUpdates(t, dut,
		dynamicUpdateTestParams{
			testID:                 "AFT-6.4.2",
			prefixSet:              v6PfxSet,
			initialAllowedPrefixes: []string{prefixAFT1V6, prefixAFT2V6},
			dynamicPrefix:          prefixAFT3V6,
			nhIP:                   atePort1.IPv6,
			maskRange:              maskRange,
			policyName:             v6Policy,
			nhIndex:                "6001",
			isIPv6:                 true,
		},
	)
}

// testDynamicUpdates validates dynamic AFT prefix filtering behavior for both IPv4 and IPv6 route policies.
func testDynamicUpdates(t *testing.T, dut *ondatra.DUTDevice, pArgs dynamicUpdateTestParams) {
	t.Helper()
	var wantPrefixes map[string]bool
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Configure the appropriate AFT global filter policy.
	policyCfg := aftpf.ConfigureGlobalFilterPoliciesParams{VRFName: deviations.DefaultNetworkInstance(dut)}
	if pArgs.isIPv6 {
		policyCfg.V6Policy = pArgs.policyName
	} else {
		policyCfg.V4Policy = pArgs.policyName
	}
	aftpf.ConfigureGlobalFilterPolicies(t, dut, policyCfg)
	if pArgs.isIPv6 {
		wantPrefixes = aftpf.GeneratePrefixes(t,
			aftpf.GeneratePrefixesParams{
				V6Prefixes: pArgs.initialAllowedPrefixes,
				PfxCount:   pfxCount,
			})
	} else {
		wantPrefixes = aftpf.GeneratePrefixes(t,
			aftpf.GeneratePrefixesParams{
				V4Prefixes: pArgs.initialAllowedPrefixes,
				PfxCount:   pfxCount,
			})
	}
	gnmiClient, err := dut.RawAPIs().BindingDUT().DialGNMI(ctx)
	if err != nil {
		t.Fatalf("Failed to dial GNMI: %v", err)
	}
	//----------------------------------------------------------------------
	// Create ONE persistent collector.
	//----------------------------------------------------------------------
	collector := aftpf.NewCollector(t, dut,
		aftpf.NewCollectorParams{
			Context: ctx,
			Client:  gnmiClient,
		})
	//----------------------------------------------------------------------
	// Initial Sync
	//----------------------------------------------------------------------
	t.Logf("%s - Initial Synchronization", pArgs.testID)
	aftpf.RunCollector(t,
		aftpf.RunCollectorParams{
			Ctx:       ctx,
			Collector: collector,
			Stop: aftcache.InitialSyncStoppingCondition(t, dut, wantPrefixes,
				map[string]bool{atePort1.IPv4: true},
				map[string]bool{atePort1.IPv6: true},
			),
			Timeout: subscriptionWait,
		})
	initialAFT, err := collector.ToAFT(t, dut)
	if err != nil {
		t.Fatalf("ToAFT failed: %v", err)
	}
	aftpf.VerifyPrefixesPresent(t,
		aftpf.PrefixesParams{
			InfoAFT:  initialAFT,
			Prefixes: pArgs.initialAllowedPrefixes,
		})
	//----------------------------------------------------------------------
	// AFT-6.4.X.1 Add Prefix
	//----------------------------------------------------------------------
	t.Logf("%s.1 - Addition of Prefix to Active Set", pArgs.testID)
	aftpf.AddPrefixToPrefixSet(t, dut,
		aftpf.AddPrefixToPrefixSetParams{
			PrefixSetName: pArgs.prefixSet,
			Prefix:        pArgs.dynamicPrefix,
			MaskRange:     pArgs.maskRange,
		})
	// Wait for ADD notification on the SAME stream.
	aftpf.RunCollector(t,
		aftpf.RunCollectorParams{
			Ctx:       ctx,
			Collector: collector,
			Stop: aftcache.WaitForNotification(t,
				aftcache.NotificationExpectation{
					AddPrefix:        pArgs.dynamicPrefix,
					NotificationWait: notificationWaitTime,
				}),
			Timeout: subscriptionWait,
		})
	//----------------------------------------------------------------------
	// AFT-6.4.X.2 Delete Prefix
	//----------------------------------------------------------------------
	t.Logf("%s.2 - Deletion of Prefix from Active Set", pArgs.testID)
	aftpf.RemovePrefixFromPrefixSet(t, dut,
		aftpf.RemovePrefixFromPrefixSetParams{
			PrefixSetName: pArgs.prefixSet,
			Prefix:        pArgs.initialAllowedPrefixes[0],
			MaskRange:     pArgs.maskRange,
		},
	)
	verifyPrefixRemovedFromPrefixSet(t, dut, pArgs.prefixSet, pArgs.initialAllowedPrefixes[0], pArgs.maskRange)
	// Wait for DELETE notification on SAME stream.
	aftpf.RunCollector(t,
		aftpf.RunCollectorParams{
			Ctx:       ctx,
			Collector: collector,
			Stop: aftcache.WaitForNotification(t,
				aftcache.NotificationExpectation{
					DeletePrefix:     pArgs.initialAllowedPrefixes[0],
					NotificationWait: notificationWaitTime,
				}),
			Timeout: subscriptionWait,
		})
	//----------------------------------------------------------------------
	// AFT-6.4.X.3 Atomic Add/Delete
	//----------------------------------------------------------------------
	t.Logf("%s.3 - Simultaneous Addition and Deletion", pArgs.testID)
	atomicPrefixSetSwap(t, dut, pArgs.prefixSet, pArgs.initialAllowedPrefixes[0], pArgs.initialAllowedPrefixes[1], pArgs.maskRange)
	verifyPrefixRemovedFromPrefixSet(t, dut, pArgs.prefixSet, pArgs.initialAllowedPrefixes[1], pArgs.maskRange)
	// Wait for BOTH notifications on SAME stream.
	aftpf.RunCollector(t,
		aftpf.RunCollectorParams{
			Ctx:       ctx,
			Collector: collector,
			Stop: aftcache.WaitForNotification(t,
				aftcache.NotificationExpectation{
					AddPrefix:        pArgs.initialAllowedPrefixes[0],
					DeletePrefix:     pArgs.initialAllowedPrefixes[1],
					NotificationWait: notificationWaitTime,
				}),
			Timeout: subscriptionWait,
		})
	//----------------------------------------------------------------------
	// Final verification
	//----------------------------------------------------------------------
	finalAFT, err := collector.ToAFT(t, dut)
	if err != nil {
		t.Fatalf("ToAFT failed: %v", err)
	}
	aftpf.VerifyPrefixesPresent(t, aftpf.PrefixesParams{InfoAFT: finalAFT, Prefixes: []string{pArgs.initialAllowedPrefixes[0], pArgs.dynamicPrefix}})
	aftpf.VerifyPrefixesAbsent(t, aftpf.PrefixesParams{InfoAFT: finalAFT, Prefixes: []string{pArgs.initialAllowedPrefixes[1]}})
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

// atomicPrefixSetSwap atomically updates a prefix set.
// It adds one prefix and removes another using a single gNMI Set transaction.
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

// configurePolicies configures all routing policies required for AFT prefix
// filtering tests and installs them on the DUT using the provided gNMI batch.
func configurePolicies(t *testing.T, dut *ondatra.DUTDevice, batch *gnmi.SetBatch) {
	t.Helper()
	root := &oc.Root{}
	rp := root.GetOrCreateRoutingPolicy()
	configureCommonPolicies(t, rp)
	configureDynamicUpdatePolicies(t, rp)
	gnmi.BatchReplace(batch, gnmi.OC().RoutingPolicy().Config(), rp)
	batch.Set(t, dut)
}

// configureDynamicUpdatePolicies configures IPv4 and IPv6 routing policies
// used by AFT-6.4 dynamic prefix-set update validation.
func configureDynamicUpdatePolicies(t *testing.T, rp *oc.RoutingPolicy) {
	t.Helper()
	// IPv4 dynamic prefix filtering policy.
	aftpf.AddPrefixSetPolicy(t, rp,
		aftpf.PrefixSetPolicyParams{
			PolicyName:     v4Policy,
			StatementNames: []string{"10"},
			PrefixSetNames: []string{v4PfxSet},
			PrefixList:     policyIPv4Prefixes,
			PrefixMode:     maskRange,
			PolicyResult:   oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
		})
	// IPv6 dynamic prefix filtering policy.
	aftpf.AddPrefixSetPolicy(t, rp,
		aftpf.PrefixSetPolicyParams{
			PolicyName:     v6Policy,
			StatementNames: []string{"10"},
			PrefixSetNames: []string{v6PfxSet},
			PrefixList:     policyIPv6Prefixes,
			PrefixMode:     maskRange,
			PolicyResult:   oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
		})
}

// configureCommonPolicies configures routing policies shared across multiple
// AFT prefix filtering test cases.
func configureCommonPolicies(t *testing.T, rp *oc.RoutingPolicy) {
	t.Helper()
	// POLICY-MATCH-ALL
	aftpf.AddPrefixSetPolicy(t, rp,
		aftpf.PrefixSetPolicyParams{
			PolicyName:     matchAllPolicy,
			StatementNames: []string{"10"},
			PolicyResult:   oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
		})
	// POLICY-SUBNET-V4
	aftpf.AddPrefixSetPolicy(t, rp,
		aftpf.PrefixSetPolicyParams{
			PolicyName:     "POLICY-SUBNET-V4",
			StatementNames: []string{"10"},
			PrefixSetNames: []string{"PREFIX-SET-SUBNET-V4"},
			MatchPrefixSet: true,
			PolicyResult:   oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
		})
	// POLICY-SUBNET-V6
	aftpf.AddPrefixSetPolicy(t, rp,
		aftpf.PrefixSetPolicyParams{
			PolicyName:     "POLICY-SUBNET-V6",
			StatementNames: []string{"10"},
			PrefixSetNames: []string{"PREFIX-SET-SUBNET-V6"},
			MatchPrefixSet: true,
			PolicyResult:   oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
		})
	// POLICY-MULTI-STMT
	aftpf.AddPrefixSetPolicy(t, rp,
		aftpf.PrefixSetPolicyParams{
			PolicyName:     "POLICY-MULTI-STMT",
			StatementNames: []string{"10", "20"},
			PrefixSetNames: []string{"PREFIX-SET-A", "PREFIX-SET-SUBNET"},
			MatchPrefixSet: true,
			PolicyResult:   oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
		})
	// POLICY-DENY-PREFIX-SET-A
	aftpf.AddPrefixSetPolicy(t, rp,
		aftpf.PrefixSetPolicyParams{
			PolicyName:     "POLICY-DENY-PREFIX-SET-A",
			StatementNames: []string{"10", "20"},
			PrefixSetNames: []string{"PREFIX-SET-A", ""},
			MatchPrefixSet: true,
			PrefixDeny:     true,
			PolicyResult:   oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
		})
	// POLICY-TAG-MATCH
	aftpf.AddPrefixSetPolicy(t, rp,
		aftpf.PrefixSetPolicyParams{
			PolicyName:     "POLICY-TAG-MATCH",
			StatementNames: []string{"10"},
			MatchPrefixSet: true,
			SetTag:         true,
			PolicyResult:   oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
		})
	// POLICY-PREFIX-SET-VRF-A
	aftpf.AddPrefixSetPolicy(t, rp,
		aftpf.PrefixSetPolicyParams{
			PolicyName:     "POLICY-PREFIX-SET-VRF-A",
			StatementNames: []string{"10"},
			PrefixSetNames: []string{"PREFIX-SET-VRF-A"},
			PrefixList:     []string{vrfV4Pfx},
			PrefixMode:     "24..32",
			PolicyResult:   oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
		})
	// POLICY-PREFIX-SET-VRF-B
	aftpf.AddPrefixSetPolicy(t, rp,
		aftpf.PrefixSetPolicyParams{
			PolicyName:     "POLICY-PREFIX-SET-VRF-B",
			StatementNames: []string{"20"},
			PrefixSetNames: []string{"PREFIX-SET-VRF-B"},
			PrefixList:     []string{vrfV6Pfx},
			PrefixMode:     "65..128",
			PolicyResult:   oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
		})
}
