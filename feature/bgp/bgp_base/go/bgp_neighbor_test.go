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

var neighborAddress = "1.2.3.4"

func newNeighbor() *Neighbor {
	return &Neighbor{
		oc: oc.NetworkInstance_Protocol_Bgp_Neighbor{
			NeighborAddress: ygot.String(neighborAddress),
		},
	}
}

// TestNewNeighbor tests the NewNeighbor function.
func TestNewNeighbor(t *testing.T) {
	want := newNeighbor()
	got := NewNeighbor(neighborAddress)
	if got == nil {
		t.Fatalf("New returned nil")
	}
	if diff := cmp.Diff(want.oc, got.oc); diff != "" {
		t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
	}
}

// TestAddress tests the Address method.
func TestAddress(t *testing.T) {
	n := newNeighbor()
	if got, want := n.Address(), neighborAddress; got != want {
		t.Errorf("got %v but expecting %v", got, want)
	}
}

// TestWithAFISAFI tests setting AS for BGP global.
func TestWithAFISAFI(t *testing.T) {
	afisafi := oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST
	want := oc.NetworkInstance_Protocol_Bgp_Neighbor{
		NeighborAddress: ygot.String("1.2.3.4"),
		AfiSafi: map[oc.E_BgpTypes_AFI_SAFI_TYPE]*oc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi{
			afisafi: &oc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi{
				AfiSafiName: afisafi,
				Enabled:     ygot.Bool(true),
			},
		},
	}

	n := newNeighbor()
	res := n.WithAFISAFI(afisafi)
	if res == nil {
		t.Fatalf("WithAFISAFI returned nil")
	}

	if diff := cmp.Diff(want, res.oc); diff != "" {
		t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
	}
}

// TestWithPeerGroup tests setting peer-group for BGP global.
func TestWithPeerGroup(t *testing.T) {
	pgname := "GLOBAL-PEER"
	want := oc.NetworkInstance_Protocol_Bgp_Neighbor{
		NeighborAddress: ygot.String("1.2.3.4"),
		PeerGroup:       ygot.String(pgname),
	}

	n := newNeighbor()

	res := n.WithPeerGroup(pgname)
	if res == nil {
		t.Fatalf("WithPeerGroup returned nil")
	}

	if diff := cmp.Diff(want, res.oc); diff != "" {
		t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
	}
}

// TestWithLogStateChanges tests setting log state changes for BGP global.
func TestWithLogStateChanges(t *testing.T) {
	want := oc.NetworkInstance_Protocol_Bgp_Neighbor{
		NeighborAddress: ygot.String("1.2.3.4"),
		LoggingOptions: &oc.NetworkInstance_Protocol_Bgp_Neighbor_LoggingOptions{
			LogNeighborStateChanges: ygot.Bool(true),
		},
	}

	n := newNeighbor()
	res := n.WithLogStateChanges(true)
	if res == nil {
		t.Fatalf("WithLogStateChanges returned nil")
	}

	if diff := cmp.Diff(want, res.oc); diff != "" {
		t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
	}
}

// TestWithAuthPassword tests setting auth-password for BGP global.
func TestWithAuthPassword(t *testing.T) {
	pwd := "foobar"
	want := oc.NetworkInstance_Protocol_Bgp_Neighbor{
		NeighborAddress: ygot.String("1.2.3.4"),
		AuthPassword:    ygot.String(pwd),
	}

	n := newNeighbor()
	res := n.WithAuthPassword(pwd)
	if res == nil {
		t.Fatalf("WithAuthPassword returned nil")
	}

	if diff := cmp.Diff(want, res.oc); diff != "" {
		t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
	}
}

// TestWithDescription tests setting description for BGP global.
func TestWithDescription(t *testing.T) {
	desc := "foobar"
	want := oc.NetworkInstance_Protocol_Bgp_Neighbor{
		NeighborAddress: ygot.String("1.2.3.4"),
		Description:     ygot.String(desc),
	}

	n := newNeighbor()
	res := n.WithDescription(desc)
	if res == nil {
		t.Fatalf("WithDescription returned nil")
	}

	if diff := cmp.Diff(want, res.oc); diff != "" {
		t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
	}
}

// TestWithTransport tests setting auth-password for BGP global.
func TestWithTransport(t *testing.T) {
	tests := []struct {
		desc      string
		transport Transport
		want      *oc.NetworkInstance_Protocol_Bgp_Neighbor
	}{{
		desc: "passive mode set",
		transport: Transport{
			PassiveMode: true,
		},
		want: func() *oc.NetworkInstance_Protocol_Bgp_Neighbor {
			oc := &oc.NetworkInstance_Protocol_Bgp_Neighbor{
				NeighborAddress: ygot.String("1.2.3.4"),
			}
			toc := oc.GetOrCreateTransport()
			toc.PassiveMode = ygot.Bool(true)
			toc.MtuDiscovery = ygot.Bool(false)
			return oc
		}(),
	}, {
		desc: "TCP MSS",
		transport: Transport{
			TCPMSS: 1234,
		},
		want: func() *oc.NetworkInstance_Protocol_Bgp_Neighbor {
			oc := &oc.NetworkInstance_Protocol_Bgp_Neighbor{
				NeighborAddress: ygot.String("1.2.3.4"),
			}
			toc := oc.GetOrCreateTransport()
			toc.TcpMss = ygot.Uint16(1234)
			toc.PassiveMode = ygot.Bool(false)
			toc.MtuDiscovery = ygot.Bool(false)
			return oc
		}(),
	}, {
		desc: "MTU Discovery",
		transport: Transport{
			MTUDiscovery: true,
		},
		want: func() *oc.NetworkInstance_Protocol_Bgp_Neighbor {
			oc := &oc.NetworkInstance_Protocol_Bgp_Neighbor{
				NeighborAddress: ygot.String("1.2.3.4"),
			}
			toc := oc.GetOrCreateTransport()
			toc.MtuDiscovery = ygot.Bool(true)
			toc.PassiveMode = ygot.Bool(false)
			return oc
		}(),
	}, {
		desc: "Local address",
		transport: Transport{
			LocalAddress: "1.2.3.4",
		},
		want: func() *oc.NetworkInstance_Protocol_Bgp_Neighbor {
			oc := &oc.NetworkInstance_Protocol_Bgp_Neighbor{
				NeighborAddress: ygot.String("1.2.3.4"),
			}
			toc := oc.GetOrCreateTransport()
			toc.MtuDiscovery = ygot.Bool(false)
			toc.PassiveMode = ygot.Bool(false)
			toc.LocalAddress = ygot.String("1.2.3.4")
			return oc
		}(),
	}}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			n := newNeighbor()
			res := n.WithTransport(test.transport)
			if res == nil {
				t.Fatalf("WithTransport returned nil")
			}

			if diff := cmp.Diff(test.want, &res.oc); diff != "" {
				t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
			}
		})
	}
}

// TestWithLocalAS tests setting local AS for BGP global.
func TestWithLocalAS(t *testing.T) {
	as := uint32(1234)
	want := oc.NetworkInstance_Protocol_Bgp_Neighbor{
		NeighborAddress: ygot.String("1.2.3.4"),
		LocalAs:         ygot.Uint32(as),
	}

	n := newNeighbor()
	res := n.WithLocalAS(as)
	if res == nil {
		t.Fatalf("WithLocalAS returned nil")
	}

	if diff := cmp.Diff(want, res.oc); diff != "" {
		t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
	}
}

// TestWithPeerAS tests setting peer AS for BGP global.
func TestWithPeerAS(t *testing.T) {
	as := uint32(1234)
	want := oc.NetworkInstance_Protocol_Bgp_Neighbor{
		NeighborAddress: ygot.String("1.2.3.4"),
		PeerAs:          ygot.Uint32(as),
	}

	n := newNeighbor()
	res := n.WithPeerAS(as)
	if res == nil {
		t.Fatalf("WithPeerAS returned nil")
	}

	if diff := cmp.Diff(want, res.oc); diff != "" {
		t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
	}
}

// TestWithRemovePrivateAS tests setting remove private AS for BGP global.
func TestWithRemovePrivateAS(t *testing.T) {
	val := oc.BgpTypes_RemovePrivateAsOption_PRIVATE_AS_REMOVE_ALL
	want := oc.NetworkInstance_Protocol_Bgp_Neighbor{
		NeighborAddress: ygot.String("1.2.3.4"),
		RemovePrivateAs: val,
	}

	n := newNeighbor()
	res := n.WithRemovePrivateAS(val)
	if res == nil {
		t.Fatalf("WithRemovePrivateAS returned nil")
	}

	if diff := cmp.Diff(want, res.oc); diff != "" {
		t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
	}
}

// TestWithSendCommunity tests setting send-community for BGP global.
func TestWithSendCommunity(t *testing.T) {
	val := oc.BgpTypes_CommunityType_BOTH
	want := oc.NetworkInstance_Protocol_Bgp_Neighbor{
		NeighborAddress: ygot.String("1.2.3.4"),
		SendCommunity:   val,
	}

	n := newNeighbor()
	res := n.WithSendCommunity(val)
	if res == nil {
		t.Fatalf("WithSendCommunity returned nil")
	}

	if diff := cmp.Diff(want, res.oc); diff != "" {
		t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
	}
}

// TestWithV4PrefixLimit tests setting auth-password for BGP global.
func TestWithV4PrefixLimit(t *testing.T) {
	tests := []struct {
		desc string
		pl   PrefixLimit
		want *oc.NetworkInstance_Protocol_Bgp_Neighbor
	}{{
		desc: "max prefixes",
		pl: PrefixLimit{
			MaxPrefixes: 2000,
		},
		want: func() *oc.NetworkInstance_Protocol_Bgp_Neighbor {
			noc := &oc.NetworkInstance_Protocol_Bgp_Neighbor{
				NeighborAddress: ygot.String("1.2.3.4"),
			}
			ploc := noc.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).GetOrCreateIpv4Unicast().GetOrCreatePrefixLimit()
			ploc.MaxPrefixes = ygot.Uint32(2000)
			ploc.PreventTeardown = ygot.Bool(false)
			return noc
		}(),
	}, {
		desc: "Prevent teardown",
		pl: PrefixLimit{
			PreventTeardown: true,
		},
		want: func() *oc.NetworkInstance_Protocol_Bgp_Neighbor {
			noc := &oc.NetworkInstance_Protocol_Bgp_Neighbor{
				NeighborAddress: ygot.String("1.2.3.4"),
			}
			ploc := noc.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).GetOrCreateIpv4Unicast().GetOrCreatePrefixLimit()
			ploc.PreventTeardown = ygot.Bool(true)
			return noc
		}(),
	}, {
		desc: "Restart timer",
		pl: PrefixLimit{
			RestartTimer: 5 * time.Second,
		},
		want: func() *oc.NetworkInstance_Protocol_Bgp_Neighbor {
			noc := &oc.NetworkInstance_Protocol_Bgp_Neighbor{
				NeighborAddress: ygot.String("1.2.3.4"),
			}
			ploc := noc.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).GetOrCreateIpv4Unicast().GetOrCreatePrefixLimit()
			ploc.PreventTeardown = ygot.Bool(false)
			ploc.RestartTimer = ygot.Float64(float64(5))
			return noc
		}(),
	}, {
		desc: "Warning threshold",
		pl: PrefixLimit{
			WarningThresholdPct: 90,
		},
		want: func() *oc.NetworkInstance_Protocol_Bgp_Neighbor {
			noc := &oc.NetworkInstance_Protocol_Bgp_Neighbor{
				NeighborAddress: ygot.String("1.2.3.4"),
			}
			ploc := noc.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).GetOrCreateIpv4Unicast().GetOrCreatePrefixLimit()
			ploc.PreventTeardown = ygot.Bool(false)
			ploc.WarningThresholdPct = ygot.Uint8(90)
			return noc
		}(),
	}}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			n := newNeighbor()
			res := n.WithV4PrefixLimit(test.pl)
			if res == nil {
				t.Fatalf("WithV4PrefixLimit returned nil")
			}

			if diff := cmp.Diff(test.want, &res.oc); diff != "" {
				t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
			}
		})
	}
}

// TestWithTimers tests setting auth-password for BGP global.
func TestWithTimers(t *testing.T) {
	tests := []struct {
		desc   string
		timers Timers
		want   *oc.NetworkInstance_Protocol_Bgp_Neighbor
	}{{
		desc: "min advertisement interval",
		timers: Timers{
			MinimumAdvertisementInterval: 5 * time.Second,
		},
		want: func() *oc.NetworkInstance_Protocol_Bgp_Neighbor {
			noc := &oc.NetworkInstance_Protocol_Bgp_Neighbor{
				NeighborAddress: ygot.String("1.2.3.4"),
			}
			timersoc := noc.GetOrCreateTimers()
			timersoc.MinimumAdvertisementInterval = ygot.Float64(float64(5))
			return noc
		}(),
	}, {
		desc: "hold time",
		timers: Timers{
			HoldTime: 5 * time.Second,
		},
		want: func() *oc.NetworkInstance_Protocol_Bgp_Neighbor {
			noc := &oc.NetworkInstance_Protocol_Bgp_Neighbor{
				NeighborAddress: ygot.String("1.2.3.4"),
			}
			timersoc := noc.GetOrCreateTimers()
			timersoc.HoldTime = ygot.Float64(float64(5))
			return noc
		}(),
	}, {
		desc: "Keepalive interval",
		timers: Timers{
			KeepaliveInterval: 5 * time.Second,
		},
		want: func() *oc.NetworkInstance_Protocol_Bgp_Neighbor {
			noc := &oc.NetworkInstance_Protocol_Bgp_Neighbor{
				NeighborAddress: ygot.String("1.2.3.4"),
			}
			timersoc := noc.GetOrCreateTimers()
			timersoc.KeepaliveInterval = ygot.Float64(float64(5))
			return noc
		}(),
	}, {
		desc: "Connect Retry",
		timers: Timers{
			ConnectRetry: 5 * time.Second,
		},
		want: func() *oc.NetworkInstance_Protocol_Bgp_Neighbor {
			noc := &oc.NetworkInstance_Protocol_Bgp_Neighbor{
				NeighborAddress: ygot.String("1.2.3.4"),
			}
			timersoc := noc.GetOrCreateTimers()
			timersoc.ConnectRetry = ygot.Float64(float64(5))
			return noc
		}(),
	}}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			n := newNeighbor()
			res := n.WithTimers(test.timers)
			if res == nil {
				t.Fatalf("WithTimers returned nil")
			}

			if diff := cmp.Diff(test.want, &res.oc); diff != "" {
				t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
			}
		})
	}
}

// TestAugmentBGP tests the BGP neighbor augment to BGP global.
func TestAugmentBGP(t *testing.T) {
	tests := []struct {
		desc    string
		bgp     *oc.NetworkInstance_Protocol_Bgp
		wantErr bool
	}{{
		desc: "Empty BGP",
		bgp:  &oc.NetworkInstance_Protocol_Bgp{},
	}, {
		desc: "BGP contains neighbor",
		bgp: func() *oc.NetworkInstance_Protocol_Bgp {
			n := &oc.NetworkInstance_Protocol_Bgp_Neighbor{
				NeighborAddress: ygot.String("1.2.3.4"),
			}
			nibgp := &oc.NetworkInstance_Protocol_Bgp{}
			if err := nibgp.AppendNeighbor(n); err != nil {
				t.Fatalf("unexpected error %v", err)
			}
			return nibgp
		}(),
		wantErr: true,
	}}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			l := newNeighbor()
			dcopy, err := ygot.DeepCopy(test.bgp)
			if err != nil {
				t.Fatalf("unexpected error %v", err)
			}
			wantBGP := dcopy.(*oc.NetworkInstance_Protocol_Bgp)

			err = l.AugmentGlobal(test.bgp)
			if test.wantErr {
				if err == nil {
					t.Fatalf("error expected")
				}
			} else {
				if err != nil {
					t.Fatalf("error not expected")
				}
				if err := wantBGP.AppendNeighbor(&l.oc); err != nil {
					t.Fatalf("unexpected error %v", err)
				}
				if diff := cmp.Diff(wantBGP, test.bgp); diff != "" {
					t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
				}
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
		desc string
		err  error
	}{{
		desc: "error not expected",
	}, {
		desc: "error expected",
		err:  errors.New("some error"),
	}}

	for _, test := range tests {
		n := NewNeighbor("1.2.3.4")
		ff := &FakeNeighborFeature{Err: test.err}
		gotErr := n.WithFeature(ff)
		if !ff.augmentCalled {
			t.Errorf("AugmentNeighbor was not called")
		}
		if ff.oc != &n.oc {
			t.Errorf("neighbor ptr is not equal")
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
