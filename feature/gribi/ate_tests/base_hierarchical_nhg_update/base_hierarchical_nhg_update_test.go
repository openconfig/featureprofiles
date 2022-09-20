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
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/gribigo/chk"
	"github.com/openconfig/gribigo/client"
	"github.com/openconfig/gribigo/constants"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/ygot/ygot"
)

const (
	vrfName = "VRF-1"

	// Destination ATE MAC address for port-2 and port-3
	pMAC = "00:1A:11:00:00:01"

	// port-2 nexthop ID
	p2ID = 40
	// port-3 nexthop ID
	p3ID = 41

	// Interface route next-hop-group ID
	interfaceID = 42
	// Interface route nexthop IP
	interfaceNH = "203.0.113.1"
	// Interface route prefix
	interfacePfx = "203.0.113.1/32"

	// Destination route next-hop ID
	dstNHID = 43
	// Destination route next-hop-group ID
	dstNHGID = 44
	// Destination route prefix for DUT to ATE traffic.
	dstPfx      = "198.51.100.0/24"
	dstPfxMin   = "198.51.100.0"
	dstPfxMax   = "198.51.100.255"
	dstPfxCount = 256
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

	addInterfaceRoute(ctx, t, gribic, p2ID, dut.Port(t, "port2").Name(), atePort2.IPv4)
	addDestinationRoute(ctx, t, gribic)

	validateTrafficFlows(t, ate, []*ondatra.Flow{p2flow}, []*ondatra.Flow{p3flow})

	addInterfaceRoute(ctx, t, gribic, p3ID, dut.Port(t, "port3").Name(), atePort3.IPv4)

	validateTrafficFlows(t, ate, []*ondatra.Flow{p3flow}, []*ondatra.Flow{p2flow})
}

// addDestinationRoute creates a GRIBI route to dstPfx via interfaceNH.
func addDestinationRoute(ctx context.Context, t *testing.T, gribic *fluent.GRIBIClient) {
	dnh := fluent.NextHopEntry().WithNetworkInstance(*deviations.DefaultNetworkInstance).
		WithIndex(dstNHID).WithIPAddress(interfaceNH)
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

// addInterfaceRoute creates a GRIBI route that points to the egress interface defined by id,
// port, and nhip.
func addInterfaceRoute(ctx context.Context, t *testing.T, gribic *fluent.GRIBIClient, id uint64, port string, nhip string) {
	inh := fluent.NextHopEntry().WithNetworkInstance(*deviations.DefaultNetworkInstance).
		WithIndex(id).WithInterfaceRef(port).WithIPAddress(nhip).WithMacAddress(pMAC)
	inhg := fluent.NextHopGroupEntry().WithNetworkInstance(*deviations.DefaultNetworkInstance).
		WithID(interfaceID).AddNextHop(id, 1)
	ipfx := fluent.IPv4Entry().WithNetworkInstance(*deviations.DefaultNetworkInstance).
		WithPrefix(interfacePfx).WithNextHopGroup(interfaceID)

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
			WithNextHopGroupOperation(interfaceID).
			WithProgrammingResult(fluent.InstalledInFIB).
			WithOperationType(constants.Add).
			AsResult(),
		fluent.OperationResult().
			WithIPv4Operation(interfacePfx).
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
	d := dut.Config()

	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")
	p3 := dut.Port(t, "port3")

	vrf := &telemetry.NetworkInstance{
		Name:    ygot.String(vrfName),
		Enabled: ygot.Bool(true),
		Type:    telemetry.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF,
		EnabledAddressFamilies: []telemetry.E_Types_ADDRESS_FAMILY{
			telemetry.Types_ADDRESS_FAMILY_IPV4,
			telemetry.Types_ADDRESS_FAMILY_IPV6,
		},
	}

	p1VRF := vrf.GetOrCreateInterface(p1.Name())
	p1VRF.Interface = ygot.String(p1.Name())
	p1VRF.Subinterface = ygot.Uint32(0)
	dut.Config().NetworkInstance(vrfName).Replace(t, vrf)

	d.Interface(p1.Name()).Replace(t, dutPort1.NewInterface(p1.Name()))
	d.Interface(p2.Name()).Replace(t, dutPort2.NewInterface(p2.Name()))
	d.Interface(p3.Name()).Replace(t, dutPort3.NewInterface(p3.Name()))
}

// createFlow returns a flow from atePort1 to the dstPfx, expected to arrive on ATE interface dsts.
func createFlow(name string, ate *ondatra.ATEDevice, ateTop *ondatra.ATETopology, dsts ...*attrs.Attributes) *ondatra.Flow {
	hdr := ondatra.NewIPv4Header()
	hdr.WithSrcAddress(dutPort1.IPv4).
		DstAddressRange().WithMin(dstPfxMin).WithMax(dstPfxMax).WithCount(dstPfxCount)

	endpoints := []ondatra.Endpoint{}
	for _, dst := range dsts {
		endpoints = append(endpoints, ateTop.Interfaces()[dst.Name])
	}

	flow := ate.Traffic().NewFlow(name).
		WithSrcEndpoints(ateTop.Interfaces()[atePort1.Name]).
		WithDstEndpoints(endpoints...).
		WithHeaders(ondatra.NewEthernetHeader(), hdr)

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

// validateTrafficFlows starts traffic and ensures that good flows have 0% loss and bad flows have
// 100% loss.
//
// TODO: Packets should be validated to arrive at ATE with destination MAC pMAC.
func validateTrafficFlows(t *testing.T, ate *ondatra.ATEDevice, good []*ondatra.Flow, bad []*ondatra.Flow) {
	if len(good) == 0 && len(bad) == 0 {
		return
	}

	flows := append(good, bad...)
	ate.Traffic().Start(t, flows...)
	time.Sleep(15 * time.Second)
	ate.Traffic().Stop(t)

	for _, flow := range good {
		if got := ate.Telemetry().Flow(flow.Name()).LossPct().Get(t); got > 0 {
			t.Fatalf("LossPct for flow %s: got %g, want 0", flow.Name(), got)
		}
	}

	for _, flow := range bad {
		if got := ate.Telemetry().Flow(flow.Name()).LossPct().Get(t); got < 100 {
			t.Fatalf("LossPct for flow %s: got %g, want 100", flow.Name(), got)
		}
	}
}
