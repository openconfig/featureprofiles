package cisco_gribi

import (
	"context"
	"testing"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/ygot/ygot"
)

const (
	pbrName = "Transit"
)

func configbasePBR(t *testing.T, args *testArgs) {
	r1 := telemetry.NetworkInstance_PolicyForwarding_Policy_Rule{}
	r1.Ipv4 = &telemetry.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
		DscpSet: []uint8{*ygot.Uint8(16)},
	}
	r1.Action = &telemetry.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String("TE")}

	r2 := telemetry.NetworkInstance_PolicyForwarding_Policy_Rule{}
	r2.Ipv4 = &telemetry.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
		Protocol: telemetry.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP,
	}
	r2.Action = &telemetry.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String("TE")}

	p := telemetry.NetworkInstance_PolicyForwarding_Policy{}
	p.PolicyId = ygot.String(pbrName)
	p.Type = telemetry.Policy_Type_VRF_SELECTION_POLICY
	p.Rule = map[uint32]*telemetry.NetworkInstance_PolicyForwarding_Policy_Rule{1: &r1, 2: &r2}

	args.dut.Config().NetworkInstance("default").PolicyForwarding().Policy(pbrName).Replace(t, &p)
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

func generateBundleMemberInterfaceConfig(t *testing.T, name, bundleId string) *telemetry.Interface {
	i := &telemetry.Interface{}
	i.Name = ygot.String(name)
	i.Type = telemetry.IETFInterfaces_InterfaceType_ethernetCsmacd
	e := i.GetOrCreateEthernet()
	e.AutoNegotiate = ygot.Bool(false)
	e.AggregateId = ygot.String(bundleId)
	return i
}

func configPBRunderInterface(t *testing.T, args *testArgs, interfaceName, policyName string) {
	args.dut.Config().NetworkInstance(instance).PolicyForwarding().Interface(interfaceName).ApplyVrfSelectionPolicy().Update(t, policyName)
}

// Remove the policy under physical interface and add the related physical interface under bundle interface which use the same PBR policy
func testMovePhysicalToBundle(ctx context.Context, t *testing.T, args *testArgs) {
	physicalInterface := fptest.SortPorts(args.dut.Ports())[0].Name()
	physicalInterfaceConfig := args.dut.Config().Interface(physicalInterface)

	// Configure the physcial interface
	config := generatePhysicalInterfaceConfig(t, physicalInterface, "192.192.192.1", 24)
	physicalInterfaceConfig.Replace(t, config)

	// Configure policy on the physical interface and bunlde interface
	configPBRunderInterface(t, args, physicalInterface, pbrName)
	configPBRunderInterface(t, args, args.interfaces.in[0], pbrName)

	// Remove the interface from physical to bundle interface
	memberConfig := generateBundleMemberInterfaceConfig(t, physicalInterface, args.interfaces.in[0])
	physicalInterfaceConfig.Replace(t, memberConfig)
	// physicalInterfaceConfig.Aggregation().Delete(t)

	// Program GRIBI entry on the router
	defer flushSever(t, args)

	weights := []float64{10 * 15, 20 * 15, 30 * 15, 10 * 85, 20 * 85, 30 * 85, 40 * 85}

	configureBaseDoubleRecusionVip1Entry(ctx, t, args)
	configureBaseDoubleRecusionVip2Entry(ctx, t, args)
	configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)

	// Create Traffic and check traffic
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	// dstEndPoint := []*ondatra.Interface{args.top.Interfaces()[atePort2.Name], args.top.Interfaces()[atePort3.Name]}

	testTraffic(t, args.ate, args.top, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, args, weights...)
}
