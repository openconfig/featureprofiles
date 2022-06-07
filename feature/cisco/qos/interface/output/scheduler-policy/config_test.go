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
	setup.ResetStruct(bcInterfaceOutput, []string{"SchedulerPolicy"})
	bcInterfaceOutputSchedulerPolicy := bcInterfaceOutput.SchedulerPolicy
	setup.ResetStruct(bcInterfaceOutputSchedulerPolicy, []string{})
	dut.Config().Qos().Replace(t, bc)
	return bc
}

func teardownQos(t *testing.T, dut *ondatra.DUTDevice, baseConfig *oc.Qos) {
	dut.Config().Qos().Delete(t)
}
func TestNameAtContainer(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	inputs := []string{
		"::",
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /qos/interfaces/interface/output/scheduler-policy/config/name using value %v", input), func(t *testing.T) {
			baseConfigInterface := setup.GetAnyValue(baseConfig.Interface)
			baseConfigInterfaceOutput := baseConfigInterface.Output
			baseConfigInterfaceOutputSchedulerPolicy := baseConfigInterfaceOutput.SchedulerPolicy
			*baseConfigInterfaceOutputSchedulerPolicy.Name = input

			config := dut.Config().Qos().Interface(*baseConfigInterface.InterfaceId).Output().SchedulerPolicy()
			state := dut.Telemetry().Qos().Interface(*baseConfigInterface.InterfaceId).Output().SchedulerPolicy()

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigInterfaceOutputSchedulerPolicy)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.Name != input {
						t.Errorf("Config /qos/interfaces/interface/output/scheduler-policy/config/name: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.Name != input {
						t.Errorf("State /qos/interfaces/interface/output/scheduler-policy/config/name: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete container", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).Name != nil {
						t.Errorf("Delete /qos/interfaces/interface/output/scheduler-policy/config/name fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestNameAtLeaf(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	inputs := []string{
		"::",
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /qos/interfaces/interface/output/scheduler-policy/config/name using value %v", input), func(t *testing.T) {
			baseConfigInterface := setup.GetAnyValue(baseConfig.Interface)

			config := dut.Config().Qos().Interface(*baseConfigInterface.InterfaceId).Output().SchedulerPolicy().Name()
			state := dut.Telemetry().Qos().Interface(*baseConfigInterface.InterfaceId).Output().SchedulerPolicy().Name()

			t.Run("Replace leaf", func(t *testing.T) {
				config.Replace(t, input)
			})
			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot != input {
						t.Errorf("Config /qos/interfaces/interface/output/scheduler-policy/config/name: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot != input {
						t.Errorf("State /qos/interfaces/interface/output/scheduler-policy/config/name: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete leaf", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs != nil {
						t.Errorf("Delete /qos/interfaces/interface/output/scheduler-policy/config/name fail: got %v", qs)
					}
				}
			})
		})
	}
}
