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

func TestTrafficClassAtContainer(t *testing.T) {
	t.Skip()
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQos(t, dut, "base_config_classifier_term_mpls.json")
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testTrafficClassInput {
		t.Run(fmt.Sprintf("Testing /qos/classifiers/classifier/terms/term/conditions/mpls/config/traffic-class using value %v", input), func(t *testing.T) {
			baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)
			baseConfigClassifierTerm := setup.GetAnyValue(baseConfigClassifier.Term)
			baseConfigClassifierTermConditions := baseConfigClassifierTerm.Conditions
			baseConfigClassifierTermConditionsMpls := baseConfigClassifierTermConditions.Mpls
			*baseConfigClassifierTermConditionsMpls.TrafficClass = input

			config := gnmi.OC().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Mpls()
			state := gnmi.OC().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Mpls()

			// Replace is appending to existing traffic-class 5 -> 5 7
			t.Run("Replace container", func(t *testing.T) {
				gnmi.Replace(t, dut, config.Config(), baseConfigClassifierTermConditionsMpls)
			})
			// Get returns the first element -> 5
			t.Run("Get container", func(t *testing.T) {
				configGot := gnmi.Get(t, dut, config.Config())
				if diff := cmp.Diff(*configGot, *baseConfigClassifierTermConditionsMpls); diff != "" {
					t.Errorf("Config /qos/classifiers/classifier/terms/term/conditions/mpls/config/traffic-class: %v", diff)
				}
			})
			// ERR:No sysdb paths found for yang path qos/classifiers/classifier/terms/term/conditions/mpls\x00"} (*gnmi.SubscribeResponse_Error)
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := gnmi.Get(t, dut, state.State())
					if diff := cmp.Diff(*stateGot, *baseConfigClassifierTermConditionsMpls); diff != "" {
						t.Errorf("State /qos/classifiers/classifier/terms/term/conditions/mpls/config/traffic-class: %v", diff)
					}
				})
			}
			// Delete request goes through fine but nothing is getting deleted actually.
			t.Run("Delete container", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				if !setup.SkipSubscribe() {
					if qs, _ := gnmi.LookupConfig(t, dut, config.Config()).Val(); qs.TrafficClass != nil {
						t.Errorf("Delete /qos/classifiers/classifier/terms/term/conditions/mpls/config/traffic-class fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestTrafficClassAtLeaf(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	var baseConfig *oc.Qos = setupQos(t, dut, "base_config_classifier_term_mpls.json")
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testTrafficClassInput {
		t.Run(fmt.Sprintf("Testing /qos/classifiers/classifier/terms/term/conditions/mpls/config/traffic-class using value %v", input), func(t *testing.T) {
			baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)
			baseConfigClassifierTerm := baseConfigClassifier.Term["cmap_mpls"]

			config := gnmi.OC().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Mpls().TrafficClass()
			state := gnmi.OC().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Mpls().TrafficClass()

			t.Run("Replace leaf", func(t *testing.T) {
				gnmi.Replace(t, dut, config.Config(), input)
			})
			t.Run("Get leaf", func(t *testing.T) {
				configGot := gnmi.Get(t, dut, config.Config())
				if configGot != input {
					t.Errorf("Config /qos/classifiers/classifier/terms/term/conditions/mpls/config/traffic-class: got %v, want %v", configGot, input)
				}
			})
			// ERR:No sysdb paths found for yang path qos/classifiers/classifier/terms/term/conditions/mpls/state/traffic-class\x00"} (*gnmi.SubscribeResponse_Error)
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := gnmi.Get(t, dut, state.State())
					if stateGot != input {
						t.Errorf("State /qos/classifiers/classifier/terms/term/conditions/mpls/state/traffic-class: got %v, want %v", stateGot, input)
					}
				})
			}
			// Delete gives no error but nothing is getting deleted actually
			t.Run("Delete leaf", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				if !setup.SkipSubscribe() {
					if qs := gnmi.LookupConfig(t, dut, config.Config()); qs != nil {
						t.Errorf("Delete /qos/classifiers/classifier/terms/term/conditions/mpls/config/traffic-class fail: got %v", qs)
					}
				}
			})
		})
	}
}
