package qos_test

import (
	"strings"
	"testing"
	"time"

	//"fmt"

	ciscoFlags "github.com/openconfig/featureprofiles/internal/cisco/flags"
	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
)

type trafficData struct {
	trafficRate float64
	frameSize   uint32
	dscp        uint8
	queue       string
	srcmac      string
	dstmac      string
	srcendpoint *ondatra.Interface
}

const (
	dstPfx                = "198.51.100.1"
	mask                  = "32"
	dstPfxMin             = "198.51.100.1"
	dstPfx1               = "11.1.1.1"
	dstPfxCount1          = 10
	innersrcPfx           = "200.1.0.1"
	innerdstPfxMin_bgp    = "202.1.0.1"
	innerdstPfxCount_bgp  = 10
	innerdstPfxMin_isis   = "201.1.0.1"
	innerdstPfxCount_isis = 10
)

func testTraffic(t *testing.T, expectPass bool, ate *ondatra.ATEDevice, top *ondatra.ATETopology, srcEndPoint *ondatra.Interface, allPorts map[string]*ondatra.Interface, scale int, hostIP string, args *testArgs, dscp uint8, weights ...float64) {
	dscpList := []uint8{1, 9, 17, 25, 33, 41, 49}
	ondatraFlowList := []*ondatra.Flow{}
	for _, dscp := range dscpList {

		ethHeader := ondatra.NewEthernetHeader()
		// ethHeader.WithSrcAddress("00:11:01:00:00:01")
		// ethHeader.WithDstAddress("00:01:00:02:00:00")

		ipv4Header := ondatra.NewIPv4Header()
		ipv4Header.SrcAddressRange().
			WithMin("198.51.100.0").
			WithMax("198.51.100.254").
			WithCount(250)
		ipv4Header.WithDSCP(dscp)
		ipv4Header.DstAddressRange().WithMin(hostIP).WithCount(uint32(scale)).WithStep("0.0.0.1")

		innerIpv4Header := ondatra.NewIPv4Header()
		innerIpv4Header.WithSrcAddress("200.1.0.2")
		innerIpv4Header.DstAddressRange().WithMin("201.1.0.2").WithCount(10000).WithStep("0.0.0.1")
		dstEndPoint := []ondatra.Endpoint{}

		for _, v := range allPorts {
			if *v != *srcEndPoint {
				dstEndPoint = append(dstEndPoint, v)
			}
		}

		flow := ate.Traffic().NewFlow("Flow").
			WithSrcEndpoints(srcEndPoint).
			WithDstEndpoints(dstEndPoint...)

		flow.WithFrameSize(300).WithFrameRateFPS(100).WithHeaders(ethHeader, ipv4Header, innerIpv4Header)
		ondatraFlowList = append(ondatraFlowList, flow)
	}

	ate.Traffic().Start(t, ondatraFlowList...)
	time.Sleep(60 * time.Second)
	threshold := 0.90
	stats := gnmi.GetAll(t, ate, gnmi.OC().InterfaceAny().Counters().State())
	trafficPass := util.CheckTrafficPassViaPortPktCounter(stats, threshold)

	if trafficPass == expectPass {
		t.Log("Traffic works as expected")
	} else {
		t.Error("Traffic doesn't work as expected")
	}
	//for _, trflow := range ondatraFlowList {
	//	flowstats := ate.Telemetry().Flow(trflow.Name()).Counters().Get(t)

	//	fmt.Println("number of out packets in flow is", flowstats.OutPkts)

	//}

	// if expectPass {
	// 	tolerance := float64(0.03)
	// 	interval := 45 * time.Second
	// 	if len(weights) > 0 {
	// 		CheckDUTTrafficViaInterfaceTelemetry(t, args.dut, args.interfaces.in, args.interfaces.out[:len(weights)], weights, interval, tolerance)
	// 	}
	// }
	ate.Traffic().Stop(t)

	time.Sleep(60 * time.Second)
	//flowstats:= ate.Telemetry().FlowAny().Counters().Get(t)
	//for _, s  := range flowstats {
	//       fmt.Println("number of out packets in flow is",*s.OutPkts)

}
func testTrafficsreaming(t *testing.T, expectPass bool, ate *ondatra.ATEDevice, top *ondatra.ATETopology, srcEndPoint *ondatra.Interface, allPorts map[string]*ondatra.Interface, scale int, hostIP string, args *testArgs, dscp uint8, weights ...float64) {
	dscpList := []uint8{1, 9, 17, 25, 33, 41, 49}
	ondatraFlowList := []*ondatra.Flow{}
	for _, dscp := range dscpList {

		ethHeader := ondatra.NewEthernetHeader()
		// ethHeader.WithSrcAddress("00:11:01:00:00:01")
		// ethHeader.WithDstAddress("00:01:00:02:00:00")

		ipv4Header := ondatra.NewIPv4Header()
		ipv4Header.SrcAddressRange().
			WithMin("198.51.100.0").
			WithMax("198.51.100.254").
			WithCount(250)
		ipv4Header.WithDSCP(dscp)
		ipv4Header.DstAddressRange().WithMin(hostIP).WithCount(uint32(scale)).WithStep("0.0.0.1")

		innerIpv4Header := ondatra.NewIPv4Header()
		innerIpv4Header.WithSrcAddress("200.1.0.2")
		innerIpv4Header.DstAddressRange().WithMin("201.1.0.2").WithCount(10000).WithStep("0.0.0.1")
		dstEndPoint := []ondatra.Endpoint{}

		for _, v := range allPorts {
			if *v != *srcEndPoint {
				dstEndPoint = append(dstEndPoint, v)
			}
		}

		flow := ate.Traffic().NewFlow("Flow").
			WithSrcEndpoints(srcEndPoint).
			WithDstEndpoints(dstEndPoint...)

		flow.WithFrameSize(300).WithFrameRateFPS(1000).WithHeaders(ethHeader, ipv4Header, innerIpv4Header)
		ondatraFlowList = append(ondatraFlowList, flow)
	}

	ate.Traffic().Start(t, ondatraFlowList...)

	time.Sleep(60 * time.Second)
	threshold := 0.90
	stats := gnmi.GetAll(t, ate, gnmi.OC().InterfaceAny().Counters().State())
	trafficPass := util.CheckTrafficPassViaPortPktCounter(stats, threshold)

	if trafficPass == expectPass {
		t.Log("Traffic works as expected")
	} else {
		t.Error("Traffic doesn't work as expected")
	}

}
func testTrafficipv6(t *testing.T, expectPass bool, ate *ondatra.ATEDevice, top *ondatra.ATETopology, srcEndPoint *ondatra.Interface, dstEndPoint *ondatra.Interface, scale int, hostIP string, args *testArgs, dscp uint8) {
	dscpList := []uint8{1, 9, 17, 25, 33, 41, 49}
	ondatraFlowList := []*ondatra.Flow{}
	for _, dscp := range dscpList {

		ethHeader := ondatra.NewEthernetHeader()
		ethHeader.WithSrcAddress("00:11:01:00:00:01")
		ethHeader.WithDstAddress("00:01:00:02:00:00")

		ipv6Header := ondatra.NewIPv6Header()
		ipv6Header.WithSrcAddress("2000::100:120:1:2")
		ipv6Header.WithDSCP(dscp)
		ipv6Header.WithDstAddress("2000::100:121:1:2")
		flow := ate.Traffic().NewFlow("Flow").
			WithSrcEndpoints(srcEndPoint).
			WithDstEndpoints(dstEndPoint)
		flow.WithFrameSize(300).WithFrameRateFPS(100).WithHeaders(ethHeader, ipv6Header)
		ondatraFlowList = append(ondatraFlowList, flow)
	}
	ate.Traffic().Start(t, ondatraFlowList...)
	time.Sleep(60 * time.Second)
	threshold := 0.90
	stats := gnmi.GetAll(t, ate, gnmi.OC().InterfaceAny().Counters().State())
	trafficPass := util.CheckTrafficPassViaPortPktCounter(stats, threshold)

	if trafficPass == expectPass {
		t.Log("Traffic works as expected")
	} else {
		t.Error("Traffic doesn't work as expected")
	}

	ate.Traffic().Stop(t)

	time.Sleep(time.Minute)
}
func testTrafficqos(t *testing.T, expectPass bool, ate *ondatra.ATEDevice, top *ondatra.ATETopology, srcEndPoints []*ondatra.Interface, dstEndPoint *ondatra.Interface, scale int, hostIP string, args *testArgs, dscp uint8, weights ...float64) {

	ondatraFlowList := []*ondatra.Flow{}
	dstmacaddress := []string{"00:01:00:03:00:00", "00:01:00:04:00:00"}
	srcmacaddress := []string{"00:16:01:00:00:01", "00:17:01:00:00:01"}
	//var trafficFlows map[string]*trafficData
	trafficFlows := map[string]*trafficData{

		"flow1-tc7": {frameSize: 1000, trafficRate: 45, dscp: 56, queue: "tc7", srcmac: srcmacaddress[0], dstmac: dstmacaddress[0], srcendpoint: srcEndPoints[0]},
		"flow1-tc6": {frameSize: 1000, trafficRate: 35, dscp: 48, queue: "tc6", srcmac: srcmacaddress[0], dstmac: dstmacaddress[0], srcendpoint: srcEndPoints[0]},
		"flow1-tc5": {frameSize: 1000, trafficRate: 2, dscp: 33, queue: "tc5", srcmac: srcmacaddress[0], dstmac: dstmacaddress[0], srcendpoint: srcEndPoints[0]},
		"flow1-tc4": {frameSize: 1000, trafficRate: 2, dscp: 25, queue: "tc4", srcmac: srcmacaddress[0], dstmac: dstmacaddress[0], srcendpoint: srcEndPoints[0]},
		"flow1-tc3": {frameSize: 1000, trafficRate: 2, dscp: 17, queue: "tc3", srcmac: srcmacaddress[0], dstmac: dstmacaddress[0], srcendpoint: srcEndPoints[0]},
		"flow1-tc2": {frameSize: 1000, trafficRate: 2, dscp: 9, queue: "tc2", srcmac: srcmacaddress[0], dstmac: dstmacaddress[0], srcendpoint: srcEndPoints[0]},
		"flow1-tc1": {frameSize: 1000, trafficRate: 1, dscp: 1, queue: "tc1", srcmac: srcmacaddress[0], dstmac: dstmacaddress[0], srcendpoint: srcEndPoints[0]},
		"flow2-tc7": {frameSize: 1000, trafficRate: 45, dscp: 56, queue: "tc7", srcmac: srcmacaddress[1], dstmac: dstmacaddress[1], srcendpoint: srcEndPoints[1]},
		"flow2-tc6": {frameSize: 1000, trafficRate: 35, dscp: 48, queue: "tc6", srcmac: srcmacaddress[1], dstmac: dstmacaddress[1], srcendpoint: srcEndPoints[1]},
		"flow2-tc5": {frameSize: 1000, trafficRate: 2, dscp: 33, queue: "tc5", srcmac: srcmacaddress[1], dstmac: dstmacaddress[1], srcendpoint: srcEndPoints[1]},
		"flow2-tc4": {frameSize: 1000, trafficRate: 2, dscp: 25, queue: "tc4", srcmac: srcmacaddress[1], dstmac: dstmacaddress[1], srcendpoint: srcEndPoints[1]},
		"flow2-tc3": {frameSize: 1000, trafficRate: 2, dscp: 17, queue: "tc3", srcmac: srcmacaddress[1], dstmac: dstmacaddress[1], srcendpoint: srcEndPoints[1]},
		"flow2-tc2": {frameSize: 1000, trafficRate: 2, dscp: 9, queue: "tc2", srcmac: srcmacaddress[1], dstmac: dstmacaddress[1], srcendpoint: srcEndPoints[1]},
		"flow2-tc1": {frameSize: 1000, trafficRate: 1, dscp: 1, queue: "tc1", srcmac: srcmacaddress[1], dstmac: dstmacaddress[1], srcendpoint: srcEndPoints[1]},
	}

	for trafficID, data := range trafficFlows {

		ethHeader := ondatra.NewEthernetHeader()
		ethHeader.WithSrcAddress(data.srcmac)
		ethHeader.WithDstAddress(data.dstmac)

		ipv4Header := ondatra.NewIPv4Header()
		ipv4Header.SrcAddressRange().
			WithMin("198.51.100.0").
			WithMax("198.51.100.254").
			WithCount(250)
		ipv4Header.WithDSCP(data.dscp)
		ipv4Header.DstAddressRange().WithMin(hostIP).WithCount(uint32(scale)).WithStep("0.0.0.1")

		innerIpv4Header := ondatra.NewIPv4Header()
		innerIpv4Header.WithSrcAddress("200.1.0.2")
		innerIpv4Header.DstAddressRange().WithMin("201.1.0.2").WithCount(10000).WithStep("0.0.0.1")

		flow := ate.Traffic().NewFlow(trafficID).
			WithSrcEndpoints(data.srcendpoint).
			WithDstEndpoints(dstEndPoint)

		flow.WithFrameSize(300).WithFrameRatePct(data.trafficRate).WithHeaders(ethHeader, ipv4Header, innerIpv4Header)
		ondatraFlowList = append(ondatraFlowList, flow)
	}

	ate.Traffic().Start(t, ondatraFlowList...)
	time.Sleep(60 * time.Second)
	tc7flows := []string{"flow1-tc7", "flow2-tc7"}
	for _, tc7flow := range tc7flows {
		lossPct := gnmi.Get(t, ate, gnmi.OC().Flow(tc7flow).LossPct().State())
		if lossPct >= 1 {
			t.Errorf("Get(traffic loss for queue tc7): got %v, want < 1", lossPct)
		}
	}
	//for _, trflow := range ondatraFlowList {

	// flowPath := ate.Telemetry().Flow(flow.Name())
	// if got := flowPath.LossPct().Get(t); got > 0 {
	// 	t.Errorf("LossPct for flow %s got %g, want 0", flow.Name(), got)
	// }
	ate.Traffic().Stop(t)

	time.Sleep(time.Minute)

}

func testTrafficqos2(t *testing.T, expectPass bool, ate *ondatra.ATEDevice, top *ondatra.ATETopology, srcEndPoints []*ondatra.Interface, dstEndPoint *ondatra.Interface, scale int, hostIP string, args *testArgs, dscp uint8, weights ...float64) {

	ondatraFlowList := []*ondatra.Flow{}
	dstmacaddress := []string{"00:01:00:03:00:00", "00:01:00:04:00:00"}
	srcmacaddress := []string{"00:16:01:00:00:01", "00:17:01:00:00:01"}
	//var trafficFlows map[string]*trafficData
	trafficFlows := map[string]*trafficData{

		"flow1-tc6": {frameSize: 1000, trafficRate: 45, dscp: 48, queue: "tc6", srcmac: srcmacaddress[0], dstmac: dstmacaddress[0], srcendpoint: srcEndPoints[0]},
		"flow1-tc5": {frameSize: 1000, trafficRate: 20, dscp: 33, queue: "tc5", srcmac: srcmacaddress[0], dstmac: dstmacaddress[0], srcendpoint: srcEndPoints[0]},
		"flow1-tc4": {frameSize: 1000, trafficRate: 20, dscp: 25, queue: "tc4", srcmac: srcmacaddress[0], dstmac: dstmacaddress[0], srcendpoint: srcEndPoints[0]},
		"flow1-tc3": {frameSize: 1000, trafficRate: 2, dscp: 17, queue: "tc3", srcmac: srcmacaddress[0], dstmac: dstmacaddress[0], srcendpoint: srcEndPoints[0]},
		"flow1-tc2": {frameSize: 1000, trafficRate: 2, dscp: 9, queue: "tc2", srcmac: srcmacaddress[0], dstmac: dstmacaddress[0], srcendpoint: srcEndPoints[0]},
		"flow1-tc1": {frameSize: 1000, trafficRate: 1, dscp: 1, queue: "tc1", srcmac: srcmacaddress[0], dstmac: dstmacaddress[0], srcendpoint: srcEndPoints[0]},
		"flow2-tc6": {frameSize: 1000, trafficRate: 45, dscp: 48, queue: "tc6", srcmac: srcmacaddress[1], dstmac: dstmacaddress[1], srcendpoint: srcEndPoints[1]},
		"flow2-tc5": {frameSize: 1000, trafficRate: 20, dscp: 33, queue: "tc5", srcmac: srcmacaddress[1], dstmac: dstmacaddress[1], srcendpoint: srcEndPoints[1]},
		"flow2-tc4": {frameSize: 1000, trafficRate: 20, dscp: 25, queue: "tc4", srcmac: srcmacaddress[1], dstmac: dstmacaddress[1], srcendpoint: srcEndPoints[1]},
		"flow2-tc3": {frameSize: 1000, trafficRate: 2, dscp: 17, queue: "tc3", srcmac: srcmacaddress[1], dstmac: dstmacaddress[1], srcendpoint: srcEndPoints[1]},
		"flow2-tc2": {frameSize: 1000, trafficRate: 2, dscp: 9, queue: "tc2", srcmac: srcmacaddress[1], dstmac: dstmacaddress[1], srcendpoint: srcEndPoints[1]},
		"flow2-tc1": {frameSize: 1000, trafficRate: 1, dscp: 1, queue: "tc1", srcmac: srcmacaddress[1], dstmac: dstmacaddress[1], srcendpoint: srcEndPoints[1]},
	}

	for trafficID, data := range trafficFlows {

		ethHeader := ondatra.NewEthernetHeader()
		ethHeader.WithSrcAddress(data.srcmac)
		ethHeader.WithDstAddress(data.dstmac)

		ipv4Header := ondatra.NewIPv4Header()
		ipv4Header.SrcAddressRange().
			WithMin("198.51.100.0").
			WithMax("198.51.100.254").
			WithCount(250)
		ipv4Header.WithDSCP(data.dscp)
		ipv4Header.DstAddressRange().WithMin(hostIP).WithCount(uint32(scale)).WithStep("0.0.0.1")

		innerIpv4Header := ondatra.NewIPv4Header()
		innerIpv4Header.WithSrcAddress("200.1.0.2")
		innerIpv4Header.DstAddressRange().WithMin("201.1.0.2").WithCount(10000).WithStep("0.0.0.1")

		flow := ate.Traffic().NewFlow(trafficID).
			WithSrcEndpoints(data.srcendpoint).
			WithDstEndpoints(dstEndPoint)

		flow.WithFrameSize(300).WithFrameRatePct(data.trafficRate).WithHeaders(ethHeader, ipv4Header, innerIpv4Header)
		ondatraFlowList = append(ondatraFlowList, flow)
	}

	ate.Traffic().Start(t, ondatraFlowList...)
	time.Sleep(60 * time.Second)
	tc7flows := []string{"flow1-tc6", "flow2-tc6"}
	for _, tc7flow := range tc7flows {
		lossPct := gnmi.Get(t, ate, gnmi.OC().Flow(tc7flow).LossPct().State())
		if lossPct >= 1 {
			t.Errorf("Get(traffic loss for queue tc7): got %v, want < 1", lossPct)
		}
	}
	//for _, trflow := range ondatraFlowList {

	// flowPath := ate.Telemetry().Flow(flow.Name())
	// if got := flowPath.LossPct().Get(t); got > 0 {
	// 	t.Errorf("LossPct for flow %s got %g, want 0", flow.Name(), got)
	// }
	ate.Traffic().Stop(t)

	time.Sleep(time.Minute)

}

// configureATE configures port1, port2 and port3 on the ATE.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) *ondatra.ATETopology {
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
func testTrafficqoswrr(t *testing.T, expectPass bool, ate *ondatra.ATEDevice, top *ondatra.ATETopology, srcEndPoints []*ondatra.Interface, dstEndPoint *ondatra.Interface, scale int, hostIP string, args *testArgs, dscp uint8, weights ...float64) {
	ondatraFlowList := []*ondatra.Flow{}
	// dstmacaddress := []string{"00:01:00:03:00:00", "00:01:00:04:00:00"}
	// srcmacaddress := []string{"00:16:01:00:00:01", "00:17:01:00:00:01"}
	trafficFlows := map[string]*trafficData{

		"flow1-tc7": {frameSize: 1000, trafficRate: 1, dscp: 56, queue: "tc7", srcendpoint: srcEndPoints[0]},
		"flow1-tc6": {frameSize: 1000, trafficRate: 1, dscp: 48, queue: "tc6", srcendpoint: srcEndPoints[0]},
		"flow1-tc5": {frameSize: 1000, trafficRate: 18, dscp: 33, queue: "tc5", srcendpoint: srcEndPoints[0]},
		"flow1-tc4": {frameSize: 1000, trafficRate: 18, dscp: 25, queue: "tc4", srcendpoint: srcEndPoints[0]},
		"flow1-tc3": {frameSize: 1000, trafficRate: 18, dscp: 17, queue: "tc3", srcendpoint: srcEndPoints[0]},
		"flow1-tc2": {frameSize: 1000, trafficRate: 18, dscp: 9, queue: "tc2", srcendpoint: srcEndPoints[0]},
		"flow1-tc1": {frameSize: 1000, trafficRate: 18, dscp: 1, queue: "tc1", srcendpoint: srcEndPoints[0]},
		"flow2-tc7": {frameSize: 1000, trafficRate: 1, dscp: 56, queue: "tc7", srcendpoint: srcEndPoints[1]},
		"flow2-tc6": {frameSize: 1000, trafficRate: 1, dscp: 48, queue: "tc6", srcendpoint: srcEndPoints[1]},
		"flow2-tc5": {frameSize: 1000, trafficRate: 18, dscp: 33, queue: "tc5", srcendpoint: srcEndPoints[1]},
		"flow2-tc4": {frameSize: 1000, trafficRate: 18, dscp: 25, queue: "tc4", srcendpoint: srcEndPoints[1]},
		"flow2-tc3": {frameSize: 1000, trafficRate: 18, dscp: 17, queue: "tc3", srcendpoint: srcEndPoints[1]},
		"flow2-tc2": {frameSize: 1000, trafficRate: 18, dscp: 9, queue: "tc2", srcendpoint: srcEndPoints[1]},
		"flow2-tc1": {frameSize: 1000, trafficRate: 18, dscp: 1, queue: "tc1", srcendpoint: srcEndPoints[1]},
	}
	for trafficID, data := range trafficFlows {

		ethHeader := ondatra.NewEthernetHeader()

		ipv4Header := ondatra.NewIPv4Header()
		ipv4Header.SrcAddressRange().
			WithMin("198.51.100.0").
			WithMax("198.51.100.254").
			WithCount(250)
		ipv4Header.WithDSCP(data.dscp)
		ipv4Header.WithECN(1)
		ipv4Header.DstAddressRange().WithMin(hostIP).WithCount(uint32(scale)).WithStep("0.0.0.1")

		innerIpv4Header := ondatra.NewIPv4Header()
		innerIpv4Header.WithSrcAddress("200.1.0.2")
		innerIpv4Header.DstAddressRange().WithMin("201.1.0.2").WithCount(10000).WithStep("0.0.0.1")

		flow := ate.Traffic().NewFlow(trafficID).
			WithSrcEndpoints(data.srcendpoint).
			WithDstEndpoints(dstEndPoint)

		flow.WithFrameSize(300).WithFrameRatePct(data.trafficRate).WithHeaders(ethHeader, ipv4Header, innerIpv4Header)
		ondatraFlowList = append(ondatraFlowList, flow)
	}

	ate.Traffic().Start(t, ondatraFlowList...)
	time.Sleep(5 * time.Minute)
	type flow struct {
		flow7 []string
		flow6 []string
		flow5 []string
		flow1 []string
	}
	ixiaflows := flow{
		[]string{"flow1-tc7", "flow2-tc7"},
		[]string{"flow1-tc6", "flow2-tc6"},
		[]string{"flow1-tc5", "flow2-tc5"},
		[]string{"flow1-tc1", "flow2-tc1"},
	}

	for _, tc7flow := range ixiaflows.flow7 {
		lossPct := gnmi.Get(t, ate, gnmi.OC().Flow(tc7flow).LossPct().State())
		if lossPct >= 1 {
			t.Errorf("Get(traffic loss for queue tc7): got %v, want < 1", lossPct)
		}
	}
	for _, tc6flow := range ixiaflows.flow6 {
		lossPct := gnmi.Get(t, ate, gnmi.OC().Flow(tc6flow).LossPct().State())
		if lossPct >= 1 {
			t.Errorf("Get(traffic loss for queue tc7): got %v, want < 1", lossPct)
		}
	}
	ate.Traffic().Stop(t)

	time.Sleep(2 * time.Minute)

	var TotalInPktstc5 uint64
	var TotalInOctstc5 uint64
	var TotalInPktstc1 uint64
	var TotalInOctstc1 uint64
	for _, tc5flow := range ixiaflows.flow5 {
		flowcounterstc5 := gnmi.Get(t, ate, gnmi.OC().Flow(tc5flow).Counters().State())
		TotalInPktstc5 += *flowcounterstc5.InPkts
		TotalInOctstc5 += *flowcounterstc5.InOctets
	}
	for _, tc1flow := range ixiaflows.flow1 {
		flowcounterstc1 := gnmi.Get(t, ate, gnmi.OC().Flow(tc1flow).Counters().State())
		TotalInPktstc1 += *flowcounterstc1.InPkts
		TotalInOctstc1 += *flowcounterstc1.InOctets
	}

	mul := 2.9

	if float64(TotalInPktstc5)/float64(TotalInPktstc1) < mul {
		t.Errorf(" ERROR the flows not honoring configured Bandwidth remaining ratio")
	} else {
		t.Logf("flows are honoring the configured BRR")
	}

	//time.Sleep(6 * time.Hour)

	//lossPct := ate.Telemetry().Flow("flow1-tc5").InRate().Get(t)
}
func testTrafficqoswrrgoog(t *testing.T, expectPass bool, ate *ondatra.ATEDevice, top *ondatra.ATETopology, srcEndPoints []*ondatra.Interface, dstEndPoint *ondatra.Interface, scale int, hostIP string, args *testArgs, dscp uint8, trafficFlows map[string]*trafficData, weights ...float64) {
	ondatraFlowList := []*ondatra.Flow{}
	// dstmacaddress := []string{"00:01:00:02:00:00", "00:01:00:04:00:00"}
	// srcmacaddress := []string{"00:11:01:00:00:01", "00:17:01:00:00:01"}
	// trafficFlows := map[string]*trafficData{

	// 	"flow1-tc7": {frameSize: 1000, trafficRate: 0.1, dscp: 56, queue: "tc7", srcmac: srcmacaddress[0], dstmac: dstmacaddress[0], srcendpoint: srcEndPoints[0]},
	// 	"flow1-tc6": {frameSize: 1000, trafficRate: 99.9, dscp: 48, queue: "tc6", srcmac: srcmacaddress[0], dstmac: dstmacaddress[0], srcendpoint: srcEndPoints[0]},
	// 	"flow2-tc7": {frameSize: 1000, trafficRate: 0.7, dscp: 56, queue: "tc7", srcmac: srcmacaddress[1], dstmac: dstmacaddress[1], srcendpoint: srcEndPoints[1]},
	// 	"flow2-tc6": {frameSize: 1000, trafficRate: 99.3, dscp: 48, queue: "tc6", srcmac: srcmacaddress[1], dstmac: dstmacaddress[1], srcendpoint: srcEndPoints[1]},
	// }
	for trafficID, data := range trafficFlows {

		ethHeader := ondatra.NewEthernetHeader()
		ethHeader.WithSrcAddress(data.srcmac)
		ethHeader.WithDstAddress(data.dstmac)

		ipv4Header := ondatra.NewIPv4Header()
		ipv4Header.SrcAddressRange().
			WithMin("198.51.100.0").
			WithMax("198.51.100.254").
			WithCount(250)
		ipv4Header.WithDSCP(data.dscp)
		ipv4Header.WithECN(1)
		ipv4Header.DstAddressRange().WithMin(hostIP).WithCount(uint32(scale)).WithStep("0.0.0.1")

		innerIpv4Header := ondatra.NewIPv4Header()
		innerIpv4Header.WithSrcAddress("200.1.0.2")
		innerIpv4Header.DstAddressRange().WithMin("201.1.0.2").WithCount(10000).WithStep("0.0.0.1")

		flow := ate.Traffic().NewFlow(trafficID).
			WithSrcEndpoints(data.srcendpoint).
			WithDstEndpoints(dstEndPoint)

		flow.WithFrameSize(300).WithFrameRatePct(data.trafficRate).WithHeaders(ethHeader, ipv4Header, innerIpv4Header)
		ondatraFlowList = append(ondatraFlowList, flow)
	}

	ate.Traffic().Start(t, ondatraFlowList...)
	time.Sleep(5 * time.Minute)
	type flow struct {
		flow7 []string
		flow6 []string
		flow5 []string
		flow1 []string
	}
	ixiaflows := flow{
		[]string{"flow1-tc7", "flow2-tc7"},
		[]string{"flow1-tc6", "flow2-tc6"},
		[]string{"flow1-tc5", "flow2-tc5"},
		[]string{"flow1-tc1", "flow2-tc1"},
	}

	for _, tc7flow := range ixiaflows.flow7 {
		lossPct := gnmi.Get(t, ate, gnmi.OC().Flow(tc7flow).LossPct().State())
		if lossPct > 0 {
			t.Errorf("Get(traffic loss for queue tc7): got %v, want < 1", lossPct)
		}
	}

	for trafficID := range trafficFlows {

		if !strings.Contains(trafficID, "tc7") {

			lossPct := gnmi.Get(t, ate, gnmi.OC().Flow(trafficID).LossPct().State())
			if lossPct < 50 {
				t.Errorf("Get(traffic loss for queue nontc7): got %v, want > 50", lossPct)
			}

		}

	}

	ate.Traffic().Stop(t)

	time.Sleep(3 * time.Minute)

	//time.Sleep(6 * time.Hour)

	//lossPct := ate.Telemetry().Flow("flow1-tc5").InRate().Get(t)
}
func testTrafficqoswrrgoog2P(t *testing.T, expectPass bool, ate *ondatra.ATEDevice, top *ondatra.ATETopology, srcEndPoints []*ondatra.Interface, dstEndPoint *ondatra.Interface, scale int, hostIP string, args *testArgs, dscp uint8, trafficFlows map[string]*trafficData, weights ...float64) {
	ondatraFlowList := []*ondatra.Flow{}
	// dstmacaddress := []string{"00:01:00:02:00:00", "00:01:00:04:00:00"}
	// srcmacaddress := []string{"00:11:01:00:00:01", "00:17:01:00:00:01"}
	// trafficFlows := map[string]*trafficData{

	// 	"flow1-tc7": {frameSize: 1000, trafficRate: 0.1, dscp: 56, queue: "tc7", srcmac: srcmacaddress[0], dstmac: dstmacaddress[0], srcendpoint: srcEndPoints[0]},
	// 	"flow1-tc6": {frameSize: 1000, trafficRate: 99.9, dscp: 48, queue: "tc6", srcmac: srcmacaddress[0], dstmac: dstmacaddress[0], srcendpoint: srcEndPoints[0]},
	// 	"flow2-tc7": {frameSize: 1000, trafficRate: 0.7, dscp: 56, queue: "tc7", srcmac: srcmacaddress[1], dstmac: dstmacaddress[1], srcendpoint: srcEndPoints[1]},
	// 	"flow2-tc6": {frameSize: 1000, trafficRate: 99.3, dscp: 48, queue: "tc6", srcmac: srcmacaddress[1], dstmac: dstmacaddress[1], srcendpoint: srcEndPoints[1]},
	// }
	for trafficID, data := range trafficFlows {

		ethHeader := ondatra.NewEthernetHeader()
		ethHeader.WithSrcAddress(data.srcmac)
		ethHeader.WithDstAddress(data.dstmac)

		ipv4Header := ondatra.NewIPv4Header()
		ipv4Header.SrcAddressRange().
			WithMin("198.51.100.0").
			WithMax("198.51.100.254").
			WithCount(250)
		ipv4Header.WithDSCP(data.dscp)
		ipv4Header.WithECN(1)
		ipv4Header.DstAddressRange().WithMin(hostIP).WithCount(uint32(scale)).WithStep("0.0.0.1")

		innerIpv4Header := ondatra.NewIPv4Header()
		innerIpv4Header.WithSrcAddress("200.1.0.2")
		innerIpv4Header.DstAddressRange().WithMin("201.1.0.2").WithCount(10000).WithStep("0.0.0.1")

		flow := ate.Traffic().NewFlow(trafficID).
			WithSrcEndpoints(data.srcendpoint).
			WithDstEndpoints(dstEndPoint)

		flow.WithFrameSize(300).WithFrameRatePct(data.trafficRate).WithHeaders(ethHeader, ipv4Header, innerIpv4Header)
		ondatraFlowList = append(ondatraFlowList, flow)
	}

	ate.Traffic().Start(t, ondatraFlowList...)
	time.Sleep(5 * time.Minute)
	type flow struct {
		flow7 []string
		flow6 []string
		flow5 []string
		flow1 []string
	}
	ixiaflows := flow{
		[]string{"flow1-tc7", "flow2-tc7"},
		[]string{"flow1-tc6", "flow2-tc6"},
		[]string{"flow1-tc5", "flow2-tc5"},
		[]string{"flow1-tc1", "flow2-tc1"},
	}

	for _, tc6flow := range ixiaflows.flow6 {
		lossPct := gnmi.Get(t, ate, gnmi.OC().Flow(tc6flow).LossPct().State())
		if lossPct > 0 {
			t.Errorf("Get(traffic loss for queue tc6): got %v, want < 1", lossPct)
		}
	}

	for trafficID := range trafficFlows {

		if !strings.Contains(trafficID, "tc6") {

			lossPct := gnmi.Get(t, ate, gnmi.OC().Flow(trafficID).LossPct().State())
			t.Logf("Loss percent is %v", lossPct)

		}

	}

	ate.Traffic().Stop(t)

	time.Sleep(5 * time.Minute)

	//lossPct := ate.Telemetry().Flow("flow1-tc5").InRate().Get(t)
}

func testTrafficqoswrrgoog2Pwrr(t *testing.T, expectPass bool, ate *ondatra.ATEDevice, top *ondatra.ATETopology, srcEndPoints []*ondatra.Interface, dstEndPoint *ondatra.Interface, scale int, hostIP string, args *testArgs, dscp uint8, trafficFlows map[string]*trafficData, weights ...float64) {
	ondatraFlowList := []*ondatra.Flow{}
	// dstmacaddress := []string{"00:01:00:02:00:00", "00:01:00:04:00:00"}
	// srcmacaddress := []string{"00:11:01:00:00:01", "00:17:01:00:00:01"}
	// trafficFlows := map[string]*trafficData{

	// 	"flow1-tc7": {frameSize: 1000, trafficRate: 0.1, dscp: 56, queue: "tc7", srcmac: srcmacaddress[0], dstmac: dstmacaddress[0], srcendpoint: srcEndPoints[0]},
	// 	"flow1-tc6": {frameSize: 1000, trafficRate: 99.9, dscp: 48, queue: "tc6", srcmac: srcmacaddress[0], dstmac: dstmacaddress[0], srcendpoint: srcEndPoints[0]},
	// 	"flow2-tc7": {frameSize: 1000, trafficRate: 0.7, dscp: 56, queue: "tc7", srcmac: srcmacaddress[1], dstmac: dstmacaddress[1], srcendpoint: srcEndPoints[1]},
	// 	"flow2-tc6": {frameSize: 1000, trafficRate: 99.3, dscp: 48, queue: "tc6", srcmac: srcmacaddress[1], dstmac: dstmacaddress[1], srcendpoint: srcEndPoints[1]},
	// }
	for trafficID, data := range trafficFlows {

		ethHeader := ondatra.NewEthernetHeader()
		ethHeader.WithSrcAddress(data.srcmac)
		ethHeader.WithDstAddress(data.dstmac)

		ipv4Header := ondatra.NewIPv4Header()
		ipv4Header.SrcAddressRange().
			WithMin("198.51.100.0").
			WithMax("198.51.100.254").
			WithCount(250)
		ipv4Header.WithDSCP(data.dscp)
		ipv4Header.WithECN(1)
		ipv4Header.DstAddressRange().WithMin(hostIP).WithCount(uint32(scale)).WithStep("0.0.0.1")

		innerIpv4Header := ondatra.NewIPv4Header()
		innerIpv4Header.WithSrcAddress("200.1.0.2")
		innerIpv4Header.DstAddressRange().WithMin("201.1.0.2").WithCount(10000).WithStep("0.0.0.1")

		flow := ate.Traffic().NewFlow(trafficID).
			WithSrcEndpoints(data.srcendpoint).
			WithDstEndpoints(dstEndPoint)

		flow.WithFrameSize(300).WithFrameRatePct(data.trafficRate).WithHeaders(ethHeader, ipv4Header, innerIpv4Header)
		//flow.WithFrameSize(512).WithFrameRatePct(data.trafficRate).WithHeaders(ethHeader, ipv4Header, innerIpv4Header).Transmission().WithPatternBurst().WithPacketsPerBurst(1200).WithInterburstGapBytes(48000)
		ondatraFlowList = append(ondatraFlowList, flow)
	}

	ate.Traffic().Start(t, ondatraFlowList...)
	time.Sleep(5 * time.Minute)

	ate.Traffic().Stop(t)

	time.Sleep(5 * time.Minute)

	//lossPct := ate.Telemetry().Flow("flow1-tc5").InRate().Get(t)
}

func testTrafficqoswrrgoogmix(t *testing.T, expectPass bool, ate *ondatra.ATEDevice, top *ondatra.ATETopology, srcEndPoints []*ondatra.Interface, dstEndPoint *ondatra.Interface, scale int, hostIP string, args *testArgs, dscp uint8, weights ...float64) {
	ondatraFlowList := []*ondatra.Flow{}
	dstmacaddress := []string{"00:01:00:02:00:00", "00:01:00:04:00:00"}
	srcmacaddress := []string{"00:11:01:00:00:01", "00:17:01:00:00:01"}
	trafficFlows := map[string]*trafficData{

		"flow1-tc7": {frameSize: 1000, trafficRate: 0.1, dscp: 56, queue: "tc7", srcmac: srcmacaddress[0], dstmac: dstmacaddress[0], srcendpoint: srcEndPoints[0]},
		"flow1-tc6": {frameSize: 1000, trafficRate: 18, dscp: 48, queue: "tc6", srcmac: srcmacaddress[0], dstmac: dstmacaddress[0], srcendpoint: srcEndPoints[0]},
		"flow1-tc5": {frameSize: 1000, trafficRate: 40, dscp: 33, queue: "tc5", srcmac: srcmacaddress[0], dstmac: dstmacaddress[0], srcendpoint: srcEndPoints[0]},
		"flow1-tc4": {frameSize: 1000, trafficRate: 8, dscp: 25, queue: "tc4", srcmac: srcmacaddress[0], dstmac: dstmacaddress[0], srcendpoint: srcEndPoints[0]},
		"flow1-tc3": {frameSize: 1000, trafficRate: 12, dscp: 17, queue: "tc3", srcmac: srcmacaddress[0], dstmac: dstmacaddress[0], srcendpoint: srcEndPoints[0]},
		"flow1-tc2": {frameSize: 1000, trafficRate: 1, dscp: 9, queue: "tc2", srcmac: srcmacaddress[0], dstmac: dstmacaddress[0], srcendpoint: srcEndPoints[0]},
		"flow1-tc1": {frameSize: 1000, trafficRate: 1, dscp: 1, queue: "tc1", srcmac: srcmacaddress[0], dstmac: dstmacaddress[0], srcendpoint: srcEndPoints[0]},
		"flow2-tc7": {frameSize: 1000, trafficRate: 0.9, dscp: 56, queue: "tc7", srcmac: srcmacaddress[1], dstmac: dstmacaddress[1], srcendpoint: srcEndPoints[1]},
		"flow2-tc6": {frameSize: 1000, trafficRate: 20, dscp: 48, queue: "tc6", srcmac: srcmacaddress[1], dstmac: dstmacaddress[1], srcendpoint: srcEndPoints[1]},
		"flow2-tc5": {frameSize: 1000, trafficRate: 24, dscp: 33, queue: "tc5", srcmac: srcmacaddress[1], dstmac: dstmacaddress[1], srcendpoint: srcEndPoints[1]},
		"flow2-tc4": {frameSize: 1000, trafficRate: 24, dscp: 25, queue: "tc4", srcmac: srcmacaddress[1], dstmac: dstmacaddress[1], srcendpoint: srcEndPoints[1]},
		"flow2-tc3": {frameSize: 1000, trafficRate: 4, dscp: 17, queue: "tc3", srcmac: srcmacaddress[1], dstmac: dstmacaddress[1], srcendpoint: srcEndPoints[1]},
		"flow2-tc2": {frameSize: 1000, trafficRate: 7, dscp: 9, queue: "tc2", srcmac: srcmacaddress[1], dstmac: dstmacaddress[1], srcendpoint: srcEndPoints[1]},
		"flow2-tc1": {frameSize: 1000, trafficRate: 1, dscp: 1, queue: "tc1", srcmac: srcmacaddress[1], dstmac: dstmacaddress[1], srcendpoint: srcEndPoints[1]},
	}
	for trafficID, data := range trafficFlows {

		ethHeader := ondatra.NewEthernetHeader()
		ethHeader.WithSrcAddress(data.srcmac)
		ethHeader.WithDstAddress(data.dstmac)

		ipv4Header := ondatra.NewIPv4Header()
		ipv4Header.SrcAddressRange().
			WithMin("198.51.100.0").
			WithMax("198.51.100.254").
			WithCount(250)
		ipv4Header.WithDSCP(data.dscp)
		ipv4Header.WithECN(1)
		ipv4Header.DstAddressRange().WithMin(hostIP).WithCount(uint32(scale)).WithStep("0.0.0.1")

		innerIpv4Header := ondatra.NewIPv4Header()
		innerIpv4Header.WithSrcAddress("200.1.0.2")
		innerIpv4Header.DstAddressRange().WithMin("201.1.0.2").WithCount(10000).WithStep("0.0.0.1")

		flow := ate.Traffic().NewFlow(trafficID).
			WithSrcEndpoints(data.srcendpoint).
			WithDstEndpoints(dstEndPoint)

		flow.WithFrameSize(300).WithFrameRatePct(data.trafficRate).WithHeaders(ethHeader, ipv4Header, innerIpv4Header)
		ondatraFlowList = append(ondatraFlowList, flow)
	}

	ate.Traffic().Start(t, ondatraFlowList...)
	time.Sleep(5 * time.Minute)
	type flow struct {
		flow7 []string
		flow6 []string
		flow5 []string
		flow4 []string
		flow3 []string
		flow2 []string
		flow1 []string
	}
	ixiaflows := flow{
		[]string{"flow1-tc7", "flow2-tc7"},
		[]string{"flow1-tc6", "flow2-tc6"},
		[]string{"flow1-tc5", "flow2-tc5"},
		[]string{"flow1-tc4", "flow2-tc4"},
		[]string{"flow1-tc3", "flow2-tc3"},
		[]string{"flow1-tc2", "flow2-tc2"},
		[]string{"flow1-tc1", "flow2-tc1"},
	}

	for _, tc7flow := range ixiaflows.flow7 {
		lossPct := gnmi.Get(t, ate, gnmi.OC().Flow(tc7flow).LossPct().State())
		if lossPct >= 1 {
			t.Errorf("Get(traffic loss for queue tc7): got %v, want < 1", lossPct)
		}
	}
	for _, tc6flow := range ixiaflows.flow6 {
		lossPct := gnmi.Get(t, ate, gnmi.OC().Flow(tc6flow).LossPct().State())
		if lossPct >= 1 {
			t.Errorf("Get(traffic loss for queue tc7): got %v, want < 1", lossPct)
		}
	}
	ate.Traffic().Stop(t)

	time.Sleep(5 * time.Minute)

}
func testTrafficqoswrrgoog2Pwrrburst(t *testing.T, expectPass bool, ate *ondatra.ATEDevice, top *ondatra.ATETopology, srcEndPoints []*ondatra.Interface, dstEndPoint *ondatra.Interface, scale int, hostIP string, args *testArgs, dscp uint8, trafficFlows map[string]*trafficData, weights ...float64) {
	ondatraFlowList := []*ondatra.Flow{}
	// dstmacaddress := []string{"00:01:00:02:00:00", "00:01:00:04:00:00"}
	// srcmacaddress := []string{"00:11:01:00:00:01", "00:17:01:00:00:01"}
	// trafficFlows := map[string]*trafficData{

	// 	"flow1-tc7": {frameSize: 1000, trafficRate: 0.1, dscp: 56, queue: "tc7", srcmac: srcmacaddress[0], dstmac: dstmacaddress[0], srcendpoint: srcEndPoints[0]},
	// 	"flow1-tc6": {frameSize: 1000, trafficRate: 99.9, dscp: 48, queue: "tc6", srcmac: srcmacaddress[0], dstmac: dstmacaddress[0], srcendpoint: srcEndPoints[0]},
	// 	"flow2-tc7": {frameSize: 1000, trafficRate: 0.7, dscp: 56, queue: "tc7", srcmac: srcmacaddress[1], dstmac: dstmacaddress[1], srcendpoint: srcEndPoints[1]},
	// 	"flow2-tc6": {frameSize: 1000, trafficRate: 99.3, dscp: 48, queue: "tc6", srcmac: srcmacaddress[1], dstmac: dstmacaddress[1], srcendpoint: srcEndPoints[1]},
	// }
	for trafficID, data := range trafficFlows {

		ethHeader := ondatra.NewEthernetHeader()
		ethHeader.WithSrcAddress(data.srcmac)
		ethHeader.WithDstAddress(data.dstmac)

		ipv4Header := ondatra.NewIPv4Header()
		ipv4Header.SrcAddressRange().
			WithMin("198.51.100.0").
			WithMax("198.51.100.254").
			WithCount(250)
		ipv4Header.WithDSCP(data.dscp)
		ipv4Header.WithECN(1)
		ipv4Header.DstAddressRange().WithMin(hostIP).WithCount(uint32(scale)).WithStep("0.0.0.1")

		innerIpv4Header := ondatra.NewIPv4Header()
		innerIpv4Header.WithSrcAddress("200.1.0.2")
		innerIpv4Header.DstAddressRange().WithMin("201.1.0.2").WithCount(10000).WithStep("0.0.0.1")

		flow := ate.Traffic().NewFlow(trafficID).
			WithSrcEndpoints(data.srcendpoint).
			WithDstEndpoints(dstEndPoint)

		flow.WithFrameSize(300).WithFrameRatePct(data.trafficRate).WithHeaders(ethHeader, ipv4Header, innerIpv4Header).Transmission().WithPatternBurst().WithPacketsPerBurst(1200).WithInterburstGapBytes(48000)
		//flow.WithFrameSize(512).WithFrameRatePct(data.trafficRate).WithHeaders(ethHeader, ipv4Header, innerIpv4Header).Transmission().WithPatternBurst().WithPacketsPerBurst(1200).WithInterburstGapBytes(48000)
		ondatraFlowList = append(ondatraFlowList, flow)
	}

	ate.Traffic().Start(t, ondatraFlowList...)
	time.Sleep(5 * time.Minute)
	lossPcts := gnmi.GetAll(t, ate, gnmi.OC().FlowAny().LossPct().State())

	for _, loss := range lossPcts {
		if loss > 0 {
			t.Errorf("loss is more than 0")

		}

	}

	ate.Traffic().Stop(t)

	time.Sleep(5 * time.Minute)

}
