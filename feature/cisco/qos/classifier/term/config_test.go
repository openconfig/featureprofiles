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
	"github.com/openconfig/testt"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	ondatra.RunTests(m, binding.New)
}

func TestIdAtLeaf(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQos(t, dut, "base_config_classifier_term.json")
	defer teardownQos(t, dut, baseConfig)

	t.Run("Testing /qos/classifiers/classifier/terms/term/config/id", func(t *testing.T) {
		baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)
		baseConfigClassifierTerm := setup.GetAnyValue(baseConfigClassifier.Term)

		config := gnmi.OC().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Id()
		state := gnmi.OC().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Id()

		t.Run("Get container", func(t *testing.T) {
			configGot := gnmi.GetConfig(t, dut, config.Config())
			if configGot != *baseConfigClassifierTerm.Id {
				t.Errorf("Config /qos/classifiers/classifier/terms/term/config/id: want %s got %s", *baseConfigClassifierTerm.Id, configGot)
			}
		})
		// No sysdb paths found for yang path qos/classifiers/classifier/terms/term/state/id
		if !setup.SkipSubscribe() {
			t.Run("Subscribe container", func(t *testing.T) {
				stateGot := gnmi.Get(t, dut, state.State())
				if stateGot != *baseConfigClassifierTerm.Id {
					t.Errorf("Config /qos/classifiers/classifier/terms/term/state/id: want %s got %s", *baseConfigClassifierTerm.Id, stateGot)
				}
			})
		}
	})
}

// TestClassifier verifies that the Classifier configuration paths can be read,
// updated, and deleted.
//
// path:/qos/classifiers/classifier/terms/term
func TestTerm(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQos(t, dut, "base_config_classifier_term.json")
	defer teardownQos(t, dut, baseConfig)

	baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)

	t.Run("Testing /qos/classifiers/classifier/terms/term", func(t *testing.T) {
		for termId, baseConfigTerm := range baseConfigClassifier.Term {
			newTermId := fmt.Sprintf("%s_new", termId)
			baseConfigTerm.Id = ygot.String(newTermId)

			config := gnmi.OC().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigTerm.Id)

			t.Run(fmt.Sprintf("Create class-map %s", *baseConfigTerm.Id), func(t *testing.T) {
				gnmi.Update(t, dut, config.Config(), baseConfigTerm)
				// config.Replace(t, baseConfigTerm)
			})
			t.Run(fmt.Sprintf("Get class-map %s", *baseConfigTerm.Id), func(t *testing.T) {
				configGot := gnmi.GetConfig(t, dut, config.Config())
				if diff := cmp.Diff(*configGot, *baseConfigTerm); diff != "" {
					t.Errorf("Config class-map fail: \n%v", diff)
				}
			})
			t.Run(fmt.Sprintf("Delete class-map %s", *baseConfigTerm.Id), func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				// Lookup not working
				// if qs := config.Lookup(t); qs != nil {
				// 	t.Errorf("Delete class map fail: got %v", qs)
				// }
			})
			t.Run(fmt.Sprintf("Re create class-map %s", *baseConfigTerm.Id), func(t *testing.T) {
				gnmi.Update(t, dut, config.Config(), baseConfigTerm)
				// config.Replace(t, baseConfigTerm)
			})
			t.Run(fmt.Sprintf("Get class-map %s", *baseConfigTerm.Id), func(t *testing.T) {
				configGot := gnmi.GetConfig(t, dut, config.Config())
				if diff := cmp.Diff(*configGot, *baseConfigTerm); diff != "" {
					t.Errorf("Config class-map fail: \n%v", diff)
				}
			})
		}
	})
}

func TestDeleteAllClassMaps(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQos(t, dut, "base_config_classifier_term.json")

	baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)

	t.Run(fmt.Sprintf("Delete class-maps one-by-one from %s", *baseConfigClassifier.Name), func(t *testing.T) {
		for termId := range baseConfigClassifier.Term {
			config := gnmi.OC().Qos().Classifier(*baseConfigClassifier.Name).Term(termId)
			gnmi.Delete(t, dut, config.Config())
		}
	})
	//gnmi.Delete(t, dut, gnmi.OC().Qos().Classifier(*baseConfigClassifier.Name).Config())

	t.Run(fmt.Sprintf("Verify whether %s is deleted or not", *baseConfigClassifier.Name), func(t *testing.T) {
		for termId := range baseConfigClassifier.Term {
			config := gnmi.OC().Qos().Classifier(*baseConfigClassifier.Name).Term(termId)
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				gnmi.GetConfig(t, dut, config.Config()) //catch the error  as it is expected and absorb the panic.
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This update should have failed ")
			}
		}
	})
}

func TestSetDscp(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQos(t, dut, "base_config_classifier_term.json")
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testSetDscpInput {
		t.Run(fmt.Sprintf("Testing /qos/classifiers/classifier/terms/term/actions/remark/config/set-dscp using value %v", input), func(t *testing.T) {
			baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)
			baseConfigClassifierTerm := setup.GetAnyValue(baseConfigClassifier.Term)
			baseConfigClassifierTerm.Actions.Remark.SetDscp = ygot.Uint8(input)

			config := gnmi.OC().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id)
			state := gnmi.OC().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id)

			t.Run("Replace container", func(t *testing.T) {
				gnmi.Replace(t, dut, config.Config(), baseConfigClassifierTerm)
			})
			t.Run("Get container", func(t *testing.T) {
				configGot := gnmi.GetConfig(t, dut, config.Config())
				if diff := cmp.Diff(*configGot, *baseConfigClassifierTerm); diff != "" {
					t.Errorf("Config /qos/classifiers/classifier/terms/term/actions/remark/config/set-dscp fail:\n%v", diff)
				}
			})
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := gnmi.Get(t, dut, state.State())
					if diff := cmp.Diff(*stateGot, *baseConfigClassifierTerm); diff != "" {
						t.Errorf("State /qos/classifiers/classifier/terms/term/actions/remark/state/set-dscp fail:\n%v", diff)
					}
				})
			}
		})
	}
}
func TestSetMplsTc(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQos(t, dut, "base_config_classifier_term.json")
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testSetMplsTcInput {
		t.Run(fmt.Sprintf("Testing /qos/classifiers/classifier/terms/term/actions/remark/config/set-mpls-tc using value %v", input), func(t *testing.T) {
			baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)
			baseConfigClassifierTerm := setup.GetAnyValue(baseConfigClassifier.Term)
			baseConfigClassifierTerm.Actions.Remark.SetMplsTc = ygot.Uint8(input)

			config := gnmi.OC().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id)
			state := gnmi.OC().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id)

			t.Run("Replace container", func(t *testing.T) {
				gnmi.Replace(t, dut, config.Config(), baseConfigClassifierTerm)
			})
			t.Run("Get container", func(t *testing.T) {
				configGot := gnmi.GetConfig(t, dut, config.Config())
				if diff := cmp.Diff(*configGot, *baseConfigClassifierTerm); diff != "" {
					t.Errorf("Config /qos/classifiers/classifier/terms/term/actions/remark/config/set-mpls-tc fail:\n%v", diff)
				}
			})
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := gnmi.Get(t, dut, state.State())
					if diff := cmp.Diff(*stateGot, *baseConfigClassifierTerm); diff != "" {
						t.Errorf("State /qos/classifiers/classifier/terms/term/actions/remark/state/set-mpls-tc fail:\n%v", diff)
					}
				})
			}
		})
	}
}
