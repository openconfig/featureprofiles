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

var peerGroupName = "GLOBAL-PEER"

func newPeerGroup() *PeerGroup {
	return &PeerGroup{
		oc: oc.NetworkInstance_Protocol_Bgp_PeerGroup{
			PeerGroupName: ygot.String(peerGroupName),
		},
	}
}

// TestNewPeerGroup tests the NewPeerGroup function.
func TestNewPeerGroup(t *testing.T) {
	want := newPeerGroup()
	got := NewPeerGroup(peerGroupName)
	if got == nil {
		t.Fatalf("NewPeerGroup returned nil")
	}
	if diff := cmp.Diff(want.oc, got.oc); diff != "" {
		t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
	}
}

// TestName tests the Name method.
func TestName(t *testing.T) {
	n := newPeerGroup()
	if got, want := n.Name(), peerGroupName; got != want {
		t.Errorf("got %v but expecting %v", got, want)
	}
}

// TestPGWithAFISAFI tests setting AS for BGP peer-group.
func TestPGWithAFISAFI(t *testing.T) {
	afisafi := oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST
	want := oc.NetworkInstance_Protocol_Bgp_PeerGroup{
		PeerGroupName: ygot.String("GLOBAL-PEER"),
		AfiSafi: map[oc.E_BgpTypes_AFI_SAFI_TYPE]*oc.NetworkInstance_Protocol_Bgp_PeerGroup_AfiSafi{
			afisafi: {
				AfiSafiName: afisafi,
				Enabled:     ygot.Bool(true),
			},
		},
	}

	pg := newPeerGroup()
	res := pg.WithAFISAFI(afisafi)
	if res == nil {
		t.Fatalf("WithAFISAFI returned nil")
	}

	if diff := cmp.Diff(want, res.oc); diff != "" {
		t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
	}
}

// TestPGWithAuthPassword tests setting auth-password for BGP peer-group.
func TestPGWithAuthPassword(t *testing.T) {
	pwd := "foobar"
	want := oc.NetworkInstance_Protocol_Bgp_PeerGroup{
		PeerGroupName: ygot.String("GLOBAL-PEER"),
		AuthPassword:  ygot.String(pwd),
	}

	pg := newPeerGroup()
	res := pg.WithAuthPassword(pwd)
	if res == nil {
		t.Fatalf("WithAuthPassword returned nil")
	}

	if diff := cmp.Diff(want, res.oc); diff != "" {
		t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
	}
}

// TestPGWithDescription tests setting description for BGP global.
func TestPGWithDescription(t *testing.T) {
	desc := "foobar"
	want := oc.NetworkInstance_Protocol_Bgp_PeerGroup{
		PeerGroupName: ygot.String("GLOBAL-PEER"),
		Description:   ygot.String(desc),
	}

	pg := newPeerGroup()
	res := pg.WithDescription(desc)
	if res == nil {
		t.Fatalf("WithDescription returned nil")
	}

	if diff := cmp.Diff(want, res.oc); diff != "" {
		t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
	}
}

// TestPGWithTransport tests setting auth-password for BGP global.
func TestPGWithTransport(t *testing.T) {
	tests := []struct {
		desc      string
		transport Transport
		want      *oc.NetworkInstance_Protocol_Bgp_PeerGroup
	}{{
		desc: "passive mode set",
		transport: Transport{
			PassiveMode: true,
		},
		want: func() *oc.NetworkInstance_Protocol_Bgp_PeerGroup {
			oc := &oc.NetworkInstance_Protocol_Bgp_PeerGroup{
				PeerGroupName: ygot.String("GLOBAL-PEER"),
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
		want: func() *oc.NetworkInstance_Protocol_Bgp_PeerGroup {
			oc := &oc.NetworkInstance_Protocol_Bgp_PeerGroup{
				PeerGroupName: ygot.String("GLOBAL-PEER"),
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
		want: func() *oc.NetworkInstance_Protocol_Bgp_PeerGroup {
			oc := &oc.NetworkInstance_Protocol_Bgp_PeerGroup{
				PeerGroupName: ygot.String("GLOBAL-PEER"),
			}
			toc := oc.GetOrCreateTransport()
			toc.MtuDiscovery = ygot.Bool(true)
			toc.PassiveMode = ygot.Bool(false)
			return oc
		}(),
	}, {
		desc: "Local address",
		transport: Transport{
			LocalAddress: "GLOBAL-PEER",
		},
		want: func() *oc.NetworkInstance_Protocol_Bgp_PeerGroup {
			oc := &oc.NetworkInstance_Protocol_Bgp_PeerGroup{
				PeerGroupName: ygot.String("GLOBAL-PEER"),
			}
			toc := oc.GetOrCreateTransport()
			toc.MtuDiscovery = ygot.Bool(false)
			toc.PassiveMode = ygot.Bool(false)
			toc.LocalAddress = ygot.String("GLOBAL-PEER")
			return oc
		}(),
	}}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			n := &PeerGroup{
				oc: oc.NetworkInstance_Protocol_Bgp_PeerGroup{
					PeerGroupName: ygot.String("GLOBAL-PEER"),
				},
			}

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

// TestPGWithLocalAS tests setting local AS for BGP global.
func TestPGWithLocalAS(t *testing.T) {
	as := uint32(1234)
	want := oc.NetworkInstance_Protocol_Bgp_PeerGroup{
		PeerGroupName: ygot.String("GLOBAL-PEER"),
		LocalAs:       ygot.Uint32(as),
	}

	pg := newPeerGroup()
	res := pg.WithLocalAS(as)
	if res == nil {
		t.Fatalf("WithLocalAS returned nil")
	}

	if diff := cmp.Diff(want, res.oc); diff != "" {
		t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
	}
}

// TestPGWithPeerAS tests setting peer AS for BGP global.
func TestPGWithPeerAS(t *testing.T) {
	as := uint32(1234)
	want := oc.NetworkInstance_Protocol_Bgp_PeerGroup{
		PeerGroupName: ygot.String("GLOBAL-PEER"),
		PeerAs:        ygot.Uint32(as),
	}

	pg := newPeerGroup()
	res := pg.WithPeerAS(as)
	if res == nil {
		t.Fatalf("WithPeerAS returned nil")
	}

	if diff := cmp.Diff(want, res.oc); diff != "" {
		t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
	}
}

// TestPGWithPeerType tests setting peer AS for BGP global.
func TestPGWithPeerType(t *testing.T) {
	pt := oc.BgpTypes_PeerType_EXTERNAL
	want := oc.NetworkInstance_Protocol_Bgp_PeerGroup{
		PeerGroupName: ygot.String("GLOBAL-PEER"),
		PeerType:      pt,
	}

	pg := newPeerGroup()
	res := pg.WithPeerType(pt)
	if res == nil {
		t.Fatalf("WithPeerType returned nil")
	}

	if diff := cmp.Diff(want, res.oc); diff != "" {
		t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
	}
}

// TestPGWithRemovePrivateAS tests setting remove private AS for BGP global.
func TestPGWithRemovePrivateAS(t *testing.T) {
	val := oc.BgpTypes_RemovePrivateAsOption_PRIVATE_AS_REMOVE_ALL
	want := oc.NetworkInstance_Protocol_Bgp_PeerGroup{
		PeerGroupName:   ygot.String("GLOBAL-PEER"),
		RemovePrivateAs: val,
	}

	pg := newPeerGroup()
	res := pg.WithRemovePrivateAS(val)
	if res == nil {
		t.Fatalf("WithRemovePrivateAS returned nil")
	}

	if diff := cmp.Diff(want, res.oc); diff != "" {
		t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
	}
}

// TestPGWithSendCommunity tests setting send-community for BGP global.
func TestPGWithSendCommunity(t *testing.T) {
	val := oc.BgpTypes_CommunityType_BOTH
	want := oc.NetworkInstance_Protocol_Bgp_PeerGroup{
		PeerGroupName: ygot.String("GLOBAL-PEER"),
		SendCommunity: val,
	}

	pg := newPeerGroup()
	res := pg.WithSendCommunity(val)
	if res == nil {
		t.Fatalf("WithSendCommunity returned nil")
	}

	if diff := cmp.Diff(want, res.oc); diff != "" {
		t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
	}
}

// TestPGWithV4PrefixLimit tests setting auth-password for BGP global.
func TestPGWithV4PrefixLimit(t *testing.T) {
	tests := []struct {
		desc string
		pl   PrefixLimit
		want *oc.NetworkInstance_Protocol_Bgp_PeerGroup
	}{{
		desc: "max prefixes",
		pl: PrefixLimit{
			MaxPrefixes: 2000,
		},
		want: func() *oc.NetworkInstance_Protocol_Bgp_PeerGroup {
			noc := &oc.NetworkInstance_Protocol_Bgp_PeerGroup{
				PeerGroupName: ygot.String("GLOBAL-PEER"),
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
		want: func() *oc.NetworkInstance_Protocol_Bgp_PeerGroup {
			noc := &oc.NetworkInstance_Protocol_Bgp_PeerGroup{
				PeerGroupName: ygot.String("GLOBAL-PEER"),
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
		want: func() *oc.NetworkInstance_Protocol_Bgp_PeerGroup {
			noc := &oc.NetworkInstance_Protocol_Bgp_PeerGroup{
				PeerGroupName: ygot.String("GLOBAL-PEER"),
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
		want: func() *oc.NetworkInstance_Protocol_Bgp_PeerGroup {
			noc := &oc.NetworkInstance_Protocol_Bgp_PeerGroup{
				PeerGroupName: ygot.String("GLOBAL-PEER"),
			}
			ploc := noc.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).GetOrCreateIpv4Unicast().GetOrCreatePrefixLimit()
			ploc.PreventTeardown = ygot.Bool(false)
			ploc.WarningThresholdPct = ygot.Uint8(90)
			return noc
		}(),
	}}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			pg := newPeerGroup()
			res := pg.WithV4PrefixLimit(test.pl)
			if res == nil {
				t.Fatalf("WithV4PrefixLimit returned nil")
			}

			if diff := cmp.Diff(test.want, &res.oc); diff != "" {
				t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
			}
		})
	}
}

// TestPGWithTimers tests setting auth-password for BGP global.
func TestPGWithTimers(t *testing.T) {
	tests := []struct {
		desc   string
		timers Timers
		want   *oc.NetworkInstance_Protocol_Bgp_PeerGroup
	}{{
		desc: "min advertisement interval",
		timers: Timers{
			MinimumAdvertisementInterval: 5 * time.Second,
		},
		want: func() *oc.NetworkInstance_Protocol_Bgp_PeerGroup {
			noc := &oc.NetworkInstance_Protocol_Bgp_PeerGroup{
				PeerGroupName: ygot.String("GLOBAL-PEER"),
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
		want: func() *oc.NetworkInstance_Protocol_Bgp_PeerGroup {
			noc := &oc.NetworkInstance_Protocol_Bgp_PeerGroup{
				PeerGroupName: ygot.String("GLOBAL-PEER"),
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
		want: func() *oc.NetworkInstance_Protocol_Bgp_PeerGroup {
			noc := &oc.NetworkInstance_Protocol_Bgp_PeerGroup{
				PeerGroupName: ygot.String("GLOBAL-PEER"),
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
		want: func() *oc.NetworkInstance_Protocol_Bgp_PeerGroup {
			noc := &oc.NetworkInstance_Protocol_Bgp_PeerGroup{
				PeerGroupName: ygot.String("GLOBAL-PEER"),
			}
			timersoc := noc.GetOrCreateTimers()
			timersoc.ConnectRetry = ygot.Float64(float64(5))
			return noc
		}(),
	}}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			pg := newPeerGroup()
			res := pg.WithTimers(test.timers)
			if res == nil {
				t.Fatalf("WithTimers returned nil")
			}

			if diff := cmp.Diff(test.want, &res.oc); diff != "" {
				t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
			}
		})
	}
}

// TestPGAugmentBGP tests the BGP neighbor augment to BGP global.
func TestPGAugmentBGP(t *testing.T) {
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
			n := &oc.NetworkInstance_Protocol_Bgp_PeerGroup{
				PeerGroupName: ygot.String("GLOBAL-PEER"),
			}
			nibgp := &oc.NetworkInstance_Protocol_Bgp{}
			if err := nibgp.AppendPeerGroup(n); err != nil {
				t.Fatalf("unexpected error %v", err)
			}
			return nibgp
		}(),
		wantErr: true,
	}}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			l := &PeerGroup{
				oc: oc.NetworkInstance_Protocol_Bgp_PeerGroup{
					PeerGroupName: ygot.String("GLOBAL-PEER"),
				},
			}
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
				if err := wantBGP.AppendPeerGroup(&l.oc); err != nil {
					t.Fatalf("unexpected error %v", err)
				}
				if diff := cmp.Diff(wantBGP, test.bgp); diff != "" {
					t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
				}
			}
		})
	}
}

type FakePeerGroupFeature struct {
	Err           error
	augmentCalled bool
	oc            *oc.NetworkInstance_Protocol_Bgp_PeerGroup
}

func (f *FakePeerGroupFeature) AugmentPeerGroup(oc *oc.NetworkInstance_Protocol_Bgp_PeerGroup) error {
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
