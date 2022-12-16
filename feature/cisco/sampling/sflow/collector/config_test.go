package sampling_test

import (
	"fmt"
	"testing"

	"github.com/openconfig/featureprofiles/feature/cisco/sampling/setup"
	"github.com/openconfig/featureprofiles/topologies/binding"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	oc "github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/ygnmi/ygnmi"
)

func TestMain(m *testing.M) {
	ondatra.RunTests(m, binding.New)
}

func TestNetworkInstanceAtContainer(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Sampling = setupSampling(t, dut)
	defer teardownSampling(t, dut, baseConfig)

	for _, input := range testNetworkInstanceInput {
		t.Run(fmt.Sprintf("Testing /sampling/sflow/collectors/collector/config/network-instance using value %v", input), func(t *testing.T) {
			baseConfigSflow := baseConfig.Sflow
			baseConfigSflowCollector := setup.GetAnyValue(baseConfigSflow.Collector)
			*baseConfigSflowCollector.NetworkInstance = input

			config := gnmi.OC().Sampling().Sflow().Collector(*baseConfigSflowCollector.Address, *baseConfigSflowCollector.Port)
			state := gnmi.OC().Sampling().Sflow().Collector(*baseConfigSflowCollector.Address, *baseConfigSflowCollector.Port)

			t.Run("Replace container", func(t *testing.T) {
				gnmi.Replace(t, dut, config.Config(), baseConfigSflowCollector)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := gnmi.GetConfig(t, dut, config.Config())
					if *configGot.NetworkInstance != input {
						t.Errorf("Config /sampling/sflow/collectors/collector/config/network-instance: got %v, want %v", configGot, input)
					}
				})
			}
			t.Run("Update container", func(t *testing.T) {
				gnmi.Update(t, dut, config.Config(), baseConfigSflowCollector)
			})

			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := gnmi.Get(t, dut, state.State())
					if *stateGot.NetworkInstance != input {
						t.Errorf("State /sampling/sflow/collectors/collector/config/network-instance: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete container", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				if !setup.SkipSubscribe() {
					if qs := gnmi.LookupConfig(t, dut, config.Config()); qs.Val(t).NetworkInstance != nil {
						t.Errorf("Delete /sampling/sflow/collectors/collector/config/network-instance fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestNetworkInstanceAtLeaf(t *testing.T) {
	t.Skip()
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupSampling(t, dut)
	defer teardownSampling(t, dut, baseConfig)

	for _, input := range testNetworkInstanceInput {
		t.Run(fmt.Sprintf("Testing /sampling/sflow/collectors/collector/config/network-instance using value %v", input), func(t *testing.T) {
			baseConfigSflowCollector := setup.GetAnyValue(baseConfig.Sflow.Collector)

			config := gnmi.OC().Sampling().Sflow().Collector(*baseConfigSflowCollector.Address, *baseConfigSflowCollector.Port).NetworkInstance()
			state := gnmi.OC().Sampling().Sflow().Collector(*baseConfigSflowCollector.Address, *baseConfigSflowCollector.Port).NetworkInstance()

			t.Run("Replace leaf", func(t *testing.T) {
				gnmi.Replace(t, dut, config.Config(), input)
			})
			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := gnmi.GetConfig(t, dut, config.Config())
					if configGot != input {
						t.Errorf("Config /sampling/sflow/collectors/collector/config/network-instance: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := gnmi.Get(t, dut, state.State())
					if stateGot != input {
						t.Errorf("State /sampling/sflow/collectors/collector/config/network-instance: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete leaf", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				if !setup.SkipSubscribe() {
					if qs := gnmi.LookupConfig(t, dut, config.Config()); qs != nil {
						t.Errorf("Delete /sampling/sflow/collectors/collector/config/network-instance fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestPortAtContainer(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Sampling = setupSampling(t, dut)
	defer teardownSampling(t, dut, baseConfig)

	for _, input := range testPortInput {
		t.Run(fmt.Sprintf("Testing /sampling/sflow/collectors/collector/config/port using value %v", input), func(t *testing.T) {
			baseConfigSflow := baseConfig.Sflow
			baseConfigSflowCollector := setup.GetAnyValue(baseConfigSflow.Collector)
			*baseConfigSflowCollector.Port = input

			config := gnmi.OC().Sampling().Sflow().Collector(*baseConfigSflowCollector.Address, *baseConfigSflowCollector.Port)
			state := gnmi.OC().Sampling().Sflow().Collector(*baseConfigSflowCollector.Address, *baseConfigSflowCollector.Port)

			t.Run("Replace container", func(t *testing.T) {
				gnmi.Replace(t, dut, config.Config(), baseConfigSflowCollector)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := gnmi.GetConfig(t, dut, config.Config())
					if *configGot.Port != input {
						t.Errorf("Config /sampling/sflow/collectors/collector/config/port: got %v, want %v", configGot, input)
					}
				})
			}
			t.Run("Update container", func(t *testing.T) {
				gnmi.Update(t, dut, config.Config(), baseConfigSflowCollector)
			})
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := gnmi.Get(t, dut, state.State())
					if *stateGot.Port != input {
						t.Errorf("State /sampling/sflow/collectors/collector/config/port: got %v, want %v", stateGot, input)
					}
				})
			}
		})
	}
}
func TestAddressAtContainer(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Sampling = setupSampling(t, dut)
	defer teardownSampling(t, dut, baseConfig)

	for _, input := range testAddressInput {
		t.Run(fmt.Sprintf("Testing /sampling/sflow/collectors/collector/config/address using value %v", input), func(t *testing.T) {
			baseConfigSflow := baseConfig.Sflow
			baseConfigSflowCollector := setup.GetAnyValue(baseConfigSflow.Collector)
			*baseConfigSflowCollector.Address = input

			config := gnmi.OC().Sampling().Sflow().Collector(*baseConfigSflowCollector.Address, *baseConfigSflowCollector.Port)
			state := gnmi.OC().Sampling().Sflow().Collector(*baseConfigSflowCollector.Address, *baseConfigSflowCollector.Port)

			t.Run("Replace container", func(t *testing.T) {
				gnmi.Replace(t, dut, config.Config(), baseConfigSflowCollector)
			})

			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := gnmi.GetConfig(t, dut, config.Config())
					if *configGot.Address != input {
						t.Errorf("Config /sampling/sflow/collectors/collector/config/address: got %v, want %v", configGot, input)
					}
				})
			}
			t.Run("Update container", func(t *testing.T) {
				gnmi.Update(t, dut, config.Config(), baseConfigSflowCollector)
			})
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := gnmi.Get(t, dut, state.State())
					if *stateGot.Address != input {
						t.Errorf("State /sampling/sflow/collectors/collector/config/address: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Subscribe Address", func(t *testing.T) {
				gnmi.Get(t, dut, state.Address().State())
			})
			t.Run("Subscribe Port", func(t *testing.T) {
				gnmi.Get(t, dut, state.Port().State())
			})
			t.Run("Subscribe NetworkInstance", func(t *testing.T) {
				gnmi.Get(t, dut, state.NetworkInstance().State())
			})
		})
	}
}
