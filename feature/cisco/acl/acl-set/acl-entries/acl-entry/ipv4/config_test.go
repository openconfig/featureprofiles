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
	setup.ResetStruct(bcAclSetAclEntry, []string{"Ipv4", "Actions"})
	bcAclSetAclEntryIpv4 := bcAclSetAclEntry.Ipv4
	setup.ResetStruct(bcAclSetAclEntryIpv4, []string{})
	gnmi.Replace(t, dut, gnmi.OC().Acl().Config(), bc)
	return bc
}

func teardownAcl(t *testing.T, dut *ondatra.DUTDevice, baseConfig *oc.Acl) {
	gnmi.Delete(t, dut, gnmi.OC().Acl().Config())
}
func TestDscp(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	baseConfig := setupAcl(t, dut)
	defer teardownAcl(t, dut, baseConfig)

	inputs := []uint8{
		10,
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv4/config/dscp using value %v", input), func(t *testing.T) {
			baseConfigAclSet := setup.GetAnyValue(baseConfig.AclSet)
			baseConfigAclSetAclEntry := setup.GetAnyValue(baseConfigAclSet.AclEntry)
			baseConfigAclSetAclEntryIpv4 := baseConfigAclSetAclEntry.Ipv4
			*baseConfigAclSetAclEntryIpv4.Dscp = input

			config := gnmi.OC().Acl().AclSet(*baseConfigAclSet.Name, baseConfigAclSet.Type).AclEntry(*baseConfigAclSetAclEntry.SequenceId).Ipv4()
			state := gnmi.OC().Acl().AclSet(*baseConfigAclSet.Name, baseConfigAclSet.Type).AclEntry(*baseConfigAclSetAclEntry.SequenceId).Ipv4()

			t.Run("Replace", func(t *testing.T) {
				gnmi.Replace(t, dut, config.Config(), baseConfigAclSetAclEntryIpv4)
			})
			if !setup.SkipGet() {
				t.Run("Get", func(t *testing.T) {
					configGot := gnmi.Get(t, dut, config.Config())
					if *configGot.Dscp != input {
						t.Errorf("Config /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv4/config/dscp: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe", func(t *testing.T) {
					stateGot := gnmi.Get(t, dut, state.State())
					if *stateGot.Dscp != input {
						t.Errorf("State /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv4/config/dscp: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				if !setup.SkipSubscribe() {
					if qs, _ := gnmi.LookupConfig(t, dut, config.Config()).Val(); qs.Dscp != nil {
						t.Errorf("Delete /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv4/config/dscp fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestProtocol(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	baseConfig := setupAcl(t, dut)
	defer teardownAcl(t, dut, baseConfig)

	inputs := []oc.Acl_AclSet_AclEntry_Ipv4_Protocol_Union{
		oc.UnionUint8(6),
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv4/config/protocol using value %v", input), func(t *testing.T) {
			baseConfigAclSet := setup.GetAnyValue(baseConfig.AclSet)
			baseConfigAclSetAclEntry := setup.GetAnyValue(baseConfigAclSet.AclEntry)
			baseConfigAclSetAclEntryIpv4 := baseConfigAclSetAclEntry.Ipv4
			baseConfigAclSetAclEntryIpv4.Protocol = input

			config := gnmi.OC().Acl().AclSet(*baseConfigAclSet.Name, baseConfigAclSet.Type).AclEntry(*baseConfigAclSetAclEntry.SequenceId).Ipv4()
			state := gnmi.OC().Acl().AclSet(*baseConfigAclSet.Name, baseConfigAclSet.Type).AclEntry(*baseConfigAclSetAclEntry.SequenceId).Ipv4()

			t.Run("Replace", func(t *testing.T) {
				gnmi.Replace(t, dut, config.Config(), baseConfigAclSetAclEntryIpv4)
			})
			if !setup.SkipGet() {
				t.Run("Get", func(t *testing.T) {
					configGot := gnmi.Get(t, dut, config.Config())
					if configGot.Protocol != input {
						t.Errorf("Config /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv4/config/protocol: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe", func(t *testing.T) {
					stateGot := gnmi.Get(t, dut, state.State())
					if stateGot.Protocol != input {
						t.Errorf("State /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv4/config/protocol: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				if !setup.SkipSubscribe() {
					if qs, _ := gnmi.LookupConfig(t, dut, config.Config()).Val(); qs.Protocol != nil {
						t.Errorf("Delete /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv4/config/protocol fail: got %v", qs)
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
		"190.74.38.75/32",
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv4/config/source-address using value %v", input), func(t *testing.T) {
			baseConfigAclSet := setup.GetAnyValue(baseConfig.AclSet)
			baseConfigAclSetAclEntry := setup.GetAnyValue(baseConfigAclSet.AclEntry)
			baseConfigAclSetAclEntryIpv4 := baseConfigAclSetAclEntry.Ipv4
			*baseConfigAclSetAclEntryIpv4.SourceAddress = input

			config := gnmi.OC().Acl().AclSet(*baseConfigAclSet.Name, baseConfigAclSet.Type).AclEntry(*baseConfigAclSetAclEntry.SequenceId).Ipv4()
			state := gnmi.OC().Acl().AclSet(*baseConfigAclSet.Name, baseConfigAclSet.Type).AclEntry(*baseConfigAclSetAclEntry.SequenceId).Ipv4()

			t.Run("Replace", func(t *testing.T) {
				gnmi.Replace(t, dut, config.Config(), baseConfigAclSetAclEntryIpv4)
			})
			if !setup.SkipGet() {
				t.Run("Get", func(t *testing.T) {
					configGot := gnmi.Get(t, dut, config.Config())
					if *configGot.SourceAddress != input {
						t.Errorf("Config /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv4/config/source-address: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe", func(t *testing.T) {
					stateGot := gnmi.Get(t, dut, state.State())
					if *stateGot.SourceAddress != input {
						t.Errorf("State /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv4/config/source-address: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				if !setup.SkipSubscribe() {
					if qs, _ := gnmi.LookupConfig(t, dut, config.Config()).Val(); qs.SourceAddress != nil {
						t.Errorf("Delete /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv4/config/source-address fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestDscpSet(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	baseConfig := setupAcl(t, dut)
	defer teardownAcl(t, dut, baseConfig)

	inputs := [][]uint8{
		{
			10,
		},
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv4/config/dscp-set using value %v", input), func(t *testing.T) {
			baseConfigAclSet := setup.GetAnyValue(baseConfig.AclSet)
			baseConfigAclSetAclEntry := setup.GetAnyValue(baseConfigAclSet.AclEntry)
			baseConfigAclSetAclEntryIpv4 := baseConfigAclSetAclEntry.Ipv4
			baseConfigAclSetAclEntryIpv4.Dscp = nil
			baseConfigAclSetAclEntryIpv4.DscpSet = input

			config := gnmi.OC().Acl().AclSet(*baseConfigAclSet.Name, baseConfigAclSet.Type).AclEntry(*baseConfigAclSetAclEntry.SequenceId).Ipv4()
			state := gnmi.OC().Acl().AclSet(*baseConfigAclSet.Name, baseConfigAclSet.Type).AclEntry(*baseConfigAclSetAclEntry.SequenceId).Ipv4()

			t.Run("Replace", func(t *testing.T) {
				gnmi.Replace(t, dut, config.Config(), baseConfigAclSetAclEntryIpv4)
			})
			if !setup.SkipGet() {
				t.Run("Get", func(t *testing.T) {
					configGot := gnmi.Get(t, dut, config.Config())
					for i, cg := range configGot.DscpSet {
						if cg != input[i] {
							t.Errorf("Config /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv4/config/dscp-set: got %v, want %v", cg, input[i])
						}
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe", func(t *testing.T) {
					stateGot := gnmi.Get(t, dut, state.State())
					for i, sg := range stateGot.DscpSet {
						if sg != input[i] {
							t.Errorf("Config /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv4/config/dscp-set: got %v, want %v", sg, input[i])
						}
					}
				})
			}
			t.Run("Delete", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				if !setup.SkipSubscribe() {
					if qs, _ := gnmi.LookupConfig(t, dut, config.Config()).Val(); qs.DscpSet != nil {
						t.Errorf("Delete /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv4/config/dscp-set fail: got %v", qs)
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
		"4.129.101.154/32",
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv4/config/destination-address using value %v", input), func(t *testing.T) {
			baseConfigAclSet := setup.GetAnyValue(baseConfig.AclSet)
			baseConfigAclSetAclEntry := setup.GetAnyValue(baseConfigAclSet.AclEntry)
			baseConfigAclSetAclEntryIpv4 := baseConfigAclSetAclEntry.Ipv4
			*baseConfigAclSetAclEntryIpv4.DestinationAddress = input

			config := gnmi.OC().Acl().AclSet(*baseConfigAclSet.Name, baseConfigAclSet.Type).AclEntry(*baseConfigAclSetAclEntry.SequenceId).Ipv4()
			state := gnmi.OC().Acl().AclSet(*baseConfigAclSet.Name, baseConfigAclSet.Type).AclEntry(*baseConfigAclSetAclEntry.SequenceId).Ipv4()

			t.Run("Replace", func(t *testing.T) {
				gnmi.Replace(t, dut, config.Config(), baseConfigAclSetAclEntryIpv4)
			})
			if !setup.SkipGet() {
				t.Run("Get", func(t *testing.T) {
					configGot := gnmi.Get(t, dut, config.Config())
					if *configGot.DestinationAddress != input {
						t.Errorf("Config /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv4/config/destination-address: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe", func(t *testing.T) {
					stateGot := gnmi.Get(t, dut, state.State())
					if *stateGot.DestinationAddress != input {
						t.Errorf("State /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv4/config/destination-address: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				if !setup.SkipSubscribe() {
					if qs, _ := gnmi.LookupConfig(t, dut, config.Config()).Val(); qs.DestinationAddress != nil {
						t.Errorf("Delete /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv4/config/destination-address fail: got %v", qs)
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
		10,
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv4/config/hop-limit using value %v", input), func(t *testing.T) {
			baseConfigAclSet := setup.GetAnyValue(baseConfig.AclSet)
			baseConfigAclSetAclEntry := setup.GetAnyValue(baseConfigAclSet.AclEntry)
			baseConfigAclSetAclEntryIpv4 := baseConfigAclSetAclEntry.Ipv4
			*baseConfigAclSetAclEntryIpv4.HopLimit = input

			config := gnmi.OC().Acl().AclSet(*baseConfigAclSet.Name, baseConfigAclSet.Type).AclEntry(*baseConfigAclSetAclEntry.SequenceId).Ipv4()
			state := gnmi.OC().Acl().AclSet(*baseConfigAclSet.Name, baseConfigAclSet.Type).AclEntry(*baseConfigAclSetAclEntry.SequenceId).Ipv4()

			t.Run("Replace", func(t *testing.T) {
				gnmi.Replace(t, dut, config.Config(), baseConfigAclSetAclEntryIpv4)
			})
			if !setup.SkipGet() {
				t.Run("Get", func(t *testing.T) {
					configGot := gnmi.Get(t, dut, config.Config())
					if *configGot.HopLimit != input {
						t.Errorf("Config /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv4/config/hop-limit: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe", func(t *testing.T) {
					stateGot := gnmi.Get(t, dut, state.State())
					if *stateGot.HopLimit != input {
						t.Errorf("State /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv4/config/hop-limit: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				if !setup.SkipSubscribe() {
					if qs, _ := gnmi.LookupConfig(t, dut, config.Config()).Val(); qs.HopLimit != nil {
						t.Errorf("Delete /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv4/config/hop-limit fail: got %v", qs)
					}
				}
			})
		})
	}
}
