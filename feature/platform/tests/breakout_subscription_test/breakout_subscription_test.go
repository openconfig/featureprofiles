// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package breakout_subscription_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/args"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/helpers"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	spb "github.com/openconfig/gnoi/system"
	tpb "github.com/openconfig/gnoi/types"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Settings for configuring the aggregate testbed with the test
// topology.  IxNetwork flow requires both source and destination
// networks be configured on the ATE.  It is not possible to send
// packets to the ether.
//
// The testbed consists of ate:port{1-2} -> dut:port{1-2} and dut:port3 ->
// ate:port3.  The first pair is called the "source" aggregatepair, and the
// second  link the "destination" pair.
//
//   * Source: ate:port{1-2} -> dut:port{1-2} subnet 192.0.2.0/30 2001:db8::0/126
//   * Destination: dut:port3 -> ate:port3
//     subnet 192.0.2.4/30 2001:db8::4/126
//
// Note that the first (.0, .4) and last (.3, .7) IPv4 addresses are
// reserved from the subnet for broadcast, so a /30 leaves exactly 2
// usable addresses.  This does not apply to IPv6 which allows /127
// for point to point links, but we use /126 so the numbering is
// consistent with IPv4.
//

const (
	syncResponseWaitTimeOut = 300 * time.Second
)

var (
	telemetryPaths = []ygnmi.PathStruct{
		gnmi.OC().InterfaceAny().AdminStatus().State().PathStruct(),
		gnmi.OC().InterfaceAny().OperStatus().State().PathStruct(),
		gnmi.OC().InterfaceAny().Id().State().PathStruct(),
		gnmi.OC().InterfaceAny().HardwarePort().State().PathStruct(),
		gnmi.OC().InterfaceAny().Ethernet().MacAddress().State().PathStruct(),
		gnmi.OC().InterfaceAny().Ethernet().PortSpeed().State().PathStruct(),
		gnmi.OC().InterfaceAny().ForwardingViable().State().PathStruct(),
		gnmi.OC().Lacp().InterfaceAny().MemberAny().Interface().State().PathStruct(),
		gnmi.OC().ComponentAny().IntegratedCircuit().NodeId().State().PathStruct(),
		gnmi.OC().ComponentAny().Parent().State().PathStruct(),
		gnmi.OC().ComponentAny().OperStatus().State().PathStruct(),
		gnmi.OC().ComponentAny().Name().State().PathStruct(),
	}
)

const (
	plen4          = 30
	plen6          = 126
	opUp           = oc.Interface_OperStatus_UP
	ethernetCsmacd = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	ieee8023adLag  = oc.IETFInterfaces_InterfaceType_ieee8023adLag
)

var (
	dutSrc = attrs.Attributes{
		Desc:    "dutsrc",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::1",
		IPv4Len: plen4,
		IPv6Len: plen6,
	}

	ateSrc = attrs.Attributes{
		Name:    "atesrc",
		MAC:     "02:11:01:00:00:01",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::2",
		IPv4Len: plen4,
		IPv6Len: plen6,
	}

	dutDst = attrs.Attributes{
		Desc:    "dutdst",
		IPv4:    "192.0.2.5",
		IPv6:    "2001:db8::5",
		IPv4Len: plen4,
		IPv6Len: plen6,
	}

	ateDst = attrs.Attributes{
		Name:    "atedst",
		MAC:     "02:12:01:00:00:01",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::6",
		IPv4Len: plen4,
		IPv6Len: plen6,
	}
	maxCompWaitTime uint64 = 600
)

const (
	lagTypeLACP = oc.IfAggregate_AggregationType_LACP
)

type testCase struct {
	dut     *ondatra.DUTDevice
	ate     *ondatra.ATEDevice
	top     gosnappi.Config
	lagType oc.E_IfAggregate_AggregationType

	dutPorts []*ondatra.Port
	atePorts []*ondatra.Port
	aggID    string
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// createSubscriptionList creates a subscription list for the given telemetry OC paths
func createSubscriptionList(t *testing.T, telemetryData []ygnmi.PathStruct) *gpb.SubscriptionList {
	subscriptions := make([]*gpb.Subscription, 0)
	for _, path := range telemetryData {
		gnmiPath, _, err := ygnmi.ResolvePath(path)
		if err != nil {
			t.Errorf("[Error]:Error in resolving gnmi path =%v", path)
		}
		gnmiRequest := &gpb.Subscription{
			Path: gnmiPath,
			Mode: gpb.SubscriptionMode_ON_CHANGE,
		}
		subscriptions = append(subscriptions, gnmiRequest)
	}
	return &gpb.SubscriptionList{
		Subscription: subscriptions,
		Mode:         gpb.SubscriptionList_STREAM,
	}
}

// incrementMAC uses a mac string and increments it by the given i
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

// configureATE configures the ATE with the testbed topology.
func (tc *testCase) configureATE(t *testing.T) {
	p0 := tc.atePorts[2]
	tc.top.Ports().Add().SetName(p0.ID())
	d0 := tc.top.Devices().Add().SetName(ateDst.Name)
	srcEth := d0.Ethernets().Add().SetName(ateDst.Name + ".Eth").SetMac(ateDst.MAC)
	srcEth.Connection().SetPortName(p0.ID())
	srcEth.Ipv4Addresses().Add().SetName(ateDst.Name + ".IPv4").SetAddress(ateDst.IPv4).SetGateway(dutDst.IPv4).SetPrefix(uint32(ateDst.IPv4Len))
	srcEth.Ipv6Addresses().Add().SetName(ateDst.Name + ".IPv6").SetAddress(ateDst.IPv6).SetGateway(dutDst.IPv6).SetPrefix(uint32(ateDst.IPv6Len))

	agg := tc.top.Lags().Add().SetName("LAG")
	for i, p := range tc.atePorts[0:1] {
		port := tc.top.Ports().Add().SetName(p.ID())
		lagPort := agg.Ports().Add()
		newMac, err := incrementMAC(ateSrc.MAC, i+1)
		if err != nil {
			t.Fatal(err)
		}
		lagPort.SetPortName(port.Name()).
			Ethernet().SetMac(newMac).
			SetName("LAGRx-" + strconv.Itoa(i))
		lagPort.Lacp().SetActorPortNumber(uint32(i + 1)).SetActorPortPriority(1).SetActorActivity("active")
	}
	agg.Protocol().Lacp().SetActorKey(1).SetActorSystemPriority(1).SetActorSystemId("01:01:01:01:01:01")

	// Gather only the ports actively added to the topology
	activePorts := []*ondatra.Port{tc.atePorts[2], tc.atePorts[0]}

	// Disable FEC for active 100G-FR ports because Novus does not support it.
	var p100gbasefr []string
	for _, p := range activePorts {
		if p.PMD() == ondatra.PMD100GBASEFR {
			p100gbasefr = append(p100gbasefr, p.ID())
		}
	}

	if len(p100gbasefr) > 0 {
		l1Settings := tc.top.Layer1().Add().SetName("L1").SetPortNames(p100gbasefr)
		l1Settings.SetAutoNegotiate(true).SetIeeeMediaDefaults(false).SetSpeed("speed_100_gbps")
		autoNegotiate := l1Settings.AutoNegotiation()
		autoNegotiate.SetRsFec(false)
	}

	dstDev := tc.top.Devices().Add().SetName(agg.Name() + ".dev")
	dstEth := dstDev.Ethernets().Add().SetName(ateSrc.Name + ".Eth").SetMac(ateSrc.MAC)
	dstEth.Connection().SetLagName(agg.Name())
	dstEth.Ipv4Addresses().Add().SetName(ateSrc.Name + ".IPv4").SetAddress(ateSrc.IPv4).SetGateway(dutSrc.IPv4).SetPrefix(uint32(ateSrc.IPv4Len))
	dstEth.Ipv6Addresses().Add().SetName(ateSrc.Name + ".IPv6").SetAddress(ateSrc.IPv6).SetGateway(dutSrc.IPv6).SetPrefix(uint32(ateSrc.IPv6Len))

	tc.ate.OTG().PushConfig(t, tc.top)
	tc.ate.OTG().StartProtocols(t)
}

// clearAggregateMembers clears the aggregate members of the DUT.
func (tc *testCase) clearAggregateMembers(t *testing.T) {
	for n, port := range tc.dutPorts {
		if n < 1 {
			// We designate port 0 as the source link, not part of LAG.
			continue
		}
		gnmi.Delete(t, tc.dut, gnmi.OC().Interface(port.Name()).Ethernet().AggregateId().Config())
	}
}

func (tc *testCase) setupAggregateAtomically(t *testing.T) {
	d := &oc.Root{}

	if tc.lagType == lagTypeLACP {
		d.GetOrCreateLacp().GetOrCreateInterface(tc.aggID)
	}
	agg := d.GetOrCreateInterface(tc.aggID)
	agg.GetOrCreateAggregation().LagType = tc.lagType
	agg.Type = ieee8023adLag
	for n, port := range tc.dutPorts {
		if n < 1 {
			// We designate port 0 as the source link, not part of LAG.
			continue
		}
		i := d.GetOrCreateInterface(port.Name())
		i.GetOrCreateEthernet().AggregateId = ygot.String(tc.aggID)
		i.Type = ethernetCsmacd

		if deviations.InterfaceEnabled(tc.dut) {
			i.Enabled = ygot.Bool(true)
		}
	}
	p := gnmi.OC()
	fptest.LogQuery(t, fmt.Sprintf("%s to Update()", tc.dut), p.Config(), d)
	gnmi.Update(t, tc.dut, p.Config(), d)
}

func (tc *testCase) configSrcAggregateDUT(i *oc.Interface, a *attrs.Attributes) {
	tc.configDstDUT(i, a)
	i.Type = ieee8023adLag
	g := i.GetOrCreateAggregation()
	g.LagType = tc.lagType
}

func (tc *testCase) configSrcMemberDUT(i *oc.Interface, p *ondatra.Port) {
	i.Description = ygot.String(p.String())
	i.Type = ethernetCsmacd
	if deviations.InterfaceEnabled(tc.dut) {
		i.Enabled = ygot.Bool(true)
	}

	e := i.GetOrCreateEthernet()
	e.AggregateId = ygot.String(tc.aggID)
}

func (tc *testCase) configDstDUT(i *oc.Interface, a *attrs.Attributes) {
	i.Description = ygot.String(a.Desc)
	if deviations.InterfaceEnabled(tc.dut) {
		i.Enabled = ygot.Bool(true)
	}

	s := i.GetOrCreateSubinterface(0)
	s4 := s.GetOrCreateIpv4()
	if deviations.InterfaceEnabled(tc.dut) && !deviations.IPv4MissingEnabled(tc.dut) {
		s4.Enabled = ygot.Bool(true)
	}
	a4 := s4.GetOrCreateAddress(a.IPv4)
	a4.PrefixLength = ygot.Uint8(plen4)

	s6 := s.GetOrCreateIpv6()
	if deviations.InterfaceEnabled(tc.dut) {
		s6.Enabled = ygot.Bool(true)
	}
	s6.GetOrCreateAddress(a.IPv6).PrefixLength = ygot.Uint8(plen6)
}

func (tc *testCase) configureDUT(t *testing.T) {
	t.Logf("dut ports = %v", tc.dutPorts)
	d := gnmi.OC()

	if deviations.AggregateAtomicUpdate(tc.dut) {
		tc.clearAggregateMembers(t)
		tc.setupAggregateAtomically(t)

		t.Logf("Waiting 15 seconds for the control plane to settle after atomic aggregate setup...")
		time.Sleep(15 * time.Second)
	}

	// Configure LACP SECOND
	if tc.lagType == lagTypeLACP {
		lacp := &oc.Lacp_Interface{Name: ygot.String(tc.aggID)}
		lacp.LacpMode = oc.Lacp_LacpActivityType_ACTIVE

		lacpPath := d.Lacp().Interface(tc.aggID)
		fptest.LogQuery(t, "LACP", lacpPath.Config(), lacp)
		gnmi.Replace(t, tc.dut, lacpPath.Config(), lacp)
	}

	// Create the Aggregate Interface FIRST
	agg := &oc.Interface{Name: ygot.String(tc.aggID)}
	tc.configSrcAggregateDUT(agg, &dutSrc)
	aggPath := d.Interface(tc.aggID)
	fptest.LogQuery(t, tc.aggID, aggPath.Config(), agg)
	gnmi.Replace(t, tc.dut, aggPath.Config(), agg)

	time.Sleep(5 * time.Second)

	// Configure the destination port
	dstp := tc.dutPorts[2]
	dsti := &oc.Interface{Name: ygot.String(dstp.Name())}
	tc.configDstDUT(dsti, &dutDst)
	dsti.Type = ethernetCsmacd
	dstiPath := d.Interface(dstp.Name())
	fptest.LogQuery(t, dstp.String(), dstiPath.Config(), dsti)
	gnmi.Replace(t, tc.dut, dstiPath.Config(), dsti)
	if deviations.ExplicitInterfaceInDefaultVRF(tc.dut) {
		fptest.AssignToNetworkInstance(t, tc.dut, dstp.Name(), deviations.DefaultNetworkInstance(tc.dut), 0)
		fptest.AssignToNetworkInstance(t, tc.dut, tc.aggID, deviations.DefaultNetworkInstance(tc.dut), 0)
	}

	// Configure the member ports
	for _, port := range tc.dutPorts[0:1] {
		i := &oc.Interface{Name: ygot.String(port.Name())}
		tc.configSrcMemberDUT(i, port)
		iPath := d.Interface(port.Name())
		fptest.LogQuery(t, port.String(), iPath.Config(), i)
		gnmi.Replace(t, tc.dut, iPath.Config(), i)
	}

	if deviations.ExplicitPortSpeed(tc.dut) {
		for _, port := range tc.dutPorts {
			fptest.SetPortSpeed(t, port)
		}
	}
}

// sortPorts sorts the ports by the testbed port ID.
func sortPorts(ports []*ondatra.Port) []*ondatra.Port {
	sort.SliceStable(ports, func(i, j int) bool {
		return ports[i].ID() < ports[j].ID()
	})
	return ports
}

func (tc *testCase) verifyDUT(t *testing.T) {
	// Wait for LAG negotiation and verify LAG type for the aggregate interface.
	gnmi.Await(t, tc.dut, gnmi.OC().Interface(tc.aggID).Type().State(), time.Minute, ieee8023adLag)
	for _, port := range tc.dutPorts {
		path := gnmi.OC().Interface(port.Name())
		gnmi.Await(t, tc.dut, path.OperStatus().State(), time.Minute, oc.Interface_OperStatus_UP)
	}
}

// LinecardReboot performs a linecard reboot.
func LinecardReboot(t *testing.T, dut *ondatra.DUTDevice) {
	const linecardBoottime = 20 * time.Minute
	lcs := components.FindComponentsByType(t, dut, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_LINECARD)
	t.Logf("Found linecard list: %v", lcs)

	var validCards []string
	// don't consider the empty linecard slots.
	if len(lcs) > *args.NumLinecards {
		for _, lc := range lcs {
			empty, ok := gnmi.Lookup(t, dut, gnmi.OC().Component(lc).Empty().State()).Val()
			if !ok || (ok && !empty) {
				validCards = append(validCards, lc)
			}
		}
	} else {
		validCards = lcs
	}
	if *args.NumLinecards >= 0 && len(validCards) < *args.NumLinecards {
		t.Errorf("Incorrect number of linecards: got %v, want at least %v (specified by flag)", len(validCards), *args.NumLinecards)
	}

	if got := len(validCards); got == 0 {
		t.Skipf("Not enough linecards for the test on %v: got %v, want > 0", dut.Model(), got)
	}

	var removableLinecard string
	for _, lc := range validCards {
		t.Logf("Check if %s is removable", lc)
		if got := gnmi.Lookup(t, dut, gnmi.OC().Component(lc).Removable().State()).IsPresent(); !got {
			t.Logf("Detected non-removable line card: %v", lc)
			continue
		}
		if got := gnmi.Get(t, dut, gnmi.OC().Component(lc).Removable().State()); got {
			t.Logf("Found removable line card: %v", lc)
			removableLinecard = lc
		}
	}
	if removableLinecard == "" {
		if *args.NumLinecards > 0 {
			t.Fatalf("No removable line card found for the testing on a modular device")
		} else {
			t.Skipf("No removable line card found for the testing")
		}
	}

	gnoiClient := dut.RawAPIs().GNOI(t)
	useNameOnly := deviations.GNOISubcomponentPath(dut)
	rebootSubComponentRequest := &spb.RebootRequest{
		Method: spb.RebootMethod_COLD,
		Subcomponents: []*tpb.Path{
			components.GetSubcomponentPath(removableLinecard, useNameOnly),
		},
	}
	intfsOperStatusUPBeforeReboot := helpers.FetchOperStatusUPIntfs(t, dut, *args.CheckInterfacesInBinding)
	t.Logf("OperStatusUP interfaces before reboot: %v", intfsOperStatusUPBeforeReboot)

	t.Logf("rebootSubComponentRequest: %v", rebootSubComponentRequest)
	rebootResponse, err := gnoiClient.System().Reboot(context.Background(), rebootSubComponentRequest)
	if err != nil {
		t.Fatalf("Failed to perform line card reboot with unexpected err: %v", err)
	}
	t.Logf("gnoiClient.System().Reboot() response: %v, err: %v", rebootResponse, err)

	t.Logf("Wait for 10s to allow the subcomponent's reboot process to start")
	time.Sleep(10 * time.Second)

	req := &spb.RebootStatusRequest{
		Subcomponents: rebootSubComponentRequest.GetSubcomponents(),
	}

	if deviations.GNOISubcomponentRebootStatusUnsupported(dut) {
		req.Subcomponents = nil
	}
	rebootDeadline := time.Now().Add(linecardBoottime)
	for retry := true; retry; {
		t.Log("Waiting for 10 seconds before checking.")
		time.Sleep(10 * time.Second)
		if time.Now().After(rebootDeadline) {
			retry = false
			break
		}
		resp, err := gnoiClient.System().RebootStatus(context.Background(), req)
		switch {
		case status.Code(err) == codes.Unimplemented:
			t.Fatalf("Unimplemented RebootStatus() is not fully compliant with the Reboot spec.")
		case err == nil:
			retry = resp.GetActive()
		default:
			// any other error just sleep.
		}
	}

	t.Logf("Validate removable linecard %v status", removableLinecard)
	gnmi.Await(t, dut, gnmi.OC().Component(removableLinecard).Removable().State(), linecardBoottime, true)

	helpers.ValidateOperStatusUPIntfs(t, dut, intfsOperStatusUPBeforeReboot, 10*time.Minute)
}

// chassisReboot performs a chassis reboot.
func chassisReboot(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	maxRebootTime := uint64(20 * time.Minute.Seconds())
	preRebootCompStatus := gnmi.GetAll(t, dut, gnmi.OC().ComponentAny().OperStatus().State())
	t.Logf("DUT components status pre reboot: %v", preRebootCompStatus)

	preRebootCompDebug := gnmi.GetAll(t, dut, gnmi.OC().ComponentAny().State())
	var preCompMatrix []string
	for _, preComp := range preRebootCompDebug {
		if preComp.GetOperStatus() != oc.PlatformTypes_COMPONENT_OPER_STATUS_UNSET {
			preCompMatrix = append(preCompMatrix, preComp.GetName()+":"+preComp.GetOperStatus().String())
		}
	}
	gnoiClient := dut.RawAPIs().GNOI(t)
	rebootRequest := &spb.RebootRequest{
		Method: spb.RebootMethod_COLD,
	}
	t.Logf("rebootRequest: %v", rebootRequest)
	rebootResponse, err := gnoiClient.System().Reboot(context.Background(), rebootRequest)
	if err != nil {
		t.Fatalf("Failed to perform chassis reboot with unexpected err: %v", err)
	}
	t.Logf("gnoiClient.System().Reboot() response: %v, err: %v", rebootResponse, err)
	t.Logf("Wait for 10s to allow the chassis reboot process to start")
	time.Sleep(10 * time.Second)
	req := &spb.RebootStatusRequest{}
	if deviations.GNOISubcomponentRebootStatusUnsupported(dut) {
		req.Subcomponents = nil
	}
	startReboot := time.Now()
	t.Logf("Wait for DUT to boot up by polling the telemetry output.")
	for {
		var currentTime string
		t.Logf("Time elapsed %.2f seconds since reboot started.", time.Since(startReboot).Seconds())
		time.Sleep(30 * time.Second)
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			currentTime = gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State())
		}); errMsg != nil {
			t.Logf("Got testt.CaptureFatal errMsg: %s, keep polling ...", *errMsg)
		} else {
			t.Logf("Device rebooted successfully with received time: %v", currentTime)
			break
		}

		if uint64(time.Since(startReboot).Seconds()) > maxRebootTime {
			t.Errorf("Check boot time: got %v, want < %v", time.Since(startReboot), maxRebootTime)
		}
	}
	t.Logf("Device boot time: %.2f seconds", time.Since(startReboot).Seconds())
	startComp := time.Now()

	for {
		postRebootCompStatus := gnmi.GetAll(t, dut, gnmi.OC().ComponentAny().OperStatus().State())
		postRebootCompDebug := gnmi.GetAll(t, dut, gnmi.OC().ComponentAny().State())
		var postCompMatrix []string
		for _, postComp := range postRebootCompDebug {
			if postComp.GetOperStatus() != oc.PlatformTypes_COMPONENT_OPER_STATUS_UNSET {
				postCompMatrix = append(postCompMatrix, postComp.GetName()+":"+postComp.GetOperStatus().String())
			}
		}

		if len(preRebootCompStatus) == len(postRebootCompStatus) {
			t.Logf("All components on the DUT are in responsive state")
			time.Sleep(10 * time.Second)
			break
		}

		if uint64(time.Since(startComp).Seconds()) > maxCompWaitTime {
			t.Logf("DUT components status post reboot: %v", postRebootCompStatus)
			if rebootDiff := cmp.Diff(preCompMatrix, postCompMatrix); rebootDiff != "" {
				t.Logf("[DEBUG] Unexpected diff after reboot (-component missing from pre reboot, +component added from pre reboot): %v ", rebootDiff)
			}
			t.Fatalf("There's a difference in components obtained in pre reboot: %v and post reboot: %v.", len(preRebootCompStatus), len(postRebootCompStatus))
		}
		time.Sleep(10 * time.Second)
	}
}

// startStreamReader creates a single, dedicated goroutine to safely read from the gRPC stream.
// It pipes all incoming updates into a buffered channel to prevent concurrency issues.
func startStreamReader(t testing.TB, stream gpb.GNMI_SubscribeClient) <-chan *gpb.Notification {
	notifCh := make(chan *gpb.Notification, 100000)
	go func() {
		defer close(notifCh)
		for {
			respUpdate, err := stream.Recv()
			if err != nil {
				log.Printf("Stream reader terminated (expected at end of test): %v", err)
				return
			}
			if respUpdate != nil && respUpdate.GetUpdate() != nil {
				if n, ok := respUpdate.GetResponse().(*gpb.SubscribeResponse_Update); ok {
					notifCh <- n.Update
				}
			}
		}
	}()
	return notifCh
}

// drainChannel empties the notification buffer of any background noise
// before we trigger a specific action.
func drainChannel(ch <-chan *gpb.Notification) {
	for {
		select {
		case <-ch:
		default:
			return
		}
	}
}

// recieveUpdateWithTimeout reads safely from the notification channel until the timeout expires.
func recieveUpdateWithTimeout(ctx context.Context, t *testing.T, notifCh <-chan *gpb.Notification, updateTimeout time.Duration) ([]*gpb.Notification, error) {
	t.Helper()
	ctxTimeout, cancelTimeout := context.WithTimeout(ctx, updateTimeout)
	defer cancelTimeout()

	var notifications []*gpb.Notification

	for {
		select {
		case n, ok := <-notifCh:
			if !ok {
				return notifications, fmt.Errorf("notification channel closed")
			}
			notifications = append(notifications, n)
		case <-ctxTimeout.Done():
			return notifications, ctxTimeout.Err()
		}
	}
}

// newSubscribeRequest is a function that creates a subscribe request to the DUT.
func newSubscribeRequest(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice) gpb.GNMI_SubscribeClient {
	subscribeList := createSubscriptionList(t, telemetryPaths)

	subscribeRequest := &gpb.SubscribeRequest{
		Request: &gpb.SubscribeRequest_Subscribe{
			Subscribe: subscribeList,
		},
	}
	stream, err := dut.RawAPIs().GNMI(t).Subscribe(ctx)
	if err != nil {
		t.Fatalf("[Fail]:Failed to create subscribe stream: %v", err)
	}

	if err := stream.Send(subscribeRequest); err != nil {
		t.Fatalf("[Fail]:Failed to send subscribe request: %v", err)
	}
	return stream
}

// setDUTInterfaceState sets the admin state on the dut interface
func setDUTInterfaceWithState(t testing.TB, dut *ondatra.DUTDevice, dutPort *ondatra.Port, state bool) {
	dc := gnmi.OC()
	i := &oc.Interface{}
	i.Enabled = ygot.Bool(state)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	i.Name = ygot.String(dutPort.Name())
	gnmi.Update(t, dut, dc.Interface(dutPort.Name()).Config(), i)
}

// checkSyncResponse is a function that checks if the DUT has sent the sync response.
func checkSyncResponse(t *testing.T, stream gpb.GNMI_SubscribeClient) {
	startTime := time.Now()
	for {
		resp, err := stream.Recv()
		if resp.GetSyncResponse() == true {
			t.Logf("Received sync_response!")
			break
		}
		if err != nil {
			t.Errorf("[Error]: While receieving the subcription response %v", err)
		}

		if time.Since(startTime).Seconds() > float64(syncResponseWaitTimeOut) {
			t.Fatalf("[Fail]:Didn't receive sync_response. Time limit = %v  exceeded", syncResponseWaitTimeOut)
		}
	}
}

// verifyNotificationPaths verifies the paths in the notifications, accounting for parent-level grouping.
func verifyNotificationPaths(t *testing.T, notifications []*gpb.Notification, expectedUpdatePaths []string) {
	t.Helper()
	var paths []string

	for _, notification := range notifications {
		prefixPath := ""
		if notification.GetPrefix() != nil {
			for _, elem := range notification.GetPrefix().GetElem() {
				prefixPath += "/" + elem.GetName()
			}
		}

		for _, update := range notification.GetUpdate() {
			fullPath := prefixPath
			if update.GetPath() != nil {
				for _, elem := range update.GetPath().GetElem() {
					fullPath += "/" + elem.GetName()
				}
			}
			paths = append(paths, fullPath)
		}
	}

	type pathResult struct {
		path  string
		found bool
	}
	var pathResults []pathResult
	for _, expectedPath := range expectedUpdatePaths {
		pathResults = append(pathResults, pathResult{path: expectedPath, found: false})
	}

	for i, pathResult := range pathResults {
		for _, p := range paths {
			// Check for exact match OR if the device grouped the update at a parent node
			// Example: expected "/.../state/admin-status" matches received "/.../state"
			if p == pathResult.path || strings.HasPrefix(pathResult.path, p+"/") {
				t.Logf("Expected path %v found (Matched against received path: %v)", pathResult.path, p)
				pathResults[i].found = true
				break
			}
		}
	}

	// Deduplicate received paths for cleaner error logging
	uniquePaths := make(map[string]bool)
	for _, p := range paths {
		uniquePaths[p] = true
	}
	var sample []string
	for p := range uniquePaths {
		sample = append(sample, p)
	}

	for _, pathResult := range pathResults {
		if !pathResult.found {
			t.Errorf("Error: Expected path %v not found in received updates. Extracted unique paths: %v", pathResult.path, sample)
		}
	}
}

// verifyUpdateValue verifies the value of the leaf, extracting it from JSON objects if grouped.
func verifyUpdateValue(t testing.TB, notifications []*gpb.Notification, dut *ondatra.DUTDevice, actionState string) {
	t.Helper()
	var expectedUpdateValue []string
	if actionState == "UP" {
		expectedUpdateValue = []string{"UP"}
	} else if actionState == "DOWN" {
		switch dut.Vendor() {
		case ondatra.ARISTA, ondatra.JUNIPER:
			expectedUpdateValue = []string{"LOWER_LAYER_DOWN", "DOWN"}
		default:
			expectedUpdateValue = []string{"DOWN"}
		}
	}

	for _, notification := range notifications {
		for _, update := range notification.GetUpdate() {
			var allElems []*gpb.PathElem
			if notification.GetPrefix() != nil {
				allElems = append(allElems, notification.GetPrefix().GetElem()...)
			}
			if update.GetPath() != nil {
				allElems = append(allElems, update.GetPath().GetElem()...)
			}

			leafName := ""
			if len(allElems) > 0 {
				leafName = allElems[len(allElems)-1].GetName()
			}

			jsonBytes := update.GetVal().GetJsonVal()
			if jsonBytes == nil {
				continue
			}

			var val interface{}
			if err := json.Unmarshal(jsonBytes, &val); err != nil {
				continue
			}

			// Case 1: The update is bundled in a parent JSON object
			if m, ok := val.(map[string]interface{}); ok {
				if adminStatus, exists := m["admin-status"]; exists {
					adminStr := fmt.Sprintf("%v", adminStatus)
					if slices.Contains(expectedUpdateValue, adminStr) {
						t.Logf("Found grouped admin-status: %v", adminStr)
					} else {
						t.Errorf("Error: Grouped admin-status: %v, want: %v", adminStr, expectedUpdateValue)
					}
				}
				if operStatus, exists := m["oper-status"]; exists {
					operStr := fmt.Sprintf("%v", operStatus)
					if slices.Contains(expectedUpdateValue, operStr) {
						t.Logf("Found grouped oper-status: %v", operStr)
					} else {
						t.Errorf("Error: Grouped oper-status: %v, want: %v", operStr, expectedUpdateValue)
					}
				}
			}

			// Case 2: The update was sent directly to the exact leaf path
			if leafName == "admin-status" || leafName == "oper-status" {
				valStr := ""
				if s, ok := val.(string); ok {
					valStr = s
				} else {
					valStr = fmt.Sprintf("%v", val)
				}

				if slices.Contains(expectedUpdateValue, valStr) {
					t.Logf("Found direct %s: %v", leafName, valStr)
				} else {
					t.Errorf("Error: Direct %s: %v, want: %v", leafName, valStr, expectedUpdateValue)
				}
			}
		}
	}
}

func lineCardUp(t testing.TB, dut *ondatra.DUTDevice, fpc string) {
	c := gnmi.OC().Component(fpc)
	config := c.Linecard().PowerAdminState().Config()
	t.Logf("Starting %s POWER_ENABLED", fpc)
	start := time.Now()
	gnmi.Replace(t, dut, config, oc.Platform_ComponentPowerType_POWER_ENABLED)
	oper, ok := gnmi.Await(t, dut, c.OperStatus().State(), 20*time.Minute, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE).Val()
	if !ok {
		t.Errorf("Component %s oper-status after POWER_ENABLED, got: %v, want: %v", fpc, oper, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE)
	}
	t.Logf("Component %s, oper-status after %f minutes: %v", fpc, time.Since(start).Minutes(), oper)
}

// findLinecardFromPort dynamically finds the parent line card component for a given list of ports
// by traversing the OpenConfig component hierarchy via gNMI.
func findLinecardFromPort(t testing.TB, portArray []string, dut *ondatra.DUTDevice) ([]string, error) {
	t.Helper()
	var linecardArray []string

	for _, portName := range portArray {
		// 1. Determine the starting component for the port.
		compName := portName
		if hwPort, present := gnmi.Lookup(t, dut, gnmi.OC().Interface(portName).HardwarePort().State()).Val(); present {
			compName = hwPort
		}

		parentName := compName
		found := false

		// 2. Traverse the component tree upwards until a LINECARD is found.
		for parentName != "" {
			comp, present := gnmi.Lookup(t, dut, gnmi.OC().Component(parentName).State()).Val()
			if !present {
				break
			}

			// Check if the current component is a Linecard
			if comp.GetType() == oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_LINECARD {
				linecardArray = append(linecardArray, parentName)
				found = true
				break
			}

			// Move up to the parent component
			parentName = comp.GetParent()
		}
		if !found {
			return nil, fmt.Errorf("could not find parent line card component for port: %s", portName)
		}
	}
	return linecardArray, nil
}

func verifyNotificationPathsForPortUpdates(t *testing.T, notifications []*gpb.Notification, selectedFpc string) {
	t.Helper()
	for _, notification := range notifications {
		// Check Prefix
		for _, elem := range notification.GetPrefix().GetElem() {
			for key, value := range elem.GetKey() {
				if value == selectedFpc {
					t.Logf("Notification prefix path has update for part: %v. Key: %v, Value: %v", selectedFpc, key, value)
					return
				}
			}
		}
		// Check Update paths
		for _, update := range notification.GetUpdate() {
			for _, elem := range update.GetPath().GetElem() {
				for key, value := range elem.GetKey() {
					if value == selectedFpc {
						t.Logf("Notification update path has update for part: %v. Key: %v, Value: %v", selectedFpc, key, value)
						return
					}
				}
			}
		}
	}
	t.Errorf("Notification is missing update for part: %v", selectedFpc)
}

func linecardDown(t testing.TB, dut *ondatra.DUTDevice, fpc string, lcs []string) {
	var validCards []string
	// don't consider the empty linecard slots.
	if len(lcs) > *args.NumLinecards {
		for _, lc := range lcs {
			t.Logf("lc: %v", lc)
			empty, ok := gnmi.Lookup(t, dut, gnmi.OC().Component(lc).Empty().State()).Val()
			if !ok || (ok && !empty) {
				validCards = append(validCards, lc)
			}
		}
	} else {
		validCards = lcs
	}
	if *args.NumLinecards >= 0 && len(validCards) < *args.NumLinecards {
		t.Errorf("Incorrect number of linecards: got %v, want at least %v (specified by flag)", len(validCards), *args.NumLinecards)
	}

	if got := len(validCards); got == 0 {
		t.Skipf("Not enough linecards for the test on %v: got %v, want > 0", dut.Model(), got)
	}

	c := gnmi.OC().Component(fpc)
	if deviations.PowerDisableEnableLeafRefValidation(dut) {
		gnmi.Update(t, dut, c.Config(), &oc.Component{
			Name: ygot.String(fpc),
		})
	}
	config := c.Linecard().PowerAdminState().Config()
	t.Logf("Starting %s POWER_DISABLE", fpc)
	gnmi.Replace(t, dut, config, oc.Platform_ComponentPowerType_POWER_DISABLED)

	t.Logf("Wait for 15 seconds to allow the sub component's power down process to complete")
	time.Sleep(15 * time.Second)
}

func uniqueString(input []string) []string {
	seen := make(map[string]struct{})
	var result []string

	for _, item := range input {
		if _, exists := seen[item]; !exists {
			seen[item] = struct{}{} // Mark as seen
			result = append(result, item)
		}
	}
	return result
}

func selectFpc(t testing.TB, fpcList []string, dut *ondatra.DUTDevice) string {
	t.Helper()
	var selectedFpc string
	uniqueFpcList := uniqueString(fpcList)

	if len(uniqueFpcList) > 0 {
		sort.Strings(uniqueFpcList)
		selectedFpc = uniqueFpcList[len(uniqueFpcList)-1]

		// Check if the component is physically removable. If fixed, skip gracefully.
		removable, ok := gnmi.Lookup(t, dut, gnmi.OC().Component(selectedFpc).Removable().State()).Val()
		if ok && !removable {
			t.Skipf("Skipping test: %s is a non-removable component on fixed chassis (%s)", selectedFpc, dut.Model())
		}
	} else {
		t.Fatalf("No Line card found for the test")
	}

	return selectedFpc
}

func TestBreakoutSubscription(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	aggID := netutil.NextAggregateInterface(t, dut)
	tc := &testCase{
		dut:     dut,
		ate:     ate,
		lagType: lagTypeLACP,
		top:     gosnappi.NewConfig(),

		dutPorts: sortPorts(dut.Ports()),
		atePorts: sortPorts(ate.Ports()),
		aggID:    aggID,
	}
	tc.configureATE(t)
	tc.configureDUT(t)
	ctx := context.Background()
	t.Run("verifyDUT", tc.verifyDUT)

	stream := newSubscribeRequest(ctx, t, dut)
	checkSyncResponse(t, stream)
	notifCh := startStreamReader(t, stream)

	t.Run("PLT-1.2.1 Check response after a triggered interface state change", func(t *testing.T) {
		drainChannel(notifCh)

		setDUTInterfaceWithState(t, dut, tc.dutPorts[0], false)
		setDUTInterfaceWithState(t, dut, tc.dutPorts[2], false)
		// time.Sleep(2 * time.Second)
		updateTimeout := 30 * time.Second
		receivedNotifications, err := recieveUpdateWithTimeout(ctx, t, notifCh, updateTimeout)
		if err != nil {
			t.Logf("Received error(possibly end of updates): %v", err)
		}
		expectedUpdatePaths := []string{
			"/interfaces/interface/state/admin-status",
			"/lacp/interfaces/interface/members/member/state/interface",
			"/interfaces/interface/state/oper-status",
		}
		verifyNotificationPaths(t, receivedNotifications, expectedUpdatePaths)
		verifyUpdateValue(t, receivedNotifications, dut, "DOWN")

		drainChannel(notifCh)

		setDUTInterfaceWithState(t, dut, tc.dutPorts[0], true)
		setDUTInterfaceWithState(t, dut, tc.dutPorts[2], true)
		receivedNotifications, _ = recieveUpdateWithTimeout(ctx, t, notifCh, updateTimeout)
		verifyUpdateValue(t, receivedNotifications, dut, "UP")
	})

	// Check response after a triggered interface flap
	t.Run("PLT-1.2.2 Check response after a triggered interface flap", func(t *testing.T) {
		counter := 5
		var receivedNotifications []*gpb.Notification
		var err error
		for i := 0; i < counter; i++ {
			drainChannel(notifCh)

			setDUTInterfaceWithState(t, dut, tc.dutPorts[0], false)
			setDUTInterfaceWithState(t, dut, tc.dutPorts[2], false)
			updateTimeout := 45 * time.Second
			receivedNotifications, err = recieveUpdateWithTimeout(ctx, t, notifCh, updateTimeout)
			if err != nil {
				t.Logf("Received error(possibly end of updates): %v", err)
			}
			verifyUpdateValue(t, receivedNotifications, dut, "DOWN")

			drainChannel(notifCh)

			setDUTInterfaceWithState(t, dut, tc.dutPorts[0], true)
			setDUTInterfaceWithState(t, dut, tc.dutPorts[2], true)
			receivedNotifications, _ = recieveUpdateWithTimeout(ctx, t, notifCh, updateTimeout)
			verifyUpdateValue(t, receivedNotifications, dut, "UP")
		}
		expectedUpdatePaths := []string{
			"/interfaces/interface/state/admin-status",
			"/lacp/interfaces/interface/members/member/state/interface",
			"/interfaces/interface/state/oper-status",
		}
		verifyNotificationPaths(t, receivedNotifications, expectedUpdatePaths)
	})

	// Check response after a triggered LC reboot
	t.Run("PLT-1.2.3 Check response after a triggered LC reboot", func(t *testing.T) {
		LinecardReboot(t, dut)
		updateTimeout := 300 * time.Second
		receivedNotifications, err := recieveUpdateWithTimeout(ctx, t, notifCh, updateTimeout)
		if err != nil {
			t.Logf("Received error:(possibly end of updates) %v", err)
			t.Logf("Received notifications in main function: %v", receivedNotifications)
		} else {
			t.Logf("Received notifications in main function: %v", receivedNotifications)
		}
		expectedUpdatePaths := []string{
			"/components/component/state/oper-status",
		}
		verifyNotificationPaths(t, receivedNotifications, expectedUpdatePaths)
	})

	defer stream.CloseSend()

	// Check response after a triggered chassis reboot
	t.Run("PLT-1.2.4 Check response after a triggered chassis reboot", func(t *testing.T) {
		chassisReboot(t, dut)
		streamReboot := newSubscribeRequest(ctx, t, dut)
		checkSyncResponse(t, streamReboot)
		defer streamReboot.CloseSend()
	})

	// Check response after a triggered breakout module reboot
	t.Run("PLT-1.2.5 Check response after a triggered breakout module reboot", func(t *testing.T) {
		lcs := components.FindComponentsByType(t, dut, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_LINECARD)
		if len(lcs) == 0 {
			t.Skipf("Skipping PLT-1.2.5: No LINECARD components found. Device %s is a Fixed Form Factor (FFF) chassis.", dut.Model())
		}
		intfsOperStatusUPBeforeReboot := helpers.FetchOperStatusUPIntfs(t, dut, *args.CheckInterfacesInBinding)
		t.Logf("intfsOperStatusUPBeforeReboot: %v", intfsOperStatusUPBeforeReboot)
		lcList, err := findLinecardFromPort(t, intfsOperStatusUPBeforeReboot, dut)
		if err != nil {
			t.Fatalf("Failed to find Line Card from port: %v", err)
		}
		t.Logf("LinecardList: %v", lcList)
		selectedLC := selectFpc(t, lcList, dut)
		t.Logf("selectedFpc: %v", selectedLC)
		linecardDown(t, dut, selectedLC, lcs)

		streamLC := newSubscribeRequest(ctx, t, dut)
		checkSyncResponse(t, streamLC)
		notifChLC := startStreamReader(t, streamLC)

		lineCardUp(t, dut, selectedLC)
		updateTimeout := 10 * time.Minute
		receivedNotifications, err := recieveUpdateWithTimeout(ctx, t, notifChLC, updateTimeout)
		if err != nil {
			t.Logf("Received error(possibly end of updates): %v", err)
		}
		verifyNotificationPathsForPortUpdates(t, receivedNotifications, selectedLC)
		defer streamLC.CloseSend()
	})
}
