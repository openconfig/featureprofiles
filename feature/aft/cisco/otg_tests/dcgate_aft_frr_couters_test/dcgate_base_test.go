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
package dcgate_aft_frr_couters_test

import (
	"context"
	"fmt"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/open-traffic-generator/snappi/gosnappi"
	aftUtil "github.com/openconfig/featureprofiles/feature/aft/cisco/aftUtils"
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
	"github.com/openconfig/ygot/ygot"
	"strconv"
	"testing"
	"time"
)

const (
	ipipProtocol                             = 4
	ipv6ipProtocol                           = 41
	ethertypeIPv4                            = oc.PacketMatchTypes_ETHERTYPE_ETHERTYPE_IPV4
	ethertypeIPv6                            = oc.PacketMatchTypes_ETHERTYPE_ETHERTYPE_IPV6
	clusterPolicy                            = "vrf_selection_policy_c"
	wanPolicy                                = "vrf_selection_policy_w"
	vrfDecap                                 = "DECAP_TE_VRF"
	vrfTransit                               = "TE_VRF_111"
	vrfRepaired                              = "TE_VRF_222"
	vrfEncapA                                = "ENCAP_TE_VRF_A"
	vrfEncapB                                = "ENCAP_TE_VRF_B"
	vrfDecapPostRepaired                     = "DECAP"
	ipv4PrefixLen                            = 30
	ipv6PrefixLen                            = 126
	trafficDuration                          = 15 * time.Second
	nhg10ID                                  = 10
	nh201ID                                  = 201
	nh202ID                                  = 202
	nhg1ID                                   = 1
	nh1ID                                    = 1
	dscpEncapA1                              = 10
	dscpEncapA2                              = 18
	dscpEncapB1                              = 20
	dscpEncapB2                              = 28
	dscpEncapNoMatch                         = 30
	magicMac                                 = "02:00:00:00:00:01"
	tunnelDstIP1                             = "203.0.113.1"
	tunnelDstIP2                             = "203.0.113.2"
	tunnelDstIP3                             = "203.0.113.100"
	ipv4OuterSrc111                          = "198.51.100.111"
	ipv4OuterSrc222                          = "198.51.100.222"
	ipv4OuterSrcIpInIp                       = "198.100.200.123"
	vipIP1                                   = "192.0.2.111"
	vipIP2                                   = "192.0.2.222"
	vipIP3                                   = "192.0.2.133"
	innerV4DstIP                             = "198.18.1.1"
	innerV4SrcIP                             = "198.18.0.255"
	InnerV6SrcIP                             = "2001:DB8::198:1"
	InnerV6DstIP                             = "2001:DB8:2:0:192::10"
	ipv4FlowIP                               = "138.0.11.8"
	ipv4EntryPrefix                          = "138.0.11.0"
	ipv4EntryPrefixLen                       = 24
	ipv6FlowIP                               = "2015:aa8::1"
	ipv6EntryPrefix                          = "2015:aa8::"
	ipv6EntryPrefixLen                       = 32
	ratioTunEncap1                           = 0.25 // 1/4
	ratioTunEncap2                           = 0.75 // 3/4
	ratioTunEncapTol                         = 0.05 // 5/100
	ttl                                      = uint32(100)
	trfDistTolerance                         = 0.02
	ipv4PrefixDoesNotExistInEncapVrf         = "140.0.0.1"
	ipv6PrefixDoesNotExistInEncapVrf         = "2016::140:0:0:1"
	sampleInterval                           = 5 * time.Second
	collectTime                              = 60 * time.Second
	aftCountertolerance              float64 = 1.0
)

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

	AFTValidationExact     = "exact"     // AFT counters must match TGEN traffic
	AFTValidationTransit   = "transit"   // AFT counters should not change
	AFTValidationIncrement = "increment" // AFT counters should increase but don't need to match
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

var (
	otgDstPorts = []string{"port2", "port3", "port4", "port5"}
	otgSrcPort  = "port1"
	//wantWeights = []float64{
	//	0.0625, // 1/4 * 1/4 - port2
	//	0.1875, // 1/4 * 3/4 - port3
	//	0.3,    // 3/4 * 2/5 - port4
	//	0.45,   // 3/5 * 3/4 - port5
	//}
	//noMatchWeight = []float64{
	//	1, 0, 0, 0,
	//}
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

type flowAttr struct {
	src      string   // source IP address
	dst      string   // destination IP address
	srcPort  string   // source OTG port
	dstPorts []string // destination OTG ports
	srcMac   string   // source MAC address
	dstMac   string   // destination MAC address
	// dscp     uint32
	topo gosnappi.Config
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
	//faIPinIP = flowAttr{
	//	src:      ipv4OuterSrcIpInIp,
	//	dst:      ipv4FlowIP,
	//	srcMac:   otgPort1.MAC,
	//	dstMac:   dutPort1.MAC,
	//	srcPort:  otgSrcPort,
	//	dstPorts: otgDstPorts,
	//	topo:     gosnappi.NewConfig(),
	//}
	//fa4NoPrefix = flowAttr{
	//	src:      otgPort1.IPv4,
	//	dst:      ipv4PrefixDoesNotExistInEncapVrf,
	//	srcMac:   otgPort1.MAC,
	//	dstMac:   dutPort1.MAC,
	//	srcPort:  otgSrcPort,
	//	dstPorts: otgDstPorts,
	//	topo:     gosnappi.NewConfig(),
	//}
	//fa6NoPrefix = flowAttr{
	//	src:      otgPort1.IPv6,
	//	dst:      ipv6PrefixDoesNotExistInEncapVrf,
	//	srcMac:   otgPort1.MAC,
	//	dstMac:   dutPort1.MAC,
	//	srcPort:  otgSrcPort,
	//	dstPorts: otgDstPorts,
	//	topo:     gosnappi.NewConfig(),
	//}
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
	dut               *ondatra.DUTDevice
	ate               *ondatra.ATEDevice
	topo              gosnappi.Config
	client            *gribi.Client
	aftValidationType string
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

// configureNetworkInstance creates nonDefaultVRFs
func configureNetworkInstance(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	c := &oc.Root{}
	vrfs := []string{vrfDecap, vrfTransit, vrfRepaired, vrfEncapA, vrfEncapB, vrfDecapPostRepaired}
	for _, vrf := range vrfs {
		ni := c.GetOrCreateNetworkInstance(vrf)
		ni.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF
		gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(vrf).Config(), ni)
	}

	fptest.ConfigureDefaultNetworkInstance(t, dut)
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
	otgutils.WaitForARP(t, ate.OTG(), topo, "IPv4")
	otgutils.WaitForARP(t, ate.OTG(), topo, "IPv6")
	return topo
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
func sendTraffic(t *testing.T, args *testArgs, flows []gosnappi.Flow, capture bool) (totalOutPkts float32,
	totalInPkts float32) {
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
	time.Sleep(5 * time.Second)
	t.Log("Traffic stopped")

	totalOutPkts, totalInPkts = gatherFlowStats(t, flows, args.ate.OTG())

	return totalOutPkts, totalInPkts
}

// validateTrafficFlows verifies that the flow on ATE should pass for good flow and fail for bad flow.
func validateTrafficFlows(t *testing.T, args *testArgs, flows []gosnappi.Flow, capture bool,
	match bool, aftValidationType string) {

	dut := ondatra.DUT(t, "dut")
	gnmiClient := dut.RawAPIs().GNMI(t)
	otg := args.ate.OTG()

	//1) Get pre-traffic counters
	preCounters, err := aftUtil.GetAftCountersSample(t, gnmiClient, sampleInterval, collectTime)
	if err != nil {
		t.Fatalf("Failed to get pre-counters via poll: %v", err)
	}

	packetsOut, packetsIn := sendTraffic(t, args, flows, capture)
	t.Logf("TGRN Packets Out %f Packets In %f", packetsOut, packetsIn)

	postCounters, err := aftUtil.GetAftCountersSample(t, gnmiClient, sampleInterval, collectTime)
	if err != nil {
		t.Fatalf("Failed to get post-counters via poll: %v", err)
	}

	flowDetails := aftUtil.GetOtgFlowDetails(t, flows, packetsOut)

	results := aftUtil.BuildAftPrefixChain(t, dut, preCounters, postCounters)
	aftUtil.AftCounterResults(t, flowDetails, results, aftValidationType, len(postCounters),
		aftCountertolerance, "Encap")

	otgutils.LogPortMetrics(t, otg, args.topo)
	otgutils.LogFlowMetrics(t, otg, args.topo)

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
	gnmi.Update(t, dut, gnmi.OC().Interface(p2.Name()).Config(), assignIPAsSecondary(&dutPort2DummyIP, p2.Name(), dut))
	gnmi.Update(t, dut, gnmi.OC().Interface(p3.Name()).Config(), assignIPAsSecondary(&dutPort3DummyIP, p3.Name(), dut))
	gnmi.Update(t, dut, gnmi.OC().Interface(p4.Name()).Config(), assignIPAsSecondary(&dutPort4DummyIP, p4.Name(), dut))
	gnmi.Update(t, dut, gnmi.OC().Interface(p5.Name()).Config(), assignIPAsSecondary(&dutPort5DummyIP, p5.Name(), dut))
	gnmi.Update(t, dut, gnmi.OC().Interface(p2.Name()).Config(), configStaticArp(p2.Name(), otgPort2DummyIP.IPv4, magicMac))
	gnmi.Update(t, dut, gnmi.OC().Interface(p3.Name()).Config(), configStaticArp(p3.Name(), otgPort3DummyIP.IPv4, magicMac))
	gnmi.Update(t, dut, gnmi.OC().Interface(p4.Name()).Config(), configStaticArp(p4.Name(), otgPort4DummyIP.IPv4, magicMac))
	gnmi.Update(t, dut, gnmi.OC().Interface(p5.Name()).Config(), configStaticArp(p5.Name(), otgPort5DummyIP.IPv4, magicMac))
}

// override ip address type as secondary
func assignIPAsSecondary(a *attrs.Attributes, port string, dut *ondatra.DUTDevice) *oc.Interface {
	intf := a.NewOCInterface(port, dut)
	s := intf.GetOrCreateSubinterface(0)
	s4 := s.GetOrCreateIpv4()
	a4 := s4.GetOrCreateAddress(a.IPv4)
	a4.Type = oc.IfIp_Ipv4AddressType_SECONDARY
	return intf
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

// CLI to configure falback vrf
func configFallBackVrf(t *testing.T, dut *ondatra.DUTDevice, vrf []string) {
	ctx := context.Background()
	for _, v := range vrf {
		fConf := fmt.Sprintf("vrf %v fallback-vrf default\n", v)
		config.TextWithGNMI(ctx, t, dut, fConf)
	}
}

func testTraffic(t *testing.T, args *testArgs, weights []float64, shouldPass bool) {
	flows := []gosnappi.Flow{fa4.getFlow("ipv4", "ip4a1", dscpEncapA1), fa6.getFlow("ipv6", "ip6a1", dscpEncapA1)}
	t.Log("Validate traffic flows")
	validateTrafficFlows(t, args, flows, false, shouldPass, args.aftValidationType)
	if shouldPass {
		t.Log("Validate hierarchical traffic distribution")
		validateTrafficDistribution(t, args.ate, weights)
	}
}
func testTransitTraffic(t *testing.T, args *testArgs, weights []float64, shouldPass bool, aftValidationType string) {
	flows := []gosnappi.Flow{faTransit.getFlow("ipv4in4", "ip4inipa1", dscpEncapA1), faTransit.getFlow("ipv6in4", "ip6inipa1", dscpEncapA1)}
	t.Log("Validate traffic flows")
	validateTrafficFlows(t, args, flows, false, shouldPass, aftValidationType)
	if shouldPass {
		t.Log("Validate hierarchical traffic distribution")
		validateTrafficDistribution(t, args.ate, weights)
	}
}

func testTransitTrafficWithDscp(t *testing.T, args *testArgs, weights []float64, dscp uint32, shouldPass bool) {
	flows := []gosnappi.Flow{faTransit.getFlow("ipv4in4", "ip4inipa1", dscp), faTransit.getFlow("ipv6in4", "ip6inipa1", dscp)}
	t.Log("Validate traffic flows")
	validateTrafficFlows(t, args, flows, false, shouldPass, args.aftValidationType)
	if shouldPass {
		t.Log("Validate hierarchical traffic distribution")
		validateTrafficDistribution(t, args.ate, weights)
	}
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func gatherFlowStats(
	t *testing.T,
	flows []gosnappi.Flow,
	otg *otg.OTG,
) (float32, float32) {

	var totalOutPkts, totalInPkts float32
	for _, flow := range flows {
		outPkts := float32(
			gnmi.Get(t, otg, gnmi.OTG().Flow(flow.Name()).Counters().OutPkts().State()),
		)
		inPkts := float32(
			gnmi.Get(t, otg, gnmi.OTG().Flow(flow.Name()).Counters().InPkts().State()),
		)

		// Use whatever flow-loss helper you have:
		lossPct := otgutils.GetFlowLossPct(t, otg, flow.Name(), 10*time.Second)
		if lossPct > 1 {
			t.Fatalf(
				"For flow %s inPkts=%.0f outPkts=%.0f loss=%.2f%% (wanted near 0%%)",
				flow.Name(), inPkts, outPkts, lossPct,
			)
		} else {
			t.Logf(
				"For flow %s inPkts=%.0f outPkts=%.0f loss=%.2f%%",
				flow.Name(), inPkts, outPkts, lossPct,
			)
		}

		totalOutPkts += outPkts
		totalInPkts += inPkts
	}

	return totalOutPkts, totalInPkts
}
