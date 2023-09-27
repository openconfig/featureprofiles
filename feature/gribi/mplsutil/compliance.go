// Package mplsutil defines helpers that are used to provide common functionality across gRIBI
// tests that handle MPLS programming.
package mplsutil

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/openconfig/gribigo/chk"
	"github.com/openconfig/gribigo/client"
	"github.com/openconfig/gribigo/compliance"
	"github.com/openconfig/gribigo/constants"
	"github.com/openconfig/gribigo/fluent"
)

var (
	// electionID is the global election ID used between test cases.
	electionID = func() *atomic.Uint64 {
		eid := new(atomic.Uint64)
		eid.Store(1)
		return eid
	}()
)

// flushServer removes all entries from the server and can be called between
// test cases in order to remove the server's RIB contents.
func flushServer(t *testing.T, c *fluent.GRIBIClient) {
	ctx := context.Background()
	c.Start(ctx, t)
	defer c.Stop(t)

	if _, err := c.Flush().
		WithElectionOverride().
		WithAllNetworkInstances().
		Send(); err != nil {
		t.Fatalf("Could not remove all entries from server, got: %v", err)
	}
}

// TrafficFunc defines a function that can be run following the compliance
// test. Functions are called with two arguments, a testing.T that is called
// with test results, and the packet's label stack.
type TrafficFunc func(t *testing.T, labelStack []uint32)

// modify performs a set of operations (in ops) on the supplied gRIBI client,
// reporting errors via t.
func modify(t *testing.T, c *fluent.GRIBIClient, ops []func()) []*client.OpResult {
	c.Connection().
		WithRedundancyMode(fluent.ElectedPrimaryClient).
		WithInitialElectionID(electionID.Load(), 0)

	return compliance.DoModifyOps(c, t, ops, fluent.InstalledInRIB, false)
}

// PushLabelStack defines a test that programs a DUT via gRIBI with a
// label forwarding entry within defaultNIName, with a label stack with
// numLabels in it, starting at baseLabel, if trafficFunc is non-nil it is run
// to validate the dataplane.
//
// The DUT is expected to have a next-hop of 192.0.2.2 that is resolvable.
func PushLabelStack(t *testing.T, c *fluent.GRIBIClient, defaultNIName string, baseLabel, numLabels int, trafficFunc TrafficFunc) {
	defer electionID.Add(1)
	defer flushServer(t, c)

	var labels []uint32
	for n := 1; n <= numLabels; n++ {
		labels = append(labels, uint32(baseLabel+n))
	}

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

	res := modify(t, c, ops)

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
