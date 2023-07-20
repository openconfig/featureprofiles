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

package ha_test

import (
	"testing"
	"time"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
)

const (
	innersrcPfx         = "200.1.0.1"
	innerdstPfxMin_bgp  = "202.1.0.1"
	innerdstPfxMin_isis = "201.1.0.1"
)

// TGNoptions are optional parameters to a validate traffic function.
type TGNoptions struct {
	burst                    bool
	tolerance                float32
	wait                     int
	start_after_verification bool
	SrcIP                    string
	LCOIR                    string
}

// configureATE configures port1, port2 and port3 on the ATE.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) *ondatra.ATETopology {
	top := ate.Topology().New()

	p1 := ate.Port(t, "port1")
	i1 := top.AddInterface(atePort1.Name).WithPort(p1)
	i1.IPv4().
		WithAddress(atePort1.IPv4CIDR()).
		WithDefaultGateway(dutPort1.IPv4)
	// i1.IPv6().
	// 	WithAddress(atePort1.IPv6CIDR()).
	// 	WithDefaultGateway(dutPort1.IPv6)

	p2 := ate.Port(t, "port2")
	i2 := top.AddInterface(atePort2.Name).WithPort(p2)
	i2.IPv4().
		WithAddress(atePort2.IPv4CIDR()).
		WithDefaultGateway(dutPort2.IPv4)
	// i2.IPv6().
	// 	WithAddress(atePort2.IPv6CIDR()).
	// 	WithDefaultGateway(dutPort2.IPv6)

	p3 := ate.Port(t, "port3")
	i3 := top.AddInterface(atePort3.Name).WithPort(p3)
	i3.IPv4().
		WithAddress(atePort3.IPv4CIDR()).
		WithDefaultGateway(dutPort3.IPv4)
	// i3.IPv6().
	// 	WithAddress(atePort3.IPv6CIDR()).
	// 	WithDefaultGateway(dutPort3.IPv6)

	p4 := ate.Port(t, "port4")
	i4 := top.AddInterface(atePort4.Name).WithPort(p4)
	i4.IPv4().
		WithAddress(atePort4.IPv4CIDR()).
		WithDefaultGateway(dutPort4.IPv4)
	// i4.IPv6().
	// 	WithAddress(atePort4.IPv6CIDR()).
	// 	WithDefaultGateway(dutPort4.IPv6)

	p5 := ate.Port(t, "port5")
	i5 := top.AddInterface(atePort5.Name).WithPort(p5)
	i5.IPv4().
		WithAddress(atePort5.IPv4CIDR()).
		WithDefaultGateway(dutPort5.IPv4)
	// i5.IPv6().
	// 	WithAddress(atePort5.IPv6CIDR()).
	// 	WithDefaultGateway(dutPort5.IPv6)

	p6 := ate.Port(t, "port6")
	i6 := top.AddInterface(atePort6.Name).WithPort(p6)
	i6.IPv4().
		WithAddress(atePort6.IPv4CIDR()).
		WithDefaultGateway(dutPort6.IPv4)
	// i6.IPv6().
	// 	WithAddress(atePort6.IPv6CIDR()).
	// 	WithDefaultGateway(dutPort6.IPv6)

	p7 := ate.Port(t, "port7")
	i7 := top.AddInterface(atePort7.Name).WithPort(p7)
	i7.IPv4().
		WithAddress(atePort7.IPv4CIDR()).
		WithDefaultGateway(dutPort7.IPv4)
	// i7.IPv6().
	// 	WithAddress(atePort7.IPv6CIDR()).
	// 	WithDefaultGateway(dutPort7.IPv6)

	p8 := ate.Port(t, "port8")
	i8 := top.AddInterface(atePort8.Name).WithPort(p8)
	i8.IPv4().
		WithAddress(atePort8.IPv4CIDR()).
		WithDefaultGateway(dutPort8.IPv4)
	// i8.IPv6().
	// 	WithAddress(atePort8.IPv6CIDR()).
	// 	WithDefaultGateway(dutPort8.IPv6)
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
	intfs["atePort8"].WithIPv4Loopback("100.100.100.100/32")
	if isisPfx != 0 || bgpPfx != 0 {
		addAteISISL2(t, top, "atePort8", "B4", "isis_network", 20, innerdstPfxMin_isis+"/"+mask, isisPfx)
		addAteEBGPPeer(t, top, "atePort8", dutPort8.IPv4, 64001, "bgp_recursive", atePort8.IPv4, innerdstPfxMin_bgp+"/"+mask, bgpPfx, true)
	} else {
		addAteISISL2(t, top, "atePort8", "B4", "isis_network", 20, innerdstPfxMin_isis+"/"+mask, gribi_Scale)
		addAteEBGPPeer(t, top, "atePort8", dutPort8.IPv4, 64001, "bgp_recursive", atePort8.IPv4, innerdstPfxMin_bgp+"/"+mask, gribi_Scale, true)
	}
	top.Push(t).StartProtocols(t)
}

// createFlow returns a flow from atePort1 to the dstPfx, expected to arrive on ATE interface dst.
func (a *testArgs) createFlow(name string, srcEndPoint *ondatra.Interface, dstEndPoint []ondatra.Endpoint, innerdstPfxMin string, innerdstPfxCount uint32, opts ...*TGNoptions) *ondatra.Flow {
	hdr := ondatra.NewIPv4Header()
	if len(opts) != 0 {
		for _, opt := range opts {
			if opt.SrcIP != "" {
				if gribi_Scale < nhg_Scale_REPAIR {
					hdr.WithSrcAddress(opt.SrcIP).DstAddressRange().WithMin(dstPfxMin).WithCount(uint32(gribi_Scale)).WithStep("0.0.0.1")
				} else {
					hdr.WithSrcAddress(opt.SrcIP).DstAddressRange().WithMin(dstPfxMin).WithCount(uint32(nhg_Scale_REPAIR)).WithStep("0.0.0.1")

				}
			}
		}
	} else {
		hdr.WithSrcAddress(dutPort1.IPv4).DstAddressRange().WithMin("198.51.100.0").WithCount(uint32(gribi_Scale)).WithStep("0.0.0.1")
	}

	innerIpv4Header := ondatra.NewIPv4Header()
	innerIpv4Header.WithSrcAddress(innersrcPfx)
	if innerdstPfxCount > uint32(gribi_Scale) {
		innerIpv4Header.DstAddressRange().WithMin(innerdstPfxMin).WithCount(innerdstPfxCount).WithStep("0.0.0.1")
	} else {
		innerIpv4Header.DstAddressRange().WithMin(innerdstPfxMin).WithCount(uint32(gribi_Scale)).WithStep("0.0.0.1")
	}
	flow := a.ate.Traffic().NewFlow(name).
		WithSrcEndpoints(srcEndPoint).
		WithDstEndpoints(dstEndPoint...).
		WithHeaders(ondatra.NewEthernetHeader(), hdr, innerIpv4Header).WithFrameRateFPS(1000000).WithFrameSize(300)

	return flow
}

// allFlows designs all the flows needed for the backup testing
func (a *testArgs) allFlows(t *testing.T, opts ...*TGNoptions) []*ondatra.Flow {
	srcEndPoint := a.top.Interfaces()[atePort1.Name]
	dstEndPoint := []ondatra.Endpoint{}
	flows := []*ondatra.Flow{}
	for intf, intf_data := range a.top.Interfaces() {
		if intf != "atePort1" {
			dstEndPoint = append(dstEndPoint, intf_data)
		}
	}
	if len(opts) != 0 {
		for _, opt := range opts {
			if opt.SrcIP != "" {
				// src_ip_isis_flow := a.createFlow("Src_ip_isis_flow", srcEndPoint, dstEndPoint, innerdstPfxMin_isis, innerdstPfxCount_isis, &TGNoptions{SrcIP: opt.SrcIP})
				src_ip_bgp_flow := a.createFlow("Src_ip_bgp_flow", srcEndPoint, dstEndPoint, innerdstPfxMin_bgp, innerdstPfxCount_bgp, &TGNoptions{SrcIP: opt.SrcIP})
				// flows = append(flows, src_ip_bgp_flow, src_ip_isis_flow)
				flows = append(flows, src_ip_bgp_flow)
			}
		}
	} else {
		bgp_flow := a.createFlow("BaseFlow_BGP", srcEndPoint, dstEndPoint, innerdstPfxMin_bgp, innerdstPfxCount_bgp)
		// isis_flow := a.createFlow("BaseFlow_ISIS", srcEndPoint, dstEndPoint, innerdstPfxMin_isis, innerdstPfxCount_isis)
		// flows = append(flows, bgp_flow, isis_flow)
		flows = append(flows, bgp_flow)
	}
	return flows
}

// validateTrafficFlows validates traffic loss on tgn side and DUT incoming and outgoing counters
func (a *testArgs) validateTrafficFlows(t *testing.T, flows []*ondatra.Flow, drop bool, d_port map[string][]string, opts ...*TGNoptions) {
	ateTxInit := make(map[string]uint64)
	ateTxFin := make(map[string]uint64)
	ateRxInit := make(map[string]uint64)
	ateRxFin := make(map[string]uint64)

	src_port := gnmi.OC().Interface("Bundle-Ether120")
	subintf1 := src_port.Subinterface(0)
	dutSrcInitTraffic := map[string]uint64{"ipv4": gnmi.Get(t, a.dut, subintf1.Ipv4().Counters().InPkts().State())}

	dutDstInitTraffic := make(map[string][]uint64)
	var checked_intf_b4 []string
	for _, dp_list := range d_port {
		for _, dest := range dp_list {
			flag := false
			if len(checked_intf_b4) != 0 {
				for _, elem := range checked_intf_b4 {
					if elem == dest {
						flag = true
					}
				}
			}
			if flag {
				break
			}
			checked_intf_b4 = append(checked_intf_b4, dest)
			dst_port := gnmi.OC().Interface(dest)
			subintf2 := dst_port.Subinterface(0)
			dutDstInitTraffic["ipv4"] = append(dutDstInitTraffic["ipv4"], gnmi.Get(t, a.dut, subintf2.Ipv4().Counters().OutPkts().State()))
		}
	}
	//aggregriate dst_port counter
	totalDstInitTraffic := map[string]uint64{"ipv4": 0}
	for _, data := range dutDstInitTraffic["ipv4"] {
		totalDstInitTraffic["ipv4"] = totalDstInitTraffic["ipv4"] + uint64(data)
	}

	for _, opt := range opts {
		if opt.burst {
			// run traffic for 120 seconds and check stats
			a.ate.Traffic().Start(t, flows...)
			time.Sleep(120 * time.Second)
			ateTxInit["ipv4"] = 0
			ateRxInit["ipv4"] = 0
		} else {
			for _, f := range flows {
				ateTxInit = map[string]uint64{"ipv4": gnmi.Get(t, a.ate, gnmi.OC().Flow(f.Name()).Counters().OutPkts().State())}
				ateRxInit = map[string]uint64{"ipv4": gnmi.Get(t, a.ate, gnmi.OC().Flow(f.Name()).Counters().InPkts().State())}
			}
			if opt.wait != 0 {
				// assuming traffic is running and we wait for user provided time
				time.Sleep(time.Duration(opt.wait) * time.Second)
			} else {
				// assuming traffic is running and we sleep for 60seconds
				time.Sleep(60 * time.Second)
			}
		}
		a.ate.Traffic().Stop(t)
	}

	for _, f := range flows {
		ateTxFin = map[string]uint64{"ipv4": gnmi.Get(t, a.ate, gnmi.OC().Flow(f.Name()).Counters().OutPkts().State())}
		ateRxFin = map[string]uint64{"ipv4": gnmi.Get(t, a.ate, gnmi.OC().Flow(f.Name()).Counters().InPkts().State())}

		flowPath := gnmi.OC().Flow(f.Name())
		got := gnmi.Get(t, a.ate, flowPath.LossPct().State())
		if drop {
			if got != 100 {
				t.Errorf("Traffic passing for flow %s got %g, want 100 percent loss", f.Name(), got)
			}
		} else {
			if len(opts) != 0 {
				for _, opt := range opts {
					if got > opt.tolerance {
						t.Errorf("LossPct for flow %s got %g, want 0", f.Name(), got)
					}
				}
			} else if got > 0 {
				t.Errorf("LossPct for flow %s got %g, want 0", f.Name(), got)
			}
		}
	}
	time.Sleep(time.Minute)
	dutSrcFinTraffic := map[string]uint64{"ipv4": gnmi.Get(t, a.dut, subintf1.Ipv4().Counters().InPkts().State())}
	dutDstFinTraffic := make(map[string][]uint64)
	var checked_intf_after []string
	for _, dp_list := range d_port {
		for _, dest := range dp_list {
			flag := false
			if len(checked_intf_after) != 0 {
				for _, elem := range checked_intf_after {
					if elem == dest {
						flag = true
					}
				}
			}
			if flag {
				break
			}
			checked_intf_after = append(checked_intf_after, dest)
			dst_port := gnmi.OC().Interface(dest)
			subintf2 := dst_port.Subinterface(0)
			dutDstFinTraffic["ipv4"] = append(dutDstFinTraffic["ipv4"], gnmi.Get(t, a.dut, subintf2.Ipv4().Counters().OutPkts().State()))
		}
	}
	//aggregriate dst_port counter
	totalDstFinTraffic := map[string]uint64{"ipv4": 0}
	for _, data := range dutDstFinTraffic["ipv4"] {
		totalDstFinTraffic["ipv4"] = totalDstFinTraffic["ipv4"] + uint64(data)
	}
	for k := range dutDstFinTraffic {
		if got, want := totalDstFinTraffic[k]-totalDstInitTraffic[k], ateRxFin[k]-ateRxInit[k]; got <= want {
			t.Errorf("Get less inPkts from telemetry: got %v, want >= %v", got, want)
		}
		if got, want := dutSrcFinTraffic[k]-dutSrcInitTraffic[k], ateTxFin[k]-ateTxInit[k]; got <= want {
			t.Errorf("Get less outPkts from telemetry: got %v, want >= %v", got, want)
		}
	}
	for _, opt := range opts {
		// if start after verfication of existing flow
		if opt.start_after_verification {
			a.ate.Traffic().Start(t, flows...)
			return
		}
	}
}

// // validateTrafficFlows validates traffic loss on tgn side and DUT incoming and outgoing counters
// func (a *testArgs) new(t *testing.T, flows []*ondatra.Flow, drop bool, d_port map[string][]string, opts ...*TGNoptions) {

// 	stats := make(map[string]map[string]map[string]map[string]uint64)

// 	// DUT source/destination interface accounting before traffic

// 	src_port := gnmi.OC().Interface("Bundle-Ether120")
// 	subintf1 := src_port.Subinterface(0)
// 	stats["DUT"]["ALL"]["TxS"]["IPv4"] = subintf1.Ipv4().Counters().InPkts().Get(t)

// 	dutDstInitTraffic := make(map[string][]uint64)
// 	var checked_intf_b4 []string
// 	for _, dp_list := range d_port {
// 		for _, dest := range dp_list {
// 			flag := false
// 			if len(checked_intf_b4) != 0 {
// 				for _, elem := range checked_intf_b4 {
// 					if elem == dest {
// 						flag = true
// 					}
// 				}
// 			}
// 			if flag {
// 				break
// 			}
// 			checked_intf_b4 = append(checked_intf_b4, dest)
// 			dst_port := gnmi.OC().Interface(dest)
// 			subintf2 := dst_port.Subinterface(0)
// 			dutDstInitTraffic["ipv4"] = append(dutDstInitTraffic["ipv4"], subintf2.Ipv4().Counters().OutPkts().Get(t))
// 		}
// 	}
// 	totalDstInitTraffic := map[string]uint64{"ipv4": 0}
// 	for _, data := range dutDstInitTraffic["ipv4"] {
// 		totalDstInitTraffic["ipv4"] = totalDstInitTraffic["ipv4"] + uint64(data)
// 	}
// 	stats["DUT"]["ALL"]["RxS"]["IPv4"] = totalDstInitTraffic["ipv4"]

// 	// Running traffic
// 	for _, opt := range opts {
// 		if opt.burst {
// 			for _, f := range flows {
// 				stats["ATE"][f.Name()]["TxS"]["IPv4"] = 0
// 				stats["ATE"][f.Name()]["RxS"]["IPv4"] = 0
// 			}
// 			// run traffic for 120 seconds and check stats
// 			a.ate.Traffic().Start(t, flows...)
// 			time.Sleep(120 * time.Second)
// 		} else {
// 			for _, f := range flows {
// 				stats["ATE"][f.Name()]["TxS"]["IPv4"] = gnmi.OC().Flow(f.Name()).Counters().OutPkts().Get(t)
// 				stats["ATE"][f.Name()]["RxS"]["IPv4"] = gnmi.OC().Flow(f.Name()).Counters().InPkts().Get(t)
// 			}
// 			if opt.wait != 0 {
// 				// assuming traffic is running and we wait for user provided time
// 				time.Sleep(time.Duration(opt.wait) * time.Second)
// 			} else {
// 				// assuming traffic is running and we sleep for 60seconds
// 				time.Sleep(60 * time.Second)
// 			}
// 		}
// 		a.ate.Traffic().Stop(t)
// 	}

// 	// ATE traffic stats after traffic
// 	for _, f := range flows {
// 		stats["ATE"][f.Name()]["TxF"]["IPv4"] = a.ate.Telemetry().Flow(f.Name()).Counters().OutPkts().Get(t)
// 		stats["ATE"][f.Name()]["RxF"]["IPv4"] = a.ate.Telemetry().Flow(f.Name()).Counters().InPkts().Get(t)

// 		flowPath := gnmi.OC().Flow(f.Name())
// 		got := flowPath.LossPct().Get(t)
// 		if drop {
// 			if got < 50 {
// 				t.Errorf("Traffic passing for flow %s got %g, want 100 percent loss", f.Name(), got)
// 			}
// 		} else {
// 			if len(opts) != 0 {
// 				for _, opt := range opts {
// 					if got > opt.tolerance {
// 						t.Errorf("LossPct for flow %s got %g, want 0", f.Name(), got)
// 					}
// 				}
// 			} else if got > 0 {
// 				t.Errorf("LossPct for flow %s got %g, want 0", f.Name(), got)
// 			}
// 		}
// 	}

// 	// DUT accounting after traffic

// 	stats["DUT"]["ALL"]["TxF"]["IPv4"] = subintf1.Ipv4().Counters().InPkts().Get(t)
// 	dutDstFinTraffic := make(map[string][]uint64)
// 	var checked_intf_after []string
// 	for _, dp_list := range d_port {
// 		for _, dest := range dp_list {
// 			flag := false
// 			if len(checked_intf_after) != 0 {
// 				for _, elem := range checked_intf_after {
// 					if elem == dest {
// 						flag = true
// 					}
// 				}
// 			}
// 			if flag {
// 				break
// 			}
// 			checked_intf_after = append(checked_intf_after, dest)
// 			dst_port := a.dut.Telemetry().Interface(dest)
// 			subintf2 := dst_port.Subinterface(0)
// 			dutDstFinTraffic["ipv4"] = append(dutDstFinTraffic["ipv4"], subintf2.Ipv4().Counters().OutPkts().Get(t))
// 		}
// 	}
// 	//aggregriate dst_port counter
// 	totalDstFinTraffic := map[string]uint64{"ipv4": 0}
// 	for _, data := range dutDstFinTraffic["ipv4"] {
// 		totalDstFinTraffic["ipv4"] = totalDstFinTraffic["ipv4"] + uint64(data)
// 	}
// 	stats["DUT"]["ALL"]["RxF"]["IPv4"] = totalDstFinTraffic["ipv4"]
// 	for _, opt := range opts {
// 		// if start after verfication of existing flow
// 		if opt.start_after_verification {
// 			a.ate.Traffic().Start(t, flows...)
// 			return
// 		}
// 	}
// }
