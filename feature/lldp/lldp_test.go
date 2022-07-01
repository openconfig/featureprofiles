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

package lldp

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/yang/fpoc"
	"github.com/openconfig/ygot/ygot"
)

// TestAugmentDevice tests the features of LLDP config.
func TestAugmentDevice(t *testing.T) {
	tests := []struct {
		desc       string
		lldp       *LLDP
		inDevice   *fpoc.Device
		wantDevice *fpoc.Device
	}{{
		desc:     "LLDP globally enabled",
		lldp:     New(),
		inDevice: &fpoc.Device{},
		wantDevice: &fpoc.Device{
			Lldp: &fpoc.Lldp{
				Enabled: ygot.Bool(true),
			},
		},
	}, {
		desc:     "LLDP with single interface",
		lldp:     New().EnableInterface("Ethernet1.1"),
		inDevice: &fpoc.Device{},
		wantDevice: &fpoc.Device{
			Lldp: &fpoc.Lldp{
				Enabled: ygot.Bool(true),
				Interface: map[string]*fpoc.Lldp_Interface{
					"Ethernet1.1": {
						Name:    ygot.String("Ethernet1.1"),
						Enabled: ygot.Bool(true),
					},
				},
			},
		},
	}, {
		desc:     "LLDP with multiple interfaces",
		lldp:     New().EnableInterface("Ethernet1.1").EnableInterface("Ethernet1.2"),
		inDevice: &fpoc.Device{},
		wantDevice: &fpoc.Device{
			Lldp: &fpoc.Lldp{
				Enabled: ygot.Bool(true),
				Interface: map[string]*fpoc.Lldp_Interface{
					"Ethernet1.1": {
						Name:    ygot.String("Ethernet1.1"),
						Enabled: ygot.Bool(true),
					},
					"Ethernet1.2": {
						Name:    ygot.String("Ethernet1.2"),
						Enabled: ygot.Bool(true),
					},
				},
			},
		},
	}, {
		desc: "Device contains LLDP config, no conflicts",
		lldp: New().EnableInterface("Ethernet1.2"),
		inDevice: &fpoc.Device{
			Lldp: &fpoc.Lldp{
				Enabled: ygot.Bool(true),
				Interface: map[string]*fpoc.Lldp_Interface{
					"Ethernet1.1": {
						Name:    ygot.String("Ethernet1.1"),
						Enabled: ygot.Bool(true),
					},
				},
			},
		},
		wantDevice: &fpoc.Device{
			Lldp: &fpoc.Lldp{
				Enabled: ygot.Bool(true),
				Interface: map[string]*fpoc.Lldp_Interface{
					"Ethernet1.1": {
						Name:    ygot.String("Ethernet1.1"),
						Enabled: ygot.Bool(true),
					},
					"Ethernet1.2": {
						Name:    ygot.String("Ethernet1.2"),
						Enabled: ygot.Bool(true),
					},
				},
			},
		},
	}}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			if err := test.lldp.AugmentDevice(test.inDevice); err != nil {
				t.Fatalf("error not expected")
			}
			if diff := cmp.Diff(test.wantDevice, test.inDevice); diff != "" {
				t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
			}
		})
	}
}

// TestAugmentDeviceErrors tests the error handling of AugmentDevice.
func TestAugmentDeviceErrors(t *testing.T) {
	tests := []struct {
		desc          string
		lldp          *LLDP
		inDevice      *fpoc.Device
		wantErrSubStr string
	}{{
		desc: "Device contains LLDP config with conflict",
		lldp: New().EnableInterface("Ethernet1.1"),
		inDevice: &fpoc.Device{
			Lldp: &fpoc.Lldp{
				Enabled: ygot.Bool(true),
				Interface: map[string]*fpoc.Lldp_Interface{
					"Ethernet1.1": {
						Name:    ygot.String("Ethernet1.1"),
						Enabled: ygot.Bool(false),
					},
				},
			},
		},
		wantErrSubStr: "destination value was set",
	}}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			err := test.lldp.AugmentDevice(test.inDevice)
			if err == nil {
				t.Fatalf("error expected")
			}
			if !strings.Contains(err.Error(), test.wantErrSubStr) {
				t.Errorf("Error sub-string does not match: %v", err)
			}
		})
	}
}
