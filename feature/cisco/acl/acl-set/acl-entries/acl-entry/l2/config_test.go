
package acl_test
import (
	"testing"
	"fmt"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/featureprofiles/internal/fptest"
	oc "github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/featureprofiles/feature/cisco/acl/setup"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}


func setupAcl(t *testing.T, dut *ondatra.DUTDevice) *oc.Acl {
	bc := new(oc.Acl)
	*bc = setup.BaseConfig
	setup.ResetStruct(bc, []string{"AclSet"})
	bcAclSet := setup.GetAnyValue(bc.AclSet)
	setup.ResetStruct(bcAclSet, []string{"AclEntry"})
	bcAclSetAclEntry := setup.GetAnyValue(bcAclSet.AclEntry)
	setup.ResetStruct(bcAclSetAclEntry, []string{"L2", "Actions"})
	dut.Config().Acl().Replace(t, bc)
	return bc
}

func teardownAcl(t *testing.T, dut *ondatra.DUTDevice, baseConfig *oc.Acl) {
	dut.Config().Acl().Delete(t)
}
func TestSourceMacMask(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	
	baseConfig := setupAcl(t, dut)
	defer teardownAcl(t, dut, baseConfig)

	inputs := []string {
		"83:8F:e0:cb:c7:5D", 
		"a8:81:EC:Bc:9e:04", 
	}
	

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /acl/acl-sets/acl-set/acl-entries/acl-entry/l2/config/source-mac-mask using value %v", input) , func(t *testing.T) {
			baseConfigAclSet := setup.GetAnyValue(baseConfig.AclSet)
			baseConfigAclSetAclEntry := setup.GetAnyValue(baseConfigAclSet.AclEntry)
			baseConfigAclSetAclEntryL2 := baseConfigAclSetAclEntry.L2
			*baseConfigAclSetAclEntryL2.SourceMacMask = input 

			config := dut.Config().Acl().AclSet(*baseConfigAclSet.Name,baseConfigAclSet.Type,).AclEntry(*baseConfigAclSetAclEntry.SequenceId,).L2()
			state := dut.Telemetry().Acl().AclSet(*baseConfigAclSet.Name,baseConfigAclSet.Type,).AclEntry(*baseConfigAclSetAclEntry.SequenceId,).L2()

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
				if qs := config.Lookup(t); qs.Val(t).SourceMacMask != nil {
					t.Errorf("Delete /acl/acl-sets/acl-set/acl-entries/acl-entry/l2/config/source-mac-mask fail: got %v", qs)
				}
			})
		})
	}
}
func TestDestinationMac(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	
	baseConfig := setupAcl(t, dut)
	defer teardownAcl(t, dut, baseConfig)

	inputs := []string {
		"4A:bD:44:62:45:c8", 
		"ed:eE:8D:e9:81:CA", 
	}
	

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /acl/acl-sets/acl-set/acl-entries/acl-entry/l2/config/destination-mac using value %v", input) , func(t *testing.T) {
			baseConfigAclSet := setup.GetAnyValue(baseConfig.AclSet)
			baseConfigAclSetAclEntry := setup.GetAnyValue(baseConfigAclSet.AclEntry)
			baseConfigAclSetAclEntryL2 := baseConfigAclSetAclEntry.L2
			*baseConfigAclSetAclEntryL2.DestinationMac = input 

			config := dut.Config().Acl().AclSet(*baseConfigAclSet.Name,baseConfigAclSet.Type,).AclEntry(*baseConfigAclSetAclEntry.SequenceId,).L2()
			state := dut.Telemetry().Acl().AclSet(*baseConfigAclSet.Name,baseConfigAclSet.Type,).AclEntry(*baseConfigAclSetAclEntry.SequenceId,).L2()

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
				if qs := config.Lookup(t); qs.Val(t).DestinationMac != nil {
					t.Errorf("Delete /acl/acl-sets/acl-set/acl-entries/acl-entry/l2/config/destination-mac fail: got %v", qs)
				}
			})
		})
	}
}
func TestEthertype(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	
	baseConfig := setupAcl(t, dut)
	defer teardownAcl(t, dut, baseConfig)

	inputs := []oc.Acl_AclSet_AclEntry_L2_Ethertype_Union {
		oc.UnionUint16(59629), 
		oc.UnionUint16(53744), 
	}
	

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /acl/acl-sets/acl-set/acl-entries/acl-entry/l2/config/ethertype using value %v", input) , func(t *testing.T) {
			baseConfigAclSet := setup.GetAnyValue(baseConfig.AclSet)
			baseConfigAclSetAclEntry := setup.GetAnyValue(baseConfigAclSet.AclEntry)
			baseConfigAclSetAclEntryL2 := baseConfigAclSetAclEntry.L2
			baseConfigAclSetAclEntryL2.Ethertype = input 

			config := dut.Config().Acl().AclSet(*baseConfigAclSet.Name,baseConfigAclSet.Type,).AclEntry(*baseConfigAclSetAclEntry.SequenceId,).L2()
			state := dut.Telemetry().Acl().AclSet(*baseConfigAclSet.Name,baseConfigAclSet.Type,).AclEntry(*baseConfigAclSetAclEntry.SequenceId,).L2()

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
				if qs := config.Lookup(t); qs.Val(t).Ethertype != nil {
					t.Errorf("Delete /acl/acl-sets/acl-set/acl-entries/acl-entry/l2/config/ethertype fail: got %v", qs)
				}
			})
		})
	}
}
func TestSourceMac(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	
	baseConfig := setupAcl(t, dut)
	defer teardownAcl(t, dut, baseConfig)

	inputs := []string {
		"E1:75:DB:61:ee:b8", 
		"9a:b9:E4:EE:d4:c8", 
	}
	

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /acl/acl-sets/acl-set/acl-entries/acl-entry/l2/config/source-mac using value %v", input) , func(t *testing.T) {
			baseConfigAclSet := setup.GetAnyValue(baseConfig.AclSet)
			baseConfigAclSetAclEntry := setup.GetAnyValue(baseConfigAclSet.AclEntry)
			baseConfigAclSetAclEntryL2 := baseConfigAclSetAclEntry.L2
			*baseConfigAclSetAclEntryL2.SourceMac = input 

			config := dut.Config().Acl().AclSet(*baseConfigAclSet.Name,baseConfigAclSet.Type,).AclEntry(*baseConfigAclSetAclEntry.SequenceId,).L2()
			state := dut.Telemetry().Acl().AclSet(*baseConfigAclSet.Name,baseConfigAclSet.Type,).AclEntry(*baseConfigAclSetAclEntry.SequenceId,).L2()

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
				if qs := config.Lookup(t); qs.Val(t).SourceMac != nil {
					t.Errorf("Delete /acl/acl-sets/acl-set/acl-entries/acl-entry/l2/config/source-mac fail: got %v", qs)
				}
			})
		})
	}
}
func TestDestinationMacMask(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	
	baseConfig := setupAcl(t, dut)
	defer teardownAcl(t, dut, baseConfig)

	inputs := []string {
		"e3:6e:D3:8d:a3:de", 
		"2C:DA:c5:20:3C:48", 
	}
	

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /acl/acl-sets/acl-set/acl-entries/acl-entry/l2/config/destination-mac-mask using value %v", input) , func(t *testing.T) {
			baseConfigAclSet := setup.GetAnyValue(baseConfig.AclSet)
			baseConfigAclSetAclEntry := setup.GetAnyValue(baseConfigAclSet.AclEntry)
			baseConfigAclSetAclEntryL2 := baseConfigAclSetAclEntry.L2
			*baseConfigAclSetAclEntryL2.DestinationMacMask = input 

			config := dut.Config().Acl().AclSet(*baseConfigAclSet.Name,baseConfigAclSet.Type,).AclEntry(*baseConfigAclSetAclEntry.SequenceId,).L2()
			state := dut.Telemetry().Acl().AclSet(*baseConfigAclSet.Name,baseConfigAclSet.Type,).AclEntry(*baseConfigAclSetAclEntry.SequenceId,).L2()

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
				if qs := config.Lookup(t); qs.Val(t).DestinationMacMask != nil {
					t.Errorf("Delete /acl/acl-sets/acl-set/acl-entries/acl-entry/l2/config/destination-mac-mask fail: got %v", qs)
				}
			})
		})
	}
}
