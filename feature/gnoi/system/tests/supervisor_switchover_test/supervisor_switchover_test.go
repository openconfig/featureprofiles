// Copyright 2022 Google LLC
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

package supervisor_switchover_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/args"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	spb "github.com/openconfig/gnoi/system"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	maxSwitchoverTime = 15 * time.Minute
	controlcardType   = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD
	ateLagName        = "Port-Channel1"
	flowName          = "Flow-LACP-Continuous"
	ipv4PrefixLen     = 30
	flowPPS           = 500
	flowPacketSize    = 512
)

var (
	dutSrc = attrs.Attributes{
		Desc:    "dutSrc",
		IPv4:    "192.0.2.1",
		IPv4Len: ipv4PrefixLen,
	}
	ateSrc = attrs.Attributes{
		Name:    "ateSrc",
		IPv4:    "192.0.2.2",
		MAC:     "02:00:01:01:01:01",
		IPv4Len: ipv4PrefixLen,
	}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// configureDUT configures two physical ports and an LACP bundle on the DUT.
// Corresponds to README section "Test environment setup":
// * Configure an LACP port-channel across 2 DUT ports connected to the IXIA/ATE.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) ([]*ondatra.Port, string) {
	t.Helper()
	lagName := netutil.NextAggregateInterface(t, dut)
	t.Logf("Dynamically allocated aggregate interface name for DUT %v: %q", dut.Model(), lagName)
	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")
	ports := []*ondatra.Port{p1, p2}

	t.Cleanup(func() {
		batch := &gnmi.SetBatch{}
		for _, port := range ports {
			gnmi.BatchDelete(batch, gnmi.OC().Interface(port.Name()).Config())
		}
		gnmi.BatchDelete(batch, gnmi.OC().Interface(lagName).Config())
		if !deviations.LacpInterfaceFallbackOCUnsupported(dut) {
			gnmi.BatchDelete(batch, gnmi.OC().Lacp().Interface(lagName).Config())
		}
		batch.Set(t, dut)
	})

	batch := &gnmi.SetBatch{}

	// 1. Configure the aggregate LACP interface
	t.Logf("Configuring aggregate LACP interface %q with LagType=LACP...", lagName)
	lagIntf := &oc.Interface{Name: ygot.String(lagName)}
	lagIntf.Type = oc.IETFInterfaces_InterfaceType_ieee8023adLag
	lagIntf.GetOrCreateAggregation().LagType = oc.IfAggregate_AggregationType_LACP
	lagIntf.Enabled = ygot.Bool(true)
	lagIntf.Description = ygot.String("LACP Port-Channel bundle for Supervisor Switchover test")
	sub := lagIntf.GetOrCreateSubinterface(0)
	s4 := sub.GetOrCreateIpv4()
	if deviations.InterfaceEnabled(dut) {
		s4.Enabled = ygot.Bool(true)
	}
	s4.GetOrCreateAddress(dutSrc.IPv4).PrefixLength = ygot.Uint8(dutSrc.IPv4Len)

	gnmi.BatchReplace(batch, gnmi.OC().Interface(lagName).Config(), lagIntf)

	if !deviations.LacpInterfaceFallbackOCUnsupported(dut) {
		t.Logf("Configuring LACP parameters on aggregate interface %q...", lagName)
		lacp := &oc.Lacp_Interface{Name: ygot.String(lagName)}
		lacp.LacpMode = oc.Lacp_LacpActivityType_ACTIVE
		lacp.Interval = oc.Lacp_LacpPeriodType_FAST
		gnmi.BatchReplace(batch, gnmi.OC().Lacp().Interface(lagName).Config(), lacp)
	}

	// 2. Configure member Ethernet interfaces and bind them to the aggregate interface.
	for _, port := range ports {
		t.Logf("Configuring member interface %q and binding to aggregate %q...", port.Name(), lagName)
		intf := &oc.Interface{Name: ygot.String(port.Name())}
		intf.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
		intf.Enabled = ygot.Bool(true)
		eth := intf.GetOrCreateEthernet()
		eth.AggregateId = ygot.String(lagName)
		gnmi.BatchReplace(batch, gnmi.OC().Interface(port.Name()).Config(), intf)
	}

	// Execute batch
	t.Logf("Applying batch configuration for aggregate, LACP, and members...")
	batch.Set(t, dut)
	t.Logf("Successfully applied batch configuration")

	return ports, lagName
}

// configureOTG configures the OTG with LACP bundle and continuous data-plane traffic.
// Corresponds to README section "Test environment setup":
// * Start continuous data-plane traffic from the IXIA/ATE over the LACP interfaces to the DUT.
func configureOTG(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	t.Helper()
	top := gosnappi.NewConfig()
	p1 := ate.Port(t, "port1")
	p2 := ate.Port(t, "port2")
	top.Ports().Add().SetName(p1.ID())
	top.Ports().Add().SetName(p2.ID())

	lag := top.Lags().Add().SetName(ateLagName)
	lag.Protocol().Lacp().SetActorKey(1).SetActorSystemPriority(1).SetActorSystemId("00:11:01:00:00:01")
	lp1 := lag.Ports().Add().SetPortName(p1.ID())
	lp1.Ethernet().SetMac(ateSrc.MAC).SetName(p1.ID() + ".mac")
	lp1.Lacp().SetActorActivity("active").SetActorPortNumber(1).SetActorPortPriority(1).SetLacpduTimeout(0)

	lp2 := lag.Ports().Add().SetPortName(p2.ID())
	lp2.Ethernet().SetMac(ateSrc.MAC).SetName(p2.ID() + ".mac")
	lp2.Lacp().SetActorActivity("active").SetActorPortNumber(2).SetActorPortPriority(1).SetLacpduTimeout(0)

	dev := top.Devices().Add().SetName(ateSrc.Name)
	eth := dev.Ethernets().Add().SetName(ateSrc.Name + ".eth").SetMac(ateSrc.MAC)
	eth.Connection().SetLagName(lag.Name())
	ip := eth.Ipv4Addresses().Add().SetName(ateSrc.Name + ".ipv4").SetAddress(ateSrc.IPv4).SetGateway(dutSrc.IPv4).SetPrefix(uint32(ateSrc.IPv4Len))

	flow := top.Flows().Add().SetName(flowName)
	flow.Metrics().SetEnable(true)
	flow.TxRx().Device().SetTxNames([]string{ip.Name()}).SetRxNames([]string{ip.Name()})
	flow.Size().SetFixed(flowPacketSize)
	flow.Rate().SetPps(flowPPS)
	flow.Duration().Continuous()

	ethPkt := flow.Packet().Add().Ethernet()
	ethPkt.Src().SetValue(ateSrc.MAC)
	ipPkt := flow.Packet().Add().Ipv4()
	ipPkt.Src().SetValue(ateSrc.IPv4)
	ipPkt.Dst().SetValue(dutSrc.IPv4)

	return top
}

// verifyLACPState checks if the LAG interface and all member ports are UP and IN_SYNC.
// Addresses README gNOI-3.3.1 Step 3:
// * Verify the LACP session does not flap and connected ports remain up
// * Validate the member ports are in-sync
func verifyLACPState(t *testing.T, dut *ondatra.DUTDevice, ports []*ondatra.Port, lagName string) {
	t.Helper()
	t.Logf("Waiting for aggregate interface %q OperStatus=UP...", lagName)
	gnmi.Await(t, dut, gnmi.OC().Interface(lagName).OperStatus().State(), 2*time.Minute, oc.Interface_OperStatus_UP)
	t.Logf("Aggregate interface %q OperStatus is UP", lagName)

	if lagTypeVal := gnmi.Lookup(t, dut, gnmi.OC().Interface(lagName).Aggregation().LagType().State()); lagTypeVal.IsPresent() {
		val, _ := lagTypeVal.Val()
		t.Logf("Verified aggregate interface %q LagType state: %v", lagName, val)
	} else {
		t.Logf("Aggregate interface %q LagType state not reported by device telemetry", lagName)
	}

	for _, port := range ports {
		t.Logf("Waiting for member interface %q OperStatus=UP...", port.Name())
		gnmi.Await(t, dut, gnmi.OC().Interface(port.Name()).OperStatus().State(), 2*time.Minute, oc.Interface_OperStatus_UP)
		t.Logf("Member interface %q OperStatus is UP", port.Name())
		if !deviations.LACPInterfaceMemberStateInterfaceUnsupported(dut) {
			t.Logf("Waiting for member interface %q LACP synchronization=IN_SYNC...", port.Name())
			syncState := gnmi.OC().Lacp().Interface(lagName).Member(port.Name()).Synchronization()
			gnmi.Await(t, dut, syncState.State(), 2*time.Minute, oc.Lacp_LacpSynchronizationType_IN_SYNC)
			t.Logf("Member interface %q LACP synchronization is IN_SYNC", port.Name())
		}
	}
}

// verifyZeroTrafficLoss checks the continuous traffic flow on OTG and verifies 0 drop percentage.
// Addresses README gNOI-3.3.1 Step 3:
// * Validate there is zero traffic loss from IXIA over the LACP ports during the entire switchover event.
func verifyZeroTrafficLoss(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config) {
	t.Helper()
	otg := ate.OTG()
	otgutils.LogFlowMetrics(t, otg, top)
	for _, f := range top.Flows().Items() {
		otgutils.ExpectedTrafficLoss(t, otg, f.Name(), 0, 0.1)
	}
}

func TestSupervisorSwitchover(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	controllerCards := components.FindComponentsByType(t, dut, controlcardType)
	t.Logf("Found controller card list: %v", controllerCards)

	if *args.NumControllerCards >= 0 && len(controllerCards) != *args.NumControllerCards {
		t.Errorf("Incorrect number of controller cards: got %v, want exactly %v (specified by flag)", len(controllerCards), *args.NumControllerCards)
	}
	if got, want := len(controllerCards), 2; got < want {
		t.Skipf("Not enough controller cards for the test on %v: got %v, want at least %v", dut.Model(), got, want)
	}

	// 1. Test environment setup: Configure LACP port-channel and OTG.
	dutPorts, lagName := configureDUT(t, dut)
	otgTop := configureOTG(t, ate)
	otg := ate.OTG()
	otg.PushConfig(t, otgTop)
	otg.StartProtocols(t)

	verifyLACPState(t, dut, dutPorts, lagName)
	otgutils.WaitForARP(t, otg, otgTop, "IPv4")
	// Start continuous data-plane traffic. Must run continuously for the entire test suite.
	otg.StartTraffic(t)
	t.Cleanup(func() {
		otg.StopTraffic(t)
	})
	verifyZeroTrafficLoss(t, ate, otgTop)

	intfsOperStatusUPBeforeSwitch := helpers.FetchOperStatusUPIntfs(t, dut, *args.CheckInterfacesInBinding)

	t.Run("RecoveryValidation", func(t *testing.T) {
		testRecoveryValidation(t, dut, ate, otgTop, controllerCards, dutPorts, lagName, intfsOperStatusUPBeforeSwitch)
	})

	t.Run("BackToBackSwitchover", func(t *testing.T) {
		testBackToBackSwitchover(t, dut, ate, otgTop, controllerCards)
	})

	t.Run("PowerDisabledStandby", func(t *testing.T) {
		testPowerDisabledStandby(t, dut, ate, otgTop, controllerCards)
	})
}

// gNOI-3.3.1: Supervisor Switchover and Recovery Validation
func testRecoveryValidation(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, top gosnappi.Config, controllerCards []string, dutPorts []*ondatra.Port, lagName string, intfsOperStatusUPBeforeSwitch []string) {
	rpStandbyBeforeSwitch, rpActiveBeforeSwitch := components.FindStandbyControllerCard(t, dut, controllerCards)
	t.Logf("Detected rpStandby: %v, rpActive: %v", rpStandbyBeforeSwitch, rpActiveBeforeSwitch)

	switchoverReady := gnmi.OC().Component(rpActiveBeforeSwitch).SwitchoverReady()
	gnmi.Await(t, dut, switchoverReady.State(), 30*time.Minute, true)

	gnoiClient := dut.RawAPIs().GNOI(t)
	// Step 1: Issue gnoi.SwitchControlProcessor to the chassis.
	switchoverRequest := &spb.SwitchControlProcessorRequest{
		ControlProcessor: components.GetSubcomponentPath(rpStandbyBeforeSwitch, deviations.GNOISubcomponentPath(dut)),
	}
	if _, err := gnoiClient.System().SwitchControlProcessor(context.Background(), switchoverRequest); err != nil {
		t.Fatalf("Failed to initiate supervisor switchover: %v", err)
	}
	// Step 2: Validate the switchover was successful:
	// * Verify the standby RE/SUP becomes active (PRIMARY).
	// * Verify the old active RE/SUP transitions to STANDBY (SECONDARY).
	gnmi.Await(t, dut, gnmi.OC().Component(rpStandbyBeforeSwitch).RedundantRole().State(), maxSwitchoverTime, oc.Platform_ComponentRedundantRole_PRIMARY)
	gnmi.Await(t, dut, gnmi.OC().Component(rpActiveBeforeSwitch).RedundantRole().State(), maxSwitchoverTime, oc.Platform_ComponentRedundantRole_SECONDARY)

	rpStandbyAfterSwitch, rpActiveAfterSwitch := components.FindStandbyControllerCard(t, dut, controllerCards)
	t.Logf("Found standbyRP after switchover: %v, activeRP: %v", rpStandbyAfterSwitch, rpActiveAfterSwitch)

	if got, want := rpActiveAfterSwitch, rpStandbyBeforeSwitch; got != want {
		t.Errorf("Get rpActiveAfterSwitch: got %v, want %v", got, want)
	}
	if got, want := rpStandbyAfterSwitch, rpActiveBeforeSwitch; got != want {
		t.Errorf("Get rpStandbyAfterSwitch: got %v, want %v", got, want)
	}

	// Step 3: Validate traffic and LACP state during and after the switchover.
	verifyLACPState(t, dut, dutPorts, lagName)
	helpers.ValidateOperStatusUPIntfs(t, dut, intfsOperStatusUPBeforeSwitch, 5*time.Minute)
	verifyZeroTrafficLoss(t, ate, top)

	// Step 4: Validate management plane recovery.
	t.Log("Validate management plane recovery by writing and reading interface description via gNMI.")
	origDesc := "LACP Port-Channel bundle for Supervisor Switchover test"
	testDesc := fmt.Sprintf("Updated %s description post-switchover", lagName)
	t.Cleanup(func() {
		gnmi.Update(t, dut, gnmi.OC().Interface(lagName).Description().Config(), origDesc)
	})
	gnmi.Update(t, dut, gnmi.OC().Interface(lagName).Description().Config(), testDesc)
	if got, want := gnmi.Get(t, dut, gnmi.OC().Interface(lagName).Description().State()), testDesc; got != want {
		t.Errorf("Management plane recovery validation failed: got description %q, want %q", got, want)
	}

	activeRP := gnmi.OC().Component(rpActiveAfterSwitch)
	swTime, swTimePresent := gnmi.Watch(t, dut, activeRP.LastSwitchoverTime().State(), 1*time.Minute, func(val *ygnmi.Value[uint64]) bool { return val.IsPresent() }).Await(t)
	if !swTimePresent {
		t.Errorf("activeRP.LastSwitchoverTime().Watch(t).IsPresent(): got %v, want %v", false, true)
	} else {
		st, _ := swTime.Val()
		t.Logf("Found activeRP.LastSwitchoverTime(): %v", st)
	}

	if got, want := gnmi.Lookup(t, dut, activeRP.LastSwitchoverReason().State()).IsPresent(), true; got != want {
		t.Errorf("activeRP.LastSwitchoverReason().Lookup(t).IsPresent(): got %v, want %v", got, want)
	} else {
		lastSwitchoverReason := gnmi.Get(t, dut, activeRP.LastSwitchoverReason().State())
		t.Logf("Found lastSwitchoverReason.GetDetails(): %v", lastSwitchoverReason.GetDetails())
		t.Logf("Found lastSwitchoverReason.GetTrigger().String(): %v", lastSwitchoverReason.GetTrigger().String())
		if !deviations.GNOISwitchoverReasonMissingUserInitiated(dut) {
			if got, want := lastSwitchoverReason.GetTrigger(), oc.PlatformTypes_ComponentRedundantRoleSwitchoverReasonTrigger_USER_INITIATED; got != want {
				t.Errorf("lastSwitchoverReason.GetTrigger(): got %v, want %v", got, want)
			}
		}
	}
}

// gNOI-3.3.2: Back-to-Back Switchover (Negative Case)
func testBackToBackSwitchover(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, top gosnappi.Config, controllerCards []string) {
	rpStandbyBeforeSwitch, rpActiveBeforeSwitch := components.FindStandbyControllerCard(t, dut, controllerCards)
	t.Logf("Detected rpStandby for BackToBack: %v, rpActive: %v", rpStandbyBeforeSwitch, rpActiveBeforeSwitch)

	gnmi.Await(t, dut, gnmi.OC().Component(rpActiveBeforeSwitch).SwitchoverReady().State(), 30*time.Minute, true)
	gnoiClient := dut.RawAPIs().GNOI(t)
	useNameOnly := deviations.GNOISubcomponentPath(dut)
	// Step 1: Trigger an SSO via gnoi.SwitchControlProcessor.
	firstRequest := &spb.SwitchControlProcessorRequest{
		ControlProcessor: components.GetSubcomponentPath(rpStandbyBeforeSwitch, useNameOnly),
	}
	if _, err := gnoiClient.System().SwitchControlProcessor(context.Background(), firstRequest); err != nil {
		t.Fatalf("Failed to initiate first supervisor switchover: %v", err)
	}

	// Step 2: Immediately issue a second gnoi.SwitchControlProcessor request while unready.
	secondRequest := &spb.SwitchControlProcessorRequest{
		ControlProcessor: components.GetSubcomponentPath(rpActiveBeforeSwitch, useNameOnly),
	}
	t.Logf("Immediately sending second SwitchControlProcessor request targeting unready supervisor: %v", secondRequest)
	_, err := gnoiClient.System().SwitchControlProcessor(context.Background(), secondRequest)
	// Step 3: Validate the system gracefully rejects the second request or handles it safely without crashing.
	if err == nil {
		t.Errorf("Back-to-back switchover request unexpectedly succeeded while new standby was unready; expected rejection")
	} else {
		t.Logf("Back-to-back switchover request safely and correctly rejected with err: %v", err)
	}

	// Verify active supervisor maintains control and traffic continues with zero loss.
	gnmi.Await(t, dut, gnmi.OC().Component(rpStandbyBeforeSwitch).RedundantRole().State(), maxSwitchoverTime, oc.Platform_ComponentRedundantRole_PRIMARY)
	gnmi.Await(t, dut, gnmi.OC().Component(rpActiveBeforeSwitch).RedundantRole().State(), maxSwitchoverTime, oc.Platform_ComponentRedundantRole_SECONDARY)
	verifyZeroTrafficLoss(t, ate, top)
	gnmi.Await(t, dut, gnmi.OC().Component(rpStandbyBeforeSwitch).SwitchoverReady().State(), 30*time.Minute, true)
}

// gNOI-3.3.3: Switchover with Power-Disabled Standby (Negative Case)
func testPowerDisabledStandby(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, top gosnappi.Config, controllerCards []string) {
	rpStandbyBeforeSwitch, rpActiveBeforeSwitch := components.FindStandbyControllerCard(t, dut, controllerCards)
	t.Logf("Detected rpStandby for PowerDisabledStandby: %v, rpActive: %v", rpStandbyBeforeSwitch, rpActiveBeforeSwitch)

	t.Cleanup(func() {
		t.Logf("Cleaning up: Re-enabling power on standby supervisor %s to restore redundancy", rpStandbyBeforeSwitch)
		components.SetControllerCardPowerState(t, dut, rpStandbyBeforeSwitch, oc.Platform_ComponentPowerType_POWER_ENABLED, 10*time.Minute)
		gnmi.Await(t, dut, gnmi.OC().Component(rpStandbyBeforeSwitch).SwitchoverReady().State(), 30*time.Minute, true)
	})

	// Step 1: Disable the standby supervisor.
	components.SetControllerCardPowerState(t, dut, rpStandbyBeforeSwitch, oc.Platform_ComponentPowerType_POWER_DISABLED, 5*time.Minute)

	gnoiClient := dut.RawAPIs().GNOI(t)
	useNameOnly := deviations.GNOISubcomponentPath(dut)
	// Step 2: Attempt to trigger an SSO via gnoi.SwitchControlProcessor.
	switchoverRequest := &spb.SwitchControlProcessorRequest{
		ControlProcessor: components.GetSubcomponentPath(rpStandbyBeforeSwitch, useNameOnly),
	}
	t.Logf("Attempting SwitchControlProcessor request targeting power-disabled standby: %v", switchoverRequest)
	// Step 3: Verify the switchover request is rejected.
	_, err := gnoiClient.System().SwitchControlProcessor(context.Background(), switchoverRequest)
	if err == nil {
		t.Errorf("SwitchControlProcessor request to power-disabled standby unexpectedly succeeded; expected rejection")
	} else {
		t.Logf("SwitchControlProcessor request to power-disabled standby correctly rejected with err: %v", err)
	}

	// Step 4: Verify the current active supervisor safely maintains control and there is zero traffic loss.
	role := gnmi.Get(t, dut, gnmi.OC().Component(rpActiveBeforeSwitch).RedundantRole().State())
	if role != oc.Platform_ComponentRedundantRole_PRIMARY {
		t.Errorf("Active supervisor role changed unexpectedly after rejected switchover: got %v, want PRIMARY", role)
	}
	verifyZeroTrafficLoss(t, ate, top)
}
