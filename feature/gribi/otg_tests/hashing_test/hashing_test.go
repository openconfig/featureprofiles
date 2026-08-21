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
	maskLen24     = "24"
	targetSubnet  = "198.51.100.0"
	targetStartIP = "198.51.100.1"
	targetIPCount = 254
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

// Physical DUT Ports mapping: 8 physical loop pairs + 2 ATE links = 18 ports
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
		lagMac := portToMacMap[p.Name()]

		// Populate LAG Interface
		lagIntf := root.GetOrCreateInterface(lagName)
		populateLAGInterface(dut, lagIntf, lagName, cfg.ip, plen, true, lagMac)

		// Populate Physical Interface
		physIntf := root.GetOrCreateInterface(p.Name())
		populatePhysicalInterfaceForLAG(physIntf, p.Name(), lagName, true, cfg.loopbackMode)
	}

	// Populate Soft Loops Interfaces
	for _, sl := range softLoops {
		lagName := portToLagMap[sl.physName]

		// Populate LAG Interface
		lagIntf := root.GetOrCreateInterface(lagName)
		populateLAGInterface(dut, lagIntf, lagName, sl.ip, plen, true, sl.mac)

		// Populate Physical Interface
		physIntf := root.GetOrCreateInterface(sl.physName)
		populatePhysicalInterfaceForLAG(physIntf, sl.physName, lagName, true, oc.Interfaces_LoopbackModeType_TERMINAL)
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

	// 2. Configure Network Instances (VRFs)
	vrfs := []string{vrfTransit, vrfSelfSite, vrfEgress}
	for _, vrf := range vrfs {
		ni := &oc.NetworkInstance{
			Name: ygot.String(vrf),
			Type: oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF,
		}
		gnmi.Replace(t, dut, d.NetworkInstance(vrf).Config(), ni)
	}

	// 3. Assign LAG Interfaces to VRFs
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

	// 4. Configure ACL Drop on RX for all soft loops
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

	// Route definitions across all VRFs
	routePrefix := targetSubnet + "/" + maskLen24
	rDef := fluent.IPv4Entry().WithNetworkInstance(defNI).
		WithPrefix(routePrefix).
		WithNextHopGroup(1).
		WithNextHopGroupNetworkInstance(defNI)
	entries = append(entries, rDef)

	rTransit := fluent.IPv4Entry().WithNetworkInstance(vrfTransit).
		WithPrefix(routePrefix).
		WithNextHopGroup(2).
		WithNextHopGroupNetworkInstance(vrfTransit)
	entries = append(entries, rTransit)

	rSelfSite := fluent.IPv4Entry().WithNetworkInstance(vrfSelfSite).
		WithPrefix(routePrefix).
		WithNextHopGroup(3).
		WithNextHopGroupNetworkInstance(vrfSelfSite)
	entries = append(entries, rSelfSite)

	rEgress := fluent.IPv4Entry().WithNetworkInstance(vrfEgress).
		WithPrefix(routePrefix).
		WithNextHopGroup(4).
		WithNextHopGroupNetworkInstance(vrfEgress)
	entries = append(entries, rEgress)

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

	// Discover 12 soft loops dynamically from unused DUT interfaces
	var excludePorts []string
	for _, p := range dut.Ports() {
		excludePorts = append(excludePorts, p.Name())
	}

	discoveredPorts := discoverSoftLoops(t, dut, 12, excludePorts)

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
	configureStaticARP(t, dut, portToLagMap, portToMacMap, softLoops)

	ctx := context.Background()
	programGRIBIVRF(ctx, t, dut, gClient, softLoops, portToLagMap, portToMacMap)

	ate := ondatra.ATE(t, "ate")
	otgConfig := configureOTG(t, ate)
	otgutils.WaitForARP(t, ate.OTG(), otgConfig, "IPv4")
	t.Log("Starting traffic...")
	ate.OTG().StartTraffic(t)

	// Build verification port lists
	// Stage 1: Loop 1 TX (lc1_p3) + Soft 0, 1, 2
	var stage1Ports []string
	p1_3 := dut.Port(t, "lc1_p3")
	stage1Ports = append(stage1Ports, portToLagMap[p1_3.Name()])
	for _, i := range []int{0, 1, 2} {
		stage1Ports = append(stage1Ports, portToLagMap[softLoops[i].physName])
	}

	// Stage 2: Loops 2, 3, 4, 5 TX + Soft 3, 4, 5, 6
	var transitPorts []string
	for _, portID := range []string{"lc1_p4", "lc1_p5", "lc1_p6", "lc1_p1"} {
		p := dut.Port(t, portID)
		transitPorts = append(transitPorts, portToLagMap[p.Name()])
	}
	for _, i := range []int{3, 4, 5, 6} {
		transitPorts = append(transitPorts, portToLagMap[softLoops[i].physName])
	}

	// Stage 3: Loops 6, 7, 8 TX + Soft 7, 8, 9, 10, 11
	var selfSitePorts []string
	for _, portID := range []string{"lc2_p8", "lc2_p7", "lc2_p2"} {
		p := dut.Port(t, portID)
		selfSitePorts = append(selfSitePorts, portToLagMap[p.Name()])
	}
	for _, i := range []int{7, 8, 9, 10, 11} {
		selfSitePorts = append(selfSitePorts, portToLagMap[softLoops[i].physName])
	}

	// Egress DUT port lc2_p9
	egressPortName := dut.Port(t, "lc2_p9").Name()
	egressLagName := portToLagMap[egressPortName]

	allVerifyPorts := append(stage1Ports, transitPorts...)
	allVerifyPorts = append(allVerifyPorts, selfSitePorts...)
	allVerifyPorts = append(allVerifyPorts, egressLagName)

	ingressPortName := dut.Port(t, "lc2_p10").Name()
	ingressCounterPath := gnmi.OC().Interface(ingressPortName).Counters().InPkts().State()

	t.Log("Reading initial counters...")
	initialCounters := getEgressPacketsPhys(t, dut, allVerifyPorts)
	initialIngress := gnmi.Get(t, dut, ingressCounterPath)
	initialPhysCounters := getPhysicalPortCounters(t, dut)

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
		t.Logf("  Phys Port %s (%s): InDelta=%d, OutDelta=%d (In: %d -> %d, Out: %d -> %d)", portID, pName, inDelta, outDelta, initC.inPkts, finalC.inPkts, initC.outPkts, finalC.outPkts)
	}

	t.Log("Stopping traffic...")
	ate.OTG().StopTraffic(t)
	otgutils.LogFlowMetrics(t, ate.OTG(), otgConfig)

	deltas := make(map[string]uint64)
	for _, portID := range allVerifyPorts {
		if finalCounters[portID] < initialCounters[portID] {
			t.Logf("Warning: counter rolled over for port %s", portID)
			deltas[portID] = finalCounters[portID]
		} else {
			deltas[portID] = finalCounters[portID] - initialCounters[portID]
		}
	}

	// 1. Stage 1 Hashing (WCMP 7:1:1:1)
	t.Run("Stage 1: Default VRF WCMP Hashing", func(t *testing.T) {
		expected := []expectedRatio{
			{portID: stage1Ports[0], name: "Physical lc1_p3", ratio: 0.7000},
			{portID: stage1Ports[1], name: "Soft 0", ratio: 0.1000},
			{portID: stage1Ports[2], name: "Soft 1", ratio: 0.1000},
			{portID: stage1Ports[3], name: "Soft 2", ratio: 0.1000},
		}
		verifyWCMPDistribution(t, "Stage 1 (Default VRF WCMP 7:1:1:1)", deltas, expected, 0.03)
	})

	// 2. Stage 2 Hashing (Transit VRF 8-wide ECMP)
	t.Run("Stage 2: Transit VRF ECMP Hashing", func(t *testing.T) {
		verifyDistribution(t, "TRANSIT", deltas, transitPorts, 0.1250, 0.03)
	})

	// 3. Stage 3 Hashing (Self-Site VRF 8-wide ECMP)
	t.Run("Stage 3: Self-Site VRF ECMP Hashing", func(t *testing.T) {
		verifyDistribution(t, "SELF_SITE", deltas, selfSitePorts, 0.1250, 0.03)
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

func discoverSoftLoops(t *testing.T, dut *ondatra.DUTDevice, count int, excludePorts []string) []string {
	t.Helper()
	interfaces := gnmi.GetAll(t, dut, gnmi.OC().InterfaceAny().State())

	excludeMap := make(map[string]bool)
	for _, p := range excludePorts {
		excludeMap[p] = true
	}

	var discovered []string
	for _, intf := range interfaces {
		name := intf.GetName()
		// Filter non-physical or excluded ports
		if excludeMap[name] {
			continue
		}
		if intf.Ethernet != nil && intf.Ethernet.AggregateId != nil {
			continue
		}
		nameLower := strings.ToLower(name)
		// Check OpenConfig interface type or vendor physical Ethernet naming conventions
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
			continue
		}

		discovered = append(discovered, name)
		if len(discovered) == count {
			break
		}
	}

	if len(discovered) < count {
		t.Fatalf("Could not find enough unused interfaces. Need %d, found %d: %v", count, len(discovered), discovered)
	}
	return discovered
}

func getEgressPacketsPhys(t *testing.T, dut *ondatra.DUTDevice, ports []string) map[string]uint64 {
	t.Helper()
	stats := make(map[string]uint64)
	batch := gnmi.OCBatch()
	for _, portName := range ports {
		batch.AddPaths(gnmi.OC().Interface(portName).Counters().OutPkts())
	}
	rootVal := gnmi.Get(t, dut, batch.State())

	for _, portName := range ports {
		if intf := rootVal.GetInterface(portName); intf != nil && intf.Counters != nil {
			stats[portName] = intf.GetCounters().GetOutPkts()
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
			Name:          "drop-all-" + portName,
			DefaultPermit: false,
			ACLType:       oc.Acl_ACL_TYPE_ACL_IPV4,
			Intf:          portName,
			Ingress:       true,
		}
		cfgplugins.ConfigureACL(t, dut, batch, params)
	}

	t.Logf("Applying ACLs to soft loops: %v", softLoops)
	batch.Set(t, dut)
}

func populateLAGInterface(dut *ondatra.DUTDevice, i *oc.Interface, lagName string, ip string, prefixLen uint8, enabled bool, mac string) {
	i.Name = ygot.String(lagName)
	i.Type = oc.IETFInterfaces_InterfaceType_ieee8023adLag
	if enabled {
		i.Enabled = ygot.Bool(true)
	}
	agg := i.GetOrCreateAggregation()
	agg.LagType = oc.IfAggregate_AggregationType_STATIC
	if mac != "" {
		i.GetOrCreateEthernet().MacAddress = ygot.String(mac)
	}
	s := i.GetOrCreateSubinterface(0)
	s4 := s.GetOrCreateIpv4()
	if !deviations.IPv4MissingEnabled(dut) {
		s4.Enabled = ygot.Bool(true)
	}
	a := s4.GetOrCreateAddress(ip)
	a.PrefixLength = ygot.Uint8(prefixLen)
}

func populatePhysicalInterfaceForLAG(i *oc.Interface, name string, lagName string, enabled bool, loopbackMode oc.E_Interfaces_LoopbackModeType) {
	i.Name = ygot.String(name)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	if enabled {
		i.Enabled = ygot.Bool(true)
	}
	if loopbackMode != oc.Interfaces_LoopbackModeType_NONE {
		i.LoopbackMode = loopbackMode
	}
	ethernet := i.GetOrCreateEthernet()
	ethernet.AggregateId = ygot.String(lagName)
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

	batch := &gnmi.SetBatch{}

	// Delete ACL interface references
	for lagName := range targetLags {
		gnmi.BatchDelete(batch, d.Acl().Interface(lagName).Config())
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
				gnmi.BatchReplace(batch, d.Interface(name).Config(), cleanIntf)
			}
		}
	}

	vrfs := []string{vrfTransit, vrfSelfSite, vrfEgress}
	for _, vrf := range vrfs {
		t.Logf("Deleting VRF %s", vrf)
		gnmi.BatchDelete(batch, d.NetworkInstance(vrf).Config())
	}

	for lagName := range targetLags {
		gnmi.BatchDelete(batch, d.Interface(lagName).Config())
	}

	t.Log("Executing batched cleanup...")
	batch.Set(t, dut)
}

func configureOTG(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	t.Helper()
	otg := ate.OTG()
	config := gosnappi.NewConfig()

	// Ingress port: ixia2 connected to DUT lc2_p10
	ap2 := ate.Port(t, "ixia2")
	p2 := config.Ports().Add().SetName(ap2.ID())

	// Egress port: ixia1 connected to DUT lc2_p9
	ap1 := ate.Port(t, "ixia1")
	p1 := config.Ports().Add().SetName(ap1.ID())

	// Layer1 settings
	ly1 := config.Layer1().Add().SetName("ly1")
	ly1.SetPortNames([]string{p2.Name(), p1.Name()})
	ly1.AutoNegotiation().SetRsFec(false)

	// Tx Device (ATE Port 2 / ixia2)
	d2 := config.Devices().Add().SetName("atePort2.Device")
	eth2 := d2.Ethernets().Add().SetName("atePort2.Eth").SetMac(ateIngressMAC)
	eth2.Connection().SetPortName(p2.Name())
	ip2 := eth2.Ipv4Addresses().Add().SetName("atePort2.IPv4").
		SetAddress("192.0.2.2").
		SetGateway("192.0.2.1").
		SetPrefix(30)

	// Rx Device (ATE Port 1 / ixia1)
	d1 := config.Devices().Add().SetName("atePort1.Device")
	eth1 := d1.Ethernets().Add().SetName("atePort1.Eth").SetMac(ateEgressMAC)
	eth1.Connection().SetPortName(p1.Name())
	ip1 := eth1.Ipv4Addresses().Add().SetName("atePort1.IPv4").
		SetAddress("192.0.2.6").
		SetGateway("192.0.2.5").
		SetPrefix(30)

	// Flow 1: Ingress (ixia2) -> Egress (ixia1)
	flow1 := config.Flows().Add().SetName("HashingFlow")
	flow1.Metrics().SetEnable(true)
	flow1.TxRx().Device().SetTxNames([]string{ip2.Name()}).SetRxNames([]string{ip1.Name()})
	flow1.Size().SetFixed(512)
	flow1.Rate().SetPps(5000)
	flow1.Duration().Continuous()

	ethHeader1 := flow1.Packet().Add().Ethernet()
	ethHeader1.Src().SetValue(ateIngressMAC)
	ipHeader1 := flow1.Packet().Add().Ipv4()
	ipHeader1.Src().Increment().SetStart("192.0.2.2").SetCount(50000).SetStep("0.0.0.1")
	ipHeader1.Dst().Increment().SetStart(targetStartIP).SetCount(targetIPCount).SetStep("0.0.0.1")
	udpHeader1 := flow1.Packet().Add().Udp()
	udpHeader1.SrcPort().Increment().SetStart(1024).SetCount(60000).SetStep(1)
	udpHeader1.DstPort().Increment().SetStart(1024).SetCount(60000).SetStep(1)

	otg.PushConfig(t, config)
	otg.StartProtocols(t)

	return config
}

type expectedRatio struct {
	portID string
	name   string
	ratio  float64
}

func verifyWCMPDistribution(t *testing.T, name string, deltas map[string]uint64, expected []expectedRatio, tolerance float64) {
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
		minExpected := exp.ratio - tolerance
		maxExpected := exp.ratio + tolerance
		t.Logf("  Port %s (%s): %d packets, ratio: %.4f (expected: %.4f [%.4f, %.4f])", exp.portID, exp.name, deltas[exp.portID], ratio, exp.ratio, minExpected, maxExpected)
		if ratio < minExpected || ratio > maxExpected {
			t.Errorf("  Port %s (%s) ratio %.4f is out of expected range [%.4f, %.4f]", exp.portID, exp.name, ratio, minExpected, maxExpected)
		}
	}
}

func verifyDistribution(t *testing.T, name string, deltas map[string]uint64, ports []string, expectedRatio float64, tolerance float64) {
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

	minExpected := expectedRatio - tolerance
	maxExpected := expectedRatio + tolerance
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

	// Soft loops static ARP (to peer IP)
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
