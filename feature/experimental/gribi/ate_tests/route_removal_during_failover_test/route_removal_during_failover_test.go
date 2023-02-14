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

package route_removal_during_failover_test

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/attrs"
	cmp "github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/gribigo/chk"
	"github.com/openconfig/gribigo/constants"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"

	spb "github.com/openconfig/gnoi/system"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Settings for configuring the baseline testbed with the test
// topology.
//
// The testbed consists of ate:port1 -> dut:port1
// and dut:port2 -> ate:port2.
// There are 64 SubInterfaces between dut:port2
// and ate:port2
//
//   - ate:port1 -> dut:port1 subnet 192.0.2.0/30
//   - ate:port2 -> dut:port2 64 Sub interfaces:
//   - ate:port2.0 -> dut:port2.0 VLAN-ID: 0 subnet 198.51.100.0/30
//   - ate:port2.1 -> dut:port2.1 VLAN-ID: 1 subnet 198.51.100.4/30
//   - ate:port2.2 -> dut:port2.2 VLAN-ID: 2 subnet 198.51.100.8/30
//   - ate:port2.i -> dut:port2.i VLAN-ID i subnet 198.51.100.(4*i)/30
//   - ate:port2.63 -> dut:port2.63 VLAN-ID 63 subnet 198.51.100.252/30
const (
	ipv4PrefixLen       = 30              // ipv4PrefixLen is the ATE and DUT interface IP prefix length.
	IPBlock1            = "198.18.0.1/18" // IPBlock1 represents the ipv4 entries in VRF1
	nhgID               = 1
	controlcardType     = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD
	primaryController   = oc.Platform_ComponentRedundantRole_PRIMARY
	secondaryController = oc.Platform_ComponentRedundantRole_SECONDARY
	switchTrigger       = oc.PlatformTypes_ComponentRedundantRoleSwitchoverReasonTrigger_SYSTEM_INITIATED
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
)

// createIPv4Entries creates IPv4 Entries given the totalCount and starting prefix
func createIPv4Entries(startIP string) []string {

	_, netCIDR, _ := net.ParseCIDR(startIP)
	netMask := binary.BigEndian.Uint32(netCIDR.Mask)
	firstIP := binary.BigEndian.Uint32(netCIDR.IP)
	lastIP := (firstIP & netMask) | (netMask ^ 0xffffffff)
	entries := []string{}
	for i := firstIP; i <= lastIP; i++ {
		ip := make(net.IP, 4)
		binary.BigEndian.PutUint32(ip, i)
		entries = append(entries, fmt.Sprint(ip))
	}
	return entries
}

// pushDefaultEntries creates NextHopGroup entries using the 64 SubIntf address and creates 1000 IPV4 Entries.
func pushDefaultEntries(t *testing.T, args *testArgs, nextHops []string) {

	for i := range nextHops {
		index := uint64(i + 1)
		args.client.Modify().AddEntry(t,
			fluent.NextHopEntry().
				WithNetworkInstance(*deviations.DefaultNetworkInstance).
				WithIndex(index).
				WithIPAddress(nextHops[i]).
				WithElectionID(args.electionID.Low, args.electionID.High))

		args.client.Modify().AddEntry(t,
			fluent.NextHopGroupEntry().
				WithNetworkInstance(*deviations.DefaultNetworkInstance).
				WithID(uint64(nhgID)).
				AddNextHop(index, 64).
				WithElectionID(args.electionID.Low, args.electionID.High))
	}

	time.Sleep(time.Minute)
	virtualVIPs := createIPv4Entries("198.18.196.1/22")

	for ip := range virtualVIPs {
		args.client.Modify().AddEntry(t,
			fluent.IPv4Entry().
				WithPrefix(virtualVIPs[ip]+"/32").
				WithNetworkInstance(*deviations.DefaultNetworkInstance).
				WithNextHopGroup(uint64(nhgID)).
				WithElectionID(args.electionID.Low, args.electionID.High))
	}
	if err := awaitTimeout(args.ctx, args.client, t, time.Minute); err != nil {
		t.Fatalf("Could not program entries via clientA, got err: %v", err)
	}

	for ip := range virtualVIPs {
		chk.HasResult(t, args.client.Results(t),
			fluent.OperationResult().
				WithIPv4Operation(virtualVIPs[ip]+"/32").
				WithOperationType(constants.Add).
				WithProgrammingResult(fluent.InstalledInFIB).
				AsResult(),
			chk.IgnoreOperationID(),
		)
	}
}

// pushConfig pushes the configuration generated by this
// struct to the device using gNMI SetReplace.
func pushConfig(t *testing.T, dut *ondatra.DUTDevice, dutPort *ondatra.Port, d *oc.Root) {
	t.Helper()

	iname := dutPort.Name()
	i := d.GetOrCreateInterface(iname)
	gnmi.Replace(t, dut, gnmi.OC().Interface(iname).Config(), i)
}

// configureInterfaceDUT configures a single DUT layer 2 port.
func configureInterfaceDUT(t *testing.T, dutPort *ondatra.Port, d *oc.Root, desc string) {
	t.Helper()

	i := d.GetOrCreateInterface(dutPort.Name())
	i.Description = ygot.String(desc)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	if *deviations.InterfaceEnabled {
		i.Enabled = ygot.Bool(true)
	}
	t.Logf("DUT port %s configured", dutPort)
}

// generateSubIntfPair takes the number of subInterfaces, dut,ate,ports and Ixia topology.
// It configures ATE/DUT SubInterfaces on the target device
// It returns a slice of the corresponding ATE IPAddresses.
func generateSubIntfPair(t *testing.T, dut *ondatra.DUTDevice, dutPort *ondatra.Port, ate *ondatra.ATEDevice, atePort *ondatra.Port, top *ondatra.ATETopology, d *oc.Root) []string {
	nextHops := []string{}
	nextHopCount := 63 // nextHopCount specifies number of nextHop IPs needed.
	for i := 0; i <= nextHopCount; i++ {
		vlanID := uint16(i)
		if *deviations.NoMixOfTaggedAndUntaggedSubinterfaces {
			vlanID = uint16(i) + 1
		}
		name := fmt.Sprintf(`dst%d`, i)
		Index := uint32(i)
		ateIPv4 := fmt.Sprintf(`198.51.100.%d`, ((4 * i) + 1))
		dutIPv4 := fmt.Sprintf(`198.51.100.%d`, ((4 * i) + 2))
		configureSubinterfaceDUT(t, d, dutPort, Index, vlanID, dutIPv4)
		configureATE(t, top, atePort, name, vlanID, dutIPv4, ateIPv4+"/30")
		nextHops = append(nextHops, ateIPv4)
	}
	configureInterfaceDUT(t, dutPort, d, "dst")
	pushConfig(t, dut, dutPort, d)
	return nextHops
}

// configureSubinterfaceDUT configures a single DUT layer 3 sub-interface.
func configureSubinterfaceDUT(t *testing.T, d *oc.Root, dutPort *ondatra.Port, index uint32, vlanID uint16, dutIPv4 string) {
	t.Helper()

	i := d.GetOrCreateInterface(dutPort.Name())
	s := i.GetOrCreateSubinterface(index)
	if vlanID != 0 {
		if *deviations.DeprecatedVlanID {
			s.GetOrCreateVlan().VlanId = oc.UnionUint16(vlanID)
		} else {
			s.GetOrCreateVlan().GetOrCreateMatch().GetOrCreateSingleTagged().VlanId = ygot.Uint16(vlanID)
		}
	}

	sipv4 := s.GetOrCreateIpv4()

	if *deviations.InterfaceEnabled && !*deviations.IPv4MissingEnabled {
		sipv4.Enabled = ygot.Bool(true)
	}

	a := sipv4.GetOrCreateAddress(dutIPv4)
	a.PrefixLength = ygot.Uint8(uint8(ipv4PrefixLen))

}

// configureATE configures a single ATE layer 3 interface.
func configureATE(t *testing.T, top *ondatra.ATETopology, atePort *ondatra.Port, Name string, vlanID uint16, dutIPv4 string, ateIPv4 string) {
	t.Helper()

	i := top.AddInterface(Name).WithPort(atePort)
	if vlanID != 0 {
		i.Ethernet().WithVLANID(vlanID)
	}
	i.IPv4().WithAddress(ateIPv4)
	i.IPv4().WithDefaultGateway(dutIPv4)
}

// awaitTimeout calls a fluent client Await, adding a timeout to the context.
func awaitTimeout(ctx context.Context, c *fluent.GRIBIClient, t testing.TB, timeout time.Duration) error {
	subctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return c.Await(subctx, t)
}

// testArgs holds the objects needed by a test case.
type testArgs struct {
	ctx        context.Context
	client     *fluent.GRIBIClient
	dut        *ondatra.DUTDevice
	ate        *ondatra.ATEDevice
	top        *ondatra.ATETopology
	electionID gribi.Uint128
}

// createTrafficFlow generates traffic flow from source network to
// destination network via srcEndPoint to dstEndPoint and checks for
// packet loss.
func createTrafficFlow(t *testing.T, ate *ondatra.ATEDevice, top *ondatra.ATETopology) *ondatra.Flow {
	ethHeader := ondatra.NewEthernetHeader()
	ipv4Header := ondatra.NewIPv4Header()
	ipv4Header.DstAddressRange().
		WithMin("198.18.196.1").
		WithMax("198.18.199.255").
		WithCount(1020)

	srcEndPoint := top.Interfaces()["src"]
	dstEndPoint := []ondatra.Endpoint{}
	for intf, intfData := range top.Interfaces() {
		if intf != "src" {
			dstEndPoint = append(dstEndPoint, intfData)
		}
	}

	flow := ate.Traffic().NewFlow("Flow").
		WithSrcEndpoints(srcEndPoint).
		WithDstEndpoints(dstEndPoint...).
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
	flowPath := gnmi.OC().Flow(flow.Name())
	if got := gnmi.Get(t, ate, flowPath.LossPct().State()); got > 0 {
		t.Errorf("LossPct for flow %s got %g, want 0", flow.Name(), got)
	} else {
		t.Logf("Traffic flows fine from ATE-port1 to ATE-port2")
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

// Function to stop traffic
func stopTraffic(t *testing.T, ate *ondatra.ATEDevice) {
	t.Logf("Stopping traffic")
	ate.Traffic().Stop(t)
}

// switchoverReady is to check if controller is ready for switchover
func switchoverReady(t *testing.T, dut *ondatra.DUTDevice, controller string) bool {
	switchoverReady := gnmi.OC().Component(controller).SwitchoverReady()
	_, ok := gnmi.Watch(t, dut, switchoverReady.State(), 30*time.Minute, func(val *ygnmi.Value[bool]) bool {
		ready, present := val.Val()
		return present && ready
	}).Await(t)
	return ok
}

// validateTelemetry validates telemetry sensors
func validateSwitchoverTelemetry(t *testing.T, dut *ondatra.DUTDevice, primaryAfterSwitch string) {
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
}

// checkNIHasNEntries uses the Get RPC to validate that the network instance named ni
// contains want (an integer) entries.
func checkNIHasNEntries(ctx context.Context, c *fluent.GRIBIClient, ni string, t testing.TB) int {
	t.Helper()
	gr, err := c.Get().
		WithNetworkInstance(ni).
		WithAFT(fluent.AllAFTs).
		Send()

	if err != nil {
		t.Fatalf("Unexpected error from get, got: %v", err)
	}
	return len(gr.GetEntry())
}

// Send traffic and validate traffic.
func testTraffic(t *testing.T, args testArgs, flow *ondatra.Flow) {

	sendTraffic(t, args.ate, flow)
	stopTraffic(t, args.ate)
	verifyTraffic(t, args.ate, flow)

}

// validateSwitchoverStatus is to validate switchover status.
func validateSwitchoverStatus(t *testing.T, dut *ondatra.DUTDevice, secondaryBeforeSwitch string) string {
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
	return secondaryBeforeSwitch

}

// TestRouteRemovalDuringFailover is to test gRIBI flush and slave switchover
// concurrently, validate reinject of gRIBI programmed routes and traffic.
func TestRouteRemovalDuringFailover(t *testing.T) {
	d := &oc.Root{}
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	ctx := context.Background()
	gribic := dut.RawAPIs().GRIBI().Default(t)
	dp1 := dut.Port(t, "port1")
	ap1 := ate.Port(t, "port1")
	top := ate.Topology().New()
	// configure DUT port#1 - source port
	configureSubinterfaceDUT(t, d, dp1, 0, 0, dutPort1.IPv4)
	configureInterfaceDUT(t, dp1, d, "src")
	configureATE(t, top, ap1, "src", 0, dutPort1.IPv4, atePort1.IPv4CIDR())
	pushConfig(t, dut, dp1, d)
	dp2 := dut.Port(t, "port2")
	ap2 := ate.Port(t, "port2")
	// Configure 64 subinterfaces on DUT-ATE- PORT#2
	subIntfIPs := generateSubIntfPair(t, dut, dp2, ate, ap2, top, d)
	top.Push(t).StartProtocols(t)

	// Connect gRIBI client to DUT referred to as gRIBI - using PRESERVE persistence and
	// SINGLE_PRIMARY mode, with FIB ACK requested. Specify gRIBI as the leader.
	client := fluent.NewClient()
	client.Connection().WithStub(gribic).WithPersistence().WithInitialElectionID(1, 0).
		WithFIBACK().WithRedundancyMode(fluent.ElectedPrimaryClient)
	client.Start(ctx, t)
	defer client.Stop(t)

	defer func() {
		// Flush all entries after test.
		if err := gribi.FlushAll(client); err != nil {
			t.Error(err)
		}
	}()

	client.StartSending(ctx, t)
	if err := awaitTimeout(ctx, client, t, time.Minute); err != nil {
		t.Fatalf("Await got error during session negotiation for clientA: %v", err)
	}
	eID := gribi.BecomeLeader(t, client)

	t.Log("Flush existing gRIBI routes before test.")
	if err := gribi.FlushAll(client); err != nil {
		t.Fatal(err)
	}

	args := &testArgs{
		ctx:        ctx,
		client:     client,
		dut:        dut,
		ate:        ate,
		top:        top,
		electionID: eID,
	}
	// virtualVIPs are ipv4 entries in default vrf
	pushDefaultEntries(t, args, subIntfIPs)

	// Send traffic from ATE port-1 to prefixes in IPBlock1 and ensure traffic
	// flows 100% and reaches ATE port-2.
	flow := createTrafficFlow(t, args.ate, args.top)
	testTraffic(t, *args, flow)

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

	gnoiClient := dut.RawAPIs().GNOI().Default(t)
	switchoverRequest := &spb.SwitchControlProcessorRequest{
		ControlProcessor: cmp.GetSubcomponentPath(secondaryBeforeSwitch),
	}
	t.Logf("switchoverRequest: %v", switchoverRequest)

	entriesBefore := checkNIHasNEntries(ctx, client, *deviations.DefaultNetworkInstance, t)

	// Concurrently run switchover and gribi route flush
	t.Log("Execute gRIBi flush and master switchover concurrently")
	go func(msg string) {
		if _, err := gribi.Flush(client, eID, *deviations.DefaultNetworkInstance); err != nil {
			t.Logf("Unexpected error from flush, got: %v", err)
		}
	}("gRIBi Flush")

	go func(msg string) {
		switchoverResponse, err := gnoiClient.System().SwitchControlProcessor(context.Background(), switchoverRequest)
		if err != nil {
			t.Logf("Failed to perform control processor switchover with unexpected err: %v", err)
		}
		t.Logf("gnoiClient.System().SwitchControlProcessor() response: %v, err: %v", switchoverResponse, err)
	}("Master Switchover")

	t.Log("Switchover/Flush is completed, validate switchoverStatus now")

	primaryAfterSwitch := validateSwitchoverStatus(t, dut, secondaryBeforeSwitch)

	validateSwitchoverTelemetry(t, dut, primaryAfterSwitch)

	// Following reconnection of the gRIBI client to a new master supervisor,
	// validate if partially deleted entries of IPBlock1  are not present in the FIB
	// using a get RPC.
	// Connect gRIBI client to DUT referred to as gRIBI - using PRESERVE persistence and
	// SINGLE_PRIMARY mode, with FIB ACK requested. Specify gRIBI as the leader.
	// Check vars for WithInitialElectionID

	t.Log("Added wait time to make sure things are stablized post switchover")
	time.Sleep(3 * time.Minute)

	t.Log("Reconnect gRIBi client after switchover on new master")
	client.Connection().WithStub(gribic).WithPersistence().WithInitialElectionID(eID.Low, eID.High).
		WithFIBACK().WithRedundancyMode(fluent.ElectedPrimaryClient)

	client.Start(ctx, t)
	defer client.Stop(t)
	defer func() {
		// Flush all entries after test.
		if err := gribi.FlushAll(client); err != nil {
			t.Error(err)
		}
	}()

	// Reconnect gribi client
	client.StartSending(ctx, t)

	if err := awaitTimeout(ctx, client, t, time.Minute); err != nil {
		t.Log("Try to connect gRIBi client again, retrying...")
		client.Connection().WithStub(gribic).WithPersistence().WithInitialElectionID(eID.Low, eID.High).
			WithFIBACK().WithRedundancyMode(fluent.ElectedPrimaryClient)
		client.Start(ctx, t)
		client.StartSending(ctx, t)
		if err := awaitTimeout(ctx, client, t, time.Minute); err != nil {
			t.Fatalf("Await got error during session negotiation for client: %v", err)
		}
	}

	t.Log("Compare route entries after switchover")
	entriesAfter := checkNIHasNEntries(ctx, client, *deviations.DefaultNetworkInstance, t)

	if entriesAfter >= entriesBefore {
		t.Error("After switchover, on new master seeing unexpected number of route entries")
		t.Errorf("Network instance has %d entries before switchover, found after switchover: %d", entriesBefore, entriesAfter)
	}

	// TODO: Check for coredumps in the DUT and validate that none are present post failover

	t.Log("Re-inject routes from IPBlock1 in default VRF with NHGID: #1.")
	pushDefaultEntries(t, args, subIntfIPs)

	t.Log("Send traffic and validate")
	testTraffic(t, *args, flow)

	top.StopProtocols(t)
}
