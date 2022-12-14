// Copyright 2022 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package system_ntp_test

import (
	"testing"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// TestNtpEnable validates the NTP enable path does not return an error.
func TestNtpEnable(t *testing.T) {
	t.Skip("Need working implementation to validate against")

	dut := ondatra.DUT(t, "dut")
	config := gnmi.OC().System().Ntp()
	state := gnmi.OC().System().Ntp()

	gnmi.Replace(t, dut, config.Enabled().Config(), true)
	if gnmi.Get(t, dut, state.Enabled().State()) != true {
		t.Error("NTP Enable Telemetry failed: want true, got false")
	}
	gnmi.Delete(t, dut, config.Enabled().Config())
}
