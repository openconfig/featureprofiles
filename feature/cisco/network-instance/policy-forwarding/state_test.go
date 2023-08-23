package network_instance_test

import (
	"context"
	"testing"
	"time"

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

			if *stateGot.InterfaceId != InterfaceName {
				t.Errorf("Failed: Fetching state leaf for interface-id, got %v, want %v",
					*stateGot.InterfaceId, InterfaceName)
			} else {
				t.Logf("Passed: Configured interface-id = %v", *stateGot.InterfaceId)
			}

			if *stateGot.ApplyVrfSelectionPolicy != pbrName {
				t.Errorf("Failed: Fetching state leaf for apply-vrf-selection-policy, got %v, want %v",
					*stateGot.ApplyVrfSelectionPolicy, pbrName)
			} else {
				t.Logf("Passed: Configured apply-vrf-selection-policy = %v",
					*stateGot.ApplyVrfSelectionPolicy)
			}

			intfRef := state.InterfaceRef()
			intfRefStateGot := gnmi.Get(t, dut, intfRef.State())
			if *intfRefStateGot.Interface != InterfaceName {
				t.Errorf("Failed: Fetching state leaf for interface-ref, got %v, want %v",
					*intfRefStateGot.Interface, InterfaceName)
			} else {
				t.Logf("Passed: Configured interface-ref interface = %v",
					*intfRefStateGot.Interface)
			}
		})

		t.Run("Delete", func(t *testing.T) {
			gnmi.Delete(t, dut, gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().Interface(InterfaceName).Config())

			gnmi.Delete(t, dut, gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().Policy(pbrName).Config())

		})
	})
}

func Test_Decap_Feature_Telemetry(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	t.Log("Remove Flowspec Config")
	configToChange := "no flowspec \n"
	ctx := context.Background()
	util.GNMIWithText(ctx, t, dut, configToChange)

	r1 := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
	r1.SequenceId = ygot.Uint32(SeqID1)

	r1.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
		Protocol:      oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP,
		DscpSet:       []uint8{*ygot.Uint8(4), *ygot.Uint8(8)},
		SourceAddress: ygot.String("222.222.222.222/32"),
	}

	r1.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{
		DecapNetworkInstance:         ygot.String(vrfBLUE),
		DecapFallbackNetworkInstance: ygot.String(vrfRED),
		PostDecapNetworkInstance:     ygot.String(vrfGREEN),
	}

	r2 := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
	r2.SequenceId = ygot.Uint32(SeqID2)

	r2.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
		Protocol:      oc.UnionUint8(41),
		DscpSet:       []uint8{*ygot.Uint8(1), *ygot.Uint8(2)},
		SourceAddress: ygot.String("111.111.111.111/32"),
	}

	r2.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{
		DecapNetworkInstance:         ygot.String(vrfORANGE),
		DecapFallbackNetworkInstance: ygot.String(vrfPINK),
		PostDecapNetworkInstance:     ygot.String(vrfBROWN),
	}

	p := oc.NetworkInstance_PolicyForwarding_Policy{}
	p.PolicyId = ygot.String(pbrName)
	p.Type = oc.Policy_Type_VRF_SELECTION_POLICY
	p.Rule = map[uint32]*oc.NetworkInstance_PolicyForwarding_Policy_Rule{1: &r1, 2: &r2}

	policy := oc.NetworkInstance_PolicyForwarding{}
	policy.Policy = map[string]*oc.NetworkInstance_PolicyForwarding_Policy{pbrName: &p}

	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().Config(), &policy)
	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().Interface(InterfaceName).ApplyVrfSelectionPolicy().Config(), pbrName)
	defer gnmi.Delete(t, dut, gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().Config())
	subscriptionDuration := 65 * time.Second
	expectedSamples := 2

	t.Run("NetworkInstance_PolicyForwarding_PolicyPath", func(t *testing.T) {
		//t.Parallel()
		statePath := gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().Policy(pbrName)
		if got := gnmi.Collect(t, dut, statePath.State(), subscriptionDuration).Await(t); len(got) < expectedSamples {
			t.Errorf("NetworkInstance_PolicyForwarding_PolicyPath samples: got %d, want %d", len(got), expectedSamples)
		} else {
			t.Logf("Collected samples for NetworkInstance_PolicyForwarding_PolicyPath :\n%v", got)
		}
	})
	t.Run("NetworkInstance_PolicyForwarding_Policy_RulePath", func(t *testing.T) {
		//t.Parallel()
		statePath := gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().Policy(pbrName).Rule(SeqID1)
		if got := gnmi.Collect(t, dut, statePath.State(), subscriptionDuration).Await(t); len(got) < expectedSamples {
			t.Errorf("NetworkInstance_PolicyForwarding_Policy_RulePath samples: got %d, want %d", len(got), expectedSamples)
		} else {
			t.Logf("Collected samples for NetworkInstance_PolicyForwarding_Policy_RulePath :\n%v", got)
		}
	})

	t.Run("NetworkInstance_PolicyForwarding_Policy_Rule_ActionPath", func(t *testing.T) {
		//t.Parallel()
		statePath := gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().Policy(pbrName).Rule(SeqID1).Action()
		if got := gnmi.Collect(t, dut, statePath.State(), subscriptionDuration).Await(t); len(got) < expectedSamples {
			t.Errorf("NetworkInstance_PolicyForwarding_Policy_Rule_ActionPath samples: got %d, want %d", len(got), expectedSamples)
		} else {
			t.Logf("Collected samples for NetworkInstance_PolicyForwarding_Policy_Rule_ActionPath :\n%v", got)
		}
	})

	t.Run("NetworkInstance_PolicyForwarding_Policy_Rule_Action_DecapNetworkInstancePath", func(t *testing.T) {
		//t.Parallel()
		statePath := gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().Policy(pbrName).Rule(SeqID1).Action().DecapNetworkInstance()
		if got := gnmi.Collect(t, dut, statePath.State(), subscriptionDuration).Await(t); len(got) < expectedSamples {
			t.Errorf("NetworkInstance_PolicyForwarding_Policy_Rule_Action_DecapNetworkInstancePath samples: got %d, want %d", len(got), expectedSamples)
		} else {
			t.Logf("Collected samples for NetworkInstance_PolicyForwarding_Policy_Rule_Action_DecapNetworkInstancePath :\n%v", got)
		}
	})

	t.Run("NetworkInstance_PolicyForwarding_Policy_Rule_Action_DecapFallbackNetworkInstancePath", func(t *testing.T) {
		//t.Parallel()
		statePath := gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().Policy(pbrName).Rule(SeqID1).Action().DecapFallbackNetworkInstance()
		if got := gnmi.Collect(t, dut, statePath.State(), subscriptionDuration).Await(t); len(got) < expectedSamples {
			t.Errorf("NetworkInstance_PolicyForwarding_Policy_Rule_Action_DecapFallbackNetworkInstancePath samples: got %d, want %d", len(got), expectedSamples)
		} else {
			t.Logf("Collected samples for NetworkInstance_PolicyForwarding_Policy_Rule_Action_DecapFallbackNetworkInstancePath :\n%v", got)
		}
	})

	t.Run("NetworkInstance_PolicyForwarding_Policy_Rule_Action_PostDecapNetworkInstancePath", func(t *testing.T) {
		//t.Parallel()
		statePath := gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().Policy(pbrName).Rule(SeqID1).Action().PostDecapNetworkInstance()
		if got := gnmi.Collect(t, dut, statePath.State(), subscriptionDuration).Await(t); len(got) < expectedSamples {
			t.Errorf("NetworkInstance_PolicyForwarding_Policy_Rule_Action_PostDecapNetworkInstancePath samples: got %d, want %d", len(got), expectedSamples)
		} else {
			t.Logf("Collected samples for NetworkInstance_PolicyForwarding_Policy_Rule_Action_PostDecapNetworkInstancePath :\n%v", got)
		}
	})
}
