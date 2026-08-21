// Copyright 2026 Google LLC
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

// Package accountingauthenfailunitest implements ACCTZ-8.1:
// gNSI.acctz.v1 (Accounting) Test Accounting Authentication Failure - Uni-transaction.
package accountingauthenfailuni_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/fptest"
	acctzlib "github.com/openconfig/featureprofiles/internal/security/acctz"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	gnoipb "github.com/openconfig/gnoi/system"
	acctzpb "github.com/openconfig/gnsi/acctz"
	gribi "github.com/openconfig/gribi/v1/proto/service"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/binding/introspect"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
	p4pb "github.com/p4lang/p4runtime/go/p4/v1"
	"golang.org/x/crypto/ssh"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	ipProtoTCP          = 6
	successPassword     = "verysecurepasswordTest123!"
	wrongPassword       = "definitelywrongpassword999"
	unconfiguredUser    = "nonexistent_user_xyz"
	ocUserRole          = "network-admin"
	wrongCertCN         = "acctz-test-wrong-client"
	untrustedCACN       = "acctz-test-untrusted-ca"
	collectDeadline     = time.Minute
	dialTimeout         = 15 * time.Second
	t0Offset            = 2 * time.Minute
	caSerialNumber      = 1
	clientSerialNumber  = 2
	certValidity        = time.Hour
	certNotBeforeOffset = time.Minute
	pemTypeCertificate  = "CERTIFICATE"
	pemTypeECPrivateKey = "EC PRIVATE KEY"
	pingDestination     = "127.0.0.1"
	pingCount           = 1
	metadataKeyUsername = "username"
	metadataKeyPassword = "password"
	sshPort             = "22"
	cliServiceName      = "cli"
)

var (
	serviceTable  []*serviceEntry
	scenarioTable = []*scenarioEntry{
		{name: "empty-user-correct-pass", user: "", pass: successPassword, certFailure: false},
		{name: "unknown-user-correct-pass", user: unconfiguredUser, pass: successPassword, certFailure: false},
		{name: "success-user-empty-pass", user: acctzlib.SuccessUsername, pass: "", certFailure: false},
		{name: "success-user-wrong-pass", user: acctzlib.SuccessUsername, pass: wrongPassword, certFailure: false},
		{name: "success-user-wrong-cert", user: acctzlib.SuccessUsername, pass: successPassword, certFailure: true},
	}
	cliScenarioTable = []*scenarioEntry{
		{name: "empty-user-correct-pass", user: "", pass: successPassword, certFailure: false},
		{name: "unknown-user-correct-pass", user: unconfiguredUser, pass: successPassword, certFailure: false},
		{name: "success-user-empty-pass", user: acctzlib.SuccessUsername, pass: "", certFailure: false},
		{name: "success-user-wrong-pass", user: acctzlib.SuccessUsername, pass: wrongPassword, certFailure: false},
		{name: "success-user-wrong-sshkey", user: acctzlib.SuccessUsername, certFailure: true},
	}
)

// serviceEntry describes one gRPC service under test.
type serviceEntry struct {
	name    string
	target  string // host:port resolved from the binding file at runtime
	svcType acctzpb.GrpcService_GrpcServiceType
	rpcFn   func(*testing.T, *rpcConfig) error
}

// scenarioEntry describes one credential-failure scenario.
type scenarioEntry struct {
	name        string
	user        string
	pass        string
	certFailure bool
}

// rpcConfig holds the parameters for a single RPC helper invocation.
type rpcConfig struct {
	conn   *grpc.ClientConn
	ctx    context.Context
	failOK bool
}

// connRecord captures the network metadata of one completed dial attempt.
type connRecord struct {
	serviceType acctzpb.GrpcService_GrpcServiceType
	cmdSvcType  acctzpb.CommandService_CmdServiceType
	isCLI       bool
	localAddr   string
	localPort   uint32
	remoteAddr  string
	remotePort  uint32
	username    string
	testName    string
	certFailure bool
}

// dialConfig is the unified config for dialAndFail/dialAndFailSSH. When certFailure is
// true, wrongCert (gRPC) or wrongSSHKey (SSH) is used instead of password credentials.
type dialConfig struct {
	target      string
	username    string
	password    string
	testName    string
	certFailure bool
	wrongCert   tls.Certificate
	wrongSSHKey ssh.Signer
	svcType     acctzpb.GrpcService_GrpcServiceType
	rpcFn       func(*testing.T, *rpcConfig) error
}

// RecordReceiver is satisfied by any gRPC streaming client that can Recv accounting records.
type RecordReceiver interface {
	Recv() (*acctzpb.RecordResponse, error)
}

// rpcCredentials provides per-RPC username/password credentials to gRPC.
type rpcCredentials struct {
	username string
	password string
}

// credentialer is satisfied by binding implementations that expose RPC credentials.
type credentialer interface {
	RPCUsername() string
	RPCPassword() string
}

// subtestConfig holds the parameters for a runVerifySubtest call.
type subtestConfig struct {
	res        *dialResult
	gotRecords []*acctzpb.RecordResponse
	usedRecord []bool
	t0         time.Time
}

// dialResult captures the outcome of one dialAndFail attempt.
type dialResult struct {
	rec  *connRecord
	skip bool
}

// matchRecordConfig holds the parameters for a matchRecord call.
type matchRecordConfig struct {
	resp *acctzpb.RecordResponse
	conn *connRecord
}

// verifyRecordConfig holds the parameters for a verifyAuthenFailRecord call.
type verifyRecordConfig struct {
	resp *acctzpb.RecordResponse
	conn *connRecord
	t0   time.Time
}

// deviceRecordsConfig holds the parameters for a deviceRecords call.
type deviceRecordsConfig struct {
	t        *testing.T
	client   RecordReceiver
	deadline time.Duration
}

// buildServiceTable builds the gRPC service table using service targets from the DUT binding.
func buildServiceTable(t *testing.T, dut *ondatra.DUTDevice) []*serviceEntry {
	t.Helper()
	gnmiTarget := introspect.DUTDialer(t, dut, introspect.GNMI).DialTarget
	gribiTarget := introspect.DUTDialer(t, dut, introspect.GRIBI).DialTarget
	p4rtTarget := introspect.DUTDialer(t, dut, introspect.P4RT).DialTarget
	return []*serviceEntry{
		{name: "gnmi", target: gnmiTarget, svcType: acctzpb.GrpcService_GRPC_SERVICE_TYPE_GNMI, rpcFn: rpcGNMI},
		{name: "gnoi", target: gnmiTarget, svcType: acctzpb.GrpcService_GRPC_SERVICE_TYPE_GNOI, rpcFn: rpcGNOI},
		{name: "gnsi", target: gnmiTarget, svcType: acctzpb.GrpcService_GRPC_SERVICE_TYPE_GNSI, rpcFn: rpcGNSI},
		{name: "gribi", target: gribiTarget, svcType: acctzpb.GrpcService_GRPC_SERVICE_TYPE_GRIBI, rpcFn: rpcGRIBI},
		{name: "p4rt", target: p4rtTarget, svcType: acctzpb.GrpcService_GRPC_SERVICE_TYPE_P4RT, rpcFn: rpcP4RT},
	}
}

// TestMain initializes and runs all tests using the featureprofiles test framework.
func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// testCase defines a service/scenario test case and its dial result.
type testCase struct {
	name string
	svc  *serviceEntry
	sc   *scenarioEntry
	res  *dialResult
}

// TestAccountingAuthenFailUni validates authentication-failure accounting across
// all per-transaction gRPC services discovered from the DUT binding.
func TestAccountingAuthenFailUni(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	acctzUsername := ""
	acctzPassword := ""
	acctzTarget := ""
	acctzConn := (*grpc.ClientConn)(nil)
	acctzSubClient := (grpc.ServerStreamingClient[acctzpb.RecordResponse])(nil)
	gotRecords := ([]*acctzpb.RecordResponse)(nil)
	usedRecord := ([]bool)(nil)
	err := error(nil)

	serviceTable = buildServiceTable(t, dut)

	setupTestUser(t, dut)

	wrongCert := mustGenerateWrongClientCert(t)
	wrongSSHKey := mustGenerateWrongSSHKey(t)
	mustVerifyServiceConnectivity(t, dut, serviceTable)

	t0 := time.Now().Add(-t0Offset)
	t.Logf("T0 = %v", t0)

	tests := make([]testCase, 0, len(serviceTable)*len(scenarioTable)+len(cliScenarioTable))
	for _, svc := range serviceTable {
		for _, sc := range scenarioTable {
			tests = append(tests, testCase{
				name: svc.name + "/" + sc.name,
				svc:  svc,
				sc:   sc,
			})
		}
	}

	cliHost, _ := mustHostPortInfo(t, serviceTable[0].target)
	cliTarget := net.JoinHostPort(cliHost, sshPort)
	cliIdx := len(tests)
	for _, sc := range cliScenarioTable {
		tests = append(tests, testCase{
			name: cliServiceName + "/" + sc.name,
			svc:  &serviceEntry{name: cliServiceName, target: cliTarget},
			sc:   sc,
		})
	}

	for i := range tests {
		if i >= cliIdx {
			rec, ok := dialAndFailSSH(t, &dialConfig{
				target:      tests[i].svc.target,
				username:    tests[i].sc.user,
				password:    tests[i].sc.pass,
				testName:    tests[i].name,
				certFailure: tests[i].sc.certFailure,
				wrongSSHKey: wrongSSHKey,
			})
			tests[i].res = &dialResult{rec: rec, skip: !ok}
			continue
		}
		rec, ok := dialAndFail(t, &dialConfig{
			target:      tests[i].svc.target,
			username:    tests[i].sc.user,
			password:    tests[i].sc.pass,
			testName:    tests[i].name,
			certFailure: tests[i].sc.certFailure,
			wrongCert:   wrongCert,
			svcType:     tests[i].svc.svcType,
			rpcFn:       tests[i].svc.rpcFn,
		})
		tests[i].res = &dialResult{rec: rec, skip: !ok}
	}
	t.Logf("completed %d per-transaction connection attempts (including %d SSH/CLI attempts)",
		len(tests), len(tests)-cliIdx)

	acctzUsername, acctzPassword = dutRPCCredentials(dut)
	acctzTarget = serviceTable[0].target
	acctzConn, err = grpc.NewClient(
		acctzTarget,
		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
			InsecureSkipVerify: true, //nolint:gosec
		})),
		grpc.WithPerRPCCredentials(&rpcCredentials{username: acctzUsername, password: acctzPassword}),
	)
	if err != nil {
		t.Fatalf("failed to create acctz gRPC connection to %s: %v", acctzTarget, err)
	}
	defer acctzConn.Close()

	acctzSubClient, err = acctzpb.NewAcctzStreamClient(acctzConn).RecordSubscribe(
		t.Context(),
		&acctzpb.RecordRequest{Timestamp: timestamppb.New(t0)},
	)
	if err != nil {
		t.Fatalf("recordSubscribe failed: %v", err)
	}
	gotRecords, err = deviceRecords(&deviceRecordsConfig{t: t, client: acctzSubClient, deadline: collectDeadline})
	if err != nil {
		t.Fatalf("failed receiving accounting records: %v", err)
	}
	t.Logf("received %d accounting records from DUT", len(gotRecords))
	for i, r := range gotRecords {
		si := r.GetSessionInfo()
		t.Logf("  record[%d]: local=%s:%d remote=%s:%d user=%q channelID=%q "+
			"sessionStatus=%v authnStatus=%v svcType=%v taskIDs=%v ts=%v",
			i,
			si.GetLocalAddress(), si.GetLocalPort(),
			si.GetRemoteAddress(), si.GetRemotePort(),
			si.GetUser().GetIdentity(),
			si.GetChannelId(),
			si.GetStatus(), si.GetAuthn().GetStatus(),
			r.GetGrpcService().GetServiceType(),
			r.GetTaskIds(),
			r.GetTimestamp().AsTime(),
		)
	}

	usedRecord = make([]bool, len(gotRecords))
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if tc.res.skip {
				t.Skip("dial attempt skipped (TCP/gRPC setup failed)")
			}
			for _, verifyErr := range runVerifySubtest(&subtestConfig{
				res:        tc.res,
				gotRecords: gotRecords,
				usedRecord: usedRecord,
				t0:         t0,
			}) {
				t.Errorf("%v", verifyErr)
			}
		})
	}
}

// runVerifySubtest matches and validates the accounting record for a test case.
func runVerifySubtest(cfg *subtestConfig) []error {
	matched := (*acctzpb.RecordResponse)(nil)
	for j, resp := range cfg.gotRecords {
		if !cfg.usedRecord[j] && matchRecord(&matchRecordConfig{resp: resp, conn: cfg.res.rec}) {
			cfg.usedRecord[j] = true
			matched = resp
			break
		}
	}
	if matched == nil {
		return []error{fmt.Errorf("no accounting record found for (testCase=%s svcType=%v DUT=%s:%d tester=%s:%d user=%q certFailure=%v)",
			cfg.res.rec.testName, cfg.res.rec.serviceType,
			cfg.res.rec.remoteAddr, cfg.res.rec.remotePort,
			cfg.res.rec.localAddr, cfg.res.rec.localPort,
			cfg.res.rec.username, cfg.res.rec.certFailure,
		)}
	}
	return verifyAuthenFailRecord(&verifyRecordConfig{resp: matched, conn: cfg.res.rec, t0: cfg.t0})
}

// GetRequestMetadata returns gRPC per-RPC metadata with username and password.
func (r *rpcCredentials) GetRequestMetadata(_ context.Context, _ ...string) (map[string]string, error) {
	return map[string]string{
		metadataKeyUsername: r.username,
		metadataKeyPassword: r.password,
	}, nil
}

// RequireTransportSecurity indicates that TLS is required for these credentials.
func (r *rpcCredentials) RequireTransportSecurity() bool { return true }

// rpcGNMI executes a gNMI Capabilities RPC.
func rpcGNMI(t *testing.T, cfg *rpcConfig) error {
	t.Helper()
	_, err := gnmipb.NewGNMIClient(cfg.conn).Capabilities(cfg.ctx, &gnmipb.CapabilityRequest{})
	if err != nil && !cfg.failOK {
		return fmt.Errorf("rpcGNMI: Capabilities failed: %w", err)
	}
	return nil
}

// rpcGNOI executes a gNOI System.Ping RPC.
func rpcGNOI(t *testing.T, cfg *rpcConfig) error {
	t.Helper()
	stream, err := gnoipb.NewSystemClient(cfg.conn).Ping(cfg.ctx, &gnoipb.PingRequest{
		Destination: pingDestination,
		Count:       pingCount,
	})
	if err != nil {
		if !cfg.failOK {
			return fmt.Errorf("rpcGNOI: Ping failed: %w", err)
		}
		return nil
	}
	if _, err = stream.Recv(); err != nil && !cfg.failOK {
		return fmt.Errorf("rpcGNOI: Ping stream Recv failed: %w", err)
	}
	return nil
}

// rpcGNSI executes a gNSI Acctz.RecordSubscribe RPC.
func rpcGNSI(t *testing.T, cfg *rpcConfig) error {
	t.Helper()
	stream, err := acctzpb.NewAcctzStreamClient(cfg.conn).RecordSubscribe(
		cfg.ctx,
		&acctzpb.RecordRequest{Timestamp: timestamppb.New(time.Now())},
	)
	if err != nil {
		if !cfg.failOK {
			return fmt.Errorf("rpcGNSI: RecordSubscribe failed: %w", err)
		}
		return nil
	}
	_, _ = stream.Recv()
	return nil
}

// rpcGRIBI executes a gRIBI Get RPC.
func rpcGRIBI(t *testing.T, cfg *rpcConfig) error {
	t.Helper()
	stream, err := gribi.NewGRIBIClient(cfg.conn).Get(cfg.ctx, &gribi.GetRequest{
		NetworkInstance: &gribi.GetRequest_All{},
		Aft:             gribi.AFTType_IPV4,
	})
	if err != nil {
		if !cfg.failOK {
			return fmt.Errorf("rpcGRIBI: Get failed: %w", err)
		}
		return nil
	}
	_, _ = stream.Recv()
	return nil
}

// rpcP4RT executes a P4RT Capabilities RPC.
func rpcP4RT(t *testing.T, cfg *rpcConfig) error {
	t.Helper()
	_, err := p4pb.NewP4RuntimeClient(cfg.conn).Capabilities(cfg.ctx, &p4pb.CapabilitiesRequest{})
	if err != nil && !cfg.failOK {
		return fmt.Errorf("rpcP4RT: Capabilities failed: %w", err)
	}
	return nil
}

// mustVerifyServiceConnectivity verifies connectivity to all configured services.
func mustVerifyServiceConnectivity(t *testing.T, dut *ondatra.DUTDevice, serviceTable []*serviceEntry) {
	t.Helper()
	username, password := dutRPCCredentials(dut)
	for _, svc := range serviceTable {
		target := svc.target
		conn, err := grpc.NewClient(
			target,
			grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
				InsecureSkipVerify: true,
			})),
		)
		if err != nil {
			t.Fatalf("verifyConnectivity %s: grpc.NewClient: %v", svc.name, err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), dialTimeout)
		ctx = metadata.AppendToOutgoingContext(ctx, metadataKeyUsername, username, metadataKeyPassword, password)
		if err := svc.rpcFn(t, &rpcConfig{conn: conn, ctx: ctx, failOK: false}); err != nil {
			cancel()
			conn.Close()
			t.Fatalf("verifyConnectivity %s: %v", svc.name, err)
		}
		cancel()
		conn.Close()
		t.Logf("verifyConnectivity: %s OK (%s)", svc.name, target)
	}
}

// dialAndFail performs a failed authentication attempt and records connection metadata.
func dialAndFail(t *testing.T, cfg *dialConfig) (*connRecord, bool) {
	used := false

	t.Helper()

	tcpCtx, tcpCancel := context.WithTimeout(context.Background(), dialTimeout)
	defer tcpCancel()

	rawConn, err := (&net.Dialer{}).DialContext(tcpCtx, "tcp", cfg.target)
	if err != nil {
		t.Logf("TCP pre-dial %s: %v (skipping)", cfg.target, err)
		return nil, false
	}

	localAddr, localPort := mustHostPortInfo(t, rawConn.LocalAddr().String())
	remoteAddr, remotePort := mustHostPortInfo(t, cfg.target)

	reuseOrDialTCP := func(ctx context.Context, addr string) (net.Conn, error) {
		if !used {
			used = true
			return rawConn, nil
		}
		return (&net.Dialer{}).DialContext(ctx, "tcp", addr)
	}

	tlsCfg := &tls.Config{InsecureSkipVerify: true}
	if cfg.certFailure {
		tlsCfg.Certificates = []tls.Certificate{cfg.wrongCert}
	}

	grpcConn, err := grpc.NewClient(
		cfg.target,
		grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg)),
		grpc.WithContextDialer(reuseOrDialTCP),
	)
	if err != nil {
		rawConn.Close()
		t.Logf("grpc.NewClient %s: %v (skipping)", cfg.target, err)
		return nil, false
	}
	defer grpcConn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), dialTimeout)
	defer cancel()
	if cfg.certFailure {
		ctx = metadata.AppendToOutgoingContext(ctx, metadataKeyUsername, cfg.username)
	} else {
		ctx = metadata.AppendToOutgoingContext(ctx, metadataKeyUsername, cfg.username, metadataKeyPassword, cfg.password)
	}

	_ = cfg.rpcFn(t, &rpcConfig{conn: grpcConn, ctx: ctx, failOK: true})

	t.Logf("dial attempt: testCase=%s user=%q local=%s:%d remote=%s:%d cert=%v",
		cfg.testName, cfg.username, localAddr, localPort, remoteAddr, remotePort, cfg.certFailure)

	return &connRecord{
		serviceType: cfg.svcType,
		localAddr:   localAddr,
		localPort:   localPort,
		remoteAddr:  remoteAddr,
		remotePort:  remotePort,
		username:    cfg.username,
		testName:    cfg.testName,
		certFailure: cfg.certFailure,
	}, true
}

// matchRecord returns true if resp is the DUT accounting record for conn.
func matchRecord(cfg *matchRecordConfig) bool {
	resp := cfg.resp
	conn := cfg.conn
	si := resp.GetSessionInfo()
	if si == nil {
		return false
	}
	if si.GetStatus() != acctzpb.SessionInfo_SESSION_STATUS_ONCE {
		return false
	}
	if si.GetAuthn().GetStatus() != acctzpb.AuthnDetail_AUTHN_STATUS_FAIL {
		return false
	}
	if si.GetLocalAddress() != conn.remoteAddr || si.GetLocalPort() != conn.remotePort {
		return false
	}
	if conn.isCLI {
		cmdSvc := resp.GetCmdService()
		if cmdSvc == nil || cmdSvc.GetServiceType() != conn.cmdSvcType {
			return false
		}
	} else {
		grpcSvc := resp.GetGrpcService()
		if grpcSvc == nil || grpcSvc.GetServiceType() != conn.serviceType {
			return false
		}
	}

	if ra := si.GetRemoteAddress(); ra != "" && ra != "0.0.0.0" && ra != "::" {
		if ra != conn.localAddr {
			return false
		}
		if rp := si.GetRemotePort(); rp != 0 && rp != conn.localPort {
			return false
		}
	}

	got := si.GetUser().GetIdentity()
	if conn.certFailure && !conn.isCLI {
		return got == "" || got == conn.username || got == wrongCertCN
	}
	return got == conn.username
}

// deviceRecords collects accounting records from the DUT stream until deadline elapses.
func deviceRecords(cfg *deviceRecordsConfig) ([]*acctzpb.RecordResponse, error) {
	cfg.t.Helper()

	records := ([]*acctzpb.RecordResponse)(nil)
	rChan := make(chan *struct {
		record *acctzpb.RecordResponse
		err    error
	})

	go func() {
		defer close(rChan)
		for {
			resp, err := cfg.client.Recv()
			rChan <- &struct {
				record *acctzpb.RecordResponse
				err    error
			}{resp, err}
			if err != nil {
				return
			}
		}
	}()

	timer := time.NewTimer(cfg.deadline)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			return records, nil
		case r, ok := <-rChan:
			if !ok {
				return records, nil
			}
			if r.err != nil {
				return records, r.err
			}
			records = append(records, r.record)
		}
	}
}

// verifyAuthenFailRecord validates an authentication-failure accounting record.
func verifyAuthenFailRecord(cfg *verifyRecordConfig) []error {
	validationErrs := ([]error)(nil)
	resp := cfg.resp
	conn := cfg.conn
	t0 := cfg.t0
	add := func(format string, args ...any) {
		validationErrs = append(validationErrs, fmt.Errorf(format, args...))
	}

	ts := resp.GetTimestamp()
	if ts == nil {
		add("recordResponse.timestamp is nil")
	} else if recTime := ts.AsTime(); !recTime.After(t0) {
		add("timestamp %v is not after T0 %v", recTime, t0)
	}

	if resp.GetHistoryIstruncated() {
		add("history_is_truncated is set but should not be")
	}

	si := resp.GetSessionInfo()
	if si == nil {
		add("session_info is nil")
		return validationErrs
	}

	if got := si.GetIpProto(); got != ipProtoTCP {
		add("session_info.ip_proto: got %d, want %d (TCP)", got, ipProtoTCP)
	}

	if got := si.GetLocalAddress(); got != conn.remoteAddr {
		add("session_info.local_address: got %q, want %q", got, conn.remoteAddr)
	}
	if got := si.GetLocalPort(); got != conn.remotePort {
		add("session_info.local_port: got %d, want %d", got, conn.remotePort)
	}

	if ra := si.GetRemoteAddress(); ra != conn.localAddr {
		add("session_info.remote_address: got %q, want %q", ra, conn.localAddr)
	}
	if rp := si.GetRemotePort(); rp != conn.localPort {
		add("session_info.remote_port: got %d, want %d", rp, conn.localPort)
	}

	if got := si.GetChannelId(); got != "0" {
		add("session_info.channel_id: got %q, want \"0\"", got)
	}

	if !conn.isCLI {
		if got := si.GetTty(); got != "" {
			add("session_info.tty: got %q, want omitted for gRPC", got)
		}
	}

	if got := si.GetStatus(); got != acctzpb.SessionInfo_SESSION_STATUS_ONCE {
		add("session_info.status: got %v, want SESSION_STATUS_ONCE", got)
	}

	authn := si.GetAuthn()
	if authn == nil {
		add("session_info.authn is nil")
		return validationErrs
	}

	switch {
	case conn.certFailure && !conn.isCLI:
		if got := authn.GetType(); got != acctzpb.AuthnDetail_AUTHN_TYPE_TLSCERT {
			add("session_info.authn.type: got %v, want AUTHN_TYPE_TLSCERT (wrong client certificate)", got)
		}
	case conn.certFailure && conn.isCLI:
		if got := authn.GetType(); got != acctzpb.AuthnDetail_AUTHN_TYPE_SSHKEY {
			add("session_info.authn.type: got %v, want AUTHN_TYPE_SSHKEY (wrong SSH key)", got)
		}
	default:
		if got := authn.GetType(); got == acctzpb.AuthnDetail_AUTHN_TYPE_UNSPECIFIED {
			add("session_info.authn.type: got UNSPECIFIED, must equal the authentication method used")
		}
	}
	if got := authn.GetStatus(); got != acctzpb.AuthnDetail_AUTHN_STATUS_FAIL {
		add("session_info.authn.status: got %v, want AUTHN_STATUS_FAIL", got)
	}
	if got := authn.GetCause(); got == "" {
		add("session_info.authn.cause: must be non-empty on authentication failure")
	}

	user := si.GetUser()
	if user == nil {
		add("session_info.user is nil")
		return validationErrs
	}

	if !(conn.certFailure && !conn.isCLI) {
		if got := user.GetIdentity(); got != conn.username {
			add("session_info.user.identity: got %q, want %q", got, conn.username)
		}
	}
	if got := user.GetRole(); got != "" {
		add("session_info.user.role (privilege_level): got %q, want omitted on auth failure", got)
	}

	if conn.isCLI {
		cmdSvc := resp.GetCmdService()
		if cmdSvc == nil {
			add("cmd_service is nil; README requires service_type to be set")
			return validationErrs
		}
		if got := cmdSvc.GetServiceType(); got != conn.cmdSvcType {
			add("cmd_service.service_type: got %v, want %v", got, conn.cmdSvcType)
		}
		if got := cmdSvc.GetCmd(); got != "" {
			add("cmd_service.cmd: got %q, want omitted (no command is authorized/executed on an authentication failure)", got)
		}
		if cmdSvc.GetCmdIstruncated() {
			add("cmd_service.cmd_istruncated: got true, want omitted (no command is present on an authentication failure)")
		}
		if got := cmdSvc.GetCmdArgs(); len(got) != 0 {
			add("cmd_service.cmd_args: got %v, want omitted (no command args are present on an authentication failure)", got)
		}
		if cmdSvc.GetCmdArgsIstruncated() {
			add("cmd_service.cmd_args_istruncated: got true, want omitted (no command args are present on an authentication failure)")
		}
		if authz := cmdSvc.GetAuthz(); authz != nil {
			add("cmd_service.authz: got %v, want omitted (authorization is never reached when authentication fails)", authz)
		}
		return validationErrs
	}

	grpcSvc := resp.GetGrpcService()
	if grpcSvc == nil {
		add("grpc_service is nil; README requires service_type to be set")
		return validationErrs
	}
	if got := grpcSvc.GetServiceType(); got != conn.serviceType {
		add("grpc_service.service_type: got %v, want %v", got, conn.serviceType)
	}
	if got := grpcSvc.GetRpcName(); got != "" {
		add("grpc_service.rpc_name: got %q, want omitted (all fields except service_type must be omitted)", got)
	}
	if got := grpcSvc.GetPayloads(); len(got) != 0 {
		add("grpc_service.payloads: got %d entries, want omitted (the RPC is never processed on an authentication failure)", len(got))
	}
	if grpcSvc.GetPayloadIstruncated() {
		add("grpc_service.payload_istruncated: got true, want omitted (no payload is present on an authentication failure)")
	}
	if got := grpcSvc.GetProtoVal(); got != nil {
		add("grpc_service.proto_val: got %v, want omitted (no payload is present on an authentication failure)", got)
	}
	if got := grpcSvc.GetStringVal(); got != "" {
		add("grpc_service.string_val: got %q, want omitted (no payload is present on an authentication failure)", got)
	}
	if authz := grpcSvc.GetAuthz(); authz != nil {
		add("grpc_service.authz: got %v, want omitted (authorization is never reached when authentication fails)", authz)
	}

	return validationErrs
}

// cleanupTestUser removes the test user from the DUT.
func cleanupTestUser(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	userPath := gnmi.OC().System().Aaa().Authentication().User(acctzlib.SuccessUsername)
	gnmi.Delete(t, dut, userPath.Config())
	t.Logf("deleted test user %q via gNMI OC path", acctzlib.SuccessUsername)
}

// setupTestUser provisions the test user on the DUT via gNMI OC and schedules cleanup.
func setupTestUser(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	userPath := gnmi.OC().System().Aaa().Authentication().User(acctzlib.SuccessUsername)
	user := &oc.System_Aaa_Authentication_User{
		Username: ygot.String(acctzlib.SuccessUsername),
		Role:     oc.UnionString(ocUserRole),
		Password: ygot.String(successPassword),
	}
	gnmi.Replace(t, dut, userPath.Config(), user)
	t.Logf("provisioned test user %q with role %q via gNMI", acctzlib.SuccessUsername, ocUserRole)
	t.Cleanup(func() { cleanupTestUser(t, dut) })
}

// dutRPCCredentials returns the DUT's RPC username and password from the binding, or empty strings if the binding does not expose credentials.
func dutRPCCredentials(dut *ondatra.DUTDevice) (string, string) {
	if c, ok := dut.RawAPIs().BindingDUT().(credentialer); ok {
		return c.RPCUsername(), c.RPCPassword()
	}
	return "", ""
}

// mustGenerateWrongClientCert creates a self-signed TLS client certificate signed by an untrusted CA entirely in memory (no disk I/O), guaranteeing auth failure.
func mustGenerateWrongClientCert(t *testing.T) tls.Certificate {
	t.Helper()

	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate CA key: %v", err)
	}
	now := time.Now()
	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(caSerialNumber),
		Subject:               pkix.Name{CommonName: untrustedCACN},
		NotBefore:             now.Add(-certNotBeforeOffset),
		NotAfter:              now.Add(certValidity),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create CA cert: %v", err)
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatalf("parse CA cert: %v", err)
	}

	clientKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate client key: %v", err)
	}
	clientTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(clientSerialNumber),
		Subject:      pkix.Name{CommonName: wrongCertCN},
		NotBefore:    now.Add(-certNotBeforeOffset),
		NotAfter:     now.Add(certValidity),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	clientDER, err := x509.CreateCertificate(rand.Reader, clientTemplate, caCert, &clientKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create client cert: %v", err)
	}
	clientKeyDER, err := x509.MarshalECPrivateKey(clientKey)
	if err != nil {
		t.Fatalf("marshal client key: %v", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: pemTypeCertificate, Bytes: clientDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: pemTypeECPrivateKey, Bytes: clientKeyDER})
	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("X509KeyPair: %v", err)
	}
	t.Logf("generated wrong client TLS certificate in memory: CN=%s", wrongCertCN)
	return tlsCert
}

// mustGenerateWrongSSHKey creates an in-memory SSH key pair not registered as
// an authorized key on the DUT, guaranteeing auth failure.
func mustGenerateWrongSSHKey(t *testing.T) ssh.Signer {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate wrong SSH key: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatalf("ssh.NewSignerFromKey: %v", err)
	}
	t.Logf("generated wrong SSH client key in memory")
	return signer
}

// dialAndFailSSH performs a failed SSH authentication attempt against the DUT's
// CLI (CommandService) access method and records connection metadata, mirroring dialAndFail's gRPC handling.
func dialAndFailSSH(t *testing.T, cfg *dialConfig) (*connRecord, bool) {
	t.Helper()

	tcpCtx, tcpCancel := context.WithTimeout(context.Background(), dialTimeout)
	defer tcpCancel()

	rawConn, err := (&net.Dialer{}).DialContext(tcpCtx, "tcp", cfg.target)
	if err != nil {
		t.Logf("SSH TCP pre-dial %s: %v (skipping)", cfg.target, err)
		return nil, false
	}

	localAddr, localPort := mustHostPortInfo(t, rawConn.LocalAddr().String())
	remoteAddr, remotePort := mustHostPortInfo(t, cfg.target)

	sshCfg := &ssh.ClientConfig{
		User:            cfg.username,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec
		Timeout:         dialTimeout,
	}
	if cfg.certFailure {
		sshCfg.Auth = []ssh.AuthMethod{ssh.PublicKeys(cfg.wrongSSHKey)}
	} else {
		sshCfg.Auth = []ssh.AuthMethod{ssh.Password(cfg.password)}
	}

	sc, chans, reqs, err := ssh.NewClientConn(rawConn, cfg.target, sshCfg)
	if err != nil {
		rawConn.Close()
	} else {
		client := ssh.NewClient(sc, chans, reqs)
		client.Close()
		t.Logf("dial attempt: testCase=%s SSH authentication unexpectedly succeeded", cfg.testName)
	}

	t.Logf("SSH dial attempt: testCase=%s user=%q local=%s:%d remote=%s:%d wrongKey=%v",
		cfg.testName, cfg.username, localAddr, localPort, remoteAddr, remotePort, cfg.certFailure)

	return &connRecord{
		cmdSvcType:  acctzpb.CommandService_CMD_SERVICE_TYPE_CLI,
		isCLI:       true,
		localAddr:   localAddr,
		localPort:   localPort,
		remoteAddr:  remoteAddr,
		remotePort:  remotePort,
		username:    cfg.username,
		testName:    cfg.testName,
		certFailure: cfg.certFailure,
	}, true
}

// mustHostPortInfo splits an address into host and port, fatally failing on error.
func mustHostPortInfo(t *testing.T, address string) (string, uint32) {
	t.Helper()
	ip, portRaw, err := net.SplitHostPort(address)
	if err != nil {
		t.Fatalf("SplitHostPort(%q): %v", address, err)
	}
	p, err := strconv.ParseUint(portRaw, 10, 32)
	if err != nil {
		t.Fatalf("parse port %q: %v", portRaw, err)
	}
	return ip, uint32(p)
}
