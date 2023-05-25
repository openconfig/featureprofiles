// Copyright 2022 Google LLC
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

package tls_authentication_over_grpc_test

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/knebind/solver"
	"github.com/openconfig/ygot/ygot"
	"golang.org/x/crypto/ssh"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
	tpb "github.com/openconfig/kne/proto/topo"
)

const (
	sshPort = 22
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func keyboardInteraction(password string) ssh.KeyboardInteractiveChallenge {
	return func(user, instruction string, questions []string, echos []bool) ([]string, error) {
		if len(questions) == 0 {
			return []string{}, nil
		}
		return []string{password}, nil
	}
}

func gnmiClient(ctx context.Context, sshIP string, dut *ondatra.DUTDevice) (gpb.GNMIClient, error) {
	// TODO(greg-dennis): Remove hard-coded gNMI port.
	var gnmiPort int
	switch dut.Vendor() {
	case ondatra.JUNIPER:
		gnmiPort = 9339
	default:
		gnmiPort = 6030
	}

	conn, err := grpc.DialContext(
		ctx,
		fmt.Sprintf("%s:%d", sshIP, gnmiPort),
		grpc.WithTransportCredentials(
			credentials.NewTLS(&tls.Config{
				InsecureSkipVerify: true, // NOLINT
			})),
	)
	if err != nil {
		return nil, fmt.Errorf("grpc.DialContext => unexpected failure dialing GNMI (should not require auth): %w", err)
	}
	return gpb.NewGNMIClient(conn), nil
}

func TestAuthentication(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	serviceMap := dut.CustomData(solver.KNEServiceMapKey).(map[string]*tpb.Service)
	sshService, ok := serviceMap["ssh"]
	if !ok {
		t.Fatal("No SSH service available on dut")
	}
	sshIP := sshService.GetOutsideIp()

	gnmi.Replace(t, dut, gnmi.OC().System().Aaa().Authentication().
		User("alice").Config(), &oc.System_Aaa_Authentication_User{
		Username: ygot.String("alice"),
		Password: ygot.String("password"),
		Role:     oc.AaaTypes_SYSTEM_DEFINED_ROLES_SYSTEM_ROLE_ADMIN,
	})

	tests := []struct {
		desc       string
		user, pass string
		wantErr    bool
	}{{
		desc: "good username and password",
		user: "alice",
		pass: "password",
	}, {
		desc:    "good username bad password",
		user:    "alice",
		pass:    "badpass",
		wantErr: true,
	}, {
		desc:    "bad username",
		user:    "bob",
		pass:    "password",
		wantErr: true,
	}}
	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			t.Log("Trying SSH credentials")
			sshClient, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", sshIP, sshPort), &ssh.ClientConfig{
				User: tc.user,
				Auth: []ssh.AuthMethod{
					ssh.KeyboardInteractive(keyboardInteraction(tc.pass)),
				},
				HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			})
			if err == nil {
				defer sshClient.Close()
			}
			if tc.wantErr != (err != nil) {
				if tc.wantErr {
					t.Errorf("ssh.Dial got nil error, want error for user %q, password %q", tc.user, tc.pass)
				} else {
					t.Errorf("ssh.Dial got error %v, want nil for user %q, password %q", err, tc.user, tc.pass)
				}
			}

			ctx := metadata.AppendToOutgoingContext(
				context.Background(),
				"username", tc.user,
				"password", tc.pass)
			gnmi, err := gnmiClient(ctx, sshIP, dut)
			if err != nil {
				t.Fatal(err)
			}
			t.Log("Configuring hostname using GNMI Set")
			_, err = gnmi.Set(ctx, &gpb.SetRequest{
				Replace: []*gpb.Update{{
					Path: &gpb.Path{
						Elem: []*gpb.PathElem{
							{Name: "system"}, {Name: "config"}, {Name: "hostname"}},
					},
					Val: &gpb.TypedValue{
						Value: &gpb.TypedValue_JsonIetfVal{JsonIetfVal: []byte("\"ondatraDUT\"")},
					},
				}},
			})
			if tc.wantErr != (err != nil) {
				if tc.wantErr {
					t.Errorf("gnmi.Set nil error when error expected for user %q", tc.user)
				} else {
					t.Errorf("gnmi.Set unexpected error for user %q: %v", tc.user, err)
				}
			}
			t.Log("Trying credentials with GNMI Get")
			_, err = gnmi.Get(ctx, &gpb.GetRequest{
				Path: []*gpb.Path{{
					Elem: []*gpb.PathElem{
						{Name: "system"}, {Name: "config"}, {Name: "hostname"}}},
				},
				Type:     gpb.GetRequest_CONFIG,
				Encoding: gpb.Encoding_JSON_IETF,
			})
			if tc.wantErr != (err != nil) {
				if tc.wantErr {
					t.Errorf("gnmi.Get nil error when error expected for user %q", tc.user)
				} else {
					t.Errorf("gnmi.Get unexpected error for user %q: %v", tc.user, err)
				}
			}
			t.Log("Trying credentials with GNMI Set")
			jsonConfig, _ := json.Marshal(deviations.BannerDelimiter(dut) + "message of the day" + deviations.BannerDelimiter(dut))
			_, err = gnmi.Set(ctx, &gpb.SetRequest{
				Replace: []*gpb.Update{{
					Path: &gpb.Path{
						Elem: []*gpb.PathElem{
							{Name: "system"}, {Name: "config"}, {Name: "motd-banner"}},
					},
					Val: &gpb.TypedValue{Value: &gpb.TypedValue_JsonIetfVal{JsonIetfVal: jsonConfig}},
				}},
			})
			if tc.wantErr != (err != nil) {
				if tc.wantErr {
					t.Errorf("gnmi.Set nil error when error expected for user %q", tc.user)
				} else {
					t.Errorf("gnmi.Set unexpected error for user %q: %v", tc.user, err)
				}
			}
		})
	}
}
