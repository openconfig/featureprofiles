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

func TestHopLimitAtContainer(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testHopLimitInput {
		t.Run(fmt.Sprintf("Testing /qos/classifiers/classifier/terms/term/conditions/ipv6/config/hop-limit using value %v", input), func(t *testing.T) {
			baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)
			baseConfigClassifierTerm := setup.GetAnyValue(baseConfigClassifier.Term)
			baseConfigClassifierTermConditions := baseConfigClassifierTerm.Conditions
			baseConfigClassifierTermConditionsIpv6 := baseConfigClassifierTermConditions.Ipv6
			*baseConfigClassifierTermConditionsIpv6.HopLimit = input

			config := dut.Config().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Ipv6()
			state := dut.Telemetry().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Ipv6()

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigClassifierTermConditionsIpv6)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.HopLimit != input {
						t.Errorf("Config /qos/classifiers/classifier/terms/term/conditions/ipv6/config/hop-limit: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.HopLimit != input {
						t.Errorf("State /qos/classifiers/classifier/terms/term/conditions/ipv6/config/hop-limit: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete container", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).HopLimit != nil {
						t.Errorf("Delete /qos/classifiers/classifier/terms/term/conditions/ipv6/config/hop-limit fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestHopLimitAtLeaf(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testHopLimitInput {
		t.Run(fmt.Sprintf("Testing /qos/classifiers/classifier/terms/term/conditions/ipv6/config/hop-limit using value %v", input), func(t *testing.T) {
			baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)
			baseConfigClassifierTerm := setup.GetAnyValue(baseConfigClassifier.Term)

			config := dut.Config().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Ipv6().HopLimit()
			state := dut.Telemetry().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Ipv6().HopLimit()

			t.Run("Replace leaf", func(t *testing.T) {
				config.Replace(t, input)
			})
			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot != input {
						t.Errorf("Config /qos/classifiers/classifier/terms/term/conditions/ipv6/config/hop-limit: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot != input {
						t.Errorf("State /qos/classifiers/classifier/terms/term/conditions/ipv6/config/hop-limit: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete leaf", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs != nil {
						t.Errorf("Delete /qos/classifiers/classifier/terms/term/conditions/ipv6/config/hop-limit fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestDestinationFlowLabelAtContainer(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testDestinationFlowLabelInput {
		t.Run(fmt.Sprintf("Testing /qos/classifiers/classifier/terms/term/conditions/ipv6/config/destination-flow-label using value %v", input), func(t *testing.T) {
			baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)
			baseConfigClassifierTerm := setup.GetAnyValue(baseConfigClassifier.Term)
			baseConfigClassifierTermConditions := baseConfigClassifierTerm.Conditions
			baseConfigClassifierTermConditionsIpv6 := baseConfigClassifierTermConditions.Ipv6
			*baseConfigClassifierTermConditionsIpv6.DestinationFlowLabel = input

			config := dut.Config().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Ipv6()
			state := dut.Telemetry().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Ipv6()

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigClassifierTermConditionsIpv6)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.DestinationFlowLabel != input {
						t.Errorf("Config /qos/classifiers/classifier/terms/term/conditions/ipv6/config/destination-flow-label: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.DestinationFlowLabel != input {
						t.Errorf("State /qos/classifiers/classifier/terms/term/conditions/ipv6/config/destination-flow-label: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete container", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).DestinationFlowLabel != nil {
						t.Errorf("Delete /qos/classifiers/classifier/terms/term/conditions/ipv6/config/destination-flow-label fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestDestinationFlowLabelAtLeaf(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testDestinationFlowLabelInput {
		t.Run(fmt.Sprintf("Testing /qos/classifiers/classifier/terms/term/conditions/ipv6/config/destination-flow-label using value %v", input), func(t *testing.T) {
			baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)
			baseConfigClassifierTerm := setup.GetAnyValue(baseConfigClassifier.Term)

			config := dut.Config().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Ipv6().DestinationFlowLabel()
			state := dut.Telemetry().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Ipv6().DestinationFlowLabel()

			t.Run("Replace leaf", func(t *testing.T) {
				config.Replace(t, input)
			})
			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot != input {
						t.Errorf("Config /qos/classifiers/classifier/terms/term/conditions/ipv6/config/destination-flow-label: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot != input {
						t.Errorf("State /qos/classifiers/classifier/terms/term/conditions/ipv6/config/destination-flow-label: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete leaf", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs != nil {
						t.Errorf("Delete /qos/classifiers/classifier/terms/term/conditions/ipv6/config/destination-flow-label fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestDestinationAddressAtContainer(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testDestinationAddressInput {
		t.Run(fmt.Sprintf("Testing /qos/classifiers/classifier/terms/term/conditions/ipv6/config/destination-address using value %v", input), func(t *testing.T) {
			baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)
			baseConfigClassifierTerm := setup.GetAnyValue(baseConfigClassifier.Term)
			baseConfigClassifierTermConditions := baseConfigClassifierTerm.Conditions
			baseConfigClassifierTermConditionsIpv6 := baseConfigClassifierTermConditions.Ipv6
			*baseConfigClassifierTermConditionsIpv6.DestinationAddress = input

			config := dut.Config().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Ipv6()
			state := dut.Telemetry().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Ipv6()

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigClassifierTermConditionsIpv6)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.DestinationAddress != input {
						t.Errorf("Config /qos/classifiers/classifier/terms/term/conditions/ipv6/config/destination-address: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.DestinationAddress != input {
						t.Errorf("State /qos/classifiers/classifier/terms/term/conditions/ipv6/config/destination-address: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete container", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).DestinationAddress != nil {
						t.Errorf("Delete /qos/classifiers/classifier/terms/term/conditions/ipv6/config/destination-address fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestDestinationAddressAtLeaf(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testDestinationAddressInput {
		t.Run(fmt.Sprintf("Testing /qos/classifiers/classifier/terms/term/conditions/ipv6/config/destination-address using value %v", input), func(t *testing.T) {
			baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)
			baseConfigClassifierTerm := setup.GetAnyValue(baseConfigClassifier.Term)

			config := dut.Config().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Ipv6().DestinationAddress()
			state := dut.Telemetry().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Ipv6().DestinationAddress()

			t.Run("Replace leaf", func(t *testing.T) {
				config.Replace(t, input)
			})
			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot != input {
						t.Errorf("Config /qos/classifiers/classifier/terms/term/conditions/ipv6/config/destination-address: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot != input {
						t.Errorf("State /qos/classifiers/classifier/terms/term/conditions/ipv6/config/destination-address: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete leaf", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs != nil {
						t.Errorf("Delete /qos/classifiers/classifier/terms/term/conditions/ipv6/config/destination-address fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestDscpAtContainer(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQos(t, dut)
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
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
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
func TestSourceAddressAtContainer(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testSourceAddressInput {
		t.Run(fmt.Sprintf("Testing /qos/classifiers/classifier/terms/term/conditions/ipv6/config/source-address using value %v", input), func(t *testing.T) {
			baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)
			baseConfigClassifierTerm := setup.GetAnyValue(baseConfigClassifier.Term)
			baseConfigClassifierTermConditions := baseConfigClassifierTerm.Conditions
			baseConfigClassifierTermConditionsIpv6 := baseConfigClassifierTermConditions.Ipv6
			*baseConfigClassifierTermConditionsIpv6.SourceAddress = input

			config := dut.Config().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Ipv6()
			state := dut.Telemetry().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Ipv6()

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigClassifierTermConditionsIpv6)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.SourceAddress != input {
						t.Errorf("Config /qos/classifiers/classifier/terms/term/conditions/ipv6/config/source-address: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.SourceAddress != input {
						t.Errorf("State /qos/classifiers/classifier/terms/term/conditions/ipv6/config/source-address: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete container", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).SourceAddress != nil {
						t.Errorf("Delete /qos/classifiers/classifier/terms/term/conditions/ipv6/config/source-address fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestSourceAddressAtLeaf(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testSourceAddressInput {
		t.Run(fmt.Sprintf("Testing /qos/classifiers/classifier/terms/term/conditions/ipv6/config/source-address using value %v", input), func(t *testing.T) {
			baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)
			baseConfigClassifierTerm := setup.GetAnyValue(baseConfigClassifier.Term)

			config := dut.Config().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Ipv6().SourceAddress()
			state := dut.Telemetry().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Ipv6().SourceAddress()

			t.Run("Replace leaf", func(t *testing.T) {
				config.Replace(t, input)
			})
			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot != input {
						t.Errorf("Config /qos/classifiers/classifier/terms/term/conditions/ipv6/config/source-address: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot != input {
						t.Errorf("State /qos/classifiers/classifier/terms/term/conditions/ipv6/config/source-address: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete leaf", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs != nil {
						t.Errorf("Delete /qos/classifiers/classifier/terms/term/conditions/ipv6/config/source-address fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestSourceFlowLabelAtContainer(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testSourceFlowLabelInput {
		t.Run(fmt.Sprintf("Testing /qos/classifiers/classifier/terms/term/conditions/ipv6/config/source-flow-label using value %v", input), func(t *testing.T) {
			baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)
			baseConfigClassifierTerm := setup.GetAnyValue(baseConfigClassifier.Term)
			baseConfigClassifierTermConditions := baseConfigClassifierTerm.Conditions
			baseConfigClassifierTermConditionsIpv6 := baseConfigClassifierTermConditions.Ipv6
			*baseConfigClassifierTermConditionsIpv6.SourceFlowLabel = input

			config := dut.Config().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Ipv6()
			state := dut.Telemetry().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Ipv6()

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigClassifierTermConditionsIpv6)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.SourceFlowLabel != input {
						t.Errorf("Config /qos/classifiers/classifier/terms/term/conditions/ipv6/config/source-flow-label: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.SourceFlowLabel != input {
						t.Errorf("State /qos/classifiers/classifier/terms/term/conditions/ipv6/config/source-flow-label: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete container", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).SourceFlowLabel != nil {
						t.Errorf("Delete /qos/classifiers/classifier/terms/term/conditions/ipv6/config/source-flow-label fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestSourceFlowLabelAtLeaf(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testSourceFlowLabelInput {
		t.Run(fmt.Sprintf("Testing /qos/classifiers/classifier/terms/term/conditions/ipv6/config/source-flow-label using value %v", input), func(t *testing.T) {
			baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)
			baseConfigClassifierTerm := setup.GetAnyValue(baseConfigClassifier.Term)

			config := dut.Config().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Ipv6().SourceFlowLabel()
			state := dut.Telemetry().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Ipv6().SourceFlowLabel()

			t.Run("Replace leaf", func(t *testing.T) {
				config.Replace(t, input)
			})
			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot != input {
						t.Errorf("Config /qos/classifiers/classifier/terms/term/conditions/ipv6/config/source-flow-label: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot != input {
						t.Errorf("State /qos/classifiers/classifier/terms/term/conditions/ipv6/config/source-flow-label: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete leaf", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs != nil {
						t.Errorf("Delete /qos/classifiers/classifier/terms/term/conditions/ipv6/config/source-flow-label fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestDscpSetAtContainer(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQos(t, dut)
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
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
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
func TestProtocolAtContainer(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testProtocolInput {
		t.Run(fmt.Sprintf("Testing /qos/classifiers/classifier/terms/term/conditions/ipv6/config/protocol using value %v", input), func(t *testing.T) {
			baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)
			baseConfigClassifierTerm := setup.GetAnyValue(baseConfigClassifier.Term)
			baseConfigClassifierTermConditions := baseConfigClassifierTerm.Conditions
			baseConfigClassifierTermConditionsIpv6 := baseConfigClassifierTermConditions.Ipv6
			baseConfigClassifierTermConditionsIpv6.Protocol = input

			config := dut.Config().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Ipv6()
			state := dut.Telemetry().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Ipv6()

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigClassifierTermConditionsIpv6)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot.Protocol != input {
						t.Errorf("Config /qos/classifiers/classifier/terms/term/conditions/ipv6/config/protocol: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot.Protocol != input {
						t.Errorf("State /qos/classifiers/classifier/terms/term/conditions/ipv6/config/protocol: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete container", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).Protocol != nil {
						t.Errorf("Delete /qos/classifiers/classifier/terms/term/conditions/ipv6/config/protocol fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestProtocolAtLeaf(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testProtocolInput {
		t.Run(fmt.Sprintf("Testing /qos/classifiers/classifier/terms/term/conditions/ipv6/config/protocol using value %v", input), func(t *testing.T) {
			baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)
			baseConfigClassifierTerm := setup.GetAnyValue(baseConfigClassifier.Term)

			config := dut.Config().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Ipv6().Protocol()
			state := dut.Telemetry().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Ipv6().Protocol()

			t.Run("Replace leaf", func(t *testing.T) {
				config.Replace(t, input)
			})
			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot != input {
						t.Errorf("Config /qos/classifiers/classifier/terms/term/conditions/ipv6/config/protocol: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot != input {
						t.Errorf("State /qos/classifiers/classifier/terms/term/conditions/ipv6/config/protocol: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete leaf", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs != nil {
						t.Errorf("Delete /qos/classifiers/classifier/terms/term/conditions/ipv6/config/protocol fail: got %v", qs)
					}
				}
			})
		})
	}
}
