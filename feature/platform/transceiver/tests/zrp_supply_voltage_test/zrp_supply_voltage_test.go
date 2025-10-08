// Copyright 2024 Google LLC
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

package zrp_supply_voltage_test

import (
	"flag"
	"fmt"
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
	samplingInterval  = 10 * time.Second
	intUpdateTime     = 5 * time.Minute
	targetOutputPower = -3
	frequency         = 193100000
)

var (
	operationalModeFlag = flag.Int("operational_mode", 5, "vendor-specific operational-mode for the channel")
	operationalMode     uint16
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func verifyVoltageValue(t *testing.T, pStream *samplestream.SampleStream[float64], path string) float64 {
	voltageSample := pStream.Next()
	if voltageSample == nil {
		t.Fatalf("Voltage telemetry %s was not streamed in the most recent subscription interval", path)
	}
	voltageVal, ok := voltageSample.Val()
	if !ok {
		t.Fatalf("Voltage %q telemetry is not present", voltageSample)
	}
	// Check voltage return value of correct type
	if reflect.TypeOf(voltageVal).Kind() != reflect.Float64 {
		t.Fatalf("Return value is not type float64")
	}
	t.Logf("Voltage sample value %s: %v", path, voltageVal)
	return voltageVal
}

func TestZrSupplyVoltage(t *testing.T) {
	if operationalModeFlag != nil {
		operationalMode = uint16(*operationalModeFlag)
	} else {
		t.Fatalf("Please specify the vendor-specific operational-mode flag")
	}
	dut := ondatra.DUT(t, "dut")
	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")
	och1 := components.OpticalChannelComponentFromPort(t, dut, dp1)
	och2 := components.OpticalChannelComponentFromPort(t, dut, dp2)
	cfgplugins.ConfigOpticalChannel(t, dut, och1, frequency, targetOutputPower, operationalMode)
	cfgplugins.ConfigOpticalChannel(t, dut, och2, frequency, targetOutputPower, operationalMode)

	for _, port := range []string{"port1", "port2"} {
		t.Run(fmt.Sprintf("Port:%s", port), func(t *testing.T) {
			dp := dut.Port(t, port)
			t.Logf("Port %s", dp.Name())

			gnmi.Await(t, dut, gnmi.OC().Interface(dp.Name()).OperStatus().State(), intUpdateTime, oc.Interface_OperStatus_UP)

			// Derive transceiver names from ports.
			tr := gnmi.Get(t, dut, gnmi.OC().Interface(dp.Name()).Transceiver().State())
			component := gnmi.OC().Component(tr)

			streamInst := samplestream.New(t, dut, component.Transceiver().SupplyVoltage().Instant().State(), samplingInterval)
			defer streamInst.Close()

			volInst := verifyVoltageValue(t, streamInst, "Instant")
			t.Logf("Port %s instant voltage: %v", dp.Name(), volInst)

			// Disable interface
			cfgplugins.ToggleInterface(t, dut, dp.Name(), false)
			// Wait for the cooling-off period
			gnmi.Await(t, dut, gnmi.OC().Interface(dp.Name()).OperStatus().State(), intUpdateTime, oc.Interface_OperStatus_DOWN)

			volInstNew := verifyVoltageValue(t, streamInst, "Instant")
			t.Logf("Port %s instant voltage after port down: %v", dp.Name(), volInstNew)

			// Enable interface again.
			cfgplugins.ToggleInterface(t, dut, dp.Name(), true)
			// Wait for the cooling-off period
			gnmi.Await(t, dut, gnmi.OC().Interface(dp.Name()).OperStatus().State(), intUpdateTime, oc.Interface_OperStatus_UP)
		})
	}
}
