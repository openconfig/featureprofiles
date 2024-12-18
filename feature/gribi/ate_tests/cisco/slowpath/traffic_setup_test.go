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
package slowpath_test

import (
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

// configureATE configures port1, port2 and port3 on the ATE.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) *ondatra.ATETopology {
	t.Helper()

	top := ate.Topology().New()
	atePort1.ConfigureSubATE(t, top, ate)
	atePort2.ConfigureSubATE(t, top, ate)
	atePort3.ConfigureSubATE(t, top, ate)
	atePort4.ConfigureSubATE(t, top, ate)
	atePort5.ConfigureSubATE(t, top, ate)
	atePort6.ConfigureSubATE(t, top, ate)
	atePort8.ConfigureSubATE(t, top, ate)

	p7 := ate.Port(t, "port7")
	i7 := top.AddInterface(atePort7.Name).WithPort(p7)
	i7.IPv4().
		WithAddress(atePort7.IPv4CIDR()).
		WithDefaultGateway(dutPort7.IPv4)
	i7.IPv6().
		WithAddress(atePort7.IPv6CIDR()).
		WithDefaultGateway(dutPort7.IPv6)
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

func testTrafficmin(t *testing.T, ate *ondatra.ATEDevice, top *ondatra.ATETopology, count int, encap bool, case4 bool, case5 bool) {

	allIntf := top.Interfaces()

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
	ethHeader := ondatra.NewEthernetHeader()
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
	ate.Traffic().Start(t, flow)
	time.Sleep(2 * time.Minute)
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
}
