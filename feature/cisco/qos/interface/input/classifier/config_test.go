package qos_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/feature/cisco/qos/setup"
	"github.com/openconfig/featureprofiles/topologies/binding"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	ondatra.RunTests(m, binding.New)
}

// This testcase fails
// The expectation from XR side is the set request is applied on all the 3 types,
// which means either ondatra should allow multiple input types or not take type as input.
func TestInterfaceInputClassifier(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	baseConfigInterface := setup.GetAnyValue(baseConfig.Interface)

	defer dut.Config().Qos().Delete(t)

	for _, input := range testNameInput {
		baseConfigInterfaceInput := baseConfigInterface.Input
		baseConfigInterfaceInputClassifier := setup.GetAnyValue(baseConfigInterfaceInput.Classifier)
		baseConfigInterfaceInputClassifier.Name = ygot.String(input)

		config := dut.Config().Qos().Interface(*baseConfigInterface.InterfaceId).Input().Classifier(baseConfigInterfaceInputClassifier.Type)
		state := dut.Telemetry().Qos().Interface(*baseConfigInterface.InterfaceId).Input().Classifier(baseConfigInterfaceInputClassifier.Type)

		t.Run("Replace container", func(t *testing.T) {
			config.Replace(t, baseConfigInterfaceInputClassifier)
		})

		if !setup.SkipGet() {
			t.Run("Get InterfaceInputClassifier Config", func(t *testing.T) {
				configGot := config.Get(t)
				if diff := cmp.Diff(*configGot, *baseConfigInterfaceInputClassifier); diff != "" {
					t.Errorf("Config InterfaceInputClassifier fail:\n%v", diff)
				}
			})
		}
		if !setup.SkipSubscribe() {
			t.Run("Get InterfaceInputClassifier Telemetry", func(t *testing.T) {
				stateGot := state.Get(t)
				if diff := cmp.Diff(*stateGot, *baseConfigInterfaceInputClassifier); diff != "" {
					t.Errorf("Telemetry InterfaceInputClassifier fail:\n%v", diff)
				}
			})
		}
		t.Run("Delete InterfaceInputClassifier", func(t *testing.T) {
			config.Delete(t)
			if !setup.SkipSubscribe() {
				if qs := config.Lookup(t); qs.IsPresent() == true {
					t.Errorf("Delete InterfaceInputClassifier fail: got %v", qs)
				}
			}
		})
	}
}
