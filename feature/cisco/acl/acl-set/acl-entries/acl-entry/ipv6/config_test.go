package acl_test

import (
	"fmt"
	"testing"

	"github.com/openconfig/featureprofiles/feature/cisco/acl/setup"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func setupAcl(t *testing.T, dut *ondatra.DUTDevice) *oc.Acl {
	bc := new(oc.Acl)
	*bc = setup.BaseConfig()
	setup.ResetStruct(bc, []string{"AclSet"})
	bcAclSet := setup.GetAnyValue(bc.AclSet)
	setup.ResetStruct(bcAclSet, []string{"AclEntry"})
	bcAclSetAclEntry := setup.GetAnyValue(bcAclSet.AclEntry)
	bcAclSet.Type = oc.E_Acl_ACL_TYPE(2) // IPV6
	setup.ResetStruct(bcAclSetAclEntry, []string{"Ipv6", "Actions"})
	bcAclSetAclEntryIpv6 := bcAclSetAclEntry.Ipv6
	setup.ResetStruct(bcAclSetAclEntryIpv6, []string{})
	gnmi.Replace(t, dut, gnmi.OC().Acl().Config(), bc)
	return bc
}

func teardownAcl(t *testing.T, dut *ondatra.DUTDevice, baseConfig *oc.Acl) {
	gnmi.Delete(t, dut, gnmi.OC().Acl().Config())
}
func TestProtocol(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	baseConfig := setupAcl(t, dut)
	defer teardownAcl(t, dut, baseConfig)

	inputs := []oc.Acl_AclSet_AclEntry_Ipv6_Protocol_Union{
		oc.UnionUint8(6),
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/protocol using value %v", input), func(t *testing.T) {
			baseConfigAclSet := setup.GetAnyValue(baseConfig.AclSet)
			baseConfigAclSetAclEntry := setup.GetAnyValue(baseConfigAclSet.AclEntry)
			baseConfigAclSetAclEntryIpv6 := baseConfigAclSetAclEntry.Ipv6
			baseConfigAclSetAclEntryIpv6.Protocol = input

			config := gnmi.OC().Acl().AclSet(*baseConfigAclSet.Name, baseConfigAclSet.Type).AclEntry(*baseConfigAclSetAclEntry.SequenceId).Ipv6()
			state := gnmi.OC().Acl().AclSet(*baseConfigAclSet.Name, baseConfigAclSet.Type).AclEntry(*baseConfigAclSetAclEntry.SequenceId).Ipv6()

			t.Run("Replace", func(t *testing.T) {
				gnmi.Replace(t, dut, config.Config(), baseConfigAclSetAclEntryIpv6)
			})
			if !setup.SkipGet() {
				t.Run("Get", func(t *testing.T) {
					configGot := gnmi.Get(t, dut, config.Config())
					if configGot.Protocol != input {
						t.Errorf("Config /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/protocol: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe", func(t *testing.T) {
					stateGot := gnmi.Get(t, dut, state.State())
					if stateGot.Protocol != input {
						t.Errorf("State /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/protocol: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				if !setup.SkipSubscribe() {
					if qs, _ := gnmi.LookupConfig(t, dut, config.Config()).Val(); qs.Protocol != nil {
						t.Errorf("Delete /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/protocol fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestSourceFlowLabel(t *testing.T) {
	t.Skip()
	dut := ondatra.DUT(t, "dut")

	baseConfig := setupAcl(t, dut)
	defer teardownAcl(t, dut, baseConfig)

	inputs := []uint32{
		1040812,
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/source-flow-label using value %v", input), func(t *testing.T) {
			baseConfigAclSet := setup.GetAnyValue(baseConfig.AclSet)
			baseConfigAclSetAclEntry := setup.GetAnyValue(baseConfigAclSet.AclEntry)
			baseConfigAclSetAclEntryIpv6 := baseConfigAclSetAclEntry.Ipv6
			*baseConfigAclSetAclEntryIpv6.SourceFlowLabel = input

			config := gnmi.OC().Acl().AclSet(*baseConfigAclSet.Name, baseConfigAclSet.Type).AclEntry(*baseConfigAclSetAclEntry.SequenceId).Ipv6()
			state := gnmi.OC().Acl().AclSet(*baseConfigAclSet.Name, baseConfigAclSet.Type).AclEntry(*baseConfigAclSetAclEntry.SequenceId).Ipv6()

			t.Run("Replace", func(t *testing.T) {
				gnmi.Replace(t, dut, config.Config(), baseConfigAclSetAclEntryIpv6)
			})
			if !setup.SkipGet() {
				t.Run("Get", func(t *testing.T) {
					configGot := gnmi.Get(t, dut, config.Config())
					if *configGot.SourceFlowLabel != input {
						t.Errorf("Config /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/source-flow-label: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe", func(t *testing.T) {
					stateGot := gnmi.Get(t, dut, state.State())
					if *stateGot.SourceFlowLabel != input {
						t.Errorf("State /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/source-flow-label: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				if !setup.SkipSubscribe() {
					if qs, _ := gnmi.LookupConfig(t, dut, config.Config()).Val(); qs.SourceFlowLabel != nil {
						t.Errorf("Delete /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/source-flow-label fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestDscpSet(t *testing.T) {
	t.Skip() // Skip till CSCwb98756 is fixed
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupAcl(t, dut)
	defer teardownAcl(t, dut, baseConfig)

	inputs := [][]uint8{
		{
			10,
		},
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/dscp-set using value %v", input), func(t *testing.T) {
			baseConfigAclSet := setup.GetAnyValue(baseConfig.AclSet)
			baseConfigAclSetAclEntry := setup.GetAnyValue(baseConfigAclSet.AclEntry)
			baseConfigAclSetAclEntryIpv6 := baseConfigAclSetAclEntry.Ipv6
			baseConfigAclSetAclEntryIpv6.DscpSet = input
			baseConfigAclSetAclEntryIpv6.Dscp = nil

			config := gnmi.OC().Acl().AclSet(*baseConfigAclSet.Name, baseConfigAclSet.Type).AclEntry(*baseConfigAclSetAclEntry.SequenceId).Ipv6()
			state := gnmi.OC().Acl().AclSet(*baseConfigAclSet.Name, baseConfigAclSet.Type).AclEntry(*baseConfigAclSetAclEntry.SequenceId).Ipv6()

			t.Run("Replace", func(t *testing.T) {
				gnmi.Replace(t, dut, config.Config(), baseConfigAclSetAclEntryIpv6)
			})
			if !setup.SkipGet() {
				t.Run("Get", func(t *testing.T) {
					configGot := gnmi.Get(t, dut, config.Config())
					for i, cg := range configGot.DscpSet {
						if cg != input[i] {
							t.Errorf("Config /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/dscp-set: got %v, want %v", cg, input[i])
						}
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe", func(t *testing.T) {
					stateGot := gnmi.Get(t, dut, state.State())
					for i, sg := range stateGot.DscpSet {
						if sg != input[i] {
							t.Errorf("Config /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/dscp-set: got %v, want %v", sg, input[i])
						}
					}
				})
			}
			t.Run("Delete", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				if !setup.SkipSubscribe() {
					if qs, _ := gnmi.LookupConfig(t, dut, config.Config()).Val(); qs.DscpSet != nil {
						t.Errorf("Delete /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/dscp-set fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestDestinationFlowLabel(t *testing.T) {
	t.Skip()
	dut := ondatra.DUT(t, "dut")

	baseConfig := setupAcl(t, dut)
	defer teardownAcl(t, dut, baseConfig)

	inputs := []uint32{
		832101,
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/destination-flow-label using value %v", input), func(t *testing.T) {
			baseConfigAclSet := setup.GetAnyValue(baseConfig.AclSet)
			baseConfigAclSetAclEntry := setup.GetAnyValue(baseConfigAclSet.AclEntry)
			baseConfigAclSetAclEntryIpv6 := baseConfigAclSetAclEntry.Ipv6
			*baseConfigAclSetAclEntryIpv6.DestinationFlowLabel = input

			config := gnmi.OC().Acl().AclSet(*baseConfigAclSet.Name, baseConfigAclSet.Type).AclEntry(*baseConfigAclSetAclEntry.SequenceId).Ipv6()
			state := gnmi.OC().Acl().AclSet(*baseConfigAclSet.Name, baseConfigAclSet.Type).AclEntry(*baseConfigAclSetAclEntry.SequenceId).Ipv6()

			t.Run("Replace", func(t *testing.T) {
				gnmi.Replace(t, dut, config.Config(), baseConfigAclSetAclEntryIpv6)
			})
			if !setup.SkipGet() {
				t.Run("Get", func(t *testing.T) {
					configGot := gnmi.Get(t, dut, config.Config())
					if *configGot.DestinationFlowLabel != input {
						t.Errorf("Config /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/destination-flow-label: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe", func(t *testing.T) {
					stateGot := gnmi.Get(t, dut, state.State())
					if *stateGot.DestinationFlowLabel != input {
						t.Errorf("State /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/destination-flow-label: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				if !setup.SkipSubscribe() {
					if qs, _ := gnmi.LookupConfig(t, dut, config.Config()).Val(); qs.DestinationFlowLabel != nil {
						t.Errorf("Delete /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/destination-flow-label fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestDestinationAddress(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	baseConfig := setupAcl(t, dut)
	defer teardownAcl(t, dut, baseConfig)

	inputs := []string{
		"D::221F/64",
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/destination-address using value %v", input), func(t *testing.T) {
			baseConfigAclSet := setup.GetAnyValue(baseConfig.AclSet)
			baseConfigAclSetAclEntry := setup.GetAnyValue(baseConfigAclSet.AclEntry)
			baseConfigAclSetAclEntryIpv6 := baseConfigAclSetAclEntry.Ipv6
			*baseConfigAclSetAclEntryIpv6.DestinationAddress = input

			config := gnmi.OC().Acl().AclSet(*baseConfigAclSet.Name, baseConfigAclSet.Type).AclEntry(*baseConfigAclSetAclEntry.SequenceId).Ipv6()
			state := gnmi.OC().Acl().AclSet(*baseConfigAclSet.Name, baseConfigAclSet.Type).AclEntry(*baseConfigAclSetAclEntry.SequenceId).Ipv6()

			t.Run("Replace", func(t *testing.T) {
				gnmi.Replace(t, dut, config.Config(), baseConfigAclSetAclEntryIpv6)
			})
			if !setup.SkipGet() {
				t.Run("Get", func(t *testing.T) {
					configGot := gnmi.Get(t, dut, config.Config())
					if *configGot.DestinationAddress != input {
						t.Errorf("Config /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/destination-address: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe", func(t *testing.T) {
					stateGot := gnmi.Get(t, dut, state.State())
					if *stateGot.DestinationAddress != input {
						t.Errorf("State /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/destination-address: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				if !setup.SkipSubscribe() {
					if qs, _ := gnmi.LookupConfig(t, dut, config.Config()).Val(); qs.DestinationAddress != nil {
						t.Errorf("Delete /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/destination-address fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestDscp(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	baseConfig := setupAcl(t, dut)
	defer teardownAcl(t, dut, baseConfig)

	inputs := []uint8{
		10,
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/dscp using value %v", input), func(t *testing.T) {
			baseConfigAclSet := setup.GetAnyValue(baseConfig.AclSet)
			baseConfigAclSetAclEntry := setup.GetAnyValue(baseConfigAclSet.AclEntry)
			baseConfigAclSetAclEntryIpv6 := baseConfigAclSetAclEntry.Ipv6
			*baseConfigAclSetAclEntryIpv6.Dscp = input

			config := gnmi.OC().Acl().AclSet(*baseConfigAclSet.Name, baseConfigAclSet.Type).AclEntry(*baseConfigAclSetAclEntry.SequenceId).Ipv6()
			state := gnmi.OC().Acl().AclSet(*baseConfigAclSet.Name, baseConfigAclSet.Type).AclEntry(*baseConfigAclSetAclEntry.SequenceId).Ipv6()

			t.Run("Replace", func(t *testing.T) {
				gnmi.Replace(t, dut, config.Config(), baseConfigAclSetAclEntryIpv6)
			})
			if !setup.SkipGet() {
				t.Run("Get", func(t *testing.T) {
					configGot := gnmi.Get(t, dut, config.Config())
					if *configGot.Dscp != input {
						t.Errorf("Config /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/dscp: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe", func(t *testing.T) {
					stateGot := gnmi.Get(t, dut, state.State())
					if *stateGot.Dscp != input {
						t.Errorf("State /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/dscp: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				if !setup.SkipSubscribe() {
					if qs, _ := gnmi.LookupConfig(t, dut, config.Config()).Val(); qs.Dscp != nil {
						t.Errorf("Delete /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/dscp fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestHopLimit(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	baseConfig := setupAcl(t, dut)
	defer teardownAcl(t, dut, baseConfig)

	inputs := []uint8{
		6,
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/hop-limit using value %v", input), func(t *testing.T) {
			baseConfigAclSet := setup.GetAnyValue(baseConfig.AclSet)
			baseConfigAclSetAclEntry := setup.GetAnyValue(baseConfigAclSet.AclEntry)
			baseConfigAclSetAclEntryIpv6 := baseConfigAclSetAclEntry.Ipv6
			*baseConfigAclSetAclEntryIpv6.HopLimit = input

			config := gnmi.OC().Acl().AclSet(*baseConfigAclSet.Name, baseConfigAclSet.Type).AclEntry(*baseConfigAclSetAclEntry.SequenceId).Ipv6()
			state := gnmi.OC().Acl().AclSet(*baseConfigAclSet.Name, baseConfigAclSet.Type).AclEntry(*baseConfigAclSetAclEntry.SequenceId).Ipv6()

			t.Run("Replace", func(t *testing.T) {
				gnmi.Replace(t, dut, config.Config(), baseConfigAclSetAclEntryIpv6)
			})
			if !setup.SkipGet() {
				t.Run("Get", func(t *testing.T) {
					configGot := gnmi.Get(t, dut, config.Config())
					if *configGot.HopLimit != input {
						t.Errorf("Config /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/hop-limit: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe", func(t *testing.T) {
					stateGot := gnmi.Get(t, dut, state.State())
					if *stateGot.HopLimit != input {
						t.Errorf("State /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/hop-limit: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				if !setup.SkipSubscribe() {
					if qs, _ := gnmi.LookupConfig(t, dut, config.Config()).Val(); qs.HopLimit != nil {
						t.Errorf("Delete /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/hop-limit fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestSourceAddress(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	baseConfig := setupAcl(t, dut)
	defer teardownAcl(t, dut, baseConfig)

	inputs := []string{
		"Ae::/64",
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/source-address using value %v", input), func(t *testing.T) {
			baseConfigAclSet := setup.GetAnyValue(baseConfig.AclSet)
			baseConfigAclSetAclEntry := setup.GetAnyValue(baseConfigAclSet.AclEntry)
			baseConfigAclSetAclEntryIpv6 := baseConfigAclSetAclEntry.Ipv6
			*baseConfigAclSetAclEntryIpv6.SourceAddress = input

			config := gnmi.OC().Acl().AclSet(*baseConfigAclSet.Name, baseConfigAclSet.Type).AclEntry(*baseConfigAclSetAclEntry.SequenceId).Ipv6()
			state := gnmi.OC().Acl().AclSet(*baseConfigAclSet.Name, baseConfigAclSet.Type).AclEntry(*baseConfigAclSetAclEntry.SequenceId).Ipv6()

			t.Run("Replace", func(t *testing.T) {
				gnmi.Replace(t, dut, config.Config(), baseConfigAclSetAclEntryIpv6)
			})
			if !setup.SkipGet() {
				t.Run("Get", func(t *testing.T) {
					configGot := gnmi.Get(t, dut, config.Config())
					if *configGot.SourceAddress != input {
						t.Errorf("Config /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/source-address: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe", func(t *testing.T) {
					stateGot := gnmi.Get(t, dut, state.State())
					if *stateGot.SourceAddress != input {
						t.Errorf("State /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/source-address: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				if !setup.SkipSubscribe() {
					if qs, _ := gnmi.LookupConfig(t, dut, config.Config()).Val(); qs.SourceAddress != nil {
						t.Errorf("Delete /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/source-address fail: got %v", qs)
					}
				}
			})
		})
	}
}
