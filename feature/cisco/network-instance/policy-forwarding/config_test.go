package network_instance_test

import (
	"context"
	"testing"

	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
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

		r1 := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
		r1.SequenceId = ygot.Uint32(1)
		r1.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
			DscpSet: []uint8{*ygot.Uint8(16)},
		}
		r1.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String("TE")}

		p := oc.NetworkInstance_PolicyForwarding_Policy{}
		p.PolicyId = ygot.String(pbrName)
		// openconfig-network-instance:network-instances/network-instance/policy-forwarding/policies/policy/config/type
		p.Type = oc.Policy_Type_VRF_SELECTION_POLICY
		p.Rule = map[uint32]*oc.NetworkInstance_PolicyForwarding_Policy_Rule{1: &r1}

		policy := oc.NetworkInstance_PolicyForwarding{}
		policy.Policy = map[string]*oc.NetworkInstance_PolicyForwarding_Policy{pbrName: &p}

		t.Run("Replace", func(t *testing.T) {
			gnmi.Replace(t, dut, gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Config(), &policy)
		})

		t.Run("Delete", func(t *testing.T) {
			gnmi.Delete(t, dut, gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Policy(pbrName).Type().Config())
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

		r1 := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
		r1.SequenceId = ygot.Uint32(1)
		r1.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
			DscpSet: []uint8{*ygot.Uint8(16)},
		}
		r1.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String("TE")}

		p := oc.NetworkInstance_PolicyForwarding_Policy{}
		// openconfig-network-instance:network-instances/network-instance/policy-forwarding/policies/policy/policy-id
		p.PolicyId = ygot.String(pbrName)
		p.Type = oc.Policy_Type_VRF_SELECTION_POLICY
		p.Rule = map[uint32]*oc.NetworkInstance_PolicyForwarding_Policy_Rule{1: &r1}

		policy := oc.NetworkInstance_PolicyForwarding{}
		policy.Policy = map[string]*oc.NetworkInstance_PolicyForwarding_Policy{pbrName: &p}

		t.Run("Replace", func(t *testing.T) {
			gnmi.Replace(t, dut, gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Config(), &policy)
		})

		t.Run("Delete", func(t *testing.T) {
			gnmi.Delete(t, dut, gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Policy(pbrName).Rule(uint32(1)).Config())
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

		r1 := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
		// openconfig-network-instance:network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/sequence-id
		r1.SequenceId = ygot.Uint32(1)
		r1.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
			DscpSet: []uint8{*ygot.Uint8(16)},
		}
		r1.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String("TE")}

		p := oc.NetworkInstance_PolicyForwarding_Policy{}
		p.PolicyId = ygot.String(pbrName)
		p.Type = oc.Policy_Type_VRF_SELECTION_POLICY
		p.Rule = map[uint32]*oc.NetworkInstance_PolicyForwarding_Policy_Rule{1: &r1}

		policy := oc.NetworkInstance_PolicyForwarding{}
		policy.Policy = map[string]*oc.NetworkInstance_PolicyForwarding_Policy{pbrName: &p}

		t.Run("Replace", func(t *testing.T) {
			gnmi.Replace(t, dut, gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Config(), &policy)
		})

		t.Run("Delete", func(t *testing.T) {
			gnmi.Delete(t, dut, gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Policy(pbrName).Config())
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
		r1 := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
		r1.SequenceId = ygot.Uint32(1)
		r1.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String("TE")}

		// openconfig-network-instance:network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/l2/config/ethertype
		r1.L2 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_L2{
			Ethertype: oc.PacketMatchTypes_ETHERTYPE_ETHERTYPE_IPV4,
		}

		p := oc.NetworkInstance_PolicyForwarding_Policy{}
		p.PolicyId = ygot.String(pbrName)
		p.Type = oc.Policy_Type_VRF_SELECTION_POLICY
		p.Rule = map[uint32]*oc.NetworkInstance_PolicyForwarding_Policy_Rule{1: &r1}
		policy := oc.NetworkInstance_PolicyForwarding{}
		policy.Policy = map[string]*oc.NetworkInstance_PolicyForwarding_Policy{pbrName: &p}

		t.Run("Replace", func(t *testing.T) {
			gnmi.Replace(t, dut, gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Config(), &policy)
		})

		t.Run("Delete", func(t *testing.T) {
			gnmi.Delete(t, dut, gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Policy(pbrName).Config())
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
		r1 := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
		r1.SequenceId = ygot.Uint32(1)
		r1.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String("TE")}
		// openconfig-network-instance:network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv4/config/dscp-set
		r1.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
			DscpSet: []uint8{*ygot.Uint8(16)},
		}

		p := oc.NetworkInstance_PolicyForwarding_Policy{}
		p.PolicyId = ygot.String(pbrName)
		p.Type = oc.Policy_Type_VRF_SELECTION_POLICY
		p.Rule = map[uint32]*oc.NetworkInstance_PolicyForwarding_Policy_Rule{1: &r1}
		policy := oc.NetworkInstance_PolicyForwarding{}
		policy.Policy = map[string]*oc.NetworkInstance_PolicyForwarding_Policy{pbrName: &p}

		t.Run("Replace", func(t *testing.T) {
			gnmi.Replace(t, dut, gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Config(), &policy)
		})

		t.Run("Delete", func(t *testing.T) {
			gnmi.Delete(t, dut, gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Policy(pbrName).Rule(uint32(1)).Config())
		})
		gnmi.Delete(t, dut, gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Policy(pbrName).Config())
	})
}

func Test_Ipv6_Dscp_set(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	t.Log("Remove Flowspec Config")
	configToChange := "no flowspec \n"
	ctx := context.Background()
	util.GNMIWithText(ctx, t, dut, configToChange)

	t.Run("Testing openconfig-network-instance:network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv6/config/dscp-set", func(t *testing.T) {
		r1 := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
		r1.SequenceId = ygot.Uint32(1)
		r1.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String("TE")}
		// openconfig-network-instance:network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv6/config/dscp-set
		r1.Ipv6 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv6{
			DscpSet: []uint8{*ygot.Uint8(16)},
		}

		p := oc.NetworkInstance_PolicyForwarding_Policy{}
		p.PolicyId = ygot.String(pbrName)
		p.Type = oc.Policy_Type_VRF_SELECTION_POLICY
		p.Rule = map[uint32]*oc.NetworkInstance_PolicyForwarding_Policy_Rule{1: &r1}
		policy := oc.NetworkInstance_PolicyForwarding{}
		policy.Policy = map[string]*oc.NetworkInstance_PolicyForwarding_Policy{pbrName: &p}

		t.Run("Replace", func(t *testing.T) {
			gnmi.Replace(t, dut, gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Config(), &policy)
		})

		t.Run("Delete", func(t *testing.T) {
			gnmi.Delete(t, dut, gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Policy(pbrName).Config())
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
		r1 := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
		r1.SequenceId = ygot.Uint32(1)
		r1.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String("TE")}
		// openconfig-network-instance:network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv4/config/protocol
		r1.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
			Protocol: oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP,
		}

		p := oc.NetworkInstance_PolicyForwarding_Policy{}
		p.PolicyId = ygot.String(pbrName)
		p.Type = oc.Policy_Type_VRF_SELECTION_POLICY
		p.Rule = map[uint32]*oc.NetworkInstance_PolicyForwarding_Policy_Rule{1: &r1}
		policy := oc.NetworkInstance_PolicyForwarding{}
		policy.Policy = map[string]*oc.NetworkInstance_PolicyForwarding_Policy{pbrName: &p}

		t.Run("Replace", func(t *testing.T) {
			gnmi.Replace(t, dut, gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Config(), &policy)
		})

		t.Run("Delete", func(t *testing.T) {
			gnmi.Delete(t, dut, gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Policy(pbrName).Config())
		})
	})
}

func Test_Ipv6_Protocol(t *testing.T) {
	t.Skip() // The support for protocol leaf under IPv6 has been removed CSCwc29866
	dut := ondatra.DUT(t, "dut")

	t.Log("Remove Flowspec Config")
	configToChange := "no flowspec \n"
	ctx := context.Background()
	util.GNMIWithText(ctx, t, dut, configToChange)

	t.Run("Testing openconfig-network-instance:network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv6/config/protocol", func(t *testing.T) {
		r1 := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
		r1.SequenceId = ygot.Uint32(1)
		r1.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String("TE")}
		// openconfig-network-instance:network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv6/config/protocol
		r1.Ipv6 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv6{
			Protocol: oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP,
		}

		p := oc.NetworkInstance_PolicyForwarding_Policy{}
		p.PolicyId = ygot.String(pbrName)
		p.Type = oc.Policy_Type_VRF_SELECTION_POLICY
		p.Rule = map[uint32]*oc.NetworkInstance_PolicyForwarding_Policy_Rule{1: &r1}
		policy := oc.NetworkInstance_PolicyForwarding{}
		policy.Policy = map[string]*oc.NetworkInstance_PolicyForwarding_Policy{pbrName: &p}

		t.Run("Replace", func(t *testing.T) {
			gnmi.Replace(t, dut, gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Config(), &policy)
		})

		t.Run("Delete", func(t *testing.T) {
			gnmi.Delete(t, dut, gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Policy(pbrName).Config())
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
		r1 := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
		r1.SequenceId = ygot.Uint32(1)
		// openconfig-network-instance:network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/config/network-instance
		r1.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String("TE")}
		r1.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
			Protocol: oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP,
		}

		p := oc.NetworkInstance_PolicyForwarding_Policy{}
		p.PolicyId = ygot.String(pbrName)
		p.Type = oc.Policy_Type_VRF_SELECTION_POLICY
		p.Rule = map[uint32]*oc.NetworkInstance_PolicyForwarding_Policy_Rule{1: &r1}
		policy := oc.NetworkInstance_PolicyForwarding{}
		policy.Policy = map[string]*oc.NetworkInstance_PolicyForwarding_Policy{pbrName: &p}

		t.Run("Replace", func(t *testing.T) {
			gnmi.Replace(t, dut, gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Config(), &policy)
		})

		t.Run("Delete", func(t *testing.T) {
			gnmi.Delete(t, dut, gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Policy(pbrName).Config())
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
		r1 := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
		r1.SequenceId = ygot.Uint32(1)
		r1.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
			DscpSet: []uint8{*ygot.Uint8(16)},
		}
		r1.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String("TE")}

		p := oc.NetworkInstance_PolicyForwarding_Policy{}
		p.PolicyId = ygot.String(pbrName)
		p.Type = oc.Policy_Type_VRF_SELECTION_POLICY
		p.Rule = map[uint32]*oc.NetworkInstance_PolicyForwarding_Policy_Rule{1: &r1}

		policy := oc.NetworkInstance_PolicyForwarding{}
		policy.Policy = map[string]*oc.NetworkInstance_PolicyForwarding_Policy{pbrName: &p}

		gnmi.Replace(t, dut, gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Config(), &policy)
		// openconfig-network-instance:network-instances/network-instance/policy-forwarding/interfaces/interface/interface-id
		// openconfig-network-instance:network-instances/network-instance/policy-forwarding/interfaces/interface/config/apply-vrf-selection-policy
		t.Run("Replace", func(t *testing.T) {
			gnmi.Update(t, dut, gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Interface(InterfaceName).ApplyVrfSelectionPolicy().Config(), pbrName)
		})

		t.Run("Delete", func(t *testing.T) {
			//dut.Config().NetworkInstance("DEFAULT").PolicyForwarding().Delete(t)
			gnmi.Delete(t, dut, gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Interface(InterfaceName).Config())
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
		r1 := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
		r1.SequenceId = ygot.Uint32(1)
		r1.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
			DscpSet: []uint8{*ygot.Uint8(16)},
		}
		r1.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String("TE")}

		// openconfig-network-instance:network-instances/network-instance/policy-forwarding/interfaces/interface/interface-id
		r3 := &oc.NetworkInstance_PolicyForwarding_Interface{
			//ApplyForwardingPolicy: ygot.String("apply-vrf-selection-policy"),
			InterfaceId: ygot.String(InterfaceName),
		}

		p := oc.NetworkInstance_PolicyForwarding_Policy{}
		p.PolicyId = ygot.String(pbrName)
		p.Type = oc.Policy_Type_VRF_SELECTION_POLICY
		p.Rule = map[uint32]*oc.NetworkInstance_PolicyForwarding_Policy_Rule{1: &r1}

		policy := oc.NetworkInstance_PolicyForwarding{}
		var store = make(map[string]*oc.NetworkInstance_PolicyForwarding_Interface)
		store["id1"] = r3
		policy.Interface = store
		policy.Policy = map[string]*oc.NetworkInstance_PolicyForwarding_Policy{pbrName: &p}

		gnmi.Replace(t, dut, gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Config(), &policy)

		t.Run("Replace", func(t *testing.T) {
			gnmi.Update(t, dut, gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Interface(InterfaceName).ApplyVrfSelectionPolicy().Config(), pbrName)
		})

		t.Run("Delete", func(t *testing.T) {
			//dut.Config().NetworkInstance("DEFAULT").PolicyForwarding().Delete(t)
			gnmi.Delete(t, dut, gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Interface(InterfaceName).Config())
		})

	})

}
