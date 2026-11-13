// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package polarization provides validation and test helpers for optical-channel polarization telemetry.
package polarization

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/samplestream"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
)

const (
	minAllowedPMDValue            = 0.0
	maxAllowedPMDValue            = 200.0
	inactivePMDValue              = 1.0
	minAllowedSecondOrderPMDValue = 0.0
	maxAllowedSecondOrderPMDValue = 2000.0
	inactiveSecondOrderPMDValue   = 1.0
	minAllowedSOPROCValue         = 0.0
	maxAllowedSOPROCValue         = 5000.0
	inactiveSOPROCValue           = 1.0

	inactivePower     = -25.0
	powerLoss         = 2.0
	powerReadingError = 2.0
	errorTolerance    = 0.05
	samplingInterval  = 10 * time.Second
	timeout           = 10 * time.Minute

	componentPath = "openconfig/components/component[name=%v]"
)

// TunableParameters contains the tunable parameters for polarization telemetry tests.
type TunableParameters struct {
	OperationalModeList    cfgplugins.OperationalModeList
	FrequencyList          cfgplugins.FrequencyList
	TargetOpticalPowerList cfgplugins.TargetOpticalPowerList
}

type leafCheck struct {
	desc  string
	path  string
	value any
	err   error
}

// IsPolarizationStreamReady returns true if the optical channel stream is ready for polarization
// validation at the given operational status and config parameters.
func IsPolarizationStreamReady(val *oc.Component, operStatus oc.E_Interface_OperStatus, params *cfgplugins.ConfigParameters) bool {
	if val == nil || params == nil {
		return false
	}
	och := val.GetOpticalChannel()
	if och == nil || och.GetOperationalMode() != params.OperationalMode {
		return false
	}

	inPower := och.GetInputPower()
	outPower := och.GetOutputPower()
	if inPower == nil || inPower.Instant == nil || inPower.Avg == nil || inPower.Min == nil || inPower.Max == nil {
		return false
	}
	if outPower == nil || outPower.Instant == nil || outPower.Avg == nil || outPower.Min == nil || outPower.Max == nil {
		return false
	}

	switch operStatus {
	case oc.Interface_OperStatus_UP:
		powerReady := inPower.GetMax() <= (params.TargetOpticalPower+powerReadingError) &&
			inPower.GetMax() >= (params.TargetOpticalPower-powerLoss-powerReadingError) &&
			inPower.GetMin() <= (params.TargetOpticalPower+powerReadingError) &&
			inPower.GetMin() >= (params.TargetOpticalPower-powerLoss-powerReadingError) &&
			inPower.GetAvg() <= (inPower.GetMax()+math.Abs(inPower.GetMax())*errorTolerance) &&
			inPower.GetAvg() >= (inPower.GetMin()-math.Abs(inPower.GetMin())*errorTolerance) &&
			inPower.GetInstant() <= (inPower.GetMax()+math.Abs(inPower.GetMax())*errorTolerance) &&
			inPower.GetInstant() >= (inPower.GetMin()-math.Abs(inPower.GetMin())*errorTolerance) &&
			outPower.GetMax() <= (params.TargetOpticalPower+powerReadingError) &&
			outPower.GetMax() >= (params.TargetOpticalPower-powerReadingError) &&
			outPower.GetMin() <= (params.TargetOpticalPower+powerReadingError) &&
			outPower.GetMin() >= (params.TargetOpticalPower-powerReadingError) &&
			outPower.GetAvg() <= (outPower.GetMax()+math.Abs(outPower.GetMax())*errorTolerance) &&
			outPower.GetAvg() >= (outPower.GetMin()-math.Abs(outPower.GetMin())*errorTolerance) &&
			outPower.GetInstant() <= (outPower.GetMax()+math.Abs(outPower.GetMax())*errorTolerance) &&
			outPower.GetInstant() >= (outPower.GetMin()-math.Abs(outPower.GetMin())*errorTolerance)

		return powerReady &&
			och.GetPolarizationModeDispersion() != nil &&
			och.GetSecondOrderPolarizationModeDispersion() != nil &&
			och.GetSopRoc() != nil
	default:
		return inPower.GetMax() <= inactivePower &&
			inPower.GetMin() <= inactivePower &&
			inPower.GetAvg() <= inactivePower &&
			inPower.GetInstant() <= inactivePower &&
			outPower.GetMax() <= inactivePower &&
			outPower.GetMin() <= inactivePower &&
			outPower.GetAvg() <= inactivePower &&
			outPower.GetInstant() <= inactivePower
	}
}

func validateOpticalChannelPolarization(portName string, opticalChannelValue *oc.Component, params *cfgplugins.ConfigParameters, wantOperStatus oc.E_Interface_OperStatus) []leafCheck {
	var ochName string
	if params != nil && params.OpticalChannelNames != nil {
		ochName = params.OpticalChannelNames[portName]
	}
	if ochName == "" && opticalChannelValue != nil {
		ochName = opticalChannelValue.GetName()
	}

	if opticalChannelValue == nil || opticalChannelValue.GetOpticalChannel() == nil {
		return []leafCheck{{
			desc:  "Optical Channel Polarization Container Validation",
			path:  fmt.Sprintf(componentPath+"/optical-channel/state", ochName),
			value: "<nil>",
			err:   fmt.Errorf("optical-channel component/state is nil for port %v", portName),
		}}
	}

	if wantOperStatus != oc.Interface_OperStatus_UP && wantOperStatus != oc.Interface_OperStatus_DOWN {
		return []leafCheck{{
			desc:  "Optical Channel Polarization Status Validation",
			path:  fmt.Sprintf(componentPath+"/optical-channel/state", ochName),
			value: wantOperStatus,
			err:   fmt.Errorf("unsupported target oper status: %v", wantOperStatus),
		}}
	}

	och := opticalChannelValue.GetOpticalChannel()
	var checks []leafCheck

	pmd := och.GetPolarizationModeDispersion()
	checks = append(checks, checkPolarizationContainer(
		ochName,
		"Polarization Mode Dispersion",
		"polarization-mode-dispersion",
		wantOperStatus,
		minAllowedPMDValue,
		maxAllowedPMDValue,
		inactivePMDValue,
		pmd != nil,
		func() (*float64, *float64, *float64, *float64) {
			if pmd == nil {
				return nil, nil, nil, nil
			}
			return pmd.Instant, pmd.Avg, pmd.Min, pmd.Max
		},
	)...)

	sopmd := och.GetSecondOrderPolarizationModeDispersion()
	checks = append(checks, checkPolarizationContainer(
		ochName,
		"Second Order Polarization Mode Dispersion",
		"second-order-polarization-mode-dispersion",
		wantOperStatus,
		minAllowedSecondOrderPMDValue,
		maxAllowedSecondOrderPMDValue,
		inactiveSecondOrderPMDValue,
		sopmd != nil,
		func() (*float64, *float64, *float64, *float64) {
			if sopmd == nil {
				return nil, nil, nil, nil
			}
			return sopmd.Instant, sopmd.Avg, sopmd.Min, sopmd.Max
		},
	)...)

	sopROC := och.GetSopRoc()
	checks = append(checks, checkPolarizationContainer(
		ochName,
		"SOP-ROC",
		"sop-roc",
		wantOperStatus,
		minAllowedSOPROCValue,
		maxAllowedSOPROCValue,
		inactiveSOPROCValue,
		sopROC != nil,
		func() (*float64, *float64, *float64, *float64) {
			if sopROC == nil {
				return nil, nil, nil, nil
			}
			return sopROC.Instant, sopROC.Avg, sopROC.Min, sopROC.Max
		},
	)...)

	return checks
}

func checkPolarizationContainer(
	ochName string,
	metricDesc string,
	metricPath string,
	wantOperStatus oc.E_Interface_OperStatus,
	minAllowed float64,
	maxAllowed float64,
	inactiveAllowed float64,
	containerPresent bool,
	extract func() (instant, avg, minVal, maxVal *float64),
) []leafCheck {
	basePath := fmt.Sprintf(componentPath+"/optical-channel/state/%s", ochName, metricPath)
	if !containerPresent {
		var err error
		if wantOperStatus == oc.Interface_OperStatus_UP {
			err = fmt.Errorf("container is nil while oper-status is UP")
		}
		return []leafCheck{{
			desc:  fmt.Sprintf("Optical Channel %s Validation", metricDesc),
			path:  basePath,
			value: "<nil>",
			err:   err,
		}}
	}

	instant, avg, minVal, maxVal := extract()
	leaves := []struct {
		label string
		name  string
		ptr   *float64
		bound bool
	}{
		{label: "Instant", name: "instant", ptr: instant, bound: true},
		{label: "Average", name: "avg", ptr: avg, bound: true},
		{label: "Minimum", name: "min", ptr: minVal, bound: false},
		{label: "Maximum", name: "max", ptr: maxVal, bound: false},
	}

	var checks []leafCheck
	for _, leaf := range leaves {
		path := fmt.Sprintf("%s/%s", basePath, leaf.name)
		desc := fmt.Sprintf("Optical Channel %s %s Validation", leaf.label, metricDesc)

		if leaf.ptr == nil {
			var err error
			if wantOperStatus == oc.Interface_OperStatus_UP {
				err = fmt.Errorf("leaf is nil while oper-status is UP")
			}
			checks = append(checks, leafCheck{
				desc:  desc,
				path:  path,
				value: "<nil>",
				err:   err,
			})
			continue
		}

		val := *leaf.ptr
		var err error
		if math.IsNaN(val) || math.IsInf(val, 0) {
			err = fmt.Errorf("invalid non-finite float64 value: %v", val)
		} else {
			switch wantOperStatus {
			case oc.Interface_OperStatus_UP:
				if val < minAllowed || val > maxAllowed {
					err = fmt.Errorf("out of range: got %v, want >= %v and <= %v", val, minAllowed, maxAllowed)
				} else if minVal != nil && maxVal != nil {
					minFinite := !math.IsNaN(*minVal) && !math.IsInf(*minVal, 0)
					maxFinite := !math.IsNaN(*maxVal) && !math.IsInf(*maxVal, 0)
					if minFinite && maxFinite {
						if *minVal > *maxVal && (leaf.name == "min" || leaf.name == "max") {
							err = fmt.Errorf("invalid statistical range: min (%v) > max (%v)", *minVal, *maxVal)
						} else if *minVal <= *maxVal && leaf.bound {
							statMin := *minVal - math.Abs(*minVal)*errorTolerance
							statMax := *maxVal + math.Abs(*maxVal)*errorTolerance
							if val < statMin || val > statMax {
								err = fmt.Errorf("outside statistical bounds [%v, %v]: got %v (min=%v, max=%v, tolerance=%v)", statMin, statMax, val, *minVal, *maxVal, errorTolerance)
							}
						}
					}
				}
			case oc.Interface_OperStatus_DOWN:
				if val > inactiveAllowed {
					err = fmt.Errorf("out of range while DOWN: got %v, want <= %v", val, inactiveAllowed)
				}
			}
		}

		checks = append(checks, leafCheck{
			desc:  desc,
			path:  path,
			value: val,
			err:   err,
		})
	}

	return checks
}

// ValidateOpticalChannelPolarizationComponent validates the 12 optical-channel polarization telemetry
// leaves against expected bounds for the target operational status.
func ValidateOpticalChannelPolarizationComponent(t *testing.T, portName string, opticalChannelValue *oc.Component, params *cfgplugins.ConfigParameters, wantOperStatus oc.E_Interface_OperStatus) {
	t.Helper()
	checks := validateOpticalChannelPolarization(portName, opticalChannelValue, params, wantOperStatus)
	for _, c := range checks {
		t.Run(fmt.Sprintf("%s of %v", c.desc, portName), func(t *testing.T) {
			t.Logf("\n%s: %s = %v\n\n", portName, c.path, c.value)
			if c.err != nil {
				t.Errorf("\n%s: %s: %v\n\n", portName, c.path, c.err)
			}
		})
	}
}

// PolarizationTelemetryTest runs the standalone optical-channel polarization telemetry streaming
// test across interface DOWN -> UP -> DOWN state transitions.
func PolarizationTelemetryTest(t *testing.T, tp *TunableParameters) {
	t.Helper()

	dut := ondatra.DUT(t, "dut")
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	ports := dut.Ports()
	if len(ports) < 2 || len(ports)%2 != 0 {
		t.Fatalf("Expected an even number of DUT ports >= 2, got %d", len(ports))
	}

	if tp == nil {
		tp = &TunableParameters{}
	}
	setToDefaults(t, dut, tp)

	for _, operationalMode := range tp.OperationalModeList {
		for _, frequency := range tp.FrequencyList {
			for _, targetOpticalPower := range tp.TargetOpticalPowerList {
				t.Run(fmt.Sprintf("mode_%d_freq_%d_power_%v", operationalMode, frequency, targetOpticalPower), func(t *testing.T) {
					t.Logf("Step 1: Configuring interfaces in disabled state (OperationalMode=%v, Frequency=%v, TargetOpticalPower=%v)", operationalMode, frequency, targetOpticalPower)
					params := &cfgplugins.ConfigParameters{
						Enabled:            false,
						Frequency:          frequency,
						TargetOpticalPower: targetOpticalPower,
						OperationalMode:    operationalMode,
					}
					batch := &gnmi.SetBatch{}
					cfgplugins.NewInterfaceConfigAll(t, dut, batch, params)

					cleanupParams := *params
					cleanupParams.Enabled = true
					t.Cleanup(func() {
						t.Logf("Cleanup: restoring interfaces to enabled state.")
						cfgplugins.ToggleInterfaceState(t, dut, &cleanupParams)
					})
					batch.Set(t, dut)

					ochStreams := make(map[string]*samplestream.SampleStream[*oc.Component], len(ports))
					for _, p := range ports {
						stream := samplestream.New(t, dut, gnmi.OC().Component(params.OpticalChannelNames[p.Name()]).State(), samplingInterval)
						ochStreams[p.Name()] = stream
						t.Cleanup(stream.Close)
					}

					t.Run("Step 2: Validate polarization telemetry in initial DOWN state", func(t *testing.T) {
						validatePortsPolarizationState(t, dut, ports, params, oc.Interface_OperStatus_DOWN, ochStreams)
					})

					t.Run("Step 3: Enable interfaces and validate polarization telemetry in UP state", func(t *testing.T) {
						params.Enabled = true
						cfgplugins.ToggleInterfaceState(t, dut, params)
						validatePortsPolarizationState(t, dut, ports, params, oc.Interface_OperStatus_UP, ochStreams)
					})

					t.Run("Step 4: Flap interfaces DOWN, validate polarization telemetry, and restore UP", func(t *testing.T) {
						params.Enabled = false
						cfgplugins.ToggleInterfaceState(t, dut, params)
						validatePortsPolarizationState(t, dut, ports, params, oc.Interface_OperStatus_DOWN, ochStreams)

						params.Enabled = true
						cfgplugins.ToggleInterfaceState(t, dut, params)
						for _, p := range ports {
							interfaceName := cfgplugins.InterfaceName(dut, p, params)
							gnmi.Await(t, dut, gnmi.OC().Interface(interfaceName).OperStatus().State(), timeout, oc.Interface_OperStatus_UP)
						}
					})
				})
			}
		}
	}
}

func validatePortsPolarizationState(
	t *testing.T,
	dut *ondatra.DUTDevice,
	ports []*ondatra.Port,
	params *cfgplugins.ConfigParameters,
	wantOperStatus oc.E_Interface_OperStatus,
	ochStreams map[string]*samplestream.SampleStream[*oc.Component],
) {
	t.Helper()
	for _, p := range ports {
		interfaceName := cfgplugins.InterfaceName(dut, p, params)
		gnmi.Await(t, dut, gnmi.OC().Interface(interfaceName).OperStatus().State(), timeout, wantOperStatus)

		nextSample, ok := ochStreams[p.Name()].AwaitNext(timeout, func(v *ygnmi.Value[*oc.Component]) bool {
			val, present := v.Val()
			return present && IsPolarizationStreamReady(val, wantOperStatus, params)
		})
		if !ok {
			t.Fatalf("Optical channel %v did not reach ready polarization telemetry state for %v within %v.", params.OpticalChannelNames[p.Name()], wantOperStatus, timeout)
		}
		ochVal, ok := nextSample.Val()
		if !ok {
			t.Fatalf("Optical channel sample value is empty for port %v.", p.Name())
		}
		ValidateOpticalChannelPolarizationComponent(t, p.Name(), ochVal, params, wantOperStatus)
	}
}

func setToDefaults(t *testing.T, dut *ondatra.DUTDevice, tp *TunableParameters) {
	t.Helper()
	if len(tp.OperationalModeList) == 0 {
		tp.OperationalModeList = tp.OperationalModeList.Default(t, dut)
	}
	if len(tp.FrequencyList) == 0 {
		tp.FrequencyList = tp.FrequencyList.Default(t, dut)
	}
	if len(tp.TargetOpticalPowerList) == 0 {
		tp.TargetOpticalPowerList = tp.TargetOpticalPowerList.Default(t, dut)
	}
}
