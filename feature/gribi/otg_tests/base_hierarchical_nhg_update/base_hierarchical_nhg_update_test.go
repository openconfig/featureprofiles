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

package base_hierarchical_nhg_update_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/gribigo/chk"
	"github.com/openconfig/gribigo/client"
	"github.com/openconfig/gribigo/constants"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

const (
	vrfName = "VRF-1"

	// Destination ATE MAC address for port-2 and port-3.
	pMAC = "00:1A:11:00:1A:BC"

	// port-2 nexthop ID.
	p2NHID = 40
	// port-3 nexthop ID.
	p3NHID = 41

	// VirtualIP route next-hop-group ID.
	virtualIPNHGID = 42
	// VirtualIP route nexthop.
	virtualIP = "203.0.113.1"
	// VirtualIP route prefix.
	virtualPfx = "203.0.113.1/32"

	// Destination route next-hop ID
	dstNHID = 43
	// Destination route next-hop-group ID
	dstNHGID = 44
	// Destination route prefix for DUT to ATE traffic.
	dstPfx            = "198.51.100.0/24"
	dstPfxFlowIP      = "198.51.100.0"
	ipv4PrefixLen     = 30
	ipv4FlowCount     = 65000
	innerSrcIPv4Start = "198.18.0.0"
	innerDstIPv4Start = "198.19.0.0"
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "dutPort1",
		IPv4:    "192.0.2.1",
		IPv4Len: 30,
	}
	dutPort2 = attrs.Attributes{
		Desc:    "dutPort2",
		IPv4:    "192.0.2.5",
		IPv4Len: 30,
	}
	dutPort3 = attrs.Attributes{
		Desc:    "dutPort3",
		IPv4:    "192.0.2.9",
		IPv4Len: 30,
	}

	atePort1 = attrs.Attributes{
		Name:    "atePort1",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.0.2.2",
		IPv4Len: 30,
	}
	atePort2 = attrs.Attributes{
		Name:    "atePort2",
		MAC:     "02:00:02:01:01:01",
		IPv4:    "192.0.2.6",
		IPv4Len: 30,
	}
	atePort3 = attrs.Attributes{
		Name:    "atePort3",
		MAC:     "02:00:03:01:01:01",
		IPv4:    "192.0.2.10",
		IPv4Len: 30,
	}

	dutPort2DummyIP = attrs.Attributes{
		Desc:    "dutPort2",
		IPv4:    "192.0.2.21",
		IPv4Len: 30,
	}
	dutPort3DummyIP = attrs.Attributes{
		Desc:    "dutPort3",
		IPv4:    "192.0.2.41",
		IPv4Len: 30,
	}
	atePort2DummyIP = attrs.Attributes{
		Desc:    "atePort2",
		IPv4:    "192.0.2.22",
		IPv4Len: 32,
	}
	atePort3DummyIP = attrs.Attributes{
		Desc:    "atePort3",
		IPv4:    "192.0.2.42",
		IPv4Len: 32,
	}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestBaseHierarchicalNHGUpdate(t *testing.T) {
	ctx := context.Background()

	dut := ondatra.DUT(t, "dut")
	configureDUT(t, dut)

	ate := ondatra.ATE(t, "ate")
	top := configureATE(t, ate)

	p2flow := "Port 1 to Port 2"
	p3flow := "Port 1 to Port 3"
	lbFlow := "Port 1 to Port 2 and Port 3"
	createFlow(t, p2flow, top, &atePort2)
	createFlow(t, p3flow, top, &atePort3)
	createFlow(t, lbFlow, top, &atePort2, &atePort3)

	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)

	gribic, err := gribiClient(ctx, t, dut)
	if err != nil {
		t.Fatalf("Got error during gribi client setup: %v", err)
	}

	defer func() {
		// Flush all entries after test.
		if err = gribi.FlushAll(gribic); err != nil {
			t.Error(err)
		}
		if deviations.GRIBIMACOverrideStaticARPStaticRoute(dut) {
			sp := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
			gnmi.Delete(t, dut, sp.Static(atePort2DummyIP.IPv4CIDR()).Config())
			gnmi.Delete(t, dut, sp.Static(atePort3DummyIP.IPv4CIDR()).Config())
		}
	}()

	gribi.BecomeLeader(t, gribic)
	dutP2 := dut.Port(t, "port2").Name()
	dutP3 := dut.Port(t, "port3").Name()

	t.Logf("Adding gribi routes and validating traffic forwarding via port %v and NH ID %v", dutP2, p2NHID)
	if deviations.GRIBIMACOverrideWithStaticARP(dut) || deviations.GRIBIMACOverrideStaticARPStaticRoute(dut) {
		addVIPRoute(ctx, dut, t, gribic, p2NHID, dutP2, atePort2DummyIP.IPv4)
	} else {
		addVIPRoute(ctx, dut, t, gribic, p2NHID, dutP2)
	}
	addDestinationRoute(ctx, dut, t, gribic)
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")
	validateTrafficFlows(t, p2flow, p3flow)

	t.Logf("Adding a new NH via port %v with ID %v", dutP3, p3NHID)
	if deviations.GRIBIMACOverrideWithStaticARP(dut) || deviations.GRIBIMACOverrideStaticARPStaticRoute(dut) {
		addNH(ctx, t, dut, gribic, p3NHID, dutP3, pMAC, atePort3DummyIP.IPv4)
	} else {
		addNH(ctx, t, dut, gribic, p3NHID, dutP3, pMAC)
	}

	t.Logf("Performing implicit in-place replace with two next-hops (NH IDs: %v and %v)", p2NHID, p3NHID)
	addNHG(ctx, dut, t, gribic, virtualIPNHGID, []uint64{p2NHID, p3NHID})
	validateTrafficFlows(t, lbFlow, "")

	t.Logf("Performing implicit in-place replace using the next-hop with ID %v", p3NHID)
	addNHG(ctx, dut, t, gribic, virtualIPNHGID, []uint64{p3NHID})
	validateTrafficFlows(t, p3flow, p2flow)

	t.Logf("Performing implicit in-place replace using the next-hop with ID %v", p2NHID)
	addNHG(ctx, dut, t, gribic, virtualIPNHGID, []uint64{p2NHID})
	validateTrafficFlows(t, p2flow, p3flow)
}

// addNH adds a GRIBI NH with a FIB ACK confirmation via Modify RPC
func addNH(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, gribic *fluent.GRIBIClient, id uint64, intf, mac string, nhip ...string) {
	nh := fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
		WithIndex(id).WithInterfaceRef(intf).WithMacAddress(mac)
	if len(nhip) > 0 {
		nh = nh.WithIPAddress(nhip[0])
	}

	gribic.Modify().AddEntry(t, nh)
	if err := awaitTimeout(ctx, gribic, t, 2*time.Minute); err != nil {
		t.Fatalf("Await got error for entries: %v", err)
	}
	result := fluent.InstalledInFIB
	if deviations.GRIBIMACOverrideStaticARPStaticRoute(dut) {
		result = fluent.InstalledInRIB
	}
	wantOperationResults := []*client.OpResult{
		fluent.OperationResult().
			WithNextHopOperation(id).
			WithProgrammingResult(result).
			WithOperationType(constants.Add).
			AsResult(),
	}
	for _, wantResult := range wantOperationResults {
		chk.HasResult(t, gribic.Results(t), wantResult, chk.IgnoreOperationID())
	}
}

// addNHG adds a GRIBI NHG with a FIB ACK confirmation via Modify RPC
func addNHG(ctx context.Context, dut *ondatra.DUTDevice, t *testing.T, gribic *fluent.GRIBIClient, id uint64, nhs []uint64) {
	nhg := fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
		WithID(id)
	for _, nh := range nhs {
		nhg.AddNextHop(nh, 1)
	}
	gribic.Modify().AddEntry(t, nhg)
	if err := awaitTimeout(ctx, gribic, t, time.Minute); err != nil {
		t.Fatalf("Await got error for entries: %v", err)
	}
	wantOperationResults := []*client.OpResult{
		fluent.OperationResult().
			WithNextHopGroupOperation(id).
			WithProgrammingResult(fluent.InstalledInFIB).
			WithOperationType(constants.Add).
			AsResult(),
	}
	for _, wantResult := range wantOperationResults {
		chk.HasResult(t, gribic.Results(t), wantResult, chk.IgnoreOperationID())
	}
}

// addDestinationRoute adds a GRIBI route to dstPfx via the VirtualIP GRIBI nexthop.
func addDestinationRoute(ctx context.Context, dut *ondatra.DUTDevice, t *testing.T, gribic *fluent.GRIBIClient) {
	dnh := fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
		WithIndex(dstNHID).WithIPAddress(virtualIP)
	dnhg := fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
		WithID(dstNHGID).AddNextHop(dstNHID, 1)
	dpfx := fluent.IPv4Entry().WithNetworkInstance(vrfName).WithPrefix(dstPfx).WithNextHopGroup(dstNHGID).WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut))

	gribic.Modify().AddEntry(t, dnh, dnhg, dpfx)
	if err := awaitTimeout(ctx, gribic, t, time.Minute); err != nil {
		t.Fatalf("Await got error for entries: %v", err)
	}

	wantOperationResults := []*client.OpResult{
		fluent.OperationResult().
			WithNextHopOperation(dstNHID).
			WithProgrammingResult(fluent.InstalledInFIB).
			WithOperationType(constants.Add).
			AsResult(),
		fluent.OperationResult().
			WithNextHopGroupOperation(dstNHGID).
			WithProgrammingResult(fluent.InstalledInFIB).
			WithOperationType(constants.Add).
			AsResult(),
		fluent.OperationResult().
			WithIPv4Operation(dstPfx).
			WithProgrammingResult(fluent.InstalledInFIB).
			WithOperationType(constants.Add).
			AsResult(),
	}

	for _, wantResult := range wantOperationResults {
		chk.HasResult(t, gribic.Results(t), wantResult, chk.IgnoreOperationID())
	}
}

// addVIPRoute creates a GRIBI route that points to the egress interface defined by id,
// port, and nhip.
func addVIPRoute(ctx context.Context, dut *ondatra.DUTDevice, t *testing.T, gribic *fluent.GRIBIClient, id uint64, port string, nhip ...string) {
	inh := fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
		WithIndex(id).WithInterfaceRef(port).WithMacAddress(pMAC)
	inhg := fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
		WithID(virtualIPNHGID).AddNextHop(id, 1)
	ipfx := fluent.IPv4Entry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
		WithPrefix(virtualPfx).WithNextHopGroup(virtualIPNHGID)
	if len(nhip) > 0 {
		inh = inh.WithIPAddress(nhip[0])
	}

	gribic.Modify().AddEntry(t, inh, inhg, ipfx)
	if err := awaitTimeout(ctx, gribic, t, time.Minute); err != nil {
		t.Fatalf("Await got error for entries: %v", err)
	}

	wantOperationResults := []*client.OpResult{
		fluent.OperationResult().
			WithNextHopOperation(id).
			WithProgrammingResult(fluent.InstalledInFIB).
			WithOperationType(constants.Add).
			AsResult(),
		fluent.OperationResult().
			WithNextHopGroupOperation(virtualIPNHGID).
			WithProgrammingResult(fluent.InstalledInFIB).
			WithOperationType(constants.Add).
			AsResult(),
		fluent.OperationResult().
			WithIPv4Operation(virtualPfx).
			WithProgrammingResult(fluent.InstalledInFIB).
			WithOperationType(constants.Add).
			AsResult(),
	}

	for _, wantResult := range wantOperationResults {
		chk.HasResult(t, gribic.Results(t), wantResult, chk.IgnoreOperationID())
	}
}

// awaitTimeout calls a fluent client Await, adding a timeout to the context.
func awaitTimeout(ctx context.Context, c *fluent.GRIBIClient, t testing.TB, timeout time.Duration) error {
	subctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return c.Await(subctx, t)
}

func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	top := ate.OTG().NewConfig(t)

	p1 := ate.Port(t, "port1")
	p2 := ate.Port(t, "port2")
	p3 := ate.Port(t, "port3")

	atePort1.AddToOTG(top, p1, &dutPort1)
	atePort2.AddToOTG(top, p2, &dutPort2)
	atePort3.AddToOTG(top, p3, &dutPort3)

	return top
}

func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	d := gnmi.OC()

	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")
	p3 := dut.Port(t, "port3")

	vrf := &oc.NetworkInstance{
		Name: ygot.String(vrfName),
		Type: oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF,
	}

	p1VRF := vrf.GetOrCreateInterface(p1.Name())
	p1VRF.Interface = ygot.String(p1.Name())
	p1VRF.Subinterface = ygot.Uint32(0)

	// For interface configuration, Arista prefers config Vrf first then the IP address
	if deviations.InterfaceConfigVRFBeforeAddress(dut) {
		gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(vrfName).Config(), vrf)
	}

	gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), dutPort1.NewOCInterface(p1.Name(), dut))

	if !deviations.InterfaceConfigVRFBeforeAddress(dut) {
		gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(vrfName).Config(), vrf)
	}

	gnmi.Replace(t, dut, d.Interface(p2.Name()).Config(), dutPort2.NewOCInterface(p2.Name(), dut))
	gnmi.Replace(t, dut, d.Interface(p3.Name()).Config(), dutPort3.NewOCInterface(p3.Name(), dut))

	if deviations.ExplicitIPv6EnableForGRIBI(dut) {
		gnmi.Update(t, dut, d.Interface(p2.Name()).Subinterface(0).Ipv6().Enabled().Config(), true)
		gnmi.Update(t, dut, d.Interface(p3.Name()).Subinterface(0).Ipv6().Enabled().Config(), true)
	}

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, p1)
		fptest.SetPortSpeed(t, p2)
		fptest.SetPortSpeed(t, p3)
	}
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, p2.Name(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, p3.Name(), deviations.DefaultNetworkInstance(dut), 0)
	}
	if deviations.ExplicitGRIBIUnderNetworkInstance(dut) {
		fptest.EnableGRIBIUnderNetworkInstance(t, dut, deviations.DefaultNetworkInstance(dut))
		fptest.EnableGRIBIUnderNetworkInstance(t, dut, vrfName)
	}

	if deviations.GRIBIMACOverrideWithStaticARP(dut) {
		staticARPWithSecondaryIP(t, dut)
	}
	if deviations.GRIBIMACOverrideStaticARPStaticRoute(dut) {
		staticARPWithMagicUniversalIP(t, dut)
	}
}

func staticARPWithSecondaryIP(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	p2 := dut.Port(t, "port2")
	p3 := dut.Port(t, "port3")
	gnmi.Update(t, dut, gnmi.OC().Interface(p2.Name()).Config(), dutPort2DummyIP.NewOCInterface(p2.Name(), dut))
	gnmi.Update(t, dut, gnmi.OC().Interface(p3.Name()).Config(), dutPort3DummyIP.NewOCInterface(p3.Name(), dut))
	gnmi.Update(t, dut, gnmi.OC().Interface(p2.Name()).Config(), configStaticArp(p2, atePort2DummyIP.IPv4, pMAC))
	gnmi.Update(t, dut, gnmi.OC().Interface(p3.Name()).Config(), configStaticArp(p3, atePort3DummyIP.IPv4, pMAC))
}

func staticARPWithMagicUniversalIP(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	p2 := dut.Port(t, "port2")
	p3 := dut.Port(t, "port3")
	s2 := &oc.NetworkInstance_Protocol_Static{
		Prefix: ygot.String(atePort2DummyIP.IPv4CIDR()),
		NextHop: map[string]*oc.NetworkInstance_Protocol_Static_NextHop{
			strconv.Itoa(p2NHID): {
				Index: ygot.String(strconv.Itoa(p2NHID)),
				InterfaceRef: &oc.NetworkInstance_Protocol_Static_NextHop_InterfaceRef{
					Interface: ygot.String(p2.Name()),
				},
			},
		},
	}
	s3 := &oc.NetworkInstance_Protocol_Static{
		Prefix: ygot.String(atePort3DummyIP.IPv4CIDR()),
		NextHop: map[string]*oc.NetworkInstance_Protocol_Static_NextHop{
			strconv.Itoa(p3NHID): {
				Index: ygot.String(strconv.Itoa(p3NHID)),
				InterfaceRef: &oc.NetworkInstance_Protocol_Static_NextHop_InterfaceRef{
					Interface: ygot.String(p3.Name()),
				},
			},
		},
	}
	sp := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
	gnmi.Replace(t, dut, sp.Static(atePort2DummyIP.IPv4CIDR()).Config(), s2)
	gnmi.Replace(t, dut, sp.Static(atePort3DummyIP.IPv4CIDR()).Config(), s3)
	gnmi.Update(t, dut, gnmi.OC().Interface(p2.Name()).Config(), configStaticArp(p2, atePort2DummyIP.IPv4, pMAC))
	gnmi.Update(t, dut, gnmi.OC().Interface(p3.Name()).Config(), configStaticArp(p3, atePort3DummyIP.IPv4, pMAC))
}

// createFlow returns a flow from atePort1 to the dstPfx, expected to arrive on ATE interface dsts.
func createFlow(_ *testing.T, name string, ateTop gosnappi.Config, dsts ...*attrs.Attributes) {
	var rxEndpoints []string
	for _, dst := range dsts {
		rxEndpoints = append(rxEndpoints, dst.Name+".IPv4")
	}

	flowipv4 := ateTop.Flows().Add().SetName(name)
	flowipv4.Metrics().SetEnable(true)
	e1 := flowipv4.Packet().Add().Ethernet()
	e1.Src().SetValue(atePort1.MAC)
	e1.Dst().SetChoice("value").SetValue(pMAC)
	if len(dsts) > 1 {
		flowipv4.TxRx().Port().SetTxName("port1")
	} else {
		flowipv4.TxRx().Device().SetTxNames([]string{atePort1.Name + ".IPv4"}).SetRxNames(rxEndpoints)
	}
	outerIPHeader := flowipv4.Packet().Add().Ipv4()
	outerIPHeader.Src().SetValue(atePort1.IPv4)
	outerIPHeader.Dst().SetValue(dstPfxFlowIP)
	innerIPHeader := flowipv4.Packet().Add().Ipv4()
	innerIPHeader.Src().Increment().SetStart(innerSrcIPv4Start).SetStep("0.0.0.1").SetCount(ipv4FlowCount)
	innerIPHeader.Dst().Increment().SetStart(innerDstIPv4Start).SetStep("0.0.0.1").SetCount(ipv4FlowCount)
	flowipv4.Size().SetFixed(100)
}

func gribiClient(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice) (*fluent.GRIBIClient, error) {
	gribic := dut.RawAPIs().GRIBI().Default(t)
	c := fluent.NewClient()
	c.Connection().WithStub(gribic).
		WithPersistence().
		WithFIBACK().
		WithRedundancyMode(fluent.ElectedPrimaryClient).
		WithInitialElectionID(1, 0)
	c.Start(ctx, t)
	t.Cleanup(func() { c.Stop(t) })
	c.StartSending(ctx, t)
	if err := awaitTimeout(ctx, c, t, time.Minute); err != nil {
		return nil, err
	}

	return c, nil
}

// validateTrafficFlows starts traffic and ensures that good flows have 0% loss and bad flows have
// 100% loss.
func validateTrafficFlows(t *testing.T, goodFlow, badFlow string) {

	otg := ondatra.ATE(t, "ate").OTG()
	config := otg.FetchConfig(t)
	otg.StartTraffic(t)
	time.Sleep(15 * time.Second)
	otg.StopTraffic(t)

	otgutils.LogFlowMetrics(t, otg, config)
	otgutils.LogPortMetrics(t, otg, config)
	if got := getLossPct(t, goodFlow); got > 0 {
		t.Errorf("LossPct for flow %s: got %v, want 0", goodFlow, got)
	}
	if badFlow != "" {
		if got := getLossPct(t, badFlow); got < 100 {
			t.Errorf("LossPct for flow %s: got %v, want 100", badFlow, got)
		}
	}
}

// getLossPct returns the loss percentage for a given flow
func getLossPct(t *testing.T, flowName string) uint64 {
	t.Helper()
	otg := ondatra.ATE(t, "ate").OTG()
	flowStats := gnmi.Get(t, otg, gnmi.OTG().Flow(flowName).State())
	txPackets := flowStats.GetCounters().GetOutPkts()
	rxPackets := flowStats.GetCounters().GetInPkts()
	lostPackets := txPackets - rxPackets
	if txPackets == 0 {
		t.Fatalf("Tx packets should be higher than 0 for flow %s", flowName)
	}
	lossPct := lostPackets * 100 / txPackets
	return lossPct
}

func configStaticArp(p *ondatra.Port, ipv4addr string, macAddr string) *oc.Interface {
	i := &oc.Interface{Name: ygot.String(p.Name())}
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	s := i.GetOrCreateSubinterface(0)
	s4 := s.GetOrCreateIpv4()
	n4 := s4.GetOrCreateNeighbor(ipv4addr)
	n4.LinkLayerAddress = ygot.String(macAddr)
	return i
}
