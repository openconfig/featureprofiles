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

package route_test

import (
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest/fptest"
	"github.com/openconfig/featureprofiles/internal/isissession/isissession"
	"github.com/openconfig/ondatra/gnmi/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc/oc"
	"github.com/openconfig/ondatra/ondatra"
	"github.com/openconfig/ygnmi/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// The testbed consists of ate:port1 -> dut:port1 and
// dut:port2 -> ate:port2.  The first pair is called the "source"
// pair, and the second the "destination" pair.
//
// * Source: ate:port1 -> dut:port1 subnet 192.0.2.0/30 2001:db8::192:0:2:0/126
// * Destination: dut:port2 -> ate:port2 subnet 192.0.2.4/30 2001:db8::192:0:2:4/126
//
// Note that the first (.0, .3) and last (.4, .7) IPv4 addresses are
// reserved from the subnet for broadcast, so a /30 leaves exactly 2
// usable addresses. This does not apply to IPv6 which allows /127
// for point to point links, but we use /126 so the numbering is
// consistent with IPv4.

const (
	advertisedRoutesv4Prefix = 32
	advertisedRoutesv6Prefix = 128
	dutAS                    = 65501
	ate1AS                   = 64501
	ate2AS                   = 200
	plenIPv4                 = 30
	plenIPv6                 = 126
	rplType                  = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
	rplName                  = "ALLOW"
	peerGrpNamev4            = "BGP-PEER-GROUP-V4"
	peerGrpNamev6            = "BGP-PEER-GROUP-V6"
	peerGrpNamev4P1          = "BGP-PEER-GROUP-V4-P1"
	peerGrpNamev6P1          = "BGP-PEER-GROUP-V6-P1"
	peerGrpNamev4P2          = "BGP-PEER-GROUP-V4-P2"
	peerGrpNamev6P2          = "BGP-PEER-GROUP-V6-P2"
	isisRoute                = "199.0.0.1"
	bgpRoute                 = "203.0.113.0"
	isisRoutev6              = "2001:db8::203:0:113:1"
	bgpRoutev6               = "2001:DB8:2::1"
	RouteCount               = uint32(1000)
)

var (
	dutP1 = attrs.Attributes{
		Desc:    "DUT to ATE source",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::1",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	ateP1 = attrs.Attributes{
		Name:    "ateP1",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::2",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}

	dutP2 = attrs.Attributes{
		Desc:    "DUT to ATE destination",
		IPv4:    "192.0.2.5",
		IPv6:    "2001:db8::5",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}

	ateP2 = attrs.Attributes{
		Name:    "ateP2",
		MAC:     "02:00:02:01:01:01",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::6",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
)

// configureDUT configures all the interfaces and BGP on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	dc := gnmi.OC()
	p1 := dut.Port(t, "port1").Name()
	i1 := dutP1.NewOCInterface(p1, dut)
	gnmi.Replace(t, dut, dc.Interface(p1).Config(), i1)

	p2 := dut.Port(t, "port2").Name()
	i2 := dutP2.NewOCInterface(p2, dut)
	gnmi.Replace(t, dut, dc.Interface(p2).Config(), i2)

	// Configure Network instance type on DUT
	t.Log("Configure/update Network Instance")
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, dut.Port(t, "port1"))
		fptest.SetPortSpeed(t, dut.Port(t, "port2"))
	}
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, p1, deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, p2, deviations.DefaultNetworkInstance(dut), 0)
	}
	configureRoutePolicy(t, dut, rplName, rplType)

	dutConfPath := dc.NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	dutConf := createBGPNeighborP1(dutAS, ate1AS, dut)
	gnmi.Replace(t, dut, dutConfPath.Config(), dutConf)
	dutConf = createBGPNeighborP2(dutAS, ate2AS, dut)
	gnmi.Update(t, dut, dutConfPath.Config(), dutConf)
	ts := isissession.MustNew(t).WithISIS()
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
	ts.PushAndStart(t)
}

type BGPNeighbor struct {
	as         uint32
	neighborip string
	isV4       bool
}

func createBGPNeighborP1(localAs, peerAs uint32, dut *ondatra.DUTDevice) *oc.NetworkInstance_Protocol {
	nbrs := []*BGPNeighbor{
		{as: peerAs, neighborip: ateP1.IPv4, isV4: true},
		{as: peerAs, neighborip: ateP1.IPv6, isV4: false},
	}

	d := &oc.Root{}
	ni1 := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niProto := ni1.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := niProto.GetOrCreateBgp()

	global := bgp.GetOrCreateGlobal()
	global.As = ygot.Uint32(localAs)
	global.RouterId = ygot.String(dutP1.IPv4)

	global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
	global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)

	// Note: we have to define the peer group even if we aren't setting any policy because it's
	// invalid OC for the neighbor to be part of a peer group that doesn't exist.
	pgv4 := bgp.GetOrCreatePeerGroup(peerGrpNamev4P1)
	pgv4.PeerAs = ygot.Uint32(peerAs)
	pgv4.PeerGroupName = ygot.String(peerGrpNamev4P1)
	pgv6 := bgp.GetOrCreatePeerGroup(peerGrpNamev6P1)
	pgv6.PeerAs = ygot.Uint32(peerAs)
	pgv6.PeerGroupName = ygot.String(peerGrpNamev6P1)

	for _, nbr := range nbrs {
		if nbr.isV4 {
			nv4 := bgp.GetOrCreateNeighbor(nbr.neighborip)
			nv4.PeerAs = ygot.Uint32(nbr.as)
			nv4.Enabled = ygot.Bool(true)
			nv4.PeerGroup = ygot.String(peerGrpNamev4P1)
			afisafi := nv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
			afisafi.Enabled = ygot.Bool(true)
			nv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(false)
			if deviations.RoutePolicyUnderAFIUnsupported(dut) {
				rpl := pgv4.GetOrCreateApplyPolicy()
				rpl.ImportPolicy = []string{rplName}
				rpl.ExportPolicy = []string{rplName}
			} else {
				pgafv4 := pgv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
				pgafv4.Enabled = ygot.Bool(true)
				rpl := pgafv4.GetOrCreateApplyPolicy()
				rpl.ImportPolicy = []string{rplName}
				rpl.ExportPolicy = []string{rplName}
			}
		} else {
			nv6 := bgp.GetOrCreateNeighbor(nbr.neighborip)
			nv6.PeerAs = ygot.Uint32(nbr.as)
			nv6.Enabled = ygot.Bool(true)
			nv6.PeerGroup = ygot.String(peerGrpNamev6P1)
			afisafi6 := nv6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
			afisafi6.Enabled = ygot.Bool(true)
			nv6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(false)
			if deviations.RoutePolicyUnderAFIUnsupported(dut) {
				rpl := pgv6.GetOrCreateApplyPolicy()
				rpl.ImportPolicy = []string{rplName}
				rpl.ExportPolicy = []string{rplName}
			} else {
				pgafv6 := pgv6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
				pgafv6.Enabled = ygot.Bool(true)
				rpl := pgafv6.GetOrCreateApplyPolicy()
				rpl.ImportPolicy = []string{rplName}
				rpl.ExportPolicy = []string{rplName}

			}
		}
	}
	return niProto
}

func createBGPNeighborP2(localAs, peerAs uint32, dut *ondatra.DUTDevice) *oc.NetworkInstance_Protocol {
	nbrs := []*BGPNeighbor{
		{as: peerAs, neighborip: ateP2.IPv4, isV4: true},
		{as: peerAs, neighborip: ateP2.IPv6, isV4: false},
	}

	d := &oc.Root{}
	ni1 := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niProto := ni1.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := niProto.GetOrCreateBgp()

	global := bgp.GetOrCreateGlobal()
	global.As = ygot.Uint32(localAs)
	global.RouterId = ygot.String(dutP1.IPv4)

	global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
	global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)

	// Note: we have to define the peer group even if we aren't setting any policy because it's
	// invalid OC for the neighbor to be part of a peer group that doesn't exist.
	pgv4 := bgp.GetOrCreatePeerGroup(peerGrpNamev4P2)
	pgv4.PeerAs = ygot.Uint32(peerAs)
	pgv4.PeerGroupName = ygot.String(peerGrpNamev4P2)
	pgv6 := bgp.GetOrCreatePeerGroup(peerGrpNamev6P2)
	pgv6.PeerAs = ygot.Uint32(peerAs)
	pgv6.PeerGroupName = ygot.String(peerGrpNamev6P2)

	for _, nbr := range nbrs {
		if nbr.isV4 {
			nv4 := bgp.GetOrCreateNeighbor(nbr.neighborip)
			nv4.PeerAs = ygot.Uint32(nbr.as)
			nv4.Enabled = ygot.Bool(true)
			nv4.PeerGroup = ygot.String(peerGrpNamev4P2)
			afisafi := nv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
			afisafi.Enabled = ygot.Bool(true)
			nv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(false)
			if deviations.RoutePolicyUnderAFIUnsupported(dut) {
				rpl := pgv4.GetOrCreateApplyPolicy()
				rpl.ImportPolicy = []string{rplName}
				rpl.ExportPolicy = []string{rplName}
			} else {
				pgafv4 := pgv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
				pgafv4.Enabled = ygot.Bool(true)
				rpl := pgafv4.GetOrCreateApplyPolicy()
				rpl.ImportPolicy = []string{rplName}
				rpl.ExportPolicy = []string{rplName}
			}
		} else {
			nv6 := bgp.GetOrCreateNeighbor(nbr.neighborip)
			nv6.PeerAs = ygot.Uint32(nbr.as)
			nv6.Enabled = ygot.Bool(true)
			nv6.PeerGroup = ygot.String(peerGrpNamev6P2)
			afisafi6 := nv6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
			afisafi6.Enabled = ygot.Bool(true)
			nv6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(false)
			if deviations.RoutePolicyUnderAFIUnsupported(dut) {
				rpl := pgv6.GetOrCreateApplyPolicy()
				rpl.ImportPolicy = []string{rplName}
				rpl.ExportPolicy = []string{rplName}
			} else {
				pgafv6 := pgv6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
				pgafv6.Enabled = ygot.Bool(true)
				rpl := pgafv6.GetOrCreateApplyPolicy()
				rpl.ImportPolicy = []string{rplName}
				rpl.ExportPolicy = []string{rplName}

			}
		}
	}
	return niProto
}

func configureRoutePolicy(t *testing.T, dut *ondatra.DUTDevice, name string, pr oc.E_RoutingPolicy_PolicyResultType) {
	d := &oc.Root{}
	rp := d.GetOrCreateRoutingPolicy()
	pd := rp.GetOrCreatePolicyDefinition(name)
	st, err := pd.AppendNewStatement("id-1")
	if err != nil {
		t.Fatal(err)
	}
	st.GetOrCreateActions().PolicyResult = pr
	gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)
}

func waitForBGPSession(t *testing.T, dut *ondatra.DUTDevice, wantEstablished bool) {
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	nbrPath := statePath.Neighbor(ateP2.IPv4)
	nbrPathv6 := statePath.Neighbor(ateP2.IPv6)
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

	_, ok := gnmi.Watch(t, dut, nbrPath.SessionState().State(), 2*time.Minute, compare).Await(t)
	if !ok {
		fptest.LogQuery(t, "BGP reported state", nbrPath.State(), gnmi.Get(t, dut, nbrPath.State()))
		if wantEstablished {
			t.Fatal("No BGP neighbor formed...")
		} else {
			t.Fatal("BGPv4 session didn't teardown.")
		}
	}
	_, ok = gnmi.Watch(t, dut, nbrPathv6.SessionState().State(), 2*time.Minute, compare).Await(t)
	if !ok {
		fptest.LogQuery(t, "BGPv6 reported state", nbrPathv6.State(), gnmi.Get(t, dut, nbrPathv6.State()))
		if wantEstablished {
			t.Fatal("No BGPv6 neighbor formed...")
		} else {
			t.Fatal("BGPv6 session didn't teardown.")
		}
	}
}

func verifyBGPTelemetry(t *testing.T, dut *ondatra.DUTDevice) {
	t.Log("Waiting for BGPv4 neighbor to establish...")
	waitForBGPSession(t, dut, true)

}

func configureATE(t *testing.T) gosnappi.Config {
	ate := ondatra.ATE(t, "ate")
	ap1 := ate.Port(t, "port1")
	ap2 := ate.Port(t, "port2")
	config := gosnappi.NewConfig()
	// add ports
	p1 := config.Ports().Add().SetName(ap1.ID())
	p2 := config.Ports().Add().SetName(ap2.ID())
	// add devices
	d1 := config.Devices().Add().SetName("p1.d1")
	d2 := config.Devices().Add().SetName("p2.d1")
	// Configuration on port1.
	d1Eth1 := d1.Ethernets().
		Add().
		SetName("p1.d1.eth1").
		SetMac("00:00:02:02:02:02").
		SetMtu(1500)
	d1Eth1.
		Connection().
		SetPortName(p1.Name())

	d1ipv41 := d1Eth1.
		Ipv4Addresses().
		Add().
		SetName("p1.d1.eth1.ipv4").
		SetAddress("192.0.2.2").
		SetGateway("192.0.2.1").
		SetPrefix(30)

	d1ipv61 := d1Eth1.
		Ipv6Addresses().
		Add().
		SetName("p1.d1.eth1.ipv6").
		SetAddress("2001:db8::2").
		SetGateway("2001:db8::1").
		SetPrefix(126)

	// isis router
	d1isis := d1.Isis().
		SetName("p1.d1.isis").
		SetSystemId("650000000001")
	d1isis.Basic().
		SetIpv4TeRouterId(d1ipv41.Address()).
		SetHostname("ixia-c-port1")
	d1isis.Advanced().SetAreaAddresses([]string{"49"})
	d1isisint := d1isis.Interfaces().
		Add().
		SetName("p1.d1.isis.intf").
		SetEthName(d1Eth1.Name()).
		SetNetworkType(gosnappi.IsisInterfaceNetworkType.POINT_TO_POINT).
		SetLevelType(gosnappi.IsisInterfaceLevelType.LEVEL_2).
		SetMetric(10)
	d1isisint.TrafficEngineering().Add().PriorityBandwidths()
	d1isisint.Advanced().SetAutoAdjustMtu(true).SetAutoAdjustArea(true).SetAutoAdjustSupportedProtocols(true)

	d1IsisRoute1 := d1isis.V4Routes().Add().SetName("p1.d1.isis.rr1")
	d1IsisRoute1.Addresses().
		Add().
		SetAddress(isisRoute).
		SetPrefix(32).SetCount(RouteCount)

	d1IsisRoute1v6 := d1isis.V6Routes().Add().SetName("p1.d1.isis.rr1.v6")
	d1IsisRoute1v6.Addresses().
		Add().
		SetAddress(isisRoutev6).
		SetPrefix(126).SetCount(RouteCount)

	configureBGPDev(d1, d1ipv41, d1ipv61, ate1AS)

	// configuration on port2
	d2Eth1 := d2.Ethernets().
		Add().
		SetName("p2.d1.eth1").
		SetMac("00:00:03:03:03:03").
		SetMtu(1500)
	d2Eth1.
		Connection().
		SetPortName(p2.Name())
	d2ipv41 := d2Eth1.Ipv4Addresses().
		Add().
		SetName("p2.d1.eth1.ipv4").
		SetAddress("192.0.2.6").
		SetGateway("192.0.2.5").
		SetPrefix(30)

	d2ipv61 := d2Eth1.
		Ipv6Addresses().
		Add().
		SetName("p2.d1.eth1.ipv6").
		SetAddress("2001:db8::6").
		SetGateway("2001:db8::5").
		SetPrefix(126)

	// isis router
	d2isis := d2.Isis().
		SetName("p2.d1.isis").
		SetSystemId("650000000001")
	d2isis.Basic().
		SetIpv4TeRouterId(d2ipv41.Address()).
		SetHostname("ixia-c-port2")
	d2isis.Advanced().SetAreaAddresses([]string{"49"})
	d2isisint := d2isis.Interfaces().
		Add().
		SetName("p2.d1.isis.intf").
		SetEthName(d2Eth1.Name()).
		SetNetworkType(gosnappi.IsisInterfaceNetworkType.POINT_TO_POINT).
		SetLevelType(gosnappi.IsisInterfaceLevelType.LEVEL_2).
		SetMetric(10)
	d2isisint.TrafficEngineering().Add().PriorityBandwidths()
	d2isisint.Advanced().SetAutoAdjustMtu(true).SetAutoAdjustArea(true).SetAutoAdjustSupportedProtocols(true)

	d2IsisRoute1 := d2isis.V4Routes().Add().SetName("p2.d1.isis.rr1")
	d2IsisRoute1.Addresses().
		Add().
		SetAddress(isisRoute).
		SetPrefix(32).
		SetCount(RouteCount)

	d2IsisRoute1V6 := d2isis.V6Routes().Add().SetName("p2.d1.isis.rr1.v6")
	d2IsisRoute1V6.Addresses().
		Add().
		SetAddress(isisRoutev6).
		SetPrefix(126).
		SetCount(RouteCount)

	configureBGPDev(d2, d2ipv41, d2ipv61, ate2AS)

	return config
}

// configureBGPDev configures the BGP on the OTG dev
func configureBGPDev(dev gosnappi.Device, Ipv4 gosnappi.DeviceIpv4, Ipv6 gosnappi.DeviceIpv6, as int) {

	Bgp := dev.Bgp().SetRouterId(Ipv4.Address())
	Bgp4Peer := Bgp.Ipv4Interfaces().Add().SetIpv4Name(Ipv4.Name()).Peers().Add().SetName(dev.Name() + ".BGP4.peer")
	Bgp4Peer.SetPeerAddress(Ipv4.Gateway()).SetAsNumber(uint32(as)).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
	Bgp6Peer := Bgp.Ipv6Interfaces().Add().SetIpv6Name(Ipv6.Name()).Peers().Add().SetName(dev.Name() + ".BGP6.peer")
	Bgp6Peer.SetPeerAddress(Ipv6.Gateway()).SetAsNumber(uint32(as)).SetAsType(gosnappi.BgpV6PeerAsType.EBGP)

	configureBGPv4Routes(Bgp4Peer, Ipv4.Address(), Bgp4Peer.Name()+"v4route", bgpRoute, RouteCount)
	configureBGPv6Routes(Bgp6Peer, Ipv6.Address(), Bgp6Peer.Name()+"v6route", bgpRoutev6, RouteCount)

}

func configureBGPv4Routes(peer gosnappi.BgpV4Peer, ipv4 string, name string, prefix string, count uint32) {
	routes := peer.V4Routes().Add().SetName(name)
	routes.SetNextHopIpv4Address(ipv4).
		SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).
		SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)
	routes.Addresses().Add().
		SetAddress(prefix).
		SetPrefix(advertisedRoutesv4Prefix).
		SetCount(count)
}

func configureBGPv6Routes(peer gosnappi.BgpV6Peer, ipv6 string, name string, prefix string, count uint32) {
	routes := peer.V6Routes().Add().SetName(name)
	routes.SetNextHopIpv6Address(ipv6).
		SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).
		SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL)
	routes.Addresses().Add().
		SetAddress(prefix).
		SetPrefix(advertisedRoutesv6Prefix).
		SetCount(count)
}
func VerifyDUT(t *testing.T, dut *ondatra.DUTDevice) {

	dni := deviations.DefaultNetworkInstance(dut)
	if got, ok := gnmi.Await(t, dut, gnmi.OC().NetworkInstance(dni).Afts().AftSummaries().Ipv4Unicast().Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP).Counters().AftEntries().State(), 1*time.Minute, uint64(RouteCount)).Val(); !ok {
		t.Errorf("ipv4 BGP entries, got: %d, want: %d", got, RouteCount)
	} else {
		t.Logf("Test case Passed: ipv4 BGP entries, got: %d, want: %d", got, RouteCount)
	}
	if got, ok := gnmi.Await(t, dut, gnmi.OC().NetworkInstance(dni).Afts().AftSummaries().Ipv6Unicast().Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP).Counters().AftEntries().State(), 1*time.Minute, uint64(RouteCount)).Val(); !ok {
		t.Errorf("ipv6 BGP entries, got: %d, want: %d", got, RouteCount)
	} else {
		t.Logf("Test case Passed:ipv6 BGP entries, got: %d, want: %d", got, RouteCount)
	}

	if got, ok := gnmi.Await(t, dut, gnmi.OC().NetworkInstance(dni).Afts().AftSummaries().Ipv4Unicast().Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS).Counters().AftEntries().State(), 1*time.Minute, uint64(RouteCount)).Val(); !ok {
		t.Errorf("ipv4 isis entries, got: %d, want: %d", got, RouteCount)
	} else {
		t.Logf("Test case Passed: ipv4 isis entries, got: %d, want: %d", got, RouteCount)

	}

	if got, ok := gnmi.Await(t, dut, gnmi.OC().NetworkInstance(dni).Afts().AftSummaries().Ipv6Unicast().Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS).Counters().AftEntries().State(), 1*time.Minute, uint64(RouteCount)).Val(); !ok {
		t.Errorf("ipv6 isis entries, got: %d, want: %d", got, RouteCount)
	} else {
		t.Logf("Test case Passed: ipv6 isis entries, got: %d, want: %d", got, RouteCount)
	}
}

func TestBGP(t *testing.T) {

	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	// DUT Configuration
	t.Log("Start DUT interface Config")
	configureDUT(t, dut)
	// ATE Configuration.
	t.Log("Start ATE Config")
	config := configureATE(t)
	ate.OTG().PushConfig(t, config)
	time.Sleep(time.Second * 20)
	ate.OTG().StartProtocols(t)
	time.Sleep(time.Second * 20)
	verifyBGPTelemetry(t, dut)
	VerifyDUT(t, dut)
}
