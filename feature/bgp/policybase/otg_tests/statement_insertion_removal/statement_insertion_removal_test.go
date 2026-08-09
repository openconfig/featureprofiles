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

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	otg "github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	peerGrpName = "BGP-PEER-GROUP1"
	dutAS       = 65501
	ateAS       = 65502
	plenIPv4    = 30
	policyName  = "Test-Policy"
)

type communitySet struct {
	statement string
	name      string
	members   string
}

type bgpNeighbor struct {
	as      uint32
	nbrAddr string
	afiSafi oc.E_BgpTypes_AFI_SAFI_TYPE
	peerGrp string
}

// communitySetsMap is a map of configuration type to a slice of communitySet.
type communitySetsMap map[string][]communitySet

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
		as:      ateAS,
		peerGrp: peerGrpName}
	ebgpNbrs = []*bgpNeighbor{ebgp1NbrV4}

	communitySets = []communitySet{
		{
			// This is just a placeholder communitySet, do not use.
			statement: "Stmnt_0",
			name:      "Comm_100_0",
			members:   "100:0",
		},
		{
			statement: "Stmnt_1",
			name:      "Comm_100_1",
			members:   "100:1",
		},
		{
			statement: "Stmnt_2",
			name:      "Comm_100_2",
			members:   "100:2",
		},
		{
			statement: "Stmnt_3",
			name:      "Comm_100_3",
			members:   "100:3",
		},
		{
			statement: "Stmnt_4",
			name:      "Comm_100_4",
			members:   "100:4",
		},
		{
			statement: "Stmnt_5",
			name:      "Comm_100_5",
			members:   "100:5",
		},
		{
			statement: "Stmnt_6",
			name:      "Comm_100_6",
			members:   "100:6",
		},
		{
			statement: "Stmnt_7",
			name:      "Comm_100_7",
			members:   "100:7",
		},
		{
			statement: "Stmnt_8",
			name:      "Comm_100_8",
			members:   "100:8",
		},
		{
			statement: "Stmnt_9",
			name:      "Comm_100_9",
			members:   "100:9",
		},
		{
			statement: "Stmnt_10",
			name:      "Comm_100_10",
			members:   "100:10",
		},
		{
			statement: "Stmnt_11",
			name:      "Comm_100_11",
			members:   "100:11",
		},
	}

	communitySetsConfigurations = communitySetsMap{
		"initial": []communitySet{
			communitySets[1], communitySets[3], communitySets[5], communitySets[7], communitySets[9],
		},
		"insertion": []communitySet{
			communitySets[1], communitySets[2], communitySets[3], communitySets[5], communitySets[7],
			communitySets[9],
		},
		"removal": []communitySet{
			communitySets[1], communitySets[2], communitySets[3], communitySets[7], communitySets[9],
		},
		"re-insertion": []communitySet{
			communitySets[1], communitySets[2], communitySets[3], communitySets[4], communitySets[5],
			communitySets[6], communitySets[7], communitySets[8], communitySets[9], communitySets[10],
		},
		"edit": []communitySet{
			communitySets[11], communitySets[2], communitySets[3], communitySets[4], communitySets[5],
			communitySets[6], communitySets[7], communitySets[8], communitySets[9], communitySets[10],
		},
	}
)

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

func createRoutePolicy(t *testing.T, dut *ondatra.DUTDevice, communitySets []communitySet) *oc.RoutingPolicy {
	t.Helper()
	d := &oc.Root{}
	rp := d.GetOrCreateRoutingPolicy()
	pdef := rp.GetOrCreatePolicyDefinition(policyName)

	for idx := range communitySets {
		communitySet := &communitySets[idx]

		commSet := rp.GetOrCreateDefinedSets().GetOrCreateBgpDefinedSets().GetOrCreateCommunitySet(communitySet.name)
		commMemberUnion := []oc.RoutingPolicy_DefinedSets_BgpDefinedSets_CommunitySet_CommunityMember_Union{oc.UnionString(communitySet.members)}
		commSet.SetCommunityMember(commMemberUnion)

		stmt, _ := pdef.AppendNewStatement(communitySet.statement)
		matchCommunitySet := stmt.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet()
		matchCommunitySet.SetCommunitySet(communitySet.name)
		matchCommunitySet.SetMatchSetOptions(oc.RoutingPolicy_MatchSetOptionsType_ANY)
		stmt.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_NEXT_STATEMENT
	}

	stmtLast, _ := pdef.AppendNewStatement("Stmnt_Last")
	stmtLast.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE

	return rp
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

		if deviations.BgpCommunityTypeSliceInputUnsupported(dut) {
			pg.SetSendCommunity(oc.Bgp_CommunityType_STANDARD)
		} else if !deviations.SkipBgpSendCommunityType(dut) {
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
	iDut1Bgp4Peer.SetPeerAddress(iDut1Ipv4.Gateway()).SetAsNumber(ateAS).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
	iDut1Bgp4Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true)

	otg.PushConfig(t, config)
	otg.StartProtocols(t)
	return config
}

func isRoutingPolicyEqual(t *testing.T, dut *ondatra.DUTDevice, importPolicy string, rpWant *oc.RoutingPolicy) bool {

	pdefGot := gnmi.Get(t, dut, gnmi.OC().RoutingPolicy().PolicyDefinition(importPolicy).Config())
	pdefWant := rpWant.GetPolicyDefinition(policyName)

	if len(pdefGot.Statement.Keys()) != len(pdefWant.Statement.Keys()) {
		return false
	}
	for _, stmt := range pdefWant.Statement.Keys() {
		// Check if the community set name matches.
		got := pdefGot.Statement.Get(stmt).GetConditions().GetBgpConditions().GetMatchCommunitySet().GetCommunitySet()
		want := pdefWant.Statement.Get(stmt).GetConditions().GetBgpConditions().GetMatchCommunitySet().GetCommunitySet()
		if got != want {
			return false
		}
	}
	return true
}

// statement_insertion_removal_test tests the statement insertion and removal.
func TestStatementInsertionRemoval(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	otg := ate.OTG()
	configureDUT(t, dut)

	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	gnmi.Delete(t, dut, dutConfPath.Config())
	dutConf := bgpCreateNbr(dutAS, ateAS, dut)
	gnmi.Replace(t, dut, dutConfPath.Config(), dutConf)
	configureOTG(t, otg)
	verifyBgpState(t, dut)

	bgpPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")

	t.Run("initial_policy", func(t *testing.T) {
		rp := createRoutePolicy(t, dut, communitySetsConfigurations["initial"])
		gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)
		pathV4 := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp().PeerGroup(peerGrpName).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).ApplyPolicy()
		d := &oc.Root{}
		policyV4 := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut)).GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").GetOrCreateBgp().GetOrCreatePeerGroup(peerGrpName).GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).GetOrCreateApplyPolicy()
		policyV4.SetImportPolicy([]string{policyName})
		policyV4.SetExportPolicy([]string{policyName})
		gnmi.Update(t, dut, pathV4.Config(), policyV4)

		configuredPolicy := gnmi.Get(t, dut, bgpPath.Bgp().PeerGroup(peerGrpName).AfiSafi(ebgp1NbrV4.afiSafi).ApplyPolicy().State())
		if len(configuredPolicy.ImportPolicy) > 0 && !isRoutingPolicyEqual(t, dut, configuredPolicy.ImportPolicy[0], rp) {
			t.Errorf("isRoutingPolicyEqual(t, dut, importPolicy, rpWant):\n%v \nwant:\n%v", configuredPolicy, rp)
		}
	})
	t.Run("policy_statement_insertion", func(t *testing.T) {
		rp := createRoutePolicy(t, dut, communitySetsConfigurations["insertion"])
		gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)
		configuredPolicy := gnmi.Get(t, dut, bgpPath.Bgp().PeerGroup(peerGrpName).AfiSafi(ebgp1NbrV4.afiSafi).ApplyPolicy().State())
		if len(configuredPolicy.ImportPolicy) > 0 && !isRoutingPolicyEqual(t, dut, configuredPolicy.ImportPolicy[0], rp) {
			t.Errorf("isRoutingPolicyEqual(t, dut, importPolicy, rpWant):\n%v \nwant:\n%v", configuredPolicy, rp)
		}
	})
	t.Run("policy_statement_removal", func(t *testing.T) {
		rp := createRoutePolicy(t, dut, communitySetsConfigurations["removal"])
		gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)
		configuredPolicy := gnmi.Get(t, dut, bgpPath.Bgp().PeerGroup(peerGrpName).AfiSafi(ebgp1NbrV4.afiSafi).ApplyPolicy().State())
		if len(configuredPolicy.ImportPolicy) > 0 && !isRoutingPolicyEqual(t, dut, configuredPolicy.ImportPolicy[0], rp) {
			t.Errorf("isRoutingPolicyEqual(t, dut, importPolicy, rpWant):\n%v \nwant:\n%v", configuredPolicy, rp)
		}
	})
	t.Run("policy_statement_re-insertion", func(t *testing.T) {
		rp := createRoutePolicy(t, dut, communitySetsConfigurations["re-insertion"])
		gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)
		rp = createRoutePolicy(t, dut, communitySetsConfigurations["removal"])
		gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)
		configuredPolicy := gnmi.Get(t, dut, bgpPath.Bgp().PeerGroup(peerGrpName).AfiSafi(ebgp1NbrV4.afiSafi).ApplyPolicy().State())
		if len(configuredPolicy.ImportPolicy) > 0 && !isRoutingPolicyEqual(t, dut, configuredPolicy.ImportPolicy[0], rp) {
			t.Errorf("isRoutingPolicyEqual(t, dut, importPolicy, rpWant):\n%v \nwant:\n%v", configuredPolicy, rp)
		}
	})
	t.Run("edit_policy_statement", func(t *testing.T) {
		rp := createRoutePolicy(t, dut, communitySetsConfigurations["edit"])
		gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)
		configuredPolicy := gnmi.Get(t, dut, bgpPath.Bgp().PeerGroup(peerGrpName).AfiSafi(ebgp1NbrV4.afiSafi).ApplyPolicy().State())
		if len(configuredPolicy.ImportPolicy) > 0 && !isRoutingPolicyEqual(t, dut, configuredPolicy.ImportPolicy[0], rp) {
			t.Errorf("isRoutingPolicyEqual(t, dut, importPolicy, rpWant):\n%v \nwant:\n%v", configuredPolicy, rp)
		}
	})
}
