package policy_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/cisco/config"
	ciscoFlags "github.com/openconfig/featureprofiles/internal/cisco/flags"
	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

func testRemAddHWModule(ctx context.Context, t *testing.T, args *testArgs) {

	if !*ciscoFlags.PbrPrecommitTests {
		t.Skip()
	}

	t.Helper()
	defer flushServer(t, args)

	// weights := []float64{10 * 15, 20 * 15, 30 * 15, 10 * 85, 20 * 85, 30 * 85, 40 * 85}
	srcEndPoint := args.top.Interfaces()[atePort1.Name]

	// disable hwmodule and expect the traffic to be failed even after adding gribi routes
	t.Run("Trying no hw-module profile pbr vrf-redirect", func(t *testing.T) {

		configToChange := "no hw-module profile pbr vrf-redirect\n"
		util.GNMIWithText(ctx, t, args.dut, configToChange)
		args.clientA.StartWithNoCache(t)
		args.clientA.BecomeLeader(t)
		configureBaseDoubleRecusionVip1Entry(ctx, t, args)
		configureBaseDoubleRecusionVip2Entry(ctx, t, args)
		configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)
		testTraffic(t, true, args.ate, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, 10)
	})

	// enable hwmodule and expect the traffic to be passed after adding gribi routes
	t.Run("Trying hw-module profile pbr vrf-redirect", func(t *testing.T) {

		t.Log("Trying hw-module profile pbr vrf-redirect")
		beforeReloadConfig := "hw-module profile pbr vrf-redirect\n"
		util.GNMIWithText(ctx, t, args.dut, beforeReloadConfig)
		args.clientA.StartWithNoCache(t)
		args.clientA.BecomeLeader(t)
		configureBaseDoubleRecusionVip1Entry(ctx, t, args)
		configureBaseDoubleRecusionVip2Entry(ctx, t, args)
		configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)
		testTraffic(t, true, args.ate, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, 10)
	})

	// router reload and expect the traffic to be passed after adding gribi routes
	t.Run("Trying Reload the router", func(t *testing.T) {

		reloadDevice(t, args.dut)
		args.clientA.Close(t)
		time.Sleep(2 * time.Minute)
		if err := args.clientA.Start(t); err != nil {
			t.Fatalf("gRIBI Connection can not be established")
		}
		args.clientA.StartWithNoCache(t)
		args.clientA.BecomeLeader(t)
		configureBaseDoubleRecusionVip1Entry(ctx, t, args)
		configureBaseDoubleRecusionVip2Entry(ctx, t, args)
		configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)
		testTraffic(t, true, args.ate, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, 10)
	})
}

func removePBRFromBaseConfing(t *testing.T, baseConfig string) string {
	t.Helper()
	lines := strings.Split(baseConfig, "\n")
	linesWithoutPBR := []string{}
	skip := false
	// remove policy and class-maps
	for _, line := range lines {
		if strings.HasPrefix(line, "class-map") || strings.HasPrefix(line, "policy-map") {
			skip = true
		}
		if !skip {
			linesWithoutPBR = append(linesWithoutPBR, line)
		}
		if line == "! " {
			skip = false
		}
	}

	// prepare the modified baseConfig
	updatedBaseConf := ""
	for _, line := range linesWithoutPBR {
		if updatedBaseConf == "" {
			updatedBaseConf = line
			continue
		}
		updatedBaseConf = fmt.Sprintf("%s\n%s", updatedBaseConf, line)
	}
	return updatedBaseConf
}

func removeInterfacePBRFromBaseConfing(t *testing.T, baseConfig string) string {
	t.Helper()
	lines := strings.Split(baseConfig, "\n")
	// remove policy from interface
	linesWithoutPBRAndInterface := []string{}
	for _, line := range lines {
		if !strings.HasPrefix(line, " service-policy type pbr input") {
			linesWithoutPBRAndInterface = append(linesWithoutPBRAndInterface, line)
		}
	}
	// prepare the modified baseConfig
	updatedBaseConf := ""
	for _, line := range linesWithoutPBRAndInterface {
		if updatedBaseConf == "" {
			updatedBaseConf = line
			continue
		}
		updatedBaseConf = fmt.Sprintf("%s\n%s", updatedBaseConf, line)
	}
	return updatedBaseConf
}

func testRemAddPBRWithGNMIReplace(ctx context.Context, t *testing.T, args *testArgs) {
	t.Helper()
	defer flushServer(t, args)

	baseConfig := removeConfHeader(config.CMDViaGNMI(ctx, t, args.dut, "show running-config"))
	defer config.GNMICommitReplace(context.Background(), t, args.dut, baseConfig)
	baseConfigWithoutPBR := removePBRFromBaseConfing(t, baseConfig)
	baseConfigWithoutPBRAndInterface := removeInterfacePBRFromBaseConfing(t, baseConfigWithoutPBR)

	// weights := []float64{10 * 15, 20 * 15, 30 * 15, 10 * 85, 20 * 85, 30 * 85, 40 * 85}
	srcEndPoint := args.top.Interfaces()[atePort1.Name]

	t.Log("Adding GRIBI Entries")
	args.clientA.StartWithNoCache(t)
	args.clientA.BecomeLeader(t)
	configureBaseDoubleRecusionVip1Entry(ctx, t, args)
	configureBaseDoubleRecusionVip2Entry(ctx, t, args)
	configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)

	// remove PBR and expect the traffic to be failed even after adding gribi routes
	t.Log("Remowing PBR Config and Intreface and checking traffic, the traffic should fail")
	config.GNMICommitReplace(context.Background(), t, args.dut, baseConfigWithoutPBRAndInterface)
	testTraffic(t, false, args.ate, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, 0)

	// add PBR config back and expect the traffic to be passed after adding gribi routes
	t.Log("Adding PBR Config and Intreface again and checking traffic, the traffic should pass")
	config.GNMICommitReplace(context.Background(), t, args.dut, baseConfig)
	testTraffic(t, true, args.ate, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, 0)
}

// func getBasePBROCConfig() (ygnmi.PathStruct, interface{}) {
// 	r1 := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
// 	r1.SequenceId = ygot.Uint32(1)
// 	r1.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
// 		Protocol: oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP,
// 	}
// 	r1.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String(*ciscoFlags.NonDefaultNetworkInstance)}

// 	r2 := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
// 	r2.SequenceId = ygot.Uint32(2)
// 	r2.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
// 		DscpSet: []uint8{*ygot.Uint8(16)},
// 	}
// 	r2.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String(*ciscoFlags.NonDefaultNetworkInstance)}

// 	r3 := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
// 	r3.SequenceId = ygot.Uint32(3)
// 	r3.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
// 		DscpSet: []uint8{*ygot.Uint8(18)},
// 	}
// 	r3.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String("VRF1")}

// 	r4 := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
// 	r4.SequenceId = ygot.Uint32(4)
// 	r4.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
// 		DscpSet: []uint8{*ygot.Uint8(48)},
// 	}
// 	r4.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String(*ciscoFlags.NonDefaultNetworkInstance)}

// 	p := oc.NetworkInstance_PolicyForwarding_Policy{}
// 	p.PolicyId = ygot.String(pbrName)
// 	p.Type = oc.Policy_Type_VRF_SELECTION_POLICY
// 	p.Rule = map[uint32]*oc.NetworkInstance_PolicyForwarding_Policy_Rule{1: &r1, 2: &r2, 3: &r3, 4: &r4}

// 	policy := oc.NetworkInstance_PolicyForwarding{}
// 	policy.Policy = map[string]*oc.NetworkInstance_PolicyForwarding_Policy{pbrName: &p}

// 	return gnmi.OC().NetworkInstance(*ciscoFlags.PbrInstance).PolicyForwarding(), &policy
// }

func getPartialPBROCConfig(t *testing.T, args *testArgs) {
	fptest.ConfigureDefaultNetworkInstance(t, args.dut)
	r1 := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
	r1.SequenceId = ygot.Uint32(1)
	r1.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
		Protocol: oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP,
	}
	r1.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String(*ciscoFlags.NonDefaultNetworkInstance)}

	r2 := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
	r2.SequenceId = ygot.Uint32(2)
	r2.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
		DscpSet: []uint8{*ygot.Uint8(17)}, // wrong value
	}
	r2.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String(*ciscoFlags.NonDefaultNetworkInstance)}

	r3 := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
	r3.SequenceId = ygot.Uint32(3)
	r3.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
		DscpSet: []uint8{*ygot.Uint8(19)}, //wrong value
	}
	r3.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String("VRF1")}

	r4 := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
	r4.SequenceId = ygot.Uint32(4)
	r4.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
		DscpSet: []uint8{*ygot.Uint8(49)}, // wrong value
	}

	p := oc.NetworkInstance_PolicyForwarding_Policy{}
	p.PolicyId = ygot.String(pbrName)
	p.Type = oc.Policy_Type_VRF_SELECTION_POLICY
	p.Rule = map[uint32]*oc.NetworkInstance_PolicyForwarding_Policy_Rule{1: &r1, 2: &r2, 3: &r3, 4: &r4}

	policy := oc.NetworkInstance_PolicyForwarding{}
	policy.Policy = map[string]*oc.NetworkInstance_PolicyForwarding_Policy{pbrName: &p}
	gnmi.Replace(t, args.dut, gnmi.OC().NetworkInstance(*ciscoFlags.PbrInstance).PolicyForwarding().Config(), &policy)
}

func removeHWModuleFromBaseConfing(t *testing.T, baseConfig string) string {
	t.Helper()
	lines := strings.Split(baseConfig, "\n")
	// remove policy from interface
	linesWithoutHWModule := []string{}
	for _, line := range lines {
		if !strings.HasPrefix(line, "hw-module profile pbr vrf-redirect") {
			linesWithoutHWModule = append(linesWithoutHWModule, line)
		}
	}
	// prepare the modified baseConfig
	updatedBaseConf := ""
	for _, line := range linesWithoutHWModule {
		if updatedBaseConf == "" {
			updatedBaseConf = line
			continue
		}
		updatedBaseConf = fmt.Sprintf("%s\n%s", updatedBaseConf, line)
	}
	return updatedBaseConf

}

func addHWModule(t *testing.T, baseConfig string) string {
	t.Helper()
	lines := strings.Split(baseConfig, "\n")
	// remove policy from interface
	for _, line := range lines {
		if strings.HasPrefix(line, "hw-module profile pbr vrf-redirect") {
			return baseConfig
		}
	}
	// prepare the modified baseConfig
	updatedBaseConf := ""
	for _, line := range lines {
		if updatedBaseConf == "" {
			updatedBaseConf = line
			continue
		}
		if line == "end" {
			updatedBaseConf = fmt.Sprintf("%s\n%s", updatedBaseConf, "hw-module profile pbr vrf-redirect")
		}
		updatedBaseConf = fmt.Sprintf("%s\n%s", updatedBaseConf, line)
	}
	return updatedBaseConf

}

func removeConfHeader(baseConf string) string {
	lines := strings.Split(baseConf, "\n")
	skip := false
	baseConfig := ""
	for _, line := range lines {
		if skip {
			baseConfig = fmt.Sprintf("%s\n%s", baseConfig, line)
		}
		if strings.HasPrefix(line, "!! IOS XR Configuration") {
			skip = true
			baseConfig = line
		}
	}
	return baseConfig
}
func testRemAddHWWithGNMIReplaceAndPBRwithOC(ctx context.Context, t *testing.T, args *testArgs) {

	if !*ciscoFlags.PbrPrecommitTests {
		t.Skip()
	}
	configBasePBR(t, args.dut)
	defer flushServer(t, args)
	baseConfig := removeConfHeader(config.CMDViaGNMI(ctx, t, args.dut, "show running-config"))
	defer config.GNMICommitReplace(context.Background(), t, args.dut, baseConfig)
	baseConfigWithoutHWModule := removeHWModuleFromBaseConfing(t, baseConfig)
	baseConfigWithoutPBR := removePBRFromBaseConfing(t, baseConfig)
	baseConfigWithoutPBR = addHWModule(t, baseConfigWithoutPBR)
	t.Logf("BaseConfig: %s", baseConfig)
	t.Logf("BaseConfigWithoutPBR: %s", baseConfigWithoutPBR)

	// weights := []float64{10 * 15, 20 * 15, 30 * 15, 10 * 85, 20 * 85, 30 * 85, 40 * 85}
	srcEndPoint := args.top.Interfaces()[atePort1.Name]

	//remove  HWModule and set PBR to wrong config,  and  expect the traffic to be failed even after adding gribi routes
	t.Log("Remove  HWModule and set PBR to wrong config, reload the router and check the traffic")
	config.GNMICommitReplace(context.Background(), t, args.dut, baseConfigWithoutHWModule)
	getPartialPBROCConfig(t, args)
	args.clientA.StartWithNoCache(t)
	args.clientA.BecomeLeader(t)
	configureBaseDoubleRecusionVip1Entry(ctx, t, args)
	configureBaseDoubleRecusionVip2Entry(ctx, t, args)
	configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)
	testTraffic(t, false, args.ate, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, 0)

	// add PBR with OC and HWModule with text, and expect the traffic to be passed after adding gribi routes
	t.Log("Add HWModule and set PBR to the right config, reload the router and check the traffic")
	config.GNMICommitReplace(context.Background(), t, args.dut, baseConfig)
	args.clientA.StartWithNoCache(t)
	args.clientA.BecomeLeader(t)
	configureBaseDoubleRecusionVip1Entry(ctx, t, args)
	configureBaseDoubleRecusionVip2Entry(ctx, t, args)
	configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)

	testTraffic(t, true, args.ate, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, 0)
}
