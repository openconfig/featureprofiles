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

// IntfRefConfigUnsupported deviation set to true for devices that do not support
// interface-ref configuration when applying features to interface.
func IntfRefConfigUnsupported(*ondatra.DUTDevice) bool {
	return *intfRefConfigUnsupported
}

var (
	cpuMissingAncestor       = flag.Bool("deviation_cpu_missing_ancestor", false, "Set to true for devices where the CPU components do not map to a FRU parent component in the OC tree.")
	intfRefConfigUnsupported = flag.Bool("deviation_intf_ref_config_unsupported", false, "Set to true for devices that do not support interface-ref configuration when applying features to interface.")
)
