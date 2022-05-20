package cisco_gribi_test

import (
	"context"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi/util"
	"github.com/openconfig/featureprofiles/topologies/binding/cisco/config"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/ygot/ygot"
)

const (
	pbrName = "PBR"
)

func configBasePBR(t *testing.T, dut *ondatra.DUTDevice) {
	r1 := telemetry.NetworkInstance_PolicyForwarding_Policy_Rule{}
	r1.SequenceId = ygot.Uint32(1)
	r1.Ipv4 = &telemetry.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
		Protocol: telemetry.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP,
	}
	r1.Action = &telemetry.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String("TE")}

	r2 := telemetry.NetworkInstance_PolicyForwarding_Policy_Rule{}
	r2.SequenceId = ygot.Uint32(2)
	r2.Ipv4 = &telemetry.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
		DscpSet: []uint8{*ygot.Uint8(16)},
	}
	r2.Action = &telemetry.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String("TE")}

	r3 := telemetry.NetworkInstance_PolicyForwarding_Policy_Rule{}
	r3.SequenceId = ygot.Uint32(3)
	r3.Ipv4 = &telemetry.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
		DscpSet: []uint8{*ygot.Uint8(18)},
	}
	r3.Action = &telemetry.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String("VRF1")}

	r4 := telemetry.NetworkInstance_PolicyForwarding_Policy_Rule{}
	r4.SequenceId = ygot.Uint32(4)
	r4.Ipv4 = &telemetry.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
		DscpSet: []uint8{*ygot.Uint8(48)},
	}
	r4.Action = &telemetry.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String("TE")}

	p := telemetry.NetworkInstance_PolicyForwarding_Policy{}
	p.PolicyId = ygot.String(pbrName)
	p.Type = telemetry.Policy_Type_VRF_SELECTION_POLICY
	p.Rule = map[uint32]*telemetry.NetworkInstance_PolicyForwarding_Policy_Rule{1: &r1, 2: &r2, 3: &r3, 4: &r4}

	policy := telemetry.NetworkInstance_PolicyForwarding{}
	policy.Policy = map[string]*telemetry.NetworkInstance_PolicyForwarding_Policy{pbrName: &p}

	dut.Config().NetworkInstance("default").PolicyForwarding().Replace(t, &policy)
}

func configNewPolicy(t *testing.T, dut *ondatra.DUTDevice, policyName string, dscp uint8) {
	r1 := telemetry.NetworkInstance_PolicyForwarding_Policy_Rule{}
	r1.SequenceId = ygot.Uint32(1)
	r1.Ipv4 = &telemetry.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
		Protocol: telemetry.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP,
		DscpSet: []uint8{
			*ygot.Uint8(dscp),
		},
	}
	r1.Action = &telemetry.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String("TE")}

	p := telemetry.NetworkInstance_PolicyForwarding_Policy{}
	p.PolicyId = ygot.String(policyName)
	p.Type = telemetry.Policy_Type_VRF_SELECTION_POLICY
	p.Rule = map[uint32]*telemetry.NetworkInstance_PolicyForwarding_Policy_Rule{1: &r1}

	dut.Config().NetworkInstance("default").PolicyForwarding().Policy(policyName).Update(t, &p)
}

func configNewRule(t *testing.T, dut *ondatra.DUTDevice, policyName string, ruleID uint32, protocol uint8, dscp ...uint8) {
	r := telemetry.NetworkInstance_PolicyForwarding_Policy_Rule{}
	r.SequenceId = ygot.Uint32(ruleID)
	r.Ipv4 = &telemetry.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
		DscpSet: dscp,
	}
	if protocol == 4 {
		r.Ipv4.Protocol = telemetry.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP
	}
	r.Action = &telemetry.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String("TE")}
	dut.Config().NetworkInstance(instance).PolicyForwarding().Policy(policyName).Rule(ruleID).Replace(t, &r)
}

func generatePhysicalInterfaceConfig(t *testing.T, name, ipv4 string, prefixlen uint8) *telemetry.Interface {
	i := &telemetry.Interface{}
	i.Name = ygot.String(name)
	i.Type = telemetry.IETFInterfaces_InterfaceType_ethernetCsmacd
	s := i.GetOrCreateSubinterface(0)
	s4 := s.GetOrCreateIpv4()
	a := s4.GetOrCreateAddress(ipv4)
	a.PrefixLength = ygot.Uint8(prefixlen)
	return i
}

func generateBundleMemberInterfaceConfig(t *testing.T, name, bundleID string) *telemetry.Interface {
	i := &telemetry.Interface{Name: ygot.String(name)}
	i.Type = telemetry.IETFInterfaces_InterfaceType_ethernetCsmacd
	e := i.GetOrCreateEthernet()
	e.AutoNegotiate = ygot.Bool(false)
	e.AggregateId = ygot.String(bundleID)
	return i
}

func configPBRunderInterface(t *testing.T, args *testArgs, interfaceName, policyName string) {
	args.dut.Config().NetworkInstance(instance).PolicyForwarding().Interface(interfaceName).ApplyVrfSelectionPolicy().Replace(t, policyName)
}

func unconfigPBRunderInterface(t *testing.T, args *testArgs, interfaceName string) {
	args.dut.Config().NetworkInstance(instance).PolicyForwarding().Interface(interfaceName).ApplyVrfSelectionPolicy().Delete(t)
}

// Remove flowspec and add as pbr
func convertFlowspecToPBR(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice) {
	t.Log("Remove Flowspec Config and add HW Module Config")
	configToChange := "no flowspec \nhw-module profile pbr vrf-redirect\n"
	util.GNMIWithText(ctx, t, dut, configToChange)

	t.Log("Configure PBR policy and Apply it under interface")
	configBasePBR(t, dut)
	dut.Config().NetworkInstance(instance).PolicyForwarding().Interface("Bundle-Ether120").ApplyVrfSelectionPolicy().Update(t, pbrName)

	t.Log("Reload the router to activate hw module config")
	util.ReloadDUT(t, dut)

}

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

// Remove the policy under physical interface and add the related physical interface under bundle interface which use the same PBR policy
func movePhysicalToBundle(ctx context.Context, t *testing.T, args *testArgs, samePolicy bool) {
	configBasePBR(t, args.dut)

	physicalInterface := fptest.SortPorts(args.dut.Ports())[0].Name()
	physicalInterfaceConfig := args.dut.Config().Interface(physicalInterface)

	// Configure the physcial interface
	config := generatePhysicalInterfaceConfig(t, physicalInterface, "192.192.192.1", 24)
	physicalInterfaceConfig.Replace(t, config)

	// Configure policy on the physical interface and bunlde interface
	policyName := pbrName
	if !samePolicy {
		policyName = "new-PBR"
		configNewPolicy(t, args.dut, policyName, 0)
		defer args.dut.Config().NetworkInstance(instance).PolicyForwarding().Policy(policyName).Delete(t)
	}
	configPBRunderInterface(t, args, physicalInterface, policyName)
	configPBRunderInterface(t, args, args.interfaces.in[0], pbrName)

	// Remove the interface from physical to bundle interface
	memberConfig := generateBundleMemberInterfaceConfig(t, physicalInterface, args.interfaces.in[0])
	physicalInterfaceConfig.Replace(t, memberConfig)

	// Program GRIBI entry on the router
	defer flushSever(t, args)

	weights := []float64{10 * 15, 20 * 15, 30 * 15, 10 * 85, 20 * 85, 30 * 85, 40 * 85}

	configureBaseDoubleRecusionVip1Entry(ctx, t, args)
	configureBaseDoubleRecusionVip2Entry(ctx, t, args)
	configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)

	// Create Traffic and check traffic
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	// dstEndPoint := []*ondatra.Interface{args.top.Interfaces()[atePort2.Name], args.top.Interfaces()[atePort3.Name]}

	testTraffic(t, true, args.ate, args.top, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, args, 0, weights...)
}

func testMovePhysicalToBundleWithSamePolicy(ctx context.Context, t *testing.T, args *testArgs) {
	movePhysicalToBundle(ctx, t, args, true)
}

func testMovePhysicalToBundleWithDifferentPolicy(ctx context.Context, t *testing.T, args *testArgs) {
	movePhysicalToBundle(ctx, t, args, false)
}

// testChangePBRUnderInterface tests changing the PBR policy under the interface
func testChangePBRUnderInterface(ctx context.Context, t *testing.T, args *testArgs) {
	// Program GRIBI entry on the router
	defer flushSever(t, args)

	weights := []float64{10 * 15, 20 * 15, 30 * 15, 10 * 85, 20 * 85, 30 * 85, 40 * 85}

	configureBaseDoubleRecusionVip1Entry(ctx, t, args)
	configureBaseDoubleRecusionVip2Entry(ctx, t, args)
	configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)

	newPbrName := "new-PBR"
	dscpVal := uint8(10)

	// Configure new policy that matches the dscp and protocol IPinIP
	configNewPolicy(t, args.dut, newPbrName, dscpVal)
	defer args.dut.Config().NetworkInstance(instance).PolicyForwarding().Policy(newPbrName).Delete(t)

	// Change policy on the bunlde interface
	unconfigPBRunderInterface(t, args, args.interfaces.in[0])
	configPBRunderInterface(t, args, args.interfaces.in[0], newPbrName)
	defer configPBRunderInterface(t, args, args.interfaces.in[0], pbrName)

	// Create Traffic and check traffic
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	// dstEndPoint := []*ondatra.Interface{args.top.Interfaces()[atePort2.Name], args.top.Interfaces()[atePort3.Name]}

	testTraffic(t, true, args.ate, args.top, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, args, dscpVal, weights...)
}

// testIPv6InIPv4Traffic tests sending IPv6inIPv4 and verify it is not matched by IPinIP
func testIPv6InIPv4Traffic(ctx context.Context, t *testing.T, args *testArgs) {
	// Program GRIBI entry on the router
	defer flushSever(t, args)

	weights := []float64{10 * 15, 20 * 15, 30 * 15, 10 * 85, 20 * 85, 30 * 85, 40 * 85}

	configureBaseDoubleRecusionVip1Entry(ctx, t, args)
	configureBaseDoubleRecusionVip2Entry(ctx, t, args)
	configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)

	// Create Traffic and check traffic
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	// dstEndPoint := []*ondatra.Interface{args.top.Interfaces()[atePort2.Name], args.top.Interfaces()[atePort3.Name]}

	testTrafficWithInnerIPv6(t, false, args.ate, args.top, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, args, 0, weights...)
}

// testRemoveClassMap tests removing existing class-map which is not related to IPinIP match and verify traffic
func testRemoveClassMap(ctx context.Context, t *testing.T, args *testArgs) {
	defer configBasePBR(t, args.dut)

	defer flushSever(t, args)

	weights := []float64{10 * 15, 20 * 15, 30 * 15, 10 * 85, 20 * 85, 30 * 85, 40 * 85}

	configureBaseDoubleRecusionVip1Entry(ctx, t, args)
	configureBaseDoubleRecusionVip2Entry(ctx, t, args)
	configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)

	// Remove existing class map
	args.dut.Config().NetworkInstance("default").PolicyForwarding().Policy(pbrName).Rule(2).Delete(t)

	// Create Traffic and check traffic
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	// dstEndPoint := []*ondatra.Interface{args.top.Interfaces()[atePort2.Name], args.top.Interfaces()[atePort3.Name]}

	testTraffic(t, true, args.ate, args.top, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, args, 0, weights...)
}

func testChangeAction(ctx context.Context, t *testing.T, args *testArgs) {
	defer configBasePBR(t, args.dut)
	defer flushSever(t, args)

	weights := []float64{10 * 15, 20 * 15, 30 * 15, 10 * 85, 20 * 85, 30 * 85, 40 * 85}

	configureBaseDoubleRecusionVip1Entry(ctx, t, args)
	configureBaseDoubleRecusionVip2Entry(ctx, t, args)
	configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)

	// Change action for matching protocol IPinIP class
	args.dut.Config().NetworkInstance("default").PolicyForwarding().Policy(pbrName).Rule(1).Action().NetworkInstance().Replace(t, *ygot.String("VRF1"))

	// Create Traffic and check traffic
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	// dstEndPoint := []*ondatra.Interface{args.top.Interfaces()[atePort2.Name], args.top.Interfaces()[atePort3.Name]}

	// Expecting Traffic fail
	testTraffic(t, false, args.ate, args.top, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, args, 0, weights...)
}

func testAddClassMap(ctx context.Context, t *testing.T, args *testArgs) {
	defer configBasePBR(t, args.dut)
	defer flushSever(t, args)

	ruleID := uint32(10)
	dscp := uint8(32)
	configNewRule(t, args.dut, pbrName, ruleID, 4, dscp)
	defer args.dut.Config().NetworkInstance("default").PolicyForwarding().Policy(pbrName).Rule(ruleID).Delete(t)

	weights := []float64{10 * 15, 20 * 15, 30 * 15, 10 * 85, 20 * 85, 30 * 85, 40 * 85}

	configureBaseDoubleRecusionVip1Entry(ctx, t, args)
	configureBaseDoubleRecusionVip2Entry(ctx, t, args)
	configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)

	// Change action for matching protocol IPinIP class
	args.dut.Config().NetworkInstance("default").PolicyForwarding().Policy(pbrName).Rule(1).Action().NetworkInstance().Replace(t, *ygot.String("VRF1"))

	// Create Traffic and check traffic
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	// dstEndPoint := []*ondatra.Interface{args.top.Interfaces()[atePort2.Name], args.top.Interfaces()[atePort3.Name]}

	// Expecting Traffic fail
	testTraffic(t, false, args.ate, args.top, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, args, dscp, weights...)
}

func testAddRemoveHWModule(ctx context.Context, t *testing.T, args *testArgs) {
	defer flushSever(t, args)
	
	weights := []float64{10 * 15, 20 * 15, 30 * 15, 10 * 85, 20 * 85, 30 * 85, 40 * 85}
	configureBaseDoubleRecusionVip1Entry(ctx, t, args)
	configureBaseDoubleRecusionVip2Entry(ctx, t, args)
	configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)

	// Create Traffic and check traffic
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	// dstEndPoint := []*ondatra.Interface{args.top.Interfaces()[atePort2.Name], args.top.Interfaces()[atePort3.Name]}

	testTraffic(t, true, args.ate, args.top, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, args, 0, weights...)

	// disable hwmodule and reload and expect the traffic to be failed even after adding gribi routes
	beforeReloadConfig := "no hw-module profile pbr vrf-redirect"
	afterReloadConfig := ""
	config.Reload(context.Background(), t, args.dut, beforeReloadConfig, afterReloadConfig, 6*time.Minute)
	args.clientA.StartWithNoCache(t)
	args.clientA.BecomeLeader(t)
	configureBaseDoubleRecusionVip1Entry(ctx, t, args)
	configureBaseDoubleRecusionVip2Entry(ctx, t, args)
	configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)
	testTraffic(t, false, args.ate, args.top, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, args, 0, weights...)

	// enable hwmodule and reload and expect the traffic to be passed after adding gribi routes
	beforeReloadConfig = "hw-module profile pbr vrf-redirect"
	afterReloadConfig = ""
	config.Reload(context.Background(), t, args.dut, beforeReloadConfig, afterReloadConfig, 6*time.Minute)
	args.clientA.StartWithNoCache(t)
	args.clientA.BecomeLeader(t)
	configureBaseDoubleRecusionVip1Entry(ctx, t, args)
	configureBaseDoubleRecusionVip2Entry(ctx, t, args)
	configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)
	testTraffic(t, true, args.ate, args.top, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, args, 0, weights...)
}


func testRemoveADDPBRConfigWithGNMIReplace(ctx context.Context, t *testing.T, args *testArgs) {
	defer flushSever(t, args)
	
	weights := []float64{10 * 15, 20 * 15, 30 * 15, 10 * 85, 20 * 85, 30 * 85, 40 * 85}
	configureBaseDoubleRecusionVip1Entry(ctx, t, args)
	configureBaseDoubleRecusionVip2Entry(ctx, t, args)
	configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)

	// Create Traffic and check traffic
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	// dstEndPoint := []*ondatra.Interface{args.top.Interfaces()[atePort2.Name], args.top.Interfaces()[atePort3.Name]}

	testTraffic(t, true, args.ate, args.top, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, args, 0, weights...)

	// remove PBR expect the traffic to be failed even after adding gribi routes

	testTraffic(t, false, args.ate, args.top, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, args, 0, weights...)

	// add PBR config and expect the traffic to be passed after adding gribi routes

	testTraffic(t, true, args.ate, args.top, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, args, 0, weights...)
}
