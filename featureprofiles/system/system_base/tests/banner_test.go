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

	"github.com/openconfig/ondatra"
)

// TestMotdBanner verifies that the MOTD configuration paths can be read,
// updated, and deleted.
//
// config_path:/system/config/motd-banner
// telemetry_path:/system/state/motd-banner
func TestMotdBanner(t *testing.T) {
	t.Skip("Need working implementation to validate against")

	testCases := []struct {
		description string
		banner      string
	}{
		{"Empty String", ""},
		{"Single Character", "x"},
		{"Short String", "Warning Text"},
		{"Long String", "WARNING : Unauthorized access to this system is forbidden and will be prosecuted by law. By accessing this system, you agree that your actions may be monitored if unauthorized usage is suspected."},
	}

	dut := ondatra.DUT(t, "dut1")

	for _, testCase := range testCases {
		t.Run(testCase.description, func(t *testing.T) {
			config := dut.Config().System().MotdBanner()
			state := dut.Config().System().MotdBanner()

			config.Replace(t, testCase.banner)

			configGot := config.Get(t)
			if configGot != testCase.banner {
				t.Errorf("Config MOTD Banner: got %s, want %s", configGot, testCase.banner)
			}

			stateGot := state.Get(t)
			if stateGot != testCase.banner {
				t.Errorf("Telemetry MOTD Banner: got %v, want %s", stateGot, testCase.banner)
			}

			config.Delete(t)
			if qs := config.Lookup(t); qs.IsPresent() == true {
				t.Errorf("Delete MOTD Banner fail: got %v", qs)
			}
		})
	}
}

// TestLoginBanner verifies that the Login Banner configuration paths can be
// read, updated, and deleted.
//
// config_path:/system/config/login-banner
// telemetry_path:/system/state/login-banner
func TestLoginBanner(t *testing.T) {
	t.Skip("Need working implementation to validate against")

	testCases := []struct {
		description string
		banner      string
	}{
		{"Empty String", ""},
		{"Single Character", "x"},
		{"Short String", "Warning Text"},
		{"Long String", "WARNING : Unauthorized access to this system is forbidden and will be prosecuted by law. By accessing this system, you agree that your actions may be monitored if unauthorized usage is suspected."},
	}

	dut := ondatra.DUT(t, "dut1")

	for _, testCase := range testCases {
		t.Run(testCase.description, func(t *testing.T) {
			config := dut.Config().System().LoginBanner()
			state := dut.Config().System().LoginBanner()

			config.Replace(t, testCase.banner)

			configGot := config.Get(t)
			if configGot != testCase.banner {
				t.Errorf("Config Login Banner: got %s, want %s", configGot, testCase.banner)
			}

			stateGot := state.Get(t)
			if stateGot != testCase.banner {
				t.Errorf("Telemetry Login Banner: got %v, want %s", stateGot, testCase.banner)
			}

			config.Delete(t)
			if qs := config.Lookup(t); qs.IsPresent() == true {
				t.Errorf("Delete Login Banner fail: got %v", qs)
			}
		})
	}
}
