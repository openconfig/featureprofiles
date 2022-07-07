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

// TestAddress tests the Address method.
func TestAddress(t *testing.T) {
	n := NewNeighbor("192.0.2.1")
	if got, want := n.Address(), "192.0.2.1"; got != want {
		t.Errorf("got %v but expecting %v", got, want)
	}
}

// TestAugmentBGP tests the BGP neighbor augment to BGP neighbor.
func TestAugmentBGP(t *testing.T) {
	tests := []struct {
		desc     string
		neighbor *Neighbor
		inBGP    *fpoc.NetworkInstance_Protocol_Bgp
		wantBGP  *fpoc.NetworkInstance_Protocol_Bgp
	}{{
		desc:     "Neighbor with no params",
		neighbor: NewNeighbor("192.0.2.1"),
		inBGP:    &fpoc.NetworkInstance_Protocol_Bgp{},
		wantBGP: &fpoc.NetworkInstance_Protocol_Bgp{
			Neighbor: map[string]*fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
				"192.0.2.1": {
					NeighborAddress: ygot.String("192.0.2.1"),
				},
			},
		},
	}, {
		desc:     "Neighbor with AFI-SAFI",
		neighbor: NewNeighbor("192.0.2.1").WithAFISAFI(fpoc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST),
		inBGP:    &fpoc.NetworkInstance_Protocol_Bgp{},
		wantBGP: &fpoc.NetworkInstance_Protocol_Bgp{
			Neighbor: map[string]*fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
				"192.0.2.1": {
					NeighborAddress: ygot.String("192.0.2.1"),
					AfiSafi: map[fpoc.E_BgpTypes_AFI_SAFI_TYPE]*fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi{
						fpoc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST: {
							AfiSafiName: fpoc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST,
							Enabled:     ygot.Bool(true),
						},
					},
				},
			},
		},
	}, {
		desc:     "Neighbor with peer-group",
		neighbor: NewNeighbor("192.0.2.1").WithPeerGroup("GLOBAL-PEER"),
		inBGP:    &fpoc.NetworkInstance_Protocol_Bgp{},
		wantBGP: &fpoc.NetworkInstance_Protocol_Bgp{
			Neighbor: map[string]*fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
				"192.0.2.1": {
					NeighborAddress: ygot.String("192.0.2.1"),
					PeerGroup:       ygot.String("GLOBAL-PEER"),
				},
			},
		},
	}, {
		desc:     "Neighbor with log-state-changes",
		neighbor: NewNeighbor("192.0.2.1").WithLogStateChanges(true),
		inBGP:    &fpoc.NetworkInstance_Protocol_Bgp{},
		wantBGP: &fpoc.NetworkInstance_Protocol_Bgp{
			Neighbor: map[string]*fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
				"192.0.2.1": {
					NeighborAddress: ygot.String("192.0.2.1"),
					LoggingOptions: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_LoggingOptions{
						LogNeighborStateChanges: ygot.Bool(true),
					},
				},
			},
		},
	}, {
		desc:     "Neighbor with auth-password",
		neighbor: NewNeighbor("192.0.2.1").WithAuthPassword("password"),
		inBGP:    &fpoc.NetworkInstance_Protocol_Bgp{},
		wantBGP: &fpoc.NetworkInstance_Protocol_Bgp{
			Neighbor: map[string]*fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
				"192.0.2.1": {
					NeighborAddress: ygot.String("192.0.2.1"),
					AuthPassword:    ygot.String("password"),
				},
			},
		},
	}, {
		desc:     "Neighbor with description",
		neighbor: NewNeighbor("192.0.2.1").WithDescription("description"),
		inBGP:    &fpoc.NetworkInstance_Protocol_Bgp{},
		wantBGP: &fpoc.NetworkInstance_Protocol_Bgp{
			Neighbor: map[string]*fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
				"192.0.2.1": {
					NeighborAddress: ygot.String("192.0.2.1"),
					Description:     ygot.String("description"),
				},
			},
		},
	}, {
		desc:     "Neighbor with passive-mode",
		neighbor: NewNeighbor("192.0.2.1").WithPassiveMode(true),
		inBGP:    &fpoc.NetworkInstance_Protocol_Bgp{},
		wantBGP: &fpoc.NetworkInstance_Protocol_Bgp{
			Neighbor: map[string]*fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
				"192.0.2.1": {
					NeighborAddress: ygot.String("192.0.2.1"),
					Transport: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_Transport{
						PassiveMode: ygot.Bool(true),
					},
				},
			},
		},
	}, {
		desc:     "Neighbor with tcp-mss",
		neighbor: NewNeighbor("192.0.2.1").WithTCPMSS(12345),
		inBGP:    &fpoc.NetworkInstance_Protocol_Bgp{},
		wantBGP: &fpoc.NetworkInstance_Protocol_Bgp{
			Neighbor: map[string]*fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
				"192.0.2.1": {
					NeighborAddress: ygot.String("192.0.2.1"),
					Transport: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_Transport{
						TcpMss: ygot.Uint16(12345),
					},
				},
			},
		},
	}, {
		desc:     "Neighbor with mtu-discovery",
		neighbor: NewNeighbor("192.0.2.1").WithMTUDiscovery(true),
		inBGP:    &fpoc.NetworkInstance_Protocol_Bgp{},
		wantBGP: &fpoc.NetworkInstance_Protocol_Bgp{
			Neighbor: map[string]*fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
				"192.0.2.1": {
					NeighborAddress: ygot.String("192.0.2.1"),
					Transport: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_Transport{
						MtuDiscovery: ygot.Bool(true),
					},
				},
			},
		},
	}, {
		desc:     "Neighbor with local-address",
		neighbor: NewNeighbor("192.0.2.1").WithLocalAddress("192.0.2.2"),
		inBGP:    &fpoc.NetworkInstance_Protocol_Bgp{},
		wantBGP: &fpoc.NetworkInstance_Protocol_Bgp{
			Neighbor: map[string]*fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
				"192.0.2.1": {
					NeighborAddress: ygot.String("192.0.2.1"),
					Transport: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_Transport{
						LocalAddress: ygot.String("192.0.2.2"),
					},
				},
			},
		},
	}, {
		desc:     "Neighbor with local-as",
		neighbor: NewNeighbor("192.0.2.1").WithLocalAS(1234),
		inBGP:    &fpoc.NetworkInstance_Protocol_Bgp{},
		wantBGP: &fpoc.NetworkInstance_Protocol_Bgp{
			Neighbor: map[string]*fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
				"192.0.2.1": {
					NeighborAddress: ygot.String("192.0.2.1"),
					LocalAs:         ygot.Uint32(1234),
				},
			},
		},
	}, {
		desc:     "Neighbor with peer-as",
		neighbor: NewNeighbor("192.0.2.1").WithPeerAS(1234),
		inBGP:    &fpoc.NetworkInstance_Protocol_Bgp{},
		wantBGP: &fpoc.NetworkInstance_Protocol_Bgp{
			Neighbor: map[string]*fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
				"192.0.2.1": {
					NeighborAddress: ygot.String("192.0.2.1"),
					PeerAs:          ygot.Uint32(1234),
				},
			},
		},
	}, {
		desc:     "Neighbor with renmove-private-as",
		neighbor: NewNeighbor("192.0.2.1").WithRemovePrivateAS(fpoc.BgpTypes_RemovePrivateAsOption_PRIVATE_AS_REMOVE_ALL),
		inBGP:    &fpoc.NetworkInstance_Protocol_Bgp{},
		wantBGP: &fpoc.NetworkInstance_Protocol_Bgp{
			Neighbor: map[string]*fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
				"192.0.2.1": {
					NeighborAddress: ygot.String("192.0.2.1"),
					RemovePrivateAs: fpoc.BgpTypes_RemovePrivateAsOption_PRIVATE_AS_REMOVE_ALL,
				},
			},
		},
	}, {
		desc:     "Neighbor with send-community",
		neighbor: NewNeighbor("192.0.2.1").WithSendCommunity(fpoc.BgpTypes_CommunityType_BOTH),
		inBGP:    &fpoc.NetworkInstance_Protocol_Bgp{},
		wantBGP: &fpoc.NetworkInstance_Protocol_Bgp{
			Neighbor: map[string]*fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
				"192.0.2.1": {
					NeighborAddress: ygot.String("192.0.2.1"),
					SendCommunity:   fpoc.BgpTypes_CommunityType_BOTH,
				},
			},
		},
	}, {
		desc: "Neighbor with max-prefixes",
		neighbor: NewNeighbor("192.0.2.1").WithV4PrefixLimit(2000, PrefixLimitOptions{
			PreventTeardown:     true,
			RestartTime:         5 * time.Second,
			WarningThresholdPct: 90,
		}),
		inBGP: &fpoc.NetworkInstance_Protocol_Bgp{},
		wantBGP: &fpoc.NetworkInstance_Protocol_Bgp{
			Neighbor: map[string]*fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
				"192.0.2.1": {
					NeighborAddress: ygot.String("192.0.2.1"),
					AfiSafi: map[fpoc.E_BgpTypes_AFI_SAFI_TYPE]*fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi{
						fpoc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST: {
							AfiSafiName: fpoc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST,
							Ipv4Unicast: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_Ipv4Unicast{
								PrefixLimit: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_Ipv4Unicast_PrefixLimit{
									MaxPrefixes:         ygot.Uint32(2000),
									PreventTeardown:     ygot.Bool(true),
									WarningThresholdPct: ygot.Uint8(90),
								},
							},
						},
					},
					Timers: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_Timers{
						RestartTime: ygot.Uint16(5),
					},
				},
			},
		},
	}, {
		desc:     "Neighbor with keepalive-interval",
		neighbor: NewNeighbor("192.0.2.1").WithKeepaliveInterval(5*time.Second, 15*time.Second),
		inBGP:    &fpoc.NetworkInstance_Protocol_Bgp{},
		wantBGP: &fpoc.NetworkInstance_Protocol_Bgp{
			Neighbor: map[string]*fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
				"192.0.2.1": {
					NeighborAddress: ygot.String("192.0.2.1"),
					Timers: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_Timers{
						HoldTime:          ygot.Float64(15),
						KeepaliveInterval: ygot.Float64(5),
					},
				},
			},
		},
	}, {
		desc:     "Neighbor with MRAI",
		neighbor: NewNeighbor("192.0.2.1").WithMRAI(5 * time.Second),
		inBGP:    &fpoc.NetworkInstance_Protocol_Bgp{},
		wantBGP: &fpoc.NetworkInstance_Protocol_Bgp{
			Neighbor: map[string]*fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
				"192.0.2.1": {
					NeighborAddress: ygot.String("192.0.2.1"),
					Timers: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_Timers{
						MinimumAdvertisementInterval: ygot.Float64(5),
					},
				},
			},
		},
	}, {
		desc:     "Neighbor with connect-retry",
		neighbor: NewNeighbor("192.0.2.1").WithConnectRetry(5 * time.Second),
		inBGP:    &fpoc.NetworkInstance_Protocol_Bgp{},
		wantBGP: &fpoc.NetworkInstance_Protocol_Bgp{
			Neighbor: map[string]*fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
				"192.0.2.1": {
					NeighborAddress: ygot.String("192.0.2.1"),
					Timers: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_Timers{
						ConnectRetry: ygot.Float64(5),
					},
				},
			},
		},
	}, {
		desc:     "BGP already contains neighbor with no conflicts",
		neighbor: NewNeighbor("192.0.2.1"),
		inBGP: &fpoc.NetworkInstance_Protocol_Bgp{
			Neighbor: map[string]*fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
				"192.0.2.1": {
					NeighborAddress: ygot.String("192.0.2.1"),
				},
			},
		},
		wantBGP: &fpoc.NetworkInstance_Protocol_Bgp{
			Neighbor: map[string]*fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
				"192.0.2.1": {
					NeighborAddress: ygot.String("192.0.2.1"),
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

// TestAugmentBGPErrors tests the BGP neighbor augment to BGP neighbor validation.
func TestAugmentBGPErrors(t *testing.T) {
	tests := []struct {
		desc          string
		neighbor      *Neighbor
		inBGP         *fpoc.NetworkInstance_Protocol_Bgp
		wantErrSubStr string
	}{{
		desc:     "Neighbor already exists but with conflicts",
		neighbor: NewNeighbor("192.0.2.1").WithMRAI(6 * time.Second),
		inBGP: &fpoc.NetworkInstance_Protocol_Bgp{
			Neighbor: map[string]*fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
				"192.0.2.1": {
					NeighborAddress: ygot.String("192.0.2.1"),
					Timers: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_Timers{
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
	oc            *fpoc.NetworkInstance_Protocol_Bgp_Neighbor
}

func (f *FakeNeighborFeature) AugmentNeighbor(oc *fpoc.NetworkInstance_Protocol_Bgp_Neighbor) error {
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
		n := NewNeighbor("192.0.2.1")
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
