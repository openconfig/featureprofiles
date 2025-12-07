// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package multipath_bgp_ecmp_protocol_nexthop

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/gnmi/otg"
	"github.com/openconfig/ygnmi/ygnmi"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// The test topology consists of a DUT connected to an ATE with four ports.
// iBGP is configured between DUT Port2 and ATE Port2, DUT Port3 and ATE Port3,
// and DUT Port4 and ATE Port4.
// ISIS is configured on interfaces for ports 2, 3, and 4.
// ATE Port2, Port3 and Port4 advertise loopback addresses via ISIS.
// ATE Port2, Port3 and Port4 advertise prefix 100.1.1.0/24 via iBGP, with
// next-hops being their loopback addresses respectively.
// These loopback addresses are reachable via ISIS paths over DUT Port2, Port3 and Port4.
// The test verifies that DUT installs ECMP paths for prefixes 100.1.1.0/24
// and 100.1.1.1::1/128.
//
// Topology:
//
//	ATE port 1 -------- DUT port 1
//	ATE port 2 -------- DUT port 2
//	ATE port 3 -------- DUT port 3
//	ATE port 4 -------- DUT port 4
//
// Configuration:
//
//	DUT
//	  Port 1: 192.1.1.1/24, 2001:192:1:1::1/64
//	  Port 2: 192.1.2.1/24, 2001:192:1:2::1/64
//	  Port 3: 192.1.3.1/24, 2001:192:1:3::1/64
//	  Port 4: 192.1.4.1/24, 2001:192:1:4::1/64
//	  ISIS level2 on Port2, Port3, Port4 with metric 10
//	  iBGP sessions with 192.1.2.2, 192.1.3.2, 192.1.4.2 (ATE Port2,3,4)
//	  iBGP sessions with 2001:192:1:2::2, 2001:192:1:3::2, 2001:192:1:4::2 (ATE Port2,3,4)
//	  BGP router-id: 192.1.1.1
//	  AS: 65001
//    iBGP multipath enabled.
//
//	ATE
//	  Port 1: 192.1.1.2/24, 2001:192:1:1::2/64
//	  Port 2: 192.1.2.2/24, 2001:192:1:2::2/64
//	    Loopback IPv4: 193.1.1.1/32 (advertised in ISIS)
//	    Loopback IPv6: 193:1:1::1/128 (advertised in ISIS)
//	  Port 3: 192.1.3.2/24, 2001:192:1:3::2/64
//	    Loopback IPv4: 193.1.1.2/32 (advertised in ISIS)
//	    Loopback IPv6: 193:1:2::1/128 (advertised in ISIS)
//	  Port 4: 192.1.4.2/24, 2001:192:1:4::2/64
//	    Loopback IPv4: 193.1.1.3/32 (advertised in ISIS)
//	    Loopback IPv6: 193:1:3::1/128 (advertised in ISIS)
//	  ISIS level2 on Port2, Port3, Port4 with metric 10
//	  iBGP sessions with 192.1.2.1, 192.1.3.1, 192.1.4.1 (DUT Port2,3,4)
//	  iBGP sessions with 2001:192:1:2::1, 2001:192:1:3::1, 2001:192:1:4::1 (DUT Port2,3,4)
//	  AS: 65001
//	  ATE Port2 advertises 100.1.1.0/24 (next-hop 193.1.1.1) and 100:1:1::/48 (next-hop 193:1:1::1)
//	  ATE Port3 advertises 100.1.1.0/24 (next-hop 193.1.1.2) and 100:1:1::/48 (next-hop 193:1:2::1)
//	  ATE Port4 advertises 100.1.1.0/24 (next-hop 193.1.1.3) and 100:1:1::/48 (next-hop 193:1:3::1)

const (
	dutAS            = 64501
	ateAS            = 64501
	ateP2Lo0IP       = "193.1.1.1"
	ateP3Lo0IP       = "193.1.1.2"
	ateP4Lo0IP       = "193.1.1.3"
	p1IPv4           = "192.1.1.1"
	p1IPv4Len        = 24
	p2IPv4           = "192.1.2.1"
	p2IPv4Len        = 24
	p3IPv4           = "192.1.3.1"
	p3IPv4Len        = 24
	p4IPv4           = "192.1.4.1"
	p4IPv4Len        = 24
	ateP1IPv4        = "192.1.1.2"
	ateP2IPv4        = "192.1.2.2"
	ateP3IPv4        = "192.1.3.2"
	ateP4IPv4        = "192.1.4.2"
	p1IPv6           = "2001:192:1:1::1"
	p1IPv6Len        = 64
	p2IPv6           = "2001:192:1:2::1"
	p2IPv6Len        = 64
	p3IPv6           = "2001:192:1:3::1"
	p3IPv6Len        = 64
	p4IPv6           = "2001:192:1:4::1"
	p4IPv6Len        = 64
	ateP1IPv6        = "2001:192:1:1::2"
	ateP2IPv6        = "2001:192:1:2::2"
	ateP3IPv6        = "2001:192:1:3::2"
	ateP4IPv6        = "2001:192:1:4::2"
	ateP2Lo0IPv6     = "193:1:1::1"
	ateP3Lo0IPv6     = "193:1:2::1"
	ateP4Lo0IPv6     = "193:1:3::1"
	isisMetric       = 10
	prefixMin        = "100.1.1.0"
	prefixLen        = 24
	prefixV6Min      = "100:1:1::"
	prefixV6Len      = 48
	prefixCount      = 1
	bgpPassword      = "BGPKEY"
	isisAreaAddr     = "49.0001"
	isisSysID        = "640000000001"
	totalPackets     = 1000
	trafficPps       = 100
	lossTolerancePct = 0  // Packet loss tolerance in percentage
	lbToleranceFms   = 20 // Load Balance Tolerance in percentage
)

var (
	dutP1 = attrs.Attributes{
		Desc:    "DUT to ATE Port 1",
		IPv4:    p1IPv4,
		IPv4Len: p1IPv4Len,
		IPv6:    p1IPv6,
		IPv6Len: p1IPv6Len,
	}
	dutP2 = attrs.Attributes{
		Desc:    "DUT to ATE Port 2",
		IPv4:    p2IPv4,
		IPv4Len: p2IPv4Len,
		IPv6:    p2IPv6,
		IPv6Len: p2IPv6Len,
	}
	dutP3 = attrs.Attributes{
		Desc:    "DUT to ATE Port 3",
		IPv4:    p3IPv4,
		IPv4Len: p3IPv4Len,
		IPv6:    p3IPv6,
		IPv6Len: p3IPv6Len,
	}
	dutP4 = attrs.Attributes{
		Desc:    "DUT to ATE Port 4",
		IPv4:    p4IPv4,
		IPv4Len: p4IPv4Len,
		IPv6:    p4IPv6,
		IPv6Len: p4IPv6Len,
	}
	ateP1 = attrs.Attributes{
		Name:    "ateP1",
		IPv4:    ateP1IPv4,
		IPv4Len: p1IPv4Len,
		IPv6:    ateP1IPv6,
		IPv6Len: p1IPv6Len,
	}
	ateP2 = attrs.Attributes{
		Name:    "ateP2",
		IPv4:    ateP2IPv4,
		IPv4Len: p2IPv4Len,
		IPv6:    ateP2IPv6,
		IPv6Len: p2IPv6Len,
	}
	ateP3 = attrs.Attributes{
		Name:    "ateP3",
		IPv4:    ateP3IPv4,
		IPv4Len: p3IPv4Len,
		IPv6:    ateP3IPv6,
		IPv6Len: p3IPv6Len,
	}
	ateP4 = attrs.Attributes{
		Name:    "ateP4",
		IPv4:    ateP4IPv4,
		IPv4Len: p4IPv4Len,
		IPv6:    ateP4IPv6,
		IPv6Len: p4IPv6Len,
	}
)

type testCase struct {
	desc      string
	ipv4      bool
	ipv6      bool
	multipath bool
}

func TestMultipathBGPEcmpProtocolNexthop(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	configureDUT(t, dut)

	otgCfg := configureATE(t, ate)
	ate.OTG().PushConfig(t, otgCfg)
	ate.OTG().StartProtocols(t)
	testCases := []testCase{
		{
			desc:      "Testing with multipath disabled for ipv4 ",
			ipv4:      true,
			ipv6:      true,
			multipath: false,
		},
		{
			desc:      "Testing with multipath enabled for ipv4",
			ipv4:      true,
			ipv6:      false,
			multipath: true,
		},
		{
			desc:      "Testing with multipath disabled for ipv6",
			ipv4:      false,
			ipv6:      true,
			multipath: false,
		},
		{
			desc:      "Testing with multipath enabled for ipv6",
			ipv4:      false,
			ipv6:      true,
			multipath: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			if tc.ipv4 {
				t.Logf("Validating traffic test for IPv4 prefixes: [%s, %s]", prefixMin, prefixLen)
				if tc.multipath {
					t.Logf("Multipath is enabled for IPv4 prefixes: [%s, %s]", prefixMin, prefixLen)
				} else {
					t.Logf("Multipath is disabled for IPv4 prefixes: [%s, %s]", prefixMin, prefixLen)
				}
				verifyDUT(t, dut)
				verifyTraffic(t, ate, "ipv4", 0)
				sleepTime := time.Duration(totalPackets/trafficPps) + 5
				ate.OTG().StartTraffic(t)
				time.Sleep(sleepTime * time.Second)
				ate.OTG().StopTraffic(t)
				otgutils.LogFlowMetrics(t, ate.OTG(), ate)
				checkPacketLoss(t, ate)
				verifyECMPLoadBalance(t, ate, int(cfgplugins.PortCount4), 3)
			}
			if tc.ipv6 {
				t.Logf("Validating traffic test for IPv6 prefixes: [%s, %s]", prefixV6Min, prefixV6Len)
				if tc.multipath {
					t.Logf("Multipath is enabled for IPv6 prefixes: [%s, %s]", prefixV6Min, prefixV6Len)
				} else {
					t.Logf("Multipath is disabled for IPv6 prefixes: [%s, %s]", prefixV6Min, prefixV6Len)
				}
				verifyDUT(t, dut)
				verifyTraffic(t, ate, "ipv4", 0)
				sleepTime := time.Duration(totalPackets/trafficPps) + 5
				ate.OTG().StartTraffic(t)
				time.Sleep(sleepTime * time.Second)
				ate.OTG().StopTraffic(t)

				otgutils.LogFlowMetrics(t, ate.OTG(), ate)
				checkPacketLoss(t, ate)
				verifyECMPLoadBalance(t, ate, int(cfgplugins.PortCount4), 3)
			}
		})
	}
}

func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	d := gnmi.OC()
	isis := d.NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, deviations.ISISInstance(dut)).Isis()
	gnmi.Replace(t, dut, isis.Global().Config(), &oc.NetworkInstance_Protocol_Isis_Global{
		LevelCapability: oc.Isis_LevelCapability_LEVEL_2,
		Net:             []string{fmt.Sprintf("%s.%s.00", isisAreaAddr, isisSysID)},
		Instance:        ygnmi.String(deviations.ISISInstance(dut)),
	})

	// Configure DUT port 1 with IPv4 and IPv6 addresses.
	// This is the DUT side of ATE port 1 used for sending traffic. Source traffic
	// is sent from ATE port 1.
	p1 := dut.Port(t, "port1")
	gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), dutP1.NewOCInterface(p1.Name(), dut))

	for _, p := range []struct {
		port *ondatra.Port
		attr attrs.Attributes
	}{
		{dut.Port(t, "port2"), dutP2},
		{dut.Port(t, "port3"), dutP3},
		{dut.Port(t, "port4"), dutP4},
	} {
		intfName := p.port.Name()
		gnmi.Replace(t, dut, d.Interface(intfName).Config(), p.attr.NewOCInterface(intfName, dut))
		i := isis.Interface(intfName)
		gnmi.Replace(t, dut, i.Config(), &oc.NetworkInstance_Protocol_Isis_Interface{
			Enabled:     ygnmi.Bool(true),
			CircuitType: oc.Isis_CircuitType_POINT_TO_POINT,
		})
		gnmi.Replace(t, dut, i.Level(2).Config(), &oc.NetworkInstance_Protocol_Isis_Interface_Level{LevelNumber: ygnmi.Uint8(2)})
		gnmi.Replace(t, dut, i.Level(2).Af(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Config(), &oc.NetworkInstance_Protocol_Isis_Interface_Level_Af{
			Metric: ygnmi.Uint32(isisMetric),
		})
		gnmi.Replace(t, dut, i.Level(2).Af(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Config(), &oc.NetworkInstance_Protocol_Isis_Interface_Level_Af{
			Metric: ygnmi.Uint32(isisMetric),
		})
	}

	bgp := d.NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	gnmi.Replace(t, dut, bgp.Global().Config(), &oc.NetworkInstance_Protocol_Bgp_Global{
		As:       ygnmi.Uint32(dutAS),
		RouterId: ygnmi.String(p1IPv4),
	})
	gnmi.Update(t, dut, bgp.Global().UseMultiplePaths().Ibgp().Config(), &oc.NetworkInstance_Protocol_Bgp_Global_UseMultiplePaths_Ibgp{MaximumPaths: ygnmi.Uint32(8)})

	for _, neighborIP := range []string{ateP2IPv4, ateP3IPv4, ateP4IPv4} {
		n := bgp.Neighbor(neighborIP)
		gnmi.Replace(t, dut, n.Config(), &oc.NetworkInstance_Protocol_Bgp_Neighbor{
			PeerAs:       ygnmi.Uint32(ateAS),
			Enabled:      ygnmi.Bool(true),
			AuthPassword: ygnmi.String(bgpPassword),
		})
		gnmi.Replace(t, dut, n.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Config(), &oc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi{
			AfiSafiName: oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST,
			Enabled:     ygnmi.Bool(true),
		})
	}

	for _, neighborIP := range []string{ateP2IPv6, ateP3IPv6, ateP4IPv6} {
		n := bgp.Neighbor(neighborIP)
		gnmi.Replace(t, dut, n.Config(), &oc.NetworkInstance_Protocol_Bgp_Neighbor{
			PeerAs:       ygnmi.Uint32(ateAS),
			Enabled:      ygnmi.Bool(true),
			AuthPassword: ygnmi.String(bgpPassword),
		})
		gnmi.Replace(t, dut, n.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Config(), &oc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi{
			AfiSafiName: oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST,
			Enabled:     ygnmi.Bool(true),
		})
	}
}

func configureATE(t *testing.T, ate *ondatra.ATEDevice) *otg.Config {
	top := ate.Topology().New()
	t.Log("Configure ATE interface")
	p1 := ate.Port(t, "port1")
	p2 := ate.Port(t, "port2")
	p3 := ate.Port(t, "port3")
	p4 := ate.Port(t, "port4")

	// Add interface for ATE port 1.
	i1 := top.AddInterface(ateP1.Name).WithPort(p1)
	i1.IPv4().WithAddress(fmt.Sprintf("%s/%d", ateP1.IPv4, ateP1.IPv4Len)).WithDefaultGateway(p1IPv4)
	i1.IPv6().WithAddress(fmt.Sprintf("%s/%d", ateP1.IPv6, ateP1.IPv6Len)).WithDefaultGateway(p1IPv6)

	ports := []struct {
		portIdx int
		port    *ondatra.Port
		attrs   attrs.Attributes
		loIPv4  string
		loIPv6  string
		dutIPv4 string
		dutIPv6 string
	}{
		{portIdx: 2, port: p2, attrs: ateP2, loIPv4: ateP2Lo0IP, loIPv6: ateP2Lo0IPv6, dutIPv4: p2IPv4, dutIPv6: p2IPv6},
		{portIdx: 3, port: p3, attrs: ateP3, loIPv4: ateP3Lo0IP, loIPv6: ateP3Lo0IPv6, dutIPv4: p3IPv4, dutIPv6: p3IPv6},
		{portIdx: 4, port: p4, attrs: ateP4, loIPv4: ateP4Lo0IP, loIPv6: ateP4Lo0IPv6, dutIPv4: p4IPv4, dutIPv6: p4IPv6},
	}

	for _, p := range ports {
		intf := top.AddInterface(p.attrs.Name).WithPort(p.port)
		intf.IPv4().WithAddress(fmt.Sprintf("%s/%d", p.attrs.IPv4, p.attrs.IPv4Len)).WithDefaultGateway(p.dutIPv4)
		intf.IPv6().WithAddress(fmt.Sprintf("%s/%d", p.attrs.IPv6, p.attrs.IPv6Len)).WithDefaultGateway(p.dutIPv6)
		intf.ISIS().WithLevelL2().WithMetric(isisMetric).WithNetworkTypePointToPoint().WithHelloPaddingEnabled(true)
		net := intf.AddNetwork(fmt.Sprintf("n%d", p.portIdx))
		net.IPv4().WithAddress(p.loIPv4 + "/32")
		net.IPv6().WithAddress(p.loIPv6 + "/128")
		net.ISIS().WithIPReachabilityInternal()
		bgp := intf.BGP()
		bgp4 := bgp.AddPeer()
		bgp4.WithPeerAddress(p.dutIPv4).WithLocalASN(ateAS).WithTypeInternal().WithMD5Key(bgpPassword)
		bgp4.Capabilities().WithIPv4UnicastEnabled(true)
		bgpNet4 := intf.AddNetwork(fmt.Sprintf("bgpNet%dIpv4", p.portIdx))
		bgpNet4.IPv4().WithAddress(prefixMin + "/" + fmt.Sprint(prefixLen)).WithCount(prefixCount)
		// bgpNet4.BGP().WithNextHopAddress(p.loIPv4).WithPeer(bgp4)
		bgp6 := bgp.AddPeer()
		bgp6.WithPeerAddress(p.dutIPv6).WithLocalASN(ateAS).WithTypeInternal().WithMD5Key(bgpPassword)
		bgp6.Capabilities().WithIPv6UnicastEnabled(true)
		bgpNet6 := intf.AddNetwork(fmt.Sprintf("bgpNet%dIpv6", p.portIdx))
		bgpNet6.IPv6().WithAddress(prefixV6Min + "/" + fmt.Sprint(prefixV6Len)).WithCount(prefixCount)
		// bgpNet6.BGP().WithNextHopAddress(p.loIPv6).WithPeer(bgp6)
	}
	return top
}

func verifyDUT(t *testing.T, dut *ondatra.DUTDevice) {
	statePathV4 := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).
		Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").
		Bgp().Rib().AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Ipv4Unicast()
	statePathV6 := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).
		Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").
		Bgp().Rib().AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Ipv6Unicast()

	t.Logf("Verifying DUT BGP sessions up for IPv4")
	for _, neighborIP := range []string{ateP2IPv4, ateP3IPv4, ateP4IPv4} {
		gnmi.Await(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).
			Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").
			Bgp().Neighbor(neighborIP).SessionState().State(), 2*time.Minute, oc.Bgp_Neighbor_SessionState_ESTABLISHED)
	}
	t.Logf("Verifying DUT BGP sessions up for IPv6")
	for _, neighborIP := range []string{ateP2IPv6, ateP3IPv6, ateP4IPv6} {
		gnmi.Await(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).
			Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").
			Bgp().Neighbor(neighborIP).SessionState().State(), 2*time.Minute, oc.Bgp_Neighbor_SessionState_ESTABLISHED)
	}

	prefixV4 := fmt.Sprintf("%s/%d", prefixMin, prefixLen)
	for _, neighborIP := range []string{ateP2IPv4, ateP3IPv4, ateP4IPv4} {
		routes := gnmi.Lookup(t, dut, statePathV4.Neighbor(neighborIP).AdjRibInPost().Route(prefixV4, 0).State())
		if !routes.IsPresent() {
			t.Errorf("Prefix %s not found in AdjRibInPost for neighbor %s", prefixV4, neighborIP)
		}
		locRib := gnmi.Lookup(t, dut, statePathV4.LocRib().Route(prefixV4, oc.UnionString(neighborIP), 0).State())
		if !locRib.IsPresent() {
			t.Errorf("Prefix %s not found in LocRib from %s", prefixV4, neighborIP)
		}
	}

	prefixV6 := fmt.Sprintf("%s/%d", prefixV6Min, prefixV6Len)
	for _, neighborIP := range []string{ateP2IPv6, ateP3IPv6, ateP4IPv6} {
		routes := gnmi.Lookup(t, dut, statePathV6.Neighbor(neighborIP).AdjRibInPost().Route(prefixV6, 0).State())
		if !routes.IsPresent() {
			t.Errorf("Prefix %s not found in AdjRibInPost for neighbor %s", prefixV6, neighborIP)
		}
		locRib := gnmi.Lookup(t, dut, statePathV6.LocRib().Route(prefixV6, oc.UnionString(neighborIP), 0).State())
		if !locRib.IsPresent() {
			t.Errorf("Prefix %s not found in LocRib from %s", prefixV6, neighborIP)
		}
	}

	ipv4Entries := gnmi.Lookup(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Afts().Ipv4Entry(prefixV4).State())
	if !ipv4Entries.IsPresent() {
		t.Fatalf("Prefix %s not found in AFT", prefixV4)
	}
	for _, ipv4Entry := range ipv4Entries {
		nhgID := ipv4Entry.GetNextHopGroup()
		if nhgID == 0 {
			t.Fatalf("Prefix %s doesn't have a next-hop-group", prefixV4)
		}
		nhs := gnmi.Lookup(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Afts().NextHopGroup(nhgID).NextHopAny().State())
		if len(nhs) != 3 {
			t.Errorf("Prefix %s has %d next-hops in NHG %d, want 3 for ECMP", prefixV4, len(nhs), nhgID)
		} else {
			t.Logf("Prefix %s has %d next-hops in NHG %d, ECMP is active.", prefixV4, len(nhs), nhgID)
		}
	}

	ipv6Entries := gnmi.Lookup(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Afts().Ipv6Entry(prefixV6).State())
	if !ipv6Entries.IsPresent() {
		t.Fatalf("Prefix %s not found in AFT", prefixV6)
	}
	for _, ipv6Entry := range ipv6Entries {
		nhgID := ipv6Entry.GetNextHopGroup()
		if nhgID == 0 {
			t.Fatalf("Prefix %s doesn't have a next-hop-group", prefixV6)
		}
		nhs := gnmi.Lookup(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Afts().NextHopGroup(nhgID).NextHopAny().State())
		if len(nhs) != 3 {
			t.Errorf("Prefix %s has %d next-hops in NHG %d, want 3 for ECMP", prefixV6, len(nhs), nhgID)
		} else {
			t.Logf("Prefix %s has %d next-hops in NHG %d, ECMP is active.", prefixV6, len(nhs), nhgID)
		}
	}
}

func configureFlow(t *testing.T, bs *cfgplugins.BGPSession, prefixPair []string, prefixType string, index int) {
	flow := bs.ATETop.Flows().Add().SetName("flow" + prefixType + strconv.Itoa(index))
	flow.Metrics().SetEnable(true)

	if prefixType == "ipv4" {
		flow.TxRx().Device().
			SetTxNames([]string{bs.ATEPorts[0].Name + ".IPv4"}).
			SetRxNames([]string{bs.ATEPorts[1].Name + ".BGP4.peer.dut." + strconv.Itoa(index)})
	} else {
		flow.TxRx().Device().
			SetTxNames([]string{bs.ATEPorts[0].Name + ".IPv6"}).
			SetRxNames([]string{bs.ATEPorts[1].Name + ".BGP6.peer.dut." + strconv.Itoa(index)})
	}

	flow.Duration().FixedPackets().SetPackets(totalPackets)
	flow.Size().SetFixed(1500)
	flow.Rate().SetPps(trafficPps)

	e := flow.Packet().Add().Ethernet()
	e.Src().SetValue(bs.ATEPorts[1].MAC)

	if prefixType == "ipv4" {
		v4 := flow.Packet().Add().Ipv4()
		v4.Src().SetValue(bs.ATEPorts[0].IPv4)
		v4.Dst().SetValues(prefixPair)
	} else {
		v6 := flow.Packet().Add().Ipv6()
		v6.Src().SetValue(bs.ATEPorts[0].IPv6)
		v6.Dst().SetValues(prefixPair)
	}
}

func verifyTraffic(t *testing.T, ate *ondatra.ATEDevice, prefixType string, index int) {
	recvMetric := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow("flow"+prefixType+strconv.Itoa(index)).State())
	framesTx := recvMetric.GetCounters().GetOutPkts()
	framesRx := recvMetric.GetCounters().GetInPkts()

	if framesTx == 0 {
		t.Error("No traffic was generated and frames transmitted were 0")
	} else if framesRx == framesTx {
		t.Logf("Traffic validation successful FramesTx: %d FramesRx: %d", framesTx, framesRx)
	}
}

func checkPacketLoss(t *testing.T, ate *ondatra.ATEDevice) {
	countersPath := gnmi.OTG().Flow("flow").Counters()
	rxPackets := gnmi.Get(t, ate.OTG(), countersPath.InPkts().State())
	txPackets := gnmi.Get(t, ate.OTG(), countersPath.OutPkts().State())
	lostPackets := txPackets - rxPackets
	if txPackets < 1 {
		t.Fatalf("Tx packets should be higher than 0")
	}

	if got := lostPackets * 100 / txPackets; got != lossTolerancePct {
		t.Errorf("Packet loss percentage for flow: got %v, want %v", got, lossTolerancePct)
	}
}

func verifyECMPLoadBalance(t *testing.T, ate *ondatra.ATEDevice, pc int, expectedLinks int) {
	dut := ondatra.DUT(t, "dut")
	framesTx := gnmi.Get(t, ate.OTG(), gnmi.OTG().Port(ate.Port(t, "port1").ID()).Counters().OutFrames().State())
	expectedPerLinkFms := framesTx / uint64(expectedLinks)
	t.Logf("Total packets %d flow through the %d links and expected per link packets %d", framesTx, expectedLinks, expectedPerLinkFms)
	min := expectedPerLinkFms - (expectedPerLinkFms * lbToleranceFms / 100)
	max := expectedPerLinkFms + (expectedPerLinkFms * lbToleranceFms / 100)

	got := 0
	for i := 3; i <= pc; i++ {
		framesRx := gnmi.Get(t, ate.OTG(), gnmi.OTG().Port(ate.Port(t, "port"+strconv.Itoa(i)).ID()).Counters().InFrames().State())
		if framesRx <= lbToleranceFms {
			t.Logf("Skip: Traffic through port%d interface is %d", i, framesRx)
			continue
		}
		if int64(min) < int64(framesRx) && int64(framesRx) < int64(max) {
			t.Logf("Traffic %d is in expected range: %d - %d, Load balance Test Passed", framesRx, min, max)
			got++
		} else {
			if !deviations.BgpMaxMultipathPathsUnsupported(dut) {
				t.Errorf("Traffic is expected in range %d - %d but got %d. Load balance Test Failed", min, max, framesRx)
			}
		}
	}
	if !deviations.BgpMaxMultipathPathsUnsupported(dut) {
		if got != expectedLinks {
			t.Errorf("invalid number of load balancing interfaces, got: %d want %d", got, expectedLinks)
		}
	}
}
