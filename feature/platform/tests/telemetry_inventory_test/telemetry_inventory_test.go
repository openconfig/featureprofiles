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
	"github.com/openconfig/featureprofiles/internal/components"
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
	"Fan Tray":    oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_FAN_TRAY,
	"PowerSupply": oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_POWER_SUPPLY,
	"Supervisor":  oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD,
	"SwitchChip":  oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_INTEGRATED_CIRCUIT,
	"Transceiver": oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_TRANSCEIVER,
	"TempSensor":  oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_SENSOR,
	"Cpu":         oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CPU,
	"Storage":     oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_STORAGE,
}

// validInstallComponentTypes indicates for each component type, which types of
// install-component it can have (i.e., what types of components can it be installed into).
var validInstallComponentTypes = map[oc.Component_Type_Union]map[oc.Component_Type_Union]bool{
	oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD: {
		oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CHASSIS: true,
	},
	oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_FABRIC: {
		oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CHASSIS: true,
	},
	oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_FAN: {
		oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CHASSIS:  true,
		oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_FAN_TRAY: true,
	},
	oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_FAN_TRAY: {
		oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CHASSIS: true,
	},
	oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_LINECARD: {
		oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CHASSIS: true,
	},
	oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_POWER_SUPPLY: {
		oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CHASSIS: true,
		// Sometimes the parent is the power tray, which has type FRU.
		oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_FRU: true,
	},
	oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_TRANSCEIVER: {
		oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CHASSIS:  true,
		oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_LINECARD: true,
	},
}

// use this map to cache related components used in subtests to run the test faster.
var componentsByType map[string][]*oc.Component

// Define a superset of the checklist for each component
type properties struct {
	descriptionValidation                 bool
	idValidation                          bool
	installPositionAndComponentValidation bool
	nameValidation                        bool
	partNoValidation                      bool
	serialNoValidation                    bool
	mfgNameValidation                     bool
	mfgDateValidation                     bool
	// If modelNameValidation is being used, the /components/component/state/model-name
	// of the chassis component must be equal to the ondatra hardware_model name
	// of its device.
	modelNameValidation bool
	swVerValidation     bool
	hwVerValidation     bool
	fwVerValidation     bool
	rrValidation        bool
	operStatus          oc.E_PlatformTypes_COMPONENT_OPER_STATUS
	parentValidation    bool
	pType               oc.Component_Type_Union
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
//   - Fan Tray
//   - Supervisor or Controller
//   - Validate telemetry components/component/state/software-version.
//   - SwitchChip
//   - Validate the presence of the following OC paths under SwitchChip component:
//   - integrated-circuit/backplane-facing-capacity/state/available-pct
//   - integrated-circuit/backplane-facing-capacity/state/consumed-capacity
//   - integrated-circuit/backplane-facing-capacity/state/total
//   - integrated-circuit/backplane-facing-capacity/state/total-operational-capacity
//   - components/component/subcomponents/subcomponent/name
//   - components/component/subcomponents/subcomponent/state/name
//   - Transceiver
//   - Storage
//   - Validate telemetry /components/component/storage exists.
//   - TempSensor
//   - Validate telemetry /components/component/state/temperature/instant exists.
//	 - Validate telemetry /components/component/state/model-name for Chassis.
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
				modelNameValidation:   true,
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
				descriptionValidation:                 true,
				idValidation:                          true,
				installPositionAndComponentValidation: true,
				nameValidation:                        true,
				partNoValidation:                      true,
				serialNoValidation:                    true,
				mfgNameValidation:                     true,
				mfgDateValidation:                     false,
				hwVerValidation:                       true,
				fwVerValidation:                       false,
				rrValidation:                          false,
				operStatus:                            oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE,
				parentValidation:                      true,
				pType:                                 componentType["Fabric"],
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
				parentValidation:      true,
				pType:                 componentType["Fan"],
			},
		}, {
			desc: "Fan Tray",
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
				parentValidation:      true,
				pType:                 componentType["Fan Tray"],
			},
		}, {
			desc: "Linecard",
			cardFields: properties{
				descriptionValidation:                 true,
				idValidation:                          true,
				installPositionAndComponentValidation: true,
				nameValidation:                        true,
				partNoValidation:                      true,
				serialNoValidation:                    true,
				mfgNameValidation:                     true,
				mfgDateValidation:                     false,
				hwVerValidation:                       true,
				fwVerValidation:                       false,
				rrValidation:                          false,
				operStatus:                            oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE,
				parentValidation:                      true,
				pType:                                 oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_LINECARD,
			},
		}, {
			desc: "PowerSupply",
			cardFields: properties{
				descriptionValidation:                 true,
				idValidation:                          true,
				installPositionAndComponentValidation: true,
				nameValidation:                        true,
				partNoValidation:                      true,
				serialNoValidation:                    true,
				mfgNameValidation:                     true,
				mfgDateValidation:                     false,
				hwVerValidation:                       true,
				fwVerValidation:                       false,
				rrValidation:                          false,
				operStatus:                            oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE,
				parentValidation:                      true,
				pType:                                 componentType["PowerSupply"],
			},
		}, {
			desc: "Supervisor",
			cardFields: properties{
				descriptionValidation:                 true,
				idValidation:                          true,
				installPositionAndComponentValidation: true,
				nameValidation:                        true,
				partNoValidation:                      true,
				serialNoValidation:                    true,
				mfgNameValidation:                     true,
				mfgDateValidation:                     false,
				swVerValidation:                       false,
				hwVerValidation:                       true,
				fwVerValidation:                       false,
				rrValidation:                          true,
				operStatus:                            oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE,
				parentValidation:                      true,
				pType:                                 componentType["Supervisor"],
			},
		}, {
			desc: "Transceiver",
			cardFields: properties{
				descriptionValidation:                 false,
				idValidation:                          false,
				installPositionAndComponentValidation: true,
				nameValidation:                        true,
				partNoValidation:                      true,
				serialNoValidation:                    true,
				mfgNameValidation:                     true,
				mfgDateValidation:                     false,
				swVerValidation:                       false,
				hwVerValidation:                       true,
				fwVerValidation:                       true,
				rrValidation:                          false,
				operStatus:                            oc.PlatformTypes_COMPONENT_OPER_STATUS_UNSET,
				parentValidation:                      false,
				pType:                                 componentType["Transceiver"],
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
			} else if tc.desc == "Fan Tray" && *args.NumFanTrays == 0 {
				t.Skip("Skip Fan Tray Telemetry check for fixed form factor devices.")
			} else if tc.desc == "Fan" && *args.NumFans == 0 {
				t.Skip("Skip Fan Telemetry check for fixed form factor devices.")
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

func TestControllerCardEmpty(t *testing.T) {
	if *args.NumControllerCards <= 0 {
		t.Skip("Skip ControllerCardEmpty Telemetry check for fixed form factor devices.")
	}

	dut := ondatra.DUT(t, "dut")
	controllerCards := findComponentsListByType(t, dut)["Supervisor"]
	if len(controllerCards) == 0 {
		t.Fatalf("Get ControllerCard list for %q: got 0, want > 0", dut.Model())
	}

	t.Logf("ControllerCard components count: %d", len(controllerCards))

	nonEmptyControllerCards := 0
	for _, controllerCard := range controllerCards {
		if controllerCard.Name == nil {
			t.Errorf("Encountered a ControllerCard with no Name")
			continue
		}

		sName := controllerCard.GetName()
		t.Run(sName, func(t *testing.T) {
			t.Logf("ControllerCard %s Id: %s", sName, controllerCard.GetId())

			if !controllerCard.GetEmpty() {
				nonEmptyControllerCards++
			}
		})
	}

	if got, want := nonEmptyControllerCards, *args.NumControllerCards; got != want {
		t.Errorf("Number of non-empty ControllerCard: got %d, want %d", got, want)
	}
}

// validateSubcomponentsExistAsComponents checks that if the given component has subcomponents, that
// those subcomponents exist as components on the device (i.e. the leafref is valid).
func validateSubcomponentsExistAsComponents(c *oc.Component, components []*oc.Component, t *testing.T, dut *ondatra.DUTDevice) {
	cName := c.GetName()
	subcomponentsValue := gnmi.Lookup(t, dut, gnmi.OC().Component(cName).SubcomponentMap().State())
	subcomponents, ok := subcomponentsValue.Val()
	if !ok {
		// Not all components have subcomponents
		// If the component doesn't have subcomponent, skip the check and return early
		return
	}
	for _, subc := range subcomponents {
		subcName := subc.GetName()
		subComponent := gnmi.Lookup(t, dut, gnmi.OC().Component(subcName).State())
		if !subComponent.IsPresent() {
			t.Errorf("Subcomponent %s does not exist as a component on the device", subcName)
		}
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
			validateSubcomponentsExistAsComponents(card, validCards, t, dut)
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

			if p.installPositionAndComponentValidation && !deviations.InstallPositionAndInstallComponentUnsupported(dut) {
				// If the component has a location and is removable, then it needs to have install-component
				// and install-position.
				if card.GetLocation() != "" && card.GetRemovable() {
					testInstallComponentAndInstallPosition(t, card, validCards)
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
					pLoookup := gnmi.Lookup(t, dut, gnmi.OC().Component(parent).Type().State())
					parentType, present := pLoookup.Val()
					if present && parentType == componentType["Chassis"] {
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

			if p.modelNameValidation {
				if deviations.ModelNameUnsupported(dut) {
					t.Logf("Telemetry path /components/component/state/model-name is not supported due to deviation ModelNameUnsupported. Skipping model name validation.")
				} else if card.GetModelName() != dut.Model() {
					t.Errorf("Component %s ModelName: got %s, want %s (dut's hardware model)", cName, card.GetModelName(), dut.Model())
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

func testInstallComponentAndInstallPosition(t *testing.T, c *oc.Component, components []*oc.Component) {
	icName := c.GetInstallComponent()
	ip := c.GetInstallPosition()
	hasInstallComponentAndPosition(t, c, icName, ip)
	validateInstallComponent(t, icName, components, c)
}

func validateInstallComponent(t *testing.T, icName string, components []*oc.Component, c *oc.Component) {
	compMap := compNameMap(t, ondatra.DUT(t, "dut"))
	ic, ok := compMap[icName]
	if !ok {
		t.Errorf("Component %s's install-component %s is not in component tree", icName, c.GetName())
		return
	}
	validTypes := validInstallComponentTypes[c.GetType()]
	icType := ic.GetType()
	if !validTypes[icType] {
		t.Errorf("Component %s's install-component %s is not a supported parent type (%s)", c.GetName(), icName, icType)
	}
}

func hasInstallComponentAndPosition(t *testing.T, c *oc.Component, icName string, ip string) {
	if icName == "" {
		t.Errorf("Component %s is missing install-component", c.GetName())
		return
	}
	if ip == "" {
		t.Errorf("Component %s is missing install-position", c.GetName())
		return
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

// Creates a map of component Name to corresponding Component OC object.
func compNameMap(t *testing.T, dut *ondatra.DUTDevice) map[string]*oc.Component {
	compMap := make(map[string]*oc.Component)
	for _, c := range gnmi.GetAll(t, dut, gnmi.OC().ComponentAny().State()) {
		compMap[c.GetName()] = c
	}
	return compMap
}

func TestInterfaceComponentHierarchy(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	compMap := compNameMap(t, dut)

	// Map of populated Transceivers to a random integer.
	transceivers := make(map[string]int)
	tvs := components.FindComponentsByType(t, dut, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_TRANSCEIVER)
	for idx, tv := range tvs {
		if compMap[tv].GetMfgName() == "" {
			continue
		}
		transceivers[compMap[tv].GetName()] = idx
	}

	numHardwareIntfs := 0
	integratedCircuits := make(map[string]*oc.Component)

	t.Run("Interface to Integrated Circuit mapping", func(t *testing.T) {
		for _, intf := range gnmi.GetAll(t, dut, gnmi.OC().InterfaceAny().State()) {
			if intf.GetHardwarePort() == "" {
				continue
			}
			if _, ok := transceivers[intf.GetTransceiver()]; !ok {
				continue
			}
			t.Run(intf.GetHardwarePort(), func(t *testing.T) {
				numHardwareIntfs++
				c, ok := compMap[intf.GetHardwarePort()]
				if !ok {
					t.Fatalf("Couldn't find interface hardware port(%s) in component tree for port: %s", intf.GetHardwarePort(), intf.GetName())
				}
				for {
					if c.GetType() == oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_INTEGRATED_CIRCUIT {
						break
					}
					if c.GetParent() == "" {
						t.Fatalf("Couldn't get parent for component: %s", c.GetName())
					}
					c, ok = compMap[c.GetParent()]
					if !ok {
						t.Fatalf("Couldn't find parent component(%s) for component: %s", c.GetParent(), c.GetName())
					}
				}
				integratedCircuits[c.GetName()] = c
			})
		}
	})
	if len(integratedCircuits) == 0 {
		t.Fatalf("Couldn't find integrated circuits for %q", dut.Model())
	}
	chassis := make(map[string]*oc.Component)
	t.Run("Integrated Circuit to Chassis mapping", func(t *testing.T) {
		for _, ic := range integratedCircuits {
			t.Run(ic.GetName(), func(t *testing.T) {
				c, ok := ic, true
				for {
					if c.GetType() == oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CHASSIS {
						break
					}
					if c.GetParent() == "" {
						t.Fatalf("Couldn't get parent for component: %s", c.GetName())
					}
					c, ok = compMap[c.GetParent()]
					if !ok {
						t.Fatalf("Couldn't find parent component(%s) for component: %s", c.GetParent(), c.GetName())
					}
				}
				chassis[c.GetName()] = c
			})
		}
	})
	if len(chassis) == 0 {
		t.Fatalf("Couldn't find chassis for %q", dut.Model())
	}
}

func TestDefaultPowerAdminState(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	fabrics := []*oc.Component{}
	linecards := []*oc.Component{}
	supervisors := []*oc.Component{}

	components := gnmi.GetAll(t, dut, gnmi.OC().ComponentAny().State())
	for compName := range componentType {
		for _, c := range components {
			if c.GetType() == nil || c.GetType() != componentType[compName] {
				continue
			}
			switch compName {
			case "Fabric":
				fabrics = append(fabrics, c)
			case "Linecard":
				linecards = append(linecards, c)
			case "Supervisor":
				supervisors = append(supervisors, c)
			}
		}
	}

	t.Logf("Fabrics: %v", fabrics)
	t.Logf("Linecards: %v", linecards)
	t.Logf("Supervisors: %v", supervisors)

	if len(fabrics) != 0 {
		pas := gnmi.Get(t, dut, gnmi.OC().Component(fabrics[0].GetName()).Fabric().PowerAdminState().Config())
		t.Logf("Component %s PowerAdminState: %v", fabrics[0].GetName(), pas)
		if pas == oc.Platform_ComponentPowerType_UNSET {
			t.Errorf("Component %s PowerAdminState is unset", fabrics[0].GetName())
		}
	}

	if len(linecards) != 0 {
		pas := gnmi.Get(t, dut, gnmi.OC().Component(linecards[0].GetName()).Linecard().PowerAdminState().Config())
		t.Logf("Component %s PowerAdminState: %v", linecards[0].GetName(), pas)
		if pas == oc.Platform_ComponentPowerType_UNSET {
			t.Errorf("Component %s PowerAdminState is unset", linecards[0].GetName())
		}
	}
	if !deviations.SkipControllerCardPowerAdmin(dut) {
		if len(supervisors) != 0 {
			pas := gnmi.Get(t, dut, gnmi.OC().Component(supervisors[0].GetName()).ControllerCard().PowerAdminState().Config())
			t.Logf("Component %s PowerAdminState: %v", supervisors[0].GetName(), pas)
			if pas == oc.Platform_ComponentPowerType_UNSET {
				t.Errorf("Component %s PowerAdminState is unset", supervisors[0].GetName())
			}
		}
	}
}
