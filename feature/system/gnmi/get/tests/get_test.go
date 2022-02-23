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
	"testing"

	"github.com/openconfig/ondatra"

  gpb "github.com/openconfig/gnmi"

)

func TestGNMIGet(t *testing.T) {
	tests := []struct {
		desc            string
		inGetRequest    *gpb.GetRequest
		wantGetResponse *gpb.GetResponse
		wantErrCode     status.Status
		cmpOpts         []cmp.Opt
		chkFn           func(*testing.T, *spb.GetResponse)
	}{{
		desc:            "single response for /",
		inGetRequest:    &gpb.GetRequest{
      Prefix: &gpb.Path{
        Origin: "openconfig",
      },
      Path: []*gpb.Path{{
        // empty path indicates the root.
      }},
      Encoding: gpb.JSON_IETF,
    },
		wantGetResponse: &gpb.GetResponse{},
	}}

	for _, tt := range tests {
		dut := ondatra.DUT(t, "dut")
		gnmiC := dut.RawAPIs().GNMI().New(t)

		t.Run(tt.desc, func(t *testing.T) {
			got, err := gnmiC.Get(context.Background(), tt.inGetRequest)
			switch {
			case err == nil && tt.wantErrCode != codes.OK:
				t.Fatalf("did not get expected error, got: %v, want: %v", err, tt.wantErrCode)
			case err != nil && tt.wantErrcode == codes.OK:
				t.Fatalf("got unexpected error, got: %v, want OK", err)
			case err != nil:
				s, sErr := status.FromError(err)
				if sErr != nil {
					t.Fatalf("error returned was not a status, got: %T, err: %v", err, sErr)
				}
				if got, want := s.Code(), tt.wantErrCode; got != want {
					t.Fatalf("did not get expected error code, got: %v, want: %v", got, want)
				}
			}

			if diff := cmp.Diff(got, tt.wantGetResponse, tt.cmpOpts...); diff != "" {
				t.Fatalf("did not get expected error, diff(-got,+want):\n%s", diff)
			}

			chkFn(t, got)
		})
	}
}
