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

var protocolKey = fpoc.NetworkInstance_Protocol_Key{
	Identifier: fpoc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS,
	Name:       "isis",
}

// TestAugmentNetworkInstance tests the ISIS augment to NI OC.
func TestAugmentNetworkInstance(t *testing.T) {
	tests := []struct {
		desc   string
		isis   *ISIS
		inNI   *fpoc.NetworkInstance
		wantNI *fpoc.NetworkInstance
	}{{
		desc: "ISIS with no params",
		isis: New(),
		inNI: &fpoc.NetworkInstance{},
		wantNI: &fpoc.NetworkInstance{
			Protocol: map[fpoc.NetworkInstance_Protocol_Key]*fpoc.NetworkInstance_Protocol{
				protocolKey: {
					Identifier: fpoc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS,
					Name:       ygot.String("isis"),
				},
			},
		},
	}, {
		desc: "ISIS with Net",
		isis: New().WithNet("50.3131.3131.3131.00"),
		inNI: &fpoc.NetworkInstance{},
		wantNI: &fpoc.NetworkInstance{
			Protocol: map[fpoc.NetworkInstance_Protocol_Key]*fpoc.NetworkInstance_Protocol{
				protocolKey: {
					Identifier: fpoc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS,
					Name:       ygot.String("isis"),
					Isis: &fpoc.NetworkInstance_Protocol_Isis{
						Global: &fpoc.NetworkInstance_Protocol_Isis_Global{
							Net: []string{"50.3131.3131.3131.00"},
						},
					},
				},
			},
		},
	}, {
		desc: "ISIS with Global AfiSafi",
		isis: New().WithAFISAFI(fpoc.IsisTypes_AFI_TYPE_IPV4, fpoc.IsisTypes_SAFI_TYPE_UNICAST),
		inNI: &fpoc.NetworkInstance{},
		wantNI: &fpoc.NetworkInstance{
			Protocol: map[fpoc.NetworkInstance_Protocol_Key]*fpoc.NetworkInstance_Protocol{
				protocolKey: {
					Identifier: fpoc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS,
					Name:       ygot.String("isis"),
					Isis: &fpoc.NetworkInstance_Protocol_Isis{
						Global: &fpoc.NetworkInstance_Protocol_Isis_Global{
							Af: map[fpoc.NetworkInstance_Protocol_Isis_Global_Af_Key]*fpoc.NetworkInstance_Protocol_Isis_Global_Af{
								{AfiName: fpoc.IsisTypes_AFI_TYPE_IPV4, SafiName: fpoc.IsisTypes_SAFI_TYPE_UNICAST}: {
									AfiName:  fpoc.IsisTypes_AFI_TYPE_IPV4,
									SafiName: fpoc.IsisTypes_SAFI_TYPE_UNICAST,
									Enabled:  ygot.Bool(true),
								},
							},
						},
					},
				},
			},
		},
	}, {
		desc: "ISIS with Level Capability",
		isis: New().WithLevelCapability(fpoc.IsisTypes_LevelType_LEVEL_1_2),
		inNI: &fpoc.NetworkInstance{},
		wantNI: &fpoc.NetworkInstance{
			Protocol: map[fpoc.NetworkInstance_Protocol_Key]*fpoc.NetworkInstance_Protocol{
				protocolKey: {
					Identifier: fpoc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS,
					Name:       ygot.String("isis"),
					Isis: &fpoc.NetworkInstance_Protocol_Isis{
						Global: &fpoc.NetworkInstance_Protocol_Isis_Global{
							LevelCapability: fpoc.IsisTypes_LevelType_LEVEL_1_2,
						},
					},
				},
			},
		},
	}, {
		desc: "ISIS with LSP MTU size",
		isis: New().WithLSPMTUSize(9012),
		inNI: &fpoc.NetworkInstance{},
		wantNI: &fpoc.NetworkInstance{
			Protocol: map[fpoc.NetworkInstance_Protocol_Key]*fpoc.NetworkInstance_Protocol{
				protocolKey: {
					Identifier: fpoc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS,
					Name:       ygot.String("isis"),
					Isis: &fpoc.NetworkInstance_Protocol_Isis{
						Global: &fpoc.NetworkInstance_Protocol_Isis_Global{
							Transport: &fpoc.NetworkInstance_Protocol_Isis_Global_Transport{
								LspMtuSize: ygot.Uint16(9012),
							},
						},
					},
				},
			},
		},
	}, {
		desc: "ISIS with LSP Lifetime Interval",
		isis: New().WithLSPLifetimeInterval(10 * time.Second),
		inNI: &fpoc.NetworkInstance{},
		wantNI: &fpoc.NetworkInstance{
			Protocol: map[fpoc.NetworkInstance_Protocol_Key]*fpoc.NetworkInstance_Protocol{
				protocolKey: {
					Identifier: fpoc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS,
					Name:       ygot.String("isis"),
					Isis: &fpoc.NetworkInstance_Protocol_Isis{
						Global: &fpoc.NetworkInstance_Protocol_Isis_Global{
							Timers: &fpoc.NetworkInstance_Protocol_Isis_Global_Timers{
								LspLifetimeInterval: ygot.Uint16(10),
							},
						},
					},
				},
			},
		},
	}, {
		desc: "ISIS with LSP Refresh Interval",
		isis: New().WithLSPRefreshInterval(10 * time.Second),
		inNI: &fpoc.NetworkInstance{},
		wantNI: &fpoc.NetworkInstance{
			Protocol: map[fpoc.NetworkInstance_Protocol_Key]*fpoc.NetworkInstance_Protocol{
				protocolKey: {
					Identifier: fpoc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS,
					Name:       ygot.String("isis"),
					Isis: &fpoc.NetworkInstance_Protocol_Isis{
						Global: &fpoc.NetworkInstance_Protocol_Isis_Global{
							Timers: &fpoc.NetworkInstance_Protocol_Isis_Global_Timers{
								LspRefreshInterval: ygot.Uint16(10),
							},
						},
					},
				},
			},
		},
	}, {
		desc: "ISIS with SPF First Interval",
		isis: New().WithSPFFirstInterval(10 * time.Millisecond),
		inNI: &fpoc.NetworkInstance{},
		wantNI: &fpoc.NetworkInstance{
			Protocol: map[fpoc.NetworkInstance_Protocol_Key]*fpoc.NetworkInstance_Protocol{
				protocolKey: {
					Identifier: fpoc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS,
					Name:       ygot.String("isis"),
					Isis: &fpoc.NetworkInstance_Protocol_Isis{
						Global: &fpoc.NetworkInstance_Protocol_Isis_Global{
							Timers: &fpoc.NetworkInstance_Protocol_Isis_Global_Timers{
								Spf: &fpoc.NetworkInstance_Protocol_Isis_Global_Timers_Spf{
									SpfFirstInterval: ygot.Uint64(10),
								},
							},
						},
					},
				},
			},
		},
	}, {
		desc: "ISIS with SPF Hold Interval",
		isis: New().WithSPFHoldInterval(10 * time.Millisecond),
		inNI: &fpoc.NetworkInstance{},
		wantNI: &fpoc.NetworkInstance{
			Protocol: map[fpoc.NetworkInstance_Protocol_Key]*fpoc.NetworkInstance_Protocol{
				protocolKey: {
					Identifier: fpoc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS,
					Name:       ygot.String("isis"),
					Isis: &fpoc.NetworkInstance_Protocol_Isis{
						Global: &fpoc.NetworkInstance_Protocol_Isis_Global{
							Timers: &fpoc.NetworkInstance_Protocol_Isis_Global_Timers{
								Spf: &fpoc.NetworkInstance_Protocol_Isis_Global_Timers_Spf{
									SpfHoldInterval: ygot.Uint64(10),
								},
							},
						},
					},
				},
			},
		},
	}, {
		desc: "NI contains ISIS OC with no conflicts",
		isis: New().WithNet("50.3131.3131.3131.00"),
		inNI: &fpoc.NetworkInstance{
			Protocol: map[fpoc.NetworkInstance_Protocol_Key]*fpoc.NetworkInstance_Protocol{
				protocolKey: {
					Identifier: fpoc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS,
					Name:       ygot.String("isis"),
					Isis: &fpoc.NetworkInstance_Protocol_Isis{
						Global: &fpoc.NetworkInstance_Protocol_Isis_Global{
							Net: []string{"50.3131.3131.3131.00"},
						},
					},
				},
			},
		},
		wantNI: &fpoc.NetworkInstance{
			Protocol: map[fpoc.NetworkInstance_Protocol_Key]*fpoc.NetworkInstance_Protocol{
				protocolKey: {
					Identifier: fpoc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS,
					Name:       ygot.String("isis"),
					Isis: &fpoc.NetworkInstance_Protocol_Isis{
						Global: &fpoc.NetworkInstance_Protocol_Isis_Global{
							Net: []string{"50.3131.3131.3131.00"},
						},
					},
				},
			},
		},
	}}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			if err := test.isis.AugmentNetworkInstance(test.inNI); err != nil {
				t.Fatalf("error not expected")
			}
			if diff := cmp.Diff(test.wantNI, test.inNI); diff != "" {
				t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
			}
		})
	}
}

// TestAugmentNetworkInstanceErrors tests the ISIS augment to NI OC validation.
func TestAugmentNetworkInstanceErrors(t *testing.T) {
	tests := []struct {
		desc          string
		isis          *ISIS
		inNI          *fpoc.NetworkInstance
		wantErrSubStr string
	}{{
		desc: "NI contains ISIS OC with conflicts",
		isis: New().WithLSPMTUSize(9011),
		inNI: &fpoc.NetworkInstance{
			Protocol: map[fpoc.NetworkInstance_Protocol_Key]*fpoc.NetworkInstance_Protocol{
				protocolKey: {
					Identifier: fpoc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS,
					Name:       ygot.String("isis"),
					Isis: &fpoc.NetworkInstance_Protocol_Isis{
						Global: &fpoc.NetworkInstance_Protocol_Isis_Global{
							Transport: &fpoc.NetworkInstance_Protocol_Isis_Global_Transport{
								LspMtuSize: ygot.Uint16(9012),
							},
						},
					},
				},
			},
		},
		wantErrSubStr: "destination value was set, but was not equal to source value",
	}}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			err := test.isis.AugmentNetworkInstance(test.inNI)
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
	oc            *fpoc.NetworkInstance_Protocol_Isis
}

func (f *FakeFeature) AugmentISIS(oc *fpoc.NetworkInstance_Protocol_Isis) error {
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
		b := New().WithNet("50.3131.3131.3131.00")
		ff := &FakeFeature{Err: test.wantErr}
		gotErr := b.WithFeature(ff)
		if !ff.augmentCalled {
			t.Errorf("AugmentGlobal was not called")
		}
		if ff.oc != b.oc.GetIsis() {
			t.Errorf("ISIS ptr is not equal")
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
