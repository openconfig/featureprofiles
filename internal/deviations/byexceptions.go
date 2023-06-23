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

package deviations

import (
	"flag"

	"github.com/openconfig/ondatra"
)

// CPUMissingAncestor deviation set to true for devices where the CPU components
// do not map to a FRU parent component in the OC tree.
func CPUMissingAncestor(*ondatra.DUTDevice) bool {
	return *cpuMissingAncestor
}

var (
	cpuMissingAncestor = flag.Bool("cpu_missing_ancestor", false, "Set to true for devices where the CPU components do not map to a FRU parent component in the OC tree.")
)
