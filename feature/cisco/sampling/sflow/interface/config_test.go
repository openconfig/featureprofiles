package sampling_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/feature/cisco/sampling/setup"
	"github.com/openconfig/featureprofiles/topologies/binding"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
)

func TestMain(m *testing.M) {
	ondatra.RunTests(m, binding.New)
}

func TestEnabledAtContainer(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Sampling = setupSampling(t, dut)
	defer teardownSampling(t, dut, baseConfig)

	for _, input := range testEnabledInput {
		t.Run(fmt.Sprintf("Testing /sampling/sflow/interfaces/interface/config/enabled using value %v", input), func(t *testing.T) {
			baseConfigSflow := baseConfig.Sflow
			baseConfigSflowInterface := setup.GetAnyValue(baseConfigSflow.Interface)
			*baseConfigSflowInterface.Enabled = input

			config := gnmi.OC().Sampling().Sflow().Interface(*baseConfigSflowInterface.Name)
			state := gnmi.OC().Sampling().Sflow().Interface(*baseConfigSflowInterface.Name)

			t.Run("Replace container", func(t *testing.T) {
				gnmi.Replace(t, dut, config.Config(), baseConfigSflowInterface)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := gnmi.GetConfig(t, dut, config.Config())
					if *configGot.Enabled != input {
						t.Errorf("Config /sampling/sflow/interfaces/interface/config/enabled: got %v, want %v", configGot, input)
					}
				})
			}
			t.Run("Update container", func(t *testing.T) {
				gnmi.Update(t, dut, config.Config(), baseConfigSflowInterface)
			})
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := gnmi.Get(t, dut, state.State())
					if *stateGot.Enabled != input {
						t.Errorf("State /sampling/sflow/interfaces/interface/config/enabled: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete container", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				if !setup.SkipSubscribe() {
					if qs := gnmi.LookupConfig(t, dut, config.Config()); qs.IsPresent() == true {
						t.Errorf("Delete /sampling/sflow/interfaces/interface/config/enabled fail: got %v", qs.IsPresent())
					}
				}
			})
		})
	}
}

func TestEnabledAtLeaf(t *testing.T) {
	t.Skip()
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupSampling(t, dut)
	defer teardownSampling(t, dut, baseConfig)

	for _, input := range testEnabledInput {
		t.Run(fmt.Sprintf("Testing /sampling/sflow/interfaces/interface/config/enabled using value %v", input), func(t *testing.T) {
			baseConfigSflowInterface := setup.GetAnyValue(baseConfig.Sflow.Interface)

			config := gnmi.OC().Sampling().Sflow().Interface(*baseConfigSflowInterface.Name).Enabled()
			state := gnmi.OC().Sampling().Sflow().Interface(*baseConfigSflowInterface.Name).Enabled()

			t.Run("Replace leaf", func(t *testing.T) {
				gnmi.Replace(t, dut, config.Config(), input)
			})
			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := gnmi.GetConfig(t, dut, config.Config())
					if configGot != input {
						t.Errorf("Config /sampling/sflow/interfaces/interface/config/enabled: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := gnmi.Get(t, dut, state.State())
					if stateGot != input {
						t.Errorf("State /sampling/sflow/interfaces/interface/config/enabled: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete leaf", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				if !setup.SkipSubscribe() {
					if qs := gnmi.LookupConfig(t, dut, config.Config()); qs != nil {
						t.Errorf("Delete /sampling/sflow/interfaces/interface/config/enabled fail: got %v", qs)
					}
				}
			})
		})
	}
}

func TestNameAtContainer(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Sampling = setupSampling(t, dut)
	defer teardownSampling(t, dut, baseConfig)

	for _, input := range testNameInput {
		t.Run(fmt.Sprintf("Testing /sampling/sflow/interfaces/interface/config/name using value %v", input), func(t *testing.T) {
			baseConfigSflow := baseConfig.Sflow
			baseConfigSflowInterface := setup.GetAnyValue(baseConfigSflow.Interface)
			*baseConfigSflowInterface.Name = input
			*baseConfigSflowInterface.Enabled = true
			*baseConfigSflow.Enabled = true

			config := gnmi.OC().Sampling().Sflow().Interface(*baseConfigSflowInterface.Name)
			state := gnmi.OC().Sampling().Sflow().Interface(*baseConfigSflowInterface.Name)

			t.Run("Replace container", func(t *testing.T) {
				gnmi.Replace(t, dut, config.Config(), baseConfigSflowInterface)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := gnmi.GetConfig(t, dut, config.Config())
					if *configGot.Name != input {
						t.Errorf("Config /sampling/sflow/interfaces/interface/config/name: got %v, want %v", configGot, input)
					}
				})
			}
			t.Run("Update container", func(t *testing.T) {
				gnmi.Update(t, dut, config.Config(), baseConfigSflowInterface)
			})
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := gnmi.Get(t, dut, state.State())
					if *stateGot.Name != input {
						t.Errorf("State /sampling/sflow/interfaces/interface/config/name: got %v, want %v", stateGot, input)
					}
				})
			}
		})
	}
}

func TestGlobalSampleSize(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Sampling = setupSampling(t, dut)
	defer teardownSampling(t, dut, baseConfig)

	t.Run("Testing /sampling/sflow/config/sample-size", func(t *testing.T) {
		baseConfigSflow := baseConfig.Sflow
		*baseConfigSflow.Enabled = true
		*baseConfigSflow.SampleSize = 256

		config := gnmi.OC().Sampling().Sflow()
		t.Run("Replace container", func(t *testing.T) {
			gnmi.Replace(t, dut, config.Config(), baseConfigSflow)
		})
		t.Run("Replace leaf", func(t *testing.T) {
			gnmi.Replace(t, dut, gnmi.OC().Sampling().Sflow().SampleSize().Config(), 128)
		})
		if !setup.SkipGet() {
			t.Run("Get container", func(t *testing.T) {
				configGot := gnmi.GetConfig(t, dut, gnmi.OC().Sampling().Sflow().SampleSize().Config())
				if configGot != 128 {
					t.Errorf("Config /sampling/sflow/config/sample-size: got %v, want 710", configGot)
				}
			})
		}
		t.Run("Update leaf", func(t *testing.T) {
			gnmi.Update(t, dut, gnmi.OC().Sampling().Sflow().SampleSize().Config(), 256)
		})
		t.Run("Delete leaf", func(t *testing.T) {
			gnmi.Delete(t, dut, gnmi.OC().Sampling().Sflow().SampleSize().Config())
			if !setup.SkipSubscribe() {
				if qs, _ := gnmi.LookupConfig(t, dut, config.Config()).Val(); qs.SampleSize != nil {
					t.Errorf("Delete /sampling/sflow/config/sample-size fail: got %v", qs.SampleSize)
				}
			}
		})
	})
}

// /sampling/sflow/interfaces/interface/config/ingress-sampling-rate
func TestInterfaceIngressSamplingRate(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupSampling(t, dut)
	defer teardownSampling(t, dut, baseConfig)

	for _, input := range testInterfaceIngressSamplingRate {
		t.Run(fmt.Sprintf("Testing /sampling/sflow/interfaces/interface/config/ingress-sampling-rate using value %v", input), func(t *testing.T) {
			baseConfigSflow := baseConfig.Sflow
			baseConfigSflowInterface := setup.GetAnyValue(baseConfigSflow.Interface)
			*baseConfigSflowInterface.IngressSamplingRate = input
			config := gnmi.OC().Sampling().Sflow().Interface(*baseConfigSflowInterface.Name)
			state := gnmi.OC().Sampling().Sflow().Interface(*baseConfigSflowInterface.Name).IngressSamplingRate()

			t.Run("Replace leaf", func(t *testing.T) {
				gnmi.Replace(t, dut, config.Config(), baseConfigSflowInterface)
			})
			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := gnmi.GetConfig(t, dut, config.Config())
					if *configGot.IngressSamplingRate != input {
						t.Errorf("Config /sampling/sflow/interfaces/interface/config/ingress-sampling-rate : got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := gnmi.Get(t, dut, state.State())
					if stateGot != input {
						t.Errorf("State /sampling/sflow/interfaces/interface/config/ingress-sampling-rate : got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Update leaf", func(t *testing.T) {
				gnmi.Update(t, dut, config.Config(), baseConfigSflowInterface)
			})
			t.Run("Delete leaf", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				if !setup.SkipSubscribe() {
					if qs := gnmi.LookupConfig(t, dut, config.Config()); qs.IsPresent() == true {
						t.Errorf("Delete /sampling/sflow/interfaces/interface/config/ingress-sampling-rate fail: got %v", qs)
					}
				}
			})
		})
	}
}

// /sampling/sflow/interfaces/interface/state/egress-sampling-rate
func TestInterfaceEgressSamplingRate(t *testing.T) {
	t.Skip() // egress-sampling-rate not supported
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupSampling(t, dut)
	defer teardownSampling(t, dut, baseConfig)

	for _, input := range testInterfaceEgressSamplingRate {
		t.Run(fmt.Sprintf("Testing /sampling/sflow/interfaces/interface/state/egress-sampling-rate using value %v", input), func(t *testing.T) {
			baseConfigSflow := baseConfig.Sflow
			baseConfigSflowInterface := setup.GetAnyValue(baseConfigSflow.Interface)
			*baseConfigSflowInterface.EgressSamplingRate = input
			config := gnmi.OC().Sampling().Sflow().Interface(*baseConfigSflowInterface.Name)
			state := gnmi.OC().Sampling().Sflow().Interface(*baseConfigSflowInterface.Name).EgressSamplingRate()

			t.Run("Replace leaf", func(t *testing.T) {
				gnmi.Replace(t, dut, config.Config(), baseConfigSflowInterface)
			})
			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := gnmi.GetConfig(t, dut, config.Config())
					if *configGot.EgressSamplingRate != input {
						t.Errorf("Config /sampling/sflow/interfaces/interface/state/egress-sampling-rate : got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := gnmi.Get(t, dut, state.State())
					if stateGot != input {
						t.Errorf("State /sampling/sflow/interfaces/interface/state/egress-sampling-rate : got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Update leaf", func(t *testing.T) {
				gnmi.Update(t, dut, config.Config(), baseConfigSflowInterface)
			})
			t.Run("Delete leaf", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				if !setup.SkipSubscribe() {
					if qs := gnmi.LookupConfig(t, dut, config.Config()); qs.IsPresent() == true {
						t.Errorf("Delete /sampling/sflow/interfaces/interface/state/egress-sampling-rate fail: got %v", qs)
					}
				}
			})
		})
	}
}

func TestInterfaceStateLeafs(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupSampling(t, dut)
	defer teardownSampling(t, dut, baseConfig)
	baseConfigSflow := baseConfig.Sflow
	baseConfigSflowInterface := setup.GetAnyValue(baseConfigSflow.Interface)
	state := gnmi.OC().Sampling().Sflow().Interface(*baseConfigSflowInterface.Name)
	//state.PollingInterval() ?
	t.Run("Subscribe Container level", func(t *testing.T) {
		gnmi.Get(t, dut, state.State())
	})
	t.Run("Subscribe Enabled", func(t *testing.T) {
		stateGot := gnmi.Get(t, dut, state.Enabled().State())
		if stateGot != true {
			t.Errorf("State Enabled: got %v, want %v", stateGot, true)
		}
	})
	t.Log("Watch on Enabled")
	_, ok := gnmi.Watch(t, dut, state.Enabled().State(), time.Minute, func(val *ygnmi.Value[bool]) bool {
		currState, ok := val.Val()
		return ok && currState == true
	}).Await(t)
	if !ok {
		t.Errorf("Enabled not true")
	}
	t.Run("Subscribe Name", func(t *testing.T) {
		stateGot := gnmi.Get(t, dut, state.Name().State())
		if stateGot != "Bundle-Ether1" {
			t.Errorf("State Name: got %v, want %v", stateGot, "Bundle-Ether1")
		}
	})
	t.Log("Watch on Name")
	_, ok = gnmi.Watch(t, dut, state.Name().State(), time.Minute, func(val *ygnmi.Value[string]) bool {
		currState, ok := val.Val()
		return ok && currState == "Bundle-Ether1"
	}).Await(t)
	if !ok {
		t.Errorf("Name not correct")
	}
	t.Run("Subscribe IngressSamplingRate", func(t *testing.T) {
		stateGot := gnmi.Get(t, dut, state.IngressSamplingRate().State())
		if stateGot != 80 {
			t.Errorf("State IngressSamplingRate: got %v, want %v", stateGot, 80)
		}
	})
	t.Log("Watch on IngressSamplingRate")
	_, ok = gnmi.Watch(t, dut, state.IngressSamplingRate().State(), time.Minute, func(val *ygnmi.Value[uint32]) bool {
		currState, ok := val.Val()
		return ok && currState == 80
	}).Await(t)
	if !ok {
		t.Errorf("IngressSamplingRate not correct")
	}
	// EgressSamplingRate not supported now
	// t.Run("Subscribe EgressSamplingRate", func(t *testing.T) {
	// 	stateGot := gnmi.Get(t, dut, state.EgressSamplingRate().State())
	// 	if stateGot != 90 {
	// 		t.Errorf("State EnaEgressSamplingRatebled: got %v, want %v", stateGot, 90)
	// 	}
	// })
	// t.Log("Watch on EgressSamplingRate")
	// _, ok = gnmi.Watch(t, dut, state.EgressSamplingRate().State(), time.Minute, func(val *ygnmi.Value[uint32]) bool {
	// 	currState, ok := val.Val()
	// 	return ok && currState == 90
	// }).Await(t)
	// if !ok {
	// 	t.Errorf("EgressSamplingRate not correct")
	// }
}
