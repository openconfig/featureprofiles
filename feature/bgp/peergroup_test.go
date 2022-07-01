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
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/yang/fpoc"
	"github.com/openconfig/ygot/ygot"
)

// TestName tests the Name method.
func TestName(t *testing.T) {
	n := NewPeerGroup("GLOBAL-PEER")
	if got, want := n.Name(), "GLOBAL-PEER"; got != want {
		t.Errorf("got %v but expecting %v", got, want)
	}
}

// TestPGAugmentBGP tests the BGP pg augment to BGP pg.
func TestPGAugmentBGP(t *testing.T) {
	tests := []struct {
		desc    string
		pg      *PeerGroup
		inBGP   *fpoc.NetworkInstance_Protocol_Bgp
		wantBGP *fpoc.NetworkInstance_Protocol_Bgp
	}{{
		desc:  "PeerGroup with no params",
		pg:    NewPeerGroup("GLOBAL-PEER"),
		inBGP: &fpoc.NetworkInstance_Protocol_Bgp{},
		wantBGP: &fpoc.NetworkInstance_Protocol_Bgp{
			PeerGroup: map[string]*fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
				"GLOBAL-PEER": {
					PeerGroupName: ygot.String("GLOBAL-PEER"),
				},
			},
		},
	}, {
		desc:  "PeerGroup with AFI-SAFI",
		pg:    NewPeerGroup("GLOBAL-PEER").WithAFISAFI(fpoc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST),
		inBGP: &fpoc.NetworkInstance_Protocol_Bgp{},
		wantBGP: &fpoc.NetworkInstance_Protocol_Bgp{
			PeerGroup: map[string]*fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
				"GLOBAL-PEER": {
					PeerGroupName: ygot.String("GLOBAL-PEER"),
					AfiSafi: map[fpoc.E_BgpTypes_AFI_SAFI_TYPE]*fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AfiSafi{
						fpoc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST: {
							AfiSafiName: fpoc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST,
							Enabled:     ygot.Bool(true),
						},
					},
				},
			},
		},
	}, {
		desc:  "PeerGroup with auth-password",
		pg:    NewPeerGroup("GLOBAL-PEER").WithAuthPassword("password"),
		inBGP: &fpoc.NetworkInstance_Protocol_Bgp{},
		wantBGP: &fpoc.NetworkInstance_Protocol_Bgp{
			PeerGroup: map[string]*fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
				"GLOBAL-PEER": {
					PeerGroupName: ygot.String("GLOBAL-PEER"),
					AuthPassword:  ygot.String("password"),
				},
			},
		},
	}, {
		desc:  "PeerGroup with description",
		pg:    NewPeerGroup("GLOBAL-PEER").WithDescription("description"),
		inBGP: &fpoc.NetworkInstance_Protocol_Bgp{},
		wantBGP: &fpoc.NetworkInstance_Protocol_Bgp{
			PeerGroup: map[string]*fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
				"GLOBAL-PEER": {
					PeerGroupName: ygot.String("GLOBAL-PEER"),
					Description:   ygot.String("description"),
				},
			},
		},
	}, {
		desc:  "PeerGroup with passive-mode",
		pg:    NewPeerGroup("GLOBAL-PEER").WithPassiveMode(true),
		inBGP: &fpoc.NetworkInstance_Protocol_Bgp{},
		wantBGP: &fpoc.NetworkInstance_Protocol_Bgp{
			PeerGroup: map[string]*fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
				"GLOBAL-PEER": {
					PeerGroupName: ygot.String("GLOBAL-PEER"),
					Transport: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_Transport{
						PassiveMode: ygot.Bool(true),
					},
				},
			},
		},
	}, {
		desc:  "PeerGroup with tcp-mss",
		pg:    NewPeerGroup("GLOBAL-PEER").WithTCPMSS(12345),
		inBGP: &fpoc.NetworkInstance_Protocol_Bgp{},
		wantBGP: &fpoc.NetworkInstance_Protocol_Bgp{
			PeerGroup: map[string]*fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
				"GLOBAL-PEER": {
					PeerGroupName: ygot.String("GLOBAL-PEER"),
					Transport: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_Transport{
						TcpMss: ygot.Uint16(12345),
					},
				},
			},
		},
	}, {
		desc:  "PeerGroup with mtu-discovery",
		pg:    NewPeerGroup("GLOBAL-PEER").WithMTUDiscovery(true),
		inBGP: &fpoc.NetworkInstance_Protocol_Bgp{},
		wantBGP: &fpoc.NetworkInstance_Protocol_Bgp{
			PeerGroup: map[string]*fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
				"GLOBAL-PEER": {
					PeerGroupName: ygot.String("GLOBAL-PEER"),
					Transport: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_Transport{
						MtuDiscovery: ygot.Bool(true),
					},
				},
			},
		},
	}, {
		desc:  "PeerGroup with local-address",
		pg:    NewPeerGroup("GLOBAL-PEER").WithLocalAddress("192.0.2.2"),
		inBGP: &fpoc.NetworkInstance_Protocol_Bgp{},
		wantBGP: &fpoc.NetworkInstance_Protocol_Bgp{
			PeerGroup: map[string]*fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
				"GLOBAL-PEER": {
					PeerGroupName: ygot.String("GLOBAL-PEER"),
					Transport: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_Transport{
						LocalAddress: ygot.String("192.0.2.2"),
					},
				},
			},
		},
	}, {
		desc:  "PeerGroup with local-as",
		pg:    NewPeerGroup("GLOBAL-PEER").WithLocalAS(1234),
		inBGP: &fpoc.NetworkInstance_Protocol_Bgp{},
		wantBGP: &fpoc.NetworkInstance_Protocol_Bgp{
			PeerGroup: map[string]*fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
				"GLOBAL-PEER": {
					PeerGroupName: ygot.String("GLOBAL-PEER"),
					LocalAs:       ygot.Uint32(1234),
				},
			},
		},
	}, {
		desc:  "PeerGroup with peer-as",
		pg:    NewPeerGroup("GLOBAL-PEER").WithPeerAS(1234),
		inBGP: &fpoc.NetworkInstance_Protocol_Bgp{},
		wantBGP: &fpoc.NetworkInstance_Protocol_Bgp{
			PeerGroup: map[string]*fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
				"GLOBAL-PEER": {
					PeerGroupName: ygot.String("GLOBAL-PEER"),
					PeerAs:        ygot.Uint32(1234),
				},
			},
		},
	}, {
		desc:  "PeerGroup with renmove-private-as",
		pg:    NewPeerGroup("GLOBAL-PEER").WithRemovePrivateAS(fpoc.BgpTypes_RemovePrivateAsOption_PRIVATE_AS_REMOVE_ALL),
		inBGP: &fpoc.NetworkInstance_Protocol_Bgp{},
		wantBGP: &fpoc.NetworkInstance_Protocol_Bgp{
			PeerGroup: map[string]*fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
				"GLOBAL-PEER": {
					PeerGroupName:   ygot.String("GLOBAL-PEER"),
					RemovePrivateAs: fpoc.BgpTypes_RemovePrivateAsOption_PRIVATE_AS_REMOVE_ALL,
				},
			},
		},
	}, {
		desc:  "PeerGroup with send-community",
		pg:    NewPeerGroup("GLOBAL-PEER").WithSendCommunity(fpoc.BgpTypes_CommunityType_BOTH),
		inBGP: &fpoc.NetworkInstance_Protocol_Bgp{},
		wantBGP: &fpoc.NetworkInstance_Protocol_Bgp{
			PeerGroup: map[string]*fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
				"GLOBAL-PEER": {
					PeerGroupName: ygot.String("GLOBAL-PEER"),
					SendCommunity: fpoc.BgpTypes_CommunityType_BOTH,
				},
			},
		},
	}, {
		desc: "PeerGroup with max-prefixes",
		pg: NewPeerGroup("GLOBAL-PEER").WithV4PrefixLimit(2000, PrefixLimitOptions{
			PreventTeardown:     true,
			RestartTime:         5 * time.Second,
			WarningThresholdPct: 90,
		}),
		inBGP: &fpoc.NetworkInstance_Protocol_Bgp{},
		wantBGP: &fpoc.NetworkInstance_Protocol_Bgp{
			PeerGroup: map[string]*fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
				"GLOBAL-PEER": {
					PeerGroupName: ygot.String("GLOBAL-PEER"),
					AfiSafi: map[fpoc.E_BgpTypes_AFI_SAFI_TYPE]*fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AfiSafi{
						fpoc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST: {
							AfiSafiName: fpoc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST,
							Ipv4Unicast: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AfiSafi_Ipv4Unicast{
								PrefixLimit: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AfiSafi_Ipv4Unicast_PrefixLimit{
									MaxPrefixes:         ygot.Uint32(2000),
									PreventTeardown:     ygot.Bool(true),
									WarningThresholdPct: ygot.Uint8(90),
								},
							},
						},
					},
					Timers: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_Timers{
						RestartTime: ygot.Uint16(5),
					},
				},
			},
		},
	}, {
		desc:  "PeerGroup with keepalive-interval",
		pg:    NewPeerGroup("GLOBAL-PEER").WithKeepaliveInterval(5*time.Second, 15*time.Second),
		inBGP: &fpoc.NetworkInstance_Protocol_Bgp{},
		wantBGP: &fpoc.NetworkInstance_Protocol_Bgp{
			PeerGroup: map[string]*fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
				"GLOBAL-PEER": {
					PeerGroupName: ygot.String("GLOBAL-PEER"),
					Timers: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_Timers{
						HoldTime:          ygot.Float64(15),
						KeepaliveInterval: ygot.Float64(5),
					},
				},
			},
		},
	}, {
		desc:  "PeerGroup with MRAI",
		pg:    NewPeerGroup("GLOBAL-PEER").WithMRAI(5 * time.Second),
		inBGP: &fpoc.NetworkInstance_Protocol_Bgp{},
		wantBGP: &fpoc.NetworkInstance_Protocol_Bgp{
			PeerGroup: map[string]*fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
				"GLOBAL-PEER": {
					PeerGroupName: ygot.String("GLOBAL-PEER"),
					Timers: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_Timers{
						MinimumAdvertisementInterval: ygot.Float64(5),
					},
				},
			},
		},
	}, {
		desc:  "PeerGroup with connect-retry",
		pg:    NewPeerGroup("GLOBAL-PEER").WithConnectRetry(5 * time.Second),
		inBGP: &fpoc.NetworkInstance_Protocol_Bgp{},
		wantBGP: &fpoc.NetworkInstance_Protocol_Bgp{
			PeerGroup: map[string]*fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
				"GLOBAL-PEER": {
					PeerGroupName: ygot.String("GLOBAL-PEER"),
					Timers: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_Timers{
						ConnectRetry: ygot.Float64(5),
					},
				},
			},
		},
	}, {
		desc: "BGP already contains pg with no conflicts",
		pg:   NewPeerGroup("GLOBAL-PEER"),
		inBGP: &fpoc.NetworkInstance_Protocol_Bgp{
			PeerGroup: map[string]*fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
				"GLOBAL-PEER": {
					PeerGroupName: ygot.String("GLOBAL-PEER"),
				},
			},
		},
		wantBGP: &fpoc.NetworkInstance_Protocol_Bgp{
			PeerGroup: map[string]*fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
				"GLOBAL-PEER": {
					PeerGroupName: ygot.String("GLOBAL-PEER"),
				},
			},
		},
	}}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			if err := test.pg.AugmentGlobal(test.inBGP); err != nil {
				t.Fatalf("error not expected: %v", err)
			}
			if diff := cmp.Diff(test.wantBGP, test.inBGP); diff != "" {
				t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
			}
		})
	}
}

// TestPGAugmentBGPErrors tests the BGP pg augment to BGP pg validation.
func TestPGAugmentBGPErrors(t *testing.T) {
	tests := []struct {
		desc          string
		pg            *PeerGroup
		inBGP         *fpoc.NetworkInstance_Protocol_Bgp
		wantErrSubStr string
	}{{
		desc: "PeerGroup already exists but with conflicts",
		pg:   NewPeerGroup("GLOBAL-PEER").WithMRAI(6 * time.Second),
		inBGP: &fpoc.NetworkInstance_Protocol_Bgp{
			PeerGroup: map[string]*fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
				"GLOBAL-PEER": {
					PeerGroupName: ygot.String("GLOBAL-PEER"),
					Timers: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_Timers{
						MinimumAdvertisementInterval: ygot.Float64(5),
					},
				},
			},
		},
		wantErrSubStr: "destination value was set, but was not equal to source value",
	}}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			err := test.pg.AugmentGlobal(test.inBGP)
			if err == nil {
				t.Fatalf("error expected")
			}
			if !strings.Contains(err.Error(), test.wantErrSubStr) {
				t.Errorf("Error string does not match: %v", err)
			}
		})
	}
}

type FakePeerGroupFeature struct {
	Err           error
	augmentCalled bool
	oc            *fpoc.NetworkInstance_Protocol_Bgp_PeerGroup
}

func (f *FakePeerGroupFeature) AugmentPeerGroup(oc *fpoc.NetworkInstance_Protocol_Bgp_PeerGroup) error {
	f.oc = oc
	f.augmentCalled = true
	return f.Err
}

// TestPeerGroupWithFeature tests the WithFeature method.
func TestPeerGroupWithFeature(t *testing.T) {
	tests := []struct {
		desc string
		err  error
	}{{
		desc: "error not expected",
	}, {
		desc: "error expected",
		err:  errors.New("some error"),
	}}

	for _, test := range tests {
		n := NewPeerGroup("GLOBAL-PEER")
		ff := &FakePeerGroupFeature{Err: test.err}
		gotErr := n.WithFeature(ff)
		if !ff.augmentCalled {
			t.Errorf("AugmentPeerGroup was not called")
		}
		if ff.oc != &n.oc {
			t.Errorf("PG ptr is not equal")
		}
		if test.err != nil {
			if gotErr != nil {
				if !strings.Contains(gotErr.Error(), test.err.Error()) {
					t.Errorf("Error strings are not equal")
				}
			}
			if gotErr == nil {
				t.Errorf("Expecting error but got none")
			}
		}
	}
}
