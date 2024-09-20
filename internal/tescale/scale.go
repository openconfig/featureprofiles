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
package tescale

import (
	"sync"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/iputil"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
)

const (
	// VRFT vrf t
	VRFT = "vrf_t"
	// VRFR vrf r
	VRFR = "vrf_r"
	// VRFRD vrf rd
	VRFRD = "vrf_rd"

	// V4TunnelIPBlock tunnel IP block
	V4TunnelIPBlock = "198.18.0.1/16"
	// V4VIPIPBlock vip IP block
	V4VIPIPBlock = "198.18.196.1/22"

	tunnelSrcIP = "198.18.204.1"
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
}

// Param TE holds scale parameters.
type Param struct {
	V4TunnelCount         int
	V4TunnelNHGCount      int
	V4TunnelNHGSplitCount int
	EgressNHGSplitCount   int
	V4ReEncapNHGCount     int
}

// BuildVRFConfig creates scale new scale VRF configurations.
func BuildVRFConfig(dut *ondatra.DUTDevice, egressIPs []string, param Param) []*VRFConfig {
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
		vrfTConf.V4Entries = append(vrfTConf.V4Entries,
			fluent.IPv4Entry().WithPrefix(ip+"/32").WithNextHopGroup(nhgID).WithNetworkInstance(VRFT).WithNextHopGroupNetworkInstance(defaultVRF),
		)
	}

	// Default VRF:

	// * each VIP 1:1 map to a NHG
	// * each NHG points to unique NHs
	for _, ip := range v4VIPAddrs.AllIPs() {
		nhgID := idPool.NextNHGID()
		nhgEntry := fluent.NextHopGroupEntry().WithID(nhgID).WithNetworkInstance(defaultVRF)
		// Build NHs and link NHs to NHG.
		for i := 0; i < param.EgressNHGSplitCount; i++ {
			vip := v4EgressIPAddrs.AllIPs()[i]
			nhID = idPool.NextNHID()
			vrfDefault.NHs = append(vrfDefault.NHs,
				fluent.NextHopEntry().WithIndex(nhID).WithNetworkInstance(defaultVRF).WithIPAddress(vip),
			)
			nhgEntry = nhgEntry.AddNextHop(nhID, 1)
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
