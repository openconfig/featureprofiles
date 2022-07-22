package qos_test

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/feature/cisco/qos/setup"
	"github.com/openconfig/featureprofiles/topologies/binding"
	"github.com/openconfig/ondatra"
	oc "github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	ondatra.RunTests(m, binding.New)
}

// Fails if "type" field in base_config
// CSCwc13851
func TestIdAtContainer(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQos(t, dut, "base_config_classifier_term.json")
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testIdInput {
		t.Run(fmt.Sprintf("Testing /qos/classifiers/classifier/terms/term/config/id using value %v", input), func(t *testing.T) {
			baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)
			baseConfigClassifierTerm := setup.GetAnyValue(baseConfigClassifier.Term)
			*baseConfigClassifierTerm.Id = input

			config := dut.Config().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id)
			state := dut.Telemetry().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id)

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigClassifierTerm)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if diff := cmp.Diff(*configGot, *baseConfigClassifierTerm); diff != "" {
						t.Errorf("Config /qos/classifiers/classifier/terms/term/config/id: %v", diff)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					if diff := cmp.Diff(*stateGot, *baseConfigClassifierTerm); diff != "" {
						t.Errorf("Config /qos/classifiers/classifier/terms/term/config/id: %v", diff)
					}
				})
			}
		})
	}
}

// TestClassifier verifies that the Classifier configuration paths can be read,
// updated, and deleted.
//
// path:/qos/classifiers/classifier/terms/term
func TestTerm(t *testing.T) {
	t.Skip()
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQos(t, dut, "base_config_classifier_term.json")
	defer teardownQos(t, dut, baseConfig)

	baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)

	t.Run("Testing /qos/classifiers/classifier/terms/term", func(t *testing.T) {
		for termId, baseConfigTerm := range baseConfigClassifier.Term {
			newTermId := fmt.Sprintf("%s_new", termId)
			baseConfigTerm.Id = ygot.String(newTermId)

			config := dut.Config().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigTerm.Id)

			t.Run(fmt.Sprintf("Create class-map %s", *baseConfigTerm.Id), func(t *testing.T) {
				config.Update(t, baseConfigTerm)
				// config.Replace(t, baseConfigTerm)
			})
			t.Run(fmt.Sprintf("Get class-map %s", *baseConfigTerm.Id), func(t *testing.T) {
				configGot := config.Get(t)
				if diff := cmp.Diff(*configGot, *baseConfigTerm); diff != "" {
					t.Errorf("Config class-map fail: \n%v", diff)
				}
			})
			t.Run(fmt.Sprintf("Delete class-map %s", *baseConfigTerm.Id), func(t *testing.T) {
				config.Delete(t)
				// Lookup not working
				if qs := config.Lookup(t); qs != nil {
					t.Errorf("Delete class map fail: got %v", qs)
				}
			})
			t.Run(fmt.Sprintf("Re create class-map %s", *baseConfigTerm.Id), func(t *testing.T) {
				config.Update(t, baseConfigTerm)
				// config.Replace(t, baseConfigTerm)
			})
			t.Run(fmt.Sprintf("Get class-map %s", *baseConfigTerm.Id), func(t *testing.T) {
				configGot := config.Get(t)
				if diff := cmp.Diff(*configGot, *baseConfigTerm); diff != "" {
					t.Errorf("Config class-map fail: \n%v", diff)
				}
			})
		}
	})
}

func TestDeleteAllClassMaps(t *testing.T) {
	t.Skip()
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQos(t, dut, "base_config_classifier_term.json")
	defer teardownQos(t, dut, baseConfig)

	baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)

	t.Run(fmt.Sprintf("Delete class-maps one-by-one from %s", *baseConfigClassifier.Name), func(t *testing.T) {
		for termId, _ := range baseConfigClassifier.Term {
			config := dut.Config().Qos().Classifier(*baseConfigClassifier.Name).Term(termId)
			config.Delete(t)
		}
	})
	t.Run(fmt.Sprintf("Verify whether %s is deleted or not", *baseConfigClassifier.Name), func(t *testing.T) {
		config := dut.Config().Qos().Classifier(*baseConfigClassifier.Name)
		if qs := config.Lookup(t); qs.IsPresent() == true {
			t.Errorf("%s is not deleted: got %v", *baseConfigClassifier.Name, qs)
		}
	})
}

func TestSetDscp(t *testing.T) {
	t.Skip()
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQos(t, dut, "base_config_classifier_term.json")
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testSetDscpInput {
		t.Run(fmt.Sprintf("Testing /qos/classifiers/classifier/terms/term/actions/remark/config/set-dscp using value %v", input), func(t *testing.T) {
			baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)
			baseConfigClassifierTerm := setup.GetAnyValue(baseConfigClassifier.Term)
			baseConfigClassifierTerm.Actions.Remark.SetDscp = ygot.Uint8(input)

			config := dut.Config().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id)
			state := dut.Telemetry().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id)

			t.Run("Replace leaf", func(t *testing.T) {
				config.Replace(t, baseConfigClassifierTerm)
			})
			t.Run("Get leaf", func(t *testing.T) {
				configGot := config.Get(t)
				if diff := cmp.Diff(*configGot, *baseConfigClassifierTerm); diff != "" {
					t.Errorf("Config /qos/classifiers/classifier/terms/term/actions/remark/config/set-dscp fail:\n%v", diff)
				}
			})
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := state.Get(t)
					if diff := cmp.Diff(*stateGot, *baseConfigClassifierTerm); diff != "" {
						t.Errorf("State /qos/classifiers/classifier/terms/term/actions/remark/state/set-dscp fail:\n%v", diff)
					}
				})
			}
		})
	}
}
func TestSetMplsTc(t *testing.T) {
	t.Skip()
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQos(t, dut, "base_config_classifier_term.json")
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testSetMplsTcInput {
		t.Run(fmt.Sprintf("Testing /qos/classifiers/classifier/terms/term/actions/remark/config/set-mpls-tc using value %v", input), func(t *testing.T) {
			baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)
			baseConfigClassifierTerm := setup.GetAnyValue(baseConfigClassifier.Term)
			baseConfigClassifierTerm.Actions.Remark.SetMplsTc = ygot.Uint8(input)

			config := dut.Config().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id)
			state := dut.Telemetry().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id)

			t.Run("Replace leaf", func(t *testing.T) {
				config.Replace(t, baseConfigClassifierTerm)
			})
			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := config.Get(t)
					if diff := cmp.Diff(*configGot, *baseConfigClassifierTerm); diff != "" {
						t.Errorf("Config /qos/classifiers/classifier/terms/term/actions/remark/config/set-mpls-tc fail:\n%v", diff)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := state.Get(t)
					if diff := cmp.Diff(*stateGot, *baseConfigClassifierTerm); diff != "" {
						t.Errorf("State /qos/classifiers/classifier/terms/term/actions/remark/state/set-mpls-tc fail:\n%v", diff)
					}
				})
			}
		})
	}
}
