package cisco_gribi_test

import (
	"testing"
	"time"
//"fmt"

	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/ondatra"
)

func testTrafficWithInnerIPv6(t *testing.T, expectPass bool, ate *ondatra.ATEDevice, top *ondatra.ATETopology, srcEndPoint *ondatra.Interface, allPorts map[string]*ondatra.Interface, scale int, hostIP string, args *testArgs, dscp uint8, weights ...float64) {
	ethHeader := ondatra.NewEthernetHeader()
	ethHeader.WithSrcAddress("00:11:01:00:00:01")
	ethHeader.WithDstAddress("00:01:00:02:00:00")

	ipv4Header := ondatra.NewIPv4Header()
	ipv4Header.SrcAddressRange().
		WithMin("198.51.100.0").
		WithMax("198.51.100.254").
		WithCount(250)
	ipv4Header.WithDSCP(dscp)
	ipv4Header.DstAddressRange().WithMin(hostIP).WithCount(uint32(scale)).WithStep("0.0.0.1")

	innerIpv6Header := ondatra.NewIPv6Header()
	innerIpv6Header.WithSrcAddress("1::1")
	innerIpv6Header.DstAddressRange().WithMin("2::2").WithCount(10000).WithStep("::1")
	dstEndPoint := []ondatra.Endpoint{}

	for _, v := range allPorts {
		if *v != *srcEndPoint {
			dstEndPoint = append(dstEndPoint, v)
		}
	}

	flow := ate.Traffic().NewFlow("Flow").
		WithSrcEndpoints(srcEndPoint).
		WithDstEndpoints(dstEndPoint...)

	flow.WithFrameSize(300).WithFrameRateFPS(1000).WithHeaders(ethHeader, ipv4Header, innerIpv6Header)

	ate.Traffic().Start(t, flow)
	time.Sleep(15 * time.Second)

	stats := ate.Telemetry().InterfaceAny().Counters().Get(t)
	if got := util.CheckTrafficPassViaPortPktCounter(stats); got != expectPass {
		t.Errorf("Flow %s is not working as expected", flow.Name())
	}

	// tolerance := float64(0.03)
	// interval := 45 * time.Second
	// if len(weights) > 0 {
	// 	CheckDUTTrafficViaInterfaceTelemetry(t, args.dut, args.interfaces.in, args.interfaces.out[:len(weights)], weights, interval, tolerance)
	// }
	ate.Traffic().Stop(t)

	time.Sleep(time.Minute)

	// flowPath := ate.Telemetry().Flow(flow.Name())
	// if got := flowPath.LossPct().Get(t); got > 0 {
	// 	t.Errorf("LossPct for flow %s got %g, want 0", flow.Name(), got)
	// }
}

func testTraffic(t *testing.T, expectPass bool, ate *ondatra.ATEDevice, top *ondatra.ATETopology, srcEndPoint *ondatra.Interface, allPorts map[string]*ondatra.Interface, scale int, hostIP string, args *testArgs, dscp uint8, weights ...float64) {
        dscpList := []uint8{1, 9, 17, 25,33,41,49,57}
        ondatraFlowList := []*ondatra.Flow{}
        for _, dscp := range dscpList {

	ethHeader := ondatra.NewEthernetHeader()
	ethHeader.WithSrcAddress("00:11:01:00:00:01")
	ethHeader.WithDstAddress("00:01:00:02:00:00")

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
	stats := ate.Telemetry().InterfaceAny().Counters().Get(t)
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

	time.Sleep(time.Minute)
        //flowstats:= ate.Telemetry().FlowAny().Counters().Get(t)
        //for _, s  := range flowstats {
        //       fmt.Println("number of out packets in flow is",*s.OutPkts)


        }
        func testTrafficqos(t *testing.T, expectPass bool, ate *ondatra.ATEDevice, top *ondatra.ATETopology, srcEndPoint *ondatra.Interface, dstEndPoint *ondatra.Interface, scale int, hostIP string, args *testArgs, dscp uint8, weights ...float64) {
        dscpList := []uint8{1, 9, 17, 25,33,41,49,57}
        ondatraFlowList := []*ondatra.Flow{}
        for _, dscp := range dscpList {

        ethHeader := ondatra.NewEthernetHeader()
        ethHeader.WithSrcAddress("00:11:01:00:00:01")
        ethHeader.WithDstAddress("00:01:00:02:00:00")

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

        flow := ate.Traffic().NewFlow("Flow").
                WithSrcEndpoints(srcEndPoint).
                WithDstEndpoints(dstEndPoint)

        flow.WithFrameSize(300).WithFrameRateFPS(100).WithHeaders(ethHeader, ipv4Header, innerIpv4Header)
        ondatraFlowList = append(ondatraFlowList, flow)
      }

        ate.Traffic().Start(t, ondatraFlowList...)
        time.Sleep(60 * time.Second)
        threshold := 0.90
        stats := ate.Telemetry().InterfaceAny().Counters().Get(t)
        trafficPass := util.CheckTrafficPassViaPortPktCounter(stats, threshold)

        if trafficPass == expectPass {
                t.Log("Traffic works as expected")
        } else {
                t.Error("Traffic doesn't work as expected")
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

	p4 := ate.Port(t, "port4")
	i4 := top.AddInterface(atePort4.Name).WithPort(p4)
	i4.IPv4().
		WithAddress(atePort4.IPv4CIDR()).
		WithDefaultGateway(dutPort4.IPv4)

	p5 := ate.Port(t, "port5")
	i5 := top.AddInterface(atePort5.Name).WithPort(p5)
	i5.IPv4().
		WithAddress(atePort5.IPv4CIDR()).
		WithDefaultGateway(dutPort5.IPv4)

	p6 := ate.Port(t, "port6")
	i6 := top.AddInterface(atePort6.Name).WithPort(p6)
	i6.IPv4().
		WithAddress(atePort6.IPv4CIDR()).
		WithDefaultGateway(dutPort6.IPv4)

	p7 := ate.Port(t, "port7")
	i7 := top.AddInterface(atePort7.Name).WithPort(p7)
	i7.IPv4().
		WithAddress(atePort7.IPv4CIDR()).
		WithDefaultGateway(dutPort7.IPv4)

	p8 := ate.Port(t, "port8")
	i8 := top.AddInterface(atePort8.Name).WithPort(p8)
	i8.IPv4().
		WithAddress(atePort8.IPv4CIDR()).
		WithDefaultGateway(dutPort8.IPv4)

	//Configure vlans on ATE port2
	i2v10 := top.AddInterface("atePort2Vlan10").WithPort(p2)
	i2v10.Ethernet().WithMTU(1518).WithVLANID(10)
	i2v10.IPv4().
		WithAddress(atePort2Vlan10.IPv4CIDR()).
		WithDefaultGateway(dutPort2Vlan10.IPv4)
	i2v10.IPv6().
		WithAddress(atePort2Vlan10.IPv6CIDR()).
		WithDefaultGateway(dutPort2Vlan10.IPv6)

	i2v20 := top.AddInterface("atePort2Vlan20").WithPort(p2)
	i2v20.Ethernet().WithMTU(1518).WithVLANID(20)
	i2v20.IPv4().
		WithAddress(atePort2Vlan20.IPv4CIDR()).
		WithDefaultGateway(dutPort2Vlan20.IPv4)
	i2v20.IPv6().
		WithAddress(atePort2Vlan20.IPv6CIDR()).
		WithDefaultGateway(dutPort2Vlan20.IPv6)

	i2v30 := top.AddInterface("atePort2Vlan30").WithPort(p2)
	i2v30.Ethernet().WithMTU(1518).WithVLANID(30)
	i2v30.IPv4().
		WithAddress(atePort2Vlan30.IPv4CIDR()).
		WithDefaultGateway(dutPort2Vlan30.IPv4)
	i2v30.IPv6().
		WithAddress(atePort2Vlan30.IPv6CIDR()).
		WithDefaultGateway(dutPort2Vlan30.IPv6)

	return top
}

func testTrafficForFlows(t *testing.T, ate *ondatra.ATEDevice, topology *ondatra.ATETopology, expectPass bool, threshold float64, flow ...*ondatra.Flow) {

	ate.Traffic().Start(t, flow...)
	defer ate.Traffic().Stop(t)

	time.Sleep(60 * time.Second)

	stats := ate.Telemetry().InterfaceAny().Counters().Get(t)
	t.Log("Packets transmitted by ports: ", ate.Telemetry().InterfaceAny().Counters().OutPkts().Get(t))
	t.Log("Packets received by ports: ", ate.Telemetry().InterfaceAny().Counters().InPkts().Get(t))
	trafficPass := util.CheckTrafficPassViaPortPktCounter(stats, threshold)

	if trafficPass == expectPass {
		t.Log("Traffic works as expected")
	} else {
		t.Error("Traffic doesn't work as expected")
	}
}
