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
	"testing"
	"time"
	
	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/featureprofiles/internal/helpers"
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

	// Egress Next Hop ID
	nhIDEgress uint64 = 401

	// Minimum ingress packets required to guarantee ECMP/WCMP hashing convergence within +-2% tolerance
	minPacketsForHashing uint64 = 1000000

	// Minimum traffic transmission duration to average PPS and hashing distribution
	trafficDuration = 120 * time.Second
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
	"lc1_p8": {ip: "192.0.2.29", loopbackMode: oc.Interfaces_LoopbackModeType_NONE},
	"lc2_p8": {ip: "192.0.2.30", loopbackMode: oc.Interfaces_LoopbackModeType_NONE},

	// Loop 7 (Stage 3 Self-Site -> Egress)
	"lc1_p7": {ip: "192.0.2.33", loopbackMode: oc.Interfaces_LoopbackModeType_NONE},
	"lc2_p7": {ip: "192.0.2.34", loopbackMode: oc.Interfaces_LoopbackModeType_NONE},

	// Loop 8 (Stage 3 Self-Site -> Egress)
	"lc1_p2": {ip: "192.0.2.37", loopbackMode: oc.Interfaces_LoopbackModeType_NONE},
	"lc2_p2": {ip: "192.0.2.38", loopbackMode: oc.Interfaces_LoopbackModeType_NONE},
}

func configureDUTVRF(t *testing.T, dut *ondatra.DUTDevice, portToLagMap map[string]string, portToMacMap map[string]string) {
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
		// SelfSite VRF:
		"lc2_p3": vrfSelfSite, // Loop 1 RX (Port 2 from Ingress)
		"lc2_p4": vrfSelfSite, // Loop 2 RX (Port 3 from Ingress)
		"lc1_p1": vrfSelfSite, // Loop 5 TX (Port 6 to Egress)
		"lc1_p8": vrfSelfSite, // Loop 6 TX (Port 7 to Egress)
		"lc1_p7": vrfSelfSite, // Loop 7 TX (Port 8 to Egress)
		"lc1_p2": vrfSelfSite, // Loop 8 TX (Port 9 to Egress)

		// Egress VRF:
		"lc2_p5": vrfEgress, // Loop 3 RX (Port 4 from Ingress)
		"lc2_p6": vrfEgress, // Loop 4 RX (Port 5 from Ingress)
		"lc2_p1": vrfEgress, // Loop 5 RX (Port 6 from SelfSite)
		"lc2_p8": vrfEgress, // Loop 6 RX (Port 7 from SelfSite)
		"lc2_p7": vrfEgress, // Loop 7 RX (Port 8 from SelfSite)
		"lc2_p2": vrfEgress, // Loop 8 RX (Port 9 from SelfSite)
		"lc2_p9": vrfEgress, // DUT Egress Port -> ATE Egress (Port 10 / ixia1)
	}

	for portID, vrfName := range vrfAssignments {
		p := dut.Port(t, portID)
		lagName := portToLagMap[p.Name()]
		fptest.AssignToNetworkInstance(t, dut, lagName, vrfName, 0)
	}

	// Assign remaining physical ports in vrfPortMap to default VRF if required (lc2_p10, lc1_p3, lc1_p4, lc1_p5)
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
}

func programGRIBIVRF(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, gClient *gribi.Client, portToLagMap map[string]string, portToMacMap map[string]string) {
	t.Helper()

	c := gClient.Fluent(t)
	defNI := deviations.DefaultNetworkInstance(dut)

	type nhDetail struct {
		txPortName string
		rxPortName string
		vrfName    string
	}

	portNHs := map[uint64]nhDetail{
		// Ingress (Default VRF) NextHops:
		// NH 2: Port 2 (lc1_p3 -> lc2_p3 in SelfSite)
		2: {txPortName: "lc1_p3", rxPortName: "lc2_p3", vrfName: defNI},
		// NH 3: Port 3 (lc1_p4 -> lc2_p4 in SelfSite)
		3: {txPortName: "lc1_p4", rxPortName: "lc2_p4", vrfName: defNI},
		// NH 4: Port 4 (lc1_p5 -> lc2_p5 in Egress)
		4: {txPortName: "lc1_p5", rxPortName: "lc2_p5", vrfName: defNI},
		// NH 5: Port 5 (lc1_p6 -> lc2_p6 in Egress)
		5: {txPortName: "lc1_p6", rxPortName: "lc2_p6", vrfName: defNI},

		// SelfSite VRF NextHops:
		// NH 6: Port 6 (lc1_p1 -> lc2_p1 in Egress)
		6: {txPortName: "lc1_p1", rxPortName: "lc2_p1", vrfName: vrfSelfSite},
		// NH 7: Port 7 (lc1_p8 -> lc2_p8 in Egress)
		7: {txPortName: "lc1_p8", rxPortName: "lc2_p8", vrfName: vrfSelfSite},
		// NH 8: Port 8 (lc1_p7 -> lc2_p7 in Egress)
		8: {txPortName: "lc1_p7", rxPortName: "lc2_p7", vrfName: vrfSelfSite},
		// NH 9: Port 9 (lc1_p2 -> lc2_p2 in Egress)
		9: {txPortName: "lc1_p2", rxPortName: "lc2_p2", vrfName: vrfSelfSite},

		// Egress VRF NextHop:
		// NH 10: Port 10 (lc2_p9 -> ATE ixia1)
		nhIDEgress: {txPortName: "lc2_p9", rxPortName: "", vrfName: vrfEgress},
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

	// NHG 1: Default VRF (Ingress) -> 4-way ECMP (1:1:1:1) across NH 2, 3, 4, 5
	nhg1 := fluent.NextHopGroupEntry().WithNetworkInstance(defNI).WithID(1).
		AddNextHop(2, 1).
		AddNextHop(3, 1).
		AddNextHop(4, 1).
		AddNextHop(5, 1)
	entries = append(entries, nhg1)

	// NHG 2: SelfSite VRF -> 4-way ECMP (1:1:1:1) across NH 6, 7, 8, 9
	nhg2 := fluent.NextHopGroupEntry().WithNetworkInstance(vrfSelfSite).WithID(2).
		AddNextHop(6, 1).
		AddNextHop(7, 1).
		AddNextHop(8, 1).
		AddNextHop(9, 1)
	entries = append(entries, nhg2)

	// NHG 3: Egress VRF -> Egress port lc2_p9
	nhg3 := fluent.NextHopGroupEntry().WithNetworkInstance(vrfEgress).WithID(3).
		AddNextHop(nhIDEgress, 1)
	entries = append(entries, nhg3)

	// Route definitions for Plain IP (198.51.0.0/16) and Encap (172.16.0.0/16)
	subnets := []string{plainSubnet, encapSubnet}
	for _, pfx := range subnets {
		entries = append(entries,
			fluent.IPv4Entry().WithNetworkInstance(defNI).WithPrefix(pfx).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(defNI),
			fluent.IPv4Entry().WithNetworkInstance(vrfSelfSite).WithPrefix(pfx).WithNextHopGroup(2).WithNextHopGroupNetworkInstance(vrfSelfSite),
			fluent.IPv4Entry().WithNetworkInstance(vrfEgress).WithPrefix(pfx).WithNextHopGroup(3).WithNextHopGroupNetworkInstance(vrfEgress),
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
	defNI := deviations.DefaultNetworkInstance(dut)

	// Perform cleanup of LAGs and VRFs before test
	cleanupDevice(t, dut)

	if dut.Vendor() == ondatra.NOKIA {
		helpers.GnmiCLIConfig(t, dut, "system load-balancing hash-options per-complex-randomization true")
	}

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

	configureDUTVRF(t, dut, portToLagMap, portToMacMap)

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

	configureStaticARP(t, dut, portToLagMap, portToMacMap)

	ctx := context.Background()
	programGRIBIVRF(ctx, t, dut, gClient, portToLagMap, portToMacMap)

	ate := ondatra.ATE(t, "ate")

	lag2 := portToLagMap[dut.Port(t, "lc1_p3").Name()]
	lag3 := portToLagMap[dut.Port(t, "lc1_p4").Name()]
	lag4 := portToLagMap[dut.Port(t, "lc1_p5").Name()]
	lag5 := portToLagMap[dut.Port(t, "lc1_p6").Name()]
	lag6 := portToLagMap[dut.Port(t, "lc1_p1").Name()]
	lag7 := portToLagMap[dut.Port(t, "lc1_p8").Name()]
	lag8 := portToLagMap[dut.Port(t, "lc1_p7").Name()]
	lag9 := portToLagMap[dut.Port(t, "lc1_p2").Name()]
	egressLagName := portToLagMap[dut.Port(t, "lc2_p9").Name()]

	allVerifyPorts := []string{lag2, lag3, lag4, lag5, lag6, lag7, lag8, lag9, egressLagName}

	ingressPortName := dut.Port(t, "lc2_p10").Name()
	ingressCounterPath := gnmi.OC().Interface(ingressPortName).Counters().InPkts().State()

	runTrafficAndVerify := func(t *testing.T, profileName string, isWCMP bool, setupFlow func(flow gosnappi.Flow)) {
		t.Logf("=== Starting Traffic Profile: %s ===", profileName)
		otgConfig := configureOTGForFlow(t, ate, setupFlow)
		otgutils.WaitForARP(t, ate.OTG(), otgConfig, "IPv4")

		t.Log("Reading initial counters...")
		initialCounters := getEgressPacketsPhys(t, dut, allVerifyPorts)
		initialIngress := gnmi.Get(t, dut, ingressCounterPath)
		initialPhysCounters := getPhysicalPortCounters(t, dut)

		t.Logf("Starting traffic for %v (minimum 2 mins)...", trafficDuration)
		ate.OTG().StartTraffic(t)
		time.Sleep(trafficDuration)

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

		// 1. Stage 1 Hashing
		t.Run("Stage 1: Ingress Hashing", func(t *testing.T) {
			var expectedStage1 []expectedRatio
			var testName string
			if isWCMP {
				testName = "Default VRF (Stage 1 WCMP 4:3:2:1)"
				expectedStage1 = []expectedRatio{
					{portID: lag2, name: "Port 2: LAG 2 (lc1_p3 -> SelfSite)", ratio: 0.40},
					{portID: lag3, name: "Port 3: LAG 3 (lc1_p4 -> SelfSite)", ratio: 0.30},
					{portID: lag4, name: "Port 4: LAG 4 (lc1_p5 -> Egress)", ratio: 0.20},
					{portID: lag5, name: "Port 5: LAG 5 (lc1_p6 -> Egress)", ratio: 0.10},
				}
			} else {
				testName = "Default VRF (Stage 1 ECMP 1:1:1:1)"
				expectedStage1 = []expectedRatio{
					{portID: lag2, name: "Port 2: LAG 2 (lc1_p3 -> SelfSite)", ratio: 0.25},
					{portID: lag3, name: "Port 3: LAG 3 (lc1_p4 -> SelfSite)", ratio: 0.25},
					{portID: lag4, name: "Port 4: LAG 4 (lc1_p5 -> Egress)", ratio: 0.25},
					{portID: lag5, name: "Port 5: LAG 5 (lc1_p6 -> Egress)", ratio: 0.25},
				}
			}
			stage1Deltas := map[string]uint64{
				lag2: deltas[lag2],
				lag3: deltas[lag3],
				lag4: deltas[lag4],
				lag5: deltas[lag5],
			}
			verifyWCMPDistribution(t, testName, stage1Deltas, expectedStage1)
		})

		// 2. Stage 2 Hashing (SelfSite VRF 1:1:1:1 ECMP)
		t.Run("Stage 2: SelfSite VRF ECMP Hashing", func(t *testing.T) {
			expectedStage2 := []expectedRatio{
				{portID: lag6, name: "Port 6: LAG 6 (lc1_p1 -> Egress)", ratio: 0.25},
				{portID: lag7, name: "Port 7: LAG 7 (lc1_p8 -> Egress)", ratio: 0.25},
				{portID: lag8, name: "Port 8: LAG 8 (lc1_p7 -> Egress)", ratio: 0.25},
				{portID: lag9, name: "Port 9: LAG 9 (lc1_p2 -> Egress)", ratio: 0.25},
			}
			stage2Deltas := map[string]uint64{
				lag6: deltas[lag6],
				lag7: deltas[lag7],
				lag8: deltas[lag8],
				lag9: deltas[lag9],
			}
			verifyWCMPDistribution(t, "SelfSite VRF (Stage 2 ECMP 1:1:1:1)", stage2Deltas, expectedStage2)
		})

		// 3. Stage 3 (Egress Multi-Stream Arrival)
		t.Run("Stage 3: Egress Multi-Stream Arrival", func(t *testing.T) {
			var expectedStage3 []expectedRatio
			if isWCMP {
				// SelfSite receives 70% total. 4-way ECMP gives 70%/4 = 17.5% per link.
				expectedStage3 = []expectedRatio{
					{portID: lag4, name: "Port 4: LAG 4 (Direct)", ratio: 0.20},
					{portID: lag5, name: "Port 5: LAG 5 (Direct)", ratio: 0.10},
					{portID: lag6, name: "Port 6: LAG 6 (via SelfSite)", ratio: 0.175},
					{portID: lag7, name: "Port 7: LAG 7 (via SelfSite)", ratio: 0.175},
					{portID: lag8, name: "Port 8: LAG 8 (via SelfSite)", ratio: 0.175},
					{portID: lag9, name: "Port 9: LAG 9 (via SelfSite)", ratio: 0.175},
				}
			} else {
				// SelfSite receives 50% total. 4-way ECMP gives 50%/4 = 12.5% per link.
				expectedStage3 = []expectedRatio{
					{portID: lag4, name: "Port 4: LAG 4 (Direct)", ratio: 0.25},
					{portID: lag5, name: "Port 5: LAG 5 (Direct)", ratio: 0.25},
					{portID: lag6, name: "Port 6: LAG 6 (via SelfSite)", ratio: 0.125},
					{portID: lag7, name: "Port 7: LAG 7 (via SelfSite)", ratio: 0.125},
					{portID: lag8, name: "Port 8: LAG 8 (via SelfSite)", ratio: 0.125},
					{portID: lag9, name: "Port 9: LAG 9 (via SelfSite)", ratio: 0.125},
				}
			}
			stage3Deltas := map[string]uint64{
				lag4: deltas[lag4],
				lag5: deltas[lag5],
				lag6: deltas[lag6],
				lag7: deltas[lag7],
				lag8: deltas[lag8],
				lag9: deltas[lag9],
			}
			verifyWCMPDistribution(t, "Egress Multi-Stream Arrival", stage3Deltas, expectedStage3)
		})

		// 4. Stage 4: Full Traffic Arrival on ATE Egress
		t.Run("Stage 4: Egress Port Arrival", func(t *testing.T) {
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
				ip.Src().Increment().SetStart("10.0.0.1").SetCount(1009).SetStep("0.0.0.1")
				ip.Dst().Increment().SetStart("198.51.0.1").SetCount(1013).SetStep("0.0.0.1")
				udp := flow.Packet().Add().Udp()
				udp.SrcPort().Increment().SetStart(1024).SetCount(1019).SetStep(1)
				udp.DstPort().Increment().SetStart(1024).SetCount(1021).SetStep(1)
			},
		},
		{
			name: "IPnIP_Encap",
			flowSetter: func(flow gosnappi.Flow) {
				eth := flow.Packet().Add().Ethernet()
				eth.Src().SetValue(ateIngressMAC)
				outer := flow.Packet().Add().Ipv4()
				outer.Src().SetValue("10.10.10.1")
				outer.Dst().Increment().SetStart("172.16.0.1").SetCount(254).SetStep("0.0.0.1")
				inner := flow.Packet().Add().Ipv4()
				inner.Src().Increment().SetStart("10.0.0.1").SetCount(1009).SetStep("0.0.0.1")
				inner.Dst().Increment().SetStart("198.51.0.1").SetCount(1013).SetStep("0.0.0.1")
				udp := flow.Packet().Add().Udp()
				udp.SrcPort().Increment().SetStart(1024).SetCount(1019).SetStep(1)
				udp.DstPort().Increment().SetStart(1024).SetCount(1021).SetStep(1)
			},
		},
	}

	// Scenario 1: Multi-Stage WCMP & ECMP
	t.Run("Scenario_1_Hash_Distribution", func(t *testing.T) {
		t.Run("Subcase_1_1_WCMP_4_3_2_1", func(t *testing.T) {
			nhg1WCMP := fluent.NextHopGroupEntry().WithNetworkInstance(defNI).WithID(1).
				AddNextHop(2, 4).
				AddNextHop(3, 3).
				AddNextHop(4, 2).
				AddNextHop(5, 1)
			c := gClient.Fluent(t)
			c.Modify().AddEntry(t, nhg1WCMP)
			ctxTimeout, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			if err := c.Await(ctxTimeout, t); err != nil {
				t.Fatalf("Failed to program NHG1 WCMP: %v", err)
			}
			for _, p := range profiles {
				t.Run(p.name, func(t *testing.T) {
					runTrafficAndVerify(t, p.name, true /* isWCMP */, p.flowSetter)
				})
			}
		})

		t.Run("Subcase_1_2_ECMP_1_1_1_1", func(t *testing.T) {
			nhg1ECMP := fluent.NextHopGroupEntry().WithNetworkInstance(defNI).WithID(1).
				AddNextHop(2, 1).
				AddNextHop(3, 1).
				AddNextHop(4, 1).
				AddNextHop(5, 1)
			c := gClient.Fluent(t)
			c.Modify().AddEntry(t, nhg1ECMP)
			ctxTimeout, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			if err := c.Await(ctxTimeout, t); err != nil {
				t.Fatalf("Failed to program NHG1 ECMP: %v", err)
			}
			for _, p := range profiles {
				t.Run(p.name, func(t *testing.T) {
					runTrafficAndVerify(t, p.name, false /* isWCMP */, p.flowSetter)
				})
			}
		})
	})

	// Scenario 2: Intra-LAG Member Traffic Distribution
	t.Run("Scenario_2_Intra_LAG", func(t *testing.T) {
		t.Log("=== Starting Scenario 2: Intra-LAG Member Traffic Distribution ===")
		lag201 := getLagName(dut, 201)
		lag202 := getLagName(dut, 202)

		txPortIDs := []string{"lc1_p4", "lc1_p5", "lc1_p6", "lc1_p1", "lc1_p8", "lc1_p7", "lc1_p2"}
		rxPortIDs := []string{"lc2_p4", "lc2_p5", "lc2_p6", "lc2_p1", "lc2_p8", "lc2_p7", "lc2_p2"}

		b := &gnmi.SetBatch{}
		d := gnmi.OC()

		// Populate lag201 (TX) and lag202 (RX)
		lagIntf201 := &oc.Interface{Name: ygot.String(lag201), Type: oc.IETFInterfaces_InterfaceType_ieee8023adLag, Enabled: ygot.Bool(true)}
		populateLAGInterface(dut, lagIntf201, lag201, "198.18.201.1", plen, true)
		gnmi.BatchUpdate(b, d.Interface(lag201).Config(), lagIntf201)

		lagIntf202 := &oc.Interface{Name: ygot.String(lag202), Type: oc.IETFInterfaces_InterfaceType_ieee8023adLag, Enabled: ygot.Bool(true)}
		populateLAGInterface(dut, lagIntf202, lag202, "198.18.201.2", plen, true)
		gnmi.BatchUpdate(b, d.Interface(lag202).Config(), lagIntf202)

		// Reassign the 7 TX physical ports to lag201
		for _, pID := range txPortIDs {
			p := dut.Port(t, pID)
			gnmi.BatchUpdate(b, d.Interface(p.Name()).Ethernet().AggregateId().Config(), lag201)
		}
		// Reassign the 7 RX physical ports to lag202
		for _, pID := range rxPortIDs {
			p := dut.Port(t, pID)
			gnmi.BatchUpdate(b, d.Interface(p.Name()).Ethernet().AggregateId().Config(), lag202)
		}

		mac201 := getMacForLagIndex(201)
		mac202 := getMacForLagIndex(202)
		gnmi.BatchUpdate(b, d.Interface(lag201).Config(), configStaticArpLag(lag201, "198.18.201.2", mac202))
		gnmi.BatchUpdate(b, d.Interface(lag202).Config(), configStaticArpLag(lag202, "198.18.201.1", mac201))

		b.Set(t, dut)

		// Reassign Loop 1 RX (lc2_p3) to vrfTransit (unbind from SELF_SITE first)
		pLoop1RX := dut.Port(t, "lc2_p3")
		lagLoop1RX := portToLagMap[pLoop1RX.Name()]
		gnmi.Delete(t, dut, d.NetworkInstance(vrfSelfSite).Interface(fmt.Sprintf("%s.0", lagLoop1RX)).Config())
		fptest.AssignToNetworkInstance(t, dut, lagLoop1RX, vrfTransit, 0)

		fptest.AssignToNetworkInstance(t, dut, lag201, vrfTransit, 0)
		fptest.AssignToNetworkInstance(t, dut, lag202, vrfEgress, 0)

		// In DEFAULT VRF, route 100% of traffic to Loop 1 TX (NH 2)
		nhgDef100 := fluent.NextHopGroupEntry().WithNetworkInstance(defNI).WithID(100).AddNextHop(2, 1)
		ipv4DefPlain := fluent.IPv4Entry().WithNetworkInstance(defNI).WithPrefix(plainSubnet).WithNextHopGroup(100)
		ipv4DefEncap := fluent.IPv4Entry().WithNetworkInstance(defNI).WithPrefix(encapSubnet).WithNextHopGroup(100)

		// Program gRIBI for Scenario 2
		nh501 := fluent.NextHopEntry().
			WithNetworkInstance(vrfTransit).
			WithIndex(501).
			WithIPAddress("198.18.201.2").
			WithMacAddress(mac202).
			WithInterfaceRef(lag201)

		nhg501 := fluent.NextHopGroupEntry().
			WithNetworkInstance(vrfTransit).
			WithID(501).
			AddNextHop(501, 1)

		ipv4Transit2Plain := fluent.IPv4Entry().
			WithNetworkInstance(vrfTransit).
			WithPrefix(plainSubnet).
			WithNextHopGroup(501)

		ipv4Transit2Encap := fluent.IPv4Entry().
			WithNetworkInstance(vrfTransit).
			WithPrefix(encapSubnet).
			WithNextHopGroup(501)

		c := gClient.Fluent(t)
		c.Modify().AddEntry(t, nhgDef100, ipv4DefPlain, ipv4DefEncap, nh501, nhg501, ipv4Transit2Plain, ipv4Transit2Encap)
		ctxTimeout, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		if err := c.Await(ctxTimeout, t); err != nil {
			t.Fatalf("Error updating gRIBI for Scenario 2: %v", err)
		}

		for _, p := range profiles {
			t.Run(p.name, func(t *testing.T) {
				otgConfig := configureOTGForFlow(t, ate, p.flowSetter)
				otgutils.WaitForARP(t, ate.OTG(), otgConfig, "IPv4")

				initialPhysCounters := getPhysicalPortCounters(t, dut)
				initialCounters := getEgressPacketsPhys(t, dut, []string{egressLagName})

				t.Logf("Running traffic for %v (minimum 2 mins)...", trafficDuration)
				ate.OTG().StartTraffic(t)
				time.Sleep(trafficDuration)
				ate.OTG().StopTraffic(t)
				finalPhysCounters := getPhysicalPortCounters(t, dut)
				finalCounters := getEgressPacketsPhys(t, dut, []string{egressLagName})

				var totalMemberPkts uint64
				memberDeltas := make(map[string]uint64)
				for _, pID := range txPortIDs {
					p := dut.Port(t, pID)
					delta := finalPhysCounters[p.Name()].outPkts - initialPhysCounters[p.Name()].outPkts
					memberDeltas[p.Name()] = delta
					totalMemberPkts += delta
				}

				t.Logf("Scenario 2 (7-Member LAG %s) Total Member OutPkts: %d", lag201, totalMemberPkts)
				if totalMemberPkts == 0 {
					t.Errorf("Scenario 2 total packets is 0")
					return
				}

				expectedRatio := 1.0 / 7.0
				minExpected := expectedRatio * 0.98
				maxExpected := expectedRatio * 1.02

				for _, pID := range txPortIDs {
					p := dut.Port(t, pID)
					delta := memberDeltas[p.Name()]
					ratio := float64(delta) / float64(totalMemberPkts)
					t.Logf("  Member %s (%s): %d packets, ratio: %.4f (expected: %.4f [%.4f, %.4f])", pID, p.Name(), delta, ratio, expectedRatio, minExpected, maxExpected)
					if ratio < minExpected || ratio > maxExpected {
						t.Errorf("  Member %s (%s) ratio %.4f is out of expected range [%.4f, %.4f]", pID, p.Name(), ratio, minExpected, maxExpected)
					}
				}

				egressDelta := finalCounters[egressLagName] - initialCounters[egressLagName]
				t.Logf("DUT Egress LAG %s OutPkts delta: %d", egressLagName, egressDelta)
				if egressDelta == 0 {
					t.Errorf("DUT Egress LAG %s received 0 packets in Scenario 2", egressLagName)
				}
			})
		}
	})

	// Scenario 3: Asymmetric Paths & Weighted Load Balancing (3-Wide LAGs)
	t.Run("Scenario_3_Asymmetric_LAGs", func(t *testing.T) {
		t.Log("=== Starting Scenario 3: Asymmetric Paths & Weighted Load Balancing ===")
		lag201 := getLagName(dut, 201)
		lag202 := getLagName(dut, 202)
		lag203 := getLagName(dut, 203) // LAG A TX (3 members)
		lag204 := getLagName(dut, 204) // LAG A RX (3 members)
		lag205 := getLagName(dut, 205) // LAG B TX (2 members)
		lag206 := getLagName(dut, 206) // LAG B RX (2 members)
		lag207 := getLagName(dut, 207) // LAG C TX (2 members)
		lag208 := getLagName(dut, 208) // LAG C RX (2 members)

		lagATX := []string{"lc1_p4", "lc1_p5", "lc1_p6"}
		lagARX := []string{"lc2_p4", "lc2_p5", "lc2_p6"}
		lagBTX := []string{"lc1_p1", "lc1_p8"}
		lagBRX := []string{"lc2_p1", "lc2_p8"}
		lagCTX := []string{"lc1_p7", "lc1_p2"}
		lagCRX := []string{"lc2_p7", "lc2_p2"}

		b := &gnmi.SetBatch{}
		d := gnmi.OC()

		// Unassign and delete lag201 and lag202 from Scenario 2
		gnmi.BatchDelete(b, d.NetworkInstance(vrfTransit).Interface(fmt.Sprintf("%s.0", lag201)).Config())
		gnmi.BatchDelete(b, d.NetworkInstance(vrfEgress).Interface(fmt.Sprintf("%s.0", lag202)).Config())
		gnmi.BatchDelete(b, d.Interface(lag201).Config())
		gnmi.BatchDelete(b, d.Interface(lag202).Config())

		// Populate LAG A, B, C TX & RX
		lagConfigs := []struct {
			txName string
			rxName string
			txIP   string
			rxIP   string
			txIdx  int
			rxIdx  int
		}{
			{lag203, lag204, "198.18.203.1", "198.18.203.2", 203, 204},
			{lag205, lag206, "198.18.205.1", "198.18.205.2", 205, 206},
			{lag207, lag208, "198.18.207.1", "198.18.207.2", 207, 208},
		}

		for _, cfg := range lagConfigs {
			intfTX := &oc.Interface{Name: ygot.String(cfg.txName), Type: oc.IETFInterfaces_InterfaceType_ieee8023adLag, Enabled: ygot.Bool(true)}
			populateLAGInterface(dut, intfTX, cfg.txName, cfg.txIP, plen, true)
			gnmi.BatchUpdate(b, d.Interface(cfg.txName).Config(), intfTX)

			intfRX := &oc.Interface{Name: ygot.String(cfg.rxName), Type: oc.IETFInterfaces_InterfaceType_ieee8023adLag, Enabled: ygot.Bool(true)}
			populateLAGInterface(dut, intfRX, cfg.rxName, cfg.rxIP, plen, true)
			gnmi.BatchUpdate(b, d.Interface(cfg.rxName).Config(), intfRX)

			macTX := getMacForLagIndex(cfg.txIdx)
			macRX := getMacForLagIndex(cfg.rxIdx)
			gnmi.BatchUpdate(b, d.Interface(cfg.txName).Config(), configStaticArpLag(cfg.txName, cfg.rxIP, macRX))
			gnmi.BatchUpdate(b, d.Interface(cfg.rxName).Config(), configStaticArpLag(cfg.rxName, cfg.txIP, macTX))
		}

		// Reassign member ports
		for _, pID := range lagATX {
			p := dut.Port(t, pID)
			gnmi.BatchUpdate(b, d.Interface(p.Name()).Ethernet().AggregateId().Config(), lag203)
		}
		for _, pID := range lagARX {
			p := dut.Port(t, pID)
			gnmi.BatchUpdate(b, d.Interface(p.Name()).Ethernet().AggregateId().Config(), lag204)
		}
		for _, pID := range lagBTX {
			p := dut.Port(t, pID)
			gnmi.BatchUpdate(b, d.Interface(p.Name()).Ethernet().AggregateId().Config(), lag205)
		}
		for _, pID := range lagBRX {
			p := dut.Port(t, pID)
			gnmi.BatchUpdate(b, d.Interface(p.Name()).Ethernet().AggregateId().Config(), lag206)
		}
		for _, pID := range lagCTX {
			p := dut.Port(t, pID)
			gnmi.BatchUpdate(b, d.Interface(p.Name()).Ethernet().AggregateId().Config(), lag207)
		}
		for _, pID := range lagCRX {
			p := dut.Port(t, pID)
			gnmi.BatchUpdate(b, d.Interface(p.Name()).Ethernet().AggregateId().Config(), lag208)
		}

		b.Set(t, dut)

		fptest.AssignToNetworkInstance(t, dut, lag203, vrfTransit, 0)
		fptest.AssignToNetworkInstance(t, dut, lag205, vrfTransit, 0)
		fptest.AssignToNetworkInstance(t, dut, lag207, vrfTransit, 0)
		fptest.AssignToNetworkInstance(t, dut, lag204, vrfEgress, 0)
		fptest.AssignToNetworkInstance(t, dut, lag206, vrfEgress, 0)
		fptest.AssignToNetworkInstance(t, dut, lag208, vrfEgress, 0)

		// Program gRIBI NextHops 601, 602, 603
		nh601 := fluent.NextHopEntry().WithNetworkInstance(vrfTransit).WithIndex(601).WithIPAddress("198.18.203.2").WithMacAddress(getMacForLagIndex(204)).WithInterfaceRef(lag203)
		nh602 := fluent.NextHopEntry().WithNetworkInstance(vrfTransit).WithIndex(602).WithIPAddress("198.18.205.2").WithMacAddress(getMacForLagIndex(206)).WithInterfaceRef(lag205)
		nh603 := fluent.NextHopEntry().WithNetworkInstance(vrfTransit).WithIndex(603).WithIPAddress("198.18.207.2").WithMacAddress(getMacForLagIndex(208)).WithInterfaceRef(lag207)

		scenario3Subcases := []struct {
			name        string
			weights     []uint64 // A, B, C
			expectedExp []expectedRatio
		}{
			{
				name:    "Subcase_3_1_Capacity_Based_3_2_2",
				weights: []uint64{3, 2, 2},
				expectedExp: []expectedRatio{
					{portID: lag203, name: "LAG A (3 members)", ratio: 3.0 / 7.0},
					{portID: lag205, name: "LAG B (2 members)", ratio: 2.0 / 7.0},
					{portID: lag207, name: "LAG C (2 members)", ratio: 2.0 / 7.0},
				},
			},
			{
				name:    "Subcase_3_2_Uniform_Override_1_1_1",
				weights: []uint64{1, 1, 1},
				expectedExp: []expectedRatio{
					{portID: lag203, name: "LAG A (3 members)", ratio: 1.0 / 3.0},
					{portID: lag205, name: "LAG B (2 members)", ratio: 1.0 / 3.0},
					{portID: lag207, name: "LAG C (2 members)", ratio: 1.0 / 3.0},
				},
			},
		}

		scenario3VerifyPorts := []string{lag203, lag205, lag207, egressLagName}

		for _, sc := range scenario3Subcases {
			t.Run(sc.name, func(t *testing.T) {
				nhg601 := fluent.NextHopGroupEntry().WithNetworkInstance(vrfTransit).WithID(601).
					AddNextHop(601, sc.weights[0]).
					AddNextHop(602, sc.weights[1]).
					AddNextHop(603, sc.weights[2])

				ipv4Transit3Plain := fluent.IPv4Entry().WithNetworkInstance(vrfTransit).WithPrefix(plainSubnet).WithNextHopGroup(601)
				ipv4Transit3Encap := fluent.IPv4Entry().WithNetworkInstance(vrfTransit).WithPrefix(encapSubnet).WithNextHopGroup(601)

				c := gClient.Fluent(t)
				c.Modify().AddEntry(t, nh601, nh602, nh603, nhg601, ipv4Transit3Plain, ipv4Transit3Encap)
				ctxTimeout, cancel := context.WithTimeout(ctx, 30*time.Second)
				defer cancel()
				if err := c.Await(ctxTimeout, t); err != nil {
					t.Fatalf("Error updating gRIBI for Scenario 3: %v", err)
				}

				for _, p := range profiles {
					t.Run(p.name, func(t *testing.T) {
						otgConfig := configureOTGForFlow(t, ate, p.flowSetter)
						otgutils.WaitForARP(t, ate.OTG(), otgConfig, "IPv4")

						initialCounters := getEgressPacketsPhys(t, dut, scenario3VerifyPorts)

						t.Logf("Running traffic for %v (minimum 2 mins)...", trafficDuration)
						ate.OTG().StartTraffic(t)
						time.Sleep(trafficDuration)
						ate.OTG().StopTraffic(t)
						finalCounters := getEgressPacketsPhys(t, dut, scenario3VerifyPorts)

						deltas := make(map[string]uint64)
						for _, lagName := range scenario3VerifyPorts {
							if finalCounters[lagName] >= initialCounters[lagName] {
								deltas[lagName] = finalCounters[lagName] - initialCounters[lagName]
							} else {
								deltas[lagName] = finalCounters[lagName]
							}
						}

						verifyWCMPDistribution(t, fmt.Sprintf("Scenario 3 (%s)", sc.name), deltas, sc.expectedExp)

						egressDelta := deltas[egressLagName]
						t.Logf("DUT Egress LAG %s OutPkts delta: %d", egressLagName, egressDelta)
						if egressDelta == 0 {
							t.Errorf("DUT Egress LAG %s received 0 packets in Scenario 3", egressLagName)
						}
					})
				}
			})
		}
	})
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
	b := &gnmi.SetBatch{}
	// Ingress LAG static ARP -> ATE ixia2
	pIn := dut.Port(t, "lc2_p10")
	lagIn := portToLagMap[pIn.Name()]
	cfgIn := vrfPortMap["lc2_p10"]
	peerIPIn := getPeerIP(t, cfgIn.ip)
	gnmi.BatchUpdate(b, gnmi.OC().Interface(lagIn).Config(), configStaticArpLag(lagIn, peerIPIn, ateIngressMAC))

	// Egress LAG static ARP -> ATE ixia1
	pEg := dut.Port(t, "lc2_p9")
	lagEg := portToLagMap[pEg.Name()]
	cfgEg := vrfPortMap["lc2_p9"]
	peerIPEg := getPeerIP(t, cfgEg.ip)
	gnmi.BatchUpdate(b, gnmi.OC().Interface(lagEg).Config(), configStaticArpLag(lagEg, peerIPEg, ateEgressMAC))

	b.Set(t, dut)
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

	targetLags := make(map[string]bool)
	for i := 101; i <= 220; i++ {
		targetLags[getLagName(dut, i)] = true
		targetLags[fmt.Sprintf("lag%d", i)] = true
	}

	batch1 := &gnmi.SetBatch{}

	// Delete ACL interface references
	for lagName := range targetLags {
		gnmi.BatchDelete(batch1, d.Acl().Interface(lagName).Config())
	}

	// Unassign LAG interfaces and subinterfaces from default VRF
	defNI := deviations.DefaultNetworkInstance(dut)
	for lagName := range targetLags {
		gnmi.BatchDelete(batch1, d.NetworkInstance(defNI).Interface(lagName).Config())
		gnmi.BatchDelete(batch1, d.NetworkInstance(defNI).Interface(fmt.Sprintf("%s.0", lagName)).Config())
	}

	// Explicitly remove aggregate-id and loopback-mode from all ports configured by the test
	for portID := range vrfPortMap {
		p := dut.Port(t, portID)
		t.Logf("Reverting member port %s", p.Name())
		gnmi.BatchDelete(batch1, d.Interface(p.Name()).Ethernet().AggregateId().Config())
		gnmi.BatchDelete(batch1, d.Interface(p.Name()).LoopbackMode().Config())
	}

	t.Log("Executing stage 1 cleanup (member ports, default VRF unbind, & ACLs)...")
	batch1.Set(t, dut)

	batch2 := &gnmi.SetBatch{}
	vrfs := []string{vrfTransit, vrfSelfSite, vrfEgress}
	for _, vrf := range vrfs {
		t.Logf("Deleting VRF %s", vrf)
		gnmi.BatchDelete(batch2, d.NetworkInstance(vrf).Config())
	}

	gnmi.BatchDelete(batch2, d.NetworkInstance(defNI).PolicyForwarding().Config())

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
	flow.Rate().SetPps(500000)
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

func configureStaticARP(t *testing.T, dut *ondatra.DUTDevice, portToLagMap map[string]string, portToMacMap map[string]string) {
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

	batch := &gnmi.SetBatch{}
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
		gnmi.BatchUpdate(batch, d.Interface(lag1).Config(), configStaticArpLag(lag1, ip2, mac2))
		t.Logf("Configuring static ARP on %s for %s (%s)", lag2, ip1, mac1)
		gnmi.BatchUpdate(batch, d.Interface(lag2).Config(), configStaticArpLag(lag2, ip1, mac1))
	}

	batch.Set(t, dut)
}

func logRuntimeDebug(t *testing.T, dut *ondatra.DUTDevice, portToLagMap map[string]string) {
	t.Helper()
	rxPorts := []string{"lc2_p3", "lc2_p4", "lc2_p5", "lc2_p6", "lc2_p1", "lc2_p8", "lc2_p7", "lc2_p2"}

	batch := gnmi.OCBatch()
	for _, portID := range rxPorts {
		p := dut.Port(t, portID)
		lagName := portToLagMap[p.Name()]
		batch.AddPaths(gnmi.OC().Interface(lagName).Counters().InPkts())
	}

	for portID := range vrfPortMap {
		p := dut.Port(t, portID)
		pName := p.Name()
		lagName := portToLagMap[pName]
		batch.AddPaths(
			gnmi.OC().Interface(pName).OperStatus(),
			gnmi.OC().Interface(pName).AdminStatus(),
			gnmi.OC().Interface(pName).Counters().InPkts(),
			gnmi.OC().Interface(pName).Counters().OutPkts(),
			gnmi.OC().Interface(pName).Counters().InErrors(),
			gnmi.OC().Interface(pName).Counters().OutErrors(),
			gnmi.OC().Interface(lagName).OperStatus(),
			gnmi.OC().Interface(lagName).AdminStatus(),
			gnmi.OC().Interface(lagName).Counters().InErrors(),
			gnmi.OC().Interface(lagName).Counters().OutErrors(),
		)
	}

	rootVal := gnmi.Get(t, dut, batch.State())

	t.Log("Logging Ingress Counters for Loop RX ports:")
	for _, portID := range rxPorts {
		p := dut.Port(t, portID)
		lagName := portToLagMap[p.Name()]
		if intf := rootVal.GetInterface(lagName); intf != nil && intf.Counters != nil && intf.Counters.InPkts != nil {
			t.Logf("  Port %s (%s) InPackets: %d", portID, lagName, intf.GetCounters().GetInPkts())
		} else {
			t.Logf("  Port %s (%s) InPackets: N/A", portID, lagName)
		}
	}

	t.Log("Logging Interface Status:")
	for portID := range vrfPortMap {
		p := dut.Port(t, portID)
		pName := p.Name()
		lagName := portToLagMap[pName]

		var pAdmin oc.E_Interface_AdminStatus
		var pOper oc.E_Interface_OperStatus
		var pInPkts, pOutPkts, pInErr, pOutErr uint64
		if pIntf := rootVal.GetInterface(pName); pIntf != nil {
			pAdmin = pIntf.GetAdminStatus()
			pOper = pIntf.GetOperStatus()
			if pIntf.Counters != nil {
				pInPkts = pIntf.GetCounters().GetInPkts()
				pOutPkts = pIntf.GetCounters().GetOutPkts()
				pInErr = pIntf.GetCounters().GetInErrors()
				pOutErr = pIntf.GetCounters().GetOutErrors()
			}
		}
		t.Logf("  Phys Port %s (%s): Admin=%v, Oper=%v, InPkts=%v, OutPkts=%v, InErr=%v, OutErr=%v", portID, pName, pAdmin, pOper, pInPkts, pOutPkts, pInErr, pOutErr)

		var lAdmin oc.E_Interface_AdminStatus
		var lOper oc.E_Interface_OperStatus
		var lInErr, lOutErr uint64
		if lIntf := rootVal.GetInterface(lagName); lIntf != nil {
			lAdmin = lIntf.GetAdminStatus()
			lOper = lIntf.GetOperStatus()
			if lIntf.Counters != nil {
				lInErr = lIntf.GetCounters().GetInErrors()
				lOutErr = lIntf.GetCounters().GetOutErrors()
			}
		}
		t.Logf("  LAG %s: Admin=%v, Oper=%v, InErr=%v, OutErr=%v", lagName, lAdmin, lOper, lInErr, lOutErr)
	}
}
