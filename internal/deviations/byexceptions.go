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

// RequireRoutedSubinterface0 returns true if device needs to configure subinterface 0
// for non-zero sub-interfaces.
func RequireRoutedSubinterface0(*ondatra.DUTDevice) bool {
	return *requireRoutedSubinterface0
}

// SwitchControlProcessorSystemInitiated returns true for devices that report
// last-switchover-reason as SYSTEM_INITIATED for gNOI.SwitchControlProcessor.
func SwitchControlProcessorSystemInitiated(*ondatra.DUTDevice) bool {
	return *switchControlProcessorSystemInitiated
}

var (
	cpuMissingAncestor                    = flag.Bool("deviation_cpu_missing_ancestor", false, "Set to true for devices where the CPU components do not map to a FRU parent component in the OC tree.")
	intfRefConfigUnsupported              = flag.Bool("deviation_intf_ref_config_unsupported", false, "Set to true for devices that do not support interface-ref configuration when applying features to interface.")
	requireRoutedSubinterface0            = flag.Bool("deviation_require_routed_subinterface_0", false, "Set to true for a device that needs subinterface 0 to be routed for non-zero sub-interfaces")
	switchControlProcessorSystemInitiated = flag.Bool("deviation_switch_control_processor_system_initiated", false, "Set to true for devices that report last-switchover-reason as SYSTEM_INITIATED for gNOI.SwitchControlProcessor.")
)
