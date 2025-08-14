// Package mpls_gue_ipv4_decap_scale_test tests mplsogue decap functionality.
package mpls_gue_ipv4_decap_scale_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/iputil"
	otgconfighelpers "github.com/openconfig/featureprofiles/internal/otg_helpers/otg_config_helpers"
	otgvalidationhelpers "github.com/openconfig/featureprofiles/internal/otg_helpers/otg_validation_helpers"
	"github.com/openconfig/featureprofiles/internal/otg_helpers/packetvalidationhelpers"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygot/ygot"
)

// TestMain calls main function.
func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	ethernetCsmacd         = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	ieee8023adLag          = oc.IETFInterfaces_InterfaceType_ieee8023adLag
	mplsLabelCount         = 2000
	intCount               = 2000
	mplsV4Label            = 99991
	mplsV6Label            = 110993
	dutIntStartIP          = "169.254.0.1"
	otgIntStartIP          = "169.254.0.2"
	dutIntStartIpV6        = "2000:0:0:1::1"
	otgIntStartIpV6        = "2000:0:0:1::2"
	intStepV4              = "0.0.0.4"
	intStepV6              = "0:0:0:1::"
	staticRoutePrefix      = "10.99.1.0/24"
	staticRouteNextHop     = "194.0.2.2"
	outerSrcIpv4           = "100.64.0.1"
	outerDstIpv4           = "11.1.1.1"
	innerSrcIpv4           = "22.1.1.1"
	innerDstIpv4           = "21.1.1.1"
	innerSrcIpv6           = "2000:1::1"
	innerDstIpv6           = "3000:1::1"
	mcastDst               = "239.1.1.1"
	udpDstPort             = 6635
	flowSrcCount           = 10000
	dutIpv4Len             = 30
	dutIpv6Len             = 126
	dutMtu                 = 9202
	ratePps                = 100
	totalPkts              = 10000
	sleepTime              = 30
	carrierDelayUp         = 3000
	carrierDelayDown       = 150
	outerFlowRate          = 0
	innerTrafficClassCount = 0
	innerTrafficClass      = 10
	innerRawPriorityCount  = 0
	innerRawPriority       = 10
	innerSrcCount          = 0
	innerSrcPort           = 49152
)

var (
	top       = gosnappi.NewConfig()
	aggID     string
	custPorts = []string{"port1", "port2"}
	corePorts = []string{"port3", "port4"}
	coreIntf  = attrs.Attributes{
		Desc:    "Core_Interface",
		IPv4:    "194.0.2.1",
		IPv6:    "2001:10:1:6::1",
		MTU:     9202,
		IPv4Len: 24,
		IPv6Len: 126,
	}

	agg1 = &otgconfighelpers.Port{
		Name:        "Port-Channel1",
		AggMAC:      "02:00:01:01:01:07",
		MemberPorts: []string{"port1", "port2"},
		LagID:       1,
		IsLag:       true,
	}
	agg2 = &otgconfighelpers.Port{
		Name:        "Port-Channel2",
		AggMAC:      "02:00:01:01:01:01",
		MemberPorts: []string{"port3", "port4"},
		Interfaces:  []*otgconfighelpers.InterfaceProperties{agg2interface},
		LagID:       2,
		IsLag:       true,
	}

	agg2interface = &otgconfighelpers.InterfaceProperties{
		IPv4:        "194.0.2.2",
		IPv6:        "2001:10:1:6::2",
		IPv4Gateway: "194.0.2.1",
		IPv6Gateway: "2001:10:1:6::1",
		Name:        "Port-Channel2",
		MAC:         "02:00:01:01:01:02",
		IPv4Len:     29,
		IPv6Len:     126,
	}

	// Custom IMIX settings for all flows.
	sizeWeightProfile = []otgconfighelpers.SizeWeightPair{
		{Size: 64, Weight: 20},
		{Size: 128, Weight: 20},
		{Size: 256, Weight: 20},
		{Size: 512, Weight: 10},
		{Size: 1500, Weight: 30},
	}

	flowResolveArp = &otgvalidationhelpers.OTGValidation{
		Interface: &otgvalidationhelpers.InterfaceParams{Names: []string{agg2.Name}},
	}
	// flowOuterIPv4 Decap IPv4 Interface IPv4 Payload traffic params Outer Header.
	flowOuterIPv4 = &otgconfighelpers.Flow{
		TxNames:           []string{agg2.Interfaces[0].Name + ".IPv4"},
		RxNames:           []string{},
		SizeWeightProfile: &sizeWeightProfile,
		Flowrate:          100,
		FlowName:          "MPLSOGUE-IPv4-Traffic",
		EthFlow:           &otgconfighelpers.EthFlowParams{SrcMAC: agg2.AggMAC},
		IPv4Flow:          &otgconfighelpers.IPv4FlowParams{IPv4Src: outerSrcIpv4, IPv4Dst: outerDstIpv4, IPv4SrcCount: flowSrcCount},
		MPLSFlow:          &otgconfighelpers.MPLSFlowParams{MPLSLabel: mplsV4Label, MPLSExp: 7, MPLSLabelCount: mplsLabelCount},
		UDPFlow:           &otgconfighelpers.UDPFlowParams{UDPSrcPort: innerSrcPort, UDPDstPort: udpDstPort},
	}
	// flowOuterIPv4Validation MPLSOGUE traffic IPv4 interface IPv4 Payload.
	flowOuterIPv4Validation = &otgvalidationhelpers.OTGValidation{
		Interface: &otgvalidationhelpers.InterfaceParams{Names: []string{}, Ports: append(agg1.MemberPorts, agg2.MemberPorts...)},
		Flow:      &otgvalidationhelpers.FlowParams{Name: flowOuterIPv4.FlowName, TolerancePct: 5},
	}
	// flowInnerIPv4 Inner Header IPv4 Payload.
	flowInnerIPv4 = &otgconfighelpers.Flow{
		IPv4Flow: &otgconfighelpers.IPv4FlowParams{IPv4Src: innerSrcIpv4, IPv4Dst: innerDstIpv4, IPv4SrcCount: flowSrcCount, RawPriority: 0, RawPriorityCount: 255},
		TCPFlow:  &otgconfighelpers.TCPFlowParams{TCPSrcPort: innerSrcPort, TCPDstPort: 80, TCPSrcCount: flowSrcCount},
	}
	// flowOuterIPv6 Decap IPv6 Interface IPv6 Payload traffic params Outer Header.
	flowOuterIPv6 = &otgconfighelpers.Flow{
		TxNames:           []string{agg2.Interfaces[0].Name + ".IPv4"},
		RxNames:           []string{},
		SizeWeightProfile: &sizeWeightProfile,
		Flowrate:          100,
		FlowName:          "MPLSOGUE-IPv6-Traffic",
		EthFlow:           &otgconfighelpers.EthFlowParams{SrcMAC: agg2.AggMAC},
		IPv4Flow:          &otgconfighelpers.IPv4FlowParams{IPv4Src: outerSrcIpv4, IPv4Dst: outerDstIpv4, IPv4SrcCount: flowSrcCount},
		MPLSFlow:          &otgconfighelpers.MPLSFlowParams{MPLSLabel: mplsV6Label, MPLSExp: 7, MPLSLabelCount: mplsLabelCount},
		UDPFlow:           &otgconfighelpers.UDPFlowParams{UDPSrcPort: innerSrcPort, UDPDstPort: udpDstPort},
	}
	// flowOuterIPv6Validation MPLSOGUE traffic IPv6 interface IPv6 Payload.
	flowOuterIPv6Validation = &otgvalidationhelpers.OTGValidation{
		Interface: &otgvalidationhelpers.InterfaceParams{Names: []string{}, Ports: append(agg1.MemberPorts, agg2.MemberPorts...)},
		Flow:      &otgvalidationhelpers.FlowParams{Name: flowOuterIPv6.FlowName, TolerancePct: 5},
	}
	// flowInnerIPv6 Inner Header IPv6 Payload.
	flowInnerIPv6 = &otgconfighelpers.Flow{
		IPv6Flow: &otgconfighelpers.IPv6FlowParams{IPv6Src: innerSrcIpv6, IPv6Dst: innerDstIpv6, IPv6SrcCount: flowSrcCount, TrafficClass: 0, TrafficClassCount: 255},
		TCPFlow:  &otgconfighelpers.TCPFlowParams{TCPSrcPort: innerSrcPort, TCPDstPort: 80, TCPSrcCount: flowSrcCount},
	}
	// flowOuterMcast is the “outer” MPLS‐encapsulated flow whose payload is an IPv4+UDP multicast packet.
	flowOuterMcast = &otgconfighelpers.Flow{
		TxNames:           []string{agg2.Interfaces[0].Name + ".IPv4"},
		RxNames:           []string{},
		SizeWeightProfile: &sizeWeightProfile,
		Flowrate:          100,
		FlowName:          "MPLSoGUE-Mcast-Traffic",
		EthFlow:           &otgconfighelpers.EthFlowParams{SrcMAC: agg2.AggMAC},
		IPv4Flow:          &otgconfighelpers.IPv4FlowParams{IPv4Src: outerSrcIpv4, IPv4Dst: outerDstIpv4, IPv4SrcCount: flowSrcCount},
		MPLSFlow:          &otgconfighelpers.MPLSFlowParams{MPLSLabel: mplsV4Label, MPLSExp: 7, MPLSLabelCount: mplsLabelCount},
		UDPFlow:           &otgconfighelpers.UDPFlowParams{UDPSrcPort: innerSrcPort, UDPDstPort: udpDstPort},
	}
	// flowOuterIPv4McastValidation MPLSOGUE traffic IPv4 interface IPv4 Payload.
	flowOuterIPv4McastValidation = &otgvalidationhelpers.OTGValidation{
		Interface: &otgvalidationhelpers.InterfaceParams{Names: []string{}, Ports: append(agg1.MemberPorts, agg2.MemberPorts...)},
		Flow:      &otgvalidationhelpers.FlowParams{Name: flowOuterMcast.FlowName, TolerancePct: 5},
	}
	// flowInnerMcast is the “inner” multicast payload (IPv4 + UDP to the same group).
	flowInnerMcast = &otgconfighelpers.Flow{
		IPv4Flow: &otgconfighelpers.IPv4FlowParams{IPv4Src: innerSrcIpv4, IPv4Dst: mcastDst, IPv4SrcCount: flowSrcCount, RawPriority: 0, RawPriorityCount: 255},
		TCPFlow:  &otgconfighelpers.TCPFlowParams{TCPSrcPort: innerSrcPort, TCPDstPort: 80, TCPSrcCount: flowSrcCount},
	}
	validationsIPv4 = []packetvalidationhelpers.ValidationType{
		packetvalidationhelpers.ValidateIPv4Header,
		packetvalidationhelpers.ValidateTCPHeader,
	}
	validationsIPv6 = []packetvalidationhelpers.ValidationType{
		packetvalidationhelpers.ValidateIPv6Header,
	}
	decapValidationIPv4 = &packetvalidationhelpers.PacketValidation{
		PortName:    "port1",
		CaptureName: "ipv4_decap",
		Validations: validationsIPv4,
		IPv4Layer:   &packetvalidationhelpers.IPv4Layer{DstIP: innerDstIpv4, Tos: 10, TTL: 64, Protocol: packetvalidationhelpers.TCP},
		TCPLayer:    &packetvalidationhelpers.TCPLayer{SrcPort: 49152, DstPort: 80},
	}
	decapValidationIPv6 = &packetvalidationhelpers.PacketValidation{
		PortName:    "port2",
		CaptureName: "ipv6_decap",
		Validations: validationsIPv6,
		IPv6Layer:   &packetvalidationhelpers.IPv6Layer{DstIP: innerDstIpv6, TrafficClass: 10, HopLimit: 64},
	}
	lagECMPValidation = &otgvalidationhelpers.OTGValidation{
		Interface: &otgvalidationhelpers.InterfaceParams{Ports: agg1.MemberPorts},
		Flow:      &otgvalidationhelpers.FlowParams{Name: flowOuterIPv4.FlowName},
	}
	interfaces = []*attrs.Attributes{}
)

type networkConfig struct {
	DutIPs   []string
	OtgIPs   []string
	OtgMACs  []string
	DutIPsV6 []string
	OtgIPsV6 []string
}

// generateNetConfig generates and returns a networkConfig object containing
// IPv4, IPv6, and MAC address allocations for both DUT and OTG interfaces.
func generateNetConfig(intCount int) (*networkConfig, error) {
	dutIPs, err := iputil.GenerateIPv4sWithStep(dutIntStartIP, intCount, intStepV4)
	if err != nil {
		return nil, fmt.Errorf("failed to generate DUT IPs: %w", err)
	}

	otgIPs, err := iputil.GenerateIPv4sWithStep(otgIntStartIP, intCount, intStepV4)
	if err != nil {
		return nil, fmt.Errorf("failed to generate OTG IPs: %w", err)
	}

	otgMACs, err := iputil.GenerateMACs("00:00:00:00:00:AA", intCount, "00:00:00:00:00:01")
	if err != nil {
		return nil, fmt.Errorf("failed to generate MACs: %v", err)
	}
	dutIPsV6, err := iputil.GenerateIPv6sWithStep(dutIntStartIpV6, intCount, intStepV6)
	if err != nil {
		return nil, fmt.Errorf("failed to generate DUT IPv6s: %w", err)
	}

	otgIPsV6, err := iputil.GenerateIPv6sWithStep(otgIntStartIpV6, intCount, intStepV6)
	if err != nil {
		return nil, fmt.Errorf("failed to generate OTG IPv6s: %w", err)
	}

	return &networkConfig{
		DutIPs:   dutIPs,
		OtgIPs:   otgIPs,
		OtgMACs:  otgMACs,
		DutIPsV6: dutIPsV6,
		OtgIPsV6: otgIPsV6,
	}, nil
}

// configureOTG sets up the Open Traffic Generator (OTG) test configuration
func configureOTG(t *testing.T) {
	t.Helper()
	top.Captures().Clear()
	ate := ondatra.ATE(t, "ate")

	// Create a slice of aggPortData for easier iteration
	aggs := []*otgconfighelpers.Port{agg1, agg2}

	// Configure OTG Interfaces
	for _, agg := range aggs {
		otgconfighelpers.ConfigureNetworkInterface(t, top, ate, agg)
	}
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
}

// PF-1.20.1: Generate DUT Configuration
func configureDut(t *testing.T, dut *ondatra.DUTDevice, netConfig *networkConfig, mplsStaticLabels []int, mplsStaticLabelsForIpv6 []int, ocPFParams cfgplugins.OcPolicyForwardingParams) {
	aggID = netutil.NextAggregateInterface(t, dut)

	for i := range intCount {
		interfaces = append(interfaces, &attrs.Attributes{
			Desc:         "Customer_connect",
			MTU:          dutMtu,
			IPv4:         netConfig.DutIPs[i],
			IPv4Len:      dutIpv4Len,
			IPv6:         netConfig.DutIPsV6[i],
			IPv6Len:      dutIpv6Len,
			Subinterface: uint32(i + 1),
		})
	}
	configureInterfaces(t, dut, custPorts, interfaces, aggID)
	aggID = netutil.NextAggregateInterface(t, dut)
	configureInterfaces(t, dut, corePorts, []*attrs.Attributes{&coreIntf}, aggID)
	configureStaticRoute(t, dut)
	_, ni, pf := cfgplugins.SetupPolicyForwardingInfraOC(ocPFParams.NetworkInstanceName)
	decapMPLSInGUE(t, dut, pf, ni, netConfig, mplsStaticLabels, mplsStaticLabelsForIpv6, ocPFParams)
}

func TestSetup(t *testing.T) {
	t.Log("PF-1.20.1: Generate DUT Configuration")
	dut := ondatra.DUT(t, "dut")
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	netConfig, err := generateNetConfig(intCount)
	if err != nil {
		t.Fatalf("Error generating net config: %v", err)
	}
	mplsStaticLabels := func() []int {
		r := make([]int, mplsLabelCount)
		for i := range r {
			r[i] = mplsV4Label + i
		}
		return r
	}()

	mplsStaticLabelsForIpv6 := func() []int {
		r := make([]int, mplsLabelCount)
		for i := range r {
			r[i] = mplsV6Label + i
		}
		return r
	}()

	var interfaces []*otgconfighelpers.InterfaceProperties

	for i := range intCount {
		agg1.Interfaces = append(interfaces, &otgconfighelpers.InterfaceProperties{
			Name:        fmt.Sprintf("agg1port%d", i+1),
			IPv4:        netConfig.OtgIPs[i],
			IPv4Gateway: netConfig.DutIPs[i],
			Vlan:        uint32(i + 1),
			IPv4Len:     dutIpv4Len,
			IPv6:        netConfig.OtgIPsV6[i],
			IPv6Gateway: netConfig.DutIPsV6[i],
			IPv6Len:     dutIpv6Len,
			MAC:         netConfig.OtgMACs[i],
		})
	}

	// Get default parameters for OC Policy Forwarding
	ocPFParams := defaultOcPolicyForwardingParams()

	// Pass ocPFParams to configureDut
	configureDut(t, dut, netConfig, mplsStaticLabels, mplsStaticLabelsForIpv6, ocPFParams)
	// after agg1.Interfaces has been populated...
	for _, intf := range agg1.Interfaces {
		// tell the validator which ingress interfaces to watch
		flowOuterIPv4Validation.Interface.Names = append(
			flowOuterIPv4Validation.Interface.Names,
			intf.Name,
		)
		// tell the flow which Rx device names to bind to
		flowOuterIPv4.RxNames = append(
			flowOuterIPv4.RxNames,
			intf.Name+".IPv4",
		)

		flowOuterIPv6Validation.Interface.Names = append(
			flowOuterIPv6Validation.Interface.Names,
			intf.Name,
		)
		flowOuterIPv6.RxNames = append(
			flowOuterIPv6.RxNames,
			intf.Name+".IPv6",
		)
		// and for multicast:
		flowOuterIPv4McastValidation.Interface.Names = append(
			flowOuterIPv4McastValidation.Interface.Names,
			intf.Name,
		)
		flowOuterMcast.RxNames = append(
			flowOuterMcast.RxNames,
			intf.Name+".IPv4", // or whatever your mcast device name is
		)
	}
	configureOTG(t)
}

// defaultOcPolicyForwardingParams provides default parameters for the generator,
// matching the values in the provided JSON example.
func defaultOcPolicyForwardingParams() cfgplugins.OcPolicyForwardingParams {
	return cfgplugins.OcPolicyForwardingParams{
		NetworkInstanceName: "DEFAULT",
		InterfaceID:         "Agg1.10",
		AppliedPolicyName:   "customer1",
	}
}

// decapMPLSInGUE should also include the OC config , within these deviations there should be a switch statement is needed
// Modified to accept pf, ni, and ocPFParams
func decapMPLSInGUE(t *testing.T, dut *ondatra.DUTDevice, pf *oc.NetworkInstance_PolicyForwarding, ni *oc.NetworkInstance, netConfig *networkConfig, mplsStaticLabels []int, mplsStaticLabelsForIpv6 []int, ocPFParams cfgplugins.OcPolicyForwardingParams) {
	cfgplugins.MplsConfig(t, dut)
	cfgplugins.QosClassificationConfig(t, dut)
	cfgplugins.LabelRangeConfig(t, dut)
	cfgplugins.DecapGroupConfigGue(t, dut, pf, ocPFParams)
	cfgplugins.MPLSStaticLSPScaleConfig(t, dut, ni, netConfig.OtgIPs, netConfig.OtgIPsV6, mplsStaticLabels, mplsStaticLabelsForIpv6, ocPFParams)
	if !deviations.PolicyForwardingOCUnsupported(dut) {
		pushPolicyForwardingConfig(t, dut, ni)
	}
}

// PF-1.20.2: Verify PF MPLSoGUE Decap action for IPv4 and IPv6 traffic.
func TestMPLSOGUEDecapIPv4AndIPv6(t *testing.T) {
	ate := ondatra.ATE(t, "ate")
	t.Log("Verify MPLSoGUE decapsulate action for IPv4 payload")
	createflow(t, top, flowOuterIPv4, flowInnerIPv4, true)
	time.Sleep(sleepTime * time.Second) // Scale test taking time to create flow
	sendTraffic(t, ate, ate.OTG())
	if err := flowOuterIPv4Validation.ValidateLossOnFlows(t, ate); err != nil {
		t.Errorf("ValidateLossOnFlows(): got err: %q, want nil", err)
	}
	if err := lagECMPValidation.ValidateECMPonLAGWithTolPer(t, ate); err != nil {
		t.Errorf("ECMPValidationFailed(): got err: %q, want nil", err)
	}
	t.Log("Verify MPLSoGUE decapsulate action for Multicast IPv4 payload")
	createflow(t, top, flowOuterMcast, flowInnerMcast, true)
	sendTraffic(t, ate, ate.OTG())
	if err := flowOuterIPv4McastValidation.ValidateLossOnFlows(t, ate); err != nil {
		t.Errorf("ValidateLossOnFlows(): got err: %q, want nil", err)
	}
	t.Log("Verify MPLSoGUE decapsulate action for IPv6 payload")
	createflow(t, top, flowOuterIPv6, flowInnerIPv6, true)
	time.Sleep(sleepTime * time.Second) // Scale test taking time to create flow
	sendTraffic(t, ate, ate.OTG())
	if err := flowOuterIPv6Validation.ValidateLossOnFlows(t, ate); err != nil {
		t.Errorf("ValidateLossOnFlows(): got err: %q, want nil", err)
	}
}

// Verify MPLSoGUE DSCP/TTL preserve operation.
func TestMPLSOGUEDecapInnerPayloadPreserve(t *testing.T) {
	ate := ondatra.ATE(t, "ate")
	t.Log("Verify MPLSoGUE DSCP/TTL preserve operation")
	flowOuterIPv4.FlowName = "MPLSOGUE-IPv4-Traffic"
	packetvalidationhelpers.ClearCapture(t, top, ate)
	updateFlow(t, flowOuterIPv4, flowInnerIPv4, true, ratePps, totalPkts)
	time.Sleep(sleepTime * time.Second) // Scale test taking time to update flow
	packetvalidationhelpers.ConfigurePacketCapture(t, top, decapValidationIPv4)
	sendTrafficCapture(t, ate, ate.OTG())
	if err := flowOuterIPv4Validation.ValidateLossOnFlows(t, ate); err != nil {
		t.Errorf("ValidateLossOnFlows(): got err: %q, want nil", err)
	}
	if err := packetvalidationhelpers.CaptureAndValidatePackets(t, ate, decapValidationIPv4); err != nil {
		t.Errorf("CaptureAndValidatePackets(): got err: %q", err)
	}
	updateFlow(t, flowOuterIPv6, flowInnerIPv6, true, ratePps, totalPkts)
	time.Sleep(sleepTime * time.Second) // Scale test taking time to update flow
	packetvalidationhelpers.ConfigurePacketCapture(t, top, decapValidationIPv6)
	sendTrafficCapture(t, ate, ate.OTG())
	if err := packetvalidationhelpers.CaptureAndValidatePackets(t, ate, decapValidationIPv6); err != nil {
		t.Errorf("CaptureAndValidatePackets(): got err: %q", err)
	}
}

func sendTraffic(t *testing.T, ate *ondatra.ATEDevice, config *otg.OTG) {
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	time.Sleep(sleepTime * time.Second) // Scale test taking time to bring up all the protocols
	flowResolveArp.IsIPv4Interfaceresolved(t, ate)
	ate.OTG().StartTraffic(t)
	time.Sleep(sleepTime * time.Second)
	ate.OTG().StopTraffic(t)
	otgutils.LogFlowMetrics(t, config, top)
	otgutils.LogPortMetrics(t, config, top)
}

func sendTrafficCapture(t *testing.T, ate *ondatra.ATEDevice, config *otg.OTG) {
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	time.Sleep(sleepTime * time.Second) // Scale test taking time to bring up all the protocols
	flowResolveArp.IsIPv4Interfaceresolved(t, ate)
	cs := packetvalidationhelpers.StartCapture(t, ate)
	ate.OTG().StartTraffic(t)
	time.Sleep(sleepTime * time.Second)
	ate.OTG().StopTraffic(t)
	packetvalidationhelpers.StopCapture(t, ate, cs)
}

func createflow(t *testing.T, top gosnappi.Config, paramsOuter *otgconfighelpers.Flow, paramsInner *otgconfighelpers.Flow, clearFlows bool) {
	if clearFlows {
		top.Flows().Clear()
	}
	paramsOuter.CreateFlow(top)
	paramsOuter.AddEthHeader()
	paramsOuter.AddIPv4Header()
	paramsOuter.AddUDPHeader()
	paramsOuter.AddMPLSHeader()
	if paramsInner.IPv4Flow != nil {
		*paramsOuter.IPv4Flow = *paramsInner.IPv4Flow
		paramsOuter.AddIPv4Header()
	}
	if paramsInner.IPv6Flow != nil {
		paramsOuter.IPv6Flow = paramsInner.IPv6Flow
		paramsOuter.AddIPv6Header()
	}
	if paramsInner.TCPFlow != nil {
		paramsOuter.TCPFlow = paramsInner.TCPFlow
		paramsOuter.AddTCPHeader()
	}
	if paramsInner.UDPFlow != nil {
		paramsOuter.UDPFlow = paramsInner.UDPFlow
		paramsOuter.AddUDPHeader()
	}
}

func updateFlow(t *testing.T, paramsOuter *otgconfighelpers.Flow, paramsInner *otgconfighelpers.Flow, clearFlows bool, pps uint64, totalPackets uint32) {
	paramsOuter.PacketsToSend = totalPackets
	paramsOuter.PpsRate = pps
	paramsOuter.Flowrate = outerFlowRate
	if paramsInner.IPv6Flow != nil {
		paramsInner.IPv6Flow.TrafficClassCount = innerTrafficClassCount
		paramsInner.IPv6Flow.TrafficClass = innerTrafficClass
	}
	if paramsInner.IPv4Flow != nil {
		paramsInner.IPv4Flow.RawPriorityCount = innerRawPriorityCount
		paramsInner.IPv4Flow.RawPriority = innerRawPriority
		if paramsInner.TCPFlow != nil {
			paramsInner.TCPFlow.TCPSrcCount = innerSrcCount
			paramsInner.TCPFlow.TCPSrcPort = innerSrcPort
		}
		paramsOuter.IPv4Flow.IPv4Src = outerSrcIpv4
		paramsOuter.IPv4Flow.IPv4Dst = outerDstIpv4
	}
	createflow(t, top, paramsOuter, paramsInner, clearFlows)
}

func configureInterfaces(t *testing.T, dut *ondatra.DUTDevice, dutPorts []string, subinterfaces []*attrs.Attributes, aggID string) {
	t.Helper()
	d := gnmi.OC()
	dutAggPorts := []*ondatra.Port{}
	for _, port := range dutPorts {
		dutAggPorts = append(dutAggPorts, dut.Port(t, port))
	}
	if deviations.AggregateAtomicUpdate(dut) {
		cfgplugins.DeleteAggregate(t, dut, aggID, dutAggPorts)
		cfgplugins.SetupAggregateAtomically(t, dut, aggID, dutAggPorts)
	}

	lacp := &oc.Lacp_Interface{Name: ygot.String(aggID)}
	lacp.LacpMode = oc.Lacp_LacpActivityType_ACTIVE
	lacpPath := d.Lacp().Interface(aggID)
	fptest.LogQuery(t, "LACP", lacpPath.Config(), lacp)
	gnmi.Replace(t, dut, lacpPath.Config(), lacp)
	// TODO - to remove this sleep later
	time.Sleep(5 * time.Second)

	agg := &oc.Interface{Name: ygot.String(aggID)}
	configDUTInterface(agg, subinterfaces, dut)
	agg.GetOrCreateAggregation().LagType = oc.IfAggregate_AggregationType_LACP
	agg.Type = ieee8023adLag
	aggPath := d.Interface(aggID)
	fptest.LogQuery(t, aggID, aggPath.Config(), agg)
	gnmi.Replace(t, dut, aggPath.Config(), agg)

	for _, port := range dutAggPorts {
		holdTimeConfig := &oc.Interface_HoldTime{
			Up:   ygot.Uint32(carrierDelayUp),
			Down: ygot.Uint32(carrierDelayDown),
		}
		intfPath := gnmi.OC().Interface(port.Name())
		gnmi.Update(t, dut, intfPath.HoldTime().Config(), holdTimeConfig)
	}
}
func configDUTInterface(i *oc.Interface, subinterfaces []*attrs.Attributes, dut *ondatra.DUTDevice) {
	for _, a := range subinterfaces {
		i.Description = ygot.String(a.Desc)
		if deviations.InterfaceEnabled(dut) {
			i.Enabled = ygot.Bool(true)
		}
		s1 := i.GetOrCreateSubinterface(0)
		b4 := s1.GetOrCreateIpv4()
		b6 := s1.GetOrCreateIpv6()
		b4.Mtu = ygot.Uint16(a.MTU)
		b6.Mtu = ygot.Uint32(uint32(a.MTU))
		if deviations.InterfaceEnabled(dut) {
			b4.Enabled = ygot.Bool(true)
		}
		if a.Subinterface != 0 {
			s := i.GetOrCreateSubinterface(a.Subinterface)
			s.GetOrCreateVlan().GetOrCreateMatch().GetOrCreateSingleTagged().SetVlanId(uint16(a.Subinterface))
			configureInterfaceAddress(dut, s, a)
		} else {
			configureInterfaceAddress(dut, s1, a)
		}
	}
}
func configureInterfaceAddress(dut *ondatra.DUTDevice, s *oc.Interface_Subinterface, a *attrs.Attributes) {
	s4 := s.GetOrCreateIpv4()
	if deviations.InterfaceEnabled(dut) {
		s4.Enabled = ygot.Bool(true)
	}
	if a.IPv4 != "" {
		a4 := s4.GetOrCreateAddress(a.IPv4)
		a4.PrefixLength = ygot.Uint8(a.IPv4Len)
	}
	s6 := s.GetOrCreateIpv6()
	if deviations.InterfaceEnabled(dut) {
		s6.Enabled = ygot.Bool(true)
	}
	if a.IPv6 != "" {
		s6.GetOrCreateAddress(a.IPv6).PrefixLength = ygot.Uint8(a.IPv6Len)
	}

	if a.IPv6Sec != "" {
		s6_2 := s.GetOrCreateIpv6()
		if deviations.InterfaceEnabled(dut) {
			s6_2.Enabled = ygot.Bool(true)
		}
		s6_2.GetOrCreateAddress(a.IPv6Sec).PrefixLength = ygot.Uint8(a.IPv6Len)
	}
}

func configureStaticRoute(t *testing.T, dut *ondatra.DUTDevice) {
	b := new(gnmi.SetBatch)
	sV4 := &cfgplugins.StaticRouteCfg{
		NetworkInstance: deviations.DefaultNetworkInstance(dut),
		Prefix:          staticRoutePrefix,
		NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
			"0": oc.UnionString(staticRouteNextHop),
		},
	}
	if _, err := cfgplugins.NewStaticRouteCfg(b, sV4, dut); err != nil {
		t.Fatalf("Failed to configure IPv4 static route: %v", err)
	}
	b.Set(t, dut)
}

// pushPolicyForwardingConfig pushes the given policy forwarding configuration
// for the specified network instance to the DUT via gNMI Replace.
func pushPolicyForwardingConfig(t *testing.T, dut *ondatra.DUTDevice, ni *oc.NetworkInstance) {
	t.Helper()
	niPath := gnmi.OC().NetworkInstance(ni.GetName()).Config()
	gnmi.Replace(t, dut, niPath, ni)
}
