// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package afts_base_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/open_traffic_generator/gosnappi"
	"github.com/openconfig/featureprofiles/internal/afts"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/isissession"
	"github.com/openconfig/featureprofiles/internal/telemetry/aftcache"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ygnmi/ygnmi"

	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	advertisedRoutesV4Prefix = 32
	advertisedRoutesV6Prefix = 128
	aftConvergenceTime       = 20 * time.Minute
	applyPolicyName          = "ALLOW"
	applyPolicyType          = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
	ateASN                   = 200
	bgpRoute                 = "200.0.0.0"
	bgpRoutev6               = "3001:1::0"
	bgpTimeout               = 2 * time.Minute
	dutASN                   = 65501
	isisRoute                = "199.0.0.1"
	isisRouteCount           = 100
	isisRoutev6              = "2001:db8::203:0:113:1"
	isisSystemID             = "650000000001"
	linkLocalAddress         = "fe80::200:2ff:fe02:202"
	mtu                      = 1500
	peerGrpNameV4P1          = "BGP-PEER-GROUP-V4-P1"
	peerGrpNameV4P2          = "BGP-PEER-GROUP-V4-P2"
	peerGrpNameV6P1          = "BGP-PEER-GROUP-V6-P1"
	peerGrpNameV6P2          = "BGP-PEER-GROUP-V6-P2"
	port1MAC                 = "00:00:02:02:02:02"
	port2MAC                 = "00:00:03:03:03:03"
	startingBGPRouteIPv4     = "200.0.0.0/32"
	startingBGPRouteIPv6     = "3001:1::0/128"
	startingISISRouteIPv4    = "199.0.0.1/32"
	startingISISRouteIPv6    = "2001:db8::203:0:113:1/128"
	v4PrefixLen              = 30
	v6PrefixLen              = 126
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
	port1Name = "port1"
	port2Name = "port2"

	dutAttrs = []*attrs.Attributes{
		&dutP1,
		&dutP2,
	}
	ateAttrs = []*attrs.Attributes{
		&ateP1,
		&ateP2,
	}
	portNames = []string{port1Name, port2Name}

	wantIPv4NHs          = map[string]bool{ateP1.IPv4: true, ateP2.IPv4: true}
	wantIPv6NHs          = map[string]bool{ateP1.IPv6: true, ateP2.IPv6: true}
	wantIPv4NHsPostChurn = map[string]bool{ateP1.IPv4: true}
)

// getPostChurnIPv6NH returns the expected IPv6 next hops after a churn event.
// It returns a map of IP addresses to a boolean indicating if the address is expected.
func getPostChurnIPv6NH(dut *ondatra.DUTDevice) map[string]bool {
	if deviations.LinkLocalInsteadOfNh(dut) {
		return map[string]bool{linkLocalAddress: true}
	}
	return map[string]bool{ateP1.IPv6: true}
}

func (tc *testCase) configureDUTRoutingPolicy(t *testing.T, applyPolicyName string) {
	t.Helper()
	d := &oc.Root{}
	routePolicy := d.GetOrCreateRoutingPolicy()
	policyDefinition := routePolicy.GetOrCreatePolicyDefinition(applyPolicyName)
	statement, err := policyDefinition.AppendNewStatement("id-1")
	if err != nil {
		t.Fatalf("failed to append new statement: %v", err)
	}
	statement.GetOrCreateActions().PolicyResult = applyPolicyType
	gnmi.Update(t, tc.dut, gnmi.OC().RoutingPolicy().Config(), routePolicy)
}

func (tc *testCase) configureDUTBGP(t *testing.T, applyPolicyName string) {
	t.Helper()
	routerID := dutP1.IPv4
	bgpProtocol := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(tc.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	for _, ateAttr := range ateAttrs {
		nbrs := []*afts.BGPNeighbor{
			{IP: ateAttr.IPv4, Version: oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST},
			{IP: ateAttr.IPv6, Version: oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST},
		}
		eParams := &afts.EBGPParams{
			DUT:             tc.dut,
			DUTASN:          dutASN,
			RouterID:        routerID,
			PeerGrpNameV4:   peerGrpNameV4P1,
			PeerGrpNameV6:   peerGrpNameV6P1,
			Neighbors:       nbrs,
			ApplyPolicyName: applyPolicyName,
		}
		dutConf := afts.CreateEBGPNeighbor(t, eParams)
		gnmi.Update(t, tc.dut, bgpProtocol.Config(), dutConf)
	}
}

func (tc *testCase) configureDUTISIS(t *testing.T) {
	t.Helper()
	ts := isissession.MustNew(t).WithISIS()
	ts.ConfigISIS(func(isis *oc.NetworkInstance_Protocol_Isis) {
		global := isis.GetOrCreateGlobal()
		global.HelloPadding = oc.Isis_HelloPaddingType_DISABLE

		if deviations.ISISSingleTopologyRequired(ts.DUT) {
			afv6 := global.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST)
			afv6.GetOrCreateMultiTopology().SetAfiName(oc.IsisTypes_AFI_TYPE_IPV4)
			afv6.GetOrCreateMultiTopology().SetSafiName(oc.IsisTypes_SAFI_TYPE_UNICAST)
		}
	})
	ts.ATEIntf1.Isis().Advanced().SetEnableHelloPadding(false)
	ts.PushAndStart(t)
}

// configureDUT configures all the interfaces and BGP on the DUT.
func (tc *testCase) configureDUT(t *testing.T) {
	t.Helper()
	dut := tc.dut
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	for ix, dutAttr := range dutAttrs {
		portName := portNames[ix]
		p := dut.Port(t, portName).Name()
		i := dutAttr.NewOCInterface(p, dut)
		gnmi.Update(t, dut, gnmi.OC().Interface(p).Config(), i)

		if deviations.ExplicitPortSpeed(dut) {
			fptest.SetPortSpeed(t, dut.Port(t, portName))
		}
		if deviations.ExplicitInterfaceInDefaultVRF(dut) {
			fptest.AssignToNetworkInstance(t, dut, p, deviations.DefaultNetworkInstance(dut), 0)
		}
	}

	applyPolicyName := "ALLOW"
	tc.configureDUTRoutingPolicy(t, applyPolicyName)
	tc.configureDUTBGP(t, applyPolicyName)
	tc.configureDUTISIS(t)
}

type BGPNeighbor struct {
	as         uint32
	neighborip string
	version    IPFamily
}
type IPFamily int

const (
	// UnknownIPFamily indicates an unspecified or unknown IP address family.
	UnknownIPFamily IPFamily = iota
	IPv4
	IPv6
)

func (tc *testCase) waitForBGPSession(t *testing.T) error {
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(tc.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	nbrPath := statePath.Neighbor(ateP2.IPv4)
	nbrPathv6 := statePath.Neighbor(ateP2.IPv6)
	verifySessionState := func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
		state, ok := val.Val()
		if !ok {
			return false
		}
		t.Logf("BGP session state: %s", state.String())
		return state == oc.Bgp_Neighbor_SessionState_ESTABLISHED
	}

	_, ok := gnmi.Watch(t, tc.dut, nbrPath.SessionState().State(), bgpTimeout, verifySessionState).Await(t)
	if !ok {
		fptest.LogQuery(t, "BGP reported state", nbrPath.State(), gnmi.Get(t, tc.dut, nbrPath.State()))
		return fmt.Errorf("no BGP neighbor formed yet")
	}
	_, ok = gnmi.Watch(t, tc.dut, nbrPathv6.SessionState().State(), bgpTimeout, verifySessionState).Await(t)
	if !ok {
		fptest.LogQuery(t, "BGPv6 reported state", nbrPathv6.State(), gnmi.Get(t, tc.dut, nbrPathv6.State()))
		return fmt.Errorf("no BGPv6 neighbor formed yet")
	}
	return nil
}

func (tc *testCase) configureATE(t *testing.T) {
	ate := tc.ate
	config := gosnappi.NewConfig()

	for ix, ateAttr := range ateAttrs {
		dutAttr := dutAttrs[ix]
		portName := portNames[ix]
		ap := ate.Port(t, portName)
		p := config.Ports().Add().SetName(ap.ID())
		d := config.Devices().Add().SetName(fmt.Sprintf("%s.d%d", p.Name(), ix))
		eth := d.Ethernets().Add().SetName(d.Name() + ".eth").
			SetMac(ateAttr.MAC).
			SetMtu(mtu)
		eth.Connection().
			SetPortName(p.Name())
		ipv4 := eth.Ipv4Addresses().Add().SetName(eth.Name() + ".IPv4").
			SetAddress(ateAttr.IPv4).
			SetGateway(dutAttr.IPv4).
			SetPrefix(uint32(ateAttr.IPv4Len))
		ipv6 := eth.Ipv6Addresses().Add().SetName(eth.Name() + ".IPv6").
			SetAddress(ateAttr.IPv6).
			SetGateway(dutAttr.IPv6).
			SetPrefix(uint32(ateAttr.IPv6Len))

		isisParams := &afts.ISISParams{
			Dev:      d,
			V4Addr:   ateAttr.IPv4,
			EthName:  eth.Name(),
			PortName: ap.ID(),

			V4AdvertisedAddr:   isisRoute,
			V4AdvertisedPrefix: advertisedRoutesV4Prefix,
			V4AdvertisedCount:  isisRouteCount,

			V6AdvertisedAddr:   isisRoutev6,
			V6AdvertisedPrefix: advertisedRoutesV6Prefix,
			V6AdvertisedCount:  isisRouteCount,
		}
		afts.ConfigureAdvertisedISISRoutes(t, isisParams)

		bgpRoute := &afts.BGPRoute{
			Dev: d,

			RouterID: ateP1.IPv4,
			ASN:      ateASN,

			IPV4DevAddr:  ipv4,
			V4Prefix:     advertisedRoutesV4Prefix,
			V4RouteCount: afts.RouteCount(tc.dut, true),

			IPV6DevAddr:  ipv6,
			V6Prefix:     advertisedRoutesV6Prefix,
			V6RouteCount: afts.RouteCount(tc.dut, false),
		}
		afts.ConfigureAdvertisedEBGPRoutes(t, bgpRoute)
	}

	ate.OTG().PushConfig(t, config)
	ate.OTG().StartProtocols(t)
}

func (tc *testCase) generateWantPrefixes(t *testing.T) map[string]bool {
	wantPrefixes := make(map[string]bool)
	for pfix := range netutil.GenCIDRs(t, startingBGPRouteIPv4, int(afts.RouteCount(tc.dut, true))) {
		wantPrefixes[pfix] = true
	}
	for pfix6 := range netutil.GenCIDRs(t, startingBGPRouteIPv6, int(afts.RouteCount(tc.dut, false))) {
		wantPrefixes[pfix6] = true
	}
	return wantPrefixes
}

func (tc *testCase) verifyPrefixes(t *testing.T, aft *aftcache.AFTData, ip string, routeCount int, wantNHCount int) error {
	for pfix := range netutil.GenCIDRs(t, ip, routeCount) {
		nhgID, ok := aft.Prefixes[pfix]

		if !ok {
			return fmt.Errorf("prefix %s not found in AFT", pfix)
		}
		nhg, ok := aft.NextHopGroups[nhgID]
		if !ok {
			return fmt.Errorf("next hop group %d not found in AFT for prefix %s", nhgID, pfix)
		}

		if len(nhg.NHIDs) != wantNHCount {
			return fmt.Errorf("next hop group %d has %d next hops, want %d", nhgID, len(nhg.NHIDs), wantNHCount)
		}

		var firstWeight uint64 = 0 // Initialize with a value that won't be a valid weight
		for i := 0; i < wantNHCount; i++ {
			nhID := nhg.NHIDs[i]
			nh, ok := aft.NextHops[nhID]
			if !ok {
				return fmt.Errorf("next hop %d not found in AFT for next-hop group: %d for prefix: %s", nhID, nhgID, pfix)
			}
			// TODO: - Add check for exact interface name
			// TODO: - Remove deviation and add recursive check for interface
			if !deviations.SkipInterfaceNameCheck(tc.dut) {
				if nh.IntfName == "" {
					return fmt.Errorf("next hop interface not found in AFT for next-hop: %d for prefix: %s", nhID, pfix)
				}
			}
			if nh.IP == "" {
				return fmt.Errorf("next hop IP not found in AFT for next-hop: %d for prefix: %s", nhID, pfix)
			}
			weight, ok := nhg.NHWeights[nhID]
			if !ok {
				return fmt.Errorf("next hop weight not found in AFT for next-hop: %d for prefix: %s", nhID, pfix)
			}
			if weight <= 0 {
				return fmt.Errorf("next hop weight are not proper for next-hop: %d for prefix: %s", nhID, pfix)
			}
			// Check if weights are equal
			if firstWeight == 0 { // This is the first next hop, set the reference weight
				firstWeight = weight
			} else if weight != firstWeight { // Compare with the first encountered weight
				return fmt.Errorf("next hop group %d has unequal weights. Expected %d, got %d for next-hop %d for prefix %s", nhgID, firstWeight, weight, nhID, pfix)
			}
		}
	}
	return nil
}

func (tc *testCase) cache(t *testing.T, stoppingCondition aftcache.PeriodicHook) (*aftcache.AFTData, error) {
	t.Helper()
	aftSession := aftcache.NewAFTStreamSession(t.Context(), t, tc.gnmiClient, tc.dut)
	aftSession.ListenUntil(t.Context(), t, aftConvergenceTime, stoppingCondition)

	// Get the AFT from the cache.
	aft, err := aftSession.Cache.ToAFT(tc.dut)
	if err != nil {
		return nil, fmt.Errorf("error getting AFT: %v", err)
	}
	return aft, nil
}

func (tc *testCase) otgInterfaceState(t *testing.T, portName string, state gosnappi.StatePortLinkStateEnum) {
	portStateAction := gosnappi.NewControlState()
	portStateAction.Port().Link().SetPortNames([]string{portName}).SetState(state)
	tc.ate.OTG().SetControlState(t, portStateAction)
}

type testCase struct {
	name       string
	dut        *ondatra.DUTDevice
	ate        *ondatra.ATEDevice
	gnmiClient gnmipb.GNMIClient
}

func TestBGP(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	gnmiClient, err := dut.RawAPIs().BindingDUT().DialGNMI(t.Context())
	if err != nil {
		t.Fatalf("Failed to dial GNMI: %v", err)
	}
	tc := &testCase{
		name:       "AFT Churn Test With Scale",
		dut:        dut,
		ate:        ate,
		gnmiClient: gnmiClient,
	}

	// Pre-generate all expected prefixes once for efficiency
	wantPrefixes := tc.generateWantPrefixes(t)

	// Helper function for verifying AFT state when given prefixes and expected next hops.
	verifyAFTState := func(desc string, wantNHCount int, wantV4NHs, wantV6NHs map[string]bool) *aftcache.AFTData {
		t.Helper()
		t.Log(desc)
		stoppingCondition := aftcache.InitialSyncStoppingCondition(t, dut, wantPrefixes, wantV4NHs, wantV6NHs)
		aft, err := tc.cache(t, stoppingCondition)
		if err != nil {
			t.Fatalf("failed to get AFT Cache: %v", err)
		}
		if err := tc.verifyPrefixes(t, aft, startingBGPRouteIPv4, int(afts.RouteCount(dut, true)), wantNHCount); err != nil {
			t.Errorf("failed to verify IPv4 BGP prefixes: %v", err)
		}
		if err := tc.verifyPrefixes(t, aft, startingBGPRouteIPv6, int(afts.RouteCount(dut, false)), wantNHCount); err != nil {
			t.Errorf("failed to verify IPv6 BGP prefixes: %v", err)
		}
		return aft
	}

	// --- Test Setup ---
	tc.configureDUT(t)
	tc.configureATE(t)

	t.Log("Waiting for BGPv4 neighbor to establish...")
	if err := tc.waitForBGPSession(t); err != nil {
		t.Fatalf("Unable to establish BGP session: %v", err)
	}

	// Step 1: Initial state verification (BGP: 2 NHs, ISIS: 1 NH)
	aft := verifyAFTState("Initial AFT verification", 2, wantIPv4NHs, wantIPv6NHs)

	// Verify ISIS prefixes are present in AFT.
	if err := tc.verifyPrefixes(t, aft, startingISISRouteIPv4, isisRouteCount, 1); err != nil {
		t.Errorf("failed to verify IPv4 ISIS prefixes: %v", err)
	}
	if err := tc.verifyPrefixes(t, aft, startingISISRouteIPv6, isisRouteCount, 1); err != nil {
		t.Errorf("failed to verify IPv6 ISIS prefixes: %v", err)
	}
	t.Log("ISIS verification successful")

	// Step 2: Stop Port2 interface to create Churn (BGP: 1 NH)
	t.Log("Stopping Port2 interface to create Churn")
	tc.otgInterfaceState(t, port2Name, gosnappi.StatePortLinkState.DOWN)
	verifyAFTState("AFT verification after port 2 churn", 1, wantIPv4NHsPostChurn, getPostChurnIPv6NH(tc.dut))

	// Step 3: Stop Port1 interface to create full Churn (BGP: deletion expected)
	t.Log("Stopping Port1 interface to create Churn")
	tc.otgInterfaceState(t, port1Name, gosnappi.StatePortLinkState.DOWN)
	if _, err := tc.cache(t, aftcache.DeletionStoppingCondition(t, dut, wantPrefixes)); err != nil {
		t.Fatalf("failed to get AFT Cache after deletion: %v", err)
	}

	// Step 4: Start Port1 interface to remove Churn (BGP: 1 NH - Port2 still down)
	t.Log("Starting Port1 interface to remove Churn")
	tc.otgInterfaceState(t, port1Name, gosnappi.StatePortLinkState.UP)
	verifyAFTState("AFT verification after port 1 up", 1, wantIPv4NHsPostChurn, getPostChurnIPv6NH(tc.dut))

	// Step 5: Start Port2 interface to remove Churn (BGP: 2 NHs - full recovery)
	t.Log("Starting Port2 interface to remove Churn")
	tc.otgInterfaceState(t, port2Name, gosnappi.StatePortLinkState.UP)
	verifyAFTState("AFT verification after port 2 up", 2, wantIPv4NHs, wantIPv6NHs)
}
