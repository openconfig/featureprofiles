package qos_test

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
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

	var baseConfig *oc.Qos = setupQos(t, dut, "base_config_classifier_term_actions.json")
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testTargetGroupInput {
		t.Run(fmt.Sprintf("Testing /qos/classifiers/classifier/terms/term/actions/config/target-group using value %v", input), func(t *testing.T) {
			baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)
			baseConfigClassifierTerm := setup.GetAnyValue(baseConfigClassifier.Term)
			baseConfigClassifierTermActions := baseConfigClassifierTerm.Actions
			// *baseConfigClassifierTermActions.TargetGroup = input

			config := dut.Config().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Actions()
			state := dut.Telemetry().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Actions()

			// "error-message": "Edit/Update request should have Conditions/Classifier_config_type"
			t.Run("Replace container", func(t *testing.T) {
				t.Skip()
				config.Replace(t, baseConfigClassifierTermActions)
			})
			t.Run("Get container", func(t *testing.T) {
				configGot := config.Get(t)
				if diff := cmp.Diff(*configGot, *baseConfigClassifierTermActions); diff != "" {
					t.Errorf("Config /qos/classifiers/classifier/terms/term/actions/config/target-group: %v", diff)
				}
			})
			// No sysdb paths found for yang path qos/classifiers/classifier/terms/term/actions
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					if diff := cmp.Diff(*stateGot, *baseConfigClassifierTermActions); diff != "" {
						t.Errorf("State /qos/classifiers/classifier/terms/term/actions/config/target-group: %v", diff)
					}
				})
			}
			// "error-message": "Edit/Update request should have Conditions/Classifier_config_type"
			t.Run("Delete container", func(t *testing.T) {
				t.Skip()
				config.Delete(t)
				if qs := config.Lookup(t); qs != nil {
					t.Errorf("Delete /qos/classifiers/classifier/terms/term/actions/config/target-group fail: got %v", qs)
				}
			})
		})
	}
}

func TestTargetGroupAtLeaf(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	var baseConfig *oc.Qos = setupQos(t, dut, "base_config_classifier_term_actions.json")
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testTargetGroupInput {
		t.Run(fmt.Sprintf("Testing /qos/classifiers/classifier/terms/term/actions/config/target-group using value %v", input), func(t *testing.T) {
			baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)
			baseConfigClassifierTerm := setup.GetAnyValue(baseConfigClassifier.Term)
			// *baseConfigClassifierTerm.Actions.TargetGroup = input

			config := dut.Config().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Actions().TargetGroup()
			state := dut.Telemetry().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Actions().TargetGroup()

			// "error-message": "Edit/Update request should have Conditions/Classifier_config_type"
			t.Run("Replace leaf", func(t *testing.T) {
				t.Skip()
				config.Replace(t, input)
			})
			t.Run("Get leaf", func(t *testing.T) {
				configGot := config.Get(t)
				if configGot != *baseConfigClassifierTerm.Actions.TargetGroup {
					t.Errorf("Config /qos/classifiers/classifier/terms/term/actions/config/target-group: got %v, want %v", configGot, input)
				}
			})
			// No sysdb paths found for yang path qos/classifiers/classifier/terms/term/actions/state/target-group
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot != input {
						t.Errorf("State /qos/classifiers/classifier/terms/term/actions/state/target-group: got %v, want %v", stateGot, input)
					}
				})
			}
			// "error-message": "Edit/Update request should have Conditions/Classifier_config_type"
			t.Run("Delete leaf", func(t *testing.T) {
				t.Skip()
				config.Delete(t)
				if qs := config.Lookup(t); qs != nil {
					t.Errorf("Delete /qos/classifiers/classifier/terms/term/actions/config/target-group fail: got %v", qs)
				}
			})
		})
	}
}
