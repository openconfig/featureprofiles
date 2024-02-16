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
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Topology:
//   dut1:port1 <--> port1:dut2
//

func TestZRTemperatureState(t *testing.T) {
	dut1 := ondatra.DUT(t, "dut1")
	dp1 := dut1.Port(t, "port1")
	t.Logf("dut1: %v", dut1)
	t.Logf("dut1 dp1 name: %v", dp1.Name())
	// Check if TRANSCEIVER is of type 400ZR
	if dp1.PMD() != ondatra.PMD400GBASEZR {
		t.Fatalf("%s Transceiver is not 400ZR its of type: %v", dp1, dp1.PMD())
	}
	component1 := gnmi.OC().Component(dp1.Name())
	temperatureInstantdp1 := gnmi.Get(t, dut1, component1.Temperature().Instant().State())
	// Check temperature return value of correct type
	if reflect.TypeOf(temperatureInstantdp1).Kind() != reflect.Float64 {
		t.Fatalf("Return value is not type float64")
	}
	t.Logf("Port1 dut1 %s Instant Temperature: %v", dp1.Name(), temperatureInstantdp1)
	temperatureMin := gnmi.Get(t, dut1, component1.Temperature().Min().State())
	t.Logf("Port1 dut1 %s Min Temperature: %v", dp1.Name(), temperatureMin)
	if reflect.TypeOf(temperatureMin).Kind() != reflect.Float64 {
		t.Fatalf("Return value is not type float64")
	}
	temperatureMax := gnmi.Get(t, dut1, component1.Temperature().Max().State())
	t.Logf("Port1 dut1 %s Max Temperature: %v", dp1.Name(), temperatureMax)
	if reflect.TypeOf(temperatureMax).Kind() != reflect.Float64 {
		t.Fatalf("Return value is not type float64")
	}
	temperatureAvg := gnmi.Get(t, dut1, component1.Temperature().Avg().State())
	if reflect.TypeOf(temperatureAvg).Kind() != reflect.Float64 {
		t.Fatalf("Return value is not type float64")
	}
	t.Logf("Port1 dut1 %s Avg Temperature: %v", dp1.Name(), temperatureAvg)
	// temperature average should be between min and max
	if temperatureAvg >= temperatureMin && temperatureAvg <= temperatureMax {
		t.Logf("The average is between the maximum and minimum values")
	} else {
		t.Fatalf("The average is not between the maximum and minimum values")
	}
}
func TestZRTemperatureStateInterfaceFlap(t *testing.T) {
	dut1 := ondatra.DUT(t, "dut")
	dp1 := dut1.Port(t, "port1")
	t.Logf("dut1: %v", dut1)
	t.Logf("dut1 dp1 name: %v", dp1.Name())
	// Check if TRANSCEIVER is of type 400ZR
	if dp1.PMD() != ondatra.PMD400GBASEZR {
		t.Fatalf("%s Transceiver is not 400ZR its of type: %v", dp1, dp1.PMD())
	}
	component1 := gnmi.OC().Component(dp1.Name())
	d := &oc.Root{}
	i := d.GetOrCreateInterface(dp1.Name())
	i.Enabled = ygot.Bool(false)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	// Disable interface
	gnmi.Replace(t, dut1, gnmi.OC().Interface(dp1.Name()).Config(), i)
	// During interface disable temperature sensor should not return invalid type
	temperatureInstantdp1 := gnmi.Get(t, dut1, component1.Temperature().Instant().State())
	// Check temperature return value of correct type
	t.Logf("Port1 dut1 %s Instant Temperature: %v", dp1.Name(), temperatureInstantdp1)
	temperatureMin := gnmi.Get(t, dut1, component1.Temperature().Min().State())
	t.Logf("Port1 dut1 %s Min Temperature: %v", dp1.Name(), temperatureMin)
	temperatureMax := gnmi.Get(t, dut1, component1.Temperature().Max().State())
	t.Logf("Port1 dut1 %s Max Temperature: %v", dp1.Name(), temperatureMax)
	temperatureAvg := gnmi.Get(t, dut1, component1.Temperature().Avg().State())
	t.Logf("Port1 dut1 %s Avg Temperature: %v", dp1.Name(), temperatureAvg)
	if reflect.TypeOf(temperatureInstantdp1).Kind() != reflect.Float64 {
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
	gnmi.Await(t, dut1, gnmi.OC().Interface(dp1.Name()).OperStatus().State(), intUpdateTime, oc.Interface_OperStatus_DOWN)
	temperatureMin = gnmi.Get(t, dut1, component1.Temperature().Min().State())
	t.Logf("Port1 dut1 %s MinTemperature: %v", dp1.Name(), temperatureMin)
	temperatureMax = gnmi.Get(t, dut1, component1.Temperature().Max().State())
	t.Logf("Port1 dut1 %s Max Temperature: %v", dp1.Name(), temperatureMax)
	temperatureAvg = gnmi.Get(t, dut1, component1.Temperature().Avg().State())
	t.Logf("Port1 dut1 %s Avg Temperature: %v", dp1.Name(), temperatureAvg)
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
	temperatureMin = gnmi.Get(t, dut1, component1.Temperature().Min().State())
	t.Logf("Port1 dut1 %s Min Temperature: %v", dp1.Name(), temperatureMin)
	temperatureMax = gnmi.Get(t, dut1, component1.Temperature().Max().State())
	t.Logf("Port1 dut1 %s Max Temperature: %v", dp1.Name(), temperatureMax)
	temperatureAvg = gnmi.Get(t, dut1, component1.Temperature().Avg().State())
	t.Logf("Port1 dut1 %s Avg Temperature: %v", dp1.Name(), temperatureAvg)
	if temperatureAvg >= temperatureMin && temperatureAvg <= temperatureMax {
		t.Logf("The average is between the maximum and minimum values")
	} else {
		t.Fatalf("The average is not between the maximum and minimum values")
	}
}
