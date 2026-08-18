// Copyright 2026 Google LLC
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

package client_optics_dom_telemetry_test

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/openconfig/ygot/ygot"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/functional-translators/registrar"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/gnmi/oc/platform"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ygnmi/ygnmi"
)

const (
	ethernetCsmacd          = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	transceiverType         = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_TRANSCEIVER
	sensorType              = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_SENSOR
	sleepDuration           = time.Minute
	minOpticsPower          = -40.0
	maxOpticsPower          = 10.0
	maxOpticsPowerAdminDown = -30.0
	minOpticsHighThreshold  = 1.0
	maxOpticsLowThreshold   = -1.0
)

type testStatus string

const (
	statusPass    testStatus = "PASS"
	statusFail    testStatus = "FAIL"
	statusSkipped testStatus = "SKIP"
	statusWarning testStatus = "WARN"
)

type traceRecord struct {
	TestName string
	Path     string
	Status   testStatus
	Detail   string
}

var (
	traceMutex sync.Mutex
	traceLogs  []traceRecord
)

func tracePath(t *testing.T, path string, status testStatus, format string, args ...any) {
	t.Helper()
	detail := fmt.Sprintf(format, args...)
	traceMutex.Lock()
	defer traceMutex.Unlock()
	traceLogs = append(traceLogs, traceRecord{
		TestName: t.Name(),
		Path:     path,
		Status:   status,
		Detail:   detail,
	})
	if status == statusFail {
		t.Errorf("[%s] FAIL: %s - %s", t.Name(), path, detail)
	} else if status == statusWarning {
		t.Logf("[%s] WARN: %s - %s", t.Name(), path, detail)
	}
}

func getPathStr(pathStruct ygnmi.PathStruct) string {
	resolved, _, err := ygnmi.ResolvePath(pathStruct)
	if err != nil {
		return fmt.Sprintf("%v", pathStruct)
	}
	var elems []string
	for _, e := range resolved.GetElem() {
		if len(e.GetKey()) > 0 {
			var keys []string
			for k, v := range e.GetKey() {
				keys = append(keys, fmt.Sprintf("%s=%s", k, v))
			}
			elems = append(elems, fmt.Sprintf("%s[%s]", e.GetName(), strings.Join(keys, ",")))
		} else {
			elems = append(elems, e.GetName())
		}
	}
	return "/" + strings.Join(elems, "/")
}

func printSummary() {
	fmt.Println("\n==========================================================================================")
	fmt.Println("                           OPENCONFIG TEST PATH CHECK SUMMARY                            ")
	fmt.Println("==========================================================================================")

	// Print failures first
	fmt.Println("\n--- FAILURES ---")
	failCount := 0
	for _, r := range traceLogs {
		if r.Status == statusFail {
			fmt.Printf("[FAIL] Test: %-40s\n       Path: %s\n       Reason: %s\n\n", r.TestName, r.Path, r.Detail)
			failCount++
		}
	}
	if failCount == 0 {
		fmt.Println(" None")
	}

	// Print warnings
	fmt.Println("--- WARNINGS ---")
	warnCount := 0
	for _, r := range traceLogs {
		if r.Status == statusWarning {
			fmt.Printf("[WARN] Test: %-40s\n       Path: %s\n       Reason: %s\n\n", r.TestName, r.Path, r.Detail)
			warnCount++
		}
	}
	if warnCount == 0 {
		fmt.Println(" None")
	}

	// Print skips
	fmt.Println("--- SKIPPED ---")
	skipCount := 0
	for _, r := range traceLogs {
		if r.Status == statusSkipped {
			fmt.Printf("[SKIP] Test: %-40s\n       Path: %s\n       Reason: %s\n\n", r.TestName, r.Path, r.Detail)
			skipCount++
		}
	}
	if skipCount == 0 {
		fmt.Println(" None")
	}

	// Print passes (condensed)
	fmt.Println("--- PASSES ---")
	passCount := 0
	for _, r := range traceLogs {
		if r.Status == statusPass {
			fmt.Printf("[PASS] %s -> %s\n", r.Path, r.Detail)
			passCount++
		}
	}
	if passCount == 0 {
		fmt.Println(" None")
	}

	fmt.Println("\n==========================================================================================")
	fmt.Printf("TOTAL VERIFIED PATHS: %d | PASS: %d | WARN: %d | FAIL: %d | SKIPPED: %d\n", len(traceLogs), passCount, warnCount, failCount, skipCount)
	fmt.Println("==========================================================================================")
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
	printSummary()
}

func getOptsForFunctionalTranslator(t *testing.T, dut *ondatra.DUTDevice, functionalTranslatorName string) []ygnmi.Option {
	if functionalTranslatorName == "" {
		return nil
	}
	ft, ok := registrar.FunctionalTranslatorRegistry[functionalTranslatorName]
	if !ok {
		t.Fatalf("Functional translator %s is not registered", functionalTranslatorName)
	}
	deviceSoftwareVersion := strings.Split(dut.Version(), "-")[0]
	ftMetadata := ft.Metadata()
	for _, m := range ftMetadata {
		if m.SoftwareVersion == deviceSoftwareVersion {
			return []ygnmi.Option{ygnmi.WithFT(ft)}
		}
	}
	return nil
}

func gnmiOpts(t *testing.T, dut *ondatra.DUTDevice, mode gpb.SubscriptionMode, interval time.Duration, functionalTranslatorName ...string) *gnmi.Opts {
	opts := []ygnmi.Option{ygnmi.WithSubscriptionMode(mode), ygnmi.WithSampleInterval(interval)}
	if len(functionalTranslatorName) > 0 {
		opts = append(opts, getOptsForFunctionalTranslator(t, dut, functionalTranslatorName[0])...)
	}
	return dut.GNMIOpts().WithYGNMIOpts(opts...)
}

type checkThresholdParams struct {
	transceiver string
	isPortUp    bool
	opts        []ygnmi.Option
	lowerPath   ygnmi.SingletonQuery[float64]
	upperPath   ygnmi.SingletonQuery[float64]
	instantPath ygnmi.SingletonQuery[float64]
	name        string
	ftName      string
}

func checkThreshold(t *testing.T, dut *ondatra.DUTDevice, params checkThresholdParams) {
	t.Helper()
	lV := gnmi.Lookup(t, dut.GNMIOpts().WithYGNMIOpts(params.opts...), params.lowerPath)
	uV := gnmi.Lookup(t, dut.GNMIOpts().WithYGNMIOpts(params.opts...), params.upperPath)
	l, lOK := lV.Val()
	u, uOK := uV.Val()

	lowerPathStr := getPathStr(params.lowerPath.PathStruct())
	upperPathStr := getPathStr(params.upperPath.PathStruct())
	instantPathStr := getPathStr(params.instantPath.PathStruct())

	if !lOK {
		tracePath(t, lowerPathStr, statusFail, "Transceiver %s: threshold %s-lower is not set", params.transceiver, params.name)
	} else {
		tracePath(t, lowerPathStr, statusPass, "Value: %v", l)
	}
	if !uOK {
		tracePath(t, upperPathStr, statusFail, "Transceiver %s: threshold %s-upper is not set", params.transceiver, params.name)
	} else {
		tracePath(t, upperPathStr, statusPass, "Value: %v", u)
	}

	iV := gnmi.Lookup(t, dut.GNMIOpts().WithYGNMIOpts(getOptsForFunctionalTranslator(t, dut, params.ftName)...), params.instantPath)
	i, iOK := iV.Val()
	if !iOK {
		tracePath(t, instantPathStr, statusFail, "Transceiver %s: instant %s is not set", params.transceiver, params.name)
	} else {
		if lOK && uOK {
			if !params.isPortUp {
				tracePath(t, instantPathStr, statusSkipped, "Skipping range check for transceiver %s because port is not UP.", params.transceiver)
			} else if (params.name == "input-power" || params.name == "output-power") && i == minOpticsPower {
				tracePath(t, instantPathStr, statusSkipped, "Skipping range check because instant value is %v (link not stable).", minOpticsPower)
			} else if i < l || i > u {
				tracePath(t, instantPathStr, statusFail, "Transceiver %s: instant value %v is outside threshold range [%v, %v]", params.transceiver, i, l, u)
			} else {
				tracePath(t, instantPathStr, statusPass, "Transceiver %s: instant value %v is within threshold range [%v, %v]", params.transceiver, i, l, u)
			}
		}
	}

	if lOK && uOK && l >= u {
		tracePath(t, lowerPathStr, statusFail, "Transceiver %s: %s-lower (%v) must be less than %s-upper (%v)", params.transceiver, params.name, l, params.name, u)
	}
}

func validateThresholds(t *testing.T, dut *ondatra.DUTDevice, transceiver string, isPortUp bool, sev oc.E_AlarmTypes_OPENCONFIG_ALARM_SEVERITY, component *platform.ComponentPath, opts []ygnmi.Option) {
	t.Helper()
	threshold := component.Transceiver().Threshold(sev)

	checkThreshold(t, dut, checkThresholdParams{
		transceiver: transceiver,
		isPortUp:    isPortUp,
		opts:        opts,
		lowerPath:   threshold.ModuleTemperatureLower().State(),
		upperPath:   threshold.ModuleTemperatureUpper().State(),
		instantPath: component.Temperature().Instant().State(),
		name:        "module-temperature",
	})
	checkThreshold(t, dut, checkThresholdParams{
		transceiver: transceiver,
		isPortUp:    isPortUp,
		opts:        opts,
		lowerPath:   threshold.InputPowerLower().State(),
		upperPath:   threshold.InputPowerUpper().State(),
		instantPath: component.Transceiver().Channel(0).InputPower().Instant().State(),
		name:        "input-power",
		ftName:      deviations.CiscoxrTransceiverFt(dut),
	})
	checkThreshold(t, dut, checkThresholdParams{
		transceiver: transceiver,
		isPortUp:    isPortUp,
		opts:        opts,
		lowerPath:   threshold.OutputPowerLower().State(),
		upperPath:   threshold.OutputPowerUpper().State(),
		instantPath: component.Transceiver().Channel(0).OutputPower().Instant().State(),
		name:        "output-power",
		ftName:      deviations.CiscoxrTransceiverFt(dut),
	})
	checkThreshold(t, dut, checkThresholdParams{
		transceiver: transceiver,
		isPortUp:    isPortUp,
		opts:        opts,
		lowerPath:   threshold.LaserBiasCurrentLower().State(),
		upperPath:   threshold.LaserBiasCurrentUpper().State(),
		instantPath: component.Transceiver().Channel(0).LaserBiasCurrent().Instant().State(),
		name:        "laser-bias-current",
		ftName:      deviations.CiscoxrTransceiverFt(dut),
	})
	checkThreshold(t, dut, checkThresholdParams{
		transceiver: transceiver,
		isPortUp:    isPortUp,
		opts:        opts,
		lowerPath:   threshold.SupplyVoltageLower().State(),
		upperPath:   threshold.SupplyVoltageUpper().State(),
		instantPath: component.Transceiver().SupplyVoltage().Instant().State(),
		name:        "supply-voltage",
		ftName:      deviations.CiscoxrTransceiverFt(dut),
	})
}

func TestOpticsPowerBiasCurrent(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	ports := dut.Ports()
	for _, dp := range ports {
		t.Run(dp.Name(), func(t *testing.T) {
			transceiverPath := gnmi.OC().Interface(dp.Name()).Transceiver().State().PathStruct()
			transceiverLookup := gnmi.Lookup(t, dut, gnmi.OC().Interface(dp.Name()).Transceiver().State())
			transceiverName, ok := transceiverLookup.Val()
			if !ok || transceiverName == "" {
				tracePath(t, getPathStr(transceiverPath), statusFail, "Failed to find transceiver for port %q", dp.Name())
				t.Fatalf("Failed to find transceiver for port %q", dp.Name())
			}
			tracePath(t, getPathStr(transceiverPath), statusPass, "Transceiver: %s", transceiverName)

			component := gnmi.OC().Component(transceiverName)
			mfgLookup := gnmi.Lookup(t, dut, component.MfgName().State())
			mfgName, ok := mfgLookup.Val()
			if !ok {
				tracePath(t, getPathStr(component.MfgName().State().PathStruct()), statusFail, "MfgName not defined")
			} else {
				tracePath(t, getPathStr(component.MfgName().State().PathStruct()), statusPass, "MfgName: %s", mfgName)
			}

			// Check admin status using state/admin-status and config/enabled
			adminStatusLookup := gnmi.Lookup(t, dut, gnmi.OC().Interface(dp.Name()).AdminStatus().State())
			adminStatus, adminStatusPresent := adminStatusLookup.Val()
			adminStatusPathStr := getPathStr(gnmi.OC().Interface(dp.Name()).AdminStatus().State().PathStruct())

			configEnabledLookup := gnmi.Lookup(t, dut, gnmi.OC().Interface(dp.Name()).Enabled().Config())
			configEnabled, configEnabledPresent := configEnabledLookup.Val()
			configEnabledPathStr := getPathStr(gnmi.OC().Interface(dp.Name()).Enabled().Config().PathStruct())

			operStatusLookup := gnmi.Lookup(t, dut, gnmi.OC().Interface(dp.Name()).OperStatus().State())
			operStatus, operStatusPresent := operStatusLookup.Val()
			operStatusPathStr := getPathStr(gnmi.OC().Interface(dp.Name()).OperStatus().State().PathStruct())

			isAdminDown := false
			if adminStatusPresent && adminStatus == oc.Interface_AdminStatus_DOWN {
				isAdminDown = true
			} else if configEnabledPresent && !configEnabled {
				isAdminDown = true
			}

			if adminStatusPresent {
				tracePath(t, adminStatusPathStr, statusPass, "Interface AdminStatus: %v", adminStatus)
			} else {
				tracePath(t, adminStatusPathStr, statusSkipped, "Interface AdminStatus path not present on device")
			}

			if configEnabledPresent {
				tracePath(t, configEnabledPathStr, statusPass, "Interface Enabled Config: %v", configEnabled)
			} else {
				tracePath(t, configEnabledPathStr, statusSkipped, "Interface Enabled Config path not present on device")
			}

			if operStatusPresent {
				tracePath(t, operStatusPathStr, statusPass, "Interface OperStatus: %v", operStatus)
			} else {
				tracePath(t, operStatusPathStr, statusSkipped, "Interface OperStatus path not present on device")
			}

			if isAdminDown {
				tracePath(t, adminStatusPathStr, statusSkipped, "Skipping transceiver %s because interface is admin down", transceiverName)
				return
			}

			optsForTransceiver := gnmiOpts(t, dut, gpb.SubscriptionMode_SAMPLE, time.Second*30, deviations.CiscoxrTransceiverFt(dut))

			inputPowerPathStr := getPathStr(component.Transceiver().ChannelAny().InputPower().Instant().State().PathStruct())
			inputPowers := gnmi.CollectAll(t, optsForTransceiver, component.Transceiver().ChannelAny().InputPower().Instant().State(), time.Second*30).Await(t)
			if len(inputPowers) == 0 {
				tracePath(t, inputPowerPathStr, statusFail, "Get inputPowers list: got 0, want > 0")
			} else {
				for _, val := range inputPowers {
					if v, ok := val.Val(); ok {
						pStr, err := ygot.PathToString(val.Path)
						if err != nil {
							pStr = val.Path.String()
						}
						tracePath(t, pStr, statusPass, "Value: %v", v)
					}
				}
			}

			outputPowerPathStr := getPathStr(component.Transceiver().ChannelAny().OutputPower().Instant().State().PathStruct())
			outputPowers := gnmi.CollectAll(t, optsForTransceiver, component.Transceiver().ChannelAny().OutputPower().Instant().State(), time.Second*30).Await(t)
			if len(outputPowers) == 0 {
				tracePath(t, outputPowerPathStr, statusFail, "Get outputPowers list: got 0, want > 0")
			} else {
				for _, val := range outputPowers {
					if v, ok := val.Val(); ok {
						pStr, err := ygot.PathToString(val.Path)
						if err != nil {
							pStr = val.Path.String()
						}
						tracePath(t, pStr, statusPass, "Value: %v", v)
					}
				}
			}

			biasPathStr := getPathStr(component.Transceiver().ChannelAny().LaserBiasCurrent().Instant().State().PathStruct())
			biasCurrents := gnmi.CollectAll(t, optsForTransceiver, component.Transceiver().ChannelAny().LaserBiasCurrent().Instant().State(), time.Second*30).Await(t)
			if len(biasCurrents) == 0 {
				tracePath(t, biasPathStr, statusFail, "Get biasCurrents list: got 0, want > 0")
			} else {
				for _, val := range biasCurrents {
					if v, ok := val.Val(); ok {
						pStr, err := ygot.PathToString(val.Path)
						if err != nil {
							pStr = val.Path.String()
						}
						tracePath(t, pStr, statusPass, "Value: %v", v)
					}
				}
			}

			subcomponentNames := gnmi.LookupAll[string](t, dut, component.SubcomponentAny().Name().State())
			sensorComponentChecked := false
			for _, s := range subcomponentNames {
				subcName, ok := s.Val()
				if ok {
					sensorTypeLookup := gnmi.Lookup(t, dut, gnmi.OC().Component(subcName).Type().State())
					sType, ok := sensorTypeLookup.Val()
					if ok && sType == sensorType {
						scomponent := gnmi.OC().Component(subcName)
						sensorComponentChecked = true
						descLookup := gnmi.Lookup(t, dut, scomponent.Description().State())
						desc, _ := descLookup.Val()
						if !deviations.TemperatureSensorCheck(dut) || strings.Contains(desc, "Temperature Sensor") {
							tempPathStr := getPathStr(scomponent.Temperature().Instant().State().PathStruct())
							v := gnmi.Lookup(t, dut, scomponent.Temperature().Instant().State())
							if val, ok := v.Val(); !ok {
								tracePath(t, tempPathStr, statusFail, "Sensor %s: Temperature instant is not defined", subcName)
							} else {
								tracePath(t, tempPathStr, statusPass, "Temperature value: %v", val)
							}
						}
					}
				}
			}
			if len(subcomponentNames) == 0 || !sensorComponentChecked {
				tempPathStr := getPathStr(component.Temperature().Instant().State().PathStruct())
				v := gnmi.Lookup(t, dut, component.Temperature().Instant().State())
				if val, ok := v.Val(); !ok {
					tracePath(t, tempPathStr, statusFail, "Transceiver %s: Temperature instant is not defined", transceiverName)
				} else {
					tracePath(t, tempPathStr, statusPass, "Temperature value: %v", val)
				}
			}

			if deviations.TransceiverThresholdsUnsupported(dut) {
				tracePath(t, getPathStr(component.Transceiver().ThresholdAny().Severity().State().PathStruct()), statusSkipped, "Skipping verification of transceiver threshold leaves due to deviation")
				return
			}

			opts := getOptsForFunctionalTranslator(t, dut, deviations.CiscoxrLaserFt(dut))
			operStatusVal := gnmi.Get(t, dut, gnmi.OC().Interface(dp.Name()).OperStatus().State())
			isPortUp := operStatusVal == oc.Interface_OperStatus_UP
			validateThresholds(t, dut, transceiverName, isPortUp, oc.AlarmTypes_OPENCONFIG_ALARM_SEVERITY_CRITICAL, component, opts)
		})
	}
}

func TestOpticsPowerUpdate(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ports := dut.Ports()

	for _, dp := range ports {
		t.Run(fmt.Sprintf("Flap-%s", dp.Name()), func(t *testing.T) {
			d := &oc.Root{}
			i := d.GetOrCreateInterface(dp.Name())

			cases := []struct {
				desc                string
				IntfStatus          bool
				expectedStatus      oc.E_Interface_OperStatus
				expectedMaxOutPower float64
				checkMinOutPower    bool
			}{{
				desc:                "Check initial input and output optics powers are OK",
				IntfStatus:          true,
				expectedStatus:      oc.Interface_OperStatus_UP,
				expectedMaxOutPower: maxOpticsPower,
				checkMinOutPower:    true,
			}, {
				desc:                "Check output optics power is very small after interface is disabled",
				IntfStatus:          false,
				expectedStatus:      oc.Interface_OperStatus_DOWN,
				expectedMaxOutPower: maxOpticsPowerAdminDown,
				checkMinOutPower:    false,
			}, {
				desc:                "Check output optics power is normal after interface is re-enabled",
				IntfStatus:          true,
				expectedStatus:      oc.Interface_OperStatus_UP,
				expectedMaxOutPower: maxOpticsPower,
				checkMinOutPower:    true,
			}}
			for _, tc := range cases {
				intUpdateTime := 2 * time.Minute
				t.Run(tc.desc, func(t *testing.T) {
					i.Enabled = ygot.Bool(tc.IntfStatus)
					i.Type = ethernetCsmacd
					if deviations.ExplicitPortSpeed(dut) {
						eth := i.GetOrCreateEthernet()
						speed := fptest.GetIfSpeed(t, dp)
						if speed == 0 {
							speed = oc.IfEthernet_ETHERNET_SPEED_SPEED_100GB
							t.Logf("Testbed port speed unspecified, defaulting to SPEED_100GB for breakout interface %s", dp.Name())
						}
						eth.PortSpeed = speed
						eth.DuplexMode = oc.Ethernet_DuplexMode_FULL
					}
					gnmi.Replace(t, dut, gnmi.OC().Interface(dp.Name()).Config(), i)
					gnmi.Await(t, dut, gnmi.OC().Interface(dp.Name()).OperStatus().State(), intUpdateTime, tc.expectedStatus)

					operPathStr := getPathStr(gnmi.OC().Interface(dp.Name()).OperStatus().State().PathStruct())
					tracePath(t, operPathStr, statusPass, "Reached expected operational status: %v", tc.expectedStatus)

					transceiverName := gnmi.Get(t, dut, gnmi.OC().Interface(dp.Name()).Transceiver().State())
					component := gnmi.OC().Component(transceiverName)
					if !gnmi.Lookup(t, dut, component.MfgName().State()).IsPresent() {
						tracePath(t, getPathStr(component.MfgName().State().PathStruct()), statusSkipped, "Skipping: component MfgName not present")
						t.Skipf("component.MfgName().Lookup(t).IsPresent() for %q is false. skip it", transceiverName)
					}

					mfgName := gnmi.Get(t, dut, component.MfgName().State())
					tracePath(t, getPathStr(component.MfgName().State().PathStruct()), statusPass, "MfgName: %s", mfgName)

					channels := gnmi.OC().Component(transceiverName).Transceiver().ChannelAny()
					opts := getOptsForFunctionalTranslator(t, dut, deviations.CiscoxrTransceiverFt(dut))

					inputPowers := gnmi.LookupAll(t, dut.GNMIOpts().WithYGNMIOpts(opts...), channels.InputPower().Instant().State())
					for _, inputPower := range inputPowers {
						pStr, err := ygot.PathToString(inputPower.Path)
						if err != nil {
							pStr = inputPower.Path.String()
						}
						inPower, ok := inputPower.Val()
						if !ok {
							tracePath(t, pStr, statusFail, "input power is not defined")
							continue
						}
						if inPower > maxOpticsPower || inPower < minOpticsPower {
							tracePath(t, pStr, statusFail, "value %.2f is outside range [%f, %f]", inPower, minOpticsPower, maxOpticsPower)
						} else {
							tracePath(t, pStr, statusPass, "value %.2f is within range [%f, %f]", inPower, minOpticsPower, maxOpticsPower)
						}
					}

					outputPowers := gnmi.LookupAll(t, dut.GNMIOpts().WithYGNMIOpts(opts...), channels.OutputPower().Instant().State())
					for _, outputPower := range outputPowers {
						pStr, err := ygot.PathToString(outputPower.Path)
						if err != nil {
							pStr = outputPower.Path.String()
						}
						outPower, ok := outputPower.Val()
						if !ok {
							tracePath(t, pStr, statusFail, "output power is not defined")
							continue
						}
						if outPower > tc.expectedMaxOutPower {
							tracePath(t, pStr, statusFail, "value %.2f is above maximum threshold <= %f", outPower, tc.expectedMaxOutPower)
						} else if tc.checkMinOutPower && outPower < minOpticsPower {
							tracePath(t, pStr, statusFail, "value %.2f is below minimum threshold >= %f", outPower, minOpticsPower)
						} else {
							tracePath(t, pStr, statusPass, "value %.2f is within expected range", outPower)
						}
					}

					if deviations.TransceiverThresholdsUnsupported(dut) {
						tracePath(t, getPathStr(component.Transceiver().ThresholdAny().Severity().State().PathStruct()), statusSkipped, "Skipping verification of transceiver threshold leaves due to deviation")
					} else {
						opts := getOptsForFunctionalTranslator(t, dut, deviations.CiscoxrLaserFt(dut))
						isPortUp := tc.expectedStatus == oc.Interface_OperStatus_UP
						validateThresholds(t, dut, transceiverName, isPortUp, oc.AlarmTypes_OPENCONFIG_ALARM_SEVERITY_CRITICAL, component, opts)
					}
				})
			}
		})
	}
}

func TestInterfacesWithTransceivers(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	ports := dut.Ports()
	for _, dp := range ports {
		t.Run(fmt.Sprintf("Interface:%s", dp.Name()), func(t *testing.T) {
			intfNameLookup := gnmi.Lookup(t, dut, gnmi.OC().Interface(dp.Name()).Name().State())
			intfName, ok := intfNameLookup.Val()
			intfNamePath := getPathStr(gnmi.OC().Interface(dp.Name()).Name().State().PathStruct())
			if !ok || intfName == "" {
				tracePath(t, intfNamePath, statusFail, "Failed to retrieve interface name state")
				t.Fatalf("Failed to retrieve interface name state for %q", dp.Name())
			}
			tracePath(t, intfNamePath, statusPass, "Interface name: %s", intfName)

			transceiverLookup := gnmi.Lookup(t, dut, gnmi.OC().Interface(dp.Name()).Transceiver().State())
			tv, tvOk := transceiverLookup.Val()
			transceiverPath := getPathStr(gnmi.OC().Interface(dp.Name()).Transceiver().State().PathStruct())
			if !tvOk || tv == "" {
				tracePath(t, transceiverPath, statusFail, "Failed to retrieve transceiver state")
				t.Fatalf("Failed to retrieve transceiver state for %q", dp.Name())
			}
			tracePath(t, transceiverPath, statusPass, "Interface transceiver: %s", tv)

			t.Run(fmt.Sprintf("Transceiver:%s", tv), func(t *testing.T) {
				mfgName, _ := gnmi.Lookup(t, dut, gnmi.OC().Component(tv).MfgName().State()).Val()
				tvPathStr := getPathStr(gnmi.OC().Component(tv).State().PathStruct())
				if mfgName == "" {
					tracePath(t, tvPathStr, statusSkipped, "Skipping check for Transceiver: got no MfgName.")
					t.Skipf("Skipping check for Transceiver: %q, got no MfgName.", tv)
				}

				formFactor, _ := gnmi.Lookup(t, dut, gnmi.OC().Component(tv).Transceiver().FormFactor().State()).Val()
				if formFactor == oc.TransportTypes_TRANSCEIVER_FORM_FACTOR_TYPE_UNSET {
					tracePath(t, tvPathStr, statusFail, "transceiver form-factor unset")
				} else {
					tracePath(t, tvPathStr, statusPass, "MfgName: %s, FormFactor: %v", mfgName, formFactor)
				}

				// Verify Component Type
				typePath := gnmi.OC().Component(tv).Type().State()
				compType, typePresent := gnmi.Lookup(t, dut, typePath).Val()
				if !typePresent {
					tracePath(t, getPathStr(typePath.PathStruct()), statusFail, "Component type is not present")
				} else {
					if compType == transceiverType {
						tracePath(t, getPathStr(typePath.PathStruct()), statusPass, "Component type: %v", compType)
					} else {
						tracePath(t, getPathStr(typePath.PathStruct()), statusFail, "Component type: got %v, want PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_TRANSCEIVER", compType)
					}
				}

				// Verify Part Number
				partNoPath := gnmi.OC().Component(tv).PartNo().State()
				partNo, partNoPresent := gnmi.Lookup(t, dut, partNoPath).Val()
				if !partNoPresent || partNo == "" {
					tracePath(t, getPathStr(partNoPath.PathStruct()), statusFail, "Part number is not present or empty")
				} else {
					tracePath(t, getPathStr(partNoPath.PathStruct()), statusPass, "Part number: %s", partNo)
				}

				// Verify Serial Number
				serialNoPath := gnmi.OC().Component(tv).SerialNo().State()
				serialNo, serialNoPresent := gnmi.Lookup(t, dut, serialNoPath).Val()
				if !serialNoPresent || serialNo == "" {
					tracePath(t, getPathStr(serialNoPath.PathStruct()), statusFail, "Serial number is not present or empty")
				} else {
					tracePath(t, getPathStr(serialNoPath.PathStruct()), statusPass, "Serial number: %s", serialNo)
				}

				// Verify Firmware Version
				firmwareVersionPath := gnmi.OC().Component(tv).FirmwareVersion().State()
				firmwareVersion, firmwareVersionPresent := gnmi.Lookup(t, dut, firmwareVersionPath).Val()
				if !firmwareVersionPresent || firmwareVersion == "" {
					tracePath(t, getPathStr(firmwareVersionPath.PathStruct()), statusWarning, "Firmware version is not present or empty")
				} else {
					tracePath(t, getPathStr(firmwareVersionPath.PathStruct()), statusPass, "Firmware version: %s", firmwareVersion)
				}

				// Verify Hardware Version
				hardwareVersionPath := gnmi.OC().Component(tv).HardwareVersion().State()
				hardwareVersion, hardwareVersionPresent := gnmi.Lookup(t, dut, hardwareVersionPath).Val()
				if !hardwareVersionPresent || hardwareVersion == "" {
					tracePath(t, getPathStr(hardwareVersionPath.PathStruct()), statusFail, "Hardware version is not present or empty")
				} else {
					tracePath(t, getPathStr(hardwareVersionPath.PathStruct()), statusPass, "Hardware version: %s", hardwareVersion)
				}

				// Verify Description
				descPath := gnmi.OC().Component(tv).Description().State()
				if deviations.SkipTransceiverDescription(dut) {
					tracePath(t, getPathStr(descPath.PathStruct()), statusSkipped, "Skipping verification of transceiver description due to deviation")
				} else {
					desc, descPresent := gnmi.Lookup(t, dut, descPath).Val()
					if !descPresent || desc == "" {
						tracePath(t, getPathStr(descPath.PathStruct()), statusWarning, "Description is not present or empty")
					} else {
						tracePath(t, getPathStr(descPath.PathStruct()), statusPass, "Description: %s", desc)
					}
				}

				// Verify Mfg Date
				mfgDatePath := gnmi.OC().Component(tv).MfgDate().State()
				if deviations.ComponentMfgDateUnsupported(dut) {
					tracePath(t, getPathStr(mfgDatePath.PathStruct()), statusSkipped, "Skipping verification of transceiver manufacturing date due to deviation")
				} else {
					mfgDate, mfgDatePresent := gnmi.Lookup(t, dut, mfgDatePath).Val()
					if !mfgDatePresent || mfgDate == "" {
						tracePath(t, getPathStr(mfgDatePath.PathStruct()), statusFail, "Manufacturing date is not present or empty")
					} else {
						tracePath(t, getPathStr(mfgDatePath.PathStruct()), statusPass, "Manufacturing date: %s", mfgDate)
					}
				}

				channelsMap := make(map[uint16]bool)
				channels := gnmi.LookupAll(t, dut, gnmi.OC().Component(tv).Transceiver().ChannelAny().State())
				for _, ch := range channels {
					if val, ok := ch.Val(); ok {
						channelsMap[val.GetIndex()] = true
					}
				}

				physicalChannelLookup := gnmi.Lookup(t, dut, gnmi.OC().Interface(dp.Name()).PhysicalChannel().State())
				physicalChannel, pcOk := physicalChannelLookup.Val()
				pcPath := getPathStr(gnmi.OC().Interface(dp.Name()).PhysicalChannel().State().PathStruct())
				if !pcOk || len(physicalChannel) == 0 {
					tracePath(t, pcPath, statusFail, "physical-channel unset for Interface: %q", intfName)
					t.Errorf("physical-channel unset for Interface: %q", intfName)
				} else {
					t.Logf("Interface %s physical-channel: %v", dp.Name(), physicalChannel)
					tracePath(t, pcPath, statusPass, "Physical channels: %v", physicalChannel)
					for _, p := range physicalChannel {
						if !channelsMap[p] {
							t.Errorf("Transceiver %s failed to get channel index %v", tv, p)
						}
					}
				}
			})
		})
	}
}
