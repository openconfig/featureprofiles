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

package routereflector

import (
     "strings"
     "testing"

     "github.com/google/go-cmp/cmp"
     "github.com/openconfig/featureprofiles/yang/fpoc"
     "github.com/openconfig/ygot/ygot"
)

// TestAugmentNeighbor tests the BGP PL augment to BGP neighbor.
func TestAugmentNeighbor(t *testing.T) {
     tests := []struct {
          desc string
          rr *RouteReflector
          inNeighbor *fpoc.NetworkInstance_Protocol_Bgp_Neighbor
          wantNeighbor *fpoc.NetworkInstance_Protocol_Bgp_Neighbor
     }{{
          desc: "RR enabled with no params",
          rr: New(),
          inNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{},
          wantNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
               RouteReflector: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_RouteReflector{
                    RouteReflectorClient: ygot.Bool(true),
               },
          },
     }, {
          desc: "With string route-reflector-cluster-id",
          rr: New().WithStringRouteReflectorClusterId("abc"),
          inNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{},
          wantNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
               RouteReflector: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_RouteReflector{
                    RouteReflectorClient: ygot.Bool(true),
                    RouteReflectorClusterId: ygot.UnionString("abc"),
               },
          },
     }, {
          desc: "With uint32 route-reflector-cluster-id",
          rr: New().WithIntRouteReflectorClusterId(5),
          inNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{},
          wantNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
               RouteReflector: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_RouteReflector{
                    RouteReflectorClient: ygot.Bool(true),
                    RouteReflectorClusterId: ygot.UnionUint32(5),
               },
          },
     }, {
          desc: "Neighbor contains RR, no conflicts",
          rr: New().WithIntRouteReflectorClusterId(5),
          inNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
               RouteReflector: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_RouteReflector{
                    RouteReflectorClient: ygot.Bool(true),
               },
          },
          wantNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
               RouteReflector: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_RouteReflector{
                    RouteReflectorClient: ygot.Bool(true),
                    RouteReflectorClusterId: ygot.UnionUint32(5),
               },
          },
     }}

     for _, test := range tests {
          t.Run(test.desc, func(t *testing.T) {
               err := test.rr.AugmentNeighbor(test.inNeighbor)
               if err != nil {
                    t.Fatalf("Error not expected: %v", err)
               }
               if diff := cmp.Diff(test.wantNeighbor, test.inNeighbor); diff != "" {
                    t.Errorf("Did not get expected state, diff(-want,+got):\n%s", diff)
               }
          })
     }
}

// TestAugmentNeighborErrors tests the BGP PL augment to BGP neighbor errors.
func TestAugmentNeighborErrors(t *testing.T) {
     tests := []struct {
          desc string
          rr *RouteReflector
          inNeighbor *fpoc.NetworkInstance_Protocol_Bgp_Neighbor
          wantErrSubStr string
     }{{
          desc: "Neighbor contains RR with conflicts",
          rr: New()
          inNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
               RouteReflector: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_RouteReflector{
                    RouteReflectorClient: ygot.Bool(false),
               },
          },
          wantErrSubStr: "destination value was set",
     }}

     for _, test := range tests {
          t.Run(test.desc, func(t *testing.T) {
               err := test.rr.AugmentNeighbor(test.inNeighbor)
               if err == nil {
                    t.Fatalf("error expected")
               }
               if !strings.Contains(err.Error(), test.wantErrSubStr) {
                    t.Errorf("error strings are not equal: %v", err)
               }
          })
     }
}

// TestAugmentPeerGroup tests the BGP PL augment to BGP peer group.
func TestAugmentPeerGroup(t *testing.T) {
     tests := []struct {
          desc string
          rr *RouteReflector
          inPG *fpoc.NetworkInstance_Protocol_Bgp_PeerGroup
          wantPG *fpoc.NetworkInstance_Protocol_Bgp_PeerGroup
     }{{
          desc: "RR enabled with no params",
          rr: New(),
          inPG: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{},
          wantPG: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
               RouteReflector: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_RouteReflector{
                    RouteReflectorClient: ygot.Bool(true),
               },
          },
     }, {
          desc: "With string route-reflector-cluster-id",
          rr: New().WithStringRouteReflectorClusterId("abc"),
          inPG: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{},
          wantPG: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
               RouteReflector: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_RouteReflector{
                    RouteReflectorClient: ygot.Bool(true),
                    RouteReflectorClusterId: ygot.UnionString("abc"),
               },
          },
     }, {
          desc: "With int route-reflector-cluster-id",
          rr: New().WithIntRouteReflectorClusterId(5),
          inPG: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{},
          wantPG: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
               RouteReflector: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_RouteReflector{
                    RouteReflectorClient: ygot.Bool(true),
                    RouteReflectorClusterId: ygot.UnionUint32(5),
               },
          },
     }, {
          desc: "Peer group contains route reflector, no conflicts",
          rr: New().WithIntRouteReflectorClusterId(5),
          inPG: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
               RouteReflector: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_RouteReflector{
                    RouteReflectorClient: ygot.Bool(true),
               },
          },
          wantPG: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
               RouteReflector: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_RouteReflector{
                    RouteReflectorClient: ygot.Bool(true),
                    RouteReflectorClusterId: ygot.UnionUint32(5),
               },
          },
     }}

     for _, test := range tests {
          t.Run(test.desc, func(t *testing.T) {
               err := test.rr.AugmentPeerGroup(test.inPG)
               if err != nil {
                    t.Fatalf("Error not expected: %v", err)
               }
               if diff := cmp.Diff(test.wantPG, test.inPG); diff != "" {
                    t.Errorf("Did not get expected state, diff(-want,+got):\n%s", diff)
               }
          })
     }
}

// TestAugmentPeerGroupErrors tests the BGP PL augment to BGP peer group errors.
func TestAugmentPeerGroupErrors(t *testing.T) {
     tests := []struct {
          desc string
          rr *RouteReflector
          inPG *fpoc.NetworkInstance_Protocol_Bgp_PeerGroup
          wantErrSubStr string
     }{{
          desc: "Peer group contains RR with conflicts",
          rr: New(),
          inNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
               RouteReflector: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_RouteReflector{
                    RouteReflectorClient: ygot.Bool(false),
               },
          },
          wantErrSubStr: "destination value was set",
     }}

     for _, test := range tests {
          t.Run(test.desc, func(t *testing.T) {
               err := test.rr.AugmentPeerGroup(test.inPG)
               if err == nil {
                    t.Fatalf("error expected")
               }
               if !strings.Contains(err.Error(), test.wantErrSubStr) {
                    t.Errorf("error strings are not equal; got %v", err)
               }
          })
     }
}