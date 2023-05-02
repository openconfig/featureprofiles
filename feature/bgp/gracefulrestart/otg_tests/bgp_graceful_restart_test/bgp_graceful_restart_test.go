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
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/gnmi/oc/acl"
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
	trafficDuration          = 1 * time.Minute
	grTimer                  = 2 * time.Minute
	grRestartTime            = 120
	grStaleRouteTime         = 300
	ipv4SrcTraffic           = "192.0.2.2"
	ipv6SrcTraffic           = "2001:db8::192:0:2:2"
	ipv4DstTrafficStart      = "203.0.113.1"
	ipv4DstTrafficEnd        = "203.0.113.2"
	ipv6DstTrafficStart      = "2001:db8::203:0:113:1"
	ipv6DstTrafficEnd        = "2001:db8::203:0:113:2"
	advertisedRoutesv4Net    = "203.0.113.1"
	advertisedRoutesv6Net    = "2001:db8::203:0:113:1"
	advertisedRoutesv4Net2   = "203.0.113.3"
	advertisedRoutesv6Net2   = "2001:db8::203:0:113:3"
	advertisedRoutesv4Prefix = 32
	advertisedRoutesv6Prefix = 128
	aclNullPrefix            = "0.0.0.0/0"
	aclv6NullPrefix          = "::/0"
	aclName                  = "BGP-DENY-ACL"
	aclv6Name                = "ipv6-policy-acl"
	routeCount               = 2
	dutAS                    = 64500
	ateAS                    = 64501
	plenIPv4                 = 30
	plenIPv6                 = 126
	bgpPort                  = 179
	peerv4GrpName            = "BGP-PEER-GROUP-V4"
	peerv6GrpName            = "BGP-PEER-GROUP-V6"
	ateDstCIDR               = "192.0.2.6/32"
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

func configureRoutePolicy(t *testing.T, dut *ondatra.DUTDevice, name string, pr oc.E_RoutingPolicy_PolicyResultType) {
	d := &oc.Root{}
	rp := d.GetOrCreateRoutingPolicy()
	pd := rp.GetOrCreatePolicyDefinition(name)
	st := pd.GetOrCreateStatement("id-1")
	st.GetOrCreateActions().PolicyResult = pr
	gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)
}

// configureDUT configures all the interfaces and network instance on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	dc := gnmi.OC()
	i1 := dutSrc.NewOCInterface(dut.Port(t, "port1").Name())
	gnmi.Replace(t, dut, dc.Interface(i1.GetName()).Config(), i1)

	i2 := dutDst.NewOCInterface(dut.Port(t, "port2").Name())
	gnmi.Replace(t, dut, dc.Interface(i2.GetName()).Config(), i2)

	t.Log("Configure/update Network Instance")
	dutConfNIPath := dc.NetworkInstance(*deviations.DefaultNetworkInstance)
	gnmi.Replace(t, dut, dutConfNIPath.Type().Config(), oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)

	if *deviations.ExplicitPortSpeed {
		fptest.SetPortSpeed(t, dut.Port(t, "port1"))
		fptest.SetPortSpeed(t, dut.Port(t, "port2"))
	}
	if *deviations.ExplicitInterfaceInDefaultVRF {
		fptest.AssignToNetworkInstance(t, dut, i1.GetName(), *deviations.DefaultNetworkInstance, 0)
		fptest.AssignToNetworkInstance(t, dut, i2.GetName(), *deviations.DefaultNetworkInstance, 0)
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

func buildNbrList(asN uint32) []*bgpNeighbor {
	nbr1v4 := &bgpNeighbor{as: asN, neighborip: ateSrc.IPv4, isV4: true}
	nbr1v6 := &bgpNeighbor{as: asN, neighborip: ateSrc.IPv6, isV4: false}
	nbr2v4 := &bgpNeighbor{as: asN, neighborip: ateDst.IPv4, isV4: true}
	nbr2v6 := &bgpNeighbor{as: asN, neighborip: ateDst.IPv6, isV4: false}
	return []*bgpNeighbor{nbr1v4, nbr2v4, nbr1v6, nbr2v6}
}

func bgpWithNbr(as uint32, nbrs []*bgpNeighbor) *oc.NetworkInstance_Protocol {
	d := &oc.Root{}
	ni1 := d.GetOrCreateNetworkInstance(*deviations.DefaultNetworkInstance)
	ni_proto := ni1.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := ni_proto.GetOrCreateBgp()

	g := bgp.GetOrCreateGlobal()
	g.As = ygot.Uint32(as)
	g.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
	g.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)
	g.RouterId = ygot.String(dutDst.IPv4)
	bgpgr := g.GetOrCreateGracefulRestart()
	bgpgr.Enabled = ygot.Bool(true)
	bgpgr.RestartTime = ygot.Uint16(grRestartTime)
	bgpgr.StaleRoutesTime = ygot.Uint16(grStaleRouteTime)

	pg := bgp.GetOrCreatePeerGroup(peerv4GrpName)
	pg.PeerAs = ygot.Uint32(ateAS)
	pg.PeerGroupName = ygot.String(peerv4GrpName)

	pgv6 := bgp.GetOrCreatePeerGroup(peerv6GrpName)
	pgv6.PeerAs = ygot.Uint32(ateAS)
	pgv6.PeerGroupName = ygot.String(peerv6GrpName)

	if *deviations.RoutePolicyUnderPeerGroup {
		pg1af4 := pg.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
		pg1af4.Enabled = ygot.Bool(true)

		pg1rpl4 := pg1af4.GetOrCreateApplyPolicy()
		pg1rpl4.SetExportPolicy([]string{"ALLOW"})
		pg1rpl4.SetImportPolicy([]string{"ALLOW"})

		pg1af6 := pgv6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
		pg1af6.Enabled = ygot.Bool(true)
		pg1rpl6 := pg1af6.GetOrCreateApplyPolicy()
		pg1rpl6.SetExportPolicy([]string{"ALLOW"})
		pg1rpl6.SetImportPolicy([]string{"ALLOW"})
	} else {
		rpl := pg.GetOrCreateApplyPolicy()
		rpl.SetExportPolicy([]string{"ALLOW"})
		rpl.SetImportPolicy([]string{"ALLOW"})
		rplv6 := pgv6.GetOrCreateApplyPolicy()
		rplv6.SetExportPolicy([]string{"ALLOW"})
		rplv6.SetImportPolicy([]string{"ALLOW"})
	}

	for _, nbr := range nbrs {
		if nbr.isV4 {
			nv4 := bgp.GetOrCreateNeighbor(nbr.neighborip)
			nv4.PeerGroup = ygot.String(peerv4GrpName)
			nv4.GetOrCreateTimers().HoldTime = ygot.Uint16(180)
			nv4.GetOrCreateTimers().KeepaliveInterval = ygot.Uint16(60)
			nv4.PeerAs = ygot.Uint32(nbr.as)
			nv4.Enabled = ygot.Bool(true)
			af4 := nv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
			af4.Enabled = ygot.Bool(true)
			af6 := nv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
			af6.Enabled = ygot.Bool(false)
		} else {
			nv6 := bgp.GetOrCreateNeighbor(nbr.neighborip)
			nv6.PeerGroup = ygot.String(peerv6GrpName)
			nv6.GetOrCreateTimers().HoldTime = ygot.Uint16(180)
			nv6.GetOrCreateTimers().KeepaliveInterval = ygot.Uint16(60)
			nv6.PeerAs = ygot.Uint32(nbr.as)
			nv6.Enabled = ygot.Bool(true)
			nv6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
			af6 := nv6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
			af6.Enabled = ygot.Bool(true)
			af4 := nv6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
			af4.Enabled = ygot.Bool(false)
		}
	}
	return ni_proto
}

func checkBgpStatus(t *testing.T, dut *ondatra.DUTDevice) {
	t.Log("Verifying BGP state")
	statePath := gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	nbrPath := statePath.Neighbor(ateSrc.IPv4)
	nbrPathv6 := statePath.Neighbor(ateSrc.IPv6)

	// Get BGP adjacency state
	t.Log("Waiting for BGP neighbor to establish...")
	_, ok := gnmi.Watch(t, dut, nbrPath.SessionState().State(), 2*time.Minute, func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
		currState, ok := val.Val()
		return ok && currState == oc.Bgp_Neighbor_SessionState_ESTABLISHED
	}).Await(t)
	if !ok {
		fptest.LogQuery(t, "BGP reported state", nbrPath.State(), gnmi.Get(t, dut, nbrPath.State()))
		t.Fatal("No BGP neighbor formed...")
	}

	// Get BGPv6 adjacency state
	t.Log("Waiting for BGPv6 neighbor to establish...")
	_, ok = gnmi.Watch(t, dut, nbrPathv6.SessionState().State(), 2*time.Minute, func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
		currState, ok := val.Val()
		return ok && currState == oc.Bgp_Neighbor_SessionState_ESTABLISHED
	}).Await(t)
	if !ok {
		fptest.LogQuery(t, "BGPv6 reported state", nbrPathv6.State(), gnmi.Get(t, dut, nbrPathv6.State()))
		t.Fatal("No BGPv6 neighbor formed...")
	}

	isGrEnabled := gnmi.Get(t, dut, statePath.Global().GracefulRestart().Enabled().State())
	t.Logf("isGrEnabled %v", isGrEnabled)
	if isGrEnabled {
		t.Logf("Graceful restart on neighbor %v enabled as Expected", ateDst.IPv4)
	} else {
		t.Errorf("Expected Graceful restart status on neighbor: got %v, want Enabled", isGrEnabled)
	}

	grTimerVal := gnmi.Get(t, dut, statePath.Global().GracefulRestart().RestartTime().State())
	t.Logf("grTimerVal %v", grTimerVal)
	if grTimerVal == uint16(grRestartTime) {
		t.Logf("Graceful restart timer enabled as expected to be %v", grRestartTime)
	} else {
		t.Errorf("Expected Graceful restart timer: got %v, want %v", grTimerVal, grRestartTime)
	}

	t.Log("Waiting for BGP v4 prefix to be installed")
	got, found := gnmi.Watch(t, dut, nbrPath.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Prefixes().Installed().State(), 180*time.Second, func(val *ygnmi.Value[uint32]) bool {
		prefixCount, ok := val.Val()
		return ok && prefixCount == routeCount
	}).Await(t)
	if !found {
		t.Errorf("Installed prefixes v4 mismatch: got %v, want %v", got, routeCount)
	}
	t.Log("Waiting for BGP v6 prefix to be installed")
	got, found = gnmi.Watch(t, dut, nbrPathv6.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Prefixes().Installed().State(), 180*time.Second, func(val *ygnmi.Value[uint32]) bool {
		prefixCount, ok := val.Val()
		return ok && prefixCount == routeCount
	}).Await(t)
	if !found {
		t.Errorf("Installed prefixes v6 mismatch: got %v, want %v", got, routeCount)
	}
}

func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {

	config := ate.OTG().NewConfig(t)
	srcPort := config.Ports().Add().SetName("port1")
	dstPort := config.Ports().Add().SetName("port2")

	srcDev := config.Devices().Add().SetName(ateSrc.Name)
	srcEth := srcDev.Ethernets().Add().SetName(ateSrc.Name + ".Eth").SetMac(ateSrc.MAC)
	srcEth.Connection().SetChoice(gosnappi.EthernetConnectionChoice.PORT_NAME).SetPortName(srcPort.Name())
	srcIpv4 := srcEth.Ipv4Addresses().Add().SetName(ateSrc.Name + ".IPv4")
	srcIpv4.SetAddress(ateSrc.IPv4).SetGateway(dutSrc.IPv4).SetPrefix(int32(ateSrc.IPv4Len))
	srcIpv6 := srcEth.Ipv6Addresses().Add().SetName(ateSrc.Name + ".IPv6")
	srcIpv6.SetAddress(ateSrc.IPv6).SetGateway(dutSrc.IPv6).SetPrefix(int32(ateSrc.IPv6Len))

	dstDev := config.Devices().Add().SetName(ateDst.Name)
	dstEth := dstDev.Ethernets().Add().SetName(ateDst.Name + ".Eth").SetMac(ateDst.MAC)
	dstEth.Connection().SetChoice(gosnappi.EthernetConnectionChoice.PORT_NAME).SetPortName(dstPort.Name())
	dstIpv4 := dstEth.Ipv4Addresses().Add().SetName(ateDst.Name + ".IPv4")
	dstIpv4.SetAddress(ateDst.IPv4).SetGateway(dutDst.IPv4).SetPrefix(int32(ateDst.IPv4Len))
	dstIpv6 := dstEth.Ipv6Addresses().Add().SetName(ateDst.Name + ".IPv6")
	dstIpv6.SetAddress(ateDst.IPv6).SetGateway(dutDst.IPv6).SetPrefix(int32(ateDst.IPv6Len))

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

	srcBgp4PeerRoutes := dstBgp4Peer.V4Routes().Add().SetName("bgpNeti1")
	srcBgp4PeerRoutes.SetNextHopIpv4Address(dstIpv4.Address()).
		SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).
		SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)
	srcBgp4PeerRoutes.Addresses().Add().SetAddress(advertisedRoutesv4Net2).SetPrefix(advertisedRoutesv4Prefix).SetCount(routeCount)
	srcBgp6PeerRoutes := dstBgp6Peer.V6Routes().Add().SetName("bgpNeti1v6")
	srcBgp6PeerRoutes.SetNextHopIpv6Address(dstIpv6.Address()).
		SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).
		SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL)
	srcBgp6PeerRoutes.Addresses().Add().SetAddress(advertisedRoutesv6Net2).SetPrefix(advertisedRoutesv6Prefix).SetCount(routeCount)

	dstBgp4PeerRoutes := dstBgp4Peer.V4Routes().Add().SetName("bgpNeti2")
	dstBgp4PeerRoutes.SetNextHopIpv4Address(dstIpv4.Address()).
		SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).
		SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)
	dstBgp4PeerRoutes.Addresses().Add().SetAddress(advertisedRoutesv4Net).SetPrefix(advertisedRoutesv4Prefix).SetCount(routeCount)
	dstBgp6PeerRoutes := dstBgp6Peer.V6Routes().Add().SetName("bgpNeti2v6")
	dstBgp6PeerRoutes.SetNextHopIpv6Address(dstIpv6.Address()).
		SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).
		SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL)
	dstBgp6PeerRoutes.Addresses().Add().SetAddress(advertisedRoutesv6Net).SetPrefix(advertisedRoutesv6Prefix).SetCount(routeCount)

	// ATE Traffic Configuration
	t.Logf("start ate Traffic config")
	flowipv4 := config.Flows().Add().SetName("Ipv4")
	flowipv4.Metrics().SetEnable(true)
	flowipv4.TxRx().Device().
		SetTxNames([]string{srcIpv4.Name()}).
		SetRxNames([]string{dstIpv4.Name()})
	flowipv4.Size().SetFixed(512)
	flowipv4.Rate().SetPps(100)
	flowipv4.Duration().SetChoice("continuous")
	e1 := flowipv4.Packet().Add().Ethernet()
	e1.Src().SetValue(srcEth.Mac())
	v4 := flowipv4.Packet().Add().Ipv4()
	v4.Src().SetValue(ipv4SrcTraffic)
	v4.Dst().Increment().SetStart(ipv4DstTrafficStart).SetCount(routeCount)

	ate.OTG().PushConfig(t, config)
	ate.OTG().StartProtocols(t)
	t.Log("Pushing config to ATE and starting protocols...")
	return config
}

func verifyNoPacketLoss(t *testing.T, ate *ondatra.ATEDevice, c gosnappi.Config) {
	otg := ate.OTG()
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
			t.Errorf("Traffic Loss Pct for Flow %s: got %v", f.Name(), lossPct)
		}
	}
}

func confirmPacketLoss(t *testing.T, ate *ondatra.ATEDevice, c gosnappi.Config) {
	otg := ate.OTG()
	otgutils.LogFlowMetrics(t, otg, c)
	for _, f := range c.Flows().Items() {
		t.Logf("Verifying flow metrics for flow %s\n", f.Name())
		recvMetric := gnmi.Get(t, otg, gnmi.OTG().Flow(f.Name()).State())
		txPackets := recvMetric.GetCounters().GetOutPkts()
		rxPackets := recvMetric.GetCounters().GetInPkts()
		lostPackets := txPackets - rxPackets
		if txPackets == 0 {
			t.Fatalf("Tx packets should be higher than 0 for flow %s", f.Name())
		}
		if lossPct := lostPackets * 100 / txPackets; lossPct > 99.0 {
			t.Logf("Traffic Test Passed! Loss seen as expected: got %v, want 100%% ", lossPct)
		} else {
			t.Errorf("Traffic %s is expected to fail: got %v, want 100%% failure", f.Name(), lossPct)
		}
	}
}

func sendTraffic(t *testing.T, ate *ondatra.ATEDevice, c gosnappi.Config) {
	otg := ate.OTG()
	t.Logf("Starting traffic")
	otg.StartTraffic(t)
	time.Sleep(trafficDuration)
	t.Logf("Stop traffic")
	otg.StopTraffic(t)
}

func configACL(d *oc.Root, name string) *oc.Acl_AclSet {
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

func configAdmitAllACL(d *oc.Root, name string) *oc.Acl_AclSet {
	acl := d.GetOrCreateAcl().GetOrCreateAclSet(aclName, oc.Acl_ACL_TYPE_ACL_IPV4)
	acl.DeleteAclEntry(10)
	acl.DeleteAclEntry(20)
	return acl
}

func configACLInterface(t *testing.T, iFace *oc.Acl_Interface, ifName string) *acl.Acl_InterfacePath {
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

// Helper function to replicate configACL() configs in native model
// Define the values for each ACL entry and marshal for json encoding.
// Then craft a gNMI set Request to update the changes.
func configACLNative(t testing.TB, d *ondatra.DUTDevice, name string) {
	t.Helper()
	switch d.Vendor() {
	case ondatra.NOKIA:
		var aclEntry10Val = []any{
			map[string]any{
				"action": map[string]any{
					"drop": map[string]any{},
				},
				"match": map[string]any{
					"destination-ip": map[string]any{
						"prefix": ateDstCIDR,
					},
					"source-ip": map[string]any{
						"prefix": aclNullPrefix,
					},
				},
			},
		}
		entry10Update, err := json.Marshal(aclEntry10Val)
		if err != nil {
			t.Fatalf("Error with json Marshal: %v", err)
		}

		var aclEntry20Val = []any{
			map[string]any{
				"action": map[string]any{
					"drop": map[string]any{},
				},
				"match": map[string]any{
					"source-ip": map[string]any{
						"prefix": ateDstCIDR,
					},
					"destination-ip": map[string]any{
						"prefix": aclNullPrefix,
					},
				},
			},
		}
		entry20Update, err := json.Marshal(aclEntry20Val)
		if err != nil {
			t.Fatalf("Error with json Marshal: %v", err)
		}

		var aclEntry30Val = []any{
			map[string]any{
				"action": map[string]any{
					"accept": map[string]any{},
				},
				"match": map[string]any{
					"source-ip": map[string]any{
						"prefix": aclNullPrefix,
					},
					"destination-ip": map[string]any{
						"prefix": aclNullPrefix,
					},
				},
			},
		}
		entry30Update, err := json.Marshal(aclEntry30Val)
		if err != nil {
			t.Fatalf("Error with json Marshal: %v", err)
		}
		gpbSetRequest := &gpb.SetRequest{
			Prefix: &gpb.Path{
				Origin: "srl",
			},
			Update: []*gpb.Update{
				{
					Path: &gpb.Path{
						Elem: []*gpb.PathElem{
							{Name: "acl"},
							{Name: "ipv4-filter", Key: map[string]string{"name": name}},
							{Name: "entry", Key: map[string]string{"sequence-id": "10"}},
						},
					},
					Val: &gpb.TypedValue{
						Value: &gpb.TypedValue_JsonIetfVal{
							JsonIetfVal: entry10Update,
						},
					},
				},
				{
					Path: &gpb.Path{
						Elem: []*gpb.PathElem{
							{Name: "acl"},
							{Name: "ipv4-filter", Key: map[string]string{"name": name}},
							{Name: "entry", Key: map[string]string{"sequence-id": "20"}},
						},
					},
					Val: &gpb.TypedValue{
						Value: &gpb.TypedValue_JsonIetfVal{
							JsonIetfVal: entry20Update,
						},
					},
				},
				{
					Path: &gpb.Path{
						Elem: []*gpb.PathElem{
							{Name: "acl"},
							{Name: "ipv4-filter", Key: map[string]string{"name": name}},
							{Name: "entry", Key: map[string]string{"sequence-id": "30"}},
						},
					},
					Val: &gpb.TypedValue{
						Value: &gpb.TypedValue_JsonIetfVal{
							JsonIetfVal: entry30Update,
						},
					},
				},
			},
		}
		gnmiClient := d.RawAPIs().GNMI().Default(t)
		if _, err := gnmiClient.Set(context.Background(), gpbSetRequest); err != nil {
			t.Fatalf("Unexpected error configuring SRL ACL: %v", err)
		}
	default:
		t.Fatalf("Unsupported vendor %s for deviation 'UseVendorNativeACLConfiguration'", d.Vendor())
	}
}

// Helper function to replicate AdmitAllACL() configs in native model,
// then craft a gNMI set Request to update the changes.
func configAdmitAllACLNative(t testing.TB, d *ondatra.DUTDevice, name string) {
	t.Helper()
	switch d.Vendor() {
	case ondatra.NOKIA:
		gpbDelRequest := &gpb.SetRequest{
			Prefix: &gpb.Path{
				Origin: "srl",
			},
			Delete: []*gpb.Path{
				{
					Elem: []*gpb.PathElem{
						{Name: "acl"},
						{Name: "ipv4-filter", Key: map[string]string{"name": name}},
						{Name: "entry", Key: map[string]string{"sequence-id": "10"}},
					},
				},
				{
					Elem: []*gpb.PathElem{
						{Name: "acl"},
						{Name: "ipv4-filter", Key: map[string]string{"name": name}},
						{Name: "entry", Key: map[string]string{"sequence-id": "20"}},
					},
				},
			},
		}
		gnmiClient := d.RawAPIs().GNMI().Default(t)
		if _, err := gnmiClient.Set(context.Background(), gpbDelRequest); err != nil {
			t.Fatalf("Unexpected error removing SRL ACL: %v", err)
		}
	default:
		t.Fatalf("Unsupported vendor %s for deviation 'UseVendorNativeACLConfiguration'", d.Vendor())
	}
}

// Helper function to replicate configACLInterface in native model.
// Set ACL at interface ingress,
// then craft a gNMI set Request to update the changes.
func configACLInterfaceNative(t *testing.T, d *ondatra.DUTDevice, ifName string) {
	t.Helper()
	switch d.Vendor() {
	case ondatra.NOKIA:
		var interfaceAclVal = []any{
			map[string]any{
				"ipv4-filter": []any{
					aclName,
				},
			},
		}
		interfaceAclUpdate, err := json.Marshal(interfaceAclVal)
		if err != nil {
			t.Fatalf("Error with json Marshal: %v", err)
		}
		gpbSetRequest := &gpb.SetRequest{
			Prefix: &gpb.Path{
				Origin: "srl",
			},
			Update: []*gpb.Update{
				{
					Path: &gpb.Path{
						Elem: []*gpb.PathElem{
							{Name: "interface", Key: map[string]string{"name": ifName}},
							{Name: "subinterface", Key: map[string]string{"index": "0"}},
							{Name: "acl"},
							{Name: "input"},
						},
					},
					Val: &gpb.TypedValue{
						Value: &gpb.TypedValue_JsonIetfVal{
							JsonIetfVal: interfaceAclUpdate,
						},
					},
				},
			},
		}
		gnmiClient := d.RawAPIs().GNMI().Default(t)
		if _, err := gnmiClient.Set(context.Background(), gpbSetRequest); err != nil {
			t.Fatalf("Unexpected error configuring interface ACL: %v", err)
		}
	default:
		t.Fatalf("Unsupported vendor %s for deviation 'UseVendorNativeACLConfiguration'", d.Vendor())
	}
}

func TestTrafficWithGracefulRestartSpeaker(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	// Configure interface on the DUT
	t.Run("configureDut", func(t *testing.T) {
		t.Log("Start DUT interface Config")
		configureDUT(t, dut)
		if *deviations.RoutePolicyUnderPeerGroup {
			configureRoutePolicy(t, dut, "ALLOW", oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)
		}
	})

	// Configure BGP+Neighbors on the DUT
	t.Run("configureBGP", func(t *testing.T) {
		t.Log("Configure BGP with Graceful Restart option under Global Bgp")
		dutConfPath := gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
		nbrList := buildNbrList(ateAS)
		dutConf := bgpWithNbr(dutAS, nbrList)
		gnmi.Replace(t, dut, dutConfPath.Config(), dutConf)
		fptest.LogQuery(t, "DUT BGP Config", dutConfPath.Config(), gnmi.GetConfig(t, dut, dutConfPath.Config()))
	})
	// ATE Configuration.
	t.Log("Start ATE Config")
	config := configureATE(t, ate)

	// Verify Port Status
	t.Run("verifyDUTPorts", func(t *testing.T) {
		t.Log("Verifying port status")
		verifyPortsUp(t, dut.Device)
	})
	t.Run("VerifyBGPParameters", func(t *testing.T) {
		t.Log("Check BGP parameters")
		checkBgpStatus(t, dut)
	})
	// Starting ATE Traffic
	t.Run("VerifyTrafficPassBeforeAcLBlock", func(t *testing.T) {
		t.Log("Send Traffic with GR timer enabled. Traffic should pass")
		sendTraffic(t, ate, config)
		verifyNoPacketLoss(t, ate, config)
	})
	// Configure an ACL to block BGP
	d := &oc.Root{}
	ifName := dut.Port(t, "port2").Name()
	iFace := d.GetOrCreateAcl().GetOrCreateInterface(ifName)
	t.Run("VerifyTrafficPasswithGRTimerWithAclApplied", func(t *testing.T) {
		t.Log("Configure Acl to block BGP on port 179")
		const stopDuration = 45 * time.Second
		gnmi.Replace(t, dut, gnmi.OC().Acl().AclSet(aclName, oc.Acl_ACL_TYPE_ACL_IPV4).Config(), configACL(d, aclName))
		aclConf := configACLInterface(t, iFace, ifName)
		t.Log("Starting traffic")
		ate.OTG().StartTraffic(t)
		startTime := time.Now()
		gnmi.Replace(t, dut, aclConf.Config(), iFace)
		replaceDuration := time.Since(startTime)
		time.Sleep(grTimer - stopDuration - replaceDuration)
		t.Log("Send Traffic while GR timer counting down. Traffic should pass as BGP GR is enabled!")
		ate.OTG().StopTraffic(t)
		t.Log("Traffic stopped")
		verifyNoPacketLoss(t, ate, config)
	})

	statePath := gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	nbrPath := statePath.Neighbor(ateDst.IPv4)
	t.Run("VerifyBGPNOTEstablished", func(t *testing.T) {
		t.Log("Waiting for BGP neighbor to Not be in Established state after applying ACL DENY policy..")
		_, ok := gnmi.Watch(t, dut, nbrPath.SessionState().State(), 2*time.Minute, func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
			currState, ok := val.Val()
			return ok && currState != oc.Bgp_Neighbor_SessionState_ESTABLISHED
		}).Await(t)
		if !ok {
			fptest.LogQuery(t, "BGP reported state", nbrPath.State(), gnmi.Get(t, dut, nbrPath.State()))
			t.Errorf("BGP session did not go Down as expected")
		}
	})

	t.Log("Wait till LLGR/Stale timer expires to delete long live routes.....")
	time.Sleep(time.Second * grRestartTime)
	time.Sleep(time.Second * grStaleRouteTime)

	t.Run("VerifyTrafficFailureAfterGRexpired", func(t *testing.T) {
		t.Log("Send Traffic Again after GR timer has expired. This traffic should fail!")
		sendTraffic(t, ate, config)
		confirmPacketLoss(t, ate, config)
	})

	t.Run("RemoveAclInterface", func(t *testing.T) {
		t.Log("Removing Acl on the interface to restore BGP GR. Traffic should now pass!")
		if deviations.UseVendorNativeACLConfig(dut) {
			configAdmitAllACLNative(t, dut, aclName)
			configACLInterfaceNative(t, dut, ifName)
		} else {
			gnmi.Replace(t, dut, gnmi.OC().Acl().AclSet(aclName, oc.Acl_ACL_TYPE_ACL_IPV4).Config(), configAdmitAllACL(d, aclName))
			aclPath := configACLInterface(t, iFace, ifName)
			gnmi.Replace(t, dut, aclPath.Config(), iFace)
		}
	})

	t.Run("VerifyBGPEstablished", func(t *testing.T) {
		t.Logf("Waiting for BGP neighbor to establish...")
		_, ok := gnmi.Watch(t, dut, nbrPath.SessionState().State(), 2*time.Minute, func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
			currState, ok := val.Val()
			return ok && currState == oc.Bgp_Neighbor_SessionState_ESTABLISHED
		}).Await(t)
		if !ok {
			fptest.LogQuery(t, "BGP reported state", nbrPath.State(), gnmi.Get(t, dut, nbrPath.State()))
			t.Errorf("BGP session not Established as expected")
		}
	})

	t.Run("VerifyTrafficPassBGPRestored", func(t *testing.T) {
		status := gnmi.Get(t, dut, nbrPath.SessionState().State())
		if want := oc.Bgp_Neighbor_SessionState_ESTABLISHED; status != want {
			t.Errorf("Get(BGP peer %s status): got %d, want %d", ateDst.IPv4, status, want)
		}
		sendTraffic(t, ate, config)
		verifyNoPacketLoss(t, ate, config)
	})

}
