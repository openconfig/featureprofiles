
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
	setup.ResetStruct(bcAclSetAclEntry, []string{"InputInterface", "Actions"})
	bcAclSetAclEntryInputInterface := bcAclSetAclEntry.InputInterface
	setup.ResetStruct(bcAclSetAclEntryInputInterface, []string{"InterfaceRef"})
	bcAclSetAclEntryInputInterfaceInterfaceRef := bcAclSetAclEntryInputInterface.InterfaceRef
	setup.ResetStruct(bcAclSetAclEntryInputInterfaceInterfaceRef, []string{})
	dut.Config().Acl().Replace(t, bc)
	return bc
}

func teardownAcl(t *testing.T, dut *ondatra.DUTDevice, baseConfig *oc.Acl) {
	dut.Config().Acl().Delete(t)
}
func TestInterface(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	
	baseConfig := setupAcl(t, dut)
	defer teardownAcl(t, dut, baseConfig)

	inputs := []string {
		"::", 
	}
	

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /acl/acl-sets/acl-set/acl-entries/acl-entry/input-interface/interface-ref/config/interface using value %v", input) , func(t *testing.T) {
			baseConfigAclSet := setup.GetAnyValue(baseConfig.AclSet)
			baseConfigAclSetAclEntry := setup.GetAnyValue(baseConfigAclSet.AclEntry)
			baseConfigAclSetAclEntryInputInterface := baseConfigAclSetAclEntry.InputInterface
			baseConfigAclSetAclEntryInputInterfaceInterfaceRef := baseConfigAclSetAclEntryInputInterface.InterfaceRef
			*baseConfigAclSetAclEntryInputInterfaceInterfaceRef.Interface = input 

			config := dut.Config().Acl().AclSet(*baseConfigAclSet.Name,baseConfigAclSet.Type,).AclEntry(*baseConfigAclSetAclEntry.SequenceId,).InputInterface().InterfaceRef()
			state := dut.Telemetry().Acl().AclSet(*baseConfigAclSet.Name,baseConfigAclSet.Type,).AclEntry(*baseConfigAclSetAclEntry.SequenceId,).InputInterface().InterfaceRef()

			t.Run("Replace", func(t *testing.T) {
				config.Replace(t, baseConfigAclSetAclEntryInputInterfaceInterfaceRef)
			})
			if !setup.SkipGet() {
				t.Run("Get", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.Interface != input {
						t.Errorf("Config /acl/acl-sets/acl-set/acl-entries/acl-entry/input-interface/interface-ref/config/interface: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.Interface != input {
						t.Errorf("State /acl/acl-sets/acl-set/acl-entries/acl-entry/input-interface/interface-ref/config/interface: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).Interface != nil {
						t.Errorf("Delete /acl/acl-sets/acl-set/acl-entries/acl-entry/input-interface/interface-ref/config/interface fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestSubinterface(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	
	baseConfig := setupAcl(t, dut)
	defer teardownAcl(t, dut, baseConfig)

	inputs := []uint32 {
		4030193341, 
	}
	

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /acl/acl-sets/acl-set/acl-entries/acl-entry/input-interface/interface-ref/config/subinterface using value %v", input) , func(t *testing.T) {
			baseConfigAclSet := setup.GetAnyValue(baseConfig.AclSet)
			baseConfigAclSetAclEntry := setup.GetAnyValue(baseConfigAclSet.AclEntry)
			baseConfigAclSetAclEntryInputInterface := baseConfigAclSetAclEntry.InputInterface
			baseConfigAclSetAclEntryInputInterfaceInterfaceRef := baseConfigAclSetAclEntryInputInterface.InterfaceRef
			*baseConfigAclSetAclEntryInputInterfaceInterfaceRef.Subinterface = input 

			config := dut.Config().Acl().AclSet(*baseConfigAclSet.Name,baseConfigAclSet.Type,).AclEntry(*baseConfigAclSetAclEntry.SequenceId,).InputInterface().InterfaceRef()
			state := dut.Telemetry().Acl().AclSet(*baseConfigAclSet.Name,baseConfigAclSet.Type,).AclEntry(*baseConfigAclSetAclEntry.SequenceId,).InputInterface().InterfaceRef()

			t.Run("Replace", func(t *testing.T) {
				config.Replace(t, baseConfigAclSetAclEntryInputInterfaceInterfaceRef)
			})
			if !setup.SkipGet() {
				t.Run("Get", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.Subinterface != input {
						t.Errorf("Config /acl/acl-sets/acl-set/acl-entries/acl-entry/input-interface/interface-ref/config/subinterface: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.Subinterface != input {
						t.Errorf("State /acl/acl-sets/acl-set/acl-entries/acl-entry/input-interface/interface-ref/config/subinterface: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).Subinterface != nil {
						t.Errorf("Delete /acl/acl-sets/acl-set/acl-entries/acl-entry/input-interface/interface-ref/config/subinterface fail: got %v", qs)
					}
				}
			})
		})
	}
}
