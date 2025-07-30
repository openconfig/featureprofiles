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

	"github.com/open-traffic-generator/snappi/gosnappi"
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
	advertisedRoutesV4Prefix  = 32
	advertisedRoutesV6Prefix  = 128
	dutAS                     = 65501
	ateAS                     = 200
	v4PrefixLen               = 30
	v6PrefixLen               = 126
	mtu                       = 1500
	isisSystemID              = "650000000001"
	applyPolicyType           = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
	applyPolicyName           = "ALLOW"
	peerGrpNameV4P1           = "BGP-PEER-GROUP-V4-P1"
	peerGrpNameV6P1           = "BGP-PEER-GROUP-V6-P1"
	peerGrpNameV4P2           = "BGP-PEER-GROUP-V4-P2"
	peerGrpNameV6P2           = "BGP-PEER-GROUP-V6-P2"
	port1MAC                  = "00:00:02:02:02:02"
	port2MAC                  = "00:00:03:03:03:03"
	bgpRoute                  = "200.0.0.0"
	bgpRoutev6                = "3001:1::0"
	startingBGPRouteIPv4      = "200.0.0.0/32"
	startingBGPRouteIPv6      = "3001:1::0/128"
	isisRouteCount            = 100
	isisRoute                 = "199.0.0.1"
	isisRoutev6               = "2001:db8::203:0:113:1"
	startingISISRouteIPv4     = "199.0.0.1/32"
	startingISISRouteIPv6     = "2001:db8::203:0:113:1/128"
	aftConvergenceTime        = 20 * time.Minute
	bgpTimeout                = 2 * time.Minute
	linkLocalAddress          = "fe80::200:2ff:fe02:202"
	bgpRouteCountIPv4LowScale = 100000
	bgpRouteCountIPv6LowScale = 100000
	bgpRouteCountIPv4Default  = 2000000
	bgpRouteCountIPv6Default  = 1000000
)

var (
	dutP1 = attrs.Attributes{
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::1",
		IPv4Len: v4PrefixLen,
		IPv6Len: v6PrefixLen,
	}
	ateP1 = attrs.Attributes{
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::2",
		IPv4Len: v4PrefixLen,
		IPv6Len: v6PrefixLen,
	}
	dutP2 = attrs.Attributes{
		IPv4:    "192.0.2.5",
		IPv6:    "2001:db8::5",
		IPv4Len: v4PrefixLen,
		IPv6Len: v6PrefixLen,
	}
	ateP2 = attrs.Attributes{
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::6",
		IPv4Len: v4PrefixLen,
		IPv6Len: v6PrefixLen,
	}
	wantIPv4NHs          = map[string]bool{ateP1.IPv4: true, ateP2.IPv4: true}
	wantIPv6NHs          = map[string]bool{ateP1.IPv6: true, ateP2.IPv6: true}
	wantIPv4NHsPostChurn = map[string]bool{ateP1.IPv4: true}
	port1Name            = "port1"
	port2Name            = "port2"
)

// getRouteCount returns the expected route count for the given dut and IP family.
func getRouteCount(dut *ondatra.DUTDevice, afi IPFamily) uint32 {
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

// getPostChurnIPv6NH returns the expected IPv6 next hops after a churn event.
// It returns a map of IP addresses to a boolean indicating if the address is expected.
func getPostChurnIPv6NH(dut *ondatra.DUTDevice) map[string]bool {
	if deviations.LinkLocalInsteadOfNh(dut) {
		return map[string]bool{linkLocalAddress: true}
	}
	return map[string]bool{ateP1.IPv6: true}
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

	// Configure Network instance type on DUT.
	t.Log("Configure/update Network Instance")
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
	statement, err := policyDefinition.AppendNewStatement("id-1")
	if err != nil {
		return err
	}
	statement.GetOrCreateActions().PolicyResult = applyPolicyType
	gnmi.Update(t, dut, gnmi.OC().RoutingPolicy().Config(), routePolicy)

	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	nbrs := []*BGPNeighbor{
		{as: ateAS, neighborip: ateP1.IPv4, version: IPv4},
		{as: ateAS, neighborip: ateP1.IPv6, version: IPv6},
	}
	dutConf := createBGPNeighbor(peerGrpNameV4P1, peerGrpNameV6P1, nbrs, dut)
	gnmi.Update(t, dut, dutConfPath.Config(), dutConf)
	nbrs = []*BGPNeighbor{
		{as: ateAS, neighborip: ateP2.IPv4, version: IPv4},
		{as: ateAS, neighborip: ateP2.IPv6, version: IPv6},
	}
	dutConf = createBGPNeighbor(peerGrpNameV4P2, peerGrpNameV6P2, nbrs, dut)
	gnmi.Update(t, dut, dutConfPath.Config(), dutConf)
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

	if err := ts.PushAndStart(t); err != nil {
		return err
	}

	if _, err = ts.AwaitAdjacency(); err != nil {
		return fmt.Errorf("no IS-IS adjacency formed: %v", err)
	}
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

func createBGPNeighbor(peerGrpNameV4, peerGrpNameV6 string, nbrs []*BGPNeighbor, dut *ondatra.DUTDevice) *oc.NetworkInstance_Protocol {
	d := &oc.Root{}
	ni := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niProtocol := ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := niProtocol.GetOrCreateBgp()

	global := bgp.GetOrCreateGlobal()
	global.SetAs(dutAS)
	global.SetRouterId(dutP1.IPv4)

	// Note: we have to define the peer group even if we aren't setting any policy because it's
	// invalid OC for the neighbor to be part of a peer group that doesn't exist.
	peerGroupV4 := bgp.GetOrCreatePeerGroup(peerGrpNameV4)
	peerGroupV4.SetPeerAs(ateAS)
	peerGroupV6 := bgp.GetOrCreatePeerGroup(peerGrpNameV6)
	peerGroupV6.SetPeerAs(ateAS)

	afiSAFI := global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
	afiSAFI.SetEnabled(true)
	afiSAFI.GetOrCreateUseMultiplePaths().GetOrCreateEbgp().SetMaximumPaths(2)
	asisafi6 := global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
	asisafi6.SetEnabled(true)
	asisafi6.GetOrCreateUseMultiplePaths().GetOrCreateEbgp().SetMaximumPaths(2)

	peerGroupV4AfiSafi := peerGroupV4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
	peerGroupV4AfiSafi.SetEnabled(true)
	peerGroupV4AfiSafi.GetOrCreateUseMultiplePaths().SetEnabled(true)
	peerGroupV6AfiSafi := peerGroupV6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
	peerGroupV6AfiSafi.SetEnabled(true)
	peerGroupV6AfiSafi.GetOrCreateUseMultiplePaths().SetEnabled(true)

	for _, nbr := range nbrs {
		neighbor := bgp.GetOrCreateNeighbor(nbr.neighborip)
		neighbor.SetPeerAs(nbr.as)
		neighbor.SetEnabled(true)
		switch nbr.version {
		case IPv4:
			neighbor.SetPeerGroup(peerGrpNameV4)
			neighbor.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).SetEnabled(true)
			neighbourAFV4 := peerGroupV4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
			neighbourAFV4.SetEnabled(true)
			applyPolicy := neighbourAFV4.GetOrCreateApplyPolicy()
			applyPolicy.ImportPolicy = []string{applyPolicyName}
			applyPolicy.ExportPolicy = []string{applyPolicyName}
		case IPv6:
			neighbor.SetPeerGroup(peerGrpNameV6)
			neighbor.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).SetEnabled(true)
			neighbourAFV6 := peerGroupV6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
			neighbourAFV6.SetEnabled(true)
			applyPolicy := neighbourAFV6.GetOrCreateApplyPolicy()
			applyPolicy.ImportPolicy = []string{applyPolicyName}
			applyPolicy.ExportPolicy = []string{applyPolicyName}
		}
	}
	return niProtocol
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
	ap1 := ate.Port(t, port1Name)
	ap2 := ate.Port(t, port2Name)
	config := gosnappi.NewConfig()
	// add ports
	p1 := config.Ports().Add().SetName(ap1.ID())
	p2 := config.Ports().Add().SetName(ap2.ID())
	// add devices
	d1 := config.Devices().Add().SetName(p1.Name() + ".d1")
	d2 := config.Devices().Add().SetName(p2.Name() + ".d2")

	// Configuration on port1.
	d1Eth := d1.Ethernets().
		Add().
		SetName(d1.Name() + ".eth").
		SetMac(port1MAC).
		SetMtu(mtu)
	d1Eth.
		Connection().
		SetPortName(p1.Name())

	d1IPv4 := d1Eth.
		Ipv4Addresses().
		Add().
		SetName(d1Eth.Name() + ".IPv4").
		SetAddress(ateP1.IPv4).
		SetGateway(dutP1.IPv4).
		SetPrefix(v4PrefixLen)

	d1IPv6 := d1Eth.
		Ipv6Addresses().
		Add().
		SetName(d1Eth.Name() + ".IPv6").
		SetAddress(ateP1.IPv6).
		SetGateway(dutP1.IPv6).
		SetPrefix(v6PrefixLen)

	d1ISIS := d1.Isis().
		SetName(d1.Name() + ".isis").
		SetSystemId(isisSystemID)
	d1ISIS.Basic().
		SetIpv4TeRouterId(d1IPv4.Address()).
		SetHostname("ixia-c-port1")
	d1ISIS.Advanced().SetAreaAddresses([]string{"49"})
	d1ISISInt := d1ISIS.Interfaces().
		Add().
		SetName(d1ISIS.Name() + ".intf").
		SetEthName(d1Eth.Name()).
		SetNetworkType(gosnappi.IsisInterfaceNetworkType.POINT_TO_POINT).
		SetLevelType(gosnappi.IsisInterfaceLevelType.LEVEL_2).
		SetMetric(10)
	d1ISISInt.TrafficEngineering().Add().PriorityBandwidths()
	d1ISISInt.Advanced().SetAutoAdjustMtu(true).SetAutoAdjustArea(true).SetAutoAdjustSupportedProtocols(true)

	d1ISISRoute := d1ISIS.V4Routes().Add().SetName(d1ISIS.Name() + ".rr")
	d1ISISRoute.Addresses().
		Add().
		SetAddress(isisRoute).
		SetPrefix(advertisedRoutesV4Prefix).SetCount(isisRouteCount)

	d1ISISRouteV6 := d1ISIS.V6Routes().Add().SetName(d1ISISRoute.Name() + ".v6")
	d1ISISRouteV6.Addresses().
		Add().
		SetAddress(isisRoutev6).
		SetPrefix(advertisedRoutesV6Prefix).SetCount(isisRouteCount)

	tc.configureBGPDev(d1, d1IPv4, d1IPv6)

	// Configuration on port2
	d2Eth := d2.Ethernets().
		Add().
		SetName(d2.Name() + ".eth").
		SetMac(port2MAC).
		SetMtu(mtu)
	d2Eth.
		Connection().
		SetPortName(p2.Name())
	d2IPv4 := d2Eth.Ipv4Addresses().
		Add().
		SetName(d2Eth.Name() + ".IPv4").
		SetAddress(ateP2.IPv4).
		SetGateway(dutP2.IPv4).
		SetPrefix(v4PrefixLen)

	d2IPv6 := d2Eth.
		Ipv6Addresses().
		Add().
		SetName(d2Eth.Name() + ".IPv6").
		SetAddress(ateP2.IPv6).
		SetGateway(dutP2.IPv6).
		SetPrefix(v6PrefixLen)

	d2ISIS := d2.Isis().
		SetName(d2.Name() + ".isis").
		SetSystemId(isisSystemID)
	d2ISIS.Basic().
		SetIpv4TeRouterId(d2IPv4.Address()).
		SetHostname("ixia-c-port2")
	d2ISIS.Advanced().SetAreaAddresses([]string{"49"})
	d2ISISInt := d2ISIS.Interfaces().
		Add().
		SetName(d2ISIS.Name() + ".intf").
		SetEthName(d2Eth.Name()).
		SetNetworkType(gosnappi.IsisInterfaceNetworkType.POINT_TO_POINT).
		SetLevelType(gosnappi.IsisInterfaceLevelType.LEVEL_2).
		SetMetric(10)
	d2ISISInt.TrafficEngineering().Add().PriorityBandwidths()
	d2ISISInt.Advanced().SetAutoAdjustMtu(true).SetAutoAdjustArea(true).SetAutoAdjustSupportedProtocols(true)

	d2ISISRoute := d2ISIS.V4Routes().Add().SetName(d2ISIS.Name() + ".rr")
	d2ISISRoute.Addresses().
		Add().
		SetAddress(isisRoute).
		SetPrefix(advertisedRoutesV4Prefix).
		SetCount(isisRouteCount)

	d2ISISRouteV6 := d2ISIS.V6Routes().Add().SetName(d2ISISRoute.Name() + ".v6")
	d2ISISRouteV6.Addresses().
		Add().
		SetAddress(isisRoutev6).
		SetPrefix(advertisedRoutesV6Prefix).
		SetCount(isisRouteCount)

	tc.configureBGPDev(d2, d2IPv4, d2IPv6)

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
		SetCount(getRouteCount(tc.dut, IPv4))

	routesV6 := bgp6Peer.V6Routes().Add().SetName(bgp6Peer.Name() + ".v6route")
	routesV6.SetNextHopIpv6Address(ipv6.Address()).
		SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).
		SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL)
	routesV6.Addresses().Add().
		SetAddress(bgpRoutev6).
		SetPrefix(advertisedRoutesV6Prefix).
		SetCount(getRouteCount(tc.dut, IPv6))
}

func (tc *testCase) generateWantPrefixes(t *testing.T) map[string]bool {
	wantPrefixes := make(map[string]bool)
	for pfix := range netutil.GenCIDRs(t, startingBGPRouteIPv4, int(getRouteCount(tc.dut, IPv4))) {
		wantPrefixes[pfix] = true
	}
	for pfix6 := range netutil.GenCIDRs(t, startingBGPRouteIPv6, int(getRouteCount(tc.dut, IPv6))) {
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
		if err := tc.verifyPrefixes(t, aft, startingBGPRouteIPv4, int(getRouteCount(dut, IPv4)), wantNHCount); err != nil {
			t.Errorf("failed to verify IPv4 BGP prefixes: %v", err)
		}
		if err := tc.verifyPrefixes(t, aft, startingBGPRouteIPv6, int(getRouteCount(dut, IPv6)), wantNHCount); err != nil {
			t.Errorf("failed to verify IPv6 BGP prefixes: %v", err)
		}
		return aft
	}

	// --- Test Setup ---
	if err := tc.configureDUT(t); err != nil {
		t.Fatalf("failed to configure DUT: %v", err)
	}
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
