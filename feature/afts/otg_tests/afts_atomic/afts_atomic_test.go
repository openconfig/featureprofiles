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

	"github.com/open_traffic_generator/gosnappi"
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

	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	advertisedRoutesV4Prefix  = 32
	advertisedRoutesV6Prefix  = 128
	aftConvergenceTime        = 20 * time.Minute
	applyPolicyName           = "ALLOW"
	applyPolicyType           = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
	ateAS                     = 200
	bgpRoute                  = "200.0.0.0"
	bgpRouteCountIPv4Default  = 2000000
	bgpRouteCountIPv4LowScale = 100000
	bgpRouteCountIPv6Default  = 1000000
	bgpRouteCountIPv6LowScale = 100000
	bgpRoutev6                = "3001:1::0"
	bgpTimeout                = 2 * time.Minute
	dutAS                     = 65501
	isisArea                  = "49.0001"
	isisRoute                 = "199.0.0.1"
	isisRouteCount            = 100
	isisRoutev6               = "2001:db8::203:0:113:1"
	isisSystemID              = "1920.0000.2001"
	linkLocalAddress          = "fe80::200:2ff:fe02:202"
	mtu                       = 1500
	peerGrpNameV4P1           = "BGP-PEER-GROUP-V4-P1"
	peerGrpNameV4P2           = "BGP-PEER-GROUP-V4-P2"
	peerGrpNameV6P1           = "BGP-PEER-GROUP-V6-P1"
	peerGrpNameV6P2           = "BGP-PEER-GROUP-V6-P2"
	port1MAC                  = "00:00:02:02:02:02"
	port2MAC                  = "00:00:03:03:03:03"
	startingBGPRouteIPv4      = "200.0.0.0/32"
	startingBGPRouteIPv6      = "3001:1::0/128"
	startingISISRouteIPv4     = "199.0.0.1/32"
	startingISISRouteIPv6     = "2001:db8::203:0:113:1/128"
	v4PrefixLen               = 30
	v6PrefixLen               = 126
)

var (
	ateP1 = attrs.Attributes{
		IPv4:    "192.0.2.2",
		IPv4Len: v4PrefixLen,
		IPv6:    "2001:db8::2",
		IPv6Len: v6PrefixLen,
		MAC:     port1MAC,
	}
	ateP2 = attrs.Attributes{
		IPv4:    "192.0.2.6",
		IPv4Len: v4PrefixLen,
		IPv6:    "2001:db8::6",
		IPv6Len: v6PrefixLen,
		MAC:     port2MAC,
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

	nbrs1 = []*BGPNeighbor{
		{as: ateAS, neighborip: ateP1.IPv4, version: IPv4},
		{as: ateAS, neighborip: ateP1.IPv6, version: IPv6},
	}
	nbrs2 = []*BGPNeighbor{
		{as: ateAS, neighborip: ateP2.IPv4, version: IPv4},
		{as: ateAS, neighborip: ateP2.IPv6, version: IPv6},
	}

	port1Name            = "port1"
	port2Name            = "port2"
	wantIPv4NHs          = map[string]bool{ateP1.IPv4: true, ateP2.IPv4: true}
	wantIPv4NHsPostChurn = map[string]bool{ateP1.IPv4: true}
	wantIPv6NHs          = map[string]bool{ateP1.IPv6: true, ateP2.IPv6: true}
)

// routeCount returns the expected route count for the given dut and IP family.
func routeCount(dut *ondatra.DUTDevice, afi IPFamily) uint32 {
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

// configureDUT configures all the interfaces and BGP on the DUT.
func (tc *testCase) configureDUT(t *testing.T) error {
	dut := tc.dut
	p1 := dut.Port(t, port1Name).Name()
	i1 := dutP1.NewOCInterface(p1, dut)
	gnmi.Update(t, dut, gnmi.OC().Interface(p1).Config(), i1)

	p2 := dut.Port(t, port2Name).Name()
	i2 := dutP2.NewOCInterface(p2, dut)
	gnmi.Update(t, dut, gnmi.OC().Interface(p2).Config(), i2)

	// Configure default network instance.
	t.Log("Configure Default Network Instance")
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, dut.Port(t, port1Name))
		fptest.SetPortSpeed(t, dut.Port(t, port2Name))
	}
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, p1, deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, p2, deviations.DefaultNetworkInstance(dut), 0)
	}

	d := &oc.Root{}
	routePolicy := d.GetOrCreateRoutingPolicy()
	policyDefinition := routePolicy.GetOrCreatePolicyDefinition(applyPolicyName)
	policyStatement, err := policyDefinition.AppendNewStatement("policy-1")
	if err != nil {
		return err
	}
	policyStatement.GetOrCreateActions().PolicyResult = applyPolicyType
	gnmi.Update(t, dut, gnmi.OC().RoutingPolicy().Config(), routePolicy)

	dutNIProtocol := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).
		Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	niProtocol := configureBGP(t, dut)
	gnmi.Update(t, dut, dutNIProtocol.Config(), niProtocol)

	b := &gnmi.SetBatch{}
	isisData := &cfgplugins.ISISGlobalParams{
		DUTArea:             isisArea,
		DUTSysID:            isisSystemID,
		NetworkInstanceName: deviations.DefaultNetworkInstance(dut),
		ISISInterfaceNames:  []string{p1, p2},
	}

	isisRoot := cfgplugins.NewISIS(t, dut, isisData, b)
	if deviations.ISISSingleTopologyRequired(dut) {
		niName := deviations.DefaultNetworkInstance(dut)
		isis := isisRoot.GetOrCreateNetworkInstance(niName).
			GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisData.NetworkInstanceName).
			GetOrCreateIsis()
		multiTopology := isis.GetOrCreateGlobal().
			GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).
			GetOrCreateMultiTopology()
		multiTopology.SetAfiName(oc.IsisTypes_AFI_TYPE_IPV4)
		multiTopology.SetSafiName(oc.IsisTypes_SAFI_TYPE_UNICAST)
	}
	b.Set(t, dut)

	return nil
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

func configureBGP(t *testing.T, dut *ondatra.DUTDevice) *oc.NetworkInstance_Protocol {
	d := &oc.Root{}
	ni := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niProtocol := ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := niProtocol.GetOrCreateBgp()

	global := bgp.GetOrCreateGlobal()
	global.SetAs(dutAS)
	global.SetRouterId(dutP1.IPv4)

	afiSAFI := global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
	afiSAFI.SetEnabled(true)
	afiSAFI.GetOrCreateUseMultiplePaths().GetOrCreateEbgp().SetMaximumPaths(2)
	afiSAFIV6 := global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
	afiSAFIV6.SetEnabled(true)
	afiSAFIV6.GetOrCreateUseMultiplePaths().GetOrCreateEbgp().SetMaximumPaths(2)

	configureBGPPeerGroup(bgp, peerGrpNameV4P1, peerGrpNameV6P1, nbrs1)
	configureBGPPeerGroup(bgp, peerGrpNameV4P2, peerGrpNameV6P2, nbrs2)

	return niProtocol
}

func configureBGPPeerGroup(bgp *oc.NetworkInstance_Protocol_Bgp, peerGrpNameV4, peerGrpNameV6 string, nbrs []*BGPNeighbor) {
	// Note: we have to define the peer group even if we aren't setting any policy because it's
	// invalid OC for the neighbor to be part of a peer group that doesn't exist.
	peerGroupV4 := bgp.GetOrCreatePeerGroup(peerGrpNameV4)
	peerGroupV4.SetPeerAs(ateAS)
	peerGroupV6 := bgp.GetOrCreatePeerGroup(peerGrpNameV6)
	peerGroupV6.SetPeerAs(ateAS)

	peerGroupV4AfiSafi := peerGroupV4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
	peerGroupV4AfiSafi.SetEnabled(true)
	peerGroupV4AfiSafi.GetOrCreateUseMultiplePaths().SetEnabled(true)
	applyPolicyV4 := peerGroupV4AfiSafi.GetOrCreateApplyPolicy()
	applyPolicyV4.ImportPolicy = []string{applyPolicyName}
	applyPolicyV4.ExportPolicy = []string{applyPolicyName}

	peerGroupV6AfiSafi := peerGroupV6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
	peerGroupV6AfiSafi.SetEnabled(true)
	peerGroupV6AfiSafi.GetOrCreateUseMultiplePaths().SetEnabled(true)
	applyPolicyV6 := peerGroupV6AfiSafi.GetOrCreateApplyPolicy()
	applyPolicyV6.ImportPolicy = []string{applyPolicyName}
	applyPolicyV6.ExportPolicy = []string{applyPolicyName}

	for _, nbr := range nbrs {
		neighbor := bgp.GetOrCreateNeighbor(nbr.neighborip)
		neighbor.SetPeerAs(nbr.as)
		neighbor.SetEnabled(true)
		switch nbr.version {
		case IPv4:
			neighbor.SetPeerGroup(peerGrpNameV4)
			neighbor.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).SetEnabled(true)
		case IPv6:
			neighbor.SetPeerGroup(peerGrpNameV6)
			neighbor.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).SetEnabled(true)
		}
	}
}

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

	if _, ok := gnmi.Watch(t, tc.dut, nbrPath.SessionState().State(), bgpTimeout, verifySessionState).Await(t); !ok {
		fptest.LogQuery(t, "BGP reported state", nbrPath.State(), gnmi.Get(t, tc.dut, nbrPath.State()))
		return fmt.Errorf("no BGP neighbor formed yet")
	}

	if _, ok := gnmi.Watch(t, tc.dut, nbrPathv6.SessionState().State(), bgpTimeout, verifySessionState).Await(t); !ok {
		fptest.LogQuery(t, "BGPv6 reported state", nbrPathv6.State(), gnmi.Get(t, tc.dut, nbrPathv6.State()))
		return fmt.Errorf("no BGPv6 neighbor formed yet")
	}

	return nil
}

func (tc *testCase) configureATE(t *testing.T) {
	ate := tc.ate
	config := gosnappi.NewConfig()

	portData := []struct {
		portName      string
		ateAttrs      attrs.Attributes
		dutAttrs      attrs.Attributes
		addISISRoutes bool
	}{
		{
			portName:      port1Name,
			ateAttrs:      ateP1,
			dutAttrs:      dutP1,
			addISISRoutes: true,
		},
		{
			portName:      port2Name,
			ateAttrs:      ateP2,
			dutAttrs:      dutP2,
			addISISRoutes: false, // README specifies to only advertise ISIS routes over one port.
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
			SetSystemId(strings.ReplaceAll(isisSystemID, ".", ""))
		isis.Basic().
			SetIpv4TeRouterId(ipv4.Address()).
			SetHostname(fmt.Sprintf("ixia-c-port%d", i+1))
		isis.Advanced().SetAreaAddresses([]string{strings.ReplaceAll(isisArea, ".", "")})
		isis.Advanced().SetEnableHelloPadding(false)
		isisInt := isis.Interfaces().Add().SetName(isis.Name() + ".intf").
			SetEthName(eth.Name()).
			SetNetworkType(gosnappi.IsisInterfaceNetworkType.POINT_TO_POINT).
			SetLevelType(gosnappi.IsisInterfaceLevelType.LEVEL_2).
			SetMetric(10)
		isisInt.TrafficEngineering().Add().PriorityBandwidths()
		isisInt.Advanced().SetAutoAdjustMtu(true).SetAutoAdjustArea(true).SetAutoAdjustSupportedProtocols(true)

		if p.addISISRoutes {
			v4Route := isis.V4Routes().Add().SetName(isis.Name() + ".rr")
			v4Route.Addresses().
				Add().
				SetAddress(isisRoute).
				SetPrefix(advertisedRoutesV4Prefix).SetCount(isisRouteCount)
			v6Route := isis.V6Routes().Add().SetName(isis.Name() + ".v6")
			v6Route.Addresses().
				Add().
				SetAddress(isisRoutev6).
				SetPrefix(advertisedRoutesV6Prefix).SetCount(isisRouteCount)
		}
		tc.configureBGPDev(dev, ipv4, ipv6)
	}

	ate.OTG().PushConfig(t, config)
	ate.OTG().StartProtocols(t)
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
		SetCount(routeCount(tc.dut, IPv4))

	routesV6 := bgp6Peer.V6Routes().Add().SetName(bgp6Peer.Name() + ".v6route")
	routesV6.SetNextHopIpv6Address(ipv6.Address()).
		SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).
		SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL)
	routesV6.Addresses().Add().
		SetAddress(bgpRoutev6).
		SetPrefix(advertisedRoutesV6Prefix).
		SetCount(routeCount(tc.dut, IPv6))
}

func (tc *testCase) generateWantPrefixes(t *testing.T) map[string]bool {
	wantPrefixes := make(map[string]bool)
	for pfix := range netutil.GenCIDRs(t, startingBGPRouteIPv4, int(routeCount(tc.dut, IPv4))) {
		wantPrefixes[pfix] = true
	}
	for pfix6 := range netutil.GenCIDRs(t, startingBGPRouteIPv6, int(routeCount(tc.dut, IPv6))) {
		wantPrefixes[pfix6] = true
	}
	return wantPrefixes
}

// otgInterfaceState sets the state of the provided port.
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

func TestAtomic(t *testing.T) {
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

	wantPrefixes := tc.generateWantPrefixes(t)

	if err := tc.configureDUT(t); err != nil {
		t.Fatalf("failed to configure DUT: %v", err)
	}
	tc.configureATE(t)

	t.Log("Waiting for BGPv4 neighbor to establish...")
	if err := tc.waitForBGPSession(t); err != nil {
		t.Fatalf("Unable to establish BGP session: %v", err)
	}

	allPrefixesStoppingCondition := aftcache.InitialSyncStoppingCondition(t, dut, wantPrefixes, wantIPv4NHs, wantIPv6NHs)
	aftSession := aftcache.NewAFTStreamSession(t.Context(), t, tc.gnmiClient, tc.dut)
	aftSession.ListenUntilPreUpdateHook(t.Context(), t, aftConvergenceTime, []aftcache.NotificationHook{aftcache.VerifyAtomicFlagHook(t)}, allPrefixesStoppingCondition)

	t.Log("Stopping Port1 interface to create churn.")
	tc.otgInterfaceState(t, port1Name, gosnappi.StatePortLinkState.DOWN)
	t.Log("Stopping Port2 interface to create churn.")
	tc.otgInterfaceState(t, port2Name, gosnappi.StatePortLinkState.DOWN)
	aftSession.ListenUntilPreUpdateHook(t.Context(), t, aftConvergenceTime, []aftcache.NotificationHook{aftcache.VerifyAtomicFlagHook(t)}, aftcache.DeletionStoppingCondition(t, dut, wantPrefixes))

	t.Log("Starting Port1 interface to restore missing routes.")
	tc.otgInterfaceState(t, port1Name, gosnappi.StatePortLinkState.UP)
	t.Log("Starting Port2 interface to restore missing routes.")
	tc.otgInterfaceState(t, port2Name, gosnappi.StatePortLinkState.UP)

	aftSession.ListenUntilPreUpdateHook(t.Context(), t, aftConvergenceTime, []aftcache.NotificationHook{aftcache.VerifyAtomicFlagHook(t)}, allPrefixesStoppingCondition)
}
