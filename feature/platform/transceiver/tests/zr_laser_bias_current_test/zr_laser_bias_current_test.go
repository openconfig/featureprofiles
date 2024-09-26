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
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/samplestream"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Topology:
//   dut:port1 <--> port2:dut
//

func verifyLaserBiasValue(t *testing.T, laserBiasValue float64) {
	t.Helper()
	if laserBiasValue <= 0 && laserBiasValue >= 131 {
		t.Errorf("The laser bias value is not between 0 and 131")
	}
}

func verifyLaserBiasCurrentAll(t *testing.T, p1Stream *samplestream.SampleStream[*oc.Component_OpticalChannel_LaserBiasCurrent], dut1 *ondatra.DUTDevice) {
	laserBias := p1Stream.Next()
	if laserBias == nil {
		t.Fatalf("laserBias telemetry was not streamed in the most recent subscription interval")
	}
	laserBiasVal, ok := laserBias.Val()
	if !ok {
		t.Fatalf("LaserBias telemetry is not present")
	}
	laserBiasInstant := laserBiasVal.GetInstant()
	t.Logf("laserBias Instant value: %f", laserBiasInstant)
	if deviations.MissingZROpticalChannelTunableParametersTelemetry(dut1) {
		t.Log("Skipping Min/Max/Avg Tunable Parameters Telemetry validation. Deviation MissingZROpticalChannelTunableParametersTelemetry enabled.")
	} else {
		laserBiasMin := laserBiasVal.GetMin()
		verifyLaserBiasValue(t, laserBiasMin)
		t.Logf("laserBias Min value: %f", laserBiasMin)
		laserBiasMax := laserBiasVal.GetMax()
		verifyLaserBiasValue(t, laserBiasMax)
		t.Logf("laserBias Max value: %f", laserBiasMax)
		laserBiasAvg := laserBiasVal.GetAvg()
		verifyLaserBiasValue(t, laserBiasAvg)
		t.Logf("laserBias Avg value: %f", laserBiasMin)
		if laserBiasAvg >= laserBiasMin && laserBiasAvg <= laserBiasMax {
			t.Logf("The average %f is between the maximum and minimum values", laserBiasAvg)
		} else {
			t.Fatalf("The average is not between the maximum and minimum values Avg:%f Min:%f Max:%f", laserBiasAvg, laserBiasMin, laserBiasMax)
		}
	}
}

func TestZRLaserBiasCurrentState(t *testing.T) {
	dut1 := ondatra.DUT(t, "dut")
	dp1 := dut1.Port(t, "port1")
	dp2 := dut1.Port(t, "port2")
	t.Logf("dut1: %v", dut1)
	t.Logf("dut1 dp1 name: %v", dp1.Name())
	cfgplugins.InterfaceConfig(t, dut1, dp1)
	cfgplugins.InterfaceConfig(t, dut1, dp2)
	intUpdateTime := 2 * time.Minute
	gnmi.Await(t, dut1, gnmi.OC().Interface(dp1.Name()).OperStatus().State(), intUpdateTime, oc.Interface_OperStatus_UP)
	transceiverState := gnmi.Get(t, dut1, gnmi.OC().Interface(dp1.Name()).Transceiver().State())
	if dp1.PMD() != ondatra.PMD400GBASEZR {
		t.Fatalf("%s Transceiver is not 400ZR its of type: %v", transceiverState, dp1.PMD())
	}
	componentName := components.OpticalChannelComponentFromPort(t, dut1, dp1)
	component := gnmi.OC().Component(componentName)
	p1Stream := samplestream.New(t, dut1, component.OpticalChannel().LaserBiasCurrent().State(), 10*time.Second)
	defer p1Stream.Close()
	verifyLaserBiasCurrentAll(t, p1Stream, dut1)
}

func TestZRLaserBiasCurrentStateInterfaceFlap(t *testing.T) {
	dut1 := ondatra.DUT(t, "dut")
	dp1 := dut1.Port(t, "port1")
	dp2 := dut1.Port(t, "port2")
	t.Logf("dut1: %v", dut1)
	t.Logf("dut1 dp1 name: %v", dp1.Name())
	cfgplugins.InterfaceConfig(t, dut1, dp1)
	cfgplugins.InterfaceConfig(t, dut1, dp2)
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
	gnmi.Replace(t, dut1, gnmi.OC().Interface(dp1.Name()).Name().Config(), dp1.Name())
	gnmi.Replace(t, dut1, gnmi.OC().Interface(dp1.Name()).Config(), i)
	componentName := components.OpticalChannelComponentFromPort(t, dut1, dp1)
	component := gnmi.OC().Component(componentName)
	p1Stream := samplestream.New(t, dut1, component.OpticalChannel().LaserBiasCurrent().State(), 10*time.Second)
	defer p1Stream.Close()
	verifyLaserBiasCurrentAll(t, p1Stream, dut1)
	// Wait 120 sec cooling-off period
	gnmi.Await(t, dut1, gnmi.OC().Interface(dp1.Name()).OperStatus().State(), intUpdateTime, oc.Interface_OperStatus_DOWN)
	verifyLaserBiasCurrentAll(t, p1Stream, dut1)
	// Enable interface
	i.Enabled = ygot.Bool(true)
	gnmi.Replace(t, dut1, gnmi.OC().Interface(dp1.Name()).Config(), i)
	gnmi.Await(t, dut1, gnmi.OC().Interface(dp1.Name()).OperStatus().State(), intUpdateTime, oc.Interface_OperStatus_UP)
	verifyLaserBiasCurrentAll(t, p1Stream, dut1)
}

func TestZRLaserBiasCurrentStateTransceiverOnOff(t *testing.T) {
	dut1 := ondatra.DUT(t, "dut")
	dp1 := dut1.Port(t, "port1")
	dp2 := dut1.Port(t, "port2")
	t.Logf("dut1: %v", dut1)
	t.Logf("dut1 dp1 name: %v", dp1.Name())
	cfgplugins.InterfaceConfig(t, dut1, dp1)
	cfgplugins.InterfaceConfig(t, dut1, dp2)
	intUpdateTime := 2 * time.Minute
	gnmi.Await(t, dut1, gnmi.OC().Interface(dp1.Name()).OperStatus().State(), intUpdateTime, oc.Interface_OperStatus_UP)
	transceiverState := gnmi.Get(t, dut1, gnmi.OC().Interface(dp1.Name()).Transceiver().State())
	// Check if TRANSCEIVER is of type 400ZR
	if dp1.PMD() != ondatra.PMD400GBASEZR {
		t.Fatalf("%s Transceiver is not 400ZR its of type: %v", transceiverState, dp1.PMD())
	}
	componentName := components.OpticalChannelComponentFromPort(t, dut1, dp1)
	component := gnmi.OC().Component(componentName)
	p1Stream := samplestream.New(t, dut1, component.OpticalChannel().LaserBiasCurrent().State(), 10*time.Second)
	defer p1Stream.Close()
	verifyLaserBiasCurrentAll(t, p1Stream, dut1)
	// power off interface transceiver
	gnmi.Update(t, dut1, gnmi.OC().Component(dp1.Name()).Name().Config(), dp1.Name())
	gnmi.Update(t, dut1, gnmi.OC().Component(dp1.Name()).Transceiver().Enabled().Config(), false)
	verifyLaserBiasCurrentAll(t, p1Stream, dut1)
	// power on interface transceiver
	gnmi.Update(t, dut1, gnmi.OC().Component(dp1.Name()).Transceiver().Enabled().Config(), true)
	gnmi.Await(t, dut1, gnmi.OC().Interface(dp1.Name()).OperStatus().State(), intUpdateTime, oc.Interface_OperStatus_UP)
	verifyLaserBiasCurrentAll(t, p1Stream, dut1)
}
