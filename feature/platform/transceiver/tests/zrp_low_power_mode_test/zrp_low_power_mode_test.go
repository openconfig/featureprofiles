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

package zrp_low_power_mode_test

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
	intUpdateTime                 = 5 * time.Minute
	samplingInterval              = 10 * time.Second
	targetOutputPowerdBm          = -3
	targetOutputPowerTolerancedBm = 1
	targetFrequencyMHz            = 193100000
	targetFrequencyToleranceMHz   = 100000
)

var (
	operationalModeFlag = flag.Int("operational_mode", 5, "vendor-specific operational-mode for the channel")
	operationalMode     uint16
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// validateStreamOutput validates that the OC path is streamed in the most recent subscription interval.
func validateStreamOutput(t *testing.T, streams map[string]*samplestream.SampleStream[string]) {
	for key, stream := range streams {
		output := stream.Next()
		if output == nil {
			t.Fatalf("OC path for %s not streamed in the most recent subscription interval", key)
		}
		value, ok := output.Val()
		if !ok {
			t.Fatalf("Error capturing streaming value for %s", key)
		}
		if reflect.TypeOf(value).Kind() != reflect.String {
			t.Fatalf("Return value is not type string for key :%s", key)
		}
		if value == "" {
			t.Fatalf("OC path empty for %s", key)
		}
		t.Logf("Value for OC path %s: %s", key, value)
	}
}

// validateOutputPower validates that the output power is streamed in the most recent subscription interval.
func validateOutputPower(t *testing.T, streams map[string]*samplestream.SampleStream[float64]) {
	for key, stream := range streams {
		outputStream := stream.Next()
		if outputStream == nil {
			t.Fatalf("OC path for %s not streamed in the most recent subscription interval", key)
		}
		outputPower, ok := outputStream.Val()
		if !ok {
			t.Fatalf("Error capturing streaming value for %s", key)
		}
		// Check output power value is of correct type
		if reflect.TypeOf(outputPower).Kind() != reflect.Float64 {
			t.Fatalf("Return value is not type float64 for key :%s", key)
		}
		t.Logf("Output power for %s: %f", key, outputPower)
	}
}

func TestLowPowerMode(t *testing.T) {
	if operationalModeFlag != nil {
		operationalMode = uint16(*operationalModeFlag)
	} else {
		t.Fatalf("Please specify the vendor-specific operational-mode flag")
	}
	dut := ondatra.DUT(t, "dut")
	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")
	t.Logf("dut1: %v", dut)
	t.Logf("dut1 dp1 name: %v", dp1.Name())
	och1 := components.OpticalChannelComponentFromPort(t, dut, dp1)
	och2 := components.OpticalChannelComponentFromPort(t, dut, dp2)
	cfgplugins.ConfigOpticalChannel(t, dut, och1, targetFrequencyMHz, targetOutputPowerdBm, operationalMode)
	cfgplugins.ConfigOpticalChannel(t, dut, och2, targetFrequencyMHz, targetOutputPowerdBm, operationalMode)
	for _, port := range []string{"port1", "port2"} {
		t.Run(fmt.Sprintf("Port:%s", port), func(t *testing.T) {
			dp := dut.Port(t, port)
			gnmi.Await(t, dut, gnmi.OC().Interface(dp.Name()).OperStatus().State(), intUpdateTime, oc.Interface_OperStatus_UP)

			// Derive transceiver names from ports.
			tr := gnmi.Get(t, dut, gnmi.OC().Interface(dp.Name()).Transceiver().State())
			// Stream all inventory information.
			streamSerialNo := samplestream.New(t, dut, gnmi.OC().Component(tr).SerialNo().State(), samplingInterval)
			defer streamSerialNo.Close()
			streamPartNo := samplestream.New(t, dut, gnmi.OC().Component(tr).PartNo().State(), samplingInterval)
			defer streamPartNo.Close()
			streamType := samplestream.New(t, dut, gnmi.OC().Component(tr).Type().State(), samplingInterval)
			defer streamType.Close()
			// TODO: b/333021032 - Uncomment the description check from the test once the bug is fixed.
			// streamDescription := samplestream.New(t, dut, gnmi.OC().Component(tr).Description().State(), samplingInterval)
			// defer streamDescription.Close()
			streamMfgName := samplestream.New(t, dut, gnmi.OC().Component(tr).MfgName().State(), samplingInterval)
			defer streamMfgName.Close()
			streamMfgDate := samplestream.New(t, dut, gnmi.OC().Component(tr).MfgDate().State(), samplingInterval)
			defer streamMfgDate.Close()
			streamHwVersion := samplestream.New(t, dut, gnmi.OC().Component(tr).HardwareVersion().State(), samplingInterval)
			defer streamHwVersion.Close()
			streamFirmwareVersion := samplestream.New(t, dut, gnmi.OC().Component(tr).FirmwareVersion().State(), samplingInterval)
			defer streamFirmwareVersion.Close()

			allStream := map[string]*samplestream.SampleStream[string]{
				"serialNo": streamSerialNo,
				"partNo":   streamPartNo,
				// "description":     streamDescription,
				"mfgName":         streamMfgName,
				"mfgDate":         streamMfgDate,
				"hwVersion":       streamHwVersion,
				"firmwareVersion": streamFirmwareVersion,
			}
			validateStreamOutput(t, allStream)

			// Disable interface
			cfgplugins.ToggleInterface(t, dut, dp.Name(), false)
			// Wait for interface to go down.
			gnmi.Await(t, dut, gnmi.OC().Interface(dp.Name()).OperStatus().State(), intUpdateTime, oc.Interface_OperStatus_DOWN)
			time.Sleep(3 * samplingInterval) // Wait an extra sample interval to ensure the device has time to process the change.
			validateStreamOutput(t, allStream)
			opticalChannelName := components.OpticalChannelComponentFromPort(t, dut, dp)
			samplingInterval := time.Duration(gnmi.Get(t, dut, gnmi.OC().Component(opticalChannelName).OpticalChannel().OutputPower().Interval().State())) * time.Second
			opInst := samplestream.New(t, dut, gnmi.OC().Component(opticalChannelName).OpticalChannel().OutputPower().Instant().State(), samplingInterval)
			defer opInst.Close()
			if opInstN := opInst.Next(); opInstN != nil {
				if val, ok := opInstN.Val(); ok && val != -40 {
					t.Logf("streaming /components/component/optical-channel/state/output-power/instant is reported: %f", val)
					t.Fatalf("streaming /components/component/optical-channel/state/output-power/instant is not expected to be reported")
				}
			}

			opAvg := samplestream.New(t, dut, gnmi.OC().Component(opticalChannelName).OpticalChannel().OutputPower().Avg().State(), samplingInterval)
			defer opAvg.Close()
			if opAvgN := opAvg.Next(); opAvgN != nil {
				if val, ok := opAvgN.Val(); ok && val != -40 {
					t.Logf("streaming /components/component/optical-channel/state/output-power/avg is reported: %f", val)
					t.Fatalf("streaming /components/component/optical-channel/state/output-power/avg is not expected to be reported")
				}
			}

			opMin := samplestream.New(t, dut, gnmi.OC().Component(opticalChannelName).OpticalChannel().OutputPower().Min().State(), samplingInterval)
			defer opMin.Close()
			if opMinN := opMin.Next(); opMinN != nil {
				if val, ok := opMinN.Val(); ok && val != -40 {
					t.Logf("streaming /components/component/optical-channel/state/output-power/min is reported: %f", val)
					t.Fatalf("streaming /components/component/optical-channel/state/output-power/min is not expected to be reported")
				}
			}

			opMax := samplestream.New(t, dut, gnmi.OC().Component(opticalChannelName).OpticalChannel().OutputPower().Max().State(), samplingInterval)
			defer opMax.Close()
			if opMaxN := opMax.Next(); opMaxN != nil {
				if val, ok := opMaxN.Val(); ok && val != -40 {
					t.Logf("streaming /components/component/optical-channel/state/output-power/max is reported: %f", val)
					t.Fatalf("streaming /components/component/optical-channel/state/output-power/max is not expected to be reported")
				}
			}

			// Enable interface
			cfgplugins.ToggleInterface(t, dut, dp.Name(), true)
			// Wait for interface to go up.
			gnmi.Await(t, dut, gnmi.OC().Interface(dp.Name()).OperStatus().State(), intUpdateTime, oc.Interface_OperStatus_UP)

			powerStreamMap := map[string]*samplestream.SampleStream[float64]{
				"inst": opInst,
				"avg":  opAvg,
				"min":  opMin,
				"max":  opMax,
			}
			validateOutputPower(t, powerStreamMap)
			// TODO: jchenjian - Uncomment the output power and frequency checks from the test once the bug b/382296833 is fixed.
			// cfgplugins.ValidateInterfaceConfig(t, dut, dp, targetOutputPowerdBm, targetFrequencyMHz, targetOutputPowerTolerancedBm, targetFrequencyToleranceMHz)
		})
	}
}
