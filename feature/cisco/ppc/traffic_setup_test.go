// Copyright 2021 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ppc_test

import (
	"testing"
	"time"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
)

const (
	dst                   = "202.1.0.1"
	v4mask                = "32"
	v6mask                = "128"
	dstCount              = 1
	innersrcPfx           = "200.1.0.1"
	totalbgpPfx           = 1 //set value for scale bgp setup ex: 100000
	innerdstPfxMin_bgp    = "202.1.0.1"
	innerdstPfxCount_bgp  = 1 //set value for number of inner prefix for bgp flow
	totalisisPfx          = 1 //set value for scale isis setup ex: 10000
	innerdstPfxMin_isis   = "201.1.0.1"
	innerdstPfxCount_isis = 1 //set value for number of inner prefix for isis flow

)

// TGNoptions are optional parameters to a validate traffic function.
type TGNoptions struct {
	drop, mpls, ipv4, ttl bool
	traffic_timer         int
	fps                   uint64
	fpercent              float64
	frame_size            uint32
	event                 eventType
}

// configureATE configures ports on the ATE.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) *ondatra.ATETopology {
	atePorts := sortPorts(ate.Ports())
	top := ate.Topology().New()

	atesrc := atePorts[0]
	i1 := top.AddInterface(ateSrc.Name).WithPort(atesrc)
	i1.IPv4().
		WithAddress(ateSrc.IPv4CIDR()).
		WithDefaultGateway(dutSrc.IPv4)
	i1.IPv6().
		WithAddress(ateSrc.IPv6CIDR()).
		WithDefaultGateway(dutSrc.IPv6)

	i2 := top.AddInterface(ateDst.Name)
	lag := top.AddLAG("lag").WithPorts(atePorts[1:]...)
	lag.LACP().WithEnabled(true)
	i2.WithLAG(lag)

	// Disable FEC for 100G-FR ports because Novus does not support it.
	if atesrc.PMD() == ondatra.PMD100GBASEFR {
		i1.Ethernet().FEC().WithEnabled(false)
	}
	is100gfr := false
	for _, p := range atePorts[1:] {
		if p.PMD() == ondatra.PMD100GBASEFR {
			is100gfr = true
		}
	}
	if is100gfr {
		i2.Ethernet().FEC().WithEnabled(false)
	}
	top.Push(t).StartProtocols(t)

	i2.IPv4().
		WithAddress(ateDst.IPv4CIDR()).
		WithDefaultGateway(dutDst.IPv4)
	i2.IPv6().
		WithAddress(ateDst.IPv6CIDR()).
		WithDefaultGateway(dutDst.IPv6)
	top.Update(t)
	top.StartProtocols(t)
	return top
}

// addAteISISL2 configures ISIS L2 ATE config
func addAteISISL2(t *testing.T, topo *ondatra.ATETopology, atePort, areaId, network_name string, metric uint32, v4prefix string, count uint32) {

	intfs := topo.Interfaces()
	if len(intfs) == 0 {
		t.Fatal("There are no interfaces in the Topology")
	}
	network := intfs[atePort].AddNetwork(network_name)
	network.ISIS().WithIPReachabilityMetric(metric + 1)
	network.IPv4().WithAddress(v4prefix).WithCount(count)
	rnetwork := intfs[atePort].AddNetwork("recursive")
	rnetwork.ISIS().WithIPReachabilityMetric(metric + 1)
	rnetwork.IPv4().WithAddress("100.100.100.100/32")
	intfs[atePort].ISIS().WithAreaID(areaId).WithLevelL2().WithNetworkTypePointToPoint().WithMetric(metric).WithWideMetricEnabled(true)
}

// addAteEBGPPeer configures EBGP ATE config
func addAteEBGPPeer(t *testing.T, topo *ondatra.ATETopology, atePort, peerAddress string, localAsn uint32, network_name, nexthop, prefix string, count uint32, useLoopback bool) {

	intfs := topo.Interfaces()
	if len(intfs) == 0 {
		t.Fatal("There are no interfaces in the Topology")
	}
	//

	network := intfs[atePort].AddNetwork(network_name)
	bgpAttribute := network.BGP()
	bgpAttribute.WithActive(true).WithNextHopAddress(nexthop)

	//Add prefixes, Add network instance
	if prefix != "" {

		network.IPv4().WithAddress(prefix).WithCount(count)
	}
	//Create BGP instance
	bgp := intfs[atePort].BGP()
	bgpPeer := bgp.AddPeer().WithPeerAddress(peerAddress).WithLocalASN(localAsn).WithTypeExternal()
	bgpPeer.WithOnLoopback(useLoopback)

	//Update bgpCapabilities
	bgpPeer.Capabilities().WithIPv4UnicastEnabled(true).WithIPv6UnicastEnabled(true).WithGracefulRestart(true)
}

// addPrototoAte calls ISIS/BGP api
func addPrototoAte(t *testing.T, top *ondatra.ATETopology) {

	//advertising 100.100.100.100/32 for bgp resolve over IGP prefix
	intfs := top.Interfaces()
	intfs["ateDst"].WithIPv4Loopback("100.100.100.100/32")

	addAteISISL2(t, top, "ateDst", "B4", "isis_network", 20, innerdstPfxMin_isis+"/"+v4mask, totalisisPfx)

	addAteEBGPPeer(t, top, "ateDst", dutDst.IPv4, 64001, "bgp_recursive", ateDst.IPv4, innerdstPfxMin_bgp+"/"+v4mask, totalbgpPfx, true)

	top.Push(t).StartProtocols(t)
}

// createFlow returns a flow from atePort1 to the dstPfx, expected to arrive on ATE interface dst.
func (a *testArgs) createFlow(name string, dstEndPoint []ondatra.Endpoint, opts ...*TGNoptions) *ondatra.Flow {
	srcEndPoint := a.top.Interfaces()[ateSrc.Name]
	var flow *ondatra.Flow
	var header []ondatra.Header

	for _, opt := range opts {
		if opt.mpls {
			hdr_mpls := ondatra.NewMPLSHeader()
			header = []ondatra.Header{ondatra.NewEthernetHeader(), hdr_mpls}
		}
		if opt.ipv4 {
			var hdr_ipv4 *ondatra.IPv4Header
			// explicity set ttl 0 if zero
			if opt.ttl {
				hdr_ipv4 = ondatra.NewIPv4Header().WithTTL(0)
			} else {
				hdr_ipv4 = ondatra.NewIPv4Header()
			}
			hdr_ipv4.WithSrcAddress(dutSrc.IPv4).DstAddressRange().WithMin(dst).WithCount(dstCount).WithStep("0.0.0.1")
			header = []ondatra.Header{ondatra.NewEthernetHeader(), hdr_ipv4}
		}
	}
	flow = a.ate.Traffic().NewFlow(name).
		WithSrcEndpoints(srcEndPoint).
		WithDstEndpoints(dstEndPoint...).
		WithHeaders(header...)

	if opts[0].fps != 0 {
		flow.WithFrameRateFPS(opts[0].fps)
	} else {
		flow.WithFrameRateFPS(1000)
	}

	flow.WithFrameRatePct(100)
	if opts[0].frame_size != 0 {
		flow.WithFrameSize(opts[0].frame_size)
	} else if opts[0].fpercent != 0 {
		flow.WithFrameRatePct((opts[0].fpercent))
	} else {
		flow.WithFrameSize(300)
	}

	return flow
}

// validateTrafficFlows validates traffic loss on tgn side and DUT incoming and outgoing counters
func (a *testArgs) validateTrafficFlows(t *testing.T, flow *ondatra.Flow, opts ...*TGNoptions) uint64 {
	a.ate.Traffic().Start(t, flow)
	// run traffic for 30 seconds, before introducing fault
	time.Sleep(time.Duration(30) * time.Second)

	// Set configs if needed for scenario
	for _, op := range opts {
		if eventAction, ok := op.event.(*event_interface_config); ok {
			eventAction.interface_config(t)
		} else if eventAction, ok := op.event.(*event_static_route_to_null); ok {
			eventAction.static_route_to_null(t)
		} else if eventAction, ok := op.event.(*event_enable_mpls_ldp); ok {
			eventAction.enable_mpls_ldp(t)
		}
	}

	// Space to add trigger code
	for _, tt := range triggers {
		t.Logf("Name: %s", tt.name)
		t.Logf("Description: %s", tt.desc)
		// if triggerAction, ok := tt.trigger_type.(*trigger_process_restart); ok {
		// 	triggerAction.restartProcessBackground(t, a.ctx)
		// }
		// if chassis_type == "distributed" && with_RPFO {
		// 	if triggerAction, ok := tt.trigger_type.(*trigger_rpfo); ok {
		// 		// false is for not reloading the box, since there is standby RP on distributed tb, we don't do a reload
		// 		triggerAction.rpfo(t, a.ctx, false)
		// 	}
		// } else if chassis_type == "fixed" && with_RPFO {
		// 	if triggerAction, ok := tt.trigger_type.(*trigger_rpfo); ok {
		// 		// true is for reloading the box, since there is no RPFO on fixed tb, we do a reload
		// 		triggerAction.rpfo(t, a.ctx, true)
		// 		tolerance = triggerAction.tolerance
		// 	}
		// }
		// if chassis_type == "distributed" && with_lc_reload {
		// 	if triggerAction, ok := tt.trigger_type.(*trigger_lc_reload); ok {
		// 		triggerAction.lc_reload(t)
		// 		tolerance = triggerAction.tolerance
		// 	}
		// }
	}

	time.Sleep(time.Duration(opts[0].traffic_timer) * time.Second)
	a.ate.Traffic().Stop(t)

	// remove set configs before further check
	for _, op := range opts {
		if _, ok := op.event.(*event_interface_config); ok {
			eventAction := event_interface_config{config: false, mtu: 1514, port: sortPorts(a.dut.Ports())[1:]}
			eventAction.interface_config(t)
		} else if _, ok := op.event.(*event_static_route_to_null); ok {
			eventAction := event_static_route_to_null{prefix: "202.1.0.1/32", config: false}
			eventAction.static_route_to_null(t)
		} else if _, ok := op.event.(*event_enable_mpls_ldp); ok {
			eventAction := event_enable_mpls_ldp{config: false}
			eventAction.enable_mpls_ldp(t)
		}
	}

	for _, op := range opts {
		if op.drop {
			in := gnmi.Get(t, a.ate, gnmi.OC().Flow(flow.Name()).Counters().InPkts().State())
			out := gnmi.Get(t, a.ate, gnmi.OC().Flow(flow.Name()).Counters().OutPkts().State())
			return uint64(out - in)
		}
	}
	return 0
}
