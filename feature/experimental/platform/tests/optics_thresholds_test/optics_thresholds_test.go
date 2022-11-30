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

package optics_thresholds_test

import (
	"testing"

	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
)

const (
	transceiverType                   = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_TRANSCEIVER
	minOpticsPower                    = -30.0
	maxOpticsPower                    = 10.0
	minOpticsPowerHighThreshold       = 1.0
	maxOpticsPowerLowThreshold        = -1.0
	minOpticsTemperature              = 90.0
	maxOpticsTemperature              = -20.0
	minOpticsTemperatureHighThreshold = 50.0
	maxOpticsTemperatureLowThreshold  = 10.0
	minOpticsBiasCurrent              = 95.0
	maxOpticsBiasCurrent              = 5.0
	minOpticsBiasCurrentHighThreshold = 50.0
	maxOpticsBiasCurrentLowThreshold  = 40.0
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Test cases:
//
// - Get a list of transceivers with installed optics
// - Verify that the following optics threshold  telemetry paths exist
// - Output power threshold:
//   - /components/component/Ethernet/properties/property/laser-tx-power-low-alarm-threshold/state/value
//   - /components/component/Ethernet/properties/property/laser-tx-power-high-alarm-threshold/state/value
//   - /components/component/Ethernet/properties/property/laser-tx-power-low-warn-threshold/state/value
//   - /components/component/Ethernet/properties/property/laser-tx-power-high-warn-threshold/state/value
// - Input power threshold:
//   - /components/component/Ethernet/properties/property/laser-rx-power-low-alarm-threshold/state/value
//   - /components/component/Ethernet/properties/property/laser-rx-power-high-alarm-threshold/state/value
//   - /components/component/Ethernet/properties/property/laser-rx-power-low-warn-threshold/state/value
//   - /components/component/Ethernet/properties/property/laser-rx-power-high-warn-threshold/state/value
// - Optics temperature threshold:
//   - /components/component/Ethernet/properties/property/laser-temperature-low-alarm-threshold/state/value
//   - /components/component/Ethernet/properties/property/laser-temperature-high-alarm-threshold/state/value
//   - /components/component/Ethernet/properties/property/laser-temperature-low-warn-threshold/state/value
//   - /components/component/Ethernet/properties/property/laser-temperature-high-warn-threshold/state/value
// - Optics bias-current threshold:
//   - /components/component/Ethernet/properties/property/laser-bias-current-low-alarm-threshold/state/value
//   - /components/component/Ethernet/properties/property/laser-bias-current-high-alarm-threshold/state/value
//   - /components/component/Ethernet/properties/property/laser-bias-current-low-warn-threshold/state/value
//   - /components/component/Ethernet/properties/property/laser-bias-current-high-warn-threshold/state/value

// Topology:
//   ate:port1 <--> port1:dut:port2 <--> ate:port2
//
//  Sample CLI command to get telemetry using gmic:
//   - gnmic -a ipaddr:10162 -u username -p password --skip-verify get \
//      --path /components/component --format flat
//   - gnmic tool info:
//     - https://github.com/karimra/gnmic/blob/main/README.md
//

func TestOpticsThresholds(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	transceivers := components.FindComponentsByType(t, dut, transceiverType)
	t.Logf("Found transceiver list: %v", transceivers)
	if len(transceivers) == 0 {
		t.Fatalf("Get transceiver list for %q: got 0, want > 0", dut.Model())
	}

	cases := []struct {
		desc         string
		property     string
		minThreshold float64
		maxThreshold float64
	}{{
		desc:         "Check laser-rx-power-high-warn-threshold",
		property:     "laser-rx-power-high-warn-threshold",
		minThreshold: minOpticsPowerHighThreshold,
		maxThreshold: maxOpticsPower,
	}, {
		desc:         "Check laser-rx-power-high-alarm-threshold",
		property:     "laser-rx-power-high-alarm-threshold",
		minThreshold: minOpticsPowerHighThreshold,
		maxThreshold: maxOpticsPower,
	}, {
		desc:         "Check laser-tx-power-high-warn-threshold",
		property:     "laser-tx-power-high-warn-threshold",
		minThreshold: minOpticsPowerHighThreshold,
		maxThreshold: maxOpticsPower,
	}, {
		desc:         "Check laser-tx-power-high-alarm-threshold",
		property:     "laser-tx-power-high-alarm-threshold",
		minThreshold: minOpticsPowerHighThreshold,
		maxThreshold: maxOpticsPower,
	}, {
		desc:         "Check laser-tx-power-low-warn-threshold",
		property:     "laser-tx-power-low-warn-threshold",
		minThreshold: minOpticsPower,
		maxThreshold: maxOpticsPowerLowThreshold,
	}, {
		desc:         "Check laser-tx-power-low-alarm-threshold",
		property:     "laser-tx-power-low-alarm-threshold",
		minThreshold: minOpticsPower,
		maxThreshold: maxOpticsPowerLowThreshold,
	}, {
		desc:         "Check laser-rx-power-low-warn-threshold",
		property:     "laser-rx-power-low-warn-threshold",
		minThreshold: minOpticsPower,
		maxThreshold: maxOpticsPowerLowThreshold,
	}, {
		desc:         "Check laser-rx-power-low-alarm-threshold",
		property:     "laser-rx-power-low-alarm-threshold",
		minThreshold: minOpticsPower,
		maxThreshold: maxOpticsPowerLowThreshold,
	}, {
		desc:         "Check laser-temperature-high-warn-threshold",
		property:     "laser-temperature-high-warn-threshold",
		minThreshold: minOpticsTemperatureHighThreshold,
		maxThreshold: maxOpticsTemperature,
	}, {
		desc:         "Check laser-temperature-high-alarm-threshold",
		property:     "laser-temperature-high-alarm-threshold",
		minThreshold: minOpticsTemperatureHighThreshold,
		maxThreshold: maxOpticsTemperature,
	}, {
		desc:         "Check laser-temperature-low-warn-threshold",
		property:     "laser-temperature-low-warn-threshold",
		minThreshold: minOpticsTemperature,
		maxThreshold: maxOpticsTemperatureLowThreshold,
	}, {
		desc:         "Check laser-temperature-low-alarm-threshold",
		property:     "laser-temperature-low-alarm-threshold",
		minThreshold: minOpticsTemperature,
		maxThreshold: maxOpticsTemperatureLowThreshold,
	}, {
		desc:         "Check laser-bias-current-high-warn-threshold",
		property:     "laser-bias-current-high-warn-threshold",
		minThreshold: minOpticsBiasCurrentHighThreshold,
		maxThreshold: maxOpticsBiasCurrent,
	}, {
		desc:         "Check laser-bias-current-high-alarm-threshold",
		property:     "laser-bias-current-high-alarm-threshold",
		minThreshold: minOpticsBiasCurrentHighThreshold,
		maxThreshold: maxOpticsBiasCurrent,
	}, {
		desc:         "Check laser-bias-current-low-warn-threshold",
		property:     "laser-bias-current-low-warn-threshold",
		minThreshold: minOpticsBiasCurrent,
		maxThreshold: maxOpticsBiasCurrentLowThreshold,
	}, {
		desc:         "Check laser-bias-current-low-alarm-threshold",
		property:     "laser-bias-current-low-alarm-threshold",
		minThreshold: minOpticsBiasCurrent,
		maxThreshold: maxOpticsBiasCurrentLowThreshold,
	}}

	for _, tc := range cases {
		t.Log(tc.desc)
		t.Run(tc.desc, func(t *testing.T) {
			for _, transceiver := range transceivers {
				t.Logf("Validate transceiver: %s", transceiver)
				component := gnmi.OC().Component(transceiver)
				if !gnmi.Lookup(t, dut, component.MfgName().State()).IsPresent() {
					t.Logf("component.MfgName().Lookup(t).IsPresent() for %q is false. skip it", transceiver)
					continue
				}
				mfgName := gnmi.Get(t, dut, component.MfgName().State())
				t.Logf("Transceiver %s MfgName: %s", transceiver, mfgName)

				threshold := fetchOpticsThreshold(t, dut, transceiver, tc.property)
				if threshold > tc.maxThreshold || threshold < tc.minThreshold {
					t.Errorf("Get threshold for %q): got %.2f, want within [%f, %f] ", transceiver, threshold, tc.minThreshold, tc.maxThreshold)
				}
			}
		})
	}
}

func fetchOpticsThreshold(t *testing.T, dut *ondatra.DUTDevice, opticsName string, property string) float64 {
	t.Helper()
	// TODO: Need to update the lookup code after optics threshold model is defined.
	t.Skipf("Optics threshold model needs to be defined, skip it for now.")

	val := gnmi.Get(t, dut, gnmi.OC().Component(opticsName).Property(property).State()).GetValue()
	switch v := val.(type) {
	case oc.UnionUint64:
		return float64(v)
	case oc.UnionInt64:
		return float64(v)
	case oc.UnionFloat64:
		return float64(v)
	default:
		t.Fatalf("Error extracting optics threshold, could not type assert union. union: %v", val)
		return 0
	}
}
