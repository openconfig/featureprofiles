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

package zr_firmware_version_test

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/samplestream"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

const (
	dp16QAM           = 1
	targetOutputPower = -10
	frequency         = 193100000
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Topology: dut:port1 <--> port2:dut

func configInterface(t *testing.T, dut1 *ondatra.DUTDevice, dp *ondatra.Port, enable bool) {
	d := &oc.Root{}
	i := d.GetOrCreateInterface(dp.Name())
	i.Enabled = ygot.Bool(enable)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	gnmi.Replace(t, dut1, gnmi.OC().Interface(dp.Name()).Config(), i)
	component := opticalChannelComponentFromPort(t, dut1, dp)
	gnmi.Replace(t, dut1, gnmi.OC().Component(component).OpticalChannel().Config(), &oc.Component_OpticalChannel{
		TargetOutputPower: ygot.Float64(targetOutputPower),
		Frequency:         ygot.Uint64(frequency),
	})
}
func verifyFirmwareVersionValue(t *testing.T, dut1 *ondatra.DUTDevice, pStream *samplestream.SampleStream[string]) {
	firmwareVersionSample := pStream.Next()
	if firmwareVersionSample == nil {
		t.Fatalf("Firmware telemetry %v was not streamed in the most recent subscription interval", firmwareVersionSample)
	}
	firmwareVersionVal, ok := firmwareVersionSample.Val()
	if !ok {
		t.Fatalf("Firmware version %q telemetry is not present", firmwareVersionSample)
	}
	// Check firmware version return value of correct type
	if reflect.TypeOf(firmwareVersionVal).Kind() != reflect.String {
		t.Fatalf("Return value is not type string")
	}
}

func TestZRFirmwareVersionState(t *testing.T) {
	dut1 := ondatra.DUT(t, "dut")
	dp1 := dut1.Port(t, "port1")
	dp2 := dut1.Port(t, "port2")
	t.Logf("dut1: %v", dut1)
	t.Logf("dut1 dp1 name: %v", dp1.Name())
	configInterface(t, dut1, dp1, true)
	configInterface(t, dut1, dp2, true)
	gnmi.Await(t, dut1, gnmi.OC().Interface(dp1.Name()).OperStatus().State(), time.Minute, oc.Interface_OperStatus_UP)
	transceiverName := gnmi.Get(t, dut1, gnmi.OC().Interface(dp1.Name()).Transceiver().State())
	// Check if TRANSCEIVER is of type 400ZR
	if dp1.PMD() != ondatra.PMD400GBASEZR {
		t.Fatalf("%s Transceiver is not 400ZR its of type: %v", transceiverName, dp1.PMD())
	}
	component1 := gnmi.OC().Component(transceiverName)

	p1Stream := samplestream.New(t, dut1, component1.FirmwareVersion().State(), 10*time.Second)

	verifyFirmwareVersionValue(t, dut1, p1Stream)

	p1Stream.Close()
}
func TestZRFirmwareVersionStateInterfaceFlap(t *testing.T) {
	dut1 := ondatra.DUT(t, "dut")
	dp1 := dut1.Port(t, "port1")
	dp2 := dut1.Port(t, "port2")
	t.Logf("dut1: %v", dut1)
	t.Logf("dut1 dp1 name: %v", dp1.Name())
	configInterface(t, dut1, dp1, true)
	configInterface(t, dut1, dp2, true)
	gnmi.Await(t, dut1, gnmi.OC().Interface(dp1.Name()).OperStatus().State(), time.Minute, oc.Interface_OperStatus_UP)
	transceiverName := gnmi.Get(t, dut1, gnmi.OC().Interface(dp1.Name()).Transceiver().State())
	// Check if TRANSCEIVER is of type 400ZR
	if dp1.PMD() != ondatra.PMD400GBASEZR {
		t.Fatalf("%s Transceiver is not 400ZR its of type: %v", transceiverName, dp1.PMD())
	}
	// Disable interface
	configInterface(t, dut1, dp1, false)
	component1 := gnmi.OC().Component(transceiverName)

	p1Stream := samplestream.New(t, dut1, component1.FirmwareVersion().State(), 10*time.Second)

	// Wait 60 sec cooling off period
	gnmi.Await(t, dut1, gnmi.OC().Interface(dp1.Name()).OperStatus().State(), time.Minute, oc.Interface_OperStatus_DOWN)
	verifyFirmwareVersionValue(t, dut1, p1Stream)

	// Enable interface
	configInterface(t, dut1, dp1, true)
	gnmi.Await(t, dut1, gnmi.OC().Interface(dp1.Name()).OperStatus().State(), time.Minute, oc.Interface_OperStatus_UP)
	verifyFirmwareVersionValue(t, dut1, p1Stream)
}

func opticalChannelComponentFromPort(t *testing.T, dut *ondatra.DUTDevice, p *ondatra.Port) string {
	t.Helper()
	if deviations.MissingPortToOpticalChannelMapping(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			transceiverName := gnmi.Get(t, dut, gnmi.OC().Interface(p.Name()).Transceiver().State())
			return fmt.Sprintf("%s-Optical0", transceiverName)
		default:
			t.Fatal("Manual Optical channel name required when deviation missing_port_to_optical_channel_component_mapping applied.")
		}
	}
	compName := gnmi.Get(t, dut, gnmi.OC().Interface(p.Name()).HardwarePort().State())
	for {
		comp, ok := gnmi.Lookup(t, dut, gnmi.OC().Component(compName).State()).Val()
		if !ok {
			t.Fatalf("Recursive optical channel lookup failed for port: %s, component %s not found.", p.Name(), compName)
		}
		if comp.GetType() == oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_OPTICAL_CHANNEL {
			return compName
		}
		if comp.GetParent() == "" {
			t.Fatalf("Recursive optical channel lookup failed for port: %s, parent of component %s not found.", p.Name(), compName)
		}
		compName = comp.GetParent()
	}
}
