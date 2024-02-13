package record_subscribe_partial

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/gnmi/proto/gnmi"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnoi/system"
	"github.com/openconfig/gnsi/acctz"
	gribi "github.com/openconfig/gribi/v1/proto/service"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/binding/introspect"
	ondatragnmi "github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	p4pb "github.com/p4lang/p4runtime/go/p4/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/anypb"
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
	rpcPayload           []string
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

func createNativeRole(t testing.TB, dut *ondatra.DUTDevice, role string) {
	t.Helper()
	switch dut.Vendor() {
	case ondatra.NOKIA:
		roleData, err := json.Marshal([]any{
			map[string]any{
				"services": []string{"cli", "gnmi", "gnoi", "gribi", "p4rt"},
			},
		})
		if err != nil {
			t.Fatalf("Error with json Marshal: %v", err)
		}

		successUserData, err := json.Marshal([]any{
			map[string]any{
				"password": successPassword,
				"role":     []string{role},
			},
		})
		if err != nil {
			t.Fatalf("Error with json Marshal: %v", err)
		}

		SetRequest := &gpb.SetRequest{
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
							{Name: "role", Key: map[string]string{"rolename": role}},
						},
					},
					Val: &gpb.TypedValue{
						Value: &gpb.TypedValue_JsonIetfVal{
							JsonIetfVal: roleData,
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
				},
			},
		}
		gnmiClient := dut.RawAPIs().GNMI(t)
		if _, err := gnmiClient.Set(context.Background(), SetRequest); err != nil {
			t.Fatalf("Unexpected error configuring User: %v", err)
		}
	default:
		t.Fatalf("Unsupported vendor %s for deviation 'deviation_native_users'", dut.Vendor())
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
		createNativeRole(t, dut, "acctz-fp-test")
	}
}

func dialGRPC(t *testing.T, addr string, port uint32) (*grpc.ClientConn, net.Addr) {
	var addrObj net.Addr

	conn, err := grpc.Dial(
		fmt.Sprintf("%s:%d", addr, port),
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
		t.Fatalf("failed grpc dialing %q, error: %v", addr, err)
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

func sendGNMIRPCs(t *testing.T, addr string, port uint32) []rpcRecord {
	var records []rpcRecord

	grpcConn, addrObj := dialGRPC(t, addr, port)

	gnmiClient := gnmi.NewGNMIClient(grpcConn)

	ctx := context.Background()
	ctx = metadata.AppendToOutgoingContext(ctx, "username", failUsername)
	ctx = metadata.AppendToOutgoingContext(ctx, "password", failPassword)

	startTime := time.Now()

	// send a unsuccessful gnmi capabilities request (bad creds in context)
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

	// remote from the perspective of the router :)
	// assuming that split/atoi will always work since we know we're fatal'ing out of the dial
	// func if something is bad
	addrParts := strings.Split(addrObj.String(), ":")
	remoteAddr := addrParts[0]
	remotePort, _ := strconv.Atoi(addrParts[1])

	records = append(records, rpcRecord{
		startTime: startTime,
		doneTime:  time.Now(),
		rpcType:   acctz.GrpcService_GRPC_SERVICE_TYPE_GNMI,
		rpcPath:   gnmiCapabilitiesPath,
		rpcPayload: []string{
			payload.String(),
		},
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
	})

	return records
}

func sendGNOIRPCs(t *testing.T, addr string, port uint32) []rpcRecord {
	var records []rpcRecord

	grpcConn, addrObj := dialGRPC(t, addr, port)

	gnoiSystemClient := system.NewSystemClient(grpcConn)

	ctx := context.Background()
	ctx = metadata.AppendToOutgoingContext(ctx, "username", failUsername)
	ctx = metadata.AppendToOutgoingContext(ctx, "password", failPassword)

	startTime := time.Now()

	// send a unsuccessful (bad creds) gnoi system time request (bad creds in context), we dont
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

	addrParts := strings.Split(addrObj.String(), ":")
	remoteAddr := addrParts[0]
	remotePort, _ := strconv.Atoi(addrParts[1])

	records = append(records, rpcRecord{
		startTime: startTime,
		doneTime:  time.Now(),
		rpcType:   acctz.GrpcService_GRPC_SERVICE_TYPE_GNOI,
		rpcPath:   gnoiPingPath,
		rpcPayload: []string{
			payload.String(),
		},
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
	})

	return records
}

func sendGRIBIRPCs(t *testing.T, addr string, port uint32) []rpcRecord {
	var records []rpcRecord

	grpcConn, addrObj := dialGRPC(t, addr, port)

	gribiClient := gribi.NewGRIBIClient(grpcConn)

	ctx := context.Background()
	ctx = metadata.AppendToOutgoingContext(ctx, "username", failUsername)
	ctx = metadata.AppendToOutgoingContext(ctx, "password", failPassword)

	startTime := time.Now()

	// send a unsuccessful (bad creds) gribi get request (bad creds in context), we dont
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
		rpcPayload:           nil,
		localIp:              "",
		localPort:            0,
		remoteIp:             addr,
		remotePort:           port,
		succeeded:            false,
		expectedStatus:       acctz.SessionInfo_SESSION_STATUS_OPERATION,
		expectedAuthenType:   acctz.AuthnDetail_AUTHN_TYPE_UNSPECIFIED,
		expectedAuthenStatus: acctz.AuthnDetail_AUTHN_STATUS_UNSPECIFIED,
		expectedIdentity:     failUsername,
	})

	//send a successful gribi getrequest
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

	addrParts := strings.Split(addrObj.String(), ":")
	remoteAddr := addrParts[0]
	remotePort, _ := strconv.Atoi(addrParts[1])

	records = append(records, rpcRecord{
		startTime: startTime,
		doneTime:  time.Now(),
		rpcType:   acctz.GrpcService_GRPC_SERVICE_TYPE_GRIBI,
		rpcPath:   "/gribi.gRIBI/Get",
		rpcPayload: []string{
			payload.String(),
		},
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
	})

	return records
}

func sendP4RTRPCs(t *testing.T, addr string, port uint32) []rpcRecord {
	var records []rpcRecord

	grpcConn, addrObj := dialGRPC(t, addr, port)

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
		rpcPayload:           nil,
		localIp:              "",
		localPort:            0,
		remoteIp:             addr,
		remotePort:           port,
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

	addrParts := strings.Split(addrObj.String(), ":")
	remoteAddr := addrParts[0]
	remotePort, _ := strconv.Atoi(addrParts[1])

	records = append(records, rpcRecord{
		startTime: startTime,
		doneTime:  time.Now(),
		rpcType:   acctz.GrpcService_GRPC_SERVICE_TYPE_P4RT,
		rpcPath:   "/p4.v1.P4Runtime/Capabilities",
		rpcPayload: []string{
			payload.String(),
		},
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
	})

	return records
}

func getServiceTarget(t *testing.T, dut *ondatra.DUTDevice, service introspect.Service) (string, uint32) {
	// this shouldn't happen really, but fallback to dut name for target addr
	defaultAddr := dut.Name()

	var defaultPort uint32

	target := introspect.DUTDialer(t, dut, service).DialTarget

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

	targetParts := strings.Split(target, ":")

	if len(targetParts) == 2 {
		p, err := strconv.Atoi(targetParts[1])
		if err != nil {
			t.Logf("failed parsing port from target, will use default port. target: %s", target)

			return defaultAddr, defaultPort
		}

		return targetParts[0], uint32(p)
	}

	return defaultAddr, defaultPort
}

func TestAccountzRecordSubscribePartial(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	setupUsers(t, dut)

	var records []rpcRecord

	// put enough time between the test starting a nd any prior events so we can easily know where
	// our records start
	time.Sleep(5 * time.Second)

	startTime := time.Now()

	// https://github.com/openconfig/featureprofiles/issues/2637
	// basically, just waiting to see what the "best"/"preferred" way is to get the v4/v6 of the
	// dut -- for now we just use introspection buuuuut, that wont get us v4 and v6 it will just get
	// us whatever is configured in binding, so while the test asks for v4 and v6, we'll just be
	// doing it for whatever we get
	gnmiAddr, gnmiPort := getServiceTarget(t, dut, introspect.GNMI)
	gnoiAddr, gnoiPort := getServiceTarget(t, dut, introspect.GNOI)
	gribiAddr, gribiPort := getServiceTarget(t, dut, introspect.GRIBI)
	p4rtAddr, p4rtPort := getServiceTarget(t, dut, introspect.P4RT)

	newRecords := sendGNMIRPCs(t, gnmiAddr, gnmiPort)
	records = append(records, newRecords...)

	newRecords = sendGNOIRPCs(t, gnoiAddr, gnoiPort)
	records = append(records, newRecords...)

	newRecords = sendGRIBIRPCs(t, gribiAddr, gribiPort)
	records = append(records, newRecords...)

	newRecords = sendP4RTRPCs(t, p4rtAddr, p4rtPort)
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

	firstResponse, err := acctzSubClient.Recv()
	if err != nil {
		t.Fatalf("failed getting zeroith record response")
	}

	// fetch fresh client
	acctzClient = dut.RawAPIs().GNSI(t).Acctz()

	acctzSubClient, err = acctzClient.RecordSubscribe(context.Background())
	if err != nil {
		t.Fatalf("failed getting accountz record subscribe client, error: %s", err)
	}

	err = acctzSubClient.Send(&acctz.RecordRequest{
		Timestamp: firstResponse.Timestamp,
	})
	if err != nil {
		t.Fatalf("failed sending accountz record request, error: %s", err)
	}

	secondResponse, err := acctzSubClient.Recv()
	if err != nil {
		t.Fatalf("failed getting zeroith record response")
	}

	if reflect.DeepEqual(firstResponse, secondResponse) {
		t.Fatalf("accountz server responded with same event on subsequent record request with")
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
			t.Fatal("history is truncated but it shouldnt be")
		}

		if !resp.record.Timestamp.AsTime().After(startTime) {
			// skipping record, was before test start time
			continue
		}

		serviceType := resp.record.GetGrpcService().GetServiceType()

		if serviceType == acctz.GrpcService_GRPC_SERVICE_TYPE_GNSI {
			// not checkin gnsi things since.... were already using gnsi to get these records :)
			continue
		}

		// check that the timestamp for the record is between our start/stop times for our rpc
		timestamp := resp.record.Timestamp.AsTime()

		if timestamp.UnixMilli() == lastTimestampUnixMillis {
			// this ensures that timestamps are actually changing for each record
			t.Fatalf("timestamp is the same as the previous timestamp, this shouldnt be possible!")
		}

		lastTimestampUnixMillis = timestamp.UnixMilli()

		// -2 for a little breathing room since things may not be perfectly synced up time-wise
		if records[recordIdx].startTime.Unix() < timestamp.Unix()-2 {
			t.Fatalf(
				"record timestamp is prior to rpc start time timestamp, rpc start timestamp %d, record timestamp %d",
				records[recordIdx].startTime.Unix(),
				timestamp.Unix()-2,
			)
		}

		// done time (that we recorded when making the rpc) + 2 second for some breathing room
		if records[recordIdx].doneTime.Unix()+2 < timestamp.Unix() {
			t.Fatalf(
				"record timestamp is after rpc start end timestamp, rpc end timestamp %d, record timestamp %d",
				records[recordIdx].doneTime.Unix()+2,
				timestamp.Unix(),
			)
		}

		if records[recordIdx].rpcType != serviceType {
			t.Fatalf("service type not correct, got %q, want %q", serviceType, records[recordIdx].rpcType)
		}

		servicePath := resp.record.GetGrpcService().GetRpcName()
		if records[recordIdx].rpcPath != servicePath {
			t.Fatalf("service path not correct, got %q, want %q", servicePath, records[recordIdx].rpcPath)
		}

		if len(records[recordIdx].rpcPayload) > 0 {
			// it seems like it *could* truncate payloads so that may come up at some point
			// which would obviously make this comparison not work, but for the simple rpcs in
			// this test that probably shouldnt be happening
			servicePayload := resp.record.GetGrpcService().GetPayloads()

			for idx, expected := range records[recordIdx].rpcPayload {
				actual := servicePayload[idx].String()

				if !strings.EqualFold(actual, expected) {
					t.Fatalf("service payloads not correct, got %q, want %q", actual, expected)
				}
			}
		}

		channelID := resp.record.GetSessionInfo().GetChannelId()

		// this channel check maybe should just go away entirely -- see:
		// https://github.com/openconfig/gnsi/issues/98
		// in case of nokia this is being set to the aaa session id just to have some hopefully
		// useful info in this field to identify a "session" (even if it isnt necessarily ssh/grpc
		// directly)
		if !records[recordIdx].succeeded {
			if channelID != "aaa_session_id: 0" {
				t.Fatalf("auth was not successful for this record, but channel id was set, got %q", channelID)
			}
		} else if channelID == "aaa_session_id: 0" {
			t.Fatalf("auth was successful for this record, but channel id was not set, got %q", channelID)
		}

		// tty only set for ssh things
		if serviceType == acctz.GrpcService_GRPC_SERVICE_TYPE_UNSPECIFIED {
			tty := resp.record.GetSessionInfo().GetTty()
			if tty == "" {
				t.Fatal("should have tty allocated but not set")
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
			// not a successful rpc so dont need to check anything else
			recordIdx++

			continue
		}

		t.Log("skipping 'role' check until ondatra and vendors catch up to jan2024 proto update...")
		//role := resp.record.GetSessionInfo().GetUser().GetGroup()
		//if records[recordIdx].expectedRole != role {
		//	t.Fatalf("role not correct, got %q, want %q", role, records[recordIdx].expectedRole)
		//}

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
