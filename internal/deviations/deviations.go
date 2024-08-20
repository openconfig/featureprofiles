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

	log "github.com/golang/glog"
	"github.com/openconfig/featureprofiles/internal/metadata"
	mpb "github.com/openconfig/featureprofiles/proto/metadata_go_proto"
	"github.com/openconfig/ondatra"
)

func lookupDeviations(dvc *ondatra.Device) (*mpb.Metadata_PlatformExceptions, error) {
	var matchedPlatformException *mpb.Metadata_PlatformExceptions

	for _, platformExceptions := range metadata.Get().PlatformExceptions {
		if platformExceptions.GetPlatform().Vendor.String() == "" {
			return nil, fmt.Errorf("vendor should be specified in textproto %v", platformExceptions)
		}

		if dvc.Vendor().String() != platformExceptions.GetPlatform().Vendor.String() {
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

// AggregateAtomicUpdate returns if device requires that aggregate Port-Channel and its members be defined in a single gNMI Update transaction at /interfaces.
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

// BackplaneFacingCapacityUnsupported returns whether the device supports backplane-facing-capacity leaves for some of the components.
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
// the device requires explict component path to account for a situation when there is more than one active reboot requests.
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

// ExplicitInterfaceRefDefinition returns if device requires explicit interface ref configuration when applying features to interface.
func ExplicitInterfaceRefDefinition(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetExplicitInterfaceRefDefinition()
}

// QOSDroppedOctets returns if device should skip checking QOS Dropped octets stats for interface.
func QOSDroppedOctets(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetQosDroppedOctets()
}

// ExplicitGRIBIUnderNetworkInstance returns if device requires gribi-protocol to be enabled under network-instance.
func ExplicitGRIBIUnderNetworkInstance(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetExplicitGribiUnderNetworkInstance()
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

// NtpNonDefaultVrfUnsupported returns true if the device does not support ntp nondefault vrf.
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
// subinterface with tagged vlan for p4rt packet in.
func P4RTGdpRequiresDot1QSubinterface(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetP4RtGdpRequiresDot1QSubinterface()
}

// ISISLspLifetimeIntervalRequiresLspRefreshInterval returns true for devices that require
// configuring lspRefreshInterval ISIS timer when lspLifetimeInterval is configured.
func ISISLspLifetimeIntervalRequiresLspRefreshInterval(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetIsisLspLifetimeIntervalRequiresLspRefreshInterval()
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

// GRIBISkipFIBFailedTrafficForwardingCheck returns true for devices that do not
// support fib forwarding for fib failed routes.
func GRIBISkipFIBFailedTrafficForwardingCheck(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetSkipFibFailedTrafficForwardingCheck()
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

// BgpLlgrOcUndefined returns true if device should does not support OC path to disable BGP LLGR.
func BgpLlgrOcUndefined(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetBgpLlgrOcUndefined()
}

// QOSBufferAllocationConfigRequired returns if device should configure QOS buffer-allocation-profile
func QOSBufferAllocationConfigRequired(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetQosBufferAllocationConfigRequired()
}

// BGPGlobalExtendedNextHopEncodingUnsupported returns true for devices that do not support configuring
// BGP ExtendedNextHopEncoding at thee global level.
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

// MissingHardwareResourceTelemetryBeforeConfig returns true for devices that don't
// support telemetry for hardware resources before used-threshold-upper configuration.
func MissingHardwareResourceTelemetryBeforeConfig(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetMissingHardwareResourceTelemetryBeforeConfig()
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

// SkipStaticNexthopCheck returns if device needs index starting from non zero
func SkipStaticNexthopCheck(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetSkipStaticNexthopCheck()
}

// EnableFlowctrlFlag returns if device needs set leaf specific enable flag.
func EnableFlowctrlFlag(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetEnableFlowctrlFlag()
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

// SkipPbfWithDecapEncapVrf return true if device needs to skip test with which has PBF with decap encap VRF as action
func SkipPbfWithDecapEncapVrf(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetSkipPbfWithDecapEncapVrf()
}

// TTLCopyUnsupported returns true for devices which does not support TTL copy.
func TTLCopyUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetTtlCopyUnsupported()
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

// SkipSetRpMatchSetOptions return true if device needs to skip setting match-prefix-set match-set-options while configuring routing-policy statement condition
func SkipSetRpMatchSetOptions(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetSkipSetRpMatchSetOptions()
}

// SkipSettingDisableMetricPropagation return true if device needs to skip setting disable-metric-propagation while configuring table-connection
func SkipSettingDisableMetricPropagation(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetSkipSettingDisableMetricPropagation()
}

// BGPConditionsMatchCommunitySetUnsupported returns true if device doesn't support bgp-conditions/match-community-set leaf
func BGPConditionsMatchCommunitySetUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetBgpConditionsMatchCommunitySetUnsupported()
}

// PfRequireMatchDefaultRule returns true for device which requires match condition for ethertype v4 and v6 for default rule with network-instance default-vrf in policy-forwarding.
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

// PLQGeneratorCapabilitiesMaxMTU returns supported max_mtu for devices that does not support packet link qualification(PLQ) Generator max_mtu to be atleast >= 8184.
func PLQGeneratorCapabilitiesMaxMTU(dut *ondatra.DUTDevice) uint32 {
	return lookupDUTDeviations(dut).GetPlqGeneratorCapabilitiesMaxMtu()
}

// PLQGeneratorCapabilitiesMaxPPS returns supported max_pps for devices that does not support packet link qualification(PLQ) Generator max_pps to be atleast >= 100000000.
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

// DefaultImportExportPolicy returns true when device does not have a default deny action in the absence of a route policy
func DefaultImportExportPolicy(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetDefaultImportExportPolicy()
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

// IPv6StaticRouteWithIPv4NextHopUnsupported unsported ipv6 with ipv4 nexthop
func IPv6StaticRouteWithIPv4NextHopUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetIpv6StaticRouteWithIpv4NhUnsupported()
}

// StaticRouteWithDropNhUnsupported unsuported drop nexthop
func StaticRouteWithDropNhUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetStaticRouteWithDropNh()
}

// StaticRouteWithExplicitMetric set explict metric
func StaticRouteWithExplicitMetric(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetStaticRouteWithExplicitMetric()
}

// BgpDefaultPolicyUnsupported return true if BGP default-import/export-policy is not supported.
func BgpDefaultPolicyUnsupported(dut *ondatra.DUTDevice) bool {
	return lookupDUTDeviations(dut).GetBgpDefaultPolicyUnsupported()
}

// ExplicitEnableBGPOnDefaultVRF return true if BGP needs to be explicity enabled on default VRF
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

// EncapTunnelShutBackupNhgZeroTraffic returns true when encap tunnel is shut then zero traffic flows to backup NHG
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
