package record_subscribe_non_grpc_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"github.com/openconfig/gnsi/credentialz"

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
	failUsername    = "bilbo"
	failPassword    = "baggins"
	failRoleName    = "acctz-fp-test-fail"
	command         = "show version"
	failCommand     = "show version"
	shellCommand    = "uname -a"
	sshPort         = 22
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
}

type recordRequestResult struct {
	record *acctz.RecordResponse
	err    error
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func setupUserPassword(t *testing.T, dut *ondatra.DUTDevice, username, password string) {
	request := &credentialz.RotateAccountCredentialsRequest{
		Request: &credentialz.RotateAccountCredentialsRequest_Password{
			Password: &credentialz.PasswordRequest{
				Accounts: []*credentialz.PasswordRequest_Account{
					{
						Account: username,
						Password: &credentialz.PasswordRequest_Password{
							Value: &credentialz.PasswordRequest_Password_Plaintext{
								Plaintext: password,
							},
						},
						Version:   "v1.0",
						CreatedOn: uint64(time.Now().Unix()),
					},
				},
			},
		},
	}

	credzClient := dut.RawAPIs().GNSI(t).Credentialz()

	credzRotateClient, err := credzClient.RotateAccountCredentials(context.Background())
	if err != nil {
		t.Fatalf("failed fetching credentialz rotate account credentials client, error: %s", err)
	}

	err = credzRotateClient.Send(request)
	if err != nil {
		t.Fatalf("failed sending credentialz rotate account credentials request, error: %s", err)
	}

	_, err = credzRotateClient.Recv()
	if err != nil {
		t.Fatalf("failed receiving credentialz rotate account credentials response, error: %s", err)
	}

	err = credzRotateClient.Send(&credentialz.RotateAccountCredentialsRequest{
		Request: &credentialz.RotateAccountCredentialsRequest_Finalize{
			Finalize: request.GetFinalize(),
		},
	})
	if err != nil {
		t.Fatalf("failed sending credentialz rotate account credentials finalize request, error: %s", err)
	}

	// brief sleep for finalize to get processed
	time.Sleep(time.Second)
}

func nokiaRole(t *testing.T) *gpb.SetRequest {
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

	return &gpb.SetRequest{
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
						{Name: "role", Key: map[string]string{"rolename": failRoleName}},
					},
				},
				Val: &gpb.TypedValue{
					Value: &gpb.TypedValue_JsonIetfVal{
						JsonIetfVal: failRoleData,
					},
				},
			},
		},
	}
}

func setupUsers(t *testing.T, dut *ondatra.DUTDevice) {
	var SetRequest *gpb.SetRequest

	//Create failure role in native
	switch dut.Vendor() {
	case ondatra.NOKIA:
		SetRequest = nokiaRole(t)
	}

	gnmiClient := dut.RawAPIs().GNMI(t)
	if _, err := gnmiClient.Set(context.Background(), SetRequest); err != nil {
		t.Fatalf("Unexpected error configuring role: %v", err)
	}

	//Configure users
	auth := &oc.System_Aaa_Authentication{}
	successUser := auth.GetOrCreateUser(successUsername)
	successUser.SetRole(oc.AaaTypes_SYSTEM_DEFINED_ROLES_SYSTEM_ROLE_ADMIN)
	failUser := auth.GetOrCreateUser(failUsername)
	failUser.SetRole(oc.UnionString(failRoleName))
	ondatragnmi.Update(t, dut, ondatragnmi.OC().System().Aaa().Authentication().Config(), auth)
	setupUserPassword(t, dut, successUsername, successPassword)
	setupUserPassword(t, dut, failUsername, failPassword)
}

func dialSSH(t *testing.T, username, password, target string) (net.Conn, io.Writer, io.Reader) {
	tcpConn, err := net.DialTimeout("tcp", target, 0)
	if err != nil {
		t.Fatalf("got unexpected error dialing ssh tcp connection, error: %s", err)
	}

	cConn, chans, reqs, err := ssh.NewClientConn(
		tcpConn,
		target,
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

func sendCLICommand(t *testing.T, target string) []rpcRecord {
	var records []rpcRecord

	tcpConn, w, _ := dialSSH(t, successUsername, successPassword, target)
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

	// remote from the perspective of the router
	remoteAddr, err := net.ResolveTCPAddr("tcp", tcpConn.LocalAddr().String())
	if err != nil {
		t.Fatalf("failed resolving ssh remote addr, error: %s", err)
	}
	localAddr, err := net.ResolveTCPAddr("tcp", target)
	if err != nil {
		t.Fatalf("failed resolving ssh local addr, error: %s", err)
	}

	records = append(records, rpcRecord{
		startTime:            startTime,
		doneTime:             time.Now(),
		cmdType:              acctz.CommandService_CMD_SERVICE_TYPE_CLI,
		localIp:              localAddr.IP.String(),
		localPort:            uint32(localAddr.Port),
		remoteIp:             remoteAddr.IP.String(),
		remotePort:           uint32(remoteAddr.Port),
		succeeded:            true,
		expectedStatus:       acctz.SessionInfo_SESSION_STATUS_OPERATION,
		expectedAuthenType:   acctz.AuthnDetail_AUTHN_TYPE_UNSPECIFIED,
		expectedAuthenStatus: acctz.AuthnDetail_AUTHN_STATUS_SUCCESS,
		expectedAuthenCause:  "authentication_method: local",
		expectedIdentity:     successUsername,
	})

	return records
}

func sendCLICommandFail(t *testing.T, target string) []rpcRecord {
	var records []rpcRecord

	tcpConn, w, _ := dialSSH(t, failUsername, failPassword, target)
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

	// remote from the perspective of the router
	remoteAddr, err := net.ResolveTCPAddr("tcp", tcpConn.LocalAddr().String())
	if err != nil {
		t.Fatalf("failed resolving ssh remote addr, error: %s", err)
	}
	localAddr, err := net.ResolveTCPAddr("tcp", target)
	if err != nil {
		t.Fatalf("failed resolving ssh local addr, error: %s", err)
	}

	records = append(records, rpcRecord{
		startTime:            startTime,
		doneTime:             time.Now(),
		cmdType:              acctz.CommandService_CMD_SERVICE_TYPE_CLI,
		localIp:              localAddr.IP.String(),
		localPort:            uint32(localAddr.Port),
		remoteIp:             remoteAddr.IP.String(),
		remotePort:           uint32(remoteAddr.Port),
		succeeded:            true,
		expectedStatus:       acctz.SessionInfo_SESSION_STATUS_OPERATION,
		expectedAuthenType:   acctz.AuthnDetail_AUTHN_TYPE_UNSPECIFIED,
		expectedAuthenStatus: acctz.AuthnDetail_AUTHN_STATUS_SUCCESS,
		expectedAuthenCause:  "authentication_method: local",
		expectedIdentity:     failUsername,
	})

	return records
}

func sendShellCommand(t *testing.T, dut *ondatra.DUTDevice, target string) []rpcRecord {
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

	tcpConn, w, _ := dialSSH(t, shellUsername, shellPassword, target)
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

	// remote from the perspective of the router
	remoteAddr, err := net.ResolveTCPAddr("tcp", tcpConn.LocalAddr().String())
	if err != nil {
		t.Fatalf("failed resolving ssh remote addr, error: %s", err)
	}
	localAddr, err := net.ResolveTCPAddr("tcp", target)
	if err != nil {
		t.Fatalf("failed resolving ssh local addr, error: %s", err)
	}

	records = append(records, rpcRecord{
		startTime:            startTime,
		doneTime:             time.Now(),
		cmdType:              acctz.CommandService_CMD_SERVICE_TYPE_SHELL,
		localIp:              localAddr.IP.String(),
		localPort:            uint32(localAddr.Port),
		remoteIp:             remoteAddr.IP.String(),
		remotePort:           uint32(remoteAddr.Port),
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

func prettyPrint(i interface{}) string {
	s, _ := json.MarshalIndent(i, "", "\t")
	return string(s)
}

func TestAccountzRecordSubscribeNonGRPC(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	setupUsers(t, dut)

	// https://github.com/openconfig/featureprofiles/issues/2637
	// basically, just waiting to see what the "best"/"preferred" way is to get the v4/v6 of the
	// dut -- for now we use this hacky work around because ssh isn't exposed in introspection anyway
	// so... we get what we can get.
	addr := getDutAddr(t, dut)

	// suppose ssh could be not 22 in some cases but don't think this is exposed by introspect
	target := fmt.Sprintf("%s:%d", addr, sshPort)
	t.Logf("Target for SSH service: %s", target)

	var records []rpcRecord

	// put enough time between the test starting and any prior events so we can easily know where
	// our records start
	time.Sleep(5 * time.Second)

	startTime := time.Now()

	newRecords := sendCLICommand(t, target)
	records = append(records, newRecords...)

	newRecords = sendCLICommandFail(t, target)
	records = append(records, newRecords...)

	newRecords = sendShellCommand(t, dut, target)
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
			t.Fatalf("history is truncated but it shouldn't be, Record Details: %s", prettyPrint(resp.record))
		}

		if !resp.record.Timestamp.AsTime().After(startTime) {
			// skipping record, was before test start time
			continue
		}

		// check that the timestamp for the record is between our start/stop times for our rpc
		timestamp := resp.record.Timestamp.AsTime()

		if timestamp.UnixMilli() == lastTimestampUnixMillis {
			// this ensures that timestamps are actually changing for each record
			t.Fatalf("timestamp is the same as the previous timestamp, this shouldn't be possible!, Record Details: %s", prettyPrint(resp.record))
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

		// -2 for a little breathing room since things may not be perfectly synced up time-wise
		if records[recordIdx].startTime.Unix()-2 > timestamp.Unix() {
			t.Fatalf(
				"record timestamp is prior to rpc start time timestamp, rpc start timestamp %d, record timestamp %d, Record Details: %s",
				records[recordIdx].startTime.Unix()-2,
				timestamp.Unix(),
				prettyPrint(resp.record),
			)
		}

		// done time (that we recorded when making the rpc) + 2 second for some breathing room
		if records[recordIdx].doneTime.Unix()+2 < timestamp.Unix() {
			t.Fatalf(
				"record timestamp is after rpc end timestamp, rpc end timestamp %d, record timestamp %d, Record Details: %s",
				records[recordIdx].doneTime.Unix()+2,
				timestamp.Unix(),
				prettyPrint(resp.record),
			)
		}

		cmdType := resp.record.GetCmdService().GetServiceType()

		if records[recordIdx].cmdType != cmdType {
			t.Fatalf("service type not correct, got %q, want %q, Record Details: %s", cmdType, records[recordIdx].cmdType, prettyPrint(resp.record))
		}

		servicePath := resp.record.GetGrpcService().GetRpcName()
		if records[recordIdx].rpcPath != servicePath {
			t.Fatalf("service path not correct, got %q, want %q, Record Details: %s", servicePath, records[recordIdx].rpcPath, prettyPrint(resp.record))
		}

		channelID := resp.record.GetSessionInfo().GetChannelId()

		// this channel check maybe should just go away entirely -- see:
		// https://github.com/openconfig/gnsi/issues/98
		// in case of nokia this is being set to the aaa session id just to have some hopefully
		// useful info in this field to identify a "session" (even if it isn't necessarily ssh/grpc
		// directly)
		if !records[recordIdx].succeeded {
			if channelID != "aaa_session_id: 0" {
				t.Fatalf("auth was not successful for this record, but channel id was set, got %q, Record Details: %s", channelID, prettyPrint(resp.record))
			}
		}

		// status
		sessionStatus := resp.record.GetSessionInfo().GetStatus()
		if records[recordIdx].expectedStatus != sessionStatus {
			t.Fatalf("session status not correct, got %q, want %q, Record Details: %s", sessionStatus, records[recordIdx].expectedStatus, prettyPrint(resp.record))
		}

		// authen type
		authenType := resp.record.GetSessionInfo().GetAuthn().GetType()
		if records[recordIdx].expectedAuthenType != authenType {
			t.Fatalf("authenType not correct, got %q, want %q, Record Details: %s", authenType, records[recordIdx].expectedAuthenType, prettyPrint(resp.record))
		}

		authenStatus := resp.record.GetSessionInfo().GetAuthn().GetStatus()
		if records[recordIdx].expectedAuthenStatus != authenStatus {
			t.Fatalf("authenStatus not correct, got %q, want %q, Record Details: %s", authenStatus, records[recordIdx].expectedAuthenStatus, prettyPrint(resp.record))
		}

		authenCause := resp.record.GetSessionInfo().GetAuthn().GetCause()
		if records[recordIdx].expectedAuthenCause != authenCause {
			t.Fatalf("authenCause not correct, got %q, want %q, Record Details: %s", authenCause, records[recordIdx].expectedAuthenCause, prettyPrint(resp.record))
		}

		userIdentity := resp.record.GetSessionInfo().GetUser().GetIdentity()
		if records[recordIdx].expectedIdentity != userIdentity {
			t.Fatalf("identity not correct, got %q, want %q, Record Details: %s", userIdentity, records[recordIdx].expectedIdentity, prettyPrint(resp.record))
		}

		if !records[recordIdx].succeeded {
			// not a successful rpc so don't need to check anything else
			recordIdx++
			continue
		}

		// verify the l4 bits align, this stuff is only set if auth is successful so do it down here
		localAddr := resp.record.GetSessionInfo().GetLocalAddress()
		if records[recordIdx].localIp != localAddr {
			t.Fatalf("local address not correct, got %q, want %q, Record Details: %s", localAddr, records[recordIdx].localIp, prettyPrint(resp.record))
		}

		localPort := resp.record.GetSessionInfo().GetLocalPort()
		if records[recordIdx].localPort != localPort {
			t.Fatalf("local port not correct, got %d, want %d, Record Details: %s", localPort, records[recordIdx].localPort, prettyPrint(resp.record))
		}

		remoteAddr := resp.record.GetSessionInfo().GetRemoteAddress()
		if records[recordIdx].remoteIp != remoteAddr {
			t.Fatalf("remote address not correct, got %q, want %q, Record Details: %s", remoteAddr, records[recordIdx].remoteIp, prettyPrint(resp.record))
		}

		remotePort := resp.record.GetSessionInfo().GetRemotePort()
		if records[recordIdx].remotePort != remotePort {
			t.Fatalf("remote port not correct, got %d, want %d, Record Details: %s", remotePort, records[recordIdx].remotePort, prettyPrint(resp.record))
		}

		recordIdx++
	}

	if recordIdx != len(records) {
		t.Fatal("did not process all records")
	}
}
