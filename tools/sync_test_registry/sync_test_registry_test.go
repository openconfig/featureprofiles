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

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tpb "github.com/openconfig/featureprofiles/proto/testregistry_go_proto"
)

func TestPlanIDRE(t *testing.T) {
	tests := []struct {
		planID string
		want   bool
	}{
		// Valid plan IDs.
		{planID: "RT-1.1", want: true},
		{planID: "MGT-1", want: true},
		{planID: "GNMI-1.10", want: true},
		{planID: "P4RT-3.21", want: true},
		{planID: "CERTZ-4", want: true},
		{planID: "EXAMPLE-0.1", want: true},
		// Lower or mixed case prefixes are not allowed.
		{planID: "gNMI-1.1", want: false},
		{planID: "Certz-1", want: false},
		{planID: "bootz-1.0", want: false},
		// More than one minor version is not allowed.
		{planID: "TE-16.1.2", want: false},
		// Malformed plan IDs.
		{planID: "", want: false},
		{planID: "RT", want: false},
		{planID: "RT-", want: false},
		{planID: "-1.1", want: false},
		{planID: "RT-1.", want: false},
		{planID: "RT-.1", want: false},
		{planID: "RT-1.a", want: false},
		{planID: "RT_1.1", want: false},
		{planID: "RT-1-1", want: false},
		{planID: "RT--1.1", want: false},
		{planID: " RT-1.1", want: false},
		{planID: "RT-1.1 ", want: false},
		{planID: "TestProperties", want: false},
	}
	for _, tc := range tests {
		if got := planIDRE.MatchString(tc.planID); got != tc.want {
			t.Errorf("planIDRE.MatchString(%q) = %v, want %v", tc.planID, got, tc.want)
		}
	}
}

func TestProcessMetadataFile(t *testing.T) {
	tests := []struct {
		name            string
		planID          string
		wantErrContains string
	}{
		{name: "valid_plan_id", planID: "RT-1.1"},
		{name: "valid_plan_id_with_digit_in_prefix", planID: "P4RT-3.21"},
		{name: "empty_plan_id", planID: "", wantErrContains: "plan_id is empty"},
		{name: "lower_case_plan_id", planID: "gNMI-1.1", wantErrContains: "invalid plan_id"},
		{name: "two_minor_versions", planID: "TE-16.1.2", wantErrContains: "invalid plan_id"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "metadata.textproto")
			content := `uuid: "00000000-0000-0000-0000-000000000000"
plan_id: "` + tc.planID + `"
description: "Test description"
`
			if err := os.WriteFile(path, []byte(content), 0644); err != nil {
				t.Fatalf("os.WriteFile(%q) failed: %v", path, err)
			}
			tests := make(map[string]*tpb.Test)
			err := processMetadataFile(path, tests)
			if tc.wantErrContains != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErrContains) {
					t.Fatalf("processMetadataFile(%q) error = %v, want error containing %q", path, err, tc.wantErrContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("processMetadataFile(%q) returned unexpected error: %v", path, err)
			}
			if _, ok := tests[tc.planID]; !ok {
				t.Errorf("processMetadataFile(%q) did not register plan_id %q", path, tc.planID)
			}
		})
	}
}
