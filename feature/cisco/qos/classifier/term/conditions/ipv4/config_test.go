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

// Replace on dut.Config().Qos().Classifier(<name>).Term(<termId>).Conditions().Ipv4() is appending the new dscp value to existing one.
// dscp & dscp-set is a single entity in XR that points at a list
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
			if baseConfigClassifierTermConditionsIpv4.Dscp == nil {
				baseConfigClassifierTermConditionsIpv4.Dscp = ygot.Uint8(baseConfigClassifierTermConditionsIpv4.DscpSet[0])
				baseConfigClassifierTermConditionsIpv4.DscpSet = nil
			}
			*baseConfigClassifierTermConditionsIpv4.Dscp = input

			config := gnmi.OC().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Ipv4()
			state := gnmi.OC().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Ipv4()

			t.Run("Replace container", func(t *testing.T) {
				gnmi.Replace(t, dut, config.Config(), baseConfigClassifierTermConditionsIpv4)
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
				configGot := gnmi.Get(t, dut, config.Config())
				if diff := cmp.Diff(*configGot, *baseConfigClassifierTermConditionsIpv4); diff != "" {
					t.Errorf("Config /qos/classifiers/classifier/terms/term/conditions/ipv4/config/dscp: %v", diff)
				}
			})
			// No sysdb paths found for yang path qos/classifiers/classifier/terms/term/conditions/ipv4\x00"} (*gnmi.SubscribeResponse_Error)
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := gnmi.Get(t, dut, state.State())
					if diff := cmp.Diff(*stateGot, *baseConfigClassifierTermConditionsIpv4); diff != "" {
						t.Errorf("State /qos/classifiers/classifier/terms/term/conditions/ipv4/config/dscp: %v", diff)
					}
				})
			}
			// Delete request goes through fine but nothing is getting deleted actually.
			t.Run("Delete container", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				if !setup.SkipSubscribe() {
					if qs, _ := gnmi.LookupConfig(t, dut, config.Config()).Val(); qs.Dscp != nil {
						t.Errorf("Delete /qos/classifiers/classifier/terms/term/conditions/ipv4/config/dscp fail: got %v", qs)
					}
				}
			})
		})
	}
}

func TestDscpAtLeaf(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	var baseConfig *oc.Qos = setupQos(t, dut, "base_config_classifier_term_ipv4.json")
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testDscpInput {
		t.Run(fmt.Sprintf("Testing /qos/classifiers/classifier/terms/term/conditions/ipv4/config/dscp using value %v", input), func(t *testing.T) {
			baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)
			baseConfigClassifierTerm := setup.GetAnyValue(baseConfigClassifier.Term)
			baseConfigClassifierTermConditions := baseConfigClassifierTerm.Conditions
			baseConfigClassifierTermConditionsIpv4 := baseConfigClassifierTermConditions.Ipv4
			if baseConfigClassifierTermConditionsIpv4.Dscp == nil {
				baseConfigClassifierTermConditionsIpv4.Dscp = ygot.Uint8(baseConfigClassifierTermConditionsIpv4.DscpSet[0])
				baseConfigClassifierTermConditionsIpv4.DscpSet = nil
			}

			config := gnmi.OC().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Ipv4().Dscp()
			state := gnmi.OC().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Ipv4().Dscp()

			t.Run("Replace leaf", func(t *testing.T) {
				gnmi.Replace(t, dut, config.Config(), input)
			})
			// dut.Config().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Ipv4().Dscp() return nil
			// dut.Config().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Ipv4().DscpSet() returns a list
			t.Run("Get leaf", func(t *testing.T) {
				t.Skip()
				configGot := gnmi.Get(t, dut, config.Config())
				if configGot != input {
					t.Errorf("Config /qos/classifiers/classifier/terms/term/conditions/ipv4/config/dscp: got %v, want %v", configGot, input)
				}
			})
			// ERR:No sysdb paths found for yang path qos/classifiers/classifier/terms/term/conditions/ipv4/state/dscp\x00"} (*gnmi.SubscribeResponse_Error)
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := gnmi.Get(t, dut, state.State())
					if stateGot != input {
						t.Errorf("State /qos/classifiers/classifier/terms/term/conditions/ipv4/state/dscp: got %v, want %v", stateGot, input)
					}
				})
			}
			// Delete request goes through fine but nothing is getting deleted actually.
			t.Run("Delete leaf", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				if !setup.SkipSubscribe() {
					if qs := gnmi.LookupConfig(t, dut, config.Config()); qs != nil {
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
			if baseConfigClassifierTermConditionsIpv4.DscpSet == nil {
				baseConfigClassifierTermConditionsIpv4.DscpSet = []uint8{*baseConfigClassifierTermConditionsIpv4.Dscp}
				baseConfigClassifierTermConditionsIpv4.Dscp = nil
			}
			baseConfigClassifierTermConditionsIpv4.DscpSet = input

			config := gnmi.OC().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Ipv4()
			state := gnmi.OC().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Ipv4()

			t.Run("Replace container", func(t *testing.T) {
				gnmi.Replace(t, dut, config.Config(), baseConfigClassifierTermConditionsIpv4)
			})
			// 	telemetry.Qos_Classifier_Term_Conditions_Ipv4{
			//         DestinationAddress: nil,
			//         Dscp:               nil,
			//         DscpSet: []uint8{
			// -               0x01,
			//                 0x00,
			//                 0x3f,
			//         },
			//         HopLimit:      nil,
			//         Protocol:      nil,
			//         SourceAddress: nil,
			//   }
			t.Run("Get container", func(t *testing.T) {
				configGot := gnmi.Get(t, dut, config.Config())
				if diff := cmp.Diff(*configGot, *baseConfigClassifierTermConditionsIpv4); diff != "" {
					t.Errorf("Config /qos/classifiers/classifier/terms/term/conditions/ipv4/config/dscp-set: %v", diff)
				}
			})
			// No sysdb paths found for yang path qos/classifiers/classifier/terms/term/conditions/ipv4\x00"} (*gnmi.SubscribeResponse_Error)
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := gnmi.Get(t, dut, state.State())
					for i, sg := range stateGot.DscpSet {
						if sg != input[i] {
							t.Errorf("Config /qos/classifiers/classifier/terms/term/conditions/ipv4/config/dscp-set: got %v, want %v", sg, input[i])
						}
					}
				})
			}
			// Delete request goes through fine but nothing is getting deleted actually.
			t.Run("Delete container", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				if !setup.SkipSubscribe() {
					if qs, _ := gnmi.LookupConfig(t, dut, config.Config()).Val(); qs.DscpSet != nil {
						t.Errorf("Delete /qos/classifiers/classifier/terms/term/conditions/ipv4/config/dscp-set fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestDscpSetAtLeaf(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	var baseConfig *oc.Qos = setupQos(t, dut, "base_config_classifier_term_ipv4.json")
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testDscpSetInput {
		t.Run(fmt.Sprintf("Testing /qos/classifiers/classifier/terms/term/conditions/ipv4/config/dscp-set using value %v", input), func(t *testing.T) {
			baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)
			baseConfigClassifierTerm := setup.GetAnyValue(baseConfigClassifier.Term)
			baseConfigClassifierTermConditions := baseConfigClassifierTerm.Conditions
			baseConfigClassifierTermConditionsIpv4 := baseConfigClassifierTermConditions.Ipv4
			if baseConfigClassifierTermConditionsIpv4.DscpSet == nil {
				baseConfigClassifierTermConditionsIpv4.DscpSet = []uint8{*baseConfigClassifierTermConditionsIpv4.Dscp}
				baseConfigClassifierTermConditionsIpv4.Dscp = nil
			}

			config := gnmi.OC().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Ipv4().DscpSet()
			state := gnmi.OC().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Ipv4().DscpSet()

			t.Run("Replace leaf", func(t *testing.T) {
				gnmi.Replace(t, dut, config.Config(), input)
			})
			t.Run("Get leaf", func(t *testing.T) {
				configGot := gnmi.Get(t, dut, config.Config())
				for i, cg := range configGot {
					if cg != input[i] {
						t.Errorf("Config /qos/classifiers/classifier/terms/term/conditions/ipv4/config/dscp-set: got %v, want %v", cg, input[i])
					}
				}
			})
			// ERR:No sysdb paths found for yang path qos/classifiers/classifier/terms/term/conditions/ipv4/state/dscp-set\x00"} (*gnmi.SubscribeResponse_Error)
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := gnmi.Get(t, dut, state.State())
					for i, sg := range stateGot {
						if sg != input[i] {
							t.Errorf("Config /qos/classifiers/classifier/terms/term/conditions/ipv4/state/dscp-set: got %v, want %v", sg, input[i])
						}
					}
				})
			}
			// Delete request goes through fine but nothing is getting deleted actually.
			t.Run("Delete leaf", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				if !setup.SkipSubscribe() {
					if qs := gnmi.LookupConfig(t, dut, config.Config()); qs != nil {
						t.Errorf("Delete /qos/classifiers/classifier/terms/term/conditions/ipv4/config/dscp-set fail: got %v", qs)
					}
				}
			})
		})
	}
}
