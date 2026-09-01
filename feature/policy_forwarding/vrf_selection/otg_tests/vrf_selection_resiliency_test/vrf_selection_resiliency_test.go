// Copyright 2024 Google LLC
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

package vrf_selection_resiliency_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/featureprofiles/internal/vrfpolicy"
	syspb "github.com/openconfig/gnoi/system"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	policyName = "HA_VRF_SELECTION"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func configEgressSubIntf(dut *ondatra.DUTDevice, d *oc.Root, port *ondatra.Port, subIntfIndex uint32, vrfName string) {
	intf := d.GetOrCreateInterface(port.Name())
	intf.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	if deviations.InterfaceEnabled(dut) {
		intf.Enabled = ygot.Bool(true)
	}
	s := intf.GetOrCreateSubinterface(subIntfIndex)
	if !deviations.DeprecatedVlanID(dut) {
		s.GetOrCreateVlan().GetOrCreateMatch().GetOrCreateSingleTagged().VlanId = ygot.Uint16(uint16(subIntfIndex))
	} else {
		s.GetOrCreateVlan().VlanId = oc.UnionUint16(uint16(subIntfIndex))
	}
	s.GetOrCreateIpv4().Enabled = ygot.Bool(true)
	s.GetOrCreateIpv6().Enabled = ygot.Bool(true)

	s.GetOrCreateIpv4().GetOrCreateAddress(fmt.Sprintf("100.64.%d.1", subIntfIndex)).PrefixLength = ygot.Uint8(24)
	s.GetOrCreateIpv6().GetOrCreateAddress(fmt.Sprintf("2001:db8:%d::1", subIntfIndex)).PrefixLength = ygot.Uint8(64)

	ni := d.GetOrCreateNetworkInstance(vrfName)
	ni.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF
	if vrfName == deviations.DefaultNetworkInstance(dut) {
		ni.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE
	}
	niIntf := ni.GetOrCreateInterface(fmt.Sprintf("%s.%d", port.Name(), subIntfIndex))
	niIntf.Interface = ygot.String(port.Name())
	niIntf.Subinterface = ygot.Uint32(subIntfIndex)
}

func configStaticRoute(d *oc.Root, dut *ondatra.DUTDevice, vrfName string, v4Prefix string, v4NextHop string, v6Prefix string, v6NextHop string) {
	ni := d.GetOrCreateNetworkInstance(vrfName)
	static := ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))

	if v4Prefix != "" {
		sr := static.GetOrCreateStatic(v4Prefix)
		nh := sr.GetOrCreateNextHop("0")
		nh.NextHop = oc.UnionString(v4NextHop)
	}

	if v6Prefix != "" {
		sr := static.GetOrCreateStatic(v6Prefix)
		nh := sr.GetOrCreateNextHop("0")
		nh.NextHop = oc.UnionString(v6NextHop)
	}
}

func TestVRFSelectionResiliency(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")
	p3 := dut.Port(t, "port3")
	p4 := dut.Port(t, "port4")

	t.Logf("Configuring DUT interfaces...")
	d := &oc.Root{}

	// Port-1 Ingress
	i1 := d.GetOrCreateInterface(p1.Name())
	i1.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	if deviations.InterfaceEnabled(dut) {
		i1.Enabled = ygot.Bool(true)
	}
	s1 := i1.GetOrCreateSubinterface(0)
	s1.GetOrCreateIpv4().Enabled = ygot.Bool(true)
	s1.GetOrCreateIpv4().GetOrCreateAddress("100.64.0.1").PrefixLength = ygot.Uint8(24)

	// Step 1: Initial VRF Binding for ingress
	niPfw := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut)).GetOrCreatePolicyForwarding()
	i1fwd := niPfw.GetOrCreateInterface(p1.Name())
	i1fwd.ApplyVrfSelectionPolicy = ygot.String(policyName)
	if !deviations.InterfaceRefConfigUnsupported(dut) {
		i1fwd.GetOrCreateInterfaceRef().Interface = ygot.String(p1.Name())
		i1fwd.GetOrCreateInterfaceRef().Subinterface = ygot.Uint32(0)
	}

	idx := uint32(1)
	// Port-2 (Egress 1)
	for i := 1; i <= 10; i++ {
		configEgressSubIntf(dut, d, p2, idx, fmt.Sprintf("VRF-V4-%d", i))
		configStaticRoute(d, dut, fmt.Sprintf("VRF-V4-%d", i), fmt.Sprintf("203.0.113.%d/32", i), fmt.Sprintf("192.168.%d.2", idx), "", "")
		idx++
	}
	configEgressSubIntf(dut, d, p2, idx, deviations.DefaultNetworkInstance(dut)) // DEFAULT
	configStaticRoute(d, dut, deviations.DefaultNetworkInstance(dut), "0.0.0.0/0", fmt.Sprintf("192.168.%d.2", idx), "::/0", fmt.Sprintf("2001:db8:%d::2", idx))
	idx++

	// Port-3 (Egress 2)
	for i := 11; i <= 15; i++ {
		configEgressSubIntf(dut, d, p3, idx, fmt.Sprintf("VRF-V4-%d", i))
		configStaticRoute(d, dut, fmt.Sprintf("VRF-V4-%d", i), fmt.Sprintf("203.0.113.%d/32", i), fmt.Sprintf("192.168.%d.2", idx), "", "")
		idx++
	}
	for i := 1; i <= 5; i++ {
		configEgressSubIntf(dut, d, p3, idx, fmt.Sprintf("VRF-V6-%d", i))
		configStaticRoute(d, dut, fmt.Sprintf("VRF-V6-%d", i), fmt.Sprintf("203.0.113.%d/32", i), fmt.Sprintf("192.168.%d.2", idx), fmt.Sprintf("2001:db8:a:%d::/128", i), fmt.Sprintf("2001:db8:%d::2", idx))
		idx++
	}

	// Port-4 (Egress 3)
	for i := 6; i <= 15; i++ {
		configEgressSubIntf(dut, d, p4, idx, fmt.Sprintf("VRF-V6-%d", i))
		configStaticRoute(d, dut, fmt.Sprintf("VRF-V6-%d", i), fmt.Sprintf("203.0.113.%d/32", i), fmt.Sprintf("192.168.%d.2", idx), fmt.Sprintf("2001:db8:a:%d::/128", i), fmt.Sprintf("2001:db8:%d::2", idx))
		idx++
	}

	// Create VRF-GHOST network instance
	niGhost := d.GetOrCreateNetworkInstance("VRF-GHOST")
	niGhost.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF

	// RT-3.4.1. Step 1: Add massive policy
	massivePolicy := vrfpolicy.BuildScaledVRFSelectionPolicy(t, dut, deviations.DefaultNetworkInstance(dut))
	existingPfw := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut)).GetOrCreatePolicyForwarding()
	massivePolicy.Interface = existingPfw.Interface
	d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding = massivePolicy
	d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut)).GetOrCreatePolicyForwarding().Policy = massivePolicy.Policy

	// RT-3.4.1 Step 2: Push config to DUT
	gnmi.Replace(t, dut, gnmi.OC().Interface(p1.Name()).Config(), i1)
	gnmi.Replace(t, dut, gnmi.OC().Interface(p2.Name()).Config(), d.GetInterface(p2.Name()))
	gnmi.Replace(t, dut, gnmi.OC().Interface(p3.Name()).Config(), d.GetInterface(p3.Name()))
	gnmi.Replace(t, dut, gnmi.OC().Interface(p4.Name()).Config(), d.GetInterface(p4.Name()))

	for _, ni := range d.NetworkInstance {
		gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(ni.GetName()).Config(), ni)
	}

	t.Cleanup(func() {
		t.Logf("Cleaning up configurations...")
		vrfpolicy.DeletePolicyForwarding(t, dut, p1.Name())
		for _, ni := range d.NetworkInstance {
			if ni.GetName() != deviations.DefaultNetworkInstance(dut) {
				gnmi.Delete(t, dut, gnmi.OC().NetworkInstance(ni.GetName()).Config())
			}
		}
	})

	// Delete VRF-GHOST as specified by the test procedure to ensure Traffic Drops correctly
	gnmi.Delete(t, dut, gnmi.OC().NetworkInstance("VRF-GHOST").Config())

	t.Logf("Wait for interface state to propagate...")
	gnmi.Await(t, dut, gnmi.OC().Interface(p1.Name()).OperStatus().State(), time.Minute, oc.Interface_OperStatus_UP)
	gnmi.Await(t, dut, gnmi.OC().Interface(p2.Name()).OperStatus().State(), time.Minute, oc.Interface_OperStatus_UP)
	gnmi.Await(t, dut, gnmi.OC().Interface(p3.Name()).OperStatus().State(), time.Minute, oc.Interface_OperStatus_UP)
	gnmi.Await(t, dut, gnmi.OC().Interface(p4.Name()).OperStatus().State(), time.Minute, oc.Interface_OperStatus_UP)

	// RT-3.4.1 Step 3: Generating continuous OTG traffic from ATE Port-1
	t.Logf("Generating continuous OTG traffic...")
	top := gosnappi.NewConfig()

	// Configure precise OTG Topology (Ports 1-4, Devices, Subinterfaces based on DUT)
	vrfpolicy.ConfigureOTGTopology(t, top, ate)

	// Configure all 62 flows (30 Positive, 30 Negative, 1 Ghost, 1 Shadow)
	vrfpolicy.ConfigureScaledOTGFlows(top)

	t.Logf("Starting traffic on ATE...")
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)

	// Wait for ARP resolution
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv6")
	ate.OTG().StartTraffic(t)

	// RT-3.4.1 Step 4: Validating perfect isolation, default fallbacks, ghost drops, and shadow routes
	vrfpolicy.ValidateBaselineTraffic(t, ate, top)
	t.Logf("Validated RT-3.4.1 Baseline.")

	// RT-3.4.2 - VRF Selection Policy Resilience Post Supervisor Switchover
	t.Logf("RT-3.4.2 - Step 1: Trigger Switchover")

	controllers := components.FindComponentsByType(t, dut, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD)
	if len(controllers) > 0 {
		standbyController, activeController := components.FindStandbyControllerCard(t, dut, controllers)
		t.Logf("Found active: %s, standby: %s. Initiating SwitchControlProcessor to standby...", activeController, standbyController)

		systemClient := dut.RawAPIs().GNOI(t).System()
		req := &syspb.SwitchControlProcessorRequest{
			ControlProcessor: components.GetSubcomponentPath(standbyController, deviations.GNOISubcomponentPath(dut)),
		}
		if _, err := systemClient.SwitchControlProcessor(context.Background(), req); err != nil {
			t.Fatalf("SwitchControlProcessor failed: %v", err)
		}

		// Wait for control-plane to accept GNMI again by polling a safe telemetry path.
		start := time.Now()
		timeout := 5 * time.Minute
		for {
			time.Sleep(10 * time.Second)
			var now string
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				now = gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State())
			}); errMsg != nil {
				t.Logf("gNMI not ready yet (%s); retrying...", *errMsg)
			} else {
				t.Logf("gNMI reachable; DUT time: %s", now)
				break
			}
			if time.Since(start) > timeout {
				t.Fatalf("timed out waiting for gNMI after %v", timeout)
			}
		}

		// Confirm standby became the new active.
		_, rpActiveAfterSwitch := components.FindStandbyControllerCard(t, dut, controllers)
		if rpActiveAfterSwitch != standbyController {
			t.Fatalf("Post-switchover active RP mismatch: got %q want %q", rpActiveAfterSwitch, standbyController)
		}
		t.Logf("Switchover complete: new active %s", rpActiveAfterSwitch)
	} else {
		t.Logf("No controller cards found; skipping switchover.")
	}

	t.Logf("RT-3.4.2 - Step 2: Validate 0%% traffic loss")
	vrfpolicy.ValidateBaselineTraffic(t, ate, top)

	// RT-3.4.3 - VRF Selection Policy Resilience Post Linecard OIR
	t.Logf("RT-3.4.3 - Step 1: Perform Linecard Soft OIR")
	linecardPort := cfgplugins.FindLineCardParent(t, dut, p1.Name())
	if linecardPort == "" {
		t.Logf("Could not find linecard for port: %s. Skipping Linecard OIR tests.", p1.Name())
	} else {
		linecardPath := gnmi.OC().Component(linecardPort).Linecard().PowerAdminState()
		t.Cleanup(func() {
			gnmi.Replace(t, dut, linecardPath.Config(), oc.Platform_ComponentPowerType_POWER_ENABLED)
		})
		t.Logf("Toggling linecard %s to POWER_DISABLED", linecardPort)
		gnmi.Replace(t, dut, linecardPath.Config(), oc.Platform_ComponentPowerType_POWER_DISABLED)
		gnmi.Await(t, dut, gnmi.OC().Component(linecardPort).OperStatus().State(), 5*time.Minute, oc.PlatformTypes_COMPONENT_OPER_STATUS_DISABLED)

		// Wait a moment before reviving the card
		time.Sleep(30 * time.Second)

		t.Logf("Toggling linecard %s to POWER_ENABLED", linecardPort)
		gnmi.Replace(t, dut, linecardPath.Config(), oc.Platform_ComponentPowerType_POWER_ENABLED)
		gnmi.Await(t, dut, gnmi.OC().Component(linecardPort).OperStatus().State(), 10*time.Minute, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE)

		t.Logf("Waiting for reliant ports to revert to UP state...")
		gnmi.Await(t, dut, gnmi.OC().Interface(p1.Name()).OperStatus().State(), 2*time.Minute, oc.Interface_OperStatus_UP)

		t.Logf("RT-3.4.3 - Step 2: Validating autonomous recovery")
		vrfpolicy.ValidateBaselineTraffic(t, ate, top)

	// RT-3.4.4 - Policy Deletion
	t.Logf("RT-3.4.4 - Step 1 & 2: Delete VRF Selection Policy & Push Configuration")
	vrfpolicy.DeletePolicyForwarding(t, dut, p1.Name())

	// Wait for policy detachment propagation
	interfaceID := p1.Name()
	if deviations.InterfaceRefInterfaceIDFormat(dut) || deviations.InterfaceIDFormatRequiredForPolicyForwarding(dut) {
		interfaceID = p1.Name() + ".0"
	}
	gnmi.Watch(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding().Interface(interfaceID).ApplyVrfSelectionPolicy().State(), time.Minute, func(val *ygnmi.Value[string]) bool {
		v, present := val.Val()
		return !present || v == ""
	}).Await(t)

	t.Logf("RT-3.4.4 - Step 3: Validate fallback to DEFAULT network instance mapping")
	// Validated by ensuring traffic no longer routes to specific egress ports uniquely mapped to VRF subints
	// Since verification needs to clear counts, we reset OTG state.
	ate.OTG().StopTraffic(t)
	time.Sleep(5 * time.Second)
	ate.OTG().StartTraffic(t)

	vrfpolicy.ValidateFallbackTraffic(t, ate, top)
}
