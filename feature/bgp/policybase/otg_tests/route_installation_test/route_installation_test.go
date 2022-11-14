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

package route_installation_test

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
	otgtelemetry "github.com/openconfig/ondatra/gnmi/otg"
	otg "github.com/openconfig/ondatra/otg"
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
// Source: ate:port1 -> dut:port1 subnet 192.0.2.0/30 2001:db8::0/126
// Destination: dut:port2 -> ate:port2 subnet 192.0.2.4/30 2001:db8::4/126
//
// Note that the first (.0, .3) and last (.4, .7) IPv4 addresses are
// reserved from the subnet for broadcast, so a /30 leaves exactly 2
// usable addresses. This does not apply to IPv6 which allows /127
// for point to point links, but we use /126 so the numbering is
// consistent with IPv4.
//
// A traffic flow is configured from ate:port1 as the source interface
// and ate:port2 as the destination interface. Then 255 BGP routes 203.0.113.[1-254]/32
// are adverstised from port2 and traffic is sent originating from port1 to all
// these advertised routes. The traffic will pass only if the DUT installs the
// prefixes successfully in the routing table via BGP.Successful transmission of
// traffic will ensure BGP routes are properly installed on the DUT and programmed.
// Similarly, Traffic is sent for IPv6 destinations.

const (
	trafficDuration          = 1 * time.Minute
	ipv4SrcTraffic           = "192.0.2.2"
	ipv6SrcTraffic           = "2001:db8::192:0:2:2"
	ipv4DstTrafficStart      = "203.0.113.1"
	ipv4DstTrafficEnd        = "203.0.113.254"
	ipv6DstTrafficStart      = "2001:db8::203:0:113:1"
	ipv6DstTrafficEnd        = "2001:db8::203:0:113:fe"
	advertisedRoutesv4Net    = "203.0.113.1"
	advertisedRoutesv6Net    = "2001:db8::203:0:113:1"
	advertisedRoutesv4Prefix = 32
	advertisedRoutesv6Prefix = 128
	peerGrpName              = "BGP-PEER-GROUP"
	routeCount               = 254
	dutAS                    = 64500
	ateAS                    = 64501
	badAS                    = 64502
	plenIPv4                 = 30
	plenIPv6                 = 126
	tolerance                = 50
	tolerancePct             = 2
	ipPrefixSet              = "203.0.113.0/29"
	prefixSubnetRange        = "29..32"
	allowConnected           = "ALLOW-CONNECTED"
	prefixSet                = "PREFIX-SET"
	defaultPolicy            = ""
	denyPolicy               = "DENY-ALL"
	acceptPolicy             = "PERMIT-ALL"
	setLocalPrefPolicy       = "SET-LOCAL-PREF"
	localPrefValue           = 100
	setAspathPrependPolicy   = "SET-ASPATH-PREPEND"
	asPathRepeatValue        = 3
	aclStatement1            = "10"
	aclStatement2            = "20"
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

// configureDUT configures all the interfaces on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	dc := gnmi.OC()
	i1 := dutSrc.NewOCInterface(dut.Port(t, "port1").Name())
	gnmi.Replace(t, dut, dc.Interface(i1.GetName()).Config(), i1)

	i2 := dutDst.NewOCInterface(dut.Port(t, "port2").Name())
	gnmi.Replace(t, dut, dc.Interface(i2.GetName()).Config(), i2)
}

// verifyPortsUp asserts that each port on the device is operating
func verifyPortsUp(t *testing.T, dev *ondatra.Device) {
	t.Helper()
	for _, p := range dev.Ports() {
		status := gnmi.Get(t, dev, gnmi.OC().Interface(p.Name()).OperStatus().State())
		if want := oc.Interface_OperStatus_UP; status != want {
			t.Errorf("%s Status: got %v, want %v", p, status, want)
		}
	}
}

// bgpCreateNbr creates a BGP object with neighbors pointing to ateSrc and ateDst, optionally with
// a peer group policy.
func bgpCreateNbr(localAs, peerAs uint32, policy string) *oc.NetworkInstance_Protocol_Bgp {
	nbr1v4 := &bgpNeighbor{as: peerAs, neighborip: ateSrc.IPv4, isV4: true}
	nbr1v6 := &bgpNeighbor{as: peerAs, neighborip: ateSrc.IPv6, isV4: false}
	nbr2v4 := &bgpNeighbor{as: peerAs, neighborip: ateDst.IPv4, isV4: true}
	nbr2v6 := &bgpNeighbor{as: peerAs, neighborip: ateDst.IPv6, isV4: false}
	nbrs := []*bgpNeighbor{nbr1v4, nbr2v4, nbr1v6, nbr2v6}

	d := &oc.Root{}
	ni1 := d.GetOrCreateNetworkInstance(*deviations.DefaultNetworkInstance)
	bgp := ni1.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").GetOrCreateBgp()
	global := bgp.GetOrCreateGlobal()
	global.RouterId = ygot.String(dutDst.IPv4)
	global.As = ygot.Uint32(localAs)
	// Note: we have to define the peer group even if we aren't setting any policy because it's
	// invalid OC for the neighbor to be part of a peer group that doesn't exist.
	pg := bgp.GetOrCreatePeerGroup(peerGrpName)
	pg.PeerAs = ygot.Uint32(ateAS)
	pg.PeerGroupName = ygot.String(peerGrpName)
	if policy != "" {
		pg.GetOrCreateApplyPolicy().ImportPolicy = []string{policy}
	}
	for _, nbr := range nbrs {
		if nbr.isV4 {
			nv4 := bgp.GetOrCreateNeighbor(nbr.neighborip)
			nv4.PeerGroup = ygot.String(peerGrpName)
			nv4.PeerAs = ygot.Uint32(nbr.as)
			nv4.Enabled = ygot.Bool(true)
			nv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
		} else {
			nv6 := bgp.GetOrCreateNeighbor(nbr.neighborip)
			nv6.PeerGroup = ygot.String(peerGrpName)
			nv6.PeerAs = ygot.Uint32(nbr.as)
			nv6.Enabled = ygot.Bool(true)
			nv6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)
		}

	}
	return bgp
}

// configureBGPPolicy configures a BGP routing policy to accept or reject routes based on prefix match conditions
// Additonally, it configures LocalPreference and ASPathprepend as part of the BGP policy
func configureBGPPolicy(d *oc.Root) *oc.RoutingPolicy {
	rp := d.GetOrCreateRoutingPolicy()
	pset := rp.GetOrCreateDefinedSets().GetOrCreatePrefixSet(prefixSet)
	pset.GetOrCreatePrefix(ipPrefixSet, prefixSubnetRange)
	pdef := rp.GetOrCreatePolicyDefinition(allowConnected)
	stmt5 := pdef.GetOrCreateStatement(aclStatement1)
	stmt5.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_REJECT_ROUTE
	stmt5.GetOrCreateConditions().GetOrCreateMatchPrefixSet().PrefixSet = ygot.String(prefixSet)
	stmt10 := pdef.GetOrCreateStatement(aclStatement2)
	stmt10.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE

	pdef2 := rp.GetOrCreatePolicyDefinition(acceptPolicy)
	pdef2.GetOrCreateStatement(aclStatement2).GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE

	pdef3 := rp.GetOrCreatePolicyDefinition(denyPolicy)
	pdef3.GetOrCreateStatement(aclStatement2).GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_REJECT_ROUTE

	pdef4 := rp.GetOrCreatePolicyDefinition(setLocalPrefPolicy)
	actions := pdef4.GetOrCreateStatement(aclStatement2).GetOrCreateActions()
	actions.GetOrCreateBgpActions().SetLocalPref = ygot.Uint32(localPrefValue)
	actions.PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE

	pdef5 := rp.GetOrCreatePolicyDefinition(setAspathPrependPolicy)
	actions5 := pdef5.GetOrCreateStatement(aclStatement2).GetOrCreateActions()
	aspend := actions5.GetOrCreateBgpActions().GetOrCreateSetAsPathPrepend()
	aspend.Asn = ygot.Uint32(ateAS)
	aspend.RepeatN = ygot.Uint8(asPathRepeatValue)
	actions5.PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE

	return rp
}

// verifyBgpTelemetry checks that the dut has an established BGP session with reasonable settings
func verifyBgpTelemetry(t *testing.T, dut *ondatra.DUTDevice) {
	ifName := dut.Port(t, "port1").Name()
	lastFlapTime := gnmi.Get(t, dut, gnmi.OC().Interface(ifName).LastChange().State())
	t.Logf("Verifying BGP state")
	statePath := gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	nbrPath := statePath.Neighbor(ateSrc.IPv4)
	nbrPathv6 := statePath.Neighbor(ateSrc.IPv6)
	nbr := gnmi.Get(t, dut, statePath.State()).GetNeighbor(ateSrc.IPv4)

	// Get BGP adjacency state
	t.Logf("Waiting for BGP neighbor to establish...")
	_, ok := gnmi.Watch(t, dut, nbrPath.SessionState().State(), time.Minute, func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
		state, ok := val.Val()
		return ok && state == oc.Bgp_Neighbor_SessionState_ESTABLISHED
	}).Await(t)
	if !ok {
		fptest.LogQuery(t, "BGP reported state", nbrPath.State(), gnmi.Get(t, dut, nbrPath.State()))
		t.Fatal("No BGP neighbor formed")
	}
	status := gnmi.Get(t, dut, nbrPath.SessionState().State())
	t.Logf("BGP adjacency for %s: %s", ateSrc.IPv4, status)
	if want := oc.Bgp_Neighbor_SessionState_ESTABLISHED; status != want {
		t.Errorf("BGP peer %s status got %d, want %d", ateSrc.IPv4, status, want)
	}

	// Check last established timestamp
	lestTime := gnmi.Get(t, dut, nbrPath.State()).GetLastEstablished()
	lestTimev6 := gnmi.Get(t, dut, nbrPathv6.State()).GetLastEstablished()
	t.Logf("BGP last est time :%v, flapTime :%v", lestTime, lastFlapTime)
	t.Logf("BGP v6 last est time :%d", lestTimev6)
	if lestTime < lastFlapTime {
		t.Errorf("Bad last-established timestamp: got %v, want >= %v", lestTime, lastFlapTime)
	}

	// Check BGP Transitions
	estTrans := nbr.GetEstablishedTransitions()
	t.Logf("Got established transitions: %d", estTrans)
	if estTrans != 1 {
		t.Errorf("Wrong established-transitions: got %v, want 1", estTrans)
	}

	// Check BGP neighbor address from telemetry
	addrv4 := gnmi.Get(t, dut, nbrPath.State()).GetNeighborAddress()
	addrv6 := gnmi.Get(t, dut, nbrPathv6.State()).GetNeighborAddress()
	t.Logf("Got ipv4 neighbor address: %s", addrv4)
	if addrv4 != ateSrc.IPv4 {
		t.Errorf("BGP v4 neighbor address: got %v, want %v", addrv4, ateSrc.IPv4)
	}

	t.Logf("Got Ipv6 neighbor address: %s", addrv6)
	if addrv6 != ateSrc.IPv6 {
		t.Errorf("BGP v6 neighbor address: got %v, want %v", addrv6, ateSrc.IPv6)
	}

	// Check BGP neighbor address from telemetry
	peerAS := gnmi.Get(t, dut, nbrPath.State()).GetPeerAs()
	if peerAS != ateAS {
		t.Errorf("BGP peerAs: got %v, want %v", peerAS, ateAS)
	}

	// Check BGP neighbor is enabled
	if !gnmi.Get(t, dut, nbrPath.State()).GetEnabled() {
		t.Errorf("Expected neighbor %v to be enabled", ateSrc.IPv4)
	}
}

// verifyPrefixesTelemetry confirms that the dut shows the correct numbers of installed, sent and
// received IPv4 prefixes
// TODO: Need to refactor and compare using cmp.diff
func verifyPrefixesTelemetry(t *testing.T, dut *ondatra.DUTDevice, wantInstalled, wantRx, wantSent uint32) {
	statePath := gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	prefixesv4 := statePath.Neighbor(ateDst.IPv4).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Prefixes()
	if gotInstalled := gnmi.Get(t, dut, prefixesv4.Installed().State()); gotInstalled != wantInstalled {
		t.Errorf("Installed prefixes mismatch: got %v, want %v", gotInstalled, wantInstalled)
	}
	if gotRx := gnmi.Get(t, dut, prefixesv4.ReceivedPrePolicy().State()); gotRx != wantRx {
		t.Errorf("Received prefixes mismatch: got %v, want %v", gotRx, wantRx)
	}
	if gotSent := gnmi.Get(t, dut, prefixesv4.Sent().State()); gotSent != wantSent {
		t.Errorf("Sent prefixes mismatch: got %v, want %v", gotSent, wantSent)
	}
}

// verifyPrefixesTelemetryV6 confirms that the dut shows the correct numbers of installed, sent and
// received IPv6 prefixes
func verifyPrefixesTelemetryV6(t *testing.T, dut *ondatra.DUTDevice, wantInstalledv6, wantRxv6, wantSentv6 uint32) {
	statePath := gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	prefixesv6 := statePath.Neighbor(ateDst.IPv6).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Prefixes()

	if gotInstalledv6 := gnmi.Get(t, dut, prefixesv6.Installed().State()); gotInstalledv6 != wantInstalledv6 {
		t.Errorf("IPV6 Installed prefixes mismatch: got %v, want %v", gotInstalledv6, wantInstalledv6)
	}
	if gotRxv6 := gnmi.Get(t, dut, prefixesv6.ReceivedPrePolicy().State()); gotRxv6 != wantRxv6 {
		t.Errorf("IPV6 Received prefixes mismatch: got %v, want %v", gotRxv6, wantRxv6)
	}
	if gotSentv6 := gnmi.Get(t, dut, prefixesv6.Sent().State()); gotSentv6 != wantSentv6 {
		t.Errorf("IPv6 Sent prefixes mismatch: got %v, want %v", gotSentv6, wantSentv6)
	}
}

// verifyPolicyTelemetry confirms that the dut policy is set as expected.
func verifyPolicyTelemetry(t *testing.T, dut *ondatra.DUTDevice, policy string) {
	statePath := gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	policytel := gnmi.Get(t, dut, statePath.PeerGroup(peerGrpName).ApplyPolicy().ImportPolicy().State())
	for _, val := range policytel {
		if val != policy {
			t.Errorf("Apply policy mismatch: got %v, want %v", policytel, policy)
		}
	}
}

// configureATE configures the interfaces and BGP protocols on an OTG, including advertising some
// (faked) networks over BGP.
func configureATE(t *testing.T, otg *otg.OTG) gosnappi.Config {

	config := otg.NewConfig(t)
	srcPort := config.Ports().Add().SetName("port1")
	dstPort := config.Ports().Add().SetName("port2")

	srcDev := config.Devices().Add().SetName(ateSrc.Name)
	srcEth := srcDev.Ethernets().Add().SetName(ateSrc.Name + ".Eth")
	srcEth.SetPortName(srcPort.Name()).SetMac(ateSrc.MAC)
	srcIpv4 := srcEth.Ipv4Addresses().Add().SetName(ateSrc.Name + ".IPv4")
	srcIpv4.SetAddress(ateSrc.IPv4).SetGateway(dutSrc.IPv4).SetPrefix(int32(ateSrc.IPv4Len))
	srcIpv6 := srcEth.Ipv6Addresses().Add().SetName(ateSrc.Name + ".IPv6")
	srcIpv6.SetAddress(ateSrc.IPv6).SetGateway(dutSrc.IPv6).SetPrefix(int32(ateSrc.IPv6Len))

	dstDev := config.Devices().Add().SetName(ateDst.Name)
	dstEth := dstDev.Ethernets().Add().SetName(ateDst.Name + ".Eth")
	dstEth.SetPortName(dstPort.Name()).SetMac(ateDst.MAC)
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

	dstBgp4PeerRoutes := dstBgp4Peer.V4Routes().Add().SetName(ateDst.Name + ".BGP4.peer" + ".RR4")
	dstBgp4PeerRoutes.SetNextHopIpv4Address(dstIpv4.Address()).
		SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).
		SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)
	dstBgp4PeerRoutes.Addresses().Add().
		SetAddress(advertisedRoutesv4Net).
		SetPrefix(advertisedRoutesv4Prefix).
		SetCount(routeCount)
	dstBgp6PeerRoutes := dstBgp6Peer.V6Routes().Add().SetName(ateDst.Name + ".BGP6.peer" + ".rr6")
	dstBgp6PeerRoutes.SetNextHopIpv6Address(dstIpv6.Address()).
		SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).
		SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL)
	dstBgp6PeerRoutes.Addresses().Add().
		SetAddress(advertisedRoutesv6Net).
		SetPrefix(advertisedRoutesv6Prefix).
		SetCount(routeCount)

	// ATE Traffic Configuration
	t.Logf("TestBGP:start ate Traffic config")
	flowipv4 := config.Flows().Add().SetName("bgpv4RoutesFlow")
	flowipv4.Metrics().SetEnable(true)
	flowipv4.TxRx().Device().
		SetTxNames([]string{srcIpv4.Name()}).
		SetRxNames([]string{dstBgp4PeerRoutes.Name()})
	flowipv4.Size().SetFixed(512)
	flowipv4.Rate().SetPps(100)
	flowipv4.Duration().SetChoice("continuous")
	e1 := flowipv4.Packet().Add().Ethernet()
	e1.Src().SetValue(srcEth.Mac())
	v4 := flowipv4.Packet().Add().Ipv4()
	v4.Src().SetValue(srcIpv4.Address())
	v4.Dst().Increment().SetStart(advertisedRoutesv4Net).SetCount(routeCount)

	flowipv6 := config.Flows().Add().SetName("bgpv6RoutesFlow")
	flowipv6.Metrics().SetEnable(true)
	flowipv6.TxRx().Device().
		SetTxNames([]string{srcIpv6.Name()}).
		SetRxNames([]string{dstBgp6PeerRoutes.Name()})
	flowipv6.Size().SetFixed(512)
	flowipv6.Rate().SetPps(100)
	flowipv6.Duration().SetChoice("continuous")
	e2 := flowipv6.Packet().Add().Ethernet()
	e2.Src().SetValue(srcEth.Mac())
	v6 := flowipv6.Packet().Add().Ipv6()
	v6.Src().SetValue(srcIpv6.Address())
	v6.Dst().Increment().SetStart(advertisedRoutesv6Net).SetCount(routeCount)

	t.Logf("Pushing config to ATE and starting protocols...")
	otg.PushConfig(t, config)
	otg.StartProtocols(t)
	return config
}

// verifyTraffic confirms that every traffic flow has the expected amount of loss (0% or 100%
// depending on wantLoss, +- 2%)
func verifyTraffic(t *testing.T, ate *ondatra.ATEDevice, c gosnappi.Config, wantLoss bool) {
	otg := ate.OTG()
	otgutils.LogFlowMetrics(t, otg, c)
	for _, f := range c.Flows().Items() {
		t.Logf("Verifying flow metrics for flow %s\n", f.Name())
		recvMetric := gnmi.Get(t, otg, gnmi.OTG().Flow(f.Name()).State())
		txPackets := recvMetric.GetCounters().GetOutPkts()
		rxPackets := recvMetric.GetCounters().GetInPkts()
		lostPackets := txPackets - rxPackets
		lossPct := lostPackets * 100 / txPackets
		if !wantLoss {
			if lostPackets > tolerance {
				t.Logf("Packets received not matching packets sent. Sent: %v, Received: %d", txPackets, rxPackets)
			}
			if lossPct > tolerancePct && txPackets > 0 {
				t.Errorf("Traffic Loss Pct for Flow: %s\n got %v, want max %v pct failure", f.Name(), lossPct, tolerancePct)
			} else {
				t.Logf("Traffic Test Passed! for flow %s", f.Name())
			}
		} else {
			if lossPct < 100-tolerancePct && txPackets > 0 {
				t.Errorf("Traffic is expected to fail %s\n got %v, want max %v pct failure", f.Name(), lossPct, 100-tolerancePct)
			} else {
				t.Logf("Traffic Loss Test Passed! for flow %s", f.Name())
			}
		}

	}
}

func sendTraffic(t *testing.T, otg *otg.OTG, c gosnappi.Config) {
	t.Logf("Starting traffic")
	otg.StartTraffic(t)
	time.Sleep(trafficDuration)
	t.Logf("Stop traffic")
	otg.StopTraffic(t)
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

type bgpNeighbor struct {
	as         uint32
	neighborip string
	isV4       bool
}

// TestEstablish sets up a basic BGP connection and confirms that traffic is forwarded according to
// it.
func TestEstablish(t *testing.T) {
	// DUT configurations.
	t.Logf("Start DUT config load:")
	dut := ondatra.DUT(t, "dut")

	// Configure interface on the DUT
	t.Logf("Start DUT interface Config")
	configureDUT(t, dut)

	// Configure BGP+Neighbors on the DUT
	t.Logf("Start DUT BGP Config")
	dutConfPath := gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	gnmi.Delete(t, dut, dutConfPath.Config())
	dutConf := bgpCreateNbr(dutAS, ateAS, defaultPolicy)
	gnmi.Replace(t, dut, dutConfPath.Config(), dutConf)
	fptest.LogQuery(t, "DUT BGP Config", dutConfPath.Config(), gnmi.GetConfig(t, dut, dutConfPath.Config()))

	// ATE Configuration.
	t.Logf("Start ATE Config")
	ate := ondatra.ATE(t, "ate")

	otg := ate.OTG()
	otgConfig := configureATE(t, otg)
	// Verify Port Status
	t.Logf("Verifying port status")
	verifyPortsUp(t, dut.Device)

	// Verify the OTG BGP state
	t.Logf("Verify OTG BGP sessions up")
	verifyOTGBGPTelemetry(t, otg, otgConfig, "ESTABLISHED")

	t.Logf("Check BGP parameters")
	verifyBgpTelemetry(t, dut)

	// Starting ATE Traffic and verify Traffic Flows and packet loss
	sendTraffic(t, otg, otgConfig)
	verifyTraffic(t, ate, otgConfig, false)
	verifyPrefixesTelemetry(t, dut, routeCount, routeCount, 0)
	verifyPrefixesTelemetryV6(t, dut, routeCount, routeCount, 0)

	t.Run("RoutesWithdrawn", func(t *testing.T) {
		t.Log("Breaking BGP config and confirming that forwarding stops working.")
		// Break config with a mismatching AS number
		gnmi.Replace(t, dut, dutConfPath.Config(), bgpCreateNbr(dutAS, badAS, defaultPolicy))

		// Verify the OTG BGP state
		t.Logf("Verify OTG BGP sessions down")
		verifyOTGBGPTelemetry(t, otg, otgConfig, "IDLE")

		// Resend traffic
		sendTraffic(t, otg, otgConfig)
		verifyTraffic(t, ate, otgConfig, true)
	})
}

func TestBGPPolicy(t *testing.T) {
	// DUT configurations.
	t.Logf("Start DUT config load:")
	dut := ondatra.DUT(t, "dut")

	// Configure interface on the DUT
	t.Logf("Start DUT interface Config")
	configureDUT(t, dut)

	cases := []struct {
		desc                      string
		policy                    string
		installed, received, sent uint32
		wantLoss                  bool
	}{{
		desc:      "Configure Accept All Policy",
		policy:    acceptPolicy,
		installed: routeCount,
		received:  routeCount,
		sent:      0,
		wantLoss:  false,
	}, {
		desc:      "Configure Deny All Policy",
		policy:    denyPolicy,
		installed: 0,
		received:  routeCount,
		sent:      0,
		wantLoss:  true,
	}, {
		desc:      "Configure Set Local Preference Policy",
		policy:    setLocalPrefPolicy,
		installed: routeCount,
		received:  routeCount,
		sent:      0,
		wantLoss:  false,
	}, {
		desc:      "Configure Set AS Path prepend Policy",
		policy:    setAspathPrependPolicy,
		installed: routeCount,
		received:  routeCount,
		sent:      0,
		wantLoss:  false,
	}}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			dut := ondatra.DUT(t, "dut")
			ate := ondatra.ATE(t, "ate")

			// Configure Routing Policy on the DUT
			dutConfPath := gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
			fptest.LogQuery(t, "DUT BGP Config before", dutConfPath.Config(), gnmi.GetConfig(t, dut, dutConfPath.Config()))
			d := &oc.Root{}
			t.Log("Configure BGP Policy with BGP actions on the neighbor")
			rpl := configureBGPPolicy(d)
			gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().Config(), rpl)
			bgp := bgpCreateNbr(dutAS, ateAS, tc.policy)
			// Configure ATE to setup traffic.
			otg := ate.OTG()
			otgConfig := configureATE(t, otg)
			gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp().Config(), bgp)

			// Verify the OTG BGP state
			t.Logf("Verify OTG BGP sessions up")
			verifyOTGBGPTelemetry(t, otg, otgConfig, "ESTABLISHED")

			// Send and verify traffic.
			sendTraffic(t, otg, otgConfig)
			verifyTraffic(t, ate, otgConfig, tc.wantLoss)

			// Verify traffic and telemetry.
			verifyPrefixesTelemetry(t, dut, tc.installed, tc.received, tc.sent)
			verifyPrefixesTelemetryV6(t, dut, tc.installed, tc.received, tc.sent)
			verifyPolicyTelemetry(t, dut, tc.policy)
		})
	}
}
