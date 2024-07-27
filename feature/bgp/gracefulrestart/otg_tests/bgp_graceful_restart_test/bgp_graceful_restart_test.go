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

package bgp_graceful_restart_test

import (
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	gnps "github.com/openconfig/gnoi/system"
	"github.com/openconfig/gnoigo/system"
	"github.com/openconfig/ondatra/gnmi/oc/acl"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/gnoi"
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
//   * Source: ate:port1 -> dut:port1 subnet 192.0.2.0/30 2001:db8::192:0:2:0/126
//   * Destination: dut:port2 -> ate:port2 subnet 192.0.2.4/30 2001:db8::192:0:2:4/126
//
// Note that the first (.0, .3) and last (.4, .7) IPv4 addresses are
// reserved from the subnet for broadcast, so a /30 leaves exactly 2
// usable addresses. This does not apply to IPv6 which allows /127
// for point to point links, but we use /126 so the numbering is
// consistent with IPv4.
//

const (
	trafficDuration          = 30 * time.Second
	grTimer                  = 2 * time.Minute
	triggerGrTimer           = 180
	stopDuration             = 45 * time.Second
	grRestartTime            = 120
	grStaleRouteTime         = 120
	ebgpV4AdvStartRoute      = "203.0.113.1"
	ebgpV6AdvStartRoute      = "2001:db8::203:0:113:1"
	ibgpV4AdvStartRoute      = "203.0.113.3"
	ibgpV6AdvStartRoute      = "2001:db8::203:0:113:3"
	advertisedRoutesv4Prefix = 32
	advertisedRoutesv6Prefix = 128
	routeCount               = 2
	dutAS                    = 64500
	ateAS                    = 64501
	plenIPv4                 = 30
	plenIPv6                 = 126
	bgpPort                  = 179
	peerv4GrpName            = "BGP-PEER-GROUP-V4"
	peerv6GrpName            = "BGP-PEER-GROUP-V6"
	aclNullPrefix            = "0.0.0.0/0"
	aclv6NullPrefix          = "::/0"
	aclName                  = "BGP-ACL"
	aclv6Name                = "ipv6-policy-acl"
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
	BGPDaemons = map[ondatra.Vendor]string{
		ondatra.ARISTA:  "Bgp-main",
		ondatra.CISCO:   "emsd",
		ondatra.JUNIPER: "rpd",
		ondatra.NOKIA:   "sr_bgp_mgr",
	}
)

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

// configureDUT configures all the interfaces and network instance on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	dc := gnmi.OC()
	i1 := dutSrc.NewOCInterface(dut.Port(t, "port1").Name(), dut)
	gnmi.Replace(t, dut, dc.Interface(i1.GetName()).Config(), i1)

	i2 := dutDst.NewOCInterface(dut.Port(t, "port2").Name(), dut)
	gnmi.Replace(t, dut, dc.Interface(i2.GetName()).Config(), i2)

	t.Log("Configure/update Network Instance")
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, dut.Port(t, "port1"))
		fptest.SetPortSpeed(t, dut.Port(t, "port2"))
	}
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, i1.GetName(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, i2.GetName(), deviations.DefaultNetworkInstance(dut), 0)
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

type bgpNeighbor struct {
	as         uint32
	neighborip string
	isV4       bool
}

func buildNbrList() []*bgpNeighbor {
	nbr1v4 := &bgpNeighbor{as: ateAS, neighborip: ateSrc.IPv4, isV4: true}
	nbr1v6 := &bgpNeighbor{as: ateAS, neighborip: ateSrc.IPv6, isV4: false}
	nbr2v4 := &bgpNeighbor{as: dutAS, neighborip: ateDst.IPv4, isV4: true}
	nbr2v6 := &bgpNeighbor{as: dutAS, neighborip: ateDst.IPv6, isV4: false}
	return []*bgpNeighbor{nbr1v4, nbr2v4, nbr1v6, nbr2v6}
}

func bgpWithNbr(as uint32, keepaliveTimer uint16, nbrs []*bgpNeighbor, dut *ondatra.DUTDevice) *oc.NetworkInstance_Protocol {
	d := &oc.Root{}
	ni1 := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niProto := ni1.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := niProto.GetOrCreateBgp()

	g := bgp.GetOrCreateGlobal()
	g.As = ygot.Uint32(as)
	g.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
	g.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)
	g.RouterId = ygot.String(dutDst.IPv4)
	bgpgr := g.GetOrCreateGracefulRestart()
	bgpgr.Enabled = ygot.Bool(true)

	pg := bgp.GetOrCreatePeerGroup(peerv4GrpName)
	pgGrV4 := pg.GetOrCreateGracefulRestart()
	pgGrV4.Enabled = ygot.Bool(true)
	pgGrV4.RestartTime = ygot.Uint16(grRestartTime)
	pgGrV4.StaleRoutesTime = ygot.Uint16(grStaleRouteTime)
	// pg.PeerAs = ygot.Uint32(ateAS)
	pg.PeerGroupName = ygot.String(peerv4GrpName)

	pgV6 := bgp.GetOrCreatePeerGroup(peerv6GrpName)
	pgGrV6 := pgV6.GetOrCreateGracefulRestart()
	pgGrV6.Enabled = ygot.Bool(true)
	pgGrV6.RestartTime = ygot.Uint16(grRestartTime)
	pgGrV6.StaleRoutesTime = ygot.Uint16(grStaleRouteTime)
	// pgv6.PeerAs = ygot.Uint32(ateAS)
	pgV6.PeerGroupName = ygot.String(peerv6GrpName)

	if deviations.RoutePolicyUnderAFIUnsupported(dut) {
		rpl := pg.GetOrCreateApplyPolicy()
		rpl.SetExportPolicy([]string{"ALLOW"})
		rpl.SetImportPolicy([]string{"ALLOW"})
		rplv6 := pgV6.GetOrCreateApplyPolicy()
		rplv6.SetExportPolicy([]string{"ALLOW"})
		rplv6.SetImportPolicy([]string{"ALLOW"})

	} else {
		pg1af4 := pg.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
		pg1af4.Enabled = ygot.Bool(true)

		pg1rpl4 := pg1af4.GetOrCreateApplyPolicy()
		pg1rpl4.SetExportPolicy([]string{"ALLOW"})
		pg1rpl4.SetImportPolicy([]string{"ALLOW"})

		pg1af6 := pgV6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
		pg1af6.Enabled = ygot.Bool(true)
		pg1rpl6 := pg1af6.GetOrCreateApplyPolicy()
		pg1rpl6.SetExportPolicy([]string{"ALLOW"})
		pg1rpl6.SetImportPolicy([]string{"ALLOW"})
	}

	for _, nbr := range nbrs {
		if nbr.isV4 {
			nv4 := bgp.GetOrCreateNeighbor(nbr.neighborip)
			nv4.PeerGroup = ygot.String(peerv4GrpName)
			nv4.GetOrCreateTimers().HoldTime = ygot.Uint16(3 * keepaliveTimer)
			nv4.GetOrCreateTimers().KeepaliveInterval = ygot.Uint16(keepaliveTimer)
			nv4.PeerAs = ygot.Uint32(nbr.as)
			nv4.Enabled = ygot.Bool(true)
			af4 := nv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
			af4.Enabled = ygot.Bool(true)
			af6 := nv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
			af6.Enabled = ygot.Bool(false)
		} else {
			nv6 := bgp.GetOrCreateNeighbor(nbr.neighborip)
			nv6.PeerGroup = ygot.String(peerv6GrpName)
			nv6.GetOrCreateTimers().HoldTime = ygot.Uint16(3 * keepaliveTimer)
			nv6.GetOrCreateTimers().KeepaliveInterval = ygot.Uint16(keepaliveTimer)
			nv6.PeerAs = ygot.Uint32(nbr.as)
			nv6.Enabled = ygot.Bool(true)
			nv6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
			af6 := nv6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
			af6.Enabled = ygot.Bool(true)
			af4 := nv6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
			af4.Enabled = ygot.Bool(false)
		}
	}
	return niProto
}

func checkBgpGRConfig(t *testing.T, dut *ondatra.DUTDevice) {
	t.Log("Verifying BGP configuration")
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	nbrPath := statePath.Neighbor(ateSrc.IPv4)
	nbrPathv6 := statePath.Neighbor(ateSrc.IPv6)

	isGrEnabled := gnmi.Get(t, dut, statePath.Global().GracefulRestart().Enabled().State())
	t.Logf("isGrEnabled %v", isGrEnabled)
	if isGrEnabled {
		t.Logf("Graceful restart on neighbor %v enabled as Expected", ateDst.IPv4)
	} else {
		t.Errorf("Expected Graceful restart status on neighbor: got %v, want Enabled", isGrEnabled)
	}

	grTimerVal := gnmi.Get(t, dut, nbrPath.GracefulRestart().RestartTime().State())
	t.Logf("grTimerVal %v", grTimerVal)
	if grTimerVal == uint16(grRestartTime) {
		t.Logf("Graceful restart timer enabled as expected to be %v", grRestartTime)
	} else {
		t.Errorf("Expected Graceful restart timer: got %v, want %v", grTimerVal, grRestartTime)
	}

	grTimerValV6 := gnmi.Get(t, dut, nbrPathv6.GracefulRestart().RestartTime().State())
	t.Logf("grTimerVal %v", grTimerValV6)
	if grTimerValV6 == uint16(grRestartTime) {
		t.Logf("Graceful restart timer enabled as expected to be %v", grRestartTime)
	} else {
		t.Errorf("Expected Graceful restart timer: got %v, want %v", grTimerValV6, grRestartTime)
	}
}

func checkBgpStatus(t *testing.T, dut *ondatra.DUTDevice) {
	t.Log("Verifying BGP state")
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()

	for _, attr := range []attrs.Attributes{ateSrc, ateDst} {

		nbrPath := statePath.Neighbor(attr.IPv4)
		nbrPathv6 := statePath.Neighbor(attr.IPv6)

		// Get BGP adjacency state
		t.Logf("Waiting for BGP neighbor %s to establish", attr.IPv4)
		_, ok := gnmi.Watch(t, dut, nbrPath.SessionState().State(), 2*time.Minute, func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
			currState, ok := val.Val()
			t.Logf("current state is %s", currState)
			return ok && currState == oc.Bgp_Neighbor_SessionState_ESTABLISHED
		}).Await(t)
		if !ok {
			fptest.LogQuery(t, "BGP reported state", nbrPath.State(), gnmi.Get(t, dut, nbrPath.State()))
			t.Fatal("No BGP neighbor formed...")
		}

		// Get BGPv6 adjacency state
		t.Logf("Waiting for BGPv6 neighbor %s to establish", attr.IPv6)
		_, ok = gnmi.Watch(t, dut, nbrPathv6.SessionState().State(), 2*time.Minute, func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
			currState, ok := val.Val()
			t.Logf("current state is %s", currState)
			return ok && currState == oc.Bgp_Neighbor_SessionState_ESTABLISHED
		}).Await(t)
		if !ok {
			fptest.LogQuery(t, "BGPv6 reported state", nbrPathv6.State(), gnmi.Get(t, dut, nbrPathv6.State()))
			t.Fatal("No BGPv6 neighbor formed...")
		}

		t.Log("Waiting for BGP v4 prefixes to be installed")
		got, found := gnmi.Watch(t, dut, nbrPath.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Prefixes().Installed().State(), 180*time.Second, func(val *ygnmi.Value[uint32]) bool {
			prefixCount, ok := val.Val()
			return ok && prefixCount == routeCount
		}).Await(t)
		if !found {
			t.Errorf("Installed prefixes v4 mismatch: got %v, want %v", got, routeCount)
		}
		t.Log("Waiting for BGP v6 prefixes to be installed")
		got, found = gnmi.Watch(t, dut, nbrPathv6.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Prefixes().Installed().State(), 180*time.Second, func(val *ygnmi.Value[uint32]) bool {
			prefixCount, ok := val.Val()
			return ok && prefixCount == routeCount
		}).Await(t)
		if !found {
			t.Errorf("Installed prefixes v6 mismatch: got %v, want %v", got, routeCount)
		}
	}
}

func configureATE(t *testing.T, ate *ondatra.ATEDevice, keepaliveTimer uint32) {
	t.Helper()
	config := gosnappi.NewConfig()
	p1 := ate.Port(t, "port1")
	ateSrc.AddToOTG(config, p1, &dutSrc)
	p2 := ate.Port(t, "port2")
	ateDst.AddToOTG(config, p2, &dutDst)
	srcDev := config.Devices().Items()[0]
	srcEth := srcDev.Ethernets().Items()[0]
	srcIpv4 := srcEth.Ipv4Addresses().Items()[0]
	srcIpv6 := srcEth.Ipv6Addresses().Items()[0]
	dstDev := config.Devices().Items()[1]
	dstEth := dstDev.Ethernets().Items()[0]
	dstIpv4 := dstEth.Ipv4Addresses().Items()[0]
	dstIpv6 := dstEth.Ipv6Addresses().Items()[0]

	srcBgp := srcDev.Bgp().SetRouterId(srcIpv4.Address())
	srcBgp4Peer := srcBgp.Ipv4Interfaces().Add().SetIpv4Name(srcIpv4.Name()).Peers().Add().SetName(ateSrc.Name + ".BGP4.peer")
	srcBgp4Peer.Advanced().SetKeepAliveInterval(keepaliveTimer).SetHoldTimeInterval(3 * keepaliveTimer)
	srcBgp4Peer.GracefulRestart().SetEnableGr(true).SetRestartTime(grRestartTime)
	srcBgp4Peer.SetPeerAddress(srcIpv4.Gateway()).SetAsNumber(ateAS).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
	srcBgp6Peer := srcBgp.Ipv6Interfaces().Add().SetIpv6Name(srcIpv6.Name()).Peers().Add().SetName(ateSrc.Name + ".BGP6.peer")
	srcBgp6Peer.Advanced().SetKeepAliveInterval(keepaliveTimer).SetHoldTimeInterval(3 * keepaliveTimer)
	srcBgp6Peer.GracefulRestart().SetEnableGr(true).SetRestartTime(grRestartTime)
	srcBgp6Peer.SetPeerAddress(srcIpv6.Gateway()).SetAsNumber(ateAS).SetAsType(gosnappi.BgpV6PeerAsType.EBGP)

	dstBgp := dstDev.Bgp().SetRouterId(dstIpv4.Address())
	dstBgp4Peer := dstBgp.Ipv4Interfaces().Add().SetIpv4Name(dstIpv4.Name()).Peers().Add().SetName(ateDst.Name + ".BGP4.peer")
	dstBgp4Peer.Advanced().SetKeepAliveInterval(keepaliveTimer).SetHoldTimeInterval(3 * keepaliveTimer)
	dstBgp4Peer.GracefulRestart().SetEnableGr(true).SetRestartTime(grRestartTime)
	dstBgp4Peer.SetPeerAddress(dstIpv4.Gateway()).SetAsNumber(dutAS).SetAsType(gosnappi.BgpV4PeerAsType.IBGP)
	dstBgp6Peer := dstBgp.Ipv6Interfaces().Add().SetIpv6Name(dstIpv6.Name()).Peers().Add().SetName(ateDst.Name + ".BGP6.peer")
	dstBgp6Peer.Advanced().SetKeepAliveInterval(keepaliveTimer).SetHoldTimeInterval(3 * keepaliveTimer)
	dstBgp6Peer.GracefulRestart().SetEnableGr(true).SetRestartTime(grRestartTime)
	dstBgp6Peer.SetPeerAddress(dstIpv6.Gateway()).SetAsNumber(dutAS).SetAsType(gosnappi.BgpV6PeerAsType.IBGP)

	srcBgp4PeerRoutes := srcBgp4Peer.V4Routes().Add().SetName("bgpNeti1")
	srcBgp4PeerRoutes.SetNextHopIpv4Address(srcIpv4.Address()).
		SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).
		SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)
	srcBgp4PeerRoutes.Addresses().Add().SetAddress(ebgpV4AdvStartRoute).SetPrefix(advertisedRoutesv4Prefix).SetCount(routeCount)
	srcBgp6PeerRoutes := srcBgp6Peer.V6Routes().Add().SetName("bgpNeti1v6")
	srcBgp6PeerRoutes.SetNextHopIpv6Address(srcIpv6.Address()).
		SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).
		SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL)
	srcBgp6PeerRoutes.Addresses().Add().SetAddress(ebgpV6AdvStartRoute).SetPrefix(advertisedRoutesv6Prefix).SetCount(routeCount)

	dstBgp4PeerRoutes := dstBgp4Peer.V4Routes().Add().SetName("bgpNeti2")
	dstBgp4PeerRoutes.SetNextHopIpv4Address(dstIpv4.Address()).
		SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).
		SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)
	dstBgp4PeerRoutes.Addresses().Add().SetAddress(ibgpV4AdvStartRoute).SetPrefix(advertisedRoutesv4Prefix).SetCount(routeCount)
	dstBgp6PeerRoutes := dstBgp6Peer.V6Routes().Add().SetName("bgpNeti2v6")
	dstBgp6PeerRoutes.SetNextHopIpv6Address(dstIpv6.Address()).
		SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).
		SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL)
	dstBgp6PeerRoutes.Addresses().Add().SetAddress(ibgpV6AdvStartRoute).SetPrefix(advertisedRoutesv6Prefix).SetCount(routeCount)

	ate.OTG().PushConfig(t, config)
	// ate.OTG().StartProtocols(t)
	t.Log("Pushing config to ATE and starting protocols...")

}

func configureFlow(t *testing.T, ate *ondatra.ATEDevice, src, dst attrs.Attributes, dstStart string) {

	// ATE Traffic Configuration
	t.Logf("start ate Traffic config")
	config := ate.OTG().FetchConfig(t)
	config.Flows().Clear()
	t.Logf("Creating the traffic flow with source %s and destination %s", src.IPv4, dstStart)
	flowipv4 := config.Flows().Add().SetName("Ipv4")
	flowipv4.Metrics().SetEnable(true)
	flowipv4.TxRx().Device().
		SetTxNames([]string{src.Name + ".IPv4"}).
		SetRxNames([]string{dst.Name + ".IPv4"})
	flowipv4.Size().SetFixed(512)
	flowipv4.Rate().SetPps(100)
	e1 := flowipv4.Packet().Add().Ethernet()
	e1.Src().SetValue(src.MAC)
	v4 := flowipv4.Packet().Add().Ipv4()
	v4.Src().SetValue(src.IPv4)
	v4.Dst().Increment().SetStart(dstStart).SetCount(routeCount)

	ate.OTG().PushConfig(t, config)
	ate.OTG().StartProtocols(t)

}

func verifyNoPacketLoss(t *testing.T, ate *ondatra.ATEDevice) {
	otg := ate.OTG()
	c := otg.FetchConfig(t)
	otgutils.LogFlowMetrics(t, otg, c)
	for _, f := range c.Flows().Items() {
		t.Logf("Verifying flow metrics for flow %s\n", f.Name())
		recvMetric := gnmi.Get(t, otg, gnmi.OTG().Flow(f.Name()).State())
		txPackets := float32(recvMetric.GetCounters().GetOutPkts())
		rxPackets := float32(recvMetric.GetCounters().GetInPkts())
		lostPackets := txPackets - rxPackets
		if txPackets == 0 {
			t.Fatalf("Tx packets should be higher than 0 for flow %s", f.Name())
		}
		if lossPct := lostPackets * 100 / txPackets; lossPct < 5.0 {
			t.Logf("Traffic Test Passed! Got %v loss", lossPct)
		} else {
			t.Errorf("Traffic Loss Pct for Flow %s: got %f", f.Name(), lossPct)
		}
	}
}

func confirmPacketLoss(t *testing.T, ate *ondatra.ATEDevice) {
	otg := ate.OTG()
	c := otg.FetchConfig(t)
	otgutils.LogFlowMetrics(t, otg, c)
	for _, f := range c.Flows().Items() {
		t.Logf("Verifying flow metrics for flow %s\n", f.Name())
		recvMetric := gnmi.Get(t, otg, gnmi.OTG().Flow(f.Name()).State())
		txPackets := float32(recvMetric.GetCounters().GetOutPkts())
		rxPackets := float32(recvMetric.GetCounters().GetInPkts())
		lostPackets := txPackets - rxPackets
		if txPackets == 0 {
			t.Fatalf("Tx packets should be higher than 0 for flow %s", f.Name())
		}
		if lossPct := lostPackets * 100 / txPackets; lossPct > 99.0 {
			t.Logf("Traffic Test Passed! Loss seen as expected: got %v, want 100%% ", lossPct)
		} else {
			t.Errorf("Traffic %s is expected to fail: got %f, want 100%% failure", f.Name(), lossPct)
		}
	}
}

func sendTraffic(t *testing.T, ate *ondatra.ATEDevice) {
	t.Helper()
	t.Logf("Starting traffic")
	ate.OTG().StartTraffic(t)
	time.Sleep(trafficDuration)
	t.Logf("Stop traffic")
	ate.OTG().StopTraffic(t)
}

// createGracefulRestartAction create a bgp control action for initiating the graceful restart process
func createGracefulRestartAction(t *testing.T, peerNames []string, restartDelay uint32) gosnappi.ControlAction {
	t.Helper()
	grAction := gosnappi.NewControlAction()
	grAction.Protocol().Bgp().InitiateGracefulRestart().
		SetPeerNames(peerNames).SetRestartDelay(restartDelay)
	return grAction
}

// findProcessByName uses telemetry to find out the PID of a process
func findProcessByName(t *testing.T, dut *ondatra.DUTDevice, pName string) uint64 {
	t.Helper()
	pList := gnmi.GetAll(t, dut, gnmi.OC().System().ProcessAny().State())
	var pID uint64
	for _, proc := range pList {
		if proc.GetName() == pName {
			pID = proc.GetPid()
			t.Logf("Pid of daemon '%s' is '%d'", pName, pID)
		}
	}
	return pID
}

// gNOIKillProcess kills a daemon on the DUT, given its name and pid.
func gNOIKillProcess(t *testing.T, dut *ondatra.DUTDevice, pName string, pID uint32, mode string) {
	t.Helper()
	if mode == "gracefully" {
		killResponse := gnoi.Execute(t, dut, system.NewKillProcessOperation().Name(pName).PID(pID).Signal(gnps.KillProcessRequest_SIGNAL_TERM).Restart(true))
		t.Logf("Got kill-terminate process response: %v\n\n", killResponse)
	}
	if mode == "abruptly" {
		killResponse := gnoi.Execute(t, dut, system.NewKillProcessOperation().Name(pName).PID(pID).Signal(gnps.KillProcessRequest_SIGNAL_KILL).Restart(true))
		t.Logf("Got kill process response: %v\n\n", killResponse)
	}
}

// gNOIBGPRequest sends soft or hard gnoi BGP notification
func gNOIBGPRequest(t *testing.T, mode string) {
	t.Helper()
	if mode == "soft" {
		// requestResponse := gnoi.Execute(t, dut, soft)
		t.Logf("Got kill-terminate process response: \n\n")

	}
	if mode == "hard" {
		// requestResponse := gnoi.Execute(t, dut, hard)
		t.Logf("Got kill process response: \n\n")
	}
}

func configACL(d *oc.Root, ateDstCIDR string) *oc.Acl_AclSet {
	acl := d.GetOrCreateAcl().GetOrCreateAclSet(aclName, oc.Acl_ACL_TYPE_ACL_IPV4)
	aclEntry10 := acl.GetOrCreateAclEntry(10)
	aclEntry10.SequenceId = ygot.Uint32(10)
	aclEntry10.GetOrCreateActions().ForwardingAction = oc.Acl_FORWARDING_ACTION_DROP
	a := aclEntry10.GetOrCreateIpv4()
	a.SourceAddress = ygot.String(aclNullPrefix)
	a.DestinationAddress = ygot.String(ateDstCIDR)

	aclEntry20 := acl.GetOrCreateAclEntry(20)
	aclEntry20.SequenceId = ygot.Uint32(20)
	aclEntry20.GetOrCreateActions().ForwardingAction = oc.Acl_FORWARDING_ACTION_DROP
	a2 := aclEntry20.GetOrCreateIpv4()
	a2.SourceAddress = ygot.String(ateDstCIDR)
	a2.DestinationAddress = ygot.String(aclNullPrefix)

	aclEntry30 := acl.GetOrCreateAclEntry(30)
	aclEntry30.SequenceId = ygot.Uint32(30)
	aclEntry30.GetOrCreateActions().ForwardingAction = oc.Acl_FORWARDING_ACTION_ACCEPT
	a3 := aclEntry30.GetOrCreateIpv4()
	a3.SourceAddress = ygot.String(aclNullPrefix)
	a3.DestinationAddress = ygot.String(aclNullPrefix)
	return acl
}

func configAdmitAllACL(d *oc.Root) *oc.Acl_AclSet {
	acl := d.GetOrCreateAcl().GetOrCreateAclSet(aclName, oc.Acl_ACL_TYPE_ACL_IPV4)
	acl.DeleteAclEntry(10)
	acl.DeleteAclEntry(20)
	return acl
}

func configACLInterface(iFace *oc.Acl_Interface, ifName string) *acl.Acl_InterfacePath {
	aclConf := gnmi.OC().Acl().Interface(ifName)
	if ifName != "" {
		iFace.GetOrCreateIngressAclSet(aclName, oc.Acl_ACL_TYPE_ACL_IPV4)
		iFace.GetOrCreateInterfaceRef().Interface = ygot.String(ifName)
		iFace.GetOrCreateInterfaceRef().Subinterface = ygot.Uint32(0)
	} else {
		iFace.GetOrCreateIngressAclSet(aclName, oc.Acl_ACL_TYPE_ACL_IPV4)
		iFace.DeleteIngressAclSet(aclName, oc.Acl_ACL_TYPE_ACL_IPV4)
	}
	return aclConf
}

func verifyBGPActive(t *testing.T, mode string, dst attrs.Attributes) {
	t.Logf("Waiting for %s BGP neighbor %s to go to ACTIVE state", mode, dst.IPv4)
	dut := ondatra.DUT(t, "dut")
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	nbrPath := statePath.Neighbor(dst.IPv4)

	bgpState := oc.Bgp_Neighbor_SessionState_ACTIVE
	_, ok := gnmi.Watch(t, dut, nbrPath.SessionState().State(), 2*time.Minute, func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
		currState, ok := val.Val()
		t.Logf("current state of neighbour is %s", currState.String())
		return ok && currState == bgpState
	}).Await(t)
	if !ok {
		fptest.LogQuery(t, "BGP reported state", nbrPath.State(), gnmi.Get(t, dut, nbrPath.State()))
		t.Errorf("BGP session did not go ACTIVE as expected")
	}
}

func blockBGPTCP(t *testing.T, dst attrs.Attributes, dutDstIfName string) *oc.Acl_Interface {
	d := &oc.Root{}
	dut := ondatra.DUT(t, "dut")
	dstCIDR := dst.IPv4 + "/32"
	iFace := d.GetOrCreateAcl().GetOrCreateInterface(dutDstIfName)
	gnmi.Replace(t, dut, gnmi.OC().Acl().AclSet(aclName, oc.Acl_ACL_TYPE_ACL_IPV4).Config(), configACL(d, dstCIDR))
	aclConf := configACLInterface(iFace, dutDstIfName)
	gnmi.Replace(t, dut, aclConf.Config(), iFace)
	return iFace

}

func unblockBGPTCP(t *testing.T, iface *oc.Acl_Interface, dutDstIfName string) {
	d := &oc.Root{}
	dut := ondatra.DUT(t, "dut")
	gnmi.Replace(t, dut, gnmi.OC().Acl().AclSet(aclName, oc.Acl_ACL_TYPE_ACL_IPV4).Config(), configAdmitAllACL(d))
	aclPath := configACLInterface(iface, dutDstIfName)
	gnmi.Replace(t, dut, aclPath.Config(), iface)

}

func TestBGPPGracefulRestart(t *testing.T) {
	cases := []struct {
		name      string
		restarter string
		mode      string
		desc      string
	}{{
		name:      "RT-1.4.2 Restart DUT Speaker Gracefully",
		restarter: "speaker",
		mode:      "gracefully",
		desc:      "RT-1.4.2 Restarting DUT speaker whose BGP process was killed gracefully",
	}, {
		name:      "RT-1.4.3 Restart DUT Speaker Abruptly",
		restarter: "speaker",
		mode:      "abruptly",
		desc:      "RT-1.4.3 Restarting DUT speaker whose BGP process was killed abruptly",
	}, {
		name:      "RT-1.4.4 Restart Receiver Gracefully",
		restarter: "receiver",
		mode:      "gracefully",
		desc:      "RT-1.4.4 DUT Helper for a restarting EBGP speaker whose BGP process was gracefully killed",
	}, {
		name:      "RT-1.4.5 Restart Receiver Abruptly",
		restarter: "receiver",
		mode:      "abruptly",
		desc:      "RT-1.4.5 DUT Helper for a restarting EBGP speaker whose BGP process was killed abruptly",
	}}

	t.Run("RT-1.4.1 Enable and validate BGP Graceful restart feature", func(t *testing.T) {
		dut := ondatra.DUT(t, "dut")
		ate := ondatra.ATE(t, "ate")

		// ATE Configuration.
		t.Log("Start ATE Config")
		configureATE(t, ate, 60)

		// Configure interface on the DUT
		t.Log("Start DUT interface Config")
		configureDUT(t, dut)
		configureRoutePolicy(t, dut, "ALLOW", oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)

		// Configure BGP+Neighbors on the DUT
		t.Log("Configure BGP with Graceful Restart option under Global Bgp")
		dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
		nbrList := buildNbrList()
		dutConf := bgpWithNbr(dutAS, 60, nbrList, dut)
		gnmi.Replace(t, dut, dutConfPath.Config(), dutConf)
		fptest.LogQuery(t, "DUT BGP Config", dutConfPath.Config(), gnmi.Get(t, dut, dutConfPath.Config()))

		// Start protocols
		ate.OTG().StartProtocols(t)
		// Verify Port Status
		t.Run("verify DUT Ports", func(t *testing.T) {
			t.Log("Verifying port status")
			verifyPortsUp(t, dut.Device)
		})
		t.Run("Verify BGP Parameters", func(t *testing.T) {
			t.Log("Check BGP parameters")
			checkBgpGRConfig(t, dut)
		})
		t.Run("Check BGP status", func(t *testing.T) {
			t.Log("Check BGP status")
			checkBgpStatus(t, dut)
		})

	})

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dut := ondatra.DUT(t, "dut")
			ate := ondatra.ATE(t, "ate")

			var src, dst attrs.Attributes
			var dstStart string
			modes := []string{"IBGP", "EBGP"}
			for _, mode := range modes {
				if mode == "EBGP" {
					src = ateDst
					dst = ateSrc
					dstStart = ebgpV4AdvStartRoute
				}
				if mode == "IBGP" {
					src = ateSrc
					dst = ateDst
					dstStart = ibgpV4AdvStartRoute
				}
				t.Log(tc.desc)
				t.Logf("Starting the test for %s", mode)
				// Creating traffic
				configureFlow(t, ate, src, dst, dstStart)
				// Starting traffic before graceful restart from DUT
				checkBgpStatus(t, dut)
				t.Log("Starting traffic before Graceful restart trigger from DUT")
				ate.OTG().StartTraffic(t)
				var startTime time.Time

				if tc.restarter == "speaker" {
					var pId uint64
					pName := BGPDaemons[dut.Vendor()]
					t.Logf("Finding the process id")
					pId = findProcessByName(t, dut, pName)
					if pId == 0 {
						t.Fatalf("Couldn't find pid of BGP daemon '%s'", pName)
					} else {
						t.Logf("Pid of BGP daemon '%s' is '%d'", pName, pId)
					}
					// Kill BGP daemon through gNOI Kill Request.
					t.Logf("Kill the BGP process on the dut")
					gNOIKillProcess(t, dut, pName, uint32(pId), tc.mode)
					startTime = time.Now()
					time.Sleep(2 * time.Second)

				}

				if tc.restarter == "receiver" {
					if tc.mode == "gracefully" {
						// Send Graceful Restart Trigger from ATE to DUT within the GR timer configured on the DUT
						t.Log("Send Graceful Restart Trigger from OTG to DUT")
						ate.OTG().SetControlAction(t, createGracefulRestartAction(t, []string{dst.Name + ".BGP4.peer"}, triggerGrTimer))
						startTime = time.Now()
						t.Log("Sending Traffic while GR timer counting down. Traffic should pass as BGP GR is enabled!")
					}
					if tc.mode == "abruptly" {
						t.Logf("Stop BGP on the %s ATE Peer", mode)
						stopBgp := gosnappi.NewControlState()
						stopBgp.Protocol().Bgp().Peers().SetPeerNames([]string{dst.Name + ".BGP4.peer"}).
							SetState(gosnappi.StateProtocolBgpPeersState.DOWN)
						ate.OTG().SetControlState(t, stopBgp)
						startTime = time.Now()

					}
				}

				t.Run("Verify BGP NOT Established", func(t *testing.T) {
					verifyBGPActive(t, mode, dst)
				})

				if tc.restarter == "speaker" {
					t.Logf("Stop BGP on the %s ATE Peer to delay the BGP reestablishment for a period longer than the stale routes timer", mode)
					stopBgp := gosnappi.NewControlState()
					stopBgp.Protocol().Bgp().Peers().SetPeerNames([]string{dst.Name + ".BGP4.peer"}).
						SetState(gosnappi.StateProtocolBgpPeersState.DOWN)
					ate.OTG().SetControlState(t, stopBgp)

				}
				replaceDuration := time.Since(startTime)
				waitDuration := grStaleRouteTime*time.Second - replaceDuration - 10*time.Second
				t.Logf("Waiting for %s short of stale route time expiration", waitDuration)
				time.Sleep(waitDuration)
				ate.OTG().StopTraffic(t)
				t.Run("Verify no Packet Loss for "+mode, func(t *testing.T) {
					verifyNoPacketLoss(t, ate)
				})
				t.Run("Verify BGP still ACTIVE", func(t *testing.T) {
					verifyBGPActive(t, mode, dst)
				})

				waitDuration = grStaleRouteTime*time.Second - time.Since(startTime) + 30*time.Second
				t.Logf("Waiting another %s seconds to ensure the stale time expired", waitDuration)
				time.Sleep(waitDuration)

				sendTraffic(t, ate)
				t.Run("Confirm Packet Loss for "+mode, func(t *testing.T) {
					confirmPacketLoss(t, ate)
				})
			}
		})
	}
	t.Run("RT-1.4.6 Test support for RFC8538 compliance by letting Hold-time expire", func(t *testing.T) {
		dut := ondatra.DUT(t, "dut")
		ate := ondatra.ATE(t, "ate")
		var keepaliveTimer uint16 = 30

		// ATE Configuration.
		t.Log("Start ATE Config")

		configureATE(t, ate, uint32(keepaliveTimer))

		// Configure interface on the DUT
		t.Log("Start DUT interface Config")
		configureDUT(t, dut)
		configureRoutePolicy(t, dut, "ALLOW", oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)

		// Configure BGP+Neighbors on the DUT with longer staleroute timer than holdtime timer
		t.Log("Configure BGP with Graceful Restart option under Global Bgp")
		dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
		nbrList := buildNbrList()
		dutConf := bgpWithNbr(dutAS, keepaliveTimer, nbrList, dut)
		gnmi.Replace(t, dut, dutConfPath.Config(), dutConf)
		fptest.LogQuery(t, "DUT BGP Config", dutConfPath.Config(), gnmi.Get(t, dut, dutConfPath.Config()))

		// Start protocols
		ate.OTG().StartProtocols(t)
		t.Run("Check BGP status", func(t *testing.T) {
			t.Log("Check BGP status")
			checkBgpStatus(t, dut)
		})

		var src, dst attrs.Attributes
		var dstStart, dutDstIfName string
		modes := []string{"IBGP", "EBGP"}
		for _, mode := range modes {
			if mode == "EBGP" {
				src = ateDst
				dst = ateSrc
				dstStart = ebgpV4AdvStartRoute
				dutDstIfName = dut.Port(t, "port1").Name()
			}
			if mode == "IBGP" {
				src = ateSrc
				dst = ateDst
				dstStart = ibgpV4AdvStartRoute
				dutDstIfName = dut.Port(t, "port2").Name()
			}
			t.Logf("Starting the test for %s", mode)
			// Creating traffic
			configureFlow(t, ate, src, dst, dstStart)
			checkBgpStatus(t, dut)

			t.Logf("Configure Acl to block BGP on port 179 on interface %s", dutDstIfName)
			iFace := blockBGPTCP(t, dst, dutDstIfName)
			startTime := time.Now()

			ate.OTG().StartTraffic(t)
			holdTimer := 3 * keepaliveTimer
			waitDuration := time.Duration(holdTimer)*time.Second - time.Since(startTime) + 10*time.Second
			t.Logf("Waiting %s seconds to ensure the hold time expired", waitDuration)
			time.Sleep(waitDuration)

			t.Run("Verify no Packet Loss for "+mode, func(t *testing.T) {
				verifyNoPacketLoss(t, ate)
			})
			startTime = time.Now()
			t.Run("Verify BGP NOT Established", func(t *testing.T) {
				verifyBGPActive(t, mode, dst)
			})

			waitDuration = time.Duration(grStaleRouteTime)*time.Second - time.Since(startTime)
			time.Sleep(waitDuration)
			sendTraffic(t, ate)
			t.Run("Confirm Packet Loss for "+mode, func(t *testing.T) {
				confirmPacketLoss(t, ate)
			})

			t.Logf("Removing Acl on the dut interface %s to restore BGP", dutDstIfName)
			unblockBGPTCP(t, iFace, dutDstIfName)

		}
	})

	nextCases := []struct {
		name         string
		direction    string
		notification string
		desc         string
	}{{
		name:         "RT-1.4.7 Send Soft Notification",
		direction:    "send",
		notification: "soft",
		desc:         "RT-1.4.7 Test support for RFC8538 compliance by sending a BGP Notification message to the peer",
	}, {
		name:         "RT-1.4.8 Receive Soft Notification",
		direction:    "receive",
		notification: "hard",
		desc:         "RT-1.4.8 Test support for RFC8538 compliance by receiving a BGP Notification message from the peer",
	}, {
		name:         "RT-1.4.9 Send Hard Notification",
		direction:    "send",
		notification: "soft",
		desc:         "RT-1.4.9 Test support for RFC8538 compliance by sending a BGP Hard Notification message to the peer",
	}, {
		name:         "RT-1.4.10 Receive Hard Notification",
		direction:    "receive",
		notification: "hard",
		desc:         "RT-1.4.10 Test support for RFC8538 compliance by receiving a BGP Hard Notification message from the peer",
	}}

	for _, tc := range nextCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Skip()
			dut := ondatra.DUT(t, "dut")
			ate := ondatra.ATE(t, "ate")
			// ATE Configuration.
			t.Log("Start ATE Config")
			configureATE(t, ate, 60)

			var src, dst attrs.Attributes
			var dstStart, dutDstIfName string
			modes := []string{"EBGP", "IBGP"}
			for _, mode := range modes {
				if mode == "EBGP" {
					src = ateDst
					dst = ateSrc
					dstStart = ebgpV4AdvStartRoute
					dutDstIfName = dut.Port(t, "port1").Name()
				}
				if mode == "IBGP" {
					src = ateSrc
					dst = ateDst
					dstStart = ibgpV4AdvStartRoute
					dutDstIfName = dut.Port(t, "port2").Name()
				}
				t.Log(tc.desc)
				t.Logf("Starting the test for %s", mode)
				// Creating traffic
				configureFlow(t, ate, src, dst, dstStart)
				checkBgpStatus(t, dut)
				// Starting traffic before clear BGP notifications
				t.Log("Starting traffic before clear BGP notifications")
				ate.OTG().StartTraffic(t)

				if tc.direction == "send" {
					// Sending BGP clear request
					t.Logf("Sending Clear BGP Notification")
					gNOIBGPRequest(t, tc.notification)
					time.Sleep(2 * time.Second)
				}
				if tc.direction == "receive" {
					// Receiving BGP clear request. Ate is sending
					t.Logf("Receiving Clear BGP Notification. ATE is sending the notification")
					customAction := gosnappi.NewControlAction()
					if tc.notification == "soft" {
						// to add the correct code and subcode
						customAction.Protocol().Bgp().Notification().SetNames([]string{dst.Name + ".BGP4.peer"}).Custom().SetCode(1).SetSubcode(6)
					}
					if tc.notification == "hard" {
						// to add the correct code and subcode
						customAction.Protocol().Bgp().Notification().SetNames([]string{dst.Name + ".BGP4.peer"}).Custom().SetCode(6).SetSubcode(6)
					}
				}
				t.Logf("Configure Acl to block BGP on port 179 on interface %s", dutDstIfName)
				iFace := blockBGPTCP(t, dst, dutDstIfName)
				startTime := time.Now()

				replaceDuration := time.Since(startTime)
				waitDuration := grStaleRouteTime*time.Second - replaceDuration - 10*time.Second
				t.Logf("Waiting for %s short of stale route time expiration", waitDuration)
				time.Sleep(waitDuration)
				ate.OTG().StopTraffic(t)
				t.Run("Verify no Packet Loss for "+mode, func(t *testing.T) {
					verifyNoPacketLoss(t, ate)
				})

				waitDuration = grStaleRouteTime*time.Second - time.Since(startTime) + 30*time.Second
				t.Logf("Waiting another %s seconds to ensure the stale time expired", waitDuration)
				time.Sleep(waitDuration)

				sendTraffic(t, ate)
				t.Run("Confirm Packet Loss for "+mode, func(t *testing.T) {
					confirmPacketLoss(t, ate)
				})
				t.Logf("Removing Acl on the dut interface %s to restore BGP", dutDstIfName)
				unblockBGPTCP(t, iFace, dutDstIfName)

			}
		})
	}

}
