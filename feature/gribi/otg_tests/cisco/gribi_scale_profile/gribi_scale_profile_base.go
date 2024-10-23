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
// feature/experimental/system/gnmi/benchmarking/otg_tests/
// Do not use elsewhere.
package gribi_scale_profile

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/ondatra/netutil"
	// "github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	util "github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
	"golang.org/x/exp/maps"
)

const (
	trafficDuration          = 15 * time.Second
	ipipProtocol             = 4
	ipv6ipProtocol           = 41
	nhUdpProtocol            = 17
	clusterPolicy            = "vrf_selection_policy_c"
	wanPolicy                = "vrf_selection_policy_w"
	dscpEncapA1              = 10
	dscpEncapA2              = 18
	dscpEncapB1              = 20
	dscpEncapB2              = 28
	dscpEncapNoMatch         = 30
	peerGrpName              = "BGP-PEER-GROUP"
	policyName               = "ALLOW"
	dutBGPRID                = "18.18.18.18"
	peerBGPRID               = "8.8.8.8"
	ateRID                   = "192.0.2.31"
	ipv4OuterSrc111          = "198.51.100.111"
	ipv4OuterSrc222          = "198.51.100.222"
	innerSrcIPv4Start        = "198.19.0.1"
	innerDstIPv4Start        = "138.0.11.1"
	innerSrcIPv6Start        = "2001:DB8::198:1"
	innerDstIPv6Start        = "2001:db8::138:0:11:1"
	v4BGPDefault             = "203.0.113.0"
	v6BGPDefault             = "2001:DB8:2::1"
	dutPeerBundleIPRange     = "88.1.1.0/24"
	flowCount                = 254
	vrfDecap                 = "DECAP"
	vrfTransit               = "TRANSIT_VRF"
	vrfRepaired              = "REPAIRED"
	vrfRepair                = "REPAIR"
	encapIPv4FlowIP          = "138.0.11.8"
	encapVrfIPv4Prefix       = "138.0.11.0/24"
	encapVrfIPv6Prefix       = "2001:db8::138:0:11:0/126"
	vrfEncapA                = "VRF-LowPriority"
	vrfEncapB                = "VRF-HighPriority"
	vrfEncapC                = "VRF-LowLatency"
	vrfDefault               = "DEFAULT"
	ipv4PrefixLen            = 30
	ipv6PrefixLen            = 126
	ethertypeIPv4            = oc.PacketMatchTypes_ETHERTYPE_ETHERTYPE_IPV4
	ethertypeIPv6            = oc.PacketMatchTypes_ETHERTYPE_ETHERTYPE_IPV6
	trfDistTolerance         = 0.02
	staticDstMAC             = "001a.1117.5f80" //Static ARP/Local Static MAC address
	lagName                  = "LAGRx"          // LAG name for OTG
	tgenBundleID1            = "100"            // Bundle ID1 for OTG
	tgenBundleID2            = "200"            // Bundle ID1 for OTG
	advertisedRoutesv4Prefix = 32
	advertisedRoutesv6Prefix = 128
	dutAS                    = 68888
	ateAS                    = 67777
)

var (
	dutSrc1 = attrs.Attributes{
		Desc:    "dutSrc1",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::1",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}

	otgSrc1 = attrs.Attributes{
		Name:    "otgSrc1",
		MAC:     "02:11:01:00:00:01",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::2",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}

	dutSrc2 = attrs.Attributes{
		Desc:    "dutSrc2",
		IPv4:    "192.0.2.5",
		IPv6:    "2001:db8::5",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}

	otgSrc2 = attrs.Attributes{
		Name:    "otgSrc2",
		MAC:     "02:12:01:00:00:01",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::6",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}

	peerDst = attrs.Attributes{
		Desc:    "peerDst",
		IPv4:    "192.0.2.9",
		IPv6:    "2001:db8::8",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}

	otgDst = attrs.Attributes{
		Name:    "otgDst",
		MAC:     "02:13:01:00:00:01",
		IPv4:    "192.0.2.10",
		IPv6:    "2001:db8::b",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
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
	dut    *ondatra.DUTDevice
	otg    *ondatra.ATEDevice
	top    gosnappi.Config
	ctx    context.Context
	client *gribi.Client
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

// configureNetworkInstance Creates nonDefaultVRFs
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

// configStaticRoute Creates v4 & v6 static route in default VRF. Delete flag will delete the route
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

// CreatePbrPolicy returns Policy map defined in pbrRules struct for cluster & wan policy
func CreatePbrPolicy(dut *ondatra.DUTDevice, name string, cluster_facing bool) *oc.NetworkInstance_PolicyForwarding {
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
// func baseGribiProgramming(t *testing.T, dut *ondatra.DUTDevice) {
// 	// Configure the gRIBI client
// 	gribiClient := gribi.Client{
// 		DUT:         dut,
// 		FIBACK:      true,
// 		Persistence: true,
// 	}
// 	if err := gribiClient.Start(t); err != nil {
// 		t.Fatalf("gRIBI Connection can not be established")
// 	}
// 	gribiClient.BecomeLeader(t)
// 	gribiClient.FlushAll(t)
// 	t.Log("Adding ENCAP VRF gRIBI entries")
// 	//Backup NHG with redirect/NH to default VRF
// 	gribiClient.AddNH(t, 2000, "VRFOnly", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHOptions{VrfName: deviations.DefaultNetworkInstance(dut)})
// 	gribiClient.AddNHG(t, 200, map[uint64]uint64{2000: 1}, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
// 	//gRIBI entries for Encap VRF prefixes
// 	gribiClient.AddNH(t, 201, "Encap", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHOptions{Src: ipv4OuterSrc111, Dest: "203.0.113.1", VrfName: vrfTransit})
// 	gribiClient.AddNH(t, 202, "Encap", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHOptions{Src: ipv4OuterSrc111, Dest: "203.10.113.2", VrfName: vrfTransit})
// 	gribiClient.AddNHG(t, 10, map[uint64]uint64{201: 1, 202: 3}, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: 200})
// 	gribiClient.AddNHG(t, 11, map[uint64]uint64{201: 3, 202: 1}, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: 200})
// 	gribiClient.AddIPv4(t, encapVrfIPv4Prefix, 10, vrfEncapA, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
// 	gribiClient.AddIPv4(t, encapVrfIPv4Prefix, 11, vrfEncapB, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)

// 	t.Log("Adding TRANSIT TE VRF gRIBI entries")
// 	//gRIBI entries for Encap NH Tunnel DA=203.0.113.1
// 	gribiClient.AddNH(t, 10, otgPort2.IPv4, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
// 	gribiClient.AddNH(t, 11, otgPort3.IPv4, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
// 	gribiClient.AddNHG(t, 2, map[uint64]uint64{10: 1, 11: 3}, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
// 	gribiClient.AddNH(t, 100, otgPort4.IPv4, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
// 	gribiClient.AddNHG(t, 3, map[uint64]uint64{100: 1}, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
// 	gribiClient.AddNH(t, 1, "192.0.2.101", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
// 	gribiClient.AddNH(t, 2, "192.0.2.102", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
// 	//Backup NHG for this Encap Tunnel
// 	gribiClient.AddNH(t, 1000, "DecapEncap", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHOptions{Src: ipv4OuterSrc222, Dest: "203.0.113.100", VrfName: vrfRepaired})
// 	gribiClient.AddNHG(t, 100, map[uint64]uint64{1000: 1}, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)

// 	gribiClient.AddNHG(t, 1, map[uint64]uint64{1: 1, 2: 3}, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: 100})
// 	gribiClient.AddIPv4(t, "203.0.113.1"+"/"+"32", 1, vrfTransit, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
// 	gribiClient.AddIPv4(t, "192.0.2.101"+"/"+"32", 2, deviations.DefaultNetworkInstance(dut), deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
// 	gribiClient.AddIPv4(t, "192.0.2.102"+"/"+"32", 3, deviations.DefaultNetworkInstance(dut), deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)

// 	//gRIBI entries for Encap NH Tunnel DA=203.10.113.2
// 	gribiClient.AddNH(t, 13, otgPort6.IPv4, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
// 	gribiClient.AddNH(t, 4, "192.0.2.104", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
// 	gribiClient.AddNHG(t, 5, map[uint64]uint64{13: 1}, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
// 	//Backup NHG for this Encap Tunnel
// 	gribiClient.AddNH(t, 1001, "DecapEncap", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHOptions{Src: ipv4OuterSrc222, Dest: "203.0.113.101", VrfName: vrfRepaired})
// 	gribiClient.AddNHG(t, 101, map[uint64]uint64{1001: 1}, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)

// 	gribiClient.AddNHG(t, 4, map[uint64]uint64{4: 1}, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: 101})
// 	gribiClient.AddIPv4(t, "192.0.2.104"+"/"+"32", 5, deviations.DefaultNetworkInstance(dut), deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
// 	gribiClient.AddIPv4(t, "203.10.113.2"+"/"+"32", 4, vrfTransit, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)

// 	t.Log("Adding REPAIRED VRF gRIBI entries")
// 	//Decap Backup NHG
// 	gribiClient.AddNH(t, 2001, "Decap", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
// 	gribiClient.AddNHG(t, 201, map[uint64]uint64{2001: 1}, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)

// 	//gRIBI entries for Backup NHG DA prefix 203.0.113.100 in Repaired VRF for Encap tunnel DA=203.0.113.1
// 	gribiClient.AddNH(t, 12, otgPort5.IPv4, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
// 	gribiClient.AddNH(t, 3, "192.0.2.103", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
// 	gribiClient.AddNHG(t, 8, map[uint64]uint64{12: 1}, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
// 	gribiClient.AddNHG(t, 7, map[uint64]uint64{3: 1}, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: 201})
// 	gribiClient.AddIPv4(t, "192.0.2.103"+"/"+"32", 8, deviations.DefaultNetworkInstance(dut), deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
// 	gribiClient.AddIPv4(t, "203.0.113.100"+"/"+"32", 7, vrfRepaired, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)

// 	//gRIBI entries for Backup NHG DA prefix 203.0.113.101 in Repaired VRF for Encap tunnel DA=203.10.113.2
// 	gribiClient.AddNH(t, 14, otgPort7.IPv4, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
// 	gribiClient.AddNH(t, 5, "192.0.2.105", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
// 	gribiClient.AddNHG(t, 155, map[uint64]uint64{14: 1}, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
// 	gribiClient.AddNHG(t, 9, map[uint64]uint64{5: 1}, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: 201})
// 	gribiClient.AddIPv4(t, "192.0.2.105"+"/"+"32", 155, deviations.DefaultNetworkInstance(dut), deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
// 	gribiClient.AddIPv4(t, "203.0.113.101"+"/"+"32", 9, vrfRepaired, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)

// 	gribiClient.Close(t)
// }

// configureDUTBundle configures DUT side bundle for DUT-TGEN.
func configureDUTBundle(t *testing.T, dut *ondatra.DUTDevice, dutIntfAttr attrs.Attributes, aggPorts []*ondatra.Port, aggID string) {
	t.Helper()
	agg := dutIntfAttr.NewOCInterface(aggID, dut)
	agg.Type = oc.IETFInterfaces_InterfaceType_ieee8023adLag
	agg.GetOrCreateAggregation().LagType = oc.IfAggregate_AggregationType_LACP
	gnmi.Replace(t, dut, gnmi.OC().Interface(aggID).Config(), agg)

	for _, port := range aggPorts {
		d := &oc.Root{}
		i := d.GetOrCreateInterface(port.Name())
		i.GetOrCreateEthernet().AggregateId = ygot.String(aggID)
		i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd

		if deviations.InterfaceEnabled(dut) {
			i.Enabled = ygot.Bool(true)
		}
		gnmi.Replace(t, dut, gnmi.OC().Interface(port.Name()).Config(), i)
	}
}

// configureDUTBundle configures OTG side bundle for DUT-TGEN.
func configureOTGBundle(t *testing.T, ate *ondatra.ATEDevice, otgIntfAttr attrs.Attributes, top gosnappi.Config, aggPorts []*ondatra.Port, aggID string) {
	t.Helper()
	agg := top.Lags().Add().SetName(lagName)
	lagID, _ := strconv.Atoi(aggID)
	agg.Protocol().Static().SetLagId(uint32(lagID))
	for i, p := range aggPorts {
		port := top.Ports().Add().SetName(p.ID())
		newMac, err := incrementMAC(otgIntfAttr.MAC, i+1)
		if err != nil {
			t.Fatal(err)
		}
		agg.Ports().Add().SetPortName(port.Name()).Ethernet().SetMac(newMac).SetName("LAGRx-" + strconv.Itoa(i))
	}

	dstDev := top.Devices().Add().SetName(agg.Name() + ".dev")
	dstEth := dstDev.Ethernets().Add().SetName(lagName + ".Eth").SetMac(otgIntfAttr.MAC)
	dstEth.Connection().SetLagName(agg.Name())
	dstEth.Ipv4Addresses().Add().SetName(lagName + ".IPv4").SetAddress(otgIntfAttr.IPv4).SetGateway(otgIntfAttr.IPv4).SetPrefix(uint32(otgIntfAttr.IPv4Len))
}

func configureDUTInterfaces(t *testing.T, dut *ondatra.DUTDevice) {

	t.Log("Configuring DUT-TGEN Bundle interface1")
	aggID1 := netutil.NextAggregateInterface(t, dut)
	dutBundle1Member := []*ondatra.Port{
		dut.Port(t, "port1"),
		dut.Port(t, "port2"),
		dut.Port(t, "port3"),
		dut.Port(t, "port4"),
		dut.Port(t, "port5"),
		dut.Port(t, "port6"),
		dut.Port(t, "port7"),
		dut.Port(t, "port8"),
	}
	configureDUTBundle(t, dut, dutSrc1, dutBundle1Member, aggID1)

	t.Log("Configuring DUT-TGEN Bundle interface2")
	aggID2 := netutil.NextAggregateInterface(t, dut)
	dutBundle2Member := []*ondatra.Port{
		dut.Port(t, "port9"),
		dut.Port(t, "port10"),
		dut.Port(t, "port11"),
		dut.Port(t, "port12"),
		dut.Port(t, "port13"),
		dut.Port(t, "port14"),
		dut.Port(t, "port15"),
	}
	configureDUTBundle(t, dut, dutSrc2, dutBundle2Member, aggID2)

}

// incrementMAC increments the MAC by i. Returns error if the mac cannot be parsed or overflows the mac address space
func incrementMAC(mac string, i int) (string, error) {
	macAddr, err := net.ParseMAC(mac)
	if err != nil {
		return "", err
	}
	convMac := binary.BigEndian.Uint64(append([]byte{0, 0}, macAddr...))
	convMac = convMac + uint64(i)
	buf := new(bytes.Buffer)
	err = binary.Write(buf, binary.BigEndian, convMac)
	if err != nil {
		return "", err
	}
	newMac := net.HardwareAddr(buf.Bytes()[2:8])
	return newMac.String(), nil
}

// Creates otg Traffic Flow with parameters Flowname, Inner v4 or v6, Outer DA & SA, DSCP, Dest Ports.
// trafficflowAttr for setting the Inner IP DA/SA & Egress tracking offset & width
// func (fa *trafficflowAttr) CreateTrafficFlow(t *testing.T, otg *ondatra.ATEDevice, flowName, innerProtocolType, outerIPDst, outerIPSrc string, dscp uint8, dstPorts []string) *ondatra.Flow {
// 	topo := otg.Topology().New()
// 	flow := otg.Traffic().NewFlow(flowName)
// 	p1 := otg.Port(t, "port1")
// 	srcPort := topo.AddInterface(otgPort1.Name).WithPort(p1)
// 	dstEndPoints := []ondatra.Endpoint{}
// 	for _, v := range dstPorts {
// 		p := otg.Port(t, v)
// 		d := topo.AddInterface(v).WithPort(p)
// 		dstEndPoints = append(dstEndPoints, d)
// 	}
// 	ethHeader := ondatra.NewEthernetHeader()
// 	ethHeader.WithSrcAddress(otgPort1.MAC)
// 	outerv4Header := ondatra.NewIPv4Header().WithSrcAddress(outerIPSrc).WithDstAddress(outerIPDst).WithDSCP(dscp).WithTTL(100)
// 	udpHeader := ondatra.NewUDPHeader()
// 	udpHeader.DstPortRange().WithMin(50000).WithStep(1).WithCount(1000)
// 	udpHeader.SrcPortRange().WithMin(50000).WithStep(1).WithCount(1000)
// 	if innerProtocolType == "IPv4" {
// 		innerV4Header := ondatra.NewIPv4Header()
// 		innerV4Header.SrcAddressRange().WithMin(fa.innerSrcStart).WithCount(uint32(fa.innerFlowCount)).WithStep("0.0.0.1")
// 		innerV4Header.DstAddressRange().WithMin(fa.innerdstStart).WithCount(uint32(fa.innerFlowCount)).WithStep("0.0.0.1")
// 		innerV4Header.WithDSCP(dscp)
// 		v4UdpHeader := ondatra.NewUDPHeader()
// 		v4UdpHeader.DstPortRange().WithMin(50000).WithStep(1).WithCount(1000)
// 		v4UdpHeader.SrcPortRange().WithMin(50000).WithStep(1).WithCount(1000)
// 		flow.WithSrcEndpoints(srcPort).WithHeaders(ethHeader, outerv4Header, innerV4Header, v4UdpHeader).WithDstEndpoints(dstEndPoints...).WithFrameRotgFPS(10000)
// 	} else if innerProtocolType == "IPv6" {
// 		innerV6Header := ondatra.NewIPv6Header()
// 		innerV6Header.SrcAddressRange().WithMin(fa.innerSrcStart).WithCount(uint32(fa.innerFlowCount)).WithStep("::1")
// 		innerV6Header.DstAddressRange().WithMin(fa.innerdstStart).WithCount(uint32(fa.innerFlowCount)).WithStep("::1")
// 		innerV6Header.WithDSCP(dscp)
// 		v6UdpHeader := ondatra.NewUDPHeader()
// 		v6UdpHeader.DstPortRange().WithMin(50000).WithStep(1).WithCount(1000)
// 		v6UdpHeader.SrcPortRange().WithMin(50000).WithStep(1).WithCount(1000)
// 		flow.WithSrcEndpoints(srcPort).WithHeaders(ethHeader, outerv4Header, innerV6Header, v6UdpHeader).WithDstEndpoints(dstEndPoints...).WithFrameRotgFPS(10000)
// 	} else {
// 		flow.WithSrcEndpoints(srcPort).WithHeaders(ethHeader, outerv4Header, udpHeader).WithDstEndpoints(dstEndPoints...).WithFrameRotgFPS(10000)
// 	}
// 	flow.EgressTracking().WithOffset(fa.egressTrackingOffset).WithWidth(fa.egressTrackingWidth)
// 	return flow
// }

// validateAftTelmetry verifies aft telemetry entries.
func (a *testArgs) validateAftTelemetry(t *testing.T, vrfName, prefix string, nhEntryGot int) {
	aftPfxPath := gnmi.OC().NetworkInstance(vrfName).Afts().Ipv4Entry(prefix)
	aftPfxVal, found := gnmi.Watch(t, a.dut, aftPfxPath.State(), 2*time.Minute, func(val *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv4Entry]) bool {
		value, present := val.Val()
		return present && value.GetNextHopGroup() != 0
	}).Await(t)
	if !found {
		t.Fatalf("Could not find prefix %s in telemetry AFT", prefix)
	}
	aftPfx, _ := aftPfxVal.Val()

	aftNHG := gnmi.Get(t, a.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(a.dut)).Afts().NextHopGroup(aftPfx.GetNextHopGroup()).State())
	if got := len(aftNHG.NextHop); got != nhEntryGot {
		t.Fatalf("Prefix %s next-hop entry count: got %d, want 1", prefix, nhEntryGot)
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
func validateTrafficDistribution(t *testing.T, otg *ondatra.ATEDevice, wantWeights []float64, dstPorts []string) {
	dstPortInPktList := []uint64{}
	for _, dstPort := range dstPorts {
		otgP := otg.Port(t, dstPort)
		dstPortInPktList = append(dstPortInPktList, gnmi.Get(t, otg, gnmi.OC().Interface(otgP.Name()).Counters().InPkts().State()))
	}
	gotWeights, _ := normalize(dstPortInPktList)

	t.Log("got ratio:", gotWeights)
	t.Log("want ratio:", wantWeights)
	if diff := cmp.Diff(wantWeights, gotWeights, cmpopts.EquateApprox(0, trfDistTolerance)); diff != "" {
		t.Errorf("Packet distribution ratios -want,+got:\n%s", diff)
	}
}

func sendTraffic(t *testing.T, otg *ondatra.ATEDevice, allFlows []*ondatra.Flow) {
	t.Logf("*** Starting traffic ...")
	otg.Traffic().Start(t, allFlows...)
	time.Sleep(trafficDuration)
	t.Logf("*** Stop traffic ...")
	otg.Traffic().Stop(t)
}

// configreRoutePolicy adds route-policy config
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

func bgpWithNbr(as uint32, routerID string, nbr *oc.NetworkInstance_Protocol_Bgp_Neighbor, dut *ondatra.DUTDevice) *oc.NetworkInstance_Protocol {

	d := &oc.Root{}
	ni1 := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niProto := ni1.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := niProto.GetOrCreateBgp()
	bgp.GetOrCreateGlobal().As = ygot.Uint32(as)
	bgp.GetOrCreateGlobal().GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)

	if routerID != "" {
		bgp.Global.RouterId = ygot.String(routerID)
	}

	// Note: we have to define the peer group even if we aren't setting any policy because it's
	// invalid OC for the neighbor to be part of a peer group that doesn't exist.
	pg := bgp.GetOrCreatePeerGroup(peerGrpName)
	pg.PeerAs = ygot.Uint32(*nbr.PeerAs)
	pg.PeerGroupName = ygot.String(peerGrpName)
	pgaf := pg.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
	pgaf.Enabled = ygot.Bool(true)
	rpl := pgaf.GetOrCreateApplyPolicy()
	rpl.ImportPolicy = []string{policyName}
	rpl.ExportPolicy = []string{policyName}

	bgp.AppendNeighbor(nbr)
	return niProto
}

// func configureBGP(t *testing.T, dut *ondatra.DUTDevice, peer *ondatra.ATEDevice) {
// 	// Apply simple config
// 	dutConf := bgpWithNbr(dutAS, dutBGPRID, &oc.NetworkInstance_Protocol_Bgp_Neighbor{
// 		PeerAs:          ygot.Uint32(dutAS),
// 		NeighborAddress: ygot.String(ateIP),
// 		PeerGroup:       ygot.String(peerGrpName),
// 	}, dut)
// 	peerConf := bgpWithNbr(dutAS, peerBGPRID, &oc.NetworkInstance_Protocol_Bgp_Neighbor{
// 		PeerAs:          ygot.Uint32(dutAS),
// 		NeighborAddress: ygot.String(dutIP),
// 		PeerGroup:       ygot.String(peerGrpName),
// 	}, dut)

// }

func configureDevices(t *testing.T, dut, peer *ondatra.DUTDevice) {
	//Configure DUT Device
	t.Log("Configure VRFs")
	// configureNetworkInstance(t, dut)
	// t.Log("Configure Fallback in Encap VRF")

	t.Log("Configure DUT-TGEN Bundle Interface")
	// configureDUTInterfaces(t, dut)
	t.Log("Configure DUT-PEER dynamic Bundle Interface")
	bundleMap := util.ConfigureBundleIntfDynamic(t, dut, peer, 4)
	bundleIntfList := maps.Keys(bundleMap)
	t.Log("Configure BGP for DUT-PEER")
	configureDeviceBGP(t, dut, peer, bundleIntfList)
	t.Log("Configure Fallback in Encap VRF")

	t.Log("Configure WAN facing VRF selection Policy")
	wanPBR := CreatePbrPolicy(dut, wanPolicy, false)
	defaultNiPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut))
	gnmi.Replace(t, dut, defaultNiPath.PolicyForwarding().Config(), wanPBR)
	t.Log("Configure Cluster facing VRF selection Policy")
	clusterPBR := CreatePbrPolicy(dut, clusterPolicy, true)
	gnmi.Update(t, dut, defaultNiPath.PolicyForwarding().Config(), clusterPBR)

	//Configure PEER Device
}

func configureDeviceBGP(t *testing.T, dut, peer *ondatra.DUTDevice, bundList []string) {
	dutBund, peerBund := configureBundleIPAddr(t, dut, peer, bundList)
	configureRoutePolicy(t, dut, policyName, oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)
	configureRoutePolicy(t, peer, policyName, oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	fptest.ConfigureDefaultNetworkInstance(t, peer)
	dutBundlev4Addr := dutBund[bundList[0]]
	peerBundlev4Addr := peerBund[bundList[0]]
	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	peerConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	dutConf := bgpWithNbr(dutAS, dutBGPRID, &oc.NetworkInstance_Protocol_Bgp_Neighbor{
		PeerAs:          ygot.Uint32(dutAS),
		NeighborAddress: ygot.String(peerBundlev4Addr),
		PeerGroup:       ygot.String(peerGrpName),
	}, dut)
	peerConf := bgpWithNbr(dutAS, ateRID, &oc.NetworkInstance_Protocol_Bgp_Neighbor{
		PeerAs:          ygot.Uint32(dutAS),
		NeighborAddress: ygot.String(dutBundlev4Addr),
		PeerGroup:       ygot.String(peerGrpName),
	}, peer)
	gnmi.Update(t, dut, dutConfPath.Config(), dutConf)
	gnmi.Update(t, peer, peerConfPath.Config(), peerConf)
	t.Log("Verify iBGP session between DUT-PEER is established")
	cfgplugins.VerifyDUTBGPEstablished(t, dut)
	t.Log("Peer Tgen interface")
	p1 := peer.Port(t, "port16")
	gnmi.Replace(t, peer, gnmi.OC().Interface(p1.Name()).Config(), peerDst.NewOCInterface(p1.Name(), peer))
	t.Log("Configure eBGP between PEER and TGEN")
	peerEbgpConf := bgpWithNbr(dutAS, ateRID, &oc.NetworkInstance_Protocol_Bgp_Neighbor{
		PeerAs:          ygot.Uint32(ateAS),
		NeighborAddress: ygot.String(otgDst.IPv4),
		PeerGroup:       ygot.String(peerGrpName),
	}, peer)
	gnmi.Update(t, peer, peerConfPath.Config(), peerEbgpConf)
}

func configureBundleIPAddr(t *testing.T, dut, peer *ondatra.DUTDevice, bundlelist []string) (map[string]string, map[string]string) {
	//Configure Bundle interface IP address
	dutBundleIPMap := make(map[string]string)
	peerBundleIPMap := make(map[string]string)
	for i, bundle := range bundlelist {
		ipr, err := util.IncrementSubnetCIDR(dutPeerBundleIPRange, i)
		if err != nil {
			t.Errorf("Error in incrementing subnet")
		}
		dutIP, peerIP := util.GetUsableIPs(ipr)
		dutBundleIPMap[bundle] = dutIP.String()
		peerBundleIPMap[bundle] = peerIP.String()
		dutConf := fmt.Sprintf("interface %v ipv4 address %v/24", bundle, dutIP.String())
		peerConf := fmt.Sprintf("interface %v ipv4 address %v/24", bundle, peerIP.String())
		helpers.GnmiCLIConfig(t, dut, dutConf)
		helpers.GnmiCLIConfig(t, peer, peerConf)
	}
	return dutBundleIPMap, peerBundleIPMap
}

// configreOTG configures IP addresses on tgen Bundle1 & Bundle2 on the otg.
func configureOTG(t *testing.T, otg *ondatra.ATEDevice) gosnappi.Config {
	config := gosnappi.NewConfig()
	t.Log("Configuring DUT-TGEN Bundle interface1")
	aggID1 := "100"
	tgenBundle1 := []*ondatra.Port{
		otg.Port(t, "port1"),
		otg.Port(t, "port2"),
		otg.Port(t, "port3"),
		otg.Port(t, "port4"),
		otg.Port(t, "port5"),
		otg.Port(t, "port6"),
		otg.Port(t, "port7"),
		otg.Port(t, "port8"),
	}
	configureOTGBundle(t, otg, otgSrc1, config, tgenBundle1, aggID1)
	t.Log("Configuring DUT-TGEN Bundle interface1")
	aggID2 := "200"
	tgenBundle2 := []*ondatra.Port{
		otg.Port(t, "port1"),
		otg.Port(t, "port2"),
		otg.Port(t, "port3"),
		otg.Port(t, "port4"),
		otg.Port(t, "port5"),
		otg.Port(t, "port6"),
		otg.Port(t, "port7"),
		otg.Port(t, "port8"),
	}
	configureOTGBundle(t, otg, otgSrc1, config, tgenBundle2, aggID2)
	//Configure PEER-TGEN interface
	p16 := otg.Port(t, "port16")
	otgDst.AddToOTG(config, p16, &peerDst)
	//Configure PEER-TGEN eBGP
	dstPort := config.Ports().Add().SetName("port16")
	dstDev := config.Devices().Add().SetName(otgDst.Name)
	dstEth := dstDev.Ethernets().Add().SetName(otgDst.Name + ".Eth").SetMac(otgDst.MAC)
	dstEth.Connection().SetPortName(dstPort.Name())
	dstIpv4 := dstEth.Ipv4Addresses().Add().SetName(otgDst.Name + ".IPv4")
	dstIpv4.SetAddress(otgDst.IPv4).SetGateway(peerDst.IPv4).SetPrefix(uint32(otgDst.IPv4Len))
	dstIpv6 := dstEth.Ipv6Addresses().Add().SetName(otgDst.Name + ".IPv6")
	dstIpv6.SetAddress(otgDst.IPv6).SetGateway(peerDst.IPv6).SetPrefix(uint32(otgDst.IPv6Len))
	dstBgp := dstDev.Bgp().SetRouterId(dstIpv4.Address())
	dstBgp4Peer := dstBgp.Ipv4Interfaces().Add().SetIpv4Name(dstIpv4.Name()).Peers().Add().SetName(otgDst.Name + ".BGP4.peer")
	dstBgp4Peer.SetPeerAddress(dstIpv4.Gateway()).SetAsNumber(ateAS).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
	dstBgp6Peer := dstBgp.Ipv6Interfaces().Add().SetIpv6Name(dstIpv6.Name()).Peers().Add().SetName(otgDst.Name + ".BGP6.peer")
	dstBgp6Peer.SetPeerAddress(dstIpv6.Gateway()).SetAsNumber(ateAS).SetAsType(gosnappi.BgpV6PeerAsType.EBGP)
	configureOTGBGPv4Routes(dstBgp4Peer, dstIpv4.Address(), "v4Default", v4BGPDefault, 200)
	configureOTGBGPv6Routes(dstBgp6Peer, dstIpv6.Address(), "v6Default", v6BGPDefault, 200)
	t.Logf("Pushing config to otg and starting protocols...")
	otg.OTG().PushConfig(t, config)
	time.Sleep(30 * time.Second)
	otg.OTG().StartProtocols(t)
	time.Sleep(30 * time.Second)
	return config
}

func configureOTGBGPv4Routes(peer gosnappi.BgpV4Peer, ipv4 string, name string, prefix string, count uint32) {
	routes := peer.V4Routes().Add().SetName(name)
	routes.SetNextHopIpv4Address(ipv4).
		SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).
		SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)
	routes.Addresses().Add().
		SetAddress(prefix).
		SetPrefix(advertisedRoutesv4Prefix).
		SetCount(count)
}

func configureOTGBGPv6Routes(peer gosnappi.BgpV6Peer, ipv6 string, name string, prefix string, count uint32) {
	routes := peer.V6Routes().Add().SetName(name)
	routes.SetNextHopIpv6Address(ipv6).
		SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).
		SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL)
	routes.Addresses().Add().
		SetAddress(prefix).
		SetPrefix(advertisedRoutesv6Prefix).
		SetCount(count)
}

// configureBaseProfile configures DUT,PEER,TGEN baseconfig
func configureBaseProfile(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	peer := ondatra.DUT(t, "peer")
	otg := ondatra.ATE(t, "ate")
	t.Log("Configure DUT & PEER devices")
	configureDevices(t, dut, peer)
	t.Log("Configure TGEN OTG")
	configureOTG(t, otg)
	t.Log("wait")
}
