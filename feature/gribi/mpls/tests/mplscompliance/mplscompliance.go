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

// Package mplscompliance defines additional compliance
// tests for gRIBI that relate to programming MPLS entries.
package mplscompliance

import (
	"context"
	"fmt"
	"testing"

	"github.com/openconfig/gribigo/chk"
	"github.com/openconfig/gribigo/client"
	"github.com/openconfig/gribigo/constants"
  "github.com/openconfig/gribigo/fluent"
  "github.com/openconfig/gribigo/compliance"
	"go.uber.org/atomic"
)

var (
	// electionID is the global election ID used between test cases.
	electionID = atomic.Uint64{}
)

func init() {
	// Ensure that the election ID starts at 1.
	electionID.Store(1)
}

// flushServer removes all entries from the server and can be called between
// test cases in order to remove the server's RIB contents.
func flushServer(c *fluent.GRIBIClient, t *testing.T) {
	ctx := context.Background()
	c.Start(ctx, t)
	defer c.Stop(t)

	if _, err := c.Flush().
		WithElectionOverride().
		WithAllNetworkInstances().
		Send(); err != nil {
		t.Fatalf("could not remove all entries from server, got: %v", err)
	}
}

// TrafficFunc defines a function that can be run following the compliance
// test. Functions are called with two arguments, a testing.T that is called
// with test results, and the packet's label stack.
type TrafficFunc func(t *testing.T, labelStack []uint32)

// modify performs a set of operations (in ops) on the supplied gRIBI client,
// reporting errors via t.
func modify(ctx context.Context, t *testing.T, c *fluent.GRIBIClient, ops []func()) []*client.OpResult {
	c.Connection().
		WithRedundancyMode(fluent.ElectedPrimaryClient).
		WithInitialElectionID(electionID.Load(), 0)

	return compliance.DoModifyOps(c, t, ops, fluent.InstalledInRIB, false)
}

// EgressLabelStack defines a test that programs a DUT via gRIBI with a label
// forwarding entry within the defaultNIName VRF (the global routing table,
// default VRF), with a label stack with numLabels in it, starting at
// baseLabel. After the DUT has been programmed if trafficFunc is non-nil it is
// run to validate the dataplane.
func EgressLabelStack(t *testing.T, c *fluent.GRIBIClient, defaultNIName string, baseLabel, numLabels int, trafficFunc TrafficFunc) {
	defer electionID.Inc()
	defer flushServer(c, t)

	labels := []uint32{}
	for n := 1; n <= numLabels; n++ {
		labels = append(labels, uint32(baseLabel+n))
	}
	// add a label that is the top of the stack that is
	// the one that is forwarded on.
	labels = append(labels, uint32(32768))

	ops := []func(){
		func() {
			c.Modify().AddEntry(t,
				fluent.NextHopEntry().
					WithNetworkInstance(defaultNIName).
					WithIndex(1).
					WithIPAddress("192.0.2.2").
					WithPushedLabelStack(labels...))

			c.Modify().AddEntry(t,
				fluent.NextHopGroupEntry().
					WithNetworkInstance(defaultNIName).
					WithID(1).
					AddNextHop(1, 1))

			c.Modify().AddEntry(t,
				fluent.LabelEntry().
					WithLabel(100).
					WithNetworkInstance(defaultNIName).
					WithNextHopGroup(1))
		},
	}

	res := modify(context.Background(), t, c, ops)

	chk.HasResult(t, res,
		fluent.OperationResult().
			WithMPLSOperation(100).
			WithProgrammingResult(fluent.InstalledInRIB).
			WithOperationType(constants.Add).
			AsResult(),
		chk.IgnoreOperationID(),
	)

	chk.HasResult(t, res,
		fluent.OperationResult().
			WithNextHopGroupOperation(1).
			WithProgrammingResult(fluent.InstalledInRIB).
			WithOperationType(constants.Add).
			AsResult(),
		chk.IgnoreOperationID(),
	)

	chk.HasResult(t, res,
		fluent.OperationResult().
			WithNextHopOperation(1).
			WithProgrammingResult(fluent.InstalledInRIB).
			WithOperationType(constants.Add).
			AsResult(),
		chk.IgnoreOperationID(),
	)

	if trafficFunc != nil {
		t.Run(fmt.Sprintf("%d labels, traffic test", numLabels), func(t *testing.T) {
			trafficFunc(t, labels)
		})
	}
}
