// Package mpls_gre_ipv4_decap_test tests mplsogre decap functionality.
package mpls_gre_ipv4_decap_test

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
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

// TestMain calls main function.
func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	ethernetCsmacd        = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	ieee8023adLag         = oc.IETFInterfaces_InterfaceType_ieee8023adLag
	mplsLabelCount        = 2000
	intCount              = 2000
	dutIntStartIP         = "169.254.0.1"
	otgIntStartIP         = "169.254.0.2"
	dutIntStartIpV6       = "2000:0:0:1::1"
	otgIntStartIpV6       = "2000:0:0:1::2"
	intStepV4             = "0.0.0.4"
	intStepV6             = "0:0:0:1::"
	mplsLabelStep         = 200
	mplsLabelStartforIpv4 = 16
	mplsLabelStartforIpv6 = 524280
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
		Interface: &otgvalidationhelpers.InterfaceParams{Names: []string{}},
	}
	// FlowOuterIPv4 Decap IPv4 Interface IPv4 Payload traffic params Outer Header.
	flowOuterIPv4 = &otgconfighelpers.Flow{
		TxNames:           []string{agg2.Interfaces[0].Name + ".IPv4"},
		RxNames:           []string{},
		SizeWeightProfile: &sizeWeightProfile,
		Flowrate:          45,
		FlowName:          "MPLSOGRE traffic IPv4 interface IPv4 Payload",
		EthFlow:           &otgconfighelpers.EthFlowParams{SrcMAC: agg2.AggMAC},
		IPv4Flow:          &otgconfighelpers.IPv4FlowParams{IPv4Src: "100.64.0.1", IPv4Dst: "11.1.1.1", IPv4SrcCount: 10000},
		MPLSFlow:          &otgconfighelpers.MPLSFlowParams{MPLSLabel: mplsLabelStartforIpv4, MPLSExp: 7, MPLSLabelCount: mplsLabelCount, MPLSLabelStep: mplsLabelStep},
		GREFlow:           &otgconfighelpers.GREFlowParams{Protocol: otgconfighelpers.IanaMPLSEthertype},
	}
	// FlowOuterIPv4Validation MPLSOGRE traffic IPv4 interface IPv4 Payload.
	flowOuterIPv4Validation = &otgvalidationhelpers.OTGValidation{
		Interface: &otgvalidationhelpers.InterfaceParams{Names: []string{}, Ports: append(agg1.MemberPorts, agg2.MemberPorts...)},
		Flow:      &otgvalidationhelpers.FlowParams{Name: flowOuterIPv4.FlowName, TolerancePct: 0.5},
	}
	// FlowInnerIPv4 Inner Header IPv4 Payload.
	flowInnerIPv4 = &otgconfighelpers.Flow{
		IPv4Flow: &otgconfighelpers.IPv4FlowParams{IPv4Src: "22.1.1.1", IPv4Dst: "21.1.1.1", IPv4SrcCount: 10000, RawPriority: 0, RawPriorityCount: 255},
		TCPFlow:  &otgconfighelpers.TCPFlowParams{TCPSrcPort: 49152, TCPDstPort: 80, TCPSrcCount: 10000},
	}
	// FlowOuterIPv6 Decap IPv6 Interface IPv6 Payload traffic params Outer Header.
	flowOuterIPv6 = &otgconfighelpers.Flow{
		TxNames:           []string{agg2.Name + ".IPv4"},
		RxNames:           []string{},
		SizeWeightProfile: &sizeWeightProfile,
		Flowrate:          45,
		FlowName:          "MPLSOGRE traffic IPv6 interface IPv6 Payload",
		EthFlow:           &otgconfighelpers.EthFlowParams{SrcMAC: agg2.AggMAC},
		IPv4Flow:          &otgconfighelpers.IPv4FlowParams{IPv4Src: "100.64.0.1", IPv4Dst: "11.1.1.1", IPv4SrcCount: 10000},
		MPLSFlow:          &otgconfighelpers.MPLSFlowParams{MPLSLabel: mplsLabelStartforIpv6, MPLSExp: 7, MPLSLabelCount: mplsLabelCount, MPLSLabelStep: mplsLabelStep},
		GREFlow:           &otgconfighelpers.GREFlowParams{Protocol: otgconfighelpers.IanaMPLSEthertype},
	}
	// FlowOuterIPv6Validation MPLSOGRE traffic IPv6 interface IPv6 Payload.
	flowOuterIPv6Validation = &otgvalidationhelpers.OTGValidation{
		Interface: &otgvalidationhelpers.InterfaceParams{Names: []string{}, Ports: append(agg1.MemberPorts, agg2.MemberPorts...)},
		Flow:      &otgvalidationhelpers.FlowParams{Name: flowOuterIPv6.FlowName, TolerancePct: 0.5},
	}
	// FlowInnerIPv6 Inner Header IPv6 Payload.
	flowInnerIPv6 = &otgconfighelpers.Flow{
		IPv6Flow: &otgconfighelpers.IPv6FlowParams{IPv6Src: "2000:1::1", IPv6Dst: "3000:1::1", IPv6SrcCount: 10000, TrafficClass: 0, TrafficClassCount: 255},
		TCPFlow:  &otgconfighelpers.TCPFlowParams{TCPSrcPort: 49152, TCPDstPort: 80, TCPSrcCount: 10000},
	}
	validationsIPv4 = []packetvalidationhelpers.ValidationType{
		packetvalidationhelpers.ValidateIPv4Header,
	}
	validationsIPv6 = []packetvalidationhelpers.ValidationType{
		packetvalidationhelpers.ValidateIPv6Header,
	}
	decapValidationIPv4 = &packetvalidationhelpers.PacketValidation{
		PortName:    "port1",
		CaptureName: "ipv4_decap",
		Validations: validationsIPv4,
		IPv4Layer:   &packetvalidationhelpers.IPv4Layer{DstIP: "21.1.1.1", Tos: 10, TTL: 64, Protocol: packetvalidationhelpers.TCP},
	}
	decapValidationIPv6 = &packetvalidationhelpers.PacketValidation{
		PortName:    "port2",
		CaptureName: "ipv6_decap",
		Validations: validationsIPv6,
		IPv6Layer:   &packetvalidationhelpers.IPv6Layer{DstIP: "3000:1::1", TrafficClass: 10, HopLimit: 64},
	}
	lagECMPValidation = &otgvalidationhelpers.OTGValidation{
		Interface: &otgvalidationhelpers.InterfaceParams{Ports: agg1.MemberPorts},
		Flow:      &otgvalidationhelpers.FlowParams{Name: flowOuterIPv4.FlowName},
	}
)

type networkConfig struct {
	dutIPs   []string
	otgIPs   []string
	otgMACs  []string
	dutIPsV6 []string
	otgIPsV6 []string
}

func generateNetConfig(intCount int) (*networkConfig, error) {
	dutIPs, err := iputil.GenerateIPsWithStep(dutIntStartIP, intCount, intStepV4)
	if err != nil {
		return nil, fmt.Errorf("failed to generate DUT IPs: %w", err)
	}

	otgIPs, err := iputil.GenerateIPsWithStep(otgIntStartIP, intCount, intStepV4)
	if err != nil {
		return nil, fmt.Errorf("failed to generate OTG IPs: %w", err)
	}

	otgMACs := iputil.GenerateMACs("00:00:00:00:00:AA", intCount, "00:00:00:00:00:01")

	dutIPsV6, err := iputil.GenerateIPv6sWithStep(dutIntStartIpV6, intCount, intStepV6)
	if err != nil {
		return nil, fmt.Errorf("failed to generate DUT IPv6s: %w", err)
	}

	otgIPsV6, err := iputil.GenerateIPv6sWithStep(otgIntStartIpV6, intCount, intStepV6)
	if err != nil {
		return nil, fmt.Errorf("failed to generate OTG IPv6s: %w", err)
	}

	return &networkConfig{
		dutIPs:   dutIPs,
		otgIPs:   otgIPs,
		otgMACs:  otgMACs,
		dutIPsV6: dutIPsV6,
		otgIPsV6: otgIPsV6,
	}, nil
}

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

// PF-1.13.1: Generate DUT Configuration
func configureDut(t *testing.T, dut *ondatra.DUTDevice, netConfig *networkConfig, ocPFParams cfgplugins.OcPolicyForwardingParams) {
	aggID = netutil.NextAggregateInterface(t, dut)

	var interfaces []*attrs.Attributes
	for i := range intCount {
		iface := &attrs.Attributes{
			Desc:         "Customer_connect",
			MTU:          1500,
			IPv4:         netConfig.dutIPs[i],
			IPv4Len:      30,
			IPv6:         netConfig.dutIPsV6[i],
			IPv6Len:      126,
			Subinterface: uint32(i + 1),
		}
		interfaces = append(interfaces, iface)
	}

	configureInterfaces(t, dut, custPorts, interfaces, aggID)
	aggID = netutil.NextAggregateInterface(t, dut)
	configureInterfaces(t, dut, corePorts, []*attrs.Attributes{&coreIntf}, aggID)
	configureStaticRoute(t, dut)
	_, ni, pf := cfgplugins.SetupPolicyForwardingInfraOC(ocPFParams.NetworkInstanceName)
	decapMPLSInGRE(t, dut, pf, ni, netConfig, ocPFParams)

}

func TestSetup(t *testing.T) {
	t.Log("PF-1.13.1: Generate DUT Configuration")
	dut := ondatra.DUT(t, "dut")
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	netConfig, err := generateNetConfig(intCount)
	if err != nil {
		t.Fatalf("Error generating net config: %v", err)
	}
	mplsStaticLabels := func() []int {
		var r []int
		for count, label := 0, mplsLabelStartforIpv4; count < mplsLabelCount; count++ {
			r = append(r, label)
			label += mplsLabelStep
		}
		return r
	}()

	mplsStaticLabelsForIpv6 := func() []int {
		var r []int
		for count, label := 0, mplsLabelStartforIpv6; count < mplsLabelCount; count++ {
			r = append(r, label)
			label += mplsLabelStep
		}
		return r
	}()

	var interfaces []*otgconfighelpers.InterfaceProperties

	for i := range intCount {
		iface := &otgconfighelpers.InterfaceProperties{
			Name:        fmt.Sprintf("agg1port%d", i+1),
			IPv4:        netConfig.otgIPs[i],
			IPv4Gateway: netConfig.dutIPs[i],
			Vlan:        uint32(i + 1),
			IPv4Len:     30,
			IPv6:        netConfig.otgIPsV6[i],
			IPv6Gateway: netConfig.dutIPsV6[i],
			IPv6Len:     126,
			MAC:         netConfig.otgMACs[i],
		}
		interfaces = append(interfaces, iface)
	}

	agg1.Interfaces = interfaces
	// Get default parameters for OC Policy Forwarding
	ocPFParams := fetchDefaultOcPolicyForwardingParams()

	flowOuterIPv4Validation.Interface.Names = append(flowOuterIPv4Validation.Interface.Names, agg1.Interfaces[0].Name)
	flowOuterIPv4.RxNames = append(flowOuterIPv4.RxNames, agg1.Interfaces[0].Name+".IPv4")
	flowOuterIPv6Validation.Interface.Names = append(flowOuterIPv6Validation.Interface.Names, agg1.Interfaces[0].Name)
	flowOuterIPv6.RxNames = append(flowOuterIPv6.RxNames, agg1.Interfaces[0].Name+".IPv4")

	for i, iface := range agg1.Interfaces {
		// Limiting it to 50 since checking ARP for 2000 interfaces takes long time
		if i >= 50 {
			break
		}
		flowResolveArp.Interface.Names = append(flowResolveArp.Interface.Names, iface.Name)
	}
	configureOTG(t)

	// Pass ocPFParams to ConfigureDut
	ocPFParams.DecapPolicy.MplsStaticLabels = mplsStaticLabels
	ocPFParams.DecapPolicy.MplsStaticLabelsForIpv6 = mplsStaticLabelsForIpv6
	configureDut(t, dut, netConfig, ocPFParams)

}

// fetchDefaultOcPolicyForwardingParams provides default parameters for the generator,
// matching the values in the provided JSON example.
func fetchDefaultOcPolicyForwardingParams() cfgplugins.OcPolicyForwardingParams {
	return cfgplugins.OcPolicyForwardingParams{
		NetworkInstanceName: "DEFAULT",
		InterfaceID:         "Agg1.10",
		AppliedPolicyName:   "customer1",
	}
}

// function should also include the OC config , within these deviations there should be a switch statement is needed
// Modified to accept pf, ni, and ocPFParams
func decapMPLSInGRE(t *testing.T, dut *ondatra.DUTDevice, pf *oc.NetworkInstance_PolicyForwarding, ni *oc.NetworkInstance, netConfig *networkConfig, ocPFParams cfgplugins.OcPolicyForwardingParams) {
	ocPFParams.DecapPolicy.NextHops = netConfig.otgIPs
	ocPFParams.DecapPolicy.NextHopsV6 = netConfig.otgIPsV6
	ocPFParams.DecapPolicy.ScaleStaticLSP = true
	cfgplugins.MplsConfig(t, dut)
	cfgplugins.QosClassificationConfig(t, dut)
	cfgplugins.LabelRangeConfig(t, dut)
	cfgplugins.DecapGroupConfigGre(t, dut, pf, ocPFParams)
	cfgplugins.MPLSStaticLSPConfig(t, dut, ni, ocPFParams)
	if !deviations.PolicyForwardingOCUnsupported(dut) {
		pushPolicyForwardingConfig(t, dut, ni)
	}
}

// PF-1.13.2: Verify IPV4/IPV6 traffic scale
func TestMPLSOGREDecapIPv4AndIPv6(t *testing.T) {
	ate := ondatra.ATE(t, "ate")
	t.Log("Verify IPV4/IPV6 traffic scale")
	createflow(t, top, flowOuterIPv4, flowInnerIPv4, true)
	sendTraffic(t, ate)
	var err error
	start := time.Now()
	for {
		err = flowOuterIPv4Validation.ValidateLossOnFlows(t, ate)
		if err == nil || time.Since(start) > 1*time.Minute {
			break
		}
		time.Sleep(20 * time.Second)
	}

	if err != nil {
		t.Errorf("ValidateLossOnFlows(): got err: %q, want nil", err)
	}

	if err = lagECMPValidation.ValidateECMPonLAG(t, ate); err != nil {
		t.Errorf("ECMPValidationFailed(): got err: %q, want nil", err)
	}
	createflow(t, top, flowOuterIPv6, flowInnerIPv6, true)
	sendTraffic(t, ate)
	start = time.Now()
	for {
		err := flowOuterIPv6Validation.ValidateLossOnFlows(t, ate)
		if err == nil || time.Since(start) > 1*time.Minute {
			break
		}
		time.Sleep(20 * time.Second)
	}
	if err != nil {
		t.Errorf("ValidateLossOnFlows(): got err: %q, want nil", err)
	}
}

func TestMPLSOGREDecapInnerPayloadPreserve(t *testing.T) {
	ate := ondatra.ATE(t, "ate")
	t.Log("Verify MPLSoGRE DSCP/TTL preserve operation")
	packetvalidationhelpers.ClearCapture(t, top, ate)
	updateFlow(t, flowOuterIPv4, flowInnerIPv4, true, 100, 1000)
	packetvalidationhelpers.ConfigurePacketCapture(t, top, decapValidationIPv4)
	sendTrafficCapture(t, ate)
	var err error
	start := time.Now()
	for {
		err = flowOuterIPv4Validation.ValidateLossOnFlows(t, ate)
		if err == nil || time.Since(start) > 1*time.Minute {
			break
		}
		time.Sleep(20 * time.Second)
	}

	if err != nil {
		t.Errorf("ValidateLossOnFlows(): got err: %q, want nil", err)
	}
	if err := packetvalidationhelpers.CaptureAndValidatePackets(t, ate, decapValidationIPv4); err != nil {
		t.Errorf("CaptureAndValidatePackets(): got err: %q", err)
	}
	updateFlow(t, flowOuterIPv6, flowInnerIPv6, true, 100, 1000)
	packetvalidationhelpers.ConfigurePacketCapture(t, top, decapValidationIPv6)
	sendTrafficCapture(t, ate)
	start = time.Now()
	for {
		err = flowOuterIPv6Validation.ValidateLossOnFlows(t, ate)
		if err == nil || time.Since(start) > 1*time.Minute {
			break
		}
		time.Sleep(20 * time.Second)
	}

	if err != nil {
		t.Errorf("ValidateLossOnFlows(): got err: %q, want nil", err)
	}

	if err := packetvalidationhelpers.CaptureAndValidatePackets(t, ate, decapValidationIPv6); err != nil {
		t.Errorf("CaptureAndValidatePackets(): got err: %q", err)
	}
}

func sendTraffic(t *testing.T, ate *ondatra.ATEDevice) {
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	flowResolveArp.IsIPv4Interfaceresolved(t, ate)
	ate.OTG().StartTraffic(t)
	time.Sleep(10 * time.Second)
	ate.OTG().StopTraffic(t)
}

func sendTrafficCapture(t *testing.T, ate *ondatra.ATEDevice) {
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	flowResolveArp.IsIPv4Interfaceresolved(t, ate)
	cs := packetvalidationhelpers.StartCapture(t, ate)
	ate.OTG().StartTraffic(t)
	time.Sleep(20 * time.Second)
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
	paramsOuter.AddGREHeader()
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
	paramsOuter.Flowrate = 0
	if paramsInner.IPv6Flow != nil {
		paramsInner.IPv6Flow.TrafficClassCount = 0
		paramsInner.IPv6Flow.TrafficClass = 10
	}
	if paramsInner.IPv4Flow != nil {
		paramsInner.IPv4Flow.RawPriorityCount = 0
		paramsInner.IPv4Flow.RawPriority = 10
		paramsOuter.IPv4Flow.IPv4Src = "100.64.0.1"
		paramsOuter.IPv4Flow.IPv4Dst = "11.1.1.1"
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

	agg := &oc.Interface{Name: ygot.String(aggID)}
	configDUTInterface(agg, subinterfaces, dut)
	agg.GetOrCreateAggregation().LagType = oc.IfAggregate_AggregationType_LACP
	agg.Type = ieee8023adLag
	aggPath := d.Interface(aggID)
	fptest.LogQuery(t, aggID, aggPath.Config(), agg)
	gnmi.Replace(t, dut, aggPath.Config(), agg)

	for _, port := range dutAggPorts {
		holdTimeConfig := &oc.Interface_HoldTime{
			Up:   ygot.Uint32(3000),
			Down: ygot.Uint32(150),
		}
		intfPath := gnmi.OC().Interface(port.Name())
		gnmi.Update(t, dut, intfPath.HoldTime().Config(), holdTimeConfig)
	}

	_, ok := gnmi.Watch(t, dut, gnmi.OC().Interface(aggID).OperStatus().State(), time.Minute, func(val *ygnmi.Value[oc.E_Interface_OperStatus]) bool {
		status, present := val.Val()
		return present && status == oc.Interface_OperStatus_UP
	}).Await(t)
	if !ok {
		t.Fatalf("LAG  %s is not ready. Expected %s got %s", aggID, oc.Interface_OperStatus_UP.String(), gnmi.Get(t, dut, gnmi.OC().Interface(aggID).OperStatus().State()).String())
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
		s62 := s.GetOrCreateIpv6()
		if deviations.InterfaceEnabled(dut) {
			s62.Enabled = ygot.Bool(true)
		}
		s62.GetOrCreateAddress(a.IPv6Sec).PrefixLength = ygot.Uint8(a.IPv6Len)
	}
}

func configureStaticRoute(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	b := &gnmi.SetBatch{}
	sV4 := &cfgplugins.StaticRouteCfg{
		NetworkInstance: deviations.DefaultNetworkInstance(dut),
		Prefix:          "10.99.1.0/24",
		NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
			"0": oc.UnionString("194.0.2.2"),
		},
	}
	if _, err := cfgplugins.NewStaticRouteCfg(b, sV4, dut); err != nil {
		t.Fatalf("Failed to configure IPv4 static route: %v", err)
	}
	b.Set(t, dut)
}

func pushPolicyForwardingConfig(t *testing.T, dut *ondatra.DUTDevice, ni *oc.NetworkInstance) {
	t.Helper()
	niPath := gnmi.OC().NetworkInstance(ni.GetName()).Config()
	gnmi.Replace(t, dut, niPath, ni)
}
