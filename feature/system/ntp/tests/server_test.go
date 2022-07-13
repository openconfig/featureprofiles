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
	"net"
	"testing"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/testt"
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
		{"IPv6 Basic Server", "2001:DB8::1"},
		{"IPv6 RFC5952", "2001:db8::2"},
	}

	dut := ondatra.DUT(t, "dut")
	for _, testCase := range testCases {
		t.Run(testCase.description, func(t *testing.T) {
			config := dut.Config().System().Ntp()
			state := dut.Telemetry().System().Ntp()

			ntpServer := telemetry.System_Ntp_Server{
				Address: &testCase.address,
			}
			config.Server(testCase.address).Replace(t, &ntpServer)

			t.Run("Get NTP Server Config", func(t *testing.T) {
				wantAddress := net.ParseIP(testCase.address)
				address := config.Server(testCase.address).Address().Lookup(t)
				if address.IsPresent() == false {
					address = config.Server(wantAddress.String()).Address().Lookup(t)
					if address.IsPresent() == false {
						t.Errorf("Config NTP Server Failed Lookup: %v and %v", testCase.address, wantAddress.String())
					}
				}
				if gotAddress := net.ParseIP(address.Val(t)); gotAddress == nil || !wantAddress.Equal(gotAddress) {
					t.Errorf("Config NTP Server Address: got %v, want %v", gotAddress, wantAddress)
				}
			})

			t.Run("Get NTP Server Telemetry", func(t *testing.T) {
				wantAddress := net.ParseIP(testCase.address)
				address := state.Server(testCase.address).Address().Lookup(t)
				if address.IsPresent() == false {
					address = state.Server(wantAddress.String()).Address().Lookup(t)
					if address.IsPresent() == false {
						t.Errorf("Telemetry NTP Server Failed Lookup: %v and %v", testCase.address, wantAddress.String())
					}
				}
				if gotAddress := net.ParseIP(address.Val(t)); gotAddress == nil || !wantAddress.Equal(gotAddress) {
					t.Errorf("Telemetry NTP Server Address: got %v, want %v", gotAddress, wantAddress)
				}
			})

			t.Run("Delete NTP Server", func(t *testing.T) {
				address := net.ParseIP(testCase.address)
				testt.CaptureFatal(t, func(t testing.TB) {
					config.Server(address.String()).Delete(t)
				})
				testt.CaptureFatal(t, func(t testing.TB) {
					config.Server(testCase.address).Delete(t)
				})

				if qs := config.Server(address.String()).Lookup(t); qs.IsPresent() == true {
					t.Errorf("Delete NTP Server fail: got %v", qs)
				}
				if qs := config.Server(testCase.address).Lookup(t); qs.IsPresent() == true {
					t.Errorf("Delete NTP Server fail: got %v", qs)
				}
			})
		})
	}
}
