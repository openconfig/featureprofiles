package qos_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/feature/cisco/qos/setup"
	"github.com/openconfig/featureprofiles/topologies/binding"
	"github.com/openconfig/ondatra"
)

func TestMain(m *testing.M) {
	ondatra.RunTests(m, binding.New)
}

// TestClassifier verifies that the Classifier configuration paths can be read,
// updated, and deleted.
//
// path:/qos/classifiers/classifier
func TestClassifier(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	baseConfig := setup.BaseConfig()
	baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)

	config := dut.Config().Qos().Classifier(*baseConfigClassifier.Name)
	state := dut.Telemetry().Qos().Classifier(*baseConfigClassifier.Name)

	t.Run("Create Classifier", func(t *testing.T) {
		config.Update(t, baseConfigClassifier)
	})
	if !setup.SkipGet() {
		t.Run("Get Classifier Config", func(t *testing.T) {
			configGot := config.Get(t)
			if diff := cmp.Diff(*configGot, *baseConfigClassifier); diff != "" {
				t.Errorf("Config Classifier fail: \n%v", diff)
			}
		})
	}
	if !setup.SkipSubscribe() {
		t.Run("Get Classifier Telemetry", func(t *testing.T) {
			stateGot := state.Lookup(t)
			if diff := cmp.Diff(*stateGot.Val(t), *baseConfigClassifier); diff != "" {
				t.Errorf("Telemetry Classifier fail: \n%v", diff)
			}
		})
	}
	t.Run("Delete Classifier", func(t *testing.T) {
		config.Delete(t)
		if qs := config.Lookup(t); qs.IsPresent() == true {
			t.Errorf("Delete Classifier fail: got %v", qs)
		}
	})
}
