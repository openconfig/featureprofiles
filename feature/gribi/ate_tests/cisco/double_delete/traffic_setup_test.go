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

package double_delete_test

import (
	"testing"
	"time"

	ciscoFlags "github.com/openconfig/featureprofiles/internal/cisco/flags"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
)

type TGNoptions struct {
	SrcIP    string
	DstIP    string
	SrcIf    string
	Scalenum int
	Ifname   string
}

// configureATE configures port1, port2 and port3 on the ATE.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) *ondatra.ATETopology {
	t.Helper()

	top := ate.Topology().New()

	p1 := ate.Port(t, "port1")
	i1 := top.AddInterface(atePort1.Name).WithPort(p1)
	i1.IPv4().
		WithAddress(atePort1.IPv4CIDR()).
		WithDefaultGateway(dutPort1.IPv4)
	i1.IPv6().
		WithAddress(atePort1.IPv6CIDR()).
		WithDefaultGateway(dutPort1.IPv6)

	p2 := ate.Port(t, "port2")
	i2 := top.AddInterface(atePort2.Name).WithPort(p2)
	i2.IPv4().
		WithAddress(atePort2.IPv4CIDR()).
		WithDefaultGateway(dutPort2.IPv4)
	i2.IPv6().
		WithAddress(atePort2.IPv6CIDR()).
		WithDefaultGateway(dutPort2.IPv6)

	p3 := ate.Port(t, "port3")
	i3 := top.AddInterface(atePort3.Name).WithPort(p3)
	i3.IPv4().
		WithAddress(atePort3.IPv4CIDR()).
		WithDefaultGateway(dutPort3.IPv4)
	i3.IPv6().
		WithAddress(atePort3.IPv6CIDR()).
		WithDefaultGateway(dutPort3.IPv6)

	p4 := ate.Port(t, "port4")
	i4 := top.AddInterface(atePort4.Name).WithPort(p4)
	i4.IPv4().
		WithAddress(atePort4.IPv4CIDR()).
		WithDefaultGateway(dutPort4.IPv4)
	i4.IPv6().
		WithAddress(atePort4.IPv6CIDR()).
		WithDefaultGateway(dutPort4.IPv6)

	p5 := ate.Port(t, "port5")
	i5 := top.AddInterface(atePort5.Name).WithPort(p5)
	i5.IPv4().
		WithAddress(atePort5.IPv4CIDR()).
		WithDefaultGateway(dutPort5.IPv4)
	i5.IPv6().
		WithAddress(atePort5.IPv6CIDR()).
		WithDefaultGateway(dutPort5.IPv6)

	p6 := ate.Port(t, "port6")
	i6 := top.AddInterface(atePort6.Name).WithPort(p6)
	i6.IPv4().
		WithAddress(atePort6.IPv4CIDR()).
		WithDefaultGateway(dutPort6.IPv4)
	i6.IPv6().
		WithAddress(atePort6.IPv6CIDR()).
		WithDefaultGateway(dutPort6.IPv6)

	p7 := ate.Port(t, "port7")
	i7 := top.AddInterface(atePort7.Name).WithPort(p7)
	i7.IPv4().
		WithAddress(atePort7.IPv4CIDR()).
		WithDefaultGateway(dutPort7.IPv4)
	i7.IPv6().
		WithAddress(atePort7.IPv6CIDR()).
		WithDefaultGateway(dutPort7.IPv6)

	p8 := ate.Port(t, "port8")
	i8 := top.AddInterface(atePort8.Name).WithPort(p8)
	i8.IPv4().
		WithAddress(atePort8.IPv4CIDR()).
		WithDefaultGateway(dutPort8.IPv4)
	i8.IPv6().
		WithAddress(atePort8.IPv6CIDR()).
		WithDefaultGateway(dutPort8.IPv6)
	return top
}

// addAteISISL2 configures ISIS L2 ATE config
func addAteISISL2(t *testing.T, topo *ondatra.ATETopology, atePort, areaId, network_name string, metric uint32, v4prefix string, count uint32) {
	t.Helper()

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
	t.Helper()

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
	t.Helper()

	// addAteISISL2(t, top, "atePort8", "B4", "isis_network", 20, innerdstPfxMin_isis+"/"+mask, uint32(innerdstPfxCount_isis))
	// addAteEBGPPeer(t, top, "atePort8", dutPort8.IPv4, 64001, "bgp_network", atePort8.IPv4, innerdstPfxMin_bgp+"/"+mask, innerdstPfxCount_bgp, false)

	//advertising 100.100.100.100/32 for bgp resolve over IGP prefix
	intfs := top.Interfaces()
	intfs["atePort8"].WithIPv4Loopback("100.100.100.100/32")
	if innerdstPfxCount_isis > uint32(*ciscoFlags.GRIBIScale) || innerdstPfxCount_bgp > uint32(*ciscoFlags.GRIBIScale) {
		addAteISISL2(t, top, "atePort8", "B4", "isis_network", 20, innerdstPfxMin_isis+"/"+mask, innerdstPfxCount_isis)
		addAteEBGPPeer(t, top, "atePort8", dutPort8.IPv4, 64001, "bgp_recursive", atePort8.IPv4, innerdstPfxMin_bgp+"/"+mask, innerdstPfxCount_bgp, true)
	} else {
		addAteISISL2(t, top, "atePort8", "B4", "isis_network", 20, innerdstPfxMin_isis+"/"+mask, uint32(*ciscoFlags.GRIBIScale))
		addAteEBGPPeer(t, top, "atePort8", dutPort8.IPv4, 64001, "bgp_recursive", atePort8.IPv4, innerdstPfxMin_bgp+"/"+mask, uint32(*ciscoFlags.GRIBIScale), true)
	}
	top.Push(t).StartProtocols(t)
}

// createFlow returns a flow from atePort1 to the dstPfx, expected to arrive on ATE interface dst.
func (a *testArgs) createFlow(name string, srcEndPoint *ondatra.Interface, dstEndPoint []ondatra.Endpoint, innerdstPfxMin string, innerdstPfxCount uint32, opts ...*TGNoptions) *ondatra.Flow {
	hdr := ondatra.NewIPv4Header()
	if len(opts) != 0 {
		for _, opt := range opts {
			hdr.WithSrcAddress(opt.SrcIP).DstAddressRange().WithMin(opt.DstIP).WithCount(uint32(opt.Scalenum)).WithStep("0.0.0.1")
		}
	} else {
		hdr.WithSrcAddress(dutPort1.IPv4).DstAddressRange().WithMin(dstPfxMin).WithCount(uint32(*ciscoFlags.GRIBIScale)).WithStep("0.0.0.1")
	}

	innerIpv4Header := ondatra.NewIPv4Header()
	innerIpv4Header.WithSrcAddress(innersrcPfx)
	if innerdstPfxCount > uint32(*ciscoFlags.GRIBIScale) {
		innerIpv4Header.DstAddressRange().WithMin(innerdstPfxMin).WithCount(innerdstPfxCount).WithStep("0.0.0.1")
	} else {
		innerIpv4Header.DstAddressRange().WithMin(innerdstPfxMin).WithCount(uint32(*ciscoFlags.GRIBIScale)).WithStep("0.0.0.1")
	}
	flow := a.ate.Traffic().NewFlow(name).
		WithSrcEndpoints(srcEndPoint).
		WithDstEndpoints(dstEndPoint...).
		WithHeaders(ondatra.NewEthernetHeader(), hdr, innerIpv4Header).WithFrameRateFPS(10).WithFrameSize(300)

	return flow
}

// allFlows designs all the flows needed for the backup testing
func (a *testArgs) allFlows(t *testing.T, opts ...*TGNoptions) []*ondatra.Flow {
	t.Helper()

	srcEndPoint := a.top.Interfaces()[atePort1.Name]
	if len(opts) != 0 {
		for _, opt := range opts {
			if opt.SrcIf != "" {
				srcEndPoint = a.top.Interfaces()[opt.SrcIf]
			}
		}
	}
	dstEndPoint := []ondatra.Endpoint{}
	if len(opts) != 0 {
		for _, opt := range opts {
			if opt.SrcIf != "" {
				for intf, intf_data := range a.top.Interfaces() {
					if intf != opt.SrcIf {
						dstEndPoint = append(dstEndPoint, intf_data)
					}
				}
			}
		}
	} else {
		for intf, intf_data := range a.top.Interfaces() {
			if intf != "atePort1" {
				dstEndPoint = append(dstEndPoint, intf_data)
			}
		}
	}
	flows := []*ondatra.Flow{}

	if len(opts) != 0 {
		for _, opt := range opts {
			bgp_flow := a.createFlow("BaseFlow_BGP", srcEndPoint, dstEndPoint, innerdstPfxMin_bgp, innerdstPfxCount_bgp, &TGNoptions{SrcIP: opt.SrcIP, DstIP: opt.DstIP, Scalenum: opt.Scalenum})
			isis_flow := a.createFlow("BaseFlow_ISIS", srcEndPoint, dstEndPoint, innerdstPfxMin_isis, innerdstPfxCount_isis, &TGNoptions{SrcIP: opt.SrcIP, DstIP: opt.DstIP, Scalenum: opt.Scalenum})
			flows = append(flows, bgp_flow, isis_flow)
		}
	} else {
		bgp_flow := a.createFlow("BaseFlow_BGP", srcEndPoint, dstEndPoint, innerdstPfxMin_bgp, innerdstPfxCount_bgp)
		isis_flow := a.createFlow("BaseFlow_ISIS", srcEndPoint, dstEndPoint, innerdstPfxMin_isis, innerdstPfxCount_isis)
		//flows := []*ondatra.Flow{}
		flows = append(flows, bgp_flow, isis_flow)
	}
	return flows
}

// validateTrafficFlows validates traffic loss on tgn side and DUT incoming and outgoing counters
func (a *testArgs) validateTrafficFlows(t *testing.T, flows []*ondatra.Flow, drop bool, d_port []string, opts ...*TGNoptions) {
	t.Helper()

	src_port := gnmi.OC().Interface("Bundle-Ether120")
	if len(opts) != 0 {
		for _, opt := range opts {
			if opt.Ifname != "" {
				src_port = gnmi.OC().Interface(opt.Ifname)
			}
		}
	}
	subintf1 := src_port.Subinterface(0)
	dutOutPktsBeforeTraffic := map[string]uint64{"ipv4": gnmi.Get(t, a.dut, subintf1.Ipv4().Counters().InPkts().State())}

	dutInPktsBeforeTraffic := make(map[string][]uint64)
	for _, dp := range d_port {
		dst_port := gnmi.OC().Interface(dp)
		subintf2 := dst_port.Subinterface(0)
		dutInPktsBeforeTraffic["ipv4"] = append(dutInPktsBeforeTraffic["ipv4"], gnmi.Get(t, a.dut, subintf2.Ipv4().Counters().OutPkts().State()))
	}
	//aggregriate dst_port counter
	totalInPktsBeforeTraffic := map[string]uint64{"ipv4": 0}
	for _, data := range dutInPktsBeforeTraffic["ipv4"] {
		totalInPktsBeforeTraffic["ipv4"] = totalInPktsBeforeTraffic["ipv4"] + uint64(data)
	}

	a.ate.Traffic().Start(t, flows...)
	time.Sleep(60 * time.Second)
	a.ate.Traffic().Stop(t)

	// sleeping while DUT interface counters are updated
	time.Sleep(20 * time.Second)

	for _, f := range flows {
		ateTxPkts := map[string]uint64{"ipv4": gnmi.Get(t, a.ate, gnmi.OC().Flow(f.Name()).Counters().OutPkts().State())}
		ateRxPkts := map[string]uint64{"ipv4": gnmi.Get(t, a.ate, gnmi.OC().Flow(f.Name()).Counters().InPkts().State())}

		flowPath := gnmi.OC().Flow(f.Name())
		got := gnmi.Get(t, a.ate, flowPath.LossPct().State())
		if drop {
			if got == 0 {
				t.Log("No stats collected as interfaces are down due to LC reload")
				break
			} else if got != 100 {
				t.Errorf("Traffic passing for flow %s got %g, want 100 percent loss", f.Name(), got)
			}
		} else {
			if got > 0 {
				t.Errorf("LossPct for flow %s got %g, want 0", f.Name(), got)
			}
		}

		if !drop {
			dutOutPktsAfterTraffic := map[string]uint64{"ipv4": gnmi.Get(t, a.dut, subintf1.Ipv4().Counters().InPkts().State())}
			dutInPktsAfterTraffic := make(map[string][]uint64)
			for _, dp := range d_port {
				dst_port := gnmi.OC().Interface(dp)
				subintf2 := dst_port.Subinterface(0)
				dutInPktsAfterTraffic["ipv4"] = append(dutInPktsAfterTraffic["ipv4"], gnmi.Get(t, a.dut, subintf2.Ipv4().Counters().OutPkts().State()))
			}

			//aggregriate dst_port counter
			totalInPktsAfterTraffic := map[string]uint64{"ipv4": 0}
			for _, data := range dutInPktsAfterTraffic["ipv4"] {
				totalInPktsAfterTraffic["ipv4"] = totalInPktsAfterTraffic["ipv4"] + uint64(data)
			}

			for k := range dutInPktsAfterTraffic {
				if got, want := totalInPktsAfterTraffic[k]-totalInPktsBeforeTraffic[k], ateTxPkts[k]; got <= want {
					t.Errorf("Get less inPkts from telemetry: got %v, want >= %v", got, want)
				}
				if got, want := dutOutPktsAfterTraffic[k]-dutOutPktsBeforeTraffic[k], ateRxPkts[k]; got <= want {
					t.Errorf("Get less outPkts from telemetry: got %v, want >= %v", got, want)
				}
			}
		}
	}
}
