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

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
)

// TestBootTime verifies the timestamp that the system was last restarted can
// be read and is not an unreasonable value.
//
// telemetry_path:/system/state/boot-time
func testBootTime(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	defer observer.RecordYgot(t, "SUBSCRIBE", gnmi.OC().System().BootTime())
	t.Run("Subscribe//system/state/boot-time", func(t *testing.T) {
		bt := gnmi.Get(t, dut, gnmi.OC().System().BootTime().State())
		if bt < 1640131200000000000 {
			t.Errorf("Unexpected boot timestamp: got %d; check clock", bt)
		}
	})
	// Boot time should be after Dec 22, 2021 00:00:00 GMT in nanoseconds

}
