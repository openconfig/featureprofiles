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

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/telemetry"
)

// TestNtpServerConfigurability tests basic configurability of NTP server paths.
//
// TODO(bstoll): port is a configurable path not tested here.
func TestNtpServerConfigurability(t *testing.T) {
	testCases := []struct {
		description string
		address     string
	}{
		{"IPv4 Basic Server", "192.0.2.1"},
		{"IPv6 Basic Server (RFC5952)", "2001:db8::1"},
	}

	dut := ondatra.DUT(t, "dut")
	for _, testCase := range testCases {
		t.Run(testCase.description, func(t *testing.T) {
			config := dut.Config().System().Ntp()
			state := dut.Telemetry().System().Ntp()

			ntpServer := telemetry.System_Ntp_Server{
				Address: &testCase.address,
			}
			if *deviations.NtpAssociationType {
				ntpServer.AssociationType = telemetry.Server_AssociationType_SERVER
			}
			config.Server(testCase.address).Replace(t, &ntpServer)

			t.Run("Get NTP Server Config", func(t *testing.T) {
				configGot := config.Server(testCase.address).Get(t)
				if address := configGot.GetAddress(); address != testCase.address {
					t.Errorf("Config NTP Server: got %s, want %s", address, testCase.address)
				}
			})

			t.Run("Get NTP Server Telemetry", func(t *testing.T) {
				stateGot := state.Server(testCase.address).Get(t)
				if address := stateGot.GetAddress(); address != testCase.address {
					t.Errorf("Telemetry NTP Server: got %s, want %s", address, testCase.address)
				}
			})

			t.Run("Delete NTP Server", func(t *testing.T) {
				config.Server(testCase.address).Delete(t)
				if qs := config.Server(testCase.address).Lookup(t); qs.IsPresent() == true {
					t.Errorf("Delete NTP Server fail: got %v", qs)
				}
			})
		})
	}
}
