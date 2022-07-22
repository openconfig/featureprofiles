package qos_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/feature/cisco/qos/setup"
	"github.com/openconfig/featureprofiles/topologies/binding"
	"github.com/openconfig/ondatra"
	oc "github.com/openconfig/ondatra/telemetry"
)

func TestMain(m *testing.M) {
	ondatra.RunTests(m, binding.New)
}

// Replace on dut.Config().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Ipv4()
// is appending the new dscp value to existing one
func TestDscpAtContainer(t *testing.T) {
	t.Skip()
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQos(t, dut, "base_config_classifier_term_ipv4.json")
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testDscpInput {
		t.Run(fmt.Sprintf("Testing /qos/classifiers/classifier/terms/term/conditions/ipv4/config/dscp using value %v", input), func(t *testing.T) {
			baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)
			baseConfigClassifierTerm := setup.GetAnyValue(baseConfigClassifier.Term)
			baseConfigClassifierTermConditions := baseConfigClassifierTerm.Conditions
			baseConfigClassifierTermConditionsIpv4 := baseConfigClassifierTermConditions.Ipv4
			*baseConfigClassifierTermConditionsIpv4.Dscp = input

			config := dut.Config().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Ipv4()
			state := dut.Telemetry().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Ipv4()

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigClassifierTermConditionsIpv4)
			})
			// 	telemetry.Qos_Classifier_Term_Conditions_Ipv4{
			//         DestinationAddress: nil,
			// -       Dscp:               nil,
			// +       Dscp:               &63,
			// -       DscpSet:            []uint8{0x01, 0x3f},
			// +       DscpSet:            nil,
			//         HopLimit:           nil,
			//         Protocol:           nil,
			//         SourceAddress:      nil,
			//   }
			t.Run("Get container", func(t *testing.T) {
				configGot := config.Get(t)
				if diff := cmp.Diff(*configGot, *baseConfigClassifierTermConditionsIpv4); diff != "" {
					t.Errorf("Config /qos/classifiers/classifier/terms/term/conditions/ipv4/config/dscp: %v", diff)
				}
			})
			// No sysdb paths found for yang path qos/classifiers/classifier/terms/term/conditions/ipv4\x00"} (*gnmi.SubscribeResponse_Error)
			if setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					if diff := cmp.Diff(*stateGot, *baseConfigClassifierTermConditionsIpv4); diff != "" {
						t.Errorf("State /qos/classifiers/classifier/terms/term/conditions/ipv4/config/dscp: %v", diff)
					}
				})
			}
			// Delete gives no error but nothing is getting deleted actually. qs.Val(t).Dscp = nil as dscp is nil (error)
			t.Run("Delete container", func(t *testing.T) {
				config.Delete(t)
				if setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).Dscp != nil {
						t.Errorf("Delete /qos/classifiers/classifier/terms/term/conditions/ipv4/config/dscp fail: got %v", qs)
					}
				}
			})
		})
	}
}

func TestDscpAtLeaf(t *testing.T) {
	t.Skip()
	dut := ondatra.DUT(t, "dut")
	dut.Config().Qos().Delete(t)
	var baseConfig *oc.Qos = setupQos(t, dut, "base_config_classifier_term_ipv4.json")
	// defer teardownQos(t, dut, baseConfig)

	for _, input := range testDscpInput {
		t.Run(fmt.Sprintf("Testing /qos/classifiers/classifier/terms/term/conditions/ipv4/config/dscp using value %v", input), func(t *testing.T) {
			baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)
			baseConfigClassifierTerm := setup.GetAnyValue(baseConfigClassifier.Term)

			config := dut.Config().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Ipv4().Dscp()
			state := dut.Telemetry().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Ipv4().Dscp()

			t.Run("Replace leaf", func(t *testing.T) {
				config.Replace(t, input)
			})
			// dut.Config().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Ipv4().Dscp() doesn't return anything but
			// dut.Config().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Ipv4().DscpSet() returns a list
			t.Run("Get leaf", func(t *testing.T) {
				configGot := config.Get(t)
				if configGot != input {
					t.Errorf("Config /qos/classifiers/classifier/terms/term/conditions/ipv4/config/dscp: got %v, want %v", configGot, input)
				}
			})
			// ERR:No sysdb paths found for yang path qos/classifiers/classifier/terms/term/conditions/ipv4/state/dscp\x00"} (*gnmi.SubscribeResponse_Error)
			if setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot != input {
						t.Errorf("State /qos/classifiers/classifier/terms/term/conditions/ipv4/config/dscp: got %v, want %v", stateGot, input)
					}
				})
			}
			// Delete gives no error but nothing is getting deleted
			t.Run("Delete leaf", func(t *testing.T) {

				config.Delete(t)
				if setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs != nil {
						t.Errorf("Delete /qos/classifiers/classifier/terms/term/conditions/ipv4/config/dscp fail: got %v", qs)
					}
				}
			})
		})
	}
}

// config.Replace(t, baseConfigClassifierTermConditionsIpv4) -> appends to existing list rather than replace
func TestDscpSetAtContainer(t *testing.T) {
	t.Skip()
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQos(t, dut, "base_config_classifier_term_ipv4.json")
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testDscpSetInput {
		t.Run(fmt.Sprintf("Testing /qos/classifiers/classifier/terms/term/conditions/ipv4/config/dscp-set using value %v", input), func(t *testing.T) {
			baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)
			baseConfigClassifierTerm := setup.GetAnyValue(baseConfigClassifier.Term)
			baseConfigClassifierTermConditions := baseConfigClassifierTerm.Conditions
			baseConfigClassifierTermConditionsIpv4 := baseConfigClassifierTermConditions.Ipv4
			baseConfigClassifierTermConditionsIpv4.DscpSet = input

			config := dut.Config().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Ipv4()
			state := dut.Telemetry().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Ipv4()

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigClassifierTermConditionsIpv4)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					for i, cg := range configGot.DscpSet {
						if cg != input[i] {
							t.Errorf("Config /qos/classifiers/classifier/terms/term/conditions/ipv4/config/dscp-set: got %v, want %v", cg, input[i])
						}
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					for i, sg := range stateGot.DscpSet {
						if sg != input[i] {
							t.Errorf("Config /qos/classifiers/classifier/terms/term/conditions/ipv4/config/dscp-set: got %v, want %v", sg, input[i])
						}
					}
				})
			}
			t.Run("Delete container", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).DscpSet != nil {
						t.Errorf("Delete /qos/classifiers/classifier/terms/term/conditions/ipv4/config/dscp-set fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestDscpSetAtLeaf(t *testing.T) {
	// t.Skip()
	dut := ondatra.DUT(t, "dut")
	var baseConfig *oc.Qos = setupQos(t, dut, "base_config_classifier_term_ipv4.json")
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testDscpSetInput {
		t.Run(fmt.Sprintf("Testing /qos/classifiers/classifier/terms/term/conditions/ipv4/config/dscp-set using value %v", input), func(t *testing.T) {
			baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)
			baseConfigClassifierTerm := setup.GetAnyValue(baseConfigClassifier.Term)

			config := dut.Config().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Ipv4().DscpSet()
			state := dut.Telemetry().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Ipv4().DscpSet()

			t.Run("Replace leaf", func(t *testing.T) {
				config.Replace(t, input)
			})
			t.Run("Get leaf", func(t *testing.T) {
				configGot := config.Get(t)
				for i, cg := range configGot {
					if cg != input[i] {
						t.Errorf("Config /qos/classifiers/classifier/terms/term/conditions/ipv4/config/dscp-set: got %v, want %v", cg, input[i])
					}
				}
			})
			// ERR:No sysdb paths found for yang path qos/classifiers/classifier/terms/term/conditions/ipv4/state/dscp-set\x00"} (*gnmi.SubscribeResponse_Error)
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := state.Get(t)
					for i, sg := range stateGot {
						if sg != input[i] {
							t.Errorf("Config /qos/classifiers/classifier/terms/term/conditions/ipv4/config/dscp-set: got %v, want %v", sg, input[i])
						}
					}
				})
			}
			// Delete doesn't delete anything
			t.Run("Delete leaf", func(t *testing.T) {
				config.Delete(t)
				time.Sleep(1 * time.Minute)
				if setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs != nil {
						t.Errorf("Delete /qos/classifiers/classifier/terms/term/conditions/ipv4/config/dscp-set fail: got %v", qs)
					}
				}
			})
		})
	}
}
