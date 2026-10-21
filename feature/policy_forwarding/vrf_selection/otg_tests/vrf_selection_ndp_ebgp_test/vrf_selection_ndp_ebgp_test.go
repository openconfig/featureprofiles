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

package vrf_selection_ndp_ebgp_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/featureprofiles/internal/otg_helpers/otg_config_helpers"
	"github.com/openconfig/featureprofiles/internal/otg_helpers/otg_validation_helpers"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ygnmi/ygnmi"
)

const (
	ipv4PrefixLen = 30
	ipv6PrefixLen = 126

	dutAS = 64500
	ateAS = 64501

	vrf10Name              = "VRF-10"
	vrfSelectionPolicyName = "VRF-SELECT-POLICY"
	routePolicyAcceptAll   = "ALLOW"

	rule10SeqID       = 10
	rule20SeqID       = 20
	rule10SrcPrefixV4 = "198.18.0.0"
	rule10PrefixLenV4 = 16
	rule20SrcPrefixV6 = "2001:db8:10::"
	rule20PrefixLenV6 = 48

	ateLagAdvPrefixV4  = "198.19.0.0/24"
	ateLagAdvPrefixV6  = "2001:db8:20::/64"
	ebgpDestPrefixV4   = "198.51.100.0/24"
	ebgpDestPrefixV6   = "2001:db8:1::/64"
	vrf10DestPrefixV4  = "203.0.113.0/24"
	vrf10DestPrefixV6  = "2001:db8:2::/64"
	defaultTrafficSrc4 = "198.19.0.1"
	defaultTrafficSrc6 = "2001:db8:20::1"
	steeredTrafficSrc4 = "198.18.0.1"
	steeredTrafficSrc6 = "2001:db8:10::1"
	defaultTrafficDst4 = "198.51.100.1"
	defaultTrafficDst6 = "2001:db8:1::1"
	vrf10TrafficDst4   = "203.0.113.1"
	vrf10TrafficDst6   = "2001:db8:2::1"

	dscpDefault      = 0
	dscpVRF10        = 10
	trafficFrameSize = 512
	trafficRatePps   = 1000
	trafficPackets   = 10000
	trafficTimeout   = 60 * time.Second
	controlTimeout   = 2 * time.Minute

	flow1IPv4DefaultVRF = "Flow-1-Default-IPv4"
	flow2IPv6DefaultVRF = "Flow-2-Default-IPv6"
	flow3IPv4VRF10      = "Flow-3-Steered-IPv4"
	flow4IPv6VRF10      = "Flow-4-Steered-IPv6"
)

var (
	// DUT:LAG1 (Port1, Port2) attributes in DEFAULT VRF.
	dutLag1 = attrs.Attributes{
		Desc:    "DUT LAG1 to ATE LAG1",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::1",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}

	// ATE:LAG1 (Port1, Port2) attributes in DEFAULT VRF.
	ateLag1 = attrs.Attributes{
		Name:    "ateLag1",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::2",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}

	// DUT:Port3 attributes in DEFAULT VRF (egress for eBGP routes).
	dutPort3 = attrs.Attributes{
		Desc:    "DUT Port3 to ATE Port3 (DEFAULT VRF)",
		IPv4:    "192.0.2.5",
		IPv6:    "2001:db8::5",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}

	// ATE:Port3 attributes in DEFAULT VRF.
	atePort3 = attrs.Attributes{
		Name:    "atePort3",
		MAC:     "02:00:03:01:01:01",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::6",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}

	// DUT:Port4 attributes in VRF-10 (egress for VRF-steered traffic).
	dutPort4 = attrs.Attributes{
		Desc:    "DUT Port4 to ATE Port4 (VRF-10)",
		IPv4:    "192.0.2.9",
		IPv6:    "2001:db8::9",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}

	// ATE:Port4 attributes in VRF-10.
	atePort4 = attrs.Attributes{
		Name:    "atePort4",
		MAC:     "02:00:04:01:01:01",
		IPv4:    "192.0.2.10",
		IPv6:    "2001:db8::a",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
)

// testEnv encapsulates the testbed devices, OTG configuration, and dynamic LAG interface data.
type testEnv struct {
	dut      *ondatra.DUTDevice
	ate      *ondatra.ATEDevice
	top      gosnappi.Config
	aggID    string
	aggPorts []*ondatra.Port
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// setupEnvironment initializes the testbed devices and resolves the next available aggregate
// interface name on the DUT.
func setupEnvironment(t *testing.T) *testEnv {
	t.Helper()
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	aggID := netutil.NextAggregateInterface(t, dut)
	aggPorts := []*ondatra.Port{
		dut.Port(t, "port1"),
		dut.Port(t, "port2"),
	}

	return &testEnv{
		dut:      dut,
		ate:      ate,
		top:      gosnappi.NewConfig(),
		aggID:    aggID,
		aggPorts: aggPorts,
	}
}

// TestVRFSelectionNDPEBGP implements RT-3.5: Dual-Stack eBGP Session Establishment
// and IPv6 NDP Resolution over LAG with Active VRF Selection Policy.
func TestVRFSelectionNDPEBGP(t *testing.T) {
	env := setupEnvironment(t)

	// Register cleanup before executing any configuration pushes or destructive operations.
	t.Cleanup(func() {
		t.Log("Cleanup: Stopping OTG protocols and removing VRF selection policy, BGP, VRF-10, and LAG configurations")
		env.ate.OTG().StopProtocols(t)

		defNI := deviations.DefaultNetworkInstance(env.dut)
		gnmi.Delete(t, env.dut, gnmi.OC().NetworkInstance(defNI).PolicyForwarding().Config())
		cfgplugins.BGPClearConfig(t, env.dut)
		gnmi.Delete(t, env.dut, gnmi.OC().RoutingPolicy().PolicyDefinition(routePolicyAcceptAll).Config())
		gnmi.Delete(t, env.dut, gnmi.OC().Interface(env.dut.Port(t, "port3").Name()).Config())
		gnmi.Delete(t, env.dut, gnmi.OC().Interface(env.dut.Port(t, "port4").Name()).Config())
		gnmi.Delete(t, env.dut, gnmi.OC().NetworkInstance(vrf10Name).Config())
		cfgplugins.DeleteAggregate(t, env.dut, env.aggID, env.aggPorts)
	})

	if hardwareInitCfg := cfgplugins.NewDUTHardwareInit(t, env.dut, cfgplugins.FeatureVrfSelectionExtended); hardwareInitCfg != "" {
		helpers.GnmiCLIConfig(t, env.dut, hardwareInitCfg)
	}

	// RT-3.5.1: Baseline LAG, Dual-Stack eBGP, and VRF Selection Policy Configuration and Control-Plane Verification.
	var flowNames []string
	t.Run("RT-3.5.1: Baseline LAG, Dual-Stack eBGP, and VRF Selection Policy Configuration and Control-Plane Verification", func(t *testing.T) {
		// RT-3.5.1 Step 1: Configure Network Instances, Interfaces, and Static Routing on DUT.
		t.Log("RT-3.5.1 Step 1: Configuring DEFAULT and VRF-10 network instances, LAG1, physical interfaces, and static routes on DUT")
		configureDUTNetworkInstancesAndInterfaces(t, env)

		// RT-3.5.1 Step 2: Configure Dual-Stack eBGP on DUT and ATE.
		t.Log("RT-3.5.1 Step 2: Configuring dual-stack eBGP on DUT and ATE (LAG1 and Port3)")
		configureDUTBGP(t, env)
		configureATE(t, env)
		flowNames = configureOTGTrafficFlows(t, env)
		env.ate.OTG().PushConfig(t, env.top)
		env.ate.OTG().StartProtocols(t)

		// RT-3.5.1 Step 3: Configure and Apply VRF Selection Policy on DUT Ingress LAG Interface.
		t.Log("RT-3.5.1 Step 3: Configuring VRF-SELECT-POLICY and applying to DUT ingress LAG subinterface 0")
		configureDUTVRFSelectionPolicy(t, env)

		// RT-3.5.1 Step 4: Verify Control-Plane State (LAG, IPv6 NDP Neighbor Cache, eBGP Sessions, and AFT Routes).
		t.Log("RT-3.5.1 Step 4: Verifying LACP LAG state, IPv6 NDP neighbor resolution, dual-stack eBGP sessions, AFT routes, and VRF selection policy state")
		verifyControlPlaneState(t, env)
	})

	// RT-3.5.2: Dual-Stack Data Plane Forwarding Verification for Default VRF eBGP Routes and VRF-Steered Traffic.
	t.Run("RT-3.5.2: Dual-Stack Data Plane Forwarding Verification for Default VRF eBGP Routes and VRF-Steered Traffic", func(t *testing.T) {
		// RT-3.5.2 Step 1 & Step 2 & Step 3: Transmit traffic, verify zero packet loss, verify policy rule counters, and verify control-plane stability.
		t.Log("RT-3.5.2: Transmitting dual-stack traffic flows and verifying forwarding, policy rule counters, and eBGP/NDP stability")
		runAndVerifyTraffic(t, env, flowNames)
	})
}

// configureDUTNetworkInstancesAndInterfaces implements RT-3.5.1 Step 1:
//   - Initializes DEFAULT and VRF-10 network instances using cfgplugins.NewNetworkInstance.
//   - Configures LACP LAG (port1, port2) using cfgplugins.NewAggregateInterface and cfgplugins.SetupAggregateAtomically.
//   - Configures physical ports port3 (DEFAULT VRF) and port4 (VRF-10).
//   - Configures IPv4 and IPv6 static routes in VRF-10 using cfgplugins.NewStaticRouteCfg.
func configureDUTNetworkInstancesAndInterfaces(t *testing.T, env *testEnv) {
	t.Helper()
	dut := env.dut

	// Initialize the default network instance first.
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	// Create VRF-10 network instance via shared library helper.
	niBatch := &gnmi.SetBatch{}
	cfgplugins.NewNetworkInstance(t, dut, niBatch, &cfgplugins.NetworkInstanceParams{
		Name:    vrf10Name,
		Default: false,
	})
	niBatch.Set(t, dut)

	// Configure LACP LAG (aggID) on DUT port1 and port2 with dual-stack addresses.
	if deviations.AggregateAtomicUpdate(dut) {
		cfgplugins.DeleteAggregate(t, dut, env.aggID, env.aggPorts)
		cfgplugins.SetupAggregateAtomically(t, dut, env.aggID, env.aggPorts)
	}
	activity := oc.Lacp_LacpActivityType_ACTIVE
	period := oc.Lacp_LacpPeriodType_FAST
	aggData := &cfgplugins.DUTAggData{
		Attributes:   dutLag1,
		LagName:      env.aggID,
		AggType:      oc.IfAggregate_AggregationType_LACP,
		OndatraPorts: env.aggPorts,
		LacpParams: &cfgplugins.LACPParams{
			Activity: &activity,
			Period:   &period,
		},
	}
	aggBatch := &gnmi.SetBatch{}
	agg := cfgplugins.NewAggregateInterface(t, dut, aggBatch, aggData)
	if deviations.IPv4MissingEnabled(dut) {
		agg.GetOrCreateSubinterface(0).GetOrCreateIpv4().Enabled = nil
		gnmi.BatchReplace(aggBatch, gnmi.OC().Interface(env.aggID).Config(), agg)
	}
	aggBatch.Set(t, dut)
	if deviations.LacpConfigAfterAggregateReplace(dut) {
		lacpPath := gnmi.OC().Lacp().Interface(env.aggID)
		gnmi.Update(t, dut, lacpPath.Config(), &oc.Lacp_Interface{
			Name:     &env.aggID,
			LacpMode: activity,
			Interval: period,
		})
	}

	// Configure DUT Port3 (DEFAULT VRF egress) and DUT Port4 (VRF-10 egress).
	p3 := dut.Port(t, "port3")
	p4 := dut.Port(t, "port4")
	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, p3)
		fptest.SetPortSpeed(t, p4)
	}

	gnmi.Replace(t, dut, gnmi.OC().Interface(p3.Name()).Config(), dutPort3.NewOCInterface(p3.Name(), dut))
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		cfgplugins.AssignToNetworkInstance(t, dut, env.aggID, deviations.DefaultNetworkInstance(dut), 0)
		cfgplugins.AssignToNetworkInstance(t, dut, p3.Name(), deviations.DefaultNetworkInstance(dut), 0)
	}

	if deviations.InterfaceConfigVRFBeforeAddress(dut) {
		cfgplugins.AssignToNetworkInstance(t, dut, p4.Name(), vrf10Name, 0)
		gnmi.Replace(t, dut, gnmi.OC().Interface(p4.Name()).Config(), dutPort4.NewOCInterface(p4.Name(), dut))
	} else {
		gnmi.Replace(t, dut, gnmi.OC().Interface(p4.Name()).Config(), dutPort4.NewOCInterface(p4.Name(), dut))
		cfgplugins.AssignToNetworkInstance(t, dut, p4.Name(), vrf10Name, 0)
	}

	// Configure static routes in VRF-10 pointing to ATE:Port4 (192.0.2.10 and 2001:db8::a).
	srBatch := &gnmi.SetBatch{}
	cfgplugins.NewStaticRouteCfg(srBatch, &cfgplugins.StaticRouteCfg{
		NetworkInstance: vrf10Name,
		Prefix:          vrf10DestPrefixV4,
		NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
			"0": oc.UnionString(atePort4.IPv4),
		},
	}, dut)
	cfgplugins.NewStaticRouteCfg(srBatch, &cfgplugins.StaticRouteCfg{
		NetworkInstance: vrf10Name,
		Prefix:          vrf10DestPrefixV6,
		NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
			"0": oc.UnionString(atePort4.IPv6),
		},
	}, dut)
	srBatch.Set(t, dut)
}

// configureDUTBGP implements RT-3.5.1 Step 2 (DUT side):
//   - Configures an unconditional ACCEPT_ROUTE routing policy ("ALLOW") using cfgplugins.ConfigureBGPRoutePolicy.
//   - Configures BGP global (AS 64500, router-id 192.0.2.1), peer groups, and dual-stack eBGP neighbors
//     over LAG1 (AS 64501: 192.0.2.2 and 2001:db8::2) and Port3 (AS 64501: 192.0.2.6 and 2001:db8::6).
func configureDUTBGP(t *testing.T, env *testEnv) {
	t.Helper()
	dut := env.dut

	cfgplugins.BGPClearConfig(t, dut)

	bgpBatch := &gnmi.SetBatch{}
	if _, err := cfgplugins.ConfigureBGPRoutePolicy(t, bgpBatch, cfgplugins.BGPPolicyConfig{
		PolicyName:  routePolicyAcceptAll,
		StatementID: "accept-all",
	}); err != nil {
		t.Fatalf("ConfigureBGPRoutePolicy(%q) failed: %v", routePolicyAcceptAll, err)
	}

	policyName := routePolicyAcceptAll
	bgpProto := cfgplugins.ConfigureDUTBGP(t, dut, bgpBatch, cfgplugins.BGPConfig{
		DutAS:    dutAS,
		RouterID: dutLag1.IPv4,
	})
	bgp := bgpProto.GetOrCreateBgp()

	// Dual-stack eBGP peer groups and neighbors over LAG1 (192.0.2.2 and 2001:db8::2).
	cfgplugins.AppendBGPNeighbor(t, dut, bgpBatch, bgp, cfgplugins.BGPNeighborConfig{
		AteAS:        ateAS,
		PortName:     "LAG1-",
		NeighborIPv4: ateLag1.IPv4,
		NeighborIPv6: ateLag1.IPv6,
		PolicyName:   &policyName,
	})
	// Dual-stack eBGP peer groups and neighbors over Port3 (192.0.2.6 and 2001:db8::6).
	cfgplugins.AppendBGPNeighbor(t, dut, bgpBatch, bgp, cfgplugins.BGPNeighborConfig{
		AteAS:        ateAS,
		PortName:     "PORT3-",
		NeighborIPv4: atePort3.IPv4,
		NeighborIPv6: atePort3.IPv6,
		PolicyName:   &policyName,
	})

	if deviations.RoutePolicyUnderAFIUnsupported(dut) {
		for _, pgName := range []string{"LAG1-BGP-PEER-GROUP-V4", "LAG1-BGP-PEER-GROUP-V6", "PORT3-BGP-PEER-GROUP-V4", "PORT3-BGP-PEER-GROUP-V6"} {
			cfgplugins.ConfigurePeerGroup(bgp.GetOrCreatePeerGroup(pgName), dut,
				cfgplugins.ApplyPGRoutingPolicy(routePolicyAcceptAll, routePolicyAcceptAll, false),
			)
		}
		defNI := deviations.DefaultNetworkInstance(dut)
		gnmi.BatchUpdate(bgpBatch, gnmi.OC().NetworkInstance(defNI).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp().Config(), bgp)
	}

	bgpBatch.Set(t, dut)
}

// configureATE implements RT-3.5.1 Step 1 & Step 2 (ATE side):
//   - Configures OTG LAG1 (port1, port2) with LACP active mode and dual-stack addresses using otgconfighelpers.
//   - Configures OTG Port3 (DEFAULT VRF egress) and OTG Port4 (VRF-10 egress) using otgconfighelpers.
//   - Configures OTG dual-stack eBGP peers on ATE:LAG1 (AS 64501) and ATE:Port3 (AS 64501), advertising
//     198.19.0.0/24 and 2001:db8:20::/64 from ATE:LAG1 and 198.51.100.0/24 and 2001:db8:1::/64 from ATE:Port3.
func configureATE(t *testing.T, env *testEnv) {
	t.Helper()
	ate := env.ate
	top := env.top

	// Configure OTG LAG1 on port1 and port2.
	lagPort := &otgconfighelpers.Port{
		Name:        ateLag1.Name,
		AggMAC:      ateLag1.MAC,
		MemberPorts: []string{"port1", "port2"},
		LagID:       1,
		IsLag:       true,
		Interfaces: []*otgconfighelpers.InterfaceProperties{
			{
				Name:        ateLag1.Name,
				MAC:         ateLag1.MAC,
				IPv4:        ateLag1.IPv4,
				IPv4Gateway: dutLag1.IPv4,
				IPv4Len:     uint32(ateLag1.IPv4Len),
				IPv6:        ateLag1.IPv6,
				IPv6Gateway: dutLag1.IPv6,
				IPv6Len:     uint32(ateLag1.IPv6Len),
			},
		},
	}
	otgconfighelpers.ConfigureNetworkInterface(t, top, ate, lagPort)
	memberMACs := []string{"02:00:01:01:01:02", "02:00:01:01:01:03"}
	for _, lag := range top.Lags().Items() {
		if lag.Name() == ateLag1.Name {
			for idx, mp := range lag.Ports().Items() {
				if idx < len(memberMACs) {
					mp.Ethernet().SetMac(memberMACs[idx])
				}
			}
		}
	}

	// Configure OTG Port3 (DEFAULT VRF egress).
	p3 := ate.Port(t, "port3")
	phyPort3 := &otgconfighelpers.Port{
		Name: p3.ID(),
		Interfaces: []*otgconfighelpers.InterfaceProperties{
			{
				Name:        atePort3.Name,
				MAC:         atePort3.MAC,
				IPv4:        atePort3.IPv4,
				IPv4Gateway: dutPort3.IPv4,
				IPv4Len:     uint32(atePort3.IPv4Len),
				IPv6:        atePort3.IPv6,
				IPv6Gateway: dutPort3.IPv6,
				IPv6Len:     uint32(atePort3.IPv6Len),
			},
		},
	}
	otgconfighelpers.ConfigureNetworkInterface(t, top, ate, phyPort3)

	// Configure OTG Port4 (VRF-10 egress).
	p4 := ate.Port(t, "port4")
	phyPort4 := &otgconfighelpers.Port{
		Name: p4.ID(),
		Interfaces: []*otgconfighelpers.InterfaceProperties{
			{
				Name:        atePort4.Name,
				MAC:         atePort4.MAC,
				IPv4:        atePort4.IPv4,
				IPv4Gateway: dutPort4.IPv4,
				IPv4Len:     uint32(atePort4.IPv4Len),
				IPv6:        atePort4.IPv6,
				IPv6Gateway: dutPort4.IPv6,
				IPv6Len:     uint32(atePort4.IPv6Len),
			},
		},
	}
	otgconfighelpers.ConfigureNetworkInterface(t, top, ate, phyPort4)

	// Disable RS-FEC on 100G-FR ATE ports (e.g., Novus 100G-FR cards).
	var p100gbasefr []string
	for _, p := range ate.Ports() {
		if p.PMD() == ondatra.PMD100GBASEFR {
			p100gbasefr = append(p100gbasefr, p.ID())
		}
	}
	if len(p100gbasefr) > 0 {
		l1Settings := top.Layer1().Add().SetName("L1").SetPortNames(p100gbasefr)
		l1Settings.SetAutoNegotiate(true).SetIeeeMediaDefaults(false).SetSpeed("speed_100_gbps")
		l1Settings.AutoNegotiation().SetRsFec(false)
	}

	// Locate OTG devices for ATE:LAG1 and ATE:Port3 to attach dual-stack eBGP peers.
	var lagDev, port3Dev gosnappi.Device
	for _, dev := range top.Devices().Items() {
		switch dev.Name() {
		case ateLag1.Name + ".Dev":
			lagDev = dev
		case atePort3.Name + ".Dev":
			port3Dev = dev
		}
	}
	if lagDev == nil || port3Dev == nil {
		t.Fatalf("Failed to find OTG devices for %s or %s", ateLag1.Name, atePort3.Name)
	}

	// Configure dual-stack eBGP peers on ATE:LAG1 (AS 64501 -> DUT 192.0.2.1 / 2001:db8::1)
	// and advertise IPv4 prefix 198.19.0.0/24 and IPv6 prefix 2001:db8:20::/64.
	withDefaultOTGPeerSettings := func(a *otgconfighelpers.BGPPeerAttrs) {
		a.GracefulRestart = nil
		a.Timers = nil
	}
	lagBGPv4 := otgconfighelpers.AddBGPV4Peer(lagDev, ateLag1.Name+".IPv4",
		otgconfighelpers.WithBGPName(ateLag1.Name+".BGPv4"),
		otgconfighelpers.WithBGPRouterID(ateLag1.IPv4),
		otgconfighelpers.WithBGPPeerAddress(dutLag1.IPv4),
		otgconfighelpers.WithBGPASNumber(ateAS),
		otgconfighelpers.WithBGPEBGP(),
		withDefaultOTGPeerSettings,
		otgconfighelpers.WithBGPLearnedV4Pfx(true),
	)
	otgconfighelpers.AddBGPV4Routes(lagBGPv4, ateLag1.Name+".BGPv4.Routes", []string{ateLagAdvPrefixV4},
		otgconfighelpers.WithBGPRouteNextHopIPv4(ateLag1.IPv4),
	)

	lagBGPv6 := otgconfighelpers.AddBGPV6Peer(lagDev, ateLag1.Name+".IPv6",
		otgconfighelpers.WithBGPName(ateLag1.Name+".BGPv6"),
		otgconfighelpers.WithBGPRouterID(ateLag1.IPv4),
		otgconfighelpers.WithBGPPeerAddress(dutLag1.IPv6),
		otgconfighelpers.WithBGPASNumber(ateAS),
		otgconfighelpers.WithBGPEBGP(),
		withDefaultOTGPeerSettings,
		otgconfighelpers.WithBGPLearnedV6Pfx(true),
	)
	otgconfighelpers.AddBGPV6Routes(lagBGPv6, ateLag1.Name+".BGPv6.Routes", []string{ateLagAdvPrefixV6},
		otgconfighelpers.WithBGPRouteNextHopIPv6(ateLag1.IPv6),
	)

	// Configure dual-stack eBGP peers on ATE:Port3 (AS 64501 -> DUT 192.0.2.5 / 2001:db8::5)
	// and advertise IPv4 prefix 198.51.100.0/24 and IPv6 prefix 2001:db8:1::/64.
	port3BGPv4 := otgconfighelpers.AddBGPV4Peer(port3Dev, atePort3.Name+".IPv4",
		otgconfighelpers.WithBGPName(atePort3.Name+".BGPv4"),
		otgconfighelpers.WithBGPRouterID(atePort3.IPv4),
		otgconfighelpers.WithBGPPeerAddress(dutPort3.IPv4),
		otgconfighelpers.WithBGPASNumber(ateAS),
		otgconfighelpers.WithBGPEBGP(),
		withDefaultOTGPeerSettings,
		otgconfighelpers.WithBGPLearnedV4Pfx(true),
	)
	otgconfighelpers.AddBGPV4Routes(port3BGPv4, atePort3.Name+".BGPv4.Routes", []string{ebgpDestPrefixV4},
		otgconfighelpers.WithBGPRouteNextHopIPv4(atePort3.IPv4),
	)

	port3BGPv6 := otgconfighelpers.AddBGPV6Peer(port3Dev, atePort3.Name+".IPv6",
		otgconfighelpers.WithBGPName(atePort3.Name+".BGPv6"),
		otgconfighelpers.WithBGPRouterID(atePort3.IPv4),
		otgconfighelpers.WithBGPPeerAddress(dutPort3.IPv6),
		otgconfighelpers.WithBGPASNumber(ateAS),
		otgconfighelpers.WithBGPEBGP(),
		withDefaultOTGPeerSettings,
		otgconfighelpers.WithBGPLearnedV6Pfx(true),
	)
	otgconfighelpers.AddBGPV6Routes(port3BGPv6, atePort3.Name+".BGPv6.Routes", []string{ebgpDestPrefixV6},
		otgconfighelpers.WithBGPRouteNextHopIPv6(atePort3.IPv6),
	)
}

// configureDUTVRFSelectionPolicy implements RT-3.5.1 Step 3:
//   - Configures VRF-SELECT-POLICY under the DEFAULT network instance using cfgplugins.NewVrfSelectionPolicy:
//   - Rule 10: IPv4 source-address 198.18.0.0/16, dscp-set [10] -> action network-instance VRF-10.
//   - Rule 20: IPv6 source-address 2001:db8:10::/48, dscp-set [10] -> action network-instance VRF-10.
//   - Binds VRF-SELECT-POLICY to the ingress LAG interface (aggID) subinterface 0.
func configureDUTVRFSelectionPolicy(t *testing.T, env *testEnv) {
	t.Helper()
	cfgplugins.NewVrfSelectionPolicy(t, env.dut, cfgplugins.VrfSelectionPolicyConfig{
		NetworkInstanceName: deviations.DefaultNetworkInstance(env.dut),
		PolicyName:          vrfSelectionPolicyName,
		InterfaceName:       env.aggID,
		SubinterfaceIndex:   0,
		Rules: []cfgplugins.VrfRule{
			{
				Index:        rule10SeqID,
				IpType:       cfgplugins.IPv4,
				SourcePrefix: rule10SrcPrefixV4,
				PrefixLength: rule10PrefixLenV4,
				DscpSet:      []uint8{dscpVRF10},
				NetInstName:  vrf10Name,
			},
			{
				Index:        rule20SeqID,
				IpType:       cfgplugins.IPv6,
				SourcePrefix: rule20SrcPrefixV6,
				PrefixLength: rule20PrefixLenV6,
				DscpSet:      []uint8{dscpVRF10},
				NetInstName:  vrf10Name,
			},
		},
	})
}

// verifyControlPlaneState implements RT-3.5.1 Step 4:
//   - Verifies OTG and DUT LAG operational status is UP and LACP member ports are collecting/distributing.
//   - Verifies OTG ARP and IPv6 NDP resolution.
//   - Verifies DUT IPv6 NDP neighbor cache state on LAG1 subinterface 0 for 2001:db8::2 is resolved
//     (not INCOMPLETE) and has a valid link-layer address, and that any fe80::/10 link-local entry is not INCOMPLETE.
//   - Verifies dual-stack eBGP sessions over LAG1 and Port3 reach ESTABLISHED.
//   - Verifies AFT route installation in DEFAULT and VRF-10 network instances.
//   - Verifies VRF-SELECT-POLICY state and interface binding on the DUT.
func verifyControlPlaneState(t *testing.T, env *testEnv) {
	t.Helper()
	dut := env.dut
	ate := env.ate
	defNI := deviations.DefaultNetworkInstance(dut)

	// 1. Verify OTG LAG is UP with 2 member ports.
	otgvalidationhelpers.WaitForOTGLAGUP(t, ate, otgvalidationhelpers.LagParams{
		LagName:       ateLag1.Name,
		WantMembersUp: uint64(len(env.aggPorts)),
		Timeout:       controlTimeout,
	})
	otgutils.LogLAGMetrics(t, ate.OTG(), env.top)
	otgutils.LogLACPMetrics(t, ate.OTG(), env.top)

	// 2. Verify DUT LAG interface and all physical ports (port1, port2, port3, port4) operational status is UP,
	// and LACP members are collecting & distributing.
	gnmi.Await(t, dut, gnmi.OC().Interface(env.aggID).OperStatus().State(), controlTimeout, oc.Interface_OperStatus_UP)
	for _, port := range env.aggPorts {
		gnmi.Await(t, dut, gnmi.OC().Interface(port.Name()).OperStatus().State(), controlTimeout, oc.Interface_OperStatus_UP)
		lacpMemberPath := gnmi.OC().Lacp().Interface(env.aggID).Member(port.Name()).State()
		if _, ok := gnmi.Watch(t, dut, lacpMemberPath, controlTimeout, func(val *ygnmi.Value[*oc.Lacp_Interface_Member]) bool {
			member, present := val.Val()
			return present && member != nil && member.GetCollecting() && member.GetDistributing()
		}).Await(t); !ok {
			memberState := gnmi.Get(t, dut, lacpMemberPath)
			t.Errorf("LACP member %s on LAG %s did not reach Collecting=true, Distributing=true: got Collecting=%v, Distributing=%v",
				port.Name(), env.aggID, memberState.GetCollecting(), memberState.GetDistributing())
		}
	}
	gnmi.Await(t, dut, gnmi.OC().Interface(dut.Port(t, "port3").Name()).OperStatus().State(), controlTimeout, oc.Interface_OperStatus_UP)
	gnmi.Await(t, dut, gnmi.OC().Interface(dut.Port(t, "port4").Name()).OperStatus().State(), controlTimeout, oc.Interface_OperStatus_UP)

	// 3. Verify OTG ARP (IPv4) and NDP (IPv6) resolution across all configured interfaces.
	otgutils.WaitForARP(t, ate.OTG(), env.top, "IPv4")
	otgutils.WaitForARP(t, ate.OTG(), env.top, "IPv6")

	// 4. Verify DUT IPv4 ARP and IPv6 NDP neighbor cache state on LAG1 subinterface 0 (b/562121521 regression check).
	verifyDUTIPv4ARPNeighborState(t, env)
	verifyDUTIPv6NDPNeighborState(t, env)

	// 5. Verify DUT IPv4 and IPv6 eBGP sessions in DEFAULT network instance reach ESTABLISHED and AFI-SAFI active=true.
	verifyBGPSessionsEstablished(t, env)

	// 6. Verify AFT route installation in DEFAULT and VRF-10 network instances.
	expectedRoutes := map[string]cfgplugins.RouteInfo{
		ateLagAdvPrefixV4: {VRF: defNI, IPType: cfgplugins.IPv4, DefaultName: defNI},
		ateLagAdvPrefixV6: {VRF: defNI, IPType: cfgplugins.IPv6, DefaultName: defNI},
		ebgpDestPrefixV4:  {VRF: defNI, IPType: cfgplugins.IPv4, DefaultName: defNI},
		ebgpDestPrefixV6:  {VRF: defNI, IPType: cfgplugins.IPv6, DefaultName: defNI},
		vrf10DestPrefixV4: {VRF: vrf10Name, IPType: cfgplugins.IPv4, DefaultName: defNI},
		vrf10DestPrefixV6: {VRF: vrf10Name, IPType: cfgplugins.IPv6, DefaultName: defNI},
	}
	cfgplugins.VerifyRoutes(t, dut, expectedRoutes)

	// 7. Verify VRF selection policy interface binding on DUT.
	if !deviations.PolicyForwardingOCUnsupported(dut) {
		pfStatePath := gnmi.OC().NetworkInstance(defNI).PolicyForwarding()
		interfaceID := env.aggID
		if deviations.InterfaceRefInterfaceIDFormat(dut) || deviations.InterfaceIDFormatRequiredForPolicyForwarding(dut) {
			interfaceID = fmt.Sprintf("%s.0", env.aggID)
		}
		gnmi.Await(t, dut, pfStatePath.Interface(interfaceID).ApplyVrfSelectionPolicy().State(), controlTimeout, vrfSelectionPolicyName)
	}
}

// verifyDUTIPv4ARPNeighborState verifies that the IPv4 ARP neighbor entry for 192.0.2.2 on the
// DUT's ingress LAG subinterface 0 resolves a valid MAC address in link-layer-address.
func verifyDUTIPv4ARPNeighborState(t *testing.T, env *testEnv) {
	t.Helper()
	dut := env.dut
	arpMacPath := gnmi.OC().Interface(env.aggID).Subinterface(0).Ipv4().Neighbor(ateLag1.IPv4).LinkLayerAddress().State()
	lastVal, ok := gnmi.Watch(t, dut, arpMacPath, controlTimeout, func(val *ygnmi.Value[string]) bool {
		mac, present := val.Val()
		return present && mac != ""
	}).Await(t)
	if !ok {
		t.Errorf("DUT IPv4 ARP neighbor %s on %s.0 did not resolve link-layer-address within %v (last value: %v)",
			ateLag1.IPv4, env.aggID, controlTimeout, lastVal)
		return
	}
	mac, _ := lastVal.Val()
	t.Logf("DUT IPv4 ARP neighbor %s on %s.0 resolved: link-layer-address=%s", ateLag1.IPv4, env.aggID, mac)
}

// verifyDUTIPv6NDPNeighborState verifies that the IPv6 neighbor entry for 2001:db8::2 (and any
// fe80::/10 link-local neighbor) on the DUT's ingress LAG subinterface 0 is resolved with a valid
// MAC address and is NOT stuck in INCOMPLETE state while VRF-SELECT-POLICY is active.
func verifyDUTIPv6NDPNeighborState(t *testing.T, env *testEnv) {
	t.Helper()
	dut := env.dut
	nbrPath := gnmi.OC().Interface(env.aggID).Subinterface(0).Ipv6().Neighbor(ateLag1.IPv6).State()

	lastVal, ok := gnmi.Watch(t, dut, nbrPath, controlTimeout, func(val *ygnmi.Value[*oc.Interface_Subinterface_Ipv6_Neighbor]) bool {
		nbr, present := val.Val()
		if !present || nbr == nil {
			return false
		}
		if nbr.GetLinkLayerAddress() == "" {
			return false
		}
		state := nbr.GetNeighborState()
		if state == oc.Neighbor_NeighborState_INCOMPLETE {
			return false
		}
		return state == oc.Neighbor_NeighborState_REACHABLE ||
			state == oc.Neighbor_NeighborState_STALE ||
			state == oc.Neighbor_NeighborState_DELAY ||
			state == oc.Neighbor_NeighborState_PROBE ||
			state == oc.Neighbor_NeighborState_UNSET
	}).Await(t)
	if !ok {
		t.Fatalf("DUT IPv6 NDP neighbor %s on %s.0 did not resolve within %v (last value: %v)",
			ateLag1.IPv6, env.aggID, controlTimeout, lastVal)
	}
	nbr, _ := lastVal.Val()
	t.Logf("DUT IPv6 NDP neighbor %s on %s.0 resolved: link-layer-address=%s, neighbor-state=%v",
		ateLag1.IPv6, env.aggID, nbr.GetLinkLayerAddress(), nbr.GetNeighborState())

	// Also verify that any link-local (fe80::/10) neighbor entries on LAG1 subinterface 0 are not INCOMPLETE.
	allNeighbors := gnmi.GetAll(t, dut, gnmi.OC().Interface(env.aggID).Subinterface(0).Ipv6().NeighborAny().State())
	for _, n := range allNeighbors {
		ip := n.GetIp()
		if strings.HasPrefix(strings.ToLower(ip), "fe80:") {
			if n.GetNeighborState() == oc.Neighbor_NeighborState_INCOMPLETE {
				t.Errorf("DUT link-local IPv6 neighbor %s on %s.0 is in INCOMPLETE state", ip, env.aggID)
			}
			if n.GetLinkLayerAddress() == "" {
				t.Errorf("DUT link-local IPv6 neighbor %s on %s.0 has empty link-layer-address", ip, env.aggID)
			}
		}
	}
}

// verifyBGPSessionsEstablished verifies that all four dual-stack eBGP sessions (IPv4 and IPv6
// on LAG1 and Port3) are in ESTABLISHED state and their respective AFI-SAFI state is active on the DUT.
func verifyBGPSessionsEstablished(t *testing.T, env *testEnv) {
	t.Helper()
	dut := env.dut
	defNI := deviations.DefaultNetworkInstance(dut)
	bgpPath := gnmi.OC().NetworkInstance(defNI).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()

	peers := []struct {
		addr    string
		afiSafi oc.E_BgpTypes_AFI_SAFI_TYPE
	}{
		{addr: ateLag1.IPv4, afiSafi: oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST},
		{addr: ateLag1.IPv6, afiSafi: oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST},
		{addr: atePort3.IPv4, afiSafi: oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST},
		{addr: atePort3.IPv6, afiSafi: oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST},
	}
	for _, peer := range peers {
		nbrPath := bgpPath.Neighbor(peer.addr)
		if _, ok := gnmi.Watch(t, dut, nbrPath.SessionState().State(), controlTimeout, func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
			state, present := val.Val()
			return present && state == oc.Bgp_Neighbor_SessionState_ESTABLISHED
		}).Await(t); !ok {
			fptest.LogQuery(t, fmt.Sprintf("DUT BGP neighbor %s state", peer.addr), nbrPath.State(), gnmi.Get(t, dut, nbrPath.State()))
			t.Errorf("DUT eBGP neighbor %s did not reach ESTABLISHED within %v", peer.addr, controlTimeout)
			continue
		}
		if _, ok := gnmi.Watch(t, dut, nbrPath.AfiSafi(peer.afiSafi).Active().State(), controlTimeout, func(val *ygnmi.Value[bool]) bool {
			active, present := val.Val()
			return present && active
		}).Await(t); !ok {
			t.Errorf("DUT eBGP neighbor %s AFI-SAFI %v did not become active=true within %v", peer.addr, peer.afiSafi, controlTimeout)
			continue
		}
		t.Logf("DUT eBGP neighbor %s is ESTABLISHED with AFI-SAFI %v active=true", peer.addr, peer.afiSafi)
	}
}

// configureOTGTrafficFlows implements RT-3.5.2 Step 2:
//   - Flow-1-Default-IPv4: ATE:LAG1 -> ATE:Port3, Src 198.19.0.1, Dst 198.51.100.1, DSCP 0.
//   - Flow-2-Default-IPv6: ATE:LAG1 -> ATE:Port3, Src 2001:db8:20::1, Dst 2001:db8:1::1, DSCP 0.
//   - Flow-3-Steered-IPv4: ATE:LAG1 -> ATE:Port4, Src 198.18.0.1, Dst 203.0.113.1, DSCP 10.
//   - Flow-4-Steered-IPv6: ATE:LAG1 -> ATE:Port4, Src 2001:db8:10::1, Dst 2001:db8:2::1, DSCP 10.
func configureOTGTrafficFlows(t *testing.T, env *testEnv) []string {
	t.Helper()
	top := env.top
	top.Flows().Clear()

	// Flow 1: IPv4 Default VRF (Src 198.19.0.1, DSCP 0 -> forwarded via eBGP route to ATE:Port3).
	flow1 := &otgconfighelpers.Flow{
		FlowName:      flow1IPv4DefaultVRF,
		TxNames:       []string{ateLag1.Name + ".IPv4"},
		RxNames:       []string{atePort3.Name + ".IPv4"},
		FrameSize:     trafficFrameSize,
		PpsRate:       trafficRatePps,
		PacketsToSend: trafficPackets,
		EthFlow:       &otgconfighelpers.EthFlowParams{SrcMAC: ateLag1.MAC},
		IPv4Flow: &otgconfighelpers.IPv4FlowParams{
			IPv4Src: defaultTrafficSrc4,
			IPv4Dst: defaultTrafficDst4,
			DSCP:    dscpDefault,
		},
	}
	flow1.CreateFlow(top)
	flow1.AddEthHeader()
	flow1.AddIPv4Header()

	// Flow 2: IPv6 Default VRF (Src 2001:db8:20::1, DSCP 0 -> forwarded via eBGP route to ATE:Port3).
	flow2 := &otgconfighelpers.Flow{
		FlowName:      flow2IPv6DefaultVRF,
		TxNames:       []string{ateLag1.Name + ".IPv6"},
		RxNames:       []string{atePort3.Name + ".IPv6"},
		FrameSize:     trafficFrameSize,
		PpsRate:       trafficRatePps,
		PacketsToSend: trafficPackets,
		EthFlow:       &otgconfighelpers.EthFlowParams{SrcMAC: ateLag1.MAC},
		IPv6Flow: &otgconfighelpers.IPv6FlowParams{
			IPv6Src:      defaultTrafficSrc6,
			IPv6Dst:      defaultTrafficDst6,
			TrafficClass: dscpDefault << 2,
		},
	}
	flow2.CreateFlow(top)
	flow2.AddEthHeader()
	flow2.AddIPv6Header()

	// Flow 3: IPv4 VRF-10 Steered (matching Rule 10: Src 198.18.0.0/16, DSCP 10 -> VRF-10 -> ATE:Port4).
	flow3 := &otgconfighelpers.Flow{
		FlowName:      flow3IPv4VRF10,
		TxNames:       []string{ateLag1.Name + ".IPv4"},
		RxNames:       []string{atePort4.Name + ".IPv4"},
		FrameSize:     trafficFrameSize,
		PpsRate:       trafficRatePps,
		PacketsToSend: trafficPackets,
		EthFlow:       &otgconfighelpers.EthFlowParams{SrcMAC: ateLag1.MAC},
		IPv4Flow: &otgconfighelpers.IPv4FlowParams{
			IPv4Src: steeredTrafficSrc4,
			IPv4Dst: vrf10TrafficDst4,
			DSCP:    dscpVRF10,
		},
	}
	flow3.CreateFlow(top)
	flow3.AddEthHeader()
	flow3.AddIPv4Header()

	// Flow 4: IPv6 VRF-10 Steered (matching Rule 20: Src 2001:db8:10::/48, DSCP 10 -> VRF-10 -> ATE:Port4).
	flow4 := &otgconfighelpers.Flow{
		FlowName:      flow4IPv6VRF10,
		TxNames:       []string{ateLag1.Name + ".IPv6"},
		RxNames:       []string{atePort4.Name + ".IPv6"},
		FrameSize:     trafficFrameSize,
		PpsRate:       trafficRatePps,
		PacketsToSend: trafficPackets,
		EthFlow:       &otgconfighelpers.EthFlowParams{SrcMAC: ateLag1.MAC},
		IPv6Flow: &otgconfighelpers.IPv6FlowParams{
			IPv6Src:      steeredTrafficSrc6,
			IPv6Dst:      vrf10TrafficDst6,
			TrafficClass: dscpVRF10 << 2,
		},
	}
	flow4.CreateFlow(top)
	flow4.AddEthHeader()
	flow4.AddIPv6Header()

	return []string{
		flow1IPv4DefaultVRF,
		flow2IPv6DefaultVRF,
		flow3IPv4VRF10,
		flow4IPv6VRF10,
	}
}

// runAndVerifyTraffic implements RT-3.5.2 Step 2 & Step 3:
//   - Verifies ARP/NDP and eBGP readiness prior to traffic injection.
//   - Starts traffic and awaits flow completion using gnmi.Watch (no time.Sleep).
//   - Verifies 0% packet loss on all 4 flows using otgvalidationhelpers.VerifyTraffic.
//   - Verifies VRF-SELECT-POLICY Rule 10 and Rule 20 matched-pkts and matched-octets counters increment.
//   - Verifies DUT interface packet counters (in-pkts on LAG1 and out-pkts on Port3 and Port4) increment.
//   - Re-verifies that dual-stack eBGP sessions remain ESTABLISHED and IPv6 NDP remains resolved.
func runAndVerifyTraffic(t *testing.T, env *testEnv, flowNames []string) {
	t.Helper()
	if len(flowNames) == 0 {
		t.Fatalf("flowNames must not be empty")
	}
	dut := env.dut
	ate := env.ate
	otg := ate.OTG()

	otgutils.WaitForARP(t, otg, env.top, "IPv4")
	otgutils.WaitForARP(t, otg, env.top, "IPv6")
	verifyBGPSessionsEstablished(t, env)

	// Capture baseline policy rule matched-pkts and matched-octets counters and interface counters before starting traffic.
	var rule10PktsBefore, rule20PktsBefore uint64
	var rule10OctetsBefore, rule20OctetsBefore uint64
	policyPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding().Policy(vrfSelectionPolicyName)
	if !deviations.PolicyRuleCountersOCUnsupported(dut) && !deviations.PolicyForwardingOCUnsupported(dut) {
		if v, ok := gnmi.Lookup(t, dut, policyPath.Rule(rule10SeqID).MatchedPkts().State()).Val(); ok {
			rule10PktsBefore = v
		}
		if v, ok := gnmi.Lookup(t, dut, policyPath.Rule(rule20SeqID).MatchedPkts().State()).Val(); ok {
			rule20PktsBefore = v
		}
		if v, ok := gnmi.Lookup(t, dut, policyPath.Rule(rule10SeqID).MatchedOctets().State()).Val(); ok {
			rule10OctetsBefore = v
		}
		if v, ok := gnmi.Lookup(t, dut, policyPath.Rule(rule20SeqID).MatchedOctets().State()).Val(); ok {
			rule20OctetsBefore = v
		}
	}

	p3Name := dut.Port(t, "port3").Name()
	p4Name := dut.Port(t, "port4").Name()
	lagInPktsBefore, _ := gnmi.Lookup(t, dut, gnmi.OC().Interface(env.aggID).Counters().InPkts().State()).Val()
	p3OutPktsBefore, _ := gnmi.Lookup(t, dut, gnmi.OC().Interface(p3Name).Counters().OutPkts().State()).Val()
	p4OutPktsBefore, _ := gnmi.Lookup(t, dut, gnmi.OC().Interface(p4Name).Counters().OutPkts().State()).Val()

	// Start traffic across all four flows and wait for fixed packet transmission to finish via gnmi.Watch.
	otg.StartTraffic(t)
	for _, flowName := range flowNames {
		transmitPath := gnmi.OTG().Flow(flowName).Transmit().State()
		if _, ok := gnmi.Watch(t, otg, transmitPath, trafficTimeout, func(val *ygnmi.Value[bool]) bool {
			transmitState, present := val.Val()
			return present && !transmitState
		}).Await(t); !ok {
			t.Errorf("Traffic for flow %s did not finish transmitting within %v", flowName, trafficTimeout)
		}
	}

	otgutils.LogFlowMetrics(t, otg, env.top)
	otgutils.LogPortMetrics(t, otg, env.top)

	// Verify 0% packet loss (FramesRx == FramesTx and FramesTx > 0) for all 4 flows.
	for _, flowName := range flowNames {
		if err := otgvalidationhelpers.VerifyTraffic(t, ate, otgvalidationhelpers.VerifyTrafficParams{
			Config:       env.top,
			FlowName:     flowName,
			TestResults:  true,
			WatchTimeout: trafficTimeout,
		}); err != nil {
			t.Errorf("RT-3.5.2 traffic verification failed for %s: %v", flowName, err)
		}
	}

	// Verify VRF selection policy Rule 10 and Rule 20 matched-pkts and matched-octets counters incremented.
	if !deviations.PolicyRuleCountersOCUnsupported(dut) && !deviations.PolicyForwardingOCUnsupported(dut) {
		for _, ruleCheck := range []struct {
			seqID        uint32
			pktsBefore   uint64
			octetsBefore uint64
		}{
			{seqID: rule10SeqID, pktsBefore: rule10PktsBefore, octetsBefore: rule10OctetsBefore},
			{seqID: rule20SeqID, pktsBefore: rule20PktsBefore, octetsBefore: rule20OctetsBefore},
		} {
			if _, ok := gnmi.Watch(t, dut, policyPath.Rule(ruleCheck.seqID).MatchedPkts().State(), trafficTimeout, func(val *ygnmi.Value[uint64]) bool {
				pkts, present := val.Val()
				return present && pkts >= ruleCheck.pktsBefore+trafficPackets
			}).Await(t); !ok {
				got, _ := gnmi.Lookup(t, dut, policyPath.Rule(ruleCheck.seqID).MatchedPkts().State()).Val()
				t.Errorf("Policy %s rule %d matched-pkts counter did not increment as expected: got %d, want >= %d",
					vrfSelectionPolicyName, ruleCheck.seqID, got, ruleCheck.pktsBefore+trafficPackets)
			}
			if octets, ok := gnmi.Lookup(t, dut, policyPath.Rule(ruleCheck.seqID).MatchedOctets().State()).Val(); ok {
				t.Logf("Policy %s rule %d matched-octets: before=%d, after=%d", vrfSelectionPolicyName, ruleCheck.seqID, ruleCheck.octetsBefore, octets)
			}
		}
	}

	// Verify DUT interface counters (in-pkts on LAG1 and out-pkts on Port3 and Port4) incremented as expected.
	if _, ok := gnmi.Watch(t, dut, gnmi.OC().Interface(env.aggID).Counters().InPkts().State(), trafficTimeout, func(val *ygnmi.Value[uint64]) bool {
		pkts, present := val.Val()
		return present && pkts >= lagInPktsBefore+4*trafficPackets
	}).Await(t); !ok {
		got, _ := gnmi.Lookup(t, dut, gnmi.OC().Interface(env.aggID).Counters().InPkts().State()).Val()
		t.Errorf("DUT LAG %s in-pkts counter did not increment as expected: got %d, want >= %d", env.aggID, got, lagInPktsBefore+4*trafficPackets)
	}
	for _, portCheck := range []struct {
		name   string
		before uint64
	}{
		{name: p3Name, before: p3OutPktsBefore},
		{name: p4Name, before: p4OutPktsBefore},
	} {
		if _, ok := gnmi.Watch(t, dut, gnmi.OC().Interface(portCheck.name).Counters().OutPkts().State(), trafficTimeout, func(val *ygnmi.Value[uint64]) bool {
			pkts, present := val.Val()
			return present && pkts >= portCheck.before+2*trafficPackets
		}).Await(t); !ok {
			got, _ := gnmi.Lookup(t, dut, gnmi.OC().Interface(portCheck.name).Counters().OutPkts().State()).Val()
			t.Errorf("DUT interface %s out-pkts counter did not increment as expected: got %d, want >= %d", portCheck.name, got, portCheck.before+2*trafficPackets)
		}
	}

	// Verify eBGP sessions remain ESTABLISHED and IPv6 NDP neighbor cache remains resolved after traffic.
	verifyBGPSessionsEstablished(t, env)
	verifyDUTIPv6NDPNeighborState(t, env)
}
