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

	// IS-IS parameters for RT-5.18 (DEFAULT network instance, protocol identifier ISIS, name DEFAULT, Area ID 49.0001).
	// System IDs: DUT 1920.0000.2001 (NET 49.0001.1920.0000.2001.00), ATE Port-Channel1 1920.0000.2002, ATE Port-4 1920.0000.2003.
	isisInstance   = "DEFAULT"
	dutISISNet     = "49.0001.1920.0000.2001.00"
	ateLagSysID    = "192000002002"
	atePort4SysID  = "192000002003"
	isisLvl2Metric = 10

	// Advertised route pools in DEFAULT VRF (RT-5.18 Test Environment Setup & RT-5.18.1 - RT-5.18.4):
	// IS-IS reachability prefixes advertised from ATE Port-Channel1: 198.18.0.0/24 and 2001:db8:2000::/64.
	lagISISV4Prefix    = "198.18.0.0"
	lagISISV4PrefixLen = 24
	lagISISV4FirstIP   = "198.18.0.1"
	lagISISV4Count     = 1

	lagISISV6Prefix    = "2001:db8:2000::"
	lagISISV6PrefixLen = 64
	lagISISV6FirstIP   = "2001:db8:2000::1"
	lagISISV6Count     = 1

	// eBGP prefixes advertised from ATE Port-Channel1 (AS 64501):
	// 10,000 IPv4 /24 prefixes (100.64.0.0/24..100.103.15.0/24) and
	// 10,000 IPv6 /64 prefixes (2001:db8:1000::/64..2001:db8:1000:270f::/64).
	lagBGPV4Prefix    = "100.64.0.0"
	lagBGPV4PrefixLen = 24
	lagBGPV4FirstIP   = "100.64.0.1"
	lagBGPV4Step      = "0.0.1.0"
	lagBGPV4Count     = 10000

	lagBGPV6Prefix    = "2001:db8:1000::"
	lagBGPV6PrefixLen = 64
	lagBGPV6FirstIP   = "2001:db8:1000::1"
	lagBGPV6Step      = "0:0:0:1::"
	lagBGPV6Count     = 10000

	// eBGP transit prefixes advertised from ATE Port-4 (AS 64502): 198.51.100.0/24 and 2001:db8:3000::/64.
	port4BGPV4Prefix    = "198.51.100.0"
	port4BGPV4PrefixLen = 24
	port4BGPV4FirstIP   = "198.51.100.1"
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
	flow4ISISIPv6Fwd  = "Flow-4a-IS-IS-IPv6-Fwd"
	flow4BGPIPv6Fwd   = "Flow-4b-BGP-IPv6-Fwd"
	flow4IPv6BidirRev = "Flow-4c-BGP-IPv6-Rev"

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
	dutLagMAC   string
	dutPort4MAC string
	allFlows    []string
	b2CoreFlows []string
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// TestAggregateAdminDownDrain verifies RT-5.18: Aggregate Interface (LAG) Drain via Admin Down,
// covering subtests RT-5.18.1 through RT-5.18.6 and Cleanup as specified in README.md.
func TestAggregateAdminDownDrain(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	aggID := netutil.NextAggregateInterface(t, dut)

	// Configure RT-5.18 Test Environment Setup on DUT and ATE.
	env := setupEnvironment(t, dut, ate, aggID)

	// RT-5.18.1 - Admin Down Drain - Disable (Steps 1-8).
	t.Run("RT-5.18.1_Admin_Down_Drain_Disable", func(t *testing.T) {
		testAdminDownDrainDisable(t, env)
	})

	// RT-5.18.2 - Admin Down Drain - Re-enable (Steps 1-6).
	t.Run("RT-5.18.2_Admin_Down_Drain_ReEnable", func(t *testing.T) {
		testAdminDownDrainReEnable(t, env)
	})

	// RT-5.18.3 - Negative Test - Admin Down on Individual Member Links (Steps 1-7).
	t.Run("RT-5.18.3_Negative_Test_Admin_Down_On_Individual_Member_Links", func(t *testing.T) {
		testNegativeAdminDownMemberLinks(t, env)
	})

	// RT-5.18.4 - Negative Test - Admin Down on Already Disabled LAG (Steps 1-6).
	t.Run("RT-5.18.4_Negative_Test_Admin_Down_On_Already_Disabled_LAG", func(t *testing.T) {
		testNegativeAdminDownAlreadyDisabledLAG(t, env)
	})

	// RT-5.18.5 - Uplink LAG Member Retention and Inband BGP Reachability in Non-Default VRF (B2_CORE) Across Drain States (Steps 1-8).
	t.Run("RT-5.18.5_Uplink_LAG_Member_Retention_And_Inband_BGP_In_B2_CORE", func(t *testing.T) {
		testUplinkLAGMemberRetentionAndB2CoreBGP(t, env)
	})

	// RT-5.18.6 - Differential Member Drain Behavior: Standard Member Drain (forwarding-viable: false) vs. Retained Uplink LAG Members (Steps 1-5).
	t.Run("RT-5.18.6_Differential_Member_Drain_Forwarding_Viable_False", func(t *testing.T) {
		testDifferentialMemberDrainForwardingViable(t, env)
	})
}

// setupEnvironment configures the DUT and ATE according to the RT-5.18 Test Environment Setup in README.md:
// - 1. 3-member LACP Aggregate Interface (Port-Channel1 with member links Port-1..Port-3, IPv4 192.0.2.1/30, IPv6 2001:db8::1/126)
// - 2. Standalone Layer 3 Transit Interface (Port-4 with IPv4 192.0.2.5/30, IPv6 2001:db8::5/126)
// - 3. Control Plane Configuration (IS-IS Level-2 and Dual-Stack eBGP in DEFAULT network instance)
// - 4. Deterministic Route Pools (10,000 IPv4 and 10,000 IPv6 BGP prefixes from ATE LAG; IS-IS and Port-4 transit prefixes)
// - 5. Continuous Bidirectional IPv4 and IPv6 Traffic Flows.
func setupEnvironment(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, aggID string) *testEnv {
	t.Helper()
	memberPortIDs, memberPorts := cfgplugins.SortPortIDsByDUTName(t, dut, []string{"port1", "port2", "port3"})
	p1, p2, p3 := memberPorts[0], memberPorts[1], memberPorts[2]
	p4 := dut.Port(t, "port4")

	// Register RT-5.18 DUT cleanup routine to restore DUT interfaces, LACP,
	// subinterfaces, and network instances (DEFAULT and B2_CORE) to their pre-test baseline state.
	t.Cleanup(func() {
		t.Log("Executing RT-5.18 DUT Cleanup: restoring DUT interface and protocol state")
		cfgplugins.CleanupAggregateDrainTest(t, dut, cfgplugins.AggregateDrainCleanupParams{
			AggID:          aggID,
			MemberPorts:    memberPorts,
			TransitPort:    p4,
			ISISInstance:   isisInstance,
			LocalAS:        dutAS,
			NonDefaultVRF:  b2CoreVRFName,
			TaggedSubIndex: b2CoreSubintfIndex,
		})
	})

	fptest.ConfigureDefaultNetworkInstance(t, dut)
	cfgplugins.ClearAggregateMembers(t, dut, nil, memberPorts)

	sb := &gnmi.SetBatch{}

	// RT-5.18 Test Environment Setup - 1. Configure 3-member LAG interface (Port-Channel1 - Interface Under Test):
	// - /interfaces/interface[name=Port-Channel1]/config/type = ieee8023adLag
	// - /interfaces/interface[name=Port-Channel1]/aggregation/config/lag-type = LACP
	// - /interfaces/interface[name=Port-Channel1]/aggregation/config/min-links = 1
	// - /lacp/interfaces/interface[name=Port-Channel1]/config/lacp-mode = ACTIVE
	// - /interfaces/interface[name=<DUT Port-1..Port-3>]/ethernet/config/aggregate-id = Port-Channel1
	// - Subinterface 0 IPv4 192.0.2.1/30 and IPv6 2001:db8::1/126 in DEFAULT network instance.
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

	// RT-5.18 Test Environment Setup - 2. Configure standalone Layer 3 transit interface (Port-4):
	// - Subinterface 0 IPv4 192.0.2.5/30 and IPv6 2001:db8::5/126 in DEFAULT network instance.
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

	// RT-5.18 Test Environment Setup - 3. Control Plane Configuration (IS-IS & eBGP in DEFAULT VRF):
	protoBatch := &gnmi.SetBatch{}

	// 3.1 Establish IS-IS (DEFAULT network instance, protocol identifier ISIS, name ISIS, LEVEL_2,
	// WIDE_METRIC, Area ID 49.0001, DUT NET 49.0001.1920.0000.2001.00, POINT_TO_POINT metric 10 on Port-Channel1 and Port-4).
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

	// 3.2 Establish eBGP (DEFAULT network instance, protocol identifier BGP, name BGP, DUT local AS 64500)
	// with 4 neighbors (IPV4_UNICAST and IPV6_UNICAST):
	// - Over Port-Channel1 (ATE AS 64501): 192.0.2.2 (local 192.0.2.1) and 2001:db8::2 (local 2001:db8::1)
	// - Over Port-4 (ATE AS 64502): 192.0.2.6 (local 192.0.2.5) and 2001:db8::6 (local 2001:db8::5).
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

	// Configure OTG LAG, IS-IS, eBGP route pools, and continuous bidirectional traffic flows on ATE.
	dutLagMAC := otgvalidationhelpers.ResolveDUTInterfaceMAC(t, dut, ate, "", "", "", aggID)
	dutPort4MAC := otgvalidationhelpers.ResolveDUTInterfaceMAC(t, dut, ate, "", "", "", p4.Name())
	otgConfig := configureOTG(t, ate, memberPortIDs, dutLagMAC, dutPort4MAC)
	ate.OTG().PushConfig(t, otgConfig)
	ate.OTG().StartProtocols(t)
	t.Cleanup(func() {
		t.Log("Executing RT-5.18 ATE Cleanup: stopping ATE traffic and protocols")
		ate.OTG().StopTraffic(t)
		ate.OTG().StopProtocols(t)
	})

	return &testEnv{
		dut:         dut,
		ate:         ate,
		otgConfig:   otgConfig,
		aggID:       aggID,
		memberPorts: memberPorts,
		port4:       p4,
		dutLagMAC:   dutLagMAC,
		dutPort4MAC: dutPort4MAC,
		allFlows: []string{
			flow1ISISIPv4Fwd,
			flow2BGPIPv4Fwd,
			flow3BGPIPv4Rev,
			flow4ISISIPv6Fwd,
			flow4BGPIPv6Fwd,
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
func configureOTG(t *testing.T, ate *ondatra.ATEDevice, memberPortIDs []string, dutLagMAC, dutPort4MAC string) gosnappi.Config {
	t.Helper()
	config := gosnappi.NewConfig()

	// Configure 3-member LACP ACTIVE LAG on ATE Port-1, Port-2, and Port-3 using shared otgconfighelpers.ConfigureLag.
	otgconfighelpers.ConfigureLag(t, config, ate, &otgconfighelpers.Port{
		Name:        ateLagAttrs.Name,
		AggMAC:      ateLagAttrs.MAC,
		MemberMACs:  ateMemberMACs,
		MemberPorts: memberPortIDs,
		LagID:       1,
		IsLag:       true,
	})

	ateP4 := ate.Port(t, "port4")
	config.Ports().Add().SetName(ateP4.ID())

	// Device on ATE LAG (Port-1..Port-3) in DEFAULT VRF (IPv4 192.0.2.2/30, IPv6 2001:db8::2/126).
	lagDev := config.Devices().Add().SetName(ateLagAttrs.Name + ".dev")
	lagEth := lagDev.Ethernets().Add().SetName(ateLagAttrs.Name + ".Eth").SetMac(ateLagAttrs.MAC)
	lagEth.Connection().SetLagName(ateLagAttrs.Name)
	lagIPv4 := lagEth.Ipv4Addresses().Add().SetName(ateLagAttrs.Name + ".IPv4").
		SetAddress(ateLagAttrs.IPv4).
		SetGateway(dutLagAttrs.IPv4).
		SetPrefix(uint32(ateLagAttrs.IPv4Len))
	lagIPv6 := lagEth.Ipv6Addresses().Add().SetName(ateLagAttrs.Name + ".IPv6").
		SetAddress(ateLagAttrs.IPv6).
		SetGateway(dutLagAttrs.IPv6).
		SetPrefix(uint32(ateLagAttrs.IPv6Len))

	// Configure IS-IS on ATE Port-Channel1 (System ID 1920.0000.2002) advertising reachability prefixes
	// 198.18.0.0/24 (IPv4) and 2001:db8:2000::/64 (IPv6).
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

	// Configure eBGP (AS 64501) on ATE Port-Channel1 advertising 10,000 IPv4 /24 prefixes
	// (100.64.0.0/24..100.103.15.0/24) and 10,000 IPv6 /64 prefixes (2001:db8:1000::/64..2001:db8:1000:270f::/64).
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

	// Device on ATE Port-4 in DEFAULT VRF (IPv4 192.0.2.6/30, IPv6 2001:db8::6/126).
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

	// Configure IS-IS (System ID 1920.0000.2003) and eBGP (AS 64502) on ATE Port-4 advertising
	// transit prefixes 198.51.100.0/24 (IPv4) and 2001:db8:3000::/64 (IPv6).
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

	// Configure continuous bidirectional IPv4 and IPv6 traffic flows as defined in RT-5.18.1 Step 2:
	// - Forward IPv4 Flows (Port-4 -> Port-Channel1): Source IP 198.51.100.1 to IS-IS destination IP 198.18.0.1
	//   and to all 10,000 BGP destination IPs 100.64.0.1..100.103.15.1.
	otgconfighelpers.AddContinuousUDPFlow(config, otgconfighelpers.ContinuousUDPFlowParams{
		FlowName:     flow1ISISIPv4Fwd,
		TxEndpoint:   p4IPv4.Name(),
		RxEndpoint:   lagIPv4.Name(),
		SrcMAC:       atePort4Attrs.MAC,
		DstMAC:       dutPort4MAC,
		SrcIP:        port4BGPV4FirstIP,
		DstIP:        lagISISV4FirstIP,
		IPStep:       lagBGPV4Step,
		IPCount:      lagISISV4Count,
		IncrementDst: true,
		FrameSize:    flowFrameSize,
		PpsRate:      flowPPS,
	})
	otgconfighelpers.AddContinuousUDPFlow(config, otgconfighelpers.ContinuousUDPFlowParams{
		FlowName:     flow2BGPIPv4Fwd,
		TxEndpoint:   p4IPv4.Name(),
		RxEndpoint:   lagIPv4.Name(),
		SrcMAC:       atePort4Attrs.MAC,
		DstMAC:       dutPort4MAC,
		SrcIP:        port4BGPV4FirstIP,
		DstIP:        lagBGPV4FirstIP,
		IPStep:       lagBGPV4Step,
		IPCount:      lagBGPV4Count,
		IncrementDst: true,
		FrameSize:    flowFrameSize,
		PpsRate:      flowPPS,
	})

	// - Reverse IPv4 Flow (Port-Channel1 -> Port-4): Source IPs 100.64.0.1..100.103.15.1 to destination IP 198.51.100.1.
	otgconfighelpers.AddContinuousUDPFlow(config, otgconfighelpers.ContinuousUDPFlowParams{
		FlowName:     flow3BGPIPv4Rev,
		TxEndpoint:   lagIPv4.Name(),
		RxEndpoint:   p4IPv4.Name(),
		SrcMAC:       ateLagAttrs.MAC,
		DstMAC:       dutLagMAC,
		SrcIP:        lagBGPV4FirstIP,
		DstIP:        port4BGPV4FirstIP,
		IPStep:       lagBGPV4Step,
		IPCount:      lagBGPV4Count,
		IncrementSrc: true,
		FrameSize:    flowFrameSize,
		PpsRate:      flowPPS,
	})

	// - Forward IPv6 Flows (Port-4 -> Port-Channel1): Source IP 2001:db8:3000::1 to IS-IS destination IP 2001:db8:2000::1
	//   and to all 10,000 BGP destination IPs 2001:db8:1000:0::1..2001:db8:1000:270f::1.
	otgconfighelpers.AddContinuousUDPFlow(config, otgconfighelpers.ContinuousUDPFlowParams{
		FlowName:     flow4ISISIPv6Fwd,
		TxEndpoint:   p4IPv6.Name(),
		RxEndpoint:   lagIPv6.Name(),
		SrcMAC:       atePort4Attrs.MAC,
		DstMAC:       dutPort4MAC,
		IsIPv6:       true,
		SrcIP:        port4BGPV6FirstIP,
		DstIP:        lagISISV6FirstIP,
		IPStep:       lagBGPV6Step,
		IPCount:      lagISISV6Count,
		IncrementDst: true,
		FrameSize:    flowFrameSize,
		PpsRate:      flowPPS,
	})
	otgconfighelpers.AddContinuousUDPFlow(config, otgconfighelpers.ContinuousUDPFlowParams{
		FlowName:     flow4BGPIPv6Fwd,
		TxEndpoint:   p4IPv6.Name(),
		RxEndpoint:   lagIPv6.Name(),
		SrcMAC:       atePort4Attrs.MAC,
		DstMAC:       dutPort4MAC,
		IsIPv6:       true,
		SrcIP:        port4BGPV6FirstIP,
		DstIP:        lagBGPV6FirstIP,
		IPStep:       lagBGPV6Step,
		IPCount:      lagBGPV6Count,
		IncrementDst: true,
		FrameSize:    flowFrameSize,
		PpsRate:      flowPPS,
	})

	// - Reverse IPv6 Flow (Port-Channel1 -> Port-4): Source IPs 2001:db8:1000:0::1..2001:db8:1000:270f::1 to destination IP 2001:db8:3000::1.
	otgconfighelpers.AddContinuousUDPFlow(config, otgconfighelpers.ContinuousUDPFlowParams{
		FlowName:     flow4IPv6BidirRev,
		TxEndpoint:   lagIPv6.Name(),
		RxEndpoint:   p4IPv6.Name(),
		SrcMAC:       ateLagAttrs.MAC,
		DstMAC:       dutLagMAC,
		IsIPv6:       true,
		SrcIP:        lagBGPV6FirstIP,
		DstIP:        port4BGPV6FirstIP,
		IPStep:       lagBGPV6Step,
		IPCount:      lagBGPV6Count,
		IncrementSrc: true,
		FrameSize:    flowFrameSize,
		PpsRate:      flowPPS,
	})

	return config
}

// updateDefaultFlowMACs updates the Ethernet source and destination MAC addresses on all 6 DEFAULT VRF flows.
func updateDefaultFlowMACs(env *testEnv) {
	for _, f := range env.otgConfig.Flows().Items() {
		for _, pkt := range f.Packet().Items() {
			if pkt.Choice() == gosnappi.FlowHeaderChoice.ETHERNET {
				eth := pkt.Ethernet()
				switch f.Name() {
				case flow1ISISIPv4Fwd, flow2BGPIPv4Fwd, flow4ISISIPv6Fwd, flow4BGPIPv6Fwd:
					eth.Src().SetValue(atePort4Attrs.MAC)
					if env.dutPort4MAC != "" {
						eth.Dst().SetValue(env.dutPort4MAC)
					}
				case flow3BGPIPv4Rev, flow4IPv6BidirRev:
					eth.Src().SetValue(ateLagAttrs.MAC)
					if env.dutLagMAC != "" {
						eth.Dst().SetValue(env.dutLagMAC)
					}
				}
			}
		}
	}
}

// testAdminDownDrainDisable implements RT-5.18.1 - Admin Down Drain - Disable (Steps 1-8) as defined in README.md.
func testAdminDownDrainDisable(t *testing.T, env *testEnv) {
	// RT-5.18.1 Step 1: Verify via gNMI Watch (Await) that all 4 eBGP sessions (192.0.2.2, 2001:db8::2,
	// 192.0.2.6, 2001:db8::6) are ESTABLISHED and IS-IS Level-2 adjacencies on Port-Channel1 and Port-4 are UP.
	// Also verify aggregate interface, member ports, LACP state, and that BGP installed prefixes on Port-Channel1
	// reach 10000 for IPv4 (192.0.2.2) and 10000 for IPv6 (2001:db8::2):
	// - /network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=BGP][name=BGP]/bgp/neighbors/neighbor[neighbor-address=192.0.2.2]/afi-safis/afi-safi[afi-safi-name=IPV4_UNICAST]/state/prefixes/installed
	// - /network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=BGP][name=BGP]/bgp/neighbors/neighbor[neighbor-address=2001:db8::2]/afi-safis/afi-safi[afi-safi-name=IPV6_UNICAST]/state/prefixes/installed
	t.Log("RT-5.18.1 Step 1: Verifying LAG/LACP state, all 4 eBGP sessions ESTABLISHED, IS-IS Level-2 adjacencies UP, and 10,000 IPv4/IPv6 BGP prefixes installed")
	cfgplugins.AwaitInterfaceAdminAndOperStatus(t, env.dut, env.aggID, true, oc.Interface_AdminStatus_UP, oc.Interface_OperStatus_UP, stateTimeout)
	cfgplugins.AwaitAggregateState(t, env.dut, env.aggID, oc.IfAggregate_AggregationType_LACP, stateTimeout)
	for _, p := range env.memberPorts {
		cfgplugins.AwaitInterfaceAdminAndOperStatus(t, env.dut, p.Name(), true, oc.Interface_AdminStatus_UP, oc.Interface_OperStatus_UP, stateTimeout)
		cfgplugins.AwaitMemberForwardingViableAndAggregateID(t, env.dut, p.Name(), env.aggID, true, stateTimeout)
	}
	cfgplugins.AwaitInterfaceAdminAndOperStatus(t, env.dut, env.port4.Name(), true, oc.Interface_AdminStatus_UP, oc.Interface_OperStatus_UP, stateTimeout)
	cfgplugins.AwaitLACPMemberState(t, env.dut, env.aggID, env.memberPorts, stateTimeout)
	otgvalidationhelpers.WaitForOTGLAGUP(t, env.ate, otgvalidationhelpers.LagParams{
		LagName:       ateLagAttrs.Name,
		WantMembersUp: uint64(len(env.memberPorts)),
		Timeout:       stateTimeout,
	})

	for _, peer := range []string{ateLagAttrs.IPv4, ateLagAttrs.IPv6, atePort4Attrs.IPv4, atePort4Attrs.IPv6} {
		cfgplugins.AwaitBGPSessionStatus(t, env.dut, deviations.DefaultNetworkInstance(env.dut), peer, true, protocolTimeout)
	}
	cfgplugins.AwaitISISAdjacencyStatus(t, env.dut, isisInstance, env.aggID, 0, true, protocolTimeout)
	cfgplugins.AwaitISISAdjacencyStatus(t, env.dut, isisInstance, env.port4.Name(), 0, true, protocolTimeout)

	cfgplugins.AwaitBGPPrefixes(t, env.dut, deviations.DefaultNetworkInstance(env.dut), ateLagAttrs.IPv4, oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST, lagBGPV4Count, lagBGPV4Count, protocolTimeout)
	cfgplugins.AwaitBGPPrefixes(t, env.dut, deviations.DefaultNetworkInstance(env.dut), ateLagAttrs.IPv6, oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST, lagBGPV6Count, lagBGPV6Count, protocolTimeout)
	cfgplugins.AwaitBGPPrefixes(t, env.dut, deviations.DefaultNetworkInstance(env.dut), atePort4Attrs.IPv4, oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST, port4BGPV4Count, port4BGPV4Count, protocolTimeout)
	cfgplugins.AwaitBGPPrefixes(t, env.dut, deviations.DefaultNetworkInstance(env.dut), atePort4Attrs.IPv6, oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST, port4BGPV6Count, port4BGPV6Count, protocolTimeout)

	// RT-5.18.1 Step 2: Ensure IPv4 ARP and IPv6 ND resolution completes (WaitForARP) on all ATE interfaces,
	// then start continuous bidirectional IPv4 and IPv6 traffic flows between ATE Port-4 and ATE Port-Channel1 (Port-1..Port-3).
	t.Log("RT-5.18.1 Step 2: Resolving IPv4 ARP and IPv6 ND (WaitForARP) and starting continuous bidirectional IPv4 and IPv6 traffic flows")
	otgutils.WaitForARP(t, env.ate.OTG(), env.otgConfig, "IPv4")
	otgutils.WaitForARP(t, env.ate.OTG(), env.otgConfig, "IPv6")
	if resolvedLagMAC := otgvalidationhelpers.ResolveDUTInterfaceMAC(t, env.dut, env.ate, ateLagAttrs.Name+".Eth", dutLagAttrs.IPv4, dutLagAttrs.IPv6, env.aggID); resolvedLagMAC != "" {
		env.dutLagMAC = resolvedLagMAC
	}
	if resolvedPort4MAC := otgvalidationhelpers.ResolveDUTInterfaceMAC(t, env.dut, env.ate, atePort4Attrs.Name+".Eth", dutPort4Attrs.IPv4, dutPort4Attrs.IPv6, env.port4.Name()); resolvedPort4MAC != "" {
		env.dutPort4MAC = resolvedPort4MAC
	}
	updateDefaultFlowMACs(env)
	env.ate.OTG().StartTraffic(t)

	// RT-5.18.1 Step 3: Verify 0% steady-state traffic loss across all flows (using gNMI counter Watch, with no static sleep wait time).
	t.Log("RT-5.18.1 Step 3: Verifying 0% steady-state traffic loss across all flows and incrementing counters on Port-Channel1 and Port-1..Port-3")
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

	// RT-5.18.1 Step 4: Administratively disable the LAG interface by setting
	// /interfaces/interface[name=Port-Channel1]/config/enabled to false using gNMI.Set with REPLACE option.
	t.Logf("RT-5.18.1 Step 4: Administratively disabling aggregate interface %s (/interfaces/interface[name=%s]/config/enabled = false)", env.aggID, env.aggID)
	cfgplugins.SetInterfaceAdminEnabled(t, env.dut, nil, env.aggID, false)

	// RT-5.18.1 Step 5: Verify via gNMI Subscribe (Watch) that /interfaces/interface[name=Port-Channel1]/state/admin-status
	// is DOWN and /interfaces/interface[name=Port-Channel1]/state/oper-status is DOWN, while member ports retain aggregate membership.
	t.Logf("RT-5.18.1 Step 5: Verifying %s state/enabled == false, admin-status == DOWN, and oper-status == DOWN", env.aggID)
	cfgplugins.AwaitInterfaceAdminAndOperStatus(t, env.dut, env.aggID, false, oc.Interface_AdminStatus_DOWN, oc.Interface_OperStatus_DOWN, stateTimeout)
	for _, p := range env.memberPorts {
		cfgplugins.AwaitMemberForwardingViableAndAggregateID(t, env.dut, p.Name(), env.aggID, true, stateTimeout)
		if got := gnmi.Get(t, env.dut, gnmi.OC().Interface(p.Name()).Enabled().Config()); !got {
			t.Errorf("RT-5.18.1 Step 5: Member interface %s config/enabled = %v, want true", p.Name(), got)
		}
	}

	// RT-5.18.1 Step 6: Verify that the associated BGP sessions on Port-Channel1 (192.0.2.2 and 2001:db8::2)
	// transition to ACTIVE or IDLE (not ESTABLISHED) via
	// /network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=BGP][name=BGP]/bgp/neighbors/neighbor[neighbor-address=<192.0.2.2|2001:db8::2>]/state/session-state,
	// while the BGP sessions on Port-4 (192.0.2.6 and 2001:db8::6) remain ESTABLISHED.
	t.Log("RT-5.18.1 Step 6: Verifying eBGP sessions over Port-Channel1 transition out of ESTABLISHED while Port-4 sessions remain ESTABLISHED")
	cfgplugins.AwaitBGPSessionStatus(t, env.dut, deviations.DefaultNetworkInstance(env.dut), ateLagAttrs.IPv4, false, protocolTimeout)
	cfgplugins.AwaitBGPSessionStatus(t, env.dut, deviations.DefaultNetworkInstance(env.dut), ateLagAttrs.IPv6, false, protocolTimeout)
	cfgplugins.AwaitBGPSessionStatus(t, env.dut, deviations.DefaultNetworkInstance(env.dut), atePort4Attrs.IPv4, true, protocolTimeout)
	cfgplugins.AwaitBGPSessionStatus(t, env.dut, deviations.DefaultNetworkInstance(env.dut), atePort4Attrs.IPv6, true, protocolTimeout)

	// RT-5.18.1 Step 7: Verify that the IS-IS adjacency on Port-Channel1 transitions to DOWN (or is removed from the active adjacency table)
	// via /network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=ISIS]/isis/interfaces/interface[interface-id=Port-Channel1]/levels/level[level-number=2]/adjacencies/adjacency/state/adjacency-state,
	// while the IS-IS adjacency on Port-4 remains UP.
	t.Log("RT-5.18.1 Step 7: Verifying IS-IS adjacency on Port-Channel1 transitions to DOWN while Port-4 remains UP")
	cfgplugins.AwaitISISAdjacencyStatus(t, env.dut, isisInstance, env.aggID, 0, false, protocolTimeout)
	cfgplugins.AwaitISISAdjacencyStatus(t, env.dut, isisInstance, env.port4.Name(), 0, true, protocolTimeout)

	// RT-5.18.1 Step 8: Verify that all bidirectional traffic traversing Port-Channel1 stops completely (steady-state traffic loss is 100%).
	t.Log("RT-5.18.1 Step 8: Verifying all bidirectional traffic traversing Port-Channel1 stops completely (100% steady-state loss)")
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
	cfgplugins.AwaitInterfaceAdminAndOperStatus(t, env.dut, env.port4.Name(), true, oc.Interface_AdminStatus_UP, oc.Interface_OperStatus_UP, stateTimeout)
	env.ate.OTG().StopTraffic(t)
}

// restartTrafficAfterLAGChange waits for DUT and OTG LACP member synchronization across activeMembers
// and restarts OTG traffic so IxNetwork re-binds LAG-sourced device flows onto active member links.
func restartTrafficAfterLAGChange(t *testing.T, env *testEnv, activeMembers []*ondatra.Port) {
	t.Helper()
	activeATEPortIDs := make([]string, 0, len(activeMembers))
	for _, p := range activeMembers {
		activeATEPortIDs = append(activeATEPortIDs, env.ate.Port(t, p.ID()).ID())
	}
	cfgplugins.AwaitLACPMemberState(t, env.dut, env.aggID, activeMembers, stateTimeout)
	otgvalidationhelpers.WaitForOTGLAGUP(t, env.ate, otgvalidationhelpers.LagParams{
		LagName:       ateLagAttrs.Name,
		WantMembersUp: uint64(len(activeMembers)),
		MemberPortIDs: activeATEPortIDs,
		Timeout:       stateTimeout,
	})
	env.ate.OTG().StopTraffic(t)
	otgutils.WaitForARP(t, env.ate.OTG(), env.otgConfig, "IPv4")
	otgutils.WaitForARP(t, env.ate.OTG(), env.otgConfig, "IPv6")
	env.ate.OTG().StartTraffic(t)
	if otgvalidationhelpers.AreFlowsTransmitting(t, env.ate, env.allFlows, 20, 8*time.Second) {
		return
	}
	t.Log("restartTrafficAfterLAGChange: flows not yet incrementing OutPkts/InPkts after StartTraffic; re-pushing OTG config to re-bind LAG traffic items")
	updateDefaultFlowMACs(env)
	env.ate.OTG().StopTraffic(t)
	env.ate.OTG().StopProtocols(t)
	env.ate.OTG().PushConfig(t, env.otgConfig)
	env.ate.OTG().StartProtocols(t)
	cfgplugins.AwaitLACPMemberState(t, env.dut, env.aggID, activeMembers, stateTimeout)
	otgvalidationhelpers.WaitForOTGLAGUP(t, env.ate, otgvalidationhelpers.LagParams{
		LagName:       ateLagAttrs.Name,
		WantMembersUp: uint64(len(activeMembers)),
		MemberPortIDs: activeATEPortIDs,
		Timeout:       stateTimeout,
	})
	for _, peer := range []string{ateLagAttrs.IPv4, ateLagAttrs.IPv6, atePort4Attrs.IPv4, atePort4Attrs.IPv6} {
		cfgplugins.AwaitBGPSessionStatus(t, env.dut, deviations.DefaultNetworkInstance(env.dut), peer, true, protocolTimeout)
	}
	cfgplugins.AwaitISISAdjacencyStatus(t, env.dut, isisInstance, env.aggID, 0, true, protocolTimeout)
	cfgplugins.AwaitISISAdjacencyStatus(t, env.dut, isisInstance, env.port4.Name(), 0, true, protocolTimeout)
	cfgplugins.AwaitBGPPrefixes(t, env.dut, deviations.DefaultNetworkInstance(env.dut), ateLagAttrs.IPv4, oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST, lagBGPV4Count, lagBGPV4Count, protocolTimeout)
	cfgplugins.AwaitBGPPrefixes(t, env.dut, deviations.DefaultNetworkInstance(env.dut), ateLagAttrs.IPv6, oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST, lagBGPV6Count, lagBGPV6Count, protocolTimeout)
	cfgplugins.AwaitBGPPrefixes(t, env.dut, deviations.DefaultNetworkInstance(env.dut), atePort4Attrs.IPv4, oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST, port4BGPV4Count, port4BGPV4Count, protocolTimeout)
	cfgplugins.AwaitBGPPrefixes(t, env.dut, deviations.DefaultNetworkInstance(env.dut), atePort4Attrs.IPv6, oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST, port4BGPV6Count, port4BGPV6Count, protocolTimeout)
	otgutils.WaitForARP(t, env.ate.OTG(), env.otgConfig, "IPv4")
	otgutils.WaitForARP(t, env.ate.OTG(), env.otgConfig, "IPv6")
	env.ate.OTG().StartTraffic(t)
}

// testAdminDownDrainReEnable implements RT-5.18.2 - Admin Down Drain - Re-enable (Steps 1-6) as defined in README.md.
func testAdminDownDrainReEnable(t *testing.T, env *testEnv) {
	// RT-5.18.2 Step 1: Re-enable the LAG interface by setting /interfaces/interface[name=Port-Channel1]/config/enabled
	// to true using gNMI.Set with REPLACE option.
	t.Logf("RT-5.18.2 Step 1: Re-enabling aggregate interface %s (/interfaces/interface[name=%s]/config/enabled = true)", env.aggID, env.aggID)
	cfgplugins.SetInterfaceAdminEnabled(t, env.dut, nil, env.aggID, true)

	// RT-5.18.2 Step 2: Verify via gNMI Subscribe (Watch) that /interfaces/interface[name=Port-Channel1]/state/admin-status
	// is UP and /interfaces/interface[name=Port-Channel1]/state/oper-status is UP, and all 3 LACP members synchronize.
	t.Logf("RT-5.18.2 Step 2: Verifying %s state/enabled == true, admin-status == UP, oper-status == UP, and LACP members IN_SYNC", env.aggID)
	cfgplugins.AwaitInterfaceAdminAndOperStatus(t, env.dut, env.aggID, true, oc.Interface_AdminStatus_UP, oc.Interface_OperStatus_UP, stateTimeout)
	cfgplugins.AwaitLACPMemberState(t, env.dut, env.aggID, env.memberPorts, stateTimeout)
	otgvalidationhelpers.WaitForOTGLAGUP(t, env.ate, otgvalidationhelpers.LagParams{
		LagName:       ateLagAttrs.Name,
		WantMembersUp: uint64(len(env.memberPorts)),
		Timeout:       stateTimeout,
	})

	// RT-5.18.2 Step 3: Verify that the BGP sessions over Port-Channel1 (192.0.2.2 and 2001:db8::2) re-establish
	// by checking session-state is ESTABLISHED at the paths from RT-5.18.1 Step 6.
	t.Log("RT-5.18.2 Step 3: Verifying eBGP sessions over Port-Channel1 (192.0.2.2 and 2001:db8::2) re-establish to ESTABLISHED")
	cfgplugins.AwaitBGPSessionStatus(t, env.dut, deviations.DefaultNetworkInstance(env.dut), ateLagAttrs.IPv4, true, protocolTimeout)
	cfgplugins.AwaitBGPSessionStatus(t, env.dut, deviations.DefaultNetworkInstance(env.dut), ateLagAttrs.IPv6, true, protocolTimeout)

	// RT-5.18.2 Step 4: Verify that the IS-IS Level-2 adjacency on Port-Channel1 re-establishes
	// by checking adjacency-state is UP at the path from RT-5.18.1 Step 7.
	t.Log("RT-5.18.2 Step 4: Verifying IS-IS Level-2 adjacency on Port-Channel1 re-establishes to UP")
	cfgplugins.AwaitISISAdjacencyStatus(t, env.dut, isisInstance, env.aggID, 0, true, protocolTimeout)

	// RT-5.18.2 Step 5: Verify via gNMI Watch that the BGP installed prefix count returns to 10000
	// for both IPV4_UNICAST and IPV6_UNICAST at the paths from RT-5.18.1 Step 1.
	t.Log("RT-5.18.2 Step 5: Verifying BGP installed prefix count returns to 10,000 for IPV4_UNICAST and IPV6_UNICAST")
	cfgplugins.AwaitBGPPrefixes(t, env.dut, deviations.DefaultNetworkInstance(env.dut), ateLagAttrs.IPv4, oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST, lagBGPV4Count, lagBGPV4Count, protocolTimeout)
	cfgplugins.AwaitBGPPrefixes(t, env.dut, deviations.DefaultNetworkInstance(env.dut), ateLagAttrs.IPv6, oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST, lagBGPV6Count, lagBGPV6Count, protocolTimeout)

	// RT-5.18.2 Step 6: Once gNMI confirms control plane and AFT convergence, verify that steady-state
	// bidirectional traffic loss returns to 0%.
	t.Log("RT-5.18.2 Step 6: Verifying steady-state bidirectional traffic loss returns to 0% across all flows and all 3 member links")
	restartTrafficAfterLAGChange(t, env, env.memberPorts)
	cfgplugins.AwaitInterfaceCountersIncrement(t, env.dut, env.aggID, 50, trafficTimeout)
	for _, p := range env.memberPorts {
		cfgplugins.AwaitInterfaceCountersIncrement(t, env.dut, p.Name(), 10, trafficTimeout)
	}
	for _, flow := range env.allFlows {
		otgvalidationhelpers.AwaitFlowSteadyState(t, env.ate, otgvalidationhelpers.SteadyStateFlowParams{
			FlowName:      flow,
			MinWindowPkts: 200,
			WantZeroLoss:  true,
			Timeout:       trafficTimeout,
		})
	}
}

// testNegativeAdminDownMemberLinks implements RT-5.18.3 - Negative Test - Admin Down on Individual Member Links (Steps 1-7) as defined in README.md.
func testNegativeAdminDownMemberLinks(t *testing.T, env *testEnv) {
	p1 := env.memberPorts[0]
	p2 := env.memberPorts[1]
	p3 := env.memberPorts[2]

	// RT-5.18.3 Step 1: Verify BGP installed prefixes are at 10000 for IPv4 (192.0.2.2) and 10000 for IPv6 (2001:db8::2)
	// via gNMI telemetry at the paths from RT-5.18.1 Step 1.
	t.Log("RT-5.18.3 Step 1: Verifying BGP installed prefixes are at 10,000 for IPv4 (192.0.2.2) and 10,000 for IPv6 (2001:db8::2)")
	cfgplugins.AwaitInterfaceAdminAndOperStatus(t, env.dut, env.aggID, true, oc.Interface_AdminStatus_UP, oc.Interface_OperStatus_UP, stateTimeout)
	for _, p := range env.memberPorts {
		cfgplugins.AwaitInterfaceAdminAndOperStatus(t, env.dut, p.Name(), true, oc.Interface_AdminStatus_UP, oc.Interface_OperStatus_UP, stateTimeout)
	}
	cfgplugins.AwaitBGPPrefixes(t, env.dut, deviations.DefaultNetworkInstance(env.dut), ateLagAttrs.IPv4, oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST, lagBGPV4Count, lagBGPV4Count, protocolTimeout)
	cfgplugins.AwaitBGPPrefixes(t, env.dut, deviations.DefaultNetworkInstance(env.dut), ateLagAttrs.IPv6, oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST, lagBGPV6Count, lagBGPV6Count, protocolTimeout)

	// RT-5.18.3 Step 2: Send continuous bidirectional IPv4 and IPv6 traffic flows between ATE Port-4 and
	// ATE Port-Channel1 (Port-1..Port-3) via the advertised prefixes at 10% of a single member link's line rate.
	t.Log("RT-5.18.3 Step 2: Verifying continuous bidirectional IPv4 and IPv6 traffic flows are forwarding at 0% loss")
	for _, flow := range env.allFlows {
		otgvalidationhelpers.AwaitFlowSteadyState(t, env.ate, otgvalidationhelpers.SteadyStateFlowParams{
			FlowName:      flow,
			MinWindowPkts: 200,
			WantZeroLoss:  true,
			Timeout:       trafficTimeout,
		})
	}

	// RT-5.18.3 Step 3: Administratively disable 2 out of the 3 individual member links of Port-Channel1
	// (DUT Port-1 and DUT Port-2, leaving DUT Port-3 active) by setting
	// /interfaces/interface[name=<DUT Port-1|Port-2>]/config/enabled to false using gNMI.Set with REPLACE option.
	t.Logf("RT-5.18.3 Step 3: Administratively disabling member links %s and %s (enabled = false) while leaving %s active", p1.Name(), p2.Name(), p3.Name())
	disableBatch := &gnmi.SetBatch{}
	cfgplugins.SetInterfaceAdminEnabled(t, env.dut, disableBatch, p1.Name(), false)
	cfgplugins.SetInterfaceAdminEnabled(t, env.dut, disableBatch, p2.Name(), false)
	disableBatch.Set(t, env.dut)

	// RT-5.18.3 Step 4: Verify via gNMI that the disabled member links (Port-1 and Port-2) transition to
	// oper-status = DOWN, while the aggregate interface /interfaces/interface[name=Port-Channel1]/state/oper-status
	// remains UP (via active member Port-3).
	t.Logf("RT-5.18.3 Step 4: Verifying %s and %s transition to oper-status == DOWN while %s remains oper-status == UP via active member %s",
		p1.Name(), p2.Name(), env.aggID, p3.Name())
	cfgplugins.AwaitInterfaceAdminAndOperStatus(t, env.dut, p1.Name(), false, oc.Interface_AdminStatus_DOWN, oc.Interface_OperStatus_DOWN, stateTimeout)
	cfgplugins.AwaitInterfaceAdminAndOperStatus(t, env.dut, p2.Name(), false, oc.Interface_AdminStatus_DOWN, oc.Interface_OperStatus_DOWN, stateTimeout)
	cfgplugins.AwaitInterfaceAdminAndOperStatus(t, env.dut, env.aggID, true, oc.Interface_AdminStatus_UP, oc.Interface_OperStatus_UP, stateTimeout)

	// RT-5.18.3 Step 5: Verify that the eBGP (192.0.2.2, 2001:db8::2) and IS-IS sessions over Port-Channel1
	// remain ESTABLISHED and UP at their respective paths without flapping.
	t.Log("RT-5.18.3 Step 5: Verifying eBGP (192.0.2.2, 2001:db8::2) and IS-IS sessions over Port-Channel1 remain ESTABLISHED and UP")
	cfgplugins.AwaitBGPSessionStatus(t, env.dut, deviations.DefaultNetworkInstance(env.dut), ateLagAttrs.IPv4, true, protocolTimeout)
	cfgplugins.AwaitBGPSessionStatus(t, env.dut, deviations.DefaultNetworkInstance(env.dut), ateLagAttrs.IPv6, true, protocolTimeout)
	cfgplugins.AwaitISISAdjacencyStatus(t, env.dut, isisInstance, env.aggID, 0, true, protocolTimeout)

	// RT-5.18.3 Step 6: Verify that bidirectional traffic continues to flow (0% steady-state loss)
	// over the remaining active member link (DUT Port-3).
	t.Logf("RT-5.18.3 Step 6: Verifying bidirectional traffic continues to flow at 0%% steady-state loss over remaining active member %s", p3.Name())
	cfgplugins.AwaitLACPMemberState(t, env.dut, env.aggID, []*ondatra.Port{p3}, stateTimeout)
	otgvalidationhelpers.WaitForOTGLAGUP(t, env.ate, otgvalidationhelpers.LagParams{
		LagName:       ateLagAttrs.Name,
		WantMembersUp: 1,
		MemberPortIDs: []string{env.ate.Port(t, p3.ID()).ID()},
		Timeout:       stateTimeout,
	})
	if !otgvalidationhelpers.AreFlowsTransmitting(t, env.ate, env.allFlows, 20, 8*time.Second) {
		restartTrafficAfterLAGChange(t, env, []*ondatra.Port{p3})
	}
	cfgplugins.AwaitInterfaceCountersIncrement(t, env.dut, p3.Name(), 10, trafficTimeout)
	for _, flow := range env.allFlows {
		otgvalidationhelpers.AwaitFlowSteadyState(t, env.ate, otgvalidationhelpers.SteadyStateFlowParams{
			FlowName:      flow,
			MinWindowPkts: 200,
			WantZeroLoss:  true,
			Timeout:       trafficTimeout,
		})
	}

	// RT-5.18.3 Step 7: Re-enable the disabled member links (DUT Port-1 and DUT Port-2) by setting
	// /interfaces/interface[name=<DUT Port-1|Port-2>]/config/enabled to true using gNMI.Set with REPLACE option,
	// verify their oper-status returns to UP, and confirm traffic remains at 0% loss across all 3 member links.
	t.Logf("RT-5.18.3 Step 7: Re-enabling %s and %s (enabled = true), verifying oper-status == UP, and confirming 0%% loss across all 3 member links", p1.Name(), p2.Name())
	restoreBatch := &gnmi.SetBatch{}
	cfgplugins.SetInterfaceAdminEnabled(t, env.dut, restoreBatch, p1.Name(), true)
	cfgplugins.SetInterfaceAdminEnabled(t, env.dut, restoreBatch, p2.Name(), true)
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
	for _, p := range env.memberPorts {
		cfgplugins.AwaitInterfaceCountersIncrement(t, env.dut, p.Name(), 10, trafficTimeout)
	}
	for _, flow := range env.allFlows {
		otgvalidationhelpers.AwaitFlowSteadyState(t, env.ate, otgvalidationhelpers.SteadyStateFlowParams{
			FlowName:      flow,
			MinWindowPkts: 200,
			WantZeroLoss:  true,
			Timeout:       trafficTimeout,
		})
	}
}

// testNegativeAdminDownAlreadyDisabledLAG implements RT-5.18.4 - Negative Test - Admin Down on Already Disabled LAG (Steps 1-6) as defined in README.md.
func testNegativeAdminDownAlreadyDisabledLAG(t *testing.T, env *testEnv) {
	// RT-5.18.4 Step 1: Administratively disable the LAG interface by setting
	// /interfaces/interface[name=Port-Channel1]/config/enabled to false using gNMI.Set with REPLACE option.
	t.Logf("RT-5.18.4 Step 1: Administratively disabling %s (enabled = false)", env.aggID)
	cfgplugins.SetInterfaceAdminEnabled(t, env.dut, nil, env.aggID, false)

	// RT-5.18.4 Step 2: Verify via gNMI that /interfaces/interface[name=Port-Channel1]/state/admin-status is DOWN,
	// /interfaces/interface[name=Port-Channel1]/state/oper-status is DOWN, and traffic loss across Port-Channel1 is 100%.
	t.Logf("RT-5.18.4 Step 2: Verifying %s admin-status == DOWN, oper-status == DOWN, and 100%% traffic loss", env.aggID)
	cfgplugins.AwaitInterfaceAdminAndOperStatus(t, env.dut, env.aggID, false, oc.Interface_AdminStatus_DOWN, oc.Interface_OperStatus_DOWN, stateTimeout)
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

	// RT-5.18.4 Step 3: Send a second idempotent gNMI.Set (REPLACE) setting
	// /interfaces/interface[name=Port-Channel1]/config/enabled to false on the already disabled Port-Channel1 interface.
	// RT-5.18.4 Step 4: Verify that the DUT accepts the redundant gNMI.Set configuration without returning any RPC errors.
	t.Logf("RT-5.18.4 Steps 3 & 4: Pushing second idempotent gNMI Replace with enabled = false on already disabled %s", env.aggID)
	cfgplugins.SetInterfaceAdminEnabled(t, env.dut, nil, env.aggID, false)

	// RT-5.18.4 Step 5: Verify that /interfaces/interface[name=Port-Channel1]/state/admin-status and
	// /interfaces/interface[name=Port-Channel1]/state/oper-status remain DOWN and traffic remains at 100% loss.
	t.Log("RT-5.18.4 Step 5: Verifying Port-Channel1 admin-status and oper-status remain DOWN, Port-4 remains UP, and traffic remains at 100% loss")
	cfgplugins.AwaitInterfaceAdminAndOperStatus(t, env.dut, env.aggID, false, oc.Interface_AdminStatus_DOWN, oc.Interface_OperStatus_DOWN, stateTimeout)
	cfgplugins.AwaitInterfaceAdminAndOperStatus(t, env.dut, env.port4.Name(), true, oc.Interface_AdminStatus_UP, oc.Interface_OperStatus_UP, stateTimeout)
	cfgplugins.AwaitISISAdjacencyStatus(t, env.dut, isisInstance, env.port4.Name(), 0, true, protocolTimeout)
	cfgplugins.AwaitBGPSessionStatus(t, env.dut, deviations.DefaultNetworkInstance(env.dut), atePort4Attrs.IPv4, true, protocolTimeout)
	cfgplugins.AwaitBGPSessionStatus(t, env.dut, deviations.DefaultNetworkInstance(env.dut), atePort4Attrs.IPv6, true, protocolTimeout)
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
	env.ate.OTG().StopTraffic(t)

	// RT-5.18.4 Step 6: Restore Port-Channel1 by setting /interfaces/interface[name=Port-Channel1]/config/enabled
	// to true using gNMI.Set with REPLACE option, and verify via gNMI that admin-status and oper-status return to UP,
	// eBGP (192.0.2.2, 2001:db8::2) and IS-IS sessions re-establish to ESTABLISHED/UP, BGP installed prefixes return
	// to 10000 (IPv4 and IPv6), and steady-state traffic loss returns to 0%.
	t.Logf("RT-5.18.4 Step 6: Re-enabling %s (enabled = true) and verifying recovery of interface, eBGP/IS-IS sessions, 10,000 prefixes, and 0%% traffic loss", env.aggID)
	cfgplugins.SetInterfaceAdminEnabled(t, env.dut, nil, env.aggID, true)
	cfgplugins.AwaitInterfaceAdminAndOperStatus(t, env.dut, env.aggID, true, oc.Interface_AdminStatus_UP, oc.Interface_OperStatus_UP, stateTimeout)
	cfgplugins.AwaitBGPSessionStatus(t, env.dut, deviations.DefaultNetworkInstance(env.dut), ateLagAttrs.IPv4, true, protocolTimeout)
	cfgplugins.AwaitBGPSessionStatus(t, env.dut, deviations.DefaultNetworkInstance(env.dut), ateLagAttrs.IPv6, true, protocolTimeout)
	cfgplugins.AwaitISISAdjacencyStatus(t, env.dut, isisInstance, env.aggID, 0, true, protocolTimeout)
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
// Inband BGP Reachability in Non-Default VRF (B2_CORE) Across Drain States (Steps 1-8) as defined in README.md.
func testUplinkLAGMemberRetentionAndB2CoreBGP(t *testing.T, env *testEnv) {
	// Capture resolved DUT interface MAC addresses for Port-Channel1 and Port-4 before reconfiguring VRF
	// so B2_CORE tagged IPv4 and IPv6 flows have explicit Ethernet destination MACs.
	dutLagMAC := otgvalidationhelpers.ResolveDUTInterfaceMAC(t, env.dut, env.ate, ateLagAttrs.Name+".Eth", dutLagAttrs.IPv4, dutLagAttrs.IPv6, env.aggID)
	if dutLagMAC == "" {
		dutLagMAC = env.dutLagMAC
	}
	dutPort4MAC := otgvalidationhelpers.ResolveDUTInterfaceMAC(t, env.dut, env.ate, atePort4Attrs.Name+".Eth", dutPort4Attrs.IPv4, dutPort4Attrs.IPv6, env.port4.Name())
	if dutPort4MAC == "" {
		dutPort4MAC = env.dutPort4MAC
	}
	t.Logf("Resolved DUT MACs for B2_CORE flows: %s=%s, %s=%s", env.aggID, dutLagMAC, env.port4.Name(), dutPort4MAC)

	// Cleanly remove DEFAULT VRF protocols and untagged Subinterface(0) addresses prior to configuring
	// single-tagged VLAN 100 subinterfaces in B2_CORE.
	cfgplugins.ClearDefaultVRFSubinterfacesAndProtocols(t, env.dut, isisInstance, dutAS, []string{env.aggID, env.port4.Name()})

	// RT-5.18.5 Step 1: Configure a non-default Layer 3 VRF network instance named B2_CORE and bind
	// Port-Channel1 subinterface 100 (Port-Channel1.100) and Port-4 subinterface 100 (Port-4.100) to B2_CORE:
	// - /network-instances/network-instance[name=B2_CORE]/config/name = B2_CORE
	// - /network-instances/network-instance[name=B2_CORE]/config/type = L3VRF
	// - /network-instances/network-instance[name=B2_CORE]/interfaces/interface[id=Port-Channel1.100]/config/{interface=Port-Channel1, subinterface=100}
	// - /network-instances/network-instance[name=B2_CORE]/interfaces/interface[id=Port-4.100]/config/{interface=Port-4, subinterface=100}
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

	// RT-5.18.5 Step 2: Configure single-tagged VLAN 100 and dual-stack Layer 3 addressing on
	// Port-Channel1 subinterface 100 (192.0.2.9/30, 2001:db8::9/126) and Port-4 subinterface 100 (192.0.2.13/30, 2001:db8::d/126).
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

	// RT-5.18.5 Step 3: Configure uplink LAG Port-Channel1 and ensure all 3 uplink member ports (Port-1, Port-2, and Port-3)
	// retain their LAG membership and forwarding viability in the drained/initial uplink state:
	// - /interfaces/interface[name=Port-Channel1]/aggregation/config/lag-type = LACP
	// - /lacp/interfaces/interface[name=Port-Channel1]/config/lacp-mode = ACTIVE
	// - /lacp/interfaces/interface[name=Port-Channel1]/config/interval = FAST
	// - /interfaces/interface[name=<Port-1..Port-3>]/config/enabled = true
	// - /interfaces/interface[name=<Port-1..Port-3>]/ethernet/config/aggregate-id = Port-Channel1
	// - /interfaces/interface[name=<Port-1..Port-3>]/config/forwarding-viable = true
	t.Logf("RT-5.18.5 Step 3: Ensuring uplink LAG %s (LACP, ACTIVE, FAST) and member ports Port-1..Port-3 retain forwarding-viable = true and aggregate-id = %s", env.aggID, env.aggID)
	viableTrue := true
	lagBatch := &gnmi.SetBatch{}
	cfgplugins.ConfigureL3AggregateInterface(t, env.dut, lagBatch, cfgplugins.L3AggregateParams{
		AggID:            env.aggID,
		MemberPorts:      env.memberPorts,
		LagType:          oc.IfAggregate_AggregationType_LACP,
		LacpMode:         oc.Lacp_LacpActivityType_ACTIVE,
		LacpInterval:     oc.Lacp_LacpPeriodType_FAST,
		MinLinks:         1,
		ForwardingViable: &viableTrue,
	})
	lagBatch.Set(t, env.dut)

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

	// Configure and push OTG VLAN 100 subinterfaces, eBGP peers, and B2_CORE traffic flows on ATE.
	env.ate.OTG().StopProtocols(t)
	configureB2CoreOTG(t, env, dutLagMAC, dutPort4MAC)
	env.ate.OTG().PushConfig(t, env.otgConfig)
	env.ate.OTG().StartProtocols(t)

	// RT-5.18.5 Step 5: Verify via gNMI Subscribe (Watch with Await) that all 3 uplink member ports
	// (Port-1, Port-2, and Port-3) remain bound to Port-Channel1 and actively participating in LACP:
	// - /interfaces/interface[name=<Port-1..Port-3>]/ethernet/state/aggregate-id == Port-Channel1
	// - /interfaces/interface[name=<Port-1..Port-3>]/state/forwarding-viable == true
	// - /interfaces/interface[name=<Port-1..Port-3>]/state/oper-status == UP and /interfaces/interface[name=Port-Channel1]/state/oper-status == UP
	// - /lacp/interfaces/interface[name=Port-Channel1]/members/member[interface=<Port-1..Port-3>]/state/{activity=ACTIVE, aggregatable=true, collecting=true, distributing=true, synchronization=IN_SYNC}.
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

	// RT-5.18.5 Step 6: Verify via gNMI Watch (Await) that all 4 eBGP sessions in the B2_CORE network instance
	// (192.0.2.10, 2001:db8::a, 192.0.2.14, and 2001:db8::e) reach ESTABLISHED and that installed prefixes reach 1
	// for both IPV4_UNICAST and IPV6_UNICAST.
	t.Logf("RT-5.18.5 Step 6: Verifying all 4 eBGP sessions in %s reach ESTABLISHED and prefixes are installed", b2CoreVRFName)
	for _, peer := range []string{ateLagVlan100IPv4, ateLagVlan100IPv6, atePort4Vlan100IPv4, atePort4Vlan100IPv6} {
		cfgplugins.AwaitBGPSessionStatus(t, env.dut, b2CoreVRFName, peer, true, protocolTimeout)
	}
	cfgplugins.AwaitBGPPrefixes(t, env.dut, b2CoreVRFName, ateLagVlan100IPv4, oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST, b2CoreLagBGPV4Count, b2CoreLagBGPV4Count, protocolTimeout)
	cfgplugins.AwaitBGPPrefixes(t, env.dut, b2CoreVRFName, ateLagVlan100IPv6, oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST, b2CoreLagBGPV6Count, b2CoreLagBGPV6Count, protocolTimeout)
	cfgplugins.AwaitBGPPrefixes(t, env.dut, b2CoreVRFName, atePort4Vlan100IPv4, oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST, b2CorePort4BGPV4Count, b2CorePort4BGPV4Count, protocolTimeout)
	cfgplugins.AwaitBGPPrefixes(t, env.dut, b2CoreVRFName, atePort4Vlan100IPv6, oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST, b2CorePort4BGPV6Count, b2CorePort4BGPV6Count, protocolTimeout)

	// RT-5.18.5 Step 7: Confirm IPv4 ARP and IPv6 ND neighbor resolution on all ATE VLAN 100 interfaces,
	// then start bidirectional IPv4 and IPv6 traffic flows in the B2_CORE VRF between ATE Port-4 (VLAN 100)
	// and ATE Port-Channel1 (VLAN 100).
	t.Log("RT-5.18.5 Step 7: Confirming IPv4 ARP and IPv6 ND resolution on VLAN 100 interfaces and starting B2_CORE bidirectional IPv4 and IPv6 flows")
	b2CoreGateways := []otgvalidationhelpers.SubinterfaceGatewayNeighbor{
		{OTGEthName: "ateLagVlan100.Eth", IPv4GW: dutLagVlan100IPv4, IPv6GW: dutLagVlan100IPv6},
		{OTGEthName: "atePort4Vlan100.Eth", IPv4GW: dutPort4Vlan100IPv4, IPv6GW: dutPort4Vlan100IPv6},
	}
	otgvalidationhelpers.WaitForSubinterfaceARPAndND(t, env.ate, b2CoreGateways)
	env.ate.OTG().StartTraffic(t)

	// RT-5.18.5 Step 8: Verify via gNMI counter Watch that steady-state packet loss is 0% for all four IPv4 and IPv6 flows
	// and that egress traffic on Port-Channel1 is distributed across all 3 retained member ports (Port-1, Port-2, and Port-3).
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

	// Replace OTG flows with the 4 B2_CORE VLAN 100 bidirectional flows for RT-5.18.5 Step 7 and RT-5.18.6:
	// - Forward IPv4 (Port-4.100 -> Port-Channel1.100): Src 198.51.100.1 -> Dst 203.0.113.1 (VLAN 100)
	// - Reverse IPv4 (Port-Channel1.100 -> Port-4.100): Src 203.0.113.1 -> Dst 198.51.100.1 (VLAN 100)
	// - Forward IPv6 (Port-4.100 -> Port-Channel1.100): Src 2001:db8:3000::1 -> Dst 2001:db8:4000::1 (VLAN 100)
	// - Reverse IPv6 (Port-Channel1.100 -> Port-4.100): Src 2001:db8:4000::1 -> Dst 2001:db8:3000::1 (VLAN 100)
	env.otgConfig.Flows().Clear()
	otgconfighelpers.AddContinuousUDPFlow(env.otgConfig, otgconfighelpers.ContinuousUDPFlowParams{
		FlowName:     flowB2CoreIPv4Fwd,
		TxEndpoint:   p4VlanIPv4.Name(),
		RxEndpoint:   lagVlanIPv4.Name(),
		SrcMAC:       atePort4Attrs.MAC,
		DstMAC:       dutPort4MAC,
		VlanID:       b2CoreVlanID,
		SrcIP:        b2CorePort4BGPV4FirstIP,
		DstIP:        b2CoreLagBGPV4FirstIP,
		IPStep:       "0.0.0.1",
		IPCount:      64,
		IncrementSrc: true,
		IncrementDst: true,
		FrameSize:    flowFrameSize,
		PpsRate:      flowPPS,
	})
	otgconfighelpers.AddContinuousUDPFlow(env.otgConfig, otgconfighelpers.ContinuousUDPFlowParams{
		FlowName:     flowB2CoreIPv4Rev,
		TxEndpoint:   lagVlanIPv4.Name(),
		RxEndpoint:   p4VlanIPv4.Name(),
		SrcMAC:       ateLagAttrs.MAC,
		DstMAC:       dutLagMAC,
		VlanID:       b2CoreVlanID,
		SrcIP:        b2CoreLagBGPV4FirstIP,
		DstIP:        b2CorePort4BGPV4FirstIP,
		IPStep:       "0.0.0.1",
		IPCount:      64,
		IncrementSrc: true,
		IncrementDst: true,
		FrameSize:    flowFrameSize,
		PpsRate:      flowPPS,
	})
	otgconfighelpers.AddContinuousUDPFlow(env.otgConfig, otgconfighelpers.ContinuousUDPFlowParams{
		FlowName:     flowB2CoreIPv6Fwd,
		TxEndpoint:   p4VlanIPv6.Name(),
		RxEndpoint:   lagVlanIPv6.Name(),
		SrcMAC:       atePort4Attrs.MAC,
		DstMAC:       dutPort4MAC,
		VlanID:       b2CoreVlanID,
		IsIPv6:       true,
		SrcIP:        b2CorePort4BGPV6FirstIP,
		DstIP:        b2CoreLagBGPV6FirstIP,
		IPStep:       "::1",
		IPCount:      64,
		IncrementSrc: true,
		IncrementDst: true,
		FrameSize:    flowFrameSize,
		PpsRate:      flowPPS,
	})
	otgconfighelpers.AddContinuousUDPFlow(env.otgConfig, otgconfighelpers.ContinuousUDPFlowParams{
		FlowName:     flowB2CoreIPv6Rev,
		TxEndpoint:   lagVlanIPv6.Name(),
		RxEndpoint:   p4VlanIPv6.Name(),
		SrcMAC:       ateLagAttrs.MAC,
		DstMAC:       dutLagMAC,
		VlanID:       b2CoreVlanID,
		IsIPv6:       true,
		SrcIP:        b2CoreLagBGPV6FirstIP,
		DstIP:        b2CorePort4BGPV6FirstIP,
		IPStep:       "::1",
		IPCount:      64,
		IncrementSrc: true,
		IncrementDst: true,
		FrameSize:    flowFrameSize,
		PpsRate:      flowPPS,
	})
}

// testDifferentialMemberDrainForwardingViable implements RT-5.18.6 - Differential Member Drain Behavior:
// Standard Member Drain (forwarding-viable: false) vs. Retained Uplink LAG Members (Steps 1-5) as defined in README.md.
func testDifferentialMemberDrainForwardingViable(t *testing.T, env *testEnv) {
	p1 := env.memberPorts[0]
	p2 := env.memberPorts[1]
	p3 := env.memberPorts[2]
	activeMembers := []*ondatra.Port{p2, p3}
	b2CoreGateways := []otgvalidationhelpers.SubinterfaceGatewayNeighbor{
		{OTGEthName: "ateLagVlan100.Eth", IPv4GW: dutLagVlan100IPv4, IPv6GW: dutLagVlan100IPv6},
		{OTGEthName: "atePort4Vlan100.Eth", IPv4GW: dutPort4Vlan100IPv4, IPv6GW: dutPort4Vlan100IPv6},
	}

	// RT-5.18.6 Step 1: Starting from the converged B2_CORE VRF topology in RT-5.18.5, apply a member drain state
	// to Port-1 while retaining Port-2 and Port-3 as active forwarding uplink members via gNMI.Set:
	// - /interfaces/interface[name=Port-1]/config/forwarding-viable = false
	// - /interfaces/interface[name=<Port-1..Port-3>]/ethernet/config/aggregate-id = Port-Channel1
	// - /interfaces/interface[name=<Port-2|Port-3>]/config/forwarding-viable = true
	t.Logf("RT-5.18.6 Step 1: Setting %s config/forwarding-viable = false while retaining %s and %s with forwarding-viable = true",
		p1.Name(), p2.Name(), p3.Name())
	drainBatch := &gnmi.SetBatch{}
	cfgplugins.SetMemberForwardingViable(t, env.dut, drainBatch, p1.Name(), env.aggID, false)
	cfgplugins.SetMemberForwardingViable(t, env.dut, drainBatch, p2.Name(), env.aggID, true)
	cfgplugins.SetMemberForwardingViable(t, env.dut, drainBatch, p3.Name(), env.aggID, true)
	drainBatch.Set(t, env.dut)

	// RT-5.18.6 Step 2: Verify via gNMI Subscribe (Watch with Await) the differential member telemetry state:
	// - /interfaces/interface[name=Port-1]/state/forwarding-viable transitions to false
	// - /interfaces/interface[name=<Port-2|Port-3>]/state/forwarding-viable remains true
	// - /interfaces/interface[name=<Port-1..Port-3>]/ethernet/state/aggregate-id remains Port-Channel1.
	t.Log("RT-5.18.6 Step 2: Verifying Port-1 state/forwarding-viable == false, Port-2 & Port-3 state/forwarding-viable == true, and aggregate-id retained")
	cfgplugins.AwaitMemberForwardingViableAndAggregateID(t, env.dut, p1.Name(), env.aggID, false, stateTimeout)
	cfgplugins.AwaitMemberForwardingViableAndAggregateID(t, env.dut, p2.Name(), env.aggID, true, stateTimeout)
	cfgplugins.AwaitMemberForwardingViableAndAggregateID(t, env.dut, p3.Name(), env.aggID, true, stateTimeout)

	// RT-5.18.6 Step 3: Verify via gNMI that the dual-stack eBGP sessions in B2_CORE (192.0.2.10 and 2001:db8::a)
	// remain ESTABLISHED without flapping:
	// - /network-instances/network-instance[name=B2_CORE]/protocols/protocol[identifier=BGP][name=BGP]/bgp/neighbors/neighbor[neighbor-address=<192.0.2.10|2001:db8::a>]/state/session-state
	t.Logf("RT-5.18.6 Step 3: Verifying inband eBGP sessions in %s (%s and %s) remain ESTABLISHED", b2CoreVRFName, ateLagVlan100IPv4, ateLagVlan100IPv6)
	cfgplugins.AwaitBGPSessionStatus(t, env.dut, b2CoreVRFName, ateLagVlan100IPv4, true, protocolTimeout)
	cfgplugins.AwaitBGPSessionStatus(t, env.dut, b2CoreVRFName, ateLagVlan100IPv6, true, protocolTimeout)

	// RT-5.18.6 Step 4: Send continuous bidirectional IPv4 and IPv6 traffic in the B2_CORE VRF between
	// ATE Port-4.100 and ATE Port-Channel1.100 and verify via gNMI interface counter Watch:
	// - DUT Port-1 transmits zero egress data-plane traffic (/interfaces/interface[name=Port-1]/state/counters/out-unicast-pkts rate is 0)
	// - Port-2 and Port-3 carry 100% of the egress traffic from the DUT
	// - Steady-state bidirectional IPv4 and IPv6 traffic loss across B2_CORE is 0%.
	t.Log("RT-5.18.6 Step 4: Verifying Port-1 out-unicast-pkts rate drops to 0 while Port-2 & Port-3 carry 100% of egress traffic with 0% steady-state loss")
	env.ate.OTG().StopTraffic(t)
	otgvalidationhelpers.WaitForSubinterfaceARPAndND(t, env.ate, b2CoreGateways)
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

	// RT-5.18.6 Step 5: Restore Port-1 to the retained Access Core uplink state via gNMI.Set and verify recovery:
	// - Set /interfaces/interface[name=Port-1]/config/forwarding-viable = true
	// - Verify /interfaces/interface[name=Port-1]/state/forwarding-viable returns to true
	// - Confirm egress traffic resumes across all 3 member links (Port-1, Port-2, and Port-3) with 0% steady-state loss.
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
	otgvalidationhelpers.WaitForSubinterfaceARPAndND(t, env.ate, b2CoreGateways)
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
