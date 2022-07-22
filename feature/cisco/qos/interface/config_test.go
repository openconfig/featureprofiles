package qos_test

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/feature/cisco/qos/setup"
	"github.com/openconfig/featureprofiles/topologies/binding"
	"github.com/openconfig/ondatra"
	oc "github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/testt"
)

func TestMain(m *testing.M) {
	ondatra.RunTests(m, binding.New)
}

func TestInterfaceIdAtContainer(t *testing.T) {
	t.Skip()
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQos(t, dut, "base_config_interface.json")
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testInterfaceIdInput {
		t.Run(fmt.Sprintf("Testing /qos/interfaces/interface/config/interface-id using value %v", input), func(t *testing.T) {
			baseConfigInterface := setup.GetAnyValue(baseConfig.Interface)
			*baseConfigInterface.InterfaceId = input

			config := dut.Config().Qos().Interface(*baseConfigInterface.InterfaceId)

			t.Run("Replace container", func(t *testing.T) {
				config.Update(t, baseConfigInterface)
				// config.Replace(t, baseConfigInterface)
			})
			t.Run("Get container", func(t *testing.T) {
				configGot := config.Get(t)
				if diff := cmp.Diff(*configGot, *baseConfigInterface); diff != "" {
					t.Errorf("Config /qos/interfaces/interface/config/interface-id: got %v", diff)
				}
			})
		})
	}
}

func TestDeleteClassifier(t *testing.T) {
	t.Skip()
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQos(t, dut, "base_config_interface.json")
	defer teardownQos(t, dut, baseConfig)

	baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)

	t.Run("Delete a Classifier Policy attached to an interface", func(t *testing.T) {
		if got := testt.ExpectFatal(t, func(t testing.TB) {
			config := dut.Config().Qos().Classifier(*baseConfigClassifier.Name)
			config.Delete(t)
		}); got == "" {
			t.Errorf("Delete did not fail fatally as expected")
		}
	})
}

func TestDeleteLastClassMap(t *testing.T) {
	t.Skip()
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQos(t, dut, "base_config_interface.json")
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
