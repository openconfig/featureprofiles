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

package weighted_balancing_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/gribigo/chk"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

// nextHopsEvenly generates []nexthop that distributes weights evenly
// across the atePorts.
func nextHopsEvenly(atePorts []*ondatra.Port) []nextHop {
	var nexthops []nextHop
	for _, ap := range atePorts {
		ateid := "ate:" + ap.ID()
		if ateid == ateSrcPort {
			continue
		}
		nexthops = append(nexthops, nextHop{ateid, 1, 0})
	}
	return nexthops
}

// portWantsEvenly generates wanted weights assuming that the traffic
// should be evenly distributed across the ports that are still up.
func portWantsEvenly(atePorts []*ondatra.Port, numUps int) []float64 {
	weights := make([]float64, len(atePorts))
	x := 1.0 / float64(numUps)
	for i := 1; i <= numUps; i++ {
		weights[i] = x
	}
	return weights
}

func testNextHopRemaining(
	t *testing.T,
	numUps int,
	dut *ondatra.DUTDevice,
	ate *ondatra.ATEDevice,
	top gosnappi.Config,
) {
	// Generate and analyze traffic.
	atePorts, inPkts, outPkts := generateTraffic(t, ate, top)
	t.Logf("atePorts = %v", atePorts)
	t.Logf("inPkts = %v", inPkts)
	t.Logf("outPkts = %v", outPkts)

	got, inSum := normalize(inPkts)
	want := portWantsEvenly(atePorts, numUps)
	t.Logf("inPkts normalized got: %v", got)
	t.Logf("weights want: %v", want)

	// Report diagnosis.
	t.Run("Ratio", func(t *testing.T) {
		if diff := cmp.Diff(want, got, approxOpt); diff != "" {
			t.Errorf("Packet distribution ratios -want,+got:\n%s", diff)
		}
	})
	t.Run("Loss", func(t *testing.T) {
		if outPkts[0] > inSum {
			t.Errorf("Traffic flow sent %d packets, received only %d",
				outPkts[0], inSum)
		}
	})
}

const portFlapDesc = "With NHG 10 containing 8 next-hops, with a weight of 1 assigned to each, sequentially remove each next-hop by turning down the port at the ATE (invalidates nexthop), ensure that traffic is rebalanced across remaining NHs until only one NH remains"

func TestPortFlap(t *testing.T) {
	// Dial gRIBI
	ctx := context.Background()
	dut := ondatra.DUT(t, "dut")
	gribic := dut.RawAPIs().GRIBI(t)

	// Configure the ATE
	ate := ondatra.ATE(t, "ate")
	top := configureATE(t, ate)

	// Configure the DUT
	configureDUT(t, dut)

	ate.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")

	// Create nexthops across the dst atePorts.
	atePorts := sortPorts(ate.Ports())
	nexthops := nextHopsEvenly(atePorts)
	t.Logf("Description: %s", portFlapDesc)
	t.Logf("NextHops: %+v", nexthops)

	// Configure the gRIBI client.
	c := fluent.NewClient()
	c.Connection().
		WithStub(gribic).
		WithRedundancyMode(fluent.ElectedPrimaryClient).
		WithInitialElectionID(1 /* low */, 0 /* hi */). // ID must be > 0.
		WithPersistence()
	c.Start(ctx, t)
	defer c.Stop(t)
	c.StartSending(ctx, t)
	if err := awaitTimeout(ctx, c, t, time.Minute); err != nil {
		t.Fatalf("Await got error during session negotiation: %v", err)
	}
	gribi.BecomeLeader(t, c)

	// Flush all entries before test.
	if err := gribi.FlushAll(c); err != nil {
		t.Errorf("Cannot flush: %v", err)
	}

	ents, wants := buildNextHops(t, dut, nexthops, 1)

	c.Modify().AddEntry(t, ents...)
	if err := awaitTimeout(ctx, c, t, time.Minute); err != nil {
		t.Fatalf("Await got error for entries: %v", err)
	}

	res := c.Results(t)

	for _, want := range wants {
		t.Logf("Checking for result: %+v", want)
		chk.HasResult(t, res, want)
	}

	// Turn down ports one by one.
	dt := gnmi.OC()

	for i := len(atePorts); i >= 2; i-- {
		numUps := i - 1
		numDowns := len(atePorts) - i
		testName := fmt.Sprintf("%d Up, %d Down", numUps, numDowns)
		if i < len(atePorts) {
			dp := dut.Port(t, atePorts[i].ID())
			if deviations.ATEPortLinkStateOperationsUnsupported(ate) {
				defer setDUTInterfaceState(t, dut, dp, true)
			} else {
				t.Logf("Bringing down ate port: %v", atePorts[i])
				portStateAction := gosnappi.NewControlState()
				portStateAction.Port().Link().SetPortNames([]string{atePorts[i].ID()}).SetState(gosnappi.StatePortLinkState.UP)
				defer ate.OTG().SetControlState(t, portStateAction)
			}
		}

		t.Run(testName, func(t *testing.T) {
			if i < len(atePorts) {
				dp := dut.Port(t, atePorts[i].ID())
				if deviations.ATEPortLinkStateOperationsUnsupported(ate) {
					// Setting admin state down on the DUT interface.
					// Setting the otg interface down has no effect on kne
					t.Logf("Bringing down dut port: %v", dp.Name())
					setDUTInterfaceState(t, dut, dp, false)
				} else {
					t.Logf("Bringing down ate port: %v", atePorts[i])
					portStateAction := gosnappi.NewControlState()
					portStateAction.Port().Link().SetPortNames([]string{atePorts[i].ID()}).SetState(gosnappi.StatePortLinkState.DOWN)
					ate.OTG().SetControlState(t, portStateAction)
				}
				// ATE and DUT ports in the linked pair have the same ID(), but
				// they are mapped to different Name().
				t.Logf("Awaiting DUT port down: %v", dp)
				dip := dt.Interface(dp.Name())
				gnmi.Await(t, dut, dip.OperStatus().State(), time.Minute, oc.Interface_OperStatus_DOWN)
				t.Log("Port is down.")
			}
			testNextHopRemaining(t, numUps, dut, ate, top)
			debugGRIBI(t, dut)
		})
	}
}

// setDUTInterfaceState sets the admin state on the dut interface
func setDUTInterfaceState(t testing.TB, dut *ondatra.DUTDevice, p *ondatra.Port, state bool) {
	t.Helper()
	dc := gnmi.OC()
	i := &oc.Interface{}
	i.Enabled = ygot.Bool(state)
	i.Type = ethernetCsmacd
	i.Name = ygot.String(p.Name())
	gnmi.Update(t, dut, dc.Interface(p.Name()).Config(), i)
}
