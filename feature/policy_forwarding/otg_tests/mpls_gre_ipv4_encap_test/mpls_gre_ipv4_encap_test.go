// Package mpls_gre_ipv4_encap_test tests mplsogre encap functionality.
package mpls_gre_ipv4_encap_test

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
	GREProtocol    = 47
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
	// MPLSOGRE Encap IPv4 interface IPv4 Payload
	FlowIPv4 = &otgconfighelpers.Flow{
		TxNames:           []string{agg1.Interfaces[0].Name + ".IPv4"},
		RxNames:           []string{agg2.Name + ".IPv4"},
		SizeWeightProfile: &sizeWeightProfile,
		Flowrate:          80,
		FlowName:          "GCI traffic IPv4 interface IPv4 Payload",
		EthFlow:           &otgconfighelpers.EthFlowParams{SrcMAC: agg1.AggMAC},
		VLANFlow:          &otgconfighelpers.VLANFlowParams{VLANId: 20},
		IPv4Flow:          &otgconfighelpers.IPv4FlowParams{IPv4Src: "12.1.1.1", IPv4Dst: "11.1.1.1", IPv4SrcCount: 100, RawPriority: 0, RawPriorityCount: 100},
	}

	FlowIPv4Validation = &otgvalidationhelpers.OTGValidation{
		Interface: &otgvalidationhelpers.InterfaceParams{Names: []string{agg2.Name, agg1.Interfaces[0].Name}, Ports: append(agg1.MemberPorts, agg2.MemberPorts...)},
		Flow:      &otgvalidationhelpers.FlowParams{Name: FlowIPv4.FlowName, TolerancePct: 0.5},
	}
	// FlowMultiCloudIPv4 IPv4 Flow configuration.
	FlowMultiCloudIPv4 = &otgconfighelpers.Flow{
		TxNames:           []string{agg1.Interfaces[3].Name + ".IPv4"},
		RxNames:           []string{agg2.Name + ".IPv4"},
		SizeWeightProfile: &sizeWeightProfile,
		FlowName:          "GCI traffic IPv4 interface IPv4 MultiCloud Payload",
		EthFlow:           &otgconfighelpers.EthFlowParams{SrcMAC: agg1.AggMAC},
		VLANFlow:          &otgconfighelpers.VLANFlowParams{VLANId: 23},
		IPv4Flow:          &otgconfighelpers.IPv4FlowParams{IPv4Src: "12.1.1.1", IPv4Dst: "11.1.1.1", IPv4DstCount: 10, DSCP: 0, DSCPCount: 63},
	}

	// FlowMultiCloudIPv4Validation Encap IPv4 flow validation.
	FlowMultiCloudIPv4Validation = &otgvalidationhelpers.OTGValidation{
		Interface: &otgvalidationhelpers.InterfaceParams{Names: []string{agg2.Name, agg1.Interfaces[3].Name}, Ports: append(agg1.MemberPorts, agg2.MemberPorts...)},
		Flow:      &otgvalidationhelpers.FlowParams{Name: FlowMultiCloudIPv4.FlowName, TolerancePct: 0.5},
	}
	// MPLSOGRE Encap IPv6 interface IPv6 Payload
	FlowIPv6 = &otgconfighelpers.Flow{
		TxNames:   []string{agg1.Interfaces[1].Name + ".IPv6"},
		RxNames:   []string{agg2.Name + ".IPv6"},
		FrameSize: 1500,
		Flowrate:  80,
		FlowName:  "GCI traffic IPv6 interface IPv6 Payload",
		EthFlow:   &otgconfighelpers.EthFlowParams{SrcMAC: agg1.AggMAC},
		VLANFlow:  &otgconfighelpers.VLANFlowParams{VLANId: 21},
		IPv6Flow:  &otgconfighelpers.IPv6FlowParams{IPv6Src: "3000:1::1", IPv6Dst: "2000:1::1", IPv6SrcCount: 100, TrafficClass: 0, TrafficClassCount: 100},
	}

	FlowIPv6Validation = &otgvalidationhelpers.OTGValidation{
		Interface: &otgvalidationhelpers.InterfaceParams{Names: []string{agg2.Name, agg1.Interfaces[1].Name}, Ports: append(agg1.MemberPorts, agg2.MemberPorts...)},
		Flow:      &otgvalidationhelpers.FlowParams{Name: FlowIPv6.FlowName, TolerancePct: 0.5},
	}
	// MPLSOGRE Encap IPv4+IPv6 Interface IPv4 Payload
	FlowDualIPv4 = &otgconfighelpers.Flow{
		TxNames:   []string{agg1.Interfaces[2].Name + ".IPv4"},
		RxNames:   []string{agg2.Name + ".IPv4"},
		FrameSize: 1500,
		FlowName:  "GCI traffic IPv4+IPv6 interface IPv4 Payload",
		EthFlow:   &otgconfighelpers.EthFlowParams{SrcMAC: agg1.AggMAC},
		VLANFlow:  &otgconfighelpers.VLANFlowParams{VLANId: 22},
		IPv4Flow:  &otgconfighelpers.IPv4FlowParams{IPv4Src: "12.1.1.1", IPv4Dst: "11.1.1.1", IPv4SrcCount: 100, RawPriority: 0, RawPriorityCount: 100},
	}

	FlowDualIPv4Validation = &otgvalidationhelpers.OTGValidation{
		Interface: &otgvalidationhelpers.InterfaceParams{Names: []string{agg2.Name, agg1.Interfaces[2].Name}, Ports: append(agg1.MemberPorts, agg2.MemberPorts...)},
		Flow:      &otgvalidationhelpers.FlowParams{Name: FlowDualIPv4.FlowName, TolerancePct: 0.5},
	}

	// MPLSOGRE Encap IPv4+IPv6 Interface IPv6 Payload
	FlowDualIPv6 = &otgconfighelpers.Flow{
		TxNames:   []string{agg1.Interfaces[2].Name + ".IPv6"},
		RxNames:   []string{agg2.Name + ".IPv6"},
		FrameSize: 1500,
		FlowName:  "GCI traffic IPv4+IPv6 interface IPv6 Payload",
		EthFlow:   &otgconfighelpers.EthFlowParams{SrcMAC: agg1.AggMAC},
		VLANFlow:  &otgconfighelpers.VLANFlowParams{VLANId: 22},
		IPv6Flow:  &otgconfighelpers.IPv6FlowParams{IPv6Src: "3000:1::1", IPv6Dst: "2000:1::1", IPv6SrcCount: 100, TrafficClass: 0, TrafficClassCount: 100},
	}

	FlowDualIPv6Validation = &otgvalidationhelpers.OTGValidation{
		Interface: &otgvalidationhelpers.InterfaceParams{Names: []string{agg2.Name, agg1.Interfaces[2].Name}, Ports: append(agg1.MemberPorts, agg2.MemberPorts...)},
		Flow:      &otgvalidationhelpers.FlowParams{Name: FlowDualIPv6.FlowName, TolerancePct: 0.5},
	}
	// MPLSOGRE Encap IPv4 Interface BGPv4 Payload
	FlowBGPv4 = &otgconfighelpers.Flow{
		TxNames:   []string{agg1.Interfaces[0].Name + ".IPv4"},
		RxNames:   []string{agg2.Name + ".IPv4"},
		FrameSize: 1500,
		FlowName:  "GCI traffic IPv4 interface, BGPv4 Payload",
		EthFlow:   &otgconfighelpers.EthFlowParams{SrcMAC: agg1.AggMAC},
		VLANFlow:  &otgconfighelpers.VLANFlowParams{VLANId: 20},
		IPv4Flow:  &otgconfighelpers.IPv4FlowParams{IPv4Src: "12.1.1.1", IPv4Dst: "11.1.1.1", IPv4SrcCount: 100, RawPriority: 0, RawPriorityCount: 100},
		TCPFlow:   &otgconfighelpers.TCPFlowParams{TCPSrcPort: 49152, TCPDstPort: 179},
	}

	FlowBGPv4Validation = &otgvalidationhelpers.OTGValidation{
		Interface: &otgvalidationhelpers.InterfaceParams{Names: []string{agg2.Name, agg1.Interfaces[0].Name}, Ports: append(agg1.MemberPorts, agg2.MemberPorts...)},
		Flow:      &otgvalidationhelpers.FlowParams{Name: FlowBGPv4.FlowName, TolerancePct: 0.5},
	}

	// MPLSOGRE Encap IPv4 Interface BFDv4 Payload
	FlowBFDv4 = &otgconfighelpers.Flow{
		TxNames:   []string{agg1.Interfaces[0].Name + ".IPv4"},
		RxNames:   []string{agg2.Name + ".IPv4"},
		FrameSize: 1500,
		FlowName:  "GCI traffic IPv4 interface, BFDv4 Payload",
		EthFlow:   &otgconfighelpers.EthFlowParams{SrcMAC: agg1.AggMAC},
		VLANFlow:  &otgconfighelpers.VLANFlowParams{VLANId: 20},
		IPv4Flow:  &otgconfighelpers.IPv4FlowParams{IPv4Src: "12.1.1.1", IPv4Dst: "11.1.1.1", IPv4SrcCount: 100, RawPriority: 0, RawPriorityCount: 100},
		UDPFlow:   &otgconfighelpers.UDPFlowParams{UDPSrcPort: 49152, UDPDstPort: 3784},
	}

	FlowBFDv4Validation = &otgvalidationhelpers.OTGValidation{
		Interface: &otgvalidationhelpers.InterfaceParams{Names: []string{agg2.Name, agg1.Interfaces[0].Name}, Ports: append(agg1.MemberPorts, agg2.MemberPorts...)},
		Flow:      &otgvalidationhelpers.FlowParams{Name: FlowBFDv4.FlowName, TolerancePct: 0.5},
	}
	Validations = []packetvalidationhelpers.ValidationType{
		packetvalidationhelpers.ValidateIPv4Header,
		packetvalidationhelpers.ValidateMPLSLayer,
		packetvalidationhelpers.ValidateInnerIPv4Header,
	}
	OuterGREIPLayerIPv4 = &packetvalidationhelpers.IPv4Layer{
		Protocol: GREProtocol,
		DstIP:    "10.99.1.1",
		Tos:      96,
		TTL:      64,
	}
	MPLSLayer = &packetvalidationhelpers.MPLSLayer{
		Label: 116383,
		Tc:    1,
	}
	InnerIPLayerIPv4 = &packetvalidationhelpers.IPv4Layer{
		DstIP: "11.1.1.1",
		Tos:   1,
		TTL:   63,
	}
	InnerIPLayerIPv6 = &packetvalidationhelpers.IPv6Layer{
		DstIP:        "2000:1::1",
		TrafficClass: 10,
		HopLimit:     63,
	}
	EncapPacketValidation = &packetvalidationhelpers.PacketValidation{
		PortName:         "port3",
		IPv4Layer:        OuterGREIPLayerIPv4,
		MPLSLayer:        MPLSLayer,
		Validations:      Validations,
		InnerIPLayerIPv4: InnerIPLayerIPv4,
		InnerIPLayerIPv6: InnerIPLayerIPv6,
		TCPLayer:         &packetvalidationhelpers.TCPLayer{SrcPort: 49152, DstPort: 179},
		UDPLayer:         &packetvalidationhelpers.UDPLayer{SrcPort: 49152, DstPort: 3784},
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

// PF-1.14.1: Generate DUT Configuration
// Modified to create and pass OC root, ni, pf
func ConfigureDut(t *testing.T, dut *ondatra.DUTDevice, ocPFParams cfgplugins.OcPolicyForwardingParams, ocNHGParams cfgplugins.StaticNextHopGroupParams) {
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
	EncapMPLSInGRE(t, dut, pf, ni, ocPFParams, ocNHGParams)

}

func TestSetup(t *testing.T) {
	t.Log("PF-1.14.1: Generate DUT Configuration")
	dut := ondatra.DUT(t, "dut")
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	// Get default parameters for OC Policy Forwarding
	ocPFParams := GetDefaultOcPolicyForwardingParams()
	ocNHGParams := GetDefaultStaticNextHopGroupParams()

	// Pass ocPFParams to ConfigureDut
	ConfigureDut(t, dut, ocPFParams, ocNHGParams)
	ConfigureOTG(t)
}

// GetDefaultStaticNextHopGroupParams provides default parameters for the generator.
// matching the values in the provided JSON example.
func GetDefaultStaticNextHopGroupParams() cfgplugins.StaticNextHopGroupParams {
	return cfgplugins.StaticNextHopGroupParams{

		StaticNHGName: "MPLS_in_GRE_Encap",
		NHIPAddr1:     "nh_ip_addr_1",
		NHIPAddr2:     "nh_ip_addr_2",
		// TODO: b/417988636 - Set the MplsLabel to the correct value.
	}
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
func EncapMPLSInGRE(t *testing.T, dut *ondatra.DUTDevice, pf *oc.NetworkInstance_PolicyForwarding, ni *oc.NetworkInstance, ocPFParams cfgplugins.OcPolicyForwardingParams, ocNHGParams cfgplugins.StaticNextHopGroupParams) {
	cfgplugins.MplsConfig(t, dut)
	cfgplugins.QosClassificationConfig(t, dut)
	cfgplugins.LabelRangeConfig(t, dut)
	cfgplugins.NextHopGroupConfig(t, dut, "v4", ni, ocNHGParams)
	cfgplugins.PolicyForwardingConfig(t, dut, "v4", pf, ocPFParams)
	cfgplugins.NextHopGroupConfig(t, dut, "v6", ni, ocNHGParams)
	cfgplugins.PolicyForwardingConfig(t, dut, "v6", pf, ocPFParams)
	cfgplugins.NextHopGroupConfig(t, dut, "dualstack", ni, ocNHGParams)
	cfgplugins.PolicyForwardingConfig(t, dut, "dualstack", pf, ocPFParams)
	cfgplugins.NextHopGroupConfig(t, dut, "multicloudv4", ni, ocNHGParams)
	cfgplugins.PolicyForwardingConfig(t, dut, "multicloudv4", pf, ocPFParams)
	if !deviations.PolicyForwardingOCUnsupported(dut) {
		PushPolicyForwardingConfig(t, dut, ni)
	}
}

// TestMPLSOGREEncapIPv4 tests MPLSOGRE Encap test case for IPv4 flow.
func TestMPLSOGREEncapIPv4(t *testing.T) {
	ate := ondatra.ATE(t, "ate")
	t.Log("PF-1.14.2: Verify PF MPLSoGRE encapsulate action for IPv4 traffic")
	createflow(t, top, FlowIPv4, true)
	sendTraffic(t, ate)

	if err := FlowIPv4Validation.ValidateLossOnFlows(t, ate); err != nil {
		t.Errorf("Validation on flows failed (): %q", err)
	}
	FlowIPv4.IPv4Flow.RawPriority = 1
	FlowIPv4.IPv4Flow.RawPriorityCount = 0
	FlowIPv4.PacketsToSend = 1000

	createflow(t, top, FlowIPv4, true)
	packetvalidationhelpers.ConfigurePacketCapture(t, top, EncapPacketValidation)
	sendTrafficCapture(t, ate)
	if err := FlowIPv4Validation.ValidateLossOnFlows(t, ate); err != nil {
		packetvalidationhelpers.ClearCapture(t, top, ate)
		t.Errorf("Validation on flows failed (): %q", err)
	}

	if err := packetvalidationhelpers.CaptureAndValidatePackets(t, top, ate, EncapPacketValidation); err != nil {
		packetvalidationhelpers.ClearCapture(t, top, ate)
		t.Errorf("Capture And ValidatePackets Failed (): %q", err)
	}
	packetvalidationhelpers.ClearCapture(t, top, ate)
}

func TestMPLSOGREEncapIPv4Ttl(t *testing.T) {
	t.Log("PF-1.14.7: Verify IPV4/IPV6 selective local traffic processing")
	ate := ondatra.ATE(t, "ate")

	FlowIPv4.IPv4Flow.TTL = 1
	createflow(t, top, FlowIPv4, true)
	sendTraffic(t, ate)
	if err := FlowIPv4Validation.ValidateLossOnFlows(t, ate); err != nil {
		t.Errorf("ValidateLossOnFlows(): %q", err)
	}
}

// TestMPLSOGREEncapIPv6 tests mplsogre encap functionality for IPv6 traffic.
func TestMPLSOGREEncapIPv6(t *testing.T) {
	t.Log("PF-1.14.3: Verify PF MPLSoGRE encapsulate action for IPv6 traffic")
	ate := ondatra.ATE(t, "ate")

	createflow(t, top, FlowIPv6, true)
	sendTraffic(t, ate)
	if err := FlowIPv6Validation.ValidateLossOnFlows(t, ate); err != nil {
		t.Errorf("ValidateLossOnFlows(): %q", err)
	}

	FlowIPv6.IPv6Flow.TrafficClass = 10
	FlowIPv6.IPv6Flow.TrafficClassCount = 0
	FlowIPv6.PacketsToSend = 1000
	createflow(t, top, FlowIPv6, true)
	packetvalidationhelpers.ConfigurePacketCapture(t, top, EncapPacketValidation)
	sendTrafficCapture(t, ate)
	if err := FlowIPv6Validation.ValidateLossOnFlows(t, ate); err != nil {
		packetvalidationhelpers.ClearCapture(t, top, ate)
		t.Errorf("ValidateLossOnFlows(): %q", err)
	}
	EncapPacketValidation.IPv4Layer.DstIP = "10.99.1.2"
	EncapPacketValidation.MPLSLayer.Label = 99999
	EncapPacketValidation.Validations = []packetvalidationhelpers.ValidationType{
		packetvalidationhelpers.ValidateIPv4Header,
		packetvalidationhelpers.ValidateMPLSLayer,
		packetvalidationhelpers.ValidateInnerIPv6Header,
	}

	if err := packetvalidationhelpers.CaptureAndValidatePackets(t, top, ate, EncapPacketValidation); err != nil {
		packetvalidationhelpers.ClearCapture(t, top, ate)
		t.Errorf("CaptureAndValidatePackets(): %q", err)
	}
	packetvalidationhelpers.ClearCapture(t, top, ate)
}
func TestMPLSOGREEncapIPv6Ttl(t *testing.T) {
	ate := ondatra.ATE(t, "ate")
	// OtgPreValidation(t, FlowIPv6Validation, "IPv6")
	FlowIPv6.IPv6Flow.HopLimit = 1
	createflow(t, top, FlowIPv6, true)
	sendTraffic(t, ate)
	if err := FlowIPv6Validation.ValidateLossOnFlows(t, ate); err != nil {
		t.Errorf("ValidateLossOnFlows(): %q", err)
	}
}

func TestMPLSOGREEncapDualStack(t *testing.T) {
	t.Log("PF-1.14.4: Verify PF MPLSoGRE encapsulate action for IPv6 traffic")
	ate := ondatra.ATE(t, "ate")
	createflow(t, top, FlowDualIPv4, true)
	createflow(t, top, FlowDualIPv6, false)
	sendTraffic(t, ate)
	if err := FlowDualIPv4Validation.ValidateLossOnFlows(t, ate); err != nil {
		t.Errorf("ValidateLossOnFlows(): %q", err)
	}
	if err := FlowDualIPv6Validation.ValidateLossOnFlows(t, ate); err != nil {
		t.Errorf("ValidateLossOnFlows(): %q", err)
	}

}
func TestMPLSOGREEncapBGPv4(t *testing.T) {
	ate := ondatra.ATE(t, "ate")

	createflow(t, top, FlowBGPv4, true)
	sendTraffic(t, ate)
	if err := FlowBGPv4Validation.ValidateLossOnFlows(t, ate); err != nil {
		t.Fatalf("ValidateLossOnFlows(): got err: %q", err)
	}

	FlowBGPv4.IPv4Flow.RawPriority = 1
	FlowBGPv4.IPv4Flow.RawPriorityCount = 0
	FlowBGPv4.PacketsToSend = 1000

	createflow(t, top, FlowBGPv4, true)

}

func TestMPLSOGREEncapBFDv4(t *testing.T) {

	ate := ondatra.ATE(t, "ate")

	createflow(t, top, FlowBFDv4, true)
	sendTraffic(t, ate)
	if err := FlowBFDv4Validation.ValidateLossOnFlows(t, ate); err != nil {
		t.Errorf("ValidateLossOnFlows(): got err: %q", err)
	}

	FlowBFDv4.IPv4Flow.RawPriority = 1
	FlowBFDv4.IPv4Flow.RawPriorityCount = 0
	FlowBFDv4.PacketsToSend = 1000

	createflow(t, top, FlowBFDv4, true)

}

// OTGPreValidation validates the OTG port status and interface resolution.
func OTGPreValidation(t *testing.T, params *otgvalidationhelpers.OTGValidation, interfaceType string) {
	ate := ondatra.ATE(t, "ate")
	if err := params.ValidatePortIsActive(t, ate); err != nil {
		t.Errorf("ValidatePortIsActive(): %q", err)
	}
	if interfaceType == "IPv4" {
		if err := params.IsIPv4Interfaceresolved(t, ate); err != nil {
			t.Errorf("IsIPv4Interfaceresolved(): %q", err)
		}
	}
	if interfaceType == "IPv6" {
		if err := params.IsIPv6Interfaceresolved(t, ate); err != nil {
			t.Errorf("IsIPv6Interfaceresolved(): %q", err)
		}
	}
}

func sendTraffic(t *testing.T, ate *ondatra.ATEDevice) {
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	ate.OTG().StartTraffic(t)
	time.Sleep(120 * time.Second)
	ate.OTG().StopTraffic(t)
}

func createflow(t *testing.T, top gosnappi.Config, params *otgconfighelpers.Flow, clearFlows bool) {
	if clearFlows {
		top.Flows().Clear()
	}
	params.CreateFlow(top)
	params.AddEthHeader()
	params.AddVLANHeader()
	if params.IPv4Flow != nil {
		params.AddIPv4Header()
	}
	if params.IPv6Flow != nil {
		params.AddIPv6Header()
	}
	if params.TCPFlow != nil {
		params.AddTCPHeader()
	}
	if params.UDPFlow != nil {
		params.AddUDPHeader()
	}
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

func sendTrafficCapture(t *testing.T, ate *ondatra.ATEDevice) {
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	cs := packetvalidationhelpers.StartCapture(t, ate)
	ate.OTG().StartTraffic(t)
	time.Sleep(60 * time.Second)
	ate.OTG().StopTraffic(t)
	time.Sleep(60 * time.Second)
	packetvalidationhelpers.StopCapture(t, ate, cs)
}

func PushPolicyForwardingConfig(t *testing.T, dut *ondatra.DUTDevice, ni *oc.NetworkInstance) {
	t.Helper()
	niPath := gnmi.OC().NetworkInstance(ni.GetName()).Config()
	gnmi.Replace(t, dut, niPath, ni)
}
