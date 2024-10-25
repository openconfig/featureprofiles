// Copyright 2023 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package bgp_keepalive_and_holdtimer_configuration_test

import (
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// The testbed consists of ate:port1 <--eBGP-> dut:port1 and dut:port2 <--eBGP--> ate:port2
// The first pair is called the "source" pair, and the second the "destination" pair.
//   - Source: ate:port1 -> dut:port1 subnet 192.0.2.0/30 2001:db8::192:0:2:0/126
//   - Destination: dut:port2 -> ate:port2 subnet 192.0.2.4/30 2001:db8::192:0:2:4/126
//
// Modify BGP timer values on peers to 10/30 and to 5/15.
//   - Verify that the sessions are established after soft reset.
//   - Validate BGP session state and updated timers on DUT.
const (
	ipv4PrefixLen  = 30
	ipv6PrefixLen  = 126
	ipv4SrcTraffic = "192.0.2.2"
	ipv6SrcTraffic = "2001:db8::192:0:2:2"
)

var (
	dutSrc = attrs.Attributes{
		Desc:    "DUT to ATE source",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::192:0:2:1",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
	ateSrc = attrs.Attributes{
		Name:    "ateSrc",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::192:0:2:2",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
	dutDst = attrs.Attributes{
		Desc:    "DUT to ATE destination",
		IPv4:    "192.0.2.5",
		IPv6:    "2001:db8::192:0:2:5",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
	ateDst = attrs.Attributes{
		Name:    "atedst",
		MAC:     "02:00:02:01:01:01",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::192:0:2:6",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
)

type bgpTimers struct {
	keepAliveTimer uint16
	holdTimer      uint16
}
type bgpNeighbor struct {
	localAs, peerAs, pfxLimit uint32
	neighborip                string
	isV4                      bool
}
type bgpAttrs struct {
	rplName, advertisedRoutesv4Net, advertisedRoutesv6Net, peerGrpNamev4, peerGrpNamev6 string
	prefixLimit, dutAS, ateAS, advertisedRoutesv4Prefix, advertisedRoutesv6Prefix       uint32
	grRestartTime                                                                       uint16
}

var (
	bgpGlobalAttrs = bgpAttrs{
		rplName:       "ALLOW",
		grRestartTime: 60,
		prefixLimit:   200,
		dutAS:         64500,
		ateAS:         64501,
		peerGrpNamev4: "BGP-PEER-GROUP-V4",
		peerGrpNamev6: "BGP-PEER-GROUP-V6",
	}
	bgpRouteAttrs = bgpAttrs{
		advertisedRoutesv4Net:    "203.0.113.1",
		advertisedRoutesv6Net:    "2001:db8::203:0:113:1",
		advertisedRoutesv4Prefix: 32,
		advertisedRoutesv6Prefix: 128,
	}
)

// Static list of eBGP/iBGP neighbors formed with DUT.
var (
	bgpNbrs = []*bgpNeighbor{
		{localAs: bgpGlobalAttrs.dutAS, peerAs: bgpGlobalAttrs.ateAS, pfxLimit: bgpGlobalAttrs.prefixLimit, neighborip: ateSrc.IPv4, isV4: true},
		{localAs: bgpGlobalAttrs.dutAS, peerAs: bgpGlobalAttrs.ateAS, pfxLimit: bgpGlobalAttrs.prefixLimit, neighborip: ateSrc.IPv6, isV4: false},
		{localAs: bgpGlobalAttrs.dutAS, peerAs: bgpGlobalAttrs.ateAS, pfxLimit: bgpGlobalAttrs.prefixLimit, neighborip: ateDst.IPv4, isV4: true},
		{localAs: bgpGlobalAttrs.dutAS, peerAs: bgpGlobalAttrs.ateAS, pfxLimit: bgpGlobalAttrs.prefixLimit, neighborip: ateDst.IPv6, isV4: false},
	}
)

type config struct {
	topo       gosnappi.Config
	bgpv4RR    gosnappi.BgpV4RouteRange
	bgpv6RR    gosnappi.BgpV6RouteRange
	flowV4Incr gosnappi.PatternFlowIpv4DstCounter
	flowV6Incr gosnappi.PatternFlowIpv6DstCounter
}

// configureDUT configures all the interfaces on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	dc := gnmi.OC()
	i1 := dutSrc.NewOCInterface(dut.Port(t, "port1").Name(), dut)
	i1.Description = ygot.String(dutSrc.Desc)
	gnmi.Replace(t, dut, dc.Interface(i1.GetName()).Config(), i1)
	i2 := dutDst.NewOCInterface(dut.Port(t, "port2").Name(), dut)
	i2.Description = ygot.String(dutDst.Desc)
	gnmi.Replace(t, dut, dc.Interface(i2.GetName()).Config(), i2)
	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, dut.Port(t, "port1"))
		fptest.SetPortSpeed(t, dut.Port(t, "port2"))
	}
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, i1.GetName(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, i2.GetName(), deviations.DefaultNetworkInstance(dut), 0)
	}
}

// configureATE configures the interfaces and BGP on the ATE/OTG.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) *config {
	topo := gosnappi.NewConfig()
	p1 := ate.Port(t, "port1")
	ateSrc.AddToOTG(topo, p1, &dutSrc)
	p2 := ate.Port(t, "port2")
	ateDst.AddToOTG(topo, p2, &dutDst)
	srcDev := topo.Devices().Items()[0]
	srcEth := srcDev.Ethernets().Items()[0]
	srcIpv4 := srcEth.Ipv4Addresses().Items()[0]
	srcIpv6 := srcEth.Ipv6Addresses().Items()[0]
	dstDev := topo.Devices().Items()[1]
	dstEth := dstDev.Ethernets().Items()[0]
	dstIpv4 := dstEth.Ipv4Addresses().Items()[0]
	dstIpv6 := dstEth.Ipv6Addresses().Items()[0]
	// Setup ATE BGP route v4/v6 advertisement
	srcBgp := srcDev.Bgp().SetRouterId(srcIpv4.Address())
	srcBgp4Peer := srcBgp.Ipv4Interfaces().Add().SetIpv4Name(srcIpv4.Name()).Peers().Add().SetName(ateSrc.Name + ".BGP4.peer")
	srcBgp4Peer.SetPeerAddress(srcIpv4.Gateway()).SetAsNumber(uint32(bgpGlobalAttrs.ateAS)).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
	srcBgp6Peer := srcBgp.Ipv6Interfaces().Add().SetIpv6Name(srcIpv6.Name()).Peers().Add().SetName(ateSrc.Name + ".BGP6.peer")
	srcBgp6Peer.SetPeerAddress(srcIpv6.Gateway()).SetAsNumber(uint32(bgpGlobalAttrs.ateAS)).SetAsType(gosnappi.BgpV6PeerAsType.EBGP)
	dstBgp := dstDev.Bgp().SetRouterId(dstIpv4.Address())
	dstBgp4Peer := dstBgp.Ipv4Interfaces().Add().SetIpv4Name(dstIpv4.Name()).Peers().Add().SetName(ateDst.Name + ".BGP4.peer")
	dstBgp4Peer.SetPeerAddress(dstIpv4.Gateway()).SetAsNumber(uint32(bgpGlobalAttrs.ateAS)).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
	dstBgp6Peer := dstBgp.Ipv6Interfaces().Add().SetIpv6Name(dstIpv6.Name()).Peers().Add().SetName(ateDst.Name + ".BGP6.peer")
	dstBgp6Peer.SetPeerAddress(dstIpv6.Gateway()).SetAsNumber(uint32(bgpGlobalAttrs.ateAS)).SetAsType(gosnappi.BgpV6PeerAsType.EBGP)
	dstBgp4PeerRoutes := dstBgp4Peer.V4Routes().Add().SetName(ateDst.Name + ".BGP4.peer" + ".RR4")
	dstBgp4PeerRoutes.SetNextHopIpv4Address(dstIpv4.Address()).
		SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).
		SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)
	dstBgp4PeerRoutes.Addresses().Add().
		SetAddress(bgpRouteAttrs.advertisedRoutesv4Net).
		SetPrefix(uint32(bgpRouteAttrs.advertisedRoutesv4Prefix)).
		SetCount(1)
	dstBgp6PeerRoutes := dstBgp6Peer.V6Routes().Add().SetName(ateDst.Name + ".BGP6.peer" + ".RR6")
	dstBgp6PeerRoutes.SetNextHopIpv6Address(dstIpv6.Address()).
		SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).
		SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL)
	dstBgp6PeerRoutes.Addresses().Add().
		SetAddress(bgpRouteAttrs.advertisedRoutesv6Net).
		SetPrefix(uint32(bgpRouteAttrs.advertisedRoutesv6Prefix)).
		SetCount(1)
	v4DstIncrement, v6DstIncrement := ateFlowConfig(t, topo, srcEth, srcIpv4, srcIpv6, dstBgp4PeerRoutes, dstBgp6PeerRoutes)
	t.Logf("Pushing config to ATE and starting protocols...")
	ate.OTG().PushConfig(t, topo)
	ate.OTG().StartProtocols(t)
	return &config{topo, dstBgp4PeerRoutes, dstBgp6PeerRoutes, v4DstIncrement, v6DstIncrement}
}
func ateFlowConfig(t *testing.T, topo gosnappi.Config, srcEth gosnappi.DeviceEthernet, srcIpv4 gosnappi.DeviceIpv4, srcIpv6 gosnappi.DeviceIpv6, dstBgp4PeerRoutes gosnappi.BgpV4RouteRange, dstBgp6PeerRoutes gosnappi.BgpV6RouteRange) (gosnappi.PatternFlowIpv4DstCounter, gosnappi.PatternFlowIpv6DstCounter) {
	// ATE Traffic Configuration
	t.Logf("TestBGP:start ate Traffic config")
	//  BGP V4 Traffic
	flowipv4 := topo.Flows().Add().SetName("IPv4")
	flowipv4.Metrics().SetEnable(true)
	flowipv4.TxRx().Device().
		SetTxNames([]string{srcIpv4.Name()}).
		SetRxNames([]string{dstBgp4PeerRoutes.Name()})
	flowipv4.Size().SetFixed(512)
	flowipv4.Duration().Continuous()
	e1 := flowipv4.Packet().Add().Ethernet()
	e1.Src().SetValue(srcEth.Mac())
	v4 := flowipv4.Packet().Add().Ipv4()
	v4.Src().SetValue(ipv4SrcTraffic)
	v4DstIncrement := v4.Dst().Increment().SetStart(bgpRouteAttrs.advertisedRoutesv4Net).SetCount(uint32(bgpGlobalAttrs.prefixLimit))
	// BGP IP V6 traffic
	flowipv6 := topo.Flows().Add().SetName("IPv6")
	flowipv6.Metrics().SetEnable(true)
	flowipv6.TxRx().Device().
		SetTxNames([]string{srcIpv6.Name()}).
		SetRxNames([]string{dstBgp6PeerRoutes.Name()})
	flowipv6.Size().SetFixed(512)
	flowipv6.Duration().Continuous()
	e2 := flowipv6.Packet().Add().Ethernet()
	e2.Src().SetValue(srcEth.Mac())
	v6 := flowipv6.Packet().Add().Ipv6()
	v6.Src().SetValue(ipv6SrcTraffic)
	v6DstIncrement := v6.Dst().Increment().SetStart(bgpRouteAttrs.advertisedRoutesv6Net).SetCount(uint32(bgpGlobalAttrs.prefixLimit))
	return v4DstIncrement, v6DstIncrement
}

// bgpCreateNbr creates a BGP object with neighbors pointing to ateSrc and ateDst, optionally with
// a peer group policy.
func bgpCreateNbr(dut *ondatra.DUTDevice) *oc.NetworkInstance_Protocol {
	d := &oc.Root{}
	ni1 := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niProto := ni1.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := niProto.GetOrCreateBgp()
	global := bgp.GetOrCreateGlobal()
	global.As = ygot.Uint32(uint32(bgpGlobalAttrs.dutAS))
	global.RouterId = ygot.String(dutSrc.IPv4)
	global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
	global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)
	// Note: we have to define the peer group even if we aren't setting any policy because it's
	// invalid OC for the neighbor to be part of a peer group that doesn't exist.
	pgv4 := bgp.GetOrCreatePeerGroup(bgpGlobalAttrs.peerGrpNamev4)
	pgv4.PeerAs = ygot.Uint32(uint32(bgpGlobalAttrs.ateAS))
	pgv4.PeerGroupName = ygot.String(bgpGlobalAttrs.peerGrpNamev4)
	pgv6 := bgp.GetOrCreatePeerGroup(bgpGlobalAttrs.peerGrpNamev6)
	pgv6.PeerAs = ygot.Uint32(uint32(bgpGlobalAttrs.ateAS))
	pgv6.PeerGroupName = ygot.String(bgpGlobalAttrs.peerGrpNamev6)
	for _, nbr := range bgpNbrs {
		if nbr.isV4 {
			nv4 := bgp.GetOrCreateNeighbor(nbr.neighborip)
			nv4.PeerAs = ygot.Uint32(nbr.peerAs)
			nv4.Enabled = ygot.Bool(true)
			nv4.PeerGroup = ygot.String(bgpGlobalAttrs.peerGrpNamev4)
			nv4.GetOrCreateTimers().RestartTime = ygot.Uint16(bgpGlobalAttrs.grRestartTime)
			afisafi := nv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
			afisafi.Enabled = ygot.Bool(true)
			if deviations.BGPExplicitPrefixLimitReceived(dut) {
				prefixLimit := afisafi.GetOrCreateIpv4Unicast().GetOrCreatePrefixLimitReceived()
				prefixLimit.MaxPrefixes = ygot.Uint32(uint32(nbr.pfxLimit))
			} else {
				prefixLimit := afisafi.GetOrCreateIpv4Unicast().GetOrCreatePrefixLimit()
				prefixLimit.MaxPrefixes = ygot.Uint32(uint32(nbr.pfxLimit))
			}
			if deviations.RoutePolicyUnderAFIUnsupported(dut) {
				rpl := pgv4.GetOrCreateApplyPolicy()
				rpl.ImportPolicy = []string{bgpGlobalAttrs.rplName}
				rpl.ExportPolicy = []string{bgpGlobalAttrs.rplName}
			} else {
				pgafv4 := pgv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
				pgafv4.Enabled = ygot.Bool(true)
				rpl := pgafv4.GetOrCreateApplyPolicy()
				rpl.ImportPolicy = []string{bgpGlobalAttrs.rplName}
				rpl.ExportPolicy = []string{bgpGlobalAttrs.rplName}

			}
		} else {
			nv6 := bgp.GetOrCreateNeighbor(nbr.neighborip)
			nv6.PeerAs = ygot.Uint32(nbr.peerAs)
			nv6.Enabled = ygot.Bool(true)
			nv6.PeerGroup = ygot.String(bgpGlobalAttrs.peerGrpNamev6)
			nv6.GetOrCreateTimers().RestartTime = ygot.Uint16(bgpGlobalAttrs.grRestartTime)
			afisafi6 := nv6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
			afisafi6.Enabled = ygot.Bool(true)
			if deviations.BGPExplicitPrefixLimitReceived(dut) {
				prefixLimit := afisafi6.GetOrCreateIpv6Unicast().GetOrCreatePrefixLimitReceived()
				prefixLimit.MaxPrefixes = ygot.Uint32(uint32(nbr.pfxLimit))
			} else {
				prefixLimit := afisafi6.GetOrCreateIpv6Unicast().GetOrCreatePrefixLimit()
				prefixLimit.MaxPrefixes = ygot.Uint32(uint32(nbr.pfxLimit))
			}
			if deviations.RoutePolicyUnderAFIUnsupported(dut) {
				rpl := pgv6.GetOrCreateApplyPolicy()
				rpl.ImportPolicy = []string{bgpGlobalAttrs.rplName}
				rpl.ExportPolicy = []string{bgpGlobalAttrs.rplName}
			} else {
				pgafv6 := pgv6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
				pgafv6.Enabled = ygot.Bool(true)
				rpl := pgafv6.GetOrCreateApplyPolicy()
				rpl.ImportPolicy = []string{bgpGlobalAttrs.rplName}
				rpl.ExportPolicy = []string{bgpGlobalAttrs.rplName}

			}
		}
	}
	return niProto
}

// bgpTimersConfig sets the right config for BGP timers.
func (tc *testCase) bgpTimersConfig(t *testing.T, dut *ondatra.DUTDevice) *oc.NetworkInstance_Protocol {
	d := &oc.Root{}
	ni1 := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niProto := ni1.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := niProto.GetOrCreateBgp()
	for _, nbr := range bgpNbrs {
		bgpNeighbor := bgp.GetOrCreateNeighbor(nbr.neighborip)
		bgpNeighbor.GetOrCreateTimers().SetKeepaliveInterval(tc.bgpTimers.keepAliveTimer)
		bgpNeighbor.GetOrCreateTimers().SetHoldTime(tc.bgpTimers.holdTimer)
	}
	return niProto
}

// waitForBGPSession waits for the BGP session state to become established.
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
func (tc *testCase) verifyBGPSessionState(t *testing.T, dut *ondatra.DUTDevice) {
	t.Log("Waiting for BGPv4 neighbor to establish...")
	waitForBGPSession(t, dut, tc.wantEstablished)
}

// configureBGPRoutes configure BGP routes by modifying OTG BGP configuration and starting protocol.
func configureBGPRoutes(t *testing.T, configElement *config, routeCount uint32) {
	ate := ondatra.ATE(t, "ate")
	otg := ate.OTG()
	// Modifying the OTG BGP routes configuration
	configElement.bgpv4RR.Addresses().Clear()
	configElement.bgpv4RR.Addresses().Add().
		SetAddress(bgpRouteAttrs.advertisedRoutesv4Net).
		SetPrefix(bgpRouteAttrs.advertisedRoutesv4Prefix).
		SetCount(routeCount)
	configElement.bgpv6RR.Addresses().Clear()
	configElement.bgpv6RR.Addresses().Add().
		SetAddress(bgpRouteAttrs.advertisedRoutesv6Net).
		SetPrefix(bgpRouteAttrs.advertisedRoutesv6Prefix).
		SetCount(routeCount)
	// Modifying the OTG flows
	configElement.flowV4Incr.SetCount(routeCount)
	configElement.flowV6Incr.SetCount(routeCount)
	otg.PushConfig(t, configElement.topo)
	otg.StartProtocols(t)
}

// verifyPortsUp confirms the status of ports to be in UP state.
func (tc *testCase) verifyPortsUp(t *testing.T, dev *ondatra.Device) {
	for _, p := range dev.Ports() {
		portStatus := gnmi.Get(t, dev, gnmi.OC().Interface(p.Name()).OperStatus().State())
		if want := oc.Interface_OperStatus_UP; portStatus != want {
			t.Errorf("%s Status: got %v, want %v", p, portStatus, want)
		}
	}
}

// verifyBGPTimers verify BGP timers like keepalive timer and hold timer.
func (tc *testCase) verifyBGPTimers(t *testing.T, dut *ondatra.DUTDevice) {
	t.Log("Verifying BGP timers")
	bgpPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	for _, nbr := range bgpNbrs {
		timerPath := bgpPath.Neighbor(nbr.neighborip).Timers()
		gotBgptimers := bgpTimers{
			keepAliveTimer: gnmi.Get(t, dut, timerPath.KeepaliveInterval().State()),
			holdTimer:      gnmi.Get(t, dut, timerPath.HoldTime().State()),
		}
		if want := tc.bgpTimers; gotBgptimers != want {
			t.Errorf("BGP timers: got %v, want %v", gotBgptimers, want)
		}
	}
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

type testCase struct {
	desc            string
	name            string
	numRoutes       int32
	wantEstablished bool
	bgpTimers       bgpTimers
}

func (tc *testCase) run(t *testing.T, conf *config, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice) {
	t.Log(tc.desc)
	configureBGPRoutes(t, conf, uint32(tc.numRoutes))
	// Configure BGP Timers
	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	t.Logf("Start DUT BGP Config")
	dutBgpTimerConf := tc.bgpTimersConfig(t, dut)
	gnmi.Update(t, dut, dutConfPath.Config(), dutBgpTimerConf)
	fptest.LogQuery(t, "Updated DUT BGP Config", dutConfPath.Config(), gnmi.Get(t, dut, dutConfPath.Config()))
	// Verify Port Status
	t.Log(" Verifying port status")
	t.Run("verifyPortsUp", func(t *testing.T) {
		tc.verifyPortsUp(t, dut.Device)
	})
	// Verify BGP Session State
	t.Run("verifyBGPSessionState", func(t *testing.T) {
		tc.verifyBGPSessionState(t, dut)
	})
	// Verify BGP Configuration timers
	t.Run("verifyBGPTimers", func(t *testing.T) {
		tc.verifyBGPTimers(t, dut)
	})
}

func TestBgpKeepAliveHoldTimerConfiguration(t *testing.T) {
	defaultTimer := bgpTimers{
		keepAliveTimer: 30,
		holdTimer:      90,
	}
	tenThirty := bgpTimers{
		keepAliveTimer: 10,
		holdTimer:      30,
	}
	fiveFifteen := bgpTimers{
		keepAliveTimer: 5,
		holdTimer:      15,
	}
	cases := []testCase{{
		name:            "BGP Timers Default Configuration",
		desc:            "BGP configuration with default timers",
		numRoutes:       int32(bgpGlobalAttrs.prefixLimit),
		wantEstablished: true,
		bgpTimers:       defaultTimer,
	}, {
		name:            "BGP Timers Updated Configuration",
		desc:            "BGP configuration with values of 10 and 30",
		numRoutes:       int32(bgpGlobalAttrs.prefixLimit),
		wantEstablished: true,
		bgpTimers:       tenThirty,
	}, {
		name:            "BGP Timers Updated Configuration",
		desc:            "BGP configuration with values of 5 and 15",
		numRoutes:       int32(bgpGlobalAttrs.prefixLimit),
		wantEstablished: true,
		bgpTimers:       fiveFifteen,
	}}
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	// DUT Configuration
	t.Log("Start DUT interface Config")
	configureDUT(t, dut)
	t.Log("Configure RPL")
	configureRoutePolicy(t, dut, bgpGlobalAttrs.rplName, oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	t.Logf("Start DUT BGP Config")
	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	dutConf := bgpCreateNbr(dut)
	gnmi.Replace(t, dut, dutConfPath.Config(), dutConf)
	fptest.LogQuery(t, "DUT BGP Config", dutConfPath.Config(), gnmi.Get(t, dut, dutConfPath.Config()))
	// ATE Configuration.
	t.Log("Start ATE Config")
	otgConfig := configureATE(t, ate)
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			tc.run(t, otgConfig, dut, ate)
		})
	}
}
