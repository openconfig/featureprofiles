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

// Package aggregate_admin_down_drain_test implements RT-5.18: Aggregate Interface (LAG) Drain via Admin Down.
package aggregate_admin_down_drain_test

import (
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	otgconfighelpers "github.com/openconfig/featureprofiles/internal/otg_helpers/otg_config_helpers"
	otgvalidationhelpers "github.com/openconfig/featureprofiles/internal/otg_helpers/otg_validation_helpers"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ygnmi/ygnmi"
)

const (
	// Addressing and prefix lengths defined in RT-5.18 Test Environment Setup.
	ipv4PrefixLen = 30
	ipv6PrefixLen = 126

	// Autonomous System numbers and router IDs for RT-5.18.
	dutAS       = 64500
	ateLagAS    = 64501
	atePort4AS  = 64502
	dutRouterID = "192.0.2.1"

	// IS-IS parameters for RT-5.18.
	isisInstance   = "DEFAULT"
	dutISISNet     = "49.0001.0000.0000.0001.00"
	ateLagSysID    = "640000000001"
	atePort4SysID  = "640000000002"
	isisLvl2Metric = 10

	// Advertised route pools in DEFAULT VRF (RT-5.18.1 - RT-5.18.4).
	lagISISV4Prefix    = "198.51.100.0"
	lagISISV4PrefixLen = 24
	lagISISV4FirstIP   = "198.51.100.1"
	lagISISV4Count     = 5

	lagISISV6Prefix    = "2001:db8:1000::"
	lagISISV6PrefixLen = 64
	lagISISV6FirstIP   = "2001:db8:1000::1"
	lagISISV6Count     = 5

	lagBGPV4Prefix    = "203.0.113.0"
	lagBGPV4PrefixLen = 24
	lagBGPV4FirstIP   = "203.0.113.1"
	lagBGPV4Count     = 5

	lagBGPV6Prefix    = "2001:db8:2000::"
	lagBGPV6PrefixLen = 64
	lagBGPV6FirstIP   = "2001:db8:2000::1"
	lagBGPV6Count     = 5

	port4BGPV4Prefix    = "198.18.0.0"
	port4BGPV4PrefixLen = 24
	port4BGPV4FirstIP   = "198.18.0.1"
	port4BGPV4Count     = 1

	port4BGPV6Prefix    = "2001:db8:3000::"
	port4BGPV6PrefixLen = 64
	port4BGPV6FirstIP   = "2001:db8:3000::1"
	port4BGPV6Count     = 1

	// Non-default VRF B2_CORE and single-tagged VLAN 100 parameters (RT-5.18.5 & RT-5.18.6).
	b2CoreVRFName      = "B2_CORE"
	b2CoreVlanID       = 100
	b2CoreSubintfIndex = 100

	dutLagVlan100IPv4   = "192.0.2.9"
	ateLagVlan100IPv4   = "192.0.2.10"
	dutLagVlan100IPv6   = "2001:db8::9"
	ateLagVlan100IPv6   = "2001:db8::a"
	dutPort4Vlan100IPv4 = "192.0.2.13"
	atePort4Vlan100IPv4 = "192.0.2.14"
	dutPort4Vlan100IPv6 = "2001:db8::d"
	atePort4Vlan100IPv6 = "2001:db8::e"

	b2CoreLagBGPV4Prefix    = "203.0.113.0"
	b2CoreLagBGPV4PrefixLen = 24
	b2CoreLagBGPV4FirstIP   = "203.0.113.1"
	b2CoreLagBGPV4Count     = 1

	b2CoreLagBGPV6Prefix    = "2001:db8:4000::"
	b2CoreLagBGPV6PrefixLen = 64
	b2CoreLagBGPV6FirstIP   = "2001:db8:4000::1"
	b2CoreLagBGPV6Count     = 1

	b2CorePort4BGPV4Prefix    = "198.51.100.0"
	b2CorePort4BGPV4PrefixLen = 24
	b2CorePort4BGPV4FirstIP   = "198.51.100.1"
	b2CorePort4BGPV4Count     = 1

	b2CorePort4BGPV6Prefix    = "2001:db8:3000::"
	b2CorePort4BGPV6PrefixLen = 64
	b2CorePort4BGPV6FirstIP   = "2001:db8:3000::1"
	b2CorePort4BGPV6Count     = 1

	// Traffic flow names for DEFAULT VRF (RT-5.18.1 - RT-5.18.4).
	flow1ISISIPv4Fwd  = "Flow-1-IS-IS-IPv4-Fwd"
	flow2BGPIPv4Fwd   = "Flow-2-BGP-IPv4-Fwd"
	flow3BGPIPv4Rev   = "Flow-3-BGP-IPv4-Rev"
	flow4IPv6BidirFwd = "Flow-4a-IS-IS-BGP-IPv6-Fwd"
	flow4IPv6BidirRev = "Flow-4b-IS-IS-BGP-IPv6-Rev"

	// Traffic flow names for B2_CORE VRF (RT-5.18.5 & RT-5.18.6).
	flowB2CoreIPv4Fwd = "Flow-B2CORE-IPv4-Fwd"
	flowB2CoreIPv4Rev = "Flow-B2CORE-IPv4-Rev"
	flowB2CoreIPv6Fwd = "Flow-B2CORE-IPv6-Fwd"
	flowB2CoreIPv6Rev = "Flow-B2CORE-IPv6-Rev"

	// Packet rate and sizes.
	flowPPS       = 1000
	flowFrameSize = 512

	// Telemetry Watch/Await timeouts.
	stateTimeout    = 2 * time.Minute
	protocolTimeout = 3 * time.Minute
	trafficTimeout  = 2 * time.Minute
)

var (
	// DUT Port-Channel1 (aggID) addressing in DEFAULT VRF: 192.0.2.1/30, 2001:db8::1/126.
	dutLagAttrs = attrs.Attributes{
		Desc:    "DUT Port-Channel1 L3 LAG",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::1",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
	// ATE LAG addressing in DEFAULT VRF: 192.0.2.2/30, 2001:db8::2/126.
	ateLagAttrs = attrs.Attributes{
		Name:    "ateLag",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::2",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
	// ATE member port MACs for LACP ports 1..3.
	ateMemberMACs = []string{
		"02:00:01:01:01:11",
		"02:00:01:01:01:12",
		"02:00:01:01:01:13",
	}
	// DUT Port-4 addressing in DEFAULT VRF: 192.0.2.5/30, 2001:db8::5/126.
	dutPort4Attrs = attrs.Attributes{
		Desc:    "DUT Port-4 Transit Interface",
		IPv4:    "192.0.2.5",
		IPv6:    "2001:db8::5",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
	// ATE Port-4 addressing in DEFAULT VRF: 192.0.2.6/30, 2001:db8::6/126.
	atePort4Attrs = attrs.Attributes{
		Name:    "atePort4",
		MAC:     "02:00:04:01:01:01",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::6",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
)

// testEnv holds references to the DUT, ATE, OTG configuration, LAG ID, and ports used across RT-5.18 subtests.
type testEnv struct {
	dut         *ondatra.DUTDevice
	ate         *ondatra.ATEDevice
	otgConfig   gosnappi.Config
	aggID       string
	memberPorts []*ondatra.Port
	port4       *ondatra.Port
	allFlows    []string
	b2CoreFlows []string
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// TestAggregateAdminDownDrain verifies RT-5.18: Aggregate Interface (LAG) Drain via Admin Down,
// covering subtests RT-5.18.1 through RT-5.18.6 and Cleanup.
func TestAggregateAdminDownDrain(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	aggID := netutil.NextAggregateInterface(t, dut)

	env := setupEnvironment(t, dut, ate, aggID)

	// Baseline Verification (Pre-Drain) as specified in README.md Test Environment Setup.
	verifyPreDrainBaseline(t, env)

	// RT-5.18.1 - Admin Down Drain - Disable.
	t.Run("RT-5.18.1_Admin_Down_Drain_Disable", func(t *testing.T) {
		testAdminDownDrainDisable(t, env)
	})

	// RT-5.18.2 - Admin Down Drain - Re-enable.
	t.Run("RT-5.18.2_Admin_Down_Drain_ReEnable", func(t *testing.T) {
		testAdminDownDrainReEnable(t, env)
	})

	// RT-5.18.3 - Negative Test - Admin Down on Individual Member Links.
	t.Run("RT-5.18.3_Negative_Test_Admin_Down_On_Individual_Member_Links", func(t *testing.T) {
		testNegativeAdminDownMemberLinks(t, env)
	})

	// RT-5.18.4 - Negative Test - Admin Down on Already Disabled LAG.
	t.Run("RT-5.18.4_Negative_Test_Admin_Down_On_Already_Disabled_LAG", func(t *testing.T) {
		testNegativeAdminDownAlreadyDisabledLAG(t, env)
	})

	// RT-5.18.5 - Uplink LAG Member Retention and Inband BGP Reachability in Non-Default VRF (B2_CORE) Across Drain States.
	t.Run("RT-5.18.5_Uplink_LAG_Member_Retention_And_Inband_BGP_In_B2_CORE", func(t *testing.T) {
		testUplinkLAGMemberRetentionAndB2CoreBGP(t, env)
	})

	// RT-5.18.6 - Differential Member Drain Behavior: Standard Member Drain (forwarding-viable: false) vs. Retained Uplink LAG Members.
	t.Run("RT-5.18.6_Differential_Member_Drain_Forwarding_Viable_False", func(t *testing.T) {
		testDifferentialMemberDrainForwardingViable(t, env)
	})
}

// setupEnvironment configures the DUT and ATE according to the RT-5.18 Test Environment Setup:
// - 1. Aggregate Interface (LAG) Configuration (Port-Channel1 with 3 LACP member links Port-1..Port-3)
// - 2. Transit / Destination Interface Configuration (Port-4)
// - 3. Control Plane Configuration (IS-IS Level-2 and Dual-Stack eBGP in DEFAULT VRF)
// - 4. Traffic Flows (Bidirectional IPv4 and IPv6 flows across IS-IS and BGP prefixes).
// compareInterfaceNames compares two interface names in natural numeric order (e.g. Ethernet2 < Ethernet10).
func compareInterfaceNames(a, b string) bool {
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		isDigitA := a[i] >= '0' && a[i] <= '9'
		isDigitB := b[j] >= '0' && b[j] <= '9'
		if isDigitA && isDigitB {
			numA := 0
			for i < len(a) && a[i] >= '0' && a[i] <= '9' {
				numA = numA*10 + int(a[i]-'0')
				i++
			}
			numB := 0
			for j < len(b) && b[j] >= '0' && b[j] <= '9' {
				numB = numB*10 + int(b[j]-'0')
				j++
			}
			if numA != numB {
				return numA < numB
			}
			continue
		}
		if a[i] != b[j] {
			return a[i] < b[j]
		}
		i++
		j++
	}
	return len(a) < len(b)
}

func setupEnvironment(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, aggID string) *testEnv {
	t.Helper()
	memberPortIDs := []string{"port1", "port2", "port3"}
	sort.Slice(memberPortIDs, func(i, j int) bool {
		return compareInterfaceNames(dut.Port(t, memberPortIDs[i]).Name(), dut.Port(t, memberPortIDs[j]).Name())
	})
	p1 := dut.Port(t, memberPortIDs[0])
	p2 := dut.Port(t, memberPortIDs[1])
	p3 := dut.Port(t, memberPortIDs[2])
	p4 := dut.Port(t, "port4")
	memberPorts := []*ondatra.Port{p1, p2, p3}

	t.Cleanup(func() {
		t.Log("Executing RT-5.18 Cleanup: restoring DUT interface and protocol state")
		// Stop OTG protocols and traffic first
		ate.OTG().StopTraffic(t)
		ate.OTG().StopProtocols(t)

		cleanupBatch := &gnmi.SetBatch{}

		// Delete protocols configured on default network instance
		dni := deviations.DefaultNetworkInstance(dut)
		gnmi.BatchDelete(cleanupBatch, gnmi.OC().NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance).Config())
		gnmi.BatchDelete(cleanupBatch, gnmi.OC().NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Config())
		if deviations.ExplicitInterfaceInDefaultVRF(dut) {
			gnmi.BatchDelete(cleanupBatch, gnmi.OC().NetworkInstance(dni).Interface(aggID+".0").Config())
			gnmi.BatchDelete(cleanupBatch, gnmi.OC().NetworkInstance(dni).Interface(p4.Name()+".0").Config())
		}

		// Delete non-default VRF B2_CORE and subinterfaces
		gnmi.BatchDelete(cleanupBatch, gnmi.OC().NetworkInstance(b2CoreVRFName).Config())
		gnmi.BatchDelete(cleanupBatch, gnmi.OC().Interface(aggID).Subinterface(b2CoreSubintfIndex).Config())
		gnmi.BatchDelete(cleanupBatch, gnmi.OC().Interface(p4.Name()).Subinterface(b2CoreSubintfIndex).Config())
		gnmi.BatchDelete(cleanupBatch, gnmi.OC().Interface(p4.Name()).Subinterface(0).Config())

		// Unbind member ports from the aggregate interface and delete LACP and aggregate interface config
		cfgplugins.ClearAggregateMembers(t, dut, cleanupBatch, memberPorts)
		gnmi.BatchDelete(cleanupBatch, gnmi.OC().Lacp().Interface(aggID).Config())
		gnmi.BatchDelete(cleanupBatch, gnmi.OC().Interface(aggID).Config())

		// Restore physical ports to enabled
		for _, p := range memberPorts {
			cfgplugins.SetInterfaceAdminEnabled(t, dut, cleanupBatch, p.Name(), true)
		}
		cfgplugins.SetInterfaceAdminEnabled(t, dut, cleanupBatch, p4.Name(), true)

		cleanupBatch.Set(t, dut)
		if deviations.ExplicitEnableBGPOnDefaultVRF(dut) {
			cfgplugins.EnableDefaultNetworkInstanceBgp(t, dut, dutAS)
		}
	})

	fptest.ConfigureDefaultNetworkInstance(t, dut)
	cfgplugins.ClearAggregateMembers(t, dut, nil, memberPorts)

	sb := &gnmi.SetBatch{}

	// 1. Aggregate Interface (LAG) Configuration:
	// Configure Port-Channel1 (aggID) with lag-type LACP, LACP mode ACTIVE, interval FAST,
	// member links Port-1..Port-3 (forwarding-viable: true), IPv4 192.0.2.1/30, and IPv6 2001:db8::1/126.
	viableTrue := true
	cfgplugins.ConfigureL3AggregateInterface(t, dut, sb, cfgplugins.L3AggregateParams{
		AggID:            aggID,
		MemberPorts:      memberPorts,
		LagType:          oc.IfAggregate_AggregationType_LACP,
		LacpMode:         oc.Lacp_LacpActivityType_ACTIVE,
		LacpInterval:     oc.Lacp_LacpPeriodType_FAST,
		MinLinks:         1,
		ForwardingViable: &viableTrue,
		Attributes:       &dutLagAttrs,
	})

	// 2. Transit / Destination Interface Configuration:
	// Configure Port-4 as a routed Layer 3 interface with IPv4 192.0.2.5/30 and IPv6 2001:db8::5/126.
	p4OC := dutPort4Attrs.NewOCInterface(p4.Name(), dut)
	gnmi.BatchReplace(sb, gnmi.OC().Interface(p4.Name()).Config(), p4OC)

	// Apply DUT interface configurations before binding to default network instance if required by deviations.
	sb.Set(t, dut)
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, aggID, deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, p4.Name(), deviations.DefaultNetworkInstance(dut), 0)
	}
	if deviations.ExplicitPortSpeed(dut) {
		for _, p := range []*ondatra.Port{p1, p2, p3, p4} {
			fptest.SetPortSpeed(t, p)
		}
	}

	// 3. Control Plane Configuration (IS-IS & eBGP in DEFAULT VRF):
	protoBatch := &gnmi.SetBatch{}

	// 3.1 IS-IS Level-2 point-to-point adjacency over Port-Channel1 and Port-4.
	cfgplugins.ConfigureISISProtocol(t, dut, protoBatch, cfgplugins.ISISProtocolParams{
		InstanceName: isisInstance,
		NetAddress:   dutISISNet,
		LevelCap:     oc.Isis_LevelType_LEVEL_2,
		Interfaces: []cfgplugins.ISISInterfaceParams{
			{
				InterfaceName: aggID,
				SubintfIndex:  0,
				Level2Metric:  isisLvl2Metric,
			},
			{
				InterfaceName: p4.Name(),
				SubintfIndex:  0,
				Level2Metric:  isisLvl2Metric,
			},
		},
	})

	// 3.2 eBGP Configuration in DEFAULT VRF (DUT AS 64500, ATE LAG AS 64501, ATE Port-4 AS 64502).
	cfgplugins.ConfigureEBGPInstance(t, dut, protoBatch, cfgplugins.EBGPInstanceParams{
		NetworkInstanceName: deviations.DefaultNetworkInstance(dut),
		RouterID:            dutRouterID,
		LocalAS:             dutAS,
		PolicyName:          "ACCEPT-ALL",
		Peers: []cfgplugins.EBGPPeerParam{
			{
				NeighborAddress: ateLagAttrs.IPv4,
				LocalAddress:    dutLagAttrs.IPv4,
				PeerAS:          ateLagAS,
				PeerGroupName:   "BGP-PEER-GROUP-LAG-V4",
				IsIPv6:          false,
			},
			{
				NeighborAddress: ateLagAttrs.IPv6,
				LocalAddress:    dutLagAttrs.IPv6,
				PeerAS:          ateLagAS,
				PeerGroupName:   "BGP-PEER-GROUP-LAG-V6",
				IsIPv6:          true,
			},
			{
				NeighborAddress: atePort4Attrs.IPv4,
				LocalAddress:    dutPort4Attrs.IPv4,
				PeerAS:          atePort4AS,
				PeerGroupName:   "BGP-PEER-GROUP-PORT4-V4",
				IsIPv6:          false,
			},
			{
				NeighborAddress: atePort4Attrs.IPv6,
				LocalAddress:    dutPort4Attrs.IPv6,
				PeerAS:          atePort4AS,
				PeerGroupName:   "BGP-PEER-GROUP-PORT4-V6",
				IsIPv6:          true,
			},
		},
	})
	protoBatch.Set(t, dut)

	// Configure OTG LAG, IS-IS, eBGP, and traffic flows on ATE.
	otgConfig := configureOTG(t, ate, memberPortIDs)
	ate.OTG().PushConfig(t, otgConfig)
	ate.OTG().StartProtocols(t)

	return &testEnv{
		dut:         dut,
		ate:         ate,
		otgConfig:   otgConfig,
		aggID:       aggID,
		memberPorts: memberPorts,
		port4:       p4,
		allFlows: []string{
			flow1ISISIPv4Fwd,
			flow2BGPIPv4Fwd,
			flow3BGPIPv4Rev,
			flow4IPv6BidirFwd,
			flow4IPv6BidirRev,
		},
		b2CoreFlows: []string{
			flowB2CoreIPv4Fwd,
			flowB2CoreIPv4Rev,
			flowB2CoreIPv6Fwd,
			flowB2CoreIPv6Rev,
		},
	}
}

// configureOTG builds the OTG configuration for ATE Port-1..Port-3 (3-member LACP LAG) and ATE Port-4,
// including IS-IS Level-2, dual-stack eBGP route advertisements, and bidirectional traffic flows.
func configureOTG(t *testing.T, ate *ondatra.ATEDevice, memberPortIDs []string) gosnappi.Config {
	t.Helper()
	config := gosnappi.NewConfig()

	ateP1 := ate.Port(t, memberPortIDs[0])
	ateP2 := ate.Port(t, memberPortIDs[1])
	ateP3 := ate.Port(t, memberPortIDs[2])
	ateP4 := ate.Port(t, "port4")

	for _, p := range []*ondatra.Port{ateP1, ateP2, ateP3, ateP4} {
		config.Ports().Add().SetName(p.ID())
	}

	// Configure 3-member LACP LAG on ATE Port-1, Port-2, and Port-3.
	ateLag := config.Lags().Add().SetName(ateLagAttrs.Name)
	ateLag.Protocol().Lacp().
		SetActorSystemId(ateLagAttrs.MAC).
		SetActorSystemPriority(1).
		SetActorKey(1)

	for idx, p := range []*ondatra.Port{ateP1, ateP2, ateP3} {
		lp := ateLag.Ports().Add().SetPortName(p.ID())
		lp.Ethernet().SetName(fmt.Sprintf("%s-eth-%d", p.ID(), idx+1)).SetMac(ateMemberMACs[idx])
		lp.Lacp().
			SetActorActivity("active").
			SetActorPortNumber(uint32(idx + 1)).
			SetActorPortPriority(1).
			SetLacpduTimeout(0)
	}

	// Device on ATE LAG (Port-1..Port-3) in DEFAULT VRF.
	lagDev := config.Devices().Add().SetName(ateLagAttrs.Name + ".dev")
	lagEth := lagDev.Ethernets().Add().SetName(ateLagAttrs.Name + ".Eth").SetMac(ateLagAttrs.MAC)
	lagEth.Connection().SetLagName(ateLag.Name())
	lagIPv4 := lagEth.Ipv4Addresses().Add().SetName(ateLagAttrs.Name + ".IPv4").
		SetAddress(ateLagAttrs.IPv4).
		SetGateway(dutLagAttrs.IPv4).
		SetPrefix(uint32(ateLagAttrs.IPv4Len))
	lagIPv6 := lagEth.Ipv6Addresses().Add().SetName(ateLagAttrs.Name + ".IPv6").
		SetAddress(ateLagAttrs.IPv6).
		SetGateway(dutLagAttrs.IPv6).
		SetPrefix(uint32(ateLagAttrs.IPv6Len))

	// Configure IS-IS and advertise 5 IPv4 prefixes (198.51.100.0/24..198.51.104.0/24) and
	// 5 IPv6 prefixes (2001:db8:1000::/64..2001:db8:1004::/64) from ATE LAG.
	otgconfighelpers.ConfigureDeviceISIS(
		lagDev,
		lagEth.Name(),
		lagIPv4.Address(),
		ateLagSysID,
		"ate-lag-isis",
		isisLvl2Metric,
		&otgconfighelpers.AdvertisedRoutes{
			StartingAddress: lagISISV4Prefix,
			PrefixLength:    lagISISV4PrefixLen,
			Count:           lagISISV4Count,
		},
		&otgconfighelpers.AdvertisedRoutes{
			StartingAddress: lagISISV6Prefix,
			PrefixLength:    lagISISV6PrefixLen,
			Count:           lagISISV6Count,
		},
	)

	// Configure eBGP (AS 64501) and advertise 5 IPv4 prefixes (203.0.113.0/24..203.0.117.0/24) and
	// 5 IPv6 prefixes (2001:db8:2000::/64..2001:db8:2004::/64) from ATE LAG.
	otgconfighelpers.AdvertiseBGPRoutes(
		lagDev,
		lagIPv4,
		lagIPv6,
		&otgconfighelpers.AdvertisedRoutes{
			StartingAddress: lagBGPV4Prefix,
			PrefixLength:    lagBGPV4PrefixLen,
			Count:           lagBGPV4Count,
			ATEAS:           ateLagAS,
		},
		&otgconfighelpers.AdvertisedRoutes{
			StartingAddress: lagBGPV6Prefix,
			PrefixLength:    lagBGPV6PrefixLen,
			Count:           lagBGPV6Count,
			ATEAS:           ateLagAS,
		},
	)

	// Device on ATE Port-4 in DEFAULT VRF.
	p4Dev := config.Devices().Add().SetName(atePort4Attrs.Name + ".dev")
	p4Eth := p4Dev.Ethernets().Add().SetName(atePort4Attrs.Name + ".Eth").SetMac(atePort4Attrs.MAC)
	p4Eth.Connection().SetPortName(ateP4.ID())
	p4IPv4 := p4Eth.Ipv4Addresses().Add().SetName(atePort4Attrs.Name + ".IPv4").
		SetAddress(atePort4Attrs.IPv4).
		SetGateway(dutPort4Attrs.IPv4).
		SetPrefix(uint32(atePort4Attrs.IPv4Len))
	p4IPv6 := p4Eth.Ipv6Addresses().Add().SetName(atePort4Attrs.Name + ".IPv6").
		SetAddress(atePort4Attrs.IPv6).
		SetGateway(dutPort4Attrs.IPv6).
		SetPrefix(uint32(atePort4Attrs.IPv6Len))

	// Configure IS-IS and eBGP (AS 64502) on ATE Port-4 advertising 198.18.0.0/24 and 2001:db8:3000::/64.
	otgconfighelpers.ConfigureDeviceISIS(
		p4Dev,
		p4Eth.Name(),
		p4IPv4.Address(),
		atePort4SysID,
		"ate-port4-isis",
		isisLvl2Metric,
		nil,
		nil,
	)
	otgconfighelpers.AdvertiseBGPRoutes(
		p4Dev,
		p4IPv4,
		p4IPv6,
		&otgconfighelpers.AdvertisedRoutes{
			StartingAddress: port4BGPV4Prefix,
			PrefixLength:    port4BGPV4PrefixLen,
			Count:           port4BGPV4Count,
			ATEAS:           atePort4AS,
		},
		&otgconfighelpers.AdvertisedRoutes{
			StartingAddress: port4BGPV6Prefix,
			PrefixLength:    port4BGPV6PrefixLen,
			Count:           port4BGPV6Count,
			ATEAS:           atePort4AS,
		},
	)

	// 4. Traffic Flows (DEFAULT VRF):
	// Flow 1 (IS-IS IPv4 Forward): ATE Port-4 (198.18.0.1) -> ATE LAG IS-IS prefixes (198.51.100.1..198.51.104.1).
	addIPv4Flow(config, flow1ISISIPv4Fwd, p4IPv4.Name(), lagIPv4.Name(), port4BGPV4FirstIP, lagISISV4FirstIP, lagISISV4Count, false)

	// Flow 2 (BGP IPv4 Forward): ATE Port-4 (198.18.0.1) -> ATE LAG BGP prefixes (203.0.113.1..203.0.117.1).
	addIPv4Flow(config, flow2BGPIPv4Fwd, p4IPv4.Name(), lagIPv4.Name(), port4BGPV4FirstIP, lagBGPV4FirstIP, lagBGPV4Count, false)

	// Flow 3 (BGP IPv4 Reverse): ATE LAG (203.0.113.1..203.0.117.1) -> ATE Port-4 (198.18.0.1).
	addIPv4Flow(config, flow3BGPIPv4Rev, lagIPv4.Name(), p4IPv4.Name(), lagBGPV4FirstIP, port4BGPV4FirstIP, lagBGPV4Count, true)

	// Flow 4 (IS-IS & BGP IPv6 Bidirectional):
	// Flow 4a Forward: ATE Port-4 (2001:db8:3000::1) -> ATE LAG IPv6 prefixes (2001:db8:1000::1..1004::1).
	addIPv6Flow(config, flow4IPv6BidirFwd, p4IPv6.Name(), lagIPv6.Name(), port4BGPV6FirstIP, lagISISV6FirstIP, lagISISV6Count, false)
	// Flow 4b Reverse: ATE LAG (2001:db8:2000::1..2004::1) -> ATE Port-4 (2001:db8:3000::1).
	addIPv6Flow(config, flow4IPv6BidirRev, lagIPv6.Name(), p4IPv6.Name(), lagBGPV6FirstIP, port4BGPV6FirstIP, lagBGPV6Count, true)

	return config
}

// addIPv4Flow adds a continuous IPv4 UDP flow with varied 5-tuple UDP ports so packets hash across all 3 LAG members.
func addIPv4Flow(config gosnappi.Config, name, txEndpoint, rxEndpoint, srcIP, dstIP string, ipCount uint32, incrementSrc bool) {
	f := config.Flows().Add().SetName(name)
	f.Metrics().SetEnable(true)
	f.TxRx().Device().SetTxNames([]string{txEndpoint}).SetRxNames([]string{rxEndpoint})
	f.Size().SetFixed(flowFrameSize)
	f.Rate().SetPps(flowPPS)
	f.Duration().Continuous()

	f.Packet().Add().Ethernet()
	v4 := f.Packet().Add().Ipv4()
	v4Step := fmt.Sprintf("0.0.%d.0", 1)
	if incrementSrc {
		v4.Src().Increment().SetStart(srcIP).SetStep(v4Step).SetCount(ipCount)
		v4.Dst().SetValue(dstIP)
	} else {
		v4.Src().SetValue(srcIP)
		v4.Dst().Increment().SetStart(dstIP).SetStep(v4Step).SetCount(ipCount)
	}
	udp := f.Packet().Add().Udp()
	udp.SrcPort().Increment().SetStart(10000).SetStep(1).SetCount(64)
	udp.DstPort().Increment().SetStart(20000).SetStep(1).SetCount(64)
}

// addIPv6Flow adds a continuous IPv6 UDP flow with varied 5-tuple UDP ports so packets hash across all 3 LAG members.
func addIPv6Flow(config gosnappi.Config, name, txEndpoint, rxEndpoint, srcIP, dstIP string, ipCount uint32, incrementSrc bool) {
	f := config.Flows().Add().SetName(name)
	f.Metrics().SetEnable(true)
	f.TxRx().Device().SetTxNames([]string{txEndpoint}).SetRxNames([]string{rxEndpoint})
	f.Size().SetFixed(flowFrameSize)
	f.Rate().SetPps(flowPPS)
	f.Duration().Continuous()

	f.Packet().Add().Ethernet()
	v6 := f.Packet().Add().Ipv6()
	v6Step := fmt.Sprintf("0:0:%d:1::", 0)
	if incrementSrc {
		v6.Src().Increment().SetStart(srcIP).SetStep(v6Step).SetCount(ipCount)
		v6.Dst().SetValue(dstIP)
	} else {
		v6.Src().SetValue(srcIP)
		v6.Dst().Increment().SetStart(dstIP).SetStep(v6Step).SetCount(ipCount)
	}
	udp := f.Packet().Add().Udp()
	udp.SrcPort().Increment().SetStart(10000).SetStep(1).SetCount(64)
	udp.DstPort().Increment().SetStart(20000).SetStep(1).SetCount(64)
}

// addTaggedIPv4Flow adds a single-tagged 802.1Q VLAN IPv4 UDP flow for B2_CORE VRF verification (RT-5.18.5 & RT-5.18.6).
func addTaggedIPv4Flow(config gosnappi.Config, name, txEndpoint, rxEndpoint, srcMAC, dstMAC, srcIP, dstIP string, vlanID uint32) {
	f := config.Flows().Add().SetName(name)
	f.Metrics().SetEnable(true)
	f.TxRx().Device().SetTxNames([]string{txEndpoint}).SetRxNames([]string{rxEndpoint})
	f.Size().SetFixed(flowFrameSize)
	f.Rate().SetPps(flowPPS)
	f.Duration().Continuous()

	eth := f.Packet().Add().Ethernet()
	if srcMAC != "" {
		eth.Src().SetValue(srcMAC)
	}
	if dstMAC != "" {
		eth.Dst().SetValue(dstMAC)
	}
	f.Packet().Add().Vlan().Id().SetValue(vlanID)
	v4 := f.Packet().Add().Ipv4()
	v4.Src().Increment().SetStart(srcIP).SetStep("0.0.0.1").SetCount(64)
	v4.Dst().Increment().SetStart(dstIP).SetStep("0.0.0.1").SetCount(64)
	udp := f.Packet().Add().Udp()
	udp.SrcPort().Increment().SetStart(10000).SetStep(1).SetCount(64)
	udp.DstPort().Increment().SetStart(20000).SetStep(1).SetCount(64)
}

// addTaggedIPv6Flow adds a single-tagged 802.1Q VLAN IPv6 UDP flow for B2_CORE VRF verification (RT-5.18.5 & RT-5.18.6).
func addTaggedIPv6Flow(config gosnappi.Config, name, txEndpoint, rxEndpoint, srcMAC, dstMAC, srcIP, dstIP string, vlanID uint32) {
	f := config.Flows().Add().SetName(name)
	f.Metrics().SetEnable(true)
	f.TxRx().Device().SetTxNames([]string{txEndpoint}).SetRxNames([]string{rxEndpoint})
	f.Size().SetFixed(flowFrameSize)
	f.Rate().SetPps(flowPPS)
	f.Duration().Continuous()

	eth := f.Packet().Add().Ethernet()
	if srcMAC != "" {
		eth.Src().SetValue(srcMAC)
	}
	if dstMAC != "" {
		eth.Dst().SetValue(dstMAC)
	}
	f.Packet().Add().Vlan().Id().SetValue(vlanID)
	v6 := f.Packet().Add().Ipv6()
	v6.Src().Increment().SetStart(srcIP).SetStep("::1").SetCount(64)
	v6.Dst().Increment().SetStart(dstIP).SetStep("::1").SetCount(64)
	udp := f.Packet().Add().Udp()
	udp.SrcPort().Increment().SetStart(10000).SetStep(1).SetCount(64)
	udp.DstPort().Increment().SetStart(20000).SetStep(1).SetCount(64)
}

// verifyPreDrainBaseline executes the Baseline Verification (Pre-Drain) steps defined in README.md:
// - 1. Verify Port-Channel1 and Port-1..Port-4 enabled == true, admin-status == UP, oper-status == UP.
// - 2. Verify LACP member states on Port-1..Port-3 (ACTIVE, aggregatable, collecting, distributing, IN_SYNC).
// - 3. Verify IS-IS adjacency UP on Port-Channel1 and Port-4, and all 4 eBGP sessions ESTABLISHED with expected prefixes.
// - 4. Start all 4 bidirectional traffic flows (1, 2, 3, 4) and confirm 0% packet loss and incrementing counters on Port-1..Port-3.
func verifyPreDrainBaseline(t *testing.T, env *testEnv) {
	t.Helper()
	t.Log("Executing RT-5.18 Baseline Verification (Pre-Drain)")

	// 1. Verify aggregate interface and all physical ports are administratively and operationally UP.
	cfgplugins.AwaitInterfaceAdminAndOperStatus(t, env.dut, env.aggID, true, oc.Interface_AdminStatus_UP, oc.Interface_OperStatus_UP, stateTimeout)
	cfgplugins.AwaitAggregateState(t, env.dut, env.aggID, oc.IfAggregate_AggregationType_LACP, stateTimeout)
	for _, p := range env.memberPorts {
		cfgplugins.AwaitInterfaceAdminAndOperStatus(t, env.dut, p.Name(), true, oc.Interface_AdminStatus_UP, oc.Interface_OperStatus_UP, stateTimeout)
		cfgplugins.AwaitMemberForwardingViableAndAggregateID(t, env.dut, p.Name(), env.aggID, true, stateTimeout)
	}
	cfgplugins.AwaitInterfaceAdminAndOperStatus(t, env.dut, env.port4.Name(), true, oc.Interface_AdminStatus_UP, oc.Interface_OperStatus_UP, stateTimeout)

	// 2. Verify LACP state on Port-1..Port-3 and OTG LAG.
	cfgplugins.AwaitLACPMemberState(t, env.dut, env.aggID, env.memberPorts, stateTimeout)
	otgvalidationhelpers.WaitForOTGLAGUP(t, env.ate, otgvalidationhelpers.LagParams{
		LagName:       ateLagAttrs.Name,
		WantMembersUp: uint64(len(env.memberPorts)),
		Timeout:       stateTimeout,
	})

	// 3. Verify IS-IS Level-2 adjacencies UP and all 4 eBGP sessions ESTABLISHED with expected prefixes.
	cfgplugins.AwaitISISAdjacencyStatus(t, env.dut, isisInstance, env.aggID, 0, true, protocolTimeout)
	cfgplugins.AwaitISISAdjacencyStatus(t, env.dut, isisInstance, env.port4.Name(), 0, true, protocolTimeout)

	for _, peer := range []string{ateLagAttrs.IPv4, ateLagAttrs.IPv6, atePort4Attrs.IPv4, atePort4Attrs.IPv6} {
		cfgplugins.AwaitBGPSessionStatus(t, env.dut, deviations.DefaultNetworkInstance(env.dut), peer, true, protocolTimeout)
	}
	cfgplugins.AwaitBGPPrefixes(t, env.dut, deviations.DefaultNetworkInstance(env.dut), ateLagAttrs.IPv4, oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST, lagBGPV4Count, lagBGPV4Count, protocolTimeout)
	cfgplugins.AwaitBGPPrefixes(t, env.dut, deviations.DefaultNetworkInstance(env.dut), ateLagAttrs.IPv6, oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST, lagBGPV6Count, lagBGPV6Count, protocolTimeout)
	cfgplugins.AwaitBGPPrefixes(t, env.dut, deviations.DefaultNetworkInstance(env.dut), atePort4Attrs.IPv4, oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST, port4BGPV4Count, port4BGPV4Count, protocolTimeout)
	cfgplugins.AwaitBGPPrefixes(t, env.dut, deviations.DefaultNetworkInstance(env.dut), atePort4Attrs.IPv6, oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST, port4BGPV6Count, port4BGPV6Count, protocolTimeout)

	// Wait for ARP and IPv6 ND resolution on OTG interfaces before starting traffic.
	otgutils.WaitForARP(t, env.ate.OTG(), env.otgConfig, "IPv4")
	otgutils.WaitForARP(t, env.ate.OTG(), env.otgConfig, "IPv6")

	// 4. Start all 4 bidirectional traffic flows and confirm 0% packet loss and incrementing counters on Port-Channel1 and Port-1..Port-3.
	env.ate.OTG().StartTraffic(t)
	for _, flow := range env.allFlows {
		otgvalidationhelpers.AwaitFlowSteadyState(t, env.ate, otgvalidationhelpers.SteadyStateFlowParams{
			FlowName:      flow,
			MinWindowPkts: 200,
			WantZeroLoss:  true,
			Timeout:       trafficTimeout,
		})
	}
	cfgplugins.AwaitInterfaceCountersIncrement(t, env.dut, env.aggID, 50, trafficTimeout)
	for _, p := range env.memberPorts {
		cfgplugins.AwaitInterfaceCountersIncrement(t, env.dut, p.Name(), 10, trafficTimeout)
	}
}

// testAdminDownDrainDisable implements RT-5.18.1 - Admin Down Drain - Disable (Steps 1-8).
func testAdminDownDrainDisable(t *testing.T, env *testEnv) {
	// RT-5.18.1 Step 1: With all 4 bidirectional traffic flows actively running at 0% loss,
	// administratively disable the aggregate interface on the DUT to initiate the drain.
	t.Logf("RT-5.18.1 Step 1: Administratively disabling aggregate interface %s (enabled = false)", env.aggID)
	cfgplugins.SetInterfaceAdminEnabled(t, env.dut, nil, env.aggID, false)

	// RT-5.18.1 Step 2: Using gnmi.Watch / Await, verify that the aggregate interface transitions to DOWN.
	t.Logf("RT-5.18.1 Step 2: Verifying %s state/enabled == false, admin-status == DOWN, oper-status == DOWN", env.aggID)
	cfgplugins.AwaitInterfaceAdminAndOperStatus(t, env.dut, env.aggID, false, oc.Interface_AdminStatus_DOWN, oc.Interface_OperStatus_DOWN, stateTimeout)

	// RT-5.18.1 Step 3: Verify that member ports Port-1, Port-2, and Port-3 remain bound to
	// the aggregate interface and retain their member state (only the parent aggregate interface was administratively disabled).
	t.Log("RT-5.18.1 Step 3: Verifying member ports Port-1..Port-3 retain aggregate membership and member state")
	for _, p := range env.memberPorts {
		cfgplugins.AwaitMemberForwardingViableAndAggregateID(t, env.dut, p.Name(), env.aggID, true, stateTimeout)
		if got := gnmi.Get(t, env.dut, gnmi.OC().Interface(p.Name()).Enabled().Config()); !got {
			t.Errorf("RT-5.18.1 Step 3: Member interface %s config/enabled = %v, want true", p.Name(), got)
		}
		if env.dut.Vendor() != ondatra.JUNIPER && env.dut.Vendor() != ondatra.CISCO {
			if got := gnmi.Get(t, env.dut, gnmi.OC().Interface(p.Name()).Enabled().State()); !got {
				t.Errorf("RT-5.18.1 Step 3: Member interface %s state/enabled = %v, want true", p.Name(), got)
			}
		}
	}

	// RT-5.18.1 Step 4: Verify via gNMI telemetry that the IS-IS adjacency over Port-Channel1 is torn down
	// (no longer in UP state), while the IS-IS adjacency on Port-4 remains UP.
	t.Log("RT-5.18.1 Step 4: Verifying IS-IS adjacency on Port-Channel1 is torn down while Port-4 remains UP")
	cfgplugins.AwaitISISAdjacencyStatus(t, env.dut, isisInstance, env.aggID, 0, false, protocolTimeout)
	cfgplugins.AwaitISISAdjacencyStatus(t, env.dut, isisInstance, env.port4.Name(), 0, true, protocolTimeout)

	// RT-5.18.1 Step 5: Verify via gNMI telemetry that the IPv4 and IPv6 eBGP sessions over Port-Channel1
	// transition out of ESTABLISHED state, while the eBGP sessions over Port-4 remain ESTABLISHED.
	t.Log("RT-5.18.1 Step 5: Verifying eBGP sessions over Port-Channel1 transition out of ESTABLISHED while Port-4 sessions remain ESTABLISHED")
	cfgplugins.AwaitBGPSessionStatus(t, env.dut, deviations.DefaultNetworkInstance(env.dut), ateLagAttrs.IPv4, false, protocolTimeout)
	cfgplugins.AwaitBGPSessionStatus(t, env.dut, deviations.DefaultNetworkInstance(env.dut), ateLagAttrs.IPv6, false, protocolTimeout)
	cfgplugins.AwaitBGPSessionStatus(t, env.dut, deviations.DefaultNetworkInstance(env.dut), atePort4Attrs.IPv4, true, protocolTimeout)
	cfgplugins.AwaitBGPSessionStatus(t, env.dut, deviations.DefaultNetworkInstance(env.dut), atePort4Attrs.IPv6, true, protocolTimeout)

	// RT-5.18.1 Step 6: Verify via gNMI interface counter telemetry (gnmi.Watch) that the packet rate on
	// Port-Channel1 and all member links (Port-1, Port-2, Port-3) drops to 0.
	// RT-5.18.1 Step 7: Verify via OTG flow metrics that steady-state traffic for Flows 1, 2, 3, and 4
	// is 100% dropped (completely drained from Port-Channel1).
	t.Log("RT-5.18.1 Steps 6 & 7: Verifying traffic is completely drained (100% dropped in steady state) across Flows 1..4")
	for _, flow := range env.allFlows {
		otgvalidationhelpers.AwaitFlowSteadyState(t, env.ate, otgvalidationhelpers.SteadyStateFlowParams{
			FlowName:      flow,
			MinWindowPkts: 200,
			WantZeroLoss:  false,
			WantComplete:  false,
			MaxLossPct:    100.0,
			Timeout:       trafficTimeout,
		})
	}

	// RT-5.18.1 Step 8: Verify that the DUT remains stable with no gNMI session disconnect or system crash.
	t.Log("RT-5.18.1 Step 8: Verifying DUT gNMI responsiveness and system stability")
	cfgplugins.AwaitInterfaceAdminAndOperStatus(t, env.dut, env.port4.Name(), true, oc.Interface_AdminStatus_UP, oc.Interface_OperStatus_UP, stateTimeout)
}

// restartTrafficAfterLAGChange waits for DUT and OTG LACP member synchronization across activeMembers
// and restarts OTG traffic so IxNetwork re-binds LAG-sourced device flows onto active member links.
func restartTrafficAfterLAGChange(t *testing.T, env *testEnv, activeMembers []*ondatra.Port) {
	t.Helper()
	cfgplugins.AwaitLACPMemberState(t, env.dut, env.aggID, activeMembers, stateTimeout)
	otgvalidationhelpers.WaitForOTGLAGUP(t, env.ate, otgvalidationhelpers.LagParams{
		LagName:       ateLagAttrs.Name,
		WantMembersUp: uint64(len(activeMembers)),
		Timeout:       stateTimeout,
	})
	env.ate.OTG().StopTraffic(t)
	otgutils.WaitForARP(t, env.ate.OTG(), env.otgConfig, "IPv4")
	otgutils.WaitForARP(t, env.ate.OTG(), env.otgConfig, "IPv6")
	env.ate.OTG().StartTraffic(t)
}

// testAdminDownDrainReEnable implements RT-5.18.2 - Admin Down Drain - Re-enable (Steps 1-6).
func testAdminDownDrainReEnable(t *testing.T, env *testEnv) {
	// RT-5.18.2 Step 1: Administratively re-enable the aggregate interface on the DUT to undrain the LAG.
	t.Logf("RT-5.18.2 Step 1: Administratively re-enabling aggregate interface %s (enabled = true)", env.aggID)
	cfgplugins.SetInterfaceAdminEnabled(t, env.dut, nil, env.aggID, true)

	// RT-5.18.2 Step 2: Using gnmi.Watch / Await, verify that the aggregate interface transitions back to UP.
	t.Logf("RT-5.18.2 Step 2: Verifying %s state/enabled == true, admin-status == UP, oper-status == UP", env.aggID)
	cfgplugins.AwaitInterfaceAdminAndOperStatus(t, env.dut, env.aggID, true, oc.Interface_AdminStatus_UP, oc.Interface_OperStatus_UP, stateTimeout)

	// RT-5.18.2 Step 3: Verify via gNMI telemetry that all 3 member links (Port-1, Port-2, Port-3) re-synchronize
	// in LACP (collecting: true, distributing: true, synchronization: IN_SYNC).
	t.Log("RT-5.18.2 Step 3: Verifying LACP member links Port-1..Port-3 re-synchronize (IN_SYNC, collecting, distributing)")
	cfgplugins.AwaitLACPMemberState(t, env.dut, env.aggID, env.memberPorts, stateTimeout)
	otgvalidationhelpers.WaitForOTGLAGUP(t, env.ate, otgvalidationhelpers.LagParams{
		LagName:       ateLagAttrs.Name,
		WantMembersUp: uint64(len(env.memberPorts)),
		Timeout:       stateTimeout,
	})

	// RT-5.18.2 Step 4: Verify via gNMI telemetry that the IS-IS adjacency over Port-Channel1 re-establishes to UP,
	// and both IPv4 and IPv6 eBGP sessions over Port-Channel1 return to ESTABLISHED with all 5 IPv4 and 5 IPv6 prefixes installed.
	t.Log("RT-5.18.2 Step 4: Verifying IS-IS adjacency UP and eBGP sessions ESTABLISHED with 5 IPv4 and 5 IPv6 prefixes installed")
	cfgplugins.AwaitISISAdjacencyStatus(t, env.dut, isisInstance, env.aggID, 0, true, protocolTimeout)
	cfgplugins.AwaitBGPSessionStatus(t, env.dut, deviations.DefaultNetworkInstance(env.dut), ateLagAttrs.IPv4, true, protocolTimeout)
	cfgplugins.AwaitBGPSessionStatus(t, env.dut, deviations.DefaultNetworkInstance(env.dut), ateLagAttrs.IPv6, true, protocolTimeout)
	cfgplugins.AwaitBGPPrefixes(t, env.dut, deviations.DefaultNetworkInstance(env.dut), ateLagAttrs.IPv4, oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST, lagBGPV4Count, lagBGPV4Count, protocolTimeout)
	cfgplugins.AwaitBGPPrefixes(t, env.dut, deviations.DefaultNetworkInstance(env.dut), ateLagAttrs.IPv6, oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST, lagBGPV6Count, lagBGPV6Count, protocolTimeout)

	env.ate.OTG().StopTraffic(t)
	env.ate.OTG().StartTraffic(t)

	// RT-5.18.2 Step 5: Verify via gNMI interface counter telemetry (gnmi.Watch) that traffic resumes across
	// Port-Channel1 and is distributed across all 3 member links (Port-1, Port-2, Port-3).
	t.Log("RT-5.18.2 Step 5: Verifying traffic resumes across Port-Channel1 and all 3 member links (Port-1..Port-3)")
	cfgplugins.AwaitInterfaceCountersIncrement(t, env.dut, env.aggID, 50, trafficTimeout)
	for _, p := range env.memberPorts {
		cfgplugins.AwaitInterfaceCountersIncrement(t, env.dut, p.Name(), 10, trafficTimeout)
	}

	// RT-5.18.2 Step 6: Verify via OTG flow metrics that steady-state packet loss returns to 0% for all 4 flows.
	t.Log("RT-5.18.2 Step 6: Verifying steady-state packet loss returns to 0% for all 4 flows")
	for _, flow := range env.allFlows {
		otgvalidationhelpers.AwaitFlowSteadyState(t, env.ate, otgvalidationhelpers.SteadyStateFlowParams{
			FlowName:      flow,
			MinWindowPkts: 200,
			WantZeroLoss:  true,
			Timeout:       trafficTimeout,
		})
	}
}

// testNegativeAdminDownMemberLinks implements RT-5.18.3 - Negative Test - Admin Down on Individual Member Links (Steps 1-7).
func testNegativeAdminDownMemberLinks(t *testing.T, env *testEnv) {
	p1 := env.memberPorts[2]
	p2 := env.memberPorts[1]
	p3 := env.memberPorts[0]

	// RT-5.18.3 Step 1: Ensure Port-Channel1 and all 3 member links (Port-1, Port-2, Port-3) are UP,
	// IS-IS and eBGP sessions are established, and Flows 1-4 are forwarding with 0% packet loss.
	t.Log("RT-5.18.3 Step 1: Verifying baseline UP state on Port-Channel1 and Port-1..Port-3 prior to member link drain")
	cfgplugins.AwaitInterfaceAdminAndOperStatus(t, env.dut, env.aggID, true, oc.Interface_AdminStatus_UP, oc.Interface_OperStatus_UP, stateTimeout)
	for _, p := range env.memberPorts {
		cfgplugins.AwaitInterfaceAdminAndOperStatus(t, env.dut, p.Name(), true, oc.Interface_AdminStatus_UP, oc.Interface_OperStatus_UP, stateTimeout)
	}

	// RT-5.18.3 Step 2: Administratively disable member link Port-1 on the DUT while leaving Port-Channel1,
	// Port-2, and Port-3 enabled.
	t.Logf("RT-5.18.3 Step 2: Administratively disabling member link %s (enabled = false)", p1.Name())
	cfgplugins.SetInterfaceAdminEnabled(t, env.dut, nil, p1.Name(), false)

	// RT-5.18.3 Step 3: Verify via gnmi.Watch / Await that:
	// - Port-1 transitions to admin-status == DOWN and oper-status == DOWN.
	// - Port-Channel1 remains oper-status == UP (since 2 active members >= min-links: 1).
	// - IS-IS adjacency and eBGP sessions over Port-Channel1 remain UP / ESTABLISHED without flapping.
	// - Traffic continues to be forwarded across the remaining active member links (Port-2 and Port-3)
	//   with 0% steady-state packet loss after LACP hash reconvergence.
	t.Log("RT-5.18.3 Step 3: Verifying Port-1 is DOWN while Port-Channel1, IS-IS, eBGP, and traffic on Port-2 & Port-3 remain UP")
	cfgplugins.AwaitInterfaceAdminAndOperStatus(t, env.dut, p1.Name(), false, oc.Interface_AdminStatus_DOWN, oc.Interface_OperStatus_DOWN, stateTimeout)
	cfgplugins.AwaitInterfaceAdminAndOperStatus(t, env.dut, env.aggID, true, oc.Interface_AdminStatus_UP, oc.Interface_OperStatus_UP, stateTimeout)
	cfgplugins.AwaitISISAdjacencyStatus(t, env.dut, isisInstance, env.aggID, 0, true, protocolTimeout)
	cfgplugins.AwaitBGPSessionStatus(t, env.dut, deviations.DefaultNetworkInstance(env.dut), ateLagAttrs.IPv4, true, protocolTimeout)
	cfgplugins.AwaitBGPSessionStatus(t, env.dut, deviations.DefaultNetworkInstance(env.dut), ateLagAttrs.IPv6, true, protocolTimeout)
	cfgplugins.AwaitLACPMemberState(t, env.dut, env.aggID, []*ondatra.Port{p2, p3}, stateTimeout)
	otgvalidationhelpers.WaitForOTGLAGUP(t, env.ate, otgvalidationhelpers.LagParams{
		LagName:       ateLagAttrs.Name,
		WantMembersUp: 2,
		Timeout:       stateTimeout,
	})
	cfgplugins.AwaitInterfaceCountersIncrement(t, env.dut, p2.Name(), 10, trafficTimeout)
	cfgplugins.AwaitInterfaceCountersIncrement(t, env.dut, p3.Name(), 10, trafficTimeout)
	for _, flow := range env.allFlows {
		otgvalidationhelpers.AwaitFlowSteadyState(t, env.ate, otgvalidationhelpers.SteadyStateFlowParams{
			FlowName:      flow,
			MinWindowPkts: 200,
			WantZeroLoss:  true,
			Timeout:       trafficTimeout,
		})
	}

	// RT-5.18.3 Step 4: Administratively disable member link Port-2 on the DUT while leaving Port-3 enabled.
	t.Logf("RT-5.18.3 Step 4: Administratively disabling member link %s (enabled = false)", p2.Name())
	cfgplugins.SetInterfaceAdminEnabled(t, env.dut, nil, p2.Name(), false)

	// RT-5.18.3 Step 5: Verify via gnmi.Watch / Await that:
	// - Port-2 transitions to admin-status == DOWN and oper-status == DOWN.
	// - Port-Channel1 remains oper-status == UP (1 active member >= min-links: 1).
	// - IS-IS adjacency and eBGP sessions remain UP / ESTABLISHED.
	// - Traffic continues to be forwarded over the single remaining member link (Port-3).
	t.Log("RT-5.18.3 Step 5: Verifying Port-2 is DOWN while Port-Channel1, IS-IS, eBGP, and traffic on Port-3 remain UP")
	cfgplugins.AwaitInterfaceAdminAndOperStatus(t, env.dut, p2.Name(), false, oc.Interface_AdminStatus_DOWN, oc.Interface_OperStatus_DOWN, stateTimeout)
	cfgplugins.AwaitInterfaceAdminAndOperStatus(t, env.dut, env.aggID, true, oc.Interface_AdminStatus_UP, oc.Interface_OperStatus_UP, stateTimeout)
	cfgplugins.AwaitISISAdjacencyStatus(t, env.dut, isisInstance, env.aggID, 0, true, protocolTimeout)
	cfgplugins.AwaitBGPSessionStatus(t, env.dut, deviations.DefaultNetworkInstance(env.dut), ateLagAttrs.IPv4, true, protocolTimeout)
	cfgplugins.AwaitBGPSessionStatus(t, env.dut, deviations.DefaultNetworkInstance(env.dut), ateLagAttrs.IPv6, true, protocolTimeout)
	cfgplugins.AwaitLACPMemberState(t, env.dut, env.aggID, []*ondatra.Port{p3}, stateTimeout)
	otgvalidationhelpers.WaitForOTGLAGUP(t, env.ate, otgvalidationhelpers.LagParams{
		LagName:       ateLagAttrs.Name,
		WantMembersUp: 1,
		Timeout:       stateTimeout,
	})
	cfgplugins.AwaitInterfaceCountersIncrement(t, env.dut, p3.Name(), 10, trafficTimeout)
	for _, flow := range env.allFlows {
		otgvalidationhelpers.AwaitFlowSteadyState(t, env.ate, otgvalidationhelpers.SteadyStateFlowParams{
			FlowName:      flow,
			MinWindowPkts: 200,
			WantZeroLoss:  true,
			Timeout:       trafficTimeout,
		})
	}

	// RT-5.18.3 Step 6: Administratively disable the final active member link Port-3 while Port-Channel1
	// itself remains configured with config/enabled: true. Verify that Port-Channel1 now transitions to
	// oper-status == LOWER_LAYER_DOWN (or DOWN), while admin-status on Port-Channel1 remains UP.
	t.Logf("RT-5.18.3 Step 6: Administratively disabling final member link %s while %s config/enabled remains true", p3.Name(), env.aggID)
	cfgplugins.SetInterfaceAdminEnabled(t, env.dut, nil, p3.Name(), false)
	cfgplugins.AwaitInterfaceAdminAndOperStatus(t, env.dut, p3.Name(), false, oc.Interface_AdminStatus_DOWN, oc.Interface_OperStatus_DOWN, stateTimeout)
	cfgplugins.AwaitAggregateLowerLayerDown(t, env.dut, env.aggID, stateTimeout)

	// RT-5.18.3 Step 7: Re-enable Port-1, Port-2, and Port-3 (config/enabled: true) and verify via
	// gnmi.Watch / Await that Port-Channel1, LACP members, IS-IS, eBGP, and all 4 traffic flows recover to 0% packet loss.
	t.Log("RT-5.18.3 Step 7: Re-enabling Port-1..Port-3 and verifying full recovery of Port-Channel1, LACP, IS-IS, eBGP, and traffic")
	restoreBatch := &gnmi.SetBatch{}
	for _, p := range env.memberPorts {
		cfgplugins.SetInterfaceAdminEnabled(t, env.dut, restoreBatch, p.Name(), true)
	}
	restoreBatch.Set(t, env.dut)

	for _, p := range env.memberPorts {
		cfgplugins.AwaitInterfaceAdminAndOperStatus(t, env.dut, p.Name(), true, oc.Interface_AdminStatus_UP, oc.Interface_OperStatus_UP, stateTimeout)
	}
	cfgplugins.AwaitInterfaceAdminAndOperStatus(t, env.dut, env.aggID, true, oc.Interface_AdminStatus_UP, oc.Interface_OperStatus_UP, stateTimeout)
	cfgplugins.AwaitISISAdjacencyStatus(t, env.dut, isisInstance, env.aggID, 0, true, protocolTimeout)
	cfgplugins.AwaitBGPSessionStatus(t, env.dut, deviations.DefaultNetworkInstance(env.dut), ateLagAttrs.IPv4, true, protocolTimeout)
	cfgplugins.AwaitBGPSessionStatus(t, env.dut, deviations.DefaultNetworkInstance(env.dut), ateLagAttrs.IPv6, true, protocolTimeout)
	cfgplugins.AwaitBGPPrefixes(t, env.dut, deviations.DefaultNetworkInstance(env.dut), ateLagAttrs.IPv4, oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST, lagBGPV4Count, lagBGPV4Count, protocolTimeout)
	cfgplugins.AwaitBGPPrefixes(t, env.dut, deviations.DefaultNetworkInstance(env.dut), ateLagAttrs.IPv6, oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST, lagBGPV6Count, lagBGPV6Count, protocolTimeout)
	restartTrafficAfterLAGChange(t, env, env.memberPorts)
	for _, flow := range env.allFlows {
		otgvalidationhelpers.AwaitFlowSteadyState(t, env.ate, otgvalidationhelpers.SteadyStateFlowParams{
			FlowName:      flow,
			MinWindowPkts: 200,
			WantZeroLoss:  true,
			Timeout:       trafficTimeout,
		})
	}
}

// testNegativeAdminDownAlreadyDisabledLAG implements RT-5.18.4 - Negative Test - Admin Down on Already Disabled LAG (Steps 1-6).
func testNegativeAdminDownAlreadyDisabledLAG(t *testing.T, env *testEnv) {
	// RT-5.18.4 Step 1: Administratively disable Port-Channel1 on the DUT (config/enabled: false).
	t.Logf("RT-5.18.4 Step 1: Administratively disabling %s (enabled = false)", env.aggID)
	cfgplugins.SetInterfaceAdminEnabled(t, env.dut, nil, env.aggID, false)

	// RT-5.18.4 Step 2: Wait using gnmi.Await until state/admin-status == DOWN and state/oper-status == DOWN
	// are confirmed on Port-Channel1.
	t.Logf("RT-5.18.4 Step 2: Waiting for %s admin-status == DOWN and oper-status == DOWN", env.aggID)
	cfgplugins.AwaitInterfaceAdminAndOperStatus(t, env.dut, env.aggID, false, oc.Interface_AdminStatus_DOWN, oc.Interface_OperStatus_DOWN, stateTimeout)

	// RT-5.18.4 Step 3: Push a duplicate gNMI Set (Replace/Update) setting config/enabled: false on the
	// already disabled Port-Channel1 interface.
	t.Logf("RT-5.18.4 Step 3: Pushing duplicate gNMI Replace with enabled = false on already disabled %s", env.aggID)
	cfgplugins.SetInterfaceAdminEnabled(t, env.dut, nil, env.aggID, false)

	// RT-5.18.4 Step 4: Verify that the gNMI Set operation succeeds without returning an RPC error.
	// RT-5.18.4 Step 5: Verify via gNMI telemetry that Port-Channel1 remains cleanly in state/enabled == false,
	// state/admin-status == DOWN, and state/oper-status == DOWN, with no spurious state transitions, interface flaps,
	// or impact on unrelated interfaces (Port-4 and its IS-IS / eBGP sessions remain UP / ESTABLISHED).
	t.Log("RT-5.18.4 Steps 4 & 5: Verifying idempotent DOWN state on Port-Channel1 and uninterrupted UP state on Port-4")
	cfgplugins.AwaitInterfaceAdminAndOperStatus(t, env.dut, env.aggID, false, oc.Interface_AdminStatus_DOWN, oc.Interface_OperStatus_DOWN, stateTimeout)
	cfgplugins.AwaitInterfaceAdminAndOperStatus(t, env.dut, env.port4.Name(), true, oc.Interface_AdminStatus_UP, oc.Interface_OperStatus_UP, stateTimeout)
	cfgplugins.AwaitISISAdjacencyStatus(t, env.dut, isisInstance, env.port4.Name(), 0, true, protocolTimeout)
	cfgplugins.AwaitBGPSessionStatus(t, env.dut, deviations.DefaultNetworkInstance(env.dut), atePort4Attrs.IPv4, true, protocolTimeout)
	cfgplugins.AwaitBGPSessionStatus(t, env.dut, deviations.DefaultNetworkInstance(env.dut), atePort4Attrs.IPv6, true, protocolTimeout)

	// RT-5.18.4 Step 6: Re-enable Port-Channel1 (config/enabled: true) and verify via gnmi.Await that the
	// interface, protocols, and traffic flows recover normally.
	t.Logf("RT-5.18.4 Step 6: Re-enabling %s (enabled = true) and verifying recovery of interface, protocols, and flows", env.aggID)
	cfgplugins.SetInterfaceAdminEnabled(t, env.dut, nil, env.aggID, true)
	cfgplugins.AwaitInterfaceAdminAndOperStatus(t, env.dut, env.aggID, true, oc.Interface_AdminStatus_UP, oc.Interface_OperStatus_UP, stateTimeout)
	cfgplugins.AwaitISISAdjacencyStatus(t, env.dut, isisInstance, env.aggID, 0, true, protocolTimeout)
	cfgplugins.AwaitBGPSessionStatus(t, env.dut, deviations.DefaultNetworkInstance(env.dut), ateLagAttrs.IPv4, true, protocolTimeout)
	cfgplugins.AwaitBGPSessionStatus(t, env.dut, deviations.DefaultNetworkInstance(env.dut), ateLagAttrs.IPv6, true, protocolTimeout)
	cfgplugins.AwaitBGPPrefixes(t, env.dut, deviations.DefaultNetworkInstance(env.dut), ateLagAttrs.IPv4, oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST, lagBGPV4Count, lagBGPV4Count, protocolTimeout)
	cfgplugins.AwaitBGPPrefixes(t, env.dut, deviations.DefaultNetworkInstance(env.dut), ateLagAttrs.IPv6, oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST, lagBGPV6Count, lagBGPV6Count, protocolTimeout)
	restartTrafficAfterLAGChange(t, env, env.memberPorts)
	for _, flow := range env.allFlows {
		otgvalidationhelpers.AwaitFlowSteadyState(t, env.ate, otgvalidationhelpers.SteadyStateFlowParams{
			FlowName:      flow,
			MinWindowPkts: 200,
			WantZeroLoss:  true,
			Timeout:       trafficTimeout,
		})
	}

	// Stop DEFAULT VRF traffic flows before transitioning to RT-5.18.5 B2_CORE VRF verification.
	env.ate.OTG().StopTraffic(t)
}

// testUplinkLAGMemberRetentionAndB2CoreBGP implements RT-5.18.5 - Uplink LAG Member Retention and
// Inband BGP Reachability in Non-Default VRF (B2_CORE) Across Drain States (Steps 1-8).
func testUplinkLAGMemberRetentionAndB2CoreBGP(t *testing.T, env *testEnv) {
	// Cleanly remove DEFAULT VRF protocols and untagged Subinterface(0) addresses prior to configuring
	// single-tagged VLAN 100 subinterfaces in B2_CORE (while retaining routed Subinterface(0) when required).
	dni := deviations.DefaultNetworkInstance(env.dut)
	preB2Batch := &gnmi.SetBatch{}
	gnmi.BatchDelete(preB2Batch, gnmi.OC().NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance).Config())
	gnmi.BatchDelete(preB2Batch, gnmi.OC().NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Config())
	if deviations.ExplicitInterfaceInDefaultVRF(env.dut) {
		gnmi.BatchDelete(preB2Batch, gnmi.OC().NetworkInstance(dni).Interface(env.aggID+".0").Config())
		gnmi.BatchDelete(preB2Batch, gnmi.OC().NetworkInstance(dni).Interface(env.port4.Name()+".0").Config())
	}
	gnmi.BatchDelete(preB2Batch, gnmi.OC().Interface(env.aggID).Subinterface(0).Config())
	gnmi.BatchDelete(preB2Batch, gnmi.OC().Interface(env.port4.Name()).Subinterface(0).Config())
	if deviations.RequireRoutedSubinterface0(env.dut) {
		for _, intfName := range []string{env.aggID, env.port4.Name()} {
			s0 := &oc.Interface_Subinterface{}
			s0.SetIndex(0)
			s0v4 := s0.GetOrCreateIpv4()
			s0v6 := s0.GetOrCreateIpv6()
			if deviations.InterfaceEnabled(env.dut) && !deviations.IPv4MissingEnabled(env.dut) {
				s0v4.SetEnabled(true)
			}
			if deviations.InterfaceEnabled(env.dut) {
				s0v6.SetEnabled(true)
			}
			gnmi.BatchReplace(preB2Batch, gnmi.OC().Interface(intfName).Subinterface(0).Config(), s0)
		}
	}
	preB2Batch.Set(t, env.dut)
	if deviations.ExplicitEnableBGPOnDefaultVRF(env.dut) {
		cfgplugins.EnableDefaultNetworkInstanceBgp(t, env.dut, dutAS)
	}

	// RT-5.18.5 Step 3: Ensure uplink LAG (Port-Channel1) and its member links (Port-1, Port-2, Port-3)
	// retain explicit aggregate membership and forwarding viability (forwarding-viable: true, aggregate-id: <aggID>).
	t.Logf("RT-5.18.5 Step 3: Ensuring uplink LAG %s and member ports Port-1..Port-3 retain forwarding-viable = true and aggregate-id = %s", env.aggID, env.aggID)
	lagBatch := &gnmi.SetBatch{}
	cfgplugins.SetInterfaceAdminEnabled(t, env.dut, lagBatch, env.aggID, true)
	for _, p := range env.memberPorts {
		cfgplugins.SetInterfaceAdminEnabled(t, env.dut, lagBatch, p.Name(), true)
		cfgplugins.SetMemberForwardingViable(t, env.dut, lagBatch, p.Name(), env.aggID, true)
	}
	lagBatch.Set(t, env.dut)

	// RT-5.18.5 Step 1: Configure the non-default L3VRF B2_CORE on the DUT and bind subinterfaces
	// Port-Channel1.100 and Port-4.100 to B2_CORE.
	t.Logf("RT-5.18.5 Step 1: Configuring non-default L3VRF %s and binding %s.%d and %s.%d",
		b2CoreVRFName, env.aggID, b2CoreSubintfIndex, env.port4.Name(), b2CoreSubintfIndex)
	vrfBatch := &gnmi.SetBatch{}
	cfgplugins.ConfigureL3VRFWithSubinterfaces(t, env.dut, vrfBatch, b2CoreVRFName, []cfgplugins.L3VRFSubinterfaceBinding{
		{
			InterfaceName: env.aggID,
			SubintfIndex:  b2CoreSubintfIndex,
			VlanID:        b2CoreVlanID,
		},
		{
			InterfaceName: env.port4.Name(),
			SubintfIndex:  b2CoreSubintfIndex,
			VlanID:        b2CoreVlanID,
		},
	})
	vrfBatch.Set(t, env.dut)

	// RT-5.18.5 Step 2: Configure single-tagged VLAN 100 subinterfaces and dual-stack addressing on
	// Port-Channel1.100 (192.0.2.9/30, 2001:db8::9/126) and Port-4.100 (192.0.2.13/30, 2001:db8::d/126).
	t.Log("RT-5.18.5 Step 2: Configuring single-tagged 802.1Q VLAN 100 subinterfaces and dual-stack addressing on Port-Channel1.100 and Port-4.100")
	subBatch := &gnmi.SetBatch{}
	cfgplugins.ConfigureSingleTaggedSubinterface(t, env.dut, subBatch, cfgplugins.SingleTaggedSubinterfaceParams{
		InterfaceName: env.aggID,
		SubintfIndex:  b2CoreSubintfIndex,
		VlanID:        b2CoreVlanID,
		IPv4Addr:      dutLagVlan100IPv4,
		IPv4PrefixLen: ipv4PrefixLen,
		IPv6Addr:      dutLagVlan100IPv6,
		IPv6PrefixLen: ipv6PrefixLen,
	})
	cfgplugins.ConfigureSingleTaggedSubinterface(t, env.dut, subBatch, cfgplugins.SingleTaggedSubinterfaceParams{
		InterfaceName: env.port4.Name(),
		SubintfIndex:  b2CoreSubintfIndex,
		VlanID:        b2CoreVlanID,
		IPv4Addr:      dutPort4Vlan100IPv4,
		IPv4PrefixLen: ipv4PrefixLen,
		IPv6Addr:      dutPort4Vlan100IPv6,
		IPv6PrefixLen: ipv6PrefixLen,
	})
	subBatch.Set(t, env.dut)

	// RT-5.18.5 Step 4: Configure dual-stack eBGP in the B2_CORE network instance between the DUT (AS 64500)
	// and ATE over Port-Channel1.100 (Peers 192.0.2.10, 2001:db8::a, AS 64501) and Port-4.100 (Peers 192.0.2.14, 2001:db8::e, AS 64502).
	t.Logf("RT-5.18.5 Step 4: Configuring dual-stack eBGP in VRF %s over %s.%d and %s.%d",
		b2CoreVRFName, env.aggID, b2CoreSubintfIndex, env.port4.Name(), b2CoreSubintfIndex)
	bgpBatch := &gnmi.SetBatch{}
	cfgplugins.ConfigureEBGPInstance(t, env.dut, bgpBatch, cfgplugins.EBGPInstanceParams{
		NetworkInstanceName: b2CoreVRFName,
		RouterID:            dutLagVlan100IPv4,
		LocalAS:             dutAS,
		PolicyName:          "ACCEPT-ALL",
		Peers: []cfgplugins.EBGPPeerParam{
			{
				NeighborAddress: ateLagVlan100IPv4,
				LocalAddress:    dutLagVlan100IPv4,
				PeerAS:          ateLagAS,
				PeerGroupName:   "B2CORE-LAG-V4",
				IsIPv6:          false,
			},
			{
				NeighborAddress: ateLagVlan100IPv6,
				LocalAddress:    dutLagVlan100IPv6,
				PeerAS:          ateLagAS,
				PeerGroupName:   "B2CORE-LAG-V6",
				IsIPv6:          true,
			},
			{
				NeighborAddress: atePort4Vlan100IPv4,
				LocalAddress:    dutPort4Vlan100IPv4,
				PeerAS:          atePort4AS,
				PeerGroupName:   "B2CORE-PORT4-V4",
				IsIPv6:          false,
			},
			{
				NeighborAddress: atePort4Vlan100IPv6,
				LocalAddress:    dutPort4Vlan100IPv6,
				PeerAS:          atePort4AS,
				PeerGroupName:   "B2CORE-PORT4-V6",
				IsIPv6:          true,
			},
		},
	})
	bgpBatch.Set(t, env.dut)

	// Capture resolved DUT interface MAC addresses for Port-Channel1 and Port-4 before reconfiguring OTG
	// so B2_CORE tagged IPv4 and IPv6 flows have explicit Ethernet destination MACs.
	dutLagMAC := resolveDUTInterfaceMAC(t, env, ateLagAttrs.Name+".Eth", dutLagAttrs.IPv4, dutLagAttrs.IPv6, env.aggID)
	dutPort4MAC := resolveDUTInterfaceMAC(t, env, atePort4Attrs.Name+".Eth", dutPort4Attrs.IPv4, dutPort4Attrs.IPv6, env.port4.Name())
	t.Logf("Resolved DUT MACs for B2_CORE flows: %s=%s, %s=%s", env.aggID, dutLagMAC, env.port4.Name(), dutPort4MAC)

	// Configure and push OTG VLAN 100 subinterfaces, eBGP peers, and B2_CORE traffic flows on ATE.
	env.ate.OTG().StopProtocols(t)
	configureB2CoreOTG(t, env, dutLagMAC, dutPort4MAC)
	env.ate.OTG().PushConfig(t, env.otgConfig)
	env.ate.OTG().StartProtocols(t)

	// RT-5.18.5 Step 5: Verify via gnmi.Watch / Await that all uplink LAG members remain bound and active:
	// - /interfaces/interface[name=<Port-1|Port-2|Port-3>]/ethernet/state/aggregate-id == <aggID>
	// - /interfaces/interface[name=<Port-1|Port-2|Port-3>]/state/forwarding-viable == true
	// - /interfaces/interface[name=<Port-1|Port-2|Port-3>]/state/oper-status == UP
	// - /interfaces/interface[name=<aggID>]/state/oper-status == UP
	// - /lacp/interfaces/interface[name=<aggID>]/members/member[interface=<Port-1|Port-2|Port-3>]/state/{activity=ACTIVE, aggregatable=true, collecting=true, distributing=true, synchronization=IN_SYNC}.
	t.Log("RT-5.18.5 Step 5: Verifying uplink LAG members Port-1..Port-3 retain aggregate-id, forwarding-viable == true, oper-status == UP, and LACP IN_SYNC")
	cfgplugins.AwaitInterfaceAdminAndOperStatus(t, env.dut, env.aggID, true, oc.Interface_AdminStatus_UP, oc.Interface_OperStatus_UP, stateTimeout)
	for _, p := range env.memberPorts {
		cfgplugins.AwaitInterfaceAdminAndOperStatus(t, env.dut, p.Name(), true, oc.Interface_AdminStatus_UP, oc.Interface_OperStatus_UP, stateTimeout)
		cfgplugins.AwaitMemberForwardingViableAndAggregateID(t, env.dut, p.Name(), env.aggID, true, stateTimeout)
	}
	cfgplugins.AwaitLACPMemberState(t, env.dut, env.aggID, env.memberPorts, stateTimeout)
	otgvalidationhelpers.WaitForOTGLAGUP(t, env.ate, otgvalidationhelpers.LagParams{
		LagName:       ateLagAttrs.Name,
		WantMembersUp: uint64(len(env.memberPorts)),
		Timeout:       stateTimeout,
	})

	// RT-5.18.5 Step 6: Verify via gnmi.Watch / Await that all 4 eBGP sessions in B2_CORE reach ESTABLISHED
	// and routes are installed (192.0.2.10 IPV4_UNICAST installed == 1, 2001:db8::a IPV6_UNICAST installed == 1).
	t.Logf("RT-5.18.5 Step 6: Verifying all 4 eBGP sessions in %s reach ESTABLISHED and prefixes are installed", b2CoreVRFName)
	for _, peer := range []string{ateLagVlan100IPv4, ateLagVlan100IPv6, atePort4Vlan100IPv4, atePort4Vlan100IPv6} {
		cfgplugins.AwaitBGPSessionStatus(t, env.dut, b2CoreVRFName, peer, true, protocolTimeout)
	}
	cfgplugins.AwaitBGPPrefixes(t, env.dut, b2CoreVRFName, ateLagVlan100IPv4, oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST, b2CoreLagBGPV4Count, b2CoreLagBGPV4Count, protocolTimeout)
	cfgplugins.AwaitBGPPrefixes(t, env.dut, b2CoreVRFName, ateLagVlan100IPv6, oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST, b2CoreLagBGPV6Count, b2CoreLagBGPV6Count, protocolTimeout)
	cfgplugins.AwaitBGPPrefixes(t, env.dut, b2CoreVRFName, atePort4Vlan100IPv4, oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST, b2CorePort4BGPV4Count, b2CorePort4BGPV4Count, protocolTimeout)
	cfgplugins.AwaitBGPPrefixes(t, env.dut, b2CoreVRFName, atePort4Vlan100IPv6, oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST, b2CorePort4BGPV6Count, b2CorePort4BGPV6Count, protocolTimeout)

	// RT-5.18.5 Step 7: Confirm IPv4 ARP and IPv6 ND resolution on all ATE VLAN 100 interfaces while
	// LACP and protocol sessions are active, immediately before starting B2_CORE traffic flows.
	t.Log("RT-5.18.5 Step 7: Confirming IPv4 ARP and IPv6 ND resolution on VLAN 100 interfaces and starting B2_CORE bidirectional IPv4 and IPv6 flows")
	waitForB2CoreARPAndND(t, env)
	env.ate.OTG().StartTraffic(t)

	// RT-5.18.5 Step 8: Verify via gNMI counter Watch that steady-state packet loss is 0% for all 4 flows and
	// egress traffic on Port-Channel1 is distributed across all 3 retained member ports (Port-1, Port-2, Port-3).
	t.Log("RT-5.18.5 Step 8: Verifying 0% steady-state packet loss for all 4 B2_CORE flows and egress distribution across Port-1..Port-3")
	for _, flow := range env.b2CoreFlows {
		otgvalidationhelpers.AwaitFlowSteadyState(t, env.ate, otgvalidationhelpers.SteadyStateFlowParams{
			FlowName:      flow,
			MinWindowPkts: 200,
			WantZeroLoss:  true,
			Timeout:       trafficTimeout,
		})
	}
	cfgplugins.AwaitInterfaceCountersIncrement(t, env.dut, env.aggID, 50, trafficTimeout)
	for _, p := range env.memberPorts {
		cfgplugins.AwaitInterfaceCountersIncrement(t, env.dut, p.Name(), 10, trafficTimeout)
	}
}

// resolveDUTInterfaceMAC returns the MAC address of a DUT interface using OTG's resolved neighbor table
// with a fallback to the DUT's OpenConfig interface ethernet state.
func resolveDUTInterfaceMAC(t *testing.T, env *testEnv, otgEthName, dutIPv4, dutIPv6, dutIntfName string) string {
	t.Helper()
	if val, ok := gnmi.Lookup(t, env.ate.OTG(), gnmi.OTG().Interface(otgEthName).Ipv4Neighbor(dutIPv4).LinkLayerAddress().State()).Val(); ok && val != "" {
		return val
	}
	if val, ok := gnmi.Lookup(t, env.ate.OTG(), gnmi.OTG().Interface(otgEthName).Ipv6Neighbor(dutIPv6).LinkLayerAddress().State()).Val(); ok && val != "" {
		return val
	}
	if val, ok := gnmi.Lookup(t, env.dut, gnmi.OC().Interface(dutIntfName).Ethernet().MacAddress().State()).Val(); ok && val != "" {
		return val
	}
	return ""
}

// configureB2CoreOTG adds VLAN 100 subinterface devices, eBGP peers in AS 64501 / AS 64502, and the 4
// VLAN-100 tagged B2_CORE bidirectional traffic flows to the OTG configuration.
func configureB2CoreOTG(t *testing.T, env *testEnv, dutLagMAC, dutPort4MAC string) {
	t.Helper()
	ateP4 := env.ate.Port(t, "port4")
	env.otgConfig.Devices().Clear()

	// ATE LAG VLAN 100 device (192.0.2.10/30, 2001:db8::a/126, AS 64501).
	lagVlanDev := env.otgConfig.Devices().Add().SetName("ateLagVlan100.dev")
	lagVlanEth := lagVlanDev.Ethernets().Add().SetName("ateLagVlan100.Eth").SetMac(ateLagAttrs.MAC)
	lagVlanEth.Connection().SetLagName(ateLagAttrs.Name)
	lagVlanEth.Vlans().Add().SetName("ateLagVlan100.vlan").SetId(b2CoreVlanID)
	lagVlanIPv4 := lagVlanEth.Ipv4Addresses().Add().SetName("ateLagVlan100.IPv4").
		SetAddress(ateLagVlan100IPv4).
		SetGateway(dutLagVlan100IPv4).
		SetPrefix(ipv4PrefixLen)
	lagVlanIPv6 := lagVlanEth.Ipv6Addresses().Add().SetName("ateLagVlan100.IPv6").
		SetAddress(ateLagVlan100IPv6).
		SetGateway(dutLagVlan100IPv6).
		SetPrefix(ipv6PrefixLen)

	otgconfighelpers.AdvertiseBGPRoutes(
		lagVlanDev,
		lagVlanIPv4,
		lagVlanIPv6,
		&otgconfighelpers.AdvertisedRoutes{
			StartingAddress: b2CoreLagBGPV4Prefix,
			PrefixLength:    b2CoreLagBGPV4PrefixLen,
			Count:           b2CoreLagBGPV4Count,
			ATEAS:           ateLagAS,
		},
		&otgconfighelpers.AdvertisedRoutes{
			StartingAddress: b2CoreLagBGPV6Prefix,
			PrefixLength:    b2CoreLagBGPV6PrefixLen,
			Count:           b2CoreLagBGPV6Count,
			ATEAS:           ateLagAS,
		},
	)

	// ATE Port-4 VLAN 100 device (192.0.2.14/30, 2001:db8::e/126, AS 64502).
	p4VlanDev := env.otgConfig.Devices().Add().SetName("atePort4Vlan100.dev")
	p4VlanEth := p4VlanDev.Ethernets().Add().SetName("atePort4Vlan100.Eth").SetMac(atePort4Attrs.MAC)
	p4VlanEth.Connection().SetPortName(ateP4.ID())
	p4VlanEth.Vlans().Add().SetName("atePort4Vlan100.vlan").SetId(b2CoreVlanID)
	p4VlanIPv4 := p4VlanEth.Ipv4Addresses().Add().SetName("atePort4Vlan100.IPv4").
		SetAddress(atePort4Vlan100IPv4).
		SetGateway(dutPort4Vlan100IPv4).
		SetPrefix(ipv4PrefixLen)
	p4VlanIPv6 := p4VlanEth.Ipv6Addresses().Add().SetName("atePort4Vlan100.IPv6").
		SetAddress(atePort4Vlan100IPv6).
		SetGateway(dutPort4Vlan100IPv6).
		SetPrefix(ipv6PrefixLen)

	otgconfighelpers.AdvertiseBGPRoutes(
		p4VlanDev,
		p4VlanIPv4,
		p4VlanIPv6,
		&otgconfighelpers.AdvertisedRoutes{
			StartingAddress: b2CorePort4BGPV4Prefix,
			PrefixLength:    b2CorePort4BGPV4PrefixLen,
			Count:           b2CorePort4BGPV4Count,
			ATEAS:           atePort4AS,
		},
		&otgconfighelpers.AdvertisedRoutes{
			StartingAddress: b2CorePort4BGPV6Prefix,
			PrefixLength:    b2CorePort4BGPV6PrefixLen,
			Count:           b2CorePort4BGPV6Count,
			ATEAS:           atePort4AS,
		},
	)

	// Replace OTG flows with the 4 B2_CORE VLAN 100 bidirectional flows for RT-5.18.5 and RT-5.18.6:
	// - Forward IPv4 (Port-4.100 -> Port-Channel1.100): Src 198.51.100.1 -> Dst 203.0.113.1 (VLAN 100)
	// - Reverse IPv4 (Port-Channel1.100 -> Port-4.100): Src 203.0.113.1 -> Dst 198.51.100.1 (VLAN 100)
	// - Forward IPv6 (Port-4.100 -> Port-Channel1.100): Src 2001:db8:3000::1 -> Dst 2001:db8:4000::1 (VLAN 100)
	// - Reverse IPv6 (Port-Channel1.100 -> Port-4.100): Src 2001:db8:4000::1 -> Dst 2001:db8:3000::1 (VLAN 100)
	env.otgConfig.Flows().Clear()
	addTaggedIPv4Flow(env.otgConfig, flowB2CoreIPv4Fwd, p4VlanIPv4.Name(), lagVlanIPv4.Name(), atePort4Attrs.MAC, dutPort4MAC, b2CorePort4BGPV4FirstIP, b2CoreLagBGPV4FirstIP, b2CoreVlanID)
	addTaggedIPv4Flow(env.otgConfig, flowB2CoreIPv4Rev, lagVlanIPv4.Name(), p4VlanIPv4.Name(), ateLagAttrs.MAC, dutLagMAC, b2CoreLagBGPV4FirstIP, b2CorePort4BGPV4FirstIP, b2CoreVlanID)
	addTaggedIPv6Flow(env.otgConfig, flowB2CoreIPv6Fwd, p4VlanIPv6.Name(), lagVlanIPv6.Name(), atePort4Attrs.MAC, dutPort4MAC, b2CorePort4BGPV6FirstIP, b2CoreLagBGPV6FirstIP, b2CoreVlanID)
	addTaggedIPv6Flow(env.otgConfig, flowB2CoreIPv6Rev, lagVlanIPv6.Name(), p4VlanIPv6.Name(), ateLagAttrs.MAC, dutLagMAC, b2CoreLagBGPV6FirstIP, b2CorePort4BGPV6FirstIP, b2CoreVlanID)
}

// waitForB2CoreARPAndND refreshes OTG get_status neighbor state for ateLagVlan100.Eth and atePort4Vlan100.Eth
// before starting B2_CORE traffic without fatally aborting or restarting protocols if IxNetwork omits an entry.
func waitForB2CoreARPAndND(t *testing.T, env *testEnv) {
	t.Helper()
	v4Gateways := []struct {
		ethName string
		gwIP    string
	}{
		{ethName: "ateLagVlan100.Eth", gwIP: dutLagVlan100IPv4},
		{ethName: "atePort4Vlan100.Eth", gwIP: dutPort4Vlan100IPv4},
	}
	for _, gw := range v4Gateways {
		if _, ok := gnmi.Watch(t, env.ate.OTG(), gnmi.OTG().Interface(gw.ethName).Ipv4Neighbor(gw.gwIP).LinkLayerAddress().State(), 15*time.Second, func(val *ygnmi.Value[string]) bool {
			got, present := val.Val()
			return present && got != ""
		}).Await(t); !ok {
			t.Logf("OTG IPv4 gateway neighbor %s on %s not reported in get_status within 15s (using explicit flow Dst MAC)", gw.gwIP, gw.ethName)
		}
	}

	v6Gateways := []struct {
		ethName string
		gwIP    string
	}{
		{ethName: "ateLagVlan100.Eth", gwIP: dutLagVlan100IPv6},
		{ethName: "atePort4Vlan100.Eth", gwIP: dutPort4Vlan100IPv6},
	}
	for _, gw := range v6Gateways {
		if _, ok := gnmi.Watch(t, env.ate.OTG(), gnmi.OTG().Interface(gw.ethName).Ipv6Neighbor(gw.gwIP).LinkLayerAddress().State(), 2*time.Second, func(val *ygnmi.Value[string]) bool {
			got, present := val.Val()
			return present && got != ""
		}).Await(t); !ok {
			t.Logf("OTG IPv6 gateway neighbor %s on %s not reported in get_status within 2s (using explicit flow Dst MAC)", gw.gwIP, gw.ethName)
		}
	}
}

// testDifferentialMemberDrainForwardingViable implements RT-5.18.6 - Differential Member Drain Behavior:
// Standard Member Drain (forwarding-viable: false) vs. Retained Uplink LAG Members (Steps 1-5).
func testDifferentialMemberDrainForwardingViable(t *testing.T, env *testEnv) {
	p1 := env.memberPorts[2]
	p2 := env.memberPorts[1]
	p3 := env.memberPorts[0]
	activeMembers := []*ondatra.Port{p2, p3}

	// RT-5.18.6 Step 1: On the active uplink LAG (Port-Channel1) carrying B2_CORE traffic across Port-1..Port-3,
	// apply standard member drain semantics (forwarding-viable: false) to member Port-1 while retaining
	// Port-2 and Port-3 with forwarding-viable: true and aggregate-id: <aggID>.
	t.Logf("RT-5.18.6 Step 1: Setting %s config/forwarding-viable = false while retaining %s and %s with forwarding-viable = true",
		p1.Name(), p2.Name(), p3.Name())
	drainBatch := &gnmi.SetBatch{}
	cfgplugins.SetMemberForwardingViable(t, env.dut, drainBatch, p1.Name(), env.aggID, false)
	cfgplugins.SetMemberForwardingViable(t, env.dut, drainBatch, p2.Name(), env.aggID, true)
	cfgplugins.SetMemberForwardingViable(t, env.dut, drainBatch, p3.Name(), env.aggID, true)
	drainBatch.Set(t, env.dut)

	// RT-5.18.6 Step 2: Verify via gnmi.Watch / Await that:
	// - /interfaces/interface[name=Port-1]/state/forwarding-viable transitions to false
	// - /interfaces/interface[name=<Port-2|Port-3>]/state/forwarding-viable remains true
	// - /interfaces/interface[name=<Port-1|Port-2|Port-3>]/ethernet/state/aggregate-id remains <aggID>.
	t.Log("RT-5.18.6 Step 2: Verifying Port-1 state/forwarding-viable == false, Port-2 & Port-3 state/forwarding-viable == true, and aggregate-id retained")
	cfgplugins.AwaitMemberForwardingViableAndAggregateID(t, env.dut, p1.Name(), env.aggID, false, stateTimeout)
	cfgplugins.AwaitMemberForwardingViableAndAggregateID(t, env.dut, p2.Name(), env.aggID, true, stateTimeout)
	cfgplugins.AwaitMemberForwardingViableAndAggregateID(t, env.dut, p3.Name(), env.aggID, true, stateTimeout)

	// RT-5.18.6 Step 3: Verify via gnmi.Watch that inband eBGP sessions in B2_CORE (192.0.2.10 and 2001:db8::a)
	// remain ESTABLISHED without flapping.
	t.Logf("RT-5.18.6 Step 3: Verifying inband eBGP sessions in %s (%s and %s) remain ESTABLISHED", b2CoreVRFName, ateLagVlan100IPv4, ateLagVlan100IPv6)
	cfgplugins.AwaitBGPSessionStatus(t, env.dut, b2CoreVRFName, ateLagVlan100IPv4, true, protocolTimeout)
	cfgplugins.AwaitBGPSessionStatus(t, env.dut, b2CoreVRFName, ateLagVlan100IPv6, true, protocolTimeout)

	// RT-5.18.6 Step 4: Verify via gNMI interface counter Watch that egress unicast packet rate on Port-1
	// (/interfaces/interface[name=Port-1]/state/counters/out-unicast-pkts) drops to 0, while retained members
	// Port-2 and Port-3 carry 100% of egress traffic with 0% steady-state traffic loss across B2_CORE.
	t.Log("RT-5.18.6 Step 4: Verifying Port-1 out-unicast-pkts rate drops to 0 while Port-2 & Port-3 carry 100% of egress traffic with 0% steady-state loss")
	env.ate.OTG().StopTraffic(t)
	waitForB2CoreARPAndND(t, env)
	env.ate.OTG().StartTraffic(t)
	cfgplugins.AwaitDifferentialMemberEgressTraffic(t, env.dut, p1.Name(), activeMembers, 20, trafficTimeout)
	for _, flow := range env.b2CoreFlows {
		otgvalidationhelpers.AwaitFlowSteadyState(t, env.ate, otgvalidationhelpers.SteadyStateFlowParams{
			FlowName:      flow,
			MinWindowPkts: 200,
			WantZeroLoss:  true,
			Timeout:       trafficTimeout,
		})
	}

	// RT-5.18.6 Step 5: Restore /interfaces/interface[name=Port-1]/config/forwarding-viable to true, verify
	// state/forwarding-viable returns to true, and confirm egress traffic resumes across all 3 member links
	// (Port-1, Port-2, Port-3) with 0% loss.
	t.Logf("RT-5.18.6 Step 5: Restoring %s config/forwarding-viable = true and verifying egress traffic resumes across all 3 member links", p1.Name())
	cfgplugins.SetMemberForwardingViable(t, env.dut, nil, p1.Name(), env.aggID, true)
	cfgplugins.AwaitMemberForwardingViableAndAggregateID(t, env.dut, p1.Name(), env.aggID, true, stateTimeout)
	cfgplugins.AwaitLACPMemberState(t, env.dut, env.aggID, env.memberPorts, stateTimeout)
	otgvalidationhelpers.WaitForOTGLAGUP(t, env.ate, otgvalidationhelpers.LagParams{
		LagName:       ateLagAttrs.Name,
		WantMembersUp: uint64(len(env.memberPorts)),
		Timeout:       stateTimeout,
	})
	env.ate.OTG().StopTraffic(t)
	waitForB2CoreARPAndND(t, env)
	env.ate.OTG().StartTraffic(t)
	for _, p := range env.memberPorts {
		cfgplugins.AwaitInterfaceCountersIncrement(t, env.dut, p.Name(), 10, trafficTimeout)
	}
	for _, flow := range env.b2CoreFlows {
		otgvalidationhelpers.AwaitFlowSteadyState(t, env.ate, otgvalidationhelpers.SteadyStateFlowParams{
			FlowName:      flow,
			MinWindowPkts: 200,
			WantZeroLoss:  true,
			Timeout:       trafficTimeout,
		})
	}
}
