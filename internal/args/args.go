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

// Package args define arguments for testing that depend on the available components
// and their naming on the device, if they cannot be enumerated easily from /components by type.
// Having these arguments at the project level help us run the whole suite of tests
// without defining them per test.
package args

import (
	"flag"
)

// Global test flags.
var (
	NumControllerCards            = flag.Int("arg_num_controller_cards", -1, "The expected number of controller cards. Some devices with a single controller report 0, which is a valid expected value. Expectation is not checked for values < 0.")
	NumLinecards                  = flag.Int("arg_num_linecards", -1, "The expected number of linecards. Some devices with a single linecard report 0, which is a valid expected value. Expectation is not checked for values < 0.")
	NumFabrics                    = flag.Int("arg_num_fabrics", -1, "The expected number of fabrics. Some devices with a single fabric report 0, which is a valid expected value. Expectation is not checked for values < 0.")
	NumFans                       = flag.Int("arg_num_fans", 0, "The expected number of fans (default is 0, meaning the device is not expected to have fans so none are validated).")
	NumFanTrays                   = flag.Int("arg_num_fan_trays", 0, "The expected number of fan trays (default is 0, meaning the device is not expected to have fan trays so none are validated).")
	FullConfigReplaceTime         = flag.Duration("arg_full_config_replace_time", 0, "Time taken for gNMI set operation to complete full configuration replace. Expected duration is in nanoseconds. Expectation is not checked when value is 0.")
	SubsetConfigReplaceTime       = flag.Duration("arg_subset_config_replace_time", 0, "Time taken for gNMI set operation to modify a subset of configuration. Expected duration is in nanoseconds. Expectation is not checked when value is 0.")
	QoSBaseConfigPresent          = flag.Bool("arg_qos_baseconfig_present", true, "QoS Counter subtest in gNMI-1.10 requires related base config to be loaded. Use this flag to skip the when base config is not loaded.")
	LACPBaseConfigPresent         = flag.Bool("arg_lacp_baseconfig_present", true, "LACP subtest in gNMI-1.10 requires related base config to be loaded. Use this flag to skip the test when base config is not loaded.")
	TempSensorNamePattern         = flag.String("arg_temp_sensor_name_pattern", "", "There is no component type specifically for temperature sensors. So, we use the name pattern to find them.")
	SwitchChipNamePattern         = flag.String("arg_switchchip_name_pattern", "", "There is no component type specifically for SwitchChip components. So, we use the name pattern to find them.")
	FanNamePattern                = flag.String("arg_fan_name_pattern", "", "This name pattern is used to filter out Fan components.")
	FabricChipNamePattern         = flag.String("arg_fabricChip_name_pattern", "", "This name pattern is used to filter out FabricChip components.")
	CheckInterfacesInBinding      = flag.Bool("arg_check_interfaces_in_binding", true, "GNOI tests perform interface status validation based on all interfaces. This can cause flakiness in testing environments where only connectivity of interfaces in binding is guaranteed.")
	ConvergencePathChange         = flag.Uint64("arg_convergence_path_change", 250, "Traffic loss expected during path change set as 250 ms")
	DefaultVRFIPv4Count           = flag.Int("arg_default_vrf_ipv4_count", 1064, "In gRIBI scaling tests, the number of IPv4 entries to install in default network instance for recursive lookup")
	DefaultVRFIPv4NHSize          = flag.Int("arg_default_vrf_ipv4_nh_size", 8, "In gRIBI scaling tests, the number of next-hops in each next-hop-group installed in default network instance")
	DefaultVRFIPv4NHGWeightSum    = flag.Int("arg_default_vrf_ipv4_nhg_weight_sum", 64, "In gRIBI scaling tests, the sum of weights to assign to next-hops within a next-hop-group in the default network instance")
	DefaultVRFIPv4NHCount         = flag.Int("arg_default_vrf_ipv4_nh_count", 16, "In gRIBI scaling tests, the number of next-hops to install in default network instance")
	NonDefaultVRFIPv4Count        = flag.Int("arg_non_default_vrf_ipv4_count", 32000, "In gRIBI scaling tests, the number of IPv4 entries to install in non-default VRF")
	NonDefaultVRFIPv4NHGCount     = flag.Int("arg_non_default_vrf_ipv4_nhg_count", 1000, "In gRIBI scaling tests, the number of next-hop-groups to install to be referenced from IPv4 entries in non-default VRFs")
	NonDefaultVRFIPv4NHSize       = flag.Int("arg_non_default_vrf_ipv4_nh_size", 8, "In gRIBI scaling tests, the number of next-hops in each next-hop-group referenced from IPv4 entries in non-default VRFs")
	NonDefaultVRFIPv4NHGWeightSum = flag.Int("arg_non_default_vrf_ipv4_nhg_weight_sum", 32, "In gRIBI scaling tests, the sum of weights to assign to next-hops within a next-hop-group referenced from IPv4 entries in non-default VRFs")
	DecapEncapCount               = flag.Int("arg_decap_encap_count", 64, "In gRIBI scaling tests, number of next-hop-groups with decap+encap next-hops")
	DefaultVRFPrimarySubifCount   = flag.Int("arg_default_vrf_primary_subif_count", 64, "In gRIBI scaling tests, number of subinterfaces to use for \"primary\" (i.e. non-backup) next-hop forwarding. Set such that DefaultVRFPrimarySubifCount <= (DefaultVRFIPv4NHCount - DefaultVRFIPv4NHSize)")

	V4TunnelCount         = flag.Int("arg_v4_tunnel_count", 20000, "In gRIBI scaling tests, the number of tunnel IPs.")
	V4TunnelNHGCount      = flag.Int("arg_v4_tunnel_nhg_count", 256, "In gRIBI scaling tests, the number of next-hop-groups associated to the v4 tunnels.")
	V4TunnelNHGSplitCount = flag.Int("arg_v4_tunnel_nhg_split_count", 2, "In gRIBI scaling tests, the number of next-hop per next-hop-group for the v4 tunnels.")
	EgressNHGSplitCount   = flag.Int("arg_egress_nhg_split_count", 16, "In gRIBI scaling tests, the number of next-hop per next-hop-group for the egress traffic.")
	V4ReEncapNHGCount     = flag.Int("arg_v4_re_encap_nhg_count", 256, "In gRIBI scaling tests, the number of next-hop-groups for re-encapping v4 tunnels.")
)
