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

package aspath_and_community_test

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
)

const (
	prefixV4Len  = 30
	prefixV6Len  = 126
	trafficPps   = 100
	totalPackets = 1200
	bgpName      = "BGP"
	ImpPolicy    = "routing-policy-ASPATH-COMMUNITY"
	RPLPermitAll = "PERMIT-ALL"
)

var prefixesV4 = [][]string{
	{"198.51.100.0", "198.51.100.4"},
	{"198.51.100.8", "198.51.100.12"},
	{"198.51.100.16", "198.51.100.20"},
	{"198.51.100.24", "198.51.100.28"},
	{"198.51.100.32", "198.51.100.40"},
}

var prefixesV6 = [][]string{
	{"2048:db1:64:64::0", "2048:db1:64:64::4"},
	{"2048:db1:64:64::8", "2048:db1:64:64::c"},
	{"2048:db1:64:64::10", "2048:db1:64:64::14"},
	{"2048:db1:64:64::18", "2048:db1:64:64::1c"},
	{"2048:db1:64:64::20", "2048:db1:64:64::24"},
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func configureImportBGPPolicy(t *testing.T, dut *ondatra.DUTDevice, ipv4 string, ipv6 string, aspathMatch []string, communityMatch string, aspathSetName string, communitySetName string, commMatchSetOptions oc.E_BgpPolicy_MatchSetOptionsType, aspMatchSetOptions oc.E_RoutingPolicy_MatchSetOptionsType) {
	root := &oc.Root{}
	rp := root.GetOrCreateRoutingPolicy()
	pdef1 := rp.GetOrCreatePolicyDefinition(ImpPolicy)
	stmt1, err := pdef1.AppendNewStatement("match_as_and_community")
	if err != nil {
		t.Fatalf("AppendNewStatement(%s) failed: %v", "any_my_aspath", err)
	}
	if !deviations.SkipSettingStatementForPolicy(dut) {
		stmt1.GetOrCreateActions().SetPolicyResult(oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)
	}

	aspathSet := rp.GetOrCreateDefinedSets().GetOrCreateBgpDefinedSets().GetOrCreateAsPathSet(aspathSetName)
	aspathSet.SetAsPathSetMember(aspathMatch)
	stmt1.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchAsPathSet().SetAsPathSet(aspathSetName)
	stmt1.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchAsPathSet().SetMatchSetOptions(aspMatchSetOptions)
	pdAllow := rp.GetOrCreatePolicyDefinition(RPLPermitAll)
	st, err := pdAllow.AppendNewStatement("id-1")
	if err != nil {
		t.Fatalf("AppendNewStatement(%s) failed: %v", "routePolicyStatement", err)
	}
	st.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
	gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)

	// stmt2, err := pdef1.AppendNewStatement("match_comms")
	// if err != nil {
	// 	t.Fatalf("AppendNewStatement(%s) failed: %v", "routePolicyStatement", err)
	// }
	if !deviations.SkipSettingStatementForPolicy(dut) {
		stmt1.GetOrCreateActions().SetPolicyResult(oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)
	}

	if !(deviations.CommunityMemberRegexUnsupported(dut) && communitySetName == "any_my_3_comms") {
		communitySet := rp.GetOrCreateDefinedSets().GetOrCreateBgpDefinedSets().GetOrCreateCommunitySet(communitySetName)
		cs := []oc.RoutingPolicy_DefinedSets_BgpDefinedSets_CommunitySet_CommunityMember_Union{}
		if communityMatch != "" {
			cs = append(cs, oc.UnionString(communityMatch))
		}
		communitySet.SetCommunityMember(cs)
		communitySet.SetMatchSetOptions(commMatchSetOptions)
	}

	var communitySetCLIConfig string
	if deviations.CommunityMemberRegexUnsupported(dut) && communitySetName == "any_my_3_comms" {
		switch dut.Vendor() {
		case ondatra.CISCO:
			communitySetCLIConfig = fmt.Sprintf("community-set %v\n ios-regex '(10[0-9]:1)'\n end-set", communitySetName)
		default:
			t.Fatalf("Unsupported vendor %s for deviation 'CommunityMemberRegexUnsupported'", dut.Vendor())
		}
		helpers.GnmiCLIConfig(t, dut, communitySetCLIConfig)
	}

	if deviations.BGPConditionsMatchCommunitySetUnsupported(dut) {
		stmt1.GetOrCreateConditions().GetOrCreateBgpConditions().SetCommunitySet(communitySetName)
	} else {
		stmt1.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetCommunitySet(communitySetName)
	}

	if deviations.CommunityMemberRegexUnsupported(dut) && communitySetName == "any_my_3_comms" {
		gnmi.Update(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)
	} else {
		gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)
	}

	dni := deviations.DefaultNetworkInstance(dut)
	pathV6 := gnmi.OC().NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).Bgp().Neighbor(ipv6).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).ApplyPolicy()
	policyV6 := root.GetOrCreateNetworkInstance(dni).GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).GetOrCreateBgp().GetOrCreateNeighbor(ipv6).GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).GetOrCreateApplyPolicy()
	if !deviations.DefaultRoutePolicyUnsupported(dut) {
		policyV6.SetDefaultImportPolicy(oc.RoutingPolicy_DefaultPolicyType_REJECT_ROUTE)
	}
	policyV6.SetImportPolicy([]string{ImpPolicy})
	gnmi.Replace(t, dut, pathV6.Config(), policyV6)

	pathV4 := gnmi.OC().NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).Bgp().Neighbor(ipv4).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).ApplyPolicy()
	policyV4 := root.GetOrCreateNetworkInstance(dni).GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).GetOrCreateBgp().GetOrCreateNeighbor(ipv4).GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).GetOrCreateApplyPolicy()
	if !deviations.DefaultRoutePolicyUnsupported(dut) {
		policyV4.SetDefaultImportPolicy(oc.RoutingPolicy_DefaultPolicyType_REJECT_ROUTE)
	}
	policyV4.SetImportPolicy([]string{ImpPolicy})
	gnmi.Replace(t, dut, pathV4.Config(), policyV4)
}

func configureOTG(t *testing.T, bs *cfgplugins.BGPSession, prefixesV4 [][]string, prefixesV6 [][]string) {
	var communityMembers = [][][]int{
		{
			{100, 1}, {200, 2}, {300, 3},
		},
		{
			{100, 1},
		},
		{
			{109, 1},
		},
		{
			{200, 1},
		},
		{
			{100, 1},
		},
	}

	var aspathMembers = [][]uint32{
		{100, 200, 300},
		{100, 400, 300},
		{109},
		{200},
		{300},
	}

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

		asp4 := bgp4PeerRoute.AsPath().Segments().Add()
		asp4.SetAsNumbers(aspathMembers[index])

		bgp6PeerRoute := bgp6Peer.V6Routes().Add()
		bgp6PeerRoute.SetName(bs.ATEPorts[1].Name + ".BGP6.peer.dut." + strconv.Itoa(index))
		bgp6PeerRoute.SetNextHopIpv6Address(ipv6.Address())

		route6Address1 := bgp6PeerRoute.Addresses().Add().SetAddress(prefixesV6[index][0])
		route6Address1.SetPrefix(prefixV6Len)
		route6Address2 := bgp6PeerRoute.Addresses().Add().SetAddress(prefixesV6[index][1])
		route6Address2.SetPrefix(prefixV6Len)

		asp6 := bgp6PeerRoute.AsPath().Segments().Add()
		asp6.SetAsNumbers(aspathMembers[index])

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

	flow := bs.ATETop.Flows().Add().SetName("flow" + prefixType + strconv.Itoa(index))
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

func verifyTraffic(t *testing.T, ate *ondatra.ATEDevice, prefixType string, testResults bool, index int) {
	recvMetric := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow("flow"+prefixType+strconv.Itoa(index)).State())
	framesTx := recvMetric.GetCounters().GetOutPkts()
	framesRx := recvMetric.GetCounters().GetInPkts()

	if framesTx == 0 {
		t.Error("No traffic was generated and frames transmitted were 0")
	} else if (testResults && framesRx == framesTx) || (!testResults && framesRx == 0) {
		t.Logf("Traffic validation successful for criteria [%t] FramesTx: %d FramesRx: %d", testResults, framesTx, framesRx)
	} else {
		t.Errorf("Traffic validation failed for criteria [%t] FramesTx: %d FramesRx: %d", testResults, framesTx, framesRx)
	}
}

func TestCommunitySet(t *testing.T) {
	testResults := [5]bool{true, true, true, false, false}

	bs := cfgplugins.NewBGPSession(t, cfgplugins.PortCount2, nil)
	bs.WithEBGP(t, []oc.E_BgpTypes_AFI_SAFI_TYPE{oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST, oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST}, []string{"port2"}, true, true)

	configureOTG(t, bs, prefixesV4, prefixesV6)
	bs.PushAndStart(t)

	t.Log("Verify DUT BGP sessions up")
	cfgplugins.VerifyDUTBGPEstablished(t, bs.DUT)
	t.Log("Verify OTG BGP sessions up")
	cfgplugins.VerifyOTGBGPEstablished(t, bs.ATE)

	ipv4 := bs.ATETop.Devices().Items()[1].Ethernets().Items()[0].Ipv4Addresses().Items()[0].Address()
	ipv6 := bs.ATETop.Devices().Items()[1].Ethernets().Items()[0].Ipv6Addresses().Items()[0].Address()
	aspathMatch := []string{"(10[0-9]|200)"}
	communityMatch := "(10[0-9]:1)"
	communitySetName := "any_my_3_comms"
	aspathSetName := "any_my_aspath"
	commMatchSetOptions := oc.BgpPolicy_MatchSetOptionsType_ANY
	aspMatchSetOptions := oc.RoutingPolicy_MatchSetOptionsType_ANY
	configureImportBGPPolicy(t, bs.DUT, ipv4, ipv6, aspathMatch, communityMatch, aspathSetName, communitySetName, commMatchSetOptions, aspMatchSetOptions)
	sleepTime := time.Duration(totalPackets/trafficPps) + 5

	for index, prefixPairV4 := range prefixesV4 {
		bs.ATETop.Flows().Clear()
		t.Logf("Running traffic test for IPv4 prefixes: [%s, %s]. Expected Result: [%t]", prefixPairV4[0], prefixPairV4[1], testResults[index])
		configureFlow(t, bs, prefixPairV4, "ipv4", index)
		bs.PushAndStartATE(t)
		time.Sleep(sleepTime * time.Second)
		bs.ATE.OTG().StartTraffic(t)
		time.Sleep(sleepTime * time.Second)
		bs.ATE.OTG().StopTraffic(t)
		otgutils.LogFlowMetrics(t, bs.ATE.OTG(), bs.ATETop)
		verifyTraffic(t, bs.ATE, "ipv4", testResults[index], index)

		bs.ATETop.Flows().Clear()
		t.Logf("Running traffic test for IPv6 prefixes: [%s, %s]. Expected Result: [%t]", prefixesV6[index][0], prefixesV6[index][1], testResults[index])
		configureFlow(t, bs, prefixesV6[index], "ipv6", index)
		bs.PushAndStartATE(t)
		time.Sleep(sleepTime * time.Second)
		bs.ATE.OTG().StartTraffic(t)
		time.Sleep(sleepTime * time.Second)
		bs.ATE.OTG().StopTraffic(t)
		otgutils.LogFlowMetrics(t, bs.ATE.OTG(), bs.ATETop)
		verifyTraffic(t, bs.ATE, "ipv6", testResults[index], index)
	}
}
