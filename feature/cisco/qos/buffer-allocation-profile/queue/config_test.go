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
	setup.ResetStruct(bc, []string{"BufferAllocationProfile"})
	bcBufferAllocationProfile := setup.GetAnyValue(bc.BufferAllocationProfile)
	setup.ResetStruct(bcBufferAllocationProfile, []string{"Queue"})
	bcBufferAllocationProfileQueue := setup.GetAnyValue(bcBufferAllocationProfile.Queue)
	setup.ResetStruct(bcBufferAllocationProfileQueue, []string{})
	dut.Config().Qos().Replace(t, bc)
	return bc
}

func teardownQos(t *testing.T, dut *ondatra.DUTDevice, baseConfig *oc.Qos) {
	dut.Config().Qos().Delete(t)
}
func TestSharedBufferLimitTypeAtContainer(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	inputs := []oc.E_Qos_SHARED_BUFFER_LIMIT_TYPE{
		oc.E_Qos_SHARED_BUFFER_LIMIT_TYPE(2), //STATIC
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /qos/buffer-allocation-profiles/buffer-allocation-profile/queues/queue/config/shared-buffer-limit-type using value %v", input), func(t *testing.T) {
			baseConfigBufferAllocationProfile := setup.GetAnyValue(baseConfig.BufferAllocationProfile)
			baseConfigBufferAllocationProfileQueue := setup.GetAnyValue(baseConfigBufferAllocationProfile.Queue)
			baseConfigBufferAllocationProfileQueue.SharedBufferLimitType = input

			config := dut.Config().Qos().BufferAllocationProfile(*baseConfigBufferAllocationProfile.Name).Queue(*baseConfigBufferAllocationProfileQueue.Name)
			state := dut.Telemetry().Qos().BufferAllocationProfile(*baseConfigBufferAllocationProfile.Name).Queue(*baseConfigBufferAllocationProfileQueue.Name)

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigBufferAllocationProfileQueue)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot.SharedBufferLimitType != input {
						t.Errorf("Config /qos/buffer-allocation-profiles/buffer-allocation-profile/queues/queue/config/shared-buffer-limit-type: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot.SharedBufferLimitType != input {
						t.Errorf("State /qos/buffer-allocation-profiles/buffer-allocation-profile/queues/queue/config/shared-buffer-limit-type: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete container", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).SharedBufferLimitType != 0 {
						t.Errorf("Delete /qos/buffer-allocation-profiles/buffer-allocation-profile/queues/queue/config/shared-buffer-limit-type fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestSharedBufferLimitTypeAtLeaf(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	inputs := []oc.E_Qos_SHARED_BUFFER_LIMIT_TYPE{
		oc.E_Qos_SHARED_BUFFER_LIMIT_TYPE(2), //STATIC
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /qos/buffer-allocation-profiles/buffer-allocation-profile/queues/queue/config/shared-buffer-limit-type using value %v", input), func(t *testing.T) {
			baseConfigBufferAllocationProfile := setup.GetAnyValue(baseConfig.BufferAllocationProfile)
			baseConfigBufferAllocationProfileQueue := setup.GetAnyValue(baseConfigBufferAllocationProfile.Queue)

			config := dut.Config().Qos().BufferAllocationProfile(*baseConfigBufferAllocationProfile.Name).Queue(*baseConfigBufferAllocationProfileQueue.Name).SharedBufferLimitType()
			state := dut.Telemetry().Qos().BufferAllocationProfile(*baseConfigBufferAllocationProfile.Name).Queue(*baseConfigBufferAllocationProfileQueue.Name).SharedBufferLimitType()

			t.Run("Replace leaf", func(t *testing.T) {
				config.Replace(t, input)
			})
			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot != input {
						t.Errorf("Config /qos/buffer-allocation-profiles/buffer-allocation-profile/queues/queue/config/shared-buffer-limit-type: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot != input {
						t.Errorf("State /qos/buffer-allocation-profiles/buffer-allocation-profile/queues/queue/config/shared-buffer-limit-type: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete leaf", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs != nil {
						t.Errorf("Delete /qos/buffer-allocation-profiles/buffer-allocation-profile/queues/queue/config/shared-buffer-limit-type fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestStaticSharedBufferLimitAtContainer(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	inputs := []uint32{
		1344201631,
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /qos/buffer-allocation-profiles/buffer-allocation-profile/queues/queue/config/static-shared-buffer-limit using value %v", input), func(t *testing.T) {
			baseConfigBufferAllocationProfile := setup.GetAnyValue(baseConfig.BufferAllocationProfile)
			baseConfigBufferAllocationProfileQueue := setup.GetAnyValue(baseConfigBufferAllocationProfile.Queue)
			*baseConfigBufferAllocationProfileQueue.StaticSharedBufferLimit = input

			config := dut.Config().Qos().BufferAllocationProfile(*baseConfigBufferAllocationProfile.Name).Queue(*baseConfigBufferAllocationProfileQueue.Name)
			state := dut.Telemetry().Qos().BufferAllocationProfile(*baseConfigBufferAllocationProfile.Name).Queue(*baseConfigBufferAllocationProfileQueue.Name)

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigBufferAllocationProfileQueue)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.StaticSharedBufferLimit != input {
						t.Errorf("Config /qos/buffer-allocation-profiles/buffer-allocation-profile/queues/queue/config/static-shared-buffer-limit: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.StaticSharedBufferLimit != input {
						t.Errorf("State /qos/buffer-allocation-profiles/buffer-allocation-profile/queues/queue/config/static-shared-buffer-limit: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete container", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).StaticSharedBufferLimit != nil {
						t.Errorf("Delete /qos/buffer-allocation-profiles/buffer-allocation-profile/queues/queue/config/static-shared-buffer-limit fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestStaticSharedBufferLimitAtLeaf(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	inputs := []uint32{
		1344201631,
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /qos/buffer-allocation-profiles/buffer-allocation-profile/queues/queue/config/static-shared-buffer-limit using value %v", input), func(t *testing.T) {
			baseConfigBufferAllocationProfile := setup.GetAnyValue(baseConfig.BufferAllocationProfile)
			baseConfigBufferAllocationProfileQueue := setup.GetAnyValue(baseConfigBufferAllocationProfile.Queue)

			config := dut.Config().Qos().BufferAllocationProfile(*baseConfigBufferAllocationProfile.Name).Queue(*baseConfigBufferAllocationProfileQueue.Name).StaticSharedBufferLimit()
			state := dut.Telemetry().Qos().BufferAllocationProfile(*baseConfigBufferAllocationProfile.Name).Queue(*baseConfigBufferAllocationProfileQueue.Name).StaticSharedBufferLimit()

			t.Run("Replace leaf", func(t *testing.T) {
				config.Replace(t, input)
			})
			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot != input {
						t.Errorf("Config /qos/buffer-allocation-profiles/buffer-allocation-profile/queues/queue/config/static-shared-buffer-limit: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot != input {
						t.Errorf("State /qos/buffer-allocation-profiles/buffer-allocation-profile/queues/queue/config/static-shared-buffer-limit: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete leaf", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs != nil {
						t.Errorf("Delete /qos/buffer-allocation-profiles/buffer-allocation-profile/queues/queue/config/static-shared-buffer-limit fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestUseSharedBufferAtContainer(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	inputs := []bool{
		true,
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /qos/buffer-allocation-profiles/buffer-allocation-profile/queues/queue/config/use-shared-buffer using value %v", input), func(t *testing.T) {
			baseConfigBufferAllocationProfile := setup.GetAnyValue(baseConfig.BufferAllocationProfile)
			baseConfigBufferAllocationProfileQueue := setup.GetAnyValue(baseConfigBufferAllocationProfile.Queue)
			*baseConfigBufferAllocationProfileQueue.UseSharedBuffer = input

			config := dut.Config().Qos().BufferAllocationProfile(*baseConfigBufferAllocationProfile.Name).Queue(*baseConfigBufferAllocationProfileQueue.Name)
			state := dut.Telemetry().Qos().BufferAllocationProfile(*baseConfigBufferAllocationProfile.Name).Queue(*baseConfigBufferAllocationProfileQueue.Name)

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigBufferAllocationProfileQueue)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.UseSharedBuffer != input {
						t.Errorf("Config /qos/buffer-allocation-profiles/buffer-allocation-profile/queues/queue/config/use-shared-buffer: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.UseSharedBuffer != input {
						t.Errorf("State /qos/buffer-allocation-profiles/buffer-allocation-profile/queues/queue/config/use-shared-buffer: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete container", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).UseSharedBuffer != nil {
						t.Errorf("Delete /qos/buffer-allocation-profiles/buffer-allocation-profile/queues/queue/config/use-shared-buffer fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestUseSharedBufferAtLeaf(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	inputs := []bool{
		true,
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /qos/buffer-allocation-profiles/buffer-allocation-profile/queues/queue/config/use-shared-buffer using value %v", input), func(t *testing.T) {
			baseConfigBufferAllocationProfile := setup.GetAnyValue(baseConfig.BufferAllocationProfile)
			baseConfigBufferAllocationProfileQueue := setup.GetAnyValue(baseConfigBufferAllocationProfile.Queue)

			config := dut.Config().Qos().BufferAllocationProfile(*baseConfigBufferAllocationProfile.Name).Queue(*baseConfigBufferAllocationProfileQueue.Name).UseSharedBuffer()
			state := dut.Telemetry().Qos().BufferAllocationProfile(*baseConfigBufferAllocationProfile.Name).Queue(*baseConfigBufferAllocationProfileQueue.Name).UseSharedBuffer()

			t.Run("Replace leaf", func(t *testing.T) {
				config.Replace(t, input)
			})
			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot != input {
						t.Errorf("Config /qos/buffer-allocation-profiles/buffer-allocation-profile/queues/queue/config/use-shared-buffer: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot != input {
						t.Errorf("State /qos/buffer-allocation-profiles/buffer-allocation-profile/queues/queue/config/use-shared-buffer: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete leaf", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs != nil {
						t.Errorf("Delete /qos/buffer-allocation-profiles/buffer-allocation-profile/queues/queue/config/use-shared-buffer fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestNameAtContainer(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	inputs := []string{
		"ai",
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /qos/buffer-allocation-profiles/buffer-allocation-profile/queues/queue/config/name using value %v", input), func(t *testing.T) {
			baseConfigBufferAllocationProfile := setup.GetAnyValue(baseConfig.BufferAllocationProfile)
			baseConfigBufferAllocationProfileQueue := setup.GetAnyValue(baseConfigBufferAllocationProfile.Queue)
			*baseConfigBufferAllocationProfileQueue.Name = input

			config := dut.Config().Qos().BufferAllocationProfile(*baseConfigBufferAllocationProfile.Name).Queue(*baseConfigBufferAllocationProfileQueue.Name)
			state := dut.Telemetry().Qos().BufferAllocationProfile(*baseConfigBufferAllocationProfile.Name).Queue(*baseConfigBufferAllocationProfileQueue.Name)

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigBufferAllocationProfileQueue)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.Name != input {
						t.Errorf("Config /qos/buffer-allocation-profiles/buffer-allocation-profile/queues/queue/config/name: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.Name != input {
						t.Errorf("State /qos/buffer-allocation-profiles/buffer-allocation-profile/queues/queue/config/name: got %v, want %v", stateGot, input)
					}
				})
			}
		})
	}
}
func TestDynamicLimitScalingFactorAtContainer(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	inputs := []int32{
		-1214592983,
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /qos/buffer-allocation-profiles/buffer-allocation-profile/queues/queue/config/dynamic-limit-scaling-factor using value %v", input), func(t *testing.T) {
			baseConfigBufferAllocationProfile := setup.GetAnyValue(baseConfig.BufferAllocationProfile)
			baseConfigBufferAllocationProfileQueue := setup.GetAnyValue(baseConfigBufferAllocationProfile.Queue)
			*baseConfigBufferAllocationProfileQueue.DynamicLimitScalingFactor = input

			config := dut.Config().Qos().BufferAllocationProfile(*baseConfigBufferAllocationProfile.Name).Queue(*baseConfigBufferAllocationProfileQueue.Name)
			state := dut.Telemetry().Qos().BufferAllocationProfile(*baseConfigBufferAllocationProfile.Name).Queue(*baseConfigBufferAllocationProfileQueue.Name)

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigBufferAllocationProfileQueue)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.DynamicLimitScalingFactor != input {
						t.Errorf("Config /qos/buffer-allocation-profiles/buffer-allocation-profile/queues/queue/config/dynamic-limit-scaling-factor: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.DynamicLimitScalingFactor != input {
						t.Errorf("State /qos/buffer-allocation-profiles/buffer-allocation-profile/queues/queue/config/dynamic-limit-scaling-factor: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete container", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).DynamicLimitScalingFactor != nil {
						t.Errorf("Delete /qos/buffer-allocation-profiles/buffer-allocation-profile/queues/queue/config/dynamic-limit-scaling-factor fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestDynamicLimitScalingFactorAtLeaf(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	inputs := []int32{
		-1214592983,
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /qos/buffer-allocation-profiles/buffer-allocation-profile/queues/queue/config/dynamic-limit-scaling-factor using value %v", input), func(t *testing.T) {
			baseConfigBufferAllocationProfile := setup.GetAnyValue(baseConfig.BufferAllocationProfile)
			baseConfigBufferAllocationProfileQueue := setup.GetAnyValue(baseConfigBufferAllocationProfile.Queue)

			config := dut.Config().Qos().BufferAllocationProfile(*baseConfigBufferAllocationProfile.Name).Queue(*baseConfigBufferAllocationProfileQueue.Name).DynamicLimitScalingFactor()
			state := dut.Telemetry().Qos().BufferAllocationProfile(*baseConfigBufferAllocationProfile.Name).Queue(*baseConfigBufferAllocationProfileQueue.Name).DynamicLimitScalingFactor()

			t.Run("Replace leaf", func(t *testing.T) {
				config.Replace(t, input)
			})
			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot != input {
						t.Errorf("Config /qos/buffer-allocation-profiles/buffer-allocation-profile/queues/queue/config/dynamic-limit-scaling-factor: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot != input {
						t.Errorf("State /qos/buffer-allocation-profiles/buffer-allocation-profile/queues/queue/config/dynamic-limit-scaling-factor: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete leaf", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs != nil {
						t.Errorf("Delete /qos/buffer-allocation-profiles/buffer-allocation-profile/queues/queue/config/dynamic-limit-scaling-factor fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestDedicatedBufferAtContainer(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	inputs := []uint64{
		7658831277319908658,
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /qos/buffer-allocation-profiles/buffer-allocation-profile/queues/queue/config/dedicated-buffer using value %v", input), func(t *testing.T) {
			baseConfigBufferAllocationProfile := setup.GetAnyValue(baseConfig.BufferAllocationProfile)
			baseConfigBufferAllocationProfileQueue := setup.GetAnyValue(baseConfigBufferAllocationProfile.Queue)
			*baseConfigBufferAllocationProfileQueue.DedicatedBuffer = input

			config := dut.Config().Qos().BufferAllocationProfile(*baseConfigBufferAllocationProfile.Name).Queue(*baseConfigBufferAllocationProfileQueue.Name)
			state := dut.Telemetry().Qos().BufferAllocationProfile(*baseConfigBufferAllocationProfile.Name).Queue(*baseConfigBufferAllocationProfileQueue.Name)

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigBufferAllocationProfileQueue)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.DedicatedBuffer != input {
						t.Errorf("Config /qos/buffer-allocation-profiles/buffer-allocation-profile/queues/queue/config/dedicated-buffer: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.DedicatedBuffer != input {
						t.Errorf("State /qos/buffer-allocation-profiles/buffer-allocation-profile/queues/queue/config/dedicated-buffer: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete container", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).DedicatedBuffer != nil {
						t.Errorf("Delete /qos/buffer-allocation-profiles/buffer-allocation-profile/queues/queue/config/dedicated-buffer fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestDedicatedBufferAtLeaf(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	inputs := []uint64{
		7658831277319908658,
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /qos/buffer-allocation-profiles/buffer-allocation-profile/queues/queue/config/dedicated-buffer using value %v", input), func(t *testing.T) {
			baseConfigBufferAllocationProfile := setup.GetAnyValue(baseConfig.BufferAllocationProfile)
			baseConfigBufferAllocationProfileQueue := setup.GetAnyValue(baseConfigBufferAllocationProfile.Queue)

			config := dut.Config().Qos().BufferAllocationProfile(*baseConfigBufferAllocationProfile.Name).Queue(*baseConfigBufferAllocationProfileQueue.Name).DedicatedBuffer()
			state := dut.Telemetry().Qos().BufferAllocationProfile(*baseConfigBufferAllocationProfile.Name).Queue(*baseConfigBufferAllocationProfileQueue.Name).DedicatedBuffer()

			t.Run("Replace leaf", func(t *testing.T) {
				config.Replace(t, input)
			})
			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot != input {
						t.Errorf("Config /qos/buffer-allocation-profiles/buffer-allocation-profile/queues/queue/config/dedicated-buffer: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot != input {
						t.Errorf("State /qos/buffer-allocation-profiles/buffer-allocation-profile/queues/queue/config/dedicated-buffer: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete leaf", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs != nil {
						t.Errorf("Delete /qos/buffer-allocation-profiles/buffer-allocation-profile/queues/queue/config/dedicated-buffer fail: got %v", qs)
					}
				}
			})
		})
	}
}
