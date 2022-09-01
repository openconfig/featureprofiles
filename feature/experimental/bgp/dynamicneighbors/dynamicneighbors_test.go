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

package dynamicneighbors

import (
     "strings"
     "testing"

     "github.com/google/go-cmp/cmp"
     "github.com/openconfig/featureprofiles/yang/fpoc"
     "github.com/openconfig/ygot/ygot"
)

// TestAugmentGlobal tests the BGP BP augment to BGP global.
func TestAugmentGlobal(t *testing.T) {
     tests := []struct{
          desc string
          dn *DynamicNeighbors
          inGlobal *fpoc.NetworkInstance_Protocol_Bgp_Global
          wantGlobal *fpoc.NetworkInstance_Protocol_Bgp_Global
     }{{
          desc: "DN created with no params",
          dn: New(),
          inGlobal: *fpoc.NetworkInstance_Protocol_Bgp_Global{},
          wantGlobal: *fpoc.NetworkInstance_Protocol_Bgp_Global{
               DynamicNeighborPrefix: &fpoc.NetworkInstance_Protocol_Bgp_Global_DynamicNeighborPrefix{},
          },
     }, {
          desc: "With prefix",
          dn: New().WithPrefix("prefix"),
          inGlobal: *fpoc.NetworkInstance_Protocol_Bgp_Global{},
          wantGlobal: *fpoc.NetworkInstance_Protocol_Bgp_Global{
               DynamicNeighborPrefix: &fpoc.NetworkInstance_Protocol_Bgp_Global_DynamicNeighborPrefix{
                    Prefix: ygot.String("prefix"),
               },
          },
     }, {
          desc: "With peer-group",
          dn: New().WithPeerGroup("peer-group"),
          inGlobal: *fpoc.NetworkInstance_Protocol_Bgp_Global{},
          wantGlobal: *fpoc.NetworkInstance_Protocol_Bgp_Global{
               DynamicNeighborPrefix: &fpoc.NetworkInstance_Protocol_Bgp_Global_DynamicNeighborPrefix{
                    PeerGroup: ygot.String("peer-group"),
               },
          },
     }, {
          desc: "Global contains dynamic neighbors, no conflicts",
          dn: New().WithPrefix("prefix"),
          inGlobal: *fpoc.NetworkInstance_Protocol_Bgp_Global{
               DynamicNeighborPrefix: &fpoc.NetworkInstance_Protocol_Bgp_Global_DynamicNeighborPrefix{
                    PeerGroup: ygot.String("peer-group"),
               },
          },
          wantGlobal: *fpoc.NetworkInstance_Protocol_Bgp_Global{
               DynamicNeighborPrefix: &fpoc.NetworkInstance_Protocol_Bgp_Global_DynamicNeighborPrefix{
                    Prefix: ygot.String("prefix"),
                    PeerGroup: ygot.String("peer-group"),
               },
          },
     }}

     for _, test := range tests {
          t.Run(test.desc, func(t *testing.T) {
               err := test.dn.AugmentGlobal(test.inGlobal)
               if err != nil {
                    t.Fatalf("error not expected: %v", err)
               }
               if diff := cmp.Diff(test.wantGlobal, test.inGlobal); diff != "" {
                    t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
               }
          })
     }
}