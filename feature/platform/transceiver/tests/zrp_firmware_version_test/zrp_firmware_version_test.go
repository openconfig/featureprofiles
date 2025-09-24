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

package zrp_firmware_version_test

import (
	"flag"
	"reflect"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/samplestream"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
)

const (
	targetOutputPower = -3
	frequency         = 193100000
	timeout           = 10 * time.Minute
)

var (
	operationalModeFlag = flag.Int("operational_mode", 5, "vendor-specific operational-mode for the channel")
	operationalMode     uint16
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Topology: dut:port1 <--> port2:dut

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
	t.Logf("%v", firmwareVersionVal)
}

func TestZRPFirmwareVersionState(t *testing.T) {
	if operationalModeFlag != nil {
		operationalMode = uint16(*operationalModeFlag)
	} else {
		t.Fatalf("Please specify the vendor-specific operational-mode flag")
	}
	dut1 := ondatra.DUT(t, "dut")
	dp1 := dut1.Port(t, "port1")
	dp2 := dut1.Port(t, "port2")
	t.Logf("dut1: %v", dut1)
	t.Logf("dut1 dp1 name: %v", dp1.Name())
	och1 := components.OpticalChannelComponentFromPort(t, dut1, dp1)
	och2 := components.OpticalChannelComponentFromPort(t, dut1, dp2)
	cfgplugins.ConfigOpticalChannel(t, dut1, och1, frequency, targetOutputPower, operationalMode)
	cfgplugins.ConfigOpticalChannel(t, dut1, och2, frequency, targetOutputPower, operationalMode)
	gnmi.Await(t, dut1, gnmi.OC().Interface(dp1.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_UP)
	transceiverName := gnmi.Get(t, dut1, gnmi.OC().Interface(dp1.Name()).Transceiver().State())
	// Check if TRANSCEIVER is of type 400ZR_PLUS
	if dp1.PMD() != ondatra.PMD400GBASEZRP {
		t.Fatalf("%s Transceiver is not 400ZR_PLUS its of type: %v", transceiverName, dp1.PMD())
	}
	component1 := gnmi.OC().Component(transceiverName)

	p1Stream := samplestream.New(t, dut1, component1.FirmwareVersion().State(), 10*time.Second)

	verifyFirmwareVersionValue(t, dut1, p1Stream)

	p1Stream.Close()
}

func TestZRPFirmwareVersionStateInterfaceFlap(t *testing.T) {
	if operationalModeFlag != nil {
		operationalMode = uint16(*operationalModeFlag)
	} else {
		t.Fatalf("Please specify the vendor-specific operational-mode flag")
	}
	dut1 := ondatra.DUT(t, "dut")
	dp1 := dut1.Port(t, "port1")
	dp2 := dut1.Port(t, "port2")
	t.Logf("dut1: %v", dut1)
	t.Logf("dut1 dp1 name: %v", dp1.Name())
	och1 := components.OpticalChannelComponentFromPort(t, dut1, dp1)
	och2 := components.OpticalChannelComponentFromPort(t, dut1, dp2)
	cfgplugins.ConfigOpticalChannel(t, dut1, och1, frequency, targetOutputPower, operationalMode)
	cfgplugins.ConfigOpticalChannel(t, dut1, och2, frequency, targetOutputPower, operationalMode)
	gnmi.Await(t, dut1, gnmi.OC().Interface(dp1.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_UP)
	transceiverName := gnmi.Get(t, dut1, gnmi.OC().Interface(dp1.Name()).Transceiver().State())
	// Check if TRANSCEIVER is of type 400ZR_PLUS
	if dp1.PMD() != ondatra.PMD400GBASEZRP {
		t.Fatalf("%s Transceiver is not 400ZR_PLUS its of type: %v", transceiverName, dp1.PMD())
	}
	// Disable interface
	// TODO: jchenjian - Add support for module reset (not supported in current implementation)
	cfgplugins.ToggleInterface(t, dut1, dp1.Name(), false)
	component1 := gnmi.OC().Component(transceiverName)

	p1Stream := samplestream.New(t, dut1, component1.FirmwareVersion().State(), 10*time.Second)

	// Wait 60 sec cooling-off period
	gnmi.Await(t, dut1, gnmi.OC().Interface(dp1.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_DOWN)
	t.Logf("Interfaces are down: %v", dp1.Name())
	verifyFirmwareVersionValue(t, dut1, p1Stream)

	// Enable interface
	cfgplugins.ToggleInterface(t, dut1, dp1.Name(), true)
	gnmi.Await(t, dut1, gnmi.OC().Interface(dp1.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_UP)
	t.Logf("Interfaces are up: %v", dp1.Name())
	verifyFirmwareVersionValue(t, dut1, p1Stream)
}
