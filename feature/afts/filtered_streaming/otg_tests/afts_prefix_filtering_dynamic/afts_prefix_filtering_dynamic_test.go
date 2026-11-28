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
	"net"
	"net/netip"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	aftpf "github.com/openconfig/featureprofiles/internal/aft_prefix_filtering"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/featureprofiles/internal/telemetry/aftcache"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

const (
	vrfName                   = "VRF-A"
	v4PfxSet                  = "PREFIX-SET-A"
	v6PfxSet                  = "PREFIX-SET-B"
	v4Policy                  = "POLICY-PREFIX-SET-A"
	v6Policy                  = "POLICY-PREFIX-SET-B"
	matchAllPolicy            = "POLICY-MATCH-ALL"
	subscriptionWait          = 5 * time.Minute
	prefixAFT1V4              = "198.51.100.0/24"
	prefixAFT2V4              = "203.0.113.0/28"
	prefixAFT3V4              = "192.0.2.0/24"
	prefixAFT3V4Cisco         = "198.51.101.0/24"
	prefixAFT1V6              = "2001:db8:2::/64"
	prefixAFT2V6              = "2001:db8:2::1/128"
	prefixAFT3V6              = "2001:db8:2::2/128"
	vrfV4Pfx                  = "100.64.1.0/24"
	vrfV6Pfx                  = "2001:db8:3::/64"
	vrfRouteV4Pfx             = "90.0.0.1"
	vrfRouteV6Pfx             = "4000::1"
	maskRange                 = "exact"
	notificationWaitTime      = 30 * time.Second
	notificationCheckInterval = 5 * time.Second
	staticRouteIndex          = 100
	pfxCount                  = 1
	aftFilterDUTAS            = 65001
	vrfRoutes                 = 5000
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

func dynamicIPv4Prefix(dut *ondatra.DUTDevice) string {
	if dut.Vendor() == ondatra.CISCO {
		// Cisco-specific: prefixes covering IP next-hop addresses remain
		// visible implicitly even when not specified by the AFT
		// global-filter policy. 192.0.2.0/24 covers the ATE next hop
		// 192.0.2.2, so use a different prefix such that adding it to the
		// prefix set causes a visibility change.
		// Route setup and the dynamic policy checks must use the same prefix.
		return prefixAFT3V4Cisco
	}
	return prefixAFT3V4
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
	v4Prefixes := append([]string(nil), defaultIPv4Prefixes...)
	for i, prefix := range v4Prefixes {
		if prefix == prefixAFT3V4 {
			v4Prefixes[i] = dynamicIPv4Prefix(dut)
		}
	}
	aftpf.ConfigureNetworkInstanceStaticRoute(t, dut, batch, aftpf.NetworkInstanceStaticRouteParams{
		DefaultPrefixes:     v4Prefixes,
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
	configureBGP(t, dut, defaultNI, nonDefaultNI)
	cfgplugins.UpdateNetworkInstanceOnDut(t, dut, defaultNI.GetName(), defaultNI)
	cfgplugins.UpdateNetworkInstanceOnDut(t, dut, vrfName, nonDefaultNI)
	configureDUTPort(t, dut, batch, &dutPort2, p2, vrfName)
	batch.Set(t, dut)
	aftpf.ApplyBGPMaxPrefixes(t, dut, aftpf.BGPPrefixParams{V4Prefix: atePort1.IPv4, V6Prefix: atePort1.IPv6, NetworkInstance: defaultNI})
	return batch
}

func configureDUTPort(t *testing.T, dut *ondatra.DUTDevice, batch *gnmi.SetBatch, attrs *attrs.Attributes, p *ondatra.Port, niName string) {
	t.Helper()
	ocPath := gnmi.OC()
	cfgplugins.AssignToNetworkInstance(t, dut, p.Name(), niName, 0)
	i := attrs.NewOCInterface(p.Name(), dut)
	gnmi.BatchUpdate(batch, ocPath.Interface(p.Name()).Config(), i)
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
	configureBGP(t, dut, defaultNI, nonDefaultNI)
	configureBGPGlobal(t, dut, defaultNI)
	initializeCiscoVRFBGPAddressFamilies(t, dut)
	configureBGPGlobal(t, dut, nonDefaultNI)
	configureBGPNeighbors(t, dut, defaultNI)
	configureBGPNeighbors(t, dut, nonDefaultNI)
	aftpf.ApplyBGPMaxPrefixes(t, dut, aftpf.BGPPrefixParams{V4Prefix: atePort1.IPv4, V6Prefix: atePort1.IPv6, NetworkInstance: defaultNI})
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
	rd := fmt.Sprintf("%d:64", aftFilterDUTAS)
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
	ocPath := gnmi.OC()
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
			dynamicPrefix:          dynamicIPv4Prefix(dut),
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

// ciscoIPv6NeighborMatches requires the address, interface and MAC in the same
// native neighbor record. The record can be wrapped, and XR may abbreviate
// FourHundredGigE as FH. Multicast adjacency rows must not be joined to it.
func ciscoIPv6NeighborMatches(t *testing.T, output, intf string, address netip.Addr, wantMAC net.HardwareAddr) (bool, error) {
	t.Helper()
	interfaceNames := map[string]bool{intf: true, intf + ".0": true}
	if strings.HasPrefix(intf, "FourHundredGigE") {
		shortName := "FH" + strings.TrimPrefix(intf, "FourHundredGigE")
		interfaceNames[shortName] = true
		interfaceNames[shortName+".0"] = true
	}
	var records [][]string
	pending := -1
	complete := func(fields []string) bool {
		hasMAC, hasInterface := false, false
		for _, field := range fields {
			if mac, err := net.ParseMAC(field); err == nil && len(mac) == 6 {
				hasMAC = true
			}
			if strings.Contains(field, "/") && !strings.Contains(field, ":") {
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
			// Only a partial neighbor record may consume a continuation line.
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

// verifiedCiscoDefaultPeer checks the exact port-1 peer for both address
// families. IOS XR may expose the IPv6 forwarding next hop as the peer's
// link-local address rather than its configured global address. We accept that
// equivalent only after native ND confirms its address, MAC, and interface.
func verifiedCiscoDefaultPeer(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, needIPv6 bool) func(string, []*aftcache.NextHop) (bool, error) {
	t.Helper()
	intf := dut.Port(t, "port1").Name()
	wantIPv4 := netip.MustParseAddr(atePort1.IPv4)
	wantIPv6 := netip.MustParseAddr(atePort1.IPv6)
	var linkLocal netip.Addr
	verified := false
	verificationDetail := "IPv6 neighbor was not checked"
	if needIPv6 {
		mac, err := net.ParseMAC(atePort1.MAC)
		if err != nil || len(mac) != 6 {
			t.Fatalf("ATE port1 must have a 48-bit MAC, got %q (%v)", atePort1.MAC, err)
		}
		address := [16]byte{0xfe, 0x80}
		copy(address[8:], []byte{mac[0] ^ 0x02, mac[1], mac[2], 0xff, 0xfe, mac[3], mac[4], mac[5]})
		linkLocal = netip.AddrFrom16(address)
		cliCtx, cancel := context.WithDeadline(ctx, time.Now().Add(20*time.Second))
		neighbors, err := dut.RawAPIs().CLI(t).RunCommand(cliCtx, "show ipv6 neighbors")
		cancel()
		if err != nil {
			verificationDetail = fmt.Sprintf("native ND lookup failed: %v", err)
		} else {
			verified, err = ciscoIPv6NeighborMatches(t, neighbors.Output(), intf, linkLocal, mac)
			verificationDetail = fmt.Sprintf("native ND verified=%t, detail=%v", verified, err)
		}
		t.Logf("Cisco IPv6 peer identity: interface=%s global=%s candidate link-local=%s expected MAC=%s %s", intf, wantIPv6, linkLocal, mac, verificationDetail)
	}
	return func(prefix string, nextHops []*aftcache.NextHop) (bool, error) {
		if len(nextHops) == 0 {
			return false, nil
		}
		ipv6 := strings.Contains(prefix, ":")
		for _, nextHop := range nextHops {
			if nextHop == nil || nextHop.IP == "" {
				return false, nil
			}
			ip, err := netip.ParseAddr(nextHop.IP)
			if err != nil {
				return false, fmt.Errorf("invalid next-hop address %q: %w", nextHop.IP, err)
			}
			ip = ip.WithZone("")
			correctInterface := nextHop.IntfName == "" || nextHop.IntfName == intf || nextHop.IntfName == intf+".0"
			if !correctInterface {
				return false, nil
			}
			if !ipv6 && ip != wantIPv4 {
				return false, nil
			}
			if ipv6 {
				if ip == linkLocal && !verified {
					return false, fmt.Errorf("cannot verify IPv6 next hop %s on %s for %s: %s", ip, intf, prefix, verificationDetail)
				}
				if ip != wantIPv6 && (ip != linkLocal || !verified || nextHop.IntfName == "") {
					return false, nil
				}
			}
		}
		return true, nil
	}
}

// awaitDynamicNotification checks only notifications received after this
// mutation's checkpoint. The short polling interval makes the 30-second
// notification deadline meaningful without changing the shared cache default.
func awaitDynamicNotification(ctx context.Context, t *testing.T, collector *aftcache.AFTStreamSession, since int, addPrefix, deletePrefix string) {
	t.Helper()
	t.Logf("Waiting for same-stream AFT update=%q delete=%q after notification index %d", addPrefix, deletePrefix, since)
	aftpf.RunCollector(t, aftpf.RunCollectorParams{
		Ctx:           ctx,
		Collector:     collector,
		Stop:          aftcache.WaitForNotification(t, aftcache.NotificationExpectation{AddPrefix: addPrefix, DeletePrefix: deletePrefix, NotificationWait: notificationWaitTime, SinceIndex: since}),
		Timeout:       notificationWaitTime,
		CheckInterval: notificationCheckInterval,
	})
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
	verifyDynamicPolicyMatch(t, dut, pArgs.policyName, pArgs.prefixSet)
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
	// The dynamic cases compare notification epochs on this same stream.
	// Keep the raw notifications even if the optional debug flag is disabled.
	collector.WithDebug()
	//----------------------------------------------------------------------
	// Initial Sync
	//----------------------------------------------------------------------
	t.Logf("%s - Initial Synchronization", pArgs.testID)
	initialStop := aftcache.InitialSyncStoppingCondition(t, dut, wantPrefixes,
		map[string]bool{atePort1.IPv4: true}, map[string]bool{atePort1.IPv6: true})
	if dut.Vendor() == ondatra.CISCO {
		initialStop = aftcache.InitialSyncStoppingConditionWithNextHopValidator(t, dut, wantPrefixes,
			verifiedCiscoDefaultPeer(ctx, t, dut, pArgs.isIPv6))
	}
	aftpf.RunCollector(t,
		aftpf.RunCollectorParams{
			Ctx:       ctx,
			Collector: collector,
			Stop:      initialStop,
			Timeout:   subscriptionWait,
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
	// The route must be hidden before changing the prefix set; otherwise an
	// unrelated notification could make the add stage appear to pass.
	aftpf.VerifyPrefixesAbsent(t, aftpf.PrefixesParams{InfoAFT: initialAFT, Prefixes: []string{pArgs.dynamicPrefix}})
	//----------------------------------------------------------------------
	// AFT-6.4.X.1 Add Prefix
	//----------------------------------------------------------------------
	t.Logf("%s.1 - Addition of Prefix to Active Set", pArgs.testID)
	beforeAdd := len(collector.Notifications())
	aftpf.AddPrefixToPrefixSet(t, dut,
		aftpf.AddPrefixToPrefixSetParams{
			PrefixSetName: pArgs.prefixSet,
			Prefix:        pArgs.dynamicPrefix,
			MaskRange:     pArgs.maskRange,
		})
	verifyDynamicPolicyMatch(t, dut, pArgs.policyName, pArgs.prefixSet)
	// Wait for ADD notification on the SAME stream.
	awaitDynamicNotification(ctx, t, collector, beforeAdd, pArgs.dynamicPrefix, "")
	//----------------------------------------------------------------------
	// AFT-6.4.X.2 Delete Prefix
	//----------------------------------------------------------------------
	t.Logf("%s.2 - Deletion of Prefix from Active Set", pArgs.testID)
	beforeDelete := len(collector.Notifications())
	aftpf.RemovePrefixFromPrefixSet(t, dut,
		aftpf.RemovePrefixFromPrefixSetParams{
			PrefixSetName: pArgs.prefixSet,
			Prefix:        pArgs.initialAllowedPrefixes[0],
			MaskRange:     pArgs.maskRange,
		},
	)
	verifyDynamicPolicyMatch(t, dut, pArgs.policyName, pArgs.prefixSet)
	// Wait for DELETE notification on SAME stream.
	awaitDynamicNotification(ctx, t, collector, beforeDelete, "", pArgs.initialAllowedPrefixes[0])
	verifyPrefixRemovedFromPrefixSet(t, dut, pArgs.prefixSet, pArgs.initialAllowedPrefixes[0], pArgs.maskRange)
	//----------------------------------------------------------------------
	// AFT-6.4.X.3 Atomic Add/Delete
	//----------------------------------------------------------------------
	t.Logf("%s.3 - Simultaneous Addition and Deletion", pArgs.testID)
	beforeSwap := len(collector.Notifications())
	atomicPrefixSetSwap(t, dut, pArgs.prefixSet, pArgs.initialAllowedPrefixes[0], pArgs.initialAllowedPrefixes[1], pArgs.maskRange)
	verifyDynamicPolicyMatch(t, dut, pArgs.policyName, pArgs.prefixSet)
	// Wait for BOTH notifications on SAME stream.
	awaitDynamicNotification(ctx, t, collector, beforeSwap, pArgs.initialAllowedPrefixes[0], pArgs.initialAllowedPrefixes[1])
	verifyPrefixRemovedFromPrefixSet(t, dut, pArgs.prefixSet, pArgs.initialAllowedPrefixes[1], pArgs.maskRange)
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
	set := gnmi.Get[*oc.RoutingPolicy_DefinedSets_PrefixSet](t, dut,
		gnmi.OC().RoutingPolicy().DefinedSets().PrefixSet(prefixSetName).Config())
	if entry := set.GetPrefix(prefix, maskRange); entry != nil {
		t.Fatalf("Prefix %s mask-range %s still exists in prefix-set %s: %+v", prefix, maskRange, prefixSetName, entry)
	}
	t.Logf("Verified prefix %s mask-range %s removed from prefix-set %s", prefix, maskRange, prefixSetName)
}

func verifyDynamicPolicyMatch(t *testing.T, dut *ondatra.DUTDevice, policyName, prefixSetName string) {
	t.Helper()
	policy := gnmi.Get[*oc.RoutingPolicy_PolicyDefinition](t, dut,
		gnmi.OC().RoutingPolicy().PolicyDefinition(policyName).Config())
	got := policy.GetStatement("10").GetConditions().GetMatchPrefixSet().GetPrefixSet()
	t.Logf("Dynamic AFT filter policy readback: policy=%s statement=10 match-prefix-set=%q, want %q", policyName, got, prefixSetName)
	if got != prefixSetName {
		t.Fatalf("Dynamic AFT filter policy %s does not match prefix-set %s (got %q)", policyName, prefixSetName, got)
	}
}

// atomicPrefixSetSwap atomically updates a prefix set.
// Replacing the complete set adds one prefix and removes another in one Set.
func atomicPrefixSetSwap(t *testing.T, dut *ondatra.DUTDevice, prefixName, addPrefixVal, delPrefixVal, prefixMode string) {
	t.Helper()
	if dut.Vendor() == ondatra.CISCO {
		// XR cannot change entries within the native prefix-set bag.
		aftpf.ReplacePrefixSetMembers(t, dut, prefixName,
			[]aftpf.PrefixSetMember{{Prefix: addPrefixVal, MaskRange: prefixMode}},
			[]aftpf.PrefixSetMember{{Prefix: delPrefixVal, MaskRange: prefixMode}})
		return
	}
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

// installRoutingPolicies installs the routing policies before BGP references them.
func installRoutingPolicies(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	root := &oc.Root{}
	rp := root.GetOrCreateRoutingPolicy()
	configureCommonPolicies(t, rp)
	configureDynamicUpdatePolicies(t, rp)
	policyBatch := &gnmi.SetBatch{}
	gnmi.BatchReplace(policyBatch, gnmi.OC().RoutingPolicy().Config(), rp)
	policyBatch.Set(t, dut)
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
			MatchPrefixSet: true,
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
			MatchPrefixSet: true,
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
