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
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	otgtelemetry "github.com/openconfig/ondatra/gnmi/otg"
	otg "github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygnmi/ygnmi"
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
	pathId        int32
}

var (
	otgPort1Details = otgPortDetails{
		mac:      "02:00:01:01:01:01",
		routerId: "192.0.2.2",
		pathId:   1,
	}
	otgPort2Details = otgPortDetails{
		mac:      "02:00:02:01:01:01",
		routerId: "192.0.2.6",
		pathId:   1,
	}
)

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
			bgpName := ateList[0] + ".dev.BGP4.peer"
			bgpPeer := bgp4ObjectMap[bgpName]
			ip := ipv4ObjectMap[ateList[0]+".dev.IPv4"]
			firstAdvAddr := strings.Split(ad.prefixesStart.v4, "/")[0]
			firstAdvPrefix, _ := strconv.Atoi(strings.Split(ad.prefixesStart.v4, "/")[1])
			bgp4PeerRoutes := bgpPeer.V4Routes().Add().SetName(bgpName + ".rr4").SetNextHopIpv4Address(ip.Address()).SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)
			bgp4PeerRoutes.Addresses().Add().SetAddress(firstAdvAddr).SetPrefix(int32(firstAdvPrefix)).SetCount(int32(ad.prefixesCount))
			bgp4PeerRoutes.AddPath().SetPathId(otgPort1Details.pathId)

		} else {
			bgpName := ateList[0] + ".dev.BGP6.peer"
			bgpPeer := bgp6ObjectMap[bgpName]
			firstAdvAddr := strings.Split(ad.prefixesStart.v4, "/")[0]
			firstAdvPrefix, _ := strconv.Atoi(strings.Split(ad.prefixesStart.v4, "/")[1])
			bgp4PeerRoutes := bgpPeer.V4Routes().Add().SetName(bgpName + ".rr4")
			bgp4PeerRoutes.Addresses().Add().SetAddress(firstAdvAddr).SetPrefix(int32(firstAdvPrefix)).SetCount(int32(ad.prefixesCount))
			bgp4PeerRoutes.AddPath().SetPathId(otgPort1Details.pathId)
		}
	}
	if ad.prefixesStart.v6 != "" {
		bgp6Name := ateList[0] + ".dev.BGP6.peer"
		bgp6Peer := bgp6ObjectMap[bgp6Name]
		firstAdvAddr := strings.Split(ad.prefixesStart.v6, "/")[0]
		firstAdvPrefix, _ := strconv.Atoi(strings.Split(ad.prefixesStart.v6, "/")[1])
		bgp6PeerRoutes := bgp6Peer.V6Routes().Add().SetName(bgp6Name + ".rr6")
		bgp6PeerRoutes.Addresses().Add().SetAddress(firstAdvAddr).SetPrefix(int32(firstAdvPrefix)).SetCount(int32(ad.prefixesCount))
		bgp6PeerRoutes.AddPath().SetPathId(otgPort1Details.pathId)
	}

	t.Logf("Pushing config to ATE and starting protocols...")
	otg.PushConfig(t, config)
	otg.StartProtocols(t)
	return config

}

type dutData struct {
	bgpOC *oc.NetworkInstance_Protocol_Bgp
}

func (d *dutData) Configure(t *testing.T, dut *ondatra.DUTDevice) {
	for _, a := range []attrs.Attributes{dutPort1, dutPort2} {
		ocName := dut.Port(t, a.Name).Name()
		gnmi.Replace(t, dut, gnmi.OC().Interface(ocName).Config(), a.NewOCInterface(ocName))
	}
	dutBGP := gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).
		Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	gnmi.Replace(t, dut, dutBGP.Config(), d.bgpOC)
}

func (d *dutData) AwaitBGPEstablished(t *testing.T, dut *ondatra.DUTDevice) {
	for neighbor := range d.bgpOC.Neighbor {
		gnmi.Await(t, dut, gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).
			Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").
			Bgp().
			Neighbor(neighbor).
			SessionState().State(), time.Second*15, oc.Bgp_Neighbor_SessionState_ESTABLISHED)
	}
	t.Log("BGP sessions established")
}

func verifyOTGBGPTelemetry(t *testing.T, otg *otg.OTG, c gosnappi.Config, state string) {
	for _, d := range c.Devices().Items() {
		for _, ip := range d.Bgp().Ipv4Interfaces().Items() {
			for _, configPeer := range ip.Peers().Items() {
				nbrPath := gnmi.OTG().BgpPeer(configPeer.Name())
				_, ok := gnmi.Watch(t, otg, nbrPath.SessionState().State(), time.Minute, func(val *ygnmi.Value[otgtelemetry.E_BgpPeer_SessionState]) bool {
					currState, ok := val.Val()
					return ok && currState.String() == state
				}).Await(t)
				if !ok {
					fptest.LogQuery(t, "BGP reported state", nbrPath.State(), gnmi.Get(t, otg, nbrPath.State()))
					t.Errorf("No BGP neighbor formed for peer %s", configPeer.Name())
				}
			}
		}
		for _, ip := range d.Bgp().Ipv6Interfaces().Items() {
			for _, configPeer := range ip.Peers().Items() {
				nbrPath := gnmi.OTG().BgpPeer(configPeer.Name())
				_, ok := gnmi.Watch(t, otg, nbrPath.SessionState().State(), time.Minute, func(val *ygnmi.Value[otgtelemetry.E_BgpPeer_SessionState]) bool {
					currState, ok := val.Val()
					return ok && currState.String() == state
				}).Await(t)
				if !ok {
					fptest.LogQuery(t, "BGP reported state", nbrPath.State(), gnmi.Get(t, otg, nbrPath.State()))
					t.Errorf("No BGP neighbor formed for peer %s", configPeer.Name())
				}
			}
		}
	}
}

type OTGBGPPrefix struct {
	PeerName     string
	Address      string
	PrefixLength uint32
}

// TODO: Use this function below after is fixed in OTG https://github.com/open-traffic-generator/ixia-c-gnmi-server/issues/29

// func verifyOTGBGP4Prefix(t *testing.T, otg *otg.OTG, config gosnappi.Config, expectedOTGBGPPrefix OTGBGPPrefix) {
// 	lastValue, ok := otg.Telemetry().BgpPeer(expectedOTGBGPPrefix.PeerName).UnicastIpv4PrefixAny().Watch(
// 		t,
// 		10*time.Second,
// 		func(bgpPrefix *otgtelemetry.QualifiedBgpPeer_UnicastIpv4Prefix) bool {
// 			if !bgpPrefix.IsPresent() {
// 				t.Log("Any peer not present")
// 				return false
// 			}
// 			val := bgpPrefix.Val(t)
// 			addr, plen := val.GetAddress(), val.GetPrefixLength()
// 			t.Logf("Any peer got: %s/%d", addr, plen)
// 			return addr == expectedOTGBGPPrefix.Address && plen == expectedOTGBGPPrefix.PrefixLength
// 		}).Await(t)

// 	if !ok {
// 		t.Logf("Last value was: %v", lastValue)
// 		actPrefixes := otg.Telemetry().BgpPeer(expectedOTGBGPPrefix.PeerName).UnicastIpv4PrefixAny().Get(t)
// 		for _, actPrefix := range actPrefixes {
// 			t.Logf("Peer: %v, Address: %v, Prefix Length: %v", expectedOTGBGPPrefix.PeerName, actPrefix.GetAddress(), actPrefix.GetPrefixLength())
// 		}
// 		t.Errorf("Given BGP IPv4 Prefix is not formed %v", expectedOTGBGPPrefix)
// 	}
// }

func checkOTGBGP4Prefix(t *testing.T, otg *otg.OTG, config gosnappi.Config, expectedOTGBGPPrefix OTGBGPPrefix) bool {
	bgpPrefixes := gnmi.GetAll(t, otg, gnmi.OTG().BgpPeer(expectedOTGBGPPrefix.PeerName).UnicastIpv4PrefixAny().State())
	found := false
	for _, bgpPrefix := range bgpPrefixes {
		if bgpPrefix.Address != nil && bgpPrefix.GetAddress() == expectedOTGBGPPrefix.Address &&
			bgpPrefix.PrefixLength != nil && bgpPrefix.GetPrefixLength() == expectedOTGBGPPrefix.PrefixLength {
			found = true
			break
		}
	}
	return found
}

func checkOTGBGP6Prefix(t *testing.T, otg *otg.OTG, config gosnappi.Config, expectedOTGBGPPrefix OTGBGPPrefix) bool {
	bgpPrefixes := gnmi.GetAll(t, otg, gnmi.OTG().BgpPeer(expectedOTGBGPPrefix.PeerName).UnicastIpv6PrefixAny().State())
	found := false
	for _, bgpPrefix := range bgpPrefixes {
		if bgpPrefix.Address != nil && bgpPrefix.GetAddress() == expectedOTGBGPPrefix.Address &&
			bgpPrefix.PrefixLength != nil && bgpPrefix.GetPrefixLength() == expectedOTGBGPPrefix.PrefixLength {
			found = true
			break
		}
	}
	return found
}

func waitFor(fn func() bool, t testing.TB) {
	start := time.Now()
	for {
		done := fn()
		if done {
			t.Logf("Expected BGP Prefix received")
			break
		}
		if time.Since(start) > time.Minute {
			t.Errorf("Timeout while waiting for expected stats...")
			break
		}
		time.Sleep(500 * time.Millisecond)
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
		desc:     "propagate IPv4 over IPv4",
		fullDesc: "Advertise prefixes from ATE port1, observe received prefixes at ATE port2",
		dut: dutData{&oc.NetworkInstance_Protocol_Bgp{
			Global: &oc.NetworkInstance_Protocol_Bgp_Global{
				As:       ygot.Uint32(dutAS),
				RouterId: ygot.String(dutPort2.IPv4),
			},
			Neighbor: map[string]*oc.NetworkInstance_Protocol_Bgp_Neighbor{
				"192.0.2.2": {
					PeerAs:          ygot.Uint32(ateAS1),
					NeighborAddress: ygot.String("192.0.2.2"),
					PeerGroup:       ygot.String("BGP-PEER-GROUP1"),
				},
				"192.0.2.6": {
					PeerAs:          ygot.Uint32(ateAS2),
					NeighborAddress: ygot.String("192.0.2.6"),
					PeerGroup:       ygot.String("BGP-PEER-GROUP2"),
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
		desc:     "propagate IPv6 over IPv6",
		fullDesc: "Advertise IPv6 prefixes from ATE port1, observe received prefixes at ATE port2",
		dut: dutData{&oc.NetworkInstance_Protocol_Bgp{
			Global: &oc.NetworkInstance_Protocol_Bgp_Global{
				As:       ygot.Uint32(dutAS),
				RouterId: ygot.String(dutPort2.IPv4),
			},
			Neighbor: map[string]*oc.NetworkInstance_Protocol_Bgp_Neighbor{
				"2001:db8::2": {
					PeerAs:          ygot.Uint32(ateAS1),
					PeerGroup:       ygot.String("BGP-PEER-GROUP1"),
					NeighborAddress: ygot.String("2001:db8::2"),
					AfiSafi: map[oc.E_BgpTypes_AFI_SAFI_TYPE]*oc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi{
						oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST: {
							AfiSafiName: oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST,
							Enabled:     ygot.Bool(true),
						},
					},
				},
				"2001:db8::6": {
					PeerAs:          ygot.Uint32(ateAS2),
					PeerGroup:       ygot.String("BGP-PEER-GROUP2"),
					NeighborAddress: ygot.String("2001:db8::6"),
					AfiSafi: map[oc.E_BgpTypes_AFI_SAFI_TYPE]*oc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi{
						oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST: {
							AfiSafiName: oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST,
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
		skipReason: "TODO: RFC5549 needs to be enabled explicitly and OpenConfig does not currently provide a signal.",
		fullDesc:   "IPv4 routes with an IPv6 next-hop when negotiating RFC5549 - validating that routes are accepted and advertised with the specified values.",
		dut: dutData{&oc.NetworkInstance_Protocol_Bgp{
			Global: &oc.NetworkInstance_Protocol_Bgp_Global{
				As: ygot.Uint32(dutAS),
			},
			Neighbor: map[string]*oc.NetworkInstance_Protocol_Bgp_Neighbor{
				"2001:db8::2": {
					PeerAs:          ygot.Uint32(ateAS1),
					PeerGroup:       ygot.String("BGP-PEER-GROUP1"),
					NeighborAddress: ygot.String("2001:db8::2"),
					AfiSafi: map[oc.E_BgpTypes_AFI_SAFI_TYPE]*oc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi{
						oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST: {
							AfiSafiName: oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST,
							Enabled:     ygot.Bool(true),
						},
						oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST: {
							AfiSafiName: oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST,
							Enabled:     ygot.Bool(true),
						},
					},
				},
				"192.0.2.6": {
					PeerAs:          ygot.Uint32(ateAS2),
					PeerGroup:       ygot.String("BGP-PEER-GROUP2"),
					NeighborAddress: ygot.String("192.0.2.6"),
					AfiSafi: map[oc.E_BgpTypes_AFI_SAFI_TYPE]*oc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi{
						oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST: {
							AfiSafiName: oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST,
							Enabled:     ygot.Bool(true),
						},
					},
				},
			},
		}},
		ate: ateData{
			Port1:         ip{v6: "2001:db8::2/126"},
			Port1Neighbor: dutPort1.IPv6,
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
	}}
	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			t.Log(tc.fullDesc)

			if tc.skipReason != "" {
				t.Skip(tc.skipReason)
			}

			dut := ondatra.DUT(t, "dut")
			t.Log("Configure Network Instance")
			dutConfNIPath := gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance)
			gnmi.Replace(t, dut, dutConfNIPath.Type().Config(), oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)

			tc.dut.Configure(t, dut)

			ate := ondatra.ATE(t, "ate")

			otg := ate.OTG()
			ateList := []string{"port1", "port2"}
			otgConfig := tc.ate.ConfigureOTG(t, otg, ateList)

			t.Logf("Verify DUT BGP sessions up")
			tc.dut.AwaitBGPEstablished(t, dut)
			t.Logf("Verify OTG BGP sessions up")
			verifyOTGBGPTelemetry(t, otg, otgConfig, "ESTABLISHED")

			for _, prefix := range tc.wantPrefixes {
				var expectedOTGBGPPrefix OTGBGPPrefix
				if prefix.v4 != "" {
					t.Logf("Checking for BGP Prefix %v", prefix.v4)
					addr := strings.Split(prefix.v4, "/")[0]
					prefixLen, _ := strconv.Atoi(strings.Split(prefix.v4, "/")[1])
					if tc.ate.Port2.v4 != "" {
						expectedOTGBGPPrefix = OTGBGPPrefix{PeerName: "port2.dev.BGP4.peer", Address: addr, PrefixLength: uint32(prefixLen)}
					} else {
						expectedOTGBGPPrefix = OTGBGPPrefix{PeerName: "port2.dev.BGP6.peer", Address: addr, PrefixLength: uint32(prefixLen)}
					}
					waitFor(func() bool { return checkOTGBGP4Prefix(t, otg, otgConfig, expectedOTGBGPPrefix) }, t)
					// verifyOTGBGP4Prefix(t, otg, otgConfig, expectedOTGBGPPrefix)
				}
				if prefix.v6 != "" {
					t.Logf("Checking for BGP Prefix %v", prefix.v6)
					addr := strings.Split(prefix.v6, "/")[0]
					prefixLen, _ := strconv.Atoi(strings.Split(prefix.v6, "/")[1])
					expectedOTGBGPPrefix = OTGBGPPrefix{PeerName: "port2.dev.BGP6.peer", Address: addr, PrefixLength: uint32(prefixLen)}
					waitFor(func() bool { return checkOTGBGP6Prefix(t, otg, otgConfig, expectedOTGBGPPrefix) }, t)
				}
			}
		})
	}
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}
