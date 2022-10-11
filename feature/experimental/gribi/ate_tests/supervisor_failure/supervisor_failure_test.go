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

	"github.com/openconfig/featureprofiles/internal/attrs"
	cmp "github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygot/ygot"

	spb "github.com/openconfig/gnoi/system"
	tpb "github.com/openconfig/gnoi/types"
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
	instance            = "DEFAULT"
	ateDstNetCIDR       = "203.0.113.0/24"
	staticNH            = "192.0.2.6"
	nhIndex             = 1
	nhgIndex            = 42
	controlcardType     = telemetry.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD
	primaryController   = telemetry.PlatformTypes_ComponentRedundantRole_PRIMARY
	secondaryController = telemetry.PlatformTypes_ComponentRedundantRole_SECONDARY
	switchTrigger       = telemetry.PlatformTypes_ComponentRedundantRoleSwitchoverReasonTrigger_USER_INITIATED
	maxSwitchoverTime   = 900
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "dutPort1",
		IPv4:    "192.0.2.1",
		IPv4Len: ipv4PrefixLen,
	}

	atePort1 = attrs.Attributes{
		Name:    "atePort1",
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
		IPv4:    "192.0.2.6",
		IPv4Len: ipv4PrefixLen,
	}
)

// configInterfaceDUT configures the interface with the Address.
func configInterfaceDUT(i *telemetry.Interface, a *attrs.Attributes) *telemetry.Interface {
	i.Description = ygot.String(a.Desc)
	i.Type = telemetry.IETFInterfaces_InterfaceType_ethernetCsmacd
	if *deviations.InterfaceEnabled {
		i.Enabled = ygot.Bool(true)
	}

	s := i.GetOrCreateSubinterface(0)
	s4 := s.GetOrCreateIpv4()
	if *deviations.InterfaceEnabled {
		s4.Enabled = ygot.Bool(true)
	}
	s4a := s4.GetOrCreateAddress(a.IPv4)
	s4a.PrefixLength = ygot.Uint8(ipv4PrefixLen)

	return i
}

// configureDUT configures port1 and port2 on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	d := dut.Config()

	p1 := dut.Port(t, "port1")
	i1 := &telemetry.Interface{Name: ygot.String(p1.Name())}
	d.Interface(p1.Name()).Replace(t, configInterfaceDUT(i1, &dutPort1))

	p2 := dut.Port(t, "port2")
	i2 := &telemetry.Interface{Name: ygot.String(p2.Name())}
	d.Interface(p2.Name()).Replace(t, configInterfaceDUT(i2, &dutPort2))

}

// configureATE configures port1 and port2 on the ATE.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) *ondatra.ATETopology {
	top := ate.Topology().New()

	p1 := ate.Port(t, "port1")
	i1 := top.AddInterface(atePort1.Name).WithPort(p1)
	i1.IPv4().
		WithAddress(atePort1.IPv4CIDR()).
		WithDefaultGateway(dutPort1.IPv4)

	p2 := ate.Port(t, "port2")
	i2 := top.AddInterface(atePort2.Name).WithPort(p2)
	i2.IPv4().
		WithAddress(atePort2.IPv4CIDR()).
		WithDefaultGateway(dutPort2.IPv4)

	return top
}

// createTrafficFlow generates traffic flow from source network to
// destination network via srcEndPoint to dstEndPoint and checks for
// packet loss.
func createTrafficFlow(t *testing.T, ate *ondatra.ATEDevice, top *ondatra.ATETopology, srcEndPoint, dstEndPoint *ondatra.Interface) *ondatra.Flow {
	ethHeader := ondatra.NewEthernetHeader()
	ipv4Header := ondatra.NewIPv4Header()
	ipv4Header.DstAddressRange().
		WithMin("203.0.113.0").
		WithMax("203.0.113.254").
		WithCount(250)

	flow := ate.Traffic().NewFlow("Flow").
		WithSrcEndpoints(srcEndPoint).
		WithDstEndpoints(dstEndPoint).
		WithHeaders(ethHeader, ipv4Header)

	return flow

}

// Function to send traffic
func sendTraffic(t *testing.T, ate *ondatra.ATEDevice, flow *ondatra.Flow) {
	t.Logf("Starting traffic")
	ate.Traffic().Start(t, flow)
}

// Function to verify traffic
func verifyTraffic(t *testing.T, ate *ondatra.ATEDevice, flow *ondatra.Flow) {
	flowPath := ate.Telemetry().Flow(flow.Name())
	if got := flowPath.LossPct().Get(t); got > 0 {
		t.Errorf("LossPct for flow %s got %g, want 0", flow.Name(), got)
	} else {
		t.Logf("Traffic flows fine from ATE-port1 to ATE-port2")
	}
}

// Function to stop traffic
func stopTraffic(t *testing.T, ate *ondatra.ATEDevice) {
	t.Logf("Stopping traffic")
	ate.Traffic().Stop(t)
}

// testArgs holds the objects needed by a test case.
type testArgs struct {
	ctx     context.Context
	clientA *gribi.Client
	dut     *ondatra.DUTDevice
	ate     *ondatra.ATEDevice
	top     *ondatra.ATETopology
}

// routeInstall configures a IPv4 entry through clientA. Ensure that the entry via ClientA
// is active through AFT Telemetry.
func routeInstall(ctx context.Context, t *testing.T, args *testArgs) {
	// Add an IPv4Entry for 203.0.113.0/24 pointing to ATE port-2 via gRIBI-A,
	// ensure that the entry is active through AFT telemetry
	t.Logf("Add an IPv4Entry for %s pointing to ATE port-2 via gRIBI-A", ateDstNetCIDR)
	args.clientA.AddNH(t, nhIndex, atePort2.IPv4, instance, fluent.InstalledInRIB)
	args.clientA.AddNHG(t, nhgIndex, map[uint64]uint64{nhIndex: 1}, instance, fluent.InstalledInRIB)
	args.clientA.AddIPv4(t, ateDstNetCIDR, nhgIndex, instance, "", fluent.InstalledInRIB)

	// Verify the entry for 203.0.113.0/24 is active through AFT Telemetry.
	ipv4Path := args.dut.Telemetry().NetworkInstance(instance).Afts().Ipv4Entry(ateDstNetCIDR)
	if got, want := ipv4Path.Prefix().Get(t), ateDstNetCIDR; got != want {
		t.Errorf("ipv4-entry/state/prefix got %s, want %s", got, want)
	} else {
		t.Logf("ipv4-entry entry found for %s before controller switchover..", got)
	}
}

// findSecondaryController finds out primary and secondary controllers
func findSecondaryController(t *testing.T, dut *ondatra.DUTDevice, controllers []string) (string, string) {
	var primary, secondary string
	for _, controller := range controllers {
		role := dut.Telemetry().Component(controller).RedundantRole().Get(t)
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
	primary := dut.Telemetry().Component(primaryAfterSwitch)
	if !primary.LastSwitchoverTime().Lookup(t).IsPresent() {
		t.Errorf("primary.LastSwitchoverTime().Lookup(t).IsPresent(): got false, want true")
	} else {
		t.Logf("Found primary.LastSwitchoverTime(): %v", primary.LastSwitchoverTime().Get(t))
	}

	if !primary.LastSwitchoverReason().Lookup(t).IsPresent() {
		t.Errorf("primary.LastSwitchoverReason().Lookup(t).IsPresent(): got false, want true")
	} else {
		lastSwitchoverReason := primary.LastSwitchoverReason().Get(t)
		t.Logf("Found lastSwitchoverReason.GetDetails(): %v", lastSwitchoverReason.GetDetails())
		t.Logf("Found lastSwitchoverReason.GetTrigger().String(): %v", lastSwitchoverReason.GetTrigger().String())
	}
	if primary.LastSwitchoverReason().Get(t).GetTrigger() != switchTrigger {
		t.Errorf("primary.GetLastSwitchoverReason().GetTrigger(): got %s, want USER_INITIATED.",
			primary.LastSwitchoverReason().Get(t).GetTrigger().String())
	}

	if !primary.LastRebootTime().Lookup(t).IsPresent() {
		t.Errorf("primary.LastRebootTime.().Lookup(t).IsPresent(): got false, want true")
	} else {
		lastrebootTime := primary.LastRebootTime().Get(t)
		t.Logf("Found lastRebootTime.GetDetails(): %v", lastrebootTime)
	}
	if !primary.LastRebootReason().Lookup(t).IsPresent() {
		t.Errorf("primary.LastRebootReason.().Lookup(t).IsPresent(): got false, want true")
	} else {
		lastrebootReason := primary.LastRebootReason().Get(t)
		t.Logf("Found lastRebootReason.GetDetails(): %v", lastrebootReason)
	}
}

func TestSupFailure(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ctx := context.Background()

	// Configure the DUT
	configureDUT(t, dut)

	// Configure the ATE
	ate := ondatra.ATE(t, "ate")
	top := configureATE(t, ate)
	top.Push(t).StartProtocols(t)

	// Configure the gRIBI client clientA
	clientA := gribi.Client{
		DUT:                  dut,
		FibACK:               false,
		Persistence:          true,
		InitialElectionIDLow: 10,
	}
	defer clientA.Close(t)
	if err := clientA.Start(t); err != nil {
		t.Fatalf("gRIBI Connection can not be established")
	}

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
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	dstEndPoint := args.top.Interfaces()[atePort2.Name]
	flow := createTrafficFlow(t, args.ate, args.top, srcEndPoint, dstEndPoint)
	sendTraffic(t, args.ate, flow)
	verifyTraffic(t, args.ate, flow)

	controllers := cmp.FindComponentsByType(t, dut, controlcardType)
	t.Logf("Found controller list: %v", controllers)
	// Only perform the switchover for the chassis with dual controllers.
	if len(controllers) != 2 {
		t.Skipf("Dual controllers required on %v: got %v, want 2", dut.Model(), len(controllers))
	}

	secondaryBeforeSwitch, primaryBeforeSwitch := findSecondaryController(t, dut, controllers)
	t.Logf("Detected Secondary: %v, Primary: %v", secondaryBeforeSwitch, primaryBeforeSwitch)

	switchoverReady := dut.Telemetry().Component(primaryBeforeSwitch).SwitchoverReady()
	switchoverReady.Await(t, 30*time.Minute, true)
	t.Logf("SwitchoverReady().Get(t): %v", switchoverReady.Get(t))
	if got, want := switchoverReady.Get(t), true; got != want {
		t.Errorf("switchoverReady.Get(t): got %v, want %v", got, want)
	}

	gnoiClient := dut.RawAPIs().GNOI().Default(t)
	switchoverRequest := &spb.SwitchControlProcessorRequest{
		ControlProcessor: &tpb.Path{
			Elem: []*tpb.PathElem{{Name: secondaryBeforeSwitch}},
		},
	}
	t.Logf("switchoverRequest: %v", switchoverRequest)
	switchoverResponse, err := gnoiClient.System().SwitchControlProcessor(context.Background(), switchoverRequest)
	if err != nil {
		t.Fatalf("Failed to perform control processor switchover with unexpected err: %v", err)
	}
	t.Logf("gnoiClient.System().SwitchControlProcessor() response: %v, err: %v", switchoverResponse, err)

	secondaryAfterSwitch, primaryAfterSwitch := findSecondaryController(t, dut, controllers)
	t.Logf("Found Secondary Controller after switchover: %v, Primary: %v", secondaryAfterSwitch, primaryAfterSwitch)

	startSwitchover := time.Now()
	t.Logf("Wait for new Primary controller to boot up by polling the telemetry output.")
	for {
		var currentTime string
		t.Logf("Time elapsed %.2f seconds since switchover started.", time.Since(startSwitchover).Seconds())
		time.Sleep(30 * time.Second)
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			currentTime = dut.Telemetry().System().CurrentDatetime().Get(t)
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

	validateTelemetry(t, dut, primaryAfterSwitch)
	// Assume Controller Switchover happened, ensure traffic flows without loss.
	// Verify the entry for 203.0.113.0/24 is active through AFT Telemetry.
	defer clientA.Close(t)
	if err := clientA.Start(t); err != nil {
		t.Fatalf("gRIBI Connection can not be established")
	}

	ipv4Path := args.dut.Telemetry().NetworkInstance(instance).Afts().Ipv4Entry(ateDstNetCIDR)
	if got, want := ipv4Path.Prefix().Get(t), ateDstNetCIDR; got != want {
		t.Errorf("ipv4-entry/state/prefix got %s, want %s", got, want)
	} else {
		t.Logf("ipv4-entry found for %s after controller switchover..", got)
	}

	verifyTraffic(t, args.ate, flow)
	stopTraffic(t, args.ate)
	top.StopProtocols(t)
	clientA.Close(t)
}
