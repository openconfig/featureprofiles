// Copyright 2023 Google LLC
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

package bgp_route_reflector_capabilities_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/gnmi/oc/netinstbgp"
	"github.com/openconfig/ondatra/netutil"
	otg "github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	peerGrpName1         = "BGP-PEER-GROUP1"
	peerGrpName2         = "BGP-PEER-GROUP2"
	peerGrpName3         = "BGP-PEER-GROUP3"
	peerGrpName4         = "BGP-PEER-GROUP4"
	routeCntV4500k       = 500000
	routeCntV41M         = 1000000
	routeCntV6200k       = 200000
	routeCntV6600k       = 600000
	dutAS                = 65501
	ateAS                = 65502
	plenIPv4             = 30
	plenIPv6             = 126
	rplAllowPolicy       = "ALLOW"
	dutAreaAddress       = "49.0001"
	dutSysID             = "1920.0000.2001"
	otgSysID2            = "640000000001"
	otgSysID3            = "640000000002"
	isisInstance         = "DEFAULT"
	port2LocPref         = 50
	port3LocPref         = 50
	advV4Routes500kPort2 = "20.0.0.1"  // New IPv4 block needs to be provided by Google. - 500k v4 routes
	advV4Routes500kPort3 = "30.0.0.1"  // New IPv4 block needs to be provided by Google. - 500k v4 routes
	advV4Routes1M        = "100.0.0.0" // New IPv4 block needs to be provided by Google. - 1M v4 routes
	advV6Routes200kPort2 = "2001:DB8:1::1"
	advV6Routes200kPort3 = "2001:DB8:2::1"
	advV6Routes600k      = "2001:DB8:3::1"
	otgIsisPort2LoopV4   = "203.0.113.10"
	otgIsisPort2LoopV6   = "2001:db8::203:0:113:10"
	otgIsisPort3LoopV4   = "203.0.113.20"
	otgIsisPort3LoopV6   = "2001:db8::203:0:113:20"
	clusterID            = "198.18.0.0"
	v4Prefixes           = true
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "DUT to ATE Port1",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::192:0:2:1",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	atePort1 = attrs.Attributes{
		Name:    "atePort1",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::192:0:2:2",
		MAC:     "02:00:01:01:01:01",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	dutPort2 = attrs.Attributes{
		Desc:    "DUT to ATE Port2",
		IPv4:    "192.0.2.5",
		IPv6:    "2001:db8::192:0:2:5",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	atePort2 = attrs.Attributes{
		Name:    "atePort2",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::192:0:2:6",
		MAC:     "02:00:02:01:01:01",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	dutPort3 = attrs.Attributes{
		Desc:    "DUT to ATE Port3",
		IPv4:    "192.0.2.9",
		IPv6:    "2001:db8::192:0:2:9",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	atePort3 = attrs.Attributes{
		Name:    "atePort3",
		IPv4:    "192.0.2.10",
		IPv6:    "2001:db8::192:0:2:10",
		MAC:     "02:00:03:01:01:01",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	dutlo0Attrs = attrs.Attributes{
		Desc:    "Loopback ip",
		IPv4:    "203.0.113.1",
		IPv6:    "2001:db8::203:0:113:1",
		IPv4Len: 32,
		IPv6Len: 128,
	}
	asPath           = []uint32{65000, 65499, 65498}
	commAttr         = []oc.UnionString{"65200:200"}
	asPath2          = []uint32{65001, 65002, 65003}
	asPath3          = []uint32{65004, 65005, 65006}
	commAttrExt      = []oc.UnionString{"65010:100"}
	loopbackIntfName string
)

func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	dc := gnmi.OC()
	i1 := dutPort1.NewOCInterface(dut.Port(t, "port1").Name(), dut)
	gnmi.Replace(t, dut, dc.Interface(i1.GetName()).Config(), i1)

	i2 := dutPort2.NewOCInterface(dut.Port(t, "port2").Name(), dut)
	gnmi.Replace(t, dut, dc.Interface(i2.GetName()).Config(), i2)

	i3 := dutPort3.NewOCInterface(dut.Port(t, "port3").Name(), dut)
	gnmi.Replace(t, dut, dc.Interface(i3.GetName()).Config(), i3)

	loopbackIntfName = netutil.LoopbackInterface(t, dut, 0)
	loop1 := dutlo0Attrs.NewOCInterface(loopbackIntfName, dut)
	loop1.Type = oc.IETFInterfaces_InterfaceType_softwareLoopback
	gnmi.Replace(t, dut, dc.Interface(loopbackIntfName).Config(), loop1)
}

func verifyPortsUp(t *testing.T, dev *ondatra.Device) {
	t.Helper()
	for _, p := range dev.Ports() {
		status := gnmi.Get(t, dev, gnmi.OC().Interface(p.Name()).OperStatus().State())
		if want := oc.Interface_OperStatus_UP; status != want {
			t.Errorf("%s Status: got %v, want %v", p, status, want)
		}
	}
}

func configureRoutePolicy(t *testing.T, dut *ondatra.DUTDevice, name string, pr oc.E_RoutingPolicy_PolicyResultType) {
	t.Helper()
	d := &oc.Root{}
	rp := d.GetOrCreateRoutingPolicy()
	pd := rp.GetOrCreatePolicyDefinition(name)
	st, err := pd.AppendNewStatement("id-1")
	if err != nil {
		t.Fatal(err)
	}
	stc := st.GetOrCreateConditions()
	stc.InstallProtocolEq = oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP
	st.GetOrCreateActions().PolicyResult = pr
	gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)
}

func bgpCreateNbr(localAs, peerAs uint32, dut *ondatra.DUTDevice) *oc.NetworkInstance_Protocol {
	nbr1v4 := &bgpNeighbor{as: ateAS, neighborip: atePort1.IPv4, isV4: true, peerGrp: peerGrpName1, isRR: false}
	nbr2v4 := &bgpNeighbor{as: dutAS, neighborip: otgIsisPort2LoopV4, isV4: true, peerGrp: peerGrpName2, localAddress: dutlo0Attrs.IPv4, isRR: true}
	nbr3v4 := &bgpNeighbor{as: dutAS, neighborip: otgIsisPort3LoopV4, isV4: true, peerGrp: peerGrpName2, localAddress: dutlo0Attrs.IPv4, isRR: true}
	nbr1v6 := &bgpNeighbor{as: ateAS, neighborip: atePort1.IPv6, isV4: false, peerGrp: peerGrpName3, isRR: false}
	nbr2v6 := &bgpNeighbor{as: dutAS, neighborip: otgIsisPort2LoopV6, isV4: false, peerGrp: peerGrpName4, localAddress: dutlo0Attrs.IPv6, isRR: true}
	nbr3v6 := &bgpNeighbor{as: dutAS, neighborip: otgIsisPort3LoopV6, isV4: false, peerGrp: peerGrpName4, localAddress: dutlo0Attrs.IPv6, isRR: true}
	nbrs := []*bgpNeighbor{nbr1v4, nbr2v4, nbr3v4, nbr1v6, nbr2v6, nbr3v6}

	dutOcRoot := &oc.Root{}
	ni1 := dutOcRoot.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niProto := ni1.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := niProto.GetOrCreateBgp()

	global := bgp.GetOrCreateGlobal()
	global.RouterId = ygot.String(dutlo0Attrs.IPv4)
	global.As = ygot.Uint32(localAs)
	global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
	global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)

	// Note: we have to define the peer group even if we aren't setting any policy because it's
	// invalid OC for the neighbor to be part of a peer group that doesn't exist.
	pg1 := bgp.GetOrCreatePeerGroup(peerGrpName1)
	pg1.PeerAs = ygot.Uint32(ateAS)
	pg1.PeerGroupName = ygot.String(peerGrpName1)

	pg2 := bgp.GetOrCreatePeerGroup(peerGrpName2)
	pg2.PeerAs = ygot.Uint32(dutAS)
	pg2.PeerGroupName = ygot.String(peerGrpName2)

	pg3 := bgp.GetOrCreatePeerGroup(peerGrpName3)
	pg3.PeerAs = ygot.Uint32(ateAS)
	pg3.PeerGroupName = ygot.String(peerGrpName3)

	pg4 := bgp.GetOrCreatePeerGroup(peerGrpName4)
	pg4.PeerAs = ygot.Uint32(dutAS)
	pg4.PeerGroupName = ygot.String(peerGrpName4)

	if deviations.RoutePolicyUnderAFIUnsupported(dut) {
		rp1 := pg1.GetOrCreateApplyPolicy()
		rp1.SetImportPolicy([]string{rplAllowPolicy})
		rp1.SetExportPolicy([]string{rplAllowPolicy})
		rp2 := pg2.GetOrCreateApplyPolicy()
		rp2.SetImportPolicy([]string{rplAllowPolicy})
		rp2.SetExportPolicy([]string{rplAllowPolicy})
		rp3 := pg3.GetOrCreateApplyPolicy()
		rp3.SetImportPolicy([]string{rplAllowPolicy})
		rp3.SetExportPolicy([]string{rplAllowPolicy})
		rp4 := pg4.GetOrCreateApplyPolicy()
		rp4.SetImportPolicy([]string{rplAllowPolicy})
		rp4.SetExportPolicy([]string{rplAllowPolicy})
	} else {
		pg1af4 := pg1.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
		pg1af4.Enabled = ygot.Bool(true)
		pg1rpl4 := pg1af4.GetOrCreateApplyPolicy()
		pg1rpl4.SetImportPolicy([]string{rplAllowPolicy})
		pg1rpl4.SetExportPolicy([]string{rplAllowPolicy})

		pg2af4 := pg2.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
		pg2af4.Enabled = ygot.Bool(true)
		pg2rpl4 := pg2af4.GetOrCreateApplyPolicy()
		pg2rpl4.SetImportPolicy([]string{rplAllowPolicy})
		pg2rpl4.SetExportPolicy([]string{rplAllowPolicy})

		pg3af4 := pg3.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
		pg3af4.Enabled = ygot.Bool(true)
		pg3rpl4 := pg3af4.GetOrCreateApplyPolicy()
		pg3rpl4.SetImportPolicy([]string{rplAllowPolicy})
		pg3rpl4.SetExportPolicy([]string{rplAllowPolicy})

		pg4af4 := pg4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
		pg4af4.Enabled = ygot.Bool(true)
		pg4rpl4 := pg4af4.GetOrCreateApplyPolicy()
		pg4rpl4.SetImportPolicy([]string{rplAllowPolicy})
		pg4rpl4.SetExportPolicy([]string{rplAllowPolicy})
	}

	for _, nbr := range nbrs {
		bgpNbr := bgp.GetOrCreateNeighbor(nbr.neighborip)
		bgpNbr.PeerGroup = ygot.String(nbr.peerGrp)
		bgpNbr.PeerAs = ygot.Uint32(nbr.as)
		bgpNbr.Enabled = ygot.Bool(true)
		if nbr.localAddress != "" {
			bgpNbrT := bgpNbr.GetOrCreateTransport()
			bgpNbrT.LocalAddress = ygot.String(nbr.localAddress)
		}
		if nbr.isRR {
			bgpNbrRouteRef := bgpNbr.GetOrCreateRouteReflector()
			bgpNbrRouteRef.RouteReflectorClient = ygot.Bool(true)
			bgpNbrRouteRef.RouteReflectorClusterId = oc.UnionString(clusterID)
		}
		if nbr.isV4 == true {
			af := bgpNbr.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
			af.Enabled = ygot.Bool(false)
		} else {
			af := bgpNbr.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
			af.Enabled = ygot.Bool(false)
		}
	}
	return niProto
}

func verifyBgpTelemetry(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	var nbrIP = []string{atePort1.IPv4, otgIsisPort2LoopV4, otgIsisPort3LoopV4, atePort1.IPv6, otgIsisPort2LoopV6, otgIsisPort3LoopV6}
	t.Logf("Verifying BGP state.")
	bgpPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()

	for _, nbr := range nbrIP {
		nbrPath := bgpPath.Neighbor(nbr)
		// Get BGP adjacency state.
		t.Logf("Waiting for BGP neighbor to establish...")
		var status *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]
		status, ok := gnmi.Watch(t, dut, nbrPath.SessionState().State(), time.Minute, func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
			state, ok := val.Val()
			return ok && state == oc.Bgp_Neighbor_SessionState_ESTABLISHED
		}).Await(t)
		if !ok {
			fptest.LogQuery(t, "BGP reported state", nbrPath.State(), gnmi.Get(t, dut, nbrPath.State()))
			t.Fatal("No BGP neighbor formed")
		}
		state, _ := status.Val()
		t.Logf("BGP adjacency for %s: %v", nbr, state)
		if want := oc.Bgp_Neighbor_SessionState_ESTABLISHED; state != want {
			t.Errorf("BGP peer %s status got %d, want %d", nbr, state, want)
		}
	}
}

func configureOTG(t *testing.T, otg *otg.OTG) {
	t.Helper()
	config := gosnappi.NewConfig()
	port1 := config.Ports().Add().SetName("port1")
	port2 := config.Ports().Add().SetName("port2")
	port3 := config.Ports().Add().SetName("port3")

	// Port1 Configuration.
	iDut1Dev := config.Devices().Add().SetName(atePort1.Name)
	iDut1Eth := iDut1Dev.Ethernets().Add().SetName(atePort1.Name + ".Eth").SetMac(atePort1.MAC)
	iDut1Eth.Connection().SetPortName(port1.Name())
	iDut1Ipv4 := iDut1Eth.Ipv4Addresses().Add().SetName(atePort1.Name + ".IPv4")
	iDut1Ipv4.SetAddress(atePort1.IPv4).SetGateway(dutPort1.IPv4).SetPrefix(uint32(atePort1.IPv4Len))
	iDut1Ipv6 := iDut1Eth.Ipv6Addresses().Add().SetName(atePort1.Name + ".IPv6")
	iDut1Ipv6.SetAddress(atePort1.IPv6).SetGateway(dutPort1.IPv6).SetPrefix(uint32(atePort1.IPv6Len))

	// Port2 Configuration.
	iDut2Dev := config.Devices().Add().SetName(atePort2.Name)
	iDut2Eth := iDut2Dev.Ethernets().Add().SetName(atePort2.Name + ".Eth").SetMac(atePort2.MAC)
	iDut2Eth.Connection().SetPortName(port2.Name())
	iDut2Ipv4 := iDut2Eth.Ipv4Addresses().Add().SetName(atePort2.Name + ".IPv4")
	iDut2Ipv4.SetAddress(atePort2.IPv4).SetGateway(dutPort2.IPv4).SetPrefix(uint32(atePort2.IPv4Len))
	iDut2Ipv6 := iDut2Eth.Ipv6Addresses().Add().SetName(atePort2.Name + ".IPv6")
	iDut2Ipv6.SetAddress(atePort2.IPv6).SetGateway(dutPort2.IPv6).SetPrefix(uint32(atePort2.IPv6Len))
	// Port2 Loopback Configuration.
	iDut2LoopV4 := iDut2Dev.Ipv4Loopbacks().Add().SetName("Port2LoopV4").SetEthName(iDut2Eth.Name())
	iDut2LoopV4.SetAddress(otgIsisPort2LoopV4)
	iDut2LoopV6 := iDut2Dev.Ipv6Loopbacks().Add().SetName("Port2LoopV6").SetEthName(iDut2Eth.Name())
	iDut2LoopV6.SetAddress(otgIsisPort2LoopV6)

	// Port3 Configuration.
	iDut3Dev := config.Devices().Add().SetName(atePort3.Name)
	iDut3Eth := iDut3Dev.Ethernets().Add().SetName(atePort3.Name + ".Eth").SetMac(atePort3.MAC)
	iDut3Eth.Connection().SetPortName(port3.Name())
	iDut3Ipv4 := iDut3Eth.Ipv4Addresses().Add().SetName(atePort3.Name + ".IPv4")
	iDut3Ipv4.SetAddress(atePort3.IPv4).SetGateway(dutPort3.IPv4).SetPrefix(uint32(atePort3.IPv4Len))
	iDut3Ipv6 := iDut3Eth.Ipv6Addresses().Add().SetName(atePort3.Name + ".IPv6")
	iDut3Ipv6.SetAddress(atePort3.IPv6).SetGateway(dutPort3.IPv6).SetPrefix(uint32(atePort3.IPv6Len))
	// Port3 Loopback Configuration.
	iDut3LoopV4 := iDut3Dev.Ipv4Loopbacks().Add().SetName("Port3LoopV4").SetEthName(iDut3Eth.Name())
	iDut3LoopV4.SetAddress(otgIsisPort3LoopV4)
	iDut3LoopV6 := iDut3Dev.Ipv6Loopbacks().Add().SetName("Port3LoopV6").SetEthName(iDut3Eth.Name())
	iDut3LoopV6.SetAddress(otgIsisPort3LoopV6)

	// ISIS configuration on Port2 for iBGP session establishment.
	isisDut2 := iDut2Dev.Isis().SetName("ISIS2").SetSystemId(otgSysID2)
	isisDut2.Basic().SetIpv4TeRouterId(atePort2.IPv4).SetHostname(isisDut2.Name()).SetLearnedLspFilter(true)
	isisDut2.Interfaces().Add().SetEthName(iDut2Dev.Ethernets().Items()[0].Name()).
		SetName("devIsisInt2").
		SetLevelType(gosnappi.IsisInterfaceLevelType.LEVEL_2).
		SetNetworkType(gosnappi.IsisInterfaceNetworkType.POINT_TO_POINT)

	// ISIS configuration on Port3 for iBGP session establishment.
	isisDut3 := iDut3Dev.Isis().SetName("ISIS3").SetSystemId(otgSysID3)
	isisDut3.Basic().SetIpv4TeRouterId(atePort3.IPv4).SetHostname(isisDut3.Name()).SetLearnedLspFilter(true)
	isisDut3.Interfaces().Add().SetEthName(iDut3Dev.Ethernets().Items()[0].Name()).
		SetName("devIsisInt3").
		SetLevelType(gosnappi.IsisInterfaceLevelType.LEVEL_2).
		SetNetworkType(gosnappi.IsisInterfaceNetworkType.POINT_TO_POINT)

	// Advertise OTG Port2 loopback address via ISIS.
	isisPort2V4 := iDut2Dev.Isis().V4Routes().Add().SetName("ISISPort2V4").SetLinkMetric(10)
	isisPort2V4.Addresses().Add().SetAddress(otgIsisPort2LoopV4).SetPrefix(32)
	isisPort2V6 := iDut2Dev.Isis().V6Routes().Add().SetName("ISISPort2V6").SetLinkMetric(10)
	isisPort2V6.Addresses().Add().SetAddress(otgIsisPort2LoopV6).SetPrefix(uint32(128))

	// Advertise OTG Port3 loopback address via ISIS.
	isisPort3V4 := iDut3Dev.Isis().V4Routes().Add().SetName("ISISPort3V4").SetLinkMetric(10)
	isisPort3V4.Addresses().Add().SetAddress(otgIsisPort3LoopV4).SetPrefix(32)
	isisPort3V6 := iDut3Dev.Isis().V6Routes().Add().SetName("ISISPort3V6").SetLinkMetric(10)
	isisPort3V6.Addresses().Add().SetAddress(otgIsisPort3LoopV6).SetPrefix(uint32(128))

	// eBGP v4 seesion on Port1.
	iDut1Bgp := iDut1Dev.Bgp().SetRouterId(iDut1Ipv4.Address())
	iDut1Bgp4Peer := iDut1Bgp.Ipv4Interfaces().Add().SetIpv4Name(iDut1Ipv4.Name()).Peers().Add().SetName(atePort1.Name + ".BGP4.peer")
	iDut1Bgp4Peer.SetPeerAddress(iDut1Ipv4.Gateway()).SetAsNumber(ateAS).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
	iDut1Bgp4Peer.Capability().SetIpv4UnicastAddPath(true).SetIpv6UnicastAddPath(true)
	iDut1Bgp4Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)
	// eBGP v6 seesion on Port1.
	iDut1Bgp6Peer := iDut1Bgp.Ipv6Interfaces().Add().SetIpv6Name(iDut1Ipv6.Name()).Peers().Add().SetName(atePort1.Name + ".BGP6.peer")
	iDut1Bgp6Peer.SetPeerAddress(iDut1Ipv6.Gateway()).SetAsNumber(ateAS).SetAsType(gosnappi.BgpV6PeerAsType.EBGP)
	iDut1Bgp6Peer.Capability().SetIpv4UnicastAddPath(true).SetIpv6UnicastAddPath(true)
	iDut1Bgp6Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)

	// iBGP - RR client v4 seesion on Port2.
	iDut2Bgp := iDut2Dev.Bgp().SetRouterId(otgIsisPort2LoopV4)
	iDut2Bgp4Peer := iDut2Bgp.Ipv4Interfaces().Add().SetIpv4Name(iDut2LoopV4.Name()).Peers().Add().SetName(atePort2.Name + ".BGP4.peer")
	iDut2Bgp4Peer.SetPeerAddress(dutlo0Attrs.IPv4).SetAsNumber(dutAS).SetAsType(gosnappi.BgpV4PeerAsType.IBGP)
	iDut2Bgp4Peer.Capability().SetIpv4UnicastAddPath(true).SetIpv6UnicastAddPath(true)
	iDut2Bgp4Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)
	// iBGP - RR client v6 seesion on Port2.
	iDut2Bgp6Peer := iDut2Bgp.Ipv6Interfaces().Add().SetIpv6Name(iDut2LoopV6.Name()).Peers().Add().SetName(atePort2.Name + ".BGP6.peer")
	iDut2Bgp6Peer.SetPeerAddress(dutlo0Attrs.IPv6).SetAsNumber(dutAS).SetAsType(gosnappi.BgpV6PeerAsType.IBGP)
	iDut2Bgp6Peer.Capability().SetIpv4UnicastAddPath(true).SetIpv6UnicastAddPath(true)
	iDut2Bgp6Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)

	// iBGP - RR client v4 seesion on Port3.
	iDut3Bgp := iDut3Dev.Bgp().SetRouterId(otgIsisPort3LoopV4)
	iDut3Bgp4Peer := iDut3Bgp.Ipv4Interfaces().Add().SetIpv4Name(iDut3LoopV4.Name()).Peers().Add().SetName(atePort3.Name + ".BGP4.peer")
	iDut3Bgp4Peer.SetPeerAddress(dutlo0Attrs.IPv4).SetAsNumber(dutAS).SetAsType(gosnappi.BgpV4PeerAsType.IBGP)
	iDut3Bgp4Peer.Capability().SetIpv4UnicastAddPath(true).SetIpv6UnicastAddPath(true)
	iDut3Bgp4Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)
	// iBGP - RR client v6 seesion on Port2.
	iDut3Bgp6Peer := iDut3Bgp.Ipv6Interfaces().Add().SetIpv6Name(iDut3LoopV6.Name()).Peers().Add().SetName(atePort3.Name + ".BGP6.peer")
	iDut3Bgp6Peer.SetPeerAddress(dutlo0Attrs.IPv6).SetAsNumber(dutAS).SetAsType(gosnappi.BgpV6PeerAsType.IBGP)
	iDut3Bgp6Peer.Capability().SetIpv4UnicastAddPath(true).SetIpv6UnicastAddPath(true)
	iDut3Bgp6Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)

	// BGP V4 routes from Port2 - 500k unique ipv4 prefixes.
	// These prefixes represent internal subnets and are configured to advertise with
	// same path attributes like AS-PATH and Community.
	bgpNeti1Bgp4PeerRoutes := iDut2Bgp4Peer.V4Routes().Add().SetName(atePort2.Name + ".BGP4.Route")
	bgpNeti1Bgp4PeerRoutes.SetNextHopIpv4Address(iDut2Ipv4.Address()).
		SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).
		SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL).
		Advanced().SetLocalPreference(port2LocPref).SetIncludeLocalPreference(true)
	bgpNeti1AsPath := bgpNeti1Bgp4PeerRoutes.AsPath().SetAsSetMode(gosnappi.BgpAsPathAsSetMode.INCLUDE_AS_SET)
	bgpNeti1AsPath.Segments().Add().SetAsNumbers(asPath).SetType(gosnappi.BgpAsPathSegmentType.AS_SEQ)
	bgpNeti1Bgp4PeerRoutes.Communities().Add().SetAsNumber(65200).SetAsCustom(200).
		SetType(gosnappi.BgpCommunityType.MANUAL_AS_NUMBER)
	bgpNeti1Bgp4PeerRoutes.Addresses().Add().SetAddress(advV4Routes500kPort2).SetPrefix(32).
		SetCount(routeCntV4500k).SetStep(1)

	// BGP V6 routes from Port2 - 200k unique ipv6 prefixes.
	// These prefixes represent internal subnets and are configured to advertise with
	// same path attributes like AS-PATH and Community.
	bgpNeti1Bgp6PeerRoutes := iDut2Bgp6Peer.V6Routes().Add().SetName(atePort2.Name + ".BGP6.Route")
	bgpNeti1Bgp6PeerRoutes.SetNextHopIpv6Address(iDut2Ipv6.Address()).
		SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).
		SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL).
		Advanced().SetLocalPreference(port2LocPref).SetIncludeLocalPreference(true)
	bgpNeti1V6AsPath := bgpNeti1Bgp6PeerRoutes.AsPath().SetAsSetMode(gosnappi.BgpAsPathAsSetMode.INCLUDE_AS_SET)
	bgpNeti1V6AsPath.Segments().Add().SetAsNumbers(asPath).SetType(gosnappi.BgpAsPathSegmentType.AS_SEQ)
	bgpNeti1Bgp6PeerRoutes.Communities().Add().SetAsNumber(65200).SetAsCustom(200).
		SetType(gosnappi.BgpCommunityType.MANUAL_AS_NUMBER)
	bgpNeti1Bgp6PeerRoutes.Addresses().Add().SetAddress(advV6Routes200kPort2).SetPrefix(128).
		SetCount(routeCntV6200k).SetStep(1)

	// Overlapping BGP v4 routers from Port2 - 1M overlapping ipv4 prefixes.
	// These 1M are non RFC1918 or RFC6598 addresses and represent Internet prefixes.
	// These prefixes should be common between the RR clients with different path-attributes
	// for protocol next-hop, AS-Path and community.
	bgpNeti1Bgp4OverLapRoutes := iDut2Bgp4Peer.V4Routes().Add().SetName(atePort2.Name + ".BGP4.Route.Overlap")
	bgpNeti1Bgp4OverLapRoutes.SetNextHopIpv4Address(iDut2Ipv4.Address()).
		SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).
		SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL).
		Advanced().SetLocalPreference(port2LocPref).SetIncludeLocalPreference(true)
	bgpNeti1V4OLAsPath := bgpNeti1Bgp4OverLapRoutes.AsPath().SetAsSetMode(gosnappi.BgpAsPathAsSetMode.INCLUDE_AS_SET)
	bgpNeti1V4OLAsPath.Segments().Add().SetAsNumbers(asPath2).SetType(gosnappi.BgpAsPathSegmentType.AS_SEQ)
	bgpNeti1Bgp4OverLapRoutes.Communities().Add().SetAsNumber(65100).SetAsCustom(200).
		SetType(gosnappi.BgpCommunityType.MANUAL_AS_NUMBER)
	bgpNeti1Bgp4OverLapRoutes.Addresses().Add().
		SetAddress(advV4Routes1M).SetPrefix(32).SetCount(routeCntV41M).SetStep(1)

	// Overlapping BGP v6 routers from Port2 - 600k overlapping ipv6 prefixes.
	// These 1M are non RFC1918 or RFC6598 addresses and represent Internet prefixes.
	// These prefixes should be common between the RR clients with different path-attributes
	// for protocol next-hop, AS-Path and community.
	bgpNeti1Bgp6OverLapRoutes := iDut2Bgp6Peer.V6Routes().Add().SetName(atePort2.Name + ".BGP6.Route.Overlap")
	bgpNeti1Bgp6OverLapRoutes.SetNextHopIpv6Address(iDut2Ipv6.Address()).
		SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).
		SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL).
		Advanced().SetLocalPreference(port2LocPref).SetIncludeLocalPreference(true)
	bgpNeti1V6OLAsPath := bgpNeti1Bgp6OverLapRoutes.AsPath().SetAsSetMode(gosnappi.BgpAsPathAsSetMode.INCLUDE_AS_SET)
	bgpNeti1V6OLAsPath.Segments().Add().SetAsNumbers(asPath2).SetType(gosnappi.BgpAsPathSegmentType.AS_SEQ)
	bgpNeti1Bgp6OverLapRoutes.Communities().Add().SetAsNumber(65100).SetAsCustom(200).
		SetType(gosnappi.BgpCommunityType.MANUAL_AS_NUMBER)
	bgpNeti1Bgp6OverLapRoutes.Addresses().Add().
		SetAddress(advV6Routes600k).SetPrefix(128).SetCount(routeCntV6600k).SetStep(1)

	// BGP V4 routes from Port3 - 500k unique ipv4 prefixes.
	// These prefixes represent internal subnets and are configured to advertise with
	// same path attributes like AS-PATH and Community.
	bgpNeti2Bgp4PeerRoutes := iDut3Bgp4Peer.V4Routes().Add().SetName(atePort3.Name + ".BGP4.Route")
	bgpNeti2Bgp4PeerRoutes.SetNextHopIpv4Address(iDut3Ipv4.Address()).
		SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).
		SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL).
		Advanced().SetLocalPreference(port3LocPref).SetIncludeLocalPreference(true)
	bgpNeti2AsPath := bgpNeti2Bgp4PeerRoutes.AsPath().SetAsSetMode(gosnappi.BgpAsPathAsSetMode.INCLUDE_AS_SET)
	bgpNeti2AsPath.Segments().Add().SetAsNumbers(asPath).SetType(gosnappi.BgpAsPathSegmentType.AS_SEQ)
	bgpNeti2Bgp4PeerRoutes.Communities().Add().SetAsNumber(65200).SetAsCustom(200).
		SetType(gosnappi.BgpCommunityType.MANUAL_AS_NUMBER)
	bgpNeti2Bgp4PeerRoutes.Addresses().Add().
		SetAddress(advV4Routes500kPort3).SetPrefix(32).SetCount(routeCntV4500k).SetStep(1)

	// BGP V6 routes from Port3 - 200k unique ipv6 prefixes.
	// These prefixes represent internal subnets and are configured to advertise with
	// same path attributes like AS-PATH and Community.
	bgpNeti2Bgp6PeerRoutes := iDut3Bgp6Peer.V6Routes().Add().SetName(atePort3.Name + ".BGP6.Route")
	bgpNeti2Bgp6PeerRoutes.SetNextHopIpv6Address(iDut3Ipv6.Address()).
		SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).
		SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL).
		Advanced().SetLocalPreference(port3LocPref).SetIncludeLocalPreference(true)
	bgpNeti2V6AsPath := bgpNeti2Bgp6PeerRoutes.AsPath().SetAsSetMode(gosnappi.BgpAsPathAsSetMode.INCLUDE_AS_SET)
	bgpNeti2V6AsPath.Segments().Add().SetAsNumbers(asPath).SetType(gosnappi.BgpAsPathSegmentType.AS_SEQ)
	bgpNeti2Bgp6PeerRoutes.Communities().Add().SetAsNumber(65200).SetAsCustom(200).
		SetType(gosnappi.BgpCommunityType.MANUAL_AS_NUMBER)
	bgpNeti2Bgp6PeerRoutes.Addresses().Add().
		SetAddress(advV6Routes200kPort3).SetPrefix(128).SetCount(routeCntV6200k).SetStep(1)

	// Port 3 overlapping routes - 1M overlapping ipv4 prefixes.
	// These 1M are non RFC1918 or RFC6598 addresses and represent Internet prefixes.
	// These prefixes should be common between the RR clients with different path-attributes
	// for protocol next-hop, AS-Path and community.
	bgpNeti2Bgp4OverLapRoutes := iDut3Bgp4Peer.V4Routes().Add().SetName(atePort3.Name + ".BGP4.Route.Overlap")
	bgpNeti2Bgp4OverLapRoutes.SetNextHopIpv4Address(iDut3Ipv4.Address()).
		SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).
		SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL).
		Advanced().SetLocalPreference(port3LocPref).SetIncludeLocalPreference(true)
	bgpNeti2V4OLAsPath := bgpNeti2Bgp4OverLapRoutes.AsPath().SetAsSetMode(gosnappi.BgpAsPathAsSetMode.INCLUDE_AS_SET)
	bgpNeti2V4OLAsPath.Segments().Add().SetAsNumbers(asPath3).SetType(gosnappi.BgpAsPathSegmentType.AS_SEQ)
	bgpNeti2Bgp4OverLapRoutes.Communities().Add().SetAsNumber(65100).SetAsCustom(300).
		SetType(gosnappi.BgpCommunityType.MANUAL_AS_NUMBER)
	bgpNeti2Bgp4OverLapRoutes.Addresses().Add().SetAddress(advV4Routes1M).SetPrefix(32).
		SetCount(routeCntV41M).SetStep(1)

	// Overlapping BGP v6 routers from Port3 - 600k overlapping ipv6 prefixes.
	// These 1M are non RFC1918 or RFC6598 addresses and represent Internet prefixes.
	// These prefixes should be common between the RR clients with different path-attributes
	// for protocol next-hop, AS-Path and community.
	bgpNeti2Bgp6OverLapRoutes := iDut3Bgp6Peer.V6Routes().Add().SetName(atePort3.Name + ".BGP6.Route.Overlap")
	bgpNeti2Bgp6OverLapRoutes.SetNextHopIpv6Address(iDut3Ipv6.Address()).
		SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).
		SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL).
		Advanced().SetLocalPreference(port3LocPref).SetIncludeLocalPreference(true)
	bgpNeti2V6OLAsPath := bgpNeti2Bgp6OverLapRoutes.AsPath().SetAsSetMode(gosnappi.BgpAsPathAsSetMode.INCLUDE_AS_SET)
	bgpNeti2V6OLAsPath.Segments().Add().SetAsNumbers(asPath3).SetType(gosnappi.BgpAsPathSegmentType.AS_SEQ)
	bgpNeti2Bgp6OverLapRoutes.Communities().Add().SetAsNumber(65100).SetAsCustom(300).
		SetType(gosnappi.BgpCommunityType.MANUAL_AS_NUMBER)
	bgpNeti2Bgp6OverLapRoutes.Addresses().Add().SetAddress(advV6Routes600k).SetPrefix(128).
		SetCount(routeCntV6600k).SetStep(1)

	// BGP V4 routes from Port1 -  1M overlapping ipv4 prefixes.
	// These 1M are non RFC1918 or RFC6598 addresses and represent Internet prefixes.
	// These prefixes should be common between the RR clients with different path-attributes
	// for protocol next-hop, AS-Path and community.
	bgpNeti3Bgp4PeerRoutes := iDut1Bgp4Peer.V4Routes().Add().SetName(atePort1.Name + ".BGP4.Route")
	bgpNeti3Bgp4PeerRoutes.SetNextHopIpv4Address(iDut1Ipv4.Address()).
		SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).
		SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)
	bgpNeti3Bgp4PeerRoutes.Communities().Add().SetAsNumber(65010).SetAsCustom(100).
		SetType(gosnappi.BgpCommunityType.MANUAL_AS_NUMBER)
	bgpNeti3Bgp4PeerRoutes.Addresses().Add().
		SetAddress(advV4Routes1M).SetPrefix(32).SetCount(routeCntV41M).SetStep(1)

	// BGP V6 routes from Port1 - 600k overlapping ipv6 prefixes.
	// These 1M are non RFC1918 or RFC6598 addresses and represent Internet prefixes.
	// These prefixes should be common between the RR clients with different path-attributes
	// for protocol next-hop, AS-Path and community.
	bgpNeti3Bgp6PeerRoutes := iDut1Bgp6Peer.V6Routes().Add().SetName(atePort1.Name + ".BGP6.Route")
	bgpNeti3Bgp6PeerRoutes.SetNextHopIpv6Address(iDut1Ipv6.Address()).
		SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).
		SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL)
	bgpNeti3Bgp6PeerRoutes.Communities().Add().SetAsNumber(65010).SetAsCustom(100).
		SetType(gosnappi.BgpCommunityType.MANUAL_AS_NUMBER)
	bgpNeti3Bgp6PeerRoutes.Addresses().Add().
		SetAddress(advV6Routes600k).SetPrefix(128).SetCount(routeCntV6600k).SetStep(1)

	t.Logf("Pushing config to OTG and starting protocols...")
	otg.PushConfig(t, config)
	time.Sleep(30 * time.Second)
	otg.StartProtocols(t)
	time.Sleep(30 * time.Second)
}

func verifyBGPCapabilities(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	t.Log("Verifying BGP capabilities.")
	var nbrIP = []string{atePort1.IPv4, otgIsisPort2LoopV4, otgIsisPort3LoopV4, atePort1.IPv6, otgIsisPort2LoopV6, otgIsisPort3LoopV6}
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	for _, nbr := range nbrIP {
		nbrPath := statePath.Neighbor(nbr)
		capabilities := map[oc.E_BgpTypes_BGP_CAPABILITY]bool{
			oc.BgpTypes_BGP_CAPABILITY_ROUTE_REFRESH: false,
			oc.BgpTypes_BGP_CAPABILITY_MPBGP:         false,
			oc.BgpTypes_BGP_CAPABILITY_ASN32:         false,
		}
		for _, cap := range gnmi.Get(t, dut, nbrPath.SupportedCapabilities().State()) {
			capabilities[cap] = true
		}
		for cap, present := range capabilities {
			if !present {
				t.Errorf("Capability not reported: %v", cap)
			}
		}
	}
}

func verifyBGPRRCapabilities(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	t.Log("Verifying BGP Route Reflector capabilities.")

	nbr1v4 := &validateBgpNbr{localAddress: dutlo0Attrs.IPv4, nbrIP: otgIsisPort2LoopV4}
	nbr2v4 := &validateBgpNbr{localAddress: dutlo0Attrs.IPv4, nbrIP: otgIsisPort3LoopV4}
	nbr1v6 := &validateBgpNbr{localAddress: dutlo0Attrs.IPv6, nbrIP: otgIsisPort2LoopV6}
	nbr2v6 := &validateBgpNbr{localAddress: dutlo0Attrs.IPv6, nbrIP: otgIsisPort3LoopV6}

	nbrs := []*validateBgpNbr{nbr1v4, nbr2v4, nbr1v6, nbr2v6}
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()

	for _, nbr := range nbrs {
		t.Logf("Verifying Route Reflector client capabilities for %v", nbr.nbrIP)
		nbrPath := statePath.Neighbor(nbr.nbrIP)
		nbrLocalAddress := gnmi.Get(t, dut, nbrPath.Transport().LocalAddress().State())
		if nbrLocalAddress != nbr.localAddress {
			t.Errorf("Local address mismatch, got %v and want %v", nbrLocalAddress, nbr.localAddress)
		}
		nbrPeerType := gnmi.Get(t, dut, nbrPath.PeerType().State())
		if nbrPeerType != oc.Bgp_PeerType_INTERNAL {
			t.Errorf("Neighbor Peer Type mismatch, got %v and want oc.Bgp_PeerType_INTERNAL", nbrPeerType)
		}
		routeRefClustID := gnmi.Get(t, dut, nbrPath.RouteReflector().RouteReflectorClusterId().State())
		if routeRefClustID != oc.UnionString(clusterID) {
			t.Errorf("Route reflector cluster ID mismatch, got %v and want %v", routeRefClustID, clusterID)
		}
		routeRefClientState := gnmi.Get(t, dut, nbrPath.RouteReflector().RouteReflectorClient().State())
		if routeRefClientState != true {
			t.Errorf("Route Reflector Client state mismatch, got %v and want true.", routeRefClientState)
		}
	}
}

func verifyEBGPCapabilities(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	t.Log("Verifying eBGP capabilities.")

	nbr1v4 := &validateBgpNbr{localAddress: dutPort1.IPv4, nbrIP: atePort1.IPv4}
	nbr1v6 := &validateBgpNbr{localAddress: dutPort1.IPv6, nbrIP: atePort1.IPv6}

	nbrs := []*validateBgpNbr{nbr1v4, nbr1v6}
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()

	for _, nbr := range nbrs {
		nbrPath := statePath.Neighbor(nbr.nbrIP)
		nbrLocalAddress := gnmi.Get(t, dut, nbrPath.Transport().LocalAddress().State())
		if nbrLocalAddress != nbr.localAddress {
			t.Errorf("RR client Local address mismatch, got %v and want %v", nbrLocalAddress, nbr.localAddress)
		}
		nbrPeerType := gnmi.Get(t, dut, nbrPath.PeerType().State())
		if nbrPeerType != oc.Bgp_PeerType_EXTERNAL {
			t.Errorf("RR client peer type mismatch, got %v and want oc.Bgp_PeerType_EXTERNAL.", nbrPeerType)
		}
	}
}

type validatePathAttribute struct {
	nbr       string
	prefix    string
	nexthop   string
	locpref   uint32
	asPath    []uint32
	community []oc.UnionString
	isV4      bool
}

func verifyBGPPathAttributes(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	t.Log("Verifying BGP route path attributes.")

	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	// Path attributes for the prefixes learnt via iBGP RR clients.
	pref1 := &validatePathAttribute{nbr: otgIsisPort2LoopV4, prefix: advV4Routes500kPort2, isV4: true, nexthop: atePort2.IPv4, locpref: port2LocPref, asPath: asPath, community: commAttr}
	pref2 := &validatePathAttribute{nbr: otgIsisPort3LoopV4, prefix: advV4Routes500kPort3, isV4: true, nexthop: atePort3.IPv4, locpref: port3LocPref, asPath: asPath, community: commAttr}
	pref3 := &validatePathAttribute{nbr: otgIsisPort2LoopV6, prefix: advV6Routes200kPort2, isV4: false, nexthop: atePort2.IPv6, locpref: port2LocPref, asPath: asPath, community: commAttr}
	pref4 := &validatePathAttribute{nbr: otgIsisPort3LoopV6, prefix: advV6Routes200kPort3, isV4: false, nexthop: atePort3.IPv6, locpref: port3LocPref, asPath: asPath, community: commAttr}

	// Path attributes for the prefxies learnt via eBGP peer.
	pref5 := &validatePathAttribute{nbr: atePort1.IPv4, prefix: advV4Routes1M, isV4: true, nexthop: atePort1.IPv4, locpref: 0, asPath: []uint32{ateAS}, community: commAttrExt}
	pref6 := &validatePathAttribute{nbr: atePort1.IPv6, prefix: advV6Routes600k, isV4: false, nexthop: atePort1.IPv6, locpref: 0, asPath: []uint32{ateAS}, community: commAttrExt}

	prefList := []*validatePathAttribute{pref1, pref2, pref3, pref4, pref5, pref6}
	rib := statePath.Rib()
	var attIndex uint64

	for _, pref := range prefList {
		if pref.isV4 {
			prefixPathV4 := rib.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Ipv4Unicast().
				Neighbor(pref.nbr).AdjRibInPre().Route(pref.prefix, 0).AttrIndex()
			attIndex = gnmi.Get(t, dut, prefixPathV4.State())
		} else {
			prefixPathV6 := rib.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Ipv6Unicast().
				Neighbor(pref.nbr).AdjRibInPre().Route(pref.prefix, 0).AttrIndex()
			attIndex = gnmi.Get(t, dut, prefixPathV6.State())
		}

		gotLocPref := gnmi.Get(t, dut, rib.AttrSet(attIndex).LocalPref().State())
		gotNexthop := gnmi.Get(t, dut, rib.AttrSet(attIndex).NextHop().State())
		gotAsPath := gnmi.Get(t, dut, rib.AttrSet(attIndex).AsSegmentMap().State())
		// Below code will be un-commented once ixia issue is fixed.
		// https://github.com/open-traffic-generator/fp-testbed-juniper/issues/33
		/* gotCommAtt := gnmi.Get(t, dut, rib.Community(attIndex).State())
		gotCommList := gotCommAtt.GetCommunity() */

		if gotLocPref != pref.locpref {
			t.Errorf("Obtained and configure local preference mismatch: got %v, want %v", gotLocPref, pref.locpref)
		}
		if gotNexthop != pref.nexthop {
			t.Errorf("Obtained and configure nexthop mismatch: got %v, want %v", gotNexthop, pref.nexthop)
		}
		for _, as := range gotAsPath {
			if !cmp.Equal(as.Member, pref.asPath) {
				t.Errorf("Obtained and configure AS PATH mismatch: got %v, want %v", gotAsPath, pref.asPath)
			}
		}

		// Below code will be un-commented once ixia issue is fixed.
		// https://github.com/open-traffic-generator/fp-testbed-juniper/issues/33
		/* for _, comm := range gotCommList {
			if !cmp.Equal(comm, pref.community) {
				t.Errorf("Obtained and configure AS PATH mismatch: got %v, want %v", gotAsPath, pref.asPath)
			}
		} */
	}
}

func verifyPrefixesTelemetry(t *testing.T, dut *ondatra.DUTDevice, nbr string, wantInstalled, wantRx, wantSent uint32, isV4 bool) {
	t.Helper()
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	var prefixPath *netinstbgp.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_PrefixesPath
	if isV4 {
		prefixPath = statePath.Neighbor(nbr).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Prefixes()
	} else {
		prefixPath = statePath.Neighbor(nbr).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Prefixes()
	}
	if gotInstalled := gnmi.Get(t, dut, prefixPath.Installed().State()); gotInstalled != wantInstalled {
		t.Errorf("Installed prefixes mismatch: got %v, want %v", gotInstalled, wantInstalled)
	}
	if !deviations.MissingPrePolicyReceivedRoutes(dut) {
		if gotRx := gnmi.Get(t, dut, prefixPath.ReceivedPrePolicy().State()); gotRx != wantRx {
			t.Errorf("Received prefixes mismatch: got %v, want %v", gotRx, wantRx)
		}
	}
	if gotSent := gnmi.Get(t, dut, prefixPath.Sent().State()); gotSent != wantSent {
		t.Errorf("Sent prefixes mismatch: got %v, want %v", gotSent, wantSent)
	}
	t.Logf("Prefix telemetry validation passed on DUT for peer %v", nbr)
}

type bgpNeighbor struct {
	as           uint32
	neighborip   string
	isV4         bool
	peerGrp      string
	localAddress string
	isRR         bool
}

type validateBgpNbr struct {
	localAddress string
	nbrIP        string
}

func configureISIS(t *testing.T, dut *ondatra.DUTDevice, intfName []string, dutAreaAddress, dutSysID string) {
	t.Helper()
	d := &oc.Root{}
	dutConfIsisPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance)
	netInstance := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	prot := netInstance.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance)
	prot.Enabled = ygot.Bool(true)
	isis := prot.GetOrCreateIsis()
	globalISIS := isis.GetOrCreateGlobal()
	globalISIS.LevelCapability = oc.Isis_LevelType_LEVEL_2
	globalISIS.Net = []string{fmt.Sprintf("%v.%v.00", dutAreaAddress, dutSysID)}
	globalISIS.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	if deviations.ISISInstanceEnabledRequired(dut) {
		globalISIS.Instance = ygot.String(isisInstance)
	}
	isisLevel2 := isis.GetOrCreateLevel(2)
	isisLevel2.MetricStyle = oc.Isis_MetricStyle_WIDE_METRIC
	if deviations.ISISLevelEnabled(dut) {
		isisLevel2.Enabled = ygot.Bool(true)
	}

	for _, intf := range intfName {
		isisIntf := isis.GetOrCreateInterface(intf)
		isisIntf.Enabled = ygot.Bool(true)
		isisIntf.CircuitType = oc.Isis_CircuitType_POINT_TO_POINT
		isisIntfLevel := isisIntf.GetOrCreateLevel(2)
		isisIntfLevel.Enabled = ygot.Bool(true)
		isisIntfLevelAfi := isisIntfLevel.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST)
		isisIntfLevelAfi.Metric = ygot.Uint32(200)
		isisIntfLevelAfi.Enabled = ygot.Bool(true)
	}
	gnmi.Replace(t, dut, dutConfIsisPath.Config(), prot)
}

func verifyISISTelemetry(t *testing.T, dut *ondatra.DUTDevice, dutIntf []string) {
	t.Helper()
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance).Isis()
	for _, intfName := range dutIntf {
		if deviations.ExplicitInterfaceInDefaultVRF(dut) {
			intfName = intfName + ".0"
		}
		nbrPath := statePath.Interface(intfName)
		query := nbrPath.LevelAny().AdjacencyAny().AdjacencyState().State()
		_, ok := gnmi.WatchAll(t, dut, query, time.Minute, func(val *ygnmi.Value[oc.E_Isis_IsisInterfaceAdjState]) bool {
			state, present := val.Val()
			return present && state == oc.Isis_IsisInterfaceAdjState_UP
		}).Await(t)
		if !ok {
			t.Logf("IS-IS state on %v has no adjacencies", intfName)
			t.Fatal("No IS-IS adjacencies reported.")
		}
	}
}

// TestBGPRouteReflectorCapabilities is to Validate BGP route reflector capabilities.
// Also to ensure functionality of different OC paths for "supported-capabilities",
// "BGP peer-type", "BGP Neighbor details" and "BGP transport session parameters".
// It also validates that DUT advertises all the IBGP learnt routes to the EBGP peer,
// EBGP learnt routes to the IBGP peers and advertise routes learnt from each of the RRC
// to the other.
func TestBGPRouteReflectorCapabilities(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	t.Run("Configure DUT interfaces", func(t *testing.T) {
		configureDUT(t, dut)
	})

	t.Run("Configure DEFAULT network instance", func(t *testing.T) {
		fptest.ConfigureDefaultNetworkInstance(t, dut)
	})

	t.Run("Configure ISIS on DUT", func(t *testing.T) {
		dutIsisIntfNames := []string{dut.Port(t, "port2").Name(), dut.Port(t, "port3").Name(), loopbackIntfName}
		configureISIS(t, dut, dutIsisIntfNames, dutAreaAddress, dutSysID)
	})

	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	t.Run("Configure BGP Neighbors", func(t *testing.T) {
		configureRoutePolicy(t, dut, "ALLOW", oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)
		gnmi.Delete(t, dut, dutConfPath.Config())
		dutConf := bgpCreateNbr(dutAS, ateAS, dut)
		gnmi.Replace(t, dut, dutConfPath.Config(), dutConf)
		fptest.LogQuery(t, "DUT BGP Config", dutConfPath.Config(), gnmi.Get(t, dut, dutConfPath.Config()))
	})

	otg := ate.OTG()
	t.Run("Configure OTG", func(t *testing.T) {
		configureOTG(t, otg)
		time.Sleep(3 * time.Minute)
	})

	t.Run("Verify port status on DUT", func(t *testing.T) {
		verifyPortsUp(t, dut.Device)
	})

	t.Run("Verify ISIS session status on DUT", func(t *testing.T) {
		dutIsisPeerIntf := []string{dut.Port(t, "port2").Name(), dut.Port(t, "port3").Name()}
		verifyISISTelemetry(t, dut, dutIsisPeerIntf)
	})

	t.Run("Verify BGP session telemetry", func(t *testing.T) {
		verifyBgpTelemetry(t, dut)
	})
	t.Run("Verify BGP capabilities", func(t *testing.T) {
		verifyBGPCapabilities(t, dut)
	})
	t.Run("Verify BGP Route Reflector capabilities", func(t *testing.T) {
		verifyBGPRRCapabilities(t, dut)
	})

	t.Run("Verify eBGP peer capabilities", func(t *testing.T) {
		verifyEBGPCapabilities(t, dut)
	})

	if !deviations.BGPRibOcPathUnsupported(dut) {
		t.Run("Verify BGP route path attributes for iBGP RR routes", func(t *testing.T) {
			verifyBGPPathAttributes(t, dut)
		})
	}

	t.Run("Verify prefix telemetry on DUT for all iBGP and eBGP peers", func(t *testing.T) {
		// Prefix telemetry validation for eBGP v4 Peer.
		verifyPrefixesTelemetry(t, dut, atePort1.IPv4, routeCntV41M, routeCntV41M, (routeCntV4500k + routeCntV4500k), v4Prefixes)
		// Prefix telemetry validation for iBGP v4 Peer - Route reflector client #1.
		verifyPrefixesTelemetry(t, dut, otgIsisPort2LoopV4, routeCntV4500k, (routeCntV4500k + routeCntV41M), (routeCntV4500k + routeCntV41M), v4Prefixes)
		// Prefix telemetry validation for iBGP v4 Peer - Route reflector client #2.
		verifyPrefixesTelemetry(t, dut, otgIsisPort3LoopV4, routeCntV4500k, (routeCntV4500k + routeCntV41M), (routeCntV4500k + routeCntV41M), v4Prefixes)
		// Prefix telemetry validation for eBGP v6 Peer.
		verifyPrefixesTelemetry(t, dut, atePort1.IPv6, routeCntV6600k, routeCntV6600k, (routeCntV6200k + routeCntV6200k), !v4Prefixes)
		// Prefix telemetry validation for iBGP v6 Peer - Route reflector client #1.
		verifyPrefixesTelemetry(t, dut, otgIsisPort2LoopV6, routeCntV6200k, (routeCntV6200k + routeCntV6600k), (routeCntV6200k + routeCntV6600k), !v4Prefixes)
		// Prefix telemetry validation for iBGP v6 Peer - Route reflector client #2.
		verifyPrefixesTelemetry(t, dut, otgIsisPort3LoopV6, routeCntV6200k, (routeCntV6200k + routeCntV6600k), (routeCntV6200k + routeCntV6600k), !v4Prefixes)
	})
}
