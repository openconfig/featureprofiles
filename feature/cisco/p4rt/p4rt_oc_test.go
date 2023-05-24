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

	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

func testNonExistingPortConfig(t *testing.T, args *testArgs) {
	portID := uint32(1000)
	configureP4RTIntf(t, args.dut, nonExistingPort, portID, oc.IETFInterfaces_InterfaceType_ethernetCsmacd)

	config := gnmi.OC().Interface(nonExistingPort).Id()
	defer observer.RecordYgot(t, "GET", config)

	if gotID := gnmi.GetConfig(t, args.dut, config.Config()); gotID != portID {
		t.Fatalf("Interface port-id using GNMI Get on config: want %v, got %v", gotID, portID)
	}

	state := gnmi.OC().Interface(nonExistingPort).Id()
	defer observer.RecordYgot(t, "GET", state)

	if gotID := gnmi.Get(t, args.dut, state.State()); gotID != portID {
		t.Fatalf("Interface port-id using GNMI Get telemetry: want %v, got %v", gotID, portID)
	}
}

func testReconfigureP4RTWithPacketIOSessionOn(t *testing.T, args *testArgs) {
	// p4rt packetio session tests are covered in featureprofiles/feature/p4rt/ate_tests/cisco/
	// Reconfigure p4rt
	p1 := args.dut.Port(t, "port1")
	p2 := args.dut.Port(t, "port2")
	configureP4RTDevice(t, args.dut, npu0, deviceID)

	state := gnmi.OC().Component(npu0).IntegratedCircuit().NodeId()
	defer observer.RecordYgot(t, "GET", state)

	if got := gnmi.Get(t, args.dut, state.State()); got != deviceID {
		t.Fatalf("Device IDs: want %v , got %v", deviceID, got)
	}

	config := gnmi.OC().Interface(p1.Name()).Id()
	defer observer.RecordYgot(t, "REPLACE", config)

	gnmi.Replace(t, args.dut, config.Config(), 1)

	state1 := gnmi.OC().Interface(p1.Name()).Id()
	defer observer.RecordYgot(t, "GET", state1)

	if got := gnmi.Get(t, args.dut, state.State()); got != 1 {
		t.Fatalf("Interface port-id: want 1, got %v", got)
	}

	config = gnmi.OC().Interface(p2.Name()).Id()
	defer observer.RecordYgot(t, "REPLACE", config)

	gnmi.Replace(t, args.dut, config.Config(), 2)

	state1 = gnmi.OC().Interface(p2.Name()).Id()
	defer observer.RecordYgot(t, "GET", state1)

	if got := gnmi.Get(t, args.dut, state1.State()); got != 2 {
		t.Fatalf("Interface port-id: want 2, got %v", got)
	}
}

func testConfigDeviceIDPortIDWithInterfaceDown(t *testing.T, args *testArgs) {
	p1 := args.dut.Port(t, "port1")
	p2 := args.dut.Port(t, "port2")

	// shutdown interface
	gnmi.Replace(t, args.dut, gnmi.OC().Interface(p1.Name()).Enabled().Config(), false)
	defer gnmi.Replace(t, args.dut, gnmi.OC().Interface(p1.Name()).Enabled().Config(), true)

	config := gnmi.OC().Interface(p1.Name()).Id()
	defer observer.RecordYgot(t, "REPLACE", config)
	gnmi.Replace(t, args.dut, config.Config(), 1)

	state := gnmi.OC().Interface(p1.Name()).Id()
	defer observer.RecordYgot(t, "GET", state)

	if got := gnmi.Get(t, args.dut, state.State()); got != 1 {
		t.Fatalf("Interface port-id: want 1, got %v", got)
	}

	// shutdown interface
	gnmi.Replace(t, args.dut, gnmi.OC().Interface(p2.Name()).Enabled().Config(), false)
	defer gnmi.Replace(t, args.dut, gnmi.OC().Interface(p2.Name()).Enabled().Config(), true)

	config = gnmi.OC().Interface(p2.Name()).Id()
	defer observer.RecordYgot(t, "REPLACE", config)
	gnmi.Replace(t, args.dut, config.Config(), 2)

	state = gnmi.OC().Interface(p2.Name()).Id()
	defer observer.RecordYgot(t, "GET", state)

	if got := gnmi.Get(t, args.dut, state.State()); got != 2 {
		t.Fatalf("Interface port-id: want 2, got %v", got)
	}

	configureP4RTDevice(t, args.dut, npu0, deviceID)

	state1 := gnmi.OC().Component(npu0).IntegratedCircuit().NodeId()
	defer observer.RecordYgot(t, "GET", state1)

	if got := gnmi.Get(t, args.dut, state1.State()); got != deviceID {
		t.Fatalf("Device IDs: want %v , got %v", deviceID, got)
	}
}

func testP4RTConfigurationWithBundleInterface(t *testing.T, args *testArgs) {

	int1 := "Bundle-Ether120"
	int1ID := 120
	int2 := "Bundle-Ether121"
	int2ID := 121
	configureP4RTDevice(t, args.dut, npu0, deviceID)

	state := gnmi.OC().Component(npu0).IntegratedCircuit().NodeId()
	defer observer.RecordYgot(t, "GET", state)

	if got := gnmi.Get(t, args.dut, state.State()); got != deviceID {
		t.Fatalf("Device IDs: want %v , got %v", deviceID, got)
	}

	config := gnmi.OC().Interface(int1).Id()
	defer observer.RecordYgot(t, "REPLACE", config)
	gnmi.Replace(t, args.dut, config.Config(), uint32(int1ID))

	state1 := gnmi.OC().Interface(int1).Id()
	defer observer.RecordYgot(t, "GET", state1)

	if got := gnmi.Get(t, args.dut, state1.State()); got != uint32(int1ID) {
		t.Fatalf("Interface port-id: want %v, got %v", int1ID, got)
	}

	config = gnmi.OC().Interface(int2).Id()
	defer observer.RecordYgot(t, "REPLACE", config)
	gnmi.Replace(t, args.dut, config.Config(), uint32(int2ID))

	state1 = gnmi.OC().Interface(int2).Id()
	defer observer.RecordYgot(t, "GET", state1)

	if got := gnmi.Get(t, args.dut, state1.State()); got != uint32(int2ID) {
		t.Fatalf("Interface port-id: want %v, got %v", int2ID, got)
	}
}

func testP4RTConfigurationUsingGNMIUpdate(t *testing.T, args *testArgs) {

	p1 := args.dut.Port(t, "port1")

	config := gnmi.OC().Interface(p1.Name()).Id()
	defer observer.RecordYgot(t, "UPDATE", config)
	gnmi.Update(t, args.dut, config.Config(), 1)

	state := gnmi.OC().Interface(p1.Name()).Id()
	defer observer.RecordYgot(t, "GET", state)

	if got := gnmi.Get(t, args.dut, state.State()); got != 1 {
		t.Fatalf("Interface port-id: want 1, got %v", got)
	}

	p2 := args.dut.Port(t, "port2")

	config = gnmi.OC().Interface(p2.Name()).Id()
	defer observer.RecordYgot(t, "UPDATE", config)
	gnmi.Update(t, args.dut, config.Config(), 2)

	state = gnmi.OC().Interface(p2.Name()).Id()
	defer observer.RecordYgot(t, "GET", state)

	if got := gnmi.Get(t, args.dut, state.State()); got != 2 {
		t.Fatalf("Interface port-id: want 2, got %v", got)
	}

	ic := &oc.Component_IntegratedCircuit{}
	ic.NodeId = ygot.Uint64(deviceID)

	config1 := gnmi.OC().Component(npu0).IntegratedCircuit()
	defer observer.RecordYgot(t, "UPDATE", config1)
	gnmi.Update(t, args.dut, config1.Config(), ic)

	state1 := gnmi.OC().Component(npu0).IntegratedCircuit().NodeId()
	defer observer.RecordYgot(t, "GET", state1)

	if got := gnmi.Get(t, args.dut, state1.State()); got != deviceID {
		t.Fatalf("Device IDs: want %v , got %v", deviceID, got)
	}
}

func testP4RTConfigurationDelete(t *testing.T, args *testArgs) {
	p1 := args.dut.Port(t, "port1")
	p2 := args.dut.Port(t, "port2")

	configureP4RTDevice(t, args.dut, npu0, deviceID)
	state := gnmi.OC().Component(npu0).IntegratedCircuit().NodeId()
	defer observer.RecordYgot(t, "GET", state)

	if got := gnmi.Get(t, args.dut, state.State()); got != deviceID {
		t.Fatalf("Device IDs: want %v , got %v", deviceID, got)
	}

	config := gnmi.OC().Interface(p1.Name()).Id()
	defer observer.RecordYgot(t, "REPLACE", config)
	gnmi.Replace(t, args.dut, config.Config(), 1)

	if got := gnmi.GetConfig(t, args.dut, config.Config()); got != 1 {
		t.Fatalf("Interface port-id: want 1, got %v", got)
	}

	config1 := gnmi.OC().Interface(p2.Name()).Id()
	defer observer.RecordYgot(t, "REPLACE", config1)
	gnmi.Replace(t, args.dut, config1.Config(), 2)

	state1 := gnmi.OC().Interface(p2.Name()).Id()
	defer observer.RecordYgot(t, "GET", state1)

	if got := gnmi.Get(t, args.dut, state1.State()); got != 2 {
		t.Fatalf("Interface port-id: want 2, got %v", got)
	}

	//delete node-id and port-id
	defer observer.RecordYgot(t, "DELETE", config)
	gnmi.Delete(t, args.dut, config.Config())
	defer gnmi.Replace(t, args.dut, gnmi.OC().Interface(p1.Name()).Id().Config(), 1)

	defer observer.RecordYgot(t, "DELETE", config1)
	gnmi.Delete(t, args.dut, config1.Config())
	defer gnmi.Replace(t, args.dut, gnmi.OC().Interface(p2.Name()).Id().Config(), 2)

	config2 := gnmi.OC().Component(npu0).IntegratedCircuit().NodeId()
	defer observer.RecordYgot(t, "DELETE", config2)
	gnmi.Delete(t, args.dut, config2.Config())

	defer configureP4RTDevice(t, args.dut, npu0, deviceID)
}

func testP4RTConfigurationUsingGetConfig(t *testing.T, args *testArgs) {
	p1 := args.dut.Port(t, "port1")
	p2 := args.dut.Port(t, "port2")
	configureP4RTDevice(t, args.dut, npu0, deviceID)
	config := gnmi.OC().Component(npu0).IntegratedCircuit().NodeId()
	defer observer.RecordYgot(t, "GET", config)

	if got := gnmi.GetConfig(t, args.dut, config.Config()); got != deviceID {
		t.Fatalf("Device IDs: want %v , got %v", deviceID, got)
	}

	config1 := gnmi.OC().Interface(p1.Name()).Id()
	defer observer.RecordYgot(t, "REPLACE", config1)
	gnmi.Replace(t, args.dut, config1.Config(), 1)

	defer observer.RecordYgot(t, "GET", config1)
	if got := gnmi.GetConfig(t, args.dut, config1.Config()); got != 1 {
		t.Fatalf("Interface port-id: want 1, got %v", got)
	}

	config2 := gnmi.OC().Interface(p2.Name()).Id()
	defer observer.RecordYgot(t, "REPLACE", config2)
	gnmi.Replace(t, args.dut, config2.Config(), 2)

	defer observer.RecordYgot(t, "GET", config2)
	if got := gnmi.GetConfig(t, args.dut, config2.Config()); got != 2 {
		t.Fatalf("Interface port-id: want 2, got %v", got)
	}
}

func testP4RTTelemetry(t *testing.T, args *testArgs) {
	p1 := args.dut.Port(t, "port1")
	p2 := args.dut.Port(t, "port2")
	subscriptionDuration := 65 * time.Second
	expectedSamples := 2
	configureP4RTDevice(t, args.dut, npu0, deviceID)

	config := gnmi.OC().Interface(p1.Name()).Id()
	defer observer.RecordYgot(t, "REPLACE", config)
	gnmi.Replace(t, args.dut, config.Config(), 1)

	config1 := gnmi.OC().Interface(p2.Name()).Id()
	defer observer.RecordYgot(t, "REPLACE", config1)
	gnmi.Replace(t, args.dut, config1.Config(), 2)

	t.Run("Telemetry for P4RT node-id", func(t *testing.T) {
		t.Parallel()
		statePath := gnmi.OC().Component(npu0).IntegratedCircuit().NodeId()
		defer observer.RecordYgot(t, "SUBSCRIBE", statePath)

		if got := gnmi.Collect(t, args.dut, statePath.State(), subscriptionDuration).Await(t); len(got) < expectedSamples {
			t.Fatalf("P4RT node-id samples: got %d, want %d", len(got), expectedSamples)
		} else {
			t.Logf("Collected P4RT node-id Samples :\n%v", got)
		}
	})

	t.Run(fmt.Sprintf("Telemetry for P4RT Interface port-id for %v", p1.Name()), func(t *testing.T) {
		t.Parallel()
		statePath := gnmi.OC().Interface(p1.Name()).Id()
		defer observer.RecordYgot(t, "SUBSCRIBE", statePath)

		if got := gnmi.Collect(t, args.dut, statePath.State(), subscriptionDuration).Await(t); len(got) < expectedSamples {
			t.Fatalf("P4RT Interface port-id samples: got %d, want %d", len(got), expectedSamples)
		} else {
			t.Logf("Collected P4RT Interface port-id samples :\n%v", got)
		}
	})

	t.Run(fmt.Sprintf("Telemetry for P4RT Interface port-id for %v", p2.Name()), func(t *testing.T) {
		t.Parallel()
		statePath := gnmi.OC().Interface(p2.Name()).Id()
		defer observer.RecordYgot(t, "SUBSCRIBE", statePath)

		if got := gnmi.Collect(t, args.dut, statePath.State(), subscriptionDuration).Await(t); len(got) < expectedSamples {
			t.Fatalf("P4RT Interface port-id samples: got %d, want %d", len(got), expectedSamples)
		} else {
			t.Logf("Collected P4RT Interface port-id samples :\n%v", got)
		}
	})
}

func testP4RTUprev(t *testing.T, args *testArgs) {
	p1 := args.dut.Port(t, "port1")
	for _, portID := range []uint32{4294967039, 4294967038, 1} {

		config := gnmi.OC().Interface(p1.Name()).Id()
		defer observer.RecordYgot(t, "REPLACE", config)
		gnmi.Replace(t, args.dut, config.Config(), portID)

		// Once defect is fixed move this to Get on Config.
		defer observer.RecordYgot(t, "GET", config)
		if got := gnmi.GetConfig(t, args.dut, config.Config()); got != portID {
			t.Fatalf("Interface port-id: want %v, got %v", portID, got)
		}

	}
}
