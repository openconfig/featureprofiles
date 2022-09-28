// Copyright 2022 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package p4rt_oc_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/ygot/ygot"
)

func testNonExistingPortConfig(t *testing.T, args *testArgs) {
	portID := uint32(1000)
	configureP4RTIntf(t, args.dut, nonExistingPort, portID, telemetry.IETFInterfaces_InterfaceType_ethernetCsmacd)

	config := args.dut.Config().Interface(nonExistingPort).Id()
	defer observer.RecordYgot(t, "GET", config)

	if gotID := config.Get(t); gotID != portID {
		t.Fatalf("Interface port-id using GNMI Get on config: want %v, got %v", gotID, portID)
	}

	state := args.dut.Telemetry().Interface(nonExistingPort).Id()
	defer observer.RecordYgot(t, "GET", state)

	if gotID := state.Get(t); gotID != portID {
		t.Fatalf("Interface port-id using GNMI Get telemetry: want %v, got %v", gotID, portID)
	}
}

func testReconfigureP4RTWithPacketIOSessionOn(t *testing.T, args *testArgs) {
	// p4rt packetio session tests are covered in featureprofiles/feature/p4rt/ate_tests/cisco/
	// Reconfigure p4rt
	p1 := args.dut.Port(t, "port1")
	p2 := args.dut.Port(t, "port2")
	configureP4RTDevice(t, args.dut, npu0, deviceID)

	state := args.dut.Telemetry().Component(npu0).IntegratedCircuit().NodeId()
	defer observer.RecordYgot(t, "GET", state)

	if got := state.Get(t); got != deviceID {
		t.Fatalf("Device IDs: want %v , got %v", deviceID, got)
	}

	config := args.dut.Config().Interface(p1.Name()).Id()
	defer observer.RecordYgot(t, "REPLACE", config)

	config.Replace(t, 1)

	state1 := args.dut.Telemetry().Interface(p1.Name()).Id()
	defer observer.RecordYgot(t, "GET", state1)

	if got := state.Get(t); got != 1 {
		t.Fatalf("Interface port-id: want 1, got %v", got)
	}

	config = args.dut.Config().Interface(p2.Name()).Id()
	defer observer.RecordYgot(t, "REPLACE", config)

	config.Replace(t, 2)

	state1 = args.dut.Telemetry().Interface(p2.Name()).Id()
	defer observer.RecordYgot(t, "GET", state1)

	if got := state1.Get(t); got != 2 {
		t.Fatalf("Interface port-id: want 2, got %v", got)
	}
}

func testConfigDeviceIDPortIDWithInterfaceDown(t *testing.T, args *testArgs) {
	p1 := args.dut.Port(t, "port1")
	p2 := args.dut.Port(t, "port2")

	// shutdown interface
	args.dut.Config().Interface(p1.Name()).Enabled().Replace(t, false)
	defer args.dut.Config().Interface(p1.Name()).Enabled().Replace(t, true)

	config := args.dut.Config().Interface(p1.Name()).Id()
	defer observer.RecordYgot(t, "REPLACE", config)
	config.Replace(t, 1)

	state := args.dut.Telemetry().Interface(p1.Name()).Id()
	defer observer.RecordYgot(t, "GET", state)

	if got := state.Get(t); got != 1 {
		t.Fatalf("Interface port-id: want 1, got %v", got)
	}

	// shutdown interface
	args.dut.Config().Interface(p2.Name()).Enabled().Replace(t, false)
	defer args.dut.Config().Interface(p2.Name()).Enabled().Replace(t, true)

	config = args.dut.Config().Interface(p2.Name()).Id()
	defer observer.RecordYgot(t, "REPLACE", config)
	config.Replace(t, 2)

	state = args.dut.Telemetry().Interface(p2.Name()).Id()
	defer observer.RecordYgot(t, "GET", state)

	if got := state.Get(t); got != 2 {
		t.Fatalf("Interface port-id: want 2, got %v", got)
	}

	configureP4RTDevice(t, args.dut, npu0, deviceID)

	state1 := args.dut.Telemetry().Component(npu0).IntegratedCircuit().NodeId()
	defer observer.RecordYgot(t, "GET", state1)

	if got := state1.Get(t); got != deviceID {
		t.Fatalf("Device IDs: want %v , got %v", deviceID, got)
	}
}

func testP4RTConfigurationWithBundleInterface(t *testing.T, args *testArgs) {

	int1 := "Bundle-Ether120"
	int1ID := 120
	int2 := "Bundle-Ether121"
	int2ID := 121
	configureP4RTDevice(t, args.dut, npu0, deviceID)

	state := args.dut.Telemetry().Component(npu0).IntegratedCircuit().NodeId()
	defer observer.RecordYgot(t, "GET", state)

	if got := state.Get(t); got != deviceID {
		t.Fatalf("Device IDs: want %v , got %v", deviceID, got)
	}

	config := args.dut.Config().Interface(int1).Id()
	defer observer.RecordYgot(t, "REPLACE", config)
	config.Replace(t, uint32(int1ID))

	state1 := args.dut.Telemetry().Interface(int1).Id()
	defer observer.RecordYgot(t, "GET", state1)

	if got := state1.Get(t); got != uint32(int1ID) {
		t.Fatalf("Interface port-id: want %v, got %v", int1ID, got)
	}

	config = args.dut.Config().Interface(int2).Id()
	defer observer.RecordYgot(t, "REPLACE", config)
	config.Replace(t, uint32(int2ID))

	state1 = args.dut.Telemetry().Interface(int2).Id()
	defer observer.RecordYgot(t, "GET", state1)

	if got := state1.Get(t); got != uint32(int2ID) {
		t.Fatalf("Interface port-id: want %v, got %v", int2ID, got)
	}
}

func testP4RTConfigurationUsingGNMIUpdate(t *testing.T, args *testArgs) {

	p1 := args.dut.Port(t, "port1")
	// i1 := &telemetry.Interface{
	// 	Type: telemetry.IETFInterfaces_InterfaceType_ethernetCsmacd,
	// 	Id:   ygot.Uint32(1),
	// 	Name: ygot.String(p1.Name()),
	// }
	// args.dut.Config().Interface(p1.Name()).Update(t, i1)

	config := args.dut.Config().Interface(p1.Name()).Id()
	defer observer.RecordYgot(t, "UPDATE", config)
	config.Update(t, 1)

	state := args.dut.Telemetry().Interface(p1.Name()).Id()
	defer observer.RecordYgot(t, "GET", state)

	if got := state.Get(t); got != 1 {
		t.Fatalf("Interface port-id: want 1, got %v", got)
	}

	p2 := args.dut.Port(t, "port2")
	// i2 := &telemetry.Interface{
	// 	Type: telemetry.IETFInterfaces_InterfaceType_ethernetCsmacd,
	// 	Id:   ygot.Uint32(2),
	// 	Name: ygot.String(p2.Name()),
	// }
	// args.dut.Config().Interface(p2.Name()).Update(t, i2)

	config = args.dut.Config().Interface(p2.Name()).Id()
	defer observer.RecordYgot(t, "UPDATE", config)
	config.Update(t, 2)

	state = args.dut.Telemetry().Interface(p2.Name()).Id()
	defer observer.RecordYgot(t, "GET", state)

	if got := state.Get(t); got != 2 {
		t.Fatalf("Interface port-id: want 2, got %v", got)
	}

	ic := &telemetry.Component_IntegratedCircuit{}
	ic.NodeId = ygot.Uint64(deviceID)

	config1 := args.dut.Config().Component(npu0).IntegratedCircuit()
	defer observer.RecordYgot(t, "UPDATE", config1)
	config1.Update(t, ic)

	state1 := args.dut.Telemetry().Component(npu0).IntegratedCircuit().NodeId()
	defer observer.RecordYgot(t, "GET", state1)

	if got := state1.Get(t); got != deviceID {
		t.Fatalf("Device IDs: want %v , got %v", deviceID, got)
	}
}

func testP4RTConfigurationDelete(t *testing.T, args *testArgs) {
	p1 := args.dut.Port(t, "port1")
	p2 := args.dut.Port(t, "port2")

	configureP4RTDevice(t, args.dut, npu0, deviceID)
	state := args.dut.Telemetry().Component(npu0).IntegratedCircuit().NodeId()
	defer observer.RecordYgot(t, "GET", state)

	if got := state.Get(t); got != deviceID {
		t.Fatalf("Device IDs: want %v , got %v", deviceID, got)
	}

	config := args.dut.Config().Interface(p1.Name()).Id()
	defer observer.RecordYgot(t, "REPLACE", config)
	config.Replace(t, 1)

	if got := config.Get(t); got != 1 {
		t.Fatalf("Interface port-id: want 1, got %v", got)
	}

	config1 := args.dut.Config().Interface(p2.Name()).Id()
	defer observer.RecordYgot(t, "REPLACE", config1)
	config1.Replace(t, 2)

	state1 := args.dut.Telemetry().Interface(p2.Name()).Id()
	defer observer.RecordYgot(t, "GET", state1)

	if got := state1.Get(t); got != 2 {
		t.Fatalf("Interface port-id: want 2, got %v", got)
	}

	//delete node-id and port-id
	defer observer.RecordYgot(t, "DELETE", config)
	config.Delete(t)
	defer args.dut.Config().Interface(p1.Name()).Id().Replace(t, 1)

	defer observer.RecordYgot(t, "DELETE", config1)
	config1.Delete(t)
	defer args.dut.Config().Interface(p2.Name()).Id().Replace(t, 2)

	config2 := args.dut.Config().Component(npu0).IntegratedCircuit().NodeId()
	defer observer.RecordYgot(t, "DELETE", config2)
	config2.Delete(t)

	defer configureP4RTDevice(t, args.dut, npu0, deviceID)
}

func testP4RTConfigurationUsingGetConfig(t *testing.T, args *testArgs) {
	p1 := args.dut.Port(t, "port1")
	p2 := args.dut.Port(t, "port2")
	configureP4RTDevice(t, args.dut, npu0, deviceID)
	config := args.dut.Config().Component(npu0).IntegratedCircuit().NodeId()
	defer observer.RecordYgot(t, "GET", config)

	if got := config.Get(t); got != deviceID {
		t.Fatalf("Device IDs: want %v , got %v", deviceID, got)
	}

	config1 := args.dut.Config().Interface(p1.Name()).Id()
	defer observer.RecordYgot(t, "REPLACE", config1)
	config1.Replace(t, 1)

	defer observer.RecordYgot(t, "GET", config1)
	if got := config1.Get(t); got != 1 {
		t.Fatalf("Interface port-id: want 1, got %v", got)
	}

	config2 := args.dut.Config().Interface(p2.Name()).Id()
	defer observer.RecordYgot(t, "REPLACE", config2)
	config2.Replace(t, 2)

	defer observer.RecordYgot(t, "GET", config2)
	if got := config2.Get(t); got != 2 {
		t.Fatalf("Interface port-id: want 2, got %v", got)
	}
}

func testP4RTTelemetry(t *testing.T, args *testArgs) {
	p1 := args.dut.Port(t, "port1")
	p2 := args.dut.Port(t, "port2")
	subscriptionDuration := 65 * time.Second
	expectedSamples := 2
	configureP4RTDevice(t, args.dut, npu0, deviceID)

	config := args.dut.Config().Interface(p1.Name()).Id()
	defer observer.RecordYgot(t, "REPLACE", config)
	config.Replace(t, 1)

	config1 := args.dut.Config().Interface(p2.Name()).Id()
	defer observer.RecordYgot(t, "REPLACE", config1)
	config1.Replace(t, 2)

	t.Run("Telemetry for P4RT node-id", func(t *testing.T) {
		t.Parallel()
		statePath := args.dut.Telemetry().Component(npu0).IntegratedCircuit().NodeId()
		defer observer.RecordYgot(t, "SUBSCRIBE", statePath)

		if got := statePath.Collect(t, subscriptionDuration).Await(t); len(got) < expectedSamples {
			t.Fatalf("P4RT node-id samples: got %d, want %d", len(got), expectedSamples)
		} else {
			t.Logf("Collected P4RT node-id Samples :\n%v", got)
		}
	})

	t.Run(fmt.Sprintf("Telemetry for P4RT Interface port-id for %v", p1.Name()), func(t *testing.T) {
		t.Parallel()
		statePath := args.dut.Telemetry().Interface(p1.Name()).Id()
		defer observer.RecordYgot(t, "SUBSCRIBE", statePath)

		if got := statePath.Collect(t, subscriptionDuration).Await(t); len(got) < expectedSamples {
			t.Fatalf("P4RT Interface port-id samples: got %d, want %d", len(got), expectedSamples)
		} else {
			t.Logf("Collected P4RT Interface port-id samples :\n%v", got)
		}
	})

	t.Run(fmt.Sprintf("Telemetry for P4RT Interface port-id for %v", p2.Name()), func(t *testing.T) {
		t.Parallel()
		statePath := args.dut.Telemetry().Interface(p2.Name()).Id()
		defer observer.RecordYgot(t, "SUBSCRIBE", statePath)

		if got := statePath.Collect(t, subscriptionDuration).Await(t); len(got) < expectedSamples {
			t.Fatalf("P4RT Interface port-id samples: got %d, want %d", len(got), expectedSamples)
		} else {
			t.Logf("Collected P4RT Interface port-id samples :\n%v", got)
		}
	})
}

func testP4RTUprev(t *testing.T, args *testArgs) {
	p1 := args.dut.Port(t, "port1")
	for _, portID := range []uint32{4294967039, 1} {

		config := args.dut.Config().Interface(p1.Name()).Id()
		defer observer.RecordYgot(t, "REPLACE", config)
		config.Replace(t, portID)

		// Once defect is fixed move this to Get on Config.
		defer observer.RecordYgot(t, "GET", config)
		if got := config.Get(t); got != portID {
			t.Fatalf("Interface port-id: want 1, got %v", got)
		}

	}
}
