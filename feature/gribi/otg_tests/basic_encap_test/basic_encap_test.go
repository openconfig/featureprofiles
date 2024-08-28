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

// Package basic_encap_test implements TE-16.1 of the dcgate vendor testplan
package basic_encap_test

import (
	"fmt"
	"log"
	"math/rand"
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
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/gribigo/client"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygot/ygot"
)

const (
	ipipProtocol       = 4
	ipv6ipProtocol     = 41
	udpProtocol        = 17
	ethertypeIPv4      = oc.PacketMatchTypes_ETHERTYPE_ETHERTYPE_IPV4
	ethertypeIPv6      = oc.PacketMatchTypes_ETHERTYPE_ETHERTYPE_IPV6
	clusterPolicy      = "vrf_selection_policy_c"
	vrfDecap           = "DECAP_TE_VRF"
	vrfTransit         = "TE_VRF_111"
	vrfRepaired        = "TE_VRF_222"
	vrfEncapA          = "ENCAP_TE_VRF_A"
	vrfEncapB          = "ENCAP_TE_VRF_B"
	ipv4PrefixLen      = 30
	ipv6PrefixLen      = 126
	trafficDuration    = 15 * time.Second
	nhg10ID            = 10
	nh201ID            = 201
	nh202ID            = 202
	nhg1ID             = 1
	nh1ID              = 1
	nh2ID              = 2
	nhg2ID             = 2
	nh10ID             = 10
	nh11ID             = 11
	nhg3ID             = 3
	nh100ID            = 100
	nh101ID            = 101
	dscpEncapA1        = 10
	dscpEncapA2        = 18
	dscpEncapB1        = 20
	dscpEncapB2        = 28
	dscpEncapNoMatch   = 30
	magicIp            = "192.168.1.1"
	magicMac           = "02:00:00:00:00:01"
	tunnelDstIP1       = "203.0.113.1"
	tunnelDstIP2       = "203.0.113.2"
	ipv4OuterSrc111    = "198.51.100.111"
	ipv4OuterSrc222    = "198.51.100.222"
	ipv4OuterSrcIpInIp = "198.100.200.123"
	vipIP1             = "192.0.2.111"
	vipIP2             = "192.0.2.222"
	innerV4DstIP       = "198.18.1.1"
	innerV4SrcIP       = "198.18.0.255"
	InnerV6SrcIP       = "2001:DB8::198:1"
	InnerV6DstIP       = "2001:DB8:2:0:192::10"
	ipv4FlowIP         = "138.0.11.8"
	ipv4EntryPrefix    = "138.0.11.0"
	ipv4EntryPrefixLen = 24
	ipv6FlowIP         = "2015:aa8::1"
	ipv6EntryPrefix    = "2015:aa8::"
	ipv6EntryPrefixLen = 64
	ratioTunEncap1     = 0.25 // 1/4
	ratioTunEncap2     = 0.75 // 3/4
	ratioTunEncapTol   = 0.05 // 5/100
	ttl                = uint32(100)
	trfDistTolerance   = 0.02
	// observing on IXIA OTG: Cannot start capture on more than one port belonging to the
	// same resource group or on more than one port behind the same front panel port in the chassis
	otgMutliPortCaptureSupported = false
	seqIDBase                    = uint32(10)
)

var (
	otgDstPorts = []string{"port2", "port3", "port4", "port5"}
	otgSrcPort  = "port1"
	wantWeights = []float64{
		0.0625, // 1/4 * 1/4 - port1
		0.1875, // 1/4 * 3/4 - port2
		0.3,    // 3/4 * 2/5 - port3
		0.45,   // 3/5 * 3/4 - port4
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
		Desc:       "dutPort2",
		IPv4Sec:    "192.0.2.21",
		IPv4LenSec: ipv4PrefixLen,
	}

	otgPort2DummyIP = attrs.Attributes{
		Desc:    "otgPort2",
		IPv4:    "192.0.2.22",
		IPv4Len: ipv4PrefixLen,
	}

	dutPort3DummyIP = attrs.Attributes{
		Desc:       "dutPort3",
		IPv4Sec:    "192.0.2.25",
		IPv4LenSec: ipv4PrefixLen,
	}

	otgPort3DummyIP = attrs.Attributes{
		Desc:    "otgPort3",
		IPv4:    "192.0.2.26",
		IPv4Len: ipv4PrefixLen,
	}

	dutPort4DummyIP = attrs.Attributes{
		Desc:       "dutPort4",
		IPv4Sec:    "192.0.2.29",
		IPv4LenSec: ipv4PrefixLen,
	}

	otgPort4DummyIP = attrs.Attributes{
		Desc:    "otgPort4",
		IPv4:    "192.0.2.30",
		IPv4Len: ipv4PrefixLen,
	}

	dutPort5DummyIP = attrs.Attributes{
		Desc:       "dutPort5",
		IPv4Sec:    "192.0.2.33",
		IPv4LenSec: ipv4PrefixLen,
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
	configureDUT(t, dut)

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
			pattr:              packetAttr{dscp: dscpEncapA1, protocol: ipipProtocol, ttl: 99},
			flows:              []gosnappi.Flow{fa4.getFlow("ipv4", "ip4a1", dscpEncapA1)},
			weights:            wantWeights,
			capturePorts:       otgDstPorts,
			validateEncapRatio: true,
		},
		{
			name:               fmt.Sprintf("Test2 IPv6 Traffic WCMP Encap dscp %d", dscpEncapA1),
			pattr:              packetAttr{dscp: dscpEncapA1, protocol: ipv6ipProtocol, ttl: 99},
			flows:              []gosnappi.Flow{fa6.getFlow("ipv6", "ip6a1", dscpEncapA1)},
			weights:            wantWeights,
			capturePorts:       otgDstPorts,
			validateEncapRatio: true,
		},
		{
			name:  fmt.Sprintf("Test3 IPinIP Traffic WCMP Encap dscp %d", dscpEncapA1),
			pattr: packetAttr{dscp: dscpEncapA1, protocol: ipipProtocol, ttl: 99},
			flows: []gosnappi.Flow{faIPinIP.getFlow("ipv4in4", "ip4in4a1", dscpEncapA1),
				faIPinIP.getFlow("ipv6in4", "ip6in4a1", dscpEncapA1),
			},
			weights:            wantWeights,
			capturePorts:       otgDstPorts,
			validateEncapRatio: true,
		},
		{
			name:               fmt.Sprintf("No Match Dscp %d Traffic", dscpEncapNoMatch),
			pattr:              packetAttr{protocol: udpProtocol, dscp: dscpEncapNoMatch, ttl: 99},
			flows:              []gosnappi.Flow{fa4.getFlow("ipv4", "ip4nm", dscpEncapNoMatch)},
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

	var defaultClassRule = []pbrRule{
		{
			sequence: 17,
			encapVrf: vrfDefault,
		},
	}

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

	if deviations.PfRequireMatchDefaultRule(dut) {
		pbrRules = append(pbrRules, splitDefaultClassRules...)
	} else {
		pbrRules = append(pbrRules, defaultClassRule...)
	}

	return pbrRules
}

// seqIDOffset returns sequence ID offset added with seqIDBase (10), to avoid sequences
// like 1, 10, 11, 12,..., 2, 21, 22, ... while being sent by Ondatra to the DUT.
// It now generates sequences like 11, 12, 13, ..., 19, 20, 21,..., 99.
func seqIDOffset(dut *ondatra.DUTDevice, i uint32) uint32 {
	if deviations.PfRequireSequentialOrderPbrRules(dut) {
		return i + seqIDBase
	}
	return i
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

// configureNetworkInstance creates nonDefaultVRFs
func configureNetworkInstance(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	c := &oc.Root{}
	vrfs := []string{vrfDecap, vrfTransit, vrfRepaired, vrfEncapA, vrfEncapB}
	for _, vrf := range vrfs {
		ni := c.GetOrCreateNetworkInstance(vrf)
		ni.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF
		gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(vrf).Config(), ni)
	}
}

// cidr takes as input the IPv4 address and the Mask and returns the IP string in
// CIDR notation.
func cidr(ipv4 string, ones int) string {
	return ipv4 + "/" + strconv.Itoa(ones)
}

// getPbrPolicy creates PBR rules for cluster
func getPbrPolicy(dut *ondatra.DUTDevice, name string, clusterFacing bool) *oc.NetworkInstance_PolicyForwarding {
	d := &oc.Root{}
	ni := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	pf := ni.GetOrCreatePolicyForwarding()
	p := pf.GetOrCreatePolicy(name)
	p.SetType(oc.Policy_Type_VRF_SELECTION_POLICY)

	for _, pRule := range getPbrRules(dut, clusterFacing) {
		r := p.GetOrCreateRule(seqIDOffset(dut, pRule.sequence))
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
		if deviations.PfRequireMatchDefaultRule(dut) {
			if pRule.etherType != nil {
				r.GetOrCreateL2().Ethertype = pRule.etherType
			}
		}

		if pRule.encapVrf != "" {
			r.GetOrCreateAction().SetNetworkInstance(pRule.encapVrf)
		}
	}
	return pf
}

// configureBaseconfig configures network instances and forwarding policy on the DUT
func configureBaseconfig(t *testing.T, dut *ondatra.DUTDevice) {
	t.Log("Configure VRFs")
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	configureNetworkInstance(t, dut)
	t.Log("Configure Cluster facing VRF selection Policy")
	pf := getPbrPolicy(dut, clusterPolicy, true)
	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding().Config(), pf)
}

func staticARPWithMagicUniversalIP(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	sb := &gnmi.SetBatch{}
	p2 := dut.Port(t, "port2")
	p3 := dut.Port(t, "port3")
	p4 := dut.Port(t, "port4")
	p5 := dut.Port(t, "port5")
	portList := []*ondatra.Port{p2, p3, p4, p5}
	for idx, p := range portList {
		s := &oc.NetworkInstance_Protocol_Static{
			Prefix: ygot.String(magicIp + "/32"),
			NextHop: map[string]*oc.NetworkInstance_Protocol_Static_NextHop{
				strconv.Itoa(idx): {
					Index: ygot.String(strconv.Itoa(idx)),
					InterfaceRef: &oc.NetworkInstance_Protocol_Static_NextHop_InterfaceRef{
						Interface: ygot.String(p.Name()),
					},
				},
			},
		}
		sp := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
		gnmi.BatchUpdate(sb, sp.Static(magicIp+"/32").Config(), s)
		gnmi.BatchUpdate(sb, gnmi.OC().Interface(p.Name()).Config(), configStaticArp(p.Name(), magicIp, magicMac))
	}
	sb.Set(t, dut)
}

// programEntries pushes RIB entries on the DUT required for Encap functionality
func programEntries(t *testing.T, dut *ondatra.DUTDevice, c *gribi.Client) {
	// push RIB entries
	if deviations.GRIBIMACOverrideWithStaticARP(dut) {
		c.AddNH(t, nh10ID, "MACwithIp", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHOptions{Dest: otgPort2DummyIP.IPv4, Mac: magicMac})
		c.AddNH(t, nh11ID, "MACwithIp", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHOptions{Dest: otgPort3DummyIP.IPv4, Mac: magicMac})
		c.AddNH(t, nh100ID, "MACwithIp", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHOptions{Dest: otgPort4DummyIP.IPv4, Mac: magicMac})
		c.AddNH(t, nh101ID, "MACwithIp", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHOptions{Dest: otgPort5DummyIP.IPv4, Mac: magicMac})
		c.AddNHG(t, nhg2ID, map[uint64]uint64{nh10ID: 1, nh11ID: 3}, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
		c.AddNHG(t, nhg3ID, map[uint64]uint64{nh100ID: 2, nh101ID: 3}, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	} else if deviations.GRIBIMACOverrideStaticARPStaticRoute(dut) {
		p2 := dut.Port(t, "port2")
		p3 := dut.Port(t, "port3")
		p4 := dut.Port(t, "port4")
		p5 := dut.Port(t, "port5")
		nh1, op1 := gribi.NHEntry(nh10ID, "MACwithInterface", deviations.DefaultNetworkInstance(dut),
			fluent.InstalledInFIB, &gribi.NHOptions{Interface: p2.Name(), Mac: magicMac, Dest: magicIp})
		nh2, op2 := gribi.NHEntry(nh11ID, "MACwithInterface", deviations.DefaultNetworkInstance(dut),
			fluent.InstalledInFIB, &gribi.NHOptions{Interface: p3.Name(), Mac: magicMac, Dest: magicIp})
		nh3, op3 := gribi.NHEntry(nh100ID, "MACwithInterface", deviations.DefaultNetworkInstance(dut),
			fluent.InstalledInFIB, &gribi.NHOptions{Interface: p4.Name(), Mac: magicMac, Dest: magicIp})
		nh4, op4 := gribi.NHEntry(nh101ID, "MACwithInterface", deviations.DefaultNetworkInstance(dut),
			fluent.InstalledInFIB, &gribi.NHOptions{Interface: p5.Name(), Mac: magicMac, Dest: magicIp})
		nhg1, op5 := gribi.NHGEntry(nhg2ID, map[uint64]uint64{nh10ID: 1, nh11ID: 3},
			deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
		nhg2, op6 := gribi.NHGEntry(nhg3ID, map[uint64]uint64{nh100ID: 2, nh101ID: 3},
			deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
		c.AddEntries(t, []fluent.GRIBIEntry{nh1, nh2, nh3, nh4, nhg1, nhg2},
			[]*client.OpResult{op1, op2, op3, op4, op5, op6})
	} else {
		c.AddNH(t, nh10ID, "MACwithInterface", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHOptions{Interface: dut.Port(t, "port2").Name(), Mac: magicMac})
		c.AddNH(t, nh11ID, "MACwithInterface", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHOptions{Interface: dut.Port(t, "port3").Name(), Mac: magicMac})
		c.AddNH(t, nh100ID, "MACwithInterface", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHOptions{Interface: dut.Port(t, "port4").Name(), Mac: magicMac})
		c.AddNH(t, nh101ID, "MACwithInterface", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHOptions{Interface: dut.Port(t, "port5").Name(), Mac: magicMac})
		c.AddNHG(t, nhg2ID, map[uint64]uint64{nh10ID: 1, nh11ID: 3}, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
		c.AddNHG(t, nhg3ID, map[uint64]uint64{nh100ID: 2, nh101ID: 3}, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	}
	c.AddIPv4(t, cidr(vipIP1, 32), nhg2ID, deviations.DefaultNetworkInstance(dut), deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)

	c.AddIPv4(t, cidr(vipIP2, 32), nhg3ID, deviations.DefaultNetworkInstance(dut), deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)

	nh5, op7 := gribi.NHEntry(nh1ID, vipIP1, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	nh6, op8 := gribi.NHEntry(nh2ID, vipIP2, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	nhg3, op9 := gribi.NHGEntry(nhg1ID, map[uint64]uint64{nh1ID: 1, nh2ID: 3},
		deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	c.AddEntries(t, []fluent.GRIBIEntry{nh5, nh6, nhg3}, []*client.OpResult{op7, op8, op9})

	c.AddIPv4(t, cidr(tunnelDstIP1, 32), nhg1ID, vrfTransit, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	c.AddIPv4(t, cidr(tunnelDstIP2, 32), nhg1ID, vrfTransit, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)

	nh7, op9 := gribi.NHEntry(nh201ID, "Encap", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB,
		&gribi.NHOptions{Src: ipv4OuterSrc111, Dest: tunnelDstIP1, VrfName: vrfTransit})
	nh8, op10 := gribi.NHEntry(nh202ID, "Encap", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB,
		&gribi.NHOptions{Src: ipv4OuterSrc111, Dest: tunnelDstIP2, VrfName: vrfTransit})
	nhg4, op11 := gribi.NHGEntry(nhg10ID, map[uint64]uint64{nh201ID: 1, nh202ID: 3},
		deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	c.AddEntries(t, []fluent.GRIBIEntry{nh7, nh8, nhg4}, []*client.OpResult{op9, op10, op11})
	c.AddIPv4(t, cidr(ipv4EntryPrefix, ipv4EntryPrefixLen), nhg10ID, vrfEncapA, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	c.AddIPv4(t, cidr(ipv4EntryPrefix, ipv4EntryPrefixLen), nhg10ID, vrfEncapB, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	c.AddIPv6(t, cidr(ipv6EntryPrefix, ipv6EntryPrefixLen), nhg10ID, vrfEncapA, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	c.AddIPv6(t, cidr(ipv6EntryPrefix, ipv6EntryPrefixLen), nhg10ID, vrfEncapB, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
}

func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	d := gnmi.OC()
	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")
	p3 := dut.Port(t, "port3")
	p4 := dut.Port(t, "port4")
	p5 := dut.Port(t, "port5")
	portList := []*ondatra.Port{p1, p2, p3, p4, p5}

	// configure interfaces
	for idx, a := range []attrs.Attributes{dutPort1, dutPort2, dutPort3, dutPort4, dutPort5} {
		p := portList[idx]
		intf := a.NewOCInterface(p.Name(), dut)
		if p.PMD() == ondatra.PMD100GBASEFR && dut.Vendor() != ondatra.CISCO && dut.Vendor() != ondatra.JUNIPER {
			e := intf.GetOrCreateEthernet()
			e.AutoNegotiate = ygot.Bool(false)
			e.DuplexMode = oc.Ethernet_DuplexMode_FULL
			e.PortSpeed = oc.IfEthernet_ETHERNET_SPEED_SPEED_100GB
		}
		gnmi.Replace(t, dut, d.Interface(p.Name()).Config(), intf)
	}

	// configure base PBF policies and network-instances
	configureBaseconfig(t, dut)

	// apply PBF to src interface.
	applyForwardingPolicy(t, dut, p1.Name())
	if deviations.GRIBIMACOverrideWithStaticARP(dut) {
		staticARPWithSecondaryIP(t, dut)
	} else if deviations.GRIBIMACOverrideStaticARPStaticRoute(dut) {
		staticARPWithMagicUniversalIP(t, dut)
	}
}

// applyForwardingPolicy applies the forwarding policy on the interface.
func applyForwardingPolicy(t *testing.T, dut *ondatra.DUTDevice, ingressPort string) {
	t.Logf("Applying forwarding policy on interface %v ... ", ingressPort)
	d := &oc.Root{}
	interfaceID := ingressPort
	if deviations.InterfaceRefInterfaceIDFormat(dut) {
		interfaceID = ingressPort + ".0"
	}
	pfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding().Interface(interfaceID)
	pfCfg := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut)).GetOrCreatePolicyForwarding().GetOrCreateInterface(interfaceID)
	pfCfg.ApplyVrfSelectionPolicy = ygot.String(clusterPolicy)
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

	pmd100GFRPorts := []string{}
	for _, p := range topo.Ports().Items() {
		port := ate.Port(t, p.Name())
		if port.PMD() == ondatra.PMD100GBASEFR {
			pmd100GFRPorts = append(pmd100GFRPorts, port.ID())
		}
	}
	// Disable FEC for 100G-FR ports because Novus does not support it.
	if len(pmd100GFRPorts) > 0 {
		l1Settings := topo.Layer1().Add().SetName("L1").SetPortNames(pmd100GFRPorts)
		l1Settings.SetAutoNegotiate(true).SetIeeeMediaDefaults(false).SetSpeed("speed_100_gbps")
		autoNegotiate := l1Settings.AutoNegotiation()
		autoNegotiate.SetRsFec(false)
	}

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
	pb, _ := topo.Marshal().ToProto()
	t.Log(pb.GetCaptures())
	otg.PushConfig(t, topo)
}

// clearCapture clears capture from all ports on the OTG
func clearCapture(t *testing.T, otg *otg.OTG, topo gosnappi.Config) {
	t.Log("Clearing capture")
	topo.Captures().Clear()
	otg.PushConfig(t, topo)
}

func randRange(max int, count int) []uint32 {
	rand.New(rand.NewSource(time.Now().UnixNano()))
	var result []uint32
	for len(result) < count {
		result = append(result, uint32(rand.Intn(max)))
	}
	return result
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
			innerV4.Priority().Dscp().Phb().SetValue(dscp)
		}

		// add inner ipv6 headers
		if flowType == "ipv6in4" {
			innerV6 := flow.Packet().Add().Ipv6()
			innerV6.Src().SetValue(InnerV6SrcIP)
			innerV6.Dst().SetValue(InnerV6DstIP)
			innerV6.TrafficClass().SetValue(dscp << 2)
		}
	} else if flowType == "ipv6" {
		v6 := flow.Packet().Add().Ipv6()
		v6.Src().SetValue(fa.src)
		v6.Dst().SetValue(fa.dst)
		v6.HopLimit().SetValue(ttl)
		v6.TrafficClass().SetValue(dscp << 2)
	}
	udp := flow.Packet().Add().Udp()
	udp.SrcPort().SetValues(randRange(50001, 10000))
	udp.DstPort().SetValues(randRange(50001, 10000))

	return flow
}

// sendTraffic starts traffic flows and send traffic for a fixed duration
func sendTraffic(t *testing.T, args *testArgs, flows []gosnappi.Flow, capture bool) {
	otg := args.ate.OTG()
	args.topo.Flows().Clear().Items()
	args.topo.Flows().Append(flows...)

	otg.PushConfig(t, args.topo)
	otg.StartProtocols(t)

	otgutils.WaitForARP(t, args.ate.OTG(), args.topo, "IPv4")
	otgutils.WaitForARP(t, args.ate.OTG(), args.topo, "IPv6")

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
				if !deviations.TTLCopyUnsupported(args.dut) {
					if got := uint32(v4Packet.TTL); got != pa.ttl {
						t.Errorf("TTL mismatch, got: %d, want: %d", got, pa.ttl)
						break
					}
				}
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
