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

package fptest

import (
	"strings"
	"testing"

	"github.com/openconfig/ondatra"
)

func TestLAGName(t *testing.T) {
	tests := []struct {
		desc    string
		v       ondatra.Vendor
		id      int
		want    string
		wantErr string
	}{{
		desc: "Valid Arista LAG",
		v:    ondatra.ARISTA,
		id:   1001,
		want: "Port-Channel1001",
	}, {
		desc: "Valid Cisco LAG",
		v:    ondatra.CISCO,
		id:   1001,
		want: "Bundle-Ether1001",
	}, {
		desc: "Valid Juniper LAG",
		v:    ondatra.JUNIPER,
		id:   1001,
		want: "ae1001",
	}, {
		desc:    "Invalid LAG index",
		v:       ondatra.CISCO,
		id:      0,
		wantErr: "LAG index must be >= 1",
	}, {
		desc:    "Unsupported vendor",
		v:       ondatra.IXIA,
		id:      1,
		wantErr: "unsupported vendor: IXIA",
	}}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got, gotErr := LAGName(tt.v, tt.id)
			if (gotErr == nil) != (tt.wantErr == "") || (gotErr != nil && !strings.Contains(gotErr.Error(), tt.wantErr)) {
				t.Errorf("Unexpected error: got %v, want %q", gotErr, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("LAG name invalid: got %q, want %q", got, tt.want)
			}
		})
	}
}
