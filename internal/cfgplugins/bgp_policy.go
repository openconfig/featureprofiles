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

package cfgplugins

import (
	"fmt"
	"strings"
	"testing"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
)

// DeviationCiscoRoutingPolicyBGPActionSetMed is used as an alternative to
// /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/config/set-med.
// This deviation implements CLI to perform the equivalent function.
func DeviationCiscoRoutingPolicyBGPActionSetMed(t *testing.T, dut *ondatra.DUTDevice, policyName string, statement string, prefixSetName string, setMed int, origin string) {
	// route-policy route-policy-v4
	//   #statement-name statement-v4
	//   if destination in prefix-set-v4 then
	//     set med 104
	//     set origin igp
	//   endif
	// end-policy
	cliConfig := fmt.Sprintf("route-policy %s\n #statement-name %v\n if destination in %v then\n", policyName, statement, prefixSetName)

	if setMed != 0 {
		cliConfig += fmt.Sprintf("  set med %d\n", setMed)
	}
	if origin != "" {
		cliConfig += fmt.Sprintf("  set origin %v\n", origin)
	}
	cliConfig += "  done\n endif\nend-policy\n"
	helpers.GnmiCLIConfig(t, dut, cliConfig)
}

// DeviationCiscoRoutingPolicyBGPActionSetCommunity is used as an alternative to
// /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/set-community
// This deviation implements CLI to perform the equivalent function.
func DeviationCiscoRoutingPolicyBGPActionSetCommunity(t *testing.T, dut *ondatra.DUTDevice, policyName string, statement string, community string) {
	// route-policy route-policy-v4
	//   #statement-name statement-v4
	//   set community community-set-v4
	//   done
	// end-policy
	cliConfig := fmt.Sprintf("route-policy %s\n #statement-name %v\n", policyName, statement)
	if community != "" {
		cliConfig += fmt.Sprintf("  set community %v\n", community)
	}
	cliConfig += " done\nend-policy\n"
	helpers.GnmiCLIConfig(t, dut, cliConfig)
}

// DeviationJuniperRoutingPolicyBGPActionSetCommunity is used as an alternative to
// /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/set-community
// This deviation implements CLI to perform the equivalent function.
func DeviationJuniperRoutingPolicyBGPActionSetCommunity(t *testing.T, dut *ondatra.DUTDevice, policyName string, statement string, community string) {
	config := fmt.Sprintf(`
	policy-options {
		policy-statement %s {
			term %s {
				then {
					community add %s;
				}
			}
		}
	}`, policyName, statement, community)
	helpers.GnmiCLIConfig(t, dut, config)
}

// DeviationAristaRoutingPolicyBGPAsPathSetUnsupported is used for DUTs that don't support filtering by AS-Set (in tests such as RT-1.64)
// This deviation implements CLI to perform the same function
func DeviationAristaRoutingPolicyBGPAsPathSetUnsupported(t *testing.T, dut *ondatra.DUTDevice, aclName string, routeMap string, asPathRegex string) {
	// ip as-path access-list "aclName" permit "asPathRegex"
	// ip as-path access-list "aclName" deny .*
	// route-map "routeMap" "sequence"
	// 	match as-path "aclName"
	config := fmt.Sprintf(`
ip as-path access-list %s permit %s
ip as-path access-list %s deny .*
route-map %s
match as-path %s
`, aclName, asPathRegex, aclName, routeMap, aclName)
	helpers.GnmiCLIConfig(t, dut, config)
}

// DeviationCiscoRoutingPolicyBGPToISIS is used as an alternative for DUTs that don't support
// direct redistribution from BGP to ISIS using community match as condition.
// This deviation implements CLI to perform the equivalent function.
func DeviationCiscoRoutingPolicyBGPToISIS(t *testing.T, dut *ondatra.DUTDevice, asn uint32, bgpInstance string, policyName string, community string, tagSet int) {
	// route-policy TAG_7
	//   if community in SNH then
	//     set tag 7
	//   endif
	//   pass
	// end-policy
	// router bgp 64500 instance BGP
	//   address-family ipv4 unicast
	//     table-policy TAG_7
	//   address-family ipv6 unicast
	//     table-policy TAG_7
	cliConfig := fmt.Sprintf("route-policy %s\n", policyName)
	if community != "" {
		cliConfig += fmt.Sprintf("  if community in %v then\n set tag %v\nendif\n", community, tagSet)
	}
	cliConfig += " pass\nend-policy\n"

	cliConfig += fmt.Sprintf("router bgp %v instance %v\n address-family ipv4 unicast\n table-policy %v\naddress-family ipv6 unicast\n table-policy %v\n", asn, bgpInstance, policyName, policyName)

	helpers.GnmiCLIConfig(t, dut, cliConfig)
}

// ConfigureCommonBGPPolicies applies a standard set of BGP route policies and defined sets
// to the provided oc.RoutingPolicy object.
func ConfigureCommonBGPPolicies(t *testing.T, dut *ondatra.DUTDevice) *oc.RoutingPolicy {
	t.Helper()

	d := &oc.Root{}
	rp := d.GetOrCreateRoutingPolicy()

	// 1. Configure Link Bandwidth Extended Community
	configureLinkBandwidthSet(t, dut, rp)

	// 2. Define standard Community Sets
	defineCommonCommunitySets(t, dut, rp)

	// 3. Define core Policy Definitions (IBGP, ALLOW, CONVERGENCE)
	defineCorePolicyDefinitions(t, dut, rp)

	// 4. Handle SNH Community Set and BGP_TO_ISIS policy
	configureSNHCommunityAndPolicy(t, dut, rp)
	return rp
}

// getLinkBwWildcard returns the vendor-specific link bandwidth community regex.
func getLinkBwWildcard(t *testing.T, vendor ondatra.Vendor) string {
	t.Helper()
	switch vendor {
	case ondatra.CISCO:
		return "^.*:.*$"
	case ondatra.JUNIPER:
		return "^link-bandwidth:.*"
	case ondatra.ARISTA:
		return "^lbw:.*:.*$"
	case ondatra.NOKIA:
		return "^link-bandwidth:.*:.*"
	default:
		t.Fatalf("Unsupported vendor %s for link bandwidth wildcard", vendor)
		return ""
	}
}

// configureLinkBandwidthSet sets up the link bandwidth extended community set.
func configureLinkBandwidthSet(t *testing.T, dut *ondatra.DUTDevice, rp *oc.RoutingPolicy) {
	t.Helper()
	linkbw_wildcard := getLinkBwWildcard(t, dut.Vendor())

	if deviations.BgpExtendedCommunitySetUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.CISCO:
			communitySetCLIConfig := fmt.Sprintf("community-set linkbw_wildcard \n dfa-regex '%v' \n end-set\n extcommunity-set combo-set linkbw_wildcard\n ios-regex '%v' \n end-set\n", linkbw_wildcard, linkbw_wildcard)
			helpers.GnmiCLIConfig(t, dut, communitySetCLIConfig)
		case ondatra.NOKIA:
			t.Log("Skipping linkbw_wildcard community set config for Nokia as it is not supported in OC")
		default:
			t.Fatalf("Unsupported vendor %s for native command support for deviation 'BgpExtendedCommunitySetUnsupported'", dut.Vendor())
		}
	} else {
		pdef := rp.GetOrCreateDefinedSets().GetOrCreateBgpDefinedSets()
		stmt, err := pdef.NewExtCommunitySet("linkbw_wildcard")
		if err != nil {
			t.Fatalf("NewExtCommunitySet('linkbw_wildcard') failed: %v", err)
		}
		stmt.SetExtCommunitySetName("linkbw_wildcard")
		stmt.SetExtCommunityMember([]string{linkbw_wildcard})
	}
}

// defineCommonCommunitySets defines standard BGP community sets.
func defineCommonCommunitySets(t *testing.T, dut *ondatra.DUTDevice, rp *oc.RoutingPolicy) {
	t.Helper()
	bgpSets := rp.GetOrCreateDefinedSets().GetOrCreateBgpDefinedSets()

	csEBGPIn := bgpSets.GetOrCreateCommunitySet("EBGP-Routes-IN")
	csEBGPIn.SetCommunityMember([]oc.RoutingPolicy_DefinedSets_BgpDefinedSets_CommunitySet_CommunityMember_Union{
		oc.UnionString("200:101"), oc.UnionString("200:102"), oc.UnionString("200:103"), oc.UnionString("200:104"),
		oc.UnionString("200:105"), oc.UnionString("200:106"), oc.UnionString("200:107"),
	})

	csEBGPOut := bgpSets.GetOrCreateCommunitySet("EBGP-Routes-OUT")
	csEBGPOut.SetCommunityMember([]oc.RoutingPolicy_DefinedSets_BgpDefinedSets_CommunitySet_CommunityMember_Union{
		oc.UnionString("100:101"), oc.UnionString("100:102"), oc.UnionString("100:103"), oc.UnionString("100:104"),
		oc.UnionString("100:105"), oc.UnionString("100:106"), oc.UnionString("100:107"),
	})

	csFloat := bgpSets.GetOrCreateCommunitySet("float-routes")
	csFloat.SetCommunityMember([]oc.RoutingPolicy_DefinedSets_BgpDefinedSets_CommunitySet_CommunityMember_Union{
		oc.UnionString("300:256"), oc.UnionString("300:257"), oc.UnionString("300:258"), oc.UnionString("300:259"),
		oc.UnionString("300:260"), oc.UnionString("300:261"), oc.UnionString("300:262"), oc.UnionString("300:263"),
	})
}

// addDeleteLinkBwAction adds an action to a statement to remove the link-bandwidth community.
func addDeleteLinkBwAction(t *testing.T, dut *ondatra.DUTDevice, stmt *oc.RoutingPolicy_PolicyDefinition_Statement) {
	t.Helper()
	if deviations.BgpDeleteLinkBandwidthUnsupported(dut) {
		// Handled by CLI in defineCorePolicyDefinitions
		return
	}
	ref := stmt.GetOrCreateActions().GetOrCreateBgpActions().GetOrCreateSetExtCommunity()
	ref.GetOrCreateReference().SetExtCommunitySetRefs([]string{"linkbw_wildcard"})
	ref.SetOptions(oc.BgpPolicy_BgpSetCommunityOptionType_REMOVE)
	ref.SetMethod(oc.SetCommunity_Method_REFERENCE)
}

// defineCorePolicyDefinitions defines the main IBGP, ALLOW, and CONVERGENCE policies.
func defineCorePolicyDefinitions(t *testing.T, dut *ondatra.DUTDevice, rp *oc.RoutingPolicy) {
	t.Helper()

	// ALLOW policy
	pdAllow := rp.GetOrCreatePolicyDefinition("ALLOW")
	stAllow, err := pdAllow.AppendNewStatement("id-1")
	if err != nil {
		t.Fatalf("AppendNewStatement('ALLOW/id-1') failed: %v", err)
	}
	stAllow.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE

	// IBGP-IN policy
	ibgpIN := rp.GetOrCreatePolicyDefinition("IBGP-IN")
	stmtIN1, _ := ibgpIN.AppendNewStatement("from-local-ebgp")
	applyCommunityMatch(t, dut, stmtIN1, "EBGP-Routes-IN", oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)
	stmtIN2, _ := ibgpIN.AppendNewStatement("float-routes")
	applyCommunityMatch(t, dut, stmtIN2, "float-routes", oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)
	rejectAllStmt(t, ibgpIN, "REJECT-ALL")

	// IBGP-OUT policy
	ibgpOUT := rp.GetOrCreatePolicyDefinition("IBGP-OUT")
	delBwOUT, _ := ibgpOUT.AppendNewStatement("del_linkbw")
	addDeleteLinkBwAction(t, dut, delBwOUT)
	stmtOUT1, _ := ibgpOUT.AppendNewStatement("accept-bgp-routes")
	applyCommunityMatch(t, dut, stmtOUT1, "EBGP-Routes-OUT", oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)
	rejectAllStmt(t, ibgpOUT, "REJECT-ALL")

	// ALLOW-OUT policy
	allowOUT := rp.GetOrCreatePolicyDefinition("ALLOW-OUT")
	delBwAOUT, _ := allowOUT.AppendNewStatement("del_linkbw")
	addDeleteLinkBwAction(t, dut, delBwAOUT)
	stmtAOUT1, _ := allowOUT.AppendNewStatement("match-ebgp-in")
	applyCommunityMatch(t, dut, stmtAOUT1, "EBGP-Routes-IN", oc.RoutingPolicy_PolicyResultType_REJECT_ROUTE)
	rejectAllStmt(t, allowOUT, "REJECT-ALL")

	// ALLOW-IN policy
	allowIN := rp.GetOrCreatePolicyDefinition("ALLOW-IN")
	stmtAIN1, _ := allowIN.AppendNewStatement("match-ebgp-out")
	applyCommunityMatch(t, dut, stmtAIN1, "EBGP-Routes-OUT", oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)
	stmtAIN2, _ := allowIN.AppendNewStatement("float-routes")
	applyCommunityMatch(t, dut, stmtAIN2, "float-routes", oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)
	rejectAllStmt(t, allowIN, "REJECT-ALL")

	// CONVERGENCE-OUT policy
	convergenceOUT := rp.GetOrCreatePolicyDefinition("CONVERGENCE-OUT")
	delBwCOUT, _ := convergenceOUT.AppendNewStatement("del_linkbw")
	addDeleteLinkBwAction(t, dut, delBwCOUT)
	stmtCOUT1, _ := convergenceOUT.AppendNewStatement("from-local-ebgp")
	applyCommunityMatch(t, dut, stmtCOUT1, "EBGP-Routes-IN", oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)
	stmtCOUT2, _ := convergenceOUT.AppendNewStatement("from-ibgp")
	applyCommunityMatch(t, dut, stmtCOUT2, "EBGP-Routes-OUT", oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)
	stmtCOUT3, _ := convergenceOUT.AppendNewStatement("float-routes")
	applyCommunityMatch(t, dut, stmtCOUT3, "float-routes", oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)
	rejectAllStmt(t, convergenceOUT, "REJECT-ALL")

	// CONVERGENCE-IN policy
	convergenceIN := rp.GetOrCreatePolicyDefinition("CONVERGENCE-IN")
	rejectAllStmt(t, convergenceIN, "REJECT-ALL")

	// CLI for BgpDeleteLinkBandwidthUnsupported
	if deviations.BgpDeleteLinkBandwidthUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.CISCO:
			delLinkbwCLIConfig := "route-policy delete_linkbw\n delete extcommunity bandwidth all\n pass \n end-policy"
			helpers.GnmiCLIConfig(t, dut, delLinkbwCLIConfig)
		default:
			t.Fatalf("Unsupported vendor %s for native cmd support for deviation 'BgpDeleteLinkBandwidthUnsupported'", dut.Vendor())
		}
	}
}

// applyCommunityMatch is a helper to add community match conditions and set policy result.
func applyCommunityMatch(t *testing.T, dut *ondatra.DUTDevice, stmt *oc.RoutingPolicy_PolicyDefinition_Statement, communitySetName string, result oc.E_RoutingPolicy_PolicyResultType) {
	t.Helper()
	if deviations.BGPConditionsMatchCommunitySetUnsupported(dut) {
		stmt.GetOrCreateConditions().GetOrCreateBgpConditions().SetCommunitySet(communitySetName)
	} else {
		matchSet := stmt.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet()
		matchSet.SetCommunitySet(communitySetName)
		matchSet.SetMatchSetOptions(oc.RoutingPolicy_MatchSetOptionsType_ANY)
	}
	stmt.GetOrCreateActions().SetPolicyResult(result)
}

// rejectAllStmt appends a reject-all statement to a policy definition.
func rejectAllStmt(t *testing.T, pd *oc.RoutingPolicy_PolicyDefinition, name string) {
	t.Helper()
	stmt, err := pd.AppendNewStatement(name)
	if err != nil {
		t.Fatalf("AppendNewStatement(%s) to %v failed: %v", name, pd.Name, err)
	}
	stmt.GetOrCreateActions().SetPolicyResult(oc.RoutingPolicy_PolicyResultType_REJECT_ROUTE)
}

// configureSNHCommunityAndPolicy configures SNH community and the BGP_TO_ISIS policy.
func configureSNHCommunityAndPolicy(t *testing.T, dut *ondatra.DUTDevice, rp *oc.RoutingPolicy) {
	t.Helper()

	snhCommunityMembers := []oc.RoutingPolicy_DefinedSets_BgpDefinedSets_CommunitySet_CommunityMember_Union{oc.UnionString("100:100")}
	if deviations.BGPConditionsMatchCommunitySetUnsupported(dut) {
		snhCommunityMembers = []oc.RoutingPolicy_DefinedSets_BgpDefinedSets_CommunitySet_CommunityMember_Union{oc.UnionString("100:100")}
	}

	communitySet1 := rp.GetOrCreateDefinedSets().GetOrCreateBgpDefinedSets().GetOrCreateCommunitySet("SNH")
	communitySet1.SetCommunityMember(snhCommunityMembers)

	if deviations.CommunityMemberRegexUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.CISCO:
			communitySetCLIConfig := fmt.Sprintf("community-set %v\n ios-regex '(10[0-9]:1)'\n end-set", "SNH")
			helpers.GnmiCLIConfig(t, dut, communitySetCLIConfig)
		default:
			t.Fatalf("Unsupported vendor %s for deviation 'CommunityMemberRegexUnsupported'", dut.Vendor())
		}
	}

	// BGP_TO_ISIS Policy
	pdef1 := rp.GetOrCreatePolicyDefinition("BGP_TO_ISIS")

	if deviations.BGPConditionsMatchCommunitySetUnsupported(dut) {
		tagSet := rp.GetOrCreateDefinedSets()
		snh := tagSet.GetOrCreateTagSet("TAG_7")
		tagValue := 7
		snh.SetName("TAG_7")
		snh.SetTagValue([]oc.RoutingPolicy_DefinedSets_TagSet_TagValue_Union{oc.UnionUint32(tagValue)})

		stmt3, err3 := pdef1.AppendNewStatement("id-1")
		if err3 != nil {
			t.Fatalf("AppendNewStatement(BGP_TO_ISIS/id-1) failed: %v", err3)
		}
		stmt3.GetOrCreateConditions().GetOrCreateMatchTagSet().SetTagSet("TAG_7")
		stmt3.GetOrCreateActions().SetPolicyResult(oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)
	} else {

		stmt1, err1 := pdef1.AppendNewStatement("routePolicyStatement")
		if err1 != nil {
			t.Fatalf("AppendNewStatement(BGP_TO_ISIS/routePolicyStatement) failed: %v", err1)
		}
		communitySet := stmt1.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet()
		communitySet.SetCommunitySet("SNH")
		communitySet.SetMatchSetOptions(oc.RoutingPolicy_MatchSetOptionsType_ANY)
		stmt1.GetOrCreateActions().SetPolicyResult(oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)

		if !deviations.SkipIsisSetLevel(dut) {
			stmt1.GetOrCreateActions().GetOrCreateIsisActions().SetSetLevel(2)
		}
		if !deviations.SkipIsisSetMetricStyleType(dut) {
			stmt1.GetOrCreateActions().GetOrCreateIsisActions().SetSetMetricStyleType(oc.IsisPolicy_MetricStyle_WIDE_METRIC)
		}
		stmt1.GetOrCreateActions().SetPolicyResult(oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)
	}
	rejectAllStmt(t, pdef1, "reject-all")
}

func RoutingPolicyBGPAdvertiseAggregate(t *testing.T, dut *ondatra.DUTDevice, triggerPfxName string, triggerPfx string, genPfxName string, genPfx string, bgpAS uint, localAggregateName string) {
	if deviations.BgpLocalAggregateUnsupported(dut) {
		routingPolicyBGPAdvertiseAggregate(t, dut, triggerPfxName, genPfxName, bgpAS)
	} else {
		dc := gnmi.OC()
		root := &oc.Root{}
		dni := deviations.DefaultNetworkInstance(dut)
		// t.Log("Configuring local aggregate for 0.0.0.0/0...")
		ni := root.GetOrCreateNetworkInstance(dni)

		aggProto := ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_LOCAL_AGGREGATE, localAggregateName)
		aggProto.SetIdentifier(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_LOCAL_AGGREGATE)
		aggProto.SetName(localAggregateName)

		aggProto.GetOrCreateAggregate(genPfx)
		aggProto.GetOrCreateAggregate(genPfx).SetPrefix(genPfx)
		aggProto.SetEnabled(true)

		gnmi.Replace(t, dut, dc.NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_LOCAL_AGGREGATE, localAggregateName).Config(), aggProto)
	}
}

// routingPolicyBGPAdvertiseAggregate is used for DUTs that don't support OC local aggregates
func routingPolicyBGPAdvertiseAggregate(t *testing.T, dut *ondatra.DUTDevice, triggerPfxName string, genPfxName string, bgpAS uint) {
	switch dut.Vendor() {
	case ondatra.ARISTA:
		var cliConfig strings.Builder
		t.Log("Executing CLI commands for local aggregate deviation")

		t.Log("Dynamic prefix list rcf match")
		cliConfig.WriteString("configure terminal\n")
		cliConfig.WriteString(fmt.Sprintf("dynamic prefix-list ipv4_generate_default\nmatch rcf ipv4_generate_default_conditionally()\nprefix-list ipv4 %s\n", genPfxName))

		t.Log("Dynamic Advertised Prefix installation (default route) with drop NH")
		cliConfig.WriteString("router general\nvrf default\nroutes dynamic prefix-list ipv4_generate_default install drop\n")

		t.Log("Redistribute advertised prefix into BGP")
		cliConfig.WriteString(fmt.Sprintf("router bgp %d\nredistribute dynamic\n", bgpAS))
		helpers.GnmiCLIConfig(t, dut, cliConfig.String())

		cliConfig.Reset()
		t.Log("Control functions code unit")
		cliConfig.WriteString("configure terminal\n")
		cliConfig.WriteString(fmt.Sprintf(`router general
		control-functions
	{ "cmd": "code unit ipv4_generate_default_conditionally", "input": "function ipv4_generate_default_conditionally()\n{\nif source_protocol is BGP and prefix match prefix_list_v4 %s {\nreturn true;\n}\n}\nEOF"}
	compile
	commit
	exit
	`, triggerPfxName))

		helpers.GnmiCLIConfig(t, dut, cliConfig.String())
	default:
		t.Logf("Unsupported vendor %s for native cmd support for deviation 'BgpLocalAggregateUnsupported'", dut.Vendor())
	}
}
