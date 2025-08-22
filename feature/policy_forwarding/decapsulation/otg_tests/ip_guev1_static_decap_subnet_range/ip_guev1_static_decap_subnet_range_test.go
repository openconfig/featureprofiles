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
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	packetSize       = 512
	ipv4PrefixLen    = 30
	ipv6PrefixLen    = 126
	packetPerSecond  = 1000
	timeout          = 30
	trafficSleepTime = 10
	captureWait      = 10
	ate1Asn          = 65002
	ate2Asn          = 65003
	dutAsn           = 65001
	ipv4Src          = "198.51.100.1"
	ipv4Dst          = "203.0.113.1"
	ipv6Src          = "2001:DB8:1::1"
	ipv6Dst          = "2001:DB8:2::1"
	peerv4Grp1Name   = "BGP-PEER-GROUP1-V4"
	peerv6Grp1Name   = "BGP-PEER-GROUP1-V6"
	peerv4Grp2Name   = "BGP-PEER-GROUP2-V4"
	peerv6Grp2Name   = "BGP-PEER-GROUP2-V6"
	v4NetName1       = "BGPv4RR1"
	v6NetName1       = "BGPv6RR1"
	v4NetName2       = "BGPv4RR2"
	v6NetName2       = "BGPv6RR2"
	tunIp            = "4.4.4.4"
	policyName       = "decap-policy-gue"
	policyId         = 1
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
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

type testCase struct {
	name              string
	ipType            string
	ateGuePort        int
	dutGuePort        int
	trafficDestIp     string
	trafficShouldPass bool
	verifyCounters    bool
	txRxCountersMatch bool
}

func TestIpGue1StaticDecapsulation(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")
	t.Log(dp1, dp2)

	// Configure DUT interfaces.
	ConfigureDUTIntf(t, dut)
	configureBgp(t, dut)

	// configure ATE
	topo := configureATE(t)
	ate.OTG().PushConfig(t, topo)
	enableCapture(t, topo, "port2")

	testCases := []testCase{
		{
			name:              "PF-1.4.1: GUE Decapsulation of inner IPv4 traffic over DECAP subnet range",
			ipType:            "ipv4",
			ateGuePort:        6081,
			dutGuePort:        6081,
			trafficDestIp:     tunIp,
			trafficShouldPass: true,
			verifyCounters:    true,
			txRxCountersMatch: true,
		},
		{
			name:              "PF-1.4.2: GUE Decapsulation of inner IPv6 traffic over DECAP subnet range",
			ipType:            "ipv6",
			ateGuePort:        6081,
			dutGuePort:        6081,
			trafficDestIp:     tunIp,
			trafficShouldPass: true,
			verifyCounters:    true,
			txRxCountersMatch: true,
		},
		{
			name:              "PF-1.4.3: GUE Decapsulation of inner IPv4 traffic using non-default and unconfigured GUE UDP port (Negative).",
			ipType:            "ipv4",
			ateGuePort:        6085,
			dutGuePort:        6081,
			trafficDestIp:     tunIp,
			trafficShouldPass: false,
			verifyCounters:    true,
			txRxCountersMatch: false,
		},
		{
			name:              "PF-1.4.4: GUE Decapsulation of inner IPv6 traffic using non-default and unconfigured GUE UDP port (Negative).",
			ipType:            "ipv6",
			ateGuePort:        6085,
			dutGuePort:        6081,
			trafficDestIp:     tunIp,
			trafficShouldPass: false,
			verifyCounters:    true,
			txRxCountersMatch: false,
		},
		{
			name:              "PF-1.4.5: Inner IPV4 GUE Pass-through (Negative)",
			ipType:            "ipv4",
			ateGuePort:        6081,
			dutGuePort:        6081,
			trafficDestIp:     atePort2.IPv4,
			trafficShouldPass: true,
			verifyCounters:    false,
			txRxCountersMatch: false,
		},
		{
			name:              "PF-1.4.6: Inner IPV6 GUE Pass-through (Negative)",
			ipType:            "ipv6",
			ateGuePort:        6081,
			dutGuePort:        6081,
			trafficDestIp:     atePort2.IPv4,
			trafficShouldPass: true,
			verifyCounters:    false,
			txRxCountersMatch: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.ipType == "ipv4" {
				gueDecapInnerIpv4Traffic(t, dut, ate, topo, tc.ateGuePort, tc.dutGuePort, tc.trafficDestIp, tc.trafficShouldPass, tc.verifyCounters, tc.txRxCountersMatch)
			} else {
				gueDecapInnerIpv6Traffic(t, dut, ate, topo, tc.ateGuePort, tc.dutGuePort, tc.trafficDestIp, tc.trafficShouldPass, tc.verifyCounters, tc.txRxCountersMatch)
			}
		})
	}
}

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

type bgpNeighbor struct {
	as            uint32
	neighborip    string
	isV4          bool
	PeerGroupName string
}

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

func trafficStartStop(t *testing.T, ate *ondatra.ATEDevice, config gosnappi.Config) {
	ate.OTG().StartTraffic(t)
	time.Sleep(trafficSleepTime * time.Second)
	ate.OTG().StopTraffic(t)
	otgutils.LogFlowMetrics(t, ate.OTG(), config)
}

func protocolStart(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, config gosnappi.Config) {
	ate.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), config, "IPv4")
	otgutils.WaitForARP(t, ate.OTG(), config, "IPv6")
	verifyBGPTelemetry(t, dut)
}

func verifyTrafficFlow(t *testing.T, ate *ondatra.ATEDevice, flowName string, trafficShouldPass bool) {
	recvMetricV4 := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(flowName).State())

	framesTxV4 := recvMetricV4.GetCounters().GetOutPkts()
	framesRxV4 := recvMetricV4.GetCounters().GetInPkts()

	if trafficShouldPass {
		t.Logf("traffic validation for flow %s. Expecting Traffic TX = RX.", flowName)
		if framesTxV4 == 0 {
			t.Error("No traffic was generated and frames transmitted were 0")
		} else if framesRxV4 == framesTxV4 {
			t.Logf("Traffic validation successful for [%s] FramesTx: %d FramesRx: %d", flowName, framesTxV4, framesRxV4)
		} else {
			t.Errorf("Traffic validation failed for [%s] FramesTx: %d FramesRx: %d", flowName, framesTxV4, framesRxV4)
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

// startCapture starts the capture on the otg ports
func startCapture(t *testing.T, ate *ondatra.ATEDevice) {
	otg := ate.OTG()
	cs := gosnappi.NewControlState()
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.START)
	otg.SetControlState(t, cs)
}

// stopCapture starts the capture on the otg ports
func stopCapture(t *testing.T, ate *ondatra.ATEDevice) {
	otg := ate.OTG()
	cs := gosnappi.NewControlState()
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.STOP)
	otg.SetControlState(t, cs)
}

func enableCapture(t *testing.T, config gosnappi.Config, port string) {
	config.Captures().Clear()
	t.Log("Enabling capture on ", port)
	config.Captures().Add().SetName(port).SetPortNames([]string{port}).SetFormat(gosnappi.CaptureFormat.PCAP)
}

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

func verify_policer_matched_packets(t *testing.T, dut *ondatra.DUTDevice) uint64 {
	matchpackets := uint64(0)
	const timeout = 10 * time.Second
	isPresent := func(val *ygnmi.Value[uint64]) bool { return val.IsPresent() }
	if deviations.PolicyRuleCountersOCUnsupported(dut) {
		t.Logf("Returning Matched Packet as Zero value due to Bug 425628787")
	} else {
		_, ok := gnmi.Watch(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding().Policy(policyName).Rule(policyId).MatchedPkts().State(), timeout, isPresent).Await(t)
		if !ok {
			t.Errorf("Unable to find matched packets")
		}
		matchpackets = gnmi.Get(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding().Policy(policyName).Rule(policyId).MatchedPkts().State())

	}
	return matchpackets
}

func compare_counters(t *testing.T, intialpacket uint64, finalpacket uint64, countersmatch bool) {
	t.Logf("Policer counters Before Traffic %v", intialpacket)
	t.Logf("Policer counters After Traffic %v", finalpacket)
	if countersmatch {
		t.Logf("Traffic Packet Counters on DUT based on Policer. Expecting Packet Increment after Traffic")
		if intialpacket == 0 {
			t.Errorf("Fail : Unable to find the policer matched packets. Please refer the bug ID #425628787")
		} else if finalpacket-intialpacket >= packetPerSecond {
			t.Logf("Pass : policer counters got incremented after start and stop traffic")
		} else {
			t.Errorf("Fail : policer counters not incremented after start and stop traffic")
		}
	} else {
		t.Logf("Traffic Packet Counters on DUT based on Policer. Expecting no packet Increment after Traffic ")
		if intialpacket == 0 {
			t.Errorf("Fail : Unable to find the policer matched packets. Please refer the bug ID #425628787")
		} else if finalpacket-intialpacket == 0 {
			t.Logf("Pass : policer counters did not incremented as expected")
		} else {
			t.Errorf("Fail : policer counters incremented unexpectedly")
		}
	}
}

func gueDecapInnerIpv4Traffic(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, topo gosnappi.Config, ateUdpPort int, dutUdpPort int, destIp string, trafficValidation bool, verifyCounters bool, countersMatch bool) {
	trafficID := fmt.Sprintf("Gue-Decap-Flow1-%v", ateUdpPort)
	configureIPv4Traffic(t, ate, topo, trafficID, ateUdpPort, destIp)
	configureDutWithGueDecap(t, dut, dutUdpPort, "ipv4")
	protocolStart(t, dut, ate, topo)
	startCapture(t, ate)
	intialpacket1 := verify_policer_matched_packets(t, dut)
	trafficStartStop(t, ate, topo)
	stopCapture(t, ate)
	if trafficValidation {
		verifyTrafficFlow(t, ate, trafficID, true)
		verifyCaptureDscpTtlValue(t, ate, "port2", 32, 49)
	} else {
		verifyTrafficFlow(t, ate, trafficID, false)
	}
	finalpacket1 := verify_policer_matched_packets(t, dut)
	if verifyCounters {
		compare_counters(t, intialpacket1, finalpacket1, countersMatch)
	}
}

func gueDecapInnerIpv6Traffic(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, topo gosnappi.Config, ateUdpPort int, dutUdpPort int, destIp string, trafficValidation bool, verifyCounters bool, countersMatch bool) {
	trafficID := fmt.Sprintf("Gue-Decap-Flow1-%v", ateUdpPort)
	configureIPv6Traffic(t, ate, topo, trafficID, ateUdpPort, destIp)
	configureDutWithGueDecap(t, dut, dutUdpPort, "ipv6")
	protocolStart(t, dut, ate, topo)
	intialpacket1 := verify_policer_matched_packets(t, dut)
	startCapture(t, ate)
	trafficStartStop(t, ate, topo)
	stopCapture(t, ate)
	if trafficValidation {
		verifyTrafficFlow(t, ate, trafficID, true)
		verifyCaptureDscpTtlValue(t, ate, "port2", 32, 49)
	} else {
		verifyTrafficFlow(t, ate, trafficID, false)
	}
	finalpacket1 := verify_policer_matched_packets(t, dut)
	if verifyCounters {
		compare_counters(t, intialpacket1, finalpacket1, countersMatch)
	}
}

func configureDutWithGueDecap(t *testing.T, dut *ondatra.DUTDevice, guePort int, ipType string) {
	t.Logf("Configure DUT with decapsulation UDP port %v", guePort)
	ocPFParams := getDefaultOcPolicyForwardingParams(t, dut, guePort, ipType)
	_, _, pf := cfgplugins.SetupPolicyForwardingInfraOC(ocPFParams.NetworkInstanceName)
	cfgplugins.DecapGroupConfigGue(t, dut, pf, ocPFParams)
}

// getDefaultOcPolicyForwardingParams provides default parameters for the generator,
// matching the values in the provided JSON example.
func getDefaultOcPolicyForwardingParams(t *testing.T, dut *ondatra.DUTDevice, guePort int, ipType string) cfgplugins.OcPolicyForwardingParams {
	return cfgplugins.OcPolicyForwardingParams{
		NetworkInstanceName: "DEFAULT",
		InterfaceID:         dut.Port(t, "port1").Name(),
		AppliedPolicyName:   policyName,
		TunnelIP:            tunIp,
		GuePort:             uint32(guePort),
		IpType:              ipType,
		Dynamic:             true,
	}
}

func configureIPv4Traffic(t *testing.T, ate *ondatra.ATEDevice, topo gosnappi.Config, trafficID string,
	guePort int, destIp string) {
	t.Logf("Configure Traffic from ATE with flowname %s", trafficID)
	topo.Flows().Clear()
	flow := topo.Flows().Add().SetName(trafficID)
	flow.Metrics().SetEnable(true)
	flow.TxRx().Device().SetTxNames([]string{v4NetName1}).SetRxNames([]string{v4NetName2})
	ethHeader := flow.Packet().Add().Ethernet()
	ethHeader.Src().SetValue(atePort1.MAC)
	ethHeader.Dst().Auto()
	outerIpHeader := flow.Packet().Add().Ipv4()
	outerIpHeader.Src().SetValue(atePort1.IPv4)
	outerIpHeader.Dst().SetValue(destIp)
	outerIpHeader.Priority().Dscp().Phb().SetValue(uint32(35))
	outerIpHeader.TimeToLive().SetValue(uint32(70))
	udpHeader := flow.Packet().Add().Udp()
	udpHeader.SrcPort().SetValue(30000)
	udpHeader.DstPort().SetValue(uint32(guePort))
	innerIpHeader := flow.Packet().Add().Ipv4()
	innerIpHeader.Src().SetValue(ipv4Src)
	innerIpHeader.Dst().SetValue(ipv4Dst)
	innerIpHeader.Priority().Dscp().Phb().SetValue(uint32(32))
	innerIpHeader.TimeToLive().SetValue(uint32(50))
	flow.Size().SetFixed(uint32(packetSize))
	flow.Rate().SetPps(packetPerSecond)
	flow.Duration().FixedPackets().SetPackets(packetPerSecond)
	ate.OTG().PushConfig(t, topo)
}

func configureIPv6Traffic(t *testing.T, ate *ondatra.ATEDevice, topo gosnappi.Config, trafficID string,
	guePort int, destIp string) {
	t.Logf("Configure Traffic from ATE with flowname %s", trafficID)
	topo.Flows().Clear()
	flow := topo.Flows().Add().SetName(trafficID)
	flow.Metrics().SetEnable(true)
	flow.TxRx().Device().SetTxNames([]string{v4NetName1}).SetRxNames([]string{v4NetName2})
	ethHeader := flow.Packet().Add().Ethernet()
	ethHeader.Src().SetValue(atePort1.MAC)
	ethHeader.Dst().Auto()
	outerIpHeader := flow.Packet().Add().Ipv4()
	outerIpHeader.Src().SetValue(atePort1.IPv4)
	outerIpHeader.Dst().SetValue(destIp)
	outerIpHeader.Priority().Dscp().Phb().SetValue(uint32(35))
	outerIpHeader.TimeToLive().SetValue(uint32(70))
	udpHeader := flow.Packet().Add().Udp()
	udpHeader.SrcPort().SetValue(30000)
	udpHeader.DstPort().SetValue(uint32(guePort))
	innerIpHeader := flow.Packet().Add().Ipv6()
	innerIpHeader.Src().SetValue(ipv6Src)
	innerIpHeader.Dst().SetValue(ipv6Dst)
	innerIpHeader.TrafficClass().SetValue(uint32(32))
	innerIpHeader.HopLimit().SetValue(uint32(50))
	flow.Size().SetFixed(uint32(packetSize))
	flow.Rate().SetPps(packetPerSecond)
	flow.Duration().FixedPackets().SetPackets(packetPerSecond)
	ate.OTG().PushConfig(t, topo)
}

func verifyCaptureDscpTtlValue(t *testing.T, ate *ondatra.ATEDevice, port string, dscp int, ttl int) {
	pcapfilename := processCapture(t, ate, port)
	handle, err := pcap.OpenOffline(pcapfilename)
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close()
	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	for packet := range packetSource.Packets() {
		if ipLayer := packet.Layer(layers.LayerTypeIPv4); ipLayer != nil {
			ip, _ := ipLayer.(*layers.IPv4)
			if ip.SrcIP.Equal(net.ParseIP(ipv4Src)) {
				dscpValue := ip.TOS >> 2
				ttlVal := ip.TTL
				if dscpValue == uint8(dscp) && ttlVal == uint8(ttl) {
					t.Logf("PASS: DSCP value %v and TTL value %v are Preserved", dscp, ttl)
					break
				} else {
					t.Fatalf("ERROR: DSCP and TTL value not preserved after Decap. Expected : DSCP - %v , TTL -%v Got : DSCP - %v , TTL -%v", dscp, ttl, dscpValue, ttlVal)
				}

			}

		}
	}
}

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

	nbrListv4 := []string{atePort1.IPv4, atePort2.IPv4}

	for _, nbr := range nbrListv4 {
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

func verifyBGPTelemetry(t *testing.T, dut *ondatra.DUTDevice) {
	t.Log("Waiting for BGPv4 neighbor to establish...")
	waitForBGPSession(t, dut, true)
}
