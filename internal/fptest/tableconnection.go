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

package fptest

import (
	"context"
	"encoding/json"
	"testing"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
)

// ConfigEnableTbNative enables admin-state of table-connections in native mode.
func ConfigEnableTbNative(t testing.TB, d *ondatra.DUTDevice) {
	t.Helper()
	state := "enable"
	switch d.Vendor() {
	case ondatra.NOKIA:
		adminEnable, err := json.Marshal(state)
		if err != nil {
			t.Fatalf("Error with json Marshal: %v", err)
		}

		gpbSetRequest := &gpb.SetRequest{
			Prefix: &gpb.Path{
				Origin: "native",
			},
			Update: []*gpb.Update{
				{
					Path: &gpb.Path{
						Elem: []*gpb.PathElem{
							{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
							{Name: "table-connections"},
							{Name: "admin-state"},
						},
					},
					Val: &gpb.TypedValue{
						Value: &gpb.TypedValue_JsonIetfVal{
							JsonIetfVal: adminEnable,
						},
					},
				},
			},
		}

		gnmiClient := d.RawAPIs().GNMI(t)
		if _, err := gnmiClient.Set(context.Background(), gpbSetRequest); err != nil {
			t.Fatalf("Unexpected error updating SRL static-route tag-set: %v", err)
		}
	default:
		t.Fatalf("Unsupported vendor %s for deviation 'EnableTableConnections'", d.Vendor())
	}
}
