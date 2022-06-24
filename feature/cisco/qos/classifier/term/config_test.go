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

func TestTerm(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)

	for termId, baseConfigTerm := range baseConfigClassifier.Term {
		t.Run(fmt.Sprintf("Create and Delete class-map %s", fmt.Sprintf("%s_new", termId)), func(t *testing.T) {
			baseConfigTerm.Id = ygot.String(fmt.Sprintf("%s_new", termId))

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
				if qs := config.Lookup(t); qs.IsPresent() == true {
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
		})
	}
}

func TestDeleteMultipleCmaps(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)

	t.Run("Deleting class-maps one-by-one from pmap", func(t *testing.T) {
		for termId, _ := range baseConfigClassifier.Term {
			config := dut.Config().Qos().Classifier(*baseConfigClassifier.Name).Term(termId)
			config.Delete(t)
		}
	})
	t.Run("Verify whether pmap is deleted or not", func(t *testing.T) {
		config := dut.Config().Qos().Classifier(*baseConfigClassifier.Name)
		if qs := config.Lookup(t); qs.IsPresent() == true {
			t.Errorf("pmap is not deleted: got %v", qs)
		}
	})
}

func TestSetDscp(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	baseConfig := setupQos(t, dut)
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
			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := config.Get(t)
					if diff := cmp.Diff(*configGot, *baseConfigClassifierTerm); diff != "" {
						t.Errorf("Config /qos/classifiers/classifier/terms/term/actions/remark/config/set-dscp fail:\n%v", diff)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := state.Get(t)
					if diff := cmp.Diff(*stateGot, *baseConfigClassifierTerm); diff != "" {
						t.Errorf("Config /qos/classifiers/classifier/terms/term/actions/remark/config/set-dscp fail:\n%v", diff)
					}
				})
			}
		})
	}
}
func TestSetMplsTc(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	baseConfig := setupQos(t, dut)
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
						t.Errorf("Config /qos/classifiers/classifier/terms/term/actions/remark/config/set-mpls-tc fail:\n%v", diff)
					}
				})
			}
		})
	}
}
