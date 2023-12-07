// Copyright 2023 Google LLC
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

package sampled_backplane_capacity_counters_test

import (
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
)

// Topology:
//       ATE port 1
//        |
//       DUT--------ATE port 3
//        |
//       ATE port 2

var (
	icPattern = map[ondatra.Vendor]string{
		ondatra.ARISTA:  "^SwitchChip",
		ondatra.CISCO:   "^[0-9]/[0-9]/CPU[0-9]-NPU[0-9]",
		ondatra.JUNIPER: "NPU[0-9]$",
		ondatra.NOKIA:   "^SwitchChip",
	}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestSampledBackplaneCapacityCounters(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	ics := components.FindComponentsByType(t, dut, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_INTEGRATED_CIRCUIT)
	if len(ics) == 0 {
		t.Fatalf("Get IntegratedCircuit card list for %q: got 0, want > 0", dut.Model())
	}
	t.Logf("IntegratedCircuit components count: %d", len(ics))

	subscribeTimeout := 30 * time.Second

	for _, ic := range ics {
		if !isCompNameExpected(t, ic, dut.Vendor()) {
			continue
		}

		t.Run(fmt.Sprintf("Backplane:%s", ic), func(t *testing.T) {
			if deviations.BackplaneFacingCapacityUnsupported(dut) {
				t.Skipf("Skipping check for BackplanceFacingCapacity due to deviation BackplaneFacingCapacityUnsupported")
			}

			for _, sampleInterval := range []time.Duration{10 * time.Second, 15 * time.Second} {
				minWant := int(subscribeTimeout/sampleInterval) - 1
				consumedCapacities := gnmi.Collect(t, gnmiOptsForSample(t, dut, sampleInterval), gnmi.OC().Component(ic).IntegratedCircuit().BackplaneFacingCapacity().ConsumedCapacity().State(), subscribeTimeout).Await(t)
				if len(consumedCapacities) < minWant {
					t.Errorf("ConsumedCapacities: got %d, want >= %d", len(consumedCapacities), minWant)
				}
			}
		})
	}
}

func isCompNameExpected(t *testing.T, name string, vendor ondatra.Vendor) bool {
	t.Helper()

	regexpPattern, ok := icPattern[vendor]
	if !ok {
		return false
	}
	r, err := regexp.Compile(regexpPattern)
	if err != nil {
		t.Fatalf("Cannot compile regular expression: %v", err)
	}
	return r.MatchString(name)
}

func gnmiOptsForSample(t *testing.T, dut *ondatra.DUTDevice, interval time.Duration) *gnmi.Opts {
	return dut.GNMIOpts().WithYGNMIOpts(
		ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_SAMPLE),
		ygnmi.WithSampleInterval(interval),
	)
}

func gnmiOptsForOnChange(t *testing.T, dut *ondatra.DUTDevice) *gnmi.Opts {
	return dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_ON_CHANGE))
}

func TestOnChangeBackplaneCapacityCounters(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	ics := components.FindComponentsByType(t, dut, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_INTEGRATED_CIRCUIT)
	if len(ics) == 0 {
		t.Fatalf("Get IntegratedCircuit card list for %q: got 0, want > 0", dut.Model())
	}
	t.Logf("IntegratedCircuit components count: %d", len(ics))

	fabrics := components.FindComponentsByType(t, dut, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_FABRIC)
	if len(fabrics) == 0 {
		t.Skipf("Get Fabric card list for %q: got 0, want > 0", dut.Model())
	}
	t.Logf("Fabric components count: %d", len(fabrics))

	ts1, tocs1, apct1 := getBackplaneCapacityCounters(t, dut, ics)

	fc := (len(fabrics) / 2) + 1
	for _, f := range fabrics[:fc] {
		gnmi.Replace(t, dut, gnmi.OC().Component(f).Fabric().PowerAdminState().Config(), oc.Platform_ComponentPowerType_POWER_DISABLED)
		gnmi.Await(t, dut, gnmi.OC().Component(f).Fabric().PowerAdminState().State(), time.Minute, oc.Platform_ComponentPowerType_POWER_DISABLED)
	}

	ts2, tocs2, apct2 := getBackplaneCapacityCounters(t, dut, ics)

	for _, f := range fabrics[:fc] {
		gnmi.Replace(t, dut, gnmi.OC().Component(f).Fabric().PowerAdminState().Config(), oc.Platform_ComponentPowerType_POWER_ENABLED)
		if deviations.MissingValueForDefaults(dut) {
			time.Sleep(time.Minute)
		} else {
			if power, ok := gnmi.Await(t, dut, gnmi.OC().Component(f).Fabric().PowerAdminState().State(), time.Minute, oc.Platform_ComponentPowerType_POWER_ENABLED).Val(); !ok {
				t.Errorf("Component %s, power-admin-state got: %v, want: %v", f, power, oc.Platform_ComponentPowerType_POWER_ENABLED)
			}
		}
		if oper, ok := gnmi.Await(t, dut, gnmi.OC().Component(f).OperStatus().State(), 2*time.Minute, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE).Val(); !ok {
			t.Errorf("Component %s oper-status after POWER_ENABLED, got: %v, want: %v", f, oper, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE)
		}
	}

	ts3, tocs3, apct3 := getBackplaneCapacityCounters(t, dut, ics)

	for _, ic := range ics {
		if !isCompNameExpected(t, ic, dut.Vendor()) {
			continue
		}

		t.Run(fmt.Sprintf("Backplane:CountersCheck:%s", ic), func(t *testing.T) {
			if deviations.BackplaneFacingCapacityUnsupported(dut) {
				t.Skipf("Skipping check for BackplanceFacingCapacity due to deviation BackplaneFacingCapacityUnsupported")
			}

			v1, ok1 := ts1[ic]
			v2, ok2 := ts2[ic]
			v3, ok3 := ts3[ic]
			switch {
			case !ok1 || !ok2 || !ok3:
				t.Errorf("BackplaneFacingCapacity Total not present: ok1 %t, ok2 %t, ok3 %t", ok1, ok2, ok3)
			case v1 <= v2 || v1 != v3:
				t.Errorf("BackplaneFacingCapacity Total are not valid: v1 %d, v2 %d, v3 %d", v1, v2, v3)
			}

			v1, ok1 = tocs1[ic]
			v2, ok2 = tocs2[ic]
			v3, ok3 = tocs3[ic]
			switch {
			case !ok1 || !ok2 || !ok3:
				t.Errorf("BackplaneFacingCapacity TotalOperationalCapacity not present: ok1 %t, ok2 %t, ok3 %t", ok1, ok2, ok3)
			case v1 <= v2 || v1 != v3:
				t.Errorf("BackplaneFacingCapacity TotalOperationalCapacity are not valid: v1 %d, v2 %d, v3 %d", v1, v2, v3)
			}

			v1, ok1 = apct1[ic]
			v2, ok2 = apct2[ic]
			v3, ok3 = apct3[ic]
			switch {
			case !ok1 || !ok2 || !ok3:
				t.Errorf("BackplaneFacingCapacity AvailablePct not present: ok1 %t, ok2 %t, ok3 %t", ok1, ok2, ok3)
			case v1 <= v2 || v1 != v3:
				t.Errorf("BackplaneFacingCapacity AvailablePct are not valid: v1 %d, v2 %d, v3 %d", v1, v2, v3)
			}
		})
	}
}

func getBackplaneCapacityCounters(t *testing.T, dut *ondatra.DUTDevice, ics []string) (map[string]uint64, map[string]uint64, map[string]uint64) {
	subscribeTimeout := 30 * time.Second

	totals := make(map[string]uint64)
	totalOperationalCapacities := make(map[string]uint64)
	availablePcts := make(map[string]uint64)
	for _, ic := range ics {
		if !isCompNameExpected(t, ic, dut.Vendor()) {
			continue
		}

		t.Run(fmt.Sprintf("Backplane:%s", ic), func(t *testing.T) {
			if deviations.BackplaneFacingCapacityUnsupported(dut) {
				t.Skipf("Skipping check for BackplanceFacingCapacity due to deviation BackplaneFacingCapacityUnsupported")
			}

			ts, ok := gnmi.Watch(t, gnmiOptsForOnChange(t, dut), gnmi.OC().Component(ic).IntegratedCircuit().BackplaneFacingCapacity().Total().State(), subscribeTimeout, func(v *ygnmi.Value[uint64]) bool {
				return v.IsPresent()
			}).Await(t)
			if ok {
				v, _ := ts.Val()
				totals[ic] = v
			}

			tocs, ok := gnmi.Watch(t, gnmiOptsForOnChange(t, dut), gnmi.OC().Component(ic).IntegratedCircuit().BackplaneFacingCapacity().TotalOperationalCapacity().State(), subscribeTimeout, func(v *ygnmi.Value[uint64]) bool {
				return v.IsPresent()
			}).Await(t)
			if ok {
				v, _ := tocs.Val()
				totalOperationalCapacities[ic] = v
			}

			apcts, ok := gnmi.Watch(t, gnmiOptsForOnChange(t, dut), gnmi.OC().Component(ic).IntegratedCircuit().BackplaneFacingCapacity().AvailablePct().State(), subscribeTimeout, func(v *ygnmi.Value[uint16]) bool {
				return v.IsPresent()
			}).Await(t)
			if ok {
				v, _ := apcts.Val()
				availablePcts[ic] = uint64(v)
			}
		})
	}

	return totals, totalOperationalCapacities, availablePcts
}
