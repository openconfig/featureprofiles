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
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	ondatra.RunTests(m, binding.New)
}

// Same goes for IPV6

func TestDscpAtContainer(t *testing.T) {
	t.Skip()
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQos(t, dut, "base_config_classifier_term_ipv6.json")
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testDscpInput {
		t.Run(fmt.Sprintf("Testing /qos/classifiers/classifier/terms/term/conditions/ipv6/config/dscp using value %v", input), func(t *testing.T) {
			baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)
			baseConfigClassifierTerm := setup.GetAnyValue(baseConfigClassifier.Term)
			baseConfigClassifierTermConditions := baseConfigClassifierTerm.Conditions
			baseConfigClassifierTermConditionsIpv6 := baseConfigClassifierTermConditions.Ipv6
			if baseConfigClassifierTermConditionsIpv6.Dscp == nil {
				baseConfigClassifierTermConditionsIpv6.Dscp = ygot.Uint8(baseConfigClassifierTermConditionsIpv6.DscpSet[0])
				baseConfigClassifierTermConditionsIpv6.DscpSet = nil
			}
			*baseConfigClassifierTermConditionsIpv6.Dscp = input

			config := gnmi.OC().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Ipv6()
			state := gnmi.OC().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Ipv6()

			t.Run("Replace container", func(t *testing.T) {
				gnmi.Replace(t, dut, config.Config(), baseConfigClassifierTermConditionsIpv6)
			})
			t.Run("Get container", func(t *testing.T) {
				configGot := gnmi.Get(t, dut, config.Config())
				if diff := cmp.Diff(*configGot, *baseConfigClassifierTermConditionsIpv6); diff != "" {
					t.Errorf("Config /qos/classifiers/classifier/terms/term/conditions/ipv6/config/dscp: %v", diff)
				}
			})
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := gnmi.Get(t, dut, state.State())
					if *stateGot.Dscp != input {
						t.Errorf("State /qos/classifiers/classifier/terms/term/conditions/ipv6/config/dscp: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete container", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				if !setup.SkipSubscribe() {
					if qs, _ := gnmi.LookupConfig(t, dut, config.Config()).Val(); qs.Dscp != nil {
						t.Errorf("Delete /qos/classifiers/classifier/terms/term/conditions/ipv6/config/dscp fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestDscpAtLeaf(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	var baseConfig *oc.Qos = setupQos(t, dut, "base_config_classifier_term_ipv6.json")
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testDscpInput {
		t.Run(fmt.Sprintf("Testing /qos/classifiers/classifier/terms/term/conditions/ipv6/config/dscp using value %v", input), func(t *testing.T) {
			baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)
			baseConfigClassifierTerm := setup.GetAnyValue(baseConfigClassifier.Term)
			baseConfigClassifierTermConditions := baseConfigClassifierTerm.Conditions
			baseConfigClassifierTermConditionsIpv6 := baseConfigClassifierTermConditions.Ipv6
			if baseConfigClassifierTermConditionsIpv6.Dscp == nil {
				baseConfigClassifierTermConditionsIpv6.Dscp = ygot.Uint8(baseConfigClassifierTermConditionsIpv6.DscpSet[0])
				baseConfigClassifierTermConditionsIpv6.DscpSet = nil
			}

			config := gnmi.OC().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Ipv6().Dscp()
			state := gnmi.OC().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Ipv6().Dscp()

			t.Run("Replace leaf", func(t *testing.T) {
				gnmi.Replace(t, dut, config.Config(), input)
			})
			t.Run("Get leaf", func(t *testing.T) {
				t.Skip()
				configGot := gnmi.Get(t, dut, config.Config())
				if configGot != input {
					t.Errorf("Config /qos/classifiers/classifier/terms/term/conditions/ipv6/config/dscp: got %v, want %v", configGot, input)
				}
			})
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := gnmi.Get(t, dut, state.State())
					if stateGot != input {
						t.Errorf("State /qos/classifiers/classifier/terms/term/conditions/ipv6/state/dscp: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete leaf", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				if !setup.SkipSubscribe() {
					if qs := gnmi.LookupConfig(t, dut, config.Config()); qs != nil {
						t.Errorf("Delete /qos/classifiers/classifier/terms/term/conditions/ipv6/config/dscp fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestDscpSetAtContainer(t *testing.T) {
	t.Skip()
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQos(t, dut, "base_config_classifier_term_ipv6.json")
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testDscpSetInput {
		t.Run(fmt.Sprintf("Testing /qos/classifiers/classifier/terms/term/conditions/ipv6/config/dscp-set using value %v", input), func(t *testing.T) {
			baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)
			baseConfigClassifierTerm := setup.GetAnyValue(baseConfigClassifier.Term)
			baseConfigClassifierTermConditions := baseConfigClassifierTerm.Conditions
			baseConfigClassifierTermConditionsIpv6 := baseConfigClassifierTermConditions.Ipv6
			if baseConfigClassifierTermConditionsIpv6.DscpSet == nil {
				baseConfigClassifierTermConditionsIpv6.DscpSet = []uint8{}
				baseConfigClassifierTermConditionsIpv6.Dscp = nil
			}
			baseConfigClassifierTermConditionsIpv6.DscpSet = input

			config := gnmi.OC().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Ipv6()
			state := gnmi.OC().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Ipv6()

			t.Run("Replace container", func(t *testing.T) {
				gnmi.Replace(t, dut, config.Config(), baseConfigClassifierTermConditionsIpv6)
			})
			t.Run("Get container", func(t *testing.T) {
				configGot := gnmi.Get(t, dut, config.Config())
				if diff := cmp.Diff(*configGot, *baseConfigClassifierTermConditionsIpv6); diff != "" {
					t.Errorf("Config /qos/classifiers/classifier/terms/term/conditions/ipv6/config/dscp-set: %v", diff)
				}
			})
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := gnmi.Get(t, dut, state.State())
					for i, sg := range stateGot.DscpSet {
						if sg != input[i] {
							t.Errorf("Config /qos/classifiers/classifier/terms/term/conditions/ipv6/config/dscp-set: got %v, want %v", sg, input[i])
						}
					}
				})
			}
			t.Run("Delete container", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				if !setup.SkipSubscribe() {
					if qs, _ := gnmi.LookupConfig(t, dut, config.Config()).Val(); qs.DscpSet != nil {
						t.Errorf("Delete /qos/classifiers/classifier/terms/term/conditions/ipv6/config/dscp-set fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestDscpSetAtLeaf(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	var baseConfig *oc.Qos = setupQos(t, dut, "base_config_classifier_term_ipv6.json")
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testDscpSetInput {
		t.Run(fmt.Sprintf("Testing /qos/classifiers/classifier/terms/term/conditions/ipv6/config/dscp-set using value %v", input), func(t *testing.T) {
			baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)
			baseConfigClassifierTerm := setup.GetAnyValue(baseConfigClassifier.Term)
			baseConfigClassifierTermConditions := baseConfigClassifierTerm.Conditions
			baseConfigClassifierTermConditionsIpv6 := baseConfigClassifierTermConditions.Ipv6
			if baseConfigClassifierTermConditionsIpv6.DscpSet == nil {
				baseConfigClassifierTermConditionsIpv6.DscpSet = []uint8{}
				baseConfigClassifierTermConditionsIpv6.Dscp = nil
			}
			config := gnmi.OC().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Ipv6().DscpSet()
			state := gnmi.OC().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Ipv6().DscpSet()

			t.Run("Replace leaf", func(t *testing.T) {
				gnmi.Replace(t, dut, config.Config(), input)
			})
			t.Run("Get leaf", func(t *testing.T) {
				configGot := gnmi.Get(t, dut, config.Config())
				for i, cg := range configGot {
					if cg != input[i] {
						t.Errorf("Config /qos/classifiers/classifier/terms/term/conditions/ipv6/config/dscp-set: got %v, want %v", cg, input[i])
					}
				}
			})
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := gnmi.Get(t, dut, state.State())
					for i, sg := range stateGot {
						if sg != input[i] {
							t.Errorf("Config /qos/classifiers/classifier/terms/term/conditions/ipv6/state/dscp-set: got %v, want %v", sg, input[i])
						}
					}
				})
			}
			t.Run("Delete leaf", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				if !setup.SkipSubscribe() {
					if qs := gnmi.LookupConfig(t, dut, config.Config()); qs != nil {
						t.Errorf("Delete /qos/classifiers/classifier/terms/term/conditions/ipv6/config/dscp-set fail: got %v", qs)
					}
				}
			})
		})
	}
}
