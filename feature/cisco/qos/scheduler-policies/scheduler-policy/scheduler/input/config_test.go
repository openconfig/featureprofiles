package qos_test

import (
	"fmt"
	"testing"

	"github.com/openconfig/featureprofiles/feature/cisco/qos/setup"
	"github.com/openconfig/featureprofiles/topologies/binding"
	"github.com/openconfig/ondatra"
	oc "github.com/openconfig/ondatra/telemetry"
)

func TestMain(m *testing.M) {
	ondatra.RunTests(m, binding.New)
}

func TestQueueAtContainer(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testQueueInput {
		t.Run(fmt.Sprintf("Testing /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/queue using value %v", input), func(t *testing.T) {
			baseConfigSchedulerPolicy := setup.GetAnyValue(baseConfig.SchedulerPolicy)
			baseConfigSchedulerPolicyScheduler := setup.GetAnyValue(baseConfigSchedulerPolicy.Scheduler)
			baseConfigSchedulerPolicySchedulerInput := setup.GetAnyValue(baseConfigSchedulerPolicyScheduler.Input)
			*baseConfigSchedulerPolicySchedulerInput.Queue = input

			config := dut.Config().Qos().SchedulerPolicy(*baseConfigSchedulerPolicy.Name).Scheduler(*baseConfigSchedulerPolicyScheduler.Sequence).Input(*baseConfigSchedulerPolicySchedulerInput.Id)
			state := dut.Telemetry().Qos().SchedulerPolicy(*baseConfigSchedulerPolicy.Name).Scheduler(*baseConfigSchedulerPolicyScheduler.Sequence).Input(*baseConfigSchedulerPolicySchedulerInput.Id)

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigSchedulerPolicySchedulerInput)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.Queue != input {
						t.Errorf("Config /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/queue: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.Queue != input {
						t.Errorf("State /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/queue: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete container", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).Queue != nil {
						t.Errorf("Delete /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/queue fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestQueueAtLeaf(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testQueueInput {
		t.Run(fmt.Sprintf("Testing /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/queue using value %v", input), func(t *testing.T) {
			baseConfigSchedulerPolicy := setup.GetAnyValue(baseConfig.SchedulerPolicy)
			baseConfigSchedulerPolicyScheduler := setup.GetAnyValue(baseConfigSchedulerPolicy.Scheduler)
			baseConfigSchedulerPolicySchedulerInput := setup.GetAnyValue(baseConfigSchedulerPolicyScheduler.Input)

			config := dut.Config().Qos().SchedulerPolicy(*baseConfigSchedulerPolicy.Name).Scheduler(*baseConfigSchedulerPolicyScheduler.Sequence).Input(*baseConfigSchedulerPolicySchedulerInput.Id).Queue()
			state := dut.Telemetry().Qos().SchedulerPolicy(*baseConfigSchedulerPolicy.Name).Scheduler(*baseConfigSchedulerPolicyScheduler.Sequence).Input(*baseConfigSchedulerPolicySchedulerInput.Id).Queue()

			t.Run("Replace leaf", func(t *testing.T) {
				config.Replace(t, input)
			})
			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot != input {
						t.Errorf("Config /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/queue: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot != input {
						t.Errorf("State /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/queue: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete leaf", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs != nil {
						t.Errorf("Delete /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/queue fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestIdAtContainer(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testIdInput {
		t.Run(fmt.Sprintf("Testing /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/id using value %v", input), func(t *testing.T) {
			baseConfigSchedulerPolicy := setup.GetAnyValue(baseConfig.SchedulerPolicy)
			baseConfigSchedulerPolicyScheduler := setup.GetAnyValue(baseConfigSchedulerPolicy.Scheduler)
			baseConfigSchedulerPolicySchedulerInput := setup.GetAnyValue(baseConfigSchedulerPolicyScheduler.Input)
			*baseConfigSchedulerPolicySchedulerInput.Id = input

			config := dut.Config().Qos().SchedulerPolicy(*baseConfigSchedulerPolicy.Name).Scheduler(*baseConfigSchedulerPolicyScheduler.Sequence).Input(*baseConfigSchedulerPolicySchedulerInput.Id)
			state := dut.Telemetry().Qos().SchedulerPolicy(*baseConfigSchedulerPolicy.Name).Scheduler(*baseConfigSchedulerPolicyScheduler.Sequence).Input(*baseConfigSchedulerPolicySchedulerInput.Id)

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigSchedulerPolicySchedulerInput)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.Id != input {
						t.Errorf("Config /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/id: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.Id != input {
						t.Errorf("State /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/id: got %v, want %v", stateGot, input)
					}
				})
			}
		})
	}
}
func TestWeightAtContainer(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testWeightInput {
		t.Run(fmt.Sprintf("Testing /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/weight using value %v", input), func(t *testing.T) {
			baseConfigSchedulerPolicy := setup.GetAnyValue(baseConfig.SchedulerPolicy)
			baseConfigSchedulerPolicyScheduler := setup.GetAnyValue(baseConfigSchedulerPolicy.Scheduler)
			baseConfigSchedulerPolicySchedulerInput := setup.GetAnyValue(baseConfigSchedulerPolicyScheduler.Input)
			*baseConfigSchedulerPolicySchedulerInput.Weight = input

			config := dut.Config().Qos().SchedulerPolicy(*baseConfigSchedulerPolicy.Name).Scheduler(*baseConfigSchedulerPolicyScheduler.Sequence).Input(*baseConfigSchedulerPolicySchedulerInput.Id)
			state := dut.Telemetry().Qos().SchedulerPolicy(*baseConfigSchedulerPolicy.Name).Scheduler(*baseConfigSchedulerPolicyScheduler.Sequence).Input(*baseConfigSchedulerPolicySchedulerInput.Id)

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigSchedulerPolicySchedulerInput)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.Weight != input {
						t.Errorf("Config /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/weight: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.Weight != input {
						t.Errorf("State /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/weight: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete container", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).Weight != nil {
						t.Errorf("Delete /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/weight fail: got %v", qs)
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

	for _, input := range testWeightInput {
		t.Run(fmt.Sprintf("Testing /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/weight using value %v", input), func(t *testing.T) {
			baseConfigSchedulerPolicy := setup.GetAnyValue(baseConfig.SchedulerPolicy)
			baseConfigSchedulerPolicyScheduler := setup.GetAnyValue(baseConfigSchedulerPolicy.Scheduler)
			baseConfigSchedulerPolicySchedulerInput := setup.GetAnyValue(baseConfigSchedulerPolicyScheduler.Input)

			config := dut.Config().Qos().SchedulerPolicy(*baseConfigSchedulerPolicy.Name).Scheduler(*baseConfigSchedulerPolicyScheduler.Sequence).Input(*baseConfigSchedulerPolicySchedulerInput.Id).Weight()
			state := dut.Telemetry().Qos().SchedulerPolicy(*baseConfigSchedulerPolicy.Name).Scheduler(*baseConfigSchedulerPolicyScheduler.Sequence).Input(*baseConfigSchedulerPolicySchedulerInput.Id).Weight()

			t.Run("Replace leaf", func(t *testing.T) {
				config.Replace(t, input)
			})
			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot != input {
						t.Errorf("Config /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/weight: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot != input {
						t.Errorf("State /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/weight: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete leaf", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs != nil {
						t.Errorf("Delete /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/weight fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestInputTypeAtContainer(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testInputTypeInput {
		t.Run(fmt.Sprintf("Testing /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/input-type using value %v", input), func(t *testing.T) {
			baseConfigSchedulerPolicy := setup.GetAnyValue(baseConfig.SchedulerPolicy)
			baseConfigSchedulerPolicyScheduler := setup.GetAnyValue(baseConfigSchedulerPolicy.Scheduler)
			baseConfigSchedulerPolicySchedulerInput := setup.GetAnyValue(baseConfigSchedulerPolicyScheduler.Input)
			baseConfigSchedulerPolicySchedulerInput.InputType = input

			config := dut.Config().Qos().SchedulerPolicy(*baseConfigSchedulerPolicy.Name).Scheduler(*baseConfigSchedulerPolicyScheduler.Sequence).Input(*baseConfigSchedulerPolicySchedulerInput.Id)
			state := dut.Telemetry().Qos().SchedulerPolicy(*baseConfigSchedulerPolicy.Name).Scheduler(*baseConfigSchedulerPolicyScheduler.Sequence).Input(*baseConfigSchedulerPolicySchedulerInput.Id)

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigSchedulerPolicySchedulerInput)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot.InputType != input {
						t.Errorf("Config /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/input-type: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot.InputType != input {
						t.Errorf("State /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/input-type: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete container", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).InputType != 0 {
						t.Errorf("Delete /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/input-type fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestInputTypeAtLeaf(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testInputTypeInput {
		t.Run(fmt.Sprintf("Testing /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/input-type using value %v", input), func(t *testing.T) {
			baseConfigSchedulerPolicy := setup.GetAnyValue(baseConfig.SchedulerPolicy)
			baseConfigSchedulerPolicyScheduler := setup.GetAnyValue(baseConfigSchedulerPolicy.Scheduler)
			baseConfigSchedulerPolicySchedulerInput := setup.GetAnyValue(baseConfigSchedulerPolicyScheduler.Input)

			config := dut.Config().Qos().SchedulerPolicy(*baseConfigSchedulerPolicy.Name).Scheduler(*baseConfigSchedulerPolicyScheduler.Sequence).Input(*baseConfigSchedulerPolicySchedulerInput.Id).InputType()
			state := dut.Telemetry().Qos().SchedulerPolicy(*baseConfigSchedulerPolicy.Name).Scheduler(*baseConfigSchedulerPolicyScheduler.Sequence).Input(*baseConfigSchedulerPolicySchedulerInput.Id).InputType()

			t.Run("Replace leaf", func(t *testing.T) {
				config.Replace(t, input)
			})
			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot != input {
						t.Errorf("Config /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/input-type: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot != input {
						t.Errorf("State /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/input-type: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete leaf", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs != nil {
						t.Errorf("Delete /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/input-type fail: got %v", qs)
					}
				}
			})
		})
	}
}
