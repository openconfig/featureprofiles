// Copyright 2025 Google LLC
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

// Package mpls_in_udp_test implements TE-18.1 MPLS-in-UDP encapsulation tests
package mpls_in_udp_test

import (
	"context"
	"encoding/binary"
	"fmt"
	"math/rand"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/gribigo/client"
	"github.com/openconfig/gribigo/constants"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygot/ygot"
)

const (
	// Network configuration
	ethertypeIPv4   = oc.PacketMatchTypes_ETHERTYPE_ETHERTYPE_IPV4
	ethertypeIPv6   = oc.PacketMatchTypes_ETHERTYPE_ETHERTYPE_IPV6
	clusterPolicy   = "vrf_selection_policy_c"
	ipv4PrefixLen   = 30
	ipv6PrefixLen   = 126
	trafficDuration = 15 * time.Second
	seqIDBase       = uint32(10)

	// MPLS-in-UDP test configuration
	mplsLabel       = uint64(100)
	outerIPv6Src    = "2001:db8::1"
	outerIPv6Dst    = "2001:db8::100"
	outerSrcUDPPort = uint16(6635) // RFC 7510 standard MPLS-in-UDP port
	outerDstUDPPort = uint16(6635) // RFC 7510 standard MPLS-in-UDP port
	outerIPTTL      = uint8(64)
	outerDscp       = uint8(10)
	innerIPv6Prefix = "2001:db8:1::/64"
	ttl             = uint32(100) // Inner packet TTL

	// gRIBI entry IDs for MPLS-in-UDP
	mplsNHID  = uint64(1001)
	mplsNHGID = uint64(2001)

	// gRIBI entry IDs for basic routing infrastructure
	basicNHID  = uint64(300)
	basicNHGID = uint64(400)

	// Static ARP configuration
	magicIP  = "192.168.1.1"
	magicMac = "02:00:00:00:00:01"

	// Test flow configuration
	dscpEncapA1 = 10

	// OTG capture limitation: Cannot start capture on more than one port belonging to the
	// same resource group or on more than one port behind the same front panel port
	otgMutliPortCaptureSupported = false
)

var (
	otgDstPorts = []string{"port2"}
	otgSrcPort  = "port1"
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "dutPort1",
		MAC:     "02:01:00:00:00:01",
		IPv4:    "192.0.2.1",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:f:d:e::1",
		IPv6Len: ipv6PrefixLen,
	}

	otgPort1 = attrs.Attributes{
		Name:    "otgPort1",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.0.2.2",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:2",
		IPv6Len: ipv6PrefixLen,
	}

	dutPort2 = attrs.Attributes{
		Desc:    "dutPort2",
		IPv4:    "192.0.2.5",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:5",
		IPv6Len: ipv6PrefixLen,
	}

	otgPort2 = attrs.Attributes{
		Name:    "otgPort2",
		MAC:     "02:00:02:01:01:01",
		IPv4:    "192.0.2.6",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:6",
		IPv6Len: ipv6PrefixLen,
	}

	dutPort2DummyIP = attrs.Attributes{
		Desc:       "dutPort2",
		IPv4Sec:    "192.0.2.21",
		IPv4LenSec: ipv4PrefixLen,
	}

	otgPort2DummyIP = attrs.Attributes{
		Desc:    "otgPort2",
		IPv4:    "192.0.2.22",
		IPv4Len: ipv4PrefixLen,
	}
)

// pbrRule defines a policy-based routing rule configuration
type pbrRule struct {
	sequence  uint32
	etherType oc.NetworkInstance_PolicyForwarding_Policy_Rule_L2_Ethertype_Union
	encapVrf  string
}

// packetResult defines the expected packet attributes for validation
type packetResult struct {
	mplsLabel  uint64
	// NOTE: Source UDP port is not validated since it is random
	// udpSrcPort uint16
	udpDstPort uint16
	ipTTL      uint8
	srcIP      string
	dstIP      string
}

// flowAttr defines traffic flow attributes for test packets
type flowAttr struct {
	src      string   // source IP address
	dst      string   // destination IP address
	srcPort  string   // source OTG port
	dstPorts []string // destination OTG ports
	srcMac   string   // source MAC address
	dstMac   string   // destination MAC address
	topo     gosnappi.Config
}

var (
	// IPv6 flow configuration for MPLS-in-UDP testing
	fa6 = flowAttr{
		src:      otgPort1.IPv6,
		dst:      strings.Split(innerIPv6Prefix, "/")[0], // Extract IPv6 prefix for inner destination
		srcMac:   otgPort1.MAC,
		dstMac:   dutPort1.MAC,
		srcPort:  otgSrcPort,
		dstPorts: otgDstPorts,
		topo:     gosnappi.NewConfig(),
	}
)

// testArgs holds the objects needed by a test case
type testArgs struct {
	dut    *ondatra.DUTDevice
	ate    *ondatra.ATEDevice
	topo   gosnappi.Config
	client *gribi.Client
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// programBasicEntries sets up the basic routing infrastructure needed for MPLS-in-UDP encapsulation.
// This creates the necessary NH/NHG entries to route encapsulated packets to the egress port.
func programBasicEntries(t *testing.T, dut *ondatra.DUTDevice, c *gribi.Client) {
	t.Log("Setting up basic routing infrastructure for MPLS-in-UDP")

	// Create basic NH pointing to port2 interface with appropriate MAC configuration
	if deviations.GRIBIMACOverrideWithStaticARP(dut) {
		t.Logf("Using GRIBIMACOverrideWithStaticARP deviation - adding NH %d with dummy IP", basicNHID)
		c.AddNH(t, basicNHID, "MACwithIp", deviations.DefaultNetworkInstance(dut),
			fluent.InstalledInFIB, &gribi.NHOptions{Dest: otgPort2DummyIP.IPv4, Mac: magicMac})
	} else if deviations.GRIBIMACOverrideStaticARPStaticRoute(dut) {
		t.Logf("Using GRIBIMACOverrideStaticARPStaticRoute deviation - adding NH %d with magic IP", basicNHID)
		p2 := dut.Port(t, "port2")
		nh, op1 := gribi.NHEntry(basicNHID, "MACwithInterface", deviations.DefaultNetworkInstance(dut),
			fluent.InstalledInFIB, &gribi.NHOptions{Interface: p2.Name(), Mac: magicMac, Dest: magicIP})
		nhg, op2 := gribi.NHGEntry(basicNHGID, map[uint64]uint64{basicNHID: 1},
			deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
		c.AddEntries(t, []fluent.GRIBIEntry{nh, nhg}, []*client.OpResult{op1, op2})
	} else {
		t.Logf("Using default deviation - adding NH %d with interface", basicNHID)
		p2 := dut.Port(t, "port2")
		c.AddNH(t, basicNHID, "MACwithInterface", deviations.DefaultNetworkInstance(dut),
			fluent.InstalledInFIB, &gribi.NHOptions{Interface: p2.Name(), Mac: magicMac})

		// Create basic NHG for routing to port2
		c.AddNHG(t, basicNHGID, map[uint64]uint64{basicNHID: 1}, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	}

	// Add IPv6 route for outer destination IP to enable encapsulated packets to reach port2
	// This is essential - without this route, encapsulated packets cannot be forwarded
	t.Logf("Adding IPv6 route %s/128 -> NHG %d", outerIPv6Dst, basicNHGID)
	c.AddIPv6(t, outerIPv6Dst+"/128", basicNHGID, deviations.DefaultNetworkInstance(dut),
		deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
}

func TestMPLSOUDPEncap(t *testing.T) {
	ctx := context.Background()

	// Configure DUT and ATE
	dut := ondatra.DUT(t, "dut")
	configureDUT(t, dut)

	ate := ondatra.ATE(t, "ate")
	otg := ate.OTG()
	topo := configureOTG(t, ate)

	t.Log("Pushing config to ATE and starting protocols...")
	otg.PushConfig(t, topo)
	otg.StartProtocols(t)

	// Wait for protocols to initialize
	t.Log("Waiting for IPv6 neighbor discovery...")
	time.Sleep(30 * time.Second)
	otgutils.WaitForARP(t, otg, topo, "IPv6")

	// Configure gRIBI client
	c := gribi.Client{
		DUT:         dut,
		FIBACK:      true,
		Persistence: true,
	}

	if err := c.Start(t); err != nil {
		t.Fatalf("gRIBI Connection can not be established")
	}
	defer c.Close(t)
	c.BecomeLeader(t)

	// Flush all existing AFT entries and set up basic routing infrastructure
	c.FlushAll(t)
	programBasicEntries(t, dut, &c)

	// Verify basic infrastructure is properly installed
	if err := c.AwaitTimeout(ctx, t, time.Minute); err != nil {
		t.Fatalf("Failed to install basic infrastructure entries: %v", err)
	}

	// Define MPLS-in-UDP test case
	testCase := struct {
		name                string
		entries             []fluent.GRIBIEntry
		wantAddResults      []*client.OpResult
		wantDelResults      []*client.OpResult
		flows               []gosnappi.Flow
		capturePorts        []string
		wantMPLSLabel       uint64
		wantOuterDstIP      string
		wantOuterSrcIP      string
		wantOuterDstUDPPort uint16
		wantOuterIPTTL      uint8
	}{
		name: "MPLS-in-UDP IPv6 Traffic Encap",
		entries: []fluent.GRIBIEntry{
			// Create MPLS-in-UDP encapsulation next hop
			fluent.NextHopEntry().
				WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithIndex(mplsNHID).
				AddEncapHeader(
					fluent.MPLSEncapHeader().WithLabels(mplsLabel),
					fluent.UDPV6EncapHeader().
						WithSrcIP(outerIPv6Src).
						WithDstIP(outerIPv6Dst).
						WithSrcUDPPort(uint64(outerSrcUDPPort)).
						WithDstUDPPort(uint64(outerDstUDPPort)).
						WithIPTTL(uint64(outerIPTTL)).
						WithDSCP(uint64(outerDscp)),
				),
			// Create next hop group pointing to MPLS NH
			fluent.NextHopGroupEntry().
				WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithID(mplsNHGID).
				AddNextHop(mplsNHID, 1),
			// Create IPv6 route that triggers MPLS-in-UDP encapsulation
			fluent.IPv6Entry().
				WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithPrefix(innerIPv6Prefix).
				WithNextHopGroup(mplsNHGID).
				WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut)),
		},
		wantAddResults: []*client.OpResult{
			fluent.OperationResult().
				WithNextHopOperation(mplsNHID).
				WithProgrammingResult(fluent.InstalledInFIB).
				WithOperationType(constants.Add).
				AsResult(),
			fluent.OperationResult().
				WithNextHopGroupOperation(mplsNHGID).
				WithProgrammingResult(fluent.InstalledInFIB).
				WithOperationType(constants.Add).
				AsResult(),
			fluent.OperationResult().
				WithIPv6Operation(innerIPv6Prefix).
				WithProgrammingResult(fluent.InstalledInFIB).
				WithOperationType(constants.Add).
				AsResult(),
		},
		wantDelResults: []*client.OpResult{
			fluent.OperationResult().
				WithIPv6Operation(innerIPv6Prefix).
				WithProgrammingResult(fluent.InstalledInFIB).
				WithOperationType(constants.Delete).
				AsResult(),
			fluent.OperationResult().
				WithNextHopGroupOperation(mplsNHGID).
				WithProgrammingResult(fluent.InstalledInFIB).
				WithOperationType(constants.Delete).
				AsResult(),
			fluent.OperationResult().
				WithNextHopOperation(mplsNHID).
				WithProgrammingResult(fluent.InstalledInFIB).
				WithOperationType(constants.Delete).
				AsResult(),
		},
		flows:               []gosnappi.Flow{fa6.getFlow("ipv6", "ip6mpls", dscpEncapA1)},
		capturePorts:        otgDstPorts[:1],
		wantMPLSLabel:       uint64(mplsLabel),
		wantOuterDstIP:      outerIPv6Dst,
		wantOuterSrcIP:      outerIPv6Src,
		wantOuterDstUDPPort: outerDstUDPPort,
		wantOuterIPTTL:      outerIPTTL,
	}

	tcArgs := &testArgs{
		client: &c,
		dut:    dut,
		ate:    ate,
		topo:   topo,
	}

	t.Run(testCase.name, func(t *testing.T) {
		// Add MPLS-in-UDP entries
		t.Log("Adding MPLS-in-UDP entries")
		c.AddEntries(t, testCase.entries, testCase.wantAddResults)

		// Enable capture and send traffic
		expectedPacket := &packetResult{
			mplsLabel:  testCase.wantMPLSLabel,
			udpDstPort: testCase.wantOuterDstUDPPort,
			ipTTL:      testCase.wantOuterIPTTL,
			srcIP:      testCase.wantOuterSrcIP,
			dstIP:      testCase.wantOuterDstIP,
		}

		if otgMutliPortCaptureSupported {
			enableCapture(t, ate.OTG(), topo, testCase.capturePorts)
			t.Log("Start capture and send traffic")
			sendTraffic(t, tcArgs, testCase.flows, true)
			t.Log("Validate captured packet attributes")
			validateMPLSPacketCapture(t, ate, testCase.capturePorts[0], expectedPacket)
			clearCapture(t, ate.OTG(), topo)
		} else {
			for _, port := range testCase.capturePorts {
				enableCapture(t, ate.OTG(), topo, []string{port})
				t.Log("Start capture and send traffic")
				sendTraffic(t, tcArgs, testCase.flows, true)
				t.Log("Validate captured packet attributes")
				validateMPLSPacketCapture(t, ate, port, expectedPacket)
				clearCapture(t, ate.OTG(), topo)
			}
		}

		// Validate traffic flows
		t.Log("Validate traffic flows")
		validateTrafficFlows(t, tcArgs, testCase.flows, false, true)

		// Clean up MPLS entries
		t.Log("Deleting MPLS-in-UDP entries")
		slices.Reverse(testCase.entries)
		c.DeleteEntries(t, testCase.entries, testCase.wantDelResults)

		// Verify traffic fails after deletion
		t.Log("Verify traffic fails after entry deletion")
		validateTrafficFlows(t, tcArgs, testCase.flows, false, false)
	})
}

// getPbrRules returns policy-based routing rules for VRF selection
func getPbrRules(dut *ondatra.DUTDevice) []pbrRule {
	vrfDefault := deviations.DefaultNetworkInstance(dut)

	if deviations.PfRequireMatchDefaultRule(dut) {
		return []pbrRule{
			{
				sequence:  17,
				etherType: ethertypeIPv4,
				encapVrf:  vrfDefault,
			},
			{
				sequence:  18,
				etherType: ethertypeIPv6,
				encapVrf:  vrfDefault,
			},
		}
	}
	return []pbrRule{
		{
			sequence: 17,
			encapVrf: vrfDefault,
		},
	}
}

// seqIDOffset returns sequence ID with base offset to ensure proper ordering
func seqIDOffset(dut *ondatra.DUTDevice, i uint32) uint32 {
	if deviations.PfRequireSequentialOrderPbrRules(dut) {
		return i + seqIDBase
	}
	return i
}

// getPbrPolicy creates policy-based routing configuration for VRF selection
func getPbrPolicy(dut *ondatra.DUTDevice, name string) *oc.NetworkInstance_PolicyForwarding {
	d := &oc.Root{}
	ni := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	pf := ni.GetOrCreatePolicyForwarding()
	p := pf.GetOrCreatePolicy(name)
	p.SetType(oc.Policy_Type_VRF_SELECTION_POLICY)

	for _, pRule := range getPbrRules(dut) {
		r := p.GetOrCreateRule(seqIDOffset(dut, pRule.sequence))

		if deviations.PfRequireMatchDefaultRule(dut) {
			if pRule.etherType != nil {
				r.GetOrCreateL2().Ethertype = pRule.etherType
			}
		}

		if pRule.encapVrf != "" {
			r.GetOrCreateAction().SetNetworkInstance(pRule.encapVrf)
		}
	}
	return pf
}

// configureBaseconfig configures network instances and forwarding policy on the DUT
func configureBaseconfig(t *testing.T, dut *ondatra.DUTDevice) {
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	pf := getPbrPolicy(dut, clusterPolicy)
	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding().Config(), pf)
}

// staticARPWithMagicUniversalIP configures static ARP with magic universal IP
func staticARPWithMagicUniversalIP(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	sb := &gnmi.SetBatch{}
	p2 := dut.Port(t, "port2")

	s := &oc.NetworkInstance_Protocol_Static{
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
	sp := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
	gnmi.BatchUpdate(sb, sp.Static(magicIP+"/32").Config(), s)
	gnmi.BatchUpdate(sb, gnmi.OC().Interface(p2.Name()).Config(), configStaticArp(p2.Name(), magicIP, magicMac))
	sb.Set(t, dut)
}

// formatMPLSHeader formats MPLS header bytes for debugging output
func formatMPLSHeader(data []byte) string {
	if len(data) < 4 {
		return "Invalid MPLS header: too short"
	}

	headerValue := binary.BigEndian.Uint32(data[:4])
	label := (headerValue >> 12) & 0xFFFFF
	exp := uint8((headerValue >> 9) & 0x07)
	s := (headerValue >> 8) & 0x01
	ttl := uint8(headerValue & 0xFF)

	return fmt.Sprintf("MPLS Label: %d, EXP: %d, BoS: %t, TTL: %d", label, exp, s == 1, ttl)
}

// validateMPLSPacketCapture validates MPLS-in-UDP encapsulated packets from capture
func validateMPLSPacketCapture(t *testing.T, ate *ondatra.ATEDevice, otgPortName string, pr *packetResult) {
	t.Logf("=== PACKET CAPTURE VALIDATION START for port %s ===", otgPortName)

	packetBytes := ate.OTG().GetCapture(t, gosnappi.NewCaptureRequest().SetPortName(otgPortName))
	t.Logf("Captured %d bytes from port %s", len(packetBytes), otgPortName)

	if len(packetBytes) == 0 {
		t.Errorf("No packet data captured on port %s", otgPortName)
		return
	}

	// Write capture to temporary pcap file for analysis
	f, err := os.CreateTemp("", ".pcap")
	if err != nil {
		t.Fatalf("Could not create temporary pcap file: %v", err)
	}
	if _, err := f.Write(packetBytes); err != nil {
		t.Fatalf("Could not write packetBytes to pcap file: %v", err)
	}
	f.Close()

	handle, err := pcap.OpenOffline(f.Name())
	if err != nil {
		t.Fatalf("Could not open pcap file: %v", err)
	}
	defer handle.Close()
	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())

	packetCount := 0
	mplsPacketCount := 0
	validMplsPacketCount := 0

	for packet := range packetSource.Packets() {
		packetCount++

		// Look for UDP-IPv6 packets (MPLS-in-UDP encapsulation)
		udpLayer := packet.Layer(layers.LayerTypeUDP)
		ipv6Layer := packet.Layer(layers.LayerTypeIPv6)
		if udpLayer == nil || ipv6Layer == nil {
			if packetCount < 5 {
				t.Logf("Packet %d: Skipping non-UDP-IPv6 packet", packetCount)
			}
			continue
		}

		mplsPacketCount++
		t.Logf("Packet %d: Found UDP-IPv6 packet for validation", packetCount)
		packetValid := true

		// Validate IPv6 outer header
		v6Packet := ipv6Layer.(*layers.IPv6)
		t.Logf("Packet %d: IPv6 src=%s, dst=%s, hopLimit=%d", packetCount,
			v6Packet.SrcIP.String(), v6Packet.DstIP.String(), v6Packet.HopLimit)

		if v6Packet.DstIP.String() != pr.dstIP {
			t.Errorf("Packet %d: Got outer destination IP %s, want %s", packetCount, v6Packet.DstIP.String(), pr.dstIP)
			packetValid = false
		}
		if v6Packet.SrcIP.String() != pr.srcIP {
			t.Errorf("Packet %d: Got outer source IP %s, want %s", packetCount, v6Packet.SrcIP.String(), pr.srcIP)
			packetValid = false
		}
		if v6Packet.HopLimit != pr.ipTTL {
			t.Errorf("Packet %d: Got outer hop limit %d, want %d", packetCount, v6Packet.HopLimit, pr.ipTTL)
			packetValid = false
		}

		// Validate UDP header - extract raw bytes for robust parsing
		udpHeaderBytes := udpLayer.LayerContents()
		t.Logf("Packet %d: UDP header bytes: %X", packetCount, udpHeaderBytes)

		if len(udpHeaderBytes) < 8 {
			t.Errorf("Packet %d: UDP header too short (len: %d)", packetCount, len(udpHeaderBytes))
			packetValid = false
		} else {
			// Log gopacket parsed UDP ports for comparison
			if udpPkt, ok := udpLayer.(*layers.UDP); ok {
				t.Logf("Packet %d: gopacket parsed UDP SrcPort: %d, DstPort: %d", packetCount, udpPkt.SrcPort, udpPkt.DstPort)
			}

			// Manual parsing: UDP destination port is bytes 2-3 of UDP header
			manualDstPort := binary.BigEndian.Uint16(udpHeaderBytes[2:4])
			t.Logf("Packet %d: Manually parsed UDP dstPort: %d", packetCount, manualDstPort)

			if manualDstPort != uint16(pr.udpDstPort) {
				t.Errorf("Packet %d: UDP dest port is %d, want %d", packetCount, manualDstPort, pr.udpDstPort)
				packetValid = false
			}
		}

		// Validate MPLS header inside UDP payload
		payload := udpLayer.LayerPayload()
		if len(payload) < 4 {
			t.Errorf("Packet %d: UDP payload too short for MPLS header, len=%d", packetCount, len(payload))
			packetValid = false
		} else {
			mplsHeaderVal := binary.BigEndian.Uint32(payload[:4])
			label := (mplsHeaderVal >> 12) & 0xFFFFF
			bottomOfStack := (mplsHeaderVal >> 8) & 0x1
			mplsTTL := mplsHeaderVal & 0xFF

			t.Logf("Packet %d: %s", packetCount, formatMPLSHeader(payload[:4]))

			if uint64(label) != pr.mplsLabel {
				t.Errorf("Packet %d: Got MPLS Label %d, want %d", packetCount, label, pr.mplsLabel)
				packetValid = false
			}
			if bottomOfStack != 1 {
				t.Errorf("Packet %d: Got MPLS Bottom of Stack bit %d, want 1", packetCount, bottomOfStack)
				packetValid = false
			}
			expectedMPLSTTL := ttl - 1 // Inner packet TTL decremented by 1
			if uint32(mplsTTL) != expectedMPLSTTL {
				t.Errorf("Packet %d: Got MPLS TTL %d, want %d", packetCount, mplsTTL, expectedMPLSTTL)
				packetValid = false
			}
		}

		if packetValid {
			validMplsPacketCount++
			if validMplsPacketCount <= 2 {
				t.Logf("Packet %d: MPLS validation PASSED", packetCount)
			}
		} else {
			t.Logf("Packet %d: MPLS validation FAILED", packetCount)
		}
	}

	// Summary and validation results
	t.Logf("=== PACKET CAPTURE VALIDATION SUMMARY ===")
	t.Logf("Total packets captured: %d", packetCount)
	t.Logf("UDP-IPv6 packets found: %d", mplsPacketCount)
	t.Logf("Valid MPLS-in-UDP packets: %d", validMplsPacketCount)

	if packetCount == 0 {
		t.Errorf("No packets captured on port %s", otgPortName)
	} else if mplsPacketCount == 0 {
		t.Errorf("No UDP-IPv6 packets found in capture on port %s", otgPortName)
	} else if validMplsPacketCount == 0 {
		t.Errorf("No valid MPLS-in-UDP packets found in capture on port %s", otgPortName)
	} else if validMplsPacketCount < (mplsPacketCount / 2) {
		t.Errorf("Many packets (%d/%d) failed validation", mplsPacketCount-validMplsPacketCount, mplsPacketCount)
	} else {
		t.Logf("Packet capture validation PASSED: Found %d valid MPLS-in-UDP packets", validMplsPacketCount)
	}
}

func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	d := gnmi.OC()
	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")
	portList := []*ondatra.Port{p1, p2}
	dutPortAttrs := []attrs.Attributes{dutPort1, dutPort2}

	// Configure interfaces
	for idx, a := range dutPortAttrs {
		p := portList[idx]
		intf := a.NewOCInterface(p.Name(), dut)

		// Configure 100G ports for specific vendors
		if p.PMD() == ondatra.PMD100GBASELR4 && dut.Vendor() != ondatra.CISCO && dut.Vendor() != ondatra.JUNIPER {
			e := intf.GetOrCreateEthernet()
			if !deviations.AutoNegotiateUnsupported(dut) {
				e.AutoNegotiate = ygot.Bool(false)
			}
			if !deviations.DuplexModeUnsupported(dut) {
				e.DuplexMode = oc.Ethernet_DuplexMode_FULL
			}
			if !deviations.PortSpeedUnsupported(dut) {
				e.PortSpeed = oc.IfEthernet_ETHERNET_SPEED_SPEED_100GB
			}
		}
		gnmi.Replace(t, dut, d.Interface(p.Name()).Config(), intf)
	}

	// Configure base policies and network instances
	configureBaseconfig(t, dut)

	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, p1.Name(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, p2.Name(), deviations.DefaultNetworkInstance(dut), 0)
	}

	// Apply policy-based forwarding to source interface
	applyForwardingPolicy(t, dut, p1.Name())

	// Set up static ARP configuration for gRIBI NH entries
	if deviations.GRIBIMACOverrideWithStaticARP(dut) {
		staticARPWithSecondaryIP(t, dut)
	} else if deviations.GRIBIMACOverrideStaticARPStaticRoute(dut) {
		staticARPWithMagicUniversalIP(t, dut)
	}

	// Allow time for configuration to be applied
	time.Sleep(10 * time.Second)
}

// applyForwardingPolicy applies the VRF selection policy to the ingress interface
func applyForwardingPolicy(t *testing.T, dut *ondatra.DUTDevice, ingressPort string) {
	d := &oc.Root{}
	interfaceID := ingressPort
	if deviations.InterfaceRefInterfaceIDFormat(dut) {
		interfaceID = ingressPort + ".0"
	}

	pfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding().Interface(interfaceID)
	pfCfg := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut)).GetOrCreatePolicyForwarding().GetOrCreateInterface(interfaceID)
	pfCfg.ApplyVrfSelectionPolicy = ygot.String(clusterPolicy)
	pfCfg.GetOrCreateInterfaceRef().Interface = ygot.String(ingressPort)
	pfCfg.GetOrCreateInterfaceRef().Subinterface = ygot.Uint32(0)
	gnmi.Replace(t, dut, pfPath.Config(), pfCfg)
}

// configureOTG configures the OTG topology and ports
func configureOTG(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	topo := gosnappi.NewConfig()
	p1 := ate.Port(t, "port1")
	p2 := ate.Port(t, "port2")

	otgPort1.AddToOTG(topo, p1, &dutPort1)
	otgPort2.AddToOTG(topo, p2, &dutPort2)

	// Configure 100G ports to disable FEC for compatibility
	var pmd100GBASELR4 []string
	for _, p := range topo.Ports().Items() {
		port := ate.Port(t, p.Name())
		if port.PMD() == ondatra.PMD100GBASELR4 {
			pmd100GBASELR4 = append(pmd100GBASELR4, port.ID())
		}
	}
	if len(pmd100GBASELR4) > 0 {
		l1Settings := topo.Layer1().Add().SetName("L1").SetPortNames(pmd100GBASELR4)
		l1Settings.SetAutoNegotiate(true).SetIeeeMediaDefaults(false).SetSpeed("speed_100_gbps")
		autoNegotiate := l1Settings.AutoNegotiation()
		autoNegotiate.SetRsFec(false)
	}

	return topo
}

// enableCapture enables packet capture on specified OTG ports
func enableCapture(t *testing.T, otg *otg.OTG, topo gosnappi.Config, otgPortNames []string) {
	for _, port := range otgPortNames {
		topo.Captures().Add().SetName(port).SetPortNames([]string{port}).SetFormat(gosnappi.CaptureFormat.PCAP)
	}
	otg.PushConfig(t, topo)
}

// clearCapture clears packet capture from all OTG ports
func clearCapture(t *testing.T, otg *otg.OTG, topo gosnappi.Config) {
	topo.Captures().Clear()
	otg.PushConfig(t, topo)
}

func randRange(max int, count int) []uint32 {
	rand.New(rand.NewSource(time.Now().UnixNano()))
	var result []uint32
	for len(result) < count {
		result = append(result, uint32(rand.Intn(max)))
	}
	return result
}

// getFlow creates a traffic flow for MPLS-in-UDP testing
func (fa *flowAttr) getFlow(flowType string, name string, dscp uint32) gosnappi.Flow {
	flow := fa.topo.Flows().Add().SetName(name)
	flow.Metrics().SetEnable(true)

	flow.TxRx().Port().SetTxName(fa.srcPort).SetRxNames(fa.dstPorts)
	e1 := flow.Packet().Add().Ethernet()
	e1.Src().SetValue(fa.srcMac)
	e1.Dst().SetValue(fa.dstMac)

	// For MPLS-in-UDP testing, we only support IPv6 flows
	if flowType == "ipv6" {
		v6 := flow.Packet().Add().Ipv6()
		v6.Src().SetValue(fa.src)
		v6.Dst().SetValue(fa.dst)
		v6.HopLimit().SetValue(ttl)
		v6.TrafficClass().SetValue(dscp << 2)
	}

	// Add UDP payload to generate traffic
	udp := flow.Packet().Add().Udp()
	udp.SrcPort().SetValues(randRange(50001, 10000))
	udp.DstPort().SetValues(randRange(50001, 10000))

	return flow
}

// sendTraffic sends traffic flows for the specified duration
func sendTraffic(t *testing.T, args *testArgs, flows []gosnappi.Flow, capture bool) {
	otg := args.ate.OTG()
	args.topo.Flows().Clear().Items()
	args.topo.Flows().Append(flows...)

	otg.PushConfig(t, args.topo)
	otg.StartProtocols(t)

	otgutils.WaitForARP(t, args.ate.OTG(), args.topo, "IPv4")
	otgutils.WaitForARP(t, args.ate.OTG(), args.topo, "IPv6")

	if capture {
		startCapture(t, args.ate)
		defer stopCapture(t, args.ate)
	}

	otg.StartTraffic(t)
	time.Sleep(trafficDuration)
	otg.StopTraffic(t)
}

// validateTrafficFlows verifies traffic flow behavior (pass/fail) based on expected outcome
func validateTrafficFlows(t *testing.T, args *testArgs, flows []gosnappi.Flow, capture bool, match bool) {
	t.Logf("=== TRAFFIC FLOW VALIDATION START (expecting match=%v) ===", match)

	otg := args.ate.OTG()
	sendTraffic(t, args, flows, capture)

	otgutils.LogPortMetrics(t, otg, args.topo)
	otgutils.LogFlowMetrics(t, otg, args.topo)

	for _, flow := range flows {
		outPkts := float32(gnmi.Get(t, otg, gnmi.OTG().Flow(flow.Name()).Counters().OutPkts().State()))
		inPkts := float32(gnmi.Get(t, otg, gnmi.OTG().Flow(flow.Name()).Counters().InPkts().State()))
		lossPct := ((outPkts - inPkts) * 100) / outPkts

		t.Logf("Flow %s: OutPkts=%v, InPkts=%v, LossPct=%v", flow.Name(), outPkts, inPkts, lossPct)

		if outPkts == 0 {
			t.Fatalf("OutPkts for flow %s is 0, want > 0", flow.Name())
		}

		if match {
			// Expecting traffic to pass (0% loss)
			if got := lossPct; got > 0 {
				t.Fatalf("Traffic validation FAILED: Flow %s has %v%% packet loss, want 0%%", flow.Name(), got)
			} else {
				t.Logf("Traffic validation PASSED: Flow %s has 0%% packet loss", flow.Name())
			}
		} else {
			// Expecting traffic to fail (100% loss)
			if got := lossPct; got != 100 {
				t.Fatalf("Traffic validation FAILED: Flow %s has %v%% packet loss, want 100%%", flow.Name(), got)
			} else {
				t.Logf("Traffic validation PASSED: Flow %s has 100%% packet loss", flow.Name())
			}
		}
	}
}

// startCapture starts packet capture on OTG ports
func startCapture(t *testing.T, ate *ondatra.ATEDevice) {
	otg := ate.OTG()
	cs := gosnappi.NewControlState()
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.START)
	otg.SetControlState(t, cs)
}

// stopCapture stops packet capture on OTG ports
func stopCapture(t *testing.T, ate *ondatra.ATEDevice) {
	otg := ate.OTG()
	cs := gosnappi.NewControlState()
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.STOP)
	otg.SetControlState(t, cs)
}

// configStaticArp configures static ARP entries for gRIBI next hop resolution
func configStaticArp(p string, ipv4addr string, macAddr string) *oc.Interface {
	i := &oc.Interface{Name: ygot.String(p)}
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	s := i.GetOrCreateSubinterface(0)
	s4 := s.GetOrCreateIpv4()
	n4 := s4.GetOrCreateNeighbor(ipv4addr)
	n4.LinkLayerAddress = ygot.String(macAddr)
	return i
}

// staticARPWithSecondaryIP configures secondary IPs and static ARP for gRIBI compatibility
func staticARPWithSecondaryIP(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	p2 := dut.Port(t, "port2")
	gnmi.Update(t, dut, gnmi.OC().Interface(p2.Name()).Config(), dutPort2DummyIP.NewOCInterface(p2.Name(), dut))
	gnmi.Update(t, dut, gnmi.OC().Interface(p2.Name()).Config(), configStaticArp(p2.Name(), otgPort2DummyIP.IPv4, magicMac))
}
