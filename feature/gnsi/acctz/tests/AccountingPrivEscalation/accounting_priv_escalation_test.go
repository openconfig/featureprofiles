package accounting_priv_escalation_test

import (
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/featureprofiles/internal/security/acctz"
	acctzpb "github.com/openconfig/gnsi/acctz"
	"github.com/openconfig/ondatra"
)

const (
	staticBinding = false
	recordTimeout = 30 * time.Second
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

func TestAccountzRecordSubscribePrivEscalation(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	acctz.SetupUsers(t, dut, true)

	startTime := helpers.GetRouterTime(t, dut)
	time.Sleep(5 * time.Second)

	records := acctz.SendPrivEscalation(t, dut, staticBinding)
	time.Sleep(5 * time.Second)

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

	var recordIdx int
	var lastTimestampUnixMillis int64
	var loginRecords []*acctzpb.RecordResponse
	r := make(chan recordRequestResult)

	popts := []cmp.Option{protocmp.Transform(),
		protocmp.IgnoreFields(&acctzpb.RecordResponse{}, "timestamp", "task_ids", "component_name"),
		protocmp.IgnoreFields(&acctzpb.AuthzDetail{}, "detail"),
		protocmp.IgnoreFields(&acctzpb.AuthnDetail{}, "type", "cause"),
		protocmp.IgnoreFields(&acctzpb.UserDetail{}, "role"),
		protocmp.IgnoreFields(&acctzpb.CommandService{}, "cmd", "cmd_args", "authz"),
		protocmp.IgnoreFields(&acctzpb.GrpcService{}, "authz"),
		protocmp.IgnoreFields(&acctzpb.SessionInfo{}, "channel_id", "tty", "local_address", "local_port", "remote_address", "remote_port"),
	}
	for {
		if recordIdx >= len(records) {
			t.Log("Out of records to process...")
			break
		}

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
		case <-time.After(recordTimeout):
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

		sessionStatus := resp.record.GetSessionInfo().GetStatus()
		expectedSessionStatus := records[recordIdx].GetSessionInfo().GetStatus()
		if sessionStatus != expectedSessionStatus {
			if sessionStatus == acctzpb.SessionInfo_SESSION_STATUS_LOGIN && resp.record.Timestamp.AsTime().After(startTime) {
				loginRecords = append(loginRecords, resp.record)
			}
			t.Logf("Skipping record: status %v doesn't match expected %v", sessionStatus, expectedSessionStatus)
			continue
		}

		if !resp.record.Timestamp.AsTime().After(startTime) {
			t.Logf("Skipping record: timestamp %v not after start time %v", resp.record.Timestamp.AsTime(), startTime)
			continue
		}

		foundUser := false
		userIdentity := resp.record.GetSessionInfo().GetUser().GetIdentity()
		for _, rec := range records {
			t.Logf("Got %s want %s", rec.GetSessionInfo().GetUser().GetIdentity(), userIdentity)
			if rec.GetSessionInfo().GetUser().GetIdentity() == userIdentity {
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
			t.Errorf("Timestamp is the same as the previous timestamp, Record Details: %s", prettyPrint(resp.record))
		}
		lastTimestampUnixMillis = timestamp.UnixMilli()

		if diff := cmp.Diff(resp.record, records[recordIdx], popts...); diff != "" {
			t.Errorf("got diff in got/want: %s", diff)
		}

		gotSession := resp.record.GetSessionInfo()
		wantSession := records[recordIdx].GetSessionInfo()
		verifyReportedString(t, "local_address", gotSession.GetLocalAddress(), wantSession.GetLocalAddress())
		verifyReportedUint32(t, "local_port", gotSession.GetLocalPort(), wantSession.GetLocalPort())
		verifyReportedString(t, "remote_address", gotSession.GetRemoteAddress(), wantSession.GetRemoteAddress())
		verifyReportedUint32(t, "remote_port", gotSession.GetRemotePort(), wantSession.GetRemotePort())

		if !timestamp.After(requestTimestamp.AsTime()) {
			t.Errorf("Record timestamp is before record request timestamp %v, Record Details: %v", requestTimestamp.AsTime(), prettyPrint(resp.record))
		}

		if resp.record.GetSessionInfo().GetStatus() != expectedSessionStatus {
			t.Errorf("Session status mismatch, got %v want %v, Record Details: %s",
				resp.record.GetSessionInfo().GetStatus(), expectedSessionStatus, prettyPrint(resp.record))
		}
		if expectedSessionStatus != acctzpb.SessionInfo_SESSION_STATUS_ENABLE {
			t.Logf("session_info.status=%v; README requires ENABLE", expectedSessionStatus)
		}

		authnInfo := resp.record.GetSessionInfo().GetAuthn()
		expectedAuthnStatus := records[recordIdx].GetSessionInfo().GetAuthn().GetStatus()
		if authnInfo.GetStatus() != expectedAuthnStatus {
			t.Errorf("Authentication status mismatch, got %v want %v, Record Details: %s",
				authnInfo.GetStatus(), expectedAuthnStatus, prettyPrint(resp.record))
		}
		expectedAuthnType := records[recordIdx].GetSessionInfo().GetAuthn().GetType()
		if authnInfo.GetType() == acctzpb.AuthnDetail_AUTHN_TYPE_UNSPECIFIED {
			t.Logf("Authentication type is unspecified; want %v", expectedAuthnType)
		} else if authnInfo.GetType() != expectedAuthnType {
			t.Errorf("Authentication type mismatch, got %v want %v, Record Details: %s",
				authnInfo.GetType(), expectedAuthnType, prettyPrint(resp.record))
		}

		if authnInfo.GetStatus() == acctzpb.AuthnDetail_AUTHN_STATUS_FAIL && authnInfo.GetCause() == "" {
			t.Errorf("Authentication cause is not populated for failed privilege escalation, Record Details: %s", prettyPrint(resp.record))
		}

		if resp.record.GetSessionInfo().GetUser().GetIdentity() == "" {
			t.Errorf("User identity is not populated, Record Details: %s", prettyPrint(resp.record))
		}

		if mapRoleToPrivilegeLevel(resp.record.GetSessionInfo().GetUser().GetRole()) == 0 {
			t.Errorf("privilege_level is not populated for privilege escalation record, Record Details: %s", prettyPrint(resp.record))
		}

		if resp.record.GetSessionInfo().GetChannelId() == "" && !deviations.AcctzRecordSessionChannelIdUnsupported(dut) {
			t.Errorf("Channel Id is not populated for record: %v", prettyPrint(resp.record))
		} else if resp.record.GetSessionInfo().GetChannelId() != "" && resp.record.GetSessionInfo().GetChannelId() != "0" {
			t.Logf("channel_id should be 0 for SSH per README, got %q; vendor deviation", resp.record.GetSessionInfo().GetChannelId())
		}

		if resp.record.GetSessionInfo().GetTty() == "" {
			t.Errorf("Should have tty allocated but not set, Record Details: %s", prettyPrint(resp.record))
		}

		cmdServiceRecord := resp.record.GetCmdService()
		grpcServiceRecord := resp.record.GetGrpcService()
		if cmdServiceRecord == nil {
			t.Errorf("cmd_service is not populated for CLI privilege escalation, Record Details: %s", prettyPrint(resp.record))
		} else {
			if cmdServiceRecord.GetServiceType() == acctzpb.CommandService_CMD_SERVICE_TYPE_UNSPECIFIED {
				t.Errorf("cmd_service.service_type is unspecified, Record Details: %s", prettyPrint(resp.record))
			}
			if cmdServiceRecord.GetCmdIstruncated() {
				t.Errorf("cmd_service.cmd_istruncated should be omitted/false for priv escalation record, Record Details: %s", prettyPrint(resp.record))
			}
			if cmdServiceRecord.GetCmd() != "" {
				t.Logf("cmd_service.cmd populated with %q", cmdServiceRecord.GetCmd())
			}
			if cmdServiceRecord.GetCmdArgsIstruncated() {
				t.Errorf("cmd_service.cmd_args_istruncated should be omitted/false for priv escalation record, Record Details: %s", prettyPrint(resp.record))
			}
			if len(cmdServiceRecord.GetCmdArgs()) != 0 {
				t.Logf("cmd_service.cmd_args populated with %v", cmdServiceRecord.GetCmdArgs())
			}
			if cmdServiceRecord.GetAuthz().GetDetail() != "" {
				t.Errorf("cmd_service.authz.detail should be omitted, got %q, Record Details: %s", cmdServiceRecord.GetAuthz().GetDetail(), prettyPrint(resp.record))
			}
		}
		if grpcServiceRecord != nil {
			t.Errorf("grpc_service should be omitted for CLI privilege escalation, Record Details: %s", prettyPrint(resp.record))
		}

		tty := resp.record.GetSessionInfo().GetTty()
		remoteAddr := resp.record.GetSessionInfo().GetRemoteAddress()
		remotePort := resp.record.GetSessionInfo().GetRemotePort()
		foundLogin := false
		for _, lr := range loginRecords {
			lrTty := lr.GetSessionInfo().GetTty()
			lrRemoteAddr := lr.GetSessionInfo().GetRemoteAddress()
			lrRemotePort := lr.GetSessionInfo().GetRemotePort()
			if (tty != "" && lrTty == tty) ||
				(remoteAddr != "" && lrRemoteAddr == remoteAddr && lrRemotePort == remotePort) {
				foundLogin = true
				break
			}
		}
		if !foundLogin {
			t.Logf("No accompanying SESSION_STATUS_LOGIN record found for ENABLE record (tty=%q remote=%s:%d); start/stop accounting may not be supported on this platform", tty, remoteAddr, remotePort)
		}

		t.Logf("Processed Record: %s", prettyPrint(resp.record))
		recordIdx++
	}

	if recordIdx != len(records) {
		t.Fatal("Did not process all records.")
	}
	t.Logf("Processed %d records", len(records))
}

func mapRoleToPrivilegeLevel(role string) int {
	switch role {
	case "admin", "network-admin":
		return 15
	case "root-lr, cisco-support", "root-system":
		return 15
	case "operator", "network-operator":
		return 10
	case "viewer", "read-only":
		return 1
	default:
		level, err := strconv.Atoi(role)
		if err == nil && level > 0 {
			return level
		}
		return 0
	}
}

func verifyReportedString(t *testing.T, field, got, want string) {
	t.Helper()
	if got == "" {
		t.Logf("%s is not populated; want %q", field, want)
		return
	}
	if want != "" && got != want {
		t.Errorf("%s mismatch: got %q, want %q", field, got, want)
	}
}

func verifyReportedUint32(t *testing.T, field string, got, want uint32) {
	t.Helper()
	if got == 0 {
		t.Logf("%s is not populated; want %d", field, want)
		return
	}
	if want != 0 && got != want {
		t.Errorf("%s mismatch: got %d, want %d", field, got, want)
	}
}
