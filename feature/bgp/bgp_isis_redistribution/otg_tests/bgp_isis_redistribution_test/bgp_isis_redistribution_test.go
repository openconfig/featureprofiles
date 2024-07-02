// Copyright 2024 Google LLC
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

package bgp_isis_redistribution_test

import (
	"fmt"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/featureprofiles/internal/isissession"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	otgtelemetry "github.com/openconfig/ondatra/gnmi/otg"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	bgpName                         = "BGP"
	maskLenExact                    = "exact"
	dummyAS                         = uint32(64655)
	dutAS                           = uint32(64656)
	ateAS                           = uint32(64657)
	v4Route                         = "203.10.113.0"
	v4TrafficStart                  = "203.10.113.1"
	v4DummyRoute                    = "192.51.100.0"
	v4RoutePrefix                   = uint32(24)
	v6Route                         = "2001:db8:128:128:0:0:0:0"
	v6TrafficStart                  = "2001:db8:128:128:0:0:0:1"
	v6DummyRoute                    = "2001:db8:128:129:0:0:0:0"
	v6RoutePrefix                   = uint32(64)
	v4RoutePolicy                   = "route-policy-v4"
	v4Statement                     = "statement-v4"
	v4PrefixSet                     = "prefix-set-v4"
	v4FlowName                      = "flow-v4"
	v4CommunitySet                  = "community-set-v4"
	v6RoutePolicy                   = "route-policy-v6"
	v6Statement                     = "statement-v6"
	v6PrefixSet                     = "prefix-set-v6"
	v6FlowName                      = "flow-v6"
	v6CommunitySet                  = "community-set-v6"
	peerGrpNamev4                   = "BGP-PEER-GROUP-V4"
	peerGrpNamev6                   = "BGP-PEER-GROUP-V6"
	allowAllPolicy                  = "ALLOWAll"
	tablePolicyMatchCommunitySetTag = "TablePolicyMatchCommunitySetTag"
	matchTagRedistributionPolicy    = "MatchTagRedistributionPolicy"
	nonMatchingCommunityVal         = "64655:200"
	matchingCommunityVal            = "64657:100"
	routeTagVal                     = 10000
)

var (
	advertisedIPv4    ipAddr = ipAddr{address: v4Route, prefix: v4RoutePrefix}
	advertisedIPv6    ipAddr = ipAddr{address: v6Route, prefix: v6RoutePrefix}
	nonAdvertisedIPv4 ipAddr = ipAddr{address: v4DummyRoute, prefix: v4RoutePrefix}
	nonAdvertisedIPv6 ipAddr = ipAddr{address: v6DummyRoute, prefix: v6RoutePrefix}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

type ipAddr struct {
	address string
	prefix  uint32
}

func (ip *ipAddr) cidr(t *testing.T) string {
	_, net, err := net.ParseCIDR(fmt.Sprintf("%s/%d", ip.address, ip.prefix))
	if err != nil {
		t.Fatal(err)
	}
	return net.String()
}

type testCase struct {
	name                string
	desc                string
	applyPolicyFunc     func(t *testing.T, dut *ondatra.DUTDevice)
	verifyTelemetryFunc func(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice)
	testTraffic         bool
	ipv4                bool
}

func TestBGPToISISRedistribution(t *testing.T) {
	ts := isissession.MustNew(t).WithISIS()
	t.Run("ISIS Setup", func(t *testing.T) {
		ts.PushAndStart(t)
		ts.MustAdjacency(t)
	})
	configureRoutePolicyAllow(t, ts.DUT, allowAllPolicy, oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)
	setupEBGPAndAdvertise(t, ts)
	t.Run("BGP Setup", func(t *testing.T) {
		t.Log("Verify DUT BGP sessions up")
		cfgplugins.VerifyDUTBGPEstablished(t, ts.DUT)

		t.Log("Verify OTG BGP sessions up")
		cfgplugins.VerifyOTGBGPEstablished(t, ts.ATE)
	})

	testCases := []testCase{
		{
			name:                "NonMatchingPrefix",
			desc:                "Non matching IPv4 BGP prefixes in a prefix-set should not be redistributed to IS-IS",
			applyPolicyFunc:     nonMatchingPrefixRoutePolicy,
			verifyTelemetryFunc: verifyNonMatchingPrefixTelemetry,
			testTraffic:         false,
			ipv4:                true,
		},
		{
			name:                "MatchingPrefix",
			desc:                "Matching IPv4 BGP prefixes in a prefix-set should be redistributed to IS-IS",
			applyPolicyFunc:     matchingPrefixRoutePolicy,
			verifyTelemetryFunc: verifyMatchingPrefixTelemetry,
			testTraffic:         true,
			ipv4:                true,
		},
		{
			name:                "NonMatchingCommunity",
			desc:                "IPv4: Non matching BGP community in a community-set should not be redistributed to IS-IS",
			applyPolicyFunc:     nonMatchingCommunityRoutePolicy,
			verifyTelemetryFunc: verifyNonMatchingCommunityTelemetry,
			testTraffic:         false,
			ipv4:                true,
		},
		{
			name:                "MatchingCommunity",
			desc:                "IPv4: Matching BGP community in a community-set should be redistributed to IS-IS",
			applyPolicyFunc:     matchingCommunityRoutePolicy,
			verifyTelemetryFunc: verifyMatchingCommunityTelemetry,
			testTraffic:         true,
			ipv4:                true,
		},
		{
			name:                "NonMatchingPrefixV6",
			desc:                "Non matching IPv6 BGP prefixes in a prefix-set should not be redistributed to IS-IS",
			applyPolicyFunc:     nonMatchingPrefixRoutePolicyV6,
			verifyTelemetryFunc: verifyNonMatchingPrefixTelemetryV6,
			testTraffic:         false,
			ipv4:                false,
		},
		{
			name:                "MatchingPrefixV6",
			desc:                "Matching IPv6 BGP prefixes in a prefix-set should be redistributed to IS-IS",
			applyPolicyFunc:     matchingPrefixRoutePolicyV6,
			verifyTelemetryFunc: verifyMatchingPrefixTelemetryV6,
			testTraffic:         true,
			ipv4:                false,
		},
		{
			name:                "NonMatchingCommunityV6",
			desc:                "IPv6: Non matching BGP community in a community-set should not be redistributed to IS-IS",
			applyPolicyFunc:     nonMatchingCommunityRoutePolicyV6,
			verifyTelemetryFunc: verifyNonMatchingCommunityTelemetryV6,
			testTraffic:         false,
			ipv4:                false,
		},
		{
			name:                "MatchingCommunityV6",
			desc:                "IPv6: Matching BGP community in a community-set should be redistributed to IS-IS",
			applyPolicyFunc:     matchingCommunityRoutePolicyV6,
			verifyTelemetryFunc: verifyMatchingCommunityTelemetryV6,
			testTraffic:         true,
			ipv4:                false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Description: %s", tc.desc)
			tc.applyPolicyFunc(t, ts.DUT)
			tc.verifyTelemetryFunc(t, ts.DUT, ts.ATE)
			if tc.testTraffic {
				if tc.ipv4 {
					createFlow(t, ts)
					checkTraffic(t, ts, v4FlowName)
				} else {
					createFlowV6(t, ts)
					checkTraffic(t, ts, v6FlowName)
				}
			}
		})
	}
}

// setupEBGPAndAdvertise setups eBGP on DUT port1 and ATE port1
func setupEBGPAndAdvertise(t *testing.T, ts *isissession.TestSession) {
	t.Helper()
	dut := ondatra.DUT(t, "dut")
	// setup eBGP on DUT port2
	root := &oc.Root{}
	dni := root.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(ts.DUT))
	dni.SetType(oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)

	bgpP := dni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName)
	bgpP.SetEnabled(true)
	bgp := bgpP.GetOrCreateBgp()

	g := bgp.GetOrCreateGlobal()
	g.SetAs(dutAS)
	g.SetRouterId(isissession.DUTTrafficAttrs.IPv4)
	g.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
	g.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)

	pgv4 := bgp.GetOrCreatePeerGroup(peerGrpNamev4)
	pgv4.PeerGroupName = ygot.String(peerGrpNamev4)
	pgv6 := bgp.GetOrCreatePeerGroup(peerGrpNamev6)
	pgv6.PeerGroupName = ygot.String(peerGrpNamev6)
	if deviations.RoutePolicyUnderAFIUnsupported(dut) {
		rpl := pgv4.GetOrCreateApplyPolicy()
		rpl.SetExportPolicy([]string{allowAllPolicy})
		rpl.SetImportPolicy([]string{allowAllPolicy})
		rplv6 := pgv6.GetOrCreateApplyPolicy()
		rplv6.SetExportPolicy([]string{"ALLOW"})
		rplv6.SetImportPolicy([]string{"ALLOW"})
	} else {
		pg1af4 := pgv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
		pg1af4.Enabled = ygot.Bool(true)
		pg1rpl4 := pg1af4.GetOrCreateApplyPolicy()
		pg1rpl4.SetExportPolicy([]string{allowAllPolicy})
		pg1rpl4.SetImportPolicy([]string{allowAllPolicy})
		pg1af6 := pgv6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
		pg1af6.Enabled = ygot.Bool(true)
		pg1rpl6 := pg1af6.GetOrCreateApplyPolicy()
		pg1rpl6.SetExportPolicy([]string{allowAllPolicy})
		pg1rpl6.SetImportPolicy([]string{allowAllPolicy})
	}

	nV4 := bgp.GetOrCreateNeighbor(isissession.ATETrafficAttrs.IPv4)
	nV4.SetPeerAs(ateAS)
	nV4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
	nV4.PeerGroup = ygot.String(peerGrpNamev4)

	nV6 := bgp.GetOrCreateNeighbor(isissession.ATETrafficAttrs.IPv6)
	nV6.SetPeerAs(ateAS)
	nV6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)
	nV6.PeerGroup = ygot.String(peerGrpNamev6)
	gnmi.Update(t, ts.DUT, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(ts.DUT)).Config(), dni)

	// setup eBGP on ATE port2
	dev2BGP := ts.ATEIntf2.Bgp().SetRouterId(isissession.ATETrafficAttrs.IPv4)

	ipv4 := ts.ATEIntf2.Ethernets().Items()[0].Ipv4Addresses().Items()[0]
	bgp4Peer := dev2BGP.Ipv4Interfaces().Add().SetIpv4Name(ipv4.Name()).Peers().Add().SetName(ts.ATEIntf2.Name() + ".BGP4.peer")
	bgp4Peer.SetPeerAddress(isissession.DUTTrafficAttrs.IPv4).SetAsNumber(ateAS).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)

	ipv6 := ts.ATEIntf2.Ethernets().Items()[0].Ipv6Addresses().Items()[0]
	bgp6Peer := dev2BGP.Ipv6Interfaces().Add().SetIpv6Name(ipv6.Name()).Peers().Add().SetName(ts.ATEIntf2.Name() + ".BGP6.peer")
	bgp6Peer.SetPeerAddress(isissession.DUTTrafficAttrs.IPv6).SetAsNumber(ateAS).SetAsType(gosnappi.BgpV6PeerAsType.EBGP)

	// configure emulated IPv4 and IPv6 networks
	netv4 := bgp4Peer.V4Routes().Add().SetName("v4-bgpNet-dev1")
	netv4.Addresses().Add().SetAddress(advertisedIPv4.address).SetPrefix(advertisedIPv4.prefix)
	commv4 := netv4.Communities().Add()
	commv4.SetType(gosnappi.BgpCommunityType.MANUAL_AS_NUMBER)
	commv4.SetAsNumber(ateAS)
	commv4.SetAsCustom(100)

	netv6 := bgp6Peer.V6Routes().Add().SetName("v6-bgpNet-dev1")
	netv6.Addresses().Add().SetAddress(advertisedIPv6.address).SetPrefix(advertisedIPv6.prefix)
	commv6 := netv6.Communities().Add()
	commv6.SetType(gosnappi.BgpCommunityType.MANUAL_AS_NUMBER)
	commv6.SetAsNumber(ateAS)
	commv6.SetAsCustom(100)

	ts.ATE.OTG().PushConfig(t, ts.ATETop)
	ts.ATE.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, ts.ATE.OTG(), ts.ATETop, "IPv4")
	otgutils.WaitForARP(t, ts.ATE.OTG(), ts.ATETop, "IPv6")
}

func nonMatchingPrefixRoutePolicy(t *testing.T, dut *ondatra.DUTDevice) {
	root := &oc.Root{}
	rp := root.GetOrCreateRoutingPolicy()
	pdef := rp.GetOrCreatePolicyDefinition(v4RoutePolicy)
	stmt, err := pdef.AppendNewStatement(v4Statement)
	if err != nil {
		t.Fatalf("AppendNewStatement(%s) failed: %v", v4Statement, err)
	}
	stmt.GetOrCreateActions().SetPolicyResult(oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)
	if !deviations.SkipIsisSetLevel(dut) {
		stmt.GetOrCreateActions().GetOrCreateIsisActions().SetSetLevel(2)
	}
	if !deviations.SkipIsisSetMetricStyleType(dut) {
		stmt.GetOrCreateActions().GetOrCreateIsisActions().SetSetMetricStyleType(oc.IsisPolicy_MetricStyle_WIDE_METRIC)
	}

	prefixSet := rp.GetOrCreateDefinedSets().GetOrCreatePrefixSet(v4PrefixSet)
	prefixSet.SetMode(oc.PrefixSet_Mode_IPV4)
	prefixSet.GetOrCreatePrefix(nonAdvertisedIPv4.cidr(t), maskLenExact)

	if !deviations.SkipSetRpMatchSetOptions(dut) {
		stmt.GetOrCreateConditions().GetOrCreateMatchPrefixSet().SetMatchSetOptions(oc.RoutingPolicy_MatchSetOptionsRestrictedType_ANY)
	}
	stmt.GetOrCreateConditions().GetOrCreateMatchPrefixSet().SetPrefixSet(v4PrefixSet)
	gnmi.Update(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)

	// enable bgp isis redistribution
	bgpISISRedistribution(t, dut)
}

func matchingPrefixRoutePolicy(t *testing.T, dut *ondatra.DUTDevice) {
	root := &oc.Root{}
	rp := root.GetOrCreateRoutingPolicy()
	prefixSet := rp.GetOrCreateDefinedSets().GetOrCreatePrefixSet(v4PrefixSet)
	prefixSet.GetOrCreatePrefix(advertisedIPv4.cidr(t), maskLenExact)
	gnmi.Update(t, dut, gnmi.OC().RoutingPolicy().DefinedSets().PrefixSet(v4PrefixSet).Config(), prefixSet)
}

func nonMatchingCommunityRoutePolicy(t *testing.T, dut *ondatra.DUTDevice) {
	if deviations.CommunityMatchWithRedistributionUnsupported(dut) {
		configureBGPTablePolicyWithSetTag(t, v4PrefixSet, advertisedIPv4.cidr(t), v4CommunitySet, dummyAS, 200, true)
		bgpISISRedistributionWithRouteTagPolicy(t, dut, oc.Types_ADDRESS_FAMILY_IPV4)
	} else {
		root := &oc.Root{}
		rp := root.GetOrCreateRoutingPolicy()
		pdef := rp.GetOrCreatePolicyDefinition(v4RoutePolicy)
		stmt, err := pdef.AppendNewStatement(v4Statement)
		if err != nil {
			t.Fatalf("AppendNewStatement(%s) failed: %v", v4Statement, err)
		}
		stmt.GetOrCreateActions().SetPolicyResult(oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)
		if !deviations.SkipIsisSetLevel(dut) {
			stmt.GetOrCreateActions().GetOrCreateIsisActions().SetSetLevel(2)
		}
		if !deviations.SkipIsisSetMetricStyleType(dut) {
			stmt.GetOrCreateActions().GetOrCreateIsisActions().SetSetMetricStyleType(oc.IsisPolicy_MetricStyle_WIDE_METRIC)
		}

		communitySet := rp.GetOrCreateDefinedSets().GetOrCreateBgpDefinedSets().GetOrCreateCommunitySet(v4CommunitySet)
		communitySet.SetCommunityMember([]oc.RoutingPolicy_DefinedSets_BgpDefinedSets_CommunitySet_CommunityMember_Union{oc.UnionString(fmt.Sprintf("%d:%d", dummyAS, 200))})
		communitySet.SetMatchSetOptions(oc.BgpPolicy_MatchSetOptionsType_ANY)

		if deviations.BGPConditionsMatchCommunitySetUnsupported(dut) {
			stmt.GetOrCreateConditions().GetOrCreateBgpConditions().SetCommunitySet(v4CommunitySet)
		} else {
			stmt.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetCommunitySet(v4CommunitySet)
		}
		gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)
	}
}

func matchingCommunityRoutePolicy(t *testing.T, dut *ondatra.DUTDevice) {
	if deviations.CommunityMatchWithRedistributionUnsupported(dut) {
		configureBGPTablePolicyWithSetTag(t, v4PrefixSet, advertisedIPv4.cidr(t), v4CommunitySet, ateAS, 100, true)
		bgpISISRedistributionWithRouteTagPolicy(t, dut, oc.Types_ADDRESS_FAMILY_IPV4)
	} else {
		root := &oc.Root{}
		rp := root.GetOrCreateRoutingPolicy()
		communitySet := rp.GetOrCreateDefinedSets().GetOrCreateBgpDefinedSets().GetOrCreateCommunitySet(v4CommunitySet)
		communitySet.SetCommunityMember([]oc.RoutingPolicy_DefinedSets_BgpDefinedSets_CommunitySet_CommunityMember_Union{oc.UnionString(fmt.Sprintf("%d:%d", ateAS, 100))})
		gnmi.Update(t, dut, gnmi.OC().RoutingPolicy().DefinedSets().BgpDefinedSets().CommunitySet(v4CommunitySet).Config(), communitySet)
	}
}

func verifyNonMatchingPrefixTelemetry(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice) {
	rPolicy := gnmi.Get[*oc.RoutingPolicy](t, dut, gnmi.OC().RoutingPolicy().State())

	rPolicyDef := rPolicy.GetPolicyDefinition(v4RoutePolicy)
	if rpName := rPolicyDef.GetName(); rpName != v4RoutePolicy {
		t.Errorf("Routing policy name: %s, want: %s", rpName, v4RoutePolicy)
	}
	if stmtName := rPolicyDef.GetStatement(v4Statement).GetName(); stmtName != v4Statement {
		t.Errorf("Routing policy statement name: %s, want: %s", stmtName, v4Statement)
	}
	if polResult := rPolicyDef.GetStatement(v4Statement).GetActions().GetPolicyResult(); polResult != oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE {
		t.Errorf("Routing policy statement result: %s, want: %s", polResult, oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)
	}
	if !deviations.SkipIsisSetLevel(dut) {
		if isisLevel := rPolicyDef.GetStatement(v4Statement).GetActions().GetIsisActions().GetSetLevel(); isisLevel != 2 {
			t.Errorf("IS-IS level: %d, want: %d", isisLevel, 2)
		}
	}

	prefixSet := rPolicy.GetDefinedSets().GetPrefixSet(v4PrefixSet)
	if pName := prefixSet.GetName(); pName != v4PrefixSet {
		t.Errorf("Prefix set name: %s, want: %s", pName, v4PrefixSet)
	}
	if pMode := prefixSet.GetMode(); pMode != oc.PrefixSet_Mode_IPV4 {
		t.Errorf("Prefix set mode: %s, want: %s", pMode, oc.PrefixSet_Mode_IPV4)
	}
	if prefix := prefixSet.GetPrefix(nonAdvertisedIPv4.cidr(t), maskLenExact); prefix == nil {
		t.Errorf("Prefix is nil, want: %s", nonAdvertisedIPv4.cidr(t))
	}

	stmt := rPolicyDef.GetStatement(v4Statement)
	if matchSetOpts := stmt.GetConditions().GetMatchPrefixSet().GetMatchSetOptions(); matchSetOpts != oc.RoutingPolicy_MatchSetOptionsRestrictedType_ANY {
		t.Errorf("Match prefix set options: %s, want: %s", matchSetOpts, oc.RoutingPolicy_MatchSetOptionsRestrictedType_ANY)
	}
	if prefixSet := stmt.GetConditions().GetMatchPrefixSet().GetPrefixSet(); prefixSet != v4PrefixSet {
		t.Errorf("Match prefix set prefix set: %s, want: %s", prefixSet, v4PrefixSet)
	}

	tableConn := gnmi.Get[*oc.NetworkInstance_TableConnection](t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).TableConnection(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, oc.Types_ADDRESS_FAMILY_IPV4).State())
	if tableConn == nil {
		t.Errorf("Table connection is nil, want non-nil")
	}
	if metricProp := tableConn.GetDisableMetricPropagation(); metricProp != false {
		t.Errorf("Metric propagation: %t, want: %t", metricProp, false)
	}
	if defaultImportPolicy := tableConn.GetDefaultImportPolicy(); defaultImportPolicy != oc.RoutingPolicy_DefaultPolicyType_REJECT_ROUTE {
		t.Errorf("Default import policy: %s, want: %s", defaultImportPolicy, oc.RoutingPolicy_DefaultPolicyType_REJECT_ROUTE)
	}
	if importPolicy := tableConn.GetImportPolicy(); len(importPolicy) == 0 || !containsValue(importPolicy, v4RoutePolicy) {
		t.Errorf("Import policy: %v, want: %s", importPolicy, []string{v4RoutePolicy})
	}

	_, ok := gnmi.WatchAll(t, ate.OTG(), gnmi.OTG().IsisRouter("devIsis").LinkStateDatabase().LspsAny().Tlvs().ExtendedIpv4Reachability().Prefix(advertisedIPv4.address).State(), 30*time.Second, func(v *ygnmi.Value[*otgtelemetry.IsisRouter_LinkStateDatabase_Lsps_Tlvs_ExtendedIpv4Reachability_Prefix]) bool {
		prefix, present := v.Val()
		return present && prefix.GetPrefix() == advertisedIPv4.address
	}).Await(t)
	if ok {
		t.Errorf("Prefix found, not want: %s", advertisedIPv4.address)
	}
}

func verifyMatchingPrefixTelemetry(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice) {
	rPolicy := gnmi.Get[*oc.RoutingPolicy](t, dut, gnmi.OC().RoutingPolicy().State())
	pfxSet := rPolicy.GetDefinedSets().GetPrefixSet(v4PrefixSet)
	if pName := pfxSet.GetName(); pName != v4PrefixSet {
		t.Errorf("Prefix set name: %s, want: %s", pName, v4PrefixSet)
	}
	if prefix := pfxSet.GetPrefix(advertisedIPv4.cidr(t), maskLenExact); prefix == nil {
		t.Errorf("Prefix is nil, want: %s", advertisedIPv4.cidr(t))
	}

	_, ok := gnmi.WatchAll(t, ate.OTG(), gnmi.OTG().IsisRouter("devIsis").LinkStateDatabase().LspsAny().Tlvs().ExtendedIpv4Reachability().Prefix(advertisedIPv4.address).State(), 30*time.Second, func(v *ygnmi.Value[*otgtelemetry.IsisRouter_LinkStateDatabase_Lsps_Tlvs_ExtendedIpv4Reachability_Prefix]) bool {
		prefix, present := v.Val()
		return present && prefix.GetPrefix() == advertisedIPv4.address
	}).Await(t)
	if !ok {
		t.Errorf("Prefix not found, want: %s", advertisedIPv4.address)
	}
}

func verifyNonMatchingCommunityTelemetry(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice) {
	commSet := gnmi.Get[*oc.RoutingPolicy_DefinedSets_BgpDefinedSets_CommunitySet](t, dut, gnmi.OC().RoutingPolicy().DefinedSets().BgpDefinedSets().CommunitySet(v4CommunitySet).State())
	if commSet == nil {
		t.Errorf("Community set is nil, want non-nil")
	}
	if deviations.BgpCommunityMemberIsAString(dut) {
		cm := nonMatchingCommunityVal
		if commSetMember := commSet.GetCommunityMember(); len(commSetMember) == 0 || !containsValue(commSetMember, oc.RoutingPolicy_DefinedSets_BgpDefinedSets_CommunitySet_CommunityMember_Union(oc.UnionString(cm))) {
			t.Errorf("Community set member: %v, want: %s", commSetMember, cm)
		}
	} else {
		cm, _ := strconv.ParseInt(fmt.Sprintf("%04x%04x", dummyAS, 200), 16, 0)
		if commSetMember := commSet.GetCommunityMember(); len(commSetMember) == 0 || !containsValue(commSetMember, oc.RoutingPolicy_DefinedSets_BgpDefinedSets_CommunitySet_CommunityMember_Union(oc.UnionUint32(cm))) {
			t.Errorf("Community set member: %v, want: %d", commSetMember, cm)
		}
	}

	_, ok := gnmi.WatchAll(t, ate.OTG(), gnmi.OTG().IsisRouter("devIsis").LinkStateDatabase().LspsAny().Tlvs().ExtendedIpv4Reachability().Prefix(advertisedIPv4.address).State(), 30*time.Second, func(v *ygnmi.Value[*otgtelemetry.IsisRouter_LinkStateDatabase_Lsps_Tlvs_ExtendedIpv4Reachability_Prefix]) bool {
		prefix, present := v.Val()
		return present && prefix.GetPrefix() == advertisedIPv4.address
	}).Await(t)
	if ok {
		t.Errorf("Prefix found, not want: %s", advertisedIPv4.address)
	}
}

func verifyMatchingCommunityTelemetry(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice) {
	commSet := gnmi.Get[*oc.RoutingPolicy_DefinedSets_BgpDefinedSets_CommunitySet](t, dut, gnmi.OC().RoutingPolicy().DefinedSets().BgpDefinedSets().CommunitySet(v4CommunitySet).State())
	if commSet == nil {
		t.Errorf("Community set is nil, want non-nil")
	}
	if deviations.BgpCommunityMemberIsAString(dut) {
		cm := matchingCommunityVal
		if commSetMember := commSet.GetCommunityMember(); len(commSetMember) == 0 || !containsValue(commSetMember, oc.RoutingPolicy_DefinedSets_BgpDefinedSets_CommunitySet_CommunityMember_Union(oc.UnionString(cm))) {
			t.Errorf("Community set member: %v, want: %v", commSetMember, cm)
		}
	} else {
		cm, _ := strconv.ParseInt(fmt.Sprintf("%04x%04x", ateAS, 100), 16, 0)
		if commSetMember := commSet.GetCommunityMember(); len(commSetMember) == 0 || !containsValue(commSetMember, oc.RoutingPolicy_DefinedSets_BgpDefinedSets_CommunitySet_CommunityMember_Union(oc.UnionUint32(cm))) {
			t.Errorf("Community set member: %v, want: %v", commSetMember, cm)
		}
	}

	_, ok := gnmi.WatchAll(t, ate.OTG(), gnmi.OTG().IsisRouter("devIsis").LinkStateDatabase().LspsAny().Tlvs().ExtendedIpv4Reachability().Prefix(advertisedIPv4.address).State(), 30*time.Second, func(v *ygnmi.Value[*otgtelemetry.IsisRouter_LinkStateDatabase_Lsps_Tlvs_ExtendedIpv4Reachability_Prefix]) bool {
		prefix, present := v.Val()
		return present && prefix.GetPrefix() == advertisedIPv4.address
	}).Await(t)
	if !ok {
		t.Errorf("Prefix not found, want: %s", advertisedIPv4.address)
	}
}

func nonMatchingPrefixRoutePolicyV6(t *testing.T, dut *ondatra.DUTDevice) {
	root := &oc.Root{}
	rp := root.GetOrCreateRoutingPolicy()
	pdef := rp.GetOrCreatePolicyDefinition(v6RoutePolicy)
	stmt, err := pdef.AppendNewStatement(v6Statement)
	if err != nil {
		t.Fatalf("AppendNewStatement(%s) failed: %v", v6Statement, err)
	}
	stmt.GetOrCreateActions().SetPolicyResult(oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)
	if !deviations.SkipIsisSetLevel(dut) {
		stmt.GetOrCreateActions().GetOrCreateIsisActions().SetSetLevel(2)
	}
	if !deviations.SkipIsisSetMetricStyleType(dut) {
		stmt.GetOrCreateActions().GetOrCreateIsisActions().SetSetMetricStyleType(oc.IsisPolicy_MetricStyle_WIDE_METRIC)
	}

	prefixSet := rp.GetOrCreateDefinedSets().GetOrCreatePrefixSet(v6PrefixSet)
	prefixSet.SetMode(oc.PrefixSet_Mode_IPV6)
	prefixSet.GetOrCreatePrefix(nonAdvertisedIPv6.cidr(t), maskLenExact)

	if !deviations.SkipSetRpMatchSetOptions(dut) {
		stmt.GetOrCreateConditions().GetOrCreateMatchPrefixSet().SetMatchSetOptions(oc.RoutingPolicy_MatchSetOptionsRestrictedType_ANY)
	}
	stmt.GetOrCreateConditions().GetOrCreateMatchPrefixSet().SetPrefixSet(v6PrefixSet)
	gnmi.Update(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)

	// enable bgp isis redistribution
	bgpISISRedistributionV6(t, dut)
}

func matchingPrefixRoutePolicyV6(t *testing.T, dut *ondatra.DUTDevice) {
	root := &oc.Root{}
	rp := root.GetOrCreateRoutingPolicy()
	prefixSet := rp.GetOrCreateDefinedSets().GetOrCreatePrefixSet(v6PrefixSet)
	prefixSet.GetOrCreatePrefix(advertisedIPv6.cidr(t), maskLenExact)
	gnmi.Update(t, dut, gnmi.OC().RoutingPolicy().DefinedSets().PrefixSet(v6PrefixSet).Config(), prefixSet)
}

func nonMatchingCommunityRoutePolicyV6(t *testing.T, dut *ondatra.DUTDevice) {
	if deviations.CommunityMatchWithRedistributionUnsupported(dut) {
		configureBGPTablePolicyWithSetTag(t, v6PrefixSet, advertisedIPv6.cidr(t), v6CommunitySet, dummyAS, 200, false)
		bgpISISRedistributionWithRouteTagPolicy(t, dut, oc.Types_ADDRESS_FAMILY_IPV6)
	} else {
		root := &oc.Root{}
		rp := root.GetOrCreateRoutingPolicy()
		pdef := rp.GetOrCreatePolicyDefinition(v6RoutePolicy)
		stmt, err := pdef.AppendNewStatement(v6Statement)
		if err != nil {
			t.Fatalf("AppendNewStatement(%s) failed: %v", v6Statement, err)
		}
		stmt.GetOrCreateActions().SetPolicyResult(oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)
		if !deviations.SkipIsisSetLevel(dut) {
			stmt.GetOrCreateActions().GetOrCreateIsisActions().SetSetLevel(2)
		}
		if !deviations.SkipIsisSetMetricStyleType(dut) {
			stmt.GetOrCreateActions().GetOrCreateIsisActions().SetSetMetricStyleType(oc.IsisPolicy_MetricStyle_WIDE_METRIC)
		}

		communitySet := rp.GetOrCreateDefinedSets().GetOrCreateBgpDefinedSets().GetOrCreateCommunitySet(v6CommunitySet)
		communitySet.SetCommunityMember([]oc.RoutingPolicy_DefinedSets_BgpDefinedSets_CommunitySet_CommunityMember_Union{oc.UnionString(fmt.Sprintf("%d:%d", dummyAS, 200))})
		communitySet.SetMatchSetOptions(oc.BgpPolicy_MatchSetOptionsType_ANY)

		if deviations.BGPConditionsMatchCommunitySetUnsupported(dut) {
			stmt.GetOrCreateConditions().GetOrCreateBgpConditions().SetCommunitySet(v6CommunitySet)
		} else {
			stmt.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetCommunitySet(v6CommunitySet)
		}
		gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)
	}
}

func matchingCommunityRoutePolicyV6(t *testing.T, dut *ondatra.DUTDevice) {
	if deviations.CommunityMatchWithRedistributionUnsupported(dut) {
		configureBGPTablePolicyWithSetTag(t, v6PrefixSet, advertisedIPv6.cidr(t), v6CommunitySet, ateAS, 100, false)
		bgpISISRedistributionWithRouteTagPolicy(t, dut, oc.Types_ADDRESS_FAMILY_IPV6)
	} else {
		root := &oc.Root{}
		rp := root.GetOrCreateRoutingPolicy()
		communitySet := rp.GetOrCreateDefinedSets().GetOrCreateBgpDefinedSets().GetOrCreateCommunitySet(v6CommunitySet)
		communitySet.SetCommunityMember([]oc.RoutingPolicy_DefinedSets_BgpDefinedSets_CommunitySet_CommunityMember_Union{oc.UnionString(fmt.Sprintf("%d:%d", ateAS, 100))})
		gnmi.Update(t, dut, gnmi.OC().RoutingPolicy().DefinedSets().BgpDefinedSets().CommunitySet(v6CommunitySet).Config(), communitySet)
	}
}

func verifyNonMatchingPrefixTelemetryV6(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice) {
	rPolicy := gnmi.Get[*oc.RoutingPolicy](t, dut, gnmi.OC().RoutingPolicy().State())

	rPolicyDef := rPolicy.GetPolicyDefinition(v6RoutePolicy)
	if rpName := rPolicyDef.GetName(); rpName != v6RoutePolicy {
		t.Errorf("Routing policy name: %s, want: %s", rpName, v6RoutePolicy)
	}
	if stmtName := rPolicyDef.GetStatement(v6Statement).GetName(); stmtName != v6Statement {
		t.Errorf("Routing policy statement name: %s, want: %s", stmtName, v6Statement)
	}
	if polResult := rPolicyDef.GetStatement(v6Statement).GetActions().GetPolicyResult(); polResult != oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE {
		t.Errorf("Routing policy statement result: %s, want: %s", polResult, oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)
	}
	if !deviations.SkipIsisSetLevel(dut) {
		if isisLevel := rPolicyDef.GetStatement(v6Statement).GetActions().GetIsisActions().GetSetLevel(); isisLevel != 2 {
			t.Errorf("IS-IS level: %d, want: %d", isisLevel, 2)
		}
	}

	prefixSet := rPolicy.GetDefinedSets().GetPrefixSet(v6PrefixSet)
	if pName := prefixSet.GetName(); pName != v6PrefixSet {
		t.Errorf("Prefix set name: %s, want: %s", pName, v6PrefixSet)
	}
	if pMode := prefixSet.GetMode(); pMode != oc.PrefixSet_Mode_IPV6 {
		t.Errorf("Prefix set mode: %s, want: %s", pMode, oc.PrefixSet_Mode_IPV6)
	}
	if prefix := prefixSet.GetPrefix(nonAdvertisedIPv6.cidr(t), maskLenExact); prefix == nil {
		t.Errorf("Prefix is nil, want: %s", nonAdvertisedIPv6.cidr(t))
	}

	stmt := rPolicyDef.GetStatement(v6Statement)
	if matchSetOpts := stmt.GetConditions().GetMatchPrefixSet().GetMatchSetOptions(); matchSetOpts != oc.RoutingPolicy_MatchSetOptionsRestrictedType_ANY {
		t.Errorf("Match prefix set options: %s, want: %s", matchSetOpts, oc.RoutingPolicy_MatchSetOptionsRestrictedType_ANY)
	}
	if prefixSet := stmt.GetConditions().GetMatchPrefixSet().GetPrefixSet(); prefixSet != v6PrefixSet {
		t.Errorf("Match prefix set prefix set: %s, want: %s", prefixSet, v6PrefixSet)
	}

	tableConn := gnmi.Get[*oc.NetworkInstance_TableConnection](t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).TableConnection(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, oc.Types_ADDRESS_FAMILY_IPV6).State())
	if tableConn == nil {
		t.Errorf("Table connection is nil, want non-nil")
	}
	if metricProp := tableConn.GetDisableMetricPropagation(); metricProp != false {
		t.Errorf("Metric propagation: %t, want: %t", metricProp, false)
	}
	if defaultImportPolicy := tableConn.GetDefaultImportPolicy(); defaultImportPolicy != oc.RoutingPolicy_DefaultPolicyType_REJECT_ROUTE {
		t.Errorf("Default import policy: %s, want: %s", defaultImportPolicy, oc.RoutingPolicy_DefaultPolicyType_REJECT_ROUTE)
	}
	if importPolicy := tableConn.GetImportPolicy(); len(importPolicy) == 0 || !containsValue(importPolicy, v6RoutePolicy) {
		t.Errorf("Import policy: %v, want: %s", importPolicy, []string{v6RoutePolicy})
	}

	_, ok := gnmi.WatchAll(t, ate.OTG(), gnmi.OTG().IsisRouter("devIsis").LinkStateDatabase().LspsAny().Tlvs().Ipv6Reachability().Prefix(advertisedIPv6.address).State(), 60*time.Second, func(v *ygnmi.Value[*otgtelemetry.IsisRouter_LinkStateDatabase_Lsps_Tlvs_Ipv6Reachability_Prefix]) bool {
		prefix, present := v.Val()
		return present && prefix.GetPrefix() == advertisedIPv6.address
	}).Await(t)
	if ok {
		t.Errorf("Prefix found, not want: %s", advertisedIPv6.address)
	}
}

func verifyMatchingPrefixTelemetryV6(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice) {
	rPolicy := gnmi.Get[*oc.RoutingPolicy](t, dut, gnmi.OC().RoutingPolicy().State())
	pfxSet := rPolicy.GetDefinedSets().GetPrefixSet(v6PrefixSet)
	if pName := pfxSet.GetName(); pName != v6PrefixSet {
		t.Errorf("Prefix set name: %s, want: %s", pName, v6PrefixSet)
	}
	if prefix := pfxSet.GetPrefix(advertisedIPv6.cidr(t), maskLenExact); prefix == nil {
		t.Errorf("Prefix is nil, want: %s", advertisedIPv6.cidr(t))
	}

	_, ok := gnmi.WatchAll(t, ate.OTG(), gnmi.OTG().IsisRouter("devIsis").LinkStateDatabase().LspsAny().Tlvs().Ipv6Reachability().Prefix(advertisedIPv6.address).State(), 60*time.Second, func(v *ygnmi.Value[*otgtelemetry.IsisRouter_LinkStateDatabase_Lsps_Tlvs_Ipv6Reachability_Prefix]) bool {
		prefix, present := v.Val()
		return present && prefix.GetPrefix() == advertisedIPv6.address
	}).Await(t)
	if !ok {
		t.Errorf("Prefix not found, want: %s", advertisedIPv6.address)
	}
}

func verifyNonMatchingCommunityTelemetryV6(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice) {
	commSet := gnmi.Get[*oc.RoutingPolicy_DefinedSets_BgpDefinedSets_CommunitySet](t, dut, gnmi.OC().RoutingPolicy().DefinedSets().BgpDefinedSets().CommunitySet(v6CommunitySet).State())
	if commSet == nil {
		t.Errorf("Community set is nil, want non-nil")
	}
	if deviations.BgpCommunityMemberIsAString(dut) {
		cm := nonMatchingCommunityVal
		if commSetMember := commSet.GetCommunityMember(); len(commSetMember) == 0 || !containsValue(commSetMember, oc.RoutingPolicy_DefinedSets_BgpDefinedSets_CommunitySet_CommunityMember_Union(oc.UnionString(cm))) {
			t.Errorf("Community set member: %v, want: %v", commSetMember, cm)
		}
	} else {
		cm, _ := strconv.ParseInt(fmt.Sprintf("%04x%04x", dummyAS, 200), 16, 0)
		if commSetMember := commSet.GetCommunityMember(); len(commSetMember) == 0 || !containsValue(commSetMember, oc.RoutingPolicy_DefinedSets_BgpDefinedSets_CommunitySet_CommunityMember_Union(oc.UnionUint32(cm))) {
			t.Errorf("Community set member: %v, want: %d", commSetMember, cm)
		}
	}

	_, ok := gnmi.WatchAll(t, ate.OTG(), gnmi.OTG().IsisRouter("devIsis").LinkStateDatabase().LspsAny().Tlvs().Ipv6Reachability().Prefix(advertisedIPv6.address).State(), 60*time.Second, func(v *ygnmi.Value[*otgtelemetry.IsisRouter_LinkStateDatabase_Lsps_Tlvs_Ipv6Reachability_Prefix]) bool {
		prefix, present := v.Val()
		return present && prefix.GetPrefix() == advertisedIPv6.address
	}).Await(t)
	if ok {
		t.Errorf("Prefix found, not want: %s", advertisedIPv6.address)
	}
}

func verifyMatchingCommunityTelemetryV6(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice) {
	commSet := gnmi.Get[*oc.RoutingPolicy_DefinedSets_BgpDefinedSets_CommunitySet](t, dut, gnmi.OC().RoutingPolicy().DefinedSets().BgpDefinedSets().CommunitySet(v6CommunitySet).State())
	if commSet == nil {
		t.Errorf("Community set is nil, want non-nil")
	}
	if deviations.BgpCommunityMemberIsAString(dut) {
		cm := matchingCommunityVal
		if commSetMember := commSet.GetCommunityMember(); len(commSetMember) == 0 || !containsValue(commSetMember, oc.RoutingPolicy_DefinedSets_BgpDefinedSets_CommunitySet_CommunityMember_Union(oc.UnionString(cm))) {
			t.Errorf("Community set member: %v, want: %v", commSetMember, cm)
		}
	} else {
		cm, _ := strconv.ParseInt(fmt.Sprintf("%04x%04x", ateAS, 100), 16, 0)
		if commSetMember := commSet.GetCommunityMember(); len(commSetMember) == 0 || !containsValue(commSetMember, oc.RoutingPolicy_DefinedSets_BgpDefinedSets_CommunitySet_CommunityMember_Union(oc.UnionUint32(cm))) {
			t.Errorf("Community set member: %v, want: %v", commSetMember, cm)
		}
	}

	_, ok := gnmi.WatchAll(t, ate.OTG(), gnmi.OTG().IsisRouter("devIsis").LinkStateDatabase().LspsAny().Tlvs().Ipv6Reachability().Prefix(advertisedIPv6.address).State(), 60*time.Second, func(v *ygnmi.Value[*otgtelemetry.IsisRouter_LinkStateDatabase_Lsps_Tlvs_Ipv6Reachability_Prefix]) bool {
		prefix, present := v.Val()
		return present && prefix.GetPrefix() == advertisedIPv6.address
	}).Await(t)
	if !ok {
		t.Errorf("Prefix not found, want: %s", advertisedIPv6.address)
	}
}

func bgpISISRedistribution(t *testing.T, dut *ondatra.DUTDevice) {
	dni := deviations.DefaultNetworkInstance(dut)
	root := &oc.Root{}
	tableConn := root.GetOrCreateNetworkInstance(dni).GetOrCreateTableConnection(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, oc.Types_ADDRESS_FAMILY_IPV4)
	if !deviations.SkipSettingDisableMetricPropagation(dut) {
		tableConn.SetDisableMetricPropagation(false)
	}
	if !deviations.DefaultRoutePolicyUnsupported(dut) {
		tableConn.SetDefaultImportPolicy(oc.RoutingPolicy_DefaultPolicyType_REJECT_ROUTE)
	}
	tableConn.SetImportPolicy([]string{v4RoutePolicy})
	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(dni).TableConnection(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, oc.Types_ADDRESS_FAMILY_IPV4).Config(), tableConn)
}

func bgpISISRedistributionV6(t *testing.T, dut *ondatra.DUTDevice) {
	dni := deviations.DefaultNetworkInstance(dut)
	root := &oc.Root{}
	tableConn := root.GetOrCreateNetworkInstance(dni).GetOrCreateTableConnection(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, oc.Types_ADDRESS_FAMILY_IPV6)
	if !deviations.SkipSettingDisableMetricPropagation(dut) {
		tableConn.SetDisableMetricPropagation(false)
	}
	if !deviations.DefaultRoutePolicyUnsupported(dut) {
		tableConn.SetDefaultImportPolicy(oc.RoutingPolicy_DefaultPolicyType_REJECT_ROUTE)
	}
	tableConn.SetImportPolicy([]string{v6RoutePolicy})
	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(dni).TableConnection(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, oc.Types_ADDRESS_FAMILY_IPV6).Config(), tableConn)
}
func bgpISISRedistributionWithRouteTagPolicy(t *testing.T, dut *ondatra.DUTDevice, afi oc.E_Types_ADDRESS_FAMILY) {
	dni := deviations.DefaultNetworkInstance(dut)
	root := &oc.Root{}
	tableConn := root.GetOrCreateNetworkInstance(dni).GetOrCreateTableConnection(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, afi)
	if !deviations.SkipSettingDisableMetricPropagation(dut) {
		tableConn.SetDisableMetricPropagation(false)
	}
	tableConn.SetImportPolicy([]string{matchTagRedistributionPolicy})
	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(dni).TableConnection(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, afi).Config(), tableConn)
}

func configureBGPTablePolicyWithSetTag(t *testing.T, prefixSetName, prefixSetAddress, communitySetName string, commAS, commValue uint32, v4Nbr bool) {
	dut := ondatra.DUT(t, "dut")
	root := &oc.Root{}
	rp := root.GetOrCreateRoutingPolicy()
	//BGP Table-policy to match community & prefix and set the route-Tag
	pdef1 := rp.GetOrCreatePolicyDefinition(tablePolicyMatchCommunitySetTag)
	stmt1, err := pdef1.AppendNewStatement("SetTag")
	if err != nil {
		t.Fatalf("AppendNewStatement(%s) failed: %v", "routePolicyStatement", err)
	}
	//Create prefix-set
	prefixSet := rp.GetOrCreateDefinedSets().GetOrCreatePrefixSet(prefixSetName)
	prefixSet.GetOrCreatePrefix(prefixSetAddress, maskLenExact)
	gnmi.Update(t, dut, gnmi.OC().RoutingPolicy().DefinedSets().PrefixSet(prefixSetName).Config(), prefixSet)
	//Create community-set
	communitySet := rp.GetOrCreateDefinedSets().GetOrCreateBgpDefinedSets().GetOrCreateCommunitySet(communitySetName)
	communitySet.SetCommunityMember([]oc.RoutingPolicy_DefinedSets_BgpDefinedSets_CommunitySet_CommunityMember_Union{oc.UnionString(fmt.Sprintf("%d:%d", commAS, commValue))})
	communitySet.SetMatchSetOptions(oc.BgpPolicy_MatchSetOptionsType_ANY)

	stmt1.GetOrCreateConditions().GetOrCreateMatchPrefixSet().SetPrefixSet(prefixSetName)
	stmt1.GetOrCreateConditions().GetOrCreateBgpConditions().SetCommunitySet(communitySetName)
	stmt1.GetOrCreateActions().GetOrCreateSetTag().SetMode(oc.SetTag_Mode_INLINE)
	stmt1.GetOrCreateActions().GetOrCreateSetTag().GetOrCreateInline().SetTag([]oc.RoutingPolicy_PolicyDefinition_Statement_Actions_SetTag_Inline_Tag_Union{oc.UnionUint32(routeTagVal)})
	stmt1.GetOrCreateActions().SetPolicyResult(oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)

	//Create tag-set with above route tag value
	tagSet := rp.GetOrCreateDefinedSets().GetOrCreateTagSet("RouteTagForRedistribution")
	tagSet.SetName("RouteTagForRedistribution")
	tagSet.SetTagValue([]oc.RoutingPolicy_DefinedSets_TagSet_TagValue_Union{oc.UnionUint32(routeTagVal)})

	//Route-policy to match tag and accept
	pdef2 := rp.GetOrCreatePolicyDefinition("MatchTagRedistributionPolicy")
	stmt2, err := pdef2.AppendNewStatement("matchTag")
	stmt2.GetOrCreateConditions().GetOrCreateMatchPrefixSet().SetPrefixSet(prefixSetName)
	stmt2.GetOrCreateConditions().GetOrCreateMatchTagSet().SetTagSet("RouteTagForRedistribution")
	stmt2.GetOrCreateActions().SetPolicyResult(oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)

	if err != nil {
		t.Fatalf("AppendNewStatement(%s) failed: %v", "routePolicyStatement", err)
	}

	gnmi.Update(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)
	var bgpTablePolicyCLI string
	if v4Nbr {
		bgpTablePolicyCLI = fmt.Sprintf("router bgp %v instance BGP address-family ipv4 unicast \n table-policy %v", dutAS, tablePolicyMatchCommunitySetTag)
		helpers.GnmiCLIConfig(t, dut, bgpTablePolicyCLI)
	} else {
		bgpTablePolicyCLI = fmt.Sprintf("router bgp %v instance BGP address-family ipv6 unicast \n table-policy %v", dutAS, tablePolicyMatchCommunitySetTag)
		helpers.GnmiCLIConfig(t, dut, bgpTablePolicyCLI)
	}
}

func configureRoutePolicyAllow(t *testing.T, dut *ondatra.DUTDevice, name string, pr oc.E_RoutingPolicy_PolicyResultType) {
	d := &oc.Root{}
	rp := d.GetOrCreateRoutingPolicy()
	pd := rp.GetOrCreatePolicyDefinition(name)
	st, err := pd.AppendNewStatement("id-1")
	if err != nil {
		t.Fatal(err)
	}
	st.GetOrCreateActions().PolicyResult = pr
	gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)
}

func createFlow(t *testing.T, ts *isissession.TestSession) {
	ts.ATETop.Flows().Clear()
	srcIpv4 := ts.ATEIntf1.Ethernets().Items()[0].Ipv4Addresses().Items()[0]

	t.Log("Configuring v4 traffic flow ")
	v4Flow := ts.ATETop.Flows().Add().SetName(v4FlowName)
	v4Flow.Metrics().SetEnable(true)
	v4Flow.TxRx().Device().
		SetTxNames([]string{srcIpv4.Name()}).
		SetRxNames([]string{"v4-bgpNet-dev1"})
	v4Flow.Size().SetFixed(512)
	v4Flow.Rate().SetPps(100)
	v4Flow.Duration().Continuous()
	e1 := v4Flow.Packet().Add().Ethernet()
	e1.Src().SetValue(isissession.ATEISISAttrs.MAC)
	v4 := v4Flow.Packet().Add().Ipv4()
	v4.Src().SetValue(isissession.ATEISISAttrs.IPv4)
	v4.Dst().Increment().SetStart(v4TrafficStart).SetCount(1)

	ts.ATE.OTG().PushConfig(t, ts.ATETop)
	ts.ATE.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, ts.ATE.OTG(), ts.ATETop, "IPv4")
	cfgplugins.VerifyDUTBGPEstablished(t, ts.DUT)
}

func createFlowV6(t *testing.T, ts *isissession.TestSession) {
	ts.ATETop.Flows().Clear()
	srcIpv6 := ts.ATEIntf1.Ethernets().Items()[0].Ipv6Addresses().Items()[0]

	t.Log("Configuring v6 traffic flow ")
	v6Flow := ts.ATETop.Flows().Add().SetName(v6FlowName)
	v6Flow.Metrics().SetEnable(true)
	v6Flow.TxRx().Device().
		SetTxNames([]string{srcIpv6.Name()}).
		SetRxNames([]string{"v6-bgpNet-dev1"})
	v6Flow.Size().SetFixed(512)
	v6Flow.Rate().SetPps(100)
	v6Flow.Duration().Continuous()
	e1 := v6Flow.Packet().Add().Ethernet()
	e1.Src().SetValue(isissession.ATEISISAttrs.MAC)
	v6 := v6Flow.Packet().Add().Ipv6()
	v6.Src().SetValue(isissession.ATEISISAttrs.IPv6)
	v6.Dst().Increment().SetStart(v6TrafficStart).SetCount(1)

	ts.ATE.OTG().PushConfig(t, ts.ATETop)
	ts.ATE.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, ts.ATE.OTG(), ts.ATETop, "IPv6")
	cfgplugins.VerifyDUTBGPEstablished(t, ts.DUT)
}

func checkTraffic(t *testing.T, ts *isissession.TestSession, flowName string) {
	ts.ATE.OTG().StartTraffic(t)
	time.Sleep(time.Second * 30)
	ts.ATE.OTG().StopTraffic(t)

	otgutils.LogFlowMetrics(t, ts.ATE.OTG(), ts.ATETop)
	otgutils.LogPortMetrics(t, ts.ATE.OTG(), ts.ATETop)

	t.Log("Checking flow telemetry...")
	recvMetric := gnmi.Get(t, ts.ATE.OTG(), gnmi.OTG().Flow(flowName).State())
	txPackets := recvMetric.GetCounters().GetOutPkts()
	rxPackets := recvMetric.GetCounters().GetInPkts()
	lostPackets := txPackets - rxPackets
	lossPct := lostPackets * 100 / txPackets

	if lossPct > 1 {
		t.Errorf("FAIL- Got %v%% packet loss for %s ; expected < 1%%", lossPct, flowName)
	}
}

func containsValue[T comparable](slice []T, val T) bool {
	found := false
	for _, v := range slice {
		if v == val {
			found = true
			break
		}
	}
	return found
}
