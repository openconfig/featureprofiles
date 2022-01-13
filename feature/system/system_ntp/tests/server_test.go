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
	oc "github.com/openconfig/ondatra/telemetry"
)

// TestNtpServerConfigurability tests basic configurability of NTP server paths.
//
// TODO(bstoll): port, version, assocation-type are configurable paths not tested.
//
// config_path:/system/ntp/servers/server/config/address
// config_path:/system/ntp/servers/server/config/iburst
// config_path:/system/ntp/servers/server/config/prefer
// telemetry_path:/system/ntp/servers/server/state/address
// telemetry_path:/system/ntp/servers/server/state/iburst
// telemetry_path:/system/ntp/servers/server/state/prefer
func TestNtpServerConfigurability(t *testing.T) {
	testCases := []struct {
		description string
		address     string
		prefer      bool
		iburst      bool
	}{
		{"IPv4 Basic Server", "192.0.2.1", false, false},
		{"IPv4 Prefer Flag Set", "192.0.2.2", true, false},
		{"IPv4 Iburst Flag Set", "192.0.2.3", false, true},
		{"IPv6 Basic Server", "2001:DB8::1", false, false},
		{"IPv6 Prefer Flag Set", "2001:DB8::2", true, false},
		{"IPv6 Iburst Flag Set", "2001:DB8::3", false, true},
	}

	dut := ondatra.DUT(t, "dut1")
	for _, testCase := range testCases {
		t.Run(testCase.description, func(t *testing.T) {
			config := dut.Config().System().Ntp()
			state := dut.Telemetry().System().Ntp()

			ntpServer := oc.System_Ntp_Server{
				Address: &testCase.address,
				Iburst:  &testCase.iburst,
				Prefer:  &testCase.prefer,
			}
			config.Server(testCase.address).Replace(t, &ntpServer)

			configGot := config.Server(testCase.address).Get(t)
			if address := configGot.GetAddress(); address != testCase.address {
				t.Errorf("Config NTP Server: got %s, want %s", address, testCase.address)
			}

			if iburst := configGot.GetIburst(); iburst != testCase.iburst {
				t.Errorf("Config NTP iburst: got %t, want %t", iburst, testCase.iburst)
			}

			if prefer := configGot.GetPrefer(); prefer != testCase.prefer {
				t.Errorf("Config NTP prefer: got %t, want %t", prefer, testCase.prefer)
			}

			stateGot := state.Server(testCase.address).Get(t)
			if address := stateGot.GetAddress(); address != testCase.address {
				t.Errorf("Telemetry NTP Server: got %v, want %s", address, testCase.address)
			}

			if iburst := stateGot.GetIburst(); iburst != testCase.iburst {
				t.Errorf("Config NTP iburst: got %t, want %t", iburst, testCase.iburst)
			}

			if prefer := stateGot.GetPrefer(); prefer != testCase.prefer {
				t.Errorf("Config NTP prefer: got %t, want %t", prefer, testCase.prefer)
			}

			config.Server(testCase.address).Delete(t)
			if qs := config.Server(testCase.address).Lookup(t); qs.IsPresent() == true {
				t.Errorf("Delete NTP Server fail: got %v", qs)
			}
		})
	}
}

// TestNtpServerTelemetry validates NTP server telemetry data is returned for
// a configured NTP server.
//
// telemetry_path:/system/ntp/servers/server/state/offset
// telemetry_path:/system/ntp/servers/server/state/poll-interval
// telemetry_path:/system/ntp/servers/server/state/root-delay
// telemetry_path:/system/ntp/servers/server/state/root-dispersion
// telemetry_path:/system/ntp/servers/server/state/stratum
func TestNtpServerTelemetry(t *testing.T) {
	t.Skip("Need working implementation to validate against")

	// TODO(bstoll): use a fake NTP server implementation
	testNtpServer := "216.239.35.0"

	dut := ondatra.DUT(t, "dut1")
	config := dut.Config().System().Ntp()
	state := dut.Telemetry().System().Ntp()

	v := oc.System_Ntp_Server{
		Address: &testNtpServer,
	}
	config.Server(testNtpServer).Replace(t, &v)

	ntpServerState := state.Server(testNtpServer).Get(t)

	t.Run("Offset", func(t *testing.T) {
		if ntpServerState.Offset == nil {
			t.Fatal("NTP Server Offset Missing")
		}
	})

	t.Run("Poll Interval", func(t *testing.T) {
		if ntpServerState.PollInterval == nil {
			t.Fatal("NTP Server Poll Interval Missing")
		}

		if *ntpServerState.PollInterval == 0 {
			t.Fatalf("NTP Server Poll Interval Invalid: want >0, got 0")
		}
	})

	t.Run("Root Delay", func(t *testing.T) {
		if ntpServerState.RootDelay == nil {
			t.Fatal("NTP Server Root Delay Missing")
		}
	})

	t.Run("Root Dispersion", func(t *testing.T) {
		if ntpServerState.RootDispersion == nil {
			t.Fatal("NTP Server Root Dispersion Missing")
		}
	})

	t.Run("Stratum", func(t *testing.T) {
		if ntpServerState.Stratum == nil {
			t.Fatal("NTP Server Stratum Missing")
		}

		if *ntpServerState.Stratum < 1 || *ntpServerState.Stratum > 16 {
			t.Fatalf("NTP Server Stratum Mismatch: want 1-16, got %d", *ntpServerState.Stratum)
		}
	})

	config.Server(testNtpServer).Delete(t)
}
