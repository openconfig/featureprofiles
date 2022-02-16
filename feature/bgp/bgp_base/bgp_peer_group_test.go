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
		PeerGroupName: ygot.String(peerGroupName),
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
		PeerGroupName: ygot.String(peerGroupName),
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
		PeerGroupName: ygot.String(peerGroupName),
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

// TestPGWithTransportPassiveMode tests transport passive-mode for BGP neighbor.
func TestPGWithTransportPassiveMode(t *testing.T) {
	want := oc.NetworkInstance_Protocol_Bgp_PeerGroup{
		PeerGroupName: ygot.String(peerGroupName),
		Transport: &oc.NetworkInstance_Protocol_Bgp_PeerGroup_Transport{
			PassiveMode: ygot.Bool(true),
		},
	}

	n := newPeerGroup()
	res := n.WithTransportPassiveMode(true)
	if res == nil {
		t.Fatalf("WithTransportPassiveMode returned nil")
	}

	if diff := cmp.Diff(want, res.oc); diff != "" {
		t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
	}
}

// TestPGWithTransportTCPMSS tests transport tcp-mss for BGP neighbor.
func TestPGWithTransportTCPMSS(t *testing.T) {
	tcpmss := uint16(12345)
	want := oc.NetworkInstance_Protocol_Bgp_PeerGroup{
		PeerGroupName: ygot.String(peerGroupName),
		Transport: &oc.NetworkInstance_Protocol_Bgp_PeerGroup_Transport{
			TcpMss: ygot.Uint16(tcpmss),
		},
	}

	n := newPeerGroup()
	res := n.WithTransportTCPMSS(tcpmss)
	if res == nil {
		t.Fatalf("WithTransportTCPMSS returned nil")
	}

	if diff := cmp.Diff(want, res.oc); diff != "" {
		t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
	}
}

// TestPGWithTransportMTUDiscovery tests transport mtu-discovery for BGP neighbor.
func TestPGWithTransportMTUDiscovery(t *testing.T) {
	want := oc.NetworkInstance_Protocol_Bgp_PeerGroup{
		PeerGroupName: ygot.String(peerGroupName),
		Transport: &oc.NetworkInstance_Protocol_Bgp_PeerGroup_Transport{
			MtuDiscovery: ygot.Bool(true),
		},
	}

	n := newPeerGroup()
	res := n.WithTransportMTUDiscovery(true)
	if res == nil {
		t.Fatalf("WithTransportMTUDiscovery returned nil")
	}

	if diff := cmp.Diff(want, res.oc); diff != "" {
		t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
	}
}

// TestPGWithTransportLocalAddress tests transport mtu-discovery for BGP neighbor.
func TestPGWithTransportLocalAddress(t *testing.T) {
	localAddr := "1.2.3.4"
	want := oc.NetworkInstance_Protocol_Bgp_PeerGroup{
		PeerGroupName: ygot.String(peerGroupName),
		Transport: &oc.NetworkInstance_Protocol_Bgp_PeerGroup_Transport{
			LocalAddress: ygot.String(localAddr),
		},
	}

	n := newPeerGroup()
	res := n.WithTransportLocalAddress(localAddr)
	if res == nil {
		t.Fatalf("WithTransportLocalAddress returned nil")
	}

	if diff := cmp.Diff(want, res.oc); diff != "" {
		t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
	}
}

// TestPGWithLocalAS tests setting local AS for BGP global.
func TestPGWithLocalAS(t *testing.T) {
	as := uint32(1234)
	want := oc.NetworkInstance_Protocol_Bgp_PeerGroup{
		PeerGroupName: ygot.String(peerGroupName),
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
		PeerGroupName: ygot.String(peerGroupName),
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
		PeerGroupName: ygot.String(peerGroupName),
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
		PeerGroupName:   ygot.String(peerGroupName),
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
		PeerGroupName: ygot.String(peerGroupName),
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

// TestPGWithV4PrefixLimit tests setting auth-password for BGP neighbor.
func TestPGWithV4PrefixLimit(t *testing.T) {
	maxPrefixes := uint32(2000)
	tests := []struct {
		desc string
		pl   PrefixLimitOptions
		want *oc.NetworkInstance_Protocol_Bgp_PeerGroup
	}{{
		desc: "max prefixes only",
		want: func() *oc.NetworkInstance_Protocol_Bgp_PeerGroup {
			noc := &oc.NetworkInstance_Protocol_Bgp_PeerGroup{
				PeerGroupName: ygot.String(peerGroupName),
			}
			ploc := noc.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).GetOrCreateIpv4Unicast().GetOrCreatePrefixLimit()
			ploc.MaxPrefixes = ygot.Uint32(maxPrefixes)
			ploc.PreventTeardown = ygot.Bool(false)
			return noc
		}(),
	}, {
		desc: "Prevent teardown",
		pl: PrefixLimitOptions{
			PreventTeardown: true,
		},
		want: func() *oc.NetworkInstance_Protocol_Bgp_PeerGroup {
			noc := &oc.NetworkInstance_Protocol_Bgp_PeerGroup{
				PeerGroupName: ygot.String(peerGroupName),
			}
			ploc := noc.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).GetOrCreateIpv4Unicast().GetOrCreatePrefixLimit()
			ploc.MaxPrefixes = ygot.Uint32(maxPrefixes)
			ploc.PreventTeardown = ygot.Bool(true)
			return noc
		}(),
	}, {
		desc: "Restart timer",
		pl: PrefixLimitOptions{
			RestartTimer: 5 * time.Second,
		},
		want: func() *oc.NetworkInstance_Protocol_Bgp_PeerGroup {
			noc := &oc.NetworkInstance_Protocol_Bgp_PeerGroup{
				PeerGroupName: ygot.String(peerGroupName),
			}
			ploc := noc.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).GetOrCreateIpv4Unicast().GetOrCreatePrefixLimit()
			ploc.MaxPrefixes = ygot.Uint32(maxPrefixes)
			ploc.RestartTimer = ygot.Float64(float64(5))
			ploc.PreventTeardown = ygot.Bool(false)
			return noc
		}(),
	}, {
		desc: "Warning threshold",
		pl: PrefixLimitOptions{
			WarningThresholdPct: 90,
		},
		want: func() *oc.NetworkInstance_Protocol_Bgp_PeerGroup {
			noc := &oc.NetworkInstance_Protocol_Bgp_PeerGroup{
				PeerGroupName: ygot.String(peerGroupName),
			}
			ploc := noc.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).GetOrCreateIpv4Unicast().GetOrCreatePrefixLimit()
			ploc.MaxPrefixes = ygot.Uint32(maxPrefixes)
			ploc.WarningThresholdPct = ygot.Uint8(90)
			ploc.PreventTeardown = ygot.Bool(false)
			return noc
		}(),
	}}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			n := newPeerGroup()
			res := n.WithV4PrefixLimit(maxPrefixes, test.pl)
			if res == nil {
				t.Fatalf("WithV4PrefixLimit returned nil")
			}

			if diff := cmp.Diff(test.want, &res.oc); diff != "" {
				t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
			}
		})
	}
}

// TestPGWithKeepAliveInterval tests setting keep-alive for BGP neighbor.
func TestPGWithKeepAliveInterval(t *testing.T) {
	keepalive := 5 * time.Second
	hold := 15 * time.Second
	want := oc.NetworkInstance_Protocol_Bgp_PeerGroup{
		PeerGroupName: ygot.String(peerGroupName),
		Timers: &oc.NetworkInstance_Protocol_Bgp_PeerGroup_Timers{
			HoldTime:          ygot.Float64(hold.Seconds()),
			KeepaliveInterval: ygot.Float64(keepalive.Seconds()),
		},
	}

	n := newPeerGroup()
	res := n.WithKeepAliveInterval(keepalive, hold)
	if res == nil {
		t.Fatalf("WithKeepAliveInterval returned nil")
	}

	if diff := cmp.Diff(want, res.oc); diff != "" {
		t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
	}
}

// TestPGWithMinimumAdvertisementInterval tests setting keep-alive for BGP neighbor.
func TestPGWithMinimumAdvertisementInterval(t *testing.T) {
	minAdvIntv := 5 * time.Second
	want := oc.NetworkInstance_Protocol_Bgp_PeerGroup{
		PeerGroupName: ygot.String(peerGroupName),
		Timers: &oc.NetworkInstance_Protocol_Bgp_PeerGroup_Timers{
			MinimumAdvertisementInterval: ygot.Float64(minAdvIntv.Seconds()),
		},
	}

	n := newPeerGroup()
	res := n.WithMinimumAdvertisementInterval(minAdvIntv)
	if res == nil {
		t.Fatalf("WithMinimumAdvertisementInterval returned nil")
	}

	if diff := cmp.Diff(want, res.oc); diff != "" {
		t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
	}
}

// TestPGWithConnectRetry tests setting keep-alive for BGP neighbor.
func TestPGWithConnectRetry(t *testing.T) {
	connectRetry := 5 * time.Second
	want := oc.NetworkInstance_Protocol_Bgp_PeerGroup{
		PeerGroupName: ygot.String(peerGroupName),
		Timers: &oc.NetworkInstance_Protocol_Bgp_PeerGroup_Timers{
			ConnectRetry: ygot.Float64(connectRetry.Seconds()),
		},
	}

	n := newPeerGroup()
	res := n.WithConnectRetry(connectRetry)
	if res == nil {
		t.Fatalf("WithConnectRetry returned nil")
	}

	if diff := cmp.Diff(want, res.oc); diff != "" {
		t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
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
				PeerGroupName: ygot.String(peerGroupName),
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
					PeerGroupName: ygot.String(peerGroupName),
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
		n := NewPeerGroup(peerGroupName)
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
