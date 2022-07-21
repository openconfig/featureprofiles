package gribi_mpls_compliance_test

import (
	"context"
	"testing"
	"time"

	"github.com/openconfig/gribigo/chk"
	"github.com/openconfig/gribigo/constants"
	"github.com/openconfig/gribigo/fluent"
	"go.uber.org/atomic"
)

var (
	electionID = atomic.Uint64{}
)

func init() {
	// Ensure that the election ID starts at 1.
	electionID.Store(1)
}

func flushServer(c *fluent.GRIBIClient, t testing.TB) {
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

type TrafficFunc func(t testing.TB, outerLabel int)

func EgressLabelStack(t testing.TB, c *fluent.GRIBIClient, baseLabel int, defaultNIName string, numLabels int, trafficFunc TrafficFunc) {
	defer electionID.Inc()
	defer flushServer(c, t)

	c.Connection().
		WithRedundancyMode(fluent.ElectedPrimaryClient).
		WithInitialElectionID(electionID.Load(), 0)

	ctx := context.Background()
	c.Start(ctx, t)
	defer c.Stop(t)

	c.StartSending(ctx, t)

	labels := []uint32{}
	for n := 1; n <= numLabels; n++ {
		labels = append(labels, uint32(baseLabel+n))
	}

	c.Modify().AddEntry(t,
		fluent.NextHopEntry().
			WithNetworkInstance(defaultNIName).
			WithIndex(1).
			WithIPAddress("1.1.1.1").
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

	subctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	c.Await(subctx, t)

	res := c.Results(t)

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
}
