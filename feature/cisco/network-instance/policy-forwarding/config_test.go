package network_instance_test

import (
	"context"
	"testing"

	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	pbrName       = "PBR"
	InterfaceName = "Bundle-Ether1"
	SeqID1        = 1
	SeqID2        = 2
	SeqID3        = 3
	vrfBLUE       = "BLUE"
	vrfRED        = "RED"
	vrfGREEN      = "GREEN"
	vrfORANGE     = "ORANGE"
	vrfBROWN      = "BROWN"
	vrfPINK       = "PINK"
	vrfINDIGO     = "INDIGO"
	vrfBLACK      = "BLACK"
	vrfPURPLE     = "PURPLE"
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

		t.Run("Get-State", func(t *testing.T) {
			state := gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().Policy(pbrName)
			stateGot := gnmi.Get(t, dut, state.State())
			policyType := oc.Policy_Type_VRF_SELECTION_POLICY
			if stateGot.Type != oc.Policy_Type_VRF_SELECTION_POLICY {
				t.Errorf("Failed: Fetching state leaf for Policy type, got %v, want %v",
					stateGot.Type, policyType)
			} else {
				t.Logf("Passed: Configured Policy type = %v", stateGot.Type)
			}
		})

		t.Run("Delete", func(t *testing.T) {
			gnmi.Delete(t, dut, gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Policy(pbrName).Config())
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

		t.Run("Get-State", func(t *testing.T) {
			state := gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().Policy(pbrName)
			stateGot := gnmi.Get(t, dut, state.State())
			if *(stateGot.PolicyId) != pbrName {
				t.Errorf("Failed: Fetching state leaf for Policy ID, got %v, want %v",
					*(stateGot.PolicyId), pbrName)
			} else {
				t.Logf("Passed: Configured Policy ID = %v", *(stateGot.PolicyId))
			}
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

		t.Run("Get-State", func(t *testing.T) {
			state := gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().Policy(pbrName).Rule(1)
			stateGot := gnmi.Get(t, dut, state.State())
			var sequenceId uint32 = 1
			if *(stateGot.SequenceId) != sequenceId {
				t.Errorf("Failed: Fetching state leaf for Policy Sequence-ID, got %v, want %v",
					*(stateGot.SequenceId), sequenceId)
			} else {
				t.Logf("Passed: Configured Policy Sequence-ID = %v", *(stateGot.SequenceId))
			}
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

func Test_Policy_Type_PBR_POLICY(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	t.Log("Remove Flowspec Config")
	configToChange := "no flowspec \n"
	ctx := context.Background()
	util.GNMIWithText(ctx, t, dut, configToChange)
	t.Run("Testing openconfig-network-instance:network-instances/network-instance/policy-forwarding/policies/policy/config/type", func(t *testing.T) {
		r1 := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
		r1.SequenceId = ygot.Uint32(1)
		r1.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{DecapsulateGre: ygot.Bool(true)}
		r1.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
			Protocol: oc.PacketMatchTypes_IP_PROTOCOL_IP_UDP,
		}

		p := oc.NetworkInstance_PolicyForwarding_Policy{}
		p.PolicyId = ygot.String(pbrName)
		p.Type = oc.Policy_Type_PBR_POLICY
		p.Rule = map[uint32]*oc.NetworkInstance_PolicyForwarding_Policy_Rule{1: &r1}
		policy := oc.NetworkInstance_PolicyForwarding{}
		policy.Policy = map[string]*oc.NetworkInstance_PolicyForwarding_Policy{pbrName: &p}

		t.Run("Update", func(t *testing.T) {
			gnmi.Replace(t, dut, gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Config(), &policy)
		})

		t.Run("Get", func(t *testing.T) {
			config := gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Policy(pbrName).Type()
			configGot := oc.E_Policy_Type(gnmi.GetConfig(t, dut, config.Config()))

			if configGot != oc.Policy_Type_PBR_POLICY {
				t.Errorf("Failed: Fetching leaf for policy-type got %v, want %v", configGot, oc.Policy_Type_PBR_POLICY)
			} else {
				t.Logf("Passed: Configured policy-type = Obtained policy-type = %v", configGot)
			}
		})

		t.Run("Get-State", func(t *testing.T) {
			state := gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().Policy(pbrName)
			stateGot := gnmi.Get(t, dut, state.State())
			policyType := oc.Policy_Type_PBR_POLICY
			if stateGot.Type != oc.Policy_Type_PBR_POLICY {
				t.Errorf("Failed: Fetching state leaf for Policy type, got %v, want %v",
					stateGot.Type, policyType)
			} else {
				t.Logf("Passed: Configured Policy type = %v", stateGot.Type)
			}
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

		t.Run("Get-State", func(t *testing.T) {
			state := gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().Policy(pbrName).Rule(1)
			stateGot := gnmi.Get(t, dut, state.State())
			var ipv4DscpSet uint8 = 16
			if stateGot.Ipv4.DscpSet[0] != ipv4DscpSet {
				t.Errorf("Failed: Fetching state leaf for IPv4 Dscp-Set, got %v, want %v",
					stateGot.Ipv4.DscpSet[0], ipv4DscpSet)
			} else {
				t.Logf("Passed: Configured IPv4 Dscp-Set = %v", stateGot.Ipv4.DscpSet[0])
			}
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

		t.Run("Get-State", func(t *testing.T) {
			state := gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().Policy(pbrName).Rule(1)
			stateGot := gnmi.Get(t, dut, state.State())
			var ipv6DscpSet uint8 = 16
			if stateGot.Ipv6.DscpSet[0] != ipv6DscpSet {
				t.Errorf("Failed: Fetching state leaf for IPv6 Dscp-Set, got %v, want %v",
					stateGot.Ipv6.DscpSet[0], ipv6DscpSet)
			} else {
				t.Logf("Passed: Configured IPv6 Dscp-Set = %v", stateGot.Ipv6.DscpSet[0])
			}
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

func Test_Ipv4_Protocol_Ip_Gre(t *testing.T) {
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
			Protocol: oc.PacketMatchTypes_IP_PROTOCOL_IP_GRE,
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

func Test_Ipv4_Protocol_Ip_Udp(t *testing.T) {
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
			Protocol: oc.PacketMatchTypes_IP_PROTOCOL_IP_UDP,
		}

		p := oc.NetworkInstance_PolicyForwarding_Policy{}
		p.PolicyId = ygot.String(pbrName)
		p.Type = oc.Policy_Type_VRF_SELECTION_POLICY
		p.Rule = map[uint32]*oc.NetworkInstance_PolicyForwarding_Policy_Rule{1: &r1}
		policy := oc.NetworkInstance_PolicyForwarding{}
		policy.Policy = map[string]*oc.NetworkInstance_PolicyForwarding_Policy{pbrName: &p}

		t.Logf("Update policy with ipv4 protocol IP_UDP")
		t.Run("Update", func(t *testing.T) {
			gnmi.Replace(t, dut, gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Config(), &policy)
		})

		t.Logf("Replace ipv4 protocol IP_UDP with IP_IN_IP")
		r1.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
			Protocol: oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP,
		}
		p.Rule = map[uint32]*oc.NetworkInstance_PolicyForwarding_Policy_Rule{1: &r1}
		policy.Policy = map[string]*oc.NetworkInstance_PolicyForwarding_Policy{pbrName: &p}

		t.Run("Replace", func(t *testing.T) {
			gnmi.Replace(t, dut, gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Config(), &policy)
		})

		t.Logf("Replace ipv4 protocol IP_IN_IP with IP_UDP")
		r1.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
			Protocol: oc.PacketMatchTypes_IP_PROTOCOL_IP_UDP,
		}
		p.Rule = map[uint32]*oc.NetworkInstance_PolicyForwarding_Policy_Rule{1: &r1}
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

func Test_Ipv6_Protocol_Ip_Gre(t *testing.T) {
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
			Protocol: oc.PacketMatchTypes_IP_PROTOCOL_IP_GRE,
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

func Test_Ipv6_Protocol_Ip_Udp(t *testing.T) {
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
			Protocol: oc.PacketMatchTypes_IP_PROTOCOL_IP_UDP,
		}

		p := oc.NetworkInstance_PolicyForwarding_Policy{}
		p.PolicyId = ygot.String(pbrName)
		p.Type = oc.Policy_Type_VRF_SELECTION_POLICY
		p.Rule = map[uint32]*oc.NetworkInstance_PolicyForwarding_Policy_Rule{1: &r1}
		policy := oc.NetworkInstance_PolicyForwarding{}
		policy.Policy = map[string]*oc.NetworkInstance_PolicyForwarding_Policy{pbrName: &p}

		t.Logf("Update policy with ipv6 protocol IP_UDP")
		t.Run("Update", func(t *testing.T) {
			gnmi.Replace(t, dut, gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Config(), &policy)
		})

		t.Logf("Replace ipv6 protocol IP_UDP with IP_IN_IP")
		r1.Ipv6 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv6{
			Protocol: oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP,
		}
		p.Rule = map[uint32]*oc.NetworkInstance_PolicyForwarding_Policy_Rule{1: &r1}
		policy.Policy = map[string]*oc.NetworkInstance_PolicyForwarding_Policy{pbrName: &p}

		t.Run("Replace", func(t *testing.T) {
			gnmi.Replace(t, dut, gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Config(), &policy)
		})

		t.Logf("Replace ipv6 protocol IP_IN_IP with IP_UDP")
		r1.Ipv6 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv6{
			Protocol: oc.PacketMatchTypes_IP_PROTOCOL_IP_UDP,
		}
		p.Rule = map[uint32]*oc.NetworkInstance_PolicyForwarding_Policy_Rule{1: &r1}
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

		t.Run("Get-State", func(t *testing.T) {
			state := gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().Policy(pbrName).Rule(1)
			stateGot := gnmi.Get(t, dut, state.State())
			actionNetworkInstance := "TE"
			if *(stateGot.Action.NetworkInstance) != actionNetworkInstance {
				t.Errorf("Failed: Fetching state leaf for Action Network-Instance, got %v, want %v",
					*(stateGot.Action.NetworkInstance), actionNetworkInstance)
			} else {
				t.Logf("Passed: Configured Action Network-Instance = %v", *(stateGot.Action.NetworkInstance))
			}
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
			gnmi.Delete(t, dut, gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Interface(InterfaceName).Config())
			gnmi.Delete(t, dut, gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Policy(pbrName).Config())
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
			gnmi.Delete(t, dut, gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Interface(InterfaceName).Config())
			gnmi.Delete(t, dut, gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Policy(pbrName).Config())
		})

	})

}

func Test_Ipv4_Source_Addr(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	t.Log("Remove Flowspec Config")
	configToChange := "no flowspec \n"
	ctx := context.Background()
	util.GNMIWithText(ctx, t, dut, configToChange)
	t.Run("Testing openconfig-network-instance:network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv4/config/source-address", func(t *testing.T) {
		r1 := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
		r1.SequenceId = ygot.Uint32(1)
		r1.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String("TE")}
		// openconfig-network-instance:network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv4/config/source-address
		r1.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
			SourceAddress: ygot.String("1.1.1.1/32"),
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

		t.Run("Get-State", func(t *testing.T) {
			state := gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().Policy(pbrName).Rule(1)
			stateGot := gnmi.Get(t, dut, state.State())
			ipv4SourceAddress := "1.1.1.1/32"
			if *(stateGot.Ipv4.SourceAddress) != ipv4SourceAddress {
				t.Errorf("Failed: Fetching state leaf for IPv4 Source Address, got %v, want %v",
					*(stateGot.Ipv4.SourceAddress), ipv4SourceAddress)
			} else {
				t.Logf("Passed: Configured IPv4 Source Address = %v", *(stateGot.Ipv4.SourceAddress))
			}
		})

		t.Run("Delete", func(t *testing.T) {
			gnmi.Delete(t, dut, gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Policy(pbrName).Config())
		})
	})
}

func Test_Ipv4_Destination_Address(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	t.Log("Remove Flowspec Config")
	configToChange := "no flowspec \n"
	ctx := context.Background()
	util.GNMIWithText(ctx, t, dut, configToChange)
	t.Run("Testing openconfig-network-instance:network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv4/config/destination-address", func(t *testing.T) {
		r1 := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
		r1.SequenceId = ygot.Uint32(1)
		r1.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String("TE")}
		// openconfig-network-instance:network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv4/config/destination-address
		r1.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
			DestinationAddress: ygot.String("2.2.2.2/32"),
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

		t.Log("Verify after update")
		t.Run("Get", func(t *testing.T) {
			config := gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Policy(pbrName).Rule(uint32(1)).Ipv4().DestinationAddress()
			configGot := gnmi.GetConfig(t, dut, config.Config())

			ruleIpv4 := oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{}
			ruleIpv4.DestinationAddress = ygot.String("2.2.2.2/32")

			if configGot != *ruleIpv4.DestinationAddress {
				t.Errorf("Failed: Fetching leaf for ipv4 destination-address, got %v, want %v",
					configGot, *ruleIpv4.DestinationAddress)
			} else {
				t.Logf("Passed: Configured ipv4 destination-address = %v", configGot)
			}
		})

		t.Run("Get-State", func(t *testing.T) {
			state := gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().Policy(pbrName).Rule(1)
			stateGot := gnmi.Get(t, dut, state.State())
			ipv4DestinationAddress := "2.2.2.2/32"
			if *(stateGot.Ipv4.DestinationAddress) != ipv4DestinationAddress {
				t.Errorf("Failed: Fetching state leaf for IPv4 Destination Address, got %v, want %v",
					*(stateGot.Ipv4.DestinationAddress), ipv4DestinationAddress)
			} else {
				t.Logf("Passed: Configured IPv4 Destination Address = %v", *(stateGot.Ipv4.DestinationAddress))
			}
		})

		t.Run("Delete", func(t *testing.T) {
			gnmi.Delete(t, dut, gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Policy(pbrName).Config())
		})
	})
}

func Test_Ipv6_Source_Addr(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	t.Log("Remove Flowspec Config")
	configToChange := "no flowspec \n"
	ctx := context.Background()
	util.GNMIWithText(ctx, t, dut, configToChange)
	t.Run("Testing openconfig-network-instance:network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv6/config/source-address", func(t *testing.T) {
		r1 := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
		r1.SequenceId = ygot.Uint32(1)
		r1.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String("TE")}
		// openconfig-network-instance:network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv6/config/source-address
		r1.Ipv6 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv6{
			SourceAddress: ygot.String("1000::1/128"),
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

		t.Log("Verify after update")
		t.Run("Get", func(t *testing.T) {
			config := gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Policy(pbrName).Rule(uint32(1)).Ipv6().SourceAddress()
			configGot := gnmi.GetConfig(t, dut, config.Config())

			ruleIpv6 := oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv6{}
			ruleIpv6.SourceAddress = ygot.String("1000::1/128")

			if configGot != *ruleIpv6.SourceAddress {
				t.Errorf("Failed: Fetching leaf for ipv6 source-address, got %v, want %v", configGot, *ruleIpv6.SourceAddress)
			} else {
				t.Logf("Passed: Configured ipv6 source-address = %v", configGot)
			}
		})

		t.Run("Get-State", func(t *testing.T) {
			state := gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().Policy(pbrName).Rule(1)
			stateGot := gnmi.Get(t, dut, state.State())
			ipv6SourceAddress := "1000::1/128"
			if *(stateGot.Ipv6.SourceAddress) != ipv6SourceAddress {
				t.Errorf("Failed: Fetching state leaf for IPv6 Source Address, got %v, want %v",
					*(stateGot.Ipv6.SourceAddress), ipv6SourceAddress)
			} else {
				t.Logf("Passed: Configured IPv6 Source Address = %v", *(stateGot.Ipv6.SourceAddress))
			}
		})

		t.Run("Delete", func(t *testing.T) {
			gnmi.Delete(t, dut, gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Policy(pbrName).Config())
		})
	})
}

func Test_Ipv6_Destination_Address(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	t.Log("Remove Flowspec Config")
	configToChange := "no flowspec \n"
	ctx := context.Background()
	util.GNMIWithText(ctx, t, dut, configToChange)
	t.Run("Testing openconfig-network-instance:network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv6/config/destination-address", func(t *testing.T) {
		r1 := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
		r1.SequenceId = ygot.Uint32(1)
		r1.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String("TE")}
		// openconfig-network-instance:network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv6/config/destination-address
		r1.Ipv6 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv6{
			DestinationAddress: ygot.String("2000::1/128"),
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

		t.Log("Verify after update")
		t.Run("Get", func(t *testing.T) {
			config := gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Policy(pbrName).Rule(uint32(1)).Ipv6().DestinationAddress()
			configGot := gnmi.GetConfig(t, dut, config.Config())

			ruleIpv6 := oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv6{}
			ruleIpv6.DestinationAddress = ygot.String("2000::1/128")

			if configGot != *ruleIpv6.DestinationAddress {
				t.Errorf("Failed: Fetching leaf for ipv6 destination-address, got %v, want %v",
					configGot, *ruleIpv6.DestinationAddress)
			} else {
				t.Logf("Passed: Configured ipv6 destination-address = %v", configGot)
			}
		})

		t.Run("Get-State", func(t *testing.T) {
			state := gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().Policy(pbrName).Rule(1)
			stateGot := gnmi.Get(t, dut, state.State())
			ipv6DestinationAddress := "2000::1/128"
			if *(stateGot.Ipv6.DestinationAddress) != ipv6DestinationAddress {
				t.Errorf("Failed: Fetching state leaf for IPv6 Destination Address, got %v, want %v",
					*(stateGot.Ipv6.DestinationAddress), ipv6DestinationAddress)
			} else {
				t.Logf("Passed: Configured IPv6 Destination Address = %v", *(stateGot.Ipv6.DestinationAddress))
			}
		})

		t.Run("Delete", func(t *testing.T) {
			gnmi.Delete(t, dut, gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Policy(pbrName).Config())
		})
	})
}

func Test_Source_Port(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	t.Log("Remove Flowspec Config")
	configToChange := "no flowspec \n"
	ctx := context.Background()
	util.GNMIWithText(ctx, t, dut, configToChange)

	t.Run("Testing openconfig-network-instance:network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/transport/config/source-port", func(t *testing.T) {
		r1 := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
		r1.SequenceId = ygot.Uint32(1)
		r1.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String("TE")}
		// openconfig-network-instance:network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/transport/config/source-port
		r1.L2 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_L2{
			Ethertype: oc.PacketMatchTypes_ETHERTYPE_ETHERTYPE_IPV4,
		}
		r1.Transport = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Transport{
			SourcePort: oc.UnionString("100-1000"),
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

func Test_Destination_Port(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	t.Log("Remove Flowspec Config")
	configToChange := "no flowspec \n"
	ctx := context.Background()
	util.GNMIWithText(ctx, t, dut, configToChange)

	t.Run("Testing openconfig-network-instance:network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/transport/config/destination-port", func(t *testing.T) {
		r1 := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
		r1.SequenceId = ygot.Uint32(1)
		r1.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String("TE")}
		// openconfig-network-instance:network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/transport/config/destination-port
		r1.L2 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_L2{
			Ethertype: oc.PacketMatchTypes_ETHERTYPE_ETHERTYPE_IPV4,
		}
		r1.Transport = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Transport{
			DestinationPort: oc.UnionString("200-2000"),
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

func Test_Discard(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	t.Log("Remove Flowspec Config")
	configToChange := "no flowspec \n"
	ctx := context.Background()
	util.GNMIWithText(ctx, t, dut, configToChange)
	t.Run("Testing openconfig-network-instance:network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/config/discard", func(t *testing.T) {
		r1 := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
		r1.SequenceId = ygot.Uint32(1)
		r1.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{Discard: ygot.Bool(true)}
		r1.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
			Protocol: oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP,
		}

		p := oc.NetworkInstance_PolicyForwarding_Policy{}
		p.PolicyId = ygot.String(pbrName)
		p.Type = oc.Policy_Type_PBR_POLICY
		p.Rule = map[uint32]*oc.NetworkInstance_PolicyForwarding_Policy_Rule{1: &r1}
		policy := oc.NetworkInstance_PolicyForwarding{}
		policy.Policy = map[string]*oc.NetworkInstance_PolicyForwarding_Policy{pbrName: &p}

		t.Logf("TC: Configuring discard to true")
		t.Run("Update", func(t *testing.T) {
			gnmi.Replace(t, dut, gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Config(), &policy)
		})

		t.Run("Get", func(t *testing.T) {
			config := gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Policy(pbrName).Rule(uint32(1)).Action().Discard()
			configGot := gnmi.GetConfig(t, dut, config.Config())

			action := oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{}
			action.Discard = ygot.Bool(true)

			if configGot != *action.Discard {
				t.Errorf("TestFAIL : discard cfgd %v, expctd %v", configGot, *action.Discard)
			} else {
				t.Logf("Passed: Configured discard = %v, Obtained discard = %v", *action.Discard, configGot)
			}
		})

		t.Run("Get-State", func(t *testing.T) {
			state := gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().Policy(pbrName).Rule(1)
			stateGot := gnmi.Get(t, dut, state.State())
			actionDiscard := true
			if *(stateGot.Action.Discard) != actionDiscard {
				t.Errorf("Failed: Fetching state leaf for Action Discard, got %v, want %v",
					*(stateGot.Action.Discard), actionDiscard)
			} else {
				t.Logf("Passed: Configured Action Discard = %v", *(stateGot.Action.Discard))
			}
		})

		t.Run("Delete", func(t *testing.T) {
			gnmi.Delete(t, dut, gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Policy(pbrName).Config())
		})
	})
}

func Test_IntfRef(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	t.Log("Remove Flowspec Config")
	configToChange := "no flowspec \n"
	ctx := context.Background()
	util.GNMIWithText(ctx, t, dut, configToChange)
	t.Run("Testing openconfig-network-instance:network-instances/network-instance/policy-forwarding/policies/interfaces/interface/interface-ref/config/interface", func(t *testing.T) {
		r1 := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
		r1.SequenceId = ygot.Uint32(1)
		r1.Action =
			&oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String("ORANGE")}

		r1.L2 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_L2{
			Ethertype: oc.PacketMatchTypes_ETHERTYPE_ETHERTYPE_IPV4,
		}

		i := oc.NetworkInstance_PolicyForwarding_Interface{}
		i.InterfaceId = ygot.String(InterfaceName)
		i.ApplyVrfSelectionPolicy = ygot.String(pbrName)
		i.InterfaceRef = &oc.NetworkInstance_PolicyForwarding_Interface_InterfaceRef{Interface: ygot.String(InterfaceName)}

		p := oc.NetworkInstance_PolicyForwarding_Policy{}
		p.PolicyId = ygot.String(pbrName)
		p.Type = oc.Policy_Type_VRF_SELECTION_POLICY
		p.Rule = map[uint32]*oc.NetworkInstance_PolicyForwarding_Policy_Rule{1: &r1}

		policy := oc.NetworkInstance_PolicyForwarding{}
		policy.Policy = map[string]*oc.NetworkInstance_PolicyForwarding_Policy{pbrName: &p}
		policy.Interface = map[string]*oc.NetworkInstance_PolicyForwarding_Interface{InterfaceName: &i}

		t.Logf("TC: Configuring interface-ref-interface-id")
		t.Run("Update", func(t *testing.T) {
			gnmi.Replace(t, dut, gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Config(), &policy)
		})

		t.Run("Get", func(t *testing.T) {
			config := gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Interface(InterfaceName).ApplyVrfSelectionPolicy()
			configGot := gnmi.GetConfig(t, dut, config.Config())

			intf := oc.NetworkInstance_PolicyForwarding_Interface{}
			intf.ApplyVrfSelectionPolicy = ygot.String(pbrName)

			if configGot != *intf.ApplyVrfSelectionPolicy {
				t.Errorf("TestFAIL : Vrf is not applied")
			} else {
				t.Logf("Passed: Vrf is applied successfully")
			}
		})

		t.Run("Delete", func(t *testing.T) {
			gnmi.Delete(t, dut, gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Interface(InterfaceName).Config())
			gnmi.Delete(t, dut, gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Policy(pbrName).Config())
		})
	})
}

func Test_Decapgre(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	t.Log("Remove Flowspec Config")
	configToChange := "no flowspec \n"
	ctx := context.Background()
	util.GNMIWithText(ctx, t, dut, configToChange)
	t.Run("Testing openconfig-network-instance:network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/config/decapsulate-gre", func(t *testing.T) {
		r1 := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
		r1.SequenceId = ygot.Uint32(1)
		r1.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{DecapsulateGre: ygot.Bool(true)}
		r1.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
			Protocol: oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP,
		}

		p := oc.NetworkInstance_PolicyForwarding_Policy{}
		p.PolicyId = ygot.String(pbrName)
		p.Type = oc.Policy_Type_PBR_POLICY
		p.Rule = map[uint32]*oc.NetworkInstance_PolicyForwarding_Policy_Rule{1: &r1}
		policy := oc.NetworkInstance_PolicyForwarding{}
		policy.Policy = map[string]*oc.NetworkInstance_PolicyForwarding_Policy{pbrName: &p}

		t.Logf("TC: Configuring decapgre to true")
		t.Run("Update", func(t *testing.T) {
			gnmi.Replace(t, dut, gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Config(), &policy)
		})

		t.Run("Get", func(t *testing.T) {
			config := gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Policy(pbrName).Rule(uint32(1)).Action().DecapsulateGre()
			configGot := gnmi.GetConfig(t, dut, config.Config())

			action := oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{}
			action.DecapsulateGre = ygot.Bool(true)

			if configGot != *action.DecapsulateGre {
				t.Errorf("TestFAIL : decapgre cfgd %v, expctd %v", configGot, *action.DecapsulateGre)
			} else {
				t.Logf("Passed: Configured decapgre = %v, Obtained decapgre = %v", *action.DecapsulateGre, configGot)
			}
		})

		t.Run("Get-State", func(t *testing.T) {
			state := gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().Policy(pbrName).Rule(1)
			stateGot := gnmi.Get(t, dut, state.State())
			actionDecapsulateGre := true
			if *(stateGot.Action.DecapsulateGre) != actionDecapsulateGre {
				t.Errorf("Failed: Fetching state leaf for Action DecapsulateGre, got %v, want %v",
					*(stateGot.Action.DecapsulateGre), actionDecapsulateGre)
			} else {
				t.Logf("Passed: Configured Action DecapsulateGre = %v", *(stateGot.Action.DecapsulateGre))
			}
		})

		t.Run("Delete", func(t *testing.T) {
			gnmi.Delete(t, dut, gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Policy(pbrName).Config())
		})
	})
}

func Test_Decapgue(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	t.Log("Remove Flowspec Config")
	configToChange := "no flowspec \n"
	ctx := context.Background()
	util.GNMIWithText(ctx, t, dut, configToChange)
	t.Run("Testing openconfig-network-instance:network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/config/decapsulate-gue", func(t *testing.T) {
		r1 := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
		r1.SequenceId = ygot.Uint32(1)
		r1.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{DecapsulateGue: ygot.Bool(true)}
		r1.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
			Protocol: oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP,
		}

		p := oc.NetworkInstance_PolicyForwarding_Policy{}
		p.PolicyId = ygot.String(pbrName)
		p.Type = oc.Policy_Type_PBR_POLICY
		p.Rule = map[uint32]*oc.NetworkInstance_PolicyForwarding_Policy_Rule{1: &r1}
		policy := oc.NetworkInstance_PolicyForwarding{}
		policy.Policy = map[string]*oc.NetworkInstance_PolicyForwarding_Policy{pbrName: &p}

		t.Logf("TC: Configure decapgue to true")
		t.Run("Update", func(t *testing.T) {
			gnmi.Replace(t, dut, gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Config(), &policy)
		})

		t.Run("Get", func(t *testing.T) {
			config := gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Policy(pbrName).Rule(uint32(1)).Action().DecapsulateGue()
			configGot := gnmi.GetConfig(t, dut, config.Config())

			//action := oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{}
			actionDecapsulateGue := true

			if configGot != actionDecapsulateGue {
				t.Errorf("Failed : Fetching config leaf for decapgue, got %v, want %v",
					configGot, actionDecapsulateGue)
			} else {
				t.Logf("Passed: Configured decapgue = %v", actionDecapsulateGue)
			}
		})

		t.Run("Get-State", func(t *testing.T) {
			state := gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().Policy(pbrName).Rule(1)
			stateGot := gnmi.Get(t, dut, state.State())
			actionDecapsulateGue := true
			if *(stateGot.Action.DecapsulateGue) != actionDecapsulateGue {
				t.Errorf("Failed: Fetching state leaf for Action DecapsulateGue, got %v, want %v",
					*(stateGot.Action.DecapsulateGue), actionDecapsulateGue)
			} else {
				t.Logf("Passed: Configured Action DecapsulateGue = %v", *(stateGot.Action.DecapsulateGue))
			}
		})

		t.Run("Delete", func(t *testing.T) {
			gnmi.Delete(t, dut, gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Policy(pbrName).Config())
		})
	})
}

func Test_Nexthop_v4(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	t.Log("Remove Flowspec Config")
	configToChange := "no flowspec \n"
	ctx := context.Background()
	util.GNMIWithText(ctx, t, dut, configToChange)
	t.Run("Testing openconfig-network-instance:network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/config/next-hop", func(t *testing.T) {
		r1 := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
		r1.SequenceId = ygot.Uint32(1)
		r1.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NextHop: ygot.String("192.168.1.1")}
		r1.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
			Protocol: oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP,
		}

		p := oc.NetworkInstance_PolicyForwarding_Policy{}
		p.PolicyId = ygot.String(pbrName)
		p.Type = oc.Policy_Type_PBR_POLICY
		p.Rule = map[uint32]*oc.NetworkInstance_PolicyForwarding_Policy_Rule{1: &r1}
		policy := oc.NetworkInstance_PolicyForwarding{}
		policy.Policy = map[string]*oc.NetworkInstance_PolicyForwarding_Policy{pbrName: &p}

		t.Logf("TC: Configuring nexthop to 192.168.1.1")
		t.Run("Update", func(t *testing.T) {
			gnmi.Replace(t, dut, gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Config(), &policy)
		})

		t.Run("Get", func(t *testing.T) {
			config := gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().
				Policy(pbrName).Rule(uint32(1)).Action().NextHop()
			configGot := gnmi.GetConfig(t, dut, config.Config())

			action := oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{}
			action.NextHop = ygot.String("192.168.1.1")

			if configGot != *action.NextHop {
				t.Errorf("TestFAIL : nexthop cfgd %v, expctd %v", configGot, *action.NextHop)
			} else {
				t.Logf("Passed: Configured nexthop = %v, Obtained nexthop = %v", *action.NextHop, configGot)
			}
		})

		t.Run("Get-State", func(t *testing.T) {
			state := gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().Policy(pbrName).Rule(1)
			stateGot := gnmi.Get(t, dut, state.State())
			actionNextHop := "192.168.1.1"
			if *(stateGot.Action.NextHop) != actionNextHop {
				t.Errorf("Failed: Fetching state leaf for Action NextHop, got %v, want %v",
					*(stateGot.Action.NextHop), actionNextHop)
			} else {
				t.Logf("Passed: Configured Action IPv4 NextHop = %v", *(stateGot.Action.NextHop))
			}
		})

		t.Logf("TC: Modifying nexthop to 192.168.1.5")
		r1.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NextHop: ygot.String("192.168.1.5")}
		t.Run("Replace", func(t *testing.T) {
			gnmi.Replace(t, dut, gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Config(), &policy)
		})

		t.Run("Get-Config", func(t *testing.T) {
			config := gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().
				Policy(pbrName).Rule(uint32(1)).Action().NextHop()
			configGot := gnmi.GetConfig(t, dut, config.Config())

			action := oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{}
			action.NextHop = ygot.String("192.168.1.5")

			if configGot != *action.NextHop {
				t.Errorf("TestFAIL : nexthop cfgd %v, expctd %v", configGot, *action.NextHop)
			} else {
				t.Logf("Passed: Configured nexthop = %v, Obtained nexthop = %v", *action.NextHop, configGot)
			}
		})

		t.Run("Delete", func(t *testing.T) {
			gnmi.Delete(t, dut, gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Policy(pbrName).Config())
		})

	})
}

func Test_Nexthop_v6(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	t.Log("Remove Flowspec Config")
	configToChange := "no flowspec \n"
	ctx := context.Background()
	util.GNMIWithText(ctx, t, dut, configToChange)
	t.Run("Testing openconfig-network-instance:network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/config/next-hop", func(t *testing.T) {
		r1 := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
		r1.SequenceId = ygot.Uint32(1)
		r1.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NextHop: ygot.String("2003::21")}
		r1.Ipv6 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv6{
			Protocol: oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP,
		}

		p := oc.NetworkInstance_PolicyForwarding_Policy{}
		p.PolicyId = ygot.String(pbrName)
		p.Type = oc.Policy_Type_PBR_POLICY
		p.Rule = map[uint32]*oc.NetworkInstance_PolicyForwarding_Policy_Rule{1: &r1}
		policy := oc.NetworkInstance_PolicyForwarding{}
		policy.Policy = map[string]*oc.NetworkInstance_PolicyForwarding_Policy{pbrName: &p}

		t.Logf("TC: Configuring nexthop to 2003::21")
		t.Run("Update", func(t *testing.T) {
			gnmi.Replace(t, dut, gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Config(), &policy)
		})

		t.Run("Get", func(t *testing.T) {
			config := gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Policy(pbrName).Rule(uint32(1)).Action().NextHop()
			configGot := gnmi.GetConfig(t, dut, config.Config())

			action := oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{}
			action.NextHop = ygot.String("2003::21")

			if configGot != *action.NextHop {
				t.Errorf("TestFAIL : nexthop cfgd %v, expctd %v", configGot, *action.NextHop)
			} else {
				t.Logf("Passed: Configured nexthop = %v, Obtained nexthop = %v", *action.NextHop, configGot)
			}
		})

		t.Run("Get-State", func(t *testing.T) {
			state := gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().Policy(pbrName).Rule(1)
			stateGot := gnmi.Get(t, dut, state.State())
			actionNextHop := "2003::21"
			if *(stateGot.Action.NextHop) != actionNextHop {
				t.Errorf("Failed: Fetching state leaf for Action NextHop, got %v, want %v",
					*(stateGot.Action.NextHop), actionNextHop)
			} else {
				t.Logf("Passed: Configured Action IPv6 NextHop = %v", *(stateGot.Action.NextHop))
			}
		})

		r1.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NextHop: ygot.String("2003::31")}
		t.Logf("TC: Changing nexthop to 2003::31")
		t.Run("Replace", func(t *testing.T) {
			gnmi.Replace(t, dut, gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Config(), &policy)
		})

		t.Run("Get", func(t *testing.T) {
			config := gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Policy(pbrName).Rule(uint32(1)).Action().NextHop()
			configGot := gnmi.GetConfig(t, dut, config.Config())

			action := oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{}
			action.NextHop = ygot.String("2003::31")

			if configGot != *action.NextHop {
				t.Errorf("TestFAIL : nexthop cfgd %v, expctd %v", configGot, *action.NextHop)
			} else {
				t.Logf("Passed: Configured nexthop = %v, Obtained nexthop = %v", *action.NextHop, configGot)
			}
		})

		t.Run("Delete", func(t *testing.T) {
			gnmi.Delete(t, dut, gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Policy(pbrName).Config())
		})

	})
}

func Test_Rule_L2_Source_Mac(t *testing.T) {
	t.Skip() //L2 source mac is not supported currently by the platform
	dut := ondatra.DUT(t, "dut")

	t.Log("Remove Flowspec Config")
	configToChange := "no flowspec \n"
	ctx := context.Background()
	util.GNMIWithText(ctx, t, dut, configToChange)

	t.Run("Testing openconfig-network-instance:network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/l2/config/source-mac", func(t *testing.T) {
		r1 := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
		r1.SequenceId = ygot.Uint32(1)
		r1.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String("TE")}

		r1.L2 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_L2{
			Ethertype: oc.PacketMatchTypes_ETHERTYPE_ETHERTYPE_IPV4,
			SourceMac: ygot.String("00:aa:00:bb:00:cc"),
		}

		p := oc.NetworkInstance_PolicyForwarding_Policy{}
		p.PolicyId = ygot.String(pbrName)
		p.Type = oc.Policy_Type_VRF_SELECTION_POLICY
		p.Rule = map[uint32]*oc.NetworkInstance_PolicyForwarding_Policy_Rule{1: &r1}
		policy := oc.NetworkInstance_PolicyForwarding{}
		policy.Policy = map[string]*oc.NetworkInstance_PolicyForwarding_Policy{pbrName: &p}

		t.Log("Update to value: 00:aa:00:bb:00:cc")
		t.Run("Update", func(t *testing.T) {
			gnmi.Replace(t, dut, gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Config(), &policy)
		})

		t.Log("Verify after update")
		t.Run("Get", func(t *testing.T) {
			config := gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Policy(pbrName).Rule(uint32(1)).L2().SourceMac()
			configGot := gnmi.GetConfig(t, dut, config.Config())

			ruleL2 := oc.NetworkInstance_PolicyForwarding_Policy_Rule_L2{}
			ruleL2.SourceMac = ygot.String("00:aa:00:bb:00:cc")

			if configGot != *ruleL2.SourceMac {
				t.Errorf("Failed: Fetching leaf for source-mac got %v, want %v", configGot, *ruleL2.SourceMac)
			} else {
				t.Logf("Passed: Configured source-mac = Obtained source-mac = %v", configGot)
			}
		})

		r1.L2 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_L2{
			Ethertype: oc.PacketMatchTypes_ETHERTYPE_ETHERTYPE_IPV4,
			SourceMac: ygot.String("11:aa:11:bb:11:cc"),
		}
		t.Log("Replace src-mac with value: 11:aa:11:bb:11:cc")
		t.Run("Replace", func(t *testing.T) {
			gnmi.Replace(t, dut, gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Config(), &policy)
		})

		t.Log("Verify after replace")
		t.Run("Get", func(t *testing.T) {
			config := gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Policy(pbrName).Rule(uint32(1)).L2().SourceMac()
			configGot := gnmi.GetConfig(t, dut, config.Config())

			ruleL2 := oc.NetworkInstance_PolicyForwarding_Policy_Rule_L2{}
			ruleL2.SourceMac = ygot.String("11:aa:11:bb:11:cc")

			if configGot != *ruleL2.SourceMac {
				t.Errorf("Failed: Fetching leaf for source-mac got %v, want %v", configGot, *ruleL2.SourceMac)
			} else {
				t.Logf("Passed: Configured source-mac = Obtained source-mac = %v", configGot)
			}
		})

		t.Run("Delete", func(t *testing.T) {
			gnmi.Delete(t, dut, gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Policy(pbrName).Config())
		})
	})
}

func Test_Rule_L2_Destination_Mac(t *testing.T) {
	t.Skip() //L2 destination mac is not supported currently by the platform
	dut := ondatra.DUT(t, "dut")

	t.Log("Remove Flowspec Config")
	configToChange := "no flowspec \n"
	ctx := context.Background()
	util.GNMIWithText(ctx, t, dut, configToChange)

	t.Run("Testing openconfig-network-instance:network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/l2/config/destination-mac", func(t *testing.T) {
		r1 := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
		r1.SequenceId = ygot.Uint32(1)
		r1.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String("TE")}

		r1.L2 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_L2{
			Ethertype:      oc.PacketMatchTypes_ETHERTYPE_ETHERTYPE_IPV4,
			DestinationMac: ygot.String("00:dd:00:ee:00:ff"),
		}

		p := oc.NetworkInstance_PolicyForwarding_Policy{}
		p.PolicyId = ygot.String(pbrName)
		p.Type = oc.Policy_Type_VRF_SELECTION_POLICY
		p.Rule = map[uint32]*oc.NetworkInstance_PolicyForwarding_Policy_Rule{1: &r1}
		policy := oc.NetworkInstance_PolicyForwarding{}
		policy.Policy = map[string]*oc.NetworkInstance_PolicyForwarding_Policy{pbrName: &p}

		t.Log("Update to value: 00:dd:00:ee:00:ff")
		t.Run("Update", func(t *testing.T) {
			gnmi.Replace(t, dut, gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Config(), &policy)
		})

		t.Log("Verify after update")
		t.Run("Get", func(t *testing.T) {
			config := gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Policy(pbrName).Rule(uint32(1)).L2().DestinationMac()
			configGot := gnmi.GetConfig(t, dut, config.Config())

			ruleL2 := oc.NetworkInstance_PolicyForwarding_Policy_Rule_L2{}
			ruleL2.DestinationMac = ygot.String("00:dd:00:ee:00:ff")

			if configGot != *ruleL2.DestinationMac {
				t.Errorf("Failed: Fetching leaf for destination-mac got %v, want %v", configGot, *ruleL2.DestinationMac)
			} else {
				t.Logf("Passed: Configured destination-mac = Obtained destination-mac = %v", configGot)
			}
		})

		t.Log("Replace dst-mac with value: 11:dd:11:ee:11:ff")
		r1.L2 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_L2{
			Ethertype:      oc.PacketMatchTypes_ETHERTYPE_ETHERTYPE_IPV4,
			DestinationMac: ygot.String("11:dd:11:ee:11:ff"),
		}

		t.Run("Replace", func(t *testing.T) {
			gnmi.Replace(t, dut, gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Config(), &policy)
		})

		t.Log("Verify after replace")
		t.Run("Get", func(t *testing.T) {
			config := gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Policy(pbrName).Rule(uint32(1)).L2().DestinationMac()
			configGot := gnmi.GetConfig(t, dut, config.Config())

			ruleL2 := oc.NetworkInstance_PolicyForwarding_Policy_Rule_L2{}
			ruleL2.DestinationMac = ygot.String("11:dd:11:ee:11:ff")

			if configGot != *ruleL2.DestinationMac {
				t.Errorf("Failed: Fetching leaf for destination-mac got %v, want %v", configGot, *ruleL2.DestinationMac)
			} else {
				t.Logf("Passed: Configured destination-mac = Obtained destination-mac = %v", configGot)
			}
		})

		t.Run("Delete", func(t *testing.T) {
			gnmi.Delete(t, dut, gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Policy(pbrName).Config())
		})
	})
}

func Test_Action_Decap_Network_Instance(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	t.Log("Remove Flowspec Config")
	configToChange := "no flowspec \n"
	ctx := context.Background()
	util.GNMIWithText(ctx, t, dut, configToChange)
	t.Run("*", func(t *testing.T) {
		r1 := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
		r1.SequenceId = ygot.Uint32(1)
		r1.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{
			DecapNetworkInstance:         ygot.String(vrfBLUE),
			DecapFallbackNetworkInstance: ygot.String(vrfRED),
			PostDecapNetworkInstance:     ygot.String(vrfGREEN),
		}

		r1.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
			Protocol:      oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP,
			DscpSet:       []uint8{*ygot.Uint8(4)},
			SourceAddress: ygot.String("222.222.222.222/32"),
		}

		p := oc.NetworkInstance_PolicyForwarding_Policy{}
		p.PolicyId = ygot.String(pbrName)
		p.Type = oc.Policy_Type_VRF_SELECTION_POLICY
		p.Rule = map[uint32]*oc.NetworkInstance_PolicyForwarding_Policy_Rule{1: &r1}
		policy := oc.NetworkInstance_PolicyForwarding{}
		policy.Policy = map[string]*oc.NetworkInstance_PolicyForwarding_Policy{pbrName: &p}

		t.Logf("replace@policy-forwarding cont - Create a Policy with decap actions")
		t.Run("Replace", func(t *testing.T) {
			gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().Config(), &policy)
		})

		t.Run("Get-Config", func(t *testing.T) {
			config := gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().
				Policy(pbrName).Rule(uint32(SeqID1)).Action().DecapNetworkInstance()
			configGot := gnmi.GetConfig(t, dut, config.Config())

			action := oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{}
			action.DecapNetworkInstance = ygot.String(vrfBLUE)

			if configGot != *action.DecapNetworkInstance {
				t.Errorf("TestFAIL : decap-network-instance expected: '%v', received: '%v'", *action.DecapNetworkInstance, configGot)
			} else {
				t.Logf("TestPASS")
			}
		})

		t.Run("Get-State", func(t *testing.T) {
			state := gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().Policy(pbrName).Rule(SeqID1)
			stateGot := gnmi.Get(t, dut, state.State())
			if *(stateGot.Action.DecapNetworkInstance) != vrfBLUE {
				t.Errorf("TestFAIL : Fetching state leaf for Action DecapNetworkInstance. Expected: '%v', Received: '%v'",
					*(stateGot.Action.DecapNetworkInstance), vrfBLUE)
			} else {
				t.Logf("TestPASS")
			}
		})

		t.Logf("update@decap-network-instance leaf - change value from BLUE to PINK")
		t.Run("Update", func(t *testing.T) {
			gnmi.Update(t, dut, gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().Policy(pbrName).Rule(SeqID1).Action().DecapNetworkInstance().Config(), *ygot.String(vrfPINK))
		})

		t.Run("Get-Config", func(t *testing.T) {
			config := gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().
				Policy(pbrName).Rule(uint32(SeqID1)).Action().DecapNetworkInstance()
			configGot := gnmi.GetConfig(t, dut, config.Config())

			action := oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{}
			action.DecapNetworkInstance = ygot.String(vrfPINK)

			if configGot != *action.DecapNetworkInstance {
				t.Errorf("TestFAIL : decap-network-instance expected: '%v', received: '%v'", *action.DecapNetworkInstance, configGot)
			} else {
				t.Logf("TestPASS")
			}
		})

		t.Run("Get-State", func(t *testing.T) {
			state := gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().Policy(pbrName).Rule(1)
			stateGot := gnmi.Get(t, dut, state.State())
			if *(stateGot.Action.DecapNetworkInstance) != vrfPINK {
				t.Errorf("TestFAIL : Fetching state leaf for Action DecapNetworkInstance. Expected: '%v', Received: '%v'",
					*(stateGot.Action.DecapNetworkInstance), vrfPINK)
			} else {
				t.Logf("TestPASS")
			}
		})

		t.Logf("replace@decap-network-instance leaf - change value from PINK to ORANGE")
		t.Run("Replace", func(t *testing.T) {
			gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().Policy(pbrName).Rule(SeqID1).Action().DecapNetworkInstance().Config(), *ygot.String(vrfORANGE))
		})

		t.Run("Get-Config", func(t *testing.T) {
			config := gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().
				Policy(pbrName).Rule(uint32(SeqID1)).Action().DecapNetworkInstance()
			configGot := gnmi.GetConfig(t, dut, config.Config())

			action := oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{}
			action.DecapNetworkInstance = ygot.String(vrfORANGE)

			if configGot != *action.DecapNetworkInstance {
				t.Errorf("TestFAIL : decap-network-instance expected: '%v', received: '%v'", vrfORANGE, configGot)
			} else {
				t.Logf("TestPASS")
			}
		})

		t.Run("Get-State", func(t *testing.T) {
			state := gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().Policy(pbrName).Rule(SeqID1)
			stateGot := gnmi.Get(t, dut, state.State())
			if *(stateGot.Action.DecapNetworkInstance) != vrfORANGE {
				t.Errorf("TestFAIL : Fetching state leaf for Action DecapNetworkInstance. Expected: '%v', Received: '%v'",
					*(stateGot.Action.DecapNetworkInstance), vrfORANGE)
			} else {
				t.Logf("TestPASS")
			}
		})

		Decap_var := &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{
			DecapNetworkInstance: ygot.String(vrfBLACK),
		}

		t.Logf("replace@action container - expected to fail")
		got := testt.ExpectFatal(t, func(t testing.TB) {
			gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().Policy(pbrName).Rule(SeqID1).Action().Config(), Decap_var)
		})

		if got == "" {
			t.Errorf("Replace did not fail as expected")
		} else {
			t.Logf("TestPASS")
		}

		t.Run("Delete", func(t *testing.T) {
			gnmi.Delete(t, dut, gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().Policy(pbrName).Config())
		})

	})
}

func Test_Action_Decap_Fallback_Network_Instance(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	t.Log("Remove Flowspec Config")
	configToChange := "no flowspec \n"
	ctx := context.Background()
	util.GNMIWithText(ctx, t, dut, configToChange)
	t.Run("*", func(t *testing.T) {
		r1 := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
		r1.SequenceId = ygot.Uint32(SeqID1)
		r1.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{
			DecapNetworkInstance:         ygot.String(vrfBLUE),
			DecapFallbackNetworkInstance: ygot.String(vrfRED),
			PostDecapNetworkInstance:     ygot.String(vrfGREEN),
		}

		r1.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
			Protocol:      oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP,
			DscpSet:       []uint8{*ygot.Uint8(4)},
			SourceAddress: ygot.String("222.222.222.222/32"),
		}

		p := oc.NetworkInstance_PolicyForwarding_Policy{}
		p.PolicyId = ygot.String(pbrName)
		p.Type = oc.Policy_Type_VRF_SELECTION_POLICY
		p.Rule = map[uint32]*oc.NetworkInstance_PolicyForwarding_Policy_Rule{1: &r1}
		policy := oc.NetworkInstance_PolicyForwarding{}
		policy.Policy = map[string]*oc.NetworkInstance_PolicyForwarding_Policy{pbrName: &p}

		t.Logf("replace@policy-forwarding cont - Create a Policy with decap actions")
		t.Run("Replace", func(t *testing.T) {
			gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().Config(), &policy)
		})

		t.Run("Get-Config", func(t *testing.T) {
			config := gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().
				Policy(pbrName).Rule(uint32(SeqID1)).Action().DecapFallbackNetworkInstance()
			configGot := gnmi.GetConfig(t, dut, config.Config())

			action := oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{}
			action.DecapFallbackNetworkInstance = ygot.String(vrfRED)

			if configGot != *action.DecapFallbackNetworkInstance {
				t.Errorf("TestFAIL : decap-fallback-network-instance expected: '%v', received: '%v'", *action.DecapFallbackNetworkInstance, configGot)
			} else {
				t.Logf("TestPASS")
			}
		})

		t.Run("Get-State", func(t *testing.T) {
			state := gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().Policy(pbrName).Rule(SeqID1)
			stateGot := gnmi.Get(t, dut, state.State())
			if *(stateGot.Action.DecapFallbackNetworkInstance) != vrfRED {
				t.Errorf("TestFAIL : Fetching state leaf for Action DecapFallbackNetworkInstance. Expected: '%v', Received: '%v'",
					*(stateGot.Action.DecapFallbackNetworkInstance), vrfRED)
			} else {
				t.Logf("TestPASS")
			}
		})

		t.Logf("update@decap-fallback-network-instance leaf - change value from RED to BROWN")
		t.Run("Update", func(t *testing.T) {
			gnmi.Update(t, dut, gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().Policy(pbrName).Rule(1).Action().DecapFallbackNetworkInstance().Config(), *ygot.String(vrfBROWN))
		})

		t.Run("Get-Config", func(t *testing.T) {
			config := gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().
				Policy(pbrName).Rule(uint32(SeqID1)).Action().DecapFallbackNetworkInstance()
			configGot := gnmi.GetConfig(t, dut, config.Config())

			action := oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{}
			action.DecapFallbackNetworkInstance = ygot.String(vrfBROWN)

			if configGot != *action.DecapFallbackNetworkInstance {
				t.Errorf("TestFAIL : decap-fallback-network-instance expected: '%v', received: '%v'", *action.DecapFallbackNetworkInstance, configGot)
			} else {
				t.Logf("TestPASS")
			}
		})

		t.Run("Get-State", func(t *testing.T) {
			state := gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().Policy(pbrName).Rule(SeqID1)
			stateGot := gnmi.Get(t, dut, state.State())
			if *(stateGot.Action.DecapFallbackNetworkInstance) != vrfBROWN {
				t.Errorf("TestFAIL : Fetching state leaf for Action DecapFallbackNetworkInstance. Expected: '%v', Received: '%v'",
					*(stateGot.Action.DecapFallbackNetworkInstance), vrfBROWN)
			} else {
				t.Logf("TestPASS")
			}
		})

		t.Logf("replace@decap-fallback-network-instance leaf - change value from BROWN to INDIGO")
		t.Run("Replace", func(t *testing.T) {
			gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().Policy(pbrName).Rule(1).Action().DecapFallbackNetworkInstance().Config(), *ygot.String(vrfINDIGO))
		})

		t.Run("Get-Config", func(t *testing.T) {
			config := gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().
				Policy(pbrName).Rule(uint32(SeqID1)).Action().DecapFallbackNetworkInstance()
			configGot := gnmi.GetConfig(t, dut, config.Config())

			action := oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{}
			action.DecapFallbackNetworkInstance = ygot.String(vrfINDIGO)

			if configGot != *action.DecapFallbackNetworkInstance {
				t.Errorf("TestFAIL : decap-fallback-network-instance expected: '%v', received: '%v'", *action.DecapFallbackNetworkInstance, configGot)
			} else {
				t.Logf("TestPASS")
			}
		})

		t.Run("Get-State", func(t *testing.T) {
			state := gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().Policy(pbrName).Rule(SeqID1)
			stateGot := gnmi.Get(t, dut, state.State())
			if *(stateGot.Action.DecapFallbackNetworkInstance) != vrfINDIGO {
				t.Errorf("TestFAIL : Fetching state leaf for Action DecapFallbackNetworkInstance. Expected: '%v', Received: '%v'",
					*(stateGot.Action.DecapFallbackNetworkInstance), vrfINDIGO)
			} else {
				t.Logf("TestPASS")
			}
		})

		DecapFallback_var := &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{
			DecapFallbackNetworkInstance: ygot.String(vrfBLACK),
		}

		t.Logf("replace@action container - expected to fail")
		got := testt.ExpectFatal(t, func(t testing.TB) {
			gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().Policy(pbrName).Rule(SeqID1).Action().Config(), DecapFallback_var)
		})

		if got == "" {
			t.Errorf("Replace did not fail as expected")
		} else {
			t.Logf("TestPASS")
		}

		t.Run("Delete", func(t *testing.T) {
			gnmi.Delete(t, dut, gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().Policy(pbrName).Config())
		})

	})
}

func Test_Action_Post_Decap_Network_Instance(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	t.Log("Remove Flowspec Config")
	configToChange := "no flowspec \n"
	ctx := context.Background()
	util.GNMIWithText(ctx, t, dut, configToChange)
	t.Run("*", func(t *testing.T) {
		r1 := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
		r1.SequenceId = ygot.Uint32(SeqID1)
		r1.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{
			DecapNetworkInstance:         ygot.String(vrfBLUE),
			DecapFallbackNetworkInstance: ygot.String(vrfRED),
			PostDecapNetworkInstance:     ygot.String(vrfGREEN),
		}

		r1.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
			Protocol:      oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP,
			DscpSet:       []uint8{*ygot.Uint8(4)},
			SourceAddress: ygot.String("222.222.222.222/32"),
		}

		p := oc.NetworkInstance_PolicyForwarding_Policy{}
		p.PolicyId = ygot.String(pbrName)
		p.Type = oc.Policy_Type_VRF_SELECTION_POLICY
		p.Rule = map[uint32]*oc.NetworkInstance_PolicyForwarding_Policy_Rule{1: &r1}
		policy := oc.NetworkInstance_PolicyForwarding{}
		policy.Policy = map[string]*oc.NetworkInstance_PolicyForwarding_Policy{pbrName: &p}

		t.Logf("replace@policy-forwarding cont - Create a Policy with decap actions")
		t.Run("Replace", func(t *testing.T) {
			gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().Config(), &policy)
		})

		t.Run("Apply to interface", func(t *testing.T) {
			gnmi.Update(t, dut, gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().Interface(InterfaceName).ApplyVrfSelectionPolicy().Config(), pbrName)
		})

		t.Run("Get-Config", func(t *testing.T) {
			config := gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().
				Policy(pbrName).Rule(uint32(SeqID1)).Action().PostDecapNetworkInstance()
			configGot := gnmi.GetConfig(t, dut, config.Config())

			action := oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{}
			action.PostDecapNetworkInstance = ygot.String(vrfGREEN)

			if configGot != *action.PostDecapNetworkInstance {
				t.Errorf("TestFAIL : post-decap-network-instance expected: '%v', received: '%v'", *action.PostDecapNetworkInstance, configGot)
			} else {
				t.Logf("TestPASS")
			}
		})

		t.Run("Get-State", func(t *testing.T) {
			state := gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().Policy(pbrName).Rule(1)
			stateGot := gnmi.Get(t, dut, state.State())
			if *(stateGot.Action.PostDecapNetworkInstance) != vrfGREEN {
				t.Errorf("TestFAIL : Fetching state leaf for Action PostDecapNetworkInstance. Expected: '%v', Received: '%v'",
					*(stateGot.Action.PostDecapNetworkInstance), vrfGREEN)
			} else {
				t.Logf("TestPASS")
			}
		})

		t.Logf("update@post-decap-network-instance leaf - change value from GREEN to PURPLE")
		t.Run("Update", func(t *testing.T) {
			gnmi.Update(t, dut, gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().Policy(pbrName).Rule(SeqID1).Action().PostDecapNetworkInstance().Config(), *ygot.String(vrfPURPLE))
		})

		t.Run("Get-Config", func(t *testing.T) {
			config := gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().
				Policy(pbrName).Rule(uint32(SeqID1)).Action().PostDecapNetworkInstance()
			configGot := gnmi.GetConfig(t, dut, config.Config())

			action := oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{}
			action.PostDecapNetworkInstance = ygot.String(vrfPURPLE)

			if configGot != *action.PostDecapNetworkInstance {
				t.Errorf("TestFAIL : post-decap-network-instance expected: '%v', received: '%v'", *action.PostDecapNetworkInstance, configGot)
			} else {
				t.Logf("TestPASS")
			}
		})

		t.Run("Get-State", func(t *testing.T) {
			state := gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().Policy(pbrName).Rule(SeqID1)
			stateGot := gnmi.Get(t, dut, state.State())
			if *(stateGot.Action.PostDecapNetworkInstance) != vrfPURPLE {
				t.Errorf("TestFAIL : Fetching state leaf for Action PostDecapNetworkInstance. Expected: '%v', Received: '%v'",
					*(stateGot.Action.PostDecapNetworkInstance), vrfPURPLE)
			} else {
				t.Logf("TestPASS")
			}
		})

		t.Logf("replace@post-decap-network-instance leaf - change value from PURPLE to BROWN")
		t.Run("Replace", func(t *testing.T) {
			gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().Policy(pbrName).Rule(SeqID1).Action().PostDecapNetworkInstance().Config(), *ygot.String(vrfBROWN))
		})

		t.Run("Get-Config", func(t *testing.T) {
			config := gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().
				Policy(pbrName).Rule(uint32(SeqID1)).Action().PostDecapNetworkInstance()
			configGot := gnmi.GetConfig(t, dut, config.Config())

			action := oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{}
			action.PostDecapNetworkInstance = ygot.String(vrfBROWN)

			if configGot != *action.PostDecapNetworkInstance {
				t.Errorf("TestFAIL : post-decap-network-instance expected: '%v', received: '%v'", *action.PostDecapNetworkInstance, configGot)
			} else {
				t.Logf("TestPASS")
			}
		})

		t.Run("Get-State", func(t *testing.T) {
			state := gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().Policy(pbrName).Rule(SeqID1)
			stateGot := gnmi.Get(t, dut, state.State())
			if *(stateGot.Action.PostDecapNetworkInstance) != vrfBROWN {
				t.Errorf("TestFAIL : Fetching state leaf for Action PostDecapNetworkInstance. Expected: '%v', Received: '%v'",
					*(stateGot.Action.PostDecapNetworkInstance), vrfBROWN)
			} else {
				t.Logf("TestPASS")
			}
		})

		postDecap_var := &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{
			PostDecapNetworkInstance: ygot.String(vrfBLACK),
		}

		t.Logf("replace@action container - expected to fail")
		got := testt.ExpectFatal(t, func(t testing.TB) {
			gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().Policy(pbrName).Rule(SeqID1).Action().Config(), postDecap_var)
		})

		if got == "" {
			t.Errorf("Replace did not fail as expected")
		} else {
			t.Logf("TestPASS")
		}

		t.Run("Delete", func(t *testing.T) {
			gnmi.Delete(t, dut, gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().Policy(pbrName).Config())
		})

	})
}

func Test_Decap_feature(t *testing.T) {
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

	t.Run("Replace", func(t *testing.T) {
		gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().Config(), &policy)
	})

	t.Run("Update", func(t *testing.T) {
		gnmi.Update(t, dut, gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().Interface(InterfaceName).ApplyVrfSelectionPolicy().Config(), pbrName)
	})

	t.Logf("Check State information for Rule1")
	t.Run("Get-state", func(t *testing.T) {
		state := gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().Policy(pbrName).Rule(SeqID1)
		stateGot := gnmi.Get(t, dut, state.State())
		if *(stateGot.Action.DecapNetworkInstance) != vrfBLUE {
			t.Errorf("TestFAIL : Fetching state leaf for Action DecapNetworkInstance. Expected: '%v', Received: '%v'",
				*(stateGot.Action.DecapNetworkInstance), vrfBLUE)
		} else {
			t.Logf("TestPASS : DecapNetworkInstance found as expected")
		}
		if *(stateGot.Action.DecapFallbackNetworkInstance) != vrfRED {
			t.Errorf("TestFAIL : Fetching state leaf for Action DecapFallbackNetworkInstance. Expected: '%v', Received: '%v'",
				*(stateGot.Action.DecapFallbackNetworkInstance), vrfRED)
		} else {
			t.Logf("TestPASS : DecapFallbackNetworkInstance found as expected")
		}
		if *(stateGot.Action.PostDecapNetworkInstance) != vrfGREEN {
			t.Errorf("TestFAIL : Fetching state leaf for Action PostDecapNetworkInstance. Expected: '%v', Received: '%v'",
				*(stateGot.Action.PostDecapNetworkInstance), vrfGREEN)
		} else {
			t.Logf("TestPASS : PostDecapNetworkInstance found as expected")
		}
	})

	t.Logf("Check State information for Rule2")
	t.Run("Get-state", func(t *testing.T) {
		state := gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().Policy(pbrName).Rule(SeqID2)
		stateGot := gnmi.Get(t, dut, state.State())
		if *(stateGot.Action.DecapNetworkInstance) != vrfORANGE {
			t.Errorf("TestFAIL : Fetching state leaf for Action DecapNetworkInstance. Expected: '%v', Received: '%v'",
				*(stateGot.Action.DecapNetworkInstance), vrfORANGE)
		} else {
			t.Logf("TestPASS : DecapNetworkInstance found as expected")
		}
		if *(stateGot.Action.DecapFallbackNetworkInstance) != vrfPINK {
			t.Errorf("TestFAIL : Fetching state leaf for Action DecapFallbackNetworkInstance. Expected: '%v', Received: '%v'",
				*(stateGot.Action.DecapFallbackNetworkInstance), vrfPINK)
		} else {
			t.Logf("TestPASS : DecapFallbackNetworkInstance found as expected")
		}
		if *(stateGot.Action.PostDecapNetworkInstance) != vrfBROWN {
			t.Errorf("TestFAIL : Fetching state leaf for Action PostDecapNetworkInstance. Expected: '%v', Received: '%v'",
				*(stateGot.Action.PostDecapNetworkInstance), vrfBROWN)
		} else {
			t.Logf("TestPASS : PostDecapNetworkInstance found as expected")
		}
	})

	t.Logf("replace@policy list should remove the existing rules, and add the new rule")
	t.Run("Replace", func(t *testing.T) {
		r3 := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
		r3.SequenceId = ygot.Uint32(SeqID3)

		r3.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
			Protocol:      oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP,
			DscpSet:       []uint8{*ygot.Uint8(4), *ygot.Uint8(8)},
			SourceAddress: ygot.String("222.222.222.222/32"),
		}

		r3.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{
			DecapNetworkInstance:         ygot.String(vrfBLUE),
			DecapFallbackNetworkInstance: ygot.String(vrfRED),
			PostDecapNetworkInstance:     ygot.String(vrfGREEN),
		}

		p := oc.NetworkInstance_PolicyForwarding_Policy{}
		p.PolicyId = ygot.String(pbrName)
		p.Type = oc.Policy_Type_VRF_SELECTION_POLICY
		p.Rule = map[uint32]*oc.NetworkInstance_PolicyForwarding_Policy_Rule{3: &r3}

		gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().Policy(pbrName).Config(), &p)
	})

	//TODO: Validate r1 and r2 are deleted

	//t.Run("Get-Config#3", func(t *testing.T) {
	//    config := gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Policy(pbrName).Rule(uint32(SeqID1)).Ipv4()
	//    configGot := gnmi.GetConfig(t, dut, config.Config())

	//    if len(configGot) > 0 {
	//        t.Errorf("TestFAIL : Stale config exists!!!")
	//    } else {
	//        t.Logf("TestPASS")
	//    }
	//})

	t.Run("Cleanup", func(t *testing.T) {
		gnmi.Delete(t, dut, gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().Config())
	})
}
