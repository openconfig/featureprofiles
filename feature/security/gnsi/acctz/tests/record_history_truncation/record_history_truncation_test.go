// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package record_history_truncation_test

import (
	"context"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/fptest"
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

	// Try to get records from 1 day prior to device's boot time.
	recordStartTime := time.Unix(0, int64(bootTime)).Add(-24 * time.Hour)

	acctzClient := dut.RawAPIs().GNSI(t).Acctz()

	acctzSubClient, err := acctzClient.RecordSubscribe(context.Background())
	if err != nil {
		t.Fatalf("Failed getting accountz record subscribe client, error: %s", err)
	}

	err = acctzSubClient.Send(&acctz.RecordRequest{
		Timestamp: timestamppb.New(recordStartTime),
	})
	if err != nil {
		t.Fatalf("Failed sending record request, error: %s", err)
	}

	record, err := acctzSubClient.Recv()
	if err != nil {
		t.Fatalf("Failed receiving from accountz record subscribe client, error: %s", err)
	}

	if record.GetHistoryIstruncated() != true {
		t.Fatal("History is not truncated but should be.")
	}
}
