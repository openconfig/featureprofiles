// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package transceiver

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestToggleWithCleanup(t *testing.T) {
	tests := []struct {
		name      string
		actions   []bool
		wantCalls []bool
	}{
		{
			name:      "no_calls_skips_cleanup",
			actions:   nil,
			wantCalls: nil,
		},
		{
			name:      "disabled_then_enabled_skips_cleanup",
			actions:   []bool{false, true},
			wantCalls: []bool{false, true},
		},
		{
			name:      "left_disabled_restores_on_cleanup",
			actions:   []bool{false},
			wantCalls: []bool{false, true},
		},
		{
			name:      "multiple_iterations_left_disabled_restores_once",
			actions:   []bool{true, false, true, false},
			wantCalls: []bool{true, false, true, false, true},
		},
		{
			name:      "multiple_iterations_re_enabled_skips_cleanup",
			actions:   []bool{true, false, true, false, true},
			wantCalls: []bool{true, false, true, false, true},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var gotCalls []bool
			t.Run("subtest", func(t *testing.T) {
				toggle := ToggleWithCleanup(t, func(enabled bool) {
					gotCalls = append(gotCalls, enabled)
				})
				for _, action := range tc.actions {
					toggle(action)
				}
			})
			if diff := cmp.Diff(tc.wantCalls, gotCalls); diff != "" {
				t.Errorf("ToggleWithCleanup() calls diff (-want +got):\n%s", diff)
			}
		})
	}
}
