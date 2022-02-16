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
	"github.com/openconfig/featureprofiles/yang/oc"
	"github.com/openconfig/ygot/ygot"
)

// TestAddress tests the Address method.
func TestAddress(t *testing.T) {
	n := NewNeighbor("1.2.3.4")
	if got, want := n.Address(), "1.2.3.4"; got != want {
		t.Errorf("got %v but expecting %v", got, want)
	}
}

// TestAugmentBGP tests the BGP neighbor augment to BGP neighbor.
func TestAugmentBGP(t *testing.T) {
	tests := []struct {
		desc     string
		neighbor *Neighbor
		inBGP    *oc.NetworkInstance_Protocol_Bgp
		wantBGP  *oc.NetworkInstance_Protocol_Bgp
	}{{
		desc:     "Neighbor with no params",
		neighbor: NewNeighbor("1.2.3.4"),
		inBGP:    &oc.NetworkInstance_Protocol_Bgp{},
		wantBGP: &oc.NetworkInstance_Protocol_Bgp{
			Neighbor: map[string]*oc.NetworkInstance_Protocol_Bgp_Neighbor{
				"1.2.3.4": {
					NeighborAddress: ygot.String("1.2.3.4"),
				},
			},
		},
	}, {
		desc:     "Neighbor with AFI-SAFI",
		neighbor: NewNeighbor("1.2.3.4").WithAFISAFI(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST),
		inBGP:    &oc.NetworkInstance_Protocol_Bgp{},
		wantBGP: &oc.NetworkInstance_Protocol_Bgp{
			Neighbor: map[string]*oc.NetworkInstance_Protocol_Bgp_Neighbor{
				"1.2.3.4": {
					NeighborAddress: ygot.String("1.2.3.4"),
					AfiSafi: map[oc.E_BgpTypes_AFI_SAFI_TYPE]*oc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi{
						oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST: {
							AfiSafiName: oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST,
							Enabled:     ygot.Bool(true),
						},
					},
				},
			},
		},
	}, {
		desc:     "Neighbor with peer-group",
		neighbor: NewNeighbor("1.2.3.4").WithPeerGroup("GLOBAL-PEER"),
		inBGP:    &oc.NetworkInstance_Protocol_Bgp{},
		wantBGP: &oc.NetworkInstance_Protocol_Bgp{
			Neighbor: map[string]*oc.NetworkInstance_Protocol_Bgp_Neighbor{
				"1.2.3.4": {
					NeighborAddress: ygot.String("1.2.3.4"),
					PeerGroup:       ygot.String("GLOBAL-PEER"),
				},
			},
		},
	}, {
		desc:     "Neighbor with log-state-changes",
		neighbor: NewNeighbor("1.2.3.4").WithLogStateChanges(true),
		inBGP:    &oc.NetworkInstance_Protocol_Bgp{},
		wantBGP: &oc.NetworkInstance_Protocol_Bgp{
			Neighbor: map[string]*oc.NetworkInstance_Protocol_Bgp_Neighbor{
				"1.2.3.4": {
					NeighborAddress: ygot.String("1.2.3.4"),
					LoggingOptions: &oc.NetworkInstance_Protocol_Bgp_Neighbor_LoggingOptions{
						LogNeighborStateChanges: ygot.Bool(true),
					},
				},
			},
		},
	}, {
		desc:     "Neighbor with auth-password",
		neighbor: NewNeighbor("1.2.3.4").WithAuthPassword("password"),
		inBGP:    &oc.NetworkInstance_Protocol_Bgp{},
		wantBGP: &oc.NetworkInstance_Protocol_Bgp{
			Neighbor: map[string]*oc.NetworkInstance_Protocol_Bgp_Neighbor{
				"1.2.3.4": {
					NeighborAddress: ygot.String("1.2.3.4"),
					AuthPassword:    ygot.String("password"),
				},
			},
		},
	}, {
		desc:     "Neighbor with description",
		neighbor: NewNeighbor("1.2.3.4").WithDescription("description"),
		inBGP:    &oc.NetworkInstance_Protocol_Bgp{},
		wantBGP: &oc.NetworkInstance_Protocol_Bgp{
			Neighbor: map[string]*oc.NetworkInstance_Protocol_Bgp_Neighbor{
				"1.2.3.4": {
					NeighborAddress: ygot.String("1.2.3.4"),
					Description:     ygot.String("description"),
				},
			},
		},
	}, {
		desc:     "Neighbor with passive-mode",
		neighbor: NewNeighbor("1.2.3.4").WithPassiveMode(true),
		inBGP:    &oc.NetworkInstance_Protocol_Bgp{},
		wantBGP: &oc.NetworkInstance_Protocol_Bgp{
			Neighbor: map[string]*oc.NetworkInstance_Protocol_Bgp_Neighbor{
				"1.2.3.4": {
					NeighborAddress: ygot.String("1.2.3.4"),
					Transport: &oc.NetworkInstance_Protocol_Bgp_Neighbor_Transport{
						PassiveMode: ygot.Bool(true),
					},
				},
			},
		},
	}, {
		desc:     "Neighbor with tcp-mss",
		neighbor: NewNeighbor("1.2.3.4").WithTCPMSS(12345),
		inBGP:    &oc.NetworkInstance_Protocol_Bgp{},
		wantBGP: &oc.NetworkInstance_Protocol_Bgp{
			Neighbor: map[string]*oc.NetworkInstance_Protocol_Bgp_Neighbor{
				"1.2.3.4": {
					NeighborAddress: ygot.String("1.2.3.4"),
					Transport: &oc.NetworkInstance_Protocol_Bgp_Neighbor_Transport{
						TcpMss: ygot.Uint16(12345),
					},
				},
			},
		},
	}, {
		desc:     "Neighbor with mtu-discovery",
		neighbor: NewNeighbor("1.2.3.4").WithMTUDiscovery(true),
		inBGP:    &oc.NetworkInstance_Protocol_Bgp{},
		wantBGP: &oc.NetworkInstance_Protocol_Bgp{
			Neighbor: map[string]*oc.NetworkInstance_Protocol_Bgp_Neighbor{
				"1.2.3.4": {
					NeighborAddress: ygot.String("1.2.3.4"),
					Transport: &oc.NetworkInstance_Protocol_Bgp_Neighbor_Transport{
						MtuDiscovery: ygot.Bool(true),
					},
				},
			},
		},
	}, {
		desc:     "Neighbor with local-address",
		neighbor: NewNeighbor("1.2.3.4").WithLocalAddress("1.2.3.5"),
		inBGP:    &oc.NetworkInstance_Protocol_Bgp{},
		wantBGP: &oc.NetworkInstance_Protocol_Bgp{
			Neighbor: map[string]*oc.NetworkInstance_Protocol_Bgp_Neighbor{
				"1.2.3.4": {
					NeighborAddress: ygot.String("1.2.3.4"),
					Transport: &oc.NetworkInstance_Protocol_Bgp_Neighbor_Transport{
						LocalAddress: ygot.String("1.2.3.5"),
					},
				},
			},
		},
	}, {
		desc:     "Neighbor with local-as",
		neighbor: NewNeighbor("1.2.3.4").WithLocalAS(1234),
		inBGP:    &oc.NetworkInstance_Protocol_Bgp{},
		wantBGP: &oc.NetworkInstance_Protocol_Bgp{
			Neighbor: map[string]*oc.NetworkInstance_Protocol_Bgp_Neighbor{
				"1.2.3.4": {
					NeighborAddress: ygot.String("1.2.3.4"),
					LocalAs:         ygot.Uint32(1234),
				},
			},
		},
	}, {
		desc:     "Neighbor with peer-as",
		neighbor: NewNeighbor("1.2.3.4").WithPeerAS(1234),
		inBGP:    &oc.NetworkInstance_Protocol_Bgp{},
		wantBGP: &oc.NetworkInstance_Protocol_Bgp{
			Neighbor: map[string]*oc.NetworkInstance_Protocol_Bgp_Neighbor{
				"1.2.3.4": {
					NeighborAddress: ygot.String("1.2.3.4"),
					PeerAs:          ygot.Uint32(1234),
				},
			},
		},
	}, {
		desc:     "Neighbor with renmove-private-as",
		neighbor: NewNeighbor("1.2.3.4").WithRemovePrivateAS(oc.BgpTypes_RemovePrivateAsOption_PRIVATE_AS_REMOVE_ALL),
		inBGP:    &oc.NetworkInstance_Protocol_Bgp{},
		wantBGP: &oc.NetworkInstance_Protocol_Bgp{
			Neighbor: map[string]*oc.NetworkInstance_Protocol_Bgp_Neighbor{
				"1.2.3.4": {
					NeighborAddress: ygot.String("1.2.3.4"),
					RemovePrivateAs: oc.BgpTypes_RemovePrivateAsOption_PRIVATE_AS_REMOVE_ALL,
				},
			},
		},
	}, {
		desc:     "Neighbor with send-community",
		neighbor: NewNeighbor("1.2.3.4").WithSendCommunity(oc.BgpTypes_CommunityType_BOTH),
		inBGP:    &oc.NetworkInstance_Protocol_Bgp{},
		wantBGP: &oc.NetworkInstance_Protocol_Bgp{
			Neighbor: map[string]*oc.NetworkInstance_Protocol_Bgp_Neighbor{
				"1.2.3.4": {
					NeighborAddress: ygot.String("1.2.3.4"),
					SendCommunity:   oc.BgpTypes_CommunityType_BOTH,
				},
			},
		},
	}, {
		desc: "Neighbor with max-prefixes",
		neighbor: NewNeighbor("1.2.3.4").WithV4PrefixLimit(2000, PrefixLimitOptions{
			PreventTeardown:     true,
			RestartTimer:        5 * time.Second,
			WarningThresholdPct: 90,
		}),
		inBGP: &oc.NetworkInstance_Protocol_Bgp{},
		wantBGP: &oc.NetworkInstance_Protocol_Bgp{
			Neighbor: map[string]*oc.NetworkInstance_Protocol_Bgp_Neighbor{
				"1.2.3.4": {
					NeighborAddress: ygot.String("1.2.3.4"),
					AfiSafi: map[oc.E_BgpTypes_AFI_SAFI_TYPE]*oc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi{
						oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST: {
							AfiSafiName: oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST,
							Ipv4Unicast: &oc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_Ipv4Unicast{
								PrefixLimit: &oc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_Ipv4Unicast_PrefixLimit{
									MaxPrefixes:         ygot.Uint32(2000),
									PreventTeardown:     ygot.Bool(true),
									RestartTimer:        ygot.Float64(float64(5)),
									WarningThresholdPct: ygot.Uint8(90),
								},
							},
						},
					},
				},
			},
		},
	}, {
		desc:     "Neighbor with keepalive-interval",
		neighbor: NewNeighbor("1.2.3.4").WithKeepaliveInterval(5*time.Second, 15*time.Second),
		inBGP:    &oc.NetworkInstance_Protocol_Bgp{},
		wantBGP: &oc.NetworkInstance_Protocol_Bgp{
			Neighbor: map[string]*oc.NetworkInstance_Protocol_Bgp_Neighbor{
				"1.2.3.4": {
					NeighborAddress: ygot.String("1.2.3.4"),
					Timers: &oc.NetworkInstance_Protocol_Bgp_Neighbor_Timers{
						HoldTime:          ygot.Float64(15),
						KeepaliveInterval: ygot.Float64(5),
					},
				},
			},
		},
	}, {
		desc:     "Neighbor with MRAI",
		neighbor: NewNeighbor("1.2.3.4").WithMRAI(5 * time.Second),
		inBGP:    &oc.NetworkInstance_Protocol_Bgp{},
		wantBGP: &oc.NetworkInstance_Protocol_Bgp{
			Neighbor: map[string]*oc.NetworkInstance_Protocol_Bgp_Neighbor{
				"1.2.3.4": {
					NeighborAddress: ygot.String("1.2.3.4"),
					Timers: &oc.NetworkInstance_Protocol_Bgp_Neighbor_Timers{
						MinimumAdvertisementInterval: ygot.Float64(5),
					},
				},
			},
		},
	}, {
		desc:     "Neighbor with connect-retry",
		neighbor: NewNeighbor("1.2.3.4").WithConnectRetry(5 * time.Second),
		inBGP:    &oc.NetworkInstance_Protocol_Bgp{},
		wantBGP: &oc.NetworkInstance_Protocol_Bgp{
			Neighbor: map[string]*oc.NetworkInstance_Protocol_Bgp_Neighbor{
				"1.2.3.4": {
					NeighborAddress: ygot.String("1.2.3.4"),
					Timers: &oc.NetworkInstance_Protocol_Bgp_Neighbor_Timers{
						ConnectRetry: ygot.Float64(5),
					},
				},
			},
		},
	}, {
		desc:     "BGP already contains neighbor with no conflicts",
		neighbor: NewNeighbor("1.2.3.4"),
		inBGP: &oc.NetworkInstance_Protocol_Bgp{
			Neighbor: map[string]*oc.NetworkInstance_Protocol_Bgp_Neighbor{
				"1.2.3.4": {
					NeighborAddress: ygot.String("1.2.3.4"),
				},
			},
		},
		wantBGP: &oc.NetworkInstance_Protocol_Bgp{
			Neighbor: map[string]*oc.NetworkInstance_Protocol_Bgp_Neighbor{
				"1.2.3.4": {
					NeighborAddress: ygot.String("1.2.3.4"),
				},
			},
		},
	}}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			if err := test.neighbor.AugmentGlobal(test.inBGP); err != nil {
				t.Fatalf("error not expected: %v", err)
			}
			if diff := cmp.Diff(test.wantBGP, test.inBGP); diff != "" {
				t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
			}
		})
	}
}

// TestAugmentBGP_Errors tests the BGP neighbor augment to BGP neighbor validation.
func TestAugmentBGP_Errors(t *testing.T) {
	tests := []struct {
		desc          string
		neighbor      *Neighbor
		inBGP         *oc.NetworkInstance_Protocol_Bgp
		wantErrSubStr string
	}{{
		desc:     "Neighbor already exists but with conflicts",
		neighbor: NewNeighbor("1.2.3.4").WithMRAI(6 * time.Second),
		inBGP: &oc.NetworkInstance_Protocol_Bgp{
			Neighbor: map[string]*oc.NetworkInstance_Protocol_Bgp_Neighbor{
				"1.2.3.4": {
					NeighborAddress: ygot.String("1.2.3.4"),
					Timers: &oc.NetworkInstance_Protocol_Bgp_Neighbor_Timers{
						MinimumAdvertisementInterval: ygot.Float64(5),
					},
				},
			},
		},
		wantErrSubStr: "destination value was set, but was not equal to source value",
	}}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			err := test.neighbor.AugmentGlobal(test.inBGP)
			if err == nil {
				t.Fatalf("error expected")
			}
			if !strings.Contains(err.Error(), test.wantErrSubStr) {
				t.Errorf("Error string does not match: %v", err)
			}
		})
	}
}

type FakeNeighborFeature struct {
	Err           error
	augmentCalled bool
	oc            *oc.NetworkInstance_Protocol_Bgp_Neighbor
}

func (f *FakeNeighborFeature) AugmentNeighbor(oc *oc.NetworkInstance_Protocol_Bgp_Neighbor) error {
	f.oc = oc
	f.augmentCalled = true
	return f.Err
}

// TestNeighborWithFeature tests the WithFeature method.
func TestNeighborWithFeature(t *testing.T) {
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
		n := NewNeighbor("1.2.3.4")
		ff := &FakeNeighborFeature{Err: test.wantErr}
		gotErr := n.WithFeature(ff)
		if !ff.augmentCalled {
			t.Errorf("AugmentNeighbor was not called")
		}
		if ff.oc != &n.oc {
			t.Errorf("neighbor ptr is not equal")
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
