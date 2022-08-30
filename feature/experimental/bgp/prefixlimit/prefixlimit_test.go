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

package prefixlimit

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
          pl *PrefixLimit
          inNeighbor *fpoc.NetworkInstance_Protocol_Bgp_Neighbor
          wantNeighbor *fpoc.NetworkInstance_Protocol_Bgp_Neighbor
     }{{
          desc: "PL enabled with no params",
          pl: New(),
          inNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{},
          wantNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
               Enabled: ygot.Bool(true),
          },
     }, {
          desc: "With max-prefixes",
          pl: New().WithMaxPrefixes(5),
          inNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{},
          wantNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
               AfiSafi: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi{
                    Ipv4Unicast: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_Ipv4Unicast{
                         PrefixLimit: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_Ipv4Unicast_PrefixLimit{
                              MaxPrefixes: ygot.Uint32(5),
                         },
                    },
               },
          },
     }, {
          desc: "With warning-threshold-pct",
          pl: New().WithWarningThresholdPct(5),
          inNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{},
          wantNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
               AfiSafi: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi{
                    Ipv4Unicast: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_Ipv4Unicast{
                         PrefixLimit: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_Ipv4Unicast_PrefixLimit{
                              WarningThresholdPct: ygot.Uint8(5),
                         },
                    },
               },
          },
     }, {
          desc: "With restart-time",
          pl: New().WithRestartTime(5),
          inNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{},
          wantNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
               Timers: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_Timers{
                    RestartTime: ygot.Uint16(5),
               },
          },
     }, {
          desc: "Neighbor contains prefix limit, no conflicts",
          pl: New().WithMaxPrefixes(5),
          inNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
               AfiSafi: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi{
                    Ipv4Unicast: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_Ipv4Unicast{
                         PrefixLimit: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_Ipv4Unicast_PrefixLimit{
                              WarningThresholdPct: ygot.Uint8(5),
                         },
                    },
               },
          },
          wantNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
               AfiSafi: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi{
                    Ipv4Unicast: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_Ipv4Unicast{
                         PrefixLimit: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_Ipv4Unicast_PrefixLimit{
                              MaxPrefixes: ygot.Uint32(5),
                              WarningThresholdPct: ygot.Uint8(5),
                         },
                    },
               },
          },
     }}

     for _, test := range tests {
          t.Run(test.desc, func(t *testing.T) {
               err := test.pl.AugmentNeighbor(test.inNeighbor)
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
          pl *PrefixLimit
          inNeighbor *fpoc.NetworkInstance_Protocol_Bgp_Neighbor
          wantErrSubStr string
     }{{
          desc: "Neighbor contains PL with conflicts",
          pl: New().WithMaxPrefixes(5),
          inNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
               AfiSafi: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi{
                    Ipv4Unicast: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_Ipv4Unicast{
                         PrefixLimit: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_Ipv4Unicast_PrefixLimit{
                              MaxPrefixes: ygot.Uint32(2),
                         },
                    },
               },
          },
          wantErrSubStr: "destination value was set",
     }}

     for _, test := range tests {
          t.Run(test.desc, func(t *testing.T) {
               err := test.pl.AugmentNeighbor(test.inNeighbor)
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
          pl *PrefixLimit
          inPG *fpoc.NetworkInstance_Protocol_Bgp_PeerGroup
          wantPG *fpoc.NetworkInstance_Protocol_Bgp_PeerGroup
     }{{
          desc: "PL enabled with no params",
          pl: New(),
          inPG: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{},
          wantPG: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
               Enabled: ygot.Bool(true),
          },
     }, {
          desc: "With max-prefixes",
          pl: New().WithMaxPrefixes(5),
          inPG: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{},
          wantPG: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
               AfiSafi: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AfiSafi{
                    Ipv4Unicast: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AfiSafi_Ipv4Unicast{
                         PrefixLimit: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AfiSafi_Ipv4Unicast_PrefixLimit{
                              MaxPrefixes: ygot.Uint32(5),
                         },
                    },
               },
          },
     }, {
          desc: "With restart-time",
          pl: New().WithRestartTime(5),
          inPG: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{},
          wantPG: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
               Timers: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_Timers{
                    RestartTime: ygot.Uint16(5),
               },
          },
     }, {
          desc: "Peer group contains prefix limit, no conflicts",
          pl: New().WithMaxPrefixes(5),
          inPG: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
               AfiSafi: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AfiSafi{
                    Ipv4Unicast: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AfiSafi_Ipv4Unicast{
                         PrefixLimit: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AfiSafi_Ipv4Unicast_PrefixLimit{
                              WarningThresholdPct: ygot.Uint8(5),
                         },
                    },
               },
          },
          wantPG: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
               AfiSafi: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AfiSafi{
                    Ipv4Unicast: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AfiSafi_Ipv4Unicast{
                         PrefixLimit: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AfiSafi_Ipv4Unicast_PrefixLimit{
                              MaxPrefixes: ygot.Uint32(5),
                              WarningThresholdPct: ygot.Uint8(5),
                         },
                    },
               },
          },
     }}

     for _, test := range tests {
          t.Run(test.desc, func(t *testing.T) {
               err := test.pl.AugmentPeerGroup(test.inPG)
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
          pl *PrefixLimit
          inPG *fpoc.NetworkInstance_Protocol_Bgp_PeerGroup
          wantErrSubStr string
     }{{
          desc: "Peer group contains PL with conflicts",
          pl: New().WithMaxPrefixes(5),
          inNeighbor: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
               AfiSafi: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AfiSafi{
                    Ipv4Unicast: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AfiSafi_Ipv4Unicast{
                         PrefixLimit: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AfiSafi_Ipv4Unicast_PrefixLimit{
                              MaxPrefixes: ygot.Uint32(2),
                         },
                    },
               },
          },
          wantErrSubStr: "destination value was set",
     }}

     for _, test := range tests {
          t.Run(test.desc, func(t *testing.T) {
               err := test.pl.AugmentPeerGroup(test.inPG)
               if err == nil {
                    t.Fatalf("error expected")
               }
               if !strings.Contains(err.Error(), test.wantErrSubStr) {
                    t.Errorf("error strings are not equal; got %v", err)
               }
          })
     }
}