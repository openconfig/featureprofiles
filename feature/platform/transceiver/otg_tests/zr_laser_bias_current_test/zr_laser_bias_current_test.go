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

	"github.com/openconfig/featureprofiles/internal/fptest"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
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

func gnmiOpts(t *testing.T, dut *ondatra.DUTDevice, mode gpb.SubscriptionMode, interval time.Duration) *gnmi.Opts {
	return dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(mode), ygnmi.WithSampleInterval(interval))
}

func verifyLaserBiasCurrent(t *testing.T, dut1 *ondatra.DUTDevice, transceiverName string, interfaceName string, interfaceState bool) {

	component1 := gnmi.OC().Component(transceiverName)
	laserBiasInstantSample := gnmi.Collect(t, gnmiOpts(t, dut1, gpb.SubscriptionMode_SAMPLE, time.Second*30), component1.OpticalChannel().LaserBiasCurrent().Instant().State(), time.Second*30)
	laserBiasInstantSamples := laserBiasInstantSample.Await(t)
	if len(laserBiasInstantSamples) == 0 {
		t.Fatalf("Get laser bias Instant sample list for interface %q: got 0, want > 0", interfaceName)
	}
	// Taking first sample
	laserBiasInstant, _ := laserBiasInstantSamples[0].Val()
	// Check laser bias return value of correct type
	if reflect.TypeOf(laserBiasInstant).Kind() != reflect.Float64 {
		t.Fatalf("Return value is not type float64")
	}
	t.Logf("Port1 dut1 %s laser bias current instant: %v", interfaceName, laserBiasInstant)
	laserBiasMinSample := gnmi.Collect(t, gnmiOpts(t, dut1, gpb.SubscriptionMode_SAMPLE, time.Second*30), component1.OpticalChannel().LaserBiasCurrent().Min().State(), time.Second*30)
	laserBiasMinSamples := laserBiasMinSample.Await(t)
	if len(laserBiasMinSamples) == 0 {
		t.Fatalf("Get Temeprature Min sample list for interface %q: got 0, want > 0", interfaceName)
	}
	// Taking first sample
	laserBiasMin, _ := laserBiasMinSamples[0].Val()
	// Check laser bias return value of correct type
	if reflect.TypeOf(laserBiasMin).Kind() != reflect.Float64 {
		t.Fatalf("Return value is not type float64")
	}
	t.Logf("Port1 dut1 %s laser bias current Min: %v", interfaceName, laserBiasMin)
	laserBiasMaxSample := gnmi.Collect(t, gnmiOpts(t, dut1, gpb.SubscriptionMode_SAMPLE, time.Second*30), component1.OpticalChannel().LaserBiasCurrent().Max().State(), time.Second*30)
	laserBiasMaxSamples := laserBiasMaxSample.Await(t)
	if len(laserBiasMaxSamples) == 0 {
		t.Fatalf("Get Temeprature Max sample list for interface %q: got 0, want > 0", interfaceName)
	}
	// Taking first sample
	laserBiasMax, _ := laserBiasMaxSamples[0].Val()
	// Check laser bias return value of correct type
	if reflect.TypeOf(laserBiasMax).Kind() != reflect.Float64 {
		t.Fatalf("Return value is not type float64")
	}
	t.Logf("Port1 dut1 %s laser bias current Max: %v", interfaceName, laserBiasMax)
	laserBiasAvgSample := gnmi.Collect(t, gnmiOpts(t, dut1, gpb.SubscriptionMode_SAMPLE, time.Second*30), component1.OpticalChannel().LaserBiasCurrent().Avg().State(), time.Second*30)
	laserBiasAvgSamples := laserBiasAvgSample.Await(t)
	if len(laserBiasAvgSamples) == 0 {
		t.Fatalf("Get Temeprature Avg sample list for interface %q: got 0, want > 0", interfaceName)
	}
	// Taking first sample
	laserBiasAvg, _ := laserBiasAvgSamples[0].Val()
	// Check laser bias return value of correct type
	if reflect.TypeOf(laserBiasAvg).Kind() != reflect.Float64 {
		t.Fatalf("Return value is not type float64")
	}
	t.Logf("Port1 dut1 %s laser bias current Avg: %v", interfaceName, laserBiasAvg)
	if interfaceState {
		if laserBiasAvg >= laserBiasMin && laserBiasAvg <= laserBiasMax {
			t.Logf("The average is between the maximum and minimum values")
		} else {
			t.Fatalf("The average is not between the maximum and minimum values")
		}
		if laserBiasAvg <= 0 && laserBiasAvg >= 131 {
			t.Fatalf("The laser bias Avg is not between 0 and 131")
		}
		if laserBiasMin <= 0 && laserBiasMin >= 131 {
			t.Fatalf("The laser bias Min is not between 0 and 131")
		}
		if laserBiasMax <= 0 && laserBiasMax >= 131 {
			t.Fatalf("The laser bias Max is not between 0 and 131")
		}
		if laserBiasInstant <= 0 && laserBiasInstant >= 131 {
			t.Fatalf("The laser bias Instant is not between 0 and 131")
		}
	} else {
		if laserBiasInstant != 0 {
			t.Fatalf("The laser bias Instant is not 0 when interface is down")
		}
	}
}
func TestZRLaserBiasCurrentState(t *testing.T) {
	dut1 := ondatra.DUT(t, "dut1")
	dp1 := dut1.Port(t, "port1")
	t.Logf("dut1: %v", dut1)
	t.Logf("dut1 dp1 name: %v", dp1.Name())
	d := &oc.Root{}
	i := d.GetOrCreateInterface(dp1.Name())
	i.Enabled = ygot.Bool(true)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	intUpdateTime := 2 * time.Minute
	gnmi.Replace(t, dut1, gnmi.OC().Interface(dp1.Name()).Config(), i)
	gnmi.Await(t, dut1, gnmi.OC().Interface(dp1.Name()).OperStatus().State(), intUpdateTime, oc.Interface_OperStatus_UP)
	transceiverName := gnmi.Get(t, dut1, gnmi.OC().Interface(dp1.Name()).Transceiver().State())
	// Check if TRANSCEIVER is of type 400ZR
	if dp1.PMD() != ondatra.PMD400GBASEZR {
		t.Fatalf("%s Transceiver is not 400ZR its of type: %v", transceiverName, dp1.PMD())
	}
	verifyLaserBiasCurrent(t, dut1, transceiverName, dp1.Name(), true)
}
func TestZRLaserBiasCurrentStateInterface_Flap(t *testing.T) {
	dut1 := ondatra.DUT(t, "dut1")
	dp1 := dut1.Port(t, "port1")
	t.Logf("dut1: %v", dut1)
	t.Logf("dut1 dp1 name: %v", dp1.Name())
	d := &oc.Root{}
	i := d.GetOrCreateInterface(dp1.Name())
	i.Enabled = ygot.Bool(false)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	// Disable interface
	gnmi.Replace(t, dut1, gnmi.OC().Interface(dp1.Name()).Config(), i)
	transceiverName := gnmi.Get(t, dut1, gnmi.OC().Interface(dp1.Name()).Transceiver().State())
	// Check if TRANSCEIVER is of type 400ZR
	if dp1.PMD() != ondatra.PMD400GBASEZR {
		t.Fatalf("%s Transceiver is not 400ZR its of type: %v", transceiverName, dp1.PMD())
	}
	// During interface disable temperature sensor should not return invalid type
	verifyLaserBiasCurrent(t, dut1, transceiverName, dp1.Name(), false)
	// Wait 120 sec cooling off period
	intUpdateTime := 2 * time.Minute
	gnmi.Await(t, dut1, gnmi.OC().Interface(dp1.Name()).OperStatus().State(), intUpdateTime, oc.Interface_OperStatus_DOWN)
	verifyLaserBiasCurrent(t, dut1, transceiverName, dp1.Name(), false)
	i.Enabled = ygot.Bool(true)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	// Enable interface
	gnmi.Replace(t, dut1, gnmi.OC().Interface(dp1.Name()).Config(), i)
	gnmi.Await(t, dut1, gnmi.OC().Interface(dp1.Name()).OperStatus().State(), intUpdateTime, oc.Interface_OperStatus_UP)
	verifyLaserBiasCurrent(t, dut1, transceiverName, dp1.Name(), true)
}
func TestZRLaserBiasCurrentStateTransceiverOnOff(t *testing.T) {
	dut1 := ondatra.DUT(t, "dut1")
	dp1 := dut1.Port(t, "port1")
	t.Logf("dut1: %v", dut1)
	t.Logf("dut1 dp1 name: %v", dp1.Name())
	transceiverName := gnmi.Get(t, dut1, gnmi.OC().Interface(dp1.Name()).Transceiver().State())
	// Check if TRANSCEIVER is of type 400ZR
	if dp1.PMD() != ondatra.PMD400GBASEZR {
		t.Fatalf("%s Transceiver is not 400ZR its of type: %v", transceiverName, dp1.PMD())
	}
	// Disable interface transceiver power off
	gnmi.Update(t, dut1, gnmi.OC().Component(dp1.Name()).Transceiver().Enabled().Config(), false)
	verifyLaserBiasCurrent(t, dut1, transceiverName, dp1.Name(), false)
	// Enable interface transceiver power on
	intUpdateTime := 2 * time.Minute
	gnmi.Update(t, dut1, gnmi.OC().Component(dp1.Name()).Transceiver().Enabled().Config(), true)
	gnmi.Await(t, dut1, gnmi.OC().Interface(dp1.Name()).OperStatus().State(), intUpdateTime, oc.Interface_OperStatus_UP)
	verifyLaserBiasCurrent(t, dut1, transceiverName, dp1.Name(), true)
}
