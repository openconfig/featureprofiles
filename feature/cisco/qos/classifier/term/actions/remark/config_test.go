package qos_test

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/feature/cisco/qos/setup"
	"github.com/openconfig/featureprofiles/topologies/binding"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
)

func TestMain(m *testing.M) {
	ondatra.RunTests(m, binding.New)
}

// "error-message": "Edit/Update request should have Conditions/Classifier_config_type"
func TestSetDscpAtContainer(t *testing.T) {
	t.Skip()
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQos(t, dut, "base_config_classifier_term_ipv4.json")
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testSetDscpInput {
		t.Run(fmt.Sprintf("Testing /qos/classifiers/classifier/terms/term/actions/remark/config/set-dscp using value %v", input), func(t *testing.T) {
			baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)
			baseConfigClassifierTerm := setup.GetAnyValue(baseConfigClassifier.Term)
			baseConfigClassifierTermActions := baseConfigClassifierTerm.Actions
			baseConfigClassifierTermActionsRemark := baseConfigClassifierTermActions.Remark
			*baseConfigClassifierTermActionsRemark.SetDscp = input

			config := gnmi.OC().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Actions().Remark()
			state := gnmi.OC().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Actions().Remark()

			t.Run("Replace container", func(t *testing.T) {
				gnmi.Replace(t, dut, config.Config(), baseConfigClassifierTermActionsRemark)
			})
			t.Run("Get container", func(t *testing.T) {
				configGot := gnmi.Get(t, dut, config.Config())
				if diff := cmp.Diff(*configGot, *baseConfigClassifierTermActionsRemark); diff != "" {
					t.Errorf("Config /qos/classifiers/classifier/terms/term/actions/remark/config/set-dscp: %v", diff)
				}
			})
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := gnmi.Get(t, dut, state.State())
					if diff := cmp.Diff(*stateGot, *baseConfigClassifierTermActionsRemark); diff != "" {
						t.Errorf("State /qos/classifiers/classifier/terms/term/actions/remark/config/set-dscp: %v", diff)
					}
				})
			}
			t.Run("Delete container", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				if qs := gnmi.LookupConfig(t, dut, config.Config()); qs != nil {
					t.Errorf("Delete /qos/classifiers/classifier/terms/term/actions/remark/config/set-dscp fail: got %v", qs)
				}
			})
		})
	}
}

// "error-message": "Edit/Update request should have Conditions/Classifier_config_type"
func TestSetDscpAtLeaf(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	var baseConfig *oc.Qos = setupQos(t, dut, "base_config_classifier_term_ipv4.json")
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testSetDscpInput {
		t.Run(fmt.Sprintf("Testing /qos/classifiers/classifier/terms/term/actions/remark/config/set-dscp using value %v", input), func(t *testing.T) {
			baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)
			baseConfigClassifierTerm := setup.GetAnyValue(baseConfigClassifier.Term)
			// *baseConfigClassifierTerm.Actions.Remark.SetDscp = input

			config := gnmi.OC().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Actions().Remark().SetDscp()
			state := gnmi.OC().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Actions().Remark().SetDscp()

			t.Run("Replace leaf", func(t *testing.T) {
				t.Skip()
				gnmi.Replace(t, dut, config.Config(), input)
			})
			t.Run("Get leaf", func(t *testing.T) {
				configGot := gnmi.Get(t, dut, config.Config())
				if configGot != *baseConfigClassifierTerm.Actions.Remark.SetDscp {
					t.Errorf("Config /qos/classifiers/classifier/terms/term/actions/remark/config/set-dscp: got %v, want %v", configGot, input)
				}
			})
			// No sysdb paths found for yang path qos/classifiers/classifier/terms/term/actions/remark/state/set-dscp
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := gnmi.Get(t, dut, state.State())
					if stateGot != input {
						t.Errorf("State /qos/classifiers/classifier/terms/term/actions/remark/config/set-dscp: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete leaf", func(t *testing.T) {
				t.Skip()
				gnmi.Delete(t, dut, config.Config())
				if qs := gnmi.LookupConfig(t, dut, config.Config()); qs != nil {
					t.Errorf("Delete /qos/classifiers/classifier/terms/term/actions/remark/config/set-dscp fail: got %v", qs)
				}
			})
		})
	}
}

// "error-message": "Edit/Update request should have Conditions/Classifier_config_type"
func TestSetMplsTcAtContainer(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQos(t, dut, "base_config_classifier_term_mpls.json")
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testSetMplsTcInput {
		t.Run(fmt.Sprintf("Testing /qos/classifiers/classifier/terms/term/actions/remark/config/set-mpls-tc using value %v", input), func(t *testing.T) {
			t.Skip()
			baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)
			baseConfigClassifierTerm := setup.GetAnyValue(baseConfigClassifier.Term)
			baseConfigClassifierTermActions := baseConfigClassifierTerm.Actions
			baseConfigClassifierTermActionsRemark := baseConfigClassifierTermActions.Remark
			*baseConfigClassifierTermActionsRemark.SetMplsTc = input

			config := gnmi.OC().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Actions().Remark()
			state := gnmi.OC().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Actions().Remark()

			t.Run("Replace container", func(t *testing.T) {
				gnmi.Replace(t, dut, config.Config(), baseConfigClassifierTermActionsRemark)
			})
			t.Run("Get container", func(t *testing.T) {
				configGot := gnmi.Get(t, dut, config.Config())
				if diff := cmp.Diff(*configGot, *baseConfigClassifierTermActionsRemark); diff != "" {
					t.Errorf("Config /qos/classifiers/classifier/terms/term/actions/remark/config/set-mpls-tc: %v", diff)
				}
			})
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := gnmi.Get(t, dut, state.State())
					if diff := cmp.Diff(*stateGot, *baseConfigClassifierTermActionsRemark); diff != "" {
						t.Errorf("State /qos/classifiers/classifier/terms/term/actions/remark/config/set-mpls-tc: %v", diff)
					}
				})
			}
			t.Run("Delete container", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				if qs := gnmi.LookupConfig(t, dut, config.Config()); qs != nil {
					t.Errorf("Delete /qos/classifiers/classifier/terms/term/actions/remark/config/set-mpls-tc fail: got %v", qs)
				}
			})
		})
	}
}

// "error-message": "Edit/Update request should have Conditions/Classifier_config_type"
func TestSetMplsTcAtLeaf(t *testing.T) {
	t.Skip()
	dut := ondatra.DUT(t, "dut")
	var baseConfig *oc.Qos = setupQos(t, dut, "base_config_classifier_term_mpls.json")
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testSetMplsTcInput {
		t.Run(fmt.Sprintf("Testing /qos/classifiers/classifier/terms/term/actions/remark/config/set-mpls-tc using value %v", input), func(t *testing.T) {
			baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)
			baseConfigClassifierTerm := setup.GetAnyValue(baseConfigClassifier.Term)

			config := gnmi.OC().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Actions().Remark().SetMplsTc()
			state := gnmi.OC().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Actions().Remark().SetMplsTc()

			t.Run("Replace leaf", func(t *testing.T) {
				gnmi.Replace(t, dut, config.Config(), input)
			})
			t.Run("Get leaf", func(t *testing.T) {
				configGot := gnmi.Get(t, dut, config.Config())
				if configGot != input {
					t.Errorf("Config /qos/classifiers/classifier/terms/term/actions/remark/config/set-mpls-tc: got %v, want %v", configGot, input)
				}
			})
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := gnmi.Get(t, dut, state.State())
					if stateGot != input {
						t.Errorf("State /qos/classifiers/classifier/terms/term/actions/remark/config/set-mpls-tc: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete leaf", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				if qs := gnmi.LookupConfig(t, dut, config.Config()); qs != nil {
					t.Errorf("Delete /qos/classifiers/classifier/terms/term/actions/remark/config/set-mpls-tc fail: got %v", qs)
				}
			})
		})
	}
}
