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
package multihop

import (
     "github.com/openconfig/featureprofiles/yang/fpoc"
     "github.com/openconfig/ygot/ygot"
)

// Multihop struct to store OC attributes.
type Multihop struct {
     noc fpoc.NetworkInstance_Protocol_Bgp_Neighbor_EbgpMultihop
     poc fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_EbgpMultihop
}

// New returns a new Multihop object.
func New() *Multihop {
     return &Multihop{
          noc: fpoc.NetworkInstance_Protocol_Bgp_Neighbor_EbgpMultihop{
               Enabled: ygot.Bool(true)
          },
          poc: fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_EbgpMultihop{
               Enabled: ygot.Bool(true)
          },
     }
}

// WithMultihopTtl sets the multihopttl for Multihop feature.
func (mh *Multihop) WithMultihopTtl(val uint8) *Multihop {
     mh.noc.MultihopTtl = ygot.Uint8(val)
     mh.poc.MultihopTtl = ygot.Uint8(val)
     return mh
}

// AugmentNeighbor implements the bgp.NeighborFeature interface.
// This method augments the BGP neighbor with multihop feature.
// Use with n.WithFeature(mh) instead of calling this feature directly.
func (mh *Multihop) AugmentNeighbor(n *fpoc.NetworkInstance_Protocol_Bgp_Neighbor) error {
     if err := mh.noc.Validate(); err != nil {
          return err
     }
     if n.EbgpMultihop == nil {
          n.EbgpMultihop = &mh.noc
          return nil
     }
     return ygot.MergeStructInto(n.EbgpMultihop, &mh.noc)
}

// AugmentPeerGroup implements the bgp.PeerGroupFeature interface.
// This method augments the BGP neighbor with multihop feature.
// Use with pg.WithFeature(mh) instead of calling this feature directly.
func (mh *Multihop) AugmentPeerGroup(pg *fpoc.NetworkInstance_Protocol_Bgp_PeerGroup) error {
     if err := mh.poc.Validate(); err != nil {
          return err
     }
     if pg.EbgpMultihop == nil {
          pg.EbgpMultihop = &mh.poc
          return nil
     }
     return ygot.MergeStructInto(pg.EbgpMultihop, &mh.poc)
}