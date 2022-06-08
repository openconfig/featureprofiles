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
	setup.ResetStruct(bcInterfaceOutput, []string{})
	dut.Config().Qos().Replace(t, bc)
	return bc
}

func teardownQos(t *testing.T, dut *ondatra.DUTDevice, baseConfig *oc.Qos) {
	dut.Config().Qos().Delete(t)
}
func TestMulticastBufferAllocationProfileAtContainer(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	inputs := []string{
		"i",
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /qos/interfaces/interface/output/config/multicast-buffer-allocation-profile using value %v", input), func(t *testing.T) {
			baseConfigInterface := setup.GetAnyValue(baseConfig.Interface)
			baseConfigInterfaceOutput := baseConfigInterface.Output
			*baseConfigInterfaceOutput.MulticastBufferAllocationProfile = input

			config := dut.Config().Qos().Interface(*baseConfigInterface.InterfaceId).Output()
			state := dut.Telemetry().Qos().Interface(*baseConfigInterface.InterfaceId).Output()

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigInterfaceOutput)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.MulticastBufferAllocationProfile != input {
						t.Errorf("Config /qos/interfaces/interface/output/config/multicast-buffer-allocation-profile: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.MulticastBufferAllocationProfile != input {
						t.Errorf("State /qos/interfaces/interface/output/config/multicast-buffer-allocation-profile: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete container", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).MulticastBufferAllocationProfile != nil {
						t.Errorf("Delete /qos/interfaces/interface/output/config/multicast-buffer-allocation-profile fail: got %v", qs)
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
		"i",
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /qos/interfaces/interface/output/config/multicast-buffer-allocation-profile using value %v", input), func(t *testing.T) {
			baseConfigInterface := setup.GetAnyValue(baseConfig.Interface)

			config := dut.Config().Qos().Interface(*baseConfigInterface.InterfaceId).Output().MulticastBufferAllocationProfile()
			state := dut.Telemetry().Qos().Interface(*baseConfigInterface.InterfaceId).Output().MulticastBufferAllocationProfile()

			t.Run("Replace leaf", func(t *testing.T) {
				config.Replace(t, input)
			})
			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot != input {
						t.Errorf("Config /qos/interfaces/interface/output/config/multicast-buffer-allocation-profile: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot != input {
						t.Errorf("State /qos/interfaces/interface/output/config/multicast-buffer-allocation-profile: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete leaf", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs != nil {
						t.Errorf("Delete /qos/interfaces/interface/output/config/multicast-buffer-allocation-profile fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestBufferAllocationProfileAtContainer(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	inputs := []string{
		":iss:",
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /qos/interfaces/interface/output/config/buffer-allocation-profile using value %v", input), func(t *testing.T) {
			baseConfigInterface := setup.GetAnyValue(baseConfig.Interface)
			baseConfigInterfaceOutput := baseConfigInterface.Output
			*baseConfigInterfaceOutput.BufferAllocationProfile = input

			config := dut.Config().Qos().Interface(*baseConfigInterface.InterfaceId).Output()
			state := dut.Telemetry().Qos().Interface(*baseConfigInterface.InterfaceId).Output()

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigInterfaceOutput)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.BufferAllocationProfile != input {
						t.Errorf("Config /qos/interfaces/interface/output/config/buffer-allocation-profile: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.BufferAllocationProfile != input {
						t.Errorf("State /qos/interfaces/interface/output/config/buffer-allocation-profile: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete container", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).BufferAllocationProfile != nil {
						t.Errorf("Delete /qos/interfaces/interface/output/config/buffer-allocation-profile fail: got %v", qs)
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
		":iss:",
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /qos/interfaces/interface/output/config/buffer-allocation-profile using value %v", input), func(t *testing.T) {
			baseConfigInterface := setup.GetAnyValue(baseConfig.Interface)

			config := dut.Config().Qos().Interface(*baseConfigInterface.InterfaceId).Output().BufferAllocationProfile()
			state := dut.Telemetry().Qos().Interface(*baseConfigInterface.InterfaceId).Output().BufferAllocationProfile()

			t.Run("Replace leaf", func(t *testing.T) {
				config.Replace(t, input)
			})
			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot != input {
						t.Errorf("Config /qos/interfaces/interface/output/config/buffer-allocation-profile: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot != input {
						t.Errorf("State /qos/interfaces/interface/output/config/buffer-allocation-profile: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete leaf", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs != nil {
						t.Errorf("Delete /qos/interfaces/interface/output/config/buffer-allocation-profile fail: got %v", qs)
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
		"::",
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /qos/interfaces/interface/output/config/unicast-buffer-allocation-profile using value %v", input), func(t *testing.T) {
			baseConfigInterface := setup.GetAnyValue(baseConfig.Interface)
			baseConfigInterfaceOutput := baseConfigInterface.Output
			*baseConfigInterfaceOutput.UnicastBufferAllocationProfile = input

			config := dut.Config().Qos().Interface(*baseConfigInterface.InterfaceId).Output()
			state := dut.Telemetry().Qos().Interface(*baseConfigInterface.InterfaceId).Output()

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigInterfaceOutput)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.UnicastBufferAllocationProfile != input {
						t.Errorf("Config /qos/interfaces/interface/output/config/unicast-buffer-allocation-profile: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.UnicastBufferAllocationProfile != input {
						t.Errorf("State /qos/interfaces/interface/output/config/unicast-buffer-allocation-profile: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete container", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).UnicastBufferAllocationProfile != nil {
						t.Errorf("Delete /qos/interfaces/interface/output/config/unicast-buffer-allocation-profile fail: got %v", qs)
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
		"::",
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /qos/interfaces/interface/output/config/unicast-buffer-allocation-profile using value %v", input), func(t *testing.T) {
			baseConfigInterface := setup.GetAnyValue(baseConfig.Interface)

			config := dut.Config().Qos().Interface(*baseConfigInterface.InterfaceId).Output().UnicastBufferAllocationProfile()
			state := dut.Telemetry().Qos().Interface(*baseConfigInterface.InterfaceId).Output().UnicastBufferAllocationProfile()

			t.Run("Replace leaf", func(t *testing.T) {
				config.Replace(t, input)
			})
			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot != input {
						t.Errorf("Config /qos/interfaces/interface/output/config/unicast-buffer-allocation-profile: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot != input {
						t.Errorf("State /qos/interfaces/interface/output/config/unicast-buffer-allocation-profile: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete leaf", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs != nil {
						t.Errorf("Delete /qos/interfaces/interface/output/config/unicast-buffer-allocation-profile fail: got %v", qs)
					}
				}
			})
		})
	}
}
