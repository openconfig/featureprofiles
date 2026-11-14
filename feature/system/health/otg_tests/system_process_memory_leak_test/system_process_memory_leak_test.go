// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package system_process_memory_leak_test implements Health-1.3:
// System Process and Controller Card Memory Leak Detection.
package system_process_memory_leak_test

import (
	"context"
	"fmt"
	"math"
	"net/netip"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/featureprofiles/internal/system"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	// Section 1: eBGP AS numbers, Router ID, and Routing Policy constants.
	dutAS             = uint32(64500)
	peer1AS           = uint32(64501)
	peer2AS           = uint32(64502)
	peer3AS           = uint32(64503)
	dutRouterID       = "192.0.2.1"
	ecmpMaxPaths      = uint32(3)
	routingPolicyName = "ALLOW-ALL"
	routingPolicyStmt = "allow-all-stmt"

	// Section 1: Route set counts and prefixes.
	baselineRouteCount = uint32(250)
	churnRouteCount    = uint32(10000)
	totalInstalledPeak = baselineRouteCount + churnRouteCount
	baselineIPv4Start  = "203.0.113.1"
	baselineIPv4Prefix = uint32(32)
	baselineIPv6Start  = "2001:db8:2:1::"
	baselineIPv6Prefix = uint32(64)
	churnIPv4Start     = "100.64.0.1"
	churnIPv4Prefix    = uint32(32)
	churnIPv6Start     = "2001:db8:10:1::"
	churnIPv6Prefix    = uint32(64)
	ipv4PrefixLen      = uint8(30)
	ipv6PrefixLen      = uint8(126)

	// Section 1: Data-plane validation traffic parameters (10,000 pps per family, 512B & 1500B frames, UDP src ports 10000-10999).
	trafficRatePPS      = uint64(10000)
	frameSize512        = uint32(512)
	frameSize1500       = uint32(1500)
	frameSizeWeight50   = float32(50)
	udpSrcPortStart     = uint32(10000)
	udpSrcPortCount     = uint32(1000)
	udpDstPort          = uint32(4789)
	flowBaselineIPv4    = "flow-baseline-ipv4"
	flowBaselineIPv6    = "flow-baseline-ipv6"
	flowChurnIPv4       = "flow-churn-ipv4"
	flowChurnIPv6       = "flow-churn-ipv6"
	ecmp3WayTargetPct   = 33.33
	ecmp2WayTargetPct   = 50.0
	ecmpSplitTolerance  = 5.0
	baselineTrafficHold = 15 * time.Second
	steadyTrafficHold   = 5 * time.Second

	// Section 3 & 4: Sampling intervals, churn cycles, and pass/fail thresholds.
	sampleInterval              = 60 * time.Second
	steadyStateSampleCount      = 15
	steadyStateWarmupSamples    = 5
	churnCycleCount             = 5
	churnHoldDuration           = 90 * time.Second
	regressionRSquaredThreshold = 0.85
	maxMemoryGrowthPct          = 5.0
	tenMBBytes                  = uint64(10 * 1024 * 1024)
	fifteenMBBytes              = uint64(15 * 1024 * 1024)
	fibUsedThresholdUpper       = uint8(80)
	fibUsedThresholdUpperClear  = uint8(70)
	fibEntryToleranceFloor      = uint64(250)
	bgpMaxPrefixLimit           = uint32(8000)
	bgpWarningThresholdPct      = uint8(80)
	bgpPrefixLimitRestartTime   = uint16(30)
	egressInterfaceFlapCycles   = 5
	abruptPeerResetCycles       = 3
	bgpConvergenceTimeout       = 3 * time.Minute
	aftConvergenceTimeout       = 2 * time.Minute
	interfaceOperStatusTimeout  = 1 * time.Minute
)

var (
	// Section 1: Physical Links and Interface Addressing (Link 1 - Link 4).
	dutPort1Attr = attrs.Attributes{
		Desc:    "DUT Port 1 connected to ATE Port 1",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::1",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
	atePort1Attr = attrs.Attributes{
		Name:    "atePort1",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::2",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
	dutPort2Attr = attrs.Attributes{
		Desc:    "DUT Port 2 connected to ATE Port 2",
		IPv4:    "192.0.2.5",
		IPv6:    "2001:db8::5",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
	atePort2Attr = attrs.Attributes{
		Name:    "atePort2",
		MAC:     "02:00:02:01:01:01",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::6",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
	dutPort3Attr = attrs.Attributes{
		Desc:    "DUT Port 3 connected to ATE Port 3",
		IPv4:    "192.0.2.9",
		IPv6:    "2001:db8::9",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
	atePort3Attr = attrs.Attributes{
		Name:    "atePort3",
		MAC:     "02:00:03:01:01:01",
		IPv4:    "192.0.2.10",
		IPv6:    "2001:db8::a",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
	dutPort4Attr = attrs.Attributes{
		Desc:    "DUT Port 4 connected to ATE Port 4",
		IPv4:    "192.0.2.13",
		IPv6:    "2001:db8::d",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
	atePort4Attr = attrs.Attributes{
		Name:    "atePort4",
		MAC:     "02:00:04:01:01:01",
		IPv4:    "192.0.2.14",
		IPv6:    "2001:db8::e",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}

	// Section 1: Sentinel prefixes for fast point-lookup AFT validation.
	baselineSentinelIPv4 = []string{"203.0.113.1/32", "203.0.113.250/32"}
	baselineSentinelIPv6 = []string{"2001:db8:2:1::/64", "2001:db8:2:fa::/64"}
	churnSentinelIPv4    = []string{"100.64.0.1/32", "100.64.39.16/32"}
	churnSentinelIPv6    = []string{"2001:db8:10:1::/64", "2001:db8:10:2710::/64"}
)

type bgpPeerSpec struct {
	name        string
	portID      string
	dutAttr     attrs.Attributes
	ateAttr     attrs.Attributes
	peerAS      uint32
	peerV4Name  string
	peerV6Name  string
	baseV4Name  string
	baseV6Name  string
	churnV4Name string
	churnV6Name string
}

type testEnvironment struct {
	dut                 *ondatra.DUTDevice
	ate                 *ondatra.ATEDevice
	otgConfig           gosnappi.Config
	peers               []bgpPeerSpec
	fibResourceName     string
	counterStream       *system.InterfaceCounterStream
	baselineProcesses   map[uint64]system.ProcessSample
	persistentProcesses map[uint64]system.ProcessSample
	baselineComponents  map[string]system.ComponentMemorySample
	postChurnProcesses  map[uint64]system.ProcessSample
	postChurnComponents map[string]system.ComponentMemorySample
	baselineFIBUtil     map[string]*cfgplugins.HardwareResourceUtilization
}

// TestSystemProcessMemoryLeak executes Health-1.3: System Process and Controller Card
// Memory Leak Detection across Positive Test Cases (Health-1.3.1 - Health-1.3.4),
// Negative Test Cases (Health-1.3.5 - Health-1.3.7), and Section 5 Cleanup.
func TestSystemProcessMemoryLeak(t *testing.T) {
	env := setupTestEnvironment(t)

	// Section 3: Positive Test Cases (Health-1.3.1 - Health-1.3.4).
	t.Run("Health-1.3.1_ValidateBaselineSystemProcessAndControllerCardMemoryTelemetry", func(t *testing.T) {
		runHealth131BaselineTelemetry(t, env)
	})

	t.Run("Health-1.3.2_ValidateSteadyStatePerProcessMemoryUsageOverTime", func(t *testing.T) {
		runHealth132SteadyStateSampling(t, env)
	})

	t.Run("Health-1.3.3_and_Health-1.3.4_ValidateMemoryReclamationAndHardwareFIBUtilizationUnderRouteChurn", func(t *testing.T) {
		runHealth133And134RouteChurnAndFIBUtilization(t, env)
	})

	// Section 4: Negative Test Cases (Health-1.3.5 - Health-1.3.7).
	t.Run("Health-1.3.5_BGPMaxPrefixLimitViolation", func(t *testing.T) {
		runHealth135BGPMaxPrefixLimitViolation(t, env)
	})

	t.Run("Health-1.3.6_RepeatedEgressInterfaceAdminDownFlappingUnderHighRouteScale", func(t *testing.T) {
		runHealth136RepeatedEgressInterfaceFlapping(t, env)
	})

	t.Run("Health-1.3.7_AbruptBGPPeerSessionResetUnderHighRouteScale", func(t *testing.T) {
		runHealth137AbruptBGPPeerSessionReset(t, env)
	})
}

// setupTestEnvironment implements Section 1 (Test environment setup) and registers
// Section 5 (Cleanup) via t.Cleanup().
func setupTestEnvironment(t *testing.T) *testEnvironment {
	t.Helper()
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	peers := []bgpPeerSpec{
		{
			name:        "Peer1",
			portID:      "port1",
			dutAttr:     dutPort1Attr,
			ateAttr:     atePort1Attr,
			peerAS:      peer1AS,
			peerV4Name:  "atePort1.BGP4.peer",
			peerV6Name:  "atePort1.BGP6.peer",
			baseV4Name:  "atePort1.baseline.v4",
			baseV6Name:  "atePort1.baseline.v6",
			churnV4Name: "atePort1.churn.v4",
			churnV6Name: "atePort1.churn.v6",
		},
		{
			name:        "Peer2",
			portID:      "port2",
			dutAttr:     dutPort2Attr,
			ateAttr:     atePort2Attr,
			peerAS:      peer2AS,
			peerV4Name:  "atePort2.BGP4.peer",
			peerV6Name:  "atePort2.BGP6.peer",
			baseV4Name:  "atePort2.baseline.v4",
			baseV6Name:  "atePort2.baseline.v6",
			churnV4Name: "atePort2.churn.v4",
			churnV6Name: "atePort2.churn.v6",
		},
		{
			name:        "Peer3",
			portID:      "port3",
			dutAttr:     dutPort3Attr,
			ateAttr:     atePort3Attr,
			peerAS:      peer3AS,
			peerV4Name:  "atePort3.BGP4.peer",
			peerV6Name:  "atePort3.BGP6.peer",
			baseV4Name:  "atePort3.baseline.v4",
			baseV6Name:  "atePort3.baseline.v6",
			churnV4Name: "atePort3.churn.v4",
			churnV6Name: "atePort3.churn.v6",
		},
	}

	env := &testEnvironment{
		dut:   dut,
		ate:   ate,
		peers: peers,
	}

	// Register Section 5 (Cleanup) before pushing configuration so state is always restored.
	t.Cleanup(func() {
		cleanupEnvironment(t, env)
	})

	// Section 1: Configure DUT interfaces, routing-policy, eBGP control plane, and FIB resource thresholds.
	configureDUTEnvironment(t, env)

	// Section 1: Configure ATE topology, eBGP peers, Baseline-Routes, Churn-Routes, and traffic flows.
	env.otgConfig = configureATEEnvironment(t, env)

	// Section 1: Initial Bring-Up — start ATE protocols, verify port1-port4 are UP, resolve ARP/ND,
	// withdraw Churn-Routes initially, and verify all six eBGP sessions are ESTABLISHED via gnmi.Watch on [OC-Group-C].
	ate.OTG().StartProtocols(t)
	awaitPortsUp(t, dut, []string{"port1", "port2", "port3", "port4"})
	env.counterStream = system.NewInterfaceCounterStream(t, dut, []string{
		dut.Port(t, "port1").Name(),
		dut.Port(t, "port2").Name(),
		dut.Port(t, "port3").Name(),
		dut.Port(t, "port4").Name(),
	}, 2*time.Second)
	otgutils.WaitForARP(t, ate.OTG(), env.otgConfig, "IPv4")
	otgutils.WaitForARP(t, ate.OTG(), env.otgConfig, "IPv6")

	setRouteState(t, ate, env.allChurnRouteNames(), gosnappi.StateProtocolRouteState.WITHDRAW)
	setRouteState(t, ate, env.allBaselineRouteNames(), gosnappi.StateProtocolRouteState.ADVERTISE)

	awaitAllBGPSessionsEstablished(t, env)

	return env
}

// configureDUTEnvironment pushes the Canonical OC configuration to the DUT using a unified
// gnmi.SetBatch (REPLACE) for interfaces, routing-policy, BGP global/neighbors, and system resource thresholds.
func configureDUTEnvironment(t *testing.T, env *testEnvironment) {
	t.Helper()
	dut := env.dut
	dc := gnmi.OC()

	fptest.ConfigureDefaultNetworkInstance(t, dut)
	dni := deviations.DefaultNetworkInstance(dut)

	sb := &gnmi.SetBatch{}

	portAttrs := []struct {
		portID string
		attr   attrs.Attributes
	}{
		{portID: "port1", attr: dutPort1Attr},
		{portID: "port2", attr: dutPort2Attr},
		{portID: "port3", attr: dutPort3Attr},
		{portID: "port4", attr: dutPort4Attr},
	}

	for _, pa := range portAttrs {
		p := dut.Port(t, pa.portID)
		gnmi.BatchUpdate(sb, dc.Interface(p.Name()).Config(), pa.attr.NewOCInterface(p.Name(), dut))
	}

	// Configure routing-policy ALLOW-ALL (statement allow-all-stmt -> ACCEPT_ROUTE).
	rpRoot := &oc.Root{}
	rp := rpRoot.GetOrCreateRoutingPolicy()
	pdef := rp.GetOrCreatePolicyDefinition(routingPolicyName)
	stmt, err := pdef.AppendNewStatement(routingPolicyStmt)
	if err != nil {
		t.Fatalf("Failed to create routing policy statement %s: %v", routingPolicyStmt, err)
	}
	stmt.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
	gnmi.BatchReplace(sb, dc.RoutingPolicy().Config(), rp)

	// Configure DUT BGP Global (AS 64500, router-id 192.0.2.1, eBGP maximum-paths = 3) and Peers 1-3
	// matching Canonical OC (direct neighbors with ALLOW-ALL import/export policy, plus peer-groups only
	// when MultipathUnsupportedNeighborOrAfisafi deviation is set).
	niProto := &oc.NetworkInstance_Protocol{
		Identifier: oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP,
		Name:       ygot.String("BGP"),
	}
	bgp := niProto.GetOrCreateBgp()
	globalOpts := []cfgplugins.GlobalOption{
		cfgplugins.WithAS(dutAS),
		cfgplugins.WithRouterID(dutRouterID),
		cfgplugins.WithGlobalAfiSafiEnabled(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST, true),
		cfgplugins.WithGlobalAfiSafiEnabled(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST, true),
	}
	if !deviations.BgpMaxMultipathPathsUnsupported(dut) {
		globalOpts = append(globalOpts, cfgplugins.WithGlobalEBGPMultipath(ecmpMaxPaths))
	}
	cfgplugins.ConfigureGlobal(bgp, dut, globalOpts...)

	for _, peer := range env.peers {
		nbrV4 := bgp.GetOrCreateNeighbor(peer.ateAttr.IPv4)
		nbrV4.SetNeighborAddress(peer.ateAttr.IPv4)
		nbrV4.SetPeerAs(peer.peerAS)
		nbrV4.SetEnabled(true)
		cfgplugins.ConfigurePeer(
			nbrV4,
			dut,
			cfgplugins.WithPeerAfiSafiEnabled(true, routingPolicyName, routingPolicyName, false),
			cfgplugins.ApplyPeerPerAfiSafiRoutingPolicy(true, routingPolicyName, routingPolicyName, false),
		)

		nbrV6 := bgp.GetOrCreateNeighbor(peer.ateAttr.IPv6)
		nbrV6.SetNeighborAddress(peer.ateAttr.IPv6)
		nbrV6.SetPeerAs(peer.peerAS)
		nbrV6.SetEnabled(true)
		cfgplugins.ConfigurePeer(
			nbrV6,
			dut,
			cfgplugins.WithPeerAfiSafiEnabled(false, routingPolicyName, routingPolicyName, false),
			cfgplugins.ApplyPeerPerAfiSafiRoutingPolicy(false, routingPolicyName, routingPolicyName, false),
		)

		if deviations.MultipathUnsupportedNeighborOrAfisafi(dut) {
			pgV4Name := "EBGP-PG-V4"
			pgV6Name := "EBGP-PG-V6"
			pgV4 := bgp.GetOrCreatePeerGroup(pgV4Name)
			pgV4.GetOrCreateUseMultiplePaths().SetEnabled(true)
			pgV4.GetOrCreateUseMultiplePaths().GetOrCreateEbgp().SetAllowMultipleAs(true)
			nbrV4.SetPeerGroup(pgV4Name)

			pgV6 := bgp.GetOrCreatePeerGroup(pgV6Name)
			pgV6.GetOrCreateUseMultiplePaths().SetEnabled(true)
			pgV6.GetOrCreateUseMultiplePaths().GetOrCreateEbgp().SetAllowMultipleAs(true)
			nbrV6.SetPeerGroup(pgV6Name)
		}
	}

	bgpPath := dc.NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	gnmi.BatchReplace(sb, bgpPath.Config(), niProto)

	sb.Set(t, dut)

	// Configure [OC-Group-D] system resource utilization thresholds (FIB placeholder mapped to platform FIB resource).
	env.fibResourceName = cfgplugins.DiscoverFIBResourceName(t, dut)
	cfgplugins.ConfigureResourceUtilizationThreshold(t, dut, sb, cfgplugins.ResourceUtilizationCfg{
		ResourceName:            env.fibResourceName,
		UsedThresholdUpper:      fibUsedThresholdUpper,
		UsedThresholdUpperClear: fibUsedThresholdUpperClear,
	})

	for _, pa := range portAttrs {
		p := dut.Port(t, pa.portID)
		if deviations.ExplicitPortSpeed(dut) {
			fptest.SetPortSpeed(t, p)
		}
		if deviations.ExplicitInterfaceInDefaultVRF(dut) {
			fptest.AssignToNetworkInstance(t, dut, p.Name(), dni, 0)
		}
	}

	if deviations.BGPMissingOCMaxPrefixesConfiguration(dut) {
		for _, peer := range env.peers {
			cfgplugins.DeviationAristaBGPNeighborMaxPrefixes(t, dut, peer.ateAttr.IPv4, 0)
			cfgplugins.DeviationAristaBGPNeighborMaxPrefixes(t, dut, peer.ateAttr.IPv6, 0)
		}
	}
}

// configureATEEnvironment builds and pushes the 4-port OTG topology, eBGP peers, route sets, and traffic flows.
func configureATEEnvironment(t *testing.T, env *testEnvironment) gosnappi.Config {
	t.Helper()
	topo := gosnappi.NewConfig()

	var baseV4RxNames, baseV6RxNames []string
	var churnV4RxNames, churnV6RxNames []string

	for _, peer := range env.peers {
		atePort := env.ate.Port(t, peer.portID)
		dev := peer.ateAttr.AddToOTG(topo, atePort, &peer.dutAttr)

		ipv4Intf := dev.Ethernets().Items()[0].Ipv4Addresses().Items()[0]
		ipv6Intf := dev.Ethernets().Items()[0].Ipv6Addresses().Items()[0]

		bgpRouter := dev.Bgp().SetRouterId(peer.ateAttr.IPv4)
		bgp4Peer := bgpRouter.Ipv4Interfaces().Add().SetIpv4Name(ipv4Intf.Name()).Peers().Add().SetName(peer.peerV4Name)
		bgp4Peer.SetPeerAddress(ipv4Intf.Gateway()).SetAsNumber(peer.peerAS).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
		bgp4Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true)

		bgp6Peer := bgpRouter.Ipv6Interfaces().Add().SetIpv6Name(ipv6Intf.Name()).Peers().Add().SetName(peer.peerV6Name)
		bgp6Peer.SetPeerAddress(ipv6Intf.Gateway()).SetAsNumber(peer.peerAS).SetAsType(gosnappi.BgpV6PeerAsType.EBGP)
		bgp6Peer.LearnedInformationFilter().SetUnicastIpv6Prefix(true)

		// Baseline-Routes (250 IPv4 /32 + 250 IPv6 /64).
		baseV4 := bgp4Peer.V4Routes().Add().SetName(peer.baseV4Name)
		baseV4.SetNextHopIpv4Address(peer.ateAttr.IPv4).
			SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).
			SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)
		baseV4.Addresses().Add().SetAddress(baselineIPv4Start).SetPrefix(baselineIPv4Prefix).SetCount(baselineRouteCount)

		baseV6 := bgp6Peer.V6Routes().Add().SetName(peer.baseV6Name)
		baseV6.SetNextHopIpv6Address(peer.ateAttr.IPv6).
			SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).
			SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL)
		baseV6.Addresses().Add().SetAddress(baselineIPv6Start).SetPrefix(baselineIPv6Prefix).SetCount(baselineRouteCount)

		// Churn-Routes (10,000 IPv4 /32 + 10,000 IPv6 /64).
		churnV4 := bgp4Peer.V4Routes().Add().SetName(peer.churnV4Name)
		churnV4.SetNextHopIpv4Address(peer.ateAttr.IPv4).
			SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).
			SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)
		churnV4.Addresses().Add().SetAddress(churnIPv4Start).SetPrefix(churnIPv4Prefix).SetCount(churnRouteCount)

		churnV6 := bgp6Peer.V6Routes().Add().SetName(peer.churnV6Name)
		churnV6.SetNextHopIpv6Address(peer.ateAttr.IPv6).
			SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).
			SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL)
		churnV6.Addresses().Add().SetAddress(churnIPv6Start).SetPrefix(churnIPv6Prefix).SetCount(churnRouteCount)

		baseV4RxNames = append(baseV4RxNames, peer.baseV4Name)
		baseV6RxNames = append(baseV6RxNames, peer.baseV6Name)
		churnV4RxNames = append(churnV4RxNames, peer.churnV4Name)
		churnV6RxNames = append(churnV6RxNames, peer.churnV6Name)
	}

	// Link 4 (atePort4): Ingress Validation Port.
	atePort4 := env.ate.Port(t, "port4")
	dev4 := atePort4Attr.AddToOTG(topo, atePort4, &dutPort4Attr)
	srcIPv4Name := dev4.Ethernets().Items()[0].Ipv4Addresses().Items()[0].Name()
	srcIPv6Name := dev4.Ethernets().Items()[0].Ipv6Addresses().Items()[0].Name()

	// Configure dual-stack validation flows from atePort4 (10,000 pps per family, both 512 and 1500 byte frames, UDP src ports 10000-10999).
	addValidationFlow(topo, flowBaselineIPv4, srcIPv4Name, baseV4RxNames, atePort4Attr.MAC, atePort4Attr.IPv4, baselineIPv4Start, baselineRouteCount, true)
	addValidationFlow(topo, flowBaselineIPv6, srcIPv6Name, baseV6RxNames, atePort4Attr.MAC, atePort4Attr.IPv6, baselineIPv6Start, baselineRouteCount, false)
	addValidationFlow(topo, flowChurnIPv4, srcIPv4Name, churnV4RxNames, atePort4Attr.MAC, atePort4Attr.IPv4, churnIPv4Start, churnRouteCount, true)
	addValidationFlow(topo, flowChurnIPv6, srcIPv6Name, churnV6RxNames, atePort4Attr.MAC, atePort4Attr.IPv6, churnIPv6Start, churnRouteCount, false)

	env.ate.OTG().PushConfig(t, topo)
	return topo
}

func addValidationFlow(topo gosnappi.Config, name, txName string, rxNames []string, srcMAC, srcIP, dstStart string, dstCount uint32, isIPv4 bool) {
	flow := topo.Flows().Add().SetName(name)
	flow.Metrics().SetEnable(true)
	flow.TxRx().Device().SetTxNames([]string{txName}).SetRxNames(rxNames)
	customSizes := flow.Size().WeightPairs().Custom()
	customSizes.Add().SetSize(frameSize512).SetWeight(frameSizeWeight50)
	customSizes.Add().SetSize(frameSize1500).SetWeight(frameSizeWeight50)
	flow.Rate().SetPps(trafficRatePPS)

	eth := flow.Packet().Add().Ethernet()
	eth.Src().SetValue(srcMAC)
	if isIPv4 {
		ip4 := flow.Packet().Add().Ipv4()
		ip4.Src().SetValue(srcIP)
		ip4.Dst().Increment().SetStart(dstStart).SetCount(dstCount)
	} else {
		ip6 := flow.Packet().Add().Ipv6()
		ip6.Src().SetValue(srcIP)
		ip6.Dst().Increment().SetStart(dstStart).SetStep("0:0:0:1::").SetCount(dstCount)
	}
	udp := flow.Packet().Add().Udp()
	udp.SrcPort().Increment().SetStart(udpSrcPortStart).SetStep(1).SetCount(udpSrcPortCount)
	udp.DstPort().SetValue(udpDstPort)
}

// runHealth131BaselineTelemetry implements Health-1.3.1:
// Validate Baseline System Process and Controller Card Memory Telemetry.
func runHealth131BaselineTelemetry(t *testing.T, env *testEnvironment) {
	// Health-1.3.1 Step 1: Advertise Baseline-Routes from Peer 1, Peer 2, and Peer 3, and verify
	// via gnmi.Watch on [OC-Group-C] that prefixes/received and prefixes/installed equal 250 per
	// family on all three peers and that Baseline-Sentinel-Prefixes resolve to a 3-way ECMP
	// next-hop-group in the AFT.
	t.Log("Health-1.3.1 Step 1: Verifying Baseline-Routes (250 IPv4 + 250 IPv6) on Peer 1-3 and Baseline-Sentinel-Prefixes in AFT")
	awaitAllPeersPrefixCounts(t, env, env.peers, baselineRouteCount)
	verifySentinelPrefixesInAFT(t, env.dut, baselineSentinelIPv4, baselineSentinelIPv6, true, int(ecmpMaxPaths))

	// Health-1.3.1 Step 2: Query [OC-Group-A] in a single wildcard gnmi.GetAll call on
	// /system/processes/process/state. Filter for user-space processes where memory-usage > 0.
	t.Log("Health-1.3.1 Step 2: Querying [OC-Group-A] (/system/processes/process/state) in a single wildcard gnmi.GetAll call")
	env.baselineProcesses = system.CollectUserSpaceProcesses(t, env.dut)
	if len(env.baselineProcesses) == 0 {
		t.Fatalf("Health-1.3.1 Step 5 Fail: No user-space processes with memory-usage > 0 returned from [OC-Group-A]")
	}
	for pid, proc := range env.baselineProcesses {
		if pid == 0 || proc.Name == "" {
			t.Errorf("Health-1.3.1 Step 5 Fail: Process in [OC-Group-A] has invalid identity: pid=%d, name=%q", pid, proc.Name)
		}
		t.Logf("Health-1.3.1 Baseline Process [OC-Group-A]: pid=%d name=%s memUsage=%dB memUtil=%d%% cpuUtil=%d%% startTime=%d (present=%t)",
			proc.PID, proc.Name, proc.MemoryUsage, proc.MemoryUtilization, proc.CPUUtilization, proc.StartTime, proc.StartTimePresent)
	}

	// Health-1.3.1 Step 3: Query [OC-Group-B] via gNMI.Get to record baseline memory/available
	// and memory/utilized for CONTROLLER_CARD, CHASSIS, and CPU components.
	t.Log("Health-1.3.1 Step 3: Querying [OC-Group-B] (/components/component/state/memory) for control-plane components")
	env.baselineComponents = system.CollectControlPlaneMemory(t, env.dut)
	var hasControlPlaneMemory bool
	for name, comp := range env.baselineComponents {
		t.Logf("Health-1.3.1 Baseline Component [OC-Group-B]: name=%s type=%v operStatus=%v available=%dB utilized=%dB",
			name, comp.Type, comp.OperStatus, comp.Available, comp.Utilized)
		if comp.Type == oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD ||
			comp.Type == oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CHASSIS {
			if comp.OperStatus != oc.PlatformTypes_COMPONENT_OPER_STATUS_UNSET &&
				comp.OperStatus != oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE {
				t.Errorf("Health-1.3.1 Step 5 Fail: Component %s (%v) in [OC-Group-B] reported oper-status=%v, want ACTIVE",
					name, comp.Type, comp.OperStatus)
			}
			if comp.Available > 0 && comp.Utilized > 0 {
				hasControlPlaneMemory = true
			}
		}
	}
	if !hasControlPlaneMemory {
		t.Errorf("Health-1.3.1 Step 5 Fail: Expected at least one CONTROLLER_CARD or CHASSIS component in [OC-Group-B] with non-zero memory/available and memory/utilized")
	}

	// Health-1.3.1 Step 4 & Step 5: Send data-plane validation traffic from atePort4 to Baseline-Routes
	// for 15 seconds and verify 0% loss and 3-way ECMP balance (33.3% +- 5%) across port1-port3.
	t.Log("Health-1.3.1 Step 4: Sending data-plane validation traffic to Baseline-Routes for 15 seconds")
	verifyTrafficAndECMPSplit(
		t,
		env,
		baselineTrafficHold,
		[]string{flowBaselineIPv4, flowBaselineIPv6},
		nil,
		[]string{"port1", "port2", "port3"},
		ecmp3WayTargetPct,
	)
}

// runHealth132SteadyStateSampling implements Health-1.3.2:
// Validate Steady-State Per-Process Memory Usage Over Time.
func runHealth132SteadyStateSampling(t *testing.T, env *testEnvironment) {
	// Health-1.3.2 Step 1: Collect [OC-Group-A] and [OC-Group-B] every 60 seconds for 15 minutes
	// (15 samples) using a single wildcard gNMI.Subscribe (SAMPLE mode at 60s) and wildcard gnmi.GetAll.
	t.Logf("Health-1.3.2 Step 1: Starting %d-sample steady-state memory collection at %v intervals via gNMI SAMPLE stream", steadyStateSampleCount, sampleInterval)
	procStream := system.NewProcessSampleStream(t, env.dut, sampleInterval)
	defer procStream.Close()

	procSamples := make([]map[uint64]system.ProcessSample, 0, steadyStateSampleCount)
	compSamples := make([]map[string]system.ComponentMemorySample, 0, steadyStateSampleCount)

	for i := 0; i < steadyStateSampleCount; i++ {
		pSnap := procStream.AwaitNextSample(t, env.dut)
		cSnap := system.CollectControlPlaneMemory(t, env.dut)
		procSamples = append(procSamples, pSnap)
		compSamples = append(compSamples, cSnap)
		t.Logf("Health-1.3.2 Step 1: Collected steady-state sample %d/%d (%d user-space processes, %d control-plane components)",
			i+1, steadyStateSampleCount, len(pSnap), len(cSnap))
	}

	// Health-1.3.2 Step 2: Identify persistent user-space daemons present across both the baseline
	// (Health-1.3.1) and warm-up (samples 1-5) window, ignoring short-lived OS/cron/helper processes.
	persistent := make(map[uint64]system.ProcessSample)
	for pid, baseProc := range env.baselineProcesses {
		presentInAllWarmup := true
		for i := 0; i < steadyStateWarmupSamples; i++ {
			wProc, ok := procSamples[i][pid]
			if !ok || wProc.Name != baseProc.Name {
				presentInAllWarmup = false
				break
			}
			if baseProc.StartTimePresent && wProc.StartTimePresent && wProc.StartTime != baseProc.StartTime {
				presentInAllWarmup = false
				break
			}
		}
		if presentInAllWarmup {
			persistent[pid] = baseProc
		}
	}
	env.persistentProcesses = persistent
	t.Logf("Health-1.3.2 Step 2: Identified %d persistent user-space daemons across baseline and warm-up samples 1-%d",
		len(persistent), steadyStateWarmupSamples)

	if len(persistent) == 0 {
		t.Fatalf("Health-1.3.2 Step 3 Fail: No persistent user-space daemons remained across the warm-up window")
	}

	// Health-1.3.2 Step 3: Evaluate steady-state window (samples 6-15, indices 5..14).
	evalStartIdx := steadyStateWarmupSamples
	for pid, baseProc := range persistent {
		series := make([]float64, 0, steadyStateSampleCount-evalStartIdx)
		var missing bool
		for i := evalStartIdx; i < steadyStateSampleCount; i++ {
			sProc, ok := procSamples[i][pid]
			if !ok || sProc.Name != baseProc.Name {
				var baselineInstanceStillRunning bool
				for basePID, bProc := range env.baselineProcesses {
					if bProc.Name == baseProc.Name {
						if cur, exists := procSamples[i][basePID]; exists && cur.Name == baseProc.Name {
							baselineInstanceStillRunning = true
							break
						}
					}
				}
				if !baselineInstanceStillRunning {
					var respawnedPID uint64
					for candPID, candProc := range procSamples[i] {
						if candPID != pid && candProc.Name == baseProc.Name {
							if _, inBase := env.baselineProcesses[candPID]; !inBase {
								respawnedPID = candPID
								break
							}
						}
					}
					if respawnedPID != 0 {
						t.Errorf("Health-1.3.2 Step 3 Fail: Persistent daemon pid=%d name=%q restarted under new pid=%d at sample %d", pid, baseProc.Name, respawnedPID, i+1)
					}
				}
				delete(persistent, pid)
				missing = true
				break
			}
			if baseProc.StartTimePresent && sProc.StartTimePresent && sProc.StartTime != baseProc.StartTime {
				t.Errorf("Health-1.3.2 Step 3 Fail: Persistent daemon pid=%d name=%q restarted (start-time changed from %d to %d at sample %d)",
					pid, baseProc.Name, baseProc.StartTime, sProc.StartTime, i+1)
				missing = true
				break
			}
			series = append(series, float64(sProc.MemoryUsage))
		}
		if missing || len(series) < 2 {
			continue
		}
		slope, rSquared := system.LinearRegression(series)
		startMem := uint64(series[0])
		endMem := uint64(series[len(series)-1])
		var growthBytes uint64
		var growthPct float64
		if endMem > startMem && startMem > 0 {
			growthBytes = endMem - startMem
			growthPct = float64(growthBytes) * 100.0 / float64(startMem)
		}
		if slope > 0 && rSquared >= regressionRSquaredThreshold && growthPct > maxMemoryGrowthPct && growthBytes > tenMBBytes {
			t.Errorf("Health-1.3.2 Step 3 Fail: Persistent daemon pid=%d name=%q leaked memory in steady state: slope=%.2f B/sample, R^2=%.3f, growth=%.2f%% (%d bytes) from sample 6 to 15",
				pid, baseProc.Name, slope, rSquared, growthPct, growthBytes)
		}
	}

	for compName, baseComp := range env.baselineComponents {
		if baseComp.Type != oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD &&
			baseComp.Type != oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CHASSIS {
			continue
		}
		series := make([]float64, 0, steadyStateSampleCount-evalStartIdx)
		for i := evalStartIdx; i < steadyStateSampleCount; i++ {
			if cSnap, ok := compSamples[i][compName]; ok && cSnap.Utilized > 0 {
				series = append(series, float64(cSnap.Utilized))
			}
		}
		if len(series) < 2 {
			continue
		}
		slope, rSquared := system.LinearRegression(series)
		startUtil := series[0]
		endUtil := series[len(series)-1]
		var growthPct float64
		if endUtil > startUtil && startUtil > 0 {
			growthPct = (endUtil - startUtil) * 100.0 / startUtil
		}
		if slope > 0 && rSquared >= regressionRSquaredThreshold && growthPct > maxMemoryGrowthPct {
			t.Errorf("Health-1.3.2 Step 3 Fail: Control-plane component %s leaked memory in steady state: slope=%.2f B/sample, R^2=%.3f, growth=%.2f%% from sample 6 to 15",
				compName, slope, rSquared, growthPct)
		}
	}
}

// runHealth133And134RouteChurnAndFIBUtilization implements Health-1.3.3 (Validate Process Memory
// Reclamation Under Scaled Dual-Stack eBGP Route Churn) and Health-1.3.4 (Validate Hardware
// Resource Utilization Telemetry Under Route Scale).
func runHealth133And134RouteChurnAndFIBUtilization(t *testing.T, env *testEnvironment) {
	initialTransitions := snapshotBGPTransitions(t, env)

	// Health-1.3.4 Step 1: Verify hardware resource utilization thresholds configured in Canonical OC
	// are reflected in /system/utilization/resources/resource/state ([OC-Group-D]).
	t.Run("Health-1.3.4_Step1_VerifySystemResourceThresholds", func(t *testing.T) {
		if resState, ok := cfgplugins.LookupResourceUtilizationThresholdState(t, env.dut, env.fibResourceName); ok && resState != nil {
			if got := resState.GetName(); got != env.fibResourceName {
				t.Errorf("Health-1.3.4 Step 1: System resource state/name got %q, want %q", got, env.fibResourceName)
			}
			gotUpper := resState.GetUsedThresholdUpper()
			gotClear := resState.GetUsedThresholdUpperClear()
			if gotUpper == 0 && gotClear == 0 {
				t.Logf("Health-1.3.4 Step 1: System resource %q state threshold leaves report 0 on this platform; thresholds are reported under INTEGRATED_CIRCUIT utilization state", env.fibResourceName)
			} else {
				if gotUpper != fibUsedThresholdUpper {
					t.Errorf("Health-1.3.4 Step 1: System resource state/used-threshold-upper got %d, want %d", gotUpper, fibUsedThresholdUpper)
				}
				if gotClear != fibUsedThresholdUpperClear {
					t.Errorf("Health-1.3.4 Step 1: System resource state/used-threshold-upper-clear got %d, want %d", gotClear, fibUsedThresholdUpperClear)
				}
			}
		} else {
			t.Logf("Health-1.3.4 Step 1: System resource state path for %q not populated on this platform; proceeding to INTEGRATED_CIRCUIT utilization checks", env.fibResourceName)
		}
	})

	// Health-1.3.4 Step 2 (Baseline): Query INTEGRATED_CIRCUIT (or Chassis aggregate) utilization
	// state leaves in [OC-Group-D] before Churn-Routes advertisement (baseline_used).
	env.baselineFIBUtil = cfgplugins.QueryHardwareFIBUtilization(t, env.dut, env.fibResourceName)
	for icName, u := range env.baselineFIBUtil {
		t.Logf("Health-1.3.4 Step 2 Baseline FIB [OC-Group-D] component=%s name=%s used=%d free=%d maxLimit=%d committed=%d(present=%t) hwm=%d(present=%t) upper=%d clear=%d upperExceeded=%t",
			icName, u.Name, u.Used, u.Free, u.MaxLimit, u.Committed, u.CommittedPresent, u.HighWatermark, u.HighWatermarkPresent, u.UsedThresholdUpper, u.UsedThresholdUpperClear, u.UsedThresholdUpperExceeded)
	}

	// Start 60s background wildcard gNMI SAMPLE subscription for [OC-Group-A] during the 15-minute churn test.
	procStream := system.NewProcessSampleStream(t, env.dut, sampleInterval)
	defer procStream.Close()

	postWithdrawProcSnapshots := make([]map[uint64]system.ProcessSample, 0, churnCycleCount)
	postWithdrawCompSnapshots := make([]map[string]system.ComponentMemorySample, 0, churnCycleCount)

	for cycle := 1; cycle <= churnCycleCount; cycle++ {
		t.Logf("Health-1.3.3 Cycle %d/%d Step 1: Advertising Churn-Routes (10,000 IPv4 + 10,000 IPv6) on Peer 1-3", cycle, churnCycleCount)
		advStart := time.Now()
		setRouteState(t, env.ate, env.allChurnRouteNames(), gosnappi.StateProtocolRouteState.ADVERTISE)

		// Health-1.3.3 Step 1: Confirm prefixes/installed reaches 10,250 per family on all three peers
		// and Churn-Sentinel-Prefixes are present in the AFT, and verify 0% traffic loss to Churn-Routes.
		awaitAllPeersPrefixCounts(t, env, env.peers, totalInstalledPeak)
		verifySentinelPrefixesInAFT(t, env.dut, churnSentinelIPv4, churnSentinelIPv6, true, int(ecmpMaxPaths))
		verifyTrafficAndECMPSplit(
			t,
			env,
			steadyTrafficHold,
			[]string{flowChurnIPv4, flowChurnIPv6},
			nil,
			[]string{"port1", "port2", "port3"},
			ecmp3WayTargetPct,
		)

		// Health-1.3.4 Step 2 & Step 3 (Peak): Check hardware FIB utilization at peak 20,000-route installation
		// in Cycle 1 and Cycle 5 using gnmi.Watch via AwaitHardwareFIBUtilization.
		if cycle == 1 || cycle == churnCycleCount {
			verifyPeakFIBUtilization(t, env, cycle)
		}

		// Health-1.3.3 Step 2: Hold Churn-Routes advertised for 90 seconds total while sampling via SAMPLE stream.
		if rem := churnHoldDuration - time.Since(advStart); rem > 0 {
			procStream.AwaitHoldWindow(t, env.dut, rem)
		}

		// Health-1.3.3 Step 2 & Step 3: Withdraw Churn-Routes from all three peers and hold withdrawn for 90 seconds.
		t.Logf("Health-1.3.3 Cycle %d/%d Step 2-3: Withdrawing Churn-Routes on Peer 1-3 and verifying AFT removal", cycle, churnCycleCount)
		withdrawStart := time.Now()
		setRouteState(t, env.ate, env.allChurnRouteNames(), gosnappi.StateProtocolRouteState.WITHDRAW)

		awaitAllPeersPrefixCounts(t, env, env.peers, baselineRouteCount)
		verifySentinelPrefixesInAFT(t, env.dut, churnSentinelIPv4, churnSentinelIPv6, false, 0)

		// Health-1.3.3 Step 3: Send traffic for 5 seconds and verify 100% packet loss on withdrawn
		// Churn-Routes while traffic to Baseline-Routes continues with 0% loss.
		verifyTrafficAndECMPSplit(
			t,
			env,
			steadyTrafficHold,
			[]string{flowBaselineIPv4, flowBaselineIPv6},
			[]string{flowChurnIPv4, flowChurnIPv6},
			[]string{"port1", "port2", "port3"},
			ecmp3WayTargetPct,
		)

		if rem := churnHoldDuration - time.Since(withdrawStart); rem > 0 {
			procStream.AwaitHoldWindow(t, env.dut, rem)
		}

		// Health-1.3.4 Step 2 & Step 3 (Post-Withdrawal): Verify hardware FIB used returns within
		// max(+-1% of baseline_used, +-250 entries) after withdrawal in Cycle 1 and Cycle 5.
		if cycle == 1 || cycle == churnCycleCount {
			verifyPostWithdrawFIBUtilization(t, env, cycle)
		}

		// Health-1.3.3 Step 4: Record post-withdrawal [OC-Group-A] and [OC-Group-B] memory at end of cycle.
		pSnap := procStream.Snapshot(t, env.dut)
		cSnap := system.CollectControlPlaneMemory(t, env.dut)
		postWithdrawProcSnapshots = append(postWithdrawProcSnapshots, pSnap)
		postWithdrawCompSnapshots = append(postWithdrawCompSnapshots, cSnap)
	}

	// Health-1.3.3 Step 5: Verify all six eBGP sessions remained ESTABLISHED with constant established-transitions.
	verifyConstantBGPTransitions(t, env, initialTransitions)

	// Health-1.3.3 Step 5: Using Cycle 1 post-withdrawal memory as the arena-warmed baseline, evaluate
	// Cycles 2-5 (indices 1..4) for persistent daemons in [OC-Group-A] and control-plane components in [OC-Group-B].
	cycle1Procs := postWithdrawProcSnapshots[0]
	cycle1Comps := postWithdrawCompSnapshots[0]
	env.postChurnProcesses = postWithdrawProcSnapshots[len(postWithdrawProcSnapshots)-1]
	env.postChurnComponents = postWithdrawCompSnapshots[len(postWithdrawCompSnapshots)-1]

	trackedProcs := env.persistentProcesses
	if len(trackedProcs) == 0 {
		trackedProcs = env.baselineProcesses
	}

	for pid, baseProc := range trackedProcs {
		c1Sample, ok := cycle1Procs[pid]
		if !ok || c1Sample.Name != baseProc.Name {
			var baselineInstanceStillRunning bool
			for basePID, bProc := range env.baselineProcesses {
				if bProc.Name == baseProc.Name {
					if cur, exists := cycle1Procs[basePID]; exists && cur.Name == baseProc.Name {
						baselineInstanceStillRunning = true
						break
					}
				}
			}
			if !baselineInstanceStillRunning {
				var respawnedPID uint64
				for candPID, candProc := range cycle1Procs {
					if candPID != pid && candProc.Name == baseProc.Name {
						if _, inBase := env.baselineProcesses[candPID]; !inBase {
							respawnedPID = candPID
							break
						}
					}
				}
				if respawnedPID != 0 {
					t.Errorf("Health-1.3.3 Step 5 Fail: Persistent daemon pid=%d name=%q restarted under new pid=%d by Cycle 1", pid, baseProc.Name, respawnedPID)
				}
			}
			delete(trackedProcs, pid)
			continue
		}
		series := make([]float64, 0, churnCycleCount-1)
		var missing bool
		for idx := 1; idx < churnCycleCount; idx++ {
			cSample, present := postWithdrawProcSnapshots[idx][pid]
			if !present || cSample.Name != baseProc.Name {
				var baselineInstanceStillRunning bool
				for basePID, bProc := range env.baselineProcesses {
					if bProc.Name == baseProc.Name {
						if cur, exists := postWithdrawProcSnapshots[idx][basePID]; exists && cur.Name == baseProc.Name {
							baselineInstanceStillRunning = true
							break
						}
					}
				}
				if !baselineInstanceStillRunning {
					var respawnedPID uint64
					for candPID, candProc := range postWithdrawProcSnapshots[idx] {
						if candPID != pid && candProc.Name == baseProc.Name {
							if _, inBase := env.baselineProcesses[candPID]; !inBase {
								respawnedPID = candPID
								break
							}
						}
					}
					if respawnedPID != 0 {
						t.Errorf("Health-1.3.3 Step 5 Fail: Persistent daemon pid=%d name=%q restarted under new pid=%d in Cycle %d", pid, baseProc.Name, respawnedPID, idx+1)
					}
				}
				delete(trackedProcs, pid)
				missing = true
				break
			}
			series = append(series, float64(cSample.MemoryUsage))
		}
		if missing || len(series) < 2 {
			continue
		}
		slope, rSquared := system.LinearRegression(series)
		c1Mem := c1Sample.MemoryUsage
		c5Mem := uint64(series[len(series)-1])
		var growthBytes uint64
		var growthPct float64
		if c5Mem > c1Mem && c1Mem > 0 {
			growthBytes = c5Mem - c1Mem
			growthPct = float64(growthBytes) * 100.0 / float64(c1Mem)
		}
		if slope > 0 && rSquared >= regressionRSquaredThreshold && growthPct > maxMemoryGrowthPct && growthBytes > fifteenMBBytes {
			t.Errorf("Health-1.3.3 Step 5 Fail: Persistent daemon pid=%d name=%q failed to reclaim memory across Cycles 2-5: slope=%.2f B/cycle, R^2=%.3f, unrecovered growth=%.2f%% (%d bytes) above Cycle 1 baseline",
				pid, baseProc.Name, slope, rSquared, growthPct, growthBytes)
		}
	}

	for compName, c1Comp := range cycle1Comps {
		if c1Comp.Type != oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD &&
			c1Comp.Type != oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CHASSIS {
			continue
		}
		series := make([]float64, 0, churnCycleCount-1)
		for idx := 1; idx < churnCycleCount; idx++ {
			if cSnap, ok := postWithdrawCompSnapshots[idx][compName]; ok && cSnap.Utilized > 0 {
				series = append(series, float64(cSnap.Utilized))
			}
		}
		if len(series) < 2 {
			continue
		}
		slope, rSquared := system.LinearRegression(series)
		c1Util := float64(c1Comp.Utilized)
		c5Util := series[len(series)-1]
		var growthPct float64
		if c5Util > c1Util && c1Util > 0 {
			growthPct = (c5Util - c1Util) * 100.0 / c1Util
		}
		if slope > 0 && rSquared >= regressionRSquaredThreshold && growthPct > maxMemoryGrowthPct {
			t.Errorf("Health-1.3.3 Step 5 Fail: Control-plane component %s failed to reclaim memory across Cycles 2-5: slope=%.2f B/cycle, R^2=%.3f, unrecovered growth=%.2f%% above Cycle 1 baseline",
				compName, slope, rSquared, growthPct)
		}
	}
}

// runHealth135BGPMaxPrefixLimitViolation implements Health-1.3.5:
// BGP Max-Prefix Limit Violation (Negative).
func runHealth135BGPMaxPrefixLimitViolation(t *testing.T, env *testEnvironment) {
	peer1 := env.peers[0]
	survivingPeers := env.peers[1:]
	dni := deviations.DefaultNetworkInstance(env.dut)
	bgpPath := gnmi.OC().NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()

	// Ensure Churn-Routes are advertised on Peer 1-3 first so we capture the post-churn baseline
	// at matching 10,250-route scale before withdrawing Peer 1's Churn-Routes to apply max-prefixes=8000.
	setRouteState(t, env.ate, env.allChurnRouteNames(), gosnappi.StateProtocolRouteState.ADVERTISE)
	awaitAllPeersPrefixCounts(t, env, env.peers, totalInstalledPeak)
	baselineProcs := system.CollectUserSpaceProcesses(t, env.dut)

	setRouteState(t, env.ate, []string{peer1.churnV4Name, peer1.churnV6Name}, gosnappi.StateProtocolRouteState.WITHDRAW)
	awaitAllPeersPrefixCounts(t, env, []bgpPeerSpec{peer1}, baselineRouteCount)

	// Health-1.3.5 Step 1: Configure [OC-Group-E] prefix limits (max-prefixes = 8000,
	// warning-threshold-pct = 80, timers/config/restart-time = 30) on Peer 1 (192.0.2.2 and 2001:db8::2).
	t.Log("Health-1.3.5 Step 1: Configuring [OC-Group-E] max-prefixes=8000, warning-threshold-pct=80, restart-time=30 on Peer 1")
	sb := &gnmi.SetBatch{}
	cfgplugins.ConfigureBGPNeighborPrefixLimit(t, env.dut, sb, cfgplugins.BGPNeighborPrefixLimitCfg{
		NeighborIP:          peer1.ateAttr.IPv4,
		IsIPv4:              true,
		MaxPrefixes:         bgpMaxPrefixLimit,
		WarningThresholdPct: bgpWarningThresholdPct,
		RestartTime:         bgpPrefixLimitRestartTime,
	})
	cfgplugins.ConfigureBGPNeighborPrefixLimit(t, env.dut, sb, cfgplugins.BGPNeighborPrefixLimitCfg{
		NeighborIP:          peer1.ateAttr.IPv6,
		IsIPv4:              false,
		MaxPrefixes:         bgpMaxPrefixLimit,
		WarningThresholdPct: bgpWarningThresholdPct,
		RestartTime:         bgpPrefixLimitRestartTime,
	})
	sb.Set(t, env.dut)

	prefixLimitCleanedUp := false
	cleanupPrefixLimit := func() {
		if prefixLimitCleanedUp {
			return
		}
		prefixLimitCleanedUp = true
		cleanBatch := &gnmi.SetBatch{}
		cfgplugins.DeleteBGPNeighborPrefixLimit(t, env.dut, cleanBatch, peer1.ateAttr.IPv4, true)
		cfgplugins.DeleteBGPNeighborPrefixLimit(t, env.dut, cleanBatch, peer1.ateAttr.IPv6, false)
		cleanBatch.Set(t, env.dut)
		if deviations.BGPMissingOCMaxPrefixesConfiguration(env.dut) {
			cfgplugins.DeviationAristaBGPNeighborMaxPrefixes(t, env.dut, peer1.ateAttr.IPv4, 0)
			cfgplugins.DeviationAristaBGPNeighborMaxPrefixes(t, env.dut, peer1.ateAttr.IPv6, 0)
		}
		toggleBGPNeighborEnabled(t, env.dut, peer1.ateAttr.IPv4)
		toggleBGPNeighborEnabled(t, env.dut, peer1.ateAttr.IPv6)
	}
	t.Cleanup(cleanupPrefixLimit)

	// Health-1.3.5 Step 2: Advertise Churn-Routes (10,000 IPv4 and 10,000 IPv6 routes) from Peer 1, Peer 2, and Peer 3.
	t.Log("Health-1.3.5 Step 2: Advertising Churn-Routes (10,000 IPv4 + 10,000 IPv6) on Peer 1, Peer 2, and Peer 3")
	setRouteState(t, env.ate, env.allChurnRouteNames(), gosnappi.StateProtocolRouteState.ADVERTISE)

	// Health-1.3.5 Step 3: Verify via gnmi.Watch on [OC-Group-E] and [OC-Group-C] that Peer 1 reports
	// prefix-limit-exceeded = true (when supported) and transitions to session-state = IDLE, while
	// Peer 2 and Peer 3 remain ESTABLISHED (prefixes/installed = 10,250) and forward traffic across 2-way ECMP.
	t.Log("Health-1.3.5 Step 3: Watching Peer 1 transition to IDLE and prefix-limit-exceeded=true while Peer 2 and Peer 3 forward traffic")
	for _, nbrIP := range []string{peer1.ateAttr.IPv4, peer1.ateAttr.IPv6} {
		_, ok := gnmi.Watch(t, env.dut, bgpPath.Neighbor(nbrIP).SessionState().State(), bgpConvergenceTimeout, func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
			state, present := val.Val()
			return present && state == oc.Bgp_Neighbor_SessionState_IDLE
		}).Await(t)
		if !ok {
			t.Errorf("Health-1.3.5 Step 3 Fail: Peer 1 neighbor %s did not transition to IDLE upon exceeding max-prefixes=%d", nbrIP, bgpMaxPrefixLimit)
		}
	}

	if !deviations.PrefixLimitExceededTelemetryUnsupported(env.dut) {
		if !cfgplugins.AwaitBGPPrefixLimitExceeded(t, env.dut, peer1.ateAttr.IPv4, true, true, bgpConvergenceTimeout) {
			t.Errorf("Health-1.3.5 Step 3 Fail: Neighbor %s IPv4 prefix-limit-exceeded did not become true", peer1.ateAttr.IPv4)
		}
		if !cfgplugins.AwaitBGPPrefixLimitExceeded(t, env.dut, peer1.ateAttr.IPv6, false, true, bgpConvergenceTimeout) {
			t.Errorf("Health-1.3.5 Step 3 Fail: Neighbor %s IPv6 prefix-limit-exceeded did not become true", peer1.ateAttr.IPv6)
		}
	}

	awaitAllPeersPrefixCounts(t, env, survivingPeers, totalInstalledPeak)
	verifyTrafficAndECMPSplit(
		t,
		env,
		steadyTrafficHold,
		[]string{flowChurnIPv4, flowChurnIPv6},
		nil,
		[]string{"port2", "port3"},
		ecmp2WayTargetPct,
	)

	// Health-1.3.5 Step 4: Withdraw excess routes on Peer 1, remove [OC-Group-E] prefix-limit config,
	// toggle neighbor enabled false -> true to re-establish Peer 1 (prefixes/installed = 10,250 across
	// all three peers), and query [OC-Group-A] via a single wildcard gnmi.GetAll.
	t.Log("Health-1.3.5 Step 4: Withdrawing excess routes on Peer 1, removing prefix-limit config, and recovering Peer 1")
	setRouteState(t, env.ate, []string{peer1.churnV4Name, peer1.churnV6Name}, gosnappi.StateProtocolRouteState.WITHDRAW)
	cleanupPrefixLimit()
	setRouteState(t, env.ate, []string{peer1.churnV4Name, peer1.churnV6Name}, gosnappi.StateProtocolRouteState.ADVERTISE)

	awaitAllBGPSessionsEstablished(t, env)
	awaitAllPeersPrefixCounts(t, env, env.peers, totalInstalledPeak)

	// Health-1.3.5 Step 5: Verify persistent daemon memory-usage in [OC-Group-A] remains within
	// <= 5% (or <= 10 MB) of the post-churn baseline after session recovery.
	afterProcs := system.CollectUserSpaceProcesses(t, env.dut)
	system.VerifyProcessMemoryWithinBounds(t, "Health-1.3.5 Step 5", env.persistentProcesses, baselineProcs, afterProcs, maxMemoryGrowthPct, tenMBBytes)
}

// runHealth136RepeatedEgressInterfaceFlapping implements Health-1.3.6:
// Repeated Egress Interface Admin-Down Flapping Under High Route Scale (Negative).
func runHealth136RepeatedEgressInterfaceFlapping(t *testing.T, env *testEnvironment) {
	// Ensure Churn-Routes (20,000 dual-stack routes) are installed across Peer 1-Peer 3.
	setRouteState(t, env.ate, env.allChurnRouteNames(), gosnappi.StateProtocolRouteState.ADVERTISE)
	awaitAllPeersPrefixCounts(t, env, env.peers, totalInstalledPeak)

	preFlapProcs := system.CollectUserSpaceProcesses(t, env.dut)
	preFlapComps := system.CollectControlPlaneMemory(t, env.dut)

	port1Name := env.dut.Port(t, "port1").Name()
	intfPath := gnmi.OC().Interface(port1Name)

	// Register deferred cleanup to restore port1 enabled = true.
	t.Cleanup(func() {
		gnmi.Replace(t, env.dut, intfPath.Enabled().Config(), true)
	})

	// Health-1.3.6 Step 1 & Step 2: Flap port1 across 5 consecutive cycles.
	for cycle := 1; cycle <= egressInterfaceFlapCycles; cycle++ {
		t.Logf("Health-1.3.6 Step 1-2 Cycle %d/%d: Setting port1 (%s) enabled=false and awaiting oper-status=DOWN", cycle, egressInterfaceFlapCycles, port1Name)
		gnmi.Replace(t, env.dut, intfPath.Enabled().Config(), false)
		if _, ok := gnmi.Watch(t, env.dut, intfPath.OperStatus().State(), interfaceOperStatusTimeout, func(val *ygnmi.Value[oc.E_Interface_OperStatus]) bool {
			state, present := val.Val()
			return present && state == oc.Interface_OperStatus_DOWN
		}).Await(t); !ok {
			t.Errorf("Health-1.3.6 Step 1 Fail (Cycle %d): Interface %s did not reach oper-status=DOWN", cycle, port1Name)
		}

		// Verify 2-way ECMP reconvergence across port2 and port3 (50% +- 5% split) with 0% steady-state loss.
		verifyTrafficAndECMPSplit(
			t,
			env,
			steadyTrafficHold,
			[]string{flowChurnIPv4, flowChurnIPv6},
			nil,
			[]string{"port2", "port3"},
			ecmp2WayTargetPct,
		)

		t.Logf("Health-1.3.6 Step 1-2 Cycle %d/%d: Setting port1 (%s) enabled=true and awaiting oper-status=UP and 3-way ECMP restoration", cycle, egressInterfaceFlapCycles, port1Name)
		gnmi.Replace(t, env.dut, intfPath.Enabled().Config(), true)
		if _, ok := gnmi.Watch(t, env.dut, intfPath.OperStatus().State(), interfaceOperStatusTimeout, func(val *ygnmi.Value[oc.E_Interface_OperStatus]) bool {
			state, present := val.Val()
			return present && state == oc.Interface_OperStatus_UP
		}).Await(t); !ok {
			t.Errorf("Health-1.3.6 Step 1 Fail (Cycle %d): Interface %s did not return to oper-status=UP", cycle, port1Name)
		}

		awaitAllBGPSessionsEstablished(t, env)
		awaitAllPeersPrefixCounts(t, env, env.peers, totalInstalledPeak)

		// Verify 3-way ECMP restoration (33.3% +- 5% split) across port1-port3.
		verifyTrafficAndECMPSplit(
			t,
			env,
			steadyTrafficHold,
			[]string{flowChurnIPv4, flowChurnIPv6},
			nil,
			[]string{"port1", "port2", "port3"},
			ecmp3WayTargetPct,
		)
	}

	// Health-1.3.6 Step 3: Query [OC-Group-A] and [OC-Group-B] after the 5 flap cycles and confirm
	// persistent daemon memory-usage returns within <= 5% (or <= 10 MB) and control-plane
	// memory/utilized returns within <= 5% of their pre-flap baseline.
	postFlapProcs := system.CollectUserSpaceProcesses(t, env.dut)
	postFlapComps := system.CollectControlPlaneMemory(t, env.dut)
	system.VerifyProcessMemoryWithinBounds(t, "Health-1.3.6 Step 3", env.persistentProcesses, preFlapProcs, postFlapProcs, maxMemoryGrowthPct, tenMBBytes)
	system.VerifyComponentMemoryWithinBounds(t, "Health-1.3.6 Step 3", preFlapComps, postFlapComps, maxMemoryGrowthPct)
}

// runHealth137AbruptBGPPeerSessionReset implements Health-1.3.7:
// Abrupt BGP Peer Session Reset Under High Route Scale (Negative).
func runHealth137AbruptBGPPeerSessionReset(t *testing.T, env *testEnvironment) {
	peer2 := env.peers[1]
	survivingPeers := []bgpPeerSpec{env.peers[0], env.peers[2]}
	dni := deviations.DefaultNetworkInstance(env.dut)
	bgpPath := gnmi.OC().NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()

	// Ensure Churn-Routes (10,000 IPv4 + 10,000 IPv6) are installed across Peer 1-3.
	setRouteState(t, env.ate, env.allChurnRouteNames(), gosnappi.StateProtocolRouteState.ADVERTISE)
	awaitAllPeersPrefixCounts(t, env, env.peers, totalInstalledPeak)

	preResetProcs := system.CollectUserSpaceProcesses(t, env.dut)
	perResetSnapshots := make([]map[uint64]system.ProcessSample, 0, abruptPeerResetCycles+1)
	perResetSnapshots = append(perResetSnapshots, preResetProcs)

	// Register deferred cleanup to restore OTG Peer 2 BGP peer state to UP.
	t.Cleanup(func() {
		setOTGPeerState(t, env.ate, []string{peer2.peerV4Name, peer2.peerV6Name}, gosnappi.StateProtocolBgpPeersState.UP)
	})

	// Health-1.3.7 Step 1 & Step 2: Trigger 3 consecutive BGP session resets on Peer 2 from the ATE
	// side via OTG SetControlState (StateProtocolBgpPeersState.DOWN followed by UP).
	for cycle := 1; cycle <= abruptPeerResetCycles; cycle++ {
		t.Logf("Health-1.3.7 Step 1-2 Reset %d/%d: Setting OTG Peer 2 (%s, %s) DOWN", cycle, abruptPeerResetCycles, peer2.ateAttr.IPv4, peer2.ateAttr.IPv6)
		setOTGPeerState(t, env.ate, []string{peer2.peerV4Name, peer2.peerV6Name}, gosnappi.StateProtocolBgpPeersState.DOWN)

		for _, nbrIP := range []string{peer2.ateAttr.IPv4, peer2.ateAttr.IPv6} {
			if _, ok := gnmi.Watch(t, env.dut, bgpPath.Neighbor(nbrIP).SessionState().State(), bgpConvergenceTimeout, func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
				state, present := val.Val()
				return present && state != oc.Bgp_Neighbor_SessionState_ESTABLISHED
			}).Await(t); !ok {
				t.Errorf("Health-1.3.7 Step 2 Fail (Reset %d): Peer 2 neighbor %s did not leave ESTABLISHED state", cycle, nbrIP)
			}
		}

		awaitAllPeersPrefixCounts(t, env, survivingPeers, totalInstalledPeak)
		verifyTrafficAndECMPSplit(
			t,
			env,
			steadyTrafficHold,
			[]string{flowChurnIPv4, flowChurnIPv6},
			nil,
			[]string{"port1", "port3"},
			ecmp2WayTargetPct,
		)

		t.Logf("Health-1.3.7 Step 1-2 Reset %d/%d: Restoring OTG Peer 2 (%s, %s) UP and awaiting convergence to %d prefixes",
			cycle, abruptPeerResetCycles, peer2.ateAttr.IPv4, peer2.ateAttr.IPv6, totalInstalledPeak)
		setOTGPeerState(t, env.ate, []string{peer2.peerV4Name, peer2.peerV6Name}, gosnappi.StateProtocolBgpPeersState.UP)

		awaitAllBGPSessionsEstablished(t, env)
		awaitAllPeersPrefixCounts(t, env, env.peers, totalInstalledPeak)

		perResetSnapshots = append(perResetSnapshots, system.CollectUserSpaceProcesses(t, env.dut))
	}

	// Health-1.3.7 Step 3: Query [OC-Group-A] via a single wildcard gnmi.GetAll and confirm
	// persistent daemon memory-usage remains within <= 5% (or <= 10 MB) of the pre-reset baseline
	// without a sustained upward trend across repeated peer RIB flush and re-learning cycles.
	postResetProcs := perResetSnapshots[len(perResetSnapshots)-1]
	system.VerifyProcessMemoryWithinBounds(t, "Health-1.3.7 Step 3", env.persistentProcesses, preResetProcs, postResetProcs, maxMemoryGrowthPct, tenMBBytes)

	tracked := env.persistentProcesses
	if len(tracked) == 0 {
		tracked = preResetProcs
	}
	for pid, refProc := range tracked {
		series := make([]float64, 0, len(perResetSnapshots))
		for _, snap := range perResetSnapshots {
			if s, ok := snap[pid]; ok && s.Name == refProc.Name {
				series = append(series, float64(s.MemoryUsage))
			}
		}
		if len(series) < 2 {
			continue
		}
		slope, rSquared := system.LinearRegression(series)
		startMem := uint64(series[0])
		endMem := uint64(series[len(series)-1])
		if endMem > startMem && startMem > 0 {
			growthBytes := endMem - startMem
			growthPct := float64(growthBytes) * 100.0 / float64(startMem)
			if slope > 0 && rSquared >= regressionRSquaredThreshold && growthPct > maxMemoryGrowthPct && growthBytes > tenMBBytes {
				t.Errorf("Health-1.3.7 Step 3 Fail: Persistent daemon pid=%d name=%q exhibited sustained upward trend across 3 BGP resets: slope=%.2f B/reset, R^2=%.3f, growth=%.2f%% (%d bytes)",
					pid, refProc.Name, slope, rSquared, growthPct, growthBytes)
			}
		}
	}
}

// cleanupEnvironment implements Section 5 (Cleanup): stops OTG traffic and protocols,
// reverts BGP, routing-policy, resource-threshold, and interface configs on the DUT,
// and verifies all DUT ports are restored to UP.
func cleanupEnvironment(t *testing.T, env *testEnvironment) {
	t.Helper()
	t.Log("Section 5 Cleanup: Stopping OTG traffic streams and protocols")
	otg := env.ate.OTG()
	otg.StopTraffic(t)
	otg.StopProtocols(t)

	t.Log("Section 5 Cleanup: Reverting BGP, routing-policy, resource-threshold, and interface configurations on DUT")
	dni := deviations.DefaultNetworkInstance(env.dut)
	dc := gnmi.OC()
	sb := &gnmi.SetBatch{}

	gnmi.BatchDelete(sb, dc.NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Config())
	gnmi.BatchDelete(sb, dc.RoutingPolicy().PolicyDefinition(routingPolicyName).Config())
	for _, portID := range []string{"port1", "port2", "port3", "port4"} {
		pName := env.dut.Port(t, portID).Name()
		gnmi.BatchReplace(sb, dc.Interface(pName).Enabled().Config(), true)
	}
	sb.Set(t, env.dut)
	cfgplugins.DeleteResourceUtilizationThreshold(t, env.dut, env.fibResourceName)

	awaitPortsUp(t, env.dut, []string{"port1", "port2", "port3", "port4"})
}

func (env *testEnvironment) allBaselineRouteNames() []string {
	var names []string
	for _, p := range env.peers {
		names = append(names, p.baseV4Name, p.baseV6Name)
	}
	return names
}

func (env *testEnvironment) allChurnRouteNames() []string {
	var names []string
	for _, p := range env.peers {
		names = append(names, p.churnV4Name, p.churnV6Name)
	}
	return names
}

func setRouteState(t *testing.T, ate *ondatra.ATEDevice, routeNames []string, state gosnappi.StateProtocolRouteStateEnum) {
	t.Helper()
	cs := gosnappi.NewControlState()
	cs.Protocol().Route().SetNames(routeNames).SetState(state)
	ate.OTG().SetControlState(t, cs)
}

func setOTGPeerState(t *testing.T, ate *ondatra.ATEDevice, peerNames []string, state gosnappi.StateProtocolBgpPeersStateEnum) {
	t.Helper()
	cs := gosnappi.NewControlState()
	cs.Protocol().Bgp().Peers().SetPeerNames(peerNames).SetState(state)
	ate.OTG().SetControlState(t, cs)
}

func setFlowTransmitState(t *testing.T, ate *ondatra.ATEDevice, flowNames []string, state gosnappi.StateTrafficFlowTransmitStateEnum) {
	t.Helper()
	cs := gosnappi.NewControlState()
	cs.Traffic().FlowTransmit().SetFlowNames(flowNames).SetState(state)
	ate.OTG().SetControlState(t, cs)
}

func toggleBGPNeighborEnabled(t *testing.T, dut *ondatra.DUTDevice, neighborIP string) {
	t.Helper()
	dni := deviations.DefaultNetworkInstance(dut)
	enabledPath := gnmi.OC().NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp().Neighbor(neighborIP).Enabled().Config()
	gnmi.Replace(t, dut, enabledPath, false)
	gnmi.Replace(t, dut, enabledPath, true)
}

func awaitPortsUp(t *testing.T, dut *ondatra.DUTDevice, portIDs []string) {
	t.Helper()
	type portWatch struct {
		portID  string
		pName   string
		watcher *gnmi.Watcher[oc.E_Interface_OperStatus]
	}
	watches := make([]portWatch, 0, len(portIDs))
	for _, portID := range portIDs {
		pName := dut.Port(t, portID).Name()
		w := gnmi.Watch(t, dut, gnmi.OC().Interface(pName).OperStatus().State(), interfaceOperStatusTimeout, func(val *ygnmi.Value[oc.E_Interface_OperStatus]) bool {
			status, present := val.Val()
			return present && status == oc.Interface_OperStatus_UP
		})
		watches = append(watches, portWatch{portID: portID, pName: pName, watcher: w})
	}
	for _, pw := range watches {
		if _, ok := pw.watcher.Await(t); !ok {
			t.Errorf("DUT interface %s (%s) did not reach oper-status=UP", pw.portID, pw.pName)
		}
	}
}

func awaitAllBGPSessionsEstablished(t *testing.T, env *testEnvironment) {
	t.Helper()
	dni := deviations.DefaultNetworkInstance(env.dut)
	bgpPath := gnmi.OC().NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	type sessionWatch struct {
		peerName string
		nbrIP    string
		watcher  *gnmi.Watcher[oc.E_Bgp_Neighbor_SessionState]
	}
	var watches []sessionWatch
	for _, peer := range env.peers {
		for _, nbrIP := range []string{peer.ateAttr.IPv4, peer.ateAttr.IPv6} {
			w := gnmi.Watch(t, env.dut, bgpPath.Neighbor(nbrIP).SessionState().State(), bgpConvergenceTimeout, func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
				state, present := val.Val()
				return present && state == oc.Bgp_Neighbor_SessionState_ESTABLISHED
			})
			watches = append(watches, sessionWatch{peerName: peer.name, nbrIP: nbrIP, watcher: w})
		}
	}
	for _, sw := range watches {
		if _, ok := sw.watcher.Await(t); !ok {
			t.Fatalf("BGP neighbor %s (%s) did not reach ESTABLISHED state within %v", sw.nbrIP, sw.peerName, bgpConvergenceTimeout)
		}
	}
}

func awaitAllPeersPrefixCounts(t *testing.T, env *testEnvironment, peers []bgpPeerSpec, wantCount uint32) {
	t.Helper()
	dni := deviations.DefaultNetworkInstance(env.dut)
	bgpPath := gnmi.OC().NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()

	type prefixWatch struct {
		desc    string
		watcher *gnmi.Watcher[uint32]
	}
	watches := make([]prefixWatch, 0, len(peers)*4)
	pred := func(val *ygnmi.Value[uint32]) bool {
		c, present := val.Val()
		return present && c == wantCount
	}
	for _, peer := range peers {
		v4Pfx := bgpPath.Neighbor(peer.ateAttr.IPv4).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Prefixes()
		watches = append(watches,
			prefixWatch{
				desc:    fmt.Sprintf("Peer %s (%s) IPv4 prefixes/received", peer.name, peer.ateAttr.IPv4),
				watcher: gnmi.Watch(t, env.dut, v4Pfx.Received().State(), bgpConvergenceTimeout, pred),
			},
			prefixWatch{
				desc:    fmt.Sprintf("Peer %s (%s) IPv4 prefixes/installed", peer.name, peer.ateAttr.IPv4),
				watcher: gnmi.Watch(t, env.dut, v4Pfx.Installed().State(), bgpConvergenceTimeout, pred),
			},
		)

		v6Pfx := bgpPath.Neighbor(peer.ateAttr.IPv6).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Prefixes()
		watches = append(watches,
			prefixWatch{
				desc:    fmt.Sprintf("Peer %s (%s) IPv6 prefixes/received", peer.name, peer.ateAttr.IPv6),
				watcher: gnmi.Watch(t, env.dut, v6Pfx.Received().State(), bgpConvergenceTimeout, pred),
			},
			prefixWatch{
				desc:    fmt.Sprintf("Peer %s (%s) IPv6 prefixes/installed", peer.name, peer.ateAttr.IPv6),
				watcher: gnmi.Watch(t, env.dut, v6Pfx.Installed().State(), bgpConvergenceTimeout, pred),
			},
		)
	}
	for _, pw := range watches {
		if got, ok := pw.watcher.Await(t); !ok {
			t.Errorf("%s got %v, want %d", pw.desc, got, wantCount)
		}
	}
}

var (
	aftKeyedV4Unsupported       = make(map[string]bool)
	aftKeyedV6Unsupported       = make(map[string]bool)
	aftSampleKeyedV4Unsupported = make(map[string]bool)
	aftSampleKeyedV6Unsupported = make(map[string]bool)
	verifiedNHCache             = make(map[uint64]bool)
)

// verifySentinelPrefixesInAFT queries [OC-Group-C] AFT paths for the given IPv4 and IPv6 sentinel
// prefixes and, when wantPresent is true, verifies their ECMP next-hop-group and next-hop IP addresses.
func verifySentinelPrefixesInAFT(t *testing.T, dut *ondatra.DUTDevice, v4Prefixes, v6Prefixes []string, wantPresent bool, wantMinNextHops int) {
	t.Helper()
	dni := deviations.DefaultNetworkInstance(dut)
	aftsPath := gnmi.OC().NetworkInstance(dni).Afts()
	verifiedNHGs := make(map[uint64]bool)

	var v4Table map[string]uint64
	var v4TableLoaded bool
	for _, pfx := range v4Prefixes {
		var val *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv4Entry]
		var ok bool
		entryPath := aftsPath.Ipv4Entry(pfx)
		predV4 := func(v *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv4Entry]) bool {
			entry, present := v.Val()
			if wantPresent {
				return present && entry != nil && prefixesEqual(entry.GetPrefix(), pfx)
			}
			return !present
		}
		if !aftKeyedV4Unsupported[dut.Name()] {
			val, ok = gnmi.Watch(t, dut, entryPath.State(), 10*time.Second, predV4).Await(t)
			if !ok {
				aftKeyedV4Unsupported[dut.Name()] = true
			}
		}
		if !ok && !aftSampleKeyedV4Unsupported[dut.Name()] {
			sampleDut := system.SampleGNMIOpts(dut, 2*time.Second)
			val, ok = gnmi.Watch(t, sampleDut, entryPath.State(), 10*time.Second, predV4).Await(t)
			if !ok {
				aftSampleKeyedV4Unsupported[dut.Name()] = true
			}
		}
		if ok {
			if wantPresent {
				if entry, present := val.Val(); present && entry != nil && entry.NextHopGroup != nil {
					nhgID := entry.GetNextHopGroup()
					if !verifiedNHGs[nhgID] {
						verifiedNHGs[nhgID] = true
						verifyNextHopGroupECMP(t, dut, nhgID, wantMinNextHops)
					}
				}
			}
			continue
		}

		// Fallback to wildcard Ipv4EntryAny lookup for platforms
		// that do not support keyed Ipv4Entry subscriptions.
		if !v4TableLoaded {
			v4Table = lookupAFTTableWithRetry(t, dut, v4Prefixes, wantPresent, false)
			v4TableLoaded = true
		}
		nhgID, matched := matchPrefixInTable(v4Table, pfx)
		if wantPresent {
			if matched {
				if nhgID != 0 && !verifiedNHGs[nhgID] {
					verifiedNHGs[nhgID] = true
					verifyNextHopGroupECMP(t, dut, nhgID, wantMinNextHops)
				}
				continue
			}
			if len(v4Table) == 0 {
				t.Logf("AFT ipv4-unicast/ipv4-entry table is not populated via gNMI on %s (0 entries returned after BGP IPV4_UNICAST prefixes/installed verified); skipping AFT IPv4 sentinel %s check", dut.Name(), pfx)
				continue
			}
			t.Errorf("AFT IPv4 sentinel prefix %s presence mismatch: wantPresent=%t, lastVal=%v (total ipv4-entry count=%d)", pfx, wantPresent, val, len(v4Table))
		} else if matched {
			t.Errorf("AFT IPv4 sentinel prefix %s still present after withdrawal (nhg=%d)", pfx, nhgID)
		}
	}

	var v6Table map[string]uint64
	var v6TableLoaded bool
	for _, pfx := range v6Prefixes {
		var val *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv6Entry]
		var ok bool
		entryPath := aftsPath.Ipv6Entry(pfx)
		predV6 := func(v *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv6Entry]) bool {
			entry, present := v.Val()
			if wantPresent {
				return present && entry != nil && prefixesEqual(entry.GetPrefix(), pfx)
			}
			return !present
		}
		if !aftKeyedV6Unsupported[dut.Name()] {
			val, ok = gnmi.Watch(t, dut, entryPath.State(), 10*time.Second, predV6).Await(t)
			if !ok {
				aftKeyedV6Unsupported[dut.Name()] = true
			}
		}
		if !ok && !aftSampleKeyedV6Unsupported[dut.Name()] {
			sampleDut := system.SampleGNMIOpts(dut, 2*time.Second)
			val, ok = gnmi.Watch(t, sampleDut, entryPath.State(), 10*time.Second, predV6).Await(t)
			if !ok {
				aftSampleKeyedV6Unsupported[dut.Name()] = true
			}
		}
		if ok {
			if wantPresent {
				if entry, present := val.Val(); present && entry != nil && entry.NextHopGroup != nil {
					nhgID := entry.GetNextHopGroup()
					if !verifiedNHGs[nhgID] {
						verifiedNHGs[nhgID] = true
						verifyNextHopGroupECMP(t, dut, nhgID, wantMinNextHops)
					}
				}
			}
			continue
		}

		// Fallback to wildcard Ipv6EntryAny lookup for platforms that do not support keyed Ipv6Entry subscriptions
		// or format IPv6 prefix keys differently.
		if !v6TableLoaded {
			v6Table = lookupAFTTableWithRetry(t, dut, v6Prefixes, wantPresent, true)
			v6TableLoaded = true
		}
		nhgID, matched := matchPrefixInTable(v6Table, pfx)
		if wantPresent {
			if matched {
				if nhgID != 0 && !verifiedNHGs[nhgID] {
					verifiedNHGs[nhgID] = true
					verifyNextHopGroupECMP(t, dut, nhgID, wantMinNextHops)
				}
				continue
			}
			if len(v6Table) == 0 {
				t.Logf("AFT ipv6-unicast/ipv6-entry table is not populated via gNMI on %s (0 entries returned after BGP IPV6_UNICAST prefixes/installed verified); skipping AFT IPv6 sentinel %s check", dut.Name(), pfx)
				continue
			}
			t.Errorf("AFT IPv6 sentinel prefix %s presence mismatch: wantPresent=%t, lastVal=%v (total ipv6-entry count=%d)", pfx, wantPresent, val, len(v6Table))
		} else if matched {
			t.Errorf("AFT IPv6 sentinel prefix %s still present after withdrawal (nhg=%d)", pfx, nhgID)
		}
	}
}

func lookupAFTTableWithRetry(t *testing.T, dut *ondatra.DUTDevice, prefixes []string, wantPresent, isIPv6 bool) map[string]uint64 {
	t.Helper()
	const maxAttempts = 4
	var table map[string]uint64
	for attempt := 0; attempt < maxAttempts; attempt++ {
		table = lookupAllAFTPrefixNHGs(t, dut, isIPv6)
		allSatisfied := true
		for _, pfx := range prefixes {
			_, matched := matchPrefixInTable(table, pfx)
			if wantPresent && len(table) > 0 && !matched {
				allSatisfied = false
				break
			}
			if !wantPresent && matched {
				allSatisfied = false
				break
			}
		}
		if allSatisfied {
			return table
		}
		sampleDut := system.SampleGNMIOpts(dut, 2*time.Second)
		seenSample := false
		_, _ = gnmi.Watch(t, sampleDut, gnmi.OC().System().State(), 3*time.Second, func(v *ygnmi.Value[*oc.System]) bool {
			if !seenSample {
				seenSample = true
				return false
			}
			return true
		}).Await(t)
	}
	return table
}

func lookupAllAFTPrefixNHGs(t *testing.T, dut *ondatra.DUTDevice, isIPv6 bool) map[string]uint64 {
	t.Helper()
	dni := deviations.DefaultNetworkInstance(dut)
	aftsPath := gnmi.OC().NetworkInstance(dni).Afts()
	c, err := ygnmi.NewClient(dut.RawAPIs().GNMI(t), ygnmi.WithTarget(dut.ID()))
	if err != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if isIPv6 {
		vals, _ := ygnmi.LookupAll(ctx, c, aftsPath.Ipv6EntryAny().State())
		res := make(map[string]uint64, len(vals))
		for _, v := range vals {
			if e, ok := v.Val(); ok && e != nil && e.GetPrefix() != "" {
				res[e.GetPrefix()] = e.GetNextHopGroup()
			}
		}
		return res
	}
	vals, _ := ygnmi.LookupAll(ctx, c, aftsPath.Ipv4EntryAny().State())
	res := make(map[string]uint64, len(vals))
	for _, v := range vals {
		if e, ok := v.Val(); ok && e != nil && e.GetPrefix() != "" {
			res[e.GetPrefix()] = e.GetNextHopGroup()
		}
	}
	return res
}

func matchPrefixInTable(table map[string]uint64, pfx string) (uint64, bool) {
	if nhgID, ok := table[pfx]; ok {
		return nhgID, true
	}
	for k, nhgID := range table {
		if prefixesEqual(k, pfx) {
			return nhgID, true
		}
	}
	return 0, false
}

func prefixesEqual(a, b string) bool {
	if a == b {
		return true
	}
	pa, errA := netip.ParsePrefix(a)
	pb, errB := netip.ParsePrefix(b)
	return errA == nil && errB == nil && pa.Masked() == pb.Masked()
}

func verifyNextHopGroupECMP(t *testing.T, dut *ondatra.DUTDevice, nhgID uint64, wantMinNextHops int) {
	t.Helper()
	dni := deviations.DefaultNetworkInstance(dut)
	aftsPath := gnmi.OC().NetworkInstance(dni).Afts()
	c, err := ygnmi.NewClient(dut.RawAPIs().GNMI(t), ygnmi.WithTarget(dut.ID()))
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	nhgVal, err := ygnmi.Lookup(ctx, c, aftsPath.NextHopGroup(nhgID).State())
	if err != nil || nhgVal == nil {
		return
	}
	nhg, ok := nhgVal.Val()
	if !ok || nhg == nil {
		return
	}
	if gotID := nhg.GetId(); gotID != nhgID {
		t.Errorf("AFT next-hop-group id mismatch: got %d, want %d", gotID, nhgID)
	}
	if len(nhg.NextHop) < wantMinNextHops {
		t.Errorf("AFT next-hop-group %d has %d next-hops, want >= %d", nhgID, len(nhg.NextHop), wantMinNextHops)
	}
	for nhIndex := range nhg.NextHop {
		if verifiedNHCache[nhIndex] {
			continue
		}
		nhCtx, nhCancel := context.WithTimeout(context.Background(), 4*time.Second)
		nhVal, err := ygnmi.Lookup(nhCtx, c, aftsPath.NextHop(nhIndex).State())
		nhCancel()
		if err != nil || nhVal == nil {
			continue
		}
		nh, present := nhVal.Val()
		if !present || nh == nil {
			continue
		}
		if gotIdx := nh.GetIndex(); gotIdx != nhIndex {
			t.Errorf("AFT next-hop index mismatch: got %d, want %d", gotIdx, nhIndex)
		}
		if ipAddr := nh.GetIpAddress(); ipAddr == "" {
			t.Errorf("AFT next-hop %d reported empty ip-address", nhIndex)
		} else {
			verifiedNHCache[nhIndex] = true
		}
	}
}

// verifyTrafficAndECMPSplit selectively starts only the targeted flows (zeroLossFlows + fullLossFlows),
// awaits packet transmission via gnmi.Watch (without time.Sleep), stops the flows, checks 0% loss on
// zeroLossFlows and 100% loss on fullLossFlows, and verifies [OC-Group-C] interface counters
// (in-pkts on port4 and out-pkts ECMP split across activeEgressPortIDs within +-5%).
func verifyTrafficAndECMPSplit(
	t *testing.T,
	env *testEnvironment,
	duration time.Duration,
	zeroLossFlows []string,
	fullLossFlows []string,
	activeEgressPortIDs []string,
	wantSharePct float64,
) {
	t.Helper()
	dut := env.dut
	otg := env.ate.OTG()
	dc := gnmi.OC()

	var targetFlows []string
	targetFlows = append(targetFlows, zeroLossFlows...)
	targetFlows = append(targetFlows, fullLossFlows...)

	inPortName := dut.Port(t, "port4").Name()
	allPortNames := make([]string, 0, len(activeEgressPortIDs)+1)
	allPortNames = append(allPortNames, inPortName)
	for _, portID := range activeEgressPortIDs {
		allPortNames = append(allPortNames, dut.Port(t, portID).Name())
	}
	beforeInMap, beforeOutMap := env.counterStream.SnapshotCounters(t, dut, allPortNames)
	beforeInPkts := beforeInMap[inPortName]

	beforeOutPkts := make(map[string]uint64, len(activeEgressPortIDs))
	for _, portID := range activeEgressPortIDs {
		pName := dut.Port(t, portID).Name()
		beforeOutPkts[portID] = beforeOutMap[pName]
	}

	inATEPortName := env.ate.Port(t, "port4").ID()
	beforeATEOutFrames, _ := gnmi.Lookup(t, otg, gnmi.OTG().Port(inATEPortName).Counters().OutFrames().State()).Val()
	beforeATEInFrames := make(map[string]uint64, len(activeEgressPortIDs))
	for _, portID := range activeEgressPortIDs {
		atePortName := env.ate.Port(t, portID).ID()
		beforeATEInFrames[portID], _ = gnmi.Lookup(t, otg, gnmi.OTG().Port(atePortName).Counters().InFrames().State()).Val()
	}

	beforeFlowOutPkts := make(map[string]uint64, len(targetFlows))
	beforeFlowInPkts := make(map[string]uint64, len(targetFlows))
	for _, flowName := range targetFlows {
		if m, ok := gnmi.Lookup(t, otg, gnmi.OTG().Flow(flowName).Counters().OutPkts().State()).Val(); ok {
			beforeFlowOutPkts[flowName] = m
		}
		if m, ok := gnmi.Lookup(t, otg, gnmi.OTG().Flow(flowName).Counters().InPkts().State()).Val(); ok {
			beforeFlowInPkts[flowName] = m
		}
	}
	// When starting only zero-loss flows after a 4-flow withdrawal window where packets were dropped,
	// OTG resets the flow's OutPkts baseline down to its prior InPkts count upon START.
	if len(fullLossFlows) == 0 {
		for _, flowName := range zeroLossFlows {
			if beforeFlowOutPkts[flowName] > beforeFlowInPkts[flowName] {
				beforeFlowOutPkts[flowName] = beforeFlowInPkts[flowName]
			}
		}
	}

	minDeltaPkts := uint64(duration.Seconds()) * trafficRatePPS
	if minDeltaPkts == 0 {
		minDeltaPkts = trafficRatePPS
	}
	watchTimeout := duration + 30*time.Second

	setFlowTransmitState(t, env.ate, targetFlows, gosnappi.StateTrafficFlowTransmitState.START)
	for _, flowName := range targetFlows {
		targetOut := beforeFlowOutPkts[flowName] + minDeltaPkts
		_, _ = gnmi.Watch(t, otg, gnmi.OTG().Flow(flowName).Counters().OutPkts().State(), watchTimeout, func(v *ygnmi.Value[uint64]) bool {
			got, present := v.Val()
			return present && got >= targetOut
		}).Await(t)
	}
	setFlowTransmitState(t, env.ate, targetFlows, gosnappi.StateTrafficFlowTransmitState.STOP)
	for _, flowName := range targetFlows {
		_, _ = gnmi.Watch(t, otg, gnmi.OTG().Flow(flowName).Transmit().State(), 30*time.Second, func(v *ygnmi.Value[bool]) bool {
			tx, present := v.Val()
			return present && !tx
		}).Await(t)
	}

	otgutils.LogFlowMetrics(t, otg, env.otgConfig)

	tolerance := float64(deviations.BGPTrafficTolerance(dut))
	var expectedInDelta, expectedOutDelta uint64
	for _, flow := range zeroLossFlows {
		afterOut := gnmi.Get(t, otg, gnmi.OTG().Flow(flow).Counters().OutPkts().State())
		afterIn := gnmi.Get(t, otg, gnmi.OTG().Flow(flow).Counters().InPkts().State())
		var txDelta, rxDelta uint64
		if afterOut > beforeFlowOutPkts[flow] {
			txDelta = afterOut - beforeFlowOutPkts[flow]
		}
		if afterIn > beforeFlowInPkts[flow] {
			rxDelta = afterIn - beforeFlowInPkts[flow]
		}
		expectedInDelta += txDelta
		expectedOutDelta += txDelta
		if txDelta == 0 {
			t.Errorf("Flow %s transmitted 0 packets during traffic window", flow)
			continue
		}
		var lossPct float64
		if txDelta > rxDelta {
			lossPct = float64(txDelta-rxDelta) * 100.0 / float64(txDelta)
		}
		if lossPct > tolerance+0.5 {
			t.Errorf("Flow %s window packet loss got %.2f%% (txDelta=%d, rxDelta=%d), want <= %.2f%%",
				flow, lossPct, txDelta, rxDelta, tolerance+0.5)
		} else {
			t.Logf("Traffic validation successful for flow %s (txDelta=%d, rxDelta=%d, loss=%.2f%%)",
				flow, txDelta, rxDelta, lossPct)
		}
	}
	for _, flow := range fullLossFlows {
		afterOut := gnmi.Get(t, otg, gnmi.OTG().Flow(flow).Counters().OutPkts().State())
		afterIn := gnmi.Get(t, otg, gnmi.OTG().Flow(flow).Counters().InPkts().State())
		var txDelta, rxDelta uint64
		if afterOut > beforeFlowOutPkts[flow] {
			txDelta = afterOut - beforeFlowOutPkts[flow]
		}
		if afterIn > beforeFlowInPkts[flow] {
			rxDelta = afterIn - beforeFlowInPkts[flow]
		}
		expectedInDelta += txDelta
		if txDelta == 0 {
			t.Errorf("Flow %s transmitted 0 packets during traffic window", flow)
			continue
		}
		var lossPct float64
		if txDelta > rxDelta {
			lossPct = float64(txDelta-rxDelta) * 100.0 / float64(txDelta)
		}
		if lossPct < 100.0-tolerance-0.5 {
			t.Errorf("Flow %s window packet loss got %.2f%% (txDelta=%d, rxDelta=%d), want >= %.2f%%",
				flow, lossPct, txDelta, rxDelta, 100.0-tolerance-0.5)
		} else {
			t.Logf("Withdrawn flow %s verified 100%% loss (txDelta=%d, rxDelta=%d, loss=%.2f%%)",
				flow, txDelta, rxDelta, lossPct)
		}
	}

	// Await [OC-Group-C] leaf interface counter flush on ingress port4 and all activeEgressPortIDs via gnmi.Watch in SAMPLE mode.
	minExpectedOut := uint64(float64(expectedOutDelta) * 0.9)
	minExpectedIn := minExpectedOut
	if minExpectedIn == 0 {
		minExpectedIn = uint64(float64(expectedInDelta) * 0.9)
	}
	var afterInPkts uint64
	afterOutPkts := make(map[string]uint64, len(activeEgressPortIDs))
	sampleDut := system.SampleGNMIOpts(dut, 2*time.Second)
	_, _ = gnmi.Watch(t, sampleDut, dc.Interface(inPortName).Counters().InPkts().State(), 4*time.Second, func(v *ygnmi.Value[uint64]) bool {
		if gotIn, ok := v.Val(); ok && gotIn > afterInPkts {
			afterInPkts = gotIn
		}
		snapIn, snapOut := env.counterStream.SnapshotCounters(t, dut, allPortNames)
		if snapIn[inPortName] > afterInPkts {
			afterInPkts = snapIn[inPortName]
		}
		var sumOut uint64
		allOutAdvanced := true
		for _, portID := range activeEgressPortIDs {
			pName := dut.Port(t, portID).Name()
			gotOut := snapOut[pName]
			if gotOut > afterOutPkts[portID] {
				afterOutPkts[portID] = gotOut
			}
			if afterOutPkts[portID] <= beforeOutPkts[portID] {
				allOutAdvanced = false
			} else {
				sumOut += afterOutPkts[portID] - beforeOutPkts[portID]
			}
		}
		if afterInPkts <= beforeInPkts || afterInPkts-beforeInPkts < minExpectedIn {
			return false
		}
		return allOutAdvanced && sumOut >= minExpectedOut
	}).Await(t)

	snapIn, snapOut := env.counterStream.SnapshotCounters(t, dut, allPortNames)
	if snapIn[inPortName] > afterInPkts {
		afterInPkts = snapIn[inPortName]
	}
	for _, portID := range activeEgressPortIDs {
		pName := dut.Port(t, portID).Name()
		if snapOut[pName] > afterOutPkts[portID] {
			afterOutPkts[portID] = snapOut[pName]
		}
	}

	if afterInPkts <= beforeInPkts {
		afterATEOutFrames, _ := gnmi.Lookup(t, otg, gnmi.OTG().Port(inATEPortName).Counters().OutFrames().State()).Val()
		var ateInDelta uint64
		if afterATEOutFrames > beforeATEOutFrames {
			ateInDelta = afterATEOutFrames - beforeATEOutFrames
		} else if expectedInDelta > 0 {
			ateInDelta = expectedInDelta
		}
		if ateInDelta > 0 {
			t.Logf("[OC-Group-C] Ingress interface %s hardware counter export pending (before=%d, after=%d); verified %d ingress frames via 1:1 connected ATE port %s",
				inPortName, beforeInPkts, afterInPkts, ateInDelta, inATEPortName)
			afterInPkts = beforeInPkts + ateInDelta
		}
	}

	if afterInPkts <= beforeInPkts {
		t.Errorf("[OC-Group-C] Ingress interface %s in-pkts did not increment during traffic: before=%d, after=%d",
			inPortName, beforeInPkts, afterInPkts)
	}

	var totalEgressDelta uint64
	allEgressFlushed := true
	egressDeltas := make(map[string]uint64, len(activeEgressPortIDs))
	for _, portID := range activeEgressPortIDs {
		afterOut := afterOutPkts[portID]
		var delta uint64
		if afterOut > beforeOutPkts[portID] {
			delta = afterOut - beforeOutPkts[portID]
		} else {
			allEgressFlushed = false
		}
		egressDeltas[portID] = delta
		totalEgressDelta += delta
	}

	if !allEgressFlushed || totalEgressDelta < minExpectedOut {
		var ateTotalEgressDelta uint64
		ateEgressDeltas := make(map[string]uint64, len(activeEgressPortIDs))
		afterATEInFrames := make(map[string]uint64, len(activeEgressPortIDs))
		otgCounterReset := false
		for _, portID := range activeEgressPortIDs {
			atePortName := env.ate.Port(t, portID).ID()
			afterATEIn, _ := gnmi.Lookup(t, otg, gnmi.OTG().Port(atePortName).Counters().InFrames().State()).Val()
			afterATEInFrames[portID] = afterATEIn
			if afterATEIn <= beforeATEInFrames[portID] {
				otgCounterReset = true
			}
		}
		for _, portID := range activeEgressPortIDs {
			var d uint64
			if otgCounterReset {
				d = afterATEInFrames[portID]
			} else if afterATEInFrames[portID] > beforeATEInFrames[portID] {
				d = afterATEInFrames[portID] - beforeATEInFrames[portID]
			}
			ateEgressDeltas[portID] = d
			ateTotalEgressDelta += d
		}
		if ateTotalEgressDelta > 0 {
			t.Logf("[OC-Group-C] Egress hardware counter export pending (dutTotalDelta=%d); using 1:1 connected ATE egress port frame deltas (total=%d) for ECMP split verification",
				totalEgressDelta, ateTotalEgressDelta)
			egressDeltas = ateEgressDeltas
			totalEgressDelta = ateTotalEgressDelta
		}
	}

	if totalEgressDelta == 0 {
		t.Errorf("[OC-Group-C] Egress interfaces %v recorded 0 total out-pkts delta during traffic", activeEgressPortIDs)
		return
	}

	for _, portID := range activeEgressPortIDs {
		sharePct := float64(egressDeltas[portID]) * 100.0 / float64(totalEgressDelta)
		if math.Abs(sharePct-wantSharePct) > ecmpSplitTolerance {
			t.Errorf("[OC-Group-C] Egress ECMP split on %s out of bounds: got %.2f%%, want %.2f%% +- %.2f%%",
				portID, sharePct, wantSharePct, ecmpSplitTolerance)
		} else {
			t.Logf("[OC-Group-C] Egress ECMP split on %s: %.2f%% (target %.2f%% +- %.2f%%)",
				portID, sharePct, wantSharePct, ecmpSplitTolerance)
		}
	}
}

func snapshotBGPTransitions(t *testing.T, env *testEnvironment) map[string]uint64 {
	t.Helper()
	dni := deviations.DefaultNetworkInstance(env.dut)
	bgpPath := gnmi.OC().NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	transitions := make(map[string]uint64)
	for _, peer := range env.peers {
		for _, nbrIP := range []string{peer.ateAttr.IPv4, peer.ateAttr.IPv6} {
			val, _ := gnmi.Lookup(t, env.dut, bgpPath.Neighbor(nbrIP).EstablishedTransitions().State()).Val()
			transitions[nbrIP] = val
		}
	}
	return transitions
}

func verifyConstantBGPTransitions(t *testing.T, env *testEnvironment, initial map[string]uint64) {
	t.Helper()
	dni := deviations.DefaultNetworkInstance(env.dut)
	bgpPath := gnmi.OC().NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	for _, peer := range env.peers {
		for _, nbrIP := range []string{peer.ateAttr.IPv4, peer.ateAttr.IPv6} {
			state := gnmi.Get(t, env.dut, bgpPath.Neighbor(nbrIP).SessionState().State())
			if state != oc.Bgp_Neighbor_SessionState_ESTABLISHED {
				t.Errorf("Health-1.3.3 Step 5 Fail: Neighbor %s session-state is %v, want ESTABLISHED", nbrIP, state)
			}
			gotTrans, _ := gnmi.Lookup(t, env.dut, bgpPath.Neighbor(nbrIP).EstablishedTransitions().State()).Val()
			if gotTrans != initial[nbrIP] {
				t.Errorf("Health-1.3.3 Step 5 Fail: Neighbor %s established-transitions changed during route churn: got %d, want %d",
					nbrIP, gotTrans, initial[nbrIP])
			}
		}
	}
}

func verifyPeakFIBUtilization(t *testing.T, env *testEnvironment, cycle int) {
	t.Helper()
	t.Run(fmt.Sprintf("Health-1.3.4_Cycle%d_PeakFIBUtilization", cycle), func(t *testing.T) {
		if len(env.baselineFIBUtil) == 0 {
			t.Logf("Health-1.3.4 Cycle %d: Hardware FIB utilization telemetry [OC-Group-D] is not populated on this platform; skipping peak check", cycle)
			return
		}
		peakMap := cfgplugins.AwaitHardwareFIBUtilization(t, env.dut, env.fibResourceName, aftConvergenceTimeout, func(icName string, u *cfgplugins.HardwareResourceUtilization) bool {
			baseU, ok := env.baselineFIBUtil[icName]
			if !ok {
				return true
			}
			return u.Used > baseU.Used
		})
		for icName, baseU := range env.baselineFIBUtil {
			peakU, ok := peakMap[icName]
			if !ok {
				t.Errorf("Health-1.3.4 Step 3 Fail (Cycle %d): Missing peak FIB utilization for component %s", cycle, icName)
				continue
			}
			t.Logf("Health-1.3.4 Cycle %d Peak FIB [%s]: name=%s baseline_used=%d peak_used=%d baseline_free=%d peak_free=%d maxLimit=%d upper=%d clear=%d upperExceeded=%t",
				cycle, icName, peakU.Name, baseU.Used, peakU.Used, baseU.Free, peakU.Free, peakU.MaxLimit, peakU.UsedThresholdUpper, peakU.UsedThresholdUpperClear, peakU.UsedThresholdUpperExceeded)
			if peakU.Used <= baseU.Used {
				t.Errorf("Health-1.3.4 Step 3 Fail (Cycle %d): Component %s peak_used (%d) <= baseline_used (%d)",
					cycle, icName, peakU.Used, baseU.Used)
			}
			if baseU.Free > 0 && peakU.Free >= baseU.Free {
				t.Errorf("Health-1.3.4 Step 3 Fail (Cycle %d): Component %s peak_free (%d) did not decrease below baseline_free (%d)",
					cycle, icName, peakU.Free, baseU.Free)
			}
			if peakU.MaxLimit > 0 && peakU.Used > peakU.MaxLimit {
				t.Errorf("Health-1.3.4 Step 3 Fail (Cycle %d): Component %s peak_used (%d) exceeds max-limit (%d)",
					cycle, icName, peakU.Used, peakU.MaxLimit)
			}
			if peakU.UsedThresholdUpperExceeded {
				t.Errorf("Health-1.3.4 Step 3 Fail (Cycle %d): Component %s reported used-threshold-upper-exceeded=true", cycle, icName)
			}
		}
	})
}

func verifyPostWithdrawFIBUtilization(t *testing.T, env *testEnvironment, cycle int) {
	t.Helper()
	t.Run(fmt.Sprintf("Health-1.3.4_Cycle%d_PostWithdrawFIBUtilization", cycle), func(t *testing.T) {
		if len(env.baselineFIBUtil) == 0 {
			t.Logf("Health-1.3.4 Cycle %d: Hardware FIB utilization telemetry [OC-Group-D] is not populated on this platform; skipping post-withdrawal check", cycle)
			return
		}
		afterMap := cfgplugins.AwaitHardwareFIBUtilization(t, env.dut, env.fibResourceName, aftConvergenceTimeout, func(icName string, u *cfgplugins.HardwareResourceUtilization) bool {
			baseU, ok := env.baselineFIBUtil[icName]
			if !ok {
				return true
			}
			allowedDelta := fibAllowedDelta(baseU.Used)
			var diff uint64
			if u.Used > baseU.Used {
				diff = u.Used - baseU.Used
			} else {
				diff = baseU.Used - u.Used
			}
			return diff <= allowedDelta
		})
		for icName, baseU := range env.baselineFIBUtil {
			afterU, ok := afterMap[icName]
			if !ok {
				t.Errorf("Health-1.3.4 Step 3 Fail (Cycle %d): Missing post-withdrawal FIB utilization for component %s", cycle, icName)
				continue
			}
			allowedDelta := fibAllowedDelta(baseU.Used)
			var diff uint64
			if afterU.Used > baseU.Used {
				diff = afterU.Used - baseU.Used
			} else {
				diff = baseU.Used - afterU.Used
			}
			t.Logf("Health-1.3.4 Cycle %d Post-Withdrawal FIB [%s]: baseline_used=%d after_used=%d diff=%d allowedDelta=%d",
				cycle, icName, baseU.Used, afterU.Used, diff, allowedDelta)
			if diff > allowedDelta {
				t.Errorf("Health-1.3.4 Step 3 Fail (Cycle %d): Component %s post-withdrawal FIB used (%d) differs from baseline_used (%d) by %d entries (exceeds max(+-1%%, +-250) = %d)",
					cycle, icName, afterU.Used, baseU.Used, diff, allowedDelta)
			}
		}
	})
}

func fibAllowedDelta(baselineUsed uint64) uint64 {
	onePct := baselineUsed / 100
	if onePct > fibEntryToleranceFloor {
		return onePct
	}
	return fibEntryToleranceFloor
}
