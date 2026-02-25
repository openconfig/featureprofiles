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
	NetworkInstance  string
	Prefix           string
	NextHops         map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union
	IPType           string
	NextHopAddr      string
	NexthopGroup     bool
	NexthopGroupName string
	Metric           uint32
	Recurse          bool
	T                *testing.T
	TrafficType      oc.E_Aft_EncapsulationHeaderType
	PolicyName       string
	Rule             string
}

// StaticVRFRouteCfg represents a static route configuration within a specific network instance (VRF). It defines the destination prefix, associated next-hop group, and the protocol string used for identification.
type StaticVRFRouteCfg struct {
	NetworkInstance string
	Prefix          string
	NextHopGroup    string
	ProtocolStr     string
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
	if cfg.NexthopGroup {
		if deviations.StaticRouteToNHGOCUnsupported(d) {
			switch d.Vendor() {
			case ondatra.ARISTA:
				cli := fmt.Sprintf(`ipv6 route %s nexthop-group %s`, cfg.Prefix, cfg.NexthopGroupName)
				helpers.GnmiCLIConfig(cfg.T, d, cli)
				staticRouteToNextHopGroupCLI(cfg.T, d, *cfg)
			default:
				return s, fmt.Errorf("deviation StaticRouteToNHGOCUnsupported is not handled for the dut: %s", d.Vendor())
			}
		} else {
			nhg := s.GetOrCreateNextHopGroup()
			nhg.SetName(cfg.NexthopGroupName)
		}
	}
	if cfg.NextHops != nil {
		for k, v := range cfg.NextHops {
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
	}
	return s, nil
}

// StaticRouteNextNetworkInstance configures a static route with a next network instance (cross-VRF routing).
func StaticRouteNextNetworkInstance(t *testing.T, dut *ondatra.DUTDevice, cfg *StaticRouteCfg) {
	t.Helper()
	c := &oc.NetworkInstance_Protocol{
		Identifier: oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC,
		Name:       ygot.String(deviations.StaticProtocolName(dut)),
	}
	spNetInst := c.GetOrCreateStatic(cfg.Prefix)
	if deviations.StaticRouteNextNetworkInstanceOCUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			t.Logf("Configuring route with NextNetworkInstance")
			cli := fmt.Sprintf(`%s route vrf %s %s egress-vrf default %s
			`, cfg.IPType, cfg.NetworkInstance, cfg.NextHopAddr, cfg.Prefix)
			helpers.GnmiCLIConfig(t, dut, cli)
		default:
			// Log a message if the vendor is not supported for this specific CLI deviation.
			t.Logf("Unsupported vendor %s for native command support for deviation 'NextNetworkInstance config'", dut.Vendor())
		}
	} else {
		spNetInst.GetOrCreateNextHop("0").SetNextNetworkInstance("DEFAULT")
		spNetInst.GetOrCreateNextHop("0").SetNextHop(oc.UnionString(cfg.Prefix))
	}
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
            	redirect next-hop group %s`, params.PolicyName, params.Rule, groupType, params.NexthopGroupName)
		helpers.GnmiCLIConfig(t, dut, cli)
	default:
		t.Logf("Unsupported vendor %s for native command support for deviation 'policy-forwarding config'", dut.Vendor())
	}
}

// NewStaticVRFRoute configures a static route inside a given VRF on the DUT.
func NewStaticVRFRoute(t *testing.T, batch *gnmi.SetBatch, cfg *StaticVRFRouteCfg, d *ondatra.DUTDevice) (*oc.NetworkInstance_Protocol_Static, error) {
	t.Helper()
	if cfg == nil {
		return nil, errors.New("cfg must be defined")
	}

	if deviations.NextHopGroupOCUnsupported(d) {
		switch d.Vendor() {
		case ondatra.ARISTA:
			staticNHGCmd := fmt.Sprintf(`
				%s route vrf %s %s nexthop-group %s
			`, cfg.ProtocolStr, cfg.NetworkInstance, cfg.Prefix, cfg.NextHopGroup)
			helpers.GnmiCLIConfig(t, d, staticNHGCmd)

			// Return nil since we're using CLI (no OC object created)
			return nil, nil
		default:
			t.Errorf("Deviation NextHopGroupOCUnsupported is not handled for the dut: %v", d.Vendor())
		}
	}

	ni := normalizeNIName(cfg.NetworkInstance, d)

	c := &oc.NetworkInstance_Protocol{
		Identifier: oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC,
		Name:       ygot.String(deviations.StaticProtocolName(d)),
	}
	s := c.GetOrCreateStatic(cfg.Prefix)
	s.GetOrCreateNextHopGroup().SetName(cfg.NextHopGroup)

	sp := gnmi.OC().NetworkInstance(ni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(d))
	gnmi.BatchUpdate(batch, sp.Config(), c)
	gnmi.BatchReplace(batch, sp.Static(cfg.Prefix).Config(), s)

	return s, nil
}
