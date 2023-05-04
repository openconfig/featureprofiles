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

// Package deviations defines the arguments to enable temporary workarounds for the
// featureprofiles test suite using command line flags.
//
// If we consider device compliance level in tiers:
//
//   - Tier 0: Full OpenConfig compliance.  The device can do everything specified by
//     OpenConfig.
//   - Tier 1: Test plan compliance.  The device can pass a test without deviation, which
//     means it satisfies the test requirements.  This is the target compliance tier for
//     featureprofiles tests.
//   - Tier 2: Deviated test plan compliance.  The device can pass a test with deviation.
//
// Deviations typically work by reducing testing requirements or by changing the way the
// configuration is done.  However, the targeted compliance tier is always without
// deviation.
//
// Requirements for deviations:
//
//   - Deviations may only use OpenConfig compliant behavior.
//   - Deviations should be small in scope, typically affecting one sub-test, one
//     OpenConfig path or small OpenConfig sub-tree.
//
// If a device could not pass without deviation, that is considered non-compliant
// behavior.  Ideally, a device should pass both with and without a deviation which means
// the deviation could be safely removed.  However, when the OpenConfig model allows the
// device to reject the deviated case even if it is compliant, then this should be
// explained on a case-by-case basis.
//
// To add a deviation:
//
//   - Submit a github issue explaining the need for the deviation.
//   - Submit a pull request referencing the above issue to add a flag to
//     this file and updates to the tests where it is intended to be used.
//   - Make sure the deviation defaults to false.  False (not deviated) means strictly
//     compliant behavior.  True (deviated) activates the workaround.
//
// To remove a deviation:
//
//   - Submit a pull request which proposes to resolve the relevant
//     github issue by removing the deviation and it's usage within tests.
//   - Typically the author or an affiliate of the author's organization
//     is expected to remove a deviation they introduced.
//
// To enable the deviations for a test run:
//
//   - By default, deviations are not enabled and instead require the
//     test invocation to set an argument to enable the deviation.
//   - For example:
//     go test my_test.go --deviation_interface_enabled=true
package deviations

import (
	"flag"

	"github.com/openconfig/ondatra"
)

// P4RTMissingDelete returns whether the device does not support delete mode in P4RT write requests.
func P4RTMissingDelete(_ *ondatra.DUTDevice) bool {
	return *p4rtMissingDelete
}

// P4RTUnsetElectionIDUnsupported returns whether the device does not support unset election ID.
func P4RTUnsetElectionIDUnsupported(_ *ondatra.DUTDevice) bool {
	return *p4rtUnsetElectionIDUnsupported
}

// ISISRestartSuppressUnsupported returns whether the device should skip isis restart-suppress check.
func ISISRestartSuppressUnsupported(_ *ondatra.DUTDevice) bool {
	return *isisRestartSuppressUnsupported
}

// MissingBgpLastNotificationErrorCode returns whether the last-notification-error-code leaf is missing in bgp.
func MissingBgpLastNotificationErrorCode(_ *ondatra.DUTDevice) bool {
	return *missingBgpLastNotificationErrorCode
}

// GRIBIMACOverrideWithStaticARP returns whether for a gRIBI IPv4 route the device does not support a mac-address only next-hop-entry.
func GRIBIMACOverrideWithStaticARP(_ *ondatra.DUTDevice) bool {
	return *gribiMACOverrideWithStaticARP
}

// CLITakesPrecedenceOverOC returns whether config pushed through origin CLI takes precedence over config pushed through origin OC.
func CLITakesPrecedenceOverOC(_ *ondatra.DUTDevice) bool {
	return *cliTakesPrecedenceOverOC
}

// BGPTrafficTolerance returns the allowed tolerance for BGP traffic flow while comparing for pass or fail conditions.
func BGPTrafficTolerance(_ *ondatra.DUTDevice) int {
	return *bgpTrafficTolerance
}

// MacAddressMissing returns whether device does not support /system/mac-address/state
func MacAddressMissing(_ *ondatra.DUTDevice) bool {
	return *macAddressMissing
}

// UseVendorNativeACLConfig returns whether a device requires native model to configure ACL, specifically for RT-1.4.
func UseVendorNativeACLConfig(_ *ondatra.DUTDevice) bool {
	return *useVendorNativeACLConfiguration
}

// SwitchChipIDUnsupported returns whether the device supports id leaf for SwitchChip components.
func SwitchChipIDUnsupported(_ *ondatra.DUTDevice) bool {
	return *switchChipIDUnsupported
}

// BackplaneFacingCapacityUnsupported returns whether the device supports backplane-facing-capacity leaves for some of the components.
func BackplaneFacingCapacityUnsupported(_ *ondatra.DUTDevice) bool {
	return *backplaneFacingCapacityUnsupported
}

// ComponentsSoftwareModuleUnsupported returns whether the device supports software module components.
func ComponentsSoftwareModuleUnsupported(_ *ondatra.DUTDevice) bool {
	return *componentsSoftwareModuleUnsupported
}

// SchedulerInputWeightLimit returns whether the device does not support weight above 100.
func SchedulerInputWeightLimit(_ *ondatra.DUTDevice) bool {
	return *schedulerInputWeightLimit
}

// ECNProfileRequiredDefinition returns whether the device requires additional config for ECN.
func ECNProfileRequiredDefinition(_ *ondatra.DUTDevice) bool {
	return *ecnProfileRequiredDefinition
}

// ISISGlobalAuthenticationNotRequired returns true if ISIS Global authentication not required.
func ISISGlobalAuthenticationNotRequired(_ *ondatra.DUTDevice) bool {
	return *isisGlobalAuthenticationNotRequired
}

// ISISLevelAuthenticationNotRequired returns true if ISIS Level authentication not required.
func ISISLevelAuthenticationNotRequired(_ *ondatra.DUTDevice) bool {
	return *isisLevelAuthenticationNotRequired
}

// Ipv6DiscardedPktsUnsupported returns whether the device supports interface ipv6 discarded packet stats.
func Ipv6DiscardedPktsUnsupported(_ *ondatra.DUTDevice) bool {
	return *ipv6DiscardedPktsUnsupported
}

// FanOperStatusUnsupported returns whether the device supports oper-status leaf for fan components.
func FanOperStatusUnsupported(_ *ondatra.DUTDevice) bool {
	return *fanOperStatusUnsupported
}

// StatePathsUnsupported returns whether the device supports following state paths
func StatePathsUnsupported(_ *ondatra.DUTDevice) bool {
	return *statePathsUnsupported
}

// DropWeightLeavesUnsupported returns whether the device supports drop and weight leaves under queue management profile
func DropWeightLeavesUnsupported(_ *ondatra.DUTDevice) bool {
	return *dropWeightLeavesUnsupported
}

// SwVersionUnsupported returns true if the device does not support reporting software version according to the requirements in gNMI-1.10.
func SwVersionUnsupported(_ *ondatra.DUTDevice) bool {
	return *swVersionUnsupported
}

// HierarchicalWeightResolutionTolerance returns the allowed tolerance for BGP traffic flow while comparing for pass or fail conditions.
func HierarchicalWeightResolutionTolerance(_ *ondatra.DUTDevice) float64 {
	return *hierarchicalWeightResolutionTolerance
}

// InterfaceCountersFromContainer returns if the device only supports querying counters from the state container, not from individual counter leaves.
func InterfaceCountersFromContainer(_ *ondatra.DUTDevice) bool {
	return *interfaceCountersFromContainer
}

// IPNeighborMissing returns true if the device does not support interface/ipv4(6)/neighbor,
// so test can suppress the related check for interface/ipv4(6)/neighbor.
func IPNeighborMissing(_ *ondatra.DUTDevice) bool {
	return *ipNeighborMissing
}

// NTPAssociationTypeRequired returns if device requires NTP association-type to be explicitly set.
// OpenConfig defaults the association-type to SERVER if not set.
func NTPAssociationTypeRequired(_ *ondatra.DUTDevice) bool {
	return *ntpAssociationTypeRequired
}

// GRIBIRIBAckOnly returns if device only supports RIB ack, so tests that normally expect FIB_ACK will allow just RIB_ACK.
// Full gRIBI compliant devices should pass both with and without this deviation.
func GRIBIRIBAckOnly(_ *ondatra.DUTDevice) bool {
	return *gRIBIRIBAckOnly
}

// MissingInterfacePhysicalChannel returns if device does not support interface/physicalchannel leaf.
func MissingInterfacePhysicalChannel(_ *ondatra.DUTDevice) bool {
	return *missingInterfacePhysicalChannel
}

// MissingInterfaceHardwarePort returns if device does not support interface/hardwareport leaf.
func MissingInterfaceHardwarePort(_ *ondatra.DUTDevice) bool {
	return *missingInterfaceHardwarePort
}

// TraceRouteL4ProtocolUDP returns if device only support UDP as l4 protocol for traceroute.
func TraceRouteL4ProtocolUDP(_ *ondatra.DUTDevice) bool {
	return *traceRouteL4ProtocolUDP
}

// TraceRouteFragmentation returns if device does not support fragmentation bit for traceroute.
func TraceRouteFragmentation(_ *ondatra.DUTDevice) bool {
	return *traceRouteFragmentation
}

// SubinterfacePacketCountersMissing returns if device is missing subinterface packet counters for IPv4/IPv6,
// so the test will skip checking them.
// Full OpenConfig compliant devices should pass both with and without this deviation.
func SubinterfacePacketCountersMissing(_ *ondatra.DUTDevice) bool {
	return *subinterfacePacketCountersMissing
}

// OSActivateNoReboot returns if device requires separate reboot to activate OS.
func OSActivateNoReboot(_ *ondatra.DUTDevice) bool {
	return *osActivateNoReboot
}

// InstallOSForStandbyRP returns if device requires OS installation on standby RP as well as active RP.
func InstallOSForStandbyRP(_ *ondatra.DUTDevice) bool {
	return *installOSForStandbyRP
}

// Vendor deviation flags.
// All new flags should not be exported (define them in lowercase) and accessed
// from tests through a public accessors like those above.
var (
	BannerDelimiter = flag.String("deviation_banner_delimiter", "",
		"Device requires the banner to have a delimiter character. Full OpenConfig compliant devices should work without delimiter.")

	ntpAssociationTypeRequired = flag.Bool("deviation_ntp_association_type_required", false,
		"Device requires NTP association-type to be explicitly set.  OpenConfig defaults the association-type to SERVER if not set.")

	InterfaceEnabled = flag.Bool("deviation_interface_enabled", false,
		"Device requires interface enabled leaf booleans to be explicitly set to true.  Full OpenConfig compliant devices should pass both with and without this deviation.")

	IPv4MissingEnabled = flag.Bool("deviation_ipv4_missing_enabled", false, "Device does not support interface/ipv4/enabled, so suppress configuring this leaf.")

	ipNeighborMissing = flag.Bool("deviation_ip_neighbor_missing", false, "Device does not support interface/ipv4(6)/neighbor, so suppress the related check for interface/ipv4(6)/neighbor.")

	interfaceCountersFromContainer = flag.Bool("deviation_interface_counters_from_container", false, "Device only supports querying counters from the state container, not from individual counter leaves.")

	AggregateAtomicUpdate = flag.Bool("deviation_aggregate_atomic_update", false,
		"Device requires that aggregate Port-Channel and its members be defined in a single gNMI Update transaction at /interfaces; otherwise lag-type will be dropped, and no member can be added to the aggregate.  Full OpenConfig compliant devices should pass both with and without this deviation.")

	DefaultNetworkInstance = flag.String("deviation_default_network_instance", "DEFAULT",
		"The name used for the default network instance for VRF.  The default name in OpenConfig is \"DEFAULT\" but some legacy devices still use \"default\".  Full OpenConfig compliant devices should be able to use any operator-assigned value.")

	subinterfacePacketCountersMissing = flag.Bool("deviation_subinterface_packet_counters_missing", false,
		"Device is missing subinterface packet counters for IPv4/IPv6, so the test will skip checking them.  Full OpenConfig compliant devices should pass both with and without this deviation.")

	OmitL2MTU = flag.Bool("deviation_omit_l2_mtu", false,
		"Device does not support setting the L2 MTU, so omit it.  OpenConfig allows a device to enforce that L2 MTU, which has a default value of 1514, must be set to a higher value than L3 MTU, so a full OpenConfig compliant device may fail with the deviation.")

	gRIBIRIBAckOnly = flag.Bool("deviation_gribi_riback_only", false, "Device only supports RIB ack, so tests that normally expect FIB_ACK will allow just RIB_ACK.  Full gRIBI compliant devices should pass both with and without this deviation.")

	MissingValueForDefaults = flag.Bool("deviation_missing_value_for_defaults", false,
		"Device returns no value for some OpenConfig paths if the operational value equals the default. A fully compliant device should pass regardless of this deviation.")

	StaticProtocolName = flag.String("deviation_static_protocol_name", "DEFAULT", "The name used for the static routing protocol.  The default name in OpenConfig is \"DEFAULT\" but some devices use other names.")

	GNOISubcomponentPath = flag.Bool("deviation_gnoi_subcomponent_path", false, "Device currently uses component name instead of a full openconfig path, so suppress creating a full oc compliant path for subcomponent.")

	GNOIStatusWithEmptySubcomponent = flag.Bool("deviation_gnoi_status_empty_subcomponent", false, "The response of gNOI reboot status is a single value (not a list), so the device requires explict component path to account for a situation when there is more than one active reboot requests.")

	osActivateNoReboot = flag.Bool("deviation_osactivate_noreboot", false, "Device requires separate reboot to activate OS.")

	installOSForStandbyRP = flag.Bool("deviation_osinstall_for_standby_rp", false, "Device requires OS installation on standby RP as well as active RP.")

	DeprecatedVlanID = flag.Bool("deviation_deprecated_vlan_id", false, "Device requires using the deprecated openconfig-vlan:vlan/config/vlan-id or openconfig-vlan:vlan/state/vlan-id leaves.")

	ExplicitInterfaceInDefaultVRF = flag.Bool("deviation_explicit_interface_in_default_vrf", false,
		"Device requires explicit attachment of an interface or subinterface to the default network instance. OpenConfig expects an unattached interface or subinterface to be implicitly part of the default network instance. Fully-compliant devices should pass with and without this deviation.")

	ExplicitPortSpeed = flag.Bool("deviation_explicit_port_speed", false, "Device requires port-speed to be set because its default value may not be usable. Fully compliant devices should select the highest speed available based on negotiation.")

	ExplicitP4RTNodeComponent = flag.Bool("deviation_explicit_p4rt_node_component", false, "Device does not report P4RT node names in the component hierarchy, so use hard coded P4RT node names by passing them through internal/args flags. Fully compliant devices should report the PORT hardware components with the INTEGRATED_CIRCUIT components as their parents, as the P4RT node names.")

	RoutePolicyUnderPeerGroup = flag.Bool("deviation_rpl_under_peergroup", false, "Device requires route-policy configuration under bgp peer-group. Fully-compliant devices should pass with and without this deviation.")

	MissingPrePolicyReceivedRoutes = flag.Bool("deviation_prepolicy_received_routes", false, "Device does not support bgp/neighbors/neighbor/afi-safis/afi-safi/state/prefixes/received-pre-policy. Fully-compliant devices should pass with and without this deviation.")

	RoutePolicyUnderNeighborAfiSafi = flag.Bool("deviation_rpl_under_neighbor_afisafi", false, "Device requires route-policy configuration under bgp neighbor afisafi. Fully-compliant devices should pass with this deviation set to true.")

	traceRouteL4ProtocolUDP = flag.Bool("deviation_traceroute_l4_protocol_udp", false, "Device only support UDP as l4 protocol for traceroute. Use this flag to set default l4 protocol as UDP and skip the tests explictly use TCP or ICMP.")

	traceRouteFragmentation = flag.Bool("deviation_traceroute_fragmentation", false, "Device does not support fragmentation bit for traceroute.")

	ConnectRetry = flag.Bool("deviation_connect_retry", false, "Connect-retry is not supported /bgp/neighbors/neighbor/timers/config/connect-retry.")

	ExplicitIPv6EnableForGRIBI = flag.Bool("deviation_ipv6_enable_for_gribi_nh_dmac", false, "Device requires Ipv6 to be enabled on interface for gRIBI NH programmed with destination mac address")

	ISISInterfaceLevel1DisableRequired = flag.Bool("deviation_isis_interface_level1_disable_required", false,
		"Disable isis level1 under interface mode on the device if value is true, Default value is false and enables isis level2 under interface mode")

	MissingIsisInterfaceAfiSafiEnable = flag.Bool("deviation_missing_isis_interface_afi_safi_enable", false,
		"Set and validate isis interface address family enable on the device if value is true, Default value is false and validate isis address family enable at global mode")

	IsisSingleTopologyRequired = flag.Bool("deviation_isis_single_topology_required", false,
		"Set isis af ipv6 single topology on the device if value is true, Default value is false and sets multi topology for isis af ipv6")

	ISISprotocolEnabledNotRequired = flag.Bool("deviation_isis_protocol_enabled_not_required", false,
		"Unset isis protocol enable flag on the device if value is true, Default value is false and protocol enable flag is set")

	ISISInstanceEnabledNotRequired = flag.Bool("deviation_isis_instance_enabled_not_required", false,
		"Don't set isis instance enable flag on the device if value is true, Default value is false and instance enable flag is set")

	ExplicitInterfaceRefDefinition = flag.Bool("deviation_explicit_interface_ref_definition", false, "Device requires explicit interface ref configuration when applying features to interface")

	NoMixOfTaggedAndUntaggedSubinterfaces = flag.Bool("deviation_no_mix_of_tagged_and_untagged_subinterfaces", false,
		"Use this deviation when the device does not support a mix of tagged and untagged subinterfaces")

	GRIBIDelayedAckResponse = flag.Bool("deviation_gribi_delayed_ack_response", false, "Device requires delay in sending ack response")

	LLDPInterfaceConfigOverrideGlobal = flag.Bool("deviation_lldp_interface_config_override_global", false,
		"Set this flag for LLDP interface config to override the global config,expect neighbours are seen when lldp is disabled globally but enabled on interface")

	missingInterfacePhysicalChannel = flag.Bool("deviation_missing_interface_physical_channel", false,
		"Device does not support interface/physicalchannel leaf. Set this flag to skip checking the leaf.")

	missingInterfaceHardwarePort = flag.Bool("deviation_missing_interface_hardware_port", false,
		"Device does not support interface/hardwareport leaf. Set this flag to skip checking the leaf.")

	InterfaceConfigVrfBeforeAddress = flag.Bool("deviation_interface_config_vrf_before_address", false, "When configuring interface, config Vrf prior config IP address")

	bgpTrafficTolerance = flag.Int("deviation_bgp_tolerance_value", 0,
		"Allowed tolerance for BGP traffic flow while comparing for pass or fail condition.")

	ExplicitGRIBIUnderNetworkInstance = flag.Bool("deviation_explicit_gribi_under_network_instance", false,
		"Device requires gribi-protocol to be enabled under network-instance.")

	BGPMD5RequiresReset = flag.Bool("deviation_bgp_md5_requires_reset", false, "Device requires a BGP session reset to utilize a new MD5 key")

	QOSDroppedOctets = flag.Bool("deviation_qos_dropped_octets", false, "Set to true to skip checking QOS Dropped octets stats for interface")

	SkipBGPTestPasswordMismatch = flag.Bool("deviation_skip_bgp_test_password_mismatch", false,
		"Skip BGP TestPassword mismatch subtest if value is true, Default value is false")

	p4rtMissingDelete = flag.Bool("deviation_p4rt_missing_delete", false, "Device does not support delete mode in P4RT write requests")

	p4rtUnsetElectionIDUnsupported = flag.Bool("deviation_p4rt_unsetelectionid_unsupported", false, "Device does not support unset Election ID")

	NetworkInstanceTableDeletionRequired = flag.Bool("deviation_network_instance_table_deletion_required", false,
		"Set to true for device requiring explicit deletion of network-instance table, default is false")

	ISISMultiTopologyUnsupported = flag.Bool("deviation_isis_multi_topology_unsupported", false,
		"Device skip isis multi-topology check if value is true, Default value is false")

	isisRestartSuppressUnsupported = flag.Bool("deviation_isis_restart_suppress_unsupported", false,
		"Device skip isis restart-suppress check if value is true, Default value is false")

	macAddressMissing = flag.Bool("deviation_mac_address_missing", false, "Device does not support /system/mac-address/state.")

	gribiMACOverrideWithStaticARP = flag.Bool("deviation_gribi_mac_override_with_static_arp", false, "Set to true for device not supporting programming a gribi flow with a next-hop entry of mac-address only, default is false")

	cliTakesPrecedenceOverOC = flag.Bool("deviation_cli_takes_precedence_over_oc", false, "Set to true for device in which config pushed through origin CLI takes precedence over config pushed through origin OC, default is false")

	missingBgpLastNotificationErrorCode = flag.Bool("deviation_missing_bgp_last_notification_error_code", false, "Set to true to skip check for bgp/neighbors/neighbor/state/messages/received/last-notification-error-code leaf missing case")

	useVendorNativeACLConfiguration = flag.Bool("deviation_use_vendor_native_acl_config", false, "Configure ACLs using vendor native model specifically for RT-1.4")

	switchChipIDUnsupported = flag.Bool("deviation_switch_chip_id_unsupported", false, "Device does not support id leaf for SwitchChip components. Set this flag to skip checking the leaf.")

	backplaneFacingCapacityUnsupported = flag.Bool("deviation_backplane_facing_capacity_unsupported", false, "Device does not support backplane-facing-capacity leaves for some of the components. Set this flag to skip checking the leaves.")

	componentsSoftwareModuleUnsupported = flag.Bool("deviation_components_software_module_unsupported", false, "Set true for Device that does not support software module components, default is false.")

	schedulerInputWeightLimit = flag.Bool("deviation_scheduler_input_weight_limit", false, "device does not support weight above 100")

	ecnProfileRequiredDefinition = flag.Bool("deviation_ecn_profile_required_definition", false, "device requires additional config for ECN")

	isisGlobalAuthenticationNotRequired = flag.Bool("deviation_isis_global_authentication_not_required", false,
		"Don't set isis global authentication-check on the device if value is true, Default value is false and ISIS global authentication-check is set")

	isisLevelAuthenticationNotRequired = flag.Bool("deviation_isis_level_authentication_not_required", false,
		"Don't set isis level authentication on the device if value is true, Default value is false and ISIS level authentication is configured")

	ipv6DiscardedPktsUnsupported = flag.Bool("deviation_ipv6_discarded_pkts_unsupported", false, "Set true for device that does not support interface ipv6 discarded packet statistics, default is false")

	fanOperStatusUnsupported = flag.Bool("deviation_fan_oper_status_unsupported", false, "Device does not support oper-status leaves for some of the fan components. Set this flag to skip checking the leaf.")

	statePathsUnsupported = flag.Bool("deviation_state_path_unsupported", false, "Device does not support these state paths, Set this flag to skip checking the leaves")

	dropWeightLeavesUnsupported = flag.Bool("deviation_drop_weight_leaves_unsupported", false, "Device does not support drop and weight leaves under queue management profile, Set this flag to skip checking the leaves")

	swVersionUnsupported = flag.Bool("deviation_sw_version_unsupported", false, "Device does not support reporting software version according to the requirements in gNMI-1.10.")

	hierarchicalWeightResolutionTolerance = flag.Float64("deviation_hierarchical_weight_resolution_tolerance", 0.2, "Set it to expected ucmp traffic tolerance, default is 0.2")
)
