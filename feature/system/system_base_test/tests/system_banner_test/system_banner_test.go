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

package system_banner_test

import (
	"strings"
	"testing"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
)

// TestMotdBanner verifies that the MOTD configuration paths can be read,
// updated, and deleted.
//
// config_path:/system/config/motd-banner
// telemetry_path:/system/state/motd-banner

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestMotdBanner(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	testCases := []struct {
		description string
		banner      string
	}{
		{"Single Character", deviations.BannerDelimiter(dut) + "x" + deviations.BannerDelimiter(dut)},
		{"Short String", deviations.BannerDelimiter(dut) + "Warning Text" + deviations.BannerDelimiter(dut)},
		{"Long String", deviations.BannerDelimiter(dut) + "WARNING : Unauthorized access to this system is forbidden and will be prosecuted by law. By accessing this system, you agree that your actions may be monitored if unauthorized usage is suspected." + deviations.BannerDelimiter(dut)},
	}

	for _, testCase := range testCases {
		t.Run(testCase.description, func(t *testing.T) {
			config := gnmi.OC().System().MotdBanner()
			state := gnmi.OC().System().MotdBanner()

			gnmi.Replace(t, dut, config.Config(), testCase.banner)

			t.Run("Get MOTD Config", func(t *testing.T) {

				configGot := gnmi.Get(t, dut, config.Config())
				configGot = strings.TrimSpace(configGot)
				if configGot != testCase.banner {
					t.Errorf("Config MOTD Banner: got %s, want %s", configGot, testCase.banner)
				}

			})

			t.Run("Get MOTD Telemetry", func(t *testing.T) {
				if testCase.banner == deviations.BannerDelimiter(dut)+""+deviations.BannerDelimiter(dut) {
					stateGot := gnmi.Get(t, dut, state.State())
					stateGot = strings.TrimSpace(stateGot)
					stateGot = deviations.BannerDelimiter(dut) + stateGot + deviations.BannerDelimiter(dut)
					if stateGot != testCase.banner {
						t.Errorf("Telemetry MOTD Banner: got %v, want %s", stateGot, testCase.banner)
					}
				}
			})

			t.Run("Delete MOTD", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				if qs := gnmi.LookupConfig(t, dut, config.Config()); qs.IsPresent() == true {
					t.Errorf("Delete MOTD Banner fail: got %v", qs)
				}
			})
		})
	}
}

// TestLoginBanner verifies that the Login Banner configuration paths can be
// read, updated, and deleted.
//
// config_path:/system/config/login-banner
// telemetry_path:/system/state/login-banner
func TestLoginBanner(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	testCases := []struct {
		description string
		banner      string
	}{
		{"Single Character", deviations.BannerDelimiter(dut) + "x" + deviations.BannerDelimiter(dut)},
		{"Short String", deviations.BannerDelimiter(dut) + "Warning Text" + deviations.BannerDelimiter(dut)},
		{"Long String", deviations.BannerDelimiter(dut) + "WARNING : Unauthorized access to this system is forbidden and will be prosecuted by law. By accessing this system, you agree that your actions may be monitored if unauthorized usage is suspected." + deviations.BannerDelimiter(dut)},
	}

	for _, testCase := range testCases {
		t.Run(testCase.description, func(t *testing.T) {
			config := gnmi.OC().System().LoginBanner()
			state := gnmi.OC().System().LoginBanner()

			gnmi.Replace(t, dut, config.Config(), testCase.banner)

			t.Run("Get Login Banner Config", func(t *testing.T) {
				configGot := gnmi.Get(t, dut, config.Config())
				configGot = strings.TrimSpace(configGot)
				if configGot != testCase.banner {
					t.Errorf("Config Login Banner: got %s, want %s", configGot, testCase.banner)
				}
			})

			t.Run("Get Login Banner Telemetry", func(t *testing.T) {
				if testCase.banner == deviations.BannerDelimiter(dut)+""+deviations.BannerDelimiter(dut) {
					stateGot := gnmi.Get(t, dut, state.State())
					stateGot = strings.TrimSpace(stateGot)
					stateGot = deviations.BannerDelimiter(dut) + stateGot + deviations.BannerDelimiter(dut)
					if stateGot != testCase.banner {
						t.Errorf("Telemetry Login Banner: got %v, want %s", stateGot, testCase.banner)
					}
				}
			})
			t.Run("Delete Login Banner", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				if qs := gnmi.LookupConfig(t, dut, config.Config()); qs.IsPresent() == true {
					t.Errorf("Delete Login Banner fail: got %v", qs)
				}
			})
		})
	}
}
