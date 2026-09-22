/*
 Copyright 2022 Google LLC

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

      https://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package system_time_test

import (
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ygnmi/ygnmi"
)

// TestCurrentDateTime verifies that the current date and time state path can
// be parsed as RFC3339 time format.
//
// telemetry_path:/system/state/current-datetime

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestCurrentDateTime(t *testing.T) {
	t.Skip("Need working implementation to validate against")

	dut := ondatra.DUT(t, "dut")
	now := gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State())
	_, err := time.Parse(time.RFC3339, now)
	if err != nil {
		t.Errorf("Failed to parse current time: got %s: %s", now, err)
	}
}

// TestBootTime verifies the timestamp that the system was last restarted can
// be read and is not an unreasonable value.
//
// telemetry_path:/system/state/boot-time
func TestBootTime(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	bt := gnmi.Get(t, dut, gnmi.OC().System().BootTime().State())

	// Boot time should be after Dec 22, 2021 00:00:00 GMT in nanoseconds
	if bt < 1640131200000000000 {
		t.Errorf("Unexpected boot timestamp: got %d; check clock", bt)
	}
}

// TestLastConfigurationTimestamp verifies that the timestamp of the last
// configuration change can be read and monotonically increases after a
// configuration update.
//
// config_path:/system/config/domain-name
// telemetry_path:/system/state/last-configuration-timestamp
func TestLastConfigurationTimestamp(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	domainConfig := gnmi.OC().System().DomainName()
	lastCfgTS := gnmi.OC().System().LastConfigurationTimestamp()

	initialDomainName, hasInitialDomainName := gnmi.LookupConfig(t, dut, domainConfig.Config()).Val()
	t.Cleanup(func() {
		if hasInitialDomainName {
			gnmi.Replace(t, dut, domainConfig.Config(), initialDomainName)
		} else {
			gnmi.Delete(t, dut, domainConfig.Config())
		}
	})

	initialTS := gnmi.Get(t, dut, lastCfgTS.State())
	t.Logf("Initial /system/state/last-configuration-timestamp: %d", initialTS)
	// Last configuration timestamp is reported in nanoseconds since the Unix
	// Epoch (Jan 1, 1970 00:00:00 UTC) and should be after Dec 22, 2021
	// 00:00:00 GMT (1640131200000000000 ns), verifying that the device clock is
	// synchronized and reporting in nanoseconds rather than seconds/milliseconds.
	if initialTS < 1640131200000000000 {
		t.Errorf("Unexpected initial last-configuration-timestamp: got %d, want >= 1640131200000000000", initialTS)
	}

	targetDomain := "test.name.example"
	if hasInitialDomainName && initialDomainName == targetDomain {
		targetDomain = "updated.test.name.example"
	}
	gnmi.Replace(t, dut, domainConfig.Config(), targetDomain)

	val, ok := gnmi.Watch(t, dut, lastCfgTS.State(), time.Minute, func(v *ygnmi.Value[uint64]) bool {
		ts, present := v.Val()
		return present && ts > initialTS
	}).Await(t)
	if !ok {
		updatedTS, _ := val.Val()
		t.Errorf("/system/state/last-configuration-timestamp did not increase after config change: initial %d, got %d", initialTS, updatedTS)
		return
	}
	updatedTS, _ := val.Val()
	t.Logf("Updated /system/state/last-configuration-timestamp: %d", updatedTS)
}

// TestTimeZone verifies the timezone-name config values can be read and set
//
// config_path:/system/clock/config/timezone-name
// telemetry_path:/system/clock/state/timezone-name
func TestTimeZone(t *testing.T) {
	t.Skip("Need working implementation to validate against")

	testCases := []struct {
		description string
		tz          string
	}{
		{"UTC", "Etc/UTC"},
		{"GMT", "Etc/GMT"},
		{"Short UTC", "UTC"},
		{"Short GMT", "GMT"},
		{"America/Chicago", "America/Chicago"},
		{"PST8PDT", "PST8PDT"},
		{"Europe/London", "Europe/London"},
	}

	dut := ondatra.DUT(t, "dut")

	for _, testCase := range testCases {
		t.Run(testCase.description, func(t *testing.T) {
			config := gnmi.OC().System().Clock().TimezoneName()
			state := gnmi.OC().System().Clock().TimezoneName()

			gnmi.Replace(t, dut, config.Config(), testCase.tz)

			t.Run("Get Timezone Config", func(t *testing.T) {
				configGot := gnmi.Get(t, dut, config.Config())
				if configGot != testCase.tz {
					t.Errorf("Config timezone: got %s, want %s", configGot, testCase.tz)
				}
			})

			t.Run("Get Timezone Telemetry", func(t *testing.T) {
				stateGot := gnmi.Await(t, dut, state.State(), 1*time.Minute, testCase.tz)
				if got, _ := stateGot.Val(); got != testCase.tz {
					t.Errorf("State domainname: got %v, want %s", stateGot, testCase.tz)
				}
			})

			t.Run("Delete Timezone", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				if qs := gnmi.LookupConfig(t, dut, config.Config()); qs.IsPresent() == true {
					t.Errorf("Delete timezone fail: got %v", qs)
				}
			})
		})
	}
}
