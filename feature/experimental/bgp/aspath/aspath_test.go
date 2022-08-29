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

package aspath

import (
     "strings"
     "testing"

     "github.com/google/go-cmp/cmp"
     "github.com/openconfig/featureprofiles/yang/fpoc"
     "github.com/openconfig/ygot/ygot"
)

// TestAugmentNeighbor tests the BGP AP augment to BGP neighbor.
func TestAugmentNeighbor(t *testing.T) {
     tests := []struct {
          desc string
          ap *AsPathOptions
          inNeighbor *fpoc.NetworkInstance_Protocol_Bgp_Neighbor
          wantNeighbor *fpoc.NetworkInstance_Protocol_Bgp_Neighbor
     }{{
          desc: "AP enabled with no params",
          ap: New(),
          inNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{},
          wantNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
               AsPathOptions: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AsPathOptions{
                    Enabled: ygot.Bool(true),
               },
          },
     }, {
          desc: "With allow-own-as",
          ap: New().WithAllowOwnAs(1),
          inNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{},
          wantNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
               AsPathOptions: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AsPathOptions{
                    Enabled: ygot.Bool(true),
                    AllowOwnAs: ygot.Uint8(1),
               },
          },
     }, {
          desc: "With replace-peer-as",
          ap: New().WithReplacePeerAs(true),
          inNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{},
          wantNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
               AsPathOptions: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AsPathOptions{
                    Enabled: ygot.Bool(true),
                    ReplacePeerAs: ygot.Bool(true),
               },
          },
     }, {
          desc: "With disable-peer-as-filter",
          ap: New().WithDisablePeerAsFilter(true),
          inNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{},
          wantNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
               AsPathOptions: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AsPathOptions{
                    Enabled: ygot.Bool(true),
                    DisablePeerAsFilter: ygot.Bool(true),
               },
          },
     }, {
          desc: "Neighbor contains as-path, no conflicts",
          ap: New().WithReplacePeerAs(true),
          inNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
               AsPathOptions: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AsPathOptions{
                    Enabled: ygot.Bool(true),
               },
          },
          wantNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
               AsPathOptions: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AsPathOptions{
                    Enabled: ygot.Bool(true),
                    ReplacePeerAs: ygot.Bool(true),
               },
          },
     }}

     for _, test := range tests {
          t.Run(test.desc, func(t *testing.T) {
               err := test.ap.AugmentNeighbor(test.inNeighbor)
               if err != nil {
                    t.Fatalf("Error not expected: %v", err)
               }
               if diff := cmp.Diff(test.wantNeighbor, test.inNeighbor); diff != "" {
                    t.Errorf("Did not get expected state, diff(-want,+got):\n%s", diff)
               }
          })
     }
}

// TestAugmentNeighborErrors tests the BGP AP augment to BGP neighbor errors.
func TestAugmentNeighborErrors(t *testing.T) {
     tests := []struct {
          desc string
          ap *AsPathOptions
          inNeighbor *fpoc.NetworkInstance_Protocol_Bgp_Neighbor
          wantErrSubStr string
     }{{
          desc: "Neighbor contains AP with conflicts",
          ap: New(),
          inNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
               AsPathOptions: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AsPathOptions{
                    Enabled: ygot.Bool(false),
               },
          },
          wantErrSubStr: "destination value was set",
     }}

     for _, test := range tests {
          t.Run(test.desc, func(t *testing.T) {
               err := test.ap.AugmentNeighbor(test.inNeighbor)
               if err == nil {
                    t.Fatalf("error expected")
               }
               if !strings.Contains(err.Error(), test.wantErrSubStr) {
                    t.Errorf("error strings are not equal: %v", err)
               }
          })
     }
}

// TestAugmentPeerGroup tests the BGP AP augment to BGP peer group.
func TestAugmentPeerGroup(t *testing.T) {
     tests := []struct {
          desc string
          ap *AsPathOptions
          inPG *fpoc.NetworkInstance_Protocol_Bgp_PeerGroup
          wantPG *fpoc.NetworkInstance_Protocol_Bgp_PeerGroup
     }{{
          desc: "AP enabled with no params",
          ap: New(),
          inPG: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{},
          wantPG: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
               AsPathOptions: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AsPathOptions{
                    Enabled: ygot.Bool(true),
               },
          },
     }, {
          desc: "With allow-own-as",
          ap: New().WithAllowOwnAs(1),
          inPG: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{},
          wantPG: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
               AsPathOptions: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AsPathOptions{
                    Enabled: ygot.Bool(true),
                    AllowOwnAs: ygot.Uint8(1),
               },
          },
     }, {
          desc: "With replace-peer-as",
          ap: New().WithReplacePeerAs(true),
          inPG: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{},
          wantPG: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
               AsPathOptions: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AsPathOptions{
                    Enabled: ygot.Bool(true),
                    ReplacePeerAs: ygot.Bool(true),
               },
          },
     }, {
          desc: "With disable-peer-as-filter",
          ap: New().WithDisablePeerAsFilter(true),
          inPG: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{},
          wantPG: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
               AsPathOptions: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AsPathOptions{
                    Enabled: ygot.Bool(true),
                    DisablePeerAsFilter: ygot.Bool(true),
               },
          },
     }, {
          desc: "Neighbor contains as-path, no conflicts",
          ap: New().WithReplacePeerAs(true),
          inPG: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
               AsPathOptions: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AsPathOptions{
                    Enabled: ygot.Bool(true),
               },
          },
          wantPG: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
               AsPathOptions: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AsPathOptions{
                    Enabled: ygot.Bool(true),
                    ReplacePeerAs: ygot.Bool(true),
               },
          },
     }}
}

// TestAugmentPeerGroupErrors tests the BGP AP augment to BGP peer group errors.
func TestAugmentPeerGroupErrors(t *testing.T) {
     tests := []struct {
          desc string
          ap *AsPathOptions
          inPG *fpoc.NetworkInstance_Protocol_Bgp_PeerGroup
          wantErrSubStr string
     }{{
          desc: "PeerGroup contains AP with conflicts",
          ap: New()
          inPG: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
               AsPathOptions: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AsPathOptions{
                    Enabled: ygot.Bool(false),
               },
          },
          wantErrSubStr: "destination value was set",
     }}

     for _, test := range tests {
          t.Run(test.desc, func(t *testing.T) {
               err := test.ap.AugmentPeerGroup(test.inPG)
               if err == nil {
                    t.Fatalf("error expected")
               }
               if !strings.Contains(err.Error(), test.wantErrSubStr) {
                    t.Errorf("error strings are not equal; got %v", err)
               }
          })
     }
}