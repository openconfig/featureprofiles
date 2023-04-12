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
var componentsByType map[string][]string

func TestHardwarecards(t *testing.T) {
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
			t.Logf("Found card list for %v: %v", tc.desc, cards)

			if len(cards) == 0 {
				t.Fatalf("Get card list for %q) on %v: got 0, want > 0", tc.desc, dut.Model())
			}
			ValidateComponentState(t, dut, cards, tc.cardFields)
		})
	}
}

func findComponentsListByType(t *testing.T, dut *ondatra.DUTDevice) map[string][]string {
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
	componentsByType = make(map[string][]string)
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
			componentsByType[compName] = append(componentsByType[compName], c.GetName())
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
		t.Fatalf("Get SwitchChip card list for %q): got 0, want > 0", dut.Model())
	}
	t.Logf("Found SwitchChip list: %v", cards)

	ValidateComponentState(t, dut, cards, cardFields)

	for _, card := range cards {
		t.Logf("Validate card %s", card)
		component := gnmi.OC().Component(card)

		if deviations.BackplaneFacingCapacityUnsupported(ondatra.DUT(t, "dut")) && regexp.MustCompile("NPU[0-9]$").Match([]byte(card)) {
			// Vendor does not support backplane-facing-capacity for nodes named 'NPU'.
			continue
		} else {
			// For SwitchChip, check OC integrated-circuit paths.
			bpCapacity := component.IntegratedCircuit().BackplaneFacingCapacity()

			totalCapacity := gnmi.Get(t, dut, bpCapacity.TotalOperationalCapacity().State())
			t.Logf("Hardware card %s totalCapacity: %d", card, totalCapacity)
			if totalCapacity <= 0 {
				t.Errorf("bpCapacity.TotalOperationalCapacity().Get(t) for %q): got %v, want > 0", card, totalCapacity)
			}

			total := gnmi.Get(t, dut, bpCapacity.Total().State())
			t.Logf("Hardware card %s total: %d", card, totalCapacity)
			if total <= 0 {
				t.Errorf("bpCapacity.Total().Get(t) for %q): got %v, want > 0", card, total)
			}

			if !gnmi.Lookup(t, dut, bpCapacity.AvailablePct().State()).IsPresent() {
				t.Errorf("bpCapacity.AvailablePct() for %q): got none, want >= 0", card)
			}
			availablePct := gnmi.Get(t, dut, bpCapacity.AvailablePct().State())
			t.Logf("Hardware card %s availablePct: %d", card, availablePct)

			if !gnmi.Lookup(t, dut, bpCapacity.ConsumedCapacity().State()).IsPresent() {
				t.Errorf("bpCapacity.ConsumedCapacity() for %q): got none, want >= 0", card)
			}
			consumedCapacity := gnmi.Get(t, dut, bpCapacity.ConsumedCapacity().State())
			t.Logf("Hardware card %s consumedCapacity: %d", card, consumedCapacity)
		}
	}
}

func TestTempSensor(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	sensors := findComponentsListByType(t, dut)["TempSensor"]
	if len(sensors) == 0 {
		t.Fatalf("Get TempSensor list for %q: got 0, want > 0", dut.Model())
	}
	t.Logf("Found TempSensor list: %v", sensors)

	for _, sensor := range sensors {
		t.Logf("Validate card %s", sensor)
		component := gnmi.OC().Component(sensor)

		if !gnmi.Lookup(t, dut, component.Id().State()).IsPresent() {
			// Just log the result using Logf instead of Errorf.
			t.Logf("component.Id().Lookup(t) for %q: got false, want true", sensor)
		} else {
			t.Logf("TempSensor %s Id: %s", sensor, gnmi.Get(t, dut, component.Id().State()))
		}

		if !gnmi.Lookup(t, dut, component.Name().State()).IsPresent() {
			t.Errorf("component.Name().Lookup(t) for %q: got false, want true", sensor)
		} else {
			t.Logf("TempSensor %s Name: %s", sensor, gnmi.Get(t, dut, component.Name().State()))
		}

		if !gnmi.Lookup(t, dut, component.Type().State()).IsPresent() {
			t.Errorf("component.Type().Lookup(t) for %q: got false, want true", sensor)
			want := componentType["tempsensor"]
			got := fmt.Sprintf("%v", gnmi.Get(t, dut, component.Type().State()))
			if want != got {
				t.Errorf("component.Type().Val(t) for %q: got %v, want %v", sensor, got, want)
			}
		} else {
			t.Logf("TempSensor %s Type: %s", sensor, gnmi.Get(t, dut, component.Type().State()))
		}

		if !gnmi.Lookup(t, dut, component.Temperature().Instant().State()).IsPresent() {
			t.Errorf("Temperature().Instant().Lookup(t) for %q: got false, want true", sensor)
		} else {
			t.Logf("TempSensor %s Temperature instant: %v", sensor, gnmi.Get(t, dut, component.Temperature().Instant().State()))
		}

		if !gnmi.Lookup(t, dut, component.Temperature().AlarmStatus().State()).IsPresent() {
			t.Errorf("Temperature().AlarmStatus().Lookup(t) for %q: got false, want true", sensor)
		} else {
			t.Logf("TempSensor %s Temperature AlarmStatus: %v", sensor, gnmi.Get(t, dut, component.Temperature().AlarmStatus().State()))
		}

		if !gnmi.Lookup(t, dut, component.Temperature().Max().State()).IsPresent() {
			t.Errorf("Temperature().Max().Lookup(t) for %q: got false, want true", sensor)
		} else {
			t.Logf("TempSensor %s Temperature Max: %v", sensor, gnmi.Get(t, dut, component.Temperature().Max().State()))
		}

		if !gnmi.Lookup(t, dut, component.Temperature().MaxTime().State()).IsPresent() {
			t.Errorf("Temperature().MaxTime().Lookup(t) for %q: got false, want true", sensor)
		} else {
			t.Logf("TempSensor %s Temperature MaxTime: %v", sensor, gnmi.Get(t, dut, component.Temperature().MaxTime().State()))
		}
	}
}

func ValidateComponentState(t *testing.T, dut *ondatra.DUTDevice, cards []string, p properties) {
	t.Helper()

	for _, card := range cards {
		t.Logf("Validate card %s", card)
		component := gnmi.OC().Component(card)

		// For transceiver, only check the transceiver with optics installed.
		if strings.Contains(card, "transceiver") {
			if gnmi.Lookup(t, dut, component.MfgName().State()).IsPresent() {
				t.Logf("Optics is detected in %s with expected parent: %s", card, gnmi.Lookup(t, dut, component.Parent().State()))
			} else {
				t.Logf("Optics is not installed in %s, skip testing this transceiver", card)
				continue
			}
		}

		if p.descriptionValidation {
			description := gnmi.Get(t, dut, component.Description().State())
			t.Logf("Hardware card %s Description: %s", card, description)
			if description == "" {
				t.Errorf("component.Description().Get(t) for %q): got empty string, want non-empty string", card)
			}
		}

		if p.idValidation {
			if deviations.SwitchChipIDUnsupported(ondatra.DUT(t, "dut")) {
				t.Logf("Skipping check for switch chip id unsupport")
			} else {
				id := gnmi.Get(t, dut, component.Id().State())
				t.Logf("Hardware card %s Id: %s", card, id)
				if id == "" {
					t.Errorf("component.Id().Get(t) for %q): got empty string, want non-empty string", card)
				}
			}
		}

		if p.nameValidation {
			name := gnmi.Get(t, dut, component.Name().State())
			t.Logf("Hardware card %s Name: %s", card, name)
			if name == "" {
				t.Errorf("component.Name().Get(t) for %q): got empty string, want non-empty string", card)
			}
		}

		if p.partNoValidation {
			partNo := gnmi.Lookup(t, dut, component.PartNo().State())
			t.Logf("Hardware card %s PartNo: %v", card, partNo)
			if !partNo.IsPresent() {
				if gnmi.Get(t, dut, component.Type().State()) == fanType {
					fanTray := gnmi.Get(t, dut, component.Parent().State())
					fanTrayPartNo := gnmi.Lookup(t, dut, gnmi.OC().Component(fanTray).PartNo().State())
					t.Logf("Hardware card %s (parent of %s) PartNo: %v", fanTray, card, fanTrayPartNo)
					if !fanTrayPartNo.IsPresent() {
						t.Errorf("component.PartNo().Get(t) for %q and its parent): got empty string, want non-empty string", card)
					}
				} else {
					t.Errorf("component.PartNo().Get(t) for %q): got empty string, want non-empty string", card)
				}
			}
		}

		if p.serialNoValidation {
			serialNo := gnmi.Lookup(t, dut, component.SerialNo().State())
			t.Logf("Hardware card %s serialNo: %s", card, serialNo)
			if !serialNo.IsPresent() {
				if gnmi.Get(t, dut, component.Type().State()) == fanType {
					fanTray := gnmi.Get(t, dut, component.Parent().State())
					fanTraySErialNo := gnmi.Lookup(t, dut, gnmi.OC().Component(fanTray).SerialNo().State())
					t.Logf("Hardware card %s (parent of %s) PartNo: %v", fanTray, card, fanTraySErialNo)
					if !fanTraySErialNo.IsPresent() {
						t.Errorf("component.SerialNo().Get(t) for %q and its parent): got empty string, want non-empty string", card)
					}
				} else {
					t.Errorf("component.SerialNo().Get(t) for %q): got empty string, want non-empty string", card)
				}
			}
		}

		if p.mfgNameValidation {
			mfgName := gnmi.Get(t, dut, component.MfgName().State())
			t.Logf("Hardware card %s mfgName: %s", card, mfgName)
			if mfgName == "" {
				t.Errorf("Get mfgName for %q): got empty string, want non-empty string", card)
			}
		}

		if p.mfgDateValidation {
			mfgDate := gnmi.Get(t, dut, component.MfgDate().State())
			t.Logf("Hardware card %s mfgDate: %s", card, mfgDate)
			if mfgDate == "" {
				t.Errorf("component.MfgName().Get(t) for %q): got empty string, want non-empty string", card)
			}
		}

		if p.swVerValidation {
			// Only a subset of cards are expected to report Software Version.
			sw, present := gnmi.Lookup(t, dut, component.SoftwareVersion().State()).Val()
			if present {
				t.Logf("Hardware card %s SoftwareVersion: %s", card, sw)
				if sw == "" {
					t.Errorf("component.SoftwareVersion().Get(t) for %q): got empty string, want non-empty string", card)
				}
			} else {
				t.Errorf("component.SoftwareVersion().Lookup(t) for %q): got no value.", card)
			}
		}

		if p.hwVerValidation {
			hardwareVersion := gnmi.Get(t, dut, component.HardwareVersion().State())
			t.Logf("Hardware card %s hardwareVersion: %s", card, hardwareVersion)
			if hardwareVersion == "" {
				t.Errorf("component.HardwareVersion().Get(t) for %q): got empty string, want non-empty string", card)
			}
		}

		if p.fwVerValidation {
			firmwareVersion := gnmi.Get(t, dut, component.FirmwareVersion().State())
			t.Logf("Hardware card %s FirmwareVersion: %s", card, firmwareVersion)
			if firmwareVersion == "" {
				t.Errorf("component.FirmwareVersion().Get(t) for %q): got empty string, want non-empty string", card)
			}
		}

		if p.rrValidation {
			redundantRole := gnmi.Get(t, dut, component.RedundantRole().State()).String()
			t.Logf("Hardware card %s RedundantRole: %s", card, redundantRole)
			if redundantRole != "PRIMARY" && redundantRole != "SECONDARY" {
				t.Errorf("component.RedundantRole().Get(t) for %q): got %v, want %v", card, redundantRole, p.operStatus)
			}
		}

		if p.operStatus != "" {
			if deviations.FanOperStatusUnsupported(dut) && strings.Contains(card, "Fan") {
				t.Logf("Skipping check for fan oper-status")
			} else {
				operStatus := gnmi.Get(t, dut, component.OperStatus().State()).String()
				t.Logf("Hardware card %s OperStatus: %s", card, operStatus)
				if operStatus != activeStatus {
					t.Errorf("component.OperStatus().Get(t) for %q): got %v, want %v", card, operStatus, p.operStatus)
				}
			}
		}

		if p.parentValidation {
			cur := card
			for {
				val := gnmi.Lookup(t, dut, gnmi.OC().Component(cur).Parent().State())
				parent, present := val.Val()
				if !present {
					t.Errorf("Hardware card %s Parent: Chassis component NOT found in the hierarchy tree of component", card)
					break
				}
				parentType := gnmi.Get(t, dut, gnmi.OC().Component(parent).Type().State())
				if parentType == chassisType {
					t.Logf("Hardware card %s Parent: Found chassis component in the hierarchy tree of component", card)
					break
				}
				if parent == cur {
					t.Errorf("Hardware card %s Parent: Chassis component NOT found in the hierarchy tree of component", card)
					break
				}
				cur = parent
			}
		}

		if p.pType != nil {
			ptype := gnmi.Get(t, dut, component.Type().State())
			t.Logf("Hardware card %s type: %v", card, ptype)
			if ptype != p.pType {
				t.Errorf("component.Type().Get(t) for %q): got %v, want %v", card, ptype, p.pType)
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
