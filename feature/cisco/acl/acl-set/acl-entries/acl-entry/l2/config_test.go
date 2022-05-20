package acl_test

import (
	"fmt"
	"testing"

	"github.com/openconfig/featureprofiles/feature/cisco/acl/setup"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	oc "github.com/openconfig/ondatra/telemetry"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func setupAcl(t *testing.T, dut *ondatra.DUTDevice) *oc.Acl {
	bc := new(oc.Acl)
	*bc = setup.BaseConfig()
	setup.ResetStruct(bc, []string{"AclSet"})
	bcAclSet := setup.GetAnyValue(bc.AclSet)
	bcAclSet.Type = oc.E_Acl_ACL_TYPE(3) // L3
	setup.ResetStruct(bcAclSet, []string{"AclEntry"})
	bcAclSetAclEntry := setup.GetAnyValue(bcAclSet.AclEntry)
	setup.ResetStruct(bcAclSetAclEntry, []string{"L2", "Actions"})
	bcAclSetAclEntry.Actions.LogAction = oc.E_Acl_LOG_ACTION(0)
	bcAclSetAclEntryL2 := bcAclSetAclEntry.L2
	setup.ResetStruct(bcAclSetAclEntryL2, []string{})
	dut.Config().Acl().Replace(t, bc)
	return bc
}

func teardownAcl(t *testing.T, dut *ondatra.DUTDevice, baseConfig *oc.Acl) {
	dut.Config().Acl().Delete(t)
}
func TestDestinationMacMask(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	baseConfig := setupAcl(t, dut)
	defer teardownAcl(t, dut, baseConfig)

	inputs := []string{
		"Ff:a2:20:9B:5D:00",
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /acl/acl-sets/acl-set/acl-entries/acl-entry/l2/config/destination-mac-mask using value %v", input), func(t *testing.T) {
			baseConfigAclSet := setup.GetAnyValue(baseConfig.AclSet)
			baseConfigAclSetAclEntry := setup.GetAnyValue(baseConfigAclSet.AclEntry)
			baseConfigAclSetAclEntryL2 := baseConfigAclSetAclEntry.L2
			*baseConfigAclSetAclEntryL2.DestinationMacMask = input

			config := dut.Config().Acl().AclSet(*baseConfigAclSet.Name, baseConfigAclSet.Type).AclEntry(*baseConfigAclSetAclEntry.SequenceId).L2()
			state := dut.Telemetry().Acl().AclSet(*baseConfigAclSet.Name, baseConfigAclSet.Type).AclEntry(*baseConfigAclSetAclEntry.SequenceId).L2()

			t.Run("Replace", func(t *testing.T) {
				config.Replace(t, baseConfigAclSetAclEntryL2)
			})
			if !setup.SkipGet() {
				t.Run("Get", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.DestinationMacMask != input {
						t.Errorf("Config /acl/acl-sets/acl-set/acl-entries/acl-entry/l2/config/destination-mac-mask: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.DestinationMacMask != input {
						t.Errorf("State /acl/acl-sets/acl-set/acl-entries/acl-entry/l2/config/destination-mac-mask: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).DestinationMacMask != nil {
						t.Errorf("Delete /acl/acl-sets/acl-set/acl-entries/acl-entry/l2/config/destination-mac-mask fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestEthertype(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	baseConfig := setupAcl(t, dut)
	defer teardownAcl(t, dut, baseConfig)

	inputs := []oc.Acl_AclSet_AclEntry_L2_Ethertype_Union{
		oc.PacketMatchTypes_ETHERTYPE_ETHERTYPE_ARP,
		//oc.PacketMatchTypes_ETHERTYPE_ETHERTYPE_IPV4,
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /acl/acl-sets/acl-set/acl-entries/acl-entry/l2/config/ethertype using value %v", input), func(t *testing.T) {
			baseConfigAclSet := setup.GetAnyValue(baseConfig.AclSet)
			baseConfigAclSetAclEntry := setup.GetAnyValue(baseConfigAclSet.AclEntry)
			baseConfigAclSetAclEntryL2 := baseConfigAclSetAclEntry.L2
			baseConfigAclSetAclEntryL2.Ethertype = input

			config := dut.Config().Acl().AclSet(*baseConfigAclSet.Name, baseConfigAclSet.Type).AclEntry(*baseConfigAclSetAclEntry.SequenceId).L2()
			state := dut.Telemetry().Acl().AclSet(*baseConfigAclSet.Name, baseConfigAclSet.Type).AclEntry(*baseConfigAclSetAclEntry.SequenceId).L2()

			t.Run("Replace", func(t *testing.T) {
				config.Replace(t, baseConfigAclSetAclEntryL2)
			})
			if !setup.SkipGet() {
				t.Run("Get", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot.Ethertype != input {
						t.Errorf("Config /acl/acl-sets/acl-set/acl-entries/acl-entry/l2/config/ethertype: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot.Ethertype != input {
						t.Errorf("State /acl/acl-sets/acl-set/acl-entries/acl-entry/l2/config/ethertype: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).Ethertype != nil {
						t.Errorf("Delete /acl/acl-sets/acl-set/acl-entries/acl-entry/l2/config/ethertype fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestDestinationMac(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	baseConfig := setupAcl(t, dut)
	defer teardownAcl(t, dut, baseConfig)

	inputs := []string{
		"CA:5b:e7:CB:24:6B",
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /acl/acl-sets/acl-set/acl-entries/acl-entry/l2/config/destination-mac using value %v", input), func(t *testing.T) {
			baseConfigAclSet := setup.GetAnyValue(baseConfig.AclSet)
			baseConfigAclSetAclEntry := setup.GetAnyValue(baseConfigAclSet.AclEntry)
			baseConfigAclSetAclEntryL2 := baseConfigAclSetAclEntry.L2
			*baseConfigAclSetAclEntryL2.DestinationMac = input

			config := dut.Config().Acl().AclSet(*baseConfigAclSet.Name, baseConfigAclSet.Type).AclEntry(*baseConfigAclSetAclEntry.SequenceId).L2()
			state := dut.Telemetry().Acl().AclSet(*baseConfigAclSet.Name, baseConfigAclSet.Type).AclEntry(*baseConfigAclSetAclEntry.SequenceId).L2()

			t.Run("Replace", func(t *testing.T) {
				config.Replace(t, baseConfigAclSetAclEntryL2)
			})
			if !setup.SkipGet() {
				t.Run("Get", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.DestinationMac != input {
						t.Errorf("Config /acl/acl-sets/acl-set/acl-entries/acl-entry/l2/config/destination-mac: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.DestinationMac != input {
						t.Errorf("State /acl/acl-sets/acl-set/acl-entries/acl-entry/l2/config/destination-mac: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).DestinationMac != nil {
						t.Errorf("Delete /acl/acl-sets/acl-set/acl-entries/acl-entry/l2/config/destination-mac fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestSourceMac(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	baseConfig := setupAcl(t, dut)
	defer teardownAcl(t, dut, baseConfig)

	inputs := []string{
		"a6:dF:f3:ef:4D:C4",
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /acl/acl-sets/acl-set/acl-entries/acl-entry/l2/config/source-mac using value %v", input), func(t *testing.T) {
			baseConfigAclSet := setup.GetAnyValue(baseConfig.AclSet)
			baseConfigAclSetAclEntry := setup.GetAnyValue(baseConfigAclSet.AclEntry)
			baseConfigAclSetAclEntryL2 := baseConfigAclSetAclEntry.L2
			*baseConfigAclSetAclEntryL2.SourceMac = input

			config := dut.Config().Acl().AclSet(*baseConfigAclSet.Name, baseConfigAclSet.Type).AclEntry(*baseConfigAclSetAclEntry.SequenceId).L2()
			state := dut.Telemetry().Acl().AclSet(*baseConfigAclSet.Name, baseConfigAclSet.Type).AclEntry(*baseConfigAclSetAclEntry.SequenceId).L2()

			t.Run("Replace", func(t *testing.T) {
				config.Replace(t, baseConfigAclSetAclEntryL2)
			})
			if !setup.SkipGet() {
				t.Run("Get", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.SourceMac != input {
						t.Errorf("Config /acl/acl-sets/acl-set/acl-entries/acl-entry/l2/config/source-mac: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.SourceMac != input {
						t.Errorf("State /acl/acl-sets/acl-set/acl-entries/acl-entry/l2/config/source-mac: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).SourceMac != nil {
						t.Errorf("Delete /acl/acl-sets/acl-set/acl-entries/acl-entry/l2/config/source-mac fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestSourceMacMask(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	baseConfig := setupAcl(t, dut)
	defer teardownAcl(t, dut, baseConfig)

	inputs := []string{
		"De:3c:A6:Fa:D1:BA",
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /acl/acl-sets/acl-set/acl-entries/acl-entry/l2/config/source-mac-mask using value %v", input), func(t *testing.T) {
			baseConfigAclSet := setup.GetAnyValue(baseConfig.AclSet)
			baseConfigAclSetAclEntry := setup.GetAnyValue(baseConfigAclSet.AclEntry)
			baseConfigAclSetAclEntryL2 := baseConfigAclSetAclEntry.L2
			*baseConfigAclSetAclEntryL2.SourceMacMask = input

			config := dut.Config().Acl().AclSet(*baseConfigAclSet.Name, baseConfigAclSet.Type).AclEntry(*baseConfigAclSetAclEntry.SequenceId).L2()
			state := dut.Telemetry().Acl().AclSet(*baseConfigAclSet.Name, baseConfigAclSet.Type).AclEntry(*baseConfigAclSetAclEntry.SequenceId).L2()

			t.Run("Replace", func(t *testing.T) {
				config.Replace(t, baseConfigAclSetAclEntryL2)
			})
			if !setup.SkipGet() {
				t.Run("Get", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.SourceMacMask != input {
						t.Errorf("Config /acl/acl-sets/acl-set/acl-entries/acl-entry/l2/config/source-mac-mask: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.SourceMacMask != input {
						t.Errorf("State /acl/acl-sets/acl-set/acl-entries/acl-entry/l2/config/source-mac-mask: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).SourceMacMask != nil {
						t.Errorf("Delete /acl/acl-sets/acl-set/acl-entries/acl-entry/l2/config/source-mac-mask fail: got %v", qs)
					}
				}
			})
		})
	}
}
