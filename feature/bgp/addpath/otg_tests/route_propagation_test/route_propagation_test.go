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
	"github.com/openconfig/featureprofiles/internal/attrs/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest/fptest"
	"github.com/openconfig/ondatra/gnmi/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc/oc"
	otgtelemetry "github.com/openconfig/ondatra/gnmi/otg/otg"
	otg "github.com/openconfig/ondatra/otg/otg"
	"github.com/openconfig/ygnmi/ygnmi/ygnmi"
)

// This test topology connects the DUT and ATE over two ports.
//
// We are testing BGP route propagation across the DUT, so we are using the ATE to simulate two
// neighbors: AS 65511 on ATE port1, and AS 65512 on ATE port2, which both neighbor the DUT AS
// 65501 (both port1 and port2).

type otgPortDetails struct {
	mac, routerID string
	pathID        int32
}

var (
	otgPort1Details = otgPortDetails{
		mac:      "02:00:01:01:01:01",
		routerID: "192.0.2.2",
		pathID:   1,
	}
	otgPort2Details = otgPortDetails{
		mac:      "02:00:02:01:01:01",
		routerID: "192.0.2.6",
		pathID:   1,
	}
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
	bs            *cfgplugins.BGPSession
}

func (ad *ateData) ConfigureOTG(t *testing.T, ateList []string) {

	bgp4ObjectMap := make(map[string]gosnappi.BgpV4Peer)
	bgp6ObjectMap := make(map[string]gosnappi.BgpV6Peer)
	ipv4ObjectMap := make(map[string]gosnappi.DeviceIpv4)
	ipv6ObjectMap := make(map[string]gosnappi.DeviceIpv6)

	for idx, v := range []struct {
		iface    otgPortDetails
		ip       ip
		neighbor string
		as       uint32
	}{
		{otgPort1Details, ad.Port1, ad.Port1Neighbor, cfgplugins.AteAS1},
		{otgPort2Details, ad.Port2, ad.Port1Neighbor, cfgplugins.AteAS2},
	} {
		devName := ateList[idx]
		bgp4Name := devName + ".BGP4.peer"
		bgp6Name := devName + ".BGP6.peer"
		dev := ad.bs.ATETop.Devices().Items()[idx]
		eths := dev.Ethernets()

		if v.ip.v4 != "" {
			bgpPeers := dev.Bgp().Ipv4Interfaces().Items()[0].Peers()
			bgpPeers.Items()[0].Capability().SetIpv4UnicastAddPath(true).SetIpv6UnicastAddPath(true)
			bgpPeers.Items()[0].LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)
			bgp4ObjectMap[bgp4Name] = bgpPeers.Items()[0]
			ipv4s := eths.Items()[0].Ipv4Addresses()
			ipv4ObjectMap[devName+".IPv4"] = ipv4s.Items()[0]
		}
		if v.ip.v6 != "" {
			bgpPeers := dev.Bgp().Ipv6Interfaces().Items()[0].Peers()
			bgpPeers.Items()[0].Capability().SetIpv4UnicastAddPath(true).SetIpv6UnicastAddPath(true).SetExtendedNextHopEncoding(true)
			bgpPeers.Items()[0].LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)
			bgp6ObjectMap[bgp6Name] = bgpPeers.Items()[0]
			ipv6s := eths.Items()[0].Ipv6Addresses()
			ipv6ObjectMap[devName+".IPv6"] = ipv6s.Items()[0]
		}
	}
	if ad.prefixesStart.v4 != "" {
		if ad.Port1.v4 != "" {
			bgpName := ateList[0] + ".BGP4.peer"
			bgpPeer := bgp4ObjectMap[bgpName]
			ip := ipv4ObjectMap[ateList[0]+".IPv4"]
			firstAdvAddr := strings.Split(ad.prefixesStart.v4, "/")[0]
			firstAdvPrefix, _ := strconv.Atoi(strings.Split(ad.prefixesStart.v4, "/")[1])
			bgp4PeerRoutes := bgpPeer.V4Routes().Add().SetName(bgpName + ".rr4").SetNextHopIpv4Address(ip.Address()).SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)
			bgp4PeerRoutes.Addresses().Add().SetAddress(firstAdvAddr).SetPrefix(uint32(firstAdvPrefix)).SetCount(uint32(ad.prefixesCount))
			bgp4PeerRoutes.AddPath().SetPathId(uint32(otgPort1Details.pathID))

		} else {
			bgpName := ateList[0] + ".BGP6.peer"
			bgpPeer := bgp6ObjectMap[bgpName]
			firstAdvAddr := strings.Split(ad.prefixesStart.v4, "/")[0]
			firstAdvPrefix, _ := strconv.Atoi(strings.Split(ad.prefixesStart.v4, "/")[1])
			bgp4PeerRoutes := bgpPeer.V4Routes().Add().SetName(bgpName + ".rr4")
			bgp4PeerRoutes.Addresses().Add().SetAddress(firstAdvAddr).SetPrefix(uint32(firstAdvPrefix)).SetCount(uint32(ad.prefixesCount))
			bgp4PeerRoutes.AddPath().SetPathId(uint32(otgPort1Details.pathID))
		}
	}
	if ad.prefixesStart.v6 != "" {
		bgp6Name := ateList[0] + ".BGP6.peer"
		bgp6Peer := bgp6ObjectMap[bgp6Name]
		firstAdvAddr := strings.Split(ad.prefixesStart.v6, "/")[0]
		firstAdvPrefix, _ := strconv.Atoi(strings.Split(ad.prefixesStart.v6, "/")[1])
		bgp6PeerRoutes := bgp6Peer.V6Routes().Add().SetName(bgp6Name + ".rr6")
		bgp6PeerRoutes.Addresses().Add().SetAddress(firstAdvAddr).SetPrefix(uint32(firstAdvPrefix)).SetCount(uint32(ad.prefixesCount))
		bgp6PeerRoutes.AddPath().SetPathId(uint32(otgPort1Details.pathID))
	}
	t.Logf("Pushing config to ATE and starting protocols...")
	ad.bs.ATE.OTG().PushConfig(t, ad.bs.ATETop)
	ad.bs.ATE.OTG().StartProtocols(t)
}

type OTGBGPPrefix struct {
	PeerName     string
	Address      string
	PrefixLength uint32
}

func checkOTGBGP4Prefix(t *testing.T, otg *otg.OTG, expectedOTGBGPPrefix OTGBGPPrefix) bool {
	t.Helper()
	t.Logf("expectedPrefix: %+v", expectedOTGBGPPrefix)
	_, ok := gnmi.WatchAll(t,
		otg,
		gnmi.OTG().BgpPeer(expectedOTGBGPPrefix.PeerName).UnicastIpv4PrefixAny().State(),
		time.Minute,
		func(v *ygnmi.Value[*otgtelemetry.BgpPeer_UnicastIpv4Prefix]) bool {
			_, present := v.Val()
			return present
		}).Await(t)

	found := false
	if ok {
		bgpPrefixes := gnmi.GetAll(t, otg, gnmi.OTG().BgpPeer(expectedOTGBGPPrefix.PeerName).UnicastIpv4PrefixAny().State())
		for _, bgpPrefix := range bgpPrefixes {
			if bgpPrefix.Address != nil && bgpPrefix.GetAddress() == expectedOTGBGPPrefix.Address &&
				bgpPrefix.PrefixLength != nil && bgpPrefix.GetPrefixLength() == expectedOTGBGPPrefix.PrefixLength {
				found = true
				break
			}
		}
	}
	return found
}

func checkOTGBGP6Prefix(t *testing.T, otg *otg.OTG, expectedOTGBGPPrefix OTGBGPPrefix) bool {
	t.Helper()
	t.Logf("expectedPrefix: %+v", expectedOTGBGPPrefix)
	_, ok := gnmi.WatchAll(t,
		otg,
		gnmi.OTG().BgpPeer(expectedOTGBGPPrefix.PeerName).UnicastIpv6PrefixAny().State(),
		time.Minute,
		func(v *ygnmi.Value[*otgtelemetry.BgpPeer_UnicastIpv6Prefix]) bool {
			_, present := v.Val()
			return present
		}).Await(t)

	found := false
	if ok {
		bgpPrefixes := gnmi.GetAll(t, otg, gnmi.OTG().BgpPeer(expectedOTGBGPPrefix.PeerName).UnicastIpv6PrefixAny().State())
		for _, bgpPrefix := range bgpPrefixes {
			if bgpPrefix.Address != nil && bgpPrefix.GetAddress() == expectedOTGBGPPrefix.Address &&
				bgpPrefix.PrefixLength != nil && bgpPrefix.GetPrefixLength() == expectedOTGBGPPrefix.PrefixLength {
				found = true
				break
			}
		}
	}
	return found
}

func TestBGP(t *testing.T) {
	tests := []struct {
		desc, fullDesc string
		skipReason     string
		bs             *cfgplugins.BGPSession
		ate            ateData
		wantPrefixes   []ip
	}{{
		desc:     "propagate IPv4 over IPv4",
		fullDesc: "Advertise prefixes from ATE port1, observe received prefixes at ATE port2",
		bs: func() *cfgplugins.BGPSession {
			bs := cfgplugins.NewBGPSession(t, cfgplugins.PortCount2, nil)
			bs.WithEBGP(t, []oc.E_BgpTypes_AFI_SAFI_TYPE{oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST}, []string{"port1", "port2"}, false, false)
			return bs
		}(),
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
		bs: func() *cfgplugins.BGPSession {
			bs := cfgplugins.NewBGPSession(t, cfgplugins.PortCount2, nil)
			bs.WithEBGP(t, []oc.E_BgpTypes_AFI_SAFI_TYPE{oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST}, []string{"port1", "port2"}, false, false)
			return bs
		}(),
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
		desc:     "propagate IPv4 over IPv6",
		fullDesc: "IPv4 routes with an IPv6 next-hop when negotiating RFC5549 - validating that routes are accepted and advertised with the specified values.",
		bs: func() *cfgplugins.BGPSession {
			bs := cfgplugins.NewBGPSession(t, cfgplugins.PortCount2, nil)
			bs.WithEBGP(t, []oc.E_BgpTypes_AFI_SAFI_TYPE{oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST, oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST}, []string{"port1", "port2"}, false, false)
			return bs
		}(),
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

			tc.ate.bs = tc.bs
			ateList := []string{"port1", "port2"}
			tc.ate.ConfigureOTG(t, ateList)

			dni := deviations.DefaultNetworkInstance(tc.bs.DUT)
			tc.bs.DUTConf.GetOrCreateNetworkInstance(dni).GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").GetOrCreateBgp()

			tc.bs.PushDUT(t)

			t.Logf("Verify DUT BGP sessions up")
			cfgplugins.VerifyDUTBGPEstablished(t, tc.bs.DUT)

			t.Logf("Verify OTG BGP sessions up")
			cfgplugins.VerifyOTGBGPEstablished(t, tc.bs.ATE)

			for _, prefix := range tc.wantPrefixes {
				var expectedOTGBGPPrefix OTGBGPPrefix
				if prefix.v4 != "" {
					t.Logf("Checking for BGP Prefix %v", prefix.v4)
					addr := strings.Split(prefix.v4, "/")[0]
					prefixLen, _ := strconv.Atoi(strings.Split(prefix.v4, "/")[1])
					if tc.ate.Port2.v4 != "" {
						expectedOTGBGPPrefix = OTGBGPPrefix{PeerName: "port2.BGP4.peer", Address: addr, PrefixLength: uint32(prefixLen)}
					} else {
						expectedOTGBGPPrefix = OTGBGPPrefix{PeerName: "port2.BGP6.peer", Address: addr, PrefixLength: uint32(prefixLen)}
					}
					if !checkOTGBGP4Prefix(t, tc.bs.ATE.OTG(), expectedOTGBGPPrefix) {
						t.Errorf("Prefix %v is not being learned", expectedOTGBGPPrefix.Address)
					}
				}
				if prefix.v6 != "" {
					t.Logf("Checking for BGP Prefix %v", prefix.v6)
					addr := strings.Split(prefix.v6, "/")[0]
					prefixLen, _ := strconv.Atoi(strings.Split(prefix.v6, "/")[1])
					expectedOTGBGPPrefix = OTGBGPPrefix{PeerName: "port2.BGP6.peer", Address: addr, PrefixLength: uint32(prefixLen)}
					if !checkOTGBGP6Prefix(t, tc.bs.ATE.OTG(), expectedOTGBGPPrefix) {
						t.Errorf("Prefix %v is not being learned", expectedOTGBGPPrefix.Address)
					}
				}
			}
			t.Logf("Clearing BGP configuration from %v", tc.bs.DUT.Name())
			cfgplugins.BGPClearConfig(t, tc.bs.DUT)
		})
	}
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}
