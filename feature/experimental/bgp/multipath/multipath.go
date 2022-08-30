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
package multipath

import (
     "github.com/openconfig/featureprofiles/yang/fpoc"
     "github.com/openconfig/ygot/ygot"
)

// Multipath struct to store OC attributes.
type Multipath struct {
     goc fpoc.NetworkInstance_Protocol_Bgp_Global_UseMultiplePaths
     noc fpoc.NetworkInstance_Protocol_Bgp_Neighbor_UseMultiplePaths
     poc fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_UseMultiplePaths
}

// New returns a new Multipath object.
func New() *Multipath {
     return &Multipath{
          goc: fpoc.NetworkInstance_Protocol_Bgp_Global_UseMultiplePaths{
               Enabled: ygot.Bool(true),
          },
          noc: fpoc.NetworkInstance_Protocol_Bgp_Neighbor_UseMultiplePaths{
               Enabled: ygot.Bool(true),
          },
          poc: fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_UseMultiplePaths{
               Enabled: ygot.Bool(true),
          },
     }
}


// WithEbgpAllowMultipleAs sets the ebgp allow-multiple-as for Multipath feature.
func (mp *Multipath) WithEbgpAllowMultipleAs(val bool) *Multipath {
     // Global
     if mp.goc.Ebgp == nil {
          mp.goc.Ebgp = &fpoc.NetworkInstance_Protocol_Bgp_Global_UseMultiplePaths_Ebgp{
               AllowMultipleAs: ygot.Bool(val),
          }
     } else {
          mp.goc.Ebgp.AllowMultipleAs = ygot.Bool(val)
     }

     // Neighbor
     if mp.noc.Ebgp == nil {
          mp.noc.Ebgp = &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_UseMultiplePaths_Ebgp{
               AllowMultipleAs: ygot.Bool(val),
          }
     } else {
          mp.noc.Ebgp.AllowMultipleAs = ygot.Bool(val)
     }

     // Peer Group
     if mp.poc.Ebgp == nil {
          mp.poc.Ebgp = &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_UseMultiplePaths_Ebgp{
               AllowMultipleAs: ygot.Bool(val),
          }
     } else {
          mp.poc.Ebgp.AllowMultipleAs = ygot.Bool(val)
     }

     return mp
}

// WithEbgpMaximumPaths sets the ebgp maximum-paths for Multipath feature.
func (mp *Multipath) WithEbgpMaximumPaths(val uint32) *Multipath {
     // Global
     if mp.goc.Ebgp == nil {
          mp.goc.Ebgp = &fpoc.NetworkInstance_Protocol_Bgp_Global_UseMultiplePaths_Ebgp{
               MaximumPaths: ygot.Uint32(val),
          }
     } else {
          mp.goc.Ebgp.MaximumPaths = ygot.Uint32(val)
     }

     // Neighbor
     if mp.noc.Ebgp == nil {
          mp.noc.Ebgp = &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_UseMultiplePaths_Ebgp{
               MaximumPaths: ygot.Uint32(val),
          }
     } else {
          mp.noc.Ebgp.MaximumPaths = ygot.Uint32(val)
     }

     // Peer Group
     if mp.poc.Ebgp == nil {
          mp.poc.Ebgp = &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_UseMultiplePaths_Ebgp{
               MaximumPaths: ygot.Uint32(val),
          }
     } else {
          mp.poc.Ebgp.MaximumPaths = ygot.Uint32(val)
     }

     return mp
}

// WithIbgpMaximumPaths sets the ibgp maximum-paths for Multipath feature.
func (mp *Multipath) WithIbgpMaximumPaths(val uint32) *Multipath {
     // Global
     if mp.goc.Ibgp == nil {
          mp.goc.Ibgp = &fpoc.NetworkInstance_Protocol_Bgp_Global_UseMultiplePaths_Ibgp{
               MaximumPaths: ygot.Uint32(val),
          }
     } else {
          mp.goc.Ibgp.MaximumPaths = ygot.Uint32(val)
     }

     // Neighbor
     if mp.noc.Ibgp == nil {
          mp.noc.Ibgp = &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_UseMultiplePaths_Ibgp{
               MaximumPaths: ygot.Uint32(val),
          }
     } else {
          mp.noc.Ibgp.MaximumPaths = ygot.Uint32(val)
     }

     // Peer Group
     if mp.poc.Ibgp == nil {
          mp.poc.Ibgp = &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_UseMultiplePaths_Ibgp{
               MaximumPaths: ygot.Uint32(val),
          }
     } else {
          mp.poc.Ibgp.MaximumPaths = ygot.Uint32(val)
     }

     return mp
}

// AugmentNeighbor implements the bgp.NeighborFeature interface.
// This method augments the BGP neighbor with multihop feature.
// Use with n.WithFeature(mh) instead of calling this feature directly.
func (mp *Multipath) AugmentNeighbor(n *fpoc.NetworkInstance_Protocol_Bgp_Neighbor) error {
     if err := mp.noc.Validate(); err != nil {
          return err
     }
     if n.UseMultiplePaths == nil {
          n.UseMultiplePaths = &mp.noc
          return nil
     }
     return ygot.MergeStructInto(n.UseMultiplePaths, &mp.noc)
}

// AugmentPeerGroup implements the bgp.PeerGroupFeature interface.
// This method augments the BGP neighbor with multihop feature.
// Use with pg.WithFeature(mh) instead of calling this feature directly.
func (mp *Multipath) AugmentPeerGroup(pg *fpoc.NetworkInstance_Protocol_Bgp_PeerGroup) error {
     if err := mp.poc.Validate(); err != nil {
          return err
     }
     if pg.UseMultiplePaths == nil {
          pg.UseMultiplePaths = &mp.poc
          return nil
     }
     return ygot.MergeStructInto(pg.UseMultiplePaths, &mp.poc)
}