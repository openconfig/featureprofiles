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
	"fmt"
	"testing"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/helpers"
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
	NexthopGroup    bool
	Metric          uint32
	Recurse         bool
	T               *testing.T
	TrafficType     oc.E_Aft_EncapsulationHeaderType
	PolicyName      string
	Rule            string
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
		if cfg.NexthopGroup {
			if deviations.StaticRouteToNextHopGroupOCNotSupported(d) {
				switch d.Vendor() {
				case ondatra.ARISTA:
					cli := fmt.Sprintf(`ipv6 route %s nexthop-group %s`, cfg.Prefix, v)
					helpers.GnmiCLIConfig(cfg.T, d, cli)
					staticRouteToNextHopGroupCLI(cfg.T, d, *cfg)
				default:
					return s, fmt.Errorf("deviation IPv4StaticRouteWithIPv6NextHopUnsupported is not handled for the dut: %s", d.Vendor())
				}
				return s, nil
			} else {
				ngName := fmt.Sprintf("%s", v)
				nhg := s.GetOrCreateNextHopGroup()
				nhg.SetName(ngName)
			}
		}
		nh := s.GetOrCreateNextHop(k)
		nh.SetIndex(k)
		nh.NextHop = v
		if cfg.Metric != 0 {
			nh.SetMetric(cfg.Metric)
		}
		if cfg.Recurse {
			nh.SetRecurse(cfg.Recurse)
		}
	}
	sp := gnmi.OC().NetworkInstance(ni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(d))
	gnmi.BatchUpdate(batch, sp.Config(), c)
	gnmi.BatchReplace(batch, sp.Static(cfg.Prefix).Config(), s)

	return s, nil
}

// staticRouteToNextHopGroupCLI configures routes to a next-hop-group for gue encapsulation
func staticRouteToNextHopGroupCLI(t *testing.T, dut *ondatra.DUTDevice, params StaticRouteCfg) {
	t.Helper()
	groupType := ""

	switch params.TrafficType {
	case oc.Aft_EncapsulationHeaderType_UDPV4:
		groupType = "ipv4"
	case oc.Aft_EncapsulationHeaderType_UDPV6:
		groupType = "ipv6"
	}

	// Configure traffic policy
	cli := ""
	switch dut.Vendor() {
	case ondatra.ARISTA:
		cli = fmt.Sprintf(`
				traffic-policies
				traffic-policy %s
      			match %s %s
         		actions
            	redirect next-hop group %s`, params.PolicyName, params.Rule, groupType, params.NextHops["0"])
		helpers.GnmiCLIConfig(t, dut, cli)
	default:
		t.Logf("Unsupported vendor %s for native command support for deviation 'policy-forwarding config'", dut.Vendor())
	}
}
