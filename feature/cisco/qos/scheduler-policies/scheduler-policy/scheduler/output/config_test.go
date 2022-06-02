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
	setup.ResetStruct(bc, []string{"SchedulerPolicy"})
	bcSchedulerPolicy := setup.GetAnyValue(bc.SchedulerPolicy)
	setup.ResetStruct(bcSchedulerPolicy, []string{"Scheduler"})
	bcSchedulerPolicyScheduler := setup.GetAnyValue(bcSchedulerPolicy.Scheduler)
	setup.ResetStruct(bcSchedulerPolicyScheduler, []string{"Output"})
	bcSchedulerPolicySchedulerOutput := bcSchedulerPolicyScheduler.Output
	setup.ResetStruct(bcSchedulerPolicySchedulerOutput, []string{})
	dut.Config().Qos().Replace(t, bc)
	return bc
}

func teardownQos(t *testing.T, dut *ondatra.DUTDevice, baseConfig *oc.Qos) {
	dut.Config().Qos().Delete(t)
}
func TestOutputTypeAtContainer(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	inputs := []oc.E_Output_OutputType{
		oc.E_Output_OutputType(3), //INTERFACE
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/output/config/output-type using value %v", input), func(t *testing.T) {
			baseConfigSchedulerPolicy := setup.GetAnyValue(baseConfig.SchedulerPolicy)
			baseConfigSchedulerPolicyScheduler := setup.GetAnyValue(baseConfigSchedulerPolicy.Scheduler)
			baseConfigSchedulerPolicySchedulerOutput := baseConfigSchedulerPolicyScheduler.Output
			baseConfigSchedulerPolicySchedulerOutput.OutputType = input

			config := dut.Config().Qos().SchedulerPolicy(*baseConfigSchedulerPolicy.Name).Scheduler(*baseConfigSchedulerPolicyScheduler.Sequence).Output()
			state := dut.Telemetry().Qos().SchedulerPolicy(*baseConfigSchedulerPolicy.Name).Scheduler(*baseConfigSchedulerPolicyScheduler.Sequence).Output()

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigSchedulerPolicySchedulerOutput)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot.OutputType != input {
						t.Errorf("Config /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/output/config/output-type: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot.OutputType != input {
						t.Errorf("State /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/output/config/output-type: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete container", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).OutputType != 0 {
						t.Errorf("Delete /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/output/config/output-type fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestOutputTypeAtLeaf(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	inputs := []oc.E_Output_OutputType{
		oc.E_Output_OutputType(3), //INTERFACE
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/output/config/output-type using value %v", input), func(t *testing.T) {
			baseConfigSchedulerPolicy := setup.GetAnyValue(baseConfig.SchedulerPolicy)
			baseConfigSchedulerPolicyScheduler := setup.GetAnyValue(baseConfigSchedulerPolicy.Scheduler)

			config := dut.Config().Qos().SchedulerPolicy(*baseConfigSchedulerPolicy.Name).Scheduler(*baseConfigSchedulerPolicyScheduler.Sequence).Output().OutputType()
			state := dut.Telemetry().Qos().SchedulerPolicy(*baseConfigSchedulerPolicy.Name).Scheduler(*baseConfigSchedulerPolicyScheduler.Sequence).Output().OutputType()

			t.Run("Replace leaf", func(t *testing.T) {
				config.Replace(t, input)
			})
			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot != input {
						t.Errorf("Config /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/output/config/output-type: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot != input {
						t.Errorf("State /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/output/config/output-type: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete leaf", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs != nil {
						t.Errorf("Delete /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/output/config/output-type fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestChildSchedulerAtContainer(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	inputs := []string{
		":c",
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/output/config/child-scheduler using value %v", input), func(t *testing.T) {
			baseConfigSchedulerPolicy := setup.GetAnyValue(baseConfig.SchedulerPolicy)
			baseConfigSchedulerPolicyScheduler := setup.GetAnyValue(baseConfigSchedulerPolicy.Scheduler)
			baseConfigSchedulerPolicySchedulerOutput := baseConfigSchedulerPolicyScheduler.Output
			*baseConfigSchedulerPolicySchedulerOutput.ChildScheduler = input

			config := dut.Config().Qos().SchedulerPolicy(*baseConfigSchedulerPolicy.Name).Scheduler(*baseConfigSchedulerPolicyScheduler.Sequence).Output()
			state := dut.Telemetry().Qos().SchedulerPolicy(*baseConfigSchedulerPolicy.Name).Scheduler(*baseConfigSchedulerPolicyScheduler.Sequence).Output()

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigSchedulerPolicySchedulerOutput)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.ChildScheduler != input {
						t.Errorf("Config /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/output/config/child-scheduler: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.ChildScheduler != input {
						t.Errorf("State /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/output/config/child-scheduler: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete container", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).ChildScheduler != nil {
						t.Errorf("Delete /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/output/config/child-scheduler fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestChildSchedulerAtLeaf(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	inputs := []string{
		":c",
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/output/config/child-scheduler using value %v", input), func(t *testing.T) {
			baseConfigSchedulerPolicy := setup.GetAnyValue(baseConfig.SchedulerPolicy)
			baseConfigSchedulerPolicyScheduler := setup.GetAnyValue(baseConfigSchedulerPolicy.Scheduler)

			config := dut.Config().Qos().SchedulerPolicy(*baseConfigSchedulerPolicy.Name).Scheduler(*baseConfigSchedulerPolicyScheduler.Sequence).Output().ChildScheduler()
			state := dut.Telemetry().Qos().SchedulerPolicy(*baseConfigSchedulerPolicy.Name).Scheduler(*baseConfigSchedulerPolicyScheduler.Sequence).Output().ChildScheduler()

			t.Run("Replace leaf", func(t *testing.T) {
				config.Replace(t, input)
			})
			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot != input {
						t.Errorf("Config /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/output/config/child-scheduler: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot != input {
						t.Errorf("State /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/output/config/child-scheduler: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete leaf", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs != nil {
						t.Errorf("Delete /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/output/config/child-scheduler fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestOutputFwdGroupAtContainer(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	inputs := []string{
		"a",
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/output/config/output-fwd-group using value %v", input), func(t *testing.T) {
			baseConfigSchedulerPolicy := setup.GetAnyValue(baseConfig.SchedulerPolicy)
			baseConfigSchedulerPolicyScheduler := setup.GetAnyValue(baseConfigSchedulerPolicy.Scheduler)
			baseConfigSchedulerPolicySchedulerOutput := baseConfigSchedulerPolicyScheduler.Output
			*baseConfigSchedulerPolicySchedulerOutput.OutputFwdGroup = input

			config := dut.Config().Qos().SchedulerPolicy(*baseConfigSchedulerPolicy.Name).Scheduler(*baseConfigSchedulerPolicyScheduler.Sequence).Output()
			state := dut.Telemetry().Qos().SchedulerPolicy(*baseConfigSchedulerPolicy.Name).Scheduler(*baseConfigSchedulerPolicyScheduler.Sequence).Output()

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigSchedulerPolicySchedulerOutput)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.OutputFwdGroup != input {
						t.Errorf("Config /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/output/config/output-fwd-group: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.OutputFwdGroup != input {
						t.Errorf("State /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/output/config/output-fwd-group: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete container", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).OutputFwdGroup != nil {
						t.Errorf("Delete /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/output/config/output-fwd-group fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestOutputFwdGroupAtLeaf(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	inputs := []string{
		"a",
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/output/config/output-fwd-group using value %v", input), func(t *testing.T) {
			baseConfigSchedulerPolicy := setup.GetAnyValue(baseConfig.SchedulerPolicy)
			baseConfigSchedulerPolicyScheduler := setup.GetAnyValue(baseConfigSchedulerPolicy.Scheduler)

			config := dut.Config().Qos().SchedulerPolicy(*baseConfigSchedulerPolicy.Name).Scheduler(*baseConfigSchedulerPolicyScheduler.Sequence).Output().OutputFwdGroup()
			state := dut.Telemetry().Qos().SchedulerPolicy(*baseConfigSchedulerPolicy.Name).Scheduler(*baseConfigSchedulerPolicyScheduler.Sequence).Output().OutputFwdGroup()

			t.Run("Replace leaf", func(t *testing.T) {
				config.Replace(t, input)
			})
			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot != input {
						t.Errorf("Config /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/output/config/output-fwd-group: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot != input {
						t.Errorf("State /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/output/config/output-fwd-group: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete leaf", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs != nil {
						t.Errorf("Delete /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/output/config/output-fwd-group fail: got %v", qs)
					}
				}
			})
		})
	}
}
