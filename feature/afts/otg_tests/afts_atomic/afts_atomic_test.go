// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package afts_atomic_test

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
	"github.com/openconfig/featureprofiles/internal/telemetry/aftcache"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ygnmi/ygnmi"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	advertisedRoutesV4Prefix = 32
	advertisedRoutesV6Prefix = 128
	aftConvergenceTime       = 20 * time.Minute
	applyPolicyType          = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
	ateAS                    = 200
	bgpRoute                 = "200.0.0.0"
	// REVERT TO SCALE BEFORE SUBMITTING
	bgpRouteCountIPv4Default  = 200
	bgpRouteCountIPv4LowScale = 200
	bgpRouteCountIPv6Default  = 200
	bgpRouteCountIPv6LowScale = 200
	bgpRouteV6                = "3001:1::0"
	gnmiTimeout               = 2 * time.Minute
	dutAS                     = 65501
	isisATEArea               = "49"
	isisATESystemID           = "6400.0000.0001"
	isisDUTArea               = "49.0001"
	isisDUTSystemID           = "1920.0000.2001"
	isisRoute                 = "192.0.2.1"
	isisRouteCount            = 100
	isisRouteV6               = "2001:DB8::1"
	linkLocalAddress          = "fe80::200:2ff:fe02:202"
	mtu                       = 1500
	startingBGPRouteIPv4      = "200.0.0.0/32"
	startingBGPRouteIPv6      = "3001:1::0/128"
	startingISISRouteIPv4     = "192.0.2.1/32"
	startingISISRouteIPv6     = "2001:DB8::1/128"
	v4PrefixLen               = 30
	v6PrefixLen               = 126

	peerGrpNameV4P1 = "BGP-PEER-GROUP-V4-P1"
	peerGrpNameV6P1 = "BGP-PEER-GROUP-V6-P1"
	peerGrpNameV4P2 = "BGP-PEER-GROUP-V4-P2"
	peerGrpNameV6P2 = "BGP-PEER-GROUP-V6-P2"

	IPv4 = "IPv4"
	IPv6 = "IPv6"
)

var (
	ateP1 = attrs.Attributes{
		IPv4:    "192.0.2.2",
		IPv4Len: v4PrefixLen,
		IPv6:    "2001:db8::2",
		IPv6Len: v6PrefixLen,
		MAC:     "00:00:02:02:02:02",
	}
	ateP2 = attrs.Attributes{
		IPv4:    "192.0.2.6",
		IPv4Len: v4PrefixLen,
		IPv6:    "2001:db8::6",
		IPv6Len: v6PrefixLen,
		MAC:     "00:00:03:03:03:03",
	}
	dutP1 = attrs.Attributes{
		IPv4:    "192.0.2.1",
		IPv4Len: v4PrefixLen,
		IPv6:    "2001:db8::1",
		IPv6Len: v6PrefixLen,
	}
	dutP2 = attrs.Attributes{
		IPv4:    "192.0.2.5",
		IPv4Len: v4PrefixLen,
		IPv6:    "2001:db8::5",
		IPv6Len: v6PrefixLen,
	}

	ateAttrs       = []attrs.Attributes{ateP1, ateP2}
	v4PeerGrpNames = []string{peerGrpNameV4P1, peerGrpNameV4P2}
	v6PeerGrpNames = []string{peerGrpNameV6P1, peerGrpNameV6P2}

	port1Name = "port1"
	port2Name = "port2"

	ipv4OneNH  = map[string]bool{ateP1.IPv4: true}
	ipv4TwoNHs = map[string]bool{ateP1.IPv4: true, ateP2.IPv4: true}

	ipv6LinkLocalNH = map[string]bool{linkLocalAddress: true}
	ipv6OneNH       = map[string]bool{ateP1.IPv6: true}
	ipv6TwoNHs      = map[string]bool{ateP1.IPv6: true, ateP2.IPv6: true}
)

func configureAllowPolicy(t *testing.T, dut *ondatra.DUTDevice) error {
	t.Helper()
	d := &oc.Root{}
	routePolicy := d.GetOrCreateRoutingPolicy()
	policyDefinition := routePolicy.GetOrCreatePolicyDefinition(cfgplugins.ALLOW)
	statement, err := policyDefinition.AppendNewStatement("id-1")
	if err != nil {
		return fmt.Errorf("failed to append new statement to policy definition %s: %v", cfgplugins.ALLOW, err)
	}
	statement.GetOrCreateActions().PolicyResult = applyPolicyType
	gnmi.Update(t, dut, gnmi.OC().RoutingPolicy().Config(), routePolicy)
	return nil
}

// configureDUT configures all the interfaces and BGP on the DUT.
func (tc *testCase) configureDUT(t *testing.T) error {
	dut := tc.dut
	dutPort1 := dut.Port(t, port1Name).Name()
	dutIntf1 := dutP1.NewOCInterface(dutPort1, dut)
	gnmi.Update(t, dut, gnmi.OC().Interface(dutPort1).Config(), dutIntf1)

	dutPort2 := dut.Port(t, port2Name).Name()
	dutIntf2 := dutP2.NewOCInterface(dutPort2, dut)
	gnmi.Update(t, dut, gnmi.OC().Interface(dutPort2).Config(), dutIntf2)

	// Configure default network instance.
	t.Log("Configure Default Network Instance")
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, dut.Port(t, port1Name))
		fptest.SetPortSpeed(t, dut.Port(t, port2Name))
	}
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, dutPort1, deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, dutPort2, deviations.DefaultNetworkInstance(dut), 0)
	}
	configureAllowPolicy(t, dut)

	t.Log("Configure BGP")
	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	for ix, ateAttr := range ateAttrs {
		nbrs := []*cfgplugins.BgpNeighbor{
			{LocalAS: dutAS, PeerAS: ateAS, Neighborip: ateAttr.IPv4, IsV4: true},
			{LocalAS: dutAS, PeerAS: ateAS, Neighborip: ateAttr.IPv6, IsV4: false},
		}
		routerID := dutP1.IPv4
		dutConf, err := cfgplugins.CreateBGPNeighbors(t, routerID, v4PeerGrpNames[ix], v6PeerGrpNames[ix], nbrs, dut)
		if err != nil {
			return err
		}
		gnmi.Update(t, dut, dutConfPath.Config(), dutConf)
		if deviations.BGPMissingOCMaxPrefixesConfiguration(dut) {
			cfgplugins.UpdateNeighborMaxPrefix(t, dut, nbrs)
		}
	}

	t.Log("Configure ISIS")
	b := &gnmi.SetBatch{}
	isisData := &cfgplugins.ISISGlobalParams{
		DUTArea:             isisDUTArea,
		DUTSysID:            isisDUTSystemID,
		ISISInterfaceNames:  []string{dutPort1, dutPort2},
		NetworkInstanceName: deviations.DefaultNetworkInstance(dut),
	}

	root := cfgplugins.NewISIS(t, dut, isisData, b)
	if deviations.ISISSingleTopologyRequired(dut) {
		protocol := root.GetNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			GetProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, deviations.DefaultNetworkInstance(dut))
		multiTopology := protocol.GetOrCreateIsis().GetOrCreateGlobal().
			GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).
			GetOrCreateMultiTopology()
		multiTopology.SetAfiName(oc.IsisTypes_AFI_TYPE_IPV4)
		multiTopology.SetSafiName(oc.IsisTypes_SAFI_TYPE_UNICAST)
	}
	b.Set(t, dut)

	return nil
}

func (tc *testCase) configureATE(t *testing.T) {
	ate := tc.ate
	config := gosnappi.NewConfig()

	portData := []struct {
		portName string
		ateAttrs attrs.Attributes
		dutAttrs attrs.Attributes
	}{
		{
			portName: port1Name,
			ateAttrs: ateP1,
			dutAttrs: dutP1,
		},
		{
			portName: port2Name,
			ateAttrs: ateP2,
			dutAttrs: dutP2,
		},
	}

	for i, p := range portData {
		atePort := ate.Port(t, p.portName)
		port := config.Ports().Add().SetName(atePort.ID())
		dev := config.Devices().Add().SetName(fmt.Sprintf("%s.dev", p.portName))

		eth := dev.Ethernets().Add().SetName(dev.Name() + ".eth").
			SetMac(p.ateAttrs.MAC).
			SetMtu(mtu)
		eth.Connection().SetPortName(port.Name())

		ipv4 := eth.Ipv4Addresses().Add().SetName(eth.Name() + ".IPv4").
			SetAddress(p.ateAttrs.IPv4).
			SetGateway(p.dutAttrs.IPv4).
			SetPrefix(v4PrefixLen)

		ipv6 := eth.Ipv6Addresses().Add().SetName(eth.Name() + ".IPv6").
			SetAddress(p.ateAttrs.IPv6).
			SetGateway(p.dutAttrs.IPv6).
			SetPrefix(v6PrefixLen)

		isis := dev.Isis().SetName(dev.Name() + ".isis").
			SetSystemId(strings.ReplaceAll(isisATESystemID, ".", ""))
		isis.Basic().
			SetIpv4TeRouterId(ipv4.Address()).
			SetHostname(fmt.Sprintf("ixia-c-port%d", i+1))
		isis.Advanced().SetAreaAddresses([]string{isisATEArea})
		isis.Advanced().SetEnableHelloPadding(false)
		isisInt := isis.Interfaces().Add().SetName(isis.Name() + ".intf").
			SetEthName(eth.Name()).
			SetNetworkType(gosnappi.IsisInterfaceNetworkType.POINT_TO_POINT).
			SetLevelType(gosnappi.IsisInterfaceLevelType.LEVEL_2).
			SetMetric(10)
		isisInt.TrafficEngineering().Add().PriorityBandwidths()
		isisInt.Advanced().
			SetAutoAdjustMtu(true).
			SetAutoAdjustArea(true).
			SetAutoAdjustSupportedProtocols(true)

		// Why do we need to advertise routes on both ports?
		v4Route := isis.V4Routes().Add().SetName(isis.Name() + ".rr")
		v4Route.Addresses().Add().SetAddress(isisRoute).
			SetPrefix(advertisedRoutesV4Prefix).
			SetCount(isisRouteCount)
		v6Route := isis.V6Routes().Add().SetName(isis.Name() + ".v6")
		v6Route.Addresses().Add().SetAddress(isisRouteV6).
			SetPrefix(advertisedRoutesV6Prefix).
			SetCount(isisRouteCount)
		tc.configureBGPDev(dev, ipv4, ipv6)
	}

	ate.OTG().PushConfig(t, config)
	ate.OTG().StartProtocols(t)
}

func (tc *testCase) waitForBGPSession(t *testing.T) error {
	bgpPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(tc.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	verifySessionState := func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
		state, ok := val.Val()
		if !ok {
			return false
		}
		t.Logf("BGP session state: %s", state.String())
		return state == oc.Bgp_Neighbor_SessionState_ESTABLISHED
	}

	for i, ateAttr := range ateAttrs {
		nbrPath := bgpPath.Neighbor(ateAttr.IPv4)
		if _, ok := gnmi.Watch(t, tc.dut, nbrPath.SessionState().State(), gnmiTimeout, verifySessionState).Await(t); !ok {
			fptest.LogQuery(t, "BGP reported state", nbrPath.State(), gnmi.Get(t, tc.dut, nbrPath.State()))
			return fmt.Errorf("no BGP neighbor formed for port%d IPv4 (%s)", i+1, ateAttr.IPv4)
		}

		nbrPathv6 := bgpPath.Neighbor(ateAttr.IPv6)
		if _, ok := gnmi.Watch(t, tc.dut, nbrPathv6.SessionState().State(), gnmiTimeout, verifySessionState).Await(t); !ok {
			fptest.LogQuery(t, "BGPv6 reported state", nbrPathv6.State(), gnmi.Get(t, tc.dut, nbrPathv6.State()))
			return fmt.Errorf("no BGPv6 neighbor formed for port%d IPv6 (%s)", i+1, ateAttr.IPv6)
		}
	}

	return nil
}

func (tc *testCase) waitForISISAdjacency(t *testing.T) error {
	isisPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(tc.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, deviations.DefaultNetworkInstance(tc.dut)).Isis()

	verifyAdjacencyState := func(val *ygnmi.Value[oc.E_Isis_IsisInterfaceAdjState]) bool {
		state, ok := val.Val()
		if !ok {
			return false
		}
		t.Logf("ISIS adjacency state: %s", state.String())
		return state == oc.Isis_IsisInterfaceAdjState_UP
	}

	dutPort1 := tc.dut.Port(t, port1Name).Name()
	dutPort2 := tc.dut.Port(t, port2Name).Name()

	for i, dutPort := range []string{dutPort1, dutPort2} {
		adjPath := isisPath.Interface(dutPort).Level(2).Adjacency(isisATESystemID)
		if _, ok := gnmi.Watch(t, tc.dut, adjPath.AdjacencyState().State(), gnmiTimeout, verifyAdjacencyState).Await(t); !ok {
			fptest.LogQuery(t, "ISIS reported state", adjPath.State(), gnmi.Get(t, tc.dut, adjPath.State()))
			return fmt.Errorf("no ISIS adjacency formed for port%d (%s)", i+1, dutPort)
		}
	}

	return nil
}

// bgpRouteCount returns the expected route count for the given dut and IP family.
func bgpRouteCount(dut *ondatra.DUTDevice, afi string) uint32 {
	if deviations.LowScaleAft(dut) {
		if afi == IPv4 {
			return bgpRouteCountIPv4LowScale
		}
		return bgpRouteCountIPv6LowScale
	}
	if afi == IPv4 {
		return bgpRouteCountIPv4Default
	}
	return bgpRouteCountIPv6Default
}

func (tc *testCase) configureBGPDev(dev gosnappi.Device, ipv4 gosnappi.DeviceIpv4, ipv6 gosnappi.DeviceIpv6) {
	bgp := dev.Bgp().SetRouterId(ipv4.Address())
	bgp4Peer := bgp.Ipv4Interfaces().Add().SetIpv4Name(ipv4.Name()).Peers().Add().SetName(dev.Name() + ".BGP4.peer")
	bgp4Peer.SetPeerAddress(ipv4.Gateway()).SetAsNumber(uint32(ateAS)).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
	bgp6Peer := bgp.Ipv6Interfaces().Add().SetIpv6Name(ipv6.Name()).Peers().Add().SetName(dev.Name() + ".BGP6.peer")
	bgp6Peer.SetPeerAddress(ipv6.Gateway()).SetAsNumber(uint32(ateAS)).SetAsType(gosnappi.BgpV6PeerAsType.EBGP)

	routes := bgp4Peer.V4Routes().Add().SetName(bgp4Peer.Name() + ".v4route")
	routes.SetNextHopIpv4Address(ipv4.Address()).
		SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).
		SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)
	routes.Addresses().Add().
		SetAddress(bgpRoute).
		SetPrefix(advertisedRoutesV4Prefix).
		SetCount(bgpRouteCount(tc.dut, IPv4))

	routesV6 := bgp6Peer.V6Routes().Add().SetName(bgp6Peer.Name() + ".v6route")
	routesV6.SetNextHopIpv6Address(ipv6.Address()).
		SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).
		SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL)
	routesV6.Addresses().Add().
		SetAddress(bgpRouteV6).
		SetPrefix(advertisedRoutesV6Prefix).
		SetCount(bgpRouteCount(tc.dut, IPv6))
}

func generateBGPPrefixes(t *testing.T, dut *ondatra.DUTDevice) map[string]bool {
	wantPrefixes := make(map[string]bool)
	for pfix := range netutil.GenCIDRs(t, startingBGPRouteIPv4, int(bgpRouteCount(dut, IPv4))) {
		wantPrefixes[pfix] = true
	}
	for pfix6 := range netutil.GenCIDRs(t, startingBGPRouteIPv6, int(bgpRouteCount(dut, IPv6))) {
		wantPrefixes[pfix6] = true
	}
	return wantPrefixes
}

func generateISISPrefixes(t *testing.T) map[string]bool {
	wantPrefixes := make(map[string]bool)
	for pfix := range netutil.GenCIDRs(t, startingISISRouteIPv4, isisRouteCount) {
		wantPrefixes[pfix] = true
	}
	for pfix6 := range netutil.GenCIDRs(t, startingISISRouteIPv6, isisRouteCount) {
		wantPrefixes[pfix6] = true
	}
	return wantPrefixes
}

// setOTGInterfaceState sets the state of the provided port.
func setOTGInterfaceState(t *testing.T, ate *ondatra.ATEDevice, portName string, state gosnappi.StatePortLinkStateEnum) {
	portStateAction := gosnappi.NewControlState()
	portStateAction.Port().Link().SetPortNames([]string{portName}).SetState(state)
	ate.OTG().SetControlState(t, portStateAction)
}

func postChurnIPv6(t *testing.T, dut *ondatra.DUTDevice) map[string]bool {
	t.Helper()
	if deviations.LinkLocalInsteadOfNh(dut) {
		return ipv6LinkLocalNH
	}
	return ipv6OneNH
}

type testCase struct {
	name string
	dut  *ondatra.DUTDevice
	ate  *ondatra.ATEDevice

	// churn downs one or more ports to create churn.
	churn func()
	// stoppingCondition provides the expected prefixes after the churn.
	stoppingCondition aftcache.PeriodicHook
	// additionalVerification provides additional verification after churn, if any.
	additionalVerification func(*aftcache.AFTStreamSession)
	// revert restores the port(s) to the original state.
	revert func()
}

func TestAtomic(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	bgpPrefixes := generateBGPPrefixes(t, dut)
	isisPrefixes := generateISISPrefixes(t)
	prefixes := make(map[string]bool)
	for pfx := range bgpPrefixes {
		prefixes[pfx] = true
	}
	for pfx := range isisPrefixes {
		prefixes[pfx] = true
	}

	verifyBGPPrefixes := aftcache.InitialSyncStoppingCondition(t, dut, bgpPrefixes, ipv4TwoNHs, ipv6TwoNHs)
	verifyISISPrefixes := aftcache.AssertNextHopCount(t, dut, isisPrefixes, 1)
	oneLinkDownBGP := aftcache.InitialSyncStoppingCondition(t, dut, bgpPrefixes, ipv4OneNH, postChurnIPv6(t, dut))
	twoLinksDown := aftcache.DeletionStoppingCondition(t, dut, prefixes)

	setOneLinkDown := func() {
		t.Logf("Stopping interface %s to create churn.", port2Name)
		setOTGInterfaceState(t, ate, port2Name, gosnappi.StatePortLinkState.DOWN)
	}
	setOneLinkUp := func() {
		t.Logf("Starting interface %s to restore missing routes.", port2Name)
		setOTGInterfaceState(t, ate, port2Name, gosnappi.StatePortLinkState.UP)
	}

	setTwoLinksDown := func() {
		t.Logf("Stopping interface %s to create churn.", port1Name)
		setOTGInterfaceState(t, ate, port1Name, gosnappi.StatePortLinkState.DOWN)
		t.Logf("Stopping interface %s to create churn.", port2Name)
		setOTGInterfaceState(t, ate, port2Name, gosnappi.StatePortLinkState.DOWN)
	}
	setTwoLinksUp := func() {
		t.Logf("Starting interface %s to restore missing routes.", port1Name)
		setOTGInterfaceState(t, ate, port1Name, gosnappi.StatePortLinkState.UP)
		t.Logf("Starting interface %s to restore missing routes.", port2Name)
		setOTGInterfaceState(t, ate, port2Name, gosnappi.StatePortLinkState.UP)
	}

	testCases := []*testCase{
		{
			name: "AFT-3.1.1: AFT Atomic Flag check scenario 1",
			dut:  dut,
			ate:  ate,

			churn:             setOneLinkDown,
			stoppingCondition: oneLinkDownBGP,
			additionalVerification: func(aftSession *aftcache.AFTStreamSession) {
				t.Helper()
				aftSession.ListenUntilPreUpdateHook(t.Context(), t, aftConvergenceTime, []aftcache.NotificationHook{aftcache.VerifyAtomicFlagHook(t)}, verifyISISPrefixes)
			},
			revert: setOneLinkUp,
		},
		{
			name: "AFT-3.1.2: AFT Atomic Flag Check Link Down and Up scenario 2",
			dut:  dut,
			ate:  ate,

			churn:             setTwoLinksDown,
			stoppingCondition: twoLinksDown,
			revert:            setTwoLinksUp,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gnmiClient, err := tc.dut.RawAPIs().BindingDUT().DialGNMI(t.Context())
			if err != nil {
				t.Fatalf("Failed to dial GNMI: %v", err)
			}

			if err := tc.configureDUT(t); err != nil {
				t.Fatalf("failed to configure DUT: %v", err)
			}
			tc.configureATE(t)

			t.Log("Waiting for BGP neighbor to establish...")
			if err := tc.waitForBGPSession(t); err != nil {
				t.Fatalf("Unable to establish BGP session: %v", err)
			}

			t.Log("Waiting for ISIS adjacency to form...")
			if err := tc.waitForISISAdjacency(t); err != nil {
				t.Fatalf("Unable to establish ISIS adjacency: %v", err)
			}

			aftSession := aftcache.NewAFTStreamSession(t.Context(), t, gnmiClient, tc.dut)

			t.Logf("Initial verification of %d bgp prefixes and %d isis prefixes", len(bgpPrefixes), len(isisPrefixes))
			aftSession.ListenUntilPreUpdateHook(t.Context(), t, aftConvergenceTime, []aftcache.NotificationHook{aftcache.VerifyAtomicFlagHook(t)}, verifyBGPPrefixes)
			aftSession.ListenUntilPreUpdateHook(t.Context(), t, aftConvergenceTime, []aftcache.NotificationHook{aftcache.VerifyAtomicFlagHook(t)}, verifyISISPrefixes)
			t.Log("Done listening for initial verification.")

			t.Log("Modifying port state to create churn.")
			tc.churn()
			aftSession.ListenUntilPreUpdateHook(t.Context(), t, aftConvergenceTime, []aftcache.NotificationHook{aftcache.VerifyAtomicFlagHook(t)}, tc.stoppingCondition)
			if tc.additionalVerification != nil {
				t.Log("Running additional verification.")
				tc.additionalVerification(aftSession)
			}
			t.Log("Done listening for churn.")

			t.Log("Reverting port state to restore missing routes.")
			tc.revert()
			aftSession.ListenUntilPreUpdateHook(t.Context(), t, aftConvergenceTime, []aftcache.NotificationHook{aftcache.VerifyAtomicFlagHook(t)}, verifyBGPPrefixes)
			aftSession.ListenUntilPreUpdateHook(t.Context(), t, aftConvergenceTime, []aftcache.NotificationHook{aftcache.VerifyAtomicFlagHook(t)}, verifyISISPrefixes)
			t.Log("Done listening after revert.")
		})
	}
}
