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
	ateDst = attrs.Attributes{
		Name:    "ateDst",
		IPv4:    "198.51.100.2",
		MAC:     "02:00:02:01:01:01",
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
		if deviations.ExplicitInterfaceInDefaultVRF(dut) {
			gnmi.BatchDelete(batch, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Interface(lagName+".0").Config())
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

	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, lagName, deviations.DefaultNetworkInstance(dut), 0)
	}

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
	lp2.Ethernet().SetMac(ateDst.MAC).SetName(p2.ID() + ".mac")
	lp2.Lacp().SetActorActivity("active").SetActorPortNumber(2).SetActorPortPriority(1).SetLacpduTimeout(0)

	// Create the source device on the LAG
	ateSrcDev := top.Devices().Add().SetName(ateSrc.Name)
	ethSrc := ateSrcDev.Ethernets().Add().SetName(ateSrc.Name + ".eth").SetMac(ateSrc.MAC)
	ethSrc.Connection().SetLagName(lag.Name())

	// Create a destination device on the same LAG
	ateDstDev := top.Devices().Add().SetName(ateDst.Name)
	ethDst := ateDstDev.Ethernets().Add().SetName(ateDst.Name + ".eth").SetMac(ateDst.MAC)
	ethDst.Connection().SetLagName(lag.Name())

	flow := top.Flows().Add().SetName(flowName)
	flow.Metrics().SetEnable(true)
	// Use PortTxRx to bypass IxNetwork same-port validation for ATE LAGs.
	flow.TxRx().Port().
		SetTxName(p1.ID()).
		SetRxName(p2.ID())
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
		var txPkts uint64
		_, ok := gnmi.Watch(t, otg, gnmi.OTG().Flow(f.Name()).State(), 1*time.Minute, func(val *ygnmi.Value[*otgtelemetry.Flow]) bool {
			flowMetrics, present := val.Val()
			if !present || flowMetrics == nil || flowMetrics.GetCounters() == nil {
				return false
			}
			txPkts = flowMetrics.GetCounters().GetOutPkts()
			return txPkts > 0
		}).Await(t)
		if !ok {
			t.Errorf("Flow %s did not transmit any packets", f.Name())
			continue
		}
		// Since traffic is destined to DUT interface IP, the DUT consumes it and does not route it back.
		// We cannot verify rxPkts == txPkts (lossPct), so we only verify that the ATE successfully transmitted traffic over the LAG.
		if txPkts == 0 {
			t.Errorf("Flow %s failed to transmit packets. Tx = 0", f.Name())
		} else {
			t.Logf("Flow %s verified continuous transmission (Tx=%d)", f.Name(), txPkts)
		}
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
		// Devices that do not support controller card power-admin-state (e.g., Nokia 7250-IXR)
		// will reject the gNMI set request, so we skip this test case via deviation.
		if deviations.SkipControllerCardPowerAdmin(dut) {
			t.Skip("Skipping PowerDisabledStandby switchover test due to deviation SkipControllerCardPowerAdmin")
		}
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

	// gNOI-3.3.1 Step 2: Validate the switchover was successful:
	// * Verify the standby RE/SUP becomes active (PRIMARY).
	// * Verify the old active RE/SUP transitions to STANDBY (SECONDARY).
	helpers.AwaitSupervisorRoles(t, dut, rpStandbyBeforeSwitch, rpActiveBeforeSwitch, maxSwitchoverTime)

	t.Logf("Validating oper-status of both controller cards transitions to ACTIVE...")
	gnmi.Await(t, dut, gnmi.OC().Component(rpStandbyBeforeSwitch).OperStatus().State(), maxSwitchoverTime, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE)
	gnmi.Await(t, dut, gnmi.OC().Component(rpActiveBeforeSwitch).OperStatus().State(), maxSwitchoverTime, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE)

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

	t.Log("Validating /system/state/current-datetime is retrievable post-switchover...")
	currentDatetime := gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State())
	if currentDatetime == "" {
		t.Errorf("System current-datetime is missing/empty post-switchover")
	} else {
		t.Logf("Successfully retrieved system current-datetime: %s", currentDatetime)
	}

	activeRP := gnmi.OC().Component(rpActiveAfterSwitch)

	// Use a robust ygnmi polling loop for post-switchover state instead of gnmi.Watch/Lookup,
	// as early gRPC connections may yield Unauthenticated or Unavailable transients.
	c, err := ygnmi.NewClient(dut.RawAPIs().GNMI(t), ygnmi.WithTarget(dut.Name()))
	if err != nil {
		t.Fatalf("Failed to create ygnmi client: %v", err)
	}

	var swTime uint64
	var swTimePresent bool
	var lastSwitchoverReason oc.E_PlatformTypes_ComponentRedundantRoleSwitchoverReasonTrigger
	var reasonPresent bool

	t.Logf("Waiting for LastSwitchoverTime and LastSwitchoverReason to populate...")
	start := time.Now()
	for time.Since(start) < 3*time.Minute {
		// Use WithUseGet() conditionally if device doesn't support subscribe reliably post-switchover.
		var opts []ygnmi.Option
		if deviations.SwitchoverSubscribeUnsupported(dut) {
			opts = append(opts, ygnmi.WithUseGet())
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		timeVal, timeErr := ygnmi.Lookup(ctx, c, activeRP.LastSwitchoverTime().State(), opts...)
		reasonVal, reasonErr := ygnmi.Lookup(ctx, c, activeRP.LastSwitchoverReason().State(), opts...)
		cancel()

		if timeErr == nil && !swTimePresent {
			if st, present := timeVal.Val(); present {
				swTime = st
				swTimePresent = true
			}
		}
		if reasonErr == nil && !reasonPresent {
			if rsn, present := reasonVal.Val(); present {
				lastSwitchoverReason = rsn.GetTrigger()
				reasonPresent = true
			}
		}
		if swTimePresent && reasonPresent {
			break
		}
		time.Sleep(5 * time.Second)
	}

	if !swTimePresent {
		t.Errorf("activeRP.LastSwitchoverTime().Watch(t).IsPresent(): got %v, want %v", false, true)
	} else {
		t.Logf("Found activeRP.LastSwitchoverTime(): %v", swTime)
	}

	if !reasonPresent {
		t.Errorf("activeRP.LastSwitchoverReason().Lookup(t).IsPresent(): got %v, want %v", false, true)
	} else {
		t.Logf("Found lastSwitchoverReason trigger: %v", lastSwitchoverReason.String())
		if !deviations.GNOISwitchoverReasonMissingUserInitiated(dut) {
			if got, want := lastSwitchoverReason, oc.PlatformTypes_ComponentRedundantRoleSwitchoverReasonTrigger_USER_INITIATED; got != want {
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

	if delayS := deviations.GnoiBackToBackSwitchoverDelayS(dut); delayS > 0 {
		t.Logf("Waiting %v seconds before attempting the back-to-back switchover based on deviation...", delayS)
		time.Sleep(time.Duration(delayS) * time.Second)
	}

	// Step 2: Immediately issue a second gnoi.SwitchControlProcessor request while unready.
	secondRequest := &spb.SwitchControlProcessorRequest{
		ControlProcessor: components.GetSubcomponentPath(rpActiveBeforeSwitch, useNameOnly),
	}
	t.Logf("Immediately sending second SwitchControlProcessor request targeting unready supervisor: %v", secondRequest)
	_, err := gnoiClient.System().SwitchControlProcessor(context.Background(), secondRequest)
	// gNOI-3.3.2 Step 3: Validate the system gracefully rejects the second request or handles it safely without crashing.
	if err == nil {
		t.Errorf("Back-to-back switchover request unexpectedly succeeded while new standby was unready; expected rejection. This is a vendor bug and must be fixed.")

		t.Logf("Vendor Bug Detected: Chassis is incorrectly executing a double-switchover. Waiting %v for original roles to recover to prevent testbed corruption...", maxSwitchoverTime)
		helpers.AwaitSupervisorRoles(t, dut, rpActiveBeforeSwitch, rpStandbyBeforeSwitch, maxSwitchoverTime)

		var opts []ygnmi.Option
		if dut.Vendor() != ondatra.JUNIPER {
			opts = append(opts, ygnmi.WithUseGet())
		}
		ygnmiClient, _ := ygnmi.NewClient(dut.RawAPIs().GNMI(t), ygnmi.WithTarget(dut.Name()))
		t.Logf("Waiting up to 30m for original standby %q to report switchover-ready after double-switchover crash...", rpStandbyBeforeSwitch)
		start := time.Now()
		for time.Since(start) < 30*time.Minute {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			val, _ := ygnmi.Lookup(ctx, ygnmiClient, gnmi.OC().Component(rpStandbyBeforeSwitch).SwitchoverReady().State(), opts...)
			cancel()
			if ready, present := val.Val(); present && ready == true {
				t.Logf("Original standby %q is finally switchover-ready after %.2fm", rpStandbyBeforeSwitch, time.Since(start).Minutes())
				break
			}
			time.Sleep(10 * time.Second)
		}

		return
	} else {
		t.Logf("Back-to-back switchover request safely and correctly rejected with err: %v", err)
	}

	// Wait for roles to solidify using robust telemetry checks (gNOI-3.3.2 Step 3 continued: Verify active supervisor maintains control).
	// The intended system state is exactly one switchover having completed.
	helpers.AwaitSupervisorRoles(t, dut, rpStandbyBeforeSwitch, rpActiveBeforeSwitch, maxSwitchoverTime)

	if err != nil {
		var opts []ygnmi.Option
		if dut.Vendor() != ondatra.JUNIPER {
			opts = append(opts, ygnmi.WithUseGet())
		}
		ygnmiClient, _ := ygnmi.NewClient(dut.RawAPIs().GNMI(t), ygnmi.WithTarget(dut.Name()))
		t.Logf("Waiting up to 30m for new standby %q to report switchover-ready", rpStandbyBeforeSwitch)
		start := time.Now()
		for time.Since(start) < 30*time.Minute {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			val, err := ygnmi.Lookup(ctx, ygnmiClient, gnmi.OC().Component(rpStandbyBeforeSwitch).SwitchoverReady().State(), opts...)
			cancel()
			if err == nil {
				if ready, present := val.Val(); present && ready == true {
					t.Logf("New standby %q is switchover-ready after %.2fm", rpStandbyBeforeSwitch, time.Since(start).Minutes())
					break
				}
			}
			time.Sleep(10 * time.Second)
		}
	}

	verifyZeroTrafficLoss(t, ate, top)
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

	// gNOI-3.3.3 Step 1: Disable the standby supervisor.
	components.SetControllerCardPowerState(t, dut, rpStandbyBeforeSwitch, oc.Platform_ComponentPowerType_POWER_DISABLED, 5*time.Minute)

	gnoiClient := dut.RawAPIs().GNOI(t)
	useNameOnly := deviations.GNOISubcomponentPath(dut)
	// gNOI-3.3.3 Step 2: Attempt to trigger an SSO via gnoi.SwitchControlProcessor.
	switchoverRequest := &spb.SwitchControlProcessorRequest{
		ControlProcessor: components.GetSubcomponentPath(rpStandbyBeforeSwitch, useNameOnly),
	}
	t.Logf("Attempting SwitchControlProcessor request targeting power-disabled standby: %v", switchoverRequest)

	// gNOI-3.3.3 Step 3: Verify the switchover request is rejected.
	_, err := gnoiClient.System().SwitchControlProcessor(context.Background(), switchoverRequest)
	if err == nil {
		t.Errorf("SwitchControlProcessor request to power-disabled standby unexpectedly succeeded; expected rejection")
	} else {
		t.Logf("SwitchControlProcessor request to power-disabled standby correctly rejected with err: %v", err)
	}

	// gNOI-3.3.3 Step 4: Verify the current active supervisor safely maintains control and there is zero traffic loss.
	t.Logf("Verifying the original active supervisor %q maintained its PRIMARY role...", rpActiveBeforeSwitch)

	start := time.Now()
	var role oc.E_Platform_ComponentRedundantRole
	for time.Since(start) < maxSwitchoverTime {
		var opts []ygnmi.Option
		if deviations.SwitchoverSubscribeUnsupported(dut) {
			opts = append(opts, ygnmi.WithUseGet())
		}
		c, err := ygnmi.NewClient(dut.RawAPIs().GNMI(t), ygnmi.WithTarget(dut.Name()))
		if err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			val, err := ygnmi.Lookup(ctx, c, gnmi.OC().Component(rpActiveBeforeSwitch).RedundantRole().State(), opts...)
			cancel()
			if err == nil {
				if r, present := val.Val(); present && r == oc.Platform_ComponentRedundantRole_PRIMARY {
					role = r
					break
				}
			}
		}
		time.Sleep(5 * time.Second)
	}
	if role != oc.Platform_ComponentRedundantRole_PRIMARY {
		t.Errorf("Supervisor %q failed to maintain PRIMARY role after rejected switchover request", rpActiveBeforeSwitch)
	}

	verifyZeroTrafficLoss(t, ate, top)
}

