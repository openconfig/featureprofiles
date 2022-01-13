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

package system_base_test

import (
	"testing"
	"time"

	"github.com/openconfig/ondatra"
)

// TestHostname verifies that the hostname configuration paths can be read,
// updated, and deleted.
//
// config_path:/system/config/hostname
// telemetry_path:/system/state/hostname
func TestHostname(t *testing.T) {
	testCases := []struct {
		description string
		hostname    string
	}{
		{"15 Letters", "abcdefghijkmnop"},
		{"15 Numbers", "123456789012345"},
		{"Single Character", "x"},
		{"Dash and Underscore", "foo_bar-baz"},
		{"Periods", "test.name.example"},
		{"63 Characters", "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"},
	}

	dut := ondatra.DUT(t, "dut1")

	for _, testCase := range testCases {
		t.Run(testCase.description, func(t *testing.T) {
			config := dut.Config().System().Hostname()
			state := dut.Telemetry().System().Hostname()

			config.Replace(t, testCase.hostname)

			configGot := config.Get(t)
			if configGot != testCase.hostname {
				t.Errorf("Config hostname: got %s, want %s", configGot, testCase.hostname)
			}

			stateGot := state.Await(t, 5*time.Second, testCase.hostname)
			if stateGot.Val(t) != testCase.hostname {
				t.Errorf("Telemetry hostname: got %v, want %s", stateGot, testCase.hostname)
			}

			config.Delete(t)
			if qs := config.Lookup(t); qs.IsPresent() == true {
				t.Errorf("Delete hostname fail: got %v", qs)
			}
		})
	}
}

// TestDomainName verifies that the domainname configuration paths can be read,
// updated, and deleted.
//
// config_path:/system/config/domain-name
// telemetry_path:/system/state/domain-name
func TestDomainName(t *testing.T) {
	testCases := []struct {
		description string
		domainname  string
	}{
		{"15 Letters", "abcdefghijkmnop"},
		{"15 Numbers", "123456789012345"},
		{"Single Character", "x"},
		{"Dash and Underscore", "foo_bar-baz"},
		{"Periods", "test.name.example"},
		{"63 Characters", "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"},
	}

	dut := ondatra.DUT(t, "dut1")

	for _, testCase := range testCases {
		t.Run(testCase.description, func(t *testing.T) {
			config := dut.Config().System().DomainName()
			state := dut.Telemetry().System().DomainName()

			config.Replace(t, testCase.domainname)

			configGot := config.Get(t)
			if configGot != testCase.domainname {
				t.Errorf("Config domainname: got %s, want %s", configGot, testCase.domainname)
			}

			stateGot := state.Await(t, 5*time.Second, testCase.domainname)
			if stateGot.Val(t) != testCase.domainname {
				t.Errorf("Telemetry domainname: got %v, want %s", stateGot, testCase.domainname)
			}

			config.Delete(t)
			if qs := config.Lookup(t); qs.IsPresent() == true {
				t.Errorf("Delete domainname fail: got %v", qs)
			}
		})
	}
}
