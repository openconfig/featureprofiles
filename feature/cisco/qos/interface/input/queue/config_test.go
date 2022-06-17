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

func TestNameAtContainer(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testNameInput {
		t.Run(fmt.Sprintf("Testing /qos/interfaces/interface/input/queues/queue/config/name using value %v", input), func(t *testing.T) {
			baseConfigInterface := setup.GetAnyValue(baseConfig.Interface)
			baseConfigInterfaceInput := baseConfigInterface.Input
			baseConfigInterfaceInputQueue := setup.GetAnyValue(baseConfigInterfaceInput.Queue)
			*baseConfigInterfaceInputQueue.Name = input

			config := dut.Config().Qos().Interface(*baseConfigInterface.InterfaceId).Input().Queue(*baseConfigInterfaceInputQueue.Name)
			state := dut.Telemetry().Qos().Interface(*baseConfigInterface.InterfaceId).Input().Queue(*baseConfigInterfaceInputQueue.Name)

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigInterfaceInputQueue)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.Name != input {
						t.Errorf("Config /qos/interfaces/interface/input/queues/queue/config/name: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.Name != input {
						t.Errorf("State /qos/interfaces/interface/input/queues/queue/config/name: got %v, want %v", stateGot, input)
					}
				})
			}
		})
	}
}
func TestQueueManagementProfileAtContainer(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testQueueManagementProfileInput {
		t.Run(fmt.Sprintf("Testing /qos/interfaces/interface/input/queues/queue/config/queue-management-profile using value %v", input), func(t *testing.T) {
			baseConfigInterface := setup.GetAnyValue(baseConfig.Interface)
			baseConfigInterfaceInput := baseConfigInterface.Input
			baseConfigInterfaceInputQueue := setup.GetAnyValue(baseConfigInterfaceInput.Queue)
			*baseConfigInterfaceInputQueue.QueueManagementProfile = input

			config := dut.Config().Qos().Interface(*baseConfigInterface.InterfaceId).Input().Queue(*baseConfigInterfaceInputQueue.Name)
			state := dut.Telemetry().Qos().Interface(*baseConfigInterface.InterfaceId).Input().Queue(*baseConfigInterfaceInputQueue.Name)

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigInterfaceInputQueue)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.QueueManagementProfile != input {
						t.Errorf("Config /qos/interfaces/interface/input/queues/queue/config/queue-management-profile: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.QueueManagementProfile != input {
						t.Errorf("State /qos/interfaces/interface/input/queues/queue/config/queue-management-profile: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete container", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).QueueManagementProfile != nil {
						t.Errorf("Delete /qos/interfaces/interface/input/queues/queue/config/queue-management-profile fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestQueueManagementProfileAtLeaf(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testQueueManagementProfileInput {
		t.Run(fmt.Sprintf("Testing /qos/interfaces/interface/input/queues/queue/config/queue-management-profile using value %v", input), func(t *testing.T) {
			baseConfigInterface := setup.GetAnyValue(baseConfig.Interface)
			baseConfigInterfaceInputQueue := setup.GetAnyValue(baseConfigInterface.Input.Queue)

			config := dut.Config().Qos().Interface(*baseConfigInterface.InterfaceId).Input().Queue(*baseConfigInterfaceInputQueue.Name).QueueManagementProfile()
			state := dut.Telemetry().Qos().Interface(*baseConfigInterface.InterfaceId).Input().Queue(*baseConfigInterfaceInputQueue.Name).QueueManagementProfile()

			t.Run("Replace leaf", func(t *testing.T) {
				config.Replace(t, input)
			})
			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot != input {
						t.Errorf("Config /qos/interfaces/interface/input/queues/queue/config/queue-management-profile: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot != input {
						t.Errorf("State /qos/interfaces/interface/input/queues/queue/config/queue-management-profile: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete leaf", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs != nil {
						t.Errorf("Delete /qos/interfaces/interface/input/queues/queue/config/queue-management-profile fail: got %v", qs)
					}
				}
			})
		})
	}
}
