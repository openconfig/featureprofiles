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
	"fmt"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
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

func (ad *ateData) Configure(t *testing.T, ate *ondatra.ATEDevice) {
	topo := ate.Topology().New()

	if1 := topo.AddInterface("port1").WithPort(ate.Port(t, "port1"))
	if2 := topo.AddInterface("port2").WithPort(ate.Port(t, "port2"))

	for _, v := range []struct {
		iface    *ondatra.Interface
		ip       ip
		neighbor string
		as       uint32
	}{
		{if1, ad.Port1, ad.Port1Neighbor, ateAS1},
		{if2, ad.Port2, ad.Port2Neighbor, ateAS2},
	} {
		if v.ip.v4 != "" {
			v.iface.IPv4().WithAddress(v.ip.v4).WithDefaultGateway(v.neighbor)
		}
		if v.ip.v6 != "" {
			v.iface.IPv6().WithAddress(v.ip.v6).WithDefaultGateway(v.neighbor)
		}
		v.iface.BGP().AddPeer().WithPeerAddress(v.neighbor).WithLocalASN(v.as).
			WithTypeExternal().Capabilities().
			WithExtendedNextHopEncodingEnabled(true).
			WithIPv4UnicastEnabled(true).
			WithIPv6UnicastEnabled(true)
	}

	if ad.prefixesStart.v4 != "" {
		if1.AddNetwork("bgpNet1").
			IPv4().WithAddress(ad.prefixesStart.v4).WithCount(ad.prefixesCount)
	}
	if ad.prefixesStart.v6 != "" {
		if1.AddNetwork("bgpNet1").
			IPv6().WithAddress(ad.prefixesStart.v6).WithCount(ad.prefixesCount)
	}

	topo.Push(t)
	topo.StartProtocols(t)
}

type dutData struct {
	bgpOC *oc.NetworkInstance_Protocol_Bgp
}

func (d *dutData) Configure(t *testing.T, dut *ondatra.DUTDevice) {
	for _, a := range []attrs.Attributes{dutPort1, dutPort2} {
		ocName := dut.Port(t, a.Name).Name()
		gnmi.Replace(t, dut, gnmi.OC().Interface(ocName).Config(), a.NewOCInterface(ocName))
	}

	t.Log("Configure Network Instance")
	dutConfNIPath := gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance)
	gnmi.Replace(t, dut, dutConfNIPath.Type().Config(), oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)

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
			tc.dut.Configure(t, dut)

			ate := ondatra.ATE(t, "ate")
			tc.ate.Configure(t, ate)

			tc.dut.AwaitBGPEstablished(t, dut)

			for _, prefix := range tc.wantPrefixes {
				rib := gnmi.OC().NetworkInstance("port2").
					Protocol(
						oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP,
						fmt.Sprintf("%d", ateAS2),
					).Bgp().
					Rib()
				// Don't care about the value, but I can only fetch leaves from ATE telemetry. This
				// should fail in the Get(t) method if the Route is missing.
				if prefix.v4 != "" {
					_, found := gnmi.Watch(t, ate, rib.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Ipv4Unicast().
						Neighbor(tc.ate.Port2Neighbor).
						AdjRibInPre().
						Route(prefix.v4, 0).
						AttrIndex().State(), 1*time.Minute, func(i *ygnmi.Value[uint64]) bool {
						return i.IsPresent()
					}).Await(t)
					if !found {
						t.Fatalf("Did not find RIB for prefix %s", prefix.v4)
					}
				}
				if prefix.v6 != "" {
					_, found := gnmi.Watch(t, ate, rib.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Ipv6Unicast().
						Neighbor(tc.ate.Port2Neighbor).
						AdjRibInPre().
						Route(prefix.v6, 0).
						AttrIndex().State(), 1*time.Minute, func(i *ygnmi.Value[uint64]) bool {
						return i.IsPresent()
					}).Await(t)
					if !found {
						t.Fatalf("Did not find RIB for prefix %s", prefix.v6)
					}
				}
			}
		})
	}
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}
