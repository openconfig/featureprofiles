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

package route_summary_counters_test

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/isissession"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	otgtelemetry "github.com/openconfig/ondatra/gnmi/otg"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

var (
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

	otgPort1 = attrs.Attributes{
		MAC:  "02:00:01:01:01:01",
		IPv4: "192.0.2.2",
		IPv6: "2001:db8::2",
	}
	otgPort2 = attrs.Attributes{
		MAC:  "02:00:02:01:01:01",
		IPv4: "192.0.2.6",
		IPv6: "2001:db8::6",
	}

	pathID        = 1
	prefixesCount = 4
	bgpPeerGroup  = "BGP-PEER-GROUP"

	targetNetwork = &attrs.Attributes{
		Desc:    "External network (simulated by ATE)",
		IPv4:    "198.51.100.0",
		IPv4Len: 24,
		IPv6:    "2001:db8::198:51:100:0",
		IPv6Len: 112,
	}
)

const (
	rplPermitAll = "PERMIT-ALL"
	dutAS        = uint32(65537)
	ateAS1       = uint32(65536)
	ateAS2       = uint32(65538)
)

type ip struct {
	v4 string
	v6 string
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func configureOTG(t *testing.T, ts *isissession.TestSession) {
	// netv4 is a simulated network containing the ipv4 addresses specified by targetNetwork
	netv4 := ts.ATEIntf1.Isis().V4Routes().Add().SetName("netv4").SetLinkMetric(10).SetOriginType(gosnappi.IsisV4RouteRangeOriginType.EXTERNAL)
	netv4.Addresses().Add().SetAddress(targetNetwork.IPv4).SetPrefix(uint32(targetNetwork.IPv4Len)).SetCount(uint32(prefixesCount))

	// netv6 is a simulated network containing the ipv6 addresses specified by targetNetwork
	netv6 := ts.ATEIntf1.Isis().V6Routes().Add().SetName("netv6").SetLinkMetric(10).SetOriginType(gosnappi.IsisV6RouteRangeOriginType.EXTERNAL)
	netv6.Addresses().Add().SetAddress(targetNetwork.IPv6).SetPrefix(uint32(targetNetwork.IPv6Len)).SetCount(uint32(prefixesCount))

	t.Log("Starting protocols on ATE...")
	ts.PushAndStart(t)
	ts.MustAdjacency(t)
}

func TestRouteSummaryWithISIS(t *testing.T) {
	ts := isissession.MustNew(t).WithISIS()
	otg := ts.ATE.OTG()

	ts.ConfigISIS(func(isis *oc.NetworkInstance_Protocol_Isis) {
		global := isis.GetOrCreateGlobal()
		global.HelloPadding = oc.Isis_HelloPaddingType_DISABLE

		if deviations.ISISSingleTopologyRequired(ts.DUT) {
			afv6 := global.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST)
			afv6.GetOrCreateMultiTopology().SetAfiName(oc.IsisTypes_AFI_TYPE_IPV4)
			afv6.GetOrCreateMultiTopology().SetSafiName(oc.IsisTypes_SAFI_TYPE_UNICAST)
		}
	})
	ts.ATEIntf1.Isis().Advanced().SetEnableHelloPadding(false)

	configureOTG(t, ts)
	gnmi.Watch(t, otg, gnmi.OTG().IsisRouter("devIsis").Counters().Level2().InLsp().State(), 30*time.Second, func(v *ygnmi.Value[uint64]) bool {
		time.Sleep(5 * time.Second)
		val, present := v.Val()
		return present && val >= 1
	}).Await(t)

	dni := deviations.DefaultNetworkInstance(ts.DUT)
	ipv4Entry := gnmi.Get(t, ts.DUT, gnmi.OC().NetworkInstance(dni).Afts().AftSummaries().Ipv4Unicast().Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS).Counters().AftEntries().State())
	if ipv4Entry == 0 {
		t.Errorf("ipv4 BGP entries, got: %d, want: %d", ipv4Entry, prefixesCount)
	}

	ipv6Entry := gnmi.Get(t, ts.DUT, gnmi.OC().NetworkInstance(dni).Afts().AftSummaries().Ipv6Unicast().Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS).Counters().AftEntries().State())
	if ipv6Entry == 0 {
		t.Errorf("ipv6 BGP entries, got: %d, want: %d", ipv6Entry, prefixesCount)
	}
}

func TestRouteSummaryWithBGP(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	tests := []struct {
		desc string
		dut  dutData
		ate  ateData
	}{{
		desc: "propagate IPv4 over IPv4",
		dut: dutData{
			routerID: dutPort1.IPv4,
			neighborPG: map[string]string{
				otgPort1.IPv4: bgpPeerGroup,
				otgPort2.IPv4: bgpPeerGroup,
			},
			neighborAS: map[string]uint32{
				otgPort1.IPv4: ateAS1,
				otgPort2.IPv4: ateAS2,
			},
			ipv4: true,
		},
		ate: ateData{
			Port1:         ip{v4: "192.0.2.2/30"},
			Port1Neighbor: dutPort1.IPv4,
			Port2:         ip{v4: "192.0.2.6/30"},
			Port2Neighbor: dutPort2.IPv4,
			prefixesStart: ip{v4: "198.51.100.0/32"},
		},
	}, {
		desc: "propagate IPv6 over IPv6",
		dut: dutData{
			routerID: dutPort1.IPv4,
			neighborPG: map[string]string{
				otgPort1.IPv6: bgpPeerGroup,
				otgPort2.IPv6: bgpPeerGroup,
			},
			neighborAS: map[string]uint32{
				otgPort1.IPv6: ateAS1,
				otgPort2.IPv6: ateAS2,
			},
			ipv4: false,
		},
		ate: ateData{
			Port1:         ip{v6: "2001:db8::2/126"},
			Port1Neighbor: dutPort1.IPv6,
			Port2:         ip{v6: "2001:db8::6/126"},
			Port2Neighbor: dutPort2.IPv6,
			prefixesStart: ip{v6: "2001:db8:1::1/128"},
		},
	}}
	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			tc.dut.Configure(t, dut)

			ate := ondatra.ATE(t, "ate")
			otgConfig := tc.ate.ConfigureOTG(t, ate.OTG(), []string{"port1", "port2"})

			t.Logf("Verify DUT BGP sessions up")
			tc.dut.AwaitBGPEstablished(t, dut, tc.dut.neighborAS)
			t.Logf("Verify OTG BGP sessions up")
			verifyOTGBGPTelemetry(t, ate.OTG(), otgConfig, "ESTABLISHED")

			dni := deviations.DefaultNetworkInstance(dut)
			if tc.dut.ipv4 {
				ipv4Entry := gnmi.Get(t, dut, gnmi.OC().NetworkInstance(dni).Afts().AftSummaries().Ipv4Unicast().Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP).Counters().AftEntries().State())
				if ipv4Entry == 0 {
					t.Errorf("ipv4 BGP entries, got: %d, want: %d", ipv4Entry, prefixesCount)
				}
			} else {
				ipv6Entry := gnmi.Get(t, dut, gnmi.OC().NetworkInstance(dni).Afts().AftSummaries().Ipv6Unicast().Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP).Counters().AftEntries().State())
				if ipv6Entry == 0 {
					t.Errorf("ipv4 BGP entries, got: %d, want: %d", ipv6Entry, prefixesCount)
				}
			}
		})
	}
}

type ateData struct {
	Port1         ip
	Port2         ip
	Port1Neighbor string
	Port2Neighbor string
	prefixesStart ip
}

func (ad *ateData) ConfigureOTG(t *testing.T, otg *otg.OTG, ateList []string) gosnappi.Config {
	config := gosnappi.NewConfig()
	bgp4ObjectMap := make(map[string]gosnappi.BgpV4Peer)
	bgp6ObjectMap := make(map[string]gosnappi.BgpV6Peer)
	ipv4ObjectMap := make(map[string]gosnappi.DeviceIpv4)
	ipv6ObjectMap := make(map[string]gosnappi.DeviceIpv6)
	for ateIndex, v := range []struct {
		iface    attrs.Attributes
		ip       ip
		neighbor string
		as       uint32
	}{
		{otgPort1, ad.Port1, ad.Port1Neighbor, ateAS1},
		{otgPort2, ad.Port2, ad.Port2Neighbor, ateAS2},
	} {

		devName := ateList[ateIndex] + ".dev"
		port := config.Ports().Add().SetName(ateList[ateIndex])
		dev := config.Devices().Add().SetName(devName)

		eth := dev.Ethernets().Add().SetName(devName + ".Eth").SetMac(v.iface.MAC)
		eth.Connection().SetChoice(gosnappi.EthernetConnectionChoice.PORT_NAME).SetPortName(port.Name())
		bgp := dev.Bgp().SetRouterId(v.iface.IPv4)
		if v.ip.v4 != "" {
			address := strings.Split(v.ip.v4, "/")[0]
			prefixInt4, _ := strconv.Atoi(strings.Split(v.ip.v4, "/")[1])
			ipv4 := eth.Ipv4Addresses().Add().SetName(devName + ".IPv4").SetAddress(address).SetGateway(v.neighbor).SetPrefix(uint32(prefixInt4))
			bgp4Name := devName + ".BGP4.peer"
			bgp4Peer := bgp.Ipv4Interfaces().Add().SetIpv4Name(ipv4.Name()).Peers().Add().SetName(bgp4Name).SetPeerAddress(ipv4.Gateway()).SetAsNumber(uint32(v.as)).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)

			bgp4Peer.Capability().SetIpv4UnicastAddPath(true).SetIpv6UnicastAddPath(true)
			bgp4Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)

			bgp4ObjectMap[bgp4Name] = bgp4Peer
			ipv4ObjectMap[devName+".IPv4"] = ipv4
		}
		if v.ip.v6 != "" {
			address := strings.Split(v.ip.v6, "/")[0]
			prefixInt6, _ := strconv.Atoi(strings.Split(v.ip.v6, "/")[1])
			ipv6 := eth.Ipv6Addresses().Add().SetName(devName + ".IPv6").SetAddress(address).SetGateway(v.neighbor).SetPrefix(uint32(prefixInt6))
			bgp6Name := devName + ".BGP6.peer"
			bgp6Peer := bgp.Ipv6Interfaces().Add().SetIpv6Name(ipv6.Name()).Peers().Add().SetName(bgp6Name).SetPeerAddress(ipv6.Gateway()).SetAsNumber(uint32(v.as)).SetAsType(gosnappi.BgpV6PeerAsType.EBGP)

			bgp6Peer.Capability().SetIpv4UnicastAddPath(true).SetIpv6UnicastAddPath(true).SetExtendedNextHopEncoding(true)
			bgp6Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)

			bgp6ObjectMap[bgp6Name] = bgp6Peer
			ipv6ObjectMap[devName+".IPv6"] = ipv6
		}
	}
	if ad.prefixesStart.v4 != "" {
		bgpName := ateList[0] + ".dev.BGP4.peer"
		bgpPeer := bgp4ObjectMap[bgpName]
		ip := ipv4ObjectMap[ateList[0]+".dev.IPv4"]
		firstAdvAddr := strings.Split(ad.prefixesStart.v4, "/")[0]
		firstAdvPrefix, _ := strconv.Atoi(strings.Split(ad.prefixesStart.v4, "/")[1])
		bgp4PeerRoutes := bgpPeer.V4Routes().Add().SetName(bgpName + ".rr4").SetNextHopIpv4Address(ip.Address()).SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)
		bgp4PeerRoutes.Addresses().Add().SetAddress(firstAdvAddr).SetPrefix(uint32(firstAdvPrefix)).SetCount(uint32(prefixesCount))
		bgp4PeerRoutes.AddPath().SetPathId(uint32(pathID))
	}
	if ad.prefixesStart.v6 != "" {
		bgp6Name := ateList[0] + ".dev.BGP6.peer"
		bgp6Peer := bgp6ObjectMap[bgp6Name]
		firstAdvAddr := strings.Split(ad.prefixesStart.v6, "/")[0]
		firstAdvPrefix, _ := strconv.Atoi(strings.Split(ad.prefixesStart.v6, "/")[1])
		bgp6PeerRoutes := bgp6Peer.V6Routes().Add().SetName(bgp6Name + ".rr6")
		bgp6PeerRoutes.Addresses().Add().SetAddress(firstAdvAddr).SetPrefix(uint32(firstAdvPrefix)).SetCount(uint32(prefixesCount))
		bgp6PeerRoutes.AddPath().SetPathId(uint32(pathID))
	}

	t.Logf("Pushing config to ATE and starting protocols...")
	otg.PushConfig(t, config)
	otg.StartProtocols(t)
	return config

}

type dutData struct {
	routerID   string
	neighborPG map[string]string
	neighborAS map[string]uint32
	ipv4       bool
}

func configureRoutingPolicy(d *oc.Root) (*oc.RoutingPolicy, error) {
	rp := d.GetOrCreateRoutingPolicy()
	pdef := rp.GetOrCreatePolicyDefinition(rplPermitAll)
	stmt, err := pdef.AppendNewStatement("20")
	if err != nil {
		return nil, err
	}
	stmt.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
	return rp, nil
}

func (d *dutData) Configure(t *testing.T, dut *ondatra.DUTDevice) {
	aftType := oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST
	if !d.ipv4 {
		aftType = oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST
	}
	bgpOC := cfgplugins.BuildBGPOCConfig(t, dut, d.routerID, aftType, d.neighborAS, d.neighborPG)

	for _, a := range []attrs.Attributes{dutPort1, dutPort2} {
		ocName := dut.Port(t, a.Name).Name()
		gnmi.Replace(t, dut, gnmi.OC().Interface(ocName).Config(), a.NewOCInterface(ocName, dut))
	}

	t.Log("Configure Network Instance")
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	if deviations.ExplicitPortSpeed(dut) {
		for _, a := range []attrs.Attributes{dutPort1, dutPort2} {
			fptest.SetPortSpeed(t, dut.Port(t, a.Name))
		}
	}
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		for _, a := range []attrs.Attributes{dutPort1, dutPort2} {
			ocName := dut.Port(t, a.Name).Name()
			fptest.AssignToNetworkInstance(t, dut, ocName, deviations.DefaultNetworkInstance(dut), 0)
		}
	}

	dutProto := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	niProtocol := &oc.NetworkInstance_Protocol{
		Identifier: oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP,
		Name:       ygot.String("BGP"),
		Bgp:        bgpOC,
	}
	rpl, err := configureRoutingPolicy(&oc.Root{})
	if err != nil {
		t.Fatalf("Failed to configure routing policy: %v", err)
	}
	gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().Config(), rpl)
	gnmi.Replace(t, dut, dutProto.Config(), niProtocol)
}

func (d *dutData) AwaitBGPEstablished(t *testing.T, dut *ondatra.DUTDevice, neighborAS map[string]uint32) {
	for neighbor := range neighborAS {
		gnmi.Await(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).
			Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").
			Bgp().
			Neighbor(neighbor).
			SessionState().State(), time.Second*120, oc.Bgp_Neighbor_SessionState_ESTABLISHED)
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
