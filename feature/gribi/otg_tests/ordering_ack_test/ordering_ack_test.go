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

package ordering_ack_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/gribigo/chk"
	"github.com/openconfig/gribigo/constants"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Settings for configuring the baseline testbed with the test
// topology.  IxNetwork flow requires both source and destination
// networks be configured on the ATE.  It is not possible to send
// packets to the ether.
//
// The testbed consists of ate:port1 -> dut:port1 and
// dut:port2 -> ate:port2.  The first pair is called the "source"
// pair, and the second the "destination" pair.
//
//   - Source: ate:port1 -> dut:port1 subnet 192.0.2.0/30
//   - Destination: dut:port2 -> ate:port2 subnet 192.0.2.4/30
//
// A traffic flow from a source network is configured to be sent from
// ate:port1, with a destination network expected to be received at
// ate:port{2-9}.
//
//   - Source network: 198.51.100.0/24 (TEST-NET-2)
//   - Destination network: 203.0.113.0/24 (TEST-NET-3)
const (
	plen4 = 30

	ateDstNetName         = "dstnet"
	ateDstNetCIDR         = "203.0.113.0/24"
	ateDstNetStartIp      = "203.0.113.1"
	ateDstNetAddressCount = 250

	nhIndex  = 42
	nhWeight = 1
	nhgIndex = 10

	awaitDuration = 2 * time.Minute
)

var (
	ateSrc = attrs.Attributes{
		Name:    "ateSrc",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.0.2.1",
		IPv4Len: plen4,
	}

	dutSrc = attrs.Attributes{
		Desc:    "DUT to ATE source",
		IPv4:    "192.0.2.2",
		IPv4Len: plen4,
	}

	dutDst = attrs.Attributes{
		Desc:    "DUT to ATE destination",
		IPv4:    "192.0.2.5",
		IPv4Len: plen4,
	}

	ateDst = attrs.Attributes{
		Name:    "dst",
		MAC:     "02:00:02:01:01:01",
		IPv4:    "192.0.2.6",
		IPv4Len: plen4,
	}
)

// configInterfaceDUT configures the interface with the Addrs.
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
	s4a.PrefixLength = ygot.Uint8(plen4)

	return i
}

// configureDUT configures port1 and port2 on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	d := gnmi.OC()

	p1 := dut.Port(t, "port1")
	i1 := &oc.Interface{Name: ygot.String(p1.Name())}
	gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), configInterfaceDUT(i1, &dutSrc, dut))

	p2 := dut.Port(t, "port2")
	i2 := &oc.Interface{Name: ygot.String(p2.Name())}
	gnmi.Replace(t, dut, d.Interface(p2.Name()).Config(), configInterfaceDUT(i2, &dutDst, dut))

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, p1)
		fptest.SetPortSpeed(t, p2)
	}
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, p1.Name(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, p2.Name(), deviations.DefaultNetworkInstance(dut), 0)
	}
}

// configureATE configures port1 and port2 on the ATE.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	top := gosnappi.NewConfig()

	top.Ports().Add().SetName(ate.Port(t, "port1").ID())
	i1 := top.Devices().Add().SetName(ate.Port(t, "port1").ID())
	eth1 := i1.Ethernets().Add().SetName(ateSrc.Name + ".Eth").SetMac(ateSrc.MAC)
	eth1.Connection().SetPortName(i1.Name())
	eth1.Ipv4Addresses().Add().SetName(ateSrc.Name + ".IPv4").
		SetAddress(ateSrc.IPv4).SetGateway(dutSrc.IPv4).
		SetPrefix(uint32(ateSrc.IPv4Len))

	top.Ports().Add().SetName(ate.Port(t, "port2").ID())
	i2 := top.Devices().Add().SetName(ate.Port(t, "port2").ID())
	eth2 := i2.Ethernets().Add().SetName(ateDst.Name + ".Eth").SetMac(ateDst.MAC)
	eth2.Connection().SetPortName(i2.Name())
	eth2.Ipv4Addresses().Add().SetName(ateDst.Name + ".IPv4").
		SetAddress(ateDst.IPv4).SetGateway(dutDst.IPv4).
		SetPrefix(uint32(ateDst.IPv4Len))

	return top
}

// testTraffic generates traffic flow from source network to
// destination network via ate:port1 to ate:port2 and checks for
// packet loss.

func testTraffic(
	t *testing.T,
	ate *ondatra.ATEDevice,
	top gosnappi.Config,
) {
	otg := ate.OTG()
	flowName := "Flow"
	otgutils.WaitForARP(t, otg, top, "IPv4")
	dstMac := gnmi.Get(t, otg, gnmi.OTG().Interface(ateSrc.Name+".Eth").Ipv4Neighbor(dutSrc.IPv4).LinkLayerAddress().State())
	top.Flows().Clear().Items()
	flowipv4 := top.Flows().Add().SetName("Flow")
	flowipv4.Metrics().SetEnable(true)
	flowipv4.TxRx().Port().
		SetTxName(ate.Port(t, "port1").ID()).
		SetRxName(ate.Port(t, "port2").ID())
	flowipv4.Duration().Continuous()
	e1 := flowipv4.Packet().Add().Ethernet()
	e1.Src().SetValue(ateSrc.MAC)
	e1.Dst().SetValue(dstMac)
	v4 := flowipv4.Packet().Add().Ipv4()
	v4.Src().SetValue(ateSrc.IPv4)
	v4.Dst().Increment().SetStart(ateDstNetStartIp).SetCount(ateDstNetAddressCount)
	otg.PushConfig(t, top)
	otg.StartProtocols(t)

	otg.StartTraffic(t)
	time.Sleep(15 * time.Second)
	t.Logf("Stop traffic")
	otg.StopTraffic(t)

	otgutils.LogFlowMetrics(t, otg, top)
	txPkts := float32(gnmi.Get(t, otg, gnmi.OTG().Flow("Flow").Counters().OutPkts().State()))
	rxPkts := float32(gnmi.Get(t, otg, gnmi.OTG().Flow("Flow").Counters().InPkts().State()))
	if txPkts == 0 {
		t.Fatalf("TxPkts == 0, want > 0")
	}

	if got := (txPkts - rxPkts) * 100 / txPkts; got > 0 {
		t.Errorf("LossPct for flow %s got %v, want 0", flowName, got)
	}
}

// awaitTimeout calls a fluent client Await, adding a timeout to the context.
func awaitTimeout(ctx context.Context, c *fluent.GRIBIClient, t testing.TB) error {
	subctx, cancel := context.WithTimeout(ctx, awaitDuration)
	defer cancel()
	return c.Await(subctx, t)
}

// testArgs holds the objects needed by a test case.
type testArgs struct {
	ctx           context.Context
	c             *fluent.GRIBIClient
	dut           *ondatra.DUTDevice
	ate           *ondatra.ATEDevice
	top           gosnappi.Config
	wantInstalled fluent.ProgrammingResult
}

// testCaseFunc describes a test case function.
type testCaseFunc func(t *testing.T, args *testArgs)

// testModifyNHG configures a NextHopGroup referencing a NextHop.
func testModifyNHG(t *testing.T, args *testArgs) {
	args.c.Modify().AddEntry(t,
		fluent.NextHopEntry().
			WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
			WithIndex(nhIndex).
			WithIPAddress(ateDst.IPv4),
		fluent.NextHopGroupEntry().
			WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
			WithID(nhgIndex).
			AddNextHop(nhIndex, nhWeight),
	)
	if err := awaitTimeout(args.ctx, args.c, t); err != nil {
		t.Errorf("Await got error for ModifyRequest: %v", err)
	}

	res := args.c.Results(t)
	chk.HasResult(t, res,
		fluent.OperationResult().
			WithOperationID(1).
			WithOperationType(constants.Add).
			WithNextHopOperation(nhIndex).
			WithProgrammingResult(args.wantInstalled).
			AsResult(),
	)
	chk.HasResult(t, res,
		fluent.OperationResult().
			WithOperationID(2).
			WithOperationType(constants.Add).
			WithNextHopGroupOperation(nhgIndex).
			WithProgrammingResult(args.wantInstalled).
			AsResult(),
	)

	t.Run("Telemetry", func(t *testing.T) {
		got, err := aftNextHopWeights(t, args.dut, nhgIndex, deviations.DefaultNetworkInstance(args.dut))
		if err != nil {
			t.Errorf("Error getting weights for nhg %d : %v", nhIndex, err)
		} else {
			want := []uint64{nhWeight}
			// when a next hop group (nhg) has only one next hop, some FIB implemenation map the nhg to a single path and ignore the weight.
			// In this case, AFT may returns no value or zero as weight, so validate weights only for nhg with more than one nh.
			if len(want) > 1 {
				ok := cmp.Equal(want, got, cmpopts.SortSlices(func(a, b uint64) bool { return a < b }))
				if !ok {
					t.Errorf("next-hop-group/next-hop/state/weight got %v, want %v", got, want)
				}
			}
		}
	})
}

// testModifyIPv4NHG configures a ModifyRequest with a NextHop and an IPv4Entry before a
// NextHopGroup which is invalid due to the forward reference.
func testModifyIPv4NHG(t *testing.T, args *testArgs) {
	args.c.Modify().AddEntry(t,
		fluent.NextHopEntry().
			WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
			WithIndex(nhIndex).
			WithIPAddress(ateDst.IPv4),
		fluent.IPv4Entry().
			WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
			WithPrefix(ateDstNetCIDR).
			WithNextHopGroup(nhgIndex),
		fluent.NextHopGroupEntry().
			WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
			WithID(nhgIndex).
			AddNextHop(nhIndex, nhWeight),
	)
	if err := awaitTimeout(args.ctx, args.c, t); err != nil {
		t.Fatalf("Await got error for ModifyRequest: %v", err)
	}

	res := args.c.Results(t)
	chk.HasResult(t, res,
		fluent.OperationResult().
			WithOperationID(2).
			WithOperationType(constants.Add).
			WithIPv4Operation(ateDstNetCIDR).
			WithProgrammingResult(fluent.ProgrammingFailed).
			AsResult(),
	)
}

// testModifyNHGIPv4 configures a ModifyRequest with a NextHopGroup and IPv4Entry.
func testModifyNHGIPv4(t *testing.T, args *testArgs) {
	args.c.Modify().AddEntry(t,
		fluent.NextHopEntry().
			WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
			WithIndex(nhIndex).
			WithIPAddress(ateDst.IPv4),
		fluent.NextHopGroupEntry().
			WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
			WithID(nhgIndex).
			AddNextHop(nhIndex, nhWeight),
		fluent.IPv4Entry().
			WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
			WithPrefix(ateDstNetCIDR).
			WithNextHopGroup(nhgIndex),
	)
	if err := awaitTimeout(args.ctx, args.c, t); err != nil {
		t.Fatalf("Await got error for ModifyRequest: %v", err)
	}

	res := args.c.Results(t)
	chk.HasResult(t, res,
		fluent.OperationResult().
			WithOperationID(1).
			WithOperationType(constants.Add).
			WithNextHopOperation(nhIndex).
			WithProgrammingResult(args.wantInstalled).
			AsResult(),
	)
	chk.HasResult(t, res,
		fluent.OperationResult().
			WithOperationID(2).
			WithOperationType(constants.Add).
			WithNextHopGroupOperation(nhgIndex).
			WithProgrammingResult(args.wantInstalled).
			AsResult(),
	)
	chk.HasResult(t, res,
		fluent.OperationResult().
			WithOperationID(3).
			WithOperationType(constants.Add).
			WithIPv4Operation(ateDstNetCIDR).
			WithProgrammingResult(args.wantInstalled).
			AsResult(),
	)

	t.Run("Telemetry", func(t *testing.T) {
		got, err := aftNextHopWeights(t, args.dut, nhgIndex, deviations.DefaultNetworkInstance(args.dut))
		if err != nil {
			t.Errorf("Error getting weights for nhg %d : %v", nhIndex, err)
		} else {
			want := []uint64{nhWeight}
			// if a next hop group (nhg) has only one next hop, most FIB implemenation map the nhg to a single path and ignore the weight.
			// In this case, AFT may returns no value or zero as weight, so validate weights only for nhg with more than one nh.
			if len(want) > 1 {
				ok := cmp.Equal(want, got, cmpopts.SortSlices(func(a, b uint64) bool { return a < b }))
				if !ok {
					t.Errorf("next-hop-group/next-hop/state/weight got %v, want %v", got, want)
				}
			}
			ipv4Path := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Afts().Ipv4Entry(ateDstNetCIDR)
			if got, want := gnmi.Get(t, args.dut, ipv4Path.State()).GetPrefix(), ateDstNetCIDR; got != want {
				t.Errorf("ipv4-entry/state/prefix got %s, want %s", got, want)
			}
		}
	})

	t.Run("Traffic", func(t *testing.T) {
		testTraffic(t, args.ate, args.top)
	})
}

// aftNextHopWeights queries AFT telemetry using Get() and returns
// the weights. If not-found, an empty list is returned.
func aftNextHopWeights(t *testing.T, dut *ondatra.DUTDevice, nhg uint64, networkInstance string) ([]uint64, error) {
	aft := gnmi.Get(t, dut, gnmi.OC().NetworkInstance(networkInstance).Afts().State())
	var nhgD *oc.NetworkInstance_Afts_NextHopGroup
	for _, nhgData := range aft.NextHopGroup {
		if nhgData.GetProgrammedId() == nhg {
			nhgD = nhgData
			break
		}
	}
	if nhgD == nil {
		return []uint64{}, fmt.Errorf("next-hop-group with programing id %d is not found in AFT response", nhg)
	}

	got := []uint64{}
	for _, nhD := range nhgD.NextHop {
		got = append(got, nhD.GetWeight())
	}

	return got, nil
}

// testModifyIPv4AddDelAdd configures a ModifyRequest with AFT operations to add, delete,
// and add IPv4Entry.
func testModifyIPv4AddDelAdd(t *testing.T, args *testArgs) {
	testModifyNHG(t, args) // Uses operation IDs 1 and 2.

	ent := fluent.IPv4Entry().
		WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
		WithPrefix(ateDstNetCIDR).
		WithNextHopGroup(nhgIndex)

	args.c.Modify().
		AddEntry(t, ent).
		DeleteEntry(t, ent).
		AddEntry(t, ent)
	if err := awaitTimeout(args.ctx, args.c, t); err != nil {
		t.Fatalf("Await got error for ModifyRequest: %v", err)
	}

	res := args.c.Results(t)
	chk.HasResult(t, res,
		fluent.OperationResult().
			WithOperationID(3).
			WithOperationType(constants.Add).
			WithIPv4Operation(ateDstNetCIDR).
			WithProgrammingResult(args.wantInstalled).
			AsResult(),
	)
	chk.HasResult(t, res,
		fluent.OperationResult().
			WithOperationID(4).
			WithOperationType(constants.Delete).
			WithIPv4Operation(ateDstNetCIDR).
			WithProgrammingResult(args.wantInstalled).
			AsResult(),
	)
	chk.HasResult(t, res,
		fluent.OperationResult().
			WithOperationID(5).
			WithOperationType(constants.Add).
			WithIPv4Operation(ateDstNetCIDR).
			WithProgrammingResult(args.wantInstalled).
			AsResult(),
	)

	t.Run("Telemetry", func(t *testing.T) {
		ipv4Path := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Afts().Ipv4Entry(ateDstNetCIDR)
		if got, want := gnmi.Get(t, args.dut, ipv4Path.State()).GetPrefix(), ateDstNetCIDR; got != want {
			t.Errorf("ipv4-entry/state/prefix got %s, want %s", got, want)
		}
	})

	t.Run("Traffic", func(t *testing.T) {
		testTraffic(t, args.ate, args.top)
	})
}

var cases = []struct {
	name string
	desc string
	fn   testCaseFunc
}{
	{
		name: "Modify NHG",
		desc: "A NextHopGroup referencing a NextHop is responded to with RIB+FIB ACK, and is reported through the AFT telemetry.",
		fn:   testModifyNHG,
	},
	{
		name: "Modify IPv4 and NHG",
		desc: "A single ModifyRequest with the following ordered operations is responded to with an error: (1) An AFTOperation containing an IPv4Entry referencing NextHopGroup 10. (2) An AFTOperation containing a NextHopGroup id=10.",
		fn:   testModifyIPv4NHG,
	},
	{
		name: "Modify NHG and IPv4",
		desc: "A single ModifyRequest with the following ordered operations is installed (verified through telemetry and traffic): (1) An AFTOperation containing a NextHopGroup 10 pointing to a NextHop to ATE port-2. (2) An AFTOperation containing a IPv4Entry referencing NextHopGroup 10.",
		fn:   testModifyNHGIPv4,
	},
	{
		name: "Modify IPv4 Add Del Add",
		desc: "A single ModifyRequest with the following ordered operations is installed (verified through telemetry and traffic): (1) An AFT entry adding IPv4Entry 203.0.113.0/24. (2) An AFT entry deleting IPv4Entry 203.0.113.0/24. (3) An AFT entry adding IPv4Entry 203.0.113.0/24.",
		fn:   testModifyIPv4AddDelAdd,
	},
}

func TestOrderingACK(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	// Dial gRIBI
	ctx := context.Background()
	gribic := dut.RawAPIs().GRIBI(t)

	// Configure the ATE
	ate := ondatra.ATE(t, "ate")
	top := configureATE(t, ate)
	ate.OTG().PushConfig(t, top)

	// Configure the DUT
	configureDUT(t, dut)

	ate.OTG().StartProtocols(t)

	const usePreserve = "PRESERVE"

	t.Run(fmt.Sprintf("Persistence=%s", usePreserve), func(t *testing.T) {
		// Each case will run with its own gRIBI fluent client.
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Logf("Name: %s", tc.name)
				t.Logf("Description: %s", tc.desc)

				// Configure the gRIBI client.
				c := fluent.NewClient()
				conn := c.Connection().
					WithStub(gribic).WithPersistence().
					WithRedundancyMode(fluent.ElectedPrimaryClient).
					WithInitialElectionID(1 /* low */, 0 /* hi */) // ID must be > 0.

				if !deviations.GRIBIRIBAckOnly(dut) {
					// The main difference WithFIBACK() made was that we are now expecting
					// fluent.InstalledInFIB in []*client.OpResult, as opposed to
					// fluent.InstalledInRIB.
					conn.WithFIBACK()
				}

				c.Start(ctx, t)
				defer c.Stop(t)
				c.StartSending(ctx, t)
				if err := awaitTimeout(ctx, c, t); err != nil {
					t.Fatalf("Await got error during session negotiation: %v", err)
				}
				gribi.BecomeLeader(t, c)

				defer func() {
					if err := gribi.FlushAll(c); err != nil {
						t.Errorf("Cannot flush: %v", err)
					}
				}()

				args := &testArgs{ctx: ctx, c: c, dut: dut, ate: ate, top: top}
				args.wantInstalled = fluent.InstalledInFIB
				if deviations.GRIBIRIBAckOnly(dut) {
					args.wantInstalled = fluent.InstalledInRIB
				}
				tc.fn(t, args)
			})
		}
	})
}
