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

import "math"

const trafficDistributionTolerance = 0.02

// distributionRatiosWithinTolerance implements the inclusive, absolute 2%
// distribution threshold. Compare the larger ratio against the smaller ratio
// plus the tolerance: subtracting nearly equal ratios can incorrectly reject
// the boundary (for example, 0.75-0.73 is slightly greater than 0.02).
// Ordering the inputs keeps this comparer symmetric, as required by go-cmp.
func distributionRatiosWithinTolerance(want, got float64) bool {
	if want < 0 || want > 1 || got < 0 || got > 1 {
		return false
	}
	return math.Max(want, got) <= math.Min(want, got)+trafficDistributionTolerance
}
