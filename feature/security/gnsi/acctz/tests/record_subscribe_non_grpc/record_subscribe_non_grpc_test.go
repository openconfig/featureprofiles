package record_subscribe_non_grpc_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnsi/acctz"
	tpb "github.com/openconfig/kne/proto/topo"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/binding"
	ondatragnmi "github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"golang.org/x/crypto/ssh"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	successUsername = "acctztestuser"
	successPassword = "verysecurepassword"
	successRoleName = "acctz-fp-test-success"
	failUsername    = "bilbo"
	failPassword    = "baggins"
	failRoleName    = "acctz-fp-test-fail"
	command         = "show version"
	failCommand     = "show version"
	shellCommand    = "uname -a"
)

type rpcRecord struct {
	startTime            time.Time
	doneTime             time.Time
	cmdType              acctz.CommandService_CmdServiceType
	rpcPath              string
	localIp              string
	localPort            uint32
	remoteIp             string
	remotePort           uint32
	succeeded            bool
	expectedStatus       acctz.SessionInfo_SessionStatus
	expectedAuthenType   acctz.AuthnDetail_AuthnType
	expectedAuthenStatus acctz.AuthnDetail_AuthnStatus
	expectedAuthenCause  string
	expectedIdentity     string
	expectedRole         string
}

type recordRequestResult struct {
	record *acctz.RecordResponse
	err    error
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func createNativeRole(t testing.TB, dut *ondatra.DUTDevice) {
	var SetRequest *gpb.SetRequest
	switch dut.Vendor() {
	case ondatra.NOKIA:
		successRoleData, err := json.Marshal([]any{
			map[string]any{
				"services": []string{"cli"},
			},
		})
		if err != nil {
			t.Fatalf("Error with json Marshal: %v", err)
		}

		failRoleData, err := json.Marshal([]any{
			map[string]any{
				"services": []string{"cli"},
				"cli": map[string][]string{
					"deny-command-list": {"show version"},
				},
			},
		})
		if err != nil {
			t.Fatalf("Error with json Marshal: %v", err)
		}

		successUserData, err := json.Marshal([]any{
			map[string]any{
				"password": successPassword,
				"role":     []string{successRoleName},
			},
		})
		if err != nil {
			t.Fatalf("Error with json Marshal: %v", err)
		}

		failUserData, err := json.Marshal([]any{
			map[string]any{
				"password": failPassword,
				"role":     []string{failRoleName},
			},
		})
		if err != nil {
			t.Fatalf("Error with json Marshal: %v", err)
		}

		SetRequest = &gpb.SetRequest{
			Prefix: &gpb.Path{
				Origin: "native",
			},
			Replace: []*gpb.Update{
				{
					Path: &gpb.Path{
						Elem: []*gpb.PathElem{
							{Name: "system"},
							{Name: "aaa"},
							{Name: "authorization"},
							{Name: "role", Key: map[string]string{"rolename": successRoleName}},
						},
					},
					Val: &gpb.TypedValue{
						Value: &gpb.TypedValue_JsonIetfVal{
							JsonIetfVal: successRoleData,
						},
					},
				},
				{
					Path: &gpb.Path{
						Elem: []*gpb.PathElem{
							{Name: "system"},
							{Name: "aaa"},
							{Name: "authorization"},
							{Name: "role", Key: map[string]string{"rolename": failRoleName}},
						},
					},
					Val: &gpb.TypedValue{
						Value: &gpb.TypedValue_JsonIetfVal{
							JsonIetfVal: failRoleData,
						},
					},
				},
				{
					Path: &gpb.Path{
						Elem: []*gpb.PathElem{
							{Name: "system"},
							{Name: "aaa"},
							{Name: "authentication"},
							{Name: "user", Key: map[string]string{"username": successUsername}},
						},
					},
					Val: &gpb.TypedValue{
						Value: &gpb.TypedValue_JsonIetfVal{
							JsonIetfVal: successUserData,
						},
					},
				},
				{
					Path: &gpb.Path{
						Elem: []*gpb.PathElem{
							{Name: "system"},
							{Name: "aaa"},
							{Name: "authentication"},
							{Name: "user", Key: map[string]string{"username": failUsername}},
						},
					},
					Val: &gpb.TypedValue{
						Value: &gpb.TypedValue_JsonIetfVal{
							JsonIetfVal: failUserData,
						},
					},
				},
			},
		}
	default:
		t.Fatalf("Unsupported vendor %s for deviation 'deviation_native_users'", dut.Vendor())
	}
	gnmiClient := dut.RawAPIs().GNMI(t)
	if _, err := gnmiClient.Set(context.Background(), SetRequest); err != nil {
		t.Fatalf("Unexpected error configuring User: %v", err)
	}
}

func setupUsers(t *testing.T, dut *ondatra.DUTDevice) {
	auth := &oc.System_Aaa_Authentication{}
	auth.GetOrCreateUser(successUsername)
	auth.GetOrCreateUser(failUsername)

	ondatragnmi.Update(t, dut, ondatragnmi.OC().System().Aaa().Authentication().Config(), auth)

	if deviations.SetNativeUser(dut) {
		// probably all vendors need to handle this since the user should have a role attached to
		// it allowing us to login via ssh/console/whatever
		createNativeRole(t, dut)
	}
}

func dialSSH(t *testing.T, username, password, addr string, port uint32) (net.Conn, io.Writer, io.Reader) {
	tcpConn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", addr, port), 0)
	if err != nil {
		t.Fatalf("got unexpected error dialing ssh tcp connection, error: %s", err)
	}

	cConn, chans, reqs, err := ssh.NewClientConn(
		tcpConn,
		fmt.Sprintf("%s:%d", addr, port),
		&ssh.ClientConfig{
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
		},
	)
	if err != nil {
		t.Fatalf("got unexpected error dialing ssh, error: %s", err)
	}

	// stdin/stdout so we get a tty allocated
	conn := ssh.NewClient(cConn, chans, reqs)

	sess, err := conn.NewSession()
	if err != nil {
		t.Fatalf("failed creating ssh session, error: %s", err)
	}

	w, err := sess.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}

	r, err := sess.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}

	term := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 115200,
		ssh.TTY_OP_OSPEED: 115200,
	}

	err = sess.RequestPty(
		"xterm",
		255,
		80,
		term,
	)
	if err != nil {
		t.Fatal(err)
	}

	err = sess.Shell()
	if err != nil {
		t.Fatal(err)
	}

	return tcpConn, w, r
}

func sendCLICommand(t *testing.T, addr string, port uint32) []rpcRecord {
	var records []rpcRecord

	tcpConn, w, _ := dialSSH(t, successUsername, successPassword, addr, port)
	defer func() {
		// give things a second to percolate then close the connection
		time.Sleep(3 * time.Second)

		err := tcpConn.Close()
		if err != nil {
			t.Logf("error closing tcp(ssh) connection, will ignore, error: %s", err)
		}
	}()

	startTime := time.Now()

	time.Sleep(time.Second)

	// this might not work for other vendors, so probably we can have a switch here and pass
	// the writer to func per vendor if needed
	_, err := w.Write([]byte(fmt.Sprintf("%s\n", command)))
	if err != nil {
		t.Fatalf("failed sending cli command, error: %s", err)
	}

	addrParts := strings.Split(tcpConn.LocalAddr().String(), ":")
	remoteAddr := addrParts[0]
	remotePort, _ := strconv.Atoi(addrParts[1])

	resolvedAddr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:%d", addr, port))
	if err != nil {
		t.Fatalf("failed resolving ssh destination addr, error: %s", err)
	}

	addr = resolvedAddr.IP.String()

	records = append(records, rpcRecord{
		startTime:            startTime,
		doneTime:             time.Now(),
		cmdType:              acctz.CommandService_CMD_SERVICE_TYPE_CLI,
		localIp:              addr,
		localPort:            port,
		remoteIp:             remoteAddr,
		remotePort:           uint32(remotePort),
		succeeded:            true,
		expectedStatus:       acctz.SessionInfo_SESSION_STATUS_OPERATION,
		expectedAuthenType:   acctz.AuthnDetail_AUTHN_TYPE_UNSPECIFIED,
		expectedAuthenStatus: acctz.AuthnDetail_AUTHN_STATUS_SUCCESS,
		expectedAuthenCause:  "authentication_method: local",
		expectedIdentity:     successUsername,
		expectedRole:         successRoleName,
	})

	return records
}

func sendCLICommandFail(t *testing.T, addr string, port uint32) []rpcRecord {
	var records []rpcRecord

	tcpConn, w, _ := dialSSH(t, failUsername, failPassword, addr, port)
	defer func() {
		// give things a second to percolate then close the connection
		time.Sleep(3 * time.Second)

		err := tcpConn.Close()
		if err != nil {
			t.Logf("error closing tcp(ssh) connection, will ignore, error: %s", err)
		}
	}()

	startTime := time.Now()

	time.Sleep(time.Second)

	_, err := w.Write([]byte(fmt.Sprintf("%s\n", failCommand)))
	if err != nil {
		t.Fatalf("failed sending cli command, error: %s", err)
	}

	addrParts := strings.Split(tcpConn.LocalAddr().String(), ":")
	remoteAddr := addrParts[0]
	remotePort, _ := strconv.Atoi(addrParts[1])

	resolvedAddr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:%d", addr, port))
	if err != nil {
		t.Fatalf("failed resolving ssh destination addr, error: %s", err)
	}

	addr = resolvedAddr.IP.String()

	records = append(records, rpcRecord{
		startTime:            startTime,
		doneTime:             time.Now(),
		cmdType:              acctz.CommandService_CMD_SERVICE_TYPE_CLI,
		localIp:              addr,
		localPort:            port,
		remoteIp:             remoteAddr,
		remotePort:           uint32(remotePort),
		succeeded:            true,
		expectedStatus:       acctz.SessionInfo_SESSION_STATUS_OPERATION,
		expectedAuthenType:   acctz.AuthnDetail_AUTHN_TYPE_UNSPECIFIED,
		expectedAuthenStatus: acctz.AuthnDetail_AUTHN_STATUS_SUCCESS,
		expectedAuthenCause:  "authentication_method: local",
		expectedIdentity:     failUsername,
		expectedRole:         failRoleName,
	})

	return records
}

func sendShellCommand(t *testing.T, dut *ondatra.DUTDevice, addr string, port uint32) []rpcRecord {
	var records []rpcRecord

	shellUsername := successUsername
	shellPassword := successPassword

	switch dut.Vendor() {
	case ondatra.NOKIA:
		// assuming linuxadmin is present and ssh'ing directly via this user gets us to shell
		// straight away so this is easy button to trigger a shell record
		shellUsername = "linuxadmin"
		shellPassword = "NokiaSrl1!"
	}

	tcpConn, w, _ := dialSSH(t, shellUsername, shellPassword, addr, port)
	defer func() {
		// give things a second to percolate then close the connection
		time.Sleep(3 * time.Second)

		err := tcpConn.Close()
		if err != nil {
			t.Logf("error closing tcp(ssh) connection, will ignore, error: %s", err)
		}
	}()

	startTime := time.Now()

	// this might not work for other vendors, so probably we can have a switch here and pass
	// the writer to func per vendor if needed
	_, err := w.Write([]byte(fmt.Sprintf("%s\n", shellCommand)))
	if err != nil {
		t.Fatalf("failed sending cli command, error: %s", err)
	}

	addrParts := strings.Split(tcpConn.LocalAddr().String(), ":")
	remoteAddr := addrParts[0]
	remotePort, _ := strconv.Atoi(addrParts[1])

	resolvedAddr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:%d", addr, port))
	if err != nil {
		t.Fatalf("failed resolving ssh destination addr, error: %s", err)
	}

	addr = resolvedAddr.IP.String()

	records = append(records, rpcRecord{
		startTime:            startTime,
		doneTime:             time.Now(),
		cmdType:              acctz.CommandService_CMD_SERVICE_TYPE_SHELL,
		localIp:              addr,
		localPort:            port,
		remoteIp:             remoteAddr,
		remotePort:           uint32(remotePort),
		succeeded:            true,
		expectedStatus:       acctz.SessionInfo_SESSION_STATUS_OPERATION,
		expectedAuthenType:   acctz.AuthnDetail_AUTHN_TYPE_UNSPECIFIED,
		expectedAuthenStatus: acctz.AuthnDetail_AUTHN_STATUS_UNSPECIFIED,
		expectedAuthenCause:  "",
		expectedIdentity:     shellUsername,
	})

	return records
}

func getDutAddr(t *testing.T, dut *ondatra.DUTDevice) string {
	var serviceDUT interface {
		Service(string) (*tpb.Service, error)
	}

	err := binding.DUTAs(dut.RawAPIs().BindingDUT(), &serviceDUT)
	if err != nil {
		t.Log("DUT does not support `Service` function, will attempt to use dut name field")

		return dut.Name()
	}

	dutSSHService, err := serviceDUT.Service("ssh")
	if err != nil {
		t.Fatal(err)
	}

	return dutSSHService.GetOutsideIp()
}

func TestAccountzRecordSubscribeNonGRPC(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	setupUsers(t, dut)

	// https://github.com/openconfig/featureprofiles/issues/2637
	// basically, just waiting to see what the "best"/"preferred" way is to get the v4/v6 of the
	// dut -- for now we use this hacky work around because ssh isn't exposed in introspection anyway
	// so... we get what we can get.
	addr := getDutAddr(t, dut)

	var records []rpcRecord

	// put enough time between the test starting and any prior events so we can easily know where
	// our records start
	time.Sleep(5 * time.Second)

	startTime := time.Now()

	// suppose ssh could be not 22 in some cases but don't think this is exposed by introspect
	newRecords := sendCLICommand(t, addr, 22)
	records = append(records, newRecords...)

	newRecords = sendCLICommandFail(t, addr, 22)
	records = append(records, newRecords...)

	newRecords = sendShellCommand(t, dut, addr, 22)
	records = append(records, newRecords...)

	// quick sleep to ensure all the records have been processed/ready for us
	time.Sleep(5 * time.Second)

	// get gnsi record subscribe client
	acctzClient := dut.RawAPIs().GNSI(t).Acctz()

	acctzSubClient, err := acctzClient.RecordSubscribe(context.Background())
	if err != nil {
		t.Fatalf("failed getting accountz record subscribe client, error: %s", err)
	}

	// this will have to move up to RecordSubscribe call after this is brought into fp/ondatra stuff
	// https://github.com/openconfig/gnsi/pull/149/files
	err = acctzSubClient.Send(&acctz.RecordRequest{
		Timestamp: &timestamppb.Timestamp{
			Seconds: 0,
			Nanos:   0,
		},
	})
	if err != nil {
		t.Fatalf("failed sending accountz record request, error: %s", err)
	}

	var recordIdx int

	var lastTimestampUnixMillis int64
	var lastTaskID string

	for {
		if recordIdx >= len(records) {
			t.Log("out of records to process...")

			break
		}

		r := make(chan recordRequestResult)

		go func(r chan recordRequestResult) {
			var response *acctz.RecordResponse

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
			t.Log("done receiving records...")

			break
		}

		if resp.err != nil {
			t.Fatalf("failed receiving record response, error: %s", resp.err)
		}

		if resp.record.GetHistoryIstruncated() {
			t.Fatal("history is truncated but it shouldnt be")
		}

		if !resp.record.Timestamp.AsTime().After(startTime) {
			// skipping record, was before test start time
			continue
		}

		// check that the timestamp for the record is between our start/stop times for our rpc
		timestamp := resp.record.Timestamp.AsTime()

		if timestamp.UnixMilli() == lastTimestampUnixMillis {
			// this ensures that timestamps are actually changing for each record
			t.Fatalf("timestamp is the same as the previous timestamp, this shouldnt be possible!")
		}

		lastTimestampUnixMillis = timestamp.UnixMilli()

		// some task ids may be tracked multiple times (for start/stop accounting). if we see two in
		// a row that are the same task we know this is what's up and we can skip this record and
		// continue
		currentTaskID := resp.record.TaskIds[0]
		if currentTaskID == lastTaskID {
			continue
		}

		lastTaskID = currentTaskID

		if records[recordIdx].startTime.Unix() > timestamp.Unix() {
			t.Fatalf(
				"record timestamp is prior to rpc start time timestamp, rpc start timestamp %d, record timestamp %d",
				records[recordIdx].startTime.Unix(),
				timestamp.Unix(),
			)
		}

		// done time (that we recorded when making the rpc) + 2 second for some breathing room
		if records[recordIdx].doneTime.Unix()+2 < timestamp.Unix() {
			t.Fatalf(
				"record timestamp is after rpc end timestamp, rpc end timestamp %d, record timestamp %d",
				records[recordIdx].doneTime.Unix()+2,
				timestamp.Unix(),
			)
		}

		cmdType := resp.record.GetCmdService().GetServiceType()

		if records[recordIdx].cmdType != cmdType {
			t.Fatalf("service type not correct, got %q, want %q", cmdType, records[recordIdx].cmdType)
		}

		servicePath := resp.record.GetGrpcService().GetRpcName()
		if records[recordIdx].rpcPath != servicePath {
			t.Fatalf("service path not correct, got %q, want %q", servicePath, records[recordIdx].rpcPath)
		}

		channelID := resp.record.GetSessionInfo().GetChannelId()

		// this channel check maybe should just go away entirely -- see:
		// https://github.com/openconfig/gnsi/issues/98
		// in case of nokia this is being set to the aaa session id just to have some hopefully
		// useful info in this field to identify a "session" (even if it isn't necessarily ssh/grpc
		// directly)
		if !records[recordIdx].succeeded {
			if channelID != "aaa_session_id: 0" {
				t.Fatalf("auth was not successful for this record, but channel id was set, got %q", channelID)
			}
		}

		// status
		sessionStatus := resp.record.GetSessionInfo().GetStatus()
		if records[recordIdx].expectedStatus != sessionStatus {
			t.Fatalf("session status not correct, got %q, want %q", sessionStatus, records[recordIdx].expectedStatus)
		}

		// authen type
		authenType := resp.record.GetSessionInfo().GetAuthn().GetType()
		if records[recordIdx].expectedAuthenType != authenType {
			t.Fatalf("authenType not correct, got %q, want %q", authenType, records[recordIdx].expectedAuthenType)
		}

		authenStatus := resp.record.GetSessionInfo().GetAuthn().GetStatus()
		if records[recordIdx].expectedAuthenStatus != authenStatus {
			t.Fatalf("authenStatus not correct, got %q, want %q", authenStatus, records[recordIdx].expectedAuthenStatus)
		}

		authenCause := resp.record.GetSessionInfo().GetAuthn().GetCause()
		if records[recordIdx].expectedAuthenCause != authenCause {
			t.Fatalf("authenCause not correct, got %q, want %q", authenCause, records[recordIdx].expectedAuthenCause)
		}

		userIdentity := resp.record.GetSessionInfo().GetUser().GetIdentity()
		if records[recordIdx].expectedIdentity != userIdentity {
			t.Fatalf("identity not correct, got %q, want %q", userIdentity, records[recordIdx].expectedIdentity)
		}

		if !records[recordIdx].succeeded {
			// not a successful rpc so don't need to check anything else
			recordIdx++

			continue
		}

		role := resp.record.GetSessionInfo().GetUser().GetRole()
		if records[recordIdx].expectedRole != role {
			t.Fatalf("role not correct, got %q, want %q", role, records[recordIdx].expectedRole)
		}

		// verify the l4 bits align, this stuff is only set if auth is successful so do it down here
		localAddr := resp.record.GetSessionInfo().GetLocalAddress()
		if records[recordIdx].localIp != localAddr {
			t.Fatalf("local address not correct, got %q, want %q", localAddr, records[recordIdx].localIp)
		}

		localPort := resp.record.GetSessionInfo().GetLocalPort()
		if records[recordIdx].localPort != localPort {
			t.Fatalf("local port not correct, got %d, want %d", localPort, records[recordIdx].localPort)
		}

		remoteAddr := resp.record.GetSessionInfo().GetRemoteAddress()
		if records[recordIdx].remoteIp != remoteAddr {
			t.Fatalf("remote address not correct, got %q, want %q", remoteAddr, records[recordIdx].remoteIp)
		}

		remotePort := resp.record.GetSessionInfo().GetRemotePort()
		if records[recordIdx].remotePort != remotePort {
			t.Fatalf("remote port not correct, got %d, want %d", remotePort, records[recordIdx].remotePort)
		}

		recordIdx++
	}

	if recordIdx != len(records) {
		t.Fatal("did not process all records")
	}
}
