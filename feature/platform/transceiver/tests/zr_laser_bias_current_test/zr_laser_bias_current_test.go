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

package zr_laser_bias_current_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/components"
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

// Topology:
//   dut:port1 <--> port2:dut
//

func verifyLaserBiasCurrent(t *testing.T, pStream *samplestream.SampleStream[float64], sensorName string) float64 {
	laserBias := pStream.Next()
	if laserBias == nil {
		t.Fatalf("laserBias telemetry %q  was not streamed in the most recent subscription interval", sensorName)
	}
	laserBiasVal, ok := laserBias.Val()
	if !ok {
		t.Fatalf("LaserBias %q telemetry is not present", sensorName)
	}
	if reflect.TypeOf(laserBiasVal).Kind() != reflect.Float64 {
		t.Errorf("Return value is not type float64")
	}
	if laserBiasVal <= 0 && laserBiasVal >= 131 {
		t.Errorf("The laser bias value is not between 0 and 131")
	}
	t.Logf("laserBias value: %f", laserBiasVal)
	return laserBiasVal
}

func verifyLaserBiasCurrentAll(t *testing.T, pStreamInstant *samplestream.SampleStream[float64], pStreamAvg *samplestream.SampleStream[float64], pStreamMax *samplestream.SampleStream[float64], pStreamMin *samplestream.SampleStream[float64]) {
	laserbiasInstant := verifyLaserBiasCurrent(t, pStreamInstant, "laserbiasInstant")
	t.Logf("laserBias Instant value: %f", laserbiasInstant)
	laserbiasMin := verifyLaserBiasCurrent(t, pStreamMin, "laserbiasMin")
	t.Logf("laserBias Min value: %f", laserbiasMin)
	laserbiasMax := verifyLaserBiasCurrent(t, pStreamMax, "laserbiasMax")
	t.Logf("laserBias Max value: %f", laserbiasMax)
	laserbiasAvg := verifyLaserBiasCurrent(t, pStreamAvg, "laserbiasAvg")
	t.Logf("laserBias Avg value: %f", laserbiasAvg)
	if laserbiasAvg >= laserbiasMin && laserbiasAvg <= laserbiasMax {
		t.Logf("The average is between the maximum and minimum values")
	} else {
		t.Fatalf("The average is not between the maximum and minimum values Avg:%f Min:%f Max:%f", laserbiasAvg, laserbiasMin, laserbiasMax)
	}
}

func interfaceConfig(t *testing.T, dut1 *ondatra.DUTDevice, dp *ondatra.Port) {
	d := &oc.Root{}
	i := d.GetOrCreateInterface(dp.Name())
	i.Enabled = ygot.Bool(true)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	gnmi.Replace(t, dut1, gnmi.OC().Interface(dp.Name()).Config(), i)
	OCcomponent := components.OpticalChannelComponentFromPort(t, dut1, dp)
	gnmi.Replace(t, dut1, gnmi.OC().Component(OCcomponent).OpticalChannel().Config(), &oc.Component_OpticalChannel{
		TargetOutputPower: ygot.Float64(targetOutputPower),
		Frequency:         ygot.Uint64(frequency),
	})
}

func TestZRLaserBiasCurrentState(t *testing.T) {
	dut1 := ondatra.DUT(t, "dut")
	dp1 := dut1.Port(t, "port1")
	dp2 := dut1.Port(t, "port2")
	t.Logf("dut1: %v", dut1)
	t.Logf("dut1 dp1 name: %v", dp1.Name())
	interfaceConfig(t, dut1, dp1)
	interfaceConfig(t, dut1, dp2)
	intUpdateTime := 2 * time.Minute
	gnmi.Await(t, dut1, gnmi.OC().Interface(dp1.Name()).OperStatus().State(), intUpdateTime, oc.Interface_OperStatus_UP)
	transceiverState := gnmi.Get(t, dut1, gnmi.OC().Interface(dp1.Name()).Transceiver().State())
	if dp1.PMD() != ondatra.PMD400GBASEZR {
		t.Fatalf("%s Transceiver is not 400ZR its of type: %v", transceiverState, dp1.PMD())
	}
	OCcomponent := components.OpticalChannelComponentFromPort(t, dut1, dp1)
	component1 := gnmi.OC().Component(OCcomponent)
	p1StreamInstant := samplestream.New(t, dut1, component1.OpticalChannel().LaserBiasCurrent().Instant().State(), 10*time.Second)
	p1StreamMin := samplestream.New(t, dut1, component1.OpticalChannel().LaserBiasCurrent().Min().State(), 10*time.Second)
	p1StreamMax := samplestream.New(t, dut1, component1.OpticalChannel().LaserBiasCurrent().Max().State(), 10*time.Second)
	p1StreamAvg := samplestream.New(t, dut1, component1.OpticalChannel().LaserBiasCurrent().Avg().State(), 10*time.Second)
	defer p1StreamAvg.Close()
	defer p1StreamMax.Close()
	defer p1StreamMin.Close()
	defer p1StreamInstant.Close()
	verifyLaserBiasCurrentAll(t, p1StreamInstant, p1StreamAvg, p1StreamMax, p1StreamMin)
}

func TestZRLaserBiasCurrentStateInterface_Flap(t *testing.T) {
	dut1 := ondatra.DUT(t, "dut")
	dp1 := dut1.Port(t, "port1")
	dp2 := dut1.Port(t, "port2")
	t.Logf("dut1: %v", dut1)
	t.Logf("dut1 dp1 name: %v", dp1.Name())
	interfaceConfig(t, dut1, dp1)
	interfaceConfig(t, dut1, dp2)
	intUpdateTime := 2 * time.Minute
	// Check interface is up
	gnmi.Await(t, dut1, gnmi.OC().Interface(dp1.Name()).OperStatus().State(), intUpdateTime, oc.Interface_OperStatus_UP)
	// Check if TRANSCEIVER is of type 400ZR
	transceiverState := gnmi.Get(t, dut1, gnmi.OC().Interface(dp1.Name()).Transceiver().State())
	if dp1.PMD() != ondatra.PMD400GBASEZR {
		t.Fatalf("%s Transceiver is not 400ZR its of type: %v", transceiverState, dp1.PMD())
	}
	// Disable interface
	d := &oc.Root{}
	i := d.GetOrCreateInterface(dp1.Name())
	i.Enabled = ygot.Bool(false)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	gnmi.Replace(t, dut1, gnmi.OC().Interface(dp1.Name()).Config(), i)
	OCcomponent := components.OpticalChannelComponentFromPort(t, dut1, dp1)
	component1 := gnmi.OC().Component(OCcomponent)
	p1StreamInstant := samplestream.New(t, dut1, component1.OpticalChannel().LaserBiasCurrent().Instant().State(), 10*time.Second)
	p1StreamMin := samplestream.New(t, dut1, component1.OpticalChannel().LaserBiasCurrent().Min().State(), 10*time.Second)
	p1StreamMax := samplestream.New(t, dut1, component1.OpticalChannel().LaserBiasCurrent().Max().State(), 10*time.Second)
	p1StreamAvg := samplestream.New(t, dut1, component1.OpticalChannel().LaserBiasCurrent().Avg().State(), 10*time.Second)
	defer p1StreamInstant.Close()
	defer p1StreamMin.Close()
	defer p1StreamMax.Close()
	defer p1StreamAvg.Close()
	verifyLaserBiasCurrentAll(t, p1StreamInstant, p1StreamAvg, p1StreamMax, p1StreamMin)
	// Wait 120 sec cooling off period
	gnmi.Await(t, dut1, gnmi.OC().Interface(dp1.Name()).OperStatus().State(), intUpdateTime, oc.Interface_OperStatus_DOWN)
	// Enable interface
	verifyLaserBiasCurrentAll(t, p1StreamInstant, p1StreamAvg, p1StreamMax, p1StreamMin)
	i.Enabled = ygot.Bool(true)
	gnmi.Replace(t, dut1, gnmi.OC().Interface(dp1.Name()).Config(), i)
	gnmi.Await(t, dut1, gnmi.OC().Interface(dp1.Name()).OperStatus().State(), intUpdateTime, oc.Interface_OperStatus_UP)
	verifyLaserBiasCurrentAll(t, p1StreamInstant, p1StreamAvg, p1StreamMax, p1StreamMin)
}

func TestZRLaserBiasCurrentStateTransceiverOnOff(t *testing.T) {
	dut1 := ondatra.DUT(t, "dut")
	dp1 := dut1.Port(t, "port1")
	dp2 := dut1.Port(t, "port2")
	t.Logf("dut1: %v", dut1)
	t.Logf("dut1 dp1 name: %v", dp1.Name())
	interfaceConfig(t, dut1, dp1)
	interfaceConfig(t, dut1, dp2)
	intUpdateTime := 2 * time.Minute
	gnmi.Await(t, dut1, gnmi.OC().Interface(dp1.Name()).OperStatus().State(), intUpdateTime, oc.Interface_OperStatus_UP)
	transceiverState := gnmi.Get(t, dut1, gnmi.OC().Interface(dp1.Name()).Transceiver().State())
	// Check if TRANSCEIVER is of type 400ZR
	if dp1.PMD() != ondatra.PMD400GBASEZR {
		t.Fatalf("%s Transceiver is not 400ZR its of type: %v", transceiverState, dp1.PMD())
	}
	OCcomponent := components.OpticalChannelComponentFromPort(t, dut1, dp1)
	component1 := gnmi.OC().Component(OCcomponent)
	p1StreamInstant := samplestream.New(t, dut1, component1.OpticalChannel().LaserBiasCurrent().Instant().State(), 10*time.Second)
	p1StreamMin := samplestream.New(t, dut1, component1.OpticalChannel().LaserBiasCurrent().Min().State(), 10*time.Second)
	p1StreamMax := samplestream.New(t, dut1, component1.OpticalChannel().LaserBiasCurrent().Max().State(), 10*time.Second)
	p1StreamAvg := samplestream.New(t, dut1, component1.OpticalChannel().LaserBiasCurrent().Avg().State(), 10*time.Second)
	defer p1StreamInstant.Close()
	defer p1StreamMin.Close()
	defer p1StreamMax.Close()
	defer p1StreamAvg.Close()
	// Disable interface transceiver power off
	gnmi.Update(t, dut1, gnmi.OC().Component(dp1.Name()).Transceiver().Enabled().Config(), false)
	verifyLaserBiasCurrentAll(t, p1StreamInstant, p1StreamAvg, p1StreamMax, p1StreamMin)
	// Enable interface transceiver power on
	gnmi.Update(t, dut1, gnmi.OC().Component(dp1.Name()).Transceiver().Enabled().Config(), true)
	gnmi.Await(t, dut1, gnmi.OC().Interface(dp1.Name()).OperStatus().State(), intUpdateTime, oc.Interface_OperStatus_UP)
	verifyLaserBiasCurrentAll(t, p1StreamInstant, p1StreamAvg, p1StreamMax, p1StreamMin)
}
