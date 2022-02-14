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
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/yang/oc"
	"github.com/openconfig/ygot/ygot"
)

// TestAugmentNeighbor tests the BGP GR augment to BGP neighbor.
func TestAugmentNeighbor(t *testing.T) {
	tests := []struct {
		desc         string
		gr           *GracefulRestart
		wantNeighbor *oc.NetworkInstance_Protocol_Bgp_Neighbor
	}{{
		desc: "No params",
		gr: func() *GracefulRestart {
			return New()
		}(),
		wantNeighbor: &oc.NetworkInstance_Protocol_Bgp_Neighbor{
			GracefulRestart: &oc.NetworkInstance_Protocol_Bgp_Neighbor_GracefulRestart{
				Enabled: ygot.Bool(true),
			},
		},
	}, {
		desc: "with restart-time",
		gr: func() *GracefulRestart {
			return New().WithRestartTime(5 * time.Second)
		}(),
		wantNeighbor: &oc.NetworkInstance_Protocol_Bgp_Neighbor{
			GracefulRestart: &oc.NetworkInstance_Protocol_Bgp_Neighbor_GracefulRestart{
				Enabled:     ygot.Bool(true),
				RestartTime: ygot.Uint16(5),
			},
		},
	}, {
		desc: "with stale-routes-time",
		gr: func() *GracefulRestart {
			// TODO: ygot validation bug where only 0 is accepted
			return New().WithStaleRoutesTime(0 * time.Second)
		}(),
		wantNeighbor: &oc.NetworkInstance_Protocol_Bgp_Neighbor{
			GracefulRestart: &oc.NetworkInstance_Protocol_Bgp_Neighbor_GracefulRestart{
				Enabled:         ygot.Bool(true),
				StaleRoutesTime: ygot.Float64(0),
			},
		},
	}, {
		desc: "with helper-only",
		gr: func() *GracefulRestart {
			return New().WithHelperOnly(true)
		}(),
		wantNeighbor: &oc.NetworkInstance_Protocol_Bgp_Neighbor{
			GracefulRestart: &oc.NetworkInstance_Protocol_Bgp_Neighbor_GracefulRestart{
				Enabled:    ygot.Bool(true),
				HelperOnly: ygot.Bool(true),
			},
		},
	}}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			inNeighbor := &oc.NetworkInstance_Protocol_Bgp_Neighbor{}
			err := test.gr.AugmentNeighbor(inNeighbor)
			if err != nil {
				t.Fatalf("Error not expected: %v", err)
			}
			if diff := cmp.Diff(test.wantNeighbor, inNeighbor); diff != "" {
				t.Errorf("Did not get expected state, diff(-want,+got):\n%s", diff)
			}
		})
	}
}

// TestAugmentNeighbor_Errors tests the BGP GR augment to BGP neighbor errors.
func TestAugmentNeighbor_Errors(t *testing.T) {
	tests := []struct {
		desc          string
		gr            *GracefulRestart
		inNeighbor    *oc.NetworkInstance_Protocol_Bgp_Neighbor
		wantErrSubStr string
	}{{
		desc: "Neighbor already contains GR",
		gr: func() *GracefulRestart {
			return New()
		}(),
		inNeighbor: &oc.NetworkInstance_Protocol_Bgp_Neighbor{
			GracefulRestart: &oc.NetworkInstance_Protocol_Bgp_Neighbor_GracefulRestart{
				Enabled: ygot.Bool(true),
			},
		},
		wantErrSubStr: "field is not nil",
	}}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			err := test.gr.AugmentNeighbor(test.inNeighbor)
			if err == nil {
				t.Fatalf("Error expected")
			}
			if !strings.Contains(err.Error(), test.wantErrSubStr) {
				t.Errorf("Error strings are not equal; got %v", err)
			}
		})
	}
}

// TestAugmentPeerGroup tests the BGP GR augment to BGP neighbor.
func TestAugmentPeerGroup(t *testing.T) {
	tests := []struct {
		desc   string
		gr     *GracefulRestart
		wantPG *oc.NetworkInstance_Protocol_Bgp_PeerGroup
	}{{
		desc: "No params",
		gr: func() *GracefulRestart {
			return New()
		}(),
		wantPG: &oc.NetworkInstance_Protocol_Bgp_PeerGroup{
			GracefulRestart: &oc.NetworkInstance_Protocol_Bgp_PeerGroup_GracefulRestart{
				Enabled: ygot.Bool(true),
			},
		},
	}, {
		desc: "with restart-time",
		gr: func() *GracefulRestart {
			return New().WithRestartTime(5 * time.Second)
		}(),
		wantPG: &oc.NetworkInstance_Protocol_Bgp_PeerGroup{
			GracefulRestart: &oc.NetworkInstance_Protocol_Bgp_PeerGroup_GracefulRestart{
				Enabled:     ygot.Bool(true),
				RestartTime: ygot.Uint16(5),
			},
		},
	}, {
		desc: "with stale-routes-time",
		gr: func() *GracefulRestart {
			// TODO: ygot validation bug where only 0 is accepted
			return New().WithStaleRoutesTime(0 * time.Second)
		}(),
		wantPG: &oc.NetworkInstance_Protocol_Bgp_PeerGroup{
			GracefulRestart: &oc.NetworkInstance_Protocol_Bgp_PeerGroup_GracefulRestart{
				Enabled:         ygot.Bool(true),
				StaleRoutesTime: ygot.Float64(0),
			},
		},
	}, {
		desc: "with helper-only",
		gr: func() *GracefulRestart {
			return New().WithHelperOnly(true)
		}(),
		wantPG: &oc.NetworkInstance_Protocol_Bgp_PeerGroup{
			GracefulRestart: &oc.NetworkInstance_Protocol_Bgp_PeerGroup_GracefulRestart{
				Enabled:    ygot.Bool(true),
				HelperOnly: ygot.Bool(true),
			},
		},
	}}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			inPG := &oc.NetworkInstance_Protocol_Bgp_PeerGroup{}
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

// TestAugmentPeerGroup_Errors tests the BGP GR augment to BGP peer-group errors.
func TestAugmentPeerGroup_Errors(t *testing.T) {
	tests := []struct {
		desc          string
		gr            *GracefulRestart
		inPG          *oc.NetworkInstance_Protocol_Bgp_PeerGroup
		wantErrSubStr string
	}{{
		desc: "PeerGroup already contains GR",
		gr: func() *GracefulRestart {
			return New()
		}(),
		inPG: &oc.NetworkInstance_Protocol_Bgp_PeerGroup{
			GracefulRestart: &oc.NetworkInstance_Protocol_Bgp_PeerGroup_GracefulRestart{
				Enabled: ygot.Bool(true),
			},
		},
		wantErrSubStr: "field is not nil",
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
