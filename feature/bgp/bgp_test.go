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

package bgp

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/yang/fpoc"
	"github.com/openconfig/ygot/ygot"
)

var protocolKey = fpoc.NetworkInstance_Protocol_Key{
	Identifier: fpoc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP,
	Name:       "bgp",
}

// TestAugmentNetworkInstance tests the BGP augment to NI OC.
func TestAugmentNetworkInstance(t *testing.T) {
	tests := []struct {
		desc   string
		bgp    *BGP
		inNI   *fpoc.NetworkInstance
		wantNI *fpoc.NetworkInstance
	}{{
		desc: "BGP with no params",
		bgp:  New(),
		inNI: &fpoc.NetworkInstance{},
		wantNI: &fpoc.NetworkInstance{
			Protocol: map[fpoc.NetworkInstance_Protocol_Key]*fpoc.NetworkInstance_Protocol{
				protocolKey: {
					Identifier: fpoc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP,
					Name:       ygot.String("bgp"),
				},
			},
		},
	}, {
		desc: "BGP with AS",
		bgp:  New().WithAS(1234),
		inNI: &fpoc.NetworkInstance{},
		wantNI: &fpoc.NetworkInstance{
			Protocol: map[fpoc.NetworkInstance_Protocol_Key]*fpoc.NetworkInstance_Protocol{
				protocolKey: {
					Identifier: fpoc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP,
					Name:       ygot.String("bgp"),
					Bgp: &fpoc.NetworkInstance_Protocol_Bgp{
						Global: &fpoc.NetworkInstance_Protocol_Bgp_Global{
							As: ygot.Uint32(1234),
						},
					},
				},
			},
		},
	}, {
		desc: "BGP with router-id",
		bgp:  New().WithRouterID("192.0.2.1"),
		inNI: &fpoc.NetworkInstance{},
		wantNI: &fpoc.NetworkInstance{
			Protocol: map[fpoc.NetworkInstance_Protocol_Key]*fpoc.NetworkInstance_Protocol{
				protocolKey: {
					Identifier: fpoc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP,
					Name:       ygot.String("bgp"),
					Bgp: &fpoc.NetworkInstance_Protocol_Bgp{
						Global: &fpoc.NetworkInstance_Protocol_Bgp_Global{
							RouterId: ygot.String("192.0.2.1"),
						},
					},
				},
			},
		},
	}, {
		desc: "BGP with Global AfiSafi",
		bgp:  New().WithAFISAFI(fpoc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST),
		inNI: &fpoc.NetworkInstance{},
		wantNI: &fpoc.NetworkInstance{
			Protocol: map[fpoc.NetworkInstance_Protocol_Key]*fpoc.NetworkInstance_Protocol{
				protocolKey: {
					Identifier: fpoc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP,
					Name:       ygot.String("bgp"),
					Bgp: &fpoc.NetworkInstance_Protocol_Bgp{
						Global: &fpoc.NetworkInstance_Protocol_Bgp_Global{
							AfiSafi: map[fpoc.E_BgpTypes_AFI_SAFI_TYPE]*fpoc.NetworkInstance_Protocol_Bgp_Global_AfiSafi{
								fpoc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST: {
									AfiSafiName: fpoc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST,
									Enabled:     ygot.Bool(true),
								},
							},
						},
					},
				},
			},
		},
	}, {
		desc: "NI contains BGP OC with no conflicts",
		bgp:  New().WithRouterID("192.0.2.1"),
		inNI: &fpoc.NetworkInstance{
			Protocol: map[fpoc.NetworkInstance_Protocol_Key]*fpoc.NetworkInstance_Protocol{
				protocolKey: {
					Identifier: fpoc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP,
					Name:       ygot.String("bgp"),
					Bgp: &fpoc.NetworkInstance_Protocol_Bgp{
						Global: &fpoc.NetworkInstance_Protocol_Bgp_Global{
							As: ygot.Uint32(1234),
						},
					},
				},
			},
		},
		wantNI: &fpoc.NetworkInstance{
			Protocol: map[fpoc.NetworkInstance_Protocol_Key]*fpoc.NetworkInstance_Protocol{
				protocolKey: {
					Identifier: fpoc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP,
					Name:       ygot.String("bgp"),
					Bgp: &fpoc.NetworkInstance_Protocol_Bgp{
						Global: &fpoc.NetworkInstance_Protocol_Bgp_Global{
							As:       ygot.Uint32(1234),
							RouterId: ygot.String("192.0.2.1"),
						},
					},
				},
			},
		},
	}}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			if err := test.bgp.AugmentNetworkInstance(test.inNI); err != nil {
				t.Fatalf("error not expected")
			}
			if diff := cmp.Diff(test.wantNI, test.inNI); diff != "" {
				t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
			}
		})
	}
}

// TestAugmentNetworkInstanceErrors tests the BGP augment to NI OC validation.
func TestAugmentNetworkInstanceErrors(t *testing.T) {
	tests := []struct {
		desc          string
		bgp           *BGP
		inNI          *fpoc.NetworkInstance
		wantErrSubStr string
	}{{
		desc: "NI contains BGP OC with conflicts",
		bgp:  New().WithRouterID("192.0.2.1"),
		inNI: &fpoc.NetworkInstance{
			Protocol: map[fpoc.NetworkInstance_Protocol_Key]*fpoc.NetworkInstance_Protocol{
				protocolKey: {
					Identifier: fpoc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP,
					Name:       ygot.String("bgp"),
					Bgp: &fpoc.NetworkInstance_Protocol_Bgp{
						Global: &fpoc.NetworkInstance_Protocol_Bgp_Global{
							RouterId: ygot.String("192.0.2.2"),
						},
					},
				},
			},
		},
		wantErrSubStr: "destination value was set, but was not equal to source value",
	}}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			err := test.bgp.AugmentNetworkInstance(test.inNI)
			if err == nil {
				t.Fatalf("error expected")
			}
			if !strings.Contains(err.Error(), test.wantErrSubStr) {
				t.Errorf("Error strings don't match: %v", err)
			}
		})
	}
}

type FakeFeature struct {
	Err           error
	augmentCalled bool
	oc            *fpoc.NetworkInstance_Protocol_Bgp
}

func (f *FakeFeature) AugmentBGP(oc *fpoc.NetworkInstance_Protocol_Bgp) error {
	f.oc = oc
	f.augmentCalled = true
	return f.Err
}

// TestWithFeature tests the WithFeature method.
func TestWithFeature(t *testing.T) {
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
		b := New().WithRouterID("192.0.2.1")
		ff := &FakeFeature{Err: test.wantErr}
		gotErr := b.WithFeature(ff)
		if !ff.augmentCalled {
			t.Errorf("AugmentGlobal was not called")
		}
		if ff.oc != b.oc.GetBgp() {
			t.Errorf("BGP ptr is not equal")
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
