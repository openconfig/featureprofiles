// Copyright 2026 Google LLC
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

// Package ipv6_entry_test implements gRIBI compliance tests for IPv6 entries.
package ipv6_entry_test

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/core"
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
)

// Next-hop and Next-hop group IDs used in the test.
const (
	nhgID = 42
	nh1ID = 43
	nh2ID = 44
)

// Destination prefix and count for traffic validation.
const (
	dstPfx      = "2001:db8:a::/64"
	dstPfxMin   = "2001:db8:a::"
	dstPfxCount = 256
)

// Port attributes for DUT and ATE. Specify only IPv6 addresses since the test focuses on v6 only.
var (
	dutPort1 = attrs.Attributes{
		Desc:    "DUT Port 1",
		IPv6:    "2001:db8::1",
		IPv6Len: 126,
	}
	dutPort2 = attrs.Attributes{
		Desc:    "DUT Port 2",
		IPv6:    "2001:db8::5",
		IPv6Len: 126,
	}
	dutPort3 = attrs.Attributes{
		Desc:    "DUT Port 3",
		IPv6:    "2001:db8::9",
		IPv6Len: 126,
	}

	atePort1 = attrs.Attributes{
		Name:    "port1",
		MAC:     "02:00:01:01:01:01",
		Desc:    "ATE Port 1",
		IPv6:    "2001:db8::2",
		IPv6Len: 126,
	}
	atePort2 = attrs.Attributes{
		Name:    "port2",
		MAC:     "02:00:02:01:01:01",
		Desc:    "ATE Port 2",
		IPv6:    "2001:db8::6",
		IPv6Len: 126,
	}
	atePort3 = attrs.Attributes{
		Name:    "port3",
		MAC:     "02:00:03:01:01:01",
		Desc:    "ATE Port 3",
		IPv6:    "2001:db8::10",
		IPv6Len: 126,
	}
)

// TestMain registers the core dump validator and runs the tests.
func TestMain(m *testing.M) {
	core.Register()
	fptest.RunTests(m)
}

// persistenceMode defines the local enum for gRIBI persistence settings.
type persistenceMode int

const (
	modeDefault persistenceMode = iota
	modePreserve
)

// String returns a human readable value for persistenceMode.
func (p persistenceMode) String() string {
	switch p {
	case modeDefault:
		return "DEFAULT"
	case modePreserve:
		return "PRESERVE"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", p)
	}
}

// TestIPv6Entry validates basic IPv6 programming and forwarding.
func TestIPv6Entry(t *testing.T) {
	ctx := context.Background()
	ate := ondatra.ATE(t, "ate")
	configureATE(t, ate)

	dut := ondatra.DUT(t, "dut")
	configureDUT(t, dut)

	gribic := dut.RawAPIs().GRIBI(t)

	cases := []struct {
		desc                 string
		entries              []fluent.GRIBIEntry
		wantGoodFlows        []string
		wantBadFlows         []string
		wantOperationResults []*client.OpResult
	}{
		{
			desc: "Single next-hop",
			entries: []fluent.GRIBIEntry{
				fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
					WithIndex(nh1ID).WithIPAddress(atePort2.IPv6),
				fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
					WithID(nhgID).AddNextHop(nh1ID, 1),
				fluent.IPv6Entry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
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
					WithIPv6Operation(dstPfx).
					WithProgrammingResult(fluent.InstalledInFIB).
					WithOperationType(constants.Add).
					AsResult(),
			},
		},
	}

	for _, persist := range []persistenceMode{modePreserve} {
		t.Run(fmt.Sprintf("Persistence=%s", persist), func(t *testing.T) {
			for _, tc := range cases {
				newGoodFlows, newBadFlows := createTrafficFlows(t, ate, tc.wantGoodFlows, tc.wantBadFlows)
				t.Run(tc.desc, func(t *testing.T) {
					c := fluent.NewClient()
					conn := c.Connection().
						WithStub(gribic).
						WithRedundancyMode(fluent.ElectedPrimaryClient).
						WithInitialElectionID(1, 0)
					if persist == modePreserve {
						conn.WithPersistence()
					}
					if !deviations.GRIBIRIBAckOnly(dut) {
						conn.WithFIBACK()
					}

					c.Start(ctx, t)
					defer c.Stop(t)
					c.StartSending(ctx, t)
					if err := awaitTimeout(ctx, c, t, time.Minute); err != nil {
						t.Fatalf("Await got error during session negotiation: %v", err)
					}
					gribi.BecomeLeader(t, c)

					if persist == modePreserve {
						defer func() {
							if err := gribi.FlushAll(c); err != nil {
								t.Errorf("Cannot flush: %v", err)
							}
						}()
					}

					c.Modify().AddEntry(t, tc.entries...)
					if err := awaitTimeout(ctx, c, t, time.Minute); err != nil {
						t.Fatalf("Await got error for entries: %v", err)
					}
					defer func() {
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
					validateTrafficFlows(t, ate, newGoodFlows, newBadFlows)
				})
			}
		})
	}
}

// configureDUT configures the interfaces and network instances on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	d := gnmi.OC()

	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")
	p3 := dut.Port(t, "port3")

	gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), dutPort1.NewOCInterface(p1.Name(), dut))
	gnmi.Replace(t, dut, d.Interface(p2.Name()).Config(), dutPort2.NewOCInterface(p2.Name(), dut))
	gnmi.Replace(t, dut, d.Interface(p3.Name()).Config(), dutPort3.NewOCInterface(p3.Name(), dut))
	if deviations.ExplicitIPv6EnableForGRIBI(dut) {
		gnmi.Update(t, dut, d.Interface(p1.Name()).Subinterface(0).Ipv6().Enabled().Config(), bool(true))
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

// configureATE configures the interfaces and protocols on the ATE.
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

	otgutils.WaitForARP(t, ate.OTG(), top, "IPv6") // Wait for IPv6 NDP

	return top
}

// createFlow creates a traffic flow from ATE Port 1 to the destination prefix.
func createFlow(t *testing.T, name string, ate *ondatra.ATEDevice, ateTop gosnappi.Config, dsts ...*attrs.Attributes) string {
	var rxEndpoints []string
	for _, dst := range dsts {
		rxEndpoints = append(rxEndpoints, dst.Name+".IPv6")
	}
	flowipv6.TxRx().Device().SetTxNames([]string{atePort1.Name + ".IPv6"}).SetRxNames(rxEndpoints)
	v6 := flowipv6.Packet().Add().Ipv6()
	flowipv6.TxRx().Device().SetTxNames([]string{atePort1.Name + ".IPv6"}).SetRxNames(rxEndpoints)
	v6 := flowipv6.Packet().Add().Ipv6()
	v6.Src().SetValue(atePort1.IPv6)
	v6.Dst().Increment().SetStart(dstPfxMin).SetCount(dstPfxCount)
	return flowipv6.Name()
}

// createTrafficFlows defines the traffic flows to be generated based on the test case.
func createTrafficFlows(t *testing.T, ate *ondatra.ATEDevice, good, bad []string) (newGood, newBad []string) {
	var newGoodFlows, newBadFlows []string
	allFlows := append(good, bad...)
	otg := ate.OTG()
	ateTop := otg.GetConfig(t)
	if len(good) == 0 && len(bad) == 0 {
		otg.PushConfig(t, ateTop)
		otg.StartProtocols(t)
		return newGoodFlows, newBadFlows
	}
	ateTop.Flows().Clear().Items()
	for _, flow := range allFlows {
		if flow == "port2Flow" {
			if slices.Contains(good, flow) {
				newGoodFlows = append(newGoodFlows, createFlow(t, "Port1_to_Port2", ate, ateTop, &atePort2))
			}
			if slices.Contains(bad, flow) {
				newBadFlows = append(newBadFlows, createFlow(t, "Port1_to_Port2", ate, ateTop, &atePort2))
			}
		}
		if flow == "port3Flow" {
			if slices.Contains(good, flow) {
				newGoodFlows = append(newGoodFlows, createFlow(t, "Port1_to_Port3", ate, ateTop, &atePort3))
			}
			if slices.Contains(bad, flow) {
				newBadFlows = append(newBadFlows, createFlow(t, "Port1_to_Port3", ate, ateTop, &atePort3))
			}
		}
	}
	otg.PushConfig(t, ateTop)
	return newGoodFlows, newBadFlows
}

// validateTrafficFlows verifies that "good" flows have 0% loss and "bad" flows have 100% loss.
func validateTrafficFlows(t *testing.T, ate *ondatra.ATEDevice, good, bad []string) {
	if len(good) == 0 && len(bad) == 0 {
		return
	}

	newGoodFlows := good
	newBadFlows := bad

	ateTop := ate.OTG().GetConfig(t)

	ate.OTG().StartTraffic(t)
	time.Sleep(15 * time.Second)
	ate.OTG().StopTraffic(t)

	otgutils.LogFlowMetrics(t, ate.OTG(), ateTop)
	otgutils.LogPortMetrics(t, ate.OTG(), ateTop)

	for _, flow := range newGoodFlows {
		txPackets, rxPackets := otgutils.GetFlowStats(t, ate.OTG(), flow, 10*time.Second)
		if txPackets == 0 {
			t.Fatalf("TxPkts == 0 for flow %s, want > 0", flow)
		}
		if txPackets != rxPackets {
			t.Fatalf("LossPct for flow %s: got %d/%d rx/tx packets, want 100%% transmission", flow, rxPackets, txPackets)
		}
	}

	for _, flow := range newBadFlows {
		txPackets, rxPackets := otgutils.GetFlowStats(t, ate.OTG(), flow, 10*time.Second)
		if txPackets == 0 {
			t.Fatalf("TxPkts == 0 for flow %s, want > 0", flow)
		}
		if rxPackets > 0 {
			t.Fatalf("LossPct for flow %s: got %d rx packets, want 100%% loss (0 rx packets)", flow, rxPackets)
		}
	}
}

// awaitTimeout waits for the gRIBI client to receive responses for all pending operations.
func awaitTimeout(ctx context.Context, c *fluent.GRIBIClient, t testing.TB, timeout time.Duration) error {
	subctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return c.Await(subctx, t)
}
