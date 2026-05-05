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
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/args"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/helpers"
	bindpb "github.com/openconfig/featureprofiles/topologies/proto/binding"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	systempb "github.com/openconfig/gnoi/system"
	acctzpb "github.com/openconfig/gnsi/acctz"
	authzpb "github.com/openconfig/gnsi/authz"
	cpb "github.com/openconfig/gnsi/credentialz"
	gribi "github.com/openconfig/gribi/v1/proto/service"
	tpb "github.com/openconfig/kne/proto/topo"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/binding"
	ondatragnmi "github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
	p4pb "github.com/p4lang/p4runtime/go/p4/v1"
	"golang.org/x/crypto/ssh"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/types/known/anypb"
)

const (
	// SuccessUsername is the username for a successful test.
	SuccessUsername = "acctztestuser"
	successPassword = "verysecurepasswordTest123!"
	// FailUsername is the username for a failed test.
	FailUsername = "bilbo"
	// FailAuthenticateUsername is the username for failed authentication.
	FailAuthenticateUsername = "bilbo"
	failAuthenticatePassword = "bagginsTest123!"
	failAuthorizeUsername    = "failauthuser" // username for failed authorization
	FailAuthorizeUsername    = failAuthorizeUsername
	failAuthorizePassword    = "failauthpasswordTest123!"
	failRoleName             = "acctz-fp-test-fail" // role for failed authorization
	failDenyRoleName         = "acctz-fp-deny-fail" // role for failed deny authorization
	successCliCommand        = "show version"
	failCliCommand           = "show version"
	failDenyCliCommand       = "/.*"
	shellCommand             = "uname -a"
	gnmiCapabilitiesPath     = "/gnmi.gNMI/Capabilities"
	gnoiPingPath             = "/gnoi.system.System/Ping"
	gnoiTimePath             = "/gnoi.system.System/Time"
	gnsiGetPath              = "/gnsi.authz.v1.Authz/Get"
	gribiGetPath             = "/gribi.gRIBI/Get"
	p4rtCapabilitiesPath     = "/p4.v1.P4Runtime/Capabilities"
	defaultSSHPort           = 22
	ipProto                  = 6
)

var (
	failuser     string
	failpass     string
	failPassword = "baggins"
	// TestPaths is the list of paths to be tested for acctz.
	TestPaths = []string{gnmiCapabilitiesPath, gnoiPingPath, gnoiTimePath, gnsiGetPath, gribiGetPath, p4rtCapabilitiesPath}
)

// var gRPCClientAddr net.Addr
func setupUserPassword(t *testing.T, dut *ondatra.DUTDevice, username, password string) {
	passwordversion := fmt.Sprintf("v%d", time.Now().UnixNano())
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
						Version:   passwordversion,
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

func nokiaCreateTestGrpcServer(t *testing.T) *gnmipb.SetRequest {
	grpcServerData, err := json.Marshal(map[string]any{
		"system": map[string]any{
			"srl_nokia-grpc:grpc-server": []map[string]any{
				{
					"name":                    "mgmtVrf1",
					"admin-state":             "enable",
					"session-limit":           1024,
					"metadata-authentication": true,
					"yang-models":             "openconfig",
					"tls-profile":             "self-signed-certs-profile",
					"network-instance":        "mgmtVrf",
					"port":                    10162,
					"services":                []string{"gnmi", "gnoi", "gnsi"},
					"gnmi": map[string]any{
						"commit-save": true,
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Error with json marshal: %v", err)
	}

	return &gnmipb.SetRequest{
		Prefix: &gnmipb.Path{
			Origin: "native",
		},
		Update: []*gnmipb.Update{
			{
				Path: &gnmipb.Path{},
				Val: &gnmipb.TypedValue{
					Value: &gnmipb.TypedValue_JsonIetfVal{
						JsonIetfVal: grpcServerData,
					},
				},
			},
		},
	}
}

func nokiaFailCliRole(t *testing.T) *gnmipb.SetRequest {
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

	return &gnmipb.SetRequest{
		Prefix: &gnmipb.Path{
			Origin: "native",
		},
		Replace: []*gnmipb.Update{
			{
				Path: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{
						{Name: "system"},
						{Name: "aaa"},
						{Name: "authorization"},
						{Name: "role", Key: map[string]string{"rolename": failRoleName}},
					},
				},
				Val: &gnmipb.TypedValue{
					Value: &gnmipb.TypedValue_JsonIetfVal{
						JsonIetfVal: failRoleData,
					},
				},
			},
		},
	}
}

func juniperSetup(t *testing.T, dut *ondatra.DUTDevice, configureFailCliRole bool) {
	t.Logf("Juniper vendor, performing CLI configuration for users and roles")
	var userConfig string
	if configureFailCliRole {
		userConfig = fmt.Sprintf(`
                                        class %s {
                                                permissions [ view ];
                                                deny-commands "%s";
                                        }
                                        user %s {
                                                class %s;
                                        }
                        `, failRoleName, failCliCommand, FailUsername, failRoleName)
	}
	if !configureFailCliRole {
		userConfig = fmt.Sprintf(`
                                        class %s {
                                                deny-grpc-rpc-regexps "%s";
                                        }
                                        user %s {
                                                class %s;
                                        }
                        `, failDenyRoleName, failDenyCliCommand, FailUsername, failDenyRoleName)
	}
	config := fmt.Sprintf(`
                        system {
                                services {
                                        ssh {
                                                root-login allow;
                                        }
                                }
                                authentication-order password;
                                login {
                                        user %s {
                                                class super-user;
                                        }
                                        %s
                                }
                        }
                `, SuccessUsername, userConfig)
	helpers.GnmiCLIConfig(t, dut, config)
	ondatragnmi.Replace(t, dut, ondatragnmi.OC().System().Aaa().Authentication().
		User(FailUsername).Config(), &oc.System_Aaa_Authentication_User{
		Username: ygot.String(FailUsername),
		Password: &failPassword,
	})
	t.Logf("config on device: %s\nconfig: %s", dut.Name(), config)
	time.Sleep(60 * time.Second)
	config = `
		interfaces {
			lo0 {
			    unit 0 {
			        family inet {
			            address 127.0.0.1/32;
			        }
			    }
			}
		}
                `
	helpers.GnmiCLIConfig(t, dut, config)
	t.Logf("Loopback Configuration Completed.")
}

func aristaFailAuthzCliRole(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	// Configure a role that denies Authorization for rpcs.
	commands := []string{
		"configure",
		fmt.Sprintf("role %s", failRoleName),
		"   10 deny command .*",
		fmt.Sprintf("username %s privilege 15 role network-admin secret %s", SuccessUsername, successPassword),
		fmt.Sprintf("username %s privilege 15 role acctz-fp-test-fail secret %s", FailUsername, failPassword),
		fmt.Sprintf("username %s privilege 15 role acctz-fp-test-fail secret %s", failAuthorizeUsername, failAuthorizePassword),
		"aaa authentication login default local",
		"aaa authorization exec default local",
		"aaa authorization commands all default local",
		"management ssh",
		"   authentication protocol password",
		"management api gnmi",
		"   transport grpc default",
		"      authorization requests",
		"   transport grpc mgmt",
		"      authorization requests",
	}
	helpers.GnmiCLIConfig(t, dut, strings.Join(commands, "\n"))
}

func nokiaGrpcMetadataAuth(t *testing.T) []*gnmipb.Update {
	var updates []*gnmipb.Update
	for _, name := range []string{"mgmtVrf-gribi", "mgmtVrf-p4rt"} {
		updates = append(updates, &gnmipb.Update{
			Path: &gnmipb.Path{
				Elem: []*gnmipb.PathElem{
					{Name: "system"},
					{Name: "grpc-server", Key: map[string]string{"name": name}},
					{Name: "metadata-authentication"},
				},
			},
			Val: &gnmipb.TypedValue{
				Value: &gnmipb.TypedValue_BoolVal{
					BoolVal: true,
				},
			},
		})
	}
	return updates
}

// SetupUsers Setup users for acctz tests and optionally configure cli role for denied commands.
func SetupUsers(t *testing.T, dut *ondatra.DUTDevice, configureFailCliRole bool) {
	if dut.Vendor() == ondatra.JUNIPER {
		juniperSetup(t, dut, configureFailCliRole)
		setupUserPassword(t, dut, SuccessUsername, successPassword)
	} else {
		auth := &oc.System_Aaa_Authentication{}
		successUser := auth.GetOrCreateUser(SuccessUsername)
		successUser.SetRole(oc.AaaTypes_SYSTEM_DEFINED_ROLES_SYSTEM_ROLE_ADMIN)
		failAuthenticateUser := auth.GetOrCreateUser(FailAuthenticateUsername)
		failAuthenticateUser.SetRole(oc.AaaTypes_SYSTEM_DEFINED_ROLES_SYSTEM_ROLE_ADMIN)
		failAuthorizeUser := auth.GetOrCreateUser(failAuthorizeUsername)
		if configureFailCliRole {
			var SetRequest *gnmipb.SetRequest

			// Create failure cli role in native.
			switch dut.Vendor() {
			case ondatra.NOKIA:
				SetRequest = nokiaFailCliRole(t)
			case ondatra.ARISTA:
				aristaFailAuthzCliRole(t, dut)
			}
			// _, policyBefore := authz.Get(t, dut)
			// t.Logf("Authz Policy of the Device %s before the Rotate Trigger is %s", dut.Name(), policyBefore.PrettyPrint(t))
			// defer policyBefore.Rotate(t, dut, uint64(time.Now().Unix()), fmt.Sprintf("v0.%v", (time.Now().UnixNano())), false)
			// newpolicy := &authz.AuthorizationPolicy{
			// 	Name:       policyBefore.Name,
			// 	DenyRules:  policyBefore.DenyRules,
			// 	AllowRules: policyBefore.AllowRules,
			// }
			// newpolicy.AddDenyRules(failRoleName, []string{FailUsername}, []*gnxi.RPC{gnxi.RPCs.AllRPC})
			// newpolicy.Rotate(t, dut, uint64(time.Now().Unix()), fmt.Sprintf("v0.%v", (time.Now().UnixNano())), true)

			gnmiClient := dut.RawAPIs().GNMI(t)
			if _, err := gnmiClient.Set(context.Background(), SetRequest); err != nil {
				t.Fatalf("Unexpected error configuring role: %v", err)
			}

			if !deviations.OcAaaUserRoleLeafStringTypeUnsupported(dut) {
				failAuthorizeUser.SetRole(oc.UnionString(failRoleName))
			}
		}
		ondatragnmi.Update(t, dut, ondatragnmi.OC().System().Aaa().Authentication().Config(), auth)
		// Create separate gRPC server for testing on Nokia AFTER users are created.
		if dut.Vendor() == ondatra.NOKIA {
			gnmiClient := dut.RawAPIs().GNMI(t)
			if _, err := gnmiClient.Set(context.Background(), nokiaCreateTestGrpcServer(t)); err != nil {
				t.Fatalf("Unexpected error creating test grpc server: %v", err)
			}
			metadataReq := &gnmipb.SetRequest{
				Prefix: &gnmipb.Path{Origin: "srl_nokia"},
				Update: nokiaGrpcMetadataAuth(t),
			}
			if _, err := gnmiClient.Set(context.Background(), metadataReq); err != nil {
				t.Fatalf("Unexpected error configuring nokia metadata auth: %v", err)
			}
		}
		setupUserPassword(t, dut, SuccessUsername, successPassword)
		setupUserPassword(t, dut, FailAuthenticateUsername, failPassword)
		if configureFailCliRole {
			setupUserPassword(t, dut, failAuthorizeUsername, failAuthorizePassword)
		}
		// Configuring as taskgroup which implicit denies all commands except show interface , which is attached to the failRoleName and then failUsername.
		if deviations.OcAaaUserRoleLeafStringTypeUnsupported(dut) && configureFailCliRole {
			taskUserGroupCLI := fmt.Sprintf("taskgroup %v \n task read interface \n task execute interface \n usergroup %v taskgroup %v \n username %v \n group %v \n no group root-lr \n no group cisco-support", failRoleName, failRoleName, failRoleName, failAuthorizeUsername, failRoleName)
			helpers.GnmiCLIConfig(t, dut, taskUserGroupCLI)
		}
	}
}

// AcctzStreamClient is a local interface for the AcctzStream gRPC client.
type AcctzStreamClient interface {
	RecordSubscribe(ctx context.Context, in *acctzpb.RecordRequest, opts ...grpc.CallOption) (AcctzStream_RecordSubscribeClient, error)
}

// AcctzStream_RecordSubscribeClient is a local interface for the RecordSubscribe gRPC stream.
type AcctzStream_RecordSubscribeClient interface {
	Recv() (*acctzpb.RecordResponse, error)
	grpc.ClientStream
}

type nokiaAcctzClient struct {
	conn *grpc.ClientConn
}

func (c *nokiaAcctzClient) RecordSubscribe(ctx context.Context, in *acctzpb.RecordRequest, opts ...grpc.CallOption) (AcctzStream_RecordSubscribeClient, error) {
	stream, err := c.conn.NewStream(ctx, &grpc.StreamDesc{
		StreamName:    "RecordSubscribe",
		Handler:       nil,
		ServerStreams: true,
		ClientStreams: true,
	}, "/gnsi.acctz.v1.AcctzStream/RecordSubscribe", opts...)
	if err != nil {
		return nil, err
	}
	x := &nokiaAcctzRecordSubscribeClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type nokiaAcctzRecordSubscribeClient struct {
	grpc.ClientStream
}

func (x *nokiaAcctzRecordSubscribeClient) Recv() (*acctzpb.RecordResponse, error) {
	m := new(acctzpb.RecordResponse)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// GetNokiaCustomAcctzClient returns a custom gNSI Acctz client for Nokia devices connecting to port 10162.
func GetNokiaCustomAcctzClient(t *testing.T, dut *ondatra.DUTDevice) AcctzStreamClient {
	t.Helper()
	var dialer interface {
		DialGRPCWithPort(context.Context, int, ...grpc.DialOption) (*grpc.ClientConn, error)
	}
	bindingDUT := dut.RawAPIs().BindingDUT()
	if err := binding.DUTAs(bindingDUT, &dialer); err != nil {
		t.Fatalf("BindingDUT %T does not implement DialGRPCWithPort, which is required for Nokia custom client: %v", bindingDUT, err)
	}

	conn, err := dialer.DialGRPCWithPort(context.Background(), 10162)
	if err != nil {
		t.Fatalf("DialGRPCWithPort failed for port 10162: %v", err)
	}
	return &nokiaAcctzClient{conn: conn}
}

// func getGrpcTarget(t *testing.T, dut *ondatra.DUTDevice, service introspect.Service) string {
// 	dialTarget := introspect.DUTDialer(t, dut, service).DialTarget
// 	resolvedTarget, err := net.ResolveTCPAddr("tcp", dialTarget)
// 	if err != nil {
// 		t.Fatalf("Failed resolving %s target %s", service, dialTarget)
// 	}
// 	t.Logf("Target for %s service: %s", service, resolvedTarget)
// 	return resolvedTarget.String()
// }

// getSSHTarget returns the target for the SSH service.
func getSSHTarget(t *testing.T, dut *ondatra.DUTDevice, staticBinding bool) string {
	if staticBinding {
		f := flag.Lookup("binding")
		if f == nil {
			t.Fatal("'binding' flag not found. This is usually defined by Ondatra.")
		}
		bindingFile := f.Value.String()
		in, err := os.ReadFile(bindingFile)
		if err != nil {
			t.Fatalf("failed to read binding file: %v", err)
		}
		b := &bindpb.Binding{}
		if err := prototext.Unmarshal(in, b); err != nil {
			t.Fatalf("unable to parse binding file: %v", err)
		}
		for _, d := range b.Duts {
			if d.Id == dut.ID() {
				sshTarget := strings.Split(d.Ssh.Target, ":")
				sshIp := sshTarget[0]
				sshPort := "22"
				if len(sshTarget) > 1 {
					sshPort = sshTarget[1]
				}
				target := fmt.Sprintf("%s:%s", sshIp, sshPort)
				t.Logf("Target for ssh service: %s", target)
				return target
			}
		}
		t.Fatalf("DUT %s not found in binding file", dut.ID())
		return ""
	} else {
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
				t.Logf("Failed resolving ssh target %s, will try with fqdn", dialTarget)
				domain := "net.google.com"
				if args.Fqdn != nil {
					domain = *args.Fqdn
				}
				dialTarget = fmt.Sprintf("%s.%s:%d", dut.Name(), domain, defaultSSHPort)
				resolvedTarget, err = net.ResolveTCPAddr("tcp", dialTarget)
				if err != nil {
					t.Fatalf("Failed resolving ssh target %s", dialTarget)
				}
			}
			target = resolvedTarget.String()
		} else {
			dutSSHService, err := serviceDUT.Service("ssh")
			if err != nil {
				t.Fatal(err)
			}
			target = fmt.Sprintf("%s:%d", dutSSHService.GetOutsideIp(), dutSSHService.GetOutside())
		}

		t.Logf("Target for ssh service: %s", target)
		return target
	}
}

// func dialGrpc(t *testing.T, target string) *grpc.ClientConn {
// 	conn, err := grpc.NewClient(
// 		target,
// 		grpc.WithTransportCredentials(
// 			credentials.NewTLS(
// 				&tls.Config{
// 					InsecureSkipVerify: true,
// 				},
// 			),
// 		),
// 		grpc.WithContextDialer(func(ctx context.Context, a string) (net.Conn, error) {
// 			dst, err := net.ResolveTCPAddr("tcp", a)
// 			if err != nil {
// 				return nil, err
// 			}
// 			c, err := net.DialTCP("tcp", nil, dst)
// 			if err != nil {
// 				return nil, err
// 			}
// 			gRPCClientAddr = c.LocalAddr()
// 			return c, err
// 		}))
// 	if err != nil {
// 		t.Fatalf("Got unexpected error dialing gRPC target %q, error: %v", target, err)
// 	}

// 	return conn
// }

func extractRawSSHClient(c binding.SSHClient) *ssh.Client {
	v := reflect.ValueOf(c)
	if !v.IsValid() {
		return nil
	}
	for v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return nil
	}
	// Try to find a field of type *ssh.Client by name "Client" first.
	f := v.FieldByName("Client")
	if f.IsValid() && f.CanInterface() && f.Type().String() == "*ssh.Client" {
		return f.Interface().(*ssh.Client)
	}
	// If not found, iterate through all fields and return the first *ssh.Client found.
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		if field.CanInterface() && field.Type().String() == "*ssh.Client" {
			return field.Interface().(*ssh.Client)
		}
	}
	return nil
}

func dialSSH(t *testing.T, dut *ondatra.DUTDevice, username, password, target string) (*ssh.Client, io.WriteCloser) {
	var conn *ssh.Client
	var err error

	// Try using the binding's DialSSH first as it may handle proxies/gateways.
	auth := binding.PasswordAuth{
		User:     username,
		Password: password,
	}
	t.Logf("Attempting to dial SSH to %s with user %s", target, username)
	if bClient, bErr := dut.RawAPIs().BindingDUT().DialSSH(context.Background(), auth); bErr == nil {
		t.Logf("BindingDUT().DialSSH succeeded for target %s", target)
		if raw := extractRawSSHClient(bClient); raw != nil {
			t.Logf("Successfully extracted raw ssh.Client from binding client")
			conn = raw
		} else {
			t.Logf("extractRawSSHClient failed to find *ssh.Client in binding client")
		}
	} else {
		t.Logf("BindingDUT().DialSSH failed for target %s: %v", target, bErr)
	}

	if conn == nil {
		conn, err = ssh.Dial(
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
				HostKeyCallback: ssh.InsecureIgnoreHostKey(), // lgtm[go/insecure-hostkeycallback]
				Timeout:         120 * time.Second,
			})
		if err != nil {
			t.Fatalf("Got unexpected error dialing ssh target %s, error: %v", target, err)
		}
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

func getMetadataKeys(dut *ondatra.DUTDevice) (string, string) {
	return "username", "password"
}

// SendGnmiRPCs Setup gNMI test RPCs (successful and failed) to be used in the acctz client tests.
func SendGnmiRPCs(t *testing.T, dut *ondatra.DUTDevice) []*acctzpb.RecordResponse {
	// Per https://github.com/openconfig/featureprofiles/issues/2637, waiting to see what the
	// "best"/"preferred" way is to get the v4/v6 of the dut. For now, we just use introspection
	// but that won't get us v4 and v6, it will just get us whatever is configured in binding,
	// so while the test asks for v4 and v6 we'll just be doing it for whatever we get.
	// target := getGrpcTarget(t, dut, introspect.GNMI)

	var records []*acctzpb.RecordResponse
	// grpcConn := dialGrpc(t, target)
	userKey, passKey := getMetadataKeys(dut)
	if dut.Vendor() == ondatra.ARISTA {
		failuser = failAuthorizeUsername
		failpass = failAuthorizePassword
	} else {
		failuser = FailAuthenticateUsername
		failpass = failAuthenticatePassword
	}
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs(userKey, failuser, passKey, failpass))
	var gnmiClient gnmipb.GNMIClient
	var err error
	if dut.Vendor() == ondatra.NOKIA {
		var dialer interface {
			DialGRPCWithPort(context.Context, int, ...grpc.DialOption) (*grpc.ClientConn, error)
		}
		bindingDUT := dut.RawAPIs().BindingDUT()
		if err := binding.DUTAs(bindingDUT, &dialer); err != nil {
			t.Fatalf("BindingDUT %T does not implement DialGRPCWithPort: %v", bindingDUT, err)
		}
		conn, err := dialer.DialGRPCWithPort(ctx, 10162)
		if err != nil {
			t.Fatalf("Failed dialing custom gNMI port: %v", err)
		}
		gnmiClient = gnmipb.NewGNMIClient(conn)
	} else {
		gnmiClient, err = dut.RawAPIs().BindingDUT().DialGNMI(ctx)
		if err != nil {
			t.Fatalf("Failed dialing GNMI: %v", err)
		}
	}
	// Send an unsuccessful gNMI capabilities request (bad creds in context).
	_, err1 := gnmiClient.Capabilities(ctx, &gnmipb.CapabilityRequest{})
	if err1 != nil {
		t.Logf("Got expected error fetching capabilities with bad creds, error: %s", err1)
	} else {
		t.Logf("Did not get expected error fetching capabilities with bad creds. %v", err1)
	}

	if !deviations.AcctzRecordFailGrpcUnsupported(dut) {
		records = append(records, &acctzpb.RecordResponse{
			ServiceRequest: &acctzpb.RecordResponse_GrpcService{
				GrpcService: &acctzpb.GrpcService{
					ServiceType: acctzpb.GrpcService_GRPC_SERVICE_TYPE_GNMI,
					RpcName:     gnmiCapabilitiesPath,
					Authz: &acctzpb.AuthzDetail{
						Status: expectedAuthzStatus(dut, acctzpb.AuthzDetail_AUTHZ_STATUS_DENY, gnmiCapabilitiesPath),
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
					Identity: failuser,
				},
			},
		})
	}

	// Send a successful gNMI capabilities request.
	ctx = context.Background()
	ctx = metadata.AppendToOutgoingContext(ctx, "username", SuccessUsername)
	ctx = metadata.AppendToOutgoingContext(ctx, "password", successPassword)
	req := &gnmipb.CapabilityRequest{}
	payload, err := anypb.New(req)

	if err != nil {
		t.Fatal("Failed creating anypb payload.")
	}
	_, err = gnmiClient.Capabilities(ctx, req)
	if err != nil {
		t.Fatalf("Error fetching capabilities, error: %s", err)
	}

	// Remote from the perspective of the router.
	// remoteIP, remotePort := getHostPortInfo(t, gRPCClientAddr.String())
	// localIP, localPort := getHostPortInfo(t, target)
	// for _, intf := range ondatragnmi.GetAll(t, dut, ondatragnmi.OC().InterfaceAny().State()) {
	// 	if intf.GetType() == oc.IETFInterfaces_InterfaceType_softwareLoopback {
	// 		localIP = intf.GetIp()
	// 		localPort = intf.GetPort()
	// 	t.Logf("Interface: %v", intf)
	// }

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
			Status: acctzpb.SessionInfo_SESSION_STATUS_ONCE,
			//			LocalAddress:  localIP,
			//			LocalPort:     localPort,
			//			RemoteAddress: remoteIP,
			//			RemotePort:    remotePort,
			IpProto: ipProto,
			Authn: &acctzpb.AuthnDetail{
				Type:   acctzpb.AuthnDetail_AUTHN_TYPE_UNSPECIFIED,
				Status: acctzpb.AuthnDetail_AUTHN_STATUS_SUCCESS,
				Cause:  "authentication_method: local",
			},
			User: &acctzpb.UserDetail{
				Identity: SuccessUsername,
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
	// target := getGrpcTarget(t, dut, introspect.GNOI)

	var records []*acctzpb.RecordResponse
	// grpcConn := dialGrpc(t, target)
	// gnoiSystemClient := dut.RawAPIs().GNOI(t).System()
	// systempb.NewSystemClient(grpcConn)
	userKey, passKey := getMetadataKeys(dut)
	if dut.Vendor() == ondatra.ARISTA {
		failuser = failAuthorizeUsername
		failpass = failAuthorizePassword
	} else {
		failuser = FailAuthenticateUsername
		failpass = failAuthenticatePassword
	}
	var gnoiSystemClient systempb.SystemClient
	ctx := context.Background()

	if dut.Vendor() == ondatra.NOKIA {
		var dialer interface {
			DialGRPCWithPort(context.Context, int, ...grpc.DialOption) (*grpc.ClientConn, error)
		}
		bindingDUT := dut.RawAPIs().BindingDUT()
		if err := binding.DUTAs(bindingDUT, &dialer); err != nil {
			t.Fatalf("BindingDUT %T does not implement DialGRPCWithPort: %v", bindingDUT, err)
		}
		conn, err := dialer.DialGRPCWithPort(ctx, 10162)
		if err != nil {
			t.Fatalf("Failed dialing custom gNOI port: %v", err)
		}
		gnoiSystemClient = systempb.NewSystemClient(conn)
	} else {
		gnoiSystemClient = dut.RawAPIs().GNOI(t).System()
	}
	ctx = metadata.NewOutgoingContext(context.Background(), metadata.Pairs(userKey, failuser, passKey, failpass))
	var rpcName string
	var payload *anypb.Any
	var err error
	if dut.Vendor() == ondatra.NOKIA {
		rpcName = gnoiTimePath
		_, err = gnoiSystemClient.Time(ctx, &systempb.TimeRequest{})
		if err != nil {
			t.Logf("Got expected error getting gnoi system time with bad creds, error: %s", err)
		}
	} else {
		rpcName = gnoiPingPath
		gnoiSystemPingClient, err1 := gnoiSystemClient.Ping(ctx, &systempb.PingRequest{
			Destination: "127.0.0.1",
			Count:       1,
		})
		if err1 != nil {
			t.Errorf("Got unexpected error getting gnoi system ping client, error: %s", err1)
		}
		_, err = gnoiSystemPingClient.Recv()
		if err != nil {
			t.Logf("Got expected error getting gnoi system ping with bad creds, error: %s", err)
		}
	}

	if !deviations.AcctzRecordFailGrpcUnsupported(dut) {
		records = append(records, &acctzpb.RecordResponse{
			ServiceRequest: &acctzpb.RecordResponse_GrpcService{
				GrpcService: &acctzpb.GrpcService{
					ServiceType: acctzpb.GrpcService_GRPC_SERVICE_TYPE_GNOI,
					RpcName:     rpcName,
					Authz: &acctzpb.AuthzDetail{
						Status: expectedAuthzStatus(dut, acctzpb.AuthzDetail_AUTHZ_STATUS_DENY, rpcName),
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
					Identity: failuser,
				},
			},
		})
	}

	// Send a successful gNOI request.
	ctx = context.Background()
	ctx = metadata.AppendToOutgoingContext(ctx, "username", SuccessUsername)
	ctx = metadata.AppendToOutgoingContext(ctx, "password", successPassword)

	if dut.Vendor() == ondatra.NOKIA {
		req := &systempb.TimeRequest{}
		payload, err = anypb.New(req)
		if err != nil {
			t.Errorf("Failed creating anypb payload.")
		}
		_, err = gnoiSystemClient.Time(ctx, req)
		if err != nil {
			t.Errorf("Error fetching gnoi system time, error: %s", err)
		}
	} else {
		req := &systempb.PingRequest{
			Destination: "127.0.0.1",
			Count:       1,
		}
		payload, err = anypb.New(req)
		if err != nil {
			t.Errorf("Failed creating anypb payload.")
		}
		gnoiSystemPingClient, err1 := gnoiSystemClient.Ping(ctx, req)
		if err1 != nil {
			t.Errorf("Error fetching gnoi system ping, error: %s", err1)
		}
		_, err = gnoiSystemPingClient.Recv()
		if err != nil {
			t.Errorf("Got unexpected error getting gnoi system ping, error: %s", err)
		}
	}

	// Remote from the perspective of the router.
	// remoteIP, remotePort := getHostPortInfo(t, gRPCClientAddr.String())
	// localIP, localPort := getHostPortInfo(t, target)

	records = append(records, &acctzpb.RecordResponse{
		ServiceRequest: &acctzpb.RecordResponse_GrpcService{
			GrpcService: &acctzpb.GrpcService{
				ServiceType: acctzpb.GrpcService_GRPC_SERVICE_TYPE_GNOI,
				RpcName:     rpcName,
				Payload: &acctzpb.GrpcService_ProtoVal{
					ProtoVal: payload,
				},
				Authz: &acctzpb.AuthzDetail{
					Status: acctzpb.AuthzDetail_AUTHZ_STATUS_PERMIT,
				},
			},
		},
		SessionInfo: &acctzpb.SessionInfo{
			Status: acctzpb.SessionInfo_SESSION_STATUS_ONCE,
			// LocalAddress:  localIP,
			// LocalPort:     localPort,
			// RemoteAddress: remoteIP,
			// RemotePort:    remotePort,
			IpProto: ipProto,
			Authn: &acctzpb.AuthnDetail{
				Type:   acctzpb.AuthnDetail_AUTHN_TYPE_UNSPECIFIED,
				Status: acctzpb.AuthnDetail_AUTHN_STATUS_SUCCESS,
				Cause:  "authentication_method: local",
			},
			User: &acctzpb.UserDetail{
				Identity: SuccessUsername,
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
	// target := getGrpcTarget(t, dut, introspect.GNSI)

	var records []*acctzpb.RecordResponse
	// grpcConn := dialGrpc(t, target)
	// authzClient := dut.RawAPIs().GNSI(t).Authz()
	userKey, passKey := getMetadataKeys(dut)
	if dut.Vendor() == ondatra.ARISTA {
		failuser = failAuthorizeUsername
		failpass = failAuthorizePassword
	} else {
		failuser = FailAuthenticateUsername
		failpass = failAuthenticatePassword
	}
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs(userKey, failuser, passKey, failpass))
	var authzClient authzpb.AuthzClient
	if dut.Vendor() == ondatra.NOKIA {
		var dialer interface {
			DialGRPCWithPort(context.Context, int, ...grpc.DialOption) (*grpc.ClientConn, error)
		}
		bindingDUT := dut.RawAPIs().BindingDUT()
		if err := binding.DUTAs(bindingDUT, &dialer); err != nil {
			t.Fatalf("BindingDUT %T does not implement DialGRPCWithPort: %v", bindingDUT, err)
		}
		conn, err := dialer.DialGRPCWithPort(ctx, 10162)
		if err != nil {
			t.Fatalf("Failed dialing custom gNSI port: %v", err)
		}
		authzClient = authzpb.NewAuthzClient(conn)
	} else {
		authzClient = dut.RawAPIs().GNSI(t).Authz()
	}
	// Send an unsuccessful gNSI authz get request (bad creds in context), we don't
	// care about receiving on it, just want to make the request.
	_, err := authzClient.Get(ctx, &authzpb.GetRequest{})
	if err != nil {
		t.Logf("Got expected error fetching authz policy with bad creds, error: %s", err)
	} else {
		t.Logf("Did not get expected error fetching authz policy with bad creds.")
	}
	if !deviations.AcctzRecordFailGrpcUnsupported(dut) {
		records = append(records, &acctzpb.RecordResponse{
			ServiceRequest: &acctzpb.RecordResponse_GrpcService{
				GrpcService: &acctzpb.GrpcService{
					ServiceType: acctzpb.GrpcService_GRPC_SERVICE_TYPE_GNSI,
					RpcName:     gnsiGetPath,
					Authz: &acctzpb.AuthzDetail{
						Status: expectedAuthzStatus(dut, acctzpb.AuthzDetail_AUTHZ_STATUS_DENY, gnsiGetPath),
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
					Identity: failuser,
				},
			},
		})
	}
	// Send a successful gNSI authz get request.
	ctx = context.Background()
	ctx = metadata.AppendToOutgoingContext(ctx, "username", SuccessUsername)
	ctx = metadata.AppendToOutgoingContext(ctx, "password", successPassword)
	req := &authzpb.GetRequest{}
	payload, err := anypb.New(req)
	if err != nil {
		t.Errorf("Failed creating anypb payload.")
	}
	msg, err := authzClient.Get(ctx, &authzpb.GetRequest{})
	if err != nil && msg != nil {
		t.Errorf("Error fetching authz policy, error: %s", err)
	}

	// Remote from the perspective of the router.
	// remoteIP, remotePort := getHostPortInfo(t, gRPCClientAddr.String())
	// localIP, localPort := getHostPortInfo(t, target)

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
			Status: acctzpb.SessionInfo_SESSION_STATUS_ONCE,
			// LocalAddress:  localIP,
			// LocalPort:     localPort,
			// RemoteAddress: remoteIP,
			// RemotePort:    remotePort,
			IpProto: ipProto,
			Authn: &acctzpb.AuthnDetail{
				Type:   acctzpb.AuthnDetail_AUTHN_TYPE_UNSPECIFIED,
				Status: acctzpb.AuthnDetail_AUTHN_STATUS_SUCCESS,
				Cause:  "authentication_method: local",
			},
			User: &acctzpb.UserDetail{
				Identity: SuccessUsername,
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

	// target := getGrpcTarget(t, dut, introspect.GRIBI)

	var records []*acctzpb.RecordResponse
	// grpcConn := dialGrpc(t, target)
	// gribiClient := gribi.NewGRIBIClient(grpcConn)
	// gribiClient,err := dut.RawAPIs().BindingDUT().DialGRIBI
	userKey, passKey := getMetadataKeys(dut)
	if dut.Vendor() == ondatra.ARISTA {
		failuser = failAuthorizeUsername
		failpass = failAuthorizePassword
	} else {
		failuser = FailAuthenticateUsername
		failpass = failAuthenticatePassword
	}
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs(userKey, failuser, passKey, failpass))

	gribiClient, err := dut.RawAPIs().BindingDUT().DialGRIBI(ctx)
	if err != nil {
		t.Fatalf("Got unexpected error during gribi get request, error: %s", err)
	}
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
					Status: expectedAuthzStatus(dut, acctzpb.AuthzDetail_AUTHZ_STATUS_DENY, gribiGetPath),
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
				Identity: failuser,
			},
		},
	})

	// Send a successful gRIBI get request.
	ctx = context.Background()
	ctx = metadata.AppendToOutgoingContext(ctx, "username", SuccessUsername)
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
	// remoteIP, remotePort := getHostPortInfo(t, gRPCClientAddr.String())
	// localIP, localPort := getHostPortInfo(t, target)

	records = append(records, &acctzpb.RecordResponse{
		ServiceRequest: &acctzpb.RecordResponse_GrpcService{
			GrpcService: &acctzpb.GrpcService{
				ServiceType: acctzpb.GrpcService_GRPC_SERVICE_TYPE_GRIBI,
				RpcName:     gribiGetPath,
				Payload: &acctzpb.GrpcService_ProtoVal{
					ProtoVal: payload,
				},
				Authz: &acctzpb.AuthzDetail{
					Status: expectedAuthzStatus(dut, acctzpb.AuthzDetail_AUTHZ_STATUS_PERMIT, gribiGetPath),
				},
			},
		},
		SessionInfo: &acctzpb.SessionInfo{
			Status: acctzpb.SessionInfo_SESSION_STATUS_ONCE,
			// LocalAddress:  localIP,
			// LocalPort:     localPort,
			// RemoteAddress: remoteIP,
			// RemotePort:    remotePort,
			IpProto: ipProto,
			Authn: &acctzpb.AuthnDetail{
				Type:   acctzpb.AuthnDetail_AUTHN_TYPE_UNSPECIFIED,
				Status: acctzpb.AuthnDetail_AUTHN_STATUS_SUCCESS,
				Cause:  "authentication_method: local",
			},
			User: &acctzpb.UserDetail{
				Identity: SuccessUsername,
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
	// target := getGrpcTarget(t, dut, introspect.P4RT)

	// configure P4runtime
	switch dut.Vendor() {
	case ondatra.ARISTA:
		p4rtConfig := "p4-runtime\n transport grpc p4-foo\n ssl profile SELFSIGNED\n vrf mgmt\n no shutdown"
		helpers.GnmiCLIConfig(t, dut, p4rtConfig)
		gnmiClient := dut.RawAPIs().GNMI(t)
		deadline := time.Now().Add(2 * time.Minute)
		p4rtUp := false
		for !p4rtUp && time.Now().Before(deadline) {
			resp, err := gnmiClient.Get(context.Background(), &gnmipb.GetRequest{
				Path: []*gnmipb.Path{{
					Origin: "cli",
					Elem:   []*gnmipb.PathElem{{Name: "show p4-runtime"}},
				}},
				Encoding: gnmipb.Encoding_ASCII,
			})
			if err == nil {
				for _, notif := range resp.GetNotification() {
					for _, update := range notif.GetUpdate() {
						if strings.Contains(update.GetVal().GetAsciiVal(), "Server: running on port") {
							p4rtUp = true
						}
					}
				}
			}
			if !p4rtUp {
				time.Sleep(5 * time.Second)
			}
		}
		if !p4rtUp {
			t.Fatalf("P4Runtime agent did not start within timeout")
		}
		t.Log("P4Runtime agent is up and running")
	}
	var records []*acctzpb.RecordResponse
	// grpcConn := dialGrpc(t, target)
	userKey, passKey := getMetadataKeys(dut)
	if dut.Vendor() == ondatra.ARISTA {
		failuser = failAuthorizeUsername
		failpass = failAuthorizePassword
	} else {
		failuser = FailAuthenticateUsername
		failpass = failAuthenticatePassword
	}
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs(userKey, failuser, passKey, failpass))

	p4rtclient, err := dut.RawAPIs().BindingDUT().DialP4RT(ctx)
	if err != nil {
		t.Fatalf("Got unexpected error during p4rt get request, error: %s", err)
	}
	_, err = p4rtclient.Capabilities(ctx, &p4pb.CapabilitiesRequest{})
	if err != nil {
		t.Logf("Got expected error getting p4rt capabilities with no creds, error: %s", err)
	}
	if !deviations.AcctzRecordFailGrpcUnsupported(dut) {
		records = append(records, &acctzpb.RecordResponse{
			ServiceRequest: &acctzpb.RecordResponse_GrpcService{
				GrpcService: &acctzpb.GrpcService{
					ServiceType: acctzpb.GrpcService_GRPC_SERVICE_TYPE_P4RT,
					RpcName:     p4rtCapabilitiesPath,
					Authz: &acctzpb.AuthzDetail{
						Status: expectedAuthzStatus(dut, acctzpb.AuthzDetail_AUTHZ_STATUS_DENY, p4rtCapabilitiesPath),
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
					Identity: failuser,
				},
			},
		})
	}
	ctx = context.Background()
	ctx = metadata.AppendToOutgoingContext(ctx, "username", SuccessUsername)
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
	// remoteIP, remotePort := getHostPortInfo(t, gRPCClientAddr.String())
	// localIP, localPort := getHostPortInfo(t, target)

	records = append(records, &acctzpb.RecordResponse{
		ServiceRequest: &acctzpb.RecordResponse_GrpcService{
			GrpcService: &acctzpb.GrpcService{
				ServiceType: acctzpb.GrpcService_GRPC_SERVICE_TYPE_P4RT,
				RpcName:     p4rtCapabilitiesPath,
				Payload: &acctzpb.GrpcService_ProtoVal{
					ProtoVal: payload,
				},
				Authz: &acctzpb.AuthzDetail{
					Status: expectedAuthzStatus(dut, acctzpb.AuthzDetail_AUTHZ_STATUS_PERMIT, p4rtCapabilitiesPath),
				},
			},
		},
		SessionInfo: &acctzpb.SessionInfo{
			Status: acctzpb.SessionInfo_SESSION_STATUS_ONCE,
			// LocalAddress:  localIP,
			// LocalPort:     localPort,
			// RemoteAddress: remoteIP,
			// RemotePort:    remotePort,
			IpProto: ipProto,
			Authn: &acctzpb.AuthnDetail{
				Type:   acctzpb.AuthnDetail_AUTHN_TYPE_UNSPECIFIED,
				Status: acctzpb.AuthnDetail_AUTHN_STATUS_SUCCESS,
				Cause:  "authentication_method: local",
			},
			User: &acctzpb.UserDetail{
				Identity: SuccessUsername,
			},
		},
	})

	return records
}

// SendSuccessCliCommand Setup test CLI command (successful) to be used in the acctz client tests.
func SendSuccessCliCommand(t *testing.T, dut *ondatra.DUTDevice, staticBinding bool) []*acctzpb.RecordResponse {
	// Per https://github.com/openconfig/featureprofiles/issues/2637, waiting to see what the
	// "best"/"preferred" way is to get the v4/v6 of the dut. For now, we use this workaround
	// because ssh isn't exposed in introspection.
	target := getSSHTarget(t, dut, staticBinding)

	var records []*acctzpb.RecordResponse

	sshConn, w := dialSSH(t, dut, SuccessUsername, successPassword, target)
	defer func() {
		// Give things a second to percolate then close the connection.
		time.Sleep(6 * time.Second)
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
	// remoteIP, remotePort := getHostPortInfo(t, sshConn.LocalAddr().String())
	// localIP, localPort := getHostPortInfo(t, target)

	var authnField *acctzpb.AuthnDetail

	switch dut.Vendor() {
	case ondatra.CISCO:
		// Authn field popoulated for Cisco only for Login/Once/Enable records.
		authnField = nil
	default:
		authnField = &acctzpb.AuthnDetail{
			Type:   acctzpb.AuthnDetail_AUTHN_TYPE_UNSPECIFIED,
			Status: acctzpb.AuthnDetail_AUTHN_STATUS_SUCCESS,
			Cause:  "authentication_method: local",
		}
	}

	var userRole string
	switch dut.Vendor() {
	case ondatra.CISCO:
		userRole = "root-lr, cisco-support"
	case ondatra.NOKIA:
		userRole = "admin"
	case ondatra.ARISTA:
		userRole = "network-admin"
	default:
		userRole = ""
	}

	records = append(records, &acctzpb.RecordResponse{
		ServiceRequest: &acctzpb.RecordResponse_CmdService{
			CmdService: &acctzpb.CommandService{
				ServiceType: acctzpb.CommandService_CMD_SERVICE_TYPE_CLI,
				Cmd:         successCliCommand,
				Authz: &acctzpb.AuthzDetail{
					Status: expectedAuthzStatus(dut, acctzpb.AuthzDetail_AUTHZ_STATUS_PERMIT, successCliCommand),
				},
			},
		},
		SessionInfo: &acctzpb.SessionInfo{
			Status: acctzpb.SessionInfo_SESSION_STATUS_OPERATION,
			// LocalAddress:  localIP,
			// LocalPort:     localPort,
			// RemoteAddress: remoteIP,
			// RemotePort:    remotePort,
			IpProto: ipProto,
			Authn:   authnField,
			User: &acctzpb.UserDetail{
				Identity: SuccessUsername,
				Role:     userRole,
			},
		},
	})

	return records
}

// SendFailCliCommand Setup test CLI command (failed) to be used in the acctz client tests.
func SendFailCliCommand(t *testing.T, dut *ondatra.DUTDevice, staticBinding bool) []*acctzpb.RecordResponse {
	// Per https://github.com/openconfig/featureprofiles/issues/2637, waiting to see what the
	// "best"/"preferred" way is to get the v4/v6 of the dut. For now, we use this workaround
	// because ssh isn't exposed in introspection.
	target := getSSHTarget(t, dut, staticBinding)

	var records []*acctzpb.RecordResponse

	if dut.Vendor() == ondatra.ARISTA || dut.Vendor() == ondatra.NOKIA || dut.Vendor() == ondatra.CISCO {
		failuser = failAuthorizeUsername
		failpass = failAuthorizePassword
	} else {
		failuser = FailAuthenticateUsername
		failpass = failAuthenticatePassword
	}
	sshConn, w := dialSSH(t, dut, failuser, failpass, target)
	defer func() {
		// Give things a second to percolate then close the connection.
		time.Sleep(6 * time.Second)
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
	// remoteIP, remotePort := getHostPortInfo(t, sshConn.LocalAddr().String())
	// localIP, localPort := getHostPortInfo(t, target)

	var authnField *acctzpb.AuthnDetail

	switch dut.Vendor() {
	case ondatra.CISCO:
		// Authn field popoulated for Cisco only for Login/Once/Enable records.
		authnField = nil
	default:
		authnField = &acctzpb.AuthnDetail{
			Type:   acctzpb.AuthnDetail_AUTHN_TYPE_UNSPECIFIED,
			Status: acctzpb.AuthnDetail_AUTHN_STATUS_SUCCESS,
			Cause:  "authentication_method: local",
		}
	}

	var authzStatusField *acctzpb.AuthzDetail
	if deviations.AcctzRecordsAuthzStatusDenyUnsupported(dut) {
		authzStatusField = &acctzpb.AuthzDetail{
			Status: acctzpb.AuthzDetail_AUTHZ_STATUS_UNSPECIFIED,
		}
	} else {
		authzStatusField = &acctzpb.AuthzDetail{
			Status: expectedAuthzStatus(dut, acctzpb.AuthzDetail_AUTHZ_STATUS_DENY, failCliCommand),
		}
	}

	var userRole string
	switch dut.Vendor() {
	case ondatra.CISCO:
		userRole = "root-lr, cisco-support"
	case ondatra.NOKIA:
		userRole = "admin"
	case ondatra.ARISTA:
		userRole = "acctz-fp-test-fail"
	default:
		userRole = ""
	}

	records = append(records, &acctzpb.RecordResponse{
		ServiceRequest: &acctzpb.RecordResponse_CmdService{
			CmdService: &acctzpb.CommandService{
				ServiceType: acctzpb.CommandService_CMD_SERVICE_TYPE_CLI,
				Cmd:         failCliCommand,
				Authz:       authzStatusField,
			},
		},
		SessionInfo: &acctzpb.SessionInfo{
			Status: acctzpb.SessionInfo_SESSION_STATUS_OPERATION,
			// LocalAddress:  localIP,
			// LocalPort:     localPort,
			// RemoteAddress: remoteIP,
			// RemotePort:    remotePort,
			IpProto: ipProto,
			Authn:   authnField,
			User: &acctzpb.UserDetail{
				Identity: failuser,
				Role:     userRole,
			},
		},
	})

	return records
}

// SendShellCommand Setup test shell command (successful) to be used in the acctz client tests.
func SendShellCommand(t *testing.T, dut *ondatra.DUTDevice, staticBinding bool) []*acctzpb.RecordResponse {
	// Per https://github.com/openconfig/featureprofiles/issues/2637, waiting to see what the
	// "best"/"preferred" way is to get the v4/v6 of the dut. For now, we use this workaround
	// because ssh isn't exposed in introspection.
	target := getSSHTarget(t, dut, staticBinding)

	var records []*acctzpb.RecordResponse
	shellUsername := SuccessUsername
	shellPassword := successPassword

	switch dut.Vendor() {
	case ondatra.NOKIA:
		// Assuming linuxadmin is present and ssh'ing directly via this user gets us to shell
		// straight away so this is easy button to trigger a shell record.
		shellUsername = "linuxadmin"
		shellPassword = "NokiaSrl1!"
	}

	var userRole string
	switch dut.Vendor() {
	case ondatra.CISCO:
		userRole = "root-lr, cisco-support"
	case ondatra.NOKIA:
		userRole = "admin"
	case ondatra.ARISTA:
		userRole = "network-admin"
	default:
		userRole = ""
	}

	sshConn, w := dialSSH(t, dut, shellUsername, shellPassword, target)
	defer func() {
		// Give things a second to percolate then close the connection.
		time.Sleep(6 * time.Second)
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
	// remoteIP, remotePort := getHostPortInfo(t, sshConn.LocalAddr().String())
	// localIP, localPort := getHostPortInfo(t, target)

	records = append(records, &acctzpb.RecordResponse{
		ServiceRequest: &acctzpb.RecordResponse_CmdService{
			CmdService: &acctzpb.CommandService{
				ServiceType: acctzpb.CommandService_CMD_SERVICE_TYPE_SHELL,
				Cmd:         shellCommand,
				Authz: &acctzpb.AuthzDetail{
					Status: expectedAuthzStatus(dut, acctzpb.AuthzDetail_AUTHZ_STATUS_PERMIT, shellCommand),
				},
			},
		},
		SessionInfo: &acctzpb.SessionInfo{
			Status: acctzpb.SessionInfo_SESSION_STATUS_OPERATION,
			// LocalAddress:  localIP,
			// LocalPort:     localPort,
			// RemoteAddress: remoteIP,
			// RemotePort:    remotePort,
			IpProto: ipProto,
			Authn: &acctzpb.AuthnDetail{
				Type:   acctzpb.AuthnDetail_AUTHN_TYPE_UNSPECIFIED,
				Status: acctzpb.AuthnDetail_AUTHN_STATUS_UNSPECIFIED,
			},
			User: &acctzpb.UserDetail{
				Identity: shellUsername,
				Role:     userRole,
			},
		},
	})

	return records
}

func expectedAuthzStatus(dut *ondatra.DUTDevice, status acctzpb.AuthzDetail_AuthzStatus, rpcName string) acctzpb.AuthzDetail_AuthzStatus {
	if dut.Vendor() == ondatra.NOKIA && status == acctzpb.AuthzDetail_AUTHZ_STATUS_DENY {
		return acctzpb.AuthzDetail_AUTHZ_STATUS_ERROR
	}
	if dut.Vendor() == ondatra.ARISTA && rpcName == gribiGetPath && status == acctzpb.AuthzDetail_AUTHZ_STATUS_DENY {
		return acctzpb.AuthzDetail_AUTHZ_STATUS_PERMIT
	}
	return status
}
