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

func TestTargetGroupAtContainer(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testTargetGroupInput {
		t.Run(fmt.Sprintf("Testing /qos/classifiers/classifier/terms/term/actions/config/target-group using value %v", input), func(t *testing.T) {
			baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)
			baseConfigClassifierTerm := setup.GetAnyValue(baseConfigClassifier.Term)
			baseConfigClassifierTermActions := baseConfigClassifierTerm.Actions
			*baseConfigClassifierTermActions.TargetGroup = input

			config := dut.Config().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Actions()
			state := dut.Telemetry().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Actions()

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigClassifierTermActions)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.TargetGroup != input {
						t.Errorf("Config /qos/classifiers/classifier/terms/term/actions/config/target-group: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.TargetGroup != input {
						t.Errorf("State /qos/classifiers/classifier/terms/term/actions/config/target-group: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete container", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).TargetGroup != nil {
						t.Errorf("Delete /qos/classifiers/classifier/terms/term/actions/config/target-group fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestTargetGroupAtLeaf(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testTargetGroupInput {
		t.Run(fmt.Sprintf("Testing /qos/classifiers/classifier/terms/term/actions/config/target-group using value %v", input), func(t *testing.T) {
			baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)
			baseConfigClassifierTerm := setup.GetAnyValue(baseConfigClassifier.Term)

			config := dut.Config().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Actions().TargetGroup()
			state := dut.Telemetry().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Actions().TargetGroup()

			t.Run("Replace leaf", func(t *testing.T) {
				config.Replace(t, input)
			})
			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot != input {
						t.Errorf("Config /qos/classifiers/classifier/terms/term/actions/config/target-group: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot != input {
						t.Errorf("State /qos/classifiers/classifier/terms/term/actions/config/target-group: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete leaf", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs != nil {
						t.Errorf("Delete /qos/classifiers/classifier/terms/term/actions/config/target-group fail: got %v", qs)
					}
				}
			})
		})
	}
}
