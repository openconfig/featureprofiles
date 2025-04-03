// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package tescale provides functions for tescale
package b4_scale_profile_test

import (
	"encoding/binary"
	"fmt"
	"net"
	"net/netip"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/featureprofiles/internal/iputil"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
)

const (
	// VRFT vrf t
	VRFT = "TRANSIT_VRF"
	// VRFR vrf r
	VRFR = "REPAIRED"
	// VRFRD vrf rd
	VRFRD = "vrf_rd"
	// V4TunnelIPBlock tunnel IP block
	V4TunnelIPBlock = "200.200.200.1/16"
	// V4VIPIPBlock vip IP block
	V4VIPIPBlock        = "100.100.100.1/22"
	tunnelSrcIP         = "18.18.18.18"
	encapNhCount        = 1600
	encapNhgcount       = 100  // 200
	encapIPv4Count      = 5000 // 5000
	encapIPv6Count      = 5000 // 5000
	encapNhSize         = 2    // 8
	decapIPv4Count      = 48   // mixed prefix decap entries
	decapIPv4ScaleCount = 1000 // 1000 /32 prefix decap entries
	aftProgTimeout      = 10 * time.Minute
)

var (
	IPBlockEncapA       = "138.1.1.1/16" // IPBlockEncapA represents the ipv4 entries in EncapVRFA
	IPBlockEncapB       = "138.2.1.1/16" // IPBlockEncapB represents the ipv4 entries in EncapVRFB
	IPBlockEncapC       = "138.3.1.1/16" // IPBlockEncapC represents the ipv4 entries in EncapVRFC
	IPBlockEncapD       = "138.4.1.1/16" // IPBlockEncapD represents the ipv4 entries in EncapVRFD
	IPBlockDecap        = "102.0.0.1/15" // IPBlockDecap represents the ipv4 entries in Decap VRF
	IPv6BlockEncapA     = "2001:DB8:0:1::/64"
	IPv6BlockEncapB     = "2001:DB8:1:1::/64"
	IPv6BlockEncapC     = "2001:DB8:2:1::/64"
	IPv6BlockEncapD     = "2001:DB8:3:1::/64"
	lastNhIndex     int = 50000
	lastNhgIndex    int

	encapVrfAIPv4Enries = iputil.GenerateIPs(IPBlockEncapA, encapIPv4Count)
	encapVrfBIPv4Enries = iputil.GenerateIPs(IPBlockEncapB, encapIPv4Count)
	encapVrfCIPv4Enries = iputil.GenerateIPs(IPBlockEncapC, encapIPv4Count)
	encapVrfDIPv4Enries = iputil.GenerateIPs(IPBlockEncapD, encapIPv4Count)

	encapVrfAIPv6Enries = createIPv6Entries(IPv6BlockEncapA, encapIPv6Count)
	encapVrfBIPv6Enries = createIPv6Entries(IPv6BlockEncapB, encapIPv6Count)
	encapVrfCIPv6Enries = createIPv6Entries(IPv6BlockEncapC, encapIPv6Count)
	encapVrfDIPv6Enries = createIPv6Entries(IPv6BlockEncapD, encapIPv6Count)
)

// IPPool for IPs
type IPPool struct {
	ips   []string
	index int
	rw    sync.RWMutex
}

// NewIPPool creates a new IPPool
func NewIPPool(entries []string) *IPPool {
	return &IPPool{
		ips:   entries,
		index: -1,
	}
}

// NextIP returns the next IP
func (p *IPPool) NextIP() string {
	p.rw.Lock()
	defer p.rw.Unlock()

	p.index++
	return p.ips[p.index]
}

// AllIPs returns all IPs in the pool
func (p *IPPool) AllIPs() []string {
	return append([]string{}, p.ips...)
}

// IDPool for NH and NHG IDs
type IDPool struct {
	nhIndex  uint64
	nhgIndex uint64
	rw       sync.RWMutex
}

// NewIDPool creates a new IDPool
func NewIDPool(base uint64) *IDPool {
	return &IDPool{
		nhIndex:  base,
		nhgIndex: base,
	}
}

// NextNHID returns the next NHID
func (p *IDPool) NextNHID() uint64 {
	p.rw.Lock()
	defer p.rw.Unlock()

	p.nhIndex++
	return p.nhIndex
}

// NextNHGID returns the next NHGID
func (p *IDPool) NextNHGID() uint64 {
	p.rw.Lock()
	defer p.rw.Unlock()

	p.nhgIndex++
	return p.nhgIndex
}

// VRFConfig holds NH, NHG and IPv4 entries for the VRF.
type VRFConfig struct {
	Name      string
	NHs       []fluent.GRIBIEntry
	NHGs      []fluent.GRIBIEntry
	V4Entries []fluent.GRIBIEntry
	Pairs     []PairedEntries
}

// PairedEntries holds NHs, NHGs and IPv4 entries for the VRF.
type PairedEntries struct {
	NHs        []fluent.GRIBIEntry
	NHGs       []fluent.GRIBIEntry
	V4Entries  []fluent.GRIBIEntry
	V6Entries  []fluent.GRIBIEntry
	V4Prefixes []string
	V6Prefixes []string
}

// IPPool for IPs
type PEntriesIndex struct {
	index int
	rw    sync.RWMutex
}

// NewIPPool creates a new IPPool
func NewPEntriesIndex() *PEntriesIndex {
	return &PEntriesIndex{
		index: -1,
	}
}

func (p *PEntriesIndex) NextIndex() int {
	p.rw.Lock()
	defer p.rw.Unlock()

	p.index++
	return p.index
}

func NewPairedEntry() *PairedEntries {
	return &PairedEntries{}
}

// FetchNonEmptyPairedEntries fetches all GRIBI entries from PairedEntries if their elements have a length greater than 0.
func FetchNonEmptyPairedEntries(pairedEntries []PairedEntries) []fluent.GRIBIEntry {
	var nonEmptyEntries []fluent.GRIBIEntry

	for _, entry := range pairedEntries {
		// Check and append non-empty NHs
		if len(entry.NHs) > 0 {
			nonEmptyEntries = append(nonEmptyEntries, entry.NHs...)
		}

		// Check and append non-empty NHGs
		if len(entry.NHGs) > 0 {
			nonEmptyEntries = append(nonEmptyEntries, entry.NHGs...)
		}

		// Check and append non-empty V4Entries
		if len(entry.V4Entries) > 0 {
			nonEmptyEntries = append(nonEmptyEntries, entry.V4Entries...)
		}

		// Check and append non-empty V6Entries
		if len(entry.V6Entries) > 0 {
			nonEmptyEntries = append(nonEmptyEntries, entry.V6Entries...)
		}
	}

	return nonEmptyEntries
}

// Param TE holds scale parameters.
type ScaleParam struct {
	V4TunnelCount         int
	V4TunnelNHGCount      int
	V4TunnelNHGSplitCount int
	EgressNHGSplitCount   int
	V4ReEncapNHGCount     int
}

// routesParam holds parameters required for provisioning
// gRIBI IP entries, next-hop-groups and next-hops
type routesParam struct {
	ipEntries     []string
	ipv6Entries   []string
	prefixVRF     string
	numUniqueNHs  int
	nextHops      []string //set of next hop prefixes
	nextHopVRF    string
	nextHopType   string // default, encap, decap, decapEncap
	startNHIndex  int
	numUniqueNHGs int
	numNHPerNHG   int
	startNHGIndex int
	nextHopWeight []int
	backupNHG     int
	tunnelSrcIP   string //
}

// BuildVRFConfig creates scale new scale VRF configurations.
func buildGRIBIProgramming(dut *ondatra.DUTDevice, egressIPs []string, param ScaleParam, l1Weight, l2Weight, l3Weight uint64) []*VRFConfig {
	v4TunnelIPAddrs := NewIPPool(iputil.GenerateIPs(V4TunnelIPBlock, param.V4TunnelCount))
	v4VIPAddrs := NewIPPool(iputil.GenerateIPs(V4VIPIPBlock, (param.V4TunnelNHGCount*param.V4TunnelNHGSplitCount)+2))
	v4EgressIPAddrs := NewIPPool(egressIPs)

	defaultVRF := deviations.DefaultNetworkInstance(dut)
	vrfTConf := &VRFConfig{Name: VRFT}
	vrfRConf := &VRFConfig{Name: VRFR}
	vrfRDConf := &VRFConfig{Name: VRFRD}
	vrfDefault := &VRFConfig{Name: defaultVRF}
	idPool := NewIDPool(10000)

	// VRF_T:

	nhgID := idPool.NextNHGID()
	nhID := idPool.NextNHID()
	nhgRedirectToVrfR := nhgID
	// build backup NHG and NH.
	vrfDefault.NHs = append(vrfDefault.NHs,
		fluent.NextHopEntry().WithIndex(nhID).WithNetworkInstance(defaultVRF).WithNextHopNetworkInstance(VRFR),
	)
	vrfDefault.NHGs = append(vrfDefault.NHGs,
		fluent.NextHopGroupEntry().WithID(nhgRedirectToVrfR).AddNextHop(nhID, 1).WithNetworkInstance(defaultVRF),
	)

	// Build IPv4 entry and related NHGs and NHs.
	// * Mapping tunnel IP per the IP -> NHG ratio
	// * Each NHG has unique NHs.
	// * Each NHG has the same backup to Repair VRF.
	tunnelNHGRatio := param.V4TunnelCount / param.V4TunnelNHGCount
	for idx, ip := range v4TunnelIPAddrs.AllIPs() {
		pe := NewPairedEntry()
		if idx%tunnelNHGRatio == 0 {
			nhgID = idPool.NextNHGID()
			nhgEntry := fluent.NextHopGroupEntry().WithID(nhgID).WithNetworkInstance(defaultVRF).WithBackupNHG(nhgRedirectToVrfR)
			// Build NHs and link NHs to NHG.
			for i := 0; i < param.V4TunnelNHGSplitCount; i++ {
				vip := v4VIPAddrs.NextIP()
				nhID = idPool.NextNHID()
				vrfDefault.NHs = append(vrfDefault.NHs,
					fluent.NextHopEntry().WithIndex(nhID).WithNetworkInstance(defaultVRF).WithIPAddress(vip),
				)
				pe.NHs = append(pe.NHs, fluent.NextHopEntry().WithIndex(nhID).WithNetworkInstance(defaultVRF).WithIPAddress(vip))
				var weight uint64
				if i == 0 {
					weight = 1
				} else {
					weight = l2Weight
				}
				nhgEntry = nhgEntry.AddNextHop(nhID, weight)
			}
			vrfDefault.NHGs = append(vrfDefault.NHGs, nhgEntry)
			pe.NHGs = append(pe.NHGs, nhgEntry)
		}

		// Build IPv4 entry
		vrfTConf.V4Entries = append(vrfTConf.V4Entries,
			fluent.IPv4Entry().WithPrefix(ip+"/32").WithNextHopGroup(nhgID).WithNetworkInstance(VRFT).WithNextHopGroupNetworkInstance(defaultVRF),
		)
		pe.V4Entries = append(pe.V4Entries, fluent.IPv4Entry().WithPrefix(ip+"/32").WithNextHopGroup(nhgID).WithNetworkInstance(VRFT).WithNextHopGroupNetworkInstance(defaultVRF))
		vrfTConf.Pairs = append(vrfTConf.Pairs, *pe)
	}

	// Default VRF:

	// * each VIP 1:1 map to a NHG
	// * each NHG points to unique NHs
	fmt.Println("**** v4EgressIPAddrs.AllIPs():", v4EgressIPAddrs.AllIPs())
	for _, ip := range v4VIPAddrs.AllIPs() {
		nhgID := idPool.NextNHGID()
		nhgEntry := fluent.NextHopGroupEntry().WithID(nhgID).WithNetworkInstance(defaultVRF)
		// Build NHs and link NHs to NHG.
		for i := 0; i < param.EgressNHGSplitCount; i++ {
			vip := v4EgressIPAddrs.AllIPs()[i%len(v4EgressIPAddrs.AllIPs())] // round-robin if not enough egress IPs
			nhID = idPool.NextNHID()
			vrfDefault.NHs = append(vrfDefault.NHs,
				fluent.NextHopEntry().WithIndex(nhID).WithNetworkInstance(defaultVRF).WithIPAddress(vip),
			)
			var weight uint64
			if i == 0 {
				weight = 1
			} else {
				weight = l3Weight
			}
			nhgEntry = nhgEntry.AddNextHop(nhID, weight)
		}

		vrfDefault.NHGs = append(vrfDefault.NHGs, nhgEntry)
		// Build IPv4 entry
		vrfDefault.V4Entries = append(vrfDefault.V4Entries,
			fluent.IPv4Entry().WithPrefix(ip+"/32").WithNextHopGroup(nhgID).WithNetworkInstance(defaultVRF).WithNextHopGroupNetworkInstance(defaultVRF),
		)
	}

	// VRF_R

	// build backup NHG and NH.
	nhID = idPool.NextNHID()
	nhgID = idPool.NextNHGID()
	nhgDecapToDefault := nhgID
	vrfDefault.NHs = append(vrfDefault.NHs,
		fluent.NextHopEntry().WithIndex(nhID).WithDecapsulateHeader(fluent.IPinIP).WithNetworkInstance(defaultVRF).WithNextHopNetworkInstance(defaultVRF),
	)
	vrfDefault.NHGs = append(vrfDefault.NHGs,
		fluent.NextHopGroupEntry().WithID(nhgID).AddNextHop(nhID, 1).WithNetworkInstance(defaultVRF),
	)

	// build IP entries and related NHG and NHs.
	// * Each NHG 1:1 mapping to NH
	// * Each NH has one entry for decap and encap
	// * All NHG has a backup for decap then goto default VRF.
	reEncapNHGRatio := param.V4TunnelCount / param.V4ReEncapNHGCount
	nhgID = idPool.NextNHGID()
	nhgEntry := fluent.NextHopGroupEntry().WithID(nhgID).WithNetworkInstance(defaultVRF).WithBackupNHG(nhgDecapToDefault)
	for idx, ip := range v4TunnelIPAddrs.AllIPs() {
		nhID = idPool.NextNHID()
		vrfDefault.NHs = append(vrfDefault.NHs,
			fluent.NextHopEntry().WithIndex(nhID).WithDecapsulateHeader(fluent.IPinIP).WithEncapsulateHeader(fluent.IPinIP).
				WithNetworkInstance(defaultVRF).WithIPinIP(tunnelSrcIP, v4TunnelIPAddrs.AllIPs()[(idx+1)%len(v4TunnelIPAddrs.AllIPs())]),
		)
		if idx != 0 && idx%reEncapNHGRatio == 0 {
			vrfDefault.NHGs = append(vrfDefault.NHGs, nhgEntry)
			nhgID = idPool.NextNHGID()
			nhgEntry = fluent.NextHopGroupEntry().WithID(nhgID).WithNetworkInstance(defaultVRF).WithBackupNHG(nhgDecapToDefault)
		}
		nhgEntry = nhgEntry.AddNextHop(nhID, 1)
		vrfRConf.V4Entries = append(vrfRConf.V4Entries,
			fluent.IPv4Entry().WithPrefix(ip+"/32").WithNextHopGroup(nhgID).WithNetworkInstance(VRFR).WithNextHopGroupNetworkInstance(defaultVRF),
		)
	}
	vrfDefault.NHGs = append(vrfDefault.NHGs, nhgEntry)

	v4VIPAddrs = NewIPPool(iputil.GenerateIPs(V4VIPIPBlock, (param.V4TunnelNHGCount*param.V4TunnelNHGSplitCount)+2))

	// VRF_RP

	// * do the same as Transit VRF
	// * but with decap to default NHG
	for idx, ip := range v4TunnelIPAddrs.AllIPs() {
		if idx%tunnelNHGRatio == 0 {
			nhgID = idPool.NextNHGID()
			nhgEntry := fluent.NextHopGroupEntry().WithID(nhgID).WithNetworkInstance(defaultVRF).WithBackupNHG(nhgRedirectToVrfR)

			// Build NHs and link NHs to NHG.
			for i := 0; i < param.V4TunnelNHGSplitCount; i++ {
				vip := v4VIPAddrs.NextIP()
				nhID = idPool.NextNHID()
				vrfDefault.NHs = append(vrfDefault.NHs,
					fluent.NextHopEntry().WithIndex(nhID).WithNetworkInstance(defaultVRF).WithIPAddress(vip),
				)
				nhgEntry = nhgEntry.AddNextHop(nhID, 1)
			}
			vrfDefault.NHGs = append(vrfDefault.NHGs, nhgEntry)
		}

		// Build IPv4 entry
		vrfRDConf.V4Entries = append(vrfRDConf.V4Entries,
			fluent.IPv4Entry().WithPrefix(ip+"/32").WithNextHopGroup(nhgID).WithNetworkInstance(VRFRD).WithNextHopGroupNetworkInstance(defaultVRF),
		)
	}

	return []*VRFConfig{vrfDefault, vrfTConf, vrfRConf, vrfRDConf}
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
	vrfEntryParams[vrfEncapA] = &routesParam{
		ipEntries:     encapVrfAIPv4Enries,
		ipv6Entries:   encapVrfAIPv6Enries,
		numUniqueNHs:  encapNhgcount * encapNhSize,
		nextHops:      tunnelIPs,
		nextHopVRF:    vrfTransit,
		startNHIndex:  lastNhIndex + 1,
		numUniqueNHGs: encapNhgcount,
		numNHPerNHG:   8,
		nextHopWeight: generateNextHopWeights(16, 8),
		startNHGIndex: lastNhgIndex + 1,
		tunnelSrcIP:   ipv4OuterSrc111,
	}

	lastNhIndex = vrfEntryParams[vrfEncapA].startNHIndex + vrfEntryParams[vrfEncapA].numUniqueNHs
	lastNhgIndex = vrfEntryParams[vrfEncapA].startNHGIndex + vrfEntryParams[vrfEncapA].numUniqueNHGs

	// Add 5k entries in ENCAP-VRF-B.
	vrfEntryParams[vrfEncapB] = &routesParam{
		ipEntries:     encapVrfBIPv4Enries,
		ipv6Entries:   encapVrfBIPv6Enries,
		numUniqueNHs:  encapNhgcount * encapNhSize,
		nextHops:      tunnelIPs,
		nextHopVRF:    vrfTransit,
		startNHIndex:  lastNhIndex + 1,
		numUniqueNHGs: encapNhgcount,
		numNHPerNHG:   8,
		nextHopWeight: generateNextHopWeights(16, 8),
		startNHGIndex: lastNhgIndex + 1,
		tunnelSrcIP:   ipv4OuterSrc222,
	}

	lastNhIndex = vrfEntryParams[vrfEncapB].startNHIndex + vrfEntryParams[vrfEncapB].numUniqueNHs
	lastNhgIndex = vrfEntryParams[vrfEncapB].startNHGIndex + vrfEntryParams[vrfEncapB].numUniqueNHGs

	// Add 5k entries in ENCAP-VRF-C
	vrfEntryParams[vrfEncapC] = &routesParam{
		ipEntries:     encapVrfCIPv4Enries,
		ipv6Entries:   encapVrfCIPv6Enries,
		numUniqueNHs:  encapNhgcount * encapNhSize,
		nextHops:      tunnelIPs,
		nextHopVRF:    vrfTransit,
		startNHIndex:  lastNhIndex + 1,
		numUniqueNHGs: encapNhgcount,
		numNHPerNHG:   8,
		nextHopWeight: generateNextHopWeights(16, 8),
		startNHGIndex: lastNhgIndex + 1,
		tunnelSrcIP:   ipv4OuterSrc111,
	}

	lastNhIndex = vrfEntryParams[vrfEncapC].startNHIndex + vrfEntryParams[vrfEncapC].numUniqueNHs
	lastNhgIndex = vrfEntryParams[vrfEncapC].startNHGIndex + vrfEntryParams[vrfEncapC].numUniqueNHGs

	// Add 5k entries in ENCAP-VRF-D
	vrfEntryParams[vrfEncapD] = &routesParam{
		ipEntries:     encapVrfDIPv4Enries,
		ipv6Entries:   encapVrfDIPv6Enries,
		numUniqueNHs:  encapNhgcount * encapNhSize,
		nextHops:      tunnelIPs,
		nextHopVRF:    vrfTransit,
		startNHIndex:  lastNhIndex + 1,
		numUniqueNHGs: encapNhgcount,
		numNHPerNHG:   8,
		nextHopWeight: generateNextHopWeights(16, 8),
		startNHGIndex: lastNhgIndex + 1,
		tunnelSrcIP:   ipv4OuterSrc222,
	}

	lastNhIndex = vrfEntryParams[vrfEncapD].startNHIndex + vrfEntryParams[vrfEncapD].numUniqueNHs
	lastNhgIndex = vrfEntryParams[vrfEncapD].startNHGIndex + vrfEntryParams[vrfEncapD].numUniqueNHGs

	for _, vrf := range []string{vrfEncapA, vrfEncapB, vrfEncapC, vrfEncapD} {
		t.Logf("installing v4 entries in %s", vrf)
		installEncapEntries(t, vrf, vrfEntryParams[vrf], args)
	}
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
	if err := awaitTimeout(args.ctx, args.client, t, aftProgTimeout); err != nil {
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
	if err := awaitTimeout(args.ctx, args.client, t, aftProgTimeout); err != nil {
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
	if err := awaitTimeout(args.ctx, args.client, t, aftProgTimeout); err != nil {
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
	if err := awaitTimeout(args.ctx, args.client, t, aftProgTimeout); err != nil {
		t.Fatalf("Could not program entries via client, got err: %v", err)
	}
	t.Logf("Installed entries VRF %s - IPv6 entry count: %d, next-hop-group count: %d (index %d - %d), next-hop count: %d (index %d - %d)", vrf, len(routeParams.ipv6Entries), len(nextHopGroupIndices), nextHopGroupIndices[0], nextHopGroupIndices[len(nextHopGroupIndices)-1], len(nextHopIndices), nextHopIndices[0], nextHopIndices[len(nextHopIndices)-1])
}

// FetchEntriesForRouteParams retrieves all IPv4Entry, NextHopGroupEntry, and NextHopEntry for a given routesParam.
func FetchEntriesForRouteParams(routeParams *routesParam, dut *ondatra.DUTDevice) ([]fluent.GRIBIEntry, []fluent.GRIBIEntry, []fluent.GRIBIEntry) {
	var ipv4Entries []fluent.GRIBIEntry
	var nhgEntries []fluent.GRIBIEntry
	var nhEntries []fluent.GRIBIEntry

	// Generate NextHop entries
	for i := 0; i < routeParams.numUniqueNHs; i++ {
		index := uint64(routeParams.startNHIndex + i)
		nhEntry := fluent.NextHopEntry().
			WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(index).
			WithIPinIP(routeParams.tunnelSrcIP, routeParams.nextHops[i%len(routeParams.nextHops)]).
			WithEncapsulateHeader(fluent.IPinIP).
			WithNextHopNetworkInstance(routeParams.nextHopVRF)
		nhEntries = append(nhEntries, nhEntry)
	}

	// Generate NextHopGroup entries
	for i := 0; i < routeParams.numUniqueNHGs; i++ {
		index := uint64(routeParams.startNHGIndex + i)
		nhgEntry := fluent.NextHopGroupEntry().
			WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(index)
		if routeParams.backupNHG != 0 {
			nhgEntry.WithBackupNHG(uint64(routeParams.backupNHG))
		}
		for j := 0; j < routeParams.numNHPerNHG; j++ {
			nhgEntry.AddNextHop(uint64(routeParams.startNHIndex+(i*routeParams.numNHPerNHG+j)%routeParams.numUniqueNHs), uint64(routeParams.nextHopWeight[j]))
		}
		nhgEntries = append(nhgEntries, nhgEntry)
	}

	// Generate IPv4 entries
	for i, ip := range routeParams.ipEntries {
		ipv4Entry := fluent.IPv4Entry().
			WithPrefix(ip + "/32").
			WithNetworkInstance(routeParams.nextHopVRF).
			WithNextHopGroup(uint64(routeParams.startNHGIndex + i%routeParams.numUniqueNHGs)).
			WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut))
		ipv4Entries = append(ipv4Entries, ipv4Entry)
	}

	return ipv4Entries, nhgEntries, nhEntries
}

// FetchEntriesForRouteParamsInBatches retrieves IPv4Entry, NextHopGroupEntry, and NextHopEntry for a given routesParam
// and divides them into batches based on the specified batch count.
func FetchEntriesForRouteParamsInBatches(routeParams *routesParam, dut *ondatra.DUTDevice, batchCount int) []PairedEntries {
	var pairedEntries []PairedEntries

	// Calculate the batch size dynamically based on the total number of ipEntries and batchCount
	totalEntries := len(routeParams.ipEntries)
	batchSize := (totalEntries + batchCount - 1) / batchCount // Round up to ensure all entries are included

	for batch := 0; batch < batchCount; batch++ {
		startIndex := batch * batchSize
		endIndex := startIndex + batchSize
		if endIndex > totalEntries {
			endIndex = totalEntries
		}

		// Create a new PairedEntries for this batch
		pe := PairedEntries{}

		// Generate NextHop entries for this batch
		for i := 0; i < routeParams.numUniqueNHs; i++ {
			index := uint64(routeParams.startNHIndex + i)
			nhEntry := fluent.NextHopEntry().
				WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithIndex(index).
				WithIPinIP(routeParams.tunnelSrcIP, routeParams.nextHops[i%len(routeParams.nextHops)]).
				WithEncapsulateHeader(fluent.IPinIP).
				WithNextHopNetworkInstance(routeParams.nextHopVRF)
			pe.NHs = append(pe.NHs, nhEntry)
		}

		// Generate NextHopGroup entries for this batch
		for i := 0; i < routeParams.numUniqueNHGs; i++ {
			index := uint64(routeParams.startNHGIndex + i)
			nhgEntry := fluent.NextHopGroupEntry().
				WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithID(index)
			if routeParams.backupNHG != 0 {
				nhgEntry.WithBackupNHG(uint64(routeParams.backupNHG))
			}
			for j := 0; j < routeParams.numNHPerNHG; j++ {
				nhgEntry.AddNextHop(uint64(routeParams.startNHIndex+(i*routeParams.numNHPerNHG+j)%routeParams.numUniqueNHs), uint64(routeParams.nextHopWeight[j]))
			}
			pe.NHGs = append(pe.NHGs, nhgEntry)
		}

		// Generate IPv4 entries for this batch
		for _, ip := range routeParams.ipEntries[startIndex:endIndex] {
			ipv4Entry := fluent.IPv4Entry().
				WithPrefix(ip + "/32").
				WithNetworkInstance(routeParams.nextHopVRF).
				WithNextHopGroup(uint64(routeParams.startNHGIndex + startIndex%routeParams.numUniqueNHGs)).
				WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut))
			pe.V4Entries = append(pe.V4Entries, ipv4Entry)
			pe.V4Prefixes = append(pe.V4Prefixes, ip)
		}

		// Generate IPv6 entries for this batch
		if len(routeParams.ipv6Entries) > 0 {
			for _, ip := range routeParams.ipv6Entries[startIndex:endIndex] {
				ipv6Entry := fluent.IPv6Entry().
					WithPrefix(ip + "/128").
					WithNetworkInstance(routeParams.nextHopVRF).
					WithNextHopGroup(uint64(routeParams.startNHGIndex + startIndex%routeParams.numUniqueNHGs)).
					WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut))
				pe.V6Entries = append(pe.V6Entries, ipv6Entry)
				pe.V6Prefixes = append(pe.V6Prefixes, ip)
			}
		}

		// Add the PairedEntries for this batch to the result
		pairedEntries = append(pairedEntries, pe)
	}

	return pairedEntries
}

func GetFibSegmentGribiEntries(routeParams *routesParam, dut *ondatra.DUTDevice, batchCount int) []PairedEntries {
	var pairedEntries []PairedEntries

	// Calculate the batch size dynamically based on the total number of ipEntries and batchCount
	totalEntries := len(routeParams.ipEntries)
	batchSize := (totalEntries + batchCount - 1) / batchCount // Round up to ensure all entries are included

	// Calculate the batch-specific ranges for nhIndex and nhgIndex
	nhsPerBatch := routeParams.numUniqueNHs / batchCount
	nhgsPerBatch := routeParams.numUniqueNHGs / batchCount
	nextHopsPerBatch := len(routeParams.nextHops) / batchCount

	// If nextHops are fewer than the batch size, allow all batches to reuse the same nextHops
	if len(routeParams.nextHops) < batchSize {
		nextHopsPerBatch = len(routeParams.nextHops)
	}

	prefixToNHGRatio := len(routeParams.ipEntries) / routeParams.numUniqueNHGs

	for batch := 0; batch < batchCount; batch++ {
		startIndex := batch * batchSize
		endIndex := startIndex + batchSize
		if endIndex > totalEntries {
			endIndex = totalEntries
		}

		// Create a new PairedEntries for this batch
		pe := PairedEntries{}

		// Calculate the batch-specific base indices for nhIndex and nhgIndex
		baseNHIndex := uint64(routeParams.startNHIndex + batch*nhsPerBatch)
		baseNHGIndex := uint64(routeParams.startNHGIndex + batch*nhgsPerBatch)

		// Calculate the batch-specific nextHops
		var batchNextHops []string
		if len(routeParams.nextHops) < batchSize {
			batchNextHops = routeParams.nextHops // Reuse the same nextHops for all batches
		} else {
			batchNextHops = routeParams.nextHops[batch*nextHopsPerBatch : (batch+1)*nextHopsPerBatch]
		}

		currentNHGIndex := baseNHGIndex
		// Generate IPv4 entries, NextHopGroup entries, and NextHop entries in the same loop
		for i, ip := range routeParams.ipEntries[startIndex:endIndex] {
			// Generate NextHopGroup entry
			// currentNHGIndex := baseNHGIndex + uint64(i%nhgsPerBatch)
			if i > 0 && i%prefixToNHGRatio == 0 {
				currentNHGIndex++
			}
			nhgEntry := fluent.NextHopGroupEntry().
				WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithID(currentNHGIndex)
			if routeParams.backupNHG != 0 {
				nhgEntry.WithBackupNHG(uint64(routeParams.backupNHG))
			}

			// Generate NextHop entries and add them to the NextHopGroup
			for j := 0; j < routeParams.numNHPerNHG; j++ {
				currentNHIndex := baseNHIndex + uint64((i*routeParams.numNHPerNHG+j)%nhsPerBatch)
				nhEntry := fluent.NextHopEntry().
					WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
					WithIndex(currentNHIndex).
					WithNextHopNetworkInstance(routeParams.nextHopVRF)
				if routeParams.nextHopType == "encap" {
					nhEntry.WithIPinIP(routeParams.tunnelSrcIP, batchNextHops[j%len(batchNextHops)]).
						WithEncapsulateHeader(fluent.IPinIP)
				} else if routeParams.nextHopType == "decap" {
					nhEntry.WithDecapsulateHeader(fluent.IPinIP)
				} else if routeParams.nextHopType == "decapEncap" {
					nhEntry.WithDecapsulateHeader(fluent.IPinIP).
						WithEncapsulateHeader(fluent.IPinIP).
						WithIPinIP(routeParams.tunnelSrcIP, batchNextHops[j%len(batchNextHops)])
				} else if routeParams.nextHopType == "default" {
					nhEntry.WithIPAddress(batchNextHops[j%len(batchNextHops)])
				}
				pe.NHs = append(pe.NHs, nhEntry)

				// Add the NextHop to the NextHopGroup
				nhgEntry.AddNextHop(currentNHIndex, uint64(routeParams.nextHopWeight[j]))
			}
			pe.NHGs = append(pe.NHGs, nhgEntry)

			// Generate IPv4 entry
			ipv4Entry := fluent.IPv4Entry().
				WithPrefix(ip + "/32").
				WithNetworkInstance(routeParams.prefixVRF).
				WithNextHopGroup(currentNHGIndex).
				WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut))
			pe.V4Entries = append(pe.V4Entries, ipv4Entry)
			pe.V4Prefixes = append(pe.V4Prefixes, ip)

			// Generate IPv6 entries for this batch
			if len(routeParams.ipv6Entries) > 0 {
				ip = routeParams.ipv6Entries[startIndex:endIndex][i]
				ipv6Entry := fluent.IPv6Entry().
					WithPrefix(ip + "/128").
					WithNetworkInstance(routeParams.prefixVRF).
					WithNextHopGroup(currentNHGIndex).
					WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut))
				pe.V6Entries = append(pe.V6Entries, ipv6Entry)
				pe.V6Prefixes = append(pe.V6Prefixes, ip)
			}

		}

		// Add the PairedEntries for this batch to the result
		pairedEntries = append(pairedEntries, pe)
	}

	return pairedEntries
}

func TestChains(t *testing.T) {
	// type routesParam struct {
	// 	ipEntries     []string
	// 	ipv6Entries   []string
	// 	prefixVRF     string
	// 	numUniqueNHs  int
	// 	nextHops      []string //set of next hop prefixes
	// 	nextHopVRF    string
	// 	nextHopType   string // default, encap, decap, decapEncap
	// 	startNHIndex  int
	// 	numUniqueNHGs int
	// 	numNHPerNHG   int
	// 	startNHGIndex int
	// 	nextHopWeight []int
	// 	backupNHG     int
	// 	tunnelSrcIP   string //
	// }
	encapSegmentA := routesParam{
		ipEntries:     encapVrfAIPv4Enries[:80],
		ipv6Entries:   encapVrfAIPv6Enries[:80],
		prefixVRF:     vrfEncapA,
		numUniqueNHs:  320, //encapNhgcount * encapNhSize,
		nextHops:      encapVrfAIPv4Enries[400:500],
		nextHopVRF:    vrfTransit,
		nextHopType:   "encap",
		startNHIndex:  lastNhIndex + 1,
		numUniqueNHGs: 40, //encapNhgcount,
		numNHPerNHG:   8,
		nextHopWeight: generateNextHopWeights(16, 8),
		startNHGIndex: lastNhgIndex + 1,
		tunnelSrcIP:   ipv4OuterSrc111,
	}
	dut := ondatra.DUT(t, "dut")
	gribiInfo := GetFibSegmentGribiEntries(&encapSegmentA, dut, 8)
	for i, pe := range gribiInfo {
		t.Logf("Batch %d: IPv4 entries: %d, IPv6 entries: %d, NH entries: %d, NHG entries: %d", i, len(pe.V4Entries), len(pe.V6Entries), len(pe.NHs), len(pe.NHGs))
		// t.Logf("Batch %d\n: IPv4 entries: %v\n, IPv6 entries: %v\n, NH entries: %v\n, NHG entries: %v\n", i, pe.V4Entries, pe.V6Entries, pe.NHs, pe.NHGs)
		for j, nhg := range pe.NHGs {
			t.Logf("Batch %d, NHG %d: %v", i, j, nhg)
		}
		for j, nh := range pe.NHs {
			t.Logf("Batch %d, NH %d: %v", i, j, nh)
		}
		for j, ipv4 := range pe.V4Entries {
			t.Logf("Batch %d, IPv4 %d: %v", i, j, ipv4)
		}
		for j, ipv6 := range pe.V6Entries {
			t.Logf("Batch %d, IPv6 %d: %v", i, j, ipv6)
		}
		for j, ipv4 := range pe.V4Prefixes {
			t.Logf("Batch %d, IPv4 %d: %v", i, j, ipv4)
		}
		for j, ipv6 := range pe.V6Prefixes {
			t.Logf("Batch %d, IPv6 %d: %v", i, j, ipv6)
		}
	}
	// t.Logf("IPv4 entries: %d, IPv6 entries: %d, NH entries: %d, NHG entries: %d", len(gribiInfo.V4Entries), len(gribiInfo.V6Entries), len(gribiInfo.NHs), len(gribiInfo.NHGs))

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

func BaseGRIBIProgramming(t *testing.T, args *testArgs, egressIPs []string, param ScaleParam, l1Weight, l2Weight, l3Weight uint64) {

	args.client.StartSending(args.ctx, t)
	if err := awaitTimeout(args.ctx, args.client, t, time.Minute); err != nil {
		t.Fatalf("Await got error during session negotiation for client: %v", err)
	}
	args.electionID = gribi.BecomeLeader(t, args.client)
	vrfConfigs := buildGRIBIProgramming(args.dut, egressIPs, param, l1Weight, l2Weight, l3Weight)
	for _, vrfConfig := range vrfConfigs {
		// skip adding unwanted entries
		if vrfConfig.Name == "vrf_rd" {
			continue
		}
		entries := append(vrfConfig.NHs, vrfConfig.NHGs...)
		entries = append(entries, vrfConfig.V4Entries...)
		args.client.Modify().AddEntry(t, entries...)
		if err := awaitTimeout(args.ctx, args.client, t, aftProgTimeout); err != nil {
			t.Fatalf("Could not program entries, got err: %v", err)
		}
		t.Logf("Created %d NHs, %d NHGs, %d IPv4Entries in %s VRF", len(vrfConfig.NHs), len(vrfConfig.NHGs), len(vrfConfig.V4Entries), vrfConfig.Name)
	}

	// push encap entries
	defaultIpv4Entries := []string{}
	for _, v4Entry := range vrfConfigs[1].V4Entries {
		ep, _ := v4Entry.EntryProto()
		defaultIpv4Entries = append(defaultIpv4Entries, strings.Split(ep.GetIpv4().GetPrefix(), "/")[0])
	}

	// Inject 5000 IPv4Entry-ies and 5000 IPv6Entry-ies to each of the 4 encap VRFs.
	pushEncapEntries(t, defaultIpv4Entries, args)
	validateTrafficFlows(t, args, getEncapFlows(), false, true)

	// Inject mixed length prefixes (48 entries) in the DECAP_TE_VRF.
	decapEntries := pushDecapEntries(t, args)
	validateTrafficFlows(t, args, getDecapFlows(decapEntries), false, true)

	// Install decapIPv4ScaleCount entries with fixed prefix length of /32 in DECAP_TE_VRF.
	decapScaleEntries := iputil.GenerateIPs(IPBlockDecap, decapIPv4ScaleCount)
	pushDecapScaleEntries(t, args, decapScaleEntries)
	// Send traffic and verify packets are decapped then encapsulated and then forwarded to peer.
	validateTrafficFlows(t, args, getDecapFlows(decapScaleEntries), false, true)
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
	decapNH := fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
		WithIndex(nhIndex).WithDecapsulateHeader(fluent.IPinIP)
	if !deviations.DecapNHWithNextHopNIUnsupported(args.dut) {
		decapNH.WithNextHopNetworkInstance(deviations.DefaultNetworkInstance(args.dut))
	}
	args.client.Modify().AddEntry(t,
		decapNH,
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
			WithID(nhgIndex).AddNextHop(nhIndex, 1),
		fluent.IPv4Entry().WithNetworkInstance(niDecapTeVrf).
			WithPrefix(prefix).WithNextHopGroup(nhgIndex).
			WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(args.dut)),
	)
}
