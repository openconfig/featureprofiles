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

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/functional-translators/registrar"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/gnmi/oc/platform"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	transceiverType         = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_TRANSCEIVER
	sensorType              = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_SENSOR
	minOpticsPower          = -40.0
	maxOpticsPower          = 10.0
	maxOpticsPowerAdminDown = -30.0
	intUpdateTime           = 2 * time.Minute
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

	sevPath := threshold.Severity().State()
	sevLookup := gnmi.Lookup(t, dut.GNMIOpts().WithYGNMIOpts(opts...), sevPath)
	if sevVal, ok := sevLookup.Val(); ok {
		tracePath(t, getPathStr(sevPath.PathStruct()), statusPass, "Severity: %v", sevVal)
	} else {
		tracePath(t, getPathStr(sevPath.PathStruct()), statusPass, "Severity: %v", sev)
	}

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

func validateOpticsTelemetry(t *testing.T, dut *ondatra.DUTDevice, dp *ondatra.Port, transceiverName string, isPortUp bool, checkMinOutPower bool, expectedMaxOutPower float64) {
	t.Helper()
	component := gnmi.OC().Component(transceiverName)
	channels := component.Transceiver().ChannelAny()
	opts := getOptsForFunctionalTranslator(t, dut, deviations.CiscoxrTransceiverFt(dut))

	// Validate Input Powers
	inputPowers := gnmi.LookupAll(t, dut.GNMIOpts().WithYGNMIOpts(opts...), channels.InputPower().Instant().State())
	inputPowerPathStr := getPathStr(channels.InputPower().Instant().State().PathStruct())
	if len(inputPowers) == 0 {
		tracePath(t, inputPowerPathStr, statusFail, "Get inputPowers list: got 0, want > 0")
	} else {
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
			if isPortUp {
				if inPower > maxOpticsPower || inPower < minOpticsPower {
					tracePath(t, pStr, statusFail, "value %.2f is outside range [%f, %f]", inPower, minOpticsPower, maxOpticsPower)
				} else {
					tracePath(t, pStr, statusPass, "value %.2f is within range [%f, %f]", inPower, minOpticsPower, maxOpticsPower)
				}
			} else {
				tracePath(t, pStr, statusPass, "input power when port down: %v", inPower)
			}
		}
	}

	// Validate Output Powers
	outputPowers := gnmi.LookupAll(t, dut.GNMIOpts().WithYGNMIOpts(opts...), channels.OutputPower().Instant().State())
	outputPowerPathStr := getPathStr(channels.OutputPower().Instant().State().PathStruct())
	if len(outputPowers) == 0 {
		tracePath(t, outputPowerPathStr, statusFail, "Get outputPowers list: got 0, want > 0")
	} else {
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
			if outPower > expectedMaxOutPower {
				tracePath(t, pStr, statusFail, "value %.2f is above maximum threshold <= %f", outPower, expectedMaxOutPower)
			} else if checkMinOutPower && outPower < minOpticsPower {
				tracePath(t, pStr, statusFail, "value %.2f is below minimum threshold >= %f", outPower, minOpticsPower)
			} else {
				tracePath(t, pStr, statusPass, "value %.2f is within expected range", outPower)
			}
		}
	}

	// Validate Laser Bias Currents
	biasCurrents := gnmi.LookupAll(t, dut.GNMIOpts().WithYGNMIOpts(opts...), channels.LaserBiasCurrent().Instant().State())
	biasPathStr := getPathStr(channels.LaserBiasCurrent().Instant().State().PathStruct())
	if len(biasCurrents) == 0 {
		tracePath(t, biasPathStr, statusFail, "Get biasCurrents list: got 0, want > 0")
	} else {
		for _, biasCurrent := range biasCurrents {
			pStr, err := ygot.PathToString(biasCurrent.Path)
			if err != nil {
				pStr = biasCurrent.Path.String()
			}
			if v, ok := biasCurrent.Val(); ok {
				tracePath(t, pStr, statusPass, "Laser bias current value: %v", v)
			}
		}
	}

	// Validate Supply Voltage
	svPath := component.Transceiver().SupplyVoltage().Instant().State()
	svPathStr := getPathStr(svPath.PathStruct())
	if sv, ok := gnmi.Lookup(t, dut.GNMIOpts().WithYGNMIOpts(opts...), svPath).Val(); ok {
		tracePath(t, svPathStr, statusPass, "Supply voltage value: %v", sv)
	} else {
		tracePath(t, svPathStr, statusFail, "Supply voltage instant is not defined")
	}

	// Validate Temperature (Subcomponents sensor or component temperature)
	compLookup := gnmi.Lookup(t, dut, component.State())
	comp, compPresent := compLookup.Val()
	sensorComponentChecked := false
	if compPresent && comp != nil {
		for _, subc := range comp.Subcomponent {
			if subc == nil || subc.GetName() == "" {
				continue
			}
			subcName := subc.GetName()
			scompLookup := gnmi.Lookup(t, dut, gnmi.OC().Component(subcName).State())
			if scomp, ok := scompLookup.Val(); ok && scomp != nil && scomp.GetType() == sensorType {
				desc := scomp.GetDescription()
				if !deviations.TemperatureSensorCheck(dut) || strings.Contains(desc, "Temperature Sensor") {
					sensorComponentChecked = true
					tempPathStr := getPathStr(gnmi.OC().Component(subcName).Temperature().Instant().State().PathStruct())
					if scomp.GetTemperature() == nil || scomp.GetTemperature().Instant == nil {
						tracePath(t, tempPathStr, statusFail, "Sensor %s: Temperature instant is not defined", subcName)
					} else {
						tracePath(t, tempPathStr, statusPass, "Temperature value: %v", scomp.GetTemperature().GetInstant())
					}
				}
			}
		}
	}
	if !sensorComponentChecked {
		tempPathStr := getPathStr(component.Temperature().Instant().State().PathStruct())
		if !compPresent || comp == nil || comp.GetTemperature() == nil || comp.GetTemperature().Instant == nil {
			tracePath(t, tempPathStr, statusFail, "Transceiver %s: Temperature instant is not defined", transceiverName)
		} else {
			tracePath(t, tempPathStr, statusPass, "Temperature value: %v", comp.GetTemperature().GetInstant())
		}
	}

	// Validate Thresholds
	if deviations.TransceiverThresholdsUnsupported(dut) {
		tracePath(t, getPathStr(component.Transceiver().ThresholdAny().Severity().State().PathStruct()), statusSkipped, "Skipping verification of transceiver threshold leaves due to deviation")
	} else {
		laserOpts := getOptsForFunctionalTranslator(t, dut, deviations.CiscoxrLaserFt(dut))
		for _, sev := range []oc.E_AlarmTypes_OPENCONFIG_ALARM_SEVERITY{
			oc.AlarmTypes_OPENCONFIG_ALARM_SEVERITY_WARNING,
			oc.AlarmTypes_OPENCONFIG_ALARM_SEVERITY_CRITICAL,
		} {
			validateThresholds(t, dut, transceiverName, isPortUp, sev, component, laserOpts)
		}
	}
}

func validateTransceiverInventory(t *testing.T, dut *ondatra.DUTDevice, dp *ondatra.Port, intf *oc.Interface, comp *oc.Component, transceiverName string) {
	t.Helper()
	intfNamePath := getPathStr(gnmi.OC().Interface(dp.Name()).Name().State().PathStruct())
	if intf == nil || intf.GetName() == "" {
		tracePath(t, intfNamePath, statusFail, "Failed to retrieve interface name state")
		t.Fatalf("Failed to retrieve interface name state for %q", dp.Name())
	}
	tracePath(t, intfNamePath, statusPass, "Interface name: %s", intf.GetName())

	transceiverPath := getPathStr(gnmi.OC().Interface(dp.Name()).Transceiver().State().PathStruct())
	tracePath(t, transceiverPath, statusPass, "Interface transceiver: %s", transceiverName)

	tvPathStr := getPathStr(gnmi.OC().Component(transceiverName).State().PathStruct())
	mfgPathStr := getPathStr(gnmi.OC().Component(transceiverName).MfgName().State().PathStruct())
	if comp == nil || comp.GetMfgName() == "" {
		tracePath(t, mfgPathStr, statusFail, "MfgName not defined")
		tracePath(t, tvPathStr, statusSkipped, "Skipping check for Transceiver: got no MfgName.")
		t.Skipf("Skipping check for Transceiver: %q, got no MfgName.", transceiverName)
	}
	mfgName := comp.GetMfgName()
	tracePath(t, mfgPathStr, statusPass, "MfgName: %s", mfgName)

	// Verify Form Factor
	ffPath := gnmi.OC().Component(transceiverName).Transceiver().FormFactor().State()
	formFactor := oc.TransportTypes_TRANSCEIVER_FORM_FACTOR_TYPE_UNSET
	if comp.GetTransceiver() != nil {
		formFactor = comp.GetTransceiver().GetFormFactor()
	}
	if formFactor == oc.TransportTypes_TRANSCEIVER_FORM_FACTOR_TYPE_UNSET {
		tracePath(t, getPathStr(ffPath.PathStruct()), statusFail, "transceiver form-factor unset")
	} else {
		tracePath(t, getPathStr(ffPath.PathStruct()), statusPass, "MfgName: %s, FormFactor: %v", mfgName, formFactor)
	}

	// Verify Component Type
	typePath := gnmi.OC().Component(transceiverName).Type().State()
	compType := comp.GetType()
	if compType == nil {
		tracePath(t, getPathStr(typePath.PathStruct()), statusFail, "Component type is not present")
	} else if compType == transceiverType {
		tracePath(t, getPathStr(typePath.PathStruct()), statusPass, "Component type: %v", compType)
	} else {
		tracePath(t, getPathStr(typePath.PathStruct()), statusFail, "Component type: got %v, want PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_TRANSCEIVER", compType)
	}

	// Verify Part Number
	partNoPath := gnmi.OC().Component(transceiverName).PartNo().State()
	partNo := comp.GetPartNo()
	if partNo == "" {
		tracePath(t, getPathStr(partNoPath.PathStruct()), statusFail, "Part number is not present or empty")
	} else {
		tracePath(t, getPathStr(partNoPath.PathStruct()), statusPass, "Part number: %s", partNo)
	}

	// Verify Serial Number
	serialNoPath := gnmi.OC().Component(transceiverName).SerialNo().State()
	serialNo := comp.GetSerialNo()
	if serialNo == "" {
		tracePath(t, getPathStr(serialNoPath.PathStruct()), statusFail, "Serial number is not present or empty")
	} else {
		tracePath(t, getPathStr(serialNoPath.PathStruct()), statusPass, "Serial number: %s", serialNo)
	}

	// Verify Firmware Version
	firmwareVersionPath := gnmi.OC().Component(transceiverName).FirmwareVersion().State()
	firmwareVersion := comp.GetFirmwareVersion()
	if firmwareVersion == "" {
		tracePath(t, getPathStr(firmwareVersionPath.PathStruct()), statusWarning, "Firmware version is not present or empty")
	} else {
		tracePath(t, getPathStr(firmwareVersionPath.PathStruct()), statusPass, "Firmware version: %s", firmwareVersion)
	}

	// Verify Hardware Version
	hardwareVersionPath := gnmi.OC().Component(transceiverName).HardwareVersion().State()
	hardwareVersion := comp.GetHardwareVersion()
	if hardwareVersion == "" {
		tracePath(t, getPathStr(hardwareVersionPath.PathStruct()), statusFail, "Hardware version is not present or empty")
	} else {
		tracePath(t, getPathStr(hardwareVersionPath.PathStruct()), statusPass, "Hardware version: %s", hardwareVersion)
	}

	// Verify Description
	descPath := gnmi.OC().Component(transceiverName).Description().State()
	if deviations.SkipTransceiverDescription(dut) {
		tracePath(t, getPathStr(descPath.PathStruct()), statusSkipped, "Skipping verification of transceiver description due to deviation")
	} else {
		desc := comp.GetDescription()
		if desc == "" {
			tracePath(t, getPathStr(descPath.PathStruct()), statusWarning, "Description is not present or empty")
		} else {
			tracePath(t, getPathStr(descPath.PathStruct()), statusPass, "Description: %s", desc)
		}
	}

	// Verify Mfg Date
	mfgDatePath := gnmi.OC().Component(transceiverName).MfgDate().State()
	if deviations.ComponentMfgDateUnsupported(dut) {
		tracePath(t, getPathStr(mfgDatePath.PathStruct()), statusSkipped, "Skipping verification of transceiver manufacturing date due to deviation")
	} else {
		mfgDate := comp.GetMfgDate()
		if mfgDate == "" {
			tracePath(t, getPathStr(mfgDatePath.PathStruct()), statusFail, "Manufacturing date is not present or empty")
		} else {
			tracePath(t, getPathStr(mfgDatePath.PathStruct()), statusPass, "Manufacturing date: %s", mfgDate)
		}
	}

	// Verify Channels and Physical Channels Mapping
	channelsMap := make(map[uint16]bool)
	if comp.GetTransceiver() != nil {
		for chIdx, ch := range comp.GetTransceiver().Channel {
			channelsMap[chIdx] = true
			chIndexPath := getPathStr(gnmi.OC().Component(transceiverName).Transceiver().Channel(chIdx).Index().State().PathStruct())
			if ch != nil && ch.Index != nil {
				tracePath(t, chIndexPath, statusPass, "Channel index: %d", ch.GetIndex())
			} else {
				tracePath(t, chIndexPath, statusPass, "Channel index: %d", chIdx)
			}
		}
	}

	physicalChannel := intf.GetPhysicalChannel()
	pcPath := getPathStr(gnmi.OC().Interface(dp.Name()).PhysicalChannel().State().PathStruct())
	if len(physicalChannel) == 0 {
		tracePath(t, pcPath, statusFail, "physical-channel unset for Interface: %q", dp.Name())
	} else {
		t.Logf("Interface %s physical-channel: %v", dp.Name(), physicalChannel)
		tracePath(t, pcPath, statusPass, "Physical channels: %v", physicalChannel)
		for _, p := range physicalChannel {
			if !channelsMap[p] {
				t.Errorf("Transceiver %s failed to get channel index %v", transceiverName, p)
			}
		}
	}
}

// TestClientOpticsTelemetryAndInventory validates that all interfaces and transceivers
// are enabled, links are UP, and all static inventory and dynamic telemetry parameters
// (power, bias current, temperature, supply voltage, and alarm thresholds) are valid.
func TestClientOpticsTelemetryAndInventory(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	ports := dut.Ports()
	for _, dp := range ports {
		t.Run(fmt.Sprintf("Port:%s", dp.Name()), func(t *testing.T) {
			// Ensure interface is enabled and wait for link to be UP
			gnmi.Update(t, dut, gnmi.OC().Interface(dp.Name()).Enabled().Config(), true)
			gnmi.Await(t, dut, gnmi.OC().Interface(dp.Name()).OperStatus().State(), intUpdateTime, oc.Interface_OperStatus_UP)

			intfLookup := gnmi.Lookup(t, dut, gnmi.OC().Interface(dp.Name()).State())
			intf, intfPresent := intfLookup.Val()
			if !intfPresent || intf == nil || intf.GetTransceiver() == "" {
				t.Fatalf("Failed to retrieve interface state or transceiver mapping for %q", dp.Name())
			}
			transceiverName := intf.GetTransceiver()

			// Check and record admin/oper status paths
			adminStatusPathStr := getPathStr(gnmi.OC().Interface(dp.Name()).AdminStatus().State().PathStruct())
			if intf.AdminStatus != oc.Interface_AdminStatus_UNSET {
				tracePath(t, adminStatusPathStr, statusPass, "Interface AdminStatus: %v", intf.GetAdminStatus())
			} else {
				tracePath(t, adminStatusPathStr, statusSkipped, "Interface AdminStatus path not present on device")
			}

			configEnabledLookup := gnmi.Lookup(t, dut, gnmi.OC().Interface(dp.Name()).Enabled().Config())
			configEnabled, configEnabledPresent := configEnabledLookup.Val()
			configEnabledPathStr := getPathStr(gnmi.OC().Interface(dp.Name()).Enabled().Config().PathStruct())
			if configEnabledPresent {
				tracePath(t, configEnabledPathStr, statusPass, "Interface Enabled Config: %v", configEnabled)
			} else {
				tracePath(t, configEnabledPathStr, statusSkipped, "Interface Enabled Config path not present on device")
			}

			operStatusPathStr := getPathStr(gnmi.OC().Interface(dp.Name()).OperStatus().State().PathStruct())
			tracePath(t, operStatusPathStr, statusPass, "Interface OperStatus: %v", intf.GetOperStatus())

			compLookup := gnmi.Lookup(t, dut, gnmi.OC().Component(transceiverName).State())
			comp, _ := compLookup.Val()

			// Validate Static Inventory Information
			validateTransceiverInventory(t, dut, dp, intf, comp, transceiverName)

			// Validate Dynamic Telemetry & Thresholds
			validateOpticsTelemetry(t, dut, dp, transceiverName, true, true, maxOpticsPower)
		})
	}
}

// TestInterfaceEnabled validates interface enable/disable lifecycle.
// When disabled, verifies oper-status is DOWN and output power drops.
// When re-enabled, verifies link returns to UP and telemetry parameters are within thresholds.
func TestInterfaceEnabled(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ports := dut.Ports()

	for _, dp := range ports {
		t.Run(fmt.Sprintf("Interface-Flap-%s", dp.Name()), func(t *testing.T) {
			originalEnabled, present := gnmi.Lookup(t, dut, gnmi.OC().Interface(dp.Name()).Enabled().Config()).Val()
			t.Cleanup(func() {
				if present {
					gnmi.Update(t, dut, gnmi.OC().Interface(dp.Name()).Enabled().Config(), originalEnabled)
				} else {
					gnmi.Delete(t, dut, gnmi.OC().Interface(dp.Name()).Enabled().Config())
				}
			})

			transceiverName := gnmi.Get(t, dut, gnmi.OC().Interface(dp.Name()).Transceiver().State())
			component := gnmi.OC().Component(transceiverName)
			mfgLookup := gnmi.Lookup(t, dut, component.MfgName().State())
			mfgName, mfgOk := mfgLookup.Val()
			if !mfgOk || mfgName == "" {
				tracePath(t, getPathStr(component.MfgName().State().PathStruct()), statusSkipped, "Skipping: component MfgName not present")
				t.Skipf("component MfgName for %q is not present. skip it", transceiverName)
			}

			t.Run("Disable interface and verify link DOWN, output power drops", func(t *testing.T) {
				gnmi.Update(t, dut, gnmi.OC().Interface(dp.Name()).Enabled().Config(), false)
				gnmi.Await(t, dut, gnmi.OC().Interface(dp.Name()).OperStatus().State(), intUpdateTime, oc.Interface_OperStatus_DOWN)

				operPathStr := getPathStr(gnmi.OC().Interface(dp.Name()).OperStatus().State().PathStruct())
				tracePath(t, operPathStr, statusPass, "Reached expected operational status: %v", oc.Interface_OperStatus_DOWN)

				// Validate telemetry when disabled: output power dropped, skip min output power check
				validateOpticsTelemetry(t, dut, dp, transceiverName, false, false, maxOpticsPowerAdminDown)
			})

			t.Run("Re-enable interface and verify link UP, telemetry parameters normal", func(t *testing.T) {
				gnmi.Update(t, dut, gnmi.OC().Interface(dp.Name()).Enabled().Config(), true)
				gnmi.Await(t, dut, gnmi.OC().Interface(dp.Name()).OperStatus().State(), intUpdateTime, oc.Interface_OperStatus_UP)

				operPathStr := getPathStr(gnmi.OC().Interface(dp.Name()).OperStatus().State().PathStruct())
				tracePath(t, operPathStr, statusPass, "Reached expected operational status: %v", oc.Interface_OperStatus_UP)

				// Validate telemetry when enabled: all paths valid and within thresholds
				validateOpticsTelemetry(t, dut, dp, transceiverName, true, true, maxOpticsPower)
			})
		})
	}
}

// TestTransceiverConfigEnabled validates transceiver enable/disable lifecycle.
// When disabled, verifies interface oper-status is not UP.
// When re-enabled, verifies link returns to UP and telemetry parameters are within thresholds.
func TestTransceiverConfigEnabled(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	if deviations.TransceiverConfigEnableUnsupported(dut) {
		tracePath(t, getPathStr(gnmi.OC().ComponentAny().Transceiver().Enabled().Config().PathStruct()), statusSkipped, "Skipping transceiver enabled config test due to deviation")
		t.Skip("Skipping TestTransceiverConfigEnabled: TransceiverConfigEnableUnsupported is true")
	}

	ports := dut.Ports()
	for _, dp := range ports {
		t.Run(fmt.Sprintf("Transceiver-Flap-%s", dp.Name()), func(t *testing.T) {
			transceiverName := gnmi.Get(t, dut, gnmi.OC().Interface(dp.Name()).Transceiver().State())
			component := gnmi.OC().Component(transceiverName)
			mfgLookup := gnmi.Lookup(t, dut, component.MfgName().State())
			mfgName, mfgOk := mfgLookup.Val()
			if !mfgOk || mfgName == "" {
				tracePath(t, getPathStr(component.MfgName().State().PathStruct()), statusSkipped, "Skipping: component MfgName not present")
				t.Skipf("component MfgName for %q is not present. skip it", transceiverName)
			}

			transceiver := component.Transceiver()
			cfgEnabledPath := transceiver.Enabled().Config()
			cfgPathStr := getPathStr(cfgEnabledPath.PathStruct())

			originalEnabled, present := gnmi.Lookup(t, dut, cfgEnabledPath).Val()
			t.Cleanup(func() {
				if present {
					gnmi.Update(t, dut, cfgEnabledPath, originalEnabled)
				} else {
					gnmi.Delete(t, dut, cfgEnabledPath)
				}
			})

			t.Run("Disable transceiver and verify link is not UP", func(t *testing.T) {
				gnmi.Update(t, dut, cfgEnabledPath, false)
				tracePath(t, cfgPathStr, statusPass, "Set transceiver enabled config: false")

				statePath := transceiver.Enabled().State()
				statePathStr := getPathStr(statePath.PathStruct())
				if val, ok := gnmi.Lookup(t, dut, statePath).Val(); ok {
					tracePath(t, statePathStr, statusPass, "Transceiver enabled state: %v", val)
				} else {
					tracePath(t, statePathStr, statusSkipped, "Transceiver enabled state not present on device")
				}

				operPath := gnmi.OC().Interface(dp.Name()).OperStatus().State()
				operPathStr := getPathStr(operPath.PathStruct())
				watch := gnmi.Watch(t, dut, operPath, intUpdateTime, func(val *ygnmi.Value[oc.E_Interface_OperStatus]) bool {
					status, ok := val.Val()
					return ok && status != oc.Interface_OperStatus_UP
				})
				if val, ok := watch.Await(t); !ok {
					tracePath(t, operPathStr, statusFail, "Interface oper-status remained UP after disabling transceiver")
					t.Fatalf("Interface %s oper-status remained UP after disabling transceiver: got %v", dp.Name(), val)
				}
				operStatus, _ := gnmi.Lookup(t, dut, operPath).Val()
				tracePath(t, operPathStr, statusPass, "Reached expected non-UP operational status: %v", operStatus)
			})

			t.Run("Re-enable transceiver and verify link is UP, telemetry parameters normal", func(t *testing.T) {
				gnmi.Update(t, dut, cfgEnabledPath, true)
				tracePath(t, cfgPathStr, statusPass, "Set transceiver enabled config: true")

				statePath := transceiver.Enabled().State()
				statePathStr := getPathStr(statePath.PathStruct())
				if val, ok := gnmi.Lookup(t, dut, statePath).Val(); ok {
					tracePath(t, statePathStr, statusPass, "Transceiver enabled state: %v", val)
				} else {
					tracePath(t, statePathStr, statusSkipped, "Transceiver enabled state not present on device")
				}

				gnmi.Await(t, dut, gnmi.OC().Interface(dp.Name()).OperStatus().State(), intUpdateTime, oc.Interface_OperStatus_UP)
				operPathStr := getPathStr(gnmi.OC().Interface(dp.Name()).OperStatus().State().PathStruct())
				tracePath(t, operPathStr, statusPass, "Reached expected operational status: %v", oc.Interface_OperStatus_UP)

				// Validate telemetry when enabled: all paths valid and within thresholds
				validateOpticsTelemetry(t, dut, dp, transceiverName, true, true, maxOpticsPower)
			})
		})
	}
}
