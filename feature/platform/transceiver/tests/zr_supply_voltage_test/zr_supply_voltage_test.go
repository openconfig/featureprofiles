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

package zr_supply_voltage_test

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/samplestream"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
)

const (
	samplingInterval     = 10 * time.Second
	targetOutputPowerdBm = -10
	targetFrequencyHz    = 193100000
	intUpdateTime        = 2 * time.Minute
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func opticalChannelComponentFromPort(t *testing.T, dut *ondatra.DUTDevice, p *ondatra.Port) string {
	t.Helper()
	if deviations.MissingPortToOpticalChannelMapping(dut) {
		transceiverName := gnmi.Get(t, dut, gnmi.OC().Interface(p.Name()).Transceiver().State())
		return fmt.Sprintf("%s-Optical0", transceiverName)
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

func interfaceConfig(t *testing.T, dut *ondatra.DUTDevice, dp *ondatra.Port) {
	d := &oc.Root{}
	i := d.GetOrCreateInterface(dp.Name())
	i.Enabled = ygot.Bool(true)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	gnmi.Replace(t, dut, gnmi.OC().Interface(dp.Name()).Config(), i)
	ocComponent := opticalChannelComponentFromPort(t, dut, dp)
	t.Logf("Got opticalChannelComponent from port: %s", ocComponent)
	gnmi.Replace(t, dut, gnmi.OC().Component(ocComponent).OpticalChannel().Config(), &oc.Component_OpticalChannel{
		TargetOutputPower: ygot.Float64(targetOutputPowerdBm),
		Frequency:         ygot.Uint64(targetFrequencyHz),
	})
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
	dut := ondatra.DUT(t, "dut")
	interfaceConfig(t, dut, dut.Port(t, "port1"))
	interfaceConfig(t, dut, dut.Port(t, "port2"))

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
			streamAvg := samplestream.New(t, dut, component.Transceiver().SupplyVoltage().Avg().State(), samplingInterval)
			defer streamAvg.Close()
			streamMin := samplestream.New(t, dut, component.Transceiver().SupplyVoltage().Min().State(), samplingInterval)
			defer streamMin.Close()
			streamMax := samplestream.New(t, dut, component.Transceiver().SupplyVoltage().Max().State(), samplingInterval)
			defer streamMax.Close()

			volInst := verifyVoltageValue(t, streamInst, "Instant")
			t.Logf("Port %s instant voltage: %v", dp.Name(), volInst)
			volAvg := verifyVoltageValue(t, streamAvg, "Avg")
			t.Logf("Port %s average voltage: %v", dp.Name(), volAvg)
			volMin := verifyVoltageValue(t, streamMin, "Min")
			t.Logf("Port %s minimum voltage: %v", dp.Name(), volMin)
			volMax := verifyVoltageValue(t, streamMax, "Max")
			t.Logf("Port %s maximum voltage: %v", dp.Name(), volMax)

			if volAvg >= volMin && volAvg <= volMax {
				t.Logf("The average is between the maximum and minimum values")
			} else {
				t.Fatalf("The average is not between the maximum and minimum values, Avg:%v Max:%v Min:%v", volAvg, volMax, volMin)
			}

			// Wait for the cooling off period
			gnmi.Await(t, dut, gnmi.OC().Interface(dp.Name()).OperStatus().State(), intUpdateTime, oc.Interface_OperStatus_DOWN)

			volInstNew := verifyVoltageValue(t, streamInst, "Instant")
			t.Logf("Port %s instant voltage after port down: %v", dp.Name(), volInstNew)
			volAvgNew := verifyVoltageValue(t, streamAvg, "Avg")
			t.Logf("Port %s average voltage after port down: %v", dp.Name(), volAvgNew)
			volMinNew := verifyVoltageValue(t, streamMin, "Min")
			t.Logf("Port %s minimum voltage after port down: %v", dp.Name(), volMinNew)
			volMaxNew := verifyVoltageValue(t, streamMax, "Max")
			t.Logf("Port %s maximum voltage after port down: %v", dp.Name(), volMaxNew)

			if volAvgNew >= volMinNew && volAvgNew <= volMaxNew {
				t.Logf("The average voltage after port down is between the maximum and minimum values")
			} else {
				t.Fatalf("The average voltage after port down is not between the maximum and minimum values, Avg:%v Max:%v Min:%v", volAvgNew, volMaxNew, volMinNew)
			}
		})
	}

}
