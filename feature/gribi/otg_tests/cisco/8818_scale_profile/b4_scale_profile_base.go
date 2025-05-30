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
package b4_scale_profile_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	util "github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	ospb "github.com/openconfig/gnoi/os"
	spb "github.com/openconfig/gnoi/system"
	"github.com/openconfig/gnoigo"
	gnpsipb "github.com/openconfig/gnpsi/proto/gnpsi"
	certzpb "github.com/openconfig/gnsi/certz"
	gribis "github.com/openconfig/gribi/v1/proto/service"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	ondatra_binding "github.com/openconfig/ondatra/binding"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
	p4pb "github.com/p4lang/p4runtime/go/p4/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
	DUTISISSysID             = "0000.0000.0001"
	PeerISISSysID            = "0000.0000.0002"
	ISISAreaID               = "49.0001"
	ISISName                 = "ISIS"
	ipv4OuterSrc111          = "198.51.100.111"
	ipv4OuterSrc222          = "198.51.100.222"
	innerSrcIPv4Start        = "198.19.0.1"
	innerDstIPv4Start        = "138.0.11.1"
	innerSrcIPv6Start        = "2001:DB8::198:1"
	innerDstIPv6Start        = "2001:db8::138:0:11:1"
	v4BGPDefault             = "203.0.113.0"
	v4BGPDefaultStart        = "203.0.113.1"
	v6BGPDefault             = "2001:DB8:2::1"
	v6BGPDefaultStart        = "2001:DB8:2::1:1"
	v4DefaultSrc             = "20.20.20.20"
	v6DefaultSrc             = "20:20:20::20"
	dutPeerBundleIPv4Range   = "88.1.1.0/24"
	dutPeerBundleIPv6Range   = "2001:DB8:1::/64"
	bundleSubIntIPv4Range    = "192.192.0.0/16"
	bundleSubIntIPv6Range    = "2001:C0C0::0/112"
	v4TunnelCount            = 1024
	v4TunnelNHGCount         = 256
	v4TunnelNHGSplitCount    = 2
	v4ReEncapNHGCount        = 256
	egressNHGSplitCount      = 16 //6
	vipLevelWeight           = 3
	transitLevelWeight       = 3
	encapLevelWeight         = 3
	flowCount                = 254
	ipTTL                    = uint32(255)
	vrfDecap                 = "DECAP_TE_VRF" //"DECAP"
	vrfTransit               = "TRANSIT_VRF"
	vrfRepaired              = "REPAIRED"
	vrfRepair                = "REPAIR"
	encapIPv4FlowIP          = "138.0.11.8"
	encapVrfIPv4Prefix       = "138.0.11.0/24"
	encapVrfIPv6Prefix       = "2001:db8::138:0:11:0/126"
	vrfEncapA                = "ENCAP_TE_VRF_A"
	vrfEncapB                = "ENCAP_TE_VRF_B"
	vrfEncapC                = "ENCAP_TE_VRF_C"
	vrfEncapD                = "ENCAP_TE_VRF_D"
	niDecapTeVrf             = "DECAP_TE_VRF"
	vrfDefault               = "DEFAULT"
	ipv4PrefixLen            = 30
	ipv6PrefixLen            = 126
	ethertypeIPv4            = oc.PacketMatchTypes_ETHERTYPE_ETHERTYPE_IPV4
	ethertypeIPv6            = oc.PacketMatchTypes_ETHERTYPE_ETHERTYPE_IPV6
	trfDistTolerance         = 0.02
	staticDstMAC             = "001a.1117.5f80" //Static ARP/Local Static MAC address
	lagName1                 = "LAGRx-1"        // LAG name for OTG
	lagName2                 = "LAGRx-2"        // LAG name for OTG
	tgenBundleID1            = "100"            // Bundle ID1 for OTG
	tgenBundleID2            = "200"            // Bundle ID1 for OTG
	advertisedRoutesv4Prefix = 24
	advertisedRoutesv6Prefix = 64
	dutAS                    = 68888
	ateAS                    = 67777
)

var (
	dutSrc1 = attrs.Attributes{
		Desc:    "dutSrc1",
		MAC:     "02:01:00:00:00:01",
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
		MAC:     "02:02:00:00:00:01",
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
		MAC:     "02:03:00:00:00:01",
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
	bundleList                = []util.BundleLinks{}
	primaryBundlesubIntfIPMap = map[string]util.LinkIPs{}
	backupBundlesubIntfIPMap  = map[string]util.LinkIPs{}
	pathInfo                  = PathInfo{}
	gribiScaleVal             = ScaleParam{
		V4TunnelCount:         v4TunnelCount,
		V4TunnelNHGCount:      v4TunnelNHGCount,
		V4TunnelNHGSplitCount: v4TunnelNHGSplitCount,
		EgressNHGSplitCount:   egressNHGSplitCount,
		V4ReEncapNHGCount:     v4ReEncapNHGCount,
	}

	nextBundleSubIntfIPv4, _, _ = net.ParseCIDR(bundleSubIntIPv4Range)
	nextBundleSubIntfIPv6, _, _ = net.ParseCIDR(bundleSubIntIPv6Range)
	primarySubIntfScale         = 10 //todo increase // number of sub-interfaces on primary bundle interface
	backupSubIntfScale          = 10 //todo increase // number of sub-interfaces on backup bundle interface
	primaryPercent              = 60
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
	withInnerHeader bool // flow type
	withNativeV6    bool
	withInnerV6     bool
	outerSrc        string   // source IP address
	outerDst        []string // destination IP addresses
	innerSrc        string
	innerDst        []string // set of destination IP addresses
	innerV4SrcStart string   // Inner v4 source IP address
	innerV4DstStart string   // Inner v4 destination IP address
	innerV6SrcStart string   // Inner v6 source IP address
	innerV6DstStart string   // Inner v6 destination IP address
	innerFlowCount  uint32
	// outerDscp       uint32   // DSCP value
	innerDscp uint32   // Inner DSCP value
	srcPort   []string // source OTG port
	dstPorts  []string // destination OTG ports
	srcMac    string   // source MAC address
	dstMac    string   // destination MAC address
	// dscp     uint32
	topo gosnappi.Config
}

// testArgs holds the objects needed by a test case.
type testArgs struct {
	ctx          context.Context
	client       *fluent.GRIBIClient
	dut          *ondatra.DUTDevice
	peer         *ondatra.DUTDevice
	ate          *ondatra.ATEDevice
	topo         gosnappi.Config
	electionID   gribi.Uint128
	primaryPaths []string
	frr1Paths    []string
	activeRp     string
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

// Traffic flow attributes
var (
	defaultV4 = trafficflowAttr{
		withInnerHeader: false, // flow type
		withNativeV6:    false,
		withInnerV6:     false,
		outerSrc:        v4DefaultSrc,                    // source IP address
		outerDst:        []string{v4BGPDefaultStart},     // destination IP address
		srcPort:         []string{lagName1 + ".IPv4"},    // source OTG port
		dstPorts:        []string{otgDst.Name + ".IPv4"}, // destination OTG ports
		srcMac:          otgSrc1.MAC,                     // source MAC address
		dstMac:          dutSrc1.MAC,                     // destination MAC address
		topo:            gosnappi.NewConfig(),
	}
)

// configureNetworkInstance Creates nonDefaultVRFs
func configureNetworkInstance(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	c := &oc.Root{}
	vrfs := []string{vrfDecap, vrfTransit, vrfRepaired, vrfEncapA, vrfEncapB, vrfEncapC, vrfEncapD, niDecapTeVrf}
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
func CreatePbrPolicy(t *testing.T, dut *ondatra.DUTDevice, name string, cluster_facing bool) *oc.NetworkInstance_PolicyForwarding {
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
		t.Logf("Rule is: %v", r)
		// sleep for 5 sec if rule is nill
		if r == nil {
			t.Errorf("Rule is nil for sequence %d in policy %s", pbrRule.sequence, name)
			t.Logf("Sleep for 5 sec and check the pbr rule")
			time.Sleep(5 * time.Second)
			t.Logf("Rule after sleep %v", r)
		}
		// if rule is still nill after 5 sec sleep fail fatal
		if r == nil {
			t.Fatalf("Failed to create or get rule for sequence %d in policy %s", pbrRule.sequence, name)
		}
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

// configureDUTBundle configures DUT side bundle for DUT-TGEN.
func configureDUTBundle(t *testing.T, dut *ondatra.DUTDevice, dutIntfAttr attrs.Attributes, aggPorts []*ondatra.Port, aggID string) {
	t.Helper()
	agg := dutIntfAttr.NewOCInterface(aggID, dut)
	agg.Type = oc.IETFInterfaces_InterfaceType_ieee8023adLag
	agg.GetOrCreateAggregation().LagType = oc.IfAggregate_AggregationType_STATIC
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
func configureOTGBundle(t *testing.T, ate *ondatra.ATEDevice, otgIntfAttr, dutIntfAttr attrs.Attributes, top gosnappi.Config, aggPorts []*ondatra.Port, lagName, aggID string) {
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
	dstEth.Ipv4Addresses().Add().SetName(lagName + ".IPv4").SetAddress(otgIntfAttr.IPv4).SetGateway(dutIntfAttr.IPv4).SetPrefix(uint32(otgIntfAttr.IPv4Len))
	dstEth.Ipv6Addresses().Add().SetName(lagName + ".IPv6").SetAddress(otgIntfAttr.IPv6).SetGateway(dutIntfAttr.IPv6).SetPrefix(uint32(otgIntfAttr.IPv6Len))
}

func configureDUTInterfaces(t *testing.T, dut *ondatra.DUTDevice) (aggID1, aggID2 string) {
	t.Log("Configuring DUT-TGEN Bundle interfaces")
	allPorts := dut.Ports()
	n := len(allPorts)
	mid := n / 2
	if n%2 != 0 {
		mid++ // Make first bundle larger if odd count
	}
	dutBundle1Member := allPorts[:mid]
	dutBundle2Member := allPorts[mid:]

	aggID1 = netutil.NextAggregateInterface(t, dut)
	configureDUTBundle(t, dut, dutSrc1, dutBundle1Member, aggID1)

	aggID2 = netutil.NextAggregateInterface(t, dut)
	configureDUTBundle(t, dut, dutSrc2, dutBundle2Member, aggID2)
	return aggID1, aggID2
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

// getTrafficFlow creates otg Traffic Flow with parameters Flowname, Inner v4 or v6, Outer DA & SA, DSCP, Dest Ports
// trafficflowAttr for setting the Inner IP DA/SA, Outer DA/SA, DSCP, Src/Dst Ports, Topology
func (fa *trafficflowAttr) createTrafficFlow(name string, dscp uint32) gosnappi.Flow {
	flow := fa.topo.Flows().Add().SetName(name)
	flow.Metrics().SetEnable(true)
	flow.TxRx().Device().SetTxNames(fa.srcPort).SetRxNames(fa.dstPorts)
	e1 := flow.Packet().Add().Ethernet()
	e1.Src().SetValue(fa.srcMac)
	e1.Dst().SetValue(fa.dstMac)
	if fa.withNativeV6 {
		v6 := flow.Packet().Add().Ipv6()
		v6.Src().SetValue(fa.outerSrc)
		v6.Dst().SetValues(fa.outerDst)
		v6.HopLimit().SetValue(ipTTL)
		v6.TrafficClass().SetValue(dscp << 2)
	} else {
		v4 := flow.Packet().Add().Ipv4()
		v4.Src().SetValue(fa.outerSrc)
		v4.Dst().SetValues(fa.outerDst)
		v4.TimeToLive().SetValue(ipTTL)
		v4.Priority().Dscp().Phb().SetValue(dscp)
		if fa.withInnerHeader {
			if fa.withInnerV6 {
				innerV6 := flow.Packet().Add().Ipv6()
				if len(fa.innerDst) > 0 { // use pre-defined inner destination addresses
					innerV6.Src().SetValue(fa.innerSrc)
					innerV6.Dst().SetValues(fa.innerDst)
				} else { // create inner srouce and destination addresses
					innerV6.Src().Increment().SetStart(fa.innerV6SrcStart).SetCount(fa.innerFlowCount)
					innerV6.Dst().Increment().SetStart(fa.innerV6DstStart).SetCount(fa.innerFlowCount)
				}
				innerV6.TrafficClass().SetValue(fa.innerDscp << 2)
			} else {
				innerV4 := flow.Packet().Add().Ipv4()
				if len(fa.innerDst) > 0 { // use pre-defined inner destination addresses
					innerV4.Src().SetValue(fa.innerSrc)
					innerV4.Dst().SetValues(fa.innerDst)
				} else { // create inner srouce and destination addresses}
					innerV4.Src().Increment().SetStart(fa.innerV4SrcStart).SetCount(fa.innerFlowCount)
					innerV4.Dst().Increment().SetStart(fa.innerV4DstStart).SetCount(fa.innerFlowCount)
				}
				innerV4.Priority().Dscp().Phb().SetValue(fa.innerDscp)
			}

		}
	}
	udp := flow.Packet().Add().Udp()
	udp.SrcPort().Increment().SetStart(1000).SetStep(10).SetCount(10000)
	udp.DstPort().Increment().SetStart(2000).SetStep(25).SetCount(10000)

	return flow
}

// validateTrafficFlows verifies that the flow on TGEN should pass for given flows
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

// getDecapFlows returns the ipv4inipv4 and ipv6inipv4 flows.
func getDecapFlows(decapEntries []string) []gosnappi.Flow {

	var dInV4 = trafficflowAttr{
		withInnerHeader: true, // flow type
		withNativeV6:    false,
		withInnerV6:     false,
		outerSrc:        v4DefaultSrc,                    // source IP address
		outerDst:        []string{v4BGPDefaultStart},     // destination IP address
		srcPort:         []string{lagName2 + ".IPv4"},    // source OTG port
		dstPorts:        []string{otgDst.Name + ".IPv4"}, // destination OTG ports
		srcMac:          otgSrc2.MAC,                     // source MAC address
		dstMac:          dutSrc2.MAC,                     // destination MAC address
		topo:            gosnappi.NewConfig(),
	}

	dInV4.outerDst = decapEntries

	dInV4.outerSrc = ipv4OuterSrc111
	dInV4.innerDst = encapVrfAIPv4Enries
	dInV4.innerSrc = otgSrc2.IPv4
	dInV4.innerDscp = dscpEncapA1
	flow1 := dInV4.createTrafficFlow("flow1", dscpEncapA1)

	dInV4.outerSrc = ipv4OuterSrc222
	dInV4.innerDst = encapVrfBIPv4Enries
	dInV4.innerDscp = dscpEncapB1
	flow2 := dInV4.createTrafficFlow("flow2", dscpEncapB1)

	// dInV4.outerSrc = ipv4OuterSrc111
	// dInV4.innerDst = encapVrfCIPv4Enries
	// dInV4.innerDscp = dscpEncapA2
	// flow3 := dInV4.createTrafficFlow("flow3", dscpEncapA2)

	// dInV4.outerSrc = ipv4OuterSrc222
	// dInV4.innerDst = encapVrfDIPv4Enries
	// dInV4.innerDscp = dscpEncapB2
	// flow4 := dInV4.createTrafficFlow("flow4", dscpEncapB2)

	dInV4.withInnerV6 = true
	dInV4.outerSrc = ipv4OuterSrc111
	dInV4.innerDst = encapVrfAIPv6Enries
	dInV4.innerSrc = otgSrc2.IPv6
	dInV4.innerDscp = dscpEncapA1
	flow5 := dInV4.createTrafficFlow("flow5", dscpEncapA1)

	dInV4.outerSrc = ipv4OuterSrc222
	dInV4.innerDst = encapVrfBIPv6Enries
	dInV4.innerDscp = dscpEncapB1
	flow6 := dInV4.createTrafficFlow("flow6", dscpEncapB1)

	// dInV4.outerSrc = ipv4OuterSrc111
	// dInV4.innerDst = encapVrfCIPv6Enries
	// dInV4.innerDscp = dscpEncapA2
	// flow7 := dInV4.createTrafficFlow("flow7", dscpEncapA2)

	// dInV4.outerSrc = ipv4OuterSrc222
	// dInV4.innerDst = encapVrfDIPv6Enries
	// dInV4.innerDscp = dscpEncapB2
	// flow8 := dInV4.createTrafficFlow("flow8", dscpEncapB2)

	return []gosnappi.Flow{flow1, flow2, flow5, flow6}

}

// getEncapFlows returns ipv4 and ipv6 flows. These flows are used to simulate clusterfacing traffic.
func getEncapFlows() []gosnappi.Flow {

	// encap flow attribute
	var enFa = trafficflowAttr{
		withInnerHeader: false, // flow type
		withNativeV6:    false,
		withInnerV6:     false,
		outerSrc:        v4DefaultSrc,                    // source IP address
		outerDst:        []string{v4BGPDefaultStart},     // destination IP address
		srcPort:         []string{lagName1 + ".IPv4"},    // source OTG port
		dstPorts:        []string{otgDst.Name + ".IPv4"}, // destination OTG ports
		srcMac:          otgSrc1.MAC,                     // source MAC address
		dstMac:          dutSrc1.MAC,                     // destination MAC address
		topo:            gosnappi.NewConfig(),
	}

	enFa.outerDst = encapVrfAIPv4Enries
	flow1 := enFa.createTrafficFlow("flow1", dscpEncapA1)

	enFa.outerDst = encapVrfBIPv4Enries
	flow2 := enFa.createTrafficFlow("flow2", dscpEncapB1)

	// ipv6 native traffic
	enFa.withNativeV6 = true
	enFa.srcPort = []string{lagName1 + ".IPv6"}

	enFa.outerSrc = innerSrcIPv6Start
	enFa.outerDst = encapVrfAIPv6Enries
	flow3 := enFa.createTrafficFlow("flow3", dscpEncapA1)

	enFa.outerDst = encapVrfBIPv6Enries
	flow4 := enFa.createTrafficFlow("flow4", dscpEncapB1)

	return []gosnappi.Flow{flow1, flow2, flow3, flow4}
}

// validateAftTelmetry verifies aft telemetry entries.
// func (a *testArgs) validateAftTelemetry(t *testing.T, vrfName, prefix string, nhEntryGot int) {
// 	aftPfxPath := gnmi.OC().NetworkInstance(vrfName).Afts().Ipv4Entry(prefix)
// 	aftPfxVal, found := gnmi.Watch(t, a.dut, aftPfxPath.State(), 2*time.Minute, func(val *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv4Entry]) bool {
// 		value, present := val.Val()
// 		return present && value.GetNextHopGroup() != 0
// 	}).Await(t)
// 	if !found {
// 		t.Fatalf("Could not find prefix %s in telemetry AFT", prefix)
// 	}
// 	aftPfx, _ := aftPfxVal.Val()

// 	aftNHG := gnmi.Get(t, a.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(a.dut)).Afts().NextHopGroup(aftPfx.GetNextHopGroup()).State())
// 	if got := len(aftNHG.NextHop); got != nhEntryGot {
// 		t.Fatalf("Prefix %s next-hop entry count: got %d, want 1", prefix, nhEntryGot)
// 	}
// }

// normalize normalizes the input values so that the output values sum
// to 1.0 but reflect the proportions of the input.  For example,
// input [1, 2, 3, 4] is normalized to [0.1, 0.2, 0.3, 0.4].
// func normalize(xs []uint64) (ys []float64, sum uint64) {
// 	for _, x := range xs {
// 		sum += x
// 	}
// 	ys = make([]float64, len(xs))
// 	for i, x := range xs {
// 		ys[i] = float64(x) / float64(sum)
// 	}
// 	return ys, sum
// }

// validateTrafficDistribution checks if the packets received on receiving ports are within specificied weight ratios
// func validateTrafficDistribution(t *testing.T, otg *ondatra.ATEDevice, wantWeights []float64, dstPorts []string) {
// 	dstPortInPktList := []uint64{}
// 	for _, dstPort := range dstPorts {
// 		otgP := otg.Port(t, dstPort)
// 		dstPortInPktList = append(dstPortInPktList, gnmi.Get(t, otg, gnmi.OC().Interface(otgP.Name()).Counters().InPkts().State()))
// 	}
// 	gotWeights, _ := normalize(dstPortInPktList)

// 	t.Log("got ratio:", gotWeights)
// 	t.Log("want ratio:", wantWeights)
// 	if diff := cmp.Diff(wantWeights, gotWeights, cmpopts.EquateApprox(0, trfDistTolerance)); diff != "" {
// 		t.Errorf("Packet distribution ratios -want,+got:\n%s", diff)
// 	}
// }

// sendTraffic starts traffic flows and send traffic for a fixed duration
func sendTraffic(t *testing.T, args *testArgs, flows []gosnappi.Flow, capture bool) {
	otg := args.ate.OTG()
	args.topo.Flows().Clear().Items()
	args.topo.Flows().Append(flows...)
	// t.Logf("Flow Configuration: %v", flows)
	// t.Logf("OTG Configuration: %v", args.topo)
	otg.PushConfig(t, args.topo)
	time.Sleep(30 * time.Second) // time for otg ARP to settle
	otg.StartProtocols(t)
	time.Sleep(300 * time.Second) // time for otg ARP to settle
	t.Log("Verify BGP establsihed after OTG start protocols")
	// otgutils.WaitForARP(t, otg, args.topo, "IPv4")
	// otgutils.WaitForARP(t, otg, args.topo, "IPv6")
	cfgplugins.VerifyDUTBGPEstablished(t, args.peer)
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

// splitPrimaryBackup splits the input interface list into primary and backup slices
// according to the given primaryPercent (0-100). Ensures at least one interface in each group
// and at least two interfaces in total.
func splitPrimaryBackup(primaryPercent int, interfaces []util.BundleLinks) (primary, backup []util.BundleLinks, err error) {
	n := len(interfaces)
	if n < 2 {
		return nil, nil, fmt.Errorf("at least two interfaces required, got %d", n)
	}
	if primaryPercent < 0 || primaryPercent > 100 {
		return nil, nil, fmt.Errorf("primaryPercent must be between 0 and 100, got %d", primaryPercent)
	}
	primaryCount := n * primaryPercent / 100
	if primaryCount == 0 {
		primaryCount = 1
	}
	if primaryCount >= n {
		primaryCount = n - 1
	}
	primary = interfaces[:primaryCount]
	backup = interfaces[primaryCount:]
	return primary, backup, nil
}

type Bundles []util.BundleLinks

// ConfigureBundleLinkIPs assigns IPv4/IPv6 addresses to each InterfacePhysicalLink in the BundleMap
// and configures those addresses on the DUT and PEER devices.
func (bundles Bundles) ConfigureBundleLinkIPs(t *testing.T, dut, peer *ondatra.DUTDevice, dutPeerBundleIPv4Range, dutPeerBundleIPv6Range string) error {
	linkIndex := 0
	for _, bundle := range bundles {
		for i := range bundle.Links {
			ipv4Subnet, err := util.IncrementSubnetCIDR(dutPeerBundleIPv4Range, linkIndex)
			if err != nil {
				return fmt.Errorf("failed to increment IPv4 subnet: %v", err)
			}
			ipv6Subnet, err := util.IncrementSubnetCIDR(dutPeerBundleIPv6Range, linkIndex)
			if err != nil {
				return fmt.Errorf("failed to increment IPv6 subnet: %v", err)
			}
			intfV4, peerV4 := util.GetUsableIPs(ipv4Subnet)
			intfV6, peerV6 := util.GetUsableIPs(ipv6Subnet)
			bundle.Links[i].IntfV4Addr = intfV4.String()
			bundle.Links[i].PeerV4Addr = peerV4.String()
			bundle.Links[i].IntfV6Addr = intfV6.String()
			bundle.Links[i].PeerV6Addr = peerV6.String()

			// Configure IPv4 address on DUT and PEER
			dutConf := fmt.Sprintf("interface %v ipv4 address %v/24", bundle.Name, bundle.Links[i].IntfV4Addr)
			peerConf := fmt.Sprintf("interface %v ipv4 address %v/24", bundle.Name, bundle.Links[i].PeerV4Addr)
			helpers.GnmiCLIConfig(t, dut, dutConf)
			helpers.GnmiCLIConfig(t, peer, peerConf)

			// Configure IPv6 address on DUT and PEER
			dutConf = fmt.Sprintf("interface %v ipv6 address %v/64", bundle.Name, bundle.Links[i].IntfV6Addr)
			peerConf = fmt.Sprintf("interface %v ipv6 address %v/64", bundle.Name, bundle.Links[i].PeerV6Addr)
			helpers.GnmiCLIConfig(t, dut, dutConf)
			helpers.GnmiCLIConfig(t, peer, peerConf)

			linkIndex++
		}
	}
	return nil
}

type PathInfo struct {
	// bundleMode: true if bundle interface,  false if physical interface
	bundleMode         bool
	PrimaryInterface   []string
	PrimaryPathsV4     []string
	PrimaryPathsV6     []string
	PrimaryPathsPeerV4 []string
	PrimaryPathsPeerV6 []string
	// PrimaryIntfLcs and PrimaryPeerLcs holds either a []string or [][]string depending on the interface mode.
	// If bundleMode is false (physical interface mode), PrimaryIntfLcs is a []string.
	// If bundleMode is true (bundle interface mode), PrimaryIntfLcs is a [][]string.
	// Use type assertion after checking bundleMode:
	//   if pathInfo.bundleMode {
	//       lcs2d, ok := pathInfo.PrimaryIntfLcs.([][]string) // bundle mode
	//   } else {
	//       lcs, ok := pathInfo.PrimaryIntfLcs.([]string)     // physical mode
	//   }
	PrimaryIntfLcs any
	PrimaryPeerLcs any
	PrimarySubIntf map[string]util.LinkIPs

	BackupInterface   []string
	BackupPathsV4     []string
	BackupPathsV6     []string
	BackupPathsPeerV4 []string
	BackupPathsPeerV6 []string
	// BackupIntfLcs and BackupPeerLcs holds either a []string or [][]string depending on the interface mode.
	// If bundleMode is false (physical interface mode), PrimaryIntfLcs is a []string.
	// If bundleMode is true (bundle interface mode), PrimaryIntfLcs is a [][]string.
	// Use type assertion after checking bundleMode:
	//   if pathInfo.bundleMode {
	//       lcs2d, ok := pathInfo.PrimaryIntfLcs.([][]string) // bundle mode
	//   } else {
	//       lcs, ok := pathInfo.PrimaryIntfLcs.([]string)     // physical mode
	//   }
	BackupIntfLcs any
	BackupPeerLcs any
	BackupSubIntf map[string]util.LinkIPs
}

// fillPathInfoInterface populates the PathInfo struct's interface-related fields.
func (p *PathInfo) fillPathInfoInterface(
	primaryInterfaces, backupInterfaces []util.BundleLinks,
) {
	primaryIntfsName := util.ExtractBundleLinkField(primaryInterfaces, "name")
	backupIntfsName := util.ExtractBundleLinkField(backupInterfaces, "name")

	p.PrimaryInterface = util.ToStringSlice(primaryIntfsName)
	p.PrimaryPathsV4 = util.ToStringSlice(util.ExtractBundleLinkField(primaryInterfaces, "intfv4addr"))
	p.PrimaryPathsV6 = util.ToStringSlice(util.ExtractBundleLinkField(primaryInterfaces, "intfv6addr"))
	p.PrimaryPathsPeerV4 = util.ToStringSlice(util.ExtractBundleLinkField(primaryInterfaces, "peerintfv4addr"))
	p.PrimaryPathsPeerV6 = util.ToStringSlice(util.ExtractBundleLinkField(primaryInterfaces, "peerintfv6addr"))
	p.PrimaryIntfLcs = util.ExtractBundleLinkField(primaryInterfaces, "linecardnumber")
	p.PrimaryPeerLcs = util.ExtractBundleLinkField(primaryInterfaces, "linecardnumber")

	p.BackupInterface = util.ToStringSlice(backupIntfsName)
	p.BackupPathsV4 = util.ToStringSlice(util.ExtractBundleLinkField(backupInterfaces, "intfv4addr"))
	p.BackupPathsV6 = util.ToStringSlice(util.ExtractBundleLinkField(backupInterfaces, "intfv6addr"))
	p.BackupPathsPeerV4 = util.ToStringSlice(util.ExtractBundleLinkField(backupInterfaces, "peerintfv4addr"))
	p.BackupPathsPeerV6 = util.ToStringSlice(util.ExtractBundleLinkField(backupInterfaces, "peerintfv6addr"))
	p.BackupIntfLcs = util.ExtractBundleLinkField(backupInterfaces, "peerlinecardnumber")
	p.BackupPeerLcs = util.ExtractBundleLinkField(backupInterfaces, "peerlinecardnumber")
}

// fillPathInfoSubInterface populates the PathInfo struct's subinterface-related fields.
func (p *PathInfo) fillPathInfoSubInterface(
	primaryBundlesubIntfIPMap, backupBundlesubIntfIPMap map[string]util.LinkIPs,
) {
	p.PrimarySubIntf = primaryBundlesubIntfIPMap
	p.BackupSubIntf = backupBundlesubIntfIPMap
}

func configureDevices(t *testing.T, dut, peer *ondatra.DUTDevice, interfaceMode string) {
	//Configure DUT Device
	t.Log("Configure VRFs")
	configureNetworkInstance(t, dut)
	//Set leaf ref config for default NI
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	fptest.ConfigureDefaultNetworkInstance(t, peer)
	// t.Log("Configure Fallback in Encap VRF")
	if interfaceMode == "bundle" {
		t.Log("Configure DUT-TGEN Bundle Interface")
		aggID1, aggID2 := configureDUTInterfaces(t, dut)
		t.Log("Configure DUT-PEER dynamic Bundle Interface")
		bundleListAll := util.ConfigureBundleIntfDynamic(t, dut, peer, 4, dutPeerBundleIPv4Range, dutPeerBundleIPv6Range)
		if len(bundleListAll) < 2 {
			t.Fatalf("Expected at least 2 bundles (one for bgp/isis other for test), got %d", len(bundleList))
		}
		bundleBgpIsis := bundleListAll[0] // first interface is for BGP/ISIS
		bundleList = bundleListAll[1:]    // rest of the interfaces are for test
		configureDeviceBGP(t, dut, peer, bundleBgpIsis)
		t.Log("Configure sub Interface for Dynamic Bundle DUT-PEER")
		primaryIntfs, backupIntfs, err := splitPrimaryBackup(primaryPercent, bundleList)
		if err != nil {
			t.Fatal(err)
		}
		// pathInfo: global variable
		pathInfo.bundleMode = true
		pathInfo.fillPathInfoInterface(primaryIntfs, backupIntfs)

		nextBundleSubIntfIPv4, nextBundleSubIntfIPv6, primaryBundlesubIntfIPMap = util.CreateBundleSubInterfaces(t, dut, peer, pathInfo.PrimaryInterface, primarySubIntfScale, nextBundleSubIntfIPv4, nextBundleSubIntfIPv6)
		nextBundleSubIntfIPv4, nextBundleSubIntfIPv6, backupBundlesubIntfIPMap = util.CreateBundleSubInterfaces(t, dut, peer, pathInfo.BackupInterface, backupSubIntfScale, nextBundleSubIntfIPv4, nextBundleSubIntfIPv6)
		nextBundleSubIntfIPv4, _, _ = net.ParseCIDR(bundleSubIntIPv4Range)
		nextBundleSubIntfIPv6, _, _ = net.ParseCIDR(bundleSubIntIPv6Range)
		pathInfo.fillPathInfoSubInterface(primaryBundlesubIntfIPMap, backupBundlesubIntfIPMap)

		t.Logf("Path info: %v", pathInfo)
		t.Log("Configure ISIS for DUT-PEER")
		configureDeviceISIS(t, dut, peer, bundleBgpIsis)
		t.Log("Configure Fallback in Encap VRF")
		t.Log("Configure WAN facing VRF selection Policy")
		wanPBR := CreatePbrPolicy(t, dut, wanPolicy, false)
		defaultNiPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut))
		gnmi.Replace(t, dut, defaultNiPath.PolicyForwarding().Config(), wanPBR)
		t.Log("Configure Cluster facing VRF selection Policy")
		clusterPBR := CreatePbrPolicy(t, dut, clusterPolicy, true)
		gnmi.Update(t, dut, defaultNiPath.PolicyForwarding().Config(), clusterPBR)
		//Apply Cluster facing policy on DUT-TGEN Bundle1 interface
		applyForwardingPolicy(t, aggID1, clusterPolicy, false)
		//Apply WAN facing policy on DUT-TGEN Bundle2 interface
		applyForwardingPolicy(t, aggID2, wanPolicy, false)
	} else if interfaceMode == "physical" {
		t.Log("Configure DUT-TGEN Physical Interfaces")
		// TODO: Implement physical interface configuration logic
		// Add your physical interface configuration logic here
		// Example:
		// configureDUTPhysicalInterfaces(t, dut)
		// configureDeviceBGP(t, dut, peer, physicalIntfList)
		// configureDeviceISIS(t, dut, peer, physicalIntfList)
	} else {
		t.Fatalf("Unknown mode: %s. Must be 'bundle' or 'physical'", interfaceMode)
	}
}

func configureDeviceBGP(t *testing.T, dut, peer *ondatra.DUTDevice, bgpLink util.BundleLinks) {
	// dutBundleIPMap, peerBundleIPMap = configureBundleIPAddr(t, dut, peer, bundList)
	configureRoutePolicy(t, dut, policyName, oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)
	configureRoutePolicy(t, peer, policyName, oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)
	// dutBundlev4Addr := dutBundleIPMap[bundList]
	// peerBundlev4Addr := peerBundleIPMap[bundList]
	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	peerConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	dutConf := bgpWithNbr(dutAS, dutBGPRID, &oc.NetworkInstance_Protocol_Bgp_Neighbor{
		PeerAs:          ygot.Uint32(dutAS),
		NeighborAddress: ygot.String(bgpLink.PeerV4Addr), //peerBundlev4Addr.ipv4
		PeerGroup:       ygot.String(peerGrpName),
	}, dut)
	peerConf := bgpWithNbr(dutAS, ateRID, &oc.NetworkInstance_Protocol_Bgp_Neighbor{
		PeerAs:          ygot.Uint32(dutAS),
		NeighborAddress: ygot.String(bgpLink.IntfV4Addr), //dutBundlev4Addr.ipv4
		PeerGroup:       ygot.String(peerGrpName),
	}, peer)
	gnmi.Update(t, dut, dutConfPath.Config(), dutConf)
	gnmi.Update(t, peer, peerConfPath.Config(), peerConf)
	t.Log("Verify iBGP session between DUT-PEER is established")
	cfgplugins.VerifyDUTBGPEstablished(t, dut)
	t.Log("Config Peer Tgen interface")
	ports := peer.Ports() // Get the list of ports from peer
	if len(ports) > 0 {
		peerLastPort := ports[len(ports)-1]
		gnmi.Replace(t, peer, gnmi.OC().Interface(peerLastPort.Name()).Config(), peerDst.NewOCInterface(peerLastPort.Name(), peer))
	} else {
		t.Fatalf("No ports found on peer device %s", peer.ID())
	}
	t.Log("Configure eBGP between PEER and TGEN")
	peerEbgpConf := bgpWithNbr(dutAS, ateRID, &oc.NetworkInstance_Protocol_Bgp_Neighbor{
		PeerAs:          ygot.Uint32(ateAS),
		NeighborAddress: ygot.String(otgDst.IPv4),
		PeerGroup:       ygot.String(peerGrpName),
	}, peer)
	gnmi.Update(t, peer, peerConfPath.Config(), peerEbgpConf)
}

func configureDeviceISIS(t *testing.T, dut, peer *ondatra.DUTDevice, isisIntf util.BundleLinks) {
	root := &oc.Root{}
	t.Log("Configure ISIS on DUT")
	util.AddISISOCWithSysAreaID(t, dut, isisIntf.Name, DUTISISSysID, ISISAreaID, ISISName)
	t.Log("Configure ISIS on PEER")
	util.AddISISOCWithSysAreaID(t, peer, isisIntf.Name, PeerISISSysID, ISISAreaID, ISISName)
	t.Log("Verify ISIS session between DUT-PEER is established")
	awaitISISAdjacency(t, dut, isisIntf.Name)
	//redirstribute v4 connected routes
	tableConnv4 := root.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(peer)).GetOrCreateTableConnection(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_DIRECTLY_CONNECTED, oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, oc.Types_ADDRESS_FAMILY_IPV4)
	gnmi.Update(t, peer, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(peer)).TableConnection(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_DIRECTLY_CONNECTED, oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, oc.Types_ADDRESS_FAMILY_IPV4).Config(), tableConnv4)
	//redirstribute v4 connected routes
	tableConnv6 := root.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(peer)).GetOrCreateTableConnection(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_DIRECTLY_CONNECTED, oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, oc.Types_ADDRESS_FAMILY_IPV6)
	gnmi.Update(t, peer, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(peer)).TableConnection(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_DIRECTLY_CONNECTED, oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, oc.Types_ADDRESS_FAMILY_IPV6).Config(), tableConnv6)
}

func awaitISISAdjacency(t *testing.T, dut *ondatra.DUTDevice, intfName string) {
	isisPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, ISISName).Isis()
	intf := isisPath.Interface(intfName)

	query := intf.LevelAny().AdjacencyAny().AdjacencyState().State()
	_, ok := gnmi.WatchAll(t, dut, query, time.Minute, func(val *ygnmi.Value[oc.E_Isis_IsisInterfaceAdjState]) bool {
		v, ok := val.Val()
		return v == oc.Isis_IsisInterfaceAdjState_UP && ok
	}).Await(t)

	if !ok {
		t.Fatalf("IS-IS adjacency was not formed on interface %v", intfName)
	}
}

func getPortIDs(dev *ondatra.DUTDevice) []string {
	var ids []string
	for _, p := range dev.Ports() {
		ids = append(ids, p.ID())
	}
	return ids
}

// configreOTG configures IP addresses on tgen Bundle1 & Bundle2 on the otg.
func configureOTG(t *testing.T, otg *ondatra.ATEDevice, dut, peer *ondatra.DUTDevice) gosnappi.Config {
	topo := gosnappi.NewConfig()
	t.Log("Configuring DUT-TGEN Bundle interface1")
	// Suppose you have these slices of port IDs:
	dutPortIDs := getPortIDs(dut)
	peerPortIDs := getPortIDs(peer)

	// collect DUT port IDs
	dutPortMap := make(map[string]struct{}, len(dutPortIDs))
	for _, id := range dutPortIDs {
		dutPortMap[id] = struct{}{}
	}
	// collect PEER port ID
	peerPortMap := make(map[string]struct{}, len(peerPortIDs))
	for _, id := range peerPortIDs {
		peerPortMap[id] = struct{}{}
	}
	// Create OTG source and destination ports
	var dutPorts, peerInterface []*ondatra.Port
	for _, p := range otg.Ports() {
		if _, ok := dutPortMap[p.ID()]; ok {
			dutPorts = append(dutPorts, p) // rename
		} else if _, ok := peerPortMap[p.ID()]; ok {
			peerInterface = append(peerInterface, p)
		}
	}
	if len(dutPorts) == 0 {
		t.Fatalf("No DUT ports found on OTG device %s", otg.Name())
	}
	if len(peerInterface) == 0 {
		t.Fatalf("No PEER ports found on OTG device %s", otg.Name())
	}
	n := len(dutPorts)
	mid := n / 2
	if n%2 != 0 {
		mid++ // Make first bundle larger if odd count
	}
	tgenBundle1 := dutPorts[:mid]
	tgenBundle2 := dutPorts[mid:]

	aggID1 := "100"
	configureOTGBundle(t, otg, otgSrc1, dutSrc1, topo, tgenBundle1, lagName1, aggID1)
	t.Log("Configuring DUT-TGEN Bundle interface2")
	aggID2 := "200"
	configureOTGBundle(t, otg, otgSrc2, dutSrc2, topo, tgenBundle2, lagName2, aggID2)

	//Configure PEER-TGEN interface - Destination port
	peerLastPort := peerInterface[len(peerInterface)-1].ID()
	dstPort := topo.Ports().Add().SetName(peerLastPort)
	dstDev := topo.Devices().Add().SetName(otgDst.Name)
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
	configureOTGBGPv4Routes(dstBgp4Peer, dstIpv4.Address(), "v4Default", v4BGPDefault, 20000)
	configureOTGBGPv6Routes(dstBgp6Peer, dstIpv6.Address(), "v6Default", v6BGPDefault, 20000)
	t.Logf("Pushing config to otg and starting protocols...")
	otg.OTG().PushConfig(t, topo)
	time.Sleep(30 * time.Second)
	otg.OTG().StartProtocols(t)
	time.Sleep(30 * time.Second)
	// otgutils.WaitForARP(t, otg.OTG(), topo, "IPv4")
	// otgutils.WaitForARP(t, otg.OTG(), topo, "IPv6")
	return topo
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

// awaitTimeout calls a fluent client Await, adding a timeout to the context.
func awaitTimeout(ctx context.Context, c *fluent.GRIBIClient, t testing.TB, timeout time.Duration) error {
	subctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return c.Await(subctx, t)
}

// configureBaseProfile configures DUT,PEER,TGEN baseconfig
func configureBaseProfile(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	peer := ondatra.DUT(t, "peer")
	otg := ondatra.ATE(t, "ate")

	ctx := context.Background()
	gribic := dut.RawAPIs().GRIBI(t)
	client := fluent.NewClient()
	client.Connection().WithStub(gribic).WithPersistence().WithInitialElectionID(1, 0).
		WithRedundancyMode(fluent.ElectedPrimaryClient).WithFIBACK()

	client.Start(ctx, t)
	// cleanup all existing gRIBI entries at the end of the test
	defer gribi.FlushAll(client)
	// cleanup all existing gRIBI entries in the begining of the test
	if err := gribi.FlushAll(client); err != nil {
		t.Error(err)
	}
	client.Await(ctx, t)
	// Wait for the gribi entries get flushed
	// time.Sleep(300 * time.Second)
	defer client.Stop(t)

	t.Log("Configure DUT & PEER devices")
	configureDevices(t, dut, peer, "bundle")
	t.Log("Configure TGEN OTG")
	topo := configureOTG(t, otg, dut, peer)
	// t.Log("OTG CONFIG: ", topo)
	tcArgs := &testArgs{
		dut:    dut,
		peer:   peer,
		ate:    otg,
		topo:   topo,
		client: client,
		ctx:    ctx,
	}
	t.Run("Verify default BGP traffic", func(t *testing.T) {
		v4BGPFlow := defaultV4.createTrafficFlow("DefaultV4", dscpEncapNoMatch)
		validateTrafficFlows(t, tcArgs, []gosnappi.Flow{v4BGPFlow}, false, true)
	})

	// add static route on peer for the tunnel destination for encap, decap+encap traffic
	configStaticRoute(t, peer, "200.200.0.0/16", otgDst.IPv4, "", "", false)
	t.Log("Program base gRIBI entries")
	BaseGRIBIProgramming(t, tcArgs, pathInfo.PrimaryPathsPeerV4, gribiScaleVal, 1, transitLevelWeight, vipLevelWeight)
}

type DUTResources struct {
	Device      *ondatra.DUTDevice
	GNMI        gnmipb.GNMIClient
	GNSI        ondatra_binding.GNSIClients
	GNOI        gnoigo.Clients
	GNPSI       gnpsipb.GNPSIClient
	CLI         ondatra_binding.CLIClient
	P4RT        p4pb.P4RuntimeClient
	Console     ondatra_binding.ConsoleClient
	OSC         ospb.OSClient
	SC          spb.SystemClient
	GRIBI       gribis.GRIBIClient
	FluentGRIBI *fluent.GRIBIClient
	LCs         []string
	DualSup     bool
}
type OTGResources struct {
	Device *ondatra.ATEDevice
	GNMI   gnmipb.GNMIClient
}

// TestResources holds common resources used across tests.
type TestResources struct {
	DUT    DUTResources
	PEER   DUTResources
	OTG    OTGResources
	LogDir string
	// dualSup         bool
	// reader          io.ReadCloser
	ctx             context.Context
	CommandPatterns map[string]map[string]interface{}
}

var (
	testResources *TestResources
	once          sync.Once
)

// TODO complete this ReconnectClients
// Try to move to utils

// CheckBootTime is a method of TestResources that checks the boot time for DUT and PEER
func (tRes *TestResources) ReconnectClients(t *testing.T, maxRebootTime uint64) {
	// t.Log("Reconnect CLI")
	// reconnectCLI(t, tRes.DUT.CLI, "DUT", maxRebootTime)
	// reconnectCLI(t, tRes.PEER.CLI, "PEER", maxRebootTime)

	// t.Log("Reconnect FluentGRIBI")
	// reconnectFluentGribi(t, tRes.DUT.FluentGRIBI, "DUT", maxRebootTime)
	// reconnectFluentGribi(t, tRes.PEER.FluentGRIBI, "PEER", maxRebootTime)

	t.Log("Reconnect gNOI")
	reconnectGnoi(t, tRes.DUT.GNOI, "DUT", maxRebootTime)
	reconnectGnoi(t, tRes.PEER.GNOI, "PEER", maxRebootTime)

	t.Log("Reconnect gNMI")
	reconnectGnmi(t, tRes.DUT.GNMI, "DUT", maxRebootTime)
	reconnectGnmi(t, tRes.PEER.GNMI, "PEER", maxRebootTime)

	t.Log("Reconnect gNSI")
	reconnectGnsi(t, tRes.DUT.GNSI, "DUT", maxRebootTime)
	reconnectGnsi(t, tRes.PEER.GNSI, "PEER", maxRebootTime)

	t.Log("Reconnect P4RT")
	reconnectP4RT(t, tRes.DUT.P4RT, "DUT", maxRebootTime)
	reconnectP4RT(t, tRes.PEER.P4RT, "PEER", maxRebootTime)

	t.Log("Reconnect gRIBI")
	reconnectGribi(t, tRes.DUT.GRIBI, "DUT", maxRebootTime)
	reconnectGribi(t, tRes.PEER.GRIBI, "PEER", maxRebootTime)
}

// reconnectGnoi attempts to reconnect to a device using the gNOI client and waits until the device is ready.
// It periodically checks the device's system time to determine its availability.
//
// Parameters:
//   - t: The testing context used for logging and error reporting.
//   - gnoiClient: The gNOI client used to communicate with the device.
//   - deviceName: The name of the device being checked.
//   - maxRebootTime: The maximum allowed reboot time (in minutes) before the function fails the test.
//
// Behavior:
//   - Logs the elapsed time since the reboot started.
//   - Continuously attempts to fetch the device's system time using the gNOI client.
//   - If the service is unavailable, waits for 30 seconds before retrying.
//   - If the device becomes available, logs a success message and exits the loop.
//   - If the elapsed time exceeds maxRebootTime, the function fails the test with a fatal error.
//   - Logs the total time taken for the device to become ready.
func reconnectGnoi(t *testing.T, gnoiClient gnoigo.Clients, deviceName string, maxRebootTime uint64) {
	startReboot := time.Now()
	t.Logf("%s boot time: %.2f minutes", deviceName, time.Since(startReboot).Minutes())
	for {
		ctx := context.Background()
		response, err := gnoiClient.System().Time(ctx, &spb.TimeRequest{})

		// Log the error if it occurs
		if err != nil {
			t.Logf("Error fetching %s device time: %v", deviceName, err)
		}

		// Check if the error code indicates that the service is unavailable
		if status.Code(err) == codes.Unavailable {
			// If the service is unavailable, wait for 30 seconds before retrying
			t.Logf("%s service unavailable, retrying in 30 seconds...", deviceName)
			time.Sleep(30 * time.Second)
		} else if response != nil {
			// If the device time is fetched successfully, log the success message
			t.Logf("%s device time fetched successfully: %v", deviceName, response)
			break
		} else {
			t.Logf("Error: %s device time response is nil despite no error", deviceName)
			time.Sleep(30 * time.Second)
		}

		if uint64(time.Since(startReboot).Minutes()) > maxRebootTime {
			t.Fatalf("Check %s boot time: got %v, want < %v", deviceName, time.Since(startReboot), maxRebootTime)
		}
	}
	t.Logf("%s gnoi ready time: %.2f minutes", deviceName, time.Since(startReboot).Minutes())
}

// reconnectGnmi attempts to reconnect to a device using the gNMI client and waits until the device is ready.
// It periodically checks the device's capabilities to determine its availability.
//
// Parameters:
//   - t: The testing context used for logging and error reporting.
//   - gnmiClient: The gNMI client used to communicate with the device.
//   - deviceName: The name of the device being checked.
//   - maxRebootTime: The maximum allowed reboot time (in minutes) before the function fails the test.
//
// Behavior:
//   - Logs the elapsed time since the reboot started.
//   - Continuously attempts to fetch the device's capabilities using the gNMI client.
//   - If the service is unavailable, waits for 30 seconds before retrying.
//   - If the device becomes available, logs a success message and exits the loop.
//   - If the elapsed time exceeds maxRebootTime, the function fails the test with a fatal error.
//   - Logs the total time taken for the device to become ready.
func reconnectGnmi(t *testing.T, gnmiClient gnmipb.GNMIClient, deviceName string, maxRebootTime uint64) {
	startReboot := time.Now()
	t.Logf("%s boot time: %.2f minutes", deviceName, time.Since(startReboot).Minutes())
	for {
		ctx := context.Background()
		// Example of fetching capabilities from the repository code base
		_, err := gnmiClient.Capabilities(ctx, &gnmipb.CapabilityRequest{})

		// Log the error if it occurs
		if err != nil {
			t.Logf("Error fetching %s device capabilities: %v", deviceName, err)
		}

		// Check if the error code indicates that the service is unavailable
		if status.Code(err) == codes.Unavailable {
			// If the service is unavailable, wait for 30 seconds before retrying
			t.Logf("%s service unavailable, retrying in 30 seconds...", deviceName)
			time.Sleep(30 * time.Second)
		} else if err == nil {
			// If the capabilities are fetched successfully, log the success message
			t.Logf("%s device capabilities fetched successfully", deviceName)
			break
		} else {
			t.Logf("Error: %s device capabilities response failed with error: %v", deviceName, err)
			time.Sleep(30 * time.Second)
		}

		if uint64(time.Since(startReboot).Minutes()) > maxRebootTime {
			t.Fatalf("Check %s boot time: got %v, want < %v", deviceName, time.Since(startReboot), maxRebootTime)
		}
	}
	t.Logf("%s gNMI ready time: %.2f minutes", deviceName, time.Since(startReboot).Minutes())
}

func reconnectGnsi(t *testing.T, gnsiClient ondatra_binding.GNSIClients, deviceName string, maxRebootTime uint64) {
	startReboot := time.Now()
	t.Logf("%s boot time: %.2f minutes", deviceName, time.Since(startReboot).Minutes())
	for {
		ctx := context.Background()
		// Example of fetching GNSI certificates from the repository code base
		_, err := gnsiClient.Certz().GetProfileList(ctx, &certzpb.GetProfileListRequest{})
		// Log the error if it occurs
		if err != nil {
			t.Logf("Error fetching %s GNSI certificates: %v", deviceName, err)
		}

		if status.Code(err) == codes.Unavailable {
			t.Logf("%s service unavailable, retrying in 30 seconds...", deviceName)
			time.Sleep(30 * time.Second)
		} else if err == nil {
			t.Logf("%s GNSI certificates fetched successfully", deviceName)
			break
		} else {
			t.Logf("Error: %s GNSI response failed with error: %v", deviceName, err)
			time.Sleep(30 * time.Second)
		}

		if uint64(time.Since(startReboot).Minutes()) > maxRebootTime {
			t.Fatalf("Check %s boot time: got %v, want < %v", deviceName, time.Since(startReboot), maxRebootTime)
		}
	}
	t.Logf("%s GNSI ready time: %.2f minutes", deviceName, time.Since(startReboot).Minutes())
}

func reconnectP4RT(t *testing.T, p4rtClient p4pb.P4RuntimeClient, deviceName string, maxRebootTime uint64) {
	startReboot := time.Now()
	t.Logf("%s boot time: %.2f minutes", deviceName, time.Since(startReboot).Minutes())
	for {
		ctx := context.Background()
		_, err := p4rtClient.Capabilities(ctx, &p4pb.CapabilitiesRequest{})

		if err != nil {
			t.Logf("Error fetching %s P4RT capabilities: %v", deviceName, err)
		}

		if status.Code(err) == codes.Unavailable {
			t.Logf("%s service unavailable, retrying in 30 seconds...", deviceName)
			time.Sleep(30 * time.Second)
		} else if err == nil {
			t.Logf("%s P4RT capabilities fetched successfully", deviceName)
			break
		} else {
			t.Logf("Error: %s P4RT response failed with error: %v", deviceName, err)
			time.Sleep(30 * time.Second)
		}

		if uint64(time.Since(startReboot).Minutes()) > maxRebootTime {
			t.Fatalf("Check %s boot time: got %v, want < %v", deviceName, time.Since(startReboot), maxRebootTime)
		}
	}
	t.Logf("%s P4RT ready time: %.2f minutes", deviceName, time.Since(startReboot).Minutes())
}

func reconnectGribi(t *testing.T, gribiClient gribis.GRIBIClient, deviceName string, maxRebootTime uint64) {
	startReboot := time.Now()
	t.Logf("%s boot time: %.2f minutes", deviceName, time.Since(startReboot).Minutes())
	for {
		ctx := context.Background()

		// Use GribiGet logic to validate the connection
		getReq := gribis.GetRequest{
			NetworkInstance: &gribis.GetRequest_All{},
			Aft:             gribis.AFTType_ALL,
		}
		getStream, err := gribiClient.Get(ctx, &getReq)

		if err != nil {
			t.Logf("Error fetching %s gRIBI capabilities: %v", deviceName, err)
		}

		if status.Code(err) == codes.Unavailable {
			t.Logf("%s service unavailable, retrying in 30 seconds...", deviceName)
			time.Sleep(30 * time.Second)
		} else if err == nil {
			_, recvErr := getStream.Recv()
			if recvErr == io.EOF {
				t.Logf("%s gRIBI capabilities fetched successfully", deviceName)
				break
			} else if recvErr != nil {
				t.Logf("Error: %s gRIBI response failed with error: %v", deviceName, recvErr)
				time.Sleep(30 * time.Second)
			}
		} else {
			t.Logf("Error: %s gRIBI response failed with error: %v", deviceName, err)
			time.Sleep(30 * time.Second)
		}

		if uint64(time.Since(startReboot).Minutes()) > maxRebootTime {
			t.Fatalf("Check %s boot time: got %v, want < %v", deviceName, time.Since(startReboot), maxRebootTime)
		}
	}
	t.Logf("%s gRIBI ready time: %.2f minutes", deviceName, time.Since(startReboot).Minutes())
}
