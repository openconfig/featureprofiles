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
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/gribigo/chk"
	"github.com/openconfig/gribigo/client"
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
// There are subIntfCount SubInterfaces between dut:port2
// and ate:port2
//
//   - ate:port1 -> dut:port1 subnet 192.0.2.0/30
//   - ate:port2 -> dut:port2 subIntfCount Sub interfaces, e.g.:
//   - ate:port2.0 -> dut:port2.0 VLAN-ID: 0 subnet 198.18.192.0/30
//   - ate:port2.1 -> dut:port2.1 VLAN-ID: 1 subnet 198.18.192.4/30
//   - ate:port2.2 -> dut:port2.2 VLAN-ID: 2 subnet 198.18.192.8/30
//   - ate:port2.i -> dut:port2.i VLAN-ID i subnet 198.18.x.(4*i)/30 (out of subnet 198.18.192.0/18)
const (
	ipv4PrefixLen = 30 // ipv4PrefixLen is the ATE and DUT interface IP prefix length
	ipv6PrefixLen = 126

	vrfPolW     = "vrf_selection_policy_w"
	vrf1        = "VRF-A"
	vrf2        = "VRF-B"
	vrf3        = "VRF-C"
	decapTeVRF  = "DECAP_TE_VRF"
	encapTeVRFA = "ENCAP_TE_VRF_A"
	encapTeVRFB = "ENCAP_TE_VRF_B"
	encapTeVRFC = "ENCAP_TE_VRF_C"
	encapTeVRFD = "ENCAP_TE_VRF_D"
	teVRF111    = "TE_VRF_111"
	teVRF222    = "TE_VRF_222"

	ipBlockDefaultVRF    = "198.18.128.0/18"
	ipBlockNonDefaultVRF = "198.18.0.0/17"
	ipBlockTEVRF         = "100.72.0.0/16"
	ipBlockEncap         = "100.64.0.0/16"
	ipBlockDecap         = "100.80.0.0/16"
	ipv6BlockEncap       = "2001:DB8:0:1::/64"

	tunnelSrcIPv4Addr       = "198.51.100.99" // tunnelSrcIP represents Source IP of IPinIP Tunnel
	subifBaseIP             = "198.18.192.0"
	ipv4OuterSrc111         = "198.51.100.111"
	ipv4OuterSrc222         = "198.51.100.222"
	ipv4OuterSrc111WithMask = "198.51.100.111/32"
	ipv4OuterSrc222WithMask = "198.51.100.222/32"
	magicMAC                = "02:00:00:00:00:01"
	magicIP                 = "192.168.1.1"

	nextHopStartIndex      = 101 // set > 2 to avoid overlap with backup NH ids 1&2
	nextHopGroupStartIndex = 101 // set > 2 to avoid overlap with backup NHG ids 1&2

	dscpEncapA1      = 10
	dscpEncapA2      = 18
	dscpEncapB1      = 20
	dscpEncapB2      = 28
	dscpEncapC1      = 30
	dscpEncapC2      = 38
	dscpEncapD1      = 40
	dscpEncapD2      = 48
	dscpEncapNoMatch = 50

	// number of subinterfaces to configure in port2
	subIntfCount = 128

	// number of recursive ip addresses pointing to port2 sub interfaces
	// half of the virtual IPs would be for the primary path and half would be for the decap encap path.
	numVirtualIPsDefaultVRF = 2048

	// number of recursive ip addresses to install in each of VRF-A, VRF-B, VRF-C
	// VRF-A and VRF-B would install the same virtual IPs and VRF-C would install virtual IPs that recursively point to viirtualIPs installed in VRF-A and VRF-B
	numVirtualIPsNonDefaultVRF = 1024

	// Number of IP Address to install per encap vrf
	perEncapVRFIPCount = 5000

	// Number of NHGs per encap vrf. The total encap vrf NHGs would be 4 times this number.
	perEncapVRFNHGCount = 50

	nhWeightSum = 16

	encapNHsPerNHG = 8

	decapIPv4Count = 48

	decapIPv4ScaleCount = 5000

	tolerancePct = 2
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
	encapVRFIPv4Entries = createIPv4Entries(ipBlockEncap)
	encapVRFIPv6Entries = createIPv6Entries(ipv6BlockEncap, 4*perEncapVRFIPCount)
)

// routesParam holds parameters required for provisioning
// gRIBI IP entries, next-hop-groups and next-hops
type routesParam struct {
	ipEntries     []string
	ipv6Entries   []string
	nextHops      []string
	nextHopVRF    string
	numUniqueNHs  int
	numUniqueNHGs int
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
	seqID           uint32
	protocol        oc.UnionUint8
	dscpSet         []uint8
	sourceAddr      string
	decapNi         string
	postDecapNi     string
	decapFallbackNi string
}

func configureVRFSelectionPolicyW(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	d := &oc.Root{}
	dutPolFwdPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding()

	pfRule1 := &policyFwRule{seqID: 1, protocol: 4, dscpSet: []uint8{dscpEncapA1, dscpEncapA2}, sourceAddr: ipv4OuterSrc222WithMask,
		decapNi: decapTeVRF, postDecapNi: encapTeVRFA, decapFallbackNi: teVRF222}
	pfRule2 := &policyFwRule{seqID: 2, protocol: 41, dscpSet: []uint8{dscpEncapA1, dscpEncapA2}, sourceAddr: ipv4OuterSrc222WithMask,
		decapNi: decapTeVRF, postDecapNi: encapTeVRFA, decapFallbackNi: teVRF222}
	pfRule3 := &policyFwRule{seqID: 3, protocol: 4, dscpSet: []uint8{dscpEncapA1, dscpEncapA2}, sourceAddr: ipv4OuterSrc111WithMask,
		decapNi: decapTeVRF, postDecapNi: encapTeVRFA, decapFallbackNi: teVRF111}
	pfRule4 := &policyFwRule{seqID: 4, protocol: 41, dscpSet: []uint8{dscpEncapA1, dscpEncapA2}, sourceAddr: ipv4OuterSrc111WithMask,
		decapNi: decapTeVRF, postDecapNi: encapTeVRFA, decapFallbackNi: teVRF111}

	pfRule5 := &policyFwRule{seqID: 5, protocol: 4, dscpSet: []uint8{dscpEncapB1, dscpEncapB2}, sourceAddr: ipv4OuterSrc222WithMask,
		decapNi: decapTeVRF, postDecapNi: encapTeVRFB, decapFallbackNi: teVRF222}
	pfRule6 := &policyFwRule{seqID: 6, protocol: 41, dscpSet: []uint8{dscpEncapB1, dscpEncapB2}, sourceAddr: ipv4OuterSrc222WithMask,
		decapNi: decapTeVRF, postDecapNi: encapTeVRFB, decapFallbackNi: teVRF222}
	pfRule7 := &policyFwRule{seqID: 7, protocol: 4, dscpSet: []uint8{dscpEncapB1, dscpEncapB2}, sourceAddr: ipv4OuterSrc111WithMask,
		decapNi: decapTeVRF, postDecapNi: encapTeVRFB, decapFallbackNi: teVRF111}
	pfRule8 := &policyFwRule{seqID: 8, protocol: 41, dscpSet: []uint8{dscpEncapB1, dscpEncapB2}, sourceAddr: ipv4OuterSrc111WithMask,
		decapNi: decapTeVRF, postDecapNi: encapTeVRFB, decapFallbackNi: teVRF111}

	pfRule9 := &policyFwRule{seqID: 9, protocol: 4, dscpSet: []uint8{dscpEncapC1, dscpEncapC2}, sourceAddr: ipv4OuterSrc222WithMask,
		decapNi: decapTeVRF, postDecapNi: encapTeVRFC, decapFallbackNi: teVRF222}
	pfRule10 := &policyFwRule{seqID: 10, protocol: 41, dscpSet: []uint8{dscpEncapC1, dscpEncapC2}, sourceAddr: ipv4OuterSrc222WithMask,
		decapNi: decapTeVRF, postDecapNi: encapTeVRFC, decapFallbackNi: teVRF222}
	pfRule11 := &policyFwRule{seqID: 11, protocol: 4, dscpSet: []uint8{dscpEncapC1, dscpEncapC2}, sourceAddr: ipv4OuterSrc111WithMask,
		decapNi: decapTeVRF, postDecapNi: encapTeVRFC, decapFallbackNi: teVRF111}
	pfRule12 := &policyFwRule{seqID: 12, protocol: 41, dscpSet: []uint8{dscpEncapC1, dscpEncapC2}, sourceAddr: ipv4OuterSrc111WithMask,
		decapNi: decapTeVRF, postDecapNi: encapTeVRFC, decapFallbackNi: teVRF111}

	pfRule13 := &policyFwRule{seqID: 13, protocol: 4, dscpSet: []uint8{dscpEncapD1, dscpEncapD2}, sourceAddr: ipv4OuterSrc222WithMask,
		decapNi: decapTeVRF, postDecapNi: encapTeVRFD, decapFallbackNi: teVRF222}
	pfRule14 := &policyFwRule{seqID: 14, protocol: 41, dscpSet: []uint8{dscpEncapD1, dscpEncapD2}, sourceAddr: ipv4OuterSrc222WithMask,
		decapNi: decapTeVRF, postDecapNi: encapTeVRFD, decapFallbackNi: teVRF222}
	pfRule15 := &policyFwRule{seqID: 15, protocol: 4, dscpSet: []uint8{dscpEncapD1, dscpEncapD2}, sourceAddr: ipv4OuterSrc111WithMask,
		decapNi: decapTeVRF, postDecapNi: encapTeVRFD, decapFallbackNi: teVRF111}
	pfRule16 := &policyFwRule{seqID: 16, protocol: 41, dscpSet: []uint8{dscpEncapD1, dscpEncapD2}, sourceAddr: ipv4OuterSrc111WithMask,
		decapNi: decapTeVRF, postDecapNi: encapTeVRFD, decapFallbackNi: teVRF111}

	pfRule17 := &policyFwRule{seqID: 17, protocol: 4, sourceAddr: ipv4OuterSrc222WithMask,
		decapNi: decapTeVRF, postDecapNi: deviations.DefaultNetworkInstance(dut), decapFallbackNi: teVRF222}
	pfRule18 := &policyFwRule{seqID: 18, protocol: 41, sourceAddr: ipv4OuterSrc222WithMask,
		decapNi: decapTeVRF, postDecapNi: deviations.DefaultNetworkInstance(dut), decapFallbackNi: teVRF222}
	pfRule19 := &policyFwRule{seqID: 19, protocol: 4, sourceAddr: ipv4OuterSrc111WithMask,
		decapNi: decapTeVRF, postDecapNi: deviations.DefaultNetworkInstance(dut), decapFallbackNi: teVRF111}
	pfRule20 := &policyFwRule{seqID: 20, protocol: 41, sourceAddr: ipv4OuterSrc111WithMask,
		decapNi: decapTeVRF, postDecapNi: deviations.DefaultNetworkInstance(dut), decapFallbackNi: teVRF111}

	pfRuleList := []*policyFwRule{pfRule1, pfRule2, pfRule3, pfRule4, pfRule5, pfRule6,
		pfRule7, pfRule8, pfRule9, pfRule10, pfRule11, pfRule12, pfRule13, pfRule14,
		pfRule15, pfRule16, pfRule17, pfRule18, pfRule19, pfRule20}

	ni := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niP := ni.GetOrCreatePolicyForwarding()
	niPf := niP.GetOrCreatePolicy(vrfPolW)
	niPf.SetType(oc.Policy_Type_VRF_SELECTION_POLICY)

	for _, pfRule := range pfRuleList {
		pfR := niPf.GetOrCreateRule(pfRule.seqID)
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
	pfRAction.NetworkInstance = ygot.String(deviations.DefaultNetworkInstance(dut))

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
func pushEncapEntries(t *testing.T, virtualIPs []string, decapEncapVirtualIPs []string, args *testArgs) {

	vrfEntryParams := make(map[string]*routesParam)
	tunnelIPsPerVRF := perEncapVRFNHGCount * encapNHsPerNHG

	tunnelIPEntries := createIPv4Entries(ipBlockTEVRF)[0 : tunnelIPsPerVRF*4] // tunnelIPPerVRF * (4 encapVRFs)
	// Add 1600 TE_VRF111 tunnels
	vrfEntryParams[teVRF111] = &routesParam{
		ipEntries:     tunnelIPEntries,
		numUniqueNHs:  len(virtualIPs),
		nextHops:      virtualIPs,
		nextHopVRF:    deviations.DefaultNetworkInstance(args.dut),
		numUniqueNHGs: 32,
		nhDecapEncap:  false,
	}

	installEntries(t, teVRF111, vrfEntryParams[teVRF111], args)

	if len(encapVRFIPv4Entries) < 4*perEncapVRFIPCount {
		t.Fatalf("Encap VRF IPv4 Entries in block: %s must have at-least 4x encapIPv4Count: %v", ipBlockEncap, perEncapVRFIPCount)
	}

	// Add 5k entries in ENCAP-VRF-A
	vrfEntryParams[encapTeVRFA] = &routesParam{
		ipEntries:     encapVRFIPv4Entries[0:perEncapVRFIPCount],
		numUniqueNHs:  tunnelIPsPerVRF,
		nextHops:      tunnelIPEntries[0:tunnelIPsPerVRF],
		nextHopVRF:    teVRF111,
		numUniqueNHGs: perEncapVRFNHGCount,
		nhEncap:       true,
		tunnelSrcIP:   ipv4OuterSrc111,
	}

	// Add 5k entries in ENCAP-VRF-B.
	vrfEntryParams[encapTeVRFB] = &routesParam{
		ipEntries:     encapVRFIPv4Entries[perEncapVRFIPCount : 2*perEncapVRFIPCount],
		numUniqueNHs:  tunnelIPsPerVRF,
		nextHops:      tunnelIPEntries[tunnelIPsPerVRF : 2*tunnelIPsPerVRF],
		nextHopVRF:    teVRF111,
		numUniqueNHGs: perEncapVRFNHGCount,
		nhEncap:       true,
		tunnelSrcIP:   ipv4OuterSrc111,
	}

	// Add 5k entries in ENCAP-VRF-C
	vrfEntryParams[encapTeVRFC] = &routesParam{
		ipEntries:     encapVRFIPv4Entries[2*perEncapVRFIPCount : 3*perEncapVRFIPCount],
		numUniqueNHs:  tunnelIPsPerVRF,
		nextHops:      tunnelIPEntries[2*tunnelIPsPerVRF : 3*tunnelIPsPerVRF],
		nextHopVRF:    teVRF111,
		numUniqueNHGs: perEncapVRFNHGCount,
		nhEncap:       true,
		tunnelSrcIP:   ipv4OuterSrc111,
	}

	// Add 5k entries in ENCAP-VRF-D
	vrfEntryParams[encapTeVRFD] = &routesParam{
		ipEntries:     encapVRFIPv4Entries[3*perEncapVRFIPCount : 4*perEncapVRFIPCount],
		numUniqueNHs:  tunnelIPsPerVRF,
		nextHops:      tunnelIPEntries[3*tunnelIPsPerVRF : 4*tunnelIPsPerVRF],
		nextHopVRF:    teVRF111,
		numUniqueNHGs: perEncapVRFNHGCount,
		nhEncap:       true,
		tunnelSrcIP:   ipv4OuterSrc111,
	}

	for _, vrf := range []string{encapTeVRFA, encapTeVRFB, encapTeVRFC, encapTeVRFD} {
		installEntries(t, vrf, vrfEntryParams[vrf], args)
	}

	if len(encapVRFIPv6Entries) < 4*perEncapVRFIPCount {
		t.Fatalf("Encap VRF IPv6 Entries in block: %s must have at-least 4x encapIPv6Count: %v", ipv6BlockEncap, perEncapVRFIPCount)
	}

	// Add 5k IPv6 entries in ENCAP-VRF-A
	vrfEntryParams[encapTeVRFA] = &routesParam{
		ipv6Entries:   encapVRFIPv6Entries[0:perEncapVRFIPCount],
		numUniqueNHs:  tunnelIPsPerVRF,
		nextHops:      tunnelIPEntries[0:tunnelIPsPerVRF],
		nextHopVRF:    teVRF111,
		numUniqueNHGs: perEncapVRFNHGCount,
		nhEncap:       true,
		tunnelSrcIP:   ipv4OuterSrc111,
		isIPv6:        true,
	}

	// Add 5k IPv6 entries in ENCAP-VRF-B.
	vrfEntryParams[encapTeVRFB] = &routesParam{
		ipv6Entries:   encapVRFIPv6Entries[perEncapVRFIPCount : 2*perEncapVRFIPCount],
		numUniqueNHs:  tunnelIPsPerVRF,
		nextHops:      tunnelIPEntries[tunnelIPsPerVRF : 2*tunnelIPsPerVRF],
		nextHopVRF:    teVRF111,
		numUniqueNHGs: perEncapVRFNHGCount,
		nhEncap:       true,
		tunnelSrcIP:   ipv4OuterSrc111,
		isIPv6:        true,
	}

	// Add 5k IPv6 entries in ENCAP-VRF-C.
	vrfEntryParams[encapTeVRFC] = &routesParam{
		ipv6Entries:   encapVRFIPv6Entries[2*perEncapVRFIPCount : 3*perEncapVRFIPCount],
		numUniqueNHs:  tunnelIPsPerVRF,
		nextHops:      tunnelIPEntries[2*tunnelIPsPerVRF : 3*tunnelIPsPerVRF],
		nextHopVRF:    teVRF111,
		numUniqueNHGs: perEncapVRFNHGCount,
		nhEncap:       true,
		tunnelSrcIP:   ipv4OuterSrc111,
		isIPv6:        true,
	}

	// Add 5k IPv6 entries in ENCAP-VRF-D.
	vrfEntryParams[encapTeVRFD] = &routesParam{
		ipv6Entries:   encapVRFIPv6Entries[3*perEncapVRFIPCount : 4*perEncapVRFIPCount],
		numUniqueNHs:  tunnelIPsPerVRF,
		nextHops:      tunnelIPEntries[3*tunnelIPsPerVRF : 4*tunnelIPsPerVRF],
		nextHopVRF:    teVRF111,
		numUniqueNHGs: perEncapVRFNHGCount,
		nhEncap:       true,
		tunnelSrcIP:   ipv4OuterSrc222,
		isIPv6:        true,
	}

	for _, vrf := range []string{encapTeVRFA, encapTeVRFB, encapTeVRFC, encapTeVRFD} {
		installEntries(t, vrf, vrfEntryParams[vrf], args)
	}
}

func createAndSendTrafficFlows(t *testing.T, args *testArgs, decapEntries []string, decapRouteCount uint32) {
	t.Helper()

	_, decapStartIP, _ := net.ParseCIDR(ipBlockDecap)
	flow1 := createFlow(&flowArgs{flowName: "flow1", isInnHdrV4: true, outHdrDstIPCount: decapRouteCount,
		InnHdrSrcIP: atePort1.IPv4, InnHdrDstIP: encapVRFIPv4Entries[0:perEncapVRFIPCount], inHdrDscp: []uint32{dscpEncapA1},
		outHdrSrcIP: ipv4OuterSrc111, outHdrDstIP: decapStartIP.IP.String(), outHdrDscp: []uint32{dscpEncapA1},
	})

	flow2 := createFlow(&flowArgs{flowName: "flow2", isInnHdrV4: true, outHdrDstIPCount: decapRouteCount,
		InnHdrSrcIP: atePort1.IPv4, InnHdrDstIP: encapVRFIPv4Entries[perEncapVRFIPCount : 2*perEncapVRFIPCount], inHdrDscp: []uint32{dscpEncapB1},
		outHdrSrcIP: ipv4OuterSrc222, outHdrDstIP: decapStartIP.IP.String(), outHdrDscp: []uint32{dscpEncapB1},
	})

	flow3 := createFlow(&flowArgs{flowName: "flow3", isInnHdrV4: true, outHdrDstIPCount: decapRouteCount,
		InnHdrSrcIP: atePort1.IPv4, InnHdrDstIP: encapVRFIPv4Entries[2*perEncapVRFIPCount : 3*perEncapVRFIPCount], inHdrDscp: []uint32{dscpEncapC1},
		outHdrSrcIP: ipv4OuterSrc111, outHdrDstIP: decapStartIP.IP.String(), outHdrDscp: []uint32{dscpEncapC1},
	})

	flow4 := createFlow(&flowArgs{flowName: "flow4", isInnHdrV4: true, outHdrDstIPCount: decapRouteCount,
		InnHdrSrcIP: atePort1.IPv4, InnHdrDstIP: encapVRFIPv4Entries[3*perEncapVRFIPCount : 4*perEncapVRFIPCount], inHdrDscp: []uint32{dscpEncapD1},
		outHdrSrcIP: ipv4OuterSrc222, outHdrDstIP: decapStartIP.IP.String(), outHdrDscp: []uint32{dscpEncapD1},
	})

	// Below v6 flows will fail due to ixia issue.
	// https://github.com/open-traffic-generator/fp-testbed-juniper/issues/49

	flow5 := createFlow(&flowArgs{flowName: "flow5", isInnHdrV4: false, outHdrDstIPCount: decapRouteCount,
		InnHdrSrcIPv6: atePort1.IPv6, InnHdrDstIPv6: encapVRFIPv6Entries[0:perEncapVRFIPCount], inHdrDscp: []uint32{dscpEncapA2},
		outHdrSrcIP: ipv4OuterSrc111, outHdrDstIP: decapStartIP.IP.String(), outHdrDscp: []uint32{dscpEncapA2},
	})

	flow6 := createFlow(&flowArgs{flowName: "flow6", isInnHdrV4: false, outHdrDstIPCount: decapRouteCount,
		InnHdrSrcIPv6: atePort1.IPv6, InnHdrDstIPv6: encapVRFIPv6Entries[perEncapVRFIPCount : 2*perEncapVRFIPCount], inHdrDscp: []uint32{dscpEncapB2},
		outHdrSrcIP: ipv4OuterSrc222, outHdrDstIP: decapStartIP.IP.String(), outHdrDscp: []uint32{dscpEncapB2},
	})

	flow7 := createFlow(&flowArgs{flowName: "flow7", isInnHdrV4: false, outHdrDstIPCount: decapRouteCount,
		InnHdrSrcIPv6: atePort1.IPv6, InnHdrDstIPv6: encapVRFIPv6Entries[2*perEncapVRFIPCount : 3*perEncapVRFIPCount], inHdrDscp: []uint32{dscpEncapC2},
		outHdrSrcIP: ipv4OuterSrc111, outHdrDstIP: decapStartIP.IP.String(), outHdrDscp: []uint32{dscpEncapC2},
	})

	flow8 := createFlow(&flowArgs{flowName: "flow8", isInnHdrV4: false, outHdrDstIPCount: decapRouteCount,
		InnHdrSrcIPv6: atePort1.IPv6, InnHdrDstIPv6: encapVRFIPv6Entries[3*perEncapVRFIPCount : 4*perEncapVRFIPCount], inHdrDscp: []uint32{dscpEncapD2},
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
	otgutils.WaitForARP(t, args.ate.OTG(), args.top, "IPv4")

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

func pushDecapEntries(t *testing.T, args *testArgs, decapEntries []string) {
	lastNhIndex++
	lastNhgIndex++
	nhIdx := uint64(lastNhIndex)
	nhgIdx := uint64(lastNhgIndex)

	entries := []fluent.GRIBIEntry{
		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
			WithIndex(nhIdx).WithDecapsulateHeader(fluent.IPinIP).
			WithNextHopNetworkInstance(deviations.DefaultNetworkInstance(args.dut)),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
			WithID(nhgIdx).AddNextHop(nhIdx, 1),
	}

	mask := []string{"22", "24", "26", "28"}
	j := 0
	for i := 0; i < len(decapEntries); i++ {
		prefMask := mask[j]
		entries = append(entries, fluent.IPv4Entry().WithNetworkInstance(decapTeVRF).
			WithPrefix(decapEntries[i]+"/"+prefMask).WithNextHopGroup(nhgIdx).
			WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(args.dut)),
		)
		if i == 12|24|36 {
			j = j + 1
		}
	}

	args.client.Modify().AddEntry(t, entries...)
	if err := awaitTimeout(args.ctx, args.client, t, 3*time.Minute); err != nil {
		t.Fatalf("Could not program entries via client, got err: %v", err)
	}
	res := []*client.OpResult{
		fluent.OperationResult().
			WithNextHopOperation(nhIdx).
			WithOperationType(constants.Add).
			WithProgrammingResult(fluent.InstalledInFIB).
			AsResult(),
		fluent.OperationResult().
			WithNextHopGroupOperation(nhgIdx).
			WithOperationType(constants.Add).
			WithProgrammingResult(fluent.InstalledInFIB).
			AsResult(),
	}
	for i := 0; i < len(decapEntries); i++ {
		res = append(res,
			fluent.OperationResult().
				WithIPv4Operation(decapEntries[i]+"/32").
				WithOperationType(constants.Add).
				WithProgrammingResult(fluent.InstalledInFIB).
				AsResult(),
		)
	}

	t.Logf("Installed %v Decap VRF IPv4 entries with mixed prefix length", len(decapEntries))
}

func pushDecapScaleEntries(t *testing.T, args *testArgs, decapEntries []string) {
	lastNhIndex++
	lastNhgIndex++

	nhIdx := uint64(lastNhIndex)
	nhgIdx := uint64(lastNhgIndex)
	entries := []fluent.GRIBIEntry{
		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
			WithIndex(nhIdx).WithDecapsulateHeader(fluent.IPinIP).
			WithNextHopNetworkInstance(deviations.DefaultNetworkInstance(args.dut)),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
			WithID(nhgIdx).AddNextHop(nhIdx, 1),
	}

	for i := 0; i < len(decapEntries); i++ {
		entries = append(entries, fluent.IPv4Entry().WithNetworkInstance(decapTeVRF).
			WithPrefix(decapEntries[i]+"/"+"32").WithNextHopGroup(nhgIdx).
			WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(args.dut)))
	}

	args.client.Modify().AddEntry(t, entries...)
	if err := awaitTimeout(args.ctx, args.client, t, 5*time.Minute); err != nil {
		t.Fatalf("Could not program entries via client, got err: %v", err)
	}

	res := []*client.OpResult{
		fluent.OperationResult().
			WithNextHopOperation(nhIdx).
			WithOperationType(constants.Add).
			WithProgrammingResult(fluent.InstalledInFIB).
			AsResult(),
		fluent.OperationResult().
			WithNextHopGroupOperation(nhgIdx).
			WithOperationType(constants.Add).
			WithProgrammingResult(fluent.InstalledInFIB).
			AsResult(),
	}
	for i := 0; i < len(decapEntries); i++ {
		res = append(res,
			fluent.OperationResult().
				WithIPv4Operation(decapEntries[i]+"/32").
				WithOperationType(constants.Add).
				WithProgrammingResult(fluent.InstalledInFIB).
				AsResult(),
		)
	}
	chk.HasResultsCache(t, args.client.Results(t), res, chk.IgnoreOperationID())

	t.Logf("Installed %v Decap VRF IPv4 scale entries with prefix length 32", len(decapEntries))
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
	outerIPHdr := flow.Packet().Add().Ipv4()
	outerIPHdr.Src().SetValue(flowValues.outHdrSrcIP)
	outerIPHdr.Dst().Increment().SetStart(flowValues.outHdrDstIP).SetCount(flowValues.outHdrDstIPCount)
	outerIPHdr.Priority().Dscp().Phb().SetValues(flowValues.outHdrDscp)

	if flowValues.isInnHdrV4 {
		innerIPHdr := flow.Packet().Add().Ipv4()
		innerIPHdr.Src().SetValue(flowValues.InnHdrSrcIP)
		innerIPHdr.Dst().SetValues(flowValues.InnHdrDstIP)
		if len(flowValues.inHdrDscp) != 0 {
			innerIPHdr.Priority().Dscp().Phb().SetValues(flowValues.inHdrDscp)
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
func pushIPv4Entries(t *testing.T, virtualIPs []string, decapEncapVirtualIPs []string, args *testArgs) {

	// install backup NHGs/NHs
	// NHG {ID #1} --> NH {ID #1, network-instance: VRF-C}
	entries := []fluent.GRIBIEntry{
		fluent.NextHopEntry().
			WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
			WithIndex(1).
			WithNextHopNetworkInstance(vrf3).
			WithElectionID(args.electionID.Low, args.electionID.High),

		fluent.NextHopGroupEntry().
			WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
			WithID(1).
			AddNextHop(1, 1).
			WithElectionID(args.electionID.Low, args.electionID.High),

		// NHG {ID #2} --> NH {ID #2, decap, network-instance: DEFAULT}
		fluent.NextHopEntry().
			WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
			WithIndex(2).
			WithDecapsulateHeader(fluent.IPinIP).
			WithNextHopNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
			WithElectionID(args.electionID.Low, args.electionID.High),

		fluent.NextHopGroupEntry().
			WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
			WithID(2).
			AddNextHop(2, 1).
			WithElectionID(args.electionID.Low, args.electionID.High),
	}

	args.client.Modify().AddEntry(t, entries...)
	if err := awaitTimeout(args.ctx, args.client, t, time.Minute); err != nil {
		t.Fatalf("Could not program entries via client, got err: %v", err)
	}

	chk.HasResultsCache(t, args.client.Results(t), []*client.OpResult{
		fluent.OperationResult().
			WithNextHopGroupOperation(1).
			WithOperationType(constants.Add).
			WithProgrammingResult(fluent.InstalledInFIB).
			AsResult(),
		fluent.OperationResult().
			WithNextHopGroupOperation(2).
			WithOperationType(constants.Add).
			WithProgrammingResult(fluent.InstalledInFIB).
			AsResult(),
	}, chk.IgnoreOperationID())

	nonDefaultVIPs := createIPv4Entries(ipBlockNonDefaultVRF)
	if len(nonDefaultVIPs) < 2*numVirtualIPsNonDefaultVRF {
		t.Fatalf("Too few non-default VRF IPv4 entries in block: %s, need atleast: 2 * %d = (%d)", ipBlockNonDefaultVRF, numVirtualIPsNonDefaultVRF, 2*numVirtualIPsNonDefaultVRF)
	}

	// provision non-default VRF gRIBI entries, and associated NHGs, NHs in default instance
	vrfEntryParams := make(map[string]*routesParam)
	vrfEntryParams[vrf1] = &routesParam{
		ipEntries:     nonDefaultVIPs[0:numVirtualIPsNonDefaultVRF],
		nextHops:      virtualIPs,
		numUniqueNHs:  len(virtualIPs),
		nextHopVRF:    deviations.DefaultNetworkInstance(args.dut),
		numUniqueNHGs: 8,
		backupNHG:     1,
		nhDecapEncap:  false,
	}
	vrfEntryParams[vrf2] = &routesParam{
		ipEntries:     nonDefaultVIPs[0:numVirtualIPsNonDefaultVRF],
		nextHops:      decapEncapVirtualIPs,
		numUniqueNHs:  len(decapEncapVirtualIPs),
		nextHopVRF:    deviations.DefaultNetworkInstance(args.dut),
		numUniqueNHGs: 8,
		backupNHG:     2,
		nhDecapEncap:  false,
	}
	vrfEntryParams[vrf3] = &routesParam{
		ipEntries:     nonDefaultVIPs[numVirtualIPsNonDefaultVRF : 2*numVirtualIPsNonDefaultVRF],
		nextHops:      nonDefaultVIPs[0:numVirtualIPsNonDefaultVRF],
		numUniqueNHs:  numVirtualIPsNonDefaultVRF,
		nextHopVRF:    vrf2,
		numUniqueNHGs: 8,
		backupNHG:     2,
		nhDecapEncap:  true,
	}

	for _, vrf := range []string{vrf1, vrf2, vrf3} {
		installEntries(t, vrf, vrfEntryParams[vrf], args)
	}
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
	t.Helper()
	entries := []fluent.GRIBIEntry{}
	// Provision next-hops
	nextHopIndices := []uint64{}
	for i := 0; i < routeParams.numUniqueNHs; i++ {
		lastNhIndex++
		if routeParams.nhDecapEncap {
			entries = append(entries, fluent.NextHopEntry().
				WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
				WithIndex(uint64(lastNhIndex)).
				WithIPinIP(tunnelSrcIPv4Addr, routeParams.nextHops[i%len(routeParams.nextHops)]).
				WithDecapsulateHeader(fluent.IPinIP).
				WithEncapsulateHeader(fluent.IPinIP).
				WithNextHopNetworkInstance(routeParams.nextHopVRF).
				WithElectionID(args.electionID.Low, args.electionID.High))
		} else if routeParams.nhEncap {
			entries = append(entries, fluent.NextHopEntry().
				WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
				WithIndex(uint64(lastNhIndex)).
				WithIPinIP(routeParams.tunnelSrcIP, routeParams.nextHops[i%len(routeParams.nextHops)]).
				WithEncapsulateHeader(fluent.IPinIP).
				WithNextHopNetworkInstance(routeParams.nextHopVRF).
				WithElectionID(args.electionID.Low, args.electionID.High))
		} else {
			entries = append(entries, fluent.NextHopEntry().
				WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
				WithIndex(uint64(lastNhIndex)).
				WithIPAddress(routeParams.nextHops[i%len(routeParams.nextHops)]).
				WithElectionID(args.electionID.Low, args.electionID.High))
		}
		nextHopIndices = append(nextHopIndices, uint64(lastNhIndex))
	}

	// Provision next-hop-groups
	nextHopGroupIndices := []uint64{}
	nhPerNHG := len(nextHopIndices) / routeParams.numUniqueNHGs
	if len(nextHopIndices)%routeParams.numUniqueNHGs != 0 {
		t.Errorf("Count of NHs: %v not a multiple of Count of NHGs: %v", len(nextHopIndices), routeParams.numUniqueNHGs)
	}
	weights := generateNextHopWeights(nhWeightSum, nhPerNHG)
	for i := 0; i < routeParams.numUniqueNHGs; i++ {
		lastNhgIndex++
		nhgEntry := fluent.NextHopGroupEntry().
			WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
			WithID(uint64(lastNhgIndex)).
			WithElectionID(args.electionID.Low, args.electionID.High)

		if routeParams.backupNHG > 0 {
			nhgEntry.WithBackupNHG(uint64(routeParams.backupNHG))
		}

		nhCount := 0
		for j := 0; j < nhPerNHG; j++ {
			idx := (i * nhPerNHG) + j
			if idx >= len(nextHopIndices) {
				break
			}
			// Encap NHGs should have weighted NHs
			if routeParams.nhEncap {
				nhgEntry.AddNextHop(nextHopIndices[idx], uint64(weights[j]))
			} else {
				nhgEntry.AddNextHop(nextHopIndices[idx], 1)
			}
			nhCount++
		}
		if nhCount == 0 {
			break
		}
		entries = append(entries, nhgEntry)
		nextHopGroupIndices = append(nextHopGroupIndices, uint64(lastNhgIndex))
	}

	// Provision ip entires in VRF
	if routeParams.isIPv6 {
		for i := range routeParams.ipv6Entries {
			entries = append(entries,
				fluent.IPv6Entry().
					WithPrefix(routeParams.ipv6Entries[i]+"/128").
					WithNetworkInstance(vrf).
					WithNextHopGroup(nextHopGroupIndices[i%len(nextHopGroupIndices)]).
					WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(args.dut)))
		}
	} else {
		for i := range routeParams.ipEntries {
			entries = append(entries,
				fluent.IPv4Entry().
					WithPrefix(routeParams.ipEntries[i]+"/32").
					WithNetworkInstance(vrf).
					WithNextHopGroup(nextHopGroupIndices[i%len(nextHopGroupIndices)]).
					WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(args.dut)))
		}
	}
	args.client.Modify().AddEntry(t, entries...)
	if err := awaitTimeout(args.ctx, args.client, t, 5*time.Minute); err != nil {
		t.Fatalf("Could not program entries via client, got err: %v", err)
	}
	res := []*client.OpResult{}
	if routeParams.isIPv6 {
		for i := range routeParams.ipv6Entries {
			res = append(res,
				fluent.OperationResult().
					WithIPv6Operation(routeParams.ipv6Entries[i]+"/128").
					WithOperationType(constants.Add).
					WithProgrammingResult(fluent.InstalledInFIB).
					AsResult(),
			)
		}
	} else {
		res = append(res,
			fluent.OperationResult().
				WithNextHopGroupOperation(nextHopGroupIndices[0]).
				WithOperationType(constants.Add).
				WithProgrammingResult(fluent.InstalledInFIB).
				AsResult(),
		)

		for i := range routeParams.ipEntries {
			res = append(res,
				fluent.OperationResult().
					WithIPv4Operation(routeParams.ipEntries[i]+"/32").
					WithOperationType(constants.Add).
					WithProgrammingResult(fluent.InstalledInFIB).
					AsResult(),
			)
		}
	}
	chk.HasResultsCache(t, args.client.Results(t), res, chk.IgnoreOperationID())

	if routeParams.isIPv6 {
		t.Logf("Installed entries VRF %s - IPv6 entry count: %d, next-hop-group count: %d (index %d - %d), next-hop count: %d (index %d - %d)", vrf, len(routeParams.ipv6Entries), len(nextHopGroupIndices), nextHopGroupIndices[0], nextHopGroupIndices[len(nextHopGroupIndices)-1], len(nextHopIndices), nextHopIndices[0], nextHopIndices[len(nextHopIndices)-1])
	} else {
		t.Logf("Installed entries VRF %s - IPv4 entry count: %d, next-hop-group count: %d (index %d - %d), next-hop count: %d (index %d - %d)", vrf, len(routeParams.ipEntries), len(nextHopGroupIndices), nextHopGroupIndices[0], nextHopGroupIndices[len(nextHopGroupIndices)-1], len(nextHopIndices), nextHopIndices[0], nextHopIndices[len(nextHopIndices)-1])
	}
}

// pushDefaultEntries installs gRIBI next-hops, next-hop-groups and IP entries for base route resolution
func pushDefaultEntries(t *testing.T, args *testArgs, nextHops []*nextHopIntfRef) ([]string, []string) {
	lastNhIndex = nextHopStartIndex
	primaryNextHopIfs := nextHops[0 : subIntfCount/2]
	decapEncapNextHopIfs := nextHops[subIntfCount/2:]

	primaryNextHopIndices := []uint64{}
	entries := []fluent.GRIBIEntry{}
	for i := 0; i < len(primaryNextHopIfs); i++ {
		entry := fluent.NextHopEntry().
			WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
			WithIndex(uint64(lastNhIndex)).
			WithMacAddress(magicMAC).
			WithElectionID(args.electionID.Low, args.electionID.High)
		if deviations.GRIBIMACOverrideStaticARPStaticRoute(args.dut) {
			entry.WithIPAddress(primaryNextHopIfs[i].nextHopIPAddress)
			intfName := primaryNextHopIfs[i].intfName
			if uint64(primaryNextHopIfs[i].subintfIndex) > 0 {
				intfName += fmt.Sprintf(".%d", primaryNextHopIfs[i].subintfIndex)
			}
			entry.WithInterfaceRef(intfName)
		} else {
			entry.WithSubinterfaceRef(primaryNextHopIfs[i].intfName, uint64(primaryNextHopIfs[i].subintfIndex))
		}
		entries = append(entries, entry)
		primaryNextHopIndices = append(primaryNextHopIndices, uint64(lastNhIndex))
		lastNhIndex++
	}

	decapEncapNextHopIndices := []uint64{}
	for i := 0; i < len(decapEncapNextHopIfs); i++ {
		entry := fluent.NextHopEntry().
			WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
			WithIndex(uint64(lastNhIndex)).
			WithMacAddress(magicMAC).
			WithElectionID(args.electionID.Low, args.electionID.High)
		if deviations.GRIBIMACOverrideStaticARPStaticRoute(args.dut) {
			entry.WithIPAddress(decapEncapNextHopIfs[i].nextHopIPAddress)
			intfName := decapEncapNextHopIfs[i].intfName
			if uint64(decapEncapNextHopIfs[i].subintfIndex) > 0 {
				intfName += fmt.Sprintf(".%d", decapEncapNextHopIfs[i].subintfIndex)
			}
			entry.WithInterfaceRef(intfName)
		} else {
			entry.WithSubinterfaceRef(decapEncapNextHopIfs[i].intfName, uint64(decapEncapNextHopIfs[i].subintfIndex))
		}
		entries = append(entries, entry)
		decapEncapNextHopIndices = append(decapEncapNextHopIndices, uint64(lastNhIndex))
		lastNhIndex++
	}

	lastNhgIndex = nextHopGroupStartIndex
	primaryNHGIdx := uint64(lastNhgIndex)
	nhgEntry := fluent.NextHopGroupEntry().
		WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
		WithID(primaryNHGIdx).
		WithElectionID(args.electionID.Low, args.electionID.High)
	for j := range primaryNextHopIndices {
		nhgEntry.AddNextHop(primaryNextHopIndices[j], 1)
	}
	entries = append(entries, nhgEntry)
	lastNhgIndex++

	decapEncapNHGIdx := uint64(lastNhgIndex)
	nhgEntry = fluent.NextHopGroupEntry().
		WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
		WithID(decapEncapNHGIdx).
		WithElectionID(args.electionID.Low, args.electionID.High)
	for j := range decapEncapNextHopIndices {
		nhgEntry.AddNextHop(decapEncapNextHopIndices[j], 1)
	}
	entries = append(entries, nhgEntry)
	lastNhgIndex++

	virtualIPs := createIPv4Entries(ipBlockDefaultVRF)
	primaryVirtualIPs := virtualIPs[0 : numVirtualIPsDefaultVRF/2]
	decapEncapVirtualIPs := virtualIPs[numVirtualIPsDefaultVRF/2 : numVirtualIPsDefaultVRF]

	// install IPv4 entries for primary forwarding cases (referenced from vrf1)
	for i := range primaryVirtualIPs {
		entries = append(entries, fluent.IPv4Entry().
			WithPrefix(primaryVirtualIPs[i]+"/32").
			WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
			WithNextHopGroup(primaryNHGIdx).
			WithElectionID(args.electionID.Low, args.electionID.High))
	}

	args.client.Modify().AddEntry(t, entries...)
	if err := awaitTimeout(args.ctx, args.client, t, 5*time.Minute); err != nil {
		t.Fatalf("Could not program entries via client, got err: %v", err)
	}
	t.Logf("Installed %s VRF \"primary\" next-hop count: %d (index %d - %d)", deviations.DefaultNetworkInstance(args.dut), len(primaryNextHopIndices), primaryNextHopIndices[0], primaryNextHopIndices[len(primaryNextHopIndices)-1])
	t.Logf("Installed %s VRF \"decap/encap\" next-hop count: %d (index %d - %d)", deviations.DefaultNetworkInstance(args.dut), len(decapEncapNextHopIndices), decapEncapNextHopIndices[0], decapEncapNextHopIndices[len(decapEncapNextHopIndices)-1])
	t.Logf("Installed %s VRF \"primary\" next-hop-group count: 1 (index %d)", deviations.DefaultNetworkInstance(args.dut), primaryNHGIdx)
	t.Logf("Installed %s VRF \"decap/encap\" next-hop-group count: 1 (index %d)", deviations.DefaultNetworkInstance(args.dut), decapEncapNHGIdx)

	for i := range primaryVirtualIPs {
		chk.HasResult(t, args.client.Results(t),
			fluent.OperationResult().
				WithIPv4Operation(primaryVirtualIPs[i]+"/32").
				WithOperationType(constants.Add).
				WithProgrammingResult(fluent.InstalledInFIB).
				AsResult(),
			chk.IgnoreOperationID(),
		)
	}
	t.Logf("Installed %s VRF \"primary\" IPv4 entries, %s/32 to %s/32", deviations.DefaultNetworkInstance(args.dut), primaryVirtualIPs[0], primaryVirtualIPs[len(primaryVirtualIPs)-1])

	// install IPv4 entries for decap/encap cases (referenced from vrf2)
	for i := range decapEncapVirtualIPs {
		args.client.Modify().AddEntry(t,
			fluent.IPv4Entry().
				WithPrefix(decapEncapVirtualIPs[i]+"/32").
				WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
				WithNextHopGroup(decapEncapNHGIdx).
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

	return primaryVirtualIPs, decapEncapVirtualIPs
}

// configureDUT configures DUT interfaces and policy forwarding. Subinterfaces on DUT port2 are configured separately
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")
	d := &oc.Root{}
	dutOCRoot := gnmi.OC()

	vrfs := []string{deviations.DefaultNetworkInstance(dut), vrf1, vrf2, vrf3, decapTeVRF,
		encapTeVRFA, encapTeVRFB, encapTeVRFC, encapTeVRFD, teVRF111, teVRF222}
	createVRF(t, dut, vrfs)

	// configure Ethernet interfaces first
	gnmi.Replace(t, dut, dutOCRoot.Interface(dp1.Name()).Config(), dutPort1.NewOCInterface(dp1.Name(), dut))
	configureInterfaceDUT(t, d, dut, dp2, "dst")

	// configure an L3 subinterface without vlan tagging under DUT port#1
	createSubifDUT(t, d, dut, dp1, 0, 0, dutPort1.IPv4, ipv4PrefixLen)
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, dp1.Name(), deviations.DefaultNetworkInstance(dut), 0)
	}
}

// createVRF takes in a list of VRF names and creates them on the target devices.
func createVRF(t *testing.T, dut *ondatra.DUTDevice, vrfs []string) {
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

// configureDUTSubIfs configures subIntfCount DUT subinterfaces on the target device
func configureDUTSubIfs(t *testing.T, dut *ondatra.DUTDevice, dutPort *ondatra.Port) []*nextHopIntfRef {
	d := &oc.Root{}
	nextHops := []*nextHopIntfRef{}
	batchConfig := &gnmi.SetBatch{}
	for i := 0; i < subIntfCount; i++ {
		index := uint32(i)

		vlanID := uint16(i)
		if deviations.NoMixOfTaggedAndUntaggedSubinterfaces(dut) {
			vlanID++
		}
		dutIPv4 := incrementIP(subifBaseIP, (4*i)+2)
		ateIPv4 := incrementIP(subifBaseIP, (4*i)+1)
		intf := createSubifDUT(t, d, dut, dutPort, index, vlanID, dutIPv4, ipv4PrefixLen)
		if deviations.GRIBIMACOverrideStaticARPStaticRoute(dut) {
			intf.GetOrCreateIpv4().GetOrCreateNeighbor(magicIP).SetLinkLayerAddress(magicMAC)
		}
		gnmi.BatchUpdate(batchConfig, gnmi.OC().Interface(dutPort.Name()).Subinterface(index).Config(), intf)
		if deviations.ExplicitInterfaceInDefaultVRF(dut) {
			fptest.AssignToNetworkInstance(t, dut, dutPort.Name(), deviations.DefaultNetworkInstance(dut), index)
		}
		nhIP := ateIPv4
		if deviations.GRIBIMACOverrideStaticARPStaticRoute(dut) {
			nhIP = magicIP
		}
		nextHops = append(nextHops, &nextHopIntfRef{
			nextHopIPAddress: nhIP,
			subintfIndex:     index,
			intfName:         dutPort.Name(),
		})
	}
	if deviations.GRIBIMACOverrideStaticARPStaticRoute(dut) {
		sp := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
		p2 := dut.Port(t, "port2")
		static := &oc.NetworkInstance_Protocol_Static{
			Prefix: ygot.String(magicIP + "/32"),
			NextHop: map[string]*oc.NetworkInstance_Protocol_Static_NextHop{
				"0": {
					Index: ygot.String("0"),
					InterfaceRef: &oc.NetworkInstance_Protocol_Static_NextHop_InterfaceRef{
						Interface: ygot.String(p2.Name()),
					},
				},
			},
		}
		for i := 1; i < subIntfCount; i++ {
			idx := fmt.Sprintf("%d", i)
			static.NextHop[idx] = &oc.NetworkInstance_Protocol_Static_NextHop{
				Index: ygot.String(idx),
				InterfaceRef: &oc.NetworkInstance_Protocol_Static_NextHop_InterfaceRef{
					Interface:    ygot.String(p2.Name()),
					Subinterface: ygot.Uint32(uint32(i)),
				},
			}
		}
		gnmi.BatchReplace(batchConfig, sp.Static(magicIP+"/32").Config(), static)
	}
	batchConfig.Set(t, dut)
	return nextHops
}

// configureATESubIfs configures subIntfCount ATE subinterfaces on the target device
// It returns a slice of the corresponding ATE IPAddresses.
func configureATESubIfs(t *testing.T, top gosnappi.Config, atePort *ondatra.Port, dut *ondatra.DUTDevice) []string {
	nextHops := []string{}
	for i := 0; i < subIntfCount; i++ {
		vlanID := uint16(i)
		if deviations.NoMixOfTaggedAndUntaggedSubinterfaces(dut) {
			vlanID++
		}
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
	ate := ondatra.ATE(t, "ate")
	ctx := context.Background()
	gribic := dut.RawAPIs().GRIBI(t)
	ap2 := ate.Port(t, "port2")
	dp2 := dut.Port(t, "port2")

	top := gosnappi.NewConfig()
	top.Ports().Add().SetName(ate.Port(t, "port2").ID())

	configureDUT(t, dut)
	// configure subIntfCount L3 subinterfaces under DUT port#2 and assign them to DEFAULT vrf
	// return slice containing interface name, subinterface index and ATE next hop IP that will be used for creating gRIBI next-hop entries
	subIntfNextHops := configureDUTSubIfs(t, dut, dp2)

	configureATEPort1(t, top)
	configureATESubIfs(t, top, ap2, dut)
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")

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
	if err := gribi.FlushAll(client); err != nil {
		t.Fatal(err)
	}

	args := &testArgs{
		ctx:        ctx,
		client:     client,
		dut:        dut,
		ate:        ate,
		top:        top,
		electionID: eID,
	}

	// pushDefaultEntries installs gRIBI next-hops, next-hop-groups and IP entries for base route resolution
	// primaryIpv4Entries are ipv4 entries used for deriving nextHops for ipBlockNonDefaultVRF
	primaryIpv4Entries, decapEncapIpv4Entries := pushDefaultEntries(t, args, subIntfNextHops)

	// pushIPv4Entries builds the recursive scaling topology
	pushIPv4Entries(t, primaryIpv4Entries, decapEncapIpv4Entries, args)

	// Apply vrf_selection_policy_w to DUT port-1.
	configureVRFSelectionPolicyW(t, dut)

	// Inject 5000 IPv4Entry-ies and 5000 IPv6Entry-ies to each of the 4 encap VRFs.
	pushEncapEntries(t, primaryIpv4Entries, decapEncapIpv4Entries, args)

	if !deviations.GribiDecapMixedPlenUnsupported(dut) {
		// Inject mixed length prefixes (48 entries) in the DECAP_TE_VRF.
		decapEntries := createIPv4Entries(ipBlockDecap)[0:decapIPv4Count]
		pushDecapEntries(t, args, decapEntries)
		// Send traffic and verify packets to DUT-1.
		createAndSendTrafficFlows(t, args, decapEntries, decapIPv4Count)
		// Flush the DECAP_TE_VRF
		if _, err := gribi.Flush(client, args.electionID, decapTeVRF); err != nil {
			t.Error(err)
		}
	}

	// Install decapIPv4ScaleCount entries with fixed prefix length of /32 in DECAP_TE_VRF.
	decapScaleEntries := createIPv4Entries(ipBlockDecap)[0:decapIPv4ScaleCount]
	pushDecapScaleEntries(t, args, decapScaleEntries)
	// Send traffic and verify packets to DUT-1.
	createAndSendTrafficFlows(t, args, decapScaleEntries, decapIPv4ScaleCount)
}
