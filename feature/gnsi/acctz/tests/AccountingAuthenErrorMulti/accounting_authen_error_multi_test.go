package accounting_authen_error_multi_test

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/fptest"
	acctzlib "github.com/openconfig/featureprofiles/internal/security/acctz"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	acctzpb "github.com/openconfig/gnsi/acctz"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	portGRPC            = 9339
	ipProtoTCP          = 6
	successPassword     = "verysecurepasswordTest123!"
	tacacsUsername      = "tacacsuser"
	tacacsPassword      = "tacacspass"
	collectDeadline     = 1 * time.Minute
	dialTimeout         = 15 * time.Second
	t0Offset            = 2 * time.Minute
	metadataKeyUsername = "username"
	metadataKeyPassword = "password"
	tacacsServer        = "192.0.2.100"
	tacacsKey           = "testkey"
	authAttempts        = 3
	ocUserRole          = "network-admin"
)

var tacacsServerRE = regexp.MustCompile(
	fmt.Sprintf(`(?m)%s`, regexp.QuoteMeta(tacacsServer)),
)

// rpcCredentials provides username and password metadata for gRPC RPC authentication requests.
type rpcCredentials struct {
	username string
	password string
}

// connRecord stores connection metadata captured during an authentication attempt.
// The recorded values are later correlated with acctz accounting records returned by the DUT.
type connRecord struct {
	localAddr  string
	localPort  uint32
	remoteAddr string
	remotePort uint32
	username   string
	attempt    int
}

// recordReceiver defines the interface implemented by acctz RecordSubscribe stream clients.
type recordReceiver interface {
	Recv() (*acctzpb.RecordResponse, error)
}

// credentialer defines an interface for retrieving DUT RPC authentication credentials.
type credentialer interface {
	RPCUsername() string
	RPCPassword() string
}

// deviceRecordsConfig holds the parameters required for a deviceRecords call.
type deviceRecordsConfig struct {
	t        *testing.T
	client   recordReceiver
	deadline time.Duration
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestAccountingAuthenErrorMultiTransaction(t *testing.T) {
	t.Helper()
	dut := ondatra.DUT(t, "dut")
	batch := new(gnmi.SetBatch)
	setupTestUser(t, batch)
	// Configure TACACS normally first.
	configureTacacsAAAWithLocalFallback(t, batch)
	batch.Set(t, dut)
	verifyTacacsConfigured(t, dut)

	// Record T0 BEFORE failures.
	t0 := time.Now().Add(-t0Offset)

	t.Logf("T0 = %v", t0)

	var attempts []connRecord

	// Generate authentication attempts while TACACS backend is unavailable.
	for i := 0; i < authAttempts; i++ {
		rec, ok := dialAndFail(t, fmt.Sprintf("%s:%d", dut.Name(), portGRPC), tacacsUsername, tacacsPassword, i+1)
		if !ok {
			t.Fatalf("dial attempt %d failed", i+1)
		}
		attempts = append(attempts, rec)
	}

	// Valid gNSI connection for RecordSubscribe
	username, password := dutRPCCredentials(dut)

	conn, err := grpc.NewClient(fmt.Sprintf("%s:%d", dut.Name(), portGRPC),
		grpc.WithTransportCredentials(
			credentials.NewTLS(
				&tls.Config{
					InsecureSkipVerify: true,
				},
			),
		),
		grpc.WithPerRPCCredentials(
			&rpcCredentials{
				username: username,
				password: password,
			},
		),
	)

	if err != nil {
		t.Fatalf("grpc.NewClient() failed: %v", err)
	}

	defer conn.Close()

	stream, err := acctzpb.NewAcctzStreamClient(conn).RecordSubscribe(t.Context(), &acctzpb.RecordRequest{Timestamp: timestamppb.New(t0)})

	if err != nil {
		t.Fatalf("RecordSubscribe() failed: %v", err)
	}
	records, err := deviceRecords(deviceRecordsConfig{t: t, client: stream, deadline: collectDeadline})
	if err != nil {
		t.Fatalf("collectRecords() failed: %v", err)
	}
	t.Logf("Collected %d records", len(records))

	var matched int
	for _, attempt := range attempts {
		found := false
		for _, rec := range records {
			if matchRecord(rec, attempt) {
				found = true
				matched++
				verifyAuthenErrorRecord(t, rec, attempt, t0)
				break
			}
		}
		if !found {
			t.Errorf("no AUTHN_STATUS_ERROR record found for attempt=%d local=%s:%d remote=%s:%d", attempt.attempt, attempt.localAddr, attempt.localPort, attempt.remoteAddr, attempt.remotePort)
		}
	}
	if matched < authAttempts {
		t.Errorf("matched only %d/%d authentication ERROR records", matched, authAttempts)
	}
}

// configureTacacsAAAWithLocalFallback queues TACACS+ AAA configuration.
// When isTacacs is true, the function adds TACACS+ server-group configuration with local authentication fallback.
// When isTacacs is false, the function removes the TACACS+ server-group configuration from the batch to simulate TACACS backend unavailability.
func configureTacacsAAAWithLocalFallback(t *testing.T, batch *gnmi.SetBatch) {
	t.Helper()
	// ============================================================
	// CONFIGURE TACACS
	// ============================================================
	root := &oc.Root{}

	sys := root.GetOrCreateSystem()
	aaa := sys.GetOrCreateAaa()

	// ------------------------------------------------------------
	// TACACS server-group
	// ------------------------------------------------------------
	sg := aaa.GetOrCreateServerGroup("TACACS")
	sg.Type = oc.AaaTypes_AAA_SERVER_TYPE_TACACS
	srv := sg.GetOrCreateServer(tacacsServer)

	srv.Address = ygot.String(tacacsServer)
	srv.Timeout = ygot.Uint16(2)

	tac := srv.GetOrCreateTacacs()
	tac.SecretKey = ygot.String(tacacsKey)
	tac.Port = ygot.Uint16(49)

	// ------------------------------------------------------------
	// Authentication
	// CLI:
	// aaa authentication login default group tacacs+ local
	// ------------------------------------------------------------
	auth := aaa.GetOrCreateAuthentication()
	loginAdmin := auth.GetOrCreateAdminUser()
	loginAdmin.SetAdminPassword(successPassword)
	auth.SetAuthenticationMethod(
		[]oc.System_Aaa_Authentication_AuthenticationMethod_Union{
			oc.AaaTypes_AAA_METHOD_TYPE_TACACS_ALL,
			oc.AaaTypes_AAA_METHOD_TYPE_LOCAL,
		},
	)
	// ------------------------------------------------------------
	// Push config
	// ------------------------------------------------------------
	gnmi.BatchUpdate(batch, gnmi.OC().System().Aaa().ServerGroup("TACACS").Config(), sg)
	t.Log("Configured TACACS AAA with local fallback")
}

// verifyTacacsConfigured verifies TACACS+ AAA configuration state on the DUT.
//
// When expectConfigured is true:
//   - AAA configuration must exist.
//   - The TACACS+ server must be present in "show tacacs".
//
// When expectConfigured is false:
//   - The TACACS+ server must not appear in "show tacacs".
func verifyTacacsConfigured(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	system := gnmi.Get(t, dut, gnmi.OC().System().State())
	if system.GetAaa() == nil {
		t.Fatalf("AAA subsystem not configured")
	}
	resp, err := dut.RawAPIs().CLI(t).RunCommand(context.Background(), "show tacacs")
	if err != nil {
		t.Fatalf("show tacacs failed: %v", err)
	}

	output := resp.Output()
	expectConfigured := tacacsServerRE.MatchString(output)
	if expectConfigured {
		t.Logf("Verified TACACS+ server %q is configured", tacacsServer)
	} else {
		t.Fatalf("TACACS+ server %q not found in DUT configuration", tacacsServer)
	}
}

// setupTestUser queues configuration for a local AAA test user.
// The configured user is used for authentication testing with the DUT.
// A cleanup handler is registered to remove the user configuration after test completion.
func setupTestUser(t *testing.T, batch *gnmi.SetBatch) {
	t.Helper()
	userPath := gnmi.OC().System().Aaa().Authentication().User(acctzlib.SuccessUsername)
	user := &oc.System_Aaa_Authentication_User{
		Username: ygot.String(acctzlib.SuccessUsername),
		Role:     oc.UnionString(ocUserRole),
		Password: ygot.String(successPassword),
	}
	gnmi.BatchReplace(batch, userPath.Config(), user)

	t.Logf("Configured test user %q", acctzlib.SuccessUsername)

	t.Cleanup(func() {
		gnmi.BatchDelete(batch, userPath.Config())
	})
}

// dialAndFail establishes a TCP and gRPC connection to the target DUT and
// performs a gNMI Capabilities RPC using the provided credentials.
//
// The function captures local and remote connection details for later
// acctz record correlation and validation.
//
// Authentication failure during the RPC is expected in negative AAA test
// scenarios (for example invalid credentials or TACACS backend failure)
// and does not cause the function to fail.
//
// It returns the collected connection metadata and a boolean indicating whether the connection setup succeeded.
func dialAndFail(t *testing.T, target, username, password string, attempt int) (connRecord, bool) {
	t.Helper()
	tcpConn, err := net.DialTimeout("tcp", target, dialTimeout)

	if err != nil {
		t.Fatalf("TCP dial failed: %v", err)
	}
	defer tcpConn.Close()

	localAddr, localPort := mustHostPortInfo(t, tcpConn.LocalAddr().String())
	remoteAddr, remotePort := mustHostPortInfo(t, tcpConn.RemoteAddr().String())

	used := false
	reuseOrDial := func(ctx context.Context, addr string) (net.Conn, error) {

		if !used {
			used = true
			return tcpConn, nil
		}
		var d net.Dialer
		return d.DialContext(ctx, "tcp", addr)
	}

	conn, err := grpc.NewClient(target,
		grpc.WithTransportCredentials(
			credentials.NewTLS(
				&tls.Config{
					InsecureSkipVerify: true, //nolint:gosec
				},
			),
		),
		grpc.WithContextDialer(reuseOrDial),
	)

	if err != nil {
		t.Logf("grpc.NewClient failed: %v", err)
		return connRecord{}, false
	}

	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), dialTimeout)

	defer cancel()

	// valid credentials
	// TACACS backend unreachable AUTHN_STATUS_ERROR expected
	ctx = metadata.AppendToOutgoingContext(ctx, metadataKeyUsername, username, metadataKeyPassword, password)

	_, err = gnmipb.NewGNMIClient(conn).Capabilities(ctx, &gnmipb.CapabilityRequest{})

	if err != nil {
		t.Logf("Authentication backend failure observed as expected: %v", err)
	}

	return connRecord{
		localAddr:  localAddr,
		localPort:  localPort,
		remoteAddr: remoteAddr,
		remotePort: remotePort,
		username:   username,
		attempt:    attempt,
	}, true
}

// deviceRecords collects acctz RecordSubscribe responses from the provided
// RecordReceiver until either the configured deadline expires or the stream
// returns an error.
//
// The function asynchronously receives records from the stream and returns
// all successfully collected RecordResponse entries.
//
// A timeout expiration is treated as a successful completion and returns the records collected up to that point.
func deviceRecords(cfg deviceRecordsConfig) ([]*acctzpb.RecordResponse, error) {
	cfg.t.Helper()
	type result struct {
		record *acctzpb.RecordResponse
		err    error
	}
	var records []*acctzpb.RecordResponse
	ctx, cancel := context.WithTimeout(context.Background(), cfg.deadline)

	defer cancel()

	rChan := make(chan result, 1)

	go func() {
		defer close(rChan)

		for {
			resp, err := cfg.client.Recv()
			select {
			case rChan <- result{
				record: resp,
				err:    err,
			}:

			case <-ctx.Done():
				return
			}

			if err != nil {
				return
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return records, nil

		case r, ok := <-rChan:
			if !ok {
				return records, nil
			}

			if r.err != nil {
				if errors.Is(r.err, io.EOF) {
					return records, nil
				}
				cfg.t.Fatalf("RecordSubscribe Recv() failed: %v", r.err)
			}

			records = append(records, r.record)
		}
	}
}

// matchRecord validates whether the provided acctz RecordResponse matches
// the expected connection metadata captured during a test authentication
// attempt.
//
// The function verifies:
//   - session status is LOGIN
//   - authentication status is ERROR
//   - local and remote address/port values match
//   - authenticated username matches the expected identity
//
// It returns true when the record corresponds to the expected connection attempt, otherwise false.
func matchRecord(resp *acctzpb.RecordResponse, conn connRecord) bool {
	si := resp.GetSessionInfo()
	if si == nil {
		return false
	}
	if si.GetStatus() != acctzpb.SessionInfo_SESSION_STATUS_LOGIN {
		return false
	}
	authn := si.GetAuthn()
	if authn == nil {
		return false
	}
	if authn.GetStatus() != acctzpb.AuthnDetail_AUTHN_STATUS_ERROR {
		return false
	}
	if si.GetLocalAddress() != conn.remoteAddr {
		return false
	}
	if si.GetLocalPort() != conn.remotePort {
		return false
	}
	if ra := si.GetRemoteAddress(); ra != "" &&
		ra != "0.0.0.0" &&
		ra != "::" {

		if ra != conn.localAddr {
			return false
		}
	}
	if rp := si.GetRemotePort(); rp != 0 {
		if rp != conn.localPort {
			return false
		}
	}
	user := si.GetUser()
	if user == nil {
		return false
	}
	if got := user.GetIdentity(); got != conn.username {
		return false
	}

	return true
}

// verifyAuthenErrorRecord validates that the provided acctz authentication
// error record matches the expected connection metadata and authentication
// failure behavior for the test case.
//
// The function verifies:
//   - record timestamp occurs after T0
//   - session metadata matches the captured connection details
//   - session status is LOGIN
//   - authentication status is ERROR
//   - authentication cause indicates backend or credential failure
//   - user identity matches the expected username
//   - sensitive fields are omitted when required
//
// The function also validates optional service request information for
// gRPC or command services when populated by the device.
func verifyAuthenErrorRecord(t *testing.T, resp *acctzpb.RecordResponse, conn connRecord, t0 time.Time) {
	t.Helper()
	ts := resp.GetTimestamp()

	if ts == nil {
		t.Fatalf("record timestamp is nil")
	}

	if !ts.AsTime().After(t0) {
		t.Errorf("timestamp %v is not after T0 %v", ts.AsTime(), t0)
	}

	si := resp.GetSessionInfo()

	if si == nil {
		t.Fatalf("session_info is nil")
	}

	if got := si.GetIpProto(); got != ipProtoTCP {
		t.Errorf("ip_proto got %d want %d", got, ipProtoTCP)
	}

	if got := si.GetLocalAddress(); got != conn.remoteAddr {
		t.Errorf("local_address got %q want %q", got, conn.remoteAddr)
	}

	if got := si.GetLocalPort(); got != conn.remotePort {
		t.Errorf("local_port got %d want %d", got, conn.remotePort)
	}

	if got := si.GetChannelId(); got != "" && got != "0" {
		t.Errorf("channel_id got %q want empty/0", got)
	}

	if got := si.GetStatus(); got != acctzpb.SessionInfo_SESSION_STATUS_LOGIN {
		t.Errorf("session_status got %v want LOGIN", got)
	}
	if tty := si.GetTty(); tty != "" {
		t.Logf("TTY populated: %s", tty)
	}
	authn := si.GetAuthn()

	if authn == nil {
		t.Fatalf("authn is nil")
	}

	if got := authn.GetType(); got == acctzpb.AuthnDetail_AUTHN_TYPE_UNSPECIFIED {
		t.Errorf("authn.type is UNSPECIFIED")
	}

	if got := authn.GetStatus(); got != acctzpb.AuthnDetail_AUTHN_STATUS_ERROR {
		t.Errorf("authn.status got %v want ERROR", got)
	}

	cause := strings.ToLower(authn.GetCause())

	if cause == "" {
		t.Errorf("authn.cause empty")
	}

	valid := strings.Contains(cause, "tacacs") ||
		strings.Contains(cause, "timeout") ||
		strings.Contains(cause, "unreachable") ||
		strings.Contains(cause, "dead") ||
		strings.Contains(cause, "backend") ||
		strings.Contains(cause, "authentication failed") ||
		strings.Contains(cause, "authentication failure") ||
		strings.Contains(cause, "authentication service failure")

	if !valid {
		t.Errorf("unexpected authn.cause: %q", authn.GetCause())
	}

	user := si.GetUser()

	if user == nil {
		t.Fatalf("user nil")
	}

	if got := user.GetIdentity(); got != conn.username {
		t.Errorf("user.identity got=%q want=%q", got, conn.username)
	}

	if got := user.GetRole(); got != "" {
		t.Errorf("user.role should be omitted got=%q", got)
	}
	switch svc := resp.GetServiceRequest().(type) {

	case *acctzpb.RecordResponse_GrpcService:
		grpcSvc := svc.GrpcService

		if grpcSvc == nil {
			t.Errorf("grpc_service unexpectedly nil")
			return
		}

		if grpcSvc.GetServiceType() == acctzpb.GrpcService_GRPC_SERVICE_TYPE_UNSPECIFIED {
			t.Errorf("grpc_service.service_type unspecified")
		}

		// all other fields should be omitted
		if got := grpcSvc.GetRpcName(); got != "" {
			t.Errorf("grpc_service.rpc_name should be omitted got=%q", got)
		}

		t.Logf("Validated grpc_service type=%v", grpcSvc.GetServiceType())

	case *acctzpb.RecordResponse_CmdService:
		cmdSvc := svc.CmdService

		if cmdSvc == nil {
			t.Errorf("cmd_service unexpectedly nil")
			return
		}

		if cmdSvc.GetServiceType() == acctzpb.CommandService_CMD_SERVICE_TYPE_UNSPECIFIED {
			t.Errorf("cmd_service.service_type unspecified")
		}

		if got := cmdSvc.GetCmd(); got != "" {
			t.Errorf("cmd_service.cmd should be omitted got=%q", got)
		}

		if len(cmdSvc.GetCmdArgs()) != 0 {
			t.Errorf("cmd_service.cmd_args should be omitted got=%v", cmdSvc.GetCmdArgs())
		}

		t.Logf("Validated cmd_service type=%v", cmdSvc.GetServiceType())

	default:
		// Vendor EOS may omit service_request entirely when authentication fails before RPC/service association.
		t.Log("service_request not populated for early authentication failure")
	}
}

// GetRequestMetadata returns gRPC request metadata containing the configured username and password credentials.
func (r *rpcCredentials) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{
		metadataKeyUsername: r.username,
		metadataKeyPassword: r.password,
	}, nil
}

// RequireTransportSecurity indicates that the RPC credentials must only be transmitted over a secure transport connection.
func (r *rpcCredentials) RequireTransportSecurity() bool {
	return true
}

// dutRPCCredentials returns the RPC username and password associated with the DUT binding implementation.
// If the DUT binding does not implement the credentialer interface, empty credential values are returned.
func dutRPCCredentials(dut *ondatra.DUTDevice) (string, string) {
	if c, ok := dut.RawAPIs().
		BindingDUT().(credentialer); ok {

		return c.RPCUsername(),
			c.RPCPassword()
	}

	return "", ""
}

// mustHostPortInfo parses an address string into host and port values.
// The function fails the test immediately if the address cannot be parsed or if the port value is invalid.
func mustHostPortInfo(t *testing.T, address string) (string, uint32) {
	t.Helper()
	host, portRaw, err := net.SplitHostPort(address)

	if err != nil {
		t.Fatalf("SplitHostPort(%q): %v", address, err)
	}
	port, err := strconv.ParseUint(portRaw, 10, 32)

	if err != nil {
		t.Fatalf("ParseUint(%q): %v", portRaw, err)
	}
	return host, uint32(port)
}
