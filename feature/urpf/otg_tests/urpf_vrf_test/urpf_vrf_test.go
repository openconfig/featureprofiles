// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package urpf_nondefault_ni_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ygot/ygot"
)

const (
	plenIPv4           = 30
	plenIPv6           = 126
	dutAS              = 100
	ateAS1             = 200 // eBGP peer
	ateAS2             = 100 // iBGP peer
	routeCount         = 1
	tolerance          = 2
	nonDefaultVRF      = "VRF-1"
	importCommunity    = "65002:200"
	exportCommunity    = "65001:100"
	loopbackIntfName   = "loopback0"
	udpDestPort        = 6080
	trafficDuration    = 20 * time.Second
	ratePPS            = 100
	flowSize           = 512
	packetsToSend      = 3000
	nexthopGroupNameV4 = "GUE-NHG"
	nexthopGroupNameV6 = "GUE-NHGv6"
	GUEPolicyV4Name    = "GUE-Policy-V4"
	GUEPolicyV6Name    = "GUE-Policy-V6"
	GUEDstIPv4         = "198.50.100.1"
	isDefaultVRF       = true
)

// IP addresses and prefixes
var (
	// DUT interfaces
	dutPort1    = attrs.Attributes{Desc: "DUT to ATE Port1 (eBGP)", IPv4: "192.0.2.1", IPv6: "2001:db8:1::1", MAC: "02:00:01:01:01:01", IPv4Len: plenIPv4, IPv6Len: plenIPv6}
	dutPort2    = attrs.Attributes{Desc: "DUT to ATE Port2 (iBGP)", IPv4: "192.0.2.5", IPv6: "2001:db8:2::1", MAC: "02:00:03:01:01:01", IPv4Len: plenIPv4, IPv6Len: plenIPv6}
	dutLoopback = attrs.Attributes{Desc: "DUT Loopback for GUE", IPv4: "198.51.100.1", IPv6: "2001:db8:100::1", IPv4Len: 32, IPv6Len: 128}

	// ATE interfaces
	atePort1 = attrs.Attributes{Name: "ateP1", IPv4: "192.0.2.2", IPv6: "2001:db8:1::2", MAC: "02:00:02:01:01:01", IPv4Len: plenIPv4, IPv6Len: plenIPv6}
	atePort2 = attrs.Attributes{Name: "ateP2", IPv4: "192.0.2.6", IPv6: "2001:db8:2::2", MAC: "02:00:04:01:01:01", IPv4Len: plenIPv4, IPv6Len: plenIPv6}

	// Prefixes advertised by ATE
	// Valid source prefixes advertised from ATE Port 1
	ateAdvIPv4Prefix1 = "198.18.1.0"
	ateAdvIPv6Prefix1 = "2001:db8:10::"
	prefix1Len        = 24
	prefix1LenV6      = 64

	// Invalid source prefixes advertised from ATE Port 1 (but rejected by DUT policy)
	ateAdvIPv4Prefix2 = "198.18.2.0"
	ateAdvIPv6Prefix2 = "3001:db8:10::"

	// Destination prefixes advertised from ATE Port 2
	ateAdvIPv4Prefix3 = "198.18.3.0"
	ateAdvIPv6Prefix3 = "4001:db8:10::"
	prefix3Len        = 24
	prefix3LenV6      = 64

	dstAddr          = []string{GUEDstIPv4}
	defaultNIName    = strings.ToLower("DEFAULT")
	staticRoutePfxV4 = "/24"
	staticRoutePfxV6 = "/64"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// configureDUT configures the DUT with interfaces, a non-default VRF, BGP, route policies for leaking and rejecting routes, and uRPF.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) *gnmi.SetBatch {
	t.Helper()
	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")
	intBatch := new(gnmi.SetBatch)
	t.Logf("Configuring Interfaces")
	configureDUTInterface(t, dut, intBatch, &dutPort1, p1, true)
	configureDUTInterface(t, dut, intBatch, &dutPort2, p2, false)

	configureDUTLoopback(t, dut, intBatch)
	t.Log("Configuring Hardware Init")
	configureHardwareInit(t, dut)

	cfgplugins.EnableDefaultNetworkInstanceBgp(t, dut, dutAS)
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	t.Log("Configuring Network Instances")
	defaultNI := cfgplugins.ConfigureNetworkInstance(t, dut, defaultNIName, isDefaultVRF)
	nonDefaultNI := cfgplugins.ConfigureNetworkInstance(t, dut, nonDefaultVRF, !isDefaultVRF)
	cfgplugins.ConfigureBGPNeighbor(t, dut, nonDefaultNI, dutPort1.IPv4, atePort1.IPv4, dutAS, ateAS1, "IPv4", true)
	cfgplugins.ConfigureBGPNeighbor(t, dut, nonDefaultNI, dutPort1.IPv4, atePort1.IPv6, dutAS, ateAS1, "IPv6", true)
	cfgplugins.ConfigureBGPNeighbor(t, dut, defaultNI, dutPort2.IPv4, atePort2.IPv4, dutAS, ateAS2, "IPv4", true)
	cfgplugins.ConfigureBGPNeighbor(t, dut, defaultNI, dutPort2.IPv4, atePort2.IPv6, dutAS, ateAS2, "IPv6", true)

	cfgplugins.UpdateNetworkInstanceOnDut(t, dut, defaultNIName, defaultNI)
	cfgplugins.UpdateNetworkInstanceOnDut(t, dut, nonDefaultVRF, nonDefaultNI)
	configureDUTPort(t, dut, intBatch, &dutPort1, p1, nonDefaultVRF)
	intBatch.Set(t, dut)
	return intBatch
}

// configureDUTInterface configure interfaces on DUT with URPF config.
func configureDUTInterface(t *testing.T, dut *ondatra.DUTDevice, intBatch *gnmi.SetBatch, attrs *attrs.Attributes, p *ondatra.Port, urpf bool) {
	t.Helper()
	d := gnmi.OC()
	i := attrs.NewOCInterface(p.Name(), dut)
	i.Description = ygot.String(attrs.Desc)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	if deviations.InterfaceEnabled(dut) {
		i.Enabled = ygot.Bool(true)
	}

	i.GetOrCreateEthernet()
	i4 := i.GetOrCreateSubinterface(0).GetOrCreateIpv4()
	i4.Enabled = ygot.Bool(true)
	a := i4.GetOrCreateAddress(attrs.IPv4)
	a.PrefixLength = ygot.Uint8(attrs.IPv4Len)

	i6 := i.GetOrCreateSubinterface(0).GetOrCreateIpv6()
	i6.Enabled = ygot.Bool(true)
	a6 := i6.GetOrCreateAddress(attrs.IPv6)
	a6.PrefixLength = ygot.Uint8(attrs.IPv6Len)
	if urpf {
		cfgplugins.ConfigureURPFonDutInt(t, dut, cfgplugins.URPFConfigParams{InterfaceName: p.Name(), IPv4Obj: i4, IPv6Obj: i6})
	}
	gnmi.BatchUpdate(intBatch, d.Interface(p.Name()).Config(), i)
}

// configureDUTLoopback sets up or retrieves the loopback interface on the DUT.
func configureDUTLoopback(t *testing.T, dut *ondatra.DUTDevice, intBatch *gnmi.SetBatch) {
	t.Helper()
	d := gnmi.OC()
	// Loopback0 for GUE Encap and Router ID
	loopbackIntfName := netutil.LoopbackInterface(t, dut, 0)
	lo0 := gnmi.OC().Interface(loopbackIntfName).Subinterface(0)
	ipv4Addrs := gnmi.LookupAll(t, dut, lo0.Ipv4().AddressAny().State())
	ipv6Addrs := gnmi.LookupAll(t, dut, lo0.Ipv6().AddressAny().State())
	if len(ipv4Addrs) == 0 && len(ipv6Addrs) == 0 {
		loop1 := dutLoopback.NewOCInterface(loopbackIntfName, dut)
		loop1.Type = oc.IETFInterfaces_InterfaceType_softwareLoopback
		gnmi.BatchUpdate(intBatch, d.Interface(loopbackIntfName).Config(), loop1)
	} else {
		v4, ok := ipv4Addrs[0].Val()
		if ok {
			dutLoopback.IPv4 = v4.GetIp()
		}
		v6, ok := ipv6Addrs[0].Val()
		if ok {
			dutLoopback.IPv6 = v6.GetIp()
		}
		t.Logf("Got DUT IPv4 loopback address: %v", dutLoopback.IPv4)
		t.Logf("Got DUT IPv6 loopback address: %v", dutLoopback.IPv6)
	}

}

// configureDUTPort configure DUT ports.
func configureDUTPort(t *testing.T, dut *ondatra.DUTDevice, intBatch *gnmi.SetBatch, attrs *attrs.Attributes, p *ondatra.Port, niName string) {
	t.Helper()
	d := gnmi.OC()
	cfgplugins.AssignToNetworkInstance(t, dut, p.Name(), niName, 0)
	i := attrs.NewOCInterface(p.Name(), dut)
	gnmi.BatchUpdate(intBatch, d.Interface(p.Name()).Config(), i)
}

// configureHardwareInit sets up the initial hardware configuration on the DUT.
// It pushes hardware initialization configs for:
//  1. VRF Selection Extended feature.
//  2. Policy Forwarding feature.
func configureHardwareInit(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	hardwareVRFCfg := cfgplugins.NewDUTHardwareInit(t, dut, cfgplugins.FeatureVrfSelectionExtended)
	hardwarePfCfg := cfgplugins.NewDUTHardwareInit(t, dut, cfgplugins.FeaturePolicyForwarding)
	if hardwareVRFCfg == "" || hardwarePfCfg == "" {
		return
	}
	cfgplugins.PushDUTHardwareInitConfig(t, dut, hardwareVRFCfg)
	cfgplugins.PushDUTHardwareInitConfig(t, dut, hardwarePfCfg)
}

// configureGUETunnel configures a GUE tunnel with optional ToS and TTL.
func configureGUEEncap(t *testing.T, dut *ondatra.DUTDevice, trafficType, nextHopGrpName, srcIP, GUEPolicyName string, dstIP []string, UDPDstPort uint16) {
	t.Helper()
	d := &oc.Root{}
	ni := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	v4NexthopUDPParams := cfgplugins.NexthopGroupUDPParams{
		IPFamily:           trafficType,
		NexthopGrpName:     nextHopGrpName,
		SrcIp:              srcIP,
		DstIp:              dstIP,
		DstUdpPort:         UDPDstPort,
		NetworkInstanceObj: ni,
	}
	// Create nexthop group for v4
	cfgplugins.NextHopGroupConfigForIpOverUdp(t, dut, v4NexthopUDPParams)

	gueV4EncapPolicyParams := cfgplugins.GueEncapPolicyParams{
		IPFamily:         trafficType,
		PolicyName:       GUEPolicyName,
		NexthopGroupName: nextHopGrpName,
		SrcIntfName:      srcIP,
		DstAddr:          dstIP,
		Rule:             1,
	}
	cfgplugins.NewPolicyForwardingGueEncap(t, dut, gueV4EncapPolicyParams)

	// Apply traffic policy on interface
	interfacePolicyParams := cfgplugins.OcPolicyForwardingParams{
		InterfaceID:        dut.Port(t, "port1").Name(),
		AppliedPolicyName:  GUEPolicyName,
		InterfaceName:      dut.Port(t, "port1").Name(),
		PolicyName:         GUEPolicyName,
		NetworkInstanceObj: ni,
	}
	cfgplugins.InterfacePolicyForwardingApply(t, dut, interfacePolicyParams)
}

// configureATE configures the ATE topology with two BGP peers.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	t.Helper()
	config := gosnappi.NewConfig()
	p1 := ate.Port(t, "port1")
	p2 := ate.Port(t, "port2")
	dev1 := atePort1.AddToOTG(config, p1, &dutPort1)
	dev2 := atePort2.AddToOTG(config, p2, &dutPort2)

	ip1V4 := dev1.Ethernets().Items()[0].Ipv4Addresses().Items()[0]
	ip1V6 := dev1.Ethernets().Items()[0].Ipv6Addresses().Items()[0]
	ip2V4 := dev2.Ethernets().Items()[0].Ipv4Addresses().Items()[0]
	ip2V6 := dev2.Ethernets().Items()[0].Ipv6Addresses().Items()[0]

	bgp1 := dev1.Bgp().SetRouterId(atePort1.IPv4)
	bgp1Peer := bgp1.Ipv4Interfaces().Add().SetIpv4Name(ip1V4.Name()).Peers().Add().SetName(fmt.Sprintf("%s.v4.EBGP.peer", dev1.Name()))
	bgp1Peer.SetPeerAddress(dutPort1.IPv4).SetAsNumber(uint32(ateAS1)).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
	// Valid source prefixes
	validNetV4 := bgp1Peer.V4Routes().Add().SetName("ValidSrc_V4")
	validNetV4.SetNextHopIpv4Address(atePort1.IPv4)
	validNetV4.Addresses().Add().SetAddress(ateAdvIPv4Prefix1).SetPrefix(uint32(prefix1Len)).SetCount(routeCount)
	bgp1PeerV6 := bgp1.Ipv6Interfaces().Add().SetIpv6Name(ip1V6.Name()).Peers().Add().SetName(fmt.Sprintf("%s.v6.EBGP.peer", dev1.Name()))
	bgp1PeerV6.SetPeerAddress(dutPort1.IPv6).SetAsNumber(uint32(ateAS1)).SetAsType(gosnappi.BgpV6PeerAsType.EBGP)
	validNetV6 := bgp1PeerV6.V6Routes().Add().SetName("ValidSrc_V6")
	validNetV6.SetNextHopIpv6Address(atePort1.IPv6)
	validNetV6.Addresses().Add().SetAddress(ateAdvIPv6Prefix1).SetPrefix(uint32(prefix1LenV6)).SetCount(routeCount)

	// ATE Port 2 (iBGP)
	bgp2 := dev2.Bgp().SetRouterId(atePort2.IPv4)
	bgp2Peer := bgp2.Ipv4Interfaces().Add().SetIpv4Name(ip2V4.Name()).Peers().Add().SetName(fmt.Sprintf("%s.v4.IBGP.peer", dev2.Name()))
	bgp2Peer.SetPeerAddress(dutPort2.IPv4).SetAsNumber(uint32(ateAS2)).SetAsType(gosnappi.BgpV4PeerAsType.IBGP)
	bgp2PeerV6 := bgp2.Ipv6Interfaces().Add().SetIpv6Name(ip2V6.Name()).Peers().Add().SetName(fmt.Sprintf("%s.v6.IBGP.peer", dev2.Name()))
	bgp2PeerV6.SetPeerAddress(dutPort2.IPv6).SetAsNumber(uint32(ateAS2)).SetAsType(gosnappi.BgpV6PeerAsType.IBGP)

	// Destination prefixes
	destNetV4 := bgp2Peer.V4Routes().Add().SetName("Dest_V4")
	destNetV4.SetNextHopIpv4Address(atePort2.IPv4)
	destNetV4.Addresses().Add().SetAddress(ateAdvIPv4Prefix3).SetPrefix(uint32(prefix3Len)).SetCount(routeCount)
	destNetV6 := bgp2PeerV6.V6Routes().Add().SetName("Dest_V6")
	destNetV6.SetNextHopIpv6Address(atePort2.IPv6)
	destNetV6.Addresses().Add().SetAddress(ateAdvIPv6Prefix3).SetPrefix(uint32(prefix3LenV6)).SetCount(routeCount)

	return config
}

// createFlow creates a traffic flow from ATE port 1 to port 2.
func createFlow(t *testing.T, dut *ondatra.DUTDevice, top gosnappi.Config, name, srcIP, dstIP string, isV4 bool) gosnappi.Flow {
	t.Helper()
	macAddress := gnmi.Get(t, dut, gnmi.OC().Interface(dut.Port(t, "port1").Name()).Ethernet().MacAddress().State())
	top.Flows().Clear()
	flow := top.Flows().Add().SetName(name)
	flow.Metrics().SetEnable(true)
	flow.TxRx().Port().SetTxName(top.Ports().Items()[0].Name()).SetRxNames([]string{top.Ports().Items()[1].Name()})
	flow.Size().SetFixed(flowSize)
	flow.Rate().SetPps(ratePPS)
	flow.Duration().FixedPackets().SetPackets(packetsToSend)

	eth := flow.Packet().Add().Ethernet()
	eth.Src().SetValue(atePort1.MAC)
	eth.Dst().SetValue(macAddress)

	if isV4 {
		v4 := flow.Packet().Add().Ipv4()
		v4.Src().SetValue(srcIP)
		v4.Dst().SetValue(dstIP)
	} else {
		v6 := flow.Packet().Add().Ipv6()
		v6.Src().SetValue(srcIP)
		v6.Dst().SetValue(dstIP)
	}
	return flow
}

// verifyTraffic checks traffic flow metrics for expected loss.
func verifyTraffic(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config, flowName string, expectLoss bool) {
	t.Helper()
	t.Logf("Starting traffic for flow %s", flowName)
	ate.OTG().StartTraffic(t)
	time.Sleep(trafficDuration)
	ate.OTG().StopTraffic(t)
	t.Logf("Stopped traffic for flow %s", flowName)

	otgutils.LogFlowMetrics(t, ate.OTG(), top)
	flowMetrics := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(flowName).State())
	txPackets := flowMetrics.GetCounters().GetOutPkts()
	rxPackets := flowMetrics.GetCounters().GetInPkts()

	if txPackets == 0 {
		t.Fatalf("Flow %s did not transmit any packets.", flowName)
	}
	lostPackets := txPackets - rxPackets

	if expectLoss {
		if lostPackets != txPackets {
			t.Errorf("expected 100%% packet loss for flow %s, but got %d lost packets out of %d", flowName, lostPackets, txPackets)
		} else {
			t.Logf("Successfully verified 100%% packet loss for flow %s", flowName)
		}
	} else {
		if got := (lostPackets * 100 / txPackets); got >= tolerance {
			t.Errorf("expected no packet loss for flow %s, but lost %d packets", flowName, lostPackets)
		} else {
			t.Logf("Successfully verified no packet loss for flow %s", flowName)
		}
	}
}

// TestURPFNonDefaultNI is the main test function.
func TestURPFNonDefaultNI(t *testing.T) {
	t.Helper()
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	t.Log("Configure DUT with baseline BGP, VRF, and uRPF settings")
	batch := configureDUT(t, dut)

	t.Log("Configure ATE with eBGP and iBGP peers")
	otgConfig := configureATE(t, ate)
	ate.OTG().PushConfig(t, otgConfig)
	ate.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), otgConfig, "IPv4")
	otgutils.WaitForARP(t, ate.OTG(), otgConfig, "IPv6")

	cfgplugins.VerifyDUTVrfBGPState(t, dut, cfgplugins.VrfBGPState{NetworkInstanceName: defaultNIName, NeighborIPs: []string{atePort2.IPv4, atePort2.IPv6}})
	cfgplugins.VerifyDUTVrfBGPState(t, dut, cfgplugins.VrfBGPState{NetworkInstanceName: nonDefaultVRF, NeighborIPs: []string{atePort1.IPv4, atePort1.IPv6}})
	bgpRouteVerification(t, dut)

	testCases := []struct {
		desc           string
		gueEnabled     bool
		expectLoss     bool
		isV4           bool
		srcIP          string
		dstIP          string
		flowName       string
		verifyCounters bool
		IPStr          string
		staticRoutePfx string
		pfxAddr        string
		nextHopAddr    map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union
	}{
		{
			desc:           "URPF-1.1.1: uRPF with valid IPv4 source",
			gueEnabled:     false,
			expectLoss:     false,
			isV4:           true,
			srcIP:          ateAdvIPv4Prefix1,
			dstIP:          ateAdvIPv4Prefix3,
			flowName:       "v4_valid_src",
			IPStr:          "ip",
			staticRoutePfx: staticRoutePfxV4,
			pfxAddr:        atePort2.IPv4,
			nextHopAddr:    map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{"v4": oc.UnionString(atePort2.IPv4)},
		},
		{
			desc:           "URPF-1.1.1: uRPF with valid IPv6 source",
			gueEnabled:     false,
			expectLoss:     false,
			isV4:           false,
			srcIP:          ateAdvIPv6Prefix1,
			dstIP:          ateAdvIPv6Prefix3,
			flowName:       "v6_valid_src",
			IPStr:          "ipv6",
			staticRoutePfx: staticRoutePfxV6,
			pfxAddr:        atePort2.IPv6,
			nextHopAddr:    map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{"v6": oc.UnionString(atePort2.IPv6)},
		},
		{
			desc:           "URPF-1.1.2: uRPF with invalid IPv4 source",
			gueEnabled:     false,
			expectLoss:     true,
			isV4:           true,
			srcIP:          ateAdvIPv4Prefix2,
			dstIP:          ateAdvIPv4Prefix3,
			flowName:       "v4_invalid_src",
			verifyCounters: true,
		},
		{
			// TODO: Test validation is currently failing and will be fixed once the defect (https://partnerissuetracker.corp.google.com/u/1/issues/457962035) is resolved.
			desc:           "URPF-1.1.2: uRPF with invalid IPv6 source",
			gueEnabled:     false,
			expectLoss:     true,
			isV4:           false,
			srcIP:          ateAdvIPv6Prefix2,
			dstIP:          ateAdvIPv6Prefix3,
			flowName:       "v6_invalid_src",
			verifyCounters: true,
		},
		{
			desc:       "URPF-1.1.3: uRPF with valid IPv4 source and GUE",
			gueEnabled: true,
			expectLoss: false,
			isV4:       true,
			srcIP:      ateAdvIPv4Prefix1,
			dstIP:      ateAdvIPv4Prefix3,
			flowName:   "v4_valid_src_gue",
		},
		{
			desc:       "URPF-1.1.3: uRPF with valid IPv6 source and GUE",
			gueEnabled: true,
			expectLoss: false,
			isV4:       false,
			srcIP:      ateAdvIPv6Prefix1,
			dstIP:      ateAdvIPv6Prefix3,
			flowName:   "v6_valid_src_gue",
		},
		{
			desc:           "URPF-1.1.4: uRPF with invalid IPv4 source and GUE",
			gueEnabled:     true,
			expectLoss:     true,
			isV4:           true,
			srcIP:          ateAdvIPv4Prefix2,
			dstIP:          ateAdvIPv4Prefix3,
			flowName:       "v4_invalid_src_gue",
			verifyCounters: true,
		},
		{
			// TODO: Test validation is currently failing and will be fixed once the defect (https://partnerissuetracker.corp.google.com/u/1/issues/457962035) is resolved.
			desc:           "URPF-1.1.4: uRPF with invalid IPv6 source and GUE",
			gueEnabled:     true,
			expectLoss:     true,
			isV4:           false,
			srcIP:          ateAdvIPv6Prefix2,
			dstIP:          ateAdvIPv6Prefix3,
			flowName:       "v6_invalid_src_gue",
			verifyCounters: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			if tc.gueEnabled {
				t.Log("Configuring GUE on DUT")
				if tc.isV4 {
					configureGUEEncap(t, dut, "V4Udp", nexthopGroupNameV4, dutLoopback.IPv4, GUEPolicyV4Name, dstAddr, udpDestPort)
				} else {
					configureGUEEncap(t, dut, "V6Udp", nexthopGroupNameV6, dutLoopback.IPv4, GUEPolicyV6Name, dstAddr, udpDestPort)
				}
			}
			if !tc.expectLoss {
				if tc.gueEnabled {
					cfgParams := &cfgplugins.StaticRouteCfg{
						NetworkInstance: deviations.DefaultNetworkInstance(dut),
						Prefix:          GUEDstIPv4 + "/32",
						NextHops:        map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{"0": oc.UnionString(atePort2.IPv4), "1": oc.UnionString(atePort2.IPv6)},
					}
					if _, err := cfgplugins.NewStaticRouteCfg(batch, cfgParams, dut); err != nil {
						t.Fatalf("Failed to configure static route: %v", err)
					}
					batch.Set(t, dut)
				} else {
					cfgplugins.StaticRouteNextNetworkInstance(t, dut, &cfgplugins.StaticRouteCfg{IPType: tc.IPStr, NetworkInstance: nonDefaultVRF, Prefix: tc.pfxAddr, NextHopAddr: tc.dstIP + tc.staticRoutePfx})
				}
			}
			var initialDropCount uint64
			p1 := dut.Port(t, "port1")
			if tc.verifyCounters {
				// TODO: No support for UrpfDropPkts yet; validating drops using InUnicastPkts for now. Will update once support is added.
				initialDropCount := gnmi.Get(t, dut, gnmi.OC().Interface(p1.Name()).Counters().InUnicastPkts().State())
				t.Logf("Initial uRPF drop count: %d", initialDropCount)
			}
			flow := createFlow(t, dut, otgConfig, tc.flowName, tc.srcIP, tc.dstIP, tc.isV4)
			ate.OTG().PushConfig(t, otgConfig)
			ate.OTG().StartProtocols(t)
			verifyTraffic(t, ate, otgConfig, flow.Name(), tc.expectLoss)
			if tc.verifyCounters {
				verifyURPFCounters(t, dut, initialDropCount)
			}
		})
	}
}

// verifyURPFCounters checks if the uRPF drop counter has incremented as expected.
func verifyURPFCounters(t *testing.T, dut *ondatra.DUTDevice, initialDropCount uint64) {
	t.Helper()
	p1 := dut.Port(t, "port1")
	// TODO: No support for UrpfDropPkts yet; validating drops using InUnicastPkts for now. Will update once support is added.
	newDropCount := gnmi.Get(t, dut, gnmi.OC().Interface(p1.Name()).Counters().InUnicastPkts().State())
	dropCount := newDropCount - initialDropCount
	if dropCount < initialDropCount+(packetsToSend*0.9) { // Allow some tolerance
		t.Errorf("uRPF drop counter did not increment sufficiently. Got: %d, want approx: %d", dropCount, packetsToSend)
	} else {
		t.Logf("uRPF drop counter incremented as expected by %d packets.", dropCount)
	}
}

// bgpRouteVerification build routes parameters and verify routes if advertised routes are installed in DUT AFT.
func bgpRouteVerification(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	// Build routes to advertise
	routesToAdvertise := map[string]cfgplugins.RouteInfo{
		fmt.Sprintf("%s/%d", ateAdvIPv4Prefix1, prefix1Len):   {VRF: nonDefaultVRF, IPType: cfgplugins.IPv4, DefaultName: defaultNIName},
		fmt.Sprintf("%s/%d", ateAdvIPv4Prefix3, prefix3Len):   {VRF: defaultNIName, IPType: cfgplugins.IPv4, DefaultName: defaultNIName},
		fmt.Sprintf("%s/%d", ateAdvIPv6Prefix1, prefix1LenV6): {VRF: nonDefaultVRF, IPType: cfgplugins.IPv6, DefaultName: defaultNIName},
		fmt.Sprintf("%s/%d", ateAdvIPv6Prefix3, prefix3LenV6): {VRF: defaultNIName, IPType: cfgplugins.IPv6, DefaultName: defaultNIName},
	}

	cfgplugins.VerifyRoutes(t, dut, routesToAdvertise)
}
