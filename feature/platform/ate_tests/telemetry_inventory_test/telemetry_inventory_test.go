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

package telemetry_inventory_test

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/openconfig/featureprofiles/internal/args"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
)

var activeStatus string = "ACTIVE"

var componentType = map[string]string{
	"chassis":     "CHASSIS",
	"fabric":      "FABRIC",
	"linecard":    "LINECARD",
	"fan":         "FAN",
	"powersupply": "POWER_SUPPLY",
	"supervisor":  "CONTROLLER_CARD",
	"switchchip":  "INTEGRATED_CIRCUIT",
	"transceiver": "TRANSCEIVER",
	"tempsensor":  "SENSOR",
}

// Define a superset of the checklist for each component
type properties struct {
	descriptionValidation bool
	idValidation          bool
	nameValidation        bool
	partNoValidation      bool
	serialNoValidation    bool
	mfgNameValidation     bool
	mfgDateValidation     bool
	swVerValidation       bool
	hwVerValidation       bool
	fwVerValidation       bool
	rrValidation          bool
	operStatus            string
	parentValidation      bool
	pType                 oc.Component_Type_Union
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Test cases:
//   - Validate Telemetry for each FRU within chassis.
//   - For each of the following component types, validate
//     1) Presence of component within gNMI telemetry.
//     2) Presence of component properties such as description, part-no, serial-no and oper-status etc.
//   - Validate the following components:
//   - Chassis
//   - Line card
//   - Power Supply
//   - Fabric card
//   - FabricChip
//   - Fan
//   - Supervisor or Controller
//   - Validate telemetry components/component/state/software-version.
//   - SwitchChip
//   - Validate the presence of the following OC paths under SwitchChip component:
//   - integrated-circuit/backplane-facing-capacity/state/available-pct
//   - integrated-circuit/backplane-facing-capacity/state/consumed-capacity
//   - integrated-circuit/backplane-facing-capacity/state/total
//   - integrated-circuit/backplane-facing-capacity/state/total-operational-capacity
//   - Transceiver
//   - Storage
//   - Validate telemetry /components/component/storage exists.
//   - TempSensor
//   - Validate telemetry /components/component/state/temperature/instant exists.
//
// Topology:
//
//	dut:port1 <--> ate:port1
//
// Test notes:
//
//   - Test cases for Software Module and Storage are skipped due to the blocking bugs:
//
//   - Need to support telemetry path /components/component/software-module.
//
//   - Need to support telemetry path /components/component/storage.
//
//     Sample CLI command to get component inventory using gmic:
//
//   - gnmic -a ipaddr:10162 -u username -p password --skip-verify get \
//     --path /components/component --format flat
//
//   - gnmic tool info:
//
//   - https://github.com/karimra/gnmic/blob/main/README.md
const (
	chassisType     = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CHASSIS
	supervisorType  = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD
	linecardType    = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_LINECARD
	powerSupplyType = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_POWER_SUPPLY
	fabricType      = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_FABRIC
	switchChipType  = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_INTEGRATED_CIRCUIT
	cpuType         = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CPU
	fanType         = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_FAN
	transceiverType = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_TRANSCEIVER
	sensorType      = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_SENSOR
)

// use this map to cache related components used in subtests to run the test faster.
var componentsByType map[string][]*oc.Component

func TestHardwareCards(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	cases := []struct {
		desc          string
		regexpPattern string
		cardFields    properties
	}{
		{
			desc: "Chassis",
			cardFields: properties{
				descriptionValidation: true,
				idValidation:          false,
				nameValidation:        true,
				partNoValidation:      true,
				serialNoValidation:    true,
				mfgNameValidation:     false,
				mfgDateValidation:     false,
				hwVerValidation:       true,
				fwVerValidation:       false,
				rrValidation:          false,
				operStatus:            activeStatus,
				parentValidation:      false,
				pType:                 chassisType,
			},
		}, {
			desc: "Fabric",
			cardFields: properties{
				descriptionValidation: true,
				idValidation:          true,
				nameValidation:        true,
				partNoValidation:      true,
				serialNoValidation:    true,
				mfgNameValidation:     true,
				mfgDateValidation:     false,
				hwVerValidation:       true,
				fwVerValidation:       false,
				rrValidation:          false,
				operStatus:            activeStatus,
				parentValidation:      true,
				pType:                 fabricType,
			},
		}, {
			desc: "Fan",
			cardFields: properties{
				descriptionValidation: true,
				idValidation:          false,
				nameValidation:        true,
				partNoValidation:      true,
				serialNoValidation:    true,
				mfgNameValidation:     false,
				mfgDateValidation:     false,
				hwVerValidation:       false,
				fwVerValidation:       false,
				rrValidation:          false,
				operStatus:            activeStatus,
				parentValidation:      false,
				pType:                 fanType,
			},
		}, {
			desc: "Linecard",
			cardFields: properties{
				descriptionValidation: true,
				idValidation:          true,
				nameValidation:        true,
				partNoValidation:      true,
				serialNoValidation:    true,
				mfgNameValidation:     true,
				mfgDateValidation:     false,
				hwVerValidation:       true,
				fwVerValidation:       false,
				rrValidation:          false,
				operStatus:            activeStatus,
				parentValidation:      true,
				pType:                 linecardType,
			},
		}, {
			desc: "PowerSupply",
			cardFields: properties{
				descriptionValidation: true,
				idValidation:          true,
				nameValidation:        true,
				partNoValidation:      true,
				serialNoValidation:    true,
				mfgNameValidation:     true,
				mfgDateValidation:     false,
				hwVerValidation:       true,
				fwVerValidation:       false,
				rrValidation:          false,
				operStatus:            activeStatus,
				parentValidation:      true,
				pType:                 powerSupplyType,
			},
		}, {
			desc: "Supervisor",
			cardFields: properties{
				descriptionValidation: true,
				idValidation:          true,
				nameValidation:        true,
				partNoValidation:      true,
				serialNoValidation:    true,
				mfgNameValidation:     true,
				mfgDateValidation:     false,
				swVerValidation:       false,
				hwVerValidation:       true,
				fwVerValidation:       false,
				rrValidation:          true,
				operStatus:            activeStatus,
				parentValidation:      true,
				pType:                 supervisorType,
			},
		}, {
			desc: "Transceiver",
			cardFields: properties{
				descriptionValidation: false,
				idValidation:          false,
				nameValidation:        true,
				partNoValidation:      true,
				serialNoValidation:    true,
				mfgNameValidation:     true,
				mfgDateValidation:     false,
				swVerValidation:       false,
				hwVerValidation:       true,
				fwVerValidation:       false,
				rrValidation:          false,
				operStatus:            "",
				parentValidation:      false,
				pType:                 transceiverType,
			},
		}}

	components := findComponentsListByType(t, dut)
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			cards := components[tc.desc]
			t.Logf("%s components count: %d", tc.desc, len(cards))
			if len(cards) == 0 {
				t.Fatalf("Components list for %s on %v: got 0, want > 0", tc.desc, dut.Model())
			}
			ValidateComponentState(t, dut, cards, tc.cardFields)
		})
	}
}

func findComponentsListByType(t *testing.T, dut *ondatra.DUTDevice) map[string][]*oc.Component {
	t.Helper()
	componentType := map[string]oc.E_PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT{
		"Chassis":     chassisType,
		"Fabric":      fabricType,
		"Linecard":    linecardType,
		"PowerSupply": powerSupplyType,
		"Supervisor":  supervisorType,
		"SwitchChip":  switchChipType,
		"Transceiver": transceiverType,
		"Fan":         fanType,
		"TempSensor":  sensorType,
	}
	if len(componentsByType) != 0 {
		return componentsByType
	}
	componentsByType = make(map[string][]*oc.Component)
	components := gnmi.GetAll(t, dut, gnmi.OC().ComponentAny().State())
	for compName := range componentType {
		for _, c := range components {
			if c.GetType() == nil || c.GetType() != componentType[compName] {
				continue
			}
			switch compName {
			case "SwitchChip":
				if *args.SwitchChipNamePattern != "" &&
					!isCompNameExpected(t, c.GetName(), *args.SwitchChipNamePattern) {
					continue
				}
			case "TempSensor":
				if *args.TempSensorNamePattern != "" &&
					!isCompNameExpected(t, c.GetName(), *args.TempSensorNamePattern) {
					continue
				}
			case "Fan":
				if *args.FanNamePattern != "" &&
					!isCompNameExpected(t, c.GetName(), *args.FanNamePattern) {
					continue
				}

			}
			componentsByType[compName] = append(componentsByType[compName], c)
		}
	}
	return componentsByType
}

func isCompNameExpected(t *testing.T, name, regexpPattern string) bool {
	t.Helper()
	r, err := regexp.Compile(regexpPattern)
	if err != nil {
		t.Fatalf("Cannot compile regular expression: %v", err)
	}
	return r.MatchString(name)
}

func TestSwitchChip(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	cardFields := properties{
		descriptionValidation: false,
		idValidation:          true,
		nameValidation:        true,
		partNoValidation:      false,
		serialNoValidation:    false,
		mfgNameValidation:     false,
		mfgDateValidation:     false,
		swVerValidation:       false,
		hwVerValidation:       false,
		fwVerValidation:       false,
		operStatus:            "",
		parentValidation:      false,
		pType:                 switchChipType,
	}

	components := findComponentsListByType(t, dut)
	cards := components["SwitchChip"]
	if len(cards) == 0 {
		t.Fatalf("Get SwitchChip card list for %q: got 0, want > 0", dut.Model())
	} else {
		t.Logf("SwitchChip components count: %d", len(cards))
	}

	ValidateComponentState(t, dut, cards, cardFields)

	for _, card := range cards {

		if card.Name == nil {
			t.Errorf("Encountered SwitchChip component with no Name")
			continue
		}

		cName := card.GetName()
		t.Logf("Validate SwitchChip %s", cName)

		if deviations.BackplaneFacingCapacityUnsupported(dut) && regexp.MustCompile("NPU[0-9]$").Match([]byte(card.GetName())) {
			// Vendor does not support backplane-facing-capacity for nodes named 'NPU'.
			t.Logf("Skipping check for BackplanceFacingCapacity due to deviation BackplaneFacingCapacityUnsupported")
			continue
		} else {
			// For SwitchChip, check OC integrated-circuit paths.

			// TotalOperationalCapacity
			if card.IntegratedCircuit.BackplaneFacingCapacity.TotalOperationalCapacity == nil {
				t.Errorf("SwitchChip %s totalOperationalCapacity: got none, want > 0", cName)
			}
			totalOperCapacity := card.GetIntegratedCircuit().GetBackplaneFacingCapacity().GetTotalOperationalCapacity()
			t.Logf("SwitchChip %s totalOperationalCapacity: %d", cName, totalOperCapacity)
			if totalOperCapacity <= 0 {
				t.Errorf("SwitchChip %s totalOperationalCapacity: got %v, want > 0", cName, totalOperCapacity)
			}

			// Total
			if card.IntegratedCircuit.BackplaneFacingCapacity.Total == nil {
				t.Errorf("SwitchChip %s totalCapacity: got none, want > 0", cName)
			}
			totalCapacity := card.GetIntegratedCircuit().GetBackplaneFacingCapacity().GetTotal()
			t.Logf("SwitchChip %s totalCapacity: %d", cName, totalCapacity)
			if totalCapacity == 0 {
				t.Errorf("SwitchChip %s totalCapacity: got %v, want > 0", cName, totalCapacity)
			}

			// AvailablePct
			if card.IntegratedCircuit.BackplaneFacingCapacity.AvailablePct == nil {
				t.Errorf("SwitchChip %s availablePct: got none, want >= 0", cName)
			}
			availablePct := card.GetIntegratedCircuit().GetBackplaneFacingCapacity().GetAvailablePct()
			t.Logf("SwitchChip %s availablePct: %d", cName, availablePct)

			// ConsumedCapacity
			if card.IntegratedCircuit.BackplaneFacingCapacity.ConsumedCapacity == nil {
				t.Errorf("SwitchChip %s consumedCapacity: got none, want >= 0", cName)
			}
			consumedCapacity := card.GetIntegratedCircuit().GetBackplaneFacingCapacity().GetConsumedCapacity()
			t.Logf("SwitchChip %s consumedCapacity: %d", cName, consumedCapacity)
		}
	}
}

func TestTempSensor(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	sensors := findComponentsListByType(t, dut)["TempSensor"]
	if len(sensors) == 0 {
		t.Fatalf("Get TempSensor list for %q: got 0, want > 0", dut.Model())
	} else {
		t.Logf("TempSensor components count: %d", len(sensors))
	}

	for _, sensor := range sensors {

		if sensor.Name == nil {
			t.Errorf("Encountered a sensor with no Name")
			continue
		}

		sName := sensor.GetName()
		t.Logf("Validate sensor %s", sName)

		if sensor.GetId() == "" {
			// Just log the result using Logf instead of Errorf.
			t.Logf("TempSensor %s: Id is empty", sName)
		} else {
			t.Logf("TempSensor %s Id: %s", sName, sensor.GetId())
		}

		if sensor.Type == nil {
			t.Errorf("TempSensor %s: Type is empty", sName)
			want := componentType["tempsensor"]
			got := fmt.Sprintf("%v", sensor.GetType())
			if want != got {
				t.Errorf("Type for TempSensor %s: got %v, want %v", sName, got, want)
			}
		} else {
			t.Logf("TempSensor %s Type: %s", sName, sensor.GetType())
		}

		if sensor.Temperature.Instant == nil {
			t.Errorf("TempSensor %s: Temperature instant is nil", sName)
		} else {
			t.Logf("TempSensor %s Temperature instant: %v", sName, sensor.GetTemperature().GetInstant())
		}

		if sensor.Temperature.AlarmStatus == nil {
			t.Errorf("TempSensor %s: Temperature AlarmStatus is nil", sName)
		} else {
			t.Logf("TempSensor %s Temperature AlarmStatus: %v", sName, sensor.GetTemperature().GetAlarmStatus())
		}

		if sensor.Temperature.Max == nil {
			t.Errorf("TempSensor %s: Temperature Max is nil", sName)
		} else {
			t.Logf("TempSensor %s Temperature Max: %v", sName, sensor.GetTemperature().GetMax())
		}

		if sensor.Temperature.MaxTime == nil {
			t.Errorf("TempSensor %s: Temperature MaxTime is nil", sName)
		} else {
			t.Logf("TempSensor %s Temperature MaxTime: %v", sName, sensor.GetTemperature().GetMaxTime())
		}

	}
}

func ValidateComponentState(t *testing.T, dut *ondatra.DUTDevice, cards []*oc.Component, p properties) {
	t.Helper()
	for _, card := range cards {
		if card.Name == nil {
			t.Errorf("Encountered a component with no Name")
			continue
		}

		cName := card.GetName()
		t.Logf("Validate component %s", cName)

		// For transceiver, only check the transceiver with optics installed.
		if strings.Contains(cName, "transceiver") {
			if card.GetMfgName() != "" {
				t.Logf("Optics is detected in %s with expected parent: %s", cName, card.GetParent())
			} else {
				t.Logf("Optics is not installed in %s, skip testing this transceiver", cName)
				continue
			}
		}

		if p.descriptionValidation {
			description := card.GetDescription()
			t.Logf("Component %s Description: %s", cName, description)
			if description == "" {
				t.Errorf("Component %s Description: got empty string, want non-empty string", cName)
			}
		}

		if p.idValidation {
			if deviations.SwitchChipIDUnsupported(dut) {
				t.Logf("Skipping check for Id due to deviation SwitChipIDUnsupported")
			} else {
				id := card.GetId()
				t.Logf("Component %s Id: %s", cName, id)
				if id == "" {
					t.Errorf("Component %s Id: got empty string, want non-empty string", cName)
				}
			}
		}

		if p.nameValidation {
			name := card.GetName()
			t.Logf("Component %s Name: %s", cName, name)
			if name == "" {
				t.Errorf("Encountered empty Name for component %s", cName)
			}
		}

		if p.partNoValidation {
			partNo := card.PartNo
			t.Logf("Component %s PartNo: %v", cName, partNo)
			if partNo == nil {
				if card.GetType() == fanType {
					fanTray := card.GetParent()
					fanTrayPartNo := gnmi.Lookup(t, dut, gnmi.OC().Component(fanTray).PartNo().State())
					fanTrayPartNoVal, ftpnPresent := fanTrayPartNo.Val()
					if !ftpnPresent {
						t.Errorf("PartNo for Component %s and its parent: got empty string, want non-empty string", cName)
					}
					t.Logf("Component %s (parent of %s) PartNo: %v", fanTray, cName, fanTrayPartNoVal)
				} else {
					t.Errorf("PartNo for Component %s: got empty string, want non-empty string", cName)
				}
			}
		}

		if p.serialNoValidation {
			serialNo := card.SerialNo
			t.Logf("Component %s SerialNo: %v", cName, serialNo)
			if serialNo == nil {
				if card.GetType() == fanType {
					fanTray := card.GetParent()
					fanTraySerialNo := gnmi.Lookup(t, dut, gnmi.OC().Component(fanTray).SerialNo().State())
					fanTraySerialNoVal, ftsnPresent := fanTraySerialNo.Val()
					if !ftsnPresent {
						t.Errorf("SerialNo for Component %s and its parent: got empty string, want non-empty string", cName)
					}
					t.Logf("Component %s (parent of %s) SerialNo: %v", fanTray, cName, fanTraySerialNoVal)
				} else {
					t.Errorf("SerialNo for Component %s: got empty string, want non-empty string", cName)
				}
			}
		}

		if p.mfgNameValidation {
			mfgName := card.GetMfgName()
			t.Logf("Component %s MfgName: %s", cName, mfgName)
			if mfgName == "" {
				t.Errorf("Component %s MfgName: got empty string, want non-empty string", cName)
			}
		}

		if p.mfgDateValidation {
			mfgDate := card.GetMfgDate()
			if mfgDate == "" {
				t.Errorf("Component %s MfgDate: got empty string, want non-empty string", cName)
			}
			t.Logf("Component %s MfgDate: %s", cName, mfgDate)
		}

		if p.swVerValidation {
			// Only a subset of cards are expected to report Software Version.
			swVer := card.GetSoftwareVersion()
			if swVer == "" {
				t.Errorf("Component %s SoftwareVersion: got empty string, want non-empty string", cName)
			}
			t.Logf("Component %s SoftwareVersion: %s", cName, swVer)
		}

		if p.hwVerValidation {
			hwVer := card.GetHardwareVersion()
			if hwVer == "" {
				t.Errorf("Component %s HardwareVersion: got empty string, want non-empty string", cName)
			}
			t.Logf("Component %s HardwareVersion: %s", cName, hwVer)
		}

		if p.fwVerValidation {
			fwVer := card.GetFirmwareVersion()
			if fwVer == "" {
				t.Errorf("Component %s FirmwareVersion: got empty string, want non-empty string", cName)
			}
			t.Logf("Component %s FirmwareVersion: %s", cName, fwVer)
		}

		if p.rrValidation {
			redundantRole := card.GetRedundantRole().String()
			t.Logf("Hardware card %s RedundantRole: %s", cName, redundantRole)
			if redundantRole != "PRIMARY" && redundantRole != "SECONDARY" {
				t.Errorf("Component %s RedundantRole: got %s, want %v", cName, redundantRole, p.operStatus)
			}
		}

		if p.operStatus != "" {
			if deviations.FanOperStatusUnsupported(dut) && strings.Contains(cName, "Fan") {
				t.Logf("Skipping check for fan oper-status due to deviation FanOperStatusUnsupported")
			} else {
				operStatus := card.GetOperStatus().String()
				t.Logf("Component %s OperStatus: %s", cName, operStatus)
				if operStatus != p.operStatus {
					t.Errorf("Component %s OperStatus: got %s, want %s", cName, operStatus, p.operStatus)
				}
			}
		}

		if p.parentValidation {
			cur := cName
			for {
				val := gnmi.Lookup(t, dut, gnmi.OC().Component(cur).Parent().State())
				parent, present := val.Val()
				if !present {
					t.Errorf("Component %s Parent: Chassis component NOT found in the hierarchy tree of component", cName)
					break
				}
				parentType := gnmi.Get(t, dut, gnmi.OC().Component(parent).Type().State())
				if parentType == chassisType {
					t.Logf("Component %s Parent: Found chassis component in the hierarchy tree of component", cName)
					break
				}
				if parent == cur {
					t.Errorf("Component %s Parent: Chassis component NOT found in the hierarchy tree of component", cName)
					break
				}
				cur = parent
			}
		}

		if p.pType != nil {
			ptype := card.GetType()
			t.Logf("Component %s Type: %v", cName, ptype)
			if ptype != p.pType {
				t.Errorf("Component %s Type: got %v, want %v", cName, ptype, p.pType)
			}
		}
	}
}

func TestSoftwareModule(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	if deviations.ComponentsSoftwareModuleUnsupported(dut) {
		t.Logf("Skipping check for components software module unsupport")
	} else {
		moduleTypes := gnmi.LookupAll(t, dut, gnmi.OC().ComponentAny().SoftwareModule().ModuleType().State())
		if len(moduleTypes) == 0 {
			t.Errorf("Get moduleType list for %q: got 0, want > 0", dut.Model())
		}

		for i, moduleType := range moduleTypes {
			modVal, present := moduleType.Val()
			if !present {
				t.Fatalf("moduleType.IsPresent() item %d: got false, want true", i)
			}
			t.Logf("Telemetry moduleType path/value %d: %v=>%v.", i, moduleType.Path.String(), modVal)
		}
	}
}

func TestStorage(t *testing.T) {
	// TODO: Add Storage test case here once supported.
	t.Skipf("Telemetry path /components/component/storage is not supported.")

	dut := ondatra.DUT(t, "dut")
	storages := gnmi.LookupAll(t, dut, gnmi.OC().ComponentAny().Storage().State())
	if len(storages) == 0 {
		t.Errorf("Get Storage list for %q: got 0, want > 0", dut.Model())
	}

	for i, storage := range storages {
		storVal, present := storage.Val()
		if !present {
			t.Fatalf("storage.IsPresent() item %d: got false, want true", i)
		}
		t.Logf("Telemetry storage path/value %d: %v=>%v.", i, storage.Path.String(), storVal)
	}
}

func TestLinecardConfig(t *testing.T) {
	// TODO: Add linecard config test case here once supported.
	t.Skipf("/components/component/linecard/config is not supported.")
}

func TestHeatsinkTempSensor(t *testing.T) {
	// TODO: Add heatsink-temperature-sensor test case here once supported.
	t.Skipf("/components/component[name=<heatsink-temperature-sensor>]/state/temperature/instant is not supported.")
}
