/*
 Copyright 2022 Google LLC

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

      https://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package system_gnmi_get_test

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
	"github.com/openconfig/ygot/ytypes"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestGNMIGet(t *testing.T) {
	shortResponse := func(g *gpb.GetResponse) string {
		p := proto.Clone(g).(*gpb.GetResponse)
		for _, n := range p.Notification {
			for _, u := range n.Update {
				u.Val = nil
			}
		}
		return prototext.Format(p)
	}

	tests := []struct {
		desc            string
		inGetRequest    *gpb.GetRequest
		wantGetResponse *gpb.GetResponse
		wantErrCode     codes.Code
		cmpOpts         []cmp.Option
		chkFn           func(*testing.T, *gpb.GetResponse)
	}{{
		desc: "single response for /",
		inGetRequest: &gpb.GetRequest{
			Prefix: &gpb.Path{
				Origin: "openconfig",
			},
			Path: []*gpb.Path{{
				// empty path indicates the root.
			}},
			Type:     gpb.GetRequest_CONFIG,
			Encoding: gpb.Encoding_JSON_IETF,
		},
		wantGetResponse: &gpb.GetResponse{
			Notification: []*gpb.Notification{{
				Update: []*gpb.Update{{
					Path: &gpb.Path{},
				}},
			}},
		},
		cmpOpts: []cmp.Option{
			protocmp.IgnoreFields(&gpb.GetResponse{}, "notification"),
		},
	}, {
		desc: "response for / honours encoding",
		inGetRequest: &gpb.GetRequest{
			Prefix: &gpb.Path{
				Origin: "openconfig",
			},
			Path:     []*gpb.Path{{}},
			Type:     gpb.GetRequest_CONFIG,
			Encoding: gpb.Encoding_JSON_IETF,
		},
		wantGetResponse: &gpb.GetResponse{
			Notification: []*gpb.Notification{{
				Update: []*gpb.Update{{
					Path: &gpb.Path{},
				}},
			}},
		},
		cmpOpts: []cmp.Option{
			protocmp.IgnoreFields(&gpb.GetResponse{}, "notification"),
		},
		chkFn: func(t *testing.T, res *gpb.GetResponse) {
			d := &oc.Root{}
			for _, n := range res.Notification {
				for _, u := range n.Update {
					// The updates here are all TypedValue JSON_IETF fields that should be
					// unmarshallable using SetNode to the root (since the get is for /.
					if u.GetVal().GetJsonIetfVal() == nil {
						t.Fatalf("got an update with a non JSON_IETF schema, got: %s", u)
					}
					if err := oc.Unmarshal(u.Val.GetJsonIetfVal(), d, &ytypes.IgnoreExtraFields{}); err != nil {
						t.Fatalf("cannot call Unmarshal for path %s, err: %v", u.Path, err)
					}
				}
			}
		},
	}}

	for _, tt := range tests {
		dut := ondatra.DUT(t, "dut")
		// Not assuming that oc base config  is loaded.
		// Config the hostname to prevent the test failure when oc base config is not loaded
		gnmi.Replace(t, dut, gnmi.OC().System().Hostname().Config(), "ondatraHost")
		gnmiC := dut.RawAPIs().GNMI(t)

		t.Run(tt.desc, func(t *testing.T) {
			gotRes, err := gnmiC.Get(context.Background(), tt.inGetRequest)
			switch {
			case err == nil && tt.wantErrCode != codes.OK:
				t.Fatalf("did not get expected error, got: %v, want: %v", err, tt.wantErrCode)
			case err != nil && tt.wantErrCode == codes.OK:
				t.Fatalf("got unexpected error, got: %v, want OK", err)
			case err != nil:
				s, ok := status.FromError(err)
				if !ok {
					t.Fatalf("error returned was not a status, got: %T", err)
				}
				if got, want := s.Code(), tt.wantErrCode; got != want {
					t.Fatalf("did not get expected error code, got: %v, want: %v", got, want)
				}
			}

			if got, want := len(gotRes.Notification), len(tt.wantGetResponse.Notification); got != want {
				t.Fatalf("did not get expected number of Notification fields, got: %d, want: %d", got, want)
			}

			// Check semantics of the response that we received from the DUT independently of its contents.
			// Must be one Notification per Path, and each Notification must contain the same path.
			foundPaths := map[string]bool{}
			for _, n := range gotRes.Notification {
				var p *gpb.Path
				for _, u := range n.Update {
					if p == nil {
						p = u.Path
					}
					if !proto.Equal(p, u.Path) {
						t.Fatalf("got mixed paths within a single Notification, want: %s, got: %s (%s)", p, u.Path, shortResponse(gotRes))
					}

					ps, err := ygot.PathToString(u.Path)
					if err != nil {
						t.Fatalf("got invalid path within an Update, got: %s, err: %v", u.Path, err)
					}
					foundPaths[ps] = true
				}
			}

			if len(foundPaths) != len(tt.inGetRequest.Path) {
				t.Fatalf("did not get expected number of paths, got: %d (%v), want: %d (%v)", len(foundPaths), foundPaths, len(tt.inGetRequest.Path), tt.inGetRequest.Path)
			}

			for _, p := range tt.inGetRequest.Path {
				ps, err := ygot.PathToString(p)
				if err != nil {
					t.Fatalf("invalid path in input GetRequest, got: %v, err: %v", p, err)
				}
				if !foundPaths[ps] {
					// this path wasn't present
					t.Fatalf("did not get a response for path %s, got: nil", ps)
				}
			}

			if tt.chkFn != nil {
				tt.chkFn(t, gotRes)
			}
		})
	}
}
