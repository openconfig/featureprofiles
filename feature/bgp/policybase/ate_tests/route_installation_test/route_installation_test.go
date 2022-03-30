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
	"log"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/helpers"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/telemetry"
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
	ateType                = "software"
	trafficDuration        = 10 * time.Second
	trafficPacketRate      = 100
	ipv4SrcTraffic         = "192.0.2.2"
	ipv6SrcTraffic         = "2001:db8::192:0:2:2"
	ipv4DstTrafficStart    = "203.0.113.1"
	ipv4DstTrafficEnd      = "203.0.113.254"
	ipv6DstTrafficStart    = "2001:db8::203:0:113:1"
	ipv6DstTrafficEnd      = "2001:db8::203:0:113:fe"
	advertisedRoutesv4CIDR = "203.0.113.1/32"
	advertisedRoutesv6CIDR = "2001:db8::203:0:113:1/128"
	peerGrpName            = "BGP-PEER-GROUP"
	routeCount             = 254
	dutAS                  = 64500
	ateAS                  = 64501
	badAS                  = 64502
	plenIPv4               = 30
	plenIPv6               = 126
	tolerance              = 50
	tolerancePct           = 1
	ipPrefixSet            = "203.0.113.0/29"
	prefixSubnetRange      = "29..32"
	allowConnected         = "ALLOW-CONNECTED"
	prefixSet              = "PREFIX-SET"
	defaultPolicy          = ""
	denyPolicy             = "DENY-ALL"
	acceptPolicy           = "PERMIT-ALL"
	setLocalPrefPolicy     = "SET-LOCAL-PREF"
	localPrefValue         = 100
	setAspathPrependPolicy = "SET-ASPATH-PREPEND"
	asPathRepeatValue      = 3
	aclStatement1          = "10"
	aclStatement2          = "20"
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
		MAC:     "00:00:01:01:01:01",
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
		MAC:     "00:00:02:01:01:01",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::192:0:2:6",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
)

// configureDUT configures all the interfaces on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	dc := dut.Config()
	i1 := dutSrc.NewInterface(dut.Port(t, "port1").Name())
	dc.Interface(i1.GetName()).Replace(t, i1)

	i2 := dutDst.NewInterface(dut.Port(t, "port2").Name())
	dc.Interface(i2.GetName()).Replace(t, i2)
}

// verifyPortsUp asserts that each port on the device is operating
func verifyPortsUp(t *testing.T, dev *ondatra.Device) {
	t.Helper()
	for _, p := range dev.Ports() {
		status := dev.Telemetry().Interface(p.Name()).OperStatus().Get(t)
		if want := telemetry.Interface_OperStatus_UP; status != want {
			t.Errorf("%s Status: got %v, want %v", p, status, want)
		}
	}
}

// bgpCreateNbr creates a BGP object with neighbors pointing to ateSrc and ateDst, optionally with
// a peer group policy.
func bgpCreateNbr(localAs, peerAs uint32, policy string) *telemetry.NetworkInstance_Protocol_Bgp {
	nbr1v4 := &bgpNeighbor{as: peerAs, neighborip: ateSrc.IPv4, isV4: true}
	nbr1v6 := &bgpNeighbor{as: peerAs, neighborip: ateSrc.IPv6, isV4: false}
	nbr2v4 := &bgpNeighbor{as: peerAs, neighborip: ateDst.IPv4, isV4: true}
	nbr2v6 := &bgpNeighbor{as: peerAs, neighborip: ateDst.IPv6, isV4: false}
	nbrs := []*bgpNeighbor{nbr1v4, nbr2v4, nbr1v6, nbr2v6}

	d := &telemetry.Device{}
	ni1 := d.GetOrCreateNetworkInstance("default")
	bgp := ni1.GetOrCreateProtocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").GetOrCreateBgp()
	global := bgp.GetOrCreateGlobal()
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
			nv4.GetOrCreateAfiSafi(telemetry.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
		} else {
			nv6 := bgp.GetOrCreateNeighbor(nbr.neighborip)
			nv6.PeerGroup = ygot.String(peerGrpName)
			nv6.PeerAs = ygot.Uint32(nbr.as)
			nv6.Enabled = ygot.Bool(true)
			nv6.GetOrCreateAfiSafi(telemetry.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)
		}

	}
	return bgp
}

// configureBGPPolicy configures a BGP routing policy to accept or reject routes based on prefix match conditions
// Additonally, it configures LocalPreference and ASPathprepend as part of the BGP policy
func configureBGPPolicy(d *telemetry.Device) *telemetry.RoutingPolicy {
	rp := d.GetOrCreateRoutingPolicy()
	pset := rp.GetOrCreateDefinedSets().GetOrCreatePrefixSet(prefixSet)
	pset.GetOrCreatePrefix(ipPrefixSet, prefixSubnetRange)
	pdef := rp.GetOrCreatePolicyDefinition(allowConnected)
	stmt5 := pdef.GetOrCreateStatement(aclStatement1)
	stmt5.GetOrCreateActions().PolicyResult = telemetry.RoutingPolicy_PolicyResultType_REJECT_ROUTE
	stmt5.GetOrCreateConditions().GetOrCreateMatchPrefixSet().PrefixSet = ygot.String(prefixSet)
	stmt10 := pdef.GetOrCreateStatement(aclStatement2)
	stmt10.GetOrCreateActions().PolicyResult = telemetry.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE

	pdef2 := rp.GetOrCreatePolicyDefinition(acceptPolicy)
	pdef2.GetOrCreateStatement(aclStatement2).GetOrCreateActions().PolicyResult = telemetry.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE

	pdef3 := rp.GetOrCreatePolicyDefinition(denyPolicy)
	pdef3.GetOrCreateStatement(aclStatement2).GetOrCreateActions().PolicyResult = telemetry.RoutingPolicy_PolicyResultType_REJECT_ROUTE

	pdef4 := rp.GetOrCreatePolicyDefinition(setLocalPrefPolicy)
	actions := pdef4.GetOrCreateStatement(aclStatement2).GetOrCreateActions()
	actions.GetOrCreateBgpActions().SetLocalPref = ygot.Uint32(localPrefValue)
	actions.PolicyResult = telemetry.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE

	pdef5 := rp.GetOrCreatePolicyDefinition(setAspathPrependPolicy)
	actions5 := pdef5.GetOrCreateStatement(aclStatement2).GetOrCreateActions()
	aspend := actions5.GetOrCreateBgpActions().GetOrCreateSetAsPathPrepend()
	aspend.Asn = ygot.Uint32(ateAS)
	aspend.RepeatN = ygot.Uint8(asPathRepeatValue)
	actions5.PolicyResult = telemetry.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE

	return rp
}

// verifyBgpTelemetry checks that the dut has an established BGP session with reasonable settings
func verifyBgpTelemetry(t *testing.T, dut *ondatra.DUTDevice) {
	ifName := dut.Port(t, "port1").Name()
	lastFlapTime := dut.Telemetry().Interface(ifName).LastChange().Get(t)
	t.Logf("Verifying BGP state")
	statePath := dut.Telemetry().NetworkInstance("default").Protocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	nbrPath := statePath.Neighbor(ateSrc.IPv4)
	nbrPathv6 := statePath.Neighbor(ateSrc.IPv6)
	nbr := statePath.Get(t).GetNeighbor(ateSrc.IPv4)

	// Get BGP adjacency state
	t.Logf("Waiting for BGP neighbor to establish...")
	_, ok := nbrPath.SessionState().Watch(t, time.Minute, func(val *telemetry.QualifiedE_Bgp_Neighbor_SessionState) bool {
		return val.IsPresent() && val.Val(t) == telemetry.Bgp_Neighbor_SessionState_ESTABLISHED
	}).Await(t)
	if !ok {
		fptest.LogYgot(t, "BGP reported state", nbrPath, nbrPath.Get(t))
		t.Fatal("No BGP neighbor formed")
	}
	status := nbrPath.SessionState().Get(t)
	t.Logf("BGP adjacency for %s: %s", ateSrc.IPv4, status)
	if want := telemetry.Bgp_Neighbor_SessionState_ESTABLISHED; status != want {
		t.Errorf("BGP peer %s status got %d, want %d", ateSrc.IPv4, status, want)
	}

	// Check last established timestamp
	lestTime := nbrPath.Get(t).GetLastEstablished()
	lestTimev6 := nbrPathv6.Get(t).GetLastEstablished()
	t.Logf("BGP last est time :%v, flapTime :%v", lestTime, lastFlapTime)
	t.Logf("BGP v6 last est time :%d", lestTimev6)
	if lestTime < lastFlapTime {
		t.Errorf("Bad last-established timestamp: got %v, want >= %v", lestTime, lastFlapTime)
	}

	// Check BGP Transitions
	estTrans := nbr.GetEstablishedTransitions()
	t.Logf("Got established transitions: %d", estTrans)
	if estTrans != 1 {
		// t.Errorf("Wrong established-transitions: got %v, want 1", estTrans)
		t.Logf("Wrong established-transitions: got %v, want 1", estTrans)
	}

	// Check BGP neighbor address from telemetry
	addrv4 := nbrPath.Get(t).GetNeighborAddress()
	addrv6 := nbrPathv6.Get(t).GetNeighborAddress()
	t.Logf("Got ipv4 neighbor address: %s", addrv4)
	if addrv4 != ateSrc.IPv4 {
		t.Errorf("BGP v4 neighbor address: got %v, want %v", addrv4, ateSrc.IPv4)
	}

	t.Logf("Got Ipv6 neighbor address: %s", addrv6)
	if addrv6 != ateSrc.IPv6 {
		t.Errorf("BGP v6 neighbor address: got %v, want %v", addrv6, ateSrc.IPv6)
	}

	// Check BGP neighbor address from telemetry
	peerAS := nbrPath.Get(t).GetPeerAs()
	if peerAS != ateAS {
		t.Errorf("BGP peerAs: got %v, want %v", peerAS, ateAS)
	}

	// Check BGP neighbor is enabled
	if !nbrPath.Get(t).GetEnabled() {
		t.Errorf("Expected neighbor %v to be enabled", ateSrc.IPv4)
	}
}

// verifyPrefixesTelemetry confirms that the dut shows the correct numbers of installed, sent and
// received IPv4 prefixes
// TODO: Need to refactor and compare using cmp.diff
func verifyPrefixesTelemetry(t *testing.T, dut *ondatra.DUTDevice, wantInstalled, wantRx, wantSent uint32) {
	statePath := dut.Telemetry().NetworkInstance("default").Protocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	prefixesv4 := statePath.Neighbor(ateDst.IPv4).AfiSafi(telemetry.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Prefixes()
	if gotInstalled := prefixesv4.Installed().Get(t); gotInstalled != wantInstalled {
		t.Errorf("Installed prefixes mismatch: got %v, want %v", gotInstalled, wantInstalled)
	}
	if gotRx := prefixesv4.ReceivedPrePolicy().Get(t); gotRx != wantRx {
		t.Errorf("Received prefixes mismatch: got %v, want %v", gotRx, wantRx)
	}
	if gotSent := prefixesv4.Sent().Get(t); gotSent != wantSent {
		t.Errorf("Sent prefixes mismatch: got %v, want %v", gotSent, wantSent)
	}
}

// verifyPrefixesTelemetryV6 confirms that the dut shows the correct numbers of installed, sent and
// received IPv6 prefixes
func verifyPrefixesTelemetryV6(t *testing.T, dut *ondatra.DUTDevice, wantInstalledv6, wantRxv6, wantSentv6 uint32) {
	statePath := dut.Telemetry().NetworkInstance("default").Protocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	prefixesv6 := statePath.Neighbor(ateDst.IPv6).AfiSafi(telemetry.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Prefixes()

	if gotInstalledv6 := prefixesv6.Installed().Get(t); gotInstalledv6 != wantInstalledv6 {
		t.Errorf("IPV6 Installed prefixes mismatch: got %v, want %v", gotInstalledv6, wantInstalledv6)
	}
	if gotRxv6 := prefixesv6.ReceivedPrePolicy().Get(t); gotRxv6 != wantRxv6 {
		t.Errorf("IPV6 Received prefixes mismatch: got %v, want %v", gotRxv6, wantRxv6)
	}
	if gotSentv6 := prefixesv6.Sent().Get(t); gotSentv6 != wantSentv6 {
		t.Errorf("IPv6 Sent prefixes mismatch: got %v, want %v", gotSentv6, wantSentv6)
	}
}

// verifyPolicyTelemetry confirms that the dut policy is set as expected.
func verifyPolicyTelemetry(t *testing.T, dut *ondatra.DUTDevice, policy string) {
	statePath := dut.Telemetry().NetworkInstance("default").Protocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	policytel := statePath.PeerGroup(peerGrpName).ApplyPolicy().ImportPolicy().Get(t)
	for _, val := range policytel {
		if val != policy {
			t.Errorf("Apply policy mismatch: got %v, want %v", policytel, policy)
		}
	}
}

// configureATE configures the interfaces and BGP protocols on an ATE, including advertising some
// (faked) networks over BGP.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) []*ondatra.Flow {
	port1 := ate.Port(t, "port1")
	topo := ate.Topology().New()
	iDut1 := topo.AddInterface(ateSrc.Name).WithPort(port1)
	iDut1.IPv4().WithAddress(ateSrc.IPv4CIDR()).WithDefaultGateway(dutSrc.IPv4)
	iDut1.IPv6().WithAddress(ateSrc.IPv6CIDR()).WithDefaultGateway(dutSrc.IPv6)

	port2 := ate.Port(t, "port2")
	iDut2 := topo.AddInterface(ateDst.Name).WithPort(port2)
	iDut2.IPv4().WithAddress(ateDst.IPv4CIDR()).WithDefaultGateway(dutDst.IPv4)
	iDut2.IPv6().WithAddress(ateDst.IPv6CIDR()).WithDefaultGateway(dutDst.IPv6)

	// Setup ATE BGP route v4 advertisement
	bgpDut1 := iDut1.BGP()
	bgpDut1.AddPeer().WithPeerAddress(dutSrc.IPv4).WithLocalASN(ateAS).
		WithTypeExternal()
	bgpDut1.AddPeer().WithPeerAddress(dutSrc.IPv6).WithLocalASN(ateAS).
		WithTypeExternal()

	bgpDut2 := iDut2.BGP()
	bgpDut2.AddPeer().WithPeerAddress(dutDst.IPv4).WithLocalASN(ateAS).
		WithTypeExternal()
	bgpDut2.AddPeer().WithPeerAddress(dutDst.IPv6).WithLocalASN(ateAS).
		WithTypeExternal()

	bgpNeti1 := iDut2.AddNetwork("bgpNeti1")
	bgpNeti1.IPv4().WithAddress(advertisedRoutesv4CIDR).WithCount(routeCount)
	bgpNeti1.BGP().WithNextHopAddress(ateDst.IPv4)

	bgpNeti1v6 := iDut2.AddNetwork("bgpNeti1v6")
	bgpNeti1v6.IPv6().WithAddress(advertisedRoutesv6CIDR).WithCount(routeCount)
	bgpNeti1v6.BGP().WithActive(true).WithNextHopAddress(ateDst.IPv6)

	t.Logf("Pushing config to ATE and starting protocols...")
	topo.Push(t)
	topo.StartProtocols(t)

	// ATE Traffic Configuration
	t.Logf("TestBGP:start ate Traffic config")
	ethHeader := ondatra.NewEthernetHeader()
	// BGP V4 Traffic
	ipv4Header := ondatra.NewIPv4Header()
	ipv4Header.WithSrcAddress(ipv4SrcTraffic).DstAddressRange().
		WithMin(ipv4DstTrafficStart).WithMax(ipv4DstTrafficEnd).
		WithCount(routeCount)
	flowipv4 := ate.Traffic().NewFlow("Ipv4").
		WithSrcEndpoints(iDut1).
		WithDstEndpoints(iDut2).
		WithHeaders(ethHeader, ipv4Header).
		WithFrameSize(512)

	// BGP IP V6 traffic
	ipv6Header := ondatra.NewIPv6Header()
	ipv6Header.WithECN(0).WithSrcAddress(ipv6SrcTraffic).
		DstAddressRange().WithMin(ipv6DstTrafficStart).WithMax(ipv6DstTrafficEnd).
		WithCount(routeCount)
	flowipv6 := ate.Traffic().NewFlow("Ipv6").
		WithSrcEndpoints(iDut1).
		WithDstEndpoints(iDut2).
		WithHeaders(ethHeader, ipv6Header).
		WithFrameSize(512)

	return []*ondatra.Flow{flowipv4, flowipv6}
}

// configureOTG configures the interfaces and BGP protocols on an OTG, including advertising some
// (faked) networks over BGP.
func configureOTG(t *testing.T, ate *ondatra.ATEDevice, otg *ondatra.OTG) (gosnappi.Config, helpers.ExpectedState) {

	config := otg.NewConfig()
	srcPort := config.Ports().Add().SetName("port1")
	dstPort := config.Ports().Add().SetName("port2")

	srcDev := config.Devices().Add().SetName(ateSrc.Name)
	srcEth := srcDev.Ethernets().Add().
		SetName(ateSrc.Name + ".eth").
		SetPortName(srcPort.Name()).
		SetMac(ateSrc.MAC)
	srcIpv4 := srcEth.Ipv4Addresses().Add().
		SetName(ateSrc.Name + ".ipv4").
		SetAddress(ateSrc.IPv4).
		SetGateway(dutSrc.IPv4).
		SetPrefix(int32(ateSrc.IPv4Len))
	srcIpv6 := srcEth.Ipv6Addresses().Add().
		SetName(ateSrc.Name + ".ipv6").
		SetAddress(ateSrc.IPv6).
		SetGateway(dutSrc.IPv6).
		SetPrefix(int32(ateSrc.IPv6Len))

	dstDev := config.Devices().Add().SetName(ateDst.Name)
	dstEth := dstDev.Ethernets().Add().
		SetName(ateDst.Name + ".eth").
		SetPortName(dstPort.Name()).
		SetMac(ateDst.MAC)
	dstIpv4 := dstEth.Ipv4Addresses().Add().
		SetName(ateDst.Name + ".ipv4").
		SetAddress(ateDst.IPv4).
		SetGateway(dutDst.IPv4).
		SetPrefix(int32(ateDst.IPv4Len))
	dstIpv6 := dstEth.Ipv6Addresses().Add().
		SetName(ateDst.Name + ".ipv6").
		SetAddress(ateDst.IPv6).
		SetGateway(dutDst.IPv6).
		SetPrefix(int32(ateDst.IPv6Len))

	srcBgp4Name := ateSrc.Name + ".bgp4.peer"
	srcBgp6Name := ateSrc.Name + ".bgp6.peer"
	srcBgp := srcDev.Bgp().
		SetRouterId(srcIpv4.Address())
	srcBgp4Peer := srcBgp.Ipv4Interfaces().Add().
		SetIpv4Name(srcIpv4.Name()).
		Peers().Add().
		SetName(srcBgp4Name).
		SetPeerAddress(srcIpv4.Gateway()).
		SetAsNumber(ateAS).
		SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
	srcBgp6Peer := srcBgp.Ipv6Interfaces().Add().
		SetIpv6Name(srcIpv6.Name()).
		Peers().Add().
		SetName(srcBgp6Name).
		SetPeerAddress(srcIpv6.Gateway()).
		SetAsNumber(ateAS).
		SetAsType(gosnappi.BgpV6PeerAsType.EBGP)

	dstBgp4Name := ateDst.Name + ".bgp4.peer"
	dstBgp6Name := ateDst.Name + ".bgp6.peer"
	dstBgp := dstDev.Bgp().
		SetRouterId(dstIpv4.Address())
	dstBgp4Peer := dstBgp.Ipv4Interfaces().Add().
		SetIpv4Name(dstIpv4.Name()).
		Peers().Add().
		SetName(dstBgp4Name).
		SetPeerAddress(dstIpv4.Gateway()).
		SetAsNumber(ateAS).
		SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
	dstBgp6Peer := dstBgp.Ipv6Interfaces().Add().
		SetIpv6Name(dstIpv6.Name()).
		Peers().Add().
		SetName(dstBgp6Name).
		SetPeerAddress(dstIpv6.Gateway()).
		SetAsNumber(ateAS).
		SetAsType(gosnappi.BgpV6PeerAsType.EBGP)

	prefixInt4, _ := strconv.Atoi(strings.Split(advertisedRoutesv4CIDR, "/")[1])
	prefixInt6, _ := strconv.Atoi(strings.Split(advertisedRoutesv6CIDR, "/")[1])
	dstBgp4PeerRoutes := dstBgp4Peer.V4Routes().Add().
		SetName(dstBgp4Name + ".rr4").
		SetNextHopIpv4Address(dstIpv4.Address()).
		SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).
		SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)
	dstBgp4PeerRoutes.Addresses().Add().
		SetAddress(strings.Split(advertisedRoutesv4CIDR, "/")[0]).
		SetPrefix(int32(prefixInt4)).
		SetCount(routeCount)
	dstBgp6PeerRoutes := dstBgp6Peer.V6Routes().Add().
		SetName(dstBgp6Name + ".rr6").
		SetNextHopIpv6Address(dstIpv6.Address()).
		SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).
		SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL)
	dstBgp6PeerRoutes.Addresses().Add().
		SetAddress(strings.Split(advertisedRoutesv6CIDR, "/")[0]).
		SetPrefix(int32(prefixInt6)).
		SetCount(routeCount)

	// ATE Traffic Configuration
	t.Logf("TestBGP:start ate Traffic config")
	flowipv4 := config.Flows().Add().SetName("bgpv4RoutesFlow")
	flowipv4.Metrics().SetEnable(true)
	flowipv4.TxRx().Device().
		SetTxNames([]string{srcIpv4.Name()}).
		SetRxNames([]string{dstBgp4PeerRoutes.Name()})
	flowipv4.Size().SetFixed(512)
	flowipv4.Rate().SetPps(trafficPacketRate)
	flowipv4.Duration().SetChoice("continuous")
	e1 := flowipv4.Packet().Add().Ethernet()
	e1.Src().SetValue(srcEth.Mac())
	v4 := flowipv4.Packet().Add().Ipv4()
	v4.Src().SetValue(srcIpv4.Address())
	v4.Dst().Increment().SetStart(strings.Split(advertisedRoutesv4CIDR, "/")[0]).SetCount(routeCount)

	flowipv6 := config.Flows().Add().SetName("bgpv6RoutesFlow")
	flowipv6.Metrics().SetEnable(true)
	flowipv6.TxRx().Device().
		SetTxNames([]string{srcIpv6.Name()}).
		SetRxNames([]string{dstBgp6PeerRoutes.Name()})
	flowipv6.Size().SetFixed(512)
	flowipv6.Rate().SetPps(trafficPacketRate)
	flowipv6.Duration().SetChoice("continuous")
	e2 := flowipv6.Packet().Add().Ethernet()
	e2.Src().SetValue(srcEth.Mac())
	v6 := flowipv6.Packet().Add().Ipv6()
	v6.Src().SetValue(srcIpv6.Address())
	v6.Dst().Increment().SetStart(strings.Split(advertisedRoutesv6CIDR, "/")[0]).SetCount(routeCount)

	expected := helpers.ExpectedState{
		Bgp4: map[string]helpers.ExpectedBgpMetrics{
			srcBgp4Peer.Name(): {Advertised: 0, Received: routeCount},
			dstBgp4Peer.Name(): {Advertised: routeCount, Received: 0},
		},
		Bgp6: map[string]helpers.ExpectedBgpMetrics{
			srcBgp6Peer.Name(): {Advertised: 0, Received: routeCount},
			dstBgp6Peer.Name(): {Advertised: routeCount, Received: 0},
		},
		Flow: map[string]helpers.ExpectedFlowMetrics{
			flowipv4.Name(): {FramesRx: 0, FramesRxRate: 0},
			flowipv6.Name(): {FramesRx: 0, FramesRxRate: 0},
		},
	}

	t.Logf("Pushing config to ATE and starting protocols...")
	otg.PushConfig(t, ate, config)
	otg.StartProtocols(t)
	return config, expected
}

// verifyTraffic confirms that every traffic flow has the expected amount of loss (0% or 100%
// depending on wantLoss, +- 2%)
func verifyTrafficATE(t *testing.T, ate *ondatra.ATEDevice, allFlows []*ondatra.Flow, wantLoss bool) {
	captureTrafficStats(t, ate, wantLoss)
	for _, flow := range allFlows {
		lossPct := ate.Telemetry().Flow(flow.Name()).LossPct().Get(t)
		if !wantLoss {
			if lossPct > tolerancePct {
				t.Errorf("Traffic Loss Pct for Flow: %s\n got %v, want 0", flow.Name(), lossPct)
			} else {
				t.Logf("Traffic Test Passed!")
			}
		} else {
			if lossPct < 100-tolerancePct {
				t.Errorf("Traffic is expected to fail %s\n got %v, want 100%% failure", flow.Name(), lossPct)
			} else {
				t.Logf("Traffic Loss Test Passed!")
			}
		}
	}
}

func verifyTrafficOTG(t *testing.T, gnmiClient *helpers.GnmiClient, wantLoss bool) {
	fMetrics, err := gnmiClient.GetFlowMetrics([]string{})
	if err != nil {
		t.Fatal("Error while getting the flow metrics")
	}

	helpers.PrintMetricsTable(&helpers.MetricsTableOpts{
		ClearPrevious: false,
		FlowMetrics:   fMetrics,
	})

	for _, f := range fMetrics.Items() {
		lostPackets := f.FramesTx() - f.FramesRx()
		lossPct := lostPackets * 100 / f.FramesTx()
		if !wantLoss {
			if lostPackets > tolerance {
				t.Logf("Packets received not matching packets sent. Sent: %v, Received: %d", f.FramesTx(), f.FramesRx())
			}
			if lossPct > tolerancePct && f.FramesTx() > 0 {
				t.Errorf("Traffic Loss Pct for Flow: %s\n got %v, want max %v pct failure", f.Name(), lossPct, tolerancePct)
			} else {
				t.Logf("Traffic Test Passed! for flow %s", f.Name())
			}
		} else {
			if lossPct < 100-tolerancePct && f.FramesTx() > 0 {
				t.Errorf("Traffic is expected to fail %s\n got %v, want max %v pct failure", f.Name(), lossPct, 100-tolerancePct)
			} else {
				t.Logf("Traffic Loss Test Passed! for flow %s", f.Name())
			}
		}
	}
}

func captureTrafficStats(t *testing.T, ate *ondatra.ATEDevice, wantLoss bool) {
	ap := ate.Port(t, "port1")
	aic1 := ate.Telemetry().Interface(ap.Name()).Counters()
	outPkts := aic1.OutPkts().Get(t)
	fptest.LogYgot(t, "ate:port1 counters", aic1, aic1.Get(t))

	op := ate.Port(t, "port2")
	aic2 := ate.Telemetry().Interface(op.Name()).Counters()
	inPkts := aic2.InPkts().Get(t)
	fptest.LogYgot(t, "ate:port2 counters", aic2, aic2.Get(t))

	lostPkts := inPkts - outPkts
	t.Logf("Sent Packets: %d, received Packets: %d", outPkts, inPkts)
	if !wantLoss {
		if lostPkts > tolerance {
			t.Logf("Packets received not matching packets sent. Sent: %v, Received: %d", outPkts, inPkts)
		} else {
			t.Logf("Traffic Test Passed! Sent: %d, Received: %d", outPkts, inPkts)
		}
	}
}

func sendTrafficATE(t *testing.T, ate *ondatra.ATEDevice, allFlows []*ondatra.Flow) {
	t.Logf("Starting traffic")
	ate.Traffic().Start(t, allFlows...)
	time.Sleep(trafficDuration)
	ate.Traffic().Stop(t)
	t.Logf("Stop traffic")
}

func sendTrafficOTG(t *testing.T, otg *ondatra.OTG, gnmiClient *helpers.GnmiClient) {
	t.Logf("Starting traffic")
	otg.StartTraffic(t)
	err := gnmiClient.WatchFlowMetrics(&helpers.WaitForOpts{Interval: 2 * time.Second, Timeout: trafficDuration})
	if err != nil {
		log.Println(err)
	}
	t.Logf("Stop traffic")
	otg.StopTraffic(t)
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
	dutConfPath := dut.Config().NetworkInstance("default").Protocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	fptest.LogYgot(t, "DUT BGP Config before", dutConfPath, dutConfPath.Get(t))
	dutConfPath.Replace(t, nil)
	dutConf := bgpCreateNbr(dutAS, ateAS, defaultPolicy)
	dutConfPath.Replace(t, dutConf)

	// ATE Configuration.
	t.Logf("Start ATE Config")
	ate := ondatra.ATE(t, "ate")

	var allFlows []*ondatra.Flow
	var otgConfig gosnappi.Config
	var otgExpected helpers.ExpectedState
	var otg *ondatra.OTG
	switch {
	case ateType == "hardware":
		allFlows = configureATE(t, ate)
	case ateType == "software":
		otg = ate.OTG(t)
		otgConfig, otgExpected = configureOTG(t, ate, otg)
	}
	// Verify Port Status
	t.Logf("Verifying port status")
	verifyPortsUp(t, dut.Device)

	t.Logf("Check BGP parameters")
	verifyBgpTelemetry(t, dut)

	switch {
	case ateType == "software":
		t.Logf("Check BGP sessions on OTG")
		gnmiClient, err := helpers.NewGnmiClient(otg.NewGnmiQuery(t), otgConfig)
		if err != nil {
			t.Fatal(err)
		}
		helpers.WaitFor(t, func() (bool, error) { return gnmiClient.AllBgp4SessionUp(otgExpected) }, nil)
		helpers.WaitFor(t, func() (bool, error) { return gnmiClient.AllBgp6SessionUp(otgExpected) }, nil)
		gnmiClient.Close()
	}

	// Starting ATE Traffic and verify Traffic Flows and packet loss
	switch {
	case ateType == "hardware":
		sendTrafficATE(t, ate, allFlows)
		verifyTrafficATE(t, ate, allFlows, false)
	case ateType == "software":
		gnmiClient, err := helpers.NewGnmiClient(otg.NewGnmiQuery(t), otgConfig)
		if err != nil {
			t.Fatal(err)
		}
		sendTrafficOTG(t, otg, gnmiClient)
		verifyTrafficOTG(t, gnmiClient, false)
		gnmiClient.Close()
	}
	verifyPrefixesTelemetry(t, dut, routeCount, routeCount, 0)
	verifyPrefixesTelemetryV6(t, dut, routeCount, routeCount, 0)

	t.Run("RoutesWithdrawn", func(t *testing.T) {
		t.Log("Breaking BGP config and confirming that forwarding stops working.")
		// Break config with a mismatching AS number
		dutConfPath.Replace(t, bgpCreateNbr(dutAS, badAS, defaultPolicy))

		// Resend traffic
		switch {
		case ateType == "hardware":
			sendTrafficATE(t, ate, allFlows)
			// Verify traffic fails as routes are withdrawn and 100% packet loss is seen.
			verifyTrafficATE(t, ate, allFlows, true)
		case ateType == "software":
			gnmiClient, err := helpers.NewGnmiClient(otg.NewGnmiQuery(t), otgConfig)
			if err != nil {
				t.Fatal(err)
			}
			sendTrafficOTG(t, otg, gnmiClient)
			verifyTrafficOTG(t, gnmiClient, true)
			gnmiClient.Close()
		}
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
			dutConfPath := dut.Config().NetworkInstance("default").Protocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
			fptest.LogYgot(t, "DUT BGP Config before", dutConfPath, dutConfPath.Get(t))
			d := &telemetry.Device{}
			t.Log("Configure BGP Policy with BGP actions on the neighbor")
			rpl := configureBGPPolicy(d)
			dut.Config().RoutingPolicy().Replace(t, rpl)
			bgp := bgpCreateNbr(dutAS, ateAS, tc.policy)
			// Configure ATE to setup traffic.
			var allFlows []*ondatra.Flow
			var otgConfig gosnappi.Config
			var otg *ondatra.OTG
			switch {
			case ateType == "hardware":
				allFlows = configureATE(t, ate)
			case ateType == "software":
				otg = ate.OTG(t)
				otgConfig, _ = configureOTG(t, ate, otg)
			}
			dut.Config().NetworkInstance("default").Protocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp().Replace(t, bgp)
			// Send and verify traffic.
			switch {
			case ateType == "hardware":
				sendTrafficATE(t, ate, allFlows)
				verifyTrafficATE(t, ate, allFlows, tc.wantLoss)
			case ateType == "software":
				gnmiClient, err := helpers.NewGnmiClient(otg.NewGnmiQuery(t), otgConfig)
				if err != nil {
					t.Fatal(err)
				}
				sendTrafficOTG(t, otg, gnmiClient)
				verifyTrafficOTG(t, gnmiClient, tc.wantLoss)
				gnmiClient.Close()
			}
			// Verify traffic and telemetry.
			verifyPrefixesTelemetry(t, dut, tc.installed, tc.received, tc.sent)
			verifyPrefixesTelemetryV6(t, dut, tc.installed, tc.received, tc.sent)
			verifyPolicyTelemetry(t, dut, tc.policy)
		})
	}
}

func TestUnsetDut(t *testing.T) {
	t.Logf("Start Unsetting DUT Config")
	helpers.ConfigDUTs(map[string]string{"arista": "unset_dut.txt"})
}
