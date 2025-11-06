package ingress_handling_of_ttl_test

import (
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	trafficTimeout          = 10 * time.Second
	ipv4                    = "IPv4"
	ipv6                    = "IPv6"
	trafficFrameSize        = 512
	trafficRatePps          = 1000
	noOfPackets             = 5
	trafficPolicyName       = "customer1_gre_encap"
	nhGroupName             = "customer1_gre_encap_v4_nhg"
	rulev4                  = "encap_all_v4"
	rulev6                  = "encap_all_v6"
	rulev4TTL1              = "encap_ttl1_v4"
	rulev6TTL1              = "encap_ttl1_v6"
	ipv4PrefixLen           = 32
	ipv6PrefixLen           = 128
	ingress                 = false
	egress                  = true
	greTTL            uint8 = 64
	greProtocol             = 47
	ipv4DstNet              = "192.168.10.1"
	ipv6DstNet              = "2001:DB8::10:1"
	ipv4DstNetServ1         = "192.168.10.11"
	ipv6DstNewServ1         = "2001:DB8::10:10"
	greIpv4DstNet           = "192.168.10.21"
	captureFilePath         = "/tmp/capture.pcap"
)

var (
	dutPort1 = attrs.Attributes{
		Name:    "port1",
		Desc:    "Dut port 1",
		IPv4:    "192.168.1.1",
		IPv4Len: 30,
		IPv6:    "2001:DB8::1",
		IPv6Len: 126,
	}

	dutPort2 = attrs.Attributes{
		Name:    "port2",
		Desc:    "Dut port 2",
		IPv4:    "192.168.1.5",
		IPv4Len: 30,
		IPv6:    "2001:DB8::5",
		IPv6Len: 126,
	}

	otgPort1 = attrs.Attributes{
		Desc:    "Otg port 1",
		Name:    "port1",
		MAC:     "00:01:12:00:00:01",
		IPv4:    "192.168.1.2",
		IPv4Len: 30,
		IPv6:    "2001:DB8::2",
		IPv6Len: 126,
	}

	otgPort2 = attrs.Attributes{
		Desc:    "Otg port 2",
		Name:    "port2",
		MAC:     "00:01:12:00:00:02",
		IPv4:    "192.168.1.6",
		IPv4Len: 30,
		IPv6:    "2001:DB8::6",
		IPv6Len: 126,
	}

	ruleMatchedPackets = map[string]uint64{
		trafficPolicyName: 0,
	}
	interfaceCounterPackets = map[string]uint64{}
	staticRoutes            = map[string]string{
		fmt.Sprintf("%s/32", ipv4DstNet):       otgPort2.IPv4,
		fmt.Sprintf("%s/128", ipv6DstNet):      otgPort2.IPv6,
		fmt.Sprintf("%s/32", ipv4DstNetServ1):  otgPort2.IPv4,
		fmt.Sprintf("%s/128", ipv6DstNewServ1): otgPort2.IPv6,
		fmt.Sprintf("%s/32", greIpv4DstNet):    otgPort2.IPv4,
	}
)

type testCase struct {
	name        string
	ipType      string
	flowName    string
	destination string
	greEncap    bool
	ttl         uint8
	expectDrop  bool
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestIngressHandlingOfTTL(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	top := gosnappi.NewConfig()
	configureDUT(t, dut)

	ap1 := ate.Port(t, "port1")
	ap2 := ate.Port(t, "port2")

	otgPort1.AddToOTG(top, ap1, &dutPort1)
	otgPort2.AddToOTG(top, ap2, &dutPort2)

	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv6")

	tests := []testCase{
		{
			name:        "PF-1.8.1: IPv4 traffic with no encapsulation on DUT and TTL = 10",
			flowName:    "PF-1.8.1",
			destination: ipv4DstNet,
			ipType:      ipv4,
			greEncap:    false,
			ttl:         10,
			expectDrop:  false,
		},
		{
			name:        "PF-1.8.2: IPv6 traffic with no encapsulation on DUT and TTL = 10",
			flowName:    "PF-1.8.2",
			destination: ipv6DstNet,
			ipType:      ipv6,
			greEncap:    false,
			ttl:         10,
			expectDrop:  false,
		},
		{
			name:        "PF-1.8.3: IPv4 traffic with no encapsulation on DUT and TTL = 1",
			flowName:    "PF-1.8.3",
			destination: ipv4DstNet,
			ipType:      ipv4,
			greEncap:    false,
			ttl:         1,
			expectDrop:  true,
		},
		{
			name:        "PF-1.8.4: IPv6 traffic with no encapsulation on DUT and TTL = 1",
			flowName:    "PF-1.8.4",
			destination: ipv6DstNet,
			ipType:      ipv6,
			greEncap:    false,
			ttl:         1,
			expectDrop:  true,
		},
		{
			name:        "PF-1.8.5: IPv4 traffic with GRE encapsulation on DUT and TTL = 10",
			flowName:    "PF-1.8.5",
			destination: ipv4DstNetServ1,
			ipType:      ipv4,
			greEncap:    true,
			ttl:         10,
			expectDrop:  false,
		},
		{
			name:        "PF-1.8.6: IPv6 traffic with GRE encapsulation on DUT and TTL = 10",
			flowName:    "PF-1.8.6",
			destination: ipv6DstNewServ1,
			ipType:      ipv6,
			greEncap:    true,
			ttl:         10,
			expectDrop:  false,
		},
		{
			name:        "PF-1.8.7: IPv4 traffic with GRE encapsulation on DUT and TTL = 1 with DUT configured to process TTL = 1 on receiving interface",
			flowName:    "PF-1.8.7",
			destination: ipv4DstNetServ1,
			ipType:      ipv4,
			greEncap:    true,
			ttl:         1,
			expectDrop:  true,
		},
		{
			name:        "PF-1.8.8: IPv6 traffic with GRE encapsulation on DUT and TTL = 1 with DUT configured to process TTL = 1 on receiving interface",
			flowName:    "PF-1.8.8",
			destination: ipv6DstNewServ1,
			ipType:      ipv6,
			greEncap:    true,
			ttl:         1,
			expectDrop:  true,
		},
		{
			name:        "PF-1.8.9: GRE encapsulation of IPv4 traffic with TTL = 1 destined to router interface",
			flowName:    "PF-1.8.9",
			destination: dutPort1.IPv4,
			ipType:      ipv4,
			greEncap:    true,
			ttl:         1,
			expectDrop:  false,
		},
		{
			name:        "PF-1.8.10: GRE encapsulation of IPv6 traffic with TTL = 1 destined to router interface",
			flowName:    "PF-1.8.10",
			destination: dutPort1.IPv6,
			ipType:      ipv6,
			greEncap:    true,
			ttl:         1,
			expectDrop:  false,
		},
	}

	testsWithTTLCheck := []testCase{}
	testsSkipTTLCheck := []testCase{}

	for _, tc := range tests {
		if tc.ttl == 1 && !tc.expectDrop {
			testsSkipTTLCheck = append(testsSkipTTLCheck, tc)
		} else {
			testsWithTTLCheck = append(testsWithTTLCheck, tc)
		}

	}

	for _, tc := range testsWithTTLCheck {
		t.Run(tc.name, func(t *testing.T) {
			runTest(t, tc, dut, ate, top)
		})
	}
	configurePolicyRuleTTL1(t, dut)
	for _, tc := range testsSkipTTLCheck {
		t.Run(tc.name, func(t *testing.T) {
			runTest(t, tc, dut, ate, top)
		})
	}
}

func runTest(t *testing.T, tc testCase, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, config gosnappi.Config) {
	var captureState gosnappi.ControlState
	var captureFilename, capturePort string
	if tc.expectDrop {
		capturePort = "port1"
	} else {
		capturePort = "port2"
	}

	enableCapture(t, ate, config, []string{capturePort})
	defer clearCapture(t, ate, config)

	configureFlows(t, &config, tc, 0)
	otg := ate.OTG()
	otg.PushConfig(t, config)
	otg.StartProtocols(t)

	captureState = startCapture(t, ate)
	getInterfaceCounterValues(t, dut)
	otg.StartTraffic(t)
	waitForTraffic(t, otg, tc.flowName, trafficTimeout)
	otg.StopProtocols(t)

	stopCapture(t, ate, captureState)
	captureFilename = processCapture(t, ate, capturePort, tc)

	otgutils.LogFlowMetrics(t, otg, config)
	otgutils.LogPortMetrics(t, otg, config)

	verifyFlowStatistics(t, ate, tc)
	port1Intf := dut.Port(t, "port1").Name()
	port2Intf := dut.Port(t, "port2").Name()
	verifyDutInterfaceCounters(t, dut, port1Intf, ingress, noOfPackets)
	if tc.expectDrop {
		verifyIcmpMessage(t, captureFilename, tc)
	} else {
		verifyDutInterfaceCounters(t, dut, port2Intf, egress, noOfPackets)
		if tc.greEncap {
			checkPolicyStatistics(t, dut, tc)
			verifyReceivedInnerPacketTTL(t, captureFilename, tc)
		} else {
			verifyReceivedPacketTTL(t, captureFilename, tc)
		}
	}

}

func configurePolicyRuleTTL1(t *testing.T, dut *ondatra.DUTDevice) {
	dp1 := dut.Port(t, "port1")
	ttl1PolicyRules := []cfgplugins.PolicyForwardingRule{
		{
			Id:     1,
			Name:   rulev4TTL1,
			IpType: ipv4,
			TTL:    []uint8{1},
			Action: &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{
				EncapsulateGre: &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action_EncapsulateGre{
					Target: map[string]*oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action_EncapsulateGre_Target{
						nhGroupName: {
							IpTtl:       ygot.Uint8(greTTL),
							Source:      ygot.String(otgPort1.IPv4),
							Destination: ygot.String(greIpv4DstNet),
						},
					},
				},
			},
		},
		{
			Id:     1,
			Name:   rulev6TTL1,
			IpType: ipv6,
			TTL:    []uint8{1},
			Action: &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{
				EncapsulateGre: &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action_EncapsulateGre{
					Target: map[string]*oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action_EncapsulateGre_Target{
						nhGroupName: {
							IpTtl:       ygot.Uint8(greTTL),
							Source:      ygot.String(otgPort1.IPv4),
							Destination: ygot.String(greIpv4DstNet),
						},
					},
				},
			},
		},
	}
	t.Logf("Configuring policy forwarding to encap packets with ttl=1")
	_, ni, pf := cfgplugins.SetupPolicyForwardingInfraOC(deviations.DefaultNetworkInstance(dut))
	cfgplugins.NewPolicyForwardingEncapGre(t, dut, pf, trafficPolicyName, dp1.Name(), nhGroupName, ttl1PolicyRules)
	cfgplugins.ApplyPolicyToInterfaceOC(t, pf, dp1.Name(), trafficPolicyName)
	if !deviations.PolicyForwardingOCUnsupported(dut) {
		cfgplugins.PushPolicyForwardingConfig(t, dut, ni)
	}
}

func getInterfaceCounterValues(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	// Get initial interface counters for port1 and port2
	for _, intfName := range []string{dut.Port(t, "port1").Name(), dut.Port(t, "port2").Name()} {
		counterPath := gnmi.OC().Interface(intfName).Counters().State()
		counterValues := gnmi.Get(t, dut, counterPath)
		t.Logf("Interface %s in unicast packets: %d", intfName, counterValues.GetInUnicastPkts())
		interfaceCounterPackets[fmt.Sprintf("%s_ingress", intfName)] = counterValues.GetInUnicastPkts()
		t.Logf("Interface %s out unicast packets: %d", intfName, counterValues.GetOutUnicastPkts())
		interfaceCounterPackets[fmt.Sprintf("%s_egress", intfName)] = counterValues.GetOutUnicastPkts()
	}
}

func verifyFlowStatistics(t *testing.T, ate *ondatra.ATEDevice, tc testCase) {
	flowMetrics := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(tc.flowName).State())
	if *flowMetrics.Counters.OutPkts == 0 {
		t.Errorf("No packets transmitted")
	}
	if !tc.expectDrop {
		message := fmt.Sprintf("Expected %d packets, got %d", noOfPackets, *flowMetrics.Counters.InPkts)
		if *flowMetrics.Counters.InPkts != noOfPackets {
			t.Error(message)
		} else {
			t.Log(message)
		}
	} else {
		message := fmt.Sprintf("Expected 0 packets, got %d", *flowMetrics.Counters.InPkts)
		if *flowMetrics.Counters.InPkts != 0 {
			t.Error(message)
		} else {
			t.Log(message)
		}
	}
}

func verifyReceivedInnerPacketTTL(t *testing.T, captureFilename string, tc testCase) {
	if captureFilename == "" {
		t.Errorf("No capture file provided for TTL verification for testcase %s", tc.name)
		return
	}

	handle, err := pcap.OpenOffline(captureFilename)
	if err != nil {
		t.Errorf("Failed to open pcap file: %v", err)
		return
	}
	defer handle.Close()
	expectedInnerTTL := uint8(tc.ttl - 1)
	packetCount := 0
	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	for packet := range packetSource.Packets() {
		ipLayer := packet.Layer(layers.LayerTypeIPv4)
		if ipLayer == nil {
			continue
		}
		ipOuterLayer, ok := ipLayer.(*layers.IPv4)
		if !ok || ipOuterLayer == nil {
			t.Errorf("Outer IP layer not found %d", ipLayer)
			return
		}
		if ipOuterLayer.TTL != greTTL {
			t.Errorf("Expected outer IP TTL %d, got %d", greTTL, ipOuterLayer.TTL)
		}
		greLayer := packet.Layer(layers.LayerTypeGRE)
		grePacket, ok := greLayer.(*layers.GRE)
		if !ok || grePacket == nil {
			t.Error("GRE layer not found")
			return
		}
		if ipOuterLayer.Protocol != greProtocol {
			t.Errorf("Packet is not encapslated properly. Encapsulated protocol is: %d", ipOuterLayer.Protocol)
			return
		}
		innerPacket := gopacket.NewPacket(grePacket.Payload, grePacket.NextLayerType(), gopacket.Default)
		switch tc.ipType {
		case ipv4:
			ipInnerLayer := innerPacket.Layer(layers.LayerTypeIPv4)
			if ipInnerLayer == nil {
				t.Error("Inner IP layer not found")
				return
			}
			ipInnerPacket, ok := ipInnerLayer.(*layers.IPv4)
			if !ok || ipInnerPacket == nil {
				t.Error("Inner layer of type IPv4 not found")
				return
			}
			packetCount++
			if ipInnerPacket.TTL != expectedInnerTTL {
				t.Errorf("IPv4 inner packet %d: got TTL %d, want %d", packetCount, ipInnerPacket.TTL, expectedInnerTTL)
			}
		case ipv6:
			ipInnerLayer := innerPacket.Layer(layers.LayerTypeIPv6)
			if ipInnerLayer == nil {
				t.Error("Inner IP layer not found")
				return
			}
			ipInnerPacket, ok := ipInnerLayer.(*layers.IPv6)
			if !ok || ipInnerPacket == nil {
				t.Error("Inner layer of type IPv6 not found")
				return
			}
			packetCount++
			if ipInnerPacket.HopLimit != expectedInnerTTL {
				t.Errorf("IPv6 packet %d: got HopLimit %d, want %d", packetCount, ipInnerPacket.HopLimit, expectedInnerTTL)
			}
		default:
			t.Errorf("Unknown IP type %s in testcase %s", tc.ipType, tc.name)
			return
		}
	}
	if packetCount != noOfPackets {
		t.Errorf("Expected %d GRE packets, got %d", noOfPackets, packetCount)
	} else {
		t.Logf("Found %d GRE packets", packetCount)
	}
}

func verifyReceivedPacketTTL(t *testing.T, captureFilename string, tc testCase) {
	t.Helper()
	if captureFilename == "" {
		t.Errorf("No capture file provided for TTL verification for testcase %s", tc.name)
		return
	}

	handle, err := pcap.OpenOffline(captureFilename)
	if err != nil {
		t.Errorf("Failed to open pcap file: %v", err)
		return
	}
	defer handle.Close()

	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	expectedTTL := uint8(tc.ttl - 1)
	packetCount := 0
	var ttlField string

	switch tc.ipType {
	case ipv4:
		ttlField = "TTL"
		for packet := range packetSource.Packets() {
			if packet.Layer(layers.LayerTypeICMPv4) != nil {
				continue
			}
			if ipv4Layer := packet.Layer(layers.LayerTypeIPv4); ipv4Layer != nil {
				ipv4, _ := ipv4Layer.(*layers.IPv4)
				packetCount++
				if ipv4.TTL != expectedTTL {
					t.Errorf("IPv4 packet %d: got TTL %d, want %d", packetCount, ipv4.TTL, expectedTTL)
				}
			}
		}
	case ipv6:
		ttlField = "HopLimit"
		for packet := range packetSource.Packets() {
			if packet.Layer(layers.LayerTypeICMPv6) != nil {
				continue
			}
			if ipv6Layer := packet.Layer(layers.LayerTypeIPv6); ipv6Layer != nil {
				ipv6, _ := ipv6Layer.(*layers.IPv6)
				packetCount++
				if ipv6.HopLimit != expectedTTL {
					t.Errorf("IPv6 packet %d: got HopLimit %d, want %d", packetCount, ipv6.HopLimit, expectedTTL)
				}
			}
		}
	default:
		t.Errorf("Unknown IP type %s in testcase %s", tc.ipType, tc.name)
		return
	}

	if packetCount != noOfPackets {
		t.Errorf("Expected %d %s packets with %s %d, got %d", noOfPackets, tc.ipType, ttlField, expectedTTL, packetCount)
	} else {
		t.Logf("Found %d %s packets with %s %d", packetCount, tc.ipType, ttlField, expectedTTL)
	}
}

func verifyIcmpMessage(t *testing.T, captureFilename string, tc testCase) {
	t.Helper()
	if captureFilename == "" {
		t.Errorf("No capture file provided for ICMP verification")
		return
	}

	handle, err := pcap.OpenOffline(captureFilename)
	if err != nil {
		t.Errorf("Failed to open pcap file: %v", err)
		return
	}
	defer handle.Close()

	icmpPacketCount := 0
	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	icmpFound := false
	var icmpType string

	switch tc.ipType {
	case ipv4:
		icmpType = "ICMPv4"
		for packet := range packetSource.Packets() {
			// Check for IPv4 ICMP Time Exceeded (type 11)
			if ipv4Layer := packet.Layer(layers.LayerTypeIPv4); ipv4Layer != nil {
				if icmpLayer := packet.Layer(layers.LayerTypeICMPv4); icmpLayer != nil {
					icmp, _ := icmpLayer.(*layers.ICMPv4)
					if icmp.TypeCode.Type() == layers.ICMPv4TypeTimeExceeded {
						icmpFound = true
						icmpPacketCount++
					}
				}
			}
		}
	case ipv6:
		icmpType = "ICMPv6"
		for packet := range packetSource.Packets() {
			// Check for IPv6 ICMP Time Exceeded (type 3)
			if ipv6Layer := packet.Layer(layers.LayerTypeIPv6); ipv6Layer != nil {
				if icmp6Layer := packet.Layer(layers.LayerTypeICMPv6); icmp6Layer != nil {
					icmp6, _ := icmp6Layer.(*layers.ICMPv6)
					if icmp6.TypeCode.Type() == layers.ICMPv6TypeTimeExceeded {
						icmpFound = true
						icmpPacketCount++
					}
				}
			}
		}
	default:
		t.Errorf("Unknown IP type %s in testcase %s", tc.ipType, tc.name)
		return
	}
	if !icmpFound {
		t.Errorf("No %s Time Exceeded messages found in capture", icmpType)
	}
	if icmpPacketCount != noOfPackets {
		t.Errorf("Expected %d %s Time Exceeded messages, got %d", noOfPackets, icmpType, icmpPacketCount)
	} else {
		t.Logf("Found %d %s Time Exceeded messages in capture", icmpPacketCount, icmpType)
	}
}

func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")

	t.Logf("Configuring Interfaces")
	configureDUTPort(t, dut, &dutPort1, dp1)
	configureDUTPort(t, dut, &dutPort2, dp2)

	t.Logf("Configuring static routes")
	configureStaticRoutes(t, dut)

	t.Logf("Configuring Hardware Init")
	configureHardwareInit(t, dut)

	policyRules := []cfgplugins.PolicyForwardingRule{
		{
			Id:                 1,
			Name:               rulev4,
			IpType:             ipv4,
			DestinationAddress: fmt.Sprintf("%s/%d", ipv4DstNetServ1, ipv4PrefixLen),
			Action: &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{
				EncapsulateGre: &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action_EncapsulateGre{
					Target: map[string]*oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action_EncapsulateGre_Target{
						nhGroupName: {
							IpTtl:       ygot.Uint8(greTTL),
							Source:      ygot.String(otgPort1.IPv4),
							Destination: ygot.String(greIpv4DstNet),
						},
					},
				},
			},
		},
		{
			Id:                 2,
			Name:               rulev6,
			IpType:             ipv6,
			DestinationAddress: fmt.Sprintf("%s/%d", ipv6DstNewServ1, ipv6PrefixLen),
			Action: &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{
				EncapsulateGre: &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action_EncapsulateGre{
					Target: map[string]*oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action_EncapsulateGre_Target{
						nhGroupName: {
							IpTtl:       ygot.Uint8(greTTL),
							Source:      ygot.String(otgPort1.IPv4),
							Destination: ygot.String(greIpv4DstNet),
						},
					},
				},
			},
		},
	}

	t.Logf("Configuring policy forwarding")
	_, ni, pf := cfgplugins.SetupPolicyForwardingInfraOC(deviations.DefaultNetworkInstance(dut))
	cfgplugins.NewPolicyForwardingEncapGre(t, dut, pf, trafficPolicyName, dp1.Name(), nhGroupName, policyRules)
	cfgplugins.ApplyPolicyToInterfaceOC(t, pf, dp1.Name(), trafficPolicyName)
	if !deviations.PolicyForwardingOCUnsupported(dut) {
		cfgplugins.PushPolicyForwardingConfig(t, dut, ni)
	}
}

func waitForTraffic(t *testing.T, otg *otg.OTG, flowName string, timeout time.Duration) {
	transmitPath := gnmi.OTG().Flow(flowName).Transmit().State()
	_, ok := gnmi.Watch(t, otg, transmitPath, timeout, func(val *ygnmi.Value[bool]) bool {
		transmitState, present := val.Val()
		return present && !transmitState
	}).Await(t)

	if !ok {
		t.Errorf("Traffic for flow %s did not stop within the timeout of %d", flowName, timeout)
	} else {
		t.Logf("Traffic for flow %s has stopped", flowName)
	}
}

func configureDUTPort(t *testing.T, dut *ondatra.DUTDevice, attrs *attrs.Attributes, p *ondatra.Port) {
	t.Helper()
	d := gnmi.OC()
	i := attrs.NewOCInterface(p.Name(), dut)
	i.Description = ygot.String(attrs.Desc)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	if deviations.InterfaceEnabled(dut) {
		i.Enabled = ygot.Bool(true)
	}

	i.GetOrCreateEthernet()
	i4 := i.GetOrCreateSubinterface(0).GetOrCreateIpv4()
	i4.Enabled = ygot.Bool(true)
	a := i4.GetOrCreateAddress(attrs.IPv4)
	a.PrefixLength = ygot.Uint8(attrs.IPv4Len)

	i6 := i.GetOrCreateSubinterface(0).GetOrCreateIpv6()
	i6.Enabled = ygot.Bool(true)
	a6 := i6.GetOrCreateAddress(attrs.IPv6)
	a6.PrefixLength = ygot.Uint8(attrs.IPv6Len)

	gnmi.Replace(t, dut, d.Interface(p.Name()).Config(), i)
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, p.Name(), deviations.DefaultNetworkInstance(dut), 0)
		t.Logf("DUT %s %s %s requires explicit interface in default VRF deviation ", dut.Vendor(), dut.Model(), dut.Version())
	}

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, p)
	}
}

func configureHardwareInit(t *testing.T, dut *ondatra.DUTDevice) {
	hardwareInitCfg := cfgplugins.NewDUTHardwareInit(t, dut, cfgplugins.FeaturePolicyForwarding)
	if hardwareInitCfg == "" {
		return
	}
	cfgplugins.PushDUTHardwareInitConfig(t, dut, hardwareInitCfg)
}

func configureFlows(t *testing.T, config *gosnappi.Config, tc testCase, dscp uint32) {
	(*config).Flows().Clear()
	flow := (*config).Flows().Add().SetName(tc.flowName)
	flow.Metrics().SetEnable(true)
	flow.TxRx().Device().SetTxNames([]string{fmt.Sprintf("%s.%s", otgPort1.Name, tc.ipType)}).SetRxNames([]string{fmt.Sprintf("%s.%s", otgPort2.Name, tc.ipType)})
	flow.Size().SetFixed(trafficFrameSize)
	flow.Rate().SetPps(trafficRatePps)
	flow.Duration().SetFixedPackets(gosnappi.NewFlowFixedPackets().SetPackets(noOfPackets))

	eth := flow.Packet().Add().Ethernet()
	eth.Src().SetValue(otgPort1.MAC)

	switch tc.ipType {
	case ipv4:
		ipv4 := flow.Packet().Add().Ipv4()
		ipv4.Src().SetValue(otgPort1.IPv4)
		ipv4.Dst().SetValue(tc.destination)
		ipv4.Priority().Dscp().Phb().SetValue(dscp)
		ipv4.TimeToLive().SetValue(uint32(tc.ttl))
	case ipv6:
		ipv6 := flow.Packet().Add().Ipv6()
		ipv6.Src().SetValue(otgPort1.IPv6)
		ipv6.Dst().SetValue(tc.destination)
		ipv6.TrafficClass().SetValue(dscp << 2)
		ipv6.HopLimit().SetValue(uint32(tc.ttl))
	default:
		t.Errorf("Invalid traffic type %s", tc.ipType)
	}
}

func checkPolicyStatistics(t *testing.T, dut *ondatra.DUTDevice, tc testCase) {
	if deviations.PolicyForwardingGreEncapsulationOcUnsupported(dut) || deviations.PolicyForwardingToNextHopOcUnsupported(dut) {
		t.Errorf("Dut %s %s %s does not support checking policy statistics through OC", dut.Vendor(), dut.Model(), dut.Version())
	} else {
		checkPolicyStatisticsFromOC(t, dut, tc)
	}
}

func checkPolicyStatisticsFromOC(t *testing.T, dut *ondatra.DUTDevice, tc testCase) {
	totalMatched := gnmi.Get(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding().Policy(trafficPolicyName).Rule(uint32(1)).MatchedPkts().State())
	previouslyMatched := ruleMatchedPackets[trafficPolicyName]
	if totalMatched != previouslyMatched+noOfPackets {
		t.Errorf("Expected %d packets matched by policy %s rule %s for flow %s, but got %d", noOfPackets, trafficPolicyName, trafficPolicyName, tc.flowName, totalMatched-previouslyMatched)
	}
	ruleMatchedPackets[trafficPolicyName] = totalMatched
}

func verifyDutInterfaceCounters(t *testing.T, dut *ondatra.DUTDevice, intfName string, direction bool, expectedPkts uint64) {
	counterValues := gnmi.Get(t, dut, gnmi.OC().Interface(intfName).Counters().State())
	var intfKey, packetType string
	var total uint64
	switch direction {
	case ingress:
		intfKey = fmt.Sprintf("%s_ingress", intfName)
		t.Logf("Verifying ingress counters for interface %s", intfName)
		total = counterValues.GetInUnicastPkts()
		packetType = "in-unicast-pkts"
	case egress:
		intfKey = fmt.Sprintf("%s_egress", intfName)
		t.Logf("Verifying egress counters for interface %s", intfName)
		total = counterValues.GetOutUnicastPkts()
		packetType = "out-unicast-pkts"
	}
	got := total - interfaceCounterPackets[intfKey]
	message := fmt.Sprintf("Interface %s %s: got %d, want %d", intfName, packetType, got, expectedPkts)
	if got != expectedPkts {
		t.Error(message)
	} else {
		t.Log(message)
	}
	interfaceCounterPackets[intfKey] = total
}

func enableCapture(t *testing.T, ate *ondatra.ATEDevice, topo gosnappi.Config, otgPortNames []string) {
	t.Helper()
	for _, port := range otgPortNames {
		t.Log("Enabling capture on ", port)
		topo.Captures().Add().SetName(port).SetPortNames([]string{port}).SetFormat(gosnappi.CaptureFormat.PCAP)
	}

	pb, _ := topo.Marshal().ToProto()
	t.Log(pb.GetCaptures())
	ate.OTG().PushConfig(t, topo)
}

func clearCapture(t *testing.T, ate *ondatra.ATEDevice, topo gosnappi.Config) {
	t.Helper()
	topo.Captures().Clear()
	ate.OTG().PushConfig(t, topo)
}

func startCapture(t *testing.T, ate *ondatra.ATEDevice) gosnappi.ControlState {
	t.Helper()
	cs := gosnappi.NewControlState()
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.START)
	ate.OTG().SetControlState(t, cs)
	return cs
}

func processCapture(t *testing.T, ate *ondatra.ATEDevice, capturePort string, tc testCase) string {
	otg := ate.OTG()
	bytes := otg.GetCapture(t, gosnappi.NewCaptureRequest().SetPortName(capturePort))
	if len(bytes) == 0 {
		t.Errorf("Empty capture received for flow %s on port %s", tc.flowName, capturePort)
		return ""
	}
	f, err := os.Create(captureFilePath)
	if err != nil {
		t.Errorf("Could not create temporary pcap file: %v\n", err)
		return ""
	}
	defer f.Close()
	if _, err := f.Write(bytes); err != nil {
		t.Errorf("Could not write bytes to pcap file: %v\n", err)
		return ""
	}

	return f.Name()
}

func stopCapture(t *testing.T, ate *ondatra.ATEDevice, cs gosnappi.ControlState) {
	t.Helper()
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.STOP)
	ate.OTG().SetControlState(t, cs)
	time.Sleep(5 * time.Second)
}

func configureStaticRoutes(t *testing.T, dut *ondatra.DUTDevice) {
	index := 0
	for source, dest := range staticRoutes {
		configStaticRoute(t, dut, source, dest, strconv.Itoa(index))
		index++
	}
}

func configStaticRoute(t *testing.T, dut *ondatra.DUTDevice, prefix string, nexthop string, index string) {
	b := &gnmi.SetBatch{}
	if nexthop == "Null0" {
		nexthop = "DROP"
	}
	routeCfg := &cfgplugins.StaticRouteCfg{
		NetworkInstance: deviations.DefaultNetworkInstance(dut),
		Prefix:          prefix,
		NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
			index: oc.UnionString(nexthop),
		},
	}
	if _, err := cfgplugins.NewStaticRouteCfg(b, routeCfg, dut); err != nil {
		t.Fatalf("Failed to configure static route: %v", err)
	}
	b.Set(t, dut)
}
