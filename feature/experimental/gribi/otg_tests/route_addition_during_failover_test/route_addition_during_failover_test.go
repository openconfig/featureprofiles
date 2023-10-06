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

package route_addition_during_failover_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	cmp "github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/featureprofiles/internal/otgutils"
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
	aftspb "github.com/openconfig/gribi/v1/proto/service"
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
	ipv4PrefixLen       = 30
	nhgID               = 1
	controllerCardType  = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD
	primaryController   = oc.Platform_ComponentRedundantRole_PRIMARY
	secondaryController = oc.Platform_ComponentRedundantRole_SECONDARY
	maxSwitchoverTime   = 900
	configNhg           = true
	switchover          = true
)

type flowArgs struct {
	ipBlock          string
	flowStartAddress string
	flowEndAddress   string
	flowCount        uint32
}

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
	vendorCoreFileNamePattern = map[ondatra.Vendor]*regexp.Regexp{
		ondatra.JUNIPER: regexp.MustCompile("rpd.*core*"),
		ondatra.CISCO:   regexp.MustCompile("emsd.*core.*"),
		ondatra.NOKIA:   regexp.MustCompile("coredump-sr_gribi_server-.*"),
		ondatra.ARISTA:  regexp.MustCompile("core.*"),
	}
	fibProgrammedEntries []string
)

var ipBlock2FlowArgs = &flowArgs{
	ipBlock:          "198.18.100.0/22",
	flowStartAddress: "198.18.100.0",
	flowEndAddress:   "198.18.103.255",
	flowCount:        1024,
}

var ipBlock1FlowArgs = &flowArgs{
	ipBlock:          "198.18.196.0/22",
	flowStartAddress: "198.18.196.0",
	flowEndAddress:   "198.18.199.255",
	flowCount:        1024,
}

// coreFileCheck function is used to check if cores are found on the DUT.
func coreFileCheck(t *testing.T, dut *ondatra.DUTDevice, gnoiClient binding.GNOIClients, sysConfigTime uint64, retry bool) {
	t.Helper()
	t.Log("Checking for core files on DUT")

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
		if retry {
			t.Logf("Retry GNOI request to check %v for core files on DUT", vendorCoreFilePath[dut.Vendor()])
			validResponse, err = gnoiClient.File().Stat(context.Background(), in)
		}
	}
	if err != nil {
		t.Fatalf("Unable to stat path %v for core files on DUT, %v", vendorCoreFilePath[dut.Vendor()], err)
	}
	// Check cores creation time is greater than test start time.
	for _, fileStatsInfo := range validResponse.GetStats() {
		if fileStatsInfo.GetLastModified() > sysConfigTime {
			coreFileName := fileStatsInfo.GetPath()
			r := vendorCoreFileNamePattern[dut.Vendor()]
			if r.MatchString(coreFileName) {
				t.Errorf("Found core %v on DUT.", coreFileName)
			}
		}
	}
}

// createIPv4Entries creates IPv4 Entries given the totalCount and starting prefix.
func createIPv4Entries(t *testing.T, startIP string) []string {
	t.Helper()

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

// pushDefaultEntries creates NextHopGroup entries using the 64 SubIntf addresses and creates 1000 IPV4 Entries.
func pushDefaultEntries(t *testing.T, args *testArgs, nextHops, virtualVIPs []string, configNhg, switchover bool) {
	t.Helper()

	if configNhg {
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
	}

	for ip := range virtualVIPs {
		args.client.Modify().AddEntry(t,
			fluent.IPv4Entry().
				WithPrefix(virtualVIPs[ip]+"/32").
				WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
				WithNextHopGroup(uint64(nhgID)).
				WithElectionID(args.electionID.Low, args.electionID.High))
	}

	if err := awaitTimeout(args.ctx, args.client, t, time.Minute); err != nil {
		if switchover {
			t.Logf("Concurrent switchover/gRIBI route addition, some entries might fail to add.")
			t.Logf("Could not program entries via client, got err: %v", err)
		} else {
			t.Fatalf("Could not program entries via client, got err: %v", err)
		}
	}

	if switchover {
		for _, v := range args.client.Results(t) {
			if v.ProgrammingResult == aftspb.AFTResult_FIB_PROGRAMMED {
				fibProgrammedEntries = append(fibProgrammedEntries, v.Details.IPv4Prefix)
			}
		}
	} else {
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
			t.Fatalf("Error encountered during gRIBI Get operation: got: %v", err)
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
}

// pushConfig pushes the configuration to the device using gNMI Replace.
func pushConfig(t *testing.T, dut *ondatra.DUTDevice, dutPort *ondatra.Port, d *oc.Root) {
	t.Helper()

	intfName := dutPort.Name()
	i := d.GetOrCreateInterface(intfName)
	gnmi.Replace(t, dut, gnmi.OC().Interface(intfName).Config(), i)
}

// configureInterfaceDUT configures a single DUT layer 2 port.
func configureInterfaceDUT(t *testing.T, dutPort *ondatra.Port, dut *ondatra.DUTDevice, d *oc.Root, desc string) {
	t.Helper()

	i := d.GetOrCreateInterface(dutPort.Name())
	i.Description = ygot.String(desc)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	if deviations.InterfaceEnabled(dut) {
		i.Enabled = ygot.Bool(true)
	}
	t.Logf("DUT port %s configured", dutPort)
}

// generateSubIntfPair configures ATE/DUT SubInterfaces on the target device and returns
// a slice of the corresponding ATE IPAddresses.
func generateSubIntfPair(t *testing.T, dut *ondatra.DUTDevice, dutPort *ondatra.Port, ate *ondatra.ATEDevice, atePort *ondatra.Port, top gosnappi.Config, d *oc.Root) []string {
	t.Helper()

	nextHops := []string{}
	nextHopCount := 63 // nextHopCount specifies number of nextHop IPs needed.
	top.Ports().Add().SetName(atePort.ID())
	for i := 0; i <= nextHopCount; i++ {
		vlanID := uint16(i) + 1
		name := fmt.Sprintf(`dst%d`, i)
		Index := uint32(i) + 1
		ateIPv4 := fmt.Sprintf(`198.51.100.%d`, ((4 * i) + 1))
		dutIPv4 := fmt.Sprintf(`198.51.100.%d`, ((4 * i) + 2))
		configureSubinterfaceDUT(t, d, dutPort, Index, vlanID, dutIPv4, dut)
		MAC, err := incrementMAC(atePort1.MAC, i+1)
		if err != nil {
			t.Fatalf("Failed to generate mac address; %v", err)
		}
		configureATE(t, top, atePort, vlanID, name, MAC, dutIPv4, ateIPv4)
		nextHops = append(nextHops, ateIPv4)
	}
	if deviations.RequireRoutedSubinterface0(dut) {
		i := d.GetOrCreateInterface(dutPort.Name())
		i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
		s := i.GetOrCreateSubinterface(0).GetOrCreateIpv4()
		s.Enabled = ygot.Bool(true)
	}
	configureInterfaceDUT(t, dutPort, dut, d, "dst")
	pushConfig(t, dut, dutPort, d)
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		intf := d.GetOrCreateInterface(dutPort.Name())
		for i := 1; i <= nextHopCount+1; i++ {
			fptest.AssignToNetworkInstance(t, dut, intf.GetName(), deviations.DefaultNetworkInstance(dut), uint32(i))
		}
	}
	return nextHops
}

// configureSubinterfaceDUT configures a single DUT layer 3 sub-interface.
func configureSubinterfaceDUT(t *testing.T, d *oc.Root, dutPort *ondatra.Port, index uint32, vlanID uint16, dutIPv4 string, dut *ondatra.DUTDevice) {
	t.Helper()

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
func configureATE(t *testing.T, top gosnappi.Config, atePort *ondatra.Port, vlanID uint16, Name, MAC, dutIPv4, ateIPv4 string) {
	dev := top.Devices().Add().SetName(Name + ".Dev")
	eth := dev.Ethernets().Add().SetName(Name + ".Eth").SetMac(MAC)
	eth.Connection().SetChoice(gosnappi.EthernetConnectionChoice.PORT_NAME).SetPortName(atePort.ID())
	if vlanID != 0 {
		eth.Vlans().Add().SetName(Name).SetId(uint32(vlanID))
	}
	eth.Ipv4Addresses().Add().SetName(Name + ".IPv4").SetAddress(ateIPv4).SetGateway(dutIPv4).SetPrefix(uint32(atePort1.IPv4Len))

}

// awaitTimeout calls a fluent client Await, adding a timeout to the context.
func awaitTimeout(ctx context.Context, c *fluent.GRIBIClient, t testing.TB, timeout time.Duration) error {
	t.Helper()

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
func createTrafficFlow(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config, flowArgs *flowArgs, dstMac string) gosnappi.Flow {
	t.Helper()

	top.Flows().Clear().Items()
	flow := top.Flows().Add().SetName("Flow")
	flow.Metrics().SetEnable(true)
	flow.TxRx().Port().SetTxName("port1").SetRxName("port2")
	e1 := flow.Packet().Add().Ethernet()
	e1.Src().SetValue(atePort1.MAC)
	e1.Dst().SetChoice("value").SetValue(dstMac)
	v4 := flow.Packet().Add().Ipv4()
	v4.Src().SetValue(atePort1.IPv4)
	v4.Dst().Increment().SetStart(flowArgs.flowStartAddress).SetCount(flowArgs.flowCount)

	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)

	return flow
}

// findController finds out primary and secondary controllers.
func findController(t *testing.T, dut *ondatra.DUTDevice, controllers []string) (string, string) {
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

// awaitSwitchoverReady is to check if controller is ready for switchover.
func awaitSwitchoverReady(t *testing.T, dut *ondatra.DUTDevice, controller string) {
	t.Helper()

	switchoverReady := gnmi.OC().Component(controller).SwitchoverReady()
	gnmi.Await(t, dut, switchoverReady.State(), 30*time.Minute, true)
}

// validateSwitchoverTelemetry validates telemetry sensors.
func validateSwitchoverTelemetry(t *testing.T, dut *ondatra.DUTDevice, primaryAfterSwitch string) {
	t.Helper()

	t.Log("Validate OC Switchover time/reason.")
	primary := gnmi.OC().Component(primaryAfterSwitch)
	if !gnmi.Lookup(t, dut, primary.LastSwitchoverTime().State()).IsPresent() {
		t.Errorf("Primary.LastSwitchoverTime().Lookup(t).IsPresent(): got false, want true.")
	} else {
		t.Logf("Found primary.LastSwitchoverTime(): %v", gnmi.Get(t, dut, primary.LastSwitchoverTime().State()))
	}

	if !gnmi.Lookup(t, dut, primary.LastSwitchoverReason().State()).IsPresent() {
		t.Errorf("Primary.LastSwitchoverReason().Lookup(t).IsPresent(): got false, want true")
	} else {
		lastSwitchoverReason := gnmi.Get(t, dut, primary.LastSwitchoverReason().State())
		t.Logf("Found lastSwitchoverReason.GetDetails(): %v", lastSwitchoverReason.GetDetails())
		t.Logf("Found lastSwitchoverReason.GetTrigger().String(): %v", lastSwitchoverReason.GetTrigger().String())
	}
}

// testTraffic is to send and validate traffic.
func testTraffic(t *testing.T, args testArgs, flow gosnappi.Flow) {
	t.Helper()

	args.ate.OTG().StartTraffic(t)
	time.Sleep(2 * time.Minute)
	args.ate.OTG().StopTraffic(t)

	otgutils.LogFlowMetrics(t, args.ate.OTG(), args.top)
	txPkts := float32(gnmi.Get(t, args.ate.OTG(), gnmi.OTG().Flow("Flow").Counters().OutPkts().State()))
	rxPkts := float32(gnmi.Get(t, args.ate.OTG(), gnmi.OTG().Flow("Flow").Counters().InPkts().State()))
	if txPkts == 0 {
		t.Fatalf("Tx packets should be higher than 0")
	}

	if got := (txPkts - rxPkts) * 100 / txPkts; got > 0 {
		t.Errorf("LossPct for flow %s got %g, want 0", flow.Name(), got)
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

// TestRouteAdditionDuringFailover is to test gRIBI route addition and slave switchover
// concurrently, validate reinject of gRIBI programmed routes and traffic.
func TestRouteAdditionDuringFailover(t *testing.T) {
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
	configureSubinterfaceDUT(t, d, dp1, 0, 0, dutPort1.IPv4, dut)
	configureInterfaceDUT(t, dp1, dut, d, "src")
	configureATE(t, top, ap1, 0, atePort1.Name, atePort1.MAC, dutPort1.IPv4, atePort1.IPv4)
	pushConfig(t, dut, dp1, d)
	dp2 := dut.Port(t, "port2")
	ap2 := ate.Port(t, "port2")
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

	sysConfigTime := gnmi.Get(t, dut, gnmi.OC().Interface(dut.Port(t, "port1").Name()).LastChange().State())

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

	virtualIPs := createIPv4Entries(t, ipBlock1FlowArgs.ipBlock)

	t.Log("Inject routes from ipBlock1 in default VRF with NHGID: #1.")
	pushDefaultEntries(t, args, subIntfIPs, virtualIPs, configNhg, !switchover)

	gr, err := args.client.Get().
		WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
		WithAFT(fluent.IPv4).
		Send()
	if err != nil {
		t.Fatalf("Error encountered during gRIBI Get operation: got: %v", err)
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
	testTraffic(t, *args, createTrafficFlow(t, args.ate, args.top, ipBlock1FlowArgs, dstMac))

	controllers := cmp.FindComponentsByType(t, dut, controllerCardType)
	t.Logf("Found controller list: %v", controllers)
	// Only perform the switchover for the chassis with dual controllers.
	if len(controllers) != 2 {
		t.Skipf("Dual controllers required on %v: got %v, want 2", dut.Model(), len(controllers))
	}

	secondaryBeforeSwitch, primaryBeforeSwitch := findController(t, dut, controllers)

	awaitSwitchoverReady(t, dut, primaryBeforeSwitch)
	t.Logf("Controller %q is ready for switchover before test.", primaryBeforeSwitch)

	gnoiClient := dut.RawAPIs().GNOI(t)
	useNameOnly := deviations.GNOISubcomponentPath(dut)
	switchoverRequest := &spb.SwitchControlProcessorRequest{
		ControlProcessor: cmp.GetSubcomponentPath(secondaryBeforeSwitch, useNameOnly),
	}
	t.Logf("switchoverRequest: %v", switchoverRequest)

	// Concurrently run switchover and gribi route addition in ipBlock2.
	virtualIPsBlock2 := createIPv4Entries(t, ipBlock2FlowArgs.ipBlock)

	// Check for coredumps in the DUT and validate that none are present on DUT before switchover.
	coreFileCheck(t, dut, gnoiClient, sysConfigTime, false)

	wg := new(sync.WaitGroup)
	wg.Add(2)
	t.Log("Execute gRIBi route addition and master switchover concurrently.")
	go func() {
		defer wg.Done()
		t.Log("Inject routes from ipBlock2 in default VRF with NHGID: #1.")
		pushDefaultEntries(t, args, subIntfIPs, virtualIPsBlock2, !configNhg, switchover)
	}()

	go func() {
		defer wg.Done()
		switchoverResponse, err := gnoiClient.System().SwitchControlProcessor(context.Background(), switchoverRequest)
		if err != nil {
			t.Logf("Failed to perform control processor switchover with unexpected err: %v", err)
		}
		t.Logf("gnoiClient.System().SwitchControlProcessor() response: %v, err: %v", switchoverResponse, err)
	}()
	wg.Wait()
	t.Log("Concurrent switchover and route addition is completed, validate switchoverStatus now.")

	primaryAfterSwitch := validateSwitchoverStatus(t, dut, secondaryBeforeSwitch)

	validateSwitchoverTelemetry(t, dut, primaryAfterSwitch)

	// Following reconnection of the gRIBI client to a new master supervisor,
	// validate if partially deleted entries of ipBlock1  are not present in the FIB
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

	t.Log("Validate if partially ACKed entries of IPBlock2 are present as FIB_PROGRAMMED using a get RPC.")

	gr, err = args.client.Get().
		WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
		WithAFT(fluent.IPv4).
		Send()
	if err != nil {
		t.Fatalf("Error encountered during gRIBI Get operation, got:%v", err)
	}
	if len(fibProgrammedEntries) != 0 {
		for _, ipv4Prefix := range fibProgrammedEntries {
			for _, r := range gr.GetEntry() {
				if r.GetIpv4().Prefix == ipv4Prefix {
					if got := r.GetFibStatus().String(); got != "PROGRAMMED" {
						t.Fatalf("gRIBI entry %v is not programmed in FIB, got:%v, want: PROGRAMMED", r.GetIpv4().Prefix, got)
					}
					continue
				}
			}
		}
	}

	// Check for coredumps in the DUT and validate that none are present post failover.
	// Set retry to true since gnoi connection may be broken after switchover.
	coreFileCheck(t, dut, gnoiClient, sysConfigTime, true)

	t.Log("Re-inject routes from ipBlock2 in default VRF with NHGID: #1.")
	pushDefaultEntries(t, args, subIntfIPs, virtualIPsBlock2, !configNhg, !switchover)

	// Send traffic to ipBlock1, ipBlock2.
	t.Log("Send and validate traffic to ipBlock1 ipv4 entries.")
	testTraffic(t, *args, createTrafficFlow(t, args.ate, args.top, ipBlock1FlowArgs, dstMac))

	t.Log("Send and validate traffic to ipBlock2 ipv4 entries.")
	testTraffic(t, *args, createTrafficFlow(t, args.ate, args.top, ipBlock2FlowArgs, dstMac))
	ate.OTG().StopProtocols(t)
}
