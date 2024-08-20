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

// /system/memory/state/reserved
func testMemoryReserved(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/memory/state/reserved", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			memoryReserved := gnmi.Get(t, dut, gnmi.OC().System().Memory().Reserved().State())
			if memoryReserved == uint64(0) {
				t.Logf("Got correct reserved memory value")
			} else {
				t.Errorf("Unexpected reserved memory value")
			}
		})
	})
}

// /system/memory/state/physical
func testMemoryPhysical(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/memory/state/physical", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			memoryReserved := gnmi.Get(t, dut, gnmi.OC().System().Memory().Physical().State())
			if memoryReserved > uint64(0) {
				t.Logf("Got correct Physical memory value")
			} else {
				t.Errorf("Unexpected Physical memory value")
			}
		})
	})
}
