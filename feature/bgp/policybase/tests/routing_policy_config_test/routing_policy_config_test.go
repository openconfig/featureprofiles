// Copyright 2026 Google LLC
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

package routing_policy_config_test

import (
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
)

const (
	policyName = "FP_EFT1_RPL"
	prefixCIDR = "2001:db8:e::/64"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// TestRoutingPolicyCreateDelete covers a policy with an IPv6 prefix-set match.
func TestRoutingPolicyCreateDelete(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	policyPath := gnmi.OC().RoutingPolicy().PolicyDefinition(policyName)
	setPath := gnmi.OC().RoutingPolicy().DefinedSets().PrefixSet(policyName)
	t.Cleanup(func() {
		gnmi.Delete(t, dut, policyPath.Config())
		gnmi.Delete(t, dut, setPath.Config())
	})

	t.Log("Create an IPv6 prefix-set and a policy that matches it")
	sets := &oc.RoutingPolicy_DefinedSets{}
	prefixSet := sets.GetOrCreatePrefixSet(policyName)
	if !deviations.SkipPrefixSetMode(dut) {
		prefixSet.SetMode(oc.PrefixSet_Mode_IPV6)
	}
	prefixSet.GetOrCreatePrefix(prefixCIDR, "exact")
	gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().DefinedSets().Config(), sets)

	policy := &oc.RoutingPolicy_PolicyDefinition{}
	policy.SetName(policyName)
	stmt, err := policy.AppendNewStatement("10")
	if err != nil {
		t.Fatalf("AppendNewStatement: %v", err)
	}
	stmt.GetOrCreateConditions().GetOrCreateMatchPrefixSet().SetPrefixSet(policyName)
	stmt.GetOrCreateConditions().GetOrCreateMatchPrefixSet().SetMatchSetOptions(oc.RoutingPolicy_MatchSetOptionsRestrictedType_ANY)
	stmt.GetOrCreateActions().SetPolicyResult(oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)
	gnmi.Replace(t, dut, policyPath.Config(), policy)

	gotPolicy := gnmi.Get(t, dut, policyPath.Config())
	if gotPolicy.GetName() != policyName {
		t.Errorf("policy name = %q, want %q", gotPolicy.GetName(), policyName)
	}
	if result := gotPolicy.GetStatement("10").GetActions().GetPolicyResult(); result != oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE {
		t.Errorf("policy-result = %v, want ACCEPT_ROUTE", result)
	}
	if got := gnmi.Get(t, dut, setPath.Prefix(prefixCIDR, "exact").IpPrefix().Config()); got != prefixCIDR {
		t.Errorf("ip-prefix = %q, want %q", got, prefixCIDR)
	}
	gnmi.Await(t, dut, policyPath.Name().State(), time.Minute, policyName)

	t.Log("Delete the policy and the prefix-set")
	gnmi.Delete(t, dut, policyPath.Config())
	if gnmi.LookupConfig(t, dut, policyPath.Config()).IsPresent() {
		t.Errorf("policy %s still in config", policyName)
	}
}
