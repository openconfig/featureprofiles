// Copyright 2022 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ipv4_entry_test

import (
	"context"
	"fmt"
	"strings"
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
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	// Next-hop group ID for dstPfx
	nhgID = 42
	// Next-hop 1 ID for dutPort2
	nh1ID = 43
	// Next-hop 2 ID for dutPort3
	nh2ID = 44
	// Unconfigured next-hop ID
	badNH = 45
	// A destination MAC address set by gRIBI.
	staticDstMAC   = "02:00:00:00:00:01"
	ethernetCsmacd = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
)

const (
	// Destination prefix for DUT to ATE traffic.
	dstPfx      = "198.51.100.0/24"
	dstPfxMin   = "198.51.100.0"
	dstPfxMax   = "198.51.100.255"
	dstPfxCount = 256
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "DUT Port 1",
		IPv4:    "192.0.2.1",
		IPv4Len: 30,
	}
	dutPort2 = attrs.Attributes{
		Desc:    "DUT Port 2",
		IPv4:    "192.0.2.5",
		IPv4Len: 30,
	}
	dutPort3 = attrs.Attributes{
		Desc:    "DUT Port 3",
		IPv4:    "192.0.2.9",
		IPv4Len: 30,
	}

	atePort1 = attrs.Attributes{
		Name:    "port1",
		MAC:     "02:00:01:01:01:01",
		Desc:    "ATE Port 1",
		IPv4:    "192.0.2.2",
		IPv4Len: 30,
	}
	atePort2 = attrs.Attributes{
		Name:    "port2",
		MAC:     "02:00:02:01:01:01",
		Desc:    "ATE Port 2",
		IPv4:    "192.0.2.6",
		IPv4Len: 30,
	}
	atePort3 = attrs.Attributes{
		Name:    "port3",
		MAC:     "02:00:03:01:01:01",
		Desc:    "ATE Port 3",
		IPv4:    "192.0.2.10",
		IPv4Len: 30,
	}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// TestIPv4Entry tests a single IPv4Entry forwarding entry.
func TestIPv4Entry(t *testing.T) {
	ctx := context.Background()

	dut := ondatra.DUT(t, "dut")
	configureDUT(t, dut)

	gribic := dut.RawAPIs().GRIBI().Default(t)

	ate := ondatra.ATE(t, "ate")
	configureATE(t, ate)

	cases := []struct {
		desc                 string
		entries              []fluent.GRIBIEntry
		downPort             *ondatra.Port
		wantGoodFlows        []string
		wantBadFlows         []string
		wantOperationResults []*client.OpResult
	}{
		{
			desc: "Single next-hop",
			entries: []fluent.GRIBIEntry{
				fluent.NextHopEntry().WithNetworkInstance(*deviations.DefaultNetworkInstance).
					WithIndex(nh1ID).WithIPAddress(atePort2.IPv4),
				fluent.NextHopGroupEntry().WithNetworkInstance(*deviations.DefaultNetworkInstance).
					WithID(nhgID).AddNextHop(nh1ID, 1),
				fluent.IPv4Entry().WithNetworkInstance(*deviations.DefaultNetworkInstance).
					WithPrefix(dstPfx).WithNextHopGroup(nhgID),
			},
			wantGoodFlows: []string{"port2Flow"},
			wantBadFlows:  []string{"port3Flow"},
			wantOperationResults: []*client.OpResult{
				fluent.OperationResult().
					WithNextHopOperation(nh1ID).
					WithProgrammingResult(fluent.InstalledInFIB).
					WithOperationType(constants.Add).
					AsResult(),
				fluent.OperationResult().
					WithNextHopGroupOperation(nhgID).
					WithProgrammingResult(fluent.InstalledInFIB).
					WithOperationType(constants.Add).
					AsResult(),
				fluent.OperationResult().
					WithIPv4Operation(dstPfx).
					WithProgrammingResult(fluent.InstalledInFIB).
					WithOperationType(constants.Add).
					AsResult(),
			},
		},
		{
			desc: "Multiple next-hops",
			entries: []fluent.GRIBIEntry{
				fluent.NextHopEntry().WithNetworkInstance(*deviations.DefaultNetworkInstance).
					WithIndex(nh1ID).WithIPAddress(atePort2.IPv4),
				fluent.NextHopEntry().WithNetworkInstance(*deviations.DefaultNetworkInstance).
					WithIndex(nh2ID).WithIPAddress(atePort3.IPv4),
				fluent.NextHopGroupEntry().WithNetworkInstance(*deviations.DefaultNetworkInstance).
					WithID(nhgID).AddNextHop(nh1ID, 1).AddNextHop(nh2ID, 1),
				fluent.IPv4Entry().WithNetworkInstance(*deviations.DefaultNetworkInstance).
					WithPrefix(dstPfx).WithNextHopGroup(nhgID),
			},
			wantGoodFlows: []string{"ecmpFlow"},
			wantOperationResults: []*client.OpResult{
				fluent.OperationResult().
					WithNextHopOperation(nh1ID).
					WithProgrammingResult(fluent.InstalledInFIB).
					WithOperationType(constants.Add).
					AsResult(),
				fluent.OperationResult().
					WithNextHopOperation(nh2ID).
					WithProgrammingResult(fluent.InstalledInFIB).
					WithOperationType(constants.Add).
					AsResult(),
				fluent.OperationResult().
					WithNextHopGroupOperation(nhgID).
					WithProgrammingResult(fluent.InstalledInFIB).
					WithOperationType(constants.Add).
					AsResult(),
				fluent.OperationResult().
					WithIPv4Operation(dstPfx).
					WithProgrammingResult(fluent.InstalledInFIB).
					WithOperationType(constants.Add).
					AsResult(),
			},
		},
		{
			desc: "Multiple next-hops with MAC override",
			entries: []fluent.GRIBIEntry{
				fluent.NextHopEntry().WithNetworkInstance(*deviations.DefaultNetworkInstance).
					WithIndex(nh1ID).WithInterfaceRef(dut.Port(t, "port2").Name()).WithMacAddress(staticDstMAC),
				fluent.NextHopEntry().WithNetworkInstance(*deviations.DefaultNetworkInstance).
					WithIndex(nh2ID).WithInterfaceRef(dut.Port(t, "port3").Name()).WithMacAddress(staticDstMAC),
				fluent.NextHopGroupEntry().WithNetworkInstance(*deviations.DefaultNetworkInstance).
					WithID(nhgID).AddNextHop(nh1ID, 1).AddNextHop(nh2ID, 1),
				fluent.IPv4Entry().WithNetworkInstance(*deviations.DefaultNetworkInstance).
					WithPrefix(dstPfx).WithNextHopGroup(nhgID),
			},
			wantGoodFlows: []string{"ecmpFlow"},
			wantOperationResults: []*client.OpResult{
				fluent.OperationResult().
					WithNextHopOperation(nh1ID).
					WithProgrammingResult(fluent.InstalledInFIB).
					WithOperationType(constants.Add).
					AsResult(),
				fluent.OperationResult().
					WithNextHopOperation(nh2ID).
					WithProgrammingResult(fluent.InstalledInFIB).
					WithOperationType(constants.Add).
					AsResult(),
				fluent.OperationResult().
					WithNextHopGroupOperation(nhgID).
					WithProgrammingResult(fluent.InstalledInFIB).
					WithOperationType(constants.Add).
					AsResult(),
				fluent.OperationResult().
					WithIPv4Operation(dstPfx).
					WithProgrammingResult(fluent.InstalledInFIB).
					WithOperationType(constants.Add).
					AsResult(),
			},
		},
		{
			desc: "Nonexistant next-hop",
			entries: []fluent.GRIBIEntry{
				fluent.NextHopGroupEntry().WithNetworkInstance(*deviations.DefaultNetworkInstance).
					WithID(nhgID).AddNextHop(badNH, 1),
				fluent.IPv4Entry().WithNetworkInstance(*deviations.DefaultNetworkInstance).
					WithPrefix(dstPfx).WithNextHopGroup(nhgID),
			},
			wantBadFlows: []string{"port2Flow", "port3Flow"},
			wantOperationResults: []*client.OpResult{
				fluent.OperationResult().
					WithNextHopGroupOperation(nhgID).
					WithProgrammingResult(fluent.ProgrammingFailed).
					WithOperationType(constants.Add).
					AsResult(),
				fluent.OperationResult().
					WithIPv4Operation(dstPfx).
					WithProgrammingResult(fluent.ProgrammingFailed).
					WithOperationType(constants.Add).
					AsResult(),
			},
		},
		{
			// ate port link cannot be set to down in kne, therefore the downPort is a dut port
			desc:     "Downed next-hop interface",
			downPort: dut.Port(t, "port2"),
			entries: []fluent.GRIBIEntry{
				fluent.NextHopEntry().WithNetworkInstance(*deviations.DefaultNetworkInstance).
					WithIndex(nh1ID).WithIPAddress(atePort2.IPv4).
					WithInterfaceRef(dut.Port(t, "port2").Name()).WithMacAddress(staticDstMAC),
				fluent.NextHopGroupEntry().WithNetworkInstance(*deviations.DefaultNetworkInstance).
					WithID(nhgID).AddNextHop(nh1ID, 1),
				fluent.IPv4Entry().WithNetworkInstance(*deviations.DefaultNetworkInstance).
					WithPrefix(dstPfx).WithNextHopGroup(nhgID),
			},
			wantBadFlows: []string{"port2Flow", "port3Flow"},
			wantOperationResults: []*client.OpResult{
				fluent.OperationResult().
					WithNextHopOperation(nh1ID).
					WithProgrammingResult(fluent.InstalledInFIB).
					WithOperationType(constants.Add).
					AsResult(),
				fluent.OperationResult().
					WithNextHopGroupOperation(nhgID).
					WithProgrammingResult(fluent.InstalledInFIB).
					WithOperationType(constants.Add).
					AsResult(),
				fluent.OperationResult().
					WithIPv4Operation(dstPfx).
					WithProgrammingResult(fluent.InstalledInFIB).
					WithOperationType(constants.Add).
					AsResult(),
			},
		},
	}

	const (
		usePreserve = "PRESERVE"
	)

	// Each case will run with its own gRIBI fluent client.
	for _, persist := range []string{usePreserve} {
		t.Run(fmt.Sprintf("Persistence=%s", persist), func(t *testing.T) {

			for _, tc := range cases {
				t.Run(tc.desc, func(t *testing.T) {
					// Configure the gRIBI client.
					c := fluent.NewClient()
					conn := c.Connection().
						WithStub(gribic).
						WithRedundancyMode(fluent.ElectedPrimaryClient).
						WithInitialElectionID(1 /* low */, 0 /* hi */) // ID must be > 0.
					if persist == usePreserve {
						conn.WithPersistence()
					}

					if !deviations.GRIBIRIBAckOnly(dut) {
						// The main difference WithFIBACK() made was that we are now expecting
						// fluent.InstalledInFIB in []*client.OpResult, as opposed to
						// fluent.InstalledInRIB.
						conn.WithFIBACK()
					}

					c.Start(ctx, t)
					defer c.Stop(t)
					c.StartSending(ctx, t)
					if err := awaitTimeout(ctx, c, t, time.Minute); err != nil {
						t.Fatalf("Await got error during session negotiation: %v", err)
					}
					gribi.BecomeLeader(t, c)

					if persist == usePreserve {
						defer func() {
							if err := gribi.FlushAll(c); err != nil {
								t.Errorf("Cannot flush: %v", err)
							}
						}()
					}

					if tc.downPort != nil {
						// Setting admin state down on the DUT interface.
						// Setting the otg interface down has no effect on kne and is not yet supported in otg
						setDUTInterfaceWithState(t, dut, &dutPort2, tc.downPort, false)
						defer setDUTInterfaceWithState(t, dut, &dutPort2, tc.downPort, true)
					}

					c.Modify().AddEntry(t, tc.entries...)
					if err := awaitTimeout(ctx, c, t, time.Minute); err != nil {
						t.Fatalf("Await got error for entries: %v", err)
					}
					defer func() {
						// Delete should reverse the order of entries, i.e. IPv4Entry must be removed
						// before NextHopGroupEntry, which must be removed before NextHopEntry.
						var revEntries []fluent.GRIBIEntry
						for i := len(tc.entries) - 1; i >= 0; i-- {
							revEntries = append(revEntries, tc.entries[i])
						}
						c.Modify().DeleteEntry(t, revEntries...)
						if err := awaitTimeout(ctx, c, t, time.Minute); err != nil {
							t.Fatalf("Await got error for entries: %v", err)
						}
					}()

					for _, wantResult := range tc.wantOperationResults {
						chk.HasResult(t, c.Results(t), wantResult, chk.IgnoreOperationID())
					}

					validateTrafficFlows(t, ate, tc.wantGoodFlows, tc.wantBadFlows)
				})
			}
		})
	}
}

// configureDUT configures port1-3 on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	d := gnmi.OC()

	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")
	p3 := dut.Port(t, "port3")

	gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), dutPort1.NewOCInterface(p1.Name()))
	gnmi.Replace(t, dut, d.Interface(p2.Name()).Config(), dutPort2.NewOCInterface(p2.Name()))
	gnmi.Replace(t, dut, d.Interface(p3.Name()).Config(), dutPort3.NewOCInterface(p3.Name()))

	if deviations.ExplicitIPv6EnableForGRIBI(dut) {
		gnmi.Update(t, dut, d.Interface(p2.Name()).Subinterface(0).Ipv6().Enabled().Config(), bool(true))
		gnmi.Update(t, dut, d.Interface(p3.Name()).Subinterface(0).Ipv6().Enabled().Config(), bool(true))
	}

	if *deviations.ExplicitPortSpeed {
		fptest.SetPortSpeed(t, p1)
		fptest.SetPortSpeed(t, p2)
		fptest.SetPortSpeed(t, p3)
	}
	if *deviations.ExplicitInterfaceInDefaultVRF {
		fptest.AssignToNetworkInstance(t, dut, p1.Name(), *deviations.DefaultNetworkInstance, 0)
		fptest.AssignToNetworkInstance(t, dut, p2.Name(), *deviations.DefaultNetworkInstance, 0)
		fptest.AssignToNetworkInstance(t, dut, p3.Name(), *deviations.DefaultNetworkInstance, 0)
	}
}

// configreATE configures port1-3 on the ATE.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	top := ate.OTG().NewConfig(t)

	p1 := ate.Port(t, "port1")
	p2 := ate.Port(t, "port2")
	p3 := ate.Port(t, "port3")

	atePort1.AddToOTG(top, p1, &dutPort1)
	atePort2.AddToOTG(top, p2, &dutPort2)
	atePort3.AddToOTG(top, p3, &dutPort3)

	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)

	return top
}

// createFlow returns a flow from atePort1 to the dstPfx, expected to arrive on ATE interface dsts.
func createFlow(t *testing.T, name string, ate *ondatra.ATEDevice, ateTop gosnappi.Config, dsts ...*attrs.Attributes) string {

	// Multiple devices is not supported on the OTG flows
	modName := strings.Replace(name, " ", "_", -1)
	var rxEndpoints []string
	for _, dst := range dsts {
		rxEndpoints = append(rxEndpoints, dst.Name+".IPv4")
	}
	otg := ate.OTG()
	flowipv4 := ateTop.Flows().Add().SetName(modName)
	flowipv4.Metrics().SetEnable(true)
	e1 := flowipv4.Packet().Add().Ethernet()
	e1.Src().SetValue(atePort1.MAC)
	if len(dsts) > 1 {
		flowipv4.TxRx().Port().SetTxName(atePort1.Name)
		waitOTGARPEntry(t)
		dstMac := gnmi.Get(t, otg, gnmi.OTG().Interface(atePort1.Name+".Eth").Ipv4Neighbor(dutPort1.IPv4).LinkLayerAddress().State())
		e1.Dst().SetChoice("value").SetValue(dstMac)
	} else {
		flowipv4.TxRx().Device().SetTxNames([]string{atePort1.Name + ".IPv4"}).SetRxNames(rxEndpoints)
	}
	v4 := flowipv4.Packet().Add().Ipv4()
	v4.Src().SetValue(atePort1.IPv4)
	v4.Dst().Increment().SetStart(dstPfxMin).SetCount(dstPfxCount)
	otg.PushConfig(t, ateTop)
	otg.StartProtocols(t)
	return modName
}

func validateTrafficFlows(t *testing.T, ate *ondatra.ATEDevice, good, bad []string) {
	ateTop := ate.OTG().FetchConfig(t)
	if len(good) == 0 && len(bad) == 0 {
		return
	}

	var newGoodFlows, newBadFlows []string
	allFlows := append(good, bad...)
	ateTop.Flows().Clear().Items()
	for _, flow := range allFlows {
		if flow == "port2Flow" {
			if elementInSlice(flow, good) {
				newGoodFlows = append(newGoodFlows, createFlow(t, "Port 1 to Port 2", ate, ateTop, &atePort2))
			}
			if elementInSlice(flow, bad) {
				newBadFlows = append(newBadFlows, createFlow(t, "Port 1 to Port 2", ate, ateTop, &atePort2))
			}
		}
		if flow == "port3Flow" {
			if elementInSlice(flow, good) {
				newGoodFlows = append(newGoodFlows, createFlow(t, "Port 1 to Port 3", ate, ateTop, &atePort3))
			}
			if elementInSlice(flow, bad) {
				newBadFlows = append(newBadFlows, createFlow(t, "Port 1 to Port 3", ate, ateTop, &atePort3))
			}

		}
		if flow == "ecmpFlow" {
			if elementInSlice(flow, good) {
				newGoodFlows = append(newGoodFlows, createFlow(t, "ecmpFlow", ate, ateTop, &atePort2, &atePort3))
			}
			if elementInSlice(flow, bad) {
				newBadFlows = append(newBadFlows, createFlow(t, "ecmpFlow", ate, ateTop, &atePort2, &atePort3))
			}

		}
	}
	ate.OTG().StartTraffic(t)
	time.Sleep(15 * time.Second)
	ate.OTG().StopTraffic(t)

	otgutils.LogFlowMetrics(t, ate.OTG(), ateTop)
	otgutils.LogPortMetrics(t, ate.OTG(), ateTop)

	for _, flow := range newGoodFlows {
		var txPackets, rxPackets uint64
		if flow == "ecmpFlow" {
			for _, p := range ateTop.Ports().Items() {
				portMetrics := gnmi.Get(t, ate.OTG(), gnmi.OTG().Port(p.Name()).State())
				txPackets = txPackets + portMetrics.GetCounters().GetOutFrames()
				rxPackets = rxPackets + portMetrics.GetCounters().GetInFrames()
			}
			lostPackets := int64(txPackets - rxPackets)
			lossPct := lostPackets * 100 / int64(txPackets)
			if got := lossPct; got > 0 {
				t.Fatalf("LossPct for flow %s: got %v, want 0", flow, got)
			}
		} else {
			recvMetric := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(flow).State())
			txPackets = recvMetric.GetCounters().GetOutPkts()
			rxPackets = recvMetric.GetCounters().GetInPkts()
			lostPackets := txPackets - rxPackets
			lossPct := lostPackets * 100 / txPackets
			if got := lossPct; got > 0 {
				t.Fatalf("LossPct for flow %s: got %v, want 0", flow, got)
			}
		}
	}

	for _, flow := range newBadFlows {
		recvMetric := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(flow).State())
		txPackets := recvMetric.GetCounters().GetOutPkts()
		rxPackets := recvMetric.GetCounters().GetInPkts()
		lostPackets := txPackets - rxPackets
		lossPct := lostPackets * 100 / txPackets
		if got := lossPct; got < 100 {
			t.Fatalf("LossPct for flow %s: got %v, want 100", flow, got)
		}
	}
}

// awaitTimeout calls a fluent client Await, adding a timeout to the context.
func awaitTimeout(ctx context.Context, c *fluent.GRIBIClient, t testing.TB, timeout time.Duration) error {
	subctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return c.Await(subctx, t)
}

// Waits for at least one ARP entry on the tx OTG interface
func waitOTGARPEntry(t *testing.T) {
	ate := ondatra.ATE(t, "ate")
	got, ok := gnmi.WatchAll(t, ate.OTG(), gnmi.OTG().Interface(atePort1.Name+".Eth").Ipv4NeighborAny().LinkLayerAddress().State(), time.Minute, func(val *ygnmi.Value[string]) bool {
		return val.IsPresent()
	}).Await(t)
	if !ok {
		t.Fatalf("Did not receive OTG Neighbor entry, last got: %v", got)
	}
}

// setDUTInterfaceState sets the admin state on the dut interface
func setDUTInterfaceWithState(t testing.TB, dut *ondatra.DUTDevice, dutPort *attrs.Attributes, p *ondatra.Port, state bool) {
	dc := gnmi.OC()
	i := &oc.Interface{}
	i.Enabled = ygot.Bool(state)
	i.Type = ethernetCsmacd
	i.Name = ygot.String(p.Name())
	gnmi.Update(t, dut, dc.Interface(p.Name()).Config(), i)
}

func elementInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}
