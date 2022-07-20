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

package intf

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/yang/fpoc"
	"github.com/openconfig/ygot/ygot"
)

// TestAugmentInterface tests the features of SubInterface config.
func TestAugmentInterface(t *testing.T) {
	tests := []struct {
		desc     string
		subintf  *SubInterface
		inIntf   *fpoc.Interface
		wantIntf *fpoc.Interface
	}{{
		desc:    "New Ethernet sub-interface",
		subintf: NewSubInterface(1, "Ethernet sub-interface"),
		inIntf:  &fpoc.Interface{},
		wantIntf: &fpoc.Interface{
			Subinterface: map[uint32]*fpoc.Interface_Subinterface{
				1: {
					Index:       ygot.Uint32(uint32(1)),
					Description: ygot.String("Ethernet sub-interface"),
					Enabled:     ygot.Bool(true),
				},
			},
		},
	}, {
		desc:    "ipv4 enabled",
		subintf: NewSubInterface(1, "Ethernet sub-interface").WithIPv4Enabled(true),
		inIntf:  &fpoc.Interface{},
		wantIntf: &fpoc.Interface{
			Subinterface: map[uint32]*fpoc.Interface_Subinterface{
				1: {
					Index:       ygot.Uint32(uint32(1)),
					Description: ygot.String("Ethernet sub-interface"),
					Enabled:     ygot.Bool(true),
					Ipv4: &fpoc.Interface_Subinterface_Ipv4{
						Enabled: ygot.Bool(true),
					},
				},
			},
		},
	}, {
		desc:    "ipv4 mtu",
		subintf: NewSubInterface(1, "Ethernet sub-interface").WithIPv4MTU(9012),
		inIntf:  &fpoc.Interface{},
		wantIntf: &fpoc.Interface{
			Subinterface: map[uint32]*fpoc.Interface_Subinterface{
				1: {
					Index:       ygot.Uint32(uint32(1)),
					Description: ygot.String("Ethernet sub-interface"),
					Enabled:     ygot.Bool(true),
					Ipv4: &fpoc.Interface_Subinterface_Ipv4{
						Mtu: ygot.Uint16(uint16(9012)),
					},
				},
			},
		},
	}, {
		desc:    "ipv6 enabled",
		subintf: NewSubInterface(1, "Ethernet sub-interface").WithIPv6Enabled(true),
		inIntf:  &fpoc.Interface{},
		wantIntf: &fpoc.Interface{
			Subinterface: map[uint32]*fpoc.Interface_Subinterface{
				1: {
					Index:       ygot.Uint32(uint32(1)),
					Description: ygot.String("Ethernet sub-interface"),
					Enabled:     ygot.Bool(true),
					Ipv6: &fpoc.Interface_Subinterface_Ipv6{
						Enabled: ygot.Bool(true),
					},
				},
			},
		},
	}, {
		desc:    "ipv6 mtu",
		subintf: NewSubInterface(1, "Ethernet sub-interface").WithIPv6MTU(9012),
		inIntf:  &fpoc.Interface{},
		wantIntf: &fpoc.Interface{
			Subinterface: map[uint32]*fpoc.Interface_Subinterface{
				1: {
					Index:       ygot.Uint32(uint32(1)),
					Description: ygot.String("Ethernet sub-interface"),
					Enabled:     ygot.Bool(true),
					Ipv6: &fpoc.Interface_Subinterface_Ipv6{
						Mtu: ygot.Uint32(uint32(9012)),
					},
				},
			},
		},
	}, {
		desc:    "Device contains SubInterface config, no conflicts",
		subintf: NewSubInterface(1, "Ethernet sub-interface").WithIPv6MTU(9012),
		inIntf: &fpoc.Interface{
			Subinterface: map[uint32]*fpoc.Interface_Subinterface{
				1: {
					Index:       ygot.Uint32(uint32(1)),
					Description: ygot.String("Ethernet sub-interface"),
					Enabled:     ygot.Bool(true),
					Ipv6: &fpoc.Interface_Subinterface_Ipv6{
						Mtu: ygot.Uint32(uint32(9012)),
					},
				},
			},
		},
		wantIntf: &fpoc.Interface{
			Subinterface: map[uint32]*fpoc.Interface_Subinterface{
				1: {
					Index:       ygot.Uint32(uint32(1)),
					Description: ygot.String("Ethernet sub-interface"),
					Enabled:     ygot.Bool(true),
					Ipv6: &fpoc.Interface_Subinterface_Ipv6{
						Mtu: ygot.Uint32(uint32(9012)),
					},
				},
			},
		},
	}}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			if err := test.subintf.AugmentInterface(test.inIntf); err != nil {
				t.Fatalf("error not expected: %v", err)
			}
			if diff := cmp.Diff(test.wantIntf, test.inIntf); diff != "" {
				t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
			}
		})
	}
}

// TestAugmentInterfaceErrors tests the error handling of AugmentInterface.
func TestAugmentInterfaceErrors(t *testing.T) {
	tests := []struct {
		desc          string
		subintf       *SubInterface
		inIntf        *fpoc.Interface
		wantErrSubStr string
	}{{
		desc:    "Device contains SubInterface config, no conflicts",
		subintf: NewSubInterface(1, "Ethernet sub-interface").WithIPv6MTU(9012),
		inIntf: &fpoc.Interface{
			Subinterface: map[uint32]*fpoc.Interface_Subinterface{
				1: {
					Index:       ygot.Uint32(uint32(1)),
					Description: ygot.String("Ethernet sub-interface"),
					Enabled:     ygot.Bool(true),
					Ipv6: &fpoc.Interface_Subinterface_Ipv6{
						Mtu: ygot.Uint32(uint32(9013)),
					},
				},
			},
		},
		wantErrSubStr: "destination value was set",
	}}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			err := test.subintf.AugmentInterface(test.inIntf)
			if err == nil {
				t.Fatalf("error expected")
			}
			if !strings.Contains(err.Error(), test.wantErrSubStr) {
				t.Errorf("Error sub-string does not match: %v", err)
			}
		})
	}
}

type FakeSIFeature struct {
	Err           error
	augmentCalled bool
	oc            *fpoc.Interface_Subinterface
}

func (f *FakeSIFeature) AugmentSubInterface(oc *fpoc.Interface_Subinterface) error {
	f.oc = oc
	f.augmentCalled = true
	return f.Err
}

// TestSubInterfaceWithFeature tests the WithFeature method.
func TestSubInterfaceWithFeature(t *testing.T) {
	tests := []struct {
		desc    string
		wantErr error
	}{{
		desc: "error not expected",
	}, {
		desc:    "error expected",
		wantErr: errors.New("some error"),
	}}

	for _, test := range tests {
		i := NewSubInterface(1, "Ethernet sub-interface").WithIPv6MTU(9012)
		ff := &FakeSIFeature{Err: test.wantErr}
		gotErr := i.WithFeature(ff)
		if !ff.augmentCalled {
			t.Errorf("AugmentSubInterface was not called")
		}
		if ff.oc != &i.oc {
			t.Errorf("Interface ptr is not equal")
		}
		if test.wantErr != nil {
			if gotErr != nil {
				if !strings.Contains(gotErr.Error(), test.wantErr.Error()) {
					t.Errorf("Error strings are not equal: %v", gotErr)
				}
			}
			if gotErr == nil {
				t.Errorf("Expecting error but got none")
			}
		}
	}
}
