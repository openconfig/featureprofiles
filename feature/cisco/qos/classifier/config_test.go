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

// Get fails as XR doesn't return type
// CSCwc13851
func TestTypeAtContainer(t *testing.T) {
	t.Skip()
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQos(t, dut, "base_config_classifier.json")
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testTypeInput {
		t.Run(fmt.Sprintf("Testing /qos/classifiers/classifier/config/type using value %v", input), func(t *testing.T) {
			baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)
			baseConfigClassifier.Type = input

			config := dut.Config().Qos().Classifier(*baseConfigClassifier.Name)
			state := dut.Telemetry().Qos().Classifier(*baseConfigClassifier.Name)

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigClassifier)
			})
			t.Run("Get container", func(t *testing.T) {
				configGot := config.Get(t)
				if diff := cmp.Diff(*configGot, *baseConfigClassifier); diff != "" {
					t.Errorf("Config /qos/classifiers/classifier/config/type: %v", diff)
				}
			})
			// Get(t) at path origin:"openconfig" elem:{name:"qos"} elem:{name:"classifiers"} elem:{name:"classifier" key:{key:"name" value:"pmap"}}:
			// error receiving gNMI response: unexpected response: &{code:3 message:"openconfig:qos/classifiers/classifier[name=pmap]:
			// ERR:No sysdb paths found for yang path qos/classifiers/classifier\x00"} (*gnmi.SubscribeResponse_Error)
			// Not valid for config paths
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					if diff := cmp.Diff(*stateGot, *baseConfigClassifier); diff != "" {
						t.Errorf("State /qos/classifiers/classifier/config/type: %v", diff)
					}
				})
			}
		})
	}
}

// Config Get/Lookup fails
// Get(t) at path origin:"openconfig" elem:{name:"qos"} elem:{name:"classifiers"} elem:{name:"classifier" key:{key:"name" value:"pmap"}} elem:{name:"config"} elem:{name:"type"}:
// error receiving gNMI response: invalid nil Val in update:
// path:{origin:"openconfig" elem:{name:"qos"} elem:{name:"classifiers"} elem:{name:"classifier" key:{key:"name" value:"pmap"}}}
// CSCwc13851
func TestTypeAtLeaf(t *testing.T) {
	t.Skip()
	dut := ondatra.DUT(t, "dut")
	var baseConfig *oc.Qos = setupQos(t, dut, "base_config_classifier.json")
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testTypeInput {
		t.Run(fmt.Sprintf("Testing /qos/classifiers/classifier/config/type using value %v", input), func(t *testing.T) {
			baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)

			config := dut.Config().Qos().Classifier(*baseConfigClassifier.Name).Type()
			state := dut.Telemetry().Qos().Classifier(*baseConfigClassifier.Name).Type()

			t.Run("Replace leaf", func(t *testing.T) {
				config.Replace(t, input)
			})
			t.Run("Get leaf", func(t *testing.T) {
				configGot := config.Get(t)
				if configGot != input {
					t.Errorf("Config /qos/classifiers/classifier/config/type: got %v, want %v", configGot, input)
				}
			})
			// Get(t) at path origin:"openconfig" elem:{name:"qos"} elem:{name:"classifiers"} elem:{name:"classifier" key:{key:"name" value:"pmap"}} elem:{name:"state"} elem:{name:"type"}:
			// error receiving gNMI response: unexpected response: &{code:3 message:"openconfig:qos/classifiers/classifier[name=pmap]/state/type:
			// ERR:No sysdb paths found for yang path qos/classifiers/classifier/state/type\x00"} (*gnmi.SubscribeResponse_Error)
			t.Run("Subscribe leaf", func(t *testing.T) {
				stateGot := state.Get(t)
				if stateGot != input {
					t.Errorf("State /qos/classifiers/classifier/config/type: got %v, want %v", stateGot, input)
				}
			})
			t.Run("Delete leaf", func(t *testing.T) {
				config.Delete(t)
				if qs := config.Lookup(t); qs != nil {
					t.Errorf("Delete /qos/classifiers/classifier/config/type fail: got %v", qs)
				}
			})
		})
	}
}
func TestNameAtContainer(t *testing.T) {
	t.Skip()
	dut := ondatra.DUT(t, "dut")
	var baseConfig *oc.Qos = setupQos(t, dut, "base_config_classifier.json")
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testNameInput {
		t.Run(fmt.Sprintf("Testing /qos/classifiers/classifier/config/name using value %v", input), func(t *testing.T) {
			baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)
			*baseConfigClassifier.Name = input

			config := dut.Config().Qos().Classifier(*baseConfigClassifier.Name)
			state := dut.Telemetry().Qos().Classifier(*baseConfigClassifier.Name)

			t.Run("Replace container", func(t *testing.T) {
				config.Update(t, baseConfigClassifier)
				// config.Replace(t, baseConfigClassifier)
			})
			t.Run("Get container", func(t *testing.T) {
				configGot := config.Get(t)
				if diff := cmp.Diff(*configGot, *baseConfigClassifier); diff != "" {
					t.Errorf("Config /qos/classifiers/classifier/config/name: %v", diff)
				}
			})
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					if diff := cmp.Diff(*stateGot, *baseConfigClassifier); diff != "" {
						t.Errorf("State /qos/classifiers/classifier/config/name: %v", diff)
					}
				})
			}
			t.Run("Delete container", func(t *testing.T) {
				config.Delete(t)
				if qs := config.Lookup(t); qs != nil {
					t.Errorf("Delete /qos/classifiers/classifier/config/name: got %v", qs)
				}
			})
		})
	}
}
