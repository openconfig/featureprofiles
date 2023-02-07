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
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
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
			config := gnmi.OC().System().Ntp()
			state := gnmi.OC().System().Ntp()

			ntpServer := oc.System_Ntp_Server{
				Address: &testCase.address,
			}
			if *deviations.NTPAssociationTypeRequired {
				ntpServer.AssociationType = oc.Server_AssociationType_SERVER
			}
			gnmi.Replace(t, dut, config.Server(testCase.address).Config(), &ntpServer)

			t.Run("Get NTP Server Config", func(t *testing.T) {
				configGot := gnmi.GetConfig(t, dut, config.Server(testCase.address).Config())
				if address := configGot.GetAddress(); address != testCase.address {
					t.Errorf("Config NTP Server: got %s, want %s", address, testCase.address)
				}
			})

			t.Run("Get NTP Server Telemetry", func(t *testing.T) {
				stateGot := gnmi.Get(t, dut, state.Server(testCase.address).State())
				if address := stateGot.GetAddress(); address != testCase.address {
					t.Errorf("Telemetry NTP Server: got %s, want %s", address, testCase.address)
				}
			})

			t.Run("Delete NTP Server", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Server(testCase.address).Config())
				if qs := gnmi.LookupConfig(t, dut, config.Server(testCase.address).Config()); qs.IsPresent() == true {
					t.Errorf("Delete NTP Server fail: got %v", qs)
				}
			})
		})
	}
}
