// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package admin_distance_test

import (
	"math"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/isissession"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygot/ygot"
)

const (
	prefixV4Len    = uint32(24)
	prefixV6Len    = uint32(64)
	v4Network      = "192.168.10.0"
	v6Network      = "2024:db8:64:64::"
	pathID         = 1
	prefixesCount  = 1
	bgpName        = "BGP"
	dutAS          = uint32(64656)
	ateAS          = uint32(64657)
	peerGrpNamev4  = "BGP-PEER-GROUP-V4"
	peerGrpNamev6  = "BGP-PEER-GROUP-V6"
	ateSysID       = "640000000001"
	ateAreaAddress = "49.0002"
	lossTolerance  = 1
)

var (
	dutPort3 = &attrs.Attributes{
		Desc:    "DUT to ATE link",
		IPv4:    "192.0.2.9",
		IPv6:    "2001:db8::9",
		IPv4Len: 30,
		IPv6Len: 126,
	}

	atePort3 = &attrs.Attributes{
		Name:    "port3",
		Desc:    "ATE to DUT link",
		MAC:     "02:12:01:00:00:01",
		IPv4:    "192.0.2.10",
		IPv6:    "2001:db8::a",
		IPv4Len: 30,
		IPv6Len: 126,
	}

	advertisedIPv4 ipAddr = ipAddr{address: v4Network, prefix: prefixV4Len}
	advertisedIPv6 ipAddr = ipAddr{address: v6Network, prefix: prefixV6Len}
)

type ipAddr struct {
	address string
	prefix  uint32
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// isis on port1 and eBGP on port2
func TestAdminDistance(t *testing.T) {
	ts := isissession.MustNew(t).WithISIS()
	configurePort3(t, ts)
	advertisePrefixFromISISPort(t, ts)
	t.Run("ISIS Setup", func(t *testing.T) {
		ts.PushAndStart(t)
		ts.MustAdjacency(t)
	})

	setupEBGPAndAdvertise(t, ts)
	t.Run("BGP Setup", func(t *testing.T) {
		t.Log("Verify DUT BGP sessions up")
		cfgplugins.VerifyDUTBGPEstablished(t, ts.DUT)

		t.Log("Verify OTG BGP sessions up")
		cfgplugins.VerifyOTGBGPEstablished(t, ts.ATE)
	})

	testCases := []struct {
		desc string
		rd   uint8
		port string
		bgp  string
	}{
		{
			desc: "EBGP RD value 5",
			rd:   5,
			port: "port2",
			bgp:  "eBGP",
		},
		{
			desc: "EBGP RD value 250",
			rd:   250,
			port: "port1",
			bgp:  "eBGP",
		},
		{
			desc: "IBGP RD value 5",
			rd:   5,
			port: "port2",
			bgp:  "iBGP",
		},
		{
			desc: "IBGP RD value 250",
			rd:   250,
			port: "port1",
			bgp:  "iBGP",
		},
	}

	bgpPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(ts.DUT)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).Bgp()
	t.Run("Test Admin Distance", func(t *testing.T) {
		for _, tc := range testCases {
			t.Run(tc.desc, func(t *testing.T) {
				if tc.bgp == "iBGP" {
					changeProtocolToIBGP(t, ts)
					gnmi.Update(t, ts.DUT, bgpPath.Global().DefaultRouteDistance().InternalRouteDistance().Config(), tc.rd)
					gnmi.Await(t, ts.DUT, bgpPath.Global().DefaultRouteDistance().InternalRouteDistance().State(), 30*time.Second, tc.rd)
				} else {
					gnmi.Update(t, ts.DUT, bgpPath.Global().DefaultRouteDistance().ExternalRouteDistance().Config(), tc.rd)
					gnmi.Await(t, ts.DUT, bgpPath.Global().DefaultRouteDistance().ExternalRouteDistance().State(), 30*time.Second, tc.rd)
				}

				ts.ATETop.Flows().Clear()
				createFlow(t, ts.ATETop, ts.ATE.OTG(), false)
				createFlow(t, ts.ATETop, ts.ATE.OTG(), true)
				ts.ATE.OTG().PushConfig(t, ts.ATETop)
				ts.ATE.OTG().StartProtocols(t)
				otgutils.WaitForARP(t, ts.ATE.OTG(), ts.ATETop, "IPv4")
				otgutils.WaitForARP(t, ts.ATE.OTG(), ts.ATETop, "IPv6")

				ts.ATE.OTG().StartTraffic(t)
				// added 30 seconds for sleep for traffic flow
				time.Sleep(30 * time.Second)
				ts.ATE.OTG().StopTraffic(t)

				otgutils.LogFlowMetrics(t, ts.ATE.OTG(), ts.ATETop)
				otgutils.LogPortMetrics(t, ts.ATE.OTG(), ts.ATETop)

				txPkts := gnmi.Get[uint64](t, ts.ATE.OTG(), gnmi.OTG().Port(ts.ATE.Port(t, "port3").ID()).Counters().OutFrames().State())
				rxPkts := gnmi.Get[uint64](t, ts.ATE.OTG(), gnmi.OTG().Port(ts.ATE.Port(t, tc.port).ID()).Counters().InFrames().State())
				if got := (math.Abs(float64(txPkts)-float64(rxPkts)) * 100) / float64(txPkts); got > lossTolerance {
					t.Errorf("Packet loss percentage for flow: got %v, want %v", got, lossTolerance)
				}
			})
		}
	})
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

func changeProtocolToIBGP(t *testing.T, ts *isissession.TestSession) {
	root := &oc.Root{}
	dni := root.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(ts.DUT))
	bgpP := dni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName)
	bgpP.GetOrCreateBgp().GetOrCreateNeighbor(isissession.ATETrafficAttrs.IPv4).SetPeerAs(dutAS)
	bgpP.GetOrCreateBgp().GetOrCreateNeighbor(isissession.ATETrafficAttrs.IPv6).SetPeerAs(dutAS)
	gnmi.Update(t, ts.DUT, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(ts.DUT)).Config(), dni)

	bgp4Peer := ts.ATEIntf2.Bgp().Ipv4Interfaces().Items()[0].Peers().Items()[0]
	bgp4Peer.SetAsNumber(dutAS).SetAsType(gosnappi.BgpV4PeerAsType.IBGP)
	bgp6Peer := ts.ATEIntf2.Bgp().Ipv6Interfaces().Items()[0].Peers().Items()[0]
	bgp6Peer.SetAsNumber(dutAS).SetAsType(gosnappi.BgpV6PeerAsType.IBGP)
}

func configurePort3(t *testing.T, ts *isissession.TestSession) {
	t.Helper()
	dc := gnmi.OC()

	dp3 := ts.DUT.Port(t, "port3")
	i3 := dutPort3.ConfigOCInterface(ts.DUTConf.GetOrCreateInterface(dp3.Name()), ts.DUT)
	gnmi.Replace(t, ts.DUT, dc.Interface(i3.GetName()).Config(), i3)
	if deviations.ExplicitInterfaceInDefaultVRF(ts.DUT) {
		fptest.AssignToNetworkInstance(t, ts.DUT, dp3.Name(), deviations.DefaultNetworkInstance(ts.DUT), 0)
	}
	if deviations.ExplicitPortSpeed(ts.DUT) {
		fptest.SetPortSpeed(t, dp3)
	}
	ap3 := ts.ATE.Port(t, "port3")
	atePort3.AddToOTG(ts.ATETop, ap3, dutPort3)
}

func createFlow(t *testing.T, config gosnappi.Config, otg *otg.OTG, isV6 bool) {
	t.Helper()

	flowName := "flowV4"
	if isV6 {
		flowName = "flowV6"
	}
	flow := config.Flows().Add().SetName(flowName)
	flow.Metrics().SetEnable(true)
	if isV6 {
		flow.TxRx().Device().
			SetTxNames([]string{"port3.IPv6"}).
			SetRxNames([]string{"port1.IPv6", "port2.IPv6"})
	} else {
		flow.TxRx().Device().
			SetTxNames([]string{"port3.IPv4"}).
			SetRxNames([]string{"port1.IPv4", "port2.IPv4"})
	}
	flow.Size().SetFixed(512)
	flow.Rate().SetPps(100)
	flow.Duration().Continuous()
	ethHeader := flow.Packet().Add().Ethernet()
	ethHeader.Src().SetValue(atePort3.MAC)
	if isV6 {
		ipHeader := flow.Packet().Add().Ipv6()
		ipHeader.Src().SetValue(atePort3.IPv6)
		ipHeader.Dst().SetValue(advertisedIPv6.address)
	} else {
		ipHeader := flow.Packet().Add().Ipv4()
		ipHeader.Src().SetValue(atePort3.IPv4)
		ipHeader.Dst().SetValue(advertisedIPv4.address)
	}
}

// setupEBGPAndAdvertise setups eBGP on DUT port1 and ATE port1
func setupEBGPAndAdvertise(t *testing.T, ts *isissession.TestSession) {
	t.Helper()

	// setup eBGP on DUT port1 and iBGP on port2
	root := &oc.Root{}
	dni := root.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(ts.DUT))
	dni.SetType(oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)

	bgpP := dni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName)
	bgpP.SetEnabled(true)
	bgp := bgpP.GetOrCreateBgp()

	g := bgp.GetOrCreateGlobal()
	g.SetAs(dutAS)
	g.SetRouterId(isissession.DUTTrafficAttrs.IPv4)
	g.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
	g.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)

	pgv4 := bgp.GetOrCreatePeerGroup(peerGrpNamev4)
	pgv4.PeerAs = ygot.Uint32(dutAS)
	pgv4.PeerGroupName = ygot.String(peerGrpNamev4)
	pgv6 := bgp.GetOrCreatePeerGroup(peerGrpNamev6)
	pgv6.PeerAs = ygot.Uint32(dutAS)
	pgv6.PeerGroupName = ygot.String(peerGrpNamev6)

	nV4 := bgp.GetOrCreateNeighbor(isissession.ATETrafficAttrs.IPv4)
	nV4.SetPeerAs(ateAS)
	nV4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
	nV4.PeerGroup = ygot.String(peerGrpNamev4)
	nV6 := bgp.GetOrCreateNeighbor(isissession.ATETrafficAttrs.IPv6)
	nV6.SetPeerAs(ateAS)
	nV6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)
	nV6.PeerGroup = ygot.String(peerGrpNamev6)

	// Configure Import Allow-All policy
	configureRoutePolicy(t, ts.DUT, "ALLOW", oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)
	pg1af4 := pgv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
	pg1af4.Enabled = ygot.Bool(true)

	pg1rpl4 := pg1af4.GetOrCreateApplyPolicy()
	pg1rpl4.SetImportPolicy([]string{"ALLOW"})

	pg1af6 := pgv6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
	pg1af6.Enabled = ygot.Bool(true)
	pg1rpl6 := pg1af6.GetOrCreateApplyPolicy()
	pg1rpl6.SetImportPolicy([]string{"ALLOW"})

	gnmi.Update(t, ts.DUT, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(ts.DUT)).Config(), dni)

	// setup eBGP on ATE port1
	devBGP := ts.ATEIntf2.Bgp().SetRouterId(isissession.ATETrafficAttrs.IPv4)

	ipv4 := ts.ATEIntf2.Ethernets().Items()[0].Ipv4Addresses().Items()[0]
	bgp4Peer := devBGP.Ipv4Interfaces().Add().SetIpv4Name(ipv4.Name()).Peers().Add().SetName(ts.ATEIntf2.Name() + ".BGP4.peer")
	bgp4Peer.SetPeerAddress(isissession.DUTTrafficAttrs.IPv4).SetAsNumber(ateAS).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)

	ipv6 := ts.ATEIntf2.Ethernets().Items()[0].Ipv6Addresses().Items()[0]
	bgp6Peer := devBGP.Ipv6Interfaces().Add().SetIpv6Name(ipv6.Name()).Peers().Add().SetName(ts.ATEIntf2.Name() + ".BGP6.peer")
	bgp6Peer.SetPeerAddress(isissession.DUTTrafficAttrs.IPv6).SetAsNumber(ateAS).SetAsType(gosnappi.BgpV6PeerAsType.EBGP)

	// configure emulated IPv4 and IPv6 networks
	netv4 := bgp4Peer.V4Routes().Add().SetName("v4-bgpNet-dev")
	netv4.Addresses().Add().SetAddress(advertisedIPv4.address).SetPrefix(advertisedIPv4.prefix).SetCount(uint32(prefixesCount))

	netv6 := bgp6Peer.V6Routes().Add().SetName("v6-bgpNet-dev")
	netv6.Addresses().Add().SetAddress(advertisedIPv6.address).SetPrefix(advertisedIPv6.prefix).SetCount(uint32(prefixesCount))

	ts.ATE.OTG().PushConfig(t, ts.ATETop)
	ts.ATE.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, ts.ATE.OTG(), ts.ATETop, "IPv4")
	otgutils.WaitForARP(t, ts.ATE.OTG(), ts.ATETop, "IPv6")
}

func advertisePrefixFromISISPort(t *testing.T, ts *isissession.TestSession) {
	netv4 := ts.ATEIntf1.Isis().V4Routes().Add().SetName("netv4").SetLinkMetric(10).SetOriginType(gosnappi.IsisV4RouteRangeOriginType.EXTERNAL)
	netv4.Addresses().Add().SetAddress(advertisedIPv4.address).SetPrefix(advertisedIPv4.prefix).SetCount(uint32(prefixesCount))

	netv6 := ts.ATEIntf1.Isis().V6Routes().Add().SetName("netv6").SetLinkMetric(10).SetOriginType(gosnappi.IsisV6RouteRangeOriginType.EXTERNAL)
	netv6.Addresses().Add().SetAddress(advertisedIPv6.address).SetPrefix(advertisedIPv6.prefix).SetCount(uint32(prefixesCount))
}
