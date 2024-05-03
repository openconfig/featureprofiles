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

// Package import_export_test covers RT-7.11: BGP Policy - Import/Export Policy Action Using Multiple Criteria
package import_export_multi_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
)

const (
	prefixV4Len                      = 30
	prefixV6Len                      = 126
	trafficPps                       = 100
	totalPackets                     = 1200
	bgpName                          = "BGP"
	medValue                         = 100
	localPref                        = 5
	parentPolicy                     = "multiPolicy"
	callPolicy                       = "match_community_regex"
	rejectStatement                  = "reject_route_community"
	nestedRejectStatement            = "if_30_and_not_20_nested_reject"
	callPolicyStatement              = "match_community_regex"
	addMissingCommunitiesStatement   = "add_communities_if_missing"
	matchCommPrefixAddCommuStatement = "match_comm_and_prefix_add_2_community_sets"
	matchAspathSetMedStatement       = "match_aspath_set_med"
	rejectCommunitySet               = "reject_communities"
	nestedRejectCommunitySet         = "accept_communities"
	regexCommunitySet                = "regex-community"
	addCommunitiesSetRefs            = "add-communities"
	myCommunitySet                   = "my_community"
	prefixSetName                    = "prefix-set-5"
	myAsPathName                     = "my_aspath"
	bgpActionMethod                  = oc.SetCommunity_Method_REFERENCE
	bgpSetCommunityOptionType        = oc.BgpPolicy_BgpSetCommunityOptionType_ADD
	prefixSetNameSetOptions          = oc.RoutingPolicy_MatchSetOptionsRestrictedType_ANY
	matchAny                         = oc.BgpPolicy_MatchSetOptionsType_ANY
	matchInvert                      = oc.BgpPolicy_MatchSetOptionsType_INVERT
	rejectResult                     = oc.RoutingPolicy_PolicyResultType_REJECT_ROUTE
	nextstatementResult              = oc.RoutingPolicy_PolicyResultType_NEXT_STATEMENT
)

var prefixesV4 = [][]string{
	{"198.51.100.0", "198.51.100.4"},
	{"198.51.100.8", "198.51.100.12"},
	{"198.51.100.16", "198.51.100.20"},
	{"198.51.100.24", "198.51.100.28"},
	{"198.51.100.32", "198.51.100.36"},
	{"198.51.100.40", "198.51.100.44"},
}

var prefixesV6 = [][]string{
	{"2048:db1:64:64::0", "2048:db1:64:64::4"},
	{"2048:db1:64:64::8", "2048:db1:64:64::c"},
	{"2048:db1:64:64::10", "2048:db1:64:64::14"},
	{"2048:db1:64:64::18", "2048:db1:64:64::1c"},
	{"2048:db1:64:64::20", "2048:db1:64:64::24"},
	{"2048:db1:64:64::28", "2048:db1:64:64::2c"},
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

// TestMain triggers the test run
func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func configureImportExportAcceptAllBGPPolicy(t *testing.T, dut *ondatra.DUTDevice, ipv4 string, ipv6 string) {
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

	// Configure the policy match_community_regex which will be called from multi_policy

	pdef2 := rp.GetOrCreatePolicyDefinition(callPolicy)

	// Configure match_community_regex:STATEMENT1:match_community_regex statement

	pd2stmt1, err := pdef2.AppendNewStatement(callPolicyStatement)
	if err != nil {
		t.Fatalf("AppendNewStatement(%s) failed: %v", callPolicyStatement, err)
	}

	// Configure regex_community:["^30:.*$"] to match_community_regex statement
	communitySetRegex := rp.GetOrCreateDefinedSets().GetOrCreateBgpDefinedSets().GetOrCreateCommunitySet(regexCommunitySet)

	pd2cs1 := []oc.RoutingPolicy_DefinedSets_BgpDefinedSets_CommunitySet_CommunityMember_Union{}
	for _, commMatchPd2Cs1 := range regexCommunities {
		if commMatchPd2Cs1 != "" {
			pd2cs1 = append(pd2cs1, oc.UnionString(commMatchPd2Cs1))
		}
	}
	communitySetRegex.SetCommunityMember(pd2cs1)
	communitySetRegex.SetMatchSetOptions(matchAny)

	if deviations.BGPConditionsMatchCommunitySetUnsupported(dut) {
		pd2stmt1.GetOrCreateConditions().GetOrCreateBgpConditions().SetCommunitySet(regexCommunitySet)
	} else {
		pd2stmt1.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetCommunitySet(regexCommunitySet)
	}

	pd2stmt1.GetOrCreateActions().SetPolicyResult(nextstatementResult)

	// Configure the parent policy multi_policy.

	pdef1 := rp.GetOrCreatePolicyDefinition(parentPolicy)

	// Configure multi_policy:STATEMENT1: reject_route_community
	stmt1, err := pdef1.AppendNewStatement(rejectStatement)
	if err != nil {
		t.Fatalf("AppendNewStatement(%s) failed: %v", rejectStatement, err)
	}

	// Configure reject_communities:[10:1] to reject_route_community statement
	communitySetReject := rp.GetOrCreateDefinedSets().GetOrCreateBgpDefinedSets().GetOrCreateCommunitySet(rejectCommunitySet)

	cs1 := []oc.RoutingPolicy_DefinedSets_BgpDefinedSets_CommunitySet_CommunityMember_Union{}
	for _, commMatch1 := range rejectCommunities {
		if commMatch1 != "" {
			cs1 = append(cs1, oc.UnionString(commMatch1))
		}
	}
	communitySetReject.SetCommunityMember(cs1)
	communitySetReject.SetMatchSetOptions(matchAny)

	if deviations.BGPConditionsMatchCommunitySetUnsupported(dut) {
		stmt1.GetOrCreateConditions().GetOrCreateBgpConditions().SetCommunitySet(rejectCommunitySet)
	} else {
		stmt1.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetCommunitySet(rejectCommunitySet)
	}

	stmt1.GetOrCreateActions().SetPolicyResult(rejectResult)

	// Configure multi_policy:STATEMENT2:if_30:.*_and_not_20:1_nested_reject

	stmt2, err := pdef1.AppendNewStatement(nestedRejectStatement)
	if err != nil {
		t.Fatalf("AppendNewStatement(%s) failed: %v", nestedRejectStatement, err)
	}

	// Call child policy match_community_regex from parent policy multi_policy

	statPath := rp.GetOrCreatePolicyDefinition(parentPolicy).GetStatement(nestedRejectStatement)
	statPath.GetOrCreateConditions().SetCallPolicy(callPolicy)

	// Configure accept_communities:[20:1] to if_30_and_not_20_nested_reject statement
	communitySetNestedReject := rp.GetOrCreateDefinedSets().GetOrCreateBgpDefinedSets().GetOrCreateCommunitySet(nestedRejectCommunitySet)

	cs2 := []oc.RoutingPolicy_DefinedSets_BgpDefinedSets_CommunitySet_CommunityMember_Union{}
	for _, commMatch2 := range acceptCommunities {
		if commMatch2 != "" {
			cs2 = append(cs2, oc.UnionString(commMatch2))
		}
	}
	communitySetNestedReject.SetCommunityMember(cs2)
	communitySetNestedReject.SetMatchSetOptions(matchInvert)

	if deviations.BGPConditionsMatchCommunitySetUnsupported(dut) {
		stmt2.GetOrCreateConditions().GetOrCreateBgpConditions().SetCommunitySet(nestedRejectCommunitySet)
	} else {
		stmt2.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetCommunitySet(nestedRejectCommunitySet)
	}

	stmt2.GetOrCreateActions().SetPolicyResult(rejectResult)

	// Configure multi_policy:STATEMENT3: add_communities_if_missing statement
	stmt3, err := pdef1.AppendNewStatement(addMissingCommunitiesStatement)
	if err != nil {
		t.Fatalf("AppendNewStatement(%s) failed: %v", addMissingCommunitiesStatement, err)
	}

	// Configure add-communities: [ "40:1", "40:2" ] to add_communities_if_missing statement

	communitySetRefsAddCommunities := rp.GetOrCreateDefinedSets().GetOrCreateBgpDefinedSets().GetOrCreateCommunitySet(addCommunitiesSetRefs)

	cs3 := []oc.RoutingPolicy_DefinedSets_BgpDefinedSets_CommunitySet_CommunityMember_Union{}
	for _, commMatch4 := range addCommunitiesRefs {
		if commMatch4 != "" {
			cs3 = append(cs3, oc.UnionString(commMatch4))
		}
	}
	communitySetRefsAddCommunities.SetCommunityMember(cs3)
	communitySetRefsAddCommunities.SetMatchSetOptions(matchInvert)

	if deviations.BGPConditionsMatchCommunitySetUnsupported(dut) {
		stmt3.GetOrCreateConditions().GetOrCreateBgpConditions().SetCommunitySet(addCommunitiesSetRefs)
	} else {
		if deviations.BgpCommunitySetRefsUnsupported(dut) {
			t.Logf("TODO: community-set-refs not supported b/316833803")
			stmt3.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetCommunitySet(addCommunitiesSetRefs)
		}
	}

	if deviations.BgpCommunitySetRefsUnsupported(dut) {
		t.Logf("TODO: community-set-refs not supported b/316833803")
	} else {
		stmt3.GetOrCreateActions().GetOrCreateBgpActions().GetSetCommunity().GetOrCreateReference().SetCommunitySetRefs(addCommunitiesSetRefsAction)
		stmt3.GetOrCreateActions().GetOrCreateBgpActions().GetSetCommunity().SetMethod(bgpActionMethod)
		stmt3.GetOrCreateActions().GetOrCreateBgpActions().GetSetCommunity().SetOptions(bgpSetCommunityOptionType)
	}

	stmt3.GetOrCreateActions().SetPolicyResult(nextstatementResult)

	// Configure multi_policy:STATEMENT4: match_comm_and_prefix_add_2_community_sets statement

	stmt4, err := pdef1.AppendNewStatement(matchCommPrefixAddCommuStatement)
	if err != nil {
		t.Fatalf("AppendNewStatement(%s) failed: %v", matchCommPrefixAddCommuStatement, err)
	}

	// Configure my_community: [  "50:1"  ] to match_comm_and_prefix_add_2_community_sets statement
	communitySetMatchCommPrefixAddCommu := rp.GetOrCreateDefinedSets().GetOrCreateBgpDefinedSets().GetOrCreateCommunitySet(myCommunitySet)

	cs4 := []oc.RoutingPolicy_DefinedSets_BgpDefinedSets_CommunitySet_CommunityMember_Union{}
	for _, commMatch5 := range myCommunitySets {
		if commMatch5 != "" {
			cs4 = append(cs4, oc.UnionString(commMatch5))
		}
	}
	communitySetMatchCommPrefixAddCommu.SetCommunityMember(cs4)
	communitySetMatchCommPrefixAddCommu.SetMatchSetOptions(matchAny)

	if deviations.BGPConditionsMatchCommunitySetUnsupported(dut) {
		stmt4.GetOrCreateConditions().GetOrCreateBgpConditions().SetCommunitySet(myCommunitySet)
	} else {
		stmt4.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetCommunitySet(myCommunitySet)
	}

	// configure match-prefix-set: prefix-set-5 to match_comm_and_prefix_add_2_community_sets statement
	stmt4.GetOrCreateConditions().GetOrCreateMatchPrefixSet().SetPrefixSet(prefixSetName)
	stmt4.GetOrCreateConditions().GetOrCreateMatchPrefixSet().SetMatchSetOptions(prefixSetNameSetOptions)

	pset := rp.GetOrCreateDefinedSets().GetOrCreatePrefixSet(prefixSetName)
	pset.GetOrCreatePrefix(prefixesV4[4][0]+"/29", "29..30")
	if !deviations.SkipPrefixSetMode(dut) {
		pset.SetMode(oc.PrefixSet_Mode_IPV4)
	}

	psetV6 := rp.GetOrCreateDefinedSets().GetOrCreatePrefixSet(prefixSetName + "_V6")
	psetV6.GetOrCreatePrefix(prefixesV6[4][0]+"/125", "125..126")
	if !deviations.SkipPrefixSetMode(dut) {
		psetV6.SetMode(oc.PrefixSet_Mode_IPV6)
	}

	gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().DefinedSets().PrefixSet(prefixSetName).Config(), pset)
	gnmi.Update(t, dut, gnmi.OC().RoutingPolicy().DefinedSets().PrefixSet(prefixSetName+"_V6").Config(), psetV6)

	if deviations.BgpCommunitySetRefsUnsupported(dut) {
		t.Logf("TODO: community-set-refs not supported b/316833803")
	} else {
		// TODO: Add bgp-actions: community-set-refs to match_comm_and_prefix_add_2_community_sets statement
		stmt4.GetOrCreateActions().GetOrCreateBgpActions().GetSetCommunity().GetOrCreateReference().SetCommunitySetRefs(setCommunitySetRefs)
		stmt4.GetOrCreateActions().GetOrCreateBgpActions().GetSetCommunity().SetMethod(oc.SetCommunity_Method_REFERENCE)
		stmt4.GetOrCreateActions().GetOrCreateBgpActions().GetSetCommunity().SetOptions(oc.BgpPolicy_BgpSetCommunityOptionType_ADD)
	}
	// set-local-pref = 5
	stmt4.GetOrCreateActions().GetOrCreateBgpActions().SetSetLocalPref(localPref)

	stmt4.GetOrCreateActions().SetPolicyResult(nextstatementResult)

	// Configure multi_policy:STATEMENT5: match_aspath_set_med statement
	stmt5, err := pdef1.AppendNewStatement(matchAspathSetMedStatement)
	if err != nil {
		t.Fatalf("AppendNewStatement(%s) failed: %v", matchAspathSetMedStatement, err)
	}

	// TODO create as-path-set on the DUT, match-as-path-set not support.
	// Configure set-med 100
	stmt5.GetOrCreateActions().GetOrCreateBgpActions().SetMed = oc.UnionUint32(medValue)

	stmt5.GetOrCreateActions().SetPolicyResult(oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)

	gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)

	// Configure the parent BGP import and export policy.
	dni := deviations.DefaultNetworkInstance(dut)
	pathV6 := gnmi.OC().NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).Bgp().Neighbor(ipv6).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).ApplyPolicy()
	policyV6 := root.GetOrCreateNetworkInstance(dni).GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).GetOrCreateBgp().GetOrCreateNeighbor(ipv6).GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).GetOrCreateApplyPolicy()
	policyV6.SetImportPolicy([]string{parentPolicy})
	policyV6.SetExportPolicy([]string{parentPolicy})
	policyV6.SetDefaultImportPolicy(oc.RoutingPolicy_DefaultPolicyType_REJECT_ROUTE)
	policyV6.SetDefaultExportPolicy(oc.RoutingPolicy_DefaultPolicyType_REJECT_ROUTE)
	gnmi.Replace(t, dut, pathV6.Config(), policyV6)

	pathV4 := gnmi.OC().NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).Bgp().Neighbor(ipv4).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).ApplyPolicy()
	policyV4 := root.GetOrCreateNetworkInstance(dni).GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).GetOrCreateBgp().GetOrCreateNeighbor(ipv4).GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).GetOrCreateApplyPolicy()
	policyV4.SetImportPolicy([]string{parentPolicy})
	policyV4.SetExportPolicy([]string{parentPolicy})
	policyV4.SetDefaultImportPolicy(oc.RoutingPolicy_DefaultPolicyType_REJECT_ROUTE)
	policyV4.SetDefaultExportPolicy(oc.RoutingPolicy_DefaultPolicyType_REJECT_ROUTE)
	gnmi.Replace(t, dut, pathV4.Config(), policyV4)
}

func configureOTG(t *testing.T, bs *cfgplugins.BGPSession, prefixesV4 [][]string, prefixesV6 [][]string, communityMembers [][][]int) {
	t.Logf("configure OTG")
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
	t.Logf("configure V4 Flow on traffic generator")
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
	t.Logf("configure V6 Flow on traffic generator")
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
		} else if rxPackets != 0 && !testResults[index] {
			t.Errorf("FAIL- got %v%% packet loss for %s flow and prefixes: [%s, %s]; want >100%% traffic loss", lossPct, "flow"+"ipv4"+strconv.Itoa(index), prefixPairV4[0], prefixPairV4[1])
		} else if txPackets6 != rxPackets6 && testResults[index] {
			t.Errorf("FAIL- got %v%% packet loss for %s flow and prefixes: [%s, %s]; want < 0%% traffic loss", lossPct6, "flow"+"ipv6"+strconv.Itoa(index), prefixesV6[index][0], prefixesV6[index][1])
		} else if rxPackets6 != 0 && !testResults[index] {
			t.Errorf("FAIL- got %v%% packet loss for %s flow and prefixes: [%s, %s]; want >100%% traffic loss", lossPct6, "flow"+"ipv6"+strconv.Itoa(index), prefixesV6[index][0], prefixesV6[index][1])
		} else {
			t.Logf("Traffic validation successful for Prefixes: [%s, %s]. Result: [%t] PacketsTx: %d PacketsRx: %d", prefixesV6[index][0], prefixesV6[index][1], testResults[index], txPackets6, rxPackets6)
		}

	}
}

// TestImportExportMultifacetMatchActionsBGPPolicy covers RT-7.11
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
	configureImportExportAcceptAllBGPPolicy(t, bs.DUT, ipv4, ipv6)

	configureFlowV4(t, bs)
	configureFlowV6(t, bs)

	bs.PushAndStartATE(t)

	testResults := [6]bool{true, true, true, true, true, true}
	verifyTrafficV4AndV6(t, bs, testResults)

	configureImportExportMultifacetMatchActionsBGPPolicy(t, bs.DUT, ipv4, ipv6)

	testResults1 := [6]bool{false, true, false, false, true, true}
	verifyTrafficV4AndV6(t, bs, testResults1)
}
