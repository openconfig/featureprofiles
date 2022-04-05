// Package gribi provides helper APIs to simplify writing gribi test cases.
// It uses fluent APIs and provides wrapper functions to manage sessions and change clients? role
// roles easily without a need to keep track of the server election id.
// It also packs modify operations with the corresponding verifications to
// prevent code duplications and increase the test code readability.

package gribi

import (
	"context"
	"testing"
	"time"

	"github.com/openconfig/gribigo/chk"
	"github.com/openconfig/gribigo/constants"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
)

const (
	timeout   = time.Minute
	maxUint64 = 1<<64 - 1
)

// GRIBIHandler provides access to GRIBI APIs of the DUT.
type GRIBIHandler struct {
	DUT         *ondatra.DUTDevice
	FibACK      bool
	Persistence bool
	fluentC     *fluent.GRIBIClient
}

// Fluent resturns the fluent client that can be used to directly call the gribi fluent APIs
func (g *GRIBIHandler) Fluent(t testing.TB) *fluent.GRIBIClient {
	return g.fluentC
}

// Start function start establish a client connection with the gribi server.
// By default the client is not the leader and for that function BecomeLeader
// needs to be called.

func (g *GRIBIHandler) Start(t testing.TB) error {
	t.Helper()
	gribiC := g.DUT.RawAPIs().GRIBI().Default(t)
	g.fluentC = fluent.NewClient()
	g.fluentC.Connection().WithStub(gribiC)
	if g.Persistence {
		g.fluentC.Connection().WithInitialElectionID(1, 0).
			WithRedundancyMode(fluent.ElectedPrimaryClient).WithPersistence()
	} else {
		g.fluentC.Connection().WithInitialElectionID(1, 0).
			WithRedundancyMode(fluent.ElectedPrimaryClient)
	}
	if g.FibACK {
		g.fluentC.Connection().WithFIBACK()
	}
	g.fluentC.Start(context.Background(), t)
	g.fluentC.StartSending(context.Background(), t)
	err := g.AwaitTimeout(context.Background(), t, timeout)
	return err
}

// Close function closes the gribi session with the dut by stopping the fluent client.
func (g *GRIBIHandler) Close(t testing.TB) {
	t.Helper()
	t.Logf("closing GRIBI connection for dut: %s", g.DUT.Name())
	if g.fluentC != nil {
		g.fluentC.Stop(t)
		g.fluentC = nil
	}
}

// AwaitTimeout calls a fluent client Await by adding a timeout to the context.
func (g *GRIBIHandler) AwaitTimeout(ctx context.Context, t testing.TB, timeout time.Duration) error {
	subctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return g.fluentC.Await(subctx, t)
}

// learnElectionID learns the current server election id by sending
// a dummy modify request with election id 1.
func (g *GRIBIHandler) learnElectionID(t testing.TB) (low, high uint64) {
	t.Helper()
	t.Logf("learn GRIBI Election ID from dut: %s", g.DUT.Name())
	g.fluentC.Modify().UpdateElectionID(t, 1, 0)
	err := g.AwaitTimeout(context.Background(), t, timeout)
	if err != nil {
		t.Fatalf("learnElectionID Error: %v", err)
	}
	results := g.fluentC.Results(t)
	electionID := results[len(results)-1].CurrentServerElectionID
	return electionID.Low, electionID.High
}

// UpdateElectionID updates the election id of the dut.
// The function fails if the requsted election id is less than the server election id.
func (g *GRIBIHandler) UpdateElectionID(t testing.TB, lowElecId, highElecId uint64) {
	t.Helper()
	t.Logf("setting GRIBI Election ID for dut: %s to low=%d,high=%d", g.DUT.Name(), lowElecId, highElecId)
	g.fluentC.Modify().UpdateElectionID(t, lowElecId, highElecId)
	err := g.AwaitTimeout(context.Background(), t, timeout)
	if err != nil {
		t.Fatalf("learnElectionID Error: %v", err)
	}
	chk.HasResult(t, g.fluentC.Results(t),
		fluent.OperationResult().
			WithCurrentServerElectionID(lowElecId, highElecId).
			AsResult(),
	)
}

// BecomeLeader learns the latest election id and the make the client leader by increasing the election id by one.
func (g *GRIBIHandler) BecomeLeader(t testing.TB) {
	t.Logf("trying to be a master with increasing the election id by one on dut: %s", g.DUT.Name())
	lowElecId, highElecId := g.learnElectionID(t)
	if lowElecId == maxUint64 {
		highElecId = highElecId + 1
	} else {
		lowElecId = lowElecId + 1
	}
	g.UpdateElectionID(t, lowElecId, highElecId)
}

// AddNHG adds a NextHopGroupEntry with a given index, and a map of next hop entry indices to the weights,
// in a given network instance.
func (g *GRIBIHandler) AddNHG(t testing.TB, nhgIndex uint64, nhWeights map[uint64]uint64, instance string,
	expectedResult fluent.ProgrammingResult) {
	nhg := fluent.NextHopGroupEntry().WithNetworkInstance(instance).WithID(nhgIndex)
	for nhIndex, weight := range nhWeights {
		nhg.AddNextHop(nhIndex, weight)
	}
	g.fluentC.Modify().AddEntry(t, nhg)
	if err := g.AwaitTimeout(context.Background(), t, timeout); err != nil {
		t.Fatalf("got unexpected error from server adding a NHG, got: %v, want: nil", err)
	}

	chk.HasResult(t, g.fluentC.Results(t),
		fluent.OperationResult().
			WithNextHopGroupOperation(nhgIndex).
			WithOperationType(constants.Add).
			WithProgrammingResult(expectedResult).
			AsResult(),
		chk.IgnoreOperationID(),
	)
}

// AddNH adds a NextHopEntry with a given index to an address within a given network instance.
func (g *GRIBIHandler) AddNH(t testing.TB, nhIndex uint64, address, instance string,
	expectedResult fluent.ProgrammingResult) {
	g.fluentC.Modify().AddEntry(t,
		fluent.NextHopEntry().
			WithNetworkInstance(instance).
			WithIndex(nhIndex).
			WithIPAddress(address))

	if err := g.AwaitTimeout(context.Background(), t, timeout); err != nil {
		t.Fatalf("got unexpected error from server adding NH, got: %v, want: nil", err)
	}
	chk.HasResult(t, g.fluentC.Results(t),
		fluent.OperationResult().
			WithNextHopOperation(nhIndex).
			WithOperationType(constants.Add).
			WithProgrammingResult(expectedResult).
			AsResult(),
		chk.IgnoreOperationID(),
	)
}

// // AddIPv4 adds an IPv4Entry mapping a prefix to a given next hop group index within a given network instance.
func (g *GRIBIHandler) AddIPv4(t testing.TB, prefix string, nhgIndex uint64, instance, nhgInstance string,
	expectedResult fluent.ProgrammingResult) {
	ipv4Entry := fluent.IPv4Entry().WithPrefix(prefix).
		WithNetworkInstance(instance).
		WithNextHopGroup(nhgIndex)
	if nhgInstance != "" && nhgInstance != instance {
		ipv4Entry.WithNextHopGroupNetworkInstance(nhgInstance)
	}
	g.fluentC.Modify().AddEntry(t, ipv4Entry)
	if err := g.AwaitTimeout(context.Background(), t, timeout); err != nil {
		t.Fatalf("got unexpected error from server adding NH, got: %v, want: nil", err)
	}

	chk.HasResult(t, g.fluentC.Results(t),
		fluent.OperationResult().
			WithIPv4Operation(prefix).
			WithOperationType(constants.Add).
			WithProgrammingResult(expectedResult).
			AsResult(),
		chk.IgnoreOperationID(),
	)
}
