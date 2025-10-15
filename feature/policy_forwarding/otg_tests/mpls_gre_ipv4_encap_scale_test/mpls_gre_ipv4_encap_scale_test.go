// Package mpls_gre_ipv4_encap_scale_test tests mplsogre encap functionality.
package mpls_gre_ipv4_encap_scale_test

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
	"github.com/openconfig/ygot/ygot"
)

// TestMain calls main function.
func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	ieee8023adLag                = oc.IETFInterfaces_InterfaceType_ieee8023adLag
	greProtocol                  = 47
	dutIntStartIP                = "169.254.0.1"
	otgIntStartIP                = "169.254.0.2"
	dutIntStartIPV6              = "2000:0:0:1::1"
	otgIntStartIPV6              = "2000:0:0:1::2"
	stepV4                       = "0.0.0.4"
	stepV6                       = "0:0:0:1::"
	greTunnelDestinationsStartIP = "10.99.1.1/32"
	trafficDuration              = 20 * time.Second
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
		Interfaces:  []*otgconfighelpers.InterfaceProperties{agg2Interface},
		LagID:       2,
		IsLag:       true,
	}

	agg2Interface = &otgconfighelpers.InterfaceProperties{
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
	// MPLSOGRE Encap IPv4 interface IPv4 Payload
	flowIPv4 = &otgconfighelpers.Flow{
		TxNames:           []string{},
		RxNames:           []string{agg2.Name + ".IPv4"},
		SizeWeightProfile: &sizeWeightProfile,
		Flowrate:          40,
		FlowName:          "GCI traffic IPv4 interface IPv4 Payload",
		EthFlow:           &otgconfighelpers.EthFlowParams{SrcMAC: agg1.AggMAC},
		VLANFlow:          &otgconfighelpers.VLANFlowParams{VLANId: 1},
		IPv4Flow:          &otgconfighelpers.IPv4FlowParams{IPv4Src: "12.1.1.1", IPv4Dst: "194.0.2.2", IPv4SrcCount: 100},
	}

	flowIPv4Validation = &otgvalidationhelpers.OTGValidation{
		Interface: &otgvalidationhelpers.InterfaceParams{Names: []string{agg2.Name}, Ports: append(agg1.MemberPorts, agg2.MemberPorts...)},
		Flow:      &otgvalidationhelpers.FlowParams{Name: flowIPv4.FlowName, TolerancePct: 0.5},
	}

	// MPLSOGRE Encap IPv6 interface IPv6 Payload
	flowIPv6 = &otgconfighelpers.Flow{
		TxNames:   []string{},
		RxNames:   []string{agg2.Name + ".IPv6"},
		FrameSize: 1500,
		Flowrate:  40,
		FlowName:  "GCI traffic IPv6 interface IPv6 Payload",
		EthFlow:   &otgconfighelpers.EthFlowParams{SrcMAC: agg1.AggMAC},
		VLANFlow:  &otgconfighelpers.VLANFlowParams{VLANId: 1},
		IPv6Flow:  &otgconfighelpers.IPv6FlowParams{IPv6Src: "3000:1::1", IPv6Dst: "2000:1::1", IPv6SrcCount: 100, TrafficClass: 0, TrafficClassCount: 100},
	}

	flowIPv6Validation = &otgvalidationhelpers.OTGValidation{
		Interface: &otgvalidationhelpers.InterfaceParams{Names: []string{agg2.Name}, Ports: append(agg1.MemberPorts, agg2.MemberPorts...)},
		Flow:      &otgvalidationhelpers.FlowParams{Name: flowIPv6.FlowName, TolerancePct: 0.5},
	}
	validations = []packetvalidationhelpers.ValidationType{
		packetvalidationhelpers.ValidateIPv4Header,
		packetvalidationhelpers.ValidateMPLSLayer,
		packetvalidationhelpers.ValidateInnerIPv4Header,
	}
	outerGREIPLayerIPv4 = &packetvalidationhelpers.IPv4Layer{
		DstIP:    "10.99.1.20",
		Protocol: greProtocol,
		Tos:      96,
		TTL:      64,
	}
	mplsLayer = &packetvalidationhelpers.MPLSLayer{
		Label: 19016,
		Tc:    1,
	}
	innerIPLayerIPv4 = &packetvalidationhelpers.IPv4Layer{
		DstIP: "194.0.2.2",
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

	greTunnelSources = []string{
		"10.235.143.208", "10.235.143.209", "10.235.143.210", "10.235.143.211",
		"10.235.143.212", "10.235.143.213", "10.235.143.215", "10.235.143.216",
		"10.235.143.217", "10.235.143.218", "10.235.143.219", "10.235.143.220",
		"10.235.143.221", "10.235.143.222", "10.235.143.223", "10.235.143.224",
	}
)

// configureOTG configures the OTG interfaces and pushes the configuration.
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
}

type networkConfig struct {
	dutIPs   []string
	otgIPs   []string
	otgMACs  []string
	dutIPsV6 []string
	otgIPsV6 []string
}

// generateNetConfig generates IPv4 and IPv6 network configurations for DUT and OTG.
func generateNetConfig(intCount int) (*networkConfig, error) {
	dutIPs, err := iputil.GenerateIPsWithStep(dutIntStartIP, intCount, stepV4)
	if err != nil {
		return nil, fmt.Errorf("failed to generate DUT IPs: %w", err)
	}

	otgIPs, err := iputil.GenerateIPsWithStep(otgIntStartIP, intCount, stepV4)
	if err != nil {
		return nil, fmt.Errorf("failed to generate OTG IPs: %w", err)
	}

	otgMACs := iputil.GenerateMACs("00:00:00:00:00:AA", intCount, "00:00:00:00:00:01")

	dutIPsV6, err := iputil.GenerateIPv6sWithStep(dutIntStartIPV6, intCount, stepV6)
	if err != nil {
		return nil, fmt.Errorf("failed to generate DUT IPv6s: %w", err)
	}

	otgIPsV6, err := iputil.GenerateIPv6sWithStep(otgIntStartIPV6, intCount, stepV6)
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

// configureDUT configures DUT interfaces, static routes, and MPLS-in-GRE encapsulation.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice, netConfig *networkConfig, encapParams cfgplugins.OCEncapsulationParams, ocPFParams cfgplugins.OcPolicyForwardingParams, ocNHGParams cfgplugins.StaticNextHopGroupParams) {
	t.Helper()
	aggID = netutil.NextAggregateInterface(t, dut)
	var interfaces []*attrs.Attributes
	for i := range encapParams.Count {
		interfaces = append(interfaces, &attrs.Attributes{
			Desc:         "Customer_connect",
			MTU:          1500,
			IPv4:         netConfig.dutIPs[i],
			IPv4Len:      30,
			IPv6:         netConfig.dutIPsV6[i],
			IPv6Len:      126,
			Subinterface: uint32(i + 1),
		})
	}

	configureInterfaces(t, dut, custPorts, interfaces, aggID)
	configureInterfacePropertiesScale(t, dut, aggID, ocPFParams, interfaces)
	aggID = netutil.NextAggregateInterface(t, dut)
	configureInterfaces(t, dut, corePorts, []*attrs.Attributes{&coreIntf}, aggID)
	mustConfigureStaticRoute(t, dut)
	_, ni, pf := cfgplugins.SetupPolicyForwardingInfraOC(ocPFParams.NetworkInstanceName)

	encapMPLSInGRE(t, dut, pf, ni, encapParams, ocPFParams, ocNHGParams)

}

// mustConfigureSetup sets up the DUT and OTG configuration including encapsulation parameters.
func mustConfigureSetup(t *testing.T) {
	t.Helper()
	dut := ondatra.DUT(t, "dut")
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	// Get default parameters for OC Policy Forwarding
	ocPFParams := fetchDefaultOCPolicyForwardingParams()
	ocNHGParams := fetchDefaultStaticNextHopGroupParams()
	ocEncapParams := fetchDefaultOCEncapParams()

	switch dut.Vendor() {
	case ondatra.ARISTA:
		ocEncapParams.Count = 250
		ocEncapParams.MPLSLabelCount = 250
		ocEncapParams.MPLSLabelStartForIPv4 = 16
		ocEncapParams.MPLSLabelStartForIPv6 = 524280
		ocEncapParams.MPLSLabelStep = 1000
		ocEncapParams.GRETunnelSources = greTunnelSources
		ocEncapParams.GRETunnelDestinationsStartIP = greTunnelDestinationsStartIP
	default:
		t.Fatalf("Unsupported vendor %s for now, need to check the maximum interface count", dut.Vendor())
	}

	flowIPv4.VLANFlow.VLANCount = uint32(ocEncapParams.Count)
	flowIPv6.VLANFlow.VLANCount = uint32(ocEncapParams.Count)

	netConfig, err := generateNetConfig(int(ocEncapParams.Count))
	if err != nil {
		t.Fatalf("Error generating net config: %v", err)
	}
	ocEncapParams.MPLSStaticLabels = func() []int {
		var r []int
		for count, label := 0, ocEncapParams.MPLSLabelStartForIPv4; count < ocEncapParams.MPLSLabelCount; count++ {
			r = append(r, label)
			label += ocEncapParams.MPLSLabelStep
		}
		return r
	}()

	ocEncapParams.MPLSStaticLabelsForIPv6 = func() []int {
		var r []int
		for count, label := 0, ocEncapParams.MPLSLabelStartForIPv6; count < ocEncapParams.MPLSLabelCount; count++ {
			r = append(r, label)
			label += ocEncapParams.MPLSLabelStep
		}
		return r
	}()

	for i := range ocEncapParams.Count {
		agg1.Interfaces = append(agg1.Interfaces, &otgconfighelpers.InterfaceProperties{
			Name:        fmt.Sprintf("agg1port%d", i+1),
			IPv4:        netConfig.otgIPs[i],
			IPv4Gateway: netConfig.dutIPs[i],
			Vlan:        uint32(i + 1),
			IPv4Len:     30,
			IPv6:        netConfig.otgIPsV6[i],
			IPv6Gateway: netConfig.dutIPsV6[i],
			IPv6Len:     126,
			MAC:         netConfig.otgMACs[i],
		})
	}

	// Pass ocPFParams to ConfigureDut
	configureDUT(t, dut, netConfig, ocEncapParams, ocPFParams, ocNHGParams)
	flowIPv4Validation.Interface.Names = append(flowIPv4Validation.Interface.Names, agg1.Interfaces[0].Name)
	flowIPv4.TxNames = append(flowIPv4.TxNames, agg1.Interfaces[0].Name+".IPv4")
	flowIPv6Validation.Interface.Names = append(flowIPv6Validation.Interface.Names, agg1.Interfaces[0].Name)
	flowIPv6.TxNames = append(flowIPv6.TxNames, agg1.Interfaces[0].Name+".IPv6")
	configureOTG(t)
}

// fetchDefaultStaticNextHopGroupParams provides default parameters for the generator.
// matching the values in the provided JSON example.
func fetchDefaultStaticNextHopGroupParams() cfgplugins.StaticNextHopGroupParams {
	return cfgplugins.StaticNextHopGroupParams{

		StaticNHGName: "MPLS_in_GRE_Encap",
		NHIPAddr1:     "nh_ip_addr_1",
		NHIPAddr2:     "nh_ip_addr_2",
		// TODO: b/417988636 - Set the MplsLabel to the correct value.
	}
}

// fetchDefaultOCPolicyForwardingParams provides default parameters for the generator,
// matching the values in the provided JSON example.
func fetchDefaultOCPolicyForwardingParams() cfgplugins.OcPolicyForwardingParams {
	return cfgplugins.OcPolicyForwardingParams{
		NetworkInstanceName: "DEFAULT",
		InterfaceID:         "Agg1.10",
		AppliedPolicyName:   "customer1",
	}
}

// fetchDefaultOCEncapParams provides default parameters for mpls gre encapsulation.
func fetchDefaultOCEncapParams() cfgplugins.OCEncapsulationParams {
	return cfgplugins.OCEncapsulationParams{}
}

// configureInterfacePropertiesScale configures interface properties for scale testing.
func configureInterfacePropertiesScale(t *testing.T, dut *ondatra.DUTDevice, aggID string, ocPFParams cfgplugins.OcPolicyForwardingParams, interfaces []*attrs.Attributes) {
	t.Helper()
	_, _, pf := cfgplugins.SetupPolicyForwardingInfraOC(ocPFParams.NetworkInstanceName)

	ocPFParams.Interfaces = interfaces
	ocPFParams.AggID = aggID
	b := new(gnmi.SetBatch)
	cfgplugins.InterfaceLocalProxyConfigScale(t, dut, b, ocPFParams)
	cfgplugins.InterfaceQosClassificationConfigScale(t, dut, b, ocPFParams)
	cfgplugins.InterfacePolicyForwardingConfigScale(t, dut, b, pf, ocPFParams)
	b.Set(t, dut)
}

// encapMPLSInGRE configures MPLS-in-GRE encapsulation on the DUT.
func encapMPLSInGRE(t *testing.T, dut *ondatra.DUTDevice, pf *oc.NetworkInstance_PolicyForwarding, ni *oc.NetworkInstance, encapParams cfgplugins.OCEncapsulationParams, ocPFParams cfgplugins.OcPolicyForwardingParams, ocNHGParams cfgplugins.StaticNextHopGroupParams) {
	t.Helper()
	cfgplugins.MplsConfig(t, dut)
	cfgplugins.QosClassificationConfig(t, dut)
	cfgplugins.LabelRangeConfig(t, dut)
	b := new(gnmi.SetBatch)
	cfgplugins.NextHopGroupConfigScale(t, dut, b, encapParams, ni, ocNHGParams)
	cfgplugins.PolicyForwardingConfigScale(t, dut, b, encapParams, pf, ocPFParams)
	b.Set(t, dut)
	if !deviations.PolicyForwardingOCUnsupported(dut) {
		pushPolicyForwardingConfig(t, dut, ni)
	}
}

// mustSendTraffic sends traffic using OTG and waits for the configured duration.
func mustSendTraffic(t *testing.T, ate *ondatra.ATEDevice) {
	t.Helper()
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	if err := flowResolveArp.IsIPv4Interfaceresolved(t, ate); err != nil {
		t.Fatalf("Failed to resolve IPv4 interface for ATE: %v, error: %v", ate, err)
	}
	ate.OTG().StartTraffic(t)
	time.Sleep(trafficDuration)
	ate.OTG().StopTraffic(t)
}

// createFlow creates and configures a traffic flow in the OTG configuration.
func createFlow(top gosnappi.Config, params *otgconfighelpers.Flow, clearFlows bool) {
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

// configureInterfaces configures aggregate interfaces and subinterfaces on the DUT.
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

// configDUTInterface sets up interface configuration including subinterfaces and IP addressing.
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

// configureInterfaceAddress configures IPv4 and IPv6 addresses on a subinterface.
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

// mustConfigureStaticRoute configures a static IPv4 route on the DUT.
func mustConfigureStaticRoute(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	b := new(gnmi.SetBatch)
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

// mustSendTrafficCapture sends traffic and captures packets for validation.
func mustSendTrafficCapture(t *testing.T, ate *ondatra.ATEDevice) {
	t.Helper()
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	if err := flowResolveArp.IsIPv4Interfaceresolved(t, ate); err != nil {
		t.Fatalf("Failed to resolve IPv4 interface for ATE: %v, error: %v", ate, err)
	}
	cs := packetvalidationhelpers.StartCapture(t, ate)
	ate.OTG().StartTraffic(t)
	time.Sleep(trafficDuration)
	ate.OTG().StopTraffic(t)
	packetvalidationhelpers.StopCapture(t, ate, cs)
}

// pushPolicyForwardingConfig pushes the policy forwarding configuration to the DUT.
func pushPolicyForwardingConfig(t *testing.T, dut *ondatra.DUTDevice, ni *oc.NetworkInstance) {
	t.Helper()
	niPath := gnmi.OC().NetworkInstance(ni.GetName()).Config()
	gnmi.Replace(t, dut, niPath, ni)
}

func TestMPLSOGREEncapScale(t *testing.T) {
	ate := ondatra.ATE(t, "ate")
	mustConfigureSetup(t)

	tests := []struct {
		name             string
		setupFlows       func()
		validateLoss     func(*testing.T, *ondatra.ATEDevice) error
		packetValidation bool
	}{
		{
			name: "IPv4 & IPv6 Flow Validation",
			setupFlows: func() {
				createFlow(top, flowIPv4, true)
				createFlow(top, flowIPv6, false)
			},
			validateLoss: func(t *testing.T, ate *ondatra.ATEDevice) error {
				if err := flowIPv4Validation.ValidateLossOnFlows(t, ate); err != nil {
					return fmt.Errorf("ipv4 flow validation failed: %v", err)
				}
				if err := flowIPv6Validation.ValidateLossOnFlows(t, ate); err != nil {
					return fmt.Errorf("ipv6 flow validation failed: %v", err)
				}
				return nil
			},
			packetValidation: false,
		},
		{
			name: "IPv4 Flow with Packet Capture",
			setupFlows: func() {
				flowIPv4.VLANFlow.VLANId = 20
				flowIPv4.VLANFlow.VLANCount = 0
				flowIPv4.IPv4Flow.RawPriority = 1
				flowIPv4.IPv4Flow.RawPriorityCount = 0
				flowIPv4.PacketsToSend = 1000
				createFlow(top, flowIPv4, true)
			},
			validateLoss:     flowIPv4Validation.ValidateLossOnFlows,
			packetValidation: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.setupFlows()

			if tc.packetValidation {
				packetvalidationhelpers.ConfigurePacketCapture(t, top, encapPacketValidation)
				mustSendTrafficCapture(t, ate)
				defer packetvalidationhelpers.ClearCapture(t, top, ate)
			} else {
				mustSendTraffic(t, ate)
			}

			if err := tc.validateLoss(t, ate); err != nil {
				t.Errorf("validation on flows failed: %q", err)
				if tc.packetValidation {
					packetvalidationhelpers.ClearCapture(t, top, ate)
				}
			}

			if tc.packetValidation {
				if err := packetvalidationhelpers.CaptureAndValidatePackets(t, ate, encapPacketValidation); err != nil {
					packetvalidationhelpers.ClearCapture(t, top, ate)
					t.Errorf("capture and validatepackets failed: %q", err)
				}
			}
		})
	}
}
