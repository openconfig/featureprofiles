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
		t.Run(fmt.Sprintf("Testing /sampling/sflow/config/enabled using value %v", input), func(t *testing.T) {
			baseConfigSflow := baseConfig.Sflow
			*baseConfigSflow.Enabled = input

			config := dut.Config().Sampling().Sflow()
			state := dut.Telemetry().Sampling().Sflow()

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigSflow)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.Enabled != input {
						t.Errorf("Config /sampling/sflow/config/enabled: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.Enabled != input {
						t.Errorf("State /sampling/sflow/config/enabled: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete container", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).Enabled != nil {
						t.Errorf("Delete /sampling/sflow/config/enabled fail: got %v", qs)
					}
				}
			})
		})
	}
}

func TestEnabledAtLeaf(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupSampling(t, dut)
	defer teardownSampling(t, dut, baseConfig)

	for _, input := range testEnabledInput {
		t.Run(fmt.Sprintf("Testing /sampling/sflow/config/enabled using value %v", input), func(t *testing.T) {

			config := dut.Config().Sampling().Sflow().Enabled()
			state := dut.Telemetry().Sampling().Sflow().Enabled()

			t.Run("Replace leaf", func(t *testing.T) {
				config.Replace(t, input)
			})
			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot != input {
						t.Errorf("Config /sampling/sflow/config/enabled: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot != input {
						t.Errorf("State /sampling/sflow/config/enabled: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete leaf", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs != nil {
						t.Errorf("Delete /sampling/sflow/config/enabled fail: got %v", qs)
					}
				}
			})
		})
	}
}

func TestSampleSizeAtContainer(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Sampling = setupSampling(t, dut)
	defer teardownSampling(t, dut, baseConfig)

	for _, input := range testSampleSizeInput {
		t.Run(fmt.Sprintf("Testing /sampling/sflow/config/sample-size using value %v", input), func(t *testing.T) {
			baseConfigSflow := baseConfig.Sflow
			*baseConfigSflow.SampleSize = input

			config := dut.Config().Sampling().Sflow()
			state := dut.Telemetry().Sampling().Sflow()

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigSflow)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.SampleSize != input {
						t.Errorf("Config /sampling/sflow/config/sample-size: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.SampleSize != input {
						t.Errorf("State /sampling/sflow/config/sample-size: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete container", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).SampleSize != nil {
						t.Errorf("Delete /sampling/sflow/config/sample-size fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestSampleSizeAtLeaf(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupSampling(t, dut)
	defer teardownSampling(t, dut, baseConfig)

	for _, input := range testSampleSizeInput {
		t.Run(fmt.Sprintf("Testing /sampling/sflow/config/sample-size using value %v", input), func(t *testing.T) {

			config := dut.Config().Sampling().Sflow().SampleSize()
			state := dut.Telemetry().Sampling().Sflow().SampleSize()

			t.Run("Replace leaf", func(t *testing.T) {
				config.Replace(t, input)
			})
			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot != input {
						t.Errorf("Config /sampling/sflow/config/sample-size: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot != input {
						t.Errorf("State /sampling/sflow/config/sample-size: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete leaf", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs != nil {
						t.Errorf("Delete /sampling/sflow/config/sample-size fail: got %v", qs)
					}
				}
			})
		})
	}
}
