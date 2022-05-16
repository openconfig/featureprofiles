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
	*bc = setup.BaseConfig
	setup.ResetStruct(bc, []string{"Interface", "AclSet"})
	bcInterface := setup.GetAnyValue(bc.Interface)
	setup.ResetStruct(bcInterface, []string{"InterfaceRef"})
	bcInterfaceInterfaceRef := bcInterface.InterfaceRef
	setup.ResetStruct(bcInterfaceInterfaceRef, []string{})
	dut.Config().Acl().Replace(t, bc)
	return bc
}

func teardownAcl(t *testing.T, dut *ondatra.DUTDevice, baseConfig *oc.Acl) {
	dut.Config().Acl().Delete(t)
}
func TestSubinterface(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	baseConfig := setupAcl(t, dut)
	defer teardownAcl(t, dut, baseConfig)

	inputs := []uint32{
		1854090381,
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /acl/interfaces/interface/interface-ref/config/subinterface using value %v", input), func(t *testing.T) {
			baseConfigInterface := setup.GetAnyValue(baseConfig.Interface)
			baseConfigInterfaceInterfaceRef := baseConfigInterface.InterfaceRef
			*baseConfigInterfaceInterfaceRef.Subinterface = input

			config := dut.Config().Acl().Interface(*baseConfigInterface.Id).InterfaceRef()
			state := dut.Telemetry().Acl().Interface(*baseConfigInterface.Id).InterfaceRef()

			t.Run("Replace", func(t *testing.T) {
				config.Replace(t, baseConfigInterfaceInterfaceRef)
			})
			if !setup.SkipGet() {
				t.Run("Get", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.Subinterface != input {
						t.Errorf("Config /acl/interfaces/interface/interface-ref/config/subinterface: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.Subinterface != input {
						t.Errorf("State /acl/interfaces/interface/interface-ref/config/subinterface: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).Subinterface != nil {
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
		"s",
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /acl/interfaces/interface/interface-ref/config/interface using value %v", input), func(t *testing.T) {
			baseConfigInterface := setup.GetAnyValue(baseConfig.Interface)
			baseConfigInterfaceInterfaceRef := baseConfigInterface.InterfaceRef
			*baseConfigInterfaceInterfaceRef.Interface = input

			config := dut.Config().Acl().Interface(*baseConfigInterface.Id).InterfaceRef()
			state := dut.Telemetry().Acl().Interface(*baseConfigInterface.Id).InterfaceRef()

			t.Run("Replace", func(t *testing.T) {
				config.Replace(t, baseConfigInterfaceInterfaceRef)
			})
			if !setup.SkipGet() {
				t.Run("Get", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.Interface != input {
						t.Errorf("Config /acl/interfaces/interface/interface-ref/config/interface: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.Interface != input {
						t.Errorf("State /acl/interfaces/interface/interface-ref/config/interface: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).Interface != nil {
						t.Errorf("Delete /acl/interfaces/interface/interface-ref/config/interface fail: got %v", qs)
					}
				}
			})
		})
	}
}
