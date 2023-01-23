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
