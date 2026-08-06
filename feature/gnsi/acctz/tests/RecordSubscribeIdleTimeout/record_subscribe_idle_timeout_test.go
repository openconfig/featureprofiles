package record_subscribe_idle_timeout_test

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/security/acctz"
	acctzpb "github.com/openconfig/gnsi/acctz"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	idleTimeout          = 120 * time.Second
	connectionCloseGrace = 30 * time.Second
	recvTimeout          = 20 * time.Second
	timeUntilIdle        = 10 * time.Second
)

type recvResult struct {
	resp *acctzpb.RecordResponse
	err  error
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func startRecv(t *testing.T, stream grpc.ServerStreamingClient[acctzpb.RecordResponse], ch chan recvResult) {
	go func() {
		resp, err := stream.Recv()
		ch <- recvResult{resp, err}
	}()
}

func drainRecords(t *testing.T, stream grpc.ServerStreamingClient[acctzpb.RecordResponse], timeout time.Duration) (int, chan recvResult) {
	t.Helper()
	ch := make(chan recvResult, 1)

	startRecv(t, stream, ch)
	n := 0
	for {
		select {
		case r := <-ch:
			if r.err != nil {
				ch <- r
				return n, ch
			}
			t.Logf("Received record %d: %v", n+1, r.resp)
			n++
			startRecv(t, stream, ch)
		case <-time.After(timeout):
			return n, ch
		}
	}
}

func getSystemTime(t *testing.T, dut *ondatra.DUTDevice) time.Time {
	t.Helper()
	s := gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State())
	ts, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		t.Fatalf("Failed to parse DUT system time %q: %v", s, err)
	}
	return ts
}

func TestRecordSubscribeIdleTimeout(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	acctzClient := dut.RawAPIs().GNSI(t).AcctzStream()
	acctz.SetupUsers(t, dut, true)

	t.Log("First RecordSubscribe, discarding records")
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	firstSub, err := acctzClient.RecordSubscribe(ctx, &acctzpb.RecordRequest{
		Timestamp: timestamppb.New(getSystemTime(t, dut)),
	})
	if err != nil {
		t.Fatalf("First RecordSubscribe failed: %v", err)
	}
	n, _ := drainRecords(t, firstSub, recvTimeout)
	t.Logf("First subscription: discarded %d record(s) within %v seconds.", n, recvTimeout.Seconds())

	wait := idleTimeout - timeUntilIdle
	t.Logf("Waiting %v to approach the idle-timeout window", wait)
	time.Sleep(wait)

	t.Log("Refresh RecordSubscribe, discarding records")
	refreshSub, err := acctzClient.RecordSubscribe(t.Context(), &acctzpb.RecordRequest{
		Timestamp: timestamppb.New(getSystemTime(t, dut)),
	})
	if err != nil {
		t.Fatalf("Refresh RecordSubscribe failed: %v", err)
	}
	n, refreshCh := drainRecords(t, refreshSub, recvTimeout)
	t.Logf("Refresh subscription: discarded %d record(s) within %v seconds.", n, recvTimeout.Seconds())

	wait = idleTimeout + connectionCloseGrace
	t.Logf("Waiting %v to reach the idle-timeout window", wait)
	time.Sleep(wait)

	t.Log("Verifying DUT stream closed after the idle timeout")
	select {
	case r := <-refreshCh:
		switch {
		case r.err == nil:
			t.Fatalf("stream still open; received unexpected record: %v", r.resp)
		case r.err == io.EOF:
			t.Log("DUT closed the stream with EOF after idle timeout.")
		default:
			st, ok := status.FromError(r.err)
			if ok && st.Code() == codes.Canceled {
				t.Fatalf("stream cancelled by client, not DUT: %v", r.err)
			}
			t.Logf("DUT closed the stream after idle timeout (code=%v): %v", st.Code(), r.err)
		}
	case <-time.After(recvTimeout):
		t.Fatalf("DUT did not close the stream within %v after idle timeout", recvTimeout)
	}
}
