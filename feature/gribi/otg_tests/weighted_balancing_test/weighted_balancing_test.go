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
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/open-traffic-generator/snappi/gosnappi"

	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/gribigo/chk"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"

	spb "github.com/openconfig/gribi/v1/proto/service"
)

var (
	// The next hop group has ports ate:port{2-9}, since ate:port1
	// is the traffic source.
	cases = []struct {
		TestName    string
		Description string
		NextHops    []nextHop
	}{
		{
			TestName:    "OneNextHop",
			Description: "With NHG 10 containing 1 next hop, 100% of traffic is forwarded to the installed next-hop.",
			NextHops:    []nextHop{{"ate:port2", 0, 1.0}},
		},
		{
			TestName:    "TwoNextHops",
			Description: "With NHG 10 containing 2 next hops with no associated weights assigned, 50% of traffic is forwarded to each next-hop.",
			NextHops:    []nextHop{{"ate:port2", 0, 0.5}, {"ate:port3", 0, 0.5}},
		},
		{
			TestName:    "EightNextHops",
			Description: "With NHG 10 containing 8 next hops, with no associated weights assigned, 12.5% of traffic is forwarded to each next-hop.",
			NextHops: []nextHop{
				{"ate:port2", 0, 0.125}, {"ate:port3", 0, 0.125},
				{"ate:port4", 0, 0.125}, {"ate:port5", 0, 0.125},
				{"ate:port6", 0, 0.125}, {"ate:port7", 0, 0.125},
				{"ate:port8", 0, 0.125}, {"ate:port9", 0, 0.125},
			},
		},

		// With NHG 10 containing 2 next-hops, specify and validate the
		// following ratios:
		{
			TestName:    "Weight_1_1",
			Description: "Weight 1:1 - 50% per-NH.",
			NextHops:    []nextHop{{"ate:port2", 1, 0.5}, {"ate:port3", 1, 0.5}},
		},
		{
			TestName:    "Weight_2_1",
			Description: "Weight 2:1 - 66% traffic to NH1, 33% to NH2.",
			NextHops:    []nextHop{{"ate:port2", 2, 0.6667}, {"ate:port3", 1, 0.3333}},
		},
		{
			TestName:    "Weight_9_1",
			Description: "Weight 9:1 - 90% traffic to NH1, 10% to NH2.",
			NextHops:    []nextHop{{"ate:port2", 9, 0.9}, {"ate:port3", 1, 0.1}},
		},
		{
			TestName:    "Weight_31_1",
			Description: "Weight 31:1 - ~96.9% traffic to NH1, ~3.1% to NH2.",
			NextHops:    []nextHop{{"ate:port2", 31, 0.96875}, {"ate:port3", 1, 0.03125}},
		},
		{
			TestName:    "Weight_63_1",
			Description: "Weight 63:1 - ~98.4% traffic to NH1, ~1.6% to NH2.",
			NextHops:    []nextHop{{"ate:port2", 63, 0.984375}, {"ate:port3", 1, 0.015625}},
		},
	}

	scales = []struct {
		TestName string
		Scale    uint64
	}{
		{
			TestName: "Weights < 64K",
			Scale:    1,
		},
		{
			TestName: "Weights > 64K",
			Scale:    65536,
		},
	}
)

var approxOpt = cmpopts.EquateApprox(0 /* frac */, 0.01 /* absolute */)

// testNextHop performs traffic test according to the next hop configuration.
func testNextHop(
	ctx context.Context,
	t *testing.T,
	nexthops []nextHop,
	scale uint64, // multiplies the weights in nexthops by this.
	gribic spb.GRIBIClient,
	ate *ondatra.ATEDevice,
	top gosnappi.Config,
) {
	// Configure the gRIBI client.
	c := fluent.NewClient()
	c.Connection().
		WithStub(gribic).
		WithRedundancyMode(fluent.ElectedPrimaryClient).
		WithInitialElectionID(1 /* low */, 0 /* hi */). // ID must be > 0.
		WithPersistence()
	c.Start(ctx, t)
	defer c.Stop(t)

	defer func() {
		// Flush all entries after test.
		if err := gribi.FlushAll(c); err != nil {
			t.Errorf("Cannot flush: %v", err)
		}
	}()

	c.StartSending(ctx, t)
	if err := awaitTimeout(ctx, c, t, time.Minute); err != nil {
		t.Fatalf("Await got error during session negotiation: %v", err)
	}
	gribi.BecomeLeader(t, c)

	// Flush all entries before test.
	if err := gribi.FlushAll(c); err != nil {
		t.Errorf("Cannot flush: %v", err)
	}

	ents, wants := buildNextHops(t, nexthops, scale)

	c.Modify().AddEntry(t, ents...)
	if err := awaitTimeout(ctx, c, t, time.Minute); err != nil {
		t.Fatalf("Await got error for entries: %v", err)
	}

	res := c.Results(t)

	for _, op := range res {
		if op.ProgrammingResult == spb.AFTResult_FAILED && scale >= 65536 {
			t.Skip("Okay for device to refuse AFT when weight >= 64K")
		}
	}

	for _, want := range wants {
		t.Logf("Checking for result: %+v", want)
		chk.HasResult(t, res, want)
	}

	// Generate and analyze traffic.
	atePorts, inPkts, outPkts := generateTraffic(t, ate, top)
	t.Logf("atePorts = %v", atePorts)
	t.Logf("inPkts = %v", inPkts)
	t.Logf("outPkts = %v", outPkts)

	got, inSum := normalize(inPkts)
	want := portWants(nexthops, atePorts)
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

func TestWeightedBalancing(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	dutPorts := dut.Ports()

	// Dial gRIBI
	ctx := context.Background()
	gribic := dut.RawAPIs().GRIBI().Default(t)

	// Configure the DUT
	configureDUT(t, dut)

	// Configure the ATE
	ate := ondatra.ATE(t, "ate")
	top := configureATE(t, ate)
	ate.OTG().StartProtocols(t)

	// Run through the test cases.
	for _, s := range scales {
		t.Run(s.TestName, func(t *testing.T) {
			for _, c := range cases {
				t.Run(c.TestName, func(t *testing.T) {
					t.Logf("Description: %s", c.Description)
					t.Logf("NextHops: %+v", c.NextHops)
					if got, want := len(dutPorts), len(c.NextHops)+1; got < want {
						t.Skipf("Testbed provides only %d ports, but test case needs %d.", got, want)
					}
					testNextHop(ctx, t, c.NextHops, s.Scale, gribic, ate, top)
					debugGRIBI(t, dut)
				})
			}
		})
	}
}
