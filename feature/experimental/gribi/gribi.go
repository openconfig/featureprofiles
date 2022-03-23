package gribi

import (
	"context"
	"reflect"
	"sync"
	"testing"
	"time"
	"unsafe"

	grbp "github.com/openconfig/gribi/v1/proto/service"
	"github.com/openconfig/gribigo/chk"
	"github.com/openconfig/gribigo/constants"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"google.golang.org/grpc"
)

const (
	timeout   = time.Minute
	maxUint64 = 1<<64 - 1
)

// GRIBI provides access to GRIBI APIs of the DUT.
type GRIBIHandler struct {
	dut            *ondatra.DUTDevice
	gribiC         grbp.GRIBIClient
	fluentC        *fluent.GRIBIClient
	fibAck         bool
	persistence    bool
	redundancyMode fluent.RedundancyMode
	lastElectionID *grbp.Uint128
}

var mu sync.Mutex

func NewGRIBIFluent(t testing.TB, dut *ondatra.DUTDevice, persistance, fibAck bool) *GRIBIHandler {
	fluentClient := fluent.NewClient()
	gribiC := dut.RawAPIs().GRIBI().New(t)
	fluentClient.Connection().WithStub(gribiC)
	g := &GRIBIHandler{dut: dut,
		gribiC:         gribiC,
		fluentC:        fluentClient,
		fibAck:         fibAck,
		persistence:    persistance,
		redundancyMode: fluent.ElectedPrimaryClient, // all of test cases use ElectedPrimaryClient
		lastElectionID: &grbp.Uint128{Low: 1,
			High: 0,
		},
	}
	if persistance {
		fluentClient.Connection().WithInitialElectionID(g.lastElectionID.Low, g.lastElectionID.High).WithRedundancyMode(fluent.ElectedPrimaryClient).WithPersistence()
	} else {
		fluentClient.Connection().WithInitialElectionID(g.lastElectionID.Low, g.lastElectionID.High).WithRedundancyMode(fluent.ElectedPrimaryClient)
	}
	if fibAck {
		fluentClient.Connection().WithFIBACK()
	}
	fluentClient.Start(context.Background(), t)
	fluentClient.StartSending(context.Background(), t)
	err := g.awaitTimeout(context.Background(), t, timeout)
	if err != nil {
		t.Fatalf("can not establish the session: %v", err)
	}
	return g
}

func (g *GRIBIHandler) Close(t testing.TB) {
	t.Helper()
	t.Logf("closing GRIBI connection for dut: %s", g.dut.Name())
	g.fluentC.Stop(t)
	v := reflect.Indirect(reflect.ValueOf(g.gribiC)).FieldByName("cc")
	clientConn, ok := (reflect.NewAt(v.Type(), unsafe.Pointer(v.UnsafeAddr())).Elem().Interface()).(*grpc.ClientConn)
	if ok {
		clientConn.Close()
	} else {
		t.Fatalf("can not close gribi connection")
	}
}

func (g *GRIBIHandler) awaitTimeout(ctx context.Context, t testing.TB, timeout time.Duration) error {
	subctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return g.fluentC.Await(subctx, t)
}

// get the election id (last known election id) from a dut
func (g *GRIBIHandler) LearnElectionID(t testing.TB) (low, high uint64) {
	t.Helper()
	t.Logf("learn GRIBI Election ID from dut: %s", g.dut.Name())
	g.fluentC.Modify().UpdateElectionID(t, 1, 0)
	err := g.awaitTimeout(context.Background(), t, timeout)
	if err != nil {
		t.Fatalf("learnElectionID Error: %v", err)
	}
	results := g.fluentC.Results(t)
	mu.Lock()
	defer mu.Unlock()
	g.lastElectionID = results[len(results)-1].CurrentServerElectionID
	return g.lastElectionID.Low, g.lastElectionID.High
}

// set the election id
func (g *GRIBIHandler) UpdateElectionID(t *testing.T, lowElecId, highElecId uint64) {
	t.Helper()
	t.Logf("setting GRIBI Election ID for dut: %s to low=%d,high=%d", g.dut.Name(), lowElecId, highElecId)
	g.fluentC.Modify().UpdateElectionID(t, lowElecId, highElecId)
	err := g.awaitTimeout(context.Background(), t, timeout)
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

func (g *GRIBIHandler) GetLastElectionID(t *testing.T) (low, high uint64) {
	return g.lastElectionID.Low, g.lastElectionID.High
}

func (g *GRIBIHandler) BeMaster(t *testing.T) {
	t.Logf("trying to be a master with increasing the election id by one on dut: %s", g.dut.Name())
	lowElecId, highElecId := g.LearnElectionID(t)
	if lowElecId == maxUint64 {
		highElecId = highElecId + 1
	} else {
		lowElecId = lowElecId + 1
	}
	g.UpdateElectionID(t, lowElecId, highElecId)
}

func (g *GRIBIHandler) AddNHG(t *testing.T, nhgIndex uint64, nhs map[uint64]uint64, instance string) {
	NGH := fluent.NextHopGroupEntry().WithNetworkInstance(instance).WithID(nhgIndex)
	for nhIndex, weight := range nhs {
		NGH.AddNextHop(nhIndex, weight)
	}
	g.fluentC.Modify().AddEntry(t, NGH.WithElectionID(g.GetLastElectionID(t)))
	if err := g.awaitTimeout(context.Background(), t, timeout); err != nil {
		t.Fatalf("got unexpected error from server adding NGH, got: %v, want: nil", err)
	}
	ackType := fluent.InstalledInRIB
	if g.fibAck {
		ackType = fluent.InstalledInFIB
	}
	chk.HasResult(t, g.fluentC.Results(t),
		fluent.OperationResult().
			WithNextHopGroupOperation(nhgIndex).
			WithOperationType(constants.Add).
			WithProgrammingResult(ackType).
			AsResult(),
		chk.IgnoreOperationID(),
	)

}

func (g *GRIBIHandler) AddNH(t *testing.T, nhIndex uint64, address, instance string) {
	g.fluentC.Modify().AddEntry(t,
		fluent.NextHopEntry().
			WithNetworkInstance(instance).
			WithIndex(nhIndex).
			WithIPAddress(address).
			WithElectionID(g.GetLastElectionID(t)))

	if err := g.awaitTimeout(context.Background(), t, timeout); err != nil {
		t.Fatalf("got unexpected error from server adding NH, got: %v, want: nil", err)
	}
	ackType := fluent.InstalledInRIB
	if g.fibAck {
		ackType = fluent.InstalledInFIB
	}
	chk.HasResult(t, g.fluentC.Results(t),
		fluent.OperationResult().
			WithNextHopOperation(nhIndex).
			WithOperationType(constants.Add).
			WithProgrammingResult(ackType).
			AsResult(),
		chk.IgnoreOperationID(),
	)
}

func (g *GRIBIHandler) AddIPV4Entry(t *testing.T, nhgIndex uint64, prefix, instance string) {
	g.fluentC.Modify().AddEntry(t,
		fluent.IPv4Entry().WithPrefix(prefix).
			WithNetworkInstance(instance).
			WithNextHopGroup(nhgIndex).
			WithElectionID(g.GetLastElectionID(t)))

	if err := g.awaitTimeout(context.Background(), t, timeout); err != nil {
		t.Fatalf("got unexpected error from server adding NH, got: %v, want: nil", err)
	}
	ackType := fluent.InstalledInRIB
	if g.fibAck {
		ackType = fluent.InstalledInFIB
	}
	chk.HasResult(t, g.fluentC.Results(t),
		fluent.OperationResult().
			WithIPv4Operation(prefix).
			WithOperationType(constants.Add).
			WithProgrammingResult(ackType).
			AsResult(),
		chk.IgnoreOperationID(),
	)
}
