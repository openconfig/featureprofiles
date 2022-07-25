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

func TestInterfaceOutputTelemetry(t *testing.T) {
	// /qos/interfaces/interface/
	// // /qos/interfaces/interface/output/
	// /qos/interfaces/interface/output/queues/queue/
	// /qos/interfaces/interface/output/queues/queue/state/transmit-octets
	// /qos/interfaces/interface/output/queues/queue/state/transmit-pkts
	// /qos/interfaces/interface/output/queues/queue/state/dropped-pkts

	dut := ondatra.DUT(t, "dut")
	// dut.Config().Qos().Delete(t)
	// var baseConfig *oc.Qos = setupQos(t, dut, "base_config_interface_egress1.json")
	var baseConfig *oc.Qos = setup.BaseConfig("base_config_interface_egress1.json")
	// defer deleteQueues(t, dut, baseConfig)
	// defer teardownQos(t, dut, baseConfig)

	baseConfigInterface := setup.GetAnyValue(baseConfig.Interface)
	interfaceTelemetryPath := dut.Telemetry().Qos().Interface(*baseConfigInterface.InterfaceId)

	t.Run(fmt.Sprintf("Get Interface Telemetry %s", *baseConfigInterface.InterfaceId), func(t *testing.T) {
		got := interfaceTelemetryPath.Get(t)
		for queueName, queue := range got.Output.Queue {
			t.Run(fmt.Sprintf("Verify Transmit-Octets of %s", queueName), func(t *testing.T) {
				if !(*queue.TransmitOctets == 0) {
					t.Errorf("Get Interface Telemetry fail: got %+v", *got)
				}
			})
			t.Run(fmt.Sprintf("Verify Transmit-Packets of %s", queueName), func(t *testing.T) {
				if !(*queue.TransmitPkts == 0) {
					t.Errorf("Get Interface Telemetry fail: got %+v", *got)
				}
			})
			t.Run(fmt.Sprintf("Verify Dropped-Packets of %s", queueName), func(t *testing.T) {
				if !(*queue.DroppedPkts == 0) {
					t.Errorf("Get Interface Telemetry fail: got %+v", *got)
				}
			})
		}
	})

	baseConfigInterfaceOutput := baseConfigInterface.Output
	interfaceOutputTelemetryPath := interfaceTelemetryPath.Output()

	baseConfigInterfaceOutputSchedulerPolicy := baseConfigInterfaceOutput.SchedulerPolicy
	baseConfigSchedulerPolicy := baseConfig.SchedulerPolicy[*baseConfigInterfaceOutputSchedulerPolicy.Name]
	baseConfigSchedulerPolicyScheduler := setup.GetAnyValue(baseConfigSchedulerPolicy.Scheduler)
	baseConfigSchedulerPolicySchedulerInput := setup.GetAnyValue(baseConfigSchedulerPolicyScheduler.Input)
	ocQueueName := *baseConfigSchedulerPolicySchedulerInput.Queue
	interfaceOutputQueueTelemetryPath := interfaceOutputTelemetryPath.Queue(ocQueueName)

	t.Run(fmt.Sprintf("Get Interface Output Queue Telemetry %s %s", *baseConfigInterface.InterfaceId, ocQueueName), func(t *testing.T) {
		got := interfaceOutputQueueTelemetryPath.Get(t)
		t.Run("Verify Transmit-Octets", func(t *testing.T) {
			if !(*got.TransmitOctets == 0) {
				t.Errorf("Get Interface Output Queue Telemetry fail: got %+v", *got)
			}
		})
		t.Run("Verify Transmit-Packets", func(t *testing.T) {
			if !(*got.TransmitPkts == 0) {
				t.Errorf("Get Interface Output Queue Telemetry fail: got %+v", *got)
			}
		})
		t.Run("Verify Dropped-Packets", func(t *testing.T) {
			if !(*got.DroppedPkts == 0) {
				t.Errorf("Get Interface Output Queue Telemetry fail: got %+v", *got)
			}
		})
	})

	transmitPacketsPath := interfaceOutputQueueTelemetryPath.TransmitPkts()
	transmitOctetsPath := interfaceOutputQueueTelemetryPath.TransmitOctets()
	droppedPacketsPath := interfaceOutputQueueTelemetryPath.DroppedPkts()

	t.Run("Get Transmit-Packets", func(t *testing.T) {
		transmitPackets := transmitPacketsPath.Get(t)
		if transmitPackets != 0 {
			t.Errorf("Get Transmit-Packets fail: got %v", transmitPackets)
		}
	})
	t.Run("Get Transmit-Octets", func(t *testing.T) {
		transmitOctets := transmitOctetsPath.Get(t)
		if transmitOctets != 0 {
			t.Errorf("Get Transmit-Octets fail: got %v", transmitOctets)
		}
	})
	t.Run("Get Dropped-Packets", func(t *testing.T) {
		droppedPackets := droppedPacketsPath.Get(t)
		if droppedPackets != 0 {
			t.Errorf("Get Dropped-Packets fail: got %v", droppedPackets)
		}
	})
}
