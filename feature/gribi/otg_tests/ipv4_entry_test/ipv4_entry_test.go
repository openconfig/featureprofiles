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
	nh1IpAddr      = "192.0.2.22"
	nh2IpAddr      = "192.0.2.42"
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
	dutPort2DummyIP = attrs.Attributes{
		Desc:       "DUT Port 2",
		IPv4Sec:    "192.0.2.21",
		IPv4LenSec: 30,
	}
	dutPort3DummyIP = attrs.Attributes{
		Desc:       "DUT Port 3",
		IPv4Sec:    "192.0.2.41",
		IPv4LenSec: 30,
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

func staticARPWithMagicUniversalIP(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	p2 := dut.Port(t, "port2")
	p3 := dut.Port(t, "port3")
	s2 := &oc.NetworkInstance_Protocol_Static{
		Prefix: ygot.String(nh1IpAddr + "/32"),
		NextHop: map[string]*oc.NetworkInstance_Protocol_Static_NextHop{
			strconv.Itoa(nh1ID): {
				Index: ygot.String(strconv.Itoa(nh1ID)),
				InterfaceRef: &oc.NetworkInstance_Protocol_Static_NextHop_InterfaceRef{
					Interface: ygot.String(p2.Name()),
				},
			},
		},
	}
	s3 := &oc.NetworkInstance_Protocol_Static{
		Prefix: ygot.String(nh2IpAddr + "/32"),
		NextHop: map[string]*oc.NetworkInstance_Protocol_Static_NextHop{
			strconv.Itoa(nh2ID): {
				Index: ygot.String(strconv.Itoa(nh2ID)),
				InterfaceRef: &oc.NetworkInstance_Protocol_Static_NextHop_InterfaceRef{
					Interface: ygot.String(p3.Name()),
				},
			},
		},
	}
	sp := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
	gnmi.Replace(t, dut, sp.Static(nh1IpAddr+"/32").Config(), s2)
	gnmi.Replace(t, dut, sp.Static(nh2IpAddr+"/32").Config(), s3)
	gnmi.Update(t, dut, gnmi.OC().Interface(p2.Name()).Config(), configStaticArp(p2, nh1IpAddr, staticDstMAC))
	gnmi.Update(t, dut, gnmi.OC().Interface(p3.Name()).Config(), configStaticArp(p3, nh2IpAddr, staticDstMAC))
}

// TestIPv4Entry tests a single IPv4Entry forwarding entry.
func TestIPv4Entry(t *testing.T) {
	ctx := context.Background()
	// Configure ATE
	ate := ondatra.ATE(t, "ate")
	configureATE(t, ate)

	dut := ondatra.DUT(t, "dut")
	configureDUT(t, dut)

	gribic := dut.RawAPIs().GRIBI(t)

	cases := []struct {
		desc                                     string
		entries                                  []fluent.GRIBIEntry
		downPort                                 *ondatra.Port
		wantGoodFlows                            []string
		wantBadFlows                             []string
		wantOperationResults                     []*client.OpResult
		gribiMACOverrideWithStaticARP            bool
		gribiMACOverrideWithStaticARPStaticRoute bool
	}{
		{
			desc: "Single next-hop",
			entries: []fluent.GRIBIEntry{
				fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
					WithIndex(nh1ID).WithIPAddress(atePort2.IPv4),
				fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
					WithID(nhgID).AddNextHop(nh1ID, 1),
				fluent.IPv4Entry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
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
				fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
					WithIndex(nh1ID).WithIPAddress(atePort2.IPv4),
				fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
					WithIndex(nh2ID).WithIPAddress(atePort3.IPv4),
				fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
					WithID(nhgID).AddNextHop(nh1ID, 1).AddNextHop(nh2ID, 1),
				fluent.IPv4Entry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
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
				fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
					WithIndex(nh1ID).WithInterfaceRef(dut.Port(t, "port2").Name()).WithMacAddress(staticDstMAC),
				fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
					WithIndex(nh2ID).WithInterfaceRef(dut.Port(t, "port3").Name()).WithMacAddress(staticDstMAC),
				fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
					WithID(nhgID).AddNextHop(nh1ID, 1).AddNextHop(nh2ID, 1),
				fluent.IPv4Entry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
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
			gribiMACOverrideWithStaticARP:            deviations.GRIBIMACOverrideWithStaticARP(dut),
			gribiMACOverrideWithStaticARPStaticRoute: deviations.GRIBIMACOverrideStaticARPStaticRoute(dut),
		},
		{
			desc: "Nonexistant next-hop",
			entries: []fluent.GRIBIEntry{
				fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
					WithID(nhgID).AddNextHop(badNH, 1),
				fluent.IPv4Entry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
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
			desc: "Downed next-hop interface",
			downPort: func() *ondatra.Port {
				if deviations.ATEPortLinkStateOperationsUnsupported(ate) {
					return dut.Port(t, "port2")
				}
				return ate.Port(t, "port2")
			}(),
			entries: []fluent.GRIBIEntry{
				fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
					WithIndex(nh1ID).WithIPAddress(atePort2.IPv4).
					WithInterfaceRef(dut.Port(t, "port2").Name()).WithMacAddress(staticDstMAC),
				fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
					WithID(nhgID).AddNextHop(nh1ID, 1),
				fluent.IPv4Entry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
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
				newGoodFlows, newBadFlows := createTrafficFlows(t, ate, tc.wantGoodFlows, tc.wantBadFlows)
				t.Run(tc.desc, func(t *testing.T) {
					if tc.gribiMACOverrideWithStaticARPStaticRoute {
						staticARPWithMagicUniversalIP(t, dut)
					} else if tc.gribiMACOverrideWithStaticARP {
						//Creating a Static ARP entry for staticDstMAC
						d := gnmi.OC()
						p2 := dut.Port(t, "port2")
						p3 := dut.Port(t, "port3")
						gnmi.Update(t, dut, d.Interface(p2.Name()).Config(), dutPort2DummyIP.NewOCInterface(p2.Name(), dut))
						gnmi.Update(t, dut, d.Interface(p3.Name()).Config(), dutPort3DummyIP.NewOCInterface(p3.Name(), dut))
						gnmi.Update(t, dut, d.Interface(p2.Name()).Config(), configStaticArp(p2, nh1IpAddr, staticDstMAC))
						gnmi.Update(t, dut, d.Interface(p3.Name()).Config(), configStaticArp(p3, nh2IpAddr, staticDstMAC))
					}
					if tc.gribiMACOverrideWithStaticARP || tc.gribiMACOverrideWithStaticARPStaticRoute {
						//Programming a gRIBI flow with above IP/mac-address as the next-hop entry
						tc.entries = []fluent.GRIBIEntry{
							fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
								WithIndex(nh1ID).WithInterfaceRef(dut.Port(t, "port2").Name()).WithIPAddress(nh1IpAddr).WithMacAddress(staticDstMAC),
							fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
								WithIndex(nh2ID).WithInterfaceRef(dut.Port(t, "port3").Name()).WithIPAddress(nh2IpAddr).WithMacAddress(staticDstMAC),
							fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
								WithID(nhgID).AddNextHop(nh1ID, 1).AddNextHop(nh2ID, 1),
							fluent.IPv4Entry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
								WithPrefix(dstPfx).WithNextHopGroup(nhgID),
						}
					}
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
						if deviations.ATEPortLinkStateOperationsUnsupported(ate) {
							// Setting admin state down on the DUT interface.
							// Setting the OTG interface down has no effect in KNE environments.
							setDUTInterfaceWithState(t, dut, &dutPort2, tc.downPort, false)
							defer setDUTInterfaceWithState(t, dut, &dutPort2, tc.downPort, true)
						} else {
							portStateAction := gosnappi.NewControlState()
							linkState := portStateAction.Port().Link().SetPortNames([]string{tc.downPort.ID()}).SetState(gosnappi.StatePortLinkState.DOWN)
							ate.OTG().SetControlState(t, portStateAction)
							// Restore port state at end of test case.
							linkState.SetState(gosnappi.StatePortLinkState.UP)
							defer ate.OTG().SetControlState(t, portStateAction)
						}
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
						if tc.gribiMACOverrideWithStaticARPStaticRoute {
							sp := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
							gnmi.Delete(t, dut, sp.Static(nh1IpAddr+"/32").Config())
							gnmi.Delete(t, dut, sp.Static(nh2IpAddr+"/32").Config())
						}
					}()

					for _, wantResult := range tc.wantOperationResults {
						chk.HasResult(t, c.Results(t), wantResult, chk.IgnoreOperationID())
					}
					validateTrafficFlows(t, ate, newGoodFlows, newBadFlows)
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

	gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), dutPort1.NewOCInterface(p1.Name(), dut))
	gnmi.Replace(t, dut, d.Interface(p2.Name()).Config(), dutPort2.NewOCInterface(p2.Name(), dut))
	gnmi.Replace(t, dut, d.Interface(p3.Name()).Config(), dutPort3.NewOCInterface(p3.Name(), dut))
	if deviations.ExplicitIPv6EnableForGRIBI(dut) {
		gnmi.Update(t, dut, d.Interface(p2.Name()).Subinterface(0).Ipv6().Enabled().Config(), bool(true))
		gnmi.Update(t, dut, d.Interface(p3.Name()).Subinterface(0).Ipv6().Enabled().Config(), bool(true))
	}

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, p1)
		fptest.SetPortSpeed(t, p2)
		fptest.SetPortSpeed(t, p3)
	}
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, p1.Name(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, p2.Name(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, p3.Name(), deviations.DefaultNetworkInstance(dut), 0)
	}
}

// configreATE configures port1-3 on the ATE.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	top := gosnappi.NewConfig()

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

	var rxEndpoints []string
	for _, dst := range dsts {
		rxEndpoints = append(rxEndpoints, dst.Name+".IPv4")
	}
	flowipv4 := ateTop.Flows().Add().SetName(name)
	flowipv4.Metrics().SetEnable(true)
	e1 := flowipv4.Packet().Add().Ethernet()
	e1.Src().SetValue(atePort1.MAC)
	flowipv4.TxRx().Device().SetTxNames([]string{atePort1.Name + ".IPv4"}).SetRxNames(rxEndpoints)
	v4 := flowipv4.Packet().Add().Ipv4()
	v4.Src().SetValue(atePort1.IPv4)
	v4.Dst().Increment().SetStart(dstPfxMin).SetCount(dstPfxCount)
	return name
}

func createTrafficFlows(t *testing.T, ate *ondatra.ATEDevice, good, bad []string) (newGood, newBad []string) {
	var newGoodFlows, newBadFlows []string
	allFlows := append(good, bad...)
	otg := ate.OTG()
	ateTop := otg.FetchConfig(t)
	if len(good) == 0 && len(bad) == 0 {
		otg.PushConfig(t, ateTop)
		otg.StartProtocols(t)
		return newGoodFlows, newBadFlows
	}
	ateTop.Flows().Clear().Items()
	for _, flow := range allFlows {
		if flow == "port2Flow" {
			if elementInSlice(flow, good) {
				newGoodFlows = append(newGoodFlows, createFlow(t, "Port1_to_Port2", ate, ateTop, &atePort2))
			}
			if elementInSlice(flow, bad) {
				newBadFlows = append(newBadFlows, createFlow(t, "Port1_to_Port2", ate, ateTop, &atePort2))
			}
		}
		if flow == "port3Flow" {
			if elementInSlice(flow, good) {
				newGoodFlows = append(newGoodFlows, createFlow(t, "Port1_to_Port3", ate, ateTop, &atePort3))
			}
			if elementInSlice(flow, bad) {
				newBadFlows = append(newBadFlows, createFlow(t, "Port1_to_Port3", ate, ateTop, &atePort3))
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
	otg.PushConfig(t, ateTop)
	otg.StartProtocols(t)
	return newGoodFlows, newBadFlows
}

func validateTrafficFlows(t *testing.T, ate *ondatra.ATEDevice, good, bad []string) {
	if len(good) == 0 && len(bad) == 0 {
		return
	}

	newGoodFlows := good
	newBadFlows := bad

	ateTop := ate.OTG().FetchConfig(t)

	ate.OTG().StartTraffic(t)
	time.Sleep(15 * time.Second)
	ate.OTG().StopTraffic(t)

	otgutils.LogFlowMetrics(t, ate.OTG(), ateTop)
	otgutils.LogPortMetrics(t, ate.OTG(), ateTop)

	for _, flow := range newGoodFlows {
		var txPackets, rxPackets uint64
		recvMetric := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(flow).State())
		txPackets = recvMetric.GetCounters().GetOutPkts()
		rxPackets = recvMetric.GetCounters().GetInPkts()
		if txPackets == 0 {
			t.Fatalf("TxPkts == 0, want > 0")
		}
		lostPackets := float32(txPackets - rxPackets)
		lossPct := lostPackets * 100 / float32(txPackets)
		if got := lossPct; got > 0 {
			t.Fatalf("LossPct for flow %s: got %v, want 0", flow, got)
		}
	}

	for _, flow := range newBadFlows {
		recvMetric := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(flow).State())
		txPackets := recvMetric.GetCounters().GetOutPkts()
		rxPackets := recvMetric.GetCounters().GetInPkts()
		if txPackets == 0 {
			t.Fatalf("TxPkts == 0, want > 0")
		}
		lostPackets := float32(txPackets - rxPackets)
		lossPct := lostPackets * 100 / float32(txPackets)
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

func configStaticArp(p *ondatra.Port, ipv4addr string, macAddr string) *oc.Interface {
	i := &oc.Interface{Name: ygot.String(p.Name())}
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	s := i.GetOrCreateSubinterface(0)
	s4 := s.GetOrCreateIpv4()
	n4 := s4.GetOrCreateNeighbor(ipv4addr)
	n4.LinkLayerAddress = ygot.String(macAddr)
	return i
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
