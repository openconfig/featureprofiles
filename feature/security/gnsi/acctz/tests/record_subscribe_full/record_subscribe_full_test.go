package record_subscribe_full_test

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/gnsi/credentialz"

	"google.golang.org/protobuf/types/known/anypb"

	"github.com/openconfig/ondatra/binding/introspect"
	ondatragnmi "github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnoi/system"
	"github.com/openconfig/gnsi/acctz"
	gribi "github.com/openconfig/gribi/v1/proto/service"
	"github.com/openconfig/ondatra"
	p4pb "github.com/p4lang/p4runtime/go/p4/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	successUsername      = "acctztestuser"
	successPassword      = "verysecurepassword"
	failUsername         = "bilbo"
	failPassword         = "baggins"
	gnmiCapabilitiesPath = "/gnmi.gNMI/Capabilities"
	gnoiPingPath         = "/gnoi.system.System/Ping"
)

type rpcRecord struct {
	startTime            time.Time
	doneTime             time.Time
	rpcType              acctz.GrpcService_GrpcServiceType
	rpcPath              string
	rpcPayload           string
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

func setupUsers(t *testing.T, dut *ondatra.DUTDevice) {
	auth := &oc.System_Aaa_Authentication{}
	successUser := auth.GetOrCreateUser(successUsername)
	successUser.SetRole(oc.AaaTypes_SYSTEM_DEFINED_ROLES_SYSTEM_ROLE_ADMIN)
	auth.GetOrCreateUser(failUsername)
	ondatragnmi.Update(t, dut, ondatragnmi.OC().System().Aaa().Authentication().Config(), auth)
	setupUserPassword(t, dut, successUsername, successPassword)
}

func dialGRPC(t *testing.T, target string) (*grpc.ClientConn, net.Addr) {
	var addrObj net.Addr

	conn, err := grpc.Dial(
		target,
		grpc.WithTransportCredentials(
			credentials.NewTLS(
				&tls.Config{
					InsecureSkipVerify: true,
				},
			),
		),
		grpc.WithContextDialer(func(ctx context.Context, a string) (net.Conn, error) {
			dst, err := net.ResolveTCPAddr("tcp", a)
			if err != nil {
				return nil, err
			}

			c, err := net.DialTCP("tcp", nil, dst)
			if err != nil {
				return nil, err
			}

			addrObj = c.LocalAddr()

			return c, err
		}))
	if err != nil {
		t.Fatalf("failed grpc dialing %q, error: %v", target, err)
	}

	readyCounter := 0

	for {
		state := conn.GetState()

		if state == connectivity.Ready {
			break
		}

		readyCounter += 1

		if readyCounter >= 10 {
			t.Fatal("grpc connection never reached ready state")
		}

		time.Sleep(time.Second)
	}

	return conn, addrObj
}

func sendGNMIRPCs(t *testing.T, target string) []rpcRecord {
	var records []rpcRecord

	grpcConn, addrObj := dialGRPC(t, target)

	gnmiClient := gnmi.NewGNMIClient(grpcConn)

	ctx := context.Background()
	ctx = metadata.AppendToOutgoingContext(ctx, "username", failUsername)
	ctx = metadata.AppendToOutgoingContext(ctx, "password", failPassword)

	startTime := time.Now()

	// send an unsuccessful gnmi capabilities request (bad creds in context)
	_, err := gnmiClient.Capabilities(ctx, &gnmi.CapabilityRequest{})
	if err != nil {
		t.Logf("got expected error getting capabilities with no creds, error: %s", err)
	} else {
		t.Fatal("did not get expected error fetching capabilities with no creds")
	}

	records = append(records, rpcRecord{
		startTime:            startTime,
		doneTime:             time.Now(),
		rpcType:              acctz.GrpcService_GRPC_SERVICE_TYPE_GNMI,
		rpcPath:              gnmiCapabilitiesPath,
		succeeded:            false,
		expectedStatus:       acctz.SessionInfo_SESSION_STATUS_OPERATION,
		expectedAuthenType:   acctz.AuthnDetail_AUTHN_TYPE_UNSPECIFIED,
		expectedAuthenStatus: acctz.AuthnDetail_AUTHN_STATUS_UNSPECIFIED,
		expectedIdentity:     failUsername,
	})

	// send a successful gnmi capabilities request
	ctx = context.Background()
	ctx = metadata.AppendToOutgoingContext(ctx, "username", successUsername)
	ctx = metadata.AppendToOutgoingContext(ctx, "password", successPassword)

	req := &gnmi.CapabilityRequest{}

	payload, err := anypb.New(req)
	if err != nil {
		t.Fatal("failed creating anypb payload")
	}

	startTime = time.Now()

	_, err = gnmiClient.Capabilities(ctx, req)
	if err != nil {
		t.Fatalf("error fetching capabilities, error: %s", err)
	}

	// remote from the perspective of the router
	remoteAddr, err := net.ResolveTCPAddr("tcp", addrObj.String())
	if err != nil {
		t.Fatalf("failed resolving gnmi remote addr, error: %s", err)
	}
	localAddr, err := net.ResolveTCPAddr("tcp", target)
	if err != nil {
		t.Fatalf("failed resolving gnmi local addr, error: %s", err)
	}

	records = append(records, rpcRecord{
		startTime:            startTime,
		doneTime:             time.Now(),
		rpcType:              acctz.GrpcService_GRPC_SERVICE_TYPE_GNMI,
		rpcPath:              gnmiCapabilitiesPath,
		rpcPayload:           payload.String(),
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

func sendGNOIRPCs(t *testing.T, target string) []rpcRecord {
	var records []rpcRecord

	grpcConn, addrObj := dialGRPC(t, target)

	gnoiSystemClient := system.NewSystemClient(grpcConn)

	ctx := context.Background()
	ctx = metadata.AppendToOutgoingContext(ctx, "username", failUsername)
	ctx = metadata.AppendToOutgoingContext(ctx, "password", failPassword)

	startTime := time.Now()

	// send an unsuccessful (bad creds) gnoi system time request (bad creds in context), we don't
	// care about receiving on it, just want to make the request
	gnoiSystemPingClient, err := gnoiSystemClient.Ping(ctx, &system.PingRequest{
		Destination: "127.0.0.1",
		Count:       1,
	})
	if err != nil {
		t.Fatalf("got unexpected error getting gnoi system time client, error: %s", err)
	}

	_, err = gnoiSystemPingClient.Recv()
	if err != nil {
		t.Logf("got expected error getting gnoi system time with no creds, error: %s", err)
	}

	records = append(records, rpcRecord{
		startTime:            startTime,
		doneTime:             time.Now(),
		rpcType:              acctz.GrpcService_GRPC_SERVICE_TYPE_GNOI,
		rpcPath:              gnoiPingPath,
		succeeded:            false,
		expectedStatus:       acctz.SessionInfo_SESSION_STATUS_OPERATION,
		expectedAuthenType:   acctz.AuthnDetail_AUTHN_TYPE_UNSPECIFIED,
		expectedAuthenStatus: acctz.AuthnDetail_AUTHN_STATUS_UNSPECIFIED,
		expectedIdentity:     failUsername,
	})

	// send a successful gnmi capabilities request
	ctx = context.Background()
	ctx = metadata.AppendToOutgoingContext(ctx, "username", successUsername)
	ctx = metadata.AppendToOutgoingContext(ctx, "password", successPassword)

	req := &system.PingRequest{
		Destination: "127.0.0.1",
		Count:       1,
	}

	payload, err := anypb.New(req)
	if err != nil {
		t.Fatal("failed creating anypb payload")
	}

	startTime = time.Now()

	gnoiSystemPingClient, err = gnoiSystemClient.Ping(ctx, req)
	if err != nil {
		t.Fatalf("error fetching gnoi system time, error: %s", err)
	}

	_, err = gnoiSystemPingClient.Recv()
	if err != nil {
		t.Fatalf("got unexpected error getting gnoi system time, error: %s", err)
	}

	// remote from the perspective of the router
	remoteAddr, err := net.ResolveTCPAddr("tcp", addrObj.String())
	if err != nil {
		t.Fatalf("failed resolving gnoi remote addr, error: %s", err)
	}
	localAddr, err := net.ResolveTCPAddr("tcp", target)
	if err != nil {
		t.Fatalf("failed resolving gnoi local addr, error: %s", err)
	}

	records = append(records, rpcRecord{
		startTime:            startTime,
		doneTime:             time.Now(),
		rpcType:              acctz.GrpcService_GRPC_SERVICE_TYPE_GNOI,
		rpcPath:              gnoiPingPath,
		rpcPayload:           payload.String(),
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

func sendGRIBIRPCs(t *testing.T, target string) []rpcRecord {
	var records []rpcRecord

	grpcConn, addrObj := dialGRPC(t, target)

	gribiClient := gribi.NewGRIBIClient(grpcConn)

	ctx := context.Background()
	ctx = metadata.AppendToOutgoingContext(ctx, "username", failUsername)
	ctx = metadata.AppendToOutgoingContext(ctx, "password", failPassword)

	startTime := time.Now()

	// send an unsuccessful (bad creds) gribi get request (bad creds in context), we don't
	// care about receiving on it, just want to make the request
	gribiGetClient, err := gribiClient.Get(
		ctx,
		&gribi.GetRequest{
			NetworkInstance: &gribi.GetRequest_All{},
			Aft:             gribi.AFTType_IPV4,
		},
	)
	if err != nil {
		t.Fatalf("got unexpected error during gribi get request, error: %s", err)
	}

	_, err = gribiGetClient.Recv()
	if err != nil {
		t.Logf("got expected error during gribi recv request, error: %s", err)
	}

	records = append(records, rpcRecord{
		startTime:            startTime,
		doneTime:             time.Now(),
		rpcType:              acctz.GrpcService_GRPC_SERVICE_TYPE_GRIBI,
		rpcPath:              "/gribi.gRIBI/Get",
		rpcPayload:           "",
		succeeded:            false,
		expectedStatus:       acctz.SessionInfo_SESSION_STATUS_OPERATION,
		expectedAuthenType:   acctz.AuthnDetail_AUTHN_TYPE_UNSPECIFIED,
		expectedAuthenStatus: acctz.AuthnDetail_AUTHN_STATUS_UNSPECIFIED,
		expectedIdentity:     failUsername,
	})

	//send a successful gribi get request
	ctx = context.Background()
	ctx = metadata.AppendToOutgoingContext(ctx, "username", successUsername)
	ctx = metadata.AppendToOutgoingContext(ctx, "password", successPassword)

	req := &gribi.GetRequest{
		NetworkInstance: &gribi.GetRequest_All{},
		Aft:             gribi.AFTType_IPV4,
	}

	payload, err := anypb.New(req)
	if err != nil {
		t.Fatal("failed creating anypb payload")
	}

	startTime = time.Now()

	gribiGetClient, err = gribiClient.Get(ctx, req)
	if err != nil {
		t.Fatalf("got unexpected error during gribi get request, error: %s", err)
	}

	_, err = gribiGetClient.Recv()
	if err != nil {
		if !errors.Is(err, io.EOF) {
			// having no messages we get an EOF (makes sense!) so this is not a failure basically
			t.Fatalf("got unexpected error during gribi recv request, error: %s", err)
		}
	}

	// remote from the perspective of the router
	remoteAddr, err := net.ResolveTCPAddr("tcp", addrObj.String())
	if err != nil {
		t.Fatalf("failed resolving gribi remote addr, error: %s", err)
	}
	localAddr, err := net.ResolveTCPAddr("tcp", target)
	if err != nil {
		t.Fatalf("failed resolving gribi local addr, error: %s", err)
	}

	records = append(records, rpcRecord{
		startTime:            startTime,
		doneTime:             time.Now(),
		rpcType:              acctz.GrpcService_GRPC_SERVICE_TYPE_GRIBI,
		rpcPath:              "/gribi.gRIBI/Get",
		rpcPayload:           payload.String(),
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

func sendP4RTRPCs(t *testing.T, target string) []rpcRecord {
	var records []rpcRecord

	grpcConn, addrObj := dialGRPC(t, target)

	ctx := context.Background()
	ctx = metadata.AppendToOutgoingContext(ctx, "username", failUsername)
	ctx = metadata.AppendToOutgoingContext(ctx, "password", failPassword)

	startTime := time.Now()

	p4rtclient := p4pb.NewP4RuntimeClient(grpcConn)

	_, err := p4rtclient.Capabilities(ctx, &p4pb.CapabilitiesRequest{})
	if err != nil {
		t.Logf("got expected error getting p4rt capabilities with no creds, error: %s", err)
	} else {
		t.Fatal("did not get expected error fetching pr4t capabilities with no creds")
	}

	records = append(records, rpcRecord{
		startTime:            startTime,
		doneTime:             time.Now(),
		rpcType:              acctz.GrpcService_GRPC_SERVICE_TYPE_P4RT,
		rpcPath:              "/p4.v1.P4Runtime/Capabilities",
		rpcPayload:           "",
		succeeded:            false,
		expectedStatus:       acctz.SessionInfo_SESSION_STATUS_OPERATION,
		expectedAuthenType:   acctz.AuthnDetail_AUTHN_TYPE_UNSPECIFIED,
		expectedAuthenStatus: acctz.AuthnDetail_AUTHN_STATUS_UNSPECIFIED,
		expectedIdentity:     failUsername,
	})

	ctx = context.Background()
	ctx = metadata.AppendToOutgoingContext(ctx, "username", successUsername)
	ctx = metadata.AppendToOutgoingContext(ctx, "password", successPassword)

	req := &p4pb.CapabilitiesRequest{}

	payload, err := anypb.New(req)
	if err != nil {
		t.Fatal("failed creating anypb payload")
	}

	startTime = time.Now()

	_, err = p4rtclient.Capabilities(ctx, req)
	if err != nil {
		t.Fatalf("error fetching p4rt capabilities, error: %s", err)
	}

	// remote from the perspective of the router
	remoteAddr, err := net.ResolveTCPAddr("tcp", addrObj.String())
	if err != nil {
		t.Fatalf("failed resolving p4rt remote addr, error: %s", err)
	}
	localAddr, err := net.ResolveTCPAddr("tcp", target)
	if err != nil {
		t.Fatalf("failed resolving p4rt local addr, error: %s", err)
	}

	records = append(records, rpcRecord{
		startTime:            startTime,
		doneTime:             time.Now(),
		rpcType:              acctz.GrpcService_GRPC_SERVICE_TYPE_P4RT,
		rpcPath:              "/p4.v1.P4Runtime/Capabilities",
		rpcPayload:           payload.String(),
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

func getServiceTarget(t *testing.T, dut *ondatra.DUTDevice, service introspect.Service) string {
	// this shouldn't happen really, but fallback to dut name for target addr
	defaultAddr := dut.Name()

	var defaultPort uint32

	switch service {
	case introspect.GNMI:
		defaultPort = 9339
	case introspect.GNOI:
		defaultPort = 9339
	case introspect.GRIBI:
		defaultPort = 9340
	case introspect.P4RT:
		defaultPort = 9559
	}

	target := introspect.DUTDialer(t, dut, service).DialTarget
	_, err := net.ResolveTCPAddr("tcp", target)
	if err != nil {
		t.Logf("failed resolving %s target %s, will use default values", service, target)
		target = fmt.Sprintf("%s:%d", defaultAddr, defaultPort)
	}
	t.Logf("Target for %s service: %s", service, target)
	return target
}

func prettyPrint(i interface{}) string {
	s, _ := json.MarshalIndent(i, "", "\t")
	return string(s)
}

func TestAccountzRecordSubscribeFull(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	setupUsers(t, dut)

	var records []rpcRecord

	// put enough time between the test starting and any prior events so we can easily know where
	// our records start
	time.Sleep(5 * time.Second)

	startTime := time.Now()

	// https://github.com/openconfig/featureprofiles/issues/2637
	// basically, just waiting to see what the "best"/"preferred" way is to get the v4/v6 of the
	// dut -- for now we just use introspection but, that won't get us v4 and v6 it will just get
	// us whatever is configured in binding, so while the test asks for v4 and v6, we'll just be
	// doing it for whatever we get
	gnmiTarget := getServiceTarget(t, dut, introspect.GNMI)
	gnoiTarget := getServiceTarget(t, dut, introspect.GNOI)
	gribiTarget := getServiceTarget(t, dut, introspect.GRIBI)
	p4rtTarget := getServiceTarget(t, dut, introspect.P4RT)

	newRecords := sendGNMIRPCs(t, gnmiTarget)
	records = append(records, newRecords...)

	newRecords = sendGNOIRPCs(t, gnoiTarget)
	records = append(records, newRecords...)

	newRecords = sendGRIBIRPCs(t, gribiTarget)
	records = append(records, newRecords...)

	newRecords = sendP4RTRPCs(t, p4rtTarget)
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

		serviceType := resp.record.GetGrpcService().GetServiceType()

		if serviceType == acctz.GrpcService_GRPC_SERVICE_TYPE_GNSI {
			// not checking gnsi things since.... we're already using gnsi to get these records :)
			continue
		}

		// check that the timestamp for the record is between our start/stop times for our rpc
		timestamp := resp.record.Timestamp.AsTime()

		if timestamp.UnixMilli() == lastTimestampUnixMillis {
			// this ensures that timestamps are actually changing for each record
			t.Fatalf("timestamp is the same as the previous timestamp, this shouldn't be possible!, Record Details: %s", prettyPrint(resp.record))
		}

		lastTimestampUnixMillis = timestamp.UnixMilli()

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

		if records[recordIdx].rpcType != serviceType {
			t.Fatalf("service type not correct, got %q, want %q, Record Details: %s", serviceType, records[recordIdx].rpcType, prettyPrint(resp.record))
		}

		servicePath := resp.record.GetGrpcService().GetRpcName()
		if records[recordIdx].rpcPath != servicePath {
			t.Fatalf("service path not correct, got %q, want %q, Record Details: %s", servicePath, records[recordIdx].rpcPath, prettyPrint(resp.record))
		}

		if records[recordIdx].rpcPayload != "" {
			// it seems like it *could* truncate payloads so that may come up at some point
			// which would obviously make this comparison not work, but for the simple rpcs in
			// this test that probably shouldn't be happening
			gotServicePayload := resp.record.GetGrpcService().GetProtoVal().String()
			wantServicePayload := records[recordIdx].rpcPayload
			if !strings.EqualFold(gotServicePayload, wantServicePayload) {
				t.Fatalf("service payloads not correct, got %q, want %q, Record Details: %s", gotServicePayload, wantServicePayload, prettyPrint(resp.record))
			}
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
		} else if channelID == "aaa_session_id: 0" {
			t.Fatalf("auth was successful for this record, but channel id was not set, got %q, Record Details: %s", channelID, prettyPrint(resp.record))
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
