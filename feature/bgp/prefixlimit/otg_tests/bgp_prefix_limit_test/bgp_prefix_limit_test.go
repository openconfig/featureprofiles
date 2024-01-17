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

package bgp_prefix_limit_test

import (
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
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
	trafficDuration          = 1 * time.Minute
	grTimer                  = 2 * time.Minute
	grRestartTime            = 75
	grStaleRouteTime         = 300.0
	ipv4SrcTraffic           = "192.0.2.2"
	ipv6SrcTraffic           = "2001:DB8:1::1"
	ipv4DstTraffic           = "203.0.113.0"
	ipv6DstTraffic           = "2001:DB8:2::1"
	advertisedRoutesv4Prefix = 32
	advertisedRoutesv6Prefix = 128
	prefixLimit              = 200
	pwarnthesholdPct         = 10
	prefixTimer              = 30.0
	dutAS                    = 64500
	ateAS                    = 64501
	plenIPv4                 = 30
	plenIPv6                 = 126
	tolerance                = 50
	rplType                  = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
	rplName                  = "ALLOW"
	peerGrpNamev4            = "BGP-PEER-GROUP-V4"
	peerGrpNamev6            = "BGP-PEER-GROUP-V6"
	r4UnderLimit             = "r4UnderLimit"
	r6UnderLimit             = "r6UnderLimit"
	r4AtLimit                = "r4AtLimit"
	r6AtLimit                = "r6AtLimit"
	r4OverLimit              = "r4OverLimit"
	r6OverLimit              = "r6OverLimit"
)

var (
	dutSrc = attrs.Attributes{
		Desc:    "DUT to ATE source",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::192:0:2:1",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	ateSrc = attrs.Attributes{
		Name:    "ateSrc",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::192:0:2:2",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}

	dutDst = attrs.Attributes{
		Desc:    "DUT to ATE destination",
		IPv4:    "192.0.2.5",
		IPv6:    "2001:db8::192:0:2:5",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}

	ateDst = attrs.Attributes{
		Name:    "atedst",
		MAC:     "02:00:02:01:01:01",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::192:0:2:6",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
)

// configureDUT configures all the interfaces and BGP on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	dc := gnmi.OC()
	p1 := dut.Port(t, "port1").Name()
	i1 := dutSrc.NewOCInterface(p1, dut)
	gnmi.Replace(t, dut, dc.Interface(p1).Config(), i1)

	p2 := dut.Port(t, "port2").Name()
	i2 := dutDst.NewOCInterface(p2, dut)
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
	dutConf := createBGPNeighbor(dutAS, ateAS, prefixLimit, grRestartTime, dut)
	gnmi.Replace(t, dut, dutConfPath.Config(), dutConf)
}

func (tc *testCase) verifyPortsUp(t *testing.T, dev *ondatra.Device) {
	for _, p := range dev.Ports() {
		portStatus := gnmi.Get(t, dev, gnmi.OC().Interface(p.Name()).OperStatus().State())
		if want := oc.Interface_OperStatus_UP; portStatus != want {
			t.Errorf("%s Status: got %v, want %v", p, portStatus, want)
		}
	}
}

// configureATE configures the interfaces and BGP on the ATE, with port2 advertising routes.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	otg := ate.OTG()
	topo := gosnappi.NewConfig()
	srcPort := topo.Ports().Add().SetName("port1")
	srcDev := topo.Devices().Add().SetName(ateSrc.Name)
	srcEth := srcDev.Ethernets().Add().SetName(ateSrc.Name + ".Eth").SetMac(ateSrc.MAC)
	srcEth.Connection().SetPortName(srcPort.Name())
	srcIpv4 := srcEth.Ipv4Addresses().Add().SetName(ateSrc.Name + ".IPv4")
	srcIpv4.SetAddress(ateSrc.IPv4).SetGateway(dutSrc.IPv4).SetPrefix(uint32(ateSrc.IPv4Len))
	srcIpv6 := srcEth.Ipv6Addresses().Add().SetName(ateSrc.Name + ".IPv6")
	srcIpv6.SetAddress(ateSrc.IPv6).SetGateway(dutSrc.IPv6).SetPrefix(uint32(ateSrc.IPv6Len))

	dstPort := topo.Ports().Add().SetName("port2")
	dstDev := topo.Devices().Add().SetName(ateDst.Name)
	dstEth := dstDev.Ethernets().Add().SetName(ateDst.Name + ".Eth").SetMac(ateDst.MAC)
	dstEth.Connection().SetPortName(dstPort.Name())
	dstIpv4 := dstEth.Ipv4Addresses().Add().SetName(ateDst.Name + ".IPv4")
	dstIpv4.SetAddress(ateDst.IPv4).SetGateway(dutDst.IPv4).SetPrefix(uint32(ateDst.IPv4Len))
	dstIpv6 := dstEth.Ipv6Addresses().Add().SetName(ateDst.Name + ".IPv6")
	dstIpv6.SetAddress(ateDst.IPv6).SetGateway(dutDst.IPv6).SetPrefix(uint32(ateDst.IPv6Len))

	// Setup ATE BGP route v4 advertisement
	srcBgp := srcDev.Bgp().SetRouterId(srcIpv4.Address())
	srcBgp4Peer := srcBgp.Ipv4Interfaces().Add().SetIpv4Name(srcIpv4.Name()).Peers().Add().SetName(ateSrc.Name + ".BGP4.peer")
	srcBgp4Peer.SetPeerAddress(srcIpv4.Gateway()).SetAsNumber(ateAS).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
	srcBgp6Peer := srcBgp.Ipv6Interfaces().Add().SetIpv6Name(srcIpv6.Name()).Peers().Add().SetName(ateSrc.Name + ".BGP6.peer")
	srcBgp6Peer.SetPeerAddress(srcIpv6.Gateway()).SetAsNumber(ateAS).SetAsType(gosnappi.BgpV6PeerAsType.EBGP)

	dstBgp := dstDev.Bgp().SetRouterId(dstIpv4.Address())
	dstBgp4Peer := dstBgp.Ipv4Interfaces().Add().SetIpv4Name(dstIpv4.Name()).Peers().Add().SetName(ateDst.Name + ".BGP4.peer")
	dstBgp4Peer.SetPeerAddress(dstIpv4.Gateway()).SetAsNumber(ateAS).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
	dstBgp6Peer := dstBgp.Ipv6Interfaces().Add().SetIpv6Name(dstIpv6.Name()).Peers().Add().SetName(ateDst.Name + ".BGP6.peer")
	dstBgp6Peer.SetPeerAddress(dstIpv6.Gateway()).SetAsNumber(ateAS).SetAsType(gosnappi.BgpV6PeerAsType.EBGP)

	configureBGPv4Routes(dstBgp4Peer, dstIpv4.Address(), r4UnderLimit, ipv4DstTraffic, prefixLimit-1)
	configureBGPv6Routes(dstBgp6Peer, dstIpv6.Address(), r6UnderLimit, ipv6DstTraffic, prefixLimit-1)
	configureBGPv4Routes(dstBgp4Peer, dstIpv4.Address(), r4AtLimit, ipv4DstTraffic, prefixLimit)
	configureBGPv6Routes(dstBgp6Peer, dstIpv6.Address(), r6AtLimit, ipv6DstTraffic, prefixLimit)
	configureBGPv4Routes(dstBgp4Peer, dstIpv4.Address(), r4OverLimit, ipv4DstTraffic, prefixLimit+1)
	configureBGPv6Routes(dstBgp6Peer, dstIpv6.Address(), r6OverLimit, ipv6DstTraffic, prefixLimit+1)

	configureFlow(topo, "IPv4.UnderLimit", srcIpv4.Name(), r4UnderLimit, ateSrc.MAC, ateSrc.IPv4, ipv4DstTraffic, "ipv4", prefixLimit-1)
	configureFlow(topo, "IPv6.UnderLimit", srcIpv6.Name(), r6UnderLimit, ateSrc.MAC, ateSrc.IPv6, ipv6DstTraffic, "ipv6", prefixLimit-1)
	configureFlow(topo, "IPv4.AtLimit", srcIpv4.Name(), r4AtLimit, ateSrc.MAC, ateSrc.IPv4, ipv4DstTraffic, "ipv4", prefixLimit)
	configureFlow(topo, "IPv6.AtLimit", srcIpv6.Name(), r6AtLimit, ateSrc.MAC, ateSrc.IPv6, ipv6DstTraffic, "ipv6", prefixLimit)
	configureFlow(topo, "IPv4.OverLimit", srcIpv4.Name(), r4OverLimit, ateSrc.MAC, ateSrc.IPv4, ipv4DstTraffic, "ipv4", prefixLimit+1)
	configureFlow(topo, "IPv6.OverLimit", srcIpv6.Name(), r6OverLimit, ateSrc.MAC, ateSrc.IPv6, ipv6DstTraffic, "ipv6", prefixLimit+1)

	t.Logf("Pushing config to ATE and starting protocols...")
	otg.PushConfig(t, topo)

	return topo
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

func configureFlow(topo gosnappi.Config, name, flowSrcEndPoint, flowDstEndPoint, srcMac, srcIp, dstIp, iptype string, routeCount uint32) {
	flow := topo.Flows().Add().SetName(name)
	flow.Metrics().SetEnable(true)
	flow.TxRx().Device().
		SetTxNames([]string{flowSrcEndPoint}).
		SetRxNames([]string{flowDstEndPoint})
	flow.Size().SetFixed(1500)
	flow.Duration().FixedPackets().SetPackets(1000)
	e := flow.Packet().Add().Ethernet()
	e.Src().SetValue(srcMac)
	if iptype == "ipv4" {
		v4 := flow.Packet().Add().Ipv4()
		v4.Src().SetValue(srcIp)
		v4.Dst().Increment().SetStart(dstIp).SetCount(routeCount)
	} else {
		v6 := flow.Packet().Add().Ipv6()
		v6.Src().SetValue(srcIp)
		v6.Dst().Increment().SetStart(dstIp).SetCount(routeCount)
	}
}

type BGPNeighbor struct {
	as, pfxLimit uint32
	neighborip   string
	isV4         bool
}

func setPrefixLimitv4(dut *ondatra.DUTDevice, afisafi *oc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi, limit uint32) {
	if deviations.BGPExplicitPrefixLimitReceived(dut) {
		prefixLimitReceived := afisafi.GetOrCreateIpv4Unicast().GetOrCreatePrefixLimitReceived()
		prefixLimitReceived.MaxPrefixes = ygot.Uint32(limit)
	} else {
		prefixLimitReceived := afisafi.GetOrCreateIpv4Unicast().GetOrCreatePrefixLimit()
		prefixLimitReceived.MaxPrefixes = ygot.Uint32(limit)
	}
}

func setPrefixLimitv6(dut *ondatra.DUTDevice, afisafi *oc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi, limit uint32) {
	if deviations.BGPExplicitPrefixLimitReceived(dut) {
		prefixLimitReceived := afisafi.GetOrCreateIpv6Unicast().GetOrCreatePrefixLimitReceived()
		prefixLimitReceived.MaxPrefixes = ygot.Uint32(limit)
	} else {
		prefixLimitReceived := afisafi.GetOrCreateIpv6Unicast().GetOrCreatePrefixLimit()
		prefixLimitReceived.MaxPrefixes = ygot.Uint32(limit)
	}
}

func createBGPNeighbor(localAs, peerAs, pLimit uint32, restartTime uint16, dut *ondatra.DUTDevice) *oc.NetworkInstance_Protocol {
	nbrs := []*BGPNeighbor{
		{as: peerAs, pfxLimit: pLimit, neighborip: ateSrc.IPv4, isV4: true},
		{as: peerAs, pfxLimit: pLimit, neighborip: ateSrc.IPv6, isV4: false},
		{as: peerAs, pfxLimit: pLimit, neighborip: ateDst.IPv4, isV4: true},
		{as: peerAs, pfxLimit: pLimit, neighborip: ateDst.IPv6, isV4: false},
	}

	d := &oc.Root{}
	ni1 := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niProto := ni1.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := niProto.GetOrCreateBgp()

	global := bgp.GetOrCreateGlobal()
	global.As = ygot.Uint32(localAs)
	global.RouterId = ygot.String(dutSrc.IPv4)

	global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
	global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)

	// Note: we have to define the peer group even if we aren't setting any policy because it's
	// invalid OC for the neighbor to be part of a peer group that doesn't exist.
	pgv4 := bgp.GetOrCreatePeerGroup(peerGrpNamev4)
	pgv4.PeerAs = ygot.Uint32(peerAs)
	pgv4.PeerGroupName = ygot.String(peerGrpNamev4)
	pgv6 := bgp.GetOrCreatePeerGroup(peerGrpNamev6)
	pgv6.PeerAs = ygot.Uint32(peerAs)
	pgv6.PeerGroupName = ygot.String(peerGrpNamev6)

	for _, nbr := range nbrs {
		if nbr.isV4 {
			nv4 := bgp.GetOrCreateNeighbor(nbr.neighborip)
			nv4.PeerAs = ygot.Uint32(nbr.as)
			nv4.Enabled = ygot.Bool(true)
			nv4.PeerGroup = ygot.String(peerGrpNamev4)
			nv4.GetOrCreateTimers().RestartTime = ygot.Uint16(restartTime)
			afisafi := nv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
			afisafi.Enabled = ygot.Bool(true)
			nv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(false)
			setPrefixLimitv4(dut, afisafi, nbr.pfxLimit)
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
			nv6.PeerGroup = ygot.String(peerGrpNamev6)
			nv6.GetOrCreateTimers().RestartTime = ygot.Uint16(restartTime)
			afisafi6 := nv6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
			afisafi6.Enabled = ygot.Bool(true)
			nv6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(false)
			setPrefixLimitv6(dut, afisafi6, nbr.pfxLimit)
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
	nbrPath := statePath.Neighbor(ateDst.IPv4)
	nbrPathv6 := statePath.Neighbor(ateDst.IPv6)
	compare := func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
		state, ok := val.Val()
		if ok {
			if wantEstablished {
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

func getPrefixLimitv4(dut *ondatra.DUTDevice, neighbor *oc.NetworkInstance_Protocol_Bgp_Neighbor) (uint32, bool) {
	if deviations.BGPExplicitPrefixLimitReceived(dut) {
		prefixLimitReceived := neighbor.GetAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).GetIpv4Unicast().GetPrefixLimitReceived()
		return prefixLimitReceived.GetMaxPrefixes(), prefixLimitReceived.GetPrefixLimitExceeded()
	} else {
		prefixLimitReceived := neighbor.GetAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).GetIpv4Unicast().GetPrefixLimit()
		return prefixLimitReceived.GetMaxPrefixes(), prefixLimitReceived.GetPrefixLimitExceeded()
	}
}

func getPrefixLimitv6(dut *ondatra.DUTDevice, neighbor *oc.NetworkInstance_Protocol_Bgp_Neighbor) (uint32, bool) {
	if deviations.BGPExplicitPrefixLimitReceived(dut) {
		prefixLimitReceived := neighbor.GetAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).GetIpv6Unicast().GetPrefixLimitReceived()
		return prefixLimitReceived.GetMaxPrefixes(), prefixLimitReceived.GetPrefixLimitExceeded()
	} else {
		prefixLimitReceived := neighbor.GetAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).GetIpv6Unicast().GetPrefixLimit()
		return prefixLimitReceived.GetMaxPrefixes(), prefixLimitReceived.GetPrefixLimitExceeded()
	}
}

func verifyPrefixLimitTelemetry(t *testing.T, dut *ondatra.DUTDevice, neighbor *oc.NetworkInstance_Protocol_Bgp_Neighbor, wantEstablished bool) {
	t.Run("verifyPrefixLimitTelemetry", func(t *testing.T) {
		if *neighbor.NeighborAddress == ateDst.IPv4 {
			maxPrefix, limitExceeded := getPrefixLimitv4(dut, neighbor)
			if maxPrefix != prefixLimit {
				t.Errorf("PrefixLimit max-prefixes v4 mismatch: got %d, want %d", maxPrefix, prefixLimit)
			}
			if (wantEstablished && limitExceeded) || (!wantEstablished && !limitExceeded) {
				t.Errorf("PrefixLimitExceeded v4 mismatch: got %t, want %t", limitExceeded, !wantEstablished)
			}
		} else if *neighbor.NeighborAddress == ateDst.IPv6 {
			maxPrefix, limitExceeded := getPrefixLimitv6(dut, neighbor)
			if maxPrefix != prefixLimit {
				t.Errorf("PrefixLimit max-prefixes v6 mismatch: got %d, want %d", maxPrefix, prefixLimit)
			}
			if (wantEstablished && limitExceeded) || (!wantEstablished && !limitExceeded) {
				t.Errorf("PrefixLimitExceeded v6 mismatch: got %t, want %t", limitExceeded, !wantEstablished)
			}
		}
	})
}

func (tc *testCase) verifyBGPTelemetry(t *testing.T, dut *ondatra.DUTDevice) {
	t.Log("Waiting for BGPv4 neighbor to establish...")
	waitForBGPSession(t, dut, tc.wantEstablished)

	installedRoutes := tc.numRoutes
	if !tc.wantEstablished {
		installedRoutes = 0
	}

	compare := func(val *ygnmi.Value[uint32]) bool {
		c, ok := val.Val()
		return ok && c == installedRoutes
	}
	t.Log("Verifying BGP state")
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	prefixes := statePath.Neighbor(ateDst.IPv4).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Prefixes()
	if got, ok := gnmi.Watch(t, dut, prefixes.Received().State(), 2*time.Minute, compare).Await(t); !ok {
		t.Errorf("Received prefixes v4 mismatch: got %v, want %v", got, installedRoutes)
	}
	if got, ok := gnmi.Watch(t, dut, prefixes.Installed().State(), 2*time.Minute, compare).Await(t); !ok {
		t.Errorf("Installed prefixes v4 mismatch: got %v, want %v", got, installedRoutes)
	}
	nv4 := gnmi.Get(t, dut, statePath.Neighbor(ateDst.IPv4).State())
	verifyPrefixLimitTelemetry(t, dut, nv4, tc.wantEstablished)

	prefixesv6 := statePath.Neighbor(ateDst.IPv6).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Prefixes()
	if got, ok := gnmi.Watch(t, dut, prefixesv6.Installed().State(), time.Minute, compare).Await(t); !ok {
		t.Errorf("Installed prefixes v6 mismatch: got %v, want %v", got, installedRoutes)
	}
	if got, ok := gnmi.Watch(t, dut, prefixesv6.Received().State(), time.Minute, compare).Await(t); !ok {
		t.Errorf("Received prefixes v6 mismatch: got %v, want %v", got, installedRoutes)
	}
	nv6 := gnmi.Get(t, dut, statePath.Neighbor(ateDst.IPv6).State())
	verifyPrefixLimitTelemetry(t, dut, nv6, tc.wantEstablished)
}

func (tc *testCase) verifyNoPacketLoss(t *testing.T, ate *ondatra.ATEDevice, conf gosnappi.Config, tolerance float32, flowNames []string) {
	otg := ate.OTG()
	otgutils.LogFlowMetrics(t, otg, conf)
	for _, flow := range flowNames {
		recvMetric := gnmi.Get(t, otg, gnmi.OTG().Flow(flow).State())
		txPackets := float32(recvMetric.GetCounters().GetOutPkts())
		rxPackets := float32(recvMetric.GetCounters().GetInPkts())
		if txPackets == 0 {
			t.Fatalf("TxPkts = 0, want > 0")
		}
		lostPackets := txPackets - rxPackets
		lossPct := lostPackets * 100 / txPackets
		if lossPct > tolerance {
			t.Errorf("Traffic Loss Pct for Flow %s: got %v, want 0", flow, lossPct)
		} else {
			t.Logf("Traffic Test Passed! Got %v loss", lossPct)
		}
	}
}

func (tc *testCase) verifyPacketLoss(t *testing.T, ate *ondatra.ATEDevice, conf gosnappi.Config, tolerance float32, flowNames []string) {
	otg := ate.OTG()
	otgutils.LogFlowMetrics(t, otg, conf)
	for _, flow := range flowNames {
		recvMetric := gnmi.Get(t, otg, gnmi.OTG().Flow(flow).State())
		txPackets := float32(recvMetric.GetCounters().GetOutPkts())
		rxPackets := float32(recvMetric.GetCounters().GetInPkts())
		if txPackets == 0 {
			t.Fatalf("TxPkts = 0, want > 0")
		}
		lostPackets := txPackets - rxPackets
		lossPct := lostPackets * 100 / txPackets
		if lossPct >= (100-tolerance) && lossPct <= 100 {
			t.Logf("Traffic Test Passed! Loss seen as expected: got %v, want 100%% ", lossPct)
		} else {
			t.Errorf("Traffic %s is expected to fail: got %v, want 100%% failure", flow, lossPct)
		}
	}
}

func sendTraffic(t *testing.T, ate *ondatra.ATEDevice, duration time.Duration) {
	otg := ate.OTG()
	t.Log("Starting traffic")
	otg.StartTraffic(t)
	time.Sleep(duration)
	otg.StopTraffic(t)
	t.Log("Traffic stopped")
	time.Sleep(20 * time.Second)
}

func advertiseBGPRoutes(t *testing.T, conf gosnappi.Config, routeNames []string) {

	ate := ondatra.ATE(t, "ate")
	otg := ate.OTG()
	cs := gosnappi.NewControlState()
	cs.Protocol().Route().SetNames(routeNames).SetState(gosnappi.StateProtocolRouteState.ADVERTISE)
	otg.SetControlState(t, cs)

}

func withdrawBGPRoutes(t *testing.T, conf gosnappi.Config, routeNames []string) {

	ate := ondatra.ATE(t, "ate")
	otg := ate.OTG()
	cs := gosnappi.NewControlState()
	cs.Protocol().Route().SetNames(routeNames).SetState(gosnappi.StateProtocolRouteState.WITHDRAW)
	otg.SetControlState(t, cs)

}

type testCase struct {
	desc             string
	name             string
	numRoutes        uint32
	wantEstablished  bool
	wantNoPacketLoss bool
	routeNames       []string
}

func (tc *testCase) run(t *testing.T, conf gosnappi.Config, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice) {
	t.Log(tc.desc)
	flowNames := []string{}

	for _, name := range []string{"IPv4", "IPv6"} {
		if tc.numRoutes == prefixLimit {
			flowNames = append(flowNames, name+"."+"AtLimit")
		} else {
			flowNames = append(flowNames, name+"."+tc.name)
		}
	}

	advertiseBGPRoutes(t, conf, tc.routeNames)
	now := time.Now()

	// Verify Port Status
	t.Log(" Verifying port status")
	t.Run("verifyPortsUp", func(t *testing.T) {
		tc.verifyPortsUp(t, dut.Device)
	})

	// Verify BGP Parameters
	t.Log("Check BGP parameters with Prefix Limit not exceeded")
	t.Run("verifyBGPTelemetry", func(t *testing.T) {
		tc.verifyBGPTelemetry(t, dut)
	})
	// Time Duration for which maximum-prefix-restart-time has been active
	elapsed := time.Since(now)

	// Starting ATE Traffic
	t.Log("Verify Traffic statistics")
	if tc.name == "OverLimit" {
		trafficDurationOverlimit := grRestartTime - time.Duration(elapsed.Nanoseconds())
		sendTraffic(t, ate, trafficDurationOverlimit)
	} else {
		sendTraffic(t, ate, trafficDuration)
	}
	tolerance := float32(deviations.BGPTrafficTolerance(dut))
	if tc.wantNoPacketLoss {
		t.Run("verifyNoPacketLoss", func(t *testing.T) {
			tc.verifyNoPacketLoss(t, ate, conf, tolerance, flowNames)
		})
	} else {
		t.Run("verifyPacketLoss", func(t *testing.T) {
			tc.verifyPacketLoss(t, ate, conf, tolerance, flowNames)
		})
	}

	withdrawBGPRoutes(t, conf, tc.routeNames)
}

func TestTrafficBGPPrefixLimit(t *testing.T) {
	cases := []testCase{{
		name:             "UnderLimit",
		desc:             "BGP Prefixes within expected limit",
		numRoutes:        prefixLimit - 1,
		wantEstablished:  true,
		wantNoPacketLoss: true,
		routeNames:       []string{r4UnderLimit, r6UnderLimit},
	}, {
		name:             "AtLimit",
		desc:             "BGP Prefixes at threshold of expected limit",
		numRoutes:        prefixLimit,
		wantEstablished:  true,
		wantNoPacketLoss: true,
		routeNames:       []string{r4AtLimit, r6AtLimit},
	}, {
		name:             "OverLimit",
		desc:             "BGP Prefixes outside expected limit",
		numRoutes:        prefixLimit + 1,
		wantEstablished:  false,
		wantNoPacketLoss: false,
		routeNames:       []string{r4OverLimit, r6OverLimit},
	}, {
		name:             "ReestablishedAtLimit",
		desc:             "BGP Session ReEstablished after prefixes are within limits",
		numRoutes:        prefixLimit,
		wantEstablished:  true,
		wantNoPacketLoss: true,
		routeNames:       []string{r4AtLimit, r6AtLimit},
	}}

	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	// DUT Configuration
	t.Log("Start DUT interface Config")
	configureDUT(t, dut)

	// ATE Configuration.
	t.Log("Start ATE Config")
	conf := configureATE(t, ate)

	ate.OTG().StartProtocols(t)

	withdrawBGPRoutes(t, conf, []string{r4UnderLimit,
		r6UnderLimit,
		r4AtLimit,
		r6AtLimit,
		r4OverLimit,
		r6OverLimit})

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.run(t, conf, dut, ate)
		})
	}
}
