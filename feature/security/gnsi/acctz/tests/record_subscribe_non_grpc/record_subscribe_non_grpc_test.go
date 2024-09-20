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

package record_subscribe_non_grpc_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

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

func prettyPrint(i interface{}) string {
	s, _ := json.MarshalIndent(i, "", "\t")
	return string(s)
}

func TestAccountzRecordSubscribeNonGRPC(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	acctz.SetupSSHUsers(t, dut)
	var records []acctz.Record

	// Put enough time between the test starting and any prior events so we can easily know where
	// our records start.
	time.Sleep(5 * time.Second)

	startTime := time.Now()
	newRecords := acctz.SendSuccessCliCommand(t, dut)
	records = append(records, newRecords...)
	newRecords = acctz.SendFailCliCommand(t, dut)
	records = append(records, newRecords...)
	newRecords = acctz.SendShellCommand(t, dut)
	records = append(records, newRecords...)

	// Quick sleep to ensure all the records have been processed/ready for us.
	time.Sleep(5 * time.Second)

	// Get gNSI record subscribe client.
	acctzClient := dut.RawAPIs().GNSI(t).Acctz()

	acctzSubClient, err := acctzClient.RecordSubscribe(context.Background())
	if err != nil {
		t.Fatalf("Failed getting accountz record subscribe client, error: %s", err)
	}

	// This will have to move up to RecordSubscribe call after this is brought into FP/Ondatra.
	// https://github.com/openconfig/gnsi/pull/149/files
	err = acctzSubClient.Send(&acctzpb.RecordRequest{
		Timestamp: &timestamppb.Timestamp{
			Seconds: 0,
			Nanos:   0,
		},
	})
	if err != nil {
		t.Fatalf("Failed sending accountz record request, error: %s", err)
	}

	var recordIdx int
	var lastTimestampUnixMillis int64
	var lastTaskID string

	for {
		if recordIdx >= len(records) {
			t.Log("Out of records to process...")
			break
		}

		r := make(chan recordRequestResult)

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

		select {
		case rr := <-r:
			resp = rr
		case <-time.After(10 * time.Second):
			done = true
		}

		if done {
			t.Log("Done receiving records...")
			break
		}

		if resp.err != nil {
			t.Fatalf("Failed receiving record response, error: %s", resp.err)
		}

		if resp.record.GetHistoryIstruncated() {
			t.Fatalf("History is truncated but it shouldn't be, Record Details: %s", prettyPrint(resp.record))
		}

		if !resp.record.Timestamp.AsTime().After(startTime) {
			// Skipping record if it happened before test start time.
			continue
		}

		if resp.record.GetGrpcService().GetServiceType() != acctzpb.GrpcService_GRPC_SERVICE_TYPE_UNSPECIFIED {
			// Skipping gRPC records (if any).
			continue
		}

		// Some task ids may be tracked multiple times (for start/stop accounting). If we see two in
		// a row that are the same task, we can skip this record and continue.
		currentTaskID := resp.record.TaskIds[0]
		if currentTaskID == lastTaskID {
			continue
		}
		lastTaskID = currentTaskID

		// Check that the timestamp for the record is between our start/stop times for our cmd.
		timestamp := resp.record.Timestamp.AsTime()
		if timestamp.UnixMilli() == lastTimestampUnixMillis {
			// This ensures that timestamps are actually changing for each record.
			t.Fatalf("Timestamp is the same as the previous timestamp, this shouldn't be possible!, Record Details: %s", prettyPrint(resp.record))
		}
		lastTimestampUnixMillis = timestamp.UnixMilli()

		// -2 for a little breathing room since things may not be perfectly synced up time-wise.
		if records[recordIdx].StartTime.Unix()-2 > timestamp.Unix() {
			t.Fatalf(
				"Record timestamp is prior to cmd start time timestamp, cmd start timestamp %d, record timestamp %d, Record Details: %s",
				records[recordIdx].StartTime.Unix()-2,
				timestamp.Unix(),
				prettyPrint(resp.record),
			)
		}

		// Done time (that we recorded when making the cmd) + 2 second for some breathing room.
		if records[recordIdx].DoneTime.Unix()+2 < timestamp.Unix() {
			t.Fatalf(
				"Record timestamp is after cmd end timestamp, cmd end timestamp %d, record timestamp %d, Record Details: %s",
				records[recordIdx].DoneTime.Unix()+2,
				timestamp.Unix(),
				prettyPrint(resp.record),
			)
		}

		cmdType := resp.record.GetCmdService().GetServiceType()
		if records[recordIdx].CmdType != cmdType {
			t.Fatalf("Service type not correct, got %q, want %q, Record Details: %s", cmdType, records[recordIdx].CmdType, prettyPrint(resp.record))
		}

		cmd := resp.record.GetCmdService().GetCmd()
		if records[recordIdx].Cmd != cmd {
			t.Fatalf("Command not correct, got %q, want %q, Record Details: %s", cmd, records[recordIdx].Cmd, prettyPrint(resp.record))
		}

		// This channel check maybe should just go away entirely -- see:
		// https://github.com/openconfig/gnsi/issues/98
		// In case of Nokia this is being set to the aaa session id just to have some hopefully
		// useful info in this field to identify a "session" (even if it isn't necessarily ssh/grpc
		// directly).
		channelID := resp.record.GetSessionInfo().GetChannelId()
		if !records[recordIdx].Succeeded {
			if channelID != "aaa_session_id: 0" {
				t.Fatalf("Auth was not successful for this record, but channel id was set, got %q, Record Details: %s", channelID, prettyPrint(resp.record))
			}
		}

		// Tty only set for ssh records.
		tty := resp.record.GetSessionInfo().GetTty()
		if tty == "" {
			t.Fatalf("Should have tty allocated but not set, Record Details: %s", prettyPrint(resp.record))
		}

		sessionStatus := resp.record.GetSessionInfo().GetStatus()
		if records[recordIdx].ExpectedStatus != sessionStatus {
			t.Fatalf("Session status not correct, got %q, want %q, Record Details: %s", sessionStatus, records[recordIdx].ExpectedStatus, prettyPrint(resp.record))
		}

		authenType := resp.record.GetSessionInfo().GetAuthn().GetType()
		if records[recordIdx].ExpectedAuthenType != authenType {
			t.Fatalf("AuthenType not correct, got %q, want %q, Record Details: %s", authenType, records[recordIdx].ExpectedAuthenType, prettyPrint(resp.record))
		}

		authenStatus := resp.record.GetSessionInfo().GetAuthn().GetStatus()
		if records[recordIdx].ExpectedAuthenStatus != authenStatus {
			t.Fatalf("AuthenStatus not correct, got %q, want %q, Record Details: %s", authenStatus, records[recordIdx].ExpectedAuthenStatus, prettyPrint(resp.record))
		}

		authenCause := resp.record.GetSessionInfo().GetAuthn().GetCause()
		if records[recordIdx].ExpectedAuthenCause != authenCause {
			t.Fatalf("AuthenCause not correct, got %q, want %q, Record Details: %s", authenCause, records[recordIdx].ExpectedAuthenCause, prettyPrint(resp.record))
		}

		userIdentity := resp.record.GetSessionInfo().GetUser().GetIdentity()
		if records[recordIdx].ExpectedIdentity != userIdentity {
			t.Fatalf("Identity not correct, got %q, want %q, Record Details: %s", userIdentity, records[recordIdx].ExpectedIdentity, prettyPrint(resp.record))
		}

		if records[recordIdx].Succeeded {
			// Verify the l4 bits align, this is only set if auth is successful so do it down here.
			localAddr := resp.record.GetSessionInfo().GetLocalAddress()
			if records[recordIdx].LocalIP != localAddr {
				t.Fatalf("Local address not correct, got %q, want %q, Record Details: %s", localAddr, records[recordIdx].LocalIP, prettyPrint(resp.record))
			}

			localPort := resp.record.GetSessionInfo().GetLocalPort()
			if records[recordIdx].LocalPort != localPort {
				t.Fatalf("Local port not correct, got %d, want %d, Record Details: %s", localPort, records[recordIdx].LocalPort, prettyPrint(resp.record))
			}

			remoteAddr := resp.record.GetSessionInfo().GetRemoteAddress()
			if records[recordIdx].RemoteIP != remoteAddr {
				t.Fatalf("Remote address not correct, got %q, want %q, Record Details: %s", remoteAddr, records[recordIdx].RemoteIP, prettyPrint(resp.record))
			}

			remotePort := resp.record.GetSessionInfo().GetRemotePort()
			if records[recordIdx].RemotePort != remotePort {
				t.Fatalf("Remote port not correct, got %d, want %d, Record Details: %s", remotePort, records[recordIdx].RemotePort, prettyPrint(resp.record))
			}
		}

		t.Logf("Processed Record: %s", prettyPrint(resp.record))
		recordIdx++
	}

	if recordIdx != len(records) {
		t.Fatal("Did not process all records.")
	}
}
