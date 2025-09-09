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
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/security/credz"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
	"golang.org/x/crypto/ssh"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// helper function for native model;
// Configure a new user by passing a username and password and assign that user to a role
// ensure role has write access
func createNativeUser(t testing.TB, dut *ondatra.DUTDevice, user string, pass string, role string) {
	t.Helper()
	switch dut.Vendor() {
	case ondatra.NOKIA:
		var roleVal = []any{
			map[string]any{
				"services": []string{"cli", "gnmi"},
			},
		}
		roleUpdate, err := json.Marshal(roleVal)
		if err != nil {
			t.Fatalf("Error with json Marshal: %v", err)
		}

		var userDataVal = []any{
			map[string]any{
				"password": pass,
				"role":     []string{"admin"},
			},
		}
		userDataUpdate, err := json.Marshal(userDataVal)
		if err != nil {
			t.Fatalf("Error with json Marshal: %v", err)
		}

		var ruleVal = []any{
			map[string]any{
				"action": "write",
			},
		}
		ruleValUpdate, err := json.Marshal(ruleVal)
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
							JsonIetfVal: roleUpdate,
						},
					},
				},
				{
					Path: &gpb.Path{
						Elem: []*gpb.PathElem{
							{Name: "system"},
							{Name: "aaa"},
							{Name: "authentication"},
							{Name: "user", Key: map[string]string{"username": user}},
						},
					},
					Val: &gpb.TypedValue{
						Value: &gpb.TypedValue_JsonIetfVal{
							JsonIetfVal: userDataUpdate,
						},
					},
				},
				{
					Path: &gpb.Path{
						Elem: []*gpb.PathElem{
							{Name: "system"},
							{Name: "configuration"},
							{Name: "role", Key: map[string]string{"name": "admin"}},
							{Name: "rule", Key: map[string]string{"path-reference": "/"}},
						},
					},
					Val: &gpb.TypedValue{
						Value: &gpb.TypedValue_JsonIetfVal{
							JsonIetfVal: ruleValUpdate,
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

func TestAuthentication(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	if deviations.SetNativeUser(dut) {
		createNativeUser(t, dut, "alice", "password", "admin")
	} else {
		gnmi.Replace(t, dut, gnmi.OC().System().Aaa().Authentication().
			User("alice").Config(), &oc.System_Aaa_Authentication_User{
			Username: ygot.String("alice"),
			Password: ygot.String("password"),
			Role:     oc.AaaTypes_SYSTEM_DEFINED_ROLES_SYSTEM_ROLE_ADMIN,
		})
	}

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
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()
			client, err := credz.SSHWithPassword(ctx, dut, tc.user, tc.pass)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("Dialing ssh succeeded, but we expected to fail.")
				}
				return
			}
			if err != nil {
				t.Fatalf("Failed dialing ssh, error: %s", err)
			}
			defer client.Close()
			if tc.wantErr != (err != nil) {
				if tc.wantErr {
					t.Errorf("ssh.Dial got nil error, want error for user %q, password %q", tc.user, tc.pass)
				} else {
					t.Errorf("ssh.Dial got error %v, want nil for user %q, password %q", err, tc.user, tc.pass)
				}
			}

			ctx = metadata.AppendToOutgoingContext(
				context.Background(),
				"username", tc.user,
				"password", tc.pass)
			gnmi, err := dut.RawAPIs().BindingDUT().DialGNMI(ctx)
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
