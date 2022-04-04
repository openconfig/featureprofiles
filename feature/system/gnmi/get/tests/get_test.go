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
	"github.com/openconfig/featureprofiles/yang/fpoc"
	"github.com/openconfig/ondatra"
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
			var val *gpb.TypedValue
			for _, n := range res.Notification {
				for _, u := range n.Update {
					// we know notification[0].update[0] is the contents of our Get in JSON.
					val = u.Val
					break
				}
			}

			if val == nil {
				t.Fatalf("did not get a valid update, got: %v", shortResponse(res))
			}

			jv := val.GetJsonIetfVal()
			if jv == nil {
				t.Fatalf("did not get JSON IETF value as expected, got:  %v", val)
			}

			d := &fpoc.Device{}
			if err := fpoc.Unmarshal(jv, d, &ytypes.IgnoreExtraFields{}); err != nil {
				t.Fatalf("did not get valid JSON IETF value, got err: %v", err)
			}
		},
	}}

	for _, tt := range tests {
		dut := ondatra.DUT(t, "dut")
		gnmiC := dut.RawAPIs().GNMI().New(t)

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

			found := map[string]bool{}
			for _, n := range tt.wantGetResponse.Notification {
				for _, u := range n.Update {
					p, err := ygot.PathToString(u.Path)
					if err != nil {
						t.Fatalf("cannot convert path %v to string, err: %v", u.Path, err)
					}
					found[p] = false
				}
			}

			for _, n := range gotRes.Notification {
				// TODO(robjs): today this is not specified in the spec, but means that there can be >1 update where the path does
				// not match what the target responded.
				if len(n.Update) != 1 {
					t.Fatalf("did not get expected number of updates per Notification, got: %d (%v), want: 1", len(n.Update), shortResponse(gotRes))
				}
				msg := n.Update[0]

				p, err := ygot.PathToString(msg.Path)
				if err != nil {
					t.Fatalf("cannot convert path %v to string, err: %v", msg.Path, err)
				}

				seen, ok := found[p]
				if !ok {
					t.Errorf("found unexpected path %v in Notifications, got: %v", msg.Path, shortResponse(gotRes))
				}
				if seen {
					t.Errorf("saw repeated path %v in Notifications, got: %v", msg.Path, shortResponse(gotRes))
				}
				found[p] = true
			}

			for p, ok := range found {
				if !ok {
					t.Errorf("did not find path %v in Notifications, got: %v", p, shortResponse(gotRes))
				}
			}

			if tt.chkFn != nil {
				tt.chkFn(t, gotRes)
			}
		})
	}
}
