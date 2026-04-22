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

package recordsubscribenongrpc_test

import (
	"context"
	"encoding/json"
	"flag"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/security/acctz"
	acctzpb "github.com/openconfig/gnsi/acctz"
	"github.com/openconfig/ondatra"
)

type recordRequestResult struct {
	record *acctzpb.RecordResponse
	err    error
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func prettyPrint(i any) string {
	s, _ := json.MarshalIndent(i, "", "\t")
	return string(s)
}

var (
	staticBinding = flag.Bool("static_binding", false, "set this flag to true if test is run for testbed using static binding")
)

func TestAccountzRecordSubscribeNonGRPC(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	acctz.SetupUsers(t, dut, true)
	var records []*acctzpb.RecordResponse

	// Put enough time between the test starting and any prior events so we can easily know where
	// our records start.
	startTime := time.Now().Add(-10 * time.Second)

	newRecords := acctz.SendSuccessCliCommand(t, dut, *staticBinding)
	records = append(records, newRecords...)
	if !deviations.AcctzRecordFailCommandUnsupported(dut) {
		newRecords = acctz.SendFailCliCommand(t, dut, *staticBinding)
		records = append(records, newRecords...)
	}
	if !deviations.AcctzShellCmdAccountingUnsupported(dut) {
		newRecords = acctz.SendShellCommand(t, dut, *staticBinding)
		records = append(records, newRecords...)
	}

	// Quick sleep to ensure all the records have been processed/ready for us.
	time.Sleep(5 * time.Second)

	// Get gNSI record subscribe client.
	requestTimestamp := &timestamppb.Timestamp{
		Seconds: 0,
		Nanos:   0,
	}
	acctzClient := dut.RawAPIs().GNSI(t).AcctzStream()
	acctzSubClient, err := acctzClient.RecordSubscribe(context.Background(), &acctzpb.RecordRequest{Timestamp: requestTimestamp})
	if err != nil {
		t.Fatalf("Failed sending accountz record request, error: %s", err)
	}
	defer acctzSubClient.CloseSend()

	var recordIdx int
	var lastTimestampUnixMillis int64
	r := make(chan recordRequestResult)

	// Ignore proto fields which are set internally by the DUT (cannot be matched exactly)
	// and compare them manually later.
	popts := []cmp.Option{protocmp.Transform(),
		protocmp.IgnoreFields(&acctzpb.RecordResponse{}, "timestamp", "task_ids", "component_name"),
		protocmp.IgnoreFields(&acctzpb.AuthzDetail{}, "detail"),
		protocmp.IgnoreFields(&acctzpb.AuthnDetail{}, "type", "cause"),
		protocmp.IgnoreFields(&acctzpb.UserDetail{}, "role"),
		protocmp.IgnoreFields(&acctzpb.CommandService{}, "cmd", "cmd_args"),
		protocmp.IgnoreFields(&acctzpb.SessionInfo{}, "channel_id", "tty", "local_address", "local_port", "remote_address", "remote_port"),
	}
	for {
		if recordIdx >= len(records) {
			t.Log("Out of records to process...")
			break
		}

		// Read single acctz record from stream into channel.
		go func(r chan recordRequestResult) {
			var response *acctzpb.RecordResponse
			response, err = acctzSubClient.Recv()
			r <- recordRequestResult{
				record: response,
				err:    err,
			}
		}(r)

		var done bool
		var resp recordRequestResult

		// Read acctz record from channel for evaluation.
		// Timeout and exit if no records received on the channel for some time.
		select {
		case rr := <-r:
			resp = rr
		case <-time.After(10 * time.Second):
			done = true
		}
		if done {
			t.Log("Done receiving records (timeout or manual break)...")
			break
		}
		t.Logf("Received record: %s", prettyPrint(resp.record))

		if resp.err != nil {
			t.Fatalf("Failed receiving record response, error: %s", resp.err)
		}

		cmdServiceRecord := resp.record.GetCmdService()
		// Skip records which are non CMD type (e.g. gNMI, gNSI, etc).
		if cmdServiceRecord.GetServiceType() != acctzpb.CommandService_CMD_SERVICE_TYPE_CLI &&
			cmdServiceRecord.GetServiceType() != acctzpb.CommandService_CMD_SERVICE_TYPE_SHELL {
			t.Logf("Skipping record: not CLI type (got %v)", cmdServiceRecord.GetServiceType())
			continue
		}

		if !resp.record.Timestamp.AsTime().After(startTime) {
			t.Logf("Skipping record: timestamp %v not after start time %v", resp.record.Timestamp.AsTime(), startTime)
			continue
		}

		// Skip start/stop accounting records if present.
		sessionStatus := resp.record.GetSessionInfo().GetStatus()
		if sessionStatus == acctzpb.SessionInfo_SESSION_STATUS_LOGIN || sessionStatus == acctzpb.SessionInfo_SESSION_STATUS_LOGOUT {
			t.Logf("Skipping record: login/logout status (%v)", sessionStatus)
			continue
		}

		// Skip records from unknown users (e.g. gnetch-ro)
		foundUser := false
		userIdentity := resp.record.GetSessionInfo().GetUser().GetIdentity()
		for _, r := range records {
			if r.GetSessionInfo().GetUser().GetIdentity() == userIdentity {
				foundUser = true
				break
			}
		}
		if !foundUser {
			t.Logf("Skipping record from unknown user: %s", userIdentity)
			continue
		}

		timestamp := resp.record.Timestamp.AsTime()
		if timestamp.UnixMilli() == lastTimestampUnixMillis {
			// This ensures that timestamps are actually changing for each record.
			t.Errorf("Timestamp is the same as the previous timestamp, this shouldn't be possible!, Record Details: %s", prettyPrint(resp.record))
		}
		lastTimestampUnixMillis = timestamp.UnixMilli()

		// Verify acctz proto bits.
		if diff := cmp.Diff(resp.record, records[recordIdx], popts...); diff != "" {
			t.Errorf("got diff in got/want: %s", diff)
		}

		// Verify command matches even if split between cmd and cmd_args.
		gotCmd := strings.TrimSpace(resp.record.GetCmdService().GetCmd() + " " + strings.Join(resp.record.GetCmdService().GetCmdArgs(), " "))
		wantCmd := strings.TrimSpace(records[recordIdx].GetCmdService().GetCmd() + " " + strings.Join(records[recordIdx].GetCmdService().GetCmdArgs(), " "))
		if gotCmd != wantCmd {
			t.Errorf("Command mismatch: got %q, want %q", gotCmd, wantCmd)
		}

		// Verify record timestamp is after request timestamp.
		if !timestamp.After(requestTimestamp.AsTime()) {
			t.Errorf("Record timestamp is before record request timestamp %v, Record Details: %v", requestTimestamp.AsTime(), prettyPrint(resp.record))
		}

		// This channel check maybe should just go away entirely -- see:
		// https://github.com/openconfig/gnsi/issues/98
		// In case of Nokia this is being set to the aaa session id just to have some hopefully
		// useful info in this field to identify a "session" (even if it isn't necessarily ssh/grpc
		// directly).
		if resp.record.GetSessionInfo().GetChannelId() == "" && !deviations.AcctzRecordSessionChannelIdUnsupported(dut) {
			t.Errorf("Channel Id is not populated for record: %v", prettyPrint(resp.record))
		}

		// Tty only set for ssh records.
		if resp.record.GetSessionInfo().GetTty() == "" {
			t.Errorf("Should have tty allocated but not set, Record Details: %s", prettyPrint(resp.record))
		}

		// Verify authz detail is populated for denied cmds.
		authzInfo := resp.record.GetCmdService().GetAuthz()
		if authzInfo.Status == acctzpb.AuthzDetail_AUTHZ_STATUS_DENY && authzInfo.GetDetail() == "" {
			t.Errorf("Authorization detail is not populated for record: %v", prettyPrint(resp.record))
		}

		t.Logf("Processed Record: %s", prettyPrint(resp.record))
		recordIdx++
	}
	t.Logf("recordIdx: %d, len(records): %d", recordIdx, len(records))
	if recordIdx != len(records) {
		t.Fatal("Did not process all records.")
	}
}
