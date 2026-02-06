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
//   - Deviations should be small in scope, typically affecting one subtest, one
//     OpenConfig path or small OpenConfig subtree.
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

	log "github.com/golang/glog"
	"github.com/openconfig/featureprofiles/internal/metadata"

	mpb "github.com/openconfig/featureprofiles/proto/metadata_go_proto"
	"github.com/openconfig/ondatra"
)

func lookupDeviations(dvc *ondatra.Device) (*mpb.Metadata_PlatformExceptions, error) {
	var matchedPlatformException *mpb.Metadata_PlatformExceptions

	for _, platformExceptions := range metadata.Get().GetPlatformExceptions() {
		if platformExceptions.GetPlatform().GetVendor().String() == "" {
			return nil, fmt.Errorf("vendor should be specified in textproto %v", platformExceptions)
		}

		if dvc.Vendor().String() != platformExceptions.GetPlatform().GetVendor().String() {
			continue
		}

		// If hardware_model_regex is set and does not match, continue
		if hardwareModelRegex := platformExceptions.GetPlatform().GetHardwareModelRegex(); hardwareModelRegex != "" {
			matchHw, errHw := regexp.MatchString(hardwareModelRegex, dvc.Model())
			if errHw != nil {
				return nil, fmt.Errorf("error with regex match %v", errHw)
			}
			if !matchHw {
				continue
			}
		}

		// If software_version_regex is set and does not match, continue
		if softwareVersionRegex := platformExceptions.GetPlatform().GetSoftwareVersionRegex(); softwareVersionRegex != "" {
			matchSw, errSw := regexp.MatchString(softwareVersionRegex, dvc.Version())
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

func mustLookupDeviations(dvc *ondatra.Device) *mpb.Metadata_Deviations {
	platformExceptions, err := lookupDeviations(dvc)
	if err != nil {
		log.Exitf("Error looking up deviations: %v", err)
	}
	if platformExceptions == nil {
		log.Infof("Did not match any platform_exception %v, returning default values", metadata.Get().GetPlatformExceptions())
		return &mpb.Metadata_Deviations{}
	}
	return platformExceptions.GetDeviations()
}

func lookupDUTDeviations(dut *ondatra.DUTDevice) *mpb.Metadata_Deviations {
	return mustLookupDeviations(dut.Device)
}

func lookupATEDeviations(ate *ondatra.ATEDevice) *mpb.Metadata_Deviations {
	return mustLookupDeviations(ate.Device)
}

// BannerDelimiter returns if device requires the banner to have a delimiter character.
// Full OpenConfig compliant devices should work without delimiter.
func BannerDelimiter(dut *ondatra.DUTDevice) string {
	return lookupDUTDeviations(dut).GetBannerDelimiter()
}

// OmitL2MTU returns if device does not support setting the L2 MTU.
func OmitL2MTU(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetOmitL2Mtu()
}

// GRIBIMACOverrideStaticARPStaticRoute returns whether the device needs to configure Static ARP + Static Route to override setting MAC address in Next Hop.
func GRIBIMACOverrideStaticARPStaticRoute(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetGribiMacOverrideStaticArpStaticRoute()
}

// AggregateAtomicUpdate returns if device requires that aggregate Port-Channel and its members be defined in a single gNMI Update transaction at /interfaces,
// Otherwise lag-type will be dropped, and no member can be added to the aggregate.
// Full OpenConfig compliant devices should pass both with and without this deviation.
func AggregateAtomicUpdate(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetAggregateAtomicUpdate()
}

// DefaultNetworkInstance returns the name used for the default network instance for VRF.
func DefaultNetworkInstance(dut *ondatra.DUTDevice) string {
	if dni := lookupDUTDeviations(dut).GetDefaultNetworkInstance(); dni != "" {
		return dni
	}
	return "DEFAULT"
}

// ISISRestartSuppressUnsupported returns whether the device should skip isis restart-suppress check.
func ISISRestartSuppressUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetIsisRestartSuppressUnsupported()
}

// BgpGrHelperDisableUnsupported returns whether the device does not support to disable BGP GR Helper.
func BgpGrHelperDisableUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetBgpGrHelperDisableUnsupported()
}

// BgpGracefulRestartUnderAfiSafiUnsupported returns whether the device does not support bgp GR-RESTART under AFI/SAFI.
func BgpGracefulRestartUnderAfiSafiUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetBgpGracefulRestartUnderAfiSafiUnsupported()
}

// MissingBgpLastNotificationErrorCode returns whether the last-notification-error-code leaf is missing in bgp.
func MissingBgpLastNotificationErrorCode(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetMissingBgpLastNotificationErrorCode()
}

// GRIBIMACOverrideWithStaticARP returns whether for a gRIBI IPv4 route the device does not support a mac-address only next-hop-entry.
func GRIBIMACOverrideWithStaticARP(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetGribiMacOverrideWithStaticArp()
}

// CLITakesPrecedenceOverOC returns whether config pushed through origin CLI takes precedence over config pushed through origin OC.
func CLITakesPrecedenceOverOC(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetCliTakesPrecedenceOverOc()
}

// BGPTrafficTolerance returns the allowed tolerance for BGP traffic flow while comparing for pass or fail conditions.
func BGPTrafficTolerance(dut *ondatra.DUTDevice) int32 {
	return lookupDUTDeviations(dut).GetBgpToleranceValue()
}

// StaticProtocolName returns the name used for the static routing protocol.
func StaticProtocolName(dut *ondatra.DUTDevice) string {
	if spn := lookupDUTDeviations(dut).GetStaticProtocolName(); spn != "" {
		return spn
	}
	return "DEFAULT"
}

// SwitchChipIDUnsupported returns whether the device supports id leaf for SwitchChip components.
func SwitchChipIDUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetSwitchChipIdUnsupported()
}

// BackplaneFacingCapacityUnsupported returns whether the device supports backplane-facing-capacity leaves for some components.
func BackplaneFacingCapacityUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetBackplaneFacingCapacityUnsupported()
}

// SchedulerInputWeightLimit returns whether the device does not support weight above 100.
func SchedulerInputWeightLimit(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetSchedulerInputWeightLimit()
}

// ECNProfileRequiredDefinition returns whether the device requires additional config for ECN.
func ECNProfileRequiredDefinition(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetEcnProfileRequiredDefinition()
}

// ISISGlobalAuthenticationNotRequired returns true if ISIS Global authentication not required.
func ISISGlobalAuthenticationNotRequired(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetIsisGlobalAuthenticationNotRequired()
}

// ISISExplicitLevelAuthenticationConfig returns true if ISIS Explicit Level Authentication configuration is required
func ISISExplicitLevelAuthenticationConfig(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetIsisExplicitLevelAuthenticationConfig()
}

// ISISSingleTopologyRequired sets isis af ipv6 single topology on the device if value is true.
func ISISSingleTopologyRequired(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetIsisSingleTopologyRequired()
}

// ISISMultiTopologyUnsupported returns if device skips isis multi-topology check.
func ISISMultiTopologyUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetIsisMultiTopologyUnsupported()
}

// ISISInterfaceLevel1DisableRequired returns if device should disable isis level1 under interface mode.
func ISISInterfaceLevel1DisableRequired(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetIsisInterfaceLevel1DisableRequired()
}

// MissingIsisInterfaceAfiSafiEnable returns if device should set and validate isis interface address family enable.
// Default is validate isis address family enable at global mode.
func MissingIsisInterfaceAfiSafiEnable(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetMissingIsisInterfaceAfiSafiEnable()
}

// Ipv6DiscardedPktsUnsupported returns whether the device supports interface ipv6 discarded packet stats.
func Ipv6DiscardedPktsUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetIpv6DiscardedPktsUnsupported()
}

// LinkQualWaitAfterDeleteRequired returns whether the device requires additional time to complete post delete link qualification cleanup.
func LinkQualWaitAfterDeleteRequired(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetLinkQualWaitAfterDeleteRequired()
}

// StatePathsUnsupported returns whether the device supports following state paths
func StatePathsUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetStatePathUnsupported()
}

// DropWeightLeavesUnsupported returns whether the device supports drop and weight leaves under queue management profile.
func DropWeightLeavesUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetDropWeightLeavesUnsupported()
}

// SwVersionUnsupported returns true if the device does not support reporting software version according to the requirements in gNMI-1.10.
func SwVersionUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetSwVersionUnsupported()
}

// HierarchicalWeightResolutionTolerance returns the allowed tolerance for BGP traffic flow while comparing for pass or fail conditions.
// Default minimum value is 0.2. Anything less than 0.2 will be set to 0.2.
func HierarchicalWeightResolutionTolerance(dut *ondatra.DUTDevice) float64 {
	hwrt := lookupDUTDeviations(dut).GetHierarchicalWeightResolutionTolerance()
	if minHWRT := 0.2; hwrt < minHWRT {
		return minHWRT
	}
	return hwrt
}

// InterfaceEnabled returns if device requires interface enabled leaf booleans to be explicitly set to true.
func InterfaceEnabled(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetInterfaceEnabled()
}

// InterfaceCountersFromContainer returns if the device only supports querying counters from the state container, not from individual counter leaves.
func InterfaceCountersFromContainer(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetInterfaceCountersFromContainer()
}

// IPv4MissingEnabled returns if device does not support interface/ipv4/enabled.
func IPv4MissingEnabled(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetIpv4MissingEnabled()
}

// IPNeighborMissing returns true if the device does not support interface/ipv4(6)/neighbor,
// so test can suppress the related check for interface/ipv4(6)/neighbor.
func IPNeighborMissing(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetIpNeighborMissing()
}

// GRIBIRIBAckOnly returns if device only supports RIB ack, so tests that normally expect FIB_ACK will allow just RIB_ACK.
// Full gRIBI compliant devices should pass both with and without this deviation.
func GRIBIRIBAckOnly(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetGribiRibackOnly()
}

// MissingValueForDefaults returns if device returns no value for some OpenConfig paths if the operational value equals the default.
func MissingValueForDefaults(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetMissingValueForDefaults()
}

// TraceRouteL4ProtocolUDP returns if device only support UDP as l4 protocol for traceroute.
// Default value is false.
func TraceRouteL4ProtocolUDP(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetTracerouteL4ProtocolUdp()
}

// LLDPInterfaceConfigOverrideGlobal returns if LLDP interface config should override the global config,
// expect neighbours are seen when lldp is disabled globally but enabled on interface
func LLDPInterfaceConfigOverrideGlobal(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetLldpInterfaceConfigOverrideGlobal()
}

// SubinterfacePacketCountersMissing returns if device is missing subinterface packet counters for IPv4/IPv6,
// so the test will skip checking them.
// Full OpenConfig compliant devices should pass both with and without this deviation.
// Nokia https://b.corp.google.com/issues/4778051566
func SubinterfacePacketCountersMissing(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetSubinterfacePacketCountersMissing()
}

// MissingPrePolicyReceivedRoutes returns if device does not support bgp/neighbors/neighbor/afi-safis/afi-safi/state/prefixes/received-pre-policy.
// Fully-compliant devices should pass with and without this deviation.
func MissingPrePolicyReceivedRoutes(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetPrepolicyReceivedRoutes()
}

// DeprecatedVlanID returns if device requires using the deprecated openconfig-vlan:vlan/config/vlan-id or openconfig-vlan:vlan/state/vlan-id leaves.
func DeprecatedVlanID(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetDeprecatedVlanId()
}

// OSActivateNoReboot returns if device requires separate reboot to activate OS.
func OSActivateNoReboot(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetOsactivateNoreboot()
}

// ConnectRetry returns if /bgp/neighbors/neighbor/timers/config/connect-retry is not supported.
func ConnectRetry(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetConnectRetry()
}

// InstallOSForStandbyRP returns if device requires OS installation on standby RP as well as active RP.
func InstallOSForStandbyRP(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetOsinstallForStandbyRp()
}

// GNOIStatusWithEmptySubcomponent returns if the response of gNOI reboot status is a single value (not a list),
// the device requires explicit component path to account for a situation when there is more than one active reboot requests.
func GNOIStatusWithEmptySubcomponent(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetGnoiStatusEmptySubcomponent()
}

// NetworkInstanceTableDeletionRequired returns if device requires explicit deletion of network-instance table.
func NetworkInstanceTableDeletionRequired(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetNetworkInstanceTableDeletionRequired()
}

// ExplicitPortSpeed returns if device requires port-speed to be set because its default value may not be usable.
// Fully compliant devices selects the highest speed available based on negotiation.
func ExplicitPortSpeed(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetExplicitPortSpeed()
}

// ExplicitInterfaceInDefaultVRF returns if device requires explicit attachment of an interface or subinterface to the default network instance.
// OpenConfig expects an unattached interface or subinterface to be implicitly part of the default network instance.
// Fully-compliant devices should pass with and without this deviation.
func ExplicitInterfaceInDefaultVRF(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetExplicitInterfaceInDefaultVrf()
}

// RibWecmp returns if device requires CLI knob to enable wecmp feature.
func RibWecmp(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetRibWecmp()
}

// InterfaceConfigVRFBeforeAddress returns if vrf should be configured before IP address when configuring interface.
func InterfaceConfigVRFBeforeAddress(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetInterfaceConfigVrfBeforeAddress()
}

// BGPMD5RequiresReset returns if device requires a BGP session reset to utilize a new MD5 key.
func BGPMD5RequiresReset(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetBgpMd5RequiresReset()
}

// ExplicitIPv6EnableForGRIBI returns if device requires Ipv6 to be enabled on interface for gRIBI NH programmed with destination mac address.
func ExplicitIPv6EnableForGRIBI(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetIpv6EnableForGribiNhDmac()
}

// ISISInstanceEnabledRequired returns if isis instance name string should be set on the device.
func ISISInstanceEnabledRequired(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetIsisInstanceEnabledRequired()
}

// GNOISubcomponentPath returns if device currently uses component name instead of a full openconfig path.
func GNOISubcomponentPath(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetGnoiSubcomponentPath()
}

// NoMixOfTaggedAndUntaggedSubinterfaces returns if device does not support a mix of tagged and untagged subinterfaces
func NoMixOfTaggedAndUntaggedSubinterfaces(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetNoMixOfTaggedAndUntaggedSubinterfaces()
}

// DequeueDeleteNotCountedAsDrops returns if device dequeues and deletes the pkts after a while and those are not counted
// as drops
func DequeueDeleteNotCountedAsDrops(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetDequeueDeleteNotCountedAsDrops()
}

// RoutePolicyUnderAFIUnsupported returns if Route-Policy under the AFI/SAFI is not supported
func RoutePolicyUnderAFIUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetRoutePolicyUnderAfiUnsupported()
}

// StorageComponentUnsupported returns if telemetry path /components/component/storage is not supported.
func StorageComponentUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetStorageComponentUnsupported()
}

// GNOIFabricComponentRebootUnsupported returns if device does not support use using gNOI to reboot the Fabric Component.
func GNOIFabricComponentRebootUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetGnoiFabricComponentRebootUnsupported()
}

// NtpNonDefaultVrfUnsupported returns true if the device does not support ntp non-default vrf.
// Default value is false.
func NtpNonDefaultVrfUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetNtpNonDefaultVrfUnsupported()
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

// OSComponentParentIsSupervisorOrLinecard returns true if parent of OS component is
// of type SUPERVISOR or LINECARD.
func OSComponentParentIsSupervisorOrLinecard(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetOsComponentParentIsSupervisorOrLinecard()
}

// OSComponentParentIsChassis returns true if parent of OS component is of type CHASSIS.
func OSComponentParentIsChassis(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetOsComponentParentIsChassis()
}

// ISISRequireSameL1MetricWithL2Metric returns true for devices that require configuring
// the same ISIS Metrics for Level 1 when configuring Level 2 Metrics.
func ISISRequireSameL1MetricWithL2Metric(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetIsisRequireSameL1MetricWithL2Metric()
}

// BGPSetMedRequiresEqualOspfSetMetric returns true for devices that require configuring
// the same OSPF setMetric when BGP SetMED is configured.
func BGPSetMedRequiresEqualOspfSetMetric(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetBgpSetMedRequiresEqualOspfSetMetric()
}

// SetNativeUser creates a user and assigns role/rbac to that user via native model.
func SetNativeUser(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetSetNativeUser()
}

// P4RTGdpRequiresDot1QSubinterface returns true for devices that require configuring
// subinterface with tagged vlan for P4RT packet in.
func P4RTGdpRequiresDot1QSubinterface(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetP4RtGdpRequiresDot1QSubinterface()
}

// LinecardCPUUtilizationUnsupported returns if the device does not support telemetry path
// /components/component/cpu/utilization/state/avg for linecards' CPU card.
// Default value is false.
func LinecardCPUUtilizationUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetLinecardCpuUtilizationUnsupported()
}

// ConsistentComponentNamesUnsupported returns if the device does not support consistent component names for GNOI and GNMI.
// Default value is false.
func ConsistentComponentNamesUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetConsistentComponentNamesUnsupported()
}

// ControllerCardCPUUtilizationUnsupported returns if the device does not support telemetry path
// /components/component/cpu/utilization/state/avg for controller cards' CPU card.
// Default value is false.
func ControllerCardCPUUtilizationUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetControllerCardCpuUtilizationUnsupported()
}

// FabricDropCounterUnsupported returns if the device does not support counter for fabric block lost packets.
// Default value is false.
func FabricDropCounterUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetFabricDropCounterUnsupported()
}

// LinecardMemoryUtilizationUnsupported returns if the device does not support memory utilization related leaves for linecard components.
// Default value is false.
func LinecardMemoryUtilizationUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetLinecardMemoryUtilizationUnsupported()
}

// QOSVoqDropCounterUnsupported returns if the device does not support telemetry path
// /qos/interfaces/interface/input/virtual-output-queues/voq-interface/queues/queue/state/dropped-pkts.
// Default value is false.
func QOSVoqDropCounterUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetQosVoqDropCounterUnsupported()
}

// ISISTimersCsnpIntervalUnsupported returns true for devices that do not support
// configuring csnp-interval timer for ISIS.
func ISISTimersCsnpIntervalUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetIsisTimersCsnpIntervalUnsupported()
}

// ISISCounterManualAddressDropFromAreasUnsupported returns true for devices that do not
// support telemetry for isis system-level-counter manual-address-drop-from-areas.
func ISISCounterManualAddressDropFromAreasUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetIsisCounterManualAddressDropFromAreasUnsupported()
}

// ISISCounterPartChangesUnsupported returns true for devices that do not
// support telemetry for isis system-level-counter part-changes.
func ISISCounterPartChangesUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetIsisCounterPartChangesUnsupported()
}

// SkipTCPNegotiatedMSSCheck returns true for devices that do not
// support telemetry to check negotiated tcp mss value.
func SkipTCPNegotiatedMSSCheck(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetSkipTcpNegotiatedMssCheck()
}

// TransceiverThresholdsUnsupported returns true if the device does not support threshold container under /components/component/transceiver.
// Default value is false.
func TransceiverThresholdsUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetTransceiverThresholdsUnsupported()
}

// InterfaceLoopbackModeRawGnmi returns true if interface loopback mode needs to be updated using raw gnmi API due to server version.
// Default value is false.
func InterfaceLoopbackModeRawGnmi(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetInterfaceLoopbackModeRawGnmi()
}

// ISISLspMetadataLeafsUnsupported returns true for devices that don't support ISIS-Lsp
// metadata paths: checksum, sequence-number, remaining-lifetime.
func ISISLspMetadataLeafsUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetIsisLspMetadataLeafsUnsupported()
}

// QOSQueueRequiresID returns if device should configure QOS queue along with queue-id
func QOSQueueRequiresID(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetQosQueueRequiresId()
}

// BgpLlgrOcUndefined returns true if device does not support OC path to disable BGP LLGR.
func BgpLlgrOcUndefined(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetBgpLlgrOcUndefined()
}

// QOSBufferAllocationConfigRequired returns if device should configure QOS buffer-allocation-profile
func QOSBufferAllocationConfigRequired(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetQosBufferAllocationConfigRequired()
}

// BGPGlobalExtendedNextHopEncodingUnsupported returns true for devices that do not support configuring
// BGP ExtendedNextHopEncoding at the global level.
func BGPGlobalExtendedNextHopEncodingUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetBgpGlobalExtendedNextHopEncodingUnsupported()
}

// TunnelStatePathUnsupported returns true for devices that require configuring
// /interfaces/interface/state/counters/in-pkts, in-octets,out-pkts, out-octetsis not supported.
func TunnelStatePathUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetTunnelStatePathUnsupported()
}

// TunnelConfigPathUnsupported returns true for devices that require configuring
// Tunnel source-address destination-address, encapsulation type are not supported in OC
func TunnelConfigPathUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetTunnelConfigPathUnsupported()
}

// EcnSameMinMaxThresholdUnsupported returns true for devices that don't support the same minimum and maximum threshold values
// CISCO: minimum and maximum threshold values are not the same, the difference between minimum and maximum threshold value should be 6144.
func EcnSameMinMaxThresholdUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetEcnSameMinMaxThresholdUnsupported()
}

// QosSchedulerConfigRequired returns if device should configure QOS buffer-allocation-profile
func QosSchedulerConfigRequired(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetQosSchedulerConfigRequired()
}

// QosSetWeightConfigUnsupported returns whether the device does not support set weight leaves under qos ecn.
func QosSetWeightConfigUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetQosSetWeightConfigUnsupported()
}

// QosGetStatePathUnsupported returns whether the device does not support get state leaves under qos.
func QosGetStatePathUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetQosGetStatePathUnsupported()
}

// InterfaceRefInterfaceIDFormat returns if device is required to use interface-id format of interface name + .subinterface index with Interface-ref container
func InterfaceRefInterfaceIDFormat(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetInterfaceRefInterfaceIdFormat()
}

// ISISLevelEnabled returns if device should enable isis under level.
func ISISLevelEnabled(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetIsisLevelEnabled()
}

// MemberLinkLoopbackUnsupported returns true for devices that require configuring
// loopback on aggregated links instead of member links.
func MemberLinkLoopbackUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetMemberLinkLoopbackUnsupported()
}

// SkipPlqInterfaceOperStatusCheck returns true for devices that do not support
// PLQ operational status check for interfaces
func SkipPlqInterfaceOperStatusCheck(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetSkipPlqInterfaceOperStatusCheck()
}

// BGPExplicitPrefixLimitReceived returns if device must specify the received prefix limits explicitly
// under the "prefix-limit-received" field rather than simply "prefix-limit".
func BGPExplicitPrefixLimitReceived(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetBgpExplicitPrefixLimitReceived()
}

// BGPMissingOCMaxPrefixesConfiguration returns true for devices that does not configure BGP
// maximum routes correctly when max-prefixes OC leaf is configured.
func BGPMissingOCMaxPrefixesConfiguration(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetBgpMissingOcMaxPrefixesConfiguration()
}

// SkipBgpSessionCheckWithoutAfisafi returns if device needs to skip checking AFI-SAFI disable.
func SkipBgpSessionCheckWithoutAfisafi(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetSkipBgpSessionCheckWithoutAfisafi()
}

// MismatchedHardwareResourceNameInComponent returns true for devices that have separate
// naming conventions for hardware resource name in /system/ tree and /components/ tree.
func MismatchedHardwareResourceNameInComponent(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetMismatchedHardwareResourceNameInComponent()
}

// GNOISubcomponentRebootStatusUnsupported returns true for devices that do not support subcomponent reboot status check.
func GNOISubcomponentRebootStatusUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetGnoiSubcomponentRebootStatusUnsupported()
}

// SkipNonBgpRouteExportCheck returns true for devices that exports routes from all
// protocols to BGP if the export-policy is ACCEPT.
func SkipNonBgpRouteExportCheck(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetSkipNonBgpRouteExportCheck()
}

// ISISMetricStyleTelemetryUnsupported returns true for devices that do not support state path
// /network-instances/network-instance/protocols/protocol/isis/levels/level/state/metric-style
func ISISMetricStyleTelemetryUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetIsisMetricStyleTelemetryUnsupported()
}

// StaticRouteNextHopInterfaceRefUnsupported returns if device does not support Interface-ref under static-route next-hop
func StaticRouteNextHopInterfaceRefUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetStaticRouteNextHopInterfaceRefUnsupported()
}

// SkipStaticNexthopCheck returns if device needs index starting from non-zero
func SkipStaticNexthopCheck(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetSkipStaticNexthopCheck()
}

// Ipv6RouterAdvertisementConfigUnsupported returns true for devices which don't support Ipv6 RouterAdvertisement configuration
func Ipv6RouterAdvertisementConfigUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetIpv6RouterAdvertisementConfigUnsupported()
}

// PrefixLimitExceededTelemetryUnsupported is to skip checking prefix limit telemetry flag.
func PrefixLimitExceededTelemetryUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetPrefixLimitExceededTelemetryUnsupported()
}

// SkipSettingAllowMultipleAS return true if device needs to skip setting allow-multiple-as while configuring eBGP
func SkipSettingAllowMultipleAS(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetSkipSettingAllowMultipleAs()
}

// GribiDecapMixedPlenUnsupported returns true if devices does not support
// programming with mixed prefix length.
func GribiDecapMixedPlenUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetGribiDecapMixedPlenUnsupported()
}

// SkipIsisSetLevel return true if device needs to skip setting isis-actions set-level while configuring routing-policy statement action
func SkipIsisSetLevel(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetSkipIsisSetLevel()
}

// SkipIsisSetMetricStyleType return true if device needs to skip setting isis-actions set-metric-style-type while configuring routing-policy statement action
func SkipIsisSetMetricStyleType(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetSkipIsisSetMetricStyleType()
}

// SkipSettingDisableMetricPropagation return true if device needs to skip setting disable-metric-propagation while configuring table-connection
func SkipSettingDisableMetricPropagation(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetSkipSettingDisableMetricPropagation()
}

// BGPConditionsMatchCommunitySetUnsupported returns true if device doesn't support bgp-conditions/match-community-set leaf
func BGPConditionsMatchCommunitySetUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetBgpConditionsMatchCommunitySetUnsupported()
}

// PfRequireMatchDefaultRule returns true for device which requires match condition for ether type v4 and v6 for default rule with network-instance default-vrf in policy-forwarding.
func PfRequireMatchDefaultRule(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetPfRequireMatchDefaultRule()
}

// MissingPortToOpticalChannelMapping returns true for devices missing component tree mapping from hardware port to optical channel.
func MissingPortToOpticalChannelMapping(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetMissingPortToOpticalChannelComponentMapping()
}

// SkipContainerOp returns true if gNMI container OP needs to be skipped.
// Cisco: https://partnerissuetracker.corp.google.com/issues/322291556
func SkipContainerOp(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetSkipContainerOp()
}

// ReorderCallsForVendorCompatibilty returns true if call needs to be updated/added/deleted.
// Cisco: https://partnerissuetracker.corp.google.com/issues/322291556
func ReorderCallsForVendorCompatibilty(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetReorderCallsForVendorCompatibilty()
}

// AddMissingBaseConfigViaCli returns true if missing base config needs to be added using CLI.
// Cisco: https://partnerissuetracker.corp.google.com/issues/322291556
func AddMissingBaseConfigViaCli(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetAddMissingBaseConfigViaCli()
}

// SkipMacaddressCheck returns true if mac address for an interface via gNMI needs to be skipped.
// Cisco: https://partnerissuetracker.corp.google.com/issues/322291556
func SkipMacaddressCheck(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetSkipMacaddressCheck()
}

// BGPRibOcPathUnsupported returns true if BGP RIB OC telemetry path is not supported.
func BGPRibOcPathUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetBgpRibOcPathUnsupported()
}

// SkipPrefixSetMode return true if device needs to skip setting prefix-set mode while configuring prefix-set routing-policy
func SkipPrefixSetMode(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetSkipPrefixSetMode()
}

// SetMetricAsPreference returns true for devices which set metric as
// preference for static next-hop
func SetMetricAsPreference(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetSetMetricAsPreference()
}

// IPv6StaticRouteWithIPv4NextHopRequiresStaticARP returns true if devices don't support having an
// IPv6 static Route with an IPv4 address as next hop and requires configuring a static ARP entry.
// Arista: https://partnerissuetracker.corp.google.com/issues/316593298
func IPv6StaticRouteWithIPv4NextHopRequiresStaticARP(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetIpv6StaticRouteWithIpv4NextHopRequiresStaticArp()
}

// PfRequireSequentialOrderPbrRules returns true for device requires policy-forwarding rules to be in sequential order in the gNMI set-request.
func PfRequireSequentialOrderPbrRules(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetPfRequireSequentialOrderPbrRules()
}

// MissingStaticRouteNextHopMetricTelemetry returns true for devices missing
// static route next-hop metric telemetry.
// Arista: https://partnerissuetracker.corp.google.com/issues/321010782
func MissingStaticRouteNextHopMetricTelemetry(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetMissingStaticRouteNextHopMetricTelemetry()
}

// UnsupportedStaticRouteNextHopRecurse returns true for devices that don't support recursive
// resolution of static route next hop.
// Arista: https://partnerissuetracker.corp.google.com/issues/314449182
func UnsupportedStaticRouteNextHopRecurse(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetUnsupportedStaticRouteNextHopRecurse()
}

// MissingStaticRouteDropNextHopTelemetry returns true for devices missing
// static route telemetry with DROP next hop.
// Arista: https://partnerissuetracker.corp.google.com/issues/330619816
func MissingStaticRouteDropNextHopTelemetry(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetMissingStaticRouteDropNextHopTelemetry()
}

// MissingZROpticalChannelTunableParametersTelemetry returns true for devices missing 400ZR
// optical-channel tunable parameters telemetry: min/max/avg.
// Arista: https://partnerissuetracker.corp.google.com/issues/319314781
func MissingZROpticalChannelTunableParametersTelemetry(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetMissingZrOpticalChannelTunableParametersTelemetry()
}

// PLQReflectorStatsUnsupported returns true for devices that does not support packet link qualification(PLQ) reflector packet sent/received stats.
func PLQReflectorStatsUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetPlqReflectorStatsUnsupported()
}

// PLQGeneratorCapabilitiesMaxMTU returns supported max_mtu for devices that does not support packet link qualification(PLQ) Generator max_mtu to be at least >= 8184.
func PLQGeneratorCapabilitiesMaxMTU(dut *ondatra.DUTDevice) uint32 {
	return lookupDUTDeviations(dut).GetPlqGeneratorCapabilitiesMaxMtu()
}

// PLQGeneratorCapabilitiesMaxPPS returns supported max_pps for devices that does not support packet link qualification(PLQ) Generator max_pps to be at least >= 100000000.
func PLQGeneratorCapabilitiesMaxPPS(dut *ondatra.DUTDevice) uint64 {
	return lookupDUTDeviations(dut).GetPlqGeneratorCapabilitiesMaxPps()
}

// BgpExtendedCommunityIndexUnsupported return true if BGP extended community index is not supported.
func BgpExtendedCommunityIndexUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetBgpExtendedCommunityIndexUnsupported()
}

// BgpCommunitySetRefsUnsupported return true if BGP community set refs is not supported.
func BgpCommunitySetRefsUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetBgpCommunitySetRefsUnsupported()
}

// TableConnectionsUnsupported returns true if Table Connections are unsupported.
func TableConnectionsUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetTableConnectionsUnsupported()
}

// UseVendorNativeTagSetConfig returns whether a device requires native model to configure tag-set
func UseVendorNativeTagSetConfig(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetUseVendorNativeTagSetConfig()
}

// SkipBgpSendCommunityType return true if device needs to skip setting BGP send-community-type
func SkipBgpSendCommunityType(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetSkipBgpSendCommunityType()
}

// BgpActionsSetCommunityMethodUnsupported return true if BGP actions set-community method is unsupported
func BgpActionsSetCommunityMethodUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetBgpActionsSetCommunityMethodUnsupported()

}

// SetNoPeerGroup Ensure that no BGP configurations exists under PeerGroups.
func SetNoPeerGroup(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetSetNoPeerGroup()
}

// BgpCommunityMemberIsAString returns true if device community member is not a list
func BgpCommunityMemberIsAString(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetBgpCommunityMemberIsAString()
}

// IPv4StaticRouteWithIPv6NextHopUnsupported unsupported ipv4 with ipv6 nexthop
func IPv4StaticRouteWithIPv6NextHopUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetIpv4StaticRouteWithIpv6NhUnsupported()
}

// IPv6StaticRouteWithIPv4NextHopUnsupported unsupported ipv6 with ipv4 nexthop
func IPv6StaticRouteWithIPv4NextHopUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetIpv6StaticRouteWithIpv4NhUnsupported()
}

// StaticRouteWithDropNhUnsupported unsupported drop nexthop
func StaticRouteWithDropNhUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetStaticRouteWithDropNh()
}

// StaticRouteWithExplicitMetric set explicit metric
func StaticRouteWithExplicitMetric(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetStaticRouteWithExplicitMetric()
}

// BgpDefaultPolicyUnsupported return true if BGP default-import/export-policy is not supported.
func BgpDefaultPolicyUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetBgpDefaultPolicyUnsupported()
}

// ExplicitEnableBGPOnDefaultVRF return true if BGP needs to be explicitly enabled on default VRF
func ExplicitEnableBGPOnDefaultVRF(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetExplicitEnableBgpOnDefaultVrf()
}

// RoutingPolicyTagSetEmbedded returns true if the implementation does not support tag-set(s) as a
// separate entity, but embeds it in the policy statement
func RoutingPolicyTagSetEmbedded(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetRoutingPolicyTagSetEmbedded()
}

// SkipAfiSafiPathForBgpMultipleAs return true if device do not support afi/safi path to enable allow multiple-as for eBGP
func SkipAfiSafiPathForBgpMultipleAs(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetSkipAfiSafiPathForBgpMultipleAs()
}

// CommunityMemberRegexUnsupported return true if device do not support community member regex
func CommunityMemberRegexUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetCommunityMemberRegexUnsupported()
}

// SamePolicyAttachedToAllAfis returns true if same import policy has to be applied for all AFIs
func SamePolicyAttachedToAllAfis(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetSamePolicyAttachedToAllAfis()
}

// SkipSettingStatementForPolicy return true if device do not support afi/safi path to enable allow multiple-as for eBGP
func SkipSettingStatementForPolicy(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetSkipSettingStatementForPolicy()
}

// SkipCheckingAttributeIndex return true if device do not return bgp attribute for the bgp session specifying the index
func SkipCheckingAttributeIndex(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetSkipCheckingAttributeIndex()
}

// FlattenPolicyWithMultipleStatements return true if devices does not support policy-chaining
func FlattenPolicyWithMultipleStatements(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetFlattenPolicyWithMultipleStatements()
}

// SlaacPrefixLength128 for Slaac generated IPv6 link local address
func SlaacPrefixLength128(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetSlaacPrefixLength128()
}

// DefaultRoutePolicyUnsupported returns true if default route policy is not supported
func DefaultRoutePolicyUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetDefaultRoutePolicyUnsupported()
}

// CommunityMatchWithRedistributionUnsupported is set to true for devices that do not support matching community at the redistribution attach point.
func CommunityMatchWithRedistributionUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetCommunityMatchWithRedistributionUnsupported()
}

// BgpMaxMultipathPathsUnsupported returns true if the device does not support
// bgp max multipaths.
func BgpMaxMultipathPathsUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetBgpMaxMultipathPathsUnsupported()
}

// MultipathUnsupportedNeighborOrAfisafi returns true if the device does not
// support multipath under neighbor or afisafi.
func MultipathUnsupportedNeighborOrAfisafi(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetMultipathUnsupportedNeighborOrAfisafi()
}

// ModelNameUnsupported returns true if /components/components/state/model-name
// is not supported for any component type.
func ModelNameUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetModelNameUnsupported()
}

// InstallPositionAndInstallComponentUnsupported returns true if install
// position and install component are not supported.
func InstallPositionAndInstallComponentUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetInstallPositionAndInstallComponentUnsupported()
}

// EncapTunnelShutBackupNhgZeroTraffic returns true when encap tunnel is shut then zero traffic flows to back-up NHG
func EncapTunnelShutBackupNhgZeroTraffic(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetEncapTunnelShutBackupNhgZeroTraffic()
}

// MaxEcmpPaths supported for isis max ecmp path
func MaxEcmpPaths(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetMaxEcmpPaths()
}

// WecmpAutoUnsupported returns true if wecmp auto is not supported
func WecmpAutoUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetWecmpAutoUnsupported()
}

// RoutingPolicyChainingUnsupported returns true if policy chaining is unsupported
func RoutingPolicyChainingUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetRoutingPolicyChainingUnsupported()
}

// ISISLoopbackRequired returns true if isis loopback is required.
func ISISLoopbackRequired(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetIsisLoopbackRequired()
}

// WeightedEcmpFixedPacketVerification returns true if fixed packet is used in traffic flow
func WeightedEcmpFixedPacketVerification(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetWeightedEcmpFixedPacketVerification()
}

// OverrideDefaultNhScale returns true if default NextHop scale needs to be modified
// else returns false
func OverrideDefaultNhScale(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetOverrideDefaultNhScale()
}

// BgpExtendedCommunitySetUnsupported returns true if set bgp extended community is unsupported
func BgpExtendedCommunitySetUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetBgpExtendedCommunitySetUnsupported()
}

// BgpSetExtCommunitySetRefsUnsupported returns true if bgp set ext community refs is unsupported
func BgpSetExtCommunitySetRefsUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetBgpSetExtCommunitySetRefsUnsupported()
}

// BgpDeleteLinkBandwidthUnsupported returns true if bgp delete link bandwidth is unsupported
func BgpDeleteLinkBandwidthUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetBgpDeleteLinkBandwidthUnsupported()
}

// QOSInQueueDropCounterUnsupported returns true if /qos/interfaces/interface/input/queues/queue/state/dropped-pkts
// is not supported for any component type.
func QOSInQueueDropCounterUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetQosInqueueDropCounterUnsupported()
}

// BgpExplicitExtendedCommunityEnable returns true if explicit extended community enable is needed
func BgpExplicitExtendedCommunityEnable(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetBgpExplicitExtendedCommunityEnable()
}

// MatchTagSetConditionUnsupported returns true if match tag set condition is not supported
func MatchTagSetConditionUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetMatchTagSetConditionUnsupported()
}

// PeerGroupDefEbgpVrfUnsupported returns true if peer group definition under ebgp vrf is unsupported
func PeerGroupDefEbgpVrfUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetPeerGroupDefEbgpVrfUnsupported()
}

// RedisConnectedUnderEbgpVrfUnsupported returns true if redistribution of routes under ebgp vrf is unsupported
func RedisConnectedUnderEbgpVrfUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetRedisConnectedUnderEbgpVrfUnsupported()
}

// BgpAfiSafiInDefaultNiBeforeOtherNi returns true if certain AFI SAFIs are configured in default network instance before other network instances
func BgpAfiSafiInDefaultNiBeforeOtherNi(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetBgpAfiSafiInDefaultNiBeforeOtherNi()
}

// DefaultImportExportPolicyUnsupported returns true when device
// does not support default import export policy.
func DefaultImportExportPolicyUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetDefaultImportExportPolicyUnsupported()
}

// CommunityInvertAnyUnsupported returns true when device
// does not support community invert any.
func CommunityInvertAnyUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetCommunityInvertAnyUnsupported()
}

// Ipv6RouterAdvertisementIntervalUnsupported returns true for devices which don't support Ipv6 RouterAdvertisement interval configuration
func Ipv6RouterAdvertisementIntervalUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetIpv6RouterAdvertisementIntervalUnsupported()
}

// DecapNHWithNextHopNIUnsupported returns true if Decap NH with NextHopNetworkInstance is unsupported
func DecapNHWithNextHopNIUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetDecapNhWithNexthopNiUnsupported()
}

// SflowSourceAddressUpdateUnsupported returns true if sflow source address update is unsupported
func SflowSourceAddressUpdateUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetSflowSourceAddressUpdateUnsupported()
}

// LinkLocalMaskLen returns true if linklocal mask length is not 64
func LinkLocalMaskLen(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetLinkLocalMaskLen()
}

// UseParentComponentForTemperatureTelemetry returns true if parent component supports temperature telemetry
func UseParentComponentForTemperatureTelemetry(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetUseParentComponentForTemperatureTelemetry()
}

// ComponentMfgDateUnsupported returns true if component's mfg-date leaf is unsupported
func ComponentMfgDateUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetComponentMfgDateUnsupported()
}

// InterfaceCountersUpdateDelayed returns true if telemetry for interface counters
// does not return the latest counter values.
func InterfaceCountersUpdateDelayed(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetInterfaceCountersUpdateDelayed()
}

// OTNChannelTribUnsupported returns true if TRIB parameter is unsupported under OTN channel configuration
func OTNChannelTribUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetOtnChannelTribUnsupported()
}

// EthChannelIngressParametersUnsupported returns true if ingress parameters are unsupported under ETH channel configuration
func EthChannelIngressParametersUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetEthChannelIngressParametersUnsupported()
}

// EthChannelAssignmentCiscoNumbering returns true if eth channel assignment index starts from 1 instead of 0
func EthChannelAssignmentCiscoNumbering(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetEthChannelAssignmentCiscoNumbering()
}

// ChassisGetRPCUnsupported returns true if a Healthz Get RPC against the Chassis component is unsupported
func ChassisGetRPCUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetChassisGetRpcUnsupported()
}

// PowerDisableEnableLeafRefValidation returns true if definition of leaf-ref is not supported.
func PowerDisableEnableLeafRefValidation(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetPowerDisableEnableLeafRefValidation()
}

// SSHServerCountersUnsupported is to skip checking ssh server counters.
func SSHServerCountersUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetSshServerCountersUnsupported()
}

// OperationalModeUnsupported returns true if operational-mode leaf is unsupported
func OperationalModeUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetOperationalModeUnsupported()
}

// BgpSessionStateIdleInPassiveMode returns true if BGP session state idle is not supported instead of active in passive mode.
func BgpSessionStateIdleInPassiveMode(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetBgpSessionStateIdleInPassiveMode()
}

// EnableMultipathUnderAfiSafi returns true for devices that do not support multipath under /global path and instead support under global/afi/safi path.
func EnableMultipathUnderAfiSafi(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetEnableMultipathUnderAfiSafi()
}

// OTNChannelAssignmentCiscoNumbering returns true if OTN channel assignment index starts from 1 instead of 0
func OTNChannelAssignmentCiscoNumbering(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetOtnChannelAssignmentCiscoNumbering()
}

// CiscoPreFECBERInactiveValue returns true if a non-zero pre-fec-ber value is to be used for Cisco
func CiscoPreFECBERInactiveValue(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetCiscoPreFecBerInactiveValue()
}

// BgpAfiSafiWildcardNotSupported return true if bgp afi/safi wildcard query is not supported.
// For example, this yang path query includes the wildcard key `afi-safi-name=`:
// `/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=BGP][name=BGP]/bgp/neighbors/neighbor[neighbor-address=192.0.2.2]/afi-safis/afi-safi[afi-safi-name=]`.
// Use of this deviation is permitted if a query using an explicit key is supported (such as
// `oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST`).
func BgpAfiSafiWildcardNotSupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetBgpAfiSafiWildcardNotSupported()
}

// NoZeroSuppression returns true if device wants to remove zero suppression
func NoZeroSuppression(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetNoZeroSuppression()
}

// IsisInterfaceLevelPassiveUnsupported returns true for devices that do not support passive leaf
func IsisInterfaceLevelPassiveUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetIsisInterfaceLevelPassiveUnsupported()
}

// IsisDisSysidUnsupported returns true for devices that do not support dis-system-id leaf
func IsisDisSysidUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetIsisDisSysidUnsupported()
}

// IsisDatabaseOverloadsUnsupported returns true for devices that do not support database-overloads leaf
func IsisDatabaseOverloadsUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetIsisDatabaseOverloadsUnsupported()
}

// BgpSetMedV7Unsupported returns true if devices which are not
// supporting bgp set med union type in OC.
func BgpSetMedV7Unsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetBgpSetMedV7Unsupported()
}

// EnableTableConnections returns true if admin state of tableconnections needs to be enabled in SRL native model
func EnableTableConnections(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetEnableTableConnections()
}

// TcDefaultImportPolicyUnsupported returns true if default import policy for table connection is unsupported
func TcDefaultImportPolicyUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetTcDefaultImportPolicyUnsupported()
}

// TcMetricPropagationUnsupported returns true if metric propagation for table connection is unsupported
func TcMetricPropagationUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetTcMetricPropagationUnsupported()
}

// TcAttributePropagationUnsupported returns true if attribute propagation for table connection is unsupported
func TcAttributePropagationUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetTcAttributePropagationUnsupported()
}

// TcSubscriptionUnsupported returns true if subscription for table connection is unsupported
func TcSubscriptionUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetTcSubscriptionUnsupported()
}

// DefaultBgpInstanceName returns bgp instance name as set in deviation to override default value "DEFAULT"
func DefaultBgpInstanceName(dut *ondatra.DUTDevice) string {
	if dbin := lookupDUTDeviations(dut).GetDefaultBgpInstanceName(); dbin != "" {
		return dbin
	}
	return "DEFAULT"
}

// ChannelRateClassParametersUnsupported returns true if channel rate class parameters are unsupported
func ChannelRateClassParametersUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetChannelAssignmentRateClassParametersUnsupported()
}

// QosSchedulerIngressPolicer returns true if qos ingress policing is unsupported
func QosSchedulerIngressPolicer(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetQosSchedulerIngressPolicerUnsupported()
}

// GribiEncapHeaderUnsupported returns true if gribi encap header is unsupported
func GribiEncapHeaderUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetGribiEncapHeaderUnsupported()
}

// P4RTCapabilitiesUnsupported returns true for devices that don't support P4RT Capabilities rpc.
func P4RTCapabilitiesUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetP4RtCapabilitiesUnsupported()
}

// GNMIGetOnRootUnsupported returns true if the device does not support gNMI get on root.
func GNMIGetOnRootUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetGnmiGetOnRootUnsupported()
}

// PacketProcessingAggregateDropsUnsupported returns true if the device does not support packet processing aggregate drops.
func PacketProcessingAggregateDropsUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetPacketProcessingAggregateDropsUnsupported()
}

// FragmentTotalDropsUnsupported returns true if the device does not support fragment total drops.
func FragmentTotalDropsUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetFragmentTotalDropsUnsupported()
}

// BgpPrefixsetReqRoutepolRef returns true if devices needs route policy reference to stream prefix set info.
func BgpPrefixsetReqRoutepolRef(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetBgpPrefixsetReqRoutepolRef()
}

// OperStatusForIcUnsupported return true if oper-status leaf is unsupported for Integration Circuit
func OperStatusForIcUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetOperStatusForIcUnsupported()
}

// BgpAspathsetUnsupported returns true if as-path-set for bgp-defined-sets is unsupported
func BgpAspathsetUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetBgpAspathsetUnsupported()
}

// ExplicitDcoConfig returns true if a user-configured value is required in module-functional-type for the transceiver
func ExplicitDcoConfig(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetExplicitDcoConfig()
}

// VerifyExpectedBreakoutSupportedConfig is to skip checking for breakout config mode.
func VerifyExpectedBreakoutSupportedConfig(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetVerifyExpectedBreakoutSupportedConfig()
}

// SrIgpConfigUnsupported return true if SR IGP config is not supported
func SrIgpConfigUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetSrIgpConfigUnsupported()
}

// SetISISAuthWithInterfaceAuthenticationContainer returns true if Isis Authentication is blocked for one level specific config for P2P links, and the corresponding hello-authentication leafs can be set with ISIS Interface/Authentication container.
func SetISISAuthWithInterfaceAuthenticationContainer(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetSetIsisAuthWithInterfaceAuthenticationContainer()
}

// GreGueTunnelInterfaceOcUnsupported returns true if GRE/GUE tunnel interface oc is unsupported
func GreGueTunnelInterfaceOcUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetGreGueTunnelInterfaceOcUnsupported()
}

// LoadIntervalNotSupported returns true if load interval is not supported on vendors
func LoadIntervalNotSupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetLoadIntervalNotSupported()
}

// SkipOpticalChannelOutputPowerInterval returns true if devices do not support opticalchannel output-power interval leaf
func SkipOpticalChannelOutputPowerInterval(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetSkipOpticalChannelOutputPowerInterval()
}

// SkipTransceiverDescription returns true if devices do not support transceiver description leaf
func SkipTransceiverDescription(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetSkipTransceiverDescription()
}

// ContainerzOCUnsupported returns true if devices cannot configure containerz via OpenConfig
func ContainerzOCUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetContainerzOcUnsupported()
}

// NextHopGroupOCUnsupported returns true if devices do not support next-hop-group config
func NextHopGroupOCUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetNextHopGroupConfigUnsupported()
}

// QosShaperOCUnsupported returns true if qos shaper config is unsupported
func QosShaperOCUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetQosShaperConfigUnsupported()
}

// EthernetOverMPLSogreOCUnsupported returns true if ethernet over mplsogre is unsupported
func EthernetOverMPLSogreOCUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetEthernetOverMplsogreUnsupported()
}

// SflowOCUnsupported returns true if sflow is unsupported
func SflowOCUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetSflowUnsupported()
}

// MplsOCUnsupported returns true if mpls is unsupported
func MplsOCUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetMplsUnsupported()
}

// MacsecOCUnsupported returns true if macsec is unsupported
func MacsecOCUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetMacsecUnsupported()
}

// GueGreDecapOCUnsupported returns true if gue gre decap is unsupported
func GueGreDecapOCUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetGueGreDecapUnsupported()
}

// MplsLabelClassificationOCUnsupported returns true if mpls label classification is unsupported
func MplsLabelClassificationOCUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetMplsLabelClassificationUnsupported()
}

// LocalProxyOCUnsupported returns true if local proxy is unsupported
func LocalProxyOCUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetLocalProxyUnsupported()
}

// StaticMplsOCUnsupported returns true if static mpls is unsupported
func StaticMplsOCUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetStaticMplsUnsupported()
}

// QosClassificationOCUnsupported returns true if qos classification is unsupported
func QosClassificationOCUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetQosClassificationUnsupported()
}

// PolicyForwardingOCUnsupported returns true if policy forwarding is unsupported
func PolicyForwardingOCUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetPolicyForwardingUnsupported()
}

// InterfacePolicyForwardingOCUnsupported returns true if interface policy forwarding is unsupported
func InterfacePolicyForwardingOCUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetInterfacePolicyForwardingUnsupported()
}

// GueGreDecapUnsupported returns true if gue or gre decap is unsupported
func GueGreDecapUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetGueGreDecapUnsupported()
}

// StaticMplsUnsupported returns true if static mpls is unsupported
func StaticMplsUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetStaticMplsUnsupported()
}

// QosShaperStateOCUnsupported returns true if qos shaper state is unsupported
func QosShaperStateOCUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetQosShaperStateUnsupported()
}

// CfmOCUnsupported returns true if CFM is unsupported
func CfmOCUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetCfmUnsupported()
}

// LabelRangeOCUnsupported returns true if label range is unsupported
func LabelRangeOCUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetLabelRangeUnsupported()
}

// StaticArpOCUnsupported returns true if static arp is unsupported
func StaticArpOCUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetStaticArpUnsupported()
}

// BgpDistanceOcPathUnsupported returns true if BGP Distance OC telemetry path is not supported.
func BgpDistanceOcPathUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetBgpDistanceOcPathUnsupported()
}

// IsisMplsUnsupported returns true if there's no OC support for MPLS under ISIS
func IsisMplsUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetIsisMplsUnsupported()
}

// AutoNegotiateUnsupported returns true if there's no OC support for auto-negotiate
func AutoNegotiateUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetAutoNegotiateUnsupported()
}

// DuplexModeUnsupported returns true if there's no OC support for duplex-mode
func DuplexModeUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetDuplexModeUnsupported()
}

// PortSpeedUnsupported returns true if there's no OC support for port-speed
func PortSpeedUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetPortSpeedUnsupported()
}

// PolicyForwardingToNextHopOcUnsupported returns true if policy forwarding to next hop is not supported on vendors
func PolicyForwardingToNextHopOcUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetPolicyForwardingToNextHopOcUnsupported()
}

// BGPSetMedActionUnsupported returns true if there's no OC support for BGP set med action
func BGPSetMedActionUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetBgpSetMedActionUnsupported()
}

// reducedEcmpSetOnMixedEncapDecapNh returns true if mixed encap and decap next hops are not supported over ecmp.
// Nokia: b/459893133
func ReducedEcmpSetOnMixedEncapDecapNh(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetReducedEcmpSetOnMixedEncapDecapNh()
}

// NumPhysyicalChannelsUnsupported returns true if there's no OC support for num-physical-channels
func NumPhysyicalChannelsUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetNumPhysicalChannelsUnsupported()
}

// UseOldOCPathStaticLspNh returns true if the old OC path for static lsp next-hop is used
func UseOldOCPathStaticLspNh(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetUseOldOcPathStaticLspNh()
}

// ConfigLeafCreateRequired returns true if leaf creation is required
func ConfigLeafCreateRequired(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetConfigLeafCreateRequired()
}

// FrBreakoutFix returns true if the fix is needed
// Arista: https://issuetracker.google.com/426375784
func FrBreakoutFix(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetFrBreakoutFix()
}

// SkipInterfaceNameCheck returns if device requires skipping the interface name check.
func SkipInterfaceNameCheck(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetSkipInterfaceNameCheck()
}

// UnsupportedQoSOutputServicePolicy returns true if devices do not support qos output service-policy
func UnsupportedQoSOutputServicePolicy(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetUnsupportedQosOutputServicePolicy()
}

// InterfaceOutputQueueNonStandardName returns true if devices have non-standard output queue names
func InterfaceOutputQueueNonStandardName(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetInterfaceOutputQueueNonStandardName()
}

// MplsExpIngressClassifierOcUnsupported returns true if devices do not support classifying ingress packets based on the MPLS exp field
func MplsExpIngressClassifierOcUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetMplsExpIngressClassifierOcUnsupported()
}

// DefaultNoIgpMetricPropagation returns true for devices that do not propagate IGP metric through redistribution
func DefaultNoIgpMetricPropagation(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetDefaultNoIgpMetricPropagation()
}

// SkipBgpPeerGroupSendCommunityType return true if device needs to skip setting BGP send-community-type for peer group
func SkipBgpPeerGroupSendCommunityType(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetSkipBgpPeerGroupSendCommunityType()
}

// ExplicitSwapSrcDstMacNeededForLoopbackMode returns true if device needs to explicitly set swap-src-dst-mac for loopback mode
func ExplicitSwapSrcDstMacNeededForLoopbackMode(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetExplicitSwapSrcDstMacNeededForLoopbackMode()
}

// LinkLocalInsteadOfNh returns true if device requires link-local instead of NH.
func LinkLocalInsteadOfNh(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetLinkLocalInsteadOfNh()
}

// LowScaleAft returns if device requires link-local instead of NH.
func LowScaleAft(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetLowScaleAft()
}

// MissingSystemDescriptionConfigPath returns true if device does not support config lldp system-description leaf
func MissingSystemDescriptionConfigPath(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetMissingSystemDescriptionConfigPath()
}

// NonIntervalFecErrorCounter returns true if FEC uncorrectable errors accumulate over time and are not cleared unless the component is reset on target
func NonIntervalFecErrorCounter(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetNonIntervalFecErrorCounter()
}

// NtpSourceAddressUnsupported returns true if NTP source address is not supported
func NtpSourceAddressUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetNtpSourceAddressUnsupported()
}

// StaticMplsLspOCUnsupported returns true if static mpls lsp parameters are unsupported
func StaticMplsLspOCUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetStaticMplsLspOcUnsupported()
}

// GreDecapsulationOCUnsupported returns true if decapsulation is not supported
func GreDecapsulationOCUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetGreDecapsulationOcUnsupported()
}

// IsisSrgbSrlbUnsupported returns true if SRLB and SRGB configuration is not effective with OC config
func IsisSrgbSrlbUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetIsisSrgbSrlbUnsupported()
}

// IsisSrPrefixSegmentConfigUnsupported returns true if Isis Prefix Segment is not supported
func IsisSrPrefixSegmentConfigUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetIsisSrPrefixSegmentConfigUnsupported()
}

// IsisSrNodeSegmentConfigUnsupported returns true if ISIS SR node segment config is unsupported
func IsisSrNodeSegmentConfigUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetIsisSrNodeSegmentConfigUnsupported()
}

// IsisSrNoPhpRequired returns true if the device requires the no-php flag for ISIS SR prefix and node segments
func IsisSrNoPhpRequired(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetIsisSrNoPhpRequired()
}

// SflowIngressMinSamplingRate returns the minimum sampling rate supported for sflow ingress on the device.
func SflowIngressMinSamplingRate(dut *ondatra.DUTDevice) uint32 {
	return lookupDUTDeviations(dut).GetSflowIngressMinSamplingRate()
}

// QosRemarkOCUnsupported returns true if Qos remark parameters are unsupported
func QosRemarkOCUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetQosRemarkOcUnsupported()
}

// PolicyForwardingGreEncapsulationOcUnsupported returns true if policy forwarding GRE encapsulation is not supported on vendors
func PolicyForwardingGreEncapsulationOcUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetPolicyForwardingGreEncapsulationOcUnsupported()
}

// PolicyRuleCountersOCUnsupported returns true if policy forwarding Rule Counters is not supported on vendors
func PolicyRuleCountersOCUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetPolicyRuleCountersOcUnsupported()
}

// OTNToETHAssignment returns true if the device must have the OTN to ETH assignment.
func OTNToETHAssignment(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetOtnToEthAssignment()
}

// NetworkInstanceImportExportPolicyOCUnsupported returns true if network instance import/export policy is not supported.
func NetworkInstanceImportExportPolicyOCUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetNetworkInstanceImportExportPolicyOcUnsupported()
}

// SkipOrigin returns true if the device does not support the 'origin' field in gNMI/gNOI RPC paths.
func SkipOrigin(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetSkipOrigin()
}

// PredefinedMaxEcmpPaths returns true if max ecmp paths are predefined.
func PredefinedMaxEcmpPaths(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetPredefinedMaxEcmpPaths()
}

// DecapsulateGueOCUnsupported returns true if decapsulation group is not supported
func DecapsulateGueOCUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetDecapsulateGueOcUnsupported()
}

// LinePortUnsupported returns whether the DUT does not support line-port configuration on optical channel components.
func LinePortUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetLinePortUnsupported()
}

// UseBgpSetCommunityOptionTypeReplace returns true if BGP community set REPLACE
// option is required
func UseBgpSetCommunityOptionTypeReplace(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetUseBgpSetCommunityOptionTypeReplace()
}

// GlobalMaxEcmpPathsUnsupported returns true if Max ECMP path on global level is unsupported
func GlobalMaxEcmpPathsUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetGlobalMaxEcmpPathsUnsupported()
}

// QosTwoRateThreeColorPolicerOCUnsupported returns true if the device does not support QoS two-rate-three-color policer.
// Arista: https://partnerissuetracker.corp.google.com/issues/442749011
func QosTwoRateThreeColorPolicerOCUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetQosTwoRateThreeColorPolicerOcUnsupported()
}

// LoadBalancePolicyOCUnsupported returns true if load-balancing policy configuration is not supported through OpenConfig.
func LoadBalancePolicyOCUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetLoadBalancePolicyOcUnsupported()
}

// GribiRecordsUnsupported returns true if Gribi records creation is not supported through OpenConfig.
func GribiRecordsUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetGribiRecordsUnsupported()
}

// CiscoxrLaserFt returns the functional translator to be used for translating
// transceiver threshold leaves.
func CiscoxrLaserFt(dut *ondatra.DUTDevice) string {
	return lookupDUTDeviations(dut).GetCiscoxrLaserFt()
}

// BreakoutModeUnsupportedForEightHundredGb returns true if the device does not support breakout mode for 800G ports.
func BreakoutModeUnsupportedForEightHundredGb(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetBreakoutModeUnsupportedForEightHundredGb()
}

// PortSpeedDuplexModeUnsupportedForInterfaceConfig returns true if the device does not support port speed and duplex mode for interface config.
func PortSpeedDuplexModeUnsupportedForInterfaceConfig(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetPortSpeedDuplexModeUnsupportedForInterfaceConfig()
}

// ExplicitBreakoutInterfaceConfig returns true if the device needs explicit breakout interface config.
func ExplicitBreakoutInterfaceConfig(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetExplicitBreakoutInterfaceConfig()
}

// TelemetryNotSupportedForLowPriorityNh returns true if OC state path for the lower priority next hop not supported
func TelemetryNotSupportedForLowPriorityNh(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetTelemetryNotSupportedForLowPriorityNh()
}

// MatchAsPathSetUnsupported returns true if match-as-path-set policy configuration is not supported
func MatchAsPathSetUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetMatchAsPathSetUnsupported()
}

// SameAfiSafiAndPeergroupPoliciesUnsupported returns true if configuring same apply-policy under peer-group and peer-group/afi-safi is unsupported
func SameAfiSafiAndPeergroupPoliciesUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetSameAfiSafiAndPeergroupPoliciesUnsupported()
}

// SyslogOCUnsupported returns true if the device does not support syslog OC configuration for below OC paths.
// '/system/logging/remote-servers/remote-server/config/network-instance'
func SyslogOCUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetSyslogOcUnsupported()
}

// SIDPerInterfaceCounterUnsupported return true if device does not supprt mpls/signaling-protocols/segment-routing/interfaces/interface/sid-counters/sid-counter/
func SIDPerInterfaceCounterUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetSidPerInterfaceCounterUnsupported()
}

// TransceiverConfigEnableUnsupported returns true if devices cannot set transceiver config enable
func TransceiverConfigEnableUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetTransceiverConfigEnableUnsupported()
}

// AFTSummaryOCUnsupported returns true "/network-instances/network-instance/afts/aft-summaries" OC path is not supported.
func AFTSummaryOCUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetAftSummaryOcUnsupported()
}

// ISISLSPTlvsOCUnsupported returns true if "/network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs" OC path is not supported.
func ISISLSPTlvsOCUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetIsisLspTlvsOcUnsupported()
}

// ISISAdjacencyStreamUnsupported returns if "/network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies" OC path
// is not supported or malfunctioning when STREAM subscription is used .
func ISISAdjacencyStreamUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetIsisAdjacencyStreamUnsupported()
}

// LocalhostForContainerz returns true if the device uses an IPv6 address instead of localhost.
func LocalhostForContainerz(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetLocalhostForContainerz()
}

// AggregateBandwidthPolicyActionUnsupported returns true if device does not support aggregate bandwidth policy action.
func AggregateBandwidthPolicyActionUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetAggregateBandwidthPolicyActionUnsupported()
}

// AutoLinkBandwidthUnsupported returns true if device does not support auto link bandwidth.
func AutoLinkBandwidthUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetAutoLinkBandwidthUnsupported()
}

// AdvertisedCumulativeLBwOCUnsupported returns true if device does not support oc state path for advertised cumulative link bandwidth.
func AdvertisedCumulativeLBwOCUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetAdvertisedCumulativeLbwOcUnsupported()
}

// DisableHardwareNexthopProxy returns true if the device requires disabling hardware nexthop proxying
func DisableHardwareNexthopProxy(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetDisableHardwareNexthopProxy()
}

// URPFConfigOCUnsupported returns true if OC does not support configuring uRPF.
func URPFConfigOCUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetInterfacePolicyForwardingUnsupported()
}

// StaticRouteNextNetworkInstanceOCUnsupported returns true for devices that don't support NextNetworkInstance of static route next hop.
func StaticRouteNextNetworkInstanceOCUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetStaticRouteNextNetworkInstanceOcUnsupported()
}

// GnpsiOcUnsupported returns true if there's no OC support for configuring gNPSI
func GnpsiOcUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetGnpsiOcUnsupported()
}

// SyslogNonDefaultVrfUnsupported returns true if device does not support adding remote-syslog config under
// non-default VRF
func SyslogNonDefaultVrfUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetSyslogNonDefaultVrfUnsupported()
}

// BgpLocalAggregateUnsupported returns true for devices that don't support OC configuration of BGP local aggregates
func BgpLocalAggregateUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetBgpLocalAggregateUnsupported()
}

// SkipSamplingQosCounters returns true if device does not support sampling QoS counters
// Cisco: https://partnerissuetracker.corp.google.com/u/0/issues/463279843
func SkipSamplingQosCounters(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetSkipSamplingQosCounters()
}

// DefaultNiGnmiServerName returns the user provided default server name for gRPC server in the default network-instance.
func DefaultNiGnmiServerName(dut *ondatra.DUTDevice) string {
	if gnmiServerName := lookupDUTDeviations(dut).GetDefaultNiGnmiServerName(); gnmiServerName != "" {
		return gnmiServerName
	}
	return "DEFAULT"
}

// ConfigACLWithPrefixListNotSupported returns true if configuring prefixlist in ACL not supported
func ConfigACLWithPrefixListNotSupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetConfigAclWithPrefixlistUnsupported()
}

// ConfigACLValueAnyOcUnsupported returns true if OC for configuring parameter in ACL with value ANY not supported
func ConfigACLValueAnyOcUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetConfigAclValueAnyOcUnsupported()
}

// ConfigAclOcUnsupported returns true if OC for configuring parameter in ACL with OC is not supported
func ConfigAclOcUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetConfigAclOcUnsupported()
}

// InterfaceCountersInUnknownProtosUnsupported returns if the device does not support interface counters in unknown protos.
// https://issuetracker.google.com/issues/461368936
func InterfaceCountersInUnknownProtosUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetInterfaceCountersInUnknownProtosUnsupported()
}

// AggregateSIDCounterOutPktsUnsupported returns true if device does not support
// /network-instances/network-instance/mpls/signaling-protocols/segment-routing/aggregate-sid-counters/aggregate-sid-counter/state/out-pkts
func AggregateSIDCounterOutPktsUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetAggregateSidCounterOutPktsUnsupported()
}

// MatchCommunitySetMatchSetOptionsAllUnsupported returns true if device does not support match-set-options=ALL
// for bgp-conditions community-sets
// Arista: b/335739231
func MatchCommunitySetMatchSetOptionsAllUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetMatchCommunitySetMatchSetOptionsAllUnsupported()
}

// BMPOCUnsupported returns true if BMP configuration is not supported
func BMPOCUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetBmpOcUnsupported()
}

// BgpCommunityTypeSliceInputUnsupported returns true if device does not support slice input of BGP community type
// Cisco: https://partnerissuetracker.corp.google.com/u/0/issues/468284934
func BgpCommunityTypeSliceInputUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetBgpCommunityTypeSliceInputUnsupported()
}

// IbgpMultipathPathUnsupported returns true if device does not support configuring multipath path under ibgp
func IbgpMultipathPathUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetIbgpMultipathPathUnsupported()
}

// GetRetainGnmiCfgAfterReboot returns true if the device requires additional configuration to retain gNMI config across reboots.
// Arista: https://partnerissuetracker.corp.google.com/476271160
func GetRetainGnmiCfgAfterReboot(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetRetainGnmiCfgAfterReboot()
}

// ContainerzPluginRPCUnsupported returns true if ContainerZ plugin RPCs are unsupported.
func ContainerzPluginRPCUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetContainerzPluginRpcUnsupported()
}

// NonStandardGRPCPort returns true if the device does not use standard grpc port.
// Arista b/384040563
func NonStandardGRPCPort(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetNonStandardGrpcPort()
}

// TemperatureSensorCheck returns true if the transceiver subcomponent should look for the temperature sensor
func TemperatureSensorCheck(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetTemperatureSensorCheck()
}

// CpuUtilizationQueryAgainstBaseControllerCardComponent returns true if the device reports Controller CPU utilization against the base controller card component
// example: against "0/RP0/CPU0" and not "0/RP00/CPU0-Broadwell-DE (D-1573N)"
func CpuUtilizationQueryAgainstBaseControllerCardComponent(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetCpuUtilizationQueryAgainstBaseControllerCardComponent()
}

// CpuUtilizationQueryAgainstBaseLinecardComponent returns true if the device reports linecard CPU utilization against the base linecard component
// example: against "0/0/CPU0" and not "0/0/CPU0-Broadwell-DE (D-1573N)"
func CpuUtilizationQueryAgainstBaseLinecardComponent(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetCpuUtilizationQueryAgainstBaseLinecardComponent()
}

// NoQueueDropUnsupported returns true if device does not support no-queue drops
func NoQueueDropUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetNoQueueDropUnsupported()
}

// InterfaceEthernetInblockErrorsUnsupported returns true if device does not support interface ethernet in-block errors
func InterfaceEthernetInblockErrorsUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetInterfaceEthernetInblockErrorsUnsupported()
}

// CiscoxrTransceiverFt returns the functional translator to be used for translating
// transceiver threshold leaves.
func CiscoxrTransceiverFt(dut *ondatra.DUTDevice) string {
	return lookupDUTDeviations(dut).GetCiscoxrTransceiverFt()
}

// TransceiverStateUnsupported returns true if device does not support transceiver state leaf.
func TransceiverStateUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetTransceiverStateUnsupported()
}

// SubnetMaskChangeRequired returns true if the device requires changing the subnet mask length.
// Cisco: https://partnerissuetracker.corp.google.com/issues/478070225
func SubnetMaskChangeRequired(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetSubnetMaskChangeRequired()
}

// Ciscoxr8000IntegratedCircuitResourceFt returns the functional translator to be used for translating
// integrated circuit resource leaves.
func Ciscoxr8000IntegratedCircuitResourceFt(dut *ondatra.DUTDevice) string {
	return lookupDUTDeviations(dut).GetCiscoxr8000IntegratedCircuitResourceFt()
}

// BgpDefaultPolicyBehaviorAcceptRoute returns true if the BGP accepts routes by default when
// there is no routing policy or default policy configured.
func BgpDefaultPolicyBehaviorAcceptRoute(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetBgpDefaultPolicyBehaviorAcceptRoute()
}
