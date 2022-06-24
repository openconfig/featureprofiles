package qos_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/feature/cisco/qos/setup"
	"github.com/openconfig/featureprofiles/topologies/binding"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/testt"
)

func TestMain(m *testing.M) {
	ondatra.RunTests(m, binding.New)
}

func TestInterface(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	baseConfig := setup.BaseConfig()
	setup.ResetStruct(baseConfig, []string{"Classifier", "Interface"})

	baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)
	dut.Config().Qos().Classifier(*baseConfigClassifier.Name).Update(t, baseConfigClassifier)

	defer teardownQos(t, dut, baseConfig)

	baseConfigInterface := setup.GetAnyValue(baseConfig.Interface)

	config := dut.Config().Qos().Interface(*baseConfigInterface.InterfaceId)
	state := dut.Telemetry().Qos().Interface(*baseConfigInterface.InterfaceId)

	t.Run("Replace container", func(t *testing.T) {
		config.Update(t, baseConfigInterface)
		// config.Replace(t, baseConfigInterface)
	})
	if !setup.SkipGet() {
		t.Run("Get Interface Config", func(t *testing.T) {
			configGot := config.Get(t)
			if diff := cmp.Diff(*configGot, *baseConfigInterface); diff != "" {
				t.Errorf("Config Interface fail: \n%v", diff)
			}
		})
	}
	if !setup.SkipSubscribe() {
		t.Run("Get Interface Telemetry", func(t *testing.T) {
			stateGot := state.Get(t)
			if diff := cmp.Diff(*stateGot, *baseConfigInterface); diff != "" {
				t.Errorf("Telemetry Interface fail: \n%v", diff)
			}
		})
	}
}

func TestDeletePmap(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)

	t.Run("Delete a Policy-map attached to an interface", func(t *testing.T) {
		if got := testt.ExpectFatal(t, func(t testing.TB) {
			config := dut.Config().Qos().Classifier(*baseConfigClassifier.Name)
			config.Delete(t)
		}); got == "" {
			t.Errorf("Delete did not fail fatally as expected")
		}
	})
}

func TestDeleteLastCmap(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)

	termIds := func() []string {
		var keys []string
		for k, _ := range baseConfigClassifier.Term {
			keys = append(keys, k)
		}
		return keys
	}()
	for _, termId := range termIds[1:] {
		config := dut.Config().Qos().Classifier(*baseConfigClassifier.Name).Term(termId)
		config.Delete(t)
	}
	t.Run("Delete the last class-map from pmap attached to interface", func(t *testing.T) {
		if got := testt.ExpectFatal(t, func(t testing.TB) {
			config := dut.Config().Qos().Classifier(*baseConfigClassifier.Name).Term(termIds[0])
			config.Delete(t)
		}); got == "" {
			t.Errorf("Delete did not fail fatally as expected")
		}
	})
}
