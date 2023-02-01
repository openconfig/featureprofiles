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
)

// Vendor deviation flags.
var (
	BannerDelimiter = flag.String("deviation_banner_delimiter", "",
		"Device requires the banner to have a delimiter character. Full OpenConfig compliant devices should work without delimiter.")

	NTPAssociationTypeRequired = flag.Bool("deviation_ntp_association_type_required", false,
		"Device requires NTP association-type to be explicitly set.  OpenConfig defaults the association-type to SERVER if not set.")

	InterfaceEnabled = flag.Bool("deviation_interface_enabled", false,
		"Device requires interface enabled leaf booleans to be explicitly set to true.  Full OpenConfig compliant devices should pass both with and without this deviation.")

	InterfaceOperStatus = flag.Bool("deviation_interface_operstatus", false,
		"Device generates Interface_OperStatus_DOWN instead of Interface_OperStatus_LOWER_LAYER_DOWN for an aggregated link.")

	IPv4MissingEnabled = flag.Bool("deviation_ipv4_missing_enabled", false, "Device does not support interface/ipv4/enabled, so suppress configuring this leaf.")

	IPNeighborMissing = flag.Bool("deviation_ip_neighbor_missing", false, "Device does not support interface/ipv4(6)/neighbor, so suppress the related check for interface/ipv4(6)/neighbor.")

	InterfaceCountersFromContainer = flag.Bool("deviation_interface_counters_from_container", false, "Device only supports querying counters from the state container, not from individual counter leaves.")

	AggregateAtomicUpdate = flag.Bool("deviation_aggregate_atomic_update", false,
		"Device requires that aggregate Port-Channel and its members be defined in a single gNMI Update transaction at /interfaces; otherwise lag-type will be dropped, and no member can be added to the aggregate.  Full OpenConfig compliant devices should pass both with and without this deviation.")

	DefaultNetworkInstance = flag.String("deviation_default_network_instance", "DEFAULT",
		"The name used for the default network instance for VRF.  The default name in OpenConfig is \"DEFAULT\" but some legacy devices still use \"default\".  Full OpenConfig compliant devices should be able to use any operator-assigned value.")

	SubinterfacePacketCountersMissing = flag.Bool("deviation_subinterface_packet_counters_missing", false,
		"Device is missing subinterface packet counters for IPv4/IPv6, so the test will skip checking them.  Full OpenConfig compliant devices should pass both with and without this deviation.")

	OmitL2MTU = flag.Bool("deviation_omit_l2_mtu", false,
		"Device does not support setting the L2 MTU, so omit it.  OpenConfig allows a device to enforce that L2 MTU, which has a default value of 1514, must be set to a higher value than L3 MTU, so a full OpenConfig compliant device may fail with the deviation.")

	GRIBIPreserveOnly = flag.Bool("deviation_gribi_preserve_only", false, "Device does not support gRIBI client with persistence DELETE, so this skips the optional test cases in DELETE mode.  However, tests explicitly testing DELETE mode will still run.  Full gRIBI compliant devices should pass both with and without this deviation.")

	GRIBIRIBAckOnly = flag.Bool("deviation_gribi_riback_only", false, "Device only supports RIB ack, so tests that normally expect FIB_ACK will allow just RIB_ACK.  Full gRIBI compliant devices should pass both with and without this deviation.")

	MissingValueForDefaults = flag.Bool("deviation_missing_value_for_defaults", false,
		"Device returns no value for some OpenConfig paths if the operational value equals the default. A fully compliant device should pass regardless of this deviation.")

	StaticProtocolName = flag.String("deviation_static_protocol_name", "DEFAULT", "The name used for the static routing protocol.  The default name in OpenConfig is \"DEFAULT\" but some devices use other names.")

	GNOISubcomponentPath = flag.Bool("deviation_gnoi_subcomponent_path", false, "Device currently uses component name instead of a full openconfig path, so suppress creating a full oc compliant path for subcomponent.")

	GNOIStatusWithEmptySubcomponent = flag.Bool("deviation_gnoi_status_empty_subcomponent", false, "The response of gNOI reboot status is a single value (not a list), so the device requires explict component path to account for a situation when there is more than one active reboot requests.")

	DeprecatedVlanID = flag.Bool("deviation_deprecated_vlan_id", false, "Device requires using the deprecated openconfig-vlan:vlan/config/vlan-id or openconfig-vlan:vlan/state/vlan-id leaves.")

	ExplicitInterfaceInDefaultVRF = flag.Bool("deviation_explicit_interface_in_default_vrf", false,
		"Device requires explicit attachment of an interface or subinterface to the default network instance. OpenConfig expects an unattached interface or subinterface to be implicitly part of the default network instance. Fully-compliant devices should pass with and without this deviation.")

	ExplicitPortSpeed = flag.Bool("deviation_explicit_port_speed", false, "Device requires port-speed to be set because its default value may not be usable. Fully compliant devices should select the highest speed available based on negotiation.")

	ExplicitP4RTNodeComponent = flag.Bool("deviation_explicit_p4rt_node_component", false, "Device does not report P4RT node names in the component hierarchy, so use hard coded P4RT node names by passing them through internal/args flags. Fully compliant devices should report the PORT hardware components with the INTEGRATED_CIRCUIT components as their parents, as the P4RT node names.")

	RoutePolicyUnderPeerGroup = flag.Bool("deviation_rpl_under_peergroup", false, "Device requires route-policy configuration under bgp peer-group. Fully-compliant devices should pass with and without this deviation.")

	MissingPrePolicyReceivedRoutes = flag.Bool("deviation_prepolicy_received_routes", false, "Device does not support bgp/neighbors/neighbor/afi-safis/afi-safi/state/prefixes/received-pre-policy. Fully-compliant devices should pass with and without this deviation.")

	RoutePolicyUnderNeighborAfiSafi = flag.Bool("deviation_rpl_under_neighbor_afisafi", false, "Device requires route-policy configuration under bgp neighbor afisafi. Fully-compliant devices should pass with this deviation set to true.")

	ConnectRetry = flag.Bool("deviation_connect_retry", false, "Connect-retry is not supported /bgp/neighbors/neighbor/timers/config/connect-retry.")

	MissingBgpNeighborStatePeerGroup = flag.Bool("deviation_missing_bgp_neighbor_state_peer_group", false, "Device requires peer-group under bgp neighbor bgp/neighbors/neighbor/state/peer-group.")

)
