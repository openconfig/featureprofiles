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

// Package gracefulrestart implements the Config Library for BGP graceful
// restart feature profile.
package gracefulrestart

import (
	"time"

	"github.com/openconfig/featureprofiles/yang/fpoc"
	"github.com/openconfig/ygot/ygot"
)

// GracefulRestart struct to store OC attributes.
type GracefulRestart struct {
	noc fpoc.NetworkInstance_Protocol_Bgp_Neighbor_GracefulRestart
	poc fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_GracefulRestart
	goc fpoc.NetworkInstance_Protocol_Bgp_Global_GracefulRestart
}

// New returs a new GracefulRestart object.
func New() *GracefulRestart {
	return &GracefulRestart{
		noc: fpoc.NetworkInstance_Protocol_Bgp_Neighbor_GracefulRestart{
			Enabled: ygot.Bool(true),
		},
		poc: fpoc.NetworkInstance_Protocol_Bgp_PeerGroup_GracefulRestart{
			Enabled: ygot.Bool(true),
		},
		goc: fpoc.NetworkInstance_Protocol_Bgp_Global_GracefulRestart{
			Enabled: ygot.Bool(true),
		},
	}
}

// WithRestartTime sets the restart-time for graceful restart feature.
func (gr *GracefulRestart) WithRestartTime(secs time.Duration) *GracefulRestart {
	rt := ygot.Uint16(uint16(secs.Seconds()))
	gr.noc.RestartTime = rt
	gr.poc.RestartTime = rt
	gr.goc.RestartTime = rt
	return gr
}

// WithStaleRoutesTime sets the stale routes time for graceful restart feature.
func (gr *GracefulRestart) WithStaleRoutesTime(secs time.Duration) *GracefulRestart {
	rt := ygot.Float64(secs.Seconds())
	gr.noc.StaleRoutesTime = rt
	gr.poc.StaleRoutesTime = rt
	gr.goc.StaleRoutesTime = rt
	return gr
}

// WithHelperOnly sets the helper-only attributed for graceful restart feature.
func (gr *GracefulRestart) WithHelperOnly(val bool) *GracefulRestart {
	gr.noc.HelperOnly = ygot.Bool(val)
	gr.poc.HelperOnly = ygot.Bool(val)
	return gr
}

// AugmentNeighbor implements the bgp.NeighborFeature interface.
// This method augments the BGP neighbor with graceful restart feature.
// Use n.WithFeature(gr) instead of calling this method directly.
func (gr *GracefulRestart) AugmentNeighbor(n *fpoc.NetworkInstance_Protocol_Bgp_Neighbor) error {
	if err := gr.noc.Validate(); err != nil {
		return err
	}
	if n.GracefulRestart == nil {
		n.GracefulRestart = &gr.noc
		return nil
	}
	return ygot.MergeStructInto(n.GracefulRestart, &gr.noc)
}

// AugmentPeerGroup implements the bgp.PeerGroupFeature interface.
// This method augments the BGP peer-group with graceful restart feature.
// Use pg.WithFeature(gr) instead of calling this method directly.
func (gr *GracefulRestart) AugmentPeerGroup(pg *fpoc.NetworkInstance_Protocol_Bgp_PeerGroup) error {
	if err := gr.poc.Validate(); err != nil {
		return err
	}
	if pg.GracefulRestart == nil {
		pg.GracefulRestart = &gr.poc
		return nil
	}
	return ygot.MergeStructInto(pg.GracefulRestart, &gr.poc)
}

// AugmentGlobal implements the bgp.GlobalFeature interface.
// This method augments the BGP Global  with graceful restart feature.
// Use g.WithFeature(gr) instead of calling this method directly.
func (gr *GracefulRestart) AugmentGlobal(g *fpoc.NetworkInstance_Protocol_Bgp_Global) error {
	if err := gr.goc.Validate(); err != nil {
		return err
	}
	if g.GracefulRestart == nil {
		g.GracefulRestart = &gr.goc
		return nil
	}
	return ygot.MergeStructInto(g.GracefulRestart, &gr.goc)
}
