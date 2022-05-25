package cisco_gribi_test

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi/util"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/ygot/ygot"
	"github.com/openconfig/ygot/ytypes"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
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

func AddIpv6Address(ipv6 string, prefixlen uint8, index uint32) *telemetry.Interface_Subinterface {
	s := &telemetry.Interface_Subinterface{}
	s.Index = ygot.Uint32(index)
	s4 := s.GetOrCreateIpv6()
	a := s4.GetOrCreateAddress(ipv6)
	a.PrefixLength = ygot.Uint8(prefixlen)
	return s
}
func CreateNameSpace(t *testing.T, dut *ondatra.DUTDevice, name, intfname string, subint uint32) {
	//create empty subinterface
	si := &telemetry.Interface_Subinterface{}
	dut.Config().Interface(intfname).Subinterface(subint).Replace(t, si)

	//create vrf and apply on subinterface
	v := &telemetry.NetworkInstance{
		Name: ygot.String(name),
	}
	vi := v.GetOrCreateInterface(intfname + "." + strconv.Itoa(int(subint)))
	vi.Subinterface = ygot.Uint32(subint)
	dut.Config().NetworkInstance(name).Replace(t, v)
}

func GetSubInterface(ipv4 string, prefixlen4 uint8, ipv6 string, prefixlen6 uint8, vlanID uint16, index uint32) *telemetry.Interface_Subinterface {
	s := &telemetry.Interface_Subinterface{}
	s.Index = ygot.Uint32(index)
	s4 := s.GetOrCreateIpv4()
	a := s4.GetOrCreateAddress(ipv4)
	a.PrefixLength = ygot.Uint8(prefixlen4)
	s6 := s.GetOrCreateIpv6()
	a6 := s6.GetOrCreateAddress(ipv6)
	a6.PrefixLength = ygot.Uint8(prefixlen6)
	v := s.GetOrCreateVlan()
	m := v.GetOrCreateMatch()
	if index != 0 {
		m.GetOrCreateSingleTagged().VlanId = ygot.Uint16(vlanID)
	}
	return s
}

func configureIpv6AndVlans(t *testing.T, dut *ondatra.DUTDevice) {
	//Configure IPv6 address on Bundle-Ether120, Bundle-Ether121
	dut.Config().Interface("Bundle-Ether120").Subinterface(0).Update(t, AddIpv6Address(dutPort1.IPv6, dutPort1.IPv6Len, 0))
	dut.Config().Interface("Bundle-Ether121").Subinterface(0).Update(t, AddIpv6Address(dutPort2.IPv6, dutPort2.IPv6Len, 0))

	//Configure VLANs on Bundle-Ether121
	for i := 1; i <= 3; i++ {
		//Create VRFs and VRF enabled subinterfaces
		CreateNameSpace(t, dut, fmt.Sprintf("VRF%d", i*10), "Bundle-Ether121", uint32(i))
		//Add IPv4/IPv6 address on VLANs
		subint := GetSubInterface(fmt.Sprintf("100.121.%d.1", i*10), 24, fmt.Sprintf("2000::100:121:%d:1", i*10), 126, uint16(i*10), uint32(i))
		dut.Config().Interface("Bundle-Ether121").Subinterface(uint32(i)).Update(t, subint)
	}

}

func configurePBR(t *testing.T, dut *ondatra.DUTDevice, name, networkInstance, iptype string, index uint32, protocol telemetry.E_PacketMatchTypes_IP_PROTOCOL, dscpset []uint8) {
	r1 := telemetry.NetworkInstance_PolicyForwarding_Policy_Rule{}
	r1.SequenceId = ygot.Uint32(index)
	if iptype == "ipv4" {
		r1.Ipv4 = &telemetry.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
			Protocol: protocol,
		}
		if len(dscpset) > 0 {
			r1.Ipv4.DscpSet = dscpset
		}
	}
	if iptype == "ipv6" {
		r1.Ipv6 = &telemetry.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv6{
			Protocol: protocol,
		}
		if len(dscpset) > 0 {
			r1.Ipv6.DscpSet = dscpset
		}
	}
	r1.Action = &telemetry.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String(networkInstance)}
	p := telemetry.NetworkInstance_PolicyForwarding_Policy{}
	p.PolicyId = ygot.String(name)
	p.Type = telemetry.Policy_Type_VRF_SELECTION_POLICY
	p.AppendRule(&r1)

	policy := telemetry.NetworkInstance_PolicyForwarding{}
	policy.Policy = map[string]*telemetry.NetworkInstance_PolicyForwarding_Policy{pbrName: &p}

	dut.Config().NetworkInstance("default").PolicyForwarding().Update(t, &policy)
}

func configureL2PBR(t *testing.T, dut *ondatra.DUTDevice, name, networkInstance, iptype string, index uint32) {
	r1 := telemetry.NetworkInstance_PolicyForwarding_Policy_Rule{}
	r1.SequenceId = ygot.Uint32(index)
	if iptype == "ipv4" {
		r1.L2 = &telemetry.NetworkInstance_PolicyForwarding_Policy_Rule_L2{
			Ethertype: telemetry.PacketMatchTypes_ETHERTYPE_ETHERTYPE_IPV4,
		}
	}
	if iptype == "ipv6" {
		r1.L2 = &telemetry.NetworkInstance_PolicyForwarding_Policy_Rule_L2{
			Ethertype: telemetry.PacketMatchTypes_ETHERTYPE_ETHERTYPE_IPV6,
		}
	}
	r1.Action = &telemetry.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String(networkInstance)}
	p := telemetry.NetworkInstance_PolicyForwarding_Policy{}
	p.PolicyId = ygot.String(name)
	p.Type = telemetry.Policy_Type_VRF_SELECTION_POLICY
	p.AppendRule(&r1)
	policy := telemetry.NetworkInstance_PolicyForwarding{}
	policy.Policy = map[string]*telemetry.NetworkInstance_PolicyForwarding_Policy{pbrName: &p}

	dut.Config().NetworkInstance("default").PolicyForwarding().Update(t, &policy)
}

func GetBoundedFlow(t *testing.T, ate *ondatra.ATEDevice, srcEndpoint, dstEndPoint ondatra.Endpoint, flowName string, dscp uint8, ttl ...uint8) *ondatra.Flow {

	flow := ate.Traffic().NewFlow(flowName)
	t.Logf("Setting up flow -> %s", flowName)
	ethheader := ondatra.NewEthernetHeader()
	ipheader1 := ondatra.NewIPv4Header().WithDSCP(dscp)
	if len(ttl) > 0 {
		ipheader1.WithTTL(ttl[0])
	}
	flow.WithHeaders(ethheader, ipheader1)
	flow.WithSrcEndpoints(srcEndpoint)
	flow.WithDstEndpoints(dstEndPoint)
	flow.WithFrameRateFPS(100)
	flow.WithFrameSize(1024)
	return flow
}

func GetBoundedFlowIpv6(t *testing.T, ate *ondatra.ATEDevice, srcEndpoint, dstEndPoint ondatra.Endpoint, flowName string, dscp uint8) *ondatra.Flow {

	flow := ate.Traffic().NewFlow(flowName)
	t.Logf("Setting up flow -> %s", flowName)
	ethheader := ondatra.NewEthernetHeader()
	ipheader1 := ondatra.NewIPv6Header().WithDSCP(dscp)
	flow.WithHeaders(ethheader, ipheader1)
	flow.WithSrcEndpoints(srcEndpoint)
	flow.WithDstEndpoints(dstEndPoint)
	flow.WithFrameRateFPS(100)
	flow.WithFrameSize(1024)
	return flow
}

func GetBoundedFlowIPinIP(t *testing.T, ate *ondatra.ATEDevice, srcEndpoint, dstEndPoint ondatra.Endpoint, flowName string, dscp uint8, ttl ...uint8) *ondatra.Flow {

	flow := ate.Traffic().NewFlow(flowName)
	t.Logf("Setting up flow -> %s", flowName)
	ethheader := ondatra.NewEthernetHeader()
	outerIPHeader := ondatra.NewIPv4Header().WithDSCP(dscp)
	if len(ttl) > 0 {
		outerIPHeader.WithTTL(ttl[0])
	}
	innerIPHeader := ondatra.NewIPv4Header()
	innerIPHeader.WithSrcAddress("200.1.0.2")
	innerIPHeader.DstAddressRange().WithMin("201.1.0.2").WithCount(10000).WithStep("0.0.0.1")

	flow.WithHeaders(ethheader, outerIPHeader, innerIPHeader)
	flow.WithSrcEndpoints(srcEndpoint)
	flow.WithDstEndpoints(dstEndPoint)
	flow.WithFrameRateFPS(100)
	flow.WithFrameSize(1024)
	return flow
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

//deletePolicyFromInterface function removes the pbr policy from Bundle-Ether120 using CLI options.
//This is a temporary fix for accommodating various types of pbr policies on the interface
func deletePolicyFromInterface(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, policyName string) {
	configToChange := fmt.Sprintf("no interface Bundle-Ether120  service-policy type pbr input %s\n", policyName)
	util.GNMIWithText(ctx, t, dut, configToChange)
}

//deletePBRPolicyAndClassMaps function deletes pbr policy-map and class-map configuration using CLI.
//This is a temporary fix to cleanup the configurations.
func deletePBRPolicyAndClassMaps(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, policyName string, index int) {
	configToChange := fmt.Sprintf("no policy-map type pbr %s\n", policyName)
	for i := 1; i <= index; i++ {
		configToChange = configToChange + fmt.Sprintf("no class-map type traffic match-all %d_%s\n", i, policyName)
	}
	util.GNMIWithText(ctx, t, dut, configToChange)

}

func testDscpProtocolBasedVRFSelection(ctx context.Context, t *testing.T, args *testArgs) {
	t.Log("RT-3.1 :Protocol, DSCP-based VRF Selection - ensure that protocol and DSCP based VRF selection is configured correctly")
	//TODO - remove residual config. Fix when Delete starts working for PBR policy-map on interface.
	deletePolicyFromInterface(ctx, t, args.dut, pbrName)

	//Case1 - Matching ipv4 protocol to VRF10. Dropping IPv6 traffic in VRF10.
	configureL2PBR(t, args.dut, "L2", "VRF10", "ipv4", 1)
	configPBRunderInterface(t, args, args.interfaces.in[0], "L2")

	ipv4vlan10flow := GetBoundedFlow(t, args.ate, args.top.Interfaces()["atePort1"], args.top.Interfaces()["atePort2Vlan10"], "ipv4vlan10flow", 0, 100)
	ipv6vlan10flow := GetBoundedFlowIpv6(t, args.ate, args.top.Interfaces()["atePort1"], args.top.Interfaces()["atePort2Vlan20"], "ipv6vlan10flow", 0)
	testTrafficForFlows(t, args.ate, args.top, true, 0.90, ipv4vlan10flow)
	testTrafficForFlows(t, args.ate, args.top, false, 0.90, ipv6vlan10flow)

	//Case3 - Matching IPv4 protocol to VRF10, IPv6 protocol to VRF20. Dropping IPv6 traffic in VRF10 and IPv4 in VRF20.
	configureL2PBR(t, args.dut, "L2", "VRF20", "ipv6", 2)
	ipv4vlan20flow := GetBoundedFlow(t, args.ate, args.top.Interfaces()["atePort1"], args.top.Interfaces()["atePort2Vlan10"], "ipv4vlan20flow", 0, 100)
	ipv6vlan20flow := GetBoundedFlowIpv6(t, args.ate, args.top.Interfaces()["atePort1"], args.top.Interfaces()["atePort2Vlan20"], "ipv6vlan20flow", 0)
	testTrafficForFlows(t, args.ate, args.top, true, 0.90, ipv4vlan10flow, ipv6vlan20flow)
	testTrafficForFlows(t, args.ate, args.top, false, 0.90, ipv6vlan10flow, ipv4vlan20flow)

	//TODO - delete/replace of policy and class map and its entries is broken
	//cleanup
	deletePolicyFromInterface(ctx, t, args.dut, "L2")
	deletePBRPolicyAndClassMaps(context.Background(), t, args.dut, "L2", 2)

	//Case2 - Match IPinIP protocol to VRF10. Drop IPv4 and IPv6 traffic in VRF10.
	configurePBR(t, args.dut, "L3", "VRF10", "ipv4", 1, telemetry.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP, []uint8{})
	configPBRunderInterface(t, args, args.interfaces.in[0], "L3")
	ipinipvlan10flow := GetBoundedFlowIPinIP(t, args.ate, args.top.Interfaces()["atePort1"], args.top.Interfaces()["atePort2Vlan10"], "ipv4inipv4v10flow0", 0)

	testTrafficForFlows(t, args.ate, args.top, true, 0.90, ipinipvlan10flow)
	testTrafficForFlows(t, args.ate, args.top, false, 0.90, ipv4vlan10flow, ipv6vlan10flow)

	//Case4 - Match IPinIP and single DSCP46 to VRF10. Drop DSCP0 in VRF10.
	configurePBR(t, args.dut, "L3", "VRF10", "ipv4", 1, telemetry.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP, []uint8{46})
	ipinipvlan10flowd46 := GetBoundedFlowIPinIP(t, args.ate, args.top.Interfaces()["atePort1"], args.top.Interfaces()["atePort2Vlan10"], "ipv4inipv4v10flow46", 46)
	testTrafficForFlows(t, args.ate, args.top, false, 0.90, ipinipvlan10flow)
	testTrafficForFlows(t, args.ate, args.top, true, 0.90, ipinipvlan10flowd46)

	//Case5 - Match IPinIP and single DSCP46, DSCP42 to VRF10. Drop DSCP0 in VRF10.
	configurePBR(t, args.dut, "L3", "VRF10", "ipv4", 1, telemetry.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP, []uint8{42, 46})
	ipinipvlan10flowd42 := GetBoundedFlowIPinIP(t, args.ate, args.top.Interfaces()["atePort1"], args.top.Interfaces()["atePort2Vlan10"], "ipv4inipv4v10flow42", 42)
	testTrafficForFlows(t, args.ate, args.top, false, 0.90, ipinipvlan10flow)
	testTrafficForFlows(t, args.ate, args.top, true, 0.90, ipinipvlan10flowd46, ipinipvlan10flowd42)

	//cleanup
	deletePolicyFromInterface(ctx, t, args.dut, "L3")
	deletePBRPolicyAndClassMaps(context.Background(), t, args.dut, "L3", 1)
}

func testMultipleDscpProtocolRuleBasedVRFSelection(ctx context.Context, t *testing.T, args *testArgs) {
	t.Log("RT-3.2 : Multiple <Protocol, DSCP> Rules for VRF Selection - ensure that multiple VRF selection rules are matched correctly")
	//TODO - remove residual config. Fix when Delete starts working for PBR policy-map on interface.
	deletePolicyFromInterface(ctx, t, args.dut, pbrName)

	//Case1 - Ensure matching IPinIP with DSCP (10 - VRF10, 20- VRF20, 30-VRF30) traffic reaches to appropriate VLAN.
	configurePBR(t, args.dut, "L3", "VRF10", "ipv4", 1, telemetry.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP, []uint8{10})
	configurePBR(t, args.dut, "L3", "VRF20", "ipv4", 2, telemetry.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP, []uint8{20})
	configurePBR(t, args.dut, "L3", "VRF30", "ipv4", 3, telemetry.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP, []uint8{30})
	configPBRunderInterface(t, args, args.interfaces.in[0], "L3")

	ipinipd10 := GetBoundedFlowIPinIP(t, args.ate, args.top.Interfaces()["atePort1"], args.top.Interfaces()["atePort2Vlan10"], "ipvinipd10", 10, 100)
	ipinipd20 := GetBoundedFlowIPinIP(t, args.ate, args.top.Interfaces()["atePort1"], args.top.Interfaces()["atePort2Vlan20"], "ipvinipd20", 20, 100)
	ipinipd30 := GetBoundedFlowIPinIP(t, args.ate, args.top.Interfaces()["atePort1"], args.top.Interfaces()["atePort2Vlan30"], "ipvinipd30", 30, 100)
	testTrafficForFlows(t, args.ate, args.top, true, 0.90, ipinipd10, ipinipd20, ipinipd30)

	//Case2 - Ensure matching IPinIP with DSCP (10-12 - VRF10, 20-22- VRF20, 30-32-VRF30) traffic reaches to appropriate VLAN.
	configurePBR(t, args.dut, "L3", "VRF10", "ipv4", 1, telemetry.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP, []uint8{10, 11, 12})
	configurePBR(t, args.dut, "L3", "VRF20", "ipv4", 2, telemetry.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP, []uint8{20, 21, 22})
	configurePBR(t, args.dut, "L3", "VRF30", "ipv4", 3, telemetry.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP, []uint8{30, 31, 32})

	ipinipd11 := GetBoundedFlowIPinIP(t, args.ate, args.top.Interfaces()["atePort1"], args.top.Interfaces()["atePort2Vlan10"], "ipvinipd11", 11, 100)
	ipinipd12 := GetBoundedFlowIPinIP(t, args.ate, args.top.Interfaces()["atePort1"], args.top.Interfaces()["atePort2Vlan10"], "ipvinipd12", 12, 100)

	ipinipd21 := GetBoundedFlowIPinIP(t, args.ate, args.top.Interfaces()["atePort1"], args.top.Interfaces()["atePort2Vlan20"], "ipvinipd21", 21, 100)
	ipinipd22 := GetBoundedFlowIPinIP(t, args.ate, args.top.Interfaces()["atePort1"], args.top.Interfaces()["atePort2Vlan20"], "ipvinipd22", 22, 100)

	ipinipd31 := GetBoundedFlowIPinIP(t, args.ate, args.top.Interfaces()["atePort1"], args.top.Interfaces()["atePort2Vlan30"], "ipvinipd31", 31, 100)
	ipinipd32 := GetBoundedFlowIPinIP(t, args.ate, args.top.Interfaces()["atePort1"], args.top.Interfaces()["atePort2Vlan30"], "ipvinipd32", 32, 100)

	testTrafficForFlows(t, args.ate, args.top, true, 0.90,
		ipinipd10, ipinipd11, ipinipd12,
		ipinipd20, ipinipd21, ipinipd22,
		ipinipd30, ipinipd31, ipinipd32)
	//cleanup
	deletePolicyFromInterface(ctx, t, args.dut, "L3")
	deletePBRPolicyAndClassMaps(context.Background(), t, args.dut, "L3", 3)

	//Case3 - Ensure first matching of IPinIP with DSCP (10-12 - VRF10, 10-12 - VRF20) rule takes precedence.
	configurePBR(t, args.dut, "L3", "VRF10", "ipv4", 1, telemetry.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP, []uint8{10, 11, 12})
	configurePBR(t, args.dut, "L3", "VRF20", "ipv4", 2, telemetry.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP, []uint8{10, 11, 12})
	configPBRunderInterface(t, args, args.interfaces.in[0], "L3")

	ipinipd10v20 := GetBoundedFlowIPinIP(t, args.ate, args.top.Interfaces()["atePort1"], args.top.Interfaces()["atePort2Vlan20"], "ipvinipd10v20", 10, 100)
	ipinipd11v20 := GetBoundedFlowIPinIP(t, args.ate, args.top.Interfaces()["atePort1"], args.top.Interfaces()["atePort2Vlan20"], "ipvinipd11v20", 11, 100)
	ipinipd12v20 := GetBoundedFlowIPinIP(t, args.ate, args.top.Interfaces()["atePort1"], args.top.Interfaces()["atePort2Vlan20"], "ipvinipd12v20", 12, 100)

	testTrafficForFlows(t, args.ate, args.top, true, 0.90, ipinipd10, ipinipd11, ipinipd12)
	testTrafficForFlows(t, args.ate, args.top, false, 0.90, ipinipd10v20, ipinipd11v20, ipinipd12v20)

	//cleanup
	deletePolicyFromInterface(ctx, t, args.dut, "L3")
	deletePBRPolicyAndClassMaps(context.Background(), t, args.dut, "L3", 2)

	//Case4 - Ensure matching IPinIP to VRF10, IPinIP with DSCP20 VRF20 causes unspecified DSCP IPinIP traffic to match VRF10.
	configurePBR(t, args.dut, "L3", "VRF10", "ipv4", 1, telemetry.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP, []uint8{})
	configurePBR(t, args.dut, "L3", "VRF20", "ipv4", 2, telemetry.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP, []uint8{20})
	configPBRunderInterface(t, args, args.interfaces.in[0], "L3")

	//reuse ipinipd10, ipinipd11, ipinipd12 flows to match IPinIP to VRF10
	//reuse ipinipd20 flow to match IPinIP with DSCP20 to VRF20
	//reuse flows ipinipd10v20, ipinipd11v20, ipinipd12v20 to show they fail for VRF20

	testTrafficForFlows(t, args.ate, args.top, true, 0.90, ipinipd10, ipinipd11, ipinipd12, ipinipd20)
	testTrafficForFlows(t, args.ate, args.top, false, 0.90, ipinipd10v20, ipinipd11v20, ipinipd12v20)

	//cleanup
	deletePolicyFromInterface(ctx, t, args.dut, "L3")
	deletePBRPolicyAndClassMaps(context.Background(), t, args.dut, "L3", 2)
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

// verifyConfigPBRUnderInterface verifies that PBR is or is not configured under interface
//
// TODO: currently fails on XR due to missing key field sequence-id in GetResponse
func verifyConfigPBRUnderInterface(ctx context.Context, t *testing.T, args *testArgs, interfaceName string, shouldExist bool) {
	policyForwardingPath, _, _ := ygot.ResolvePath(args.dut.Config().NetworkInstance(instance).PolicyForwarding())
	gnmiC := args.dut.RawAPIs().GNMI().New(t)
	gotRes, err := gnmiC.Get(context.Background(), &gpb.GetRequest{
		Prefix:   &gpb.Path{Origin: "openconfig"},
		Path:     []*gpb.Path{policyForwardingPath},
		Encoding: gpb.Encoding_JSON_IETF,
		Type:     gpb.GetRequest_CONFIG,
	})
	if err != nil {
		t.Fatalf("Get(t) at path %s: %v", policyForwardingPath, err)
	}
	exists := false
	for _, n := range gotRes.Notification {
		for _, u := range n.Update {
			d := &telemetry.NetworkInstance_PolicyForwarding{}
			val := u.GetVal().GetJsonIetfVal()
			if val == nil {
				continue
			}
			if err := telemetry.Unmarshal(val, d, &ytypes.IgnoreExtraFields{}); err != nil {
				t.Fatalf("failed to unmarshal JSON_IETF in GetResponse: %v", err)
			}
			if data, ok := d.Interface[interfaceName]; ok {
				if data.ApplyVrfSelectionPolicy != nil {
					exists = true
				}
			}
		}
	}
	if exists != shouldExist {
		shouldExistString := "exist"
		if !shouldExist {
			shouldExistString = "not exist"
		}
		t.Fatalf("apply-vrf-selection-policy leaf needs to %s", shouldExistString)
	}
}

// testUnconfigPBRUnderBundleInterface tests unconfiguring the PBR policy under a bundle interface
func testUnconfigPBRUnderBundleInterface(ctx context.Context, t *testing.T, args *testArgs) {
	// Program GRIBI entry on the router
	defer flushSever(t, args)

	weights := []float64{10 * 15, 20 * 15, 30 * 15, 10 * 85, 20 * 85, 30 * 85, 40 * 85}
	interfaceName := args.interfaces.in[0]

	configureBaseDoubleRecusionVip1Entry(ctx, t, args)
	configureBaseDoubleRecusionVip2Entry(ctx, t, args)
	configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)

	t.Run("Delete apply-vrf-selection-policy leaf", func(t *testing.T) {
		args.dut.Config().NetworkInstance(instance).PolicyForwarding().Interface(interfaceName).ApplyVrfSelectionPolicy().Delete(t)
		defer configPBRunderInterface(t, args, interfaceName, pbrName)

		// // TODO: enabled once gNMI Get works on XR
		// t.Run("Verify deleted", func(t *testing.T) {
		// 	verifyConfigPBRUnderInterface(ctx, t, args, interfaceName, false)
		// })

		t.Run("Verify bundle still up", func(t *testing.T) {
			if got := args.dut.Telemetry().Interface(interfaceName).OperStatus().Get(t); got != telemetry.Interface_OperStatus_UP {
				t.Errorf("oper-status: got %v", got)
			}
		})

		t.Run("Expect traffic fail", func(t *testing.T) {
			// Create Traffic and check traffic
			srcEndPoint := args.top.Interfaces()[atePort1.Name]
			// Expecting Traffic fail
			testTraffic(t, false, args.ate, args.top, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, args, 0, weights...)
		})
	})

	t.Run("Delete interface list entry", func(t *testing.T) {
		args.dut.Config().NetworkInstance(instance).PolicyForwarding().Interface(interfaceName).Delete(t)
		defer configPBRunderInterface(t, args, interfaceName, pbrName)

		// // TODO: enabled once gNMI Get works on XR
		// t.Run("Verify deleted", func(t *testing.T) {
		// 	verifyConfigPBRUnderInterface(ctx, t, args, interfaceName, false)
		// })

		t.Run("Verify bundle still up", func(t *testing.T) {
			if got := args.dut.Telemetry().Interface(interfaceName).OperStatus().Get(t); got != telemetry.Interface_OperStatus_UP {
				t.Errorf("oper-status: got %v", got)
			}
		})

		t.Run("Expect traffic fail", func(t *testing.T) {
			// Create Traffic and check traffic
			srcEndPoint := args.top.Interfaces()[atePort1.Name]
			// Expecting Traffic fail
			testTraffic(t, false, args.ate, args.top, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, args, 0, weights...)
		})
	})
}

func equalUint8Slice(a, b []uint8) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

// verifyConfigPBRUnderInterface verifies that PBR values for dscp-set
//
// TODO: currently fails on XR due to data missing in GetResponse
func verifyConfigPbrMatchIpv4DscpSet(ctx context.Context, t *testing.T, args *testArgs, ruleID uint32, expectedDscpSet []uint8) {
	policyPath, _, _ := ygot.ResolvePath(args.dut.Config().NetworkInstance(instance).PolicyForwarding().Policy(pbrName))
	gnmiC := args.dut.RawAPIs().GNMI().New(t)
	gotRes, err := gnmiC.Get(context.Background(), &gpb.GetRequest{
		Prefix:   &gpb.Path{Origin: "openconfig"},
		Path:     []*gpb.Path{policyPath},
		Encoding: gpb.Encoding_JSON_IETF,
		Type:     gpb.GetRequest_CONFIG,
	})
	if err != nil {
		t.Fatalf("Get(t) at path %s: %v", policyPath, err)
	}
	var dscpSet []uint8
	for _, n := range gotRes.Notification {
		for _, u := range n.Update {
			d := &telemetry.NetworkInstance_PolicyForwarding_Policy{}
			val := u.GetVal().GetJsonIetfVal()
			if val == nil {
				continue
			}
			if err := telemetry.Unmarshal(val, d, &ytypes.IgnoreExtraFields{}); err != nil {
				t.Fatalf("failed to unmarshal JSON_IETF in GetResponse: %v", err)
			}
			if data, ok := d.Rule[ruleID]; ok && data.Ipv4 != nil && data.Ipv4.DscpSet != nil {
				dscpSet = data.Ipv4.DscpSet
			}
		}
	}
	if !equalUint8Slice(dscpSet, expectedDscpSet) {
		t.Fatalf("dscp-set: got %v, want %v", dscpSet, expectedDscpSet)
	} else {
		t.Logf("dscp-set=%v matched expected", dscpSet)
	}
}

// testRemoveMatchField tests existing match field in existing class-map which is not related to IPinIP match and verify traffic
func testRemoveMatchField(ctx context.Context, t *testing.T, args *testArgs) {
	defer flushSever(t, args)

	weights := []float64{10 * 15, 20 * 15, 30 * 15, 10 * 85, 20 * 85, 30 * 85, 40 * 85}

	configureBaseDoubleRecusionVip1Entry(ctx, t, args)
	configureBaseDoubleRecusionVip2Entry(ctx, t, args)
	configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)

	t.Run("Remove dscp-set", func(t *testing.T) {
		// Remove existing match field
		args.dut.Config().NetworkInstance("default").PolicyForwarding().Policy(pbrName).Rule(2).Ipv4().DscpSet().Delete(t)
		defer configBasePBR(t, args.dut)

		// // TODO: enabled once gNMI Get works on XR
		// t.Run("Verify deleted", func(t *testing.T) {
		// 	verifyConfigPbrMatchIpv4DscpSet(ctx, t, args, 2, []uint8{})
		// })

		// Create Traffic and check traffic
		t.Run("Verify traffic", func(t *testing.T) {
			srcEndPoint := args.top.Interfaces()[atePort1.Name]
			// dstEndPoint := []*ondatra.Interface{args.top.Interfaces()[atePort2.Name], args.top.Interfaces()[atePort3.Name]}

			testTraffic(t, true, args.ate, args.top, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, args, 0, weights...)
		})
	})

	t.Run("Remove protocol", func(t *testing.T) {
		defer configBasePBR(t, args.dut)

		var success bool
		success = t.Run("Pre-test config", func(t *testing.T) {
			r1 := &telemetry.NetworkInstance_PolicyForwarding_Policy_Rule{}
			r1.SequenceId = ygot.Uint32(1)
			r1.Ipv4 = &telemetry.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
				Protocol: telemetry.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP,
				DscpSet:  []uint8{10},
			}
			r1.Action = &telemetry.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String("TE")}
			args.dut.Config().NetworkInstance("default").PolicyForwarding().Policy(pbrName).Rule(1).Replace(t, r1)
		})
		if !success {
			t.Fatal("failed to apply pre-test configuration")
		}

		// FIXME: Workaround as XR isn't deleting DSCP 10 on replace
		defer deletePBRPolicyAndClassMaps(context.Background(), t, args.dut, pbrName, 4)
		defer deletePolicyFromInterface(ctx, t, args.dut, pbrName)

		// Remove existing match field
		success = t.Run("Delete", func(t *testing.T) {
			args.dut.Config().NetworkInstance("default").PolicyForwarding().Policy(pbrName).Rule(1).Ipv4().Protocol().Delete(t)
		})
		if !success {
			t.FailNow()
		}

		// // TODO: enabled once gNMI Get works on XR
		// t.Run("Verify deleted", func(t *testing.T) {
		// 	verifyConfigPbrMatchIpv4DscpSet(ctx, t, args, 2, []uint8{})
		// })

		t.Run("Expect traffic fail", func(t *testing.T) {
			srcEndPoint := args.top.Interfaces()[atePort1.Name]
			// dstEndPoint := []*ondatra.Interface{args.top.Interfaces()[atePort2.Name], args.top.Interfaces()[atePort3.Name]}

			// Expecting Traffic fail
			testTraffic(t, false, args.ate, args.top, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, args, 10, weights...)
		})
	})
}

// testModifyMatchField tests modifying existing match filed in the existing class-map and verify traffic
func testModifyMatchField(ctx context.Context, t *testing.T, args *testArgs) {
	defer configBasePBR(t, args.dut)
	defer flushSever(t, args)

	weights := []float64{10 * 15, 20 * 15, 30 * 15, 10 * 85, 20 * 85, 30 * 85, 40 * 85}

	configureBaseDoubleRecusionVip1Entry(ctx, t, args)
	configureBaseDoubleRecusionVip2Entry(ctx, t, args)
	configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)

	// Modify match field for protocol IPinIP class
	args.dut.Config().NetworkInstance("default").PolicyForwarding().Policy(pbrName).Rule(1).Ipv4().Protocol().Replace(t, telemetry.PacketMatchTypes_IP_PROTOCOL_IP_ICMP)

	// Create Traffic and check traffic
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	// dstEndPoint := []*ondatra.Interface{args.top.Interfaces()[atePort2.Name], args.top.Interfaces()[atePort3.Name]}

	// Expecting Traffic fail
	testTraffic(t, false, args.ate, args.top, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, args, 0, weights...)
}

// testAddMatchField tests adding new match field in the existing class-map and verify traffic
func testAddMatchField(ctx context.Context, t *testing.T, args *testArgs) {
	defer flushSever(t, args)

	weights := []float64{10 * 15, 20 * 15, 30 * 15, 10 * 85, 20 * 85, 30 * 85, 40 * 85}

	configureBaseDoubleRecusionVip1Entry(ctx, t, args)
	configureBaseDoubleRecusionVip2Entry(ctx, t, args)
	configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)

	t.Run("Add dscp-set", func(t *testing.T) {
		args.dut.Config().NetworkInstance("default").PolicyForwarding().Policy(pbrName).Rule(1).Ipv4().DscpSet().Replace(t, []uint8{10, 12})
		defer configBasePBR(t, args.dut)

		// // TODO: enabled once gNMI Get works on XR
		// t.Run("Verify added", func(t *testing.T) {
		// 	verifyConfigPbrMatchIpv4DscpSet(ctx, t, args, 1, []uint8{10, 12})
		// })

		// Create Traffic and check traffic
		t.Run("Verify traffic on DSCP 12", func(t *testing.T) {
			srcEndPoint := args.top.Interfaces()[atePort1.Name]
			// dstEndPoint := []*ondatra.Interface{args.top.Interfaces()[atePort2.Name], args.top.Interfaces()[atePort3.Name]}

			testTraffic(t, true, args.ate, args.top, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, args, 12, weights...)
		})

		// TODO: Make sure this is correct when current config failure is fixed.
		t.Run("Expect traffic fail without DSCP", func(t *testing.T) {
			srcEndPoint := args.top.Interfaces()[atePort1.Name]
			// dstEndPoint := []*ondatra.Interface{args.top.Interfaces()[atePort2.Name], args.top.Interfaces()[atePort3.Name]}

			// Expecting Traffic fail
			testTraffic(t, false, args.ate, args.top, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, args, 0, weights...)
		})
	})

	t.Run("Add protocol", func(t *testing.T) {
		args.dut.Config().NetworkInstance("default").PolicyForwarding().Policy(pbrName).Rule(2).Ipv4().Protocol().Replace(t, telemetry.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP)
		defer configBasePBR(t, args.dut)

		// Create Traffic and check traffic
		t.Run("Verify traffic", func(t *testing.T) {
			srcEndPoint := args.top.Interfaces()[atePort1.Name]
			// dstEndPoint := []*ondatra.Interface{args.top.Interfaces()[atePort2.Name], args.top.Interfaces()[atePort3.Name]}

			testTraffic(t, true, args.ate, args.top, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, args, 0, weights...)
		})
	})
}

// Function.PBR.:027 interface shut/unshut and verify traffic
func testTrafficFlapInterface(ctx context.Context, t *testing.T, args *testArgs) {
	defer flushSever(t, args)

	weights := []float64{10 * 15, 20 * 15, 30 * 15, 10 * 85, 20 * 85, 30 * 85, 40 * 85}

	configureBaseDoubleRecusionVip1Entry(ctx, t, args)
	configureBaseDoubleRecusionVip2Entry(ctx, t, args)
	configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)

	// Create Traffic and check traffic
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	testTraffic(t, true, args.ate, args.top, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, args, 0, weights...)

	// Flap Interface
	for _, interface_name := range args.interfaces.in {
		util.FlapInterface(t, args.dut, interface_name, 5)
	}
	// Verify Traffic again
	testTraffic(t, true, args.ate, args.top, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, args, 0, weights...)
}

// Function.PBR.:020 Verify PBR policy works with match DSCP and action VRF redirect
func testMatchDscpActionVRFRedirect(ctx context.Context, t *testing.T, args *testArgs) {
	defer flushSever(t, args)

	weights := []float64{10 * 15, 20 * 15, 30 * 15, 10 * 85, 20 * 85, 30 * 85, 40 * 85}

	configureBaseDoubleRecusionVip1Entry(ctx, t, args)
	configureBaseDoubleRecusionVip2Entry(ctx, t, args)
	configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)

	// Create Traffic and check traffic
	dscp := uint8(16)
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	testTraffic(t, true, args.ate, args.top, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, args, dscp, weights...)
}

func GetIpv4Acl(name string, sequenceId uint32, dscp uint8, action telemetry.E_Acl_FORWARDING_ACTION) *telemetry.Acl {
	acl := (&telemetry.Device{}).GetOrCreateAcl()
	aclSet := acl.GetOrCreateAclSet(name, telemetry.Acl_ACL_TYPE_ACL_IPV4)
	aclEntry := aclSet.GetOrCreateAclEntry(sequenceId)
	aclEntryIpv4 := aclEntry.GetOrCreateIpv4()
	aclEntryIpv4.Dscp = ygot.Uint8(dscp)
	aclEntryAction := aclEntry.GetOrCreateActions()
	aclEntryAction.ForwardingAction = action
	return acl
}

// Function.PBR.:024 Feature Interaction: configure ACL and PBR under same interface and verify behavior
func testAclAndPBRUnderSameInterface(ctx context.Context, t *testing.T, args *testArgs) {
	defer flushSever(t, args)

	weights := []float64{10 * 15, 20 * 15, 30 * 15, 10 * 85, 20 * 85, 30 * 85, 40 * 85}

	configureBaseDoubleRecusionVip1Entry(ctx, t, args)
	configureBaseDoubleRecusionVip2Entry(ctx, t, args)
	configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)

	// Create and apply ACL that allows DSCP16 traffic to ingress interface. Verify traffic passes.
	dscp := uint8(16)
	aclName := "dscp_pass"
	aclConfig := GetIpv4Acl(aclName, 10, dscp, telemetry.Acl_FORWARDING_ACTION_ACCEPT)
	args.dut.Config().Acl().Replace(t, aclConfig)
	args.dut.Config().Acl().Interface(args.interfaces.in[0]).IngressAclSet(aclName, telemetry.Acl_ACL_TYPE_ACL_IPV4).SetName().Replace(t, aclName)

	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	testTraffic(t, true, args.ate, args.top, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, args, dscp, weights...)

	//Create and apply ACL that drops DSCP16 traffic incoming on the ingress interface. Verify traffic drops.
	aclName = "dscp_drop"
	aclConfig = GetIpv4Acl(aclName, 10, dscp, telemetry.Acl_FORWARDING_ACTION_REJECT)
	args.dut.Config().Acl().Replace(t, aclConfig)
	args.dut.Config().Acl().Interface(args.interfaces.in[0]).IngressAclSet(aclName, telemetry.Acl_ACL_TYPE_ACL_IPV4).SetName().Replace(t, aclName)

	testTraffic(t, false, args.ate, args.top, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, args, dscp, weights...)

	// Cleanup
	args.dut.Config().Acl().Interface(args.interfaces.in[0]).IngressAclSet(aclName, telemetry.Acl_ACL_TYPE_ACL_IPV4).Delete(t)
	args.dut.Config().Acl().Delete(t)

}

func testPolicesReplace(ctx context.Context, t *testing.T, args *testArgs) {
	t.Skip()
	defer configBasePBR(t, args.dut)
	defer flushSever(t, args)

	weights := []float64{10 * 15, 20 * 15, 30 * 15, 10 * 85, 20 * 85, 30 * 85, 40 * 85}

	configureBaseDoubleRecusionVip1Entry(ctx, t, args)
	configureBaseDoubleRecusionVip2Entry(ctx, t, args)
	configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)

	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	testTraffic(t, true, args.ate, args.top, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, args, 0, weights...)

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
	p.Rule = map[uint32]*telemetry.NetworkInstance_PolicyForwarding_Policy_Rule{2: &r2, 3: &r3, 4: &r4}

	policy := telemetry.NetworkInstance_PolicyForwarding{}
	policy.Policy = map[string]*telemetry.NetworkInstance_PolicyForwarding_Policy{pbrName: &p}

	args.dut.Config().NetworkInstance("default").PolicyForwarding().Replace(t, &policy)

	testTraffic(t, false, args.ate, args.top, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, args, 0, weights...)
}

func testPolicyReplace(ctx context.Context, t *testing.T, args *testArgs) {
	t.Skip()
	defer configBasePBR(t, args.dut)
	defer flushSever(t, args)

	weights := []float64{10 * 15, 20 * 15, 30 * 15, 10 * 85, 20 * 85, 30 * 85, 40 * 85}

	configureBaseDoubleRecusionVip1Entry(ctx, t, args)
	configureBaseDoubleRecusionVip2Entry(ctx, t, args)
	configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)

	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	testTraffic(t, true, args.ate, args.top, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, args, 0, weights...)

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
	p.Rule = map[uint32]*telemetry.NetworkInstance_PolicyForwarding_Policy_Rule{2: &r2, 3: &r3, 4: &r4}

	args.dut.Config().NetworkInstance("default").PolicyForwarding().Policy(pbrName).Replace(t, &p)

	testTraffic(t, false, args.ate, args.top, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, args, 0, weights...)
}
