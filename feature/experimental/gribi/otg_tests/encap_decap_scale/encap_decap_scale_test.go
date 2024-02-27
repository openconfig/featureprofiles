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
	"errors"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	fpargs "github.com/openconfig/featureprofiles/internal/args"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/gribigo/chk"
	"github.com/openconfig/gribigo/constants"
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
	vrf1                    = "VRF-A"
	vrf2                    = "VRF-B"
	vrf3                    = "VRF-C"
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
	niTeVrf111              = "TE_VRF_111"
	niTeVrf222              = "TE_VRF_222"
	niDefault               = "DEFAULT"
	IPBlockEncapA           = "251.1.64.1/15"  // IPBlockEncapA represents the ipv4 entries in EncapVRFA
	IPBlockEncapB           = "251.5.64.1/15"  // IPBlockEncapB represents the ipv4 entries in EncapVRFB
	IPBlockEncapC           = "251.10.64.1/15" // IPBlockEncapC represents the ipv4 entries in EncapVRFC
	IPBlockEncapD           = "251.15.64.1/15" // IPBlockEncapD represents the ipv4 entries in EncapVRFD
	IPBlockDecap            = "252.0.0.1/15"   // IPBlockDecap represents the ipv4 entries in Decap VRF
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
	encapNhSize             = 8
	decapIPv4Count          = 48
	decapIPv4ScaleCount     = 3000
	decapScale              = true
	tolerancePct            = 2
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
	lastNhIndex         int
	lastNhgIndex        int
	encapVrfAIPv4Enries = createIPv4Entries(IPBlockEncapA)[0:encapIPv4Count]
	encapVrfBIPv4Enries = createIPv4Entries(IPBlockEncapB)[0:encapIPv4Count]
	encapVrfCIPv4Enries = createIPv4Entries(IPBlockEncapC)[0:encapIPv4Count]
	encapVrfDIPv4Enries = createIPv4Entries(IPBlockEncapD)[0:encapIPv4Count]

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
	nhDecapEncap  bool
	nhEncap       bool
	isIPv6        bool
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
	SeqId           uint32
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

	pfRule1 := &policyFwRule{SeqId: 1, protocol: 4, dscpSet: []uint8{dscpEncapA1, dscpEncapA2}, sourceAddr: ipv4OuterSrc222WithMask,
		decapNi: niDecapTeVrf, postDecapNi: niEncapTeVrfA, decapFallbackNi: niTeVrf222}
	pfRule2 := &policyFwRule{SeqId: 2, protocol: 41, dscpSet: []uint8{dscpEncapA1, dscpEncapA2}, sourceAddr: ipv4OuterSrc222WithMask,
		decapNi: niDecapTeVrf, postDecapNi: niEncapTeVrfA, decapFallbackNi: niTeVrf222}
	pfRule3 := &policyFwRule{SeqId: 3, protocol: 4, dscpSet: []uint8{dscpEncapA1, dscpEncapA2}, sourceAddr: ipv4OuterSrc111WithMask,
		decapNi: niDecapTeVrf, postDecapNi: niEncapTeVrfA, decapFallbackNi: niTeVrf111}
	pfRule4 := &policyFwRule{SeqId: 4, protocol: 41, dscpSet: []uint8{dscpEncapA1, dscpEncapA2}, sourceAddr: ipv4OuterSrc111WithMask,
		decapNi: niDecapTeVrf, postDecapNi: niEncapTeVrfA, decapFallbackNi: niTeVrf111}

	pfRule5 := &policyFwRule{SeqId: 5, protocol: 4, dscpSet: []uint8{dscpEncapB1, dscpEncapB2}, sourceAddr: ipv4OuterSrc222WithMask,
		decapNi: niDecapTeVrf, postDecapNi: niEncapTeVrfB, decapFallbackNi: niTeVrf222}
	pfRule6 := &policyFwRule{SeqId: 6, protocol: 41, dscpSet: []uint8{dscpEncapB1, dscpEncapB2}, sourceAddr: ipv4OuterSrc222WithMask,
		decapNi: niDecapTeVrf, postDecapNi: niEncapTeVrfB, decapFallbackNi: niTeVrf222}
	pfRule7 := &policyFwRule{SeqId: 7, protocol: 4, dscpSet: []uint8{dscpEncapB1, dscpEncapB2}, sourceAddr: ipv4OuterSrc111WithMask,
		decapNi: niDecapTeVrf, postDecapNi: niEncapTeVrfB, decapFallbackNi: niTeVrf111}
	pfRule8 := &policyFwRule{SeqId: 8, protocol: 41, dscpSet: []uint8{dscpEncapB1, dscpEncapB2}, sourceAddr: ipv4OuterSrc111WithMask,
		decapNi: niDecapTeVrf, postDecapNi: niEncapTeVrfB, decapFallbackNi: niTeVrf111}

	pfRule9 := &policyFwRule{SeqId: 9, protocol: 4, dscpSet: []uint8{dscpEncapC1, dscpEncapC2}, sourceAddr: ipv4OuterSrc222WithMask,
		decapNi: niDecapTeVrf, postDecapNi: niEncapTeVrfC, decapFallbackNi: niTeVrf222}
	pfRule10 := &policyFwRule{SeqId: 10, protocol: 41, dscpSet: []uint8{dscpEncapC1, dscpEncapC2}, sourceAddr: ipv4OuterSrc222WithMask,
		decapNi: niDecapTeVrf, postDecapNi: niEncapTeVrfC, decapFallbackNi: niTeVrf222}
	pfRule11 := &policyFwRule{SeqId: 11, protocol: 4, dscpSet: []uint8{dscpEncapC1, dscpEncapC2}, sourceAddr: ipv4OuterSrc111WithMask,
		decapNi: niDecapTeVrf, postDecapNi: niEncapTeVrfC, decapFallbackNi: niTeVrf111}
	pfRule12 := &policyFwRule{SeqId: 12, protocol: 41, dscpSet: []uint8{dscpEncapC1, dscpEncapC2}, sourceAddr: ipv4OuterSrc111WithMask,
		decapNi: niDecapTeVrf, postDecapNi: niEncapTeVrfC, decapFallbackNi: niTeVrf111}

	pfRule13 := &policyFwRule{SeqId: 13, protocol: 4, dscpSet: []uint8{dscpEncapD1, dscpEncapD2}, sourceAddr: ipv4OuterSrc222WithMask,
		decapNi: niDecapTeVrf, postDecapNi: niEncapTeVrfD, decapFallbackNi: niTeVrf222}
	pfRule14 := &policyFwRule{SeqId: 14, protocol: 41, dscpSet: []uint8{dscpEncapD1, dscpEncapD2}, sourceAddr: ipv4OuterSrc222WithMask,
		decapNi: niDecapTeVrf, postDecapNi: niEncapTeVrfD, decapFallbackNi: niTeVrf222}
	pfRule15 := &policyFwRule{SeqId: 15, protocol: 4, dscpSet: []uint8{dscpEncapD1, dscpEncapD2}, sourceAddr: ipv4OuterSrc111WithMask,
		decapNi: niDecapTeVrf, postDecapNi: niEncapTeVrfD, decapFallbackNi: niTeVrf111}
	pfRule16 := &policyFwRule{SeqId: 16, protocol: 41, dscpSet: []uint8{dscpEncapD1, dscpEncapD2}, sourceAddr: ipv4OuterSrc111WithMask,
		decapNi: niDecapTeVrf, postDecapNi: niEncapTeVrfD, decapFallbackNi: niTeVrf111}

	pfRule17 := &policyFwRule{SeqId: 17, protocol: 4, sourceAddr: ipv4OuterSrc222WithMask,
		decapNi: niDecapTeVrf, postDecapNi: niDefault, decapFallbackNi: niTeVrf222}
	pfRule18 := &policyFwRule{SeqId: 18, protocol: 41, sourceAddr: ipv4OuterSrc222WithMask,
		decapNi: niDecapTeVrf, postDecapNi: niDefault, decapFallbackNi: niTeVrf222}
	pfRule19 := &policyFwRule{SeqId: 19, protocol: 4, sourceAddr: ipv4OuterSrc111WithMask,
		decapNi: niDecapTeVrf, postDecapNi: niDefault, decapFallbackNi: niTeVrf111}
	pfRule20 := &policyFwRule{SeqId: 20, protocol: 41, sourceAddr: ipv4OuterSrc111WithMask,
		decapNi: niDecapTeVrf, postDecapNi: niDefault, decapFallbackNi: niTeVrf111}

	pfRuleList := []*policyFwRule{pfRule1, pfRule2, pfRule3, pfRule4, pfRule5, pfRule6,
		pfRule7, pfRule8, pfRule9, pfRule10, pfRule11, pfRule12, pfRule13, pfRule14,
		pfRule15, pfRule16, pfRule17, pfRule18, pfRule19, pfRule20}

	ni := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niP := ni.GetOrCreatePolicyForwarding()
	niPf := niP.GetOrCreatePolicy(vrfPolW)
	niPf.SetType(oc.Policy_Type_VRF_SELECTION_POLICY)

	for _, pfRule := range pfRuleList {
		pfR := niPf.GetOrCreateRule(pfRule.SeqId)
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
	pfR := niPf.GetOrCreateRule(21)
	pfRAction := pfR.GetOrCreateAction()
	pfRAction.NetworkInstance = ygot.String(niDefault)

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
	firstIP := binary.BigEndian.Uint64(netCIDR.IP)
	lastIP := (firstIP & netMask) | (netMask ^ 0xffffffff)
	entries := []string{}

	for i := firstIP; i <= lastIP; i++ {
		ipv6 := make(net.IP, 16)
		binary.BigEndian.PutUint64(ipv6, i)
		entries = append(entries, fmt.Sprint(ipv6))
		if uint64(len(entries)) > count {
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
func pushEncapEntries(t *testing.T, virtualVIPs []string, decapEncapVirtualIPs []string, args *testArgs) {

	vrfEntryParams := make(map[string]*routesParam)

	// Add 1600 TE_VRF111 tunnels
	vrfEntryParams[niTeVrf111] = &routesParam{
		ipEntries:     createIPv4Entries(IPBlockNonDefaultVRF)[0:teVrf111TunnelCount],
		numUniqueNHs:  teVrf111TunnelCount,
		nextHops:      virtualVIPs,
		nextHopVRF:    deviations.DefaultNetworkInstance(args.dut),
		startNHIndex:  lastNhIndex + 1,
		numUniqueNHGs: teVrf111TunnelCount,
		numNHPerNHG:   1,
		nextHopWeight: []int{1},
		startNHGIndex: lastNhgIndex + 1,
		nhDecapEncap:  false,
	}

	lastNhIndex = vrfEntryParams[niTeVrf111].startNHIndex + vrfEntryParams[niTeVrf111].numUniqueNHs
	lastNhgIndex = vrfEntryParams[niTeVrf111].startNHGIndex + vrfEntryParams[niTeVrf111].numUniqueNHGs

	installEntries(t, niTeVrf111, vrfEntryParams[niTeVrf111], args)

	// Add 5k entries in ENCAP-VRF-A
	vrfEntryParams[niEncapTeVrfA] = &routesParam{
		ipEntries:     encapVrfAIPv4Enries,
		numUniqueNHs:  encapNhgcount * encapNhSize,
		nextHops:      vrfEntryParams[niTeVrf111].ipEntries,
		nextHopVRF:    niTeVrf111,
		startNHIndex:  lastNhIndex + 1,
		numUniqueNHGs: encapNhgcount,
		numNHPerNHG:   8,
		nextHopWeight: generateNextHopWeights(16, 8),
		startNHGIndex: lastNhgIndex + 1,
		nhEncap:       true,
		tunnelSrcIP:   ipv4OuterSrc111,
	}

	lastNhIndex = vrfEntryParams[niEncapTeVrfA].startNHIndex + vrfEntryParams[niEncapTeVrfA].numUniqueNHs
	lastNhgIndex = vrfEntryParams[niEncapTeVrfA].startNHGIndex + vrfEntryParams[niEncapTeVrfA].numUniqueNHGs

	// Add 5k entries in ENCAP-VRF-B.
	vrfEntryParams[niEncapTeVrfB] = &routesParam{
		ipEntries:     encapVrfBIPv4Enries,
		numUniqueNHs:  encapNhgcount * encapNhSize,
		nextHops:      vrfEntryParams[niTeVrf111].ipEntries,
		nextHopVRF:    niTeVrf111,
		startNHIndex:  lastNhIndex + 1,
		numUniqueNHGs: encapNhgcount,
		numNHPerNHG:   8,
		nextHopWeight: generateNextHopWeights(16, 8),
		startNHGIndex: lastNhgIndex + 1,
		nhEncap:       true,
		tunnelSrcIP:   ipv4OuterSrc111,
	}

	lastNhIndex = vrfEntryParams[niEncapTeVrfB].startNHIndex + vrfEntryParams[niEncapTeVrfB].numUniqueNHs
	lastNhgIndex = vrfEntryParams[niEncapTeVrfB].startNHGIndex + vrfEntryParams[niEncapTeVrfB].numUniqueNHGs

	// Add 5k entries in ENCAP-VRF-C
	vrfEntryParams[niEncapTeVrfC] = &routesParam{
		ipEntries:     encapVrfCIPv4Enries,
		numUniqueNHs:  encapNhgcount * encapNhSize,
		nextHops:      vrfEntryParams[niTeVrf111].ipEntries,
		nextHopVRF:    niTeVrf111,
		startNHIndex:  lastNhIndex + 1,
		numUniqueNHGs: encapNhgcount,
		numNHPerNHG:   8,
		nextHopWeight: generateNextHopWeights(16, 8),
		startNHGIndex: lastNhgIndex + 1,
		nhEncap:       true,
		tunnelSrcIP:   ipv4OuterSrc111,
	}

	lastNhIndex = vrfEntryParams[niEncapTeVrfC].startNHIndex + vrfEntryParams[niEncapTeVrfC].numUniqueNHs
	lastNhgIndex = vrfEntryParams[niEncapTeVrfC].startNHGIndex + vrfEntryParams[niEncapTeVrfC].numUniqueNHGs

	// Add 5k entries in ENCAP-VRF-D
	vrfEntryParams[niEncapTeVrfD] = &routesParam{
		ipEntries:     encapVrfDIPv4Enries,
		numUniqueNHs:  encapNhgcount * encapNhSize,
		nextHops:      vrfEntryParams[niTeVrf111].ipEntries,
		nextHopVRF:    niTeVrf111,
		startNHIndex:  lastNhIndex + 1,
		numUniqueNHGs: encapNhgcount,
		numNHPerNHG:   8,
		nextHopWeight: generateNextHopWeights(16, 8),
		startNHGIndex: lastNhgIndex + 1,
		nhEncap:       true,
		tunnelSrcIP:   ipv4OuterSrc111,
	}

	lastNhIndex = vrfEntryParams[niEncapTeVrfD].startNHIndex + vrfEntryParams[niEncapTeVrfD].numUniqueNHs
	lastNhgIndex = vrfEntryParams[niEncapTeVrfD].startNHGIndex + vrfEntryParams[niEncapTeVrfD].numUniqueNHGs

	for _, vrf := range []string{niEncapTeVrfA, niEncapTeVrfB, niEncapTeVrfC, niEncapTeVrfD} {
		installEntries(t, vrf, vrfEntryParams[vrf], args)
	}

	// Add 5k IPv6 entries in ENCAP-VRF-A
	vrfEntryParams[niEncapTeVrfA] = &routesParam{
		ipv6Entries:   encapVrfAIPv6Enries,
		numUniqueNHs:  encapNhgcount * encapNhSize,
		nextHops:      vrfEntryParams[niTeVrf111].ipEntries,
		nextHopVRF:    niTeVrf111,
		startNHIndex:  lastNhIndex + 1,
		numUniqueNHGs: encapNhgcount,
		numNHPerNHG:   8,
		nextHopWeight: generateNextHopWeights(16, 8),
		startNHGIndex: lastNhgIndex + 1,
		nhEncap:       true,
		tunnelSrcIP:   ipv4OuterSrc111,
		isIPv6:        true,
	}

	lastNhIndex = vrfEntryParams[niEncapTeVrfA].startNHIndex + vrfEntryParams[niEncapTeVrfA].numUniqueNHs
	lastNhgIndex = vrfEntryParams[niEncapTeVrfA].startNHGIndex + vrfEntryParams[niEncapTeVrfA].numUniqueNHGs

	// Add 5k IPv6 entries in ENCAP-VRF-B.
	vrfEntryParams[niEncapTeVrfB] = &routesParam{
		ipv6Entries:   encapVrfBIPv6Enries,
		numUniqueNHs:  encapNhgcount * encapNhSize,
		nextHops:      vrfEntryParams[niTeVrf111].ipEntries,
		nextHopVRF:    niTeVrf111,
		startNHIndex:  lastNhIndex + 1,
		numUniqueNHGs: encapNhgcount,
		numNHPerNHG:   8,
		nextHopWeight: generateNextHopWeights(16, 8),
		startNHGIndex: lastNhgIndex + 1,
		nhEncap:       true,
		tunnelSrcIP:   ipv4OuterSrc111,
		isIPv6:        true,
	}

	lastNhIndex = vrfEntryParams[niEncapTeVrfB].startNHIndex + vrfEntryParams[niEncapTeVrfB].numUniqueNHs
	lastNhgIndex = vrfEntryParams[niEncapTeVrfB].startNHGIndex + vrfEntryParams[niEncapTeVrfB].numUniqueNHGs

	// Add 5k IPv6 entries in ENCAP-VRF-C.
	vrfEntryParams[niEncapTeVrfC] = &routesParam{
		ipv6Entries:   encapVrfCIPv6Enries,
		numUniqueNHs:  encapNhgcount * encapNhSize,
		nextHops:      vrfEntryParams[niTeVrf111].ipEntries,
		nextHopVRF:    niTeVrf111,
		startNHIndex:  lastNhIndex + 1,
		numUniqueNHGs: encapNhgcount,
		numNHPerNHG:   8,
		nextHopWeight: generateNextHopWeights(16, 8),
		startNHGIndex: lastNhgIndex + 1,
		nhEncap:       true,
		tunnelSrcIP:   ipv4OuterSrc111,
		isIPv6:        true,
	}

	lastNhIndex = vrfEntryParams[niEncapTeVrfC].startNHIndex + vrfEntryParams[niEncapTeVrfC].numUniqueNHs
	lastNhgIndex = vrfEntryParams[niEncapTeVrfC].startNHGIndex + vrfEntryParams[niEncapTeVrfC].numUniqueNHGs

	// Add 5k IPv6 entries in ENCAP-VRF-D.
	vrfEntryParams[niEncapTeVrfD] = &routesParam{
		ipv6Entries:   encapVrfDIPv6Enries,
		numUniqueNHs:  encapNhgcount * encapNhSize,
		nextHops:      vrfEntryParams[niTeVrf111].ipEntries,
		nextHopVRF:    niTeVrf111,
		startNHIndex:  lastNhIndex + 1,
		numUniqueNHGs: encapNhgcount,
		numNHPerNHG:   8,
		nextHopWeight: generateNextHopWeights(16, 8),
		startNHGIndex: lastNhgIndex + 1,
		nhEncap:       true,
		tunnelSrcIP:   ipv4OuterSrc222,
		isIPv6:        true,
	}

	lastNhIndex = vrfEntryParams[niEncapTeVrfD].startNHIndex + vrfEntryParams[niEncapTeVrfD].numUniqueNHs
	lastNhgIndex = vrfEntryParams[niEncapTeVrfD].startNHGIndex + vrfEntryParams[niEncapTeVrfD].numUniqueNHGs

	for _, vrf := range []string{niEncapTeVrfA, niEncapTeVrfB, niEncapTeVrfC, niEncapTeVrfD} {
		installEntries(t, vrf, vrfEntryParams[vrf], args)
	}
}

func createAndSendTrafficFlows(t *testing.T, args *testArgs, decapEntries []string, decapRouteCount uint32) {
	t.Helper()

	_, decapStartIP, _ := net.ParseCIDR(IPBlockDecap)
	flow1 := createFlow(&flowArgs{flowName: "flow1", isInnHdrV4: true, outHdrDstIPCount: decapRouteCount,
		InnHdrSrcIP: atePort1.IPv4, InnHdrDstIP: encapVrfAIPv4Enries, inHdrDscp: []uint32{dscpEncapA1},
		outHdrSrcIP: ipv4OuterSrc111, outHdrDstIP: decapStartIP.IP.String(), outHdrDscp: []uint32{dscpEncapA1},
	})

	flow2 := createFlow(&flowArgs{flowName: "flow2", isInnHdrV4: true, outHdrDstIPCount: decapRouteCount,
		InnHdrSrcIP: atePort1.IPv4, InnHdrDstIP: encapVrfBIPv4Enries, inHdrDscp: []uint32{dscpEncapB1},
		outHdrSrcIP: ipv4OuterSrc222, outHdrDstIP: decapStartIP.IP.String(), outHdrDscp: []uint32{dscpEncapB1},
	})

	flow3 := createFlow(&flowArgs{flowName: "flow3", isInnHdrV4: true, outHdrDstIPCount: decapRouteCount,
		InnHdrSrcIP: atePort1.IPv4, InnHdrDstIP: encapVrfCIPv4Enries, inHdrDscp: []uint32{dscpEncapC1},
		outHdrSrcIP: ipv4OuterSrc111, outHdrDstIP: decapStartIP.IP.String(), outHdrDscp: []uint32{dscpEncapC1},
	})

	flow4 := createFlow(&flowArgs{flowName: "flow4", isInnHdrV4: true, outHdrDstIPCount: decapRouteCount,
		InnHdrSrcIP: atePort1.IPv4, InnHdrDstIP: encapVrfDIPv4Enries, inHdrDscp: []uint32{dscpEncapD1},
		outHdrSrcIP: ipv4OuterSrc222, outHdrDstIP: decapStartIP.IP.String(), outHdrDscp: []uint32{dscpEncapD1},
	})

	// Below v6 flows will fail due to ixia issue.
	// https://github.com/open-traffic-generator/fp-testbed-juniper/issues/49

	flow5 := createFlow(&flowArgs{flowName: "flow5", isInnHdrV4: false, outHdrDstIPCount: decapRouteCount,
		InnHdrSrcIPv6: atePort1.IPv6, InnHdrDstIPv6: encapVrfAIPv6Enries, inHdrDscp: []uint32{dscpEncapA2},
		outHdrSrcIP: ipv4OuterSrc111, outHdrDstIP: decapStartIP.IP.String(), outHdrDscp: []uint32{dscpEncapA2},
	})

	flow6 := createFlow(&flowArgs{flowName: "flow6", isInnHdrV4: false, outHdrDstIPCount: decapRouteCount,
		InnHdrSrcIPv6: atePort1.IPv6, InnHdrDstIPv6: encapVrfBIPv6Enries, inHdrDscp: []uint32{dscpEncapB2},
		outHdrSrcIP: ipv4OuterSrc222, outHdrDstIP: decapStartIP.IP.String(), outHdrDscp: []uint32{dscpEncapB2},
	})

	flow7 := createFlow(&flowArgs{flowName: "flow7", isInnHdrV4: false, outHdrDstIPCount: decapRouteCount,
		InnHdrSrcIPv6: atePort1.IPv6, InnHdrDstIPv6: encapVrfCIPv6Enries, inHdrDscp: []uint32{dscpEncapC2},
		outHdrSrcIP: ipv4OuterSrc111, outHdrDstIP: decapStartIP.IP.String(), outHdrDscp: []uint32{dscpEncapC2},
	})

	flow8 := createFlow(&flowArgs{flowName: "flow8", isInnHdrV4: false, outHdrDstIPCount: decapRouteCount,
		InnHdrSrcIPv6: atePort1.IPv6, InnHdrDstIPv6: encapVrfDIPv6Enries, inHdrDscp: []uint32{dscpEncapD2},
		outHdrSrcIP: ipv4OuterSrc222, outHdrDstIP: decapStartIP.IP.String(), outHdrDscp: []uint32{dscpEncapD2},
	})

	flowList := []gosnappi.Flow{flow1, flow2, flow3, flow4, flow5, flow6, flow7, flow8}

	args.top.Flows().Clear()
	for _, flow := range flowList {
		args.top.Flows().Append(flow)
	}

	args.ate.OTG().PushConfig(t, args.top)
	time.Sleep(30 * time.Second)
	args.ate.OTG().StartProtocols(t)
	time.Sleep(30 * time.Second)

	t.Logf("Starting traffic")
	args.ate.OTG().StartTraffic(t)
	time.Sleep(15 * time.Second)
	t.Logf("Stop traffic")
	args.ate.OTG().StopTraffic(t)

	flowNameList := []string{"flow1", "flow2", "flow3", "flow4", "flow5", "flow6", "flow7", "flow8"}

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

func pushDecapEntries(t *testing.T, args *testArgs, decapEntries []string, decapIPv4Count int) {

	mask := []string{"22", "24", "26", "28"}
	j := 0
	nhIndex := uint64(lastNhIndex)
	nhgIndex := uint64(lastNhgIndex)
	for i := 0; i < decapIPv4Count; i++ {
		prefMask := mask[j]
		nhgIndex = nhgIndex + 1
		nhIndex = nhIndex + 1
		args.client.Modify().AddEntry(t,
			fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
				WithIndex(nhIndex).WithDecapsulateHeader(fluent.IPinIP).
				WithNextHopNetworkInstance(deviations.DefaultNetworkInstance(args.dut)),
			fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
				WithID(nhgIndex).AddNextHop(nhIndex, 1),
			fluent.IPv4Entry().WithNetworkInstance(niDecapTeVrf).
				WithPrefix(decapEntries[i]+"/"+prefMask).WithNextHopGroup(nhgIndex).
				WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(args.dut)),
		)
		if i == 12|24|36 {
			j = j + 1
		}
	}

	lastNhIndex = int(nhIndex) + 1
	lastNhgIndex = int(nhgIndex) + 1

	if err := awaitTimeout(args.ctx, args.client, t, time.Minute); err != nil {
		t.Fatalf("Could not program entries via client, got err: %v", err)
	}

	t.Logf("Installed %v Decap VRF IPv4 entries with mixed prefix length", decapIPv4Count)
}

func pushDecapScaleEntries(t *testing.T, args *testArgs, decapEntries []string, decapIPv4Count int) {

	nhIndex := uint64(lastNhIndex)
	nhgIndex := uint64(lastNhgIndex)
	for i := 0; i < decapIPv4Count; i++ {
		nhgIndex = nhgIndex + 1
		nhIndex = nhIndex + 1
		args.client.Modify().AddEntry(t,
			fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
				WithIndex(nhIndex).WithDecapsulateHeader(fluent.IPinIP).
				WithNextHopNetworkInstance(deviations.DefaultNetworkInstance(args.dut)),
			fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
				WithID(nhgIndex).AddNextHop(nhIndex, 1),
			fluent.IPv4Entry().WithNetworkInstance(niDecapTeVrf).
				WithPrefix(decapEntries[i]+"/"+"32").WithNextHopGroup(nhgIndex).
				WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(args.dut)),
		)
	}

	lastNhIndex = int(nhIndex) + 1
	lastNhgIndex = int(nhgIndex) + 1

	if err := awaitTimeout(args.ctx, args.client, t, time.Minute); err != nil {
		t.Fatalf("Could not program entries via client, got err: %v", err)
	}

	t.Logf("Installed %v Decap VRF IPv4 scale entries with prefix length 32", decapIPv4Count)
}

type flowArgs struct {
	flowName                 string
	outHdrSrcIP, outHdrDstIP string
	InnHdrSrcIP              string
	InnHdrDstIP              []string
	InnHdrSrcIPv6            string
	InnHdrDstIPv6            []string
	isInnHdrV4               bool
	outHdrDscp               []uint32
	inHdrDscp                []uint32
	outHdrDstIPCount         uint32
}

func createFlow(flowValues *flowArgs) gosnappi.Flow {
	flow := gosnappi.NewFlow().SetName(flowValues.flowName)
	flow.Metrics().SetEnable(true)
	flow.TxRx().Device().SetTxNames([]string{"atePort1.IPv4"})
	flow.TxRx().Device().SetRxNames([]string{"dst0.IPv4"})
	flow.Size().SetFixed(512)
	flow.Rate().SetPps(100)
	flow.Duration().Continuous()
	flow.Packet().Add().Ethernet().Src().SetValue(atePort1.MAC)
	// Outer IP header
	outerIpHdr := flow.Packet().Add().Ipv4()
	outerIpHdr.Src().SetValue(flowValues.outHdrSrcIP)
	outerIpHdr.Dst().Increment().SetStart(flowValues.outHdrDstIP).SetCount(flowValues.outHdrDstIPCount)
	outerIpHdr.Priority().Dscp().Phb().SetValues(flowValues.outHdrDscp)

	if flowValues.isInnHdrV4 {
		innerIpHdr := flow.Packet().Add().Ipv4()
		innerIpHdr.Src().SetValue(flowValues.InnHdrSrcIP)
		innerIpHdr.Dst().SetValues(flowValues.InnHdrDstIP)
		if len(flowValues.inHdrDscp) != 0 {
			innerIpHdr.Priority().Dscp().Phb().SetValues(flowValues.inHdrDscp)
		}
	} else {
		innerIpv6Hdr := flow.Packet().Add().Ipv6()
		innerIpv6Hdr.Src().SetValue(flowValues.InnHdrSrcIPv6)
		innerIpv6Hdr.Dst().SetValues(flowValues.InnHdrDstIPv6)
		if len(flowValues.inHdrDscp) != 0 {
			innerIpv6Hdr.TrafficClass().SetValues(flowValues.inHdrDscp)
		}
	}
	return flow
}

// pushIPv4Entries pushes gRIBI IPv4 entries in a specified VRF, with corresponding NHs and NHGs in the default NI
func pushIPv4Entries(t *testing.T, virtualVIPs []string, decapEncapVirtualIPs []string, args *testArgs) {

	// install backup NHGs/NHs
	// NHG {ID #1} --> NH {ID #1, network-instance: VRF-C}
	args.client.Modify().AddEntry(t,
		fluent.NextHopEntry().
			WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
			WithIndex(uint64(1)).
			WithNextHopNetworkInstance(vrf3).
			WithElectionID(args.electionID.Low, args.electionID.High))

	args.client.Modify().AddEntry(t,
		fluent.NextHopGroupEntry().
			WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
			WithID(uint64(1)).
			AddNextHop(uint64(1), uint64(1)).
			WithElectionID(args.electionID.Low, args.electionID.High))

	// NHG {ID #2} --> NH {ID #2, decap, network-instance: DEFAULT}
	args.client.Modify().AddEntry(t,
		fluent.NextHopEntry().
			WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
			WithIndex(uint64(2)).
			WithDecapsulateHeader(fluent.IPinIP).
			WithNextHopNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
			WithElectionID(args.electionID.Low, args.electionID.High))

	args.client.Modify().AddEntry(t,
		fluent.NextHopGroupEntry().
			WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
			WithID(uint64(2)).
			AddNextHop(uint64(2), uint64(1)).
			WithElectionID(args.electionID.Low, args.electionID.High))

	// provision non-default VRF gRIBI entries, and associated NHGs, NHs in default instance
	vrfEntryParams := make(map[string]*routesParam)
	vrfEntryParams[vrf1] = &routesParam{
		ipEntries:     createIPv4Entries(IPBlockNonDefaultVRF)[0:*fpargs.NonDefaultVRFIPv4Count],
		numUniqueNHs:  *fpargs.NonDefaultVRFIPv4NHGCount * *fpargs.NonDefaultVRFIPv4NHSize,
		nextHops:      virtualVIPs,
		nextHopVRF:    deviations.DefaultNetworkInstance(args.dut),
		startNHIndex:  nextHopStartIndex + *fpargs.DefaultVRFIPv4NHCount,
		numUniqueNHGs: *fpargs.NonDefaultVRFIPv4NHGCount,
		numNHPerNHG:   *fpargs.NonDefaultVRFIPv4NHSize,
		nextHopWeight: generateNextHopWeights(*fpargs.NonDefaultVRFIPv4NHGWeightSum, *fpargs.NonDefaultVRFIPv4NHSize),
		startNHGIndex: nextHopGroupStartIndex + *fpargs.DefaultVRFIPv4Count,
		backupNHG:     1,
		nhDecapEncap:  false,
	}
	vrfEntryParams[vrf2] = &routesParam{
		ipEntries:     createIPv4Entries(IPBlockNonDefaultVRF)[0:*fpargs.NonDefaultVRFIPv4Count],
		numUniqueNHs:  len(decapEncapVirtualIPs),
		nextHops:      decapEncapVirtualIPs,
		nextHopVRF:    deviations.DefaultNetworkInstance(args.dut),
		startNHIndex:  vrfEntryParams[vrf1].startNHIndex + vrfEntryParams[vrf1].numUniqueNHs,
		numUniqueNHGs: *fpargs.NonDefaultVRFIPv4NHGCount,
		numNHPerNHG:   *fpargs.NonDefaultVRFIPv4NHSize,
		nextHopWeight: generateNextHopWeights(*fpargs.NonDefaultVRFIPv4NHGWeightSum, *fpargs.NonDefaultVRFIPv4NHSize),
		startNHGIndex: vrfEntryParams[vrf1].startNHGIndex + vrfEntryParams[vrf1].numUniqueNHGs,
		backupNHG:     2,
		nhDecapEncap:  false,
	}
	vrfEntryParams[vrf3] = &routesParam{
		ipEntries:     createIPv4Entries(IPBlockNonDefaultVRF)[0:*fpargs.NonDefaultVRFIPv4Count],
		numUniqueNHs:  *fpargs.DecapEncapCount,
		nextHops:      createIPv4Entries(IPBlockNonDefaultVRF)[0:*fpargs.DecapEncapCount],
		nextHopVRF:    vrf2,
		startNHIndex:  vrfEntryParams[vrf2].startNHIndex + vrfEntryParams[vrf2].numUniqueNHs,
		numUniqueNHGs: *fpargs.DecapEncapCount,
		numNHPerNHG:   1,
		nextHopWeight: []int{1},
		startNHGIndex: vrfEntryParams[vrf2].startNHGIndex + vrfEntryParams[vrf2].numUniqueNHGs,
		backupNHG:     2,
		nhDecapEncap:  true,
	}

	lastNhIndex = vrfEntryParams[vrf3].startNHIndex + vrfEntryParams[vrf3].numUniqueNHs
	lastNhgIndex = vrfEntryParams[vrf3].startNHGIndex + vrfEntryParams[vrf3].numUniqueNHGs

	for _, vrf := range []string{vrf1, vrf2, vrf3} {
		installEntries(t, vrf, vrfEntryParams[vrf], args)
	}
}

// createIPv4Entries creates IPv4 Entries given the totalCount and starting prefix
func createIPv4Entries(startIP string) []string {

	_, netCIDR, _ := net.ParseCIDR(startIP)
	netMask := binary.BigEndian.Uint32(netCIDR.Mask)
	firstIP := binary.BigEndian.Uint32(netCIDR.IP)
	lastIP := (firstIP & netMask) | (netMask ^ 0xffffffff)
	entries := []string{}
	for i := firstIP; i <= lastIP; i++ {
		ip := make(net.IP, 4)
		binary.BigEndian.PutUint32(ip, i)
		entries = append(entries, fmt.Sprint(ip))
	}
	return entries
}

// installEntries installs IPv4/IPv6 Entries in the VRF with the given nextHops and nextHopGroups using gRIBI.
func installEntries(t *testing.T, vrf string, routeParams *routesParam, args *testArgs) {

	// Provision next-hops
	nextHopIndices := []uint64{}
	for i := 0; i < routeParams.numUniqueNHs; i++ {
		index := uint64(routeParams.startNHIndex + i)
		if routeParams.nhDecapEncap {
			args.client.Modify().AddEntry(t,
				fluent.NextHopEntry().
					WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
					WithIndex(index).
					WithIPinIP(tunnelSrcIPv4Addr, routeParams.nextHops[i%len(routeParams.nextHops)]).
					WithDecapsulateHeader(fluent.IPinIP).
					WithEncapsulateHeader(fluent.IPinIP).
					WithNextHopNetworkInstance(routeParams.nextHopVRF).
					WithElectionID(args.electionID.Low, args.electionID.High))
		} else if routeParams.nhEncap {
			args.client.Modify().AddEntry(t,
				fluent.NextHopEntry().
					WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
					WithIndex(index).
					WithIPinIP(routeParams.tunnelSrcIP, routeParams.nextHops[i%len(routeParams.nextHops)]).
					WithEncapsulateHeader(fluent.IPinIP).
					WithNextHopNetworkInstance(routeParams.nextHopVRF).
					WithElectionID(args.electionID.Low, args.electionID.High))
		} else {
			args.client.Modify().AddEntry(t,
				fluent.NextHopEntry().
					WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
					WithIndex(index).
					WithIPAddress(routeParams.nextHops[i%len(routeParams.nextHops)]).
					WithElectionID(args.electionID.Low, args.electionID.High))
		}
		nextHopIndices = append(nextHopIndices, index)
	}
	if err := awaitTimeout(args.ctx, args.client, t, time.Minute); err != nil {
		t.Fatalf("Could not program entries via client, got err: %v", err)
	}

	// Provision next-hop-groups
	nextHopGroupIndices := []uint64{}
	for i := 0; i < routeParams.numUniqueNHGs; i++ {
		index := uint64(routeParams.startNHGIndex + i)
		nhgEntry := fluent.NextHopGroupEntry().
			WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
			WithID(index).
			WithBackupNHG(uint64(routeParams.backupNHG)).
			WithElectionID(args.electionID.Low, args.electionID.High)
		for j := 0; j < routeParams.numNHPerNHG; j++ {
			nhgEntry.AddNextHop(nextHopIndices[(i*routeParams.numNHPerNHG+j)%len(nextHopIndices)], uint64(routeParams.nextHopWeight[j]))
		}
		args.client.Modify().AddEntry(t, nhgEntry)
		nextHopGroupIndices = append(nextHopGroupIndices, index)
	}
	if err := awaitTimeout(args.ctx, args.client, t, time.Minute); err != nil {
		t.Fatalf("Could not program entries via client, got err: %v", err)
	}

	// Provision ip entires in VRF
	if routeParams.isIPv6 {
		for i := range routeParams.ipv6Entries {
			args.client.Modify().AddEntry(t,
				fluent.IPv6Entry().
					WithPrefix(routeParams.ipv6Entries[i]+"/128").
					WithNetworkInstance(vrf).
					WithNextHopGroup(nextHopGroupIndices[i%len(nextHopGroupIndices)]).
					WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(args.dut)))
		}
	} else {
		for i := range routeParams.ipEntries {
			args.client.Modify().AddEntry(t,
				fluent.IPv4Entry().
					WithPrefix(routeParams.ipEntries[i]+"/32").
					WithNetworkInstance(vrf).
					WithNextHopGroup(nextHopGroupIndices[i%len(nextHopGroupIndices)]).
					WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(args.dut)))
		}
	}
	if err := awaitTimeout(args.ctx, args.client, t, time.Minute); err != nil {
		t.Fatalf("Could not program entries via client, got err: %v", err)
	}

	if routeParams.isIPv6 {
		t.Logf("Installed entries VRF %s - IPv6 entry count: %d, next-hop-group count: %d (index %d - %d), next-hop count: %d (index %d - %d)", vrf, len(routeParams.ipv6Entries), len(nextHopGroupIndices), nextHopGroupIndices[0], nextHopGroupIndices[len(nextHopGroupIndices)-1], len(nextHopIndices), nextHopIndices[0], nextHopIndices[len(nextHopIndices)-1])
	} else {
		t.Logf("Installed entries VRF %s - IPv4 entry count: %d, next-hop-group count: %d (index %d - %d), next-hop count: %d (index %d - %d)", vrf, len(routeParams.ipEntries), len(nextHopGroupIndices), nextHopGroupIndices[0], nextHopGroupIndices[len(nextHopGroupIndices)-1], len(nextHopIndices), nextHopIndices[0], nextHopIndices[len(nextHopIndices)-1])
	}
}

// pushDefaultEntries installs gRIBI next-hops, next-hop-groups and IP entries for base route resolution
func pushDefaultEntries(t *testing.T, args *testArgs, nextHops []*nextHopIntfRef) ([]string, []string) {

	primaryNextHopIfs := nextHops[0:*fpargs.DefaultVRFPrimarySubifCount]
	decapEncapNextHopIfs := nextHops[*fpargs.DefaultVRFPrimarySubifCount:]

	primaryNextHopIndices := []uint64{}
	for i := 0; i < (*fpargs.DefaultVRFIPv4NHCount - len(decapEncapNextHopIfs)); i++ {
		index := uint64(nextHopStartIndex + i)
		mac, _ := incrementMAC(StaticMAC, i)
		args.client.Modify().AddEntry(t,
			fluent.NextHopEntry().
				WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
				WithIndex(index).
				WithSubinterfaceRef(primaryNextHopIfs[i%len(primaryNextHopIfs)].intfName, uint64(primaryNextHopIfs[i%len(primaryNextHopIfs)].subintfIndex)).
				WithMacAddress(mac).
				WithElectionID(args.electionID.Low, args.electionID.High))
		primaryNextHopIndices = append(primaryNextHopIndices, index)
	}
	if err := awaitTimeout(args.ctx, args.client, t, time.Minute); err != nil {
		t.Fatalf("Could not program entries via client, got err: %v", err)
	}
	t.Logf("Installed %s VRF \"primary\" next-hop count: %d (index %d - %d)", deviations.DefaultNetworkInstance(args.dut), len(primaryNextHopIndices), primaryNextHopIndices[0], primaryNextHopIndices[len(primaryNextHopIndices)-1])

	decapEncapNextHopIndices := []uint64{}
	for i := 0; i < len(decapEncapNextHopIfs); i++ {
		index := primaryNextHopIndices[len(primaryNextHopIndices)-1] + uint64(1+i)
		mac, _ := incrementMAC(StaticMAC, len(primaryNextHopIndices)+i)
		args.client.Modify().AddEntry(t,
			fluent.NextHopEntry().
				WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
				WithIndex(index).
				WithSubinterfaceRef(decapEncapNextHopIfs[i%len(decapEncapNextHopIfs)].intfName, uint64(decapEncapNextHopIfs[i%len(decapEncapNextHopIfs)].subintfIndex)).
				WithMacAddress(mac).
				WithElectionID(args.electionID.Low, args.electionID.High))
		decapEncapNextHopIndices = append(decapEncapNextHopIndices, index)
	}
	if err := awaitTimeout(args.ctx, args.client, t, time.Minute); err != nil {
		t.Fatalf("Could not program entries via client, got err: %v", err)
	}
	t.Logf("Installed %s VRF \"decap/encap\" next-hop count: %d (index %d - %d)", deviations.DefaultNetworkInstance(args.dut), len(decapEncapNextHopIndices), decapEncapNextHopIndices[0], decapEncapNextHopIndices[len(decapEncapNextHopIndices)-1])

	primaryNextHopGroupIndices := []uint64{}
	for i := 0; i < (*fpargs.DefaultVRFIPv4Count - len(decapEncapNextHopIfs)); i++ {
		index := uint64(nextHopGroupStartIndex + i)
		nhgEntry := fluent.NextHopGroupEntry().
			WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
			WithID(index).
			WithElectionID(args.electionID.Low, args.electionID.High)
		for j := 0; j < *fpargs.DefaultVRFIPv4NHSize; j++ {
			nhgEntry.AddNextHop(primaryNextHopIndices[(i**fpargs.DefaultVRFIPv4NHSize+j)%len(primaryNextHopIndices)], uint64(generateNextHopWeights(*fpargs.DefaultVRFIPv4NHGWeightSum, *fpargs.DefaultVRFIPv4NHSize)[j]))
		}
		args.client.Modify().AddEntry(t, nhgEntry)
		primaryNextHopGroupIndices = append(primaryNextHopGroupIndices, index)
	}
	if err := awaitTimeout(args.ctx, args.client, t, time.Minute); err != nil {
		t.Fatalf("Could not program entries via client, got err: %v", err)
	}
	t.Logf("Installed %s VRF \"primary\" next-hop-group count: %d (index %d - %d)", deviations.DefaultNetworkInstance(args.dut), len(primaryNextHopGroupIndices), primaryNextHopGroupIndices[0], primaryNextHopGroupIndices[len(primaryNextHopGroupIndices)-1])

	decapEncapNextHopGroupIndices := []uint64{}
	for i := 0; i < len(decapEncapNextHopIfs); i++ {
		index := uint64(nextHopGroupStartIndex + len(primaryNextHopGroupIndices) + i)
		nhgEntry := fluent.NextHopGroupEntry().
			WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
			WithID(index).
			WithElectionID(args.electionID.Low, args.electionID.High)
		for j := 0; j < *fpargs.DefaultVRFIPv4NHSize; j++ {
			nhgEntry.AddNextHop(decapEncapNextHopIndices[(i**fpargs.DefaultVRFIPv4NHSize+j)%len(decapEncapNextHopIndices)], uint64(generateNextHopWeights(*fpargs.DefaultVRFIPv4NHGWeightSum, *fpargs.DefaultVRFIPv4NHSize)[j]))
		}
		args.client.Modify().AddEntry(t, nhgEntry)
		decapEncapNextHopGroupIndices = append(decapEncapNextHopGroupIndices, index)
	}
	if err := awaitTimeout(args.ctx, args.client, t, time.Minute); err != nil {
		t.Fatalf("Could not program entries via client, got err: %v", err)
	}
	t.Logf("Installed %s VRF \"decap/encap\" next-hop-group count: %d (index %d - %d)", deviations.DefaultNetworkInstance(args.dut), len(decapEncapNextHopGroupIndices), decapEncapNextHopGroupIndices[0], decapEncapNextHopGroupIndices[len(decapEncapNextHopGroupIndices)-1])

	time.Sleep(time.Minute)
	virtualVIPs := createIPv4Entries(IPBlockDefaultVRF)
	primaryVirtualVIPs := virtualVIPs[0:(*fpargs.DefaultVRFIPv4Count - len(decapEncapNextHopIfs))]
	decapEncapVirtualIPs := virtualVIPs[(*fpargs.DefaultVRFIPv4Count - len(decapEncapNextHopIfs)):*fpargs.DefaultVRFIPv4Count]

	// install IPv4 entries for primary forwarding cases (referenced from vrf1)
	for i := range primaryVirtualVIPs {
		args.client.Modify().AddEntry(t,
			fluent.IPv4Entry().
				WithPrefix(primaryVirtualVIPs[i]+"/32").
				WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
				WithNextHopGroup(primaryNextHopGroupIndices[i%len(primaryNextHopGroupIndices)]).
				WithElectionID(args.electionID.Low, args.electionID.High))
	}
	if err := awaitTimeout(args.ctx, args.client, t, time.Minute); err != nil {
		t.Fatalf("Could not program entries via client, got err: %v", err)
	}

	for i := range primaryVirtualVIPs {
		chk.HasResult(t, args.client.Results(t),
			fluent.OperationResult().
				WithIPv4Operation(primaryVirtualVIPs[i]+"/32").
				WithOperationType(constants.Add).
				WithProgrammingResult(fluent.InstalledInFIB).
				AsResult(),
			chk.IgnoreOperationID(),
		)
	}
	t.Logf("Installed %s VRF \"primary\" IPv4 entries, %s/32 to %s/32", deviations.DefaultNetworkInstance(args.dut), primaryVirtualVIPs[0], primaryVirtualVIPs[len(primaryVirtualVIPs)-1])

	// install IPv4 entries for decap/encap cases (referenced from vrf2)
	for i := range decapEncapVirtualIPs {
		args.client.Modify().AddEntry(t,
			fluent.IPv4Entry().
				WithPrefix(decapEncapVirtualIPs[i]+"/32").
				WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
				WithNextHopGroup(decapEncapNextHopGroupIndices[i%len(decapEncapNextHopGroupIndices)]).
				WithElectionID(args.electionID.Low, args.electionID.High))
	}
	if err := awaitTimeout(args.ctx, args.client, t, time.Minute); err != nil {
		t.Fatalf("Could not program entries via client, got err: %v", err)
	}

	for i := range decapEncapVirtualIPs {
		chk.HasResult(t, args.client.Results(t),
			fluent.OperationResult().
				WithIPv4Operation(decapEncapVirtualIPs[i]+"/32").
				WithOperationType(constants.Add).
				WithProgrammingResult(fluent.InstalledInFIB).
				AsResult(),
			chk.IgnoreOperationID(),
		)
	}
	t.Logf("Installed %s VRF \"decap/encap\" IPv4 entries, %s/32 to %s/32", deviations.DefaultNetworkInstance(args.dut), decapEncapVirtualIPs[0], decapEncapVirtualIPs[len(decapEncapVirtualIPs)-1])
	t.Log("Pushed gRIBI default entries")

	return primaryVirtualVIPs, decapEncapVirtualIPs
}

// configureDUT configures DUT interfaces and policy forwarding. Subinterfaces on DUT port2 are configured separately
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")
	d := &oc.Root{}
	dutOCRoot := gnmi.OC()

	vrfs := []string{deviations.DefaultNetworkInstance(dut), vrf1, vrf2, vrf3, niDecapTeVrf,
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
	for _, vrf := range vrfs {
		if vrf != deviations.DefaultNetworkInstance(dut) {
			// configure non-default VRFs
			d := &oc.Root{}
			i := d.GetOrCreateNetworkInstance(vrf)
			i.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF
			gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(vrf).Config(), i)
		} else {
			// configure DEFAULT vrf
			dutConfNIPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut))
			gnmi.Replace(t, dut, dutConfNIPath.Type().Config(), oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)

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
		vlanID := uint16(i) + 1
		dutIPv4 := incrementIP(subifBaseIP, (4*i)+2)
		ateIPv4 := incrementIP(subifBaseIP, (4*i)+1)
		gnmi.BatchUpdate(batchConfig, gnmi.OC().Interface(dutPort.Name()).Subinterface(index).Config(), createSubifDUT(t, d, dut, dutPort, index, vlanID, dutIPv4, ipv4PrefixLen))
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
		vlanID := uint16(i) + 1
		dutIPv4 := incrementIP(subifBaseIP, (4*i)+2)
		ateIPv4 := incrementIP(subifBaseIP, (4*i)+1)
		name := fmt.Sprintf(`dst%d`, i)
		mac, _ := incrementMAC(atePort1.MAC, i+1)
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

// checkInputArgs verifies that gribi scaling input args are set
func checkInputArgs(t *testing.T) error {
	t.Logf("Input arg DefaultVRFIPv4Count           = %d", *fpargs.DefaultVRFIPv4Count)
	t.Logf("Input arg DefaultVRFIPv4NHSize          = %d", *fpargs.DefaultVRFIPv4NHSize)
	t.Logf("Input arg DefaultVRFIPv4NHGWeightSum    = %d", *fpargs.DefaultVRFIPv4NHGWeightSum)
	t.Logf("Input arg DefaultVRFIPv4NHCount         = %d", *fpargs.DefaultVRFIPv4NHCount)
	t.Logf("Input arg NonDefaultVRFIPv4Count        = %d", *fpargs.NonDefaultVRFIPv4Count)
	t.Logf("Input arg NonDefaultVRFIPv4NHGCount     = %d", *fpargs.NonDefaultVRFIPv4NHGCount)
	t.Logf("Input arg NonDefaultVRFIPv4NHSize       = %d", *fpargs.NonDefaultVRFIPv4NHSize)
	t.Logf("Input arg NonDefaultVRFIPv4NHGWeightSum = %d", *fpargs.NonDefaultVRFIPv4NHGWeightSum)
	t.Logf("Input arg DecapEncapCount               = %d", *fpargs.DecapEncapCount)
	t.Logf("Input arg DefaultVRFPrimarySubifCount   = %d", *fpargs.DefaultVRFPrimarySubifCount)

	if *fpargs.DefaultVRFIPv4Count == -1 {
		return errors.New("Input argument DefaultVRFIPv4Count is not set")
	}
	if *fpargs.DefaultVRFIPv4NHSize == -1 {
		return errors.New("Input argument DefaultVRFIPv4NHSize is not set")
	}
	if *fpargs.DefaultVRFIPv4NHGWeightSum == -1 {
		return errors.New("Input argument DefaultVRFIPv4NHGWeightSum is not set")
	}
	if *fpargs.DefaultVRFIPv4NHCount == -1 {
		return errors.New("Input argument DefaultVRFIPv4NHCount is not set")
	}
	if *fpargs.NonDefaultVRFIPv4Count == -1 {
		return errors.New("Input argument NonDefaultVRFIPv4Count is not set")
	}
	if *fpargs.NonDefaultVRFIPv4NHGCount == -1 {
		return errors.New("Input argument NonDefaultVRFIPv4NHGCount is not set")
	}
	if *fpargs.NonDefaultVRFIPv4NHSize == -1 {
		return errors.New("Input argument NonDefaultVRFIPv4NHSize is not set")
	}
	if *fpargs.NonDefaultVRFIPv4NHGWeightSum == -1 {
		return errors.New("Input argument NonDefaultVRFIPv4NHGWeightSum is not set")
	}
	if *fpargs.DecapEncapCount == -1 {
		return errors.New("Input argument DecapEncapCount is not set")
	}
	if *fpargs.DefaultVRFPrimarySubifCount == -1 {
		return errors.New("Input argument DefaultVRFPrimarySubifCount is not set")
	}
	return nil
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

	if err := checkInputArgs(t); err != nil {
		t.Fatalf("Input arguments not set: %v", err)
	}

	dut := ondatra.DUT(t, "dut")
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

	// pushDefaultEntries installs gRIBI next-hops, next-hop-groups and IP entries for base route resolution
	// defaultIpv4Entries are ipv4 entries used for deriving nextHops for IPBlockNonDefaultVRF
	defaultIpv4Entries, decapEncapDefaultIpv4Entries := pushDefaultEntries(t, args, subIntfNextHops)

	// pushIPv4Entries builds the recursive scaling topology
	pushIPv4Entries(t, defaultIpv4Entries, decapEncapDefaultIpv4Entries, args)

	// Apply vrf_selection_policy_w to DUT port-1.
	configureVrfSelectionPolicyW(t, dut)

	// Inject 5000 IPv4Entry-ies and 5000 IPv6Entry-ies to each of the 4 encap VRFs.
	pushEncapEntries(t, defaultIpv4Entries, decapEncapDefaultIpv4Entries, args)

	if !deviations.GribiDecapMixedPlenUnsupported(dut) {
		// Inject mixed length prefixes (48 entries) in the DECAP_TE_VRF.
		decapEntries := createIPv4Entries(IPBlockDecap)[0:decapIPv4Count]
		pushDecapEntries(t, args, decapEntries, decapIPv4Count)
		// Send traffic and verify packets to DUT-1.
		createAndSendTrafficFlows(t, args, decapEntries, decapIPv4Count)
		// Flush the DECAP_TE_VRF
		if _, err := gribi.Flush(client, args.electionID, niDecapTeVrf); err != nil {
			t.Error(err)
		}
	}

	// Install decapIPv4ScaleCount entries with fixed prefix length of /32 in DECAP_TE_VRF.
	decapScaleEntries := createIPv4Entries(IPBlockDecap)[0:decapIPv4ScaleCount]
	pushDecapScaleEntries(t, args, decapScaleEntries, decapIPv4ScaleCount)

	// Send traffic and verify packets to DUT-1.
	createAndSendTrafficFlows(t, args, decapScaleEntries, decapIPv4ScaleCount)
}
