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

// Package setup is scoped only to be used for scripts in path
// feature/experimental/system/gnmi/benchmarking/ate_tests/
// Do not use elsewhere.
package dcgate_decap_aft_nh_counters

import (
	"context"
	"fmt"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	aftUtil "github.com/openconfig/featureprofiles/feature/aft/cisco/aftUtils"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/ygnmi/ygnmi"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/cisco/config"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

const (
	trafficDuration    = 15 * time.Second
	ipipProtocol       = 4
	ipv6ipProtocol     = 41
	nhUdpProtocol      = 17
	clusterPolicy      = "vrf_selection_policy_c"
	wanPolicy          = "vrf_selection_policy_w"
	dscpEncapA1        = 10
	dscpEncapA2        = 18
	dscpEncapB1        = 20
	dscpEncapB2        = 28
	dscpEncapNoMatch   = 30
	ipv4OuterSrc111    = "198.51.100.111"
	ipv4OuterSrc222    = "198.51.100.222"
	innerSrcIPv4Start  = "198.19.0.1"
	innerDstIPv4Start  = "138.0.11.1"
	innerSrcIPv6Start  = "2001:DB8::198:1"
	innerDstIPv6Start  = "2001:db8::138:0:11:1"
	flowCount          = 254
	vrfDecap           = "DECAP_TE_VRF"
	vrfTransit         = "TE_VRF_111"
	vrfRepaired        = "TE_VRF_222"
	encapIPv4FlowIP    = "138.0.11.8"
	encapVrfIPv4Prefix = "138.0.11.0/24"
	encapVrfIPv6Prefix = "2001:db8::138:0:11:0/126"
	vrfEncapA          = "ENCAP_TE_VRF_A"
	vrfEncapB          = "ENCAP_TE_VRF_B"
	vrfDefault         = "DEFAULT"
	ipv4PrefixLen      = 30
	ipv6PrefixLen      = 126
	ethertypeIPv4      = oc.PacketMatchTypes_ETHERTYPE_ETHERTYPE_IPV4
	ethertypeIPv6      = oc.PacketMatchTypes_ETHERTYPE_ETHERTYPE_IPV6
	trfDistTolerance   = 0.02
)

var (
	dutPort1 = attrs.Attributes{
		Name:    "port1",
		Desc:    "dutPort1",
		IPv4:    "192.0.2.1",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:1",
		IPv6Len: ipv6PrefixLen,
	}

	atePort1 = attrs.Attributes{
		Name:    "port1",
		Desc:    "atePort1",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.0.2.2",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:2",
		IPv6Len: ipv6PrefixLen,
	}

	dutPort2 = attrs.Attributes{
		Desc:    "dutPort2",
		Name:    "port2",
		IPv4:    "192.0.2.5",
		IPv4Len: ipv4PrefixLen,
	}

	atePort2 = attrs.Attributes{
		Name:    "port2",
		MAC:     "02:00:02:01:01:01",
		IPv4:    "192.0.2.6",
		IPv4Len: ipv4PrefixLen,
	}

	dutPort3 = attrs.Attributes{
		Desc:    "dutPort3",
		IPv4:    "192.0.2.9",
		IPv4Len: ipv4PrefixLen,
	}

	atePort3 = attrs.Attributes{
		Name:    "port3",
		MAC:     "02:00:03:01:01:01",
		IPv4:    "192.0.2.10",
		IPv4Len: ipv4PrefixLen,
	}

	dutPort4 = attrs.Attributes{
		Desc:    "dutPort4",
		IPv4:    "192.0.2.13",
		IPv4Len: ipv4PrefixLen,
	}

	atePort4 = attrs.Attributes{
		Name:    "port4",
		MAC:     "02:00:04:01:01:01",
		IPv4:    "192.0.2.14",
		IPv4Len: ipv4PrefixLen,
	}
	dutPort5 = attrs.Attributes{
		Desc:    "dutPort5",
		IPv4:    "192.0.2.17",
		IPv4Len: ipv4PrefixLen,
	}

	atePort5 = attrs.Attributes{
		Name:    "port5",
		MAC:     "02:00:04:01:01:01",
		IPv4:    "192.0.2.18",
		IPv4Len: ipv4PrefixLen,
	}
	dutPort6 = attrs.Attributes{
		Desc:    "dutPort6",
		IPv4:    "192.0.2.21",
		IPv4Len: ipv4PrefixLen,
	}

	atePort6 = attrs.Attributes{
		Name:    "port6",
		MAC:     "02:00:04:01:01:01",
		IPv4:    "192.0.2.22",
		IPv4Len: ipv4PrefixLen,
	}
	dutPort7 = attrs.Attributes{
		Desc:    "dutPort7",
		IPv4:    "192.0.2.25",
		IPv4Len: ipv4PrefixLen,
	}

	atePort7 = attrs.Attributes{
		Name:    "port7",
		MAC:     "02:00:04:01:01:01",
		IPv4:    "192.0.2.26",
		IPv4Len: ipv4PrefixLen,
	}
	dutPort8 = attrs.Attributes{
		Name:    "dutPort8",
		Desc:    "dutPort8",
		IPv4:    "192.0.2.29",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:5",
		IPv6Len: ipv6PrefixLen,
	}

	atePort8 = attrs.Attributes{
		Name:    "port8",
		MAC:     "02:00:08:01:01:01",
		IPv4:    "192.0.2.30",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:6",
		IPv6Len: ipv6PrefixLen,
	}
	AtePorts = map[string]attrs.Attributes{
		"port1": atePort1,
		"port2": atePort2,
		"port3": atePort3,
		"port4": atePort4,
		"port5": atePort5,
		"port6": atePort6,
		"port7": atePort7,
		"port8": atePort8,
	}
	DutPorts = map[string]attrs.Attributes{
		"port1": dutPort1,
		"port2": dutPort2,
		"port3": dutPort3,
		"port4": dutPort4,
		"port5": dutPort5,
		"port6": dutPort6,
		"port7": dutPort7,
		"port8": dutPort8,
	}
)

type PbrRule struct {
	sequence    uint32
	protocol    uint8
	src_addr    string
	dscpSet     []uint8
	dscpSetv6   []uint8
	decapVrfSet []string
	encapVrf    string
	etherType   oc.NetworkInstance_PolicyForwarding_Policy_Rule_L2_Ethertype_Union
}

type trafficflowAttr struct {
	innerSrcStart        string // Inner source IP address
	innerdstStart        string // Inner destination IP address
	innerFlowCount       int
	egressTrackingOffset uint32 // outer source IP address
	egressTrackingWidth  uint32 // outer destination IP address
}

// testArgs holds the objects needed by a test case.
type testArgs struct {
	dut               *ondatra.DUTDevice
	ate               *ondatra.ATEDevice
	topo              *ondatra.ATETopology
	ctx               context.Context
	gribiClient       *gribi.Client
	aftValidationType string
}

// WAN PBR rules
var pbrRules = []PbrRule{
	{
		sequence:    uint32(1),
		protocol:    ipipProtocol,
		dscpSet:     []uint8{dscpEncapA1, dscpEncapA2},
		decapVrfSet: []string{vrfDecap, vrfEncapA, vrfRepaired},
		src_addr:    ipv4OuterSrc222,
	},
	{
		sequence:    uint32(2),
		protocol:    ipv6ipProtocol,
		dscpSet:     []uint8{dscpEncapA1, dscpEncapA2},
		decapVrfSet: []string{vrfDecap, vrfEncapA, vrfRepaired},
		src_addr:    ipv4OuterSrc222,
	},
	{
		sequence:    uint32(3),
		protocol:    ipipProtocol,
		dscpSet:     []uint8{dscpEncapA1, dscpEncapA2},
		decapVrfSet: []string{vrfDecap, vrfEncapA, vrfTransit},
		src_addr:    ipv4OuterSrc111,
	},
	{
		sequence:    uint32(4),
		protocol:    ipv6ipProtocol,
		dscpSet:     []uint8{dscpEncapA1, dscpEncapA2},
		decapVrfSet: []string{vrfDecap, vrfEncapA, vrfTransit},
		src_addr:    ipv4OuterSrc111,
	},
	{
		sequence:    uint32(5),
		protocol:    ipipProtocol,
		dscpSet:     []uint8{dscpEncapB1, dscpEncapB2},
		decapVrfSet: []string{vrfDecap, vrfEncapB, vrfRepaired},
		src_addr:    ipv4OuterSrc222,
	},
	{
		sequence:    uint32(6),
		protocol:    ipv6ipProtocol,
		dscpSet:     []uint8{dscpEncapB1, dscpEncapB2},
		decapVrfSet: []string{vrfDecap, vrfEncapB, vrfRepaired},
		src_addr:    ipv4OuterSrc222,
	},
	{
		sequence:    uint32(7),
		protocol:    ipipProtocol,
		dscpSet:     []uint8{dscpEncapB1, dscpEncapB2},
		decapVrfSet: []string{vrfDecap, vrfEncapB, vrfTransit},
		src_addr:    ipv4OuterSrc111,
	},
	{
		sequence:    uint32(8),
		protocol:    ipv6ipProtocol,
		dscpSet:     []uint8{dscpEncapB1, dscpEncapB2},
		decapVrfSet: []string{vrfDecap, vrfEncapB, vrfTransit},
		src_addr:    ipv4OuterSrc111,
	},
	{
		sequence:    uint32(9),
		protocol:    ipipProtocol,
		decapVrfSet: []string{vrfDecap, vrfDefault, vrfRepaired},
		src_addr:    ipv4OuterSrc222,
	},
	{
		sequence:    uint32(910),
		protocol:    ipv6ipProtocol,
		decapVrfSet: []string{vrfDecap, vrfDefault, vrfRepaired},
		src_addr:    ipv4OuterSrc222,
	},
	{
		sequence:    uint32(911),
		protocol:    ipipProtocol,
		decapVrfSet: []string{vrfDecap, vrfDefault, vrfTransit},
		src_addr:    ipv4OuterSrc111,
	},
	{
		sequence:    uint32(912),
		protocol:    ipv6ipProtocol,
		decapVrfSet: []string{vrfDecap, vrfDefault, vrfTransit},
		src_addr:    ipv4OuterSrc111,
	},
	{
		sequence:  uint32(917),
		etherType: ethertypeIPv4,
		encapVrf:  vrfDefault,
	},
	{
		sequence:  uint32(918),
		etherType: ethertypeIPv6,
		encapVrf:  vrfDefault,
	},
}

// Additonal Cluster traffic PBR rules
var encapPbrRules = []PbrRule{
	{
		sequence: uint32(913),
		dscpSet:  []uint8{dscpEncapA1, dscpEncapA2},
		encapVrf: vrfEncapA,
	},
	{
		sequence:  uint32(914),
		dscpSetv6: []uint8{dscpEncapA1, dscpEncapA2},
		encapVrf:  vrfEncapA,
	},
	{
		sequence: uint32(915),
		dscpSet:  []uint8{dscpEncapB1, dscpEncapB2},
		encapVrf: vrfEncapB,
	},
	{
		sequence:  uint32(916),
		dscpSetv6: []uint8{dscpEncapB1, dscpEncapB2},
		encapVrf:  vrfEncapB,
	},
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

// configStaticRoute creates v4 & v6 static route in default VRF. Delete flag will delete the route
func configStaticRoute(t *testing.T, dut *ondatra.DUTDevice, v4Prefix, v4NextHop, v6Prefix, v6NextHop string, delete bool) {
	t.Logf("*** Configuring static route in DEFAULT network-instance ...")
	ni := oc.NetworkInstance{Name: ygot.String(deviations.DefaultNetworkInstance(dut))}
	static := ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
	if v4Prefix != "" {
		sr := static.GetOrCreateStatic(v4Prefix)
		nh := sr.GetOrCreateNextHop("0")
		nh.NextHop = oc.UnionString(v4NextHop)
		if delete {
			gnmi.Delete(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut)).Static(v4Prefix).Config())

		} else {
			gnmi.Update(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut)).Config(), static)
		}
	}
	if v6Prefix != "" {
		sr := static.GetOrCreateStatic(v6Prefix)
		nh := sr.GetOrCreateNextHop("0")
		nh.NextHop = oc.UnionString(v6NextHop)
		if delete {
			gnmi.Delete(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut)).Static(v6Prefix).Config())
		} else {
			gnmi.Update(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut)).Config(), static)
		}
	}
}

// createPbrPolicy returns Policy map defined in pbrRules struct for cluster & wan policy
func createPbrPolicy(dut *ondatra.DUTDevice, name string, cluster_facing bool) *oc.NetworkInstance_PolicyForwarding {
	d := &oc.Root{}
	ni := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	pf := ni.GetOrCreatePolicyForwarding()
	p := pf.GetOrCreatePolicy(name)
	p.SetType(oc.Policy_Type_VRF_SELECTION_POLICY)
	if cluster_facing {
		pbrRules = append(pbrRules, encapPbrRules...)
	}
	for _, pbrRule := range pbrRules {
		r, _ := p.NewRule(pbrRule.sequence)
		l2 := r.GetOrCreateL2()
		r4 := r.GetOrCreateIpv4()
		if pbrRule.dscpSet != nil {
			r4.DscpSet = pbrRule.dscpSet
		} else if pbrRule.dscpSetv6 != nil {
			r6 := r.GetOrCreateIpv6()
			r6.DscpSet = pbrRule.dscpSetv6
		}

		if pbrRule.protocol != 0 {
			r4.Protocol = oc.UnionUint8(pbrRule.protocol)
		}

		if pbrRule.src_addr != "" {
			r4.SourceAddress = ygot.String(pbrRule.src_addr + "/32")
		}

		if len(pbrRule.decapVrfSet) == 3 {
			ra := r.GetOrCreateAction()
			ra.DecapNetworkInstance = ygot.String(pbrRule.decapVrfSet[0])
			ra.PostDecapNetworkInstance = ygot.String(pbrRule.decapVrfSet[1])
			ra.DecapFallbackNetworkInstance = ygot.String(pbrRule.decapVrfSet[2])
		}

		if pbrRule.etherType != nil {
			l2.SetEthertype(pbrRule.etherType)
		}

		if pbrRule.encapVrf != "" {
			r.GetOrCreateAction().SetNetworkInstance(pbrRule.encapVrf)
		}
	}
	return pf
}

// applyForwardingPolicy applies the forwarding policy on the interface.
func applyForwardingPolicy(t *testing.T, ingressPort, policyName string, deletePolicy bool) {
	t.Logf("Applying forwarding policy on interface %v ... ", ingressPort)
	d := &oc.Root{}
	dut := ondatra.DUT(t, "dut")
	pfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding().Interface(ingressPort + ".0")
	pfCfg := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut)).GetOrCreatePolicyForwarding().GetOrCreateInterface(ingressPort + ".0")
	pfCfg.ApplyVrfSelectionPolicy = ygot.String(policyName)
	pfCfg.GetOrCreateInterfaceRef().Interface = ygot.String(ingressPort)
	pfCfg.GetOrCreateInterfaceRef().Subinterface = ygot.Uint32(0)
	if deletePolicy {
		gnmi.Delete(t, dut, pfPath.Config())
	} else {
		gnmi.Replace(t, dut, pfPath.Config(), pfCfg)
	}
}

// Program Base gRIBI entries for Encap, Transit TE, Repaired VRF
func baseGribiProgramming(t *testing.T, dut *ondatra.DUTDevice) {
	// Configure the gRIBI client
	gribiClient := gribi.Client{
		DUT:         dut,
		FIBACK:      true,
		Persistence: true,
	}
	if err := gribiClient.Start(t); err != nil {
		t.Fatalf("gRIBI Connection can not be established")
	}
	gribiClient.BecomeLeader(t)
	gribiClient.FlushAll(t)
	t.Log("Adding ENCAP VRF gRIBI entries")
	//Backup NHG with redirect/NH to default VRF
	gribiClient.AddNH(t, 2000, "VRFOnly", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHOptions{VrfName: deviations.DefaultNetworkInstance(dut)})
	gribiClient.AddNHG(t, 200, map[uint64]uint64{2000: 1}, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	//gRIBI entries for Encap VRF prefixes
	gribiClient.AddNH(t, 201, "Encap", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHOptions{Src: ipv4OuterSrc111, Dest: "203.0.113.1", VrfName: vrfTransit})
	gribiClient.AddNH(t, 202, "Encap", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHOptions{Src: ipv4OuterSrc111, Dest: "203.10.113.2", VrfName: vrfTransit})
	gribiClient.AddNHG(t, 10, map[uint64]uint64{201: 1, 202: 3}, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: 200})
	gribiClient.AddNHG(t, 11, map[uint64]uint64{201: 3, 202: 1}, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: 200})
	gribiClient.AddIPv4(t, encapVrfIPv4Prefix, 10, vrfEncapA, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	gribiClient.AddIPv4(t, encapVrfIPv4Prefix, 11, vrfEncapB, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)

	t.Log("Adding TRANSIT TE VRF gRIBI entries")
	//gRIBI entries for Encap NH Tunnel DA=203.0.113.1
	gribiClient.AddNH(t, 10, atePort2.IPv4, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	gribiClient.AddNH(t, 11, atePort3.IPv4, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	gribiClient.AddNHG(t, 2, map[uint64]uint64{10: 1, 11: 3}, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	gribiClient.AddNH(t, 100, atePort4.IPv4, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	gribiClient.AddNHG(t, 3, map[uint64]uint64{100: 1}, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	gribiClient.AddNH(t, 1, "192.0.2.101", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	gribiClient.AddNH(t, 2, "192.0.2.102", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	//Backup NHG for this Encap Tunnel
	gribiClient.AddNH(t, 1000, "DecapEncap", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHOptions{Src: ipv4OuterSrc222, Dest: "203.0.113.100", VrfName: vrfRepaired})
	gribiClient.AddNHG(t, 100, map[uint64]uint64{1000: 1}, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)

	gribiClient.AddNHG(t, 1, map[uint64]uint64{1: 1, 2: 3}, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: 100})
	gribiClient.AddIPv4(t, "203.0.113.1"+"/"+"32", 1, vrfTransit, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	gribiClient.AddIPv4(t, "192.0.2.101"+"/"+"32", 2, deviations.DefaultNetworkInstance(dut), deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	gribiClient.AddIPv4(t, "192.0.2.102"+"/"+"32", 3, deviations.DefaultNetworkInstance(dut), deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)

	//gRIBI entries for Encap NH Tunnel DA=203.10.113.2
	gribiClient.AddNH(t, 13, atePort6.IPv4, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	gribiClient.AddNH(t, 4, "192.0.2.104", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	gribiClient.AddNHG(t, 5, map[uint64]uint64{13: 1}, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	//Backup NHG for this Encap Tunnel
	gribiClient.AddNH(t, 1001, "DecapEncap", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHOptions{Src: ipv4OuterSrc222, Dest: "203.0.113.101", VrfName: vrfRepaired})
	gribiClient.AddNHG(t, 101, map[uint64]uint64{1001: 1}, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)

	gribiClient.AddNHG(t, 4, map[uint64]uint64{4: 1}, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: 101})
	gribiClient.AddIPv4(t, "192.0.2.104"+"/"+"32", 5, deviations.DefaultNetworkInstance(dut), deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	gribiClient.AddIPv4(t, "203.10.113.2"+"/"+"32", 4, vrfTransit, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)

	t.Log("Adding REPAIRED VRF gRIBI entries")
	//Decap Backup NHG
	gribiClient.AddNH(t, 2001, "Decap", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	gribiClient.AddNHG(t, 201, map[uint64]uint64{2001: 1}, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)

	//gRIBI entries for Backup NHG DA prefix 203.0.113.100 in Repaired VRF for Encap tunnel DA=203.0.113.1
	gribiClient.AddNH(t, 12, atePort5.IPv4, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	gribiClient.AddNH(t, 3, "192.0.2.103", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	gribiClient.AddNHG(t, 8, map[uint64]uint64{12: 1}, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	gribiClient.AddNHG(t, 7, map[uint64]uint64{3: 1}, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: 201})
	gribiClient.AddIPv4(t, "192.0.2.103"+"/"+"32", 8, deviations.DefaultNetworkInstance(dut), deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	gribiClient.AddIPv4(t, "203.0.113.100"+"/"+"32", 7, vrfRepaired, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)

	//gRIBI entries for Backup NHG DA prefix 203.0.113.101 in Repaired VRF for Encap tunnel DA=203.10.113.2
	gribiClient.AddNH(t, 14, atePort7.IPv4, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	gribiClient.AddNH(t, 5, "192.0.2.105", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	gribiClient.AddNHG(t, 155, map[uint64]uint64{14: 1}, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	gribiClient.AddNHG(t, 9, map[uint64]uint64{5: 1}, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: 201})
	gribiClient.AddIPv4(t, "192.0.2.105"+"/"+"32", 155, deviations.DefaultNetworkInstance(dut), deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	gribiClient.AddIPv4(t, "203.0.113.101"+"/"+"32", 9, vrfRepaired, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)

	gribiClient.Close(t)
}

// configureDUT configures port1, port2, port3, port4 on the DUT.
func configureDUTInterface(t *testing.T, dut *ondatra.DUTDevice) {
	d := gnmi.OC()
	for p, dp := range DutPorts {
		p1 := dut.Port(t, p)
		gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), dp.NewOCInterface(p1.Name(), dut))
	}

}

// CLI to configure Default static route with NH default VRF. NH VRF option is not available with OC Local routing
func configDefaultIPStaticCli(t *testing.T, dut *ondatra.DUTDevice, vrf []string) {
	ctx := context.Background()
	for _, v := range vrf {
		conf := fmt.Sprintf("router static vrf %v address-family ipv4 unicast 0.0.0.0/0 vrf default\n router static vrf %v address-family ipv6 unicast ::/0 vrf default", v, v)
		config.TextWithGNMI(ctx, t, dut, conf)
		time.Sleep(5 * time.Second)
	}
}

// configreATE configures IP addresses on port1-8 on the ATE.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) *ondatra.ATETopology {
	topo := ate.Topology().New()
	t.Logf("Configuring ATE Port1 to Port8")
	p1 := ate.Port(t, "port1")
	p2 := ate.Port(t, "port2")
	p3 := ate.Port(t, "port3")
	p4 := ate.Port(t, "port4")
	p5 := ate.Port(t, "port5")
	p6 := ate.Port(t, "port6")
	p7 := ate.Port(t, "port7")
	p8 := ate.Port(t, "port8")

	atePort1.AddToATE(topo, p1, &dutPort1)
	atePort2.AddToATE(topo, p2, &dutPort2)
	atePort3.AddToATE(topo, p3, &dutPort3)
	atePort4.AddToATE(topo, p4, &dutPort4)
	atePort5.AddToATE(topo, p5, &dutPort5)
	atePort6.AddToATE(topo, p6, &dutPort6)
	atePort7.AddToATE(topo, p7, &dutPort7)
	atePort8.AddToATE(topo, p8, &dutPort8)

	t.Logf("Pushing config to ATE and starting protocols...")
	topo.Push(t).StartProtocols(t)
	return topo
}

// Creates ATE Traffic Flow with parameters Flowname, Inner v4 or v6, Outer DA & SA, DSCP, Dest Ports.
// trafficflowAttr for setting the Inner IP DA/SA & Egress tracking offset & width
func (fa *trafficflowAttr) createTrafficFlow(t *testing.T, ate *ondatra.ATEDevice, flowName, innerProtocolType,
	outerIPDst, outerIPSrc string, dscp uint8, dstPorts []string) (*ondatra.Flow, aftUtil.FlowDetails) {
	topo := ate.Topology().New()
	flow := ate.Traffic().NewFlow(flowName)
	p1 := ate.Port(t, "port1")
	srcPort := topo.AddInterface(atePort1.Name).WithPort(p1)
	dstEndPoints := []ondatra.Endpoint{}
	for _, v := range dstPorts {
		p := ate.Port(t, v)
		d := topo.AddInterface(v).WithPort(p)
		dstEndPoints = append(dstEndPoints, d)
	}
	ethHeader := ondatra.NewEthernetHeader()
	ethHeader.WithSrcAddress(atePort1.MAC)
	outerv4Header := ondatra.NewIPv4Header().WithSrcAddress(outerIPSrc).WithDstAddress(outerIPDst).WithDSCP(dscp).WithTTL(100)
	udpHeader := ondatra.NewUDPHeader()
	udpHeader.DstPortRange().WithMin(50000).WithStep(1).WithCount(1000)
	udpHeader.SrcPortRange().WithMin(50000).WithStep(1).WithCount(1000)

	if innerProtocolType == "IPv4" {
		innerV4Header := ondatra.NewIPv4Header()
		innerV4Header.SrcAddressRange().WithMin(fa.innerSrcStart).WithCount(uint32(fa.innerFlowCount)).WithStep("0.0.0.1")
		innerV4Header.DstAddressRange().WithMin(fa.innerdstStart).WithCount(uint32(fa.innerFlowCount)).WithStep("0.0.0.1")
		innerV4Header.WithDSCP(dscp)
		v4UdpHeader := ondatra.NewUDPHeader()
		v4UdpHeader.DstPortRange().WithMin(50000).WithStep(1).WithCount(1000)
		v4UdpHeader.SrcPortRange().WithMin(50000).WithStep(1).WithCount(1000)
		flow.WithSrcEndpoints(srcPort).WithHeaders(ethHeader, outerv4Header, innerV4Header, v4UdpHeader).WithDstEndpoints(dstEndPoints...).WithFrameRateFPS(10000)
	} else if innerProtocolType == "IPv6" {
		innerV6Header := ondatra.NewIPv6Header()
		innerV6Header.SrcAddressRange().WithMin(fa.innerSrcStart).WithCount(uint32(fa.innerFlowCount)).WithStep("::1")
		innerV6Header.DstAddressRange().WithMin(fa.innerdstStart).WithCount(uint32(fa.innerFlowCount)).WithStep("::1")
		innerV6Header.WithDSCP(dscp)
		v6UdpHeader := ondatra.NewUDPHeader()
		v6UdpHeader.DstPortRange().WithMin(50000).WithStep(1).WithCount(1000)
		v6UdpHeader.SrcPortRange().WithMin(50000).WithStep(1).WithCount(1000)
		flow.WithSrcEndpoints(srcPort).WithHeaders(ethHeader, outerv4Header, innerV6Header, v6UdpHeader).WithDstEndpoints(dstEndPoints...).WithFrameRateFPS(10000)
	} else {
		flow.WithSrcEndpoints(srcPort).WithHeaders(ethHeader, outerv4Header, udpHeader).WithDstEndpoints(dstEndPoints...).WithFrameRateFPS(10000)
	}
	flow.EgressTracking().WithOffset(fa.egressTrackingOffset).WithWidth(fa.egressTrackingWidth)

	details := aftUtil.FlowDetails{
		Protocol:    innerProtocolType,
		OuterSrc:    outerIPSrc,
		OuterDst:    outerIPDst,
		DSCP:        dscp,
		DestPorts:   dstPorts,
		PacketCount: 0, // Will be updated after traffic runs
	}

	return flow, details
}

func (args *testArgs) validateAftTelemetry(t *testing.T, vrfName, prefix string, nhEntryGot int) *aftUtil.PrefixStatsMapping {
	aftPfxPath := gnmi.OC().NetworkInstance(vrfName).Afts().Ipv4Entry(prefix)
	aftPfxVal, found := gnmi.Watch(t, args.dut, aftPfxPath.State(), 2*time.Minute, func(val *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv4Entry]) bool {
		value, present := val.Val()
		return present && value.GetNextHopGroup() != 0
	}).Await(t)

	if !found {
		t.Fatalf("Could not find prefix %s in telemetry AFT", prefix)
	}

	aftPfx, _ := aftPfxVal.Val()
	aftNHG := gnmi.Get(t, args.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Afts().NextHopGroup(aftPfx.GetNextHopGroup()).State())

	if got := len(aftNHG.NextHop); got != nhEntryGot {
		t.Fatalf("Prefix %s next-hop entry count: got %d, want %d", prefix, got, nhEntryGot)
	}

	// Create stats mapping for this prefix
	statsID := fmt.Sprintf("stsaftnh,%d", aftPfx.GetNextHopGroup())
	cli := args.dut.RawAPIs().CLI(t)

	redisCmd := fmt.Sprintf("redis-cli KEYS \"aftv4route,%s,*\"", vrfName)
	output, err := cli.RunCommand(context.Background(), redisCmd)
	if err != nil {
		t.Logf("Error getting Redis keys: %v", err)
		return &aftUtil.PrefixStatsMapping{
			StatsID:     statsID,
			Prefixes:    []string{prefix},
			PrefixCount: 1,
			FlowInfo:    make(map[string]uint64), // Changed to uint64
		}
	}

	sharingPrefixes := []string{prefix}
	lines := strings.Split(output.Output(), "\n")

	for _, line := range lines {
		if line == "" {
			continue
		}
		prefixStatsCmd := fmt.Sprintf("redis-cli GET \"%s\"", line)
		statsOutput, err := cli.RunCommand(context.Background(), prefixStatsCmd)
		if err != nil {
			continue
		}

		if strings.Contains(statsOutput.Output(), statsID) {
			parts := strings.Split(line, ",")
			if len(parts) >= 3 {
				foundPrefix := parts[2]
				if foundPrefix != prefix {
					sharingPrefixes = append(sharingPrefixes, foundPrefix)
				}
			}
		}
	}

	t.Logf("Found %d prefixes sharing stats object %s: %v", len(sharingPrefixes), statsID, sharingPrefixes)

	return &aftUtil.PrefixStatsMapping{
		StatsID:     statsID,
		Prefixes:    sharingPrefixes,
		PrefixCount: len(sharingPrefixes),
		FlowInfo:    make(map[string]uint64), // Changed to uint64
	}
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
func validateTrafficDistribution(t *testing.T, ate *ondatra.ATEDevice, wantWeights []float64, dstPorts []string) {
	dstPortInPktList := []uint64{}
	for _, dstPort := range dstPorts {
		ateP := ate.Port(t, dstPort)
		dstPortInPktList = append(dstPortInPktList, gnmi.Get(t, ate, gnmi.OC().Interface(ateP.Name()).Counters().InPkts().State()))
	}
	gotWeights, _ := normalize(dstPortInPktList)

	t.Log("got ratio:", gotWeights)
	t.Log("want ratio:", wantWeights)
	if diff := cmp.Diff(wantWeights, gotWeights, cmpopts.EquateApprox(0, trfDistTolerance)); diff != "" {
		t.Errorf("Packet distribution ratios -want,+got:\n%s", diff)
	}
}

func sendTraffic(t *testing.T, ate *ondatra.ATEDevice, allFlows []*ondatra.Flow,
	flowDetails map[string]aftUtil.FlowDetails, aftValidationType string, statsMappings []*aftUtil.PrefixStatsMapping) {

	dut := ondatra.DUT(t, "dut")
	gnmiClient := dut.RawAPIs().GNMI(t)
	baselineCounters, numAftPathObj := aftUtil.GetPacketForwardedCounts(t, gnmiClient)

	t.Logf("*** Starting traffic ...")
	ate.Traffic().Start(t, allFlows...)
	time.Sleep(trafficDuration)
	t.Logf("*** Stop traffic ...")
	ate.Traffic().Stop(t)

	// Allow time for counters to settle
	time.Sleep(35 * time.Second)

	updatedCounters, _ := aftUtil.GetPacketForwardedCounts(t, gnmiClient)

	var baselinePacketsForwarded uint64
	var updatedPacketsForwarded uint64

	for _, count := range baselineCounters {
		baselinePacketsForwarded += count
	}
	for _, count := range updatedCounters {
		updatedPacketsForwarded += count
	}

	t.Logf("Baseline packets forwarded: %d", baselinePacketsForwarded)
	t.Logf("Updated packets forwarded: %d", updatedPacketsForwarded)

	counterDiffs := aftUtil.GetTrafficCounterDiff([]uint64{baselinePacketsForwarded},
		[]uint64{updatedPacketsForwarded})

	var totalOutPkts uint64
	for _, flow := range allFlows {
		flowPath := gnmi.OC().Flow(flow.Name())
		outPkts := gnmi.Get(t, ate, flowPath.Counters().OutPkts().State())
		totalOutPkts += outPkts
		if details, ok := flowDetails[flow.Name()]; ok {
			details.PacketCount = outPkts
			flowDetails[flow.Name()] = details
		}
	}

	// Update flow information in stats mappings
	for _, mapping := range statsMappings {
		if mapping == nil {
			continue
		}
		if mapping.FlowInfo == nil {
			mapping.FlowInfo = make(map[string]uint64)
		}
		for flowName, details := range flowDetails {
			mapping.FlowInfo[flowName] = details.PacketCount // Store just the packet count
		}
	}

	aftUtil.BuildAftAteStatsTable(t, ate, allFlows, flowDetails, totalOutPkts,
		baselinePacketsForwarded, updatedPacketsForwarded, counterDiffs[0],
		1.0, aftValidationType, numAftPathObj, statsMappings)
}

// configureDUT configures port1 and port8 on the DUT
func configureBaseconfig(t *testing.T, dut *ondatra.DUTDevice) {
	t.Log("Configure VRFs")
	configureNetworkInstance(t, dut)
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	t.Log("Configure Default route in Encap VRFs with NH as Default VRF")
	configDefaultIPStaticCli(t, dut, []string{vrfEncapA, vrfEncapB})
	t.Log("Configure DUT Interface")
	configureDUTInterface(t, dut)
	t.Log("Configure WAN facing VRF selection Policy")
	wanPBR := createPbrPolicy(dut, wanPolicy, false)
	defaultNiPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut))
	gnmi.Replace(t, dut, defaultNiPath.PolicyForwarding().Config(), wanPBR)
	t.Log("Configure Cluster facing VRF selection Policy")
	clusterPBR := createPbrPolicy(dut, clusterPolicy, true)
	gnmi.Update(t, dut, defaultNiPath.PolicyForwarding().Config(), clusterPBR)
}
