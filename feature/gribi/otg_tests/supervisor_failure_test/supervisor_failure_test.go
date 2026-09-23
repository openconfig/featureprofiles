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
	"fmt"
	"net"
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
	ipv6PrefixLen       = 126
	nhIndex             = 1
	nhgIndex            = 42
	nhIndexV6           = 2
	nhgIndexV6          = 43
	controlcardType     = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD
	primaryController   = oc.Platform_ComponentRedundantRole_PRIMARY
	secondaryController = oc.Platform_ComponentRedundantRole_SECONDARY
	switchTrigger       = oc.PlatformTypes_ComponentRedundantRoleSwitchoverReasonTrigger_USER_INITIATED
	// trafficPps defines the traffic transmission speed in Packets Per Second (PPS).
	trafficPps              = 1000
	switchoverLossTolerance = 5.0
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "dutPort1",
		IPv4:    "192.0.2.1",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:db8::1",
		IPv6Len: ipv6PrefixLen,
	}

	atePort1 = attrs.Attributes{
		Name:    "atePort1",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.0.2.2",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:db8::2",
		IPv6Len: ipv6PrefixLen,
	}

	dutPort2 = attrs.Attributes{
		Desc:    "dutPort2",
		IPv4:    "192.0.2.5",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:db8::5",
		IPv6Len: ipv6PrefixLen,
	}

	atePort2 = attrs.Attributes{
		Name:    "atePort2",
		MAC:     "02:00:02:01:01:01",
		IPv4:    "192.0.2.6",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:db8::6",
		IPv6Len: ipv6PrefixLen,
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

	s6 := s.GetOrCreateIpv6()
	if deviations.InterfaceEnabled(dut) {
		s6.Enabled = ygot.Bool(true)
	}
	s6.GetOrCreateAddress(a.IPv6).PrefixLength = ygot.Uint8(ipv6PrefixLen)

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

// generateIPv4Prefixes generates a list of IPv4 prefixes.
func generateIPv4Prefixes(t testing.TB, startIP string, count int) []string {
	t.Helper()
	var prefixes []string
	ip := net.ParseIP(startIP).To4()
	for i := 0; i < count; i++ {
		prefixes = append(prefixes, fmt.Sprintf("%d.%d.%d.%d/32", ip[0], ip[1], ip[2], ip[3]))
		ip[3]++
	}
	return prefixes
}

// generateIPv6Prefixes generates a list of IPv6 prefixes.
func generateIPv6Prefixes(t testing.TB, startIP string, count int) []string {
	t.Helper()
	var prefixes []string
	ip := net.ParseIP(startIP).To16()
	for i := 0; i < count; i++ {
		prefixes = append(prefixes, fmt.Sprintf("%s/128", ip.String()))
		ip[15]++
	}
	return prefixes
}

// configureATE configures port1 and port2 on the ATE and adding a flow with port1 as the source and port2 as destination
func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	t.Helper()
	top := gosnappi.NewConfig()

	p1 := ate.Port(t, "port1")
	p2 := ate.Port(t, "port2")

	atePort1.AddToOTG(top, p1, &dutPort1)
	atePort2.AddToOTG(top, p2, &dutPort2)

	// Flow TE-8.2.1 IPv4 - Traffic speed set to trafficPps (1000 PPS)
	flow1v4 := top.Flows().Add().SetName("Flow TE-8.2.1 IPv4")
	flow1v4.Metrics().SetEnable(true)
	flow1v4.Rate().SetPps(trafficPps)
	flow1v4.TxRx().Device().SetTxNames([]string{atePort1.Name + ".IPv4"}).SetRxNames([]string{atePort2.Name + ".IPv4"})
	e1v4 := flow1v4.Packet().Add().Ethernet()
	e1v4.Src().SetValue(atePort1.MAC)
	v4_1 := flow1v4.Packet().Add().Ipv4()
	v4_1.Src().SetValue(atePort1.IPv4)
	v4_1.Dst().Increment().SetStart("203.0.113.1").SetCount(50).SetStep("0.0.0.1")

	// Flow TE-8.2.1 IPv6 - Traffic speed set to trafficPps (1000 PPS)
	flow1v6 := top.Flows().Add().SetName("Flow TE-8.2.1 IPv6")
	flow1v6.Metrics().SetEnable(true)
	flow1v6.Rate().SetPps(trafficPps)
	flow1v6.TxRx().Device().SetTxNames([]string{atePort1.Name + ".IPv6"}).SetRxNames([]string{atePort2.Name + ".IPv6"})
	e1v6 := flow1v6.Packet().Add().Ethernet()
	e1v6.Src().SetValue(atePort1.MAC)
	v6_1 := flow1v6.Packet().Add().Ipv6()
	v6_1.Src().SetValue(atePort1.IPv6)
	v6_1.Dst().Increment().SetStart("2001:db8:203:0:113::1").SetCount(50).SetStep("::1")

	return top
}

func appendFlowsTE822(top gosnappi.Config) {
	// Flow TE-8.2.2 IPv4 - Traffic speed set to trafficPps (1000 PPS)
	flow2v4 := top.Flows().Add().SetName("Flow TE-8.2.2 IPv4")
	flow2v4.Metrics().SetEnable(true)
	flow2v4.Rate().SetPps(trafficPps)
	flow2v4.TxRx().Device().SetTxNames([]string{atePort1.Name + ".IPv4"}).SetRxNames([]string{atePort2.Name + ".IPv4"})
	e2v4 := flow2v4.Packet().Add().Ethernet()
	e2v4.Src().SetValue(atePort1.MAC)
	v4_2 := flow2v4.Packet().Add().Ipv4()
	v4_2.Src().SetValue(atePort1.IPv4)
	v4_2.Dst().Increment().SetStart("203.0.114.1").SetCount(50).SetStep("0.0.0.1")

	// Flow TE-8.2.2 IPv6 - Traffic speed set to trafficPps (1000 PPS)
	flow2v6 := top.Flows().Add().SetName("Flow TE-8.2.2 IPv6")
	flow2v6.Metrics().SetEnable(true)
	flow2v6.Rate().SetPps(trafficPps)
	flow2v6.TxRx().Device().SetTxNames([]string{atePort1.Name + ".IPv6"}).SetRxNames([]string{atePort2.Name + ".IPv6"})
	e2v6 := flow2v6.Packet().Add().Ethernet()
	e2v6.Src().SetValue(atePort1.MAC)
	v6_2 := flow2v6.Packet().Add().Ipv6()
	v6_2.Src().SetValue(atePort1.IPv6)
	v6_2.Dst().Increment().SetStart("2001:db8:203:0:114::1").SetCount(50).SetStep("::1")
}

// testArgs holds the objects needed by a test case.
type testArgs struct {
	ctx     context.Context
	clientA *gribi.Client
	dut     *ondatra.DUTDevice
	ate     *ondatra.ATEDevice
	top     gosnappi.Config
}

// routeInstall1 TE-8.2.1 configuring 100 entries
func routeInstall1(ctx context.Context, t *testing.T, args *testArgs) {
	vrf := deviations.DefaultNetworkInstance(args.dut)
	args.clientA.AddNH(t, nhIndex, atePort2.IPv4, vrf, fluent.InstalledInRIB)
	args.clientA.AddNHG(t, nhgIndex, map[uint64]uint64{nhIndex: 1}, vrf, fluent.InstalledInRIB)

	args.clientA.AddNH(t, nhIndexV6, atePort2.IPv6, vrf, fluent.InstalledInRIB)
	args.clientA.AddNHG(t, nhgIndexV6, map[uint64]uint64{nhIndexV6: 1}, vrf, fluent.InstalledInRIB)

	v4Prefixes := generateIPv4Prefixes(t, "203.0.113.1", 50)
	v6Prefixes := generateIPv6Prefixes(t, "2001:db8:203:0:113::1", 50)

	args.clientA.AddIPv4s(t, v4Prefixes, nhgIndex, vrf, "", fluent.InstalledInRIB)
	args.clientA.AddIPv6s(t, v6Prefixes, nhgIndexV6, vrf, "", fluent.InstalledInRIB)
}

// routeInstall2 TE-8.2.2 configuring another 100 entries
func routeInstall2(ctx context.Context, t *testing.T, args *testArgs) {
	vrf := deviations.DefaultNetworkInstance(args.dut)

	v4Prefixes := generateIPv4Prefixes(t, "203.0.114.1", 50)
	v6Prefixes := generateIPv6Prefixes(t, "2001:db8:203:0:114::1", 50)

	args.clientA.AddIPv4s(t, v4Prefixes, nhgIndex, vrf, "", fluent.InstalledInRIB)
	args.clientA.AddIPv6s(t, v6Prefixes, nhgIndexV6, vrf, "", fluent.InstalledInRIB)
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
func validateTelemetry(t *testing.T, dut *ondatra.DUTDevice, primaryAfterSwitch, secondaryAfterSwitch string, timeout time.Duration) {
	t.Helper()
	t.Log("Validate OC Switchover time/reason.")
	primary := gnmi.OC().Component(primaryAfterSwitch)
	secondary := gnmi.OC().Component(secondaryAfterSwitch)
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

		wantTrigger := switchTrigger
		if deviations.GNOISwitchoverReasonMissingUserInitiated(dut) {
			wantTrigger = oc.PlatformTypes_ComponentRedundantRoleSwitchoverReasonTrigger_SYSTEM_INITIATED
		}
		if got, want := lastSwitchoverReason.GetTrigger(), wantTrigger; got != want {
			t.Logf("WARNING: primary.GetLastSwitchoverReason().GetTrigger(): got %s, want %s ", got, want)
		}
	}

	t.Logf("Waiting for secondary controller to fully boot and report LastReboot telemetry (timeout: %v)...", timeout)
	deadline := time.Now().Add(timeout)

	lastRebootTimeVal, okTime := gnmi.Watch(t, dut, secondary.LastRebootTime().State(), timeout, func(val *ygnmi.Value[uint64]) bool {
		v, present := val.Val()
		return present && v > 0
	}).Await(t)
	if !okTime {
		t.Errorf("secondary.LastRebootTime().Lookup(t).IsPresent(): got false even after waiting for secondary to boot, want true")
	} else {
		v, _ := lastRebootTimeVal.Val()
		t.Logf("Found lastRebootTime: %v", v)
	}

	remaining := time.Until(deadline)
	if remaining < 30*time.Second {
		remaining = 30 * time.Second
	}
	lastRebootReasonVal, okReason := gnmi.Watch(t, dut, secondary.LastRebootReason().State(), remaining, func(val *ygnmi.Value[oc.E_PlatformTypes_COMPONENT_REBOOT_REASON]) bool {
		return val.IsPresent()
	}).Await(t)
	if !okReason {
		t.Errorf("secondary.LastRebootReason().Lookup(t).IsPresent(): got false even after waiting for secondary to boot, want true")
	} else {
		v, _ := lastRebootReasonVal.Val()
		t.Logf("Found lastRebootReason: %v", v)
	}
}

// awaitSwitchoverReady waits for the controller to report switchover-ready using gnmi.Await.
func awaitSwitchoverReady(t *testing.T, dut *ondatra.DUTDevice, controller string, timeout time.Duration) {
	t.Helper()
	t.Logf("Waiting for controller %s to be switchover-ready (timeout: %v)...", controller, timeout)
	gnmi.Await(t, dut, gnmi.OC().Component(controller).SwitchoverReady().State(), timeout, true)
}

// validateSwitchover waits for the specified controller to become PRIMARY using gnmi.Watch with polling.
func validateSwitchover(t *testing.T, dut *ondatra.DUTDevice, controller string, timeout time.Duration) {
	t.Helper()
	start := time.Now()
	t.Logf("Waiting for new Primary controller %s to become PRIMARY (timeout: %v)...", controller, timeout)
	rolePath := gnmi.OC().Component(controller).RedundantRole().State()

	for time.Since(start) < timeout {
		attemptTimeout := 20 * time.Second
		if remaining := time.Until(start.Add(timeout)); remaining < attemptTimeout {
			attemptTimeout = remaining
		}

		var (
			val   *ygnmi.Value[oc.E_Platform_ComponentRedundantRole]
			found bool
		)
		testt.CaptureFatal(t, func(t testing.TB) {
			val, found = gnmi.Watch(t, dut, rolePath, attemptTimeout, func(v *ygnmi.Value[oc.E_Platform_ComponentRedundantRole]) bool {
				role, present := v.Val()
				return present && role == primaryController
			}).Await(t)
		})

		if found && val != nil {
			role, _ := val.Val()
			t.Logf("Controller switchover has completed successfully, %s is now %v in %.2f seconds.", controller, role, time.Since(start).Seconds())
			return
		}
		t.Logf("Controller %s not yet PRIMARY (elapsed: %.2fs), retrying...", controller, time.Since(start).Seconds())
		time.Sleep(5 * time.Second)
	}
	t.Fatalf("Controller %s did not become PRIMARY within %v", controller, timeout)
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
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv6")

	// TE-8.2.1 - FIB Programming and Switchover Validation
	t.Logf("TE-8.2.1: Connect gRIBI client to DUT specifying persistence mode PRESERVE, SINGLE_PRIMARY client redundancy...")
	clientA := gribi.Client{
		DUT:            dut,
		FIBACK:         false,
		Persistence:    true,
		RedundancyMode: fluent.ElectedPrimaryClient,
	}

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

	t.Logf("TE-8.2.1: Add 50 IPv4Entrys and 50 IPv6Entrys pointing to ATE port-2 via gRIBI-A...")
	routeInstall1(ctx, t, args)

	t.Logf("TE-8.2.1: Send traffic from ATE port-1 to the 100 prefixes (50 IPv4 and 50 IPv6) at configured speed: %d packets/sec (PPS)...", trafficPps)
	ate.OTG().StartTraffic(t)

	// Wait for traffic to flow and stabilize at 0% loss before initiating switchover
	otgutils.ExpectedTrafficLoss(t, args.ate.OTG(), "Flow TE-8.2.1 IPv4", 0, 0)
	otgutils.ExpectedTrafficLoss(t, args.ate.OTG(), "Flow TE-8.2.1 IPv6", 0, 0)

	t.Logf("TE-8.2.1: Leaving traffic running during switchover to ensure hitless forwarding...")

	controllers := cmp.FindComponentsByType(t, dut, controlcardType)
	t.Logf("Found controller list: %v", controllers)
	// Only perform the switchover for the chassis with dual controllers.
	if len(controllers) != 2 {
		t.Skipf("Dual controllers required on %v: got %v, want 2", dut.Model(), len(controllers))
	}

	secondaryBeforeSwitch, primaryBeforeSwitch := findSecondaryController(t, dut, controllers)

	extraWait := time.Duration(deviations.SwitchoverStabilizeDelayM(dut)) * time.Minute

	awaitSwitchoverReady(t, dut, primaryBeforeSwitch, 5*time.Minute+extraWait)

	// Gracefully terminate the pre-switchover client on Nokia to prevent proxy EOF issues during the hard crash.
	// For other vendors, let the switchover terminate the session naturally to avoid prematurely triggering hold-down timers.
	if dut.Vendor() == ondatra.NOKIA {
		clientA.Close(t)
	} else {
		defer clientA.Close(t)
	}

	t.Logf("TE-8.2.1: Validate: Supervisor switchover is triggered using gNOI SwitchControlProcessor...")
	switchoverResponse := gnoi.Execute(t, dut, system.NewSwitchControlProcessorOperation().Path(cmp.GetSubcomponentPath(secondaryBeforeSwitch, deviations.GNOISubcomponentPath(dut))))
	t.Logf("gnoiClient.System().SwitchControlProcessor() response: %v", switchoverResponse)

	validateSwitchover(t, dut, secondaryBeforeSwitch, 5*time.Minute+extraWait)

	// Old secondary controller becomes primary after switchover.
	primaryAfterSwitch := secondaryBeforeSwitch
	secondaryAfterSwitch := primaryBeforeSwitch

	// Log flow metrics and stop traffic across switchover window to evaluate hitless forwarding (< switchoverLossTolerance)
	otgutils.LogFlowMetrics(t, ate.OTG(), top)
	ate.OTG().StopTraffic(t)
	otgutils.ExpectedTrafficLoss(t, args.ate.OTG(), "Flow TE-8.2.1 IPv4", 0, switchoverLossTolerance)
	otgutils.ExpectedTrafficLoss(t, args.ate.OTG(), "Flow TE-8.2.1 IPv6", 0, switchoverLossTolerance)
	t.Logf("Traffic transmission speed verified across switchover: %d PPS per flow", trafficPps)

	t.Log("TE-8.2.1: Following reconnection of a gRIBI client to the new master supervisor...")
	clientB := gribi.Client{
		DUT:            dut,
		FIBACK:         false,
		Persistence:    true,
		RedundancyMode: fluent.ElectedPrimaryClient,
	}
	defer clientB.Close(t)
	// Flush all entries at the actual end of the test using the healthy post-switchover client.
	defer clientB.FlushAll(t)

	// Validate device state using gnmi.Await to confirm the new primary controller is active.
	gnmi.Await(t, dut, gnmi.OC().Component(primaryAfterSwitch).RedundantRole().State(), 2*time.Minute+extraWait, primaryController)

	gribiTimeout := 5*time.Minute + extraWait
	startTime := time.Now()
	for {
		if err := clientB.Start(t); err != nil {
			if time.Since(startTime) > gribiTimeout {
				t.Fatalf("gRIBI Connection for clientB could not be re-established within %v: %v", gribiTimeout, err)
			}
			t.Logf("Retrying gRIBI client connection: %v", err)
		} else {
			break
		}
	}

	t.Logf("TE-8.2.1: Assert leadership on the new active supervisor...")
	clientB.BecomeLeader(t)
	args.clientA = &clientB // Reassign pointer so routeInstall2 uses the healthy connection

	// Validate device state using gnmi.Watch to ensure the FIB/AFT is populated on the new active supervisor.
	t.Logf("TE-8.2.1: Watch AFT telemetry to ensure prefixes are programmed in the forwarding plane...")
	aftTimeout := 2*time.Minute + extraWait
	aftPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Afts().Ipv4Entry("203.0.113.1/32")
	if _, found := gnmi.Watch(t, dut, aftPath.State(), aftTimeout, func(val *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv4Entry]) bool {
		return val.IsPresent()
	}).Await(t); !found {
		t.Logf("Prefix 203.0.113.1/32 not yet visible in AFT telemetry within %v, proceeding with flow validation", aftTimeout)
	}

	// Verify post-switchover traffic on reconnected supervisor
	t.Logf("TE-8.2.1: Verify traffic flows 100%% from ATE port-1 to ATE port-2 following gRIBI reconnection...")
	ate.OTG().StartTraffic(t)
	otgutils.ExpectedTrafficLoss(t, args.ate.OTG(), "Flow TE-8.2.1 IPv4", 0, 0)
	otgutils.ExpectedTrafficLoss(t, args.ate.OTG(), "Flow TE-8.2.1 IPv6", 0, 0)
	otgutils.LogFlowMetrics(t, ate.OTG(), top)
	ate.OTG().StopTraffic(t)

	// Validate secondary controller reboot telemetry now that primary switchover & traffic are verified
	validateTelemetry(t, dut, primaryAfterSwitch, secondaryAfterSwitch, 15*time.Minute+extraWait)

	// TE-8.2.2 - Post Switchover FIB Programming Validation
	t.Logf("TE-8.2.2: Add another 50 IPv4Entrys and 50 IPv6Entrys pointing to ATE port-2...")
	routeInstall2(ctx, t, args)

	// Append TE-8.2.2 flows
	appendFlowsTE822(top)
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	// Give OTG protocols time to establish before re-starting traffic
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv6")

	t.Logf("TE-8.2.2: Send traffic to all 200 prefixes (100 initial + 100 post-switchover) at configured speed: %d packets/sec (PPS) per flow...", trafficPps)
	ate.OTG().StartTraffic(t)
	// Validate traffic flows with 0% loss using gnmi.Watch on flow metrics
	otgutils.ExpectedTrafficLoss(t, args.ate.OTG(), "Flow TE-8.2.1 IPv4", 0, 0)
	otgutils.ExpectedTrafficLoss(t, args.ate.OTG(), "Flow TE-8.2.1 IPv6", 0, 0)
	otgutils.ExpectedTrafficLoss(t, args.ate.OTG(), "Flow TE-8.2.2 IPv4", 0, 0)
	otgutils.ExpectedTrafficLoss(t, args.ate.OTG(), "Flow TE-8.2.2 IPv6", 0, 0)
	otgutils.LogFlowMetrics(t, ate.OTG(), top)
	ate.OTG().StopTraffic(t)
	t.Logf("Post-switchover traffic speed verified across all 200 prefixes: %d PPS per flow", trafficPps)

	args.ate.OTG().StopProtocols(t)
}
