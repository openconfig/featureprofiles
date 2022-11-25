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
	"github.com/openconfig/ondatra/gnmi"
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

	dut := ondatra.DUT(t, "dut")

	for _, testCase := range testCases {
		t.Run(testCase.description, func(t *testing.T) {
			config := gnmi.OC().System().MotdBanner()
			state := gnmi.OC().System().MotdBanner()

			gnmi.Replace(t, dut, config.Config(), testCase.banner)

			t.Run("Get MOTD Config", func(t *testing.T) {
				configGot := gnmi.GetConfig(t, dut, config.Config())
				if configGot != testCase.banner {
					t.Errorf("Config MOTD Banner: got %s, want %s", configGot, testCase.banner)
				}
			})

			t.Run("Get MOTD Telemetry", func(t *testing.T) {
				stateGot := gnmi.GetConfig(t, dut, state.Config())
				if stateGot != testCase.banner {
					t.Errorf("Telemetry MOTD Banner: got %v, want %s", stateGot, testCase.banner)
				}
			})

			t.Run("Delete MOTD", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				if qs := gnmi.LookupConfig(t, dut, config.Config()); qs.IsPresent() == true {
					t.Errorf("Delete MOTD Banner fail: got %v", qs)
				}
			})
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

	dut := ondatra.DUT(t, "dut")

	for _, testCase := range testCases {
		t.Run(testCase.description, func(t *testing.T) {
			config := gnmi.OC().System().LoginBanner()
			state := gnmi.OC().System().LoginBanner()

			gnmi.Replace(t, dut, config.Config(), testCase.banner)

			configGot := gnmi.GetConfig(t, dut, config.Config())
			if configGot != testCase.banner {
				t.Errorf("Config Login Banner: got %s, want %s", configGot, testCase.banner)
			}

			stateGot := gnmi.GetConfig(t, dut, state.Config())
			if stateGot != testCase.banner {
				t.Errorf("Telemetry Login Banner: got %v, want %s", stateGot, testCase.banner)
			}

			gnmi.Delete(t, dut, config.Config())
			if qs := gnmi.LookupConfig(t, dut, config.Config()); qs.IsPresent() == true {
				t.Errorf("Delete Login Banner fail: got %v", qs)
			}
		})
	}
}
