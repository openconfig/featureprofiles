package basetest

import (
	"testing"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
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

		statement, err := rpd.AppendNewStatement(policy.Name + "stmt")
		if err != nil {
			t.Errorf("cannot reuse routing policy definition %v", err)
		}
		updatePolicy(statement, policy.Policy)
		t.Skip() //Skip till CSCvz13366 is fixed
		t.Run("Replace//routing-policy/policy-definitions/policy-definition", func(t *testing.T) {
			path := gnmi.OC().RoutingPolicy()

			defer observer.RecordYgot(t, "UPDATE", gnmi.OC().RoutingPolicy().PolicyDefinition(policy.Name).StatementMap().Config())
			defer observer.RecordYgot(t, "UPDATE", gnmi.OC().RoutingPolicy().PolicyDefinition(policy.Name).Name())
			gnmi.Update(t, dut, path.Config(), rpl)

		})

		if policy.Policy != nil {
			t.Run("Replace//routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result", func(t *testing.T) {
				t.Skip("Test requires a fix due to the new version of the model")
				//path := gnmi.OC().RoutingPolicy().PolicyDefinition(policy.Name).
				//path := gnmi.OC().RoutingPolicy().PolicyDefinition(policy.Name).StatementMap("id-1").Actions().PolicyResult()
				//defer observer.RecordYgot(t, "REPLACE", path)
				//gnmi.Replace(t, dut, path.Config(), oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)
			})
		}
		if policy.Policy.Bgpaction != nil {
			t.Run("Replace//routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/config/set-local-pref", func(t *testing.T) {
				//path := gnmi.OC().RoutingPolicy().PolicyDefinition(policy.Name).Statement("id-1").Actions().BgpActions().SetLocalPref()
				//defer observer.RecordYgot(t, "REPLACE", path)
				//gnmi.Replace(t, dut, path.Config(), 1)
				t.Skip("Test requires a fix due to the new version of the model")

			})

			t.Run("Replace//routing-policy/policy-definitions/policy-definition[name=DENY1]/statements/statement[name=id-1]/actions/bgp-actions/set-as-path-prepend/config/asn", func(t *testing.T) {
				//path := gnmi.OC().RoutingPolicy().PolicyDefinition(policy.Name).Statement("id-1").Actions().BgpActions().SetAsPathPrepend().Asn()
				//defer observer.RecordYgot(t, "REPLACE", path)
				//gnmi.Replace(t, dut, path.Config(), 1)
				t.Skip("Test requires a fix due to the new version of the model")

			})

			t.Run("Replace//routing-policy/policy-definitions/policy-definition[name=DENY1]/statements/statement[name=id-1]/actions/bgp-actions/set-as-path-prepend/config/repeat-n", func(t *testing.T) {
				//path := gnmi.OC().RoutingPolicy().PolicyDefinition(policy.Name).Statement("id-1").Actions().BgpActions().SetAsPathPrepend().RepeatN()
				//defer observer.RecordYgot(t, "REPLACE", path)
				//gnmi.Replace(t, dut, path.Config(), 1)
				t.Skip("Test requires a fix due to the new version of the model")

			})

			t.Run("Replace//routing-policy/policy-definitions/policy-definition[name=DENY1]/statements/statement[name=id-1]/actions/bgp-actions/config/set-med", func(t *testing.T) {
				t.Skip("Test requires a fix due to the new version of the model")
				//path := gnmi.OC().RoutingPolicy().PolicyDefinition(policy.Name).Statement("id-1").Actions().BgpActions().SetMed()
				//defer observer.RecordYgot(t, "REPLACE", path)
				//gnmi.Replace[oc.RoutingPolicy_PolicyDefinition_Statement_Actions_BgpActions_SetMed_Union](t, dut, path.Config(), oc.UnionString("3"))
			})

			t.Run("Replace//routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/config/set-local-pref", func(t *testing.T) {
				t.Skip("Test requires a fix due to the new version of the model")
				//path := gnmi.OC().RoutingPolicy().PolicyDefinition(policy.Name).Statement("id-1").Actions().BgpActions().SetLocalPref()
				//defer observer.RecordYgot(t, "REPLACE", path)
				//gnmi.Replace(t, dut, path.Config(), 1)

			})

		}
		if policy.Policy != nil {
			t.Run("Update//routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result", func(t *testing.T) {
				t.Skip("Test requires a fix due to the new version of the model")
				//path := gnmi.OC().RoutingPolicy().PolicyDefinition(policy.Name).Statement("id-1").Actions().PolicyResult()
				//defer observer.RecordYgot(t, "UPDATE", path)
				//gnmi.Update(t, dut, path.Config(), oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)
			})
		}
		if policy.Policy.Bgpaction != nil {
			t.Run("Update//routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/config/set-local-pref", func(t *testing.T) {
				//path := gnmi.OC().RoutingPolicy().PolicyDefinition(policy.Name).Statement("id-1").Actions().BgpActions().SetLocalPref()
				//defer observer.RecordYgot(t, "UPDATE", path)
				//gnmi.Update(t, dut, path.Config(), 1)
				t.Skip("Test requires a fix due to the new version of the model")

			})

			t.Run("Update//routing-policy/policy-definitions/policy-definition[name=DENY1]/statements/statement[name=id-1]/actions/bgp-actions/set-as-path-prepend/config/asn", func(t *testing.T) {
				//path := gnmi.OC().RoutingPolicy().PolicyDefinition(policy.Name).Statement("id-1").Actions().BgpActions().SetAsPathPrepend().Asn()
				//defer observer.RecordYgot(t, "UPDATE", path)
				//gnmi.Update(t, dut, path.Config(), 1)
				t.Skip("Test requires a fix due to the new version of the model")

			})

			t.Run("Update//routing-policy/policy-definitions/policy-definition[name=DENY1]/statements/statement[name=id-1]/actions/bgp-actions/set-as-path-prepend/config/repeat-n", func(t *testing.T) {
				//path := gnmi.OC().RoutingPolicy().PolicyDefinition(policy.Name).Statement("id-1").Actions().BgpActions().SetAsPathPrepend().RepeatN()
				//defer observer.RecordYgot(t, "UPDATE", path)
				//gnmi.Update(t, dut, path.Config(), 1)
				t.Skip("Test requires a fix due to the new version of the model")

			})

			t.Run("Replace//routing-policy/policy-definitions/policy-definition[name=DENY1]/statements/statement[name=id-1]/actions/bgp-actions/config/set-med", func(t *testing.T) {
				//path := gnmi.OC().RoutingPolicy().PolicyDefinition(policy.Name).Statement("id-1").Actions().BgpActions().SetMed()
				//defer observer.RecordYgot(t, "UPDATE", path)
				//gnmi.Update[oc.RoutingPolicy_PolicyDefinition_Statement_Actions_BgpActions_SetMed_Union](t, dut, path.Config(), oc.UnionString("3"))
				t.Skip("Test requires a fix due to the new version of the model")
			})

			t.Run("Update//routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/config/set-local-pref", func(t *testing.T) {
				//path := gnmi.OC().RoutingPolicy().PolicyDefinition(policy.Name).Statement("id-1").Actions().BgpActions().SetLocalPref()
				//defer observer.RecordYgot(t, "UPDATE", path)
				//gnmi.Update(t, dut, path.Config(), 1)
				t.Skip("Test requires a fix due to the new version of the model")

			})

		}
	}
}
