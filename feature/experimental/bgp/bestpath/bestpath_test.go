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

package bestpath

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
          bp *BestPath
          inGlobal *fpoc.NetworkInstance_Protocol_Bgp_Global
          wantGlobal *fpoc.NetworkInstance_Protocol_Bgp_Global
     }{{
          desc: "BP created with no params",
          bp: New(),
          inGlobal: *fpoc.NetworkInstance_Protocol_Bgp_Global{},
          wantGlobal: *fpoc.NetworkInstance_Protocol_Bgp_Global{
               RouteSelectionOptions: *fpoc.NetworkInstance_Protocol_Bgp_Global_RouteSelectionOptions{},
          }
     }, {
          desc: "With always-compare-med",
          bp: New().WithAlwaysCompareMed(true),
          inGlobal: *fpoc.NetworkInstance_Protocol_Bgp_Global{},
          wantGlobal: *fpoc.NetworkInstance_Protocol_Bgp_Global{
               RouteSelectionOptions: *fpoc.NetworkInstance_Protocol_Bgp_Global_RouteSelectionOptions{
                    AlwaysCompareMed: ygot.Bool(true),
               },
          },
     }, {
          desc: "With ignore-as-path-length",
          bp: New().WithIgnoreAsPathLength(true),
          inGlobal: *fpoc.NetworkInstance_Protocol_Bgp_Global{},
          wantGlobal: *fpoc.NetworkInstance_Protocol_Bgp_Global{
               RouteSelectionOptions: *fpoc.NetworkInstance_Protocol_Bgp_Global_RouteSelectionOptions{
                    IgnoreAsPathLength: ygot.Bool(true),
               },
          },
     }, {
          desc: "With external-compare-router-id",
          bp: New().WithCompareRid(true),
          inGlobal: *fpoc.NetworkInstance_Protocol_Bgp_Global{},
          wantGlobal: *fpoc.NetworkInstance_Protocol_Bgp_Global{
               RouteSelectionOptions: *fpoc.NetworkInstance_Protocol_Bgp_Global_RouteSelectionOptions{
                    ExternalCompareRouterId: ygot.Bool(true),
               },
          },
     }, {
          desc: "With advertise-inactive-routes",
          bp: New().WithAdvertiseInactiveRoutes(true),
          inGlobal: *fpoc.NetworkInstance_Protocol_Bgp_Global{},
          wantGlobal: *fpoc.NetworkInstance_Protocol_Bgp_Global{
               RouteSelectionOptions: *fpoc.NetworkInstance_Protocol_Bgp_Global_RouteSelectionOptions{
                    AdvertiseInactiveRoutes: ygot.Bool(true),
               },
          },
     }, {
          desc: "Global contains best path, no conflicts",
          bp: New().WithAlwaysCompareMed(true),
          inGlobal: *fpoc.NetworkInstance_Protocol_Bgp_Global{
               RouteSelectionOptions: *fpoc.NetworkInstance_Protocol_Bgp_Global_RouteSelectionOptions{
                    AdvertiseInactiveRoutes: ygot.Bool(true),
               },
          },
          wantGlobal: *fpoc.NetworkInstance_Protocol_Bgp_Global{
               RouteSelectionOptions: *fpoc.NetworkInstance_Protocol_Bgp_Global_RouteSelectionOptions{
                    AlwaysCompareMed: ygot.Bool(true),
                    AdvertiseInactiveRoutes: ygot.Bool(true),
               },
          },
     }}

     for _, test := range tests {
          t.Run(test.desc, func(t *testing.T) {
               err := test.bp.AugmentGlobal(test.inGlobal)
               if err != nil {
                    t.Fatalf("error not expected: %v", err)
               }
               if diff := cmp.Diff(test.wantGlobal, test.inGlobal); diff != "" {
                    t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
               }
          })
     }
}