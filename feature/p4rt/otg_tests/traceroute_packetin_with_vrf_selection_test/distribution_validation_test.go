// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package traceroute_packetin_with_vrf_selection_test

import (
	"math"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestDistributionRatiosWithinTolerance(t *testing.T) {
	for _, tc := range []struct {
		name      string
		want, got float64
		equal     bool
	}{
		{"exact", 0.75, 0.75, true},
		{"730_packets_lower_boundary", 0.75, 730.0 / 1000, true},
		{"770_packets_upper_boundary", 0.75, 770.0 / 1000, true},
		{"731_packets_inside", 0.75, 731.0 / 1000, true},
		{"769_packets_inside", 0.75, 769.0 / 1000, true},
		{"729_packets_below_boundary", 0.75, 729.0 / 1000, false},
		{"771_packets_above_boundary", 0.75, 771.0 / 1000, false},
		{"773_packets_outside_tolerance", 0.75, 773.0 / 1000, false},
		{"next_float_below_lower_boundary", 0.75, math.Nextafter(0.73, 0), false},
		{"next_float_above_upper_boundary", 0.75, math.Nextafter(0.77, 1), false},
		{"2000_packets_lower_boundary", 0.75, 1460.0 / 2000, true},
		{"2000_packets_upper_boundary", 0.75, 1540.0 / 2000, true},
		{"2000_packets_below_boundary", 0.75, 1459.0 / 2000, false},
		{"2000_packets_above_boundary", 0.75, 1541.0 / 2000, false},
		{"zero_weight_boundary", 0, 0.02, true},
		{"zero_weight_outside", 0, 0.021, false},
		{"unit_weight_boundary", 1, 0.98, true},
		{"unit_weight_outside", 1, 0.979, false},
		{"negative_ratio", 0, -0.001, false},
		{"ratio_above_one", 1, 1.001, false},
		{"nan", 0.75, math.NaN(), false},
		{"positive_infinity", 0.75, math.Inf(1), false},
		{"negative_infinity", 0.75, math.Inf(-1), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, pair := range [][2]float64{{tc.want, tc.got}, {tc.got, tc.want}} {
				if got := cmp.Equal(pair[0], pair[1], cmp.Comparer(distributionRatiosWithinTolerance)); got != tc.equal {
					t.Errorf("Comparing %v and %v: equal = %v, want %v", pair[0], pair[1], got, tc.equal)
				}
			}
		})
	}
}

func TestTrafficDistributionRegression(t *testing.T) {
	want := []float64{0.0156, 0.0468, 0.1875, 0, 0.75, 0, 0}
	for _, tc := range []struct {
		name  string
		got   []float64
		equal bool
	}{
		{"distribution_at_boundary", []float64{0.022, 0.046, 0.202, 0, 0.73, 0, 0}, true},
		{"distribution_outside_tolerance", []float64{0.015, 0.042, 0.17, 0, 0.773, 0, 0}, false},
		{"distribution_inside_tolerance", []float64{0.017, 0.045, 0.188, 0, 0.75, 0, 0}, true},
		{"missing_bucket", []float64{0.0156, 0.0468, 0.1875, 0, 0.75, 0}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if diff := cmp.Diff(want, tc.got, cmp.Comparer(distributionRatiosWithinTolerance)); (diff == "") != tc.equal {
				t.Errorf("Distribution equal = %v, want %v; diff:\n%s", diff == "", tc.equal, diff)
			}
		})
	}
}
