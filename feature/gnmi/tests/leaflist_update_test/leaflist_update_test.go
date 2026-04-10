// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package leaflist_update_test

import (
	"testing"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestLeafListUpdate(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	// Configure the DNS search list to ["google.com"] using Replace.
	dnsConfig := &oc.System_Dns{}
	dnsConfig.Search = []string{"google.com"}
	gnmi.Replace(t, dut, gnmi.OC().System().Dns().Config(), dnsConfig)

	// Verify the DNS search list is ["google.com"].
	searchList := gnmi.Get[[]string](t, dut, gnmi.OC().System().Dns().Search().State())
	found := false
	for _, s := range searchList {
		if s == "google.com" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("Expected search list to contain \"google.com\", but got %v", searchList)
	}

	// Update the DNS search list to add "youtube.com".
	gnmi.Update(t, dut, gnmi.OC().System().Dns().Config(), &oc.System_Dns{
		Search: []string{"youtube.com"},
	})

	// Verify the DNS search list now contains both "google.com" and "youtube.com".
	finalSearchList := gnmi.Get[[]string](t, dut, gnmi.OC().System().Dns().Search().State())
	entryFound := true
	for _, want := range []string{"google.com", "youtube.com"} {
		found := false
		for _, s := range finalSearchList {
			if s == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected search list to contain %q, but got %v", want, finalSearchList)
			entryFound = false
		}
	}
	if !entryFound {
		t.Fatalf("Final search list does not contain all expected entries: got %v, want %v", finalSearchList, []string{"google.com", "youtube.com"})
	}
}
