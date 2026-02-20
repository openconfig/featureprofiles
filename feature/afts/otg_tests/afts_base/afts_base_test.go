//
// Copyright 2025 Google LLC
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
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/featureprofiles/internal/isissession"
	"github.com/openconfig/featureprofiles/internal/telemetry/aftcache"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygnmi/ygnmi"

	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	spb "github.com/openconfig/gnoi/system"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	advertisedRoutesV4Prefix     = 32
	advertisedRoutesV6Prefix128  = 128
	advertisedRoutesV6Prefix64   = 64
	dutAS                        = 65501
	ateAS                        = 200
	v4PrefixLen                  = 30
	v6PrefixLen                  = 126
	mtu                          = 1500
	isisSystemID                 = "650000000001"
	applyPolicyType              = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
	applyPolicyName              = "ALLOW"
	peerGrpNameV4P1              = "BGP-PEER-GROUP-V4-P1"
	peerGrpNameV6P1              = "BGP-PEER-GROUP-V6-P1"
	peerGrpNameV4P2              = "BGP-PEER-GROUP-V4-P2"
	peerGrpNameV6P2              = "BGP-PEER-GROUP-V6-P2"
	port1MAC                     = "00:00:02:02:02:02"
	port2MAC                     = "00:00:03:03:03:03"
	bgpRoute                     = "200.0.0.0"
	bgpRoutev664                 = "3001:1::0"
	bgpRoutev6128                = "4001:1::0"
	startingBGPRouteIPv4         = "200.0.0.0/32"
	startingBGPRouteIPv6128      = "4001:1::0/128"
	startingBGPRouteIPv664       = "3001:1::0/64"
	isisRouteCount               = 100
	isisRoute                    = "199.0.0.1"
	isisRoutev6                  = "2001:db8::203:0:113:1"
	startingISISRouteIPv4        = "199.0.0.1/32"
	startingISISRouteIPv6        = "2001:db8::203:0:113:1/128"
	aftConvergenceTime           = 30 * time.Minute
	bgpTimeout                   = 10 * time.Minute
	linkLocalAddress             = "fe80::200:2ff:fe02:202"
	bgpRouteCountIPv4LowScale    = 1500000
	bgpRouteCountIPv6LowScale64  = 460800
	bgpRouteCountIPv6LowScale128 = 51200
	bgpRouteCountIPv4Default     = 2000000
	bgpRouteCountIPv6Default64   = 900000
	bgpRouteCountIPv6Default128  = 100000
	maxRebootTime                = 20
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
	prevNHGIDIPv4        = uint64(0)
	prevNHGIDIPv6        = uint64(0)
)

// getRouteCount returns the expected route count for the given dut and IP family.
func getRouteCount(dut *ondatra.DUTDevice, afi IPFamily) uint32 {
	if deviations.LowScaleAft(dut) {
		if afi == IPv4 {
			return bgpRouteCountIPv4LowScale
		}
		return bgpRouteCountIPv6LowScale64 + bgpRouteCountIPv6LowScale128
	}
	if afi == IPv4 {
		return bgpRouteCountIPv4Default
	}
	return bgpRouteCountIPv6Default64 + bgpRouteCountIPv6Default128
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
		return fmt.Errorf("failed to append new statement to policy definition %s: %v", applyPolicyName, err)
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
	if deviations.BGPMissingOCMaxPrefixesConfiguration(dut) {
		updateNeighborMaxPrefix(t, dut, nbrs)
	}
	nbrs = []*BGPNeighbor{
		{as: ateAS, neighborip: ateP2.IPv4, version: IPv4},
		{as: ateAS, neighborip: ateP2.IPv6, version: IPv6},
	}
	dutConf = createBGPNeighbor(peerGrpNameV4P2, peerGrpNameV6P2, nbrs, dut)
	gnmi.Update(t, dut, dutConfPath.Config(), dutConf)
	if deviations.BGPMissingOCMaxPrefixesConfiguration(dut) {
		updateNeighborMaxPrefix(t, dut, nbrs)
	}

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
	asisafi6 := global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
	asisafi6.SetEnabled(true)

	peerGroupV4AfiSafi := peerGroupV4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
	peerGroupV4AfiSafi.SetEnabled(true)
	peerGroupV6AfiSafi := peerGroupV6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
	peerGroupV6AfiSafi.SetEnabled(true)

	if deviations.MultipathUnsupportedNeighborOrAfisafi(dut) {
		peerGroupV4.GetOrCreateUseMultiplePaths().SetEnabled(true)
		peerGroupV6.GetOrCreateUseMultiplePaths().SetEnabled(true)
	} else {
		afiSAFI.GetOrCreateUseMultiplePaths().GetOrCreateEbgp().SetMaximumPaths(2)
		asisafi6.GetOrCreateUseMultiplePaths().GetOrCreateEbgp().SetMaximumPaths(2)
		peerGroupV4AfiSafi.GetOrCreateUseMultiplePaths().SetEnabled(true)
		peerGroupV6AfiSafi.GetOrCreateUseMultiplePaths().SetEnabled(true)
	}
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

func updateNeighborMaxPrefix(t *testing.T, dut *ondatra.DUTDevice, neighbors []*BGPNeighbor) {
	for _, nbr := range neighbors {
		cfgplugins.DeviationAristaBGPNeighborMaxPrefixes(t, dut, nbr.neighborip, 0)
	}
}
func (tc *testCase) waitForBGPSessions(t *testing.T, ipv4nbrs []string, ipv6nbrs []string) error {
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(tc.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	verifySessionState := func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
		state, ok := val.Val()
		if !ok {
			t.Logf("BGP session state not found for neighbor %s", val.Path.String())
			return false
		}
		t.Logf("BGP session state for neighbor %s: %s", val.Path.String(), state.String())
		return state == oc.Bgp_Neighbor_SessionState_ESTABLISHED
	}

	for _, nbr := range ipv4nbrs {
		nbrPath := statePath.Neighbor(nbr)
		_, ok := gnmi.Watch(t, tc.dut, nbrPath.SessionState().State(), bgpTimeout, verifySessionState).Await(t)
		if !ok {
			fptest.LogQuery(t, "BGP reported state", nbrPath.State(), gnmi.Get(t, tc.dut, nbrPath.State()))
			return fmt.Errorf("BGP session with %s not established", nbr)
		}
	}
	for _, nbr := range ipv6nbrs {
		nbrPathv6 := statePath.Neighbor(nbr)
		_, ok := gnmi.Watch(t, tc.dut, nbrPathv6.SessionState().State(), bgpTimeout, verifySessionState).Await(t)
		if !ok {
			fptest.LogQuery(t, "BGPv6 reported state", nbrPathv6.State(), gnmi.Get(t, tc.dut, nbrPathv6.State()))
			return fmt.Errorf("BGP session with %s not established", nbr)
		}
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
		SetPrefix(advertisedRoutesV6Prefix128).SetCount(isisRouteCount)
	d1ISISRouteV6.Addresses().
		Add().
		SetAddress(isisRoutev6).
		SetPrefix(advertisedRoutesV6Prefix64).SetCount(isisRouteCount)

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
		SetPrefix(advertisedRoutesV6Prefix128).
		SetCount(isisRouteCount)
	d2ISISRouteV6.Addresses().
		Add().
		SetAddress(isisRoutev6).
		SetPrefix(advertisedRoutesV6Prefix64).
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
		SetAddress(bgpRoutev6128).
		SetPrefix(advertisedRoutesV6Prefix128).
		SetCount(bgpRouteCountIPv6Default128)
	routesV6.Addresses().Add().
		SetAddress(bgpRoutev664).
		SetPrefix(advertisedRoutesV6Prefix64).
		SetCount(bgpRouteCountIPv6Default64)
}

func (tc *testCase) generateWantPrefixes(t *testing.T) map[string]bool {
	wantPrefixes := make(map[string]bool)
	for pfix := range netutil.GenCIDRs(t, startingBGPRouteIPv4, int(getRouteCount(tc.dut, IPv4))) {
		wantPrefixes[pfix] = true
	}
	for pfix6128 := range netutil.GenCIDRs(t, startingBGPRouteIPv6128, int(bgpRouteCountIPv6Default128)) {
		wantPrefixes[pfix6128] = true
	}
	for pfix664 := range netutil.GenCIDRs(t, startingBGPRouteIPv664, int(bgpRouteCountIPv6Default64)) {
		wantPrefixes[pfix664] = true
	}
	return wantPrefixes
}

func (tc *testCase) verifyPrefixes(t *testing.T, aft *aftcache.AFTData, ip string, routeCount int, wantNHCount int, cacheNHGID bool) error {
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
			return fmt.Errorf("prefix %s has %d next hops, want %d", pfix, len(nhg.NHIDs), wantNHCount)
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
			if deviations.SkipInterfaceNameCheck(tc.dut) {
				isAddrIPv6 := strings.Contains(pfix, ":")
				// cache nhgIDs for BGP prefixes to verify whether the NHG has changed for next test.
				if cacheNHGID {
					prevNHGIDIPv4 = nhgID
					if isAddrIPv6 {
						prevNHGIDIPv6 = nhgID
					}
				}
			} else {
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
				return fmt.Errorf("next hop weight is %d, want > 0 for next-hop: %d for prefix: %s", weight, nhID, pfix)
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

// fetchAFT starts two independent gNMI collectors to stream AFT data from the DUT.
// It waits until both collectors satisfy the provided stoppingCondition.
// After the stopping condition is met, it compares only the wantPrefixes from both sessions.
// If the wantPrefixes data is identical, it returns a single copy of the collected AFT data.
// Otherwise, it returns an error indicating the inconsistency.
// If either stream fails, it returns an error immediately without comparison.
func (tc *testCase) fetchAFT(t *testing.T, aftSession1, aftSession2 *aftcache.AFTStreamSession, stoppingCondition aftcache.PeriodicHook, wantPrefixes map[string]bool) (*aftcache.AFTData, error) {
	t.Helper()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		aftSession1.ListenUntil(t.Context(), t, aftConvergenceTime, stoppingCondition)
		//streamErr1 = aftSession1.ListenUntilWithError(sharedCtx, t, aftConvergenceTime, stoppingCondition)
	}()
	go func() {
		defer wg.Done()
		aftSession2.ListenUntil(t.Context(), t, aftConvergenceTime, stoppingCondition)
		//streamErr2 = aftSession2.ListenUntilWithError(sharedCtx, t, aftConvergenceTime, stoppingCondition)
	}()
	wg.Wait()

	// Get the AFT from the cache.
	aft1, err := aftSession1.ToAFT(t, tc.dut)
	if err != nil {
		return nil, fmt.Errorf("error getting AFT from session 1: %v", err)
	}
	aft2, err := aftSession2.ToAFT(t, tc.dut)
	if err != nil {
		return nil, fmt.Errorf("error getting AFT from session 2: %v", err)
	}

	// Extract only wantPrefixes from both sessions for comparison
	filteredAFT1 := tc.filterAFTByPrefixes(aft1, wantPrefixes)
	filteredAFT2 := tc.filterAFTByPrefixes(aft2, wantPrefixes)

	sortSlices := cmpopts.SortSlices(func(a, b uint64) bool { return a < b })
	if diff := cmp.Diff(filteredAFT1, filteredAFT2, sortSlices); diff != "" {
		return nil, fmt.Errorf("afts from two sessions are not consistent for wantPrefixes: %s", diff)
	}
	return aft1, nil
}

// filterAFTByPrefixes extracts only the specified prefixes and their associated NHGs/NHs from AFT data.
// Since aftNextHopGroup and aftNextHop are unexported, we create a new AFTData by copying
// only the wanted prefixes and their dependencies from the full AFT.
func (tc *testCase) filterAFTByPrefixes(aft *aftcache.AFTData, wantPrefixes map[string]bool) *aftcache.AFTData {
	return aft.FilterByPrefixes(wantPrefixes)
}

func (tc *testCase) otgInterfaceState(t *testing.T, portName string, state gosnappi.StatePortLinkStateEnum) {
	portStateAction := gosnappi.NewControlState()
	portStateAction.Port().Link().SetPortNames([]string{portName}).SetState(state)
	tc.ate.OTG().SetControlState(t, portStateAction)
}

type testCase struct {
	name        string
	dut         *ondatra.DUTDevice
	ate         *ondatra.ATEDevice
	gnmiClient1 gnmipb.GNMIClient
	gnmiClient2 gnmipb.GNMIClient
}

func TestBGP(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	gnmiClient1, err := dut.RawAPIs().BindingDUT().DialGNMI(t.Context())
	if err != nil {
		t.Fatalf("Failed to dial GNMI: %v", err)
	}
	gnmiClient2, err := dut.RawAPIs().BindingDUT().DialGNMI(t.Context())
	if err != nil {
		t.Fatalf("Failed to dial GNMI: %v", err)
	}
	tc := &testCase{
		name:        "AFT Churn Test With Scale",
		dut:         dut,
		ate:         ate,
		gnmiClient1: gnmiClient1,
		gnmiClient2: gnmiClient2,
	}

	// TODO: - Add  deviation if any HW profile change is required
	if tc.dut.Vendor() == ondatra.CISCO {
		t.Log("Configuring DUT HW profile for Cisco and rebooting DUT")
		if err := tc.configureHwProfile(t); err != nil {
			t.Fatalf("failed to configure DUT HW profile: %v", err)
		}
	}

	// Defer cleanup to ensure it runs at the end of the test
	defer func() {
		if tc.dut.Vendor() == ondatra.CISCO {
			t.Log("Restoring DUT HW profile for Cisco and rebooting DUT")
			if err := tc.configureDefaultHwProfile(t); err != nil {
				t.Fatalf("failed to restore DUT HW profile: %v", err)
			}
		}
	}()

	// Pre-generate all expected prefixes once for efficiency
	wantPrefixes := tc.generateWantPrefixes(t)

	// Create an AFTStreamSession per gnmiClient
	aftSession1 := aftcache.NewAFTStreamSession(t.Context(), t, tc.gnmiClient1, tc.dut)
	aftSession2 := aftcache.NewAFTStreamSession(t.Context(), t, tc.gnmiClient2, tc.dut)

	// Helper function for verifying AFT state when given prefixes and expected next hops.
	verifyAFTState := func(desc string, wantNHCount int, wantV4NHs, wantV6NHs map[string]bool) *aftcache.AFTData {
		t.Helper()
		t.Log(desc)
		stoppingCondition := aftcache.InitialSyncStoppingCondition(t, dut, wantPrefixes, wantV4NHs, wantV6NHs)
		aft, err := tc.fetchAFT(t, aftSession1, aftSession2, stoppingCondition, wantPrefixes)
		if err != nil {
			t.Fatalf("failed to get AFT Cache: %v", err)
		}
		if err := tc.verifyPrefixes(t, aft, startingBGPRouteIPv4, int(getRouteCount(dut, IPv4)), wantNHCount, true); err != nil {
			t.Errorf("failed to verify IPv4 BGP prefixes: %v", err)
		}
		if err := tc.verifyPrefixes(t, aft, startingBGPRouteIPv6128, int(bgpRouteCountIPv6Default128), wantNHCount, true); err != nil {
			t.Errorf("failed to verify IPv6 BGP prefixes: %v", err)
		}
		if err := tc.verifyPrefixes(t, aft, startingBGPRouteIPv664, int(bgpRouteCountIPv6Default64), wantNHCount, true); err != nil {
			t.Errorf("failed to verify IPv6 BGP prefixes: %v", err)
		}
		return aft
	}

	// --- Test Setup ---
	if err := tc.configureDUT(t); err != nil {
		t.Fatalf("failed to configure DUT: %v", err)
	}
	tc.configureATE(t)

	t.Log("Waiting for BGP neighbor to establish...")
	if err := tc.waitForBGPSessions(t, []string{ateP1.IPv4, ateP2.IPv4}, []string{ateP1.IPv6, ateP2.IPv6}); err != nil {
		t.Fatalf("Unable to establish BGP session: %v", err)
	}

	// Step 1: Initial state verification (BGP: 2 NHs, ISIS: 1 NH)
	aft := verifyAFTState("Initial AFT verification", 2, wantIPv4NHs, wantIPv6NHs)

	// Verify ISIS prefixes are present in AFT.
	if err := tc.verifyPrefixes(t, aft, startingISISRouteIPv4, isisRouteCount, 1, false); err != nil {
		t.Errorf("failed to verify IPv4 ISIS prefixes: %v", err)
	}
	if err := tc.verifyPrefixes(t, aft, startingISISRouteIPv6, isisRouteCount, 1, false); err != nil {
		t.Errorf("failed to verify IPv6 ISIS prefixes: %v", err)
	}
	t.Log("ISIS verification completed")

	// Step 2: Stop Port2 interface to create Churn (BGP: 1 NH)
	t.Log("SubTest 2: Stopping Port2 interface to create Churn")
	tc.otgInterfaceState(t, port2Name, gosnappi.StatePortLinkState.DOWN)
	if err := tc.waitForBGPSessions(t, []string{ateP1.IPv4}, []string{ateP1.IPv6}); err != nil {
		t.Fatalf("Unable to establish BGP session: %v", err)
	}
	verifyAFTState("AFT verification after port 2 churn", 1, wantIPv4NHsPostChurn, getPostChurnIPv6NH(tc.dut))

	// Step 3: Stop Port1 interface to create full Churn (BGP: deletion expected)
	t.Log("SubTest 3: Stopping Port1 interface to remove Churn")
	tc.otgInterfaceState(t, port1Name, gosnappi.StatePortLinkState.DOWN)
	sc := aftcache.DeletionStoppingCondition(t, dut, wantPrefixes)
	// Expecting all prefixes deleted, so pass empty map for wantPrefixes validation
	if _, err := tc.fetchAFT(t, aftSession1, aftSession2, sc, map[string]bool{}); err != nil {
		t.Fatalf("failed to get AFT Cache after deletion: %v", err)
	}

	// Step 4: Start Port1 interface to remove Churn (BGP: 1 NH - Port2 still down)
	t.Log("SubTest 4: Starting Port1 interface to remove Churn")
	tc.otgInterfaceState(t, port1Name, gosnappi.StatePortLinkState.UP)
	if err := tc.waitForBGPSessions(t, []string{ateP1.IPv4}, []string{ateP1.IPv6}); err != nil {
		t.Fatalf("Unable to establish BGP session: %v", err)
	}
	verifyAFTState("AFT verification after port 1 up", 1, wantIPv4NHsPostChurn, getPostChurnIPv6NH(tc.dut))

	// Step 5: Start Port2 interface to remove Churn (BGP: 2 NHs - full recovery)
	t.Log("SubTest 5: Starting Port2 interface and recheck Churn")
	tc.otgInterfaceState(t, port2Name, gosnappi.StatePortLinkState.UP)
	t.Log("Waiting for BGP neighbor to establish...")
	if err := tc.waitForBGPSessions(t, []string{ateP1.IPv4, ateP2.IPv4}, []string{ateP1.IPv6, ateP2.IPv6}); err != nil {
		t.Fatalf("Unable to establish BGP session: %v", err)
	}
	verifyAFTState("AFT verification after port 2 up", 2, wantIPv4NHs, wantIPv6NHs)
}

// configureHwProfile configures all the interfaces and BGP on the DUT.
func (tc *testCase) configureHwProfile(t *testing.T) error {
	ciscoConfig := `
		hw-module profile route scale lpm tcam-banks
		customshowtech GRPC_CUSTOM
		command show health gsp
		command show health sysdb
		command show tech-support gsp
		command show tech-support cfgmgr
		command show tech-support ofa
		command show tech-support pfi
		command show tech-support spi
		command show tech-support mgbl
		command show tech-support sysdb
		command show tech-support appmgr
		command show tech-support fabric
		command show tech-support yserver
		command show tech-support interface
		command show tech-support platform-fwd
		command show tech-support linux networking
		command show tech-support ethernet interfaces
		command show tech-support fabric link-include
		command show tech-support p2p-ipc process appmgr
		command show tech-support insight include-database
		command show tech-support lpts
		command show tech-support parser
		command show tech-support telemetry model-driven
		lpts pifib hardware police flow tpa rate 10000 
		`
	helpers.GnmiCLIConfig(t, tc.dut, ciscoConfig)
	tc.rebootDUT(t)
	return nil
}

// configureDefaultHwProfile configures all the interfaces and BGP on the DUT.
func (tc *testCase) configureDefaultHwProfile(t *testing.T) error {
	ciscoConfig := `
	    no hw-module profile route scale lpm tcam-banks
		`
	helpers.GnmiCLIConfig(t, tc.dut, ciscoConfig)
	tc.rebootDUT(t)
	return nil
}

func (tc *testCase) rebootDUT(t *testing.T) {
	t.Helper()
	rebootRequest := &spb.RebootRequest{
		Method:  spb.RebootMethod_COLD,
		Delay:   0,
		Message: "Reboot chassis without delay",
		Force:   true,
	}
	gnoiClient, err := tc.dut.RawAPIs().BindingDUT().DialGNOI(t.Context())
	if err != nil {
		t.Fatalf("Error dialing gNOI: %v", err)
	}
	bootTimeBeforeReboot := gnmi.Get(t, tc.dut, gnmi.OC().System().BootTime().State())
	t.Logf("DUT boot time before reboot: %v %v", bootTimeBeforeReboot, time.Now())
	t.Log("Sending reboot request to DUT")

	ctxWithTimeout, cancel := context.WithTimeout(t.Context(), 8*time.Minute)
	defer cancel()
	_, err = gnoiClient.System().Reboot(ctxWithTimeout, rebootRequest)
	defer gnoiClient.System().CancelReboot(t.Context(), &spb.CancelRebootRequest{})
	if err != nil {
		t.Fatalf("Failed to reboot chassis with unexpected err: %v", err)
	}

	// Wait for the device to become reachable again.
	dut := ondatra.DUT(t, "dut")
	deviceBootStatus(t, dut)
	t.Logf("Device is reachable, waiting for boot time to update.")

	bootTimeAfterReboot := gnmi.Get(t, tc.dut, gnmi.OC().System().BootTime().State())
	t.Logf("DUT boot time after reboot: %v", bootTimeAfterReboot)
}

func deviceBootStatus(t *testing.T, dut *ondatra.DUTDevice) {
	startReboot := time.Now()
	t.Logf("Wait for DUT to boot up by polling the telemetry output.")
	for {
		var currentTime string
		t.Logf("Time elapsed %.2f minutes since reboot started.", time.Since(startReboot).Minutes())

		time.Sleep(3 * time.Minute)
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			currentTime = gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State())
		}); errMsg != nil {
			t.Logf("Got testt.CaptureFatal errMsg: %s, keep polling ...", *errMsg)
		} else {
			t.Logf("Device rebooted successfully with received time: %v", currentTime)
			break
		}

		if uint64(time.Since(startReboot).Minutes()) > maxRebootTime {
			t.Fatalf("Check boot time: got %v, want < %v", time.Since(startReboot), maxRebootTime)
		}
	}
	t.Logf("Device boot time: %.2f minutes", time.Since(startReboot).Minutes())
}
