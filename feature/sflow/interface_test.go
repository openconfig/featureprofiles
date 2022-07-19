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

package sflow

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/yang/fpoc"
	"github.com/openconfig/ygot/ygot"
)

// TestInterfaceAugmentSflow tests the interface augment to Sflow.
func TestInterfaceAugmentSflow(t *testing.T) {
	tests := []struct {
		desc      string
		intf      *Interface
		inSflow   *fpoc.Sampling_Sflow
		wantSflow *fpoc.Sampling_Sflow
	}{{
		desc:    "Interface with no params",
		intf:    NewInterface("Ethernet1"),
		inSflow: &fpoc.Sampling_Sflow{},
		wantSflow: &fpoc.Sampling_Sflow{
			Interface: map[string]*fpoc.Sampling_Sflow_Interface{
				"Ethernet1": {
					Name:    ygot.String("Ethernet1"),
					Enabled: ygot.Bool(true),
				},
			},
		},
	}, {
		desc: "Sflow already contains interface with no conflicts",
		intf: NewInterface("Ethernet1"),
		inSflow: &fpoc.Sampling_Sflow{
			Interface: map[string]*fpoc.Sampling_Sflow_Interface{
				"Ethernet1": {
					Name:    ygot.String("Ethernet1"),
					Enabled: ygot.Bool(true),
				},
			},
		},
		wantSflow: &fpoc.Sampling_Sflow{
			Interface: map[string]*fpoc.Sampling_Sflow_Interface{
				"Ethernet1": {
					Name:    ygot.String("Ethernet1"),
					Enabled: ygot.Bool(true),
				},
			},
		},
	}}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			if err := test.intf.AugmentSflow(test.inSflow); err != nil {
				t.Fatalf("error not expected: %v", err)
			}
			if diff := cmp.Diff(test.wantSflow, test.inSflow); diff != "" {
				t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
			}
		})
	}
}

// TestInterfaceAugmentSflowErrors tests the interface augment to Sflow validation.
func TestInterfaceAugmentSflowErrors(t *testing.T) {
	tests := []struct {
		desc          string
		intf          *Interface
		inSflow       *fpoc.Sampling_Sflow
		wantErrSubStr string
	}{{
		desc: "Neighbor already exists but with conflicts",
		intf: NewInterface("Ethernet1"),
		inSflow: &fpoc.Sampling_Sflow{
			Interface: map[string]*fpoc.Sampling_Sflow_Interface{
				"Ethernet1": {
					Name:    ygot.String("Ethernet1"),
					Enabled: ygot.Bool(false),
				},
			},
		},
		wantErrSubStr: "destination value was set, but was not equal to source value",
	}}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			err := test.intf.AugmentSflow(test.inSflow)
			if err == nil {
				t.Fatalf("error expected")
			}
			if !strings.Contains(err.Error(), test.wantErrSubStr) {
				t.Errorf("Error string does not match: %v", err)
			}
		})
	}
}
