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
		t.Log("Executing CLI commands for local aggregate deviation")
		t.Log("Control functions code unit")
		bgpLocalAggConfigControlFunctions := fmt.Sprintf(`
configure terminal
!
router general
control-functions
code unit ipv4_generate_default_conditionally
function ipv4_generate_default_conditionally() {
if source_protocol is BGP and prefix match prefix_list_v4 %s {
return true;
}
}
EOF
!
compile
commit
`, triggerPfxName)

		runCliCommand(t, dut, bgpLocalAggConfigControlFunctions)

		t.Log("Dynamic prefix list rcf match")

		bgpLocalAggConfigDynamicPfxRcf := fmt.Sprintf(`
configure terminal
!
dynamic prefix-list ipv4_generate_default
match rcf ipv4_generate_default_conditionally()
prefix-list ipv4 %s
`, genPfxName)

		runCliCommand(t, dut, bgpLocalAggConfigDynamicPfxRcf)

		t.Log("Dynamic Advertised Prefix installation (default route) with drop NH")
		bgpLocalAggConfigPfxInstallDropNH := `
configure terminal
!
router general
vrf default
routes dynamic prefix-list ipv4_generate_default install drop
`
		runCliCommand(t, dut, bgpLocalAggConfigPfxInstallDropNH)

		t.Log("Redistribute advertised prefix into BGP")
		bgpLocalAggConfigPfxRedistribute := fmt.Sprintf(`
configure terminal
!
router bgp %d
redistribute dynamic
`, bgpAS)

		runCliCommand(t, dut, bgpLocalAggConfigPfxRedistribute)
	}
}
