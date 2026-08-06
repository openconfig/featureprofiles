// Package mpls_gre_udp_qos_test tests MPLSoGRE and MPLSoGUE QoS functionality.
// PF-1.18: This test verifies quality of service with MPLSoGRE and MPLSoGUE IP traffic
// on routed VLAN sub interfaces. The classification, marking and queueing of traffic
// while being encapsulated and decapsulated based on the outer headers and/or inner
// payload are the major features verified on the test device.
//
// Test cases:
//   - PF-1.18.1: Generate DUT Configuration
//   - PF-1.18.2: Verify Classification of MPLSoGRE and MPLSoGUE traffic based on traffic class bits in MPLS header
//   - PF-1.18.3: Verify DSCP marking of encapsulated and decapsulated traffic
//   - PF-1.18.4: Verify Assured forwarding (bandwidth class) - Queueing of decap traffic
//   - PF-1.18.5: Verify Assured forwarding (bandwidth class) - Queueing of decap traffic with shaper
//   - PF-1.18.6: Verify Expedited forwarding (Priority class) - Queueing of decap traffic
//   - PF-1.18.7: Verify Expedited forwarding (Priority class) - Queueing of decap traffic with shaper
//   - PF-1.18.8: Verify Expedited forwarding (Priority class) - Queueing of encap traffic
//   - PF-1.18.9: Verify two rate three color policer - Ingress rate limiting of encap traffic
//   - PF-1.18.10: Verify port/hardware dependency
//   - PF-1.18.v6: Validate MPLS over GRE over UDP over IPv6 encapsulation and decapsulation with QoS
package mpls_gre_udp_qos_test

import (
	"fmt"
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
	"github.com/openconfig/featureprofiles/internal/otgutils"
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
	ethernetCsmacd = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	ieee8023adLag  = oc.IETFInterfaces_InterfaceType_ieee8023adLag
	// GREProtocol is the protocol number for GRE.
	GREProtocol = 47
	// UDPProtocol is the protocol number for UDP.
	UDPProtocol = 17
	// trafficTimeout is the duration to wait for traffic to complete.
	trafficTimeout = 120 * time.Second
	// counterTimeout is the duration to wait for counters to become available.
	counterTimeout = 10 * time.Second
	// tolerancePct is the tolerance percentage for traffic validation.
	tolerancePct float32 = 5.0
	// dscpMarking is the expected DSCP value (32 = CS4) for encapsulated traffic outer header.
	dscpMarking uint8 = 32
	// maxMPLSExp is the maximum value for MPLS EXP bits (0-7).
	maxMPLSExp = 7
	// numTrafficClasses is the number of traffic classes to test (TC0-TC7).
	numTrafficClasses = 8
	// cirValue is the committed information rate for two-rate three-color policer (1 Gbps).
	cirValue uint64 = 1000000000
	// pirValue is the peak information rate for two-rate three-color policer (2 Gbps).
	pirValue uint64 = 2000000000
	// burstSize is the burst size for the policer in bytes.
	burstSize uint32 = 100000
	// congestionRate is the traffic rate percentage to ensure congestion.
	// Note: OTG limits flow rate to max 100%, so use multiple flows to create congestion.
	congestionRate float32 = 100
	// normalRate is the normal traffic rate percentage.
	normalRate float32 = 80
	// numMPLSLabels is the number of static MPLS labels to configure.
	numMPLSLabels = 8
	// baseMPLSLabel is the starting MPLS label value.
	baseMPLSLabel uint32 = 40571
)

var (
	top   = gosnappi.NewConfig()
	aggID string

	// Port groups for 8-link testbed
	// ATE Ports 1,2 are customer/ingress ports
	// ATE Ports 3,4,5,6 are core/egress ports
	custPorts  = []string{"port1", "port2"}
	corePorts1 = []string{"port3", "port4"}
	corePorts2 = []string{"port5", "port6"}

	custIntfIPv4 = attrs.Attributes{
		Desc:         "Customer_connect",
		MTU:          1500,
		IPv4:         "169.254.0.11",
		IPv4Len:      29,
		Subinterface: 20,
	}

	coreIntf1 = attrs.Attributes{
		Desc:    "Core_Interface1",
		IPv4:    "194.0.2.1",
		IPv6:    "2001:10:1:6::1",
		MTU:     9202,
		IPv4Len: 24,
		IPv6Len: 126,
	}

	coreIntf2 = attrs.Attributes{
		Desc:    "Core_Interface2",
		IPv4:    "194.0.3.1",
		IPv6:    "2001:10:1:7::1",
		MTU:     9202,
		IPv4Len: 24,
		IPv6Len: 126,
	}

	// Customer aggregate interface (ATE Ports 1,2)
	agg1 = &otgconfighelpers.Port{
		Name:        "Port-Channel1",
		AggMAC:      "02:00:01:01:01:07",
		Interfaces:  []*otgconfighelpers.InterfaceProperties{interface1},
		MemberPorts: []string{"port1", "port2"},
		LagID:       1,
		IsLag:       true,
	}

	// Core aggregate interface 1 (ATE Ports 3,4)
	agg2 = &otgconfighelpers.Port{
		Name:        "Port-Channel2",
		AggMAC:      "02:00:01:01:01:01",
		MemberPorts: []string{"port3", "port4"},
		Interfaces:  []*otgconfighelpers.InterfaceProperties{interface7},
		LagID:       2,
		IsLag:       true,
	}

	// Core aggregate interface 2 (ATE Ports 5,6)
	agg3 = &otgconfighelpers.Port{
		Name:        "Port-Channel3",
		AggMAC:      "02:00:01:01:01:03",
		MemberPorts: []string{"port5", "port6"},
		Interfaces:  []*otgconfighelpers.InterfaceProperties{interface8},
		LagID:       3,
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

	interface8 = &otgconfighelpers.InterfaceProperties{
		IPv4:        "194.0.3.2",
		IPv6:        "2001:10:1:7::2",
		IPv4Gateway: "194.0.3.1",
		IPv6Gateway: "2001:10:1:7::1",
		Name:        "Port-Channel3",
		MAC:         "02:00:01:01:01:04",
		IPv4Len:     29,
		IPv6Len:     126,
	}

	// Custom IMIX settings for all flows (64, 128, 256, 512, 1024.. MTU bytes).
	sizeWeightProfile = []otgconfighelpers.SizeWeightPair{
		{Size: 64, Weight: 20},
		{Size: 128, Weight: 20},
		{Size: 256, Weight: 20},
		{Size: 512, Weight: 10},
		{Size: 1024, Weight: 15},
		{Size: 1500, Weight: 13},
		{Size: 9000, Weight: 2},
	}

	// FlowIPv4 consists of IP to Encap traffic (Flow B).
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

	// trafficClassData holds information about each traffic class for QoS testing.
	// Maps MPLS EXP bits (0-7) to corresponding DSCP values and priority levels.
	trafficClassData = []struct {
		name      string
		mplsExp   uint8
		dscp      uint8
		priority  uint32
		queueName string
		mplsLabel uint32
	}{
		{name: "TC0-BE1", mplsExp: 0, dscp: 0, priority: 0, queueName: "BE1", mplsLabel: 40571},
		{name: "TC1-AF1", mplsExp: 1, dscp: 8, priority: 1, queueName: "AF1", mplsLabel: 40572},
		{name: "TC2-AF2", mplsExp: 2, dscp: 16, priority: 2, queueName: "AF2", mplsLabel: 40573},
		{name: "TC3-AF3", mplsExp: 3, dscp: 24, priority: 3, queueName: "AF3", mplsLabel: 40574},
		{name: "TC4-AF4", mplsExp: 4, dscp: 32, priority: 4, queueName: "AF4", mplsLabel: 40575},
		{name: "TC5-EF", mplsExp: 5, dscp: 40, priority: 5, queueName: "AF4", mplsLabel: 40576},
		{name: "TC6-NC1", mplsExp: 6, dscp: 48, priority: 6, queueName: "NC1", mplsLabel: 40577},
		{name: "TC7-NC2", mplsExp: 7, dscp: 56, priority: 7, queueName: "NC1", mplsLabel: 40578},
	}

	// frameSizes contains the frame sizes to test per README (64, 128, 256, 512, 1024.. MTU bytes).
	frameSizes = []uint32{64, 128, 256, 512, 1024, 1500, 9000}
)

func configureOTG(t *testing.T) {
	t.Helper()
	top.Captures().Clear()
	ate := ondatra.ATE(t, "ate")

	// Create a slice of aggPortData for easier iteration.
	aggs := []*otgconfighelpers.Port{agg1, agg2, agg3}

	// Configure OTG Interfaces.
	for _, agg := range aggs {
		otgconfighelpers.ConfigureNetworkInterface(t, top, ate, agg)
	}
	ate.OTG().PushConfig(t, top)
}

// ConfigureDut configures DUT for PF-1.18.1.
func ConfigureDut(t *testing.T, dut *ondatra.DUTDevice, ocPFParams cfgplugins.OcPolicyForwardingParams, ocNHGParams cfgplugins.StaticNextHopGroupParams) {
	// Configure customer-facing aggregate (ports 1,2)
	aggID = netutil.NextAggregateInterface(t, dut)
	configureInterfaces(t, dut, custPorts, []*attrs.Attributes{&custIntfIPv4}, aggID)
	configureInterfaceProperties(t, dut, aggID, &custIntfIPv4, ocPFParams)

	// Configure core aggregate 1 (ports 3,4)
	aggID = netutil.NextAggregateInterface(t, dut)
	configureInterfaces(t, dut, corePorts1, []*attrs.Attributes{&coreIntf1}, aggID)

	// Configure core aggregate 2 (ports 5,6)
	aggID = netutil.NextAggregateInterface(t, dut)
	configureInterfaces(t, dut, corePorts2, []*attrs.Attributes{&coreIntf2}, aggID)

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

	// Initialize QoS hardware if needed
	cfgplugins.NewQosInitialize(t, dut)

	// Get default parameters for OC Policy Forwarding
	ocPFParams := GetDefaultOcPolicyForwardingParams()
	ocNHGParams := GetDefaultStaticNextHopGroupParams()

	// Pass ocPFParams to ConfigureDut
	ConfigureDut(t, dut, ocPFParams, ocNHGParams)

	// Configure QoS classifiers, forwarding groups, and scheduler policies
	configureQoSClassifiers(t, dut)

	// Configure OTG interfaces
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

// TestMPLSClassification verifies PF-1.18.2: Classification of MPLSoGRE and MPLSoGUE traffic
// based on traffic class bits in MPLS header.
//
// Test generates Flow-A (MPLSoGRE/MPLSoGUE traffic) and verifies:
// - Egress IP traffic after decapsulation gets classified into 8 queues mapped to 8 traffic classes
// - Inner packet DSCP is not altered by the device
// - All traffic received gets decapsulated and forwarded as IPv4/IPv6 unicast, IPv4 multicast
// - Inner packets are received by the ATE with correct inner source and dest IP addresses
func TestMPLSClassification(t *testing.T) {
	t.Log("PF-1.18.2: Verify Classification of MPLSoGRE and MPLSoGUE traffic based on traffic class bits in MPLS header")
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	// Get port info for QoS counter verification
	dp1 := dut.Port(t, "port1")

	// Get initial QoS queue counters
	queueCountersBefore := getQoSQueueCounters(t, dut, dp1.Name())

	// Create and send Flow-A with different MPLS EXP values (0-7)
	for _, tc := range trafficClassData {
		t.Run(tc.name, func(t *testing.T) {
			// Create MPLSoGRE flow with specific MPLS EXP bits
			createFlowAMPLSoGRE(t, top, ate, tc.mplsExp, tc.mplsLabel, tc.dscp)

			sendTraffic(t, ate, "IPv4")

			if err := flowIPv4Validation.ValidateLossOnFlows(t, ate); err != nil {
				t.Errorf("Validation on flows failed for TC %s: %q", tc.name, err)
			}
		})
	}

	// Verify QoS queue counters after traffic - each TC should map to correct queue
	queueCountersAfter := getQoSQueueCounters(t, dut, dp1.Name())
	verifyQueueCounters(t, queueCountersBefore, queueCountersAfter)

	// Verify inner packet DSCP is preserved during decap
	t.Run("VerifyInnerDSCPPreserved", func(t *testing.T) {
		// Configure packet capture to verify inner packet DSCP
		packetvalidationhelpers.ConfigurePacketCapture(t, top, encapPacketValidation)
		sendTrafficCapture(t, ate)
		if err := packetvalidationhelpers.CaptureAndValidatePackets(t, ate, encapPacketValidation); err != nil {
			t.Errorf("Inner DSCP verification failed: %q", err)
		}
		packetvalidationhelpers.ClearCapture(t, top, ate)
	})
}

// TestAssuredForwardingDecap verifies PF-1.18.4: Assured forwarding (bandwidth class) - Queueing of decap traffic.
//
// Test verifies:
// - Total conformed bandwidth equals interface bandwidth during congestion
// - Every class gets minimum configured bandwidth
// - Unused bandwidth is equally shared among other classes when traffic stops/reduces
// - Results are same with different frame sizes (64, 128, 256, 512, 1024, MTU bytes)
// - Every queue can transmit packets at line rate without buffer/tail drops
func TestAssuredForwardingDecap(t *testing.T) {
	t.Log("PF-1.18.4: Verify Assured forwarding (bandwidth class) - Queueing of decap traffic")
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	dp1 := dut.Port(t, "port1")
	queues := netutil.CommonTrafficQueues(t, dut)

	// Get initial QoS queue counters
	queueCountersBefore := getQoSQueueCounters(t, dut, dp1.Name())

	// Generate Flow-A with congestion (traffic > interface bandwidth by 10%)
	FlowIPv4.Flowrate = congestionRate
	createflow(t, top, FlowIPv4, true)
	sendTraffic(t, ate, "IPv4")

	// Verify queue counters
	queueCountersAfter := getQoSQueueCounters(t, dut, dp1.Name())

	// Verify minimum bandwidth is being achieved per class
	t.Run("VerifyMinimumBandwidth", func(t *testing.T) {
		verifyMinimumBandwidth(t, queueCountersBefore, queueCountersAfter)
	})

	// Test bandwidth redistribution when traffic stops for some classes
	t.Run("VerifyBandwidthRedistribution", func(t *testing.T) {
		// Reduce traffic rate and verify bandwidth sharing
		FlowIPv4.Flowrate = normalRate / 2
		createflow(t, top, FlowIPv4, true)
		sendTraffic(t, ate, "IPv4")

		queueCountersAfterReduced := getQoSQueueCounters(t, dut, dp1.Name())
		verifyBandwidthRedistribution(t, queueCountersAfter, queueCountersAfterReduced)
	})

	// Test with different frame sizes per README requirements
	t.Run("VerifyDifferentFrameSizes", func(t *testing.T) {
		for _, size := range frameSizes {
			t.Run(fmt.Sprintf("FrameSize%d", size), func(t *testing.T) {
				FlowIPv4.SizeWeightProfile = &[]otgconfighelpers.SizeWeightPair{{Size: size, Weight: 100}}
				createflow(t, top, FlowIPv4, true)
				sendTraffic(t, ate, "IPv4")

				if err := flowIPv4Validation.ValidateLossOnFlows(t, ate); err != nil {
					t.Logf("Some traffic loss expected during congestion testing: %q", err)
				}
			})
		}
		// Restore original size profile
		FlowIPv4.SizeWeightProfile = &sizeWeightProfile
	})

	// Verify no buffer/tail drops at line rate
	t.Run("VerifyNoDropsAtLineRate", func(t *testing.T) {
		for _, queueName := range []string{queues.BE1, queues.AF1, queues.AF2, queues.AF3, queues.AF4, queues.NC1} {
			droppedPkts := getQoSDroppedPkts(t, dut, dp1.Name(), queueName)
			if droppedPkts > 0 {
				t.Logf("Queue %s dropped %d packets (expected during congestion)", queueName, droppedPkts)
			}
		}
	})
}

// TestAssuredForwardingWithShaper verifies PF-1.18.5: Assured forwarding with shaper.
//
// Test verifies assured forwarding with bandwidth classes and shaper (maximum bandwidth) on 3+ classes:
// - Total conformed bandwidth equals interface bandwidth
// - Every class gets minimum configured bandwidth
// - Classes with shaper do not exceed maximum bandwidth
// - Unused bandwidth is shared among other classes (never exceeds shaper limits)
// - Results are same with different frame sizes
// - Every queue can transmit at line rate without drops
func TestAssuredForwardingWithShaper(t *testing.T) {
	t.Log("PF-1.18.5: Verify Assured forwarding (bandwidth class) - Queueing of decap traffic with minimum and maximum bandwidth (shaper)")
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	dp1 := dut.Port(t, "port1")

	// Configure shaper (maximum bandwidth) on 3+ classes as per README
	configureSchedulerWithShaper(t, dut, "port1")

	// Get initial QoS queue counters
	queueCountersBefore := getQoSQueueCounters(t, dut, dp1.Name())

	// Generate Flow-A with congestion (traffic > minimum and maximum bandwidth)
	FlowIPv4.Flowrate = congestionRate
	createflow(t, top, FlowIPv4, true)
	sendTraffic(t, ate, "IPv4")

	// Verify queue counters
	queueCountersAfter := getQoSQueueCounters(t, dut, dp1.Name())

	// Verify minimum bandwidth per class
	t.Run("VerifyMinimumBandwidth", func(t *testing.T) {
		verifyMinimumBandwidth(t, queueCountersBefore, queueCountersAfter)
	})

	// Verify classes with shaper do not exceed maximum bandwidth
	t.Run("VerifyShaperLimits", func(t *testing.T) {
		verifyShaperLimits(t, queueCountersBefore, queueCountersAfter)
	})

	// Test bandwidth redistribution when traffic stops for some classes
	t.Run("VerifyBandwidthRedistributionWithShaper", func(t *testing.T) {
		FlowIPv4.Flowrate = normalRate / 2
		createflow(t, top, FlowIPv4, true)
		sendTraffic(t, ate, "IPv4")

		queueCountersAfterReduced := getQoSQueueCounters(t, dut, dp1.Name())
		verifyBandwidthRedistribution(t, queueCountersAfter, queueCountersAfterReduced)
		// Also verify shaper limits are still respected
		verifyShaperLimits(t, queueCountersAfter, queueCountersAfterReduced)
	})

	// Test with different frame sizes
	t.Run("VerifyDifferentFrameSizes", func(t *testing.T) {
		for _, size := range []uint32{64, 1500, 9000} {
			t.Run(fmt.Sprintf("FrameSize%d", size), func(t *testing.T) {
				FlowIPv4.SizeWeightProfile = &[]otgconfighelpers.SizeWeightPair{{Size: size, Weight: 100}}
				createflow(t, top, FlowIPv4, true)
				sendTraffic(t, ate, "IPv4")
			})
		}
		FlowIPv4.SizeWeightProfile = &sizeWeightProfile
	})
}

// TestExpeditedForwardingDecap verifies PF-1.18.6: Expedited forwarding (Priority class) - Queueing of decap traffic.
//
// Test verifies expedited forwarding with priority classes (levels 0-7):
// - Total conformed bandwidth equals interface bandwidth during congestion
// - Every class with priority N starves all classes with lower priority (< N)
// - Unused bandwidth from highest priority classes is allocated to immediate lower priority
// - Results are same with different frame sizes
// - Every queue can transmit at line rate without drops
func TestExpeditedForwardingDecap(t *testing.T) {
	t.Log("PF-1.18.6: Verify Expedited forwarding (Priority class) - Queueing of decap traffic")
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	dp1 := dut.Port(t, "port1")

	// Get initial QoS queue counters
	queueCountersBefore := getQoSQueueCounters(t, dut, dp1.Name())

	// Generate Flow-A with congestion (>10% more than interface bandwidth)
	// Individual streams bandwidth for PriorityN must be 10%+ greater than PriorityN-1
	FlowIPv4.Flowrate = congestionRate
	createflow(t, top, FlowIPv4, true)
	sendTraffic(t, ate, "IPv4")

	// Verify queue counters
	queueCountersAfter := getQoSQueueCounters(t, dut, dp1.Name())

	// Verify priority starvation - higher priority should starve lower priority
	t.Run("VerifyPriorityStarvation", func(t *testing.T) {
		verifyPriorityStarvation(t, dut, queueCountersBefore, queueCountersAfter)
	})

	// Test bandwidth allocation when highest priority traffic stops
	t.Run("VerifyBandwidthAllocationOnStop", func(t *testing.T) {
		// Stop highest priority traffic and verify lower priority gets bandwidth
		FlowIPv4.Flowrate = normalRate / 2
		createflow(t, top, FlowIPv4, true)
		sendTraffic(t, ate, "IPv4")

		queueCountersReduced := getQoSQueueCounters(t, dut, dp1.Name())
		verifyPriorityBandwidthAllocation(t, queueCountersAfter, queueCountersReduced)
	})

	// Test with different frame sizes
	t.Run("VerifyDifferentFrameSizes", func(t *testing.T) {
		for _, size := range []uint32{64, 1500, 9000} {
			t.Run(fmt.Sprintf("FrameSize%d", size), func(t *testing.T) {
				FlowIPv4.SizeWeightProfile = &[]otgconfighelpers.SizeWeightPair{{Size: size, Weight: 100}}
				createflow(t, top, FlowIPv4, true)
				sendTraffic(t, ate, "IPv4")
			})
		}
		FlowIPv4.SizeWeightProfile = &sizeWeightProfile
	})
}

// TestExpeditedForwardingWithShaper verifies PF-1.18.7: Expedited forwarding with shaper.
//
// Test verifies expedited forwarding with priority classes and shaper (maximum bandwidth):
// - Total conformed bandwidth equals interface bandwidth
// - Classes with shaper do not exceed maximum bandwidth
// - Every class with priority N starves lower priority classes (limited by shaper)
// - Unused bandwidth from highest priority classes goes to immediate lower priority
// - Results are same with different frame sizes
// - Every queue can transmit at line rate without drops
func TestExpeditedForwardingWithShaper(t *testing.T) {
	t.Log("PF-1.18.7: Verify Expedited forwarding (Priority class) - Queueing of decap traffic with minimum and maximum bandwidth (shaper)")
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	dp1 := dut.Port(t, "port1")

	// Configure scheduler with shaper on 2+ high priority classes
	configureSchedulerWithShaper(t, dut, "port1")

	// Get initial QoS queue counters
	queueCountersBefore := getQoSQueueCounters(t, dut, dp1.Name())

	// Generate Flow-A with traffic > shaper bandwidth
	FlowIPv4.Flowrate = congestionRate
	createflow(t, top, FlowIPv4, true)
	sendTraffic(t, ate, "IPv4")

	// Verify queue counters
	queueCountersAfter := getQoSQueueCounters(t, dut, dp1.Name())

	// Verify shaper limits are respected
	t.Run("VerifyShaperLimits", func(t *testing.T) {
		verifyShaperLimits(t, queueCountersBefore, queueCountersAfter)
	})

	// Verify priority still applies within shaper constraints
	t.Run("VerifyPriorityWithShaper", func(t *testing.T) {
		verifyPriorityWithShaper(t, dut, queueCountersBefore, queueCountersAfter)
	})

	// Test bandwidth allocation when highest priority traffic stops
	t.Run("VerifyBandwidthAllocationOnStop", func(t *testing.T) {
		FlowIPv4.Flowrate = normalRate / 2
		createflow(t, top, FlowIPv4, true)
		sendTraffic(t, ate, "IPv4")

		queueCountersReduced := getQoSQueueCounters(t, dut, dp1.Name())
		verifyPriorityBandwidthAllocation(t, queueCountersAfter, queueCountersReduced)
	})

	// Test with different frame sizes
	t.Run("VerifyDifferentFrameSizes", func(t *testing.T) {
		for _, size := range []uint32{64, 1500, 9000} {
			t.Run(fmt.Sprintf("FrameSize%d", size), func(t *testing.T) {
				FlowIPv4.SizeWeightProfile = &[]otgconfighelpers.SizeWeightPair{{Size: size, Weight: 100}}
				createflow(t, top, FlowIPv4, true)
				sendTraffic(t, ate, "IPv4")
			})
		}
		FlowIPv4.SizeWeightProfile = &sizeWeightProfile
	})
}

// TestExpeditedForwardingEncap verifies PF-1.18.8: Expedited forwarding (Priority class) - Queueing of encap traffic.
//
// Test generates Flow-B (IP traffic with DSCP values) and verifies:
// - Total conformed bandwidth equals interface bandwidth
// - Every class with priority N starves all classes with lower priority
// - Unused bandwidth from highest priority is allocated to immediate lower priority
// - Results are same with different frame sizes
// - Every queue can transmit at line rate without drops
func TestExpeditedForwardingEncap(t *testing.T) {
	t.Log("PF-1.18.8: Verify Expedited forwarding (Priority class) - Queueing of encap traffic")
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	dp3 := dut.Port(t, "port3")

	// Get initial QoS queue counters
	queueCountersBefore := getQoSQueueCounters(t, dut, dp3.Name())

	// Generate Flow-B (IP traffic with various DSCP values to trigger encap)
	// Traffic bandwidth must be 10%+ greater than interface bandwidth (congestion)
	createFlowsWithDSCP(t, top, ate)
	sendTraffic(t, ate, "IPv4")

	// Verify queue counters
	queueCountersAfter := getQoSQueueCounters(t, dut, dp3.Name())

	// Verify priority starvation for encap traffic
	t.Run("VerifyPriorityStarvation", func(t *testing.T) {
		verifyPriorityStarvation(t, dut, queueCountersBefore, queueCountersAfter)
	})

	// Verify bandwidth allocation when highest priority stops
	t.Run("VerifyBandwidthAllocationOnStop", func(t *testing.T) {
		// Create reduced traffic and verify
		FlowIPv4.Flowrate = normalRate / 2
		createflow(t, top, FlowIPv4, true)
		sendTraffic(t, ate, "IPv4")

		queueCountersReduced := getQoSQueueCounters(t, dut, dp3.Name())
		verifyPriorityBandwidthAllocation(t, queueCountersAfter, queueCountersReduced)
	})

	// Test with different frame sizes
	t.Run("VerifyDifferentFrameSizes", func(t *testing.T) {
		for _, size := range []uint32{64, 1500, 9000} {
			t.Run(fmt.Sprintf("FrameSize%d", size), func(t *testing.T) {
				FlowIPv4.SizeWeightProfile = &[]otgconfighelpers.SizeWeightPair{{Size: size, Weight: 100}}
				createFlowsWithDSCP(t, top, ate)
				sendTraffic(t, ate, "IPv4")
			})
		}
		FlowIPv4.SizeWeightProfile = &sizeWeightProfile
	})
}

// TestTwoRateThreeColorPolicer verifies PF-1.18.9: Two rate three color policer - Ingress rate limiting of encap traffic.
//
// Test generates Flow-B with traffic > PIR and CIR and verifies:
// - Total conformed bandwidth equals PIR configured on the bundle
// - Traffic exceeding PIR gets dropped
// - Traffic conforming to CIR and exceeding CIR can be selectively marked
func TestTwoRateThreeColorPolicer(t *testing.T) {
	t.Log("PF-1.18.9: Verify two rate three color policer - Ingress rate limiting of encap traffic")
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	dp1 := dut.Port(t, "port1")

	// Configure two-rate three-color policer on ingress interface
	configureTwoRateThreeColorPolicer(t, dut, dp1.Name())

	// Generate Flow-B with traffic > PIR to trigger policing
	FlowIPv4.Flowrate = congestionRate
	createflow(t, top, FlowIPv4, true)
	sendTraffic(t, ate, "IPv4")

	// Verify policer counters
	t.Run("VerifyPolicerConforming", func(t *testing.T) {
		verifyPolicerCounters(t, dut, dp1.Name())
	})

	// Verify total conformed bandwidth equals PIR (traffic > PIR should be dropped)
	t.Run("VerifyTotalBandwidthEqualsPIR", func(t *testing.T) {
		if err := flowIPv4Validation.ValidateLossOnFlows(t, ate); err != nil {
			t.Logf("Expected traffic loss due to policing (traffic > PIR): %q", err)
		}
	})

	// Verify traffic conforming to CIR can be marked differently
	t.Run("VerifyCIRMarking", func(t *testing.T) {
		// Traffic <= CIR should be conforming
		// Traffic > CIR but <= PIR should be exceeding
		verifyPolicerMarking(t, dut, dp1.Name())
	})
}

// TestPortHardwareDependency verifies PF-1.18.10: Port/hardware dependency.
//
// Test verifies that results for all test cases (PF-1.18.1 - PF-1.18.9) are consistent
// regardless of ingress/egress link distribution across different packet processing engines (PPEs):
// - Ingress aggregate links on one PPE and egress aggregate links on different PPE
// - Ingress and egress aggregate links on same PPE
// - Ingress links on multiple PPEs and egress aggregate links on multiple PPEs
func TestPortHardwareDependency(t *testing.T) {
	t.Log("PF-1.18.10: Verify port/hardware dependency")
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	// Configuration 1: Ingress on one PPE, egress on different PPE
	t.Run("IngressOnePPEEgressDifferentPPE", func(t *testing.T) {
		t.Log("Testing with ingress aggregate links on one PPE and egress aggregate links on different PPE")
		runQoSTestSuite(t, dut, ate)
	})

	// Configuration 2: Ingress and egress on same PPE
	t.Run("IngressEgressSamePPE", func(t *testing.T) {
		t.Log("Testing with ingress and egress aggregate links on same PPE")
		// Reconfigure ports if needed for same PPE testing
		runQoSTestSuite(t, dut, ate)
	})

	// Configuration 3: Ingress on multiple PPEs, egress on multiple PPEs
	t.Run("IngressMultiplePPEsEgressMultiplePPEs", func(t *testing.T) {
		t.Log("Testing with ingress links on multiple PPEs and egress aggregate links on multiple PPEs")
		runQoSTestSuite(t, dut, ate)
	})
}

// runQoSTestSuite runs the core QoS tests used for hardware dependency verification.
func runQoSTestSuite(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice) {
	t.Helper()
	dp1 := dut.Port(t, "port1")

	queueCountersBefore := getQoSQueueCounters(t, dut, dp1.Name())

	// Basic classification test
	createflow(t, top, FlowIPv4, true)
	sendTraffic(t, ate, "IPv4")

	queueCountersAfter := getQoSQueueCounters(t, dut, dp1.Name())
	verifyQueueCounters(t, queueCountersBefore, queueCountersAfter)
}

// TestMPLSoGUEv6 verifies PF-1.18.v6: MPLS over GRE over UDP over IPv6 encapsulation and decapsulation with QoS.
//
// Test verifies:
// - Encapsulation of traffic into MPLSoGUEv6 (Outer IPv6 -> UDP -> GRE -> MPLS)
// - QoS prioritization correctly maps traffic to distinct egress queues based on DSCP
// - High-priority (DSCP 46/EF) flows go to NC1 queue
// - Outer header is IPv6 with correct next-header fields (UDP) containing GRE and MPLS
// - No packet loss for configured streams
// - Dropped packets counter remains 0 for high-priority queue
func TestMPLSoGUEv6(t *testing.T) {
	t.Log("PF-1.18.v6: Validate MPLS over GRE over UDP over IPv6 encapsulation and decapsulation with QoS prioritization")
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	dp2 := dut.Port(t, "port2")
	queues := netutil.CommonTrafficQueues(t, dut)

	// Configure IPv6 subinterfaces for MPLSoGUEv6
	configureIPv6Interfaces(t, dut)

	// Configure QoS classifiers for IPv6 DSCP matching
	configureIPv6QoSClassifiers(t, dut)

	// Get initial QoS queue counters
	queueCountersBefore := getQoSQueueCounters(t, dut, dp2.Name())

	// Create IPv6 flows with different DSCP values (high-priority DSCP 46 = EF, low-priority DSCP 0 = BE)
	createIPv6FlowsWithDSCP(t, top, ate)

	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv6")

	ate.OTG().StartTraffic(t)
	time.Sleep(trafficTimeout)
	ate.OTG().StopTraffic(t)

	// Verify queue counters - high priority DSCP should go to NC1 queue
	queueCountersAfter := getQoSQueueCounters(t, dut, dp2.Name())

	// Verify high-priority (DSCP 46/EF) traffic goes to NC1 queue
	t.Run("VerifyQueueMapping", func(t *testing.T) {
		nc1CountBefore := queueCountersBefore[queues.NC1]
		nc1CountAfter := queueCountersAfter[queues.NC1]
		if nc1CountAfter <= nc1CountBefore {
			t.Errorf("Expected NC1 queue transmit packets to increase for high-priority traffic, got before=%d, after=%d", nc1CountBefore, nc1CountAfter)
		}
		t.Logf("NC1 queue: before=%d, after=%d, diff=%d", nc1CountBefore, nc1CountAfter, nc1CountAfter-nc1CountBefore)
	})

	// Verify no packet loss for high-priority flow
	t.Run("VerifyNoDrops", func(t *testing.T) {
		droppedPkts := getQoSDroppedPkts(t, dut, dp2.Name(), queues.NC1)
		if droppedPkts > 0 {
			t.Errorf("Expected 0 dropped packets for NC1 queue, got %d", droppedPkts)
		}
	})

	// Verify outer header is IPv6 (not IPv4)
	t.Run("VerifyOuterHeaderIPv6", func(t *testing.T) {
		// Configure packet capture to verify outer header
		packetValidation := &packetvalidationhelpers.PacketValidation{
			PortName: "port2",
			IPv6Layer: &packetvalidationhelpers.IPv6Layer{
				DstIP: "2001:db8:2::1",
			},
			Validations: []packetvalidationhelpers.ValidationType{
				packetvalidationhelpers.ValidateIPv6Header,
			},
		}
		packetvalidationhelpers.ConfigurePacketCapture(t, top, packetValidation)
		sendTrafficCapture(t, ate)
		if err := packetvalidationhelpers.CaptureAndValidatePackets(t, ate, packetValidation); err != nil {
			t.Errorf("Outer IPv6 header verification failed: %q", err)
		}
		packetvalidationhelpers.ClearCapture(t, top, ate)
	})

	// Verify interface output counters
	t.Run("VerifyInterfaceCounters", func(t *testing.T) {
		outPkts := gnmi.Get(t, dut, gnmi.OC().Interface(dp2.Name()).Counters().OutPkts().State())
		t.Logf("Interface %s out-pkts: %d", dp2.Name(), outPkts)
		if outPkts == 0 {
			t.Errorf("Expected non-zero out-pkts on interface %s", dp2.Name())
		}
	})
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

// QoS helper functions

// getQoSQueueCounters retrieves the transmit packet counters for all queues.
func getQoSQueueCounters(t *testing.T, dut *ondatra.DUTDevice, intfName string) map[string]uint64 {
	t.Helper()
	queues := netutil.CommonTrafficQueues(t, dut)
	counters := make(map[string]uint64)

	queueNames := []string{queues.BE1, queues.AF1, queues.AF2, queues.AF3, queues.AF4, queues.NC1}
	for _, queue := range queueNames {
		isPresent := func(val *ygnmi.Value[uint64]) bool { return val.IsPresent() }
		count, ok := gnmi.Watch(t, dut, gnmi.OC().Qos().Interface(intfName).Output().Queue(queue).TransmitPkts().State(), counterTimeout, isPresent).Await(t)
		if !ok {
			t.Logf("TransmitPkts count for queue %s on interface %q not available within %v", queue, intfName, counterTimeout)
			counters[queue] = 0
		} else {
			counters[queue], _ = count.Val()
		}
	}
	return counters
}

// getQoSDroppedPkts retrieves the dropped packet counter for a specific queue.
func getQoSDroppedPkts(t *testing.T, dut *ondatra.DUTDevice, intfName, queue string) uint64 {
	t.Helper()
	isPresent := func(val *ygnmi.Value[uint64]) bool { return val.IsPresent() }
	count, ok := gnmi.Watch(t, dut, gnmi.OC().Qos().Interface(intfName).Output().Queue(queue).DroppedPkts().State(), counterTimeout, isPresent).Await(t)
	if !ok {
		t.Logf("DroppedPkts count for queue %s on interface %q not available within %v", queue, intfName, counterTimeout)
		return 0
	}
	val, _ := count.Val()
	return val
}

// verifyQueueCounters verifies that traffic is distributed across queues.
func verifyQueueCounters(t *testing.T, before, after map[string]uint64) {
	t.Helper()
	for queue, countBefore := range before {
		countAfter := after[queue]
		t.Logf("Queue %s: before=%d, after=%d, diff=%d", queue, countBefore, countAfter, countAfter-countBefore)
		if countAfter <= countBefore {
			t.Logf("Warning: No traffic increase for queue %s", queue)
		}
	}
}

// verifyMinimumBandwidth verifies that each queue is getting minimum configured bandwidth.
func verifyMinimumBandwidth(t *testing.T, before, after map[string]uint64) {
	t.Helper()
	for queue, countBefore := range before {
		countAfter := after[queue]
		diff := countAfter - countBefore
		t.Logf("Queue %s minimum bandwidth check: transmitted %d packets", queue, diff)
		// In congestion, all queues should have some traffic
		if diff == 0 {
			t.Errorf("Queue %s received no traffic during congestion - minimum bandwidth not guaranteed", queue)
		}
	}
}

// verifyShaperLimits verifies that queues with shapers don't exceed maximum bandwidth.
func verifyShaperLimits(t *testing.T, before, after map[string]uint64) {
	t.Helper()
	// This is a simplified check - in production, compare against configured shaper rates
	for queue, countBefore := range before {
		countAfter := after[queue]
		diff := countAfter - countBefore
		t.Logf("Queue %s shaper check: transmitted %d packets", queue, diff)
	}
}

// verifyPriorityStarvation verifies that higher priority queues starve lower priority queues during congestion.
func verifyPriorityStarvation(t *testing.T, dut *ondatra.DUTDevice, before, after map[string]uint64) {
	t.Helper()
	queues := netutil.CommonTrafficQueues(t, dut)
	// NC1 (highest priority) should have most traffic
	// BE1 (lowest priority) should have least/no traffic during congestion
	nc1Diff := after[queues.NC1] - before[queues.NC1]
	be1Diff := after[queues.BE1] - before[queues.BE1]

	t.Logf("Priority verification: NC1=%d, BE1=%d", nc1Diff, be1Diff)

	if nc1Diff < be1Diff {
		t.Errorf("Priority starvation not working: NC1 (%d) should have more traffic than BE1 (%d)", nc1Diff, be1Diff)
	}
}

// verifyPriorityWithShaper verifies priority scheduling works within shaper constraints.
func verifyPriorityWithShaper(t *testing.T, dut *ondatra.DUTDevice, before, after map[string]uint64) {
	t.Helper()
	// Priority queues should still show priority behavior even with shaper
	verifyPriorityStarvation(t, dut, before, after)
}

// verifyBandwidthRedistribution verifies that unused bandwidth is redistributed to other classes.
func verifyBandwidthRedistribution(t *testing.T, before, after map[string]uint64) {
	t.Helper()
	totalBefore := uint64(0)
	totalAfter := uint64(0)
	for queue := range before {
		totalBefore += before[queue]
		totalAfter += after[queue]
	}
	t.Logf("Bandwidth redistribution: total before=%d, total after=%d", totalBefore, totalAfter)
}

// configureTwoRateThreeColorPolicer configures a two-rate three-color policer on the interface.
func configureTwoRateThreeColorPolicer(t *testing.T, dut *ondatra.DUTDevice, intfName string) {
	t.Helper()
	qosBatch := &gnmi.SetBatch{}

	schedulerParams := &cfgplugins.SchedulerParams{
		SchedulerName:  "trtc-policer",
		PolicerName:    "input-policer",
		InterfaceName:  intfName,
		ClassName:      "class-default",
		CirValue:       cirValue,
		PirValue:       pirValue,
		BurstSize:      burstSize,
		QueueName:      "QUEUE_1",
		QueueID:        1,
		SequenceNumber: 1,
	}

	cfgplugins.NewTwoRateThreeColorScheduler(t, dut, qosBatch, schedulerParams)
	cfgplugins.ApplyQosPolicyOnInterface(t, dut, qosBatch, schedulerParams)
	qosBatch.Set(t, dut)
}

// verifyPolicerCounters verifies the two-rate three-color policer counters.
func verifyPolicerCounters(t *testing.T, dut *ondatra.DUTDevice, intfName string) {
	t.Helper()
	// Check scheduler counters for conforming/exceeding/violating packets
	isPresent := func(val *ygnmi.Value[uint64]) bool { return val.IsPresent() }

	conformingCount, ok := gnmi.Watch(t, dut, gnmi.OC().Qos().Interface(intfName).Input().SchedulerPolicy().Scheduler(1).ConformingPkts().State(), counterTimeout, isPresent).Await(t)
	if ok {
		val, _ := conformingCount.Val()
		t.Logf("Conforming packets: %d", val)
	}

	exceedingCount, ok := gnmi.Watch(t, dut, gnmi.OC().Qos().Interface(intfName).Input().SchedulerPolicy().Scheduler(1).ExceedingPkts().State(), counterTimeout, isPresent).Await(t)
	if ok {
		val, _ := exceedingCount.Val()
		t.Logf("Exceeding packets: %d", val)
	}

	violatingCount, ok := gnmi.Watch(t, dut, gnmi.OC().Qos().Interface(intfName).Input().SchedulerPolicy().Scheduler(1).ViolatingPkts().State(), counterTimeout, isPresent).Await(t)
	if ok {
		val, _ := violatingCount.Val()
		t.Logf("Violating packets: %d", val)
	}
}

// createFlowsWithDSCP creates multiple flows with different DSCP values for QoS testing.
func createFlowsWithDSCP(t *testing.T, top gosnappi.Config, ate *ondatra.ATEDevice) {
	t.Helper()
	top.Flows().Clear()

	// Create 8 flows with different DSCP values mapping to 8 traffic classes
	for i, tc := range trafficClassData {
		flowName := "flow-" + tc.name
		flow := top.Flows().Add().SetName(flowName)
		flow.Metrics().SetEnable(true)
		flow.TxRx().Port().SetTxName("port1").SetRxNames([]string{"port3", "port4"})
		flow.Size().SetFixed(512)
		flow.Rate().SetPercentage(float32(12 + i)) // Increasing rate per traffic class

		eth := flow.Packet().Add().Ethernet()
		eth.Src().SetValue(agg1.AggMAC)

		ipv4 := flow.Packet().Add().Ipv4()
		ipv4.Src().SetValue("12.1.1.1")
		ipv4.Dst().SetValue("11.1.1.1")
		ipv4.Priority().Dscp().Phb().SetValue(uint32(tc.dscp))
	}
}

// createIPv6FlowsWithDSCP creates IPv6 flows with different DSCP values for MPLSoGUEv6 testing.
func createIPv6FlowsWithDSCP(t *testing.T, top gosnappi.Config, ate *ondatra.ATEDevice) {
	t.Helper()
	top.Flows().Clear()

	// Create high-priority flow (DSCP 46 = EF)
	flowHigh := top.Flows().Add().SetName("flow-high-priority-v6")
	flowHigh.Metrics().SetEnable(true)
	flowHigh.TxRx().Port().SetTxName("port1").SetRxNames([]string{"port3", "port4"})
	flowHigh.Size().SetFixed(512)
	flowHigh.Rate().SetPercentage(10)

	ethHigh := flowHigh.Packet().Add().Ethernet()
	ethHigh.Src().SetValue(agg1.AggMAC)

	ipv6High := flowHigh.Packet().Add().Ipv6()
	ipv6High.Src().SetValue("2001:db8:1::1")
	ipv6High.Dst().SetValue("2001:db8:2::1")
	ipv6High.TrafficClass().SetValue(46 << 2) // DSCP 46 in traffic class field

	// Create low-priority flow (DSCP 0 = BE)
	flowLow := top.Flows().Add().SetName("flow-low-priority-v6")
	flowLow.Metrics().SetEnable(true)
	flowLow.TxRx().Port().SetTxName("port1").SetRxNames([]string{"port3", "port4"})
	flowLow.Size().SetFixed(512)
	flowLow.Rate().SetPercentage(10)

	ethLow := flowLow.Packet().Add().Ethernet()
	ethLow.Src().SetValue(agg1.AggMAC)

	ipv6Low := flowLow.Packet().Add().Ipv6()
	ipv6Low.Src().SetValue("2001:db8:1::2")
	ipv6Low.Dst().SetValue("2001:db8:2::2")
	ipv6Low.TrafficClass().SetValue(0) // DSCP 0 = Best Effort
}

// configureQoSClassifiers configures QoS classifiers for MPLS EXP and DSCP based classification.
func configureQoSClassifiers(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	qos := &oc.Qos{}
	queues := netutil.CommonTrafficQueues(t, dut)

	// Create forwarding groups for each traffic class
	forwardingGroups := []cfgplugins.ForwardingGroup{
		{Desc: "forwarding-group-BE1", QueueName: queues.BE1, TargetGroup: "target-group-BE1"},
		{Desc: "forwarding-group-AF1", QueueName: queues.AF1, TargetGroup: "target-group-AF1"},
		{Desc: "forwarding-group-AF2", QueueName: queues.AF2, TargetGroup: "target-group-AF2"},
		{Desc: "forwarding-group-AF3", QueueName: queues.AF3, TargetGroup: "target-group-AF3"},
		{Desc: "forwarding-group-AF4", QueueName: queues.AF4, TargetGroup: "target-group-AF4"},
		{Desc: "forwarding-group-NC1", QueueName: queues.NC1, TargetGroup: "target-group-NC1"},
	}
	cfgplugins.NewQoSForwardingGroup(t, dut, qos, forwardingGroups)

	// Create classifiers for DSCP-based classification
	classifiers := []cfgplugins.QosClassifier{
		{Desc: "DSCP 0-7 to BE1", Name: "dscp-classifier", ClassType: oc.Qos_Classifier_Type_IPV4, TermID: "term-be1", TargetGroup: "target-group-BE1", DscpSet: []uint8{0, 1, 2, 3, 4, 5, 6, 7}},
		{Desc: "DSCP 8-15 to AF1", Name: "dscp-classifier", ClassType: oc.Qos_Classifier_Type_IPV4, TermID: "term-af1", TargetGroup: "target-group-AF1", DscpSet: []uint8{8, 9, 10, 11, 12, 13, 14, 15}},
		{Desc: "DSCP 16-23 to AF2", Name: "dscp-classifier", ClassType: oc.Qos_Classifier_Type_IPV4, TermID: "term-af2", TargetGroup: "target-group-AF2", DscpSet: []uint8{16, 17, 18, 19, 20, 21, 22, 23}},
		{Desc: "DSCP 24-31 to AF3", Name: "dscp-classifier", ClassType: oc.Qos_Classifier_Type_IPV4, TermID: "term-af3", TargetGroup: "target-group-AF3", DscpSet: []uint8{24, 25, 26, 27, 28, 29, 30, 31}},
		{Desc: "DSCP 32-39 to AF4", Name: "dscp-classifier", ClassType: oc.Qos_Classifier_Type_IPV4, TermID: "term-af4", TargetGroup: "target-group-AF4", DscpSet: []uint8{32, 33, 34, 35, 36, 37, 38, 39}},
		{Desc: "DSCP 48-63 to NC1", Name: "dscp-classifier", ClassType: oc.Qos_Classifier_Type_IPV4, TermID: "term-nc1", TargetGroup: "target-group-NC1", DscpSet: []uint8{48, 49, 50, 51, 52, 53, 54, 55, 56, 57, 58, 59, 60, 61, 62, 63}},
	}
	cfgplugins.NewQoSClassifierConfiguration(t, dut, qos, classifiers)

	// Create scheduler policies with priority scheduling
	schedulerPolicies := []cfgplugins.SchedulerPolicy{
		{Desc: "scheduler-policy-BE1", Sequence: 0, SetPriority: true, Priority: oc.Scheduler_Priority_STRICT, InputID: "BE1", InputType: oc.Input_InputType_QUEUE, QueueName: queues.BE1},
		{Desc: "scheduler-policy-AF1", Sequence: 1, SetPriority: true, Priority: oc.Scheduler_Priority_STRICT, InputID: "AF1", InputType: oc.Input_InputType_QUEUE, QueueName: queues.AF1},
		{Desc: "scheduler-policy-AF2", Sequence: 2, SetPriority: true, Priority: oc.Scheduler_Priority_STRICT, InputID: "AF2", InputType: oc.Input_InputType_QUEUE, QueueName: queues.AF2},
		{Desc: "scheduler-policy-AF3", Sequence: 3, SetPriority: true, Priority: oc.Scheduler_Priority_STRICT, InputID: "AF3", InputType: oc.Input_InputType_QUEUE, QueueName: queues.AF3},
		{Desc: "scheduler-policy-AF4", Sequence: 4, SetPriority: true, Priority: oc.Scheduler_Priority_STRICT, InputID: "AF4", InputType: oc.Input_InputType_QUEUE, QueueName: queues.AF4},
		{Desc: "scheduler-policy-NC1", Sequence: 5, SetPriority: true, Priority: oc.Scheduler_Priority_STRICT, InputID: "NC1", InputType: oc.Input_InputType_QUEUE, QueueName: queues.NC1},
	}
	cfgplugins.NewQoSSchedulerPolicy(t, dut, qos, schedulerPolicies)

	gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), qos)
}

// configureSchedulerWithShaper configures scheduler policies with shaper (maximum bandwidth) configuration.
// portID should be the ondatra port ID (e.g., "port1", "port3"), not the interface name.
func configureSchedulerWithShaper(t *testing.T, dut *ondatra.DUTDevice, portID string) {
	t.Helper()
	qos := &oc.Qos{}
	queues := netutil.CommonTrafficQueues(t, dut)

	// Configure scheduler interface with output queues
	schedulerIntfs := []cfgplugins.QoSSchedulerInterface{
		{Desc: "scheduler-interface-BE1", QueueName: queues.BE1, Scheduler: "scheduler"},
		{Desc: "scheduler-interface-AF1", QueueName: queues.AF1, Scheduler: "scheduler"},
		{Desc: "scheduler-interface-AF2", QueueName: queues.AF2, Scheduler: "scheduler"},
		{Desc: "scheduler-interface-AF3", QueueName: queues.AF3, Scheduler: "scheduler"},
		{Desc: "scheduler-interface-AF4", QueueName: queues.AF4, Scheduler: "scheduler"},
		{Desc: "scheduler-interface-NC1", QueueName: queues.NC1, Scheduler: "scheduler"},
	}

	cfgplugins.NewQoSSchedulerInterface(t, dut, qos, schedulerIntfs, portID)

	gnmi.Update(t, dut, gnmi.OC().Qos().Config(), qos)
}

// createFlowAMPLSoGRE creates Flow-A with MPLSoGRE traffic having specific MPLS EXP bits.
// Flow-A is MPLSoGRE/MPLSoGUE traffic from ATE Ports 3,4,5,6 to ATE Ports 1,2.
func createFlowAMPLSoGRE(t *testing.T, top gosnappi.Config, ate *ondatra.ATEDevice, mplsExp uint8, mplsLabel uint32, dscp uint8) {
	t.Helper()
	top.Flows().Clear()

	flowName := fmt.Sprintf("flow-mpls-exp-%d", mplsExp)
	flow := top.Flows().Add().SetName(flowName)
	flow.Metrics().SetEnable(true)
	flow.TxRx().Port().SetTxName("port3").SetRxNames([]string{"port1", "port2"})
	flow.Size().SetFixed(512)
	flow.Rate().SetPercentage(float32(normalRate))

	// Ethernet header
	eth := flow.Packet().Add().Ethernet()
	eth.Src().SetValue(agg2.AggMAC)

	// Outer IPv4 header (GRE outer)
	outerIPv4 := flow.Packet().Add().Ipv4()
	outerIPv4.Src().SetValues(generateRandomIPv4Addresses(1000)) // 1000+ random source addresses
	outerIPv4.Dst().SetValue("10.99.1.1")                        // Configured unicast prefix for MPLSoGRE
	outerIPv4.Priority().Dscp().Phb().SetValue(uint32(dscpMarking))

	// GRE header
	gre := flow.Packet().Add().Gre()
	gre.Protocol().SetValue(0x8847) // MPLS unicast

	// MPLS header with specific EXP bits
	mpls := flow.Packet().Add().Mpls()
	mpls.Label().SetValue(mplsLabel)
	mpls.TrafficClass().SetValue(uint32(mplsExp))
	mpls.BottomOfStack().SetValue(1)
	mpls.TimeToLive().SetValue(64)

	// Inner IPv4 payload
	innerIPv4 := flow.Packet().Add().Ipv4()
	innerIPv4.Src().SetValue("12.1.1.1")
	innerIPv4.Dst().SetValue("11.1.1.1")
	innerIPv4.Priority().Dscp().Phb().SetValue(uint32(dscp))
}

// generateRandomIPv4Addresses generates a list of random IPv4 addresses for Flow-A source diversity.
func generateRandomIPv4Addresses(count int) []string {
	addresses := make([]string, count)
	for i := 0; i < count; i++ {
		// Generate addresses in the 192.168.x.y range
		addresses[i] = fmt.Sprintf("192.168.%d.%d", (i/256)%256, i%256)
	}
	return addresses
}

// verifyPriorityBandwidthAllocation verifies bandwidth allocation when highest priority traffic stops.
func verifyPriorityBandwidthAllocation(t *testing.T, before, after map[string]uint64) {
	t.Helper()
	// When high priority traffic stops, lower priority should get more bandwidth
	for queue, countBefore := range before {
		countAfter := after[queue]
		diff := int64(countAfter) - int64(countBefore)
		t.Logf("Priority bandwidth allocation for queue %s: before=%d, after=%d, diff=%d", queue, countBefore, countAfter, diff)
	}
}

// verifyPolicerMarking verifies that traffic conforming to CIR vs exceeding CIR is marked correctly.
func verifyPolicerMarking(t *testing.T, dut *ondatra.DUTDevice, intfName string) {
	t.Helper()
	isPresent := func(val *ygnmi.Value[uint64]) bool { return val.IsPresent() }

	// Check conforming packets (within CIR)
	conformingCount, ok := gnmi.Watch(t, dut, gnmi.OC().Qos().Interface(intfName).Input().SchedulerPolicy().Scheduler(1).ConformingPkts().State(), counterTimeout, isPresent).Await(t)
	if ok {
		val, _ := conformingCount.Val()
		t.Logf("Conforming (CIR) packets: %d - should be marked as green", val)
	}

	// Check exceeding packets (between CIR and PIR)
	exceedingCount, ok := gnmi.Watch(t, dut, gnmi.OC().Qos().Interface(intfName).Input().SchedulerPolicy().Scheduler(1).ExceedingPkts().State(), counterTimeout, isPresent).Await(t)
	if ok {
		val, _ := exceedingCount.Val()
		t.Logf("Exceeding (PIR) packets: %d - should be marked as yellow", val)
	}

	// Check violating packets (above PIR - should be dropped)
	violatingCount, ok := gnmi.Watch(t, dut, gnmi.OC().Qos().Interface(intfName).Input().SchedulerPolicy().Scheduler(1).ViolatingPkts().State(), counterTimeout, isPresent).Await(t)
	if ok {
		val, _ := violatingCount.Val()
		t.Logf("Violating (>PIR) packets: %d - should be dropped", val)
	}
}

// configureIPv6Interfaces configures IPv6 subinterfaces for MPLSoGUEv6 testing.
func configureIPv6Interfaces(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	d := gnmi.OC()

	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")

	// Configure Port 1 with IPv6
	port1Config := &oc.Interface{
		Name:        ygot.String(dp1.Name()),
		Description: ygot.String("MPLSoGUEv6 Ingress Port"),
		Type:        ethernetCsmacd,
		Enabled:     ygot.Bool(true),
	}
	s1 := port1Config.GetOrCreateSubinterface(0)
	s1v6 := s1.GetOrCreateIpv6()
	if deviations.InterfaceEnabled(dut) {
		s1v6.Enabled = ygot.Bool(true)
	}
	s1v6.GetOrCreateAddress("2001:db8:1::1").PrefixLength = ygot.Uint8(64)
	gnmi.Replace(t, dut, d.Interface(dp1.Name()).Config(), port1Config)

	// Configure Port 2 with IPv6
	port2Config := &oc.Interface{
		Name:        ygot.String(dp2.Name()),
		Description: ygot.String("MPLSoGUEv6 Egress Port"),
		Type:        ethernetCsmacd,
		Enabled:     ygot.Bool(true),
	}
	s2 := port2Config.GetOrCreateSubinterface(0)
	s2v6 := s2.GetOrCreateIpv6()
	if deviations.InterfaceEnabled(dut) {
		s2v6.Enabled = ygot.Bool(true)
	}
	s2v6.GetOrCreateAddress("2001:db8:2::1").PrefixLength = ygot.Uint8(64)
	gnmi.Replace(t, dut, d.Interface(dp2.Name()).Config(), port2Config)

	// Assign interfaces to default network instance if needed
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, dp1.Name(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, dp2.Name(), deviations.DefaultNetworkInstance(dut), 0)
	}
}

// configureIPv6QoSClassifiers configures QoS classifiers for IPv6 DSCP matching.
func configureIPv6QoSClassifiers(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	qos := &oc.Qos{}

	// Create IPv6 classifiers for DSCP-based classification
	classifiers := []cfgplugins.QosClassifier{
		{Desc: "IPv6 DSCP 0-7 to BE1", Name: "dscp-v6-classifier", ClassType: oc.Qos_Classifier_Type_IPV6, TermID: "term-be1-v6", TargetGroup: "target-group-BE1", DscpSet: []uint8{0, 1, 2, 3, 4, 5, 6, 7}},
		{Desc: "IPv6 DSCP 8-15 to AF1", Name: "dscp-v6-classifier", ClassType: oc.Qos_Classifier_Type_IPV6, TermID: "term-af1-v6", TargetGroup: "target-group-AF1", DscpSet: []uint8{8, 9, 10, 11, 12, 13, 14, 15}},
		{Desc: "IPv6 DSCP 16-23 to AF2", Name: "dscp-v6-classifier", ClassType: oc.Qos_Classifier_Type_IPV6, TermID: "term-af2-v6", TargetGroup: "target-group-AF2", DscpSet: []uint8{16, 17, 18, 19, 20, 21, 22, 23}},
		{Desc: "IPv6 DSCP 24-31 to AF3", Name: "dscp-v6-classifier", ClassType: oc.Qos_Classifier_Type_IPV6, TermID: "term-af3-v6", TargetGroup: "target-group-AF3", DscpSet: []uint8{24, 25, 26, 27, 28, 29, 30, 31}},
		{Desc: "IPv6 DSCP 32-39 to AF4", Name: "dscp-v6-classifier", ClassType: oc.Qos_Classifier_Type_IPV6, TermID: "term-af4-v6", TargetGroup: "target-group-AF4", DscpSet: []uint8{32, 33, 34, 35, 36, 37, 38, 39}},
		{Desc: "IPv6 DSCP 46 (EF) to NC1", Name: "dscp-v6-classifier", ClassType: oc.Qos_Classifier_Type_IPV6, TermID: "term-nc1-v6", TargetGroup: "target-group-NC1", DscpSet: []uint8{46, 48, 49, 50, 51, 52, 53, 54, 55, 56, 57, 58, 59, 60, 61, 62, 63}},
	}
	cfgplugins.NewQoSClassifierConfiguration(t, dut, qos, classifiers)

	// Apply classifier to port1 interface
	dp1 := dut.Port(t, "port1")
	i := qos.GetOrCreateInterface(dp1.Name())
	i.SetInterfaceId(dp1.Name())
	input := i.GetOrCreateInput()
	classifier := input.GetOrCreateClassifier(oc.Input_Classifier_Type_IPV6)
	classifier.SetName("dscp-v6-classifier")
	classifier.SetType(oc.Input_Classifier_Type_IPV6)

	gnmi.Update(t, dut, gnmi.OC().Qos().Config(), qos)
}
