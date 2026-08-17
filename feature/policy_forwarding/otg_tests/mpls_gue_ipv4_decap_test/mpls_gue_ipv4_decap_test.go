// Package mpls_gue_ipv4_decap_test tests mplsogue decap functionality.
package mpls_gue_ipv4_decap_test

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
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	otgtelemetry "github.com/openconfig/ondatra/gnmi/otg"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

// TestMain calls main function.
func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

type TestFlow struct {
	flowOuterConfig  *otgconfighelpers.Flow
	flowInnerConfig  *otgconfighelpers.Flow
	flowValidation   *otgvalidationhelpers.OTGValidation
	packetValidation *packetvalidationhelpers.PacketValidation
}

const (
	ethernetCsmacd       = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	ieee8023adLag        = oc.IETFInterfaces_InterfaceType_ieee8023adLag
	udpPort              = 6635
	policyName           = "customer1"
	gueV6DecapPolicyName = "gue-v6-decap-policy"
	gueV6DecapRuleSeqID  = uint32(10)
	multicastMPLSLabel   = uint32(99995)
	pps                  = uint64(1000)
	totalPackets         = uint32(10000)
	trafficTimeout       = 2 * time.Minute
)

var (
	top       = gosnappi.NewConfig()
	aggID     string
	aggID2    string
	custAggID string
	custPorts = []string{"port1", "port2"}
	corePorts = []string{"port3", "port4"}
	// corePorts2 are the member ports of Aggregate3, the 2nd ingress aggregate.
	corePorts2   = []string{"port5", "port6"}
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
	// custIntfIPv4Global2 is the 2nd of the two required IPv4-only /30 global VLANs.
	custIntfIPv4Global2 = attrs.Attributes{
		Desc:         "Customer_connect_multicloud2",
		MTU:          1500,
		IPv4:         "169.254.0.37",
		IPv4Len:      30,
		Subinterface: 24,
	}
	// custIntfIPv6B is the 2nd of the two required IPv6-only /125 VLANs.
	custIntfIPv6B = attrs.Attributes{
		Desc:         "Customer_connectv6_2",
		MTU:          1500,
		IPv6:         "2600:2d00:0:1:9000:10:0:ca31",
		IPv6Sec:      "2600:2d00:0:1:9000:10:0:ca33",
		IPv6Len:      125,
		Subinterface: 25,
	}
	custIntfIPv4JumboMTU = attrs.Attributes{
		Desc:         "Customer_connect",
		MTU:          9080,
		IPv4:         "169.254.0.53",
		IPv4Len:      29,
		Subinterface: 26,
	}
	// custIntfdualStack2/3/4 are the remaining 3 of the 4 required dual-stack VLANs.
	custIntfdualStack2 = attrs.Attributes{
		Desc:         "Customer_connect_dualstack2",
		MTU:          1500,
		IPv4:         "169.254.0.41",
		IPv4Len:      29,
		IPv6:         "2600:2d00:0:1:a000:10:0:ca31",
		IPv6Sec:      "2600:2d00:0:1:a000:10:0:ca33",
		IPv6Len:      125,
		Subinterface: 27,
	}
	custIntfdualStack3 = attrs.Attributes{
		Desc: "Customer_connect_dualstack3",
		MTU:  1500,
		// 169.254.0.56/29 (distinct from VLAN27's 169.254.0.40/29 block).
		IPv4:         "169.254.0.57",
		IPv4Len:      29,
		IPv6:         "2600:2d00:0:1:b000:10:0:ca31",
		IPv6Sec:      "2600:2d00:0:1:b000:10:0:ca33",
		IPv6Len:      125,
		Subinterface: 28,
	}
	custIntfdualStack4 = attrs.Attributes{
		Desc: "Customer_connect_dualstack4",
		MTU:  1500,
		// 169.254.0.64/29 (distinct from VLAN26's 169.254.0.48/29 block).
		IPv4:         "169.254.0.65",
		IPv4Len:      29,
		IPv6:         "2600:2d00:0:1:c000:10:0:ca31",
		IPv6Sec:      "2600:2d00:0:1:c000:10:0:ca33",
		IPv6Len:      125,
		Subinterface: 29,
	}
	coreIntf = attrs.Attributes{
		Desc:    "Core_Interface",
		IPv4:    "194.0.2.1",
		IPv6:    "2001:10:1:6::1",
		MTU:     9216,
		IPv4Len: 24,
		IPv6Len: 126,
	}
	// coreIntf2 is the DUT-side interface for Aggregate3, the 2nd ingress aggregate.
	coreIntf2 = attrs.Attributes{
		Desc:    "Core_Interface2",
		IPv4:    "194.0.3.1",
		IPv6:    "2001:10:1:7::1",
		MTU:     9216,
		IPv4Len: 24,
		IPv6Len: 126,
	}

	agg1 = &otgconfighelpers.Port{
		Name:        "Port-Channel1",
		AggMAC:      "02:00:01:01:01:07",
		Interfaces:  []*otgconfighelpers.InterfaceProperties{interface1, interface2, interface3, interface4, interface8, interface10, interface11, interface12, interface13, interface14},
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
	// interface10 pairs with custIntfIPv4Global2 (2nd IPv4-only /30 global VLAN).
	interface10 = &otgconfighelpers.InterfaceProperties{
		IPv4:        "169.254.0.38",
		IPv4Gateway: "169.254.0.37",
		Name:        "Port-Channel1.24",
		MAC:         "02:00:01:01:01:14",
		Vlan:        24,
		IPv4Len:     30,
	}
	// interface11 pairs with custIntfIPv6B (2nd IPv6-only /125 VLAN).
	interface11 = &otgconfighelpers.InterfaceProperties{
		IPv6:        "2600:2d00:0:1:9000:10:0:ca32",
		IPv6Gateway: "2600:2d00:0:1:9000:10:0:ca31",
		Name:        "Port-Channel1.25",
		MAC:         "02:00:01:01:01:15",
		Vlan:        25,
		IPv6Len:     125,
	}
	// interface12 pairs with custIntfdualStack2.
	interface12 = &otgconfighelpers.InterfaceProperties{
		IPv4:        "169.254.0.42",
		IPv4Gateway: "169.254.0.41",
		IPv6:        "2600:2d00:0:1:a000:10:0:ca32",
		IPv6Gateway: "2600:2d00:0:1:a000:10:0:ca31",
		Name:        "Port-Channel1.27",
		MAC:         "02:00:01:01:01:16",
		Vlan:        27,
		IPv4Len:     29,
		IPv6Len:     125,
	}
	// interface13 pairs with custIntfdualStack3.
	interface13 = &otgconfighelpers.InterfaceProperties{
		IPv4:        "169.254.0.58",
		IPv4Gateway: "169.254.0.57",
		IPv6:        "2600:2d00:0:1:b000:10:0:ca32",
		IPv6Gateway: "2600:2d00:0:1:b000:10:0:ca31",
		Name:        "Port-Channel1.28",
		MAC:         "02:00:01:01:01:17",
		Vlan:        28,
		IPv4Len:     29,
		IPv6Len:     125,
	}
	// interface14 pairs with custIntfdualStack4.
	interface14 = &otgconfighelpers.InterfaceProperties{
		IPv4:        "169.254.0.66",
		IPv4Gateway: "169.254.0.65",
		IPv6:        "2600:2d00:0:1:c000:10:0:ca32",
		IPv6Gateway: "2600:2d00:0:1:c000:10:0:ca31",
		Name:        "Port-Channel1.29",
		MAC:         "02:00:01:01:01:18",
		Vlan:        29,
		IPv4Len:     29,
		IPv6Len:     125,
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
	// interface9 pairs with coreIntf2, the 2nd ingress aggregate (Aggregate3).
	interface9 = &otgconfighelpers.InterfaceProperties{
		IPv4:        "194.0.3.2",
		IPv6:        "2001:10:1:7::2",
		IPv4Gateway: "194.0.3.1",
		IPv6Gateway: "2001:10:1:7::1",
		Name:        "Port-Channel3",
		MAC:         "02:00:01:01:01:05",
		IPv4Len:     29,
		IPv6Len:     126,
	}
	// agg3 is the 2nd ingress aggregate (Aggregate3), sourced from ATE ports 5,6.
	agg3 = &otgconfighelpers.Port{
		Name:        "Port-Channel3",
		AggMAC:      "02:00:01:01:01:06",
		MemberPorts: []string{"port5", "port6"},
		Interfaces:  []*otgconfighelpers.InterfaceProperties{interface9},
		LagID:       3,
		IsLag:       true,
	}
	// Custom IMIX settings for all flows.
	sizeWeightProfile = []otgconfighelpers.SizeWeightPair{
		{Size: 256, Weight: 20},
		{Size: 512, Weight: 30},
		{Size: 1024, Weight: 30},
		{Size: 1500, Weight: 20},
	}

	flowResolveArp = &otgvalidationhelpers.OTGValidation{
		Interface: &otgvalidationhelpers.InterfaceParams{Names: []string{agg2.Name, agg3.Name}},
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
		FlowName:          "MPLSOGUETrafficIPv4InterfaceIPv4Payload",
		EthFlow:           &otgconfighelpers.EthFlowParams{SrcMAC: agg2.AggMAC},
		IPv4Flow:          &otgconfighelpers.IPv4FlowParams{IPv4Src: "100.64.0.1", IPv4Dst: "11.1.1.1", IPv4SrcCount: 1024},
		MPLSFlow:          &otgconfighelpers.MPLSFlowParams{MPLSLabel: 99991, MPLSExp: 7},
		UDPFlow:           &otgconfighelpers.UDPFlowParams{UDPSrcPort: 49152, UDPDstPort: udpPort},
	}
	// FlowOuterIPv4Validation MPLSOGUE traffic IPv4 interface IPv4 Payload.
	FlowOuterIPv4Validation = &otgvalidationhelpers.OTGValidation{
		Interface: &otgvalidationhelpers.InterfaceParams{Names: []string{agg1.Interfaces[0].Name}, Ports: agg1.MemberPorts},
		Flow:      &otgvalidationhelpers.FlowParams{Name: FlowOuterIPv4.FlowName, TolerancePct: 0.5},
	}
	// FlowInnerIPv4 Inner Header IPv4 Payload.
	FlowInnerIPv4 = &otgconfighelpers.Flow{
		IPv4Flow: &otgconfighelpers.IPv4FlowParams{IPv4Src: "22.1.1.1", IPv4Dst: "21.1.1.1", IPv4SrcCount: 1024, RawPriority: 0, RawPriorityCount: 255},
		TCPFlow:  &otgconfighelpers.TCPFlowParams{TCPSrcPort: 49152, TCPDstPort: 80, TCPSrcCount: 1024},
	}
	// FlowInnerIPv4Multicast Inner Header IPv4 multicast payload, per PF-1.19.4.
	FlowInnerIPv4Multicast = &otgconfighelpers.Flow{
		IPv4Flow: &otgconfighelpers.IPv4FlowParams{IPv4Src: "22.1.1.1", IPv4Dst: "232.1.1.1", IPv4SrcCount: 1024, RawPriority: 0, RawPriorityCount: 255},
		TCPFlow:  &otgconfighelpers.TCPFlowParams{TCPSrcPort: 49152, TCPDstPort: 80, TCPSrcCount: 1024},
	}
	// FlowOuterIPv6 Decap IPv6 Interface IPv6 Payload traffic params Outer Header.
	FlowOuterIPv6 = &otgconfighelpers.Flow{
		TxNames:           []string{agg2.Name + ".IPv4"},
		RxNames:           []string{agg1.Interfaces[1].Name + ".IPv6"},
		SizeWeightProfile: &sizeWeightProfile,
		FlowName:          "MPLSOGUETrafficIPv6InterfaceIPv6Payload",
		EthFlow:           &otgconfighelpers.EthFlowParams{SrcMAC: agg2.AggMAC},
		IPv4Flow:          &otgconfighelpers.IPv4FlowParams{IPv4Src: "100.64.0.1", IPv4Dst: "11.1.1.1", IPv4SrcCount: 1024},
		MPLSFlow:          &otgconfighelpers.MPLSFlowParams{MPLSLabel: 99992, MPLSExp: 7},
		UDPFlow:           &otgconfighelpers.UDPFlowParams{UDPSrcPort: 49152, UDPDstPort: udpPort},
	}
	// FlowOuterIPv6Validation MPLSOGUE traffic IPv6 interface IPv6 Payload.
	FlowOuterIPv6Validation = &otgvalidationhelpers.OTGValidation{
		Interface: &otgvalidationhelpers.InterfaceParams{Names: []string{agg1.Interfaces[1].Name}, Ports: agg1.MemberPorts},
		Flow:      &otgvalidationhelpers.FlowParams{Name: FlowOuterIPv6.FlowName, TolerancePct: 0.5},
	}
	// FlowInnerIPv6 Inner Header IPv6 Payload.
	FlowInnerIPv6 = &otgconfighelpers.Flow{
		IPv6Flow: &otgconfighelpers.IPv6FlowParams{IPv6Src: "2000:1::1", IPv6Dst: "3000:1::1", IPv6SrcCount: 1024, HopLimit: 64, TrafficClass: 0, TrafficClassCount: 57},
		TCPFlow:  &otgconfighelpers.TCPFlowParams{TCPSrcPort: 49152, TCPDstPort: 80, TCPSrcCount: 1024},
	}
	// FlowOuterIPv6InnerIPv6 Decap MPLSoGUE traffic with an IPv6 outer header and IPv6 inner payload.
	FlowOuterIPv6InnerIPv6 = &otgconfighelpers.Flow{
		TxNames:           []string{agg2.Name + ".IPv6"},
		RxNames:           []string{agg1.Interfaces[1].Name + ".IPv6"},
		SizeWeightProfile: &sizeWeightProfile,
		FlowName:          "MPLSOGUETrafficIPv6OuterHeaderIPv6Payload",
		EthFlow:           &otgconfighelpers.EthFlowParams{SrcMAC: agg2.AggMAC},
		IPv6Flow:          &otgconfighelpers.IPv6FlowParams{IPv6Src: interface7.IPv6, IPv6Dst: coreIntf.IPv6, IPv6SrcCount: 1024},
		MPLSFlow:          &otgconfighelpers.MPLSFlowParams{MPLSLabel: 99992, MPLSExp: 7},
		UDPFlow:           &otgconfighelpers.UDPFlowParams{UDPSrcPort: 49152, UDPDstPort: udpPort},
	}
	// FlowOuterIPv6OuterValidation MPLSOGUE traffic IPv6 outer header IPv6 payload.
	FlowOuterIPv6OuterValidation = &otgvalidationhelpers.OTGValidation{
		Interface: &otgvalidationhelpers.InterfaceParams{Names: []string{agg1.Interfaces[1].Name}, Ports: agg1.MemberPorts},
		Flow:      &otgvalidationhelpers.FlowParams{Name: FlowOuterIPv6InnerIPv6.FlowName, TolerancePct: 0.5},
	}
	// FlowOuterIPv6InnerIPv4 Decap MPLSoGUE traffic with an IPv6 outer header and IPv4 inner payload.
	FlowOuterIPv6InnerIPv4 = &otgconfighelpers.Flow{
		TxNames:           []string{agg2.Name + ".IPv6"},
		RxNames:           []string{agg1.Interfaces[0].Name + ".IPv4"},
		SizeWeightProfile: &sizeWeightProfile,
		FlowName:          "MPLSOGUETrafficIPv6OuterHeaderIPv4Payload",
		EthFlow:           &otgconfighelpers.EthFlowParams{SrcMAC: agg2.AggMAC},
		IPv6Flow:          &otgconfighelpers.IPv6FlowParams{IPv6Src: interface7.IPv6, IPv6Dst: coreIntf.IPv6, IPv6SrcCount: 1024},
		MPLSFlow:          &otgconfighelpers.MPLSFlowParams{MPLSLabel: 99991, MPLSExp: 7},
		UDPFlow:           &otgconfighelpers.UDPFlowParams{UDPSrcPort: 49152, UDPDstPort: udpPort},
	}
	// FlowOuterIPv6InnerIPv4Validation MPLSOGUE traffic IPv6 outer header IPv4 payload.
	FlowOuterIPv6InnerIPv4Validation = &otgvalidationhelpers.OTGValidation{
		Interface: &otgvalidationhelpers.InterfaceParams{Names: []string{agg1.Interfaces[0].Name}, Ports: agg1.MemberPorts},
		Flow:      &otgvalidationhelpers.FlowParams{Name: FlowOuterIPv6InnerIPv4.FlowName, TolerancePct: 0.5},
	}
	// FlowOuterIPv4Agg3 sources decap traffic from Aggregate3 (ATE ports 5,6), IPv4 payload.
	FlowOuterIPv4Agg3 = &otgconfighelpers.Flow{
		TxNames:           []string{agg3.Name + ".IPv4"},
		RxNames:           []string{agg1.Interfaces[0].Name + ".IPv4"},
		SizeWeightProfile: &sizeWeightProfile,
		FlowName:          "MPLSOGUETrafficIPv4InterfaceIPv4PayloadAgg3",
		EthFlow:           &otgconfighelpers.EthFlowParams{SrcMAC: agg3.AggMAC},
		IPv4Flow:          &otgconfighelpers.IPv4FlowParams{IPv4Src: "100.64.0.1", IPv4Dst: "11.1.1.1", IPv4SrcCount: 1024},
		MPLSFlow:          &otgconfighelpers.MPLSFlowParams{MPLSLabel: 99991, MPLSExp: 7},
		UDPFlow:           &otgconfighelpers.UDPFlowParams{UDPSrcPort: 49152, UDPDstPort: udpPort},
	}
	// FlowOuterIPv4Agg3Validation MPLSOGUE traffic IPv4 interface IPv4 Payload Agg3.
	FlowOuterIPv4Agg3Validation = &otgvalidationhelpers.OTGValidation{
		Interface: &otgvalidationhelpers.InterfaceParams{Names: []string{agg1.Interfaces[0].Name}, Ports: agg1.MemberPorts},
		Flow:      &otgvalidationhelpers.FlowParams{Name: FlowOuterIPv4Agg3.FlowName, TolerancePct: 0.5},
	}
	// FlowOuterIPv6Agg3 sources decap traffic from Aggregate3 (ATE ports 5,6), IPv6 payload.
	FlowOuterIPv6Agg3 = &otgconfighelpers.Flow{
		TxNames:           []string{agg3.Name + ".IPv4"},
		RxNames:           []string{agg1.Interfaces[1].Name + ".IPv6"},
		SizeWeightProfile: &sizeWeightProfile,
		FlowName:          "MPLSOGUETrafficIPv6InterfaceIPv6PayloadAgg3",
		EthFlow:           &otgconfighelpers.EthFlowParams{SrcMAC: agg3.AggMAC},
		IPv4Flow:          &otgconfighelpers.IPv4FlowParams{IPv4Src: "100.64.0.1", IPv4Dst: "11.1.1.1", IPv4SrcCount: 1024},
		MPLSFlow:          &otgconfighelpers.MPLSFlowParams{MPLSLabel: 99992, MPLSExp: 7},
		UDPFlow:           &otgconfighelpers.UDPFlowParams{UDPSrcPort: 49152, UDPDstPort: udpPort},
	}
	// FlowOuterIPv6Agg3Validation MPLSOGUE traffic IPv6 interface IPv6 Payload Agg3.
	FlowOuterIPv6Agg3Validation = &otgvalidationhelpers.OTGValidation{
		Interface: &otgvalidationhelpers.InterfaceParams{Names: []string{agg1.Interfaces[1].Name}, Ports: agg1.MemberPorts},
		Flow:      &otgvalidationhelpers.FlowParams{Name: FlowOuterIPv6Agg3.FlowName, TolerancePct: 0.5},
	}
	// FlowOuterIPv4Multicast Decap IPv4 Interface multicast payload traffic params Outer Header, sourced from Aggregate2 (PF-1.19.4).
	FlowOuterIPv4Multicast = &otgconfighelpers.Flow{
		TxNames:           []string{agg2.Name + ".IPv4"},
		RxNames:           []string{agg1.Interfaces[0].Name + ".IPv4"},
		SizeWeightProfile: &sizeWeightProfile,
		FlowName:          "MPLSOGUETrafficIPv4InterfaceMulticastPayload",
		EthFlow:           &otgconfighelpers.EthFlowParams{SrcMAC: agg2.AggMAC},
		IPv4Flow:          &otgconfighelpers.IPv4FlowParams{IPv4Src: "100.64.0.1", IPv4Dst: "11.1.1.1", IPv4SrcCount: 1024},
		MPLSFlow:          &otgconfighelpers.MPLSFlowParams{MPLSLabel: multicastMPLSLabel, MPLSExp: 7},
		UDPFlow:           &otgconfighelpers.UDPFlowParams{UDPSrcPort: 49152, UDPDstPort: udpPort},
	}
	// FlowOuterIPv4MulticastValidation MPLSOGUE traffic IPv4 interface multicast Payload.
	FlowOuterIPv4MulticastValidation = &otgvalidationhelpers.OTGValidation{
		Interface: &otgvalidationhelpers.InterfaceParams{Names: []string{agg1.Interfaces[0].Name}, Ports: agg1.MemberPorts},
		Flow:      &otgvalidationhelpers.FlowParams{Name: FlowOuterIPv4Multicast.FlowName, TolerancePct: 0.5},
	}
	// FlowOuterIPv4MulticastAgg3 sources multicast decap traffic from Aggregate3 (ATE ports 5,6), per PF-1.19.4.
	FlowOuterIPv4MulticastAgg3 = &otgconfighelpers.Flow{
		TxNames:           []string{agg3.Name + ".IPv4"},
		RxNames:           []string{agg1.Interfaces[0].Name + ".IPv4"},
		SizeWeightProfile: &sizeWeightProfile,
		FlowName:          "MPLSOGUETrafficIPv4InterfaceMulticastPayloadAgg3",
		EthFlow:           &otgconfighelpers.EthFlowParams{SrcMAC: agg3.AggMAC},
		IPv4Flow:          &otgconfighelpers.IPv4FlowParams{IPv4Src: "100.64.0.1", IPv4Dst: "11.1.1.1", IPv4SrcCount: 1024},
		MPLSFlow:          &otgconfighelpers.MPLSFlowParams{MPLSLabel: multicastMPLSLabel, MPLSExp: 7},
		UDPFlow:           &otgconfighelpers.UDPFlowParams{UDPSrcPort: 49152, UDPDstPort: udpPort},
	}
	// FlowOuterIPv4MulticastAgg3Validation MPLSOGUE traffic IPv4 interface multicast Payload Agg3.
	FlowOuterIPv4MulticastAgg3Validation = &otgvalidationhelpers.OTGValidation{
		Interface: &otgvalidationhelpers.InterfaceParams{Names: []string{agg1.Interfaces[0].Name}, Ports: agg1.MemberPorts},
		Flow:      &otgvalidationhelpers.FlowParams{Name: FlowOuterIPv4MulticastAgg3.FlowName, TolerancePct: 0.5},
	}
	validationsIPv4 = []packetvalidationhelpers.ValidationType{
		packetvalidationhelpers.ValidateIPv4Header,
		packetvalidationhelpers.ValidateTCPHeader,
	}
	decapValidationIPv4 = &packetvalidationhelpers.PacketValidation{
		PortName:    "port1",
		CaptureName: "ipv4_decap",
		Validations: validationsIPv4,
		// DSCP(TOS) is swept across the full range 0-56 (see updateFlow), so validate
		// that the preserved value stays within that range rather than an exact match.
		Flags:     &packetvalidationhelpers.ValidationFlags{ValidateTosRange: true},
		IPv4Layer: &packetvalidationhelpers.IPv4Layer{DstIP: "21.1.1.1", TosMin: 0, TosMax: 56, TTL: 64, Protocol: packetvalidationhelpers.TCP},
		TCPLayer:  &packetvalidationhelpers.TCPLayer{SrcPort: 49152, DstPort: 80},
	}
	validationsIPv6Outer = []packetvalidationhelpers.ValidationType{
		packetvalidationhelpers.ValidateIPv6Header,
	}
	decapValidationIPv6Outer = &packetvalidationhelpers.PacketValidation{
		PortName:    "port1",
		CaptureName: "ipv6_outer_decap",
		Validations: validationsIPv6Outer,
		IPv6Layer:   &packetvalidationhelpers.IPv6Layer{DstIP: "3000:1::1", HopLimit: 64},
	}
	decapValidationIPv6Inner = &packetvalidationhelpers.PacketValidation{
		PortName:    "port1",
		CaptureName: "ipv6_inner_decap",
		Validations: []packetvalidationhelpers.ValidationType{packetvalidationhelpers.ValidateIPv6Header},
		Flags: &packetvalidationhelpers.ValidationFlags{
			ValidateTrafficClassRange: true,
			TrafficClassMin:           0,
			TrafficClassMax:           56,
		},
		IPv6Layer: &packetvalidationhelpers.IPv6Layer{DstIP: "3000:1::1", HopLimit: 64},
	}
	validationsIPv6OuterInnerIPv4 = []packetvalidationhelpers.ValidationType{
		packetvalidationhelpers.ValidateIPv4Header,
	}
	// decapValidationIPv6OuterInnerIPv4 validates the decapsulated inner IPv4 packet
	// produced from a MPLSoGUE flow that used an IPv6 outer header (PF-1.19.v6).
	decapValidationIPv6OuterInnerIPv4 = &packetvalidationhelpers.PacketValidation{
		PortName:    "port1",
		CaptureName: "ipv6_outer_ipv4_inner_decap",
		Validations: validationsIPv6OuterInnerIPv4,
		IPv4Layer:   &packetvalidationhelpers.IPv4Layer{DstIP: "21.1.1.1", TTL: 64, SkipProtocolCheck: true},
	}
	multicastRewriteValidation = &packetvalidationhelpers.PacketValidation{
		PortName:      "port1",
		CaptureName:   "ipv4_multicast_rewrite",
		Validations:   []packetvalidationhelpers.ValidationType{packetvalidationhelpers.ValidateEthernetHeader},
		EthernetLayer: &packetvalidationhelpers.EthernetLayer{DstMAC: "01:00:5e:01:01:01"},
	}
)

func ConfigureOTG(t *testing.T, ate *ondatra.ATEDevice) {
	t.Helper()
	top.Captures().Clear()

	// Create a slice of aggPortData for easier iteration
	aggs := []*otgconfighelpers.Port{agg1, agg2, agg3}

	// Configure OTG Interfaces
	for _, agg := range aggs {
		otgconfighelpers.ConfigureNetworkInterface(t, top, ate, agg)
	}
	ate.OTG().PushConfig(t, top)
}

// PF-1.19.1: Generate DUT Configuration
func ConfigureDut(t *testing.T, dut *ondatra.DUTDevice, ocPFParams cfgplugins.OcPolicyForwardingParams) {
	custAggID = netutil.NextAggregateInterface(t, dut)
	custSubinterfaces := []*attrs.Attributes{
		&custIntfIPv4, &custIntfIPv6, &custIntfdualStack, &custIntfIPv4MultiCloud, &custIntfIPv4JumboMTU,
		&custIntfIPv4Global2, &custIntfIPv6B, &custIntfdualStack2, &custIntfdualStack3, &custIntfdualStack4,
	}
	configureInterfaces(t, dut, custPorts, custSubinterfaces, custAggID)
	for _, a := range custSubinterfaces {
		configureInterfaceProperties(t, dut, custAggID, a, ocPFParams)
	}
	aggID = netutil.NextAggregateInterface(t, dut)
	configureInterfaces(t, dut, corePorts, []*attrs.Attributes{&coreIntf}, aggID)
	// Aggregate3 is the 2nd ingress aggregate (ATE ports 5,6).
	aggID2 = netutil.NextAggregateInterface(t, dut)
	configureInterfaces(t, dut, corePorts2, []*attrs.Attributes{&coreIntf2}, aggID2)
	configureStaticRoute(t, dut)
	disableLLDP(t, dut)
	_, ni, pf := cfgplugins.SetupPolicyForwardingInfraOC(ocPFParams.NetworkInstanceName)
	DecapMPLSInGUE(t, dut, pf, ni, ocPFParams)
}

func disableLLDP(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	for _, port := range custPorts {
		physPortName := dut.Port(t, port).Name()
		gnmi.Replace(t, dut, gnmi.OC().Lldp().Interface(physPortName).Config(), &oc.Lldp_Interface{
			Name:    ygot.String(physPortName),
			Enabled: ygot.Bool(false),
		})
	}
}

func waitForLAGUp(t *testing.T, dut *ondatra.DUTDevice, aggID string, ports []string) {
	t.Helper()
	for _, portID := range ports {
		portName := dut.Port(t, portID).Name()
		gnmi.Await(t, dut, gnmi.OC().Interface(portName).OperStatus().State(), 2*time.Minute, oc.Interface_OperStatus_UP)
		memberPath := gnmi.OC().Lacp().Interface(aggID).Member(portName).State()
		_, ok := gnmi.Watch(t, dut, memberPath, 3*time.Minute, func(value *ygnmi.Value[*oc.Lacp_Interface_Member]) bool {
			if !value.IsPresent() {
				return false
			}
			member, present := value.Val()
			return present && member.Synchronization == oc.Lacp_LacpSynchronizationType_IN_SYNC && member.GetCollecting() && member.GetDistributing()
		}).Await(t)
		if !ok {
			t.Fatalf("LACP member %s in %s did not reach IN_SYNC/collecting/distributing", portName, aggID)
		}
	}
	t.Logf("DUT LAG %s members are IN_SYNC, collecting, and distributing", aggID)
}

func waitForOTGLAGUp(t *testing.T, ate *ondatra.ATEDevice, agg *otgconfighelpers.Port) {
	t.Helper()
	_, ok := gnmi.Watch(t, ate.OTG(), gnmi.OTG().Lag(agg.Name).OperStatus().State(), 2*time.Minute, func(value *ygnmi.Value[otgtelemetry.E_Lag_OperStatus]) bool {
		status, present := value.Val()
		return present && status.String() == "UP"
	}).Await(t)
	if !ok {
		t.Fatalf("OTG LAG %s did not reach UP", agg.Name)
	}

	_, ok = gnmi.Watch(t, ate.OTG(), gnmi.OTG().Lacp().State(), 2*time.Minute, func(value *ygnmi.Value[*otgtelemetry.Lacp]) bool {
		lacp, present := value.Val()
		if !present || lacp == nil {
			return false
		}
		for _, portID := range agg.MemberPorts {
			member := lacp.GetLagMember(ate.Port(t, portID).ID())
			if !member.GetCollecting() || !member.GetDistributing() {
				return false
			}
		}
		return true
	}).Await(t)
	if !ok {
		t.Fatalf("OTG LAG %s members did not reach collecting/distributing", agg.Name)
	}
	t.Logf("OTG LAG %s is UP and members are collecting/distributing", agg.Name)
}

func verifyPortsUp(t *testing.T, dev *ondatra.Device) {
	t.Helper()
	for _, p := range dev.Ports() {
		status := gnmi.Get(t, dev, gnmi.OC().Interface(p.Name()).OperStatus().State())
		if want := oc.Interface_OperStatus_UP; status != want {
			t.Fatalf("%s Status: got %v, want %v", p, status, want)
		}
	}
}

func TestSetup(t *testing.T) {
	t.Log("PF-1.19.1: Generate DUT Configuration")
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	fptest.ConfigureDefaultNetworkInstance(t, dut)

	// Get default parameters for OC Policy Forwarding
	ocPFParams := GetDefaultOcPolicyForwardingParams()

	// Pass ocPFParams to ConfigureDut
	ConfigureDut(t, dut, ocPFParams)
	ConfigureOTG(t, ate)
	ate.OTG().StartProtocols(t)
	waitForLAG(t, dut, ate)
}

func waitForLAG(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice) {
	for _, agg := range []*otgconfighelpers.Port{agg1, agg2, agg3} {
		waitForOTGLAGUp(t, ate, agg)
	}
	waitForLAGUp(t, dut, custAggID, custPorts)
	waitForLAGUp(t, dut, aggID, corePorts)
	waitForLAGUp(t, dut, aggID2, corePorts2)
}

// GetDefaultOcPolicyForwardingParams provides default parameters for the generator,
// matching the values in the provided JSON example.
func GetDefaultOcPolicyForwardingParams() cfgplugins.OcPolicyForwardingParams {
	return cfgplugins.OcPolicyForwardingParams{
		NetworkInstanceName: "DEFAULT",
		InterfaceID:         "Agg1.10",
		AppliedPolicyName:   policyName,
		InnerDstIPv4:        "11.0.0.0/8",
		DecapPolicy: cfgplugins.DecapPolicyParams{
			StaticLSPNameIPv4:         "Customer IPV4 in:99991 out:pop",
			StaticLSPLabelIPv4:        99991,
			StaticLSPNextHopIPv4:      interface1.IPv4,
			StaticLSPNameIPv6:         "Customer IPV6 in:99992 out:pop",
			StaticLSPLabelIPv6:        99992,
			StaticLSPNextHopIPv6:      interface2.IPv6,
			StaticLSPNameMulticast:    fmt.Sprintf("Customer Multicast in:%d out:pop", multicastMPLSLabel),
			StaticLSPLabelMulticast:   multicastMPLSLabel,
			StaticLSPNextHopMulticast: interface1.IPv4,
		},
	}
}

func ipv6OuterDecapParams(dut *ondatra.DUTDevice) cfgplugins.OcPolicyForwardingParams {
	return cfgplugins.OcPolicyForwardingParams{
		AppliedPolicyName:   policyName,
		NetworkInstanceName: deviations.DefaultNetworkInstance(dut),
		TunnelIP:            coreIntf.IPv6 + "/128",
		GUEPort:             udpPort,
		IPType:              cfgplugins.IPv6,
		DecapProtocol:       "mpls",
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
func DecapMPLSInGUE(t *testing.T, dut *ondatra.DUTDevice, pf *oc.NetworkInstance_PolicyForwarding, ni *oc.NetworkInstance, ocPFParams cfgplugins.OcPolicyForwardingParams) {
	cfgplugins.MplsConfig(t, dut)
	cfgplugins.QosClassificationConfig(t, dut)
	cfgplugins.LabelRangeConfig(t, dut)
	cfgplugins.DecapGroupConfigGue(t, dut, pf, ocPFParams)
	cfgplugins.DecapGroupConfigGueIPv6Outer(t, dut, pf, ipv6OuterDecapParams(dut))
	cfgplugins.MPLSStaticLSPConfig(t, dut, ni, ocPFParams)
	if !deviations.PolicyForwardingOCUnsupported(dut) {
		PushPolicyForwardingConfig(t, dut, ni)
	}
}

// PF-1.19.2: Verify PF MPLSoGUE Decap action for IPv4 and IPv6 traffic.
func TestMPLSOGUEDecapIPv4AndIPv6(t *testing.T) {
	ate := ondatra.ATE(t, "ate")
	t.Log("PF-1.19.2: Verify MPLSoGUE decapsulate action for IPv4 and IPv6 payload")
	testFlows := []TestFlow{
		{flowOuterConfig: FlowOuterIPv4, flowInnerConfig: FlowInnerIPv4, flowValidation: FlowOuterIPv4Validation},
		{flowOuterConfig: FlowOuterIPv6, flowInnerConfig: FlowInnerIPv6, flowValidation: FlowOuterIPv6Validation},
		{flowOuterConfig: FlowOuterIPv4Agg3, flowInnerConfig: FlowInnerIPv4, flowValidation: FlowOuterIPv4Agg3Validation},
		{flowOuterConfig: FlowOuterIPv6Agg3, flowInnerConfig: FlowInnerIPv6, flowValidation: FlowOuterIPv6Agg3Validation},
	}

	for _, flow := range testFlows {
		createflow(t, top, flow.flowOuterConfig, flow.flowInnerConfig, true)
		sendTraffic(t, ate, false)
		if err := flow.flowValidation.ValidateLossOnFlows(t, ate); err != nil {
			t.Errorf("ValidateLossOnFlows: got err: %q, want nil", err)
		}
		if err := flow.flowValidation.ValidateLoadBalanceOnLAG(t, ate); err != nil {
			t.Errorf("ValidateLoadBalanceOnLAG: got err: %q, want nil", err)
		}
	}
}

// PF-1.19.3: Verify MPLSoGUE decapsulate action for IPv4 and IPv6 payload with
// changes in IPv4 and IPv6 configs.
func TestMPLSOGUEDecapIPv4Ipv6ConfigRefresh(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	t.Log("PF-1.19.3: Verify MPLSoGUE decapsulate action for IPv4 and IPv6 payload with changes in IPv4 and IPv6 configs")
	testFlows := []TestFlow{
		{flowOuterConfig: FlowOuterIPv4, flowInnerConfig: FlowInnerIPv4, flowValidation: FlowOuterIPv4Validation},
		{flowOuterConfig: FlowOuterIPv6, flowInnerConfig: FlowInnerIPv6, flowValidation: FlowOuterIPv6Validation},
		{flowOuterConfig: FlowOuterIPv4Agg3, flowInnerConfig: FlowInnerIPv4, flowValidation: FlowOuterIPv4Agg3Validation},
		{flowOuterConfig: FlowOuterIPv6Agg3, flowInnerConfig: FlowInnerIPv6, flowValidation: FlowOuterIPv6Agg3Validation},
	}
	for i, flow := range testFlows {
		createflow(t, top, flow.flowOuterConfig, flow.flowInnerConfig, i == 0)
	}

	verifyBothFlows := func(step string) {
		sendTraffic(t, ate, false)
		for _, flow := range testFlows {
			if err := flow.flowValidation.ValidateLossOnFlows(t, ate); err != nil {
				t.Errorf("%s: ValidateLossOnFlows(%s): got err: %q, want nil", step, flow.flowOuterConfig.FlowName, err)
			}
		}
	}

	verifyBothFlows("baseline")

	refreshInterfaceVLAN(t, dut, &custIntfIPv4)
	verifyBothFlows("after IPv4 VLAN config refresh")
	refreshInterfaceVLAN(t, dut, &custIntfIPv6)
	verifyBothFlows("after IPv6 VLAN config refresh")

	decapPolicy := GetDefaultOcPolicyForwardingParams().DecapPolicy
	cfgplugins.RefreshStaticLSP(t, dut, decapPolicy, cfgplugins.IPv4)
	verifyBothFlows("after IPv4 decap config refresh")
	cfgplugins.RefreshStaticLSP(t, dut, decapPolicy, cfgplugins.IPv6)
	verifyBothFlows("after IPv6 decap config refresh")
}

// PF-1.19.4: Verify MPLSoGUE decapsulate action for IPv4 multicast payload.
func TestMPLSOGUEDecapIPv4Multicast(t *testing.T) {
	ate := ondatra.ATE(t, "ate")
	t.Log("PF-1.19.4: Verify MPLSoGUE decapsulate action for IPv4 multicast payload")
	testFlows := []TestFlow{
		{flowOuterConfig: FlowOuterIPv4Multicast, flowInnerConfig: FlowInnerIPv4Multicast, flowValidation: FlowOuterIPv4MulticastValidation},
		{flowOuterConfig: FlowOuterIPv4MulticastAgg3, flowInnerConfig: FlowInnerIPv4Multicast, flowValidation: FlowOuterIPv4MulticastAgg3Validation},
	}
	for _, flow := range testFlows {
		createflow(t, top, flow.flowOuterConfig, flow.flowInnerConfig, true)
		packetvalidationhelpers.ConfigurePacketCapture(t, top, multicastRewriteValidation)
		sendTraffic(t, ate, true)
		if err := flow.flowValidation.ValidateLossOnFlows(t, ate); err != nil {
			t.Errorf("ValidateLossOnFlows(%s): got err: %q, want nil", flow.flowOuterConfig.FlowName, err)
		}
		if err := flow.flowValidation.ValidateLoadBalanceOnLAG(t, ate); err != nil {
			t.Errorf("ValidateLoadBalanceOnLAG(): got err: %q, want nil", err)
		}
		if err := packetvalidationhelpers.CaptureAndValidatePackets(t, ate, multicastRewriteValidation); err != nil {
			t.Errorf("CaptureAndValidatePackets(multicast rewrite): got err: %q", err)
		}
		packetvalidationhelpers.ClearCapture(t, top, ate)
	}
}

// PF-1.19.5: Verify MPLSoGUE DSCP/TTL preserve operation.
func TestMPLSOGUEDecapInnerPayloadPreserve(t *testing.T) {
	ate := ondatra.ATE(t, "ate")
	t.Log("PF-1.19.5: Verify MPLSoGUE DSCP/TTL preserve operation")
	testFlows := []TestFlow{
		{flowOuterConfig: FlowOuterIPv4, flowInnerConfig: FlowInnerIPv4, flowValidation: FlowOuterIPv4Validation, packetValidation: decapValidationIPv4},
		{flowOuterConfig: FlowOuterIPv6, flowInnerConfig: FlowInnerIPv6, flowValidation: FlowOuterIPv6Validation, packetValidation: decapValidationIPv6Inner},
	}
	for _, tf := range testFlows {
		packetvalidationhelpers.ConfigurePacketCapture(t, top, tf.packetValidation)
		updateFlow(t, tf.flowOuterConfig, tf.flowInnerConfig, true, pps, totalPackets)
		sendTraffic(t, ate, true)
		if err := tf.flowValidation.ValidateLossOnFlows(t, ate); err != nil {
			t.Errorf("ValidateLossOnFlows(%s): got err: %q, want nil", tf.flowOuterConfig.FlowName, err)
		}
		if err := packetvalidationhelpers.CaptureAndValidatePackets(t, ate, tf.packetValidation); err != nil {
			t.Errorf("CaptureAndValidatePackets(%s): got err: %q", tf.flowOuterConfig.FlowName, err)
		}
		packetvalidationhelpers.ClearCapture(t, top, ate)
	}
}

// PF-1.19.6: Verify IPV4/IPV6 nexthop resolution of decap traffic
func TestMPLSOGUEDecapNextHopResolution(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	t.Log("PF-1.19.6: Verify IPV4/IPV6 nexthop resolution of decap traffic")
	testFlows := []TestFlow{
		{flowOuterConfig: FlowOuterIPv4, flowInnerConfig: FlowInnerIPv4, flowValidation: FlowOuterIPv4Validation},
		{flowOuterConfig: FlowOuterIPv6, flowInnerConfig: FlowInnerIPv6, flowValidation: FlowOuterIPv6Validation},
		{flowOuterConfig: FlowOuterIPv4Agg3, flowInnerConfig: FlowInnerIPv4, flowValidation: FlowOuterIPv4Agg3Validation},
		{flowOuterConfig: FlowOuterIPv6Agg3, flowInnerConfig: FlowInnerIPv6, flowValidation: FlowOuterIPv6Agg3Validation},
		{flowOuterConfig: FlowOuterIPv4Multicast, flowInnerConfig: FlowInnerIPv4Multicast, flowValidation: FlowOuterIPv4MulticastValidation},
		{flowOuterConfig: FlowOuterIPv4MulticastAgg3, flowInnerConfig: FlowInnerIPv4Multicast, flowValidation: FlowOuterIPv4MulticastAgg3Validation},
	}
	for i, flow := range testFlows {
		updateFlow(t, flow.flowOuterConfig, flow.flowInnerConfig, i == 0, pps, totalPackets)
	}
	// Clear ARP entries and IPv6 neighbors on the DUT before verifying that they
	// are correctly re-resolved upon receiving decapsulated traffic.
	cfgplugins.ClearNeighbors(t, dut)
	sendTraffic(t, ate, false)
	for _, flow := range testFlows {
		if err := flow.flowValidation.ValidateLossOnFlows(t, ate); err != nil {
			t.Errorf("ValidateLossOnFlows(%s): got err: %q, want nil", flow.flowOuterConfig.FlowName, err)
		}
	}
}

// refreshInterfaceVLAN removes and re-adds a customer subinterface VLAN configuration,
func refreshInterfaceVLAN(t *testing.T, dut *ondatra.DUTDevice, a *attrs.Attributes) {
	t.Helper()
	subIntfPath := gnmi.OC().Interface(custAggID).Subinterface(a.Subinterface)
	t.Logf("Removing subinterface %s.%d", custAggID, a.Subinterface)
	gnmi.Delete(t, dut, subIntfPath.Config())
	t.Logf("Re-adding subinterface %s.%d", custAggID, a.Subinterface)
	s := &oc.Interface_Subinterface{Index: ygot.Uint32(a.Subinterface)}
	s.GetOrCreateVlan().GetOrCreateMatch().GetOrCreateSingleTagged().SetVlanId(uint16(a.Subinterface))
	configureInterfaceAddress(dut, s, a)
	gnmi.Replace(t, dut, subIntfPath.Config(), s)
}

// PF-1.19.v6: Validate decapsulation of MPLS over GUE with an IPv6 outer header.
func TestMPLSOGUEDecapIPv6OuterHeader(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	t.Log("PF-1.19.v6: Verify MPLSoGUE decapsulate action for an IPv6 outer header")
	flowInnerIPv6NoTCP := &otgconfighelpers.Flow{
		IPv6Flow: &otgconfighelpers.IPv6FlowParams{
			IPv6Src: FlowInnerIPv6.IPv6Flow.IPv6Src,
			IPv6Dst: FlowInnerIPv6.IPv6Flow.IPv6Dst,
		},
	}
	flowInnerIPv4NoTCP := &otgconfighelpers.Flow{
		IPv4Flow: &otgconfighelpers.IPv4FlowParams{
			IPv4Src: FlowInnerIPv4.IPv4Flow.IPv4Src,
			IPv4Dst: FlowInnerIPv4.IPv4Flow.IPv4Dst,
		},
	}
	testFlows := []TestFlow{
		{flowOuterConfig: FlowOuterIPv6InnerIPv6, flowInnerConfig: flowInnerIPv6NoTCP, flowValidation: FlowOuterIPv6OuterValidation, packetValidation: decapValidationIPv6Outer},
		{flowOuterConfig: FlowOuterIPv6InnerIPv4, flowInnerConfig: flowInnerIPv4NoTCP, flowValidation: FlowOuterIPv6InnerIPv4Validation, packetValidation: decapValidationIPv6OuterInnerIPv4},
	}
	for _, flow := range testFlows {
		packetvalidationhelpers.ConfigurePacketCapture(t, top, flow.packetValidation)
		createflow(t, top, flow.flowOuterConfig, flow.flowInnerConfig, true)
		sendTraffic(t, ate, true)
		if err := flow.flowValidation.ValidateLossOnFlows(t, ate); err != nil {
			t.Errorf("ValidateLossOnFlows(%s): got err: %q", flow.flowOuterConfig.FlowName, err)
		}
		if err := packetvalidationhelpers.CaptureAndValidatePackets(t, ate, flow.packetValidation); err != nil {
			t.Errorf("CaptureAndValidatePackets(%s): got err: %q", flow.flowOuterConfig.FlowName, err)
		}
		if err := verifyCounters(t, dut); err != nil {
			t.Errorf("verifyCounters: got err: %q", err)
		}
		packetvalidationhelpers.ClearCapture(t, top, ate)
	}
}

func verifyCounters(t *testing.T, dut *ondatra.DUTDevice) error {
	t.Helper()

	if deviations.GueGreDecapUnsupported(dut) {
		t.Logf("Skipping PF rule matched-pkts check: deviation GueGreDecapUnsupported is set for vendor %v", dut.Vendor())
		return nil
	}
	niName := GetDefaultOcPolicyForwardingParams().NetworkInstanceName
	matchedPkts := gnmi.Get(t, dut, gnmi.OC().NetworkInstance(niName).PolicyForwarding().Policy(gueV6DecapPolicyName).Rule(gueV6DecapRuleSeqID).MatchedPkts().State())
	if matchedPkts == 0 {
		return fmt.Errorf("PF rule matched-pkts for policy %q rule %d: got 0, want > 0", gueV6DecapPolicyName, gueV6DecapRuleSeqID)
	}
	t.Logf("PF rule matched-pkts for policy %q rule %d: %d", gueV6DecapPolicyName, gueV6DecapRuleSeqID, matchedPkts)
	return nil
}

func sendTraffic(t *testing.T, ate *ondatra.ATEDevice, captureTraffic bool) {
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	waitForLAG(t, ondatra.DUT(t, "dut"), ate)
	if err := flowResolveArp.IsIPv4Interfaceresolved(t, ate); err != nil {
		t.Fatalf("IsIPv4Interfaceresolved(): got err: %q, want nil", err)
	}
	if err := flowResolveArp.IsIPv6Interfaceresolved(t, ate); err != nil {
		t.Fatalf("IsIPv6Interfaceresolved(): got err: %q, want nil", err)
	}
	if captureTraffic {
		cs := packetvalidationhelpers.StartCapture(t, ate)
		defer packetvalidationhelpers.StopCapture(t, ate, cs)
	}
	ate.OTG().StartTraffic(t)
	for _, flow := range top.Flows().Items() {
		waitForTraffic(t, ate.OTG(), flow.Name(), trafficTimeout)
	}
	ate.OTG().StopTraffic(t)

}

func waitForTraffic(t *testing.T, otg *otg.OTG, flowName string, timeout time.Duration) {
	t.Logf("Waiting for traffic to stop on flow %s, with a total timeout of %s", flowName, timeout)
	transmitPath := gnmi.OTG().Flow(flowName).Transmit().State()
	_, ok := gnmi.Watch(t, otg, transmitPath, timeout, func(val *ygnmi.Value[bool]) bool {
		transmitState, present := val.Val()
		return present && !transmitState
	}).Await(t)

	if !ok {
		t.Errorf("Traffic for flow %s did not stop within the timeout of %s", flowName, timeout)
	} else {
		t.Logf("Traffic for flow %s has stopped", flowName)
	}
}

func createflow(t *testing.T, top gosnappi.Config, paramsOuter *otgconfighelpers.Flow, paramsInner *otgconfighelpers.Flow, clearFlows bool) {
	if clearFlows {
		top.Flows().Clear()
	}
	flow := *paramsOuter
	flow.PpsRate = pps
	flow.PacketsToSend = totalPackets
	flow.CreateFlow(top)
	flow.AddEthHeader()
	switch {
	case flow.IPv4Flow != nil:
		flow.AddIPv4Header()
	case flow.IPv6Flow != nil:
		flow.AddIPv6Header()
	}
	flow.AddUDPHeader()
	flow.AddMPLSHeader()
	if paramsInner.IPv4Flow != nil {
		flow.IPv4Flow = paramsInner.IPv4Flow
		flow.AddIPv4Header()
	}
	if paramsInner.IPv6Flow != nil {
		flow.IPv6Flow = paramsInner.IPv6Flow
		flow.AddIPv6Header()
	}
	if paramsInner.TCPFlow != nil {
		flow.TCPFlow = paramsInner.TCPFlow
		flow.AddTCPHeader()
	}
	if paramsInner.UDPFlow != nil {
		flow.UDPFlow = paramsInner.UDPFlow
		flow.AddUDPHeader()
	}
}

func updateFlow(t *testing.T, paramsOuter *otgconfighelpers.Flow, paramsInner *otgconfighelpers.Flow, clearFlows bool, pps uint64, totalPackets uint32) {
	paramsOuter.PacketsToSend = totalPackets
	paramsOuter.PpsRate = pps
	// Sweep the full DSCP/ToS range (0-56) required by PF-1.19.5/PF-1.19.6, so that
	// preserve behavior is validated across all possible DSCP values.
	if paramsInner.IPv6Flow != nil {
		paramsInner.IPv6Flow.TrafficClass = 0
		paramsInner.IPv6Flow.TrafficClassCount = 57
	}
	if paramsInner.IPv4Flow != nil {
		paramsInner.IPv4Flow.RawPriority = 0
		paramsInner.IPv4Flow.RawPriorityCount = 57
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
	// LACP period short (fast) is the default required timeout for member links.
	lacp.Interval = oc.Lacp_LacpPeriodType_FAST
	lacpPath := d.Lacp().Interface(aggID)
	// fptest.LogQuery(t, "LACP", lacpPath.Config(), lacp)
	gnmi.Replace(t, dut, lacpPath.Config(), lacp)
	// TODO - to remove this sleep later
	time.Sleep(5 * time.Second)

	agg := &oc.Interface{Name: ygot.String(aggID)}
	configDUTInterface(agg, subinterfaces, dut)
	agg.GetOrCreateAggregation().LagType = oc.IfAggregate_AggregationType_LACP
	agg.Type = ieee8023adLag
	aggPath := d.Interface(aggID)
	// fptest.LogQuery(t, aggID, aggPath.Config(), agg)
	gnmi.Replace(t, dut, aggPath.Config(), agg)

	for _, port := range dutAggPorts {
		holdTimeConfig := &oc.Interface_HoldTime{
			Up:   ygot.Uint32(3000),
			Down: ygot.Uint32(150),
		}
		intfPath := gnmi.OC().Interface(port.Name())
		gnmi.Update(t, dut, intfPath.HoldTime().Config(), holdTimeConfig)
		if !deviations.LoadIntervalNotSupported(dut) {
			gnmi.Update(t, dut, intfPath.Rates().Config(), &oc.Interface_Rates{LoadInterval: ygot.Uint16(30)})
		}
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
			"1": oc.UnionString("194.0.3.2"),
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
