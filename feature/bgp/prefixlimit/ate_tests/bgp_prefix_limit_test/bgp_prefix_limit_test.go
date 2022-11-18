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

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/ixnet"
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
	trafficDuration        = 1 * time.Minute
	grTimer                = 2 * time.Minute
	grRestartTime          = 60
	grStaleRouteTime       = 300.0
	ipv4SrcTraffic         = "192.0.2.2"
	ipv6SrcTraffic         = "2001:db8::192:0:2:2"
	ipv4DstTrafficStart    = "203.0.113.1"
	ipv4DstTrafficEnd      = "203.0.113.254"
	ipv6DstTrafficStart    = "2001:db8::203:0:113:1"
	ipv6DstTrafficEnd      = "2001:db8::203:0:113:fe"
	advertisedRoutesv4CIDR = "203.0.113.1/32"
	advertisedRoutesv6CIDR = "2001:db8::203:0:113:1/128"
	prefixLimit            = 200
	pwarnthesholdPct       = 10
	prefixTimer            = 30.0
	dutAS                  = 64500
	ateAS                  = 64501
	plenIPv4               = 30
	plenIPv6               = 126
	tolerance              = 50
	lossTolerance          = 2
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

// configureDUT configures all the interfaces and BGP on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	dc := gnmi.OC()
	p1 := dut.Port(t, "port1").Name()
	i1 := dutSrc.NewOCInterface(p1)
	gnmi.Replace(t, dut, dc.Interface(p1).Config(), i1)

	p2 := dut.Port(t, "port2").Name()
	i2 := dutDst.NewOCInterface(p2)
	gnmi.Replace(t, dut, dc.Interface(p2).Config(), i2)

	dutConfPath := dc.NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	dutConf := createBGPNeighbor(dutAS, ateAS, prefixLimit, grRestartTime)
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

type config struct {
	topo     *ondatra.ATETopology
	allNets  []*ixnet.Network
	allFlows []*ondatra.Flow
}

// configureATE configures the interfaces and BGP on the ATE, with port2 advertising routes.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) *config {
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
	BGPDut1 := iDut1.BGP()
	BGPDut1.AddPeer().WithPeerAddress(dutSrc.IPv4).WithLocalASN(ateAS).
		WithTypeExternal()
	BGPDut1.AddPeer().WithPeerAddress(dutSrc.IPv6).WithLocalASN(ateAS).
		WithTypeExternal()

	BGPDut2 := iDut2.BGP()
	BGPDut2.AddPeer().WithPeerAddress(dutDst.IPv4).WithLocalASN(ateAS).
		WithTypeExternal()
	BGPDut2.AddPeer().WithPeerAddress(dutDst.IPv6).WithLocalASN(ateAS).
		WithTypeExternal()

	BGPNeti1 := iDut2.AddNetwork(advertisedRoutesv4CIDR)
	BGPNeti1.IPv4().WithAddress(advertisedRoutesv4CIDR).WithCount(1)
	BGPNeti1.BGP().WithNextHopAddress(ateDst.IPv4)
	BGPNeti1v6 := iDut2.AddNetwork(advertisedRoutesv6CIDR)
	BGPNeti1v6.IPv6().WithAddress(advertisedRoutesv6CIDR).WithCount(1)
	BGPNeti1v6.BGP().WithActive(true).WithNextHopAddress(ateDst.IPv6)

	t.Logf("Pushing config to ATE and starting protocols...")
	topo.Push(t)
	topo.StartProtocols(t)

	// ATE Traffic Configuration
	t.Logf("TestBGP:start ate Traffic config")
	ethHeader := ondatra.NewEthernetHeader()
	//  BGP V4 Traffic
	ipv4Header := ondatra.NewIPv4Header()
	ipv4Header.WithSrcAddress(ipv4SrcTraffic).DstAddressRange().
		WithMin(ipv4DstTrafficStart).WithMax(ipv4DstTrafficEnd).
		WithCount(prefixLimit)
	flowIPV4 := ate.Traffic().NewFlow("Ipv4").
		WithSrcEndpoints(iDut1).
		WithDstEndpoints(iDut2).
		WithHeaders(ethHeader, ipv4Header).
		WithFrameSize(512)

	// BGP IP V6 traffic
	ipv6Header := ondatra.NewIPv6Header()
	ipv6Header.WithECN(0).WithSrcAddress(ipv6SrcTraffic).
		DstAddressRange().WithMin(ipv6DstTrafficStart).WithMax(ipv6DstTrafficEnd).
		WithCount(prefixLimit)
	flowIPV6 := ate.Traffic().NewFlow("Ipv6").
		WithSrcEndpoints(iDut1).
		WithDstEndpoints(iDut2).
		WithHeaders(ethHeader, ipv6Header).
		WithFrameSize(512)

	return &config{topo, []*ixnet.Network{BGPNeti1, BGPNeti1v6}, []*ondatra.Flow{flowIPV4, flowIPV6}}
}

type BGPNeighbor struct {
	as, pfxLimit uint32
	neighborip   string
	isV4         bool
}

func createBGPNeighbor(localAs, peerAs, pLimit uint32, restartTime uint16) *oc.NetworkInstance_Protocol_Bgp {

	nbrs := []*BGPNeighbor{
		{as: peerAs, pfxLimit: pLimit, neighborip: ateSrc.IPv4, isV4: true},
		{as: peerAs, pfxLimit: pLimit, neighborip: ateSrc.IPv6, isV4: false},
		{as: peerAs, pfxLimit: pLimit, neighborip: ateDst.IPv4, isV4: true},
		{as: peerAs, pfxLimit: pLimit, neighborip: ateDst.IPv6, isV4: false},
	}

	d := &oc.Root{}
	ni1 := d.GetOrCreateNetworkInstance(*deviations.DefaultNetworkInstance)
	bgp := ni1.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").GetOrCreateBgp()
	global := bgp.GetOrCreateGlobal()
	global.As = ygot.Uint32(localAs)

	for _, nbr := range nbrs {
		if nbr.isV4 {
			nv4 := bgp.GetOrCreateNeighbor(nbr.neighborip)
			nv4.PeerAs = ygot.Uint32(nbr.as)
			nv4.Enabled = ygot.Bool(true)
			nv4.GetOrCreateTimers().RestartTime = ygot.Uint16(restartTime)
			afisafi := nv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
			afisafi.Enabled = ygot.Bool(true)
			prefixLimit := afisafi.GetOrCreateIpv4Unicast().GetOrCreatePrefixLimit()
			prefixLimit.MaxPrefixes = ygot.Uint32(nbr.pfxLimit)
		} else {
			nv6 := bgp.GetOrCreateNeighbor(nbr.neighborip)
			nv6.PeerAs = ygot.Uint32(nbr.as)
			nv6.Enabled = ygot.Bool(true)
			nv6.GetOrCreateTimers().RestartTime = ygot.Uint16(restartTime)
			afisafi6 := nv6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
			afisafi6.Enabled = ygot.Bool(true)
			prefixLimit6 := afisafi6.GetOrCreateIpv6Unicast().GetOrCreatePrefixLimit()
			prefixLimit6.MaxPrefixes = ygot.Uint32(nbr.pfxLimit)
		}
	}
	return bgp
}

func waitForBGPSession(t *testing.T, dut *ondatra.DUTDevice, wantEstablished bool) {
	statePath := gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
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

func verifyPrefixLimitTelemetry(t *testing.T, n *oc.NetworkInstance_Protocol_Bgp_Neighbor, wantEstablished bool) {
	t.Run("verifyPrefixLimitTelemetry", func(t *testing.T) {
		// TODO: Remove skip when Telemetry Parameters are supported
		t.Skip("Skipped since Telemetry parameters are not supported")
		plv4 := n.GetAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).GetIpv4Unicast().GetPrefixLimit()
		plv6 := n.GetAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).GetIpv6Unicast().GetPrefixLimit()

		maxPrefix := plv4.GetMaxPrefixes()
		limitExceeded := plv4.GetPrefixLimitExceeded()
		if maxPrefix != prefixLimit {
			t.Errorf("PrefixLimit max-prefixes v4 mismatch: got %d, want %d", maxPrefix, prefixLimit)
		}
		if (wantEstablished && limitExceeded) || (!wantEstablished && !limitExceeded) {
			t.Errorf("PrefixLimitExceeded v4 mismatch: got %t, want %t", limitExceeded, !wantEstablished)
		}

		maxPrefix = plv6.GetMaxPrefixes()
		limitExceeded = plv6.GetPrefixLimitExceeded()
		if maxPrefix != prefixLimit {
			t.Errorf("PrefixLimit max-prefixes v6 mismatch: got %d, want %d", maxPrefix, prefixLimit)
		}
		if (wantEstablished && limitExceeded) || (!wantEstablished && !limitExceeded) {
			t.Errorf("PrefixLimitExceeded v6 mismatch: got %t, want %t", limitExceeded, !wantEstablished)
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
	statePath := gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	prefixes := statePath.Neighbor(ateDst.IPv4).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Prefixes()
	if got, ok := gnmi.Watch(t, dut, prefixes.Installed().State(), time.Minute, compare).Await(t); !ok {
		t.Errorf("Installed prefixes v4 mismatch: got %v, want %v", got, installedRoutes)
	}
	if got, ok := gnmi.Watch(t, dut, prefixes.Received().State(), time.Minute, compare).Await(t); !ok {
		t.Errorf("Received prefixes v4 mismatch: got %v, want %v", got, installedRoutes)
	}
	nv4 := gnmi.Get(t, dut, statePath.Neighbor(ateDst.IPv4).State())
	verifyPrefixLimitTelemetry(t, nv4, tc.wantEstablished)

	prefixesv6 := statePath.Neighbor(ateDst.IPv6).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Prefixes()
	if got, ok := gnmi.Watch(t, dut, prefixesv6.Installed().State(), time.Minute, compare).Await(t); !ok {
		t.Errorf("Installed prefixes v6 mismatch: got %v, want %v", got, installedRoutes)
	}
	if got, ok := gnmi.Watch(t, dut, prefixesv6.Received().State(), time.Minute, compare).Await(t); !ok {
		t.Errorf("Received prefixes v6 mismatch: got %v, want %v", got, installedRoutes)
	}
	nv6 := gnmi.Get(t, dut, statePath.Neighbor(ateDst.IPv6).State())
	verifyPrefixLimitTelemetry(t, nv6, tc.wantEstablished)
}

func (tc *testCase) verifyNoPacketLoss(t *testing.T, ate *ondatra.ATEDevice, allFlows []*ondatra.Flow) {
	captureTrafficStats(t, ate)
	for _, flow := range allFlows {
		lossPct := gnmi.Get(t, ate, gnmi.OC().Flow(flow.Name()).LossPct().State())
		if lossPct > lossTolerance {
			t.Errorf("Traffic Loss Pct for Flow %s: got %v, want 0", flow.Name(), lossPct)
		} else {
			t.Logf("Traffic Test Passed! Got %v loss", lossPct)
		}
	}
}

func (tc *testCase) verifyPacketLoss(t *testing.T, ate *ondatra.ATEDevice, allFlows []*ondatra.Flow) {
	captureTrafficStats(t, ate)
	for _, flow := range allFlows {
		lossPct := gnmi.Get(t, ate, gnmi.OC().Flow(flow.Name()).LossPct().State())
		if lossPct > (100-lossTolerance) && lossPct <= 100 {
			t.Logf("Traffic Test Passed! Loss seen as expected: got %v, want 100%% ", lossPct)
		} else {
			t.Errorf("Traffic %s is expected to fail: got %v, want 100%% failure", flow.Name(), lossPct)
		}
	}
}

func captureTrafficStats(t *testing.T, ate *ondatra.ATEDevice) {
	ap := ate.Port(t, "port1")
	aic1 := gnmi.OC().Interface(ap.Name()).Counters()
	sentPkts := gnmi.Get(t, ate, aic1.OutPkts().State())
	fptest.LogQuery(t, "ate:port1 counters", aic1.State(), gnmi.Get(t, ate, aic1.State()))

	op := ate.Port(t, "port2")
	aic2 := gnmi.OC().Interface(op.Name()).Counters()
	rxPkts := gnmi.Get(t, ate, aic2.InPkts().State())
	fptest.LogQuery(t, "ate:port2 counters", aic2.State(), gnmi.Get(t, ate, aic2.State()))
	var lostPkts uint64
	//account for control plane packets in rxPkts
	if rxPkts > sentPkts {
		lostPkts = rxPkts - sentPkts
	} else {
		lostPkts = sentPkts - rxPkts
	}
	t.Logf("Packets: %d sent, %d received, %d lost", sentPkts, rxPkts, lostPkts)

	if lostPkts > tolerance {
		t.Logf("Lost Packets: %d", lostPkts)
	} else {
		t.Log("Traffic Test Passed!")
	}
}

func sendTraffic(t *testing.T, ate *ondatra.ATEDevice, allFlows []*ondatra.Flow, duration time.Duration) {
	t.Log("Starting traffic")
	ate.Traffic().Start(t, allFlows...)
	time.Sleep(duration)
	ate.Traffic().Stop(t)
	t.Log("Traffic stopped")
}

func configureBGPRoutes(t *testing.T, topo *ondatra.ATETopology, allNets []*ixnet.Network, routeCount uint32) {
	for _, net := range allNets {
		netName := net.EndpointPB().GetNetworkName()
		net.BGP().ClearASPathSegments()
		if netName == advertisedRoutesv4CIDR {
			net.IPv4().WithAddress(advertisedRoutesv4CIDR).WithCount(routeCount)
			net.BGP().WithActive(true).WithNextHopAddress(ateDst.IPv4)
		}
		if netName == advertisedRoutesv6CIDR {
			net.IPv6().WithAddress(advertisedRoutesv6CIDR).WithCount(routeCount)
			net.BGP().WithActive(true).WithNextHopAddress(ateDst.IPv6)
		}
	}
	topo.UpdateNetworks(t)
}

type testCase struct {
	desc             string
	name             string
	numRoutes        uint32
	wantEstablished  bool
	wantNoPacketLoss bool
}

func (tc *testCase) run(t *testing.T, conf *config, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice) {
	t.Log(tc.desc)
	configureBGPRoutes(t, conf.topo, conf.allNets, tc.numRoutes)
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

	// Starting ATE Traffic
	t.Log("Verify Traffic statistics")
	sendTraffic(t, ate, conf.allFlows, trafficDuration)
	if tc.wantNoPacketLoss {
		t.Run("verifyNoPacketLoss", func(t *testing.T) {
			tc.verifyNoPacketLoss(t, ate, conf.allFlows)
		})
	} else {
		t.Run("verifyPacketLoss", func(t *testing.T) {
			tc.verifyPacketLoss(t, ate, conf.allFlows)
		})
	}
}

func TestTrafficBGPPrefixLimit(t *testing.T) {
	cases := []testCase{{
		name:             "UnderLimit",
		desc:             "BGP Prefixes within expected limit",
		numRoutes:        prefixLimit - 1,
		wantEstablished:  true,
		wantNoPacketLoss: true,
	}, {
		name:             "AtLimit",
		desc:             "BGP Prefixes at threshold of expected limit",
		numRoutes:        prefixLimit,
		wantEstablished:  true,
		wantNoPacketLoss: true,
	}, {
		name:             "OverLimit",
		desc:             "BGP Prefixes outside expected limit",
		numRoutes:        prefixLimit + 1,
		wantEstablished:  false,
		wantNoPacketLoss: false,
	}, {
		name:             "ReestablishedAtLimit",
		desc:             "BGP Session ReEstablished after prefixes are within limits",
		numRoutes:        prefixLimit,
		wantEstablished:  true,
		wantNoPacketLoss: true,
	}}

	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	// DUT Configuration
	t.Log("Start DUT interface Config")
	configureDUT(t, dut)

	// ATE Configuration.
	t.Log("Start ATE Config")
	conf := configureATE(t, ate)

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.run(t, conf, dut, ate)
		})
	}
}
