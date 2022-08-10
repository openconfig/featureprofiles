// Copyright 2022 Google LLC
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

// Package route_propagation_test implements RT-1.3: BGP Route Propagation.
package route_propagation_test

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	otg "github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ondatra/telemetry"
	otgtelemetry "github.com/openconfig/ondatra/telemetry/otg"
	"github.com/openconfig/ygot/ygot"
)

// This test topology connects the DUT and ATE over two ports.
//
// We are testing BGP route propagation across the DUT, so we are using the ATE to simulate two
// neighbors: AS 65536 on ATE port1, and AS 65538 on ATE port2, which both neighbor the DUT AS
// 65537 (both port1 and port2).
var (
	// TODO: The DUT IP should be table driven as well, but assigning only an IPv6
	// address seems to prevent Ixia from bringing up the connection.
	dutPort1 = attrs.Attributes{
		Name:    "port1",
		Desc:    "To ATE",
		IPv4:    "192.0.2.1",
		IPv4Len: 30,
		IPv6:    "2001:db8::1",
		IPv6Len: 126,
	}
	dutPort2 = attrs.Attributes{
		Name:    "port2",
		Desc:    "To ATE",
		IPv4:    "192.0.2.5",
		IPv4Len: 30,
		IPv6:    "2001:db8::5",
		IPv6Len: 126,
	}
	dutAS = uint32(65537)

	ateAS1 = uint32(65536)
	ateAS2 = uint32(65538)
)

type otgPortDetails struct {
	mac, routerId string
}

var otgPort1Details otgPortDetails = otgPortDetails{
	mac:      "02:00:01:01:01:01",
	routerId: "1.1.1.1"}
var otgPort2Details otgPortDetails = otgPortDetails{
	mac:      "02:00:02:01:01:01",
	routerId: "2.2.2.2"}

type ip struct {
	v4, v6 string
}

type ateData struct {
	Port1         ip
	Port2         ip
	Port1Neighbor string
	Port2Neighbor string
	prefixesStart ip
	prefixesCount uint32
}

// configureOTG configures the interfaces and BGP protocols on an OTG, including advertising some
// (faked) networks over BGP.
func (ad *ateData) ConfigureOTG(t *testing.T, otg *otg.OTG, ateList []string) gosnappi.Config {

	config := otg.NewConfig(t)
	bgp4ObjectMap := make(map[string]gosnappi.BgpV4Peer)
	bgp6ObjectMap := make(map[string]gosnappi.BgpV6Peer)
	ipv4ObjectMap := make(map[string]gosnappi.DeviceIpv4)
	ipv6ObjectMap := make(map[string]gosnappi.DeviceIpv6)
	ateIndex := 0
	for _, v := range []struct {
		iface    otgPortDetails
		ip       ip
		neighbor string
		as       uint32
	}{
		{otgPort1Details, ad.Port1, ad.Port1Neighbor, ateAS1},
		{otgPort2Details, ad.Port2, ad.Port2Neighbor, ateAS2},
	} {

		devName := ateList[ateIndex] + ".dev"
		port := config.Ports().Add().SetName(ateList[ateIndex])
		dev := config.Devices().Add().SetName(devName)
		ateIndex++

		eth := dev.Ethernets().Add().SetName(devName + ".Eth")
		eth.SetPortName(port.Name()).SetMac(v.iface.mac)
		bgp := dev.Bgp().SetRouterId(v.iface.routerId)
		if v.ip.v4 != "" {
			address := strings.Split(v.ip.v4, "/")[0]
			prefixInt4, _ := strconv.Atoi(strings.Split(v.ip.v4, "/")[1])
			ipv4 := eth.Ipv4Addresses().Add().SetName(devName + ".IPv4").SetAddress(address).SetGateway(v.neighbor).SetPrefix(int32(prefixInt4))
			bgp4Name := devName + ".BGP4.peer"
			bgp4Peer := bgp.Ipv4Interfaces().Add().SetIpv4Name(ipv4.Name()).Peers().Add().SetName(bgp4Name).SetPeerAddress(ipv4.Gateway()).SetAsNumber(int32(v.as)).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)

			bgp4Peer.Capability().SetIpv4UnicastAddPath(true).SetIpv6UnicastAddPath(true)
			bgp4Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)

			bgp4ObjectMap[bgp4Name] = bgp4Peer
			ipv4ObjectMap[devName+".IPv4"] = ipv4
		}
		if v.ip.v6 != "" {
			address := strings.Split(v.ip.v6, "/")[0]
			prefixInt6, _ := strconv.Atoi(strings.Split(v.ip.v6, "/")[1])
			ipv6 := eth.Ipv6Addresses().Add().SetName(devName + ".IPv6").SetAddress(address).SetGateway(v.neighbor).SetPrefix(int32(prefixInt6))
			bgp6Name := devName + ".BGP6.peer"
			bgp6Peer := bgp.Ipv6Interfaces().Add().SetIpv6Name(ipv6.Name()).Peers().Add().SetName(bgp6Name).SetPeerAddress(ipv6.Gateway()).SetAsNumber(int32(v.as)).SetAsType(gosnappi.BgpV6PeerAsType.EBGP)

			bgp6Peer.Capability().SetIpv4UnicastAddPath(true).SetIpv6UnicastAddPath(true).SetExtendedNextHopEncoding(true)
			bgp6Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)

			bgp6ObjectMap[bgp6Name] = bgp6Peer
			ipv6ObjectMap[devName+".IPv6"] = ipv6
		}
	}
	if ad.prefixesStart.v4 != "" {
		if ad.Port1.v4 != "" {
			bgpName := ateList[0] + ".dev.bgp4.peer"
			bgpPeer := bgp4ObjectMap[bgpName]
			ip := ipv4ObjectMap[ateList[0]+".dev.ipv4"]
			pathIds := []int{1, 2, 3, 4}
			for pathId := range pathIds {
				firstAdvAddr := strings.Split(ad.prefixesStart.v4, "/")[0]
				firstAdvPrefix, _ := strconv.Atoi(strings.Split(ad.prefixesStart.v4, "/")[1])
				bgp4PeerRoutes := bgpPeer.V4Routes().Add().SetName(bgpName + ".rr4." + strconv.Itoa(pathId)).SetNextHopIpv4Address(ip.Address()).SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)
				bgp4PeerRoutes.Addresses().Add().SetAddress(firstAdvAddr).SetPrefix(int32(firstAdvPrefix)).SetCount(int32(ad.prefixesCount))
				bgp4PeerRoutes.AddPath().SetPathId(int32(pathId))
			}
		} else {
			bgpName := ateList[0] + ".dev.BGP6.peer"
			bgpPeer := bgp6ObjectMap[bgpName]
			pathIds := []int{1, 2, 3, 4}
			for pathId := range pathIds {
				firstAdvAddr := strings.Split(ad.prefixesStart.v4, "/")[0]
				firstAdvPrefix, _ := strconv.Atoi(strings.Split(ad.prefixesStart.v4, "/")[1])
				bgp4PeerRoutes := bgpPeer.V4Routes().Add().SetName(bgpName + ".rr4." + strconv.Itoa(pathId))
				bgp4PeerRoutes.Addresses().Add().SetAddress(firstAdvAddr).SetPrefix(int32(firstAdvPrefix)).SetCount(int32(ad.prefixesCount))
				bgp4PeerRoutes.AddPath().SetPathId(int32(pathId))
			}
		}
	}
	if ad.prefixesStart.v6 != "" {
		bgp6Name := ateList[0] + "dev.BGP6.peer"
		bgp6Peer := bgp6ObjectMap[bgp6Name]
		pathIds := []int{1, 2, 3, 4}
		for pathId := range pathIds {
			firstAdvAddr := strings.Split(ad.prefixesStart.v6, "/")[0]
			firstAdvPrefix, _ := strconv.Atoi(strings.Split(ad.prefixesStart.v6, "/")[1])
			bgp6PeerRoutes := bgp6Peer.V6Routes().Add().SetName(bgp6Name + ".rr6." + strconv.Itoa(pathId))
			bgp6PeerRoutes.Addresses().Add().SetAddress(firstAdvAddr).SetPrefix(int32(firstAdvPrefix)).SetCount(int32(ad.prefixesCount))
			bgp6PeerRoutes.AddPath().SetPathId(int32(pathId))
		}
	}

	t.Logf("Pushing config to ATE and starting protocols...")
	otg.PushConfig(t, config)
	otg.StartProtocols(t)
	return config

}

type dutData struct {
	bgpOC *telemetry.NetworkInstance_Protocol_Bgp
}

func (ad *ateData) configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	ate := ondatra.ATE(t, "ate")

	t.Logf("Configuring DUT...")
	if ad.Port1.v4 != "" {
		if ate.Port(t, "port1").Name() == "eth1" {
			dut.Config().New().WithAristaFile("set_arista_ipv4.config").Push(t)
		} else {
			dut.Config().New().WithAristaFile("set_arista_ipv4_alternate.config").Push(t)
		}
	} else {
		if ate.Port(t, "port1").Name() == "eth1" {
			dut.Config().New().WithAristaFile("set_arista.config").Push(t)
		} else {
			dut.Config().New().WithAristaFile("set_arista_alternate.config").Push(t)
		}
	}

}

func unsetDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Logf("Resetting DUT...")
	dut.Config().New().WithAristaFile("unset_arista.config").Push(t)
}

func (d *dutData) AwaitBGPEstablished(t *testing.T, dut *ondatra.DUTDevice) {
	for neighbor := range d.bgpOC.Neighbor {
		dut.Telemetry().NetworkInstance("default").
			Protocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").
			Bgp().
			Neighbor(neighbor).
			SessionState().
			Await(t, time.Second*15, telemetry.Bgp_Neighbor_SessionState_ESTABLISHED)
	}
	t.Log("BGP sessions established")
}

func verifyOTGBGPTelemetry(t *testing.T, otg *otg.OTG, c gosnappi.Config, ipType, state string) {
	for _, d := range c.Devices().Items() {
		switch ipType {
		case "IPv4":
			for _, ip := range d.Bgp().Ipv4Interfaces().Items() {
				for _, configPeer := range ip.Peers().Items() {
					nbrPath := otg.Telemetry().BgpPeer(configPeer.Name())
					_, ok := nbrPath.SessionState().Watch(t, time.Minute,
						func(val *otgtelemetry.QualifiedE_BgpPeer_SessionState) bool {
							return val.IsPresent() && val.Val(t).String() == state
						}).Await(t)
					if !ok {
						fptest.LogYgot(t, "BGP reported state", nbrPath, nbrPath.Get(t))
						t.Fatal("No BGP neighbor formed")
					} else {
						t.Logf("BGPv4 sessions for for Peer %v are in Established State !!", configPeer.Name())
					}
				}
			}
		case "IPv6":
			for _, ip := range d.Bgp().Ipv6Interfaces().Items() {
				for _, configPeer := range ip.Peers().Items() {
					nbrPath := otg.Telemetry().BgpPeer(configPeer.Name())
					_, ok := nbrPath.SessionState().Watch(t, time.Minute,
						func(val *otgtelemetry.QualifiedE_BgpPeer_SessionState) bool {
							return val.IsPresent() && val.Val(t).String() == state
						}).Await(t)
					if !ok {
						fptest.LogYgot(t, "BGP reported state", nbrPath, nbrPath.Get(t))
						t.Fatal("No BGP neighbor formed")
					} else {
						t.Logf("BGPv6 sessions for for Peer %v are in Established State !!", configPeer.Name())
					}
				}
			}
		}
	}
}

type OTGBGPPrefix struct {
	Address          string
	PrefixLength     uint32
	Origin           otgtelemetry.E_UnicastIpv4Prefix_Origin
	PathId           uint32
	NextHopV6Address string
	NextHopV4Address string
}

func otgBGPPrefixAsExpected(t *testing.T, otg *otg.OTG, config gosnappi.Config, expectedOTGBGPPrefix map[string][]OTGBGPPrefix) bool {
	for peerName, expectedOTGBGPPrefixes := range expectedOTGBGPPrefix {
		for _, expectedOTGBGPPrefix := range expectedOTGBGPPrefixes {
			bgpPrefixes := otg.Telemetry().BgpPeer(peerName).UnicastIpv4PrefixAny().Get(t)
			found := false
			for _, bgpPrefix := range bgpPrefixes {
				if bgpPrefix.Address != nil && bgpPrefix.GetAddress() == expectedOTGBGPPrefix.Address &&
					bgpPrefix.GetOrigin() == expectedOTGBGPPrefix.Origin &&
					bgpPrefix.PrefixLength != nil && bgpPrefix.GetPrefixLength() == expectedOTGBGPPrefix.PrefixLength &&
					bgpPrefix.PathId != nil && bgpPrefix.GetPathId() == expectedOTGBGPPrefix.PathId {
					found = true
					break
				}
			}

			if found != true {
				return false
			}
		}
	}
	return true
}

type OTGBGPV6Prefix struct {
	Address          string
	PrefixLength     uint32
	Origin           otgtelemetry.E_UnicastIpv6Prefix_Origin
	PathId           uint32
	NextHopV6Address string
}

func otgBGPv6PrefixAsExpected(t *testing.T, otg *otg.OTG, config gosnappi.Config, expectedOTGBGPPrefix map[string][]OTGBGPV6Prefix) bool {
	for peerName, expectedOTGBGPPrefixes := range expectedOTGBGPPrefix {
		for _, expectedOTGBGPPrefix := range expectedOTGBGPPrefixes {
			bgpPrefixes := otg.Telemetry().BgpPeer(peerName).UnicastIpv6PrefixAny().Get(t)
			found := false
			for _, bgpPrefix := range bgpPrefixes {
				if bgpPrefix.Address != nil && bgpPrefix.GetAddress() == expectedOTGBGPPrefix.Address &&
					bgpPrefix.GetOrigin() == expectedOTGBGPPrefix.Origin &&
					bgpPrefix.PrefixLength != nil && bgpPrefix.GetPrefixLength() == expectedOTGBGPPrefix.PrefixLength &&
					bgpPrefix.PathId != nil && bgpPrefix.GetPathId() == expectedOTGBGPPrefix.PathId &&
					bgpPrefix.NextHopIpv6Address != nil && bgpPrefix.GetNextHopIpv6Address() == expectedOTGBGPPrefix.NextHopV6Address {
					found = true
					break
				}
			}
			if found != true {
				return false
			}
		}
	}
	return true
}

func waitFor(fn func() bool, t testing.TB, interval time.Duration, timeout time.Duration) {
	start := time.Now()
	for {
		done := fn()
		if done {
			t.Logf("Expected BGP Prefix received...")
			break
		}
		if time.Since(start) > timeout {
			t.Fatal("Timeout while waiting for expected stats...")
			break
		}
		time.Sleep(interval)
	}
}

func TestBGP(t *testing.T) {
	tests := []struct {
		desc, fullDesc string
		skipReason     string
		dut            dutData
		ate            ateData
		wantPrefixes   []ip
	}{{
		desc:       "propagate IPv4 over IPv4",
		skipReason: "",
		fullDesc:   "Advertise prefixes from ATE port1, observe received prefixes at ATE port2",
		dut: dutData{&telemetry.NetworkInstance_Protocol_Bgp{
			Global: &telemetry.NetworkInstance_Protocol_Bgp_Global{
				As: ygot.Uint32(dutAS),
			},
			Neighbor: map[string]*telemetry.NetworkInstance_Protocol_Bgp_Neighbor{
				"192.0.2.2": {
					PeerAs:          ygot.Uint32(ateAS1),
					NeighborAddress: ygot.String("192.0.2.2"),
				},
				"192.0.2.6": {
					PeerAs:          ygot.Uint32(ateAS2),
					NeighborAddress: ygot.String("192.0.2.6"),
				},
			},
		}},
		ate: ateData{
			Port1:         ip{v4: "192.0.2.2/30"},
			Port1Neighbor: dutPort1.IPv4,
			Port2:         ip{v4: "192.0.2.6/30"},
			Port2Neighbor: dutPort2.IPv4,
			prefixesStart: ip{v4: "198.51.100.0/32"},
			prefixesCount: 4,
		},
		wantPrefixes: []ip{
			{v4: "198.51.100.0/32"},
			{v4: "198.51.100.1/32"},
			{v4: "198.51.100.2/32"},
			{v4: "198.51.100.3/32"},
		},
	}, {
		desc:       "propagate IPv6 over IPv6",
		skipReason: "",
		fullDesc:   "Advertise IPv6 prefixes from ATE port1, observe received prefixes at ATE port2",
		dut: dutData{&telemetry.NetworkInstance_Protocol_Bgp{
			Global: &telemetry.NetworkInstance_Protocol_Bgp_Global{
				As: ygot.Uint32(dutAS),
			},
			Neighbor: map[string]*telemetry.NetworkInstance_Protocol_Bgp_Neighbor{
				"2001:db8::2": {
					PeerAs:          ygot.Uint32(ateAS1),
					NeighborAddress: ygot.String("2001:db8::2"),
					AfiSafi: map[telemetry.E_BgpTypes_AFI_SAFI_TYPE]*telemetry.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi{
						telemetry.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST: {
							AfiSafiName: telemetry.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST,
							Enabled:     ygot.Bool(true),
						},
					},
				},
				"2001:db8::6": {
					PeerAs:          ygot.Uint32(ateAS2),
					NeighborAddress: ygot.String("2001:db8::6"),
					AfiSafi: map[telemetry.E_BgpTypes_AFI_SAFI_TYPE]*telemetry.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi{
						telemetry.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST: {
							AfiSafiName: telemetry.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST,
							Enabled:     ygot.Bool(true),
						},
					},
				},
			},
		}},
		ate: ateData{
			Port1:         ip{v6: "2001:db8::2/126"},
			Port1Neighbor: dutPort1.IPv6,
			Port2:         ip{v6: "2001:db8::6/126"},
			Port2Neighbor: dutPort2.IPv6,
			prefixesStart: ip{v6: "2001:db8:1::1/128"},
			prefixesCount: 4,
		},
		wantPrefixes: []ip{
			{v6: "2001:db8:1::1/128"},
			{v6: "2001:db8:1::2/128"},
			{v6: "2001:db8:1::3/128"},
			{v6: "2001:db8:1::4/128"},
		},
	}, {
		desc:       "propagate IPv4 over IPv6",
		skipReason: "",
		fullDesc:   "IPv4 routes with an IPv6 next-hop when negotiating RFC5549 - validating that routes are accepted and advertised with the specified values.",
		dut: dutData{&telemetry.NetworkInstance_Protocol_Bgp{
			Global: &telemetry.NetworkInstance_Protocol_Bgp_Global{
				As: ygot.Uint32(dutAS),
			},
			Neighbor: map[string]*telemetry.NetworkInstance_Protocol_Bgp_Neighbor{
				"2001:db8::2": {
					PeerAs:          ygot.Uint32(ateAS1),
					NeighborAddress: ygot.String("2001:db8::2"),
					AfiSafi: map[telemetry.E_BgpTypes_AFI_SAFI_TYPE]*telemetry.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi{
						telemetry.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST: {
							AfiSafiName: telemetry.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST,
							Enabled:     ygot.Bool(true),
						},
					},
				},
				"2001:db8::6": {
					PeerAs:          ygot.Uint32(ateAS2),
					NeighborAddress: ygot.String("2001:db8::6"),
					AfiSafi: map[telemetry.E_BgpTypes_AFI_SAFI_TYPE]*telemetry.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi{
						telemetry.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST: {
							AfiSafiName: telemetry.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST,
							Enabled:     ygot.Bool(true),
						},
					},
				},
			},
		}},
		ate: ateData{
			Port1:         ip{v6: "2001:db8::2/126"},
			Port1Neighbor: dutPort1.IPv6,
			Port2:         ip{v6: "2001:db8::6/126"},
			Port2Neighbor: dutPort2.IPv6,
			prefixesStart: ip{v4: "198.51.100.0/32"},
			prefixesCount: 4,
		},
		wantPrefixes: []ip{
			{v4: "198.51.100.0/32"},
			{v4: "198.51.100.1/32"},
			{v4: "198.51.100.2/32"},
			{v4: "198.51.100.3/32"},
		},
	}}
	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			t.Log(tc.fullDesc)

			if tc.skipReason != "" {
				t.Skip(tc.skipReason)
			}

			dut := ondatra.DUT(t, "dut")
			tc.ate.configureDUT(t, dut)
			defer unsetDUT(t, dut)

			ate := ondatra.ATE(t, "ate")

			otg := ate.OTG()
			ateList := []string{"port1", "port2"}
			otgConfig := tc.ate.ConfigureOTG(t, otg, ateList)

			t.Logf("Verify BGP sessions up")
			verifyOTGBGPTelemetry(t, otg, otgConfig, "IPv4", "ESTABLISHED")
			verifyOTGBGPTelemetry(t, otg, otgConfig, "IPv6", "ESTABLISHED")

			time.Sleep(time.Second * 2)

			if tc.ate.prefixesStart.v6 != "" {
				t.Logf("IPv6 Prefixes Over IPv6 BGP Peers.. ")
				expectedOTGBGPPrefix := map[string][]OTGBGPV6Prefix{
					"port2.dev.bgp6.peer": {
						{Address: "2001:db8:1::1", PrefixLength: 128, Origin: otgtelemetry.UnicastIpv6Prefix_Origin_IGP, PathId: 1, NextHopV6Address: "2001:db8::5"},
						{Address: "2001:db8:1::1", PrefixLength: 128, Origin: otgtelemetry.UnicastIpv6Prefix_Origin_IGP, PathId: 2, NextHopV6Address: "2001:db8::5"},
						{Address: "2001:db8:1::1", PrefixLength: 128, Origin: otgtelemetry.UnicastIpv6Prefix_Origin_IGP, PathId: 3, NextHopV6Address: "2001:db8::5"},
						{Address: "2001:db8:1::1", PrefixLength: 128, Origin: otgtelemetry.UnicastIpv6Prefix_Origin_IGP, PathId: 4, NextHopV6Address: "2001:db8::5"},
					},
				}

				otgutils.LogBGPv6Metrics(t, otg, otgConfig)
				otgutils.LogBGPStates(t, otg, otgConfig)
				waitFor(
					func() bool { return otgBGPv6PrefixAsExpected(t, otg, otgConfig, expectedOTGBGPPrefix) },
					t,
					500*time.Millisecond,
					time.Minute,
				)

			}

			if tc.ate.prefixesStart.v4 != "" {
				if tc.ate.Port1.v4 != "" {
					t.Logf("IPv4 Prefixes over IPv4 BGP Peers.. ")

					expectedOTGBGPPrefix := map[string][]OTGBGPPrefix{
						"port2.dev.bgp4.peer": {
							{Address: "198.51.100.1", PrefixLength: 32, Origin: otgtelemetry.UnicastIpv4Prefix_Origin_IGP, PathId: 1, NextHopV4Address: "192.0.2.5", NextHopV6Address: ""},
							{Address: "198.51.100.1", PrefixLength: 32, Origin: otgtelemetry.UnicastIpv4Prefix_Origin_IGP, PathId: 2, NextHopV4Address: "192.0.2.5", NextHopV6Address: ""},
							{Address: "198.51.100.1", PrefixLength: 32, Origin: otgtelemetry.UnicastIpv4Prefix_Origin_IGP, PathId: 3, NextHopV4Address: "192.0.2.5", NextHopV6Address: ""},
							{Address: "198.51.100.1", PrefixLength: 32, Origin: otgtelemetry.UnicastIpv4Prefix_Origin_IGP, PathId: 4, NextHopV4Address: "192.0.2.5", NextHopV6Address: ""},
						},
					}

					otgutils.LogBGPv4Metrics(t, otg, otgConfig)
					otgutils.LogBGPStates(t, otg, otgConfig)

					waitFor(
						func() bool { return otgBGPPrefixAsExpected(t, otg, otgConfig, expectedOTGBGPPrefix) },
						t,
						500*time.Millisecond,
						time.Minute,
					)

				} else {
					t.Logf("IPv4 Prefixes Over IPv6 BGP Peers.. ")
					expectedOTGBGPPrefix := map[string][]OTGBGPPrefix{
						"port2.dev.bgp6.peer": {
							{Address: "198.51.100.1", PrefixLength: 32, Origin: otgtelemetry.UnicastIpv4Prefix_Origin_IGP, PathId: 1, NextHopV4Address: "", NextHopV6Address: "2001:db8::5"},
							{Address: "198.51.100.1", PrefixLength: 32, Origin: otgtelemetry.UnicastIpv4Prefix_Origin_IGP, PathId: 2, NextHopV4Address: "", NextHopV6Address: "2001:db8::5"},
							{Address: "198.51.100.1", PrefixLength: 32, Origin: otgtelemetry.UnicastIpv4Prefix_Origin_IGP, PathId: 3, NextHopV4Address: "", NextHopV6Address: "2001:db8::5"},
							{Address: "198.51.100.1", PrefixLength: 32, Origin: otgtelemetry.UnicastIpv4Prefix_Origin_IGP, PathId: 4, NextHopV4Address: "", NextHopV6Address: "2001:db8::5"},
						},
					}

					otgutils.LogBGPv6Metrics(t, otg, otgConfig)
					otgutils.LogBGPStates(t, otg, otgConfig)
					waitFor(
						func() bool { return otgBGPPrefixAsExpected(t, otg, otgConfig, expectedOTGBGPPrefix) },
						t,
						500*time.Millisecond,
						time.Minute,
					)
				}
			}
		})
	}
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}
