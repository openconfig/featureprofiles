// Package policy_test include test for PBR interactions with gribi
package policy_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/cisco/config"
	ciscoFlags "github.com/openconfig/featureprofiles/internal/cisco/flags"
	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	spb "github.com/openconfig/gnoi/system"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygot/ygot"
)

const (
	pbrName               = "PBR"
	PbrNameSrc            = "Pbr-SrcIp"
	PbrNameSrc2           = "Pbr-SrcIp-two"
	PbrNameDscp           = "Pbr-SrcIp-Dscp"
	SeqID                 = 1
	SeqID2                = 2
	SeqID3                = 3
	IxiaSrcip             = "198.51.100.0"
	IxiaSrcip2            = "198.61.100.0"
	Dscpval               = 10
	SourceAddress         = "198.51.100.0/32"
	SourceAddress2        = "198.61.100.0/32"
	protocolNum           = 4
	protocolNumv6         = 41
	rebootDelay           = 120
	oneSecondInNanoSecond = 1e9
)

var weights = []float64{10 * 15, 20 * 15, 30 * 15, 10 * 85, 20 * 85, 30 * 85, 40 * 85}

func configBasePBR(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	r1 := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
	r1.SequenceId = ygot.Uint32(1)
	r1.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
		Protocol: oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP,
	}
	r1.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String(*ciscoFlags.NonDefaultNetworkInstance)}

	r2 := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
	r2.SequenceId = ygot.Uint32(2)
	r2.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
		DscpSet: []uint8{*ygot.Uint8(16)},
	}
	r2.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String(*ciscoFlags.NonDefaultNetworkInstance)}

	r3 := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
	r3.SequenceId = ygot.Uint32(3)
	r3.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
		DscpSet: []uint8{*ygot.Uint8(18)},
	}
	r3.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String("VRF1")}

	r4 := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
	r4.SequenceId = ygot.Uint32(4)
	r4.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
		DscpSet: []uint8{*ygot.Uint8(48)},
	}
	r4.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String(*ciscoFlags.NonDefaultNetworkInstance)}

	p := oc.NetworkInstance_PolicyForwarding_Policy{}
	p.PolicyId = ygot.String(pbrName)
	p.Type = oc.Policy_Type_VRF_SELECTION_POLICY
	p.Rule = map[uint32]*oc.NetworkInstance_PolicyForwarding_Policy_Rule{1: &r1, 2: &r2, 3: &r3, 4: &r4}

	policy := oc.NetworkInstance_PolicyForwarding{}
	policy.Policy = map[string]*oc.NetworkInstance_PolicyForwarding_Policy{pbrName: &p}

	intfRef := &oc.NetworkInstance_PolicyForwarding_Interface_InterfaceRef{}
	intfRef.SetInterface("Bundle-Ether120")
	intfRef.SetSubinterface(0)
	policy.Interface = map[string]*oc.NetworkInstance_PolicyForwarding_Interface{
		"Bundle-Ether120.0": {
			InterfaceId:             ygot.String("Bundle-Ether120.0"),
			ApplyVrfSelectionPolicy: ygot.String(pbrName),
			InterfaceRef:            intfRef,
		},
	}

	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(*ciscoFlags.PbrInstance).PolicyForwarding().Config(), &policy)
}

func configNewPolicy(t *testing.T, dut *ondatra.DUTDevice, policyName string, dscp uint8) {
	r1 := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
	r1.SequenceId = ygot.Uint32(1)
	r1.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
		Protocol: oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP,
		DscpSet: []uint8{
			*ygot.Uint8(dscp),
		},
	}
	r1.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String(*ciscoFlags.NonDefaultNetworkInstance)}

	p := oc.NetworkInstance_PolicyForwarding_Policy{}
	p.PolicyId = ygot.String(policyName)
	p.Type = oc.Policy_Type_VRF_SELECTION_POLICY
	p.Rule = map[uint32]*oc.NetworkInstance_PolicyForwarding_Policy_Rule{1: &r1}

	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(*ciscoFlags.PbrInstance).PolicyForwarding().Policy(policyName).Config(), &p)
}

func configSrcIp(t *testing.T, dut *ondatra.DUTDevice, policyName string, srcAddr string) {
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	r1 := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
	seq_id := uint32(SeqID)
	r1.SequenceId = &seq_id
	r1.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
		SourceAddress: &srcAddr,
	}
	r1.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String(*ciscoFlags.NonDefaultNetworkInstance)}

	p := oc.NetworkInstance_PolicyForwarding_Policy{}
	p.PolicyId = ygot.String(policyName)
	p.Type = oc.Policy_Type_VRF_SELECTION_POLICY
	p.Rule = map[uint32]*oc.NetworkInstance_PolicyForwarding_Policy_Rule{1: &r1}
	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).PolicyForwarding().Policy(policyName).Config(), &p)
}
func configProtocolV6(t *testing.T, dut *ondatra.DUTDevice, policyName string, srcAddr string, dscp uint8, protocol uint8) {
	r1 := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
	seq_id := uint32(SeqID)
	r1.SequenceId = &seq_id
	r1.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
		SourceAddress: &srcAddr,
		DscpSet: []uint8{
			*ygot.Uint8(dscp),
		},
	}
	if protocol == protocolNumv6 {
		r1.Ipv4.Protocol = oc.UnionUint8(protocolNumv6)
	} else {
		r1.Ipv4.Protocol = oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP
	}

	r1.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String(*ciscoFlags.NonDefaultNetworkInstance)}

	p := oc.NetworkInstance_PolicyForwarding_Policy{}
	p.PolicyId = ygot.String(policyName)
	p.Type = oc.Policy_Type_VRF_SELECTION_POLICY
	p.Rule = map[uint32]*oc.NetworkInstance_PolicyForwarding_Policy_Rule{1: &r1}
	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).PolicyForwarding().Policy(policyName).Config(), &p)
}
func configSrcIpDscp(t *testing.T, dut *ondatra.DUTDevice, policyName string, dscp uint8, srcAddr string) {
	r1 := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
	seq_id := uint32(SeqID)
	r1.SequenceId = &seq_id
	r1.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
		Protocol: oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP,
		DscpSet: []uint8{
			*ygot.Uint8(dscp),
		},
		SourceAddress: &srcAddr,
	}
	r1.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String(*ciscoFlags.NonDefaultNetworkInstance)}

	p := oc.NetworkInstance_PolicyForwarding_Policy{}
	p.PolicyId = ygot.String(policyName)
	p.Type = oc.Policy_Type_VRF_SELECTION_POLICY
	p.Rule = map[uint32]*oc.NetworkInstance_PolicyForwarding_Policy_Rule{1: &r1}

	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).PolicyForwarding().Policy(policyName).Config(), &p)
}

func updateOnlySrcIp(t *testing.T, dut *ondatra.DUTDevice, policyName string, srcAddr string) {
	seq_id := uint32(SeqID)
	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).PolicyForwarding().Policy(policyName).Rule(seq_id).Ipv4().SourceAddress().Config(), srcAddr)
}
func replaceOnlySrcIp(t *testing.T, dut *ondatra.DUTDevice, policyName string, srcAddr string) {
	t.Helper()
	seq_id := uint32(SeqID)
	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).PolicyForwarding().Policy(policyName).Rule(seq_id).Ipv4().SourceAddress().Config(), srcAddr)
}
func replaceOnlyProtocol(t *testing.T, dut *ondatra.DUTDevice, policyName string) {
	t.Helper()
	seq_id := uint32(SeqID)
	gnmi.Replace[oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4_Protocol_Union](t, dut, gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).PolicyForwarding().Policy(policyName).Rule(seq_id).Ipv4().Protocol().Config(), oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP)
}
func replaceSrcIpRule(t *testing.T, dut *ondatra.DUTDevice, policyName string, srcAddr string, SeqID uint32) {

	t.Helper()
	r1 := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
	seq_id := uint32(SeqID)
	r1.SequenceId = &seq_id
	r1.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
		SourceAddress: &srcAddr,
	}
	r1.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String(*ciscoFlags.NonDefaultNetworkInstance)}

	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(*ciscoFlags.PbrInstance).PolicyForwarding().Policy(policyName).Rule(seq_id).Config(), &r1)
}
func replaceSrcpmap(t *testing.T, dut *ondatra.DUTDevice, policyName string, srcAddr string, SeqID uint32) {

	t.Helper()
	r1 := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
	seq_id := uint32(SeqID)
	r1.SequenceId = &seq_id
	r1.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
		SourceAddress: &srcAddr,
	}
	r1.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String(*ciscoFlags.NonDefaultNetworkInstance)}

	p := oc.NetworkInstance_PolicyForwarding_Policy{}
	p.PolicyId = ygot.String(policyName)
	p.Type = oc.Policy_Type_VRF_SELECTION_POLICY
	p.Rule = map[uint32]*oc.NetworkInstance_PolicyForwarding_Policy_Rule{1: &r1}

	policy := oc.NetworkInstance_PolicyForwarding{}
	policy.Policy = map[string]*oc.NetworkInstance_PolicyForwarding_Policy{pbrName: &p}

	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(*ciscoFlags.PbrInstance).PolicyForwarding().Config(), &policy)
}
func configUpdateRule(t *testing.T, dut *ondatra.DUTDevice, policyName string, dscp uint8) {
	r2 := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
	seq_id := uint32(SeqID2)
	r2.SequenceId = &seq_id
	r2.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
		DscpSet: []uint8{
			*ygot.Uint8(dscp),
		},
	}
	r2.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String(*ciscoFlags.NonDefaultNetworkInstance)}

	r3 := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
	seq_id2 := uint32(SeqID3)
	r3.SequenceId = &seq_id2
	r3.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
		Protocol: oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP,
	}
	r3.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String(*ciscoFlags.NonDefaultNetworkInstance)}

	p := oc.NetworkInstance_PolicyForwarding_Policy{}
	p.PolicyId = ygot.String(policyName)
	p.Type = oc.Policy_Type_VRF_SELECTION_POLICY
	p.Rule = map[uint32]*oc.NetworkInstance_PolicyForwarding_Policy_Rule{2: &r2, 3: &r3}

	policy := oc.NetworkInstance_PolicyForwarding{}
	policy.Policy = map[string]*oc.NetworkInstance_PolicyForwarding_Policy{policyName: &p}

	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(*ciscoFlags.PbrInstance).PolicyForwarding().Config(), &policy)
}

func configNewRule(t *testing.T, dut *ondatra.DUTDevice, policyName string, ruleID uint32, protocol uint8, dscp ...uint8) {
	r := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
	r.SequenceId = ygot.Uint32(ruleID)
	r.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
		DscpSet: dscp,
	}
	if protocol == 4 {
		r.Ipv4.Protocol = oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP
	}
	r.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String(*ciscoFlags.NonDefaultNetworkInstance)}
	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(*ciscoFlags.PbrInstance).PolicyForwarding().Policy(policyName).Rule(ruleID).Config(), &r)
}

func generatePhysicalInterfaceConfig(name, ipv4 string, prefixlen uint8) *oc.Interface {
	i := &oc.Interface{}
	i.Name = ygot.String(name)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	s := i.GetOrCreateSubinterface(0)
	s4 := s.GetOrCreateIpv4()
	a := s4.GetOrCreateAddress(ipv4)
	a.PrefixLength = ygot.Uint8(prefixlen)
	return i
}

func generateBundleMemberInterfaceConfig(name, bundleID string) *oc.Interface {
	i := &oc.Interface{Name: ygot.String(name)}
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	e := i.GetOrCreateEthernet()
	e.AutoNegotiate = ygot.Bool(false)
	e.AggregateId = ygot.String(bundleID)
	return i
}

func configPBRunderInterface(t *testing.T, interfaceName, policyName string) {
	t.Log("configPBRunderInterface")
	dut := ondatra.DUT(t, "dut")
	pfPath := gnmi.OC().NetworkInstance(*ciscoFlags.PbrInstance).PolicyForwarding().Interface(interfaceName + ".0")
	pfCfg := getPolicyForwardingInterfaceConfig(t, policyName, interfaceName)
	gnmi.Replace(t, dut, pfPath.Config(), pfCfg)

}

func unconfigPBRunderInterface(t *testing.T, args *testArgs, interfaceName string) {
	t.Log("unconfigPBRunderInterface")
	gnmi.Delete(t, args.dut, gnmi.OC().NetworkInstance(*ciscoFlags.PbrInstance).PolicyForwarding().Interface(interfaceName+".0").Config())
}

func reloadDevice(t *testing.T, dut *ondatra.DUTDevice) {
	gnoiClient := dut.RawAPIs().GNOI(t)
	_, err := gnoiClient.System().Reboot(context.Background(), &spb.RebootRequest{
		Method:  spb.RebootMethod_COLD,
		Delay:   0,
		Message: "Reboot chassis without delay",
		Force:   true,
	})
	if err != nil {
		t.Fatalf("Reboot failed %v", err)
	}
	startReboot := time.Now()
	const maxRebootTime = 30
	t.Logf("Wait for DUT to boot up by polling the telemetry output.")
	for {
		var currentTime string
		t.Logf("Time elapsed %.2f minutes since reboot started.", time.Since(startReboot).Minutes())

		time.Sleep(3 * time.Minute)
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			currentTime = gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State())
		}); errMsg != nil {
			t.Logf("Got testt.CaptureFatal errMsg: %s, keep polling ...", *errMsg)
		} else {
			t.Logf("Device rebooted successfully with received time: %v", currentTime)
			break
		}

		if uint64(time.Since(startReboot).Minutes()) > maxRebootTime {
			t.Fatalf("Check boot time: got %v, want < %v", time.Since(startReboot), maxRebootTime)
		}
	}
	t.Logf("Device boot time: %.2f minutes", time.Since(startReboot).Minutes())
}

// Remove flowspec and add as pbr
func convertFlowspecToPBR(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice) {

	t.Log("Remove Flowspec Config and add HW Module Config")
	configToChange := "no flowspec \nhw-module profile pbr vrf-redirect\n"
	util.GNMIWithText(ctx, t, dut, configToChange)

	t.Log("Reload the router to activate hw module config")
	reloadDevice(t, dut)
	time.Sleep(2 * time.Minute)
	startGribiClient(t)

	t.Log("Configure PBR policy and Apply it under interface")
	configBasePBR(t, dut)
	getPolicyForwardingInterfaceConfig(t, pbrName, "Bundle-Ether120")
}

func getPolicyForwardingInterfaceConfig(t *testing.T, policyName, intf string) *oc.NetworkInstance_PolicyForwarding_Interface {

	t.Logf("Applying forwarding policy on interface %v ... ", intf)
	d := &oc.Root{}
	pfCfg := d.GetOrCreateNetworkInstance(*ciscoFlags.PbrInstance).GetOrCreatePolicyForwarding().GetOrCreateInterface(intf + ".0")
	pfCfg.ApplyVrfSelectionPolicy = ygot.String(policyName)
	pfCfg.GetOrCreateInterfaceRef().Interface = ygot.String(intf)
	pfCfg.GetOrCreateInterfaceRef().Subinterface = ygot.Uint32(0)
	return pfCfg

}

// Remove the policy under physical interface and add the related physical interface under bundle interface which use the same PBR policy
func movePhysicalToBundle(ctx context.Context, t *testing.T, args *testArgs, samePolicy bool) {
	configBasePBR(t, args.dut)

	physicalInterface := sortPorts(args.dut.Ports())[0].Name()
	physicalInterfaceConfig := gnmi.OC().Interface(physicalInterface)
	configToChange := "interface " + physicalInterface + "\nno shutdown\nno bundle id 120 mode on\n"
	util.GNMIWithText(ctx, t, args.dut, configToChange)

	// Configure the physcial interface
	config := generatePhysicalInterfaceConfig(physicalInterface, "192.192.192.1", 24)
	gnmi.Replace(t, args.dut, physicalInterfaceConfig.Config(), config)

	// Configure policy on the physical interface and bunlde interface
	policyName := pbrName
	if !samePolicy {
		policyName = "new-PBR"
		configNewPolicy(t, args.dut, policyName, 0)
	}
	configPBRunderInterface(t, physicalInterface, policyName)
	configPBRunderInterface(t, args.interfaces.in[0], policyName)

	gnmi.Delete(t, args.dut, gnmi.OC().Interface(physicalInterface).Subinterface(0).Ipv4().Config())
	gnmi.Delete(t, args.dut, gnmi.OC().NetworkInstance(*ciscoFlags.PbrInstance).PolicyForwarding().Interface(physicalInterface+".0").Config())

	// Remove the interface from physical to bundle interface
	memberConfig := generateBundleMemberInterfaceConfig(physicalInterface, args.interfaces.in[0])
	gnmi.Replace(t, args.dut, physicalInterfaceConfig.Config(), memberConfig)

	// Program GRIBI entry on the router
	defer flushServer(t, args)

	// weights := []float64{10 * 15, 20 * 15, 30 * 15, 10 * 85, 20 * 85, 30 * 85, 40 * 85}

	configureBaseDoubleRecusionVip1Entry(ctx, t, args)
	configureBaseDoubleRecusionVip2Entry(ctx, t, args)
	configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)

	// Create Traffic and check traffic
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	// dstEndPoint := []*ondatra.Interface{args.top.Interfaces()[atePort2.Name], args.top.Interfaces()[atePort3.Name]}

	testTraffic(t, true, args.ate, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, 0)
}

func testMovePhysicalToBundleWithSamePolicy(ctx context.Context, t *testing.T, args *testArgs) {
	movePhysicalToBundle(ctx, t, args, true)
}

func testMovePhysicalToBundleWithDifferentPolicy(ctx context.Context, t *testing.T, args *testArgs) {
	movePhysicalToBundle(ctx, t, args, false)
}

// testChangePBRUnderInterface tests changing the PBR policy under the interface
func testChangePBRUnderInterface(ctx context.Context, t *testing.T, args *testArgs) {
	defer configBasePBR(t, args.dut)
	// Program GRIBI entry on the router
	defer flushServer(t, args)

	// weights := []float64{10 * 15, 20 * 15, 30 * 15, 10 * 85, 20 * 85, 30 * 85, 40 * 85}

	configureBaseDoubleRecusionVip1Entry(ctx, t, args)
	configureBaseDoubleRecusionVip2Entry(ctx, t, args)
	configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)

	newPbrName := "new-PBR"
	dscpVal := uint8(10)

	// Configure new policy that matches the dscp and protocol IPinIP
	configNewPolicy(t, args.dut, newPbrName, dscpVal)
	defer gnmi.Delete(t, args.dut, gnmi.OC().NetworkInstance(*ciscoFlags.PbrInstance).PolicyForwarding().Policy(newPbrName).Config())

	// Change policy on the bunlde interface
	unconfigPBRunderInterface(t, args, args.interfaces.in[0])
	configPBRunderInterface(t, args.interfaces.in[0], newPbrName)
	defer configPBRunderInterface(t, args.interfaces.in[0], pbrName)

	// Create Traffic and check traffic
	srcEndPoint := args.top.Interfaces()[atePort1.Name]

	testTraffic(t, true, args.ate, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, dscpVal)
}

// testIPv6InIPv4Traffic tests sending IPv6inIPv4 and verify it is not matched by IPinIP
func testIPv6InIPv4Traffic(ctx context.Context, t *testing.T, args *testArgs) {
	defer configBasePBR(t, args.dut)
	// Program GRIBI entry on the router
	defer flushServer(t, args)

	// weights := []float64{10 * 15, 20 * 15, 30 * 15, 10 * 85, 20 * 85, 30 * 85, 40 * 85}

	configureBaseDoubleRecusionVip1Entry(ctx, t, args)
	configureBaseDoubleRecusionVip2Entry(ctx, t, args)
	configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)

	// Create Traffic and check traffic
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	// dstEndPoint := []*ondatra.Interface{args.top.Interfaces()[atePort2.Name], args.top.Interfaces()[atePort3.Name]}

	testTrafficWithInnerIPv6(t, false, args.ate, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, 0)
}

func configurePBR(t *testing.T, dut *ondatra.DUTDevice, name, networkInstance, iptype string, index uint32, protocol oc.E_PacketMatchTypes_IP_PROTOCOL, dscpset []uint8) {
	r1 := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
	r1.SequenceId = ygot.Uint32(index)
	if iptype == "ipv4" {
		r1.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
			Protocol: protocol,
		}
		if len(dscpset) > 0 {
			r1.Ipv4.DscpSet = dscpset
		}
	}
	if iptype == "ipv6" {
		r1.Ipv6 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv6{
			Protocol: protocol,
		}
		if len(dscpset) > 0 {
			r1.Ipv6.DscpSet = dscpset
		}
	}
	r1.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String(networkInstance)}
	policy := oc.NetworkInstance_PolicyForwarding{}
	p := policy.GetOrCreatePolicy(name)
	p.Type = oc.Policy_Type_VRF_SELECTION_POLICY
	p.AppendRule(&r1)
	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(*ciscoFlags.PbrInstance).PolicyForwarding().Config(), &policy)
	//dut.Config().NetworkInstance(*ciscoFlags.PbrInstance).PolicyForwarding().Replace(t, &policy)
}

func configurePBRRule(t *testing.T, dut *ondatra.DUTDevice, name, networkInstance, iptype string, index uint32, protocol oc.E_PacketMatchTypes_IP_PROTOCOL, dscpset []uint8) {
	r1 := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
	r1.SequenceId = ygot.Uint32(index)
	if iptype == "ipv4" {
		r1.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
			Protocol: protocol,
		}
		if len(dscpset) > 0 {
			r1.Ipv4.DscpSet = dscpset
		}
	}
	if iptype == "ipv6" {
		r1.Ipv6 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv6{
			Protocol: protocol,
		}
		if len(dscpset) > 0 {
			r1.Ipv6.DscpSet = dscpset
		}
	}
	r1.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String(networkInstance)}
	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(*ciscoFlags.PbrInstance).PolicyForwarding().Policy(name).Rule(index).Config(), &r1)
}

func configureL2PBR(t *testing.T, dut *ondatra.DUTDevice, name, networkInstance, iptype string, index uint32) {
	r1 := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
	r1.SequenceId = ygot.Uint32(index)
	if iptype == "ipv4" {
		r1.L2 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_L2{
			Ethertype: oc.PacketMatchTypes_ETHERTYPE_ETHERTYPE_IPV4,
		}
	}
	if iptype == "ipv6" {
		r1.L2 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_L2{
			Ethertype: oc.PacketMatchTypes_ETHERTYPE_ETHERTYPE_IPV6,
		}
	}
	r1.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String(networkInstance)}
	p := oc.NetworkInstance_PolicyForwarding_Policy{}
	p.PolicyId = ygot.String(name)
	p.Type = oc.Policy_Type_VRF_SELECTION_POLICY
	p.AppendRule(&r1)
	policy := oc.NetworkInstance_PolicyForwarding{}
	policy.Policy = map[string]*oc.NetworkInstance_PolicyForwarding_Policy{pbrName: &p}

	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(*ciscoFlags.PbrInstance).PolicyForwarding().Config(), &policy)
}

func configureL2PBRRule(t *testing.T, dut *ondatra.DUTDevice, name, networkInstance, iptype string, index uint32) {
	r1 := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
	r1.SequenceId = ygot.Uint32(index)
	if iptype == "ipv4" {
		r1.L2 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_L2{
			Ethertype: oc.PacketMatchTypes_ETHERTYPE_ETHERTYPE_IPV4,
		}
	}
	if iptype == "ipv6" {
		r1.L2 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_L2{
			Ethertype: oc.PacketMatchTypes_ETHERTYPE_ETHERTYPE_IPV6,
		}
	}
	r1.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String(networkInstance)}
	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(*ciscoFlags.PbrInstance).PolicyForwarding().Policy(name).Rule(index).Config(), &r1)
}

func getBoundedFlow(t *testing.T, ate *ondatra.ATEDevice, srcEndpoint, dstEndPoint ondatra.Endpoint, flowName string, dscp uint8, ttl ...uint8) *ondatra.Flow {

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
	flow.WithFrameRateFPS(*ciscoFlags.FlowFps)
	flow.WithFrameSize(*ciscoFlags.FrameSize)
	return flow
}

func getBoundedFlowIpv6(t *testing.T, ate *ondatra.ATEDevice, srcEndpoint, dstEndPoint ondatra.Endpoint, flowName string, dscp uint8) *ondatra.Flow {

	flow := ate.Traffic().NewFlow(flowName)
	t.Logf("Setting up flow -> %s", flowName)
	ethheader := ondatra.NewEthernetHeader()
	ipheader1 := ondatra.NewIPv6Header().WithDSCP(dscp)
	flow.WithHeaders(ethheader, ipheader1)
	flow.WithSrcEndpoints(srcEndpoint)
	flow.WithDstEndpoints(dstEndPoint)
	flow.WithFrameRateFPS(*ciscoFlags.FlowFps)
	flow.WithFrameSize(*ciscoFlags.FrameSize)
	return flow
}

func getBoundedFlowIPinIP(t *testing.T, ate *ondatra.ATEDevice, srcEndpoint, dstEndPoint ondatra.Endpoint, flowName string, dscp uint8, ttl ...uint8) *ondatra.Flow {

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
	flow.WithFrameRateFPS(*ciscoFlags.FlowFps)
	flow.WithFrameSize(*ciscoFlags.FrameSize)
	return flow
}

// deletePolicyFromInterface function removes the pbr policy from Bundle-Ether120 using CLI options.
// This is a temporary fix for accommodating various types of pbr policies on the interface
func deletePolicyFromInterface(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, policyName string) {
	configToChange := fmt.Sprintf("no interface Bundle-Ether120  service-policy type pbr input %s\n", policyName)
	config.TextWithGNMI(ctx, t, dut, configToChange)
}

// deletePBRPolicyAndClassMaps function deletes pbr policy-map and class-map configuration using CLI.
// This is a temporary fix to cleanup the configurations.
func deletePBRPolicyAndClassMaps(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, policyName string, index int) {
	configToChange := fmt.Sprintf("no policy-map type pbr %s\n", policyName)
	for i := 1; i <= index; i++ {
		configToChange = configToChange + fmt.Sprintf("no class-map type traffic match-all %d_%s\n", i, policyName)
	}
	config.TextWithGNMI(ctx, t, dut, configToChange)
	//dut.Config().NetworkInstance(*ciscoFlags.PbrInstance).PolicyForwarding().Policy(policyName).Delete(t)
}

func deletePBRPolicy(t *testing.T, dut *ondatra.DUTDevice, policyName string) {
	gnmi.Delete(t, dut, gnmi.OC().NetworkInstance(*ciscoFlags.PbrInstance).PolicyForwarding().Policy(policyName).Config())
}

func testDscpProtocolBasedVRFSelection(ctx context.Context, t *testing.T, args *testArgs) {
	defer configBasePBR(t, args.dut)
	t.Log("RT-3.1 :Protocol, DSCP-based VRF Selection - ensure that protocol and DSCP based VRF selection is configured correctly")

	srcEndPoint := args.top.Interfaces()["atePort1"]
	dstEndPointVlan10 := args.top.Interfaces()["atePort2Vlan10"]
	dstEndPointVlan20 := args.top.Interfaces()["atePort2Vlan20"]

	//Case1 - Matching ipv4 protocol to VRF10. Dropping IPv6 traffic in VRF10.
	//Create IPV4 and IPv6 flows for VLAN10 with DSCP0.
	ipv4vlan10flow := getBoundedFlow(t, args.ate, srcEndPoint, dstEndPointVlan10, "ipv4vlan10flow", 0)
	ipv6vlan10flow := getBoundedFlowIpv6(t, args.ate, srcEndPoint, dstEndPointVlan10, "ipv6vlan10flow", 0)
	t.Run("RT-3.1 Case1", func(t *testing.T) {
		configureL2PBR(t, args.dut, "L2", "VRF10", "ipv4", 1)
		configPBRunderInterface(t, args.interfaces.in[0], "L2")

		testTrafficForFlows(t, args.ate, true, 0.90, ipv4vlan10flow)
		testTrafficForFlows(t, args.ate, false, 0.90, ipv6vlan10flow)
	})

	//Case3 - Matching IPv4 protocol to VRF10, IPv6 protocol to VRF20. Dropping IPv6 traffic in VRF10 and IPv4 in VRF20.
	//Create IPv4 and IPv6 flows for VLAN20 with DSCP0.
	ipv4vlan20flow := getBoundedFlow(t, args.ate, srcEndPoint, dstEndPointVlan20, "ipv4vlan20flow", 0)
	ipv6vlan20flow := getBoundedFlowIpv6(t, args.ate, srcEndPoint, dstEndPointVlan20, "ipv6vlan20flow", 0)

	t.Run("RT-3.1 Case3", func(t *testing.T) {
		configureL2PBRRule(t, args.dut, "L2", "VRF20", "ipv6", 2)

		testTrafficForFlows(t, args.ate, true, 0.90, ipv4vlan10flow, ipv6vlan20flow)
		testTrafficForFlows(t, args.ate, false, 0.90, ipv6vlan10flow, ipv4vlan20flow)

		//cleanup
		unconfigPBRunderInterface(t, args, args.interfaces.in[0])
		deletePBRPolicy(t, args.dut, "L2")
	})

	//Case2 - Match IPinIP protocol to VRF10. Drop IPv4 and IPv6 traffic in VRF10.
	//Create IPinIP flow for VLAN10 with DSCP0.
	ipinipvlan10flow := getBoundedFlowIPinIP(t, args.ate, srcEndPoint, dstEndPointVlan10, "ipv4inipv4v10flow0", 0)
	t.Run("RT-3.1 Case2", func(t *testing.T) {
		configurePBR(t, args.dut, "L3", "VRF10", "ipv4", 1, oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP, []uint8{})
		configPBRunderInterface(t, args.interfaces.in[0], "L3")

		testTrafficForFlows(t, args.ate, true, 0.90, ipinipvlan10flow)
		testTrafficForFlows(t, args.ate, false, 0.90, ipv4vlan10flow, ipv6vlan10flow)
	})

	//Case4 - Match IPinIP and single DSCP46 to VRF10. Drop DSCP0 in VRF10.
	//Create IPinIP flow with DSCP46 for VLAN10.
	ipinipvlan10flowd46 := getBoundedFlowIPinIP(t, args.ate, srcEndPoint, dstEndPointVlan10, "ipv4inipv4v10flow46", 46)
	t.Run("RT-3.1 Case4", func(t *testing.T) {
		configurePBR(t, args.dut, "L3", "VRF10", "ipv4", 1, oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP, []uint8{46})

		testTrafficForFlows(t, args.ate, false, 0.90, ipinipvlan10flow)
		testTrafficForFlows(t, args.ate, true, 0.90, ipinipvlan10flowd46)
	})

	//Case5 - Match IPinIP and single DSCP46, DSCP42 to VRF10. Drop DSCP0 in VRF10.
	//Create IPinIP flow with DSCP42 for VLAN10.
	ipinipvlan10flowd42 := getBoundedFlowIPinIP(t, args.ate, srcEndPoint, dstEndPointVlan10, "ipv4inipv4v10flow42", 42)
	t.Run("RT-3.1 Case5", func(t *testing.T) {
		configurePBR(t, args.dut, "L3", "VRF10", "ipv4", 1, oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP, []uint8{42, 46})

		testTrafficForFlows(t, args.ate, false, 0.90, ipinipvlan10flow)
		testTrafficForFlows(t, args.ate, true, 0.90, ipinipvlan10flowd46, ipinipvlan10flowd42)

		//cleanup
		unconfigPBRunderInterface(t, args, args.interfaces.in[0])
		deletePBRPolicy(t, args.dut, "L3")
	})
}

func testMultipleDscpProtocolRuleBasedVRFSelection(ctx context.Context, t *testing.T, args *testArgs) {
	//defer configBasePBR(t, args.dut)
	t.Log("RT-3.2 : Multiple <Protocol, DSCP> Rules for VRF Selection - ensure that multiple VRF selection rules are matched correctly")

	srcEndPoint := args.top.Interfaces()["atePort1"]
	dstEndPointVlan10 := args.top.Interfaces()["atePort2Vlan10"]
	dstEndPointVlan20 := args.top.Interfaces()["atePort2Vlan20"]
	dstEndPointVlan30 := args.top.Interfaces()["atePort2Vlan30"]

	//Case1 - Ensure matching IPinIP with DSCP (10 - VRF10, 20- VRF20, 30-VRF30) traffic reaches appropriate VLAN.
	//Create IPinIP DSCP10, DSCP20, DSCP30 flows for VLAN10, VLAN20 and VLAN30 respectively.
	ipinipd10 := getBoundedFlowIPinIP(t, args.ate, srcEndPoint, dstEndPointVlan10, "ipvinipd10", 10)
	ipinipd20 := getBoundedFlowIPinIP(t, args.ate, srcEndPoint, dstEndPointVlan20, "ipvinipd20", 20)
	ipinipd30 := getBoundedFlowIPinIP(t, args.ate, srcEndPoint, dstEndPointVlan30, "ipvinipd30", 30)

	t.Run("RT-3.2 Case1", func(t *testing.T) {

		configurePBR(t, args.dut, "L3", "VRF10", "ipv4", 1, oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP, []uint8{10})
		configurePBRRule(t, args.dut, "L3", "VRF20", "ipv4", 2, oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP, []uint8{20})
		configurePBRRule(t, args.dut, "L3", "VRF30", "ipv4", 3, oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP, []uint8{30})
		configPBRunderInterface(t, args.interfaces.in[0], "L3")

		testTrafficForFlows(t, args.ate, true, 0.90, ipinipd10, ipinipd20, ipinipd30)
	})

	//Case2 - Ensure matching IPinIP with DSCP (10-12 - VRF10, 20-22- VRF20, 30-32-VRF30) traffic reaches to appropriate VLAN.
	//Create IPinIP flows with DSCP11-12 for VLAN10, DSCP21-22 for VLAN20, DSCP31-32 for VLAN30.
	//Reuse IPinIP flows for DSCP10, DSCP20 and DSCP30.
	ipinipd11 := getBoundedFlowIPinIP(t, args.ate, srcEndPoint, dstEndPointVlan10, "ipvinipd11", 11)
	ipinipd12 := getBoundedFlowIPinIP(t, args.ate, srcEndPoint, dstEndPointVlan10, "ipvinipd12", 12)

	ipinipd21 := getBoundedFlowIPinIP(t, args.ate, srcEndPoint, dstEndPointVlan20, "ipvinipd21", 21)
	ipinipd22 := getBoundedFlowIPinIP(t, args.ate, srcEndPoint, dstEndPointVlan20, "ipvinipd22", 22)

	ipinipd31 := getBoundedFlowIPinIP(t, args.ate, srcEndPoint, dstEndPointVlan30, "ipvinipd31", 31)
	ipinipd32 := getBoundedFlowIPinIP(t, args.ate, srcEndPoint, dstEndPointVlan30, "ipvinipd32", 32)

	t.Run("RT-3.2 Case2", func(t *testing.T) {
		configurePBR(t, args.dut, "L3", "VRF10", "ipv4", 1, oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP, []uint8{10, 11, 12})
		configurePBRRule(t, args.dut, "L3", "VRF20", "ipv4", 2, oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP, []uint8{20, 21, 22})
		configurePBRRule(t, args.dut, "L3", "VRF30", "ipv4", 3, oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP, []uint8{30, 31, 32})

		testTrafficForFlows(t, args.ate, true, 0.90,
			ipinipd10, ipinipd11, ipinipd12,
			ipinipd20, ipinipd21, ipinipd22,
			ipinipd30, ipinipd31, ipinipd32)
		//cleanup
		unconfigPBRunderInterface(t, args, args.interfaces.in[0])
		deletePBRPolicy(t, args.dut, "L3")

	})

	//Case3 - Ensure first matching of IPinIP with DSCP (10-12 - VRF10, 10-12 - VRF20) rule takes precedence.
	//Create IPinIP DSCP10-12 flows for VLAN20. Reuse DSCP10-12 flows for VLAN10.
	ipinipd10v20 := getBoundedFlowIPinIP(t, args.ate, srcEndPoint, dstEndPointVlan20, "ipvinipd10v20", 10)
	ipinipd11v20 := getBoundedFlowIPinIP(t, args.ate, srcEndPoint, dstEndPointVlan20, "ipvinipd11v20", 11)
	ipinipd12v20 := getBoundedFlowIPinIP(t, args.ate, srcEndPoint, dstEndPointVlan20, "ipvinipd12v20", 12)

	t.Run("RT-3.2 Case3", func(t *testing.T) {
		configurePBR(t, args.dut, "L3", "VRF10", "ipv4", 1, oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP, []uint8{10, 11, 12})
		configurePBRRule(t, args.dut, "L3", "VRF20", "ipv4", 2, oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP, []uint8{10, 11, 12})
		configPBRunderInterface(t, args.interfaces.in[0], "L3")

		testTrafficForFlows(t, args.ate, true, 0.90, ipinipd10, ipinipd11, ipinipd12)
		testTrafficForFlows(t, args.ate, false, 0.90, ipinipd10v20, ipinipd11v20, ipinipd12v20)

		//cleanup
		unconfigPBRunderInterface(t, args, args.interfaces.in[0])
		deletePBRPolicy(t, args.dut, "L3")
	})

	//Case4 - Ensure matching IPinIP to VRF10, IPinIP with DSCP20 to VRF20 causes unspecified DSCP IPinIP traffic to match VRF10.
	//Reuse ipinipd10, ipinipd11, ipinipd12 flows to match IPinIP to VRF10
	//Reuse ipinipd20 flow to match IPinIP with DSCP20 to VRF20
	//Reuse ipinipd10v20, ipinipd11v20, ipinipd12v20 flows to show they fail for VRF20
	t.Run("RT-3.2 Case4", func(t *testing.T) {
		configurePBR(t, args.dut, "L3", "VRF10", "ipv4", 1, oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP, []uint8{})
		configurePBRRule(t, args.dut, "L3", "VRF20", "ipv4", 2, oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP, []uint8{20})
		configPBRunderInterface(t, args.interfaces.in[0], "L3")

		testTrafficForFlows(t, args.ate, true, 0.90, ipinipd10, ipinipd11, ipinipd12)
		testTrafficForFlows(t, args.ate, false, 0.90, ipinipd10v20, ipinipd11v20, ipinipd12v20, ipinipd20)

		//cleanup
		unconfigPBRunderInterface(t, args, args.interfaces.in[0])
		deletePBRPolicy(t, args.dut, "L3")
	})
}

// testRemoveClassMap tests removing existing class-map which is not related to IPinIP match and verify traffic
func testRemoveClassMap(ctx context.Context, t *testing.T, args *testArgs) {
	defer configBasePBR(t, args.dut)
	defer flushServer(t, args)

	// weights := []float64{10 * 15, 20 * 15, 30 * 15, 10 * 85, 20 * 85, 30 * 85, 40 * 85}

	args.clientA.StartWithNoCache(t)
	args.clientA.BecomeLeader(t)
	configureBaseDoubleRecusionVip1Entry(ctx, t, args)
	configureBaseDoubleRecusionVip2Entry(ctx, t, args)
	configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)

	// Remove existing class map
	gnmi.Delete(t, args.dut, gnmi.OC().NetworkInstance(*ciscoFlags.PbrInstance).PolicyForwarding().Policy(pbrName).Rule(1).Config())

	// Create Traffic and check traffic
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	// dstEndPoint := []*ondatra.Interface{args.top.Interfaces()[atePort2.Name], args.top.Interfaces()[atePort3.Name]}

	testTraffic(t, false, args.ate, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, 0)
}

func testChangeAction(ctx context.Context, t *testing.T, args *testArgs) {
	defer configBasePBR(t, args.dut)
	defer flushServer(t, args)

	// weights := []float64{10 * 15, 20 * 15, 30 * 15, 10 * 85, 20 * 85, 30 * 85, 40 * 85}

	args.clientA.StartWithNoCache(t)
	args.clientA.BecomeLeader(t)
	configureBaseDoubleRecusionVip1Entry(ctx, t, args)
	configureBaseDoubleRecusionVip2Entry(ctx, t, args)
	configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)

	// Change action for matching protocol IPinIP class
	gnmi.Replace(t, args.dut, gnmi.OC().NetworkInstance(*ciscoFlags.PbrInstance).PolicyForwarding().Policy(pbrName).Rule(1).Action().NetworkInstance().Config(), *ygot.String("VRF1"))

	// Create Traffic and check traffic
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	// dstEndPoint := []*ondatra.Interface{args.top.Interfaces()[atePort2.Name], args.top.Interfaces()[atePort3.Name]}

	// Expecting Traffic fail
	testTraffic(t, false, args.ate, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, 0)
}

func testAddClassMap(ctx context.Context, t *testing.T, args *testArgs) {
	defer configBasePBR(t, args.dut)
	defer flushServer(t, args)

	ruleID := uint32(10)
	dscp := uint8(32)
	configNewRule(t, args.dut, pbrName, ruleID, 4, dscp)
	defer gnmi.Delete(t, args.dut, gnmi.OC().NetworkInstance(*ciscoFlags.PbrInstance).PolicyForwarding().Policy(pbrName).Rule(ruleID).Config())

	// weights := []float64{10 * 15, 20 * 15, 30 * 15, 10 * 85, 20 * 85, 30 * 85, 40 * 85}

	args.clientA.StartWithNoCache(t)
	args.clientA.BecomeLeader(t)
	configureBaseDoubleRecusionVip1Entry(ctx, t, args)
	configureBaseDoubleRecusionVip2Entry(ctx, t, args)
	configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)

	// Change action for matching protocol IPinIP class
	gnmi.Replace(t, args.dut, gnmi.OC().NetworkInstance(*ciscoFlags.PbrInstance).PolicyForwarding().Policy(pbrName).Rule(1).Action().NetworkInstance().Config(), *ygot.String("VRF1"))

	// Create Traffic and check traffic
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	// dstEndPoint := []*ondatra.Interface{args.top.Interfaces()[atePort2.Name], args.top.Interfaces()[atePort3.Name]}

	// Expecting Traffic fail
	testTraffic(t, false, args.ate, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, dscp)
}

// testUnconfigPBRUnderBundleInterface tests unconfiguring the PBR policy under a bundle interface
func testUnconfigPBRUnderBundleInterface(ctx context.Context, t *testing.T, args *testArgs) {
	defer configBasePBR(t, args.dut)
	// Program GRIBI entry on the router
	defer flushServer(t, args)

	// weights := []float64{10 * 15, 20 * 15, 30 * 15, 10 * 85, 20 * 85, 30 * 85, 40 * 85}
	interfaceName := args.interfaces.in[0]

	args.clientA.StartWithNoCache(t)
	args.clientA.BecomeLeader(t)
	configureBaseDoubleRecusionVip1Entry(ctx, t, args)
	configureBaseDoubleRecusionVip2Entry(ctx, t, args)
	configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)

	t.Run("Delete apply-vrf-selection-policy leaf", func(t *testing.T) {
		// gnmi.Delete(t, args.dut, gnmi.OC().NetworkInstance(*ciscoFlags.PbrInstance).PolicyForwarding().Interface(interfaceName).ApplyVrfSelectionPolicy().Config())
		gnmi.Delete(t, args.dut, gnmi.OC().NetworkInstance(*ciscoFlags.PbrInstance).PolicyForwarding().Interface(interfaceName+"."+"0").Config())
		defer configPBRunderInterface(t, interfaceName, pbrName)

		t.Run("Verify bundle still up", func(t *testing.T) {
			if got := gnmi.Get(t, args.dut, gnmi.OC().Interface(interfaceName).OperStatus().State()); got != oc.Interface_OperStatus_UP {
				t.Errorf("oper-status: got %v", got)
			}
		})

		t.Run("Expect traffic fail", func(t *testing.T) {
			// Create Traffic and check traffic
			srcEndPoint := args.top.Interfaces()[atePort1.Name]
			// Expecting Traffic fail
			testTraffic(t, false, args.ate, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, 0)
		})
	})

	t.Run("Delete interface list entry", func(t *testing.T) {
		// gnmi.Delete(t, args.dut, gnmi.OC().NetworkInstance(*ciscoFlags.PbrInstance).PolicyForwarding().Interface(interfaceName).Config())
		gnmi.Delete(t, args.dut, gnmi.OC().NetworkInstance(*ciscoFlags.PbrInstance).PolicyForwarding().Interface(interfaceName+"."+"0").Config())
		defer configPBRunderInterface(t, interfaceName, pbrName)

		// // TODO: enabled once gNMI Get works on XR
		// t.Run("Verify deleted", func(t *testing.T) {
		// 	verifyConfigPBRUnderInterface(ctx, t, args, interfaceName, false)
		// })

		t.Run("Verify bundle still up", func(t *testing.T) {
			if got := gnmi.Get(t, args.dut, gnmi.OC().Interface(interfaceName).OperStatus().State()); got != oc.Interface_OperStatus_UP {
				t.Errorf("oper-status: got %v", got)
			}
		})

		t.Run("Expect traffic fail", func(t *testing.T) {
			// Create Traffic and check traffic
			srcEndPoint := args.top.Interfaces()[atePort1.Name]
			// Expecting Traffic fail
			testTraffic(t, false, args.ate, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, 0)
		})
	})
}

// testRemoveMatchField tests existing match field in existing class-map which is not related to IPinIP match and verify traffic
func testRemoveMatchField(ctx context.Context, t *testing.T, args *testArgs) {
	defer configBasePBR(t, args.dut)
	defer flushServer(t, args)

	// weights := []float64{10 * 15, 20 * 15, 30 * 15, 10 * 85, 20 * 85, 30 * 85, 40 * 85}

	args.clientA.StartWithNoCache(t)
	args.clientA.BecomeLeader(t)
	configureBaseDoubleRecusionVip1Entry(ctx, t, args)
	configureBaseDoubleRecusionVip2Entry(ctx, t, args)
	configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)

	t.Run("Remove dscp-set", func(t *testing.T) {
		// A class map without any entry is invalid. Adding IPinIP entry in Rule2 such that on deleting
		// dscp set, class map is not left empty.
		r1 := &oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
		r1.SequenceId = ygot.Uint32(2)
		r1.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
			Protocol: oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP,
		}
		r1.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String(*ciscoFlags.NonDefaultNetworkInstance)}
		gnmi.Update(t, args.dut, gnmi.OC().NetworkInstance(*ciscoFlags.PbrInstance).PolicyForwarding().Policy(pbrName).Rule(2).Config(), r1)

		// Remove existing match field
		gnmi.Delete(t, args.dut, gnmi.OC().NetworkInstance(*ciscoFlags.PbrInstance).PolicyForwarding().Policy(pbrName).Rule(2).Ipv4().DscpSet().Config())
		defer configBasePBR(t, args.dut)

		// // TODO: enabled once gNMI Get works on XR
		// t.Run("Verify deleted", func(t *testing.T) {
		// 	verifyConfigPbrMatchIpv4DscpSet(ctx, t, args, 2, []uint8{})
		// })

		// Create Traffic and check traffic
		t.Run("Verify traffic", func(t *testing.T) {
			srcEndPoint := args.top.Interfaces()[atePort1.Name]
			// dstEndPoint := []*ondatra.Interface{args.top.Interfaces()[atePort2.Name], args.top.Interfaces()[atePort3.Name]}

			testTraffic(t, true, args.ate, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, 0)
		})
	})

	t.Run("Remove protocol", func(t *testing.T) {
		defer configBasePBR(t, args.dut)

		var success bool
		success = t.Run("Pre-test config", func(t *testing.T) {
			r1 := &oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
			r1.SequenceId = ygot.Uint32(1)
			r1.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
				Protocol: oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP,
				DscpSet:  []uint8{10},
			}
			r1.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String(*ciscoFlags.NonDefaultNetworkInstance)}
			gnmi.Replace(t, args.dut, gnmi.OC().NetworkInstance(*ciscoFlags.PbrInstance).PolicyForwarding().Policy(pbrName).Rule(1).Config(), r1)
		})
		if !success {
			t.Fatal("failed to apply pre-test configuration")
		}

		// FIXME: Workaround as XR isn't deleting DSCP 10 on replace
		defer deletePBRPolicyAndClassMaps(context.Background(), t, args.dut, pbrName, 4)
		defer deletePolicyFromInterface(ctx, t, args.dut, pbrName)

		// Remove existing match field
		success = t.Run("Delete", func(t *testing.T) {
			gnmi.Delete(t, args.dut, gnmi.OC().NetworkInstance(*ciscoFlags.PbrInstance).PolicyForwarding().Policy(pbrName).Rule(1).Ipv4().Protocol().Config())
		})
		if !success {
			t.FailNow()
		}

		// // TODO: enabled once gNMI Get works on XR
		// t.Run("Verify deleted", func(t *testing.T) {
		// 	verifyConfigPbrMatchIpv4DscpSet(ctx, t, args, 2, []uint8{})
		// })

		t.Run("Expect traffic Pass", func(t *testing.T) {
			srcEndPoint := args.top.Interfaces()[atePort1.Name]

			// Expecting Traffic to pass as Rule1 still has match on dscp set {10}
			testTraffic(t, true, args.ate, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, 10)
		})
	})
}

// testModifyMatchField tests modifying existing match filed in the existing class-map and verify traffic
func testModifyMatchField(ctx context.Context, t *testing.T, args *testArgs) {
	defer configBasePBR(t, args.dut)
	defer flushServer(t, args)

	// weights := []float64{10 * 15, 20 * 15, 30 * 15, 10 * 85, 20 * 85, 30 * 85, 40 * 85}

	args.clientA.StartWithNoCache(t)
	args.clientA.BecomeLeader(t)
	configureBaseDoubleRecusionVip1Entry(ctx, t, args)
	configureBaseDoubleRecusionVip2Entry(ctx, t, args)
	configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)

	// Modify match field for protocol IPinIP class
	gnmi.Delete(t, args.dut, gnmi.OC().NetworkInstance(*ciscoFlags.PbrInstance).PolicyForwarding().Policy(pbrName).Rule(1).Config())
	gnmi.Replace[oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4_Protocol_Union](t, args.dut, gnmi.OC().NetworkInstance(*ciscoFlags.PbrInstance).PolicyForwarding().Policy(pbrName).Rule(2).Ipv4().Protocol().Config(), oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP)

	// Create Traffic and check traffic
	srcEndPoint := args.top.Interfaces()[atePort1.Name]

	// Expecting Traffic fail
	testTraffic(t, false, args.ate, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, 0)
}

// testAddMatchField tests adding new match field in the existing class-map and verify traffic
func testAddMatchField(ctx context.Context, t *testing.T, args *testArgs) {
	defer configBasePBR(t, args.dut)
	defer flushServer(t, args)

	// weights := []float64{10 * 15, 20 * 15, 30 * 15, 10 * 85, 20 * 85, 30 * 85, 40 * 85}

	configureBaseDoubleRecusionVip1Entry(ctx, t, args)
	configureBaseDoubleRecusionVip2Entry(ctx, t, args)
	configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)

	t.Run("Add dscp-set", func(t *testing.T) {
		gnmi.Replace(t, args.dut, gnmi.OC().NetworkInstance(*ciscoFlags.PbrInstance).PolicyForwarding().Policy(pbrName).Rule(1).Ipv4().DscpSet().Config(), []uint8{10, 12})
		defer configBasePBR(t, args.dut)

		// Create Traffic and check traffic
		t.Run("Verify traffic on DSCP 12", func(t *testing.T) {
			srcEndPoint := args.top.Interfaces()[atePort1.Name]
			testTraffic(t, true, args.ate, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, 10)
		})

		// TODO: Make sure this is correct when current config failure is fixed.
		t.Run("Expect traffic fail without DSCP", func(t *testing.T) {
			srcEndPoint := args.top.Interfaces()[atePort1.Name]

			// Expecting Traffic fail
			testTraffic(t, false, args.ate, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, 0)
		})
	})

	t.Run("Add protocol", func(t *testing.T) {
		gnmi.Replace[oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4_Protocol_Union](t, args.dut, gnmi.OC().NetworkInstance(*ciscoFlags.PbrInstance).PolicyForwarding().Policy(pbrName).Rule(2).Ipv4().Protocol().Config(), oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP)
		defer configBasePBR(t, args.dut)

		// Create Traffic and check traffic
		t.Run("Verify traffic", func(t *testing.T) {
			srcEndPoint := args.top.Interfaces()[atePort1.Name]
			testTraffic(t, true, args.ate, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, 0)
		})
	})
}

// Function.PBR.:027 interface shut/unshut and verify traffic
func testTrafficFlapInterface(ctx context.Context, t *testing.T, args *testArgs) {
	defer configBasePBR(t, args.dut)
	defer flushServer(t, args)

	// weights := []float64{10 * 15, 20 * 15, 30 * 15, 10 * 85, 20 * 85, 30 * 85, 40 * 85}

	configureBaseDoubleRecusionVip1Entry(ctx, t, args)
	configureBaseDoubleRecusionVip2Entry(ctx, t, args)
	configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)

	// Create Traffic and check traffic
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	testTraffic(t, true, args.ate, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, 0)

	// Flap Interface
	for _, interface_name := range args.interfaces.in {
		util.FlapInterface(t, args.dut, interface_name, 5)
	}
	// Verify Traffic again
	testTraffic(t, true, args.ate, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, 0)
}

// Function.PBR.:020 Verify PBR policy works with match DSCP and action VRF redirect
func testMatchDscpActionVRFRedirect(ctx context.Context, t *testing.T, args *testArgs) {
	defer configBasePBR(t, args.dut)
	defer flushServer(t, args)

	// weights := []float64{10 * 15, 20 * 15, 30 * 15, 10 * 85, 20 * 85, 30 * 85, 40 * 85}

	configureBaseDoubleRecusionVip1Entry(ctx, t, args)
	configureBaseDoubleRecusionVip2Entry(ctx, t, args)
	configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)

	// Create Traffic and check traffic
	dscp := uint8(16)
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	testTraffic(t, true, args.ate, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, dscp)
}

func GetIpv4Acl(name string, sequenceId uint32, dscp uint8, action oc.E_Acl_FORWARDING_ACTION) *oc.Acl {
	acl := (&oc.Root{}).GetOrCreateAcl()
	aclSet := acl.GetOrCreateAclSet(name, oc.Acl_ACL_TYPE_ACL_IPV4)
	aclEntry := aclSet.GetOrCreateAclEntry(sequenceId)
	aclEntryIpv4 := aclEntry.GetOrCreateIpv4()
	aclEntryIpv4.Dscp = ygot.Uint8(dscp)
	aclEntryAction := aclEntry.GetOrCreateActions()
	aclEntryAction.ForwardingAction = action
	return acl
}

// Function.PBR.:024 Feature Interaction: configure ACL and PBR under same interface and verify behavior
func testAclAndPBRUnderSameInterface(ctx context.Context, t *testing.T, args *testArgs) {
	defer configBasePBR(t, args.dut)
	defer flushServer(t, args)

	// weights := []float64{10 * 15, 20 * 15, 30 * 15, 10 * 85, 20 * 85, 30 * 85, 40 * 85}

	configureBaseDoubleRecusionVip1Entry(ctx, t, args)
	configureBaseDoubleRecusionVip2Entry(ctx, t, args)
	configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)

	// Create and apply ACL that allows DSCP16 traffic to ingress interface. Verify traffic passes.
	dscp := uint8(16)
	aclName := "dscp_pass"
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	t.Run("AclWithActionAccept", func(t *testing.T) {
		aclConfig := GetIpv4Acl(aclName, 10, dscp, oc.Acl_FORWARDING_ACTION_ACCEPT)
		gnmi.Replace(t, args.dut, gnmi.OC().Acl().Config(), aclConfig)
		defer gnmi.Delete(t, args.dut, gnmi.OC().Acl().Config())

		// gnmi.Replace(t, args.dut, gnmi.OC().Acl().Interface(args.interfaces.in[0]+".0").IngressAclSet(aclName, oc.Acl_ACL_TYPE_ACL_IPV4).SetName().Config(), aclName)
		gnmi.Replace(t, args.dut, gnmi.OC().Acl().Interface(args.interfaces.in[0]).Config(), &oc.Acl_Interface{
			Id: ygot.String(args.interfaces.in[0]),
		})
		gnmi.Replace(t, args.dut, gnmi.OC().Acl().Interface(args.interfaces.in[0]).IngressAclSet(aclName, oc.Acl_ACL_TYPE_ACL_IPV4).Config(), &oc.Acl_Interface_IngressAclSet{
			Type:    oc.Acl_ACL_TYPE_ACL_IPV4,
			SetName: &aclName,
		})
		defer gnmi.Delete(t, args.dut, gnmi.OC().Acl().Interface(args.interfaces.in[0]).IngressAclSet(aclName, oc.Acl_ACL_TYPE_ACL_IPV4).Config())

		testTraffic(t, true, args.ate, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, dscp)
	})
	//Create and apply ACL that drops DSCP16 traffic incoming on the ingress interface. Verify traffic drops.
	aclName = "dscp_drop"
	t.Run("AclWithActionReject", func(t *testing.T) {
		aclConfig := GetIpv4Acl(aclName, 10, dscp, oc.Acl_FORWARDING_ACTION_REJECT)
		gnmi.Replace(t, args.dut, gnmi.OC().Acl().Config(), aclConfig)
		defer gnmi.Delete(t, args.dut, gnmi.OC().Acl().Config())

		// gnmi.Replace(t, args.dut, gnmi.OC().Acl().Interface(args.interfaces.in[0]).IngressAclSet(aclName, oc.Acl_ACL_TYPE_ACL_IPV4).SetName().Config(), aclName)
		gnmi.Replace(t, args.dut, gnmi.OC().Acl().Interface(args.interfaces.in[0]).Config(), &oc.Acl_Interface{
			Id: ygot.String(args.interfaces.in[0]),
		})
		gnmi.Replace(t, args.dut, gnmi.OC().Acl().Interface(args.interfaces.in[0]).IngressAclSet(aclName, oc.Acl_ACL_TYPE_ACL_IPV4).Config(), &oc.Acl_Interface_IngressAclSet{
			Type:    oc.Acl_ACL_TYPE_ACL_IPV4,
			SetName: &aclName,
		})
		defer gnmi.Delete(t, args.dut, gnmi.OC().Acl().Interface(args.interfaces.in[0]).IngressAclSet(aclName, oc.Acl_ACL_TYPE_ACL_IPV4).Config())

		testTraffic(t, false, args.ate, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, dscp)
	})
}

func testPolicesReplace(ctx context.Context, t *testing.T, args *testArgs) {

	configBasePBR(t, args.dut)

	// weights := []float64{10 * 15, 20 * 15, 30 * 15, 10 * 85, 20 * 85, 30 * 85, 40 * 85}

	configureBaseDoubleRecusionVip1Entry(ctx, t, args)
	configureBaseDoubleRecusionVip2Entry(ctx, t, args)
	configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)

	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	testTraffic(t, true, args.ate, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, 0)

	r2 := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
	r2.SequenceId = ygot.Uint32(2)
	r2.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
		DscpSet: []uint8{*ygot.Uint8(16)},
	}
	r2.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String(*ciscoFlags.NonDefaultNetworkInstance)}

	r3 := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
	r3.SequenceId = ygot.Uint32(3)
	r3.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
		DscpSet: []uint8{*ygot.Uint8(18)},
	}
	r3.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String("VRF1")}

	r4 := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
	r4.SequenceId = ygot.Uint32(4)
	r4.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
		DscpSet: []uint8{*ygot.Uint8(48)},
	}
	r4.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String(*ciscoFlags.NonDefaultNetworkInstance)}

	p := oc.NetworkInstance_PolicyForwarding_Policy{}
	p.PolicyId = ygot.String(pbrName)
	p.Type = oc.Policy_Type_VRF_SELECTION_POLICY
	p.Rule = map[uint32]*oc.NetworkInstance_PolicyForwarding_Policy_Rule{2: &r2, 3: &r3, 4: &r4}

	policy := oc.NetworkInstance_PolicyForwarding{}
	policy.Policy = map[string]*oc.NetworkInstance_PolicyForwarding_Policy{pbrName: &p}

	gnmi.Replace(t, args.dut, gnmi.OC().NetworkInstance(*ciscoFlags.PbrInstance).PolicyForwarding().Config(), &policy)

	testTraffic(t, false, args.ate, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, 0)
}

func testPolicyReplace(ctx context.Context, t *testing.T, args *testArgs) {

	configBasePBR(t, args.dut)
	defer configBasePBR(t, args.dut)

	// weights := []float64{10 * 15, 20 * 15, 30 * 15, 10 * 85, 20 * 85, 30 * 85, 40 * 85}

	configureBaseDoubleRecusionVip1Entry(ctx, t, args)
	configureBaseDoubleRecusionVip2Entry(ctx, t, args)
	configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)

	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	testTraffic(t, true, args.ate, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, 0)

	r2 := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
	r2.SequenceId = ygot.Uint32(2)
	r2.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
		DscpSet: []uint8{*ygot.Uint8(16)},
	}
	r2.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String(*ciscoFlags.NonDefaultNetworkInstance)}

	r3 := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
	r3.SequenceId = ygot.Uint32(3)
	r3.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
		DscpSet: []uint8{*ygot.Uint8(18)},
	}
	r3.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String("VRF1")}

	r4 := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
	r4.SequenceId = ygot.Uint32(4)
	r4.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
		DscpSet: []uint8{*ygot.Uint8(48)},
	}
	r4.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String(*ciscoFlags.NonDefaultNetworkInstance)}

	p := oc.NetworkInstance_PolicyForwarding_Policy{}
	p.PolicyId = ygot.String(pbrName)
	p.Type = oc.Policy_Type_VRF_SELECTION_POLICY
	p.Rule = map[uint32]*oc.NetworkInstance_PolicyForwarding_Policy_Rule{2: &r2, 3: &r3, 4: &r4}

	gnmi.Replace(t, args.dut, gnmi.OC().NetworkInstance(*ciscoFlags.PbrInstance).PolicyForwarding().Policy(pbrName).Config(), &p)

	testTraffic(t, false, args.ate, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, 0)
}
func testSrcIp(ctx context.Context, t *testing.T, args *testArgs) {

	// Program GRIBI entry on the router
	// weights := []float64{10 * 15, 20 * 15, 30 * 15, 10 * 85, 20 * 85, 30 * 85, 40 * 85}

	configureBaseDoubleRecusionVip1Entry(ctx, t, args)
	configureBaseDoubleRecusionVip2Entry(ctx, t, args)
	configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)

	dscpVal := uint8(Dscpval)

	// Configure policy-map that matches the SourceAddress
	configSrcIp(t, args.dut, PbrNameSrc, SourceAddress)
	defer deletePBRPolicy(t, args.dut, PbrNameSrc)

	// Configure policy under bundle-interface
	unconfigPBRunderInterface(t, args, args.interfaces.in[0])
	configPBRunderInterface(t, args.interfaces.in[0], PbrNameSrc)
	defer unconfigPBRunderInterface(t, args, args.interfaces.in[0])

	//Create Traffic and check traffic
	srcEndPoint := args.top.Interfaces()[atePort1.Name]

	testTrafficSrc(t, true, args.ate, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, dscpVal, IxiaSrcip)
}

func testSrcIpNegative(ctx context.Context, t *testing.T, args *testArgs) {

	// Program GRIBI entry on the router

	// weights := []float64{10 * 15, 20 * 15, 30 * 15, 10 * 85, 20 * 85, 30 * 85, 40 * 85}
	configureBaseDoubleRecusionVip1Entry(ctx, t, args)
	configureBaseDoubleRecusionVip2Entry(ctx, t, args)
	configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)

	dscpVal := uint8(Dscpval)

	// Configure policy that matches the SourceAddress
	configSrcIp(t, args.dut, PbrNameSrc, SourceAddress2)
	defer deletePBRPolicy(t, args.dut, PbrNameSrc)

	// configure policy under interface
	unconfigPBRunderInterface(t, args, args.interfaces.in[0])
	configPBRunderInterface(t, args.interfaces.in[0], PbrNameSrc)
	defer unconfigPBRunderInterface(t, args, args.interfaces.in[0])

	// Create Traffic and check traffic
	srcEndPoint := args.top.Interfaces()[atePort1.Name]

	//traffic fail expected
	testTrafficSrc(t, false, args.ate, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, dscpVal, IxiaSrcip)
}

func testPBRSrcIpWithDscp(ctx context.Context, t *testing.T, args *testArgs) {

	// Program GRIBI entry on the router
	// weights := []float64{10 * 15, 20 * 15, 30 * 15, 10 * 85, 20 * 85, 30 * 85, 40 * 85}

	configureBaseDoubleRecusionVip1Entry(ctx, t, args)
	configureBaseDoubleRecusionVip2Entry(ctx, t, args)
	configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)

	dscpVal := uint8(Dscpval)

	// Configure policy-map that matches the SrcIp, Dscp value
	configSrcIpDscp(t, args.dut, PbrNameDscp, dscpVal, SourceAddress)
	defer deletePBRPolicy(t, args.dut, PbrNameDscp)

	// Configure policy on the bunlde interface
	unconfigPBRunderInterface(t, args, args.interfaces.in[0])
	configPBRunderInterface(t, args.interfaces.in[0], PbrNameDscp)
	defer unconfigPBRunderInterface(t, args, args.interfaces.in[0])

	// Create Traffic and check traffic
	srcEndPoint := args.top.Interfaces()[atePort1.Name]

	testTrafficSrc(t, true, args.ate, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, dscpVal, IxiaSrcip)
}

func testDettachAndAttachWrongSrcIp(ctx context.Context, t *testing.T, args *testArgs) {

	// Program GRIBI entry on the router
	// weights := []float64{10 * 15, 20 * 15, 30 * 15, 10 * 85, 20 * 85, 30 * 85, 40 * 85}

	configureBaseDoubleRecusionVip1Entry(ctx, t, args)
	configureBaseDoubleRecusionVip2Entry(ctx, t, args)
	configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)

	dscpVal := uint8(Dscpval)

	// un-Configure policy under bundle-interface
	unconfigPBRunderInterface(t, args, args.interfaces.in[0])

	// Configure policy-map that matches the SourceAddress
	configSrcIp(t, args.dut, PbrNameSrc, SourceAddress)
	defer deletePBRPolicy(t, args.dut, PbrNameSrc)
	configSrcIp(t, args.dut, PbrNameSrc2, SourceAddress2)
	defer deletePBRPolicy(t, args.dut, PbrNameSrc2)

	// Configure policy under bundle-interface
	unconfigPBRunderInterface(t, args, args.interfaces.in[0])
	configPBRunderInterface(t, args.interfaces.in[0], PbrNameSrc)

	srcEndPoint := args.top.Interfaces()[atePort1.Name]

	// Traffic verification
	testTrafficSrc(t, true, args.ate, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, dscpVal, IxiaSrcip)

	unconfigPBRunderInterface(t, args, args.interfaces.in[0])
	configPBRunderInterface(t, args.interfaces.in[0], PbrNameSrc2)
	defer unconfigPBRunderInterface(t, args, args.interfaces.in[0])

	// Traffic failure expected
	testTrafficSrc(t, false, args.ate, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, dscpVal, IxiaSrcip)
}

func testDettachAndAttachDifferentSrcIp(ctx context.Context, t *testing.T, args *testArgs) {

	// Program GRIBI entry on the router
	// weights := []float64{10 * 15, 20 * 15, 30 * 15, 10 * 85, 20 * 85, 30 * 85, 40 * 85}

	configureBaseDoubleRecusionVip1Entry(ctx, t, args)
	configureBaseDoubleRecusionVip2Entry(ctx, t, args)
	configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)

	dscpVal := uint8(Dscpval)

	// Configure policy-map that matches the SourceAddress
	configSrcIp(t, args.dut, PbrNameSrc, SourceAddress)
	defer deletePBRPolicy(t, args.dut, PbrNameSrc)

	configSrcIp(t, args.dut, PbrNameSrc2, SourceAddress2)
	defer deletePBRPolicy(t, args.dut, PbrNameSrc2)

	// Configure policy under bundle-interface
	unconfigPBRunderInterface(t, args, args.interfaces.in[0])
	configPBRunderInterface(t, args.interfaces.in[0], PbrNameSrc)
	defer unconfigPBRunderInterface(t, args, args.interfaces.in[0])

	srcEndPoint := args.top.Interfaces()[atePort1.Name]

	// Traffic verification
	testTrafficSrc(t, true, args.ate, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, dscpVal, IxiaSrcip)

	unconfigPBRunderInterface(t, args, args.interfaces.in[0])
	configPBRunderInterface(t, args.interfaces.in[0], PbrNameSrc2)

	// Traffic pass expected
	testTrafficSrc(t, true, args.ate, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, dscpVal, IxiaSrcip2)
}
func testUpdateSrcIp(ctx context.Context, t *testing.T, args *testArgs) {

	// Program GRIBI entry on the router
	// weights := []float64{10 * 15, 20 * 15, 30 * 15, 10 * 85, 20 * 85, 30 * 85, 40 * 85}

	configureBaseDoubleRecusionVip1Entry(ctx, t, args)
	configureBaseDoubleRecusionVip2Entry(ctx, t, args)
	configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)

	dscpVal := uint8(Dscpval)

	// Configure policy-map that matches the SourceAddress
	configSrcIp(t, args.dut, PbrNameSrc, SourceAddress)
	defer deletePBRPolicy(t, args.dut, PbrNameSrc)

	// Configure policy under bundle-interface
	unconfigPBRunderInterface(t, args, args.interfaces.in[0])
	configPBRunderInterface(t, args.interfaces.in[0], PbrNameSrc)

	srcEndPoint := args.top.Interfaces()[atePort1.Name]

	// Traffic check
	testTrafficSrc(t, true, args.ate, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, dscpVal, IxiaSrcip)

	configSrcIp(t, args.dut, PbrNameSrc, SourceAddress2)
	defer deletePBRPolicy(t, args.dut, PbrNameSrc2)

	unconfigPBRunderInterface(t, args, args.interfaces.in[0])
	configPBRunderInterface(t, args.interfaces.in[0], PbrNameSrc)
	defer unconfigPBRunderInterface(t, args, args.interfaces.in[0])

	// Traffic verification - traffic pass expected
	testTrafficSrc(t, true, args.ate, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, dscpVal, IxiaSrcip2)
}
func testUpdateWrongSrcIp(ctx context.Context, t *testing.T, args *testArgs) {

	// Program GRIBI entry on the router
	// weights := []float64{10 * 15, 20 * 15, 30 * 15, 10 * 85, 20 * 85, 30 * 85, 40 * 85}

	configureBaseDoubleRecusionVip1Entry(ctx, t, args)
	configureBaseDoubleRecusionVip2Entry(ctx, t, args)
	configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)

	dscpVal := uint8(Dscpval)

	// Configure policy-map that matches the SourceAddress
	configSrcIp(t, args.dut, PbrNameSrc, SourceAddress)
	defer deletePBRPolicy(t, args.dut, PbrNameSrc)

	// Configure policy under bundle-interface
	unconfigPBRunderInterface(t, args, args.interfaces.in[0])
	configPBRunderInterface(t, args.interfaces.in[0], PbrNameSrc)

	srcEndPoint := args.top.Interfaces()[atePort1.Name]

	//Create Traffic and check traffic
	testTrafficSrc(t, true, args.ate, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, dscpVal, IxiaSrcip)

	configSrcIp(t, args.dut, PbrNameSrc, SourceAddress2)
	defer deletePBRPolicy(t, args.dut, PbrNameSrc)

	unconfigPBRunderInterface(t, args, args.interfaces.in[0])
	configPBRunderInterface(t, args.interfaces.in[0], PbrNameSrc)
	defer unconfigPBRunderInterface(t, args, args.interfaces.in[0])

	//Traffic failure expected
	testTrafficSrc(t, false, args.ate, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, dscpVal, IxiaSrcip)
}
func testReplaceAtSrcIpLeaf(ctx context.Context, t *testing.T, args *testArgs) {

	// Program GRIBI entry on the router
	configureBaseDoubleRecusionVip1Entry(ctx, t, args)
	configureBaseDoubleRecusionVip2Entry(ctx, t, args)
	configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)

	dscpVal := uint8(Dscpval)

	// Configure policy-map that matches the SrcIp, Dscp value
	configSrcIpDscp(t, args.dut, PbrNameDscp, dscpVal, SourceAddress)
	defer deletePBRPolicy(t, args.dut, PbrNameDscp)

	//Configure policy under bundle-interface
	unconfigPBRunderInterface(t, args, args.interfaces.in[0])
	configPBRunderInterface(t, args.interfaces.in[0], PbrNameDscp)
	defer unconfigPBRunderInterface(t, args, args.interfaces.in[0])

	srcEndPoint := args.top.Interfaces()[atePort1.Name]

	//Create Traffic and check traffic
	testTrafficSrc(t, true, args.ate, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, dscpVal, IxiaSrcip)

	//Configure policy under bundle-interface
	replaceOnlySrcIp(t, args.dut, PbrNameDscp, SourceAddress2)

	// Create Traffic and check traffic
	testTrafficSrc(t, true, args.ate, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, dscpVal, IxiaSrcip2)

}

func testUpdateAtSrcIpLeaf(ctx context.Context, t *testing.T, args *testArgs) {

	// Program GRIBI entry on the router
	configureBaseDoubleRecusionVip1Entry(ctx, t, args)
	configureBaseDoubleRecusionVip2Entry(ctx, t, args)
	configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)

	dscpVal := uint8(Dscpval)

	// Configure policy-map that matches the SourceAddress
	configSrcIp(t, args.dut, PbrNameSrc, SourceAddress)
	defer deletePBRPolicy(t, args.dut, PbrNameSrc)

	//Configure policy under bundle-interface
	unconfigPBRunderInterface(t, args, args.interfaces.in[0])
	configPBRunderInterface(t, args.interfaces.in[0], PbrNameSrc)
	defer unconfigPBRunderInterface(t, args, args.interfaces.in[0])

	srcEndPoint := args.top.Interfaces()[atePort1.Name]

	//Create Traffic and check traffic
	testTrafficSrc(t, true, args.ate, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, dscpVal, IxiaSrcip)

	//Configure policy under bundle-interface
	updateOnlySrcIp(t, args.dut, PbrNameSrc, SourceAddress2)

	//Create Traffic and check traffic
	testTrafficSrc(t, true, args.ate, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, dscpVal, IxiaSrcip2)

}
func testUpdateAtSrcIpLeafNegative(ctx context.Context, t *testing.T, args *testArgs) {

	// Program GRIBI entry on the router
	configureBaseDoubleRecusionVip1Entry(ctx, t, args)
	configureBaseDoubleRecusionVip2Entry(ctx, t, args)
	configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)

	dscpVal := uint8(Dscpval)

	// Configure policy-map that matches the SourceAddress
	configSrcIp(t, args.dut, PbrNameSrc, SourceAddress)
	defer deletePBRPolicy(t, args.dut, PbrNameSrc)

	//Configure policy under bundle-interface
	unconfigPBRunderInterface(t, args, args.interfaces.in[0])
	configPBRunderInterface(t, args.interfaces.in[0], PbrNameSrc)
	defer unconfigPBRunderInterface(t, args, args.interfaces.in[0])

	srcEndPoint := args.top.Interfaces()[atePort1.Name]

	//Create Traffic and check traffic
	testTrafficSrc(t, true, args.ate, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, dscpVal, IxiaSrcip)

	//Configure policy under bundle-interface
	updateOnlySrcIp(t, args.dut, PbrNameSrc, SourceAddress2)

	//Create Traffic and traffic expected to fail
	testTrafficSrc(t, false, args.ate, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, dscpVal, IxiaSrcip)

	updateOnlySrcIp(t, args.dut, PbrNameSrc, SourceAddress)

	//Create Traffic and traffic expected to pass
	testTrafficSrc(t, true, args.ate, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, dscpVal, IxiaSrcip)

}
func testReplaceSrcIpRule(ctx context.Context, t *testing.T, args *testArgs) {

	// Program GRIBI entry on the router
	configureBaseDoubleRecusionVip1Entry(ctx, t, args)
	configureBaseDoubleRecusionVip2Entry(ctx, t, args)
	configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)

	dscpVal := uint8(Dscpval)
	srcEndPoint := args.top.Interfaces()[atePort1.Name]

	// Configure policy-map that matches the SrcIp, Dscp value
	configSrcIpDscp(t, args.dut, PbrNameDscp, dscpVal, SourceAddress)
	defer deletePBRPolicy(t, args.dut, PbrNameDscp)

	//Configure policy under bundle-interface
	unconfigPBRunderInterface(t, args, args.interfaces.in[0])
	configPBRunderInterface(t, args.interfaces.in[0], PbrNameDscp)
	defer unconfigPBRunderInterface(t, args, args.interfaces.in[0])

	//Create Traffic and check traffic
	testTrafficSrc(t, true, args.ate, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, dscpVal, IxiaSrcip)

	//Replace the rule with different src adddress
	replaceSrcIpRule(t, args.dut, PbrNameDscp, SourceAddress2, SeqID)

	// Create Traffic and check traffic expected to fail
	testTrafficSrc(t, false, args.ate, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, dscpVal, IxiaSrcip)

	replaceSrcIpRule(t, args.dut, PbrNameDscp, SourceAddress, SeqID)

	//Create Traffic and check traffic expected to pass
	testTrafficSrc(t, true, args.ate, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, dscpVal, IxiaSrcip)

}
func testReplaceSrcIpEntirePolicy(ctx context.Context, t *testing.T, args *testArgs) {

	// Program GRIBI entry on the router
	configureBaseDoubleRecusionVip1Entry(ctx, t, args)
	configureBaseDoubleRecusionVip2Entry(ctx, t, args)
	configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)

	dscpVal := uint8(Dscpval)
	srcEndPoint := args.top.Interfaces()[atePort1.Name]

	// Configure policy-map that matches the SrcIp, Dscp value
	configSrcIp(t, args.dut, PbrNameSrc, SourceAddress)
	configNewRule(t, args.dut, PbrNameSrc, SeqID2, protocolNum, dscpVal)
	defer deletePBRPolicy(t, args.dut, PbrNameSrc)

	//Configure policy under bundle-interface
	unconfigPBRunderInterface(t, args, args.interfaces.in[0])
	configPBRunderInterface(t, args.interfaces.in[0], PbrNameSrc)

	// Create Traffic and check traffic
	testTrafficSrc(t, true, args.ate, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, dscpVal, IxiaSrcip)

	//Replace the pmap with different rule and match
	replaceSrcpmap(t, args.dut, PbrNameSrc2, SourceAddress2, SeqID2)
	defer deletePBRPolicy(t, args.dut, PbrNameSrc2)

	configPBRunderInterface(t, args.interfaces.in[0], PbrNameSrc2)
	defer unconfigPBRunderInterface(t, args, args.interfaces.in[0])

	// Create Traffic and check traffic expected to fail
	testTrafficSrc(t, true, args.ate, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, dscpVal, IxiaSrcip2)

}
func testSrcIpMoreRules(ctx context.Context, t *testing.T, args *testArgs) {

	// Program GRIBI entry on the router
	configureBaseDoubleRecusionVip1Entry(ctx, t, args)
	configureBaseDoubleRecusionVip2Entry(ctx, t, args)
	configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)

	dscpVal := uint8(Dscpval)
	srcEndPoint := args.top.Interfaces()[atePort1.Name]

	configSrcIp(t, args.dut, PbrNameSrc, SourceAddress)
	configUpdateRule(t, args.dut, PbrNameSrc, dscpVal)
	defer deletePBRPolicy(t, args.dut, PbrNameSrc)

	unconfigPBRunderInterface(t, args, args.interfaces.in[0])
	configPBRunderInterface(t, args.interfaces.in[0], PbrNameSrc)
	defer unconfigPBRunderInterface(t, args, args.interfaces.in[0])

	// Create Traffic and check traffic
	testTrafficSrc(t, true, args.ate, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, dscpVal, IxiaSrcip)

	configSrcIp(t, args.dut, PbrNameSrc, SourceAddress2)

	testTrafficSrc(t, true, args.ate, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, dscpVal, IxiaSrcip2)

}
func testSrcIpWithDscp(ctx context.Context, t *testing.T, args *testArgs) {

	// Program GRIBI entry on the router
	srcEndPoint := args.top.Interfaces()[atePort1.Name]

	configureBaseDoubleRecusionVip1Entry(ctx, t, args)
	configureBaseDoubleRecusionVip2Entry(ctx, t, args)
	configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)

	dscpVal := uint8(Dscpval)

	// Configure policy that matches the SourceAddress
	configSrcIpDscp(t, args.dut, PbrNameDscp, dscpVal, SourceAddress)
	defer deletePBRPolicy(t, args.dut, PbrNameDscp)

	unconfigPBRunderInterface(t, args, args.interfaces.in[0])
	configPBRunderInterface(t, args.interfaces.in[0], PbrNameDscp)
	defer unconfigPBRunderInterface(t, args, args.interfaces.in[0])

	// Create Traffic and check traffic
	testTrafficSrc(t, true, args.ate, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, dscpVal, IxiaSrcip)

	//Update Wrong Src ip value
	configSrcIpDscp(t, args.dut, PbrNameDscp, dscpVal, SourceAddress2)

	//traffic fails as expected
	testTrafficSrc(t, false, args.ate, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, dscpVal, IxiaSrcip)

	//Update Correct Src ip value
	configSrcIpDscp(t, args.dut, PbrNameDscp, dscpVal, SourceAddress)

	//traffic should pass
	testTrafficSrc(t, true, args.ate, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, dscpVal, IxiaSrcip)
}
func testProtocolV6Negative(ctx context.Context, t *testing.T, args *testArgs) {

	// Program GRIBI entry on the router
	configureBaseDoubleRecusionVip1Entry(ctx, t, args)
	configureBaseDoubleRecusionVip2Entry(ctx, t, args)
	configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)

	dscpVal := uint8(Dscpval)

	// Configure policy-map that matches the SourceAddress
	configProtocolV6(t, args.dut, PbrNameSrc, SourceAddress, dscpVal, protocolNumv6)
	defer deletePBRPolicy(t, args.dut, PbrNameSrc)

	// Configure policy under bundle-interface
	unconfigPBRunderInterface(t, args, args.interfaces.in[0])
	configPBRunderInterface(t, args.interfaces.in[0], PbrNameSrc)

	//Create Traffic and check traffic
	srcEndPoint := args.top.Interfaces()[atePort1.Name]

	testTrafficSrc(t, false, args.ate, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, dscpVal, IxiaSrcip)

	configSrcIpDscp(t, args.dut, PbrNameSrc2, dscpVal, SourceAddress2)
	defer deletePBRPolicy(t, args.dut, PbrNameSrc2)

	// Configure policy under bundle-interface
	unconfigPBRunderInterface(t, args, args.interfaces.in[0])
	configPBRunderInterface(t, args.interfaces.in[0], PbrNameSrc2)
	defer unconfigPBRunderInterface(t, args, args.interfaces.in[0])

	testTrafficSrcV6(t, false, args.ate, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, dscpVal, IxiaSrcip2)

}
func testProtocolV6(ctx context.Context, t *testing.T, args *testArgs) {

	// Program GRIBI entry on the router
	configureBaseDoubleRecusionVip1Entry(ctx, t, args)
	configureBaseDoubleRecusionVip2Entry(ctx, t, args)
	configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)

	dscpVal := uint8(Dscpval)

	// Configure policy-map that matches the SourceAddress
	configProtocolV6(t, args.dut, PbrNameSrc, SourceAddress, dscpVal, protocolNumv6)
	defer deletePBRPolicy(t, args.dut, PbrNameSrc)

	// Configure policy under bundle-interface
	unconfigPBRunderInterface(t, args, args.interfaces.in[0])
	configPBRunderInterface(t, args.interfaces.in[0], PbrNameSrc)
	defer unconfigPBRunderInterface(t, args, args.interfaces.in[0])

	//Create Traffic and check traffic
	srcEndPoint := args.top.Interfaces()[atePort1.Name]

	testTrafficSrcV6(t, true, args.ate, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, dscpVal, IxiaSrcip)

}

func testProtocolV6updateV4(ctx context.Context, t *testing.T, args *testArgs) {

	// Program GRIBI entry on the router
	configureBaseDoubleRecusionVip1Entry(ctx, t, args)
	configureBaseDoubleRecusionVip2Entry(ctx, t, args)
	configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)

	dscpVal := uint8(Dscpval)

	// Configure policy-map that matches the SourceAddress
	configProtocolV6(t, args.dut, PbrNameSrc, SourceAddress, dscpVal, protocolNumv6)
	defer deletePBRPolicy(t, args.dut, PbrNameSrc)

	// Configure policy under bundle-interface
	unconfigPBRunderInterface(t, args, args.interfaces.in[0])
	configPBRunderInterface(t, args.interfaces.in[0], PbrNameSrc)
	defer unconfigPBRunderInterface(t, args, args.interfaces.in[0])

	//Create Traffic and check traffic
	srcEndPoint := args.top.Interfaces()[atePort1.Name]

	//Create Traffic and check traffic
	testTrafficSrcV6(t, true, args.ate, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, dscpVal, IxiaSrcip)

	//Update protocol to 4
	configSrcIpDscp(t, args.dut, PbrNameSrc, dscpVal, SourceAddress)

	testTrafficSrc(t, true, args.ate, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, dscpVal, IxiaSrcip)

}
func testProtocolV6replaceV4(ctx context.Context, t *testing.T, args *testArgs) {

	// Program GRIBI entry on the router

	configureBaseDoubleRecusionVip1Entry(ctx, t, args)
	configureBaseDoubleRecusionVip2Entry(ctx, t, args)
	configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)

	dscpVal := uint8(Dscpval)

	// Configure policy-map that matches the SourceAddress
	configProtocolV6(t, args.dut, PbrNameSrc, SourceAddress, dscpVal, protocolNumv6)
	defer deletePBRPolicy(t, args.dut, PbrNameSrc)

	// Configure policy under bundle-interface
	unconfigPBRunderInterface(t, args, args.interfaces.in[0])
	configPBRunderInterface(t, args.interfaces.in[0], PbrNameSrc)
	defer unconfigPBRunderInterface(t, args, args.interfaces.in[0])

	srcEndPoint := args.top.Interfaces()[atePort1.Name]

	//Create Traffic and check traffic
	testTrafficSrcV6(t, true, args.ate, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, dscpVal, IxiaSrcip)

	//Replace  protocol to 4
	replaceOnlyProtocol(t, args.dut, PbrNameSrc)
	configPBRunderInterface(t, args.interfaces.in[0], PbrNameSrc)

	testTrafficSrc(t, true, args.ate, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, dscpVal, IxiaSrcip)
}

func startGribiClient(t *testing.T) {
	clientA := gribi.Client{
		DUT:         ondatra.DUT(t, "dut"),
		FIBACK:      false,
		Persistence: true,
	}

	clientA.Close(t)
	time.Sleep(2 * time.Minute)
	for i := 0; i < 10; i++ {
		if err := clientA.Start(t); err != nil {
			if i == 9 {
				t.Fatalf("gRIBI Connection can not be established")
			}
			time.Sleep(30 * time.Second)
		}
	}
	clientA.StartWithNoCache(t)
	clientA.BecomeLeader(t)
}
