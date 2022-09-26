package sampling_test

import (
	"fmt"
	"testing"

	"github.com/openconfig/featureprofiles/feature/cisco/sampling/setup"
	"github.com/openconfig/featureprofiles/topologies/binding"
	"github.com/openconfig/ondatra"
	oc "github.com/openconfig/ondatra/telemetry"
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

			config := dut.Config().Sampling().Sflow().Interface(*baseConfigSflowInterface.Name)
			state := dut.Telemetry().Sampling().Sflow().Interface(*baseConfigSflowInterface.Name)

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigSflowInterface)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.Enabled != input {
						t.Errorf("Config /sampling/sflow/interfaces/interface/config/enabled: got %v, want %v", configGot, input)
					}
				})
			}
			t.Run("Update container", func(t *testing.T) {
				config.Update(t, baseConfigSflowInterface)
			})
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.Enabled != input {
						t.Errorf("State /sampling/sflow/interfaces/interface/config/enabled: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete container", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).Enabled != nil {
						t.Errorf("Delete /sampling/sflow/interfaces/interface/config/enabled fail: got %v", qs)
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

			config := dut.Config().Sampling().Sflow().Interface(*baseConfigSflowInterface.Name).Enabled()
			state := dut.Telemetry().Sampling().Sflow().Interface(*baseConfigSflowInterface.Name).Enabled()

			t.Run("Replace leaf", func(t *testing.T) {
				config.Replace(t, input)
			})
			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot != input {
						t.Errorf("Config /sampling/sflow/interfaces/interface/config/enabled: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot != input {
						t.Errorf("State /sampling/sflow/interfaces/interface/config/enabled: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete leaf", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs != nil {
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

			config := dut.Config().Sampling().Sflow().Interface(*baseConfigSflowInterface.Name)
			state := dut.Telemetry().Sampling().Sflow().Interface(*baseConfigSflowInterface.Name)

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigSflowInterface)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.Name != input {
						t.Errorf("Config /sampling/sflow/interfaces/interface/config/name: got %v, want %v", configGot, input)
					}
				})
			}
			t.Run("Update container", func(t *testing.T) {
				config.Update(t, baseConfigSflowInterface)
			})
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
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

	t.Run(fmt.Sprintf("Testing /sampling/sflow/config/sample-size"), func(t *testing.T) {
		baseConfigSflow := baseConfig.Sflow
		*baseConfigSflow.Enabled = true
		*baseConfigSflow.SampleSize = 256

		config := dut.Config().Sampling().Sflow()
		t.Run("Replace container", func(t *testing.T) {
			config.Replace(t, baseConfigSflow)
		})
		t.Run("Replace leaf", func(t *testing.T) {
			dut.Config().Sampling().Sflow().SampleSize().Replace(t, 128)
		})
		if !setup.SkipGet() {
			t.Run("Get container", func(t *testing.T) {
				configGot := dut.Config().Sampling().Sflow().SampleSize().Get(t)
				if configGot != 128 {
					t.Errorf("Config /sampling/sflow/config/sample-size: got %v, want 710", configGot)
				}
			})
		}
		t.Run("Update leaf", func(t *testing.T) {
			dut.Config().Sampling().Sflow().SampleSize().Update(t, 256)
		})
		t.Run("Delete leaf", func(t *testing.T) {
			dut.Config().Sampling().Sflow().SampleSize().Delete(t)
			if !setup.SkipSubscribe() {
				if qs := config.Lookup(t); qs.Val(t).SampleSize != nil {
					t.Errorf("Delete /sampling/sflow/config/sample-size fail: got %v", qs)
				}
			}
		})
	})
}
