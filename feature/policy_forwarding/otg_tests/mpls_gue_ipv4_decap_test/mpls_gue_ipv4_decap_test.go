// Package mpls_gue_ipv4_decap_test tests mplsogue decap functionality.
package mpls_gue_ipv4_decap_test

import (
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	otgconfighelpers "github.com/openconfig/featureprofiles/internal/otg_helpers/otg_config_helpers"
	otgvalidationhelpers "github.com/openconfig/featureprofiles/internal/otg_helpers/otg_validation_helpers"
	packetvalidationhelpers "github.com/openconfig/featureprofiles/internal/otg_helpers/packetvalidationhelpers"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ygot/ygot"
)

// TestMain calls main function.
func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	ethernetCsmacd = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	ieee8023adLag  = oc.IETFInterfaces_InterfaceType_ieee8023adLag
)

var (
	top          = gosnappi.NewConfig()
	aggID        string
	custPorts    = []string{"port1", "port2"}
	corePorts    = []string{"port3", "port4"}
	custIntfIPv4 = attrs.Attributes{
		Desc:         "Customer_connect",
		MTU:          1500,
		IPv4:         "169.254.0.11",
		IPv4Len:      29,
		Subinterface: 20,
	}
	custIntfIPv6 = attrs.Attributes{
		Desc:         "Customer_connectv6",
		MTU:          1500,
		IPv6:         "2600:2d00:0:1:8000:10:0:ca31",
		IPv6Sec:      "2600:2d00:0:1:8000:10:0:ca33",
		IPv6Len:      125,
		Subinterface: 21,
	}

	custIntfdualStack = attrs.Attributes{
		Desc:         "Customer_connect_dualstack",
		MTU:          1500,
		IPv4:         "169.254.0.27",
		IPv4Len:      29,
		IPv6:         "2600:2d00:0:1:7000:10:0:ca31",
		IPv6Sec:      "2600:2d00:0:1:7000:10:0:ca33",
		IPv6Len:      125,
		Subinterface: 22,
	}
	custIntfIPv4MultiCloud = attrs.Attributes{
		Desc:         "Customer_connect_multicloud",
		MTU:          1500,
		IPv4:         "169.254.0.33",
		IPv4Len:      30,
		Subinterface: 23,
	}
	custIntfIPv4JumboMTU = attrs.Attributes{
		Desc:         "Customer_connect",
		MTU:          9066,
		IPv4:         "169.254.0.53",
		IPv4Len:      29,
		Subinterface: 26,
	}
	coreIntf = attrs.Attributes{
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
		Interfaces:  []*otgconfighelpers.InterfaceProperties{interface1, interface2, interface3, interface4, interface8},
		MemberPorts: []string{"port1", "port2"},
		LagID:       1,
		IsLag:       true,
	}
	agg2 = &otgconfighelpers.Port{
		Name:        "Port-Channel2",
		AggMAC:      "02:00:01:01:01:01",
		MemberPorts: []string{"port3", "port4"},
		Interfaces:  []*otgconfighelpers.InterfaceProperties{interface7},
		LagID:       2,
		IsLag:       true,
	}

	interface1 = &otgconfighelpers.InterfaceProperties{
		IPv4:        "169.254.0.12",
		IPv4Gateway: "169.254.0.11",
		Name:        "Port-Channel1.20",
		MAC:         "02:00:01:01:01:08",
		Vlan:        20,
		IPv4Len:     29,
	}
	interface2 = &otgconfighelpers.InterfaceProperties{
		IPv6:        "2600:2d00:0:1:8000:10:0:ca32",
		IPv6Gateway: "2600:2d00:0:1:8000:10:0:ca31",
		MAC:         "02:00:01:01:01:09",
		Name:        "Port-Channel1.21",
		Vlan:        21,
		IPv6Len:     125,
	}
	interface3 = &otgconfighelpers.InterfaceProperties{
		IPv4:        "169.254.0.26",
		IPv4Gateway: "169.254.0.27",
		IPv6:        "2600:2d00:0:1:7000:10:0:ca32",
		IPv6Gateway: "2600:2d00:0:1:7000:10:0:ca31",
		MAC:         "02:00:01:01:01:10",
		Name:        "Port-Channel1.22",
		Vlan:        22,
		IPv4Len:     29,
		IPv6Len:     125,
	}
	interface4 = &otgconfighelpers.InterfaceProperties{
		IPv4:        "169.254.0.34",
		IPv4Gateway: "169.254.0.33",
		Name:        "Port-Channel1.23",
		MAC:         "02:00:01:01:01:11",
		Vlan:        23,
		IPv4Len:     30,
	}
	interface8 = &otgconfighelpers.InterfaceProperties{
		IPv4:        "169.254.0.54",
		IPv4Gateway: "169.254.0.53",
		Name:        "Port-Channel1.26",
		MAC:         "02:00:01:01:01:13",
		Vlan:        26,
		IPv4Len:     29,
	}
	interface7 = &otgconfighelpers.InterfaceProperties{
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
		{Size: 1500, Weight: 28},
		{Size: 9000, Weight: 2},
	}

	flowResolveArp = &otgvalidationhelpers.OTGValidation{
		Interface: &otgvalidationhelpers.InterfaceParams{Names: []string{agg2.Name}},
	}
	nextHopResolutionIPv4 = &otgvalidationhelpers.OTGValidation{
		Interface: &otgvalidationhelpers.InterfaceParams{Names: []string{agg1.Interfaces[0].Name, agg1.Interfaces[2].Name, agg1.Interfaces[3].Name, agg1.Interfaces[4].Name}},
	}
	nextHopResolutionIPv6 = &otgvalidationhelpers.OTGValidation{
		Interface: &otgvalidationhelpers.InterfaceParams{Names: []string{agg1.Interfaces[1].Name, agg1.Interfaces[2].Name}},
	}
	// FlowOuterIPv4 Decap IPv4 Interface IPv4 Payload traffic params Outer Header.
	FlowOuterIPv4 = &otgconfighelpers.Flow{
		TxNames:           []string{agg2.Name + ".IPv4"},
		RxNames:           []string{agg1.Interfaces[0].Name + ".IPv4"},
		SizeWeightProfile: &sizeWeightProfile,
		Flowrate:          45,
		FlowName:          "MPLSOGUE traffic IPv4 interface IPv4 Payload",
		EthFlow:           &otgconfighelpers.EthFlowParams{SrcMAC: agg2.AggMAC},
		IPv4Flow:          &otgconfighelpers.IPv4FlowParams{IPv4Src: "100.64.0.1", IPv4Dst: "11.1.1.1", IPv4SrcCount: 10000},
		MPLSFlow:          &otgconfighelpers.MPLSFlowParams{MPLSLabel: 99991, MPLSExp: 7},
		UDPFlow:           &otgconfighelpers.UDPFlowParams{UDPSrcPort: 49152, UDPDstPort: 6635},
	}
	// FlowOuterIPv4Validation MPLSOGUE traffic IPv4 interface IPv4 Payload.
	FlowOuterIPv4Validation = &otgvalidationhelpers.OTGValidation{
		Interface: &otgvalidationhelpers.InterfaceParams{Names: []string{agg1.Interfaces[0].Name}, Ports: append(agg1.MemberPorts, agg2.MemberPorts...)},
		Flow:      &otgvalidationhelpers.FlowParams{Name: FlowOuterIPv4.FlowName, TolerancePct: 0.5},
	}
	// FlowInnerIPv4 Inner Header IPv4 Payload.
	FlowInnerIPv4 = &otgconfighelpers.Flow{
		IPv4Flow: &otgconfighelpers.IPv4FlowParams{IPv4Src: "22.1.1.1", IPv4Dst: "21.1.1.1", IPv4SrcCount: 10000, RawPriority: 0, RawPriorityCount: 255},
		TCPFlow:  &otgconfighelpers.TCPFlowParams{TCPSrcPort: 49152, TCPDstPort: 80, TCPSrcCount: 10000},
	}
	// FlowOuterIPv6 Decap IPv6 Interface IPv6 Payload traffic params Outer Header.
	FlowOuterIPv6 = &otgconfighelpers.Flow{
		TxNames:           []string{agg2.Name + ".IPv4"},
		RxNames:           []string{agg1.Interfaces[1].Name + ".IPv6"},
		SizeWeightProfile: &sizeWeightProfile,
		Flowrate:          45,
		FlowName:          "MPLSOGUE traffic IPv6 interface IPv6 Payload",
		EthFlow:           &otgconfighelpers.EthFlowParams{SrcMAC: agg2.AggMAC},
		IPv4Flow:          &otgconfighelpers.IPv4FlowParams{IPv4Src: "100.64.0.1", IPv4Dst: "11.1.1.1", IPv4SrcCount: 10000},
		MPLSFlow:          &otgconfighelpers.MPLSFlowParams{MPLSLabel: 99992, MPLSExp: 7},
		UDPFlow:           &otgconfighelpers.UDPFlowParams{UDPSrcPort: 49152, UDPDstPort: 6635},
	}
	// FlowOuterIPv6Validation MPLSOGUE traffic IPv6 interface IPv6 Payload.
	FlowOuterIPv6Validation = &otgvalidationhelpers.OTGValidation{
		Interface: &otgvalidationhelpers.InterfaceParams{Names: []string{agg1.Interfaces[1].Name}, Ports: append(agg1.MemberPorts, agg2.MemberPorts...)},
		Flow:      &otgvalidationhelpers.FlowParams{Name: FlowOuterIPv6.FlowName, TolerancePct: 0.5},
	}
	// FlowInnerIPv6 Inner Header IPv6 Payload.
	FlowInnerIPv6 = &otgconfighelpers.Flow{
		IPv6Flow: &otgconfighelpers.IPv6FlowParams{IPv6Src: "2000:1::1", IPv6Dst: "3000:1::1", IPv6SrcCount: 10000, TrafficClass: 0, TrafficClassCount: 255},
		TCPFlow:  &otgconfighelpers.TCPFlowParams{TCPSrcPort: 49152, TCPDstPort: 80, TCPSrcCount: 10000},
	}
	validationsIPv4 = []packetvalidationhelpers.ValidationType{
		packetvalidationhelpers.ValidateIPv4Header,
		packetvalidationhelpers.ValidateTCPHeader,
	}
	decapValidationIPv4 = &packetvalidationhelpers.PacketValidation{
		PortName:    "port1",
		CaptureName: "ipv4_decap",
		Validations: validationsIPv4,
		IPv4Layer:   &packetvalidationhelpers.IPv4Layer{DstIP: "21.1.1.1", Tos: 10, TTL: 64, Protocol: packetvalidationhelpers.TCP},
		TCPLayer:    &packetvalidationhelpers.TCPLayer{SrcPort: 49152, DstPort: 80},
	}
)

func ConfigureOTG(t *testing.T) {
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
}

// PF-1.19.1: Generate DUT Configuration
func ConfigureDut(t *testing.T, dut *ondatra.DUTDevice, ocPFParams cfgplugins.OcPolicyForwardingParams) {
	aggID = netutil.NextAggregateInterface(t, dut)
	configureInterfaces(t, dut, custPorts, []*attrs.Attributes{&custIntfIPv4, &custIntfIPv6, &custIntfdualStack, &custIntfIPv4MultiCloud, &custIntfIPv4JumboMTU}, aggID)
	configureInterfaceProperties(t, dut, aggID, &custIntfIPv4, ocPFParams)
	configureInterfaceProperties(t, dut, aggID, &custIntfIPv6, ocPFParams)
	configureInterfaceProperties(t, dut, aggID, &custIntfdualStack, ocPFParams)
	configureInterfaceProperties(t, dut, aggID, &custIntfIPv4MultiCloud, ocPFParams)
	configureInterfaceProperties(t, dut, aggID, &custIntfIPv4JumboMTU, ocPFParams)
	aggID = netutil.NextAggregateInterface(t, dut)
	configureInterfaces(t, dut, corePorts, []*attrs.Attributes{&coreIntf}, aggID)
	configureStaticRoute(t, dut)
	_, ni, pf := cfgplugins.SetupPolicyForwardingInfraOC(ocPFParams.NetworkInstanceName)
	DecapMPLSInGUE(t, dut, pf, ni, ocPFParams)
}

func TestSetup(t *testing.T) {
	t.Log("PF-1.19.1: Generate DUT Configuration")
	dut := ondatra.DUT(t, "dut")
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	// Get default parameters for OC Policy Forwarding
	ocPFParams := GetDefaultOcPolicyForwardingParams()

	// Pass ocPFParams to ConfigureDut
	ConfigureDut(t, dut, ocPFParams)
	ConfigureOTG(t)
}

// GetDefaultOcPolicyForwardingParams provides default parameters for the generator,
// matching the values in the provided JSON example.
func GetDefaultOcPolicyForwardingParams() cfgplugins.OcPolicyForwardingParams {
	return cfgplugins.OcPolicyForwardingParams{
		NetworkInstanceName: "DEFAULT",
		InterfaceID:         "Agg1.10",
		AppliedPolicyName:   "customer1",
	}
}

func configureInterfaceProperties(t *testing.T, dut *ondatra.DUTDevice, aggID string, a *attrs.Attributes, ocPFParams cfgplugins.OcPolicyForwardingParams) {
	_, _, pf := cfgplugins.SetupPolicyForwardingInfraOC(ocPFParams.NetworkInstanceName)

	if a.IPv4 != "" {
		cfgplugins.InterfacelocalProxyConfig(t, dut, a, aggID)
	}
	cfgplugins.InterfaceQosClassificationConfig(t, dut, a, aggID)
	cfgplugins.InterfacePolicyForwardingConfig(t, dut, a, aggID, pf, ocPFParams)
}

// function should also include the OC config , within these deviations there should be a switch statement is needed
// Modified to accept pf, ni, and ocPFParams
func DecapMPLSInGUE(t *testing.T, dut *ondatra.DUTDevice, pf *oc.NetworkInstance_PolicyForwarding, ni *oc.NetworkInstance, ocPFParams cfgplugins.OcPolicyForwardingParams) {
	cfgplugins.MplsConfig(t, dut)
	cfgplugins.QosClassificationConfig(t, dut)
	cfgplugins.LabelRangeConfig(t, dut)
	cfgplugins.DecapGroupConfigGue(t, dut, pf, ocPFParams)
	cfgplugins.MPLSStaticLSPConfig(t, dut, ni, ocPFParams)
	if !deviations.PolicyForwardingOCUnsupported(dut) {
		PushPolicyForwardingConfig(t, dut, ni)
	}
}

// PF-1.19.2: Verify PF MPLSoGUE Decap action for IPv4 and IPv6 traffic.
func TestMPLSOGUEDecapIPv4AndIPv6(t *testing.T) {
	ate := ondatra.ATE(t, "ate")
	t.Log("PF-1.19.2: Verify MPLSoGUE decapsulate action for IPv4 and IPv6 payload")
	createflow(t, top, FlowOuterIPv4, FlowInnerIPv4, true)
	createflow(t, top, FlowOuterIPv6, FlowInnerIPv6, false)
	sendTraffic(t, ate)
	if err := FlowOuterIPv4Validation.ValidateLossOnFlows(t, ate); err != nil {
		t.Errorf("ValidateLossOnFlows(): got err: %q, want nil", err)
	}
	if err := FlowOuterIPv6Validation.ValidateLossOnFlows(t, ate); err != nil {
		t.Errorf("ValidateLossOnFlows(): got err: %q, want nil", err)
	}
}

// PF-1.19.5: Verify MPLSoGUE DSCP/TTL preserve operation.
func TestMPLSOGUEDecapInnerPayloadPreserve(t *testing.T) {
	ate := ondatra.ATE(t, "ate")
	t.Log("PF-1.19.5: Verify MPLSoGUE DSCP/TTL preserve operation")
	packetvalidationhelpers.ClearCapture(t, top, ate)
	updateFlow(t, FlowOuterIPv4, FlowInnerIPv4, true, 100, 1000)
	packetvalidationhelpers.ConfigurePacketCapture(t, top, decapValidationIPv4)
	sendTrafficCapture(t, ate)
	if err := FlowOuterIPv4Validation.ValidateLossOnFlows(t, ate); err != nil {
		t.Errorf("ValidateLossOnFlows(): got err: %q, want nil", err)
	}
	if err := packetvalidationhelpers.CaptureAndValidatePackets(t, ate, decapValidationIPv4); err != nil {
		t.Errorf("CaptureAndValidatePackets(): got err: %q", err)
	}
}

// PF-1.19.6: Verify IPV4/IPV6 nexthop resolution of decap traffic
func TestMPLSOGUEDecapNextHopResolution(t *testing.T) {
	ate := ondatra.ATE(t, "ate")
	totalPackets := uint32(100000)
	pps := uint64(1000)
	t.Log("PF-1.19.6: Verify IPV4/IPV6 nexthop resolution of decap traffic")
	updateFlow(t, FlowOuterIPv4, FlowInnerIPv4, true, pps, totalPackets)
	updateFlow(t, FlowOuterIPv6, FlowInnerIPv6, false, pps, totalPackets)
	// Make sure the next hop resolution is done for IPv4 and IPv6 Interfaces facing towards customer Interfaces in OTG.
	nextHopResolutionIPv4.IsIPv4Interfaceresolved(t, ate)
	nextHopResolutionIPv6.IsIPv6Interfaceresolved(t, ate)
	sendTraffic(t, ate)
	if err := FlowOuterIPv4Validation.ValidateLossOnFlows(t, ate); err != nil {
		t.Errorf("ValidateLossOnFlows(): got err: %q, want nil", err)
	}
	if err := FlowOuterIPv6Validation.ValidateLossOnFlows(t, ate); err != nil {
		t.Errorf("ValidateLossOnFlows(): got err: %q, want nil", err)
	}
}

func sendTraffic(t *testing.T, ate *ondatra.ATEDevice) {
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	flowResolveArp.IsIPv4Interfaceresolved(t, ate)
	ate.OTG().StartTraffic(t)
	time.Sleep(120 * time.Second)
	ate.OTG().StopTraffic(t)
}

func sendTrafficCapture(t *testing.T, ate *ondatra.ATEDevice) {
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	flowResolveArp.IsIPv4Interfaceresolved(t, ate)
	cs := packetvalidationhelpers.StartCapture(t, ate)
	ate.OTG().StartTraffic(t)
	time.Sleep(60 * time.Second)
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
		*paramsOuter.UDPFlow = *paramsInner.UDPFlow
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
		if paramsInner.TCPFlow != nil {
			paramsInner.TCPFlow.TCPSrcCount = 0
			paramsInner.TCPFlow.TCPSrcPort = 49152
		}
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
			Up:   ygot.Uint32(3000),
			Down: ygot.Uint32(150),
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

func PushPolicyForwardingConfig(t *testing.T, dut *ondatra.DUTDevice, ni *oc.NetworkInstance) {
	t.Helper()
	niPath := gnmi.OC().NetworkInstance(ni.GetName()).Config()
	gnmi.Replace(t, dut, niPath, ni)
}
