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

package management_ha_test

import (
	"fmt"
	"math"
	"sort"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	prefixesStart = "2001:db8:1::1"
	prefixP6Len   = 128
	prefixesCount = 1
	pathID        = 1
	defaultRoute  = "0:0:0:0:0:0:0:0"
	dutAS         = 65501
)

var (
	dutlo0Attrs = attrs.Attributes{
		Desc:    "Loopback ip",
		IPv4:    "203.0.113.1",
		IPv6:    "2001:db8::203:0:113:1",
		IPv4Len: 32,
		IPv6Len: 128,
	}

	mgmtVRF  = "mgmtvrf1"
	bgpPorts = []string{"port1", "port2"}

	setNoPeerGroup = true
	lossTolerance  = float64(1)
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestManagementHA1(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")
	p3 := dut.Port(t, "port3")
	p4 := dut.Port(t, "port4")
	loopbackIntfName := netutil.LoopbackInterface(t, dut, 1)

	addInterfacesToVRF(t, dut, mgmtVRF, []string{p1.Name(), p2.Name(), p3.Name(), p4.Name(), loopbackIntfName})

	bs := cfgplugins.NewBGPSession(t, cfgplugins.PortCount4, &mgmtVRF)
	bs.WithEBGP(
		t,
		[]oc.E_BgpTypes_AFI_SAFI_TYPE{oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST},
		bgpPorts,
		true,
		true,
	)

	if setNoPeerGroup {
		bs.DUTConf.GetOrCreateNetworkInstance(mgmtVRF).GetOrCreateProtocol(cfgplugins.PTBGP, "BGP").GetOrCreateBgp().PeerGroup = nil
		neighbors := bs.DUTConf.GetOrCreateNetworkInstance(mgmtVRF).GetOrCreateProtocol(cfgplugins.PTBGP, "BGP").GetOrCreateBgp().Neighbor
		for _, neighbor := range neighbors {
			neighbor.PeerGroup = nil
		}
	}

	configureEmulatedNetworks(bs)

	bs.DUTConf.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut)).GetOrCreateProtocol(cfgplugins.PTBGP, "BGP").GetOrCreateBgp().GetOrCreateGlobal().SetAs(dutAS)
	bs.DUTConf.GetOrCreateNetworkInstance(mgmtVRF).SetRouteDistinguisher(fmt.Sprintf("%d:%d", dutAS, 100))

	bs.PushAndStart(t)
	if verfied := verifyDUTBGPEstablished(t, bs.DUT, mgmtVRF); verfied {
		t.Log("DUT BGP sessions established")
	} else {
		t.Fatalf("BGP sessions not established")
	}
	cfgplugins.VerifyOTGBGPEstablished(t, bs.ATE)

	configureLoopbackOnDUT(t, bs.DUT)
	advertiseDUTLoopbackToATE(t, bs.DUT)
	configureStaticRoute(t, bs.DUT, bs.ATEPorts[2].IPv6)

	t.Run("traffic received by port1 or port2", func(t *testing.T) {
		createFlowV6(t, bs)
		otgutils.WaitForARP(t, bs.ATE.OTG(), bs.ATETop, "IPv6")
		bs.ATE.OTG().StartTraffic(t)
		time.Sleep(30 * time.Second)
		bs.ATE.OTG().StopTraffic(t)
		otgutils.LogFlowMetrics(t, bs.ATE.OTG(), bs.ATETop)
		otgutils.LogPortMetrics(t, bs.ATE.OTG(), bs.ATETop)
		lossV6 := otgutils.GetFlowLossPct(t, bs.ATE.OTG(), "v6Flow", 10*time.Second)
		if lossV6 > lossTolerance {
			t.Errorf("Loss percent for IPv6 Traffic: got: %f, want %f", lossV6, lossTolerance)
		}
	})

	t.Run("traffic received by port2", func(t *testing.T) {
		createFlowV6(t, bs)
		gnmi.Replace(t, dut, gnmi.OC().Interface(p1.Name()).Enabled().Config(), false)
		gnmi.Await(t, dut, gnmi.OC().Interface(p1.Name()).AdminStatus().State(), 30*time.Second, oc.Interface_AdminStatus_DOWN)
		bs.ATE.OTG().StartTraffic(t)
		time.Sleep(30 * time.Second)
		bs.ATE.OTG().StopTraffic(t)
		otgutils.LogFlowMetrics(t, bs.ATE.OTG(), bs.ATETop)
		otgutils.LogPortMetrics(t, bs.ATE.OTG(), bs.ATETop)
		framesTx := gnmi.Get(t, bs.ATE.OTG(), gnmi.OTG().Port(bs.ATE.Port(t, "port4").ID()).Counters().OutFrames().State())
		framesRx := gnmi.Get(t, bs.ATE.OTG(), gnmi.OTG().Port(bs.ATE.Port(t, "port2").ID()).Counters().InFrames().State())
		if lossPct(float64(framesTx), float64(framesRx)) > lossTolerance {
			t.Errorf("Frames sent/received: got: %d, want: %d", framesRx, framesTx)
		}
	})

	t.Run("traffic received by port3", func(t *testing.T) {
		createFlowV6(t, bs)
		gnmi.Replace(t, dut, gnmi.OC().Interface(p2.Name()).Enabled().Config(), false)
		gnmi.Await(t, dut, gnmi.OC().Interface(p2.Name()).AdminStatus().State(), 30*time.Second, oc.Interface_AdminStatus_DOWN)
		bs.ATE.OTG().StartTraffic(t)
		time.Sleep(30 * time.Second)
		bs.ATE.OTG().StopTraffic(t)
		otgutils.LogFlowMetrics(t, bs.ATE.OTG(), bs.ATETop)
		otgutils.LogPortMetrics(t, bs.ATE.OTG(), bs.ATETop)
		framesTx := gnmi.Get(t, bs.ATE.OTG(), gnmi.OTG().Port(bs.ATE.Port(t, "port4").ID()).Counters().OutFrames().State())
		framesRx := gnmi.Get(t, bs.ATE.OTG(), gnmi.OTG().Port(bs.ATE.Port(t, "port3").ID()).Counters().InFrames().State())
		if lossPct(float64(framesTx), float64(framesRx)) > lossTolerance {
			t.Errorf("Frames sent/received: got: %d, want: %d", framesRx, framesTx)
		}
	})

	t.Run("traffic received by port1", func(t *testing.T) {
		createFlowV6(t, bs)
		gnmi.Replace(t, dut, gnmi.OC().Interface(p1.Name()).Enabled().Config(), true)
		gnmi.Await(t, dut, gnmi.OC().Interface(p1.Name()).AdminStatus().State(), 30*time.Second, oc.Interface_AdminStatus_UP)
		time.Sleep(30 * time.Second)
		bs.ATE.OTG().StartTraffic(t)
		time.Sleep(30 * time.Second)
		bs.ATE.OTG().StopTraffic(t)
		otgutils.LogFlowMetrics(t, bs.ATE.OTG(), bs.ATETop)
		otgutils.LogPortMetrics(t, bs.ATE.OTG(), bs.ATETop)
		framesTx := gnmi.Get(t, bs.ATE.OTG(), gnmi.OTG().Port(bs.ATE.Port(t, "port4").ID()).Counters().OutFrames().State())
		framesRx := gnmi.Get(t, bs.ATE.OTG(), gnmi.OTG().Port(bs.ATE.Port(t, "port1").ID()).Counters().InFrames().State())
		if lossPct(float64(framesTx), float64(framesRx)) > lossTolerance {
			t.Errorf("Frames sent/received: got: %d, want: %d", framesRx, framesTx)
		}
	})
}

func createFlowV6(t *testing.T, bs *cfgplugins.BGPSession) {
	bs.ATETop.Flows().Clear()

	t.Log("Configuring v6 traffic flow")
	v6Flow := bs.ATETop.Flows().Add().SetName("v6Flow")
	v6Flow.Metrics().SetEnable(true)
	v6Flow.TxRx().Device().
		SetTxNames([]string{"port4.IPv6"}).
		SetRxNames([]string{"port1.BGP4.peer.rr6", "port2.BGP4.peer.rr6", "port3.IPv6"})
	v6Flow.Size().SetFixed(512)
	v6Flow.Rate().SetPps(100)
	v6Flow.Duration().Continuous()
	e1 := v6Flow.Packet().Add().Ethernet()
	e1.Src().SetValues([]string{bs.ATEPorts[3].MAC})
	v6 := v6Flow.Packet().Add().Ipv6()
	v6.Src().SetValue(bs.ATEPorts[3].IPv6)
	v6.Dst().Increment().SetStart(prefixesStart).SetCount(1)
	icmp1 := v6Flow.Packet().Add().Icmp()
	icmp1.SetEcho(gosnappi.NewFlowIcmpEcho())

	bs.ATE.OTG().PushConfig(t, bs.ATETop)
	bs.ATE.OTG().StartProtocols(t)
}

func configureStaticRoute(t *testing.T, dut *ondatra.DUTDevice, nextHopIP string) {
	c := &oc.NetworkInstance_Protocol{
		Identifier: oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC,
		Name:       ygot.String(deviations.StaticProtocolName(dut)),
	}
	s := c.GetOrCreateStatic(defaultRoute + "/0")
	nh := s.GetOrCreateNextHop("0")
	nh.NextHop = oc.UnionString(nextHopIP)
	if deviations.SetMetricAsPreference(dut) {
		nh.Metric = ygot.Uint32(220)
	} else {
		nh.Preference = ygot.Uint32(220)
	}
	sp := gnmi.OC().NetworkInstance(mgmtVRF).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
	gnmi.Update(t, dut, sp.Config(), c)
	gnmi.Replace(t, dut, sp.Static(defaultRoute+"/0").Config(), s)
}

func configureEmulatedNetworks(bs *cfgplugins.BGPSession) {
	devices := bs.ATETop.Devices().Items()
	byName := func(i, j int) bool { return devices[i].Name() < devices[j].Name() }
	sort.Slice(devices, byName)
	for i, otgPort := range bs.ATEPorts[:len(bgpPorts)] {
		ipv6 := devices[i].Ethernets().Items()[0].Ipv6Addresses().Items()[0]
		bgp6Peer := devices[i].Bgp().Ipv6Interfaces().Items()[0].Peers().Items()[0]
		bgp6PeerRoute := bgp6Peer.V6Routes().Add()
		bgp6PeerRoute.SetName(otgPort.Name + ".BGP4.peer.rr6")
		bgp6PeerRoute.SetNextHopIpv6Address(ipv6.Address())
		bgp6PeerRoute.SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6)
		bgp6PeerRoute.SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL)
		bgp6PeerRoute.AddPath().SetPathId(pathID)

		bgp6PeerRoute.Addresses().Add().SetAddress(prefixesStart).SetPrefix(prefixP6Len).SetCount(prefixesCount)
		bgp6PeerRoute.Addresses().Add().SetAddress(defaultRoute).SetPrefix(0)
	}
}

func configureLoopbackOnDUT(t *testing.T, dut *ondatra.DUTDevice) {
	loopbackIntfName := netutil.LoopbackInterface(t, dut, 1)
	loop := dutlo0Attrs.NewOCInterface(loopbackIntfName, dut)
	loop.Type = oc.IETFInterfaces_InterfaceType_softwareLoopback
	gnmi.Update(t, dut, gnmi.OC().Interface(loopbackIntfName).Config(), loop)
	t.Logf("Got DUT IPv6 loopback address: %v", dutlo0Attrs.IPv6)
}

func addInterfacesToVRF(t *testing.T, dut *ondatra.DUTDevice, vrfname string, intfNames []string) {
	root := &oc.Root{}
	mgmtNI := root.GetOrCreateNetworkInstance(vrfname)
	mgmtNI.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF
	for _, intfName := range intfNames {
		vi := mgmtNI.GetOrCreateInterface(intfName)
		vi.Interface = ygot.String(intfName)
		vi.Subinterface = ygot.Uint32(0)
	}
	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(mgmtVRF).Config(), mgmtNI)
	t.Logf("Added interface %v to VRF %s", intfNames, vrfname)
}

func verifyDUTBGPEstablished(t *testing.T, dut *ondatra.DUTDevice, ni string) bool {
	nSessionState := gnmi.OC().NetworkInstance(ni).Protocol(cfgplugins.PTBGP, "BGP").Bgp().NeighborAny().SessionState().State()
	watch := gnmi.WatchAll(t, dut, nSessionState, 2*time.Minute, func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
		state, ok := val.Val()
		if !ok || state != oc.Bgp_Neighbor_SessionState_ESTABLISHED {
			return false
		}
		return true
	})
	if _, ok := watch.Await(t); !ok {
		return false
	}
	return true
}

func advertiseDUTLoopbackToATE(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()

	batchSet := &gnmi.SetBatch{}

	root := &oc.Root{}
	rp := root.GetOrCreateRoutingPolicy()
	pdef := rp.GetOrCreatePolicyDefinition("rp")
	stmt, err := pdef.AppendNewStatement("rp-stmt")
	if err != nil {
		t.Fatalf("AppendNewStatement(%s) failed: %v", "rp-stmt", err)
	}
	stmt.GetOrCreateActions().SetPolicyResult(oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)

	prefixSet := rp.GetOrCreateDefinedSets().GetOrCreatePrefixSet("ps")
	prefixSet.SetMode(oc.PrefixSet_Mode_IPV6)
	prefixSet.GetOrCreatePrefix(dutlo0Attrs.IPv6CIDR(), "exact")

	if !deviations.SkipSetRpMatchSetOptions(dut) {
		stmt.GetOrCreateConditions().GetOrCreateMatchPrefixSet().SetMatchSetOptions(oc.RoutingPolicy_MatchSetOptionsRestrictedType_ANY)
	}
	stmt.GetOrCreateConditions().GetOrCreateMatchPrefixSet().SetPrefixSet("ps")
	gnmi.BatchUpdate(batchSet, gnmi.OC().RoutingPolicy().Config(), rp)

	tableConn := root.GetOrCreateNetworkInstance(mgmtVRF).GetOrCreateTableConnection(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_DIRECTLY_CONNECTED, oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, oc.Types_ADDRESS_FAMILY_IPV6)
	if !deviations.SkipSettingDisableMetricPropagation(dut) {
		tableConn.SetDisableMetricPropagation(false)
	}
	tableConn.SetDefaultImportPolicy(oc.RoutingPolicy_DefaultPolicyType_REJECT_ROUTE)
	tableConn.SetImportPolicy([]string{"rp"})
	gnmi.BatchUpdate(batchSet, gnmi.OC().NetworkInstance(mgmtVRF).TableConnection(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_DIRECTLY_CONNECTED, oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, oc.Types_ADDRESS_FAMILY_IPV6).Config(), tableConn)

	tableConn1 := root.GetOrCreateNetworkInstance(mgmtVRF).GetOrCreateTableConnection(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_DIRECTLY_CONNECTED, oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, oc.Types_ADDRESS_FAMILY_IPV4)
	tableConn1.SetImportPolicy([]string{"rp"})
	gnmi.BatchUpdate(batchSet, gnmi.OC().NetworkInstance(mgmtVRF).TableConnection(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_DIRECTLY_CONNECTED, oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, oc.Types_ADDRESS_FAMILY_IPV4).Config(), tableConn1)

	batchSet.Set(t, dut)
}

func lossPct(tx, rx float64) float64 {
	return (math.Abs(tx-rx) * 100) / tx
}
