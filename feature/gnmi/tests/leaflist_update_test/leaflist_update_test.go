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
	"sort"
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
	if len(searchList) != 1 || searchList[0] != "google.com" {
		t.Fatalf("Expected search list to be [\"google.com\"], but got %v", searchList)
	}

	// Update the DNS search list to add "youtube.com".
	gnmi.Update(t, dut, gnmi.OC().System().Dns().Config(), &oc.System_Dns{
		Search: []string{"youtube.com"},
	})

	// Verify the DNS search list now contains both "google.com" and "youtube.com".
	finalSearchList := gnmi.Get[[]string](t, dut, gnmi.OC().System().Dns().Search().State())
	sort.Strings(finalSearchList)
	expectedList := []string{"google.com", "youtube.com"}
	if len(finalSearchList) != len(expectedList) {
		t.Fatalf("Expected search list to be %v, but got %v", expectedList, finalSearchList)
	}
	for i := range finalSearchList {
		if finalSearchList[i] != expectedList[i] {
			t.Fatalf("Expected search list to be %v, but got %v", expectedList, finalSearchList)
		}
	}
}
