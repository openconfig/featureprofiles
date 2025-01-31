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

package zr_temperature_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/samplestream"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

const (
	sensorType = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_SENSOR
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Topology:
//
//	dut:port1 <--> port2:dut

func verifyTemperatureSensorValue(t *testing.T, pStream *samplestream.SampleStream[float64], sensorName string) float64 {
	temperatureSample := pStream.Next()
	if temperatureSample == nil {
		t.Fatalf("Temperature telemetry %s was not streamed in the most recent subscription interval", sensorName)
	}
	temperatureVal, ok := temperatureSample.Val()
	if !ok {
		t.Fatalf("Temperature %q telemetry is not present", temperatureSample)
	}
	// Check temperature return value of correct type
	if reflect.TypeOf(temperatureVal).Kind() != reflect.Float64 {
		t.Fatalf("Return value is not type float64")
	} else if temperatureVal <= 0 && temperatureVal >= 300 {
		t.Fatalf("The variable temperature instent is not between 0 and 300")
	}
	t.Logf("Temperature sample value %s: %v", sensorName, temperatureVal)
	return temperatureVal
}

func TestZRTemperatureState(t *testing.T) {
	dut1 := ondatra.DUT(t, "dut")
	dp1 := dut1.Port(t, "port1")
	dp2 := dut1.Port(t, "port2")
	t.Logf("dut1: %v", dut1)
	t.Logf("dut1 dp1 name: %v", dp1.Name())
	intUpdateTime := 2 * time.Minute
	cfgplugins.InterfaceConfig(t, dut1, dp1)
	cfgplugins.InterfaceConfig(t, dut1, dp2)
	gnmi.Await(t, dut1, gnmi.OC().Interface(dp1.Name()).OperStatus().State(), intUpdateTime, oc.Interface_OperStatus_UP)
	transceiverName := gnmi.Get(t, dut1, gnmi.OC().Interface(dp1.Name()).Transceiver().State())
	// Check if TRANSCEIVER is of type 400ZR
	if dp1.PMD() != ondatra.PMD400GBASEZR {
		t.Fatalf("%s Transceiver is not 400ZR its of type: %v", transceiverName, dp1.PMD())
	}
	compWithTemperature := gnmi.OC().Component(transceiverName)
	if !deviations.UseParentComponentForTemperatureTelemetry(dut1) {
		subcomponents := gnmi.LookupAll[*oc.Component_Subcomponent](t, dut1, compWithTemperature.SubcomponentAny().State())
		for _, s := range subcomponents {
			subc, ok := s.Val()
			if ok {
				sensorComponent := gnmi.Get[*oc.Component](t, dut1, gnmi.OC().Component(subc.GetName()).State())
				if sensorComponent.GetType() == sensorType {
					scomponent := gnmi.OC().Component(sensorComponent.GetName())
					if scomponent != nil {
						compWithTemperature = scomponent
					}
				}
			}
		}
	}
	p1StreamInstant := samplestream.New(t, dut1, compWithTemperature.Temperature().Instant().State(), 10*time.Second)
	temperatureInstant := verifyTemperatureSensorValue(t, p1StreamInstant, "Instant")
	t.Logf("Port1 dut1 %s Instant Temperature: %v", dp1.Name(), temperatureInstant)
	if deviations.MissingZROpticalChannelTunableParametersTelemetry(dut1) {
		t.Log("Skipping Min/Max/Avg Tunable Parameters Telemetry validation. Deviation MissingZROpticalChannelTunableParametersTelemetry enabled.")
	} else {
		p1StreamAvg := samplestream.New(t, dut1, compWithTemperature.Temperature().Avg().State(), 10*time.Second)
		p1StreamMin := samplestream.New(t, dut1, compWithTemperature.Temperature().Min().State(), 10*time.Second)
		p1StreamMax := samplestream.New(t, dut1, compWithTemperature.Temperature().Max().State(), 10*time.Second)

		temperatureMax := verifyTemperatureSensorValue(t, p1StreamMax, "Max")
		t.Logf("Port1 dut1 %s Max Temperature: %v", dp1.Name(), temperatureMax)
		temperatureMin := verifyTemperatureSensorValue(t, p1StreamMin, "Min")
		t.Logf("Port1 dut1 %s Min Temperature: %v", dp1.Name(), temperatureMin)
		temperatureAvg := verifyTemperatureSensorValue(t, p1StreamAvg, "Avg")
		t.Logf("Port1 dut1 %s Avg Temperature: %v", dp1.Name(), temperatureAvg)
		if temperatureAvg >= temperatureMin && temperatureAvg <= temperatureMax {
			t.Logf("The average is between the maximum and minimum values")
		} else {
			t.Fatalf("The average is not between the maximum and minimum values, Avg:%v Max:%v Min:%v", temperatureAvg, temperatureMax, temperatureMin)
		}
		p1StreamMin.Close()
		p1StreamMax.Close()
		p1StreamAvg.Close()
	}
	p1StreamInstant.Close()
}

func TestZRTemperatureStateInterfaceFlap(t *testing.T) {
	dut1 := ondatra.DUT(t, "dut")
	dp1 := dut1.Port(t, "port1")
	dp2 := dut1.Port(t, "port2")
	t.Logf("dut1: %v", dut1)
	t.Logf("dut1 dp1 name: %v", dp1.Name())
	cfgplugins.InterfaceConfig(t, dut1, dp1)
	cfgplugins.InterfaceConfig(t, dut1, dp2)
	intUpdateTime := 2 * time.Minute
	gnmi.Await(t, dut1, gnmi.OC().Interface(dp1.Name()).OperStatus().State(), intUpdateTime, oc.Interface_OperStatus_UP)
	transceiverName := gnmi.Get(t, dut1, gnmi.OC().Interface(dp1.Name()).Transceiver().State())
	// Check if TRANSCEIVER is of type 400ZR
	if dp1.PMD() != ondatra.PMD400GBASEZR {
		t.Fatalf("%s Transceiver is not 400ZR its of type: %v", transceiverName, dp1.PMD())
	}
	// Disable interface
	d := &oc.Root{}
	i := d.GetOrCreateInterface(dp1.Name())
	i.Enabled = ygot.Bool(false)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	gnmi.Replace(t, dut1, gnmi.OC().Interface(dp1.Name()).Config(), i)
	compWithTemperature := gnmi.OC().Component(transceiverName)
	if !deviations.UseParentComponentForTemperatureTelemetry(dut1) {
		subcomponents := gnmi.LookupAll[*oc.Component_Subcomponent](t, dut1, compWithTemperature.SubcomponentAny().State())
		for _, s := range subcomponents {
			subc, ok := s.Val()
			if ok {
				sensorComponent := gnmi.Get[*oc.Component](t, dut1, gnmi.OC().Component(subc.GetName()).State())
				if sensorComponent.GetType() == sensorType {
					scomponent := gnmi.OC().Component(sensorComponent.GetName())
					if scomponent != nil {
						compWithTemperature = scomponent
					}
				}
			}
		}
	}
	p1StreamInstant := samplestream.New(t, dut1, compWithTemperature.Temperature().Instant().State(), 10*time.Second)
	p1StreamAvg := samplestream.New(t, dut1, compWithTemperature.Temperature().Avg().State(), 10*time.Second)
	p1StreamMin := samplestream.New(t, dut1, compWithTemperature.Temperature().Min().State(), 10*time.Second)
	p1StreamMax := samplestream.New(t, dut1, compWithTemperature.Temperature().Max().State(), 10*time.Second)
	// Wait 120 sec cooling-off period
	gnmi.Await(t, dut1, gnmi.OC().Interface(dp1.Name()).OperStatus().State(), intUpdateTime, oc.Interface_OperStatus_DOWN)
	temperatureInstant := verifyTemperatureSensorValue(t, p1StreamInstant, "Instant")
	t.Logf("Port1 dut1 %s Instant Temperature: %v", dp1.Name(), temperatureInstant)
	if deviations.MissingZROpticalChannelTunableParametersTelemetry(dut1) {
		t.Log("Skipping Min/Max/Avg Tunable Parameters Telemetry validation. Deviation MissingZROpticalChannelTunableParametersTelemetry enabled.")
	} else {
		temperatureMax := verifyTemperatureSensorValue(t, p1StreamMax, "Max")
		t.Logf("Port1 dut1 %s Max Temperature: %v", dp1.Name(), temperatureMax)
		temperatureMin := verifyTemperatureSensorValue(t, p1StreamMin, "Min")
		t.Logf("Port1 dut1 %s Min Temperature: %v", dp1.Name(), temperatureMin)
		temperatureAvg := verifyTemperatureSensorValue(t, p1StreamAvg, "Avg")
		t.Logf("Port1 dut1 %s Avg Temperature: %v", dp1.Name(), temperatureAvg)
	}
	i = d.GetOrCreateInterface(dp1.Name())
	i.Enabled = ygot.Bool(true)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	// Enable interface
	gnmi.Replace(t, dut1, gnmi.OC().Interface(dp1.Name()).Config(), i)
	gnmi.Await(t, dut1, gnmi.OC().Interface(dp1.Name()).OperStatus().State(), intUpdateTime, oc.Interface_OperStatus_UP)
	temperatureInstant = verifyTemperatureSensorValue(t, p1StreamInstant, "Instant")
	t.Logf("Port1 dut1 %s Instant Temperature: %v", dp1.Name(), temperatureInstant)
	if deviations.MissingZROpticalChannelTunableParametersTelemetry(dut1) {
		t.Log("Skipping Min/Max/Avg Tunable Parameters Telemetry validation. Deviation MissingZROpticalChannelTunableParametersTelemetry enabled.")
	} else {
		temperatureMax := verifyTemperatureSensorValue(t, p1StreamMax, "Max")
		t.Logf("Port1 dut1 %s Max Temperature: %v", dp1.Name(), temperatureMax)
		temperatureMin := verifyTemperatureSensorValue(t, p1StreamMin, "Min")
		t.Logf("Port1 dut1 %s Min Temperature: %v", dp1.Name(), temperatureMin)
		temperatureAvg := verifyTemperatureSensorValue(t, p1StreamAvg, "Avg")
		t.Logf("Port1 dut1 %s Avg Temperature: %v", dp1.Name(), temperatureAvg)
		if temperatureAvg >= temperatureMin && temperatureAvg <= temperatureMax {
			t.Logf("The average is between the maximum and minimum values")
		} else {
			t.Fatalf("The average is not between the maximum and minimum values")
		}
	}
}
