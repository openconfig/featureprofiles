package cisco_gribi_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/cisco/config"
	"github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/ygot/ygot"
	//"github.com/google/go-cmp/cmp"
)

func testRemAddHWModule(ctx context.Context, t *testing.T, args *testArgs) {
	t.Helper()
	defer flushServer(t, args)

	weights := []float64{10 * 15, 20 * 15, 30 * 15, 10 * 85, 20 * 85, 30 * 85, 40 * 85}
	srcEndPoint := args.top.Interfaces()[atePort1.Name]

	// disable hwmodule and reload and expect the traffic to be failed even after adding gribi routes
	t.Log("Trying  no hw-module profile pbr vrf-redirect")
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
	t.Log("Trying hw-module profile pbr vrf-redirect")
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

	weights := []float64{10 * 15, 20 * 15, 30 * 15, 10 * 85, 20 * 85, 30 * 85, 40 * 85}
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
	testTraffic(t, false, args.ate, args.top, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, args, 0, weights...)

	// add PBR config back and expect the traffic to be passed after adding gribi routes
	t.Log("Adding PBR Config and Intreface again and checking traffic, the traffic should pass")
	config.GNMICommitReplace(context.Background(), t, args.dut, baseConfig)
	testTraffic(t, true, args.ate, args.top, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, args, 0, weights...)
}

func getBasePBROCConfig(t *testing.T, args *testArgs) (ygot.PathStruct, interface{}) {
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

	return args.dut.Config().NetworkInstance("default").PolicyForwarding(), &policy

}

func getPartialPBROCConfig(t *testing.T, args *testArgs) (ygot.PathStruct, interface{}) {
	r1 := telemetry.NetworkInstance_PolicyForwarding_Policy_Rule{}
	r1.SequenceId = ygot.Uint32(1)
	r1.Ipv4 = &telemetry.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
		Protocol: telemetry.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP,
	}
	r1.Action = &telemetry.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String("TE")}

	r2 := telemetry.NetworkInstance_PolicyForwarding_Policy_Rule{}
	r2.SequenceId = ygot.Uint32(2)
	r2.Ipv4 = &telemetry.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
		//DscpSet: []uint8{*ygot.Uint8(14)}, // wrong value
		DscpSet: []uint8{*ygot.Uint8(17)}, // wrong value
	}
	r2.Action = &telemetry.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String("TE")}

	r3 := telemetry.NetworkInstance_PolicyForwarding_Policy_Rule{}
	r3.SequenceId = ygot.Uint32(3)
	r3.Ipv4 = &telemetry.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
		//DscpSet: []uint8{*ygot.Uint8(15)}, //wrong value
		DscpSet: []uint8{*ygot.Uint8(19)}, //wrong value
	}
	r3.Action = &telemetry.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String("VRF1")}

	r4 := telemetry.NetworkInstance_PolicyForwarding_Policy_Rule{}
	r4.SequenceId = ygot.Uint32(4)
	r4.Ipv4 = &telemetry.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
		DscpSet: []uint8{*ygot.Uint8(49)}, // wrong value
	}
	//r4.Action = &telemetry.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String("TE")}

	p := telemetry.NetworkInstance_PolicyForwarding_Policy{}
	p.PolicyId = ygot.String(pbrName)
	p.Type = telemetry.Policy_Type_VRF_SELECTION_POLICY
	p.Rule = map[uint32]*telemetry.NetworkInstance_PolicyForwarding_Policy_Rule{1: &r1, 2: &r2, 3: &r3, 4: &r4}

	policy := telemetry.NetworkInstance_PolicyForwarding{}
	policy.Policy = map[string]*telemetry.NetworkInstance_PolicyForwarding_Policy{pbrName: &p}

	return args.dut.Config().NetworkInstance("default").PolicyForwarding(), &policy

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

	defer flushServer(t, args)
	baseConfig := removeConfHeader(config.CMDViaGNMI(ctx, t, args.dut, "show running-config"))
	defer config.GNMICommitReplace(context.Background(), t, args.dut, baseConfig)
	baseConfigWithoutHWModule := removeHWModuleFromBaseConfing(t, baseConfig)
	baseConfigWithoutPBR := removePBRFromBaseConfing(t, baseConfig)
	baseConfigWithoutPBR = addHWModule(t, baseConfigWithoutPBR) // in case if it is missing
	fmt.Print(baseConfigWithoutPBR)

	weights := []float64{10 * 15, 20 * 15, 30 * 15, 10 * 85, 20 * 85, 30 * 85, 40 * 85}
	srcEndPoint := args.top.Interfaces()[atePort1.Name]

	// remove  HWModule and set PBR to wrong config,  reload the router and  expect the traffic to be failed even after adding gribi routes
	t.Log("Remove  HWModule and set PBR to wrong config, reload the router and check the traffic")
	path, wrongPolicy := getPartialPBROCConfig(t, args)
	config.GNMICommitReplaceWithOC(context.Background(), t, args.dut, baseConfigWithoutHWModule, path, wrongPolicy)
	config.Reload(context.Background(), t, args.dut, "", "", 6*time.Minute)
	args.clientA.StartWithNoCache(t)
	args.clientA.BecomeLeader(t)
	configureBaseDoubleRecusionVip1Entry(ctx, t, args)
	configureBaseDoubleRecusionVip2Entry(ctx, t, args)
	configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)
	testTraffic(t, false, args.ate, args.top, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, args, 0, weights...)

	// add PBR with OC and HWModule with text, then reload and expect the traffic to be passed after adding gribi routes
	path, basePolicy := getBasePBROCConfig(t, args)
	config.GNMICommitReplaceWithOC(context.Background(), t, args.dut, baseConfigWithoutPBR, path, basePolicy)
	t.Log("Add HWModule and set PBR to the right config, reload the router and check the traffic")
	/*result := args.dut.Config().NetworkInstance("default").PolicyForwarding().Get(t)
	if cmp.Diff(result,basePolicy)!="" {
		fmt.Println(cmp.Diff(result,basePolicy))
		// TODO: make the test case fail
	}*/
	config.Reload(context.Background(), t, args.dut, "", "", 6*time.Minute)
	args.clientA.StartWithNoCache(t)
	args.clientA.BecomeLeader(t)
	configureBaseDoubleRecusionVip1Entry(ctx, t, args)
	configureBaseDoubleRecusionVip2Entry(ctx, t, args)
	configureBaseDoubleRecusionVrfEntry(ctx, t, args.prefix.scale, args.prefix.host, "32", args)
	testTraffic(t, true, args.ate, args.top, srcEndPoint, args.top.Interfaces(), args.prefix.scale, args.prefix.host, args, 0, weights...)
}
