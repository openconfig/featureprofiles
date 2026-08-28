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

	// Flow TE-8.2.1 IPv4
	flow1v4 := top.Flows().Add().SetName("Flow TE-8.2.1 IPv4")
	flow1v4.Metrics().SetEnable(true)
	flow1v4.TxRx().Device().SetTxNames([]string{atePort1.Name + ".IPv4"}).SetRxNames([]string{atePort2.Name + ".IPv4"})
	e1v4 := flow1v4.Packet().Add().Ethernet()
	e1v4.Src().SetValue(atePort1.MAC)
	v4_1 := flow1v4.Packet().Add().Ipv4()
	v4_1.Src().SetValue(atePort1.IPv4)
	v4_1.Dst().Increment().SetStart("203.0.113.1").SetCount(50).SetStep("0.0.0.1")

	// Flow TE-8.2.1 IPv6
	flow1v6 := top.Flows().Add().SetName("Flow TE-8.2.1 IPv6")
	flow1v6.Metrics().SetEnable(true)
	flow1v6.TxRx().Device().SetTxNames([]string{atePort1.Name + ".IPv4"}).SetRxNames([]string{atePort2.Name + ".IPv4"})
	e1v6 := flow1v6.Packet().Add().Ethernet()
	e1v6.Src().SetValue(atePort1.MAC)
	v6_1 := flow1v6.Packet().Add().Ipv6()
	v6_1.Src().SetValue("2001:db8::192:0:2:2")
	v6_1.Dst().Increment().SetStart("2001:db8:203:0:113::1").SetCount(50).SetStep("::1")

	return top
}

func appendFlowsTE822(top gosnappi.Config) {
	// Flow TE-8.2.2 IPv4
	flow2v4 := top.Flows().Add().SetName("Flow TE-8.2.2 IPv4")
	flow2v4.Metrics().SetEnable(true)
	flow2v4.TxRx().Device().SetTxNames([]string{atePort1.Name + ".IPv4"}).SetRxNames([]string{atePort2.Name + ".IPv4"})
	e2v4 := flow2v4.Packet().Add().Ethernet()
	e2v4.Src().SetValue(atePort1.MAC)
	v4_2 := flow2v4.Packet().Add().Ipv4()
	v4_2.Src().SetValue(atePort1.IPv4)
	v4_2.Dst().Increment().SetStart("203.0.114.1").SetCount(50).SetStep("0.0.0.1")

	// Flow TE-8.2.2 IPv6
	flow2v6 := top.Flows().Add().SetName("Flow TE-8.2.2 IPv6")
	flow2v6.Metrics().SetEnable(true)
	flow2v6.TxRx().Device().SetTxNames([]string{atePort1.Name + ".IPv4"}).SetRxNames([]string{atePort2.Name + ".IPv4"})
	e2v6 := flow2v6.Packet().Add().Ethernet()
	e2v6.Src().SetValue(atePort1.MAC)
	v6_2 := flow2v6.Packet().Add().Ipv6()
	v6_2.Src().SetValue("2001:db8::192:0:2:2")
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
	args.clientA.AddNH(t, nhIndex, atePort2.IPv4, vrf, fluent.InstalledInFIB)
	args.clientA.AddNHG(t, nhgIndex, map[uint64]uint64{nhIndex: 1}, vrf, fluent.InstalledInFIB)

	v4Prefixes := generateIPv4Prefixes(t, "203.0.113.1", 50)
	v6Prefixes := generateIPv6Prefixes(t, "2001:db8:203:0:113::1", 50)

	args.clientA.AddIPv4s(t, v4Prefixes, nhgIndex, vrf, "", fluent.InstalledInFIB)
	args.clientA.AddIPv6s(t, v6Prefixes, nhgIndex, vrf, "", fluent.InstalledInFIB)
}

// routeInstall2 TE-8.2.2 configuring another 100 entries
func routeInstall2(ctx context.Context, t *testing.T, args *testArgs) {
	vrf := deviations.DefaultNetworkInstance(args.dut)

	v4Prefixes := generateIPv4Prefixes(t, "203.0.114.1", 50)
	v6Prefixes := generateIPv6Prefixes(t, "2001:db8:203:0:114::1", 50)

	args.clientA.AddIPv4s(t, v4Prefixes, nhgIndex, vrf, "", fluent.InstalledInFIB)
	args.clientA.AddIPv6s(t, v6Prefixes, nhgIndex, vrf, "", fluent.InstalledInFIB)
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
func validateTelemetry(t *testing.T, dut *ondatra.DUTDevice, primaryAfterSwitch, secondaryAfterSwitch string) {
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
	}
	wantTrigger := switchTrigger
	if deviations.GNOISwitchoverReasonMissingUserInitiated(dut) {
		wantTrigger = oc.PlatformTypes_ComponentRedundantRoleSwitchoverReasonTrigger_SYSTEM_INITIATED
	}
	if got, want := gnmi.Get(t, dut, primary.LastSwitchoverReason().State()).GetTrigger(), wantTrigger; got != want {
		t.Errorf("primary.GetLastSwitchoverReason().GetTrigger(): got %s, want %s.", got, want)
	}

	if !gnmi.Lookup(t, dut, secondary.LastRebootTime().State()).IsPresent() {
		t.Errorf("secondary.LastRebootTime.().Lookup(t).IsPresent(): got false, want true")
	} else {
		lastrebootTime := gnmi.Get(t, dut, secondary.LastRebootTime().State())
		t.Logf("Found lastRebootTime.GetDetails(): %v", lastrebootTime)
	}
	if !gnmi.Lookup(t, dut, secondary.LastRebootReason().State()).IsPresent() {
		t.Errorf("secondary.LastRebootReason.().Lookup(t).IsPresent(): got false, want true")
	} else {
		lastrebootReason := gnmi.Get(t, dut, secondary.LastRebootReason().State())
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
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv6")

	// TE-8.2.1 - FIB Programming and Switchover Validation
	t.Logf("TE-8.2.1: Connect gRIBI client to DUT specifying persistence mode PRESERVE, SINGLE_PRIMARY client redundancy...")
	clientA := gribi.Client{
		DUT:            dut,
		FIBACK:         true,
		Persistence:    true,
		RedundancyMode: fluent.ElectedPrimaryClient,
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

	t.Logf("TE-8.2.1: Add 50 IPv4Entrys and 50 IPv6Entrys pointing to ATE port-2 via gRIBI-A...")
	routeInstall1(ctx, t, args)
	gribi.AwaitAFTIPv4Entries(t, dut, deviations.DefaultNetworkInstance(dut), generateIPv4Prefixes(t, "203.0.113.1", 50))
	gribi.AwaitAFTIPv6Entries(t, dut, deviations.DefaultNetworkInstance(dut), generateIPv6Prefixes(t, "2001:db8:203:0:113::1", 50))

	t.Logf("TE-8.2.1: Send traffic from ATE port-1 to the 100 prefixes (50 IPv4 and 50 IPv6)...")
	ate.OTG().StartTraffic(t)

	// Wait for traffic to flow and stabilize at 0% loss before initiating switchover
	otgutils.ExpectedTrafficLoss(t, args.ate.OTG(), "Flow TE-8.2.1 IPv4", 0, 0)
	otgutils.ExpectedTrafficLoss(t, args.ate.OTG(), "Flow TE-8.2.1 IPv6", 0, 0)

	t.Logf("TE-8.2.1: Stop traffic before switchover to reset metrics for post-switchover validation...")
	ate.OTG().StopTraffic(t)

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

	t.Logf("TE-8.2.1: Validate: Supervisor switchover is triggered using gNOI SwitchControlProcessor...")
	switchoverResponse := gnoi.Execute(t, dut, system.NewSwitchControlProcessorOperation().Path(cmp.GetSubcomponentPath(secondaryBeforeSwitch, deviations.GNOISubcomponentPath(dut))))
	t.Logf("gnoiClient.System().SwitchControlProcessor() response: %v", switchoverResponse)

	startSwitchover := time.Now()
	t.Logf("Wait for new Primary controller to boot up by polling the telemetry output.")
	_, ok := gnmi.Watch(t, dut, gnmi.OC().System().CurrentDatetime().State(), time.Duration(maxSwitchoverTime)*time.Second, func(val *ygnmi.Value[string]) bool {
		currentTime, present := val.Val()
		if present {
			t.Logf("Controller switchover has completed successfully with received time: %v", currentTime)
		}
		return present
	}).Await(t)
	if !ok {
		t.Fatalf("Controller switchover did not complete successfully within %.2f seconds", float64(maxSwitchoverTime))
	}
	t.Logf("Controller switchover time: %.2f seconds", time.Since(startSwitchover).Seconds())

	// Old secondary controller becomes primary after switchover.
	primaryAfterSwitch := secondaryBeforeSwitch
	secondaryAfterSwitch := primaryBeforeSwitch
	validateTelemetry(t, dut, primaryAfterSwitch, secondaryAfterSwitch)

	t.Log("TE-8.2.1: Following reconnection of a gRIBI client to the new master supervisor...")
	retryDuration := 320 * time.Second
	retryInterval := 5 * time.Second
	startTime := time.Now()
	for {
		if err := clientA.Start(t); err != nil {
			if time.Since(startTime) > retryDuration {
				t.Fatalf("gRIBI Connection for clientA could not be re-established after multiple attempts")
			}
			t.Logf("Retrying gRIBI client connection in %v...", retryInterval)
			time.Sleep(retryInterval)
		} else {
			break
		}
	}

	t.Logf("TE-8.2.1: Assert leadership on the new active supervisor...")
	clientA.BecomeLeader(t)

	t.Logf("TE-8.2.1: Restarting traffic for post-switchover evaluation...")
	ate.OTG().StartTraffic(t)

	// Wait for default network instance AFT to be populated.
	t.Logf("TE-8.2.1: ...ensure the 100 prefixes pointing to ATE port-2 are present and traffic flows 100%%...")
	gribi.AwaitAFTIPv4Entries(t, dut, deviations.DefaultNetworkInstance(dut), generateIPv4Prefixes(t, "203.0.113.1", 50))
	gribi.AwaitAFTIPv6Entries(t, dut, deviations.DefaultNetworkInstance(dut), generateIPv6Prefixes(t, "2001:db8:203:0:113::1", 50))

	// Validate traffic flowed without loss (min 0 loss, max 0 loss or slightly higher but essentially 0%)
	otgutils.LogFlowMetrics(t, ate.OTG(), top)
	otgutils.ExpectedTrafficLoss(t, args.ate.OTG(), "Flow TE-8.2.1 IPv4", 0, 0)
	otgutils.ExpectedTrafficLoss(t, args.ate.OTG(), "Flow TE-8.2.1 IPv6", 0, 0)
	ate.OTG().StopTraffic(t)

	// TE-8.2.2 - Post Switchover FIB Programming Validation
	t.Logf("TE-8.2.2: Add another 50 IPv4Entrys and 50 IPv6Entrys pointing to ATE port-2...")
	routeInstall2(ctx, t, args)
	gribi.AwaitAFTIPv4Entries(t, dut, deviations.DefaultNetworkInstance(dut), generateIPv4Prefixes(t, "203.0.114.1", 50))
	gribi.AwaitAFTIPv6Entries(t, dut, deviations.DefaultNetworkInstance(dut), generateIPv6Prefixes(t, "2001:db8:203:0:114::1", 50))

	// Append TE-8.2.2 flows
	appendFlowsTE822(top)
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	// Give OTG protocols time to establish before re-starting traffic
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv6")

	t.Logf("TE-8.2.2: Send traffic to all 200 prefixes (100 initial + 100 post-switchover) and ensure traffic flows 100%%...")
	ate.OTG().StartTraffic(t)

	// Delegate waiting logic to ExpectedTrafficLoss
	otgutils.LogFlowMetrics(t, ate.OTG(), top)
	otgutils.ExpectedTrafficLoss(t, args.ate.OTG(), "Flow TE-8.2.1 IPv4", 0, 0)
	otgutils.ExpectedTrafficLoss(t, args.ate.OTG(), "Flow TE-8.2.1 IPv6", 0, 0)
	otgutils.ExpectedTrafficLoss(t, args.ate.OTG(), "Flow TE-8.2.2 IPv4", 0, 0)
	otgutils.ExpectedTrafficLoss(t, args.ate.OTG(), "Flow TE-8.2.2 IPv6", 0, 0)

	ate.OTG().StopTraffic(t)
	args.ate.OTG().StopProtocols(t)
}
