// Copyright 2024 Google LLC
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

package mpls_over_udp_tunnel_hashing_test

import (
	"context"
	"encoding/binary"
	"fmt"
	"math/rand"
	"os"
	"slices"
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
	ethertypeIPv4   = oc.PacketMatchTypes_ETHERTYPE_ETHERTYPE_IPV4
	ethertypeIPv6   = oc.PacketMatchTypes_ETHERTYPE_ETHERTYPE_IPV6
	ipv4PrefixLen   = 30
	ipv6PrefixLen   = 126
	trafficDuration = 15 * time.Second
	clusterPolicy   = "vrf_selection_policy_c"
	seqIDBase       = uint32(10)

	// MPLS-in-UDP test configuration
	mplsLabel       = uint64(100)
	outerIPv6Src    = "2001:db8::1"
	outerIPv6Dst    = "2001:db8::100"
	outerDstUDPPort = uint16(6635) // RFC 7510 standard MPLS-in-UDP port
	outerIPTTL      = uint8(64)
	outerDscp       = uint8(26)
	innerIPv6Prefix = "2001:db8:1::/64"
	ttl             = uint32(100) // Inner packet TTL

	// gRIBI entry IDs for MPLS-in-UDP
	mplsNHID  = uint64(1001)
	mplsNHGID = uint64(2001)

	// Static ARP configuration
	magicIP  = "192.168.1.1"
	magicMac = "02:00:00:00:00:01"

	// Test flow configuration
	dscpEncapA1 = 26

	tolerance = 10

	// OTG capture limitation: Cannot start capture on more than one port belonging to the
	// same resource group or on more than one port behind the same front panel port
	otgMultiPortCaptureSupported = false

	frameSize       = 512
	packetPerSecond = 2000
)

// testArgs holds the objects needed by a test case
type testArgs struct {
	dut    *ondatra.DUTDevice
	ate    *ondatra.ATEDevice
	topo   gosnappi.Config
	client *gribi.Client
}

var (
	atePort1        = attrs.Attributes{Name: "ateP1", MAC: "02:00:01:01:01:01", IPv4: "192.0.2.2", IPv6: "2001:db8::2", IPv4Len: ipv4PrefixLen, IPv6Len: ipv6PrefixLen}
	atePort2        = attrs.Attributes{Name: "ateP2", MAC: "02:00:02:01:01:01", IPv4: "192.0.2.6", IPv6: "2001:db8::6", IPv4Len: ipv4PrefixLen, IPv6Len: ipv6PrefixLen}
	atePort3        = attrs.Attributes{Name: "ateP3", MAC: "02:00:03:01:01:01", IPv4: "192.0.2.10", IPv6: "2001:db8::10", IPv4Len: ipv4PrefixLen, IPv6Len: ipv6PrefixLen}
	atePort4        = attrs.Attributes{Name: "ateP4", MAC: "02:00:04:01:01:01", IPv4: "192.0.2.14", IPv6: "2001:db8::14", IPv4Len: ipv4PrefixLen, IPv6Len: ipv6PrefixLen}
	dutPort1        = &attrs.Attributes{Desc: "dutPort1", MAC: "02:02:01:00:00:01", IPv6: "2001:db8::1", IPv4: "192.0.2.1", IPv4Len: ipv4PrefixLen, IPv6Len: ipv6PrefixLen}
	dutPort2        = &attrs.Attributes{Desc: "dutPort2", MAC: "02:02:02:00:00:01", IPv6: "2001:db8::5", IPv4: "192.0.2.5", IPv4Len: ipv4PrefixLen, IPv6Len: ipv6PrefixLen}
	dutPort3        = &attrs.Attributes{Desc: "dutPort3", MAC: "02:02:03:00:00:01", IPv6: "2001:db8::9", IPv4: "192.0.2.9", IPv4Len: ipv4PrefixLen, IPv6Len: ipv6PrefixLen}
	dutPort4        = &attrs.Attributes{Desc: "dutPort4", MAC: "02:02:04:00:00:01", IPv6: "2001:db8::13", IPv4: "192.0.2.13", IPv4Len: ipv4PrefixLen, IPv6Len: ipv6PrefixLen}
	dutPort2DummyIP = attrs.Attributes{Desc: "dutPort2", IPv4Sec: "192.0.2.21", IPv4LenSec: ipv4PrefixLen}
	otgPort2DummyIP = attrs.Attributes{Desc: "otgPort2", IPv4: "192.0.2.22", IPv4Len: ipv4PrefixLen}
	dutPort3DummyIP = attrs.Attributes{Desc: "dutPort2", IPv4Sec: "192.0.2.25", IPv4LenSec: ipv4PrefixLen}
	otgPort3DummyIP = attrs.Attributes{Desc: "otgPort2", IPv4: "192.0.2.26", IPv4Len: ipv4PrefixLen}
	dutPort4DummyIP = attrs.Attributes{Desc: "dutPort4", IPv4Sec: "192.0.2.29", IPv4LenSec: ipv4PrefixLen}
	otgPort4DummyIP = attrs.Attributes{Desc: "otgPort4", IPv4: "192.0.2.30", IPv4Len: ipv4PrefixLen}

	otgDstPorts = []string{"port2", "port3", "port4"}
	otgSrcPort  = "port1"

	// IPv6 flow configuration for MPLS-in-UDP testing
	fa6 = flowAttr{
		src:      atePort1.IPv6,
		dst:      strings.Split(innerIPv6Prefix, "/")[0], // Extract IPv6 prefix for inner destination
		srcMac:   atePort1.MAC,
		dstMac:   dutPort1.MAC,
		srcPort:  otgSrcPort,
		dstPorts: otgDstPorts,
		topo:     gosnappi.NewConfig(),
	}
	portsTrafficDistribution = []uint64{33, 33, 33}
)

// pbrRule defines a policy-based routing rule configuration
type pbrRule struct {
	sequence  uint32
	etherType oc.NetworkInstance_PolicyForwarding_Policy_Rule_L2_Ethertype_Union
	encapVrf  string
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

// packetResult defines the expected packet attributes for validation
type packetResult struct {
	mplsLabel uint64
	// NOTE: Source UDP port is not validated since it is random
	// udpSrcPort uint16
	udpDstPort uint16
	ipTTL      uint8
	srcIP      string
	dstIP      string
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestMPLSOverUDPTunnelHashing(t *testing.T) {
	ctx := context.Background()
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	// Configure DUT interfaces.
	ConfigureDUTIntf(t, dut)

	// configure ATE
	topo := configureATE(t)
	ate.OTG().PushConfig(t, topo)

	ate.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), topo, "IPv4")
	otgutils.WaitForARP(t, ate.OTG(), topo, "IPv6")

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
	if err := c.AwaitTimeout(ctx, t, 3*time.Minute); err != nil {
		t.Fatalf("Failed to install basic infrastructure entries: %v", err)
	}

	t.Run("Match and Encapsulate using gRIBI AFT Modify", func(t *testing.T) {
		t.Log("Adding MPLS-in-UDP entries")

		entries := []fluent.GRIBIEntry{
			// Create MPLS-in-UDP encapsulation next hop
			fluent.NextHopEntry().
				WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithIndex(mplsNHID).
				AddEncapHeader(
					fluent.MPLSEncapHeader().WithLabels(mplsLabel),
					fluent.UDPV6EncapHeader().
						WithSrcIP(outerIPv6Src).
						WithDstIP(outerIPv6Dst).
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
		}

		wantAddResults := []*client.OpResult{
			fluent.OperationResult().
				WithNextHopOperation(mplsNHID).
				WithProgrammingResult(fluent.InstalledInRIB).
				WithOperationType(constants.Add).
				AsResult(),
			fluent.OperationResult().
				WithNextHopGroupOperation(mplsNHGID).
				WithProgrammingResult(fluent.InstalledInRIB).
				WithOperationType(constants.Add).
				AsResult(),
			fluent.OperationResult().
				WithIPv6Operation(innerIPv6Prefix).
				WithProgrammingResult(fluent.InstalledInRIB).
				WithOperationType(constants.Add).
				AsResult(),
		}
		wantDelResults := []*client.OpResult{
			fluent.OperationResult().
				WithIPv6Operation(innerIPv6Prefix).
				WithProgrammingResult(fluent.InstalledInRIB).
				WithOperationType(constants.Delete).
				AsResult(),
			fluent.OperationResult().
				WithNextHopGroupOperation(mplsNHGID).
				WithProgrammingResult(fluent.InstalledInRIB).
				WithOperationType(constants.Delete).
				AsResult(),
			fluent.OperationResult().
				WithNextHopOperation(mplsNHID).
				WithProgrammingResult(fluent.InstalledInRIB).
				WithOperationType(constants.Delete).
				AsResult(),
		}
		flows := []gosnappi.Flow{fa6.getFlow("ipv6", "ip6mpls", dscpEncapA1)}

		c.AddEntries(t, entries, wantAddResults)

		// Enable capture and send traffic
		expectedPacket := &packetResult{
			mplsLabel:  uint64(mplsLabel),
			udpDstPort: outerDstUDPPort,
			ipTTL:      outerIPTTL,
			srcIP:      outerIPv6Src,
			dstIP:      outerIPv6Dst,
		}

		tcArgs := &testArgs{
			client: &c,
			dut:    dut,
			ate:    ate,
			topo:   topo,
		}

		if otgMultiPortCaptureSupported {
			enableCapture(t, ate.OTG(), topo, otgDstPorts)
			t.Log("Start capture and send traffic")
			sendTraffic(t, tcArgs, flows, true)
			t.Log("Validate captured packet attributes")
			validateMPLSPacketCapture(t, ate, otgDstPorts[0], expectedPacket)
			clearCapture(t, ate.OTG(), topo)
		} else {
			for _, port := range otgDstPorts {
				enableCapture(t, ate.OTG(), topo, []string{port})
				t.Log("Start capture and send traffic")
				sendTraffic(t, tcArgs, flows, true)
				t.Log("Validate captured packet attributes")
				validateMPLSPacketCapture(t, ate, port, expectedPacket)
				clearCapture(t, ate.OTG(), topo)
			}
		}

		// Validate traffic flows
		t.Log("Validate traffic flows")
		validateTrafficFlows(t, ate, topo, tcArgs, flows, false, true)

		// Clean up MPLS entries
		t.Log("Deleting MPLS-in-UDP entries")
		slices.Reverse(entries)
		c.DeleteEntries(t, entries, wantDelResults)

		// Verify traffic fails after deletion
		t.Log("Verify traffic fails after entry deletion")
		validateTrafficFlows(t, ate, topo, tcArgs, flows, false, false)

	})

}

func ConfigureDUTIntf(t *testing.T, dut *ondatra.DUTDevice) {
	d := gnmi.OC()
	p1 := dut.Port(t, "port1")
	gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), configInterfaceDUT(p1, dutPort1, dut))
	p2 := dut.Port(t, "port2")
	gnmi.Replace(t, dut, d.Interface(p2.Name()).Config(), configInterfaceDUT(p2, dutPort2, dut))
	p3 := dut.Port(t, "port3")
	gnmi.Replace(t, dut, d.Interface(p3.Name()).Config(), configInterfaceDUT(p3, dutPort3, dut))
	p4 := dut.Port(t, "port4")
	gnmi.Replace(t, dut, d.Interface(p4.Name()).Config(), configInterfaceDUT(p4, dutPort4, dut))

	// Configure Network instance type on DUT
	t.Log("Configure/update Network Instance")
	fptest.ConfigureDefaultNetworkInstance(t, dut)

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

// configureBaseconfig configures network instances and forwarding policy on the DUT
func configureBaseconfig(t *testing.T, dut *ondatra.DUTDevice) {
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	pf := getPbrPolicy(dut, clusterPolicy)
	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding().Config(), pf)
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

// staticARPWithMagicUniversalIP configures static ARP with magic universal IP
func staticARPWithMagicUniversalIP(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	sb := &gnmi.SetBatch{}
	p2 := dut.Port(t, "port2")
	p3 := dut.Port(t, "port3")
	p4 := dut.Port(t, "port4")

	s := &oc.NetworkInstance_Protocol_Static{
		Prefix: ygot.String(magicIP + "/32"),
		NextHop: map[string]*oc.NetworkInstance_Protocol_Static_NextHop{
			"0": {
				Index: ygot.String("0"),
				InterfaceRef: &oc.NetworkInstance_Protocol_Static_NextHop_InterfaceRef{
					Interface: ygot.String(p2.Name()),
				},
			},
			"1": {
				Index: ygot.String("1"),
				InterfaceRef: &oc.NetworkInstance_Protocol_Static_NextHop_InterfaceRef{
					Interface: ygot.String(p3.Name()),
				},
			},
			"2": {
				Index: ygot.String("2"),
				InterfaceRef: &oc.NetworkInstance_Protocol_Static_NextHop_InterfaceRef{
					Interface: ygot.String(p4.Name()),
				},
			},
		},
	}
	sp := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
	gnmi.BatchUpdate(sb, sp.Static(magicIP+"/32").Config(), s)
	gnmi.BatchUpdate(sb, gnmi.OC().Interface(p2.Name()).Config(), configStaticArp(p2.Name(), magicIP, magicMac))
	gnmi.BatchUpdate(sb, gnmi.OC().Interface(p3.Name()).Config(), configStaticArp(p3.Name(), magicIP, magicMac))
	gnmi.BatchUpdate(sb, gnmi.OC().Interface(p4.Name()).Config(), configStaticArp(p4.Name(), magicIP, magicMac))

	sb.Set(t, dut)
}

// staticARPWithSecondaryIP configures secondary IPs and static ARP for gRIBI compatibility
func staticARPWithSecondaryIP(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	p2 := dut.Port(t, "port2")
	gnmi.Update(t, dut, gnmi.OC().Interface(p2.Name()).Config(), dutPort2DummyIP.NewOCInterface(p2.Name(), dut))
	gnmi.Update(t, dut, gnmi.OC().Interface(p2.Name()).Config(), configStaticArp(p2.Name(), otgPort2DummyIP.IPv4, magicMac))

	p3 := dut.Port(t, "port3")
	gnmi.Update(t, dut, gnmi.OC().Interface(p3.Name()).Config(), dutPort3DummyIP.NewOCInterface(p3.Name(), dut))
	gnmi.Update(t, dut, gnmi.OC().Interface(p3.Name()).Config(), configStaticArp(p3.Name(), otgPort3DummyIP.IPv4, magicMac))

	p4 := dut.Port(t, "port4")
	gnmi.Update(t, dut, gnmi.OC().Interface(p4.Name()).Config(), dutPort4DummyIP.NewOCInterface(p4.Name(), dut))
	gnmi.Update(t, dut, gnmi.OC().Interface(p4.Name()).Config(), configStaticArp(p4.Name(), otgPort4DummyIP.IPv4, magicMac))
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

// Configures the given DUT interface.
func configInterfaceDUT(p *ondatra.Port, a *attrs.Attributes, dut *ondatra.DUTDevice) *oc.Interface {
	i := a.NewOCInterface(p.Name(), dut)
	s4 := i.GetOrCreateSubinterface(0).GetOrCreateIpv4()
	if deviations.InterfaceEnabled(dut) && !deviations.IPv4MissingEnabled(dut) {
		s4.Enabled = ygot.Bool(true)
	}
	i.GetOrCreateSubinterface(0).GetOrCreateIpv6()

	return i
}

// configureATE sets up the ATE interfaces and BGP configurations.
func configureATE(t *testing.T) gosnappi.Config {
	topo := gosnappi.NewConfig()
	t.Log("Configure ATE interface")
	port1 := topo.Ports().Add().SetName("port1")
	port2 := topo.Ports().Add().SetName("port2")
	port3 := topo.Ports().Add().SetName("port3")
	port4 := topo.Ports().Add().SetName("port4")

	port1Dev := topo.Devices().Add().SetName(atePort1.Name + ".dev")
	port1Eth := port1Dev.Ethernets().Add().SetName(atePort1.Name + ".Eth").SetMac(atePort1.MAC)
	port1Eth.Connection().SetPortName(port1.Name())
	port1Ipv4 := port1Eth.Ipv4Addresses().Add().SetName(atePort1.Name + ".IPv4")
	port1Ipv4.SetAddress(atePort1.IPv4).SetGateway(dutPort1.IPv4).SetPrefix(uint32(atePort1.IPv4Len))
	port1Ipv6 := port1Eth.Ipv6Addresses().Add().SetName(atePort1.Name + ".IPv6")
	port1Ipv6.SetAddress(atePort1.IPv6).SetGateway(dutPort1.IPv6).SetPrefix(uint32(atePort1.IPv6Len))

	port2Dev := topo.Devices().Add().SetName(atePort2.Name + ".dev")
	port2Eth := port2Dev.Ethernets().Add().SetName(atePort2.Name + ".Eth").SetMac(atePort2.MAC)
	port2Eth.Connection().SetPortName(port2.Name())
	port2Ipv4 := port2Eth.Ipv4Addresses().Add().SetName(atePort2.Name + ".IPv4")
	port2Ipv4.SetAddress(atePort2.IPv4).SetGateway(dutPort2.IPv4).SetPrefix(uint32(atePort2.IPv4Len))
	port2Ipv6 := port2Eth.Ipv6Addresses().Add().SetName(atePort2.Name + ".IPv6")
	port2Ipv6.SetAddress(atePort2.IPv6).SetGateway(dutPort2.IPv6).SetPrefix(uint32(atePort2.IPv6Len))

	port3Dev := topo.Devices().Add().SetName(atePort3.Name + ".dev")
	port3Eth := port3Dev.Ethernets().Add().SetName(atePort3.Name + ".Eth").SetMac(atePort3.MAC)
	port3Eth.Connection().SetPortName(port3.Name())
	port3Ipv4 := port3Eth.Ipv4Addresses().Add().SetName(atePort3.Name + ".IPv4")
	port3Ipv4.SetAddress(atePort3.IPv4).SetGateway(dutPort3.IPv4).SetPrefix(uint32(atePort3.IPv4Len))
	port3Ipv6 := port3Eth.Ipv6Addresses().Add().SetName(atePort3.Name + ".IPv6")
	port3Ipv6.SetAddress(atePort3.IPv6).SetGateway(dutPort3.IPv6).SetPrefix(uint32(atePort3.IPv6Len))

	port4Dev := topo.Devices().Add().SetName(atePort4.Name + ".dev")
	port4Eth := port4Dev.Ethernets().Add().SetName(atePort4.Name + ".Eth").SetMac(atePort4.MAC)
	port4Eth.Connection().SetPortName(port4.Name())
	port4Ipv4 := port4Eth.Ipv4Addresses().Add().SetName(atePort4.Name + ".IPv4")
	port4Ipv4.SetAddress(atePort4.IPv4).SetGateway(dutPort4.IPv4).SetPrefix(uint32(atePort4.IPv4Len))
	port4Ipv6 := port4Eth.Ipv6Addresses().Add().SetName(atePort4.Name + ".IPv6")
	port4Ipv6.SetAddress(atePort4.IPv6).SetGateway(dutPort4.IPv6).SetPrefix(uint32(atePort4.IPv6Len))

	return topo
}

func programBasicEntries(t *testing.T, dut *ondatra.DUTDevice, c *gribi.Client) {
	t.Log("Setting up basic routing infrastructure for MPLS-in-UDP with ECMP on port2, port3 and port4")

	// Assign unique IDs for NHs and NHG
	nhIDs := []uint64{300, 301, 302} // unique NextHop IDs for port3 and port4
	nhgID := uint64(400)

	var nhEntries []fluent.GRIBIEntry
	var nhOps []*client.OpResult

	// Build next-hops for both ports
	for i, p := range []string{"port2", "port3", "port4"} {
		port := dut.Port(t, p)

		switch {
		case deviations.GRIBIMACOverrideWithStaticARP(dut):
			nh, op := gribi.NHEntry(
				nhIDs[i], "MACwithIp", deviations.DefaultNetworkInstance(dut),
				fluent.InstalledInRIB,
				&gribi.NHOptions{Dest: []string{otgPort2DummyIP.IPv4, otgPort3DummyIP.IPv4, otgPort4DummyIP.IPv4}[i], Mac: magicMac},
			)
			nhEntries = append(nhEntries, nh)
			nhOps = append(nhOps, op)

		case deviations.GRIBIMACOverrideStaticARPStaticRoute(dut):
			nh, op := gribi.NHEntry(
				nhIDs[i], "MACwithInterface", deviations.DefaultNetworkInstance(dut),
				fluent.InstalledInRIB,
				&gribi.NHOptions{Interface: port.Name(), Mac: magicMac, Dest: magicIP},
			)
			nhEntries = append(nhEntries, nh)
			nhOps = append(nhOps, op)

		default:
			nh, op := gribi.NHEntry(
				nhIDs[i], "MACwithInterface", deviations.DefaultNetworkInstance(dut),
				fluent.InstalledInRIB,
				&gribi.NHOptions{Interface: port.Name(), Mac: magicMac},
			)
			nhEntries = append(nhEntries, nh)
			nhOps = append(nhOps, op)
		}
	}

	// Build NHG with both next-hops (ECMP)
	nhMap := map[uint64]uint64{
		nhIDs[0]: 1, // weight 1
		nhIDs[1]: 1, // weight 1
		nhIDs[2]: 1, // weight 1
	}
	nhg, nhgOp := gribi.NHGEntry(nhgID, nhMap, deviations.DefaultNetworkInstance(dut), fluent.InstalledInRIB)
	nhEntries = append(nhEntries, nhg)
	nhOps = append(nhOps, nhgOp)

	// Install all NH + NHG entries
	c.AddEntries(t, nhEntries, nhOps)

	// Add IPv6 route for outer destination to point to the NHG
	c.AddIPv6(t, outerIPv6Dst+"/128", nhgID,
		deviations.DefaultNetworkInstance(dut),
		deviations.DefaultNetworkInstance(dut),
		fluent.InstalledInRIB)

	t.Logf("Installed ECMP route %s/128 via ports 2, 3 and 4", outerIPv6Dst)
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
	udp.SrcPort().SetValues(randRange(30001, 10000))
	udp.DstPort().SetValues(randRange(30001, 10000))

	flow.Size().SetFixed(uint32(frameSize))
	flow.Rate().SetPps(packetPerSecond)
	flow.Duration().FixedPackets().SetPackets(packetPerSecond)

	return flow
}

func randRange(max int, count int) []uint32 {
	rand.New(rand.NewSource(time.Now().UnixNano()))
	var result []uint32
	for len(result) < count {
		result = append(result, uint32(rand.Intn(max)))
	}
	return result
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

// validateTrafficFlows verifies traffic flow behavior (pass/fail) based on expected outcome
func validateTrafficFlows(t *testing.T, ate *ondatra.ATEDevice, ateConfig gosnappi.Config, args *testArgs, flows []gosnappi.Flow, capture bool, match bool) {
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
		if match {
			rxPorts := []string{ateConfig.Ports().Items()[1].Name(), ateConfig.Ports().Items()[2].Name(), ateConfig.Ports().Items()[3].Name()}
			weights := testLoadBalance(t, ate, rxPorts)
			loadBalVal := true
			for idx, weight := range portsTrafficDistribution {
				if got, want := weights[idx], weight; got < (want-tolerance) || got > (want+tolerance) {
					t.Errorf("ECMP Percentage for Aggregate Index: %d: got %d, want %d", idx+1, got, want)
					loadBalVal = false
				}
			}
			if loadBalVal {
				t.Log("Load balancing has been verified on the Port interfaces.")
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

// testLoadBalance to ensure 50:50 Load Balancing
func testLoadBalance(t *testing.T, ate *ondatra.ATEDevice, portNames []string) []uint64 {
	t.Helper()
	var rxs []uint64
	for _, aggName := range portNames {
		metrics := gnmi.Get(t, ate.OTG(), gnmi.OTG().Port(aggName).State())
		rxs = append(rxs, (metrics.GetCounters().GetInFrames()))
	}
	var total uint64
	for _, rx := range rxs {
		total += rx
	}
	for idx, rx := range rxs {
		rxs[idx] = (rx * 100) / total
	}
	return rxs
}
