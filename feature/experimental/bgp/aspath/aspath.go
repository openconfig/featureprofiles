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
package aspath

import (
     "github.com/openconfig/featureprofiles/yang/fpoc"
     "github.com/openconfig/ygot/ygot"
)

// AsPathOptions struct to store OC attributes.
type AsPathOptions struct {
     noc fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AsPathOptions
     poc fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AsPathOptions
}

// New returns a new AsPathOptions object.
func New() *AsPathOptions {
     return &AsPathOptions{
          noc: fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AsPathOptions{},
          poc: fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AsPathOptions{},
     }
}

// WithAllowOwnAs sets the with allow own for AsPathOptions feature.
func (apo *AsPathOptions) WithAllowOwnAs(val uint8) *AsPathOptions {
     apo.noc.AllowOwnAs = ygot.Uint8(val)
     apo.poc.AllowOwnAs = ygot.Uint8(val)
     return apo
}

// WithReplacePeerAs sets the replace peer as for AsPathOptions feature.
func (apo *AsPathOptions) WithReplacePeerAs(val bool) *AsPathOptions {
     apo.noc.ReplacePeerAs = ygot.Bool(val)
     apo.poc.ReplacePeerAs = ygot.Bool(val)
     return apo
}

// WithDisablePeerAsFilter sets the disable peer as filter for 
// AsPathOptions feature.
func (apo *AsPathOptions) WithDisablePeerAsFilter(val bool) *AsPathOptions {
     apo.noc.DisablePeerAsFilter = ygot.Bool(val)
     apo.poc.DisablePeerAsFilter = ygot.Bool(val)
     return apo
}

// AugmentNeighbor implements the bgp.NeighborFeature interface.
// This method augments the BGP neighbor with as path options feature.
// Use with n.WithFeature(apo) instead of calling this feature directly.
func (apo *AsPathOptions) AugmentNeighbor(n *fpoc.NetworkInstance_Protocol_Bgp_Neighbor) error {
     if err := apo.noc.Validate(); err != nil {
          return err
     }
     if n.AsPathOptions == nil {
          n.AsPathOptions = &apo.noc
          return nil
     }
     return ygot.MergeStructInto(n.AsPathOptions, &apo.noc)
}

// AugmentPeerGroup implements the bgp.PeerGroupFeature interface.
// This method augments the BGP peer-group with as path options feature.
// Use pg.WithFeature(apo) instead of calling this feature directly.
func (apo *AsPathOptions) AugmentPeerGroup(pg *fpoc.NetworkInstance_Protocol_Bgp_PeerGroup) error {
     if err := apo.poc.Validate(); err != nil {
          return err
     }
     if pg.AsPathOptions == nil {
          pg.AsPathOptions = &apo.poc
          return nil
     }
     return ygot.MergeStructInto(pg.AsPathOptions, &apo.poc)
}