// Copyright 2026 Google LLC
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

package fpga_status_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"

	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestCiscoFpgaStatus(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	if dut.Vendor() != ondatra.CISCO {
		t.Skip("FPD Status is a Cisco-specific translation, skipping for other vendors.")
	}

	opts := fptest.GetOptsForFunctionalTranslator(t, deviations.FpgaFt(dut))

	// Since Functional Translators cannot process container lookups via GetAll(ComponentAny().State())
	// or LookupAll(ComponentAny().PropertyAny().State()), we must first discover the synthetic
	// FPD names by querying the native path.

	req := &gnmipb.GetRequest{
		Path: []*gnmipb.Path{{
			Origin: "Cisco-IOS-XR-show-fpd-loc-ng-oper",
			Elem: []*gnmipb.PathElem{
				{Name: "show-fpd"}, {Name: "hw-module-fpd"}, {Name: "fpd-info-detail"},
			},
		}},
		Encoding: gnmipb.Encoding_JSON_IETF,
		Type:     gnmipb.GetRequest_STATE,
	}

	rawGNMI := dut.RawAPIs().GNMI(t)
	resp, err := rawGNMI.Get(context.Background(), req)
	if err != nil {
		t.Fatalf("Failed to fetch native Cisco FPD paths: %v", err)
	}

	var fpdNames []string

	for _, notif := range resp.GetNotification() {
		for _, update := range notif.GetUpdate() {
			val := update.GetVal().GetJsonIetfVal()
			if len(val) > 0 {
				var fpdWrap map[string]any
				if err := json.Unmarshal(val, &fpdWrap); err == nil {
					loc, ok1 := fpdWrap["location"].(string)
					name, ok2 := fpdWrap["fpd-name"].(string)
					if ok1 && ok2 && loc != "" && name != "" {
						fpdNames = append(fpdNames, fmt.Sprintf("%s_%s", loc, name))
					}
				}
			}
		}
	}

	// Deduplicate the slice
	fpdMap := make(map[string]bool)
	for _, name := range fpdNames {
		fpdMap[name] = true
	}

	validStatuses := map[string]bool{
		"CURRENT": true, "NEED UPGD": true, "RLOAD REQ": true,
		"NOT READY": true, "UPGD DONE": true, "UPGD FAIL": true,
		"BACK IMG": true, "N/A": true,
	}

	for compName := range fpdMap {
		t.Logf("Checking component: %s", compName)
		prop := gnmi.Lookup(t, dut.GNMIOpts().WithYGNMIOpts(opts...), gnmi.OC().Component(compName).Property("fpd-status").Value().State())

		val, ok := prop.Val()
		if ok {
			var statusVal string
			if v, isString := val.(oc.UnionString); isString {
				statusVal = string(v)
			} else {
				t.Errorf("Found FPD Status for %s, but value was not a string: %v", compName, val)
				continue
			}
			if !validStatuses[statusVal] {
				t.Errorf("Component %s reported an unknown FPD status via translator: %q", compName, statusVal)
			}
		}
	}
}
