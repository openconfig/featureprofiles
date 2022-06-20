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

func TestNameAtContainer(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testNameInput {
		t.Run(fmt.Sprintf("Testing /qos/interfaces/interface/input/virtual-output-queues/voq-interface/config/name using value %v", input), func(t *testing.T) {
			baseConfigInterface := setup.GetAnyValue(baseConfig.Interface)
			baseConfigInterfaceInput := baseConfigInterface.Input
			baseConfigInterfaceInputVoqInterface := setup.GetAnyValue(baseConfigInterfaceInput.VoqInterface)
			*baseConfigInterfaceInputVoqInterface.Name = input

			config := dut.Config().Qos().Interface(*baseConfigInterface.InterfaceId).Input().VoqInterface(*baseConfigInterfaceInputVoqInterface.Name)
			state := dut.Telemetry().Qos().Interface(*baseConfigInterface.InterfaceId).Input().VoqInterface(*baseConfigInterfaceInputVoqInterface.Name)

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigInterfaceInputVoqInterface)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.Name != input {
						t.Errorf("Config /qos/interfaces/interface/input/virtual-output-queues/voq-interface/config/name: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.Name != input {
						t.Errorf("State /qos/interfaces/interface/input/virtual-output-queues/voq-interface/config/name: got %v, want %v", stateGot, input)
					}
				})
			}
		})
	}
}
