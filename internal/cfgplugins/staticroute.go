// Copyright 2023 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cfgplugins

import (
	"errors"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

// StaticRouteCfg defines commonly used attributes for setting a static route
type StaticRouteCfg struct {
	NetworkInstance string
	Prefix          string
	NextHops        map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union
}

// NewStaticRouteCfg provides OC configuration for a static route for a specific NetworkInstance,
// Prefix and NextHops.
//
// Configuration deviations are applied based on the ondatra device passed in.
func NewStaticRouteCfg(batch *gnmi.SetBatch, cfg *StaticRouteCfg, d *ondatra.DUTDevice) (*oc.NetworkInstance_Protocol_Static, error) {
	if cfg == nil {
		return nil, errors.New("cfg must be defined")
	}

	ni := normalizeNIName(cfg.NetworkInstance, d)

	c := &oc.NetworkInstance_Protocol{
		Identifier: oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC,
		Name:       ygot.String(deviations.StaticProtocolName(d)),
	}
	s := c.GetOrCreateStatic(cfg.Prefix)
	for k, v := range cfg.NextHops {
		nh := s.GetOrCreateNextHop(k)
		nh.NextHop = v
	}
	sp := gnmi.OC().NetworkInstance(ni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(d))
	gnmi.BatchReplace(batch, sp.Static(cfg.Prefix).Config(), s)

	return s, nil
}
