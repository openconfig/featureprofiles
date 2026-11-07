// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package static_lsp_test

import (
	"github.com/open-traffic-generator/snappi/gosnappi"
	otgconfighelpers "github.com/openconfig/featureprofiles/internal/otg_helpers/otg_config_helpers"
)

const (
	ipv4PrefixLen   = 30
	ipv6PrefixLen   = 126
	lspNextHopIndex = 0
	tolerance       = 0.01 // 1% Traffic Tolerance

	// MPLSoGUE tunnel (outer) parameters.
	gueUDPPort    = 6635         // Well-known UDP port for MPLSoGUE.
	guePolicyName = "customer10" // Policy forwarding name for MPLSoGUE decap.
	outerSrcIP    = "100.64.0.1" // Outer IPv4 source of the encapsulated traffic (from 100.64.0.0/22).
	outerDstIP    = "11.1.1.1"   // Outer IPv4 destination, within the DUT decap prefix (11.0.0.0/8).
	outerUDPSrc   = 49152

	// Base Static LSP labels bound to default untagged egress next-hop.
	mplsLabelIPv4 = 99991
	mplsLabelIPv6 = 99992

	// PF-1.25.v6 push (ingress) LSP parameters: match an IPv6 FEC subnet and push a
	// single label towards the ATE IPv6 next-hop on the egress port.
	v6LSPName       = "v6-static-lsp"
	mplsPushLabelV6 = 100100
	pushFECDstIP    = "2001:db8:cafe::1"
	pushFECDstCIDR  = "2001:db8:cafe::/64"

	// Inner payload addressing.
	innerIPv4Src = "198.51.100.1"
	innerIPv4Dst = "203.0.113.1"
	innerIPv6Src = "2001:db8:2::1"
	innerIPv6Dst = "2001:db8:2::2"

	// Inner payload DSCP/TTL used to validate preservation across decap.
	innerTTL = 64

	trafficPPS     = 1000
	trafficPackets = 10000
)

var (
	// Pad the two short IMIX bins explicitly. OTG implementations require up
	// to 140 bytes for encapsulated IPv6 plus tracking; use 160 bytes so
	// packet validity does not depend on the controller adjusting frame sizes.
	// Preserve the combined short-frame weight and the jumbo-frame coverage.
	sizeWeightProfile = []otgconfighelpers.SizeWeightPair{
		{Size: 160, Weight: 40},
		{Size: 256, Weight: 20},
		{Size: 512, Weight: 10},
		{Size: 1024, Weight: 10},
		{Size: 1500, Weight: 18},
		{Size: 9000, Weight: 2},
	}
)

// innerIPv4Flow returns inner IPv4 payload with random/varying source/dest IPs, DSCP range 0-56,
// and TCP/UDP source and destination port variation.
func innerIPv4Flow() *otgconfighelpers.Flow {
	return &otgconfighelpers.Flow{
		IPv4Flow: &otgconfighelpers.IPv4FlowParams{
			IPv4Src:      innerIPv4Src,
			IPv4SrcCount: 1000,
			IPv4Dst:      innerIPv4Dst,
			IPv4DstCount: 1000,
			TTL:          innerTTL,
			DSCP:         0,
			DSCPCount:    57,
		},
		TCPFlow: &otgconfighelpers.TCPFlowParams{
			TCPSrcPort:  49152,
			TCPSrcCount: 1000,
			TCPDstPort:  80,
			TCPDstCount: 1000,
		},
	}
}

// innerIPv6Flow returns inner IPv6 payload with random/varying source/dest IPs, DSCP/TC range,
// and TCP/UDP source and destination port variation.
func innerIPv6Flow() *otgconfighelpers.Flow {
	return &otgconfighelpers.Flow{
		IPv6Flow: &otgconfighelpers.IPv6FlowParams{
			IPv6Src:           innerIPv6Src,
			IPv6SrcCount:      1000,
			IPv6Dst:           innerIPv6Dst,
			IPv6DstCount:      1000,
			HopLimit:          innerTTL,
			TrafficClass:      0,
			TrafficClassCount: 255,
		},
		TCPFlow: &otgconfighelpers.TCPFlowParams{
			TCPSrcPort:  49152,
			TCPSrcCount: 1000,
			TCPDstPort:  80,
			TCPDstCount: 1000,
		},
	}
}

// allIPv4TOSValuesForDSCPRange returns the IPv4 ToS byte values that correspond to
// the DSCP range [start, end]
func allIPv4TOSValuesForDSCPRange(start, end uint32) []uint8 {
	vals := make([]uint8, 0, end-start+1)
	for dscp := start; dscp <= end; dscp++ {
		vals = append(vals, uint8(dscp<<2))
	}
	return vals
}

// innerIPv4PreserveSweepFlow sends one flow that cycles through DSCP values 0..56.
// This validates the full range without 57 independent OTG runs/captures.
func innerIPv4PreserveSweepFlow() *otgconfighelpers.Flow {
	return &otgconfighelpers.Flow{
		IPv4Flow: &otgconfighelpers.IPv4FlowParams{
			IPv4Src:   innerIPv4Src,
			IPv4Dst:   innerIPv4Dst,
			TTL:       innerTTL,
			DSCP:      0,
			DSCPCount: 57,
		},
		TCPFlow: &otgconfighelpers.TCPFlowParams{
			TCPSrcPort:  49152,
			TCPSrcCount: 1000,
			TCPDstPort:  80,
			TCPDstCount: 1000,
		},
	}
}

// createMPLSoGUEFlow builds an MPLSoGUE-encapsulated flow from an Ingress ATE port towards the DUT.
func createMPLSoGUEFlow(top gosnappi.Config, name string, txName string, txMAC string, label uint32, inner *otgconfighelpers.Flow, rxName string) {
	f := &otgconfighelpers.Flow{
		FlowName:          name,
		TxNames:           []string{txName},
		RxNames:           []string{rxName},
		SizeWeightProfile: &sizeWeightProfile,
		PpsRate:           trafficPPS,
		PacketsToSend:     trafficPackets,
		EthFlow:           &otgconfighelpers.EthFlowParams{SrcMAC: txMAC},
		IPv4Flow:          &otgconfighelpers.IPv4FlowParams{IPv4Src: outerSrcIP, IPv4SrcCount: 1000, IPv4Dst: outerDstIP},
		UDPFlow:           &otgconfighelpers.UDPFlowParams{UDPSrcPort: outerUDPSrc, UDPSrcCount: 1000, UDPDstPort: gueUDPPort},
		MPLSFlow:          &otgconfighelpers.MPLSFlowParams{MPLSLabel: label, MPLSExp: 7},
	}
	f.CreateFlow(top)
	f.AddEthHeader()
	f.AddIPv4Header()
	f.AddUDPHeader()
	f.AddMPLSHeader()
	if inner.IPv4Flow != nil {
		f.IPv4Flow = inner.IPv4Flow
		f.AddIPv4Header()
	}
	if inner.IPv6Flow != nil {
		f.IPv6Flow = inner.IPv6Flow
		f.AddIPv6Header()
	}
	// The outer GUE encapsulation uses UDP, while the inner payload TCP
	if inner.TCPFlow != nil {
		f.TCPFlow = inner.TCPFlow
		f.AddTCPHeader()
	}
}
