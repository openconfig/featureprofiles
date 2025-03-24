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

package statement_insertion_removal_test

import (
	"testing"
	"time"

	"google3/third_party/golang/ygot/ygot/ygot"
	"google3/third_party/open_traffic_generator/gosnappi/gosnappi"
	"google3/third_party/openconfig/featureprofiles/internal/attrs/attrs"
	"google3/third_party/openconfig/featureprofiles/internal/deviations/deviations"
	"google3/third_party/openconfig/featureprofiles/internal/fptest/fptest"
	"google3/third_party/openconfig/ondatra/gnmi/gnmi"
	"google3/third_party/openconfig/ondatra/gnmi/oc/oc"
	"google3/third_party/openconfig/ondatra/ondatra"
	otg "google3/third_party/openconfig/ondatra/otg/otg"
	"google3/third_party/openconfig/ygnmi/ygnmi/ygnmi"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	peerGrpName1 = "BGP-PEER-GROUP1"
	dutAS        = 65501
	ateAS1       = 65502
	plenIPv4     = 30
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "DUT to ATE Port1",
		IPv4:    "192.0.2.1",
		IPv4Len: plenIPv4,
	}
	atePort1 = attrs.Attributes{
		Name:    "atePort1",
		IPv4:    "192.0.2.2",
		IPv4Len: plenIPv4,
		MAC:     "02:00:01:01:01:01",
	}
	ebgp1NbrV4 = &bgpNeighbor{
		nbrAddr: atePort1.IPv4,
		afiSafi: oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST,
		as:      ateAS1,
		peerGrp: peerGrpName1}
	ebgpNbrs = []*bgpNeighbor{ebgp1NbrV4}

	communitySetsInitial = []communitySet{
		{
			name:    "Comm_100_1",
			members: "100:1",
		},
		{
			name:    "Comm_100_3",
			members: "100:3",
		},
		{
			name:    "Comm_100_5",
			members: "100:5",
		},
		{
			name:    "Comm_100_7",
			members: "100:7",
		},
		{
			name:    "Comm_100_9",
			members: "100:9",
		},
	}
	communitySetsInsertion = []communitySet{
		{
			name:    "Comm_100_1",
			members: "100:1",
		},
		{
			name:    "Comm_100_2",
			members: "100:2",
		},
		{
			name:    "Comm_100_3",
			members: "100:3",
		},
		{
			name:    "Comm_100_5",
			members: "100:5",
		},
		{
			name:    "Comm_100_7",
			members: "100:7",
		},
		{
			name:    "Comm_100_9",
			members: "100:9",
		},
	}
	communitySetsRemoval = []communitySet{
		{
			name:    "Comm_100_1",
			members: "100:1",
		},
		{
			name:    "Comm_100_2",
			members: "100:2",
		},
		{
			name:    "Comm_100_3",
			members: "100:3",
		},
		{
			name:    "Comm_100_7",
			members: "100:7",
		},
		{
			name:    "Comm_100_9",
			members: "100:9",
		},
	}
	communitySetsReInsertion = []communitySet{
		{
			name:    "Comm_100_1",
			members: "100:1",
		},
		{
			name:    "Comm_100_2",
			members: "100:2",
		},
		{
			name:    "Comm_100_3",
			members: "100:3",
		},
		{
			name:    "Comm_100_4",
			members: "100:4",
		},
		{
			name:    "Comm_100_5",
			members: "100:5",
		},
		{
			name:    "Comm_100_6",
			members: "100:6",
		},
		{
			name:    "Comm_100_7",
			members: "100:7",
		},
		{
			name:    "Comm_100_8",
			members: "100:8",
		},
		{
			name:    "Comm_100_9",
			members: "100:9",
		},
		{
			name:    "Comm_100_10",
			members: "100:10",
		},
	}
)

type communitySet struct {
	name    string
	members string
}

type bgpNeighbor struct {
	as      uint32
	nbrAddr string
	isV4    bool
	afiSafi oc.E_BgpTypes_AFI_SAFI_TYPE
	peerGrp string
}

func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	dc := gnmi.OC()
	i1 := dutPort1.NewOCInterface(dut.Port(t, "port1").Name(), dut)
	gnmi.Replace(t, dut, dc.Interface(i1.GetName()).Config(), i1)

	fptest.ConfigureDefaultNetworkInstance(t, dut)

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, dut.Port(t, "port1"))
	}
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, dut.Port(t, "port1").Name(), deviations.DefaultNetworkInstance(dut), 0)
	}
}

func createRoutePolicyInitial(t *testing.T, dut *ondatra.DUTDevice) *oc.RoutingPolicy {
	t.Helper()
	d := &oc.Root{}
	batchConfig := &gnmi.SetBatch{}
	var pdef *oc.RoutingPolicy_PolicyDefinition
	rp := d.GetOrCreateRoutingPolicy()
	pdef = rp.GetOrCreatePolicyDefinition("Test-Policy")
	for _, communitySet := range communitySetsInitial {
		commSet := rp.GetOrCreateDefinedSets().GetOrCreateBgpDefinedSets().GetOrCreateCommunitySet(communitySet.name)
		var commMemberUnion []oc.RoutingPolicy_DefinedSets_BgpDefinedSets_CommunitySet_CommunityMember_Union
		commMemberUnion = append(commMemberUnion, oc.UnionString(communitySet.members))
		commSet.SetCommunityMember(commMemberUnion)
	}

	stmt1, _ := pdef.AppendNewStatement("Stmnt_1")
	stmt1.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetCommunitySet("Comm_100_1")
	stmt1.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetMatchSetOptions(oc.RoutingPolicy_MatchSetOptionsType_ANY)
	stmt1.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_NEXT_STATEMENT
	stmt3, _ := pdef.AppendNewStatement("Stmnt_3")
	stmt3.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetCommunitySet("Comm_100_3")
	stmt3.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetMatchSetOptions(oc.RoutingPolicy_MatchSetOptionsType_ANY)
	stmt3.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_NEXT_STATEMENT
	stmt5, _ := pdef.AppendNewStatement("Stmnt_5")
	stmt5.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetCommunitySet("Comm_100_5")
	stmt5.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetMatchSetOptions(oc.RoutingPolicy_MatchSetOptionsType_ANY)
	stmt5.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_NEXT_STATEMENT
	stmt7, _ := pdef.AppendNewStatement("Stmnt_7")
	stmt7.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetCommunitySet("Comm_100_7")
	stmt7.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetMatchSetOptions(oc.RoutingPolicy_MatchSetOptionsType_ANY)
	stmt7.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_NEXT_STATEMENT
	stmt9, _ := pdef.AppendNewStatement("Stmnt_9")
	stmt9.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetCommunitySet("Comm_100_9")
	stmt9.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetMatchSetOptions(oc.RoutingPolicy_MatchSetOptionsType_ANY)
	stmt9.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_NEXT_STATEMENT
	gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)
	bgpPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	gnmi.BatchReplace(batchConfig, bgpPath.Neighbor(ebgp1NbrV4.nbrAddr).AfiSafi(ebgp1NbrV4.afiSafi).ApplyPolicy().ImportPolicy().Config(), []string{"Test-Policy"})
	gnmi.BatchReplace(batchConfig, bgpPath.Neighbor(ebgp1NbrV4.nbrAddr).AfiSafi(ebgp1NbrV4.afiSafi).ApplyPolicy().ExportPolicy().Config(), []string{"Test-Policy"})
	batchConfig.Set(t, dut)
	return rp
}

func createInsertionPolicy(t *testing.T, dut *ondatra.DUTDevice) *oc.RoutingPolicy {
	t.Helper()
	d := &oc.Root{}
	rpi := d.GetOrCreateRoutingPolicy()
	pdef := rpi.GetOrCreatePolicyDefinition("Test-Policy")
	for _, communitySet := range communitySetsInsertion {
		commSet := rpi.GetOrCreateDefinedSets().GetOrCreateBgpDefinedSets().GetOrCreateCommunitySet(communitySet.name)
		var commMemberUnion []oc.RoutingPolicy_DefinedSets_BgpDefinedSets_CommunitySet_CommunityMember_Union
		commMemberUnion = append(commMemberUnion, oc.UnionString(communitySet.members))
		commSet.SetCommunityMember(commMemberUnion)
	}
	stmt1, _ := pdef.AppendNewStatement("Stmnt_1")
	stmt1.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetCommunitySet("Comm_100_1")
	stmt1.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetMatchSetOptions(oc.RoutingPolicy_MatchSetOptionsType_ANY)
	stmt1.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_NEXT_STATEMENT
	stmt2, _ := pdef.AppendNewStatement("Stmnt_2")
	stmt2.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetCommunitySet("Comm_100_2")
	stmt2.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetMatchSetOptions(oc.RoutingPolicy_MatchSetOptionsType_ANY)
	stmt2.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_NEXT_STATEMENT
	stmt3, _ := pdef.AppendNewStatement("Stmnt_3")
	stmt3.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetCommunitySet("Comm_100_3")
	stmt3.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetMatchSetOptions(oc.RoutingPolicy_MatchSetOptionsType_ANY)
	stmt3.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_NEXT_STATEMENT
	stmt5, _ := pdef.AppendNewStatement("Stmnt_5")
	stmt5.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetCommunitySet("Comm_100_5")
	stmt5.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetMatchSetOptions(oc.RoutingPolicy_MatchSetOptionsType_ANY)
	stmt5.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_NEXT_STATEMENT
	stmt7, _ := pdef.AppendNewStatement("Stmnt_7")
	stmt7.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetCommunitySet("Comm_100_7")
	stmt7.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetMatchSetOptions(oc.RoutingPolicy_MatchSetOptionsType_ANY)
	stmt7.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_NEXT_STATEMENT
	stmt9, _ := pdef.AppendNewStatement("Stmnt_9")
	stmt9.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetCommunitySet("Comm_100_9")
	stmt9.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetMatchSetOptions(oc.RoutingPolicy_MatchSetOptionsType_ANY)
	stmt9.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_NEXT_STATEMENT
	return rpi
}

func createRemovalPolicy(t *testing.T, dut *ondatra.DUTDevice) *oc.RoutingPolicy {
	t.Helper()
	d := &oc.Root{}
	rpr := d.GetOrCreateRoutingPolicy()
	pdef := rpr.GetOrCreatePolicyDefinition("Test-Policy")
	for _, communitySet := range communitySetsRemoval {
		commSet := rpr.GetOrCreateDefinedSets().GetOrCreateBgpDefinedSets().GetOrCreateCommunitySet(communitySet.name)
		var commMemberUnion []oc.RoutingPolicy_DefinedSets_BgpDefinedSets_CommunitySet_CommunityMember_Union
		commMemberUnion = append(commMemberUnion, oc.UnionString(communitySet.members))
		commSet.SetCommunityMember(commMemberUnion)
	}
	stmt1, _ := pdef.AppendNewStatement("Stmnt_1")
	stmt1.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetCommunitySet("Comm_100_1")
	stmt1.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetMatchSetOptions(oc.RoutingPolicy_MatchSetOptionsType_ANY)
	stmt1.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_NEXT_STATEMENT
	stmt2, _ := pdef.AppendNewStatement("Stmnt_2")
	stmt2.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetCommunitySet("Comm_100_2")
	stmt2.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetMatchSetOptions(oc.RoutingPolicy_MatchSetOptionsType_ANY)
	stmt2.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_NEXT_STATEMENT
	stmt3, _ := pdef.AppendNewStatement("Stmnt_3")
	stmt3.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetCommunitySet("Comm_100_3")
	stmt3.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetMatchSetOptions(oc.RoutingPolicy_MatchSetOptionsType_ANY)
	stmt3.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_NEXT_STATEMENT
	stmt7, _ := pdef.AppendNewStatement("Stmnt_7")
	stmt7.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetCommunitySet("Comm_100_7")
	stmt7.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetMatchSetOptions(oc.RoutingPolicy_MatchSetOptionsType_ANY)
	stmt7.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_NEXT_STATEMENT
	stmt9, _ := pdef.AppendNewStatement("Stmnt_9")
	stmt9.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetCommunitySet("Comm_100_9")
	stmt9.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetMatchSetOptions(oc.RoutingPolicy_MatchSetOptionsType_ANY)
	stmt9.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_NEXT_STATEMENT
	return rpr
}
func createReInsertionPolicy(t *testing.T, dut *ondatra.DUTDevice) *oc.RoutingPolicy {
	t.Helper()
	d := &oc.Root{}
	rpre := d.GetOrCreateRoutingPolicy()
	pdef := rpre.GetOrCreatePolicyDefinition("Test-Policy")
	for _, communitySet := range communitySetsReInsertion {
		commSet := rpre.GetOrCreateDefinedSets().GetOrCreateBgpDefinedSets().GetOrCreateCommunitySet(communitySet.name)
		var commMemberUnion []oc.RoutingPolicy_DefinedSets_BgpDefinedSets_CommunitySet_CommunityMember_Union
		commMemberUnion = append(commMemberUnion, oc.UnionString(communitySet.members))
		commSet.SetCommunityMember(commMemberUnion)
	}
	stmt1, _ := pdef.AppendNewStatement("Stmnt_1")
	stmt1.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetCommunitySet("Comm_100_1")
	stmt1.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetMatchSetOptions(oc.RoutingPolicy_MatchSetOptionsType_ANY)
	stmt1.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_NEXT_STATEMENT
	stmt2, _ := pdef.AppendNewStatement("Stmnt_2")
	stmt2.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetCommunitySet("Comm_100_2")
	stmt2.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetMatchSetOptions(oc.RoutingPolicy_MatchSetOptionsType_ANY)
	stmt2.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_NEXT_STATEMENT
	stmt3, _ := pdef.AppendNewStatement("Stmnt_3")
	stmt3.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetCommunitySet("Comm_100_3")
	stmt3.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetMatchSetOptions(oc.RoutingPolicy_MatchSetOptionsType_ANY)
	stmt3.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_NEXT_STATEMENT
	stmt4, _ := pdef.AppendNewStatement("Stmnt_4")
	stmt4.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetCommunitySet("Comm_100_4")
	stmt4.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetMatchSetOptions(oc.RoutingPolicy_MatchSetOptionsType_ANY)
	stmt4.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_NEXT_STATEMENT
	stmt5, _ := pdef.AppendNewStatement("Stmnt_5")
	stmt5.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetCommunitySet("Comm_100_5")
	stmt5.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetMatchSetOptions(oc.RoutingPolicy_MatchSetOptionsType_ANY)
	stmt5.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_NEXT_STATEMENT
	stmt6, _ := pdef.AppendNewStatement("Stmnt_6")
	stmt6.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetCommunitySet("Comm_100_6")
	stmt6.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetMatchSetOptions(oc.RoutingPolicy_MatchSetOptionsType_ANY)
	stmt6.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_NEXT_STATEMENT
	stmt7, _ := pdef.AppendNewStatement("Stmnt_7")
	stmt7.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetCommunitySet("Comm_100_7")
	stmt7.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetMatchSetOptions(oc.RoutingPolicy_MatchSetOptionsType_ANY)
	stmt7.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_NEXT_STATEMENT
	stmt8, _ := pdef.AppendNewStatement("Stmnt_8")
	stmt8.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetCommunitySet("Comm_100_8")
	stmt8.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetMatchSetOptions(oc.RoutingPolicy_MatchSetOptionsType_ANY)
	stmt8.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_NEXT_STATEMENT
	stmt9, _ := pdef.AppendNewStatement("Stmnt_9")
	stmt9.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetCommunitySet("Comm_100_9")
	stmt9.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetMatchSetOptions(oc.RoutingPolicy_MatchSetOptionsType_ANY)
	stmt9.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_NEXT_STATEMENT
	stmt10, _ := pdef.AppendNewStatement("Stmnt_10")
	stmt10.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetCommunitySet("Comm_100_10")
	stmt10.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetMatchSetOptions(oc.RoutingPolicy_MatchSetOptionsType_ANY)
	stmt10.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_NEXT_STATEMENT
	stmtLast, _ := pdef.AppendNewStatement("Stmnt_Last")
	stmtLast.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
	return rpre
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

	for _, nbr := range ebgpNbrs {

		pg := bgp.GetOrCreatePeerGroup(nbr.peerGrp)
		pg.PeerAs = ygot.Uint32(nbr.as)
		pg.PeerGroupName = ygot.String(nbr.peerGrp)

		if !deviations.SkipBgpSendCommunityType(dut) {
			pg.SetSendCommunityType([]oc.E_Bgp_CommunityType{oc.Bgp_CommunityType_STANDARD})
		}

		as4 := pg.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
		as4.Enabled = ygot.Bool(true)

		bgpNbr := bgp.GetOrCreateNeighbor(nbr.nbrAddr)
		bgpNbr.PeerGroup = ygot.String(nbr.peerGrp)
		bgpNbr.PeerAs = ygot.Uint32(nbr.as)
		bgpNbr.Enabled = ygot.Bool(true)
		af4 := bgpNbr.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
		af4.Enabled = ygot.Bool(true)

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

func configureOTG(t *testing.T, otg *otg.OTG) gosnappi.Config {
	t.Helper()
	config := gosnappi.NewConfig()
	port1 := config.Ports().Add().SetName("port1")

	// Port1 Configuration.
	iDut1Dev := config.Devices().Add().SetName(atePort1.Name)
	iDut1Eth := iDut1Dev.Ethernets().Add().SetName(atePort1.Name + ".Eth").SetMac(atePort1.MAC)
	iDut1Eth.Connection().SetPortName(port1.Name())
	iDut1Ipv4 := iDut1Eth.Ipv4Addresses().Add().SetName(atePort1.Name + ".IPv4")
	iDut1Ipv4.SetAddress(atePort1.IPv4).SetGateway(dutPort1.IPv4).SetPrefix(uint32(atePort1.IPv4Len))

	// eBGP v4 session on Port1.
	iDut1Bgp := iDut1Dev.Bgp().SetRouterId(iDut1Ipv4.Address())
	iDut1Bgp4Peer := iDut1Bgp.Ipv4Interfaces().Add().SetIpv4Name(iDut1Ipv4.Name()).Peers().Add().SetName(atePort1.Name + ".BGP4.peer")
	iDut1Bgp4Peer.SetPeerAddress(iDut1Ipv4.Gateway()).SetAsNumber(ateAS1).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
	iDut1Bgp4Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true)

	otg.PushConfig(t, config)
	otg.StartProtocols(t)
	return config
}

type testCase struct {
	desc    string
	nbr     *bgpNeighbor
	peerGrp string
}

// statement_insertion_removal_test tests the statement insertion and removal.
func TestStatementInsertionRemoval(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	otg := ate.OTG()
	configureDUT(t, dut)

	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	gnmi.Delete(t, dut, dutConfPath.Config())
	dutConf := bgpCreateNbr(dutAS, ateAS1, dut)
	gnmi.Replace(t, dut, dutConfPath.Config(), dutConf)
	configureOTG(t, otg)
	verifyBgpState(t, dut)

	t.Run("Initial Policy", func(t *testing.T) {
		rp := createRoutePolicyInitial(t, dut)
		t.Logf("rp: %v", rp)
		//		gnmi.BatchReplace(batchConfig, bgpPath.PeerGroup(peerGrpgName).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).ApplyPolicy().ImportPolicy().Config(), []string{policyName})
	})

	//verfifyRoutingPolicy(t, dut, "test-policy-initial", ebgp1NbrV4, tc.peerGrp)

	t.Run("Policy statement insertion", func(t *testing.T) {
		rpi := createInsertionPolicy(t, dut)
		gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().Config(), rpi)
		//verfifyRoutingPolicy(t, dut, "test-policy-insertion", ebgp1NbrV4, tc.peerGrp)
	})
	t.Run("Policy statement removal", func(t *testing.T) {
		rpr := createRemovalPolicy(t, dut)
		gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().Config(), rpr)
		//verfifyRoutingPolicy(t, dut, "test-policy-removal", ebgp1NbrV4, tc.peerGrp)
	})
	t.Run("Policy statement re-insertion ", func(t *testing.T) {
		rpi := createReInsertionPolicy(t, dut)
		gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().Config(), rpi)
		//verfifyRoutingPolicy(t, dut, "test-policy-insertion", ebgp1NbrV4, tc.peerGrp)
		rpre := createRemovalPolicy(t, dut)
		gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().Config(), rpre)
		//verfify
	})
}

