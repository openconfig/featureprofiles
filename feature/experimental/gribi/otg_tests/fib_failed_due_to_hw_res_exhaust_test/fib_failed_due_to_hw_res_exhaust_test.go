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

package fib_failed_due_to_hw_res_exhaust_test

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"

	aftspb "github.com/openconfig/gribi/v1/proto/service"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Settings for configuring the baseline testbed with the test
// topology.
//
// The testbed consists of ate:port1 -> dut:port1,
// dut:port2 -> ate:port2.
//
//   * ate:port1 -> dut:port1 subnet 192.0.2.1/30
//   * ate:port2 -> dut:port2 subnet 192.0.2.5/30

const (
	dstIPBlock                = "203.0.113.0"
	vipBlock                  = "198.51.100.0"
	wantLoss                  = true
	dutAS                     = 64500
	ateAS                     = 64501
	advertisedRoutesv4        = "20.0.1.1"
	advertisedRoutesv4MaskLen = 32
	tolerancePct              = 2
	tolerance                 = 50
)

var (
	vendorSpecRoutecount = map[ondatra.Vendor]uint32{
		ondatra.JUNIPER: 2500000,
	}
	dutPort1 = attrs.Attributes{
		Desc:    "dutPort1",
		IPv4:    "192.0.2.1",
		IPv4Len: 30,
	}

	atePort1 = attrs.Attributes{
		Name:    "atePort1",
		IPv4:    "192.0.2.2",
		MAC:     "02:00:01:01:01:01",
		IPv4Len: 30,
	}

	dutPort2 = attrs.Attributes{
		Desc:    "dutPort2",
		IPv4:    "192.0.2.5",
		IPv4Len: 30,
	}

	atePort2 = attrs.Attributes{
		Name:    "atePort2",
		IPv4:    "192.0.2.6",
		MAC:     "02:00:02:01:01:01",
		IPv4Len: 30,
	}
	fibPassedDstRoute string
	fibFailedDstRoute string
)

func configureBGP(dut *ondatra.DUTDevice) *oc.NetworkInstance_Protocol {

	d := &oc.Root{}
	ni1 := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	ni_proto := ni1.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := ni_proto.GetOrCreateBgp()

	g := bgp.GetOrCreateGlobal()
	g.As = ygot.Uint32(dutAS)
	g.RouterId = ygot.String(dutPort1.IPv4)

	pg := bgp.GetOrCreatePeerGroup("BGP-PEER-GROUP-V4")
	pg.PeerAs = ygot.Uint32(ateAS)
	pg.PeerGroupName = ygot.String("BGP-PEER-GROUP-V4")

	if deviations.RoutePolicyUnderAFIUnsupported(dut) {
		rpl := pg.GetOrCreateApplyPolicy()
		rpl.SetExportPolicy([]string{"ALLOW"})
		rpl.SetImportPolicy([]string{"ALLOW"})
	} else {
		pg1af4 := pg.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
		pg1af4.Enabled = ygot.Bool(true)
		pg1rpl4 := pg1af4.GetOrCreateApplyPolicy()
		pg1rpl4.SetExportPolicy([]string{"ALLOW"})
		pg1rpl4.SetImportPolicy([]string{"ALLOW"})
	}

	bgpNbr := bgp.GetOrCreateNeighbor(atePort1.IPv4)
	bgpNbr.PeerAs = ygot.Uint32(ateAS)
	bgpNbr.Enabled = ygot.Bool(true)
	bgpNbr.PeerGroup = ygot.String("BGP-PEER-GROUP-V4")
	af4 := bgpNbr.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
	af4.Enabled = ygot.Bool(true)
	return ni_proto
}

func configureOTG(t *testing.T, otg *otg.OTG) (gosnappi.BgpV4Peer, gosnappi.DeviceIpv4, gosnappi.Config) {
	t.Helper()
	config := otg.NewConfig(t)
	port1 := config.Ports().Add().SetName("port1")
	port2 := config.Ports().Add().SetName("port2")

	iDut1Dev := config.Devices().Add().SetName(atePort1.Name)
	iDut1Eth := iDut1Dev.Ethernets().Add().SetName(atePort1.Name + ".Eth").SetMac(atePort1.MAC)
	iDut1Eth.Connection().SetChoice(gosnappi.EthernetConnectionChoice.PORT_NAME).SetPortName(port1.Name())
	iDut1Ipv4 := iDut1Eth.Ipv4Addresses().Add().SetName(atePort1.Name + ".IPv4")
	iDut1Ipv4.SetAddress(atePort1.IPv4).SetGateway(dutPort1.IPv4).SetPrefix(uint32(atePort1.IPv4Len))

	iDut2Dev := config.Devices().Add().SetName(atePort2.Name)
	iDut2Eth := iDut2Dev.Ethernets().Add().SetName(atePort2.Name + ".Eth").SetMac(atePort2.MAC)
	iDut2Eth.Connection().SetChoice(gosnappi.EthernetConnectionChoice.PORT_NAME).SetPortName(port2.Name())
	iDut2Ipv4 := iDut2Eth.Ipv4Addresses().Add().SetName(atePort2.Name + ".IPv4")
	iDut2Ipv4.SetAddress(atePort2.IPv4).SetGateway(dutPort2.IPv4).SetPrefix(uint32(atePort2.IPv4Len))

	iDut1Bgp := iDut1Dev.Bgp().SetRouterId(iDut1Ipv4.Address())
	iDut1Bgp4Peer := iDut1Bgp.Ipv4Interfaces().Add().SetIpv4Name(iDut1Ipv4.Name()).Peers().Add().SetName(atePort1.Name + ".BGP4.peer")
	iDut1Bgp4Peer.SetPeerAddress(iDut1Ipv4.Gateway()).SetAsNumber(ateAS).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
	iDut1Bgp4Peer.Capability().SetIpv4UnicastAddPath(true).SetIpv6UnicastAddPath(true)
	iDut1Bgp4Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)

	t.Logf("Pushing config to ATE and starting protocols...")
	otg.PushConfig(t, config)
	time.Sleep(30 * time.Second)
	otg.StartProtocols(t)
	time.Sleep(30 * time.Second)

	return iDut1Bgp4Peer, iDut1Ipv4, config
}

func configureRoutePolicy(t *testing.T, dut *ondatra.DUTDevice, name string, pr oc.E_RoutingPolicy_PolicyResultType) {
	d := &oc.Root{}
	rp := d.GetOrCreateRoutingPolicy()
	pd := rp.GetOrCreatePolicyDefinition(name)
	st, err := pd.AppendNewStatement("id-1")
	if err != nil {
		t.Fatal(err)
	}
	st.GetOrCreateActions().PolicyResult = pr
	gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)
}

type testArgs struct {
	ctx           context.Context
	dut           *ondatra.DUTDevice
	ate           *ondatra.ATEDevice
	otgBgpPeer    gosnappi.BgpV4Peer
	otgIPv4Device gosnappi.DeviceIpv4
	otgConfig     gosnappi.Config
	client        *fluent.GRIBIClient
	electionID    gribi.Uint128
	otg           *otg.OTG
}

// TestFibFailDueToHwResExhaust is to test gRIBI FIB_FAILED functionality
// is supported due to hardware exhaust.
func TestFibFailDueToHwResExhaust(t *testing.T) {
	ctx := context.Background()
	dc := gnmi.OC()
	dut := ondatra.DUT(t, "dut")
	configureDUT(t, dut)
	configureRoutePolicy(t, dut, "ALLOW", oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)

	t.Log("Configure/update Network Instance")
	dutConfNIPath := dc.NetworkInstance(deviations.DefaultNetworkInstance(dut))
	gnmi.Replace(t, dut, dutConfNIPath.Type().Config(), oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)

	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	t.Run("configureBGP", func(t *testing.T) {
		dutConf := configureBGP(dut)
		gnmi.Replace(t, dut, dutConfPath.Config(), dutConf)
		fptest.LogQuery(t, "DUT BGP Config", dutConfPath.Config(), gnmi.GetConfig(t, dut, dutConfPath.Config()))
	})

	ate := ondatra.ATE(t, "ate")
	otg := ate.OTG()
	var otgConfig gosnappi.Config
	var otgBgpPeer gosnappi.BgpV4Peer
	var otgIPv4Device gosnappi.DeviceIpv4
	otgBgpPeer, otgIPv4Device, otgConfig = configureOTG(t, otg)

	verifyBgpTelemetry(t, dut)

	gribic := dut.RawAPIs().GRIBI().Default(t)

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
		ctx:           ctx,
		client:        client,
		dut:           dut,
		ate:           ate,
		otgBgpPeer:    otgBgpPeer,
		otgIPv4Device: otgIPv4Device,
		otgConfig:     otgConfig,
		electionID:    eID,
		otg:           otg,
	}
	start := time.Now()
	injectEntry(ctx, t, args)
	t.Logf("Main Function: Time elapsed %.2f seconds since start", time.Since(start).Seconds())

	t.Log("Send traffic to any of the programmed entries and validate.")
	sendTraffic(t, args)
}

func sendTraffic(t *testing.T, args *testArgs) {
	// Ensure that traffic can be forwarded between ATE port-1 and ATE port-2.
	t.Helper()
	t.Logf("TestBGP:start otg Traffic config")
	flow1ipv4 := args.otgConfig.Flows().Add().SetName("Flow1")
	flow1ipv4.Metrics().SetEnable(true)
	flow1ipv4.TxRx().Device().
		SetTxNames([]string{atePort1.Name + ".IPv4"}).
		SetRxNames([]string{atePort2.Name + ".IPv4"})
	flow1ipv4.Size().SetFixed(512)
	flow1ipv4.Rate().SetPps(100)
	flow1ipv4.Duration().SetChoice("continuous")
	e1 := flow1ipv4.Packet().Add().Ethernet()
	e1.Src().SetValue(atePort1.MAC)
	v4 := flow1ipv4.Packet().Add().Ipv4()
	v4.Src().SetValue(atePort1.IPv4)
	v4.Dst().Increment().SetStart(fibPassedDstRoute)

	flow2ipv4 := args.otgConfig.Flows().Add().SetName("Flow2")
	flow2ipv4.Metrics().SetEnable(true)
	flow2ipv4.TxRx().Device().
		SetTxNames([]string{atePort1.Name + ".IPv4"}).
		SetRxNames([]string{atePort2.Name + ".IPv4"})
	flow2ipv4.Size().SetFixed(512)
	flow2ipv4.Rate().SetPps(100)
	flow2ipv4.Duration().SetChoice("continuous")
	e2 := flow2ipv4.Packet().Add().Ethernet()
	e2.Src().SetValue(atePort1.MAC)
	v4Flow2 := flow2ipv4.Packet().Add().Ipv4()
	v4Flow2.Src().SetValue(atePort1.IPv4)
	v4Flow2.Dst().Increment().SetStart(fibFailedDstRoute)

	args.otg.PushConfig(t, args.otgConfig)
	args.otg.StartProtocols(t)

	t.Logf("Starting traffic")
	args.otg.StartTraffic(t)
	time.Sleep(15 * time.Second)
	t.Logf("Stop traffic")
	args.otg.StopTraffic(t)

	verifyTraffic(t, args, flow1ipv4.Name(), !wantLoss)

	if !deviations.GRIBISkipFibFailedTrafficForwardingCheck(args.dut) {
		verifyTraffic(t, args, flow2ipv4.Name(), wantLoss)
	}
}

func verifyTraffic(t *testing.T, args *testArgs, flowName string, wantLoss bool) {
	t.Helper()
	t.Logf("Verifying flow metrics for the flow %s\n", flowName)
	recvMetric := gnmi.Get(t, args.otg, gnmi.OTG().Flow(flowName).State())
	txPackets := recvMetric.GetCounters().GetOutPkts()
	rxPackets := recvMetric.GetCounters().GetInPkts()
	lostPackets := txPackets - rxPackets
	var lossPct uint64
	if txPackets != 0 {
		lossPct = lostPackets * 100 / txPackets
	} else {
		t.Errorf("Traffic stats are not correct %v", recvMetric)
	}
	if wantLoss {
		if lossPct < 100-tolerancePct {
			t.Errorf("Traffic is expected to fail %s\n got %v, want 100%% failure", flowName, lossPct)
		} else {
			t.Logf("Traffic Loss Test Passed!")
		}
	} else {
		if lossPct > tolerancePct {
			t.Errorf("Traffic Loss Pct for Flow: %s\n got %v, want 0", flowName, lossPct)
		} else {
			t.Logf("Traffic Test Passed!")
		}
	}
}

func verifyBgpTelemetry(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	var nbrIP = []string{atePort1.IPv4}
	t.Logf("Verifying BGP state.")
	bgpPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()

	for _, nbr := range nbrIP {
		nbrPath := bgpPath.Neighbor(nbr)
		// Get BGP adjacency state.
		t.Logf("Waiting for BGP neighbor to establish...")
		status, ok := gnmi.Watch(t, dut, nbrPath.SessionState().State(), time.Minute, func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
			state, ok := val.Val()
			return ok && state == oc.Bgp_Neighbor_SessionState_ESTABLISHED
		}).Await(t)
		if !ok {
			t.Fatal("No BGP neighbor formed")
		}
		state, _ := status.Val()
		t.Logf("BGP adjacency for %s: %v", nbr, state)
		if want := oc.Bgp_Neighbor_SessionState_ESTABLISHED; state != want {
			t.Errorf("BGP peer %s status got %d, want %d", nbr, state, want)
		}
	}
}

// configureDUT configures port1-2 on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	d := gnmi.OC()

	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")

	gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), dutPort1.NewOCInterface(p1.Name(), dut))
	gnmi.Replace(t, dut, d.Interface(p2.Name()).Config(), dutPort2.NewOCInterface(p2.Name(), dut))
}

func injectBGPRoutes(t *testing.T, args *testArgs) {
	t.Helper()

	if _, ok := vendorSpecRoutecount[args.dut.Vendor()]; !ok {
		t.Fatalf("Please provide BGP route count for vendor to maxout FIB %v in var vendorSpecRoutecount ", args.dut.Vendor())
	}
	bgpNeti1Bgp4PeerRoutes := args.otgBgpPeer.V4Routes().Add().SetName(atePort1.Name + ".BGP4.Route")
	bgpNeti1Bgp4PeerRoutes.SetNextHopIpv4Address(args.otgIPv4Device.Address()).
		SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).
		SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)
	bgpNeti1Bgp4PeerRoutes.Addresses().Add().
		SetAddress(advertisedRoutesv4).
		SetPrefix(advertisedRoutesv4MaskLen).
		SetCount(vendorSpecRoutecount[args.dut.Vendor()]).SetStep(2)

	args.otg.PushConfig(t, args.otgConfig)
	time.Sleep(30 * time.Second)
	args.otg.StartProtocols(t)
	time.Sleep(30 * time.Second)
}

// awaitTimeout calls a fluent client Await, adding a timeout to the context.
func awaitTimeout(ctx context.Context, c *fluent.GRIBIClient, t testing.TB, timeout time.Duration) error {
	t.Helper()
	subctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return c.Await(subctx, t)
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

// injectEntry programs gRIBI nh, nhg and ipv4 entry.
func injectEntry(ctx context.Context, t *testing.T, args *testArgs) {
	t.Helper()
	dstIPList := createIPv4Entries(t, fmt.Sprintf("%s/%d", dstIPBlock, 20))
	vipList := createIPv4Entries(t, fmt.Sprintf("%s/%d", vipBlock, 20))
	j := uint64(0)

routeAddLoop:
	for i := uint64(1); i <= uint64(1500); i += 2 {
		vipNhIndex := i
		dstNhIndex := vipNhIndex + 1

		args.client.Modify().AddEntry(t,
			fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
				WithIndex(vipNhIndex).WithIPAddress(atePort2.IPv4),
			fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
				WithID(vipNhIndex).AddNextHop(vipNhIndex, 1),
			fluent.IPv4Entry().WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
				WithPrefix(vipList[j]).WithNextHopGroup(vipNhIndex),
		)
		args.client.Modify().AddEntry(t,
			fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
				WithIndex(dstNhIndex).WithIPAddress(vipList[j]),
			fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
				WithID(dstNhIndex).AddNextHop(dstNhIndex, 1),
			fluent.IPv4Entry().WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
				WithPrefix(dstIPList[j]).WithNextHopGroup(dstNhIndex),
		)

		if err := awaitTimeout(args.ctx, args.client, t, time.Minute); err != nil {
			t.Logf("Could not program entries via client, got err, check error codes: %v", err)
		}

		for _, v := range args.client.Results(t) {
			if v.ProgrammingResult == aftspb.AFTResult_FIB_FAILED {
				t.Logf("FIB FAILED received %v", v.Details)
				fibFailedDstRoute = dstIPList[j]
				break routeAddLoop
			}
		}
		j = j + 1
		if i >= 1458 {
			t.Errorf("FIB FAILED is not received as expected")
		}
		if j == 1 {
			fibPassedDstRoute = dstIPList[0]
			injectBGPRoutes(t, args)
			time.Sleep(5 * time.Minute)
		}
	}
}
