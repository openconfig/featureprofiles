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

	"github.com/openconfig/featureprofiles/internal/fptest"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
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
//	dut1:port1 <--> port1:dut2
func gnmiOpts(t *testing.T, dut *ondatra.DUTDevice, mode gpb.SubscriptionMode, interval time.Duration) *gnmi.Opts {
	return dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(mode), ygnmi.WithSampleInterval(interval))
}

func TestZRTemperatureState(t *testing.T) {
	dut1 := ondatra.DUT(t, "dut1")
	dp1 := dut1.Port(t, "port1")
	t.Logf("dut1: %v", dut1)
	t.Logf("dut1 dp1 name: %v", dp1.Name())
	transceiverName := gnmi.Get(t, dut1, gnmi.OC().Interface(dp1.Name()).Transceiver().State())
	// Check if TRANSCEIVER is of type 400ZR
	if dp1.PMD() != ondatra.PMD400GBASEZR {
		t.Fatalf("%s Transceiver is not 400ZR its of type: %v", transceiverName, dp1.PMD())
	}
	component1 := gnmi.OC().Component(transceiverName)
	d := &oc.Root{}
	i := d.GetOrCreateInterface(dp1.Name())
	i.Enabled = ygot.Bool(true)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	intUpdateTime := 2 * time.Minute
	gnmi.Replace(t, dut1, gnmi.OC().Interface(dp1.Name()).Config(), i)
	gnmi.Await(t, dut1, gnmi.OC().Interface(dp1.Name()).OperStatus().State(), intUpdateTime, oc.Interface_OperStatus_UP)
	temperatureInstantSample := gnmi.Collect(t, gnmiOpts(t, dut1, gpb.SubscriptionMode_SAMPLE, time.Second*10), component1.Temperature().Instant().State(), time.Second*10)
	temperatureInstantSamples := temperatureInstantSample.Await(t)
	if len(temperatureInstantSamples) == 0 {
		t.Fatalf("Get Temeprature Instant sample list for interface %q: got 0, want > 0", dp1.Name())
	}
	// Taking first sample
	temperatureInstant, _ := temperatureInstantSamples[0].Val()
	// Check temperature return value of correct type
	if reflect.TypeOf(temperatureInstant).Kind() != reflect.Float64 {
		t.Fatalf("Return value is not type float64")
	} else if temperatureInstant <= 0 && temperatureInstant >= 300 {
		t.Fatalf("The variable temperature instent is not between 0 and 300")
	}
	t.Logf("Port1 dut1 %s Instant Temperature: %v", dp1.Name(), temperatureInstant)
	temperatureMinSample := gnmi.Collect(t, gnmiOpts(t, dut1, gpb.SubscriptionMode_SAMPLE, time.Second*10), component1.Temperature().Min().State(), time.Second*10)
	temperatureMinSamples := temperatureMinSample.Await(t)
	if len(temperatureMinSamples) == 0 {
		t.Fatalf("Get Temeprature Min sample list for interface %q: got 0, want > 0", dp1.Name())
	}
	// Taking first sample
	temperatureMin, _ := temperatureMinSamples[0].Val()
	t.Logf("Port1 dut1 %s Min Temperature: %v", dp1.Name(), temperatureMin)
	if reflect.TypeOf(temperatureMin).Kind() != reflect.Float64 {
		t.Fatalf("Return value is not type float64")
	} else if temperatureMin <= 0 && temperatureMin >= 300 {
		t.Fatalf("The variable temperature min is not between 0 and 300")
	}
	temperatureMaxSample := gnmi.Collect(t, gnmiOpts(t, dut1, gpb.SubscriptionMode_SAMPLE, time.Second*10), component1.Temperature().Max().State(), time.Second*10)
	temperatureMaxSamples := temperatureMaxSample.Await(t)
	if len(temperatureMaxSamples) == 0 {
		t.Fatalf("Get Temeprature Max sample list for interface %q: got 0, want > 0", dp1.Name())
	}
	// Taking first sample
	temperatureMax, _ := temperatureMaxSamples[0].Val()
	t.Logf("Port1 dut1 %s Max Temperature: %v", dp1.Name(), temperatureMax)
	if reflect.TypeOf(temperatureMax).Kind() != reflect.Float64 {
		t.Fatalf("Return value is not type float64")
	} else if temperatureMax <= 0 && temperatureMax >= 300 {
		t.Fatalf("The variable temperature min is not between 0 and 300")
	}
	temperatureAvgSample := gnmi.Collect(t, gnmiOpts(t, dut1, gpb.SubscriptionMode_SAMPLE, time.Second*10), component1.Temperature().Avg().State(), time.Second*10)
	temperatureAvgSamples := temperatureAvgSample.Await(t)
	if len(temperatureAvgSamples) == 0 {
		t.Fatalf("Get Temeprature Avg sample list for interface %q: got 0, want > 0", dp1.Name())
	}
	// Taking first sample
	temperatureAvg, _ := temperatureAvgSamples[0].Val()
	if reflect.TypeOf(temperatureAvg).Kind() != reflect.Float64 {
		t.Fatalf("Return value is not type float64")
	} else if temperatureAvg <= 0 && temperatureAvg >= 300 {
		t.Fatalf("The variable temperature avg is not between 0 and 300")
	}
	t.Logf("Port1 dut1 %s Avg Temperature: %v", dp1.Name(), temperatureAvg)
	// temperature average should be between min and max
	if temperatureAvg >= temperatureMin && temperatureAvg <= temperatureMax {
		t.Logf("The average is between the maximum and minimum values")
	} else {
		t.Fatalf("The average is not between the maximum and minimum values")
	}
	subcomponents := gnmi.LookupAll[*oc.Component_Subcomponent](t, dut1, component1.SubcomponentAny().State())
	for _, s := range subcomponents {
		subc, ok := s.Val()
		if ok {
			sensorComponent := gnmi.Get[*oc.Component](t, dut1, gnmi.OC().Component(subc.GetName()).State())
			if sensorComponent.GetType() == sensorType {
				scomponent := gnmi.OC().Component(sensorComponent.GetName())
				v := gnmi.Lookup(t, dut1, scomponent.Temperature().Instant().State())
				if _, ok := v.Val(); !ok {
					t.Errorf("Sensor %s: Temperature instant is not defined", sensorComponent.GetName())
				}
				v = gnmi.Lookup(t, dut1, scomponent.Temperature().Min().State())
				if _, ok := v.Val(); !ok {
					t.Errorf("Sensor %s: Temperature Min is not defined", sensorComponent.GetName())
				}
				v = gnmi.Lookup(t, dut1, scomponent.Temperature().Max().State())
				if _, ok := v.Val(); !ok {
					t.Errorf("Sensor %s: Temperature Max is not defined", sensorComponent.GetName())
				}
				v = gnmi.Lookup(t, dut1, scomponent.Temperature().Avg().State())
				if _, ok := v.Val(); !ok {
					t.Errorf("Sensor %s: Temperature Avg is not defined", sensorComponent.GetName())
				}
			} else {
				t.Fatalf("Subcomponent %s is not a sensor", subc.GetName())
			}
		} else {
			t.Fatalf("Subcomponent %s is not defined", subc.GetName())
		}
	}
}
func TestZRTemperatureStateInterfaceFlap(t *testing.T) {
	dut1 := ondatra.DUT(t, "dut1")
	dp1 := dut1.Port(t, "port1")
	t.Logf("dut1: %v", dut1)
	t.Logf("dut1 dp1 name: %v", dp1.Name())
	transceiverName := gnmi.Get(t, dut1, gnmi.OC().Interface(dp1.Name()).Transceiver().State())
	// Check if TRANSCEIVER is of type 400ZR
	if dp1.PMD() != ondatra.PMD400GBASEZR {
		t.Fatalf("%s Transceiver is not 400ZR its of type: %v", transceiverName, dp1.PMD())
	}
	component1 := gnmi.OC().Component(transceiverName)
	d := &oc.Root{}
	i := d.GetOrCreateInterface(dp1.Name())
	i.Enabled = ygot.Bool(false)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	// Disable interface
	gnmi.Replace(t, dut1, gnmi.OC().Interface(dp1.Name()).Config(), i)
	// During interface disable temperature sensor should not return invalid type
	temperatureInstantSample := gnmi.Collect(t, gnmiOpts(t, dut1, gpb.SubscriptionMode_SAMPLE, time.Second*10), component1.Temperature().Instant().State(), time.Second*10)
	temperatureInstantSamples := temperatureInstantSample.Await(t)
	if len(temperatureInstantSamples) == 0 {
		t.Fatalf("Get Temeprature Instant sample list for interface %q: got 0, want > 0", dp1.Name())
	}
	// Taking first sample
	temperatureInstant, _ := temperatureInstantSamples[0].Val()
	// Check temperature return value of correct type
	t.Logf("Port1 dut1 %s Instant Temperature: %v", dp1.Name(), temperatureInstant)
	temperatureMinSample := gnmi.Collect(t, gnmiOpts(t, dut1, gpb.SubscriptionMode_SAMPLE, time.Second*10), component1.Temperature().Min().State(), time.Second*10)
	temperatureMinSamples := temperatureMinSample.Await(t)
	if len(temperatureMinSamples) == 0 {
		t.Fatalf("Get Temeprature Min sample list for interface %q: got 0, want > 0", dp1.Name())
	}
	// Taking first sample
	temperatureMin, _ := temperatureMinSamples[0].Val()
	t.Logf("Port1 dut1 %s Min Temperature: %v", dp1.Name(), temperatureMin)
	temperatureMaxSample := gnmi.Collect(t, gnmiOpts(t, dut1, gpb.SubscriptionMode_SAMPLE, time.Second*10), component1.Temperature().Max().State(), time.Second*10)
	temperatureMaxSamples := temperatureMaxSample.Await(t)
	if len(temperatureMaxSamples) == 0 {
		t.Fatalf("Get Temeprature Max sample list for interface %q: got 0, want > 0", dp1.Name())
	}
	// Taking first sample
	temperatureMax, _ := temperatureMaxSamples[0].Val()
	t.Logf("Port1 dut1 %s Max Temperature: %v", dp1.Name(), temperatureMax)
	temperatureAvgSample := gnmi.Collect(t, gnmiOpts(t, dut1, gpb.SubscriptionMode_SAMPLE, time.Second*10), component1.Temperature().Avg().State(), time.Second*10)
	temperatureAvgSamples := temperatureAvgSample.Await(t)
	if len(temperatureAvgSamples) == 0 {
		t.Fatalf("Get Temeprature Avg sample list for interface %q: got 0, want > 0", dp1.Name())
	}
	// Taking first sample
	temperatureAvg, _ := temperatureAvgSamples[0].Val()
	t.Logf("Port1 dut1 %s Avg Temperature: %v", dp1.Name(), temperatureAvg)
	if reflect.TypeOf(temperatureInstant).Kind() != reflect.Float64 {
		t.Fatalf("Return value is not type float64")
	}
	if reflect.TypeOf(temperatureMin).Kind() != reflect.Float64 {
		t.Fatalf("Return value is not type float64")
	}
	if reflect.TypeOf(temperatureMax).Kind() != reflect.Float64 {
		t.Fatalf("Return value is not type float64")
	}
	if reflect.TypeOf(temperatureAvg).Kind() != reflect.Float64 {
		t.Fatalf("Return value is not type float64")
	}
	// Wait 120 sec cooling off period
	intUpdateTime := 2 * time.Minute
	temperatureInstantSample = gnmi.Collect(t, gnmiOpts(t, dut1, gpb.SubscriptionMode_SAMPLE, time.Second*10), component1.Temperature().Instant().State(), time.Second*10)
	temperatureInstantSamples = temperatureInstantSample.Await(t)
	// Taking first sample
	temperatureInstant, _ = temperatureInstantSamples[0].Val()
	// Check temperature return value of correct type
	t.Logf("Port1 dut1 %s Instant Temperature: %v", dp1.Name(), temperatureInstant)
	temperatureMinSample = gnmi.Collect(t, gnmiOpts(t, dut1, gpb.SubscriptionMode_SAMPLE, time.Second*10), component1.Temperature().Min().State(), time.Second*10)
	temperatureMinSamples = temperatureMinSample.Await(t)
	// Taking first sample
	temperatureMin, _ = temperatureMinSamples[0].Val()
	t.Logf("Port1 dut1 %s Min Temperature: %v", dp1.Name(), temperatureMin)
	temperatureMaxSample = gnmi.Collect(t, gnmiOpts(t, dut1, gpb.SubscriptionMode_SAMPLE, time.Second*10), component1.Temperature().Max().State(), time.Second*10)
	temperatureMaxSamples = temperatureMaxSample.Await(t)
	// Taking first sample
	temperatureMax, _ = temperatureMaxSamples[0].Val()
	t.Logf("Port1 dut1 %s Max Temperature: %v", dp1.Name(), temperatureMax)
	temperatureAvgSample = gnmi.Collect(t, gnmiOpts(t, dut1, gpb.SubscriptionMode_SAMPLE, time.Second*10), component1.Temperature().Avg().State(), time.Second*10)
	temperatureAvgSamples = temperatureAvgSample.Await(t)
	// Taking first sample
	temperatureAvg, _ = temperatureAvgSamples[0].Val()
	// temperature average should be between min and max
	if temperatureAvg >= temperatureMin && temperatureAvg <= temperatureMax {
		t.Logf("The average is between the maximum and minimum values")
	} else {
		t.Fatalf("The average is not between the maximum and minimum values")
	}
	i.Enabled = ygot.Bool(true)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	// Enable interface
	gnmi.Replace(t, dut1, gnmi.OC().Interface(dp1.Name()).Config(), i)
	gnmi.Await(t, dut1, gnmi.OC().Interface(dp1.Name()).OperStatus().State(), intUpdateTime, oc.Interface_OperStatus_UP)
	temperatureInstantSample = gnmi.Collect(t, gnmiOpts(t, dut1, gpb.SubscriptionMode_SAMPLE, time.Second*10), component1.Temperature().Instant().State(), time.Second*10)
	temperatureInstantSamples = temperatureInstantSample.Await(t)
	// Taking first sample
	temperatureInstant, _ = temperatureInstantSamples[0].Val()
	// Check temperature return value of correct type
	t.Logf("Port1 dut1 %s Instant Temperature: %v", dp1.Name(), temperatureInstant)
	temperatureMinSample = gnmi.Collect(t, gnmiOpts(t, dut1, gpb.SubscriptionMode_SAMPLE, time.Second*10), component1.Temperature().Min().State(), time.Second*10)
	temperatureMinSamples = temperatureMinSample.Await(t)
	// Taking first sample
	temperatureMin, _ = temperatureMinSamples[0].Val()
	t.Logf("Port1 dut1 %s Min Temperature: %v", dp1.Name(), temperatureMin)
	temperatureMaxSample = gnmi.Collect(t, gnmiOpts(t, dut1, gpb.SubscriptionMode_SAMPLE, time.Second*10), component1.Temperature().Max().State(), time.Second*10)
	temperatureMaxSamples = temperatureMaxSample.Await(t)
	// Taking first sample
	temperatureMax, _ = temperatureMaxSamples[0].Val()
	t.Logf("Port1 dut1 %s Max Temperature: %v", dp1.Name(), temperatureMax)
	temperatureAvgSample = gnmi.Collect(t, gnmiOpts(t, dut1, gpb.SubscriptionMode_SAMPLE, time.Second*10), component1.Temperature().Avg().State(), time.Second*10)
	temperatureAvgSamples = temperatureAvgSample.Await(t)
	// Taking first sample
	temperatureAvg, _ = temperatureAvgSamples[0].Val()
	if temperatureAvg >= temperatureMin && temperatureAvg <= temperatureMax {
		t.Logf("The average is between the maximum and minimum values")
	} else {
		t.Fatalf("The average is not between the maximum and minimum values")
	}
}
