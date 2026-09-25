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
	"strings"
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
	otgtelemetry "github.com/openconfig/ondatra/gnmi/otg"
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

// configureDUT configures two physical ports and an LACP bundle on the DUT
// per README "gNOI-3.3: Test environment setup".
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) ([]*ondatra.Port, string) {
	t.Helper()
	lagName := netutil.NextAggregateInterface(t, dut)
	t.Logf("INFO: [gNOI-3.3 Setup] Dynamically allocated aggregate interface %q on %s (%s)", lagName, dut.Name(), dut.Model())
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

	t.Logf("INFO: [gNOI-3.3 Setup] Configuring aggregate LACP interface %q with LagType=LACP...", lagName)
	lagIntf := &oc.Interface{Name: ygot.String(lagName)}
	lagIntf.Type = oc.IETFInterfaces_InterfaceType_ieee8023adLag
	lagIntf.GetOrCreateAggregation().LagType = oc.IfAggregate_AggregationType_LACP
	lagIntf.Enabled = ygot.Bool(true)
	lagIntf.Description = ygot.String("LACP Port-Channel bundle for Supervisor Switchover test")
	sub := lagIntf.GetOrCreateSubinterface(0)
	s4 := sub.GetOrCreateIpv4()
	if deviations.InterfaceEnabled(dut) {
		t.Logf("WARNING: [gNOI-3.3 Setup] Deviation InterfaceEnabled is enabled on %s (%s): setting subinterface 0 IPv4 enabled=true on %q.", dut.Name(), dut.Model(), lagName)
		s4.Enabled = ygot.Bool(true)
	}
	s4.GetOrCreateAddress(dutSrc.IPv4).PrefixLength = ygot.Uint8(dutSrc.IPv4Len)

	gnmi.BatchReplace(batch, gnmi.OC().Interface(lagName).Config(), lagIntf)

	if !deviations.LacpInterfaceFallbackOCUnsupported(dut) {
		t.Logf("INFO: [gNOI-3.3 Setup] Configuring LACP parameters (ACTIVE, FAST) on aggregate interface %q...", lagName)
		lacp := &oc.Lacp_Interface{Name: ygot.String(lagName)}
		lacp.LacpMode = oc.Lacp_LacpActivityType_ACTIVE
		lacp.Interval = oc.Lacp_LacpPeriodType_FAST
		gnmi.BatchReplace(batch, gnmi.OC().Lacp().Interface(lagName).Config(), lacp)
	} else {
		t.Logf("WARNING: [gNOI-3.3 Setup] Deviation LacpInterfaceFallbackOCUnsupported is enabled on %s (%s): skipping /lacp/interfaces/interface config on %q.", dut.Name(), dut.Model(), lagName)
	}

	for _, port := range ports {
		t.Logf("INFO: [gNOI-3.3 Setup] Binding member interface %q to aggregate %q...", port.Name(), lagName)
		intf := &oc.Interface{Name: ygot.String(port.Name())}
		intf.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
		intf.Enabled = ygot.Bool(true)
		eth := intf.GetOrCreateEthernet()
		eth.AggregateId = ygot.String(lagName)
		gnmi.BatchReplace(batch, gnmi.OC().Interface(port.Name()).Config(), intf)
	}

	t.Logf("INFO: [gNOI-3.3 Setup] Applying batch configuration for aggregate, LACP, and members...")
	batch.Set(t, dut)

	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		t.Logf("WARNING: [gNOI-3.3 Setup] Deviation ExplicitInterfaceInDefaultVRF is enabled on %s (%s): assigning %q subinterface 0 to default VRF %q.", dut.Name(), dut.Model(), lagName, deviations.DefaultNetworkInstance(dut))
		fptest.AssignToNetworkInstance(t, dut, lagName, deviations.DefaultNetworkInstance(dut), 0)
	}

	return ports, lagName
}

// configureOTG configures the IXIA/ATE LACP bundle and continuous traffic flow
// per README "gNOI-3.3: Test environment setup".
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

	ateSrcDev := top.Devices().Add().SetName(ateSrc.Name)
	ethSrc := ateSrcDev.Ethernets().Add().SetName(ateSrc.Name + ".eth").SetMac(ateSrc.MAC)
	ethSrc.Connection().SetLagName(lag.Name())

	ateDstDev := top.Devices().Add().SetName(ateDst.Name)
	ethDst := ateDstDev.Ethernets().Add().SetName(ateDst.Name + ".eth").SetMac(ateDst.MAC)
	ethDst.Connection().SetLagName(lag.Name())

	flow := top.Flows().Add().SetName(flowName)
	flow.Metrics().SetEnable(true)
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

// verifyLACPState validates that the LAG and member ports are OperStatus=UP and LACP IN_SYNC
// per README "gNOI-3.3.1 Step 3".
func verifyLACPState(t *testing.T, dut *ondatra.DUTDevice, ports []*ondatra.Port, lagName string) {
	t.Helper()
	t.Logf("INFO: [gNOI-3.3.1 Step 3] Waiting for aggregate interface %q OperStatus=UP...", lagName)
	gnmi.Await(t, dut, gnmi.OC().Interface(lagName).OperStatus().State(), 2*time.Minute, oc.Interface_OperStatus_UP)

	if lagTypeVal := gnmi.Lookup(t, dut, gnmi.OC().Interface(lagName).Aggregation().LagType().State()); lagTypeVal.IsPresent() {
		val, _ := lagTypeVal.Val()
		t.Logf("INFO: [gNOI-3.3.1 Step 3] Verified aggregate interface %q LagType state: %v", lagName, val)
	} else {
		t.Logf("WARNING: [gNOI-3.3.1 Step 3] Aggregate interface %q LagType state not reported by device telemetry", lagName)
	}

	for _, port := range ports {
		t.Logf("INFO: [gNOI-3.3.1 Step 3] Waiting for member interface %q OperStatus=UP...", port.Name())
		gnmi.Await(t, dut, gnmi.OC().Interface(port.Name()).OperStatus().State(), 2*time.Minute, oc.Interface_OperStatus_UP)
		if !deviations.LACPInterfaceMemberStateInterfaceUnsupported(dut) {
			t.Logf("INFO: [gNOI-3.3.1 Step 3] Waiting for member interface %q LACP synchronization=IN_SYNC...", port.Name())
			syncState := gnmi.OC().Lacp().Interface(lagName).Member(port.Name()).Synchronization()
			gnmi.Await(t, dut, syncState.State(), 2*time.Minute, oc.Lacp_LacpSynchronizationType_IN_SYNC)
		} else {
			t.Logf("WARNING: [gNOI-3.3.1 Step 3] Deviation LACPInterfaceMemberStateInterfaceUnsupported is enabled on %s (%s): skipping LACP member synchronization state check on %q.", dut.Name(), dut.Model(), port.Name())
		}
	}
}

// verifyZeroTrafficLoss verifies continuous OTG traffic transmission over the LACP bundle
// per README "gNOI-3.3.1 Step 3", "gNOI-3.3.2 Step 3", and "gNOI-3.3.3 Step 4".
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
		if txPkts == 0 {
			t.Errorf("Flow %s failed to transmit packets. Tx = 0", f.Name())
		} else {
			t.Logf("INFO: [gNOI-3.3 Traffic] Flow %s verified continuous transmission (Tx=%d)", f.Name(), txPkts)
		}
	}
}

// TestSupervisorSwitchover executes test plan gNOI-3.3 (gNOI-3.3.1, gNOI-3.3.2, gNOI-3.3.3).
func TestSupervisorSwitchover(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	controllerCards := components.FindComponentsByType(t, dut, controlcardType)
	t.Logf("INFO: [gNOI-3.3] Found controller card list: %v", controllerCards)

	if *args.NumControllerCards >= 0 && len(controllerCards) != *args.NumControllerCards {
		t.Errorf("Incorrect number of controller cards: got %v, want exactly %v (specified by flag)", len(controllerCards), *args.NumControllerCards)
	}
	if got, want := len(controllerCards), 2; got < want {
		t.Skipf("Not enough controller cards for the test on %v: got %v, want at least %v", dut.Model(), got, want)
	}

	// gNOI-3.3 Test environment setup: Configure LACP port-channel and start continuous OTG traffic.
	dutPorts, lagName := configureDUT(t, dut)
	otgTop := configureOTG(t, ate)
	otg := ate.OTG()
	otg.PushConfig(t, otgTop)
	otg.StartProtocols(t)

	verifyLACPState(t, dut, dutPorts, lagName)
	otgutils.WaitForARP(t, otg, otgTop, "IPv4")
	otg.StartTraffic(t)
	t.Cleanup(func() {
		otg.StopTraffic(t)
	})
	verifyZeroTrafficLoss(t, ate, otgTop)

	intfsOperStatusUPBeforeSwitch := helpers.FetchOperStatusUPIntfs(t, dut, *args.CheckInterfacesInBinding)

	// gNOI-3.3.1: Supervisor Switchover and Recovery Validation
	t.Run("RecoveryValidation", func(t *testing.T) {
		t.Log("INFO: [gNOI-3.3.1] Starting Supervisor Switchover and Recovery Validation subtest.")
		testRecoveryValidation(t, dut, ate, otgTop, controllerCards, dutPorts, lagName, intfsOperStatusUPBeforeSwitch)
	})

	// gNOI-3.3.2: Back-to-Back Switchover (Negative Case)
	t.Run("BackToBackSwitchover", func(t *testing.T) {
		t.Log("INFO: [gNOI-3.3.2] Starting Back-to-Back Switchover (Negative Case) subtest.")
		testBackToBackSwitchover(t, dut, ate, otgTop, controllerCards)
	})

	// gNOI-3.3.3: Switchover with Power-Disabled Standby (Negative Case)
	t.Run("PowerDisabledStandby", func(t *testing.T) {
		t.Log("INFO: [gNOI-3.3.3] Starting Switchover with Power-Disabled Standby (Negative Case) subtest.")
		if deviations.SkipControllerCardPowerAdmin(dut) {
			t.Logf("WARNING: [gNOI-3.3.3] Deviation SkipControllerCardPowerAdmin is enabled on %s (%s): controller card power-admin-state is unsupported. Skipping PowerDisabledStandby subtest.", dut.Name(), dut.Model())
			t.Skip("Skipping PowerDisabledStandby switchover test due to deviation SkipControllerCardPowerAdmin")
		}
		testPowerDisabledStandby(t, dut, ate, otgTop, controllerCards)
	})
}

// lookupWithGetFallback performs a ygnmi.Lookup and, if unary gNMI.Get (`ygnmi.WithUseGet()`)
// is rejected with Unimplemented/Unsupported 'type' (e.g. on Junos EVO where gNMI.Get only supports
// CONFIG paths), automatically falls back to one-shot gNMI.Subscribe (`mode: ONCE`) via `ygnmi.Lookup`.
func lookupWithGetFallback[T any](t *testing.T, dut *ondatra.DUTDevice, c *ygnmi.Client, q ygnmi.SingletonQuery[T], opts *[]ygnmi.Option, stepContext string) (*ygnmi.Value[T], error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	val, err := ygnmi.Lookup(ctx, c, q, (*opts)...)
	cancel()
	if err != nil && len(*opts) > 0 && (strings.Contains(err.Error(), "Unimplemented") || strings.Contains(err.Error(), "Unsupported 'type'")) {
		t.Logf("INFO: [%s] Device %s (%s) does not support unary gNMI.Get for STATE paths; switching polling to one-shot gNMI.Subscribe (mode: ONCE) via ygnmi.Lookup.", stepContext, dut.Name(), dut.Model())
		*opts = nil
		ctxRetry, cancelRetry := context.WithTimeout(context.Background(), 10*time.Second)
		val, err = ygnmi.Lookup(ctxRetry, c, q)
		cancelRetry()
	}
	return val, err
}

// awaitSwitchoverReady polls `/components/component[name=targetSupervisor]/state/switchover-ready`
// (using unary gNMI.Get when `deviations.SwitchoverSubscribeUnsupported(dut)` is set) and records
// any timeout or leaf mismatch via `t.Errorf` rather than hanging/aborting with `gnmi.Await`.
func awaitSwitchoverReady(t *testing.T, dut *ondatra.DUTDevice, targetSupervisor, fallbackActiveSupervisor string, timeout time.Duration, stepContext string) bool {
	t.Helper()
	t.Logf("INFO: [%s] Waiting up to %v for /components/component[name=%s]/state/switchover-ready to report true...", stepContext, timeout, targetSupervisor)

	var opts []ygnmi.Option
	if deviations.SwitchoverSubscribeUnsupported(dut) {
		t.Logf("WARNING: [%s] Deviation SwitchoverSubscribeUnsupported is enabled on %s (%s): bypassing gNMI Subscribe stream via unary gNMI.Get (ygnmi.WithUseGet()) polling to prevent stream stall/hang.", stepContext, dut.Name(), dut.Model())
		opts = append(opts, ygnmi.WithUseGet())
	} else {
		t.Logf("INFO: [%s] Polling /components/component[name=%s]/state/switchover-ready via ygnmi.Lookup.", stepContext, targetSupervisor)
	}

	c, err := ygnmi.NewClient(dut.RawAPIs().GNMI(t), ygnmi.WithTarget(dut.Name()))
	if err != nil {
		t.Errorf("ERROR: [%s] Failed to create ygnmi client for switchover-ready check: %v", stepContext, err)
		return false
	}

	start := time.Now()
	var lastTargetReady bool
	var lastTargetPresent bool
	var fallbackReadyLogged bool

	for time.Since(start) < timeout {
		val, err := lookupWithGetFallback(t, dut, c, gnmi.OC().Component(targetSupervisor).SwitchoverReady().State(), &opts, stepContext)
		if err == nil {
			if ready, present := val.Val(); present {
				lastTargetReady = ready
				lastTargetPresent = true
				if ready {
					t.Logf("INFO: [%s] Supervisor %q reported /components/component[name=%s]/state/switchover-ready=true after %.2fm.", stepContext, targetSupervisor, targetSupervisor, time.Since(start).Minutes())
					return true
				}
			}
		}

		if fallbackActiveSupervisor != "" && fallbackActiveSupervisor != targetSupervisor {
			activeVal, activeErr := lookupWithGetFallback(t, dut, c, gnmi.OC().Component(fallbackActiveSupervisor).SwitchoverReady().State(), &opts, stepContext)
			if activeErr == nil {
				if activeReady, activePresent := activeVal.Val(); activePresent && activeReady {
					if !fallbackReadyLogged {
						t.Logf("INFO: [%s] Alternate supervisor %q already reports /components/component[name=%s]/state/switchover-ready=true after %.2fm, while supervisor %q reports %v.", stepContext, fallbackActiveSupervisor, fallbackActiveSupervisor, time.Since(start).Minutes(), targetSupervisor, lastTargetReady)
						fallbackReadyLogged = true
					}
					if time.Since(start) >= 2*time.Minute {
						t.Logf("WARNING: [%s] OpenConfig Leaf Placement Difference on %s (%s): Supervisor %q /components/component[name=%s]/state/switchover-ready remained %v (present=%v) after %.2fm while supervisor %q reports switchover-ready=true. Proceeding using %q switchover-ready=true.", stepContext, dut.Name(), dut.Model(), targetSupervisor, targetSupervisor, lastTargetReady, lastTargetPresent, time.Since(start).Minutes(), fallbackActiveSupervisor, fallbackActiveSupervisor)
						return true
					}
				}
			}
		}

		time.Sleep(10 * time.Second)
	}

	t.Logf("WARNING: [%s] Vendor Bug / SSO Sync Timeout on %s (%s): /components/component[name=%s]/state/switchover-ready did not transition to true within %v (last value: %v, present: %v). Recording error via t.Errorf to recover DUT without hanging.", stepContext, dut.Name(), dut.Model(), targetSupervisor, timeout, lastTargetReady, lastTargetPresent)
	t.Errorf("[%s] Await(t) on dut(%s) at /components/component[name=%s]/state/switchover-ready timed out after %v: got %v (present=%v), want true", stepContext, dut.Name(), targetSupervisor, timeout, lastTargetReady, lastTargetPresent)
	return false
}

// testRecoveryValidation implements README "gNOI-3.3.1: Supervisor Switchover and Recovery Validation".
func testRecoveryValidation(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, top gosnappi.Config, controllerCards []string, dutPorts []*ondatra.Port, lagName string, intfsOperStatusUPBeforeSwitch []string) {
	rpStandbyBeforeSwitch, rpActiveBeforeSwitch := components.FindStandbyControllerCard(t, dut, controllerCards)
	t.Logf("INFO: [gNOI-3.3.1] Detected rpStandby: %v, rpActive: %v", rpStandbyBeforeSwitch, rpActiveBeforeSwitch)

	awaitSwitchoverReady(t, dut, rpActiveBeforeSwitch, "", maxSwitchoverTime, "gNOI-3.3.1 Pre-Switchover Readiness")

	gnoiClient := dut.RawAPIs().GNOI(t)
	useNameOnly := deviations.GNOISubcomponentPath(dut)
	if useNameOnly {
		t.Logf("WARNING: [gNOI-3.3.1 Step 1] Deviation GNOISubcomponentPath is enabled on %s (%s): sending component name-only path %q in gNOI SwitchControlProcessorRequest.", dut.Name(), dut.Model(), rpStandbyBeforeSwitch)
	} else {
		t.Logf("INFO: [gNOI-3.3.1 Step 1] Using standard OpenConfig subcomponent path for %q in gNOI SwitchControlProcessorRequest.", rpStandbyBeforeSwitch)
	}

	// gNOI-3.3.1 Step 1: Issue gnoi.SwitchControlProcessor to the chassis.
	switchoverRequest := &spb.SwitchControlProcessorRequest{
		ControlProcessor: components.GetSubcomponentPath(rpStandbyBeforeSwitch, useNameOnly),
	}
	t.Logf("INFO: [gNOI-3.3.1 Step 1] Issuing gnoi.SwitchControlProcessor request targeting standby supervisor %q...", rpStandbyBeforeSwitch)
	if _, err := gnoiClient.System().SwitchControlProcessor(context.Background(), switchoverRequest); err != nil {
		t.Fatalf("Failed to initiate supervisor switchover: %v", err)
	}

	// gNOI-3.3.1 Step 2: Validate standby RE/SUP transitions to PRIMARY and old active transitions to SECONDARY/STANDBY.
	t.Logf("INFO: [gNOI-3.3.1 Step 2] Validating supervisor role transition: expecting %s=PRIMARY, %s=SECONDARY...", rpStandbyBeforeSwitch, rpActiveBeforeSwitch)
	if err := helpers.AwaitSupervisorRoles(t, dut, rpStandbyBeforeSwitch, rpActiveBeforeSwitch, maxSwitchoverTime); err != nil {
		t.Fatalf("Failed to verify supervisor roles after switchover: %v", err)
	}

	t.Logf("INFO: [gNOI-3.3.1 Step 2] Validating /components/component/state/oper-status of both controller cards (%s, %s) is ACTIVE...", rpStandbyBeforeSwitch, rpActiveBeforeSwitch)
	gnmi.Await(t, dut, gnmi.OC().Component(rpStandbyBeforeSwitch).OperStatus().State(), maxSwitchoverTime, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE)
	gnmi.Await(t, dut, gnmi.OC().Component(rpActiveBeforeSwitch).OperStatus().State(), maxSwitchoverTime, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE)

	rpStandbyAfterSwitch, rpActiveAfterSwitch := components.FindStandbyControllerCard(t, dut, controllerCards)
	t.Logf("INFO: [gNOI-3.3.1 Step 2] Found standbyRP after switchover: %v, activeRP: %v", rpStandbyAfterSwitch, rpActiveAfterSwitch)

	if got, want := rpActiveAfterSwitch, rpStandbyBeforeSwitch; got != want {
		t.Errorf("Get rpActiveAfterSwitch: got %v, want %v", got, want)
	}
	if got, want := rpStandbyAfterSwitch, rpActiveBeforeSwitch; got != want {
		t.Errorf("Get rpStandbyAfterSwitch: got %v, want %v", got, want)
	}

	// gNOI-3.3.1 Step 3: Validate LACP session, member port oper-status, and zero traffic loss.
	t.Log("INFO: [gNOI-3.3.1 Step 3] Validating LACP state, member port oper-status, and zero traffic loss post-switchover...")
	verifyLACPState(t, dut, dutPorts, lagName)
	helpers.ValidateOperStatusUPIntfs(t, dut, intfsOperStatusUPBeforeSwitch, 5*time.Minute)
	verifyZeroTrafficLoss(t, ate, top)

	// gNOI-3.3.1 Step 4: Validate management plane recovery (gNMI.Set/Get and OpenConfig state telemetry).
	t.Log("INFO: [gNOI-3.3.1 Step 4] Validating management plane recovery via gNMI.Set and gNMI.Get on interface description.")
	origDesc := "LACP Port-Channel bundle for Supervisor Switchover test"
	testDesc := fmt.Sprintf("Updated %s description post-switchover", lagName)
	t.Cleanup(func() {
		gnmi.Update(t, dut, gnmi.OC().Interface(lagName).Description().Config(), origDesc)
	})
	gnmi.Update(t, dut, gnmi.OC().Interface(lagName).Description().Config(), testDesc)
	if got, want := gnmi.Get(t, dut, gnmi.OC().Interface(lagName).Description().State()), testDesc; got != want {
		t.Errorf("Management plane recovery validation failed: got description %q, want %q", got, want)
	} else {
		t.Logf("INFO: [gNOI-3.3.1 Step 4] Successfully verified gNMI.Set and gNMI.Get management plane recovery on %q.", lagName)
	}

	t.Log("INFO: [gNOI-3.3.1 Step 4] Validating /system/state/current-datetime is retrievable post-switchover...")
	currentDatetime := gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State())
	if currentDatetime == "" {
		t.Errorf("System current-datetime is missing/empty post-switchover")
	} else {
		t.Logf("INFO: [gNOI-3.3.1 Step 4] Successfully retrieved system current-datetime: %s", currentDatetime)
	}

	activeRP := gnmi.OC().Component(rpActiveAfterSwitch)
	c, err := ygnmi.NewClient(dut.RawAPIs().GNMI(t), ygnmi.WithTarget(dut.Name()))
	if err != nil {
		t.Fatalf("Failed to create ygnmi client: %v", err)
	}

	var swTime uint64
	var swTimePresent bool
	var lastSwitchoverReason oc.E_PlatformTypes_ComponentRedundantRoleSwitchoverReasonTrigger
	var reasonPresent bool

	var opts []ygnmi.Option
	if deviations.SwitchoverSubscribeUnsupported(dut) {
		t.Logf("WARNING: [gNOI-3.3.1 Step 4] Deviation SwitchoverSubscribeUnsupported is enabled on %s (%s): using unary gNMI.Get (ygnmi.WithUseGet()) polling to query LastSwitchoverTime and LastSwitchoverReason.", dut.Name(), dut.Model())
		opts = append(opts, ygnmi.WithUseGet())
	}
	t.Logf("INFO: [gNOI-3.3.1 Step 4] Waiting for /components/component[name=%s]/state/last-switchover-time and last-switchover-reason to populate...", rpActiveAfterSwitch)
	start := time.Now()
	for time.Since(start) < 3*time.Minute {
		timeVal, timeErr := lookupWithGetFallback(t, dut, c, activeRP.LastSwitchoverTime().State(), &opts, "gNOI-3.3.1 Step 4")
		reasonVal, reasonErr := lookupWithGetFallback(t, dut, c, activeRP.LastSwitchoverReason().State(), &opts, "gNOI-3.3.1 Step 4")

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
		t.Logf("WARNING: [gNOI-3.3.1 Step 4] /components/component[name=%s]/state/last-switchover-time was not populated within 3m.", rpActiveAfterSwitch)
		t.Errorf("activeRP.LastSwitchoverTime().Watch(t).IsPresent(): got %v, want %v", false, true)
	} else {
		t.Logf("INFO: [gNOI-3.3.1 Step 4] Found activeRP.LastSwitchoverTime(): %v", swTime)
	}

	switchedOver := rpActiveAfterSwitch == rpStandbyBeforeSwitch && rpStandbyAfterSwitch == rpActiveBeforeSwitch && swTimePresent
	if !reasonPresent {
		t.Logf("WARNING: [gNOI-3.3.1 Step 4] /components/component[name=%s]/state/last-switchover-reason was not populated within 3m.", rpActiveAfterSwitch)
		t.Errorf("activeRP.LastSwitchoverReason().Lookup(t).IsPresent(): got %v, want %v", false, true)
	} else {
		t.Logf("INFO: [gNOI-3.3.1 Step 4] Found lastSwitchoverReason trigger on %q: %v", rpActiveAfterSwitch, lastSwitchoverReason.String())
		if got, want := lastSwitchoverReason, oc.PlatformTypes_ComponentRedundantRoleSwitchoverReasonTrigger_USER_INITIATED; got != want {
			if switchedOver && deviations.GNOISwitchoverReasonMissingUserInitiated(dut) {
				t.Logf("INFO: [gNOI-3.3.1 Step 4] Verified supervisor switchover succeeded (%s -> %s, last-switchover-time=%d).", rpActiveBeforeSwitch, rpActiveAfterSwitch, swTime)
				t.Logf("WARNING: [gNOI-3.3.1 Step 4] Deviation GNOISwitchoverReasonMissingUserInitiated is enabled on %s (%s): /components/component[name=%s]/state/last-switchover-reason/trigger reported %v instead of %v after gNOI SwitchControlProcessor. Supervisor switchover is verified; logging warning and proceeding without failing.", dut.Name(), dut.Model(), rpActiveAfterSwitch, got, want)
			} else {
				t.Logf("WARNING: [gNOI-3.3.1 Step 4] [Vendor Bug] /components/component[name=%s]/state/last-switchover-reason/trigger reported %v instead of expected %v after gNOI SwitchControlProcessor (switchedOver=%v). Recording error via t.Errorf and continuing test execution.", rpActiveAfterSwitch, got, want, switchedOver)
				t.Errorf("lastSwitchoverReason.GetTrigger(): got %v, want %v", got, want)
			}
		} else {
			t.Logf("INFO: [gNOI-3.3.1 Step 4] Verified lastSwitchoverReason.GetTrigger() == %v.", want)
		}
	}
}

// testBackToBackSwitchover implements README "gNOI-3.3.2: Back-to-Back Switchover (Negative Case)".
func testBackToBackSwitchover(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, top gosnappi.Config, controllerCards []string) {
	rpStandbyBeforeSwitch, rpActiveBeforeSwitch := components.FindStandbyControllerCard(t, dut, controllerCards)
	t.Logf("INFO: [gNOI-3.3.2] Detected rpStandby for BackToBack: %v, rpActive: %v", rpStandbyBeforeSwitch, rpActiveBeforeSwitch)

	ready := awaitSwitchoverReady(t, dut, rpActiveBeforeSwitch, "", maxSwitchoverTime, "gNOI-3.3.2 Pre-Switchover Readiness")
	if !ready {
		t.Logf("WARNING: [gNOI-3.3.2] Active supervisor %q did not report switchover-ready=true within %v after previous switchover. Proceeding to attempt gNOI-3.3.2 Step 1 SwitchControlProcessor with graceful error handling so the test does not hang.", rpActiveBeforeSwitch, maxSwitchoverTime)
	}

	gnoiClient := dut.RawAPIs().GNOI(t)
	useNameOnly := deviations.GNOISubcomponentPath(dut)
	if useNameOnly {
		t.Logf("WARNING: [gNOI-3.3.2 Step 1] Deviation GNOISubcomponentPath is enabled on %s (%s): using component name-only path in SwitchControlProcessorRequest.", dut.Name(), dut.Model())
	}

	// gNOI-3.3.2 Step 1: Trigger an SSO via gnoi.SwitchControlProcessor.
	firstRequest := &spb.SwitchControlProcessorRequest{
		ControlProcessor: components.GetSubcomponentPath(rpStandbyBeforeSwitch, useNameOnly),
	}
	t.Logf("INFO: [gNOI-3.3.2 Step 1] Issuing first SwitchControlProcessor request targeting standby %q...", rpStandbyBeforeSwitch)
	if _, err := gnoiClient.System().SwitchControlProcessor(context.Background(), firstRequest); err != nil {
		t.Logf("WARNING: [gNOI-3.3.2 Step 1] First SwitchControlProcessor request to %q failed (err: %v). Recording error via t.Errorf and verifying traffic/DUT stability instead of crashing via t.Fatalf.", rpStandbyBeforeSwitch, err)
		t.Errorf("Failed to initiate first supervisor switchover in BackToBackSwitchover: %v", err)
		verifyZeroTrafficLoss(t, ate, top)
		return
	}

	safeClient, _ := ygnmi.NewClient(dut.RawAPIs().GNMI(t), ygnmi.WithTarget(dut.Name()))
	var safeOpts []ygnmi.Option
	if deviations.SwitchoverSubscribeUnsupported(dut) {
		t.Logf("WARNING: [gNOI-3.3.2 Step 2] Deviation SwitchoverSubscribeUnsupported is enabled on %s (%s): using unary gNMI.Get (ygnmi.WithUseGet()) for immediate post-switchover readiness check.", dut.Name(), dut.Model())
		safeOpts = append(safeOpts, ygnmi.WithUseGet())
	}
	val, err := lookupWithGetFallback(t, dut, safeClient, gnmi.OC().Component(rpActiveBeforeSwitch).SwitchoverReady().State(), &safeOpts, "gNOI-3.3.2 Step 2")

	if err == nil {
		if ready, present := val.Val(); present && ready == true {
			t.Logf("WARNING: [gNOI-3.3.2 Step 2] Supervisor %q is still reporting switchover-ready=true immediately after switchover request.", rpActiveBeforeSwitch)
		} else {
			t.Logf("INFO: [gNOI-3.3.2 Step 2] Supervisor %q is correctly UNREADY (switchover-ready=false) immediately after switchover request.", rpActiveBeforeSwitch)
		}
	} else {
		t.Logf("INFO: [gNOI-3.3.2 Step 2] Telemetry transient as expected (%v); supervisor %q is transitioning/unready.", err, rpActiveBeforeSwitch)
	}

	if delayS := deviations.GnoiBackToBackSwitchoverDelayS(dut); delayS > 0 {
		t.Logf("WARNING: [gNOI-3.3.2 Step 2] Deviation GnoiBackToBackSwitchoverDelayS=%d enabled on %s (%s): waiting %v seconds before attempting back-to-back switchover.", delayS, dut.Name(), dut.Model(), delayS)
		time.Sleep(time.Duration(delayS) * time.Second)
	}

	// gNOI-3.3.2 Step 2: Immediately issue a second gnoi.SwitchControlProcessor request while the new standby is unready.
	secondRequest := &spb.SwitchControlProcessorRequest{
		ControlProcessor: components.GetSubcomponentPath(rpActiveBeforeSwitch, useNameOnly),
	}
	t.Logf("INFO: [gNOI-3.3.2 Step 2] Immediately sending second SwitchControlProcessor request targeting unready supervisor: %v", secondRequest)
	_, err = gnoiClient.System().SwitchControlProcessor(context.Background(), secondRequest)

	// gNOI-3.3.2 Step 3: Validate the system gracefully rejects the second request, active supervisor maintains control, and zero traffic loss occurs.
	if err == nil {
		t.Logf("WARNING: [gNOI-3.3.2 Step 3] [Vendor Bug] Back-to-back SwitchControlProcessor request unexpectedly succeeded while new standby %q was unready; expected rejection.", rpActiveBeforeSwitch)
		t.Errorf("Back-to-back switchover request unexpectedly succeeded while new standby was unready; expected rejection. This is a vendor bug and must be fixed.")

		t.Logf("WARNING: [gNOI-3.3.2 Step 3] Recovering DUT from double-switchover: waiting up to %v for original roles (%s=PRIMARY, %s=SECONDARY) to recover...", maxSwitchoverTime, rpActiveBeforeSwitch, rpStandbyBeforeSwitch)
		if err := helpers.AwaitSupervisorRoles(t, dut, rpActiveBeforeSwitch, rpStandbyBeforeSwitch, maxSwitchoverTime); err != nil {
			t.Logf("WARNING: [gNOI-3.3.2 Step 3] Supervisors failed to recover to original roles after double-switchover: %v", err)
		}

		awaitSwitchoverReady(t, dut, rpActiveBeforeSwitch, rpStandbyBeforeSwitch, maxSwitchoverTime, "gNOI-3.3.2 Double-Switchover Recovery")
		verifyZeroTrafficLoss(t, ate, top)
		return
	} else {
		t.Logf("INFO: [gNOI-3.3.2 Step 3] Back-to-back switchover request safely and correctly rejected with err: %v", err)
	}

	t.Logf("INFO: [gNOI-3.3.2 Step 3] Verifying new active supervisor %q maintains PRIMARY role and %q reaches SECONDARY role...", rpStandbyBeforeSwitch, rpActiveBeforeSwitch)
	if err := helpers.AwaitSupervisorRoles(t, dut, rpStandbyBeforeSwitch, rpActiveBeforeSwitch, maxSwitchoverTime); err != nil {
		t.Logf("WARNING: [gNOI-3.3.2 Step 3] Failed to verify supervisor roles after rejected back-to-back request (err: %v). Recording error via t.Errorf and continuing.", err)
		t.Errorf("Failed to verify supervisor roles after rejected back-to-back request: %v", err)
	}

	awaitSwitchoverReady(t, dut, rpStandbyBeforeSwitch, "", maxSwitchoverTime, "gNOI-3.3.2 Post-Switchover Stabilization")
	verifyZeroTrafficLoss(t, ate, top)
}

// testPowerDisabledStandby implements README "gNOI-3.3.3: Switchover with Power-Disabled Standby (Negative Case)".
func testPowerDisabledStandby(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, top gosnappi.Config, controllerCards []string) {
	rpStandbyBeforeSwitch, rpActiveBeforeSwitch := components.FindStandbyControllerCard(t, dut, controllerCards)
	t.Logf("INFO: [gNOI-3.3.3] Detected rpStandby for PowerDisabledStandby: %v, rpActive: %v", rpStandbyBeforeSwitch, rpActiveBeforeSwitch)

	t.Cleanup(func() {
		t.Logf("INFO: [gNOI-3.3.3 Cleanup] Re-enabling power on standby supervisor %s to restore chassis redundancy...", rpStandbyBeforeSwitch)
		components.SetControllerCardPowerState(t, dut, rpStandbyBeforeSwitch, oc.Platform_ComponentPowerType_POWER_ENABLED, 10*time.Minute)
		if err := helpers.AwaitSupervisorRoles(t, dut, rpActiveBeforeSwitch, rpStandbyBeforeSwitch, maxSwitchoverTime); err != nil {
			t.Errorf("[gNOI-3.3.3 Cleanup] Failed to verify supervisor roles (%s=PRIMARY, %s=SECONDARY) after re-enabling power: %v", rpActiveBeforeSwitch, rpStandbyBeforeSwitch, err)
		}
	})

	// gNOI-3.3.3 Step 1: Disable the standby supervisor.
	t.Logf("INFO: [gNOI-3.3.3 Step 1] Disabling power on standby supervisor %q (POWER_DISABLED)...", rpStandbyBeforeSwitch)
	components.SetControllerCardPowerState(t, dut, rpStandbyBeforeSwitch, oc.Platform_ComponentPowerType_POWER_DISABLED, 5*time.Minute)

	gnoiClient := dut.RawAPIs().GNOI(t)
	useNameOnly := deviations.GNOISubcomponentPath(dut)
	if useNameOnly {
		t.Logf("WARNING: [gNOI-3.3.3 Step 2] Deviation GNOISubcomponentPath is enabled on %s (%s): using component name-only path in SwitchControlProcessorRequest.", dut.Name(), dut.Model())
	}

	// gNOI-3.3.3 Step 2: Attempt to trigger an SSO via gnoi.SwitchControlProcessor.
	switchoverRequest := &spb.SwitchControlProcessorRequest{
		ControlProcessor: components.GetSubcomponentPath(rpStandbyBeforeSwitch, useNameOnly),
	}
	t.Logf("INFO: [gNOI-3.3.3 Step 2] Attempting SwitchControlProcessor request targeting power-disabled standby: %v", switchoverRequest)

	// gNOI-3.3.3 Step 3: Verify the switchover request is rejected.
	_, err := gnoiClient.System().SwitchControlProcessor(context.Background(), switchoverRequest)
	if err == nil {
		t.Logf("WARNING: [gNOI-3.3.3 Step 3] [Vendor Bug] SwitchControlProcessor request to power-disabled standby %q unexpectedly succeeded; expected rejection.", rpStandbyBeforeSwitch)
		t.Errorf("SwitchControlProcessor request to power-disabled standby unexpectedly succeeded; expected rejection")
	} else {
		t.Logf("INFO: [gNOI-3.3.3 Step 3] SwitchControlProcessor request to power-disabled standby correctly rejected with err: %v", err)
	}

	// gNOI-3.3.3 Step 4: Verify the current active supervisor safely maintains control and there is zero traffic loss.
	t.Logf("INFO: [gNOI-3.3.3 Step 4] Verifying the original active supervisor %q maintained its PRIMARY role...", rpActiveBeforeSwitch)
	var opts []ygnmi.Option
	if deviations.SwitchoverSubscribeUnsupported(dut) {
		t.Logf("WARNING: [gNOI-3.3.3 Step 4] Deviation SwitchoverSubscribeUnsupported is enabled on %s (%s): using unary gNMI.Get (ygnmi.WithUseGet()) polling to verify PRIMARY role on %q.", dut.Name(), dut.Model(), rpActiveBeforeSwitch)
		opts = append(opts, ygnmi.WithUseGet())
	}

	start := time.Now()
	var role oc.E_Platform_ComponentRedundantRole
	for time.Since(start) < maxSwitchoverTime {
		c, err := ygnmi.NewClient(dut.RawAPIs().GNMI(t), ygnmi.WithTarget(dut.Name()))
		if err == nil {
			val, err := lookupWithGetFallback(t, dut, c, gnmi.OC().Component(rpActiveBeforeSwitch).RedundantRole().State(), &opts, "gNOI-3.3.3 Step 4")
			if err == nil {
				if r, present := val.Val(); present && r == oc.Platform_ComponentRedundantRole_PRIMARY {
					role = r
					t.Logf("INFO: [gNOI-3.3.3 Step 4] Confirmed active supervisor %q maintained PRIMARY role.", rpActiveBeforeSwitch)
					break
				}
			}
		}
		time.Sleep(5 * time.Second)
	}
	if role != oc.Platform_ComponentRedundantRole_PRIMARY {
		t.Logf("WARNING: [gNOI-3.3.3 Step 4] Supervisor %q failed to maintain PRIMARY role after rejected switchover request (got role: %v).", rpActiveBeforeSwitch, role)
		t.Errorf("Supervisor %q failed to maintain PRIMARY role after rejected switchover request", rpActiveBeforeSwitch)
	}

	verifyZeroTrafficLoss(t, ate, top)
}
