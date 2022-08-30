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

// Package aspath implements the Config Library for BGP as path
// feature profile.
package prefixlimit

import (
     "github.com/openconfig/featureprofiles/yang/fpoc"
     "github.com/openconfig/ygot/ygot"
)

// PrefixLimit struct to store OC attributes.
type PrefixLimit struct {
     noc fpoc.NetworkInstance_Protocol_Bgp_Neighbor
     poc fpoc.NetworkInstance_Protocol_Bgp_PeerGroup
}

// New returns a new PrefixLimit object.
func New() *PrefixLimit {
     return &PrefixLimit{
          noc: fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
               Enabled: ygot.Bool(true),
          },
          poc: fpoc.NetworkInstance_Protocol_Bgp_PeerGroup{
               Enabled: ygot.Bool(true),
          },
     }
}

// WithMaxPrefixes sets the max-prefixes for prefix limit feature.
func (pl *PrefixLimit) WithMaxPrefixes(val uint32) *PrefixLimit {
     // Neighbor
     if pl.noc.AfiSafi == nil {
          pl.noc.AfiSafi = &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi{
               Ipv4Unicast: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_Ipv4Unicast{
                    PrefixLimit: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_Ipv4Unicast_PrefixLimit{
                         MaxPrefixes: ygot.Uint32(val),
                    },
               },
          }
     } else if pl.noc.AfiSafi.Ipv4Unicast == nil {
          pl.noc.AfiSafi.Ipv4Unicast = &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_Ipv4Unicast{
               PrefixLimit: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_Ipv4Unicast_PrefixLimit{
                    MaxPrefixes: ygot.Uint32(val),
               },
          }
     } else if pl.noc.AfiSafi.Ipv4Unicast.PrefixLimit == nil {
          pl.noc.AfiSafi.Ipv4Unicast.PrefixLimit = &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_Ipv4Unicast_PrefixLimit{
               MaxPrefixes: ygot.Uint32(val),
          }
     } else {
          pl.noc.AfiSafi.Ipv4Unicast.PrefixLimit.MaxPrefixes = ygot.Uint32(val)
     }

     // Peer Group
     if pl.poc.AfiSafi == nil {
          pl.poc.AfiSafi = &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AfiSafi{
               Ipv4Unicast: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AfiSafi_Ipv4Unicast{
                    PrefixLimit: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AfiSafi_Ipv4Unicast_PrefixLimit{
                         MaxPrefixes: ygot.Uint32(val),
                    },
               },
          }
     } else if pl.poc.AfiSafi.Ipv4Unicast == nil {
          pl.poc.AfiSafi.Ipv4Unicast = &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AfiSafi_Ipv4Unicast{
               PrefixLimit: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AfiSafi_Ipv4Unicast_PrefixLimit{
                    MaxPrefixes: ygot.Uint32(val),
               },
          }
     } else if pl.poc.AfiSafi.Ipv4Unicast.PrefixLimit == nil {
          pl.poc.AfiSafi.Ipv4Unicast.PrefixLimit = &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AfiSafi_Ipv4Unicast_PrefixLimit{
               MaxPrefixes: ygot.Uint32(val),
          }
     } else {
          pl.poc.AfiSafi.Ipv4Unicast.PrefixLimit.MaxPrefixes = ygot.Uint32(val)
     }

     return pl
}

// WithWarningThresholdPct sets the warning-threshold-pct for prefix limit feature.
func (pl *PrefixLimit) WithWarningThresholdPct(val uint8) *PrefixLimit {
     if pl.noc.AfiSafi == nil {
          pl.noc.AfiSafi = &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi{
               Ipv4Unicast: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_Ipv4Unicast{
                    PrefixLimit: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_Ipv4Unicast_PrefixLimit{
                         WarningThresholdPct: ygot.Uint8(val),
                    },
               },
          }
     } else if pl.noc.AfiSafi.Ipv4Unicast == nil {
          pl.noc.AfiSafi.Ipv4Unicast = &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_Ipv4Unicast{
               PrefixLimit: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_Ipv4Unicast_PrefixLimit{
                    WarningThresholdPct: ygot.Uint8(val),
               },
          }
     } else if pl.noc.AfiSafi.Ipv4Unicast.PrefixLimit == nil {
          pl.noc.AfiSafi.Ipv4Unicast.PrefixLimit = &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_Ipv4Unicast_PrefixLimit{
               WarningThresholdPct: ygot.Uint8(val),
          }
     } else {
          pl.noc.AfiSafi.Ipv4Unicast.PrefixLimit.WarningThresholdPct = ygot.Uint8(val)
     }

     return pl
}

// WithRestartTime sets the restart-time for prefix limit feature.
func (pl *PrefixLimit) WithRestartTime(val uint16) *PrefixLimit {
     // Neighbor
     if pl.noc.Timers == nil {
          pl.noc.Timers = &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_Timers{
               RestartTime: ygot.Uint16(val),
          }
     } else {
          pl.noc.Timers.RestartTime = ygot.Uint16(val)
     }

     // Peer Group
     if pl.poc.Timers == nil {
          pl.poc.Timers = &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_Timers{
               RestartTime: ygot.Uint16(val),
          }
     } else {
          pl.poc.Timers.RestartTime = ygot.Uint16(val)
     }
}

// AugmentNeighbor implements the bgp.NeighborFeature interface.
// This method augments the BGP neighbor with prefix limit feature.
// Use with n.WithFeature(pl) instead of calling this feature directly.
func (pl *PrefixLimit) AugmentNeighbor(n *fpoc.NetworkInstance_Protocol_Bgp_Neighbor) error {
     if err := pl.noc.Validate(); err != nil {
          return err
     }
     return ygot.MergeStructInto(n, &pl.noc)
}

// AugmentPeerGroup implements the bgp.PeerGroupFeature interface.
// This method augments the BGP peer group with prefix limit feature.
// Use with pg.WithFeature(pl) instead of calling this feature directly.
func (pl *PrefixLimit) AugmentPeerGroup(pg *fpoc.NetworkInstance_Protocol_Bgp_PeerGroup) error {
     if err := pl.poc.Validate(); err != nil {
          return err
     }
     return ygot.MergeStructInto(pg, &pl.poc)
}