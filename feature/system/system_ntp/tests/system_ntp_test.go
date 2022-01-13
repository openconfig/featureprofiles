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

	dut := ondatra.DUT(t, "dut1")
	config := dut.Config().System().Ntp()
	state := dut.Telemetry().System().Ntp()

	config.Enabled().Replace(t, true)
	if state.Enabled().Get(t) != true {
		t.Error("NTP Enable Telemetry failed: want true, got false")
	}
	config.Enabled().Delete(t)
}

// TestNtpEnableAuth validates the NTP enable authenication path responds.
//
// config_path:/system/ntp/config/enabled
// config_path:/system/ntp/config/enable-ntp-auth
// telemetry_path:/system/ntp/config/enable-ntp-auth
func TestNtpEnableAuth(t *testing.T) {
	dut := ondatra.DUT(t, "dut1")
	config := dut.Config().System().Ntp()
	state := dut.Telemetry().System().Ntp()

	config.Enabled().Replace(t, true)
	config.EnableNtpAuth().Replace(t, true)

	if state.EnableNtpAuth().Get(t) != true {
		t.Error("NTP Enable Authentication failed: want true, got false")
	}

	config.EnableNtpAuth().Delete(t)
}

// TestNtpSourceAddress validates NTP source addresses can be set.
//
// TODO(bstoll): In order to properly test source address, we need
// to configure an additional IP address and use it to hit a fake
// NTP server.
//
// config_path:/system/ntp/state/ntp-source-address
// telemetry_path:/system/ntp/state/ntp-source-address
func TestNtpSourceAddress(t *testing.T) {
	t.Skip("Need working implementation to validate against")

	dut := ondatra.DUT(t, "dut1")
	config := dut.Config().System().Ntp()
	state := dut.Telemetry().System().Ntp()

	sourceAddress := "192.0.2.1"

	config.NtpSourceAddress().Replace(t, "192.0.2.1")
	if got := state.NtpSourceAddress().Get(t); got != sourceAddress {
		t.Errorf("NTP Source Address mismatch: want %s, got %s", sourceAddress, got)
	}

	config.NtpSourceAddress().Delete(t)
}
