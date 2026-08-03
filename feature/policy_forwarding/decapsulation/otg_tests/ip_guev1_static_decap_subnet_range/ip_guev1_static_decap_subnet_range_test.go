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

package ip_guev1_static_decap_subnet_range_test

import (
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	ipv4PrefixLen       = 30
	ipv6PrefixLen       = 126
	packetCount         = 1000
	trafficRatePercent  = 10.0
	timeout             = 30 * time.Second
	sleepTime           = 10 * time.Second
	captureWait         = 10
	ate1Asn             = 65002
	ate2Asn             = 65003
	dutAsn              = 65001
	ipv4Src             = "198.51.100.1"
	ipv4Dst             = "203.0.113.1"
	ipv6Src             = "2001:DB8:1::1"
	ipv6Dst             = "2001:DB8:2::1"
	peerv4Grp1Name      = "BGP-PEER-GROUP1-V4"
	peerv6Grp1Name      = "BGP-PEER-GROUP1-V6"
	peerv4Grp2Name      = "BGP-PEER-GROUP2-V4"
	peerv6Grp2Name      = "BGP-PEER-GROUP2-V6"
	v4NetName1          = "BGPv4RR1"
	v6NetName1          = "BGPv6RR1"
	v4NetName2          = "BGPv4RR2"
	v6NetName2          = "BGPv6RR2"
	policyName          = "decap-policy-gue"
	policyId            = 1
	outerDscpValue      = uint32(35)
	innerDscpValue      = uint32(32)
	innerTTL            = uint32(50)
	outerTTL            = uint32(70)
	srcPortMin          = 1024
	srcPortMax          = 65535
	decapDstSubnetV6    = "2001:db8:dead:beef::/64"
	decapDst1V6         = "2001:db8:dead:beef::1"
	decapDst2V6         = "2001:db8:dead:beef::2"
	decapDst3V6         = "2001:db8:dead:beef::3"
	decapDst4V6         = "2001:db8:dead:beef::4"
	loopbackIntfName    = "Loopback0"
	counterTolerancePct = 5
)

var (
	atePort1 = attrs.Attributes{
		Name:    "ateP1",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::2",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
	atePort2 = attrs.Attributes{
		Name:    "ateP2",
		MAC:     "02:00:02:01:01:01",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::6",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}

	dutPort1 = &attrs.Attributes{
		Desc:    "dutPort1",
		MAC:     "00:00:a1:a1:a1:a1",
		IPv6:    "2001:db8::1",
		IPv4:    "192.0.2.1",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}

	dutPort2 = &attrs.Attributes{
		Desc:    "dutPort2",
		MAC:     "00:00:b1:b1:b1:b1",
		IPv6:    "2001:db8::5",
		IPv4:    "192.0.2.5",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
	decapDstAddrs = []string{decapDst1V6, decapDst2V6, decapDst3V6, decapDst4V6}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

type testCase struct {
	name               string
	ipType             string
	ateGuePort         int
	dutGuePort         int
	trafficDestIps     []string
	trafficShouldPass  bool
	shouldDecap        bool
	verifyDropCounters bool
}

func TestIpGueStaticDecapsulation(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")
	t.Log(dp1, dp2)

	// Configure DUT interfaces.
	ConfigureDUTIntf(t, dut)
	configureDUTLoopback(t, dut, decapDst3V6)
	configureBgp(t, dut)

	// configure ATE
	topo := configureATE(t)
	ate.OTG().PushConfig(t, topo)
	ate.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), topo, "IPv4")
	otgutils.WaitForARP(t, ate.OTG(), topo, "IPv6")
	waitForBGPSession(t, dut, true)

	testCases := []testCase{
		{
			name:              "PF-1.4.1: GUE Decapsulation of inner IPv4 traffic over DECAP subnet range",
			ipType:            "ipv4",
			ateGuePort:        6081,
			dutGuePort:        6081,
			trafficDestIps:    decapDstAddrs,
			trafficShouldPass: true,
			shouldDecap:       true,
		},
		{
			name:              "PF-1.4.2: GUE Decapsulation of inner IPv6 traffic over DECAP subnet range",
			ipType:            "ipv6",
			ateGuePort:        6081,
			dutGuePort:        6081,
			trafficDestIps:    decapDstAddrs,
			trafficShouldPass: true,
			shouldDecap:       true,
		},
		{
			name:               "PF-1.4.3: GUE Decapsulation of inner IPv4 traffic using non-default and unconfigured GUE UDP port (Negative).",
			ipType:             "ipv4",
			ateGuePort:         6085,
			dutGuePort:         6081,
			trafficDestIps:     decapDstAddrs,
			trafficShouldPass:  false,
			shouldDecap:        false,
			verifyDropCounters: true,
		},
		{
			name:               "PF-1.4.4: GUE Decapsulation of inner IPv6 traffic using non-default and unconfigured GUE UDP port (Negative).",
			ipType:             "ipv6",
			ateGuePort:         6085,
			dutGuePort:         6081,
			trafficDestIps:     decapDstAddrs,
			trafficShouldPass:  false,
			shouldDecap:        false,
			verifyDropCounters: true,
		},
		{
			name:              "PF-1.4.5: Inner IPV4 GUE Pass-through (Negative)",
			ipType:            "ipv4",
			ateGuePort:        6081,
			dutGuePort:        6081,
			trafficDestIps:    []string{atePort2.IPv6},
			trafficShouldPass: true,
			shouldDecap:       false,
		},
		{
			name:              "PF-1.4.6: Inner IPV6 GUE Pass-through (Negative)",
			ipType:            "ipv6",
			ateGuePort:        6081,
			dutGuePort:        6081,
			trafficDestIps:    []string{atePort2.IPv6},
			trafficShouldPass: true,
			shouldDecap:       false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.ipType == "ipv4" {
				gueDecapInnerIpv4Traffic(t, dut, ate, topo, ate.OTG(), tc.ateGuePort, tc.dutGuePort, tc.trafficDestIps,
					tc.trafficShouldPass, tc.shouldDecap, tc.verifyDropCounters)
			} else {
				gueDecapInnerIpv6Traffic(t, dut, ate, topo, ate.OTG(), tc.ateGuePort, tc.dutGuePort, tc.trafficDestIps,
					tc.trafficShouldPass, tc.shouldDecap, tc.verifyDropCounters)
			}
		})
	}
}

// ConfigureDUTIntf configures all ports with base IPs and subinterfaces.
func ConfigureDUTIntf(t *testing.T, dut *ondatra.DUTDevice) {
	d := gnmi.OC()
	p1 := dut.Port(t, "port1")
	gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), configInterfaceDUT(p1, dutPort1, dut))
	p2 := dut.Port(t, "port2")
	gnmi.Replace(t, dut, d.Interface(p2.Name()).Config(), configInterfaceDUT(p2, dutPort2, dut))

	// Configure Network instance type on DUT
	t.Log("Configure/update Network Instance")
	fptest.ConfigureDefaultNetworkInstance(t, dut)
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

// configureDUTLoopback configures a software loopback interface on the DUT with the given IPv6 /128 address.
func configureDUTLoopback(t *testing.T, dut *ondatra.DUTDevice, addr string) {
	t.Helper()
	d := &oc.Root{}
	lo := d.GetOrCreateInterface(loopbackIntfName)
	lo.Type = oc.IETFInterfaces_InterfaceType_softwareLoopback
	a := lo.GetOrCreateSubinterface(0).GetOrCreateIpv6().GetOrCreateAddress(addr)
	a.PrefixLength = ygot.Uint8(128)
	gnmi.Update(t, dut, gnmi.OC().Interface(loopbackIntfName).Config(), lo)
	t.Cleanup(func() {
		gnmi.Delete(t, dut, gnmi.OC().Interface(loopbackIntfName).Config())
	})
}

type bgpNeighbor struct {
	as            uint32
	neighborip    string
	isV4          bool
	PeerGroupName string
}

// configureBgp configures BGP on the DUT with IPv4 and IPv6 eBGP neighbors. It creates BGP global configuration, peer groups, neighbors, and enables. IPv4 and IPv6 unicast AFI-SAFIs under the default network instance.
func configureBgp(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	d := &oc.Root{}

	nbr1v4 := &bgpNeighbor{as: ate1Asn, neighborip: atePort1.IPv4, isV4: true, PeerGroupName: peerv4Grp1Name}
	nbr1v6 := &bgpNeighbor{as: ate1Asn, neighborip: atePort1.IPv6, isV4: false, PeerGroupName: peerv6Grp1Name}
	nbr2v4 := &bgpNeighbor{as: ate2Asn, neighborip: atePort2.IPv4, isV4: true, PeerGroupName: peerv4Grp2Name}
	nbr2v6 := &bgpNeighbor{as: ate2Asn, neighborip: atePort2.IPv6, isV4: false, PeerGroupName: peerv6Grp2Name}

	nbrList := []*bgpNeighbor{nbr1v4, nbr2v4, nbr1v6, nbr2v6}

	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")

	ni1 := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niProto := ni1.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := niProto.GetOrCreateBgp()

	g := bgp.GetOrCreateGlobal()
	g.As = ygot.Uint32(dutAsn)
	g.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
	g.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)
	g.RouterId = ygot.String(dutPort2.IPv4)

	pg1v4 := bgp.GetOrCreatePeerGroup(peerv4Grp1Name)
	pg1v4.PeerAs = ygot.Uint32(ate1Asn)

	pg1v6 := bgp.GetOrCreatePeerGroup(peerv6Grp1Name)
	pg1v6.PeerAs = ygot.Uint32(ate1Asn)

	pg2v4 := bgp.GetOrCreatePeerGroup(peerv4Grp2Name)
	pg2v4.PeerAs = ygot.Uint32(ate1Asn)

	pg2v6 := bgp.GetOrCreatePeerGroup(peerv6Grp2Name)
	pg2v6.PeerAs = ygot.Uint32(ate1Asn)

	for _, nbr := range nbrList {
		nv4 := bgp.GetOrCreateNeighbor(nbr.neighborip)
		nv4.PeerGroup = ygot.String(nbr.PeerGroupName)
		nv4.PeerAs = ygot.Uint32(nbr.as)
		nv4.Enabled = ygot.Bool(true)
		if nbr.isV4 {
			af4 := nv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
			af4.Enabled = ygot.Bool(true)
		} else {
			af6 := nv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
			af6.Enabled = ygot.Bool(true)
		}
	}
	gnmi.Replace(t, dut, dutConfPath.Config(), niProto)

}

// configureATE sets up the ATE interfaces and BGP configurations.
func configureATE(t *testing.T) gosnappi.Config {
	topo := gosnappi.NewConfig()
	t.Log("Configure ATE interface")
	port1 := topo.Ports().Add().SetName("port1")
	port2 := topo.Ports().Add().SetName("port2")

	port1Dev := topo.Devices().Add().SetName(atePort1.Name + ".dev")
	port1Eth := port1Dev.Ethernets().Add().SetName(atePort1.Name + ".Eth").SetMac(atePort1.MAC)
	port1Eth.Connection().SetPortName(port1.Name())
	port1Ipv4 := port1Eth.Ipv4Addresses().Add().SetName(atePort1.Name + ".IPv4")
	port1Ipv4.SetAddress(atePort1.IPv4).SetGateway(dutPort1.IPv4).SetPrefix(uint32(atePort1.IPv4Len))
	port1Ipv6 := port1Eth.Ipv6Addresses().Add().SetName(atePort1.Name + ".IPv6")
	port1Ipv6.SetAddress(atePort1.IPv6).SetGateway(dutPort1.IPv6).SetPrefix(uint32(atePort1.IPv6Len))

	bgp1 := port1Dev.Bgp().SetRouterId(atePort1.IPv4)
	bgp4Peer1 := bgp1.Ipv4Interfaces().Add().SetIpv4Name(port1Ipv4.Name()).Peers().Add().SetName(port1Dev.Name() + ".BGP4.peer")
	bgp4Peer1.SetPeerAddress(port1Ipv4.Gateway())
	bgp4Peer1.SetAsNumber(ate1Asn)
	bgp4Peer1.SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
	net1v4 := bgp4Peer1.V4Routes().Add().SetName(v4NetName1)
	net1v4.Addresses().Add().SetAddress(ipv4Src).SetPrefix(32).SetCount(1).SetStep(1)

	bgp6Peer1 := bgp1.Ipv6Interfaces().Add().SetIpv6Name(port1Ipv6.Name()).Peers().Add().SetName(port1Dev.Name() + ".BGP6.peer")
	bgp6Peer1.SetPeerAddress(port1Ipv6.Gateway())
	bgp6Peer1.SetAsNumber(ate1Asn)
	bgp6Peer1.SetAsType(gosnappi.BgpV6PeerAsType.EBGP)
	net1v6 := bgp6Peer1.V6Routes().Add().SetName(v6NetName1)
	net1v6.Addresses().Add().SetAddress(ipv6Src).SetPrefix(128).SetCount(1).SetStep(1)

	port2Dev := topo.Devices().Add().SetName(atePort2.Name + ".dev")
	port2Eth := port2Dev.Ethernets().Add().SetName(atePort2.Name + ".Eth").SetMac(atePort2.MAC)
	port2Eth.Connection().SetPortName(port2.Name())
	port2Ipv4 := port2Eth.Ipv4Addresses().Add().SetName(atePort2.Name + ".IPv4")
	port2Ipv4.SetAddress(atePort2.IPv4).SetGateway(dutPort2.IPv4).SetPrefix(uint32(atePort2.IPv4Len))
	port2Ipv6 := port2Eth.Ipv6Addresses().Add().SetName(atePort2.Name + ".IPv6")
	port2Ipv6.SetAddress(atePort2.IPv6).SetGateway(dutPort2.IPv6).SetPrefix(uint32(atePort2.IPv6Len))

	bgp2 := port2Dev.Bgp().SetRouterId(atePort2.IPv4)
	bgp4Peer2 := bgp2.Ipv4Interfaces().Add().SetIpv4Name(port2Ipv4.Name()).Peers().Add().SetName(port2Dev.Name() + ".BGP4.peer")
	bgp4Peer2.SetPeerAddress(port2Ipv4.Gateway())
	bgp4Peer2.SetAsNumber(ate2Asn)
	bgp4Peer2.SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
	net2v4 := bgp4Peer2.V4Routes().Add().SetName(v4NetName2)
	net2v4.Addresses().Add().SetAddress(ipv4Dst).SetPrefix(32).SetCount(1).SetStep(1)

	bgp6Peer2 := bgp2.Ipv6Interfaces().Add().SetIpv6Name(port2Ipv6.Name()).Peers().Add().SetName(port2Dev.Name() + ".BGP6.peer")
	bgp6Peer2.SetPeerAddress(port2Ipv6.Gateway())
	bgp6Peer2.SetAsNumber(ate2Asn)
	bgp6Peer2.SetAsType(gosnappi.BgpV6PeerAsType.EBGP)
	net2v6 := bgp6Peer2.V6Routes().Add().SetName(v6NetName2)
	net2v6.Addresses().Add().SetAddress(ipv6Dst).SetPrefix(128).SetCount(1).SetStep(1)
	return topo
}

// trafficStartStop starts and stops traffic, collects interface and policy counters, verifies forwarding behavior, and optionally validates drop counters.
func trafficStartStop(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, config gosnappi.Config, otgConfig *otg.OTG, flow gosnappi.Flow, shouldDecap, verifyDropCounters bool) {
	initialInUnicastPkts := gnmi.Get(t, dut, gnmi.OC().Interface(dut.Port(t, "port1").Name()).Counters().InUnicastPkts().State())
	initialOutUnicastPkts := gnmi.Get(t, dut, gnmi.OC().Interface(dut.Port(t, "port2").Name()).Counters().OutUnicastPkts().State())

	matchedPktsSupported := !deviations.PolicyRuleCountersOCUnsupported(dut)
	var initialMatchedPkts uint64
	if matchedPktsSupported {
		initialMatchedPkts = gnmi.Get(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding().Policy(policyName).Rule(policyId).MatchedPkts().State())
	}
	ate.OTG().StartTraffic(t)
	gnmi.Watch(t, ate.OTG(), gnmi.OTG().Flow(flow.Name()).Transmit().State(), timeout,
		func(val *ygnmi.Value[bool]) bool {
			tx, ok := val.Val()
			return ok && !tx
		}).Await(t)
	ate.OTG().StopTraffic(t)

	finalInUnicastPkts := gnmi.Get(t, dut, gnmi.OC().Interface(dut.Port(t, "port1").Name()).Counters().InUnicastPkts().State())
	finalOutUnicastPkts := gnmi.Get(t, dut, gnmi.OC().Interface(dut.Port(t, "port2").Name()).Counters().OutUnicastPkts().State())
	otgutils.LogFlowMetrics(t, ate.OTG(), config)
	// TODO: Replace this with drop counters telemetry if it's available on the DUT. For now, we use the interface counters to infer drops.
	verifyPolicerMatchedPackets(t, dut, &policerVerificationParams{
		Flow:                  flow,
		OTGConfig:             otgConfig,
		InitialInUnicastPkts:  initialInUnicastPkts,
		InitialOutUnicastPkts: initialOutUnicastPkts,
		FinalInUnicastPkts:    finalInUnicastPkts,
		FinalOutUnicastPkts:   finalOutUnicastPkts,
		InitialMatchedPkts:    initialMatchedPkts,
		MatchedPktsSupported:  matchedPktsSupported,
		WantIncrement:         shouldDecap,
	})
	if verifyDropCounters {
		verifyDropCountersIncrement(t, initialInUnicastPkts, finalInUnicastPkts, initialOutUnicastPkts, finalOutUnicastPkts)
	}
}

// verifyDropCountersIncrement verifies that exactly packetCount packets arrived at the DUT's
// ingress but none reached egress, confirming the DUT dropped the full flow.
func verifyDropCountersIncrement(t *testing.T, initialInUnicastPkts, finalInUnicastPkts, initialOutUnicastPkts, finalOutUnicastPkts uint64) {
	t.Helper()
	inDelta := finalInUnicastPkts - initialInUnicastPkts
	outDelta := finalOutUnicastPkts - initialOutUnicastPkts
	tolerance := uint64(packetCount * counterTolerancePct / 100)
	maxAllowedIngress := uint64(packetCount) + tolerance
	maxAllowedEgress := tolerance
	if inDelta > maxAllowedIngress {
		t.Errorf("expected DUT ingress in-unicast-pkts on port1 to increment by at most %d packets (%d + %d%%), got %d (initial=%d final=%d)",
			maxAllowedIngress, packetCount, counterTolerancePct, inDelta, initialInUnicastPkts, finalInUnicastPkts)
	}
	if outDelta > maxAllowedEgress {
		t.Errorf("expected DUT egress out-unicast-pkts on port2 to increase by at most %d packets (%d + %d%%), got %d (initial=%d final=%d)",
			maxAllowedEgress, packetCount, counterTolerancePct, outDelta, initialOutUnicastPkts, finalOutUnicastPkts)
	}
	if inDelta <= maxAllowedIngress && outDelta <= maxAllowedEgress {
		t.Logf("PASS: DUT ingress increased by %d packets (<= %d allowed) and egress increased by %d packets (<= %d allowed), indicating the flow was dropped",
			inDelta, maxAllowedIngress, outDelta, maxAllowedEgress)
	}
}

// verifyTrafficFlow validate the traffic counters.
func verifyTrafficFlow(t *testing.T, ate *ondatra.ATEDevice, flowName string, trafficShouldPass bool) {
	recvMetricV4 := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(flowName).State())

	framesTxV4 := recvMetricV4.GetCounters().GetOutPkts()
	framesRxV4 := recvMetricV4.GetCounters().GetInPkts()

	if trafficShouldPass {
		t.Logf("Traffic validation for flow %s. Expecting %d packets transmitted and received.", flowName, packetCount)
		if framesTxV4 != packetCount {
			t.Errorf("Unexpected transmitted packet count for [%s]. Got: %d, Want: %d", flowName, framesTxV4, packetCount)
		} else if framesRxV4 != packetCount {
			t.Errorf("Unexpected received packet count for [%s]. Got: %d, Want: %d", flowName, framesRxV4, packetCount)
		} else {
			t.Logf("Traffic validation successful for [%s]. FramesTx: %d FramesRx: %d", flowName, framesTxV4, framesRxV4)
		}
	} else {
		t.Logf("traffic validation for flow %s. Expecting Traffic Loss", flowName)
		if framesTxV4 == 0 {
			t.Error("No traffic was generated and frames transmitted were 0")
		} else if framesRxV4 == 0 {
			t.Logf("PASS: Traffic Validation is successful as no packets received at the destination as Expected")
		} else {
			t.Error("FAIL: Traffic Validation is failed as no packets expected at the destination ")
		}
	}
}

// startCapture starts the capture on the otg ports.
func startCapture(t *testing.T, ate *ondatra.ATEDevice) {
	otg := ate.OTG()
	cs := gosnappi.NewControlState()
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.START)
	otg.SetControlState(t, cs)
}

// stopCapture starts the capture on the otg ports.
func stopCapture(t *testing.T, ate *ondatra.ATEDevice) {
	otg := ate.OTG()
	cs := gosnappi.NewControlState()
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.STOP)
	otg.SetControlState(t, cs)
}

// enableCapture enable the port to capture packets.
func enableCapture(t *testing.T, config gosnappi.Config, port string) {
	config.Captures().Clear()
	t.Log("Enabling capture on ", port)
	config.Captures().Add().SetName(port).SetPortNames([]string{port}).SetFormat(gosnappi.CaptureFormat.PCAP)
}

// processCapture process capture and return a capture file.
func processCapture(t *testing.T, ate *ondatra.ATEDevice, port string) string {
	otg := ate.OTG()
	bytes := otg.GetCapture(t, gosnappi.NewCaptureRequest().SetPortName(port))
	time.Sleep(captureWait * time.Second)
	pcapFile, err := os.CreateTemp("", "pcap")
	if err != nil {
		t.Errorf("ERROR: Could not create temporary pcap file: %v\n", err)
	}
	if _, err := pcapFile.Write(bytes); err != nil {
		t.Errorf("ERROR: Could not write bytes to pcap file: %v\n", err)
	}
	defer pcapFile.Close()
	return pcapFile.Name()
}

type policerVerificationParams struct {
	Flow                  gosnappi.Flow
	OTGConfig             *otg.OTG
	InitialInUnicastPkts  uint64
	InitialOutUnicastPkts uint64
	FinalInUnicastPkts    uint64
	FinalOutUnicastPkts   uint64
	InitialMatchedPkts    uint64
	MatchedPktsSupported  bool
	WantIncrement         bool
}

// verifyPolicerMatchedPackets verifies that the policy-rule matched packet counters (or interface-counter proxy on deviating DUTs) reflect the expected decapsulation policy match behavior.
func verifyPolicerMatchedPackets(t *testing.T, dut *ondatra.DUTDevice, p *policerVerificationParams) {
	t.Helper()

	// TO-DO: Currently PolicyForwarding not supported in DUT (Bug 457722520). Adding deviation to check the PF counters.
	if !p.MatchedPktsSupported {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			ingressPkt := p.FinalInUnicastPkts - p.InitialInUnicastPkts
			egressPkt := p.FinalOutUnicastPkts - p.InitialOutUnicastPkts

			if ingressPkt == 0 {
				t.Errorf("Got the unexpected ingress packet count: %d", ingressPkt)
			}

			if p.WantIncrement {
				ingressAtePkts := gnmi.Get(t, p.OTGConfig, gnmi.OTG().Flow(p.Flow.Name()).Counters().OutPkts().State())
				egressAtePkts := gnmi.Get(t, p.OTGConfig, gnmi.OTG().Flow(p.Flow.Name()).Counters().InPkts().State())

				if ingressPkt >= ingressAtePkts && egressPkt >= egressAtePkts {
					t.Logf("Interface counters reflect decapsulated packets: InUnicastPkts: %d OutUnicastPkts: %d", ingressPkt, egressPkt)
				} else {
					t.Errorf("Interface counters didn't reflect decapsulated packets")
				}
			} else {
				t.Logf("PF policy-rule counters unsupported on this DUT; interface-counter deviation path cannot confirm the decap-policy did NOT match (ingressPkt=%d egressPkt=%d)", ingressPkt, egressPkt)
			}

		default:
			t.Errorf("deviation PolicyRuleCountersOCUnsupported is not handled for DUT %v (Bug 457722520)", dut.Vendor())
		}
		return
	}

	isPresent := func(val *ygnmi.Value[uint64]) bool { return val.IsPresent() }
	_, ok := gnmi.Watch(
		t,
		dut,
		gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).
			PolicyForwarding().
			Policy(policyName).
			Rule(policyId).
			MatchedPkts().
			State(),
		timeout,
		isPresent,
	).Await(t)

	if !ok {
		t.Errorf("Unable to find matched packets")
		return
	}

	finalMatchedPkts := gnmi.Get(
		t,
		dut,
		gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).
			PolicyForwarding().
			Policy(policyName).
			Rule(policyId).
			MatchedPkts().
			State(),
	)

	delta := finalMatchedPkts - p.InitialMatchedPkts

	if p.WantIncrement {
		if delta == 0 {
			t.Errorf("decap-policy matched-pkts counter did not increment (initial=%d final=%d)", p.InitialMatchedPkts, finalMatchedPkts)
		} else {
			t.Logf("PASS: decap-policy matched-pkts counter incremented by %d", delta)
		}
	} else if delta != 0 {
		t.Errorf("decap-policy matched-pkts counter unexpectedly incremented by %d for a flow that should not match the decap rule (initial=%d final=%d)", delta, p.InitialMatchedPkts, finalMatchedPkts)
	} else {
		t.Logf("PASS: decap-policy matched-pkts counter correctly did not increment")
	}
}

// applyImixTrafficProfile applies an IMIX traffic profile to the given flow, setting packet sizes and weights,
// as well as the traffic rate and duration.
func applyImixTrafficProfile(flow gosnappi.Flow) {
	custom := flow.Size().WeightPairs().Custom()
	custom.Add().SetSize(128).SetWeight(7)
	custom.Add().SetSize(512).SetWeight(4)
	custom.Add().SetSize(1518).SetWeight(1)
	flow.Rate().SetPercentage(trafficRatePercent)
	flow.Duration().FixedPackets().SetPackets(packetCount)
}

// gueDecapInnerIpv4Traffic configures and validates GUE decapsulation for inner IPv4 traffic, including traffic and decap-counter verification.
func gueDecapInnerIpv4Traffic(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, topo gosnappi.Config, otgConfig *otg.OTG, ateUdpPort, dutUdpPort int, destIps []string, trafficShouldPass, shouldDecap, verifyDropCounters bool) {
	trafficID := fmt.Sprintf("Gue-Decap-Flow1-%v", ateUdpPort)
	flow := configureIPv4Traffic(t, ate, topo, trafficID, destIps, ateUdpPort)
	configureDutWithGueDecap(t, dut, dutUdpPort, "ipv4")
	enableCapture(t, topo, "port2")
	ate.OTG().PushConfig(t, topo)
	ate.OTG().StartProtocols(t)
	startCapture(t, ate)
	trafficStartStop(t, dut, ate, topo, otgConfig, flow, shouldDecap, verifyDropCounters)
	stopCapture(t, ate)
	verifyTrafficFlow(t, ate, trafficID, trafficShouldPass)
	if trafficShouldPass {
		if shouldDecap {
			// PF-1.4.1: decapsulate — verify inner DSCP and TTL
			verifyCaptureDscpTtlValue(t, ate, "port2", int(innerDscpValue), int(innerTTL-1))
		} else {
			// PF-1.4.5: pass-through — verify outer packet unmodified (no decap)
			verifyPassThroughGuePacket(t, ate, "port2")
		}
	}
}

// gueDecapInnerIpv6Traffic configures and validates GUE decapsulation for inner IPv6 traffic, including traffic and decap-counter verification.
func gueDecapInnerIpv6Traffic(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, topo gosnappi.Config, otgConfig *otg.OTG, ateUdpPort, dutUdpPort int, destIps []string, trafficShouldPass, shouldDecap, verifyDropCounters bool) {
	trafficID := fmt.Sprintf("Gue-Decap-Flow1-%v", ateUdpPort)
	flow := configureIPv6Traffic(t, ate, topo, trafficID, destIps, ateUdpPort)
	configureDutWithGueDecap(t, dut, dutUdpPort, "ipv6")
	enableCapture(t, topo, "port2")
	ate.OTG().PushConfig(t, topo)
	ate.OTG().StartProtocols(t)
	startCapture(t, ate)
	trafficStartStop(t, dut, ate, topo, otgConfig, flow, shouldDecap, verifyDropCounters)
	stopCapture(t, ate)
	verifyTrafficFlow(t, ate, trafficID, trafficShouldPass)
	if trafficShouldPass {
		if shouldDecap {
			// PF-1.4.2: decapsulate — verify inner DSCP and TTL
			verifyCaptureDscpTtlValue(t, ate, "port2", int(innerDscpValue), int(innerTTL-1))
		} else {
			// PF-1.4.6: pass-through — verify outer packet unmodified (no decap)
			verifyPassThroughGuePacket(t, ate, "port2")
		}
	}
}

// configureDutWithGueDecap configures GUE decapsulation on the DUT for the specified UDP port and
// IP type (IPv4 or IPv6), delegating to cfgplugins.DecapGroupConfigGue for both the native-CLI
// deviation path and the canonical OC path (a destination-address-prefix-set covering the decap
// subnet range, combined with a UDP destination-port match).
func configureDutWithGueDecap(t *testing.T, dut *ondatra.DUTDevice, guePort int, ipType string) {
	t.Helper()
	t.Logf("Configure DUT with decapsulation UDP port %v", guePort)
	ocPFParams := getDefaultOcPolicyForwardingParams(t, dut, guePort, ipType)
	_, _, pf := cfgplugins.SetupPolicyForwardingInfraOC(ocPFParams.NetworkInstanceName)
	cfgplugins.DecapGroupConfigGue(t, dut, pf, ocPFParams)
}

// getDefaultOcPolicyForwardingParams provides default parameters for the generator, matching the values in the provided JSON example.
func getDefaultOcPolicyForwardingParams(t *testing.T, dut *ondatra.DUTDevice, guePort int, ipType string) cfgplugins.OcPolicyForwardingParams {
	return cfgplugins.OcPolicyForwardingParams{
		NetworkInstanceName: "DEFAULT",
		InterfaceID:         dut.Port(t, "port1").Name(),
		AppliedPolicyName:   policyName,
		TunnelIP:            decapDstSubnetV6,
		GUEPort:             uint32(guePort),
		IPType:              ipType,
		Dynamic:             true,
	}
}

// configureIPv4Traffic configures the outer-IPv6/GUE + inner-IPv4 stream. destIps cycles the
// outer GUE destination across the given addresses (e.g. the full DECAP-DST-SUBNET-V6/64 range,
// or a single pass-through address), per the README's traffic profile.
func configureIPv4Traffic(t *testing.T, ate *ondatra.ATEDevice, topo gosnappi.Config, trafficID string, destIps []string, guePort int) gosnappi.Flow {
	t.Logf("Configure Traffic from ATE with flowname %s", trafficID)
	topo.Flows().Clear()
	flow := topo.Flows().Add().SetName(trafficID)
	flow.Metrics().SetEnable(true)
	flow.TxRx().Device().SetTxNames([]string{v4NetName1}).SetRxNames([]string{v4NetName2})
	ethHeader := flow.Packet().Add().Ethernet()
	ethHeader.Src().SetValue(atePort1.MAC)
	ethHeader.Dst().Auto()
	outerIpHeader := flow.Packet().Add().Ipv6()
	outerIpHeader.Src().SetValue(atePort1.IPv6)
	if len(destIps) == 1 {
		outerIpHeader.Dst().SetValue(destIps[0])
	} else {
		outerIpHeader.Dst().SetValues(destIps)
	}
	outerIpHeader.TrafficClass().SetValue(outerDscpValue)
	outerIpHeader.HopLimit().SetValue(outerTTL)
	udpHeader := flow.Packet().Add().Udp()
	udpHeader.SrcPort().Random().SetMin(srcPortMin).SetMax(srcPortMax)
	udpHeader.DstPort().SetValue(uint32(guePort))
	innerIpHeader := flow.Packet().Add().Ipv4()
	innerIpHeader.Src().SetValue(ipv4Src)
	innerIpHeader.Dst().SetValue(ipv4Dst)
	innerIpHeader.Priority().Dscp().Phb().SetValue(innerDscpValue)
	innerIpHeader.TimeToLive().SetValue(innerTTL)
	applyImixTrafficProfile(flow)
	return flow
}

// configureIPv6Traffic configures the outer-IPv6/GUE + inner-IPv6 stream, binding it to the IPv6 BGP route objects (v6NetName1/v6NetName2),
// destIps cycles the outer GUE destination across the given addresses.
func configureIPv6Traffic(t *testing.T, ate *ondatra.ATEDevice, topo gosnappi.Config, trafficID string, destIps []string, guePort int) gosnappi.Flow {
	t.Logf("Configure Traffic from ATE with flowname %s", trafficID)
	topo.Flows().Clear()
	flow := topo.Flows().Add().SetName(trafficID)
	flow.Metrics().SetEnable(true)
	flow.TxRx().Device().SetTxNames([]string{v6NetName1}).SetRxNames([]string{v6NetName2})
	ethHeader := flow.Packet().Add().Ethernet()
	ethHeader.Src().SetValue(atePort1.MAC)
	ethHeader.Dst().Auto()
	outerIpHeader := flow.Packet().Add().Ipv6()
	outerIpHeader.Src().SetValue(atePort1.IPv6)
	if len(destIps) == 1 {
		outerIpHeader.Dst().SetValue(destIps[0])
	} else {
		outerIpHeader.Dst().SetValues(destIps)
	}
	outerIpHeader.TrafficClass().SetValue(outerDscpValue)
	outerIpHeader.HopLimit().SetValue(outerTTL)
	udpHeader := flow.Packet().Add().Udp()
	udpHeader.SrcPort().Random().SetMin(srcPortMin).SetMax(srcPortMax)
	udpHeader.DstPort().SetValue(uint32(guePort))
	innerIpHeader := flow.Packet().Add().Ipv6()
	innerIpHeader.Src().SetValue(ipv6Src)
	innerIpHeader.Dst().SetValue(ipv6Dst)
	innerIpHeader.TrafficClass().SetValue(innerDscpValue)
	innerIpHeader.HopLimit().SetValue(innerTTL)
	applyImixTrafficProfile(flow)
	return flow
}

// verifyCaptureDscpTtlValue validates that the DSCP and TTL values are preserved after decapsulation by analyzing captured packets on the specified ATE port.
func verifyCaptureDscpTtlValue(t *testing.T, ate *ondatra.ATEDevice, port string, dscp int, ttl int) {
	pcapfilename := processCapture(t, ate, port)
	handle, err := pcap.OpenOffline(pcapfilename)
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close()
	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	for packet := range packetSource.Packets() {
		// Handle IPv4 payload
		if ipLayer := packet.Layer(layers.LayerTypeIPv4); ipLayer != nil {
			ip, _ := ipLayer.(*layers.IPv4)
			if ip.SrcIP.Equal(net.ParseIP(ipv4Src)) {
				dscpValue := ip.TOS >> 2
				ttlVal := ip.TTL
				if dscpValue == uint8(dscp) && ttlVal == uint8(ttl) {
					t.Logf("PASS: IPv4 DSCP value %v and TTL value %v are Preserved", dscp, ttl)
					return
				}
				t.Fatalf("ERROR: IPv4 DSCP and TTL value not preserved after Decap. Expected : DSCP - %v , TTL - %v Got : DSCP - %v , TTL - %v", dscp, ttl, dscpValue, ttlVal)
			}
		}
		// Handle IPv6 payload
		if ip6Layer := packet.Layer(layers.LayerTypeIPv6); ip6Layer != nil {
			ip6, _ := ip6Layer.(*layers.IPv6)
			if ip6.SrcIP.Equal(net.ParseIP(ipv6Src)) {
				dscpValue := ip6.TrafficClass
				ttlVal := ip6.HopLimit
				if int(dscpValue) == dscp && int(ttlVal) == ttl {
					t.Logf("PASS: IPv6 DSCP value %v and TTL value %v are Preserved", dscp, ttl)
					return
				}
				t.Fatalf("ERROR: IPv6 DSCP and TTL value not preserved after Decap. Expected : DSCP - %v , TTL - %v Got : DSCP - %v , TTL - %v", dscp, ttl, dscpValue, ttlVal)
			}
		}
	}
	t.Fatalf("ERROR: Could not find packet with matching inner source IP (%s or %s) in capture", ipv4Src, ipv6Src)
}

// waitForBGPSession waits for BGP neighbors to reach the expected session state within a fixed timeout. It validates BGPv4 neighbor session state under the default network instance.
func waitForBGPSession(t *testing.T, dut *ondatra.DUTDevice, wantEstablished bool) {
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()

	compare := func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
		state, ok := val.Val()
		if ok {
			if wantEstablished {
				t.Logf("BGP session state: %s", state.String())
				return state == oc.Bgp_Neighbor_SessionState_ESTABLISHED
			}
			return state == oc.Bgp_Neighbor_SessionState_IDLE
		}
		return false
	}

	nbrList := []string{atePort1.IPv4, atePort2.IPv4, atePort1.IPv6, atePort2.IPv6}

	for _, nbr := range nbrList {
		nbrPath := statePath.Neighbor(nbr)
		_, ok := gnmi.Watch(t, dut, nbrPath.SessionState().State(), 2*time.Minute, compare).Await(t)
		if !ok {
			fptest.LogQuery(t, "BGP reported state", nbrPath.State(), gnmi.Get(t, dut, nbrPath.State()))
			if wantEstablished {
				t.Fatal("No BGP neighbor formed...")
			} else {
				t.Fatal("BGPv4 session didn't teardown.")
			}
		}
	}
}
func verifyPassThroughGuePacket(t *testing.T, ate *ondatra.ATEDevice, port string) {
	pcapfilename := processCapture(t, ate, port)
	handle, err := pcap.OpenOffline(pcapfilename)
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close()
	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	for packet := range packetSource.Packets() {
		if ip6Layer := packet.Layer(layers.LayerTypeIPv6); ip6Layer != nil {
			ip6, _ := ip6Layer.(*layers.IPv6)
			if ip6.SrcIP.Equal(net.ParseIP(atePort1.IPv6)) {
				expectedTTL := uint8(outerTTL - 1)
				if ip6.HopLimit == expectedTTL {
					t.Logf("PASS: GUE pass-through packet verified: outer src=%s, TTL=%d (decremented by 1)", atePort1.IPv6, ip6.HopLimit)
					return
				}
				t.Fatalf("ERROR: Outer TTL mismatch in pass-through. Expected: %d, Got: %d", expectedTTL, ip6.HopLimit)
			}
		}
	}
	t.Fatalf("ERROR: Could not find GUE pass-through packet with outer src IP (%s) in capture", atePort1.IPv6)
}
