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
	setup.ResetStruct(bc, []string{"Queue"})
	bcQueue := setup.GetAnyValue(bc.Queue)
	setup.ResetStruct(bcQueue, []string{})
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
		"a",
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /qos/queues/queue/config/name using value %v", input), func(t *testing.T) {
			baseConfigQueue := setup.GetAnyValue(baseConfig.Queue)
			*baseConfigQueue.Name = input

			config := dut.Config().Qos().Queue(*baseConfigQueue.Name)
			state := dut.Telemetry().Qos().Queue(*baseConfigQueue.Name)

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigQueue)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.Name != input {
						t.Errorf("Config /qos/queues/queue/config/name: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.Name != input {
						t.Errorf("State /qos/queues/queue/config/name: got %v, want %v", stateGot, input)
					}
				})
			}
		})
	}
}
