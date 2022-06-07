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
	setup.ResetStruct(bc, []string{"QueueManagementProfile"})
	bcQueueManagementProfile := setup.GetAnyValue(bc.QueueManagementProfile)
	setup.ResetStruct(bcQueueManagementProfile, []string{"Wred"})
	bcQueueManagementProfileWred := bcQueueManagementProfile.Wred
	setup.ResetStruct(bcQueueManagementProfileWred, []string{"Uniform"})
	bcQueueManagementProfileWredUniform := bcQueueManagementProfileWred.Uniform
	setup.ResetStruct(bcQueueManagementProfileWredUniform, []string{})
	dut.Config().Qos().Replace(t, bc)
	return bc
}

func teardownQos(t *testing.T, dut *ondatra.DUTDevice, baseConfig *oc.Qos) {
	dut.Config().Qos().Delete(t)
}
func TestWeightAtContainer(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	inputs := []uint32{
		3905483675,
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/weight using value %v", input), func(t *testing.T) {
			baseConfigQueueManagementProfile := setup.GetAnyValue(baseConfig.QueueManagementProfile)
			baseConfigQueueManagementProfileWred := baseConfigQueueManagementProfile.Wred
			baseConfigQueueManagementProfileWredUniform := baseConfigQueueManagementProfileWred.Uniform
			*baseConfigQueueManagementProfileWredUniform.Weight = input

			config := dut.Config().Qos().QueueManagementProfile(*baseConfigQueueManagementProfile.Name).Wred().Uniform()
			state := dut.Telemetry().Qos().QueueManagementProfile(*baseConfigQueueManagementProfile.Name).Wred().Uniform()

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigQueueManagementProfileWredUniform)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.Weight != input {
						t.Errorf("Config /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/weight: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.Weight != input {
						t.Errorf("State /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/weight: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete container", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).Weight != nil {
						t.Errorf("Delete /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/weight fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestWeightAtLeaf(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	inputs := []uint32{
		3905483675,
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/weight using value %v", input), func(t *testing.T) {
			baseConfigQueueManagementProfile := setup.GetAnyValue(baseConfig.QueueManagementProfile)

			config := dut.Config().Qos().QueueManagementProfile(*baseConfigQueueManagementProfile.Name).Wred().Uniform().Weight()
			state := dut.Telemetry().Qos().QueueManagementProfile(*baseConfigQueueManagementProfile.Name).Wred().Uniform().Weight()

			t.Run("Replace leaf", func(t *testing.T) {
				config.Replace(t, input)
			})
			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot != input {
						t.Errorf("Config /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/weight: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot != input {
						t.Errorf("State /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/weight: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete leaf", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs != nil {
						t.Errorf("Delete /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/weight fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestEnableEcnAtContainer(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	inputs := []bool{
		true,
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/enable-ecn using value %v", input), func(t *testing.T) {
			baseConfigQueueManagementProfile := setup.GetAnyValue(baseConfig.QueueManagementProfile)
			baseConfigQueueManagementProfileWred := baseConfigQueueManagementProfile.Wred
			baseConfigQueueManagementProfileWredUniform := baseConfigQueueManagementProfileWred.Uniform
			*baseConfigQueueManagementProfileWredUniform.EnableEcn = input

			config := dut.Config().Qos().QueueManagementProfile(*baseConfigQueueManagementProfile.Name).Wred().Uniform()
			state := dut.Telemetry().Qos().QueueManagementProfile(*baseConfigQueueManagementProfile.Name).Wred().Uniform()

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigQueueManagementProfileWredUniform)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.EnableEcn != input {
						t.Errorf("Config /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/enable-ecn: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.EnableEcn != input {
						t.Errorf("State /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/enable-ecn: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete container", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).EnableEcn != nil {
						t.Errorf("Delete /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/enable-ecn fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestEnableEcnAtLeaf(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	inputs := []bool{
		true,
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/enable-ecn using value %v", input), func(t *testing.T) {
			baseConfigQueueManagementProfile := setup.GetAnyValue(baseConfig.QueueManagementProfile)

			config := dut.Config().Qos().QueueManagementProfile(*baseConfigQueueManagementProfile.Name).Wred().Uniform().EnableEcn()
			state := dut.Telemetry().Qos().QueueManagementProfile(*baseConfigQueueManagementProfile.Name).Wred().Uniform().EnableEcn()

			t.Run("Replace leaf", func(t *testing.T) {
				config.Replace(t, input)
			})
			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot != input {
						t.Errorf("Config /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/enable-ecn: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot != input {
						t.Errorf("State /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/enable-ecn: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete leaf", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs != nil {
						t.Errorf("Delete /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/enable-ecn fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestMinThresholdAtContainer(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	inputs := []uint64{
		15517279399695938449,
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/min-threshold using value %v", input), func(t *testing.T) {
			baseConfigQueueManagementProfile := setup.GetAnyValue(baseConfig.QueueManagementProfile)
			baseConfigQueueManagementProfileWred := baseConfigQueueManagementProfile.Wred
			baseConfigQueueManagementProfileWredUniform := baseConfigQueueManagementProfileWred.Uniform
			*baseConfigQueueManagementProfileWredUniform.MinThreshold = input

			config := dut.Config().Qos().QueueManagementProfile(*baseConfigQueueManagementProfile.Name).Wred().Uniform()
			state := dut.Telemetry().Qos().QueueManagementProfile(*baseConfigQueueManagementProfile.Name).Wred().Uniform()

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigQueueManagementProfileWredUniform)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.MinThreshold != input {
						t.Errorf("Config /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/min-threshold: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.MinThreshold != input {
						t.Errorf("State /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/min-threshold: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete container", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).MinThreshold != nil {
						t.Errorf("Delete /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/min-threshold fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestMinThresholdAtLeaf(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	inputs := []uint64{
		15517279399695938449,
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/min-threshold using value %v", input), func(t *testing.T) {
			baseConfigQueueManagementProfile := setup.GetAnyValue(baseConfig.QueueManagementProfile)

			config := dut.Config().Qos().QueueManagementProfile(*baseConfigQueueManagementProfile.Name).Wred().Uniform().MinThreshold()
			state := dut.Telemetry().Qos().QueueManagementProfile(*baseConfigQueueManagementProfile.Name).Wred().Uniform().MinThreshold()

			t.Run("Replace leaf", func(t *testing.T) {
				config.Replace(t, input)
			})
			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot != input {
						t.Errorf("Config /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/min-threshold: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot != input {
						t.Errorf("State /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/min-threshold: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete leaf", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs != nil {
						t.Errorf("Delete /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/min-threshold fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestMaxDropProbabilityPercentAtContainer(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	inputs := []uint8{
		1,
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/max-drop-probability-percent using value %v", input), func(t *testing.T) {
			baseConfigQueueManagementProfile := setup.GetAnyValue(baseConfig.QueueManagementProfile)
			baseConfigQueueManagementProfileWred := baseConfigQueueManagementProfile.Wred
			baseConfigQueueManagementProfileWredUniform := baseConfigQueueManagementProfileWred.Uniform
			*baseConfigQueueManagementProfileWredUniform.MaxDropProbabilityPercent = input

			config := dut.Config().Qos().QueueManagementProfile(*baseConfigQueueManagementProfile.Name).Wred().Uniform()
			state := dut.Telemetry().Qos().QueueManagementProfile(*baseConfigQueueManagementProfile.Name).Wred().Uniform()

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigQueueManagementProfileWredUniform)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.MaxDropProbabilityPercent != input {
						t.Errorf("Config /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/max-drop-probability-percent: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.MaxDropProbabilityPercent != input {
						t.Errorf("State /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/max-drop-probability-percent: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete container", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).MaxDropProbabilityPercent != nil {
						t.Errorf("Delete /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/max-drop-probability-percent fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestMaxDropProbabilityPercentAtLeaf(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	inputs := []uint8{
		1,
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/max-drop-probability-percent using value %v", input), func(t *testing.T) {
			baseConfigQueueManagementProfile := setup.GetAnyValue(baseConfig.QueueManagementProfile)

			config := dut.Config().Qos().QueueManagementProfile(*baseConfigQueueManagementProfile.Name).Wred().Uniform().MaxDropProbabilityPercent()
			state := dut.Telemetry().Qos().QueueManagementProfile(*baseConfigQueueManagementProfile.Name).Wred().Uniform().MaxDropProbabilityPercent()

			t.Run("Replace leaf", func(t *testing.T) {
				config.Replace(t, input)
			})
			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot != input {
						t.Errorf("Config /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/max-drop-probability-percent: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot != input {
						t.Errorf("State /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/max-drop-probability-percent: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete leaf", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs != nil {
						t.Errorf("Delete /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/max-drop-probability-percent fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestDropAtContainer(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	inputs := []bool{
		true,
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/drop using value %v", input), func(t *testing.T) {
			baseConfigQueueManagementProfile := setup.GetAnyValue(baseConfig.QueueManagementProfile)
			baseConfigQueueManagementProfileWred := baseConfigQueueManagementProfile.Wred
			baseConfigQueueManagementProfileWredUniform := baseConfigQueueManagementProfileWred.Uniform
			*baseConfigQueueManagementProfileWredUniform.Drop = input

			config := dut.Config().Qos().QueueManagementProfile(*baseConfigQueueManagementProfile.Name).Wred().Uniform()
			state := dut.Telemetry().Qos().QueueManagementProfile(*baseConfigQueueManagementProfile.Name).Wred().Uniform()

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigQueueManagementProfileWredUniform)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.Drop != input {
						t.Errorf("Config /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/drop: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.Drop != input {
						t.Errorf("State /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/drop: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete container", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).Drop != nil {
						t.Errorf("Delete /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/drop fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestDropAtLeaf(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	inputs := []bool{
		true,
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/drop using value %v", input), func(t *testing.T) {
			baseConfigQueueManagementProfile := setup.GetAnyValue(baseConfig.QueueManagementProfile)

			config := dut.Config().Qos().QueueManagementProfile(*baseConfigQueueManagementProfile.Name).Wred().Uniform().Drop()
			state := dut.Telemetry().Qos().QueueManagementProfile(*baseConfigQueueManagementProfile.Name).Wred().Uniform().Drop()

			t.Run("Replace leaf", func(t *testing.T) {
				config.Replace(t, input)
			})
			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot != input {
						t.Errorf("Config /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/drop: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot != input {
						t.Errorf("State /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/drop: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete leaf", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs != nil {
						t.Errorf("Delete /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/drop fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestMaxThresholdAtContainer(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	inputs := []uint64{
		2123107431377213550,
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/max-threshold using value %v", input), func(t *testing.T) {
			baseConfigQueueManagementProfile := setup.GetAnyValue(baseConfig.QueueManagementProfile)
			baseConfigQueueManagementProfileWred := baseConfigQueueManagementProfile.Wred
			baseConfigQueueManagementProfileWredUniform := baseConfigQueueManagementProfileWred.Uniform
			*baseConfigQueueManagementProfileWredUniform.MaxThreshold = input

			config := dut.Config().Qos().QueueManagementProfile(*baseConfigQueueManagementProfile.Name).Wred().Uniform()
			state := dut.Telemetry().Qos().QueueManagementProfile(*baseConfigQueueManagementProfile.Name).Wred().Uniform()

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigQueueManagementProfileWredUniform)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.MaxThreshold != input {
						t.Errorf("Config /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/max-threshold: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.MaxThreshold != input {
						t.Errorf("State /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/max-threshold: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete container", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).MaxThreshold != nil {
						t.Errorf("Delete /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/max-threshold fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestMaxThresholdAtLeaf(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	inputs := []uint64{
		2123107431377213550,
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/max-threshold using value %v", input), func(t *testing.T) {
			baseConfigQueueManagementProfile := setup.GetAnyValue(baseConfig.QueueManagementProfile)

			config := dut.Config().Qos().QueueManagementProfile(*baseConfigQueueManagementProfile.Name).Wred().Uniform().MaxThreshold()
			state := dut.Telemetry().Qos().QueueManagementProfile(*baseConfigQueueManagementProfile.Name).Wred().Uniform().MaxThreshold()

			t.Run("Replace leaf", func(t *testing.T) {
				config.Replace(t, input)
			})
			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot != input {
						t.Errorf("Config /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/max-threshold: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot != input {
						t.Errorf("State /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/max-threshold: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete leaf", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs != nil {
						t.Errorf("Delete /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/max-threshold fail: got %v", qs)
					}
				}
			})
		})
	}
}
