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

	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/ondatra"
)

// func DeviationCiscoRoutingPolicyBGPActionSetCommunity is used as an alternative to
// /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/config/set-med.
// This deviation implements CLI to perform the equivalent function.
func DeviationCiscoRoutingPolicyBGPActionSetMed(t *testing.T, dut *ondatra.DUTDevice, policyName string, statement string, prefixSetName string, setMed int, origin string) {
	//route-policy route-policy-v4
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

// func DeviationCiscoRoutingPolicyBGPActionSetCommunity is used as an alternative to
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
