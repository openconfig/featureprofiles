package network_instance_test

import (
	"context"
	"testing"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi/util"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	pbrName       = "PBR"
	InterfaceName = "Bundle-Ether1"
)

func Test_Type(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	t.Log("Remove Flowspec Config")
	configToChange := "no flowspec \n"
	ctx := context.Background()
	util.GNMIWithText(ctx, t, dut, configToChange)

	t.Run("Testing openconfig-network-instance:network-instances/network-instance/policy-forwarding/policies/policy/config/type", func(t *testing.T) {

		r1 := telemetry.NetworkInstance_PolicyForwarding_Policy_Rule{}
		r1.SequenceId = ygot.Uint32(1)
		r1.Ipv4 = &telemetry.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
			DscpSet: []uint8{*ygot.Uint8(16)},
		}
		r1.Action = &telemetry.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String("TE")}

		p := telemetry.NetworkInstance_PolicyForwarding_Policy{}
		p.PolicyId = ygot.String(pbrName)
		// openconfig-network-instance:network-instances/network-instance/policy-forwarding/policies/policy/config/type
		p.Type = telemetry.Policy_Type_VRF_SELECTION_POLICY
		p.Rule = map[uint32]*telemetry.NetworkInstance_PolicyForwarding_Policy_Rule{1: &r1}

		policy := telemetry.NetworkInstance_PolicyForwarding{}
		policy.Policy = map[string]*telemetry.NetworkInstance_PolicyForwarding_Policy{pbrName: &p}

		t.Run("Replace", func(t *testing.T) {
			dut.Config().NetworkInstance("default").PolicyForwarding().Replace(t, &policy)
		})

		t.Run("Delete", func(t *testing.T) {
			dut.Config().NetworkInstance("default").PolicyForwarding().Policy(pbrName).Type().Delete(t)
		})
	})
}

func Test_Policy_id(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	t.Log("Remove Flowspec Config")
	configToChange := "no flowspec \n"
	ctx := context.Background()
	util.GNMIWithText(ctx, t, dut, configToChange)

	t.Run("Testing openconfig-network-instance:network-instances/network-instance/policy-forwarding/policies/policy/policy-id", func(t *testing.T) {

		r1 := telemetry.NetworkInstance_PolicyForwarding_Policy_Rule{}
		r1.SequenceId = ygot.Uint32(1)
		r1.Ipv4 = &telemetry.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
			DscpSet: []uint8{*ygot.Uint8(16)},
		}
		r1.Action = &telemetry.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String("TE")}

		p := telemetry.NetworkInstance_PolicyForwarding_Policy{}
		// openconfig-network-instance:network-instances/network-instance/policy-forwarding/policies/policy/policy-id
		p.PolicyId = ygot.String(pbrName)
		p.Type = telemetry.Policy_Type_VRF_SELECTION_POLICY
		p.Rule = map[uint32]*telemetry.NetworkInstance_PolicyForwarding_Policy_Rule{1: &r1}

		policy := telemetry.NetworkInstance_PolicyForwarding{}
		policy.Policy = map[string]*telemetry.NetworkInstance_PolicyForwarding_Policy{pbrName: &p}

		t.Run("Replace", func(t *testing.T) {
			dut.Config().NetworkInstance("default").PolicyForwarding().Replace(t, &policy)
		})

		t.Run("Delete", func(t *testing.T) {
			dut.Config().NetworkInstance("default").PolicyForwarding().Policy(pbrName).Rule(uint32(1)).Delete(t)
		})
	})
}

func Test_Sequence_id(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	t.Log("Remove Flowspec Config")
	configToChange := "no flowspec \n"
	ctx := context.Background()
	util.GNMIWithText(ctx, t, dut, configToChange)

	t.Run("Testing openconfig-network-instance:network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/sequence-id", func(t *testing.T) {

		r1 := telemetry.NetworkInstance_PolicyForwarding_Policy_Rule{}
		// openconfig-network-instance:network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/sequence-id
		r1.SequenceId = ygot.Uint32(1)
		r1.Ipv4 = &telemetry.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
			DscpSet: []uint8{*ygot.Uint8(16)},
		}
		r1.Action = &telemetry.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String("TE")}

		p := telemetry.NetworkInstance_PolicyForwarding_Policy{}
		p.PolicyId = ygot.String(pbrName)
		p.Type = telemetry.Policy_Type_VRF_SELECTION_POLICY
		p.Rule = map[uint32]*telemetry.NetworkInstance_PolicyForwarding_Policy_Rule{1: &r1}

		policy := telemetry.NetworkInstance_PolicyForwarding{}
		policy.Policy = map[string]*telemetry.NetworkInstance_PolicyForwarding_Policy{pbrName: &p}

		t.Run("Replace", func(t *testing.T) {
			dut.Config().NetworkInstance("default").PolicyForwarding().Replace(t, &policy)
		})

		t.Run("Delete", func(t *testing.T) {
			dut.Config().NetworkInstance("default").PolicyForwarding().Policy(pbrName).Delete(t)
		})
	})
}

func Test_Ethertype(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	t.Log("Remove Flowspec Config")
	configToChange := "no flowspec \n"
	ctx := context.Background()
	util.GNMIWithText(ctx, t, dut, configToChange)

	t.Run("Testing openconfig-network-instance:network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/l2/config/ethertype", func(t *testing.T) {
		r1 := telemetry.NetworkInstance_PolicyForwarding_Policy_Rule{}
		r1.SequenceId = ygot.Uint32(1)
		r1.Action = &telemetry.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String("TE")}

		// openconfig-network-instance:network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/l2/config/ethertype
		r1.L2 = &telemetry.NetworkInstance_PolicyForwarding_Policy_Rule_L2{
			Ethertype: telemetry.PacketMatchTypes_ETHERTYPE_ETHERTYPE_IPV4,
		}

		p := telemetry.NetworkInstance_PolicyForwarding_Policy{}
		p.PolicyId = ygot.String(pbrName)
		p.Type = telemetry.Policy_Type_VRF_SELECTION_POLICY
		p.Rule = map[uint32]*telemetry.NetworkInstance_PolicyForwarding_Policy_Rule{1: &r1}
		policy := telemetry.NetworkInstance_PolicyForwarding{}
		policy.Policy = map[string]*telemetry.NetworkInstance_PolicyForwarding_Policy{pbrName: &p}

		t.Run("Replace", func(t *testing.T) {
			dut.Config().NetworkInstance("default").PolicyForwarding().Replace(t, &policy)
		})

		t.Run("Delete", func(t *testing.T) {
			dut.Config().NetworkInstance("default").PolicyForwarding().Policy(pbrName).Delete(t)
		})
	})
}

func Test_Ipv4_Dscp_set(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	t.Log("Remove Flowspec Config")
	configToChange := "no flowspec \n"
	ctx := context.Background()
	util.GNMIWithText(ctx, t, dut, configToChange)

	t.Run("Testing openconfig-network-instance:network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv4/config/dscp-set", func(t *testing.T) {
		r1 := telemetry.NetworkInstance_PolicyForwarding_Policy_Rule{}
		r1.SequenceId = ygot.Uint32(1)
		r1.Action = &telemetry.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String("TE")}
		// openconfig-network-instance:network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv4/config/dscp-set
		r1.Ipv4 = &telemetry.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
			DscpSet: []uint8{*ygot.Uint8(16)},
		}

		p := telemetry.NetworkInstance_PolicyForwarding_Policy{}
		p.PolicyId = ygot.String(pbrName)
		p.Type = telemetry.Policy_Type_VRF_SELECTION_POLICY
		p.Rule = map[uint32]*telemetry.NetworkInstance_PolicyForwarding_Policy_Rule{1: &r1}
		policy := telemetry.NetworkInstance_PolicyForwarding{}
		policy.Policy = map[string]*telemetry.NetworkInstance_PolicyForwarding_Policy{pbrName: &p}

		t.Run("Replace", func(t *testing.T) {
			dut.Config().NetworkInstance("default").PolicyForwarding().Replace(t, &policy)
		})

		t.Run("Delete", func(t *testing.T) {
			dut.Config().NetworkInstance("default").PolicyForwarding().Policy(pbrName).Rule(uint32(1)).Delete(t)
		})
		dut.Config().NetworkInstance("default").PolicyForwarding().Policy(pbrName).Delete(t)
	})
}

func Test_Ipv6_Dscp_set(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	t.Log("Remove Flowspec Config")
	configToChange := "no flowspec \n"
	ctx := context.Background()
	util.GNMIWithText(ctx, t, dut, configToChange)

	t.Run("Testing openconfig-network-instance:network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv6/config/dscp-set", func(t *testing.T) {
		r1 := telemetry.NetworkInstance_PolicyForwarding_Policy_Rule{}
		r1.SequenceId = ygot.Uint32(1)
		r1.Action = &telemetry.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String("TE")}
		// openconfig-network-instance:network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv6/config/dscp-set
		r1.Ipv6 = &telemetry.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv6{
			DscpSet: []uint8{*ygot.Uint8(16)},
		}

		p := telemetry.NetworkInstance_PolicyForwarding_Policy{}
		p.PolicyId = ygot.String(pbrName)
		p.Type = telemetry.Policy_Type_VRF_SELECTION_POLICY
		p.Rule = map[uint32]*telemetry.NetworkInstance_PolicyForwarding_Policy_Rule{1: &r1}
		policy := telemetry.NetworkInstance_PolicyForwarding{}
		policy.Policy = map[string]*telemetry.NetworkInstance_PolicyForwarding_Policy{pbrName: &p}

		t.Run("Replace", func(t *testing.T) {
			dut.Config().NetworkInstance("default").PolicyForwarding().Replace(t, &policy)
		})

		t.Run("Delete", func(t *testing.T) {
			dut.Config().NetworkInstance("default").PolicyForwarding().Policy(pbrName).Delete(t)
		})
	})
}

func Test_Ipv4_Protocol(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	t.Log("Remove Flowspec Config")
	configToChange := "no flowspec \n"
	ctx := context.Background()
	util.GNMIWithText(ctx, t, dut, configToChange)

	t.Run("Testing openconfig-network-instance:network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv4/config/protocol", func(t *testing.T) {
		r1 := telemetry.NetworkInstance_PolicyForwarding_Policy_Rule{}
		r1.SequenceId = ygot.Uint32(1)
		r1.Action = &telemetry.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String("TE")}
		// openconfig-network-instance:network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv4/config/protocol
		r1.Ipv4 = &telemetry.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
			Protocol: telemetry.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP,
		}

		p := telemetry.NetworkInstance_PolicyForwarding_Policy{}
		p.PolicyId = ygot.String(pbrName)
		p.Type = telemetry.Policy_Type_VRF_SELECTION_POLICY
		p.Rule = map[uint32]*telemetry.NetworkInstance_PolicyForwarding_Policy_Rule{1: &r1}
		policy := telemetry.NetworkInstance_PolicyForwarding{}
		policy.Policy = map[string]*telemetry.NetworkInstance_PolicyForwarding_Policy{pbrName: &p}

		t.Run("Replace", func(t *testing.T) {
			dut.Config().NetworkInstance("default").PolicyForwarding().Replace(t, &policy)
		})

		t.Run("Delete", func(t *testing.T) {
			dut.Config().NetworkInstance("default").PolicyForwarding().Policy(pbrName).Delete(t)
		})
	})
}

func Test_Ipv6_Protocol(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	t.Log("Remove Flowspec Config")
	configToChange := "no flowspec \n"
	ctx := context.Background()
	util.GNMIWithText(ctx, t, dut, configToChange)

	t.Run("Testing openconfig-network-instance:network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv6/config/protocol", func(t *testing.T) {
		r1 := telemetry.NetworkInstance_PolicyForwarding_Policy_Rule{}
		r1.SequenceId = ygot.Uint32(1)
		r1.Action = &telemetry.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String("TE")}
		// openconfig-network-instance:network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv6/config/protocol
		r1.Ipv6 = &telemetry.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv6{
			Protocol: telemetry.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP,
		}

		p := telemetry.NetworkInstance_PolicyForwarding_Policy{}
		p.PolicyId = ygot.String(pbrName)
		p.Type = telemetry.Policy_Type_VRF_SELECTION_POLICY
		p.Rule = map[uint32]*telemetry.NetworkInstance_PolicyForwarding_Policy_Rule{1: &r1}
		policy := telemetry.NetworkInstance_PolicyForwarding{}
		policy.Policy = map[string]*telemetry.NetworkInstance_PolicyForwarding_Policy{pbrName: &p}

		t.Run("Replace", func(t *testing.T) {
			dut.Config().NetworkInstance("default").PolicyForwarding().Replace(t, &policy)
		})

		t.Run("Delete", func(t *testing.T) {
			dut.Config().NetworkInstance("default").PolicyForwarding().Policy(pbrName).Delete(t)
		})
	})
}

func Test_Network_instance(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	t.Log("Remove Flowspec Config")
	configToChange := "no flowspec \n"
	ctx := context.Background()
	util.GNMIWithText(ctx, t, dut, configToChange)

	t.Run("Testing openconfig-network-instance:network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/config/network-instance", func(t *testing.T) {
		r1 := telemetry.NetworkInstance_PolicyForwarding_Policy_Rule{}
		r1.SequenceId = ygot.Uint32(1)
		// openconfig-network-instance:network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/config/network-instance
		r1.Action = &telemetry.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String("TE")}
		r1.Ipv6 = &telemetry.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv6{
			Protocol: telemetry.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP,
		}

		p := telemetry.NetworkInstance_PolicyForwarding_Policy{}
		p.PolicyId = ygot.String(pbrName)
		p.Type = telemetry.Policy_Type_VRF_SELECTION_POLICY
		p.Rule = map[uint32]*telemetry.NetworkInstance_PolicyForwarding_Policy_Rule{1: &r1}
		policy := telemetry.NetworkInstance_PolicyForwarding{}
		policy.Policy = map[string]*telemetry.NetworkInstance_PolicyForwarding_Policy{pbrName: &p}

		t.Run("Replace", func(t *testing.T) {
			dut.Config().NetworkInstance("default").PolicyForwarding().Replace(t, &policy)
		})

		t.Run("Delete", func(t *testing.T) {
			dut.Config().NetworkInstance("default").PolicyForwarding().Policy(pbrName).Delete(t)
		})
	})
}

func Test_Interface_ApplyVrfSelectionPolicy(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	t.Log("Remove Flowspec Config")
	configToChange := "no flowspec \n"
	ctx := context.Background()
	util.GNMIWithText(ctx, t, dut, configToChange)

	t.Run("Testing openconfig-network-instance:network-instances/network-instance/policy-forwarding/interfaces/interface/config/apply-vrf-selection-policy", func(t *testing.T) {
		r1 := telemetry.NetworkInstance_PolicyForwarding_Policy_Rule{}
		r1.SequenceId = ygot.Uint32(1)
		r1.Ipv4 = &telemetry.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
			DscpSet: []uint8{*ygot.Uint8(16)},
		}
		r1.Action = &telemetry.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String("TE")}

		p := telemetry.NetworkInstance_PolicyForwarding_Policy{}
		p.PolicyId = ygot.String(pbrName)
		p.Type = telemetry.Policy_Type_VRF_SELECTION_POLICY
		p.Rule = map[uint32]*telemetry.NetworkInstance_PolicyForwarding_Policy_Rule{1: &r1}

		policy := telemetry.NetworkInstance_PolicyForwarding{}
		policy.Policy = map[string]*telemetry.NetworkInstance_PolicyForwarding_Policy{pbrName: &p}

		dut.Config().NetworkInstance("default").PolicyForwarding().Replace(t, &policy)
		// openconfig-network-instance:network-instances/network-instance/policy-forwarding/interfaces/interface/interface-id
		// openconfig-network-instance:network-instances/network-instance/policy-forwarding/interfaces/interface/config/apply-vrf-selection-policy
		t.Run("Replace", func(t *testing.T) {
			dut.Config().NetworkInstance("default").PolicyForwarding().Interface(InterfaceName).ApplyVrfSelectionPolicy().Update(t, pbrName)
		})

		t.Run("Delete", func(t *testing.T) {
			//dut.Config().NetworkInstance("default").PolicyForwarding().Delete(t)
			dut.Config().NetworkInstance("default").PolicyForwarding().Interface(InterfaceName).Delete(t)
		})
	})
}

func Test_Interface_InterfaceId(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	t.Log("Remove Flowspec Config")
	configToChange := "no flowspec \n"
	ctx := context.Background()
	util.GNMIWithText(ctx, t, dut, configToChange)

	t.Run("Testing openconfig-network-instance:network-instances/network-instance/policy-forwarding/interfaces/interface/interface-id", func(t *testing.T) {
		r1 := telemetry.NetworkInstance_PolicyForwarding_Policy_Rule{}
		r1.SequenceId = ygot.Uint32(1)
		r1.Ipv4 = &telemetry.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
			DscpSet: []uint8{*ygot.Uint8(16)},
		}
		r1.Action = &telemetry.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String("TE")}

		// openconfig-network-instance:network-instances/network-instance/policy-forwarding/interfaces/interface/interface-id
		r3 := &telemetry.NetworkInstance_PolicyForwarding_Interface{
			//ApplyForwardingPolicy: ygot.String("apply-vrf-selection-policy"),
			InterfaceId: ygot.String(InterfaceName),
		}

		p := telemetry.NetworkInstance_PolicyForwarding_Policy{}
		p.PolicyId = ygot.String(pbrName)
		p.Type = telemetry.Policy_Type_VRF_SELECTION_POLICY
		p.Rule = map[uint32]*telemetry.NetworkInstance_PolicyForwarding_Policy_Rule{1: &r1}

		policy := telemetry.NetworkInstance_PolicyForwarding{}
		var store = make(map[string]*telemetry.NetworkInstance_PolicyForwarding_Interface)
		store["id1"] = r3
		policy.Interface = store
		policy.Policy = map[string]*telemetry.NetworkInstance_PolicyForwarding_Policy{pbrName: &p}

		dut.Config().NetworkInstance("default").PolicyForwarding().Replace(t, &policy)

		t.Run("Replace", func(t *testing.T) {
			dut.Config().NetworkInstance("default").PolicyForwarding().Interface(InterfaceName).ApplyVrfSelectionPolicy().Update(t, pbrName)
		})

		t.Run("Delete", func(t *testing.T) {
			//dut.Config().NetworkInstance("default").PolicyForwarding().Delete(t)
			dut.Config().NetworkInstance("default").PolicyForwarding().Interface(InterfaceName).Delete(t)
		})

	})

}
