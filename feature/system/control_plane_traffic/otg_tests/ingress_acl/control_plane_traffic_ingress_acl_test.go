// Copyright 2025 Google LLC
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

package control_plane_ingress_acl_test

import (
	"sort"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ygot/ygot"
)

// Constants for test parameters
const (
	// IPs for DUT interfaces and loopback
	dutPort1IPv4  = "192.0.2.1"
	atePort1IPv4  = "192.0.2.2"
	dutPort1IPv6  = "2001:db8:0:1::1"
	atePort1IPv6  = "2001:db8:0:1::2"
	ipv4PrefixLen = 30
	ipv6PrefixLen = 126

	dutLoopbackIPv4 = "198.51.100.1"
	dutLoopbackIPv6 = "2001:db8::1"

	// Source IPs for testing
	mgmtSrcIPv4    = "192.0.2.100" // Simulated Management Source IP
	mgmtSrcIPv6    = "2001:db8::100"
	unknownSrcIPv4 = "192.0.2.200" // Simulated Unknown Source IP
	unknownSrcIPv6 = "2001:db8::200"

	// ACL Names and Types
	aclNameIPv4 = "CONTROL_PLANE_ACL_IPV4"
	aclNameIPv6 = "CONTROL_PLANE_ACL_IPV6"
	aclTypeIPv4 = oc.Acl_ACL_TYPE_ACL_IPV4
	aclTypeIPv6 = oc.Acl_ACL_TYPE_ACL_IPV6

	// ACL Term Names and Sequence IDs
	grpcTermName = "ALLOW_GRPC"
	sshTermName  = "ALLOW_SSH"
	icmpTermName = "ALLOW_ICMP"
	denyTermName = "DENY_ALL"

	grpcTermSeqID = 10
	sshTermSeqID  = 20
	icmpTermSeqID = 30
	denyTermSeqID = 40 // Must be last for explicit deny

	// Ports and Protocols
	sshPort       = 22
	grpcPort      = 50051 // Standard gRPC port, adjust if different
	ipProtoTCP    = 6
	ipProtoICMP   = 1
	ipProtoICMPv6 = 58
	ipProtoAny    = 0 // For matching any protocol in ACL

	// Traffic flow parameters
	packetCount = 100
	frameSize   = 512 // bytes
	flowRatePPS = 10  // packets per second
)

// Define DUT and ATE interfaces using Ondatra attributes.
var (
	dutPort1 = attrs.Attributes{
		Desc:    "DUT Port 1",
		IPv4:    dutPort1IPv4,
		IPv6:    dutPort1IPv6,
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
	atePort1 = attrs.Attributes{
		Name:    "atePort1",
		IPv4:    atePort1IPv4,
		IPv6:    atePort1IPv6,
		MAC:     "02:00:01:01:01:01", // Example MAC
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
	dutLoopback = attrs.Attributes{
		Desc:    "Loopback ",
		IPv4:    dutLoopbackIPv4,
		IPv6:    dutLoopbackIPv6,
		IPv4Len: 32,
		IPv6Len: 128,
	}
)

// TestMain sets up the test environment.
func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func configureDUTLoopback(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	lb := netutil.LoopbackInterface(t, dut, 0)
	lo0 := gnmi.OC().Interface(lb).Subinterface(0)
	ipv4Addrs := gnmi.LookupAll(t, dut, lo0.Ipv4().AddressAny().State())
	foundV4 := false
	for _, ip := range ipv4Addrs {
		if v, ok := ip.Val(); ok {
			foundV4 = true
			dutLoopback.IPv4 = v.GetIp()
			break
		}
	}
	if !foundV4 {
		lo1 := dutLoopback.NewOCInterface(lb, dut)
		lo1.Type = oc.IETFInterfaces_InterfaceType_softwareLoopback
		gnmi.Update(t, dut, gnmi.OC().Interface(lb).Config(), lo1)
	}
}

// configureDUT configures the DUT interfaces and loopback.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	p1 := dut.Port(t, "port1")

	// Configure DUT Port 1
	gnmi.Replace(t, dut, gnmi.OC().Interface(p1.Name()).Config(), dutPort1.NewOCInterface(p1.Name(), dut))

	// Configure Loopback Interface
	configureDUTLoopback(t, dut)
	t.Logf("DUT configuration applied.")
}

// sortPorts sorts the ports by the testbed port ID.
func sortPorts(ports []*ondatra.Port) []*ondatra.Port {
	sort.SliceStable(ports, func(i, j int) bool {
		return ports[i].ID() < ports[j].ID()
	})
	return ports
}

// configureATE configures the ATE interfaces.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) {
	top := gosnappi.NewConfig()
	atePorts := sortPorts(ate.Ports())
	p0 := atePorts[0]
	top.Ports().Add().SetName(p0.ID())
	srcDev := top.Devices().Add().SetName(atePort1.Name)
	srcEth := srcDev.Ethernets().Add().SetName(atePort1.Name + ".Eth").SetMac(atePort1.MAC)
	srcEth.Connection().SetPortName(p0.ID())
	srcEth.Ipv4Addresses().Add().SetName(atePort1.Name + ".IPv4").SetAddress(atePort1.IPv4).SetGateway(dutPort1.IPv4).SetPrefix(uint32(atePort1.IPv4Len))
	srcEth.Ipv6Addresses().Add().SetName(atePort1.Name + ".IPv6").SetAddress(atePort1.IPv6).SetGateway(dutPort1.IPv6).SetPrefix(uint32(atePort1.IPv6Len))

	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
}

// configureACLs defines and pushes the IPv4 and IPv6 ACLs to the DUT.
func configureACLs(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	aclRoot := &oc.Root{}
	acl := aclRoot.GetOrCreateAcl()

	// --- Define IPv4 ACL ---
	aclSet4 := acl.GetOrCreateAclSet(aclNameIPv4, aclTypeIPv4)
	aclSet4.Description = ygot.String("Control Plane Ingress IPv4 ACL")

	// Term 10: Allow gRPC from Any
	term10Ipv4 := aclSet4.GetOrCreateAclEntry(grpcTermSeqID)
	term10Ipv4.Description = ygot.String(grpcTermName)
	term10Ipv4.GetOrCreateTransport().DestinationPort = oc.UnionUint16(grpcPort)
	term10Ipv4.GetOrCreateIpv4().Protocol = oc.UnionUint8(ipProtoTCP)
	term10Ipv4.GetOrCreateActions().ForwardingAction = oc.Acl_FORWARDING_ACTION_ACCEPT

	// Term 20: Allow SSH from MGMT_SRC
	term20Ipv4 := aclSet4.GetOrCreateAclEntry(sshTermSeqID)
	term20Ipv4.Description = ygot.String(sshTermName)
	term20Ipv4.GetOrCreateTransport().DestinationPort = oc.UnionUint16(sshPort)
	term20Ipv4Ipv4 := term20Ipv4.GetOrCreateIpv4()
	term20Ipv4Ipv4.SourceAddress = ygot.String(mgmtSrcIPv4 + "/32")
	term20Ipv4Ipv4.Protocol = oc.UnionUint8(ipProtoTCP)
	term20Ipv4.GetOrCreateActions().ForwardingAction = oc.Acl_FORWARDING_ACTION_ACCEPT

	// Term 30: Allow ICMP from MGMT_SRC
	term30Ipv4 := aclSet4.GetOrCreateAclEntry(icmpTermSeqID)
	term30Ipv4.Description = ygot.String(icmpTermName)
	term30Ipv4Ipv4 := term30Ipv4.GetOrCreateIpv4()
	term30Ipv4Ipv4.SourceAddress = ygot.String(mgmtSrcIPv4 + "/32")
	term30Ipv4Ipv4.Protocol = oc.UnionUint8(ipProtoICMP)
	term30Ipv4.GetOrCreateActions().ForwardingAction = oc.Acl_FORWARDING_ACTION_ACCEPT

	// Term 40: Explicit Deny All
	term40Ipv4 := aclSet4.GetOrCreateAclEntry(denyTermSeqID)
	term40Ipv4.Description = ygot.String(denyTermName)
	term40Ipv4.GetOrCreateIpv4()
	term40Ipv4.GetOrCreateActions().ForwardingAction = oc.Acl_FORWARDING_ACTION_REJECT

	// --- Define IPv6 ACL ---
	aclSet6 := acl.GetOrCreateAclSet(aclNameIPv6, aclTypeIPv6)
	aclSet6.Description = ygot.String("Control Plane Ingress IPv6 ACL")

	// Term 10: Allow gRPC from Any
	term10Ipv6 := aclSet6.GetOrCreateAclEntry(grpcTermSeqID)
	term10Ipv6.Description = ygot.String(grpcTermName)
	term10Ipv6.GetOrCreateTransport().DestinationPort = oc.UnionUint16(grpcPort)
	term10Ipv6.GetOrCreateIpv6().Protocol = oc.UnionUint8(ipProtoTCP)
	term10Ipv6.GetOrCreateActions().ForwardingAction = oc.Acl_FORWARDING_ACTION_ACCEPT

	// Term 20: Allow SSH from MGMT_SRC
	term20Ipv6 := aclSet6.GetOrCreateAclEntry(sshTermSeqID)
	term20Ipv6.Description = ygot.String(sshTermName)
	term20Ipv6.GetOrCreateTransport().DestinationPort = oc.UnionUint16(sshPort)
	term20Ipv6Ipv6 := term20Ipv6.GetOrCreateIpv6()
	term20Ipv6Ipv6.SourceAddress = ygot.String(mgmtSrcIPv6 + "/128")
	term20Ipv6Ipv6.Protocol = oc.UnionUint8(ipProtoTCP)
	term20Ipv6.GetOrCreateActions().ForwardingAction = oc.Acl_FORWARDING_ACTION_ACCEPT

	// Term 30: Allow ICMPv6 from MGMT_SRC
	term30Ipv6 := aclSet6.GetOrCreateAclEntry(icmpTermSeqID)
	term30Ipv6.Description = ygot.String(icmpTermName)
	term30Ipv6Ipv6 := term30Ipv6.GetOrCreateIpv6()
	term30Ipv6Ipv6.SourceAddress = ygot.String(mgmtSrcIPv6 + "/128")
	term30Ipv6Ipv6.Protocol = oc.UnionUint8(ipProtoICMPv6)
	term30Ipv6.GetOrCreateActions().ForwardingAction = oc.Acl_FORWARDING_ACTION_ACCEPT

	// Term 40: Explicit Deny All
	term40Ipv6 := aclSet6.GetOrCreateAclEntry(denyTermSeqID)
	term40Ipv6.Description = ygot.String(denyTermName)
	term40Ipv6.GetOrCreateIpv6()
	term40Ipv6.GetOrCreateActions().ForwardingAction = oc.Acl_FORWARDING_ACTION_REJECT

	// Push ACL configuration
	t.Log("Pushing ACL configuration...")
	gnmi.Replace(t, dut, gnmi.OC().Acl().Config(), acl)
	t.Log("ACL configuration applied.")
}

// applyACLsToControlPlane applies the configured ACLs to the DUT's control plane ingress.
func applyACLsToControlPlane(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()

	// Apply IPv4 ACL to control plane ingress using individual updates
	ingressv4 := &oc.System_ControlPlaneTraffic_Ingress{}
	ingressv4.GetOrCreateAclSet(aclNameIPv4, aclTypeIPv4)
	gnmi.Update(t, dut, gnmi.OC().System().ControlPlaneTraffic().Ingress().Config(), ingressv4)

	// Apply IPv6 ACL to control plane ingress using individual updates
	ingressv6 := &oc.System_ControlPlaneTraffic_Ingress{}
	ingressv6.GetOrCreateAclSet(aclNameIPv6, aclTypeIPv6)
	gnmi.Update(t, dut, gnmi.OC().System().ControlPlaneTraffic().Ingress().Config(), ingressv6)

	t.Log("ACLs applied to control plane ingress using gnmi.Update.")
}

// getACLMatchedPackets retrieves the matched packet count for a specific ACL entry applied to the control plane.
func getACLMatchedPackets(t *testing.T, dut *ondatra.DUTDevice, aclName string, aclType oc.E_Acl_ACL_TYPE, seqID uint32) uint64 {
	t.Helper()
	counterQuery := gnmi.OC().System().ControlPlaneTraffic().Ingress().AclSet(aclName, aclType).AclEntry(seqID).MatchedPackets().State()
	val := gnmi.Lookup(t, dut, counterQuery)
	count, present := val.Val()
	if !present {
		t.Logf("ACL counter not present for ACL %s, Type %s, Seq %d. Assuming 0.", aclName, aclType, seqID)
		return 0 // Return 0 if the counter path doesn't exist yet
	}
	return count
}

// createFlow defines a traffic flow using OTG/GoSNappi.
func createFlow(t *testing.T, ate *ondatra.ATEDevice, flowName, srcMac, dstMac, srcIP, dstIP string, proto uint8, srcPort, dstPort uint16, isIPv6 bool) gosnappi.Flow {
	t.Helper()
	flow := gosnappi.NewConfig().Flows().Add().SetName(flowName)
	flow.Metrics().SetEnable(true)
	flow.TxRx().Port().SetTxName(atePort1.Name).SetRxNames([]string{atePort1.Name}) // Loopback traffic
	flow.Size().SetFixed(frameSize)
	flow.Rate().SetPps(flowRatePPS)
	flow.Duration().FixedPackets().SetPackets(packetCount)

	eth := flow.Packet().Add().Ethernet()
	eth.Src().SetValue(srcMac)
	eth.Dst().SetValue(dstMac) // Should be DUT's MAC on the connected interface
	if isIPv6 {
		ipv6 := flow.Packet().Add().Ipv6()
		ipv6.Src().SetValue(srcIP)
		ipv6.Dst().SetValue(dstIP)
		ipv6.NextHeader().SetValue(uint32(proto))

		if proto == ipProtoTCP {
			tcp := flow.Packet().Add().Tcp()
			tcp.SrcPort().SetValue(uint32(srcPort))
			tcp.DstPort().SetValue(uint32(dstPort))
		} else if proto == ipProtoICMPv6 {
			icmpv6 := flow.Packet().Add().Icmpv6() // Echo Request
			icmpv6.SetEcho(gosnappi.NewFlowIcmpv6Echo())

		}
	} else {
		ipv4 := flow.Packet().Add().Ipv4()
		ipv4.Src().SetValue(srcIP)
		ipv4.Dst().SetValue(dstIP)
		ipv4.Protocol().SetValue(uint32(proto))

		if proto == ipProtoTCP {
			tcp := flow.Packet().Add().Tcp()
			tcp.SrcPort().SetValue(uint32(srcPort))
			tcp.DstPort().SetValue(uint32(dstPort))
			tcp.CtlSyn().SetValue(1) // Set SYN flag
		} else if proto == ipProtoICMP {
			icmp := flow.Packet().Add().Icmp()
			icmp.SetEcho(gosnappi.NewFlowIcmpEcho())
		}
	}
	return flow
}

// verifyCounters checks if the ACL counter has incremented.
func verifyCounters(t *testing.T, dut *ondatra.DUTDevice, aclName string, aclType oc.E_Acl_ACL_TYPE, seqID uint32, initialCount uint64) {
	t.Helper()
	finalCount := getACLMatchedPackets(t, dut, aclName, aclType, seqID)
	if finalCount <= initialCount {
		t.Errorf("ACL counter for %s, Type %s, Seq %d did not increment: initial %d, final %d", aclName, aclType, seqID, initialCount, finalCount)
	} else {
		t.Logf("ACL counter for %s, Type %s, Seq %d incremented: initial %d, final %d", aclName, aclType, seqID, initialCount, finalCount)
	}
}

// TestControlPlaneACL is the main test function.
func TestControlPlaneACL(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	// 1. Configure DUT interfaces and loopback
	configureDUT(t, dut)

	// 2. Configure ATE interfaces
	configureATE(t, ate)

	// 3. Configure ACLs
	configureACLs(t, dut)

	// 4. Apply ACLs to Control Plane
	applyACLsToControlPlane(t, dut)

	// --- Get DUT's MAC address for traffic generation ---
	// This assumes the ATE learned the DUT's MAC via ARP/ND.
	// A more robust method might involve fetching it via GNMI if needed.
	dutPort1Mac := gnmi.Get(t, ate.OTG(), gnmi.OTG().Interface(atePort1.Name+".Eth").Ipv4Neighbor(dutPort1IPv4).LinkLayerAddress().State())
	// dutPort1Macv6 := gnmi.Get(t, ate.OTG(), gnmi.OTG().Interface(atePort1.Name+".Eth").Ipv6Neighbor(dutPort1IPv6).LinkLayerAddress().State()) // Get IPv6 neighbor MAC if different/needed

	// === Test Case SYS-2.1.1: Verify ingress control-plane ACL permit ===
	t.Run("SYS-2.1.1: Verify Permit", func(t *testing.T) {
		// Get initial counters
		initialICMPv4Count := getACLMatchedPackets(t, dut, aclNameIPv4, aclTypeIPv4, icmpTermSeqID)
		initialSSHv4Count := getACLMatchedPackets(t, dut, aclNameIPv4, aclTypeIPv4, sshTermSeqID)
		initialICMPv6Count := getACLMatchedPackets(t, dut, aclNameIPv6, aclTypeIPv6, icmpTermSeqID)
		initialSSHv6Count := getACLMatchedPackets(t, dut, aclNameIPv6, aclTypeIPv6, sshTermSeqID)

		// Create OTG Traffic Flows
		otgConfig := gosnappi.NewConfig()
		// IPv4 ICMP from MGMT_SRC
		flowICMPv4 := createFlow(t, ate, "Permit_ICMPv4", atePort1.MAC, dutPort1Mac, mgmtSrcIPv4, dutLoopbackIPv4, ipProtoICMP, 0, 0, false)
		otgConfig.Flows().Append(flowICMPv4)
		// IPv4 SSH from MGMT_SRC
		flowSSHv4 := createFlow(t, ate, "Permit_SSHv4", atePort1.MAC, dutPort1Mac, mgmtSrcIPv4, dutLoopbackIPv4, ipProtoTCP, 12345, sshPort, false) // Random source port
		otgConfig.Flows().Append(flowSSHv4)
		// IPv6 ICMP from MGMT_SRC
		flowICMPv6 := createFlow(t, ate, "Permit_ICMPv6", atePort1.MAC, dutPort1Mac, mgmtSrcIPv6, dutLoopbackIPv6, ipProtoICMPv6, 0, 0, true)
		otgConfig.Flows().Append(flowICMPv6)
		// IPv6 SSH from MGMT_SRC
		flowSSHv6 := createFlow(t, ate, "Permit_SSHv6", atePort1.MAC, dutPort1Mac, mgmtSrcIPv6, dutLoopbackIPv6, ipProtoTCP, 12345, sshPort, true) // Random source port
		otgConfig.Flows().Append(flowSSHv6)

		// Start Traffic
		t.Log("Starting Permit Traffic...")
		ate.OTG().PushConfig(t, otgConfig)
		ate.OTG().StartTraffic(t)
		time.Sleep(15 * time.Second) // Allow time for traffic and counter updates
		ate.OTG().StopTraffic(t)
		t.Log("Permit Traffic Stopped.")

		// Verify Flow Metrics (Optional but good practice)
		otgutils.LogFlowMetrics(t, ate.OTG(), otgConfig)
		for _, f := range otgConfig.Flows().Items() {
			recvMetric := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(f.Name()).State())
			if recvMetric.GetCounters().GetOutPkts() != packetCount {
				t.Errorf("Sent packets do not match expected for flow %s: got %d, want %d", f.Name(), recvMetric.GetCounters().GetOutPkts(), packetCount)
			}
			// Note: We don't expect to receive this traffic back on ATE as it's destined for DUT's loopback
		}

		// Verify ACL Counters Increment
		t.Log("Verifying Permit ACL counters...")
		verifyCounters(t, dut, aclNameIPv4, aclTypeIPv4, icmpTermSeqID, initialICMPv4Count)
		verifyCounters(t, dut, aclNameIPv4, aclTypeIPv4, sshTermSeqID, initialSSHv4Count)
		verifyCounters(t, dut, aclNameIPv6, aclTypeIPv6, icmpTermSeqID, initialICMPv6Count)
		verifyCounters(t, dut, aclNameIPv6, aclTypeIPv6, sshTermSeqID, initialSSHv6Count)

		// TODO: Add verification for DUT response (ICMP reply / TCP SYN-ACK)
		// This typically requires packet capture on the ATE or checking DUT state,
		// which adds complexity. Relying on counters is the primary check here.
	})

	// === Test Case SYS-2.1.2: Verify control-plane ACL deny ===
	t.Run("SYS-2.1.2: Verify Deny", func(t *testing.T) {
		// Get initial counters
		initialDenyIPv4Count := getACLMatchedPackets(t, dut, aclNameIPv4, aclTypeIPv4, denyTermSeqID)
		initialDenyIPv6Count := getACLMatchedPackets(t, dut, aclNameIPv6, aclTypeIPv6, denyTermSeqID)

		// Create OTG Traffic Flows
		otgConfig := gosnappi.NewConfig()
		// IPv4 ICMP from UNKNOWN_SRC
		flowICMPv4Deny := createFlow(t, ate, "Deny_ICMPv4", atePort1.MAC, dutPort1Mac, unknownSrcIPv4, dutLoopbackIPv4, ipProtoICMP, 0, 0, false)
		otgConfig.Flows().Append(flowICMPv4Deny)
		// IPv4 SSH from UNKNOWN_SRC
		flowSSHv4Deny := createFlow(t, ate, "Deny_SSHv4", atePort1.MAC, dutPort1Mac, unknownSrcIPv4, dutLoopbackIPv4, ipProtoTCP, 54321, sshPort, false) // Random source port
		otgConfig.Flows().Append(flowSSHv4Deny)
		// IPv6 ICMP from UNKNOWN_SRC
		flowICMPv6Deny := createFlow(t, ate, "Deny_ICMPv6", atePort1.MAC, dutPort1Mac, unknownSrcIPv6, dutLoopbackIPv6, ipProtoICMPv6, 0, 0, true)
		otgConfig.Flows().Append(flowICMPv6Deny)
		// IPv6 SSH from UNKNOWN_SRC
		flowSSHv6Deny := createFlow(t, ate, "Deny_SSHv6", atePort1.MAC, dutPort1Mac, unknownSrcIPv6, dutLoopbackIPv6, ipProtoTCP, 54321, sshPort, true) // Random source port
		otgConfig.Flows().Append(flowSSHv6Deny)

		// Start Traffic
		t.Log("Starting Deny Traffic...")
		ate.OTG().PushConfig(t, otgConfig)
		ate.OTG().StartTraffic(t)
		time.Sleep(15 * time.Second) // Allow time for traffic and counter updates
		ate.OTG().StopTraffic(t)
		t.Log("Deny Traffic Stopped.")

		// Verify Flow Metrics (Optional)
		otgutils.LogFlowMetrics(t, ate.OTG(), otgConfig)
		for _, f := range otgConfig.Flows().Items() {
			recvMetric := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(f.Name()).State())
			if recvMetric.GetCounters().GetOutPkts() != packetCount {
				t.Errorf("Sent packets do not match expected for flow %s: got %d, want %d", f.Name(), recvMetric.GetCounters().GetOutPkts(), packetCount)
			}
		}

		// Verify Explicit Deny ACL Counters Increment
		t.Log("Verifying Deny ACL counters...")
		verifyCounters(t, dut, aclNameIPv4, aclTypeIPv4, denyTermSeqID, initialDenyIPv4Count)
		verifyCounters(t, dut, aclNameIPv6, aclTypeIPv6, denyTermSeqID, initialDenyIPv6Count)

	})

}
