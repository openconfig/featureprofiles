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

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
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
	// 15-bit filter for egress flow tracking. 1ABC in hex == 43981 in decimal.
	pMACFilter = "6844"

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

	// Destination route next-hop ID.
	dstNHID = 43
	// Destination route next-hop-group ID.
	dstNHGID = 44
	// Destination route prefix for DUT to ATE traffic.
	dstPfx            = "198.51.100.0/24"
	dstPfxFlowIP      = "198.51.100.0"
	ipv4PrefixLen     = 30
	ipv4FlowCount     = 65000
	innerSrcIPv4Start = "198.18.0.0"
	innerDstIPv4Start = "198.19.0.0"

	// load balancing precision, %. Defines expected +-% delta for ECMP flows.
	// E.g. 48-52% with two equal-weighted NHs.
	lbPrecision = 2
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
		IPv4:    "192.0.2.2",
		IPv4Len: 30,
	}
	atePort2 = attrs.Attributes{
		Name:    "atePort2",
		IPv4:    "192.0.2.6",
		IPv4Len: 30,
	}
	atePort3 = attrs.Attributes{
		Name:    "atePort3",
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

	p2flow := createFlow("Port 1 to Port 2", ate, top, &atePort2)
	p3flow := createFlow("Port 1 to Port 3", ate, top, &atePort3)

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
			sp := gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, *deviations.StaticProtocolName)
			gnmi.Delete(t, dut, sp.Static(atePort2DummyIP.IPv4CIDR()).Config())
			gnmi.Delete(t, dut, sp.Static(atePort3DummyIP.IPv4CIDR()).Config())
		}
	}()

	gribi.BecomeLeader(t, gribic)
	dutP2 := dut.Port(t, "port2").Name()
	dutP3 := dut.Port(t, "port3").Name()

	t.Logf("Adding gribi routes and validating traffic forwarding via port %v and NH ID %v", dutP2, p2NHID)
	if deviations.GRIBIMACOverrideWithStaticARP(dut) || deviations.GRIBIMACOverrideStaticARPStaticRoute(dut) {
		addVIPRoute(ctx, t, gribic, p2NHID, dutP2, atePort2DummyIP.IPv4)
	} else {
		addVIPRoute(ctx, t, gribic, p2NHID, dutP2)
	}
	addDestinationRoute(ctx, t, gribic)
	validateTrafficFlows(t, ate, []*ondatra.Flow{p2flow}, []*ondatra.Flow{p3flow}, nil, pMACFilter)

	t.Logf("Adding a new NH via port %v with ID %v", dutP3, p3NHID)
	if deviations.GRIBIMACOverrideWithStaticARP(dut) || deviations.GRIBIMACOverrideStaticARPStaticRoute(dut) {
		addNH(ctx, t, dut, gribic, p3NHID, dutP3, pMAC, atePort3DummyIP.IPv4)
	} else {
		addNH(ctx, t, dut, gribic, p3NHID, dutP3, pMAC)
	}

	t.Logf("Performing implicit in-place replace with two next-hops (NH IDs: %v and %v)", p2NHID, p3NHID)
	addNHG(ctx, t, gribic, virtualIPNHGID, []uint64{p2NHID, p3NHID})
	validateTrafficFlows(t, ate, nil, nil, []*ondatra.Flow{p2flow, p3flow}, pMACFilter)

	t.Logf("Performing implicit in-place replace using the next-hop with ID %v", p3NHID)
	addNHG(ctx, t, gribic, virtualIPNHGID, []uint64{p3NHID})
	validateTrafficFlows(t, ate, []*ondatra.Flow{p3flow}, []*ondatra.Flow{p2flow}, nil, pMACFilter)

	t.Logf("Performing implicit in-place replace using the next-hop with ID %v", p2NHID)
	addNHG(ctx, t, gribic, virtualIPNHGID, []uint64{p2NHID})
	validateTrafficFlows(t, ate, []*ondatra.Flow{p2flow}, []*ondatra.Flow{p3flow}, nil, pMACFilter)
}

// addNH adds a GRIBI NH with a FIB ACK confirmation via Modify RPC.
func addNH(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, gribic *fluent.GRIBIClient, id uint64, intf, mac string, nhip ...string) {
	nh := fluent.NextHopEntry().WithNetworkInstance(*deviations.DefaultNetworkInstance).
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

// addNHG adds a GRIBI NHG with a FIB ACK confirmation via Modify RPC.
func addNHG(ctx context.Context, t *testing.T, gribic *fluent.GRIBIClient, id uint64, nhs []uint64) {
	nhg := fluent.NextHopGroupEntry().WithNetworkInstance(*deviations.DefaultNetworkInstance).
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
func addDestinationRoute(ctx context.Context, t *testing.T, gribic *fluent.GRIBIClient) {
	dnh := fluent.NextHopEntry().WithNetworkInstance(*deviations.DefaultNetworkInstance).
		WithIndex(dstNHID).WithIPAddress(virtualIP)
	dnhg := fluent.NextHopGroupEntry().WithNetworkInstance(*deviations.DefaultNetworkInstance).
		WithID(dstNHGID).AddNextHop(dstNHID, 1)
	dpfx := fluent.IPv4Entry().WithNetworkInstance(vrfName).WithPrefix(dstPfx).WithNextHopGroup(dstNHGID).WithNextHopGroupNetworkInstance(*deviations.DefaultNetworkInstance)

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
func addVIPRoute(ctx context.Context, t *testing.T, gribic *fluent.GRIBIClient, id uint64, port string, nhip ...string) {
	inh := fluent.NextHopEntry().WithNetworkInstance(*deviations.DefaultNetworkInstance).
		WithIndex(id).WithInterfaceRef(port).WithMacAddress(pMAC)
	inhg := fluent.NextHopGroupEntry().WithNetworkInstance(*deviations.DefaultNetworkInstance).
		WithID(virtualIPNHGID).AddNextHop(id, 1)
	ipfx := fluent.IPv4Entry().WithNetworkInstance(*deviations.DefaultNetworkInstance).
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

func configureATE(t *testing.T, ate *ondatra.ATEDevice) *ondatra.ATETopology {
	top := ate.Topology().New()

	p1 := ate.Port(t, "port1")
	p2 := ate.Port(t, "port2")
	p3 := ate.Port(t, "port3")

	atePort1.AddToATE(top, p1, &dutPort1)
	atePort2.AddToATE(top, p2, &dutPort2)
	atePort3.AddToATE(top, p3, &dutPort3)

	top.Push(t).StartProtocols(t)

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
	if *deviations.InterfaceConfigVrfBeforeAddress {
		gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(vrfName).Config(), vrf)
	}

	gnmi.Update(t, dut, d.Interface(p1.Name()).Config(), dutPort1.NewOCInterface(p1.Name()))

	if !*deviations.InterfaceConfigVrfBeforeAddress {
		gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(vrfName).Config(), vrf)
	}

	gnmi.Update(t, dut, d.Interface(p2.Name()).Config(), dutPort2.NewOCInterface(p2.Name()))
	gnmi.Update(t, dut, d.Interface(p3.Name()).Config(), dutPort3.NewOCInterface(p3.Name()))
	if deviations.ExplicitIPv6EnableForGRIBI(dut) {
		gnmi.Update(t, dut, d.Interface(p2.Name()).Subinterface(0).Ipv6().Enabled().Config(), true)
		gnmi.Update(t, dut, d.Interface(p3.Name()).Subinterface(0).Ipv6().Enabled().Config(), true)
	}
	if *deviations.ExplicitPortSpeed {
		fptest.SetPortSpeed(t, p1)
		fptest.SetPortSpeed(t, p2)
		fptest.SetPortSpeed(t, p3)
	}
	if *deviations.ExplicitInterfaceInDefaultVRF {
		fptest.AssignToNetworkInstance(t, dut, p2.Name(), *deviations.DefaultNetworkInstance, 0)
		fptest.AssignToNetworkInstance(t, dut, p3.Name(), *deviations.DefaultNetworkInstance, 0)
	}
	if deviations.ExplicitGRIBIUnderNetworkInstance(dut) {
		fptest.EnableGRIBIUnderNetworkInstance(t, dut, *deviations.DefaultNetworkInstance)
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
	gnmi.Update(t, dut, gnmi.OC().Interface(p2.Name()).Config(), dutPort2DummyIP.NewOCInterface(p2.Name()))
	gnmi.Update(t, dut, gnmi.OC().Interface(p3.Name()).Config(), dutPort3DummyIP.NewOCInterface(p3.Name()))
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
	sp := gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, *deviations.StaticProtocolName)
	gnmi.Replace(t, dut, sp.Static(atePort2DummyIP.IPv4CIDR()).Config(), s2)
	gnmi.Replace(t, dut, sp.Static(atePort3DummyIP.IPv4CIDR()).Config(), s3)
	gnmi.Update(t, dut, gnmi.OC().Interface(p2.Name()).Config(), configStaticArp(p2, atePort2DummyIP.IPv4, pMAC))
	gnmi.Update(t, dut, gnmi.OC().Interface(p3.Name()).Config(), configStaticArp(p3, atePort3DummyIP.IPv4, pMAC))
}

// createFlow returns a flow from atePort1 to the dstPfx, expected to arrive on ATE interface dsts.
func createFlow(name string, ate *ondatra.ATEDevice, ateTop *ondatra.ATETopology, dsts ...*attrs.Attributes) *ondatra.Flow {
	ipv4Header := ondatra.NewIPv4Header()
	ipv4Header.WithSrcAddress(atePort1.IPv4)
	ipv4Header.WithDstAddress(dstPfxFlowIP)
	innerIpv4Header := ondatra.NewIPv4Header()
	innerIpv4Header.SrcAddressRange().WithMin(innerSrcIPv4Start).WithCount(ipv4FlowCount).WithStep("0.0.0.1")
	innerIpv4Header.DstAddressRange().WithMin(innerDstIPv4Start).WithCount(ipv4FlowCount).WithStep("0.0.0.1")

	endpoints := []ondatra.Endpoint{}
	for _, dst := range dsts {
		endpoints = append(endpoints, ateTop.Interfaces()[dst.Name])
	}

	flow := ate.Traffic().NewFlow(name).
		WithSrcEndpoints(ateTop.Interfaces()[atePort1.Name]).
		WithDstEndpoints(endpoints...).
		WithHeaders(ondatra.NewEthernetHeader(), ipv4Header, innerIpv4Header)
	flow.EgressTracking().WithOffset(33).WithWidth(15)
	return flow
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

// validateTrafficFlows starts traffic and ensures that good flows have 0% loss (50% in case of LB)
// and the correct destination MAC, and bad flows have 100% loss.
func validateTrafficFlows(t *testing.T, ate *ondatra.ATEDevice, good, bad, lb []*ondatra.Flow, macFilter string) {
	if len(good) == 0 && len(bad) == 0 && len(lb) == 0 {
		return
	}

	flows := append(good, bad...)
	ate.Traffic().Start(t, flows...)
	time.Sleep(15 * time.Second)
	ate.Traffic().Stop(t)

	for _, flow := range good {
		flowPath := gnmi.OC().Flow(flow.Name())
		if got := gnmi.Get(t, ate, flowPath.LossPct().State()); got > 0 {
			t.Fatalf("LossPct for flow %s: got %g, want 0", flow.Name(), got)
		}
		etPath := flowPath.EgressTrackingAny()
		ets := gnmi.GetAll(t, ate, etPath.State())
		if got := len(ets); got != 1 {
			t.Errorf("EgressTracking got %d items, want %d", got, 1)
			return
		}
		if got := ets[0].GetFilter(); got != macFilter {
			t.Errorf("EgressTracking filter got %q, want %q", got, macFilter)
		}
		inPkts := gnmi.Get(t, ate, flowPath.State()).GetCounters().GetInPkts()
		if got := ets[0].GetCounters().GetInPkts(); got != inPkts {
			t.Errorf("EgressTracking counter in-pkts got %d, want %d", got, inPkts)
		}
	}
	for _, flow := range lb {
		// for LB flows, we expect to receive between 48-52% of packets on each interface (before and after filtering).
		lbPct := 50.0
		flowPath := gnmi.OC().Flow(flow.Name())
		if diff := cmp.Diff(float32(lbPct), gnmi.Get(t, ate, flowPath.LossPct().State()), cmpopts.EquateApprox(0, lbPrecision)); diff != "" {
			t.Errorf("Received number of packets -want,+got:\n%s", diff)
		}
		etPath := flowPath.EgressTrackingAny()
		ets := gnmi.GetAll(t, ate, etPath.State())
		if got := len(ets); got != 1 {
			t.Errorf("EgressTracking got %d items, want %d", got, 1)
			return
		}
		if got := ets[0].GetFilter(); got != macFilter {
			t.Errorf("EgressTracking filter got %q, want %q", got, macFilter)
		}
		inPkts := gnmi.Get(t, ate, flowPath.State()).GetCounters().GetInPkts()
		if diff := cmp.Diff(inPkts, ets[0].GetCounters().GetInPkts(), cmpopts.EquateApprox(lbPct, lbPrecision)); diff != "" {
			t.Errorf("EgressTracking received number of packets -want,+got:\n%s", diff)
		}
	}
	for _, flow := range bad {
		if got := gnmi.Get(t, ate, gnmi.OC().Flow(flow.Name()).LossPct().State()); got < 100 {
			t.Fatalf("LossPct for flow %s: got %g, want 100", flow.Name(), got)
		}
	}
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
