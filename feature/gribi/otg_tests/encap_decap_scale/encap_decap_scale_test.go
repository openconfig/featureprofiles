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

package encap_decap_scale_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	fpargs "github.com/openconfig/featureprofiles/internal/args"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/featureprofiles/internal/iputil"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/featureprofiles/internal/tescale"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Settings for configuring the baseline testbed with the test
// topology.
//
// The testbed consists of ate:port1 -> dut:port1
// and dut:port2 -> ate:port2.
// There are DefaultVRFIPv4NHCount SubInterfaces between dut:port2
// and ate:port2
//
//   - ate:port1 -> dut:port1 subnet 192.0.2.0/30
//   - ate:port2 -> dut:port2 DefaultVRFIPv4NHCount Sub interfaces, e.g.:
//   - ate:port2.0 -> dut:port2.0 VLAN-ID: 0 subnet 198.18.192.0/30
//   - ate:port2.1 -> dut:port2.1 VLAN-ID: 1 subnet 198.18.192.4/30
//   - ate:port2.2 -> dut:port2.2 VLAN-ID: 2 subnet 198.18.192.8/30
//   - ate:port2.i -> dut:port2.i VLAN-ID i subnet 198.18.x.(4*i)/30 (out of subnet 198.18.192.0/18)
const (
	ipv4PrefixLen           = 30 // ipv4PrefixLen is the ATE and DUT interface IP prefix length
	ipv6PrefixLen           = 126
	IPBlockDefaultVRF       = "198.18.128.0/18"
	IPBlockNonDefaultVRF    = "198.18.0.0/17"
	tunnelSrcIPv4Addr       = "198.51.100.99" // tunnelSrcIP represents Source IP of IPinIP Tunnel
	StaticMAC               = "00:1A:11:00:00:01"
	subifBaseIP             = "198.18.192.0"
	nextHopStartIndex       = 101 // set > 2 to avoid overlap with backup NH ids 1&2
	nextHopGroupStartIndex  = 101 // set > 2 to avoid overlap with backup NHG ids 1&2
	dscpEncapA1             = 10
	dscpEncapA2             = 18
	dscpEncapB1             = 20
	dscpEncapB2             = 28
	dscpEncapC1             = 30
	dscpEncapC2             = 38
	dscpEncapD1             = 40
	dscpEncapD2             = 48
	dscpEncapNoMatch        = 50
	ipv4OuterSrc111WithMask = "198.51.100.111/32"
	ipv4OuterSrc222WithMask = "198.51.100.222/32"
	ipv4OuterSrc222         = "198.51.100.222"
	magicMac                = "02:00:00:00:00:01"
	prot4                   = 4
	prot41                  = 41
	vrfPolW                 = "vrf_selection_policy_w"
	niDecapTeVrf            = "DECAP_TE_VRF"
	niEncapTeVrfA           = "ENCAP_TE_VRF_A"
	niEncapTeVrfB           = "ENCAP_TE_VRF_B"
	niEncapTeVrfC           = "ENCAP_TE_VRF_C"
	niEncapTeVrfD           = "ENCAP_TE_VRF_D"
	niTeVrf111              = "vrf_t"
	niTeVrf222              = "vrf_r"
	niDefault               = "DEFAULT"
	IPBlockEncapA           = "101.1.64.1/15"  // IPBlockEncapA represents the ipv4 entries in EncapVRFA
	IPBlockEncapB           = "101.5.64.1/15"  // IPBlockEncapB represents the ipv4 entries in EncapVRFB
	IPBlockEncapC           = "101.10.64.1/15" // IPBlockEncapC represents the ipv4 entries in EncapVRFC
	IPBlockEncapD           = "101.15.64.1/15" // IPBlockEncapD represents the ipv4 entries in EncapVRFD
	IPBlockDecap            = "102.0.0.1/15"   // IPBlockDecap represents the ipv4 entries in Decap VRF
	ipv4OuterSrc111         = "198.51.100.111"
	gribiIPv4EntryVRF1111   = "203.0.113.1"
	IPv6BlockEncapA         = "2001:DB8:0:1::/64"
	IPv6BlockEncapB         = "2001:DB8:1:1::/64"
	IPv6BlockEncapC         = "2001:DB8:2:1::/64"
	IPv6BlockEncapD         = "2001:DB8:3:1::/64"
	teVrf111TunnelCount     = 1600
	teVrf222TunnelCount     = 1600
	encapNhCount            = 1600
	encapNhgcount           = 200
	encapIPv4Count          = 5000
	encapIPv6Count          = 5000
	decapIPv4Count          = 48
	decapScale              = true
	tolerancePct            = 2
	seqIDBase               = 10
)

var (
	encapNhSize         = 8
	decapIPv4ScaleCount = 1000
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "dutPort1",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::192:0:2:1",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}

	atePort1 = attrs.Attributes{
		Name:    "atePort1",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::192:0:2:2",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
	lastNhIndex         int = 50000
	lastNhgIndex        int
	encapVrfAIPv4Enries = iputil.GenerateIPs(IPBlockEncapA, encapIPv4Count)
	encapVrfBIPv4Enries = iputil.GenerateIPs(IPBlockEncapB, encapIPv4Count)
	encapVrfCIPv4Enries = iputil.GenerateIPs(IPBlockEncapC, encapIPv4Count)
	encapVrfDIPv4Enries = iputil.GenerateIPs(IPBlockEncapD, encapIPv4Count)

	encapVrfAIPv6Enries = createIPv6Entries(IPv6BlockEncapA, encapIPv6Count)
	encapVrfBIPv6Enries = createIPv6Entries(IPv6BlockEncapB, encapIPv6Count)
	encapVrfCIPv6Enries = createIPv6Entries(IPv6BlockEncapC, encapIPv6Count)
	encapVrfDIPv6Enries = createIPv6Entries(IPv6BlockEncapD, encapIPv6Count)
)

// routesParam holds parameters required for provisioning
// gRIBI IP entries, next-hop-groups and next-hops
type routesParam struct {
	ipEntries     []string
	ipv6Entries   []string
	numUniqueNHs  int
	nextHops      []string
	nextHopVRF    string
	startNHIndex  int
	numUniqueNHGs int
	numNHPerNHG   int
	startNHGIndex int
	nextHopWeight []int
	backupNHG     int
	tunnelSrcIP   string
}

// Parameters needed to provision next-hop with interface reference + static MAC
type nextHopIntfRef struct {
	nextHopIPAddress string
	subintfIndex     uint32
	intfName         string
}

// Generate weights for next hops when assigning to a next-hop-group
// Weights are allocated such that there is no common divisor
func generateNextHopWeights(weightSum int, nextHopCount int) []int {
	weights := []int{}

	switch {
	case nextHopCount == 1:
		weights = append(weights, weightSum)
	case weightSum <= nextHopCount:
		for i := 0; i < nextHopCount; i++ {
			weights = append(weights, 1)
		}
	case nextHopCount == 2:
		weights = append(weights, 1, weightSum-1)
	default:
		weights = append(weights, 1, 2)
		rem := (weightSum - 1 - 2) % (nextHopCount - 2)
		weights = append(weights, rem+(weightSum-1-2)/(nextHopCount-2))
		for i := 1; i < (nextHopCount - 2); i++ {
			weights = append(weights, (weightSum-1-2)/(nextHopCount-2))
		}
	}
	return weights
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

// incrementIP increments the IPv4 address by i
func incrementIP(ip string, i int) string {
	ipAddr := net.ParseIP(ip)
	convIP := binary.BigEndian.Uint32(ipAddr.To4())
	convIP = convIP + uint32(i)
	newIP := make(net.IP, 4)
	binary.BigEndian.PutUint32(newIP, convIP)
	return newIP.String()
}

type policyFwRule struct {
	SeqID           uint32
	protocol        oc.UnionUint8
	dscpSet         []uint8
	sourceAddr      string
	decapNi         string
	postDecapNi     string
	decapFallbackNi string
}

func configureVrfSelectionPolicyW(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	d := &oc.Root{}
	dutPolFwdPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding()

	pfRule1 := &policyFwRule{SeqID: 1, protocol: 4, dscpSet: []uint8{dscpEncapA1, dscpEncapA2}, sourceAddr: ipv4OuterSrc222WithMask,
		decapNi: niDecapTeVrf, postDecapNi: niEncapTeVrfA, decapFallbackNi: niTeVrf222}
	pfRule2 := &policyFwRule{SeqID: 2, protocol: 41, dscpSet: []uint8{dscpEncapA1, dscpEncapA2}, sourceAddr: ipv4OuterSrc222WithMask,
		decapNi: niDecapTeVrf, postDecapNi: niEncapTeVrfA, decapFallbackNi: niTeVrf222}
	pfRule3 := &policyFwRule{SeqID: 3, protocol: 4, dscpSet: []uint8{dscpEncapA1, dscpEncapA2}, sourceAddr: ipv4OuterSrc111WithMask,
		decapNi: niDecapTeVrf, postDecapNi: niEncapTeVrfA, decapFallbackNi: niTeVrf111}
	pfRule4 := &policyFwRule{SeqID: 4, protocol: 41, dscpSet: []uint8{dscpEncapA1, dscpEncapA2}, sourceAddr: ipv4OuterSrc111WithMask,
		decapNi: niDecapTeVrf, postDecapNi: niEncapTeVrfA, decapFallbackNi: niTeVrf111}

	pfRule5 := &policyFwRule{SeqID: 5, protocol: 4, dscpSet: []uint8{dscpEncapB1, dscpEncapB2}, sourceAddr: ipv4OuterSrc222WithMask,
		decapNi: niDecapTeVrf, postDecapNi: niEncapTeVrfB, decapFallbackNi: niTeVrf222}
	pfRule6 := &policyFwRule{SeqID: 6, protocol: 41, dscpSet: []uint8{dscpEncapB1, dscpEncapB2}, sourceAddr: ipv4OuterSrc222WithMask,
		decapNi: niDecapTeVrf, postDecapNi: niEncapTeVrfB, decapFallbackNi: niTeVrf222}
	pfRule7 := &policyFwRule{SeqID: 7, protocol: 4, dscpSet: []uint8{dscpEncapB1, dscpEncapB2}, sourceAddr: ipv4OuterSrc111WithMask,
		decapNi: niDecapTeVrf, postDecapNi: niEncapTeVrfB, decapFallbackNi: niTeVrf111}
	pfRule8 := &policyFwRule{SeqID: 8, protocol: 41, dscpSet: []uint8{dscpEncapB1, dscpEncapB2}, sourceAddr: ipv4OuterSrc111WithMask,
		decapNi: niDecapTeVrf, postDecapNi: niEncapTeVrfB, decapFallbackNi: niTeVrf111}

	pfRule9 := &policyFwRule{SeqID: 9, protocol: 4, dscpSet: []uint8{dscpEncapC1, dscpEncapC2}, sourceAddr: ipv4OuterSrc222WithMask,
		decapNi: niDecapTeVrf, postDecapNi: niEncapTeVrfC, decapFallbackNi: niTeVrf222}
	pfRule10 := &policyFwRule{SeqID: 10, protocol: 41, dscpSet: []uint8{dscpEncapC1, dscpEncapC2}, sourceAddr: ipv4OuterSrc222WithMask,
		decapNi: niDecapTeVrf, postDecapNi: niEncapTeVrfC, decapFallbackNi: niTeVrf222}
	pfRule11 := &policyFwRule{SeqID: 11, protocol: 4, dscpSet: []uint8{dscpEncapC1, dscpEncapC2}, sourceAddr: ipv4OuterSrc111WithMask,
		decapNi: niDecapTeVrf, postDecapNi: niEncapTeVrfC, decapFallbackNi: niTeVrf111}
	pfRule12 := &policyFwRule{SeqID: 12, protocol: 41, dscpSet: []uint8{dscpEncapC1, dscpEncapC2}, sourceAddr: ipv4OuterSrc111WithMask,
		decapNi: niDecapTeVrf, postDecapNi: niEncapTeVrfC, decapFallbackNi: niTeVrf111}

	pfRule13 := &policyFwRule{SeqID: 13, protocol: 4, dscpSet: []uint8{dscpEncapD1, dscpEncapD2}, sourceAddr: ipv4OuterSrc222WithMask,
		decapNi: niDecapTeVrf, postDecapNi: niEncapTeVrfD, decapFallbackNi: niTeVrf222}
	pfRule14 := &policyFwRule{SeqID: 14, protocol: 41, dscpSet: []uint8{dscpEncapD1, dscpEncapD2}, sourceAddr: ipv4OuterSrc222WithMask,
		decapNi: niDecapTeVrf, postDecapNi: niEncapTeVrfD, decapFallbackNi: niTeVrf222}
	pfRule15 := &policyFwRule{SeqID: 15, protocol: 4, dscpSet: []uint8{dscpEncapD1, dscpEncapD2}, sourceAddr: ipv4OuterSrc111WithMask,
		decapNi: niDecapTeVrf, postDecapNi: niEncapTeVrfD, decapFallbackNi: niTeVrf111}
	pfRule16 := &policyFwRule{SeqID: 16, protocol: 41, dscpSet: []uint8{dscpEncapD1, dscpEncapD2}, sourceAddr: ipv4OuterSrc111WithMask,
		decapNi: niDecapTeVrf, postDecapNi: niEncapTeVrfD, decapFallbackNi: niTeVrf111}

	pfRule17 := &policyFwRule{SeqID: 17, protocol: 4, sourceAddr: ipv4OuterSrc222WithMask,
		decapNi: niDecapTeVrf, postDecapNi: niDefault, decapFallbackNi: niTeVrf222}
	pfRule18 := &policyFwRule{SeqID: 18, protocol: 41, sourceAddr: ipv4OuterSrc222WithMask,
		decapNi: niDecapTeVrf, postDecapNi: niDefault, decapFallbackNi: niTeVrf222}
	pfRule19 := &policyFwRule{SeqID: 19, protocol: 4, sourceAddr: ipv4OuterSrc111WithMask,
		decapNi: niDecapTeVrf, postDecapNi: niDefault, decapFallbackNi: niTeVrf111}
	pfRule20 := &policyFwRule{SeqID: 20, protocol: 41, sourceAddr: ipv4OuterSrc111WithMask,
		decapNi: niDecapTeVrf, postDecapNi: niDefault, decapFallbackNi: niTeVrf111}

	pfRuleList := []*policyFwRule{pfRule1, pfRule2, pfRule3, pfRule4, pfRule5, pfRule6,
		pfRule7, pfRule8, pfRule9, pfRule10, pfRule11, pfRule12, pfRule13, pfRule14,
		pfRule15, pfRule16, pfRule17, pfRule18, pfRule19, pfRule20}

	ni := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	ni.SetType(oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)
	niP := ni.GetOrCreatePolicyForwarding()
	niPf := niP.GetOrCreatePolicy(vrfPolW)
	niPf.SetType(oc.Policy_Type_VRF_SELECTION_POLICY)

	for _, pfRule := range pfRuleList {
		pfR := niPf.GetOrCreateRule(seqIDOffset(dut, pfRule.SeqID))
		pfRProtoIPv4 := pfR.GetOrCreateIpv4()
		pfRProtoIPv4.Protocol = oc.UnionUint8(pfRule.protocol)
		if pfRule.dscpSet != nil {
			pfRProtoIPv4.DscpSet = pfRule.dscpSet
		}
		pfRProtoIPv4.SourceAddress = ygot.String(pfRule.sourceAddr)
		pfRAction := pfR.GetOrCreateAction()
		pfRAction.DecapNetworkInstance = ygot.String(pfRule.decapNi)
		pfRAction.PostDecapNetworkInstance = ygot.String(pfRule.postDecapNi)
		pfRAction.DecapFallbackNetworkInstance = ygot.String(pfRule.decapFallbackNi)
	}

	if deviations.PfRequireMatchDefaultRule(dut) {
		pfR21 := niPf.GetOrCreateRule(seqIDOffset(dut, 21))
		pfR21.GetOrCreateL2().SetEthertype(oc.PacketMatchTypes_ETHERTYPE_ETHERTYPE_IPV4)
		pfRAction := pfR21.GetOrCreateAction()
		pfRAction.NetworkInstance = ygot.String(niDefault)

		pfR22 := niPf.GetOrCreateRule(seqIDOffset(dut, 22))
		pfR22.GetOrCreateL2().SetEthertype(oc.PacketMatchTypes_ETHERTYPE_ETHERTYPE_IPV6)
		pfRAction = pfR22.GetOrCreateAction()
		pfRAction.NetworkInstance = ygot.String(niDefault)
	} else {
		pfR := niPf.GetOrCreateRule(seqIDOffset(dut, 21))
		pfRAction := pfR.GetOrCreateAction()
		pfRAction.NetworkInstance = ygot.String(niDefault)
	}

	p1 := dut.Port(t, "port1")
	interfaceID := p1.Name()
	if deviations.InterfaceRefInterfaceIDFormat(dut) {
		interfaceID = interfaceID + ".0"
	}

	intf := niP.GetOrCreateInterface(interfaceID)
	intf.ApplyVrfSelectionPolicy = ygot.String(vrfPolW)
	intf.GetOrCreateInterfaceRef().Interface = ygot.String(p1.Name())
	intf.GetOrCreateInterfaceRef().Subinterface = ygot.Uint32(0)
	if deviations.InterfaceRefConfigUnsupported(dut) {
		intf.InterfaceRef = nil
	}
	gnmi.Replace(t, dut, dutPolFwdPath.Config(), niP)
}

// createIPv6Entries creates IPv6 Entries given the totalCount and starting prefix
func createIPv6Entries(startIP string, count uint64) []string {

	_, netCIDR, _ := net.ParseCIDR(startIP)
	netMask := binary.BigEndian.Uint64(netCIDR.Mask)
	maskSize, _ := netCIDR.Mask.Size()
	firstIP := binary.BigEndian.Uint64(netCIDR.IP)
	lastIP := (firstIP & netMask) | (netMask ^ 0xffffffff)
	entries := []string{}

	for i := firstIP; i <= lastIP; i++ {
		ipv6 := make(net.IP, 16)
		binary.BigEndian.PutUint64(ipv6, i)
		// make last byte non-zero
		p, _ := netip.ParsePrefix(fmt.Sprintf("%v/%d", ipv6, maskSize))
		entries = append(entries, p.Addr().Next().String())
		if uint64(len(entries)) >= count {
			break
		}
	}
	return entries
}

// pushEncapEntries pushes IP entries in a specified Encap VRFs and tunnel VRFs.
// The entries in the encap VRFs should point to NextHopGroups in the DEFAULT VRF.
// Inject 200 such NextHopGroups in the DEFAULT VRF. Each NextHopGroup should have
// 8 NextHops where each NextHop points to a tunnel in the TE_VRF_111.
// In addition, the weights specified in the NextHopGroup should be co-prime and the
// sum of the weights should be 16.
func pushEncapEntries(t *testing.T, tunnelIPs []string, args *testArgs) {
	vrfEntryParams := make(map[string]*routesParam)

	// Add 5k entries in ENCAP-VRF-A
	vrfEntryParams[niEncapTeVrfA] = &routesParam{
		ipEntries:     encapVrfAIPv4Enries,
		ipv6Entries:   encapVrfAIPv6Enries,
		numUniqueNHs:  encapNhgcount * encapNhSize,
		nextHops:      tunnelIPs,
		nextHopVRF:    niTeVrf111,
		startNHIndex:  lastNhIndex + 1,
		numUniqueNHGs: encapNhgcount,
		numNHPerNHG:   8,
		nextHopWeight: generateNextHopWeights(16, 8),
		startNHGIndex: lastNhgIndex + 1,
		tunnelSrcIP:   ipv4OuterSrc111,
	}

	lastNhIndex = vrfEntryParams[niEncapTeVrfA].startNHIndex + vrfEntryParams[niEncapTeVrfA].numUniqueNHs
	lastNhgIndex = vrfEntryParams[niEncapTeVrfA].startNHGIndex + vrfEntryParams[niEncapTeVrfA].numUniqueNHGs

	// Add 5k entries in ENCAP-VRF-B.
	vrfEntryParams[niEncapTeVrfB] = &routesParam{
		ipEntries:     encapVrfBIPv4Enries,
		ipv6Entries:   encapVrfBIPv6Enries,
		numUniqueNHs:  encapNhgcount * encapNhSize,
		nextHops:      tunnelIPs,
		nextHopVRF:    niTeVrf111,
		startNHIndex:  lastNhIndex + 1,
		numUniqueNHGs: encapNhgcount,
		numNHPerNHG:   8,
		nextHopWeight: generateNextHopWeights(16, 8),
		startNHGIndex: lastNhgIndex + 1,
		tunnelSrcIP:   ipv4OuterSrc222,
	}

	lastNhIndex = vrfEntryParams[niEncapTeVrfB].startNHIndex + vrfEntryParams[niEncapTeVrfB].numUniqueNHs
	lastNhgIndex = vrfEntryParams[niEncapTeVrfB].startNHGIndex + vrfEntryParams[niEncapTeVrfB].numUniqueNHGs

	// Add 5k entries in ENCAP-VRF-C
	vrfEntryParams[niEncapTeVrfC] = &routesParam{
		ipEntries:     encapVrfCIPv4Enries,
		ipv6Entries:   encapVrfCIPv6Enries,
		numUniqueNHs:  encapNhgcount * encapNhSize,
		nextHops:      tunnelIPs,
		nextHopVRF:    niTeVrf111,
		startNHIndex:  lastNhIndex + 1,
		numUniqueNHGs: encapNhgcount,
		numNHPerNHG:   8,
		nextHopWeight: generateNextHopWeights(16, 8),
		startNHGIndex: lastNhgIndex + 1,
		tunnelSrcIP:   ipv4OuterSrc111,
	}

	lastNhIndex = vrfEntryParams[niEncapTeVrfC].startNHIndex + vrfEntryParams[niEncapTeVrfC].numUniqueNHs
	lastNhgIndex = vrfEntryParams[niEncapTeVrfC].startNHGIndex + vrfEntryParams[niEncapTeVrfC].numUniqueNHGs

	// Add 5k entries in ENCAP-VRF-D
	vrfEntryParams[niEncapTeVrfD] = &routesParam{
		ipEntries:     encapVrfDIPv4Enries,
		ipv6Entries:   encapVrfDIPv6Enries,
		numUniqueNHs:  encapNhgcount * encapNhSize,
		nextHops:      tunnelIPs,
		nextHopVRF:    niTeVrf111,
		startNHIndex:  lastNhIndex + 1,
		numUniqueNHGs: encapNhgcount,
		numNHPerNHG:   8,
		nextHopWeight: generateNextHopWeights(16, 8),
		startNHGIndex: lastNhgIndex + 1,
		tunnelSrcIP:   ipv4OuterSrc222,
	}

	lastNhIndex = vrfEntryParams[niEncapTeVrfD].startNHIndex + vrfEntryParams[niEncapTeVrfD].numUniqueNHs
	lastNhgIndex = vrfEntryParams[niEncapTeVrfD].startNHGIndex + vrfEntryParams[niEncapTeVrfD].numUniqueNHGs

	for _, vrf := range []string{niEncapTeVrfA, niEncapTeVrfB, niEncapTeVrfC, niEncapTeVrfD} {
		t.Logf("installing v4 entries in %s", vrf)
		installEncapEntries(t, vrf, vrfEntryParams[vrf], args)
	}
}

func createAndSendTrafficFlows(t *testing.T, args *testArgs, decapEntries []string) {
	t.Helper()

	flow1 := createFlow(&flowArgs{flowName: "flow1", isInnHdrV4: true,
		InnHdrSrcIP: atePort1.IPv4, InnHdrDstIP: encapVrfAIPv4Enries, inHdrDscp: dscpEncapA1,
		outHdrSrcIP: ipv4OuterSrc111, outHdrDstIPs: decapEntries, outHdrDscp: dscpEncapA1,
	})

	flow2 := createFlow(&flowArgs{flowName: "flow2", isInnHdrV4: true,
		InnHdrSrcIP: atePort1.IPv4, InnHdrDstIP: encapVrfBIPv4Enries, inHdrDscp: dscpEncapB1,
		outHdrSrcIP: ipv4OuterSrc222, outHdrDstIPs: decapEntries, outHdrDscp: dscpEncapB1,
	})

	flow3 := createFlow(&flowArgs{flowName: "flow3", isInnHdrV4: true,
		InnHdrSrcIP: atePort1.IPv4, InnHdrDstIP: encapVrfCIPv4Enries, inHdrDscp: dscpEncapC1,
		outHdrSrcIP: ipv4OuterSrc111, outHdrDstIPs: decapEntries, outHdrDscp: dscpEncapC1,
	})

	flow4 := createFlow(&flowArgs{flowName: "flow4", isInnHdrV4: true,
		InnHdrSrcIP: atePort1.IPv4, InnHdrDstIP: encapVrfDIPv4Enries, inHdrDscp: dscpEncapD1,
		outHdrSrcIP: ipv4OuterSrc222, outHdrDstIPs: decapEntries, outHdrDscp: dscpEncapD1,
	})

	flow5 := createFlow(&flowArgs{flowName: "flow5", isInnHdrV4: false,
		InnHdrSrcIPv6: atePort1.IPv6, InnHdrDstIPv6: encapVrfAIPv6Enries, inHdrDscp: dscpEncapA2,
		outHdrSrcIP: ipv4OuterSrc111, outHdrDstIPs: decapEntries, outHdrDscp: dscpEncapA2,
	})

	flow6 := createFlow(&flowArgs{flowName: "flow6", isInnHdrV4: false,
		InnHdrSrcIPv6: atePort1.IPv6, InnHdrDstIPv6: encapVrfBIPv6Enries, inHdrDscp: dscpEncapB2,
		outHdrSrcIP: ipv4OuterSrc222, outHdrDstIPs: decapEntries, outHdrDscp: dscpEncapB2,
	})

	flow7 := createFlow(&flowArgs{flowName: "flow7", isInnHdrV4: false,
		InnHdrSrcIPv6: atePort1.IPv6, InnHdrDstIPv6: encapVrfCIPv6Enries, inHdrDscp: dscpEncapC2,
		outHdrSrcIP: ipv4OuterSrc111, outHdrDstIPs: decapEntries, outHdrDscp: dscpEncapC2,
	})

	flow8 := createFlow(&flowArgs{flowName: "flow8", isInnHdrV4: false,
		InnHdrSrcIPv6: atePort1.IPv6, InnHdrDstIPv6: encapVrfDIPv6Enries, inHdrDscp: dscpEncapD2,
		outHdrSrcIP: ipv4OuterSrc222, outHdrDstIPs: decapEntries, outHdrDscp: dscpEncapD2,
	})

	flowList := []gosnappi.Flow{flow1, flow2, flow3, flow4, flow5, flow6, flow7, flow8}

	args.top.Flows().Clear()
	for _, flow := range flowList {
		args.top.Flows().Append(flow)
	}

	args.ate.OTG().PushConfig(t, args.top)
	time.Sleep(30 * time.Second)
	args.ate.OTG().StartProtocols(t)
	// wait for glean adjacencies to be resolved
	time.Sleep(240 * time.Second)
	otgutils.WaitForARP(t, args.ate.OTG(), args.top, "IPv4")

	t.Logf("Starting traffic")
	args.ate.OTG().StartTraffic(t)
	time.Sleep(15 * time.Second)
	t.Logf("Stop traffic")
	args.ate.OTG().StopTraffic(t)

	flowNameList := []string{"flow1", "flow2", "flow3", "flow4", "flow5", "flow6", "flow7", "flow8"}

	otgutils.LogFlowMetrics(t, args.ate.OTG(), args.top)
	otgutils.LogPortMetrics(t, args.ate.OTG(), args.top)
	verifyTraffic(t, args, flowNameList)
}

func verifyTraffic(t *testing.T, args *testArgs, flowList []string) {
	t.Helper()
	for _, flowName := range flowList {
		t.Logf("Verifying flow metrics for the flow %s\n", flowName)
		recvMetric := gnmi.Get(t, args.ate.OTG(), gnmi.OTG().Flow(flowName).State())
		txPackets := recvMetric.GetCounters().GetOutPkts()
		rxPackets := recvMetric.GetCounters().GetInPkts()

		lostPackets := txPackets - rxPackets
		var lossPct uint64
		if txPackets != 0 {
			lossPct = lostPackets * 100 / txPackets
		} else {
			t.Errorf("Traffic stats are not correct %v", recvMetric)
		}
		if lossPct > tolerancePct {
			t.Errorf("Traffic Loss Pct for Flow: %s\n got %v, want 0", flowName, lossPct)
		} else {
			t.Logf("Traffic Test Passed!")
		}
	}
}

func pushDecapEntries(t *testing.T, args *testArgs) []string {
	decapIPBlocks := []string{}
	decapIPBlocks = append(decapIPBlocks, generateIPv4Subnets("102.51.100.1/22", 12)...)
	decapIPBlocks = append(decapIPBlocks, generateIPv4Subnets("107.51.105.1/24", 12)...)
	decapIPBlocks = append(decapIPBlocks, generateIPv4Subnets("112.51.110.1/26", 12)...)
	decapIPBlocks = append(decapIPBlocks, generateIPv4Subnets("117.51.115.1/28", 12)...)

	nhIndex := uint64(lastNhIndex)
	nhgIndex := uint64(lastNhgIndex)
	decapEntries := []string{}
	for i, ipBlock := range decapIPBlocks {
		entries := iputil.GenerateIPs(ipBlock, 1)
		decapEntries = append(decapEntries, entries...)
		nhgIndex = nhgIndex + 1
		nhIndex = nhIndex + 1
		installDecapEntry(t, args, nhIndex, nhgIndex, decapIPBlocks[i])
	}

	lastNhIndex = int(nhIndex) + 1
	lastNhgIndex = int(nhgIndex) + 1

	if err := awaitTimeout(args.ctx, args.client, t, 5*time.Minute); err != nil {
		t.Fatalf("Could not program entries via client, got err: %v", err)
	}

	t.Logf("Installed %v Decap VRF IPv4 entries with mixed prefix length", decapIPv4Count)
	return decapEntries
}

func pushDecapScaleEntries(t *testing.T, args *testArgs, decapEntries []string) {
	nhIndex := uint64(lastNhIndex)
	nhgIndex := uint64(lastNhgIndex)
	for i := 0; i < len(decapEntries); i++ {
		nhgIndex = nhgIndex + 1
		nhIndex = nhIndex + 1
		installDecapEntry(t, args, nhIndex, nhgIndex, decapEntries[i]+"/32")
	}

	lastNhIndex = int(nhIndex) + 1
	lastNhgIndex = int(nhgIndex) + 1

	if err := awaitTimeout(args.ctx, args.client, t, 5*time.Minute); err != nil {
		t.Fatalf("Could not program entries via client, got err: %v", err)
	}

	t.Logf("Installed %v Decap VRF IPv4 scale entries with prefix length 32", decapIPv4ScaleCount)
}

func installDecapEntry(t *testing.T, args *testArgs, nhIndex, nhgIndex uint64, prefix string) {
	args.client.Modify().AddEntry(t,
		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
			WithIndex(nhIndex).WithDecapsulateHeader(fluent.IPinIP).
			WithNextHopNetworkInstance(deviations.DefaultNetworkInstance(args.dut)),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
			WithID(nhgIndex).AddNextHop(nhIndex, 1),
		fluent.IPv4Entry().WithNetworkInstance(niDecapTeVrf).
			WithPrefix(prefix).WithNextHopGroup(nhgIndex).
			WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(args.dut)),
	)
}

type flowArgs struct {
	flowName      string
	outHdrSrcIP   string
	outHdrDstIPs  []string
	InnHdrSrcIP   string
	InnHdrDstIP   []string
	InnHdrSrcIPv6 string
	InnHdrDstIPv6 []string
	isInnHdrV4    bool
	outHdrDscp    uint32
	inHdrDscp     uint32
}

func createFlow(flowValues *flowArgs) gosnappi.Flow {
	rxNames := []string{}
	for i := 0; i < *fpargs.DefaultVRFIPv4NHCount; i++ {
		rxNames = append(rxNames, fmt.Sprintf(`dst%d.IPv4`, i))
	}

	flow := gosnappi.NewFlow().SetName(flowValues.flowName)
	flow.Metrics().SetEnable(true)
	flow.TxRx().Device().SetTxNames([]string{"atePort1.IPv4"}).SetRxNames(rxNames)
	flow.Size().SetFixed(512)
	flow.Rate().SetPps(100)
	flow.Duration().FixedPackets().SetPackets(1000)
	flow.Packet().Add().Ethernet().Src().SetValue(atePort1.MAC)
	// Outer IP header
	outerIPHdr := flow.Packet().Add().Ipv4()
	outerIPHdr.Src().SetValue(flowValues.outHdrSrcIP)
	outerIPHdr.Dst().SetValues(flowValues.outHdrDstIPs)
	outerIPHdr.Priority().Dscp().Phb().SetValue(flowValues.outHdrDscp)

	if flowValues.isInnHdrV4 {
		innerIPHdr := flow.Packet().Add().Ipv4()
		innerIPHdr.Src().SetValue(flowValues.InnHdrSrcIP)
		innerIPHdr.Dst().SetValues(flowValues.InnHdrDstIP)
		innerIPHdr.Priority().Dscp().Phb().SetValue(flowValues.inHdrDscp)
	} else {
		innerIpv6Hdr := flow.Packet().Add().Ipv6()
		innerIpv6Hdr.Src().SetValue(flowValues.InnHdrSrcIPv6)
		innerIpv6Hdr.Dst().SetValues(flowValues.InnHdrDstIPv6)
		innerIpv6Hdr.TrafficClass().SetValue(flowValues.inHdrDscp << 2)
	}
	return flow
}

// installEncapEntries installs IPv4/IPv6 Entries in the VRF with the given nextHops and nextHopGroups using gRIBI.
func installEncapEntries(t *testing.T, vrf string, routeParams *routesParam, args *testArgs) {
	// Provision next-hops
	nextHopIndices := []uint64{}
	for i := 0; i < routeParams.numUniqueNHs; i++ {
		index := uint64(routeParams.startNHIndex + i)
		args.client.Modify().AddEntry(t, fluent.NextHopEntry().
			WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
			WithIndex(index).
			WithIPinIP(routeParams.tunnelSrcIP, routeParams.nextHops[i%len(routeParams.nextHops)]).
			WithEncapsulateHeader(fluent.IPinIP).
			WithNextHopNetworkInstance(routeParams.nextHopVRF).
			WithElectionID(args.electionID.Low, args.electionID.High),
		)
		nextHopIndices = append(nextHopIndices, index)
	}
	if err := awaitTimeout(args.ctx, args.client, t, 5*time.Minute); err != nil {
		t.Fatalf("Could not program entries via client, got err: %v", err)
	}

	// Provision next-hop-groups
	nextHopGroupIndices := []uint64{}
	for i := 0; i < routeParams.numUniqueNHGs; i++ {
		index := uint64(routeParams.startNHGIndex + i)
		nhgEntry := fluent.NextHopGroupEntry().
			WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
			WithID(index).
			WithElectionID(args.electionID.Low, args.electionID.High)
		if routeParams.backupNHG != 0 {
			nhgEntry.WithBackupNHG(uint64(routeParams.backupNHG))
		}
		for j := 0; j < routeParams.numNHPerNHG; j++ {
			nhgEntry.AddNextHop(nextHopIndices[(i*routeParams.numNHPerNHG+j)%len(nextHopIndices)], uint64(routeParams.nextHopWeight[j]))
		}
		args.client.Modify().AddEntry(t, nhgEntry)
		nextHopGroupIndices = append(nextHopGroupIndices, index)
	}
	if err := awaitTimeout(args.ctx, args.client, t, 5*time.Minute); err != nil {
		t.Fatalf("Could not program entries via client, got err: %v", err)
	}

	// Provision ipv4 entries in VRF
	for i := range routeParams.ipEntries {
		args.client.Modify().AddEntry(t,
			fluent.IPv4Entry().
				WithPrefix(routeParams.ipEntries[i]+"/32").
				WithNetworkInstance(vrf).
				WithNextHopGroup(nextHopGroupIndices[i%len(nextHopGroupIndices)]).
				WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(args.dut)))
	}
	if err := awaitTimeout(args.ctx, args.client, t, 5*time.Minute); err != nil {
		t.Fatalf("Could not program entries via client, got err: %v", err)
	}
	t.Logf("Installed entries VRF %s - IPv4 entry count: %d, next-hop-group count: %d (index %d - %d), next-hop count: %d (index %d - %d)", vrf, len(routeParams.ipEntries), len(nextHopGroupIndices), nextHopGroupIndices[0], nextHopGroupIndices[len(nextHopGroupIndices)-1], len(nextHopIndices), nextHopIndices[0], nextHopIndices[len(nextHopIndices)-1])

	// Provision ipv6 entries in VRF
	for i := range routeParams.ipv6Entries {
		args.client.Modify().AddEntry(t,
			fluent.IPv6Entry().
				WithPrefix(routeParams.ipv6Entries[i]+"/128").
				WithNetworkInstance(vrf).
				WithNextHopGroup(nextHopGroupIndices[i%len(nextHopGroupIndices)]).
				WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(args.dut)))
	}
	if err := awaitTimeout(args.ctx, args.client, t, 5*time.Minute); err != nil {
		t.Fatalf("Could not program entries via client, got err: %v", err)
	}
	t.Logf("Installed entries VRF %s - IPv6 entry count: %d, next-hop-group count: %d (index %d - %d), next-hop count: %d (index %d - %d)", vrf, len(routeParams.ipv6Entries), len(nextHopGroupIndices), nextHopGroupIndices[0], nextHopGroupIndices[len(nextHopGroupIndices)-1], len(nextHopIndices), nextHopIndices[0], nextHopIndices[len(nextHopIndices)-1])
}

// configureDUT configures DUT interfaces and policy forwarding. Subinterfaces on DUT port2 are configured separately
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")
	d := &oc.Root{}
	dutOCRoot := gnmi.OC()

	vrfs := []string{deviations.DefaultNetworkInstance(dut), niDecapTeVrf,
		niEncapTeVrfA, niEncapTeVrfB, niEncapTeVrfC, niEncapTeVrfD, niTeVrf111, niTeVrf222}
	createVrf(t, dut, vrfs)

	// configure Ethernet interfaces first
	gnmi.Replace(t, dut, dutOCRoot.Interface(dp1.Name()).Config(), dutPort1.NewOCInterface(dp1.Name(), dut))
	configureInterfaceDUT(t, d, dut, dp2, "dst")

	// configure an L3 subinterface without vlan tagging under DUT port#1
	createSubifDUT(t, d, dut, dp1, 0, 0, dutPort1.IPv4, ipv4PrefixLen)
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, dp1.Name(), deviations.DefaultNetworkInstance(dut), 0)
	}
}

// createVrf takes in a list of VRF names and creates them on the target devices.
func createVrf(t *testing.T, dut *ondatra.DUTDevice, vrfs []string) {
	d := &oc.Root{}
	for _, vrf := range vrfs {
		if vrf != deviations.DefaultNetworkInstance(dut) {
			// configure non-default VRFs
			i := d.GetOrCreateNetworkInstance(vrf)
			i.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF
			gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(vrf).Config(), i)
		} else {
			// configure DEFAULT vrf
			i := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
			i.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE
			gnmi.Update(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Config(), i)
		}
		if deviations.ExplicitGRIBIUnderNetworkInstance(dut) {
			fptest.EnableGRIBIUnderNetworkInstance(t, dut, vrf)
		}
	}
}

// configureInterfaceDUT configures a single DUT port.
func configureInterfaceDUT(t *testing.T, d *oc.Root, dut *ondatra.DUTDevice, dutPort *ondatra.Port, desc string) {
	ifName := dutPort.Name()
	i := d.GetOrCreateInterface(ifName)
	i.Description = ygot.String(desc)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	if deviations.InterfaceEnabled(dut) {
		i.Enabled = ygot.Bool(true)
	}
	if deviations.ExplicitPortSpeed(dut) {
		i.GetOrCreateEthernet().PortSpeed = fptest.GetIfSpeed(t, dutPort)
	}
	gnmi.Replace(t, dut, gnmi.OC().Interface(ifName).Config(), i)
	t.Logf("DUT port %s configured", dutPort)
}

// createSubifDUT creates a single L3 subinterface
func createSubifDUT(t *testing.T, d *oc.Root, dut *ondatra.DUTDevice, dutPort *ondatra.Port, index uint32, vlanID uint16, ipv4Addr string, ipv4SubintfPrefixLen int) *oc.Interface_Subinterface {
	i := d.GetOrCreateInterface(dutPort.Name())
	s := i.GetOrCreateSubinterface(index)
	if vlanID != 0 {
		if deviations.DeprecatedVlanID(dut) {
			s.GetOrCreateVlan().VlanId = oc.UnionUint16(vlanID)
		} else {
			s.GetOrCreateVlan().GetOrCreateMatch().GetOrCreateSingleTagged().VlanId = ygot.Uint16(vlanID)
		}
	}
	s4 := s.GetOrCreateIpv4()
	a := s4.GetOrCreateAddress(ipv4Addr)
	a.PrefixLength = ygot.Uint8(uint8(ipv4SubintfPrefixLen))
	if deviations.InterfaceEnabled(dut) && !deviations.IPv4MissingEnabled(dut) {
		s4.Enabled = ygot.Bool(true)
	}
	return s
}

// configureDUTSubIfs configures DefaultVRFIPv4NHCount DUT subinterfaces on the target device
func configureDUTSubIfs(t *testing.T, dut *ondatra.DUTDevice, dutPort *ondatra.Port) []*nextHopIntfRef {
	d := &oc.Root{}
	nextHops := []*nextHopIntfRef{}
	batchConfig := &gnmi.SetBatch{}
	for i := 0; i < *fpargs.DefaultVRFIPv4NHCount; i++ {
		index := uint32(i)
		vlanID := uint16(i)
		if deviations.NoMixOfTaggedAndUntaggedSubinterfaces(dut) {
			vlanID = uint16(i) + 1
		}
		dutIPv4 := incrementIP(subifBaseIP, (4*i)+2)
		ateIPv4 := incrementIP(subifBaseIP, (4*i)+1)
		mac, err := incrementMAC(atePort1.MAC, i+1)
		if err != nil {
			t.Fatalf("failed to increment MAC: %v", err)
		}
		gnmi.BatchUpdate(batchConfig, gnmi.OC().Interface(dutPort.Name()).Subinterface(index).Config(), createSubifDUT(t, d, dut, dutPort, index, vlanID, dutIPv4, ipv4PrefixLen))
		gnmi.BatchUpdate(batchConfig, gnmi.OC().Interface(dutPort.Name()).Subinterface(index).Config(), createStaticArpEntries(dutPort.Name(), index, ateIPv4, mac))

		if deviations.ExplicitInterfaceInDefaultVRF(dut) {
			fptest.AssignToNetworkInstance(t, dut, dutPort.Name(), deviations.DefaultNetworkInstance(dut), index)
		}
		nextHops = append(nextHops, &nextHopIntfRef{
			nextHopIPAddress: ateIPv4,
			subintfIndex:     index,
			intfName:         dutPort.Name(),
		})
	}
	batchConfig.Set(t, dut)
	return nextHops
}

// configureATESubIfs configures *fpargs.DefaultVRFIPv4NHCount ATE subinterfaces on the target device
// It returns a slice of the corresponding ATE IPAddresses.
func configureATESubIfs(t *testing.T, top gosnappi.Config, atePort *ondatra.Port, dut *ondatra.DUTDevice) []string {
	nextHops := []string{}
	for i := 0; i < *fpargs.DefaultVRFIPv4NHCount; i++ {
		vlanID := uint16(i)
		if deviations.NoMixOfTaggedAndUntaggedSubinterfaces(dut) {
			vlanID = uint16(i) + 1
		}
		dutIPv4 := incrementIP(subifBaseIP, (4*i)+2)
		ateIPv4 := incrementIP(subifBaseIP, (4*i)+1)
		name := fmt.Sprintf(`dst%d`, i)
		mac, err := incrementMAC(atePort1.MAC, i+1)
		if err != nil {
			t.Fatalf("failed to increment MAC: %v", err)
		}
		configureATE(t, top, atePort, vlanID, name, mac, dutIPv4, ateIPv4)
		nextHops = append(nextHops, ateIPv4)
	}
	return nextHops
}

// configureATE configures a single ATE layer 3 interface.
func configureATE(t *testing.T, top gosnappi.Config, atePort *ondatra.Port, vlanID uint16, Name, MAC, dutIPv4, ateIPv4 string) {
	t.Helper()

	dev := top.Devices().Add().SetName(Name + ".Dev")
	eth := dev.Ethernets().Add().SetName(Name + ".Eth").SetMac(MAC)
	eth.Connection().SetPortName(atePort.ID())
	if vlanID != 0 {
		eth.Vlans().Add().SetName(Name).SetId(uint32(vlanID))
	}
	eth.Ipv4Addresses().Add().SetName(Name + ".IPv4").SetAddress(ateIPv4).SetGateway(dutIPv4).SetPrefix(uint32(atePort1.IPv4Len))
}

func configureATEPort1(t *testing.T, top gosnappi.Config) {
	t.Helper()

	port1 := top.Ports().Add().SetName("port1")
	iDut1Dev := top.Devices().Add().SetName(atePort1.Name)
	iDut1Eth := iDut1Dev.Ethernets().Add().SetName(atePort1.Name + ".Eth").SetMac(atePort1.MAC)
	iDut1Eth.Connection().SetPortName(port1.Name())
	iDut1Ipv4 := iDut1Eth.Ipv4Addresses().Add().SetName(atePort1.Name + ".IPv4")
	iDut1Ipv4.SetAddress(atePort1.IPv4).SetGateway(dutPort1.IPv4).SetPrefix(uint32(atePort1.IPv4Len))
	iDut1Ipv6 := iDut1Eth.Ipv6Addresses().Add().SetName(atePort1.Name + ".IPv6")
	iDut1Ipv6.SetAddress(atePort1.IPv6).SetGateway(dutPort1.IPv6).SetPrefix(uint32(atePort1.IPv6Len))
}

// awaitTimeout calls a fluent client Await, adding a timeout to the context.
func awaitTimeout(ctx context.Context, c *fluent.GRIBIClient, t testing.TB, timeout time.Duration) error {
	subctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return c.Await(subctx, t)
}

// testArgs holds the objects needed by a test case.
type testArgs struct {
	ctx        context.Context
	client     *fluent.GRIBIClient
	dut        *ondatra.DUTDevice
	ate        *ondatra.ATEDevice
	top        gosnappi.Config
	electionID gribi.Uint128
}

func TestGribiEncapDecapScaling(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	overrideScaleParams(dut)

	ate := ondatra.ATE(t, "ate")
	ctx := context.Background()
	gribic := dut.RawAPIs().GRIBI(t)
	ap2 := ate.Port(t, "port2")
	dp2 := dut.Port(t, "port2")

	top := gosnappi.NewConfig()
	top.Ports().Add().SetName(ate.Port(t, "port2").ID())

	configureDUT(t, dut)
	// configure DefaultVRFIPv4NHCount L3 subinterfaces under DUT port#2 and assign them to DEFAULT vrf
	// return slice containing interface name, subinterface index and ATE next hop IP that will be used for creating gRIBI next-hop entries
	subIntfNextHops := configureDUTSubIfs(t, dut, dp2)

	configureATEPort1(t, top)
	configureATESubIfs(t, top, ap2, dut)
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)

	// Connect gRIBI client to DUT referred to as gRIBI - using PRESERVE persistence and
	// SINGLE_PRIMARY mode, with FIB ACK requested. Specify gRIBI as the leader.
	client := fluent.NewClient()
	client.Connection().WithStub(gribic).WithPersistence().WithInitialElectionID(1, 0).
		WithRedundancyMode(fluent.ElectedPrimaryClient).WithFIBACK()

	client.Start(ctx, t)
	defer client.Stop(t)

	defer func() {
		// Flush all entries after test.
		if err := gribi.FlushAll(client); err != nil {
			t.Error(err)
		}
	}()

	client.StartSending(ctx, t)
	if err := awaitTimeout(ctx, client, t, time.Minute); err != nil {
		t.Fatalf("Await got error during session negotiation for client: %v", err)
	}
	eID := gribi.BecomeLeader(t, client)

	args := &testArgs{
		ctx:        ctx,
		client:     client,
		dut:        dut,
		ate:        ate,
		top:        top,
		electionID: eID,
	}

	// Apply vrf_selection_policy_w to DUT port-1.
	configureVrfSelectionPolicyW(t, dut)

	subIntfIPs := []string{}
	for _, subIntf := range subIntfNextHops {
		subIntfIPs = append(subIntfIPs, subIntf.nextHopIPAddress)
	}

	vrfConfigs := tescale.BuildVRFConfig(dut, subIntfIPs,
		tescale.Param{
			V4TunnelCount:         *fpargs.V4TunnelCount,
			V4TunnelNHGCount:      *fpargs.V4TunnelNHGCount,
			V4TunnelNHGSplitCount: *fpargs.V4TunnelNHGSplitCount,
			EgressNHGSplitCount:   *fpargs.EgressNHGSplitCount,
			V4ReEncapNHGCount:     *fpargs.V4ReEncapNHGCount,
		},
	)
	for _, vrfConfig := range vrfConfigs {
		// skip adding unwanted entries
		if vrfConfig.Name == "vrf_rd" {
			continue
		}
		entries := append(vrfConfig.NHs, vrfConfig.NHGs...)
		entries = append(entries, vrfConfig.V4Entries...)
		client.Modify().AddEntry(t, entries...)
		if err := awaitTimeout(ctx, client, t, 5*time.Minute); err != nil {
			t.Fatalf("Could not program entries, got err: %v", err)
		}
		t.Logf("Created %d NHs, %d NHGs, %d IPv4Entries in %s VRF", len(vrfConfig.NHs), len(vrfConfig.NHGs), len(vrfConfig.V4Entries), vrfConfig.Name)
	}

	defaultIpv4Entries := []string{}
	for _, v4Entry := range vrfConfigs[1].V4Entries {
		ep, _ := v4Entry.EntryProto()
		defaultIpv4Entries = append(defaultIpv4Entries, strings.Split(ep.GetIpv4().GetPrefix(), "/")[0])
	}

	// Inject 5000 IPv4Entry-ies and 5000 IPv6Entry-ies to each of the 4 encap VRFs.
	pushEncapEntries(t, defaultIpv4Entries, args)

	if !deviations.GribiDecapMixedPlenUnsupported(dut) {
		// Inject mixed length prefixes (48 entries) in the DECAP_TE_VRF.
		decapEntries := pushDecapEntries(t, args)
		// Send traffic and verify packets to DUT-1.
		createAndSendTrafficFlows(t, args, decapEntries)
		// Flush the DECAP_TE_VRF
		if _, err := gribi.Flush(client, args.electionID, niDecapTeVrf); err != nil {
			t.Error(err)
		}
		time.Sleep(240 * time.Second)
	}
	t.Log("installing scaled decap entries")
	// Install decapIPv4ScaleCount entries with fixed prefix length of /32 in DECAP_TE_VRF.
	decapScaleEntries := iputil.GenerateIPs(IPBlockDecap, decapIPv4ScaleCount)
	pushDecapScaleEntries(t, args, decapScaleEntries)
	// Send traffic and verify packets to DUT-1.
	createAndSendTrafficFlows(t, args, decapScaleEntries)
}

// createStaticArpEntries creates static ARP entries for the given subinterface.
func createStaticArpEntries(portName string, index uint32, ipv4Addr string, macAddr string) *oc.Interface_Subinterface {
	d := &oc.Root{}
	i := d.GetOrCreateInterface(portName)
	s := i.GetOrCreateSubinterface(index)
	s4 := s.GetOrCreateIpv4()
	n4 := s4.GetOrCreateNeighbor(ipv4Addr)
	n4.LinkLayerAddress = ygot.String(macAddr)
	return s
}

// generateIPv4Subnets creates IPv4 prefixes with a given seedBlock and subNets count
func generateIPv4Subnets(seedBlock string, subNets uint32) []string {

	_, netCIDR, _ := net.ParseCIDR(seedBlock)
	maskSize, _ := netCIDR.Mask.Size()
	incrSize := 0x00000001 << (32 - maskSize)
	firstIP := binary.BigEndian.Uint32(netCIDR.IP)
	entries := []string{}
	for i := firstIP; subNets > 0; subNets-- {
		ip := make(net.IP, 4)
		binary.BigEndian.PutUint32(ip, i)
		tip := netip.MustParsePrefix(fmt.Sprintf("%v/%d", ip, maskSize))
		if tip.Addr().IsValid() {
			entries = append(entries, tip.String())
		}
		i = i + uint32(incrSize)
	}
	return entries
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

// overrideScaleParams allows to override the default scale parameters based on the DUT vendor.
func overrideScaleParams(dut *ondatra.DUTDevice) {
	if deviations.OverrideDefaultNhScale(dut) {
		if dut.Vendor() == ondatra.CISCO {
			*fpargs.V4TunnelCount = 1024
			encapNhSize = 2
			decapIPv4ScaleCount = 400
		}
	}
}
