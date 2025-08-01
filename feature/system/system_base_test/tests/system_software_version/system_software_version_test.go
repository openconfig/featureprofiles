/*
 Copyright 2025 Google LLC
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

package system_software_version_test

import (
	"testing"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// TestSoftwareVersion verifies that the software version state path can be read and is not empty.
// telemetry_path:/system/state/software-version
func TestSoftwareVersion(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	if got := gnmi.Get(t, dut, gnmi.OC().System().SoftwareVersion().State()); got == "" {
		t.Error("Telemetry software version is empty, want non-empty")
	}
}
