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
	sensorType        = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_SENSOR
	dp16QAM           = 1
	targetOutputPower = -10
	frequency         = 193100000
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Topology:
//
//	dut:port1 <--> port2:dut

func interfaceConfig(t *testing.T, dut1 *ondatra.DUTDevice, dp *ondatra.Port) {
	d := &oc.Root{}
	i := d.GetOrCreateInterface(dp.Name())
	i.Enabled = ygot.Bool(true)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	gnmi.Replace(t, dut1, gnmi.OC().Interface(dp.Name()).Config(), i)
	OCcomponent := opticalChannelComponentFromPort(t, dut1, dp)
	gnmi.Replace(t, dut1, gnmi.OC().Component(OCcomponent).OpticalChannel().Config(), &oc.Component_OpticalChannel{
		TargetOutputPower: ygot.Float64(targetOutputPower),
		Frequency:         ygot.Uint64(frequency),
	})
}
func verifyTemperatureSensorValue(t *testing.T, dut1 *ondatra.DUTDevice, pStream *samplestream.SampleStream[float64], sensorName string) float64 {
	temperatureSample := pStream.Next(t)
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
	interfaceConfig(t, dut1, dp1)
	interfaceConfig(t, dut1, dp2)
	gnmi.Await(t, dut1, gnmi.OC().Interface(dp1.Name()).OperStatus().State(), intUpdateTime, oc.Interface_OperStatus_UP)
	transceiverName := gnmi.Get(t, dut1, gnmi.OC().Interface(dp1.Name()).Transceiver().State())
	// Check if TRANSCEIVER is of type 400ZR
	if dp1.PMD() != ondatra.PMD400GBASEZR {
		t.Fatalf("%s Transceiver is not 400ZR its of type: %v", transceiverName, dp1.PMD())
	}
	component1 := gnmi.OC().Component(transceiverName)
	subcomponents := gnmi.LookupAll[*oc.Component_Subcomponent](t, dut1, component1.SubcomponentAny().State())
	for _, s := range subcomponents {
		subc, ok := s.Val()
		if ok {
			sensorComponent := gnmi.Get[*oc.Component](t, dut1, gnmi.OC().Component(subc.GetName()).State())
			if sensorComponent.GetType() == sensorType {
				scomponent := gnmi.OC().Component(sensorComponent.GetName())
				if scomponent != nil {
					component1 = scomponent
				}
			}
		}
	}
	p1StreamInstant := samplestream.New(t, dut1, component1.Temperature().Instant().State(), 10*time.Second)
	p1StreamAvg := samplestream.New(t, dut1, component1.Temperature().Avg().State(), 10*time.Second)
	p1StreamMin := samplestream.New(t, dut1, component1.Temperature().Min().State(), 10*time.Second)
	p1StreamMax := samplestream.New(t, dut1, component1.Temperature().Max().State(), 10*time.Second)
	temperatureInstant := verifyTemperatureSensorValue(t, dut1, p1StreamInstant, "Instant")
	t.Logf("Port1 dut1 %s Instant Temperature: %v", dp1.Name(), temperatureInstant)
	temperatureMax := verifyTemperatureSensorValue(t, dut1, p1StreamMax, "Max")
	t.Logf("Port1 dut1 %s Max Temperature: %v", dp1.Name(), temperatureMax)
	temperatureMin := verifyTemperatureSensorValue(t, dut1, p1StreamMin, "Min")
	t.Logf("Port1 dut1 %s Min Temperature: %v", dp1.Name(), temperatureMin)
	temperatureAvg := verifyTemperatureSensorValue(t, dut1, p1StreamAvg, "Avg")
	t.Logf("Port1 dut1 %s Avg Temperature: %v", dp1.Name(), temperatureAvg)
	if temperatureAvg >= temperatureMin && temperatureAvg <= temperatureMax {
		t.Logf("The average is between the maximum and minimum values")
	} else {
		t.Fatalf("The average is not between the maximum and minimum values, Avg:%v Max:%v Min:%v", temperatureAvg, temperatureMax, temperatureMin)
	}
	p1StreamMin.Close()
	p1StreamMax.Close()
	p1StreamAvg.Close()
	p1StreamInstant.Close()
}
func TestZRTemperatureStateInterfaceFlap(t *testing.T) {
	dut1 := ondatra.DUT(t, "dut")
	dp1 := dut1.Port(t, "port1")
	dp2 := dut1.Port(t, "port2")
	t.Logf("dut1: %v", dut1)
	t.Logf("dut1 dp1 name: %v", dp1.Name())
	interfaceConfig(t, dut1, dp1)
	interfaceConfig(t, dut1, dp2)
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
	gnmi.Replace(t, dut1, gnmi.OC().Interface(dp1.Name()).Config(), i)
	component1 := gnmi.OC().Component(transceiverName)
	subcomponents := gnmi.LookupAll[*oc.Component_Subcomponent](t, dut1, component1.SubcomponentAny().State())
	for _, s := range subcomponents {
		subc, ok := s.Val()
		if ok {
			sensorComponent := gnmi.Get[*oc.Component](t, dut1, gnmi.OC().Component(subc.GetName()).State())
			if sensorComponent.GetType() == sensorType {
				scomponent := gnmi.OC().Component(sensorComponent.GetName())
				if scomponent != nil {
					component1 = scomponent
				}
			}
		}
	}
	p1StreamInstant := samplestream.New(t, dut1, component1.Temperature().Instant().State(), 10*time.Second)
	p1StreamAvg := samplestream.New(t, dut1, component1.Temperature().Avg().State(), 10*time.Second)
	p1StreamMin := samplestream.New(t, dut1, component1.Temperature().Min().State(), 10*time.Second)
	p1StreamMax := samplestream.New(t, dut1, component1.Temperature().Max().State(), 10*time.Second)
	// Wait 120 sec cooling off period
	gnmi.Await(t, dut1, gnmi.OC().Interface(dp1.Name()).OperStatus().State(), intUpdateTime, oc.Interface_OperStatus_DOWN)
	temperatureInstant := verifyTemperatureSensorValue(t, dut1, p1StreamInstant, "Instant")
	t.Logf("Port1 dut1 %s Instant Temperature: %v", dp1.Name(), temperatureInstant)
	temperatureMax := verifyTemperatureSensorValue(t, dut1, p1StreamMax, "Max")
	t.Logf("Port1 dut1 %s Max Temperature: %v", dp1.Name(), temperatureMax)
	temperatureMin := verifyTemperatureSensorValue(t, dut1, p1StreamMin, "Min")
	t.Logf("Port1 dut1 %s Min Temperature: %v", dp1.Name(), temperatureMin)
	temperatureAvg := verifyTemperatureSensorValue(t, dut1, p1StreamAvg, "Avg")
	t.Logf("Port1 dut1 %s Avg Temperature: %v", dp1.Name(), temperatureAvg)
	i = d.GetOrCreateInterface(dp1.Name())
	i.Enabled = ygot.Bool(true)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	// Enable interface
	gnmi.Replace(t, dut1, gnmi.OC().Interface(dp1.Name()).Config(), i)
	gnmi.Await(t, dut1, gnmi.OC().Interface(dp1.Name()).OperStatus().State(), intUpdateTime, oc.Interface_OperStatus_UP)
	temperatureInstant = verifyTemperatureSensorValue(t, dut1, p1StreamInstant, "Instant")
	t.Logf("Port1 dut1 %s Instant Temperature: %v", dp1.Name(), temperatureInstant)
	temperatureMax = verifyTemperatureSensorValue(t, dut1, p1StreamMax, "Max")
	t.Logf("Port1 dut1 %s Max Temperature: %v", dp1.Name(), temperatureMax)
	temperatureMin = verifyTemperatureSensorValue(t, dut1, p1StreamMin, "Min")
	t.Logf("Port1 dut1 %s Min Temperature: %v", dp1.Name(), temperatureMin)
	temperatureAvg = verifyTemperatureSensorValue(t, dut1, p1StreamAvg, "Avg")
	t.Logf("Port1 dut1 %s Avg Temperature: %v", dp1.Name(), temperatureAvg)
	if temperatureAvg >= temperatureMin && temperatureAvg <= temperatureMax {
		t.Logf("The average is between the maximum and minimum values")
	} else {
		t.Fatalf("The average is not between the maximum and minimum values")
	}
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
