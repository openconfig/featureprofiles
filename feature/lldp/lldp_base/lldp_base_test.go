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

package lldpbase

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/yang/oc"
	"github.com/openconfig/ygot/ygot"
)

// TestLLDP tests the features of LLDP config.
func TestLLDP(t *testing.T) {
	tests := []struct {
		desc      string
		lldp      *LLDP
		inDevice  *oc.Device
		outDevice *oc.Device
		wantErr   bool
	}{{
		desc: "LLDP globally enabled",
		lldp: func() *LLDP {
			return New()
		}(),
		inDevice: &oc.Device{},
		outDevice: &oc.Device{
			Lldp: &oc.Lldp{
				Enabled: ygot.Bool(true),
			},
		},
	}, {
		desc: "LLDP with single interface",
		lldp: func() *LLDP {
			return New().WithInterface("Ethernet1.1")
		}(),
		inDevice: &oc.Device{},
		outDevice: &oc.Device{
			Lldp: &oc.Lldp{
				Enabled: ygot.Bool(true),
				Interface: map[string]*oc.Lldp_Interface{
					"Ethernet1.1": &oc.Lldp_Interface{
						Name:    ygot.String("Ethernet1.1"),
						Enabled: ygot.Bool(true),
					},
				},
			},
		},
	}, {
		desc: "LLDP with multiple interfaces",
		lldp: func() *LLDP {
			return New().WithInterface("Ethernet1.1").WithInterface("Ethernet1.2")
		}(),
		inDevice: &oc.Device{},
		outDevice: &oc.Device{
			Lldp: &oc.Lldp{
				Enabled: ygot.Bool(true),
				Interface: map[string]*oc.Lldp_Interface{
					"Ethernet1.1": &oc.Lldp_Interface{
						Name:    ygot.String("Ethernet1.1"),
						Enabled: ygot.Bool(true),
					},
					"Ethernet1.2": &oc.Lldp_Interface{
						Name:    ygot.String("Ethernet1.2"),
						Enabled: ygot.Bool(true),
					},
				},
			},
		},
	}, {
		desc: "Negative test: device already contains lldp so should error",
		lldp: func() *LLDP {
			return New()
		}(),
		inDevice: &oc.Device{
			Lldp: &oc.Lldp{
				Enabled: ygot.Bool(true),
			},
		},
		wantErr: true,
	}}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			err := test.lldp.AugmentDevice(test.inDevice)
			if test.wantErr {
				if err == nil {
					t.Fatalf("error expected")
				}
			} else {
				if err != nil {
					t.Fatalf("error not expected")
				}
				if diff := cmp.Diff(test.outDevice, test.inDevice); diff != "" {
					t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
				}
			}
		})
	}
}
