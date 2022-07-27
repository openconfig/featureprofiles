package cisco_gribi_test

import (
	"context"
	"testing"
        "fmt"
        "time"
        "github.com/openconfig/featureprofiles/feature/cisco/qos/setup"
	"github.com/openconfig/featureprofiles/internal/attrs"
        //"github.com/openconfig/featureprofiles/internal/cisco/config"
        //"github.com/openconfig/ondatra"
	"github.com/openconfig/gribigo/fluent"
	oc "github.com/openconfig/ondatra/telemetry"
)

const (
	ipv4PrefixLen = 24
	ipv6PrefixLen = 126
	instance      = "default"
	vlanMTU       = 1518
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "dutPort1",
		IPv4:    "100.120.1.1",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2000::100:120:1:1",
		IPv6Len: ipv6PrefixLen,
	}

	atePort1 = attrs.Attributes{
		Name:    "atePort1",
		IPv4:    "100.120.1.2",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2000::100:120:1:2",
		IPv6Len: ipv6PrefixLen,
	}

	dutPort2 = attrs.Attributes{
		Desc:    "dutPort2",
		IPv4:    "100.121.1.1",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2000::100:121:1:1",
		IPv6Len: ipv6PrefixLen,
	}

	atePort2 = attrs.Attributes{
		Name:    "atePort2",
		IPv4:    "100.121.1.2",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2000::100:121:1:2",
		IPv6Len: ipv6PrefixLen,
	}

	dutPort3 = attrs.Attributes{
		Desc:    "dutPort3",
		IPv4:    "100.122.1.1",
		IPv4Len: ipv4PrefixLen,
	}

	atePort3 = attrs.Attributes{
		Name:    "atePort3",
		IPv4:    "100.122.1.2",
		IPv4Len: ipv4PrefixLen,
	}

	dutPort4 = attrs.Attributes{
		Desc:    "dutPort4",
		IPv4:    "100.123.1.1",
		IPv4Len: ipv4PrefixLen,
	}

	atePort4 = attrs.Attributes{
		Name:    "atePort4",
		IPv4:    "100.123.1.2",
		IPv4Len: ipv4PrefixLen,
	}
	dutPort5 = attrs.Attributes{
		Desc:    "dutPort5",
		IPv4:    "100.124.1.1",
		IPv4Len: ipv4PrefixLen,
	}

	atePort5 = attrs.Attributes{
		Name:    "atePort5",
		IPv4:    "100.124.1.2",
		IPv4Len: ipv4PrefixLen,
	}
	dutPort6 = attrs.Attributes{
		Desc:    "dutPort6",
		IPv4:    "100.125.1.1",
		IPv4Len: ipv4PrefixLen,
	}

	atePort6 = attrs.Attributes{
		Name:    "atePort6",
		IPv4:    "100.125.1.2",
		IPv4Len: ipv4PrefixLen,
	}
	dutPort7 = attrs.Attributes{
		Desc:    "dutPort7",
		IPv4:    "100.126.1.1",
		IPv4Len: ipv4PrefixLen,
	}

	atePort7 = attrs.Attributes{
		Name:    "atePort7",
		IPv4:    "100.126.1.2",
		IPv4Len: ipv4PrefixLen,
	}
	dutPort8 = attrs.Attributes{
		Desc:    "dutPort8",
		IPv4:    "100.127.1.1",
		IPv4Len: ipv4PrefixLen,
	}
	atePort8 = attrs.Attributes{
		Name:    "atePort8",
		IPv4:    "100.127.1.2",
		IPv4Len: ipv4PrefixLen,
	}

	dutPort2Vlan10 = attrs.Attributes{
		Desc:    "dutPort2Vlan10",
		IPv4:    "100.121.10.1",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2000::100:121:10:1",
		IPv6Len: ipv6PrefixLen,
		MTU:     vlanMTU,
	}

	atePort2Vlan10 = attrs.Attributes{
		Name:    "atePort2Vlan10",
		IPv4:    "100.121.10.2",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2000::100:121:10:2",
		IPv6Len: ipv6PrefixLen,
		MTU:     vlanMTU,
	}

	dutPort2Vlan20 = attrs.Attributes{
		Desc:    "dutPort2Vlan20",
		IPv4:    "100.121.20.1",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2000::100:121:20:1",
		IPv6Len: ipv6PrefixLen,
		MTU:     vlanMTU,
	}

	atePort2Vlan20 = attrs.Attributes{
		Name:    "atePort2Vlan20",
		IPv4:    "100.121.20.2",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2000::100:121:20:2",
		IPv6Len: ipv6PrefixLen,
		MTU:     vlanMTU,
	}

	dutPort2Vlan30 = attrs.Attributes{
		Desc:    "dutPort2Vlan30",
		IPv4:    "100.121.30.1",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2000::100:121:30:1",
		IPv6Len: ipv6PrefixLen,
		MTU:     vlanMTU,
	}

	atePort2Vlan30 = attrs.Attributes{
		Name:    "atePort2Vlan20",
		IPv4:    "100.121.30.2",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2000::100:121:30:2",
		IPv6Len: ipv6PrefixLen,
		MTU:     vlanMTU,
	}
)

func testDoubleRecursionWithUCMP(ctx context.Context, t *testing.T, args *testArgs) {
	// defer flushServer(t, args)
        //var baseConfig *oc.Qos = setupQos(t,args.dut)
        //println(baseConfig)
        var baseConfigEgress *oc.Qos = setupQosEgress(t, args.dut)
        println(baseConfigEgress)
        var baseConfig *oc.Qos = setupQos(t,args.dut)
        println(baseConfig)
        time.Sleep(2*time.Minute)


	weights := []float64{10 * 15, 20 * 15, 30 * 15, 10 * 85, 20 * 85, 30 * 85, 40 * 85}

	configureBaseDoubleRecusionVip1Entry(ctx, t, args)
	configureBaseDoubleRecusionVip2Entry(ctx, t, args)
	configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)
        //cfg := 
//`interface Bundle-Ether122
//shutdown
//!
//interface Bundle-Ether123
//shutdown
//!         
//interface Bundle-Ether124
//shutdown 
//!         
//interface Bundle-Ether125
//shutdown 
//!  
//interface Bundle-Ether126
//shutdown
//!
      
//`
 //       config.TextWithGNMI(context.Background(), t, args.dut, cfg )
  //      time.Sleep(time.Minute)

	srcEndPoint := args.top.Interfaces()[atePort1.Name]
        //dstEndPoint := args.top.Interfaces()[atePort2.Name]
	// dstEndPoint := []*ondatra.Interface{args.top.Interfaces()[atePort2.Name], args.top.Interfaces()[atePort3.Name]}

	testTraffic(t, true, args.ate, args.top, srcEndPoint, args.top.Interfaces() , args.prefix.scale, args.prefix.host, args, 0, weights...)
        

        outpackets:= []uint64{}
        inpackets:=[]uint64{}
	flowstats := args.ate.Telemetry().FlowAny().Counters().Get(t)
        for _, s  := range flowstats {
               fmt.Println("number of out packets in flow is",*s.OutPkts)
			   outpackets = append(outpackets,*s.OutPkts)
                           inpackets = append(inpackets,*s.InPkts)
            }
       outpupacket:= outpackets[0]
       fmt.Printf("*********************oupackets is %+v",outpackets)
       fmt.Printf("*********************inputpackets is %+v", inpackets)
       //time.Sleep(2*time.Minute)
       baseConfigTele := setupQosTele(t, args.dut)
       baseConfigInterface := setup.GetAnyValue(baseConfigTele.Interface)
       interfaceTelemetryPath := args.dut.Telemetry().Qos().Interface("Bundle-Ether120")


        t.Run(fmt.Sprintf("Get Interface Telemetry %s", *baseConfigInterface.InterfaceId), func(t *testing.T) {
                got := interfaceTelemetryPath.Get(t)
                for classifierType, classifier := range got.Input.Classifier {
                        for termId, term := range classifier.Term {
                                t.Run(fmt.Sprintf("Verify Matched-Packets of %v %s", classifierType, termId), func(t *testing.T) {
                                        if !(*term.MatchedPackets >= outpupacket) {
                                                t.Errorf("Get Interface Telemetry fail: got %+v", *got)
                                        }
                                })
                        }
                }
			})
       interfaceList := []string{}
	for i := 121; i < 128; i++ {
		interfaceList = append(interfaceList, fmt.Sprintf("Bundle-Ether%d", i))
	}
	queuestats := make(map[string]uint64)
	ixiastats := make(map[string]uint64)
	queueNames := []string{}
        for _, EgressInterface := range interfaceList {
		interfaceTelemetryEgrPath := args.dut.Telemetry().Qos().Interface(EgressInterface)
		t.Run(fmt.Sprintf("Get Interface Telemetry %s", EgressInterface), func(t *testing.T) {
			gote := interfaceTelemetryEgrPath.Get(t)
			for queueName, queue := range gote.Output.Queue {
				queuestats[queueName] += *queue.TransmitPkts

				queueNames = append(queueNames, queueName)

			}
		})
	}
        for index, inPkt := range inpackets {
		ixiastats[queueNames[index]] = inPkt
	}
        fmt.Printf("queuestats is %+v",queuestats)
        fmt.Printf("ixiastats is %+v",ixiastats)
	for name, _ := range queuestats {
		if !(queuestats[name] >= ixiastats[name] ){
			t.Errorf("Stats not matching for queue %+v",name)

		}

	}

       // var baseConfig *oc.Qos = setupQos(t,args.dut)
       // println(baseConfig)
       //var baseConfigEgress *oc.Qos = setupQosEgress(t, args.dut)
       //println(baseConfigEgress)
}

func testDeleteAndAddUCMP(ctx context.Context, t *testing.T, args *testArgs) {
	defer flushServer(t, args)

	// Programm the base double recursion entry
	configureBaseDoubleRecusionVip1Entry(ctx, t, args)
	configureBaseDoubleRecusionVip2Entry(ctx, t, args)
	configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)

	srcEndPoint := args.top.Interfaces()[atePort1.Name]

	// Delete UCMP at VRF Level by changing current NHG to single PATH
	args.clientA.AddNHG(t, args.prefix.vrfNhgIndex+1, map[uint64]uint64{args.prefix.vrfNhIndex + 1: 1}, instance, fluent.InstalledInRIB)
	weights := []float64{10, 20, 30}
	testTraffic(t, true, args.ate, args.top, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, args, 0, weights...)

	// Add back UCMP at VRF Level by changing NHG back to UCMP
	args.clientA.AddNHG(t, args.prefix.vrfNhgIndex+1, map[uint64]uint64{args.prefix.vrfNhIndex + 1: 15, args.prefix.vrfNhIndex + 2: 85}, instance, fluent.InstalledInRIB)
	weights = []float64{10 * 15, 20 * 15, 30 * 15, 10 * 85, 20 * 85, 30 * 85, 40 * 85}
	testTraffic(t, true, args.ate, args.top, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, args, 0, weights...)
}

func testVRFnonRecursion(ctx context.Context, t *testing.T, args *testArgs) {
	defer flushServer(t, args)

	// Programm the base double recursion entry
	configureBaseDoubleRecusionVip1Entry(ctx, t, args)
	configureBaseDoubleRecusionVip2Entry(ctx, t, args)
	configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)

	srcEndPoint := args.top.Interfaces()[atePort1.Name]

	// Change VRF Level NHG to single recursion which is same as the VIP1
	args.clientA.AddNHG(t, args.prefix.vrfNhgIndex+1, map[uint64]uint64{args.prefix.vip1NhIndex + 2: 10, args.prefix.vip1NhIndex + 3: 20, args.prefix.vip1NhIndex + 4: 30}, instance, fluent.InstalledInRIB)
	weights := []float64{10, 20, 30}
	testTraffic(t, true, args.ate, args.top, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, args, 0, weights...)

	// Change VRF Level NHG to back to double recursion
	args.clientA.AddNHG(t, args.prefix.vrfNhgIndex+1, map[uint64]uint64{args.prefix.vrfNhIndex + 1: 15, args.prefix.vrfNhIndex + 2: 85}, instance, fluent.InstalledInRIB)
	weights = []float64{10 * 15, 20 * 15, 30 * 15, 10 * 85, 20 * 85, 30 * 85, 40 * 85}
	testTraffic(t, true, args.ate, args.top, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, args, 0, weights...)
}
