// Copyright 2023 Google LLC
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
	"strings"
	"testing"
)

func TestSVGBadge(t *testing.T) {
	cases := []struct {
		name    string
		label   string
		message string
		want    string
	}{
		{
			name:  "Label exists in output",
			label: "label text",
			want:  "label text",
		},
		{
			name:    "Message exists in output",
			message: "message text",
			want:    "message text",
		},
		{
			name:    "Success message uses color #4C1",
			label:   "label text",
			message: "success",
			want:    "#4C1",
		},
		{
			name:    "Failure message uses color #E05D44",
			label:   "label text",
			message: "failure",
			want:    "#E05D44",
		},
		{
			name:    "Message default color is #9F9F9F",
			label:   "label text",
			message: "other status",
			want:    "#9F9F9F",
		},
		{
			name: "Empty inputs",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			badge, err := svgBadge(tc.label, tc.message)
			if err != nil {
				t.Errorf("svgBadge(%q, %q) got error %v, want nil", tc.label, tc.message, err)
			}

			if !strings.Contains(badge.String(), tc.want) {
				t.Errorf("svgBadge(%q, %q) missing %q in result: %s", tc.label, tc.message, tc.want, badge)
			}
		})
	}
}

func TestEstimateStringWidth(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want int
	}{
		{
			name: "Empty String",
			in:   "",
			want: 10,
		},
		{
			name: "Standard String",
			in:   "Standard String",
			want: 115,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := estimateStringWidth(tc.in)
			if tc.want != got {
				t.Errorf("estimateStringWidth(%q) got %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}
