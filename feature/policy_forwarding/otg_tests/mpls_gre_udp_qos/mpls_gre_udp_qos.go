// Package mpls_gre_udp_qos_test tests mplsogre encap functionality.
package mpls_gre_udp_qos_test

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
	// GREProtocol is the protocol number for GRE.
	GREProtocol = 47
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
		Interfaces:  []*otgconfighelpers.InterfaceProperties{interface1},
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
	// FlowIPv4 consists of MPLSOGRE Encap IPv4 interface IPv4 Payload.
	FlowIPv4 = &otgconfighelpers.Flow{
		TxNames:           []string{agg1.Interfaces[0].Name + ".IPv4"},
		RxNames:           []string{agg2.Name + ".IPv4"},
		SizeWeightProfile: &sizeWeightProfile,
		Flowrate:          80,
		FlowName:          "traffic IPv4 interface IPv4 Payload",
		EthFlow:           &otgconfighelpers.EthFlowParams{SrcMAC: agg1.AggMAC},
		VLANFlow:          &otgconfighelpers.VLANFlowParams{VLANId: 20},
		IPv4Flow:          &otgconfighelpers.IPv4FlowParams{IPv4Src: "12.1.1.1", IPv4Dst: "11.1.1.1", IPv4SrcCount: 100, RawPriority: 0, RawPriorityCount: 100},
	}
	// flowIPv4Validation consists of flow validation params.
	flowIPv4Validation = &otgvalidationhelpers.OTGValidation{
		Interface: &otgvalidationhelpers.InterfaceParams{Names: []string{agg2.Name, agg1.Interfaces[0].Name}, Ports: append(agg1.MemberPorts, agg2.MemberPorts...)},
		Flow:      &otgvalidationhelpers.FlowParams{Name: FlowIPv4.FlowName, TolerancePct: 0.5},
	}
	validations = []packetvalidationhelpers.ValidationType{
		packetvalidationhelpers.ValidateIPv4Header,
		packetvalidationhelpers.ValidateMPLSLayer,
		packetvalidationhelpers.ValidateInnerIPv4Header,
	}
	outerGREIPLayerIPv4 = &packetvalidationhelpers.IPv4Layer{
		Protocol: GREProtocol,
		DstIP:    "10.99.1.1",
		Tos:      96,
		TTL:      64,
	}
	mplsLayer = &packetvalidationhelpers.MPLSLayer{
		Label: 116383,
		Tc:    1,
	}
	innerIPLayerIPv4 = &packetvalidationhelpers.IPv4Layer{
		DstIP: "11.1.1.1",
		Tos:   1,
		TTL:   63,
	}
	innerIPLayerIPv6 = &packetvalidationhelpers.IPv6Layer{
		DstIP:        "2000:1::1",
		TrafficClass: 10,
		HopLimit:     63,
	}
	encapPacketValidation = &packetvalidationhelpers.PacketValidation{
		PortName:         "port3",
		IPv4Layer:        outerGREIPLayerIPv4,
		MPLSLayer:        mplsLayer,
		Validations:      validations,
		InnerIPLayerIPv4: innerIPLayerIPv4,
		InnerIPLayerIPv6: innerIPLayerIPv6,
		TCPLayer:         &packetvalidationhelpers.TCPLayer{SrcPort: 49152, DstPort: 179},
		UDPLayer:         &packetvalidationhelpers.UDPLayer{SrcPort: 49152, DstPort: 3784},
	}
)

func configureOTG(t *testing.T) {
	t.Helper()
	top.Captures().Clear()
	ate := ondatra.ATE(t, "ate")

	// Create a slice of aggPortData for easier iteration.
	aggs := []*otgconfighelpers.Port{agg1, agg2}

	// Configure OTG Interfaces.
	for _, agg := range aggs {
		otgconfighelpers.ConfigureNetworkInterface(t, top, ate, agg)
	}
	ate.OTG().PushConfig(t, top)
}

// ConfigureDut configures DUT for PF-1.18.1.
func ConfigureDut(t *testing.T, dut *ondatra.DUTDevice, ocPFParams cfgplugins.OcPolicyForwardingParams, ocNHGParams cfgplugins.StaticNextHopGroupParams) {
	aggID = netutil.NextAggregateInterface(t, dut)
	configureInterfaces(t, dut, custPorts, []*attrs.Attributes{&custIntfIPv4}, aggID)
	configureInterfaceProperties(t, dut, aggID, &custIntfIPv4, ocPFParams)
	aggID = netutil.NextAggregateInterface(t, dut)
	configureInterfaces(t, dut, corePorts, []*attrs.Attributes{&coreIntf}, aggID)
	configureStaticRoute(t, dut)
	_, ni, pf := cfgplugins.SetupPolicyForwardingInfraOC(ocPFParams.NetworkInstanceName)
	encapMPLSInGRE(t, dut, pf, ni, ocPFParams, ocNHGParams)
	decapMPLSInGRE(t, dut, pf, ni, ocPFParams)
	if !deviations.PolicyForwardingOCUnsupported(dut) {
		pushPolicyForwardingConfig(t, dut, ni)
	}

}

// TestSetup configures the DUT and ATE for the test.
func TestSetup(t *testing.T) {
	t.Log("PF-1.18.1: Generate DUT Configuration")
	dut := ondatra.DUT(t, "dut")
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	// Get default parameters for OC Policy Forwarding
	ocPFParams := GetDefaultOcPolicyForwardingParams()
	ocNHGParams := GetDefaultStaticNextHopGroupParams()

	// Pass ocPFParams to ConfigureDut
	ConfigureDut(t, dut, ocPFParams, ocNHGParams)
	configureOTG(t)
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
func encapMPLSInGRE(t *testing.T, dut *ondatra.DUTDevice, pf *oc.NetworkInstance_PolicyForwarding, ni *oc.NetworkInstance, ocPFParams cfgplugins.OcPolicyForwardingParams, ocNHGParams cfgplugins.StaticNextHopGroupParams) {
	cfgplugins.MplsConfig(t, dut)
	cfgplugins.QosClassificationConfig(t, dut)
	cfgplugins.LabelRangeConfig(t, dut)
	cfgplugins.NextHopGroupConfig(t, dut, "v4", ni, ocNHGParams)
	cfgplugins.PolicyForwardingConfig(t, dut, "v4", pf, ocPFParams)
	cfgplugins.NextHopGroupConfig(t, dut, "multicloudv4", ni, ocNHGParams)
	cfgplugins.PolicyForwardingConfig(t, dut, "multicloudv4", pf, ocPFParams)
}

// TestMPLSOGREEncapIPv4 verifies PF-1.18.3: Verify DSCP marking of encapsulated and decapsulated traffic.
func TestMPLSOGREEncapIPv4(t *testing.T) {
	t.Logf("PF-1.18.3: Verify DSCP marking of encapsulated and decapsulated traffic")
	ate := ondatra.ATE(t, "ate")

	createflow(t, top, FlowIPv4, true)
	sendTraffic(t, ate, "IPv4")

	if err := flowIPv4Validation.ValidateLossOnFlows(t, ate); err != nil {
		t.Errorf("Validation on flows failed (): %q", err)
	}
	FlowIPv4.IPv4Flow.RawPriority = 1
	FlowIPv4.IPv4Flow.RawPriorityCount = 0
	FlowIPv4.PacketsToSend = 1000

	createflow(t, top, FlowIPv4, true)
	packetvalidationhelpers.ConfigurePacketCapture(t, top, encapPacketValidation)
	sendTrafficCapture(t, ate)
	if err := flowIPv4Validation.ValidateLossOnFlows(t, ate); err != nil {
		packetvalidationhelpers.ClearCapture(t, top, ate)
		t.Errorf("Validation on flows failed (): %q", err)
	}

	if err := packetvalidationhelpers.CaptureAndValidatePackets(t, ate, encapPacketValidation); err != nil {
		packetvalidationhelpers.ClearCapture(t, top, ate)
		t.Errorf("Capture And ValidatePackets Failed (): %q", err)
	}
	packetvalidationhelpers.ClearCapture(t, top, ate)
}

func sendTraffic(t *testing.T, ate *ondatra.ATEDevice, traffictype string) {
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	if traffictype == "IPv4" {
		flowIPv4Validation.IsIPv4Interfaceresolved(t, ate)
	}
	ate.OTG().StartTraffic(t)
	time.Sleep(120 * time.Second)
	ate.OTG().StopTraffic(t)
}

func createflow(t *testing.T, top gosnappi.Config, params *otgconfighelpers.Flow, clearFlows bool) {
	t.Helper()
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
	var dutAggPorts []*ondatra.Port
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

func decapMPLSInGRE(t *testing.T, dut *ondatra.DUTDevice, pf *oc.NetworkInstance_PolicyForwarding, ni *oc.NetworkInstance, ocPFParams cfgplugins.OcPolicyForwardingParams) {
	t.Helper()
	cfgplugins.MplsConfig(t, dut)
	cfgplugins.QosClassificationConfig(t, dut)
	cfgplugins.LabelRangeConfig(t, dut)
	cfgplugins.DecapGroupConfigGre(t, dut, pf, ocPFParams)
	cfgplugins.MPLSStaticLSPConfig(t, dut, ni, ocPFParams)
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
