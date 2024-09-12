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

package basetest

import (
	"testing"
	"time"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
)

// TestHostname verifies that the hostname configuration paths can be read,
// updated, and deleted.
//
// config_path:/system/config/hostname
// telemetry_path:/system/state/hostname
func testHostname(t *testing.T) {
	testCases := []struct {
		description string
		hostname    string
	}{
		{"15 Letters", "abcdefghijkmnop"},
		{"15 Numbers", "123456789012345"},
		{"Single Character", "x"},
		{"Periods", "test.name.example"},
		{"63 Characters", "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"},
	}

	dut := ondatra.DUT(t, device1)

	for _, testCase := range testCases {
		t.Run(testCase.description, func(t *testing.T) {
			config := gnmi.OC().System().Hostname()
			state := gnmi.OC().System().Hostname()
			t.Run("Replace//system/config/hostname", func(t *testing.T) {
				defer observer.RecordYgot(t, "REPLACE", config)
				gnmi.Replace(t, dut, config.Config(), testCase.hostname)
			})

			t.Run("Subscribe//system/state/hostname", func(t *testing.T) {
				defer observer.RecordYgot(t, "SUBSCRIBE", state)
				stateGot := gnmi.Await(t, dut, state.State(), 35*time.Second, testCase.hostname)
				time.Sleep(35 * time.Second)
				value, _ := stateGot.Val()
				if value != testCase.hostname {
					t.Errorf("Telemetry hostname: got %v, want %s", stateGot, testCase.hostname)
				}
			})

			t.Run("Update//system/config/hostname", func(t *testing.T) {
				defer observer.RecordYgot(t, "UPDATE", config)
				gnmi.Update(t, dut, config.Config(), testCase.hostname+"New")
			})

			t.Run("Delete//system/config/hostname", func(t *testing.T) {
				defer observer.RecordYgot(t, "DELETE", config)
				gnmi.Delete(t, dut, config.Config())
			})
		})
	}
}
