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

package policybase

import (
     "strings"
     "testing"

     "github.com/google/go-cmp/cmp"
     "github.com/openconfig/featureprofiles/yang/fpoc"
     "github.com/openconfig/ygot/ygot"
)

// TestAugmentNeighbor tests the BGP PB augment to BGP neighbor.
func TestAugmentNeighbor(t *testing.T) {
     tests := []struct {
          desc string
          p *PolicyBase
          inNeighbor *fpoc.NetworkInstance_Protocol_Bgp_Neighbor
          wantNeighbor *fpoc.NetworkInstance_Protocol_Bgp_Neighbor
     }{{
          desc: "PB enabled with no params",
          p: New(),
          inNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{},
          wantNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
               AfiSafi: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi{
                    Enabled: ygot.Bool(true),
               },
          },
     }, {
          desc: "With name",
          p: New().WithName(fpoc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST),
          inNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{},
          wantNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
               AfiSafi: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi{
                    Enabled: ygot.Bool(true),
                    AfiSafiName: fpoc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST,
               },
          },
     }, {
          desc: "With default-import-policy",
          p: New().WithDefaultImportPolicy(fpoc.E_RoutingPolicy_DefaultPolicyType),
          inNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{},
          wantNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
               AfiSafi: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi{
                    Enabled: ygot.Bool(true),
                    ApplyPolicy: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_ApplyPolicy{
                         DefaultImportPolicy: fpoc.E_RoutingPolicy_DefaultPolicyType,
                    },
               },
          },
     }, {
          desc: "With default-export-policy",
          p: New().WithDefaultExportPolicy(fpoc.E_RoutingPolicy_DefaultPolicyType),
          inNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{},
          wantNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
               AfiSafi: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi{
                    Enabled: ygot.Bool(true),
                    ApplyPolicy: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_ApplyPolicy{
                         DefaultExportPolicy: fpoc.E_RoutingPolicy_DefaultPolicyType,
                    },
               },
          },
     }, {
          desc: "With export-policy",
          p: New().WithExportPolicy("export-policy"),
          inNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{},
          wantNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
               AfiSafi: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi{
                    Enabled: ygot.Bool(true),
                    ApplyPolicy: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_ApplyPolicy{
                         ExportPolicy: ygot.String("export-policy"),
                    },
               },
          },
     }, {
          desc: "With import-policy",
          p: New().WithImportPolicy("import-policy"),
          inNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{},
          wantNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
               AfiSafi: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi{
                    Enabled: ygot.Bool(true),
                    ApplyPolicy: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_ApplyPolicy{
                         ImportPolicy: ygot.String("import-policy"),
                    },
               },
          },
     }, {
          desc: "Neighbor contains PB, no conflicts",
          p: New().WithImportPolicy("import-policy"),
          inNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
               AfiSafi: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi{
                    ApplyPolicy: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_ApplyPolicy{
                         ExportPolicy: ygot.String("export-policy"),
                    },
               },
          },
          wantNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
               AfiSafi: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi{
                    Enabled: ygot.Bool(true),
                    ApplyPolicy: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_ApplyPolicy{
                         ImportPolicy: ygot.String("import-policy"),
                         ExportPolicy: ygot.String("export-policy"),
                    },
               },
          },
     }}

     for _, test := range tests {
          t.Run(test.desc, func(t *testing.T) {
               err := test.p.AugmentNeighbor(test.inNeighbor)
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
          p *PolicyBase
          inNeighbor *fpoc.NetworkInstance_Protocol_Bgp_Neighbor
          wantErrSubStr string
     }{{
          desc: "Neighbor contains PB with conflicts",
          p: New(),
          inNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
               AfiSafi: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi{
                    Enabled: ygot.Bool(false),
               },
          },
          wantErrSubStr: "destination value was set",
     }}

     for _, test := range tests {
          t.Run(test.desc, func(t *testing.T) {
               err := test.p.AugmentNeighbor(test.inNeighbor)
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
          p *PolicyBase
          inNeighbor *fpoc.NetworkInstance_Protocol_Bgp_Neighbor
          wantNeighbor *fpoc.NetworkInstance_Protocol_Bgp_Neighbor
     }{{
          desc: "PB enabled with no params",
          p: New(),
          inNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{},
          wantNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
               AfiSafi: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AfiSafi{
                    Enabled: ygot.Bool(true),
               },
          },
     }, {
          desc: "With name",
          p: New().WithName(fpoc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST),
          inNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{},
          wantNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
               AfiSafi: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AfiSafi{
                    Enabled: ygot.Bool(true),
                    AfiSafiName: fpoc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST,
               },
          },
     }, {
          desc: "With default-import-policy",
          p: New().WithDefaultImportPolicy(fpoc.E_RoutingPolicy_DefaultPolicyType),
          inNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{},
          wantNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
               AfiSafi: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AfiSafi{
                    Enabled: ygot.Bool(true),
                    ApplyPolicy: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AfiSafi_ApplyPolicy{
                         DefaultImportPolicy: fpoc.E_RoutingPolicy_DefaultPolicyType,
                    },
               },
          },
     }, {
          desc: "With default-export-policy",
          p: New().WithDefaultExportPolicy(fpoc.E_RoutingPolicy_DefaultPolicyType),
          inNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{},
          wantNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
               AfiSafi: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AfiSafi{
                    Enabled: ygot.Bool(true),
                    ApplyPolicy: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AfiSafi_ApplyPolicy{
                         DefaultExportPolicy: fpoc.E_RoutingPolicy_DefaultPolicyType,
                    },
               },
          },
     }, {
          desc: "With export-policy",
          p: New().WithExportPolicy("export-policy"),
          inNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{},
          wantNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
               AfiSafi: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AfiSafi{
                    Enabled: ygot.Bool(true),
                    ApplyPolicy: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AfiSafi_ApplyPolicy{
                         ExportPolicy: ygot.String("export-policy"),
                    },
               },
          },
     }, {
          desc: "With import-policy",
          p: New().WithImportPolicy("import-policy"),
          inNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{},
          wantNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
               AfiSafi: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AfiSafi{
                    Enabled: ygot.Bool(true),
                    ApplyPolicy: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AfiSafi_ApplyPolicy{
                         ImportPolicy: ygot.String("import-policy"),
                    },
               },
          },
     }, {
          desc: "Peer group contains PB, no conflicts",
          p: New().WithImportPolicy("import-policy"),
          inNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
               AfiSafi: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AfiSafi{
                    ApplyPolicy: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AfiSafi_ApplyPolicy{
                         ExportPolicy: ygot.String("export-policy"),
                    },
               },
          },
          wantNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
               AfiSafi: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AfiSafi{
                    Enabled: ygot.Bool(true),
                    ApplyPolicy: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AfiSafi_ApplyPolicy{
                         ImportPolicy: ygot.String("import-policy"),
                         ExportPolicy: ygot.String("export-policy"),
                    },
               },
          },
     }}

     for _, test := range tests {
          t.Run(test.desc, func(t *testing.T) {
               err := test.p.AugmentPeerGroup(test.inPG)
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
          p *PolicyBase
          inPG *fpoc.NetworkInstance_Protocol_Bgp_PeerGroup
          wantErrSubStr string
     }{{
          desc: "Peer group contains PB with conflicts",
          p: New(),
          inPG: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
               AfiSafi: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AfiSafi{
                    Enabled: ygot.Bool(false),
               },
          },
          wantErrSubStr: "destination value was set",
     }}

     for _, test := range tests {
          t.Run(test.desc, func(t *testing.T) {
               err := test.p.AugmentPeerGroup(test.inPG)
               if err == nil {
                    t.Fatalf("error expected")
               }
               if !strings.Contains(err.Error(), test.wantErrSubStr) {
                    t.Errorf("error strings are not equal; got %v", err)
               }
          })
     }
}