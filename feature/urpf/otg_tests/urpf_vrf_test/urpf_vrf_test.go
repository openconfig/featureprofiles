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
	"github.com/openconfig/featureprofiles/internal/helpers"
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
	loopbackIntfName   = "loopback0"
	udpDestPort        = 6080
	trafficDuration    = 45 * time.Second
	ratePPS            = 100
	flowSize           = 512
	packetsToSend      = 3000
	nexthopGroupNameV4 = "GUE-NHG"
	nexthopGroupNameV6 = "GUE-NHGv6"
	GUEPolicyV4Name    = "GUE-Policy-V4"
	GUEPolicyV6Name    = "GUE-Policy-V6"
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
	prefixIPv4Len     = 24
	prefixIPv6Len     = 64
	// Invalid source prefixes advertised from ATE Port 1 (but rejected by DUT policy)
	ateAdvIPv4Prefix2 = "198.18.2.0"
	ateAdvIPv6Prefix2 = "3001:db8:10::"
	// Destination prefixes advertised from ATE Port 2
	ateAdvIPv4Prefix3 = "198.18.3.0"
	ateAdvIPv6Prefix3 = "4001:db8:10::"
	defaultNIName     = strings.ToLower("DEFAULT")
	// Connected subnets of DUT port1, used for the uRPF lookup in the non-default VRF.
	dutPort1ConnectedV4 = "192.0.2.0/30"
	dutPort1ConnectedV6 = "2001:db8:1::/126"
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
	cfgplugins.ConfigureBGPNeighbor(t, dut, defaultNI, dutPort1.IPv4, atePort1.IPv4, dutAS, ateAS1, "IPv4", true)
	cfgplugins.ConfigureBGPNeighbor(t, dut, defaultNI, dutPort1.IPv4, atePort1.IPv6, dutAS, ateAS1, "IPv6", true)
	cfgplugins.ConfigureBGPNeighbor(t, dut, defaultNI, dutPort2.IPv4, atePort2.IPv4, dutAS, ateAS2, "IPv4", true)
	cfgplugins.ConfigureBGPNeighbor(t, dut, defaultNI, dutPort2.IPv4, atePort2.IPv6, dutAS, ateAS2, "IPv6", true)
	cfgplugins.UpdateNetworkInstanceOnDut(t, dut, defaultNIName, defaultNI)
	cfgplugins.UpdateNetworkInstanceOnDut(t, dut, nonDefaultVRF, nonDefaultNI)
	t.Log("Configuring uRPF lookup routes in the non-default VRF")
	configureURPFLookupRoutes(t, dut, intBatch)
	intBatch.Set(t, dut)
	return intBatch
}

// configureURPFLookupRoutes installs the routes that the uRPF lookup is performed against in the
// non-default VRF. Only the valid source prefixes and the DUT port1 connected subnet are installed,
// no default route is configured or leaked into the non-default VRF.
//
// The non-default VRF has no interfaces of its own (both DUT ports live in the default VRF), so the
// next-hops cannot be resolved inside the non-default VRF. Each route therefore has to point at the
// default network-instance as the egress network-instance, otherwise the route stays inactive and
// never lands in the non-default VRF FIB, which silently disables the uRPF lookup.
func configureURPFLookupRoutes(t *testing.T, dut *ondatra.DUTDevice, batch *gnmi.SetBatch) {
	t.Helper()
	egressNI := deviations.DefaultNetworkInstance(dut)
	routes := []*cfgplugins.StaticRouteCfg{
		{
			IPType:              "ip",
			Prefix:              fmt.Sprintf("%s/%d", ateAdvIPv4Prefix1, prefixIPv4Len),
			NextHopAddr:         atePort1.IPv4,
			NextNetworkInstance: egressNI,
		},
		{
			IPType:              "ipv6",
			Prefix:              fmt.Sprintf("%s/%d", ateAdvIPv6Prefix1, prefixIPv6Len),
			NextHopAddr:         atePort1.IPv6,
			NextNetworkInstance: egressNI,
		},
		{
			IPType:              "ip",
			Prefix:              dutPort1ConnectedV4,
			NextHopAddr:         atePort1.IPv4,
			NextNetworkInstance: egressNI,
		},
		{
			IPType:              "ipv6",
			Prefix:              dutPort1ConnectedV6,
			NextHopAddr:         atePort1.IPv6,
			NextNetworkInstance: egressNI,
		},
	}
	for _, r := range routes {
		r.T = t
		r.NetworkInstance = nonDefaultVRF
		if deviations.StaticRouteNextNetworkInstanceOCUnsupported(dut) {
			switch dut.Vendor() {
			case ondatra.ARISTA:
				helpers.GnmiCLIConfig(t, dut, fmt.Sprintf("%s route vrf %s %s egress-vrf %s %s",
					r.IPType, r.NetworkInstance, r.Prefix, egressNI, r.NextHopAddr))
			default:
				t.Fatalf("deviation StaticRouteNextNetworkInstanceOCUnsupported is not handled for vendor %s", dut.Vendor())
			}
			continue
		}
		r.NextHops = map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{"0": oc.UnionString(r.NextHopAddr)}
		if _, err := cfgplugins.NewStaticRouteCfg(batch, r, dut); err != nil {
			t.Fatalf("Failed to configure static route %s in %s: %v", r.Prefix, r.NetworkInstance, err)
		}
	}
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
	if !deviations.IPv4MissingEnabled(dut) {
		i4.Enabled = ygot.Bool(true)
	}
	a := i4.GetOrCreateAddress(attrs.IPv4)
	a.PrefixLength = ygot.Uint8(attrs.IPv4Len)
	i6 := i.GetOrCreateSubinterface(0).GetOrCreateIpv6()
	i6.Enabled = ygot.Bool(true)
	a6 := i6.GetOrCreateAddress(attrs.IPv6)
	a6.PrefixLength = ygot.Uint8(attrs.IPv6Len)
	if urpf {
		cfgplugins.ConfigureURPFonDutInt(t, dut, cfgplugins.URPFConfigParams{
			InterfaceName:         p.Name(),
			IPv4Obj:               i4,
			IPv6Obj:               i6,
			Mode:                  oc.IfIp_UrpfMode_LOOSE,
			LookupNetworkInstance: nonDefaultVRF,
		})
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

// configureHardwareInit sets up the initial hardware configuration on the DUT.
func configureHardwareInit(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	features := []cfgplugins.FeatureType{
		cfgplugins.FeatureEgressURPF,
	}
	for _, feature := range features {
		hardwareInitCfg := cfgplugins.NewDUTHardwareInit(t, dut, feature)
		if hardwareInitCfg != "" {
			cfgplugins.PushDUTHardwareInitConfig(t, dut, hardwareInitCfg)
		}
	}
}

// configureGUEEncap configures a GUE tunnel with optional ToS and TTL.
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
func configureATE(t *testing.T, ate *ondatra.ATEDevice) (gosnappi.Config, []string) {
	t.Helper()
	var interfaceNamesList []string
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
	validNetV4.Addresses().Add().SetAddress(ateAdvIPv4Prefix1).SetPrefix(uint32(prefixIPv4Len)).SetCount(routeCount)
	bgp1PeerV6 := bgp1.Ipv6Interfaces().Add().SetIpv6Name(ip1V6.Name()).Peers().Add().SetName(fmt.Sprintf("%s.v6.EBGP.peer", dev1.Name()))
	bgp1PeerV6.SetPeerAddress(dutPort1.IPv6).SetAsNumber(uint32(ateAS1)).SetAsType(gosnappi.BgpV6PeerAsType.EBGP)
	validNetV6 := bgp1PeerV6.V6Routes().Add().SetName("ValidSrc_V6")
	validNetV6.SetNextHopIpv6Address(atePort1.IPv6)
	validNetV6.Addresses().Add().SetAddress(ateAdvIPv6Prefix1).SetPrefix(uint32(prefixIPv6Len)).SetCount(routeCount)
	// ATE Port 2 (iBGP)
	bgp2 := dev2.Bgp().SetRouterId(atePort2.IPv4)
	bgp2Peer := bgp2.Ipv4Interfaces().Add().SetIpv4Name(ip2V4.Name()).Peers().Add().SetName(fmt.Sprintf("%s.v4.IBGP.peer", dev2.Name()))
	bgp2Peer.SetPeerAddress(dutPort2.IPv4).SetAsNumber(uint32(ateAS2)).SetAsType(gosnappi.BgpV4PeerAsType.IBGP)
	bgp2PeerV6 := bgp2.Ipv6Interfaces().Add().SetIpv6Name(ip2V6.Name()).Peers().Add().SetName(fmt.Sprintf("%s.v6.IBGP.peer", dev2.Name()))
	bgp2PeerV6.SetPeerAddress(dutPort2.IPv6).SetAsNumber(uint32(ateAS2)).SetAsType(gosnappi.BgpV6PeerAsType.IBGP)
	// Destination prefixes
	destNetV4 := bgp2Peer.V4Routes().Add().SetName("Dest_V4")
	destNetV4.SetNextHopIpv4Address(atePort2.IPv4)
	destNetV4.Addresses().Add().SetAddress(ateAdvIPv4Prefix3).SetPrefix(uint32(prefixIPv4Len)).SetCount(routeCount)
	destNetV6 := bgp2PeerV6.V6Routes().Add().SetName("Dest_V6")
	destNetV6.SetNextHopIpv6Address(atePort2.IPv6)
	destNetV6.Addresses().Add().SetAddress(ateAdvIPv6Prefix3).SetPrefix(uint32(prefixIPv6Len)).SetCount(routeCount)
	// Collect interface/device names
	for _, dev := range config.Devices().Items() {
		interfaceNamesList = append(interfaceNamesList, dev.Name())
	}
	return config, interfaceNamesList
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
func verifyTraffic(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config, flowName string, expectLoss bool) uint64 {
	t.Helper()
	t.Logf("Starting traffic for flow %s", flowName)
	ate.OTG().StartTraffic(t)
	deadline := time.Now().Add(trafficDuration)
	expectedTxPkts := uint64(packetsToSend)
	for {
		flowMetrics := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(flowName).State())
		txPackets := flowMetrics.GetCounters().GetOutPkts()
		if txPackets >= expectedTxPkts {
			break
		}
		if time.Now().After(deadline) {
			t.Logf("Timed out waiting for fixed-packet flow completion for %s: got tx=%d want tx>=%d", flowName, txPackets, expectedTxPkts)
			break
		}
		time.Sleep(1 * time.Second)
	}
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
	return txPackets
}

// TestURPFNonDefaultNI is the main test function.
func TestURPFNonDefaultNI(t *testing.T) {
	t.Helper()
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	t.Log("Configure DUT with baseline BGP, VRF, and uRPF settings")
	configureDUT(t, dut)
	t.Log("Configure ATE with eBGP and iBGP peers")
	otgConfig, interfaceNamesList := configureATE(t, ate)
	ate.OTG().PushConfig(t, otgConfig)
	ate.OTG().StartProtocols(t)
	cfgplugins.IsIPv4InterfaceARPresolved(t, ate, cfgplugins.AddressFamilyParams{InterfaceNames: interfaceNamesList})
	cfgplugins.IsIPv6InterfaceARPresolved(t, ate, cfgplugins.AddressFamilyParams{InterfaceNames: interfaceNamesList})
	cfgplugins.VerifyDUTVrfBGPState(t, dut, cfgplugins.VrfBGPState{NetworkInstanceName: defaultNIName, NeighborIPs: []string{atePort2.IPv4, atePort2.IPv6}})
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
	}{
		{
			desc:       "URPF-1.1.1: uRPF with valid IPv4 source",
			gueEnabled: false,
			expectLoss: false,
			isV4:       true,
			srcIP:      ateAdvIPv4Prefix1,
			dstIP:      ateAdvIPv4Prefix3,
			flowName:   "v4_valid_src",
		},
		{
			desc:       "URPF-1.1.1: uRPF with valid IPv6 source",
			gueEnabled: false,
			expectLoss: false,
			isV4:       false,
			srcIP:      ateAdvIPv6Prefix1,
			dstIP:      ateAdvIPv6Prefix3,
			flowName:   "v6_valid_src",
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
				dstAddr := []string{atePort2.IPv4}
				if tc.isV4 {
					configureGUEEncap(t, dut, "V4Udp", nexthopGroupNameV4, dutLoopback.IPv4, GUEPolicyV4Name, dstAddr, udpDestPort)
				} else {
					configureGUEEncap(t, dut, "V6Udp", nexthopGroupNameV6, dutLoopback.IPv4, GUEPolicyV6Name, dstAddr, udpDestPort)
				}
			}
			var initialDropCount uint64
			var exactURPFCounter bool
			p1 := dut.Port(t, "port1")
			if tc.verifyCounters {
				initialDropCount, exactURPFCounter = urpfDropPkts(t, dut, p1.Name(), tc.isV4)
				t.Logf("Initial uRPF drop count: %d", initialDropCount)
			}
			flow := createFlow(t, dut, otgConfig, tc.flowName, tc.srcIP, tc.dstIP, tc.isV4)
			ate.OTG().PushConfig(t, otgConfig)
			ate.OTG().StartProtocols(t)
			txPackets := verifyTraffic(t, ate, otgConfig, flow.Name(), tc.expectLoss)
			if tc.verifyCounters {
				verifyURPFCounters(t, dut, p1.Name(), tc.isV4, initialDropCount, txPackets, exactURPFCounter)
			}
		})
	}
}

// verifyURPFCounters checks if the uRPF drop counter has incremented as expected.
func verifyURPFCounters(t *testing.T, dut *ondatra.DUTDevice, portName string, isV4 bool, initialDropCount, expectedIncrement uint64, exactURPFCounter bool) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		newDropCount, _ := urpfDropPkts(t, dut, portName, isV4)
		dropCount := newDropCount - initialDropCount
		if exactURPFCounter {
			if dropCount == expectedIncrement {
				t.Logf("uRPF drop counter incremented by %d packets as expected.", dropCount)
				return
			}
		} else {
			if dropCount >= expectedIncrement {
				t.Logf("uRPF fallback counter incremented by %d packets (>= expected %d).", dropCount, expectedIncrement)
				return
			}
		}
		if time.Now().After(deadline) {
			if exactURPFCounter {
				t.Errorf("uRPF drop counter increment mismatch. Got increment: %d, want: %d", dropCount, expectedIncrement)
			} else {
				t.Errorf("uRPF fallback counter increment too low. Got increment: %d, want at least: %d", dropCount, expectedIncrement)
			}
			return
		}
		time.Sleep(1 * time.Second)
	}
}

// urpfDropPkts reads the IPv4 or IPv6 uRPF drop packet counter from subinterface 0.
// The second return value indicates whether this is the dedicated uRPF drop counter (true)
// or a fallback proxy counter (false).
func urpfDropPkts(t *testing.T, dut *ondatra.DUTDevice, portName string, isV4 bool) (uint64, bool) {
	t.Helper()
	// TODO: No support for UrpfDropPkts yet; validating drops using InUnicastPkts for now. Will uncomment the below lines once support is added.
	// if isV4 {
	// 	if v, ok := gnmi.Lookup(t, dut, gnmi.OC().Interface(portName).Subinterface(0).Ipv4().Counters().UrpfDropPkts().State()).Val(); ok {
	// 		return v, true
	// 	}
	// } else {
	// 	if v, ok := gnmi.Lookup(t, dut, gnmi.OC().Interface(portName).Subinterface(0).Ipv6().Counters().UrpfDropPkts().State()).Val(); ok {
	// 		return v, true
	// 	}
	// }
	return gnmi.Get(t, dut, gnmi.OC().Interface(portName).Counters().InUnicastPkts().State()), false
}

// bgpRouteVerification build routes parameters and verify routes if advertised routes are installed in DUT AFT.
func bgpRouteVerification(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	// Build routes to advertise
	routesToAdvertise := map[string]cfgplugins.RouteInfo{
		fmt.Sprintf("%s/%d", ateAdvIPv4Prefix1, prefixIPv4Len): {VRF: nonDefaultVRF, IPType: cfgplugins.IPv4, DefaultName: defaultNIName},
		fmt.Sprintf("%s/%d", ateAdvIPv4Prefix3, prefixIPv4Len): {VRF: defaultNIName, IPType: cfgplugins.IPv4, DefaultName: defaultNIName},
		fmt.Sprintf("%s/%d", ateAdvIPv6Prefix1, prefixIPv6Len): {VRF: nonDefaultVRF, IPType: cfgplugins.IPv6, DefaultName: defaultNIName},
		fmt.Sprintf("%s/%d", ateAdvIPv6Prefix3, prefixIPv6Len): {VRF: defaultNIName, IPType: cfgplugins.IPv6, DefaultName: defaultNIName},
	}
	cfgplugins.VerifyRoutes(t, dut, routesToAdvertise)
}
