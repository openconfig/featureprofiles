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

package collector_flap_test

import (
	"context"
	"fmt"
	"io"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/components"
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
	aftConvergenceTime        = 40 * time.Minute
	bgpTimeout                = 10 * time.Minute
	bgpRouteCountIPv4LowScale = 1250000
	bgpRouteCountIPv6LowScale = 500000
	bgpRouteCountIPv4Default  = 2000000
	bgpRouteCountIPv6Default  = 1000000
	policyStatementID         = "id-1"
	cpuPctThreshold           = 80.0
	memPctThreshold           = 80.0
	usagePollingInterval      = 5 * time.Second
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
	wantIPv4NHs         = map[string]bool{ateP1.IPv4: true, ateP2.IPv4: true}
	wantIPv6NHs         = map[string]bool{ateP1.IPv6: true, ateP2.IPv6: true}
	port1Name           = "port1"
	port2Name           = "port2"
	ciscoProcsToMonitor = []string{"emsd", "rib_mgr", "bgp_epe", "fib_mgr"}
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

// configureDUTInterfaces configures DUT interfaces.
func (tc *testCase) configureDUTInterfaces(t *testing.T) {
	dut := tc.dut
	p1 := dut.Port(t, port1Name).Name()
	i1 := dutP1.NewOCInterface(p1, dut)
	gnmi.Update(t, dut, gnmi.OC().Interface(p1).Config(), i1)

	p2 := dut.Port(t, port2Name).Name()
	i2 := dutP2.NewOCInterface(p2, dut)
	gnmi.Update(t, dut, gnmi.OC().Interface(p2).Config(), i2)

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, dut.Port(t, port1Name))
		fptest.SetPortSpeed(t, dut.Port(t, port2Name))
	}
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, p1, deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, p2, deviations.DefaultNetworkInstance(dut), 0)
	}
}

// configureRoutingPolicy configures a routing policy to allow all routes.
func (tc *testCase) configureRoutingPolicy(t *testing.T) error {
	dut := tc.dut
	d := &oc.Root{}
	routePolicy := d.GetOrCreateRoutingPolicy()
	policyDefinition := routePolicy.GetOrCreatePolicyDefinition(applyPolicyName)
	statement, err := policyDefinition.AppendNewStatement(policyStatementID)
	if err != nil {
		return fmt.Errorf("failed to append new statement to policy definition %s: %v", applyPolicyName, err)
	}
	statement.GetOrCreateActions().SetPolicyResult(applyPolicyType)
	gnmi.Update(t, dut, gnmi.OC().RoutingPolicy().Config(), routePolicy)
	return nil
}

// configureBGP configures BGP neighbors on the DUT.
func (tc *testCase) configureBGP(t *testing.T) {
	dut := tc.dut
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
}

// configureISIS configures ISIS on the DUT.
func (tc *testCase) configureISIS(t *testing.T) error {
	ts := isissession.MustNew(t).WithISIS()
	ts.ConfigISIS(func(isis *oc.NetworkInstance_Protocol_Isis) {
		global := isis.GetOrCreateGlobal()
		global.SetHelloPadding(oc.Isis_HelloPaddingType_DISABLE)

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

	if _, err := ts.AwaitAdjacency(); err != nil {
		return fmt.Errorf("No IS-IS adjacency formed: %v", err)
	}
	return nil
}

// configureDUT configures all the interfaces and BGP on the DUT.
func (tc *testCase) configureDUT(t *testing.T) error {
	dut := tc.dut
	// Configure Network instance type on DUT.
	t.Log("Configure/update Network Instance")
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	tc.configureDUTInterfaces(t)

	if err := tc.configureRoutingPolicy(t); err != nil {
		return err
	}
	tc.configureBGP(t)
	if err := tc.configureISIS(t); err != nil {
		return err
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

func (tc *testCase) verifyPrefixes(t *testing.T, aft *aftcache.AFTData, ip string, routeCount int, wantNHCount int) error {
	for pfix := range netutil.GenCIDRs(t, ip, routeCount) {
		nhgID, ok := aft.Prefixes[pfix]
		if !ok {
			return fmt.Errorf("Prefix %s not found in AFT", pfix)
		}
		nhg, ok := aft.NextHopGroups[nhgID]
		if !ok {
			return fmt.Errorf("Next hop group %d not found in AFT for prefix %s", nhgID, pfix)
		}

		if len(nhg.NHIDs) != wantNHCount {
			return fmt.Errorf("Prefix %s has %d next hops, want %d", pfix, len(nhg.NHIDs), wantNHCount)
		}

		var firstWeight uint64 = 0 // Initialize with a value that won't be a valid weight
		for i := 0; i < wantNHCount; i++ {
			nhID := nhg.NHIDs[i]
			nh, ok := aft.NextHops[nhID]
			if !ok {
				return fmt.Errorf("Next hop %d not found in AFT for next-hop group: %d for prefix: %s", nhID, nhgID, pfix)
			}
			if !deviations.SkipInterfaceNameCheck(tc.dut) && nh.IntfName == "" {
				return fmt.Errorf("Next hop interface not found in AFT for next-hop: %d for prefix: %s", nhID, pfix)
			}
			if nh.IP == "" {
				return fmt.Errorf("Next hop IP not found in AFT for next-hop: %d for prefix: %s", nhID, pfix)
			}
			weight, ok := nhg.NHWeights[nhID]
			if !ok {
				return fmt.Errorf("Next hop weight not found in AFT for next-hop: %d for prefix: %s", nhID, pfix)
			}
			if weight <= 0 {
				return fmt.Errorf("Next hop weight is %d, want > 0 for next-hop: %d for prefix: %s", weight, nhID, pfix)
			}
			// Check if weights are equal
			if firstWeight == 0 { // This is the first next hop, set the reference weight
				firstWeight = weight
			} else if weight != firstWeight { // Compare with the first encountered weight
				return fmt.Errorf("Next hop group %d has unequal weights. Expected %d, got %d for next-hop %d for prefix %s", nhgID, firstWeight, weight, nhID, pfix)
			}
		}
	}
	return nil
}

// verifyAFTConsistency fetches the AFT from two sessions, compares them, and returns the AFT data from the first session if they are consistent.
func (tc *testCase) verifyAFTConsistency(t *testing.T, aftSession1, aftSession2 *aftcache.AFTStreamSession, desc string) *aftcache.AFTData {
	t.Helper()
	aft1, err := aftSession1.ToAFT(t, tc.dut)
	if err != nil {
		t.Fatalf("Error getting AFT from session 1 for %s verification: %v", desc, err)
	}
	aft2, err := aftSession2.ToAFT(t, tc.dut)
	if err != nil {
		t.Fatalf("Error getting AFT from session 2 for %s verification: %v", desc, err)
	}
	sortSlices := cmpopts.SortSlices(func(a, b uint64) bool { return a < b })
	if diff := cmp.Diff(aft1, aft2, sortSlices); diff != "" {
		t.Fatalf("AFTs from two sessions are not consistent for %s verification: %s", desc, diff)
	}
	return aft1
}

func (tc *testCase) verifyInitialAFT(t *testing.T, ctx1 context.Context, aftSession1 *aftcache.AFTStreamSession, ctx2 context.Context, aftSession2 *aftcache.AFTStreamSession, wantPrefixes map[string]bool) {
	t.Helper()
	t.Log("Initial AFT verification...")
	initialStoppingCondition := aftcache.InitialSyncStoppingCondition(t, tc.dut, wantPrefixes, wantIPv4NHs, wantIPv6NHs)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		aftSession1.ListenUntil(ctx1, t, aftConvergenceTime, initialStoppingCondition)
	}()
	go func() {
		defer wg.Done()
		aftSession2.ListenUntil(ctx2, t, aftConvergenceTime, initialStoppingCondition)
	}()
	wg.Wait()

	aft := tc.verifyAFTConsistency(t, aftSession1, aftSession2, "Initial AFT verification")
	t.Log("AFT is consistent. Verifying prefixes...")
	if err := tc.verifyPrefixes(t, aft, startingBGPRouteIPv4, int(routeCount(tc.dut, IPv4)), 2); err != nil {
		t.Errorf("Failed to verify initial IPv4 BGP prefixes: %v", err)
	}
	if err := tc.verifyPrefixes(t, aft, startingBGPRouteIPv6, int(routeCount(tc.dut, IPv6)), 2); err != nil {
		t.Errorf("Failed to verify initial IPv6 BGP prefixes: %v", err)
	}

	// Verify ISIS prefixes are present in AFT.
	if err := tc.verifyPrefixes(t, aft, startingISISRouteIPv4, isisRouteCount, 1); err != nil {
		t.Errorf("Failed to verify IPv4 ISIS prefixes: %v", err)
	}
	if err := tc.verifyPrefixes(t, aft, startingISISRouteIPv6, isisRouteCount, 1); err != nil {
		t.Errorf("Failed to verify IPv6 ISIS prefixes: %v", err)
	}
	t.Log("ISIS verification completed.")
}

type usageRecord struct {
	time    time.Time
	process string
	metric  string
	before  float64
	after   float64
	usedPct float64
	desc    string
}

type usageHistory struct {
	mu      sync.Mutex
	records []usageRecord
}

func (h *usageHistory) add(rec usageRecord) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, rec)
}

func (h *usageHistory) print(t *testing.T) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.records) == 0 {
		return
	}

	sort.Slice(h.records, func(i, j int) bool {
		if h.records[i].metric != h.records[j].metric {
			return h.records[i].metric < h.records[j].metric
		}
		return h.records[i].time.Before(h.records[j].time)
	})

	t.Log("Usage Summary Table:")
	t.Logf("%-25s | %-15s | %-10s | %-10s | %-10s | %-10s | %s", "Time", "Process", "Metric", "Before", "After", "Used(%)", "Description")
	t.Log("-------------------------------------------------------------------------------------------------------------------------------------------------")
	for _, r := range h.records {
		t.Logf("%-25s | %-15s | %-10s | %-10.2f | %-10.2f | %-10.2f | %s",
			r.time.Format("15:04:05.000"), r.process, r.metric, r.before, r.after, r.usedPct, r.desc)
	}
}

// checkMemoryUsage checks for memory increase and logs an error if it's above the threshold.
// It returns the current memory usage.
func checkMemoryUsage(t *testing.T, dut *ondatra.DUTDevice, history *usageHistory, memBefore uint64, desc string, procName ...string) uint64 {
	t.Helper()
	var memAfter uint64
	var totalMem uint64
	var usedMemPct float64
	// TODO: Add memory usage check for Default case.
	pName := "system"
	if len(procName) > 0 && procName[0] != "" {
		pName = procName[0]
	}

	switch dut.Vendor() {
	case ondatra.ARISTA:
		t.Logf("Checking memory usage %s.", desc)
		memAfter = gnmi.Get(t, dut, gnmi.OC().System().Memory().Used().State())
		totalMem = gnmi.Get(t, dut, gnmi.OC().System().Memory().Physical().State())
		usedMemPct = (float64(memAfter) / float64(totalMem)) * 100
	case ondatra.CISCO:
		if pName == "system" {
			t.Fatal("Process name must be provided for Cisco memory check.")
		}
		pid := getProcessPid(t, dut, pName)
		t.Logf("Checking memory usage for %s process (PID: %d) %s.", pName, pid, desc)
		memAfter = uint64(gnmi.Get(t, dut, gnmi.OC().System().Process(pid).MemoryUtilization().State()))
		usedMemPct = float64(memAfter) // Cisco returns percentage directly
	default:
		t.Logf("Skipping memory usage check for non-ARISTA and non-CISCO device: %v.", dut.Vendor())
		return memBefore // Return previous value to not break chaining.
	}

	if usedMemPct > memPctThreshold {
		t.Errorf("Memory usage for process %s is %.2f%% of total memory, which is more than the %.f%% threshold.", pName, usedMemPct, memPctThreshold)
	} else {
		t.Logf("Memory usage for process %s is %.2f%% of total memory, which is within the %.f%% threshold.", pName, usedMemPct, memPctThreshold)
	}

	if history != nil {
		history.add(usageRecord{
			time:    time.Now(),
			process: pName,
			metric:  "Memory",
			before:  float64(memBefore),
			after:   float64(memAfter),
			usedPct: usedMemPct,
			desc:    desc,
		})
	}
	return memAfter
}

// getProcessPid returns the PID of the given process name.
func getProcessPid(t *testing.T, dut *ondatra.DUTDevice, procName string) uint64 {
	t.Helper()
	procs := gnmi.GetAll(t, dut, gnmi.OC().System().ProcessAny().State())
	for _, p := range procs {
		if p.GetName() == procName {
			pid := p.GetPid()
			t.Logf("Found %q process with PID: %d", procName, pid)
			return pid
		}
	}
	t.Fatalf("No process named %q found", procName)
	return 0
}

// getCPUComponents returns the names of CPU components.
func getCPUComponents(t *testing.T, dut *ondatra.DUTDevice) []string {
	t.Helper()
	cpus := components.FindComponentsByType(t, dut, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CPU)
	if len(cpus) == 0 {
		t.Fatalf("No CPU component found")
	}
	sort.Strings(cpus)
	if len(cpus) > 1 {
		t.Logf("Found %d CPU components: %v.", len(cpus), cpus)
	}
	return cpus
}

// checkCPUUsage checks for CPU increase and logs an error if it's above the threshold.
// It returns the current CPU usage.
func checkCPUUsage(t *testing.T, dut *ondatra.DUTDevice, history *usageHistory, cpuBefore uint64, desc string, cpuComponent string, procName ...string) uint64 {
	t.Helper()
	var cpuAfter uint64
	var usedCPUPct float64
	var pName string // Identifier for logging: component name for Arista, process name for Cisco.

	switch dut.Vendor() {
	case ondatra.ARISTA:
		if cpuComponent == "" {
			t.Fatal("cpuComponent must be provided for Arista CPU check.")
		}
		pName = cpuComponent
		t.Logf("Checking CPU usage for component %s %s.", cpuComponent, desc)
		cpuAfter = uint64(gnmi.Get(t, dut, gnmi.OC().Component(cpuComponent).Cpu().Utilization().Avg().State()))
	case ondatra.CISCO:
		if len(procName) == 0 || procName[0] == "" {
			t.Fatal("Process name must be provided for Cisco CPU check.")
		}
		pName = procName[0]
		pid := getProcessPid(t, dut, pName)
		t.Logf("Checking CPU usage for %s process (PID: %d) %s.", pName, pid, desc)
		cpuAfter = uint64(gnmi.Get(t, dut, gnmi.OC().System().Process(pid).CpuUtilization().State()))
	default:
		t.Logf("Skipping CPU usage check for non-ARISTA and non-CISCO device: %v.", dut.Vendor())
		return cpuBefore // Return previous value to not break chaining.
	}
	usedCPUPct = float64(cpuAfter)
	if usedCPUPct > cpuPctThreshold {
		t.Errorf("CPU usage for process %s increased by %.2f%%, which is more than the %.f%% threshold.", pName, usedCPUPct, cpuPctThreshold)
	} else {
		t.Logf("CPU usage change for process %s is %.2f%%, which is within the %.f%% threshold.", pName, usedCPUPct, cpuPctThreshold)
	}
	if history != nil {
		history.add(usageRecord{
			time:    time.Now(),
			process: pName,
			metric:  "CPU",
			before:  float64(cpuBefore),
			after:   float64(cpuAfter),
			usedPct: float64(usedCPUPct),
			desc:    desc,
		})
	}
	return cpuAfter
}

type usageBaselines struct {
	aristaMem     uint64
	aristaCPU     map[string]uint64
	ciscoMem      map[string]uint64
	ciscoCPU      map[string]uint64
	cpuComponents []string
}

func collectBaselines(t *testing.T, dut *ondatra.DUTDevice) *usageBaselines {
	t.Helper()
	b := &usageBaselines{}
	if dut.Vendor() == ondatra.ARISTA {
		b.aristaMem = gnmi.Get(t, dut, gnmi.OC().System().Memory().Used().State())
		b.cpuComponents = getCPUComponents(t, dut)
		b.aristaCPU = make(map[string]uint64)
		for _, comp := range b.cpuComponents {
			b.aristaCPU[comp] = uint64(gnmi.Get(t, dut, gnmi.OC().Component(comp).Cpu().Utilization().Avg().State()))
		}
	} else if dut.Vendor() == ondatra.CISCO {
		b.ciscoMem = make(map[string]uint64)
		b.ciscoCPU = make(map[string]uint64)
		for _, procName := range ciscoProcsToMonitor {
			pid := getProcessPid(t, dut, procName)
			b.ciscoMem[procName] = uint64(gnmi.Get(t, dut, gnmi.OC().System().Process(pid).MemoryUtilization().State()))
			b.ciscoCPU[procName] = uint64(gnmi.Get(t, dut, gnmi.OC().System().Process(pid).CpuUtilization().State()))
		}
	}
	return b
}

func aristaMonitorAndConverge(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, aftSession *aftcache.AFTStreamSession, stoppingCondition aftcache.PeriodicHook, history *usageHistory, memBaseline *uint64, cpuBaselines map[string]uint64, cpuComponents []string) {
	t.Helper()
	var wg sync.WaitGroup
	wg.Add(1)
	memDone := make(chan struct{})
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(usagePollingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				checkMemoryUsage(t, dut, history, *memBaseline, "during collector 2 convergence")
				for _, comp := range cpuComponents {
					checkCPUUsage(t, dut, history, cpuBaselines[comp], "during collector 2 convergence", comp)
				}
			case <-memDone:
				return
			}
		}
	}()
	aftSession.ListenUntil(ctx, t, aftConvergenceTime, stoppingCondition)
	close(memDone)
	wg.Wait()
	*memBaseline = checkMemoryUsage(t, dut, history, *memBaseline, "after collector 2 convergence")
	for _, comp := range cpuComponents {
		cpuBaselines[comp] = checkCPUUsage(t, dut, history, cpuBaselines[comp], "after collector 2 convergence", comp)
	}
}

func ciscoMonitorAndConverge(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, aftSession *aftcache.AFTStreamSession, stoppingCondition aftcache.PeriodicHook, history *usageHistory, memBaseline map[string]uint64, cpuBaseline map[string]uint64) {
	t.Helper()
	var wg sync.WaitGroup
	wg.Add(1)
	memDone := make(chan struct{})
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(usagePollingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				for procName, memBase := range memBaseline {
					checkMemoryUsage(t, dut, history, memBase, "during collector 2 convergence", procName)
				}
				for procName, cpuBase := range cpuBaseline {
					checkCPUUsage(t, dut, history, cpuBase, "during collector 2 convergence", "", procName)
				}
			case <-memDone:
				return
			}
		}
	}()
	aftSession.ListenUntil(ctx, t, aftConvergenceTime, stoppingCondition)
	close(memDone)
	wg.Wait()
	for procName, memBase := range memBaseline {
		memBaseline[procName] = checkMemoryUsage(t, dut, history, memBase, "after collector 2 convergence", procName)
	}
	for procName, cpuBase := range cpuBaseline {
		cpuBaseline[procName] = checkCPUUsage(t, dut, history, cpuBase, "after collector 2 convergence", "", procName)
	}
}

func (tc *testCase) restartCollector2(t *testing.T, cancelSession2 context.CancelFunc) (context.Context, context.CancelFunc, *aftcache.AFTStreamSession, *usageBaselines, time.Time) {
	t.Helper()
	// TODO: Add memory usage check for Juniper and Nokia device.
	b := collectBaselines(t, tc.dut)
	t.Log("Starting collector restart test, stopping collector 2 by canceling its context...")
	cancelSession2() // This will cause the ListenUntil to stop.
	// Explicitly close the underlying gNMI client connection.
	if closer, ok := tc.gnmiClient2.(io.Closer); ok {
		if err := closer.Close(); err != nil {
			t.Logf("Failed to close gnmiClient2, proceeding anyway: %v", err)
		}
		tc.gnmiClient2 = nil
	} else {
		t.Logf("Warning: gnmiClient2 does not implement io.Closer.")
	}

	t.Log("Recreating collector 2...")
	startTime := time.Now()
	gnmiClient2, err := tc.dut.RawAPIs().BindingDUT().DialGNMI(t.Context())
	if err != nil {
		t.Fatalf("Failed to dial GNMI for collector 2: %v", err)
	}
	tc.gnmiClient2 = gnmiClient2 // Update the testCase struct.
	sessionCtx2, newCancelSession2 := context.WithCancel(t.Context())
	aftSession2 := aftcache.NewAFTStreamSession(sessionCtx2, t, tc.gnmiClient2, tc.dut)
	return sessionCtx2, newCancelSession2, aftSession2, b, startTime
}

func (tc *testCase) verifyAFTConvergenceAfterFlap(t *testing.T, sessionCtx2 context.Context, aftSession1 *aftcache.AFTStreamSession, aftSession2 *aftcache.AFTStreamSession, b *usageBaselines, wantPrefixes map[string]bool, history *usageHistory) {
	t.Helper()
	t.Log("Waiting for restarted collector 2 to converge...")
	stoppingCondition := aftcache.InitialSyncStoppingCondition(t, tc.dut, wantPrefixes, wantIPv4NHs, wantIPv6NHs)
	switch tc.dut.Vendor() {
	case ondatra.ARISTA:
		aristaMonitorAndConverge(sessionCtx2, t, tc.dut, aftSession2, stoppingCondition, history, &b.aristaMem, b.aristaCPU, b.cpuComponents)
	case ondatra.CISCO:
		ciscoMonitorAndConverge(sessionCtx2, t, tc.dut, aftSession2, stoppingCondition, history, b.ciscoMem, b.ciscoCPU)
	default:
		aftSession2.ListenUntil(sessionCtx2, t, aftConvergenceTime, stoppingCondition)
	}
	t.Log("Verifying convergence for restarted collector 2...")

	t.Log("Verifying AFT consistency after restarting collector 2...")
	aftn := tc.verifyAFTConsistency(t, aftSession1, aftSession2, "post-flap")

	t.Log("AFT is consistent. Verifying prefixes...")
	if err := tc.verifyPrefixes(t, aftn, startingBGPRouteIPv4, int(routeCount(tc.dut, IPv4)), 2); err != nil {
		t.Errorf("Failed to verify IPv4 BGP prefixes after collector 2 flap: %v", err)
	}
	if err := tc.verifyPrefixes(t, aftn, startingBGPRouteIPv6, int(routeCount(tc.dut, IPv6)), 2); err != nil {
		t.Errorf("Failed to verify IPv6 BGP prefixes after collector 2 flap: %v", err)
	}
}

type testCase struct {
	name        string
	dut         *ondatra.DUTDevice
	ate         *ondatra.ATEDevice
	gnmiClient1 gnmipb.GNMIClient
	gnmiClient2 gnmipb.GNMIClient
}

func TestCollectorFlap(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	var history usageHistory

	gnmiClient1, err := dut.RawAPIs().BindingDUT().DialGNMI(t.Context())
	if err != nil {
		t.Fatalf("Failed to dial GNMI: %v", err)
	}
	gnmiClient2, err := dut.RawAPIs().BindingDUT().DialGNMI(t.Context())
	if err != nil {
		t.Fatalf("Failed to dial GNMI: %v", err)
	}
	tc := &testCase{
		name:        "collectorRestartTest",
		dut:         dut,
		ate:         ate,
		gnmiClient1: gnmiClient1,
		gnmiClient2: gnmiClient2,
	}

	// The gNMI client is closed and re-created in this test. The deferred
	// function will close the second client. The first is closed manually
	// before it is re-created.
	defer func() {
		if tc.gnmiClient2 != nil {
			if closer, ok := tc.gnmiClient2.(io.Closer); ok {
				if err := closer.Close(); err != nil {
					t.Logf("Error closing gNMI client 2: %v .", err)
				}
			}
		}
	}()

	// Pre-generate all expected prefixes once for efficiency
	wantPrefixes := tc.generateWantPrefixes(t)

	// Create an AFTStreamSession per gnmiClient
	aftSession1 := aftcache.NewAFTStreamSession(t.Context(), t, tc.gnmiClient1, tc.dut)
	sessionCtx2, cancelSession2 := context.WithCancel(t.Context())
	aftSession2 := aftcache.NewAFTStreamSession(sessionCtx2, t, tc.gnmiClient2, tc.dut)

	if err := tc.configureDUT(t); err != nil {
		t.Fatalf("Failed to configure DUT: %v", err)
	}
	tc.configureATE(t)

	t.Log("Waiting for BGP neighbor to establish...")
	if err := tc.waitForBGPSessions(t, []string{ateP1.IPv4, ateP2.IPv4}, []string{ateP1.IPv6, ateP2.IPv6}); err != nil {
		t.Fatalf("Unable to establish BGP session: %v", err)
	}

	tc.verifyInitialAFT(t, t.Context(), aftSession1, sessionCtx2, aftSession2, wantPrefixes)

	sessionCtx2, cancelSession2, aftSession2, b, startTime := tc.restartCollector2(t, cancelSession2)
	defer cancelSession2()

	tc.verifyAFTConvergenceAfterFlap(t, sessionCtx2, aftSession1, aftSession2, b, wantPrefixes, &history)
	convergenceTime := time.Since(startTime)
	t.Logf("Collector 2 convergence time: %v", convergenceTime)
	history.print(t)
}
