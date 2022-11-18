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
	grRestartTime            = 60
	grStaleRouteTime         = 300.0
	ipv4SrcTraffic           = "192.0.2.2"
	ipv6SrcTraffic           = "2001:db8::192:0:2:2"
	ipv4DstTrafficStart      = "203.0.113.1"
	ipv4DstTrafficEnd        = "203.0.113.254"
	ipv6DstTrafficStart      = "2001:db8::203:0:113:1"
	ipv6DstTrafficEnd        = "2001:db8::203:0:113:fe"
	advertisedRoutesv4CIDR   = "203.0.113.1/32"
	advertisedRoutesv6CIDR   = "2001:db8::203:0:113:1/128"
	advertisedRoutesv4Net    = "203.0.113.1"
	advertisedRoutesv6Net    = "2001:db8::203:0:113:1"
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
	lossTolerance            = 1
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
	topo       gosnappi.Config
	bgpv4RR    gosnappi.BgpV4RouteRange
	bgpv6RR    gosnappi.BgpV6RouteRange
	flowV4Incr gosnappi.PatternFlowIpv4DstCounter
	flowV6Incr gosnappi.PatternFlowIpv6DstCounter
}

// configureATE configures the interfaces and BGP on the ATE, with port2 advertising routes.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) *config {
	otg := ate.OTG()
	topo := otg.NewConfig(t)
	srcPort := topo.Ports().Add().SetName("port1")
	srcDev := topo.Devices().Add().SetName(ateSrc.Name)
	srcEth := srcDev.Ethernets().Add().SetName(ateSrc.Name + ".Eth")
	srcEth.SetPortName(srcPort.Name()).SetMac(ateSrc.MAC)
	srcIpv4 := srcEth.Ipv4Addresses().Add().SetName(ateSrc.Name + ".IPv4")
	srcIpv4.SetAddress(ateSrc.IPv4).SetGateway(dutSrc.IPv4).SetPrefix(int32(ateSrc.IPv4Len))
	srcIpv6 := srcEth.Ipv6Addresses().Add().SetName(ateSrc.Name + ".IPv6")
	srcIpv6.SetAddress(ateSrc.IPv6).SetGateway(dutSrc.IPv6).SetPrefix(int32(ateSrc.IPv6Len))

	dstPort := topo.Ports().Add().SetName("port2")
	dstDev := topo.Devices().Add().SetName(ateDst.Name)
	dstEth := dstDev.Ethernets().Add().SetName(ateDst.Name + ".Eth")
	dstEth.SetPortName(dstPort.Name()).SetMac(ateDst.MAC)
	dstIpv4 := dstEth.Ipv4Addresses().Add().SetName(ateDst.Name + ".IPv4")
	dstIpv4.SetAddress(ateDst.IPv4).SetGateway(dutDst.IPv4).SetPrefix(int32(ateDst.IPv4Len))
	dstIpv6 := dstEth.Ipv6Addresses().Add().SetName(ateDst.Name + ".IPv6")
	dstIpv6.SetAddress(ateDst.IPv6).SetGateway(dutDst.IPv6).SetPrefix(int32(ateDst.IPv6Len))

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

	dstBgp4PeerRoutes := dstBgp4Peer.V4Routes().Add().SetName(ateDst.Name + ".BGP4.peer" + ".RR4")
	dstBgp4PeerRoutes.SetNextHopIpv4Address(dstIpv4.Address()).
		SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).
		SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)
	dstBgp4PeerRoutes.Addresses().Add().
		SetAddress(advertisedRoutesv4Net).
		SetPrefix(advertisedRoutesv4Prefix).
		SetCount(1)
	dstBgp6PeerRoutes := dstBgp6Peer.V6Routes().Add().SetName(ateDst.Name + ".BGP6.peer" + ".RR6")
	dstBgp6PeerRoutes.SetNextHopIpv6Address(dstIpv6.Address()).
		SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).
		SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL)
	dstBgp6PeerRoutes.Addresses().Add().
		SetAddress(advertisedRoutesv6Net).
		SetPrefix(advertisedRoutesv6Prefix).
		SetCount(1)

	// ATE Traffic Configuration
	t.Logf("TestBGP:start ate Traffic config")
	//  BGP V4 Traffic
	flowipv4 := topo.Flows().Add().SetName("IPv4")
	flowipv4.Metrics().SetEnable(true)
	flowipv4.TxRx().Device().
		SetTxNames([]string{srcIpv4.Name()}).
		SetRxNames([]string{dstBgp4PeerRoutes.Name()})
	flowipv4.Size().SetFixed(512)
	flowipv4.Duration().SetChoice("continuous")
	e1 := flowipv4.Packet().Add().Ethernet()
	e1.Src().SetValue(srcEth.Mac())
	v4 := flowipv4.Packet().Add().Ipv4()
	v4.Src().SetValue(ipv4SrcTraffic)
	v4DstIncrement := v4.Dst().Increment().SetStart(advertisedRoutesv4Net).SetCount(prefixLimit)

	// BGP IP V6 traffic
	flowipv6 := topo.Flows().Add().SetName("IPv6")
	flowipv6.Metrics().SetEnable(true)
	flowipv6.TxRx().Device().
		SetTxNames([]string{srcIpv6.Name()}).
		SetRxNames([]string{dstBgp6PeerRoutes.Name()})
	flowipv6.Size().SetFixed(512)
	flowipv6.Duration().SetChoice("continuous")
	e2 := flowipv6.Packet().Add().Ethernet()
	e2.Src().SetValue(srcEth.Mac())
	v6 := flowipv6.Packet().Add().Ipv6()
	v6.Src().SetValue(ipv6SrcTraffic)
	v6DstIncrement := v6.Dst().Increment().SetStart(advertisedRoutesv6Net).SetCount(prefixLimit)

	t.Logf("Pushing config to ATE and starting protocols...")
	otg.PushConfig(t, topo)
	otg.StartProtocols(t)

	return &config{topo, dstBgp4PeerRoutes, dstBgp6PeerRoutes, v4DstIncrement, v6DstIncrement}
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

func (tc *testCase) verifyNoPacketLoss(t *testing.T, ate *ondatra.ATEDevice, conf gosnappi.Config) {
	captureTrafficStats(t, ate, conf)
	otg := ate.OTG()
	otgutils.LogFlowMetrics(t, otg, conf)
	for _, flow := range conf.Flows().Items() {
		recvMetric := gnmi.Get(t, otg, gnmi.OTG().Flow(flow.Name()).State())
		txPackets := recvMetric.GetCounters().GetOutPkts()
		rxPackets := recvMetric.GetCounters().GetInPkts()
		lostPackets := txPackets - rxPackets
		lossPct := lostPackets * 100 / txPackets
		if lossPct > lossTolerance {
			t.Errorf("Traffic Loss Pct for Flow %s: got %v, want 0", flow.Name(), lossPct)
		} else {
			t.Logf("Traffic Test Passed! Got %v loss", lossPct)
		}
	}
}

func (tc *testCase) verifyPacketLoss(t *testing.T, ate *ondatra.ATEDevice, conf gosnappi.Config) {
	captureTrafficStats(t, ate, conf)
	otg := ate.OTG()
	otgutils.LogFlowMetrics(t, otg, conf)
	for _, flow := range conf.Flows().Items() {
		recvMetric := gnmi.Get(t, otg, gnmi.OTG().Flow(flow.Name()).State())
		txPackets := recvMetric.GetCounters().GetOutPkts()
		rxPackets := recvMetric.GetCounters().GetInPkts()
		lostPackets := txPackets - rxPackets
		lossPct := lostPackets * 100 / txPackets
		if lossPct > (100-lossTolerance) && lossPct <= 100 {
			t.Logf("Traffic Test Passed! Loss seen as expected: got %v, want 100%% ", lossPct)
		} else {
			t.Errorf("Traffic %s is expected to fail: got %v, want 100%% failure", flow.Name(), lossPct)
		}
	}
}

func captureTrafficStats(t *testing.T, ate *ondatra.ATEDevice, conf gosnappi.Config) {
	otg := ate.OTG()
	otgutils.LogPortMetrics(t, otg, conf)
	ap := ate.Port(t, "port1")
	aic1 := gnmi.OTG().Port(ap.ID()).Counters()
	sentPkts := gnmi.Get(t, otg, aic1.OutFrames().State())
	fptest.LogQuery(t, "ate:port1 counters", aic1.State(), gnmi.Get(t, otg, aic1.State()))

	op := ate.Port(t, "port2")
	aic2 := gnmi.OTG().Port(op.ID()).Counters()
	rxPkts := gnmi.Get(t, otg, aic2.InFrames().State())
	fptest.LogQuery(t, "ate:port2 counters", aic2.State(), gnmi.Get(t, otg, aic2.State()))
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

func sendTraffic(t *testing.T, ate *ondatra.ATEDevice) {
	otg := ate.OTG()
	t.Log("Starting traffic")
	otg.StartTraffic(t)
	time.Sleep(trafficDuration)
	otg.StopTraffic(t)
	t.Log("Traffic stopped")
}

func configureBGPRoutes(t *testing.T, configElement *config, routeCount uint32) {

	ate := ondatra.ATE(t, "ate")
	otg := ate.OTG()

	// Modifying the OTG BGP routes configuration
	configElement.bgpv4RR.Addresses().Clear()
	configElement.bgpv4RR.Addresses().Add().
		SetAddress(advertisedRoutesv4Net).
		SetPrefix(advertisedRoutesv4Prefix).
		SetCount(int32(routeCount))

	configElement.bgpv6RR.Addresses().Clear()
	configElement.bgpv6RR.Addresses().Add().
		SetAddress(advertisedRoutesv6Net).
		SetPrefix(advertisedRoutesv6Prefix).
		SetCount(int32(routeCount))

	// Modifying the OTG flows
	configElement.flowV4Incr.SetCount(int32(routeCount))
	configElement.flowV6Incr.SetCount(int32(routeCount))

	otg.PushConfig(t, configElement.topo)
	otg.StartProtocols(t)

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
	configureBGPRoutes(t, conf, tc.numRoutes)
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
	sendTraffic(t, ate)
	if tc.wantNoPacketLoss {
		t.Run("verifyNoPacketLoss", func(t *testing.T) {
			tc.verifyNoPacketLoss(t, ate, conf.topo)
		})
	} else {
		t.Run("verifyPacketLoss", func(t *testing.T) {
			tc.verifyPacketLoss(t, ate, conf.topo)
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
