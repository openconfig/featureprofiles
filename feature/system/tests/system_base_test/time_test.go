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

package system_base_test

import (
	"testing"
	"time"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
)

// TestCurrentDateTime verifies that the current date and time state path can
// be parsed as RFC3339 time format.
//
// telemetry_path:/system/state/current-datetime
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
				configGot := gnmi.GetConfig(t, dut, config.Config())
				if configGot != testCase.tz {
					t.Errorf("Config timezone: got %s, want %s", configGot, testCase.tz)
				}
			})

			t.Run("Get Timezone Telemetry", func(t *testing.T) {
				stateGot := gnmi.Await(t, dut, state.State(), 5*time.Second, testCase.tz)
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
