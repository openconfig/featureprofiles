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
		t.Run(fmt.Sprintf("Testing /sampling/sflow/config/enabled using value %v", input), func(t *testing.T) {
			baseConfigSflow := baseConfig.Sflow
			*baseConfigSflow.Enabled = input

			config := gnmi.OC().Sampling().Sflow()
			state := gnmi.OC().Sampling().Sflow()

			t.Run("Replace container", func(t *testing.T) {
				gnmi.Replace(t, dut, config.Config(), baseConfigSflow)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := gnmi.GetConfig(t, dut, config.Config())
					if *configGot.Enabled != input {
						t.Errorf("Config /sampling/sflow/config/enabled: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := gnmi.Get(t, dut, state.State())
					if *stateGot.Enabled != input {
						t.Errorf("State /sampling/sflow/config/enabled: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete container", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				if !setup.SkipSubscribe() {
					if qs := gnmi.LookupConfig(t, dut, config.Config()); qs.IsPresent() == true {
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

			config := gnmi.OC().Sampling().Sflow().Enabled()
			state := gnmi.OC().Sampling().Sflow().Enabled()

			t.Run("Replace leaf", func(t *testing.T) {
				gnmi.Replace(t, dut, config.Config(), input)
			})
			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := gnmi.GetConfig(t, dut, config.Config())
					if configGot != input {
						t.Errorf("Config /sampling/sflow/config/enabled: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := gnmi.Get(t, dut, state.State())
					if stateGot != input {
						t.Errorf("State /sampling/sflow/config/enabled: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete leaf", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				if !setup.SkipSubscribe() {
					if qs := gnmi.LookupConfig(t, dut, config.Config()); qs.IsPresent() == true {
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

			config := gnmi.OC().Sampling().Sflow()
			state := gnmi.OC().Sampling().Sflow()

			t.Run("Replace container", func(t *testing.T) {
				gnmi.Replace(t, dut, config.Config(), baseConfigSflow)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := gnmi.GetConfig(t, dut, config.Config())
					if *configGot.SampleSize != input {
						t.Errorf("Config /sampling/sflow/config/sample-size: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := gnmi.Get(t, dut, state.State())
					if *stateGot.SampleSize != input {
						t.Errorf("State /sampling/sflow/config/sample-size: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete container", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				if !setup.SkipSubscribe() {
					if qs := gnmi.LookupConfig(t, dut, config.Config()); qs.IsPresent() == true {
						t.Errorf("Delete /sampling/sflow/config/sample-size fail: got %v", qs.IsPresent())
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

			config := gnmi.OC().Sampling().Sflow().SampleSize()
			state := gnmi.OC().Sampling().Sflow().SampleSize()

			t.Run("Replace leaf", func(t *testing.T) {
				gnmi.Replace(t, dut, config.Config(), input)
			})
			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := gnmi.GetConfig(t, dut, config.Config())
					if configGot != input {
						t.Errorf("Config /sampling/sflow/config/sample-size: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := gnmi.Get(t, dut, state.State())
					if stateGot != input {
						t.Errorf("State /sampling/sflow/config/sample-size: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete leaf", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				if !setup.SkipSubscribe() {
					if qs := gnmi.LookupConfig(t, dut, config.Config()); qs.IsPresent() == true {
						t.Errorf("Delete /sampling/sflow/config/sample-size fail: got %v", qs)
					}
				}
			})
		})
	}
}

// /sampling/sflow/config/agent-id-ipv4
func TestAgentIdIpv4(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupSampling(t, dut)
	defer teardownSampling(t, dut, baseConfig)

	for _, input := range testAgentIdv4Input {
		t.Run(fmt.Sprintf("Testing /sampling/sflow/config/agent-id-ipv4 using value %v", input), func(t *testing.T) {

			config := gnmi.OC().Sampling().Sflow().AgentIdIpv4()
			state := gnmi.OC().Sampling().Sflow().AgentIdIpv4()

			t.Run("Replace leaf", func(t *testing.T) {
				gnmi.Replace(t, dut, config.Config(), input)
			})

			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := gnmi.GetConfig(t, dut, config.Config())
					if configGot != input {
						t.Errorf("Config /sampling/sflow/config/agent-id-ipv4: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := gnmi.Get(t, dut, state.State())
					if stateGot != input {
						t.Errorf("State /sampling/sflow/config/agent-id-ipv4: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Update leaf", func(t *testing.T) {
				gnmi.Update(t, dut, config.Config(), input)
			})
			t.Run("Delete leaf", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				if !setup.SkipSubscribe() {
					if qs := gnmi.LookupConfig(t, dut, config.Config()); qs.IsPresent() == true {
						t.Errorf("Delete /sampling/sflow/config/agent-id-ipv4 fail: got %v", qs)
					}
				}
			})
		})
	}
}

// /sampling/sflow/config/agent-id-ipv6
func TestAgentIdIpv6(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupSampling(t, dut)
	defer teardownSampling(t, dut, baseConfig)

	for _, input := range testAgentIdv6Input {
		t.Run(fmt.Sprintf("Testing /sampling/sflow/config/agent-id-ipv6 using value %v", input), func(t *testing.T) {

			config := gnmi.OC().Sampling().Sflow().AgentIdIpv6()
			state := gnmi.OC().Sampling().Sflow().AgentIdIpv6()

			t.Run("Replace leaf", func(t *testing.T) {
				gnmi.Replace(t, dut, config.Config(), input)
			})
			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := gnmi.GetConfig(t, dut, config.Config())
					if configGot != input {
						t.Errorf("Config /sampling/sflow/config/agent-id-ipv6: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := gnmi.Get(t, dut, state.State())
					if stateGot != input {
						t.Errorf("State /sampling/sflow/config/agent-id-ipv6: got %v, want %v", stateGot, input)
					}
				})
			}

			t.Run("Update leaf", func(t *testing.T) {
				gnmi.Update(t, dut, config.Config(), input)
			})

			t.Run("Delete leaf", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				if !setup.SkipSubscribe() {
					if qs := gnmi.LookupConfig(t, dut, config.Config()); qs.IsPresent() == true {
						t.Errorf("Delete /sampling/sflow/config/agent-id-ipv6 fail: got %v", qs)
					}
				}
			})
		})
	}
}

// /sampling/sflow/config/dscp
func TestDscp(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupSampling(t, dut)
	defer teardownSampling(t, dut, baseConfig)

	for _, input := range testDscpInput {
		t.Run(fmt.Sprintf("Testing /sampling/sflow/config/dscp using value %v", input), func(t *testing.T) {

			config := gnmi.OC().Sampling().Sflow().Dscp()
			state := gnmi.OC().Sampling().Sflow().Dscp()

			t.Run("Replace leaf", func(t *testing.T) {
				gnmi.Replace(t, dut, config.Config(), input)
			})
			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := gnmi.GetConfig(t, dut, config.Config())
					if configGot != input {
						t.Errorf("Config /sampling/sflow/config/dscp : got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := gnmi.Get(t, dut, state.State())
					if stateGot != input {
						t.Errorf("State /sampling/sflow/config/dscp : got %v, want %v", stateGot, input)
					}
				})
			}

			t.Run("Update leaf", func(t *testing.T) {
				gnmi.Update(t, dut, config.Config(), input)
			})
			t.Run("Delete leaf", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				if !setup.SkipSubscribe() {
					if qs := gnmi.LookupConfig(t, dut, config.Config()); qs.IsPresent() == true {
						t.Errorf("Delete /sampling/sflow/config/dscp fail: got %v", qs)
					}
				}
			})
		})
	}
}

// /sampling/sflow/config/polling-interval
func TestPollingInterval(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupSampling(t, dut)
	defer teardownSampling(t, dut, baseConfig)

	for _, input := range testPollingIntervalInput {
		t.Run(fmt.Sprintf("Testing /sampling/sflow/config/polling-interval using value %v", input), func(t *testing.T) {

			config := gnmi.OC().Sampling().Sflow().PollingInterval()
			state := gnmi.OC().Sampling().Sflow().PollingInterval()

			t.Run("Replace leaf", func(t *testing.T) {
				gnmi.Replace(t, dut, config.Config(), input)
			})
			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := gnmi.GetConfig(t, dut, config.Config())
					if configGot != input {
						t.Errorf("Config /sampling/sflow/config/polling-interval : got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := gnmi.Get(t, dut, state.State())
					if stateGot != input {
						t.Errorf("State /sampling/sflow/config/polling-interval : got %v, want %v", stateGot, input)
					}
				})
			}

			t.Run("Update leaf", func(t *testing.T) {
				gnmi.Update(t, dut, config.Config(), input)
			})
			t.Run("Delete leaf", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				if !setup.SkipSubscribe() {
					if qs := gnmi.LookupConfig(t, dut, config.Config()); qs.IsPresent() == true {
						t.Errorf("Delete /sampling/sflow/config/polling-interval fail: got %v", qs)
					}
				}
			})
		})
	}
}

// /sampling/sflow/config/ingress-sampling-rate
func TestIngressSamplingRate(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupSampling(t, dut)
	defer teardownSampling(t, dut, baseConfig)

	for _, input := range testIngressSamplingRate {
		t.Run(fmt.Sprintf("Testing /sampling/sflow/config/ingress-sampling-rate using value %v", input), func(t *testing.T) {

			config := gnmi.OC().Sampling().Sflow().IngressSamplingRate()
			state := gnmi.OC().Sampling().Sflow().IngressSamplingRate()

			t.Run("Replace leaf", func(t *testing.T) {
				gnmi.Replace(t, dut, config.Config(), input)
			})
			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := gnmi.GetConfig(t, dut, config.Config())
					if configGot != input {
						t.Errorf("Config /sampling/sflow/config/ingress-sampling-rate : got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := gnmi.Get(t, dut, state.State())
					if stateGot != input {
						t.Errorf("State /sampling/sflow/config/ingress-sampling-rate : got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Update leaf", func(t *testing.T) {
				gnmi.Update(t, dut, config.Config(), input)
			})
			t.Run("Delete leaf", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				if !setup.SkipSubscribe() {
					if qs := gnmi.LookupConfig(t, dut, config.Config()); qs.IsPresent() == true {
						t.Errorf("Delete /sampling/sflow/config/ingress-sampling-rate fail: got %v", qs)
					}
				}
			})
		})
	}
}

func TestStateLeafs(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupSampling(t, dut)
	defer teardownSampling(t, dut, baseConfig)
	state := gnmi.OC().Sampling().Sflow()
	t.Run("Subscribe Container level", func(t *testing.T) {
		gnmi.Get(t, dut, state.State())
	})
	t.Run("Subscribe Enabled", func(t *testing.T) {
		gnmi.Get(t, dut, state.Enabled().State())
	})
	t.Log("Watch on Enabled")
	_, ok := gnmi.Watch(t, dut, state.Enabled().State(), time.Minute, func(val *ygnmi.Value[bool]) bool {
		currState, ok := val.Val()
		return ok && currState == true
	}).Await(t)
	if !ok {
		t.Errorf("Enabled not true")
	}
	t.Run("Subscribe SampleSize", func(t *testing.T) {
		gnmi.Get(t, dut, state.SampleSize().State())
	})
	t.Log("Watch on SampleSize")
	_, ok = gnmi.Watch(t, dut, state.SampleSize().State(), time.Minute, func(val *ygnmi.Value[uint16]) bool {
		currState, ok := val.Val()
		return ok && currState == 128
	}).Await(t)
	if !ok {
		t.Errorf("SampleSize not correct")
	}
	t.Run("Subscribe Dscp", func(t *testing.T) {
		gnmi.Get(t, dut, state.Dscp().State())
	})
	t.Log("Watch on Dscp")
	_, ok = gnmi.Watch(t, dut, state.Dscp().State(), time.Minute, func(val *ygnmi.Value[uint8]) bool {
		currState, ok := val.Val()
		return ok && currState == 60
	}).Await(t)
	if !ok {
		t.Errorf("Dscp not correct")
	}
	t.Run("Subscribe PollingInterval", func(t *testing.T) {
		gnmi.Get(t, dut, state.PollingInterval().State())
	})
	t.Log("Watch on PollingInterval")
	_, ok = gnmi.Watch(t, dut, state.PollingInterval().State(), time.Minute, func(val *ygnmi.Value[uint16]) bool {
		currState, ok := val.Val()
		return ok && currState == 60
	}).Await(t)
	if !ok {
		t.Errorf("PollingInterval not correct")
	}
}
