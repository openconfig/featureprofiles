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
)

// TestHostname verifies that the hostname configuration paths can be read,
// updated, and deleted.
//
// config_path:/system/config/hostname
// telemetry_path:/system/state/hostname
func TestHostname(t *testing.T) {
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
			config := dut.Config().System().Hostname()
			state := dut.Telemetry().System().Hostname()
			t.Run("Replace//system/config/hostname", func(t *testing.T) {
				defer observer.RecordYgot(t, "REPLACE", config)
				config.Replace(t, testCase.hostname)
			})

			t.Run("Subscribe//system/state/hostname", func(t *testing.T) {
				defer observer.RecordYgot(t, "SUBSCRIBE", state)
				stateGot := state.Await(t, 35*time.Second, testCase.hostname)
				time.Sleep(35 * time.Second)
				if stateGot.Val(t) != testCase.hostname {
					t.Errorf("Telemetry hostname: got %v, want %s", stateGot, testCase.hostname)
				}
			})

			t.Run("Update//system/config/hostname", func(t *testing.T) {
				defer observer.RecordYgot(t, "UPDATE", config)
				config.Update(t, testCase.hostname+"New")
			})

			t.Run("Delete//system/config/hostname", func(t *testing.T) {
				defer observer.RecordYgot(t, "DELETE", config)
				config.Delete(t)
			})
		})
	}
}
