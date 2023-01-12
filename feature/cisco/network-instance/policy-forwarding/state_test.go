package network_instance_test

import (
	"context"
	"testing"

	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

const (
    NetworkInstanceDefault = "DEFAULT"
)

func Test_State_Interface(t *testing.T) {

    dut := ondatra.DUT(t, "dut")
	ctx := context.Background()
	util.GNMIWithText(ctx, t, dut, "\n")

    t.Log("Testing openconfig-network-instance:network-instances/network-instance/policy-forwarding/interfaces/interface \n")

	t.Run("Test", func(t *testing.T) {
		r1 := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
		r1.SequenceId = ygot.Uint32(1)
		r1.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String("TE")}
		r1.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
            SourceAddress: ygot.String("1.1.1.1/32"),
		}

		p := oc.NetworkInstance_PolicyForwarding_Policy{}
		p.PolicyId = ygot.String(pbrName)
		p.Type = oc.Policy_Type_VRF_SELECTION_POLICY
		p.Rule = map[uint32]*oc.NetworkInstance_PolicyForwarding_Policy_Rule{1: &r1}

        r2 := oc.NetworkInstance_PolicyForwarding_Interface{}
        r2.InterfaceId = ygot.String(InterfaceName)

		policy := oc.NetworkInstance_PolicyForwarding{}
		policy.Policy = map[string]*oc.NetworkInstance_PolicyForwarding_Policy{pbrName: &p}
        policy.Interface = map[string]*oc.NetworkInstance_PolicyForwarding_Interface{"id1": &r2}

        t.Run("Update", func(t *testing.T) {
			gnmi.Update(t, dut, gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().Config(), &policy)

            gnmi.Update(t, dut, gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().Interface(InterfaceName).ApplyVrfSelectionPolicy().Config(), pbrName)
		})

        t.Run("Get-State", func(t *testing.T) {
            state := gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().Interface(InterfaceName)
            stateGot := gnmi.Get(t, dut, state.State())

            if *stateGot.InterfaceId != *r2.InterfaceId {
               t.Errorf("Failed: Fetching state leaf for interface-id, got %v, want %v", *stateGot.InterfaceId, *r2.InterfaceId)
            } else {
               t.Logf("Passed: Configured interface-id = %v", *stateGot.InterfaceId)
            }

            if *stateGot.ApplyVrfSelectionPolicy != pbrName {
               t.Errorf("Failed: Fetching state leaf for apply-vrf-selection-policy, got %v, want %v",
                        *stateGot.ApplyVrfSelectionPolicy, pbrName)
            } else {
               t.Logf("Passed: Configured apply-vrf-selection-policy = %v", *stateGot.ApplyVrfSelectionPolicy)
            }
        })

		t.Run("Delete", func(t *testing.T) {
            gnmi.Delete(t, dut, gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().Interface(InterfaceName).Config())

            gnmi.Delete(t, dut, gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().Policy(pbrName).Config())
		})
	})
}
