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
	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	plenIPv4                     = 30
	plenIPv6                     = 126
	dutAS                        = 100
	ateAS1                       = 200 // eBGP peer
	ateAS2                       = 100 // iBGP peer
	routeCount                   = 1
	nonDefaultVRF                = "VRF-1"
	loopbackIntfName             = "loopback0"
	udpDestPort                  = 6080
	udpSrcPort                   = 50000
	trafficDuration              = 45 * time.Second
	ratePPS                      = 100
	flowSize                     = 512
	packetsToSend                = 3000
	nexthopGroupNameV4           = "GUE-NHG"
	GUEPolicyV4Name              = "GUE-Policy-V4"
	isDefaultVRF                 = true
	portCounterControlPlaneSlack = 10
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
	// Host addresses used in traffic flows (must belong to advertised prefixes).
	ateFlowIPv4ValidSrc   = "198.18.1.1"
	ateFlowIPv6ValidSrc   = "2001:db8:10::1"
	ateFlowIPv4InvalidSrc = "198.18.2.1"
	ateFlowIPv6InvalidSrc = "3001:db8:10::1"
	ateFlowIPv4Dst        = "198.18.3.1"
	ateFlowIPv6Dst        = "4001:db8:10::1"
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
	defaultNIName := deviations.DefaultNetworkInstance(dut)
	intBatch := new(gnmi.SetBatch)
	t.Logf("Configuring Interfaces")
	configureDUTInterface(t, dut, intBatch, &dutPort1, p1, true)
	configureDUTInterface(t, dut, intBatch, &dutPort2, p2, false)
	configureDUTLoopback(t, dut, intBatch)
	t.Log("Configuring Hardware Init")
	configureHardwareInit(t, dut)
	cfgplugins.EnableDefaultNetworkInstanceBgp(t, dut, dutAS)
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		cfgplugins.AssignToNetworkInstance(t, dut, p1.Name(), defaultNIName, 0)
		cfgplugins.AssignToNetworkInstance(t, dut, p2.Name(), defaultNIName, 0)
	}
	t.Log("Configuring Network Instances")
	defaultNI := cfgplugins.ConfigureNetworkInstance(t, dut, defaultNIName, isDefaultVRF)
	nonDefaultNI := cfgplugins.ConfigureNetworkInstance(t, dut, nonDefaultVRF, !isDefaultVRF)
	cfgplugins.ConfigureBGPNeighbor(t, dut, defaultNI, dutPort1.IPv4, atePort1.IPv4, dutAS, ateAS1, "IPv4", true)
	cfgplugins.ConfigureBGPNeighbor(t, dut, defaultNI, dutPort1.IPv4, atePort1.IPv6, dutAS, ateAS1, "IPv6", true)
	cfgplugins.ConfigureBGPNeighbor(t, dut, defaultNI, dutPort2.IPv4, atePort2.IPv4, dutAS, ateAS2, "IPv4", true)
	cfgplugins.ConfigureBGPNeighbor(t, dut, defaultNI, dutPort2.IPv4, atePort2.IPv6, dutAS, ateAS2, "IPv6", true)
	cfgplugins.UpdateNetworkInstanceOnDut(t, dut, defaultNIName, defaultNI)
	cfgplugins.UpdateNetworkInstanceOnDut(t, dut, nonDefaultVRF, nonDefaultNI)
	intBatch.Set(t, dut)
	t.Log("Configuring uRPF lookup routes in the non-default VRF")
	routeBatch := new(gnmi.SetBatch)
	configureURPFLookupRoutes(t, dut, routeBatch)
	routeBatch.Set(t, dut)
	return routeBatch
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

// configureGUEEncap configures GUE using the RT-3.53 model:
// destination prefixes are resolved through a static route to a UDPv4 next-hop-group.
func configureGUEEncap(t *testing.T, dut *ondatra.DUTDevice, srcIP string, dstIP []string, UDPDstPort uint16) {
	t.Helper()
	d := &oc.Root{}
	ni := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	v4NexthopUDPParams := cfgplugins.NexthopGroupUDPParams{
		IPFamily:           "V4Udp",
		NexthopGrpName:     nexthopGroupNameV4,
		Index:              "0",
		SrcIp:              srcIP,
		DstIp:              dstIP,
		SrcUdpPort:         udpSrcPort,
		DstUdpPort:         UDPDstPort,
		NetworkInstanceObj: ni,
	}
	// Create UDPv4 next-hop-group
	cfgplugins.NextHopGroupConfigForIpOverUdp(t, dut, v4NexthopUDPParams)

	if !deviations.NextHopGroupOCUnsupported(dut) {
		cfgplugins.UpdateNetworkInstanceOnDut(t, dut, deviations.DefaultNetworkInstance(dut), ni)
	}

	b := &gnmi.SetBatch{}
	for _, pfx := range []struct {
		prefix      string
		prefixLen   int
		policyRule  string
		trafficType oc.E_Aft_EncapsulationHeaderType
	}{
		{prefix: ateAdvIPv4Prefix3, prefixLen: prefixIPv4Len, policyRule: "rule1", trafficType: oc.Aft_EncapsulationHeaderType_UDPV4},
		{prefix: ateAdvIPv6Prefix3, prefixLen: prefixIPv6Len, policyRule: "rule1", trafficType: oc.Aft_EncapsulationHeaderType_UDPV4},
	} {
		sr := &cfgplugins.StaticRouteCfg{
			NetworkInstance:  deviations.DefaultNetworkInstance(dut),
			Prefix:           fmt.Sprintf("%s/%d", pfx.prefix, pfx.prefixLen),
			NexthopGroup:     true,
			NexthopGroupName: nexthopGroupNameV4,
			T:                t,
			TrafficType:      pfx.trafficType,
			PolicyName:       GUEPolicyV4Name,
			Rule:             pfx.policyRule,
		}
		if _, err := cfgplugins.NewStaticRouteCfg(b, sr, dut); err != nil {
			t.Fatalf("Failed to configure GUE static route %s/%d: %v", pfx.prefix, pfx.prefixLen, err)
		}
	}
	b.Set(t, dut)
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
	invalidNetV4 := bgp1Peer.V4Routes().Add().SetName("InvalidSrc_V4")
	invalidNetV4.SetNextHopIpv4Address(atePort1.IPv4)
	invalidNetV4.Addresses().Add().SetAddress(ateAdvIPv4Prefix2).SetPrefix(uint32(prefixIPv4Len)).SetCount(routeCount)
	bgp1PeerV6 := bgp1.Ipv6Interfaces().Add().SetIpv6Name(ip1V6.Name()).Peers().Add().SetName(fmt.Sprintf("%s.v6.EBGP.peer", dev1.Name()))
	bgp1PeerV6.SetPeerAddress(dutPort1.IPv6).SetAsNumber(uint32(ateAS1)).SetAsType(gosnappi.BgpV6PeerAsType.EBGP)
	validNetV6 := bgp1PeerV6.V6Routes().Add().SetName("ValidSrc_V6")
	validNetV6.SetNextHopIpv6Address(atePort1.IPv6)
	validNetV6.Addresses().Add().SetAddress(ateAdvIPv6Prefix1).SetPrefix(uint32(prefixIPv6Len)).SetCount(routeCount)
	invalidNetV6 := bgp1PeerV6.V6Routes().Add().SetName("InvalidSrc_V6")
	invalidNetV6.SetNextHopIpv6Address(atePort1.IPv6)
	invalidNetV6.Addresses().Add().SetAddress(ateAdvIPv6Prefix2).SetPrefix(uint32(prefixIPv6Len)).SetCount(routeCount)
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

// verifyTraffic checks traffic flow metrics and validates Port2 ingress counters.
func verifyTraffic(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config, flowName string, expectLoss bool, rxPortName string) uint64 {
	t.Helper()
	port2InFramesBefore := gnmi.Get(t, ate.OTG(), gnmi.OTG().Port(rxPortName).Counters().InFrames().State())
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
	port2InFramesAfter := gnmi.Get(t, ate.OTG(), gnmi.OTG().Port(rxPortName).Counters().InFrames().State())
	port2Delta := port2InFramesAfter - port2InFramesBefore
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
		if rxPackets != 0 {
			t.Errorf("expected zero received packets on flow %s, got %d", flowName, rxPackets)
		}
		if port2Delta > portCounterControlPlaneSlack {
			t.Errorf("expected near-zero ingress packets on Port2 for loss case %s, got delta=%d (> slack %d)", flowName, port2Delta, portCounterControlPlaneSlack)
		}
	} else {
		if lostPackets != 0 || txPackets != rxPackets {
			t.Errorf("expected zero packet loss for flow %s, got tx=%d rx=%d lost=%d", flowName, txPackets, rxPackets, lostPackets)
		} else {
			t.Logf("Successfully verified no packet loss for flow %s", flowName)
		}
		// README requires Port2 observed packets to match received flow packets.
		// Use bounded tolerance to avoid false negatives from sampling/timing skew.
		var deltaDiff uint64
		if port2Delta >= rxPackets {
			deltaDiff = port2Delta - rxPackets
		} else {
			deltaDiff = rxPackets - port2Delta
		}
		if deltaDiff > portCounterControlPlaneSlack {
			t.Errorf("Port2 ingress counter mismatch for flow %s: delta=%d, flow-rx=%d, diff=%d (> slack %d)", flowName, port2Delta, rxPackets, deltaDiff, portCounterControlPlaneSlack)
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
	rxOTGPortName := otgConfig.Ports().Items()[1].Name()
	ate.OTG().PushConfig(t, otgConfig)
	ate.OTG().StartProtocols(t)
	cfgplugins.IsIPv4InterfaceARPresolved(t, ate, cfgplugins.AddressFamilyParams{InterfaceNames: interfaceNamesList})
	cfgplugins.IsIPv6InterfaceARPresolved(t, ate, cfgplugins.AddressFamilyParams{InterfaceNames: interfaceNamesList})
	if !deviations.URPFConfigOCUnsupported(dut) {
		verifyURPFTelemetryState(t, dut, dut.Port(t, "port1").Name())
	}
	defaultNIName := deviations.DefaultNetworkInstance(dut)
	cfgplugins.VerifyDUTVrfBGPState(t, dut, cfgplugins.VrfBGPState{NetworkInstanceName: defaultNIName, NeighborIPs: []string{atePort1.IPv4, atePort1.IPv6, atePort2.IPv4, atePort2.IPv6}})
	bgpRouteVerification(t, dut)
	verifyInvalidPrefixPlacement(t, dut)
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
			srcIP:      ateFlowIPv4ValidSrc,
			dstIP:      ateFlowIPv4Dst,
			flowName:   "v4_valid_src",
		},
		{
			desc:       "URPF-1.1.1: uRPF with valid IPv6 source",
			gueEnabled: false,
			expectLoss: false,
			isV4:       false,
			srcIP:      ateFlowIPv6ValidSrc,
			dstIP:      ateFlowIPv6Dst,
			flowName:   "v6_valid_src",
		},
		{
			desc:           "URPF-1.1.2: uRPF with invalid IPv4 source",
			gueEnabled:     false,
			expectLoss:     true,
			isV4:           true,
			srcIP:          ateFlowIPv4InvalidSrc,
			dstIP:          ateFlowIPv4Dst,
			flowName:       "v4_invalid_src",
			verifyCounters: true,
		},
		{
			desc:           "URPF-1.1.2: uRPF with invalid IPv6 source",
			gueEnabled:     false,
			expectLoss:     true,
			isV4:           false,
			srcIP:          ateFlowIPv6InvalidSrc,
			dstIP:          ateFlowIPv6Dst,
			flowName:       "v6_invalid_src",
			verifyCounters: true,
		},
		{
			desc:       "URPF-1.1.3: uRPF with valid IPv4 source and GUE",
			gueEnabled: true,
			expectLoss: false,
			isV4:       true,
			srcIP:      ateFlowIPv4ValidSrc,
			dstIP:      ateFlowIPv4Dst,
			flowName:   "v4_valid_src_gue",
		},
		{
			desc:       "URPF-1.1.3: uRPF with valid IPv6 source and GUE",
			gueEnabled: true,
			expectLoss: false,
			isV4:       false,
			srcIP:      ateFlowIPv6ValidSrc,
			dstIP:      ateFlowIPv6Dst,
			flowName:   "v6_valid_src_gue",
		},
		{
			desc:           "URPF-1.1.4: uRPF with invalid IPv4 source and GUE",
			gueEnabled:     true,
			expectLoss:     true,
			isV4:           true,
			srcIP:          ateFlowIPv4InvalidSrc,
			dstIP:          ateFlowIPv4Dst,
			flowName:       "v4_invalid_src_gue",
			verifyCounters: true,
		},
		{
			desc:           "URPF-1.1.4: uRPF with invalid IPv6 source and GUE",
			gueEnabled:     true,
			expectLoss:     true,
			isV4:           false,
			srcIP:          ateFlowIPv6InvalidSrc,
			dstIP:          ateFlowIPv6Dst,
			flowName:       "v6_invalid_src_gue",
			verifyCounters: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			if tc.expectLoss && isKnownURPFLookupVrfUnsupportedOnDUT(dut) {
				t.Skipf("Skipping %s: DUT supports CLI uRPF reachable-via any but not non-default lookup-vrf semantics; invalid-source traffic may be validated in default VRF and forward", tc.desc)
			}
			verifyGUEEncap := tc.gueEnabled && !tc.expectLoss
			if verifyGUEEncap {
				enableCapture(t, otgConfig, []string{rxOTGPortName})
			}
			if tc.gueEnabled {
				t.Log("Configuring GUE on DUT")
				dstAddr := []string{atePort2.IPv4}
				configureGUEEncap(t, dut, dutLoopback.IPv4, dstAddr, udpDestPort)
			}
			var initialDropCount uint64
			p1 := dut.Port(t, "port1")
			if tc.verifyCounters {
				if !deviations.URPFConfigOCUnsupported(dut) {
					initialDropCount = urpfDropPkts(t, dut, p1.Name(), tc.isV4)
				}
				t.Logf("Initial uRPF drop count: %d", initialDropCount)
			}
			flow := createFlow(t, dut, otgConfig, tc.flowName, tc.srcIP, tc.dstIP, tc.isV4)
			ate.OTG().PushConfig(t, otgConfig)
			ate.OTG().StartProtocols(t)
			var cs gosnappi.ControlState
			if verifyGUEEncap {
				cs = startCapture(t, ate.OTG())
			}
			txPackets := verifyTraffic(t, ate, otgConfig, flow.Name(), tc.expectLoss, rxOTGPortName)
			if verifyGUEEncap {
				stopCapture(t, ate.OTG(), cs)
				verifyGUECaptureOnPort(t, ate.OTG(), rxOTGPortName, dutLoopback.IPv4, atePort2.IPv4, udpDestPort)
			}
			if tc.verifyCounters {
				if !deviations.URPFConfigOCUnsupported(dut) {
					verifyURPFCounters(t, dut, p1.Name(), tc.isV4, initialDropCount, txPackets)
				}
			}
		})
	}
}

// isKnownURPFLookupVrfUnsupportedOnDUT returns true for platforms where this test's
// non-default lookup-vrf behavior is not enforceable via available uRPF CLI.
func isKnownURPFLookupVrfUnsupportedOnDUT(dut *ondatra.DUTDevice) bool {
	return deviations.URPFConfigOCUnsupported(dut) && dut.Vendor() == ondatra.ARISTA
}

// verifyURPFTelemetryState validates the uRPF telemetry leaves covered by the README.
func verifyURPFTelemetryState(t *testing.T, dut *ondatra.DUTDevice, portName string) {
	t.Helper()
	v4URPF := gnmi.OC().Interface(portName).Subinterface(0).Ipv4().Urpf()
	v6URPF := gnmi.OC().Interface(portName).Subinterface(0).Ipv6().Urpf()

	if got := gnmi.Get(t, dut, v4URPF.Enabled().State()); !got {
		t.Errorf("IPv4 uRPF enabled state mismatch on %s: got %v, want true", portName, got)
	}
	if got := gnmi.Get(t, dut, v4URPF.Mode().State()); got != oc.IfIp_UrpfMode_LOOSE {
		t.Errorf("IPv4 uRPF mode state mismatch on %s: got %v, want %v", portName, got, oc.IfIp_UrpfMode_LOOSE)
	}

	if got := gnmi.Get(t, dut, v6URPF.Enabled().State()); !got {
		t.Errorf("IPv6 uRPF enabled state mismatch on %s: got %v, want true", portName, got)
	}
	if got := gnmi.Get(t, dut, v6URPF.Mode().State()); got != oc.IfIp_UrpfMode_LOOSE {
		t.Errorf("IPv6 uRPF mode state mismatch on %s: got %v, want %v", portName, got, oc.IfIp_UrpfMode_LOOSE)
	}

	if deviations.URPFConfigOCUnsupported(dut) {
		t.Logf("Skipping strict lookup-network-instance telemetry verification on %s due to urpf_config_oc_unsupported deviation", dut.Vendor())
		return
	}

	if got, ok := gnmi.Lookup(t, dut, v4URPF.LookupNetworkInstance().State()).Val(); !ok {
		t.Errorf("IPv4 uRPF lookup-network-instance state missing on %s", portName)
	} else if got != nonDefaultVRF {
		t.Errorf("IPv4 uRPF lookup-network-instance mismatch on %s: got %q, want %q", portName, got, nonDefaultVRF)
	}

	if got, ok := gnmi.Lookup(t, dut, v6URPF.LookupNetworkInstance().State()).Val(); !ok {
		t.Errorf("IPv6 uRPF lookup-network-instance state missing on %s", portName)
	} else if got != nonDefaultVRF {
		t.Errorf("IPv6 uRPF lookup-network-instance mismatch on %s: got %q, want %q", portName, got, nonDefaultVRF)
	}
}

// enableCapture configures OTG captures on the provided OTG port names.
// Capture entries use the same port name for both capture name and binding.
func enableCapture(t *testing.T, config gosnappi.Config, portNames []string) {
	config.Captures().Clear()
	for _, portName := range portNames {
		cap := config.Captures().Add()
		cap.SetName(portName)
		cap.SetPortNames([]string{portName})
		cap.SetFormat(gosnappi.CaptureFormat.PCAP)
		t.Logf("Enabled capture on port %s", portName)
	}
}

// startCapture starts packet capture on all capture-enabled OTG ports.
func startCapture(t *testing.T, ateOTG *otg.OTG) gosnappi.ControlState {
	t.Helper()
	cs := gosnappi.NewControlState()
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.START)
	ateOTG.SetControlState(t, cs)
	return cs
}

// stopCapture stops packet capture using the control state used to start capture.
func stopCapture(t *testing.T, ateOTG *otg.OTG, cs gosnappi.ControlState) {
	t.Helper()
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.STOP)
	ateOTG.SetControlState(t, cs)
}

// verifyGUECaptureOnPort validates that at least one captured packet on the given
// OTG port matches the expected GUE outer IPv4 source/destination and UDP destination port.
func verifyGUECaptureOnPort(t *testing.T, ateOTG *otg.OTG, portName, wantSrcIP, wantDstIP string, wantUDPDstPort uint16) {
	t.Helper()
	bytes := ateOTG.GetCapture(t, gosnappi.NewCaptureRequest().SetPortName(portName))
	pcapFile, err := os.CreateTemp("", "urpf-gue-*.pcap")
	if err != nil {
		t.Fatalf("failed to create pcap temp file: %v", err)
	}
	defer os.Remove(pcapFile.Name())
	defer pcapFile.Close()
	if _, err := pcapFile.Write(bytes); err != nil {
		t.Fatalf("failed writing capture bytes: %v", err)
	}

	handle, err := pcap.OpenOffline(pcapFile.Name())
	if err != nil {
		t.Fatalf("failed opening capture file: %v", err)
	}
	defer handle.Close()

	found := false
	for pkt := range gopacket.NewPacketSource(handle, handle.LinkType()).Packets() {
		ipv4Layer := pkt.Layer(layers.LayerTypeIPv4)
		udpLayer := pkt.Layer(layers.LayerTypeUDP)
		if ipv4Layer == nil || udpLayer == nil {
			continue
		}
		ip4, ok := ipv4Layer.(*layers.IPv4)
		if !ok {
			continue
		}
		udp, ok := udpLayer.(*layers.UDP)
		if !ok {
			continue
		}
		if ip4.SrcIP.String() == wantSrcIP && ip4.DstIP.String() == wantDstIP && uint16(udp.DstPort) == wantUDPDstPort {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("did not find GUE encapsulated packet on %s with outer src=%s dst=%s udp dst=%d", portName, wantSrcIP, wantDstIP, wantUDPDstPort)
	}
	t.Logf("Verified GUE encapsulation on %s: outer src=%s dst=%s udp dst=%d", portName, wantSrcIP, wantDstIP, wantUDPDstPort)
}

// verifyURPFCounters checks if the uRPF drop counter has incremented as expected.
func verifyURPFCounters(t *testing.T, dut *ondatra.DUTDevice, portName string, isV4 bool, initialDropCount, expectedIncrement uint64) {
	t.Helper()
	var query ygnmi.SingletonQuery[uint64]
	if isV4 {
		query = gnmi.OC().Interface(portName).Subinterface(0).Ipv4().Counters().UrpfDropPkts().State()
	} else {
		query = gnmi.OC().Interface(portName).Subinterface(0).Ipv6().Counters().UrpfDropPkts().State()
	}
	gnmi.Watch(t, dut, query, 30*time.Second, func(val *ygnmi.Value[uint64]) bool {
		newDropCount, present := val.Val()
		if !present {
			return false
		}
		dropCount := newDropCount - initialDropCount
		return dropCount == expectedIncrement
	}).Await(t)
}

// urpfDropPkts reads the IPv4 or IPv6 uRPF drop packet counter from subinterface 0.
func urpfDropPkts(t *testing.T, dut *ondatra.DUTDevice, portName string, isV4 bool) uint64 {
	t.Helper()
	if isV4 {
		if v, ok := gnmi.Lookup(t, dut, gnmi.OC().Interface(portName).Subinterface(0).Ipv4().Counters().UrpfDropPkts().State()).Val(); ok {
			return v
		}
		t.Fatalf("IPv4 urpf-drop-pkts is not available on interface %s subinterface 0", portName)
	}
	if v, ok := gnmi.Lookup(t, dut, gnmi.OC().Interface(portName).Subinterface(0).Ipv6().Counters().UrpfDropPkts().State()).Val(); ok {
		return v
	}
	t.Fatalf("IPv6 urpf-drop-pkts is not available on interface %s subinterface 0", portName)
	return 0
}

// bgpRouteVerification build routes parameters and verify routes if advertised routes are installed in DUT AFT.
func bgpRouteVerification(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	defaultNIName := deviations.DefaultNetworkInstance(dut)
	// Build routes to advertise
	routesToAdvertise := map[string]cfgplugins.RouteInfo{
		fmt.Sprintf("%s/%d", ateAdvIPv4Prefix1, prefixIPv4Len): {VRF: nonDefaultVRF, IPType: cfgplugins.IPv4, DefaultName: defaultNIName},
		fmt.Sprintf("%s/%d", ateAdvIPv4Prefix2, prefixIPv4Len): {VRF: defaultNIName, IPType: cfgplugins.IPv4, DefaultName: defaultNIName},
		fmt.Sprintf("%s/%d", ateAdvIPv4Prefix3, prefixIPv4Len): {VRF: defaultNIName, IPType: cfgplugins.IPv4, DefaultName: defaultNIName},
		fmt.Sprintf("%s/%d", ateAdvIPv6Prefix1, prefixIPv6Len): {VRF: nonDefaultVRF, IPType: cfgplugins.IPv6, DefaultName: defaultNIName},
		fmt.Sprintf("%s/%d", ateAdvIPv6Prefix2, prefixIPv6Len): {VRF: defaultNIName, IPType: cfgplugins.IPv6, DefaultName: defaultNIName},
		fmt.Sprintf("%s/%d", ateAdvIPv6Prefix3, prefixIPv6Len): {VRF: defaultNIName, IPType: cfgplugins.IPv6, DefaultName: defaultNIName},
	}
	cfgplugins.VerifyRoutes(t, dut, routesToAdvertise)
}

// verifyInvalidPrefixPlacement validates the invalid prefixes are learned in default VRF but not present
// in the non-default VRF used for uRPF lookup.
func verifyInvalidPrefixPlacement(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	defaultNIName := deviations.DefaultNetworkInstance(dut)
	tests := []struct {
		prefix string
		isV4   bool
	}{
		{prefix: fmt.Sprintf("%s/%d", ateAdvIPv4Prefix2, prefixIPv4Len), isV4: true},
		{prefix: fmt.Sprintf("%s/%d", ateAdvIPv6Prefix2, prefixIPv6Len), isV4: false},
	}

	for _, tc := range tests {
		if !prefixInNIAFT(t, dut, defaultNIName, tc.prefix, tc.isV4) {
			t.Errorf("prefix %s not found in %s AFT, want present", tc.prefix, defaultNIName)
		}
		if prefixInNIAFT(t, dut, nonDefaultVRF, tc.prefix, tc.isV4) {
			t.Errorf("prefix %s found in %s AFT, want absent", tc.prefix, nonDefaultVRF)
		}
	}
}

// prefixInNIAFT reports whether the given prefix exists in the specified network-instance AFT.
func prefixInNIAFT(t *testing.T, dut *ondatra.DUTDevice, niName, prefix string, isV4 bool) bool {
	t.Helper()
	aft := gnmi.OC().NetworkInstance(niName).Afts()
	if isV4 {
		_, ok := gnmi.Lookup(t, dut, aft.Ipv4Entry(prefix).State()).Val()
		return ok
	}
	_, ok := gnmi.Lookup(t, dut, aft.Ipv6Entry(prefix).State()).Val()
	return ok
}
