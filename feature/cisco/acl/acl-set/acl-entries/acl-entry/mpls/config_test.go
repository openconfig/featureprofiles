
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
	setup.ResetStruct(bcAclSetAclEntry, []string{"Mpls", "Actions"})
	bcAclSetAclEntryMpls := bcAclSetAclEntry.Mpls
	setup.ResetStruct(bcAclSetAclEntryMpls, []string{})
	dut.Config().Acl().Replace(t, bc)
	return bc
}

func teardownAcl(t *testing.T, dut *ondatra.DUTDevice, baseConfig *oc.Acl) {
	dut.Config().Acl().Delete(t)
}
func TestStartLabelValue(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	
	baseConfig := setupAcl(t, dut)
	defer teardownAcl(t, dut, baseConfig)

	inputs := []oc.Acl_AclSet_AclEntry_Mpls_StartLabelValue_Union {
		oc.UnionUint32(871788), 
	}
	

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /acl/acl-sets/acl-set/acl-entries/acl-entry/mpls/config/start-label-value using value %v", input) , func(t *testing.T) {
			baseConfigAclSet := setup.GetAnyValue(baseConfig.AclSet)
			baseConfigAclSetAclEntry := setup.GetAnyValue(baseConfigAclSet.AclEntry)
			baseConfigAclSetAclEntryMpls := baseConfigAclSetAclEntry.Mpls
			baseConfigAclSetAclEntryMpls.StartLabelValue = input 

			config := dut.Config().Acl().AclSet(*baseConfigAclSet.Name,baseConfigAclSet.Type,).AclEntry(*baseConfigAclSetAclEntry.SequenceId,).Mpls()
			state := dut.Telemetry().Acl().AclSet(*baseConfigAclSet.Name,baseConfigAclSet.Type,).AclEntry(*baseConfigAclSetAclEntry.SequenceId,).Mpls()

			t.Run("Replace", func(t *testing.T) {
				config.Replace(t, baseConfigAclSetAclEntryMpls)
			})
			if !setup.SkipGet() {
				t.Run("Get", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot.StartLabelValue != input {
						t.Errorf("Config /acl/acl-sets/acl-set/acl-entries/acl-entry/mpls/config/start-label-value: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot.StartLabelValue != input {
						t.Errorf("State /acl/acl-sets/acl-set/acl-entries/acl-entry/mpls/config/start-label-value: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).StartLabelValue != nil {
						t.Errorf("Delete /acl/acl-sets/acl-set/acl-entries/acl-entry/mpls/config/start-label-value fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestTrafficClass(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	
	baseConfig := setupAcl(t, dut)
	defer teardownAcl(t, dut, baseConfig)

	inputs := []uint8 {
		3, 
	}
	

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /acl/acl-sets/acl-set/acl-entries/acl-entry/mpls/config/traffic-class using value %v", input) , func(t *testing.T) {
			baseConfigAclSet := setup.GetAnyValue(baseConfig.AclSet)
			baseConfigAclSetAclEntry := setup.GetAnyValue(baseConfigAclSet.AclEntry)
			baseConfigAclSetAclEntryMpls := baseConfigAclSetAclEntry.Mpls
			*baseConfigAclSetAclEntryMpls.TrafficClass = input 

			config := dut.Config().Acl().AclSet(*baseConfigAclSet.Name,baseConfigAclSet.Type,).AclEntry(*baseConfigAclSetAclEntry.SequenceId,).Mpls()
			state := dut.Telemetry().Acl().AclSet(*baseConfigAclSet.Name,baseConfigAclSet.Type,).AclEntry(*baseConfigAclSetAclEntry.SequenceId,).Mpls()

			t.Run("Replace", func(t *testing.T) {
				config.Replace(t, baseConfigAclSetAclEntryMpls)
			})
			if !setup.SkipGet() {
				t.Run("Get", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.TrafficClass != input {
						t.Errorf("Config /acl/acl-sets/acl-set/acl-entries/acl-entry/mpls/config/traffic-class: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.TrafficClass != input {
						t.Errorf("State /acl/acl-sets/acl-set/acl-entries/acl-entry/mpls/config/traffic-class: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).TrafficClass != nil {
						t.Errorf("Delete /acl/acl-sets/acl-set/acl-entries/acl-entry/mpls/config/traffic-class fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestEndLabelValue(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	
	baseConfig := setupAcl(t, dut)
	defer teardownAcl(t, dut, baseConfig)

	inputs := []oc.Acl_AclSet_AclEntry_Mpls_EndLabelValue_Union {
		oc.UnionUint32(816728), 
	}
	

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /acl/acl-sets/acl-set/acl-entries/acl-entry/mpls/config/end-label-value using value %v", input) , func(t *testing.T) {
			baseConfigAclSet := setup.GetAnyValue(baseConfig.AclSet)
			baseConfigAclSetAclEntry := setup.GetAnyValue(baseConfigAclSet.AclEntry)
			baseConfigAclSetAclEntryMpls := baseConfigAclSetAclEntry.Mpls
			baseConfigAclSetAclEntryMpls.EndLabelValue = input 

			config := dut.Config().Acl().AclSet(*baseConfigAclSet.Name,baseConfigAclSet.Type,).AclEntry(*baseConfigAclSetAclEntry.SequenceId,).Mpls()
			state := dut.Telemetry().Acl().AclSet(*baseConfigAclSet.Name,baseConfigAclSet.Type,).AclEntry(*baseConfigAclSetAclEntry.SequenceId,).Mpls()

			t.Run("Replace", func(t *testing.T) {
				config.Replace(t, baseConfigAclSetAclEntryMpls)
			})
			if !setup.SkipGet() {
				t.Run("Get", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot.EndLabelValue != input {
						t.Errorf("Config /acl/acl-sets/acl-set/acl-entries/acl-entry/mpls/config/end-label-value: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot.EndLabelValue != input {
						t.Errorf("State /acl/acl-sets/acl-set/acl-entries/acl-entry/mpls/config/end-label-value: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).EndLabelValue != nil {
						t.Errorf("Delete /acl/acl-sets/acl-set/acl-entries/acl-entry/mpls/config/end-label-value fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestTtlValue(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	
	baseConfig := setupAcl(t, dut)
	defer teardownAcl(t, dut, baseConfig)

	inputs := []uint8 {
		59, 
	}
	

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /acl/acl-sets/acl-set/acl-entries/acl-entry/mpls/config/ttl-value using value %v", input) , func(t *testing.T) {
			baseConfigAclSet := setup.GetAnyValue(baseConfig.AclSet)
			baseConfigAclSetAclEntry := setup.GetAnyValue(baseConfigAclSet.AclEntry)
			baseConfigAclSetAclEntryMpls := baseConfigAclSetAclEntry.Mpls
			*baseConfigAclSetAclEntryMpls.TtlValue = input 

			config := dut.Config().Acl().AclSet(*baseConfigAclSet.Name,baseConfigAclSet.Type,).AclEntry(*baseConfigAclSetAclEntry.SequenceId,).Mpls()
			state := dut.Telemetry().Acl().AclSet(*baseConfigAclSet.Name,baseConfigAclSet.Type,).AclEntry(*baseConfigAclSetAclEntry.SequenceId,).Mpls()

			t.Run("Replace", func(t *testing.T) {
				config.Replace(t, baseConfigAclSetAclEntryMpls)
			})
			if !setup.SkipGet() {
				t.Run("Get", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.TtlValue != input {
						t.Errorf("Config /acl/acl-sets/acl-set/acl-entries/acl-entry/mpls/config/ttl-value: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.TtlValue != input {
						t.Errorf("State /acl/acl-sets/acl-set/acl-entries/acl-entry/mpls/config/ttl-value: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).TtlValue != nil {
						t.Errorf("Delete /acl/acl-sets/acl-set/acl-entries/acl-entry/mpls/config/ttl-value fail: got %v", qs)
					}
				}
			})
		})
	}
}
