// Copyright 2023 Google LLC
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
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	cmp "github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	gpb "github.com/openconfig/gribi/v1/proto/service"
	"github.com/openconfig/gribigo/chk"
	"github.com/openconfig/gribigo/constants"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/binding"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygot/ygot"

	fpb "github.com/openconfig/gnoi/file"
	spb "github.com/openconfig/gnoi/system"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Settings for configuring the baseline testbed with the test topology.
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
	ipv4PrefixLen       = 30                // ipv4PrefixLen is the ATE and DUT interface IP prefix length.
	ipBlock1            = "198.18.196.1/22" // ipBlock1 represents the ipv4 entries in VRF1
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
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.0.2.2",
		IPv4Len: ipv4PrefixLen,
	}
	vendorCoreFilePath = map[ondatra.Vendor]string{
		ondatra.JUNIPER: "/var/core/",
		ondatra.CISCO:   "/misc/disk1/",
		ondatra.NOKIA:   "/var/core/",
		ondatra.ARISTA:  "/var/core/",
	}
	vendorCoreFileNamePattern = map[ondatra.Vendor]string{
		ondatra.JUNIPER: "rpd",
		ondatra.CISCO:   "emsd.*core.*",
		ondatra.NOKIA:   "coredump-sr_gribi_server-.*",
		ondatra.ARISTA:  "core.*",
	}
)

// coreFileCheck function is used to check if any cores found during test execution.
func coreFilecheck(t *testing.T, dut *ondatra.DUTDevice, gnoiClient binding.GNOIClients, sysConfigTime uint64) {
	// vendorCoreFilePath and vendorCoreProcName should be provided to fetch core file on dut.
	if _, ok := vendorCoreFilePath[dut.Vendor()]; !ok {
		t.Fatalf("Please add support for vendor %v in var vendorCoreFilePath ", dut.Vendor())
	}
	if _, ok := vendorCoreFileNamePattern[dut.Vendor()]; !ok {
		t.Fatalf("Please add support for vendor %v in var vendorCoreFileNamePattern.", dut.Vendor())
	}

	in := &fpb.StatRequest{
		Path: vendorCoreFilePath[dut.Vendor()],
	}
	validResponse, err := gnoiClient.File().Stat(context.Background(), in)
	if err != nil {
		t.Errorf("Unable to stat path %v for core files on DUT, %v", vendorCoreFilePath[dut.Vendor()], err)
	}
	// Check cores creation time is greater than test start time.
	for _, fileStatsInfo := range validResponse.GetStats() {
		if fileStatsInfo.GetLastModified() > sysConfigTime {
			coreFileName := fileStatsInfo.GetPath()
			r := regexp.MustCompile(vendorCoreFileNamePattern[dut.Vendor()])
			if r.Match([]byte(coreFileName)) {
				t.Errorf("Found core %v on DUT post switchover.", coreFileName)
			}
		}
	}
}

// createIPv4Entries creates IPv4 Entries given the totalCount and starting prefix.
func createIPv4Entries(t *testing.T, startIP string) []string {
	_, netCIDR, err := net.ParseCIDR(startIP)
	if err != nil {
		t.Fatalf("Failed to parse prefix: %v", err)
	}
	netMask := binary.BigEndian.Uint32(netCIDR.Mask)
	firstIP := binary.BigEndian.Uint32(netCIDR.IP)
	lastIP := (firstIP & netMask) | (netMask ^ 0xffffffff)
	var entries []string
	for i := firstIP; i <= lastIP; i++ {
		ip := make(net.IP, 4)
		binary.BigEndian.PutUint32(ip, i)
		entries = append(entries, fmt.Sprint(ip))
	}
	return entries
}

// pushDefaultEntries creates NextHopGroup entries using the 64 SubIntf address and creates 1000 IPV4 Entries.
func pushDefaultEntries(t *testing.T, args *testArgs, nextHops, virtualVIPs []string) {
	t.Helper()

	fluentNhgVar := fluent.NextHopGroupEntry()
	fluentNhgVar.WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).WithID(uint64(nhgID)).
		WithElectionID(args.electionID.Low, args.electionID.High)

	for i := range nextHops {
		index := uint64(i + 1)
		args.client.Modify().AddEntry(t,
			fluent.NextHopEntry().
				WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
				WithIndex(index).
				WithIPAddress(nextHops[i]).
				WithElectionID(args.electionID.Low, args.electionID.High))

		fluentNhgVar.AddNextHop(index, 64)
	}

	args.client.Modify().AddEntry(t, fluentNhgVar)

	for ip := range virtualVIPs {
		args.client.Modify().AddEntry(t,
			fluent.IPv4Entry().
				WithPrefix(virtualVIPs[ip]+"/32").
				WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
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

	gr, err := args.client.Get().
		WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
		WithAFT(fluent.IPv4).
		Send()
	if err != nil {
		t.Fatalf("Got unexpected error from get, got: %v", err)
	}

	for ip := range virtualVIPs {
		chk.GetResponseHasEntries(t, gr,
			fluent.IPv4Entry().
				WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
				WithNextHopGroup(uint64(nhgID)).
				WithPrefix(virtualVIPs[ip]+"/32"),
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
func configureInterfaceDUT(t *testing.T, dutPort *ondatra.Port, dut *ondatra.DUTDevice, d *oc.Root, desc string) {

	i := d.GetOrCreateInterface(dutPort.Name())
	i.Description = ygot.String(desc)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	if deviations.InterfaceEnabled(dut) {
		i.Enabled = ygot.Bool(true)
	}
	t.Logf("DUT port %s configured", dutPort)
}

// generateSubIntfPair takes the number of subInterfaces, dut,ate,ports and Ixia topology.
// This function configures ATE/DUT SubInterfaces on the target device and returns a slice of the
// corresponding ATE IPAddresses.
func generateSubIntfPair(t *testing.T, dut *ondatra.DUTDevice, dutPort *ondatra.Port, ate *ondatra.ATEDevice, atePort *ondatra.Port, top gosnappi.Config, d *oc.Root) []string {
	t.Helper()
	nextHops := []string{}
	nextHopCount := 63 // nextHopCount specifies number of nextHop IPs needed.
	for i := 0; i <= nextHopCount; i++ {
		vlanID := uint16(i)
		if deviations.NoMixOfTaggedAndUntaggedSubinterfaces(dut) {
			vlanID = uint16(i) + 1
		}
		name := fmt.Sprintf(`dst%d`, i)
		Index := uint32(i)
		ateIPv4 := fmt.Sprintf(`198.51.100.%d`, ((4 * i) + 1))
		dutIPv4 := fmt.Sprintf(`198.51.100.%d`, ((4 * i) + 2))
		configureSubinterfaceDUT(d, dutPort, Index, vlanID, dutIPv4, dut)
		MAC, err := incrementMAC(atePort1.MAC, i+1)
		if err != nil {
			t.Fatalf("Failed to increment MAC")
		}
		configureATE(top, atePort, name, vlanID, dutIPv4, ateIPv4, MAC)
		nextHops = append(nextHops, ateIPv4)
	}
	configureInterfaceDUT(t, dutPort, dut, d, "dst")
	pushConfig(t, dut, dutPort, d)
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		intf := d.GetOrCreateInterface(dutPort.Name())
		for i := 0; i <= nextHopCount; i++ {
			fptest.AssignToNetworkInstance(t, dut, intf.GetName(), deviations.DefaultNetworkInstance(dut), uint32(i))
		}
	}
	return nextHops
}

// configureSubinterfaceDUT configures a single DUT layer 3 sub-interface.
func configureSubinterfaceDUT(d *oc.Root, dutPort *ondatra.Port, index uint32, vlanID uint16, dutIPv4 string, dut *ondatra.DUTDevice) {
	i := d.GetOrCreateInterface(dutPort.Name())
	s := i.GetOrCreateSubinterface(index)
	if vlanID != 0 {
		if deviations.DeprecatedVlanID(dut) {
			s.GetOrCreateVlan().VlanId = oc.UnionUint16(vlanID)
		} else {
			s.GetOrCreateVlan().GetOrCreateMatch().GetOrCreateSingleTagged().VlanId = ygot.Uint16(vlanID)
		}
	}

	sipv4 := s.GetOrCreateIpv4()

	if deviations.InterfaceEnabled(dut) && !deviations.IPv4MissingEnabled(dut) {
		sipv4.Enabled = ygot.Bool(true)
	}

	a := sipv4.GetOrCreateAddress(dutIPv4)
	a.PrefixLength = ygot.Uint8(uint8(ipv4PrefixLen))
}

// configureATE configures a single ATE layer 3 interface.
func configureATE(top gosnappi.Config, atePort *ondatra.Port, Name string, vlanID uint16, dutIPv4, ateIPv4, ateMAC string) {
	dev := top.Devices().Add().SetName(Name + ".Dev")
	eth := dev.Ethernets().Add().SetName(Name + ".Eth").SetMac(ateMAC)
	eth.Connection().SetChoice(gosnappi.EthernetConnectionChoice.PORT_NAME).SetPortName(atePort.ID())
	if vlanID != 0 {
		eth.Vlans().Add().SetName(Name).SetId(uint32(vlanID))
	}
	eth.Ipv4Addresses().Add().SetName(Name + ".IPv4").SetAddress(ateIPv4).SetGateway(dutIPv4).SetPrefix(uint32(ipv4PrefixLen))

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
	top        gosnappi.Config
	electionID gribi.Uint128
}

// createTrafficFlow generates traffic flow from source network to destination network via
// srcEndPoint to dstEndPoint and checks for packet loss.
func createTrafficFlow(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config, dstMac string) {

	top.Flows().Clear().Items()
	flow := top.Flows().Add().SetName("Flow")
	flow.Metrics().SetEnable(true)
	flow.TxRx().Port().SetTxName("port1").SetRxName("port2")
	e1 := flow.Packet().Add().Ethernet()
	e1.Src().SetValue(atePort1.MAC)
	e1.Dst().SetChoice("value").SetValue(dstMac)
	v4 := flow.Packet().Add().Ipv4()
	v4.Src().SetValue(atePort1.IPv4)
	v4.Dst().Increment().SetStart("198.18.196.1").SetCount(1020)

}

// findSecondaryController finds out primary and secondary controllers.
func findSecondaryController(t *testing.T, dut *ondatra.DUTDevice, controllers []string) (string, string) {
	t.Helper()
	var primary, secondary string
	for _, controller := range controllers {
		role := gnmi.Get(t, dut, gnmi.OC().Component(controller).RedundantRole().State())
		t.Logf("Component(controller).RedundantRole().Get(t): %v, Role: %v", controller, role)
		switch role {
		case secondaryController:
			secondary = controller
		case primaryController:
			primary = controller
		default:
			t.Fatalf("Expected controller %s to be active or standby, got %v", controller, role)
		}
	}
	if secondary == "" || primary == "" {
		t.Fatalf("Expected non-empty primary and secondary Controller, got primary: %v, secondary: %v", primary, secondary)
	}
	t.Logf("Detected primary: %v, secondary: %v", primary, secondary)

	return secondary, primary
}

// switchoverReady is to check if controller is ready for switchover.
func switchoverReady(t *testing.T, dut *ondatra.DUTDevice, controller string) {
	switchoverReady := gnmi.OC().Component(controller).SwitchoverReady()
	gnmi.Await(t, dut, switchoverReady.State(), 30*time.Minute, true)
}

// validateSwitchoverTelemetry validates telemetry sensors.
func validateSwitchoverTelemetry(t *testing.T, dut *ondatra.DUTDevice, primaryAfterSwitch string) {
	t.Helper()
	t.Log("Validate OC Switchover time/reason.")
	primary := gnmi.OC().Component(primaryAfterSwitch)
	if !gnmi.Lookup(t, dut, primary.LastSwitchoverTime().State()).IsPresent() {
		t.Errorf("primary.LastSwitchoverTime().Lookup(t).IsPresent(): got false, want true.")
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
func testTraffic(t *testing.T, args testArgs) {
	t.Helper()
	args.ate.OTG().StartTraffic(t)
	// Send traffic for 1 minute.
	time.Sleep(1 * time.Minute)
	args.ate.OTG().StopTraffic(t)
	otgutils.LogFlowMetrics(t, args.ate.OTG(), args.top)

	flowMetrics := gnmi.Get(t, args.ate.OTG(), gnmi.OTG().Flow("Flow").Counters().State())
	txPkts := float32(flowMetrics.GetInPkts())
	rxPkts := float32(flowMetrics.GetOutPkts())
	if txPkts == 0 {
		t.Fatalf("Tx packets should be higher than 0")
	}

	if got := (txPkts - rxPkts) * 100 / txPkts; got > 0 {
		t.Errorf("LossPct got %f, want 0", got)
	} else {
		t.Logf("Traffic flows fine from ATE-port1 to ATE-port2.")
	}
}

// validateSwitchoverStatus is to validate switchover status.
func validateSwitchoverStatus(t *testing.T, dut *ondatra.DUTDevice, secondaryBeforeSwitch string) string {
	t.Helper()
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

// incrementMAC increments the MAC by i. Returns error if the mac cannot be parsed or overflows the mac address space
func incrementMAC(mac string, i int) (string, error) {
	macAddr, err := net.ParseMAC(mac)
	if err != nil {
		return "", err
	}
	convMac := binary.BigEndian.Uint64(append([]byte{0, 0}, macAddr...))
	convMac = convMac + uint64(i)
	buf := new(bytes.Buffer)
	err = binary.Write(buf, binary.BigEndian, convMac)
	if err != nil {
		return "", err
	}
	newMac := net.HardwareAddr(buf.Bytes()[2:8])
	return newMac.String(), nil
}

// TestRouteRemovalDuringFailover is to test gRIBI flush and slave switchover
// concurrently, validate reinject of gRIBI programmed routes and traffic.
func TestRouteRemovalDuringFailover(t *testing.T) {
	d := &oc.Root{}
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	ctx := context.Background()
	gribic := dut.RawAPIs().GRIBI(t)
	dp1 := dut.Port(t, "port1")
	ap1 := ate.Port(t, "port1")
	top := gosnappi.NewConfig()
	top.Ports().Add().SetName(ap1.ID())
	// configure DUT port#1 - source port.
	configureSubinterfaceDUT(d, dp1, 0, 0, dutPort1.IPv4, dut)
	configureInterfaceDUT(t, dp1, dut, d, "src")
	configureATE(top, ap1, atePort1.Name, 0, dutPort1.IPv4, atePort1.IPv4, atePort1.MAC)
	ate.OTG().PushConfig(t, top)
	pushConfig(t, dut, dp1, d)
	dp2 := dut.Port(t, "port2")
	ap2 := ate.Port(t, "port2")
	top.Ports().Add().SetName(ap2.ID())
	// Configure 64 subinterfaces on DUT-ATE- PORT#2.
	subIntfIPs := generateSubIntfPair(t, dut, dp2, ate, ap2, top, d)
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, dp1.Name(), deviations.DefaultNetworkInstance(dut), 0)
	}
	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, dp1)
		fptest.SetPortSpeed(t, dp2)
	}

	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)

	dutPortName := dut.Port(t, "port1").Name()
	sysConfigTime := gnmi.Get(t, dut, gnmi.OC().Interface(dutPortName).LastChange().State())

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

	virtualIPs := createIPv4Entries(t, ipBlock1)

	t.Log("Inject routes from ipBlock1 in default VRF with NHGID: #1.")
	pushDefaultEntries(t, args, subIntfIPs, virtualIPs)

	gr, err := args.client.Get().
		WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
		WithAFT(fluent.IPv4).
		Send()
	if err != nil {
		t.Fatalf("Got unexpected error from get, got: %v", err)
	}

	for _, r := range gr.GetEntry() {
		entry := r.GetIpv4().Prefix
		if got := r.GetFibStatus().String(); got != "PROGRAMMED" {
			t.Fatalf("gRIBI entry %v is not programmed in FIB, got:%v, want: PROGRAMMED", entry, got)
		}
	}

	// Send traffic from ATE port-1 to prefixes in ipBlock1 and ensure traffic
	// flows 100% and reaches ATE port-2.
	otgutils.WaitForARP(t, args.ate.OTG(), args.top, "IPv4")
	dstMac := gnmi.Get(t, args.ate.OTG(), gnmi.OTG().Interface(atePort1.Name+".Eth").Ipv4Neighbor(dutPort1.IPv4).LinkLayerAddress().State())
	createTrafficFlow(t, args.ate, args.top, dstMac)
	args.ate.OTG().PushConfig(t, top)
	args.ate.OTG().StartProtocols(t)

	testTraffic(t, *args)

	controllers := cmp.FindComponentsByType(t, dut, controlcardType)
	t.Logf("Found controller list: %v", controllers)
	// Only perform the switchover for the chassis with dual controllers.
	if len(controllers) != 2 {
		t.Skipf("Dual controllers required on %v: got %v, want 2", dut.Model(), len(controllers))
	}

	secondaryBeforeSwitch, primaryBeforeSwitch := findSecondaryController(t, dut, controllers)

	switchoverReady(t, dut, primaryBeforeSwitch)
	t.Logf("Controller %q is ready for switchover before test.", primaryBeforeSwitch)

	var gnoiClient binding.GNOIClients = dut.RawAPIs().GNOI(t)
	useNameOnly := deviations.GNOISubcomponentPath(dut)
	switchoverRequest := &spb.SwitchControlProcessorRequest{
		ControlProcessor: cmp.GetSubcomponentPath(secondaryBeforeSwitch, useNameOnly),
	}
	t.Logf("switchoverRequest: %v", switchoverRequest)

	entriesBefore := checkNIHasNEntries(ctx, client, deviations.DefaultNetworkInstance(dut), t)

	// Concurrently run switchover and gribi route flush.
	var flushRes, wantFlushRes *gpb.FlushResponse
	t.Log("Execute gRIBi flush and master switchover concurrently.")
	go func(msg string) {
		flushRes, err = gribi.Flush(client, eID, deviations.DefaultNetworkInstance(dut))
		if err != nil {
			t.Logf("Unexpected error from flush, got: %v, %v", err, flushRes)
		}
		wantFlushRes = &gpb.FlushResponse{
			Result: gpb.FlushResponse_OK,
		}
	}("gRIBi Flush")

	go func(msg string) {
		switchoverResponse, err := gnoiClient.System().SwitchControlProcessor(context.Background(), switchoverRequest)
		if err != nil {
			t.Logf("Failed to perform control processor switchover with unexpected err: %v", err)
		}
		t.Logf("gnoiClient.System().SwitchControlProcessor() response: %v, err: %v", switchoverResponse, err)
	}("Master Switchover")

	// Check the response of gribi flush call. If-else loop for further verification
	// set flag for flush status.

	t.Log("Switchover/Flush is completed, validate switchoverStatus now.")

	primaryAfterSwitch := validateSwitchoverStatus(t, dut, secondaryBeforeSwitch)

	validateSwitchoverTelemetry(t, dut, primaryAfterSwitch)

	// Following reconnection of the gRIBI client to a new master supervisor,
	// validate if partially deleted entries of ipBlock1 are not present in the FIB
	// using a get RPC.
	// Connect gRIBI client to DUT referred to as gRIBI - using PRESERVE persistence and
	// SINGLE_PRIMARY mode, with FIB ACK requested. Specify gRIBI as the leader.
	// Check vars for WithInitialElectionID.

	t.Log("Reconnect gRIBi client after switchover on new master.")
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

	// Reconnect gribi client.
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

	t.Log("Compare route entries after switchover based on flush response.")

	gr, err = args.client.Get().
		WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
		WithAFT(fluent.IPv4).
		Send()
	if err != nil {
		t.Fatalf("got unexpected error from get, got: %v", err)
	}

	var prefixEntryList []string
	for _, r := range gr.GetEntry() {
		entry := strings.Split(r.GetIpv4().Prefix, "/")
		prefixEntryList = append(prefixEntryList, entry[0])
	}

	if len(prefixEntryList) != 0 && flushRes.Result == wantFlushRes.Result {
		t.Errorf("Network instance has %d entries before switchover, found after switchover: %d", entriesBefore, len(prefixEntryList))
	}

	// Check for coredumps in the DUT and validate that none are present post failover.
	// Reconnect gnoi connection after switchover.
	gnoiClient, err = dut.RawAPIs().BindingDUT().DialGNOI(context.Background())
	if err != nil {
		t.Fatalf("Error dialing gNOI: %v", err)
	}
	coreFilecheck(t, dut, gnoiClient, sysConfigTime)

	t.Log("Re-inject routes from ipBlock1 in default VRF with NHGID: #1.")
	pushDefaultEntries(t, args, subIntfIPs, virtualIPs)

	t.Log("Send and validate traffic after reinjecting routes in ipBlock1 post switchover.")
	testTraffic(t, *args)

	args.ate.OTG().StopProtocols(t)
}
