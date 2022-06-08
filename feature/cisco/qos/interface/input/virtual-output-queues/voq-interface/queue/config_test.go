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
	setup.ResetStruct(bcInterfaceInput, []string{"VoqInterface"})
	bcInterfaceInputVoqInterface := setup.GetAnyValue(bcInterfaceInput.VoqInterface)
	setup.ResetStruct(bcInterfaceInputVoqInterface, []string{"Queue"})
	bcInterfaceInputVoqInterfaceQueue := setup.GetAnyValue(bcInterfaceInputVoqInterface.Queue)
	setup.ResetStruct(bcInterfaceInputVoqInterfaceQueue, []string{})
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
		"aiia",
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /qos/interfaces/interface/input/virtual-output-queues/voq-interface/queues/queue/config/name using value %v", input), func(t *testing.T) {
			baseConfigInterface := setup.GetAnyValue(baseConfig.Interface)
			baseConfigInterfaceInput := baseConfigInterface.Input
			baseConfigInterfaceInputVoqInterface := setup.GetAnyValue(baseConfigInterfaceInput.VoqInterface)
			baseConfigInterfaceInputVoqInterfaceQueue := setup.GetAnyValue(baseConfigInterfaceInputVoqInterface.Queue)
			*baseConfigInterfaceInputVoqInterfaceQueue.Name = input

			config := dut.Config().Qos().Interface(*baseConfigInterface.InterfaceId).Input().VoqInterface(*baseConfigInterfaceInputVoqInterface.Name).Queue(*baseConfigInterfaceInputVoqInterfaceQueue.Name)
			state := dut.Telemetry().Qos().Interface(*baseConfigInterface.InterfaceId).Input().VoqInterface(*baseConfigInterfaceInputVoqInterface.Name).Queue(*baseConfigInterfaceInputVoqInterfaceQueue.Name)

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigInterfaceInputVoqInterfaceQueue)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.Name != input {
						t.Errorf("Config /qos/interfaces/interface/input/virtual-output-queues/voq-interface/queues/queue/config/name: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.Name != input {
						t.Errorf("State /qos/interfaces/interface/input/virtual-output-queues/voq-interface/queues/queue/config/name: got %v, want %v", stateGot, input)
					}
				})
			}
		})
	}
}
