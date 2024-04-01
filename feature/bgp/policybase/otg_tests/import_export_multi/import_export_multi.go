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

package import_export_multi

import (
	"strconv"
	"testing"
	"time"

	"github.com/open_traffic_generator/gosnappi"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra"
)

const (
	prefixV4Len           = 30
	prefixV6Len           = 126
	trafficPps            = 100
	totalPackets          = 1200
	bgpName               = "BGP"
	parentPolicyName      = "multi_policy"
	callPolicyName        = "match_community_regex"
	parentPolicyStatement = "if_30:.*_and_not_20:1_nested_reject"
	communitySetNameTC3   = "accept_communities"
	callPolicyStatement   = "match_community_regex"
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

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func configureImportExportAcceptAllBGPPolicy(t *testing.T, dut *ondatra.DUTDevice, ipv4 string, ipv6 string, communitySetName string, communityMatch [3]string, matchSetOptions oc.E_BgpPolicy_MatchSetOptionsType) {
	root := &oc.Root{}
	rp := root.GetOrCreateRoutingPolicy()
	pdef1 := rp.GetOrCreatePolicyDefinition("routePolicy")
	stmt1, err := pdef1.AppendNewStatement("routePolicyStatement")
	if err != nil {
		t.Fatalf("AppendNewStatement(%s) failed: %v", "routePolicyStatement", err)
	}
	stmt1.GetOrCreateActions().SetPolicyResult(oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)

	communitySet := rp.GetOrCreateDefinedSets().GetOrCreateBgpDefinedSets().GetOrCreateCommunitySet(communitySetName)

	cs := []oc.RoutingPolicy_DefinedSets_BgpDefinedSets_CommunitySet_CommunityMember_Union{}
	for _, commMatch := range communityMatch {
		if commMatch != "" {
			cs = append(cs, oc.UnionString(commMatch))
		}
	}
	communitySet.SetCommunityMember(cs)
	communitySet.SetMatchSetOptions(matchSetOptions)

	if deviations.BGPConditionsMatchCommunitySetUnsupported(dut) {
		stmt1.GetOrCreateConditions().GetOrCreateBgpConditions().SetCommunitySet(communitySetName)
	} else {
		stmt1.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetCommunitySet(communitySetName)
	}

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

	// TODO: create as-path-set on the DUT, match-as-path-set not support, vendor is working on it.
}

func configureImportExportRejectBGPPolicy(t *testing.T, dut *ondatra.DUTDevice, ipv4 string, ipv6 string, communitySetName string, communityMatch [3]string, matchSetOptions oc.E_BgpPolicy_MatchSetOptionsType) {
	root := &oc.Root{}
	rp := root.GetOrCreateRoutingPolicy()
	pdef1 := rp.GetOrCreatePolicyDefinition("multi_policy")
	stmt1, err := pdef1.AppendNewStatement("reject_route_community")
	if err != nil {
		t.Fatalf("AppendNewStatement(%s) failed: %v", "reject_route_community", err)
	}
	stmt1.GetOrCreateActions().SetPolicyResult(oc.RoutingPolicy_PolicyResultType_REJECT_ROUTE)

	communitySet := rp.GetOrCreateDefinedSets().GetOrCreateBgpDefinedSets().GetOrCreateCommunitySet(communitySetName)

	cs := []oc.RoutingPolicy_DefinedSets_BgpDefinedSets_CommunitySet_CommunityMember_Union{}
	for _, commMatch := range communityMatch {
		if commMatch != "" {
			cs = append(cs, oc.UnionString(commMatch))
		}
	}
	communitySet.SetCommunityMember(cs)
	communitySet.SetMatchSetOptions(matchSetOptions)

	if deviations.BGPConditionsMatchCommunitySetUnsupported(dut) {
		stmt1.GetOrCreateConditions().GetOrCreateBgpConditions().SetCommunitySet(communitySetName)
	} else {
		stmt1.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetCommunitySet(communitySetName)
	}

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

	// TODO: create as-path-set on the DUT, match-as-path-set not support, vendor is working on it.
}

func configureImportExportNestedBGPPolicy(t *testing.T, dut *ondatra.DUTDevice, ipv4 string, ipv6 string) {
	root := &oc.Root{}
	rp := root.GetOrCreateRoutingPolicy()

	// Configure a route-policy to set the local preference.
	pdef1 := rp.GetOrCreatePolicyDefinition(parentPolicyName)
	stmt1, err := pdef1.AppendNewStatement(parentPolicyStatement)
	if err != nil {
		t.Fatalf("AppendNewStatement(%s) failed: %v", parentPolicyStatement, err)
	}
	stmt1.GetOrCreateActions().SetPolicyResult(oc.RoutingPolicy_PolicyResultType_REJECT_ROUTE)

	communitySet := rp.GetOrCreateDefinedSets().GetOrCreateBgpDefinedSets().GetOrCreateCommunitySet(communitySetNameTC3)

	communityMatch := [3]string{"20:1"}
	cs := []oc.RoutingPolicy_DefinedSets_BgpDefinedSets_CommunitySet_CommunityMember_Union{}
	for _, commMatch := range communityMatch {
		if commMatch != "" {
			cs = append(cs, oc.UnionString(commMatch))
		}
	}
	communitySet.SetCommunityMember(cs)
	communitySet.SetMatchSetOptions(oc.BgpPolicy_MatchSetOptionsType_INVERT)

	if deviations.BGPConditionsMatchCommunitySetUnsupported(dut) {
		stmt1.GetOrCreateConditions().GetOrCreateBgpConditions().SetCommunitySet(communitySetNameTC3)
	} else {
		stmt1.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetCommunitySet(communitySetNameTC3)
	}

	// Configure a route-policy to match the community_regex.
	pdef2 := rp.GetOrCreatePolicyDefinition(callPolicyName)
	stmt2, err := pdef2.AppendNewStatement(callPolicyStatement)
	if err != nil {
		t.Fatalf("AppendNewStatement(%s) failed: %v", callPolicyStatement, err)
	}
	stmt2.GetOrCreateActions().SetPolicyResult(oc.RoutingPolicy_PolicyResultType_NEXT_STATEMENT)
	gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)

	// Configure the nested policy.
	dni := deviations.DefaultNetworkInstance(dut)
	rpPolicy := root.GetOrCreateRoutingPolicy()
	statPath := rpPolicy.GetOrCreatePolicyDefinition(parentPolicyName).GetStatement(parentPolicyStatement).GetConditions()
	statPath.SetCallPolicy(callPolicyName)
	gnmi.Update(t, dut, gnmi.OC().RoutingPolicy().Config(), rpPolicy)

	// Configure the parent BGP import and export policy.
	pathV6 := gnmi.OC().NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).Bgp().Neighbor(ipv6).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).ApplyPolicy()
	policyV6 := root.GetOrCreateNetworkInstance(dni).GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).GetOrCreateBgp().GetOrCreateNeighbor(ipv6).GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).GetOrCreateApplyPolicy()
	policyV6.SetImportPolicy([]string{parentPolicyName})
	policyV6.SetExportPolicy([]string{parentPolicyName})
	gnmi.Update(t, dut, pathV6.Config(), policyV6)

	pathV4 := gnmi.OC().NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).Bgp().Neighbor(ipv4).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).ApplyPolicy()
	policyV4 := root.GetOrCreateNetworkInstance(dni).GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).GetOrCreateBgp().GetOrCreateNeighbor(ipv4).GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).GetOrCreateApplyPolicy()
	policyV4.SetImportPolicy([]string{parentPolicyName})
	policyV4.SetExportPolicy([]string{parentPolicyName})
	gnmi.Update(t, dut, pathV4.Config(), policyV4)

	// TODO: create as-path-set on the DUT, match-as-path-set not support, vendor is working on it.
}

func configureMultiStatementsBgpPolicy(t *testing.T, dut *ondatra.DUTDevice, ipv4 string, ipv6 string, communitySetName string, communityMatch [3]string, matchSetOptions oc.E_BgpPolicy_MatchSetOptionsType) {
	root := &oc.Root{}
	rp := root.GetOrCreateRoutingPolicy()
	pdef1 := rp.GetOrCreatePolicyDefinition("multi_policy")
	stmt1, err := pdef1.AppendNewStatement("add_communities_if_missing")
	if err != nil {
		t.Fatalf("AppendNewStatement(%s) failed: %v", "add_communities_if_missing", err)
	}
	stmt1.GetOrCreateActions().SetPolicyResult(oc.RoutingPolicy_PolicyResultType_NEXT_STATEMENT)
	
	// TODO: Arista: community-set-refs is not supported

	// TODO: create as-path-set on the DUT, match-as-path-set not support, vendor is working on it.

}

func configureOTG(t *testing.T, bs *cfgplugins.BGPSession, prefixesV4 [][]string, prefixesV6 [][]string, communityMembers [][][]int) {
	devices := bs.ATETop.Devices().Items()

	ipv4 := devices[0].Ethernets().Items()[0].Ipv4Addresses().Items()[0]
	bgp4Peer := devices[0].Bgp().Ipv4Interfaces().Items()[0].Peers().Items()[0]

	ipv6 := devices[0].Ethernets().Items()[0].Ipv6Addresses().Items()[0]
	bgp6Peer := devices[0].Bgp().Ipv6Interfaces().Items()[0].Peers().Items()[0]

	for index, prefixes := range prefixesV4 {
		bgp4PeerRoute := bgp4Peer.V4Routes().Add()
		bgp4PeerRoute.SetName(bs.ATEPorts[0].Name + ".BGP4.peer.dut." + strconv.Itoa(index))
		bgp4PeerRoute.SetNextHopIpv4Address(ipv4.Address())

		route4Address1 := bgp4PeerRoute.Addresses().Add().SetAddress(prefixes[0])
		route4Address1.SetPrefix(prefixV4Len)
		route4Address2 := bgp4PeerRoute.Addresses().Add().SetAddress(prefixes[1])
		route4Address2.SetPrefix(prefixV4Len)

		bgp6PeerRoute := bgp6Peer.V6Routes().Add()
		bgp6PeerRoute.SetName(bs.ATEPorts[0].Name + ".BGP6.peer.dut." + strconv.Itoa(index))
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

func configureFlow(t *testing.T, bs *cfgplugins.BGPSession, prefixPair []string, prefixType string, index int) {
	flow := bs.ATETop.Flows().Add().SetName("flow" + prefixType)
	flow.Metrics().SetEnable(true)

	if prefixType == "ipv4" {
		flow.TxRx().Device().
			SetTxNames([]string{bs.ATEPorts[0].Name + ".IPv4"}).
			SetRxNames([]string{bs.ATEPorts[1].Name + ".BGP4.peer.dut." + strconv.Itoa(index)})
	} else {
		flow.TxRx().Device().
			SetTxNames([]string{bs.ATEPorts[0].Name + ".IPv6"}).
			SetRxNames([]string{bs.ATEPorts[1].Name + ".BGP6.peer.dut." + strconv.Itoa(index)})
	}

	flow.Duration().FixedPackets().SetPackets(totalPackets)
	flow.Size().SetFixed(1500)
	flow.Rate().SetPps(trafficPps)

	e := flow.Packet().Add().Ethernet()
	e.Src().SetValue(bs.ATEPorts[1].MAC)

	if prefixType == "ipv4" {
		v4 := flow.Packet().Add().Ipv4()
		v4.Src().SetValue(bs.ATEPorts[0].IPv4)
		v4.Dst().SetValues(prefixPair)
	} else {
		v6 := flow.Packet().Add().Ipv6()
		v6.Src().SetValue(bs.ATEPorts[0].IPv6)
		v6.Dst().SetValues(prefixPair)
	}
}

func verifyTraffic(t *testing.T, ate *ondatra.ATEDevice, ports int, testResults bool) {
	framesTx := gnmi.Get[uint64](t, ate.OTG(), gnmi.OTG().Port(ate.Port(t, "port1").ID()).Counters().OutFrames().State())
	framesRx := gnmi.Get[uint64](t, ate.OTG(), gnmi.OTG().Port(ate.Port(t, "port2").ID()).Counters().InFrames().State())

	if framesTx == 0 {
		t.Error("No traffic was generated and frames transmitted were 0")
	} else if (testResults && framesRx == framesTx) || (!testResults && framesRx == 0) {
		t.Logf("Traffic validation successful for criteria [%t] FramesTx: %d FramesRx: %d", testResults, framesTx, framesRx)
	} else {
		t.Errorf("Traffic validation failed for criteria [%t] FramesTx: %d FramesRx: %d", testResults, framesTx, framesRx)
	}
}

type verifyPolicy struct {
	desc             string
	communitySetName string
	communityMatch   [3]string
	matchSetOptions  oc.E_BgpPolicy_MatchSetOptionsType
	testResults      [6]bool
}

func TestAcceptAllPolicy(t *testing.T) {
	bs := cfgplugins.NewBGPSession(t, cfgplugins.PortCount2, nil)
	bs.WithEBGP(t, []oc.E_BgpTypes_AFI_SAFI_TYPE{oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST, oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST}, []string{
		"port1", "port2"}, true, false)

	var communityMembers = [][][]int{
		{
			{10, 1},
		},
		{
			{20, 1},
		},
		{
			{30, 1},
		},
		{
			{20, 2}, {30, 3},
		},
		{
			{40, 1},
		},
		{
			{50, 1},
		},
	}

	configureOTG(t, bs, prefixesV4, prefixesV6, communityMembers)
	bs.PushAndStart(t)

	t.Log("Verify DUT BGP sessions up")
	cfgplugins.VerifyDUTBGPEstablished(t, bs.DUT)
	t.Log("Verify OTG BGP sessions up")
	cfgplugins.VerifyOTGBGPEstablished(t, bs.ATE)

	ipv4 := bs.ATETop.Devices().Items()[1].Ethernets().Items()[0].Ipv4Addresses().Items()[0].Address()
	ipv6 := bs.ATETop.Devices().Items()[1].Ethernets().Items()[0].Ipv6Addresses().Items()[0].Address()

	verifyPolicy := []verifyPolicy{
		{
			desc:             "Testing with reject_communities",
			communitySetName: "reject_communities",
			communityMatch:   [3]string{"10:1"},
			matchSetOptions:  oc.BgpPolicy_MatchSetOptionsType_ANY,
			testResults:      [6]bool{true, false, false, false, false, false},
		},
		{
			desc:             "Testing with accept_communities",
			communitySetName: "accept_communities",
			communityMatch:   [3]string{"20:1"},
			matchSetOptions:  oc.BgpPolicy_MatchSetOptionsType_ANY,
			testResults:      [6]bool{false, true, false, false, false, false},
		},
		{
			desc:             "Testing with regex_community",
			communitySetName: "regex_community",
			communityMatch:   [3]string{"^30:.*$"},
			matchSetOptions:  oc.BgpPolicy_MatchSetOptionsType_ANY,
			testResults:      [6]bool{false, false, true, true, false, false},
		},
		{
			desc:             "Testing with add_communities",
			communitySetName: "add_communities",
			communityMatch:   [3]string{"40:1", "40:2"},
			matchSetOptions:  oc.BgpPolicy_MatchSetOptionsType_ANY,
			testResults:      [6]bool{false, false, false, false, true, false},
		},
		{
			desc:             "Testing with my_community",
			communitySetName: "my_community",
			communityMatch:   [3]string{"50:1"},
			matchSetOptions:  oc.BgpPolicy_MatchSetOptionsType_ANY,
			testResults:      [6]bool{false, false, false, false, false, true},
		},
	}
	for _, vp := range verifyPolicy {

		configureImportExportAcceptAllBGPPolicy(t, bs.DUT, ipv4, ipv6, vp.communitySetName, vp.communityMatch, vp.matchSetOptions)

		sleepTime := time.Duration(totalPackets/trafficPps) + 2

		for index, prefixPairV4 := range prefixesV4 {

			bs.ATETop.Flows().Clear()
			configureFlow(t, bs, prefixPairV4, "ipv4", index)
			configureFlow(t, bs, prefixesV6[index], "ipv6", index)
			bs.PushAndStartATE(t)

			t.Logf("Running traffic test for IPv4 prefixes: [%s, %s]. Expected Result: [%t]", prefixPairV4[0], prefixPairV4[1], vp.testResults[index])
			bs.ATE.OTG().StartTraffic(t)
			time.Sleep(sleepTime * time.Second)
			bs.ATE.OTG().StopTraffic(t)
			otgutils.LogFlowMetrics(t, bs.ATE.OTG(), bs.ATETop)
			verifyTraffic(t, bs.ATE, int(cfgplugins.PortCount2), vp.testResults[index])

			t.Logf("Running traffic test for IPv6 prefixes: [%s, %s]. Expected Result: [%t]", prefixesV6[index][0], prefixesV6[index][1], vp.testResults[index])

			bs.ATE.OTG().StartTraffic(t)
			time.Sleep(sleepTime * time.Second)
			bs.ATE.OTG().StopTraffic(t)
			otgutils.LogFlowMetrics(t, bs.ATE.OTG(), bs.ATETop)
			verifyTraffic(t, bs.ATE, int(cfgplugins.PortCount2), vp.testResults[index])
		}
	}
}

func TestRejectPolicy(t *testing.T) {
	bs := cfgplugins.NewBGPSession(t, cfgplugins.PortCount2, nil)
	bs.WithEBGP(t, []oc.E_BgpTypes_AFI_SAFI_TYPE{oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST, oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST}, []string{
		"port1", "port2"}, true, false)

	var communityMembers = [][][]int{
		{
			{10, 1},
		},
	}

	configureOTG(t, bs, prefixesV4, prefixesV6, communityMembers)
	bs.PushAndStart(t)

	t.Log("Verify DUT BGP sessions up")
	cfgplugins.VerifyDUTBGPEstablished(t, bs.DUT)
	t.Log("Verify OTG BGP sessions up")
	cfgplugins.VerifyOTGBGPEstablished(t, bs.ATE)

	ipv4 := bs.ATETop.Devices().Items()[1].Ethernets().Items()[0].Ipv4Addresses().Items()[0].Address()
	ipv6 := bs.ATETop.Devices().Items()[1].Ethernets().Items()[0].Ipv6Addresses().Items()[0].Address()

	verifyPolicy := []verifyPolicy{
		{
			desc:             "Testing with reject_communities",
			communitySetName: "reject_communities",
			communityMatch:   [3]string{"10:1"},
			matchSetOptions:  oc.BgpPolicy_MatchSetOptionsType_ANY,
			testResults:      [6]bool{false, true, true, true, true, true},
		},
	}
	for _, vp := range verifyPolicy {

		configureImportExportRejectBGPPolicy(t, bs.DUT, ipv4, ipv6, vp.communitySetName, vp.communityMatch, vp.matchSetOptions)

		sleepTime := time.Duration(totalPackets/trafficPps) + 2

		for index, prefixPairV4 := range prefixesV4 {

			bs.ATETop.Flows().Clear()
			configureFlow(t, bs, prefixPairV4, "ipv4", index)
			configureFlow(t, bs, prefixesV6[index], "ipv6", index)
			bs.PushAndStartATE(t)

			t.Logf("Running traffic test for IPv4 prefixes: [%s, %s]. Expected Result: [%t]", prefixPairV4[0], prefixPairV4[1], vp.testResults[index])
			bs.ATE.OTG().StartTraffic(t)
			time.Sleep(sleepTime * time.Second)
			bs.ATE.OTG().StopTraffic(t)
			otgutils.LogFlowMetrics(t, bs.ATE.OTG(), bs.ATETop)
			verifyTraffic(t, bs.ATE, int(cfgplugins.PortCount2), vp.testResults[index])

			t.Logf("Running traffic test for IPv6 prefixes: [%s, %s]. Expected Result: [%t]", prefixesV6[index][0], prefixesV6[index][1], vp.testResults[index])

			bs.ATE.OTG().StartTraffic(t)
			time.Sleep(sleepTime * time.Second)
			bs.ATE.OTG().StopTraffic(t)
			otgutils.LogFlowMetrics(t, bs.ATE.OTG(), bs.ATETop)
			verifyTraffic(t, bs.ATE, int(cfgplugins.PortCount2), vp.testResults[index])
		}
	}
}

func TestNestedBgpPolicy(t *testing.T) {
	bs := cfgplugins.NewBGPSession(t, cfgplugins.PortCount2, nil)
	bs.WithEBGP(t, []oc.E_BgpTypes_AFI_SAFI_TYPE{oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST, oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST}, []string{
		"port1", "port2"}, true, false)

	var communityMembers = [][][]int{
		{
			{10, 1},
		},
	}

	configureOTG(t, bs, prefixesV4, prefixesV6, communityMembers)
	bs.PushAndStart(t)

	t.Log("Verify DUT BGP sessions up")
	cfgplugins.VerifyDUTBGPEstablished(t, bs.DUT)
	t.Log("Verify OTG BGP sessions up")
	cfgplugins.VerifyOTGBGPEstablished(t, bs.ATE)

	ipv4 := bs.ATETop.Devices().Items()[1].Ethernets().Items()[0].Ipv4Addresses().Items()[0].Address()
	ipv6 := bs.ATETop.Devices().Items()[1].Ethernets().Items()[0].Ipv6Addresses().Items()[0].Address()

	configureImportExportNestedBGPPolicy(t, bs.DUT, ipv4, ipv6)
	testResults := [6]bool{true, true, false, false, true, true}

	sleepTime := time.Duration(totalPackets/trafficPps) + 2

	for index, prefixPairV4 := range prefixesV4 {

		bs.ATETop.Flows().Clear()
		configureFlow(t, bs, prefixPairV4, "ipv4", index)
		configureFlow(t, bs, prefixesV6[index], "ipv6", index)
		bs.PushAndStartATE(t)

		t.Logf("Running traffic test for IPv4 prefixes: [%s, %s]. Expected Result: [%t]", prefixPairV4[0], prefixPairV4[1], testResults[index])
		bs.ATE.OTG().StartTraffic(t)
		time.Sleep(sleepTime * time.Second)
		bs.ATE.OTG().StopTraffic(t)
		otgutils.LogFlowMetrics(t, bs.ATE.OTG(), bs.ATETop)
		verifyTraffic(t, bs.ATE, int(cfgplugins.PortCount2), testResults[index])

		t.Logf("Running traffic test for IPv6 prefixes: [%s, %s]. Expected Result: [%t]", prefixesV6[index][0], prefixesV6[index][1], testResults[index])

		bs.ATE.OTG().StartTraffic(t)
		time.Sleep(sleepTime * time.Second)
		bs.ATE.OTG().StopTraffic(t)
		otgutils.LogFlowMetrics(t, bs.ATE.OTG(), bs.ATETop)
		verifyTraffic(t, bs.ATE, int(cfgplugins.PortCount2), testResults[index])
	}
}

func TestMultipleStatementsBgpPolicy(t *testing.T) {
	bs := cfgplugins.NewBGPSession(t, cfgplugins.PortCount2, nil)
	bs.WithEBGP(t, []oc.E_BgpTypes_AFI_SAFI_TYPE{oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST, oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST}, []string{
		"port1", "port2"}, true, false)

	var communityMembers = [][][]int{
		{
			{10, 1},
		},
	}

	configureOTG(t, bs, prefixesV4, prefixesV6, communityMembers)
	bs.PushAndStart(t)

	t.Log("Verify DUT BGP sessions up")
	cfgplugins.VerifyDUTBGPEstablished(t, bs.DUT)
	t.Log("Verify OTG BGP sessions up")
	cfgplugins.VerifyOTGBGPEstablished(t, bs.ATE)

	ipv4 := bs.ATETop.Devices().Items()[1].Ethernets().Items()[0].Ipv4Addresses().Items()[0].Address()
	ipv6 := bs.ATETop.Devices().Items()[1].Ethernets().Items()[0].Ipv6Addresses().Items()[0].Address()

	verifyPolicy := []verifyPolicy{
		{
			desc:             "Testing with add_communities_if_missing",
			communitySetName: "add-communities",
			communityMatch:   [3]string{"40:1", "40:2"},
			matchSetOptions:  oc.BgpPolicy_MatchSetOptionsType_INVERT,
			testResults:      [6]bool{true, true, false, false, true, true},
		},
		{
			desc:             "Testing with match_comm_and_prefix_add_2_community_sets",
			communitySetName: "my_community",
			communityMatch:   [3]string{"50:1"},
			matchSetOptions:  oc.BgpPolicy_MatchSetOptionsType_ANY,
			testResults:      [6]bool{true, true, false, false, true, true},
		},
		{
			desc:             "Testing with match_aspath_set_med",
			communitySetName: "my_aspath",
			communityMatch:   [3]string{"50:1"},
			matchSetOptions:  oc.BgpPolicy_MatchSetOptionsType_ANY,
			testResults:      [6]bool{true, true, false, false, true, true},
		},
	}
	for _, vp := range verifyPolicy {

		configureMultiStatementsBgpPolicy(t, bs.DUT, ipv4, ipv6, vp.communitySetName, vp.communityMatch, vp.matchSetOptions)

		sleepTime := time.Duration(totalPackets/trafficPps) + 2

		for index, prefixPairV4 := range prefixesV4 {

			bs.ATETop.Flows().Clear()
			configureFlow(t, bs, prefixPairV4, "ipv4", index)
			configureFlow(t, bs, prefixesV6[index], "ipv6", index)
			bs.PushAndStartATE(t)

			t.Logf("Running traffic test for IPv4 prefixes: [%s, %s]. Expected Result: [%t]", prefixPairV4[0], prefixPairV4[1], vp.testResults[index])
			bs.ATE.OTG().StartTraffic(t)
			time.Sleep(sleepTime * time.Second)
			bs.ATE.OTG().StopTraffic(t)
			otgutils.LogFlowMetrics(t, bs.ATE.OTG(), bs.ATETop)
			verifyTraffic(t, bs.ATE, int(cfgplugins.PortCount2), vp.testResults[index])

			t.Logf("Running traffic test for IPv6 prefixes: [%s, %s]. Expected Result: [%t]", prefixesV6[index][0], prefixesV6[index][1], vp.testResults[index])

			bs.ATE.OTG().StartTraffic(t)
			time.Sleep(sleepTime * time.Second)
			bs.ATE.OTG().StopTraffic(t)
			otgutils.LogFlowMetrics(t, bs.ATE.OTG(), bs.ATETop)
			verifyTraffic(t, bs.ATE, int(cfgplugins.PortCount2), vp.testResults[index])
		}
	}
}
