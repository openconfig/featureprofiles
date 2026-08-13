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
	ipv4PrefixLen      = 30
	ipv6PrefixLen      = 126
	ateDstNetStartIP   = "203.0.113.1"
	ateDstNetStartIPv6 = "2001:db8:203:0:113::1"

	ateDstPhase2StartIP   = "203.0.114.1"
	ateDstPhase2StartIPv6 = "2001:db8:203:0:114::1"

	ipv4Prefix1Start = "203.0.113."
	ipv6Prefix1Start = "2001:db8:203:0:113::"
	ipv4Prefix2Start = "203.0.114."
	ipv6Prefix2Start = "2001:db8:203:0:114::"
	pfxCount         = 50

	staticNH            = "192.0.2.6"
	nhIndex             = 1
	nhgIndex            = 42
	controlcardType     = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD
	primaryController   = oc.Platform_ComponentRedundantRole_PRIMARY
	secondaryController = oc.Platform_ComponentRedundantRole_SECONDARY
	switchTrigger       = oc.PlatformTypes_ComponentRedundantRoleSwitchoverReasonTrigger_USER_INITIATED
	maxSwitchoverTime   = 900
	flowV4Name          = "FlowV4"
	flowV6Name          = "FlowV6"
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

func generatePrefixes(ipv4Start, ipv6Start string, count int) ([]string, []string) {
	var ipv4s, ipv6s []string
	for i := 1; i <= count; i++ {
		ipv4s = append(ipv4s, fmt.Sprintf("%s%d/32", ipv4Start, i))
		ipv6s = append(ipv6s, fmt.Sprintf("%s%x/128", ipv6Start, i))
	}
	return ipv4s, ipv6s
}

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
func configureATE(t *testing.T, ate *ondatra.ATEDevice, phase2 bool) gosnappi.Config {
	t.Helper()
	top := gosnappi.NewConfig()

	p1 := ate.Port(t, "port1")
	p2 := ate.Port(t, "port2")

	atePort1.AddToOTG(top, p1, &dutPort1)
	atePort2.AddToOTG(top, p2, &dutPort2)

	// Phase 1 Flows
	flowV4_1 := top.Flows().Add().SetName(flowV4Name)
	flowV4_1.Metrics().SetEnable(true)
	e1 := flowV4_1.Packet().Add().Ethernet()
	e1.Src().SetValue(atePort1.MAC)
	flowV4_1.TxRx().Device().SetTxNames([]string{atePort1.Name + ".IPv4"}).SetRxNames([]string{atePort2.Name + ".IPv4"})
	v4_1 := flowV4_1.Packet().Add().Ipv4()
	v4_1.Src().SetValue(atePort1.IPv4)
	v4_1.Dst().Increment().SetStart(ateDstNetStartIP).SetCount(pfxCount)

	flowV6_1 := top.Flows().Add().SetName(flowV6Name)
	flowV6_1.Metrics().SetEnable(true)
	e2 := flowV6_1.Packet().Add().Ethernet()
	e2.Src().SetValue(atePort1.MAC)
	flowV6_1.TxRx().Device().SetTxNames([]string{atePort1.Name + ".IPv6"}).SetRxNames([]string{atePort2.Name + ".IPv6"})
	v6_1 := flowV6_1.Packet().Add().Ipv6()
	v6_1.Src().SetValue(atePort1.IPv6)
	v6_1.Dst().Increment().SetStart(ateDstNetStartIPv6).SetCount(pfxCount)

	if phase2 {
		// Phase 2 Flows
		flowV4_2 := top.Flows().Add().SetName(flowV4Name + "Phase2")
		flowV4_2.Metrics().SetEnable(true)
		e3 := flowV4_2.Packet().Add().Ethernet()
		e3.Src().SetValue(atePort1.MAC)
		flowV4_2.TxRx().Device().SetTxNames([]string{atePort1.Name + ".IPv4"}).SetRxNames([]string{atePort2.Name + ".IPv4"})
		v4_2 := flowV4_2.Packet().Add().Ipv4()
		v4_2.Src().SetValue(atePort1.IPv4)
		v4_2.Dst().Increment().SetStart(ateDstPhase2StartIP).SetCount(pfxCount)

		flowV6_2 := top.Flows().Add().SetName(flowV6Name + "Phase2")
		flowV6_2.Metrics().SetEnable(true)
		e4 := flowV6_2.Packet().Add().Ethernet()
		e4.Src().SetValue(atePort1.MAC)
		flowV6_2.TxRx().Device().SetTxNames([]string{atePort1.Name + ".IPv6"}).SetRxNames([]string{atePort2.Name + ".IPv6"})
		v6_2 := flowV6_2.Packet().Add().Ipv6()
		v6_2.Src().SetValue(atePort1.IPv6)
		v6_2.Dst().Increment().SetStart(ateDstPhase2StartIPv6).SetCount(pfxCount)
	}

	return top
}

// Verify traffic
func verifyTraffic(t *testing.T, ate *ondatra.ATEDevice, flows []string) {
	for _, flowName := range flows {
		lossPct := otgutils.GetFlowLossPct(t, ate.OTG(), flowName, time.Second*5)
		if lossPct > 0 {
			t.Errorf("LossPct for flow %s got %f, want 0", flowName, lossPct)
		} else {
			t.Logf("Traffic flows fine for flow %s from ATE-port1 to ATE-port2", flowName)
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

// routeInstall configures IPv4 and IPv6 entries through clientA. Ensure that the entries via ClientA
// are active through AFT Telemetry.
func routeInstall(ctx context.Context, t *testing.T, args *testArgs, ipv4Prefixes, ipv6Prefixes []string) {
	vrf := deviations.DefaultNetworkInstance(args.dut)
	args.clientA.AddNH(t, nhIndex, atePort2.IPv4, vrf, fluent.InstalledInFIB)
	args.clientA.AddNHG(t, nhgIndex, map[uint64]uint64{nhIndex: 1}, vrf, fluent.InstalledInFIB)

	args.clientA.AddIPv4s(t, ipv4Prefixes, nhgIndex, vrf, "", fluent.InstalledInFIB)
	args.clientA.AddIPv6s(t, ipv6Prefixes, nhgIndex, vrf, "", fluent.InstalledInFIB)
}

func verifyAFT(t *testing.T, dut *ondatra.DUTDevice, ipv4Prefixes, ipv6Prefixes []string) {
	vrf := deviations.DefaultNetworkInstance(dut)
	for _, pfx := range ipv4Prefixes {
		ipv4Path := gnmi.OC().NetworkInstance(vrf).Afts().Ipv4Entry(pfx)
		if _, found := gnmi.Watch(t, dut, ipv4Path.State(), 2*time.Minute, func(val *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv4Entry]) bool {
			value, present := val.Val()
			return present && value.GetPrefix() == pfx
		}).Await(t); !found {
			t.Fatalf("Could not find prefix %s in telemetry AFT", pfx)
		}
	}
	for _, pfx := range ipv6Prefixes {
		ipv6Path := gnmi.OC().NetworkInstance(vrf).Afts().Ipv6Entry(pfx)
		if _, found := gnmi.Watch(t, dut, ipv6Path.State(), 2*time.Minute, func(val *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv6Entry]) bool {
			value, present := val.Val()
			return present && value.GetPrefix() == pfx
		}).Await(t); !found {
			t.Fatalf("Could not find prefix %s in telemetry AFT", pfx)
		}
	}
	t.Logf("All prefixes found in AFT telemetry.")
}

func TestSupFailure(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ctx := context.Background()

	// Configure the DUT
	configureDUT(t, dut)

	// Configure the ATE
	ate := ondatra.ATE(t, "ate")
	top := configureATE(t, ate, false)
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)

	// Configure the gRIBI client clientA
	clientA := gribi.Client{
		DUT:         dut,
		FIBACK:      true,
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

	ipv4Pfx1, ipv6Pfx1 := generatePrefixes(ipv4Prefix1Start, ipv6Prefix1Start, pfxCount)
	ipv4Pfx2, ipv6Pfx2 := generatePrefixes(ipv4Prefix2Start, ipv6Prefix2Start, pfxCount)

	// Program a route and ensure AFT telemetry returns FIB_PROGRAMMED
	routeInstall(ctx, t, args, ipv4Pfx1, ipv6Pfx1)
	verifyAFT(t, dut, ipv4Pfx1, ipv6Pfx1)

	// Verify that static routes to ATE port-2 are preferred by the traffic.
	t.Logf("Starting traffic")
	ate.OTG().StartTraffic(t)
	defer ate.OTG().StopTraffic(t)
	time.Sleep(15 * time.Second)

	t.Logf("Verifying traffic flows 100%% before switchover")
	otgutils.LogFlowMetrics(t, ate.OTG(), top)
	verifyTraffic(t, args.ate, []string{flowV4Name, flowV6Name})

	// We leave traffic running during switchover!

	controllers := cmp.FindComponentsByType(t, dut, controlcardType)
	t.Logf("Found controller list: %v", controllers)
	// Only perform the switchover for the chassis with dual controllers.
	if len(controllers) != 2 {
		t.Skipf("Dual controllers required on %v: got %v, want 2", dut.Model(), len(controllers))
	}

	secondaryBeforeSwitch, primaryBeforeSwitch := cmp.FindStandbyControllerCard(t, dut, controllers)

	if ok := cmp.SwitchoverReady(t, dut, primaryBeforeSwitch); !ok {
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
	secondaryAfterSwitch := primaryBeforeSwitch

	wantTrigger := switchTrigger
	if deviations.GNOISwitchoverReasonMissingUserInitiated(dut) {
		wantTrigger = oc.PlatformTypes_ComponentRedundantRoleSwitchoverReasonTrigger_SYSTEM_INITIATED
	}
	cmp.ValidateTelemetry(t, dut, primaryAfterSwitch, secondaryAfterSwitch, wantTrigger)

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

	vrf := deviations.DefaultNetworkInstance(dut)

	// Re-verify that prefixes are still present via Get RPC
	clientA.VerifyRestoredIPv4s(t, ipv4Pfx1, vrf)
	clientA.VerifyRestoredIPv6s(t, ipv6Pfx1, vrf)

	// Verify Phase 1 prefixes are active through AFT Telemetry.
	verifyAFT(t, dut, ipv4Pfx1, ipv6Pfx1)

	otgutils.LogFlowMetrics(t, ate.OTG(), top)
	verifyTraffic(t, args.ate, []string{flowV4Name, flowV6Name})
	ate.OTG().StopTraffic(t)

	// TE-8.2.2 Phase 2 post-switchover FIB validation
	t.Log("Programming Phase 2 prefixes post-switchover")
	clientA.AddIPv4s(t, ipv4Pfx2, nhgIndex, vrf, "", fluent.InstalledInFIB)
	clientA.AddIPv6s(t, ipv6Pfx2, nhgIndex, vrf, "", fluent.InstalledInFIB)

	verifyAFT(t, dut, ipv4Pfx2, ipv6Pfx2)

	// Update traffic generator for all prefixes
	top = configureATE(t, ate, true)
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	ate.OTG().StartTraffic(t)
	time.Sleep(15 * time.Second)
	ate.OTG().StopTraffic(t)

	otgutils.LogFlowMetrics(t, ate.OTG(), top)
	verifyTraffic(t, args.ate, []string{flowV4Name, flowV6Name, flowV4Name + "Phase2", flowV6Name + "Phase2"})
}
