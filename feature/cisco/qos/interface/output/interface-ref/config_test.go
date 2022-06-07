package qos_test

import (
	"fmt"
	"testing"

	"github.com/openconfig/featureprofiles/feature/cisco/qos/setup"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	oc "github.com/openconfig/ondatra/telemetry"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func setupQos(t *testing.T, dut *ondatra.DUTDevice) *oc.Qos {
	bc := setup.BaseConfig()
	setup.ResetStruct(bc, []string{"Interface"})
	bcInterface := setup.GetAnyValue(bc.Interface)
	setup.ResetStruct(bcInterface, []string{"Output"})
	bcInterfaceOutput := bcInterface.Output
	setup.ResetStruct(bcInterfaceOutput, []string{"InterfaceRef"})
	bcInterfaceOutputInterfaceRef := bcInterfaceOutput.InterfaceRef
	setup.ResetStruct(bcInterfaceOutputInterfaceRef, []string{})
	dut.Config().Qos().Replace(t, bc)
	return bc
}

func teardownQos(t *testing.T, dut *ondatra.DUTDevice, baseConfig *oc.Qos) {
	dut.Config().Qos().Delete(t)
}
func TestInterfaceAtContainer(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	inputs := []string{
		"assa",
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /qos/interfaces/interface/output/interface-ref/config/interface using value %v", input), func(t *testing.T) {
			baseConfigInterface := setup.GetAnyValue(baseConfig.Interface)
			baseConfigInterfaceOutput := baseConfigInterface.Output
			baseConfigInterfaceOutputInterfaceRef := baseConfigInterfaceOutput.InterfaceRef
			*baseConfigInterfaceOutputInterfaceRef.Interface = input

			config := dut.Config().Qos().Interface(*baseConfigInterface.InterfaceId).Output().InterfaceRef()
			state := dut.Telemetry().Qos().Interface(*baseConfigInterface.InterfaceId).Output().InterfaceRef()

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigInterfaceOutputInterfaceRef)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.Interface != input {
						t.Errorf("Config /qos/interfaces/interface/output/interface-ref/config/interface: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.Interface != input {
						t.Errorf("State /qos/interfaces/interface/output/interface-ref/config/interface: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete container", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).Interface != nil {
						t.Errorf("Delete /qos/interfaces/interface/output/interface-ref/config/interface fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestInterfaceAtLeaf(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	inputs := []string{
		"assa",
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /qos/interfaces/interface/output/interface-ref/config/interface using value %v", input), func(t *testing.T) {
			baseConfigInterface := setup.GetAnyValue(baseConfig.Interface)

			config := dut.Config().Qos().Interface(*baseConfigInterface.InterfaceId).Output().InterfaceRef().Interface()
			state := dut.Telemetry().Qos().Interface(*baseConfigInterface.InterfaceId).Output().InterfaceRef().Interface()

			t.Run("Replace leaf", func(t *testing.T) {
				config.Replace(t, input)
			})
			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot != input {
						t.Errorf("Config /qos/interfaces/interface/output/interface-ref/config/interface: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot != input {
						t.Errorf("State /qos/interfaces/interface/output/interface-ref/config/interface: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete leaf", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs != nil {
						t.Errorf("Delete /qos/interfaces/interface/output/interface-ref/config/interface fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestSubinterfaceAtContainer(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	inputs := []uint32{
		2395897030,
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /qos/interfaces/interface/output/interface-ref/config/subinterface using value %v", input), func(t *testing.T) {
			baseConfigInterface := setup.GetAnyValue(baseConfig.Interface)
			baseConfigInterfaceOutput := baseConfigInterface.Output
			baseConfigInterfaceOutputInterfaceRef := baseConfigInterfaceOutput.InterfaceRef
			*baseConfigInterfaceOutputInterfaceRef.Subinterface = input

			config := dut.Config().Qos().Interface(*baseConfigInterface.InterfaceId).Output().InterfaceRef()
			state := dut.Telemetry().Qos().Interface(*baseConfigInterface.InterfaceId).Output().InterfaceRef()

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigInterfaceOutputInterfaceRef)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.Subinterface != input {
						t.Errorf("Config /qos/interfaces/interface/output/interface-ref/config/subinterface: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.Subinterface != input {
						t.Errorf("State /qos/interfaces/interface/output/interface-ref/config/subinterface: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete container", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).Subinterface != nil {
						t.Errorf("Delete /qos/interfaces/interface/output/interface-ref/config/subinterface fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestSubinterfaceAtLeaf(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	inputs := []uint32{
		2395897030,
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /qos/interfaces/interface/output/interface-ref/config/subinterface using value %v", input), func(t *testing.T) {
			baseConfigInterface := setup.GetAnyValue(baseConfig.Interface)

			config := dut.Config().Qos().Interface(*baseConfigInterface.InterfaceId).Output().InterfaceRef().Subinterface()
			state := dut.Telemetry().Qos().Interface(*baseConfigInterface.InterfaceId).Output().InterfaceRef().Subinterface()

			t.Run("Replace leaf", func(t *testing.T) {
				config.Replace(t, input)
			})
			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot != input {
						t.Errorf("Config /qos/interfaces/interface/output/interface-ref/config/subinterface: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot != input {
						t.Errorf("State /qos/interfaces/interface/output/interface-ref/config/subinterface: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete leaf", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs != nil {
						t.Errorf("Delete /qos/interfaces/interface/output/interface-ref/config/subinterface fail: got %v", qs)
					}
				}
			})
		})
	}
}
