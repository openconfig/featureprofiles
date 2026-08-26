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
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	otgtelemetry "github.com/openconfig/ondatra/gnmi/otg"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	ieee8023adLag          = oc.IETFInterfaces_InterfaceType_ieee8023adLag
	mplsLabelCount         = 2000
	intCount               = 2000
	dutIntStartIP          = "169.254.0.1"
	otgIntStartIP          = "169.254.0.2"
	dutIntStartIPV6        = "2000:0:0:1::1"
	otgIntStartIPV6        = "2000:0:0:1::2"
	intStepV4              = "0.0.0.4"
	intStepV6              = "0:0:0:1::"
	mplsLabelStep          = 200
	mplsLabelStartforIpv4  = 16
	mplsLabelStartforIpv6  = 524280
	staticRoutePrefix      = "10.99.1.0/24"
	staticRouteV6Prefix    = "3000:1::/64"
	staticRouteNextHop     = "194.0.2.2"
	outerSrcIPv4           = "100.64.0.1"
	outerDstIPv4           = "11.1.1.1"
	innerSrcIPv4           = "22.1.1.1"
	innerDstIPv4           = "21.1.1.1"
	innerSrcIPv6           = "2000:1::1"
	innerDstIPv6           = "3000:1::1"
	flowSrcCount           = 10000
	ratePPS                = 100
	totalPkts              = 0
	sleepTime              = 15
	carrierDelayUp         = 3000
	carrierDelayDown       = 150
	outerFlowRate          = 0
	innerTrafficClassCount = 0
	innerTrafficClass      = 10
	innerRawPriorityCount  = 0
	innerRawPriority       = 10
	innerSrcCount          = 0
	innerSrcPort           = 49152
	tolerancePct           = 1
	pushStartWaitTime      = 30 * time.Second
)

var (
	top       = gosnappi.NewConfig()
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
		Flowrate:          100,
		PacketsToSend:     totalPkts,
		FlowName:          "MPLSOGRE traffic IPv4 interface IPv4 Payload",
		EthFlow:           &otgconfighelpers.EthFlowParams{SrcMAC: agg2.AggMAC},
		IPv4Flow:          &otgconfighelpers.IPv4FlowParams{IPv4Src: outerSrcIPv4, IPv4Dst: outerDstIPv4, IPv4SrcCount: flowSrcCount},
		MPLSFlow:          &otgconfighelpers.MPLSFlowParams{MPLSLabel: mplsLabelStartforIpv4, MPLSExp: 7, MPLSLabelCount: mplsLabelCount, MPLSLabelStep: mplsLabelStep},
		GREFlow:           &otgconfighelpers.GREFlowParams{Protocol: otgconfighelpers.IanaMPLSEthertype},
	}
	// FlowOuterIPv4Validation MPLSOGRE traffic IPv4 interface IPv4 Payload.
	flowOuterIPv4Validation = &otgvalidationhelpers.OTGValidation{
		Interface: &otgvalidationhelpers.InterfaceParams{Names: []string{}, Ports: append(agg1.MemberPorts, agg2.MemberPorts...)},
		Flow:      &otgvalidationhelpers.FlowParams{Name: flowOuterIPv4.FlowName, TolerancePct: tolerancePct},
	}
	// FlowInnerIPv4 Inner Header IPv4 Payload.
	flowInnerIPv4 = &otgconfighelpers.Flow{
		IPv4Flow: &otgconfighelpers.IPv4FlowParams{IPv4Src: innerSrcIPv4, IPv4Dst: innerDstIPv4, IPv4SrcCount: flowSrcCount, RawPriority: 0, RawPriorityCount: 255},
		TCPFlow:  &otgconfighelpers.TCPFlowParams{TCPSrcPort: innerSrcPort, TCPDstPort: 80, TCPSrcCount: flowSrcCount},
	}
	// FlowOuterIPv6 Decap IPv6 Interface IPv6 Payload traffic params Outer Header.
	flowOuterIPv6 = &otgconfighelpers.Flow{
		TxNames:           []string{agg2.Interfaces[0].Name + ".IPv4"},
		RxNames:           []string{},
		SizeWeightProfile: &sizeWeightProfile,
		Flowrate:          100,
		PacketsToSend:     totalPkts,
		FlowName:          "MPLSOGRE traffic IPv6 interface IPv6 Payload",
		EthFlow:           &otgconfighelpers.EthFlowParams{SrcMAC: agg2.AggMAC},
		IPv4Flow:          &otgconfighelpers.IPv4FlowParams{IPv4Src: outerSrcIPv4, IPv4Dst: outerDstIPv4, IPv4SrcCount: flowSrcCount},
		MPLSFlow:          &otgconfighelpers.MPLSFlowParams{MPLSLabel: mplsLabelStartforIpv6, MPLSExp: 7, MPLSLabelCount: mplsLabelCount, MPLSLabelStep: mplsLabelStep},
		GREFlow:           &otgconfighelpers.GREFlowParams{Protocol: otgconfighelpers.IanaMPLSEthertype},
	}
	// FlowOuterIPv6Validation MPLSOGRE traffic IPv6 interface IPv6 Payload.
	flowOuterIPv6Validation = &otgvalidationhelpers.OTGValidation{
		Interface: &otgvalidationhelpers.InterfaceParams{Names: []string{}, Ports: append(agg1.MemberPorts, agg2.MemberPorts...)},
		Flow:      &otgvalidationhelpers.FlowParams{Name: flowOuterIPv6.FlowName, TolerancePct: tolerancePct},
	}
	// FlowInnerIPv6 Inner Header IPv6 Payload.
	flowInnerIPv6 = &otgconfighelpers.Flow{
		IPv6Flow: &otgconfighelpers.IPv6FlowParams{IPv6Src: innerSrcIPv6, IPv6Dst: innerDstIPv6, IPv6SrcCount: flowSrcCount, TrafficClass: 0, TrafficClassCount: 255},
		TCPFlow:  &otgconfighelpers.TCPFlowParams{TCPSrcPort: innerSrcPort, TCPDstPort: 80, TCPSrcCount: flowSrcCount},
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
		IPv4Layer:   &packetvalidationhelpers.IPv4Layer{DstIP: innerDstIPv4, Tos: 10, TTL: 64, Protocol: packetvalidationhelpers.TCP},
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

	dutIPsV6, err := iputil.GenerateIPv6sWithStep(dutIntStartIPV6, intCount, intStepV6)
	if err != nil {
		return nil, fmt.Errorf("failed to generate DUT IPv6s: %w", err)
	}

	otgIPsV6, err := iputil.GenerateIPv6sWithStep(otgIntStartIPV6, intCount, intStepV6)
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
func configureDUT(t *testing.T, dut *ondatra.DUTDevice, netConfig *networkConfig, ocPFParams cfgplugins.OcPolicyForwardingParams) string {
	t.Helper()
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

	custAggID := netutil.NextAggregateInterface(t, dut)
	configureInterfaces(t, dut, custPorts, interfaces, custAggID)
	coreAggID := netutil.NextAggregateInterface(t, dut)
	configureInterfaces(t, dut, corePorts, []*attrs.Attributes{&coreIntf}, coreAggID)
	configureStaticRoute(t, dut)
	_, ni, pf := cfgplugins.SetupPolicyForwardingInfraOC(ocPFParams.NetworkInstanceName)
	decapMPLSInGRE(t, dut, pf, ni, netConfig, ocPFParams)
	return custAggID
}

// waitForLAGUp waits until all specified member ports and the aggregate interface (LAG) reach an operational UP state on the DUT.
func waitForLAGUp(t *testing.T, dut *ondatra.DUTDevice, aggID string, ports []string) {
	t.Helper()

	t.Logf("Waiting for LAG %s to be UP...", aggID)

	// Wait for member ports UP
	for _, p := range ports {
		port := dut.Port(t, p)
		gnmi.Await(t, dut, gnmi.OC().Interface(port.Name()).OperStatus().State(), 2*time.Minute, oc.Interface_OperStatus_UP)
		t.Logf("Port %s is UP", p)
	}

	// Wait for LAG interface UP
	gnmi.Await(t, dut, gnmi.OC().Interface(aggID).OperStatus().State(), 3*time.Minute, oc.Interface_OperStatus_UP)

	t.Logf("LAG %s is UP", aggID)
}

func configureDUTAndOTG(t *testing.T) (*ondatra.DUTDevice, string, *networkConfig) {
	t.Helper()
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

	for i := range intCount {
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

	// Get default parameters for OC Policy Forwarding
	ocPFParams := fetchDefaultOcPolicyForwardingParams()

	// Pass ocPFParams to ConfigureDUT
	ocPFParams.DecapPolicy.DecapMPLSParams.MplsStaticLabels = mplsStaticLabels
	ocPFParams.DecapPolicy.DecapMPLSParams.MplsStaticLabelsForIPv6 = mplsStaticLabelsForIpv6
	custAggID := configureDUT(t, dut, netConfig, ocPFParams)

	for _, intf := range agg1.Interfaces {
		flowOuterIPv4Validation.Interface.Names = append(flowOuterIPv4Validation.Interface.Names, intf.Name)
		flowOuterIPv4.RxNames = append(flowOuterIPv4.RxNames, intf.Name+".IPv4")
		flowOuterIPv6Validation.Interface.Names = append(flowOuterIPv6Validation.Interface.Names, intf.Name)
		flowOuterIPv6.RxNames = append(flowOuterIPv6.RxNames, intf.Name+".IPv6")
	}

	for i, iface := range agg1.Interfaces {
		// Limiting it to 50 since checking ARP for 2000 interfaces takes long time
		if i >= 50 {
			break
		}
		flowResolveArp.Interface.Names = append(flowResolveArp.Interface.Names, iface.Name)
	}

	configureOTG(t)
	waitForLAGUp(t, dut, custAggID, custPorts)
	waitForSubinterfacesUp(t, dut, custAggID, netConfig, 180*time.Second)
	return dut, custAggID, netConfig
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

// decapMPLSInGRE configures the MPLS in GRE decap policy on the DUT.
func decapMPLSInGRE(t *testing.T, dut *ondatra.DUTDevice, pf *oc.NetworkInstance_PolicyForwarding, ni *oc.NetworkInstance, netConfig *networkConfig, ocPFParams cfgplugins.OcPolicyForwardingParams) {
	t.Helper()
	ocPFParams.DecapPolicy.DecapMPLSParams.NextHops = netConfig.otgIPs
	ocPFParams.DecapPolicy.DecapMPLSParams.NextHopsV6 = netConfig.otgIPsV6
	ocPFParams.DecapPolicy.DecapMPLSParams.ScaleStaticLSP = true
	cfgplugins.MplsConfig(t, dut)
	cfgplugins.QosClassificationConfig(t, dut)
	cfgplugins.LabelRangeConfig(t, dut)
	cfgplugins.DecapGroupConfigGre(t, dut, pf, ocPFParams)
	cfgplugins.MPLSStaticLSPConfig(t, dut, ni, ocPFParams)
	if !deviations.PolicyForwardingOCUnsupported(dut) {
		pushPolicyForwardingConfig(t, dut, ni)
	}
}

// pushAndStartProtocols pushes the OTG configuration to the ATE, starts all control-plane protocols, and waits for protocol convergence.
func pushAndStartProtocols(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config, pushStartWaitTime time.Duration) {
	t.Helper()

	t.Log("Pushing OTG config...")
	ate.OTG().PushConfig(t, top)
	t.Log("Starting protocols...")
	ate.OTG().StartProtocols(t)

	if err := waitForOTGProtocolsUpWithRetry(t, ate, top, pushStartWaitTime, false); err != nil {
		t.Log("Protocols not UP on first attempt, restarting once...")

		// Restart once
		ate.OTG().StopProtocols(t)
		ate.OTG().StartProtocols(t)

		if err := waitForOTGProtocolsUpWithRetry(t, ate, top, pushStartWaitTime, true); err != nil {
			t.Fatalf("Protocols failed to come UP even after restart: %v", err)
		}
	}

	t.Log("Protocols are stable and ready")
}

// waitForSubinterfacesUp validates that sample DUT subinterfaces across the range are configured and operational.
func waitForSubinterfacesUp(t *testing.T, dut *ondatra.DUTDevice, aggID string, netConfig *networkConfig, timeout time.Duration) {
	t.Helper()
	t.Logf("Waiting for sample subinterfaces on %s...", aggID)

	sampleIndices := []int{0, 1, 9, 49, 99, 499, 999, 1499, len(netConfig.dutIPs) - 1}
	for _, i := range sampleIndices {
		if i < 0 || i >= len(netConfig.dutIPs) {
			continue
		}
		subif := uint32(i + 1)
		// -------------------------------
		// IPv4 Address Check
		// -------------------------------
		ipv4 := netConfig.dutIPs[i]

		_, ok := gnmi.Watch(t, dut, gnmi.OC().Interface(aggID).Subinterface(subif).Ipv4().Address(ipv4).PrefixLength().State(), timeout,
			func(val *ygnmi.Value[uint8]) bool {
				_, present := val.Val()
				return present
			},
		).Await(t)

		if !ok {
			t.Fatalf("IPv4 not configured on %s.%d", aggID, subif)
		}
		// -------------------------------
		// IPv6 Address Check
		// -------------------------------
		ipv6 := netConfig.dutIPsV6[i]

		_, ok = gnmi.Watch(t, dut, gnmi.OC().Interface(aggID).Subinterface(subif).Ipv6().Address(ipv6).PrefixLength().State(), timeout,
			func(val *ygnmi.Value[uint8]) bool {
				_, present := val.Val()
				return present
			},
		).Await(t)

		if !ok {
			t.Fatalf("IPv6 not configured on %s.%d", aggID, subif)
		}
	}

	t.Log("Sample subinterfaces are configured successfully")
}

// waitForOTGProtocolsUpWithRetry waits for all OTG ports and LAGs to reach an operational UP state within the given timeout.
func waitForOTGProtocolsUpWithRetry(t *testing.T, ate *ondatra.ATEDevice, config gosnappi.Config, pushStartWaitTime time.Duration, strict bool) error {
	t.Helper()
	t.Log("Waiting for OTG ports to be UP...")
	for _, p := range config.Ports().Items() {
		_, ok := gnmi.Watch(t, ate.OTG(), gnmi.OTG().Port(p.Name()).Link().State(), pushStartWaitTime,
			func(val *ygnmi.Value[otgtelemetry.E_Port_Link]) bool {
				state, present := val.Val()
				return present && state == otgtelemetry.Port_Link_UP
			}).Await(t)

		if !ok {
			if strict {
				return fmt.Errorf("port %s not UP", p.Name())
			}
			return fmt.Errorf("retry needed: port %s not UP", p.Name())
		}
		t.Logf("Port %s is UP", p.Name())
	}

	t.Log("Waiting for LAGs to be UP...")
	for _, lag := range config.Lags().Items() {
		_, ok := gnmi.Watch(t, ate.OTG(), gnmi.OTG().Lag(lag.Name()).OperStatus().State(), pushStartWaitTime,
			func(val *ygnmi.Value[otgtelemetry.E_Lag_OperStatus]) bool {
				state, present := val.Val()
				return present && state == otgtelemetry.Lag_OperStatus_UP
			}).Await(t)

		if !ok {
			if strict {
				return fmt.Errorf("LAG %s not UP", lag.Name())
			}
			return fmt.Errorf("retry needed: LAG %s not UP", lag.Name())
		}
		t.Logf("LAG %s is UP", lag.Name())
	}

	return nil
}

func sendTraffic(t *testing.T, ate *ondatra.ATEDevice, dut *ondatra.DUTDevice, custAggID string, netConfig *networkConfig) {
	t.Helper()
	pushAndStartProtocols(t, ate, top, pushStartWaitTime)
	if err := flowResolveArp.IsIPv4Interfaceresolved(t, ate); err != nil {
		t.Fatalf("Failed to resolve IPv4 interface for ATE: %v, error: %v", ate, err)
	}
	if err := flowResolveArp.IsIPv6Interfaceresolved(t, ate); err != nil {
		t.Fatalf("Failed to resolve IPv6 interface for ATE: %v, error: %v", ate, err)
	}
	ate.OTG().StartTraffic(t)
	time.Sleep(sleepTime * time.Second)
	ate.OTG().StopTraffic(t)
	otgutils.LogFlowMetrics(t, ate.OTG(), top)
	otgutils.LogPortMetrics(t, ate.OTG(), top)
}

func sendTrafficCapture(t *testing.T, ate *ondatra.ATEDevice, dut *ondatra.DUTDevice, custAggID string, netConfig *networkConfig) {
	t.Helper()
	pushAndStartProtocols(t, ate, top, pushStartWaitTime)
	if err := flowResolveArp.IsIPv4Interfaceresolved(t, ate); err != nil {
		t.Fatalf("Failed to resolve IPv4 interface for ATE: %v, error: %v", ate, err)
	}
	if err := flowResolveArp.IsIPv6Interfaceresolved(t, ate); err != nil {
		t.Fatalf("Failed to resolve IPv6 interface for ATE: %v, error: %v", ate, err)
	}
	cs := packetvalidationhelpers.StartCapture(t, ate)
	ate.OTG().StartTraffic(t)
	time.Sleep(sleepTime * time.Second)
	ate.OTG().StopTraffic(t)
	packetvalidationhelpers.StopCapture(t, ate, cs)
}

func createflow(t *testing.T, top gosnappi.Config, outer *otgconfighelpers.Flow, inner *otgconfighelpers.Flow, clearFlows bool) {
	t.Helper()

	if clearFlows {
		top.Flows().Clear()
	}

	outerCopy := *outer

	if outer.IPv4Flow != nil {
		ipv4 := *outer.IPv4Flow
		outerCopy.IPv4Flow = &ipv4
	}
	if outer.IPv6Flow != nil {
		ipv6 := *outer.IPv6Flow
		outerCopy.IPv6Flow = &ipv6
	}
	if outer.TCPFlow != nil {
		tcp := *outer.TCPFlow
		outerCopy.TCPFlow = &tcp
	}
	if outer.UDPFlow != nil {
		udp := *outer.UDPFlow
		outerCopy.UDPFlow = &udp
	}
	if outer.MPLSFlow != nil {
		mpls := *outer.MPLSFlow
		outerCopy.MPLSFlow = &mpls
	}
	if outer.GREFlow != nil {
		gre := *outer.GREFlow
		outerCopy.GREFlow = &gre
	}

	outerCopy.CreateFlow(top)
	outerCopy.AddEthHeader()

	if outerCopy.IPv4Flow != nil {
		outerCopy.AddIPv4Header()
	}
	if outerCopy.GREFlow != nil {
		outerCopy.AddGREHeader()
	}
	if outerCopy.MPLSFlow != nil {
		outerCopy.AddMPLSHeader()
	}

	if inner != nil {
		if inner.IPv4Flow != nil {
			ipv4 := *inner.IPv4Flow
			outerCopy.IPv4Flow = &ipv4
			outerCopy.AddIPv4Header()
		}

		if inner.IPv6Flow != nil {
			ipv6 := *inner.IPv6Flow
			outerCopy.IPv6Flow = &ipv6
			outerCopy.AddIPv6Header()
		}

		if inner.TCPFlow != nil {
			tcp := *inner.TCPFlow
			outerCopy.TCPFlow = &tcp
			outerCopy.AddTCPHeader()
		}

		if inner.UDPFlow != nil {
			udp := *inner.UDPFlow
			outerCopy.UDPFlow = &udp
			outerCopy.AddUDPHeader()
		}
	}
}

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
	configDUTInterface(t, agg, subinterfaces, dut)
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

	_, ok := gnmi.Watch(t, dut, gnmi.OC().Interface(aggID).OperStatus().State(), time.Minute, func(val *ygnmi.Value[oc.E_Interface_OperStatus]) bool {
		status, present := val.Val()
		return present && status == oc.Interface_OperStatus_UP
	}).Await(t)
	if !ok {
		t.Fatalf("LAG  %s is not ready. Expected %s got %s", aggID, oc.Interface_OperStatus_UP.String(), gnmi.Get(t, dut, gnmi.OC().Interface(aggID).OperStatus().State()).String())
	}
}

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

func configureStaticRoute(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	b := &gnmi.SetBatch{}
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
	sV6 := &cfgplugins.StaticRouteCfg{
		NetworkInstance: deviations.DefaultNetworkInstance(dut),
		Prefix:          staticRouteV6Prefix,
		NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
			"1": oc.UnionString(agg2interface.IPv6Gateway),
		},
	}
	if _, err := cfgplugins.NewStaticRouteCfg(b, sV6, dut); err != nil {
		t.Fatalf("Failed to configure IPv6 static route: %v", err)
	}
	b.Set(t, dut)
}

func pushPolicyForwardingConfig(t *testing.T, dut *ondatra.DUTDevice, ni *oc.NetworkInstance) {
	t.Helper()
	niPath := gnmi.OC().NetworkInstance(ni.GetName()).Config()
	gnmi.Replace(t, dut, niPath, ni)
}

func TestMPLSOGREDecapScale(t *testing.T) {
	ate := ondatra.ATE(t, "ate")

	dut, custAggID, netConfig := configureDUTAndOTG(t)

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
				sendTrafficCapture(t, ate, dut, custAggID, netConfig)
			} else {
				createflow(t, top, tc.outerFlow, tc.innerFlow, true)
				sendTraffic(t, ate, dut, custAggID, netConfig)
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
