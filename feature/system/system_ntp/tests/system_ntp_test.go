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

package system_ntp_test

import (
	"testing"

	"github.com/openconfig/ondatra"
	kinit "github.com/openconfig/ondatra/knebind/init"
)

func TestMain(m *testing.M) {
	ondatra.RunTests(m, kinit.Init)
}

// TestNtpEnable validates the NTP enable path does not return an error.
//
// config_path:/system/ntp/config/enabled
// telemetry_path:/system/ntp/config/enabled
func TestNtpEnable(t *testing.T) {
	t.Skip("Need working implementation to validate against")

	dut := ondatra.DUT(t, "dut")
	config := dut.Config().System().Ntp()
	state := dut.Telemetry().System().Ntp()

	config.Enabled().Replace(t, true)
	if state.Enabled().Get(t) != true {
		t.Error("NTP Enable Telemetry failed: want true, got false")
	}
	config.Enabled().Delete(t)
}
