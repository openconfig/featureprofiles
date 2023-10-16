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

var componentType = map[string]oc.E_PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT{
	"Chassis":     oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CHASSIS,
	"Fabric":      oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_FABRIC,
	"Linecard":    oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_LINECARD,
	"Fan":         oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_FAN,
	"PowerSupply": oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_POWER_SUPPLY,
	"Supervisor":  oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD,
	"SwitchChip":  oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_INTEGRATED_CIRCUIT,
	"Transceiver": oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_TRANSCEIVER,
	"TempSensor":  oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_SENSOR,
	"Cpu":         oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CPU,
	"Storage":     oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_STORAGE,
}

// use this map to cache related components used in subtests to run the test faster.
var componentsByType map[string][]*oc.Component

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
	operStatus            oc.E_PlatformTypes_COMPONENT_OPER_STATUS
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
				mfgNameValidation:     true,
				mfgDateValidation:     false,
				hwVerValidation:       true,
				fwVerValidation:       false,
				rrValidation:          false,
				operStatus:            oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE,
				parentValidation:      false,
				pType:                 componentType["Chassis"],
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
				operStatus:            oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE,
				parentValidation:      true,
				pType:                 componentType["Fabric"],
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
				operStatus:            oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE,
				parentValidation:      false,
				pType:                 componentType["Fan"],
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
				operStatus:            oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE,
				parentValidation:      true,
				pType:                 oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_LINECARD,
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
				operStatus:            oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE,
				parentValidation:      true,
				pType:                 componentType["PowerSupply"],
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
				operStatus:            oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE,
				parentValidation:      true,
				pType:                 componentType["Supervisor"],
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
				fwVerValidation:       true,
				rrValidation:          false,
				operStatus:            oc.PlatformTypes_COMPONENT_OPER_STATUS_UNSET,
				parentValidation:      false,
				pType:                 componentType["Transceiver"],
			},
		}, {
			desc: "Cpu",
			cardFields: properties{
				descriptionValidation: false,
				idValidation:          false,
				nameValidation:        true,
				partNoValidation:      true,
				serialNoValidation:    true,
				mfgNameValidation:     false,
				mfgDateValidation:     false,
				swVerValidation:       false,
				hwVerValidation:       false,
				fwVerValidation:       false,
				rrValidation:          false,
				operStatus:            oc.PlatformTypes_COMPONENT_OPER_STATUS_UNSET,
				parentValidation:      false,
				pType:                 componentType["Cpu"],
			},
		}, {
			desc: "Storage",
			cardFields: properties{
				descriptionValidation: false,
				idValidation:          false,
				nameValidation:        true,
				partNoValidation:      true,
				serialNoValidation:    true,
				mfgNameValidation:     false,
				mfgDateValidation:     false,
				swVerValidation:       false,
				hwVerValidation:       false,
				fwVerValidation:       false,
				rrValidation:          false,
				operStatus:            oc.PlatformTypes_COMPONENT_OPER_STATUS_UNSET,
				parentValidation:      false,
				pType:                 componentType["Storage"],
			},
		}}

	components := findComponentsListByType(t, dut)
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			if tc.desc == "Storage" && deviations.StorageComponentUnsupported(dut) {
				t.Skipf("Telemetry path /components/component/storage is not supported.")
			} else if tc.desc == "Fabric" && *args.NumLinecards <= 0 {
				t.Skip("Skip Fabric Telemetry check for fixed form factor devices.")
			} else if tc.desc == "Linecard" && *args.NumLinecards <= 0 {
				t.Skip("Skip Linecard Telemetry check for fixed form factor devices.")
			} else if tc.desc == "Supervisor" && *args.NumControllerCards <= 0 {
				t.Skip("Skip Supervisor Telemetry check for fixed form factor devices.")
			}
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
	if *args.NumControllerCards <= 0 {
		t.Skip("Skip SwitchChip Telemetry check for fixed form factor devices.")
	}
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
		operStatus:            oc.PlatformTypes_COMPONENT_OPER_STATUS_UNSET,
		parentValidation:      false,
		pType:                 componentType["SwitchChip"],
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
		t.Run(fmt.Sprintf("Backplane:%s", cName), func(t *testing.T) {
			if deviations.BackplaneFacingCapacityUnsupported(dut) {
				v := dut.Vendor()
				// Vendor does not support backplane-facing-capacity
				if v != ondatra.JUNIPER || (v == ondatra.JUNIPER && regexp.MustCompile("NPU[0-9]$").Match([]byte(card.GetName()))) {
					t.Skipf("Skipping check for BackplanceFacingCapacity due to deviation BackplaneFacingCapacityUnsupported")
				}
			}
			// For SwitchChip, check OC integrated-circuit paths.
			if card.GetIntegratedCircuit() == nil {
				t.Fatalf("SwitchChip %s integratedCircuit: got none, want > 0", cName)
			}
			if card.GetIntegratedCircuit().GetBackplaneFacingCapacity() == nil {
				t.Fatalf("SwitchChip %s integratedCircuit.backplaneFacingCapacity: got none, want > 0", cName)
			}
			// TotalOperationalCapacity
			if card.GetIntegratedCircuit().GetBackplaneFacingCapacity().TotalOperationalCapacity == nil {
				t.Errorf("SwitchChip %s totalOperationalCapacity: got none, want > 0", cName)
			}
			totalOperCapacity := card.GetIntegratedCircuit().GetBackplaneFacingCapacity().GetTotalOperationalCapacity()
			t.Logf("SwitchChip %s totalOperationalCapacity: %d", cName, totalOperCapacity)
			if totalOperCapacity <= 0 {
				t.Errorf("SwitchChip %s totalOperationalCapacity: got %v, want > 0", cName, totalOperCapacity)
			}

			// Total
			if card.GetIntegratedCircuit().GetBackplaneFacingCapacity().Total == nil {
				t.Errorf("SwitchChip %s totalCapacity: got none, want > 0", cName)
			}
			totalCapacity := card.GetIntegratedCircuit().GetBackplaneFacingCapacity().GetTotal()
			t.Logf("SwitchChip %s totalCapacity: %d", cName, totalCapacity)
			if totalCapacity == 0 {
				t.Errorf("SwitchChip %s totalCapacity: got %v, want > 0", cName, totalCapacity)
			}

			// AvailablePct
			if card.GetIntegratedCircuit().GetBackplaneFacingCapacity().AvailablePct == nil {
				t.Errorf("SwitchChip %s availablePct: got none, want >= 0", cName)
			}
			availablePct := card.GetIntegratedCircuit().GetBackplaneFacingCapacity().GetAvailablePct()
			t.Logf("SwitchChip %s availablePct: %d", cName, availablePct)

			// ConsumedCapacity
			if card.GetIntegratedCircuit().GetBackplaneFacingCapacity().ConsumedCapacity == nil {
				t.Errorf("SwitchChip %s consumedCapacity: got none, want >= 0", cName)
			}
			consumedCapacity := card.GetIntegratedCircuit().GetBackplaneFacingCapacity().GetConsumedCapacity()
			t.Logf("SwitchChip %s consumedCapacity: %d", cName, consumedCapacity)
		})
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
		t.Run(sName, func(t *testing.T) {

			t.Logf("TempSensor %s Id: %s", sName, sensor.GetId())

			if sensor.Type == nil {
				t.Fatalf("TempSensor %s: Type is empty", sName)
			}

			t.Logf("TempSensor %s Type: %s", sName, sensor.GetType())
			if got, want := sensor.GetType(), componentType["TempSensor"]; got != want {
				t.Errorf("Type for TempSensor %s: got %v, want %v", sName, got, want)
			}

			if sensor.GetTemperature() == nil {
				t.Fatalf("TempSensor %s: Temperature is nil", sName)
			}

			t.Logf("TempSensor %s Temperature instant: %v", sName, sensor.GetTemperature().GetInstant())
			if sensor.Temperature.Instant == nil {
				t.Errorf("TempSensor %s: Temperature instant is nil", sName)
			}

			t.Logf("TempSensor %s Temperature AlarmStatus: %v", sName, sensor.GetTemperature().GetAlarmStatus())
			if sensor.Temperature.AlarmStatus == nil {
				t.Errorf("TempSensor %s: Temperature AlarmStatus is nil", sName)
			}

			t.Logf("TempSensor %s Temperature Max: %v", sName, sensor.GetTemperature().GetMax())
			if sensor.Temperature.Max == nil {
				t.Errorf("TempSensor %s: Temperature Max is nil", sName)
			}

			t.Logf("TempSensor %s Temperature MaxTime: %v", sName, sensor.GetTemperature().GetMaxTime())
			if sensor.Temperature.MaxTime == nil {
				t.Errorf("TempSensor %s: Temperature MaxTime is nil", sName)
			}
		})
	}
}

func ValidateComponentState(t *testing.T, dut *ondatra.DUTDevice, cards []*oc.Component, p properties) {
	var validCards []*oc.Component
	switch p.pType {
	case componentType["Transceiver"]:
		// For transceiver, only check the transceiver with optics installed.
		for _, card := range cards {
			if card.GetMfgName() != "" {
				validCards = append(validCards, card)
			}
		}
	default:
		for _, lc := range cards {
			if !lc.GetEmpty() {
				validCards = append(validCards, lc)
			}
		}
	}
	for _, card := range validCards {
		if card.Name == nil {
			t.Errorf("Encountered a component with no Name")
			continue
		}
		cName := card.GetName()
		t.Run(cName, func(t *testing.T) {
			if p.descriptionValidation {
				t.Logf("Component %s Description: %s", cName, card.GetDescription())
				if card.GetDescription() == "" {
					t.Errorf("Component %s Description: got empty string, want non-empty string", cName)
				}
			}
			if card.GetType() == oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_LINECARD {
				t.Logf("Component %s linecard/state/slot-id: %s", cName, card.GetLinecard().GetSlotId())
				if card.GetLinecard().GetSlotId() == "" {
					t.Errorf("Component %s LineCard SlotID: got empty string, want non-empty string", cName)
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
				t.Logf("Component %s PartNo: %s", cName, card.GetPartNo())
				if card.GetPartNo() == "" {
					switch card.GetType() {
					case componentType["Fan"]:
						fallthrough
					case componentType["Storage"]:
						fallthrough
					case componentType["Cpu"]:
						if deviations.CPUMissingAncestor(dut) {
							break
						}
						parent := card.GetParent()
						for {
							cp, ok := gnmi.Lookup(t, dut, gnmi.OC().Component(parent).State()).Val()
							if !ok {
								t.Errorf("Couldn't find component: %s, (ancestor of: %s)", parent, cName)
								break
							}
							t.Logf("Component %s (ancestor of %s) PartNo: %s", parent, cName, cp.GetPartNo())

							// Found a Part No
							if cp.GetPartNo() != "" {
								break
							}
							// Found no parent
							if cp.GetParent() == "" || cp.GetParent() == parent {
								t.Errorf("Couldn't find parent of %s, (ancestor of %s)", parent, cName)
								break
							}
							parent = cp.GetParent()
						}
					default:
						t.Errorf("PartNo for Component %s: got empty string, want non-empty string", cName)
					}
				}
			}

			if p.serialNoValidation {
				t.Logf("Component %s SerialNo: %s", cName, card.GetSerialNo())
				if card.GetSerialNo() == "" {
					switch card.GetType() {
					case componentType["Fan"]:
						fallthrough
					case componentType["Storage"]:
						fallthrough
					case componentType["Cpu"]:
						if deviations.CPUMissingAncestor(dut) {
							break
						}
						parent := card.GetParent()
						for {
							cp, ok := gnmi.Lookup(t, dut, gnmi.OC().Component(parent).State()).Val()
							if !ok {
								t.Errorf("Couldn't find component: %s, (ancestor of: %s)", parent, cName)
								break
							}
							t.Logf("Component %s (ancestor of %s) SerialNo: %s", parent, cName, cp.GetSerialNo())

							// Found a Serial No
							if cp.GetSerialNo() != "" {
								break
							}
							// Found no parent
							if cp.GetParent() == "" || cp.GetParent() == parent {
								t.Errorf("Couldn't find parent of %s, (ancestor of %s)", parent, cName)
								break
							}
							parent = cp.GetParent()
						}
					default:
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
				t.Logf("Component %s MfgDate: %s", cName, mfgDate)
				if mfgDate == "" {
					t.Errorf("Component %s MfgDate: got empty string, want non-empty string", cName)
				}
			}

			if p.swVerValidation {
				// Only a subset of cards are expected to report Software Version.
				swVer := card.GetSoftwareVersion()
				t.Logf("Component %s SoftwareVersion: %s", cName, swVer)
				if swVer == "" {
					t.Errorf("Component %s SoftwareVersion: got empty string, want non-empty string", cName)
				}
			}

			if p.hwVerValidation {
				hwVer := card.GetHardwareVersion()
				t.Logf("Component %s HardwareVersion: %s", cName, hwVer)
				if hwVer == "" {
					t.Errorf("Component %s HardwareVersion: got empty string, want non-empty string", cName)
				}
			}

			if p.fwVerValidation {
				fwVer := card.GetFirmwareVersion()
				t.Logf("Component %s FirmwareVersion: %s", cName, fwVer)

				isTransceiver := card.GetType() == componentType["Transceiver"]
				is400G := false
				if isTransceiver {
					is400G = strings.Contains(card.GetTransceiver().GetEthernetPmd().String(), "ETH_400GBASE")
				}
				if fwVer == "" {
					if isTransceiver && !is400G {
						t.Logf("Skipping firmware-version check for %s transceiver", card.GetTransceiver().GetEthernetPmd().String())
					} else {
						t.Errorf("Component %s FirmwareVersion: got empty string, want non-empty string", cName)
					}
				}
			}

			if p.rrValidation {
				redundantRole := card.GetRedundantRole().String()
				t.Logf("Hardware card %s RedundantRole: %s", cName, redundantRole)
				if redundantRole != "PRIMARY" && redundantRole != "SECONDARY" {
					t.Errorf("Component %s RedundantRole: got %s, want %v", cName, redundantRole, p.operStatus)
				}
			}

			if p.operStatus != oc.PlatformTypes_COMPONENT_OPER_STATUS_UNSET {
				operStatus := card.GetOperStatus()
				t.Logf("Component %s OperStatus: %s", cName, operStatus.String())
				if operStatus != p.operStatus {
					t.Errorf("Component %s OperStatus: got %s, want %s", cName, operStatus, p.operStatus)
				}
			}

			if p.parentValidation {
				cur := cName
				for {
					if p.pType == componentType["Cpu"] && deviations.CPUMissingAncestor(dut) {
						break
					}
					val := gnmi.Lookup(t, dut, gnmi.OC().Component(cur).Parent().State())
					parent, ok := val.Val()
					if !ok {
						t.Errorf("Component %s Parent: Chassis component NOT found in the hierarchy tree of component", cName)
						break
					}
					parentType := gnmi.Get(t, dut, gnmi.OC().Component(parent).Type().State())
					if parentType == componentType["Chassis"] {
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
		})
	}
}

func TestStorage(t *testing.T) {
	// TODO: Add Storage test case here once supported.
	t.Skipf("Telemetry path /components/component/storage is not supported.")

	dut := ondatra.DUT(t, "dut")

	if deviations.StorageComponentUnsupported(dut) {
		t.Skipf("Telemetry path /components/component/storage is not supported.")
	}

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
