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
	"net"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	cmp "github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/featureprofiles/internal/iputil"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/gnoigo/system"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/testt"

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
//   * ate:port1 -> dut:port1 subnet 192.0.2.0/30 and 2001:db8::192:0:2:0/126
//   * ate:port2 -> dut:port2 subnet 192.0.2.4/30 and 2001:db8::192:0:2:4/126

const (
	ipv4PrefixLen       = 30
	ipv6PrefixLen       = 126
	nhIndex             = 1
	nhgIndex            = 42
	controlcardType     = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD
	primaryController   = oc.Platform_ComponentRedundantRole_PRIMARY
	secondaryController = oc.Platform_ComponentRedundantRole_SECONDARY
	switchTrigger       = oc.PlatformTypes_ComponentRedundantRoleSwitchoverReasonTrigger_USER_INITIATED
	maxSwitchoverTime   = 900
	flowIPv4Initial     = "flow-ipv4-initial"
	flowIPv6Initial     = "flow-ipv6-initial"
	flowIPv4Post        = "flow-ipv4-post"
	flowIPv6Post        = "flow-ipv6-post"
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "dutPort1",
		IPv4:    "192.0.2.1",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:db8::192:0:2:1",
		IPv6Len: ipv6PrefixLen,
	}

	atePort1 = attrs.Attributes{
		Name:    "atePort1",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.0.2.2",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:db8::192:0:2:2",
		IPv6Len: ipv6PrefixLen,
	}

	dutPort2 = attrs.Attributes{
		Desc:    "dutPort2",
		IPv4:    "192.0.2.5",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:db8::192:0:2:5",
		IPv6Len: ipv6PrefixLen,
	}

	atePort2 = attrs.Attributes{
		Name:    "atePort2",
		MAC:     "02:00:02:01:01:01",
		IPv4:    "192.0.2.6",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:db8::192:0:2:6",
		IPv6Len: ipv6PrefixLen,
	}
)

// configureDUT configures port1 and port2 on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	d := gnmi.OC()

	p1 := dut.Port(t, "port1")
	gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), dutPort1.NewOCInterface(p1.Name(), dut))

	p2 := dut.Port(t, "port2")
	gnmi.Replace(t, dut, d.Interface(p2.Name()).Config(), dutPort2.NewOCInterface(p2.Name(), dut))

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, p1)
		fptest.SetPortSpeed(t, p2)
	}
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, p1.Name(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, p2.Name(), deviations.DefaultNetworkInstance(dut), 0)
	}
}

// configureATE configures port1 and port2 on the ATE and defines 4 traffic flows
func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	t.Helper()
	top := gosnappi.NewConfig()

	p1 := ate.Port(t, "port1")
	p2 := ate.Port(t, "port2")

	atePort1.AddToOTG(top, p1, &dutPort1)
	atePort2.AddToOTG(top, p2, &dutPort2)

	// 1. flow-ipv4-initial
	f4i := top.Flows().Add().SetName(flowIPv4Initial)
	f4i.Metrics().SetEnable(true)
	f4i.Packet().Add().Ethernet().Src().SetValue(atePort1.MAC)
	f4i.TxRx().Device().SetTxNames([]string{atePort1.Name + ".IPv4"}).SetRxNames([]string{atePort2.Name + ".IPv4"})
	v4i := f4i.Packet().Add().Ipv4()
	v4i.Src().SetValue(atePort1.IPv4)
	v4i.Dst().Increment().SetStart("203.0.113.1").SetCount(50)

	// 2. flow-ipv6-initial
	f6i := top.Flows().Add().SetName(flowIPv6Initial)
	f6i.Metrics().SetEnable(true)
	f6i.Packet().Add().Ethernet().Src().SetValue(atePort1.MAC)
	f6i.TxRx().Device().SetTxNames([]string{atePort1.Name + ".IPv6"}).SetRxNames([]string{atePort2.Name + ".IPv6"})
	v6i := f6i.Packet().Add().Ipv6()
	v6i.Src().SetValue(atePort1.IPv6)
	v6i.Dst().Increment().SetStart("2001:db8:203:0:113::1").SetCount(50)

	// 3. flow-ipv4-post
	f4p := top.Flows().Add().SetName(flowIPv4Post)
	f4p.Metrics().SetEnable(true)
	f4p.Packet().Add().Ethernet().Src().SetValue(atePort1.MAC)
	f4p.TxRx().Device().SetTxNames([]string{atePort1.Name + ".IPv4"}).SetRxNames([]string{atePort2.Name + ".IPv4"})
	v4p := f4p.Packet().Add().Ipv4()
	v4p.Src().SetValue(atePort1.IPv4)
	v4p.Dst().Increment().SetStart("203.0.114.1").SetCount(50)

	// 4. flow-ipv6-post
	f6p := top.Flows().Add().SetName(flowIPv6Post)
	f6p.Metrics().SetEnable(true)
	f6p.Packet().Add().Ethernet().Src().SetValue(atePort1.MAC)
	f6p.TxRx().Device().SetTxNames([]string{atePort1.Name + ".IPv6"}).SetRxNames([]string{atePort2.Name + ".IPv6"})
	v6p := f6p.Packet().Add().Ipv6()
	v6p.Src().SetValue(atePort1.IPv6)
	v6p.Dst().Increment().SetStart("2001:db8:203:0:114::1").SetCount(50)

	return top
}

// verifyTraffic verifies the healthy transmission (0% loss) for specified flows.
func verifyTraffic(t *testing.T, ate *ondatra.ATEDevice, flowNames []string) {
	t.Helper()
	for _, flowName := range flowNames {
		flowMetrics := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(flowName).Counters().State())
		txPkts := flowMetrics.GetOutPkts()
		rxPkts := flowMetrics.GetInPkts()

		if txPkts == 0 {
			t.Errorf("Flow %s: txPackets is 0", flowName)
			continue
		}
		if got := 100 * float32(txPkts-rxPkts) / float32(txPkts); got > 0 {
			t.Errorf("LossPct for flow %s got %f, want 0", flowName, got)
		} else {
			t.Logf("Traffic flow %s is healthy (0%% loss)", flowName)
		}
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

// programRoutes programs a list of IPv4 and IPv6 host routes via gRIBI.
func programRoutes(t *testing.T, args *testArgs, ipv4s []string, ipv6s []string) {
	vrf := deviations.DefaultNetworkInstance(args.dut)
	wantACK := fluent.InstalledInRIB

	for _, ip := range ipv4s {
		prefix := ip + "/32"
		args.clientA.AddIPv4(t, prefix, nhgIndex, vrf, "", wantACK)
	}
	for _, ip := range ipv6s {
		prefix := ip + "/128"
		args.clientA.AddIPv6(t, prefix, nhgIndex, vrf, "", wantACK)
	}
}

// verifyAftTelemetry verifies programmed prefixes are active in DUT AFT telemetry.
func verifyAftTelemetry(t *testing.T, args *testArgs, ipv4s []string, ipv6s []string) {
	t.Helper()
	vrf := deviations.DefaultNetworkInstance(args.dut)

	for _, ip := range ipv4s {
		prefix := ip + "/32"
		t.Logf("Verify prefix %s is active in AFT telemetry", prefix)
		ipv4Path := gnmi.OC().NetworkInstance(vrf).Afts().Ipv4Entry(prefix)
		if _, found := gnmi.Watch(t, args.dut, ipv4Path.State(), 2*time.Minute, func(val *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv4Entry]) bool {
			value, present := val.Val()
			return present && value.GetPrefix() == prefix
		}).Await(t); !found {
			t.Fatalf("Prefix %s missing in AFT telemetry", prefix)
		}
	}

	for _, ip := range ipv6s {
		prefix := ip + "/128"
		t.Logf("Verify prefix %s is active in AFT telemetry", prefix)
		ipv6Path := gnmi.OC().NetworkInstance(vrf).Afts().Ipv6Entry(prefix)
		if _, found := gnmi.Watch(t, args.dut, ipv6Path.State(), 2*time.Minute, func(val *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv6Entry]) bool {
			value, present := val.Val()
			return present && value.GetPrefix() == prefix
		}).Await(t); !found {
			t.Fatalf("Prefix %s missing in AFT telemetry", prefix)
		}
	}
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

	// Program shared Next Hop and Next Hop Group once.
	vrf := deviations.DefaultNetworkInstance(dut)
	wantACK := fluent.InstalledInRIB
	clientA.AddNH(t, nhIndex, atePort2.IPv4, vrf, wantACK)
	clientA.AddNHG(t, nhgIndex, map[uint64]uint64{nhIndex: 1}, vrf, wantACK)

	args := &testArgs{
		ctx:     ctx,
		clientA: &clientA,
		dut:     dut,
		ate:     ate,
		top:     top,
	}

	// Phase 1: TE-8.2.1 - Program and verify initial 100 prefixes
	t.Log("Phase 1: Programming initial 50 IPv4 and 50 IPv6 prefixes...")
	ipv4Initial := iputil.GenerateIPs("203.0.113.0/24", 51)[1:51]
	ipv6Initial, err := iputil.GenerateIPv6s(net.ParseIP("2001:db8:203:0:113::1"), 50)
	if err != nil {
		t.Fatalf("Failed to generate initial IPv6 addresses: %v", err)
	}

	programRoutes(t, args, ipv4Initial, ipv6Initial)
	verifyAftTelemetry(t, args, ipv4Initial, ipv6Initial)

	// Verify that initial static routes are preferred by the traffic
	t.Log("Starting traffic for initial flows...")
	ate.OTG().StartTraffic(t)
	time.Sleep(15 * time.Second)
	ate.OTG().StopTraffic(t)
	otgutils.LogFlowMetrics(t, ate.OTG(), top)
	verifyTraffic(t, args.ate, []string{flowIPv4Initial, flowIPv6Initial})

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
	secondaryAfterSwitch := secondaryBeforeSwitch
	validateTelemetry(t, dut, primaryAfterSwitch, secondaryAfterSwitch)

	t.Log("Re-establish gRIBI client connection")
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

	t.Log("Regaining gRIBI leadership on the new master supervisor...")
	clientA.BecomeLeader(t)

	t.Log("gRIBI client re-connected and became leader successfully. Sleeping 60 seconds to let gNMI and FIB stabilize...")
	time.Sleep(60 * time.Second)

	// Verify initial prefixes persist after switchover
	t.Log("Verifying initial prefixes persist in telemetry after switchover...")
	verifyAftTelemetry(t, args, ipv4Initial, ipv6Initial)

	// Phase 2: TE-8.2.2 - Post-switchover FIB Programming Validation
	t.Log("Phase 2: Programming post-switchover 50 IPv4 and 50 IPv6 prefixes...")
	ipv4Post := iputil.GenerateIPs("203.0.114.0/24", 51)[1:51]
	ipv6Post, err := iputil.GenerateIPv6s(net.ParseIP("2001:db8:203:0:114::1"), 50)
	if err != nil {
		t.Fatalf("Failed to generate post-switchover IPv6 addresses: %v", err)
	}

	programRoutes(t, args, ipv4Post, ipv6Post)
	verifyAftTelemetry(t, args, ipv4Post, ipv6Post)

	// Verify all 200 prefixes (100 initial + 100 post-switchover) receive traffic without loss
	t.Log("Starting traffic for all flows...")
	ate.OTG().StartTraffic(t)
	time.Sleep(15 * time.Second)
	ate.OTG().StopTraffic(t)
	otgutils.LogFlowMetrics(t, ate.OTG(), top)
	verifyTraffic(t, args.ate, []string{
		flowIPv4Initial, flowIPv6Initial,
		flowIPv4Post, flowIPv6Post,
	})

	ate.OTG().StopTraffic(t)
	args.ate.OTG().StopProtocols(t)
}
