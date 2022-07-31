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

package main

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestSetLabels(t *testing.T) {
	tests := []struct {
		desc    string
		in      string
		want    labels
		wantErr bool
	}{{
		desc: "single label",
		in:   "1",
		want: labels{1},
	}, {
		desc: "multiple labels",
		in:   "1,2,3,4,5",
		want: labels{1, 2, 3, 4, 5},
	}, {
		desc:    "invalid label",
		in:      "fish, chips",
		wantErr: true,
	}, {
		desc:    "invalid uint32",
		in:      "4294967299",
		wantErr: true,
	}}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			var l labels
			if err := l.Set(tt.in); (err != nil) != tt.wantErr {
				t.Fatalf("did not get expected error, got: %v, wantErr? %v", err, tt.wantErr)
			}
			if diff := cmp.Diff(tt.want, l); diff != "" {
				t.Fatalf("did not get expected labels, diff(-got,+want):\n%s", diff)
			}
		})
	}
}
