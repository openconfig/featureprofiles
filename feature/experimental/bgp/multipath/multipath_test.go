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

package multipath

import (
     "strings"
     "testing"

     "github.com/google/go-cmp/cmp"
     "github.com/openconfig/featureprofiles/yang/fpoc"
     "github.com/openconfig/ygot/ygot"
)

// TestAugmentNeighbor tests the BGP MP augment to BGP neighbor.
func TestAugmentNeighbor(t *testing.T) {
     tests := []struct {
          desc string
          mp *Multipath
          inNeighbor *fpoc.NetworkInstance_Protocol_Bgp_Neighbor
          wantNeighbor *fpoc.NetworkInstance_Protocol_Bgp_Neighbor
     }{{
          desc: "MP enabled with no params",
          mp: New(),
          inNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{},
          wantNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
               UseMultiplePaths: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_UseMultiplePaths{
                    Enabled: ygot.Bool(true),
               },
          },
     }, {
          desc: "With ebgp allow-multiple-as",
          mp: New().WithEbgpAllowMultipleAs(true),
          inNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{},
          wantNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
               UseMultiplePaths: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_UseMultiplePaths{
                    Enabled: ygot.Bool(true),
                    Egbp: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_UseMultiplePaths_Ebgp{
                         AllowMultipleAs: ygot.Bool(true),
                    },
               },
          },
     }, {
          desc: "With ebgp maximum-paths",
          mp: New().WithEbgpMaximumPaths(5),
          inNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{},
          wantNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
               UseMultiplePaths: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_UseMultiplePaths{
                    Enabled: ygot.Bool(true),
                    Ebgp: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_UseMultiplePaths_Ebgp{
                         MaximumPaths: ygot.Uint32(5),
                    },
               },
          },
     }, {
          desc: "With ibgp maximum-paths",
          mp: New().WithIbgpMaximumPaths(5),
          inNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{},
          wantNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
               UseMultiplePaths: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_UseMultiplePaths{
                    Enabled: ygot.Bool(true),
                    Ibgp: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_UseMultiplePaths_Ibgp{
                         MaximumPaths: ygot.Uint32(5),
                    },
               },
          },
     }, {
          desc: "Neighbor contains multipath, no conflicts",
          mp: New().WithEbgpAllowMultipleAs(true),
          inNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
               UseMultiplePaths: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_UseMultiplePaths{
                    Enabled: ygot.Bool(true),
               },
          },
          wantNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
               UseMultiplePaths: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_UseMultiplePaths{
                    Enabled: ygot.Bool(true),
                    Ebgp: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_UseMultiplePaths_Ebgp{
                         AllowMultipleAs: ygot.Bool(true),
                    },
               },
          },
     }}

     for _, test := range tests {
          t.Run(test.desc, func(t *testing.T) {
               err := test.mp.AugmentNeighbor(test.inNeighbor)
               if err != nil {
                    t.Fatalf("Error not expected: %v", err)
               }
               if diff := cmp.Diff(test.wantNeighbor, test.inNeighbor); diff != "" {
                    t.Errorf("Did not get expected state, diff(-want,+got):\n%s", diff)
               }
          })
     }
}

// TestAugmentNeighborErrors tests the BGP MP augment to BGP neighbor errors.
func TestAugmentNeighborErrors(t *testing.T) {
     tests := []struct {
          desc string
          mp *Multipath
          inNeighbor *fpoc.NetworkInstance_Protocol_Bgp_Neighbor
          wantErrSubStr string
     }{{
          desc: "Neighbor contains MP with conflicts",
          mp: New(),
          inNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
               UseMultiplePaths: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_UseMultiplePaths{
                    Enabled: ygot.Bool(false),
               },
          },
          wantErrSubStr: "destination value was set",
     }}

     for _, test := range tests {
          t.Run(test.desc, func(t *testing.T) {
               err := test.mp.AugmentNeighbor(test.inNeighbor)
               if err == nil {
                    t.Fatalf("error expected")
               }
               if !strings.Contains(err.Error(), test.wantErrSubStr) {
                    t.Errorf("error strings are not equal: %v", err)
               }
          })
     }
}

// TestAugmentPeerGroup tests the BGP MP augment to BGP peer group.
func TestAugmentPeerGroup(t *testing.T) {
     tests := []struct {
          desc string
          mp *Multipath
          inPG *fpoc.NetworkInstance_Protocol_Bgp_PeerGroup
          wantPG *fpoc.NetworkInstance_Protocol_Bgp_PeerGroup
     }{{
          desc: "MP enabled with no params",
          mp: New(),
          inPG: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{},
          wantPG: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
               UseMultiplePaths: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_UseMultiplePaths{
                    Enabled: ygot.Bool(true),
               },
          },
     }, {
          desc: "With ebgp allow-multiple-as",
          mp: New().WithEbgpAllowMultipleAs(true),
          inPG: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{},
          wantPG: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
               UseMultiplePaths: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_UseMultiplePaths{
                    Enabled: ygot.Bool(true),
                    Ebgp: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_UseMultiplePaths_Ebgp{
                         AllowMultipleAs: ygot.Bool(true),
                    },
               },
          },
     }, {
          desc: "With ebgp maximum-paths",
          mp: New().WithEbgpMaximumPaths(5),
          inPG: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{},
          wantPG: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
               UseMultiplePaths: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_UseMultiplePaths{
                    Enabled: ygot.Bool(true),
                    Ebgp: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_UseMultiplePaths_Ebgp{
                         MaximumPaths: ygot.Uint32(5),
                    },
               },
          },
     }, {
          desc: "With ibgp maximum-paths",
          mp: New().WithIbgpMaximumPaths(5),
          inPG: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{},
          wantPG: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
               UseMultiplePaths: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_UseMultiplePaths{
                    Enabled: ygot.Bool(true),
                    Ibgp: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_UseMultiplePaths_Ibgp{
                         MaximumPaths: ygot.Uint32(5),
                    },
               },
          },
     }, {
          desc: "Peer group contains multipath, no conflicts",
          mp: New().WithEbgpAllowMultipleAs(true),
          inPG: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
               UseMultiplePaths: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_UseMultiplePaths{
                    Enabled: ygot.Bool(true),
               },
          },
          wantPG: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
               UseMultiplePaths: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_UseMultiplePaths{
                    Enabled: ygot.Bool(true),
                    Ebgp: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_UseMultiplePaths_Ebgp{
                         AllowMultipleAs: ygot.Bool(true),
                    },
               },
          },
     }}

     for _, test := range tests {
          t.Run(test.desc, func(t *testing.T) {
               err := test.mp.AugmentPeerGroup(test.inPG)
               if err != nil {
                    t.Fatalf("Error not expected: %v", err)
               }
               if diff := cmp.Diff(test.wantPG, test.inPG); diff != "" {
                    t.Errorf("Did not get expected state, diff(-want,+got):\n%s", diff)
               }
          })
     }
}

// TestAugmentPeerGroupErrors tests the BGP AP augment to BGP peer group errors.
func TestAugmentPeerGroupErrors(t *testing.T) {
     tests := []struct {
          desc string
          mp *Multipath
          inPG *fpoc.NetworkInstance_Protocol_Bgp_PeerGroup
          wantErrSubStr string
     }{{
          desc: "PeerGroup contains MP with conflicts",
          mp: New()
          inPG: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
               UseMultiplePaths: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_UseMultiplePaths{
                    Enabled: ygot.Bool(false),
               },
          },
          wantErrSubStr: "destination value was set",
     }}

     for _, test := range tests {
          t.Run(test.desc, func(t *testing.T) {
               err := test.mp.AugmentPeerGroup(test.inPG)
               if err == nil {
                    t.Fatalf("error expected")
               }
               if !strings.Contains(err.Error(), test.wantErrSubStr) {
                    t.Errorf("error strings are not equal; got %v", err)
               }
          })
     }
}