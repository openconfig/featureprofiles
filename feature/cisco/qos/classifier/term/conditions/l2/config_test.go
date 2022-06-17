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

func TestSourceMacAtContainer(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testSourceMacInput {
		t.Run(fmt.Sprintf("Testing /qos/classifiers/classifier/terms/term/conditions/l2/config/source-mac using value %v", input), func(t *testing.T) {
			baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)
			baseConfigClassifierTerm := setup.GetAnyValue(baseConfigClassifier.Term)
			baseConfigClassifierTermConditions := baseConfigClassifierTerm.Conditions
			baseConfigClassifierTermConditionsL2 := baseConfigClassifierTermConditions.L2
			*baseConfigClassifierTermConditionsL2.SourceMac = input

			config := dut.Config().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().L2()
			state := dut.Telemetry().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().L2()

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigClassifierTermConditionsL2)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.SourceMac != input {
						t.Errorf("Config /qos/classifiers/classifier/terms/term/conditions/l2/config/source-mac: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.SourceMac != input {
						t.Errorf("State /qos/classifiers/classifier/terms/term/conditions/l2/config/source-mac: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete container", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).SourceMac != nil {
						t.Errorf("Delete /qos/classifiers/classifier/terms/term/conditions/l2/config/source-mac fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestSourceMacAtLeaf(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testSourceMacInput {
		t.Run(fmt.Sprintf("Testing /qos/classifiers/classifier/terms/term/conditions/l2/config/source-mac using value %v", input), func(t *testing.T) {
			baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)
			baseConfigClassifierTerm := setup.GetAnyValue(baseConfigClassifier.Term)

			config := dut.Config().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().L2().SourceMac()
			state := dut.Telemetry().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().L2().SourceMac()

			t.Run("Replace leaf", func(t *testing.T) {
				config.Replace(t, input)
			})
			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot != input {
						t.Errorf("Config /qos/classifiers/classifier/terms/term/conditions/l2/config/source-mac: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot != input {
						t.Errorf("State /qos/classifiers/classifier/terms/term/conditions/l2/config/source-mac: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete leaf", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs != nil {
						t.Errorf("Delete /qos/classifiers/classifier/terms/term/conditions/l2/config/source-mac fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestSourceMacMaskAtContainer(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testSourceMacMaskInput {
		t.Run(fmt.Sprintf("Testing /qos/classifiers/classifier/terms/term/conditions/l2/config/source-mac-mask using value %v", input), func(t *testing.T) {
			baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)
			baseConfigClassifierTerm := setup.GetAnyValue(baseConfigClassifier.Term)
			baseConfigClassifierTermConditions := baseConfigClassifierTerm.Conditions
			baseConfigClassifierTermConditionsL2 := baseConfigClassifierTermConditions.L2
			*baseConfigClassifierTermConditionsL2.SourceMacMask = input

			config := dut.Config().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().L2()
			state := dut.Telemetry().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().L2()

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigClassifierTermConditionsL2)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.SourceMacMask != input {
						t.Errorf("Config /qos/classifiers/classifier/terms/term/conditions/l2/config/source-mac-mask: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.SourceMacMask != input {
						t.Errorf("State /qos/classifiers/classifier/terms/term/conditions/l2/config/source-mac-mask: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete container", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).SourceMacMask != nil {
						t.Errorf("Delete /qos/classifiers/classifier/terms/term/conditions/l2/config/source-mac-mask fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestSourceMacMaskAtLeaf(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testSourceMacMaskInput {
		t.Run(fmt.Sprintf("Testing /qos/classifiers/classifier/terms/term/conditions/l2/config/source-mac-mask using value %v", input), func(t *testing.T) {
			baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)
			baseConfigClassifierTerm := setup.GetAnyValue(baseConfigClassifier.Term)

			config := dut.Config().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().L2().SourceMacMask()
			state := dut.Telemetry().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().L2().SourceMacMask()

			t.Run("Replace leaf", func(t *testing.T) {
				config.Replace(t, input)
			})
			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot != input {
						t.Errorf("Config /qos/classifiers/classifier/terms/term/conditions/l2/config/source-mac-mask: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot != input {
						t.Errorf("State /qos/classifiers/classifier/terms/term/conditions/l2/config/source-mac-mask: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete leaf", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs != nil {
						t.Errorf("Delete /qos/classifiers/classifier/terms/term/conditions/l2/config/source-mac-mask fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestEthertypeAtContainer(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testEthertypeInput {
		t.Run(fmt.Sprintf("Testing /qos/classifiers/classifier/terms/term/conditions/l2/config/ethertype using value %v", input), func(t *testing.T) {
			baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)
			baseConfigClassifierTerm := setup.GetAnyValue(baseConfigClassifier.Term)
			baseConfigClassifierTermConditions := baseConfigClassifierTerm.Conditions
			baseConfigClassifierTermConditionsL2 := baseConfigClassifierTermConditions.L2
			baseConfigClassifierTermConditionsL2.Ethertype = input

			config := dut.Config().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().L2()
			state := dut.Telemetry().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().L2()

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigClassifierTermConditionsL2)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot.Ethertype != input {
						t.Errorf("Config /qos/classifiers/classifier/terms/term/conditions/l2/config/ethertype: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot.Ethertype != input {
						t.Errorf("State /qos/classifiers/classifier/terms/term/conditions/l2/config/ethertype: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete container", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).Ethertype != nil {
						t.Errorf("Delete /qos/classifiers/classifier/terms/term/conditions/l2/config/ethertype fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestEthertypeAtLeaf(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testEthertypeInput {
		t.Run(fmt.Sprintf("Testing /qos/classifiers/classifier/terms/term/conditions/l2/config/ethertype using value %v", input), func(t *testing.T) {
			baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)
			baseConfigClassifierTerm := setup.GetAnyValue(baseConfigClassifier.Term)

			config := dut.Config().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().L2().Ethertype()
			state := dut.Telemetry().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().L2().Ethertype()

			t.Run("Replace leaf", func(t *testing.T) {
				config.Replace(t, input)
			})
			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot != input {
						t.Errorf("Config /qos/classifiers/classifier/terms/term/conditions/l2/config/ethertype: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot != input {
						t.Errorf("State /qos/classifiers/classifier/terms/term/conditions/l2/config/ethertype: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete leaf", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs != nil {
						t.Errorf("Delete /qos/classifiers/classifier/terms/term/conditions/l2/config/ethertype fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestDestinationMacAtContainer(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testDestinationMacInput {
		t.Run(fmt.Sprintf("Testing /qos/classifiers/classifier/terms/term/conditions/l2/config/destination-mac using value %v", input), func(t *testing.T) {
			baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)
			baseConfigClassifierTerm := setup.GetAnyValue(baseConfigClassifier.Term)
			baseConfigClassifierTermConditions := baseConfigClassifierTerm.Conditions
			baseConfigClassifierTermConditionsL2 := baseConfigClassifierTermConditions.L2
			*baseConfigClassifierTermConditionsL2.DestinationMac = input

			config := dut.Config().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().L2()
			state := dut.Telemetry().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().L2()

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigClassifierTermConditionsL2)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.DestinationMac != input {
						t.Errorf("Config /qos/classifiers/classifier/terms/term/conditions/l2/config/destination-mac: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.DestinationMac != input {
						t.Errorf("State /qos/classifiers/classifier/terms/term/conditions/l2/config/destination-mac: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete container", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).DestinationMac != nil {
						t.Errorf("Delete /qos/classifiers/classifier/terms/term/conditions/l2/config/destination-mac fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestDestinationMacAtLeaf(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testDestinationMacInput {
		t.Run(fmt.Sprintf("Testing /qos/classifiers/classifier/terms/term/conditions/l2/config/destination-mac using value %v", input), func(t *testing.T) {
			baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)
			baseConfigClassifierTerm := setup.GetAnyValue(baseConfigClassifier.Term)

			config := dut.Config().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().L2().DestinationMac()
			state := dut.Telemetry().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().L2().DestinationMac()

			t.Run("Replace leaf", func(t *testing.T) {
				config.Replace(t, input)
			})
			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot != input {
						t.Errorf("Config /qos/classifiers/classifier/terms/term/conditions/l2/config/destination-mac: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot != input {
						t.Errorf("State /qos/classifiers/classifier/terms/term/conditions/l2/config/destination-mac: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete leaf", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs != nil {
						t.Errorf("Delete /qos/classifiers/classifier/terms/term/conditions/l2/config/destination-mac fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestDestinationMacMaskAtContainer(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testDestinationMacMaskInput {
		t.Run(fmt.Sprintf("Testing /qos/classifiers/classifier/terms/term/conditions/l2/config/destination-mac-mask using value %v", input), func(t *testing.T) {
			baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)
			baseConfigClassifierTerm := setup.GetAnyValue(baseConfigClassifier.Term)
			baseConfigClassifierTermConditions := baseConfigClassifierTerm.Conditions
			baseConfigClassifierTermConditionsL2 := baseConfigClassifierTermConditions.L2
			*baseConfigClassifierTermConditionsL2.DestinationMacMask = input

			config := dut.Config().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().L2()
			state := dut.Telemetry().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().L2()

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigClassifierTermConditionsL2)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.DestinationMacMask != input {
						t.Errorf("Config /qos/classifiers/classifier/terms/term/conditions/l2/config/destination-mac-mask: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.DestinationMacMask != input {
						t.Errorf("State /qos/classifiers/classifier/terms/term/conditions/l2/config/destination-mac-mask: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete container", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).DestinationMacMask != nil {
						t.Errorf("Delete /qos/classifiers/classifier/terms/term/conditions/l2/config/destination-mac-mask fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestDestinationMacMaskAtLeaf(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testDestinationMacMaskInput {
		t.Run(fmt.Sprintf("Testing /qos/classifiers/classifier/terms/term/conditions/l2/config/destination-mac-mask using value %v", input), func(t *testing.T) {
			baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)
			baseConfigClassifierTerm := setup.GetAnyValue(baseConfigClassifier.Term)

			config := dut.Config().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().L2().DestinationMacMask()
			state := dut.Telemetry().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().L2().DestinationMacMask()

			t.Run("Replace leaf", func(t *testing.T) {
				config.Replace(t, input)
			})
			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot != input {
						t.Errorf("Config /qos/classifiers/classifier/terms/term/conditions/l2/config/destination-mac-mask: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot != input {
						t.Errorf("State /qos/classifiers/classifier/terms/term/conditions/l2/config/destination-mac-mask: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete leaf", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs != nil {
						t.Errorf("Delete /qos/classifiers/classifier/terms/term/conditions/l2/config/destination-mac-mask fail: got %v", qs)
					}
				}
			})
		})
	}
}
