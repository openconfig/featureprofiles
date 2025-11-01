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

// InstanceRoutePolicy parameters need to configure BGP route leaking between default and non-default VRFs.
type InstanceRoutePolicy struct {
	NetworkInstanceName string
	ImportCommunity     string
	ExportCommunity     string
}

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

// NewInterInstancePolicy configures BGP route leaking between default and non-default VRFs. It automatically chooses between OC or CLI based on deviation.
func NewInterInstancePolicy(t *testing.T, dut *ondatra.DUTDevice, batch *gnmi.SetBatch, cfg InstanceRoutePolicy) {
	t.Helper()
	if deviations.NetworkInstanceImportExportPolicyOCUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			t.Log("Configuring route leaking via CLI (deviation detected)")
			cli := fmt.Sprintf(`route-map RM-ALL-ROUTES permit 10
					router general
						vrf %[1]s
							leak routes source-vrf default subscribe-policy RM-ALL-ROUTES
						vrf default
							leak routes source-vrf %[1]s subscribe-policy RM-ALL-ROUTES
					`, cfg.NetworkInstanceName)
			helpers.GnmiCLIConfig(t, dut, cli)
		default:
			// Log a message if the vendor is not supported for this specific CLI deviation.
			t.Logf("Unsupported vendor %s for native command support for deviation 'import-export config'", dut.Vendor())
		}
	} else {
		t.Log("Configuring route leaking via OC")
		root := &oc.Root{}
		ni1 := root.GetOrCreateNetworkInstance(cfg.NetworkInstanceName)
		ni1Pol := ni1.GetOrCreateInterInstancePolicies()
		iexp1 := ni1Pol.GetOrCreateImportExportPolicy()
		iexp1.SetImportRouteTarget([]oc.NetworkInstance_InterInstancePolicies_ImportExportPolicy_ImportRouteTarget_Union{oc.UnionString(cfg.ImportCommunity)})
		iexp1.SetExportRouteTarget([]oc.NetworkInstance_InterInstancePolicies_ImportExportPolicy_ExportRouteTarget_Union{oc.UnionString(cfg.ExportCommunity)})
		gnmi.BatchReplace(batch, gnmi.OC().NetworkInstance(cfg.NetworkInstanceName).InterInstancePolicies().Config(), ni1Pol)
	}
}

// RemoveRouteLeakingFromCLI remove route leaking through CLI.
func RemoveInterInstancePolicy(t *testing.T, dut *ondatra.DUTDevice, batch *gnmi.SetBatch, cfg InstanceRoutePolicy) {
	t.Helper()
	if deviations.NetworkInstanceImportExportPolicyOCUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			cli := `
			no router general
			no route-map RM-ALL-ROUTES
			`
			helpers.GnmiCLIConfig(t, dut, cli)
		default:
			// Log a message if the vendor is not supported for this specific CLI deviation.
			t.Logf("Unsupported vendor %s for native command support for deviation 'import-export config'", dut.Vendor())
		}
	} else {
		gnmi.BatchDelete(batch, gnmi.OC().NetworkInstance(cfg.NetworkInstanceName).InterInstancePolicies().Config())
	}
}
