// Copyright 2023 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
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

// Vendor deviation flags.
// NOTE: Flags here should be added by exception only.
// All flags should have a corresponding field in the Deviations message in the metadata.proto file.
// If a flag value is set, that will take precedence over the metadata value.
var (
	cpuMissingAncestor                       = flag.Bool("deviation_cpu_missing_ancestor", false, "Set to true for devices where the CPU components do not map to a FRU parent component in the OC tree.")
	interfaceRefConfigUnsupported            = flag.Bool("deviation_interface_ref_config_unsupported", false, "Set to true for devices that do not support interface-ref configuration when applying features to interface.")
	requireRoutedSubinterface0               = flag.Bool("deviation_require_routed_subinterface_0", false, "Set to true for a device that needs subinterface 0 to be routed for non-zero sub-interfaces.")
	gnoiSwitchoverReasonMissingUserInitiated = flag.Bool("deviation_gnoi_switchover_reason_missing_user_initiated", false, "Set to true for devices that don't report last-switchover-reason as USER_INITIATED for gNOI.SwitchControlProcessor.")
	p4rtUnsetElectionIDPrimaryAllowed        = flag.Bool("deviation_p4rt_unsetelectionid_primary_allowed", false, "Device allows unset Election ID to be primary.")
	p4rtBackupArbitrationResponseCode        = flag.Bool("deviation_bkup_arbitration_resp_code", false, "Device sets ALREADY_EXISTS status code for all backup client responses.")
	backupNHGRequiresVrfWithDecap            = flag.Bool("deviation_backup_nhg_requires_vrf_with_decap", false, "Set to true for devices that require IPOverIP Decapsulation for Backup NHG without interfaces.")
	atePortLinkStateOperationsUnsupported    = flag.Bool("deviation_ate_port_link_state_operations_unsupported", false, "Set to true for ATEs that do not support setting link state on their own ports.")
	ateIPv6FlowLabelUnsupported              = flag.Bool("deviation_ate_ipv6_flow_label_unsupported", false, "Set to true for ATEs that do not support IPv6 flow labels")
)

func isFlagSet(name string) bool {
	visited := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			visited = true
		}
	})
	return visited
}

// CPUMissingAncestor deviation set to true for devices where the CPU components
// do not map to a FRU parent component in the OC tree.
func CPUMissingAncestor(dut *ondatra.DUTDevice) bool {
	if isFlagSet("deviation_cpu_missing_ancestor") {
		return *cpuMissingAncestor
	}
	return lookupDUTDeviations(dut).GetCpuMissingAncestor()
}

// InterfaceRefConfigUnsupported deviation set to true for devices that do not support
// interface-ref configuration when applying features to interface.
func InterfaceRefConfigUnsupported(dut *ondatra.DUTDevice) bool {
	if isFlagSet("deviation_interface_ref_config_unsupported") {
		return *interfaceRefConfigUnsupported
	}
	return lookupDUTDeviations(dut).GetInterfaceRefConfigUnsupported()
}

// RequireRoutedSubinterface0 returns true if device needs to configure subinterface 0
// for non-zero sub-interfaces.
func RequireRoutedSubinterface0(dut *ondatra.DUTDevice) bool {
	if isFlagSet("deviation_require_routed_subinterface_0") {
		return *requireRoutedSubinterface0
	}
	return lookupDUTDeviations(dut).GetRequireRoutedSubinterface_0()
}

// GNOISwitchoverReasonMissingUserInitiated returns true for devices that don't
// report last-switchover-reason as USER_INITIATED for gNOI.SwitchControlProcessor.
func GNOISwitchoverReasonMissingUserInitiated(dut *ondatra.DUTDevice) bool {
	if isFlagSet("deviation_gnoi_switchover_reason_missing_user_initiated") {
		return *gnoiSwitchoverReasonMissingUserInitiated
	}
	return lookupDUTDeviations(dut).GetGnoiSwitchoverReasonMissingUserInitiated()
}

// P4rtUnsetElectionIDPrimaryAllowed returns whether the device does not support unset election ID.
func P4rtUnsetElectionIDPrimaryAllowed(dut *ondatra.DUTDevice) bool {
	if isFlagSet("deviation_p4rt_unsetelectionid_primary_allowed") {
		return *p4rtUnsetElectionIDPrimaryAllowed
	}
	return lookupDUTDeviations(dut).GetP4RtUnsetelectionidPrimaryAllowed()
}

// P4rtBackupArbitrationResponseCode returns whether the device does not support unset election ID.
func P4rtBackupArbitrationResponseCode(dut *ondatra.DUTDevice) bool {
	if isFlagSet("deviation_bkup_arbitration_resp_code") {
		return *p4rtBackupArbitrationResponseCode
	}
	return lookupDUTDeviations(dut).GetBkupArbitrationRespCode()
}

// BackupNHGRequiresVrfWithDecap returns true for devices that require
// IPOverIP Decapsulation for Backup NHG without interfaces.
func BackupNHGRequiresVrfWithDecap(dut *ondatra.DUTDevice) bool {
	if isFlagSet("deviation_backup_nhg_requires_vrf_with_decap") {
		return *backupNHGRequiresVrfWithDecap
	}
	return lookupDUTDeviations(dut).GetBackupNhgRequiresVrfWithDecap()
}

// ATEPortLinkStateOperationsUnsupported returns true for traffic generators that do not support
// port link state control operations (such as port shutdown.)
func ATEPortLinkStateOperationsUnsupported(ate *ondatra.ATEDevice) bool {
	if isFlagSet("deviation_ate_port_link_state_operations_unsupported") {
		return *atePortLinkStateOperationsUnsupported
	}
	return lookupATEDeviations(ate).GetAtePortLinkStateOperationsUnsupported()
}

// ATEIPv6FlowLabelUnsupported returns true for traffic generators that do not support
// IPv6 flow labels
func ATEIPv6FlowLabelUnsupported(ate *ondatra.ATEDevice) bool {
	if isFlagSet("deviation_ate_ipv6_flow_label_unsupported") {
		return *ateIPv6FlowLabelUnsupported
	}
	return lookupATEDeviations(ate).GetAteIpv6FlowLabelUnsupported()
}
