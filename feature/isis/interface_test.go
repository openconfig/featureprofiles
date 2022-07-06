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

package isis

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/yang/fpoc"
	"github.com/openconfig/ygot/ygot"
)

// TestInterfaceAugmentGlobal tests the ISIS interface augment to ISIS global
// OC.
func TestInterfaceAugmentGlobal(t *testing.T) {
	tests := []struct {
		desc     string
		intf     *Interface
		inISIS   *fpoc.NetworkInstance_Protocol_Isis
		wantISIS *fpoc.NetworkInstance_Protocol_Isis
	}{{
		desc:   "ISIS interface with no params",
		intf:   NewInterface("Ethernet1"),
		inISIS: &fpoc.NetworkInstance_Protocol_Isis{},
		wantISIS: &fpoc.NetworkInstance_Protocol_Isis{
			Interface: map[string]*fpoc.NetworkInstance_Protocol_Isis_Interface{
				"Ethernet1": {
					Enabled:     ygot.Bool(true),
					InterfaceId: ygot.String("Ethernet1"),
				},
			},
		},
	}, {
		desc:   "ISIS interface with circuit-type",
		intf:   NewInterface("Ethernet1").WithCircuitType(fpoc.IsisTypes_CircuitType_POINT_TO_POINT),
		inISIS: &fpoc.NetworkInstance_Protocol_Isis{},
		wantISIS: &fpoc.NetworkInstance_Protocol_Isis{
			Interface: map[string]*fpoc.NetworkInstance_Protocol_Isis_Interface{
				"Ethernet1": {
					Enabled:     ygot.Bool(true),
					InterfaceId: ygot.String("Ethernet1"),
					CircuitType: fpoc.IsisTypes_CircuitType_POINT_TO_POINT,
				},
			},
		},
	}, {
		desc:   "ISIS interface with csnp-interval",
		intf:   NewInterface("Ethernet1").WithCSNPInterval(10 * time.Second),
		inISIS: &fpoc.NetworkInstance_Protocol_Isis{},
		wantISIS: &fpoc.NetworkInstance_Protocol_Isis{
			Interface: map[string]*fpoc.NetworkInstance_Protocol_Isis_Interface{
				"Ethernet1": {
					Enabled:     ygot.Bool(true),
					InterfaceId: ygot.String("Ethernet1"),
					Timers: &fpoc.NetworkInstance_Protocol_Isis_Interface_Timers{
						CsnpInterval: ygot.Uint16(10),
					},
				},
			},
		},
	}, {
		desc:   "ISIS interface with lsp-pacing-interval",
		intf:   NewInterface("Ethernet1").WithLSPPacingInterval(10 * time.Millisecond),
		inISIS: &fpoc.NetworkInstance_Protocol_Isis{},
		wantISIS: &fpoc.NetworkInstance_Protocol_Isis{
			Interface: map[string]*fpoc.NetworkInstance_Protocol_Isis_Interface{
				"Ethernet1": {
					Enabled:     ygot.Bool(true),
					InterfaceId: ygot.String("Ethernet1"),
					Timers: &fpoc.NetworkInstance_Protocol_Isis_Interface_Timers{
						LspPacingInterval: ygot.Uint64(10),
					},
				},
			},
		},
	}, {
		desc:   "ISIS interface with AfiSafi",
		intf:   NewInterface("Ethernet1").WithAFISAFI(fpoc.IsisTypes_AFI_TYPE_IPV4, fpoc.IsisTypes_SAFI_TYPE_UNICAST),
		inISIS: &fpoc.NetworkInstance_Protocol_Isis{},
		wantISIS: &fpoc.NetworkInstance_Protocol_Isis{
			Interface: map[string]*fpoc.NetworkInstance_Protocol_Isis_Interface{
				"Ethernet1": {
					Enabled:     ygot.Bool(true),
					InterfaceId: ygot.String("Ethernet1"),
					Af: map[fpoc.NetworkInstance_Protocol_Isis_Interface_Af_Key]*fpoc.NetworkInstance_Protocol_Isis_Interface_Af{
						{AfiName: fpoc.IsisTypes_AFI_TYPE_IPV4, SafiName: fpoc.IsisTypes_SAFI_TYPE_UNICAST}: {
							AfiName:  fpoc.IsisTypes_AFI_TYPE_IPV4,
							SafiName: fpoc.IsisTypes_SAFI_TYPE_UNICAST,
							Enabled:  ygot.Bool(true),
						},
					},
				},
			},
		},
	}, {
		desc: "ISIS contains Interface OC with no conflicts",
		intf: NewInterface("Ethernet1").WithLSPPacingInterval(10 * time.Millisecond),
		inISIS: &fpoc.NetworkInstance_Protocol_Isis{
			Interface: map[string]*fpoc.NetworkInstance_Protocol_Isis_Interface{
				"Ethernet1": {
					Enabled:     ygot.Bool(true),
					InterfaceId: ygot.String("Ethernet1"),
					Timers: &fpoc.NetworkInstance_Protocol_Isis_Interface_Timers{
						LspPacingInterval: ygot.Uint64(10),
					},
				},
			},
		},
		wantISIS: &fpoc.NetworkInstance_Protocol_Isis{
			Interface: map[string]*fpoc.NetworkInstance_Protocol_Isis_Interface{
				"Ethernet1": {
					Enabled:     ygot.Bool(true),
					InterfaceId: ygot.String("Ethernet1"),
					Timers: &fpoc.NetworkInstance_Protocol_Isis_Interface_Timers{
						LspPacingInterval: ygot.Uint64(10),
					},
				},
			},
		},
	}}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			if err := test.intf.AugmentGlobal(test.inISIS); err != nil {
				t.Fatalf("error not expected")
			}
			if diff := cmp.Diff(test.wantISIS, test.inISIS); diff != "" {
				t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
			}
		})
	}
}

// TestInterfaceAugmentGlobalErrors tests the ISIS interface augment to ISIS
// OC validation.
func TestInterfaceAugmentGlobalErrors(t *testing.T) {
	tests := []struct {
		desc          string
		intf          *Interface
		inISIS        *fpoc.NetworkInstance_Protocol_Isis
		wantErrSubStr string
	}{{
		desc: "ISIS contains Interface OC with conflicts",
		intf: NewInterface("Ethernet1").WithLSPPacingInterval(10 * time.Millisecond),
		inISIS: &fpoc.NetworkInstance_Protocol_Isis{
			Interface: map[string]*fpoc.NetworkInstance_Protocol_Isis_Interface{
				"Ethernet1": {
					Enabled:     ygot.Bool(true),
					InterfaceId: ygot.String("Ethernet1"),
					Timers: &fpoc.NetworkInstance_Protocol_Isis_Interface_Timers{
						LspPacingInterval: ygot.Uint64(11),
					},
				},
			},
		},
		wantErrSubStr: "destination value was set, but was not equal to source value",
	}}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			err := test.intf.AugmentGlobal(test.inISIS)
			if err == nil {
				t.Fatalf("error expected")
			}
			if !strings.Contains(err.Error(), test.wantErrSubStr) {
				t.Errorf("Error strings don't match: %v", err)
			}
		})
	}
}

type InterfaceFakeFeature struct {
	Err           error
	augmentCalled bool
	oc            *fpoc.NetworkInstance_Protocol_Isis_Interface
}

func (f *InterfaceFakeFeature) AugmentInterface(oc *fpoc.NetworkInstance_Protocol_Isis_Interface) error {
	f.oc = oc
	f.augmentCalled = true
	return f.Err
}

// TestInterfaceWithFeature tests the WithFeature method.
func TestInterfaceWithFeature(t *testing.T) {
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
		b := NewInterface("Ethernet1").WithLSPPacingInterval(10 * time.Second)
		ff := &InterfaceFakeFeature{Err: test.wantErr}
		gotErr := b.WithFeature(ff)
		if !ff.augmentCalled {
			t.Errorf("AugmentISIS was not called")
		}
		if ff.oc != &b.oc {
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
