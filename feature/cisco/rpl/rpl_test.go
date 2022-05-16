package rpl_base_test

import (
	"testing"

	"github.com/openconfig/ondatra"
	oc "github.com/openconfig/ondatra/telemetry"
)

func TestRPLConfig(t *testing.T) {
	dut := ondatra.DUT(t, device1)
	input_obj, err := testInput.GetTestInput(t)
	if err != nil {
		t.Error(err)
	}
	for _, policy := range input_obj.Device(dut).Features().Routepolicy {
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
		path := dut.Config().RoutingPolicy()
		defer observer.RecordYgot(t, "UPDATE", path)
		path.Update(t, rpl)
		defer observer.RecordYgot(t, "REPLACE", path)
		path.Update(t, rpl)
	}
}
