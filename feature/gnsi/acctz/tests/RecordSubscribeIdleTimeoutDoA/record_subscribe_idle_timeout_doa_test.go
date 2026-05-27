package record_subscribe_idle_timeout_doa_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/helpers"
	acctzpb "github.com/openconfig/gnsi/acctz"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	defaultIdleTimeout = 120 * time.Second
	gracePeriod        = 60 * time.Second
	gnsiProfileName    = "GNSI_PROFILE"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func isStreamClosedErr(err error) bool {
	if err == nil {
		return false
	}
	if st, ok := status.FromError(err); ok {
		switch st.Code() {
		case codes.Canceled, codes.Unavailable:
			return true
		default:
			return false
		}
	}
	// Non-gRPC error (e.g., EOF) indicates closed stream.
	return true
}

func prettyPrint(i any) string {
	s, err := json.MarshalIndent(i, "", "\t")
	if err != nil {
		return fmt.Sprintf("<error: %v>", err)
	}
	return string(s)
}

func TestRecordSubscribeIdleTimeoutDoA(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	configureGnsiServiceCLI(t, dut)

	systemTime := gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State())
	startTime, err := time.Parse(time.RFC3339, systemTime)
	if err != nil {
		t.Errorf("Failed to parse system time %q: %v", systemTime, err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), defaultIdleTimeout+gracePeriod+30*time.Second)
	defer cancel()

	acctzClient := dut.RawAPIs().GNSI(t).AcctzStream()
	acctzSubClient, err := acctzClient.RecordSubscribe(ctx, &acctzpb.RecordRequest{Timestamp: timestamppb.New(startTime)})
	if err != nil {
		t.Fatalf("Failed sending accountz record request, error: %s", err)
	}
	defer acctzSubClient.CloseSend()

	t.Log("RecordSubscribe stream established successfully.")
	t.Logf("Waiting %v to trigger idle timeout... %v", defaultIdleTimeout+gracePeriod, time.Now())
	time.Sleep(defaultIdleTimeout + gracePeriod)
	t.Log("Idle timeout and grace period passed:", time.Now())
	t.Logf("Try to receive a record after idle timeout: %v", time.Now())
	recv, recvErr := acctzSubClient.Recv()
	t.Logf("%v Try to log recv: %v", time.Now(), prettyPrint(recv))

	if recvErr == nil {
		t.Errorf("RecordSubscribe stream is still active after the idle timeout period, but it should have been closed by the DUT.")
		return
	}

	if !isStreamClosedErr(recvErr) {
		t.Errorf("RecordSubscribe stream returned unexpected error after idle timeout: %v", recvErr)
		return
	}
	t.Logf("Stream closed as expected after idle timeout: %v", recvErr)
}

func configureGnsiServiceCLI(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	t.Log("Configuring gnsi service through CLI")
	gnsiServerConfig := fmt.Sprintf(`
	management api gnmi
    transport grpc default
      ssl profile %s
	!
	management api gnsi
    transport gnmi default
    service acctz
    !
`, gnsiProfileName)
	helpers.GnmiCLIConfig(t, dut, gnsiServerConfig)
}
