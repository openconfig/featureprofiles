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

func TestUnicastOutputQueueAtContainer(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testUnicastOutputQueueInput {
		t.Run(fmt.Sprintf("Testing /qos/forwarding-groups/forwarding-group/config/unicast-output-queue using value %v", input), func(t *testing.T) {
			baseConfigForwardingGroup := setup.GetAnyValue(baseConfig.ForwardingGroup)
			*baseConfigForwardingGroup.UnicastOutputQueue = input

			config := dut.Config().Qos().ForwardingGroup(*baseConfigForwardingGroup.Name)
			state := dut.Telemetry().Qos().ForwardingGroup(*baseConfigForwardingGroup.Name)

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigForwardingGroup)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.UnicastOutputQueue != input {
						t.Errorf("Config /qos/forwarding-groups/forwarding-group/config/unicast-output-queue: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.UnicastOutputQueue != input {
						t.Errorf("State /qos/forwarding-groups/forwarding-group/config/unicast-output-queue: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete container", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).UnicastOutputQueue != nil {
						t.Errorf("Delete /qos/forwarding-groups/forwarding-group/config/unicast-output-queue fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestUnicastOutputQueueAtLeaf(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testUnicastOutputQueueInput {
		t.Run(fmt.Sprintf("Testing /qos/forwarding-groups/forwarding-group/config/unicast-output-queue using value %v", input), func(t *testing.T) {
			baseConfigForwardingGroup := setup.GetAnyValue(baseConfig.ForwardingGroup)

			config := dut.Config().Qos().ForwardingGroup(*baseConfigForwardingGroup.Name).UnicastOutputQueue()
			state := dut.Telemetry().Qos().ForwardingGroup(*baseConfigForwardingGroup.Name).UnicastOutputQueue()

			t.Run("Replace leaf", func(t *testing.T) {
				config.Replace(t, input)
			})
			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot != input {
						t.Errorf("Config /qos/forwarding-groups/forwarding-group/config/unicast-output-queue: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot != input {
						t.Errorf("State /qos/forwarding-groups/forwarding-group/config/unicast-output-queue: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete leaf", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs != nil {
						t.Errorf("Delete /qos/forwarding-groups/forwarding-group/config/unicast-output-queue fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestOutputQueueAtContainer(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testOutputQueueInput {
		t.Run(fmt.Sprintf("Testing /qos/forwarding-groups/forwarding-group/config/output-queue using value %v", input), func(t *testing.T) {
			baseConfigForwardingGroup := setup.GetAnyValue(baseConfig.ForwardingGroup)
			*baseConfigForwardingGroup.OutputQueue = input

			config := dut.Config().Qos().ForwardingGroup(*baseConfigForwardingGroup.Name)
			state := dut.Telemetry().Qos().ForwardingGroup(*baseConfigForwardingGroup.Name)

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigForwardingGroup)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.OutputQueue != input {
						t.Errorf("Config /qos/forwarding-groups/forwarding-group/config/output-queue: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.OutputQueue != input {
						t.Errorf("State /qos/forwarding-groups/forwarding-group/config/output-queue: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete container", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).OutputQueue != nil {
						t.Errorf("Delete /qos/forwarding-groups/forwarding-group/config/output-queue fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestOutputQueueAtLeaf(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testOutputQueueInput {
		t.Run(fmt.Sprintf("Testing /qos/forwarding-groups/forwarding-group/config/output-queue using value %v", input), func(t *testing.T) {
			baseConfigForwardingGroup := setup.GetAnyValue(baseConfig.ForwardingGroup)

			config := dut.Config().Qos().ForwardingGroup(*baseConfigForwardingGroup.Name).OutputQueue()
			state := dut.Telemetry().Qos().ForwardingGroup(*baseConfigForwardingGroup.Name).OutputQueue()

			t.Run("Replace leaf", func(t *testing.T) {
				config.Replace(t, input)
			})
			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot != input {
						t.Errorf("Config /qos/forwarding-groups/forwarding-group/config/output-queue: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot != input {
						t.Errorf("State /qos/forwarding-groups/forwarding-group/config/output-queue: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete leaf", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs != nil {
						t.Errorf("Delete /qos/forwarding-groups/forwarding-group/config/output-queue fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestNameAtContainer(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testNameInput {
		t.Run(fmt.Sprintf("Testing /qos/forwarding-groups/forwarding-group/config/name using value %v", input), func(t *testing.T) {
			baseConfigForwardingGroup := setup.GetAnyValue(baseConfig.ForwardingGroup)
			*baseConfigForwardingGroup.Name = input

			config := dut.Config().Qos().ForwardingGroup(*baseConfigForwardingGroup.Name)
			state := dut.Telemetry().Qos().ForwardingGroup(*baseConfigForwardingGroup.Name)

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigForwardingGroup)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.Name != input {
						t.Errorf("Config /qos/forwarding-groups/forwarding-group/config/name: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.Name != input {
						t.Errorf("State /qos/forwarding-groups/forwarding-group/config/name: got %v, want %v", stateGot, input)
					}
				})
			}
		})
	}
}
func TestFabricPriorityAtContainer(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testFabricPriorityInput {
		t.Run(fmt.Sprintf("Testing /qos/forwarding-groups/forwarding-group/config/fabric-priority using value %v", input), func(t *testing.T) {
			baseConfigForwardingGroup := setup.GetAnyValue(baseConfig.ForwardingGroup)
			*baseConfigForwardingGroup.FabricPriority = input

			config := dut.Config().Qos().ForwardingGroup(*baseConfigForwardingGroup.Name)
			state := dut.Telemetry().Qos().ForwardingGroup(*baseConfigForwardingGroup.Name)

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigForwardingGroup)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.FabricPriority != input {
						t.Errorf("Config /qos/forwarding-groups/forwarding-group/config/fabric-priority: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.FabricPriority != input {
						t.Errorf("State /qos/forwarding-groups/forwarding-group/config/fabric-priority: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete container", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).FabricPriority != nil {
						t.Errorf("Delete /qos/forwarding-groups/forwarding-group/config/fabric-priority fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestFabricPriorityAtLeaf(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testFabricPriorityInput {
		t.Run(fmt.Sprintf("Testing /qos/forwarding-groups/forwarding-group/config/fabric-priority using value %v", input), func(t *testing.T) {
			baseConfigForwardingGroup := setup.GetAnyValue(baseConfig.ForwardingGroup)

			config := dut.Config().Qos().ForwardingGroup(*baseConfigForwardingGroup.Name).FabricPriority()
			state := dut.Telemetry().Qos().ForwardingGroup(*baseConfigForwardingGroup.Name).FabricPriority()

			t.Run("Replace leaf", func(t *testing.T) {
				config.Replace(t, input)
			})
			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot != input {
						t.Errorf("Config /qos/forwarding-groups/forwarding-group/config/fabric-priority: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot != input {
						t.Errorf("State /qos/forwarding-groups/forwarding-group/config/fabric-priority: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete leaf", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs != nil {
						t.Errorf("Delete /qos/forwarding-groups/forwarding-group/config/fabric-priority fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestMulticastOutputQueueAtContainer(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testMulticastOutputQueueInput {
		t.Run(fmt.Sprintf("Testing /qos/forwarding-groups/forwarding-group/config/multicast-output-queue using value %v", input), func(t *testing.T) {
			baseConfigForwardingGroup := setup.GetAnyValue(baseConfig.ForwardingGroup)
			*baseConfigForwardingGroup.MulticastOutputQueue = input

			config := dut.Config().Qos().ForwardingGroup(*baseConfigForwardingGroup.Name)
			state := dut.Telemetry().Qos().ForwardingGroup(*baseConfigForwardingGroup.Name)

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigForwardingGroup)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.MulticastOutputQueue != input {
						t.Errorf("Config /qos/forwarding-groups/forwarding-group/config/multicast-output-queue: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.MulticastOutputQueue != input {
						t.Errorf("State /qos/forwarding-groups/forwarding-group/config/multicast-output-queue: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete container", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).MulticastOutputQueue != nil {
						t.Errorf("Delete /qos/forwarding-groups/forwarding-group/config/multicast-output-queue fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestMulticastOutputQueueAtLeaf(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testMulticastOutputQueueInput {
		t.Run(fmt.Sprintf("Testing /qos/forwarding-groups/forwarding-group/config/multicast-output-queue using value %v", input), func(t *testing.T) {
			baseConfigForwardingGroup := setup.GetAnyValue(baseConfig.ForwardingGroup)

			config := dut.Config().Qos().ForwardingGroup(*baseConfigForwardingGroup.Name).MulticastOutputQueue()
			state := dut.Telemetry().Qos().ForwardingGroup(*baseConfigForwardingGroup.Name).MulticastOutputQueue()

			t.Run("Replace leaf", func(t *testing.T) {
				config.Replace(t, input)
			})
			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot != input {
						t.Errorf("Config /qos/forwarding-groups/forwarding-group/config/multicast-output-queue: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot != input {
						t.Errorf("State /qos/forwarding-groups/forwarding-group/config/multicast-output-queue: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete leaf", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs != nil {
						t.Errorf("Delete /qos/forwarding-groups/forwarding-group/config/multicast-output-queue fail: got %v", qs)
					}
				}
			})
		})
	}
}
