// Copyright 2022 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package encap_test implements TE-16.1 of the dcgate vendor testplan
package encap_test

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cisco/config"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	ipipProtocol         = 4
	ipv6ipProtocol       = 41
	udpProtocol          = 17
	ethertypeIPv4        = oc.PacketMatchTypes_ETHERTYPE_ETHERTYPE_IPV4
	ethertypeIPv6        = oc.PacketMatchTypes_ETHERTYPE_ETHERTYPE_IPV6
	clusterPolicy        = "vrf_selection_policy_c"
	wanPolicy            = "vrf_selection_policy_w"
	vrfDecap             = "DECAP_TE_VRF"
	vrfTransit           = "TE_VRF_111"
	vrfRepaired          = "TE_VRF_222"
	vrfEncapA            = "ENCAP_TE_VRF_A"
	vrfEncapB            = "ENCAP_TE_VRF_B"
	vrfDecapPostRepaired = "DECAP"
	ipv4PrefixLen        = 30
	ipv6PrefixLen        = 126
	trafficDuration      = 15 * time.Second
	nhg10ID              = 10
	nh201ID              = 201
	nh202ID              = 202
	nhg1ID               = 1
	nh1ID                = 1
	nh2ID                = 2
	nhg2ID               = 2
	nh10ID               = 10
	nh11ID               = 11
	nhg3ID               = 3
	nh100ID              = 100
	nh101ID              = 101
	dscpEncapA1          = 10
	dscpEncapA2          = 18
	dscpEncapB1          = 20
	dscpEncapB2          = 28
	dscpEncapNoMatch     = 30
	magicMac             = "02:00:00:00:00:01"
	tunnelDstIP1         = "203.0.113.1"
	tunnelDstIP2         = "203.0.113.2"
	tunnelDstIP3         = "203.0.113.100"
	ipv4OuterSrc111      = "198.51.100.111"
	ipv4OuterSrc222      = "198.51.100.222"
	ipv4OuterSrcIpInIp   = "198.100.200.123"
	vipIP1               = "192.0.2.111"
	vipIP2               = "192.0.2.222"
	vipIP3               = "192.0.2.133"
	innerV4DstIP         = "198.18.1.1"
	innerV4SrcIP         = "198.18.0.255"
	InnerV6SrcIP         = "2001:DB8::198:1"
	InnerV6DstIP         = "2001:DB8:2:0:192::10"
	ipv4FlowIP           = "138.0.11.8"
	ipv4EntryPrefix      = "138.0.11.0"
	ipv4EntryPrefixLen   = 24
	ipv6FlowIP           = "2015:aa8::1"
	ipv6EntryPrefix      = "2015:aa8::"
	ipv6EntryPrefixLen   = 32
	ratioTunEncap1       = 0.25 // 1/4
	ratioTunEncap2       = 0.75 // 3/4
	ratioTunEncapTol     = 0.05 // 5/100
	ttl                  = uint32(100)
	trfDistTolerance     = 0.02
	// observing on IXIA OTG: Cannot start capture on more than one port belonging to the
	// same resource group or on more than one port behind the same front panel port in the chassis
	otgMutliPortCaptureSupported     = false
	ipv4PrefixDoesNotExistInEncapVrf = "140.0.0.1"
	ipv6PrefixDoesNotExistInEncapVrf = "2016::140:0:0:1"
)

var (
	otgDstPorts = []string{"port2", "port3", "port4", "port5"}
	otgSrcPort  = "port1"
	wantWeights = []float64{
		0.0625, // 1/4 * 1/4 - port2
		0.1875, // 1/4 * 3/4 - port3
		0.3,    // 3/4 * 2/5 - port4
		0.45,   // 3/5 * 3/4 - port5
	}
	noMatchWeight = []float64{
		1, 0, 0, 0,
	}
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "dutPort1",
		MAC:     "02:01:00:00:00:01",
		IPv4:    "192.0.2.1",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:1",
		IPv6Len: ipv6PrefixLen,
	}

	otgPort1 = attrs.Attributes{
		Name:    "otgPort1",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.0.2.2",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:2",
		IPv6Len: ipv6PrefixLen,
	}

	dutPort2 = attrs.Attributes{
		Desc:    "dutPort2",
		IPv4:    "192.0.2.5",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:5",
		IPv6Len: ipv6PrefixLen,
	}

	otgPort2 = attrs.Attributes{
		Name:    "otgPort2",
		MAC:     "02:00:02:01:01:01",
		IPv4:    "192.0.2.6",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:6",
		IPv6Len: ipv6PrefixLen,
	}

	dutPort3 = attrs.Attributes{
		Desc:    "dutPort3",
		IPv4:    "192.0.2.9",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:9",
		IPv6Len: ipv6PrefixLen,
	}

	otgPort3 = attrs.Attributes{
		Name:    "otgPort3",
		MAC:     "02:00:03:01:01:01",
		IPv4:    "192.0.2.10",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:a",
		IPv6Len: ipv6PrefixLen,
	}

	dutPort4 = attrs.Attributes{
		Desc:    "dutPort4",
		IPv4:    "192.0.2.13",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:D",
		IPv6Len: ipv6PrefixLen,
	}

	otgPort4 = attrs.Attributes{
		Name:    "otgPort4",
		MAC:     "02:00:04:01:01:01",
		IPv4:    "192.0.2.14",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:E",
		IPv6Len: ipv6PrefixLen,
	}

	dutPort5 = attrs.Attributes{
		Desc:    "dutPort5",
		IPv4:    "192.0.2.17",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:11",
		IPv6Len: ipv6PrefixLen,
	}

	otgPort5 = attrs.Attributes{
		Name:    "otgPort5",
		MAC:     "02:00:05:01:01:01",
		IPv4:    "192.0.2.18",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:12",
		IPv6Len: ipv6PrefixLen,
	}

	dutPort2DummyIP = attrs.Attributes{
		Desc:    "dutPort2",
		IPv4:    "192.0.2.21",
		IPv4Len: ipv4PrefixLen,
	}

	otgPort2DummyIP = attrs.Attributes{
		Desc:    "otgPort2",
		IPv4:    "192.0.2.22",
		IPv4Len: ipv4PrefixLen,
	}

	dutPort3DummyIP = attrs.Attributes{
		Desc:    "dutPort3",
		IPv4:    "192.0.2.25",
		IPv4Len: ipv4PrefixLen,
	}

	otgPort3DummyIP = attrs.Attributes{
		Desc:    "otgPort3",
		IPv4:    "192.0.2.26",
		IPv4Len: ipv4PrefixLen,
	}

	dutPort4DummyIP = attrs.Attributes{
		Desc:    "dutPort4",
		IPv4:    "192.0.2.29",
		IPv4Len: ipv4PrefixLen,
	}

	otgPort4DummyIP = attrs.Attributes{
		Desc:    "otgPort4",
		IPv4:    "192.0.2.30",
		IPv4Len: ipv4PrefixLen,
	}

	dutPort5DummyIP = attrs.Attributes{
		Desc:    "dutPort5",
		IPv4:    "192.0.2.33",
		IPv4Len: ipv4PrefixLen,
	}

	otgPort5DummyIP = attrs.Attributes{
		Desc:    "otgPort5",
		IPv4:    "192.0.2.34",
		IPv4Len: ipv4PrefixLen,
	}
)

type pbrRule struct {
	sequence    uint32
	protocol    uint8
	srcAddr     string
	dscpSet     []uint8
	dscpSetV6   []uint8
	decapVrfSet []string
	encapVrf    string
	etherType   oc.NetworkInstance_PolicyForwarding_Policy_Rule_L2_Ethertype_Union
}

type packetAttr struct {
	dscp     int
	protocol int
	ttl      uint32
}

type flowAttr struct {
	src      string   // source IP address
	dst      string   // destination IP address
	srcPort  string   // source OTG port
	dstPorts []string // destination OTG ports
	srcMac   string   // source MAC address
	dstMac   string   // destination MAC address
	dscp     uint32
	topo     gosnappi.Config
}

var (
	fa4 = flowAttr{
		src:      otgPort1.IPv4,
		dst:      ipv4FlowIP,
		srcMac:   otgPort1.MAC,
		dstMac:   dutPort1.MAC,
		srcPort:  otgSrcPort,
		dstPorts: otgDstPorts,
		topo:     gosnappi.NewConfig(),
	}
	fa6 = flowAttr{
		src:      otgPort1.IPv6,
		dst:      ipv6FlowIP,
		srcMac:   otgPort1.MAC,
		dstMac:   dutPort1.MAC,
		srcPort:  otgSrcPort,
		dstPorts: otgDstPorts,
		topo:     gosnappi.NewConfig(),
	}
	faIPinIP = flowAttr{
		src:      ipv4OuterSrcIpInIp,
		dst:      ipv4FlowIP,
		srcMac:   otgPort1.MAC,
		dstMac:   dutPort1.MAC,
		srcPort:  otgSrcPort,
		dstPorts: otgDstPorts,
		topo:     gosnappi.NewConfig(),
	}
	fa4NoPrefix = flowAttr{
		src:      otgPort1.IPv4,
		dst:      ipv4PrefixDoesNotExistInEncapVrf,
		srcMac:   otgPort1.MAC,
		dstMac:   dutPort1.MAC,
		srcPort:  otgSrcPort,
		dstPorts: otgDstPorts,
		topo:     gosnappi.NewConfig(),
	}
	fa6NoPrefix = flowAttr{
		src:      otgPort1.IPv6,
		dst:      ipv6PrefixDoesNotExistInEncapVrf,
		srcMac:   otgPort1.MAC,
		dstMac:   dutPort1.MAC,
		srcPort:  otgSrcPort,
		dstPorts: otgDstPorts,
		topo:     gosnappi.NewConfig(),
	}
	faTransit = flowAttr{
		src:      ipv4OuterSrc111,
		dst:      tunnelDstIP1,
		srcMac:   otgPort1.MAC,
		dstMac:   dutPort1.MAC,
		srcPort:  otgSrcPort,
		dstPorts: otgDstPorts,
		topo:     gosnappi.NewConfig(),
	}
)

// testArgs holds the objects needed by a test case.
type testArgs struct {
	dut    *ondatra.DUTDevice
	ate    *ondatra.ATEDevice
	topo   gosnappi.Config
	client *gribi.Client
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestBasicEncap(t *testing.T) {
	// Configure DUT
	dut := ondatra.DUT(t, "dut")
	configureDUT(t, dut, true)
	defer gnmi.Delete(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding().Config())

	// Configure ATE
	otg := ondatra.ATE(t, "ate")
	topo := configureOTG(t, otg)

	// configure gRIBI client
	c := gribi.Client{
		DUT:         dut,
		FIBACK:      true,
		Persistence: true,
	}

	if err := c.Start(t); err != nil {
		t.Fatalf("gRIBI Connection can not be established")
	}

	defer c.Close(t)
	c.BecomeLeader(t)

	// Flush all existing AFT entries on the router
	c.FlushAll(t)

	programEntries(t, dut, &c)

	test := []struct {
		name               string
		pattr              packetAttr
		flows              []gosnappi.Flow
		weights            []float64
		capturePorts       []string
		validateEncapRatio bool
	}{
		{
			name:               fmt.Sprintf("Test1 IPv4 Traffic WCMP Encap dscp %d", dscpEncapA1),
			pattr:              packetAttr{dscp: dscpEncapA1, protocol: ipipProtocol},
			flows:              []gosnappi.Flow{fa4.getFlow("ipv4", "ip4a1", dscpEncapA1)},
			weights:            wantWeights,
			capturePorts:       otgDstPorts,
			validateEncapRatio: true,
		},
		{
			name:               fmt.Sprintf("Test1 IPv4 Traffic WCMP Encap dscp %d", dscpEncapB1),
			pattr:              packetAttr{dscp: dscpEncapB1, protocol: ipipProtocol},
			flows:              []gosnappi.Flow{fa4.getFlow("ipv4", "ip4b1", dscpEncapB1)},
			weights:            wantWeights,
			capturePorts:       otgDstPorts,
			validateEncapRatio: true,
		},
		{
			name:               fmt.Sprintf("Test2 IPv6 Traffic WCMP Encap dscp %d", dscpEncapA1),
			pattr:              packetAttr{dscp: dscpEncapA1, protocol: ipv6ipProtocol},
			flows:              []gosnappi.Flow{fa6.getFlow("ipv6", "ip6a1", dscpEncapA1)},
			weights:            wantWeights,
			capturePorts:       otgDstPorts,
			validateEncapRatio: true,
		},
		{
			name:               fmt.Sprintf("Test2 IPv6 Traffic WCMP Encap dscp %d", dscpEncapB1),
			pattr:              packetAttr{dscp: dscpEncapB1, protocol: ipv6ipProtocol},
			flows:              []gosnappi.Flow{fa6.getFlow("ipv6", "ip6b1", dscpEncapB1)},
			weights:            wantWeights,
			capturePorts:       otgDstPorts,
			validateEncapRatio: true,
		},
		{
			name:  fmt.Sprintf("Test3 IPinIP Traffic WCMP Encap dscp %d", dscpEncapA1),
			pattr: packetAttr{dscp: dscpEncapA1, protocol: ipipProtocol},
			flows: []gosnappi.Flow{faIPinIP.getFlow("ipv4in4", "ip4in4a1", dscpEncapA1),
				faIPinIP.getFlow("ipv6in4", "ip6in4a1", dscpEncapA1),
			},
			weights:            wantWeights,
			capturePorts:       otgDstPorts,
			validateEncapRatio: true,
		},
		{
			name:               fmt.Sprintf("No Match Dscp %d Traffic", dscpEncapNoMatch),
			pattr:              packetAttr{protocol: udpProtocol, dscp: dscpEncapNoMatch},
			flows:              []gosnappi.Flow{fa4.getFlow("ipv4", "ip4nm", dscpEncapNoMatch)},
			weights:            noMatchWeight,
			capturePorts:       otgDstPorts[:1],
			validateEncapRatio: false,
		},
		{
			name:               fmt.Sprintf("IPv4 No Prefix In Encap Vrf %d Traffic", dscpEncapA1),
			pattr:              packetAttr{protocol: udpProtocol, dscp: dscpEncapA1},
			flows:              []gosnappi.Flow{fa4NoPrefix.getFlow("ipv4", "ip4NoPrefixEncapVrf", dscpEncapA1)},
			weights:            noMatchWeight,
			capturePorts:       otgDstPorts[:1],
			validateEncapRatio: false,
		},
		{
			name:               fmt.Sprintf("IPv6 No Prefix In Encap Vrf %d Traffic", dscpEncapA1),
			pattr:              packetAttr{protocol: udpProtocol, dscp: dscpEncapA1},
			flows:              []gosnappi.Flow{fa6NoPrefix.getFlow("ipv6", "ip6NoPrefixEncapVrf", dscpEncapA1)},
			weights:            noMatchWeight,
			capturePorts:       otgDstPorts[:1],
			validateEncapRatio: false,
		},
	}

	tcArgs := &testArgs{
		client: &c,
		dut:    dut,
		ate:    otg,
		topo:   topo,
	}

	for _, tc := range test {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Name: %s", tc.name)
			if strings.Contains(tc.name, "No Match Dscp") {
				configDefaultRoute(t, dut, cidr(ipv4EntryPrefix, ipv4EntryPrefixLen), otgPort2.IPv4, cidr(ipv6EntryPrefix, ipv6EntryPrefixLen), otgPort2.IPv6)
				defer gnmi.Delete(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut)).Static(cidr(ipv4EntryPrefix, ipv4EntryPrefixLen)).Config())
				defer gnmi.Delete(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut)).Static(cidr(ipv6EntryPrefix, ipv6EntryPrefixLen)).Config())
			}
			if strings.Contains(tc.name, "No Prefix In Encap Vrf") {
				configDefaultIPStaticCli(t, dut, []string{vrfEncapA})
				defer unConfigDefaultIPStaticCli(t, dut, []string{vrfEncapA})
				configDefaultRoute(t, dut, cidr(ipv4PrefixDoesNotExistInEncapVrf, 32), otgPort2.IPv4, cidr(ipv6PrefixDoesNotExistInEncapVrf, 128), otgPort2.IPv6)
				defer gnmi.Delete(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut)).Static(cidr(ipv4PrefixDoesNotExistInEncapVrf, 32)).Config())
				defer gnmi.Delete(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut)).Static(cidr(ipv6PrefixDoesNotExistInEncapVrf, 128)).Config())
			}
			if otgMutliPortCaptureSupported {
				enableCapture(t, otg.OTG(), topo, tc.capturePorts)
				t.Log("Start capture and send traffic")
				sendTraffic(t, tcArgs, tc.flows, true)
				t.Log("Validate captured packet attributes")
				var tunCounter = validatePacketCapture(t, tcArgs, tc.capturePorts, &tc.pattr)
				if tc.validateEncapRatio {
					validateTunnelEncapRatio(t, tunCounter)
				}
				clearCapture(t, otg.OTG(), topo)
			} else {
				for _, port := range tc.capturePorts {
					enableCapture(t, otg.OTG(), topo, []string{port})
					t.Log("Start capture and send traffic")
					sendTraffic(t, tcArgs, tc.flows, true)
					t.Log("Validate captured packet attributes")
					var tunCounter = validatePacketCapture(t, tcArgs, []string{port}, &tc.pattr)
					if tc.validateEncapRatio {
						validateTunnelEncapRatio(t, tunCounter)
					}
					clearCapture(t, otg.OTG(), topo)
				}
			}
			t.Log("Validate traffic flows")
			validateTrafficFlows(t, tcArgs, tc.flows, false, true)
			t.Log("Validate hierarchical traffic distribution")
			validateTrafficDistribution(t, otg, tc.weights)
		})
	}
}

// getPbrRules returns pbrRule slice for cluster facing (clusterFacing = true) or wan facing
// interface (clusterFacing = false)
func getPbrRules(dut *ondatra.DUTDevice, clusterFacing bool) []pbrRule {
	vrfDefault := deviations.DefaultNetworkInstance(dut)
	var pbrRules = []pbrRule{
		{
			sequence:    1,
			protocol:    ipipProtocol,
			dscpSet:     []uint8{dscpEncapA1, dscpEncapA2},
			decapVrfSet: []string{vrfDecap, vrfEncapA, vrfRepaired},
			srcAddr:     ipv4OuterSrc222,
		},
		{
			sequence:    2,
			protocol:    ipv6ipProtocol,
			dscpSet:     []uint8{dscpEncapA1, dscpEncapA2},
			decapVrfSet: []string{vrfDecap, vrfEncapA, vrfRepaired},
			srcAddr:     ipv4OuterSrc222,
		},
		{
			sequence:    3,
			protocol:    ipipProtocol,
			dscpSet:     []uint8{dscpEncapA1, dscpEncapA2},
			decapVrfSet: []string{vrfDecap, vrfEncapA, vrfTransit},
			srcAddr:     ipv4OuterSrc111,
		},
		{
			sequence:    4,
			protocol:    ipv6ipProtocol,
			dscpSet:     []uint8{dscpEncapA1, dscpEncapA2},
			decapVrfSet: []string{vrfDecap, vrfEncapA, vrfTransit},
			srcAddr:     ipv4OuterSrc111,
		},
		{
			sequence:    5,
			protocol:    ipipProtocol,
			dscpSet:     []uint8{dscpEncapB1, dscpEncapB2},
			decapVrfSet: []string{vrfDecap, vrfEncapB, vrfRepaired},
			srcAddr:     ipv4OuterSrc222,
		},
		{
			sequence:    6,
			protocol:    ipv6ipProtocol,
			dscpSet:     []uint8{dscpEncapB1, dscpEncapB2},
			decapVrfSet: []string{vrfDecap, vrfEncapB, vrfRepaired},
			srcAddr:     ipv4OuterSrc222,
		},
		{
			sequence:    7,
			protocol:    ipipProtocol,
			dscpSet:     []uint8{dscpEncapB1, dscpEncapB2},
			decapVrfSet: []string{vrfDecap, vrfEncapB, vrfTransit},
			srcAddr:     ipv4OuterSrc111,
		},
		{
			sequence:    8,
			protocol:    ipv6ipProtocol,
			dscpSet:     []uint8{dscpEncapB1, dscpEncapB2},
			decapVrfSet: []string{vrfDecap, vrfEncapB, vrfTransit},
			srcAddr:     ipv4OuterSrc111,
		},
		{
			sequence:    9,
			protocol:    ipipProtocol,
			decapVrfSet: []string{vrfDecap, vrfDefault, vrfRepaired},
			srcAddr:     ipv4OuterSrc222,
		},
		{
			sequence:    10,
			protocol:    ipv6ipProtocol,
			decapVrfSet: []string{vrfDecap, vrfDefault, vrfRepaired},
			srcAddr:     ipv4OuterSrc222,
		},
		{
			sequence:    11,
			protocol:    ipipProtocol,
			decapVrfSet: []string{vrfDecap, vrfDefault, vrfTransit},
			srcAddr:     ipv4OuterSrc111,
		},
		{
			sequence:    12,
			protocol:    ipv6ipProtocol,
			decapVrfSet: []string{vrfDecap, vrfDefault, vrfTransit},
			srcAddr:     ipv4OuterSrc111,
		},
	}

	var encapRules = []pbrRule{
		{
			sequence: 13,
			dscpSet:  []uint8{dscpEncapA1, dscpEncapA2},
			encapVrf: vrfEncapA,
		},
		{
			sequence:  14,
			dscpSetV6: []uint8{dscpEncapA1, dscpEncapA2},
			encapVrf:  vrfEncapA,
		},
		{
			sequence: 15,
			dscpSet:  []uint8{dscpEncapB1, dscpEncapB2},
			encapVrf: vrfEncapB,
		},
		{
			sequence:  16,
			dscpSetV6: []uint8{dscpEncapB1, dscpEncapB2},
			encapVrf:  vrfEncapB,
		},
	}

	// var defaultClassRule = []pbrRule{
	// 	{
	// 		sequence: 17,
	// 		encapVrf: vrfDefault,
	// 	},
	// }

	var splitDefaultClassRules = []pbrRule{
		{
			sequence:  17,
			etherType: ethertypeIPv4,
			encapVrf:  vrfDefault,
		},
		{
			sequence:  18,
			etherType: ethertypeIPv6,
			encapVrf:  vrfDefault,
		},
	}

	if clusterFacing {
		pbrRules = append(pbrRules, encapRules...)
	}

	pbrRules = append(pbrRules, splitDefaultClassRules...)

	// if deviations.PfDefaultRuleVariableSequenceUnsupported(dut) {
	// pbrRules = append(pbrRules, splitDefaultClassRules...)
	// } else {
	// 	pbrRules = append(pbrRules, defaultClassRule...)
	// }

	return pbrRules
}

// configDefaultRoute configures a static route in DEFAULT network-instance.
func configDefaultRoute(t *testing.T, dut *ondatra.DUTDevice, v4Prefix, v4NextHop, v6Prefix, v6NextHop string) {
	t.Logf("Configuring static route in DEFAULT network-instance")
	ni := oc.NetworkInstance{Name: ygot.String(deviations.DefaultNetworkInstance(dut))}
	static := ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
	sr := static.GetOrCreateStatic(v4Prefix)
	nh := sr.GetOrCreateNextHop("0")
	nh.NextHop = oc.UnionString(v4NextHop)
	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut)).Config(), static)
	sr = static.GetOrCreateStatic(v6Prefix)
	nh = sr.GetOrCreateNextHop("0")
	nh.NextHop = oc.UnionString(v6NextHop)
	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut)).Config(), static)
}

// configDefaultRoute configures a static route in DEFAULT network-instance.
func configIPv4DefaultRoute(t *testing.T, dut *ondatra.DUTDevice, v4Prefix, v4NextHop string) {
	t.Logf("Configuring static route in DEFAULT network-instance")
	ni := oc.NetworkInstance{Name: ygot.String(deviations.DefaultNetworkInstance(dut))}
	static := ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
	sr := static.GetOrCreateStatic(v4Prefix)
	nh := sr.GetOrCreateNextHop("0")
	nh.NextHop = oc.UnionString(v4NextHop)
	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut)).Config(), static)
}

// configDefaultRoute configures a static route in DEFAULT network-instance.
func configDefaultRouteInVrf(t *testing.T, vrf string, dut *ondatra.DUTDevice, v4Prefix, v4NextHop, v6Prefix, v6NextHop string) {
	t.Logf("Configuring static route in DEFAULT network-instance")
	ni := oc.NetworkInstance{Name: ygot.String(vrf)}
	static := ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
	sr := static.GetOrCreateStatic(v4Prefix)
	//nh := sr.GetOrCreateNextHop("0")
	nh := sr.GetOrCreateNextHop("default")
	//nh.To_NetworkInstance_Protocol_Static_NextHop_NextHop_Union("DEFAULT")
	//nh.NextHop = oc.UnionString(v4NextHop)
	nh.SetNextHop(oc.UnionString("default"))
	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(vrf).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut)).Config(), static)
	sr = static.GetOrCreateStatic(v6Prefix)
	//nh = sr.GetOrCreateNextHop("0")
	nh = sr.GetOrCreateNextHop("default")
	//nh.NextHop = oc.UnionString(v6NextHop)
	nh.SetNextHop(oc.UnionString("default"))
	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(vrf).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut)).Config(), static)
}

// configureNetworkInstance creates nonDefaultVRFs
func configureNetworkInstance(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	c := &oc.Root{}
	vrfs := []string{vrfDecap, vrfTransit, vrfRepaired, vrfEncapA, vrfEncapB, vrfDecapPostRepaired}
	for _, vrf := range vrfs {
		ni := c.GetOrCreateNetworkInstance(vrf)
		ni.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF
		// if vrf == vrfEncapA || vrf == vrfEncapB {
		// 	ni.FallbackNetworkInstance = ygot.String("default") //deviations.DefaultNetworkInstance(dut))
		// }
		gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(vrf).Config(), ni)
	}
	// configura default network instance
	ni := c.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	ni.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE
	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Config(), ni)
}

// cidr takes as input the IPv4 address and the Mask and returns the IP string in
// CIDR notation.
func cidr(ipv4 string, ones int) string {
	return ipv4 + "/" + strconv.Itoa(ones)
}

// getPbrPolicy creates PBR rules for cluster
func getPbrPolicy(dut *ondatra.DUTDevice, clusterFacing bool) *oc.NetworkInstance_PolicyForwarding {
	d := &oc.Root{}
	ni := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	ni.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE
	pf := ni.GetOrCreatePolicyForwarding()
	var name string
	if clusterFacing {
		name = clusterPolicy
	} else {
		name = wanPolicy
	}
	p := pf.GetOrCreatePolicy(name)
	p.SetType(oc.Policy_Type_VRF_SELECTION_POLICY)

	for _, pRule := range getPbrRules(dut, clusterFacing) {
		//offset sequnce by 10
		sequence := pRule.sequence + 10
		r := p.GetOrCreateRule(sequence)
		l2 := r.GetOrCreateL2()
		r4 := r.GetOrCreateIpv4()

		if pRule.dscpSet != nil {
			r4.DscpSet = pRule.dscpSet
		} else if pRule.dscpSetV6 != nil {
			r6 := r.GetOrCreateIpv6()
			r6.DscpSet = pRule.dscpSetV6
		}

		if pRule.protocol != 0 {
			r4.Protocol = oc.UnionUint8(pRule.protocol)
		}

		if pRule.srcAddr != "" {
			r4.SourceAddress = ygot.String(cidr(pRule.srcAddr, 32))
		}

		if len(pRule.decapVrfSet) == 3 {
			ra := r.GetOrCreateAction()
			ra.DecapNetworkInstance = ygot.String(pRule.decapVrfSet[0])
			ra.PostDecapNetworkInstance = ygot.String(pRule.decapVrfSet[1])
			ra.DecapFallbackNetworkInstance = ygot.String(pRule.decapVrfSet[2])
		}

		if pRule.etherType != nil {
			l2.SetEthertype(pRule.etherType)
		}
		if pRule.encapVrf != "" {
			r.GetOrCreateAction().SetNetworkInstance(pRule.encapVrf)

		}
	}
	return pf
}

// configureBaseconfig configures network instances and forwarding policy on the DUT
func configureBaseconfig(t *testing.T, dut *ondatra.DUTDevice, clusterFacing bool) {
	t.Log("Configure VRFs")
	configureNetworkInstance(t, dut)
	t.Log("Configure Cluster facing VRF selection Policy")
	pf := getPbrPolicy(dut, clusterFacing)
	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding().Config(), pf)
}

// programEntries pushes RIB entries on the DUT required for Encap functionality
func programEntries(t *testing.T, dut *ondatra.DUTDevice, c *gribi.Client) {
	// push RIB entries
	if deviations.GRIBIMACOverrideWithStaticARP(dut) {
		c.AddNH(t, nh10ID, "MACwithIp", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHOptions{Dest: otgPort2DummyIP.IPv4, Mac: magicMac})
		c.AddNH(t, nh11ID, "MACwithIp", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHOptions{Dest: otgPort3DummyIP.IPv4, Mac: magicMac})
		c.AddNH(t, nh100ID, "MACwithIp", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHOptions{Dest: otgPort4DummyIP.IPv4, Mac: magicMac})
		c.AddNH(t, nh101ID, "MACwithIp", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHOptions{Dest: otgPort5DummyIP.IPv4, Mac: magicMac})

	} else {
		c.AddNH(t, nh10ID, "MACwithInterface", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHOptions{Interface: dut.Port(t, "port2").Name(), Mac: magicMac})
		c.AddNH(t, nh11ID, "MACwithInterface", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHOptions{Interface: dut.Port(t, "port3").Name(), Mac: magicMac})
		c.AddNH(t, nh100ID, "MACwithInterface", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHOptions{Interface: dut.Port(t, "port4").Name(), Mac: magicMac})
		c.AddNH(t, nh101ID, "MACwithInterface", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHOptions{Interface: dut.Port(t, "port5").Name(), Mac: magicMac})
	}
	c.AddNHG(t, nhg2ID, map[uint64]uint64{nh10ID: 1, nh11ID: 3}, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	c.AddIPv4(t, cidr(vipIP1, 32), nhg2ID, deviations.DefaultNetworkInstance(dut), deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)

	c.AddNHG(t, nhg3ID, map[uint64]uint64{nh100ID: 2, nh101ID: 3}, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	c.AddIPv4(t, cidr(vipIP2, 32), nhg3ID, deviations.DefaultNetworkInstance(dut), deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)

	c.AddNH(t, nh1ID, vipIP1, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	c.AddNH(t, nh2ID, vipIP2, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	c.AddNHG(t, nhg1ID, map[uint64]uint64{nh1ID: 1, nh2ID: 3}, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	c.AddIPv4(t, cidr(tunnelDstIP1, 32), nhg1ID, vrfTransit, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	c.AddIPv4(t, cidr(tunnelDstIP2, 32), nhg1ID, vrfTransit, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)

	c.AddNH(t, nh201ID, "Encap", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHOptions{Src: ipv4OuterSrc111, Dest: tunnelDstIP1, VrfName: vrfTransit})
	c.AddNH(t, nh202ID, "Encap", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHOptions{Src: ipv4OuterSrc111, Dest: tunnelDstIP2, VrfName: vrfTransit})
	c.AddNHG(t, nhg10ID, map[uint64]uint64{nh201ID: 1, nh202ID: 3}, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	c.AddIPv4(t, cidr(ipv4EntryPrefix, ipv4EntryPrefixLen), nhg10ID, vrfEncapA, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	c.AddIPv4(t, cidr(ipv4EntryPrefix, ipv4EntryPrefixLen), nhg10ID, vrfEncapB, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	c.AddIPv6(t, cidr(ipv6EntryPrefix, ipv6EntryPrefixLen), nhg10ID, vrfEncapA, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	c.AddIPv6(t, cidr(ipv6EntryPrefix, ipv6EntryPrefixLen), nhg10ID, vrfEncapB, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
}

func configureDUT(t *testing.T, dut *ondatra.DUTDevice, clusterFacing bool) {
	d := gnmi.OC()
	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")
	p3 := dut.Port(t, "port3")
	p4 := dut.Port(t, "port4")
	p5 := dut.Port(t, "port5")

	// configure interfaces
	gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), dutPort1.NewOCInterface(p1.Name(), dut))
	gnmi.Replace(t, dut, d.Interface(p2.Name()).Config(), dutPort2.NewOCInterface(p2.Name(), dut))
	gnmi.Replace(t, dut, d.Interface(p3.Name()).Config(), dutPort3.NewOCInterface(p3.Name(), dut))
	gnmi.Replace(t, dut, d.Interface(p4.Name()).Config(), dutPort4.NewOCInterface(p4.Name(), dut))
	gnmi.Replace(t, dut, d.Interface(p5.Name()).Config(), dutPort5.NewOCInterface(p5.Name(), dut))

	// configure base PBF policies and network-instances
	configureBaseconfig(t, dut, clusterFacing)

	// apply PBF to src interface.
	applyForwardingPolicy(t, dut, p1.Name(), clusterFacing)
	if deviations.GRIBIMACOverrideWithStaticARP(dut) {
		staticARPWithSecondaryIP(t, dut)
	}
}

// applyForwardingPolicy applies the forwarding policy on the interface.
func applyForwardingPolicy(t *testing.T, dut *ondatra.DUTDevice, ingressPort string, clusterFacing bool) {
	policyName := wanPolicy
	if clusterFacing {
		policyName = clusterPolicy
	}
	t.Logf("Applying forwarding policy on interface %v ... ", ingressPort)
	d := &oc.Root{}
	// interfaceID := ingressPort
	// if deviations.InterfaceRefInterfaceIDFormat(dut) {
	interfaceID := ingressPort + ".0"
	// }
	pfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding().Interface(interfaceID)
	pfCfg := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut)).GetOrCreatePolicyForwarding().GetOrCreateInterface(interfaceID)
	pfCfg.ApplyVrfSelectionPolicy = ygot.String(policyName)
	pfCfg.GetOrCreateInterfaceRef().Interface = ygot.String(ingressPort)
	pfCfg.GetOrCreateInterfaceRef().Subinterface = ygot.Uint32(0)
	gnmi.Replace(t, dut, pfPath.Config(), pfCfg)
}

// configreOTG configures port1-5 on the OTG.
func configureOTG(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	otg := ate.OTG()
	topo := gosnappi.NewConfig()
	t.Logf("Configuring OTG port1")
	p1 := ate.Port(t, "port1")
	p2 := ate.Port(t, "port2")
	p3 := ate.Port(t, "port3")
	p4 := ate.Port(t, "port4")
	p5 := ate.Port(t, "port5")

	otgPort1.AddToOTG(topo, p1, &dutPort1)
	otgPort2.AddToOTG(topo, p2, &dutPort2)
	otgPort3.AddToOTG(topo, p3, &dutPort3)
	otgPort4.AddToOTG(topo, p4, &dutPort4)
	otgPort5.AddToOTG(topo, p5, &dutPort5)

	t.Logf("Pushing config to ATE and starting protocols...")
	otg.PushConfig(t, topo)
	t.Logf("starting protocols...")
	otg.StartProtocols(t)
	time.Sleep(50 * time.Second)
	otgutils.WaitForARP(t, ate.OTG(), topo, "IPv4")
	otgutils.WaitForARP(t, ate.OTG(), topo, "IPv6")
	return topo
}

// enableCapture enables packet capture on specified list of ports on OTG
func enableCapture(t *testing.T, otg *otg.OTG, topo gosnappi.Config, otgPortNames []string) {
	for _, port := range otgPortNames {
		t.Log("Enabling capture on ", port)
		topo.Captures().Add().SetName(port).SetPortNames([]string{port}).SetFormat(gosnappi.CaptureFormat.PCAP)
	}
	//t.Log(topo.Msg().GetCaptures())
	otg.PushConfig(t, topo)
}

// clearCapture clears capture from all ports on the OTG
func clearCapture(t *testing.T, otg *otg.OTG, topo gosnappi.Config) {
	t.Log("Clearing capture")
	topo.Captures().Clear()
	otg.PushConfig(t, topo)
}

// getFlow returns a flow of type ipv4, ipv4in4, ipv6in4 or ipv6 with dscp value passed in args.
func (fa *flowAttr) getFlow(flowType string, name string, dscp uint32) gosnappi.Flow {
	flow := fa.topo.Flows().Add().SetName(name)
	flow.Metrics().SetEnable(true)

	flow.TxRx().Port().SetTxName(fa.srcPort).SetRxNames(fa.dstPorts)
	e1 := flow.Packet().Add().Ethernet()
	e1.Src().SetValue(fa.srcMac)
	e1.Dst().SetValue(fa.dstMac)
	if flowType == "ipv4" || flowType == "ipv4in4" || flowType == "ipv6in4" {
		v4 := flow.Packet().Add().Ipv4()
		v4.Src().SetValue(fa.src)
		v4.Dst().SetValue(fa.dst)
		v4.TimeToLive().SetValue(ttl)
		v4.Priority().Dscp().Phb().SetValue(dscp)

		// add inner ipv4 headers
		if flowType == "ipv4in4" {
			innerV4 := flow.Packet().Add().Ipv4()
			innerV4.Src().SetValue(innerV4SrcIP)
			innerV4.Dst().SetValue(innerV4DstIP)
		}

		// add inner ipv6 headers
		if flowType == "ipv6in4" {
			innerV6 := flow.Packet().Add().Ipv6()
			innerV6.Src().SetValue(InnerV6SrcIP)
			innerV6.Dst().SetValue(InnerV6DstIP)
		}

	} else if flowType == "ipv6" {
		v6 := flow.Packet().Add().Ipv6()
		v6.Src().SetValue(fa.src)
		v6.Dst().SetValue(fa.dst)
		v6.HopLimit().SetValue(ttl)
		v6.TrafficClass().SetValue(dscp << 2)
	}

	udp := flow.Packet().Add().Udp()
	udp.SrcPort().Increment().SetStart(50001).SetCount(1000)
	udp.DstPort().Increment().SetStart(50001).SetCount(1000)

	return flow
}

// sendTraffic starts traffic flows and send traffic for a fixed duration
func sendTraffic(t *testing.T, args *testArgs, flows []gosnappi.Flow, capture bool) {
	otg := args.ate.OTG()
	args.topo.Flows().Clear().Items()
	args.topo.Flows().Append(flows...)
	otg.PushConfig(t, args.topo)
	otg.StartProtocols(t)
	if capture {
		startCapture(t, args.ate)
		defer stopCapture(t, args.ate)
	}
	t.Log("Starting traffic")
	otg.StartTraffic(t)
	time.Sleep(trafficDuration)
	otg.StopTraffic(t)
	t.Log("Traffic stopped")
}

// validateTrafficFlows verifies that the flow on ATE should pass for good flow and fail for bad flow.
func validateTrafficFlows(t *testing.T, args *testArgs, flows []gosnappi.Flow, capture bool, match bool) {

	otg := args.ate.OTG()
	sendTraffic(t, args, flows, capture)

	otgutils.LogPortMetrics(t, otg, args.topo)
	otgutils.LogFlowMetrics(t, otg, args.topo)

	for _, flow := range flows {
		outPkts := float32(gnmi.Get(t, otg, gnmi.OTG().Flow(flow.Name()).Counters().OutPkts().State()))
		inPkts := float32(gnmi.Get(t, otg, gnmi.OTG().Flow(flow.Name()).Counters().InPkts().State()))

		if outPkts == 0 {
			t.Fatalf("OutPkts for flow %s is 0, want > 0", flow)
		}
		if match {
			if got := ((outPkts - inPkts) * 100) / outPkts; got > 0 {
				t.Fatalf("LossPct for flow %s: got %v, want 0", flow.Name(), got)
			}
		} else {
			if got := ((outPkts - inPkts) * 100) / outPkts; got != 100 {
				t.Fatalf("LossPct for flow %s: got %v, want 100", flow.Name(), got)
			}
		}

	}
}

// validateTunnelEncapRatio checks whether tunnel1 and tunnel2 ecapped packets are withing specific ratio
func validateTunnelEncapRatio(t *testing.T, tunCounter map[string][]int) {
	for port, counter := range tunCounter {
		t.Logf("Validating tunnel encap ratio for %s", port)
		tunnel1Pkts := float32(counter[0])
		tunnel2Pkts := float32(counter[1])
		if tunnel1Pkts == 0 {
			t.Error("tunnel1 encapped packet count: got 0, want > 0")
		} else if tunnel2Pkts == 0 {
			t.Error("tunnel2 encapped packet count: got 0, want > 0")
		} else {
			totalPkts := tunnel1Pkts + tunnel2Pkts
			if (tunnel1Pkts/totalPkts) < (ratioTunEncap1-ratioTunEncapTol) ||
				(tunnel1Pkts/totalPkts) > (ratioTunEncap1+ratioTunEncapTol) {
				t.Errorf("tunnel1 encapsulation ratio (%f) is not within range", tunnel1Pkts/totalPkts)
			} else if (tunnel2Pkts/totalPkts) < (ratioTunEncap2-ratioTunEncapTol) ||
				(tunnel2Pkts/totalPkts) > (ratioTunEncap2+ratioTunEncapTol) {
				t.Errorf("tunnel2 encapsulation ratio (%f) is not within range", tunnel1Pkts/totalPkts)
			} else {
				t.Log("tunnel encapsulated packets are within ratio")
			}
		}
	}
}

// validatePacketCapture reads capture files and checks the encapped packet for desired protocol, dscp and ttl
func validatePacketCapture(t *testing.T, args *testArgs, otgPortNames []string, pa *packetAttr) map[string][]int {
	tunCounter := make(map[string][]int)
	for _, otgPortName := range otgPortNames {
		bytes := args.ate.OTG().GetCapture(t, gosnappi.NewCaptureRequest().SetPortName(otgPortName))
		f, err := os.CreateTemp("", ".pcap")
		if err != nil {
			t.Fatalf("ERROR: Could not create temporary pcap file: %v\n", err)
		}
		if _, err := f.Write(bytes); err != nil {
			t.Fatalf("ERROR: Could not write bytes to pcap file: %v\n", err)
		}
		f.Close()
		t.Logf("Verifying packet attributes captured on %s", otgPortName)
		handle, err := pcap.OpenOffline(f.Name())
		if err != nil {
			log.Fatal(err)
		}
		defer handle.Close()
		packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
		tunnel1Pkts := 0
		tunnel2Pkts := 0
		for packet := range packetSource.Packets() {
			ipV4Layer := packet.Layer(layers.LayerTypeIPv4)
			if ipV4Layer != nil {
				v4Packet, _ := ipV4Layer.(*layers.IPv4)
				if got := v4Packet.Protocol; got != layers.IPProtocol(pa.protocol) {
					t.Errorf("Packet protocol type mismatch, got: %d, want %d", got, pa.protocol)
					break
				}
				if got := int(v4Packet.TOS >> 2); got != pa.dscp {
					t.Errorf("Dscp value mismatch, got %d, want %d", got, pa.dscp)
					break
				}
				// if !deviations.TtlCopyToTunnelHeaderUnsupported(args.dut) {
				// 	if got := uint32(v4Packet.TTL); got != pa.ttl {
				// 		t.Errorf("TTL mismatch, got: %d, want: %d", got, pa.ttl)
				// 		break
				// 	}
				// }
				if v4Packet.DstIP.String() == tunnelDstIP1 {
					tunnel1Pkts++
				}
				if v4Packet.DstIP.String() == tunnelDstIP2 {
					tunnel2Pkts++
				}

			}
		}
		t.Logf("tunnel1, tunnel2 packet count on %s: %d , %d", otgPortName, tunnel1Pkts, tunnel2Pkts)
		tunCounter[otgPortName] = []int{tunnel1Pkts, tunnel2Pkts}
	}
	return tunCounter

}

// startCapture starts the capture on the otg ports
func startCapture(t *testing.T, ate *ondatra.ATEDevice) {
	otg := ate.OTG()
	cs := gosnappi.NewControlState()
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.START)
	otg.SetControlState(t, cs)
}

// stopCapture starts the capture on the otg ports
func stopCapture(t *testing.T, ate *ondatra.ATEDevice) {
	otg := ate.OTG()
	cs := gosnappi.NewControlState()
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.STOP)
	otg.SetControlState(t, cs)
}

// normalize normalizes the input values so that the output values sum
// to 1.0 but reflect the proportions of the input.  For example,
// input [1, 2, 3, 4] is normalized to [0.1, 0.2, 0.3, 0.4].
func normalize(xs []uint64) (ys []float64, sum uint64) {
	for _, x := range xs {
		sum += x
	}
	ys = make([]float64, len(xs))
	for i, x := range xs {
		ys[i] = float64(x) / float64(sum)
	}
	return ys, sum
}

// validateTrafficDistribution checks if the packets received on receiving ports are within specificied weight ratios
func validateTrafficDistribution(t *testing.T, ate *ondatra.ATEDevice, wantWeights []float64) {
	inFramesAllPorts := gnmi.GetAll(t, ate.OTG(), gnmi.OTG().PortAny().Counters().InFrames().State())
	// skip first entry that belongs to source port on ate
	gotWeights, _ := normalize(inFramesAllPorts[1:])

	t.Log("got ratio:", gotWeights)
	t.Log("want ratio:", wantWeights)
	if diff := cmp.Diff(wantWeights, gotWeights, cmpopts.EquateApprox(0, trfDistTolerance)); diff != "" {
		t.Errorf("Packet distribution ratios -want,+got:\n%s", diff)
	}
}

// configStaticArp configures static arp entries
func configStaticArp(p string, ipv4addr string, macAddr string) *oc.Interface {
	i := &oc.Interface{Name: ygot.String(p)}
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	s := i.GetOrCreateSubinterface(0)
	s4 := s.GetOrCreateIpv4()
	n4 := s4.GetOrCreateNeighbor(ipv4addr)
	n4.LinkLayerAddress = ygot.String(macAddr)
	return i
}

// staticARPWithSecondaryIP configures secondary IPs and static ARP.
func staticARPWithSecondaryIP(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	p2 := dut.Port(t, "port2")
	p3 := dut.Port(t, "port3")
	p4 := dut.Port(t, "port4")
	p5 := dut.Port(t, "port5")
	gnmi.Update(t, dut, gnmi.OC().Interface(p2.Name()).Config(), dutPort2DummyIP.NewOCInterface(p2.Name(), dut))
	gnmi.Update(t, dut, gnmi.OC().Interface(p3.Name()).Config(), dutPort3DummyIP.NewOCInterface(p3.Name(), dut))
	gnmi.Update(t, dut, gnmi.OC().Interface(p4.Name()).Config(), dutPort4DummyIP.NewOCInterface(p4.Name(), dut))
	gnmi.Update(t, dut, gnmi.OC().Interface(p5.Name()).Config(), dutPort5DummyIP.NewOCInterface(p5.Name(), dut))
	gnmi.Update(t, dut, gnmi.OC().Interface(p2.Name()).Config(), configStaticArp(p2.Name(), otgPort2DummyIP.IPv4, magicMac))
	gnmi.Update(t, dut, gnmi.OC().Interface(p3.Name()).Config(), configStaticArp(p3.Name(), otgPort3DummyIP.IPv4, magicMac))
	gnmi.Update(t, dut, gnmi.OC().Interface(p4.Name()).Config(), configStaticArp(p4.Name(), otgPort4DummyIP.IPv4, magicMac))
	gnmi.Update(t, dut, gnmi.OC().Interface(p5.Name()).Config(), configStaticArp(p5.Name(), otgPort5DummyIP.IPv4, magicMac))
}

func testBackupNextHopGroup(t *testing.T, args *testArgs) {
	if deviations.GRIBIMACOverrideWithStaticARP(args.dut) {
		args.client.AddNH(t, 2, "MACwithIp", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Dest: otgPort2DummyIP.IPv4, Mac: magicMac})
		args.client.AddNH(t, 3, "MACwithIp", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Dest: otgPort3DummyIP.IPv4, Mac: magicMac})

	} else {
		args.client.AddNH(t, 2, "MACwithInterface", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Interface: args.dut.Port(t, "port2").Name(), Mac: magicMac})
		args.client.AddNH(t, 3, "MACwithInterface", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Interface: args.dut.Port(t, "port3").Name(), Mac: magicMac})
	}
	args.client.AddNHG(t, 100, map[uint64]uint64{3: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNHG(t, 101, map[uint64]uint64{2: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: 100})
	args.client.AddIPv4(t, cidr(vipIP1, 32), 101, deviations.DefaultNetworkInstance(args.dut), deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	args.client.AddNH(t, nh1ID, vipIP1, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNHG(t, nhg1ID, map[uint64]uint64{nh1ID: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(tunnelDstIP1, 32), nhg1ID, vrfTransit, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(tunnelDstIP2, 32), nhg1ID, vrfTransit, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	args.client.AddNH(t, nh201ID, "Encap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Src: ipv4OuterSrc111, Dest: tunnelDstIP1, VrfName: vrfTransit})
	args.client.AddNH(t, nh202ID, "Encap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Src: ipv4OuterSrc111, Dest: tunnelDstIP2, VrfName: vrfTransit})
	args.client.AddNHG(t, nhg10ID, map[uint64]uint64{nh201ID: 1, nh202ID: 3}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(ipv4EntryPrefix, ipv4EntryPrefixLen), nhg10ID, vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(ipv4EntryPrefix, ipv4EntryPrefixLen), nhg10ID, vrfEncapB, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv6(t, cidr(ipv6EntryPrefix, ipv6EntryPrefixLen), nhg10ID, vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv6(t, cidr(ipv6EntryPrefix, ipv6EntryPrefixLen), nhg10ID, vrfEncapB, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	weights := []float64{1, 0, 0, 0}
	testTraffic(t, args, weights, true)

	t.Log("Shutdown link carrying primary traffic")
	gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port2").Name()).Subinterface(0).Enabled().Config(), false)
	defer gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port2").Name()).Subinterface(0).Enabled().Config(), true)
	weights = []float64{0, 1, 0, 0}
	testTraffic(t, args, weights, true)
}

func testUnviableTunnelBackupNextHopGroup(t *testing.T, args *testArgs) {

	args.client.AddNH(t, 2, "MACwithIp", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Dest: otgPort2DummyIP.IPv4, Mac: magicMac})
	args.client.AddNHG(t, 102, map[uint64]uint64{2: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(vipIP1, 32), 102, deviations.DefaultNetworkInstance(args.dut), deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	args.client.AddNH(t, 3, "MACwithIp", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Dest: otgPort3DummyIP.IPv4, Mac: magicMac})
	args.client.AddNHG(t, 103, map[uint64]uint64{3: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(vipIP2, 32), 103, deviations.DefaultNetworkInstance(args.dut), deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	// backup
	args.client.AddNH(t, 4, "MACwithIp", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Dest: otgPort4DummyIP.IPv4, Mac: magicMac})
	args.client.AddNHG(t, 104, map[uint64]uint64{4: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(vipIP3, 32), 104, deviations.DefaultNetworkInstance(args.dut), deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNH(t, 203, vipIP3, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNHG(t, 303, map[uint64]uint64{203: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(tunnelDstIP3, 32), 303, vrfRepaired, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNH(t, 403, "DecapEncap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Src: ipv4OuterSrc222, Dest: tunnelDstIP3, VrfName: vrfRepaired})
	args.client.AddNHG(t, 503, map[uint64]uint64{403: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	// backup end

	args.client.AddNH(t, 201, vipIP1, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNH(t, 202, vipIP2, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNHG(t, 301, map[uint64]uint64{201: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: 503})
	args.client.AddNHG(t, 302, map[uint64]uint64{202: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(tunnelDstIP1, 32), 301, vrfTransit, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(tunnelDstIP2, 32), 302, vrfTransit, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	args.client.AddNH(t, 401, "Encap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Src: ipv4OuterSrc111, Dest: tunnelDstIP1, VrfName: vrfTransit})
	args.client.AddNH(t, 402, "Encap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Src: ipv4OuterSrc111, Dest: tunnelDstIP2, VrfName: vrfTransit})
	args.client.AddNHG(t, 501, map[uint64]uint64{401: 1, 402: 3}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	// prefixes
	args.client.AddIPv4(t, cidr(ipv4EntryPrefix, ipv4EntryPrefixLen), 501, vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(ipv4EntryPrefix, ipv4EntryPrefixLen), 501, vrfEncapB, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv6(t, cidr(ipv6EntryPrefix, ipv6EntryPrefixLen), 501, vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv6(t, cidr(ipv6EntryPrefix, ipv6EntryPrefixLen), 501, vrfEncapB, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	weights := []float64{0.25, 0.75, 0, 0}
	testTraffic(t, args, weights, true)

	t.Log("Shutdown link carrying primary traffic")
	gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port2").Name()).Subinterface(0).Enabled().Config(), false)
	defer gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port2").Name()).Subinterface(0).Enabled().Config(), true)
	weights = []float64{0, 0.75, 0.25, 0}
	testTraffic(t, args, weights, true)
}

func testUnviableTunnelBothPrimaryBackupDown(t *testing.T, args *testArgs) {
	args.client.AddNH(t, 2, "MACwithIp", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Dest: otgPort2DummyIP.IPv4, Mac: magicMac})
	args.client.AddNHG(t, 102, map[uint64]uint64{2: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(vipIP1, 32), 102, deviations.DefaultNetworkInstance(args.dut), deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	args.client.AddNH(t, 3, "MACwithIp", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Dest: otgPort3DummyIP.IPv4, Mac: magicMac})
	args.client.AddNHG(t, 103, map[uint64]uint64{3: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(vipIP2, 32), 103, deviations.DefaultNetworkInstance(args.dut), deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	// backup
	args.client.AddNH(t, 4, "MACwithIp", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Dest: otgPort4DummyIP.IPv4, Mac: magicMac})
	args.client.AddNHG(t, 104, map[uint64]uint64{4: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(vipIP3, 32), 104, deviations.DefaultNetworkInstance(args.dut), deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNH(t, 203, vipIP3, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNHG(t, 303, map[uint64]uint64{203: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(tunnelDstIP3, 32), 303, vrfRepaired, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNH(t, 403, "DecapEncap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Src: ipv4OuterSrc222, Dest: tunnelDstIP3, VrfName: vrfRepaired})
	args.client.AddNHG(t, 503, map[uint64]uint64{403: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	// backup end

	args.client.AddNH(t, 201, vipIP1, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNH(t, 202, vipIP2, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNHG(t, 301, map[uint64]uint64{201: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: 503})
	args.client.AddNHG(t, 302, map[uint64]uint64{202: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(tunnelDstIP1, 32), 301, vrfTransit, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(tunnelDstIP2, 32), 302, vrfTransit, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	args.client.AddNH(t, 401, "Encap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Src: ipv4OuterSrc111, Dest: tunnelDstIP1, VrfName: vrfTransit})
	args.client.AddNH(t, 402, "Encap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Src: ipv4OuterSrc111, Dest: tunnelDstIP2, VrfName: vrfTransit})
	args.client.AddNHG(t, 501, map[uint64]uint64{401: 1, 402: 3}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	// prefixes
	args.client.AddIPv4(t, cidr(ipv4EntryPrefix, ipv4EntryPrefixLen), 501, vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(ipv4EntryPrefix, ipv4EntryPrefixLen), 501, vrfEncapB, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv6(t, cidr(ipv6EntryPrefix, ipv6EntryPrefixLen), 501, vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv6(t, cidr(ipv6EntryPrefix, ipv6EntryPrefixLen), 501, vrfEncapB, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	weights := []float64{0.25, 0.75, 0, 0}
	testTraffic(t, args, weights, true)

	t.Log("Shutdown link carrying primary traffic")
	gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port2").Name()).Subinterface(0).Enabled().Config(), false)
	defer gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port2").Name()).Subinterface(0).Enabled().Config(), true)
	gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port4").Name()).Subinterface(0).Enabled().Config(), false)
	defer gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port4").Name()).Subinterface(0).Enabled().Config(), true)

	weights = []float64{0, 1, 0, 0}
	testTraffic(t, args, weights, true)
}

func testBackupNHGTunnelToUnviableTunnel(t *testing.T, args *testArgs) {
	args.client.AddNH(t, 2, "MACwithIp", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Dest: otgPort2DummyIP.IPv4, Mac: magicMac})
	args.client.AddNHG(t, 102, map[uint64]uint64{2: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(vipIP1, 32), 102, deviations.DefaultNetworkInstance(args.dut), deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	args.client.AddNH(t, 3, "MACwithIp", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Dest: otgPort3DummyIP.IPv4, Mac: magicMac})
	args.client.AddNHG(t, 103, map[uint64]uint64{3: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(vipIP2, 32), 103, deviations.DefaultNetworkInstance(args.dut), deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	// backup
	args.client.AddNH(t, 4, "MACwithIp", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Dest: otgPort4DummyIP.IPv4, Mac: magicMac})
	args.client.AddNHG(t, 104, map[uint64]uint64{4: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(vipIP3, 32), 104, deviations.DefaultNetworkInstance(args.dut), deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNH(t, 203, vipIP3, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNHG(t, 303, map[uint64]uint64{203: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(tunnelDstIP3, 32), 303, vrfRepaired, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNH(t, 403, "DecapEncap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Src: ipv4OuterSrc222, Dest: tunnelDstIP3, VrfName: vrfRepaired})
	args.client.AddNHG(t, 503, map[uint64]uint64{403: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	// backup end

	args.client.AddNH(t, 201, vipIP1, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNH(t, 202, vipIP2, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNHG(t, 301, map[uint64]uint64{201: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: 503})
	args.client.AddNHG(t, 302, map[uint64]uint64{202: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(tunnelDstIP1, 32), 301, vrfTransit, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(tunnelDstIP2, 32), 302, vrfTransit, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	args.client.AddNH(t, 401, "Encap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Src: ipv4OuterSrc111, Dest: tunnelDstIP1, VrfName: vrfTransit})
	args.client.AddNH(t, 402, "Encap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Src: ipv4OuterSrc111, Dest: tunnelDstIP2, VrfName: vrfTransit})
	args.client.AddNHG(t, 502, map[uint64]uint64{402: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNHG(t, 501, map[uint64]uint64{401: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: 502})

	// prefixes
	args.client.AddIPv4(t, cidr(ipv4EntryPrefix, ipv4EntryPrefixLen), 501, vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(ipv4EntryPrefix, ipv4EntryPrefixLen), 501, vrfEncapB, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv6(t, cidr(ipv6EntryPrefix, ipv6EntryPrefixLen), 501, vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv6(t, cidr(ipv6EntryPrefix, ipv6EntryPrefixLen), 501, vrfEncapB, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	weights := []float64{1, 0, 0, 0}
	testTraffic(t, args, weights, true)

	t.Log("Shutdown link carrying primary and backup for tunnel1 traffic")
	gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port2").Name()).Subinterface(0).Enabled().Config(), false)
	defer gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port2").Name()).Subinterface(0).Enabled().Config(), true)
	gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port4").Name()).Subinterface(0).Enabled().Config(), false)
	defer gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port4").Name()).Subinterface(0).Enabled().Config(), true)

	weights = []float64{0, 1, 0, 0}
	testTraffic(t, args, weights, true)
}

// SSFRR_07
func testPrimaryAndBackupNHGUnviableForTunnel(t *testing.T, args *testArgs) {
	configFallBackVrf(t, args.dut, []string{vrfEncapA})
	configDefaultRoute(t, args.dut, cidr(ipv4FlowIP, 32), otgPort5.IPv4, cidr(ipv6FlowIP, 128), otgPort5.IPv6)
	defer gnmi.Delete(t, args.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(args.dut)).Static(cidr(ipv4EntryPrefix, ipv4EntryPrefixLen)).Config())
	defer gnmi.Delete(t, args.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(args.dut)).Static(cidr(ipv6EntryPrefix, ipv6EntryPrefixLen)).Config())

	args.client.AddNH(t, 2, "MACwithIp", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Dest: otgPort2DummyIP.IPv4, Mac: magicMac})
	args.client.AddNHG(t, 102, map[uint64]uint64{2: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(vipIP1, 32), 102, deviations.DefaultNetworkInstance(args.dut), deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	args.client.AddNH(t, 3, "MACwithIp", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Dest: otgPort3DummyIP.IPv4, Mac: magicMac})
	args.client.AddNHG(t, 103, map[uint64]uint64{3: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(vipIP2, 32), 103, deviations.DefaultNetworkInstance(args.dut), deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	// backup
	args.client.AddNH(t, 4, "MACwithIp", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Dest: otgPort4DummyIP.IPv4, Mac: magicMac})
	args.client.AddNHG(t, 104, map[uint64]uint64{4: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(vipIP3, 32), 104, deviations.DefaultNetworkInstance(args.dut), deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNH(t, 203, vipIP3, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNHG(t, 303, map[uint64]uint64{203: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(tunnelDstIP3, 32), 303, vrfRepaired, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNH(t, 403, "DecapEncap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Src: ipv4OuterSrc222, Dest: tunnelDstIP3, VrfName: vrfRepaired})
	args.client.AddNHG(t, 503, map[uint64]uint64{403: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	// backup end

	args.client.AddNH(t, 201, vipIP1, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNH(t, 202, vipIP2, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNHG(t, 301, map[uint64]uint64{201: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: 503})
	args.client.AddNHG(t, 302, map[uint64]uint64{202: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(tunnelDstIP1, 32), 301, vrfTransit, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(tunnelDstIP2, 32), 302, vrfTransit, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	args.client.AddNH(t, 401, "Encap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Src: ipv4OuterSrc111, Dest: tunnelDstIP1, VrfName: vrfTransit})
	args.client.AddNH(t, 402, "Encap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Src: ipv4OuterSrc111, Dest: tunnelDstIP2, VrfName: vrfTransit})
	args.client.AddNHG(t, 502, map[uint64]uint64{402: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNHG(t, 501, map[uint64]uint64{401: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: 502})

	// prefixes
	args.client.AddIPv4(t, cidr(ipv4EntryPrefix, ipv4EntryPrefixLen), 501, vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(ipv4EntryPrefix, ipv4EntryPrefixLen), 501, vrfEncapB, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv6(t, cidr(ipv6EntryPrefix, ipv6EntryPrefixLen), 501, vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv6(t, cidr(ipv6EntryPrefix, ipv6EntryPrefixLen), 501, vrfEncapB, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	weights := []float64{1, 0, 0, 0}
	testTraffic(t, args, weights, true)

	t.Log("Shutdown link carrying primary and backup for tunnel1 traffic")
	gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port2").Name()).Subinterface(0).Enabled().Config(), false)
	defer gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port2").Name()).Subinterface(0).Enabled().Config(), true)
	gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port4").Name()).Subinterface(0).Enabled().Config(), false)
	defer gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port4").Name()).Subinterface(0).Enabled().Config(), true)
	gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port3").Name()).Subinterface(0).Enabled().Config(), false)
	defer gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port3").Name()).Subinterface(0).Enabled().Config(), true)

	weights = []float64{0, 0, 0, 1}
	testTraffic(t, args, weights, true)
}

// SSFRR_08
func testPrimaryPathUnviableWihoutBackupNHG(t *testing.T, args *testArgs) {
	configFallBackVrf(t, args.dut, []string{vrfEncapA})
	configDefaultRoute(t, args.dut, cidr(ipv4FlowIP, 32), otgPort5.IPv4, cidr(ipv6FlowIP, 128), otgPort5.IPv6)
	defer gnmi.Delete(t, args.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(args.dut)).Static(cidr(ipv4EntryPrefix, ipv4EntryPrefixLen)).Config())
	defer gnmi.Delete(t, args.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(args.dut)).Static(cidr(ipv6EntryPrefix, ipv6EntryPrefixLen)).Config())

	configureVIP1(t, args)

	args.client.AddNH(t, vipNH(1), vipIP1, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNHG(t, vipNHG(1), map[uint64]uint64{vipNH(1): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(tunnelDstIP1, 32), vipNHG(1), vrfTransit, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(tunnelDstIP2, 32), vipNHG(1), vrfTransit, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	args.client.AddNH(t, tunNH(1), "Encap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Src: ipv4OuterSrc111, Dest: tunnelDstIP1, VrfName: vrfTransit})
	args.client.AddNH(t, tunNH(2), "Encap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Src: ipv4OuterSrc111, Dest: tunnelDstIP2, VrfName: vrfTransit})
	args.client.AddNHG(t, tunNHG(1), map[uint64]uint64{tunNH(1): 1, tunNH(2): 3}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	args.client.AddIPv4(t, cidr(ipv4EntryPrefix, ipv4EntryPrefixLen), tunNHG(1), vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(ipv4EntryPrefix, ipv4EntryPrefixLen), tunNHG(1), vrfEncapB, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv6(t, cidr(ipv6EntryPrefix, ipv6EntryPrefixLen), tunNHG(1), vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv6(t, cidr(ipv6EntryPrefix, ipv6EntryPrefixLen), tunNHG(1), vrfEncapB, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	weights := []float64{1, 0, 0, 0}
	testTraffic(t, args, weights, true)

	t.Log("Shutdown link carrying primary traffic")
	gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port2").Name()).Subinterface(0).Enabled().Config(), false)
	defer gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port2").Name()).Subinterface(0).Enabled().Config(), true)
	gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port3").Name()).Subinterface(0).Enabled().Config(), false)
	defer gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port3").Name()).Subinterface(0).Enabled().Config(), true)
	weights = []float64{0, 0, 0, 1}
	testTraffic(t, args, weights, false)
}

func testAllTunnelUnviable(t *testing.T, args *testArgs) {
	configFallBackVrf(t, args.dut, []string{vrfEncapA})
	configDefaultRoute(t, args.dut, cidr(ipv4FlowIP, 32), otgPort5.IPv4, cidr(ipv6FlowIP, 128), otgPort5.IPv6)
	defer gnmi.Delete(t, args.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(args.dut)).Static(cidr(ipv4EntryPrefix, ipv4EntryPrefixLen)).Config())
	defer gnmi.Delete(t, args.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(args.dut)).Static(cidr(ipv6EntryPrefix, ipv6EntryPrefixLen)).Config())

	args.client.AddNH(t, 2, "MACwithIp", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Dest: otgPort2DummyIP.IPv4, Mac: magicMac})
	args.client.AddNHG(t, 102, map[uint64]uint64{2: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(vipIP1, 32), 102, deviations.DefaultNetworkInstance(args.dut), deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	args.client.AddNH(t, 3, "MACwithIp", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Dest: otgPort3DummyIP.IPv4, Mac: magicMac})
	args.client.AddNHG(t, 103, map[uint64]uint64{3: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(vipIP2, 32), 103, deviations.DefaultNetworkInstance(args.dut), deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	// backup
	args.client.AddNH(t, 4, "MACwithIp", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Dest: otgPort4DummyIP.IPv4, Mac: magicMac})
	args.client.AddNHG(t, 104, map[uint64]uint64{4: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(vipIP3, 32), 104, deviations.DefaultNetworkInstance(args.dut), deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNH(t, 203, vipIP3, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNHG(t, 303, map[uint64]uint64{203: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(tunnelDstIP3, 32), 303, vrfRepaired, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNH(t, 403, "DecapEncap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Src: ipv4OuterSrc222, Dest: tunnelDstIP3, VrfName: vrfRepaired})
	args.client.AddNHG(t, 503, map[uint64]uint64{403: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	// backup end

	args.client.AddNH(t, 201, vipIP1, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNH(t, 202, vipIP2, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNHG(t, 301, map[uint64]uint64{201: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: 503})
	args.client.AddNHG(t, 302, map[uint64]uint64{202: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(tunnelDstIP1, 32), 301, vrfTransit, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(tunnelDstIP2, 32), 302, vrfTransit, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	args.client.AddNH(t, 401, "Encap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Src: ipv4OuterSrc111, Dest: tunnelDstIP1, VrfName: vrfTransit})
	args.client.AddNH(t, 402, "Encap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Src: ipv4OuterSrc111, Dest: tunnelDstIP2, VrfName: vrfTransit})
	args.client.AddNHG(t, 501, map[uint64]uint64{401: 1, 402: 3}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	// prefixes
	args.client.AddIPv4(t, cidr(ipv4EntryPrefix, ipv4EntryPrefixLen), 501, vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(ipv4EntryPrefix, ipv4EntryPrefixLen), 501, vrfEncapB, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv6(t, cidr(ipv6EntryPrefix, ipv6EntryPrefixLen), 501, vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv6(t, cidr(ipv6EntryPrefix, ipv6EntryPrefixLen), 501, vrfEncapB, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	weights := []float64{0.25, 0.75, 0, 0}
	testTraffic(t, args, weights, true)

	t.Log("Shutdown all primary and backup paths")
	gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port2").Name()).Subinterface(0).Enabled().Config(), false)
	defer gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port2").Name()).Subinterface(0).Enabled().Config(), true)
	gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port4").Name()).Subinterface(0).Enabled().Config(), false)
	defer gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port4").Name()).Subinterface(0).Enabled().Config(), true)
	gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port3").Name()).Subinterface(0).Enabled().Config(), false)
	defer gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port3").Name()).Subinterface(0).Enabled().Config(), true)

	weights = []float64{0, 0, 0, 1}
	testTraffic(t, args, weights, true)
}

// CLI to configure Default static route with NH default VRF. NH VRF option is not available with OC Local routing
func configDefaultIPStaticCli(t *testing.T, dut *ondatra.DUTDevice, vrf []string) {
	ctx := context.Background()
	for _, v := range vrf {
		v4Conf := fmt.Sprintf("router static vrf %v address-family ipv4 unicast 0.0.0.0/0 vrf default\n router static vrf %v address-family ipv6 unicast ::/0 vrf default", v, v)
		config.TextWithGNMI(ctx, t, dut, v4Conf)
		time.Sleep(5 * time.Second)
	}
}

// CLI to configure Default static route with NH default VRF. NH VRF option is not available with OC Local routing
func unConfigDefaultIPStaticCli(t *testing.T, dut *ondatra.DUTDevice, vrf []string) {
	ctx := context.Background()
	for _, v := range vrf {
		v4Conf := fmt.Sprintf("no router static vrf %v address-family ipv4 unicast 0.0.0.0/0 vrf default\n no router static vrf %v address-family ipv6 unicast ::/0 vrf default", v, v)
		config.TextWithGNMI(ctx, t, dut, v4Conf)
		time.Sleep(5 * time.Second)
	}
}

// CLI to configure falback vrf
func configFallBackVrf(t *testing.T, dut *ondatra.DUTDevice, vrf []string) {
	ctx := context.Background()
	for _, v := range vrf {
		fConf := fmt.Sprintf("vrf %v fallback-vrf default\n", v)
		config.TextWithGNMI(ctx, t, dut, fConf)
	}
}

// CLI to configure falback vrf
func unConfigFallBackVrf(t *testing.T, dut *ondatra.DUTDevice, vrf []string) {
	ctx := context.Background()
	for _, v := range vrf {
		fConf := fmt.Sprintf("no vrf %v fallback-vrf default\n", v)
		config.TextWithGNMI(ctx, t, dut, fConf)
	}
}
func TestEncapFrr(t *testing.T) {
	// Configure DUT
	dut := ondatra.DUT(t, "dut")
	configureDUT(t, dut, true)

	// Configure ATE
	otg := ondatra.ATE(t, "ate")
	topo := configureOTG(t, otg)

	test := []struct {
		name string
		desc string
		fn   func(t *testing.T, args *testArgs)
	}{
		{
			name: "EncapWithVIPHavingBackupNHG",
			desc: "Encap_12: Test Encap with backup NHG for a VIP",
			fn:   testBackupNextHopGroup,
		},
		{
			name: "SSFRRTunnelPrimaryPathUnviable",
			desc: "SFRR_01: Test self site FRR with primary path of a tunnel being unviable and backup being present",
			fn:   testUnviableTunnelBackupNextHopGroup,
		},
		{
			name: "SSFRRTunnelPrimaryAndBackupPathUnviable",
			desc: "SFRR_03: Test self site FRR with primary and backup path of a tunnel being unviable then traffic is shared via other tunnel.",
			fn:   testUnviableTunnelBothPrimaryBackupDown,
		},
		{
			name: "SSFRRTunnelPrimaryAndBackupPathUnviableForAllTunnel",
			desc: "SFRR_02: Test self site FRR with primary and backup path of all tunnels being unviable then traffic is routed via default vrf.",
			fn:   testAllTunnelUnviable,
		},
		{
			name: "SFRRBackupNHGTunneltoPrimaryTunnelWhenPrimaryTunnelUnviable",
			desc: "SFRR_06: Test self site FRR with primary and backup path of a tunnel being unviable then traffic is shared via other tunnel.",
			fn:   testBackupNHGTunnelToUnviableTunnel,
		},
		{
			name: "SFRRPrimaryBackupNHGforTunnelUnviable",
			desc: "SFRR_07: Verify when backup NextHopGroup is also unviable, the cluster traffic is NOT encap-ed and falls back to the BGP routes in the DEFAULT VRF",
			fn:   testPrimaryAndBackupNHGUnviableForTunnel,
		},
		{
			name: "SFRRPrimaryPathUnviableWithooutBNHG",
			desc: "SFRR_08: Verify when original NextHopGroup is unviable and it does not have a backup NextHopGroup, the cluster traffic is NOT encap-ed and falls back to the BGP routes in the DEFAULT VRF.",
			fn:   testPrimaryPathUnviableWihoutBackupNHG,
		},
	}

	for _, tc := range test {
		// configure gRIBI client
		c := gribi.Client{
			DUT:         dut,
			FIBACK:      true,
			Persistence: true,
		}

		if err := c.Start(t); err != nil {
			t.Fatalf("gRIBI Connection can not be established")
		}

		defer c.Close(t)
		c.BecomeLeader(t)

		// Flush all existing AFT entries on the router
		c.FlushAll(t)

		tcArgs := &testArgs{
			client: &c,
			dut:    dut,
			ate:    otg,
			topo:   topo,
		}

		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Description: %s", tc.desc)
			tc.fn(t, tcArgs)
		})
	}
}

func testTraffic(t *testing.T, args *testArgs, weights []float64, shouldPass bool) {
	flows := []gosnappi.Flow{fa4.getFlow("ipv4", "ip4a1", dscpEncapA1), fa6.getFlow("ipv6", "ip6a1", dscpEncapA1)}
	t.Log("Validate traffic flows")
	validateTrafficFlows(t, args, flows, false, shouldPass)
	if shouldPass {
		t.Log("Validate hierarchical traffic distribution")
		validateTrafficDistribution(t, args.ate, weights)
	}
}
func testTransitTraffic(t *testing.T, args *testArgs, weights []float64, shouldPass bool) {
	flows := []gosnappi.Flow{faTransit.getFlow("ipv4in4", "ip4inipa1", dscpEncapA1), faTransit.getFlow("ipv6in4", "ip6inipa1", dscpEncapA1)}
	t.Log("Validate traffic flows")
	validateTrafficFlows(t, args, flows, false, shouldPass)
	if shouldPass {
		t.Log("Validate hierarchical traffic distribution")
		validateTrafficDistribution(t, args.ate, weights)
	}
}

func testTransitTrafficWithDscp(t *testing.T, args *testArgs, weights []float64, dscp uint32, shouldPass bool) {
	flows := []gosnappi.Flow{faTransit.getFlow("ipv4in4", "ip4inipa1", dscp), faTransit.getFlow("ipv6in4", "ip6inipa1", dscp)}
	t.Log("Validate traffic flows")
	validateTrafficFlows(t, args, flows, false, shouldPass)
	if shouldPass {
		t.Log("Validate hierarchical traffic distribution")
		validateTrafficDistribution(t, args.ate, weights)
	}
}

func configureVIP1(t *testing.T, args *testArgs) {
	args.client.AddNH(t, baseNH(2), "MACwithIp", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Dest: otgPort2DummyIP.IPv4, Mac: magicMac})
	args.client.AddNHG(t, baseNHG(2), map[uint64]uint64{baseNH(2): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(vipIP1, 32), baseNHG(2), deviations.DefaultNetworkInstance(args.dut), deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
}

func configureVIP2(t *testing.T, args *testArgs) {
	args.client.AddNH(t, baseNH(3), "MACwithIp", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Dest: otgPort3DummyIP.IPv4, Mac: magicMac})
	args.client.AddNHG(t, baseNHG(3), map[uint64]uint64{baseNH(3): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(vipIP2, 32), baseNHG(3), deviations.DefaultNetworkInstance(args.dut), deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
}

func configureVIP2BGPPrefix(t *testing.T, args *testArgs, prefix string) {
	args.client.AddNH(t, baseNH(3), prefix, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNHG(t, baseNHG(3), map[uint64]uint64{baseNH(3): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(vipIP2, 32), baseNHG(3), deviations.DefaultNetworkInstance(args.dut), deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
}

func configureVIP3(t *testing.T, args *testArgs) {
	args.client.AddNH(t, baseNH(4), "MACwithIp", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Dest: otgPort4DummyIP.IPv4, Mac: magicMac})
	args.client.AddNHG(t, baseNHG(4), map[uint64]uint64{baseNH(4): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(vipIP3, 32), baseNHG(4), deviations.DefaultNetworkInstance(args.dut), deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
}

func configureVIP3NHGWithTunnel(t *testing.T, args *testArgs) {
	configureVIP3(t, args)

	args.client.AddNH(t, vipNH(3), vipIP3, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNHG(t, vipNHG(3), map[uint64]uint64{vipNH(3): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(tunnelDstIP3, 32), vipNHG(3), vrfRepaired, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	args.client.AddNH(t, tunNH(3), "DecapEncap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Src: ipv4OuterSrc222, Dest: tunnelDstIP3, VrfName: vrfRepaired})
	args.client.AddNHG(t, tunNHG(3), map[uint64]uint64{tunNH(3): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
}

func configureVIP3NHGWithRepairTunnelHavingBackupDecapAction(t *testing.T, args *testArgs) {

	// backup to repair
	args.client.AddNH(t, baseNH(5), "Decap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{VrfName: deviations.DefaultNetworkInstance(args.dut)})
	args.client.AddNHG(t, baseNHG(5), map[uint64]uint64{baseNH(5): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	configureVIP3(t, args)
	// repair path
	args.client.AddNH(t, vipNH(3), vipIP3, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNHG(t, vipNHG(3), map[uint64]uint64{vipNH(3): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: baseNHG(5)})
	args.client.AddIPv4(t, cidr(tunnelDstIP3, 32), vipNHG(3), vrfRepaired, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNH(t, tunNH(3), "DecapEncap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Src: ipv4OuterSrc222, Dest: tunnelDstIP3, VrfName: vrfRepaired})
	args.client.AddNHG(t, tunNHG(3), map[uint64]uint64{tunNH(3): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
}

const (
	// for Interface prefix
	baseNHOffset  = 0
	baseNHGOffset = 100
	// for VIP prefix
	vipNHOffset  = 200
	vipNHGOffset = 300
	// for Tunnel Prefix
	tunNHOffset  = 400
	tunNHGOffset = 500
	// for encap
	encapNHOffset  = 600
	encapNHGOffset = 700
	// for decap
	decapNHOffset  = 800
	decapNHGOffset = 900
)

func baseNH(i uint64) uint64   { return i + baseNHOffset }
func baseNHG(i uint64) uint64  { return i + baseNHGOffset }
func vipNH(i uint64) uint64    { return i + vipNHOffset }
func vipNHG(i uint64) uint64   { return i + vipNHGOffset }
func tunNH(i uint64) uint64    { return i + tunNHOffset }
func tunNHG(i uint64) uint64   { return i + tunNHGOffset }
func encapNH(i uint64) uint64  { return i + encapNHOffset }
func encapNHG(i uint64) uint64 { return i + encapNHGOffset }
func decapNH(i uint64) uint64  { return i + decapNHOffset }
func decapNHG(i uint64) uint64 { return i + decapNHGOffset }

func testTransitTrafficNoMatchInTransitVrf(t *testing.T, args *testArgs) {
	configFallBackVrf(t, args.dut, []string{vrfTransit})
	configIPv4DefaultRoute(t, args.dut, cidr(tunnelDstIP1, 32), otgPort2.IPv4)
	defer gnmi.Delete(t, args.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(args.dut)).Static(cidr(innerV4DstIP, 32)).Config())

	configureVIP1(t, args)
	//backup NHG
	// if want to pass traffic as is (ipinip)
	//args.client.AddNH(t, baseNH(3), "VRFOnly", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{VrfName: deviations.DefaultNetworkInstance(args.dut)})
	// if want to decap and then pass the transit traffic
	args.client.AddNH(t, baseNH(3), "Decap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{VrfName: deviations.DefaultNetworkInstance(args.dut)})
	args.client.AddNHG(t, baseNHG(3), map[uint64]uint64{baseNH(3): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	args.client.AddNH(t, vipNH(1), vipIP1, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNHG(t, vipNHG(1), map[uint64]uint64{vipNH(1): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: baseNHG(3)})
	args.client.AddIPv4(t, cidr(tunnelDstIP1, 32), vipNHG(1), vrfTransit, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	weights := []float64{1, 0, 0, 0}
	testTransitTraffic(t, args, weights, true)
	// note - traffic is expected to drop if transit vrf does not have prefix match
	t.Log("Delete tunnel prefix from transit vrf and verify traffic follows default route in the vrf")
	args.client.DeleteIPv4(t, cidr(tunnelDstIP1, 32), vrfTransit, fluent.InstalledInFIB)
	weights = []float64{1, 0, 0, 0}
	testTransitTraffic(t, args, weights, true)

	t.Log("Shutdown primary path for TransitVrf tunnel and verify traffic goes via backup NHG")
	gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port2").Name()).Subinterface(0).Enabled().Config(), false)
	defer gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port2").Name()).Subinterface(0).Enabled().Config(), true)

	configDefaultRoute(t, args.dut, cidr(innerV4DstIP, 32), otgPort5.IPv4, cidr(InnerV6DstIP, 128), otgPort5.IPv6)
	defer gnmi.Delete(t, args.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(args.dut)).Static(cidr(innerV4DstIP, 32)).Config())
	defer gnmi.Delete(t, args.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(args.dut)).Static(cidr(InnerV6DstIP, 128)).Config())

	weights = []float64{0, 0, 0, 1}
	testTransitTraffic(t, args, weights, true)

}

// check with developer
func testTransitTrafficNHGUnviableSendViaRepairTunnel(t *testing.T, args *testArgs) {
	configFallBackVrf(t, args.dut, []string{vrfEncapA})
	configIPv4DefaultRoute(t, args.dut, cidr(tunnelDstIP1, 32), otgPort2.IPv4)
	defer gnmi.Delete(t, args.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(args.dut)).Static(cidr(innerV4DstIP, 32)).Config())

	configureVIP1(t, args)
	configureVIP3NHGWithTunnel(t, args)

	args.client.AddNH(t, vipNH(1), vipIP1, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNHG(t, vipNHG(1), map[uint64]uint64{vipNH(1): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: tunNHG(3)})
	args.client.AddIPv4(t, cidr(tunnelDstIP1, 32), vipNHG(1), vrfTransit, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	weights := []float64{1, 0, 0, 0}
	testTransitTraffic(t, args, weights, true)

	t.Log("Shutdown primary path for TransitVrf tunnel and verify traffic goes via repair path tunnel")
	gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port2").Name()).Subinterface(0).Enabled().Config(), false)
	defer gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port2").Name()).Subinterface(0).Enabled().Config(), true)

	configIPv4DefaultRoute(t, args.dut, cidr(tunnelDstIP3, 32), otgPort4.IPv4)
	defer gnmi.Delete(t, args.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(args.dut)).Static(cidr(innerV4DstIP, 32)).Config())

	weights = []float64{0, 0, 1, 0}
	testTransitTraffic(t, args, weights, true)
}

// check with developer
func testTransitTrafficRepairedNHGUnviableSrc222(t *testing.T, args *testArgs) {
	oSrcIp := faTransit.src
	faTransit.src = ipv4OuterSrc222
	oDst := faTransit.dst
	faTransit.dst = tunnelDstIP3
	defer func() { faTransit.src = oSrcIp; faTransit.dst = oDst }()

	configureVIP3(t, args)

	args.client.AddNH(t, vipNH(3), "Decap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNHG(t, vipNHG(3), map[uint64]uint64{vipNH(3): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(tunnelDstIP3, 32), vipNHG(3), vrfRepaired, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	t.Log("Shutdown primary path for TransitVrf tunnel and verify traffic goes via repair path tunnel")
	gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port4").Name()).Subinterface(0).Enabled().Config(), false)
	defer gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port4").Name()).Subinterface(0).Enabled().Config(), true)

	configDefaultRoute(t, args.dut, cidr(innerV4DstIP, 32), otgPort5.IPv4, cidr(InnerV6DstIP, 128), otgPort5.IPv6)
	defer gnmi.Delete(t, args.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(args.dut)).Static(cidr(innerV4DstIP, 32)).Config())
	defer gnmi.Delete(t, args.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(args.dut)).Static(cidr(InnerV6DstIP, 128)).Config())

	weights := []float64{0, 0, 0, 1}
	testTransitTraffic(t, args, weights, true)

}

// TFRR_01
func testTransitTrafficDecapSrc222(t *testing.T, args *testArgs) {
	oSrcIp := faTransit.src
	oDstIp := faTransit.dst
	faTransit.src = ipv4OuterSrc222
	faTransit.dst = tunnelDstIP1

	defer func() { faTransit.src = oSrcIp; faTransit.dst = oDstIp }()

	args.client.AddNH(t, vipNH(1), "Decap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{VrfName: deviations.DefaultNetworkInstance(args.dut)})
	args.client.AddNHG(t, vipNHG(1), map[uint64]uint64{vipNH(1): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(tunnelDstIP1, 32), vipNHG(1), vrfDecap, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	configDefaultRoute(t, args.dut, cidr(innerV4DstIP, 32), otgPort2.IPv4, cidr(InnerV6DstIP, 128), otgPort2.IPv6)
	defer gnmi.Delete(t, args.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(args.dut)).Static(cidr(innerV4DstIP, 32)).Config())
	defer gnmi.Delete(t, args.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(args.dut)).Static(cidr(InnerV6DstIP, 128)).Config())

	weights := []float64{1, 0, 0, 0}
	testTransitTrafficWithDscp(t, args, weights, dscpEncapNoMatch, true)

}

// TFRR_05
func testTransitTrafficNonTETraffic(t *testing.T, args *testArgs) {
	oSrcIp := faTransit.src
	oDstIp := faTransit.dst
	faTransit.src = ipv4OuterSrcIpInIp
	faTransit.dst = tunnelDstIP1

	defer func() { faTransit.src = oSrcIp; faTransit.dst = oDstIp }()

	args.client.AddNH(t, vipNH(1), "Decap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{VrfName: deviations.DefaultNetworkInstance(args.dut)})
	args.client.AddNHG(t, vipNHG(1), map[uint64]uint64{vipNH(1): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(tunnelDstIP1, 32), vipNHG(1), vrfDecap, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	// non-te traffic should not get decapped
	configIPv4DefaultRoute(t, args.dut, cidr(tunnelDstIP1, 32), otgPort5.IPv4)
	defer gnmi.Delete(t, args.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(args.dut)).Static(cidr(tunnelDstIP1, 32)).Config())

	weights := []float64{0, 0, 0, 1}
	testTransitTrafficWithDscp(t, args, weights, dscpEncapNoMatch, true)

}

// TFRR_09
func testTransitFRRNoMatchDecapSrc111(t *testing.T, args *testArgs) {
	oSrcIp := faTransit.src
	oDstIp := faTransit.dst
	faTransit.src = ipv4OuterSrc111
	faTransit.dst = tunnelDstIP1

	defer func() { faTransit.src = oSrcIp; faTransit.dst = oDstIp }()

	// Dont program correct prefix in decap vrf
	configureVIP1(t, args)
	args.client.AddNH(t, vipNH(1), "Decap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{VrfName: deviations.DefaultNetworkInstance(args.dut)})
	args.client.AddNHG(t, vipNHG(1), map[uint64]uint64{vipNH(1): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(tunnelDstIP2, 32), vipNHG(1), vrfDecap, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	// Install prefix in transit vrf
	configureVIP3(t, args)
	args.client.AddNH(t, vipNH(3), vipIP3, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{VrfName: deviations.DefaultNetworkInstance(args.dut)})
	args.client.AddNHG(t, vipNHG(3), map[uint64]uint64{vipNH(3): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(tunnelDstIP1, 32), vipNHG(3), vrfTransit, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	weights := []float64{0, 0, 1, 0}
	testTransitTrafficWithDscp(t, args, weights, dscpEncapA1, true)

}

// TFRR_10 covered using testTransitTrafficNHGUnviableSendViaRepairTunnel

// TFRR_11 -WIP -> check with de
func testTransitFRRPrimaryNHGAndDecapEncapTunnelDown(t *testing.T, args *testArgs) {
	oSrcIp := faTransit.src
	oDstIp := faTransit.dst
	faTransit.src = ipv4OuterSrc111
	faTransit.dst = tunnelDstIP1

	defer func() { faTransit.src = oSrcIp; faTransit.dst = oDstIp }()

	configureVIP3NHGWithRepairTunnelHavingBackupDecapAction(t, args)
	configureVIP2(t, args)
	args.client.AddNH(t, vipNH(2), vipIP2, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{VrfName: deviations.DefaultNetworkInstance(args.dut)})
	args.client.AddNHG(t, vipNHG(2), map[uint64]uint64{vipNH(2): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: tunNHG(3)})
	args.client.AddIPv4(t, cidr(tunnelDstIP1, 32), vipNHG(2), vrfTransit, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	// verify traffic passes through primary NHG
	weights := []float64{0, 1, 0, 0}
	testTransitTrafficWithDscp(t, args, weights, dscpEncapA1, true)

	t.Log("Shutdown primary path for TransitVrf tunnel and verify traffic goes via repair path tunnel")
	gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port3").Name()).Subinterface(0).Enabled().Config(), false)
	defer gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port3").Name()).Subinterface(0).Enabled().Config(), true)

	weights = []float64{0, 0, 1, 0}
	testTransitTrafficWithDscp(t, args, weights, dscpEncapA1, true)

	t.Log("Shutdown repair tunnel path also and verify traffic passes through default vrf")
	gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port4").Name()).Subinterface(0).Enabled().Config(), false)
	defer gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port4").Name()).Subinterface(0).Enabled().Config(), true)

	configDefaultRoute(t, args.dut, cidr(innerV4DstIP, 32), otgPort5.IPv4, cidr(InnerV6DstIP, 128), otgPort5.IPv6)
	defer gnmi.Delete(t, args.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(args.dut)).Static(cidr(innerV4DstIP, 32)).Config())
	defer gnmi.Delete(t, args.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(args.dut)).Static(cidr(InnerV6DstIP, 128)).Config())

	weights = []float64{0, 0, 0, 1}
	testTransitTrafficWithDscp(t, args, weights, dscpEncapA1, true)

}

// TFRR_12a, TFRR_13, TFRR_14
func testTransitFRRMachInDecapThenEncapedThenMachInTransitVrfThenPrimaryNHGAndDecapEncapTunnelDown(t *testing.T, args *testArgs) {
	oSrcIp := faTransit.src
	oDstIp := faTransit.dst
	faTransit.src = ipv4OuterSrc111
	faTransit.dst = tunnelDstIP1

	defer func() { faTransit.src = oSrcIp; faTransit.dst = oDstIp }()

	// match in decap vrf, decap traffic and schedule to match in encap vrf
	args.client.AddNH(t, decapNH(1), "Decap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{VrfName: vrfEncapA})
	args.client.AddNHG(t, decapNHG(1), map[uint64]uint64{decapNH(1): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(tunnelDstIP1, 32), decapNHG(1), vrfDecap, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	// encap start
	args.client.AddNH(t, encapNH(1), "Encap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Src: ipv4OuterSrc111, Dest: tunnelDstIP2, VrfName: vrfTransit})
	args.client.AddNHG(t, encapNHG(1), map[uint64]uint64{encapNH(1): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	// inner hdr prefixes
	args.client.AddIPv4(t, cidr(innerV4DstIP, 32), encapNHG(1), vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv6(t, cidr(InnerV6DstIP, 128), encapNHG(1), vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	// configure repair path with backup
	configureVIP3NHGWithRepairTunnelHavingBackupDecapAction(t, args)

	// transit path
	configureVIP2(t, args)
	args.client.AddNH(t, vipNH(2), vipIP2, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{VrfName: deviations.DefaultNetworkInstance(args.dut)})
	args.client.AddNHG(t, vipNHG(2), map[uint64]uint64{vipNH(2): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: tunNHG(3)})
	args.client.AddIPv4(t, cidr(tunnelDstIP2, 32), vipNHG(2), vrfTransit, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	// verify traffic passes through primary NHG
	weights := []float64{0, 1, 0, 0}
	testTransitTrafficWithDscp(t, args, weights, dscpEncapA1, true)

	t.Log("Shutdown primary path for TransitVrf tunnel and verify traffic goes via repair path tunnel")
	gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port3").Name()).Subinterface(0).Enabled().Config(), false)
	defer gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port3").Name()).Subinterface(0).Enabled().Config(), true)

	weights = []float64{0, 0, 1, 0}
	testTransitTrafficWithDscp(t, args, weights, dscpEncapA1, true)

	t.Log("Shutdown repair tunnel path also and verify traffic passes through default vrf")
	gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port4").Name()).Subinterface(0).Enabled().Config(), false)
	defer gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port4").Name()).Subinterface(0).Enabled().Config(), true)

	// configure default route
	configDefaultRoute(t, args.dut, cidr(innerV4DstIP, 32), otgPort5.IPv4, cidr(InnerV6DstIP, 128), otgPort5.IPv6)
	defer gnmi.Delete(t, args.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(args.dut)).Static(cidr(innerV4DstIP, 32)).Config())
	defer gnmi.Delete(t, args.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(args.dut)).Static(cidr(InnerV6DstIP, 128)).Config())

	weights = []float64{0, 0, 0, 1}
	testTransitTrafficWithDscp(t, args, weights, dscpEncapA1, true)

}

// TFRR_12b
func testTransitFRRForRepairPathWithSrc222(t *testing.T, args *testArgs) {
	oSrcIp := faTransit.src
	oDstIp := faTransit.dst
	faTransit.src = ipv4OuterSrc222
	faTransit.dst = tunnelDstIP3

	defer func() { faTransit.src = oSrcIp; faTransit.dst = oDstIp }()

	// miss in decap vrf and src _222 should schedule traffic for repair vrf
	args.client.AddNH(t, decapNH(1), "Decap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{VrfName: vrfEncapA})
	args.client.AddNHG(t, decapNHG(1), map[uint64]uint64{decapNH(1): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	//args.client.AddIPv4(t, cidr(tunnelDstIP1, 32), decapNHG(1), vrfDecap, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	// encap start
	args.client.AddNH(t, encapNH(1), "Encap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Src: ipv4OuterSrc111, Dest: tunnelDstIP2, VrfName: vrfTransit})
	args.client.AddNHG(t, encapNHG(1), map[uint64]uint64{encapNH(1): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	// inner hdr prefixes
	args.client.AddIPv4(t, cidr(innerV4DstIP, 32), encapNHG(1), vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv6(t, cidr(InnerV6DstIP, 128), encapNHG(1), vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	// configure repaired path with backup
	// backup to repaired
	args.client.AddNH(t, baseNH(5), "Decap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{VrfName: deviations.DefaultNetworkInstance(args.dut)})
	args.client.AddNHG(t, baseNHG(5), map[uint64]uint64{baseNH(5): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, "0.0.0.0/0", baseNHG(5), "DECAP", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	configureVIP3(t, args)
	// repaired path
	args.client.AddNH(t, vipNH(3), vipIP3, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNHG(t, vipNHG(3), map[uint64]uint64{vipNH(3): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: baseNHG(5)})
	args.client.AddIPv4(t, cidr(tunnelDstIP3, 32), vipNHG(3), vrfRepaired, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	// verify traffic passes through primary NHG
	weights := []float64{0, 0, 1, 0}
	testTransitTrafficWithDscp(t, args, weights, dscpEncapA1, true)

	t.Log("Shutdown repair tunnel path also and verify traffic passes through default vrf")
	gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port4").Name()).Subinterface(0).Enabled().Config(), false)
	defer gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port4").Name()).Subinterface(0).Enabled().Config(), true)

	// configure default route
	configDefaultRoute(t, args.dut, cidr(innerV4DstIP, 32), otgPort5.IPv4, cidr(InnerV6DstIP, 128), otgPort5.IPv6)
	defer gnmi.Delete(t, args.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(args.dut)).Static(cidr(innerV4DstIP, 32)).Config())
	defer gnmi.Delete(t, args.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(args.dut)).Static(cidr(InnerV6DstIP, 128)).Config())

	weights = []float64{0, 0, 0, 1}
	testTransitTrafficWithDscp(t, args, weights, dscpEncapA1, true)
}

func TestTransitFrr(t *testing.T) {
	// Configure DUT
	dut := ondatra.DUT(t, "dut")
	configureDUT(t, dut, false)

	// Configure ATE
	otg := ondatra.ATE(t, "ate")
	topo := configureOTG(t, otg)

	test := []struct {
		name string
		desc string
		fn   func(t *testing.T, args *testArgs)
	}{
		{
			name: "TestTransitTrafficNoPrefixInTransitVrf",
			desc: "TTT_02: Verify if there is no match for the tunnel IP in the TRANSIT_TE_VRF, then the packet is decaped and forwarded according to the routes in the DEFAULT VRF.",
			fn:   testTransitTrafficNoMatchInTransitVrf,
		},
		{
			name: "TestTransitTrafficTransitNHGDownRepaiPathTunnelHandlesTraffic",
			desc: "TTT_03: If the NextHopGroup referenced by IPv4Entry in TRANSIT_TE_VRF is unviable (POPGate FRR behavior)and if the source IP is not 222.222.222.222, then verify this unviable tunnel is repaired by re-encapping the packet to a repair tunnel as specified in the REPAIR_TE_VRF.",
			fn:   testTransitTrafficNHGUnviableSendViaRepairTunnel,
		},
		{
			name: "TestTransitTrafficTransitNHGDownRepaiPathTunnelHandlesTraffic",
			desc: "TTT_04: If the NextHopGroup referenced by IPv4Entry in REPAIRED_TE_VRF is unviable (POPGate FRR behavior)and if the source IP is 222.222.222.222, then verify the packet is decapped and forwarded according to the BGP routes in the DEFAULT VRF. This is achieved by looking up the route for this packet in the REPAIRED_TE_VRF instead of the TRANSIT_TE_VRF.",
			fn:   testTransitTrafficRepairedNHGUnviableSrc222,
		},
		{
			name: "TestTransitFrrDecapSrc222",
			desc: "TFRR_01: Verify Tunnel traffic (6in4 and 4in4) arriving on WAN intefrace with source addresses 111.111.111.111 or 222.222.222.222 is decapped when matching entry exists in DECAP_TE_VRF.",
			fn:   testTransitTrafficDecapSrc222,
		},
		{
			name: "TestTransitFrrWithNonTETrafficOnWanInterface",
			desc: "TFRR_05: Verify TE disabled traffic arriving on the WAN interfaces, is routed according to the BGP routes in the DEFAULT VRF.",
			fn:   testTransitTrafficNonTETraffic,
		},
		{
			name: "TestTransitFrrWithNoMatchInDecapAndWithSrc111",
			desc: "TFRR_09: Verify TE traffic (6in4 and 4in4) arriving on the WAN or cluster facing interfaces is forwarded according to the rules in the TE_TRANSIT_VRF when there are no matching entries in the DECAP_TE_VRF and outer header IP_SrcAddr is of the format _._._.111.",
			fn:   testTransitFRRNoMatchDecapSrc111,
		},
		{
			name: "TestTransitFrrWithPrimaryNHGAndDecapEncapTunnelDown",
			desc: "TFRR_11: Verify for popgate (miss in DECAP_TE_VRF, hit in TE_TRANSIT_VRF) case, if the re-encap tunnels are also unviable, the packets are decapped and routed according to the BGP routes in the DEFAULT VRF.",
			fn:   testTransitFRRPrimaryNHGAndDecapEncapTunnelDown,
		},
		{
			name: "TestTransitFrrWithMatchInDecapThenEncapThenTransitPathPrimaryNHGAndDecapEncapTunnelDown",
			desc: "TFRR_12a: Verify for dcgate (hit in DECAP_TE_VRF) case, if the re-encap tunnels are also unviable, the packets are decapped and routed according to the BGP routes in the DEFAULT VRF.",
			fn:   testTransitFRRMachInDecapThenEncapedThenMachInTransitVrfThenPrimaryNHGAndDecapEncapTunnelDown,
		},
		{
			name: "TestTransitFrrRepairedPathWithSrc222",
			desc: "TFRR_12b: Verify Tunneled traffic that has already been repaired (identified by the source IP of 222.222.222.222) is forwarded according to the rules in the REPAIRED_TE_VRF.",
			fn:   testTransitFRRForRepairPathWithSrc222,
		},
	}

	for _, tc := range test {
		// configure gRIBI client
		c := gribi.Client{
			DUT:         dut,
			FIBACK:      true,
			Persistence: true,
		}

		if err := c.Start(t); err != nil {
			t.Fatalf("gRIBI Connection can not be established")
		}

		defer c.Close(t)
		c.BecomeLeader(t)

		// Flush all existing AFT entries on the router
		c.FlushAll(t)

		tcArgs := &testArgs{
			client: &c,
			dut:    dut,
			ate:    otg,
			topo:   topo,
		}

		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Description: %s", tc.desc)
			tc.fn(t, tcArgs)
		})
	}
}

// // ---- reginalization tunnels

type bgpNeighbor struct {
	as         uint32
	neighborip string
	isV4       bool
}

var bgpNbr1 = bgpNeighbor{
	as:         ateAS,
	neighborip: otgPort1.IPv4,
	isV4:       true,
}
var bgpNbr2 = bgpNeighbor{
	as:         dutAS,
	neighborip: otgPort2.IPv4,
	isV4:       true,
}

func bgpCreateNbr(t *testing.T, localAs uint32, dut *ondatra.DUTDevice, nbrs []*bgpNeighbor) *oc.NetworkInstance_Protocol {
	// t.Helper()
	dutOcRoot := &oc.Root{}
	ni1 := dutOcRoot.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niProto := ni1.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := niProto.GetOrCreateBgp()
	global := bgp.GetOrCreateGlobal()
	global.RouterId = ygot.String(dutPort2.IPv4)
	global.As = ygot.Uint32(localAs)

	for _, nbr := range nbrs {
		nv4 := bgp.GetOrCreateNeighbor(nbr.neighborip)
		nv4.PeerAs = ygot.Uint32(nbr.as)
		nv4.Enabled = ygot.Bool(true)

		global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
		global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)

		if nbr.isV4 == true {
			af4 := nv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
			af4.Enabled = ygot.Bool(true)
			rpl := af4.GetOrCreateApplyPolicy()
			rpl.ImportPolicy = []string{rplName}
			rpl.ExportPolicy = []string{rplName}
		} else {
			af6 := nv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
			af6.Enabled = ygot.Bool(true)
			rpl := af6.GetOrCreateApplyPolicy()
			rpl.ImportPolicy = []string{rplName}
			rpl.ExportPolicy = []string{rplName}
		}
	}
	return niProto
}

var (
	otgPort1V4Peer = dutPort1.IPv4
	otgPort1V6Peer = dutPort1.IPv6
	otgPort2V4Peer = dutPort2.IPv4
	otgPort2V6Peer = dutPort2.IPv6
	ateAS          = uint32(65001)
	dutAS          = uint32(65000)
)

func configureOTGBgp(t *testing.T, otg *otg.OTG, topo gosnappi.Config, otgPeerList []string) {
	otgBgpRtr := make(map[string]gosnappi.DeviceBgpRouter)
	for _, d := range topo.Devices().Items() {
		fmt.Println(" device item :", d.Name())
		switch d.Name() {
		case otgPort1.Name:
			otgBgpRtr[otgPort1.Name] = d.Bgp().SetRouterId(otgPort1.IPv4)
		case otgPort2.Name:
			otgBgpRtr[otgPort2.Name] = d.Bgp().SetRouterId(otgPort2.IPv4)
		}
	}
	// BGP seesion
	for _, peer := range otgPeerList {
		switch peer {
		case otgPort1V4Peer:
			iDut1Bgp4Peer := otgBgpRtr[otgPort1.Name].Ipv4Interfaces().Add().SetIpv4Name(otgPort1.Name + ".IPv4").Peers().Add().SetName(otgPort1V4Peer)
			iDut1Bgp4Peer.SetPeerAddress(dutPort1.IPv4).SetAsNumber(ateAS).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
			iDut1Bgp4Peer.Capability().SetIpv4UnicastAddPath(true).SetIpv6UnicastAddPath(true)
			iDut1Bgp4Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)
			configureBGPv4Routes(iDut1Bgp4Peer, otgPort1.IPv4, bgp4Routes1, startPrefix1, 10)
		case otgPort1V6Peer:
			iDut1Bgp6Peer := otgBgpRtr[otgPort1.Name].Ipv6Interfaces().Add().SetIpv6Name(otgPort1.Name + ".IPv6").Peers().Add().SetName(otgPort1V6Peer)
			iDut1Bgp6Peer.SetPeerAddress(dutPort1.IPv6).SetAsNumber(ateAS).SetAsType(gosnappi.BgpV6PeerAsType.EBGP)
			iDut1Bgp6Peer.Capability().SetIpv4UnicastAddPath(true).SetIpv6UnicastAddPath(true)
			iDut1Bgp6Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)
		case otgPort2V4Peer:
			iDut2Bgp4Peer := otgBgpRtr[otgPort2.Name].Ipv4Interfaces().Add().SetIpv4Name(otgPort2.Name + ".IPv4").Peers().Add().SetName(otgPort2V4Peer)
			iDut2Bgp4Peer.SetPeerAddress(dutPort2.IPv4).SetAsNumber(dutAS).SetAsType(gosnappi.BgpV4PeerAsType.IBGP)
			iDut2Bgp4Peer.Capability().SetIpv4UnicastAddPath(true).SetIpv6UnicastAddPath(true)
			iDut2Bgp4Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)
			configureBGPv4Routes(iDut2Bgp4Peer, otgPort2.IPv4, bgp4Routes2, startPrefix2, 10)
		case otgPort2V6Peer:
			iDut2Bgp6Peer := otgBgpRtr[otgPort2.Name].Ipv6Interfaces().Add().SetIpv6Name(otgPort2.Name + ".IPv6").Peers().Add().SetName(otgPort2V6Peer)
			iDut2Bgp6Peer.SetPeerAddress(dutPort2.IPv6).SetAsNumber(dutAS).SetAsType(gosnappi.BgpV6PeerAsType.IBGP)
			iDut2Bgp6Peer.Capability().SetIpv4UnicastAddPath(true).SetIpv6UnicastAddPath(true)
			iDut2Bgp6Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)
		}
	}

	t.Logf("Pushing config to OTG and starting protocols...")
	otg.PushConfig(t, topo)
	otg.StartProtocols(t)
	time.Sleep(30 * time.Second)
}

var (
	advertisedRoutesv4Prefix = uint32(32)
	rplName                  = "ALLOW"
	startPrefix1             = "201.0.0.1"
	startPrefix2             = "202.0.0.1"
	bgp4Routes1              = "BGPv4_1"
	bgp4Routes2              = "BGPv4_2"
	routeCount               = 10
	installedRoutes          = uint32(routeCount)
)

func configureBGPv4Routes(peer gosnappi.BgpV4Peer, ipv4 string, name string, prefix string, count uint32) {
	routes := peer.V4Routes().Add().SetName(name)
	routes.SetNextHopIpv4Address(ipv4).
		SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).
		SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)
	routes.Addresses().Add().
		SetAddress(prefix).
		SetPrefix(advertisedRoutesv4Prefix).
		SetCount(count)
}

func advertiseBGPRoutes(t *testing.T, conf gosnappi.Config, routeNames []string) {

	ate := ondatra.ATE(t, "ate")
	otg := ate.OTG()
	cs := gosnappi.NewControlState()
	cs.Protocol().Route().SetNames(routeNames).SetState(gosnappi.StateProtocolRouteState.ADVERTISE)
	otg.SetControlState(t, cs)

}

func configureRoutePolicy(t *testing.T, dut *ondatra.DUTDevice, name string, pr oc.E_RoutingPolicy_PolicyResultType) {
	d := &oc.Root{}
	rp := d.GetOrCreateRoutingPolicy()
	pd := rp.GetOrCreatePolicyDefinition(name)
	st, err := pd.AppendNewStatement("id-1")
	if err != nil {
		t.Fatal(err)
	}
	st.GetOrCreateActions().PolicyResult = pr
	gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)
}

func verifyRoutes(t *testing.T, dut *ondatra.DUTDevice) {
	t.Log("Waiting for BGPv4 neighbor to establish...")

	compare := func(val *ygnmi.Value[uint32]) bool {
		c, ok := val.Val()
		return ok && c == installedRoutes
	}
	t.Log("Verifying BGP state")
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	for _, nbr := range []string{otgPort1.IPv4, otgPort2.IPv4} {
		prefixes := statePath.Neighbor(nbr).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Prefixes()
		if got, ok := gnmi.Watch(t, dut, prefixes.Received().State(), 2*time.Minute, compare).Await(t); !ok {
			t.Errorf("Received prefixes v4 mismatch: got %v, want %v", got, installedRoutes)
		}
		if got, ok := gnmi.Watch(t, dut, prefixes.Installed().State(), 2*time.Minute, compare).Await(t); !ok {
			t.Errorf("Installed prefixes v4 mismatch: got %v, want %v", got, installedRoutes)
		}
	}
}
func TestRegionalization(t *testing.T) {

	dut := ondatra.DUT(t, "dut")
	configureDUT(t, dut, true)
	defer gnmi.Delete(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding().Config())

	// Configure ATE
	ate := ondatra.ATE(t, "ate")
	topo := configureOTG(t, ate)

	otg := ate.OTG()

	c := &oc.Root{}
	ni := c.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	ni.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE
	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Config(), ni)

	configureRoutePolicy(t, dut, rplName, oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)

	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	dutConf := bgpCreateNbr(t, dutAS, dut, []*bgpNeighbor{&bgpNbr1, &bgpNbr2})
	gnmi.Replace(t, dut, dutConfPath.Config(), dutConf)

	configureOTGBgp(t, otg, topo, []string{otgPort1V4Peer, otgPort2V4Peer})
	advertiseBGPRoutes(t, topo, []string{bgp4Routes1, bgp4Routes2})
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	nbrPath := statePath.Neighbor(otgPort1V4Peer)
	compare := func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
		state, ok := val.Val()
		if ok {
			return state == oc.Bgp_Neighbor_SessionState_ESTABLISHED
		}
		return false
	}
	gnmi.Watch(t, dut, nbrPath.SessionState().State(), 2*time.Minute, compare).Await(t)
	nbrPath = statePath.Neighbor(otgPort2V4Peer)
	gnmi.Watch(t, dut, nbrPath.SessionState().State(), 2*time.Minute, compare).Await(t)
	verifyRoutes(t, dut)
	// additional sleep time
	time.Sleep(time.Minute)
	// configure gRIBI client
	client := gribi.Client{
		DUT:         dut,
		FIBACK:      true,
		Persistence: true,
	}

	if err := client.Start(t); err != nil {
		t.Fatalf("gRIBI Connection can not be established")
	}

	defer client.Close(t)
	client.BecomeLeader(t)

	// Flush all existing AFT entries on the router
	client.FlushAll(t)

	args := &testArgs{
		client: &client,
		dut:    dut,
		ate:    ate,
		topo:   topo,
	}

	oSrcIp := faTransit.src
	oDstIp := faTransit.dst
	faTransit.src = ipv4OuterSrc222
	faTransit.dst = tunnelDstIP1
	defer func() { faTransit.src = oSrcIp; faTransit.dst = oDstIp }()

	// match in decap vrf, decap traffic and schedule to match in encap vrf
	args.client.AddNH(t, decapNH(1), "Decap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{VrfName: vrfEncapA})
	args.client.AddNHG(t, decapNHG(1), map[uint64]uint64{decapNH(1): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(tunnelDstIP1, 32), decapNHG(1), vrfDecap, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	// encap start
	args.client.AddNH(t, encapNH(1), "Encap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Src: ipv4OuterSrc111, Dest: tunnelDstIP2, VrfName: vrfTransit})
	args.client.AddNHG(t, encapNHG(1), map[uint64]uint64{encapNH(1): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	// inner hdr prefixes
	args.client.AddIPv4(t, cidr(innerV4DstIP, 32), encapNHG(1), vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv6(t, cidr(InnerV6DstIP, 128), encapNHG(1), vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(ipv4EntryPrefix, ipv4EntryPrefixLen), encapNHG(1), vrfEncapA, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	args.client.AddIPv6(t, cidr(ipv6EntryPrefix, ipv6EntryPrefixLen), encapNHG(1), vrfEncapA, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)

	// // configure repair path with backup
	// configureVIP3NHGWithRepairTunnelHavingBackupDecapAction(t, args)

	// transit path
	configureVIP2BGPPrefix(t, args, startPrefix2)
	args.client.AddNH(t, vipNH(2), vipIP2, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{VrfName: deviations.DefaultNetworkInstance(args.dut)})
	args.client.AddNHG(t, vipNHG(2), map[uint64]uint64{vipNH(2): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	// args.client.AddNHG(t, vipNHG(2), map[uint64]uint64{vipNH(2): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: tunNHG(3)})
	args.client.AddIPv4(t, cidr(tunnelDstIP2, 32), vipNHG(2), vrfTransit, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	// verify traffic passes through primary NHG
	weights := []float64{1, 0, 0, 0}
	testTransitTrafficWithDscp(t, args, weights, dscpEncapA1, true)

	// check cluster traffic also passes -TERS_03
	testTraffic(t, args, weights, true)
}
