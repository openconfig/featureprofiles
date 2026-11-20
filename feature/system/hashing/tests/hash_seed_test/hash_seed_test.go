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

package hash_seed_test

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
)

const policy = "GLOBAL"

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// openconfig-hashing is not in the ygnmi structs this module vendors, so the
// seed leaf is read and written as a JSON-IETF scalar. The binding rewrite
// wraps that scalar when restconf_json is set.
func seedPath() *gpb.Path {
	return &gpb.Path{
		Origin: "openconfig",
		Elem: []*gpb.PathElem{
			{Name: "system"},
			{Name: "hashing"},
			{Name: "hashing-policies"},
			{Name: "hashing-policy", Key: map[string]string{"name": policy}},
			{Name: "config"},
			{Name: "seed"},
		},
	}
}

func seedValue(t *testing.T, client gpb.GNMIClient) (uint64, bool) {
	t.Helper()
	resp, err := client.Get(context.Background(), &gpb.GetRequest{
		Path:     []*gpb.Path{seedPath()},
		Type:     gpb.GetRequest_ALL,
		Encoding: gpb.Encoding_JSON_IETF,
	})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return 0, false
		}
		t.Fatalf("get hash seed: %v", err)
	}
	for _, note := range resp.GetNotification() {
		for _, up := range note.GetUpdate() {
			raw := up.GetVal().GetJsonIetfVal()
			if len(raw) == 0 {
				continue
			}
			var v any
			if err := json.Unmarshal(raw, &v); err != nil {
				t.Fatalf("hash seed JSON: %v", err)
			}
			return asUint(t, v), true
		}
	}
	return 0, false
}

func asUint(t *testing.T, v any) uint64 {
	t.Helper()
	switch n := v.(type) {
	case float64:
		return uint64(n)
	case string:
		u, err := strconv.ParseUint(n, 10, 64)
		if err != nil {
			t.Fatalf("hash seed %q: %v", n, err)
		}
		return u
	case map[string]any:
		for _, child := range n {
			return asUint(t, child)
		}
	}
	t.Fatalf("hash seed has type %T", v)
	return 0
}

func setSeed(t *testing.T, client gpb.GNMIClient, seed uint64) {
	t.Helper()
	js, err := json.Marshal(seed)
	if err != nil {
		t.Fatalf("marshal seed: %v", err)
	}
	_, err = client.Set(context.Background(), &gpb.SetRequest{
		Replace: []*gpb.Update{{
			Path: seedPath(),
			Val:  &gpb.TypedValue{Value: &gpb.TypedValue_JsonIetfVal{JsonIetfVal: js}},
		}},
	})
	if err != nil {
		t.Fatalf("set hash seed %d: %v", seed, err)
	}
}

// TestHashSeedRoundTrip covers the hashing-policy seed leaf.
func TestHashSeedRoundTrip(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	client := dut.RawAPIs().GNMI(t)
	original, present := seedValue(t, client)
	t.Cleanup(func() {
		if present {
			setSeed(t, client, original)
			return
		}
		if _, err := client.Set(context.Background(), &gpb.SetRequest{Delete: []*gpb.Path{seedPath()}}); err != nil {
			t.Errorf("delete hash seed: %v", err)
		}
	})

	want := uint64(1)
	if present && original == want {
		want = 2
	}
	t.Logf("Replace the %s hashing-policy seed", policy)
	setSeed(t, client, want)
	got, ok := seedValue(t, client)
	if !ok || got != want {
		t.Errorf("hash seed = %d present %v, want %d", got, ok, want)
	}
}
