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
	"strings"
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
	NetworkInstance     string
	Prefix              string
	NextHops            map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union
	NextNetworkInstance string // Egress network instance for cross-VRF routing (e.g. egress-vrf)
	IPType              string
	NextHopAddr         string
	NexthopGroup        bool
	NexthopGroupName    string
	Metric              uint32
	Recurse             bool
	T                   *testing.T
	TrafficType         oc.E_Aft_EncapsulationHeaderType
	PolicyName          string
	Rule                string
	NextHopIntf         string
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

	cliConfigured := false
	if cfg.NexthopGroup {
		if deviations.StaticRouteToNHGOCUnsupported(d) {
			switch d.Vendor() {
			case ondatra.ARISTA:
				cli := fmt.Sprintf(`ipv6 route %s nexthop-group %s`, cfg.Prefix, cfg.NexthopGroupName)
				helpers.GnmiCLIConfig(cfg.T, d, cli)
				staticRouteToNextHopGroupCLI(cfg.T, d, *cfg)
				cliConfigured = true
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
			if cfg.NextNetworkInstance != "" {
				nh.NextNetworkInstance = ygot.String(cfg.NextNetworkInstance)
			}
		}
	}
	// Handle Interface-based NextHop (Resolution routes)
	if cfg.NextHopIntf != "" {
		// Usually "0" is used as the index if only one interface is provided
		nh := s.GetOrCreateNextHop("0")
		nh.GetOrCreateInterfaceRef().Interface = ygot.String(cfg.NextHopIntf)
		if cfg.NextNetworkInstance != "" {
			nh.NextNetworkInstance = ygot.String(cfg.NextNetworkInstance)
		}
	}

	if cliConfigured {
		return s, nil
	}

	sp := gnmi.OC().NetworkInstance(ni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(d))
	gnmi.BatchUpdate(batch, sp.Config(), c)
	gnmi.BatchReplace(batch, sp.Static(cfg.Prefix).Config(), s)
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

// ConfigureStaticRouteParams contains the parameters required to configure a static route on the DUT.
type ConfigureStaticRouteParams struct {
	NetworkInstance string
	Prefix          string
	Index           string
	NextHop         string
}

// ConfigureStaticRoute installs a static route into the default NI.
func ConfigureStaticRoute(t *testing.T, dut *ondatra.DUTDevice, batch *gnmi.SetBatch, cfg ConfigureStaticRouteParams) {
	t.Helper()
	staticRoute := &StaticRouteCfg{
		NetworkInstance: cfg.NetworkInstance,
		Prefix:          cfg.Prefix,
		NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
			cfg.Index: oc.UnionString(cfg.NextHop),
		},
	}

	if _, err := NewStaticRouteCfg(batch, staticRoute, dut); err != nil {
		t.Fatalf("Failed to configure static route %s: %v", cfg.Prefix, err)
	}
}

// ConfigureStaticRoutesInVRF configures static routes via OpenConfig for named-VRF routes
// without egress-vrf, and via CLI for egress-vrf or default-VRF routes (Arista).
func ConfigureStaticRoutesInVRF(t *testing.T, dut *ondatra.DUTDevice, routes []*StaticRouteCfg) {
	t.Helper()

	// Group named-VRF routes without NextNetworkInstance for OC configuration.
	ocRoutesByVRF := make(map[string][]*StaticRouteCfg)
	for _, r := range routes {
		if r.NextNetworkInstance == "" && r.NetworkInstance != "" {
			ocRoutesByVRF[r.NetworkInstance] = append(ocRoutesByVRF[r.NetworkInstance], r)
		}
	}

	// Configure named-VRF plain next-hop routes via OpenConfig.
	for vrfName, vrfRoutes := range ocRoutesByVRF {
		proto := &oc.NetworkInstance_Protocol{
			Identifier: oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC,
			Name:       ygot.String(deviations.StaticProtocolName(dut)),
		}
		for _, r := range vrfRoutes {
			sr := proto.GetOrCreateStatic(r.Prefix)
			sr.Prefix = ygot.String(r.Prefix)
			if r.NextHops != nil {
				for idx, nhVal := range r.NextHops {
					nh := sr.GetOrCreateNextHop(idx)
					nh.Index = ygot.String(idx)
					nh.NextHop = nhVal
				}
			} else if r.NextHopAddr != "" {
				nh := sr.GetOrCreateNextHop("0")
				nh.Index = ygot.String("0")
				nh.NextHop = oc.UnionString(r.NextHopAddr)
			}
		}
		sp := gnmi.OC().NetworkInstance(vrfName).Protocol(
			oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
		gnmi.Update(t, dut, sp.Config(), proto)
	}

	if deviations.StaticRouteInVrfOcUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			// Configure egress-vrf/default-VRF routes via CLI, batched into a single
			// gNMI CLI Set to avoid one Set per route (expensive at scale).
			var cliLines []string
			for _, r := range routes {
				if r.NextNetworkInstance == "" && r.NetworkInstance != "" {
					continue // already handled via OC above
				}
				nextHop := r.NextHopAddr
				if nextHop == "" && len(r.NextHops) > 0 {
					for _, v := range r.NextHops {
						if str, ok := v.(oc.UnionString); ok {
							nextHop = string(str)
							break
						}
					}
				}
				ipType := "ip"
				for _, ch := range r.Prefix {
					if ch == ':' {
						ipType = "ipv6"
						break
					}
				}
				var cli string
				switch {
				case r.NextNetworkInstance != "" && r.NetworkInstance != "":
					cli = fmt.Sprintf("%s route vrf %s %s egress-vrf %s %s",
						ipType, r.NetworkInstance, r.Prefix, r.NextNetworkInstance, nextHop)
				case r.NextNetworkInstance == "" && r.NetworkInstance == "":
					// Default VRF, plain next-hop — no vrf qualifier.
					cli = fmt.Sprintf("%s route %s %s", ipType, r.Prefix, nextHop)
				default:
					// NextNetworkInstance set but NetworkInstance is empty (edge case: egress from default VRF).
					cli = fmt.Sprintf("%s route %s egress-vrf %s %s",
						ipType, r.Prefix, r.NextNetworkInstance, nextHop)
				}
				cliLines = append(cliLines, cli)
			}
			if len(cliLines) > 0 {
				helpers.GnmiCLIConfig(t, dut, strings.Join(cliLines, "\n"))
			}
		}
	} else {
		// Configure via OC: routes with NextNetworkInstance or in the default VRF.
		for _, r := range routes {
			if r.NextNetworkInstance == "" && r.NetworkInstance != "" {
				continue // already handled via OC above
			}
			vrfName := r.NetworkInstance
			if vrfName == "" {
				vrfName = deviations.DefaultNetworkInstance(dut)
			}
			proto := &oc.NetworkInstance_Protocol{
				Identifier: oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC,
				Name:       ygot.String(deviations.StaticProtocolName(dut)),
			}
			sr := proto.GetOrCreateStatic(r.Prefix)
			sr.Prefix = ygot.String(r.Prefix)
			if r.NextHops != nil {
				for idx, v := range r.NextHops {
					nh := sr.GetOrCreateNextHop(idx)
					nh.Index = ygot.String(idx)
					nh.NextHop = v
					if r.NextNetworkInstance != "" {
						nh.NextNetworkInstance = ygot.String(r.NextNetworkInstance)
					}
				}
			} else {
				nh := sr.GetOrCreateNextHop("0")
				nh.Index = ygot.String("0")
				nh.NextHop = oc.UnionString(r.NextHopAddr)
				if r.NextNetworkInstance != "" {
					nh.NextNetworkInstance = ygot.String(r.NextNetworkInstance)
				}
			}
			sp := gnmi.OC().NetworkInstance(vrfName).Protocol(
				oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
			gnmi.Update(t, dut, sp.Config(), proto)
		}
	}
}

// RemoveStaticRoutesInVRF removes the given static routes; it mirrors
// ConfigureStaticRoutesInVRF, emitting "no" CLI on Arista and OC deletes elsewhere.
func RemoveStaticRoutesInVRF(t *testing.T, dut *ondatra.DUTDevice, routes []*StaticRouteCfg) {
	t.Helper()

	if deviations.StaticRouteInVrfOcUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			var cliLines []string
			for _, r := range routes {
				nextHop := r.NextHopAddr
				if nextHop == "" && len(r.NextHops) > 0 {
					for _, v := range r.NextHops {
						if str, ok := v.(oc.UnionString); ok {
							nextHop = string(str)
							break
						}
					}
				}
				ipType := "ip"
				if strings.Contains(r.Prefix, ":") {
					ipType = "ipv6"
				}
				var cli string
				switch {
				case r.NextNetworkInstance != "" && r.NetworkInstance != "":
					cli = fmt.Sprintf("%s route vrf %s %s egress-vrf %s %s",
						ipType, r.NetworkInstance, r.Prefix, r.NextNetworkInstance, nextHop)
				case r.NextNetworkInstance == "" && r.NetworkInstance == "":
					cli = fmt.Sprintf("%s route %s %s", ipType, r.Prefix, nextHop)
				case r.NextNetworkInstance == "" && r.NetworkInstance != "":
					cli = fmt.Sprintf("%s route vrf %s %s %s", ipType, r.NetworkInstance, r.Prefix, nextHop)
				default:
					cli = fmt.Sprintf("%s route %s egress-vrf %s %s",
						ipType, r.Prefix, r.NextNetworkInstance, nextHop)
				}
				cliLines = append(cliLines, "no "+cli)
			}
			if len(cliLines) > 0 {
				helpers.GnmiCLIConfig(t, dut, strings.Join(cliLines, "\n"))
			}
		}
		return
	}

	for _, r := range routes {
		vrfName := r.NetworkInstance
		if vrfName == "" {
			vrfName = deviations.DefaultNetworkInstance(dut)
		}
		sp := gnmi.OC().NetworkInstance(vrfName).Protocol(
			oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
		gnmi.Delete(t, dut, sp.Static(r.Prefix).Config())
	}
}
