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

// Package acctz provides helper APIs to simplify writing acctz test cases.
package acctz

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnoi/system"
	acctzpb "github.com/openconfig/gnsi/acctz"
	authzpb "github.com/openconfig/gnsi/authz"
	cpb "github.com/openconfig/gnsi/credentialz"
	gribi "github.com/openconfig/gribi/v1/proto/service"
	tpb "github.com/openconfig/kne/proto/topo"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/binding"
	"github.com/openconfig/ondatra/binding/introspect"
	ondatragnmi "github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	p4pb "github.com/p4lang/p4runtime/go/p4/v1"
	"golang.org/x/crypto/ssh"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/anypb"
)

const (
	successUsername      = "acctztestuser"
	successPassword      = "verysecurepassword"
	failUsername         = "bilbo"
	failPassword         = "baggins"
	failRoleName         = "acctz-fp-test-fail"
	successCliCommand    = "show version"
	failCliCommand       = "show version"
	shellCommand         = "uname -a"
	gnmiCapabilitiesPath = "/gnmi.gNMI/Capabilities"
	gnoiPingPath         = "/gnoi.system.System/Ping"
	gnsiGetPath          = "/gnsi.authz.v1.Authz/Get"
	gribiGetPath         = "/gribi.gRIBI/Get"
	p4rtCapabilitiesPath = "/p4.v1.P4Runtime/Capabilities"
	defaultSSHPort       = 22
	ipProto              = 6
)

var gRPCClientAddr net.Addr

func setupUserPassword(t *testing.T, dut *ondatra.DUTDevice, username, password string) {
	request := &cpb.RotateAccountCredentialsRequest{
		Request: &cpb.RotateAccountCredentialsRequest_Password{
			Password: &cpb.PasswordRequest{
				Accounts: []*cpb.PasswordRequest_Account{
					{
						Account: username,
						Password: &cpb.PasswordRequest_Password{
							Value: &cpb.PasswordRequest_Password_Plaintext{
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
		t.Fatalf("Failed fetching credentialz rotate account credentials client, error: %s", err)
	}
	err = credzRotateClient.Send(request)
	if err != nil {
		t.Fatalf("Failed sending credentialz rotate account credentials request, error: %s", err)
	}
	_, err = credzRotateClient.Recv()
	if err != nil {
		t.Fatalf("Failed receiving credentialz rotate account credentials response, error: %s", err)
	}
	err = credzRotateClient.Send(&cpb.RotateAccountCredentialsRequest{
		Request: &cpb.RotateAccountCredentialsRequest_Finalize{
			Finalize: request.GetFinalize(),
		},
	})
	if err != nil {
		t.Fatalf("Failed sending credentialz rotate account credentials finalize request, error: %s", err)
	}

	// Brief sleep for finalize to get processed.
	time.Sleep(time.Second)
}

func nokiaFailCliRole(t *testing.T) *gnmi.SetRequest {
	failRoleData, err := json.Marshal([]any{
		map[string]any{
			"services": []string{"cli"},
			"cli": map[string][]string{
				"deny-command-list": {failCliCommand},
			},
		},
	})
	if err != nil {
		t.Fatalf("Error with json marshal: %v", err)
	}

	return &gnmi.SetRequest{
		Prefix: &gnmi.Path{
			Origin: "native",
		},
		Replace: []*gnmi.Update{
			{
				Path: &gnmi.Path{
					Elem: []*gnmi.PathElem{
						{Name: "system"},
						{Name: "aaa"},
						{Name: "authorization"},
						{Name: "role", Key: map[string]string{"rolename": failRoleName}},
					},
				},
				Val: &gnmi.TypedValue{
					Value: &gnmi.TypedValue_JsonIetfVal{
						JsonIetfVal: failRoleData,
					},
				},
			},
		},
	}
}

// SetupUsers Setup users for acctz tests and optionally configure cli role for denied commands.
func SetupUsers(t *testing.T, dut *ondatra.DUTDevice, configureFailCliRole bool) {
	auth := &oc.System_Aaa_Authentication{}
	successUser := auth.GetOrCreateUser(successUsername)
	successUser.SetRole(oc.AaaTypes_SYSTEM_DEFINED_ROLES_SYSTEM_ROLE_ADMIN)
	failUser := auth.GetOrCreateUser(failUsername)
	if configureFailCliRole {
		var SetRequest *gnmi.SetRequest

		// Create failure cli role in native.
		switch dut.Vendor() {
		case ondatra.NOKIA:
			SetRequest = nokiaFailCliRole(t)
		}

		gnmiClient := dut.RawAPIs().GNMI(t)
		if _, err := gnmiClient.Set(context.Background(), SetRequest); err != nil {
			t.Fatalf("Unexpected error configuring role: %v", err)
		}

		failUser.SetRole(oc.UnionString(failRoleName))
	}
	ondatragnmi.Update(t, dut, ondatragnmi.OC().System().Aaa().Authentication().Config(), auth)
	setupUserPassword(t, dut, successUsername, successPassword)
	setupUserPassword(t, dut, failUsername, failPassword)
}

func getGrpcTarget(t *testing.T, dut *ondatra.DUTDevice, service introspect.Service) string {
	dialTarget := introspect.DUTDialer(t, dut, service).DialTarget
	resolvedTarget, err := net.ResolveTCPAddr("tcp", dialTarget)
	if err != nil {
		t.Fatalf("Failed resolving %s target %s", service, dialTarget)
	}
	t.Logf("Target for %s service: %s", service, resolvedTarget)
	return resolvedTarget.String()
}

func getSSHTarget(t *testing.T, dut *ondatra.DUTDevice) string {
	var serviceDUT interface {
		Service(string) (*tpb.Service, error)
	}

	var target string
	err := binding.DUTAs(dut.RawAPIs().BindingDUT(), &serviceDUT)
	if err != nil {
		t.Log("DUT does not support `Service` function, will attempt to resolve dut name field.")

		// Suppose ssh could be not 22 in some cases but don't think this is exposed by introspect.
		dialTarget := fmt.Sprintf("%s:%d", dut.Name(), defaultSSHPort)
		resolvedTarget, err := net.ResolveTCPAddr("tcp", dialTarget)
		if err != nil {
			t.Fatalf("Failed resolving ssh target %s", dialTarget)
		}
		target = resolvedTarget.String()
	} else {
		dutSSHService, err := serviceDUT.Service("ssh")
		if err != nil {
			t.Fatal(err)
		}
		target = fmt.Sprintf("%s:%d", dutSSHService.GetOutsideIp(), defaultSSHPort)
	}

	t.Logf("Target for ssh service: %s", target)
	return target
}

func dialGrpc(t *testing.T, target string) *grpc.ClientConn {
	conn, err := grpc.NewClient(
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
			gRPCClientAddr = c.LocalAddr()
			return c, err
		}))
	if err != nil {
		t.Fatalf("Got unexpected error dialing gRPC target %q, error: %v", target, err)
	}

	return conn
}

func dialSSH(t *testing.T, username, password, target string) (*ssh.Client, io.WriteCloser) {
	conn, err := ssh.Dial(
		"tcp",
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
		})
	if err != nil {
		t.Fatalf("Got unexpected error dialing ssh target %s, error: %v", target, err)
	}

	sess, err := conn.NewSession()
	if err != nil {
		t.Fatalf("Failed creating ssh session, error: %s", err)
	}

	w, err := sess.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}

	term := ssh.TerminalModes{
		ssh.ECHO:          0,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}

	err = sess.RequestPty(
		"xterm",
		40,
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

	return conn, w
}

func getHostPortInfo(t *testing.T, address string) (string, uint32) {
	ip, port, err := net.SplitHostPort(address)
	if err != nil {
		t.Fatal(err)
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil {
		t.Fatal(err)
	}
	return ip, uint32(portNumber)
}

// SendGnmiRPCs Setup gNMI test RPCs (successful and failed) to be used in the acctz client tests.
func SendGnmiRPCs(t *testing.T, dut *ondatra.DUTDevice) []*acctzpb.RecordResponse {
	// Per https://github.com/openconfig/featureprofiles/issues/2637, waiting to see what the
	// "best"/"preferred" way is to get the v4/v6 of the dut. For now, we just use introspection
	// but that won't get us v4 and v6, it will just get us whatever is configured in binding,
	// so while the test asks for v4 and v6 we'll just be doing it for whatever we get.
	target := getGrpcTarget(t, dut, introspect.GNMI)

	var records []*acctzpb.RecordResponse
	grpcConn := dialGrpc(t, target)
	gnmiClient := gnmi.NewGNMIClient(grpcConn)
	ctx := context.Background()
	ctx = metadata.AppendToOutgoingContext(ctx, "username", failUsername)
	ctx = metadata.AppendToOutgoingContext(ctx, "password", failPassword)

	// Send an unsuccessful gNMI capabilities request (bad creds in context).
	_, err := gnmiClient.Capabilities(ctx, &gnmi.CapabilityRequest{})
	if err != nil {
		t.Logf("Got expected error fetching capabilities with bad creds, error: %s", err)
	} else {
		t.Fatal("Did not get expected error fetching capabilities with bad creds.")
	}

	records = append(records, &acctzpb.RecordResponse{
		ServiceRequest: &acctzpb.RecordResponse_GrpcService{
			GrpcService: &acctzpb.GrpcService{
				ServiceType: acctzpb.GrpcService_GRPC_SERVICE_TYPE_GNMI,
				RpcName:     gnmiCapabilitiesPath,
				Authz: &acctzpb.AuthzDetail{
					Status: acctzpb.AuthzDetail_AUTHZ_STATUS_DENY,
				},
			},
		},
		SessionInfo: &acctzpb.SessionInfo{
			Status: acctzpb.SessionInfo_SESSION_STATUS_ONCE,
			Authn: &acctzpb.AuthnDetail{
				Type:   acctzpb.AuthnDetail_AUTHN_TYPE_UNSPECIFIED,
				Status: acctzpb.AuthnDetail_AUTHN_STATUS_UNSPECIFIED,
			},
			User: &acctzpb.UserDetail{
				Identity: failUsername,
			},
		},
	})

	// Send a successful gNMI capabilities request.
	ctx = context.Background()
	ctx = metadata.AppendToOutgoingContext(ctx, "username", successUsername)
	ctx = metadata.AppendToOutgoingContext(ctx, "password", successPassword)
	req := &gnmi.CapabilityRequest{}
	payload, err := anypb.New(req)
	if err != nil {
		t.Fatal("Failed creating anypb payload.")
	}
	_, err = gnmiClient.Capabilities(ctx, req)
	if err != nil {
		t.Fatalf("Error fetching capabilities, error: %s", err)
	}

	// Remote from the perspective of the router.
	remoteIP, remotePort := getHostPortInfo(t, gRPCClientAddr.String())
	localIP, localPort := getHostPortInfo(t, target)

	records = append(records, &acctzpb.RecordResponse{
		ServiceRequest: &acctzpb.RecordResponse_GrpcService{
			GrpcService: &acctzpb.GrpcService{
				ServiceType: acctzpb.GrpcService_GRPC_SERVICE_TYPE_GNMI,
				RpcName:     gnmiCapabilitiesPath,
				Payload: &acctzpb.GrpcService_ProtoVal{
					ProtoVal: payload,
				},
				Authz: &acctzpb.AuthzDetail{
					Status: acctzpb.AuthzDetail_AUTHZ_STATUS_PERMIT,
				},
			},
		},
		SessionInfo: &acctzpb.SessionInfo{
			Status:        acctzpb.SessionInfo_SESSION_STATUS_ONCE,
			LocalAddress:  localIP,
			LocalPort:     localPort,
			RemoteAddress: remoteIP,
			RemotePort:    remotePort,
			IpProto:       ipProto,
			Authn: &acctzpb.AuthnDetail{
				Type:   acctzpb.AuthnDetail_AUTHN_TYPE_UNSPECIFIED,
				Status: acctzpb.AuthnDetail_AUTHN_STATUS_SUCCESS,
				Cause:  "authentication_method: local",
			},
			User: &acctzpb.UserDetail{
				Identity: successUsername,
			},
		},
	})

	return records
}

// SendGnoiRPCs Setup gNOI test RPCs (successful and failed) to be used in the acctz client tests.
func SendGnoiRPCs(t *testing.T, dut *ondatra.DUTDevice) []*acctzpb.RecordResponse {
	// Per https://github.com/openconfig/featureprofiles/issues/2637, waiting to see what the
	// "best"/"preferred" way is to get the v4/v6 of the dut. For now, we just use introspection
	// but that won't get us v4 and v6, it will just get us whatever is configured in binding,
	// so while the test asks for v4 and v6 we'll just be doing it for whatever we get.
	target := getGrpcTarget(t, dut, introspect.GNOI)

	var records []*acctzpb.RecordResponse
	grpcConn := dialGrpc(t, target)
	gnoiSystemClient := system.NewSystemClient(grpcConn)
	ctx := context.Background()
	ctx = metadata.AppendToOutgoingContext(ctx, "username", failUsername)
	ctx = metadata.AppendToOutgoingContext(ctx, "password", failPassword)

	// Send an unsuccessful gNOI system time request (bad creds in context), we don't
	// care about receiving on it, just want to make the request.
	gnoiSystemPingClient, err := gnoiSystemClient.Ping(ctx, &system.PingRequest{
		Destination: "127.0.0.1",
		Count:       1,
	})
	if err != nil {
		t.Fatalf("Got unexpected error getting gnoi system time client, error: %s", err)
	}

	_, err = gnoiSystemPingClient.Recv()
	if err != nil {
		t.Logf("Got expected error getting gnoi system time with bad creds, error: %s", err)
	}

	records = append(records, &acctzpb.RecordResponse{
		ServiceRequest: &acctzpb.RecordResponse_GrpcService{
			GrpcService: &acctzpb.GrpcService{
				ServiceType: acctzpb.GrpcService_GRPC_SERVICE_TYPE_GNOI,
				RpcName:     gnoiPingPath,
				Authz: &acctzpb.AuthzDetail{
					Status: acctzpb.AuthzDetail_AUTHZ_STATUS_DENY,
				},
			},
		},
		SessionInfo: &acctzpb.SessionInfo{
			Status: acctzpb.SessionInfo_SESSION_STATUS_ONCE,
			Authn: &acctzpb.AuthnDetail{
				Type:   acctzpb.AuthnDetail_AUTHN_TYPE_UNSPECIFIED,
				Status: acctzpb.AuthnDetail_AUTHN_STATUS_UNSPECIFIED,
			},
			User: &acctzpb.UserDetail{
				Identity: failUsername,
			},
		},
	})

	// Send a successful gNOI ping request.
	ctx = context.Background()
	ctx = metadata.AppendToOutgoingContext(ctx, "username", successUsername)
	ctx = metadata.AppendToOutgoingContext(ctx, "password", successPassword)
	req := &system.PingRequest{
		Destination: "127.0.0.1",
		Count:       1,
	}
	payload, err := anypb.New(req)
	if err != nil {
		t.Fatal("Failed creating anypb payload.")
	}
	gnoiSystemPingClient, err = gnoiSystemClient.Ping(ctx, req)
	if err != nil {
		t.Fatalf("Error fetching gnoi system time, error: %s", err)
	}
	_, err = gnoiSystemPingClient.Recv()
	if err != nil {
		t.Fatalf("Got unexpected error getting gnoi system time, error: %s", err)
	}

	// Remote from the perspective of the router.
	remoteIP, remotePort := getHostPortInfo(t, gRPCClientAddr.String())
	localIP, localPort := getHostPortInfo(t, target)

	records = append(records, &acctzpb.RecordResponse{
		ServiceRequest: &acctzpb.RecordResponse_GrpcService{
			GrpcService: &acctzpb.GrpcService{
				ServiceType: acctzpb.GrpcService_GRPC_SERVICE_TYPE_GNOI,
				RpcName:     gnoiPingPath,
				Payload: &acctzpb.GrpcService_ProtoVal{
					ProtoVal: payload,
				},
				Authz: &acctzpb.AuthzDetail{
					Status: acctzpb.AuthzDetail_AUTHZ_STATUS_PERMIT,
				},
			},
		},
		SessionInfo: &acctzpb.SessionInfo{
			Status:        acctzpb.SessionInfo_SESSION_STATUS_ONCE,
			LocalAddress:  localIP,
			LocalPort:     localPort,
			RemoteAddress: remoteIP,
			RemotePort:    remotePort,
			IpProto:       ipProto,
			Authn: &acctzpb.AuthnDetail{
				Type:   acctzpb.AuthnDetail_AUTHN_TYPE_UNSPECIFIED,
				Status: acctzpb.AuthnDetail_AUTHN_STATUS_SUCCESS,
				Cause:  "authentication_method: local",
			},
			User: &acctzpb.UserDetail{
				Identity: successUsername,
			},
		},
	})

	return records
}

// SendGnsiRPCs Setup gNSI test RPCs (successful and failed) to be used in the acctz client tests.
func SendGnsiRPCs(t *testing.T, dut *ondatra.DUTDevice) []*acctzpb.RecordResponse {
	// Per https://github.com/openconfig/featureprofiles/issues/2637, waiting to see what the
	// "best"/"preferred" way is to get the v4/v6 of the dut. For now, we just use introspection
	// but that won't get us v4 and v6, it will just get us whatever is configured in binding,
	// so while the test asks for v4 and v6 we'll just be doing it for whatever we get.
	target := getGrpcTarget(t, dut, introspect.GNSI)

	var records []*acctzpb.RecordResponse
	grpcConn := dialGrpc(t, target)
	authzClient := authzpb.NewAuthzClient(grpcConn)
	ctx := context.Background()
	ctx = metadata.AppendToOutgoingContext(ctx, "username", failUsername)
	ctx = metadata.AppendToOutgoingContext(ctx, "password", failPassword)

	// Send an unsuccessful gNSI authz get request (bad creds in context), we don't
	// care about receiving on it, just want to make the request.
	_, err := authzClient.Get(ctx, &authzpb.GetRequest{})
	if err != nil {
		t.Logf("Got expected error fetching authz policy with bad creds, error: %s", err)
	} else {
		t.Fatal("Did not get expected error fetching authz policy with bad creds.")
	}

	records = append(records, &acctzpb.RecordResponse{
		ServiceRequest: &acctzpb.RecordResponse_GrpcService{
			GrpcService: &acctzpb.GrpcService{
				ServiceType: acctzpb.GrpcService_GRPC_SERVICE_TYPE_GNSI,
				RpcName:     gnsiGetPath,
				Authz: &acctzpb.AuthzDetail{
					Status: acctzpb.AuthzDetail_AUTHZ_STATUS_DENY,
				},
			},
		},
		SessionInfo: &acctzpb.SessionInfo{
			Status: acctzpb.SessionInfo_SESSION_STATUS_ONCE,
			Authn: &acctzpb.AuthnDetail{
				Type:   acctzpb.AuthnDetail_AUTHN_TYPE_UNSPECIFIED,
				Status: acctzpb.AuthnDetail_AUTHN_STATUS_UNSPECIFIED,
			},
			User: &acctzpb.UserDetail{
				Identity: failUsername,
			},
		},
	})

	// Send a successful gNSI authz get request.
	ctx = context.Background()
	ctx = metadata.AppendToOutgoingContext(ctx, "username", successUsername)
	ctx = metadata.AppendToOutgoingContext(ctx, "password", successPassword)
	req := &authzpb.GetRequest{}
	payload, err := anypb.New(req)
	if err != nil {
		t.Fatal("Failed creating anypb payload.")
	}
	_, err = authzClient.Get(ctx, &authzpb.GetRequest{})
	if err != nil {
		t.Fatalf("Error fetching authz policy, error: %s", err)
	}

	// Remote from the perspective of the router.
	remoteIP, remotePort := getHostPortInfo(t, gRPCClientAddr.String())
	localIP, localPort := getHostPortInfo(t, target)

	records = append(records, &acctzpb.RecordResponse{
		ServiceRequest: &acctzpb.RecordResponse_GrpcService{
			GrpcService: &acctzpb.GrpcService{
				ServiceType: acctzpb.GrpcService_GRPC_SERVICE_TYPE_GNSI,
				RpcName:     gnsiGetPath,
				Payload: &acctzpb.GrpcService_ProtoVal{
					ProtoVal: payload,
				},
				Authz: &acctzpb.AuthzDetail{
					Status: acctzpb.AuthzDetail_AUTHZ_STATUS_PERMIT,
				},
			},
		},
		SessionInfo: &acctzpb.SessionInfo{
			Status:        acctzpb.SessionInfo_SESSION_STATUS_ONCE,
			LocalAddress:  localIP,
			LocalPort:     localPort,
			RemoteAddress: remoteIP,
			RemotePort:    remotePort,
			IpProto:       ipProto,
			Authn: &acctzpb.AuthnDetail{
				Type:   acctzpb.AuthnDetail_AUTHN_TYPE_UNSPECIFIED,
				Status: acctzpb.AuthnDetail_AUTHN_STATUS_SUCCESS,
				Cause:  "authentication_method: local",
			},
			User: &acctzpb.UserDetail{
				Identity: successUsername,
			},
		},
	})

	return records
}

// SendGribiRPCs Setup gRIBI test RPCs (successful and failed) to be used in the acctz client tests.
func SendGribiRPCs(t *testing.T, dut *ondatra.DUTDevice) []*acctzpb.RecordResponse {
	// Per https://github.com/openconfig/featureprofiles/issues/2637, waiting to see what the
	// "best"/"preferred" way is to get the v4/v6 of the dut. For now, we just use introspection
	// but that won't get us v4 and v6, it will just get us whatever is configured in binding,
	// so while the test asks for v4 and v6 we'll just be doing it for whatever we get.
	target := getGrpcTarget(t, dut, introspect.GRIBI)

	var records []*acctzpb.RecordResponse
	grpcConn := dialGrpc(t, target)
	gribiClient := gribi.NewGRIBIClient(grpcConn)
	ctx := context.Background()
	ctx = metadata.AppendToOutgoingContext(ctx, "username", failUsername)
	ctx = metadata.AppendToOutgoingContext(ctx, "password", failPassword)

	// Send an unsuccessful gRIBI get request (bad creds in context), we don't
	// care about receiving on it, just want to make the request.
	gribiGetClient, err := gribiClient.Get(
		ctx,
		&gribi.GetRequest{
			NetworkInstance: &gribi.GetRequest_All{},
			Aft:             gribi.AFTType_IPV4,
		},
	)
	if err != nil {
		t.Fatalf("Got unexpected error during gribi get request, error: %s", err)
	}
	_, err = gribiGetClient.Recv()
	if err != nil {
		t.Logf("Got expected error during gribi recv request, error: %s", err)
	}

	records = append(records, &acctzpb.RecordResponse{
		ServiceRequest: &acctzpb.RecordResponse_GrpcService{
			GrpcService: &acctzpb.GrpcService{
				ServiceType: acctzpb.GrpcService_GRPC_SERVICE_TYPE_GRIBI,
				RpcName:     gribiGetPath,
				Authz: &acctzpb.AuthzDetail{
					Status: acctzpb.AuthzDetail_AUTHZ_STATUS_DENY,
				},
			},
		},
		SessionInfo: &acctzpb.SessionInfo{
			Status: acctzpb.SessionInfo_SESSION_STATUS_ONCE,
			Authn: &acctzpb.AuthnDetail{
				Type:   acctzpb.AuthnDetail_AUTHN_TYPE_UNSPECIFIED,
				Status: acctzpb.AuthnDetail_AUTHN_STATUS_UNSPECIFIED,
			},
			User: &acctzpb.UserDetail{
				Identity: failUsername,
			},
		},
	})

	// Send a successful gRIBI get request.
	ctx = context.Background()
	ctx = metadata.AppendToOutgoingContext(ctx, "username", successUsername)
	ctx = metadata.AppendToOutgoingContext(ctx, "password", successPassword)
	req := &gribi.GetRequest{
		NetworkInstance: &gribi.GetRequest_All{},
		Aft:             gribi.AFTType_IPV4,
	}
	payload, err := anypb.New(req)
	if err != nil {
		t.Fatal("Failed creating anypb payload.")
	}
	gribiGetClient, err = gribiClient.Get(ctx, req)
	if err != nil {
		t.Fatalf("Got unexpected error during gribi get request, error: %s", err)
	}
	_, err = gribiGetClient.Recv()
	if err != nil {
		// Having no messages, we get an EOF so this is not a failure.
		if !errors.Is(err, io.EOF) {
			t.Fatalf("Got unexpected error during gribi recv request, error: %s", err)
		}
	}

	// Remote from the perspective of the router.
	remoteIP, remotePort := getHostPortInfo(t, gRPCClientAddr.String())
	localIP, localPort := getHostPortInfo(t, target)

	records = append(records, &acctzpb.RecordResponse{
		ServiceRequest: &acctzpb.RecordResponse_GrpcService{
			GrpcService: &acctzpb.GrpcService{
				ServiceType: acctzpb.GrpcService_GRPC_SERVICE_TYPE_GRIBI,
				RpcName:     gribiGetPath,
				Payload: &acctzpb.GrpcService_ProtoVal{
					ProtoVal: payload,
				},
				Authz: &acctzpb.AuthzDetail{
					Status: acctzpb.AuthzDetail_AUTHZ_STATUS_PERMIT,
				},
			},
		},
		SessionInfo: &acctzpb.SessionInfo{
			Status:        acctzpb.SessionInfo_SESSION_STATUS_ONCE,
			LocalAddress:  localIP,
			LocalPort:     localPort,
			RemoteAddress: remoteIP,
			RemotePort:    remotePort,
			IpProto:       ipProto,
			Authn: &acctzpb.AuthnDetail{
				Type:   acctzpb.AuthnDetail_AUTHN_TYPE_UNSPECIFIED,
				Status: acctzpb.AuthnDetail_AUTHN_STATUS_SUCCESS,
				Cause:  "authentication_method: local",
			},
			User: &acctzpb.UserDetail{
				Identity: successUsername,
			},
		},
	})

	return records
}

// SendP4rtRPCs Setup P4RT test RPCs (successful and failed) to be used in the acctz client tests.
func SendP4rtRPCs(t *testing.T, dut *ondatra.DUTDevice) []*acctzpb.RecordResponse {
	// Per https://github.com/openconfig/featureprofiles/issues/2637, waiting to see what the
	// "best"/"preferred" way is to get the v4/v6 of the dut. For now, we just use introspection
	// but that won't get us v4 and v6, it will just get us whatever is configured in binding,
	// so while the test asks for v4 and v6 we'll just be doing it for whatever we get.
	target := getGrpcTarget(t, dut, introspect.P4RT)

	var records []*acctzpb.RecordResponse
	grpcConn := dialGrpc(t, target)
	ctx := context.Background()
	ctx = metadata.AppendToOutgoingContext(ctx, "username", failUsername)
	ctx = metadata.AppendToOutgoingContext(ctx, "password", failPassword)
	p4rtclient := p4pb.NewP4RuntimeClient(grpcConn)
	_, err := p4rtclient.Capabilities(ctx, &p4pb.CapabilitiesRequest{})
	if err != nil {
		t.Logf("Got expected error getting p4rt capabilities with no creds, error: %s", err)
	} else {
		t.Fatal("Did not get expected error fetching pr4t capabilities with no creds.")
	}

	records = append(records, &acctzpb.RecordResponse{
		ServiceRequest: &acctzpb.RecordResponse_GrpcService{
			GrpcService: &acctzpb.GrpcService{
				ServiceType: acctzpb.GrpcService_GRPC_SERVICE_TYPE_P4RT,
				RpcName:     p4rtCapabilitiesPath,
				Authz: &acctzpb.AuthzDetail{
					Status: acctzpb.AuthzDetail_AUTHZ_STATUS_DENY,
				},
			},
		},
		SessionInfo: &acctzpb.SessionInfo{
			Status: acctzpb.SessionInfo_SESSION_STATUS_ONCE,
			Authn: &acctzpb.AuthnDetail{
				Type:   acctzpb.AuthnDetail_AUTHN_TYPE_UNSPECIFIED,
				Status: acctzpb.AuthnDetail_AUTHN_STATUS_UNSPECIFIED,
			},
			User: &acctzpb.UserDetail{
				Identity: failUsername,
			},
		},
	})

	ctx = context.Background()
	ctx = metadata.AppendToOutgoingContext(ctx, "username", successUsername)
	ctx = metadata.AppendToOutgoingContext(ctx, "password", successPassword)
	req := &p4pb.CapabilitiesRequest{}
	payload, err := anypb.New(req)
	if err != nil {
		t.Fatal("Failed creating anypb payload.")
	}
	_, err = p4rtclient.Capabilities(ctx, req)
	if err != nil {
		t.Fatalf("Error fetching p4rt capabilities, error: %s", err)
	}

	// Remote from the perspective of the router.
	remoteIP, remotePort := getHostPortInfo(t, gRPCClientAddr.String())
	localIP, localPort := getHostPortInfo(t, target)

	records = append(records, &acctzpb.RecordResponse{
		ServiceRequest: &acctzpb.RecordResponse_GrpcService{
			GrpcService: &acctzpb.GrpcService{
				ServiceType: acctzpb.GrpcService_GRPC_SERVICE_TYPE_P4RT,
				RpcName:     p4rtCapabilitiesPath,
				Payload: &acctzpb.GrpcService_ProtoVal{
					ProtoVal: payload,
				},
				Authz: &acctzpb.AuthzDetail{
					Status: acctzpb.AuthzDetail_AUTHZ_STATUS_PERMIT,
				},
			},
		},
		SessionInfo: &acctzpb.SessionInfo{
			Status:        acctzpb.SessionInfo_SESSION_STATUS_ONCE,
			LocalAddress:  localIP,
			LocalPort:     localPort,
			RemoteAddress: remoteIP,
			RemotePort:    remotePort,
			IpProto:       ipProto,
			Authn: &acctzpb.AuthnDetail{
				Type:   acctzpb.AuthnDetail_AUTHN_TYPE_UNSPECIFIED,
				Status: acctzpb.AuthnDetail_AUTHN_STATUS_SUCCESS,
				Cause:  "authentication_method: local",
			},
			User: &acctzpb.UserDetail{
				Identity: successUsername,
			},
		},
	})

	return records
}

// SendSuccessCliCommand Setup test CLI command (successful) to be used in the acctz client tests.
func SendSuccessCliCommand(t *testing.T, dut *ondatra.DUTDevice) []*acctzpb.RecordResponse {
	// Per https://github.com/openconfig/featureprofiles/issues/2637, waiting to see what the
	// "best"/"preferred" way is to get the v4/v6 of the dut. For now, we use this workaround
	// because ssh isn't exposed in introspection.
	target := getSSHTarget(t, dut)

	var records []*acctzpb.RecordResponse

	sshConn, w := dialSSH(t, successUsername, successPassword, target)
	defer func() {
		// Give things a second to percolate then close the connection.
		time.Sleep(3 * time.Second)
		err := sshConn.Close()
		if err != nil {
			t.Logf("Error closing tcp(ssh) connection, will ignore, error: %s", err)
		}
	}()

	_, err := w.Write([]byte(fmt.Sprintf("%s\n", successCliCommand)))
	if err != nil {
		t.Fatalf("Failed sending cli command, error: %s", err)
	}

	// Remote from the perspective of the router.
	remoteIP, remotePort := getHostPortInfo(t, sshConn.LocalAddr().String())
	localIP, localPort := getHostPortInfo(t, target)

	records = append(records, &acctzpb.RecordResponse{
		ServiceRequest: &acctzpb.RecordResponse_CmdService{
			CmdService: &acctzpb.CommandService{
				ServiceType: acctzpb.CommandService_CMD_SERVICE_TYPE_CLI,
				Cmd:         successCliCommand,
				Authz: &acctzpb.AuthzDetail{
					Status: acctzpb.AuthzDetail_AUTHZ_STATUS_PERMIT,
				},
			},
		},
		SessionInfo: &acctzpb.SessionInfo{
			Status:        acctzpb.SessionInfo_SESSION_STATUS_OPERATION,
			LocalAddress:  localIP,
			LocalPort:     localPort,
			RemoteAddress: remoteIP,
			RemotePort:    remotePort,
			IpProto:       ipProto,
			Authn: &acctzpb.AuthnDetail{
				Type:   acctzpb.AuthnDetail_AUTHN_TYPE_UNSPECIFIED,
				Status: acctzpb.AuthnDetail_AUTHN_STATUS_SUCCESS,
				Cause:  "authentication_method: local",
			},
			User: &acctzpb.UserDetail{
				Identity: successUsername,
			},
		},
	})

	return records
}

// SendFailCliCommand Setup test CLI command (failed) to be used in the acctz client tests.
func SendFailCliCommand(t *testing.T, dut *ondatra.DUTDevice) []*acctzpb.RecordResponse {
	// Per https://github.com/openconfig/featureprofiles/issues/2637, waiting to see what the
	// "best"/"preferred" way is to get the v4/v6 of the dut. For now, we use this workaround
	// because ssh isn't exposed in introspection.
	target := getSSHTarget(t, dut)

	var records []*acctzpb.RecordResponse
	sshConn, w := dialSSH(t, failUsername, failPassword, target)

	defer func() {
		// Give things a second to percolate then close the connection.
		time.Sleep(3 * time.Second)
		err := sshConn.Close()
		if err != nil {
			t.Logf("Error closing tcp(ssh) connection, will ignore, error: %s", err)
		}
	}()

	_, err := w.Write([]byte(fmt.Sprintf("%s\n", failCliCommand)))
	if err != nil {
		t.Fatalf("Failed sending cli command, error: %s", err)
	}

	// Remote from the perspective of the router.
	remoteIP, remotePort := getHostPortInfo(t, sshConn.LocalAddr().String())
	localIP, localPort := getHostPortInfo(t, target)

	records = append(records, &acctzpb.RecordResponse{
		ServiceRequest: &acctzpb.RecordResponse_CmdService{
			CmdService: &acctzpb.CommandService{
				ServiceType: acctzpb.CommandService_CMD_SERVICE_TYPE_CLI,
				Cmd:         failCliCommand,
				Authz: &acctzpb.AuthzDetail{
					Status: acctzpb.AuthzDetail_AUTHZ_STATUS_DENY,
				},
			},
		},
		SessionInfo: &acctzpb.SessionInfo{
			Status:        acctzpb.SessionInfo_SESSION_STATUS_OPERATION,
			LocalAddress:  localIP,
			LocalPort:     localPort,
			RemoteAddress: remoteIP,
			RemotePort:    remotePort,
			IpProto:       ipProto,
			Authn: &acctzpb.AuthnDetail{
				Type:   acctzpb.AuthnDetail_AUTHN_TYPE_UNSPECIFIED,
				Status: acctzpb.AuthnDetail_AUTHN_STATUS_SUCCESS,
				Cause:  "authentication_method: local",
			},
			User: &acctzpb.UserDetail{
				Identity: failUsername,
				Role:     failRoleName,
			},
		},
	})

	return records
}

// SendShellCommand Setup test shell command (successful) to be used in the acctz client tests.
func SendShellCommand(t *testing.T, dut *ondatra.DUTDevice) []*acctzpb.RecordResponse {
	// Per https://github.com/openconfig/featureprofiles/issues/2637, waiting to see what the
	// "best"/"preferred" way is to get the v4/v6 of the dut. For now, we use this workaround
	// because ssh isn't exposed in introspection.
	target := getSSHTarget(t, dut)

	var records []*acctzpb.RecordResponse
	shellUsername := successUsername
	shellPassword := successPassword

	switch dut.Vendor() {
	case ondatra.NOKIA:
		// Assuming linuxadmin is present and ssh'ing directly via this user gets us to shell
		// straight away so this is easy button to trigger a shell record.
		shellUsername = "linuxadmin"
		shellPassword = "NokiaSrl1!"
	}

	sshConn, w := dialSSH(t, shellUsername, shellPassword, target)
	defer func() {
		// Give things a second to percolate then close the connection.
		time.Sleep(3 * time.Second)
		err := sshConn.Close()
		if err != nil {
			t.Logf("Error closing tcp(ssh) connection, will ignore, error: %s", err)
		}
	}()

	// This might not work for other vendors, so probably we can have a switch here and pass
	// the writer to func per vendor if needed.
	_, err := w.Write([]byte(fmt.Sprintf("%s\n", shellCommand)))
	if err != nil {
		t.Fatalf("Failed sending cli command, error: %s", err)
	}

	// Remote from the perspective of the router.
	remoteIP, remotePort := getHostPortInfo(t, sshConn.LocalAddr().String())
	localIP, localPort := getHostPortInfo(t, target)

	records = append(records, &acctzpb.RecordResponse{
		ServiceRequest: &acctzpb.RecordResponse_CmdService{
			CmdService: &acctzpb.CommandService{
				ServiceType: acctzpb.CommandService_CMD_SERVICE_TYPE_SHELL,
				Cmd:         shellCommand,
				Authz: &acctzpb.AuthzDetail{
					Status: acctzpb.AuthzDetail_AUTHZ_STATUS_PERMIT,
				},
			},
		},
		SessionInfo: &acctzpb.SessionInfo{
			Status:        acctzpb.SessionInfo_SESSION_STATUS_OPERATION,
			LocalAddress:  localIP,
			LocalPort:     localPort,
			RemoteAddress: remoteIP,
			RemotePort:    remotePort,
			IpProto:       ipProto,
			Authn: &acctzpb.AuthnDetail{
				Type:   acctzpb.AuthnDetail_AUTHN_TYPE_UNSPECIFIED,
				Status: acctzpb.AuthnDetail_AUTHN_STATUS_UNSPECIFIED,
			},
			User: &acctzpb.UserDetail{
				Identity: shellUsername,
			},
		},
	})

	return records
}
