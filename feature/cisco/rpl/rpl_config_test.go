package basetest

import (
	"testing"

	"github.com/openconfig/ondatra"
	oc "github.com/openconfig/ondatra/telemetry"
)

func TestRPLConfig(t *testing.T) {
	dut := ondatra.DUT(t, device1)
	inputObj, err := testInput.GetTestInput(t)
	if err != nil {
		t.Error(err)
	}
	for _, policy := range inputObj.Device(dut).Features().Routepolicy {
		rpl := &oc.RoutingPolicy{}
		rpd, err := rpl.NewPolicyDefinition(policy.Name)
		if err != nil {
			t.Errorf("cannot reuse routing policy definition %v", err)
		}

		statement, err := rpd.NewStatement(policy.Name + "stmt")
		if err != nil {
			t.Errorf("cannot reuse routing policy definition %v", err)
		}
		updatePolicy(statement, policy.Policy)
		t.Run("replaceconfig//routing-policy/policy-definitions/policy-definition", func(t *testing.T) {
			path := dut.Config().RoutingPolicy()

			defer observer.RecordYgot(t, "UPDATE", dut.Config().RoutingPolicy().PolicyDefinition(policy.Name).Statement("id-1").Name())
			defer observer.RecordYgot(t, "UPDATE", dut.Config().RoutingPolicy().PolicyDefinition(policy.Name).Name())
			path.Update(t, rpl)

		})

		if policy.Policy != nil {
			t.Run("replaceconfig//routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result", func(t *testing.T) {
				path := dut.Config().RoutingPolicy().PolicyDefinition(policy.Name).Statement("id-1").Actions().PolicyResult()
				defer observer.RecordYgot(t, "UPDATE", path)
				path.Replace(t, oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)
			})
		}
		if policy.Policy.Bgpaction != nil {
			t.Run("replaceconfig//routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/config/set-local-pref", func(t *testing.T) {
				path := dut.Config().RoutingPolicy().PolicyDefinition(policy.Name).Statement("id-1").Actions().BgpActions().SetLocalPref()
				defer observer.RecordYgot(t, "UPDATE", path)
				path.Replace(t, 1)

			})

			t.Run("replaceconfig//routing-policy/policy-definitions/policy-definition[name=DENY1]/statements/statement[name=id-1]/actions/bgp-actions/set-as-path-prepend/config/asn", func(t *testing.T) {
				path := dut.Config().RoutingPolicy().PolicyDefinition(policy.Name).Statement("id-1").Actions().BgpActions().SetAsPathPrepend().Asn()
				defer observer.RecordYgot(t, "UPDATE", path)
				path.Replace(t, 1)

			})

			t.Run("replaceconfig//routing-policy/policy-definitions/policy-definition[name=DENY1]/statements/statement[name=id-1]/actions/bgp-actions/set-as-path-prepend/config/repeat-n", func(t *testing.T) {
				path := dut.Config().RoutingPolicy().PolicyDefinition(policy.Name).Statement("id-1").Actions().BgpActions().SetAsPathPrepend().RepeatN()
				defer observer.RecordYgot(t, "UPDATE", path)
				path.Replace(t, 1)

			})

			t.Run("replaceconfig//routing-policy/policy-definitions/policy-definition[name=DENY1]/statements/statement[name=id-1]/actions/bgp-actions/config/set-med", func(t *testing.T) {
				path := dut.Config().RoutingPolicy().PolicyDefinition(policy.Name).Statement("id-1").Actions().BgpActions().SetMed()
				defer observer.RecordYgot(t, "UPDATE", path)
				path.Replace(t, oc.UnionString("3"))

			})

			t.Run("replaceconfig//routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/config/set-local-pref", func(t *testing.T) {
				path := dut.Config().RoutingPolicy().PolicyDefinition(policy.Name).Statement("id-1").Actions().BgpActions().SetLocalPref()
				defer observer.RecordYgot(t, "UPDATE", path)
				path.Replace(t, 1)

			})

		}
	}
}
