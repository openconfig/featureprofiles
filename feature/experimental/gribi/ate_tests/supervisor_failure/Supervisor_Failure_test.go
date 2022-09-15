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
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/telemetry"
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
	ipv4PrefixLen     = 30
	instance          = "DEFAULT"
	ateDstNetCIDR     = "203.0.113.0/24"
	staticNH          = "192.0.2.6"
	nhIndex           = 1
	nhgIndex          = 42
	controlcardType   = telemetry.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD
	activeController  = telemetry.PlatformTypes_ComponentRedundantRole_PRIMARY
	standbyController = telemetry.PlatformTypes_ComponentRedundantRole_SECONDARY
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

// testTraffic generates traffic flow from source network to
// destination network via srcEndPoint to dstEndPoint and checks for
// packet loss.
func testTraffic(t *testing.T, ate *ondatra.ATEDevice, top *ondatra.ATETopology, srcEndPoint, dstEndPoint *ondatra.Interface) {
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

	ate.Traffic().Start(t, flow)
	time.Sleep(15 * time.Second)
	ate.Traffic().Stop(t)

	flowPath := ate.Telemetry().Flow(flow.Name())
	if got := flowPath.LossPct().Get(t); got > 0 {
		t.Errorf("LossPct for flow %s got %g, want 0", flow.Name(), got)
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
	}
	// Verify that static route(203.0.113.0/24) to ATE port-2 is preferred by the traffic.`
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	dstEndPoint := args.top.Interfaces()[atePort2.Name]
	testTraffic(t, args.ate, args.top, srcEndPoint, dstEndPoint)

}

// findComponentsByType finds supervisor (CONTROLLER_CARD) for switchover
func findComponentsByType(t *testing.T, dut *ondatra.DUTDevice, cType telemetry.E_PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT) []string {
	components := dut.Telemetry().ComponentAny().Name().Get(t)
	var s []string
	for _, c := range components {
		lookupType := dut.Telemetry().Component(c).Type().Lookup(t)
		if !lookupType.IsPresent() {
			t.Logf("Component %s type is not found", c)
		} else {
			componentType := lookupType.Val(t)
			t.Logf("Component %s has type: %v", c, componentType)

			switch v := componentType.(type) {
			case telemetry.E_PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT:
				if v == cType {
					s = append(s, c)
				}
			default:
				t.Fatalf("Expected component type to be a hardware component, got (%T, %v)", componentType, componentType)
			}
		}
	}
	return s
}

// findStandbyRP finds out ActiveRP and StandbyRP
func findStandbyRP(t *testing.T, dut *ondatra.DUTDevice, supervisors []string) (string, string) {
	var activeRP, standbyRP string
	for _, supervisor := range supervisors {
		role := dut.Telemetry().Component(supervisor).RedundantRole().Get(t)
		t.Logf("Component(supervisor).RedundantRole().Get(t): %v, Role: %v", supervisor, role)
		if role == standbyController {
			standbyRP = supervisor
		} else if role == activeController {
			activeRP = supervisor
		} else {
			t.Fatalf("Expected controller %s to be active or standby, got %v", supervisor, role)
		}
	}
	if standbyRP == "" || activeRP == "" {
		t.Fatalf("Expected non-empty activeRP and standbyRP, got activeRP: %v, standbyRP: %v", activeRP, standbyRP)
	}
	t.Logf("Detected activeRP: %v, standbyRP: %v", activeRP, standbyRP)

	return standbyRP, activeRP
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
	supervisors := findComponentsByType(t, dut, controlcardType)
	t.Logf("Found supervisor list: %v", supervisors)
	// Only perform the switchover for the chassis with dual RPs/Supervisors.
	if len(supervisors) != 2 {
		t.Skipf("Dual RP/SUP is required on %v: got %v, want 2", dut.Model(), len(supervisors))
	}

	rpStandbyBeforeSwitch, rpActiveBeforeSwitch := findStandbyRP(t, dut, supervisors)
	t.Logf("Detected rpStandby: %v, rpActive: %v", rpStandbyBeforeSwitch, rpActiveBeforeSwitch)

	gnoiClient := dut.RawAPIs().GNOI().Default(t)
	switchoverRequest := &spb.SwitchControlProcessorRequest{
		ControlProcessor: &tpb.Path{
			Elem: []*tpb.PathElem{{Name: rpStandbyBeforeSwitch}},
		},
	}
	t.Logf("switchoverRequest: %v", switchoverRequest)
	switchoverResponse, err := gnoiClient.System().SwitchControlProcessor(context.Background(), switchoverRequest)
	if err != nil {
		t.Fatalf("Failed to perform control processor switchover with unexpected err: %v", err)
	}
	t.Logf("gnoiClient.System().SwitchControlProcessor() response: %v, err: %v", switchoverResponse, err)

	rpStandbyAfterSwitch, rpActiveAfterSwitch := findStandbyRP(t, dut, supervisors)
	t.Logf("Found standbyRP after switchover: %v, activeRP: %v", rpStandbyAfterSwitch, rpActiveAfterSwitch)
	t.Log("Validate OC Switchover time/reason.")
	activeRP := dut.Telemetry().Component(rpActiveAfterSwitch)
	if !activeRP.LastSwitchoverTime().Lookup(t).IsPresent() {
		t.Errorf("activeRP.LastSwitchoverTime().Lookup(t).IsPresent(): got false, want true")
	} else {
		t.Logf("Found activeRP.LastSwitchoverTime(): %v", activeRP.LastSwitchoverTime().Get(t))
	}

	if !activeRP.LastSwitchoverReason().Lookup(t).IsPresent() {
		t.Errorf("activeRP.LastSwitchoverReason().Lookup(t).IsPresent(): got false, want true")
	} else {
		lastSwitchoverReason := activeRP.LastSwitchoverReason().Get(t)
		t.Logf("Found lastSwitchoverReason.GetDetails(): %v", lastSwitchoverReason.GetDetails())
		t.Logf("Found lastSwitchoverReason.GetTrigger().String(): %v", lastSwitchoverReason.GetTrigger().String())
	}

	if !activeRP.LastRebootTime().Lookup(t).IsPresent() {
		t.Errorf("activeRP.LastRebootTime.().Lookup(t).IsPresent(): got false, want true")
	} else {
		lastrebootTime := activeRP.LastRebootTime().Get(t)
		t.Logf("Found lastRebootTime.GetDetails(): %v", lastrebootTime)
	}
	if !activeRP.LastRebootReason().Lookup(t).IsPresent() {
		t.Errorf("activeRP.LastRebootReason.().Lookup(t).IsPresent(): got false, want true")
	} else {
		lastrebootReason := activeRP.LastRebootReason().Get(t)
		t.Logf("Found lastRebootReason.GetDetails(): %v", lastrebootReason)
	}

	// Assume Supervisor Switchover happened, ensure traffic flows again.
	// Verify the entry for 203.0.113.0/24 is active through AFT Telemetry.
	defer clientA.Close(t)
	if err := clientA.Start(t); err != nil {
		t.Fatalf("gRIBI Connection can not be established")
	}
	ipv4Path := args.dut.Telemetry().NetworkInstance(instance).Afts().Ipv4Entry(ateDstNetCIDR)
	if got, want := ipv4Path.Prefix().Get(t), ateDstNetCIDR; got != want {
		t.Errorf("ipv4-entry/state/prefix got %s, want %s", got, want)
	}
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	dstEndPoint := args.top.Interfaces()[atePort2.Name]
	testTraffic(t, args.ate, args.top, srcEndPoint, dstEndPoint)

	top.StopProtocols(t)
	clientA.Close(t)
}
