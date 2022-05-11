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

// /system/memory/state/reserved
func TestMemoryReserved(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/memory/state/reserved", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			memory_reserved := dut.Telemetry().System().Memory().Reserved().Get(t)
			if memory_reserved == uint64(0) {
				t.Logf("Got correct reserved memory value")
			} else {
				t.Errorf("Unexpected reserved memory value")
			}
		})
	})
}

// /system/memory/state/physical
func TestMemoryPhysical(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/memory/state/physical", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			memory_reserved := dut.Telemetry().System().Memory().Physical().Get(t)
			if memory_reserved > uint64(0) {
				t.Logf("Got correct Physical memory value")
			} else {
				t.Errorf("Unexpected Physical memory value")
			}
		})
	})
}
