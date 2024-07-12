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

package bgp_default_policies_test

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
	dutAS                = 65501
	ateAS                = 65502
	plenIPv4             = 30
	plenIPv6             = 126
	dutAreaAddress       = "49.0001"
	dutSysID             = "1920.0000.2001"
	otgSysID2            = "640000000001"
	isisInstance         = "DEFAULT"
	otgIsisPort2LoopV4   = "203.0.113.10"
	otgIsisPort2LoopV6   = "2001:db8::203:0:113:10"
	v4Prefixes           = true
	rejectAll            = "REJECT-ALL"
	rejectRoute          = oc.RoutingPolicy_DefaultPolicyType_REJECT_ROUTE
	acceptRoute          = oc.RoutingPolicy_DefaultPolicyType_ACCEPT_ROUTE
	ebgpImportIPv4       = "EBGP-IMPORT-IPV4"
	ebgpImportIPv6       = "EBGP-IMPORT-IPV6"
	ebgpExportIPv4       = "EBGP-EXPORT-IPV4"
	ebgpExportIPv6       = "EBGP-EXPORT-IPV6"
	ibgpImportIPv4       = "IBGP-IMPORT-IPV4"
	ibgpImportIPv6       = "IBGP-IMPORT-IPV6"
	ibgpExportIPv4       = "IBGP-EXPORT-IPV4"
	ibgpExportIPv6       = "IBGP-EXPORT-IPV6"
	maskLengthRange32    = "32..32"
	maskLengthRange128   = "128..128"
	maskLen32            = "32"
	maskLen128           = "128"
	ipv4Prefix1          = "198.51.100.1"
	ipv4Prefix2          = "198.51.100.2"
	ipv4Prefix3          = "198.51.100.3"
	ipv4Prefix4          = "198.51.100.4"
	ipv4Prefix5          = "198.51.100.5"
	ipv4Prefix6          = "198.51.100.6"
	ipv4Prefix7          = "198.51.100.7"
	ipv4Prefix8          = "198.51.100.8"
	ipv6Prefix1          = "2001:DB8:2::1"
	ipv6Prefix2          = "2001:DB8:2::2"
	ipv6Prefix3          = "2001:DB8:2::3"
	ipv6Prefix4          = "2001:DB8:2::4"
	ipv6Prefix5          = "2001:DB8:2::5"
	ipv6Prefix6          = "2001:DB8:2::6"
	ipv6Prefix7          = "2001:DB8:2::7"
	ipv6Prefix8          = "2001:DB8:2::8"
	maskLenExact         = "exact"
	defaultStatementOnly = true
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
	dutlo0Attrs = attrs.Attributes{
		Desc:    "Loopback ip",
		IPv4:    "203.0.113.1",
		IPv6:    "2001:db8::203:0:113:1",
		IPv4Len: 32,
		IPv6Len: 128,
	}
	ebgpNbrV4        = &bgpNbrList{nbrAddr: atePort1.IPv4, isV4: true, afiSafi: oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST}
	ebgpNbrV6        = &bgpNbrList{nbrAddr: atePort1.IPv6, isV4: false, afiSafi: oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST}
	ibgpNbrV4        = &bgpNbrList{nbrAddr: otgIsisPort2LoopV4, isV4: true, afiSafi: oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST}
	ibgpNbrV6        = &bgpNbrList{nbrAddr: otgIsisPort2LoopV6, isV4: false, afiSafi: oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST}
	loopbackIntfName string
)

func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	dc := gnmi.OC()
	i1 := dutPort1.NewOCInterface(dut.Port(t, "port1").Name(), dut)
	gnmi.Replace(t, dut, dc.Interface(i1.GetName()).Config(), i1)

	i2 := dutPort2.NewOCInterface(dut.Port(t, "port2").Name(), dut)
	gnmi.Replace(t, dut, dc.Interface(i2.GetName()).Config(), i2)

	loopbackIntfName = netutil.LoopbackInterface(t, dut, 0)
	lo0 := gnmi.OC().Interface(loopbackIntfName).Subinterface(0)
	ipv4Addrs := gnmi.LookupAll(t, dut, lo0.Ipv4().AddressAny().State())
	ipv6Addrs := gnmi.LookupAll(t, dut, lo0.Ipv6().AddressAny().State())
	if len(ipv4Addrs) == 0 && len(ipv6Addrs) == 0 {
		loop1 := dutlo0Attrs.NewOCInterface(loopbackIntfName, dut)
		loop1.Type = oc.IETFInterfaces_InterfaceType_softwareLoopback
		gnmi.Update(t, dut, dc.Interface(loopbackIntfName).Config(), loop1)
	} else {
		v4, ok := ipv4Addrs[0].Val()
		if ok {
			dutlo0Attrs.IPv4 = v4.GetIp()
		}
		v6, ok := ipv6Addrs[0].Val()
		if ok {
			dutlo0Attrs.IPv6 = v6.GetIp()
		}
		t.Logf("Got DUT IPv4 loopback address: %v", dutlo0Attrs.IPv4)
		t.Logf("Got DUT IPv6 loopback address: %v", dutlo0Attrs.IPv6)
	}

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, dut.Port(t, "port1"))
		fptest.SetPortSpeed(t, dut.Port(t, "port2"))
	}
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, dut.Port(t, "port1").Name(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, dut.Port(t, "port2").Name(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, loopbackIntfName, deviations.DefaultNetworkInstance(dut), 0)
	}

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

func configurePrefixMatchPolicy(t *testing.T, dut *ondatra.DUTDevice, prefixSet, prefixSubnetRange, maskLen string, ipPrefixSet []string) *oc.RoutingPolicy {
	d := &oc.Root{}
	rp := d.GetOrCreateRoutingPolicy()
	pset := rp.GetOrCreateDefinedSets().GetOrCreatePrefixSet(prefixSet)
	for _, pref := range ipPrefixSet {
		pset.GetOrCreatePrefix(pref+"/"+maskLen, prefixSubnetRange)
		mode := oc.PrefixSet_Mode_IPV4
		if maskLen == maskLen128 {
			mode = oc.PrefixSet_Mode_IPV6
		}
		if !deviations.SkipPrefixSetMode(dut) {
			pset.SetMode(mode)
		}
	}

	pdef := rp.GetOrCreatePolicyDefinition(prefixSet)
	stmt5, err := pdef.AppendNewStatement("10")
	if err != nil {
		t.Fatal(err)
	}
	stmt5.GetOrCreateConditions().GetOrCreateMatchPrefixSet().PrefixSet = ygot.String(prefixSet)
	stmt5.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
	return rp
}

func bgpCreateNbr(localAs, peerAs uint32, dut *ondatra.DUTDevice) *oc.NetworkInstance_Protocol {
	nbr1v4 := &bgpNeighbor{as: ateAS, neighborip: atePort1.IPv4, isV4: true, peerGrp: peerGrpName1}
	nbr2v4 := &bgpNeighbor{as: dutAS, neighborip: otgIsisPort2LoopV4, isV4: true, peerGrp: peerGrpName2, localAddress: dutlo0Attrs.IPv4}
	nbr1v6 := &bgpNeighbor{as: ateAS, neighborip: atePort1.IPv6, isV4: false, peerGrp: peerGrpName3}
	nbr2v6 := &bgpNeighbor{as: dutAS, neighborip: otgIsisPort2LoopV6, isV4: false, peerGrp: peerGrpName4, localAddress: dutlo0Attrs.IPv6}
	nbrs := []*bgpNeighbor{nbr1v4, nbr2v4, nbr1v6, nbr2v6}

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

	for _, nbr := range nbrs {
		bgpNbr := bgp.GetOrCreateNeighbor(nbr.neighborip)
		bgpNbr.PeerGroup = ygot.String(nbr.peerGrp)
		bgpNbr.PeerAs = ygot.Uint32(nbr.as)
		bgpNbr.Enabled = ygot.Bool(true)
		if nbr.localAddress != "" {
			bgpNbrT := bgpNbr.GetOrCreateTransport()
			bgpNbrT.LocalAddress = ygot.String(nbr.localAddress)
		}
		if nbr.isV4 == true {
			af4 := bgpNbr.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
			af4.Enabled = ygot.Bool(true)
			af6 := bgpNbr.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
			af6.Enabled = ygot.Bool(false)
		} else {
			af4 := bgpNbr.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
			af4.Enabled = ygot.Bool(false)
			af6 := bgpNbr.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
			af6.Enabled = ygot.Bool(true)
		}
	}
	return niProto
}

func verifyBgpTelemetry(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	var nbrIP = []string{atePort1.IPv4, otgIsisPort2LoopV4, atePort1.IPv6, otgIsisPort2LoopV6}
	t.Logf("Verifying BGP state.")
	bgpPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()

	for _, nbr := range nbrIP {
		nbrPath := bgpPath.Neighbor(nbr)
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

	// ISIS configuration on Port2 for iBGP session establishment.
	isisDut2 := iDut2Dev.Isis().SetName("ISIS2").SetSystemId(otgSysID2)
	isisDut2.Basic().SetIpv4TeRouterId(atePort2.IPv4).SetHostname(isisDut2.Name()).SetLearnedLspFilter(true)
	isisDut2.Interfaces().Add().SetEthName(iDut2Dev.Ethernets().Items()[0].Name()).
		SetName("devIsisInt2").
		SetLevelType(gosnappi.IsisInterfaceLevelType.LEVEL_2).
		SetNetworkType(gosnappi.IsisInterfaceNetworkType.POINT_TO_POINT)

	// Advertise OTG Port2 loopback address via ISIS.
	isisPort2V4 := iDut2Dev.Isis().V4Routes().Add().SetName("ISISPort2V4").SetLinkMetric(10)
	isisPort2V4.Addresses().Add().SetAddress(otgIsisPort2LoopV4).SetPrefix(32)
	isisPort2V6 := iDut2Dev.Isis().V6Routes().Add().SetName("ISISPort2V6").SetLinkMetric(10)
	isisPort2V6.Addresses().Add().SetAddress(otgIsisPort2LoopV6).SetPrefix(uint32(128))

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

	// iBGP - v4 seesion on Port2.
	iDut2Bgp := iDut2Dev.Bgp().SetRouterId(otgIsisPort2LoopV4)
	iDut2Bgp4Peer := iDut2Bgp.Ipv4Interfaces().Add().SetIpv4Name(iDut2LoopV4.Name()).Peers().Add().SetName(atePort2.Name + ".BGP4.peer")
	iDut2Bgp4Peer.SetPeerAddress(dutlo0Attrs.IPv4).SetAsNumber(dutAS).SetAsType(gosnappi.BgpV4PeerAsType.IBGP)
	iDut2Bgp4Peer.Capability().SetIpv4UnicastAddPath(true).SetIpv6UnicastAddPath(true)
	iDut2Bgp4Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)
	// iBGP - v6 seesion on Port2.
	iDut2Bgp6Peer := iDut2Bgp.Ipv6Interfaces().Add().SetIpv6Name(iDut2LoopV6.Name()).Peers().Add().SetName(atePort2.Name + ".BGP6.peer")
	iDut2Bgp6Peer.SetPeerAddress(dutlo0Attrs.IPv6).SetAsNumber(dutAS).SetAsType(gosnappi.BgpV6PeerAsType.IBGP)
	iDut2Bgp6Peer.Capability().SetIpv4UnicastAddPath(true).SetIpv6UnicastAddPath(true)
	iDut2Bgp6Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)

	// eBGP V4 routes from Port1.
	bgpNeti1Bgp4PeerRoutes := iDut1Bgp4Peer.V4Routes().Add().SetName(atePort1.Name + ".BGP4.Route")
	bgpNeti1Bgp4PeerRoutes.SetNextHopIpv4Address(iDut1Ipv4.Address()).
		SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).
		SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)
	bgpNeti1Bgp4PeerRoutes.Addresses().Add().
		SetAddress(ipv4Prefix1).SetPrefix(32)
	bgpNeti1Bgp4PeerRoutes.AddPath().SetPathId(1)
	bgpNeti1Bgp4PeerRoutes.Addresses().Add().
		SetAddress(ipv4Prefix2).SetPrefix(32)
	bgpNeti1Bgp4PeerRoutes.AddPath().SetPathId(1)
	bgpNeti1Bgp4PeerRoutes.Addresses().Add().
		SetAddress(ipv4Prefix3).SetPrefix(32)
	bgpNeti1Bgp4PeerRoutes.AddPath().SetPathId(1)

	// eBGP V6 routes from Port1.
	bgpNeti1Bgp6PeerRoutes := iDut1Bgp6Peer.V6Routes().Add().SetName(atePort1.Name + ".BGP6.Route")
	bgpNeti1Bgp6PeerRoutes.SetNextHopIpv6Address(iDut1Ipv6.Address()).
		SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).
		SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL)
	bgpNeti1Bgp6PeerRoutes.Addresses().Add().
		SetAddress(ipv6Prefix1).SetPrefix(128)
	bgpNeti1Bgp6PeerRoutes.AddPath().SetPathId(1)
	bgpNeti1Bgp6PeerRoutes.Addresses().Add().
		SetAddress(ipv6Prefix2).SetPrefix(128)
	bgpNeti1Bgp6PeerRoutes.AddPath().SetPathId(1)
	bgpNeti1Bgp6PeerRoutes.Addresses().Add().
		SetAddress(ipv6Prefix3).SetPrefix(128)
	bgpNeti1Bgp6PeerRoutes.AddPath().SetPathId(1)

	// iBGP V4 routes from Port2.
	bgpNeti2Bgp4PeerRoutes := iDut2Bgp4Peer.V4Routes().Add().SetName(atePort2.Name + ".BGP4.Route")
	bgpNeti2Bgp4PeerRoutes.SetNextHopIpv4Address(iDut2Ipv4.Address()).
		SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).
		SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)
	bgpNeti2Bgp4PeerRoutes.Addresses().Add().
		SetAddress(ipv4Prefix4).SetPrefix(32)
	bgpNeti2Bgp4PeerRoutes.AddPath().SetPathId(1)
	bgpNeti2Bgp4PeerRoutes.Addresses().Add().
		SetAddress(ipv4Prefix5).SetPrefix(32)
	bgpNeti2Bgp4PeerRoutes.AddPath().SetPathId(1)
	bgpNeti2Bgp4PeerRoutes.Addresses().Add().
		SetAddress(ipv4Prefix6).SetPrefix(32)
	bgpNeti2Bgp4PeerRoutes.AddPath().SetPathId(1)

	// iBGP V6 routes from Port2.
	bgpNeti2Bgp6PeerRoutes := iDut2Bgp6Peer.V6Routes().Add().SetName(atePort2.Name + ".BGP6.Route")
	bgpNeti2Bgp6PeerRoutes.SetNextHopIpv6Address(iDut2Ipv6.Address()).
		SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).
		SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL)
	bgpNeti2Bgp6PeerRoutes.Addresses().Add().
		SetAddress(ipv6Prefix4).SetPrefix(128)
	bgpNeti2Bgp6PeerRoutes.AddPath().SetPathId(1)
	bgpNeti2Bgp6PeerRoutes.Addresses().Add().
		SetAddress(ipv6Prefix5).SetPrefix(128)
	bgpNeti2Bgp6PeerRoutes.AddPath().SetPathId(1)
	bgpNeti2Bgp6PeerRoutes.Addresses().Add().
		SetAddress(ipv6Prefix6).SetPrefix(128)
	bgpNeti2Bgp6PeerRoutes.AddPath().SetPathId(1)

	t.Logf("Pushing config to OTG and starting protocols...")
	otg.PushConfig(t, config)
	time.Sleep(40 * time.Second)
	otg.StartProtocols(t)
	time.Sleep(40 * time.Second)
}

func verifyBGPCapabilities(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	t.Log("Verifying BGP capabilities.")
	var nbrIP = []string{atePort1.IPv4, otgIsisPort2LoopV4, atePort1.IPv6, otgIsisPort2LoopV6}
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

func verifyPrefixesTelemetry(t *testing.T, dut *ondatra.DUTDevice, nbr string, wantInstalled, wantRx, wantSent uint32, isV4 bool) {
	t.Helper()
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	t.Logf("Prefix telemetry on DUT for peer %v", nbr)

	var prefixPath *netinstbgp.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_PrefixesPath
	if isV4 {
		prefixPath = statePath.Neighbor(nbr).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Prefixes()
	} else {
		prefixPath = statePath.Neighbor(nbr).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Prefixes()
	}
	if gotInstalled, ok := gnmi.Watch(t, dut, prefixPath.Installed().State(), 10*time.Second, func(val *ygnmi.Value[uint32]) bool {
		gotInstalled, ok := val.Val()
		return ok && gotInstalled == wantInstalled
	}).Await(t); !ok {
		t.Errorf("Installed prefixes mismatch: got %v, want %v", gotInstalled, wantInstalled)
	}

	if !deviations.MissingPrePolicyReceivedRoutes(dut) {
		if gotRx, ok := gnmi.Watch(t, dut, prefixPath.ReceivedPrePolicy().State(), 10*time.Second, func(val *ygnmi.Value[uint32]) bool {
			gotRx, ok := val.Val()
			return ok && gotRx == wantRx
		}).Await(t); !ok {
			t.Errorf("Received prefixes mismatch: got %v, want %v", gotRx, wantRx)
		}
	}
	if gotSent, ok := gnmi.Watch(t, dut, prefixPath.Sent().State(), 10*time.Second, func(val *ygnmi.Value[uint32]) bool {
		gotSent, ok := val.Val()
		return ok && gotSent == wantSent
	}).Await(t); !ok {
		t.Errorf("Sent prefixes mismatch: got %v, want %v", gotSent, wantSent)
	}
}

type bgpNeighbor struct {
	as           uint32
	neighborip   string
	isV4         bool
	peerGrp      string
	localAddress string
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
	globalISIS.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
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
		isisIntfLevelAfi6 := isisIntfLevel.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST)
		isisIntfLevelAfi6.Metric = ygot.Uint32(200)
		isisIntfLevelAfi6.Enabled = ygot.Bool(true)
		if deviations.ISISInterfaceAfiUnsupported(dut) {
			isisIntf.Af = nil
		}
		if deviations.MissingIsisInterfaceAfiSafiEnable(dut) {
			isisIntfLevelAfi.Enabled = nil
			isisIntfLevelAfi6.Enabled = nil
		}
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

type bgpNbrList struct {
	nbrAddr string
	isV4    bool
	afiSafi oc.E_BgpTypes_AFI_SAFI_TYPE
}

func configureBGPDefaultPolicy(t *testing.T, dut *ondatra.DUTDevice, polType oc.E_RoutingPolicy_DefaultPolicyType) {
	t.Helper()
	nbrList := []*bgpNbrList{ebgpNbrV4, ebgpNbrV6, ibgpNbrV4, ibgpNbrV6}
	bgpPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()

	for _, nbr := range nbrList {
		nbrPolPath := bgpPath.Neighbor(nbr.nbrAddr).AfiSafi(nbr.afiSafi).ApplyPolicy()
		gnmi.Replace(t, dut, nbrPolPath.DefaultImportPolicy().Config(), polType)
		gnmi.Replace(t, dut, nbrPolPath.DefaultExportPolicy().Config(), polType)
	}
}

func deleteBGPPolicy(t *testing.T, dut *ondatra.DUTDevice, nbrList []*bgpNbrList) {
	t.Helper()
	bgpPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	for _, nbr := range nbrList {
		nbrAfiSafiPath := bgpPath.Neighbor(nbr.nbrAddr).AfiSafi(nbr.afiSafi)
		b := &gnmi.SetBatch{}
		gnmi.BatchDelete(b, nbrAfiSafiPath.ApplyPolicy().ImportPolicy().Config())
		gnmi.BatchDelete(b, nbrAfiSafiPath.ApplyPolicy().ExportPolicy().Config())
		b.Set(t, dut)
	}
}

func configurePrefixMatchAndDefaultStatement(t *testing.T, dut *ondatra.DUTDevice, prefixSet, prefixSubnetRange, maskLen string, ipPrefixSet []string, action string, defaultStatementOnly bool) *oc.RoutingPolicy {
	d := &oc.Root{}
	rp := d.GetOrCreateRoutingPolicy()
	if !defaultStatementOnly {
		pset := rp.GetOrCreateDefinedSets().GetOrCreatePrefixSet(prefixSet)
		for _, pref := range ipPrefixSet {
			pset.GetOrCreatePrefix(pref+"/"+maskLen, prefixSubnetRange)
			mode := oc.PrefixSet_Mode_IPV4
			if maskLen == maskLen128 {
				mode = oc.PrefixSet_Mode_IPV6
			}
			if !deviations.SkipPrefixSetMode(dut) {
				pset.SetMode(mode)
			}
		}
	}

	pdef := rp.GetOrCreatePolicyDefinition(prefixSet)
	if !defaultStatementOnly {
		stmt1, err := pdef.AppendNewStatement("10")
		if err != nil {
			t.Fatal(err)
		}
		stmt1.GetOrCreateConditions().GetOrCreateMatchPrefixSet().PrefixSet = ygot.String(prefixSet)
		stmt1.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
	}
	stmt2, err := pdef.AppendNewStatement("50")
	if err != nil {
		t.Fatal(err)
	}
	stmt2.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
	if action == "reject" {
		stmt2.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_REJECT_ROUTE
	}

	return rp
}

func configureRoutingPolicyDefaultAction(t *testing.T, dut *ondatra.DUTDevice, action string, defaultStatementOnly bool) {
	t.Helper()
	batchConfig := &gnmi.SetBatch{}
	bgpPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()

	t.Logf("Delete prefix set policies")
	deleteBGPPolicy(t, dut, []*bgpNbrList{ebgpNbrV4, ebgpNbrV6, ibgpNbrV4, ibgpNbrV6})
	gnmi.BatchDelete(batchConfig, gnmi.OC().RoutingPolicy().Config())
	batchConfig.Set(t, dut)
	time.Sleep(20 * time.Second)

	gnmi.BatchUpdate(batchConfig, gnmi.OC().RoutingPolicy().Config(), configurePrefixMatchAndDefaultStatement(t, dut, ebgpImportIPv4, maskLenExact, maskLen32, []string{ipv4Prefix1, ipv4Prefix2}, action, defaultStatementOnly))
	gnmi.BatchUpdate(batchConfig, gnmi.OC().RoutingPolicy().Config(), configurePrefixMatchAndDefaultStatement(t, dut, ebgpImportIPv6, maskLenExact, maskLen128, []string{ipv6Prefix1, ipv6Prefix2}, action, defaultStatementOnly))
	gnmi.BatchUpdate(batchConfig, gnmi.OC().RoutingPolicy().Config(), configurePrefixMatchAndDefaultStatement(t, dut, ebgpExportIPv4, maskLenExact, maskLen32, []string{ipv4Prefix4}, action, defaultStatementOnly))
	gnmi.BatchUpdate(batchConfig, gnmi.OC().RoutingPolicy().Config(), configurePrefixMatchAndDefaultStatement(t, dut, ebgpExportIPv6, maskLenExact, maskLen128, []string{ipv6Prefix4}, action, defaultStatementOnly))
	gnmi.BatchUpdate(batchConfig, gnmi.OC().RoutingPolicy().Config(), configurePrefixMatchAndDefaultStatement(t, dut, ibgpImportIPv4, maskLenExact, maskLen32, []string{ipv4Prefix4, ipv4Prefix5}, action, defaultStatementOnly))
	gnmi.BatchUpdate(batchConfig, gnmi.OC().RoutingPolicy().Config(), configurePrefixMatchAndDefaultStatement(t, dut, ibgpImportIPv6, maskLenExact, maskLen128, []string{ipv6Prefix4, ipv6Prefix5}, action, defaultStatementOnly))
	gnmi.BatchUpdate(batchConfig, gnmi.OC().RoutingPolicy().Config(), configurePrefixMatchAndDefaultStatement(t, dut, ibgpExportIPv4, maskLenExact, maskLen32, []string{ipv4Prefix1}, action, defaultStatementOnly))
	gnmi.BatchUpdate(batchConfig, gnmi.OC().RoutingPolicy().Config(), configurePrefixMatchAndDefaultStatement(t, dut, ibgpExportIPv6, maskLenExact, maskLen128, []string{ipv6Prefix1}, action, defaultStatementOnly))

	// Apply the above policies to the respective peering at the respective AFI-SAFI levels
	gnmi.BatchReplace(batchConfig, bgpPath.Neighbor(ebgpNbrV4.nbrAddr).AfiSafi(ebgpNbrV4.afiSafi).ApplyPolicy().ImportPolicy().Config(), []string{ebgpImportIPv4})
	gnmi.BatchReplace(batchConfig, bgpPath.Neighbor(ebgpNbrV4.nbrAddr).AfiSafi(ebgpNbrV4.afiSafi).ApplyPolicy().ExportPolicy().Config(), []string{ebgpExportIPv4})
	gnmi.BatchReplace(batchConfig, bgpPath.Neighbor(ebgpNbrV6.nbrAddr).AfiSafi(ebgpNbrV6.afiSafi).ApplyPolicy().ImportPolicy().Config(), []string{ebgpImportIPv6})
	gnmi.BatchReplace(batchConfig, bgpPath.Neighbor(ebgpNbrV6.nbrAddr).AfiSafi(ebgpNbrV6.afiSafi).ApplyPolicy().ExportPolicy().Config(), []string{ebgpExportIPv6})
	gnmi.BatchReplace(batchConfig, bgpPath.Neighbor(ibgpNbrV4.nbrAddr).AfiSafi(ibgpNbrV4.afiSafi).ApplyPolicy().ImportPolicy().Config(), []string{ibgpImportIPv4})
	gnmi.BatchReplace(batchConfig, bgpPath.Neighbor(ibgpNbrV4.nbrAddr).AfiSafi(ibgpNbrV4.afiSafi).ApplyPolicy().ExportPolicy().Config(), []string{ibgpExportIPv4})
	gnmi.BatchReplace(batchConfig, bgpPath.Neighbor(ibgpNbrV6.nbrAddr).AfiSafi(ibgpNbrV6.afiSafi).ApplyPolicy().ImportPolicy().Config(), []string{ibgpImportIPv6})
	gnmi.BatchReplace(batchConfig, bgpPath.Neighbor(ibgpNbrV6.nbrAddr).AfiSafi(ibgpNbrV6.afiSafi).ApplyPolicy().ExportPolicy().Config(), []string{ibgpExportIPv6})

	batchConfig.Set(t, dut)

	time.Sleep(20 * time.Second)
}

func testDefaultPolicyRejectRouteAction(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()

	t.Run("Create and apply default-policy REJECT-ALL with action as REJECT_ROUTE", func(t *testing.T) {
		if deviations.BgpDefaultPolicyUnsupported(dut) {
			configureRoutingPolicyDefaultAction(t, dut, "reject", !defaultStatementOnly)
		} else {
			configureBGPDefaultPolicy(t, dut, rejectRoute)
		}
	})

	verifyPostPolicyPrefixTelemetry(t, dut, &peerDetails{ipAddr: atePort1.IPv4, defExportPol: rejectRoute,
		defImportPol: rejectRoute, exportPol: []string{ebgpExportIPv4}, importPol: []string{ebgpImportIPv4},
		wantInstalled: 2, wantRx: 2, wantRxPrePolicy: 3, wantSent: 1, isV4: true})
	verifyPostPolicyPrefixTelemetry(t, dut, &peerDetails{ipAddr: atePort1.IPv6, defExportPol: rejectRoute,
		defImportPol: rejectRoute, exportPol: []string{ebgpExportIPv6}, importPol: []string{ebgpImportIPv6},
		wantInstalled: 2, wantRx: 2, wantRxPrePolicy: 3, wantSent: 1, isV4: false})
	verifyPostPolicyPrefixTelemetry(t, dut, &peerDetails{ipAddr: otgIsisPort2LoopV4, defExportPol: rejectRoute,
		defImportPol: rejectRoute, exportPol: []string{ibgpExportIPv4}, importPol: []string{ibgpImportIPv4},
		wantInstalled: 2, wantRx: 2, wantRxPrePolicy: 3, wantSent: 1, isV4: true})
	verifyPostPolicyPrefixTelemetry(t, dut, &peerDetails{ipAddr: otgIsisPort2LoopV6, defExportPol: rejectRoute,
		defImportPol: rejectRoute, exportPol: []string{ibgpExportIPv6}, importPol: []string{ibgpImportIPv6},
		wantInstalled: 2, wantRx: 2, wantRxPrePolicy: 3, wantSent: 1, isV4: false})
}

func testDefaultPolicyAcceptRouteAction(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	t.Run("Create and apply default-policy ACCEPT-ALL with action as ACCEPT_ROUTE", func(t *testing.T) {
		if deviations.BgpDefaultPolicyUnsupported(dut) {
			configureRoutingPolicyDefaultAction(t, dut, "accept", !defaultStatementOnly)
		} else {
			configureBGPDefaultPolicy(t, dut, acceptRoute)
		}
	})

	verifyPostPolicyPrefixTelemetry(t, dut, &peerDetails{ipAddr: atePort1.IPv4, defExportPol: acceptRoute,
		defImportPol: acceptRoute, exportPol: []string{ebgpExportIPv4}, importPol: []string{ebgpImportIPv4},
		wantInstalled: 3, wantRx: 3, wantRxPrePolicy: 3, wantSent: 3, isV4: true})
	verifyPostPolicyPrefixTelemetry(t, dut, &peerDetails{ipAddr: atePort1.IPv6, defExportPol: acceptRoute,
		defImportPol: acceptRoute, exportPol: []string{ebgpExportIPv6}, importPol: []string{ebgpImportIPv6},
		wantInstalled: 3, wantRx: 3, wantRxPrePolicy: 3, wantSent: 3, isV4: false})
	verifyPostPolicyPrefixTelemetry(t, dut, &peerDetails{ipAddr: otgIsisPort2LoopV4, defExportPol: acceptRoute,
		defImportPol: acceptRoute, exportPol: []string{ibgpExportIPv4}, importPol: []string{ibgpImportIPv4},
		wantInstalled: 3, wantRx: 3, wantRxPrePolicy: 3, wantSent: 3, isV4: true})
	verifyPostPolicyPrefixTelemetry(t, dut, &peerDetails{ipAddr: otgIsisPort2LoopV6, defExportPol: acceptRoute,
		defImportPol: acceptRoute, exportPol: []string{ibgpExportIPv6}, importPol: []string{ibgpImportIPv6},
		wantInstalled: 3, wantRx: 3, wantRxPrePolicy: 3, wantSent: 3, isV4: false})
}

func testDefaultPolicyAcceptRouteActionOnly(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()

	t.Run("Create and apply default-policy ACCEPT-ALL with action as ACCEPT_ROUTE", func(t *testing.T) {
		if deviations.BgpDefaultPolicyUnsupported(dut) {
			configureRoutingPolicyDefaultAction(t, dut, "accept", defaultStatementOnly)
		} else {
			configureBGPDefaultPolicy(t, dut, acceptRoute)
			t.Run("Delete prefix set policies", func(t *testing.T) {
				deleteBGPPolicy(t, dut, []*bgpNbrList{ebgpNbrV4, ebgpNbrV6, ibgpNbrV4, ibgpNbrV6})
			})
		}
	})

	verifyPostPolicyPrefixTelemetry(t, dut, &peerDetails{ipAddr: atePort1.IPv4, defExportPol: acceptRoute,
		defImportPol: acceptRoute, exportPol: []string{}, importPol: []string{},
		wantInstalled: 3, wantRx: 3, wantRxPrePolicy: 3, wantSent: 3, isV4: true})
	verifyPostPolicyPrefixTelemetry(t, dut, &peerDetails{ipAddr: atePort1.IPv6, defExportPol: acceptRoute,
		defImportPol: acceptRoute, exportPol: []string{}, importPol: []string{},
		wantInstalled: 3, wantRx: 3, wantRxPrePolicy: 3, wantSent: 3, isV4: false})
	verifyPostPolicyPrefixTelemetry(t, dut, &peerDetails{ipAddr: otgIsisPort2LoopV4, defExportPol: acceptRoute,
		defImportPol: acceptRoute, exportPol: []string{}, importPol: []string{},
		wantInstalled: 3, wantRx: 3, wantRxPrePolicy: 3, wantSent: 3, isV4: true})
	verifyPostPolicyPrefixTelemetry(t, dut, &peerDetails{ipAddr: otgIsisPort2LoopV6, defExportPol: acceptRoute,
		defImportPol: acceptRoute, exportPol: []string{}, importPol: []string{},
		wantInstalled: 3, wantRx: 3, wantRxPrePolicy: 3, wantSent: 3, isV4: false})
}

func testNoPolicyConfiguredIBGPPeer(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	// TODO: RT-7.5 should be automated only after the expected behavior is confirmed in
	// https://github.com/openconfig/public/issues/981
}

func testNoPolicyConfigured(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	// TODO: RT-7.6 should be automated only after the expected behavior is confirmed in
	// https://github.com/openconfig/public/issues/981
}

func testDefaultPolicyRejectRouteActionOnly(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()

	t.Run("Create and apply default-policy REJECT-ALL with action as REJECT_ROUTE", func(t *testing.T) {
		if deviations.BgpDefaultPolicyUnsupported(dut) {
			configureRoutingPolicyDefaultAction(t, dut, "reject", defaultStatementOnly)
		} else {
			configureBGPDefaultPolicy(t, dut, rejectRoute)
		}
	})

	verifyPostPolicyPrefixTelemetry(t, dut, &peerDetails{ipAddr: atePort1.IPv4, defExportPol: rejectRoute,
		defImportPol: rejectRoute, exportPol: []string{}, importPol: []string{},
		wantInstalled: 0, wantRx: 0, wantRxPrePolicy: 3, wantSent: 0, isV4: true})
	verifyPostPolicyPrefixTelemetry(t, dut, &peerDetails{ipAddr: atePort1.IPv6, defExportPol: rejectRoute,
		defImportPol: rejectRoute, exportPol: []string{}, importPol: []string{},
		wantInstalled: 0, wantRx: 0, wantRxPrePolicy: 3, wantSent: 0, isV4: false})
	verifyPostPolicyPrefixTelemetry(t, dut, &peerDetails{ipAddr: otgIsisPort2LoopV4, defExportPol: rejectRoute,
		defImportPol: rejectRoute, exportPol: []string{}, importPol: []string{},
		wantInstalled: 0, wantRx: 0, wantRxPrePolicy: 3, wantSent: 0, isV4: true})
	verifyPostPolicyPrefixTelemetry(t, dut, &peerDetails{ipAddr: otgIsisPort2LoopV6, defExportPol: rejectRoute,
		defImportPol: rejectRoute, exportPol: []string{}, importPol: []string{},
		wantInstalled: 0, wantRx: 0, wantRxPrePolicy: 3, wantSent: 0, isV4: false})
}

func configureRoutePolicies(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	bgpPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	batchConfig := &gnmi.SetBatch{}

	gnmi.BatchUpdate(batchConfig, gnmi.OC().RoutingPolicy().Config(), configurePrefixMatchPolicy(t, dut, ebgpImportIPv4, maskLenExact, maskLen32, []string{ipv4Prefix1, ipv4Prefix2}))
	gnmi.BatchUpdate(batchConfig, gnmi.OC().RoutingPolicy().Config(), configurePrefixMatchPolicy(t, dut, ebgpImportIPv6, maskLenExact, maskLen128, []string{ipv6Prefix1, ipv6Prefix2}))
	gnmi.BatchUpdate(batchConfig, gnmi.OC().RoutingPolicy().Config(), configurePrefixMatchPolicy(t, dut, ebgpExportIPv4, maskLenExact, maskLen32, []string{ipv4Prefix4}))
	gnmi.BatchUpdate(batchConfig, gnmi.OC().RoutingPolicy().Config(), configurePrefixMatchPolicy(t, dut, ebgpExportIPv6, maskLenExact, maskLen128, []string{ipv6Prefix4}))

	gnmi.BatchUpdate(batchConfig, gnmi.OC().RoutingPolicy().Config(), configurePrefixMatchPolicy(t, dut, ibgpImportIPv4, maskLenExact, maskLen32, []string{ipv4Prefix4, ipv4Prefix5}))
	gnmi.BatchUpdate(batchConfig, gnmi.OC().RoutingPolicy().Config(), configurePrefixMatchPolicy(t, dut, ibgpImportIPv6, maskLenExact, maskLen128, []string{ipv6Prefix4, ipv6Prefix5}))
	gnmi.BatchUpdate(batchConfig, gnmi.OC().RoutingPolicy().Config(), configurePrefixMatchPolicy(t, dut, ibgpExportIPv4, maskLenExact, maskLen32, []string{ipv4Prefix1}))
	gnmi.BatchUpdate(batchConfig, gnmi.OC().RoutingPolicy().Config(), configurePrefixMatchPolicy(t, dut, ibgpExportIPv6, maskLenExact, maskLen128, []string{ipv6Prefix1}))

	// Apply the above policies to the respective peering at the repective AFI-SAFI levels
	gnmi.BatchReplace(batchConfig, bgpPath.Neighbor(ebgpNbrV4.nbrAddr).AfiSafi(ebgpNbrV4.afiSafi).ApplyPolicy().ImportPolicy().Config(), []string{ebgpImportIPv4})
	gnmi.BatchReplace(batchConfig, bgpPath.Neighbor(ebgpNbrV4.nbrAddr).AfiSafi(ebgpNbrV4.afiSafi).ApplyPolicy().ExportPolicy().Config(), []string{ebgpExportIPv4})
	gnmi.BatchReplace(batchConfig, bgpPath.Neighbor(ebgpNbrV6.nbrAddr).AfiSafi(ebgpNbrV6.afiSafi).ApplyPolicy().ImportPolicy().Config(), []string{ebgpImportIPv6})
	gnmi.BatchReplace(batchConfig, bgpPath.Neighbor(ebgpNbrV6.nbrAddr).AfiSafi(ebgpNbrV6.afiSafi).ApplyPolicy().ExportPolicy().Config(), []string{ebgpExportIPv6})
	gnmi.BatchReplace(batchConfig, bgpPath.Neighbor(ibgpNbrV4.nbrAddr).AfiSafi(ibgpNbrV4.afiSafi).ApplyPolicy().ImportPolicy().Config(), []string{ibgpImportIPv4})
	gnmi.BatchReplace(batchConfig, bgpPath.Neighbor(ibgpNbrV4.nbrAddr).AfiSafi(ibgpNbrV4.afiSafi).ApplyPolicy().ExportPolicy().Config(), []string{ibgpExportIPv4})
	gnmi.BatchReplace(batchConfig, bgpPath.Neighbor(ibgpNbrV6.nbrAddr).AfiSafi(ibgpNbrV6.afiSafi).ApplyPolicy().ImportPolicy().Config(), []string{ibgpImportIPv6})
	gnmi.BatchReplace(batchConfig, bgpPath.Neighbor(ibgpNbrV6.nbrAddr).AfiSafi(ibgpNbrV6.afiSafi).ApplyPolicy().ExportPolicy().Config(), []string{ibgpExportIPv6})

	batchConfig.Set(t, dut)
}

type peerDetails struct {
	ipAddr                                           string
	defExportPol, defImportPol                       oc.E_RoutingPolicy_DefaultPolicyType
	exportPol, importPol                             []string
	wantInstalled, wantRx, wantRxPrePolicy, wantSent uint32
	isV4                                             bool
}

func verifyPostPolicyPrefixTelemetry(t *testing.T, dut *ondatra.DUTDevice, nbr *peerDetails) {
	t.Helper()

	t.Logf("Prefix telemetry validation for the neighbor %v", nbr.ipAddr)

	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	var afiSafiPath *netinstbgp.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafiPath
	if nbr.isV4 {
		afiSafiPath = statePath.Neighbor(nbr.ipAddr).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
	} else {
		afiSafiPath = statePath.Neighbor(nbr.ipAddr).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
	}

	peerTel := gnmi.Get(t, dut, afiSafiPath.State())

	if !deviations.BgpDefaultPolicyUnsupported(dut) {
		if gotDefExPolicy := peerTel.GetApplyPolicy().GetDefaultExportPolicy(); gotDefExPolicy != nbr.defExportPol {
			t.Errorf("Default export policy type mismatch: got %v, want %v", gotDefExPolicy, nbr.defExportPol)
		}

		if gotDefImPolicy := peerTel.GetApplyPolicy().GetDefaultImportPolicy(); gotDefImPolicy != nbr.defImportPol {
			t.Errorf("Default import policy type mismatch: got %v, want %v", gotDefImPolicy, nbr.defImportPol)
		}
	}
	if len(nbr.exportPol) != 0 {
		if gotExportPol := peerTel.GetApplyPolicy().GetExportPolicy(); cmp.Diff(gotExportPol, nbr.exportPol) != "" {
			t.Errorf("Export policy type mismatch: got %v, want %v", gotExportPol, nbr.exportPol)
		}
	}
	if len(nbr.importPol) != 0 {
		if gotImportPol := peerTel.GetApplyPolicy().GetImportPolicy(); cmp.Diff(gotImportPol, nbr.importPol) != "" {
			t.Errorf("Import policy type mismatch: got %v, want %v", gotImportPol, nbr.importPol)
		}
	}

	if gotInstalled, ok := gnmi.Watch(t, dut, afiSafiPath.Prefixes().Installed().State(), 10*time.Second, func(val *ygnmi.Value[uint32]) bool {
		gotInstalled, ok := val.Val()
		return ok && gotInstalled == nbr.wantInstalled
	}).Await(t); !ok {
		t.Errorf("Installed prefixes mismatch: got %v, want %v", gotInstalled, nbr.wantInstalled)
	}

	if !deviations.MissingPrePolicyReceivedRoutes(dut) {
		if gotRxPrePol, ok := gnmi.Watch(t, dut, afiSafiPath.Prefixes().ReceivedPrePolicy().State(), 20*time.Second, func(val *ygnmi.Value[uint32]) bool {
			gotRxPrePol, ok := val.Val()
			return ok && gotRxPrePol == nbr.wantRxPrePolicy
		}).Await(t); !ok {
			t.Errorf("Received pre policy prefixes mismatch: got %v, want %v", gotRxPrePol, nbr.wantRxPrePolicy)
		}
	}
	if gotRx, ok := gnmi.Watch(t, dut, afiSafiPath.Prefixes().Received().State(), 20*time.Second, func(val *ygnmi.Value[uint32]) bool {
		gotRx, ok := val.Val()
		return ok && gotRx == nbr.wantRx
	}).Await(t); !ok {
		t.Errorf("Received prefixes mismatch: got %v, want %v", gotRx, nbr.wantRx)
	}

	if nbr.defImportPol == oc.RoutingPolicy_DefaultPolicyType_ACCEPT_ROUTE && !deviations.SkipNonBgpRouteExportCheck(dut) {
		if gotSent, ok := gnmi.Watch(t, dut, afiSafiPath.Prefixes().Sent().State(), 10*time.Second, func(val *ygnmi.Value[uint32]) bool {
			gotSent, ok := val.Val()
			return ok && gotSent == nbr.wantSent
		}).Await(t); !ok {
			t.Errorf("Sent prefixes mismatch: got %v, want %v", gotSent, nbr.wantSent)
		}
	}
}

func configStaticRoute(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	staticRoute1 := fmt.Sprintf("%s/%d", ipv4Prefix7, uint32(32))
	staticRoute2 := fmt.Sprintf("%s/%d", ipv6Prefix7, uint32(128))
	staticRoute3 := fmt.Sprintf("%s/%d", ipv4Prefix8, uint32(32))
	staticRoute4 := fmt.Sprintf("%s/%d", ipv6Prefix8, uint32(128))

	ni := oc.NetworkInstance{Name: ygot.String(deviations.DefaultNetworkInstance(dut))}
	static := ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))

	sr1 := static.GetOrCreateStatic(staticRoute1)
	nh1 := sr1.GetOrCreateNextHop("0")
	nh1.NextHop = oc.UnionString(atePort1.IPv4)

	sr2 := static.GetOrCreateStatic(staticRoute2)
	nh2 := sr2.GetOrCreateNextHop("0")
	nh2.NextHop = oc.UnionString(atePort1.IPv6)

	sr3 := static.GetOrCreateStatic(staticRoute3)
	nh3 := sr3.GetOrCreateNextHop("0")
	nh3.NextHop = oc.UnionString(atePort2.IPv4)

	sr4 := static.GetOrCreateStatic(staticRoute4)
	nh4 := sr4.GetOrCreateNextHop("0")
	nh4.NextHop = oc.UnionString(atePort2.IPv6)

	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut)).Config(), static)
}

// TestBGPDefaultPolicies is to test default-policies at the BGP peer-group and neighbor levels.
func TestBGPDefaultPolicies(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	t.Run("Configure DUT interfaces", func(t *testing.T) {
		configureDUT(t, dut)
	})

	t.Run("Configure DEFAULT network instance", func(t *testing.T) {
		fptest.ConfigureDefaultNetworkInstance(t, dut)
	})

	t.Run("Configure ISIS on DUT", func(t *testing.T) {
		dutIsisIntfNames := []string{dut.Port(t, "port2").Name(), loopbackIntfName}
		if deviations.ExplicitInterfaceInDefaultVRF(dut) {
			dutIsisIntfNames = []string{dut.Port(t, "port2").Name() + ".0", loopbackIntfName + ".0"}
		}
		configureISIS(t, dut, dutIsisIntfNames, dutAreaAddress, dutSysID)
	})

	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	t.Run("Configure BGP Neighbors", func(t *testing.T) {
		gnmi.Delete(t, dut, dutConfPath.Config())
		dutConf := bgpCreateNbr(dutAS, ateAS, dut)
		gnmi.Replace(t, dut, dutConfPath.Config(), dutConf)
		fptest.LogQuery(t, "DUT BGP Config", dutConfPath.Config(), gnmi.Get(t, dut, dutConfPath.Config()))
	})

	otg := ate.OTG()
	t.Run("Configure OTG", func(t *testing.T) {
		configureOTG(t, otg)
	})

	t.Run("Verify port status on DUT", func(t *testing.T) {
		verifyPortsUp(t, dut.Device)
	})

	t.Run("Verify ISIS session status on DUT", func(t *testing.T) {
		dutIsisPeerIntf := []string{dut.Port(t, "port2").Name()}
		verifyISISTelemetry(t, dut, dutIsisPeerIntf)
	})

	t.Run("Verify BGP session telemetry", func(t *testing.T) {
		verifyBgpTelemetry(t, dut)
	})

	t.Run("Verify BGP capabilities", func(t *testing.T) {
		verifyBGPCapabilities(t, dut)
	})

	t.Run("Verify prefix telemetry on DUT for all iBGP and eBGP peers", func(t *testing.T) {
		if deviations.DefaultImportExportPolicy(dut) {
			verifyPrefixesTelemetry(t, dut, atePort1.IPv4, 3, 3, 3, v4Prefixes)
			verifyPrefixesTelemetry(t, dut, otgIsisPort2LoopV4, 3, 3, 3, v4Prefixes)
			verifyPrefixesTelemetry(t, dut, atePort1.IPv6, 3, 3, 3, !v4Prefixes)
			verifyPrefixesTelemetry(t, dut, otgIsisPort2LoopV6, 3, 3, 3, !v4Prefixes)
		} else {
			verifyPrefixesTelemetry(t, dut, atePort1.IPv4, 0, 3, 0, v4Prefixes)
			verifyPrefixesTelemetry(t, dut, otgIsisPort2LoopV4, 3, 3, 0, v4Prefixes)
			verifyPrefixesTelemetry(t, dut, atePort1.IPv6, 0, 3, 0, !v4Prefixes)
			verifyPrefixesTelemetry(t, dut, otgIsisPort2LoopV6, 3, 3, 0, !v4Prefixes)
		}
	})

	t.Run("Add static routes for ip prefixes IPv4/v6-prefix7 and IPv4/v6-prefix8", func(t *testing.T) {
		configStaticRoute(t, dut)
	})

	t.Run("Create and apply prefix based import and export route policies", func(t *testing.T) {
		configureRoutePolicies(t, dut)
	})

	cases := []struct {
		desc     string
		funcName func()
		skipMsg  string
	}{{
		desc:     "RT-7.1.1: Policy definition in policy chain is not satisfied and Default Policy has REJECT_ROUTE action",
		funcName: func() { testDefaultPolicyRejectRouteAction(t, dut) },
	}, {
		desc:     "RT-7.1.2 : Policy definition in policy chain is not satisfied and Default Policy has ACCEPT_ROUTE action",
		funcName: func() { testDefaultPolicyAcceptRouteAction(t, dut) },
	}, {
		desc:     "RT-7.1.3 : No policy attached either at the Peer-group or at the neighbor level and Default Policy has ACCEPT_ROUTE action",
		funcName: func() { testDefaultPolicyAcceptRouteActionOnly(t, dut) },
	}, {
		desc:     "RT-7.1.4 : No policy attached either at the Peer-group or at the neighbor level and Default Policy has REJECT_ROUTE action",
		funcName: func() { testDefaultPolicyRejectRouteActionOnly(t, dut) },
	}, {
		desc:     "RT-7.1.5 : No policy including the default-policy is attached either at the Peer-group or at the neighbor level for only IBGP peer",
		funcName: func() { testNoPolicyConfiguredIBGPPeer(t, dut) },
		skipMsg:  "TODO: RT-7.1.5 should be automated only after the expected behavior is confirmed in issue num 981",
	}, {
		desc:     "RT-7.1.6 : No policy including the default-policy is attached either at the Peer-group or at the neighbor level for both EBGP and IBGP peers",
		funcName: func() { testNoPolicyConfigured(t, dut) },
		skipMsg:  "TODO: RT-7.1.6 should be automated only after the expected behavior is confirmed in issue num 981",
	}}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			if len(tc.skipMsg) > 0 {
				t.Skip(tc.skipMsg)
			}
			tc.funcName()
		})
	}
}
