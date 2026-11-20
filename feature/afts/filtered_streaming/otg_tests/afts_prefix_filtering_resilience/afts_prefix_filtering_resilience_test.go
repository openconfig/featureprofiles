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
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/netip"
	"sort"
	"strconv"
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
	"github.com/openconfig/featureprofiles/internal/helpers"
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
	// The final policy-change check must run before its 60-second deadline;
	// the AFT cache's default two-minute periodic check cannot do that.
	policyChangeCheckInterval = 5 * time.Second
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
	filterClient := &resilienceFilterClient{ctx: t.Context()}
	filterClient.connect(t, dut)
	t.Cleanup(func() { filterClient.close(t) })
	configureGlobalFilterPoliciesWithDiagnostics(t, dut, filterClient, aftpf.ConfigureGlobalFilterPoliciesParams{
		V4Policy: ipv4Policy,
		V6Policy: ipv6Policy,
		VRFName:  deviations.DefaultNetworkInstance(dut),
	})
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
		desc string
		test func(t *testing.T, dut *ondatra.DUTDevice, filterClient *resilienceFilterClient)
	}{
		{
			name: "AFTValidationAfterDeviceReboot",
			desc: "AFT-6.3.1-ValidationAfterDeviceReboot",
			test: testAfterReboot,
		},
		{
			name: "AFTScaleTest",
			desc: "AFT-6.3.2-ScaleTest",
			test: testScaleFiltering,
		},
		{
			name: "AFTPerNetworkInstanceFiltering",
			desc: "AFT-6.3.3-PerNetworkInstanceFiltering",
			test: testPerNIFiltering,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Log("Starting test")
			tc.test(t, dut, filterClient)
			t.Log("Finished test")
		})
	}
}

// configureOpenConfigDUT preserves the original non-Cisco configuration
// sequence, with policies installed before BGP attaches their references.
func configureOpenConfigDUT(t *testing.T, dut *ondatra.DUTDevice) *gnmi.SetBatch {
	t.Helper()
	batch := &gnmi.SetBatch{}
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	configureHardwareInit(t, dut)
	installRoutingPolicies(t, dut)
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

func configureNetworkInstanceOnDUTPort(t *testing.T, dut *ondatra.DUTDevice, batch *gnmi.SetBatch, attrs *attrs.Attributes, p *ondatra.Port, niName string) {
	t.Helper()
	d := gnmi.OC()
	cfgplugins.AssignToNetworkInstance(t, dut, p.Name(), niName, 0)
	i := attrs.NewOCInterface(p.Name(), dut)
	gnmi.BatchUpdate(batch, d.Interface(p.Name()).Config(), i)
}

// configureDUT selects the original setup sequence or Cisco's XR prerequisites.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) *gnmi.SetBatch {
	t.Helper()
	if dut.Vendor() != ondatra.CISCO {
		return configureOpenConfigDUT(t, dut)
	}
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	configureHardwareInit(t, dut)
	installRoutingPolicies(t, dut)

	// IOS XR needs the VRF address families before BGP can configure its
	// address families and neighbors in this network instance.
	nonDefaultNI := cfgplugins.ConfigureNetworkInstance(t, dut, vrfName, false)
	cfgplugins.UpdateNetworkInstanceOnDut(t, dut, vrfName, nonDefaultNI)
	if dut.Vendor() == ondatra.CISCO {
		helpers.GnmiCLIConfig(t, dut,
			"vrf "+vrfName+"\n address-family ipv4 unicast\n address-family ipv6 unicast\n!\n")
	}
	verifyVRFAddressFamilies(t, dut)

	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")
	configureDUTInterfaceBase(t, dut, p1)
	configureDUTInterfaceBase(t, dut, p2)
	// Cisco's unassigned port 1 is already in DEFAULT; explicitly assigning
	// the reserved routing-table name is rejected by XR.
	cfgplugins.AssignToNetworkInstance(t, dut, p2.Name(), vrfName, 0)
	verifyVRFAddressFamilies(t, dut)
	interfaceBatch := &gnmi.SetBatch{}
	configureDUTInterface(t, dut, interfaceBatch, &dutPort1, p1)
	configureDUTInterface(t, dut, interfaceBatch, &dutPort2, p2)
	interfaceBatch.Set(t, dut)

	cfgplugins.EnableDefaultNetworkInstanceBgp(t, dut, aftFilterDUTAS)
	defaultNI := cfgplugins.ConfigureNetworkInstance(t, dut, deviations.DefaultNetworkInstance(dut), true)
	configureScaleBGP(t, dut, defaultNI, nonDefaultNI)
	configureBGPGlobal(t, dut, defaultNI)
	initializeCiscoVRFBGPAddressFamilies(t, dut)
	configureBGPGlobal(t, dut, nonDefaultNI)
	configureBGPNeighbors(t, dut, defaultNI)
	configureBGPNeighbors(t, dut, nonDefaultNI)
	return &gnmi.SetBatch{}
}

// initializeCiscoVRFBGPAddressFamilies creates the global VPN parents and
// per-VRF BGP address families before OpenConfig updates the VRF's BGP protocol
// and neighbors.
func initializeCiscoVRFBGPAddressFamilies(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	if dut.Vendor() != ondatra.CISCO {
		return
	}
	// IOS XR requires VPNv4/VPNv6 in the same BGP instance before enabling
	// IPv4/IPv6 unicast within a BGP VRF.
	t.Log("Configuring global BGP VPNv4 and VPNv6 parent address families in instance BGP")
	helpers.GnmiCLIConfig(t, dut, fmt.Sprintf(
		"router bgp %d instance BGP\n address-family vpnv4 unicast\n !\n address-family vpnv6 unicast\n !\n!\n", aftFilterDUTAS))
	config := readCiscoBGPInstanceConfig(t, dut)
	for _, family := range []string{"vpnv4", "vpnv6"} {
		if !strings.Contains(config, "\n address-family "+family+" unicast\n") {
			t.Fatalf("BGP instance BGP is missing global %s unicast", family)
		}
	}
	rd := fmt.Sprintf("%d:63", aftFilterDUTAS)
	steps := []struct {
		name          string
		cli           string
		wantVRFConfig []string
	}{
		{
			name:          "BGP VRF context and route distinguisher",
			cli:           fmt.Sprintf("router bgp %d instance BGP\n vrf %s\n  rd %s\n !\n!\n", aftFilterDUTAS, vrfName, rd),
			wantVRFConfig: []string{"  rd " + rd},
		},
		{
			name:          "IPv4 BGP VRF address family",
			cli:           fmt.Sprintf("router bgp %d instance BGP\n vrf %s\n  address-family ipv4 unicast\n  !\n !\n!\n", aftFilterDUTAS, vrfName),
			wantVRFConfig: []string{"  rd " + rd, "  address-family ipv4 unicast"},
		},
		{
			name:          "IPv6 BGP VRF address family",
			cli:           fmt.Sprintf("router bgp %d instance BGP\n vrf %s\n  address-family ipv6 unicast\n  !\n !\n!\n", aftFilterDUTAS, vrfName),
			wantVRFConfig: []string{"  rd " + rd, "  address-family ipv4 unicast", "  address-family ipv6 unicast"},
		},
	}
	for _, step := range steps {
		t.Logf("Configuring %s in %s", step.name, vrfName)
		helpers.GnmiCLIConfig(t, dut, step.cli)
		verifyCiscoVRFBGPConfig(t, dut, step.wantVRFConfig...)
	}
}

func readCiscoBGPInstanceConfig(t *testing.T, dut *ondatra.DUTDevice) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	response, err := dut.RawAPIs().CLI(t).RunCommand(ctx, "show running-config router bgp")
	if err != nil {
		t.Fatalf("failed to read BGP running config: %v", err)
	}
	config := strings.ReplaceAll(response.Output(), "\r\n", "\n")
	t.Logf("BGP running config while initializing VRF %s:\n%s", vrfName, config)
	config = "\n" + config
	instanceMarker := fmt.Sprintf("router bgp %d instance BGP\n", aftFilterDUTAS)
	instanceIndex := strings.Index(config, "\n"+instanceMarker)
	if instanceIndex < 0 {
		t.Fatalf("BGP running config is missing %q", strings.TrimSpace(instanceMarker))
	}
	instanceConfig := config[instanceIndex+1:]
	if end := strings.Index(instanceConfig, "\nrouter bgp "); end >= 0 {
		instanceConfig = instanceConfig[:end]
	}
	return instanceConfig
}

func verifyCiscoVRFBGPConfig(t *testing.T, dut *ondatra.DUTDevice, wantLines ...string) {
	t.Helper()
	config := readCiscoBGPInstanceConfig(t, dut)
	vrfMarker := " vrf " + vrfName + "\n"
	vrfIndex := strings.Index(config, vrfMarker)
	if vrfIndex < 0 {
		t.Fatalf("BGP running config is missing VRF %s", vrfName)
	}
	vrfConfig := config[vrfIndex+len(vrfMarker):]
	if end := strings.Index(vrfConfig, "\n !"); end >= 0 {
		vrfConfig = vrfConfig[:end]
	}
	for _, want := range wantLines {
		if !strings.Contains("\n"+vrfConfig+"\n", "\n"+want+"\n") {
			t.Fatalf("BGP VRF %s running config is missing %q", vrfName, want)
		}
	}
}

// verifyVRFAddressFamilies checks that both VRF address families are present
// on Cisco before configuring BGP under the VRF.
func verifyVRFAddressFamilies(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	if dut.Vendor() != ondatra.CISCO {
		return
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	response, err := dut.RawAPIs().CLI(t).RunCommand(ctx, "show running-config vrf "+vrfName)
	if err != nil {
		t.Fatalf("failed to verify VRF %s address families in running config: %v", vrfName, err)
	}
	t.Logf("VRF %s running config:\n%s", vrfName, response.Output())
	if !strings.Contains(response.Output(), "vrf "+vrfName) {
		t.Fatalf("VRF %s is missing from the running config", vrfName)
	}
	for _, want := range []string{"address-family ipv4 unicast", "address-family ipv6 unicast"} {
		if !strings.Contains(response.Output(), want) {
			t.Fatalf("VRF %s running config is missing %q", vrfName, want)
		}
	}
}

// configureBGPGlobal commits the protocol's global address families before
// any neighbors are configured in either network instance.
func configureBGPGlobal(t *testing.T, dut *ondatra.DUTDevice, ni *oc.NetworkInstance) {
	t.Helper()
	const bgpName = "BGP"
	proto := ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName)
	bgp := proto.GetOrCreateBgp()
	globalOnly := &oc.NetworkInstance_Protocol{
		Identifier: oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP,
		Name:       ygot.String(bgpName),
		Enabled:    ygot.Bool(true),
		Bgp:        &oc.NetworkInstance_Protocol_Bgp{Global: bgp.GetOrCreateGlobal()},
	}
	path := gnmi.OC().NetworkInstance(ni.GetName()).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName)
	t.Logf("Configuring BGP global address families in %s before neighbors", ni.GetName())
	gnmi.Update(t, dut, path.Config(), globalOnly)
}

// configureBGPNeighbors commits each neighbor and its AF-specific policies
// after the global BGP address families exist on the DUT.
func configureBGPNeighbors(t *testing.T, dut *ondatra.DUTDevice, ni *oc.NetworkInstance) {
	t.Helper()
	const bgpName = "BGP"
	bgp := ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).GetOrCreateBgp()
	path := gnmi.OC().NetworkInstance(ni.GetName()).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName)
	var neighbors []string
	for addr := range bgp.Neighbor {
		neighbors = append(neighbors, addr)
	}
	sort.Strings(neighbors)
	for _, addr := range neighbors {
		t.Logf("Configuring BGP neighbor %s in %s", addr, ni.GetName())
		gnmi.Update(t, dut, path.Bgp().Neighbor(addr).Config(), bgp.Neighbor[addr])
	}
}

// configureDUTInterfaceBase creates the interface and subinterface before
// attaching them to a network instance; IP addresses are added afterward.
func configureDUTInterfaceBase(t *testing.T, dut *ondatra.DUTDevice, p *ondatra.Port) {
	t.Helper()
	i := &oc.Interface{
		Name: ygot.String(p.Name()),
		Type: oc.IETFInterfaces_InterfaceType_ethernetCsmacd,
	}
	if deviations.InterfaceEnabled(dut) {
		i.Enabled = ygot.Bool(true)
	}
	i.GetOrCreateEthernet()
	i.GetOrCreateSubinterface(0)
	gnmi.Update(t, dut, gnmi.OC().Interface(p.Name()).Config(), i)
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
	if dut.Vendor() != ondatra.CISCO || (deviations.InterfaceEnabled(dut) && !deviations.IPv4MissingEnabled(dut)) {
		i4.Enabled = ygot.Bool(true)
	}
	av4 := i4.GetOrCreateAddress(attrs.IPv4)
	av4.PrefixLength = ygot.Uint8(attrs.IPv4Len)
	i6 := i.GetOrCreateSubinterface(0).GetOrCreateIpv6()
	i6.Enabled = ygot.Bool(true)
	av6 := i6.GetOrCreateAddress(attrs.IPv6)
	av6.PrefixLength = ygot.Uint8(attrs.IPv6Len)
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
		return nil, fmt.Errorf("aft inconsistency detected: %q", diff)
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
// instance and in the non-default VRF to establish and for route readiness.
// The shared helper retains DEFAULT's installed-counter checks. For Cisco
// VRFs, whose per-peer RIB-installed counter is unsupported, it checks native
// BGP accepted/best-path counts and BGP-origin AFT summaries instead. This is
// only setup validation; the later per-prefix/filter-isolation checks remain.
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
func testAfterReboot(t *testing.T, dut *ondatra.DUTDevice, filterClient *resilienceFilterClient) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	wantPrefixes := aftpf.GeneratePrefixes(t, aftpf.GeneratePrefixesParams{V4Prefixes: []string{matchPrefixAft1, matchPrefixAft2}, V6Prefixes: policyIPv6Prefixes, PfxCount: pfxCount})
	// Verify configured policies before reboot.
	verifyGlobalFilterPolicies(t, dut, ipv4Policy, ipv6Policy)
	// Establish initial gNMI subscriptions.
	aftClient1 := aftpf.GnmiClientSession(t, dut, aftpf.PrefixesParams{Ctx: ctx})
	aftSession1 := aftcache.NewAFTStreamSession(ctx, t, aftClient1, dut)
	aftSession2 := aftcache.NewAFTStreamSession(ctx, t, aftpf.GnmiClientSession(t, dut, aftpf.PrefixesParams{Ctx: ctx}), dut)
	t.Log("Collecting initial filtered AFT entries")
	stoppingCondition := aftcache.InitialSyncStoppingConditionWithNextHopValidator(t, dut, wantPrefixes, verifiedDefaultPeerValidator(ctx, t, dut))
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
	filterClient.close(t)
	t.Logf("AFT reboot marker: issuing reboot UTC=%s; test-owned filter client retired", time.Now().UTC().Format(time.RFC3339Nano))
	rebootDUT(t, dut)
	// The subscription is active while the DUT reboots; verify the stream is
	// terminated by the DUT going down.
	verifyStreamTerminated(t, monitoredStream)
	// Wait for DUT recovery.
	waitForReboot(t, dut, lastBootTime)
	t.Logf("AFT reboot marker: DUT recovery observed UTC=%s", time.Now().UTC().Format(time.RFC3339Nano))
	// Never send the next policy Set over the pre-reboot transport. Reconnect
	// explicitly; a failed Set still fails the test and is not retried.
	filterClient.connect(t, dut)
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
	// Re-read neighbor identity after reboot rather than trusting pre-reboot ND.
	stoppingCondition2 := aftcache.InitialSyncStoppingConditionWithNextHopValidator(t, dut, wantPrefixes, verifiedDefaultPeerValidator(ctx, t, dut))
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
		gnmiClient, err := dut.RawAPIs().BindingDUT().DialGNMI(t.Context())
		if err != nil {
			t.Fatalf("Failed to dial GNMI: %v", err)
		}
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		configReq := &gpb.GetRequest{
			Prefix: &gpb.Path{Origin: "openconfig", Target: dut.Name()},
			Path: []*gpb.Path{
				gnmiPath(t, ocPolicyConfigPathV4),
				gnmiPath(t, ocPolicyConfigPathV6),
			},
			Type:     gpb.GetRequest_CONFIG,
			Encoding: aftpf.ConfigEncoding(dut),
		}
		configResp, err := gnmiClient.Get(ctx, configReq)
		if err != nil {
			t.Fatalf("GNMI Get for global-filter config failed: %v", err)
		}
		stateReq := &gpb.GetRequest{
			Prefix: &gpb.Path{Origin: "openconfig", Target: dut.Name()},
			Path: []*gpb.Path{
				gnmiPath(t, ocPolicyStatePathV4),
				gnmiPath(t, ocPolicyStatePathV6),
			},
			Type:     gpb.GetRequest_STATE,
			Encoding: aftpf.ConfigEncoding(dut),
		}
		stateResp, err := gnmiClient.Get(ctx, stateReq)
		if err != nil {
			t.Fatalf("GNMI Get for global-filter state failed: %v", err)
		}
		gotConfigIPv4Policy, gotConfigIPv6Policy := globalFilterPoliciesFromGet(t, configResp)
		gotStateIPv4Policy, gotStateIPv6Policy := globalFilterPoliciesFromGet(t, stateResp)
		if gotConfigIPv4Policy != wantIPv4Policy {
			t.Fatalf("IPv4 config policy mismatch got=%q want=%q", gotConfigIPv4Policy, wantIPv4Policy)
		}
		if gotConfigIPv6Policy != wantIPv6Policy {
			t.Fatalf("IPv6 config policy mismatch got=%q want=%q", gotConfigIPv6Policy, wantIPv6Policy)
		}
		if gotStateIPv4Policy != wantIPv4Policy {
			t.Fatalf("IPv4 state policy mismatch got=%q want=%q", gotStateIPv4Policy, wantIPv4Policy)
		}
		if gotStateIPv6Policy != wantIPv6Policy {
			t.Fatalf("IPv6 state policy mismatch got=%q want=%q", gotStateIPv6Policy, wantIPv6Policy)
		}
		t.Logf("Verified persisted global-filter config/state policies for %q: ipv4=%q ipv6=%q", niName, gotConfigIPv4Policy, gotConfigIPv6Policy)
	}
}

// typedStringLeaves handles both scalar and container JSON_IETF responses.
// Namespace prefixes on JSON field names are stripped, not policy values.
func typedStringLeaves(value *gpb.TypedValue, leaf string, fullPath string) ([]string, error) {
	var decoded any
	switch v := value.GetValue().(type) {
	case *gpb.TypedValue_StringVal:
		decoded = v.StringVal
	case *gpb.TypedValue_JsonIetfVal:
		if err := json.Unmarshal(v.JsonIetfVal, &decoded); err != nil {
			return nil, err
		}
	case *gpb.TypedValue_JsonVal:
		if err := json.Unmarshal(v.JsonVal, &decoded); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported string-leaf value %T", value.GetValue())
	}
	if scalar, ok := decoded.(string); ok {
		if strings.HasSuffix(fullPath, "/"+leaf) {
			return []string{scalar}, nil
		}
		return nil, nil
	}
	var values []string
	var visit func(any) error
	visit = func(node any) error {
		switch node := node.(type) {
		case map[string]any:
			for key, child := range node {
				if i := strings.LastIndex(key, ":"); i >= 0 {
					key = key[i+1:]
				}
				if key == leaf {
					s, ok := child.(string)
					if !ok {
						return fmt.Errorf("%s is %T, not a string", leaf, child)
					}
					values = append(values, s)
				} else if err := visit(child); err != nil {
					return err
				}
			}
		case []any:
			for _, child := range node {
				if err := visit(child); err != nil {
					return err
				}
			}
		}
		return nil
	}
	err := visit(decoded)
	return values, err
}

func notificationUpdatePath(t *testing.T, prefix, path *gpb.Path) string {
	t.Helper()
	// A scalar update may carry its entire path in either field.
	if prefix == nil {
		prefix = &gpb.Path{}
	}
	if path == nil {
		path = &gpb.Path{}
	}
	prefixText, err := ygot.PathToString(prefix)
	if err != nil {
		t.Fatalf("Invalid notification prefix: %v", err)
	}
	pathText, err := ygot.PathToString(path)
	if err != nil {
		t.Fatalf("Invalid update path: %v", err)
	}
	return strings.TrimRight(prefixText, "/") + "/" + strings.TrimLeft(pathText, "/")
}

func globalFilterPoliciesFromGet(t *testing.T, response *gpb.GetResponse) (string, string) {
	t.Helper()
	policies := map[string]string{}
	for _, notification := range response.GetNotification() {
		for _, update := range notification.GetUpdate() {
			path := notificationUpdatePath(t, notification.GetPrefix(), update.GetPath())
			for _, leaf := range []string{"ipv4-policy", "ipv6-policy"} {
				values, err := typedStringLeaves(update.GetVal(), leaf, path)
				if err != nil {
					t.Fatalf("Decode global-filter %s at %s: %v", leaf, path, err)
				}
				for _, value := range values {
					if old, present := policies[leaf]; present && old != value {
						t.Fatalf("Conflicting %s values: %q and %q", leaf, old, value)
					}
					policies[leaf] = value
				}
			}
		}
	}
	for _, leaf := range []string{"ipv4-policy", "ipv6-policy"} {
		if _, present := policies[leaf]; !present {
			t.Fatalf("Get response is missing global-filter %s: %v", leaf, response)
		}
	}
	return policies["ipv4-policy"], policies["ipv6-policy"]
}

// verifiedDefaultPeerValidator accepts the exact expected global next hop,
// permitting an absent interface-ref but rejecting a conflicting one. A
// link-local next hop additionally requires a verified MAC/interface mapping.
// It never accepts an arbitrary fe80:: address or rewrites cached telemetry.
func verifiedDefaultPeerValidator(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice) func(string, []*aftcache.NextHop) (bool, error) {
	t.Helper()
	if dut.Vendor() != ondatra.CISCO {
		// Retain the original exact ATE IP checks. The verified link-local
		// alternative and interface checks below address Cisco telemetry.
		return func(prefix string, nextHops []*aftcache.NextHop) (bool, error) {
			if len(nextHops) == 0 {
				return false, nil
			}
			want := atePort1.IPv4
			if strings.Contains(prefix, ":") {
				want = atePort1.IPv6
			}
			for _, nextHop := range nextHops {
				if nextHop == nil || nextHop.IP != want {
					return false, nil
				}
			}
			return true, nil
		}
	}
	intf := dut.Port(t, "port1").Name()
	mac, err := net.ParseMAC(atePort1.MAC)
	if err != nil || len(mac) != 6 {
		t.Fatalf("ATE port1 must have a 48-bit MAC, got %q (%v)", atePort1.MAC, err)
	}
	// OTG's default IPv6 link-local address uses modified EUI-64. Derivation
	// gives a candidate only; ND must confirm that address belongs to this MAC.
	address := [16]byte{0xfe, 0x80}
	copy(address[8:], []byte{mac[0] ^ 0x02, mac[1], mac[2], 0xff, 0xfe, mac[3], mac[4], mac[5]})
	linkLocal := netip.AddrFrom16(address)
	global := netip.MustParseAddr(atePort1.IPv6)
	neighborPath := fmt.Sprintf("/interfaces/interface[name=%s]/subinterfaces/subinterface[index=0]/ipv6/neighbors/neighbor[ip=%s]/state/link-layer-address", intf, linkLocal)
	path := gnmiPath(t, neighborPath)
	path.Origin = "openconfig"
	readCtx, readCancel := context.WithDeadline(ctx, time.Now().Add(30*time.Second))
	response, readErr := dut.RawAPIs().GNMI(t).Get(readCtx, &gpb.GetRequest{
		Prefix: &gpb.Path{Target: dut.Name()}, Path: []*gpb.Path{path},
		Type: gpb.GetRequest_STATE, Encoding: aftpf.ConfigEncoding(dut),
	})
	readCancel()
	verified := false
	verificationDetail := "gNMI returned no matching neighbor MAC"
	if readErr != nil {
		verificationDetail = fmt.Sprintf("gNMI lookup failed: %v", readErr)
		t.Logf("Cannot verify IPv6 link-local peer via %s: %v", neighborPath, readErr)
	} else {
		var returnedMACs []string
		for _, notification := range response.GetNotification() {
			for _, update := range notification.GetUpdate() {
				values, err := typedStringLeaves(update.GetVal(), "link-layer-address", notificationUpdatePath(t, notification.GetPrefix(), update.GetPath()))
				if err != nil {
					t.Logf("Cannot decode IPv6 neighbor MAC: %v", err)
					continue
				}
				returnedMACs = append(returnedMACs, values...)
			}
		}
		// A missing or conflicting mapping must not broaden the expectation.
		verified = len(returnedMACs) > 0
		for _, value := range returnedMACs {
			got, err := net.ParseMAC(value)
			if err != nil || !strings.EqualFold(got.String(), mac.String()) {
				verified = false
			}
		}
		verificationDetail = fmt.Sprintf("gNMI returned MACs=%v, expected=%s", returnedMACs, mac)
		t.Logf("IPv6 peer identity: interface=%s global=%s candidate link-local=%s expected MAC=%s returned MACs=%v verified=%t", intf, global, linkLocal, mac, returnedMACs, verified)
	}
	if !verified && dut.Vendor() == ondatra.CISCO {
		// The image may not expose the OC neighbor MAC leaf. Verify the same
		// address/MAC/interface association through native ND, not merely
		// the EUI-64 derivation or a match on an arbitrary link-local address.
		cliCtx, cliCancel := context.WithDeadline(ctx, time.Now().Add(20*time.Second))
		neighbors, cliErr := dut.RawAPIs().CLI(t).RunCommand(cliCtx, "show ipv6 neighbors")
		cliCancel()
		if cliErr != nil {
			verificationDetail += fmt.Sprintf("; native ND lookup failed: %v", cliErr)
			t.Logf("Native IPv6 neighbor verification failed: %v", cliErr)
		} else {
			t.Logf("Native IPv6 neighbors for peer identity verification:\n%s", neighbors.Output())
			var matchErr error
			verified, matchErr = ciscoIPv6NeighborMatches(t, neighbors.Output(), intf, linkLocal, mac)
			verificationDetail += fmt.Sprintf("; native ND verified=%t, detail=%v", verified, matchErr)
			t.Logf("Native IPv6 peer identity: interface=%s link-local=%s expected MAC=%s verified=%t detail=%v", intf, linkLocal, mac, verified, matchErr)
		}
	}
	var logMu sync.Mutex
	logged := map[string]bool{}
	diagnosed := map[uint64]bool{}
	return func(prefix string, nextHops []*aftcache.NextHop) (bool, error) {
		if len(nextHops) == 0 {
			return false, nil
		}
		wantIPv4 := netip.MustParseAddr(atePort1.IPv4)
		for _, nextHop := range nextHops {
			if nextHop == nil || nextHop.IP == "" {
				// A partially received NH can carry its interface before its IP.
				return false, nil
			}
			ip, err := netip.ParseAddr(nextHop.IP)
			if err != nil {
				return false, fmt.Errorf("invalid next-hop address %q: %w", nextHop.IP, err)
			}
			ip = ip.WithZone("")
			correctInterface := nextHop.IntfName == intf || nextHop.IntfName == intf+".0"
			// Static/global IP next hops may omit interface-ref. Restore the
			// original exact-IP validation in that case, but never ignore an
			// explicitly wrong interface. Link-local next hops remain scoped.
			if !ip.IsLinkLocalUnicast() && nextHop.IntfName == "" {
				correctInterface = true
				logMu.Lock()
				// Limit raw Gets to four distinct NH entries per validator;
				// concurrent collectors must not repeatedly dump the NH table.
				diagnose := !diagnosed[nextHop.Index] && len(diagnosed) < 4
				if diagnose {
					diagnosed[nextHop.Index] = true
				}
				logMu.Unlock()
				if diagnose {
					logRawNextHop(ctx, t, dut, prefix, nextHop)
				}
			}
			correctIP := ip == wantIPv4
			if strings.Contains(prefix, ":") {
				if ip == linkLocal && !verified {
					// Repeating AFT convergence checks cannot repair a missing
					// identity lookup. Fail with both lookup results immediately.
					return false, fmt.Errorf("cannot verify IPv6 next hop %s on %s for prefix %s: %s", ip, intf, prefix, verificationDetail)
				}
				correctIP = ip == global || (verified && ip == linkLocal)
			}
			if !correctInterface || !correctIP {
				key := nextHop.IP + "@" + nextHop.IntfName
				logMu.Lock()
				if !logged[key] {
					t.Logf("Next-hop identity mismatch for %s: IP=%s interface=%q, want port1=%s; link-local %s verified=%t", prefix, nextHop.IP, nextHop.IntfName, intf, linkLocal, verified)
					logged[key] = true
				}
				logMu.Unlock()
				return false, nil
			}
		}
		return true, nil
	}
}

// ciscoIPv6NeighborMatches requires the address, interface and MAC in the same
// native neighbor record. Wrapped rows are supported; an entry on another port
// cannot verify this peer, and conflicting MACs on the expected port fail.
func ciscoIPv6NeighborMatches(t *testing.T, output, intf string, address netip.Addr, wantMAC net.HardwareAddr) (bool, error) {
	t.Helper()
	interfaceNames := map[string]bool{intf: true, intf + ".0": true}
	// XR's native ND table prints FourHundredGigE as FH. Accept that
	// specific alias with the complete port suffix, not a substring match
	// that could mistake another port (for example /10 versus /100).
	if strings.HasPrefix(intf, "FourHundredGigE") {
		shortName := "FH" + strings.TrimPrefix(intf, "FourHundredGigE")
		interfaceNames[shortName] = true
		interfaceNames[shortName+".0"] = true
	}
	var records [][]string
	// Only a partially printed neighbor may consume a continuation line.
	// In particular, [Mcast adjacency] is a separate record, not a neighbor's
	// continuation: its zero MAC must never be attributed to the peer above it.
	pending := -1
	complete := func(fields []string) bool {
		hasMAC, hasInterface := false, false
		for _, field := range fields {
			if mac, err := net.ParseMAC(field); err == nil && len(mac) == 6 {
				hasMAC = true
			}
			if strings.Contains(field, "/") && !strings.Contains(field, ":") {
				// Interface names have a port suffix; locations such as
				// 0/0/CPU0 do not start with a letter.
				hasInterface = hasInterface || (field[0] >= 'A' && field[0] <= 'Z') || (field[0] >= 'a' && field[0] <= 'z')
			}
		}
		return hasMAC && hasInterface
	}
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			pending = -1
			continue
		}
		if ip, err := netip.ParseAddr(fields[0]); err == nil && ip.Is6() {
			records = append(records, fields)
			pending = len(records) - 1
		} else {
			// Wrapped XR rows begin with an age or a MAC. Reject headers,
			// multicast rows and unrelated CLI text as record boundaries.
			_, ageErr := strconv.ParseUint(fields[0], 10, 64)
			mac, macErr := net.ParseMAC(fields[0])
			continuation := ageErr == nil || fields[0] == "-" || (macErr == nil && len(mac) == 6) || interfaceNames[fields[0]]
			if pending < 0 || !continuation {
				pending = -1
				continue
			}
			records[pending] = append(records[pending], fields...)
		}
		if pending >= 0 && complete(records[pending]) {
			pending = -1
		}
	}
	matched := false
	for _, record := range records {
		ip, err := netip.ParseAddr(record[0])
		if err != nil || ip.WithZone("") != address {
			continue
		}
		correctInterface := false
		var macs []net.HardwareAddr
		for _, field := range record[1:] {
			if interfaceNames[field] {
				correctInterface = true
			}
			if mac, err := net.ParseMAC(field); err == nil && len(mac) == 6 {
				macs = append(macs, mac)
			}
		}
		if !correctInterface {
			continue
		}
		t.Logf("IPv6 neighbor validation selected record: address=%s interface=%s fields=%v", address, intf, record)
		if len(macs) == 0 {
			return false, fmt.Errorf("neighbor %s on %s has no resolved MAC: %v", address, intf, record)
		}
		for _, mac := range macs {
			if !strings.EqualFold(mac.String(), wantMAC.String()) {
				return false, fmt.Errorf("neighbor %s on %s has MAC %s, want %s", address, intf, mac, wantMAC)
			}
		}
		matched = true
	}
	if !matched {
		return false, fmt.Errorf("no native neighbor record for %s with MAC %s on %s", address, wantMAC, intf)
	}
	return true, nil
}

func logRawNextHop(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, prefix string, nextHop *aftcache.NextHop) {
	t.Helper()
	t.Logf("Global next hop has no decoded interface-ref: prefix=%s index=%d IP=%s decoded=%+v; checking raw DUT telemetry", prefix, nextHop.Index, nextHop.IP, nextHop)
	path := gnmiPath(t, fmt.Sprintf("/network-instances/network-instance[name=%s]/afts/next-hops/next-hop[index=%d]", deviations.DefaultNetworkInstance(dut), nextHop.Index))
	readCtx, cancel := context.WithDeadline(ctx, time.Now().Add(10*time.Second))
	defer cancel()
	response, err := dut.RawAPIs().GNMI(t).Get(readCtx, &gpb.GetRequest{
		Prefix: &gpb.Path{Origin: "openconfig", Target: dut.Name()}, Path: []*gpb.Path{path},
		Type: gpb.GetRequest_STATE, Encoding: aftpf.ConfigEncoding(dut),
	})
	if err != nil {
		t.Logf("Raw next-hop diagnostic for index %d failed: %v", nextHop.Index, err)
		return
	}
	// This is a fresh read, not a replacement for the subscription cache.
	t.Logf("Raw next-hop diagnostic for index %d: %v", nextHop.Index, response)
}

// gnmiPath converts a string-based gNMI path into a structured gNMI Path.
func gnmiPath(t *testing.T, path string) *gpb.Path {
	t.Helper()
	p, err := ygot.StringToPath(path, ygot.StructuredPath)
	if err != nil {
		t.Fatalf("Failed to parse path %q: %v", path, err)
	}
	return p
}

// installRoutingPolicies installs all routing policies before BGP or the AFT
// global filter references them.
func installRoutingPolicies(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	root := &oc.Root{}
	rp := root.GetOrCreateRoutingPolicy()
	ipv4ScalePrefixes := generateScaleIPv4Prefixes(t)
	ipv6ScalePrefixes := generateScaleIPv6Prefixes(t)
	configureBasePolicies(t, rp)
	configureScalePolicies(t, rp, ipv4ScalePrefixes, ipv6ScalePrefixes)
	policyBatch := &gnmi.SetBatch{}
	gnmi.BatchReplace(policyBatch, gnmi.OC().RoutingPolicy().Config(), rp)
	policyBatch.Set(t, dut)
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
		prefixes = append(prefixes, fmt.Sprintf("%s/%d", ip, scaleV6PfxLen))
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
					t.Logf("Time elapsed %.2f seconds, DUT not reachable yet: %q.", time.Since(startReboot).Seconds(), *errMsg)
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
	gnoiClient, err := dut.RawAPIs().BindingDUT().DialGNOI(t.Context())
	if err != nil {
		t.Fatalf("Error dialing gNOI: %v", err)
	}
	bootTimeBeforeReboot := gnmi.Get(t, dut, gnmi.OC().System().BootTime().State())
	t.Logf("DUT boot time before reboot: %v", bootTimeBeforeReboot)
	t.Log("Sending reboot request to DUT")
	ctxWithTimeout, cancel := context.WithTimeout(t.Context(), maxRebootTime)
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
func testScaleFiltering(t *testing.T, dut *ondatra.DUTDevice, filterClient *resilienceFilterClient) {
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
	// Record the post-reboot baseline before scale policy Sets, so old MDT
	// closures can be distinguished from any new transport/queue failure.
	logAFTDiagnosticCLI(t.Context(), t, dut, "show logging")
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Scope the streaming context to this scenario. Each scenario opens two
			// AFT stream sessions, and every session starts a background reader
			// goroutine that lives until its context is cancelled. Cancelling here
			// tears the sessions down at the end of the scenario so that the DUT
			// never has to serve the subscriptions of all scenarios at once.
			ctx, cancel := context.WithCancel(t.Context())
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
				configureGlobalFilterPoliciesWithDiagnostics(t, dut, filterClient, aftpf.ConfigureGlobalFilterPoliciesParams{V4Policy: tc.policyName, V6Policy: "", VRFName: deviations.DefaultNetworkInstance(dut)})
				stoppingCondition = aftcache.InitialSyncStoppingCondition(t, dut, wantPrefixes, map[string]bool{atePort1.IPv4: true}, nil)
			} else {
				configureGlobalFilterPoliciesWithDiagnostics(t, dut, filterClient, aftpf.ConfigureGlobalFilterPoliciesParams{V4Policy: "", V6Policy: tc.policyName, VRFName: deviations.DefaultNetworkInstance(dut)})
				stoppingCondition = aftcache.InitialSyncStoppingConditionWithNextHopValidator(t, dut, wantPrefixes, verifiedDefaultPeerValidator(ctx, t, dut))
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
			t.Logf("Verified scale filtering for %q: %d matched prefixes present, %d unmatched prefixes absent", tc.name, len(selectedPrefixes), len(unmatchedPrefixes))
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
			t.Fatalf("Expected %q prefix missing from filtered AFT: %q", addressFamily, pfx)
		}
	}
	// Verify installed-but-unmatched prefixes were filtered out.
	for _, pfx := range unmatchedPrefixes {
		if _, ok := aftPrefixes.Prefixes[pfx]; ok {
			t.Fatalf("Unmatched %q prefix present after filtering: %q", addressFamily, pfx)
		}
	}
	t.Logf("Verified %q filtered prefixes", addressFamily)
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
func testPerNIFiltering(t *testing.T, dut *ondatra.DUTDevice, filterClient *resilienceFilterClient) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	// Configure NI-specific AFT filters
	t.Log("Configuring per-network-instance AFT filters")
	configureGlobalFilterPoliciesWithDiagnostics(t, dut, filterClient, aftpf.ConfigureGlobalFilterPoliciesParams{V4Policy: ipv4Policy, V6Policy: ipv6Policy, VRFName: deviations.DefaultNetworkInstance(dut)})
	configureGlobalFilterPoliciesWithDiagnostics(t, dut, filterClient, aftpf.ConfigureGlobalFilterPoliciesParams{V4Policy: vrfAPolicy, V6Policy: vrfAPolicy, VRFName: vrfName})
	// Expected prefixes
	defaultWant := aftpf.GeneratePrefixes(t, aftpf.GeneratePrefixesParams{V4Prefixes: []string{matchPrefixAft1, matchPrefixAft2}, V6Prefixes: nil, PfxCount: pfxCount})
	// POLICY-PREFIX-SET-VRF-A admits 100.64.1.0/24, not the DEFAULT
	// policy's 198.51.100.0/24 (which the negative check below excludes).
	vrfWant := aftpf.GeneratePrefixes(t, aftpf.GeneratePrefixesParams{V4Prefixes: []string{matchVrfPfx1}, V6Prefixes: nil, PfxCount: pfxCount})
	// Collector-1 (DEFAULT)
	collector1 := aftcache.NewAFTStreamSession(ctx, t, aftpf.GnmiClientSession(t, dut, aftpf.PrefixesParams{Ctx: ctx}), dut)
	// Collector-2: prefixes remain scoped to VRF-A. The session helper uses
	// DEFAULT for Cisco's global NHGs/NHs, and VRF-A for other vendors.
	// Shared forwarding objects are not leaked prefixes; the route-isolation
	// checks below still examine only this collector's VRF-A prefix tables.
	collector2 := aftcache.NewAFTStreamSessionForNetworkInstance(ctx, t, aftpf.GnmiClientSession(t, dut, aftpf.PrefixesParams{Ctx: ctx}), dut, vrfName)
	defer func() {
		if t.Failed() {
			logVRFNextHopDiagnostics(t, dut, collector2, matchVrfPfx1)
		}
	}()
	t.Logf("AFT per-NI collectors created UTC=%s DEFAULT=%+v VRF-A=%+v", time.Now().UTC().Format(time.RFC3339Nano), collector1.Progress(), collector2.Progress())
	// Initial sync validation
	t.Log("Validating initial filtered AFT state")
	defaultStop := aftcache.InitialSyncStoppingCondition(t, dut, defaultWant, map[string]bool{atePort1.IPv4: true}, nil)
	vrfStop := aftcache.InitialSyncStoppingCondition(t, dut, vrfWant, map[string]bool{atePort2.IPv4: true}, nil)
	checkVRF := vrfStop.PeriodicFunc
	vrfStop.PeriodicFunc = func(session *aftcache.AFTStreamSession) (bool, error) {
		t.Logf("AFT VRF-A readiness UTC=%s stream=%+v (CLIENT buffer, not DUT queue)", time.Now().UTC().Format(time.RFC3339Nano), session.Progress())
		return checkVRF(session)
	}
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
	configureGlobalFilterPoliciesWithDiagnostics(t, dut, filterClient, aftpf.ConfigureGlobalFilterPoliciesParams{V4Policy: matchAllPolicy, V6Policy: matchAllPolicy, VRFName: vrfName})
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
	collector2 = reestablishAFTSession(ctx, t, dut, vrfName)
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
		Description: fmt.Sprintf("%q: await canary %q while checking for leaked prefixes", cfg.CollectorName, cfg.CanaryPrefix),
		PeriodicFunc: func(ss *aftcache.AFTStreamSession) (bool, error) {
			aft, err := ss.ToAFT(t, dut)
			if err != nil {
				return false, err
			}
			for _, prefix := range cfg.ForbiddenPrefixes {
				if _, ok := aft.Prefixes[prefix]; ok {
					return false, fmt.Errorf("%q received filtered-out prefix %q", cfg.CollectorName, prefix)
				}
			}
			if _, ok := aft.Prefixes[cfg.CanaryPrefix]; !ok {
				return false, nil
			}
			t.Logf("%q received canary prefix %q; no filtered-out prefixes leaked", cfg.CollectorName, cfg.CanaryPrefix)
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
	session.ListenUntilWithInterval(ctx, t, timeout, policyChangeCheckInterval, stoppingCondition)
}

// reestablishAFTSession dials a new gNMI client and returns a fresh AFT stream
// session. It is used after a global-filter policy reconfiguration, which the
// DUT is allowed to answer by terminating the existing AFT subscription. The
// previous session's stream may therefore already be closed, and continuing to
// listen on it would fail instead of validating the new filter behaviour, so
// the verification is always performed on a re-established subscription.
// The constructor preserves the prefix NI and Cisco's DEFAULT NHG/NH scope.
func reestablishAFTSession(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, networkInstance string) *aftcache.AFTStreamSession {
	t.Helper()
	return aftcache.NewAFTStreamSessionForNetworkInstance(ctx, t, aftpf.GnmiClientSession(t, dut, aftpf.PrefixesParams{Ctx: ctx}), dut, networkInstance)
}
