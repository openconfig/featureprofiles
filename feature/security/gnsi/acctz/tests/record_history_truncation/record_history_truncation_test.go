package record_history_truncation_test

import (
	"context"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/gnsi/acctz"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestAccountzRecordHistoryTruncation(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	systemState := gnmi.Get(t, dut, gnmi.OC().System().State())

	bootTime := systemState.GetBootTime()

	// try to get records from 1 day prior to devices boot time
	recordStartTime := time.Unix(0, int64(bootTime)).Add(-24 * time.Hour)

	acctzClient := dut.RawAPIs().GNSI(t).Acctz()

	acctzSubClient, err := acctzClient.RecordSubscribe(context.Background())
	if err != nil {
		t.Fatalf("failed getting accountz record subscribe client, error: %s", err)
	}

	err = acctzSubClient.Send(&acctz.RecordRequest{
		Timestamp: timestamppb.New(recordStartTime),
	})
	if err != nil {
		t.Fatalf("failed sending record request, error: %s", err)
	}

	record, err := acctzSubClient.Recv()
	if err != nil {
		t.Fatalf("failed receiving from accountz record subscribe client, error: %s", err)
	}

	if record.GetHistoryIstruncated() != true {
		t.Fatal("history is not truncated but should be")
	}
}
