// Copyright 2023 Google LLC
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

package bgp_comm_match_action_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	otgtelemetry "github.com/openconfig/ondatra/gnmi/otg"
	otg "github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	trafficDuration = 1 * time.Minute
	tolerancePct    = 2
	peerGrpName     = "BGP-PEER-GROUP"
	dutAS           = 65501
	ateAS           = 65502
	plenIPv4        = 30
	plenIPv6        = 126
	acceptPolicy    = "PERMIT-ALL"
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "DUT to ATE Port1",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::192:0:2:1",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	atePort1 = attrs.Attributes{
		Name:    "atePort1",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::192:0:2:2",
		MAC:     "02:00:01:01:01:01",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	dutPort2 = attrs.Attributes{
		Desc:    "DUT to ATE Port2",
		IPv4:    "192.0.2.5",
		IPv6:    "2001:db8::192:0:2:5",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	atePort2 = attrs.Attributes{
		Name:    "atePort2",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::192:0:2:6",
		MAC:     "02:00:02:01:01:01",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}

	ebgp1NbrV4 = &bgpNeighbor{
		nbrAddr: atePort1.IPv4,
		isV4:    true,
		afiSafi: oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST,
		as:      ateAS}
	ebgp1NbrV6 = &bgpNeighbor{
		nbrAddr: atePort1.IPv6,
		isV4:    false,
		afiSafi: oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST,
		as:      ateAS}
	ebgp2NbrV4 = &bgpNeighbor{
		nbrAddr: atePort2.IPv4,
		isV4:    true,
		afiSafi: oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST,
		as:      ateAS}
	ebgp2NbrV6 = &bgpNeighbor{
		nbrAddr: atePort2.IPv6,
		isV4:    false,
		afiSafi: oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST,
		as:      ateAS}
	ebgpNbrs = []*bgpNeighbor{ebgp1NbrV4, ebgp2NbrV4, ebgp1NbrV6, ebgp2NbrV6}

	routes = map[string]*route{
		"prefix-set-1": {
			prefixesV4:       []string{"198.51.100.0", "198.51.100.4"},
			prefixesV6:       []string{"2048:db1:64:64::", "2048:db1:64:64::4"},
			communityMembers: nil,
		},
		"prefix-set-2": {
			prefixesV4:       []string{"198.51.100.8", "198.51.100.12"},
			prefixesV6:       []string{"2048:db1:64:64::8", "2048:db1:64:64::c"},
			communityMembers: [][]int{{5, 5}, {6, 6}},
		},
	}

	communitySets = []communitySet{
		{
			name:    "match_std_comms",
			members: []string{"5:5"},
		},
		{
			name:    "add_std_comms",
			members: []string{"10:10", "20:20", "30:30"},
		},
	}
)

type route struct {
	prefixesV4       []string
	prefixesV6       []string
	communityMembers [][]int
}

type communitySet struct {
	name    string
	members []string
}

type bgpNeighbor struct {
	as      uint32
	nbrAddr string
	isV4    bool
	afiSafi oc.E_BgpTypes_AFI_SAFI_TYPE
}

func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	dc := gnmi.OC()
	i1 := dutPort1.NewOCInterface(dut.Port(t, "port1").Name(), dut)
	gnmi.Replace(t, dut, dc.Interface(i1.GetName()).Config(), i1)

	i2 := dutPort2.NewOCInterface(dut.Port(t, "port2").Name(), dut)
	gnmi.Replace(t, dut, dc.Interface(i2.GetName()).Config(), i2)
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, dut.Port(t, "port1"))
		fptest.SetPortSpeed(t, dut.Port(t, "port2"))
	}
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, dut.Port(t, "port1").Name(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, dut.Port(t, "port2").Name(), deviations.DefaultNetworkInstance(dut), 0)
	}

	// Configure PERMIT_ALL Policy
	d := &oc.Root{}
	rp := d.GetOrCreateRoutingPolicy()
	pdef := rp.GetOrCreatePolicyDefinition(acceptPolicy)
	stmt, _ := pdef.AppendNewStatement("10")
	stmt.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
	gnmi.Update(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)

	// Configure Community Sets on DUT
	for _, communitySet := range communitySets {
		configureCommunitySet(t, dut, communitySet)
	}
}

func bgpCreateNbr(localAs, peerAs uint32, dut *ondatra.DUTDevice) *oc.NetworkInstance_Protocol {

	// Configure BGP on DUT
	dutOcRoot := &oc.Root{}
	ni1 := dutOcRoot.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niProto := ni1.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := niProto.GetOrCreateBgp()

	global := bgp.GetOrCreateGlobal()
	global.RouterId = ygot.String(dutPort1.IPv4)
	global.As = ygot.Uint32(localAs)
	global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
	global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)

	pg := bgp.GetOrCreatePeerGroup(peerGrpName)
	pg.PeerAs = ygot.Uint32(ateAS)
	pg.PeerGroupName = ygot.String(peerGrpName)
	if !deviations.SkipBgpSendCommunityType(dut) {
		pg.SetSendCommunityType([]oc.E_Bgp_CommunityType{oc.Bgp_CommunityType_STANDARD})
	}
	as4 := pg.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
	as4.Enabled = ygot.Bool(true)
	as6 := pg.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
	as6.Enabled = ygot.Bool(true)

	for _, nbr := range ebgpNbrs {
		bgpNbr := bgp.GetOrCreateNeighbor(nbr.nbrAddr)
		bgpNbr.PeerGroup = ygot.String(peerGrpName)
		bgpNbr.PeerAs = ygot.Uint32(nbr.as)
		bgpNbr.Enabled = ygot.Bool(true)

		if nbr.isV4 == true {
			af4 := bgpNbr.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
			af4.Enabled = ygot.Bool(true)
			if nbr.nbrAddr == atePort2.IPv4 {
				af4.GetOrCreateApplyPolicy().ExportPolicy = []string{acceptPolicy}
			}
			af6 := bgpNbr.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
			af6.Enabled = ygot.Bool(false)
		} else {
			af4 := bgpNbr.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
			af4.Enabled = ygot.Bool(false)
			af6 := bgpNbr.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
			af6.Enabled = ygot.Bool(true)
			if nbr.nbrAddr == atePort2.IPv6 {
				af6.GetOrCreateApplyPolicy().ExportPolicy = []string{acceptPolicy}
			}
		}
	}
	return niProto
}

func verifyBgpState(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	bgpPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()

	t.Logf("Waiting for BGP neighbor to establish...")
	for _, nbr := range ebgpNbrs {
		nbrPath := bgpPath.Neighbor(nbr.nbrAddr)
		var status *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]
		status, ok := gnmi.Watch(t, dut, nbrPath.SessionState().State(), time.Minute, func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
			state, ok := val.Val()
			return ok && state == oc.Bgp_Neighbor_SessionState_ESTABLISHED
		}).Await(t)
		if !ok {
			fptest.LogQuery(t, "BGP reported state", nbrPath.State(), gnmi.Get(t, dut, nbrPath.State()))
			t.Fatal("No BGP neighbor formed")
		}
		state, _ := status.Val()
		t.Logf("BGP adjacency for %s: %v", nbr.nbrAddr, state)
		if want := oc.Bgp_Neighbor_SessionState_ESTABLISHED; state != want {
			t.Errorf("BGP peer %s status got %d, want %d", nbr.nbrAddr, state, want)
		}
	}
}

func configureCommunitySet(t *testing.T, dut *ondatra.DUTDevice, communitySet communitySet) {
	root := &oc.Root{}
	rp := root.GetOrCreateRoutingPolicy()
	commSet := rp.GetOrCreateDefinedSets().GetOrCreateBgpDefinedSets().GetOrCreateCommunitySet(communitySet.name)
	var commMemberUnion []oc.RoutingPolicy_DefinedSets_BgpDefinedSets_CommunitySet_CommunityMember_Union
	for _, commMember := range communitySet.members {
		commMemberUnion = append(commMemberUnion, oc.UnionString(commMember))
	}
	commSet.SetCommunityMember(commMemberUnion)
	gnmi.Update(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)
}

func configureRoutingPolicy(t *testing.T, dut *ondatra.DUTDevice, policyName string, nbr *bgpNeighbor, pgName string) {
	d := &oc.Root{}
	rp := d.GetOrCreateRoutingPolicy()
	batchConfig := &gnmi.SetBatch{}

	var pdef *oc.RoutingPolicy_PolicyDefinition

	switch policyName {
	case "add_std_comms":
		pdef = rp.GetOrCreatePolicyDefinition(policyName)
		stmt1, _ := pdef.AppendNewStatement("add_std_comms")
		sc := stmt1.GetOrCreateActions().GetOrCreateBgpActions().GetOrCreateSetCommunity()
		sc.GetOrCreateReference().SetCommunitySetRef("add_std_comms")
		sc.SetOptions(oc.BgpPolicy_BgpSetCommunityOptionType_ADD)
		if !deviations.BgpActionsSetCommunityMethodUnsupported(dut) {
			sc.SetMethod(oc.SetCommunity_Method_REFERENCE)
		}
		stmt1.GetOrCreateActions().SetPolicyResult(oc.RoutingPolicy_PolicyResultType_NEXT_STATEMENT)
		stmt2, _ := pdef.AppendNewStatement("accept_all_routes")
		stmt2.GetOrCreateActions().SetPolicyResult(oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)
	case "match_and_add_comms":
		pdef = rp.GetOrCreatePolicyDefinition(policyName)
		stmt1, _ := pdef.AppendNewStatement("match_and_add_std_comms")
		if deviations.BGPConditionsMatchCommunitySetUnsupported(dut) {
			stmt1.GetOrCreateConditions().GetOrCreateBgpConditions().SetCommunitySet("match_std_comms")
			ds := rp.GetOrCreateDefinedSets()
			cs := ds.GetOrCreateBgpDefinedSets().GetOrCreateCommunitySet("match_std_comms")
			cs.SetMatchSetOptions(oc.BgpPolicy_MatchSetOptionsType_ANY)
			gnmi.BatchUpdate(batchConfig, gnmi.OC().RoutingPolicy().DefinedSets().Config(), ds)
		} else {
			cs := stmt1.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet()
			cs.SetCommunitySet("match_std_comms")
			cs.SetMatchSetOptions(oc.RoutingPolicy_MatchSetOptionsType_ANY)
		}
		sc := stmt1.GetOrCreateActions().GetOrCreateBgpActions().GetOrCreateSetCommunity()
		sc.GetOrCreateReference().SetCommunitySetRef("add_std_comms")
		sc.SetOptions(oc.BgpPolicy_BgpSetCommunityOptionType_ADD)
		if !deviations.BgpActionsSetCommunityMethodUnsupported(dut) {
			sc.SetMethod(oc.SetCommunity_Method_REFERENCE)
		}
		stmt1.GetOrCreateActions().SetPolicyResult(oc.RoutingPolicy_PolicyResultType_NEXT_STATEMENT)
		stmt2, _ := pdef.AppendNewStatement("accept_all_routes")
		stmt2.GetOrCreateActions().SetPolicyResult(oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)
	}

	if pdef != nil {
		gnmi.BatchReplace(batchConfig, gnmi.OC().RoutingPolicy().PolicyDefinition(policyName).Config(), pdef)
	}

	bgpPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	if nbr != nil {
		gnmi.BatchReplace(batchConfig, bgpPath.Neighbor(nbr.nbrAddr).AfiSafi(nbr.afiSafi).ApplyPolicy().ImportPolicy().Config(), []string{policyName})
	}
	if pgName != "" {
		gnmi.BatchReplace(batchConfig, bgpPath.PeerGroup(pgName).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).ApplyPolicy().ImportPolicy().Config(), []string{policyName})
		gnmi.BatchReplace(batchConfig, bgpPath.PeerGroup(pgName).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).ApplyPolicy().ImportPolicy().Config(), []string{policyName})
		gnmi.BatchDelete(batchConfig, bgpPath.Neighbor(ebgp1NbrV4.nbrAddr).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).ApplyPolicy().ImportPolicy().Config())
		gnmi.BatchDelete(batchConfig, bgpPath.Neighbor(ebgp1NbrV6.nbrAddr).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).ApplyPolicy().ImportPolicy().Config())
	}

	batchConfig.Set(t, dut)
}

func configureOTG(t *testing.T, otg *otg.OTG) gosnappi.Config {
	t.Helper()
	config := gosnappi.NewConfig()
	port1 := config.Ports().Add().SetName("port1")
	port2 := config.Ports().Add().SetName("port2")

	// Port1 Configuration.
	iDut1Dev := config.Devices().Add().SetName(atePort1.Name)
	iDut1Eth := iDut1Dev.Ethernets().Add().SetName(atePort1.Name + ".Eth").SetMac(atePort1.MAC)
	iDut1Eth.Connection().SetPortName(port1.Name())
	iDut1Ipv4 := iDut1Eth.Ipv4Addresses().Add().SetName(atePort1.Name + ".IPv4")
	iDut1Ipv4.SetAddress(atePort1.IPv4).SetGateway(dutPort1.IPv4).SetPrefix(uint32(atePort1.IPv4Len))
	iDut1Ipv6 := iDut1Eth.Ipv6Addresses().Add().SetName(atePort1.Name + ".IPv6")
	iDut1Ipv6.SetAddress(atePort1.IPv6).SetGateway(dutPort1.IPv6).SetPrefix(uint32(atePort1.IPv6Len))

	// Port2 Configuration.
	iDut2Dev := config.Devices().Add().SetName(atePort2.Name)
	iDut2Eth := iDut2Dev.Ethernets().Add().SetName(atePort2.Name + ".Eth").SetMac(atePort2.MAC)
	iDut2Eth.Connection().SetPortName(port2.Name())
	iDut2Ipv4 := iDut2Eth.Ipv4Addresses().Add().SetName(atePort2.Name + ".IPv4")
	iDut2Ipv4.SetAddress(atePort2.IPv4).SetGateway(dutPort2.IPv4).SetPrefix(uint32(atePort2.IPv4Len))
	iDut2Ipv6 := iDut2Eth.Ipv6Addresses().Add().SetName(atePort2.Name + ".IPv6")
	iDut2Ipv6.SetAddress(atePort2.IPv6).SetGateway(dutPort2.IPv6).SetPrefix(uint32(atePort2.IPv6Len))

	// eBGP v4 session on Port1.
	iDut1Bgp := iDut1Dev.Bgp().SetRouterId(iDut1Ipv4.Address())
	iDut1Bgp4Peer := iDut1Bgp.Ipv4Interfaces().Add().SetIpv4Name(iDut1Ipv4.Name()).Peers().Add().SetName(atePort1.Name + ".BGP4.peer")
	iDut1Bgp4Peer.SetPeerAddress(iDut1Ipv4.Gateway()).SetAsNumber(ateAS).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
	iDut1Bgp4Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true)
	// eBGP v6 session on Port1.
	iDut1Bgp6Peer := iDut1Bgp.Ipv6Interfaces().Add().SetIpv6Name(iDut1Ipv6.Name()).Peers().Add().SetName(atePort1.Name + ".BGP6.peer")
	iDut1Bgp6Peer.SetPeerAddress(iDut1Ipv6.Gateway()).SetAsNumber(ateAS).SetAsType(gosnappi.BgpV6PeerAsType.EBGP)
	iDut1Bgp6Peer.LearnedInformationFilter().SetUnicastIpv6Prefix(true)

	// eBGP v4 session on Port2.
	iDut2Bgp := iDut2Dev.Bgp().SetRouterId(iDut2Ipv4.Address())
	iDut2Bgp4Peer := iDut2Bgp.Ipv4Interfaces().Add().SetIpv4Name(iDut2Ipv4.Name()).Peers().Add().SetName(atePort2.Name + ".BGP4.peer")
	iDut2Bgp4Peer.SetPeerAddress(iDut2Ipv4.Gateway()).SetAsNumber(ateAS).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
	iDut2Bgp4Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true)
	// eBGP v6 session on Port2.
	iDut2Bgp6Peer := iDut2Bgp.Ipv6Interfaces().Add().SetIpv6Name(iDut2Ipv6.Name()).Peers().Add().SetName(atePort2.Name + ".BGP6.peer")
	iDut2Bgp6Peer.SetPeerAddress(iDut2Ipv6.Gateway()).SetAsNumber(ateAS).SetAsType(gosnappi.BgpV6PeerAsType.EBGP)
	iDut2Bgp6Peer.LearnedInformationFilter().SetUnicastIpv6Prefix(true)

	for key, sendRoute := range routes {
		// eBGP V4 routes from Port1.
		bgpNeti1Bgp4PeerRoutes := iDut1Bgp4Peer.V4Routes().Add().SetName(atePort1.Name + ".BGP4.Route." + key)
		bgpNeti1Bgp4PeerRoutes.SetNextHopIpv4Address(iDut1Ipv4.Address()).
			SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).
			SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)

		for _, prefixV4 := range sendRoute.prefixesV4 {
			bgpNeti1Bgp4PeerRoutes.Addresses().Add().SetAddress(prefixV4).SetPrefix(plenIPv4)
		}

		// eBGP V6 routes from Port1.
		bgpNeti1Bgp6PeerRoutes := iDut1Bgp6Peer.V6Routes().Add().SetName(atePort1.Name + ".BGP6.Route." + key)
		bgpNeti1Bgp6PeerRoutes.SetNextHopIpv6Address(iDut1Ipv6.Address()).
			SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).
			SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL)

		for _, prefixV6 := range sendRoute.prefixesV6 {
			bgpNeti1Bgp6PeerRoutes.Addresses().Add().SetAddress(prefixV6).SetPrefix(plenIPv6)
		}

		if sendRoute.communityMembers != nil {
			for _, community := range sendRoute.communityMembers {
				commV4 := bgpNeti1Bgp4PeerRoutes.Communities().Add()
				commV4.SetType(gosnappi.BgpCommunityType.MANUAL_AS_NUMBER)
				commV4.SetAsNumber(uint32(community[0]))
				commV4.SetAsCustom(uint32(community[1]))

				commV6 := bgpNeti1Bgp6PeerRoutes.Communities().Add()
				commV6.SetType(gosnappi.BgpCommunityType.MANUAL_AS_NUMBER)
				commV6.SetAsNumber(uint32(community[0]))
				commV6.SetAsCustom(uint32(community[1]))
			}
		}
	}

	// ATE Traffic Configuration.
	t.Logf("TestBGP:start ate Traffic config")

	var dstBgp4PeerRoutes, dst4Prefixes []string
	for _, routeV4 := range iDut1Bgp4Peer.V4Routes().Items() {
		dstBgp4PeerRoutes = append(dstBgp4PeerRoutes, routeV4.Name())
		for _, prefix := range routeV4.Addresses().Items() {
			dst4Prefixes = append(dst4Prefixes, prefix.Address())
		}
	}
	flowipv4 := config.Flows().Add().SetName("bgpv4RoutesFlow")
	flowipv4.Metrics().SetEnable(true)
	flowipv4.TxRx().Device().
		SetTxNames([]string{iDut2Ipv4.Name()}).
		SetRxNames(dstBgp4PeerRoutes)
	flowipv4.Size().SetFixed(512)
	flowipv4.Duration().FixedPackets().SetPackets(1000)
	e1 := flowipv4.Packet().Add().Ethernet()
	e1.Src().SetValue(iDut2Eth.Mac())
	v4 := flowipv4.Packet().Add().Ipv4()
	v4.Src().SetValue(iDut2Ipv4.Address())
	v4.Dst().SetValues(dst4Prefixes)

	var dstBgp6PeerRoutes, dst6Prefixes []string
	for _, routeV6 := range iDut1Bgp6Peer.V6Routes().Items() {
		dstBgp6PeerRoutes = append(dstBgp6PeerRoutes, routeV6.Name())
		for _, prefix := range routeV6.Addresses().Items() {
			dst6Prefixes = append(dst6Prefixes, prefix.Address())
		}
	}
	flowipv6 := config.Flows().Add().SetName("bgpv6RoutesFlow")
	flowipv6.Metrics().SetEnable(true)
	flowipv6.TxRx().Device().
		SetTxNames([]string{iDut2Ipv6.Name()}).
		SetRxNames(dstBgp6PeerRoutes)
	flowipv6.Size().SetFixed(512)
	flowipv6.Duration().FixedPackets().SetPackets(1000)
	e2 := flowipv6.Packet().Add().Ethernet()
	e2.Src().SetValue(iDut2Eth.Mac())
	v6 := flowipv6.Packet().Add().Ipv6()
	v6.Src().SetValue(iDut2Ipv6.Address())
	v6.Dst().SetValues(dst6Prefixes)

	otg.PushConfig(t, config)
	otg.StartProtocols(t)
	return config
}

func sendTraffic(t *testing.T, otg *otg.OTG) {
	t.Logf("Starting traffic")
	otg.StartTraffic(t)
	time.Sleep(trafficDuration)
	t.Logf("Stop traffic")
	otg.StopTraffic(t)
}

func verifyTraffic(t *testing.T, ate *ondatra.ATEDevice, conf gosnappi.Config) {
	otg := ate.OTG()
	otgutils.LogFlowMetrics(t, otg, conf)
	for _, flow := range conf.Flows().Items() {
		recvMetric := gnmi.Get(t, otg, gnmi.OTG().Flow(flow.Name()).State())
		txPackets := float32(recvMetric.GetCounters().GetOutPkts())
		rxPackets := float32(recvMetric.GetCounters().GetInPkts())
		if txPackets == 0 {
			t.Fatalf("TxPkts = 0, want > 0")
		}
		lostPackets := txPackets - rxPackets
		lossPct := lostPackets * 100 / txPackets
		if lossPct > tolerancePct {
			t.Errorf("Traffic Loss Pct for Flow %s: got %v, want max %v pct failure", flow.Name(), lossPct, tolerancePct)
		} else {
			t.Logf("Traffic Test Passed! for flow %s", flow.Name())
		}
	}
}

func validateATEIPv4PrefixCommunitySet(t *testing.T, ate *ondatra.ATEDevice, bgpPeerName, subnet string, wantCommunitySet []string) {
	otg := ate.OTG()
	var gotCommunitySet []string
	peerPath := gnmi.OTG().BgpPeer(bgpPeerName)

	_, ok := gnmi.Watch(t,
		otg,
		peerPath.UnicastIpv4Prefix(subnet, plenIPv4, otgtelemetry.UnicastIpv4Prefix_Origin_IGP, 0).State(),
		time.Minute,
		func(v *ygnmi.Value[*otgtelemetry.BgpPeer_UnicastIpv4Prefix]) bool {
			prefix, ok := v.Val()
			if ok {
				gotCommunitySet = nil
				for _, community := range prefix.Community {
					gotCommunityNumber := community.GetCustomAsNumber()
					gotCommunityValue := community.GetCustomAsValue()
					gotCommunitySet = append(gotCommunitySet, fmt.Sprint(gotCommunityNumber)+":"+fmt.Sprint(gotCommunityValue))
				}
				if cmp.Equal(gotCommunitySet, wantCommunitySet, cmpopts.SortSlices(func(a, b string) bool { return a < b })) {
					t.Logf("ATE: Prefix %v learned with community %v", prefix.GetAddress(), gotCommunitySet)
					return true
				}
				prefix.Community = nil
			}
			return false
		}).Await(t)

	if !ok {
		fptest.LogQuery(t, "ATE BGP Peer reported state", peerPath.State(), gnmi.Get(t, otg, peerPath.State()))
		t.Errorf("ATE: Prefix %v got communities %v, want communities %v", subnet, gotCommunitySet, wantCommunitySet)
	}
}

func validateDutIPv4PrefixCommunitySet(t *testing.T, dut *ondatra.DUTDevice, bgpNbr *bgpNeighbor, subnet string, wantCommunitySet []string) {
	bgpPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	statePath := bgpPath.Rib().AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Ipv4Unicast()
	state := gnmi.Get(t, dut, statePath.State())

	if communityIndex := state.GetNeighbor(bgpNbr.nbrAddr).GetAdjRibInPost().GetRoute(subnet, 0).GetCommunityIndex(); communityIndex != 0 {
		t.Logf("DUT: Prefix %v learned with CommunityIndex: %v", subnet, communityIndex)
	} else {
		fptest.LogQuery(t, "Node BGP", statePath.State(), state)
		t.Logf("DUT: Could not find AdjRibInPost Community for Prefix %v", subnet)
	}
	// TODO Validate Community for ipv4 prefixes on DUT
}

func validateATEIPv6PrefixCommunitySet(t *testing.T, ate *ondatra.ATEDevice, bgpPeerName, subnet string, wantCommunitySet []string) {
	otg := ate.OTG()
	var gotCommunitySet []string
	peerPath := gnmi.OTG().BgpPeer(bgpPeerName)

	_, ok := gnmi.Watch(t,
		otg,
		peerPath.UnicastIpv6Prefix(subnet, plenIPv6, otgtelemetry.UnicastIpv6Prefix_Origin_IGP, 0).State(),
		time.Minute,
		func(v *ygnmi.Value[*otgtelemetry.BgpPeer_UnicastIpv6Prefix]) bool {
			prefix, ok := v.Val()
			if ok {
				for _, community := range prefix.Community {
					gotCommunityNumber := community.GetCustomAsNumber()
					gotCommunityValue := community.GetCustomAsValue()
					gotCommunitySet = append(gotCommunitySet, fmt.Sprint(gotCommunityNumber)+":"+fmt.Sprint(gotCommunityValue))
				}
				if cmp.Equal(gotCommunitySet, wantCommunitySet, cmpopts.SortSlices(func(a, b string) bool { return a < b })) {
					t.Logf("ATE: Prefix %v learned with community %v", prefix.GetAddress(), gotCommunitySet)
					return true
				}
				prefix.Community = nil
				gotCommunitySet = nil
			}
			return false
		}).Await(t)

	if !ok {
		fptest.LogQuery(t, "ATE BGP Peer reported state", peerPath.State(), gnmi.Get(t, otg, peerPath.State()))
		t.Errorf("ATE: Prefix %v got communities %v, want communities %v", subnet, gotCommunitySet, wantCommunitySet)
	}
}

func validateDutIPv6PrefixCommunitySet(t *testing.T, dut *ondatra.DUTDevice, bgpNbr *bgpNeighbor, subnet string, wantCommunitySet []string) {
	bgpPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	statePath := bgpPath.Rib().AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Ipv6Unicast()
	state := gnmi.Get(t, dut, statePath.State())

	if communityIndex := state.GetNeighbor(bgpNbr.nbrAddr).GetAdjRibInPost().GetRoute(subnet, 0).GetCommunityIndex(); communityIndex != 0 {
		t.Logf("DUT: Prefix %v learned with CommunityIndex: %v", subnet, communityIndex)
	} else {
		fptest.LogQuery(t, "Node BGP", statePath.State(), state)
		t.Logf("DUT: Could not find AdjRibInPost Community for Prefix %v", subnet)
	}
	// TODO Validate Community for ipv6 prefixes on DUT
}

type TestResults struct {
	prefixSetName    string
	wantCommunitySet []string
}

type testCase struct {
	desc        string
	nbr         *bgpNeighbor
	peerGrp     string
	policyName  string
	testResults []TestResults
}

// TestBGPCommMatchAction is to test community match actions at BGP neighbor & peer group levels.
func TestBGPCommMatchAction(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	otg := ate.OTG()

	configureDUT(t, dut)

	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	gnmi.Delete(t, dut, dutConfPath.Config())
	dutConf := bgpCreateNbr(dutAS, ateAS, dut)
	gnmi.Replace(t, dut, dutConfPath.Config(), dutConf)

	otgConfig := configureOTG(t, otg)
	verifyBgpState(t, dut)

	t.Run("RT-7.8.1", func(t *testing.T) {
		testCases := []testCase{
			{
				desc:       "Validate Initial Config",
				peerGrp:    "",
				policyName: acceptPolicy,
				testResults: []TestResults{
					{
						prefixSetName:    "prefix-set-1",
						wantCommunitySet: nil,
					},
					{
						prefixSetName:    "prefix-set-2",
						wantCommunitySet: []string{"5:5", "6:6"},
					},
				},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.desc, func(t *testing.T) {
				configureRoutingPolicy(t, dut, tc.policyName, ebgp1NbrV4, tc.peerGrp)
				configureRoutingPolicy(t, dut, tc.policyName, ebgp1NbrV6, tc.peerGrp)
				for _, testResult := range tc.testResults {
					for _, prefix := range routes[testResult.prefixSetName].prefixesV4 {
						validateATEIPv4PrefixCommunitySet(t, ate, atePort2.Name+".BGP4.peer", prefix, testResult.wantCommunitySet)
						validateDutIPv4PrefixCommunitySet(t, dut, ebgp1NbrV4, prefix, nil)
					}
					for _, prefix := range routes[testResult.prefixSetName].prefixesV6 {
						validateATEIPv6PrefixCommunitySet(t, ate, atePort2.Name+".BGP6.peer", prefix, testResult.wantCommunitySet)
						validateDutIPv6PrefixCommunitySet(t, dut, ebgp1NbrV6, prefix, nil)
					}
				}
				// Starting ATE Traffic and verify Traffic Flows
				sendTraffic(t, otg)
				verifyTraffic(t, ate, otgConfig)
			})
		}
	})

	t.Run("RT-7.8.2", func(t *testing.T) {
		testCases := []testCase{
			{
				desc:       "neighborV4-match_and_add_comms",
				nbr:        ebgp1NbrV4,
				peerGrp:    "",
				policyName: "match_and_add_comms",
				testResults: []TestResults{
					{
						prefixSetName:    "prefix-set-1",
						wantCommunitySet: nil,
					},
					{
						prefixSetName:    "prefix-set-2",
						wantCommunitySet: []string{"5:5", "6:6", "10:10", "20:20", "30:30"},
					},
				},
			},
			{
				desc:       "neighborV6-match_and_add_comms",
				nbr:        ebgp1NbrV6,
				peerGrp:    "",
				policyName: "match_and_add_comms",
				testResults: []TestResults{
					{
						prefixSetName:    "prefix-set-1",
						wantCommunitySet: nil,
					},
					{
						prefixSetName:    "prefix-set-2",
						wantCommunitySet: []string{"5:5", "6:6", "10:10", "20:20", "30:30"},
					},
				},
			},
			{
				desc:       "PeerGrp-match_and_add_comms",
				nbr:        nil,
				peerGrp:    peerGrpName,
				policyName: "match_and_add_comms",
				testResults: []TestResults{
					{
						prefixSetName:    "prefix-set-1",
						wantCommunitySet: nil,
					},
					{
						prefixSetName:    "prefix-set-2",
						wantCommunitySet: []string{"5:5", "6:6", "10:10", "20:20", "30:30"},
					},
				},
			},
			{
				desc:       "neighborV4-add_std_comms",
				nbr:        ebgp1NbrV4,
				peerGrp:    "",
				policyName: "add_std_comms",
				testResults: []TestResults{
					{
						prefixSetName:    "prefix-set-1",
						wantCommunitySet: []string{"10:10", "20:20", "30:30"},
					},
					{
						prefixSetName:    "prefix-set-2",
						wantCommunitySet: []string{"5:5", "6:6", "10:10", "20:20", "30:30"},
					},
				},
			},
			{
				desc:       "neighborV6-add_std_comms",
				nbr:        ebgp1NbrV6,
				peerGrp:    "",
				policyName: "add_std_comms",
				testResults: []TestResults{
					{
						prefixSetName:    "prefix-set-1",
						wantCommunitySet: []string{"10:10", "20:20", "30:30"},
					},
					{
						prefixSetName:    "prefix-set-2",
						wantCommunitySet: []string{"5:5", "6:6", "10:10", "20:20", "30:30"},
					},
				},
			},
			{
				desc:       "PeerGrp-add_std_comms",
				nbr:        nil,
				peerGrp:    peerGrpName,
				policyName: "add_std_comms",
				testResults: []TestResults{
					{
						prefixSetName:    "prefix-set-1",
						wantCommunitySet: []string{"10:10", "20:20", "30:30"},
					},
					{
						prefixSetName:    "prefix-set-2",
						wantCommunitySet: []string{"5:5", "6:6", "10:10", "20:20", "30:30"},
					},
				},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.desc, func(t *testing.T) {
				configureRoutingPolicy(t, dut, tc.policyName, tc.nbr, tc.peerGrp)
				for _, testResult := range tc.testResults {
					for _, prefix := range routes[testResult.prefixSetName].prefixesV4 {
						if (tc.nbr == nil) || (tc.nbr != nil && tc.nbr.isV4 == true) {
							validateATEIPv4PrefixCommunitySet(t, ate, atePort2.Name+".BGP4.peer", prefix, testResult.wantCommunitySet)
							validateDutIPv4PrefixCommunitySet(t, dut, ebgp1NbrV4, prefix, nil)
						}
					}
					for _, prefix := range routes[testResult.prefixSetName].prefixesV6 {
						if (tc.nbr == nil) || (tc.nbr != nil && tc.nbr.isV4 != true) {
							validateATEIPv6PrefixCommunitySet(t, ate, atePort2.Name+".BGP6.peer", prefix, testResult.wantCommunitySet)
							validateDutIPv6PrefixCommunitySet(t, dut, ebgp1NbrV6, prefix, nil)
						}
					}
				}
			})
		}
	})
}
