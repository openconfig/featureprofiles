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

package supervisor_failure_test

import (
	"context"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	cmp "github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/gnoigo/system"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygot/ygot"

	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/gnoi"
	"github.com/openconfig/ygnmi/ygnmi"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Settings for configuring the baseline testbed with the test topology.
//
// The testbed consists of ate:port1 -> dut:port1 and dut:port2 -> ate:port2
//
//   * ate:port1 -> dut:port1 subnet 192.0.2.0/30
//   * ate:port2 -> dut:port2 subnet 192.0.2.4/30
//
//   * Destination network: 203.0.113.0/24

const (
	ipv4PrefixLen       = 30
	ateDstNetCIDR       = "203.0.113.0/24"
	ateDstNetStartIP    = "203.0.113.0"
	staticNH            = "192.0.2.6"
	nhIndex             = 1
	nhgIndex            = 42
	controlcardType     = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD
	primaryController   = oc.Platform_ComponentRedundantRole_PRIMARY
	secondaryController = oc.Platform_ComponentRedundantRole_SECONDARY
	switchTrigger       = oc.PlatformTypes_ComponentRedundantRoleSwitchoverReasonTrigger_USER_INITIATED
	maxSwitchoverTime   = 900
	flowName            = "Flow"
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "dutPort1",
		IPv4:    "192.0.2.1",
		IPv4Len: ipv4PrefixLen,
	}

	atePort1 = attrs.Attributes{
		Name:    "atePort1",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.0.2.2",
		IPv4Len: ipv4PrefixLen,
	}

	dutPort2 = attrs.Attributes{
		Desc:    "dutPort2",
		IPv4:    "192.0.2.5",
		IPv4Len: ipv4PrefixLen,
	}

	atePort2 = attrs.Attributes{
		Name:    "atePort2",
		MAC:     "02:00:02:01:01:01",
		IPv4:    "192.0.2.6",
		IPv4Len: ipv4PrefixLen,
	}
)

// configInterfaceDUT configures the interface with the Address.
func configInterfaceDUT(i *oc.Interface, a *attrs.Attributes, dut *ondatra.DUTDevice) *oc.Interface {
	i.Description = ygot.String(a.Desc)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	if deviations.InterfaceEnabled(dut) {
		i.Enabled = ygot.Bool(true)
	}

	s := i.GetOrCreateSubinterface(0)
	s4 := s.GetOrCreateIpv4()
	if deviations.InterfaceEnabled(dut) && !deviations.IPv4MissingEnabled(dut) {
		s4.Enabled = ygot.Bool(true)
	}
	s4a := s4.GetOrCreateAddress(a.IPv4)
	s4a.PrefixLength = ygot.Uint8(ipv4PrefixLen)

	return i
}

// configureDUT configures port1 and port2 on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	d := gnmi.OC()

	p1 := dut.Port(t, "port1")
	i1 := &oc.Interface{Name: ygot.String(p1.Name())}
	gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), configInterfaceDUT(i1, &dutPort1, dut))

	p2 := dut.Port(t, "port2")
	i2 := &oc.Interface{Name: ygot.String(p2.Name())}
	gnmi.Replace(t, dut, d.Interface(p2.Name()).Config(), configInterfaceDUT(i2, &dutPort2, dut))

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, p1)
		fptest.SetPortSpeed(t, p2)
	}
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, p1.Name(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, p2.Name(), deviations.DefaultNetworkInstance(dut), 0)
	}
}

// configureATE configures port1 and port2 on the ATE and adding a flow with port1 as the source and port2 as destination
func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	t.Helper()
	top := gosnappi.NewConfig()

	p1 := ate.Port(t, "port1")
	p2 := ate.Port(t, "port2")

	atePort1.AddToOTG(top, p1, &dutPort1)
	atePort2.AddToOTG(top, p2, &dutPort2)

	flow := top.Flows().Add().SetName(flowName)
	flow.Metrics().SetEnable(true)
	e1 := flow.Packet().Add().Ethernet()
	e1.Src().SetValue(atePort1.MAC)
	flow.TxRx().Device().SetTxNames([]string{atePort1.Name + ".IPv4"}).SetRxNames([]string{atePort2.Name + ".IPv4"})
	v4 := flow.Packet().Add().Ipv4()
	v4.Src().SetValue(atePort1.IPv4)
	v4.Dst().Increment().SetStart(ateDstNetStartIP).SetCount(250)

	return top
}

// Function to verify traffic
func verifyTraffic(t *testing.T, ate *ondatra.ATEDevice) {
	flowMetrics := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(flowName).Counters().State())
	txPkts := flowMetrics.GetOutPkts()
	rxPkts := flowMetrics.GetInPkts()

	if txPkts == 0 {
		t.Errorf("txPackets is 0")
		return
	}
	if got := 100 * float32(txPkts-rxPkts) / float32(txPkts); got > 0 {
		t.Errorf("LossPct for flow %s got %f, want 0", flowName, got)
	} else {
		t.Logf("Traffic flows fine from ATE-port1 to ATE-port2")
	}
}

// testArgs holds the objects needed by a test case.
type testArgs struct {
	ctx     context.Context
	clientA *gribi.Client
	dut     *ondatra.DUTDevice
	ate     *ondatra.ATEDevice
	top     gosnappi.Config
}

// routeInstall configures a IPv4 entry through clientA. Ensure that the entry via ClientA
// is active through AFT Telemetry.
func routeInstall(ctx context.Context, t *testing.T, args *testArgs) {
	// Add an IPv4Entry for 203.0.113.0/24 pointing to ATE port-2 via gRIBI-A,
	// ensure that the entry is active through AFT telemetry
	t.Logf("Add an IPv4Entry for %s pointing to ATE port-2 via gRIBI-A", ateDstNetCIDR)
	vrf := deviations.DefaultNetworkInstance(args.dut)
	args.clientA.AddNH(t, nhIndex, atePort2.IPv4, vrf, fluent.InstalledInRIB)
	args.clientA.AddNHG(t, nhgIndex, map[uint64]uint64{nhIndex: 1}, vrf, fluent.InstalledInRIB)
	args.clientA.AddIPv4(t, ateDstNetCIDR, nhgIndex, vrf, "", fluent.InstalledInRIB)
}

// findSecondaryController finds out primary and secondary controllers
func findSecondaryController(t *testing.T, dut *ondatra.DUTDevice, controllers []string) (string, string) {
	var primary, secondary string
	for _, controller := range controllers {
		role := gnmi.Get(t, dut, gnmi.OC().Component(controller).RedundantRole().State())
		t.Logf("Component(controller).RedundantRole().Get(t): %v, Role: %v", controller, role)
		if role == secondaryController {
			secondary = controller
		} else if role == primaryController {
			primary = controller
		} else {
			t.Fatalf("Expected controller %s to be active or standby, got %v", controller, role)
		}
	}
	if secondary == "" || primary == "" {
		t.Fatalf("Expected non-empty primary and secondary Controller, got primary: %v, secondary: %v", primary, secondary)
	}
	t.Logf("Detected primary: %v, secondary: %v", primary, secondary)

	return secondary, primary
}

// validateTelemetry validates telemetry sensors
func validateTelemetry(t *testing.T, dut *ondatra.DUTDevice, primaryAfterSwitch string) {
	t.Log("Validate OC Switchover time/reason.")
	primary := gnmi.OC().Component(primaryAfterSwitch)
	if !gnmi.Lookup(t, dut, primary.LastSwitchoverTime().State()).IsPresent() {
		t.Errorf("primary.LastSwitchoverTime().Lookup(t).IsPresent(): got false, want true")
	} else {
		t.Logf("Found primary.LastSwitchoverTime(): %v", gnmi.Get(t, dut, primary.LastSwitchoverTime().State()))
	}

	if !gnmi.Lookup(t, dut, primary.LastSwitchoverReason().State()).IsPresent() {
		t.Errorf("primary.LastSwitchoverReason().Lookup(t).IsPresent(): got false, want true")
	} else {
		lastSwitchoverReason := gnmi.Get(t, dut, primary.LastSwitchoverReason().State())
		t.Logf("Found lastSwitchoverReason.GetDetails(): %v", lastSwitchoverReason.GetDetails())
		t.Logf("Found lastSwitchoverReason.GetTrigger().String(): %v", lastSwitchoverReason.GetTrigger().String())
	}
	wantTrigger := switchTrigger
	if deviations.GNOISwitchoverReasonMissingUserInitiated(dut) {
		wantTrigger = oc.PlatformTypes_ComponentRedundantRoleSwitchoverReasonTrigger_SYSTEM_INITIATED
	}
	if got, want := gnmi.Get(t, dut, primary.LastSwitchoverReason().State()).GetTrigger(), wantTrigger; got != want {
		t.Errorf("primary.GetLastSwitchoverReason().GetTrigger(): got %s, want %s.", got, want)
	}

	if !gnmi.Lookup(t, dut, primary.LastRebootTime().State()).IsPresent() {
		t.Errorf("primary.LastRebootTime.().Lookup(t).IsPresent(): got false, want true")
	} else {
		lastrebootTime := gnmi.Get(t, dut, primary.LastRebootTime().State())
		t.Logf("Found lastRebootTime.GetDetails(): %v", lastrebootTime)
	}
	if !gnmi.Lookup(t, dut, primary.LastRebootReason().State()).IsPresent() {
		t.Errorf("primary.LastRebootReason.().Lookup(t).IsPresent(): got false, want true")
	} else {
		lastrebootReason := gnmi.Get(t, dut, primary.LastRebootReason().State())
		t.Logf("Found lastRebootReason.GetDetails(): %v", lastrebootReason)
	}
}

func switchoverReady(t *testing.T, dut *ondatra.DUTDevice, controller string) bool {
	switchoverReady := gnmi.OC().Component(controller).SwitchoverReady()
	_, ok := gnmi.Watch(t, dut, switchoverReady.State(), 30*time.Minute, func(val *ygnmi.Value[bool]) bool {
		ready, present := val.Val()
		return present && ready
	}).Await(t)
	return ok
}

func TestSupFailure(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ctx := context.Background()

	// Configure the DUT
	configureDUT(t, dut)

	// Configure the ATE
	ate := ondatra.ATE(t, "ate")
	top := configureATE(t, ate)
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)

	// Configure the gRIBI client clientA
	clientA := gribi.Client{
		DUT:         dut,
		FIBACK:      false,
		Persistence: true,
	}
	defer clientA.Close(t)

	// Flush all entries after test.
	defer clientA.FlushAll(t)

	if err := clientA.Start(t); err != nil {
		t.Fatalf("gRIBI Connection can not be established")
	}
	clientA.BecomeLeader(t)

	// Flush all entries before test.
	clientA.FlushAll(t)

	args := &testArgs{
		ctx:     ctx,
		clientA: &clientA,
		dut:     dut,
		ate:     ate,
		top:     top,
	}
	// Program a route and ensure AFT telemetry returns FIB_PROGRAMMED
	routeInstall(ctx, t, args)
	// Verify that static route(203.0.113.0/24) to ATE port-2 is preferred by the traffic.`
	t.Logf("Starting traffic")
	ate.OTG().StartTraffic(t)
	time.Sleep(15 * time.Second)
	ate.OTG().StopTraffic(t)
	otgutils.LogFlowMetrics(t, ate.OTG(), top)
	verifyTraffic(t, args.ate)

	controllers := cmp.FindComponentsByType(t, dut, controlcardType)
	t.Logf("Found controller list: %v", controllers)
	// Only perform the switchover for the chassis with dual controllers.
	if len(controllers) != 2 {
		t.Skipf("Dual controllers required on %v: got %v, want 2", dut.Model(), len(controllers))
	}

	secondaryBeforeSwitch, primaryBeforeSwitch := findSecondaryController(t, dut, controllers)

	if ok := switchoverReady(t, dut, primaryBeforeSwitch); !ok {
		t.Fatalf("Controller %q did not become switchover-ready before test.", primaryBeforeSwitch)
	}

	switchoverResponse := gnoi.Execute(t, dut, system.NewSwitchControlProcessorOperation().Path(cmp.GetSubcomponentPath(secondaryBeforeSwitch, deviations.GNOISubcomponentPath(dut))))
	t.Logf("gnoiClient.System().SwitchControlProcessor() response: %v", switchoverResponse)

	startSwitchover := time.Now()
	t.Logf("Wait for new Primary controller to boot up by polling the telemetry output.")
	for {
		var currentTime string
		t.Logf("Time elapsed %.2f seconds since switchover started.", time.Since(startSwitchover).Seconds())
		time.Sleep(30 * time.Second)
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			currentTime = gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State())
		}); errMsg != nil {
			t.Logf("Got testt.CaptureFatal errMsg: %s, keep polling ...", *errMsg)
		} else {
			t.Logf("Controller switchover has completed successfully with received time: %v", currentTime)
			break
		}
		if uint64(time.Since(startSwitchover).Seconds()) > maxSwitchoverTime {
			t.Fatalf("time.Since(startSwitchover): got %v, want < %v", time.Since(startSwitchover), maxSwitchoverTime)
		}
	}
	t.Logf("Controller switchover time: %.2f seconds", time.Since(startSwitchover).Seconds())

	// Old secondary controller becomes primary after switchover.
	primaryAfterSwitch := secondaryBeforeSwitch

	validateTelemetry(t, dut, primaryAfterSwitch)
	// Assume Controller Switchover happened, ensure traffic flows without loss.
	// Verify the entry for 203.0.113.0/24 is active through AFT Telemetry.
	// Try starting the gribi client twice as switchover may reset the connection.
	if err := clientA.Start(t); err != nil {
		t.Logf("gRIBI Connection could not be established: %v\nRetrying...", err)
		if err = clientA.Start(t); err != nil {
			t.Fatalf("gRIBI Connection could not be established: %v", err)
		}
	}

	// Verify the entry for 203.0.113.0/24 is active through AFT Telemetry.
	t.Logf("Verify the entry for %s is active through AFT Telemetry.", ateDstNetCIDR)
	ipv4Path := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Afts().Ipv4Entry(ateDstNetCIDR)
	if _, found := gnmi.Watch(t, args.dut, ipv4Path.State(), 2*time.Minute, func(val *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv4Entry]) bool {
		value, present := val.Val()
		return present && value.GetPrefix() == ateDstNetCIDR
	}).Await(t); !found {
		t.Fatalf("Could not find prefix %s in telemetry AFT", ateDstNetCIDR)
	}
	t.Logf("ipv4-entry found for %s after controller switchover..", ateDstNetCIDR)

	otgutils.LogFlowMetrics(t, ate.OTG(), top)
	verifyTraffic(t, args.ate)
	ate.OTG().StopTraffic(t)
	args.ate.OTG().StopProtocols(t)
}
