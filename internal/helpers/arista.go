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

package helpers

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
)

const (
	coppCounterPrefix = "arista-platform-control-plane-traffic-counters:"
)

// AristaPlatform detects the Arista ASIC platform type (e.g. "sand" or "strata")
// by issuing a gNMI Get to the control-plane-traffic vendor/arista path.
func AristaPlatform(t *testing.T, dut *ondatra.DUTDevice) string {
	t.Helper()
	gnmiClient := dut.RawAPIs().GNMI(t)
	resp, err := gnmiClient.Get(context.Background(), &gpb.GetRequest{
		Path: []*gpb.Path{{
			Elem: []*gpb.PathElem{
				{Name: "components"},
				{Name: "component"},
				{Name: "integrated-circuit"},
				{Name: "pipeline-counters"},
				{Name: "control-plane-traffic"},
				{Name: "vendor"},
				{Name: "arista"},
			},
		}},
		Type:     gpb.GetRequest_STATE,
		Encoding: gpb.Encoding_JSON_IETF,
	})
	if err != nil {
		t.Fatalf("AristaPlatform: gNMI Get failed: %v", err)
	}
	platform := extractPlatformFromResponse(t, resp)
	t.Logf("AristaPlatform: detected %q platform from OC path", platform)
	return platform
}

func extractPlatformFromResponse(t *testing.T, resp *gpb.GetResponse) string {
	t.Helper()
	notifications := resp.GetNotification()
	if len(notifications) == 0 || len(notifications[0].GetUpdate()) == 0 {
		t.Fatal("extractPlatformFromResponse: empty gNMI response")
	}
	jsonData := notifications[0].GetUpdate()[0].GetVal().GetJsonIetfVal()
	var data map[string]interface{}
	if err := json.Unmarshal(jsonData, &data); err != nil {
		t.Fatalf("extractPlatformFromResponse: failed to unmarshal JSON: %v", err)
	}
	for key := range data {
		if strings.HasPrefix(key, coppCounterPrefix) {
			return strings.TrimPrefix(key, coppCounterPrefix)
		}
	}
	t.Fatalf("extractPlatformFromResponse: no platform key with prefix %q found in response", coppCounterPrefix)
	return ""
}
