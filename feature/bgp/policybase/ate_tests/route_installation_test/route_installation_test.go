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

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
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
	trafficDuration        = 1 * time.Minute
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
	tolerancePct           = 2
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
	ni1 := d.GetOrCreateNetworkInstance(*deviations.DefaultNetworkInstance)
	bgp := ni1.GetOrCreateProtocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").GetOrCreateBgp()
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
	statePath := dut.Telemetry().NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
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
		t.Errorf("Wrong established-transitions: got %v, want 1", estTrans)
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
	statePath := dut.Telemetry().NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
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
	statePath := dut.Telemetry().NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
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
	statePath := dut.Telemetry().NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
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

// verifyTraffic confirms that every traffic flow has the expected amount of loss (0% or 100%
// depending on wantLoss, +- 2%)
func verifyTraffic(t *testing.T, ate *ondatra.ATEDevice, allFlows []*ondatra.Flow, wantLoss bool) {
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

func sendTraffic(t *testing.T, ate *ondatra.ATEDevice, allFlows []*ondatra.Flow) {
	t.Logf("Starting traffic")
	ate.Traffic().Start(t, allFlows...)
	time.Sleep(trafficDuration)
	ate.Traffic().Stop(t)
	t.Logf("Stop traffic")
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
	dutConfPath := dut.Config().NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	dutConfPath.Delete(t)
	dutConf := bgpCreateNbr(dutAS, ateAS, defaultPolicy)
	dutConfPath.Replace(t, dutConf)
	fptest.LogYgot(t, "DUT BGP Config", dutConfPath, dutConfPath.Get(t))

	// ATE Configuration.
	t.Logf("Start ATE Config")
	ate := ondatra.ATE(t, "ate")
	allFlows := configureATE(t, ate)

	// Verify Port Status
	t.Logf("Verifying port status")
	verifyPortsUp(t, dut.Device)

	t.Logf("Check BGP parameters")
	verifyBgpTelemetry(t, dut)

	// Starting ATE Traffic
	sendTraffic(t, ate, allFlows)

	// Verify Traffic Flows and packet loss
	verifyTraffic(t, ate, allFlows, false)
	verifyPrefixesTelemetry(t, dut, routeCount, routeCount, 0)
	verifyPrefixesTelemetryV6(t, dut, routeCount, routeCount, 0)

	t.Run("RoutesWithdrawn", func(t *testing.T) {
		t.Log("Breaking BGP config and confirming that forwarding stops working.")
		// Break config with a mismatching AS number
		dutConfPath.Replace(t, bgpCreateNbr(dutAS, badAS, defaultPolicy))

		// Resend traffic
		sendTraffic(t, ate, allFlows)

		// Verify traffic fails as routes are withdrawn and 100% packet loss is seen.
		verifyTraffic(t, ate, allFlows, true)
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
			dutConfPath := dut.Config().NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
			fptest.LogYgot(t, "DUT BGP Config before", dutConfPath, dutConfPath.Get(t))
			d := &telemetry.Device{}
			t.Log("Configure BGP Policy with BGP actions on the neighbor")
			rpl := configureBGPPolicy(d)
			dut.Config().RoutingPolicy().Replace(t, rpl)
			bgp := bgpCreateNbr(dutAS, ateAS, tc.policy)
			// Configure ATE to setup traffic.
			allFlows := configureATE(t, ate)
			dut.Config().NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp().Replace(t, bgp)
			// Send and verify traffic.
			sendTraffic(t, ate, allFlows)
			verifyTraffic(t, ate, allFlows, tc.wantLoss)
			// Verify traffic and telemetry.
			verifyPrefixesTelemetry(t, dut, tc.installed, tc.received, tc.sent)
			verifyPrefixesTelemetryV6(t, dut, tc.installed, tc.received, tc.sent)
			verifyPolicyTelemetry(t, dut, tc.policy)
		})
	}
}
