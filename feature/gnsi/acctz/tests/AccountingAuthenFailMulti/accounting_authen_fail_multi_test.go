package accounting_authen_fail_multi_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"flag"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/featureprofiles/internal/security/acctz"
	acctzpb "github.com/openconfig/gnsi/acctz"
	"github.com/openconfig/ondatra"
	"golang.org/x/crypto/ssh"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type recordRequestResult struct {
	record *acctzpb.RecordResponse
	err    error
}

// connMetadata stores the connection metadata captured during an SSH attempt
// for later correlation with accounting records.
type connMetadata struct {
	description string
	username    string
	localAddr   string
	localPort   uint32
	remoteAddr  string
	remotePort  uint32
}

var (
	staticBinding = flag.Bool("static_binding", false, "set this flag to true if test is run for testbed using static binding")
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func prettyPrint(i any) string {
	s, _ := json.MarshalIndent(i, "", "\t")
	return string(s)
}

// invalidCredentials defines a set of invalid credentials to use for authentication failure tests.
type invalidCredentials struct {
	description string
	username    string
	password    string
	authnType   acctzpb.AuthnDetail_AuthnType
}

// InvalidCredentialSets returns the set of invalid password credential combinations to test.
func InvalidCredentialSets() []invalidCredentials {
	return []invalidCredentials{
		{
			description: "empty username",
			username:    "",
			password:    "somepassword",
			authnType:   acctzpb.AuthnDetail_AUTHN_TYPE_PASSWORD,
		},
		{
			description: "unconfigured username",
			username:    "nonexistentuser",
			password:    "somepassword",
			authnType:   acctzpb.AuthnDetail_AUTHN_TYPE_PASSWORD,
		},
		{
			description: "valid username with empty password",
			username:    acctz.SuccessUsername,
			password:    "",
			authnType:   acctzpb.AuthnDetail_AUTHN_TYPE_PASSWORD,
		},
		{
			description: "valid username with invalid password",
			username:    acctz.SuccessUsername,
			password:    "wrongpassword",
			authnType:   acctzpb.AuthnDetail_AUTHN_TYPE_PASSWORD,
		},
	}
}

// verifySessionAuthn verifies session status, authn details, user identity, and timestamp.
// Returns false if authn is nil (caller should skip further checks).
func verifySessionAuthn(t *testing.T, record *acctzpb.RecordResponse, attemptedUsernames map[string]bool, requestTimestamp *timestamppb.Timestamp) bool {
	t.Helper()
	sessionInfo := record.GetSessionInfo()

	if sessionInfo.GetStatus() != acctzpb.SessionInfo_SESSION_STATUS_LOGIN {
		t.Errorf("Record status is not LOGIN, got: %v, Record: %s", sessionInfo.GetStatus(), prettyPrint(record))
	}

	authn := sessionInfo.GetAuthn()
	if authn == nil {
		t.Errorf("Authn detail is nil for record: %s", prettyPrint(record))
		return false
	}
	validAuthnTypes := map[acctzpb.AuthnDetail_AuthnType]bool{
		acctzpb.AuthnDetail_AUTHN_TYPE_PASSWORD: true,
		acctzpb.AuthnDetail_AUTHN_TYPE_SSHKEY:   true,
		acctzpb.AuthnDetail_AUTHN_TYPE_SSHCERT:  true,
	}
	if !validAuthnTypes[authn.GetType()] {
		t.Errorf("authn.type is %v, want PASSWORD, SSHKEY, or SSHCERT for SSH auth failure: %s", authn.GetType(), prettyPrint(record))
	}
	if authn.GetStatus() != acctzpb.AuthnDetail_AUTHN_STATUS_FAIL {
		t.Errorf("Authn status is not FAIL, got: %v, Record: %s", authn.GetStatus(), prettyPrint(record))
	}
	if authn.GetCause() == "" {
		t.Errorf("Authn cause is not populated for authentication failure record: %s", prettyPrint(record))
	}

	if identity := sessionInfo.GetUser().GetIdentity(); !attemptedUsernames[identity] {
		t.Errorf("user.identity %q does not match any attempted username %v: %s", identity, attemptedUsernames, prettyPrint(record))
	}

	if !record.Timestamp.AsTime().After(requestTimestamp.AsTime()) {
		t.Errorf("Record timestamp %v is not after request timestamp %v, Record: %s", record.Timestamp.AsTime(), requestTimestamp.AsTime(), prettyPrint(record))
	}
	return true
}

// verifyNetworkFields verifies ip_proto, local/remote address and port, and channel_id.
func verifyNetworkFields(t *testing.T, record *acctzpb.RecordResponse, expectedRemoteAddr string, expectedRemotePort uint32) {
	t.Helper()
	sessionInfo := record.GetSessionInfo()

	if ipProto := sessionInfo.GetIpProto(); ipProto != 6 {
		t.Errorf("ip_proto is %d, want 6 (TCP) for SSH: %s", ipProto, prettyPrint(record))
	}

	if la := sessionInfo.GetLocalAddress(); la != "" {
		if la != expectedRemoteAddr {
			t.Errorf("local_address is %q (DUT), expected SSH target %q", la, expectedRemoteAddr)
		}
	} else {
		t.Errorf("local_address not populated by DUT")
	}
	if lp := sessionInfo.GetLocalPort(); lp != 0 {
		if lp != expectedRemotePort {
			t.Errorf("local_port is %d (DUT), expected SSH port %d", lp, expectedRemotePort)
		}
	} else {
		t.Errorf("local_port not populated by DUT")
	}

	if ra := sessionInfo.GetRemoteAddress(); ra == "" {
		t.Errorf("remote_address not populated by DUT")
	} else {
		t.Logf("remote_address=%q (client IP)", ra)
	}
	if rp := sessionInfo.GetRemotePort(); rp == 0 {
		t.Errorf("remote_port not populated by DUT")
	} else {
		t.Logf("remote_port=%d (client port)", rp)
	}

	if channelID := sessionInfo.GetChannelId(); channelID != "" && channelID != "0" {
		t.Errorf("channel_id is %q, want 0 or empty for SSH: %s", channelID, prettyPrint(record))
	}
}

// verifyServiceRequest verifies the service_request field of the record.
func verifyServiceRequest(t *testing.T, record *acctzpb.RecordResponse) {
	t.Helper()
	switch sr := record.GetServiceRequest().(type) {
	case *acctzpb.RecordResponse_CmdService:
		if sr.CmdService.GetServiceType() == acctzpb.CommandService_CMD_SERVICE_TYPE_UNSPECIFIED {
			t.Errorf("CommandService service_type is unspecified for record: %s", prettyPrint(record))
		}
		if st := sr.CmdService.GetServiceType(); st != acctzpb.CommandService_CMD_SERVICE_TYPE_CLI {
			t.Errorf("CmdService service_type is %v, want CMD_SERVICE_TYPE_CLI for SSH auth failure: %s", st, prettyPrint(record))
		}
		if sr.CmdService.GetCmd() != "" {
			t.Errorf("CmdService cmd should be omitted for auth failure record, got %q: %s", sr.CmdService.GetCmd(), prettyPrint(record))
		}
		if len(sr.CmdService.GetCmdArgs()) != 0 {
			t.Errorf("CmdService cmd_args should be omitted for auth failure record: %s", prettyPrint(record))
		}
		if authz := sr.CmdService.GetAuthz(); authz != nil && authz.GetStatus() != acctzpb.AuthzDetail_AUTHZ_STATUS_UNSPECIFIED {
			t.Errorf("CmdService authz should be omitted for auth failure record (status=%v): %s", authz.GetStatus(), prettyPrint(record))
		}
	case *acctzpb.RecordResponse_GrpcService:
		if sr.GrpcService.GetServiceType() == acctzpb.GrpcService_GRPC_SERVICE_TYPE_UNSPECIFIED {
			t.Errorf("GrpcService service_type is unspecified for record: %s", prettyPrint(record))
		}
		t.Errorf("Got GrpcService record for SSH auth failure (unexpected): %s", prettyPrint(record))
		if authz := sr.GrpcService.GetAuthz(); authz != nil && authz.GetStatus() != acctzpb.AuthzDetail_AUTHZ_STATUS_UNSPECIFIED {
			t.Errorf("GrpcService authz should be omitted for auth failure record (status=%v): %s", authz.GetStatus(), prettyPrint(record))
		}
	default:
		t.Logf("Record has no service_request set (platform-dependent for auth failures)")
	}
}

// parseHostPort splits a target "host:port" and returns the host and numeric port.
func parseHostPort(t *testing.T, target string) (string, uint32) {
	t.Helper()
	host, portStr, err := net.SplitHostPort(target)
	if err != nil {
		t.Fatalf("Failed to parse target %q: %v", target, err)
	}
	port, err := strconv.ParseUint(portStr, 10, 32)
	if err != nil {
		t.Fatalf("Failed to parse port %q: %v", portStr, err)
	}
	return host, uint32(port)
}

// attemptFailedSSH attempts an SSH connection with the given credentials and expects failure.
// It captures local address metadata from the TCP connection for later record correlation.
func attemptFailedSSH(t *testing.T, target, username, password string) *connMetadata {
	t.Helper()
	t.Logf("Attempting failed SSH authentication with user %q to %s", username, target)

	remoteAddr, remotePort := parseHostPort(t, target)
	meta := &connMetadata{
		description: "password auth for " + username,
		username:    username,
		remoteAddr:  remoteAddr,
		remotePort:  remotePort,
	}

	// Use a raw TCP dial first to capture the local address/port.
	tcpConn, err := net.DialTimeout("tcp", target, 30*time.Second)
	if err != nil {
		t.Logf("TCP connect failed for user %q (cannot capture local addr): %v", username, err)
		return meta
	}
	if localAddr, ok := tcpConn.LocalAddr().(*net.TCPAddr); ok {
		meta.localAddr = localAddr.IP.String()
		meta.localPort = uint32(localAddr.Port)
	}
	tcpConn.Close()

	config := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
			ssh.KeyboardInteractive(
				func(user, instruction string, questions []string, echos []bool) ([]string, error) {
					answers := make([]string, len(questions))
					for i := range answers {
						answers[i] = password
					}
					return answers, nil
				},
			),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	}

	conn, err := ssh.Dial("tcp", target, config)
	if err != nil {
		t.Logf("Got expected SSH authentication failure for user %q: %v", username, err)
		return meta
	}
	conn.Close()
	t.Errorf("Expected SSH authentication failure for user %q but connection succeeded", username)
	return meta
}

// attemptFailedSSHWithKey attempts an SSH connection using an unregistered SSH key and expects
// failure. This covers the "wrong SSH key/certificate" invalid-credential case from the test plan.
func attemptFailedSSHWithKey(t *testing.T, target, username string) *connMetadata {
	t.Helper()
	t.Logf("Attempting failed SSH key authentication with user %q to %s", username, target)

	remoteAddr, remotePort := parseHostPort(t, target)
	meta := &connMetadata{
		description: "key auth for " + username,
		username:    username,
		remoteAddr:  remoteAddr,
		remotePort:  remotePort,
	}

	// Use a raw TCP dial first to capture the local address/port.
	tcpConn, err := net.DialTimeout("tcp", target, 30*time.Second)
	if err != nil {
		t.Logf("TCP connect failed for user %q (cannot capture local addr): %v", username, err)
		return meta
	}
	if localAddr, ok := tcpConn.LocalAddr().(*net.TCPAddr); ok {
		meta.localAddr = localAddr.IP.String()
		meta.localPort = uint32(localAddr.Port)
	}
	tcpConn.Close()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed generating RSA key: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(privateKey)
	if err != nil {
		t.Fatalf("Failed creating SSH signer from key: %v", err)
	}

	config := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // lgtm[go/insecure-hostkeycallback]
		Timeout:         30 * time.Second,
	}

	conn, err := ssh.Dial("tcp", target, config)
	if err != nil {
		t.Logf("Got expected SSH key authentication failure for user %q: %v", username, err)
		return meta
	}
	conn.Close()
	t.Errorf("Expected SSH key authentication failure for user %q but connection succeeded", username)
	return meta
}

func TestAccountzAuthenFailMulti(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	acctz.SetupUsers(t, dut, true)

	// Get the current time from the router via gNMI to avoid clock skew issues.
	startTime := helpers.GetRouterTime(t, dut)

	// Get SSH target for the DUT.
	target := acctz.GetSSHTarget(t, dut, *staticBinding)

	// Attempt SSH connections with various invalid credentials, recording attempted usernames
	// and connection metadata for later correlation with accounting records.
	credSets := InvalidCredentialSets()
	attemptedUsernames := make(map[string]bool)
	var connMetas []*connMetadata
	for i := range credSets {
		creds := &credSets[i]
		meta := attemptFailedSSH(t, target, creds.username, creds.password)
		connMetas = append(connMetas, meta)
		attemptedUsernames[creds.username] = true
		time.Sleep(500 * time.Millisecond)
	}
	// Also attempt with an unregistered SSH key (wrong SSH key/certificate).
	meta := attemptFailedSSHWithKey(t, target, acctz.SuccessUsername)
	connMetas = append(connMetas, meta)
	attemptedUsernames[acctz.SuccessUsername] = true

	// Parse the remote address/port from the SSH target for record validation.
	expectedRemoteAddr, expectedRemotePort := parseHostPort(t, target)

	// Log all captured connection metadata for debugging.
	for _, cm := range connMetas {
		t.Logf("Connection attempt: %s user=%q local=%s:%d remote=%s:%d",
			cm.description, cm.username, cm.localAddr, cm.localPort, cm.remoteAddr, cm.remotePort)
	}

	// Quick sleep to ensure all the records have been processed/ready for us.
	time.Sleep(5 * time.Second)

	// Get gNSI record subscribe client.
	requestTimestamp := &timestamppb.Timestamp{
		Seconds: startTime.Unix(),
		Nanos:   0,
	}
	acctzClient := dut.RawAPIs().GNSI(t).AcctzStream()
	acctzSubClient, err := acctzClient.RecordSubscribe(context.Background(), &acctzpb.RecordRequest{Timestamp: requestTimestamp})
	if err != nil {
		t.Fatalf("Failed sending accountz record request, error: %s", err)
	}
	defer acctzSubClient.CloseSend()

	// Collect records from the stream.
	var gotRecords []*acctzpb.RecordResponse
	var totalRecords, skippedNotLogin, skippedNotFail, skippedOldTimestamp int
	for {
		// Read single acctz record from stream into channel.
		r := make(chan recordRequestResult, 1)
		go func() {
			response, err := acctzSubClient.Recv()
			r <- recordRequestResult{
				record: response,
				err:    err,
			}
		}()

		var done bool
		var resp recordRequestResult

		select {
		case rr := <-r:
			resp = rr
		case <-time.After(30 * time.Second):
			done = true
		}
		if done {
			t.Log("Done receiving records (timeout)...")
			break
		}

		if resp.err != nil {
			t.Fatalf("Failed receiving record response, error: %s", resp.err)
		}

		totalRecords++

		if !resp.record.Timestamp.AsTime().After(startTime) {
			skippedOldTimestamp++
			t.Logf("Skipping record: timestamp %v not after start time %v", resp.record.Timestamp.AsTime(), startTime)
			continue
		}

		// We are looking for LOGIN records that indicate authentication failure.
		sessionStatus := resp.record.GetSessionInfo().GetStatus()
		if sessionStatus != acctzpb.SessionInfo_SESSION_STATUS_LOGIN {
			skippedNotLogin++
			t.Logf("Skipping record: not LOGIN status (got %v, user=%q)", sessionStatus,
				resp.record.GetSessionInfo().GetUser().GetIdentity())
			continue
		}

		authnStatus := resp.record.GetSessionInfo().GetAuthn().GetStatus()
		if authnStatus != acctzpb.AuthnDetail_AUTHN_STATUS_FAIL {
			skippedNotFail++
			t.Logf("Skipping record: authn status is not FAIL (got %v, user=%q)", authnStatus,
				resp.record.GetSessionInfo().GetUser().GetIdentity())
			continue
		}

		t.Logf("Found authentication failure record: %s", prettyPrint(resp.record))
		gotRecords = append(gotRecords, resp.record)
	}

	t.Logf("=== Record collection summary: total=%d, matched=%d, skippedNotLogin=%d, skippedNotFail=%d, skippedOldTimestamp=%d ===",
		totalRecords, len(gotRecords), skippedNotLogin, skippedNotFail, skippedOldTimestamp)

	if len(gotRecords) == 0 {
		t.Fatal("No authentication failure records found")
	}

	t.Logf("Found %d authentication failure records (expected %d scenarios)", len(gotRecords), len(connMetas))
	for i, r := range gotRecords {
		t.Logf("  Record %d: user=%q, authn_type=%v, authn_status=%v, cause=%q",
			i, r.GetSessionInfo().GetUser().GetIdentity(),
			r.GetSessionInfo().GetAuthn().GetType(),
			r.GetSessionInfo().GetAuthn().GetStatus(),
			r.GetSessionInfo().GetAuthn().GetCause())
	}

	// Verify each authentication failure record.
	for _, record := range gotRecords {
		if !verifySessionAuthn(t, record, attemptedUsernames, requestTimestamp) {
			continue
		}
		verifyNetworkFields(t, record, expectedRemoteAddr, expectedRemotePort)

		sessionInfo := record.GetSessionInfo()
		t.Logf("tty=%q (platform-dependent, not verified)", sessionInfo.GetTty())
		if taskIDs := record.GetTaskIds(); len(taskIDs) > 0 {
			t.Logf("task_ids=%v (platform-specific)", taskIDs)
		}

		verifyServiceRequest(t, record)
		t.Logf("Processed authentication failure record: %s", prettyPrint(record))
	}
}
