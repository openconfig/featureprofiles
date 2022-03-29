package gribi

import (
	"context"
	"sync"
	"testing"
	"time"

	spb "github.com/openconfig/gribi/v1/proto/service"
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
	dut            *ondatra.DUTDevice
	fluentC        *fluent.GRIBIClient
	fibACK         bool
	persistence    bool
	redundancyMode fluent.RedundancyMode
	lastElectionID *spb.Uint128
}

var mu sync.Mutex

// NewGRIBIFluent establishes a gribi client session with the dut.
// persistence and fibACK specify how the session to be established.
// The client connection handles (grigo  and fluent clients) and session parameters are cached.
// The client is not the leader by default and for that function BecomeLeader needs to be called.
func NewGRIBIFluent(t testing.TB, dut *ondatra.DUTDevice, persistence, fibACK bool) *GRIBIHandler {
	fluentClient := fluent.NewClient()
	gribiC := dut.RawAPIs().GRIBI().Default(t)
	fluentClient.Connection().WithStub(gribiC)
	g := &GRIBIHandler{dut: dut,
		fluentC:     fluentClient,
		fibACK:      fibACK,
		persistence: persistence,
		// we assume all of test cases use ElectedPrimaryClient
		redundancyMode: fluent.ElectedPrimaryClient,
		lastElectionID: &spb.Uint128{Low: 1,
			High: 0,
		},
	}
	if persistence {
		fluentClient.Connection().WithInitialElectionID(g.lastElectionID.Low, g.lastElectionID.High).WithRedundancyMode(fluent.ElectedPrimaryClient).WithPersistence()
	} else {
		fluentClient.Connection().WithInitialElectionID(g.lastElectionID.Low, g.lastElectionID.High).WithRedundancyMode(fluent.ElectedPrimaryClient)
	}
	if fibACK {
		fluentClient.Connection().WithFIBACK()
	}
	fluentClient.Start(context.Background(), t)
	fluentClient.StartSending(context.Background(), t)
	err := g.AwaitTimeout(context.Background(), t, timeout)
	if err != nil {
		t.Fatalf("can not establish the session: %v", err)
	}
	return g
}

// Fluent resturns the fluent client that can be used to directly call the gribi fluent apis
func (g *GRIBIHandler) Fluent(t testing.TB) *fluent.GRIBIClient {
	return g.fluentC
}

// Close function closes the gribi session with the dut by
// stopping the fluent client and closing the grpc clinet.
func (g *GRIBIHandler) Close(t testing.TB) {
	t.Helper()
	t.Logf("closing GRIBI connection for dut: %s", g.dut.Name())
	g.fluentC.Stop(t)
}

// AwaitTimeout calls a fluent client Await by adding a timeout to the context.
func (g *GRIBIHandler) AwaitTimeout(ctx context.Context, t testing.TB, timeout time.Duration) error {
	subctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return g.fluentC.Await(subctx, t)
}

// LearnElectionID learns the current server election id of the dut by sending a dummy modify request. The function caches the election id.
func (g *GRIBIHandler) LearnElectionID(t testing.TB) (low, high uint64) {
	t.Helper()
	t.Logf("learn GRIBI Election ID from dut: %s", g.dut.Name())
	g.fluentC.Modify().UpdateElectionID(t, g.lastElectionID.Low, g.lastElectionID.High)
	err := g.AwaitTimeout(context.Background(), t, timeout)
	if err != nil {
		t.Fatalf("learnElectionID Error: %v", err)
	}
	results := g.fluentC.Results(t)
	mu.Lock()
	defer mu.Unlock()
	g.lastElectionID = results[len(results)-1].CurrentServerElectionID
	return g.lastElectionID.Low, g.lastElectionID.High
}

// UpdateElectionID updates the election id of the dut. The function fails if the requsted election id is less than the server election id
func (g *GRIBIHandler) UpdateElectionID(t testing.TB, lowElecId, highElecId uint64) {
	t.Helper()
	t.Logf("setting GRIBI Election ID for dut: %s to low=%d,high=%d", g.dut.Name(), lowElecId, highElecId)
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
	results := g.fluentC.Results(t)
	mu.Lock()
	defer mu.Unlock()
	g.lastElectionID = results[len(results)-1].CurrentServerElectionID
}

// GetLastElectionID returnes the latest election id that is cached without querying it from dut.
func (g *GRIBIHandler) GetLastElectionID(t testing.TB) (low, high uint64) {
	return g.lastElectionID.Low, g.lastElectionID.High
}

// BecomeLeader learns the latest election id and the make the client leader by increasing the election id by one.
func (g *GRIBIHandler) BecomeLeader(t testing.TB) {
	t.Logf("trying to be a master with increasing the election id by one on dut: %s", g.dut.Name())
	lowElecId, highElecId := g.LearnElectionID(t)
	if lowElecId == maxUint64 {
		highElecId = highElecId + 1
	} else {
		lowElecId = lowElecId + 1
	}
	g.UpdateElectionID(t, lowElecId, highElecId)
}

// AddNHG adds a NextHOP group entry
func (g *GRIBIHandler) AddNHG(t testing.TB, nhgIndex uint64, nhWeights map[uint64]uint64, instance string, expectedResult fluent.ProgrammingResult) {
	NGH := fluent.NextHopGroupEntry().WithNetworkInstance(instance).WithID(nhgIndex)
	for nhIndex, weight := range nhWeights {
		NGH.AddNextHop(nhIndex, weight)
	}
	g.fluentC.Modify().AddEntry(t, NGH)
	if err := g.AwaitTimeout(context.Background(), t, timeout); err != nil {
		t.Fatalf("got unexpected error from server adding NGH, got: %v, want: nil", err)
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

// AddNH adds a NextHOP entry
func (g *GRIBIHandler) AddNH(t testing.TB, nhIndex uint64, address, instance string, expectedResult fluent.ProgrammingResult) {
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

// AddNH adds an IPV4 entry
func (g *GRIBIHandler) AddIPV4Entry(t testing.TB, nhgIndex uint64, nhgInstance, prefix, instance string, expectedResult fluent.ProgrammingResult) {
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
