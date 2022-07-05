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

package ntp

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/yang/fpoc"
	"github.com/openconfig/ygot/ygot"
)

// TestAugmentSystem tests the NTP augment to System OC.
func TestAugmentSystem(t *testing.T) {
	tests := []struct {
		desc       string
		ntp        *NTP
		inSystem   *fpoc.System
		wantSystem *fpoc.System
	}{{
		desc:     "NTP enabled with no params",
		ntp:      New(),
		inSystem: &fpoc.System{},
		wantSystem: &fpoc.System{
			Ntp: &fpoc.System_Ntp{
				Enabled: ygot.Bool(true),
			},
		},
	}, {
		desc:     "Add one server",
		ntp:      New().AddServer("192.0.2.1", 1234),
		inSystem: &fpoc.System{},
		wantSystem: &fpoc.System{
			Ntp: &fpoc.System_Ntp{
				Enabled: ygot.Bool(true),
				Server: map[string]*fpoc.System_Ntp_Server{
					"192.0.2.1": {
						Address: ygot.String("192.0.2.1"),
						Port:    ygot.Uint16(1234),
					},
				},
			},
		},
	}, {
		desc:     "Add multiple servers",
		ntp:      New().AddServer("192.0.2.1", 1234).AddServer("192.0.2.2", 1234),
		inSystem: &fpoc.System{},
		wantSystem: &fpoc.System{
			Ntp: &fpoc.System_Ntp{
				Enabled: ygot.Bool(true),
				Server: map[string]*fpoc.System_Ntp_Server{
					"192.0.2.1": {
						Address: ygot.String("192.0.2.1"),
						Port:    ygot.Uint16(1234),
					},
					"192.0.2.2": {
						Address: ygot.String("192.0.2.2"),
						Port:    ygot.Uint16(1234),
					},
				},
			},
		},
	}, {
		desc: "Add non-conflicting servers",
		ntp:  New().AddServer("192.0.2.1", 1234),
		inSystem: &fpoc.System{
			Ntp: &fpoc.System_Ntp{
				Enabled: ygot.Bool(true),
				Server: map[string]*fpoc.System_Ntp_Server{
					"192.0.2.2": {
						Address: ygot.String("192.0.2.2"),
						Port:    ygot.Uint16(1234),
					},
				},
			},
		},
		wantSystem: &fpoc.System{
			Ntp: &fpoc.System_Ntp{
				Enabled: ygot.Bool(true),
				Server: map[string]*fpoc.System_Ntp_Server{
					"192.0.2.1": {
						Address: ygot.String("192.0.2.1"),
						Port:    ygot.Uint16(1234),
					},
					"192.0.2.2": {
						Address: ygot.String("192.0.2.2"),
						Port:    ygot.Uint16(1234),
					},
				},
			},
		},
	}, {
		desc:     "Add same server twice",
		ntp:      New().AddServer("192.0.2.1", 1234).AddServer("192.0.2.1", 1234),
		inSystem: &fpoc.System{},
		wantSystem: &fpoc.System{
			Ntp: &fpoc.System_Ntp{
				Enabled: ygot.Bool(true),
				Server: map[string]*fpoc.System_Ntp_Server{
					"192.0.2.1": {
						Address: ygot.String("192.0.2.1"),
						Port:    ygot.Uint16(1234),
					},
				},
			},
		},
	}}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			err := test.ntp.AugmentSystem(test.inSystem)
			if err != nil {
				t.Fatalf("Error not expected: %v", err)
			}
			if diff := cmp.Diff(test.wantSystem, test.inSystem); diff != "" {
				t.Errorf("Did not get expected state, diff(-want,+got):\n%s", diff)
			}
		})
	}
}

// TestAugmentSystemErrors tests the NTP augment to System errors.
func TestAugmentSystemErrors(t *testing.T) {
	tests := []struct {
		desc          string
		ntp           *NTP
		inSystem      *fpoc.System
		wantErrSubStr string
	}{{
		desc: "System contains NTP with conflicts",
		ntp:  New().AddServer("192.0.2.1", 1234),
		inSystem: &fpoc.System{
			Ntp: &fpoc.System_Ntp{
				Enabled: ygot.Bool(true),
				Server: map[string]*fpoc.System_Ntp_Server{
					"192.0.2.1": {
						Address: ygot.String("192.0.2.1"),
						Port:    ygot.Uint16(1235),
					},
				},
			},
		},
		wantErrSubStr: "destination value was set",
	}}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			err := test.ntp.AugmentSystem(test.inSystem)
			if err == nil {
				t.Fatalf("Error expected")
			}
			if !strings.Contains(err.Error(), test.wantErrSubStr) {
				t.Errorf("Error strings are not equal: %v", err)
			}
		})
	}
}
