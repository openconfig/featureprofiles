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

// Package addpath implements the config library for BGP add path
// feature profile.
package dynamicneighbors

import (
     "github.com/openconfig/featureprofiles/yang/fpoc"
     "github.com/openconfig/ygot/ygot"
)

// DynamicNeighbors struct to store OC attributes.
type DynamicNeighbors struct {
     goc fpoc.NetworkInstance_Protocol_Bgp_Global_DynamicNeighborPrefix
}

// New returns a new DynamicNeighbors object.
func New() *DynamicNeighbors {
     return &DynamicNeighbors{
          goc: fpoc.NetworkInstance_Protocol_Bgp_Global_DynamicNeighborPrefix{
               Enabled: ygot.Bool(true),
          },
     }
}

// WithPrefix sets the prefix for DynamicNeighbors feature.
func (dn *DynamicNeighbors) WithPrefix(prefix string) *DynamicNeighbors {
     dn.goc.Prefix = ygot.String(prefix)
     return dn
}

// WithPeerGroup sets the peer group for DynamicNeighbors feature.
func (dn *DynamicNeighbors) WithPeerGroup(pg string) *DynamicNeighbors {
     dn.goc.PeerGroup = ygot.String(pg)
     return dn
}

// AugmentGlobal implements the bgp.GlobalFeature interface.
// This method augments the BGP global with dynamic neighbors feature.
// Use g.WithFeature(dn) instead of calling this feature directly.
func (dn *DynamicNeighbors) AugmentGlobal(g *fpoc.NetworkInstance_Protocol_Bgp_Global) error {
     if err := dn.goc.Validate(); err != nil {
          return err
     }
     if g.DynamicNeighborPrefix == nil {
          g.DynamicNeighborPrefix = &dn.goc
          return nil
     }
     return ygot.MergeStructInto(g.DynamicNeighborPrefix, &g.goc)
}