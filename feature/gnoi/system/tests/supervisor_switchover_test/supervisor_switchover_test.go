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

	"github.com/open-traffic-generator/gosnappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/args/args"
	"github.com/openconfig/featureprofiles/internal/attrs/attrs"
	"github.com/openconfig/featureprofiles/internal/components/components"
	"github.com/openconfig/featureprofiles/internal/deviations/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest/fptest"
	"github.com/openconfig/featureprofiles/internal/gnoi/gnoi"
	"github.com/openconfig/featureprofiles/internal/helpers/helpers"
	"github.com/openconfig/featureprofiles/internal/otgutils/otgutils"
	spb "github.com/openconfig/gnoi/system"
	"github.com/openconfig/ondatra/gnmi/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc/oc"
	"github.com/openconfig/ondatra/netutil/netutil"
	"github.com/openconfig/ondatra/ondatra"
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
	dutDst = attrs.Attributes{
		Desc:    "dutDst",
		IPv4:    "198.51.100.1",
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

// replaceInterfaceConfig safely replaces interface configuration and emits actionable
// diagnostics if the vendor YANG translator rejects the interface name or type.
func replaceInterfaceConfig(t *testing.T, dut *ondatra.DUTDevice, path ygnmi.ConfigQuery[*oc.Interface], intf *oc.Interface) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			errStr := fmt.Sprintf("%v", r)
			if strings.Contains(errStr, "invalid interface type") ||
				strings.Contains(errStr, "bad-element") ||
				strings.Contains(errStr, "load failure on translation changes") ||
				strings.Contains(errStr, "could not parse json element value") ||
				strings.Contains(errStr, "Must match the pattern") {
				t.Fatalf("gNMI Replace failed with vendor schema/translation error on interface %q (type: %v).\n"+
					"DIAGNOSTIC HINT: Ensure you are not hardcoding vendor-specific LAG/interface names (e.g., using 'Port-Channel1' on JUNOS or NOKIA).\n"+
					"Always allocate LAG names dynamically using netutil.NextAggregateInterface(t, dut).\n"+
					"Original error: %v", intf.GetName(), intf.GetType(), r)
			}
			panic(r)
		}
	}()
	gnmi.Replace(t, dut, path, intf)
}

// configureDUT configures two physical ports and an LACP bundle on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) ([]*ondatra.Port, string) {
	t.Helper()
	lagName := netutil.NextAggregateInterface(t, dut)
	t.Logf("Dynamically allocated aggregate interface name for DUT %v: %q", dut.Model(), lagName)
	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")
	ports := []*ondatra.Port{p1, p2}

	// 1. Configure the aggregate LACP interface first so that lag-type
	// is present before member Ethernet interfaces reference it.
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
	replaceInterfaceConfig(t, dut, gnmi.OC().Interface(lagName).Config(), lagIntf)
	t.Logf("Successfully configured aggregate interface %q", lagName)

	if !deviations.LacpInterfaceFallbackOCUnsupported(dut) {
		t.Logf("Configuring LACP parameters on aggregate interface %q...", lagName)
		lacp := &oc.Lacp_Interface{Name: ygot.String(lagName)}
		lacp.LacpMode = oc.Lacp_LacpActivityType_ACTIVE
		lacp.Interval = oc.Lacp_LacpPeriodType_FAST
		gnmi.Replace(t, dut, gnmi.OC().Lacp().Interface(lagName).Config(), lacp)
		t.Logf("Successfully configured LACP parameters on %q", lagName)
	}

	// 2. Configure member Ethernet interfaces and bind them to the aggregate interface.
	for _, port := range ports {
		t.Logf("Configuring member interface %q and binding to aggregate %q...", port.Name(), lagName)
		intf := &oc.Interface{Name: ygot.String(port.Name())}
		intf.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
		intf.Enabled = ygot.Bool(true)
		eth := intf.GetOrCreateEthernet()
		eth.AggregateId = ygot.String(lagName)
		replaceInterfaceConfig(t, dut, gnmi.OC().Interface(port.Name()).Config(), intf)
		t.Logf("Successfully configured member interface %q", port.Name())
	}
	t.Cleanup(func() {
	t.Cleanup(func() {
		for _, port := range ports {
			gnmi.Delete(t, dut, gnmi.OC().Interface(port.Name()).Config())
		}
		gnmi.Delete(t, dut, gnmi.OC().Interface(lagName).Config())
		if !deviations.LacpInterfaceFallbackOCUnsupported(dut) {
			gnmi.Delete(t, dut, gnmi.OC().Lacp().Interface(lagName).Config())
		}
	})

	return ports, lagName
			gnmi.Delete(t, dut, gnmi.OC().Interface(port.Name()).Config())
		}
		gnmi.Delete(t, dut, gnmi.OC().Interface(lagName).Config())
		if !deviations.LacpInterfaceFallbackOCUnsupported(dut) {
			gnmi.Delete(t, dut, gnmi.OC().Lacp().Interface(lagName).Config())
		}
	})

	return ports, lagName
		}
		gnmi.Delete(t, dut, gnmi.OC().Interface(lagName).Config())
		if !deviations.LacpInterfaceFallbackOCUnsupported(dut) {
			gnmi.Delete(t, dut, gnmi.OC().Lacp().Interface(lagName).Config())
		}
	})

	return ports, lagName
}

// configureOTG configures the OTG with LACP bundle and continuous data-plane traffic.
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
func verifyZeroTrafficLoss(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config) {
	t.Helper()
	otg := ate.OTG()
	otgutils.LogFlowMetrics(t, otg, top)
	for _, f := range top.Flows().Items() {
		var txPkts, rxPkts uint64
		var lossPct float32
		_, ok := gnmi.Watch(t, otg, gnmi.OTG().Flow(f.Name()).State(), 1*time.Minute, func(val *ygnmi.Value[*otg.Flow]) bool {
			flowMetrics, present := val.Val()
			if !present {
				return false
	otgutils.WaitForARP(t, otg, otgTop, "IPv4")
			txPkts = flowMetrics.GetCounters().GetOutPkts()
			rxPkts = flowMetrics.GetCounters().GetInPkts()
			lossPct = ygot.BinaryToFloat32(flowMetrics.GetLossPct())
			return txPkts > 0
		}).Await(t)
		if !ok {
			t.Errorf("Flow %s did not transmit any packets", f.Name())
			continue
		}
		if lossPct > 0 || rxPkts < txPkts {
			t.Errorf("Flow %s experienced packet loss: Tx = %d, Rx = %d, LossPct = %f", f.Name(), txPkts, rxPkts, lossPct)
		} else {
			t.Logf("Flow %s verified zero packet loss (Tx=%d, Rx=%d)", f.Name(), txPkts, rxPkts)
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

	dutPorts, lagName := configureDUT(t, dut)
	otgTop := configureOTG(t, ate)
	otg := ate.OTG()
	otg.PushConfig(t, otgTop)
	otg.StartProtocols(t)
	time.Sleep(30 * time.Second)

	verifyLACPState(t, dut, dutPorts, lagName)
	otg.StartTraffic(t)
	time.Sleep(15 * time.Second)
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

	otg.StopTraffic(t)
}

func testRecoveryValidation(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, top gosnappi.Config, controllerCards []string, dutPorts []*ondatra.Port, lagName string, intfsOperStatusUPBeforeSwitch []string) {
	rpStandbyBeforeSwitch, rpActiveBeforeSwitch := components.FindStandbyControllerCard(t, dut, controllerCards)
	t.Logf("Detected rpStandby: %v, rpActive: %v", rpStandbyBeforeSwitch, rpActiveBeforeSwitch)

	switchoverReady := gnmi.OC().Component(rpActiveBeforeSwitch).SwitchoverReady()
	gnmi.Await(t, dut, switchoverReady.State(), 30*time.Minute, true)

	startSwitchover := time.Now()
	_ = gnoi.InitiateSupervisorSwitchover(context.Background(), t, dut, rpStandbyBeforeSwitch)
	gnoi.WaitForSwitchoverCompletion(t, dut, startSwitchover, maxSwitchoverTime)

	rpStandbyAfterSwitch, rpActiveAfterSwitch := components.FindStandbyControllerCard(t, dut, controllerCards)
	t.Logf("Found standbyRP after switchover: %v, activeRP: %v", rpStandbyAfterSwitch, rpActiveAfterSwitch)

	if got, want := rpActiveAfterSwitch, rpStandbyBeforeSwitch; got != want {
		t.Errorf("Get rpActiveAfterSwitch: got %v, want %v", got, want)
	}
	if got, want := rpStandbyAfterSwitch, rpActiveBeforeSwitch; got != want {
		t.Errorf("Get rpStandbyAfterSwitch: got %v, want %v", got, want)
	}

	verifyLACPState(t, dut, dutPorts, lagName)
	helpers.ValidateOperStatusUPIntfs(t, dut, intfsOperStatusUPBeforeSwitch, 5*time.Minute)
	verifyZeroTrafficLoss(t, ate, top)

	t.Log("Validate management plane recovery by writing and reading interface description via gNMI.")
	origDesc := "LACP Port-Channel bundle for Supervisor Switchover test"
	testDesc := fmt.Sprintf("Updated %s description post-switchover", lagName)
	gnmi.Update(t, dut, gnmi.OC().Interface(lagName).Description().Config(), testDesc)
	t.Cleanup(func() {
		gnmi.Update(t, dut, gnmi.OC().Interface(lagName).Description().Config(), origDesc)
	})
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

func testBackToBackSwitchover(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, top gosnappi.Config, controllerCards []string) {
	rpStandbyBeforeSwitch, rpActiveBeforeSwitch := components.FindStandbyControllerCard(t, dut, controllerCards)
	t.Logf("Detected rpStandby for BackToBack: %v, rpActive: %v", rpStandbyBeforeSwitch, rpActiveBeforeSwitch)

	gnmi.Await(t, dut, gnmi.OC().Component(rpActiveBeforeSwitch).SwitchoverReady().State(), 30*time.Minute, true)
	startFirst := time.Now()
	_ = gnoi.InitiateSupervisorSwitchover(context.Background(), t, dut, rpStandbyBeforeSwitch)

	gnoiClient := dut.RawAPIs().GNOI(t)
	useNameOnly := deviations.GNOISubcomponentPath(dut)
	secondRequest := &spb.SwitchControlProcessorRequest{
		ControlProcessor: components.GetSubcomponentPath(rpActiveBeforeSwitch, useNameOnly),
	}
	t.Logf("Immediately sending second SwitchControlProcessor request targeting unready supervisor: %v", secondRequest)
	_, err := gnoiClient.System().SwitchControlProcessor(context.Background(), secondRequest)
	if err == nil {
		t.Errorf("Back-to-back switchover request unexpectedly succeeded while new standby was unready; expected rejection")
	} else {
		t.Logf("Back-to-back switchover request safely and correctly rejected with err: %v", err)
	}

	gnoi.WaitForSwitchoverCompletion(t, dut, startFirst, maxSwitchoverTime)
	verifyZeroTrafficLoss(t, ate, top)
	gnmi.Await(t, dut, gnmi.OC().Component(rpStandbyBeforeSwitch).SwitchoverReady().State(), 30*time.Minute, true)
}

func testPowerDisabledStandby(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, top gosnappi.Config, controllerCards []string) {
	rpStandbyBeforeSwitch, rpActiveBeforeSwitch := components.FindStandbyControllerCard(t, dut, controllerCards)
	t.Logf("Detected rpStandby for PowerDisabledStandby: %v, rpActive: %v", rpStandbyBeforeSwitch, rpActiveBeforeSwitch)

	components.SetControllerCardPowerState(t, dut, rpStandbyBeforeSwitch, oc.Platform_ComponentPowerType_POWER_DISABLED, 5*time.Minute)

	gnoiClient := dut.RawAPIs().GNOI(t)
	useNameOnly := deviations.GNOISubcomponentPath(dut)
	switchoverRequest := &spb.SwitchControlProcessorRequest{
		ControlProcessor: components.GetSubcomponentPath(rpStandbyBeforeSwitch, useNameOnly),
	}
	t.Logf("Attempting SwitchControlProcessor request targeting power-disabled standby: %v", switchoverRequest)
	_, err := gnoiClient.System().SwitchControlProcessor(context.Background(), switchoverRequest)
	if err == nil {
		t.Errorf("SwitchControlProcessor request to power-disabled standby unexpectedly succeeded; expected rejection")
	} else {
		t.Logf("SwitchControlProcessor request to power-disabled standby correctly rejected with err: %v", err)
	}

	role := gnmi.Get(t, dut, gnmi.OC().Component(rpActiveBeforeSwitch).RedundantRole().State())
	if role != oc.Platform_ComponentRedundantRole_PRIMARY {
		t.Errorf("Active supervisor role changed unexpectedly after rejected switchover: got %v, want PRIMARY", role)
	}
	verifyZeroTrafficLoss(t, ate, top)

	t.Logf("Re-enabling power on standby supervisor %s to restore redundancy", rpStandbyBeforeSwitch)
	components.SetControllerCardPowerState(t, dut, rpStandbyBeforeSwitch, oc.Platform_ComponentPowerType_POWER_ENABLED, 10*time.Minute)
	gnmi.Await(t, dut, gnmi.OC().Component(rpStandbyBeforeSwitch).SwitchoverReady().State(), 30*time.Minute, true)
}
