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
package routereflector

import (
     "github.com/openconfig/featureprofiles/yang/fpoc"
     "github.com/openconfig/ygot/ygot"
)

// RouteReflector struct to store OC attributes.
type RouteReflector struct {
     noc fpoc.NetworkInstance_Protocol_Bgp_Neighbor_RouteReflector
     poc fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_RouteReflector
}

// New returns a new RouteReflector object.
func New() *RouteReflector {
     return &RouteReflector{
          noc: &fpoc.NetworkInstance_Protocol_Bgp_Neighbor_RouteReflector{
               RouteReflectorClient: ygot.Bool(true),
          },
          poc: &fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_RouteReflector{
               RouteReflectorClient: ygot.Bool(true),
          },
     }
}

// WithStringRouteReflectorClusterId sets the route-reflector-cluster-id for route reflector feature.
func (rr *RouteReflector) WithStringRouteReflectorClusterId(id string) *RouteReflector {
     rr.noc.NetworkInstance_Protocol_Bgp_Neighbor_RouteReflector_RouteReflectorClusterId = ygot.UnionString(id)
     rr.poc.NetworkInstance_Protocol_Bgp_PeerGroup_RouteReflector_RouteReflectorClusterId = ygot.UnionString(id)
     return rr
}

// WithIntRouteReflectorClusterId sets the route-reflector-cluster-id for route reflector feature.
func (rr *RouteReflector) WithIntRouteReflectorClusterId(id uint32) *RouteReflector {
     rr.noc.NetworkInstance_Protocol_Bgp_Neighbor_RouteReflector_RouteReflectorClusterId = ygot.UnionUint32(id)
     rr.poc.NetworkInstance_Protocol_Bgp_PeerGroup_RouteReflector_RouteReflectorClusterId = ygot.UnionUint32(id)
     return rr
}

// AugmentNeighbor implements the bgp.NeighborFeature interface.
// This method augments the BGP neighbor with route reflector feature.
// Use with n.WithFeature(rr) instead of calling this feature directly.
func (rr *RouteReflector) AugmentNeighbor(n *fpoc.NetworkInstance_Protocol_Bgp_Neighbor) error {
     if err := rr.noc.Validate(); err != nil {
          return err
     }
     if n.RouteReflector == nil {
          n.RouteReflector = &rr.noc
          return nil
     }
     return ygot.MergeStructInto(n.RouteReflector, &rr.noc)
}

// AugmentPeerGroup implements the bgp.PeerGroupFeature interface.
// This method augments the BGP peer group with route reflector feature.
// Use with pg.WithFeature(rr) instead of calling this feature directly.
func (rr *RouteReflector) AugmentPeerGroup(pg *fpoc.NetworkInstance_Protocol_Bgp_PeerGroup) error {
     if err := pl.poc.Validate(); err != nil {
          return err
     }
     if pg.RouteReflector == nil {
          pg.RouteReflector = &rr.poc
          return nil
     }
     return ygot.MergeStructInto(pg.RouteReflector, &rr.poc)
}