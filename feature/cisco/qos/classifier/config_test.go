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
)

func TestMain(m *testing.M) {
	ondatra.RunTests(m, binding.New)
}

func TestNameAtContainer(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	var baseConfig *oc.Qos = setupQos(t, dut, "base_config_classifier.json")
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testNameInput {
		t.Run(fmt.Sprintf("Testing /qos/classifiers/classifier/config/name using value %v", input), func(t *testing.T) {
			baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)
			*baseConfigClassifier.Name = input

			config := gnmi.OC().Qos().Classifier(*baseConfigClassifier.Name)
			state := gnmi.OC().Qos().Classifier(*baseConfigClassifier.Name)

			t.Run("Replace container", func(t *testing.T) {
				gnmi.Update(t, dut, config.Config(), baseConfigClassifier)
				// config.Replace(t, baseConfigClassifier)
			})
			// dscp and dscp-set causing error
			t.Run("Get container", func(t *testing.T) {
				t.Skip()
				configGot := gnmi.Get(t, dut, config.Config())
				if diff := cmp.Diff(*configGot, *baseConfigClassifier); diff != "" {
					t.Errorf("Config /qos/classifiers/classifier/config/name: %v", diff)
				}
			})
			// ERR:No sysdb paths found for yang path qos/classifiers/classifier\x00"} (*gnmi.SubscribeResponse_Error)
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := gnmi.Get(t, dut, state.State())
					if diff := cmp.Diff(*stateGot, *baseConfigClassifier); diff != "" {
						t.Errorf("State /qos/classifiers/classifier/config/name: %v", diff)
					}
				})
			}
			t.Run("Delete container", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
					gnmi.Get(t, dut, config.Config()) //catch the error  as it is expected and absorb the panic.
				}); errMsg != nil {
					t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
				} else {
					t.Errorf("This update should have failed ")
				}

			})
		})
	}
}

func TestNameAtLeaf(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	var baseConfig *oc.Qos = setupQos(t, dut, "base_config_classifier.json")
	defer teardownQos(t, dut, baseConfig)

	t.Run("Testing /qos/classifiers/classifier/config/name", func(t *testing.T) {
		baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)

		config := gnmi.OC().Qos().Classifier(*baseConfigClassifier.Name).Name()
		state := gnmi.OC().Qos().Classifier(*baseConfigClassifier.Name).Name()

		t.Run("Get container", func(t *testing.T) {
			configGot := gnmi.Get(t, dut, config.Config())
			if configGot != *baseConfigClassifier.Name {
				t.Errorf("Config /qos/classifiers/classifier/config/name: want %s got %s", *baseConfigClassifier.Name, configGot)
			}
		})
		// ERR:No sysdb paths found for yang path qos/classifiers/classifier\x00"} (*gnmi.SubscribeResponse_Error)
		if !setup.SkipSubscribe() {
			t.Run("Subscribe container", func(t *testing.T) {
				stateGot := gnmi.Get(t, dut, state.State())
				if stateGot != *baseConfigClassifier.Name {
					t.Errorf("Config /qos/classifiers/classifier/state/name: want %s got %s", *baseConfigClassifier.Name, stateGot)
				}
			})
		}
	})
}

// XR doesn't use classifier/type - CSCwc13851
// Config Get/Lookup fails
// error receiving gNMI response: invalid nil Val in update:
// path:{origin:"openconfig" elem:{name:"qos"} elem:{name:"classifiers"} elem:{name:"classifier" key:{key:"name" value:"pmap"}}}
func TestTypeAtLeaf(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	var baseConfig *oc.Qos = setupQos(t, dut, "base_config_classifier.json")
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testTypeInput {
		t.Run(fmt.Sprintf("Testing /qos/classifiers/classifier/config/type using value %v", input), func(t *testing.T) {
			t.Skip()
			baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)

			config := gnmi.OC().Qos().Classifier(*baseConfigClassifier.Name).Type()
			state := gnmi.OC().Qos().Classifier(*baseConfigClassifier.Name).Type()

			t.Run("Replace leaf", func(t *testing.T) {
				gnmi.Replace(t, dut, config.Config(), input)
			})
			t.Run("Get leaf", func(t *testing.T) {
				configGot := gnmi.Get(t, dut, config.Config())
				if configGot != input {
					t.Errorf("Config /qos/classifiers/classifier/config/type: got %v, want %v", configGot, input)
				}
			})
			// ERR:No sysdb paths found for yang path qos/classifiers/classifier/state/type\x00"} (*gnmi.SubscribeResponse_Error)
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := gnmi.Get(t, dut, state.State())
					if stateGot != input {
						t.Errorf("State /qos/classifiers/classifier/state/type: got %v, want %v", stateGot, input)
					}
				})
			}
		})
	}
}
