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

// Package bgpgracefulrestart implements the Config Library for BGP graceful
// restart feature profile.
package bgpgracefulrestart

import (
	"errors"
	"time"

	"github.com/openconfig/featureprofiles/yang/oc"
	"github.com/openconfig/ygot/ygot"
)

// GracefulRestart struct to store OC attributes.
type GracefulRestart struct {
	noc oc.NetworkInstance_Protocol_Bgp_Neighbor_GracefulRestart
	poc oc.NetworkInstance_Protocol_Bgp_PeerGroup_GracefulRestart
}

// New returs a new GracefulRestart object.
func New() *GracefulRestart {
	return &GracefulRestart{}
}

// WithRestartTime sets the restart-time for graceful restart feature.
func (gr *GracefulRestart) WithRestartTime(secs time.Duration) *GracefulRestart {
	if gr == nil {
		return nil
	}
	rt := ygot.Uint16(uint16(secs.Seconds()))
	gr.noc.RestartTime = rt
	gr.poc.RestartTime = rt
	return gr
}

// WithStaleRoutesTime sets the stale routes time for graceful restart feature.
func (gr *GracefulRestart) WithStaleRoutesTime(secs time.Duration) *GracefulRestart {
	if gr == nil {
		return nil
	}
	rt := ygot.Float64(secs.Seconds())
	gr.noc.StaleRoutesTime = rt
	gr.poc.StaleRoutesTime = rt
	return gr
}

// WithHelperOnly sets the helper-only attributed for graceful restart feature.
func (gr *GracefulRestart) WithHelperOnly(val bool) *GracefulRestart {
	if gr == nil {
		return nil
	}
	gr.noc.HelperOnly = ygot.Bool(val)
	gr.poc.HelperOnly = ygot.Bool(val)
	return gr
}

// AugmentNeighbor implements the bgp.NeighborFeature interface.
// This method augments the BGP neighbor with graceful restart feature.
// Use n.WithFeature(gr) instead of calling this method directly.
func (gr *GracefulRestart) AugmentNeighbor(n *oc.NetworkInstance_Protocol_Bgp_Neighbor) error {
	if gr == nil || n == nil {
		return errors.New("some args are nil")
	}
	if n.GracefulRestart != nil {
		return errors.New("neighbor GracefulRestart field is not nil")
	}
	n.GracefulRestart = &gr.noc
	return nil
}

// AugmentPeerGroup implements the bgp.PeerGroupFeature interface.
// This method augments the BGP peer-group with graceful restart feature.
// Use pg.WithFeature(gr) instead of calling this method directly.
func (gr *GracefulRestart) AugmentPeerGroup(pg *oc.NetworkInstance_Protocol_Bgp_PeerGroup) error {
	if gr == nil || pg == nil {
		return errors.New("some args are nil")
	}
	if pg.GracefulRestart != nil {
		return errors.New("peer-group GracefulRestart field is not nil")
	}
	pg.GracefulRestart = &gr.poc
	return nil
}
