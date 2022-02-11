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

package bgpgr

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/yang/oc"
	"github.com/openconfig/ygot/ygot"
)

// TestNew tests the New function.
func TestNew(t *testing.T) {
	want := &GracefulRestart{}
	got := New()
	if got == nil {
		t.Fatalf("New returned nil")
	}
	if diff := cmp.Diff(want.noc, got.noc); diff != "" {
		t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
	}
	if diff := cmp.Diff(want.poc, got.poc); diff != "" {
		t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
	}
}

// TestWithRestartTime tests GR restart time.
func TestWithRestartTime(t *testing.T) {
	rt := 5 * time.Second
	wantnoc := oc.NetworkInstance_Protocol_Bgp_Neighbor_GracefulRestart{
		RestartTime: ygot.Uint16(uint16(rt.Seconds())),
	}
	wantpoc := oc.NetworkInstance_Protocol_Bgp_PeerGroup_GracefulRestart{
		RestartTime: ygot.Uint16(uint16(rt.Seconds())),
	}

	gr := &GracefulRestart{}
	res := gr.WithRestartTime(rt)
	if res == nil {
		t.Fatalf("WithRestartTime returned nil")
	}

	if diff := cmp.Diff(wantnoc, res.noc); diff != "" {
		t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
	}
	if diff := cmp.Diff(wantpoc, res.poc); diff != "" {
		t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
	}
}

// TestWithStaleRoutesTime tests GR stale routes time.
func TestWithStaleRoutesTime(t *testing.T) {
	rt := 5 * time.Second
	wantnoc := oc.NetworkInstance_Protocol_Bgp_Neighbor_GracefulRestart{
		StaleRoutesTime: ygot.Float64(rt.Seconds()),
	}
	wantpoc := oc.NetworkInstance_Protocol_Bgp_PeerGroup_GracefulRestart{
		StaleRoutesTime: ygot.Float64(rt.Seconds()),
	}

	gr := &GracefulRestart{}
	res := gr.WithStaleRoutesTime(rt)
	if res == nil {
		t.Fatalf("WithStaleRoutesTime returned nil")
	}

	if diff := cmp.Diff(wantnoc, res.noc); diff != "" {
		t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
	}
	if diff := cmp.Diff(wantpoc, res.poc); diff != "" {
		t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
	}
}

// TestWithHelperOnly tests GR helper only.
func TestWithHelperOnly(t *testing.T) {
	wantnoc := oc.NetworkInstance_Protocol_Bgp_Neighbor_GracefulRestart{
		HelperOnly: ygot.Bool(true),
	}
	wantpoc := oc.NetworkInstance_Protocol_Bgp_PeerGroup_GracefulRestart{
		HelperOnly: ygot.Bool(true),
	}

	gr := &GracefulRestart{}
	res := gr.WithHelperOnly(true)
	if res == nil {
		t.Fatalf("WithHelperOnly returned nil")
	}

	if diff := cmp.Diff(wantnoc, res.noc); diff != "" {
		t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
	}
	if diff := cmp.Diff(wantpoc, res.poc); diff != "" {
		t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
	}
}

// TestAugmentNeighbor tests the BGP GR augment to BGP neighbor.
func TestAugmentNeighbor(t *testing.T) {
	tests := []struct {
		desc    string
		bgp     *oc.NetworkInstance_Protocol_Bgp_Neighbor
		wantErr bool
	}{{
		desc: "Empty Neighbor",
		bgp:  &oc.NetworkInstance_Protocol_Bgp_Neighbor{},
	}, {
		desc: "Neighbor contains GR",
		bgp: &oc.NetworkInstance_Protocol_Bgp_Neighbor{
			GracefulRestart: &oc.NetworkInstance_Protocol_Bgp_Neighbor_GracefulRestart{},
		},
		wantErr: true,
	}}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			gr := &GracefulRestart{}
			dcopy, err := ygot.DeepCopy(test.bgp)
			if err != nil {
				t.Fatalf("unexpected error %v", err)
			}
			wantBGP := dcopy.(*oc.NetworkInstance_Protocol_Bgp_Neighbor)

			err = gr.AugmentNeighbor(test.bgp)
			if test.wantErr {
				if err == nil {
					t.Fatalf("error expected")
				}
			} else {
				if err != nil {
					t.Fatalf("error not expected")
				}
				wantBGP.GracefulRestart = &gr.noc
				if diff := cmp.Diff(wantBGP, test.bgp); diff != "" {
					t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
				}
			}
		})
	}
}

// TestAugmentPeerGroup tests the BGP GR augment to BGP neighbor.
func TestAugmentPeerGroup(t *testing.T) {
	tests := []struct {
		desc    string
		bgp     *oc.NetworkInstance_Protocol_Bgp_PeerGroup
		wantErr bool
	}{{
		desc: "Empty PeerGroup",
		bgp:  &oc.NetworkInstance_Protocol_Bgp_PeerGroup{},
	}, {
		desc: "PeerGroup contains GR",
		bgp: &oc.NetworkInstance_Protocol_Bgp_PeerGroup{
			GracefulRestart: &oc.NetworkInstance_Protocol_Bgp_PeerGroup_GracefulRestart{},
		},
		wantErr: true,
	}}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			gr := &GracefulRestart{}
			dcopy, err := ygot.DeepCopy(test.bgp)
			if err != nil {
				t.Fatalf("unexpected error %v", err)
			}
			wantBGP := dcopy.(*oc.NetworkInstance_Protocol_Bgp_PeerGroup)

			err = gr.AugmentPeerGroup(test.bgp)
			if test.wantErr {
				if err == nil {
					t.Fatalf("error expected")
				}
			} else {
				if err != nil {
					t.Fatalf("error not expected")
				}
				wantBGP.GracefulRestart = &gr.poc
				if diff := cmp.Diff(wantBGP, test.bgp); diff != "" {
					t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
				}
			}
		})
	}
}
