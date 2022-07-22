package qos_test

import (
	"fmt"
	"testing"

	"github.com/openconfig/featureprofiles/feature/cisco/qos/setup"
	"github.com/openconfig/featureprofiles/topologies/binding"
	"github.com/openconfig/ondatra"
	oc "github.com/openconfig/ondatra/telemetry"
)

func TestMain(m *testing.M) {
	ondatra.RunTests(m, binding.New)
}

func TestDscpAtContainer(t *testing.T) {
	// t.Skip()
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQos(t, dut, "base_config_classifier_term.json")
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testDscpInput {
		t.Run(fmt.Sprintf("Testing /qos/classifiers/classifier/terms/term/conditions/ipv6/config/dscp using value %v", input), func(t *testing.T) {
			baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)
			baseConfigClassifierTerm := setup.GetAnyValue(baseConfigClassifier.Term)
			baseConfigClassifierTermConditions := baseConfigClassifierTerm.Conditions
			baseConfigClassifierTermConditionsIpv6 := baseConfigClassifierTermConditions.Ipv6
			*baseConfigClassifierTermConditionsIpv6.Dscp = input

			config := dut.Config().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Ipv6()
			state := dut.Telemetry().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Ipv6()

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigClassifierTermConditionsIpv6)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.Dscp != input {
						t.Errorf("Config /qos/classifiers/classifier/terms/term/conditions/ipv6/config/dscp: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.Dscp != input {
						t.Errorf("State /qos/classifiers/classifier/terms/term/conditions/ipv6/config/dscp: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete container", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).Dscp != nil {
						t.Errorf("Delete /qos/classifiers/classifier/terms/term/conditions/ipv6/config/dscp fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestDscpAtLeaf(t *testing.T) {
	t.Skip()
	dut := ondatra.DUT(t, "dut")
	var baseConfig *oc.Qos = setupQos(t, dut, "base_config_classifier_term.json")
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testDscpInput {
		t.Run(fmt.Sprintf("Testing /qos/classifiers/classifier/terms/term/conditions/ipv6/config/dscp using value %v", input), func(t *testing.T) {
			baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)
			baseConfigClassifierTerm := setup.GetAnyValue(baseConfigClassifier.Term)

			config := dut.Config().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Ipv6().Dscp()
			state := dut.Telemetry().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Ipv6().Dscp()

			t.Run("Replace leaf", func(t *testing.T) {
				config.Replace(t, input)
			})
			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot != input {
						t.Errorf("Config /qos/classifiers/classifier/terms/term/conditions/ipv6/config/dscp: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot != input {
						t.Errorf("State /qos/classifiers/classifier/terms/term/conditions/ipv6/config/dscp: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete leaf", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs != nil {
						t.Errorf("Delete /qos/classifiers/classifier/terms/term/conditions/ipv6/config/dscp fail: got %v", qs)
					}
				}
			})
		})
	}
}

// Replace on dut.Config().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Ipv6()
// is appending the new dscp value to existing one
func TestDscpSetAtContainer(t *testing.T) {
	t.Skip()
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQos(t, dut, "base_config_classifier_term.json")
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testDscpSetInput {
		t.Run(fmt.Sprintf("Testing /qos/classifiers/classifier/terms/term/conditions/ipv6/config/dscp-set using value %v", input), func(t *testing.T) {
			baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)
			baseConfigClassifierTerm := setup.GetAnyValue(baseConfigClassifier.Term)
			baseConfigClassifierTermConditions := baseConfigClassifierTerm.Conditions
			baseConfigClassifierTermConditionsIpv6 := baseConfigClassifierTermConditions.Ipv6
			baseConfigClassifierTermConditionsIpv6.DscpSet = input

			config := dut.Config().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Ipv6()
			state := dut.Telemetry().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Ipv6()

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigClassifierTermConditionsIpv6)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					for i, cg := range configGot.DscpSet {
						if cg != input[i] {
							t.Errorf("Config /qos/classifiers/classifier/terms/term/conditions/ipv6/config/dscp-set: got %v, want %v", cg, input[i])
						}
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					for i, sg := range stateGot.DscpSet {
						if sg != input[i] {
							t.Errorf("Config /qos/classifiers/classifier/terms/term/conditions/ipv6/config/dscp-set: got %v, want %v", sg, input[i])
						}
					}
				})
			}
			t.Run("Delete container", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).DscpSet != nil {
						t.Errorf("Delete /qos/classifiers/classifier/terms/term/conditions/ipv6/config/dscp-set fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestDscpSetAtLeaf(t *testing.T) {
	t.Skip()
	dut := ondatra.DUT(t, "dut")
	var baseConfig *oc.Qos = setupQos(t, dut, "base_config_classifier_term.json")
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testDscpSetInput {
		t.Run(fmt.Sprintf("Testing /qos/classifiers/classifier/terms/term/conditions/ipv6/config/dscp-set using value %v", input), func(t *testing.T) {
			baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)
			baseConfigClassifierTerm := setup.GetAnyValue(baseConfigClassifier.Term)

			config := dut.Config().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Ipv6().DscpSet()
			state := dut.Telemetry().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Ipv6().DscpSet()

			t.Run("Replace leaf", func(t *testing.T) {
				config.Replace(t, input)
			})
			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := config.Get(t)
					for i, cg := range configGot {
						if cg != input[i] {
							t.Errorf("Config /qos/classifiers/classifier/terms/term/conditions/ipv6/config/dscp-set: got %v, want %v", cg, input[i])
						}
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := state.Get(t)
					for i, sg := range stateGot {
						if sg != input[i] {
							t.Errorf("Config /qos/classifiers/classifier/terms/term/conditions/ipv6/config/dscp-set: got %v, want %v", sg, input[i])
						}
					}
				})
			}
			t.Run("Delete leaf", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs != nil {
						t.Errorf("Delete /qos/classifiers/classifier/terms/term/conditions/ipv6/config/dscp-set fail: got %v", qs)
					}
				}
			})
		})
	}
}
