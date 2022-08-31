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

// Package bestpath implements the config library for BGP as best path
// feature profile.
package bestpath

import (
     "github.com/openconfig/featureprofiles/yang/fpoc",
     "github.com/openconfig/ygot/ygot"
)

// BestPath struct to store OC attributes
type BestPath struct {
     goc fpoc.NetworkInstance_Protocol_Bgp_Global_RouteSelectionOptions
}

// New returns a new BestPath object
func New() *BestPath {
     return &BestPath{
          goc: fpoc.NetworkInstance_Protocol_Bgp_Global_RouteSelectionOptions{},
     }
}

// WithAlwaysCompareMed sets the always compare med for BestPath feature.
func (bp *BestPath) WithAlwaysCompareMed(val bool) *BestPath {
     bp.goc.AlwaysCompareMed = ygot.Bool(val)
     return bp
}

// WithIgnoreAsPathLength sets the ignore as path length for BestPath feature.
func (bp *BestPath) WithIgnoreAsPathLength(val bool) *BestPath {
     bp.goc.IgnoreAsPathLength = ygot.Bool(val)
     return bp
}

// WithCompareRid sets the compare rid for BestPath feature.
func (bp *BestPath) WithCompareRid(val bool) *BestPath {
     bp.goc.ExternalCompareRouterId = ygot.Bool(val)
     return bp
}

// WithAdvertiseInactiveRoutes sets the advertise inactive routes
// for BestPath feature.
func (bp *BestPath) WithAdvertiseInactiveRoutes(val bool) *BestPath {
     bp.goc.AdvertiseInactiveRoutes = ygot.Bool(val)
     return bp
}

// AugmentGlobal implements the bgp.GlobalFeature interface.
// This method augments the BGP Global with best path feature.
// Use g.WithFeature(bp) instead of calling this method directly.
func (bp *BestPath) AugmentGlobal(g *fpoc.NetworkInstance_Protocol_Bgp_Global) error {
     if err := bp.goc.Validate(); err != nil {
          return err
     }
     if g.RouteSelectionOptions == nil {
          g.RouteSelectionOptions = &bp.goc
          return nil
     }
     return ygot.MergeStructInto(g.RouteSelectionOptions, &gr.goc)
}