// Copyright 2026 OpenConfig Authors
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

package hashing_test

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	plen          = 30
	plainSubnet   = "198.51.0.0/16"
	encapSubnet   = "172.16.0.0/16"
	vrfTransit    = "TRANSIT"
	vrfSelfSite   = "SELF_SITE"
	vrfEgress     = "EGRESS"
	ateIngressMAC = "02:00:00:00:00:02"
	ateEgressMAC  = "02:00:00:00:00:01"
)

func getPeerIP(t *testing.T, ipStr string) string {
	t.Helper()
	ip := net.ParseIP(ipStr)
	if ip == nil {
		t.Fatalf("Invalid IP address: %s", ipStr)
	}
	ipv4 := ip.To4()
	if ipv4 == nil {
		t.Fatalf("Not an IPv4 address: %s", ipStr)
	}
	last := ipv4[3]
	if last%2 == 1 {
		ipv4[3]++
	} else {
		ipv4[3]--
	}
	return ipv4.String()
}

func getMacForLagIndex(idx int) string {
	return fmt.Sprintf("02:00:00:00:01:%02x", idx)
}

func getLagName(dut *ondatra.DUTDevice, lagIndex int) string {
	switch dut.Vendor() {
	case ondatra.CISCO:
		return fmt.Sprintf("Bundle-Ether%d", lagIndex)
	case ondatra.ARISTA:
		return fmt.Sprintf("Port-Channel%d", lagIndex)
	case ondatra.JUNIPER:
		return fmt.Sprintf("ae%d", lagIndex)
	default:
		return fmt.Sprintf("lag%d", lagIndex)
	}
}

// Physical DUT Ports mapping for dut_8_loop_2_ate.testbed (18 physical ports)
var vrfPortMap = map[string]struct {
	ip           string
	loopbackMode oc.E_Interfaces_LoopbackModeType
}{
	// Ingress from ATE (ixia2)
	"lc2_p10": {ip: "192.0.2.1", loopbackMode: oc.Interfaces_LoopbackModeType_NONE},

	// Egress to ATE (ixia1)
	"lc2_p9": {ip: "192.0.2.5", loopbackMode: oc.Interfaces_LoopbackModeType_NONE},

	// Loop 1 (Stage 1 -> Stage 2 Transit)
	"lc1_p3": {ip: "192.0.2.9", loopbackMode: oc.Interfaces_LoopbackModeType_NONE},
	"lc2_p3": {ip: "192.0.2.10", loopbackMode: oc.Interfaces_LoopbackModeType_NONE},

	// Loop 2 (Stage 2 Transit -> Egress)
	"lc1_p4": {ip: "192.0.2.13", loopbackMode: oc.Interfaces_LoopbackModeType_NONE},
	"lc2_p4": {ip: "192.0.2.14", loopbackMode: oc.Interfaces_LoopbackModeType_NONE},

	// Loop 3 (Stage 2 Transit -> Egress)
	"lc1_p5": {ip: "192.0.2.17", loopbackMode: oc.Interfaces_LoopbackModeType_NONE},
	"lc2_p5": {ip: "192.0.2.18", loopbackMode: oc.Interfaces_LoopbackModeType_NONE},

	// Loop 4 (Stage 2 Transit -> Stage 3 Self-Site)
	"lc1_p6": {ip: "192.0.2.21", loopbackMode: oc.Interfaces_LoopbackModeType_NONE},
	"lc2_p6": {ip: "192.0.2.22", loopbackMode: oc.Interfaces_LoopbackModeType_NONE},

	// Loop 5 (Stage 2 Transit -> Stage 3 Self-Site)
	"lc1_p1": {ip: "192.0.2.25", loopbackMode: oc.Interfaces_LoopbackModeType_NONE},
	"lc2_p1": {ip: "192.0.2.26", loopbackMode: oc.Interfaces_LoopbackModeType_NONE},

	// Loop 6 (Stage 3 Self-Site -> Egress)
	"lc2_p8": {ip: "192.0.2.29", loopbackMode: oc.Interfaces_LoopbackModeType_NONE},
	"lc1_p8": {ip: "192.0.2.30", loopbackMode: oc.Interfaces_LoopbackModeType_NONE},

	// Loop 7 (Stage 3 Self-Site -> Egress)
	"lc2_p7": {ip: "192.0.2.33", loopbackMode: oc.Interfaces_LoopbackModeType_NONE},
	"lc1_p7": {ip: "192.0.2.34", loopbackMode: oc.Interfaces_LoopbackModeType_NONE},

	// Loop 8 (Stage 3 Self-Site -> Egress)
	"lc2_p2": {ip: "192.0.2.37", loopbackMode: oc.Interfaces_LoopbackModeType_NONE},
	"lc1_p2": {ip: "192.0.2.38", loopbackMode: oc.Interfaces_LoopbackModeType_NONE},
}

type softLoopInfo struct {
	physName string
	ip       string
	mac      string
}

func configureDUTVRF(t *testing.T, dut *ondatra.DUTDevice, softLoops []softLoopInfo, portToLagMap map[string]string, portToMacMap map[string]string) {
	t.Helper()
	d := gnmi.OC()
	defNI := deviations.DefaultNetworkInstance(dut)

	// 1. Populate Physical and LAG Interfaces atomically
	root := &oc.Root{}

	for portID, cfg := range vrfPortMap {
		p := dut.Port(t, portID)
		lagName := portToLagMap[p.Name()]

		// Populate LAG Interface
		lagIntf := root.GetOrCreateInterface(lagName)
		populateLAGInterface(dut, lagIntf, lagName, cfg.ip, plen, true)
		if cfg.loopbackMode != oc.Interfaces_LoopbackModeType_NONE && deviations.MemberLinkLoopbackUnsupported(dut) {
			lagIntf.LoopbackMode = oc.Interfaces_LoopbackModeType_TERMINAL
		}

		// Populate Physical Interface
		physIntf := root.GetOrCreateInterface(p.Name())
		populatePhysicalInterfaceForLAG(dut, physIntf, p.Name(), lagName, true, cfg.loopbackMode)
	}

	// Populate Soft Loops Interfaces
	for _, sl := range softLoops {
		lagName := portToLagMap[sl.physName]

		// Populate LAG Interface
		lagIntf := root.GetOrCreateInterface(lagName)
		populateLAGInterface(dut, lagIntf, lagName, sl.ip, plen, true)
		if deviations.MemberLinkLoopbackUnsupported(dut) {
			lagIntf.LoopbackMode = oc.Interfaces_LoopbackModeType_TERMINAL
		}

		// Populate Physical Interface
		physIntf := root.GetOrCreateInterface(sl.physName)
		populatePhysicalInterfaceForLAG(dut, physIntf, sl.physName, lagName, true, oc.Interfaces_LoopbackModeType_TERMINAL)
	}

	t.Log("Pushing atomic interface configuration...")
	gnmi.Update(t, dut, gnmi.OC().Config(), root)

	// Configure port speed if required
	if deviations.ExplicitPortSpeed(dut) {
		for portID := range vrfPortMap {
			p := dut.Port(t, portID)
			fptest.SetPortSpeed(t, p)
		}
	}

	// Configure Static ARP on Ingress and Egress LAG interfaces
	configureStaticARPIngressAndEgress(t, dut, portToLagMap)

	// 3. Configure Network Instances (VRFs)
	vrfs := []string{vrfTransit, vrfSelfSite, vrfEgress}
	for _, vrf := range vrfs {
		ni := &oc.NetworkInstance{
			Name: ygot.String(vrf),
			Type: oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF,
		}
		gnmi.Replace(t, dut, d.NetworkInstance(vrf).Config(), ni)
	}

	// 4. Assign LAG Interfaces to VRFs
	vrfAssignments := map[string]string{
		// Transit VRF: Loop 1 RX + Loops 2, 3, 4, 5 TX
		"lc2_p3": vrfTransit,
		"lc1_p4": vrfTransit,
		"lc1_p5": vrfTransit,
		"lc1_p6": vrfTransit,
		"lc1_p1": vrfTransit,

		// Self-Site VRF: Loops 4, 5 RX + Loops 6, 7, 8 TX
		"lc2_p6": vrfSelfSite,
		"lc2_p1": vrfSelfSite,
		"lc2_p8": vrfSelfSite,
		"lc2_p7": vrfSelfSite,
		"lc2_p2": vrfSelfSite,

		// Egress VRF: Loops 2, 3 RX + Loops 6, 7, 8 RX + Egress Port
		"lc2_p4": vrfEgress,
		"lc2_p5": vrfEgress,
		"lc1_p8": vrfEgress,
		"lc1_p7": vrfEgress,
		"lc1_p2": vrfEgress,
		"lc2_p9": vrfEgress,
	}

	for portID, vrfName := range vrfAssignments {
		p := dut.Port(t, portID)
		lagName := portToLagMap[p.Name()]
		fptest.AssignToNetworkInstance(t, dut, lagName, vrfName, 0)
	}

	// Assign remaining physical ports in vrfPortMap to default VRF if required (lc2_p10, lc1_p3)
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		for portID := range vrfPortMap {
			if _, assigned := vrfAssignments[portID]; !assigned {
				p := dut.Port(t, portID)
				lagName := portToLagMap[p.Name()]
				t.Logf("Explicitly assigning physical port %s (%s) to default VRF %s", portID, lagName, defNI)
				fptest.AssignToNetworkInstance(t, dut, lagName, defNI, 0)
			}
		}
	}

	// Assign soft loops LAGs to VRFs
	for i, sl := range softLoops {
		var vrfName string
		switch {
		case i >= 3 && i <= 6:
			vrfName = vrfTransit
		case i >= 7 && i <= 11:
			vrfName = vrfSelfSite
		default:
			// 0, 1, 2 stay in default VRF
			if deviations.ExplicitInterfaceInDefaultVRF(dut) {
				vrfName = defNI
			}
		}
		if vrfName != "" {
			lagName := portToLagMap[sl.physName]
			fptest.AssignToNetworkInstance(t, dut, lagName, vrfName, 0)
		}
	}

	// 5. Configure ACL Drop on RX for all soft loops
	var softLoopLags []string
	for _, sl := range softLoops {
		softLoopLags = append(softLoopLags, portToLagMap[sl.physName])
	}
	configureSoftLoopACLsPhys(t, dut, softLoopLags)
}

func programGRIBIVRF(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, gClient *gribi.Client, softLoops []softLoopInfo, portToLagMap map[string]string, portToMacMap map[string]string) {
	t.Helper()

	c := gClient.Fluent(t)
	defNI := deviations.DefaultNetworkInstance(dut)

	type nhDetail struct {
		txPortName string
		rxPortName string
		vrfName    string
	}

	portNHs := map[uint64]nhDetail{
		// Stage 1 NH (in Default VRF): Loop 1 TX (lc1_p3) -> Loop 1 RX (lc2_p3)
		101: {txPortName: "lc1_p3", rxPortName: "lc2_p3", vrfName: defNI},

		// Stage 2 NHs (in Transit VRF): Loops 2, 3, 4, 5
		201: {txPortName: "lc1_p4", rxPortName: "lc2_p4", vrfName: vrfTransit},
		202: {txPortName: "lc1_p5", rxPortName: "lc2_p5", vrfName: vrfTransit},
		203: {txPortName: "lc1_p6", rxPortName: "lc2_p6", vrfName: vrfTransit},
		204: {txPortName: "lc1_p1", rxPortName: "lc2_p1", vrfName: vrfTransit},

		// Stage 3 NHs (in Self-Site VRF): Loops 6, 7, 8
		301: {txPortName: "lc2_p8", rxPortName: "lc1_p8", vrfName: vrfSelfSite},
		302: {txPortName: "lc2_p7", rxPortName: "lc1_p7", vrfName: vrfSelfSite},
		303: {txPortName: "lc2_p2", rxPortName: "lc1_p2", vrfName: vrfSelfSite},

		// Egress VRF NH: DUT Egress lc2_p9 -> ATE ixia1
		401: {txPortName: "lc2_p9", rxPortName: "", vrfName: vrfEgress},
	}

	var entries []fluent.GRIBIEntry

	// Program physical Next Hops (referencing LAGs)
	for nhID, detail := range portNHs {
		p := dut.Port(t, detail.txPortName)
		cfg, ok := vrfPortMap[detail.txPortName]
		if !ok {
			t.Fatalf("Port %s not found in vrfPortMap", detail.txPortName)
		}
		lagName := portToLagMap[p.Name()]
		peerIP := getPeerIP(t, cfg.ip)
		var peerMac string
		if detail.rxPortName != "" {
			peerP := dut.Port(t, detail.rxPortName)
			peerMac = portToMacMap[peerP.Name()]
		} else {
			peerMac = ateEgressMAC
		}
		t.Logf("NH %d: Egress %s (port %s), Peer IP %s, Peer MAC %s", nhID, lagName, detail.txPortName, peerIP, peerMac)
		nh := fluent.NextHopEntry().
			WithNetworkInstance(detail.vrfName).
			WithIndex(nhID).
			WithIPAddress(peerIP).
			WithMacAddress(peerMac).
			WithInterfaceRef(lagName)
		entries = append(entries, nh)
	}

	// Program soft Next Hops (referencing LAGs)
	softNHIDs := []uint64{
		111, 112, 113, // Stage 1 (3 soft hops: Soft 0, 1, 2)
		211, 212, 213, 214, // Stage 2 (4 soft hops: Soft 3, 4, 5, 6)
		311, 312, 313, 314, 315, // Stage 3 (5 soft hops: Soft 7, 8, 9, 10, 11)
	}
	softVRFs := []string{
		defNI, defNI, defNI,
		vrfTransit, vrfTransit, vrfTransit, vrfTransit,
		vrfSelfSite, vrfSelfSite, vrfSelfSite, vrfSelfSite, vrfSelfSite,
	}

	for i, sl := range softLoops {
		nhID := softNHIDs[i]
		vrfName := softVRFs[i]
		lagName := portToLagMap[sl.physName]
		peerIP := getPeerIP(t, sl.ip)
		nh := fluent.NextHopEntry().
			WithNetworkInstance(vrfName).
			WithIndex(nhID).
			WithIPAddress(peerIP).
			WithMacAddress(sl.mac).
			WithInterfaceRef(lagName)
		entries = append(entries, nh)
	}

	// NHG 1: Default VRF (Ingress) -> Stage 1 WCMP (7:1:1:1)
	nhg1 := fluent.NextHopGroupEntry().WithNetworkInstance(defNI).WithID(1).
		AddNextHop(101, 7). // Loop 1 (lc1_p3)
		AddNextHop(111, 1). // Soft 0
		AddNextHop(112, 1). // Soft 1
		AddNextHop(113, 1)  // Soft 2
	entries = append(entries, nhg1)

	// NHG 2: Transit VRF -> Stage 2 ECMP (8-wide)
	nhg2 := fluent.NextHopGroupEntry().WithNetworkInstance(vrfTransit).WithID(2).
		AddNextHop(201, 1). // Loop 2 (lc1_p4)
		AddNextHop(202, 1). // Loop 3 (lc1_p5)
		AddNextHop(203, 1). // Loop 4 (lc1_p6)
		AddNextHop(204, 1). // Loop 5 (lc1_p1)
		AddNextHop(211, 1). // Soft 3
		AddNextHop(212, 1). // Soft 4
		AddNextHop(213, 1). // Soft 5
		AddNextHop(214, 1)  // Soft 6
	entries = append(entries, nhg2)

	// NHG 3: Self-Site VRF -> Stage 3 ECMP (8-wide)
	nhg3 := fluent.NextHopGroupEntry().WithNetworkInstance(vrfSelfSite).WithID(3).
		AddNextHop(301, 1). // Loop 6 (lc2_p8)
		AddNextHop(302, 1). // Loop 7 (lc2_p7)
		AddNextHop(303, 1). // Loop 8 (lc2_p2)
		AddNextHop(311, 1). // Soft 7
		AddNextHop(312, 1). // Soft 8
		AddNextHop(313, 1). // Soft 9
		AddNextHop(314, 1). // Soft 10
		AddNextHop(315, 1)  // Soft 11
	entries = append(entries, nhg3)

	// NHG 4: Egress VRF -> Egress Port lc2_p9
	nhg4 := fluent.NextHopGroupEntry().WithNetworkInstance(vrfEgress).WithID(4).
		AddNextHop(401, 1)
	entries = append(entries, nhg4)

	// Route definitions for Plain IP (198.51.0.0/16) and Encap (172.16.0.0/16)
	subnets := []string{plainSubnet, encapSubnet}
	for _, pfx := range subnets {
		entries = append(entries,
			fluent.IPv4Entry().WithNetworkInstance(defNI).WithPrefix(pfx).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(defNI),
			fluent.IPv4Entry().WithNetworkInstance(vrfTransit).WithPrefix(pfx).WithNextHopGroup(2).WithNextHopGroupNetworkInstance(vrfTransit),
			fluent.IPv4Entry().WithNetworkInstance(vrfSelfSite).WithPrefix(pfx).WithNextHopGroup(3).WithNextHopGroupNetworkInstance(vrfSelfSite),
			fluent.IPv4Entry().WithNetworkInstance(vrfEgress).WithPrefix(pfx).WithNextHopGroup(4).WithNextHopGroupNetworkInstance(vrfEgress),
		)
	}

	c.Modify().AddEntry(t, entries...)
	ctxTimeout, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if err := c.Await(ctxTimeout, t); err != nil {
		t.Fatalf("Error waiting to program gRIBI entries: %v", err)
	}
}

func TestHashing(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	// Perform cleanup of LAGs and VRFs before test
	cleanupDevice(t, dut)

	// Register test cleanup for gRIBI and DUT configuration
	gClient := &gribi.Client{
		DUT:         dut,
		FIBACK:      true,
		Persistence: true,
	}
	if err := gClient.Start(t); err != nil {
		t.Fatalf("Could not start gRIBI client: %v", err)
	}
	gClient.BecomeLeader(t)

	t.Cleanup(func() {
		t.Log("Flushing gRIBI entries and cleaning up DUT configuration...")
		if err := gribi.FlushAll(gClient.Fluent(t)); err != nil {
			t.Logf("Failed to flush gRIBI entries during cleanup: %v", err)
		}
		cleanupDevice(t, dut)
		gClient.Close(t)
	})

	// Discover 12 soft loops dynamically from unused equipped breakout interfaces
	var excludePorts []string
	for _, p := range dut.Ports() {
		excludePorts = append(excludePorts, p.Name())
	}

	discoveredPorts := discoverSoftLoops(t, dut, 12, excludePorts)
	cleanDiscoveredPorts(t, dut, discoveredPorts)

	softLoopIPs := []string{
		"192.0.2.41", "192.0.2.45", "192.0.2.49", // Stage 1 (0, 1, 2)
		"192.0.2.53", "192.0.2.57", "192.0.2.61", "192.0.2.65", // Stage 2 (3, 4, 5, 6)
		"192.0.2.69", "192.0.2.73", "192.0.2.77", "192.0.2.81", "192.0.2.85", // Stage 3 (7, 8, 9, 10, 11)
	}

	portToLagMap := make(map[string]string)
	portToMacMap := make(map[string]string)
	lagIndex := 101

	// Map physical ports to vendor-specific LAG names and unique MACs
	for portID := range vrfPortMap {
		p := dut.Port(t, portID)
		lagName := getLagName(dut, lagIndex)
		portToLagMap[p.Name()] = lagName
		portToMacMap[p.Name()] = getMacForLagIndex(lagIndex)
		t.Logf("Mapping physical port %s (%s) to %s (MAC: %s)", portID, p.Name(), lagName, portToMacMap[p.Name()])
		lagIndex++
	}

	var softLoops []softLoopInfo
	for i, phys := range discoveredPorts {
		lagName := getLagName(dut, lagIndex)
		mac := getMacForLagIndex(lagIndex)
		portToLagMap[phys] = lagName
		portToMacMap[phys] = mac
		softLoops = append(softLoops, softLoopInfo{
			physName: phys,
			ip:       softLoopIPs[i],
			mac:      mac,
		})
		t.Logf("Mapping soft port %d (%s) to %s (MAC: %s)", i, phys, lagName, mac)
		lagIndex++
	}

	configureDUTVRF(t, dut, softLoops, portToLagMap, portToMacMap)

	// Update physical ports in portToMacMap with actual router MACs from DUT telemetry
	for portID := range vrfPortMap {
		p := dut.Port(t, portID)
		lagName := portToLagMap[p.Name()]
		intfState := gnmi.Get(t, dut, gnmi.OC().Interface(lagName).State())
		if intfState.GetEthernet() != nil && intfState.GetEthernet().GetMacAddress() != "" {
			actualMac := intfState.GetEthernet().GetMacAddress()
			portToMacMap[p.Name()] = actualMac
			t.Logf("Physical port %s (%s) using real HW MAC: %s", portID, lagName, actualMac)
		}
	}

	configureStaticARP(t, dut, portToLagMap, portToMacMap, softLoops)

	ctx := context.Background()
	programGRIBIVRF(ctx, t, dut, gClient, softLoops, portToLagMap, portToMacMap)

	ate := ondatra.ATE(t, "ate")

	// Build verification port lists (Stage 1, Stage 2, Stage 3)
	// Stage 1: Loop 1 TX (lc1_p3) + Soft Loops 0, 1, 2
	stage1Port0 := dut.Port(t, "lc1_p3").Name()
	stage1Ports := []string{
		portToLagMap[stage1Port0],
		portToLagMap[softLoops[0].physName],
		portToLagMap[softLoops[1].physName],
		portToLagMap[softLoops[2].physName],
	}

	// Stage 2: Loops 2, 3, 4, 5 TX + Soft Loops 3, 4, 5, 6
	transitPorts := []string{
		portToLagMap[dut.Port(t, "lc1_p4").Name()],
		portToLagMap[dut.Port(t, "lc1_p5").Name()],
		portToLagMap[dut.Port(t, "lc1_p6").Name()],
		portToLagMap[dut.Port(t, "lc1_p1").Name()],
		portToLagMap[softLoops[3].physName],
		portToLagMap[softLoops[4].physName],
		portToLagMap[softLoops[5].physName],
		portToLagMap[softLoops[6].physName],
	}

	// Stage 3: Loops 6, 7, 8 TX + Soft Loops 7, 8, 9, 10, 11
	selfSitePorts := []string{
		portToLagMap[dut.Port(t, "lc2_p8").Name()],
		portToLagMap[dut.Port(t, "lc2_p7").Name()],
		portToLagMap[dut.Port(t, "lc2_p2").Name()],
		portToLagMap[softLoops[7].physName],
		portToLagMap[softLoops[8].physName],
		portToLagMap[softLoops[9].physName],
		portToLagMap[softLoops[10].physName],
		portToLagMap[softLoops[11].physName],
	}

	egressLagName := portToLagMap[dut.Port(t, "lc2_p9").Name()]

	allVerifyPorts := append(stage1Ports, transitPorts...)
	allVerifyPorts = append(allVerifyPorts, selfSitePorts...)
	allVerifyPorts = append(allVerifyPorts, egressLagName)

	ingressPortName := dut.Port(t, "lc2_p10").Name()
	ingressCounterPath := gnmi.OC().Interface(ingressPortName).Counters().InPkts().State()

	updateScenario1NHGs := func(unequalWeights bool) {
		var physWeight uint64 = 1
		if unequalWeights {
			physWeight = 2
		}

		// NHG 2: Transit VRF -> Stage 2 (8-wide)
		nhg2 := fluent.NextHopGroupEntry().WithNetworkInstance(vrfTransit).WithID(2).
			AddNextHop(201, physWeight). // Loop 2 (lc1_p4)
			AddNextHop(202, physWeight). // Loop 3 (lc1_p5)
			AddNextHop(203, physWeight). // Loop 4 (lc1_p6)
			AddNextHop(204, physWeight). // Loop 5 (lc1_p1)
			AddNextHop(211, 1).          // Soft 3
			AddNextHop(212, 1).          // Soft 4
			AddNextHop(213, 1).          // Soft 5
			AddNextHop(214, 1)           // Soft 6

		// NHG 3: Self-Site VRF -> Stage 3 (8-wide)
		nhg3 := fluent.NextHopGroupEntry().WithNetworkInstance(vrfSelfSite).WithID(3).
			AddNextHop(301, physWeight). // Loop 6 (lc2_p8)
			AddNextHop(302, physWeight). // Loop 7 (lc2_p7)
			AddNextHop(303, physWeight). // Loop 8 (lc2_p2)
			AddNextHop(311, 1).          // Soft 7
			AddNextHop(312, 1).          // Soft 8
			AddNextHop(313, 1).          // Soft 9
			AddNextHop(314, 1).          // Soft 10
			AddNextHop(315, 1)           // Soft 11

		c := gClient.Fluent(t)
		c.Modify().AddEntry(t, nhg2, nhg3)
		ctxTimeout, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		if err := c.Await(ctxTimeout, t); err != nil {
			t.Fatalf("Error updating NHGs: %v", err)
		}
	}

	runTrafficAndVerify := func(t *testing.T, profileName string, setupFlow func(flow gosnappi.Flow), unequalWeights bool) {
		t.Logf("=== Starting Traffic Profile: %s ===", profileName)
		otgConfig := configureOTGForFlow(t, ate, setupFlow)
		otgutils.WaitForARP(t, ate.OTG(), otgConfig, "IPv4")

		t.Log("Reading initial counters...")
		initialCounters := getEgressPacketsPhys(t, dut, allVerifyPorts)
		initialIngress := gnmi.Get(t, dut, ingressCounterPath)
		initialPhysCounters := getPhysicalPortCounters(t, dut)

		t.Log("Starting traffic...")
		ate.OTG().StartTraffic(t)

		sleepDuration := 45 * time.Second
		t.Logf("Waiting for %v to collect stats...", sleepDuration)
		time.Sleep(sleepDuration)

		logRuntimeDebug(t, dut, portToLagMap)

		t.Log("Reading final counters...")
		finalCounters := getEgressPacketsPhys(t, dut, allVerifyPorts)
		finalIngress := gnmi.Get(t, dut, ingressCounterPath)
		finalPhysCounters := getPhysicalPortCounters(t, dut)

		t.Logf("Ingress Port %s InPackets delta: %d", ingressPortName, finalIngress-initialIngress)

		t.Log("Logging Physical Port Deltas:")
		for portID := range vrfPortMap {
			p := dut.Port(t, portID)
			pName := p.Name()
			initC := initialPhysCounters[pName]
			finalC := finalPhysCounters[pName]

			inDelta := finalC.inPkts - initC.inPkts
			outDelta := finalC.outPkts - initC.outPkts
			if finalC.inPkts < initC.inPkts {
				inDelta = finalC.inPkts
			}
			if finalC.outPkts < initC.outPkts {
				outDelta = finalC.outPkts
			}
			t.Logf("  Port %s (%s): InPkts Delta = %d, OutPkts Delta = %d", portID, pName, inDelta, outDelta)
		}

		t.Log("Stopping traffic...")
		ate.OTG().StopTraffic(t)

		deltas := make(map[string]uint64)
		for _, lagName := range allVerifyPorts {
			initVal := initialCounters[lagName]
			finalVal := finalCounters[lagName]
			if finalVal >= initVal {
				deltas[lagName] = finalVal - initVal
			} else {
				deltas[lagName] = finalVal
			}
			t.Logf("DUT LAG %s OutPkts: Initial = %d, Final = %d, Delta = %d", lagName, initVal, finalVal, deltas[lagName])
		}

		// 1. Stage 1 Hashing (Default VRF WCMP: 7:1:1:1 ratio)
		t.Run("Stage 1: Ingress WCMP Hashing", func(t *testing.T) {
			p1_3 := dut.Port(t, "lc1_p3")
			expected := []expectedRatio{
				{portID: portToLagMap[p1_3.Name()], name: "Loop 1 (lc1_p3)", ratio: 0.70},
				{portID: portToLagMap[softLoops[0].physName], name: "Soft 0", ratio: 0.10},
				{portID: portToLagMap[softLoops[1].physName], name: "Soft 1", ratio: 0.10},
				{portID: portToLagMap[softLoops[2].physName], name: "Soft 2", ratio: 0.10},
			}
			verifyWCMPDistribution(t, "Default VRF (Stage 1)", deltas, expected)
		})

		// 2. Stage 2 Hashing (Transit VRF)
		t.Run("Stage 2: Transit VRF Hashing", func(t *testing.T) {
			if !unequalWeights {
				verifyDistribution(t, "TRANSIT", deltas, transitPorts, 0.1250)
			} else {
				expectedStage2 := []expectedRatio{
					{portID: portToLagMap[dut.Port(t, "lc1_p4").Name()], name: "Loop 2 (lc1_p4)", ratio: 2.0 / 12.0},
					{portID: portToLagMap[dut.Port(t, "lc1_p5").Name()], name: "Loop 3 (lc1_p5)", ratio: 2.0 / 12.0},
					{portID: portToLagMap[dut.Port(t, "lc1_p6").Name()], name: "Loop 4 (lc1_p6)", ratio: 2.0 / 12.0},
					{portID: portToLagMap[dut.Port(t, "lc1_p1").Name()], name: "Loop 5 (lc1_p1)", ratio: 2.0 / 12.0},
					{portID: portToLagMap[softLoops[3].physName], name: "Soft 3", ratio: 1.0 / 12.0},
					{portID: portToLagMap[softLoops[4].physName], name: "Soft 4", ratio: 1.0 / 12.0},
					{portID: portToLagMap[softLoops[5].physName], name: "Soft 5", ratio: 1.0 / 12.0},
					{portID: portToLagMap[softLoops[6].physName], name: "Soft 6", ratio: 1.0 / 12.0},
				}
				verifyWCMPDistribution(t, "TRANSIT (Stage 2 Unequal)", deltas, expectedStage2)
			}
		})

		// 3. Stage 3 Hashing (Self-Site VRF)
		t.Run("Stage 3: Self-Site VRF Hashing", func(t *testing.T) {
			if !unequalWeights {
				verifyDistribution(t, "SELF_SITE", deltas, selfSitePorts, 0.1250)
			} else {
				expectedStage3 := []expectedRatio{
					{portID: portToLagMap[dut.Port(t, "lc2_p8").Name()], name: "Loop 6 (lc2_p8)", ratio: 2.0 / 11.0},
					{portID: portToLagMap[dut.Port(t, "lc2_p7").Name()], name: "Loop 7 (lc2_p7)", ratio: 2.0 / 11.0},
					{portID: portToLagMap[dut.Port(t, "lc2_p2").Name()], name: "Loop 8 (lc2_p2)", ratio: 2.0 / 11.0},
					{portID: portToLagMap[softLoops[7].physName], name: "Soft 7", ratio: 1.0 / 11.0},
					{portID: portToLagMap[softLoops[8].physName], name: "Soft 8", ratio: 1.0 / 11.0},
					{portID: portToLagMap[softLoops[9].physName], name: "Soft 9", ratio: 1.0 / 11.0},
					{portID: portToLagMap[softLoops[10].physName], name: "Soft 10", ratio: 1.0 / 11.0},
					{portID: portToLagMap[softLoops[11].physName], name: "Soft 11", ratio: 1.0 / 11.0},
				}
				verifyWCMPDistribution(t, "SELF_SITE (Stage 3 Unequal)", deltas, expectedStage3)
			}
		})

		// 4. Egress Traffic Arrival Verification
		t.Run("Stage 4: Egress Traffic Arrival", func(t *testing.T) {
			egressDelta := deltas[egressLagName]
			t.Logf("DUT Egress LAG %s OutPkts delta: %d", egressLagName, egressDelta)
			if egressDelta == 0 {
				t.Errorf("DUT Egress LAG %s received 0 packets, expected egress traffic", egressLagName)
			}
		})
	}

	profiles := []struct {
		name       string
		flowSetter func(flow gosnappi.Flow)
	}{
		{
			name: "Plain_IPv4",
			flowSetter: func(flow gosnappi.Flow) {
				eth := flow.Packet().Add().Ethernet()
				eth.Src().SetValue(ateIngressMAC)
				ip := flow.Packet().Add().Ipv4()
				ip.Src().Increment().SetStart("10.0.0.1").SetCount(60000).SetStep("0.0.0.1")
				ip.Dst().Increment().SetStart("198.51.0.1").SetCount(60000).SetStep("0.0.0.1")
				udp := flow.Packet().Add().Udp()
				udp.SrcPort().Increment().SetStart(1024).SetCount(60000).SetStep(1)
				udp.DstPort().Increment().SetStart(1024).SetCount(60000).SetStep(1)
			},
		},
		{
			name: "IPnIP_Encap",
			flowSetter: func(flow gosnappi.Flow) {
				eth := flow.Packet().Add().Ethernet()
				eth.Src().SetValue(ateIngressMAC)
				outer := flow.Packet().Add().Ipv4()
				outer.Src().SetValue("10.10.10.1")
				outer.Dst().SetValue("172.16.0.1")
				inner := flow.Packet().Add().Ipv4()
				inner.Src().Increment().SetStart("10.0.0.1").SetCount(60000).SetStep("0.0.0.1")
				inner.Dst().Increment().SetStart("172.16.0.1").SetCount(60000).SetStep("0.0.0.1")
				udp := flow.Packet().Add().Udp()
				udp.SrcPort().Increment().SetStart(1024).SetCount(60000).SetStep(1)
				udp.DstPort().Increment().SetStart(1024).SetCount(60000).SetStep(1)
			},
		},
	}

	subcases := []struct {
		name           string
		unequalWeights bool
	}{
		{name: "Subcase_1_1_Uniform_ECMP", unequalWeights: false},
		{name: "Subcase_1_2_Unequal_WCMP", unequalWeights: true},
	}

	for _, sc := range subcases {
		t.Run(sc.name, func(t *testing.T) {
			updateScenario1NHGs(sc.unequalWeights)
			for _, p := range profiles {
				t.Run(p.name, func(t *testing.T) {
					runTrafficAndVerify(t, p.name, p.flowSetter, sc.unequalWeights)
				})
			}
		})
	}
}

// getBreakoutParentPrefix returns the parent connector/prefix for a breakout port.
// For Nokia: "ethernet-1/2/1" -> "ethernet-1/2/"
// For Arista: "Ethernet1/2/1" -> "Ethernet1/2/"
// For Juniper: "et-0/0/1:0" -> "et-0/0/1:"
// For Cisco: "FourHundredGigE0/0/0/1/1" -> "FourHundredGigE0/0/0/1/"
func getBreakoutParentPrefix(name string) string {
	if idx := strings.LastIndex(name, ":"); idx != -1 {
		return name[:idx+1]
	}
	if idx := strings.LastIndex(name, "/"); idx != -1 {
		return name[:idx+1]
	}
	return name
}

func discoverSoftLoops(t *testing.T, dut *ondatra.DUTDevice, count int, excludePorts []string) []string {
	t.Helper()
	interfaces := gnmi.GetAll(t, dut, gnmi.OC().InterfaceAny().State())

	excludeMap := make(map[string]bool)
	excludePrefixes := make(map[string]bool)

	for _, p := range excludePorts {
		excludeMap[p] = true
		prefix := getBreakoutParentPrefix(p)
		if prefix != p {
			excludePrefixes[prefix] = true
		}
	}

	var candidateBreakouts []string
	var candidateOthers []string

	for _, intf := range interfaces {
		name := intf.GetName()
		if excludeMap[name] {
			continue
		}

		// Check if interface shares a parent breakout group with an excluded/used port
		parentPrefix := getBreakoutParentPrefix(name)
		if excludePrefixes[parentPrefix] {
			t.Logf("Excluding port %s: shares parent breakout prefix %s with used/testbed port", name, parentPrefix)
			continue
		}

		// Exclude interfaces that are part of a LAG / aggregate
		if intf.Ethernet != nil && intf.Ethernet.AggregateId != nil {
			continue
		}

		// Exclude active uplinks / connected ports (OperStatus UP)
		if intf.OperStatus == oc.Interface_OperStatus_UP {
			t.Logf("Excluding port %s: OperStatus is UP (active uplink / connected link)", name)
			continue
		}

		nameLower := strings.ToLower(name)
		isEthType := intf.Type == oc.IETFInterfaces_InterfaceType_ethernetCsmacd
		isEthName := strings.Contains(nameLower, "ethernet") ||
			strings.Contains(nameLower, "gige") ||
			strings.HasPrefix(nameLower, "et-") ||
			strings.HasPrefix(nameLower, "ge-") ||
			strings.HasPrefix(nameLower, "xe-") ||
			strings.HasPrefix(nameLower, "hu") ||
			strings.HasPrefix(nameLower, "fh") ||
			strings.HasPrefix(nameLower, "te")

		if !isEthType && !isEthName {
			continue
		}

		// Exclude interfaces with existing IPv4 configuration
		hasIP := false
		if intf.Subinterface != nil {
			for _, sub := range intf.Subinterface {
				if sub != nil && sub.Ipv4 != nil && len(sub.Ipv4.Address) > 0 {
					hasIP = true
					break
				}
			}
		}
		if hasIP {
			t.Logf("Excluding port %s: has existing IP configuration", name)
			continue
		}

		// Prioritize breakout channelized ports over raw connectors
		if strings.Count(name, "/") >= 2 || strings.Contains(name, ":") {
			candidateBreakouts = append(candidateBreakouts, name)
		} else {
			candidateOthers = append(candidateOthers, name)
		}
	}

	var discovered []string
	discovered = append(discovered, candidateBreakouts...)
	if len(discovered) < count {
		discovered = append(discovered, candidateOthers...)
	}

	if len(discovered) > count {
		discovered = discovered[:count]
	}

	if len(discovered) < count {
		t.Fatalf("Could not find enough unused leftover interfaces. Need %d, found %d: %v", count, len(discovered), discovered)
	}
	t.Logf("Discovered %d safe leftover soft loop interfaces: %v", len(discovered), discovered)
	return discovered
}

func getEgressPacketsPhys(t *testing.T, dut *ondatra.DUTDevice, ports []string) map[string]uint64 {
	t.Helper()
	stats := make(map[string]uint64)
	for _, portName := range ports {
		outPkts, present := gnmi.Lookup(t, dut, gnmi.OC().Interface(portName).Counters().OutPkts().State()).Val()
		if present {
			stats[portName] = outPkts
		} else {
			t.Logf("Warning: counter not present for port %s", portName)
			stats[portName] = 0
		}
	}
	return stats
}

type portCounters struct {
	inPkts  uint64
	outPkts uint64
}

func getPhysicalPortCounters(t *testing.T, dut *ondatra.DUTDevice) map[string]portCounters {
	t.Helper()
	stats := make(map[string]portCounters)
	batch := gnmi.OCBatch()

	var portNames []string
	for portID := range vrfPortMap {
		portNames = append(portNames, dut.Port(t, portID).Name())
	}

	for _, pName := range portNames {
		batch.AddPaths(
			gnmi.OC().Interface(pName).Counters().InPkts(),
			gnmi.OC().Interface(pName).Counters().OutPkts(),
		)
	}
	rootVal := gnmi.Get(t, dut, batch.State())

	for _, pName := range portNames {
		var inPkts, outPkts uint64
		if intf := rootVal.GetInterface(pName); intf != nil && intf.Counters != nil {
			inPkts = intf.GetCounters().GetInPkts()
			outPkts = intf.GetCounters().GetOutPkts()
		}
		stats[pName] = portCounters{inPkts: inPkts, outPkts: outPkts}
	}
	return stats
}

func configureSoftLoopACLsPhys(t *testing.T, dut *ondatra.DUTDevice, softLoops []string) {
	t.Helper()
	batch := &gnmi.SetBatch{}

	for _, portName := range softLoops {
		params := cfgplugins.AclParams{
			Name:    fmt.Sprintf("drop_rx_%s", portName),
			ACLType: oc.Acl_ACL_TYPE_ACL_IPV4,
			Intf:    portName,
			Ingress: true,
			Terms: []cfgplugins.AclTerm{
				{
					SeqID:  10,
					Permit: false,
					IPSrc:  "0.0.0.0/0",
					IPDst:  "0.0.0.0/0",
				},
			},
		}
		cfgplugins.ConfigureACL(t, dut, batch, params)
	}

	t.Logf("Applying ACLs to soft loops: %v", softLoops)
	batch.Set(t, dut)
}

func populateLAGInterface(dut *ondatra.DUTDevice, i *oc.Interface, lagName string, ip string, prefixLen uint8, enabled bool) {
	i.Name = ygot.String(lagName)
	i.Type = oc.IETFInterfaces_InterfaceType_ieee8023adLag
	if enabled {
		i.Enabled = ygot.Bool(true)
	}
	agg := i.GetOrCreateAggregation()
	agg.LagType = oc.IfAggregate_AggregationType_STATIC
	s := i.GetOrCreateSubinterface(0)
	s4 := s.GetOrCreateIpv4()
	if !deviations.IPv4MissingEnabled(dut) {
		s4.Enabled = ygot.Bool(true)
	}
	a := s4.GetOrCreateAddress(ip)
	a.PrefixLength = ygot.Uint8(prefixLen)
}

func cleanDiscoveredPorts(t *testing.T, dut *ondatra.DUTDevice, ports []string) {
	t.Helper()
	t.Logf("Cleaning stale config on discovered soft loop ports: %v", ports)
	batch := &gnmi.SetBatch{}
	for _, p := range ports {
		gnmi.BatchDelete(batch, gnmi.OC().Interface(p).Ethernet().PortSpeed().Config())
		gnmi.BatchDelete(batch, gnmi.OC().Interface(p).Ethernet().AggregateId().Config())
		gnmi.BatchDelete(batch, gnmi.OC().Interface(p).LoopbackMode().Config())
	}
	batch.Set(t, dut)
}

func populatePhysicalInterfaceForLAG(dut *ondatra.DUTDevice, i *oc.Interface, portName string, aggID string, enabled bool, loopbackMode oc.E_Interfaces_LoopbackModeType) {
	i.Name = ygot.String(portName)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	if enabled {
		i.Enabled = ygot.Bool(true)
	}
	if !deviations.MemberLinkLoopbackUnsupported(dut) && loopbackMode != oc.Interfaces_LoopbackModeType_NONE {
		i.LoopbackMode = loopbackMode
	}
	e := i.GetOrCreateEthernet()
	e.AggregateId = ygot.String(aggID)
}

func configureStaticARPIngressAndEgress(t *testing.T, dut *ondatra.DUTDevice, portToLagMap map[string]string) {
	t.Helper()
	// Ingress LAG static ARP -> ATE ixia2
	pIn := dut.Port(t, "lc2_p10")
	lagIn := portToLagMap[pIn.Name()]
	cfgIn := vrfPortMap["lc2_p10"]
	peerIPIn := getPeerIP(t, cfgIn.ip)
	gnmi.Update(t, dut, gnmi.OC().Interface(lagIn).Config(), configStaticArpLag(lagIn, peerIPIn, ateIngressMAC))

	// Egress LAG static ARP -> ATE ixia1
	pEg := dut.Port(t, "lc2_p9")
	lagEg := portToLagMap[pEg.Name()]
	cfgEg := vrfPortMap["lc2_p9"]
	peerIPEg := getPeerIP(t, cfgEg.ip)
	gnmi.Update(t, dut, gnmi.OC().Interface(lagEg).Config(), configStaticArpLag(lagEg, peerIPEg, ateEgressMAC))
}

func configStaticArpLag(lagName string, ipv4addr string, macAddr string) *oc.Interface {
	i := &oc.Interface{
		Name: ygot.String(lagName),
		Type: oc.IETFInterfaces_InterfaceType_ieee8023adLag,
	}
	s := i.GetOrCreateSubinterface(0)
	s4 := s.GetOrCreateIpv4()
	s4.Enabled = ygot.Bool(true)
	n := s4.GetOrCreateNeighbor(ipv4addr)
	n.LinkLayerAddress = ygot.String(macAddr)
	return i
}

func cleanupDevice(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	t.Log("Performing cleanup of LAGs and VRFs...")
	d := gnmi.OC()

	interfaces := gnmi.LookupAll(t, dut, d.InterfaceAny().Config())

	targetLags := make(map[string]bool)
	for i := 101; i <= 140; i++ {
		targetLags[getLagName(dut, i)] = true
		targetLags[fmt.Sprintf("lag%d", i)] = true
	}

	batch1 := &gnmi.SetBatch{}

	// Delete ACL interface references
	for lagName := range targetLags {
		gnmi.BatchDelete(batch1, d.Acl().Interface(lagName).Config())
	}

	for _, intfVal := range interfaces {
		intf, present := intfVal.Val()
		if !present {
			continue
		}
		name := intf.GetName()
		if intf.Ethernet != nil && intf.Ethernet.AggregateId != nil {
			lagName := *intf.Ethernet.AggregateId
			if targetLags[lagName] {
				t.Logf("Reverting member port %s (was in %s)", name, lagName)
				cleanIntf := &oc.Interface{
					Name:    ygot.String(name),
					Type:    oc.IETFInterfaces_InterfaceType_ethernetCsmacd,
					Enabled: ygot.Bool(true),
				}
				gnmi.BatchReplace(batch1, d.Interface(name).Config(), cleanIntf)
			}
		}
	}
	t.Log("Executing stage 1 cleanup (member ports & ACLs)...")
	batch1.Set(t, dut)

	batch2 := &gnmi.SetBatch{}
	vrfs := []string{vrfTransit, vrfSelfSite, vrfEgress}
	for _, vrf := range vrfs {
		t.Logf("Deleting VRF %s", vrf)
		gnmi.BatchDelete(batch2, d.NetworkInstance(vrf).Config())
	}

	gnmi.BatchDelete(batch2, d.NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding().Config())

	for lagName := range targetLags {
		gnmi.BatchDelete(batch2, d.Interface(lagName).Config())
	}

	t.Log("Executing stage 2 cleanup (LAGs & VRFs)...")
	batch2.Set(t, dut)
}

func configureOTGForFlow(t *testing.T, ate *ondatra.ATEDevice, setupFlow func(flow gosnappi.Flow)) gosnappi.Config {
	t.Helper()
	otg := ate.OTG()
	config := gosnappi.NewConfig()

	// Ingress port: ixia2 connected to DUT lc2_p10
	ap1 := ate.Port(t, "ixia2")
	p1 := config.Ports().Add().SetName(ap1.ID())

	// Egress port: ixia1 connected to DUT lc2_p9
	ap2 := ate.Port(t, "ixia1")
	p2 := config.Ports().Add().SetName(ap2.ID())

	// Layer1 settings
	ly1 := config.Layer1().Add().SetName("ly1")
	ly1.SetPortNames([]string{p1.Name(), p2.Name()})
	ly1.AutoNegotiation().SetRsFec(false)

	// Tx Device (ATE Port 2 / ixia2 -> DUT lc2_p10)
	d1 := config.Devices().Add().SetName("atePort2.Device")
	eth1 := d1.Ethernets().Add().SetName("atePort2.Eth").SetMac(ateIngressMAC)
	eth1.Connection().SetPortName(p1.Name())
	ip1 := eth1.Ipv4Addresses().Add().SetName("atePort2.IPv4").
		SetAddress("192.0.2.2").
		SetGateway("192.0.2.1").
		SetPrefix(30)

	// Rx Device (ATE Port 1 / ixia1 <- DUT lc2_p9)
	d2 := config.Devices().Add().SetName("atePort1.Device")
	eth2 := d2.Ethernets().Add().SetName("atePort1.Eth").SetMac(ateEgressMAC)
	eth2.Connection().SetPortName(p2.Name())
	ip2 := eth2.Ipv4Addresses().Add().SetName("atePort1.IPv4").
		SetAddress("192.0.2.6").
		SetGateway("192.0.2.5").
		SetPrefix(30)

	// Flow: Ingress (ixia2) -> Egress (ixia1)
	flow := config.Flows().Add().SetName("HashingFlow")
	flow.Metrics().SetEnable(true)
	flow.TxRx().Device().SetTxNames([]string{ip1.Name()}).SetRxNames([]string{ip2.Name()})
	flow.Size().SetFixed(512)
	flow.Rate().SetPps(5000)
	flow.Duration().Continuous()

	setupFlow(flow)

	otg.PushConfig(t, config)
	otg.StartProtocols(t)

	return config
}

type expectedRatio struct {
	portID string
	name   string
	ratio  float64
}

func verifyWCMPDistribution(t *testing.T, name string, deltas map[string]uint64, expected []expectedRatio) {
	t.Helper()
	var total uint64
	for _, exp := range expected {
		total += deltas[exp.portID]
	}
	t.Logf("%s Total Egress Packets: %d", name, total)
	if total == 0 {
		t.Errorf("%s total packets is 0, cannot verify distribution", name)
		return
	}

	for _, exp := range expected {
		ratio := float64(deltas[exp.portID]) / float64(total)
		minExpected := exp.ratio * 0.98
		maxExpected := exp.ratio * 1.02
		t.Logf("  Port %s (%s): %d packets, ratio: %.4f (expected: %.4f [%.4f, %.4f])", exp.portID, exp.name, deltas[exp.portID], ratio, exp.ratio, minExpected, maxExpected)
		if ratio < minExpected || ratio > maxExpected {
			t.Errorf("  Port %s (%s) ratio %.4f is out of expected range [%.4f, %.4f]", exp.portID, exp.name, ratio, minExpected, maxExpected)
		}
	}
}

func verifyDistribution(t *testing.T, name string, deltas map[string]uint64, ports []string, expectedRatio float64) {
	t.Helper()
	var total uint64
	for _, portID := range ports {
		total += deltas[portID]
	}
	t.Logf("VRF %s Total Egress Packets: %d", name, total)
	if total == 0 {
		t.Errorf("VRF %s total packets is 0, cannot verify distribution", name)
		return
	}

	minExpected := expectedRatio * 0.98
	maxExpected := expectedRatio * 1.02
	for _, portID := range ports {
		ratio := float64(deltas[portID]) / float64(total)
		t.Logf("  Port %s: %d packets, ratio: %.4f (expected: %.4f [%.4f, %.4f])", portID, deltas[portID], ratio, expectedRatio, minExpected, maxExpected)
		if ratio < minExpected || ratio > maxExpected {
			t.Errorf("  Port %s ratio %.4f is out of expected range [%.4f, %.4f]", portID, ratio, minExpected, maxExpected)
		}
	}
}

func configureStaticARP(t *testing.T, dut *ondatra.DUTDevice, portToLagMap map[string]string, portToMacMap map[string]string, softLoops []softLoopInfo) {
	t.Helper()
	d := gnmi.OC()

	// 8 Physical Loop Pairs
	pairs := []struct {
		p1 string
		p2 string
	}{
		{"lc1_p3", "lc2_p3"}, // Loop 1
		{"lc1_p4", "lc2_p4"}, // Loop 2
		{"lc1_p5", "lc2_p5"}, // Loop 3
		{"lc1_p6", "lc2_p6"}, // Loop 4
		{"lc1_p1", "lc2_p1"}, // Loop 5
		{"lc2_p8", "lc1_p8"}, // Loop 6
		{"lc2_p7", "lc1_p7"}, // Loop 7
		{"lc2_p2", "lc1_p2"}, // Loop 8
	}

	for _, pair := range pairs {
		port1 := dut.Port(t, pair.p1)
		port2 := dut.Port(t, pair.p2)
		lag1 := portToLagMap[port1.Name()]
		lag2 := portToLagMap[port2.Name()]

		ip1 := vrfPortMap[pair.p1].ip
		ip2 := vrfPortMap[pair.p2].ip

		mac1 := portToMacMap[port1.Name()]
		mac2 := portToMacMap[port2.Name()]

		t.Logf("Configuring static ARP on %s for %s (%s)", lag1, ip2, mac2)
		gnmi.Update(t, dut, d.Interface(lag1).Config(), configStaticArpLag(lag1, ip2, mac2))
		t.Logf("Configuring static ARP on %s for %s (%s)", lag2, ip1, mac1)
		gnmi.Update(t, dut, d.Interface(lag2).Config(), configStaticArpLag(lag2, ip1, mac1))
	}

	// Soft loops static ARP
	for _, sl := range softLoops {
		lagName := portToLagMap[sl.physName]
		peerIP := getPeerIP(t, sl.ip)
		t.Logf("Configuring static ARP on soft loop %s for %s (%s)", lagName, peerIP, sl.mac)
		gnmi.Update(t, dut, d.Interface(lagName).Config(), configStaticArpLag(lagName, peerIP, sl.mac))
	}
}

func logRuntimeDebug(t *testing.T, dut *ondatra.DUTDevice, portToLagMap map[string]string) {
	t.Helper()
	t.Log("Logging Ingress Counters for Loop RX ports:")
	rxPorts := []string{"lc2_p3", "lc2_p4", "lc2_p5", "lc2_p6", "lc2_p1", "lc1_p8", "lc1_p7", "lc1_p2"}
	for _, portID := range rxPorts {
		p := dut.Port(t, portID)
		lagName := portToLagMap[p.Name()]
		inPkts, present := gnmi.Lookup(t, dut, gnmi.OC().Interface(lagName).Counters().InPkts().State()).Val()
		if present {
			t.Logf("  Port %s (%s) InPackets: %d", portID, lagName, inPkts)
		} else {
			t.Logf("  Port %s (%s) InPackets: N/A", portID, lagName)
		}
	}

	t.Log("Logging Interface Status:")
	for portID := range vrfPortMap {
		p := dut.Port(t, portID)
		lagName := portToLagMap[p.Name()]

		pOper := gnmi.Get(t, dut, gnmi.OC().Interface(p.Name()).OperStatus().State())
		pAdmin := gnmi.Get(t, dut, gnmi.OC().Interface(p.Name()).AdminStatus().State())
		pInPkts, _ := gnmi.Lookup(t, dut, gnmi.OC().Interface(p.Name()).Counters().InPkts().State()).Val()
		pOutPkts, _ := gnmi.Lookup(t, dut, gnmi.OC().Interface(p.Name()).Counters().OutPkts().State()).Val()
		pInErr, _ := gnmi.Lookup(t, dut, gnmi.OC().Interface(p.Name()).Counters().InErrors().State()).Val()
		pOutErr, _ := gnmi.Lookup(t, dut, gnmi.OC().Interface(p.Name()).Counters().OutErrors().State()).Val()
		t.Logf("  Phys Port %s (%s): Admin=%v, Oper=%v, InPkts=%v, OutPkts=%v, InErr=%v, OutErr=%v", portID, p.Name(), pAdmin, pOper, pInPkts, pOutPkts, pInErr, pOutErr)

		lOper := gnmi.Get(t, dut, gnmi.OC().Interface(lagName).OperStatus().State())
		lAdmin := gnmi.Get(t, dut, gnmi.OC().Interface(lagName).AdminStatus().State())
		lInErr, _ := gnmi.Lookup(t, dut, gnmi.OC().Interface(lagName).Counters().InErrors().State()).Val()
		lOutErr, _ := gnmi.Lookup(t, dut, gnmi.OC().Interface(lagName).Counters().OutErrors().State()).Val()
		macState, _ := gnmi.Lookup(t, dut, gnmi.OC().Interface(lagName).Ethernet().MacAddress().State()).Val()
		t.Logf("  LAG %s: Admin=%v, Oper=%v, InErr=%v, OutErr=%v, StateMAC=%s", lagName, lAdmin, lOper, lInErr, lOutErr, macState)
	}
}
