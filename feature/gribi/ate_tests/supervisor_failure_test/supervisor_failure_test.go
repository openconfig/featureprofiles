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

// package supervisor_failure_test implements TE-8.2 of Popgate vendor testplan
package supervisor_failure_test

import (
	"context"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/gribigo/chk"
	"github.com/openconfig/gribigo/constants"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygot/ygot"

	spb "github.com/openconfig/gnoi/system"
	tpb "github.com/openconfig/gnoi/types"
	gpb "github.com/openconfig/gribi/v1/proto/service"
)

const (
	plen4               = 30
	maxSwitchoverTime   = 900
	awaitDuration       = 2 * time.Minute
	nhIndex             = 1
	nhgIndex            = 10
	chassisComponent    = telemetry.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CHASSIS
	controlcardType     = telemetry.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD
	primaryController   = telemetry.PlatformTypes_ComponentRedundantRole_PRIMARY
	secondaryController = telemetry.PlatformTypes_ComponentRedundantRole_SECONDARY
	vrfEntryCIDR        = "203.0.113.0/24"
	vrfEntryStartIP     = "203.0.113.0"
	vrfEntryEndIP       = "203.0.113.255"
)

var (
	ateSrc = attrs.Attributes{
		Name:    "ateSrc",
		IPv4:    "192.0.2.1",
		IPv4Len: plen4,
	}

	dutSrc = attrs.Attributes{
		Desc:    "DUT to ATE source",
		IPv4:    "192.0.2.2",
		IPv4Len: plen4,
	}

	dutDst = attrs.Attributes{
		Desc:    "DUT to ATE destination",
		IPv4:    "192.0.2.5",
		IPv4Len: plen4,
	}

	ateDst = attrs.Attributes{
		Name:    "dst",
		IPv4:    "192.0.2.6",
		IPv4Len: plen4,
	}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// configInterfaceDUT configures the DUT interface with the Addrs.
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
	s4a.PrefixLength = ygot.Uint8(a.IPv4Len)

	return i
}

// configureDUT configures port1 and port2 on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	d := dut.Config()

	p1 := dut.Port(t, "port1")
	i1 := &telemetry.Interface{Name: ygot.String(p1.Name())}
	d.Interface(p1.Name()).Replace(t, configInterfaceDUT(i1, &dutSrc))

	p2 := dut.Port(t, "port2")
	i2 := &telemetry.Interface{Name: ygot.String(p2.Name())}
	d.Interface(p2.Name()).Replace(t, configInterfaceDUT(i2, &dutDst))
}

// configureATE configures port1 and port2 on the ATE.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) *ondatra.ATETopology {
	t.Helper()
	top := ate.Topology().New()

	p1 := ate.Port(t, "port1")
	i1 := top.AddInterface(ateSrc.Name).WithPort(p1)
	i1.IPv4().
		WithAddress(ateSrc.IPv4CIDR()).
		WithDefaultGateway(dutSrc.IPv4)

	p2 := ate.Port(t, "port2")
	i2 := top.AddInterface(ateDst.Name).WithPort(p2)
	i2.IPv4().
		WithAddress(ateDst.IPv4CIDR()).
		WithDefaultGateway(dutDst.IPv4)

	return top
}

// configureGRIBIClient configures a new GRIBI client with PRESERVE and FIB_ACK.
func configureGRIBIClient(t *testing.T, dut *ondatra.DUTDevice) *fluent.GRIBIClient {
	t.Helper()
	gribic := dut.RawAPIs().GRIBI().Default(t)

	// Configure the gRIBI client.
	c := fluent.NewClient()
	c.Connection().
		WithStub(gribic).
		WithRedundancyMode(fluent.ElectedPrimaryClient).
		WithInitialElectionID(1 /* low */, 0 /* hi */).
		WithPersistence().
		WithFIBACK()

	return c
}

// awaitTimeout calls a fluent client Await, adding a timeout to the context.
func awaitTimeout(ctx context.Context, c *fluent.GRIBIClient, t testing.TB) error {
	t.Helper()
	subctx, cancel := context.WithTimeout(ctx, awaitDuration)
	defer cancel()
	return c.Await(subctx, t)
}

// injectGRIBIEntry injects an IP Prefix attached to a next-hop and next-hop-group
// in the default network instance using GRIBI Modify() and validates FIB_ACK.
func injectGRIBIEntry(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice) {
	c := configureGRIBIClient(t, dut)

	c.Start(ctx, t)
	defer c.Stop(t)
	c.StartSending(ctx, t)

	if err := awaitTimeout(ctx, c, t); err != nil {
		t.Fatalf("Await got error during session negotiation: %v.", err)
	}

	c.Modify().AddEntry(t,
		fluent.NextHopEntry().
			WithNetworkInstance(*deviations.DefaultNetworkInstance).
			WithIndex(nhIndex).
			WithIPAddress(ateDst.IPv4),
		fluent.NextHopGroupEntry().
			WithNetworkInstance(*deviations.DefaultNetworkInstance).
			WithID(nhgIndex).
			AddNextHop(nhIndex, 1),
		fluent.IPv4Entry().
			WithNetworkInstance(*deviations.DefaultNetworkInstance).
			WithPrefix(vrfEntryCIDR).
			WithNextHopGroup(nhgIndex),
	)
	if err := awaitTimeout(ctx, c, t); err != nil {
		t.Fatalf("Await got error for ModifyRequest: %v.", err)
	}

	res := c.Results(t)
	chk.HasResult(t, res,
		fluent.OperationResult().
			WithOperationID(1).
			WithOperationType(constants.Add).
			WithNextHopOperation(nhIndex).
			WithProgrammingResult(fluent.InstalledInFIB).
			AsResult(),
	)
	chk.HasResult(t, res,
		fluent.OperationResult().
			WithOperationID(2).
			WithOperationType(constants.Add).
			WithNextHopGroupOperation(nhgIndex).
			WithProgrammingResult(fluent.InstalledInFIB).
			AsResult(),
	)
	chk.HasResult(t, res,
		fluent.OperationResult().
			WithOperationID(3).
			WithOperationType(constants.Add).
			WithIPv4Operation(vrfEntryCIDR).
			WithProgrammingResult(fluent.InstalledInFIB).
			AsResult(),
	)
}

// validateGRIBIEntry validates the injected IP Prefix using a GRIBI Get().
func validateGRIBIEntry(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice) {
	gr, err := getGRIBIEntry(ctx, t, dut)
	if err != nil {
		t.Errorf("gRIBI Get request for IPv4 AFT failed: %v.", err)
	}
	for _, entry := range gr.GetEntry() {
		if entry.GetIpv4().GetPrefix() == vrfEntryCIDR {
			return
		}
	}
	t.Errorf("IPv4 Entry with prefix %s got: nil, want: non-nil.", vrfEntryCIDR)
}

func getGRIBIEntry(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice) (gr *gpb.GetResponse, err error) {
	t.Helper()
	getEntry := func() (*gpb.GetResponse, error) {
		c := configureGRIBIClient(t, dut)

		c.Start(ctx, t)
		defer c.Stop(t)
		c.StartSending(ctx, t)

		return c.Get().WithNetworkInstance(*deviations.DefaultNetworkInstance).
			WithAFT(fluent.IPv4).Send()
	}

	for i := 0; i < 3; i++ {
		gr, err = getEntry()
		if err != nil {
			t.Logf("gRIBI Get request (try %d) for IPv4 AFT failed: %v", i+1, err)
			time.Sleep(30 * time.Second)
		} else {
			return gr, nil
		}
	}
	return gr, err
}

// flushGRIBIEntry flushes the injected IP Prefix using a GRIBI Flush().
func flushGRIBIEntry(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice) {
	c := configureGRIBIClient(t, dut)

	c.Start(ctx, t)
	defer c.Stop(t)
	c.StartSending(ctx, t)

	_, err := c.Flush().
		WithElectionOverride().
		WithNetworkInstance(*deviations.DefaultNetworkInstance).
		Send()
	if err != nil {
		t.Errorf("Cannot flush: %v", err)
	}
}

// switchController triggers a SwitchControlProcessorRequest to switch to the secondary
// controller and then waits for it to turn up and then validate its telemetry.
func switchController(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, controllers []string) {
	primaryBeforeSwitch, secondaryBeforeSwitch := findControllerByRole(t, dut, controllers)
	if secondaryBeforeSwitch == "" || primaryBeforeSwitch == "" {
		t.Fatalf("Expected non-empty primary and secondary controllers, got primary: %v, secondary: %v.",
			primaryBeforeSwitch, secondaryBeforeSwitch)
	}

	t.Logf("Detected secondary: %v, primary: %v.", secondaryBeforeSwitch, primaryBeforeSwitch)

	t.Run("switchoverReady", func(t *testing.T) {
		switchoverReady(t, dut, primaryBeforeSwitch)
	})

	gnoiClient := dut.RawAPIs().GNOI().Default(t)

	switchoverRequest := &spb.SwitchControlProcessorRequest{
		ControlProcessor: &tpb.Path{
			Elem: []*tpb.PathElem{{Name: secondaryBeforeSwitch}},
		},
	}
	t.Logf("switchoverRequest: %v.", switchoverRequest)
	dut.Telemetry().Component(primaryBeforeSwitch).SwitchoverReady()
	startSwitchover, err := time.Parse(time.RFC3339, dut.Telemetry().System().CurrentDatetime().Get(t))
	if err != nil {
		t.Errorf("Unable to get dut.Telemetry().System().CurrentDatetime() falling back to local time.Now(): %v.", err)
		startSwitchover = time.Now()
	}
	switchoverResponse, err := gnoiClient.System().SwitchControlProcessor(context.Background(), switchoverRequest)
	if err != nil {
		t.Errorf("Failed to perform control processor switchover with unexpected err: %v.", err)
		return
	}
	t.Logf("gnoiClient.System().SwitchControlProcessor() response: %v.", switchoverResponse)

	want := secondaryBeforeSwitch
	if got := switchoverResponse.GetControlProcessor().GetElem()[0].GetName(); got != want {
		t.Errorf("switchoverResponse.GetControlProcessor().GetElem()[0].GetName(): got %v, want %v.", got, want)
		return
	}

	if !waitForSwitchover(t, dut, secondaryBeforeSwitch, primaryBeforeSwitch) {
		return
	}
	t.Run("validateControllerTelemetry", func(t *testing.T) {
		validateControllerTelemetry(t, dut, startSwitchover, secondaryBeforeSwitch)
	})
}

// switchoverReady waits one minute for the controller to have a switchover-ready status.
func switchoverReady(t *testing.T, dut *ondatra.DUTDevice, controller string) {
	t.Skip("TODO: switchover-ready status not currently supported. Remove skip when supported.")
	if res, ok := dut.Telemetry().Component(controller).SwitchoverReady().Watch(t, time.Minute,
		func(val *telemetry.QualifiedBool) bool {
			return val.IsPresent() && val.Val(t) == true
		}).Await(t); !ok {
		t.Logf("Switchover-Ready status for controller %s got: %t, want: true.", controller, res.Val(t))
	}
}

// testTraffic starts sending traffic from ATE:port1->ATE:port2, then switches control to
// secondary controller and stops traffic after the switch.
func testTraffic(t *testing.T, ate *ondatra.ATEDevice, top *ondatra.ATETopology, switchFunc func(t *testing.T)) {
	i1 := top.Interfaces()[ateSrc.Name]
	i2 := top.Interfaces()[ateDst.Name]

	ethHeader := ondatra.NewEthernetHeader()
	ipv4Header := ondatra.NewIPv4Header()

	ipv4Header.DstAddressRange().
		WithMin(vrfEntryStartIP).
		WithMax(vrfEntryEndIP).
		WithCount(256)

	flow := ate.Traffic().NewFlow("Flow").
		WithSrcEndpoints(i1).
		WithDstEndpoints(i2).
		WithHeaders(ethHeader, ipv4Header)

	ate.Traffic().Start(t, flow)

	switchFunc(t)
	t.Log("Stopping traffic after controller switch.")

	ate.Traffic().Stop(t)

	flowPath := ate.Telemetry().Flow(flow.Name())
	if got := flowPath.LossPct().Get(t); got > 0 {
		t.Errorf("LossPct for flow %s got %g, want 0.", flow.Name(), got)
	}
}

// waitForSwitchover waits for secondary controller to become primary and
// verifies telemetry after switch.
func waitForSwitchover(t *testing.T, dut *ondatra.DUTDevice, oldSecondary, oldPrimary string) bool {
	startSwitchover := time.Now()
	controllers := []string{oldSecondary, oldPrimary}

	t.Logf("Wait for new primary controller to boot up by polling the telemetry output.")
	for {
		var currentTime string
		t.Logf("Time elapsed %.2f seconds since switchover started.", time.Since(startSwitchover).Seconds())
		time.Sleep(30 * time.Second)
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			for _, c := range controllers {
				_ = dut.Telemetry().Component(c).RedundantRole().Get(t)
			}
			currentTime = dut.Telemetry().System().CurrentDatetime().Get(t)
		}); errMsg != nil {
			t.Logf("Got testt.CaptureFatal errMsg: %s, keep polling ...", *errMsg)
		} else {
			t.Logf("RP switchover has completed successfully with received time: %v.", currentTime)
			break
		}
		if uint64(time.Since(startSwitchover).Seconds()) > maxSwitchoverTime {
			t.Errorf("time.Since(startSwitchover): got %v, want < %v.", time.Since(startSwitchover), maxSwitchoverTime)
			return false
		}
	}
	t.Logf("RP switchover time: %.2f seconds.", time.Since(startSwitchover).Seconds())

	primaryAfterSwitch, secondaryAfterSwitch := findControllerByRole(t, dut, controllers)
	t.Logf("Found secondary after switchover: %v, primary: %v.", secondaryAfterSwitch, primaryAfterSwitch)

	if primaryAfterSwitch != oldSecondary {
		t.Errorf("Get primaryAfterSwitch: got %v, want %v.", primaryAfterSwitch, oldSecondary)
		return false
	}
	if secondaryAfterSwitch != oldPrimary {
		t.Errorf("Get secondaryAfterSwitch: got %v, want %v.", secondaryAfterSwitch, oldPrimary)
		return false
	}
	return true
}

// validateControllerTelemetry validates the telemetry data for the primaryController.
// It validates the reboot time, switchover time and the switchover reason.
func validateControllerTelemetry(t *testing.T, dut *ondatra.DUTDevice, startSwitchover time.Time, primaryController string) {
	t.Log("Validate CHASSIS & CONTROLLER_CARD reboot time/reason telemetry.")

	allChassis := components.FindComponentsByType(t, dut, chassisComponent)
	if len(allChassis) < 1 {
		t.Error("telemetry.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CHASSIS not found.")
	}
	chassis := dut.Telemetry().Component(allChassis[0]).Get(t)
	primary := dut.Telemetry().Component(primaryController).Get(t)

	if primary.GetLastSwitchoverTime() == 0 {
		t.Error("primary.GetLastSwitchoverTime(): got UNSET, want SET.")
	}
	if time.Unix(0, int64(primary.GetLastSwitchoverTime())).Before(startSwitchover) {
		t.Errorf("primary.GetLastSwitchoverTime(): got %v, want After startSwitchover: %v.",
			primary.GetLastSwitchoverTime(), uint64(startSwitchover.Unix()))
	}
	if primary.GetLastRebootTime() == 0 {
		t.Error("primary.GetLastRebootTime(): got UNSET, want SET.")
	}
	if time.Unix(0, int64(primary.GetLastRebootTime())).After(startSwitchover) {
		t.Errorf("primary.GetLastRebootTime(): got %v, want Before startSwitchover: %v.",
			primary.GetLastRebootTime(), uint64(startSwitchover.Unix()))
	}

	if primary.GetLastSwitchoverReason() == nil {
		t.Error("primary.GetLastSwitchoverReason(): got nil, want non-nil.")
	}
	if primary.GetLastSwitchoverReason().GetDetails() == "" {
		t.Error("primary.GetLastSwitchoverReason().GetDetails(): got nil, want non-nil.")
	}
	if primary.GetLastSwitchoverReason().GetTrigger() != telemetry.PlatformTypes_ComponentRedundantRoleSwitchoverReasonTrigger_SYSTEM_INITIATED {
		t.Errorf("primary.GetLastSwitchoverReason().GetTrigger(): got %s, want SYSTEM_INITIATED.",
			primary.GetLastSwitchoverReason().GetTrigger().String())
	}
	if primary.GetLastRebootReason() == telemetry.PlatformTypes_COMPONENT_REBOOT_REASON_UNSET {
		t.Error("primary.GetLastRebootReason(): got UNSET, want SET.")
	}

	if chassis.GetLastRebootTime() == 0 {
		t.Error("chassis.GetLastRebootTime(): got UNSET, want SET.")
	}
	if time.Unix(0, int64(chassis.GetLastRebootTime())).After(startSwitchover) {
		t.Errorf("chassis.GetLastRebootTime(): got %v, want Before startSwitchover: %v.",
			chassis.GetLastRebootTime(), uint64(startSwitchover.Unix()))
	}
	if chassis.GetLastRebootReason() == telemetry.PlatformTypes_COMPONENT_REBOOT_REASON_UNSET {
		t.Error("chassis.GetLastRebootReason(): got UNSET, want SET.")
	}
}

// findControllerByRole returns both the primary and the secondary controller cards of the device
// using telemetry.
func findControllerByRole(t *testing.T, dut *ondatra.DUTDevice, controllers []string) (primary string, secondary string) {
	for _, controller := range controllers {
		role := dut.Telemetry().Component(controller).RedundantRole().Get(t)
		t.Logf("Component(controller).RedundantRole().Get(t): %v, Role: %v.", controller, role)
		if role == secondaryController {
			secondary = controller
		} else if role == primaryController {
			primary = controller
		} else {
			t.Fatalf("Expected controller %s to be primary or secondary, got %v.", controller, role)
		}
	}
	return primary, secondary
}

func TestControllerFailure(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	ctx := context.Background()

	controllers := components.FindComponentsByType(t, dut, controlcardType)
	t.Logf("Found controller list: %v", controllers)
	// Only perform the switchover for the chassis with dual RPs/Controllers.
	if len(controllers) < 2 {
		t.Skipf("Dual controller card is required on %v: got %v, want 2.", dut.Model(), len(controllers))
	}

	// Configure the DUT.
	configureDUT(t, dut)

	// Configure the ATE.
	top := configureATE(t, ate)
	top.Push(t).StartProtocols(t)

	// Install gRIBI Routes.
	t.Run("injectGRIBIEntry", func(t *testing.T) {
		injectGRIBIEntry(ctx, t, dut)
	})

	// Check Traffic is forwarded between ATE ports during controller switchover.
	t.Run("testTraffic", func(t *testing.T) {
		testTraffic(t, ate, top, func(t *testing.T) {
			t.Run("switchController", func(t *testing.T) {
				switchController(ctx, t, dut, controllers)
			})
		})
	})

	// Validate gRIBI Entry is present after controller switchover.
	t.Run("validateGRIBIEntry", func(t *testing.T) {
		validateGRIBIEntry(ctx, t, dut)
	})

	// Flush installed GRIBI routes.
	flushGRIBIEntry(ctx, t, dut)

	// Stop ATE protocols.
	top.StopProtocols(t)
}
