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
package twofiftysix_ucmp_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
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

func filterPacketReceived(t *testing.T, flow string, ate *ondatra.ATEDevice) map[string]float64 {
	t.Helper()

	flowPath := gnmi.OC().Flow(flow)
	filters := gnmi.GetAll(t, ate, flowPath.EgressTrackingAny().State())

	inPkts := map[string]uint64{}
	for _, f := range filters {
		inPkts[f.GetFilter()] = f.GetCounters().GetInPkts()
	}
	inPct := map[string]float64{}
	total := gnmi.Get(t, ate, flowPath.Counters().OutPkts().State())
	for k, v := range inPkts {
		inPct[k] = (float64(v) / float64(total)) * 100.0
	}
	return inPct
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
			dstIP := atePort7.ip(uint8(0))
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

	// Configure Ethernet+IPv4 headers.
	ethHeader := ondatra.NewEthernetHeader()
	ipv4Header := ondatra.NewIPv4Header()
	ipv4Header.WithSrcAddress(ipv4FlowIP)
	ipv4Header.WithDSCP(dscpEncapA1)
	ipv4Header.WithDstAddress(dstPfx)

	innerIpv4Header := ondatra.NewIPv4Header()
	innerIpv6Header := ondatra.NewIPv6Header()
	if count != 2 {
		innerIpv4Header.SrcAddressRange().WithMin(innersrcPfx).WithCount(uint32(*ciscoFlags.GRIBIScale)).WithStep("0.0.0.1")
		innerIpv4Header.DstAddressRange().WithMin(innerdst).WithCount(uint32(count)).WithStep("0.0.0.1")
	} else {
		innerIpv6Header.SrcAddressRange().WithMin("1::1").WithCount(uint32(60000)).WithStep("::1")
		innerIpv6Header.DstAddressRange().WithMin(ipv6EntryPrefix).WithCount(uint32(1)).WithStep("::1")
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

	if got > 0 {
		t.Errorf("LossPct for flow %s: got %g, want 0", flow.Name(), got)
	}
}

func testTrafficWeight(t *testing.T, ate *ondatra.ATEDevice, top *ondatra.ATETopology, count int, encap bool, val int) {

	allIntf := top.Interfaces()
	srcEndPoint := allIntf[atePort1.IPv4]
	dstEndPoints := []ondatra.Endpoint{}
	dstEndPoints2 := []ondatra.Endpoint{}

	if val == 8 {
		for i := uint32(1); i <= 2; i++ {
			dstIP := atePort5.ip(uint8(i))
			dstEndPoints = append(dstEndPoints, allIntf[dstIP])
		}
	} else {

		for i := uint32(1); i <= 15; i++ {
			dstIP := atePort2.ip(uint8(i))
			dstEndPoints = append(dstEndPoints, allIntf[dstIP])
		}
		for i := uint32(1); i <= 15; i++ {
			dstIP2 := atePort4.ip(uint8(i))
			dstEndPoints2 = append(dstEndPoints2, allIntf[dstIP2])
		}
	}

	var innerdst string
	var c int
	var c1 int
	tolerance := 0.5
	if !encap {
		innerdst = innerdstPfxMin_bgp
		c = 60000
		c1 = 5000
	} else {
		innerdst = "197.51.100.1"
		c = 15000
		c1 = 100
	}

	ethHeader := ondatra.NewEthernetHeader()
	ipv4Header := ondatra.NewIPv4Header()
	ipv4Header.SrcAddressRange().WithMin("1.1.1.111").WithCount(1).WithStep("0.0.0.1")
	ipv4Header.WithDSCP(dscpEncapA1)
	ipv4Header.DstAddressRange().WithMin(dstPfx).WithCount(uint32(c1)).WithStep("0.0.0.1")

	innerIpv4Header := ondatra.NewIPv4Header()
	innerIpv6Header := ondatra.NewIPv6Header()

	if count != 2 {
		innerIpv4Header.SrcAddressRange().WithMin(innersrcPfx).WithCount(uint32(70000)).WithStep("0.0.0.1")
		innerIpv4Header.DstAddressRange().WithMin(innerdst).WithCount(uint32(c)).WithStep("0.0.0.1")
	} else {
		innerIpv6Header.SrcAddressRange().WithMin("1::1").WithCount(uint32(60000)).WithStep("::1")
		innerIpv6Header.DstAddressRange().WithMin(ipv6EntryPrefix).WithCount(uint32(1)).WithStep("::1")
	}

	flow := ate.Traffic().NewFlow("flow").
		WithSrcEndpoints(srcEndPoint).
		WithDstEndpoints(dstEndPoints...)

	flow2 := ate.Traffic().NewFlow("flow2").
		WithSrcEndpoints(srcEndPoint).
		WithDstEndpoints(dstEndPoints2...)

	if count == 2 {
		flow = flow.WithHeaders(ethHeader, ipv4Header, innerIpv6Header).WithFrameSize(100)
		flow2 = flow2.WithHeaders(ethHeader, ipv4Header, innerIpv6Header).WithFrameSize(100)
	} else {
		flow = flow.WithHeaders(ethHeader, ipv4Header, innerIpv4Header).WithFrameSize(100)
		flow2 = flow2.WithHeaders(ethHeader, ipv4Header, innerIpv4Header).WithFrameSize(100)
	}
	// Offset for VlanID: ((6+6+4) * 8)-12 = 116.
	flow.EgressTracking().WithOffset(116).WithWidth(12).WithCount(15)
	flow2.EgressTracking().WithOffset(116).WithWidth(12).WithCount(15)

	var results map[string]float64
	var results2 map[string]float64
	if val != 7 {
		ate.Traffic().Start(t, flow)
		time.Sleep(2 * time.Minute)
		ate.Traffic().Stop(t)
		results = filterPacketReceived(t, "flow", ate)
	}
	if val != 8 {
		if val == 7 {
			args.interfaceaction(t, "port2", false)
		}
		ate.Traffic().Start(t, flow2)
		time.Sleep(2 * time.Minute)
		ate.Traffic().Stop(t)
		results2 = filterPacketReceived(t, "flow2", ate)
	}

	switch val {
	case 1:
		wantWeights := map[string]float64{
			"1": 0.13,
			"2": 1.14,
			"3": 1.43,
			"4": 0.17,
			"5": 1.48,
			"6": 1.84,
		}
		for i := 7; i <= 15; i = i + 3 {
			wantWeights[strconv.Itoa(i)] = 0.79
		}
		for i := 8; i <= 15; i = i + 3 {
			wantWeights[strconv.Itoa(i)] = 6.59
		}
		for i := 9; i <= 15; i = i + 3 {
			wantWeights[strconv.Itoa(i)] = 8.20
		}
		if diff := cmp.Diff(wantWeights, results, cmpopts.EquateApprox(0, tolerance)); diff != "" {
			t.Errorf("Packet distribution ratios -want,+got:\n%s", diff)
		}
		wantWeights = map[string]float64{
			"1": 0.79,
			"2": 6.59,
			"3": 8.20,
			"4": 0.79,
			"5": 6.59,
			"6": 8.20,
			"7": 0.4875,
		}
		for i := 8; i <= 15; i++ {
			wantWeights[strconv.Itoa(i)] = 1.88
		}
		if diff := cmp.Diff(wantWeights, results2, cmpopts.EquateApprox(0, tolerance)); diff != "" {
			t.Errorf("Packet distribution ratios -want,+got:\n%s", diff)
		}
	case 2:
		wantWeights := map[string]float64{
			"1": 0,
		}
		for i := 2; i <= 12; i++ {
			if i == 7 {
				wantWeights[strconv.Itoa(i)] = 0.298
			} else if i == 8 {
				wantWeights[strconv.Itoa(i)] = 2.483

			} else if i == 9 {
				wantWeights[strconv.Itoa(i)] = 3.079

			} else {
				wantWeights[strconv.Itoa(i)] = 0.0
			}
		}
		wantWeights = map[string]float64{
			"13": 1.25,
			"14": 10.428,
			"15": 12.93,
			"7":  0.298,
			"8":  2.483,
			"9":  3.079,
		}
		if diff := cmp.Diff(wantWeights, results, cmpopts.EquateApprox(0, tolerance)); diff != "" {
			t.Errorf("Packet distribution ratios -want,+got:\n%s", diff)
		}
		for i := 1; i <= 4; i = i + 3 {
			wantWeights[strconv.Itoa(i)] = 1.251
		}
		for i := 2; i <= 5; i = i + 3 {
			wantWeights[strconv.Itoa(i)] = 10.428
		}
		for i := 3; i <= 6; i = i + 3 {
			wantWeights[strconv.Itoa(i)] = 12.93
		}
		for i := 7; i <= 12; i++ {
			if i == 7 {
				wantWeights[strconv.Itoa(i)] = 0.635
			} else {
				wantWeights[strconv.Itoa(i)] = 2.46
			}
		}
		for i := 13; i <= 15; i++ {
			wantWeights[strconv.Itoa(i)] = 2.46
		}
		if diff := cmp.Diff(wantWeights, results2, cmpopts.EquateApprox(0.000, tolerance)); diff != "" {
			t.Errorf("Packet distribution ratios -want,+got:\n%s", diff)
		}

	case 3:
		wantWeights := map[string]float64{
			"1": 0,
		}
		for i := 2; i <= 6; i++ {
			wantWeights[strconv.Itoa(i)] = 0.0
		}
		wantWeights = map[string]float64{
			"7":  0.29,
			"8":  2.48,
			"9":  3.08,
			"10": 2.62,
			"11": 21.85,
			"12": 27.09,
			"13": 0.61,
			"14": 5.13,
			"15": 6.36,
		}
		if diff := cmp.Diff(wantWeights, results, cmpopts.EquateApprox(0.000, tolerance)); diff != "" {
			t.Errorf("Packet distribution ratios -want,+got:\n%s", diff)
		}
		wantWeights = map[string]float64{
			"1": 0.616,
			"2": 5.131,
			"3": 6.363,
			"4": 0.616,
			"5": 5.131,
			"6": 6.363,
			"7": 0.195,
		}
		for i := 8; i <= 15; i++ {
			wantWeights[strconv.Itoa(i)] = 0.757
		}
		if diff := cmp.Diff(wantWeights, results2, cmpopts.EquateApprox(0.000, tolerance)); diff != "" {
			t.Errorf("Packet distribution ratios -want,+got:\n%s", diff)
		}
	case 5:
		wantWeights := map[string]float64{
			"1": 0.298,
			"2": 2.483,
			"3": 3.079,
		}
		for i := 4; i <= 12; i++ {
			if i == 10 {
				wantWeights[strconv.Itoa(i)] = 2.622
			} else if i == 11 {
				wantWeights[strconv.Itoa(i)] = 21.85
			} else if i == 12 {
				wantWeights[strconv.Itoa(i)] = 27.09
			} else {
				wantWeights[strconv.Itoa(i)] = 0.0
			}
		}
		wantWeights = map[string]float64{
			"1":  0.298,
			"10": 2.622,
			"11": 21.85,
			"12": 27.09,
			"13": 0.616,
			"14": 5.131,
			"15": 6.36,
			"2":  2.483,
			"3":  3.079,
		}
		if diff := cmp.Diff(wantWeights, results, cmpopts.EquateApprox(0.000, tolerance)); diff != "" {
			t.Errorf("Packet distribution ratios -want,+got:\n%s", diff)
		}
		wantWeights = map[string]float64{
			"1": 0.616,
			"2": 5.131,
			"3": 6.363,
			"4": 0.616,
			"5": 5.131,
			"6": 6.363,
			"7": 0.195,
		}
		for i := 8; i <= 15; i++ {
			wantWeights[strconv.Itoa(i)] = 0.757
		}
		if diff := cmp.Diff(wantWeights, results2, cmpopts.EquateApprox(0.000, tolerance)); diff != "" {
			t.Errorf("Packet distribution ratios -want,+got:\n%s", diff)
		}
	case 6:
		wantWeights := map[string]float64{
			"1": 0.178,
			"2": 1.48,
			"3": 1.84,
			"4": 0.13,
			"5": 1.15,
			"6": 1.43,
		}
		for i := 7; i <= 15; i = i + 3 {
			wantWeights[strconv.Itoa(i)] = 0.79
		}
		for i := 8; i <= 15; i = i + 3 {
			wantWeights[strconv.Itoa(i)] = 6.61
		}
		for i := 9; i <= 15; i = i + 3 {
			wantWeights[strconv.Itoa(i)] = 8.20
		}
		if diff := cmp.Diff(wantWeights, results, cmpopts.EquateApprox(0.000, tolerance)); diff != "" {
			t.Errorf("Packet distribution ratios -want,+got:\n%s", diff)
		}
		wantWeights = map[string]float64{
			"1": 0.79,
			"2": 6.61,
			"3": 8.20,
			"4": 0.79,
			"5": 6.61,
			"6": 8.20,
			"7": 0.48,
		}
		for i := 8; i <= 15; i++ {
			wantWeights[strconv.Itoa(i)] = 1.89
		}
		if diff := cmp.Diff(wantWeights, results2, cmpopts.EquateApprox(0.000, tolerance)); diff != "" {
			t.Errorf("Packet distribution ratios -want,+got:\n%s", diff)
		}

	case 7:
		wantWeights := map[string]float64{
			"1": 1.69,
			"2": 14.12,
			"3": 17.5,
			"4": 1.69,
			"5": 14.12,
			"6": 17.5,
			"7": 1.04,
		}
		for i := 8; i <= 15; i++ {
			wantWeights[strconv.Itoa(i)] = 4.03
		}
		if diff := cmp.Diff(wantWeights, results2, cmpopts.EquateApprox(0.000, tolerance)); diff != "" {
			t.Errorf("Packet distribution ratios -want,+got:\n%s", diff)
		}
	case 8:
		wantWeights := map[string]float64{
			"1": 0.0,
			"2": 99.32,
		}
		if diff := cmp.Diff(wantWeights, results, cmpopts.EquateApprox(0.000, tolerance)); diff != "" {
			t.Errorf("Packet distribution ratios -want,+got:\n%s", diff)
		}
	}
	t.Logf("Filters: %v ....%v", results, results2)
}
