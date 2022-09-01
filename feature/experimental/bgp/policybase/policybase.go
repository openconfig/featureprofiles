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

// Package policybase implements the Config Library for BGP policy base
// feature profile.
package policybase

import (
     "github.com/openconfig/featureprofiles/yang/fpoc"
     "github.com/openconfig/ygot/ygot"
)

// PolicyBase struct to store OC attributes.
type PolicyBase struct {
     noc fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi
     poc fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AfiSafi
}

// New returns a new PolicyBase object.
func New() *PolicyBase {
     return &PolicyBase{
          noc: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi{
               Enabled: ygot.Bool(true),
          },
          poc: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AfiSafi{
               Enabled: ygot.Bool(true),
          },
     }
}

// WithName sets the afi-safi-name for policy base feature.
func (p *PolicyBase) WithName(name fpoc.E_BgpTypes_AFI_SAFI_TYPE) *PolicyBase {
     p.noc.AfiSafiName = name
     p.poc.AfiSafiName = name
     return p
}

// WithDefaultImportPolicy sets the default-import-policy for policy base feature.
func (p *PolicyBase) WithDefaultImportPolicy(dip fpoc.E_RoutingPolicy_DefaultPolicyType) *PolicyBase {
     // Neighbor
     if p.noc.AfiSafi.ApplyPolicy == nil {
          p.noc.AfiSafi.ApplyPolicy = &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_ApplyPolicy{}
     }
     p.noc.AfiSafi.ApplyPolicy.DefaultImportPolicy = dip
     

     // Peer Group
     if p.poc.AfiSafi.ApplyPolicy == nil {
          p.poc.AfiSafi.ApplyPolicy = &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AfiSafi_ApplyPolicy{}
     }
     p.poc.AfiSafi.ApplyPolicy.DefaultImportPolicy = dip
     

     return p
}

// WithDefaultExportPolicy sets the default-export-policy for policy base feature.
func (p *PolicyBase) WithDefaultExportPolicy(dep fpoc.E_RoutingPolicy_DefaultPolicyType) *PolicyBase {
     // Neighbor
     if p.noc.AfiSafi.ApplyPolicy == nil {
          p.noc.AfiSafi.ApplyPolicy = &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_ApplyPolicy{}
     }
     p.noc.AfiSafi.ApplyPolicy.DefaultExportPolicy = dep
     

     // Peer Group
     if p.poc.AfiSafi.ApplyPolicy == nil {
          p.poc.AfiSafi.ApplyPolicy = &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AfiSafi_ApplyPolicy{}
     }
     p.poc.AfiSafi.ApplyPolicy.DefaultExportPolicy = dep
     

     return p
}

// WithExportPolicy sets the export-policy for policy base feature.
func (p *PolicyBase) WithExportPolicy(ep string) *PolicyBase {
     // Neighbor
     if p.noc.AfiSafi.ApplyPolicy == nil {
          p.noc.AfiSafi.ApplyPolicy = &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_ApplyPolicy{}
     }    
     p.noc.AfiSafi.ApplyPolicy.ExportPolicy = ygot.String(ep)
     

     // Peer Group
     if p.poc.AfiSafi.ApplyPolicy == nil {
          p.poc.AfiSafi.ApplyPolicy = &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AfiSafi_ApplyPolicy{}
     }    
     p.poc.AfiSafi.ApplyPolicy.ExportPolicy =  ygot.String(ep)
     

     return p
}

// WithImportPolicy sets the import-policy for policy base feature.
func (p *PolicyBase) WithImportPolicy(ip string) *PolicyBase {
     // Neighbor
     if p.noc.AfiSafi.ApplyPolicy == nil {
          p.noc.AfiSafi.ApplyPolicy = &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_ApplyPolicy{}
     }    
     p.noc.AfiSafi.ApplyPolicy.ImportPolicy = ygot.String(ip)
     

     // Peer Group
     if p.poc.AfiSafi.ApplyPolicy == nil {
          p.poc.AfiSafi.ApplyPolicy = &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AfiSafi_ApplyPolicy{}
     }    
     p.poc.AfiSafi.ApplyPolicy.ImportPolicy = ygot.String(ip)
     

     return p
}

// AugmentNeighbor implements the bgp.NeighborFeature interface.
// This method augments the BGP neighbor with policy base feature.
// Use with n.WithFeature(p) instead of calling this feature directly.
func (p *PolicyBase) AugmentNeighbor(n *fpoc.NetworkInstance_Protocol_Bgp_Neighbor) error {
     if err := p.noc.Validate(); err != nil {
          return err
     }
     if n.AfiSafi == nil {
          n.AfiSafi = &p.noc
          return nil
     }
     return ygot.MergeStructInto(n.AfiSafi, &p.noc)
}

// AugmentPeerGroup implements the bgp.PeerGroupFeature interface.
// This method augments the BGP peer group with policy base feature.
// Use with pg.WithFeature(p) instead of calling this feature directly.
func (p *PolicyBase) AugmentPeerGroup(pg *fpoc.NetworkInstance_Protocol_Bgp_PeerGroup) error {
     if err := p.poc.Validate(); err != nil {
          return err
     }
     if pg.AfiSafi == nil {
          pg.AfiSafi = &p.poc
          return nil
     }
     return ygot.MergeStructInto(pg.AfiSafi, &p.poc)
}