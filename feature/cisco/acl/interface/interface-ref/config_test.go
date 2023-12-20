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
	setup.ResetStruct(bc, []string{"Interface", "AclSet"})
	bcInterface := setup.GetAnyValue(bc.Interface)
	bcAclSet := setup.GetAnyValue(bc.AclSet)
	setup.ResetStruct(bcAclSet, []string{"AclEntry"})
	bcAclSetAclEntry := setup.GetAnyValue(bcAclSet.AclEntry)
	setup.ResetStruct(bcAclSetAclEntry, []string{"Actions"})
	setup.ResetStruct(bcInterface, []string{"InterfaceRef"})
	bcInterfaceInterfaceRef := bcInterface.InterfaceRef
	setup.ResetStruct(bcInterfaceInterfaceRef, []string{})
	gnmi.Replace(t, dut, gnmi.OC().Acl().Config(), bc)
	return bc
}

func teardownAcl(t *testing.T, dut *ondatra.DUTDevice, baseConfig *oc.Acl) {
	gnmi.Delete(t, dut, gnmi.OC().Acl().Config())
}
func TestSubinterface(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	baseConfig := setupAcl(t, dut)
	defer teardownAcl(t, dut, baseConfig)

	inputs := []uint32{
		0,
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /acl/interfaces/interface/interface-ref/config/subinterface using value %v", input), func(t *testing.T) {
			baseConfigInterface := setup.GetAnyValue(baseConfig.Interface)
			baseConfigInterfaceInterfaceRef := baseConfigInterface.InterfaceRef
			*baseConfigInterfaceInterfaceRef.Subinterface = input

			config := gnmi.OC().Acl().Interface(*baseConfigInterface.Id).InterfaceRef()
			state := gnmi.OC().Acl().Interface(*baseConfigInterface.Id).InterfaceRef()

			t.Run("Replace", func(t *testing.T) {
				gnmi.Replace(t, dut, config.Config(), baseConfigInterfaceInterfaceRef)
			})
			if !setup.SkipGet() {
				t.Run("Get", func(t *testing.T) {
					configGot := gnmi.Get(t, dut, config.Config())
					if *configGot.Subinterface != input {
						t.Errorf("Config /acl/interfaces/interface/interface-ref/config/subinterface: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe", func(t *testing.T) {
					stateGot := gnmi.Get(t, dut, state.State())
					if *stateGot.Subinterface != input {
						t.Errorf("State /acl/interfaces/interface/interface-ref/config/subinterface: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				if !setup.SkipSubscribe() {
					if qs, _ := gnmi.LookupConfig(t, dut, config.Config()).Val(); qs.Subinterface != nil {
						t.Errorf("Delete /acl/interfaces/interface/interface-ref/config/subinterface fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestInterface(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	baseConfig := setupAcl(t, dut)
	defer teardownAcl(t, dut, baseConfig)

	inputs := []string{
		"FourHundredGigE0/0/0/0",
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /acl/interfaces/interface/interface-ref/config/interface using value %v", input), func(t *testing.T) {
			baseConfigInterface := setup.GetAnyValue(baseConfig.Interface)
			baseConfigInterfaceInterfaceRef := baseConfigInterface.InterfaceRef
			*baseConfigInterfaceInterfaceRef.Interface = input

			config := gnmi.OC().Acl().Interface(*baseConfigInterface.Id).InterfaceRef()
			state := gnmi.OC().Acl().Interface(*baseConfigInterface.Id).InterfaceRef()

			t.Run("Replace", func(t *testing.T) {
				gnmi.Replace(t, dut, config.Config(), baseConfigInterfaceInterfaceRef)
			})
			if !setup.SkipGet() {
				t.Run("Get", func(t *testing.T) {
					configGot := gnmi.Get(t, dut, config.Config())
					if *configGot.Interface != input {
						t.Errorf("Config /acl/interfaces/interface/interface-ref/config/interface: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe", func(t *testing.T) {
					stateGot := gnmi.Get(t, dut, state.State())
					if *stateGot.Interface != input {
						t.Errorf("State /acl/interfaces/interface/interface-ref/config/interface: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				if !setup.SkipSubscribe() {
					if qs, _ := gnmi.LookupConfig(t, dut, config.Config()).Val(); qs.Interface != nil {
						t.Errorf("Delete /acl/interfaces/interface/interface-ref/config/interface fail: got %v", qs)
					}
				}
			})
		})
	}
}
