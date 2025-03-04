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
        "fmt"
	"github.com/openconfig/featureprofiles/internal/fptest"
	acctzpb "github.com/openconfig/gnsi/acctz"
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
	t.Log("Generating Acctz via gnmi get Operation")
	t.Logf("System Boot Time %v",bootTime)

        requestTimestamp := &timestamppb.Timestamp{
                Seconds:  0,
                Nanos:   0, 
        }

	acctzClient := dut.RawAPIs().GNSI(t).AcctzStream()

	acctzSubClient, err := acctzClient.RecordSubscribe(context.Background(), &acctzpb.RecordRequest{Timestamp: requestTimestamp})
	if err != nil {
		t.Fatalf("Failed getting accountz record subscribe client, error: %s", err)
	}
	record, err := acctzSubClient.Recv()
	if err != nil {
		t.Fatalf("Failed receiving from accountz record subscribe client, error: %s", err)
	}

	if record.GetHistoryIstruncated() != true {
		t.Fatal("History is not truncated but should be.")
	}
        acctzSubClient.CloseSend()
}
