package qos_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/feature/cisco/qos/setup"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cisco/config"
	ciscoFlags "github.com/openconfig/featureprofiles/internal/cisco/flags"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ygot/ygot"

	//"github.com/openconfig/ondatra/gnmi/oc"

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

func testQosCounter(ctx context.Context, t *testing.T, args *testArgs) {
	var baseConfigEgress *oc.Qos = setupQosEgress(t, args.dut)
	println(baseConfigEgress)
	var baseConfig *oc.Qos = setupQos(t, args.dut)
	println(baseConfig)
	time.Sleep(1 * time.Minute)

	weights := []float64{10 * 15, 20 * 15, 30 * 15, 10 * 85, 20 * 85, 30 * 85, 40 * 85}
	args.clientA.BecomeLeader(t)
	args.clientA.FlushServer(t)
	t.Log("Adding a defult route 0.0.0.0/0 as well pointing to a Valid NHOP ")
	config.TextWithGNMI(args.ctx, t, args.dut, "router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.40")
	config.TextWithGNMI(args.ctx, t, args.dut, "router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.42")
	defer config.TextWithGNMI(args.ctx, t, args.dut, "no router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.40")
	defer config.TextWithGNMI(args.ctx, t, args.dut, "no router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.42")

	args.clientA.AddNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.clientA.AddNH(t, 1100, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether122", false, ciscoFlags.GRIBIChecks)
	args.clientA.AddNH(t, 1200, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether123", false, ciscoFlags.GRIBIChecks)
	args.clientA.AddNH(t, 1300, atePort5.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether124", false, ciscoFlags.GRIBIChecks)
	args.clientA.AddNHG(t, 1000, 0, map[uint64]uint64{1000: 50, 1100: 30, 1200: 15, 1300: 5}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.clientA.AddIPv4(t, "192.0.2.40/32", 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.clientA.AddNH(t, 2000, atePort6.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether125", false, ciscoFlags.GRIBIChecks)
	args.clientA.AddNH(t, 2100, atePort7.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether126", false, ciscoFlags.GRIBIChecks)
	args.clientA.AddNH(t, 2200, atePort8.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether127", false, ciscoFlags.GRIBIChecks)

	args.clientA.AddNHG(t, 2000, 0, map[uint64]uint64{2000: 60, 2100: 20, 2200: 20}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.clientA.AddIPv4(t, "192.0.2.42/32", 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.clientA.AddNH(t, 100, "192.0.2.40", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.clientA.AddNH(t, 200, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.clientA.AddNHG(t, 100, 0, map[uint64]uint64{100: 85, 200: 15}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.clientA.AddIPv4(t, "11.11.11.0/32", 100, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	srcEndPoint := args.top.Interfaces()[atePort1.Name]

	testTraffic(t, true, args.ate, args.top, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, args, 0, weights...)

	outpackets := []uint64{}
	inpackets := []uint64{}
	flowstats := args.ate.Telemetry().FlowAny().Counters().Get(t)
	for _, s := range flowstats {
		fmt.Println("number of out packets in flow is", *s.OutPkts)
		outpackets = append(outpackets, *s.OutPkts)
		inpackets = append(inpackets, *s.InPkts)
	}
	outpupacket := outpackets[0]
	fmt.Printf("*********************oupackets is %+v", outpackets)
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
	fmt.Printf("queuestats is %+v", queuestats)
	fmt.Printf("ixiastats is %+v", ixiastats)

	for name := range queuestats {
		//if !(queuestats[name] >= ixiastats[name] ){
		if name == "tc7" {
			if !(queuestats[name] >= ixiastats[name]) {
				t.Errorf("Stats not matching for queue %+v", name)
			}
		} else {

			if !(queuestats[name] <= ixiastats[name]+10 ||
				queuestats[name] >= ixiastats[name]-10) {
				t.Errorf("Stats not matching for queue %+v", name)

			}
		}

	}

}

func ClearQosCounter(ctx context.Context, t *testing.T, args *testArgs) {
	//defer flushServer(t, args)
	t.Logf("clear qos counters on all interfaces")
	cliHandle := args.dut.RawAPIs().CLI(t)
	resp, err := cliHandle.SendCommand(context.Background(), "clear qos counters interface all")
	t.Logf(resp, err)
	t.Logf("sleeping after clearing qos counters")
	time.Sleep(3 * time.Minute)
	weights := []float64{10 * 15, 20 * 15, 30 * 15, 10 * 85, 20 * 85, 30 * 85, 40 * 85}
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	testTraffic(t, true, args.ate, args.top, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, args, 0, weights...)

	outpackets := []uint64{}
	inpackets := []uint64{}
	flowstats := args.ate.Telemetry().FlowAny().Counters().Get(t)
	for _, s := range flowstats {
		fmt.Println("number of out packets in flow is", *s.OutPkts)
		outpackets = append(outpackets, *s.OutPkts)
		inpackets = append(inpackets, *s.InPkts)
	}
	outpupacket := outpackets[0]

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

	//EgressInterface := "Bundle-Ether121"
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
	for name := range queuestats {
		if name == "tc7" {
			if !(queuestats[name] >= ixiastats[name]) {
				t.Errorf("Stats not matching for queue %+v", name)
			}
		} else {

			if !(queuestats[name] <= ixiastats[name]+10 ||
				queuestats[name] >= ixiastats[name]-10) {
				t.Errorf("Stats not matching for queue %+v", name)

			}
		}

	}

}

func QueueDelete(ctx context.Context, t *testing.T, args *testArgs) {
	defer args.clientA.FlushServer(t)
	defer teardownQos(t, args.dut)

	var baseConfig *oc.Qos = setupQosEgressTel(t, args.dut)
	queuNameInput := "tc1"
	baseConfigSchedulerPolicy := setup.GetAnyValue(baseConfig.SchedulerPolicy)
	baseConfigSchedulerPolicyScheduler := setup.GetAnyValue(baseConfigSchedulerPolicy.Scheduler)
	baseConfigSchedulerPolicySchedulerInput := baseConfigSchedulerPolicyScheduler.Input[queuNameInput]
	config := args.dut.Config().Qos().SchedulerPolicy(*baseConfigSchedulerPolicy.Name).Scheduler(*baseConfigSchedulerPolicyScheduler.Sequence).Input(*baseConfigSchedulerPolicySchedulerInput.Id)

	t.Run(fmt.Sprintf("Delete Queue %s", queuNameInput), func(t *testing.T) {
		config.Delete(t)

	})
	t.Run(fmt.Sprintf("Add back Queue %s", queuNameInput), func(t *testing.T) {
		config.Update(t, baseConfigSchedulerPolicySchedulerInput)

	})
	weights := []float64{10 * 15, 20 * 15, 30 * 15, 10 * 85, 20 * 85, 30 * 85, 40 * 85}

	// Programm the base double recursion entry

	srcEndPoint := args.top.Interfaces()[atePort1.Name]

	// Change VRF Level NHG to single recursion which is same as the VIP1

	testTraffic(t, true, args.ate, args.top, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, args, 0, weights...)
	baseConfigInterface := setup.GetAnyValue(baseConfig.Interface)
	t.Run(fmt.Sprintf("Get Interface Output Queue Telemetry %s %s", *baseConfigInterface.InterfaceId, queuNameInput), func(t *testing.T) {
		got := args.dut.Telemetry().Qos().Interface(*baseConfigInterface.InterfaceId).Output().Queue(queuNameInput).Get(t)
		t.Run("Verify Transmit-Octets", func(t *testing.T) {
			if !(*got.TransmitOctets > 0) {
				t.Errorf("Get Interface Output Queue Telemetry fail: got %+v", *got)
			}
		})
		t.Run("Verify Transmit-Packets", func(t *testing.T) {
			if !(*got.TransmitPkts > 0) {
				t.Errorf("Get Interface Output Queue Telemetry fail: got %+v", *got)
			}
		})
	})

}
func testQosCounteripv6(ctx context.Context, t *testing.T, args *testArgs) {
	var baseConfigEgress *oc.Qos = setupQosEgress(t, args.dut)
	println(baseConfigEgress)
	var baseConfig *oc.Qos = setupQosIpv6(t, args.dut)
	println(baseConfig)
	defer teardownQos(t, args.dut)
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	dstEndPoint := args.top.Interfaces()[atePort2.Name]
	testTrafficipv6(t, true, args.ate, args.top, srcEndPoint, dstEndPoint, args.prefix.scale, args.prefix.host, args, 0)
	outpackets := []uint64{}
	inpackets := []uint64{}
	flowstats := args.ate.Telemetry().FlowAny().Counters().Get(t)
	for _, s := range flowstats {
		fmt.Println("number of out packets in flow is", *s.OutPkts)
		outpackets = append(outpackets, *s.OutPkts)
		inpackets = append(inpackets, *s.InPkts)
	}
	outpupacket := outpackets[0]
	fmt.Printf("*********************oupackets is %+v", outpackets)
	fmt.Printf("*********************inputpackets is %+v", inpackets)
	interfaceTelemetryPath := args.dut.Telemetry().Qos().Interface("Bundle-Ether120")
	t.Run(fmt.Sprintf("Get Interface Telemetry %s", "Bundle-Ether120"), func(t *testing.T) {
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
	queuestats := make(map[string]uint64)
	ixiastats := make(map[string]uint64)
	queueNames := []string{}

	interfaceTelemetryEgrPath := args.dut.Telemetry().Qos().Interface("Bundle-Ether121")
	t.Run(fmt.Sprintf("Get Interface Telemetry %s", "Bundle-Ether121"), func(t *testing.T) {
		gote := interfaceTelemetryEgrPath.Get(t)
		for queueName, queue := range gote.Output.Queue {
			queuestats[queueName] += *queue.TransmitPkts

			queueNames = append(queueNames, queueName)

		}
	})

	for index, inPkt := range inpackets {
		ixiastats[queueNames[index]] = inPkt
	}
	fmt.Printf("queuestats is %+v", queuestats)
	fmt.Printf("ixiastats is %+v", ixiastats)

	for name := range queuestats {
		//if !(queuestats[name] >= ixiastats[name] ){
		if name == "tc7" {
			if !(queuestats[name] >= ixiastats[name]) {
				t.Errorf("Stats not matching for queue %+v", name)
			}
		} else {

			if !(queuestats[name] <= ixiastats[name]+10 ||
				queuestats[name] >= ixiastats[name]-10) {
				t.Errorf("Stats not matching for queue %+v", name)

			}
		}

	}

}

func testScheduler(ctx context.Context, t *testing.T, args *testArgs) {
	var baseConfigEgress *oc.Qos = setupQosEgressSche(t, args.dut)
	println(baseConfigEgress)
	var baseConfig *oc.Qos = setupQosSche(t, args.dut)
	println(baseConfig)
	time.Sleep(2 * time.Minute)

	defer args.clientA.FlushServer(t)
	//defer teardownQos(t, args.dut)
	//configureBaseDoubleRecusionVip1Entry(ctx, t, args)
	//configureBaseDoubleRecusionVip2Entry(ctx, t, args)
	//configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)
	//args.clientA.AddNHG(t, args.prefix.vrfNhgIndex+1, map[uint64]uint64{args.prefix.vip1NhIndex + 2: 100}, instance, fluent.InstalledInRIB)
	args.clientA.BecomeLeader(t)
	args.clientA.FlushServer(t)
	config.TextWithGNMI(args.ctx, t, args.dut, "router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.40")
	defer config.TextWithGNMI(args.ctx, t, args.dut, "no router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.40")

	args.clientA.AddNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.clientA.AddNHG(t, 1000, 0, map[uint64]uint64{1000: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.clientA.AddIPv4(t, "192.0.2.40/32", 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.clientA.AddNH(t, 100, "192.0.2.40", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.clientA.AddNHG(t, 100, 0, map[uint64]uint64{100: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.clientA.AddIPv4(t, "11.11.11.0/32", 100, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	weights := []float64{100}
	srcEndPoints := []*ondatra.Interface{args.top.Interfaces()[atePort3.Name], args.top.Interfaces()[atePort4.Name]}
	DstEndpoint := args.top.Interfaces()[atePort2.Name]
	testTrafficqos(t, true, args.ate, args.top, srcEndPoints, DstEndpoint, args.prefix.scale, args.prefix.host, args, 0, weights...)
	//time.Sleep(3 * time.Hour)
	tc7flows := []string{"flow1-tc7", "flow2-tc7"}
	var TotalInPkts uint64
	var TotalInOcts uint64
	for _, tc7flow := range tc7flows {
		flowcounters := args.ate.Telemetry().Flow(tc7flow).Counters().Get(t)
		TotalInPkts += *flowcounters.InPkts
		TotalInOcts += *flowcounters.InOctets
	}
	got := args.dut.Telemetry().Qos().Interface("Bundle-Ether121").Output().Queue("tc7").Get(t)
	t.Run("Verify Transmit-Packets for queue 7", func(t *testing.T) {
		if !(*got.TransmitPkts >= TotalInPkts) {
			t.Errorf("Get Interface Output Queue Telemetry fail: got %+v", *got)
		}
	})
	t.Run("Verify Transmit-Octets for queue 7", func(t *testing.T) {
		if !(*got.TransmitOctets >= TotalInOcts) {
			t.Errorf("Get Interface Output Queue Telemetry fail: got %+v", *got)
		}
	})

	t.Run("Verify Drooped-Packets for queue 7", func(t *testing.T) {
		if !(*got.DroppedPkts == 0) {
			t.Errorf("There should be no dropped packets: got %+v", *got)
		}
	})
	nontc7queues := []string{"tc1", "tc2", "tc3", "tc4", "tc5", "tc6"}
	for _, queues := range nontc7queues {
		got := args.dut.Telemetry().Qos().Interface("Bundle-Ether121").Output().Queue(queues).Get(t)
		t.Run("Verify Drooped-Packets for other queues", func(t *testing.T) {
			if !(*got.DroppedPkts != 0) {
				t.Errorf("There should be  dropped packets for queues: got %+v", *got)
			}
		})

	}

}
func testScheduler2(ctx context.Context, t *testing.T, args *testArgs) {
	var baseConfigEgress *oc.Qos = setupQosEgressSche(t, args.dut)
	println(baseConfigEgress)
	var baseConfig *oc.Qos = setupQosSche(t, args.dut)
	println(baseConfig)
	time.Sleep(2 * time.Minute)

	defer args.clientA.FlushServer(t)
	defer teardownQos(t, args.dut)
	args.clientA.BecomeLeader(t)
	args.clientA.FlushServer(t)
	config.TextWithGNMI(args.ctx, t, args.dut, "router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.40")
	defer config.TextWithGNMI(args.ctx, t, args.dut, "no router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.40")

	weights := []float64{100}
	args.clientA.AddNH(t, 100, "192.0.2.40", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.clientA.AddNHG(t, 100, 0, map[uint64]uint64{100: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.clientA.AddIPv4(t, "11.11.11.0/32", 100, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	args.clientA.AddNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.clientA.AddNHG(t, 1000, 0, map[uint64]uint64{1000: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.clientA.AddIPv4(t, "192.0.2.40/32", 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	srcEndPoints := []*ondatra.Interface{args.top.Interfaces()[atePort3.Name], args.top.Interfaces()[atePort4.Name]}
	DstEndpoint := args.top.Interfaces()[atePort2.Name]
	testTrafficqos2(t, true, args.ate, args.top, srcEndPoints, DstEndpoint, args.prefix.scale, args.prefix.host, args, 0, weights...)
	//time.Sleep(3 * time.Hour)
	tc6flows := []string{"flow1-tc6", "flow2-tc6"}
	var TotalInPkts uint64
	var TotalInOcts uint64
	for _, tc6flow := range tc6flows {
		flowcounters := args.ate.Telemetry().Flow(tc6flow).Counters().Get(t)
		TotalInPkts += *flowcounters.InPkts
		TotalInOcts += *flowcounters.InOctets
	}
	got := args.dut.Telemetry().Qos().Interface("Bundle-Ether121").Output().Queue("tc6").Get(t)
	t.Run("Verify Transmit-Packets for queue 6", func(t *testing.T) {
		if !(*got.TransmitPkts >= TotalInPkts) {
			t.Errorf("Get Interface Output Queue Telemetry fail: got %+v", *got)
		}
	})
	t.Run("Verify Transmit-Octets for queue 6", func(t *testing.T) {
		if !(*got.TransmitOctets >= TotalInOcts) {
			t.Errorf("Get Interface Output Queue Telemetry fail: got %+v", *got)
		}
	})

	t.Run("Verify Drooped-Packets for queue 7", func(t *testing.T) {
		if !(*got.DroppedPkts == 0) {
			t.Errorf("There should be no dropped packets: got %+v", *got)
		}
	})
	nontc6queues := []string{"tc1", "tc2", "tc3", "tc4", "tc5"}
	for _, queues := range nontc6queues {
		got := args.dut.Telemetry().Qos().Interface("Bundle-Ether121").Output().Queue(queues).Get(t)
		t.Run("Verify Drooped-Packets for other queues", func(t *testing.T) {
			if !(*got.DroppedPkts != 0) {
				t.Errorf("There should be  dropped packets for queues: got %+v", *got)
			}
		})

	}

}

func testQoswrrCounter(ctx context.Context, t *testing.T, args *testArgs) {
	ConfigureWrr(t, args.dut)
	weights := []float64{10 * 15, 20 * 15, 30 * 15, 10 * 85, 20 * 85, 30 * 85, 40 * 85}
	args.clientA.BecomeLeader(t)
	args.clientA.FlushServer(t)
	t.Log("Adding a defult route 0.0.0.0/0 as well pointing to a Valid NHOP ")
	config.TextWithGNMI(args.ctx, t, args.dut, "router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.40")
	config.TextWithGNMI(args.ctx, t, args.dut, "router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.42")
	defer config.TextWithGNMI(args.ctx, t, args.dut, "no router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.40")
	defer config.TextWithGNMI(args.ctx, t, args.dut, "no router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.42")

	args.clientA.AddNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.clientA.AddNH(t, 1100, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether122", false, ciscoFlags.GRIBIChecks)
	args.clientA.AddNH(t, 1200, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether123", false, ciscoFlags.GRIBIChecks)
	args.clientA.AddNH(t, 1300, atePort5.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether124", false, ciscoFlags.GRIBIChecks)
	args.clientA.AddNHG(t, 1000, 0, map[uint64]uint64{1000: 50, 1100: 30, 1200: 15, 1300: 5}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.clientA.AddIPv4(t, "192.0.2.40/32", 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.clientA.AddNH(t, 2000, atePort6.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether125", false, ciscoFlags.GRIBIChecks)
	args.clientA.AddNH(t, 2100, atePort7.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether126", false, ciscoFlags.GRIBIChecks)
	args.clientA.AddNH(t, 2200, atePort8.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether127", false, ciscoFlags.GRIBIChecks)

	args.clientA.AddNHG(t, 2000, 0, map[uint64]uint64{2000: 60, 2100: 20, 2200: 20}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.clientA.AddIPv4(t, "192.0.2.42/32", 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.clientA.AddNH(t, 100, "192.0.2.40", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.clientA.AddNH(t, 200, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.clientA.AddNHG(t, 100, 0, map[uint64]uint64{100: 85, 200: 15}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.clientA.AddIPv4(t, "11.11.11.0/32", 100, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	srcEndPoint := args.top.Interfaces()[atePort1.Name]

	testTraffic(t, true, args.ate, args.top, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, args, 0, weights...)
	outpackets := []uint64{}
	inpackets := []uint64{}
	flowstats := args.ate.Telemetry().FlowAny().Counters().Get(t)
	for _, s := range flowstats {
		fmt.Println("number of out packets in flow is", *s.OutPkts)
		outpackets = append(outpackets, *s.OutPkts)
		inpackets = append(inpackets, *s.InPkts)
	}
	outpupacket := outpackets[0]
	fmt.Printf("*********************oupackets is %+v", outpackets)
	fmt.Printf("*********************inputpackets is %+v", inpackets)
	//interfaceTelemetryPath := args.dut.Telemetry().Qos().Interface("Bundle-Ether120")
	t.Run(fmt.Sprintf("Get Interface Telemetry %s", "Bundle-Ether120"), func(t *testing.T) {
		classmaps := []string{"cmap1", "cmap2", "cmap3", "cmap4", "cmap5", "cmap6", "cmap7"}
		for _, term := range classmaps {
			MatchedPkts := gnmi.Get(t, args.dut, gnmi.OC().Qos().Interface("Bundle-Ether120").Input().Classifier(1).Term(term).MatchedPackets().State())
			if !(MatchedPkts >= outpupacket) {
				t.Errorf(" Error Get Interface Telemetry fail for term %v", term)
			}

		}

	})

	//gnmi.Get(t, args.dut, gnmi.OC().Qos().Interface("Bundle-Ether120").Input().Classifier(4).Term("cmap1").MatchedPackets().State())
	//gnmi.Get(t, args.dut, gnmi.OC().Qos().Interface(EgressInterface).Output().Queue(queueName).TransmitPkts().State())
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
	fmt.Printf("queuestats is %+v", queuestats)
	fmt.Printf("ixiastats is %+v", ixiastats)

	for name := range queuestats {
		//if !(queuestats[name] >= ixiastats[name] ){
		if name == "tc7" {
			if !(queuestats[name] >= ixiastats[name]) {
				t.Errorf("Stats not matching for queue %+v", name)
			}
		} else {

			if !(queuestats[name] <= ixiastats[name]+10 ||
				queuestats[name] >= ixiastats[name]-10) {
				t.Errorf("Stats not matching for queue %+v", name)

			}
		}

	}

}
func testQoswrrStreaming(ctx context.Context, t *testing.T, args *testArgs) {

	dutQosPktsBeforeTraffic := make(map[string]uint64)
	dutQosPktsAfterTraffic := make(map[string]uint64)
	queueNames := []string{"tc7", "tc6", "tc5", "tc4", "tc3", "tc2", "tc1"}
	interfaceList := []string{}
	for i := 121; i < 128; i++ {
		interfaceList = append(interfaceList, fmt.Sprintf("Bundle-Ether%d", i))
	}
	for _, EgressInterface := range interfaceList {
		for _, queueName := range queueNames {
			dutQosPktsBeforeTraffic[queueName] += gnmi.Get(t, args.dut, gnmi.OC().Qos().Interface(EgressInterface).Output().Queue(queueName).TransmitPkts().State())
		}
	}
	// for _, queueName := range queueNames {
	// 	dutQosPktsBeforeTraffic[queueName] = gnmi.Get(t, args.dut, gnmi.OC().Qos().Interface("Bundle-Ether121").Output().Queue(queueName).TransmitPkts().State())
	// }

	weights := []float64{10 * 15, 20 * 15, 30 * 15, 10 * 85, 20 * 85, 30 * 85, 40 * 85}
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	testTrafficsreaming(t, true, args.ate, args.top, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, args, 0, weights...)

	QueueCounter := args.dut.Telemetry().Qos().Interface("Bundle-Ether121")
	QueueCounter.Await(t, 3*time.Minute, QueueCounter.Get(t))
	args.ate.Traffic().Stop(t)
	for _, EgressInterface := range interfaceList {
		for _, queueName := range queueNames {
			dutQosPktsAfterTraffic[queueName] += gnmi.Get(t, args.dut, gnmi.OC().Qos().Interface(EgressInterface).Output().Queue(queueName).TransmitPkts().State())
		}
	}
	t.Logf("QoS egress packet counters before traffic: %v", dutQosPktsBeforeTraffic)
	t.Logf("QoS egress packet counters after traffic: %v", dutQosPktsAfterTraffic)
	for _, queue := range queueNames {
		if dutQosPktsAfterTraffic[queue] <= dutQosPktsBeforeTraffic[queue] {

			t.Errorf("packets not increased for queue %v", queue)
		}
	}

}
func testQoswrrdeladdseq(ctx context.Context, t *testing.T, args *testArgs) {
	defer args.clientA.FlushServer(t)
	defer teardownQos(t, args.dut)
	ConfigureDelAddSeq(t, args.dut)
	weights := []float64{10 * 15, 20 * 15, 30 * 15, 10 * 85, 20 * 85, 30 * 85, 40 * 85}
	t.Logf("clear qos counters on all interfaces")
	cliHandle := args.dut.RawAPIs().CLI(t)
	defer cliHandle.Close()
	resp, err := cliHandle.SendCommand(context.Background(), "clear qos counters interface all")
	t.Logf(resp, err)
	t.Logf("sleeping after clearing qos counters")
	time.Sleep(3 * time.Minute)
	srcEndPoint := args.top.Interfaces()[atePort1.Name]

	testTraffic(t, true, args.ate, args.top, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, args, 0, weights...)
	outpackets := []uint64{}
	inpackets := []uint64{}
	flowstats := args.ate.Telemetry().FlowAny().Counters().Get(t)
	for _, s := range flowstats {
		fmt.Println("number of out packets in flow is", *s.OutPkts)
		outpackets = append(outpackets, *s.OutPkts)
		inpackets = append(inpackets, *s.InPkts)
	}
	outpupacket := outpackets[0]
	fmt.Printf("*********************oupackets is %+v", outpackets)
	fmt.Printf("*********************inputpackets is %+v", inpackets)

	t.Run(fmt.Sprintf("Get Interface Telemetry %s", "Bundle-Ether120"), func(t *testing.T) {
		classmaps := []string{"cmap1", "cmap2", "cmap3", "cmap4", "cmap5", "cmap6", "cmap7"}
		for _, term := range classmaps {
			MatchedPkts := gnmi.Get(t, args.dut, gnmi.OC().Qos().Interface("Bundle-Ether120").Input().Classifier(1).Term(term).MatchedPackets().State())
			if !(MatchedPkts >= outpupacket) {
				t.Errorf(" Error Get Interface Telemetry fail for term %v", term)
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
	fmt.Printf("queuestats is %+v", queuestats)
	fmt.Printf("ixiastats is %+v", ixiastats)

	for name := range queuestats {
		//if !(queuestats[name] >= ixiastats[name] ){
		if name == "tc7" {
			if !(queuestats[name] >= ixiastats[name]) {
				t.Errorf("Stats not matching for queue %+v", name)
			}
		} else {

			if !(queuestats[name] <= ixiastats[name]+10 ||
				queuestats[name] >= ixiastats[name]-10) {
				t.Errorf("Stats not matching for queue %+v", name)

			}
		}

	}

}
func testSchedulerwrr(ctx context.Context, t *testing.T, args *testArgs) {
	ConfigureWrrSche(t, args.dut)
	defer args.clientA.FlushServer(t)
	defer teardownQos(t, args.dut)
	args.clientA.BecomeLeader(t)
	args.clientA.FlushServer(t)
	config.TextWithGNMI(args.ctx, t, args.dut, "router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.40")
	defer config.TextWithGNMI(args.ctx, t, args.dut, "no router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.40")

	args.clientA.AddNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.clientA.AddNHG(t, 1000, 0, map[uint64]uint64{1000: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.clientA.AddIPv4(t, "192.0.2.40/32", 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.clientA.AddNH(t, 100, "192.0.2.40", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.clientA.AddNHG(t, 100, 0, map[uint64]uint64{100: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.clientA.AddIPv4(t, "11.11.11.0/32", 100, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	weights := []float64{100}
	srcEndPoints := []*ondatra.Interface{args.top.Interfaces()[atePort3.Name], args.top.Interfaces()[atePort4.Name]}
	DstEndpoint := args.top.Interfaces()[atePort2.Name]
	testTrafficqoswrr(t, true, args.ate, args.top, srcEndPoints, DstEndpoint, args.prefix.scale, args.prefix.host, args, 0, weights...)
	queueNames := []string{"tc7", "tc6", "tc5", "tc4", "tc3", "tc2", "tc1"}
	queuestats := make(map[string]uint64)
	ixiastats := make(map[string]uint64)
	ixiaallflows := make(map[string][]string)
	for _, queueName := range queueNames {
		var flowcounterpkts uint64
		flo1 := "flow1-" + queueName
		flo2 := "flow2-" + queueName
		ixiaallflows[queueName] = []string{flo1, flo2}
		flowcounters := args.ate.Telemetry().Flow(flo1).Counters().Get(t)
		flowcounterpkts = *flowcounters.InPkts

		flowcounters = args.ate.Telemetry().Flow(flo2).Counters().Get(t)
		flowcounterpkts += *flowcounters.InPkts

		ixiastats[queueName] = flowcounterpkts
	}
	// for queueName, flowname := range ixiaallflows {
	// 	var flowcounterpkts uint64
	// 	for _, flow := range flowname {
	// 		flowcounters := args.ate.Telemetry().Flow(flow).Counters().Get(t)
	// 		flowcounterpkts += *flowcounters.InPkts
	// 	}
	// 	ixiastats[queueName] = flowcounterpkts

	// }
	interfaceTelemetryEgrPath := args.dut.Telemetry().Qos().Interface("Bundle-Ether121")
	gote := interfaceTelemetryEgrPath.Get(t)
	for _, queueName := range queueNames {
		queuestats[queueName] = *gote.Output.Queue[queueName].TransmitPkts
		if queueName == "tc7" || queueName == "tc6" {
			if !(queuestats[queueName] >= ixiastats[queueName]) || *gote.Output.Queue[queueName].DroppedPkts > 0 {
				t.Errorf("Stats not matching for queue %+v", queueName)
			}
		} else {

			if !(queuestats[queueName] <= ixiastats[queueName]+10 ||
				queuestats[queueName] >= ixiastats[queueName]-10) {
				t.Errorf("Stats not matching for queue %+v", queueName)

			}
		}

	}

}

func ConfigureWrr(t *testing.T, dut *ondatra.DUTDevice) {
	d := &oc.Device{}
	qos := d.GetOrCreateQos()
	queues := []string{"tc7", "tc6", "tc5", "tc4", "tc3", "tc2", "tc1"}
	for _, queue := range queues {
		q1 := qos.GetOrCreateQueue(queue)
		q1.Name = ygot.String(queue)
		dut.Config().Qos().Queue(*q1.Name).Update(t, q1)
	}
	priorqueues := []string{"tc7", "tc6"}
	schedulerpol := qos.GetOrCreateSchedulerPolicy("eg_policy1111")
	schedule := schedulerpol.GetOrCreateScheduler(1)
	schedule.Priority = oc.Scheduler_Priority_STRICT
	var ind uint64
	ind = 0
	for _, schedqueue := range priorqueues {
		input := schedule.GetOrCreateInput(schedqueue)
		input.Id = ygot.String(schedqueue)
		input.Weight = ygot.Uint64(7 - ind)
		input.Queue = ygot.String(schedqueue)
		ind += 1
	}
	configprior := dut.Config().Qos().SchedulerPolicy(*schedulerpol.Name).Scheduler(1)
	configprior.Replace(t, schedule)
	configGotprior := configprior.Get(t)
	if diff := cmp.Diff(*configGotprior, *schedule); diff != "" {
		t.Errorf("Config Schedule fail: \n%v", diff)
	}
	nonpriorqueues := []string{"tc5", "tc4", "tc3", "tc2", "tc1"}
	schedulenonprior := schedulerpol.GetOrCreateScheduler(2)
	schedulenonprior.Priority = oc.Scheduler_Priority_UNSET
	var weight uint64
	weight = 0
	for _, wrrqueue := range nonpriorqueues {
		inputwrr := schedulenonprior.GetOrCreateInput(wrrqueue)
		inputwrr.Id = ygot.String(wrrqueue)
		inputwrr.Queue = ygot.String(wrrqueue)
		inputwrr.Weight = ygot.Uint64(60 - weight)
		weight += 10
		configInputwrr := dut.Config().Qos().SchedulerPolicy(*schedulerpol.Name).Scheduler(2).Input(*inputwrr.Id)
		configInputwrr.Update(t, inputwrr)
		configGotwrr := configInputwrr.Get(t)
		if diff := cmp.Diff(*configGotwrr, *inputwrr); diff != "" {
			t.Errorf("Config Input fail: \n%v", diff)
		}

	}
	confignonprior := dut.Config().Qos().SchedulerPolicy(*schedulerpol.Name).Scheduler(2)
	// confignonprior.Update(t, schedulenonprior)
	configGotnonprior := confignonprior.Get(t)
	if diff := cmp.Diff(*configGotnonprior, *schedulenonprior); diff != "" {
		t.Errorf("Config Schedule fail: \n%v", diff)
	}
	interfaceList := []string{}
	for i := 121; i < 128; i++ {
		interfaceList = append(interfaceList, fmt.Sprintf("Bundle-Ether%d", i))
	}
	for _, inter := range interfaceList {

		schedinterface := qos.GetOrCreateInterface(inter)
		schedinterface.InterfaceId = ygot.String(inter)
		schedinterfaceout := schedinterface.GetOrCreateOutput()
		scheinterfaceschedpol := schedinterfaceout.GetOrCreateSchedulerPolicy()
		scheinterfaceschedpol.Name = ygot.String("eg_policy1111")

		ConfigIntf := dut.Config().Qos().Interface(*schedinterface.InterfaceId)
		ConfigIntf.Update(t, schedinterface)
		ConfigGotIntf := ConfigIntf.Get(t)
		if diff := cmp.Diff(*ConfigGotIntf, *schedinterface); diff != "" {
			t.Errorf("Config Schedule fail: \n%v", diff)
		}
	}

	qosi := d.GetOrCreateQos()
	classifiers := qosi.GetOrCreateClassifier("pmap9")
	classifiers.Name = ygot.String("pmap9")
	classifiers.Type = oc.Qos_Classifier_Type_IPV4
	classmaps := []string{"cmap1", "cmap2", "cmap3", "cmap4", "cmap5", "cmap6", "cmap7"}
	tclass := []string{"tc1", "tc2", "tc3", "tc4", "tc5", "tc6", "tc7"}
	dscps := []int{1, 9, 17, 25, 33, 41, 49}
	for index, classmap := range classmaps {
		terms := classifiers.GetOrCreateTerm(classmap)
		terms.Id = ygot.String(classmap)
		conditions := terms.GetOrCreateConditions()
		ipv4dscp := conditions.GetOrCreateIpv4()
		ipv4dscp.Dscp = ygot.Uint8(uint8(dscps[index]))

		actions := terms.GetOrCreateActions()
		actions.TargetGroup = ygot.String(tclass[index])
		fwdgroups := qosi.GetOrCreateForwardingGroup(tclass[index])
		fwdgroups.Name = ygot.String(tclass[index])
		fwdgroups.OutputQueue = ygot.String(tclass[index])

	}
	dut.Config().Qos().Update(t, qosi)
	classinterface := qosi.GetOrCreateInterface("Bundle-Ether120")
	classinterface.InterfaceId = ygot.String("Bundle-Ether120")
	Inputs := classinterface.GetOrCreateInput()
	Inputs.GetOrCreateClassifier(oc.Input_Classifier_Type_IPV4).Name = ygot.String("pmap9")
	Inputs.GetOrCreateClassifier(oc.Input_Classifier_Type_IPV6).Name = ygot.String("pmap9")
	Inputs.GetOrCreateClassifier(oc.Input_Classifier_Type_MPLS).Name = ygot.String("pmap9")
	//TODO: we use updtae due to the bug CSCwc76718, will change it to replace when the bug is fixed
	dut.Config().Qos().Interface(*classinterface.InterfaceId).Replace(t, classinterface)
}

func ConfigureDelAddSeq(t *testing.T, dut *ondatra.DUTDevice) {

	dut.Config().Qos().SchedulerPolicy("eg_policy1111").Scheduler(2).Delete(t)
	d := &oc.Device{}
	qos := d.GetOrCreateQos()
	schedulerpol := qos.GetOrCreateSchedulerPolicy("eg_policy1111")
	nonpriorqueues := []string{"tc5", "tc4", "tc3", "tc2", "tc1"}
	schedulenonprior := schedulerpol.GetOrCreateScheduler(2)
	schedulenonprior.Priority = oc.Scheduler_Priority_UNSET
	var weight uint64
	weight = 0
	for _, wrrqueue := range nonpriorqueues {
		inputwrr := schedulenonprior.GetOrCreateInput(wrrqueue)
		inputwrr.Id = ygot.String(wrrqueue)
		inputwrr.Queue = ygot.String(wrrqueue)
		inputwrr.Weight = ygot.Uint64(60 - weight)
		weight += 10
		configInputwrr := dut.Config().Qos().SchedulerPolicy(*schedulerpol.Name).Scheduler(2).Input(*inputwrr.Id)
		configInputwrr.Update(t, inputwrr)
		configGotwrr := configInputwrr.Get(t)
		if diff := cmp.Diff(*configGotwrr, *inputwrr); diff != "" {
			t.Errorf("Config Input fail: \n%v", diff)
		}

	}

}
func ConfigureWrrSche(t *testing.T, dut *ondatra.DUTDevice) {

	d := &oc.Device{}
	//defer teardownQos(t, dut)
	qos := d.GetOrCreateQos()
	queues := []string{"tc7", "tc6", "tc5", "tc4", "tc3", "tc2", "tc1"}
	for _, queue := range queues {
		q1 := qos.GetOrCreateQueue(queue)
		q1.Name = ygot.String(queue)
		dut.Config().Qos().Queue(*q1.Name).Update(t, q1)
	}
	priorqueues := []string{"tc7", "tc6"}
	schedulerpol := qos.GetOrCreateSchedulerPolicy("eg_policy1111")
	schedule := schedulerpol.GetOrCreateScheduler(1)
	schedule.Priority = oc.Scheduler_Priority_STRICT
	var ind uint64
	ind = 0
	for _, schedqueue := range priorqueues {
		input := schedule.GetOrCreateInput(schedqueue)
		input.Id = ygot.String(schedqueue)
		input.Weight = ygot.Uint64(7 - ind)
		input.Queue = ygot.String(schedqueue)
		ind += 1

	}
	nonpriorqueues := []string{"tc5", "tc4", "tc3", "tc2", "tc1"}
	schedulenonprior := schedulerpol.GetOrCreateScheduler(2)
	schedulenonprior.Priority = oc.Scheduler_Priority_UNSET
	var weight uint64
	weight = 0
	for _, wrrqueue := range nonpriorqueues {
		inputwrr := schedulenonprior.GetOrCreateInput(wrrqueue)
		inputwrr.Id = ygot.String(wrrqueue)
		inputwrr.Queue = ygot.String(wrrqueue)
		inputwrr.Weight = ygot.Uint64(60 - weight)
		weight += 10

	}
	minthresholdlist := []uint64{}
	for i := 1; i < 8; i++ {
		minthresholdlist = append(minthresholdlist, 1000000+uint64(i*6144))
	}
	maxthresholdlist := []uint64{}
	for i := 1; i < 8; i++ {
		maxthresholdlist = append(maxthresholdlist, 1300000+uint64(i*6144))
	}
	wredprofilelist := []string{}
	for i := 1; i < 8; i++ {
		wredprofilelist = append(wredprofilelist, fmt.Sprintf("wredprofile%d", i))
	}
	dropprobablity := []uint8{}
	for i := 1; i < 8; i++ {
		dropprobablity = append(dropprobablity, 10+uint8(i+2))
	}
	for i, wredprofile := range wredprofilelist {
		wredqueum := qos.GetOrCreateQueueManagementProfile(wredprofile)
		wredqueumred := wredqueum.GetOrCreateWred()
		wredqueumreduni := wredqueumred.GetOrCreateUniform()
		wredqueumreduni.MinThreshold = ygot.Uint64(minthresholdlist[i])
		wredqueumreduni.MaxThreshold = ygot.Uint64(maxthresholdlist[i])
		wredqueumreduni.EnableEcn = ygot.Bool(true)
		wredqueumreduni.MaxDropProbabilityPercent = ygot.Uint8(dropprobablity[i])

	}
	schedinterface := qos.GetOrCreateInterface("Bundle-Ether121")
	schedinterface.InterfaceId = ygot.String("Bundle-Ether121")
	schedinterfaceout := schedinterface.GetOrCreateOutput()
	scheinterfaceschedpol := schedinterfaceout.GetOrCreateSchedulerPolicy()
	scheinterfaceschedpol.Name = ygot.String("eg_policy1111")
	wrrqueues := []string{"tc1", "tc2", "tc3", "tc4", "tc5", "tc6", "tc7"}
	for i, wrrque := range wrrqueues {
		queueoutwred := schedinterfaceout.GetOrCreateQueue(wrrque)
		queueoutwred.QueueManagementProfile = ygot.String(wredprofilelist[i])
	}
	ConfigQos := dut.Config().Qos()
	ConfigQos.Update(t, qos)
	ConfigQosGet := ConfigQos.Get(t)

	if diff := cmp.Diff(*ConfigQosGet, *qos); diff != "" {
		t.Errorf("Config Schedule fail: \n%v", diff)
	}
	qosi := d.GetOrCreateQos()
	classifiers := qosi.GetOrCreateClassifier("pmap9")
	classifiers.Name = ygot.String("pmap9")
	classifiers.Type = oc.Qos_Classifier_Type_IPV4
	classmaps := []string{"cmap1", "cmap2", "cmap3", "cmap4", "cmap5", "cmap6", "cmap7"}
	tclass := []string{"tc1", "tc2", "tc3", "tc4", "tc5", "tc6", "tc7"}
	dscps := []int{1, 9, 17, 25, 33, 48, 56}
	for index, classmap := range classmaps {
		terms := classifiers.GetOrCreateTerm(classmap)
		terms.Id = ygot.String(classmap)
		conditions := terms.GetOrCreateConditions()
		ipv4dscp := conditions.GetOrCreateIpv4()
		ipv4dscp.Dscp = ygot.Uint8(uint8(dscps[index]))

		actions := terms.GetOrCreateActions()
		actions.TargetGroup = ygot.String(tclass[index])
		fwdgroups := qosi.GetOrCreateForwardingGroup(tclass[index])
		fwdgroups.Name = ygot.String(tclass[index])
		fwdgroups.OutputQueue = ygot.String(tclass[index])

	}
	dut.Config().Qos().Update(t, qosi)
	inputinterfaces := []string{"Bundle-Ether122", "Bundle-Ether123"}
	for _, inputinterface := range inputinterfaces {
		classinterface := qosi.GetOrCreateInterface(inputinterface)
		classinterface.InterfaceId = ygot.String(inputinterface)
		Inputs := classinterface.GetOrCreateInput()
		Inputs.GetOrCreateClassifier(oc.Input_Classifier_Type_IPV4).Name = ygot.String("pmap9")
		Inputs.GetOrCreateClassifier(oc.Input_Classifier_Type_IPV6).Name = ygot.String("pmap9")
		Inputs.GetOrCreateClassifier(oc.Input_Classifier_Type_MPLS).Name = ygot.String("pmap9")
		//TODO: we use updtae due to the bug CSCwc76718, will change it to replace when the bug is fixed
		dut.Config().Qos().Interface(*classinterface.InterfaceId).Replace(t, classinterface)
	}
}
