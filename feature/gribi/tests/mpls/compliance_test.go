package gribi_mpls_compliance_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/gribigo/chk"
	"github.com/openconfig/gribigo/constants"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"go.uber.org/atomic"
)

const (
	defNIName = "default"
)

var (
	electionID = atomic.Uint64{}
)

func TestMain(m *testing.M) {
	// Ensure that the election ID starts at 1.
	electionID.Store(1)
	fptest.RunTests(m)
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

// Test cases to write:
//	* push(N) labels, N = 1-20.
//	* pop(1) - terminating action
//	* pop(1) + push(N)
//	* pop(all) + push(N)

// TestMPLSLabelPushDepth validates the gRIBI actions that are used to push N labels onto
// as part of routing towards a next-hop. Note that this test does not validate against the
// dataplane, but solely the gRIBI control-plane support.
func TestMPLSLabelPushDepth(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	gribic := dut.RawAPIs().GRIBI().Default(t)
	c := fluent.NewClient()
	c.Connection().WithStub(gribic)

	baseLabel := 42

	for i := 1; i <= 20; i++ {
		t.Run(fmt.Sprintf("push %d labels", i), func(t *testing.T) {
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
			for n := 1; n <= i; n++ {
				labels = append(labels, uint32(baseLabel+n))
			}

			c.Modify().AddEntry(t,
				fluent.NextHopEntry().
					WithNetworkInstance(defNIName).
					WithIndex(1).
					WithIPAddress("1.1.1.1").
					WithPushedLabelStack(labels...))

			c.Modify().AddEntry(t,
				fluent.NextHopGroupEntry().
					WithNetworkInstance(defNIName).
					WithID(1).
					AddNextHop(1, 1))

			c.Modify().AddEntry(t,
				fluent.LabelEntry().
					WithLabel(100).
					WithNetworkInstance(defNIName).
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
		})
	}
}
