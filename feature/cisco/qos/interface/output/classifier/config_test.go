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

func TestTypeAtContainer(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testTypeInput {
		t.Run(fmt.Sprintf("Testing /qos/interfaces/interface/output/classifiers/classifier/config/type using value %v", input), func(t *testing.T) {
			baseConfigInterface := setup.GetAnyValue(baseConfig.Interface)
			baseConfigInterfaceOutput := baseConfigInterface.Output
			baseConfigInterfaceOutputClassifier := setup.GetAnyValue(baseConfigInterfaceOutput.Classifier)
			baseConfigInterfaceOutputClassifier.Type = input

			config := dut.Config().Qos().Interface(*baseConfigInterface.InterfaceId).Output().Classifier(baseConfigInterfaceOutputClassifier.Type)
			state := dut.Telemetry().Qos().Interface(*baseConfigInterface.InterfaceId).Output().Classifier(baseConfigInterfaceOutputClassifier.Type)

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigInterfaceOutputClassifier)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot.Type != input {
						t.Errorf("Config /qos/interfaces/interface/output/classifiers/classifier/config/type: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot.Type != input {
						t.Errorf("State /qos/interfaces/interface/output/classifiers/classifier/config/type: got %v, want %v", stateGot, input)
					}
				})
			}
		})
	}
}
func TestNameAtContainer(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testNameInput {
		t.Run(fmt.Sprintf("Testing /qos/interfaces/interface/output/classifiers/classifier/config/name using value %v", input), func(t *testing.T) {
			baseConfigInterface := setup.GetAnyValue(baseConfig.Interface)
			baseConfigInterfaceOutput := baseConfigInterface.Output
			baseConfigInterfaceOutputClassifier := setup.GetAnyValue(baseConfigInterfaceOutput.Classifier)
			*baseConfigInterfaceOutputClassifier.Name = input

			config := dut.Config().Qos().Interface(*baseConfigInterface.InterfaceId).Output().Classifier(baseConfigInterfaceOutputClassifier.Type)
			state := dut.Telemetry().Qos().Interface(*baseConfigInterface.InterfaceId).Output().Classifier(baseConfigInterfaceOutputClassifier.Type)

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigInterfaceOutputClassifier)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.Name != input {
						t.Errorf("Config /qos/interfaces/interface/output/classifiers/classifier/config/name: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.Name != input {
						t.Errorf("State /qos/interfaces/interface/output/classifiers/classifier/config/name: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete container", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).Name != nil {
						t.Errorf("Delete /qos/interfaces/interface/output/classifiers/classifier/config/name fail: got %v", qs)
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

	for _, input := range testNameInput {
		t.Run(fmt.Sprintf("Testing /qos/interfaces/interface/output/classifiers/classifier/config/name using value %v", input), func(t *testing.T) {
			baseConfigInterface := setup.GetAnyValue(baseConfig.Interface)
			baseConfigInterfaceOutputClassifier := setup.GetAnyValue(baseConfigInterface.Output.Classifier)

			config := dut.Config().Qos().Interface(*baseConfigInterface.InterfaceId).Output().Classifier(baseConfigInterfaceOutputClassifier.Type).Name()
			state := dut.Telemetry().Qos().Interface(*baseConfigInterface.InterfaceId).Output().Classifier(baseConfigInterfaceOutputClassifier.Type).Name()

			t.Run("Replace leaf", func(t *testing.T) {
				config.Replace(t, input)
			})
			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot != input {
						t.Errorf("Config /qos/interfaces/interface/output/classifiers/classifier/config/name: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot != input {
						t.Errorf("State /qos/interfaces/interface/output/classifiers/classifier/config/name: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete leaf", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs != nil {
						t.Errorf("Delete /qos/interfaces/interface/output/classifiers/classifier/config/name fail: got %v", qs)
					}
				}
			})
		})
	}
}
