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
package addpath

import (
     "github.com/openconfig/featureprofiles/yang/fpoc"
     "github.com/openconfig/ygot/ygot"
)

// Add
type AddPath struct {
     goc *fpoc.NetworkInstance_Protocol_Bgp_Global_AfiSafi_AddPaths
     noc *fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_AddPaths
     poc *fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AfiSafi_AddPaths
}

// New returns a new AddPath object.
func New() *AddPath {
     return &AddPath{
          goc: fpoc.NetworkInstance_Protocol_Bgp_Global_AfiSafi_AddPaths{
               Enabled: ygot.Bool(true),
          },
          noc: fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_AddPaths{
               Enabled: ygot.Bool(true),
          },
          poc: fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AfiSafi_AddPaths{
               Enabled: ygot.Bool(true),
          },
     }
}

// WithReceive sets the receive for AddPath feature.
func (ap *AddPath) WithReceive(val bool) *AddPath {
     ap.goc.Receive = ygot.Bool(val)
     ap.noc.Receive = ygot.Bool(val)
     ap.poc.Receive = ygot.Bool(val)
     return ap
}

// WithSend sets the send for AddPath feature.
func (ap *AddPath) WithSend(val bool) *AddPath {
     ap.goc.Send = ygot.Bool(val)
     ap.noc.Send = ygot.Bool(val)
     ap.poc.Send = ygot.Bool(val)
     return ap
}

// WithSendMax sets the send max for AddPath feature.
func (ap *AddPath) WithSendMax(val uint8) *AddPath {
     ap.goc.SendMax = ygot.Uint8(val)
     ap.noc.SendMax = ygot.Uint8(val)
     ap.poc.SendMax = ygot.Uint8(val)
     return ap
}

// WithEligiblePrefixPolicy sets the eligible prefix policy
// for AddPath feature.
func (ap *AddPath) WithEligiblePrefixPolicy(pol string) *AddPath {
     ap.goc.EligiblePrefixPolicy = ygot.String(pol)
     ap.noc.EligiblePrefixPolicy = ygot.String(pol)
     ap.poc.EligiblePrefixPolicy = ygot.String(pol)
     return ap
}

// AugmentGlobal implements the bgp.GlobalFeature interface.
// This method augments the BGP global with add path feature.
// Use with n.WithFeature(ap) instead of calling this feature directly.
func (ap *AddPath) AugmentGlobal(g *fpoc.NetworkInstance_Protocol_Bgp_Global) error {
     if err := ap.goc.Validate(); err != nil {
          return err
     }
     if g.AfiSafi == nil {
          g.AfiSafi = &fpoc.NetworkInstance_Protocol_Bgp_Global_AfiSafi{
               AddPaths: &ap.goc
          }
     }
     else if g.AfiSafi.AddPaths == nil {
          g.AfiSafi.AddPaths = &ap.goc
          return nil
     }
     return ygot.MergeStructInto(g.AfiSafi.AddPaths, &ap.goc)
}

// AugmentNeighbor implements the bgp.NeighborFeature interface.
// This method augments the BGP neighbor with add path feature.
// Use n.WithFeature(ap) instead of calling this feature directly.
func (ap *AddPath) AugmentNeighbor(n *fpoc.NetworkInstance_Protocol_Bgp_Neighbor) error {
     if err := ap.noc.Validate(); err != nil {
          return err
     }
     if n.AfiSafi == nil {
          n.AfiSafi = &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi{
               AddPaths: &ap.noc
          }
     }
     else if n.AfiSafi.AddPaths == nil {
          n.AfiSafi.AddPaths = &ap.noc
          return nil
     }
     return ygot.MergeStructInto(n.AfiSafi.AddPaths, &ap.noc)
}

// AugmentPeerGroup implements the bgp.PeerGroupFeature interface.
// This method augments the BGP peer-group with as path options feature.
// Use pg.WithFeature(apo) instead of calling this feature directly.
func (ap *AddPath) AugmentPeerGroup(pg *fpoc.NetworkInstance_Protocol_Bgp_PeerGroup) error {
     if err := ap.poc.Validate(); err != nil {
          return err
     }
     if pg.AfiSafi == nil {
          pg.AfiSafi = &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_AfiSafi{
               AddPaths: &ap.poc
          }
     }
     else if pg.AfiSafi.AddPaths == nil {
          pg.AfiSafi.AddPaths = &ap.poc
          return nil
     }
     return ygot.MergeStructInto(pg.AfiSafi.AddPaths, &ap.poc)
}