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

package addpath

import (
     "strings"
     "testing"

     "github.com/google/go-cmp/cmp"
     "github.com/openconfig/featureprofiles/yang/fpoc"
     "github.com/openconfig/ygot/ygot"
)

// TestAugmentGlobal ??

// TestAugmentNeighbor
func TestAugmentNeighbor(t *testing.T) {
     tests := []struct{
          desc string
          ap *AddPath
          inNeighbor *fpoc.NetworkInstance_Protocol_Bgp_Neighbor
          wantNeighbor *fpoc.NetworkInstance_Protocol_Bgp_Neighbor
     }{{
          desc: "AP enabled with no params",
          ap: New(),
          inNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{},
          wantNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
               AfiSafi: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi{
                    AddPaths: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_AddPaths{
                         Enabled: ygot.Bool(true),
                    },
               },
          },
     }, {
          desc: "With receive",
          ap: New().WithReceive(true),
          inNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{},
          wantNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
               AfiSafi: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi{
                    AddPaths: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_AddPaths{
                         Enabled: ygot.Bool(true),
                         Receive: ygot.Bool(true),

                    },
               },
          },
     }, {
          desc: "With send",
          ap: New().WithSend(true),
          inNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{},
          wantNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
               AfiSafi: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi{
                    AddPaths: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_AddPaths{
                         Enabled: ygot.Bool(true),
                         Send: ygot.Bool(true),
                    },
               },
          },
     }, {
          desc: "With send max",
          ap: New().WithSendMax(5),
          inNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{},
          wantNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
               AfiSafi: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi{
                    AddPaths: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_AddPaths{
                         Enabled: ygot.Bool(true),
                         SendMax: ygot.Uint8(5),
                    },
               },
          },
     }, {
          desc: "With Eligible Prefix Policy",
          ap: New().WithEligiblePrefixPolicy("/routing-policy/policy-definitions/policy-definition/name"),
          inNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{},
          wantNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
               AfiSafi: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi{
                    AddPaths: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_AddPaths{
                         Enabled: ygot.Bool(true),
                         EligiblePrefixPolicy: ygot.String("/routing-policy/policy-definitions/policy-definition/name"),
                    },
               },
          },
     }, {
          desc: "Neighbor contains add paths, no conflicts",
          ap: New().WithSend(true),
          inNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
               AfiSafi: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi{
                    AddPaths: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_AddPaths{
                         Enabled: ygot.Bool(true),
                    },
               },
          },
          wantNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
               AfiSafi: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi{
                    AddPaths: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_AddPaths{
                         Enabled: ygot.Bool(true),
                         Send: ygot.Bool(true),
                    },
               },
          },
     }}

     for _, test := range tests {
          t.Run(test.desc, func(t *testing.T) {
               err := test.ap.AugmentNeighbor(test.inNeighbor)
               if err != nil {
                    t.Fatalf("error not expected: %v", err)
               }
               if diff := cmp.Diff(test.wantNeighbor, test.inNeighbor); diff != "" {
                    t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
               }
          })
     }
}

// TestAugmentNeighborErrors
func TestAugmentNeighborErrors(t *testing.T) {
     tests := []struct {
          desc string
          ap *AddPath
          inNeighbor *fpoc.NetworkInstance_Protocol_Bgp_Neighbor
          wantErrSubStr string
     }{{
          desc: "Neighbor contains AP with conflicts",
          ap: New(),
          inNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
               AfiSafi: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi{
                    AddPaths: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_AddPaths{
                         Enabled: ygot.Bool(false),
                    },
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

// TestAugmentPeerGroup
func TestAugmentPeerGroup(t *testing.T) {
     tests := []struct {
          desc string
          ap *AddPath
          inPG *fpoc.NetworkInstance_Protocol_Bgp_PeerGroup
          wantPG *fpoc.NetworkInstance_Protocol_Bgp_PeerGroup
     }{{
          desc: "AP enabled with no params",
          ap: New(),
          inPG: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{},
          wantPG: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
               AfiSafi: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AfiSafi{
                    AddPaths: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AfiSafi_AddPaths{
                         Enabled: ygot.Bool(true),
                    },
               },
          },
     }, {
          desc: "With receive",
          ap: New().WithReceive(true),
          inPG: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{},
          wantPG: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
               AfiSafi: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AfiSafi{
                    AddPaths: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AfiSafi_AddPaths{
                         Enabled: ygot.Bool(true),
                         Receive: ygot.Bool(true),
                    },
               },
          },
     }, {
          desc: "With send",
          ap: New().WithSend(true),
          inPG: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{},
          wantPG: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
               AfiSafi: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AfiSafi{
                    AddPaths: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AfiSafi_AddPaths{
                         Enabled: ygot.Bool(true),
                         Send: ygot.Bool(true),
                    },
               },
          },
     }, {
          desc: "With send max",
          ap: New().WithSendMax(5),
          inPG: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{},
          wantPG: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
               AfiSafi: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AfiSafi{
                    AddPaths: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AfiSafi_AddPaths{
                         Enabled: ygot.Bool(true),
                         SendMax: ygot.Uint8(5),
                    },
               },
          },
     }, {
          desc: "With eligible prefix policy",
          ap: New().WithEligiblePrefixPolicy("routing-policy/policy-definitions/policy-definition/name"),
          inPG: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{},
          wantPG: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
               AfiSafi: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AfiSafi{
                    AddPaths: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AfiSafi_AddPaths{
                         Enabled: ygot.Bool(true),
                         EligiblePrefixPolicy: ygot.String("routing-policy/policy-definitions/policy-definition/name"),
                    },
               },
          },
     }, {
          desc: "Peer group contains add path, no conflicts",
          ap: New().WithSend(true),
          inPG: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
               AfiSafi: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AfiSafi{
                    AddPaths: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AfiSafi_AddPaths{
                         Enabled: ygot.Bool(true),
                    }
               }
          },
          wantPG: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
               AfiSafi: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AfiSafi{
                    AddPaths: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AfiSafi_AddPaths{
                         Enabled: ygot.Bool(true),
                         Send: ygot.Bool(true),
                    },
               },
          },
     }}

     for _, test := range tests {
          t.Run(test.desc, func(t *testing.T) {
               err := test.ap.AugmentPeerGroup(test.inNeighbor)
               if err != nil {
                    t.Fatalf("error not expected: %v", err)
               }
               if diff := cmp.Diff(test.wantNeighbor, test.inNeighbor); diff != "" {
                    t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
               }
          })
     }
}

// TestAugmentPeerGroupErrors
func TestAugmentPeerGroupErrors(t *testing.T) {
     tests := []struct {
          desc string
          ap *AddPath
          inPG *fpoc.NetworkInstance_Protocol_Bgp_PeerGroup
          wantErrSubStr string
     }{{
          desc: "Peer Group contains AP with conflicts",
          ap: New(),
          inPG: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
               AfiSafi: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AfiSafi{
                    AddPaths: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AfiSafi_AddPaths{
                         Enabled: ygot.Bool(false),
                    },
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
                    t.Errorf("error strings are not equal: %v", err)
               }
          })
     }
}