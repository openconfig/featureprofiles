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

package recordsubscribefull_test

import (
	"encoding/json"
	"slices"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/security/acctz"
	acctzpb "github.com/openconfig/gnsi/acctz"
	"github.com/openconfig/ondatra"
	"google.golang.org/protobuf/types/known/timestamppb"
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

func TestAccountzRecordSubscribeFull(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	acctz.SetupUsers(t, dut, false)

	startTime := time.Now()

	// Get gNSI record subscribe client.
	requestTimestamp := &timestamppb.Timestamp{
		Seconds: startTime.Unix(),
		Nanos:   0,
	}
	acctzClient := dut.RawAPIs().GNSI(t).AcctzStream()
	acctzSubClient, err := acctzClient.RecordSubscribe(t.Context(), &acctzpb.RecordRequest{Timestamp: requestTimestamp})
	if err != nil {
		t.Fatalf("Failed sending accountz record request, error: %s", err)
	}
	defer acctzSubClient.CloseSend()

	_, err = deviceRecords(t, acctzSubClient, time.Minute)
	if err != nil {
		t.Fatalf("Failed receiving record response, error: %s", err)
	}

	var wantRecords []*acctzpb.RecordResponse
	nr := acctz.SendGnmiRPCs(t, dut)
	wantRecords = append(wantRecords, nr...)
	nr = acctz.SendGnoiRPCs(t, dut)
	wantRecords = append(wantRecords, nr...)
	nr = acctz.SendGnsiRPCs(t, dut)
	wantRecords = append(wantRecords, nr...)
	nr = acctz.SendGribiRPCs(t, dut)
	if !deviations.GribiRecordsUnsupported(dut) {
		wantRecords = append(wantRecords, nr...)
	}
	if !deviations.P4RTCapabilitiesUnsupported(dut) {
		nr = acctz.SendP4rtRPCs(t, dut)
		wantRecords = append(wantRecords, nr...)
	}

	rec, err := deviceRecords(t, acctzSubClient, time.Minute)
	if err != nil {
		t.Fatalf("Failed receiving record response, error: %s", err)
	}

	// Filter out records that are not for the success or fail usernames.
	var gotRecords []*acctzpb.RecordResponse
	type key struct {
		path string
		id   string
	}
	foundMap := make(map[key]bool)
	for _, r := range rec {
		path := r.GetGrpcService().GetRpcName()
		id := r.GetSessionInfo().GetUser().GetIdentity()
		// Skip if the path is not in the list of paths to be tested or if the id is not a success or fail username.
		if !slices.Contains(acctz.TestPaths, path) || !slices.Contains([]string{acctz.SuccessUsername, acctz.FailUsername}, id) {
			continue
		}
		if foundMap[key{path: path, id: id}] {
			continue
		}

		foundMap[key{path: path, id: id}] = true
		gotRecords = append(gotRecords, r)
	}
	if len(wantRecords) != len(gotRecords) {
		t.Errorf("Got %d records, want %d", len(gotRecords), len(wantRecords))
	}

	// Ignore proto fields which are set internally by the DUT (cannot be matched exactly)
	// and compare them manually later.
	popts := []cmp.Option{
		protocmp.Transform(),
		protocmp.IgnoreFields(&acctzpb.RecordResponse{}, "timestamp", "task_ids"),
		protocmp.IgnoreFields(&acctzpb.AuthzDetail{}, "detail"),
		protocmp.IgnoreFields(&acctzpb.SessionInfo{}, "ip_proto", "channel_id", "local_address", "local_port", "remote_address", "remote_port", "status", "authn"),
		protocmp.IgnoreFields(&acctzpb.UserDetail{}, "role"),
		protocmp.IgnoreFields(&acctzpb.GrpcService{}, "proto_val", "payload_istruncated"),
	}

	var recordIdx int
	var lastTimestampUnixMillis int64
	for recordIdx < len(gotRecords) && recordIdx < len(wantRecords) {
		record := gotRecords[recordIdx]

		if record.GetHistoryIstruncated() {
			t.Errorf("History is truncated but it shouldn't be, Record Details: %s", prettyPrint(record))
		}

		timestamp := record.Timestamp.AsTime()
		if timestamp.UnixMilli() == lastTimestampUnixMillis {
			// This ensures that timestamps are actually changing for each record.
			t.Errorf("Timestamp is the same as the previous timestamp, this shouldn't be possible!, Record Details: %s", prettyPrint(record))
		}
		lastTimestampUnixMillis = timestamp.UnixMilli()

		// Verify acctz proto bits.
		if diff := cmp.Diff(record, wantRecords[recordIdx], popts...); diff != "" {
			t.Errorf("Got diff in got/want: %s", diff)
		}

		// Verify record timestamp is after request timestamp.
		if !timestamp.After(requestTimestamp.AsTime()) {
			t.Errorf("Record timestamp is before record request timestamp %v, Record Details: %v", requestTimestamp.AsTime(), prettyPrint(record))
		}

		// This channel check maybe should just go away entirely -- see:
		// https://github.com/openconfig/gnsi/issues/98
		// In case of Nokia this is being set to the aaa session id just to have some hopefully
		// useful info in this field to identify a "session" (even if it isn't necessarily ssh/grpc
		// directly).
		if record.GetSessionInfo().GetChannelId() == "" {
			t.Errorf("Channel Id is not populated for record: %v", prettyPrint(record))
		}

		// Verify authz detail is populated for denied rpcs.
		authzInfo := record.GetGrpcService().GetAuthz()
		if authzInfo.GetStatus() == acctzpb.AuthzDetail_AUTHZ_STATUS_DENY && authzInfo.GetDetail() == "" {
			t.Errorf("Authorization detail is not populated for record: %v", prettyPrint(record))
		}

		t.Logf("Processed Record: %s", prettyPrint(record))
		recordIdx++
	}
}

type recvClient interface {
	Recv() (*acctzpb.RecordResponse, error)
}

func deviceRecords(t *testing.T, client recvClient, deadline time.Duration) ([]*acctzpb.RecordResponse, error) {
	rChan := make(chan recordRequestResult)
	defer close(rChan)
	go func(ch chan recordRequestResult, c recvClient) {
		defer func() {
			if r := recover(); r != nil {
				return
			}
		}()
		for {
			resp, err := c.Recv()
			ch <- recordRequestResult{record: resp, err: err}
		}
	}(rChan, client)
	var rs []*acctzpb.RecordResponse
	startTime := time.Now()
	for time.Since(startTime) < deadline {
		select {
		case r := <-rChan:
			if r.err != nil {
				close(rChan)
				return rs, r.err
			}
			rs = append(rs, r.record)
		case <-time.After(10 * time.Second):
			continue
		}
	}
	return rs, nil
}
