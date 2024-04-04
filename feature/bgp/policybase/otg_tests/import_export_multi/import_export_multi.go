// Copyright 2023 Google LLC
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

package import_export_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/open_traffic_generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra"
)

const (
	prefixV4Len                            = 30
	prefixV6Len                            = 126
	trafficPps                             = 100
	totalPackets                           = 1200
	bgpName                                = "BGP"
	medValue                               = 100
	parentPolicy                           = "multiPolicy"
	callPolicy                             = "match_community_regex"
	rejectStatement                        = "reject_route_community"
	nestedRejectStatement                  = "if_30_and_not_20_nested_reject"
	callPolicyStatement                    = "match_community_regex"
	addMissingCommunitiesStatement         = "add_communities_if_missing"
	matchCommPrefixAddCommuStatement       = "match_comm_and_prefix_add_2_community_sets"
	matchAspathSetMedStatement             = "match_aspath_set_med"
	rejectPolicyStatementResult            = oc.RoutingPolicy_PolicyResultType_REJECT_ROUTE
	nestedRejectPolicyStatementResult      = oc.RoutingPolicy_PolicyResultType_REJECT_ROUTE
	callPolicyStatementResult              = oc.RoutingPolicy_PolicyResultType_NEXT_STATEMENT
	addMissingCommunitiesStatementResult   = oc.RoutingPolicy_PolicyResultType_NEXT_STATEMENT
	rejectCommunitySet                     = "reject_communities"
	nestedRejectCommunitySet               = "accept_communities"
	regexCommunitySet                      = "regex-community"
	addCommunitiesSetRefs                  = "add-communities"
	myCommunitySet                         = "my_community"
	prefixSetName                          = "prefix-set-5"
	myAsPathName                           = "my_aspath"
	rejectMatchSetOptions                  = oc.BgpPolicy_MatchSetOptionsType_ANY
	nestedRejectMatchSetOptions            = oc.BgpPolicy_MatchSetOptionsType_INVERT
	regexMatchSetOptions                   = oc.BgpPolicy_MatchSetOptionsType_ANY
	addCommunitiesSetRefsMatchSetOptions   = oc.BgpPolicy_MatchSetOptionsType_INVERT
	bgpActionMethod                        = oc.SetCommunity_Method_REFERENCE
	bgpSetCommunityOptionType              = oc.BgpPolicy_BgpSetCommunityOptionType_ADD
	matchCommPrefixAddCommuStatementResult = oc.RoutingPolicy_PolicyResultType_NEXT_STATEMENT
	matchCommPrefixAddCommuSetOptions      = oc.BgpPolicy_MatchSetOptionsType_ANY
	prefixSetNameSetOptions                = oc.RoutingPolicy_MatchSetOptionsRestrictedType_ANY
	matchSetOptions                        = oc.BgpPolicy_MatchSetOptionsType_ANY
)

var prefixesV4 = [][]string{
	{"198.51.100.2", "198.51.100.3"},
	{"198.51.100.4", "198.51.100.5"},
	{"198.51.100.6", "198.51.100.7"},
	{"198.51.100.8", "198.51.100.9"},
	{"198.51.100.10", "198.51.100.11"},
	{"198.51.100.12", "198.51.100.13"},
}

var prefixesV6 = [][]string{
	{"2048:db1:64:64::2", "2048:db1:64:64::3"},
	{"2048:db1:64:64::4", "2048:db1:64:64::5"},
	{"2048:db1:64:64::6", "2048:db1:64:64::7"},
	{"2048:db1:64:64::8", "2048:db1:64:64::9"},
	{"2048:db1:64:64::10", "2048:db1:64:64::11"},
	{"2048:db1:64:64::12", "2048:db1:64:64::13"},
}

var communityMembers = [][][]int{
	{
		{10, 1}, {11, 1},
	},
	{
		{20, 1}, {21, 1},
	},
	{
		{30, 1}, {31, 1},
	},
	{
		{20, 2}, {30, 3},
	},
	{
		{40, 1}, {41, 1},
	},
	{
		{50, 1}, {51, 1},
	},
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func configureImportExportAcceptAllBGPPolicy(t *testing.T, dut *ondatra.DUTDevice, ipv4 string, ipv6 string, matchSetOptions oc.E_BgpPolicy_MatchSetOptionsType) {
	root := &oc.Root{}
	rp := root.GetOrCreateRoutingPolicy()
	pdef1 := rp.GetOrCreatePolicyDefinition("routePolicy")
	stmt1, err := pdef1.AppendNewStatement("routePolicyStatement")
	if err != nil {
		t.Fatalf("AppendNewStatement(%s) failed: %v", "routePolicyStatement", err)
	}
	stmt1.GetOrCreateActions().SetPolicyResult(oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)

	gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)

	dni := deviations.DefaultNetworkInstance(dut)
	pathV6 := gnmi.OC().NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).Bgp().Neighbor(ipv6).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).ApplyPolicy()
	policyV6 := root.GetOrCreateNetworkInstance(dni).GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).GetOrCreateBgp().GetOrCreateNeighbor(ipv6).GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).GetOrCreateApplyPolicy()
	policyV6.SetImportPolicy([]string{"routePolicy"})
	policyV6.SetExportPolicy([]string{"routePolicy"})
	gnmi.Replace(t, dut, pathV6.Config(), policyV6)

	pathV4 := gnmi.OC().NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).Bgp().Neighbor(ipv4).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).ApplyPolicy()
	policyV4 := root.GetOrCreateNetworkInstance(dni).GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).GetOrCreateBgp().GetOrCreateNeighbor(ipv4).GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).GetOrCreateApplyPolicy()
	policyV4.SetImportPolicy([]string{"routePolicy"})
	policyV4.SetExportPolicy([]string{"routePolicy"})
	gnmi.Replace(t, dut, pathV4.Config(), policyV4)

}

func configureImportExportMultifacetMatchActionsBGPPolicy(t *testing.T, dut *ondatra.DUTDevice, ipv4 string, ipv6 string) {
	rejectCommunities := []string{"10:1"}
	acceptCommunities := []string{"20:1"}
	regexCommunities := []string{"^30:.*$"}
	addCommunitiesRefs := []string{"40:1", "40:2"}
	addCommunitiesSetRefsAction := []string{"add-communities"}
	setCommunitySetRefs := []string{"add_comm_60", "add_comm_70"}
	myCommunitySets := []string{"50:1"}

	root := &oc.Root{}
	rp := root.GetOrCreateRoutingPolicy()
	pdef1 := rp.GetOrCreatePolicyDefinition(parentPolicy)
	stmt1, err := pdef1.AppendNewStatement(rejectStatement)
	if err != nil {
		t.Fatalf("AppendNewStatement(%s) failed: %v", rejectStatement, err)
	}
	stmt1.GetOrCreateActions().SetPolicyResult(rejectPolicyStatementResult)

	communitySetReject := rp.GetOrCreateDefinedSets().GetOrCreateBgpDefinedSets().GetOrCreateCommunitySet(rejectCommunitySet)

	cs1 := []oc.RoutingPolicy_DefinedSets_BgpDefinedSets_CommunitySet_CommunityMember_Union{}
	for _, commMatch1 := range rejectCommunities {
		if commMatch1 != "" {
			cs1 = append(cs1, oc.UnionString(commMatch1))
		}
	}
	communitySetReject.SetCommunityMember(cs1)
	communitySetReject.SetMatchSetOptions(rejectMatchSetOptions)

	if deviations.BGPConditionsMatchCommunitySetUnsupported(dut) {
		stmt1.GetOrCreateConditions().GetOrCreateBgpConditions().SetCommunitySet(rejectCommunitySet)
	} else {
		stmt1.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetCommunitySet(rejectCommunitySet)
	}

	stmt2, err := pdef1.AppendNewStatement(nestedRejectStatement)
	if err != nil {
		t.Fatalf("AppendNewStatement(%s) failed: %v", nestedRejectStatement, err)
	}
	stmt2.GetOrCreateActions().SetPolicyResult(nestedRejectPolicyStatementResult)

	communitySetNestedReject := rp.GetOrCreateDefinedSets().GetOrCreateBgpDefinedSets().GetOrCreateCommunitySet(nestedRejectCommunitySet)

	cs2 := []oc.RoutingPolicy_DefinedSets_BgpDefinedSets_CommunitySet_CommunityMember_Union{}
	for _, commMatch2 := range acceptCommunities {
		if commMatch2 != "" {
			cs2 = append(cs2, oc.UnionString(commMatch2))
		}
	}
	communitySetNestedReject.SetCommunityMember(cs2)
	communitySetNestedReject.SetMatchSetOptions(nestedRejectMatchSetOptions)

	if deviations.BGPConditionsMatchCommunitySetUnsupported(dut) {
		stmt2.GetOrCreateConditions().GetOrCreateBgpConditions().SetCommunitySet(nestedRejectCommunitySet)
	} else {
		stmt2.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetCommunitySet(nestedRejectCommunitySet)
	}

	// defining policy "match_community_regex" will be called from "multiPolicy" policy

	pdef2 := rp.GetOrCreatePolicyDefinition(callPolicy)
	stmt3, err := pdef2.AppendNewStatement(callPolicyStatement)
	if err != nil {
		t.Fatalf("AppendNewStatement(%s) failed: %v", callPolicyStatement, err)
	}
	stmt3.GetOrCreateActions().SetPolicyResult(callPolicyStatementResult)

	communitySetRegex := rp.GetOrCreateDefinedSets().GetOrCreateBgpDefinedSets().GetOrCreateCommunitySet(regexCommunitySet)

	cs3 := []oc.RoutingPolicy_DefinedSets_BgpDefinedSets_CommunitySet_CommunityMember_Union{}
	for _, commMatch3 := range regexCommunities {
		if commMatch3 != "" {
			cs3 = append(cs3, oc.UnionString(commMatch3))
		}
	}
	communitySetRegex.SetCommunityMember(cs3)
	communitySetRegex.SetMatchSetOptions(regexMatchSetOptions)

	if deviations.BGPConditionsMatchCommunitySetUnsupported(dut) {
		stmt3.GetOrCreateConditions().GetOrCreateBgpConditions().SetCommunitySet(regexCommunitySet)
	} else {
		stmt3.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetCommunitySet(regexCommunitySet)
	}

	gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)

	// Configure the nested policy.
	dni := deviations.DefaultNetworkInstance(dut)
	rpPolicy := root.GetOrCreateRoutingPolicy()
	statPath := rpPolicy.GetOrCreatePolicyDefinition(parentPolicy).GetStatement(nestedRejectStatement).GetConditions()
	statPath.SetCallPolicy(callPolicy)

	gnmi.Update(t, dut, gnmi.OC().RoutingPolicy().Config(), rpPolicy)

	stmt4, err := pdef1.AppendNewStatement(addMissingCommunitiesStatement)
	if err != nil {
		t.Fatalf("AppendNewStatement(%s) failed: %v", addMissingCommunitiesStatement, err)
	}
	stmt4.GetOrCreateActions().SetPolicyResult(addMissingCommunitiesStatementResult)

	communitySetRefsAddCommunities := rp.GetOrCreateDefinedSets().GetOrCreateBgpDefinedSets().GetOrCreateCommunitySet(addCommunitiesSetRefs)

	cs4 := []oc.RoutingPolicy_DefinedSets_BgpDefinedSets_CommunitySet_CommunityMember_Union{}
	for _, commMatch4 := range addCommunitiesRefs {
		if commMatch4 != "" {
			cs4 = append(cs4, oc.UnionString(commMatch4))
		}
	}
	communitySetRefsAddCommunities.SetCommunityMember(cs4)
	communitySetRefsAddCommunities.SetMatchSetOptions(addCommunitiesSetRefsMatchSetOptions)

	if deviations.BGPConditionsMatchCommunitySetUnsupported(dut) {
		stmt4.GetOrCreateConditions().GetOrCreateBgpConditions().SetCommunitySet(addCommunitiesSetRefs)
	} else {
		if deviations.BgpCommunitySetRefsUnSupported(dut) {
			t.Logf("TODO: community-set-refs not supported b/316833803")
			stmt4.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetCommunitySet(addCommunitiesSetRefs)
		}
	}

	if deviations.BgpCommunitySetRefsUnSupported(dut) {
		t.Logf("TODO: community-set-refs not supported b/316833803")
	} else {
		stmt4.GetOrCreateActions().GetOrCreateBgpActions().GetSetCommunity().GetOrCreateReference().SetCommunitySetRefs(addCommunitiesSetRefsAction)
		stmt4.GetOrCreateActions().GetOrCreateBgpActions().GetSetCommunity().SetMethod(bgpActionMethod)
		stmt4.GetOrCreateActions().GetOrCreateBgpActions().GetSetCommunity().SetOptions(bgpSetCommunityOptionType)
	}

	stmt5, err := pdef1.AppendNewStatement(matchCommPrefixAddCommuStatement)
	if err != nil {
		t.Fatalf("AppendNewStatement(%s) failed: %v", matchCommPrefixAddCommuStatement, err)
	}
	stmt5.GetOrCreateActions().SetPolicyResult(matchCommPrefixAddCommuStatementResult)

	communitySetMatchCommPrefixAddCommu := rp.GetOrCreateDefinedSets().GetOrCreateBgpDefinedSets().GetOrCreateCommunitySet(myCommunitySet)

	cs5 := []oc.RoutingPolicy_DefinedSets_BgpDefinedSets_CommunitySet_CommunityMember_Union{}
	for _, commMatch5 := range myCommunitySets {
		if commMatch5 != "" {
			cs5 = append(cs5, oc.UnionString(commMatch5))
		}
	}
	communitySetMatchCommPrefixAddCommu.SetCommunityMember(cs5)
	communitySetMatchCommPrefixAddCommu.SetMatchSetOptions(matchCommPrefixAddCommuSetOptions)

	stmt5.GetOrCreateConditions().GetOrCreateMatchPrefixSet().SetPrefixSet(prefixSetName)
	stmt5.GetOrCreateConditions().GetOrCreateMatchPrefixSet().SetMatchSetOptions(prefixSetNameSetOptions)

	if deviations.BGPConditionsMatchCommunitySetUnsupported(dut) {
		stmt5.GetOrCreateConditions().GetOrCreateBgpConditions().SetCommunitySet(addCommunitiesSetRefs)
	} else {
		stmt5.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetCommunitySet(addCommunitiesSetRefs)
	}

	if deviations.BgpCommunitySetRefsUnSupported(dut) {
		t.Logf("TODO: community-set-refs not supported b/316833803")
	} else {
		stmt5.GetOrCreateActions().GetOrCreateBgpActions().GetSetCommunity().GetOrCreateReference().SetCommunitySetRefs(setCommunitySetRefs)
		stmt5.GetOrCreateActions().GetOrCreateBgpActions().GetSetCommunity().SetMethod(oc.SetCommunity_Method_REFERENCE)
		stmt5.GetOrCreateActions().GetOrCreateBgpActions().GetSetCommunity().SetOptions(oc.BgpPolicy_BgpSetCommunityOptionType_ADD)
	}

	stmt6, err := pdef2.AppendNewStatement(matchAspathSetMedStatement)
	if err != nil {
		t.Fatalf("AppendNewStatement(%s) failed: %v", matchAspathSetMedStatement, err)
	}
	stmt6.GetOrCreateActions().SetPolicyResult(oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)

	# TODO: ADD match-as-path-set verification
	stmt6.GetOrCreateActions().GetOrCreateBgpActions().SetMed = oc.UnionUint32(medValue)

	gnmi.Update(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)

	// Configure the parent BGP import and export policy.
	pathV6 := gnmi.OC().NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).Bgp().Neighbor(ipv6).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).ApplyPolicy()
	policyV6 := root.GetOrCreateNetworkInstance(dni).GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).GetOrCreateBgp().GetOrCreateNeighbor(ipv6).GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).GetOrCreateApplyPolicy()
	policyV6.SetImportPolicy([]string{parentPolicy})
	policyV6.SetExportPolicy([]string{parentPolicy})
	gnmi.Replace(t, dut, pathV6.Config(), policyV6)

	pathV4 := gnmi.OC().NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).Bgp().Neighbor(ipv4).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).ApplyPolicy()
	policyV4 := root.GetOrCreateNetworkInstance(dni).GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).GetOrCreateBgp().GetOrCreateNeighbor(ipv4).GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).GetOrCreateApplyPolicy()
	policyV4.SetImportPolicy([]string{parentPolicy})
	policyV4.SetExportPolicy([]string{parentPolicy})
	gnmi.Replace(t, dut, pathV4.Config(), policyV4)

	// TODO: create as-path-set on the DUT, match-as-path-set not support.
}

func configureOTG(t *testing.T, bs *cfgplugins.BGPSession, prefixesV4 [][]string, prefixesV6 [][]string, communityMembers [][][]int) {
	devices := bs.ATETop.Devices().Items()

	ipv4 := devices[1].Ethernets().Items()[0].Ipv4Addresses().Items()[0]
	bgp4Peer := devices[1].Bgp().Ipv4Interfaces().Items()[0].Peers().Items()[0]

	ipv6 := devices[1].Ethernets().Items()[0].Ipv6Addresses().Items()[0]
	bgp6Peer := devices[1].Bgp().Ipv6Interfaces().Items()[0].Peers().Items()[0]

	for index, prefixes := range prefixesV4 {
		bgp4PeerRoute := bgp4Peer.V4Routes().Add()
		bgp4PeerRoute.SetName(bs.ATEPorts[1].Name + ".BGP4.peer.dut." + strconv.Itoa(index))
		bgp4PeerRoute.SetNextHopIpv4Address(ipv4.Address())

		route4Address1 := bgp4PeerRoute.Addresses().Add().SetAddress(prefixes[0])
		route4Address1.SetPrefix(prefixV4Len)
		route4Address2 := bgp4PeerRoute.Addresses().Add().SetAddress(prefixes[1])
		route4Address2.SetPrefix(prefixV4Len)

		bgp6PeerRoute := bgp6Peer.V6Routes().Add()
		bgp6PeerRoute.SetName(bs.ATEPorts[1].Name + ".BGP6.peer.dut." + strconv.Itoa(index))
		bgp6PeerRoute.SetNextHopIpv6Address(ipv6.Address())

		route6Address1 := bgp6PeerRoute.Addresses().Add().SetAddress(prefixesV6[index][0])
		route6Address1.SetPrefix(prefixV6Len)
		route6Address2 := bgp6PeerRoute.Addresses().Add().SetAddress(prefixesV6[index][1])
		route6Address2.SetPrefix(prefixV6Len)

		for _, commu := range communityMembers[index] {
			if commu[0] != 0 {
				commv4 := bgp4PeerRoute.Communities().Add()
				commv4.SetType(gosnappi.BgpCommunityType.MANUAL_AS_NUMBER)
				commv4.SetAsNumber(uint32(commu[0]))
				commv4.SetAsCustom(uint32(commu[1]))

				commv6 := bgp6PeerRoute.Communities().Add()
				commv6.SetType(gosnappi.BgpCommunityType.MANUAL_AS_NUMBER)
				commv6.SetAsNumber(uint32(commu[0]))
				commv6.SetAsCustom(uint32(commu[1]))
			}
		}
	}
}

func configureFlowV4(t *testing.T, bs *cfgplugins.BGPSession) {
	for index, prefixPairV4 := range prefixesV4 {
		flow := bs.ATETop.Flows().Add().SetName("flow" + "ipv4" + strconv.Itoa(index))
		flow.Metrics().SetEnable(true)

		flow.TxRx().Device().
			SetTxNames([]string{bs.ATEPorts[0].Name + ".IPv4"}).
			SetRxNames([]string{bs.ATEPorts[1].Name + ".BGP4.peer.dut." + strconv.Itoa(index)})

		flow.Duration().FixedPackets().SetPackets(totalPackets)
		flow.Size().SetFixed(1500)
		flow.Rate().SetPps(trafficPps)

		e := flow.Packet().Add().Ethernet()
		e.Src().SetValue(bs.ATEPorts[1].MAC)

		v4 := flow.Packet().Add().Ipv4()
		v4.Src().SetValue(bs.ATEPorts[0].IPv4)
		v4.Dst().SetValues(prefixPairV4)
	}
}

func configureFlowV6(t *testing.T, bs *cfgplugins.BGPSession) {
	for index, prefixPairV6 := range prefixesV6 {
		flow := bs.ATETop.Flows().Add().SetName("flow" + "ipv6" + strconv.Itoa(index))
		flow.Metrics().SetEnable(true)

		flow.TxRx().Device().
			SetTxNames([]string{bs.ATEPorts[0].Name + ".IPv6"}).
			SetRxNames([]string{bs.ATEPorts[1].Name + ".BGP6.peer.dut." + strconv.Itoa(index)})

		flow.Duration().FixedPackets().SetPackets(totalPackets)
		flow.Size().SetFixed(1500)
		flow.Rate().SetPps(trafficPps)

		e := flow.Packet().Add().Ethernet()
		e.Src().SetValue(bs.ATEPorts[1].MAC)

		v6 := flow.Packet().Add().Ipv6()
		v6.Src().SetValue(bs.ATEPorts[0].IPv6)
		v6.Dst().SetValues(prefixPairV6)
	}
}

func verifyTrafficV4AndV6(t *testing.T, bs *cfgplugins.BGPSession, testResults [6]bool) {

	sleepTime := time.Duration(totalPackets/trafficPps) + 2
	bs.ATE.OTG().StartTraffic(t)
	time.Sleep(time.Second * sleepTime)
	bs.ATE.OTG().StopTraffic(t)

	otgutils.LogFlowMetrics(t, bs.ATE.OTG(), bs.ATETop)
	otgutils.LogPortMetrics(t, bs.ATE.OTG(), bs.ATETop)

	for index, prefixPairV4 := range prefixesV4 {
		t.Logf("Running traffic test for IPv4 prefixes: [%s, %s]. Expected Result: [%t]", prefixPairV4[0], prefixPairV4[1], testResults[index])
		t.Logf("Running traffic test for IPv6 prefixes: [%s, %s]. Expected Result: [%t]", prefixesV6[index][0], prefixesV6[index][1], testResults[index])

		t.Log("Checking flow telemetry for v4...")
		recvMetric := gnmi.Get(t, bs.ATE.OTG(), gnmi.OTG().Flow("flow"+"ipv4"+strconv.Itoa(index)).State())
		txPackets := recvMetric.GetCounters().GetOutPkts()
		rxPackets := recvMetric.GetCounters().GetInPkts()
		lostPackets := txPackets - rxPackets
		lossPct := lostPackets * 100 / txPackets

		t.Log("Checking flow telemetry for v6...")
		recvMetric6 := gnmi.Get(t, bs.ATE.OTG(), gnmi.OTG().Flow("flow"+"ipv6"+strconv.Itoa(index)).State())
		txPackets6 := recvMetric6.GetCounters().GetOutPkts()
		rxPackets6 := recvMetric6.GetCounters().GetInPkts()
		lostPackets6 := txPackets6 - rxPackets6
		lossPct6 := lostPackets6 * 100 / txPackets6

		if txPackets != rxPackets && testResults[index] {
			t.Errorf("FAIL- got %v%% packet loss for %s flow and prefixes: [%s, %s]; want < 0%% traffic loss", lossPct, "flow"+"ipv4"+strconv.Itoa(index), prefixPairV4[0], prefixPairV4[1])
			t.Errorf("FAIL- got %v%% packet loss for %s flow and prefixes: [%s, %s]; want < 0%% traffic loss", lossPct6, "flow"+"ipv6"+strconv.Itoa(index), prefixesV6[index][0], prefixesV6[index][1])
		} else if rxPackets != 0 && !testResults[index] {
			t.Errorf("FAIL- got %v%% packet loss for %s flow and prefixes: [%s, %s]; want >100%% traffic loss", lossPct, "flow"+"ipv4"+strconv.Itoa(index), prefixPairV4[0], prefixPairV4[1])
			t.Errorf("FAIL- got %v%% packet loss for %s flow and prefixes: [%s, %s]; want >100%% traffic loss", lossPct6, "flow"+"ipv6"+strconv.Itoa(index), prefixesV6[index][0], prefixesV6[index][1])
		} else {
			t.Logf("Traffic validation successful for Prefixes: [%s, %s]. Result: [%t] PacketsTx: %d PacketsRx: %d", prefixPairV4[0], prefixPairV4[1], testResults[index], txPackets, rxPackets)
			t.Logf("Traffic validation successful for Prefixes: [%s, %s]. Result: [%t] PacketsTx: %d PacketsRx: %d", prefixesV6[index][0], prefixesV6[index][1], testResults[index], txPackets6, rxPackets6)
		}

	}
}

func TestImportExportMultifacetMatchActionsBGPPolicy(t *testing.T) {
	bs := cfgplugins.NewBGPSession(t, cfgplugins.PortCount2, nil)
	bs.WithEBGP(t, []oc.E_BgpTypes_AFI_SAFI_TYPE{oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST, oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST}, []string{
		"port1", "port2"}, true, false)

	configureOTG(t, bs, prefixesV4, prefixesV6, communityMembers)
	bs.PushAndStart(t)

	t.Log("Verify DUT BGP sessions up")
	cfgplugins.VerifyDUTBGPEstablished(t, bs.DUT)
	t.Log("Verify OTG BGP sessions up")
	cfgplugins.VerifyOTGBGPEstablished(t, bs.ATE)

	ipv4 := bs.ATETop.Devices().Items()[1].Ethernets().Items()[0].Ipv4Addresses().Items()[0].Address()
	ipv6 := bs.ATETop.Devices().Items()[1].Ethernets().Items()[0].Ipv6Addresses().Items()[0].Address()

	t.Logf("Verify Import Export Accept all bgp policy")
	configureImportExportAcceptAllBGPPolicy(t, bs.DUT, ipv4, ipv6, matchSetOptions)

	configureFlowV4(t, bs)
	configureFlowV6(t, bs)

	bs.PushAndStartATE(t)

	testResults := [6]bool{true, true, true, true, true, true}
	verifyTrafficV4AndV6(t, bs, testResults)

	configureImportExportMultifacetMatchActionsBGPPolicy(t, bs.DUT, ipv4, ipv6)

	testResults1 := [6]bool{false, true, false, false, true, true}
	verifyTrafficV4AndV6(t, bs, testResults1)
}
