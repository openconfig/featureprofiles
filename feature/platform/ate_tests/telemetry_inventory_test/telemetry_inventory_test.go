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

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
)

var activeStatus string = "ACTIVE"

var componentParent = map[string]string{
	"fabric":      "Chassis",
	"linecard":    "Chassis",
	"powersupply": "Chassis",
	"supervisor":  "Chassis",
}

var componentType = map[string]string{
	"chassis":     "CHASSIS",
	"fabric":      "FABRIC",
	"fabricchip":  "INTEGRATED_CIRCUIT",
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
	operStatus            string
	parent                string
	pType                 string
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Test cases:
//  - Validate Telemetry for each FRU within chassis.
//  - For each of the following component types, validate
//    1) Presence of component within gNMI telemetry.
//    2) Presence of component properties such as description, part-no, serial-no and oper-status etc.
//  - Validate the following components:
//    - Chassis
//    - Line card
//    - Power Supply
//    - Fabric card
//    - FabricChip
//    - Fan
//    - Supervisor or Controller
//       - Validate telemetry components/component/state/software-version.
//    - SwitchChip
//       - Validate the presence of the following OC paths under SwitchChip component:
//         - integrated-circuit/backplane-facing-capacity/state/available-pct
//         - integrated-circuit/backplane-facing-capacity/state/consumed-capacity
//         - integrated-circuit/backplane-facing-capacity/state/total
//         - integrated-circuit/backplane-facing-capacity/state/total-operational-capacity
//    - Transceiver
//    - Storage
//      - Validate telemetry /components/component/storage exists.
//    - TempSensor
//      - Validate telemetry /components/component/state/temperature/instant exists.
//
// Topology:
//   dut:port1 <--> ate:port1
//
// Test notes:
//  - Test cases for Software Module and Storage are skipped due to the blocking bugs:
//     - Need to support telemetry path /components/component/software-module.
//     - Need to support telemetry path /components/component/storage.
//
//  Sample CLI command to get component inventory using gmic:
//   - gnmic -a ipaddr:10162 -u username -p password --skip-verify get \
//      --path /components/component --format flat
//   - gnmic tool info:
//     - https://github.com/karimra/gnmic/blob/main/README.md
//

func TestHardwarecards(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	cases := []struct {
		desc          string
		regexpPattern string
		cardFields    properties
	}{{
		desc:          "Chassis",
		regexpPattern: "^Chassis",
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
			operStatus:            activeStatus,
			parent:                "",
			pType:                 componentType["chassis"],
		},
	}, {
		desc:          "Fabric",
		regexpPattern: "^Fabric[0-9]",
		cardFields: properties{
			descriptionValidation: true,
			idValidation:          true,
			nameValidation:        true,
			partNoValidation:      true,
			serialNoValidation:    true,
			mfgNameValidation:     true,
			mfgDateValidation:     true,
			hwVerValidation:       true,
			fwVerValidation:       false,
			operStatus:            activeStatus,
			parent:                componentParent["fabric"],
			pType:                 componentType["fabric"],
		},
	}, {
		desc:          "FabricChip",
		regexpPattern: "^FabricChip",
		cardFields: properties{
			descriptionValidation: true,
			idValidation:          true,
			nameValidation:        true,
			partNoValidation:      true,
			serialNoValidation:    false,
			mfgNameValidation:     false,
			mfgDateValidation:     false,
			hwVerValidation:       false,
			fwVerValidation:       true,
			operStatus:            "",
			parent:                "",
			pType:                 componentType["fabricchip"],
		},
	}, {
		desc:          "FAN",
		regexpPattern: "^Fan[0-9]",
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
			operStatus:            activeStatus,
			parent:                "",
			pType:                 componentType["fan"],
		},
	}, {
		desc:          "Linecard",
		regexpPattern: "^Linecard[0-9]",
		cardFields: properties{
			descriptionValidation: true,
			idValidation:          true,
			nameValidation:        true,
			partNoValidation:      true,
			serialNoValidation:    true,
			mfgNameValidation:     true,
			mfgDateValidation:     true,
			hwVerValidation:       true,
			fwVerValidation:       false,
			operStatus:            activeStatus,
			parent:                componentParent["linecard"],
			pType:                 componentType["linecard"],
		},
	}, {
		desc:          "Power supply",
		regexpPattern: "^PowerSupply[0-9]",
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
			operStatus:            activeStatus,
			parent:                componentParent["powersupply"],
			pType:                 componentType["powersupply"],
		},
	}, {
		desc:          "Supervisor",
		regexpPattern: "^Supervisor[0-9]$",
		cardFields: properties{
			descriptionValidation: true,
			idValidation:          true,
			nameValidation:        true,
			partNoValidation:      true,
			serialNoValidation:    true,
			mfgNameValidation:     true,
			mfgDateValidation:     true,
			swVerValidation:       true,
			hwVerValidation:       true,
			fwVerValidation:       false,
			operStatus:            "",
			parent:                componentParent["supervisor"],
			pType:                 componentType["supervisor"],
		},
	}, {
		desc:          "Transceiver",
		regexpPattern: "transceiver$",
		cardFields: properties{
			descriptionValidation: false,
			idValidation:          false,
			nameValidation:        true,
			partNoValidation:      true,
			serialNoValidation:    true,
			mfgNameValidation:     true,
			mfgDateValidation:     true,
			swVerValidation:       false,
			hwVerValidation:       true,
			fwVerValidation:       false,
			operStatus:            "",
			parent:                "",
			pType:                 componentType["transceiver"],
		},
	}}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			r, err := regexp.Compile(tc.regexpPattern)
			if err != nil {
				t.Fatalf("Cannot compile regular expression: %v", err)
			}
			cards := findMatchedComponents(t, dut, r)
			t.Logf("Found card list for %v: %v", tc.desc, cards)

			if len(cards) == 0 {
				t.Fatalf("Get card list for %q) on %v: got 0, want > 0", tc.desc, dut.Model())
			}
			ValidateComponentState(t, dut, cards, tc.cardFields)
		})
	}
}

func TestSwitchChip(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	regexpPattern := "^SwitchChip"
	cardFields := properties{
		descriptionValidation: false,
		idValidation:          true,
		nameValidation:        true,
		partNoValidation:      true,
		serialNoValidation:    false,
		mfgNameValidation:     false,
		mfgDateValidation:     false,
		swVerValidation:       false,
		hwVerValidation:       false,
		fwVerValidation:       true,
		operStatus:            "",
		parent:                "",
		pType:                 componentType["switchchip"],
	}

	r, err := regexp.Compile(regexpPattern)
	if err != nil {
		t.Fatalf("Cannot compile regular expression: %v", err)
	}
	cards := findMatchedComponents(t, dut, r)
	t.Logf("Found SwitchChip list: %v", cards)

	if len(cards) == 0 {
		t.Fatalf("Get SwitchChip card list for %q): got 0, want > 0", dut.Model())
	}
	ValidateComponentState(t, dut, cards, cardFields)

	for _, card := range cards {
		t.Logf("Validate card %s", card)
		component := dut.Telemetry().Component(card)

		// For SwitchChip, check OC integrated-circuit paths.
		bpCapacity := component.IntegratedCircuit().BackplaneFacingCapacity()

		totalCapacity := bpCapacity.TotalOperationalCapacity().Get(t)
		t.Logf("Hardware card %s totalCapacity: %d", card, totalCapacity)
		if totalCapacity <= 0 {
			t.Errorf("bpCapacity.TotalOperationalCapacity().Get(t) for %q): got %v, want > 0", card, totalCapacity)
		}

		total := bpCapacity.Total().Get(t)
		t.Logf("Hardware card %s total: %d", card, totalCapacity)
		if total <= 0 {
			t.Errorf("bpCapacity.Total().Get(t) for %q): got %v, want > 0", card, total)
		}

		if !bpCapacity.AvailablePct().Lookup(t).IsPresent() {
			t.Errorf("bpCapacity.AvailablePct() for %q): got none, want >= 0", card)
		}
		availablePct := bpCapacity.AvailablePct().Get(t)
		t.Logf("Hardware card %s availablePct: %d", card, availablePct)

		if !bpCapacity.ConsumedCapacity().Lookup(t).IsPresent() {
			t.Errorf("bpCapacity.ConsumedCapacity() for %q): got none, want >= 0", card)
		}
		consumedCapacity := bpCapacity.ConsumedCapacity().Get(t)
		t.Logf("Hardware card %s consumedCapacity: %d", card, consumedCapacity)
	}
}

func TestTempSensor(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	r := regexp.MustCompile("^TempSensor")
	sensors := findMatchedComponents(t, dut, r)
	t.Logf("Found TempSensor list: %v", sensors)

	if len(sensors) == 0 {
		t.Fatalf("Get TempSensor list for %q: got 0, want > 0", dut.Model())
	}

	for _, sensor := range sensors {
		t.Logf("Validate card %s", sensor)
		component := dut.Telemetry().Component(sensor)

		if !component.Id().Lookup(t).IsPresent() {
			// Just log the result using Logf instead of Errorf.
			t.Logf("component.Id().Lookup(t) for %q: got false, want true", sensor)
		} else {
			t.Logf("TempSensor %s Id: %s", sensor, component.Id().Lookup(t).Val(t))
		}

		if !component.Name().Lookup(t).IsPresent() {
			t.Errorf("component.Name().Lookup(t) for %q: got false, want true", sensor)
		} else {
			t.Logf("TempSensor %s Name: %s", sensor, component.Name().Lookup(t).Val(t))
		}

		if !component.Type().Lookup(t).IsPresent() {
			t.Errorf("component.Type().Lookup(t) for %q: got false, want true", sensor)
			want := componentType["tempsensor"]
			got := fmt.Sprintf("%v", component.Type().Get(t))
			if want != got {
				t.Errorf("component.Type().Val(t) for %q: got %v, want %v", sensor, got, want)
			}
		} else {
			t.Logf("TempSensor %s Type: %s", sensor, component.Type().Lookup(t).Val(t))
		}

		if !component.Temperature().Instant().Lookup(t).IsPresent() {
			t.Errorf("Temperature().Instant().Lookup(t) for %q: got false, want true", sensor)
		} else {
			t.Logf("TempSensor %s Temperature instant: %v", sensor, component.Temperature().Instant().Lookup(t).Val(t))
		}

		if !component.Temperature().AlarmStatus().Lookup(t).IsPresent() {
			t.Errorf("Temperature().AlarmStatus().Lookup(t) for %q: got false, want true", sensor)
		} else {
			t.Logf("TempSensor %s Temperature AlarmStatus: %v", sensor, component.Temperature().AlarmStatus().Lookup(t).Val(t))
		}

		if !component.Temperature().Max().Lookup(t).IsPresent() {
			t.Errorf("Temperature().Max().Lookup(t) for %q: got false, want true", sensor)
		} else {
			t.Logf("TempSensor %s Temperature Max: %v", sensor, component.Temperature().Max().Lookup(t).Val(t))
		}

		if !component.Temperature().MaxTime().Lookup(t).IsPresent() {
			t.Errorf("Temperature().MaxTime().Lookup(t) for %q: got false, want true", sensor)
		} else {
			t.Logf("TempSensor %s Temperature MaxTime: %v", sensor, component.Temperature().MaxTime().Lookup(t).Val(t))
		}
	}
}

func findMatchedComponents(t *testing.T, dut *ondatra.DUTDevice, r *regexp.Regexp) []string {
	components := dut.Telemetry().ComponentAny().Name().Get(t)
	var s []string
	for _, c := range components {
		if len(r.FindString(c)) > 0 {
			s = append(s, c)
		}
	}
	return s
}

func ValidateComponentState(t *testing.T, dut *ondatra.DUTDevice, cards []string, p properties) {
	t.Helper()

	swVersionFound := false
	for _, card := range cards {
		t.Logf("Validate card %s", card)
		component := dut.Telemetry().Component(card)

		// For transceiver, only check the transceiver with optics installed.
		if strings.Contains(card, "transceiver") {
			if component.MfgName().Lookup(t).IsPresent() {
				p.parent = strings.Fields(card)[0]
				t.Logf("Optics is detected in %s with expected parent: %s", card, p.parent)
			} else {
				t.Logf("Optics is not installed in %s, skip testing this transceiver", card)
				continue
			}
		}

		if p.descriptionValidation {
			description := component.Description().Get(t)
			t.Logf("Hardware card %s Description: %s", card, description)
			if description == "" {
				t.Errorf("component.Description().Get(t) for %q): got empty string, want non-empty string", card)
			}
		}

		if p.idValidation {
			id := component.Id().Get(t)
			t.Logf("Hardware card %s Id: %s", card, id)
			if id == "" {
				t.Errorf("component.Id().Get(t) for %q): got empty string, want non-empty string", card)
			}
		}

		if p.nameValidation {
			name := component.Name().Get(t)
			t.Logf("Hardware card %s Name: %s", card, name)
			if name == "" {
				t.Errorf("component.Name().Get(t) for %q): got empty string, want non-empty string", card)
			}
		}

		if p.partNoValidation {
			partNo := component.PartNo().Get(t)
			t.Logf("Hardware card %s PartNo: %s", card, partNo)
			if partNo == "" {
				t.Errorf("component.PartNo().Get(t) for %q): got empty string, want non-empty string", card)
			}
		}

		if p.serialNoValidation {
			serialNo := component.SerialNo().Get(t)
			t.Logf("Hardware card %s serialNo: %s", card, serialNo)
			if serialNo == "" {
				t.Errorf("component.SerialNo().Get(t) for %q): got empty string, want non-empty string", card)
			}
		}

		if p.mfgNameValidation {
			mfgName := component.MfgName().Get(t)
			t.Logf("Hardware card %s mfgName: %s", card, mfgName)
			if mfgName == "" {
				t.Errorf("Get mfgName for %q): got empty string, want non-empty string", card)
			}
		}

		if p.mfgDateValidation {
			mfgDate := component.MfgDate().Get(t)
			t.Logf("Hardware card %s mfgDate: %s", card, mfgDate)
			if mfgDate == "" {
				t.Errorf("component.MfgName().Get(t) for %q): got empty string, want non-empty string", card)
			}
		}

		if p.swVerValidation {
			softwareVersion := ""
			// Only a subset of cards are expected to report Software Version.
			sw := component.SoftwareVersion().Lookup(t)
			if sw != nil {
				softwareVersion = sw.Val(t)
				t.Logf("Hardware card %s SoftwareVersion: %s", card, softwareVersion)
				swVersionFound = true
				if softwareVersion == "" {
					t.Errorf("component.SoftwareVersion().Get(t) for %q): got empty string, want non-empty string", card)
				}
			} else {
				t.Logf("component.SoftwareVersion().Lookup(t) for %q): got no value.", card)
			}
		}

		if p.hwVerValidation {
			hardwareVersion := component.HardwareVersion().Get(t)
			t.Logf("Hardware card %s hardwareVersion: %s", card, hardwareVersion)
			if hardwareVersion == "" {
				t.Errorf("component.HardwareVersion().Get(t) for %q): got empty string, want non-empty string", card)
			}
		}

		if p.fwVerValidation {
			firmwareVersion := component.FirmwareVersion().Get(t)
			t.Logf("Hardware card %s FirmwareVersion: %s", card, firmwareVersion)
			if firmwareVersion == "" {
				t.Errorf("component.FirmwareVersion().Get(t) for %q): got empty string, want non-empty string", card)
			}
		}

		if p.operStatus != "" {
			operStatus := component.OperStatus().Get(t).String()
			t.Logf("Hardware card %s OperStatus: %s", card, operStatus)
			if operStatus != activeStatus {
				t.Errorf("component.OperStatus().Get(t) for %q): got %v, want %v", card, operStatus, p.operStatus)
			}
		}

		if p.parent != "" {
			parent := component.Parent().Get(t)
			t.Logf("Hardware card %s parent: %s", card, parent)
			if parent != p.parent {
				t.Errorf("component.Parent().Get(t) for %q): got %v, want %v", card, parent, p.parent)
			}
		}

		if p.pType != "" {
			ptype := component.Type().Get(t)
			t.Logf("Hardware card %s type: %v", card, ptype)

			if fmt.Sprintf("%v", ptype) != p.pType {
				t.Errorf("component.Type().Get(t) for %q): got %v, want %v", card, ptype, p.pType)
			}
		}
	}
	if p.swVerValidation && !swVersionFound {
		t.Errorf("Failed to find software version from %v", cards)
	}
}

func TestSoftwareModule(t *testing.T) {
	// TODO: Enable Software Module test case here once supported
	t.Skipf("Telemetry path /components/component/software-module is not supported.")

	dut := ondatra.DUT(t, "dut")
	moduleTypes := dut.Telemetry().ComponentAny().SoftwareModule().ModuleType().Lookup(t)
	if len(moduleTypes) == 0 {
		t.Errorf("Get moduleType list for %q: got 0, want > 0", dut.Model())
	}

	for i, moduleType := range moduleTypes {
		if !moduleType.IsPresent() {
			t.Fatalf("moduleType.IsPresent() item %d: got false, want true", i)
		}
		t.Logf("Telemetry moduleType path/value %d: %v=>%v.", i, moduleType.GetPath().String(), moduleType.Val(t))
	}
}

func TestStorage(t *testing.T) {
	// TODO: Add Storage test case here once supported.
	t.Skipf("Telemetry path /components/component/storage is not supported.")

	dut := ondatra.DUT(t, "dut")
	storages := dut.Telemetry().ComponentAny().Storage().Lookup(t)
	if len(storages) == 0 {
		t.Errorf("Get Storage list for %q: got 0, want > 0", dut.Model())
	}

	for i, storage := range storages {
		if !storage.IsPresent() {
			t.Fatalf("storage.IsPresent() item %d: got false, want true", i)
		}
		t.Logf("Telemetry storage path/value %d: %v=>%v.", i, storage.GetPath().String(), storage.Val(t))
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
