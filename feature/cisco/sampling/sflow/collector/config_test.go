package sampling_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/openconfig/featureprofiles/feature/cisco/sampling/setup"
	"github.com/openconfig/featureprofiles/topologies/binding"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
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
					if qs := gnmi.LookupConfig(t, dut, config.Config()); qs.IsPresent() == true {
						t.Errorf("Delete /sampling/sflow/collectors/collector/config/network-instance fail: got %v", qs.IsPresent())
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

// /sampling/sflow/collectors/collector/config/source-address
func TestSourceAddressAtLeaf(t *testing.T) {
	t.Skip()
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupSampling(t, dut)
	defer teardownSampling(t, dut, baseConfig)

	for _, input := range testSourceAddressInput {
		t.Run(fmt.Sprintf("Testing /sampling/sflow/collectors/collector/config/source-address using value %v", input), func(t *testing.T) {
			baseConfigSflowCollector := setup.GetAnyValue(baseConfig.Sflow.Collector)

			config := gnmi.OC().Sampling().Sflow().Collector(*baseConfigSflowCollector.Address, *baseConfigSflowCollector.Port).SourceAddress()
			state := gnmi.OC().Sampling().Sflow().Collector(*baseConfigSflowCollector.Address, *baseConfigSflowCollector.Port).SourceAddress()

			t.Run("Replace leaf", func(t *testing.T) {
				gnmi.Replace(t, dut, config.Config(), input)
			})

			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := gnmi.GetConfig(t, dut, config.Config())
					if configGot != input {
						t.Errorf("Config /sampling/sflow/collectors/collector/config/source-address: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := gnmi.Get(t, dut, state.State())
					if stateGot != input {
						t.Errorf("State /sampling/sflow/collectors/collector/config/source-address: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete leaf", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				if !setup.SkipSubscribe() {
					if qs := gnmi.LookupConfig(t, dut, config.Config()); qs != nil {
						t.Errorf("Delete /sampling/sflow/collectors/collector/config/source-address fail: got %v", qs)
					}
				}
			})

		})
	}
}

func TestSourceAddressAtContainer(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Sampling = setupSampling(t, dut)
	defer teardownSampling(t, dut, baseConfig)

	for _, input := range testSourceAddressInput {
		t.Run(fmt.Sprintf("Testing /sampling/sflow/collectors/collector/config/source-address using value %v", input), func(t *testing.T) {
			baseConfigSflow := baseConfig.Sflow
			baseConfigSflowCollector := setup.GetAnyValue(baseConfigSflow.Collector)
			*baseConfigSflowCollector.SourceAddress = input
			config := gnmi.OC().Sampling().Sflow().Collector(*baseConfigSflowCollector.Address, *baseConfigSflowCollector.Port)
			state := gnmi.OC().Sampling().Sflow().Collector(*baseConfigSflowCollector.Address, *baseConfigSflowCollector.Port)
			if strings.Contains(input, ":") {
				config = gnmi.OC().Sampling().Sflow().Collector("2000::2", *baseConfigSflowCollector.Port)
				state = gnmi.OC().Sampling().Sflow().Collector("2000::2", *baseConfigSflowCollector.Port)
				*baseConfigSflowCollector.Address = "2000::2"
			}
			if strings.Contains(input, ".") {
				config = gnmi.OC().Sampling().Sflow().Collector("2.2.2.2", *baseConfigSflowCollector.Port)
				state = gnmi.OC().Sampling().Sflow().Collector("2.2.2.2", *baseConfigSflowCollector.Port)
				*baseConfigSflowCollector.Address = "2.2.2.2"
			}
			t.Run("Replace container", func(t *testing.T) {
				gnmi.Replace(t, dut, config.Config(), baseConfigSflowCollector)
			})

			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := gnmi.GetConfig(t, dut, config.Config())
					if *configGot.SourceAddress != input {
						t.Errorf("Config /sampling/sflow/collectors/collector/config/source-address: got %v, want %v", configGot, input)
					}
				})
			}
			t.Run("Update container", func(t *testing.T) {
				gnmi.Update(t, dut, config.Config(), baseConfigSflowCollector)
			})
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := gnmi.Get(t, dut, state.State())
					if *stateGot.SourceAddress != input {
						t.Errorf("State /sampling/sflow/collectors/collector/config/source-address: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Subscribe Source Address", func(t *testing.T) {
				gnmi.Get(t, dut, state.SourceAddress().State())
			})

		})
	}
}

func TestCollectorStateLeafs(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupSampling(t, dut)
	defer teardownSampling(t, dut, baseConfig)
	baseConfigSflow := baseConfig.Sflow
	baseConfigSflowCollector := setup.GetAnyValue(baseConfigSflow.Collector)
	state := gnmi.OC().Sampling().Sflow().Collector(*baseConfigSflowCollector.Address, *baseConfigSflowCollector.Port)

	t.Run("Subscribe Container level", func(t *testing.T) {
		gnmi.Get(t, dut, state.State())
	})
	t.Run("Subscribe Enabled", func(t *testing.T) {
		gnmi.Get(t, dut, state.Address().State())
	})
	t.Run("Subscribe SampleSize", func(t *testing.T) {
		gnmi.Get(t, dut, state.NetworkInstance().State())
	})
	t.Run("Subscribe Dscp", func(t *testing.T) {
		gnmi.Get(t, dut, state.Port().State())
	})
	t.Run("Subscribe PollingInterval", func(t *testing.T) {
		gnmi.Get(t, dut, state.SourceAddress().State())
	})
}
