// Copyright 2021 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package egressp4rt_test

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
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

type Countoptions struct {
	portin  []string
	portinp []string
	tcp     bool
	udp     bool
	tcpd    uint16
	tcps    uint16
	udps    uint16
	udpd    uint16
}

// configureATE configures port1, port2 and port3 on the ATE.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) *ondatra.ATETopology {
	t.Helper()

	top := ate.Topology().New()
	//atePort1.ConfigureSubATE(t, top, ate)
	//atePort2.ConfigureSubATE(t, top, ate)
	//atePort3.ConfigureSubATE(t, top, ate)
	//atePort4.ConfigureSubATE(t, top, ate)
	//atePort5.ConfigureSubATE(t, top, ate)
	//atePort6.ConfigureSubATE(t, top, ate)
	//atePort8.ConfigureSubATE(t, top, ate)

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
	i3.Ethernet()

	p4 := ate.Port(t, "port4")
	i4 := top.AddInterface(atePort4.Name).WithPort(p4)
	i4.Ethernet()

	p5 := ate.Port(t, "port5")
	i5 := top.AddInterface(atePort5.Name).WithPort(p5)
	i5.IPv4().
		WithAddress(atePort5.IPv4CIDR()).
		WithDefaultGateway(dutPort5.IPv4)
	i5.IPv6().
		WithAddress(atePort5.IPv6CIDR()).
		WithDefaultGateway(dutPort5.IPv6)

	p7 := ate.Port(t, "port7")
	i7 := top.AddInterface(atePort7.Name).WithPort(p7)
	i7.IPv4().
		WithAddress(atePort7.IPv4CIDR()).
		WithDefaultGateway(dutPort7.IPv4)
	i7.IPv6().
		WithAddress(atePort7.IPv6CIDR()).
		WithDefaultGateway(dutPort7.IPv6)

	p6 := ate.Port(t, "port6")
	i6 := top.AddInterface(atePort6.Name).WithPort(p6)
	i6.IPv4().
		WithAddress(atePort6.IPv4CIDR()).
		WithDefaultGateway(dutPort6.IPv4)
	i6.IPv6().
		WithAddress(atePort6.IPv6CIDR()).
		WithDefaultGateway(dutPort6.IPv6)

	p8 := ate.Port(t, "port8")
	i8 := top.AddInterface(atePort8.Name).WithPort(p8)
	i8.Ethernet()

	return top

}
func cidr(ipv4 string, ones int) string {
	return ipv4 + "/" + strconv.Itoa(ones)
}

func (a *attributes) ConfigureSubATE(t *testing.T, top *ondatra.ATETopology, ate *ondatra.ATEDevice) {
	t.Helper()
	p := ate.Port(t, a.Name)
	t.Log(atePort7.Attributes.Name + "xxxx")
	// Configure source port on ATE : Port1.
	if a.numSubIntf == 0 {
		ip := a.ip(0)
		gateway := a.gateway(0)
		intf := top.AddInterface(ip).WithPort(p)
		intf.IPv4().WithAddress(cidr(ip, 30))
		intf.IPv4().WithDefaultGateway(gateway)
		t.Logf("Adding ATE Ipv4 address: %s with gateway: %s", cidr(ip, 30), gateway)
	}
	// Configure destination port on ATE : Port2.
	for i := uint32(1); i <= a.numSubIntf; i++ {
		ip := a.ip(uint8(i))
		gateway := a.gateway(uint8(i))
		intf := top.AddInterface(ip).WithPort(p)
		intf.IPv4().WithAddress(cidr(ip, 30))
		intf.IPv4().WithDefaultGateway(gateway)
		intf.Ethernet().WithVLANID(uint16(i))
		t.Logf("Adding ATE Ipv4 address: %s with gateway: %s and VlanID: %d", cidr(ip, 30), gateway, i)
	}
}

// addAteISISL2 configures ISIS L2 ATE config
func addAteISISL2(t *testing.T, topo *ondatra.ATETopology, atePort, areaId, network_name string, metric uint32, v4prefix string, count uint32) {
	t.Helper()

	intfs := topo.Interfaces()
	if len(intfs) == 0 {
		t.Fatal("There are no interfaces in the Topology")
	}
	t.Log(atePort + "xxx")
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

	intfs := top.Interfaces()
	intfs["port7"].WithIPv4Loopback("100.100.100.100/32")
	if innerdstPfxCount_isis > uint32(*ciscoFlags.GRIBIScale) || innerdstPfxCount_bgp > uint32(*ciscoFlags.GRIBIScale) {
		addAteISISL2(t, top, "port7", "B4", "isis_network", 20, innerdstPfxMin_isis+"/"+mask, innerdstPfxCount_isis)
		addAteEBGPPeer(t, top, "port7", dutPort7.IPv4, 64001, "bgp_recursive", atePort7.IPv4, innerdstPfxMin_bgp+"/"+mask, innerdstPfxCount_bgp, true)
	} else {
		addAteISISL2(t, top, "port7", "B4", "isis_network", 20, innerdstPfxMin_isis+"/"+mask, uint32(*ciscoFlags.GRIBIScale))
		addAteEBGPPeer(t, top, "port7", dutPort7.IPv4, 64001, "bgp_recursive", "port7", innerdstPfxMin_bgp+"/"+mask, uint32(*ciscoFlags.GRIBIScale), true)
	}
	top.Push(t).StartProtocols(t)
}

func testTraffic(t *testing.T, ate *ondatra.ATEDevice, top *ondatra.ATETopology, count int, encap bool, inDst, inSrc, outDst, outSrc string, case4, case5, portval bool, opts ...*Countoptions) (string, int) {

	allIntf := top.Interfaces()
	dut := ondatra.DUT(t, "dut")

	// ATE source endpoint.
	srcEndPoint := allIntf[atePort1.IPv4]

	// ATE destination endpoints.
	dstEndPoints := []ondatra.Endpoint{}
	if case4 && case5 {
		for i := uint32(1); i <= 2; i++ {
			dstIP := atePort3.ip(uint8(i))
			dstEndPoints = append(dstEndPoints, allIntf[dstIP])
		}
		args.interfaceaction(t, "port2", false)
		args.interfaceaction(t, "port4", false)
	} else if case4 {
		for i := uint32(1); i <= 2; i++ {
			dstIP := atePort5.ip(uint8(i))
			dstEndPoints = append(dstEndPoints, allIntf[dstIP])
		}
		args.interfaceaction(t, "port2", false)
		args.interfaceaction(t, "port4", false)

	} else if case5 {
		if count == 2 {
			dstIP := atePort7.ip(0)
			dstEndPoints = append(dstEndPoints, allIntf[dstIP])

		} else {
			for i := uint32(1); i < 2; i++ {
				dstIP := atePort3.ip(uint8(i))
				dstEndPoints = append(dstEndPoints, allIntf[dstIP])
			}
		}
		args.interfaceaction(t, "port5", false)

	} else {
		for i := uint32(1); i <= 15; i++ {
			dstIP := atePort2.ip(uint8(i))
			dstEndPoints = append(dstEndPoints, allIntf[dstIP])
		}
		for i := uint32(1); i <= 15; i++ {
			dstIP := atePort4.ip(uint8(i))
			dstEndPoints = append(dstEndPoints, allIntf[dstIP])
		}
	}
	var innerdst string
	if !encap {
		innerdst = innerdstPfxMin_bgp
	} else {
		innerdst = "197.51.100.1"
	}
	var ttl, ttl2 int
	if count == 5 || count == 6 {
		ttl = 1
		ttl2 = 1
	} else if count == 4 || count == 7 {
		ttl = 1
		ttl2 = 50
	} else {
		ttl = 64
	}
	// Configure Ethernet+IPv4 headers.
	ethHeader := ondatra.NewEthernetHeader().WithSrcAddress(tracerouteSrcMAC)
	ipv4Header := ondatra.NewIPv4Header()
	ipv4Header.WithSrcAddress(ipv4FlowIP)
	ipv4Header.WithDSCP(dscpEncapA1)
	ipv4Header.WithDstAddress(dstPfx)
	ipv4Header.WithTTL(uint8(ttl))

	innerIpv4Header := ondatra.NewIPv4Header()
	innerIpv6Header := ondatra.NewIPv6Header()
	if count != 2 {
		if count == 5 || count == 4 {
			innerIpv4Header.SrcAddressRange().WithMin(innersrcPfx).WithCount(uint32(*ciscoFlags.GRIBIScale)).WithStep("0.0.0.1")
			innerIpv4Header.DstAddressRange().WithMin(innerdst).WithCount(uint32(count)).WithStep("0.0.0.1")
			innerIpv4Header.WithTTL(uint8(ttl2))
		} else {
			innerIpv4Header.SrcAddressRange().WithMin(innersrcPfx).WithCount(uint32(*ciscoFlags.GRIBIScale)).WithStep("0.0.0.1")
			innerIpv4Header.DstAddressRange().WithMin(innerdst).WithCount(uint32(500)).WithStep("0.0.0.1")
			innerIpv4Header.WithTTL(uint8(ttl2))
		}
	} else if count == 2 || count == 6 || count == 7 {
		innerIpv6Header.SrcAddressRange().WithMin("1::1").WithCount(uint32(60000)).WithStep("::1")
		innerIpv6Header.DstAddressRange().WithMin(ipv6EntryPrefix).WithCount(uint32(1)).WithStep("::1")
		innerIpv6Header.WithHopLimit(uint8(ttl2))
	}

	flow := ate.Traffic().NewFlow("flow").
		WithSrcEndpoints(srcEndPoint).
		WithDstEndpoints(dstEndPoints...)

	if count == 2 {
		flow = flow.WithHeaders(ethHeader, ipv4Header, innerIpv6Header).WithFrameRateFPS(10).WithFrameSize(300)
	} else {
		flow = flow.WithHeaders(ethHeader, ipv4Header, innerIpv4Header).WithFrameRateFPS(10).WithFrameSize(300)
	}
	dports := make(map[string]uint64)

	//dportsf := make(map[string][]string)
	dportsf := make(map[string]uint64)

	cliHandle := dut.RawAPIs().CLI(t)

	if !(count >= 4 && count <= 7) {
		if len(opts) != 0 {
			for _, opt := range opts {
				for _, p := range opt.portin {
					cmd := fmt.Sprintf("show interface %s", p)
					output, _ := cliHandle.RunCommand(context.Background(), cmd)
					re := regexp.MustCompile(`(\d+)\spackets output`)
					match := re.FindStringSubmatch(output.Output())
					val, _ := strconv.Atoi(match[1])
					dports[p] = uint64(val)
					fmt.Println("portt&val")
					fmt.Println(p)
					fmt.Println(val)
				}

			}
		}
	}
	ate.Traffic().Start(t, flow)
	time.Sleep(20000 * time.Minute)
	ate.Traffic().Stop(t)
	flowPath := gnmi.OC().Flow(flow.Name())
	got := gnmi.Get(t, args.ate, flowPath.LossPct().State())
	if count == 4 || count == 5 || count == 6 || count == 7 {
		if got != 100 {
			t.Errorf("Traffic passing for flow %s got %g, want 100 percent loss", flow.Name(), got)
		}
	} else {
		if got > 0 {
			t.Errorf("LossPct for flow %s: got %g, want 0", flow.Name(), got)
		}
	}
	var portid string
	if !(count >= 4 && count <= 7) {

		if len(opts) != 0 {
			for _, opt := range opts {
				for _, p := range opt.portin {
					cmd := fmt.Sprintf("show interface %s", p)
					fmt.Println("in loop222")

					output, _ := cliHandle.RunCommand(context.Background(), cmd)
					re := regexp.MustCompile(`(\d+)\spackets output`)
					match := re.FindStringSubmatch(output.Output())
					val, _ := strconv.Atoi(match[1])
					dportsf[p] = uint64(val)
					fmt.Println("dstportt&val")
					fmt.Println(p)
					fmt.Println(val)

					if dportsf[p]-dports[p] == 36000 {
						portid = p
						fmt.Println("in looop2")
						fmt.Println(portid)
					}
				}
			}
		}
	}
	//outPkts := gnmi.GetAll(t, ate.OTG(), gnmi.OTG().FlowAny().Counters().OutPkts().State())
	outPkts := gnmi.GetAll(t, args.ate, gnmi.OC().FlowAny().Counters().OutPkts().State())

	total := 0

	for _, count := range outPkts {
		total += int(count)
	}

	return portid, total
}

func testTrafficc(t *testing.T, ate *ondatra.ATEDevice, top *ondatra.ATETopology, count int, encap, portval, sourceport bool, ttl, ttl2 int, inDst, inSrc, outDst, outSrc string, opts ...*Countoptions) (string, string, int) {

	//allIntf := top.Interfaces()
	dut := ondatra.DUT(t, "dut")
	var intfName, intfs string
	// ATE source endpoint.
	//srcEndPoint := allIntf[atePort1.IPv4]
	if sourceport {
		intfName = atePort1.Name
		intfs = "atePort1"
	} else {
		intfName = atePort3.Name
		intfs = "atePort3"
	}
	srcEndPoint := top.Interfaces()[intfName]

	// ATE destination endpoints.
	dstEndPoints := []ondatra.Endpoint{}
	// dstEndPoints = append(dstEndPoints, allIntf[atePort6.IPv4])
	// dstEndPoints = append(dstEndPoints, allIntf[atePort8.IPv4])
	//dstEndPoints := a.top.Interfaces()[atePort1.Name]

	for intf, intf_data := range top.Interfaces() {
		if intf != intfs {
			dstEndPoints = append(dstEndPoints, intf_data)
		}
	}
	var innerdst string
	if !encap {
		innerdst = innerdstPfxMin_bgp
	} else {
		innerdst = inDst
	}
	var fps int
	// if count == 6 || count == 22 {
	// 	fps = 5
	// } else {
	// 	fps = 300
	// }
	if ttl == 1 {
		fps = 5
	} else {
		fps = 300
	}
	// Configure Ethernet+IPv4 headers.
	// outSrc = "150.150.238.42"
	// outDst = "198.51.100.71"
	// inSrc = "153.153.173.133"
	// inDst = "197.51.48.243"
	ethHeader := ondatra.NewEthernetHeader().WithSrcAddress(tracerouteSrcMAC)
	ipv4Header := ondatra.NewIPv4Header()
	ipv4Header.WithSrcAddress(outSrc)
	ipv4Header.WithDSCP(dscpEncapA1)
	ipv4Header.WithDstAddress(outDst)
	ipv4Header.WithTTL(uint8(ttl))

	innerIpv4Header := ondatra.NewIPv4Header()
	innerIpv6Header := ondatra.NewIPv6Header()

	tcpHeader := ondatra.NewTCPHeader()
	udpHeader := ondatra.NewUDPHeader()

	if count == 5 || count == 6 {
		innerIpv4Header.SrcAddressRange().WithMin(inSrc).WithCount(1).WithStep("0.0.0.1")
		innerIpv4Header.DstAddressRange().WithMin(innerdst).WithCount(1).WithStep("0.0.0.1")
		innerIpv4Header.WithTTL(uint8(ttl2))
		innerIpv4Header.WithDSCP(8)

	} else if count == 2 || count == 22 {
		innerIpv6Header.SrcAddressRange().WithMin(inSrc).WithCount(1).WithStep("::1")
		innerIpv6Header.DstAddressRange().WithMin(inDst).WithCount(uint32(1)).WithStep("::1")
		//innerIpv6Header.DstAddressRange().WithMin("2555:1::").WithCount(uint32(1)).WithStep("::1")

		innerIpv6Header.WithHopLimit(uint8(ttl2))
		innerIpv6Header.WithDSCP(8)
	}

	flow := ate.Traffic().NewFlow("flow").
		WithSrcEndpoints(srcEndPoint).
		WithDstEndpoints(dstEndPoints...)

	if outDst != inDst {
		if count == 2 || count == 22 {
			flow = flow.WithHeaders(ethHeader, ipv4Header, innerIpv6Header).WithFrameRateFPS(uint64(fps)).WithFrameSize(300)
			if len(opts) != 0 {
				for _, opt := range opts {
					if opt.tcp {
						tcpHeader.WithDstPort(opt.tcpd)
						tcpHeader.WithSrcPort(opt.tcps)
						flow = flow.WithHeaders(ethHeader, ipv4Header, innerIpv6Header, tcpHeader).WithFrameRateFPS(uint64(fps)).WithFrameSize(300)
					} else if opt.udp {
						udpHeader.WithSrcPort(opt.udps)
						udpHeader.WithDstPort(opt.udpd)
						flow = flow.WithHeaders(ethHeader, ipv4Header, innerIpv6Header, udpHeader).WithFrameRateFPS(uint64(fps)).WithFrameSize(300)
					}
				}
			}
		} else {
			flow = flow.WithHeaders(ethHeader, ipv4Header, innerIpv4Header).WithFrameRateFPS(uint64(fps)).WithFrameSize(300)
			if len(opts) != 0 {
				for _, opt := range opts {
					if opt.tcp {
						tcpHeader.WithDstPort(opt.tcpd)
						tcpHeader.WithSrcPort(opt.tcps)

						flow = flow.WithHeaders(ethHeader, ipv4Header, innerIpv4Header, tcpHeader).WithFrameRateFPS(uint64(fps)).WithFrameSize(300)
					} else if opt.udp {
						udpHeader.WithSrcPort(opt.udps)
						udpHeader.WithDstPort(opt.udpd)
						flow = flow.WithHeaders(ethHeader, ipv4Header, innerIpv4Header, udpHeader).WithFrameRateFPS(uint64(fps)).WithFrameSize(300)
					}
				}
			}
		}
	} else {
		if count == 2 || count == 22 {
			if count == 22 {
				innerIpv6Header.WithHopLimit(1)
			}
			innerIpv6Header.WithDSCP(dscpEncapA1)
			flow = flow.WithHeaders(ethHeader, innerIpv6Header).WithFrameRateFPS(uint64(fps)).WithFrameSize(300)
			if len(opts) != 0 {
				for _, opt := range opts {
					if opt.tcp {
						tcpHeader.WithDstPort(opt.tcpd)
						tcpHeader.WithSrcPort(opt.tcps)

						flow = flow.WithHeaders(ethHeader, innerIpv6Header, tcpHeader).WithFrameRateFPS(uint64(fps)).WithFrameSize(300)
					} else if opt.udp {
						udpHeader.WithSrcPort(opt.udps)
						udpHeader.WithDstPort(opt.udpd)
						flow = flow.WithHeaders(ethHeader, innerIpv6Header, udpHeader).WithFrameRateFPS(uint64(fps)).WithFrameSize(300)
					}
				}
			}
		} else {
			if count == 6 {
				innerIpv4Header.WithTTL(1)
			}
			innerIpv4Header.WithDSCP(dscpEncapA1)

			flow = flow.WithHeaders(ethHeader, innerIpv4Header).WithFrameRateFPS(uint64(fps)).WithFrameSize(300)
			if len(opts) != 0 {
				for _, opt := range opts {
					if opt.tcp {
						tcpHeader.WithDstPort(opt.tcpd)
						tcpHeader.WithSrcPort(opt.tcps)

						flow = flow.WithHeaders(ethHeader, innerIpv4Header, tcpHeader).WithFrameRateFPS(uint64(fps)).WithFrameSize(300)
					} else if opt.udp {
						udpHeader.WithSrcPort(opt.udps)
						udpHeader.WithDstPort(opt.udpd)
						flow = flow.WithHeaders(ethHeader, innerIpv4Header, udpHeader).WithFrameRateFPS(uint64(fps)).WithFrameSize(300)
					}
				}
			}
		}
	}
	dports := make(map[string]uint64)
	dportsinp := make(map[string]uint64)

	//dportsf := make(map[string][]string)
	dportsf := make(map[string]uint64)
	dportsfin := make(map[string]uint64)

	cliHandle := dut.RawAPIs().CLI(t)

	if portval {
		if len(opts) != 0 {
			for _, opt := range opts {
				for _, p := range opt.portin {
					fmt.Println("poorttt")
					fmt.Println(p)

					cmd := fmt.Sprintf("show interface %s", p)
					output, _ := cliHandle.RunCommand(context.Background(), cmd)
					re := regexp.MustCompile(`(\d+)\spackets output`)
					match := re.FindStringSubmatch(output.Output())
					fmt.Println("matchhhhhh")
					fmt.Println(match)
					val, _ := strconv.Atoi(match[1])
					dports[p] = uint64(val)
					fmt.Println("sssportt&val")
					fmt.Println(p)
					fmt.Println(val)
				}

			}
		}

		for _, opt := range opts {
			for _, pi := range opt.portinp {
				fmt.Println("poorttt")
				fmt.Println(pi)

				cmd := fmt.Sprintf("show interface %s", pi)
				output, _ := cliHandle.RunCommand(context.Background(), cmd)
				re := regexp.MustCompile(`(\d+)\spackets input`)
				match := re.FindStringSubmatch(output.Output())
				fmt.Println("matchhhhhh")
				fmt.Println(match)
				val, _ := strconv.Atoi(match[1])
				dportsinp[pi] = uint64(val)
				fmt.Println("sssportt&val")
				fmt.Println(pi)
				fmt.Println(val)
			}
		}
	}

	ate.Traffic().Start(t, flow)
	if ttl == 1 {
		fmt.Println("ttttttlllll")
		time.Sleep(10 * time.Second)
		//time.Sleep(20 * time.Minute)

	} else {
		time.Sleep(2 * time.Minute)
	}
	ate.Traffic().Stop(t)
	flowPath := gnmi.OC().Flow(flow.Name())
	got := gnmi.Get(t, args.ate, flowPath.LossPct().State())
	fmt.Println("gooott")
	fmt.Println(got)
	//if count == 6 || count == 22 {
	if ttl == 1 {

		if got != 100 {
			t.Errorf("Traffic passing for flow %s got %g, want 100 percent loss", flow.Name(), got)
		}
	} else {
		if got > 0 {
			t.Errorf("LossPct for flow %s: got %g, want 0", flow.Name(), got)
		}
	}
	var portid, portidin string
	if portval {
		time.Sleep(30 * time.Second)
		if len(opts) != 0 {
			for _, opt := range opts {
				for _, p := range opt.portin {
					cmd := fmt.Sprintf("show interface %s", p)
					fmt.Println("in loop222")

					output, _ := cliHandle.RunCommand(context.Background(), cmd)
					re := regexp.MustCompile(`(\d+)\spackets output`)
					match := re.FindStringSubmatch(output.Output())
					val, _ := strconv.Atoi(match[1])
					if val == 0 {
						val = 10
					}
					dportsf[p] = uint64(val)
					fmt.Println("dstportt&val")
					fmt.Println(p)
					fmt.Println(val)

					if dportsf[p]-dports[p] >= 30000 {
						portid = p
						fmt.Println("in looop2")
						fmt.Println(portid)
						fmt.Println(dportsf[p])
						fmt.Println("before traffic")
						fmt.Println(dports[p])
					}
				}
			}
			for _, opt := range opts {
				for _, pi := range opt.portinp {
					cmd := fmt.Sprintf("show interface %s", pi)
					fmt.Println("in loop222src")

					output, _ := cliHandle.RunCommand(context.Background(), cmd)
					re := regexp.MustCompile(`(\d+)\spackets input`)
					match := re.FindStringSubmatch(output.Output())
					val, _ := strconv.Atoi(match[1])
					if val == 0 {
						val = 10
					}
					dportsfin[pi] = uint64(val)
					fmt.Println("dstportt&val")
					fmt.Println(pi)
					fmt.Println(val)

					if dportsfin[pi]-dportsinp[pi] >= 30000 {
						portidin = pi
						fmt.Println("in looop2")
						fmt.Println(portidin)
					}
				}
			}
		}
	}
	//outPkts := gnmi.GetAll(t, ate.OTG(), gnmi.OTG().FlowAny().Counters().OutPkts().State())
	outPkts := gnmi.GetAll(t, args.ate, gnmi.OC().FlowAny().Counters().OutPkts().State())

	total := 0

	for _, count := range outPkts {
		total += int(count)
	}

	return portid, portidin, total
}
