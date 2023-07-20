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
// To add, remove and enable deviations follow the guidelines at deviations/README.md
package deviations

import (
	"fmt"
	"regexp"

	"flag"

	log "github.com/golang/glog"
	"github.com/openconfig/featureprofiles/internal/metadata"
	mpb "github.com/openconfig/featureprofiles/proto/metadata_go_proto"
	"github.com/openconfig/ondatra"
)

func lookupDeviations(dut *ondatra.DUTDevice) (*mpb.Metadata_PlatformExceptions, error) {
	var matchedPlatformException *mpb.Metadata_PlatformExceptions

	for _, platformExceptions := range metadata.Get().PlatformExceptions {
		if platformExceptions.GetPlatform().Vendor.String() == "" {
			return nil, fmt.Errorf("vendor should be specified in textproto %v", platformExceptions)
		}

		if dut.Device.Vendor().String() != platformExceptions.GetPlatform().Vendor.String() {
			continue
		}

		// If hardware_model_regex is set and does not match, continue
		if hardwareModelRegex := platformExceptions.GetPlatform().GetHardwareModelRegex(); hardwareModelRegex != "" {
			matchHw, errHw := regexp.MatchString(hardwareModelRegex, dut.Device.Model())
			if errHw != nil {
				return nil, fmt.Errorf("error with regex match %v", errHw)
			}
			if !matchHw {
				continue
			}
		}

		// If software_version_regex is set and does not match, continue
		if softwareVersionRegex := platformExceptions.GetPlatform().GetSoftwareVersionRegex(); softwareVersionRegex != "" {
			matchSw, errSw := regexp.MatchString(softwareVersionRegex, dut.Device.Model())
			if errSw != nil {
				return nil, fmt.Errorf("error with regex match %v", errSw)
			}
			if !matchSw {
				continue
			}
		}

		if matchedPlatformException != nil {
			return nil, fmt.Errorf("cannot have more than one match within platform_exceptions fields %v and %v", matchedPlatformException, platformExceptions)
		}
		matchedPlatformException = platformExceptions
	}
	return matchedPlatformException, nil
}

func mustLookupDeviations(dut *ondatra.DUTDevice) *mpb.Metadata_PlatformExceptions {
	platformExceptions, err := lookupDeviations(dut)
	if err != nil {
		log.Exitf("Error looking up deviations: %v", err)
	}
	return platformExceptions
}

func lookupDUTDeviations(dut *ondatra.DUTDevice) *mpb.Metadata_Deviations {
	if platformExceptions := mustLookupDeviations(dut); platformExceptions != nil {
		return platformExceptions.GetDeviations()
	}
	log.Infof("Did not match any platform_exception %v, returning default values", metadata.Get().GetPlatformExceptions())
	return &mpb.Metadata_Deviations{}
}

func logErrorIfFlagSet(name string) {
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			log.Errorf("Value for %v is set using metadata.textproto. Flag value will be ignored!", name)
		}
	})
}

// BannerDelimiter returns if device requires the banner to have a delimiter character.
// Full OpenConfig compliant devices should work without delimiter.
func BannerDelimiter(dut *ondatra.DUTDevice) string {
	logErrorIfFlagSet("deviation_banner_delimiter")
	return lookupDUTDeviations(dut).GetBannerDelimiter()
}

// OmitL2MTU returns if device does not support setting the L2 MTU.
func OmitL2MTU(dut *ondatra.DUTDevice) bool {
	logErrorIfFlagSet("deviation_omit_l2_mtu")
	return lookupDUTDeviations(dut).GetOmitL2Mtu()
}

// GRIBIMACOverrideStaticARPStaticRoute returns whether the device needs to configure Static ARP + Static Route to override setting MAC address in Next Hop.
func GRIBIMACOverrideStaticARPStaticRoute(dut *ondatra.DUTDevice) bool {
	logErrorIfFlagSet("deviation_gribi_mac_override_static_arp_static_route")
	return lookupDUTDeviations(dut).GetGribiMacOverrideStaticArpStaticRoute()
}

// AggregateAtomicUpdate returns if device requires that aggregate Port-Channel and its members be defined in a single gNMI Update transaction at /interfaces.
// Otherwise lag-type will be dropped, and no member can be added to the aggregate.
// Full OpenConfig compliant devices should pass both with and without this deviation.
func AggregateAtomicUpdate(dut *ondatra.DUTDevice) bool {
	logErrorIfFlagSet("deviation_aggregate_atomic_update")
	return lookupDUTDeviations(dut).GetAggregateAtomicUpdate()
}

// DefaultNetworkInstance returns the name used for the default network instance for VRF.
func DefaultNetworkInstance(dut *ondatra.DUTDevice) string {
	logErrorIfFlagSet("deviation_default_network_instance")
	//
	if dni := lookupDUTDeviations(dut).GetDefaultNetworkInstance(); dni != "" {
		return dni
	}
	return "DEFAULT"
}

// ExplicitP4RTNodeComponent returns if device does not report P4RT node names in the component hierarchy.
// Fully compliant devices should report the PORT hardware components with the INTEGRATED_CIRCUIT components as their parents, as the P4RT node names.
func ExplicitP4RTNodeComponent(dut *ondatra.DUTDevice) bool {
	logErrorIfFlagSet("deviation_explicit_p4rt_node_component")
	return lookupDUTDeviations(dut).GetExplicitP4RtNodeComponent()
}

// ISISRestartSuppressUnsupported returns whether the device should skip isis restart-suppress check.
func ISISRestartSuppressUnsupported(dut *ondatra.DUTDevice) bool {
	logErrorIfFlagSet("deviation_isis_restart_suppress_unsupported")
	return lookupDUTDeviations(dut).GetIsisRestartSuppressUnsupported()
}

// MissingBgpLastNotificationErrorCode returns whether the last-notification-error-code leaf is missing in bgp.
func MissingBgpLastNotificationErrorCode(dut *ondatra.DUTDevice) bool {
	logErrorIfFlagSet("deviation_missing_bgp_last_notification_error_code")
	return lookupDUTDeviations(dut).GetMissingBgpLastNotificationErrorCode()
}

// GRIBIMACOverrideWithStaticARP returns whether for a gRIBI IPv4 route the device does not support a mac-address only next-hop-entry.
func GRIBIMACOverrideWithStaticARP(dut *ondatra.DUTDevice) bool {
	logErrorIfFlagSet("deviation_gribi_mac_override_with_static_arp")
	return lookupDUTDeviations(dut).GetGribiMacOverrideWithStaticArp()
}

// CLITakesPrecedenceOverOC returns whether config pushed through origin CLI takes precedence over config pushed through origin OC.
func CLITakesPrecedenceOverOC(dut *ondatra.DUTDevice) bool {
	logErrorIfFlagSet("deviation_cli_takes_precedence_over_oc")
	return lookupDUTDeviations(dut).GetCliTakesPrecedenceOverOc()
}

// BGPTrafficTolerance returns the allowed tolerance for BGP traffic flow while comparing for pass or fail conditions.
func BGPTrafficTolerance(dut *ondatra.DUTDevice) int32 {
	logErrorIfFlagSet("deviation_bgp_tolerance_value")
	return lookupDUTDeviations(dut).GetBgpToleranceValue()
}

// StaticProtocolName returns the name used for the static routing protocol.
func StaticProtocolName(dut *ondatra.DUTDevice) string {
	logErrorIfFlagSet("deviation_static_protocol_name")
	if spn := lookupDUTDeviations(dut).GetStaticProtocolName(); spn != "" {
		return spn
	}
	return "DEFAULT"
}

// UseVendorNativeACLConfig returns whether a device requires native model to configure ACL, specifically for RT-1.4.
func UseVendorNativeACLConfig(dut *ondatra.DUTDevice) bool {
	logErrorIfFlagSet("deviation_use_vendor_native_acl_config")
	return lookupDUTDeviations(dut).GetUseVendorNativeAclConfig()
}

// SwitchChipIDUnsupported returns whether the device supports id leaf for SwitchChip components.
func SwitchChipIDUnsupported(dut *ondatra.DUTDevice) bool {
	logErrorIfFlagSet("deviation_switch_chip_id_unsupported")
	return lookupDUTDeviations(dut).GetSwitchChipIdUnsupported()
}

// BackplaneFacingCapacityUnsupported returns whether the device supports backplane-facing-capacity leaves for some of the components.
func BackplaneFacingCapacityUnsupported(dut *ondatra.DUTDevice) bool {
	logErrorIfFlagSet("deviation_backplane_facing_capacity_unsupported")
	return lookupDUTDeviations(dut).GetBackplaneFacingCapacityUnsupported()
}

// SchedulerInputWeightLimit returns whether the device does not support weight above 100.
func SchedulerInputWeightLimit(dut *ondatra.DUTDevice) bool {
	logErrorIfFlagSet("deviation_scheduler_input_weight_limit")
	return lookupDUTDeviations(dut).GetSchedulerInputWeightLimit()
}

// ECNProfileRequiredDefinition returns whether the device requires additional config for ECN.
func ECNProfileRequiredDefinition(dut *ondatra.DUTDevice) bool {
	logErrorIfFlagSet("deviation_ecn_profile_required_definition")
	return lookupDUTDeviations(dut).GetEcnProfileRequiredDefinition()
}

// ISISGlobalAuthenticationNotRequired returns true if ISIS Global authentication not required.
func ISISGlobalAuthenticationNotRequired(dut *ondatra.DUTDevice) bool {
	logErrorIfFlagSet("deviation_isis_global_authentication_not_required")
	return lookupDUTDeviations(dut).GetIsisGlobalAuthenticationNotRequired()
}

// ISISExplicitLevelAuthenticationConfig returns true if ISIS Explicit Level Authentication configuration is required
func ISISExplicitLevelAuthenticationConfig(dut *ondatra.DUTDevice) bool {
	logErrorIfFlagSet("deviation_isis_explicit_level_authentication_config")
	return lookupDUTDeviations(dut).GetIsisExplicitLevelAuthenticationConfig()
}

// ISISSingleTopologyRequired sets isis af ipv6 single topology on the device if value is true.
func ISISSingleTopologyRequired(dut *ondatra.DUTDevice) bool {
	logErrorIfFlagSet("deviation_isis_single_topology_required")
	return lookupDUTDeviations(dut).GetIsisSingleTopologyRequired()
}

// ISISMultiTopologyUnsupported returns if device skips isis multi-topology check.
func ISISMultiTopologyUnsupported(dut *ondatra.DUTDevice) bool {
	logErrorIfFlagSet("deviation_isis_multi_topology_unsupported")
	return lookupDUTDeviations(dut).GetIsisMultiTopologyUnsupported()
}

// ISISInterfaceLevel1DisableRequired returns if device should disable isis level1 under interface mode.
func ISISInterfaceLevel1DisableRequired(dut *ondatra.DUTDevice) bool {
	logErrorIfFlagSet("deviation_isis_interface_level1_disable_required")
	return lookupDUTDeviations(dut).GetIsisInterfaceLevel1DisableRequired()
}

// MissingIsisInterfaceAfiSafiEnable returns if device should set and validate isis interface address family enable.
// Default is validate isis address family enable at global mode.
func MissingIsisInterfaceAfiSafiEnable(dut *ondatra.DUTDevice) bool {
	logErrorIfFlagSet("deviation_missing_isis_interface_afi_safi_enable")
	return lookupDUTDeviations(dut).GetMissingIsisInterfaceAfiSafiEnable()
}

// Ipv6DiscardedPktsUnsupported returns whether the device supports interface ipv6 discarded packet stats.
func Ipv6DiscardedPktsUnsupported(dut *ondatra.DUTDevice) bool {
	logErrorIfFlagSet("deviation_ipv6_discarded_pkts_unsupported")
	return lookupDUTDeviations(dut).GetIpv6DiscardedPktsUnsupported()
}

// LinkQualWaitAfterDeleteRequired returns whether the device requires additional time to complete post delete link qualification cleanup.
func LinkQualWaitAfterDeleteRequired(dut *ondatra.DUTDevice) bool {
	logErrorIfFlagSet("deviation_link_qual_wait_after_delete_required")
	return lookupDUTDeviations(dut).GetLinkQualWaitAfterDeleteRequired()
}

// StatePathsUnsupported returns whether the device supports following state paths
func StatePathsUnsupported(dut *ondatra.DUTDevice) bool {
	logErrorIfFlagSet("deviation_state_path_unsupported")
	return lookupDUTDeviations(dut).GetStatePathUnsupported()
}

// DropWeightLeavesUnsupported returns whether the device supports drop and weight leaves under queue management profile.
func DropWeightLeavesUnsupported(dut *ondatra.DUTDevice) bool {
	logErrorIfFlagSet("deviation_drop_weight_leaves_unsupported")
	return lookupDUTDeviations(dut).GetDropWeightLeavesUnsupported()
}

// SwVersionUnsupported returns true if the device does not support reporting software version according to the requirements in gNMI-1.10.
func SwVersionUnsupported(dut *ondatra.DUTDevice) bool {
	logErrorIfFlagSet("deviation_sw_version_unsupported")
	return lookupDUTDeviations(dut).GetSwVersionUnsupported()
}

// HierarchicalWeightResolutionTolerance returns the allowed tolerance for BGP traffic flow while comparing for pass or fail conditions.
// Default minimum value is 0.2. Anything less than 0.2 will be set to 0.2.
func HierarchicalWeightResolutionTolerance(dut *ondatra.DUTDevice) float64 {
	logErrorIfFlagSet("deviation_hierarchical_weight_resolution_tolerance")
	hwrt := lookupDUTDeviations(dut).GetHierarchicalWeightResolutionTolerance()
	if minHWRT := 0.2; hwrt < minHWRT {
		return minHWRT
	}
	return hwrt
}

// InterfaceEnabled returns if device requires interface enabled leaf booleans to be explicitly set to true.
func InterfaceEnabled(dut *ondatra.DUTDevice) bool {
	logErrorIfFlagSet("deviation_interface_enabled")
	return lookupDUTDeviations(dut).GetInterfaceEnabled()
}

// InterfaceCountersFromContainer returns if the device only supports querying counters from the state container, not from individual counter leaves.
func InterfaceCountersFromContainer(dut *ondatra.DUTDevice) bool {
	logErrorIfFlagSet("deviation_interface_counters_from_container")
	return lookupDUTDeviations(dut).GetInterfaceCountersFromContainer()
}

// IPv4MissingEnabled returns if device does not support interface/ipv4/enabled.
func IPv4MissingEnabled(dut *ondatra.DUTDevice) bool {
	logErrorIfFlagSet("deviation_ipv4_missing_enabled")
	return lookupDUTDeviations(dut).GetIpv4MissingEnabled()
}

// IPNeighborMissing returns true if the device does not support interface/ipv4(6)/neighbor,
// so test can suppress the related check for interface/ipv4(6)/neighbor.
func IPNeighborMissing(dut *ondatra.DUTDevice) bool {
	logErrorIfFlagSet("deviation_ip_neighbor_missing")
	return lookupDUTDeviations(dut).GetIpNeighborMissing()
}

// GRIBIRIBAckOnly returns if device only supports RIB ack, so tests that normally expect FIB_ACK will allow just RIB_ACK.
// Full gRIBI compliant devices should pass both with and without this deviation.
func GRIBIRIBAckOnly(dut *ondatra.DUTDevice) bool {
	logErrorIfFlagSet("deviation_gribi_riback_only")
	return lookupDUTDeviations(dut).GetGribiRibackOnly()
}

// MissingInterfacePhysicalChannel returns if device does not support interface/physicalchannel leaf.
func MissingInterfacePhysicalChannel(dut *ondatra.DUTDevice) bool {
	logErrorIfFlagSet("deviation_missing_interface_physical_channel")
	return lookupDUTDeviations(dut).GetMissingInterfacePhysicalChannel()
}

// MissingValueForDefaults returns if device returns no value for some OpenConfig paths if the operational value equals the default.
func MissingValueForDefaults(dut *ondatra.DUTDevice) bool {
	logErrorIfFlagSet("deviation_missing_value_for_defaults")
	return lookupDUTDeviations(dut).GetMissingValueForDefaults()
}

// TraceRouteL4ProtocolUDP returns if device only support UDP as l4 protocol for traceroute.
// Default value is false.
func TraceRouteL4ProtocolUDP(dut *ondatra.DUTDevice) bool {
	logErrorIfFlagSet("deviation_traceroute_l4_protocol_udp")
	return lookupDUTDeviations(dut).GetTracerouteL4ProtocolUdp()
}

// LLDPInterfaceConfigOverrideGlobal returns if LLDP interface config should override the global config,
// expect neighbours are seen when lldp is disabled globally but enabled on interface
func LLDPInterfaceConfigOverrideGlobal(dut *ondatra.DUTDevice) bool {
	logErrorIfFlagSet("deviation_lldp_interface_config_override_global")
	return lookupDUTDeviations(dut).GetLldpInterfaceConfigOverrideGlobal()
}

// SubinterfacePacketCountersMissing returns if device is missing subinterface packet counters for IPv4/IPv6,
// so the test will skip checking them.
// Full OpenConfig compliant devices should pass both with and without this deviation.
func SubinterfacePacketCountersMissing(dut *ondatra.DUTDevice) bool {
	logErrorIfFlagSet("deviation_subinterface_packet_counters_missing")
	return lookupDUTDeviations(dut).GetSubinterfacePacketCountersMissing()
}

// MissingPrePolicyReceivedRoutes returns if device does not support bgp/neighbors/neighbor/afi-safis/afi-safi/state/prefixes/received-pre-policy.
// Fully-compliant devices should pass with and without this deviation.
func MissingPrePolicyReceivedRoutes(dut *ondatra.DUTDevice) bool {
	logErrorIfFlagSet("deviation_prepolicy_received_routes")
	return lookupDUTDeviations(dut).GetPrepolicyReceivedRoutes()
}

// DeprecatedVlanID returns if device requires using the deprecated openconfig-vlan:vlan/config/vlan-id or openconfig-vlan:vlan/state/vlan-id leaves.
func DeprecatedVlanID(dut *ondatra.DUTDevice) bool {
	logErrorIfFlagSet("deviation_deprecated_vlan_id")
	return lookupDUTDeviations(dut).GetDeprecatedVlanId()
}

// OSActivateNoReboot returns if device requires separate reboot to activate OS.
func OSActivateNoReboot(dut *ondatra.DUTDevice) bool {
	logErrorIfFlagSet("deviation_osactivate_noreboot")
	return lookupDUTDeviations(dut).GetOsactivateNoreboot()
}

// ConnectRetry returns if /bgp/neighbors/neighbor/timers/config/connect-retry is not supported.
func ConnectRetry(dut *ondatra.DUTDevice) bool {
	logErrorIfFlagSet("deviation_connect_retry")
	return lookupDUTDeviations(dut).GetConnectRetry()
}

// InstallOSForStandbyRP returns if device requires OS installation on standby RP as well as active RP.
func InstallOSForStandbyRP(dut *ondatra.DUTDevice) bool {
	logErrorIfFlagSet("deviation_osinstall_for_standby_rp")
	return lookupDUTDeviations(dut).GetOsinstallForStandbyRp()
}

// GNOIStatusWithEmptySubcomponent returns if the response of gNOI reboot status is a single value (not a list),
// the device requires explict component path to account for a situation when there is more than one active reboot requests.
func GNOIStatusWithEmptySubcomponent(dut *ondatra.DUTDevice) bool {
	logErrorIfFlagSet("deviation_gnoi_status_empty_subcomponent")
	return lookupDUTDeviations(dut).GetGnoiStatusEmptySubcomponent()
}

// NetworkInstanceTableDeletionRequired returns if device requires explicit deletion of network-instance table.
func NetworkInstanceTableDeletionRequired(dut *ondatra.DUTDevice) bool {
	logErrorIfFlagSet("deviation_network_instance_table_deletion_required")
	return lookupDUTDeviations(dut).GetNetworkInstanceTableDeletionRequired()
}

// ExplicitPortSpeed returns if device requires port-speed to be set because its default value may not be usable.
// Fully compliant devices selects the highest speed available based on negotiation.
func ExplicitPortSpeed(dut *ondatra.DUTDevice) bool {
	logErrorIfFlagSet("deviation_explicit_port_speed")
	return lookupDUTDeviations(dut).GetExplicitPortSpeed()
}

// ExplicitInterfaceInDefaultVRF returns if device requires explicit attachment of an interface or subinterface to the default network instance.
// OpenConfig expects an unattached interface or subinterface to be implicitly part of the default network instance.
// Fully-compliant devices should pass with and without this deviation.
func ExplicitInterfaceInDefaultVRF(dut *ondatra.DUTDevice) bool {
	logErrorIfFlagSet("deviation_explicit_interface_in_default_vrf")
	return lookupDUTDeviations(dut).GetExplicitInterfaceInDefaultVrf()
}

// InterfaceConfigVRFBeforeAddress returns if vrf should be configured before IP address when configuring interface.
func InterfaceConfigVRFBeforeAddress(dut *ondatra.DUTDevice) bool {
	logErrorIfFlagSet("deviation_interface_config_vrf_before_address")
	return lookupDUTDeviations(dut).GetInterfaceConfigVrfBeforeAddress()
}

// ExplicitInterfaceRefDefinition returns if device requires explicit interface ref configuration when applying features to interface.
func ExplicitInterfaceRefDefinition(dut *ondatra.DUTDevice) bool {
	logErrorIfFlagSet("deviation_explicit_interface_ref_definition")
	return lookupDUTDeviations(dut).GetExplicitInterfaceRefDefinition()
}

// QOSDroppedOctets returns if device should skip checking QOS Dropped octets stats for interface.
func QOSDroppedOctets(dut *ondatra.DUTDevice) bool {
	logErrorIfFlagSet("deviation_qos_dropped_octets")
	return lookupDUTDeviations(dut).GetQosDroppedOctets()
}

// ExplicitGRIBIUnderNetworkInstance returns if device requires gribi-protocol to be enabled under network-instance.
func ExplicitGRIBIUnderNetworkInstance(dut *ondatra.DUTDevice) bool {
	logErrorIfFlagSet("deviation_explicit_gribi_under_network_instance")
	return lookupDUTDeviations(dut).GetExplicitGribiUnderNetworkInstance()
}

// SkipBGPTestPasswordMismatch retuns if BGP TestPassword mismatch subtest should be skipped.
func SkipBGPTestPasswordMismatch(dut *ondatra.DUTDevice) bool {
	logErrorIfFlagSet("deviation_skip_bgp_test_password_mismatch")
	return lookupDUTDeviations(dut).GetSkipBgpTestPasswordMismatch()
}

// BGPMD5RequiresReset returns if device requires a BGP session reset to utilize a new MD5 key.
func BGPMD5RequiresReset(dut *ondatra.DUTDevice) bool {
	logErrorIfFlagSet("deviation_bgp_md5_requires_reset")
	return lookupDUTDeviations(dut).GetBgpMd5RequiresReset()
}

// ExplicitIPv6EnableForGRIBI returns if device requires Ipv6 to be enabled on interface for gRIBI NH programmed with destination mac address.
func ExplicitIPv6EnableForGRIBI(dut *ondatra.DUTDevice) bool {
	logErrorIfFlagSet("deviation_ipv6_enable_for_gribi_nh_dmac")
	return lookupDUTDeviations(dut).GetIpv6EnableForGribiNhDmac()
}

// ISISprotocolEnabledNotRequired returns if isis protocol enable flag should be unset on the device.
func ISISprotocolEnabledNotRequired(dut *ondatra.DUTDevice) bool {
	logErrorIfFlagSet("deviation_isis_protocol_enabled_not_required")
	return lookupDUTDeviations(dut).GetIsisProtocolEnabledNotRequired()
}

// ISISInstanceEnabledNotRequired returns if isis instance enable flag should not be on the device.
func ISISInstanceEnabledNotRequired(dut *ondatra.DUTDevice) bool {
	logErrorIfFlagSet("deviation_isis_instance_enabled_not_required")
	return lookupDUTDeviations(dut).GetIsisInstanceEnabledNotRequired()
}

// GNOISubcomponentPath returns if device currently uses component name instead of a full openconfig path.
func GNOISubcomponentPath(dut *ondatra.DUTDevice) bool {
	logErrorIfFlagSet("deviation_gnoi_subcomponent_path")
	return lookupDUTDeviations(dut).GetGnoiSubcomponentPath()
}

// NoMixOfTaggedAndUntaggedSubinterfaces returns if device does not support a mix of tagged and untagged subinterfaces
func NoMixOfTaggedAndUntaggedSubinterfaces(dut *ondatra.DUTDevice) bool {
	logErrorIfFlagSet("deviation_no_mix_of_tagged_and_untagged_subinterfaces")
	return lookupDUTDeviations(dut).GetNoMixOfTaggedAndUntaggedSubinterfaces()
}

// SecondaryBackupPathTrafficFailover returns if device does not support secondary backup path traffic failover
func SecondaryBackupPathTrafficFailover(dut *ondatra.DUTDevice) bool {
	logErrorIfFlagSet("deviation_secondary_backup_path_traffic_failover")
	return lookupDUTDeviations(dut).GetSecondaryBackupPathTrafficFailover()
}

// DequeueDeleteNotCountedAsDrops returns if device dequeues and deletes the pkts after a while and those are not counted
// as drops
func DequeueDeleteNotCountedAsDrops(dut *ondatra.DUTDevice) bool {
	logErrorIfFlagSet("deviation_dequeue_delete_not_counted_as_drops")
	return lookupDUTDeviations(dut).GetDequeueDeleteNotCountedAsDrops()
}

// RoutePolicyUnderAFIUnsupported returns if Route-Policy under the AFI/SAFI is not supported
func RoutePolicyUnderAFIUnsupported(dut *ondatra.DUTDevice) bool {
	logErrorIfFlagSet("deviation_route_policy_under_afi_unsupported")
	return lookupDUTDeviations(dut).GetRoutePolicyUnderAfiUnsupported()
}

// StorageComponentUnsupported returns if telemetry path /components/component/storage is not supported.
func StorageComponentUnsupported(dut *ondatra.DUTDevice) bool {
	logErrorIfFlagSet("deviation_storage_component_unsupported")
	return lookupDUTDeviations(dut).GetStorageComponentUnsupported()
}

// GNOIFabricComponentRebootUnsupported returns if device does not support use using gNOI to reboot the Fabric Component.
func GNOIFabricComponentRebootUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetGnoiFabricComponentRebootUnsupported()
}

// NtpNonDefaultVrfUnsupported returns true if the device does not support ntp nondefault vrf.
// Default value is false.
func NtpNonDefaultVrfUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetNtpNonDefaultVrfUnsupported()
}

// SkipPLQPacketsCountCheck returns if PLQ packets count check should be skipped.
// Default value is false.
func SkipPLQPacketsCountCheck(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetSkipPlqPacketsCountCheck()
}

// SkipControllerCardPowerAdmin returns if power-admin-state config on controller card should be skipped.
// Default value is false.
func SkipControllerCardPowerAdmin(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetSkipControllerCardPowerAdmin()
}

// QOSOctets returns if device should skip checking QOS octet stats for interface.
func QOSOctets(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetQosOctets()
}

// ISISInterfaceAfiUnsupported returns true for devices that don't support configuring
// ISIS /afi-safi/af/config container.
func ISISInterfaceAfiUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetIsisInterfaceAfiUnsupported()
}

// P4RTModifyTableEntryUnsupported returns true for devices that don't support
// modify table entry operation in P4 Runtime.
func P4RTModifyTableEntryUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetP4RtModifyTableEntryUnsupported()
}

// Vendor deviation flags.
// All new flags should not be exported (define them in lowercase) and accessed
// from tests through a public accessors like those above.
var (
	_ = flag.String("deviation_banner_delimiter", "",
		"Device requires the banner to have a delimiter character. Full OpenConfig compliant devices should work without delimiter.")

	_ = flag.Bool("deviation_interface_enabled", false,
		"Device requires interface enabled leaf booleans to be explicitly set to true.  Full OpenConfig compliant devices should pass both with and without this deviation.")

	_ = flag.Bool("deviation_ipv4_missing_enabled", false, "Device does not support interface/ipv4/enabled, so suppress configuring this leaf.")

	_ = flag.Bool("deviation_ip_neighbor_missing", false, "Device does not support interface/ipv4(6)/neighbor, so suppress the related check for interface/ipv4(6)/neighbor.")

	_ = flag.Bool("deviation_interface_counters_from_container", false, "Device only supports querying counters from the state container, not from individual counter leaves.")

	_ = flag.Bool("deviation_aggregate_atomic_update", false,
		"Device requires that aggregate Port-Channel and its members be defined in a single gNMI Update transaction at /interfaces; otherwise lag-type will be dropped, and no member can be added to the aggregate.  Full OpenConfig compliant devices should pass both with and without this deviation.")

	_ = flag.String("deviation_default_network_instance", "DEFAULT",
		"The name used for the default network instance for VRF.  The default name in OpenConfig is \"DEFAULT\" but some legacy devices still use \"default\".  Full OpenConfig compliant devices should be able to use any operator-assigned value.")

	_ = flag.Bool("deviation_subinterface_packet_counters_missing", false,
		"Device is missing subinterface packet counters for IPv4/IPv6, so the test will skip checking them.  Full OpenConfig compliant devices should pass both with and without this deviation.")

	_ = flag.Bool("deviation_omit_l2_mtu", false,
		"Device does not support setting the L2 MTU, so omit it.  OpenConfig allows a device to enforce that L2 MTU, which has a default value of 1514, must be set to a higher value than L3 MTU, so a full OpenConfig compliant device may fail with the deviation.")

	_ = flag.Bool("deviation_gribi_riback_only", false, "Device only supports RIB ack, so tests that normally expect FIB_ACK will allow just RIB_ACK.  Full gRIBI compliant devices should pass both with and without this deviation.")

	_ = flag.Bool("deviation_missing_value_for_defaults", false,
		"Device returns no value for some OpenConfig paths if the operational value equals the default. A fully compliant device should pass regardless of this deviation.")

	_ = flag.String("deviation_static_protocol_name", "DEFAULT", "The name used for the static routing protocol.  The default name in OpenConfig is \"DEFAULT\" but some devices use other names.")

	_ = flag.Bool("deviation_gnoi_subcomponent_path", false, "Device currently uses component name instead of a full openconfig path, so suppress creating a full oc compliant path for subcomponent.")

	_ = flag.Bool("deviation_gnoi_status_empty_subcomponent", false, "The response of gNOI reboot status is a single value (not a list), so the device requires explict component path to account for a situation when there is more than one active reboot requests.")

	_ = flag.Bool("deviation_osactivate_noreboot", false, "Device requires separate reboot to activate OS.")

	_ = flag.Bool("deviation_osinstall_for_standby_rp", false, "Device requires OS installation on standby RP as well as active RP.")

	_ = flag.Bool("deviation_deprecated_vlan_id", false, "Device requires using the deprecated openconfig-vlan:vlan/config/vlan-id or openconfig-vlan:vlan/state/vlan-id leaves.")

	_ = flag.Bool("deviation_explicit_interface_in_default_vrf", false,
		"Device requires explicit attachment of an interface or subinterface to the default network instance. OpenConfig expects an unattached interface or subinterface to be implicitly part of the default network instance. Fully-compliant devices should pass with and without this deviation.")

	_ = flag.Bool("deviation_explicit_port_speed", false, "Device requires port-speed to be set because its default value may not be usable. Fully compliant devices should select the highest speed available based on negotiation.")

	_ = flag.Bool("deviation_explicit_p4rt_node_component", false, "Device does not report P4RT node names in the component hierarchy, so use hard coded P4RT node names by passing them through internal/args flags. Fully compliant devices should report the PORT hardware components with the INTEGRATED_CIRCUIT components as their parents, as the P4RT node names.")

	_ = flag.Bool("deviation_prepolicy_received_routes", false, "Device does not support bgp/neighbors/neighbor/afi-safis/afi-safi/state/prefixes/received-pre-policy. Fully-compliant devices should pass with and without this deviation.")

	_ = flag.Bool("deviation_traceroute_l4_protocol_udp", false, "Device only support UDP as l4 protocol for traceroute. Use this flag to set default l4 protocol as UDP and skip the tests explictly use TCP or ICMP.")

	_ = flag.Bool("deviation_connect_retry", false, "Connect-retry is not supported /bgp/neighbors/neighbor/timers/config/connect-retry.")

	_ = flag.Bool("deviation_ipv6_enable_for_gribi_nh_dmac", false, "Device requires Ipv6 to be enabled on interface for gRIBI NH programmed with destination mac address")

	_ = flag.Bool("deviation_isis_interface_level1_disable_required", false,
		"Disable isis level1 under interface mode on the device if value is true, Default value is false and enables isis level2 under interface mode")

	_ = flag.Bool("deviation_missing_isis_interface_afi_safi_enable", false,
		"Set and validate isis interface address family enable on the device if value is true, Default value is false and validate isis address family enable at global mode")

	_ = flag.Bool("deviation_isis_single_topology_required", false,
		"Set isis af ipv6 single topology on the device if value is true, Default value is false and sets multi topology for isis af ipv6")

	_ = flag.Bool("deviation_isis_protocol_enabled_not_required", false,
		"Unset isis protocol enable flag on the device if value is true, Default value is false and protocol enable flag is set")

	_ = flag.Bool("deviation_isis_instance_enabled_not_required", false,
		"Don't set isis instance enable flag on the device if value is true, Default value is false and instance enable flag is set")

	_ = flag.Bool("deviation_explicit_interface_ref_definition", false, "Device requires explicit interface ref configuration when applying features to interface")

	_ = flag.Bool("deviation_no_mix_of_tagged_and_untagged_subinterfaces", false,
		"Use this deviation when the device does not support a mix of tagged and untagged subinterfaces")

	_ = flag.Bool("deviation_lldp_interface_config_override_global", false,
		"Set this flag for LLDP interface config to override the global config,expect neighbours are seen when lldp is disabled globally but enabled on interface")

	_ = flag.Bool("deviation_missing_interface_physical_channel", false,
		"Device does not support interface/physicalchannel leaf. Set this flag to skip checking the leaf.")

	_ = flag.Bool("deviation_interface_config_vrf_before_address", false, "When configuring interface, config Vrf prior config IP address")

	_ = flag.Int("deviation_bgp_tolerance_value", 0,
		"Allowed tolerance for BGP traffic flow while comparing for pass or fail condition.")

	_ = flag.Bool("deviation_explicit_gribi_under_network_instance", false,
		"Device requires gribi-protocol to be enabled under network-instance.")

	_ = flag.Bool("deviation_bgp_md5_requires_reset", false, "Device requires a BGP session reset to utilize a new MD5 key")

	_ = flag.Bool("deviation_qos_dropped_octets", false, "Set to true to skip checking QOS Dropped octets stats for interface")

	_ = flag.Bool("deviation_skip_bgp_test_password_mismatch", false,
		"Skip BGP TestPassword mismatch subtest if value is true, Default value is false")

	_ = flag.Bool("deviation_network_instance_table_deletion_required", false,
		"Set to true for device requiring explicit deletion of network-instance table, default is false")

	_ = flag.Bool("deviation_isis_multi_topology_unsupported", false,
		"Device skip isis multi-topology check if value is true, Default value is false")

	_ = flag.Bool("deviation_isis_restart_suppress_unsupported", false,
		"Device skip isis restart-suppress check if value is true, Default value is false")

	_ = flag.Bool("deviation_gribi_mac_override_with_static_arp", false, "Set to true for device not supporting programming a gribi flow with a next-hop entry of mac-address only, default is false")

	_ = flag.Bool("deviation_gribi_mac_override_static_arp_static_route", false, "Set to true for device that requires gRIBI MAC Override using Static ARP + Static Route")

	_ = flag.Bool("deviation_cli_takes_precedence_over_oc", false, "Set to true for device in which config pushed through origin CLI takes precedence over config pushed through origin OC, default is false")

	_ = flag.Bool("deviation_missing_bgp_last_notification_error_code", false, "Set to true to skip check for bgp/neighbors/neighbor/state/messages/received/last-notification-error-code leaf missing case")

	_ = flag.Bool("deviation_use_vendor_native_acl_config", false, "Configure ACLs using vendor native model specifically for RT-1.4")

	_ = flag.Bool("deviation_switch_chip_id_unsupported", false, "Device does not support id leaf for SwitchChip components. Set this flag to skip checking the leaf.")

	_ = flag.Bool("deviation_backplane_facing_capacity_unsupported", false, "Device does not support backplane-facing-capacity leaves for some of the components. Set this flag to skip checking the leaves.")

	_ = flag.Bool("deviation_scheduler_input_weight_limit", false, "device does not support weight above 100")

	_ = flag.Bool("deviation_ecn_profile_required_definition", false, "device requires additional config for ECN")

	_ = flag.Bool("deviation_isis_global_authentication_not_required", false,
		"Don't set isis global authentication-check on the device if value is true, Default value is false and ISIS global authentication-check is set")

	_ = flag.Bool("deviation_isis_explicit_level_authentication_config", false,
		"Configure CSNP, LSP and PSNP under level authentication explicitly if value is true, Default value is false to use default value for these.")

	_ = flag.Bool("deviation_ipv6_discarded_pkts_unsupported", false, "Set true for device that does not support interface ipv6 discarded packet statistics, default is false")

	_ = flag.Bool("deviation_link_qual_wait_after_delete_required", false, "Device requires additional time to complete post delete link qualification cleanup.")

	_ = flag.Bool("deviation_state_path_unsupported", false, "Device does not support these state paths, Set this flag to skip checking the leaves")

	_ = flag.Bool("deviation_drop_weight_leaves_unsupported", false, "Device does not support drop and weight leaves under queue management profile, Set this flag to skip checking the leaves")

	_ = flag.Bool("deviation_sw_version_unsupported", false, "Device does not support reporting software version according to the requirements in gNMI-1.10.")

	_ = flag.Float64("deviation_hierarchical_weight_resolution_tolerance", 0.2, "Set it to expected ucmp traffic tolerance, default is 0.2")

	_ = flag.Bool("deviation_secondary_backup_path_traffic_failover", false, "Device does not support traffic forward with secondary backup path failover")

	_ = flag.Bool("deviation_dequeue_delete_not_counted_as_drops", false, "devices do not count dequeued and deleted packets as drops, default is false")

	_ = flag.Bool("deviation_route_policy_under_afi_unsupported", false, "Set true for device that does not support route-policy under AFI/SAFI, default is false")

	_ = flag.Bool("deviation_storage_component_unsupported", false, "Set to true for device that does not support telemetry path /components/component/storage")
)
