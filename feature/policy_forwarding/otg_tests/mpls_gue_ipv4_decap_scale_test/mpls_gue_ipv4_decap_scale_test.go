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
	ieee8023adLag          = oc.IETFInterfaces_InterfaceType_ieee8023adLag
	mplsLabelCount         = 2000
	intCount               = 2000
	mplsV4Label            = 99991
	mplsV6Label            = 110993
	dutIntStartIPv4        = "169.254.0.1"
	otgIntStartIPv4        = "169.254.0.2"
	dutIntStartIPv6        = "2000:0:0:1::1"
	otgIntStartIPv6        = "2000:0:0:1::2"
	intStepV4              = "0.0.0.4"
	intStepV6              = "0:0:0:1::"
	staticRoutePrefix      = "10.99.1.0/24"
	staticRouteNextHop     = "194.0.2.2"
	outerSrcIPv4           = "100.64.0.1"
	outerDstIPv4           = "11.1.1.1"
	innerSrcIPv4           = "22.1.1.1"
	innerDstIPv4           = "21.1.1.1"
	innerSrcIPv6           = "2000:1::1"
	innerDstIPv6           = "3000:1::1"
	mcastDst               = "239.1.1.1"
	udpDstPort             = 6635
	flowSrcCount           = 10000
	dutIPv4Len             = 30
	dutIPv6Len             = 126
	dutMtu                 = 9202
	ratePPS                = 100
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
	coreIntf  = attrs.Attributes{Desc: "Core_Interface", IPv4: "194.0.2.1", IPv6: "2001:10:1:6::1", MTU: 9202, IPv4Len: 24, IPv6Len: 126}

	agg1 = &otgconfighelpers.Port{Name: "Port-Channel1", AggMAC: "02:00:01:01:01:07", MemberPorts: []string{"port1", "port2"}, LagID: 1, IsLag: true}
	agg2 = &otgconfighelpers.Port{Name: "Port-Channel2", AggMAC: "02:00:01:01:01:01", MemberPorts: []string{"port3", "port4"}, Interfaces: []*otgconfighelpers.InterfaceProperties{agg2interface}, LagID: 2, IsLag: true}

	agg2interface = &otgconfighelpers.InterfaceProperties{IPv4: "194.0.2.2", IPv6: "2001:10:1:6::2", IPv4Gateway: "194.0.2.1", IPv6Gateway: "2001:10:1:6::1", Name: "Port-Channel2", MAC: "02:00:01:01:01:02", IPv4Len: 29, IPv6Len: 126}

	// Custom IMIX settings for all flows.
	sizeWeightProfile = []otgconfighelpers.SizeWeightPair{
		{Size: 64, Weight: 20},
		{Size: 128, Weight: 20},
		{Size: 256, Weight: 20},
		{Size: 512, Weight: 10},
		{Size: 1500, Weight: 30},
	}

	flowResolveArp = &otgvalidationhelpers.OTGValidation{Interface: &otgvalidationhelpers.InterfaceParams{Names: []string{}}}
	// flowOuterIPv4 Decap IPv4 Interface IPv4 Payload traffic params Outer Header.
	flowOuterIPv4 = &otgconfighelpers.Flow{
		TxNames:           []string{agg2.Interfaces[0].Name + ".IPv4"},
		RxNames:           []string{},
		SizeWeightProfile: &sizeWeightProfile,
		Flowrate:          100,
		FlowName:          "MPLSOGUE-IPv4-Traffic",
		EthFlow:           &otgconfighelpers.EthFlowParams{SrcMAC: agg2.AggMAC},
		IPv4Flow:          &otgconfighelpers.IPv4FlowParams{IPv4Src: outerSrcIPv4, IPv4Dst: outerDstIPv4, IPv4SrcCount: flowSrcCount},
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
		IPv4Flow: &otgconfighelpers.IPv4FlowParams{IPv4Src: innerSrcIPv4, IPv4Dst: innerDstIPv4, IPv4SrcCount: flowSrcCount, RawPriority: 0, RawPriorityCount: 255},
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
		IPv4Flow:          &otgconfighelpers.IPv4FlowParams{IPv4Src: outerSrcIPv4, IPv4Dst: outerDstIPv4, IPv4SrcCount: flowSrcCount},
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
		IPv6Flow: &otgconfighelpers.IPv6FlowParams{IPv6Src: innerSrcIPv6, IPv6Dst: innerDstIPv6, IPv6SrcCount: flowSrcCount, TrafficClass: 0, TrafficClassCount: 255},
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
		IPv4Flow:          &otgconfighelpers.IPv4FlowParams{IPv4Src: outerSrcIPv4, IPv4Dst: outerDstIPv4, IPv4SrcCount: flowSrcCount},
		MPLSFlow:          &otgconfighelpers.MPLSFlowParams{MPLSLabel: mplsV4Label, MPLSExp: 7, MPLSLabelCount: mplsLabelCount},
		UDPFlow:           &otgconfighelpers.UDPFlowParams{UDPSrcPort: innerSrcPort, UDPDstPort: udpDstPort},
	}
	// flowOuterMcastValidation MPLSOGUE traffic IPv4 interface IPv4 Payload.
	flowOuterMcastValidation = &otgvalidationhelpers.OTGValidation{
		Interface: &otgvalidationhelpers.InterfaceParams{Names: []string{}, Ports: append(agg1.MemberPorts, agg2.MemberPorts...)},
		Flow:      &otgvalidationhelpers.FlowParams{Name: flowOuterMcast.FlowName, TolerancePct: 5},
	}
	// flowInnerMcast is the “inner” multicast payload (IPv4 + UDP to the same group).
	flowInnerMcast = &otgconfighelpers.Flow{
		IPv4Flow: &otgconfighelpers.IPv4FlowParams{IPv4Src: innerSrcIPv4, IPv4Dst: mcastDst, IPv4SrcCount: flowSrcCount, RawPriority: 0, RawPriorityCount: 255},
		TCPFlow:  &otgconfighelpers.TCPFlowParams{TCPSrcPort: innerSrcPort, TCPDstPort: 80, TCPSrcCount: flowSrcCount},
	}
	validationsIPv4     = []packetvalidationhelpers.ValidationType{packetvalidationhelpers.ValidateIPv4Header}
	validationsIPv6     = []packetvalidationhelpers.ValidationType{packetvalidationhelpers.ValidateIPv6Header}
	decapValidationIPv4 = &packetvalidationhelpers.PacketValidation{
		PortName:    "port1",
		CaptureName: "ipv4_decap",
		Validations: validationsIPv4,
		IPv4Layer:   &packetvalidationhelpers.IPv4Layer{DstIP: innerDstIPv4, Tos: 10, TTL: 64, SkipProtocolCheck: true},
	}
	decapValidationIPv6 = &packetvalidationhelpers.PacketValidation{
		PortName:    "port2",
		CaptureName: "ipv6_decap",
		Validations: validationsIPv6,
		IPv6Layer:   &packetvalidationhelpers.IPv6Layer{DstIP: innerDstIPv6, TrafficClass: 10, HopLimit: 64},
	}
	lagECMPValidation = &otgvalidationhelpers.OTGValidation{
		Interface: &otgvalidationhelpers.InterfaceParams{Ports: agg1.MemberPorts},
		Flow:      &otgvalidationhelpers.FlowParams{Name: flowOuterIPv4.FlowName},
	}
	lagECMPValidationV6 = &otgvalidationhelpers.OTGValidation{
		Interface: &otgvalidationhelpers.InterfaceParams{Ports: agg1.MemberPorts},
		Flow:      &otgvalidationhelpers.FlowParams{Name: flowOuterIPv6.FlowName},
	}
	lagECMPValidationMcast = &otgvalidationhelpers.OTGValidation{
		Interface: &otgvalidationhelpers.InterfaceParams{Ports: agg1.MemberPorts},
		Flow:      &otgvalidationhelpers.FlowParams{Name: flowOuterMcast.FlowName},
	}
)

type networkConfig struct {
	DutIPv4s []string
	OtgIPv4s []string
	OtgMACs  []string
	DutIPv6s []string
	OtgIPv6s []string
}

// generateNetConfig generates and returns a networkConfig object containing IPv4, IPv6, and MAC address allocations for both DUT and OTG interfaces.
func generateNetConfig(intCount int) (*networkConfig, error) {
	dutIPs, err := iputil.GenerateIPsWithStep(dutIntStartIPv4, intCount, intStepV4)
	if err != nil {
		return nil, fmt.Errorf("failed to generate DUT IPs: %w", err)
	}

	otgIPs, err := iputil.GenerateIPsWithStep(otgIntStartIPv4, intCount, intStepV4)
	if err != nil {
		return nil, fmt.Errorf("failed to generate OTG IPs: %w", err)
	}

	otgMACs := iputil.GenerateMACs("00:00:00:00:00:AA", intCount, "00:00:00:00:00:01")
	dutIPsV6, err := iputil.GenerateIPv6sWithStep(dutIntStartIPv6, intCount, intStepV6)
	if err != nil {
		return nil, fmt.Errorf("failed to generate DUT IPv6s: %w", err)
	}

	otgIPsV6, err := iputil.GenerateIPv6sWithStep(otgIntStartIPv6, intCount, intStepV6)
	if err != nil {
		return nil, fmt.Errorf("failed to generate OTG IPv6s: %w", err)
	}

	return &networkConfig{
		DutIPv4s: dutIPs,
		OtgIPv4s: otgIPs,
		OtgMACs:  otgMACs,
		DutIPv6s: dutIPsV6,
		OtgIPv6s: otgIPsV6,
	}, nil
}

// configureOTG sets up the Open Traffic Generator (OTG) test configuration.
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

// configureDUT Generate DUT Configuration.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice, netConfig *networkConfig, ocPFParams cfgplugins.OcPolicyForwardingParams) {
	t.Helper()
	aggID = netutil.NextAggregateInterface(t, dut)
	var interfaces []*attrs.Attributes
	for i := range intCount {
		interfaces = append(interfaces, &attrs.Attributes{
			Desc:         "Customer_connect",
			MTU:          dutMtu,
			IPv4:         netConfig.DutIPv4s[i],
			IPv4Len:      dutIPv4Len,
			IPv6:         netConfig.DutIPv6s[i],
			IPv6Len:      dutIPv6Len,
			Subinterface: uint32(i + 1),
		})
	}
	configureInterfaces(t, dut, custPorts, interfaces, aggID)
	aggID = netutil.NextAggregateInterface(t, dut)
	configureInterfaces(t, dut, corePorts, []*attrs.Attributes{&coreIntf}, aggID)
	configureStaticRoute(t, dut)
	_, ni, pf := cfgplugins.SetupPolicyForwardingInfraOC(ocPFParams.NetworkInstanceName)
	decapMPLSInGUE(t, dut, pf, ni, netConfig, ocPFParams)
}

// configureDUTAndOTG generates and applies DUT configuration, prepares OTG device/interface properties, and sets up validation flows.
func configureDUTAndOTG(t *testing.T) {
	t.Helper()
	t.Log("PF-1.20.1: Generate DUT Configuration")
	dut := ondatra.DUT(t, "dut")
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	netConfig, err := generateNetConfig(intCount)
	if err != nil {
		t.Fatalf("Error generating net config: %v", err)
	}
	mplsV4Labels := func() []int {
		r := make([]int, mplsLabelCount)
		for i := range r {
			r[i] = mplsV4Label + i
		}
		return r
	}()

	mplsV6Labels := func() []int {
		r := make([]int, mplsLabelCount)
		for i := range r {
			r[i] = mplsV6Label + i
		}
		return r
	}()

	for i := range intCount {
		agg1.Interfaces = append(agg1.Interfaces, &otgconfighelpers.InterfaceProperties{
			Name:        fmt.Sprintf("agg1port%d", i+1),
			IPv4:        netConfig.OtgIPv4s[i],
			IPv4Gateway: netConfig.DutIPv4s[i],
			Vlan:        uint32(i + 1),
			IPv4Len:     dutIPv4Len,
			IPv6:        netConfig.OtgIPv6s[i],
			IPv6Gateway: netConfig.DutIPv6s[i],
			IPv6Len:     dutIPv6Len,
			MAC:         netConfig.OtgMACs[i],
		})
	}

	// Get default parameters for OC Policy Forwarding
	ocPFParams := defaultOCPolicyForwardingParams()

	// Pass ocPFParams to ConfigureDut
	ocPFParams.DecapPolicy.DecapMPLSParams.MplsStaticLabels = mplsV4Labels
	ocPFParams.DecapPolicy.DecapMPLSParams.MplsStaticLabelsForIPv6 = mplsV6Labels
	// Pass ocPFParams to configureDut
	configureDUT(t, dut, netConfig, ocPFParams)
	// after agg1.Interfaces has been populated...
	for _, intf := range agg1.Interfaces {
		// tell the validator which ingress interfaces to watch
		flowOuterIPv4Validation.Interface.Names = append(flowOuterIPv4Validation.Interface.Names, intf.Name)
		// tell the flow which Rx device names to bind to
		flowOuterIPv4.RxNames = append(flowOuterIPv4.RxNames, intf.Name+".IPv4")

		flowOuterIPv6Validation.Interface.Names = append(flowOuterIPv6Validation.Interface.Names, intf.Name)
		flowOuterIPv6.RxNames = append(flowOuterIPv6.RxNames, intf.Name+".IPv6")
		// and for multicast:
		flowOuterMcastValidation.Interface.Names = append(flowOuterMcastValidation.Interface.Names, intf.Name)
		flowOuterMcast.RxNames = append(flowOuterMcast.RxNames, intf.Name+".IPv4")
	}
	configureOTG(t)
}

// defaultOCPolicyForwardingParams provides default parameters for the generator, matching the values in the provided JSON example.
func defaultOCPolicyForwardingParams() cfgplugins.OcPolicyForwardingParams {
	return cfgplugins.OcPolicyForwardingParams{
		NetworkInstanceName: "DEFAULT",
		InterfaceID:         "Agg1.10",
		AppliedPolicyName:   "customer1",
	}
}

// decapMPLSInGUE should also include the OC config , within these deviations there should be a switch statement is needed, Modified to accept pf, ni, and ocPFParams.
func decapMPLSInGUE(t *testing.T, dut *ondatra.DUTDevice, pf *oc.NetworkInstance_PolicyForwarding, ni *oc.NetworkInstance, netConfig *networkConfig, ocPFParams cfgplugins.OcPolicyForwardingParams) {
	t.Helper()
	ocPFParams.DecapPolicy.DecapMPLSParams.NextHops = netConfig.OtgIPv4s
	ocPFParams.DecapPolicy.DecapMPLSParams.NextHopsV6 = netConfig.OtgIPv6s
	ocPFParams.DecapPolicy.DecapMPLSParams.ScaleStaticLSP = true
	cfgplugins.MplsConfig(t, dut)
	cfgplugins.QosClassificationConfig(t, dut)
	cfgplugins.LabelRangeConfig(t, dut)
	cfgplugins.DecapGroupConfigGue(t, dut, pf, ocPFParams)
	cfgplugins.MPLSStaticLSPConfig(t, dut, ni, ocPFParams)
	if !deviations.PolicyForwardingOCUnsupported(dut) {
		pushPolicyForwardingConfig(t, dut, ni)
	}
}

// sendTraffic push the OTG config and start the protocols/traffic and get the flow/port metrics.
func sendTraffic(t *testing.T, ate *ondatra.ATEDevice, config *otg.OTG) {
	t.Helper()
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	if err := flowResolveArp.IsIPv4Interfaceresolved(t, ate); err != nil {
		t.Fatalf("Failed to resolve IPv4 interface for ATE: %v, error: %v", ate, err)
	}
	ate.OTG().StartTraffic(t)
	time.Sleep(sleepTime * time.Second)
	ate.OTG().StopTraffic(t)
	otgutils.LogFlowMetrics(t, config, top)
	otgutils.LogPortMetrics(t, config, top)
}

// sendTrafficCapture push the OTG config and start/stop the capture/traffic to validate the captured packets.
func sendTrafficCapture(t *testing.T, ate *ondatra.ATEDevice, config *otg.OTG) {
	t.Helper()
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	if err := flowResolveArp.IsIPv4Interfaceresolved(t, ate); err != nil {
		t.Fatalf("Failed to resolve IPv4 interface for ATE: %v, error: %v", ate, err)
	}
	cs := packetvalidationhelpers.StartCapture(t, ate)
	ate.OTG().StartTraffic(t)
	time.Sleep(sleepTime * time.Second)
	ate.OTG().StopTraffic(t)
	packetvalidationhelpers.StopCapture(t, ate, cs)
}

// createflow configure the traffic streams as per the readme.
func createflow(t *testing.T, top gosnappi.Config, paramsOuter *otgconfighelpers.Flow, paramsInner *otgconfighelpers.Flow, clearFlows bool) {
	t.Helper()
	if clearFlows {
		top.Flows().Clear()
	}
	paramsOuter.CreateFlow(top)
	paramsOuter.AddEthHeader()
	paramsOuter.AddIPv4Header()
	paramsOuter.AddUDPHeader()
	paramsOuter.AddMPLSHeader()
	if paramsInner.IPv4Flow != nil {
		paramsOuter.IPv4Flow = paramsInner.IPv4Flow
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

// updateFlow upadte the traffic streams as per the input.
func updateFlow(t *testing.T, paramsOuter *otgconfighelpers.Flow, paramsInner *otgconfighelpers.Flow, clearFlows bool, pps uint64, totalPackets uint32) {
	t.Helper()
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
		paramsOuter.IPv4Flow.IPv4Src = outerSrcIPv4
		paramsOuter.IPv4Flow.IPv4Dst = outerDstIPv4
	}
	createflow(t, top, paramsOuter, paramsInner, clearFlows)
}

// configureInterfaces configures a LAG (aggregate interface) and attaches DUT ports to it. It also applies LACP settings, enables aggregation, and sets hold-time for member interfaces.
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

	agg := &oc.Interface{Name: ygot.String(aggID)}
	configDUTInterface(t, agg, subinterfaces, dut)
	agg.GetOrCreateAggregation().LagType = oc.IfAggregate_AggregationType_LACP
	agg.Type = ieee8023adLag
	aggPath := d.Interface(aggID)
	fptest.LogQuery(t, aggID, aggPath.Config(), agg)
	gnmi.Replace(t, dut, aggPath.Config(), agg)

	for _, port := range dutAggPorts {
		holdTimeConfig := &oc.Interface_HoldTime{Up: ygot.Uint32(carrierDelayUp), Down: ygot.Uint32(carrierDelayDown)}
		intfPath := gnmi.OC().Interface(port.Name())
		gnmi.Update(t, dut, intfPath.HoldTime().Config(), holdTimeConfig)
	}
}

// configDUTInterface configures the aggregate interface and its subinterfaces based on the provided attributes.
func configDUTInterface(t *testing.T, i *oc.Interface, subinterfaces []*attrs.Attributes, dut *ondatra.DUTDevice) {
	t.Helper()
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
			configureInterfaceAddress(t, dut, s, a)
		} else {
			configureInterfaceAddress(t, dut, s1, a)
		}
	}
}

// configureInterfaceAddress assigns IPv4/IPv6 addresses to a given subinterface.
func configureInterfaceAddress(t *testing.T, dut *ondatra.DUTDevice, s *oc.Interface_Subinterface, a *attrs.Attributes) {
	t.Helper()
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
		s62 := s.GetOrCreateIpv6()
		if deviations.InterfaceEnabled(dut) {
			s62.Enabled = ygot.Bool(true)
		}
		s62.GetOrCreateAddress(a.IPv6Sec).PrefixLength = ygot.Uint8(a.IPv6Len)
	}
}

// configureStaticRoute installs a static IPv4 route on the DUT using GNMI batch configuration.
func configureStaticRoute(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	b := new(gnmi.SetBatch)
	sV4 := &cfgplugins.StaticRouteCfg{
		NetworkInstance: deviations.DefaultNetworkInstance(dut),
		Prefix:          staticRoutePrefix,
		NextHops:        map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{"0": oc.UnionString(staticRouteNextHop)},
	}
	if _, err := cfgplugins.NewStaticRouteCfg(b, sV4, dut); err != nil {
		t.Fatalf("Failed to configure IPv4 static route: %v", err)
	}
	b.Set(t, dut)
}

// pushPolicyForwardingConfig pushes the given policy forwarding configuration for the specified network instance to the DUT via gNMI Replace.
func pushPolicyForwardingConfig(t *testing.T, dut *ondatra.DUTDevice, ni *oc.NetworkInstance) {
	t.Helper()
	niPath := gnmi.OC().NetworkInstance(ni.GetName()).Config()
	gnmi.Replace(t, dut, niPath, ni)
}

func TestMPLSOGUEDecapScale(t *testing.T) {
	ate := ondatra.ATE(t, "ate")
	configureDUTAndOTG(t)
	tests := []struct {
		name                    string
		outerFlow               *otgconfighelpers.Flow
		innerFlow               *otgconfighelpers.Flow
		flowValidator           func(*testing.T, *ondatra.ATEDevice) error
		ecmpValidator           func(*testing.T, *ondatra.ATEDevice) error
		validatePayloadPreserve bool
		validationConfig        *packetvalidationhelpers.PacketValidation
	}{
		{
			name:          "IPv4 Traffic Scale",
			outerFlow:     flowOuterIPv4,
			innerFlow:     flowInnerIPv4,
			flowValidator: flowOuterIPv4Validation.ValidateLossOnFlows,
			ecmpValidator: lagECMPValidation.ValidateECMPonLAG,
		},
		{
			name:          "IPv6 Traffic Scale",
			outerFlow:     flowOuterIPv6,
			innerFlow:     flowInnerIPv6,
			flowValidator: flowOuterIPv6Validation.ValidateLossOnFlows,
			ecmpValidator: lagECMPValidationV6.ValidateECMPonLAG,
		},
		{
			name:          "Multicast Traffic Scale",
			outerFlow:     flowOuterMcast,
			innerFlow:     flowInnerMcast,
			flowValidator: flowOuterMcastValidation.ValidateLossOnFlows,
			ecmpValidator: lagECMPValidationMcast.ValidateECMPonLAG,
		},
		{
			name:                    "IPv4 Payload Preserve",
			outerFlow:               flowOuterIPv4,
			innerFlow:               flowInnerIPv4,
			flowValidator:           flowOuterIPv4Validation.ValidateLossOnFlows,
			validatePayloadPreserve: true,
			validationConfig:        decapValidationIPv4,
		},
		{
			name:                    "IPv6 Payload Preserve",
			outerFlow:               flowOuterIPv6,
			innerFlow:               flowInnerIPv6,
			flowValidator:           flowOuterIPv6Validation.ValidateLossOnFlows,
			validatePayloadPreserve: true,
			validationConfig:        decapValidationIPv6,
		},
	}

	packetvalidationhelpers.ClearCapture(t, top, ate)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Running test: %s", tc.name)

			if tc.validatePayloadPreserve {
				updateFlow(t, tc.outerFlow, tc.innerFlow, true, ratePPS, totalPkts)
				packetvalidationhelpers.ConfigurePacketCapture(t, top, tc.validationConfig)
				sendTrafficCapture(t, ate, ate.OTG())
			} else {
				createflow(t, top, tc.outerFlow, tc.innerFlow, true)
				sendTraffic(t, ate, ate.OTG())
			}

			if err := tc.flowValidator(t, ate); err != nil {
				t.Errorf("validateLossOnFlows(): got err: %q, want nil", err)
			}

			if tc.ecmpValidator != nil {
				if err := tc.ecmpValidator(t, ate); err != nil {
					t.Errorf("ecmpValidationFailed(): got err: %q, want nil", err)
				}
			}

			if tc.validatePayloadPreserve {
				if err := packetvalidationhelpers.CaptureAndValidatePackets(t, ate, tc.validationConfig); err != nil {
					t.Errorf("captureAndValidatePackets(): got err: %q", err)
				}
			}
		})
	}
}
