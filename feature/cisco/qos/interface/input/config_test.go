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
	setup.ResetStruct(bcInterface, []string{"Input"})
	bcInterfaceInput := bcInterface.Input
	setup.ResetStruct(bcInterfaceInput, []string{})
	dut.Config().Qos().Replace(t, bc)
	return bc
}

func teardownQos(t *testing.T, dut *ondatra.DUTDevice, baseConfig *oc.Qos) {
	dut.Config().Qos().Delete(t)
}
func TestBufferAllocationProfileAtContainer(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	inputs := []string{
		":",
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /qos/interfaces/interface/input/config/buffer-allocation-profile using value %v", input), func(t *testing.T) {
			baseConfigInterface := setup.GetAnyValue(baseConfig.Interface)
			baseConfigInterfaceInput := baseConfigInterface.Input
			*baseConfigInterfaceInput.BufferAllocationProfile = input

			config := dut.Config().Qos().Interface(*baseConfigInterface.InterfaceId).Input()
			state := dut.Telemetry().Qos().Interface(*baseConfigInterface.InterfaceId).Input()

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigInterfaceInput)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.BufferAllocationProfile != input {
						t.Errorf("Config /qos/interfaces/interface/input/config/buffer-allocation-profile: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.BufferAllocationProfile != input {
						t.Errorf("State /qos/interfaces/interface/input/config/buffer-allocation-profile: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete container", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).BufferAllocationProfile != nil {
						t.Errorf("Delete /qos/interfaces/interface/input/config/buffer-allocation-profile fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestBufferAllocationProfileAtLeaf(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	inputs := []string{
		":",
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /qos/interfaces/interface/input/config/buffer-allocation-profile using value %v", input), func(t *testing.T) {
			baseConfigInterface := setup.GetAnyValue(baseConfig.Interface)

			config := dut.Config().Qos().Interface(*baseConfigInterface.InterfaceId).Input().BufferAllocationProfile()
			state := dut.Telemetry().Qos().Interface(*baseConfigInterface.InterfaceId).Input().BufferAllocationProfile()

			t.Run("Replace leaf", func(t *testing.T) {
				config.Replace(t, input)
			})
			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot != input {
						t.Errorf("Config /qos/interfaces/interface/input/config/buffer-allocation-profile: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot != input {
						t.Errorf("State /qos/interfaces/interface/input/config/buffer-allocation-profile: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete leaf", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs != nil {
						t.Errorf("Delete /qos/interfaces/interface/input/config/buffer-allocation-profile fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestMulticastBufferAllocationProfileAtContainer(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	inputs := []string{
		"c",
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /qos/interfaces/interface/input/config/multicast-buffer-allocation-profile using value %v", input), func(t *testing.T) {
			baseConfigInterface := setup.GetAnyValue(baseConfig.Interface)
			baseConfigInterfaceInput := baseConfigInterface.Input
			*baseConfigInterfaceInput.MulticastBufferAllocationProfile = input

			config := dut.Config().Qos().Interface(*baseConfigInterface.InterfaceId).Input()
			state := dut.Telemetry().Qos().Interface(*baseConfigInterface.InterfaceId).Input()

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigInterfaceInput)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.MulticastBufferAllocationProfile != input {
						t.Errorf("Config /qos/interfaces/interface/input/config/multicast-buffer-allocation-profile: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.MulticastBufferAllocationProfile != input {
						t.Errorf("State /qos/interfaces/interface/input/config/multicast-buffer-allocation-profile: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete container", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).MulticastBufferAllocationProfile != nil {
						t.Errorf("Delete /qos/interfaces/interface/input/config/multicast-buffer-allocation-profile fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestMulticastBufferAllocationProfileAtLeaf(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	inputs := []string{
		"c",
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /qos/interfaces/interface/input/config/multicast-buffer-allocation-profile using value %v", input), func(t *testing.T) {
			baseConfigInterface := setup.GetAnyValue(baseConfig.Interface)

			config := dut.Config().Qos().Interface(*baseConfigInterface.InterfaceId).Input().MulticastBufferAllocationProfile()
			state := dut.Telemetry().Qos().Interface(*baseConfigInterface.InterfaceId).Input().MulticastBufferAllocationProfile()

			t.Run("Replace leaf", func(t *testing.T) {
				config.Replace(t, input)
			})
			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot != input {
						t.Errorf("Config /qos/interfaces/interface/input/config/multicast-buffer-allocation-profile: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot != input {
						t.Errorf("State /qos/interfaces/interface/input/config/multicast-buffer-allocation-profile: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete leaf", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs != nil {
						t.Errorf("Delete /qos/interfaces/interface/input/config/multicast-buffer-allocation-profile fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestUnicastBufferAllocationProfileAtContainer(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	inputs := []string{
		"s",
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /qos/interfaces/interface/input/config/unicast-buffer-allocation-profile using value %v", input), func(t *testing.T) {
			baseConfigInterface := setup.GetAnyValue(baseConfig.Interface)
			baseConfigInterfaceInput := baseConfigInterface.Input
			*baseConfigInterfaceInput.UnicastBufferAllocationProfile = input

			config := dut.Config().Qos().Interface(*baseConfigInterface.InterfaceId).Input()
			state := dut.Telemetry().Qos().Interface(*baseConfigInterface.InterfaceId).Input()

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigInterfaceInput)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.UnicastBufferAllocationProfile != input {
						t.Errorf("Config /qos/interfaces/interface/input/config/unicast-buffer-allocation-profile: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.UnicastBufferAllocationProfile != input {
						t.Errorf("State /qos/interfaces/interface/input/config/unicast-buffer-allocation-profile: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete container", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).UnicastBufferAllocationProfile != nil {
						t.Errorf("Delete /qos/interfaces/interface/input/config/unicast-buffer-allocation-profile fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestUnicastBufferAllocationProfileAtLeaf(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	inputs := []string{
		"s",
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /qos/interfaces/interface/input/config/unicast-buffer-allocation-profile using value %v", input), func(t *testing.T) {
			baseConfigInterface := setup.GetAnyValue(baseConfig.Interface)

			config := dut.Config().Qos().Interface(*baseConfigInterface.InterfaceId).Input().UnicastBufferAllocationProfile()
			state := dut.Telemetry().Qos().Interface(*baseConfigInterface.InterfaceId).Input().UnicastBufferAllocationProfile()

			t.Run("Replace leaf", func(t *testing.T) {
				config.Replace(t, input)
			})
			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot != input {
						t.Errorf("Config /qos/interfaces/interface/input/config/unicast-buffer-allocation-profile: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot != input {
						t.Errorf("State /qos/interfaces/interface/input/config/unicast-buffer-allocation-profile: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete leaf", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs != nil {
						t.Errorf("Delete /qos/interfaces/interface/input/config/unicast-buffer-allocation-profile fail: got %v", qs)
					}
				}
			})
		})
	}
}
