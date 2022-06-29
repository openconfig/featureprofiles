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

package gracefulrestart

import (
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/yang/fpoc"
	"github.com/openconfig/ygot/ygot"
)

// TestAugmentNeighbor tests the BGP GR augment to BGP neighbor.
func TestAugmentNeighbor(t *testing.T) {
	tests := []struct {
		desc         string
		gr           *GracefulRestart
		inNeighbor   *fpoc.NetworkInstance_Protocol_Bgp_Neighbor
		wantNeighbor *fpoc.NetworkInstance_Protocol_Bgp_Neighbor
	}{{
		desc:       "GR enabled with no params",
		gr:         New(),
		inNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{},
		wantNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
			GracefulRestart: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_GracefulRestart{
				Enabled: ygot.Bool(true),
			},
		},
	}, {
		desc:       "With restart-time",
		gr:         New().WithRestartTime(5 * time.Second),
		inNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{},
		wantNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
			GracefulRestart: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_GracefulRestart{
				Enabled:     ygot.Bool(true),
				RestartTime: ygot.Uint16(5),
			},
		},
	}, {
		desc:       "With stale-routes-time",
		gr:         New().WithStaleRoutesTime(60 * time.Second),
		inNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{},
		wantNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
			GracefulRestart: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_GracefulRestart{
				Enabled:         ygot.Bool(true),
				StaleRoutesTime: ygot.Float64(60),
			},
		},
	}, {
		desc:       "With helper-only",
		gr:         New().WithHelperOnly(true),
		inNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{},
		wantNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
			GracefulRestart: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_GracefulRestart{
				Enabled:    ygot.Bool(true),
				HelperOnly: ygot.Bool(true),
			},
		},
	}, {
		desc: "Neighbor contains graceful-restart, no conflicts",
		gr:   New().WithHelperOnly(true),
		inNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
			GracefulRestart: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_GracefulRestart{
				Enabled: ygot.Bool(true),
			},
		},
		wantNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
			GracefulRestart: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_GracefulRestart{
				Enabled:    ygot.Bool(true),
				HelperOnly: ygot.Bool(true),
			},
		},
	}}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			err := test.gr.AugmentNeighbor(test.inNeighbor)
			if err != nil {
				t.Fatalf("Error not expected: %v", err)
			}
			if diff := cmp.Diff(test.wantNeighbor, test.inNeighbor); diff != "" {
				t.Errorf("Did not get expected state, diff(-want,+got):\n%s", diff)
			}
		})
	}
}

// TestAugmentNeighborErrors tests the BGP GR augment to BGP neighbor errors.
func TestAugmentNeighborErrors(t *testing.T) {
	tests := []struct {
		desc          string
		gr            *GracefulRestart
		inNeighbor    *fpoc.NetworkInstance_Protocol_Bgp_Neighbor
		wantErrSubStr string
	}{{
		desc: "Neighbor contains GR with conflicts",
		gr:   New(),
		inNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
			GracefulRestart: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_GracefulRestart{
				Enabled: ygot.Bool(false),
			},
		},
		wantErrSubStr: "destination value was set",
	}}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			err := test.gr.AugmentNeighbor(test.inNeighbor)
			if err == nil {
				t.Fatalf("Error expected")
			}
			if !strings.Contains(err.Error(), test.wantErrSubStr) {
				t.Errorf("Error strings are not equal: %v", err)
			}
		})
	}
}

// TestAugmentPeerGroup tests the BGP GR augment to BGP neighbor.
func TestAugmentPeerGroup(t *testing.T) {
	tests := []struct {
		desc   string
		gr     *GracefulRestart
		inPG   *fpoc.NetworkInstance_Protocol_Bgp_PeerGroup
		wantPG *fpoc.NetworkInstance_Protocol_Bgp_PeerGroup
	}{{
		desc: "GR enabled with no params",
		gr:   New(),
		inPG: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{},
		wantPG: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
			GracefulRestart: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_GracefulRestart{
				Enabled: ygot.Bool(true),
			},
		},
	}, {
		desc: "With restart-time",
		gr:   New().WithRestartTime(5 * time.Second),
		inPG: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{},
		wantPG: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
			GracefulRestart: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_GracefulRestart{
				Enabled:     ygot.Bool(true),
				RestartTime: ygot.Uint16(5),
			},
		},
	}, {
		desc: "With stale-routes-time",
		gr:   New().WithStaleRoutesTime(60 * time.Second),
		inPG: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{},
		wantPG: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
			GracefulRestart: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_GracefulRestart{
				Enabled:         ygot.Bool(true),
				StaleRoutesTime: ygot.Float64(60),
			},
		},
	}, {
		desc: "With helper-only",
		gr:   New().WithHelperOnly(true),
		inPG: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{},
		wantPG: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
			GracefulRestart: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_GracefulRestart{
				Enabled:    ygot.Bool(true),
				HelperOnly: ygot.Bool(true),
			},
		},
	}, {
		desc: "Peer-group contains GR, no conflicts",
		gr:   New().WithHelperOnly(true),
		inPG: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
			GracefulRestart: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_GracefulRestart{
				Enabled: ygot.Bool(true),
			},
		},
		wantPG: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
			GracefulRestart: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_GracefulRestart{
				Enabled:    ygot.Bool(true),
				HelperOnly: ygot.Bool(true),
			},
		},
	}}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			inPG := &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{}
			err := test.gr.AugmentPeerGroup(inPG)
			if err != nil {
				t.Fatalf("Error not expected: %v", err)
			}
			if diff := cmp.Diff(test.wantPG, inPG); diff != "" {
				t.Errorf("Did not get expected state, diff(-want,+got):\n%s", diff)
			}
		})
	}
}

// TestAugmentPeerGroupErrors tests the BGP GR augment to BGP peer-group errors.
func TestAugmentPeerGroupErrors(t *testing.T) {
	tests := []struct {
		desc          string
		gr            *GracefulRestart
		inPG          *fpoc.NetworkInstance_Protocol_Bgp_PeerGroup
		wantErrSubStr string
	}{{
		desc: "PeerGroup contains GR with conflicts",
		gr:   New(),
		inPG: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
			GracefulRestart: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_GracefulRestart{
				Enabled: ygot.Bool(false),
			},
		},
		wantErrSubStr: "destination value was set",
	}}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			err := test.gr.AugmentPeerGroup(test.inPG)
			if err == nil {
				t.Fatalf("Error expected")
			}
			if !strings.Contains(err.Error(), test.wantErrSubStr) {
				t.Errorf("Error strings are not equal; got %v", err)
			}
		})
	}
}
