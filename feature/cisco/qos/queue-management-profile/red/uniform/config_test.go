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
	setup.ResetStruct(bcQueueManagementProfile, []string{"Red"})
	bcQueueManagementProfileRed := bcQueueManagementProfile.Red
	setup.ResetStruct(bcQueueManagementProfileRed, []string{"Uniform"})
	bcQueueManagementProfileRedUniform := bcQueueManagementProfileRed.Uniform
	setup.ResetStruct(bcQueueManagementProfileRedUniform, []string{})
	dut.Config().Qos().Replace(t, bc)
	return bc
}

func teardownQos(t *testing.T, dut *ondatra.DUTDevice, baseConfig *oc.Qos) {
	dut.Config().Qos().Delete(t)
}
func TestMinThresholdAtContainer(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	inputs := []uint64{
		8873099461260600663,
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /qos/queue-management-profiles/queue-management-profile/red/uniform/config/min-threshold using value %v", input), func(t *testing.T) {
			baseConfigQueueManagementProfile := setup.GetAnyValue(baseConfig.QueueManagementProfile)
			baseConfigQueueManagementProfileRed := baseConfigQueueManagementProfile.Red
			baseConfigQueueManagementProfileRedUniform := baseConfigQueueManagementProfileRed.Uniform
			*baseConfigQueueManagementProfileRedUniform.MinThreshold = input

			config := dut.Config().Qos().QueueManagementProfile(*baseConfigQueueManagementProfile.Name).Red().Uniform()
			state := dut.Telemetry().Qos().QueueManagementProfile(*baseConfigQueueManagementProfile.Name).Red().Uniform()

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigQueueManagementProfileRedUniform)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.MinThreshold != input {
						t.Errorf("Config /qos/queue-management-profiles/queue-management-profile/red/uniform/config/min-threshold: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.MinThreshold != input {
						t.Errorf("State /qos/queue-management-profiles/queue-management-profile/red/uniform/config/min-threshold: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete container", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).MinThreshold != nil {
						t.Errorf("Delete /qos/queue-management-profiles/queue-management-profile/red/uniform/config/min-threshold fail: got %v", qs)
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
		8873099461260600663,
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /qos/queue-management-profiles/queue-management-profile/red/uniform/config/min-threshold using value %v", input), func(t *testing.T) {
			baseConfigQueueManagementProfile := setup.GetAnyValue(baseConfig.QueueManagementProfile)

			config := dut.Config().Qos().QueueManagementProfile(*baseConfigQueueManagementProfile.Name).Red().Uniform().MinThreshold()
			state := dut.Telemetry().Qos().QueueManagementProfile(*baseConfigQueueManagementProfile.Name).Red().Uniform().MinThreshold()

			t.Run("Replace leaf", func(t *testing.T) {
				config.Replace(t, input)
			})
			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot != input {
						t.Errorf("Config /qos/queue-management-profiles/queue-management-profile/red/uniform/config/min-threshold: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot != input {
						t.Errorf("State /qos/queue-management-profiles/queue-management-profile/red/uniform/config/min-threshold: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete leaf", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs != nil {
						t.Errorf("Delete /qos/queue-management-profiles/queue-management-profile/red/uniform/config/min-threshold fail: got %v", qs)
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
		9050309890765255898,
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /qos/queue-management-profiles/queue-management-profile/red/uniform/config/max-threshold using value %v", input), func(t *testing.T) {
			baseConfigQueueManagementProfile := setup.GetAnyValue(baseConfig.QueueManagementProfile)
			baseConfigQueueManagementProfileRed := baseConfigQueueManagementProfile.Red
			baseConfigQueueManagementProfileRedUniform := baseConfigQueueManagementProfileRed.Uniform
			*baseConfigQueueManagementProfileRedUniform.MaxThreshold = input

			config := dut.Config().Qos().QueueManagementProfile(*baseConfigQueueManagementProfile.Name).Red().Uniform()
			state := dut.Telemetry().Qos().QueueManagementProfile(*baseConfigQueueManagementProfile.Name).Red().Uniform()

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigQueueManagementProfileRedUniform)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.MaxThreshold != input {
						t.Errorf("Config /qos/queue-management-profiles/queue-management-profile/red/uniform/config/max-threshold: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.MaxThreshold != input {
						t.Errorf("State /qos/queue-management-profiles/queue-management-profile/red/uniform/config/max-threshold: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete container", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).MaxThreshold != nil {
						t.Errorf("Delete /qos/queue-management-profiles/queue-management-profile/red/uniform/config/max-threshold fail: got %v", qs)
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
		9050309890765255898,
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /qos/queue-management-profiles/queue-management-profile/red/uniform/config/max-threshold using value %v", input), func(t *testing.T) {
			baseConfigQueueManagementProfile := setup.GetAnyValue(baseConfig.QueueManagementProfile)

			config := dut.Config().Qos().QueueManagementProfile(*baseConfigQueueManagementProfile.Name).Red().Uniform().MaxThreshold()
			state := dut.Telemetry().Qos().QueueManagementProfile(*baseConfigQueueManagementProfile.Name).Red().Uniform().MaxThreshold()

			t.Run("Replace leaf", func(t *testing.T) {
				config.Replace(t, input)
			})
			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot != input {
						t.Errorf("Config /qos/queue-management-profiles/queue-management-profile/red/uniform/config/max-threshold: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot != input {
						t.Errorf("State /qos/queue-management-profiles/queue-management-profile/red/uniform/config/max-threshold: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete leaf", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs != nil {
						t.Errorf("Delete /qos/queue-management-profiles/queue-management-profile/red/uniform/config/max-threshold fail: got %v", qs)
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
		false,
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /qos/queue-management-profiles/queue-management-profile/red/uniform/config/drop using value %v", input), func(t *testing.T) {
			baseConfigQueueManagementProfile := setup.GetAnyValue(baseConfig.QueueManagementProfile)
			baseConfigQueueManagementProfileRed := baseConfigQueueManagementProfile.Red
			baseConfigQueueManagementProfileRedUniform := baseConfigQueueManagementProfileRed.Uniform
			*baseConfigQueueManagementProfileRedUniform.Drop = input

			config := dut.Config().Qos().QueueManagementProfile(*baseConfigQueueManagementProfile.Name).Red().Uniform()
			state := dut.Telemetry().Qos().QueueManagementProfile(*baseConfigQueueManagementProfile.Name).Red().Uniform()

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigQueueManagementProfileRedUniform)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.Drop != input {
						t.Errorf("Config /qos/queue-management-profiles/queue-management-profile/red/uniform/config/drop: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.Drop != input {
						t.Errorf("State /qos/queue-management-profiles/queue-management-profile/red/uniform/config/drop: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete container", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).Drop != nil {
						t.Errorf("Delete /qos/queue-management-profiles/queue-management-profile/red/uniform/config/drop fail: got %v", qs)
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
		false,
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /qos/queue-management-profiles/queue-management-profile/red/uniform/config/drop using value %v", input), func(t *testing.T) {
			baseConfigQueueManagementProfile := setup.GetAnyValue(baseConfig.QueueManagementProfile)

			config := dut.Config().Qos().QueueManagementProfile(*baseConfigQueueManagementProfile.Name).Red().Uniform().Drop()
			state := dut.Telemetry().Qos().QueueManagementProfile(*baseConfigQueueManagementProfile.Name).Red().Uniform().Drop()

			t.Run("Replace leaf", func(t *testing.T) {
				config.Replace(t, input)
			})
			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot != input {
						t.Errorf("Config /qos/queue-management-profiles/queue-management-profile/red/uniform/config/drop: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot != input {
						t.Errorf("State /qos/queue-management-profiles/queue-management-profile/red/uniform/config/drop: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete leaf", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs != nil {
						t.Errorf("Delete /qos/queue-management-profiles/queue-management-profile/red/uniform/config/drop fail: got %v", qs)
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
		false,
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /qos/queue-management-profiles/queue-management-profile/red/uniform/config/enable-ecn using value %v", input), func(t *testing.T) {
			baseConfigQueueManagementProfile := setup.GetAnyValue(baseConfig.QueueManagementProfile)
			baseConfigQueueManagementProfileRed := baseConfigQueueManagementProfile.Red
			baseConfigQueueManagementProfileRedUniform := baseConfigQueueManagementProfileRed.Uniform
			*baseConfigQueueManagementProfileRedUniform.EnableEcn = input

			config := dut.Config().Qos().QueueManagementProfile(*baseConfigQueueManagementProfile.Name).Red().Uniform()
			state := dut.Telemetry().Qos().QueueManagementProfile(*baseConfigQueueManagementProfile.Name).Red().Uniform()

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigQueueManagementProfileRedUniform)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.EnableEcn != input {
						t.Errorf("Config /qos/queue-management-profiles/queue-management-profile/red/uniform/config/enable-ecn: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.EnableEcn != input {
						t.Errorf("State /qos/queue-management-profiles/queue-management-profile/red/uniform/config/enable-ecn: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete container", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).EnableEcn != nil {
						t.Errorf("Delete /qos/queue-management-profiles/queue-management-profile/red/uniform/config/enable-ecn fail: got %v", qs)
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
		false,
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /qos/queue-management-profiles/queue-management-profile/red/uniform/config/enable-ecn using value %v", input), func(t *testing.T) {
			baseConfigQueueManagementProfile := setup.GetAnyValue(baseConfig.QueueManagementProfile)

			config := dut.Config().Qos().QueueManagementProfile(*baseConfigQueueManagementProfile.Name).Red().Uniform().EnableEcn()
			state := dut.Telemetry().Qos().QueueManagementProfile(*baseConfigQueueManagementProfile.Name).Red().Uniform().EnableEcn()

			t.Run("Replace leaf", func(t *testing.T) {
				config.Replace(t, input)
			})
			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot != input {
						t.Errorf("Config /qos/queue-management-profiles/queue-management-profile/red/uniform/config/enable-ecn: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot != input {
						t.Errorf("State /qos/queue-management-profiles/queue-management-profile/red/uniform/config/enable-ecn: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete leaf", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs != nil {
						t.Errorf("Delete /qos/queue-management-profiles/queue-management-profile/red/uniform/config/enable-ecn fail: got %v", qs)
					}
				}
			})
		})
	}
}
