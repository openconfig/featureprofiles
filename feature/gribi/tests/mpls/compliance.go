// Package gribi_mpls_compliance_test defines additional compliance
// tests for gRIBI that relate to programming MPLS entries.
package gribi_mpls_compliance_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/openconfig/gribigo/chk"
	"github.com/openconfig/gribigo/client"
	"github.com/openconfig/gribigo/compliance"
	"github.com/openconfig/gribigo/constants"
	"github.com/openconfig/gribigo/fluent"
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

// EgressLabelStack defines a test that programs a DUT via gRIBI with a
// label forwarding entry within defaultNIName, with a label stack with
// numLabels in it, starting at baseLabel. After the DUT has been programmed
// if trafficFunc is non-nil it is run to validate the dataplane.
func EgressLabelStack(t *testing.T, c *fluent.GRIBIClient, defaultNIName string, baseLabel, numLabels int, trafficFunc TrafficFunc) {
	defer electionID.Inc()
	defer flushServer(c, t)

	labels := []uint32{}
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

// PushToIPPacket programs a gRIBI entry for an ingress LER function whereby MPLS labels are
// pushed to an IP packet. The entries are programmed into defaultNIName, with the stack
// imposed being a stack of numLabels labels, starting with baseLabel. After the programming
// has been verified trafficFunc is run to allow validation of the dataplane.
func PushToIPPacket(t *testing.T, c *fluent.GRIBIClient, defaultNIName string, baseLabel, numLabels int, trafficFunc TrafficFunc) {
	defer electionID.Inc()
	defer flushServer(c, t)

	c.Connection().
		WithRedundancyMode(fluent.ElectedPrimaryClient).
		WithInitialElectionID(electionID.Load(), 0)

	labels := []uint32{}
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
				fluent.IPv4Entry().
					WithPrefix("10.0.0.0/24").
					WithNetworkInstance(defaultNIName).
					WithNextHopGroupNetworkInstance(defaultNIName).
					WithNextHopGroup(1))
		},
	}

	res := modify(context.Background(), t, c, ops)

	chk.HasResult(t, res,
		fluent.OperationResult().
			WithIPv4Operation("10.0.0.0/24").
			WithProgrammingResult(fluent.InstalledInRIB).
			WithOperationType(constants.Add).
			AsResult(),
		chk.IgnoreOperationID())

	chk.HasResult(t, res,
		fluent.OperationResult().
			WithNextHopOperation(1).
			WithProgrammingResult(fluent.InstalledInRIB).
			WithOperationType(constants.Add).
			AsResult(),
		chk.IgnoreOperationID())

	chk.HasResult(t, res,
		fluent.OperationResult().
			WithNextHopGroupOperation(1).
			WithProgrammingResult(fluent.InstalledInRIB).
			WithOperationType(constants.Add).
			AsResult(),
		chk.IgnoreOperationID())

	if trafficFunc != nil {
		t.Run(fmt.Sprintf("%d label push, traffic test", numLabels), func(t *testing.T) {
			trafficFunc(t, labels)
		})
	}
}

func PopTopLabel(t *testing.T, c *fluent.GRIBIClient, defaultNIName string, trafficFunc TrafficFunc) {
	defer electionID.Inc()
	defer flushServer(c, t)

	ops := []func(){
		func() {
			c.Modify().AddEntry(t,
				fluent.NextHopEntry().
					WithNetworkInstance(defaultNIName).
					WithIndex(1).
					WithIPAddress("192.0.2.2").
					WithPopTopLabel())

			c.Modify().AddEntry(t,
				fluent.NextHopGroupEntry().
					WithNetworkInstance(defaultNIName).
					WithID(1).
					AddNextHop(1, 1))

			// Specify MPLS label that is pointed to our pop next-hop.
			c.Modify().AddEntry(t,
				fluent.LabelEntry().
					WithLabel(100).
					WithNetworkInstance(defaultNIName).
					WithNextHopGroupNetworkInstance(defaultNIName).
					WithNextHopGroup(1))

			// Specify IP prefix that is pointed to our pop next-hop.
			c.Modify().AddEntry(t,
				fluent.IPv4Entry().
					WithPrefix("10.0.0.0/24").
					WithNetworkInstance(defaultNIName).
					WithNextHopGroup(1))
		},
	}

	res := modify(context.Background(), t, c, ops)

	chk.HasResult(t, res,
		fluent.OperationResult().
			WithIPv4Operation("10.0.0.0/24").
			WithProgrammingResult(fluent.InstalledInRIB).
			WithOperationType(constants.Add).
			AsResult(),
		chk.IgnoreOperationID())

	chk.HasResult(t, res,
		fluent.OperationResult().
			WithNextHopGroupOperation(1).
			WithProgrammingResult(fluent.InstalledInRIB).
			WithOperationType(constants.Add).
			AsResult(),
		chk.IgnoreOperationID())

	chk.HasResult(t, res,
		fluent.OperationResult().
			WithNextHopOperation(1).
			WithProgrammingResult(fluent.InstalledInRIB).
			WithOperationType(constants.Add).
			AsResult(),
		chk.IgnoreOperationID())

	if trafficFunc != nil {
		t.Run("pop-top-label, traffic test", func(t *testing.T) {
			trafficFunc(t, nil)
		})
	}
}

func PopNLabels(t *testing.T, c *fluent.GRIBIClient, defaultNIName string, popLabels []uint32, trafficFunc) {
	defer electionID.Inc()
	defer flushServer(c, t)

	c.Connection().
		WithRedundancyMode(fluent.ElectedPrimaryClient).
		WithInitialElectionID(electionID.Load(), 0)

	ctx := context.Background()
	c.Start(ctx, t)
	defer c.Stop(t)

	c.StartSending(ctx, t)

	c.Modify().AddEntry(t,
		fluent.NextHopEntry().
			WithNetworkInstance(defaultNIName).
			WithIndex(1).
			WithIPAddress("192.0.2.2").
			WithPoppedLabelStack(popLabels...))

	c.Modify().AddEntry(t,
		fluent.NextHopGroupEntry().
			WithNetworkInstance(defaultNIName).
			WithID(1).
			AddNextHop(1,1))

	c.Modify().AddEntry(t,
		fluent.LabelEntry().
			WithLabel(100).
			WithNetworkInstance(defaultNIName).
			WithNextHopGroup(1))


}
