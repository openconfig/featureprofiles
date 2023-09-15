// Copyright 2023 Google LLC
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

package bgp_always_compare_med_test

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

const (
	trafficDuration          = 1 * time.Minute
	ipv4SrcTraffic           = "192.0.2.2"
	advertisedRoutesv4CIDR   = "203.0.113.1/32"
	advertisedRoutesv4Net    = "203.0.113.1"
	advertisedRoutesv4Prefix = 32
	ipv4DstTrafficStart      = "203.0.113.1"
	ipv4DstTrafficEnd        = "203.0.113.254"
	peerGrpName1             = "BGP-PEER-GROUP1"
	peerGrpName2             = "BGP-PEER-GROUP2"
	peerGrpName3             = "BGP-PEER-GROUP3"
	tolerancePct             = 2
	tolerance                = 50
	routeCount               = 254
	dutAS                    = 65501
	ateAS1                   = 65501
	ateAS2                   = 65502
	ateAS3                   = 65503
	plenIPv4                 = 30
	plenIPv6                 = 126
	setMEDPolicy100          = "SET-MED-100"
	setMEDPolicy50           = "SET-MED-50"
	aclStatement20           = "20"
	aclStatement30           = "30"
	bgpMED100                = 100
	bgpMED50                 = 50
	wantLoss                 = true
	flow1                    = "flowPort1toPort2"
	flow2                    = "flowPort1toPort3"
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
		MAC:     "02:00:01:01:01:01",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	dutDst1 = attrs.Attributes{
		Desc:    "DUT to ATE destination 1",
		IPv4:    "192.0.2.5",
		IPv6:    "2001:db8::192:0:2:5",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	ateDst1 = attrs.Attributes{
		Name:    "atedst1",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::192:0:2:6",
		MAC:     "02:00:02:01:01:01",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	dutDst2 = attrs.Attributes{
		Desc:    "DUT to ATE destination 2",
		IPv4:    "192.0.2.9",
		IPv6:    "2001:db8::192:0:2:9",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	ateDst2 = attrs.Attributes{
		Name:    "atedst2",
		IPv4:    "192.0.2.10",
		IPv6:    "2001:db8::192:0:2:10",
		MAC:     "02:00:03:01:01:01",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
)

// configureDUT configures all the interfaces on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	dc := gnmi.OC()
	i1 := dutSrc.NewOCInterface(dut.Port(t, "port1").Name(), dut)
	gnmi.Replace(t, dut, dc.Interface(i1.GetName()).Config(), i1)

	i2 := dutDst1.NewOCInterface(dut.Port(t, "port2").Name(), dut)
	gnmi.Replace(t, dut, dc.Interface(i2.GetName()).Config(), i2)

	i3 := dutDst2.NewOCInterface(dut.Port(t, "port3").Name(), dut)
	gnmi.Replace(t, dut, dc.Interface(i3.GetName()).Config(), i3)
}

// verifyPortsUp asserts that each port on the device is operating.
func verifyPortsUp(t *testing.T, dev *ondatra.Device) {
	t.Helper()
	for _, p := range dev.Ports() {
		status := gnmi.Get(t, dev, gnmi.OC().Interface(p.Name()).OperStatus().State())
		if want := oc.Interface_OperStatus_UP; status != want {
			t.Errorf("%s Status: got %v, want %v", p, status, want)
		}
	}
}

// bgpCreateNbr creates a BGP object with neighbors pointing to ateSrc and ateDst.
func bgpCreateNbr(localAs, peerAs uint32, dut *ondatra.DUTDevice) *oc.NetworkInstance_Protocol {
	nbr1v4 := &bgpNeighbor{as: ateAS1, neighborip: ateSrc.IPv4, isV4: true, peerGrp: peerGrpName1}
	nbr2v4 := &bgpNeighbor{as: ateAS2, neighborip: ateDst1.IPv4, isV4: true, peerGrp: peerGrpName2}
	nbr3v4 := &bgpNeighbor{as: ateAS3, neighborip: ateDst2.IPv4, isV4: true, peerGrp: peerGrpName3}
	nbrs := []*bgpNeighbor{nbr1v4, nbr2v4, nbr3v4}

	dutOcRoot := &oc.Root{}
	ni1 := dutOcRoot.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niProto := ni1.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := niProto.GetOrCreateBgp()
	global := bgp.GetOrCreateGlobal()
	global.RouterId = ygot.String(dutDst2.IPv4)
	global.As = ygot.Uint32(localAs)
	global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)

	// Note: we have to define the peer group even if we aren't setting any policy because it's
	// invalid OC for the neighbor to be part of a peer group that doesn't exist.
	pg1 := bgp.GetOrCreatePeerGroup(peerGrpName1)
	pg1.PeerAs = ygot.Uint32(ateAS1)
	pg1.PeerGroupName = ygot.String(peerGrpName1)

	pg2 := bgp.GetOrCreatePeerGroup(peerGrpName2)
	pg2.PeerAs = ygot.Uint32(ateAS2)
	pg2.PeerGroupName = ygot.String(peerGrpName2)

	pg3 := bgp.GetOrCreatePeerGroup(peerGrpName3)
	pg3.PeerAs = ygot.Uint32(ateAS3)
	pg3.PeerGroupName = ygot.String(peerGrpName3)

	for _, nbr := range nbrs {
		nv4 := bgp.GetOrCreateNeighbor(nbr.neighborip)
		nv4.PeerGroup = ygot.String(nbr.peerGrp)
		nv4.PeerAs = ygot.Uint32(nbr.as)
		nv4.Enabled = ygot.Bool(true)
		af4 := nv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
		af4.Enabled = ygot.Bool(true)
		af6 := nv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
		af6.Enabled = ygot.Bool(false)
	}
	return niProto
}

func sendTraffic(t *testing.T, otg *otg.OTG, c gosnappi.Config) {
	t.Helper()
	t.Logf("Starting traffic")
	otg.StartTraffic(t)
	time.Sleep(trafficDuration)
	t.Logf("Stop traffic")
	otg.StopTraffic(t)
}

func verifyOTGBGPTelemetry(t *testing.T, otg *otg.OTG, c gosnappi.Config, state string) {
	t.Helper()
	for _, d := range c.Devices().Items() {
		for _, ip := range d.Bgp().Ipv4Interfaces().Items() {
			for _, configPeer := range ip.Peers().Items() {
				nbrPath := gnmi.OTG().BgpPeer(configPeer.Name())
				_, ok := gnmi.Watch(t, otg, nbrPath.SessionState().State(), time.Minute, func(val *ygnmi.Value[otgtelemetry.E_BgpPeer_SessionState]) bool {
					currState, ok := val.Val()
					return ok && currState.String() == state
				}).Await(t)
				if !ok {
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
					t.Errorf("No BGP neighbor formed for peer %s", configPeer.Name())
				}
			}
		}
	}
}

// verifyBgpTelemetry checks that the dut has an established BGP session with reasonable settings.
func verifyBgpTelemetry(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	var nbrIP = []string{ateSrc.IPv4, ateDst1.IPv4, ateDst2.IPv4}
	t.Logf("Verifying BGP state.")
	bgpPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()

	for _, nbr := range nbrIP {
		nbrPath := bgpPath.Neighbor(nbr)
		// Get BGP adjacency state.
		t.Logf("Waiting for BGP neighbor to establish...")
		status, ok := gnmi.Watch(t, dut, nbrPath.SessionState().State(), time.Minute, func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
			state, ok := val.Val()
			return ok && state == oc.Bgp_Neighbor_SessionState_ESTABLISHED
		}).Await(t)
		if !ok {
			t.Fatal("No BGP neighbor formed")
		}
		state, _ := status.Val()
		t.Logf("BGP adjacency for %s: %v", nbr, state)
		if want := oc.Bgp_Neighbor_SessionState_ESTABLISHED; state != want {
			t.Errorf("BGP peer %s status got %d, want %d", nbr, state, want)
		}
	}
}

// configureOTG configures the interfaces and BGP protocols on an ATE, including
// advertising some(faked) networks over BGP.
func configureOTG(t *testing.T, otg *otg.OTG) gosnappi.Config {
	t.Helper()
	config := otg.NewConfig(t)
	port1 := config.Ports().Add().SetName("port1")
	port2 := config.Ports().Add().SetName("port2")
	port3 := config.Ports().Add().SetName("port3")

	iDut1Dev := config.Devices().Add().SetName(ateSrc.Name)
	iDut1Eth := iDut1Dev.Ethernets().Add().SetName(ateSrc.Name + ".Eth").SetMac(ateSrc.MAC)
	iDut1Eth.Connection().SetChoice(gosnappi.EthernetConnectionChoice.PORT_NAME).SetPortName(port1.Name())
	iDut1Ipv4 := iDut1Eth.Ipv4Addresses().Add().SetName(ateSrc.Name + ".IPv4")
	iDut1Ipv4.SetAddress(ateSrc.IPv4).SetGateway(dutSrc.IPv4).SetPrefix(uint32(ateSrc.IPv4Len))
	iDut1Ipv6 := iDut1Eth.Ipv6Addresses().Add().SetName(ateSrc.Name + ".IPv6")
	iDut1Ipv6.SetAddress(ateSrc.IPv6).SetGateway(dutSrc.IPv6).SetPrefix(uint32(ateSrc.IPv6Len))

	iDut2Dev := config.Devices().Add().SetName(ateDst1.Name)
	iDut2Eth := iDut2Dev.Ethernets().Add().SetName(ateDst1.Name + ".Eth").SetMac(ateDst1.MAC)
	iDut2Eth.Connection().SetChoice(gosnappi.EthernetConnectionChoice.PORT_NAME).SetPortName(port2.Name())
	iDut2Ipv4 := iDut2Eth.Ipv4Addresses().Add().SetName(ateDst1.Name + ".IPv4")
	iDut2Ipv4.SetAddress(ateDst1.IPv4).SetGateway(dutDst1.IPv4).SetPrefix(uint32(ateDst1.IPv4Len))
	iDut2Ipv6 := iDut2Eth.Ipv6Addresses().Add().SetName(ateDst1.Name + ".IPv6")
	iDut2Ipv6.SetAddress(ateDst1.IPv6).SetGateway(dutDst1.IPv6).SetPrefix(uint32(ateDst1.IPv6Len))

	iDut3Dev := config.Devices().Add().SetName(ateDst2.Name)
	iDut3Eth := iDut3Dev.Ethernets().Add().SetName(ateDst2.Name + ".Eth").SetMac(ateDst2.MAC)
	iDut3Eth.Connection().SetChoice(gosnappi.EthernetConnectionChoice.PORT_NAME).SetPortName(port3.Name())
	iDut3Ipv4 := iDut3Eth.Ipv4Addresses().Add().SetName(ateDst2.Name + ".IPv4")
	iDut3Ipv4.SetAddress(ateDst2.IPv4).SetGateway(dutDst2.IPv4).SetPrefix(uint32(ateDst2.IPv4Len))
	iDut3Ipv6 := iDut3Eth.Ipv6Addresses().Add().SetName(ateDst2.Name + ".IPv6")
	iDut3Ipv6.SetAddress(ateDst2.IPv6).SetGateway(dutDst2.IPv6).SetPrefix(uint32(ateDst2.IPv6Len))

	// BGP seesion
	iDut1Bgp := iDut1Dev.Bgp().SetRouterId(iDut1Ipv4.Address())
	iDut1Bgp4Peer := iDut1Bgp.Ipv4Interfaces().Add().SetIpv4Name(iDut1Ipv4.Name()).Peers().Add().SetName(ateSrc.Name + ".BGP4.peer")
	iDut1Bgp4Peer.SetPeerAddress(iDut1Ipv4.Gateway()).SetAsNumber(ateAS1).SetAsType(gosnappi.BgpV4PeerAsType.IBGP)
	iDut1Bgp4Peer.Capability().SetIpv4UnicastAddPath(true).SetIpv6UnicastAddPath(true)
	iDut1Bgp4Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)

	iDut2Bgp := iDut2Dev.Bgp().SetRouterId(iDut2Ipv4.Address())
	iDut2Bgp4Peer := iDut2Bgp.Ipv4Interfaces().Add().SetIpv4Name(iDut2Ipv4.Name()).Peers().Add().SetName(ateDst1.Name + ".BGP4.peer")
	iDut2Bgp4Peer.SetPeerAddress(iDut2Ipv4.Gateway()).SetAsNumber(ateAS2).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
	iDut2Bgp4Peer.Capability().SetIpv4UnicastAddPath(true).SetIpv6UnicastAddPath(true)
	iDut2Bgp4Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)

	iDut3Bgp := iDut3Dev.Bgp().SetRouterId(iDut3Ipv4.Address())
	iDut3Bgp4Peer := iDut3Bgp.Ipv4Interfaces().Add().SetIpv4Name(iDut3Ipv4.Name()).Peers().Add().SetName(ateDst2.Name + ".BGP4.peer")
	iDut3Bgp4Peer.SetPeerAddress(iDut3Ipv4.Gateway()).SetAsNumber(ateAS3).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
	iDut3Bgp4Peer.Capability().SetIpv4UnicastAddPath(true).SetIpv6UnicastAddPath(true)
	iDut3Bgp4Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)

	bgpNeti1Bgp4PeerRoutes := iDut2Bgp4Peer.V4Routes().Add().SetName(ateDst1.Name + ".BGP4.Route")
	bgpNeti1Bgp4PeerRoutes.SetNextHopIpv4Address(iDut2Ipv4.Address()).
		SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).
		SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)
	bgpNeti1Bgp4PeerRoutes.Addresses().Add().
		SetAddress(advertisedRoutesv4Net).
		SetPrefix(advertisedRoutesv4Prefix).
		SetCount(routeCount)

	bgpNeti2Bgp4PeerRoutes := iDut3Bgp4Peer.V4Routes().Add().SetName(ateDst2.Name + ".BGP4.Route")
	bgpNeti2Bgp4PeerRoutes.SetNextHopIpv4Address(iDut3Ipv4.Address()).
		SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).
		SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)
	bgpNeti2Bgp4PeerRoutes.Addresses().Add().
		SetAddress(advertisedRoutesv4Net).
		SetPrefix(advertisedRoutesv4Prefix).
		SetCount(routeCount)

	t.Logf("TestBGP:start otg Traffic config")
	flow1ipv4 := config.Flows().Add().SetName(flow1)
	flow1ipv4.Metrics().SetEnable(true)
	flow1ipv4.TxRx().Device().
		SetTxNames([]string{iDut1Ipv4.Name()}).
		SetRxNames([]string{bgpNeti1Bgp4PeerRoutes.Name()})
	flow1ipv4.Size().SetFixed(512)
	flow1ipv4.Rate().SetPps(100)
	flow1ipv4.Duration().SetChoice("continuous")
	e1 := flow1ipv4.Packet().Add().Ethernet()
	e1.Src().SetValue(iDut1Eth.Mac())
	v4 := flow1ipv4.Packet().Add().Ipv4()
	v4.Src().SetValue(iDut1Ipv4.Address())
	v4.Dst().Increment().SetStart(advertisedRoutesv4Net).SetCount(routeCount)

	t.Logf("TestBGP:start otg traffic config")
	flow2ipv4 := config.Flows().Add().SetName(flow2)
	flow2ipv4.Metrics().SetEnable(true)
	flow2ipv4.TxRx().Device().
		SetTxNames([]string{iDut1Ipv4.Name()}).
		SetRxNames([]string{bgpNeti2Bgp4PeerRoutes.Name()})
	flow2ipv4.Size().SetFixed(512)
	flow2ipv4.Rate().SetPps(100)
	flow2ipv4.Duration().SetChoice("continuous")
	e2 := flow2ipv4.Packet().Add().Ethernet()
	e2.Src().SetValue(iDut1Eth.Mac())
	v4Flow2 := flow2ipv4.Packet().Add().Ipv4()
	v4Flow2.Src().SetValue(iDut1Ipv4.Address())
	v4Flow2.Dst().Increment().SetStart(advertisedRoutesv4Net).SetCount(routeCount)

	t.Logf("Pushing config to OTG and starting protocols...")
	otg.PushConfig(t, config)
	time.Sleep(30 * time.Second)
	otg.StartProtocols(t)
	time.Sleep(30 * time.Second)

	return config
}

// verifyTraffic confirms that every traffic flow has the expected amount of loss (0% or 100%
// depending on wantLoss, +- 2%).
func verifyTraffic(t *testing.T, ate *ondatra.ATEDevice, c gosnappi.Config, flowName string, wantLoss bool) {
	t.Helper()
	otg := ate.OTG()
	otgutils.LogFlowMetrics(t, otg, c)
	t.Logf("Verifying flow metrics for flow %s\n", flowName)
	recvMetric := gnmi.Get(t, otg, gnmi.OTG().Flow(flowName).State())
	txPackets := recvMetric.GetCounters().GetOutPkts()
	rxPackets := recvMetric.GetCounters().GetInPkts()
	lostPackets := txPackets - rxPackets
	lossPct := lostPackets * 100 / txPackets
	if wantLoss {
		if lossPct < 100-tolerancePct {
			t.Errorf("Traffic is expected to fail %s\n got %v, want 100%% failure", flowName, lossPct)
		} else {
			t.Logf("Traffic Loss Test Passed!")
		}
	} else {
		if lossPct > tolerancePct {
			t.Errorf("Traffic Loss Pct for Flow: %s\n got %v, want 0", flowName, lossPct)
		} else {
			t.Logf("Traffic Test Passed!")
		}
	}
}

// setMED is used to configure routing policy to set BGP MED on DUT.
func setMED(t *testing.T, dut *ondatra.DUTDevice, d *oc.Root) {
	t.Helper()
	rp := d.GetOrCreateRoutingPolicy()

	pdef1 := rp.GetOrCreatePolicyDefinition(setMEDPolicy100)
	stmt1, err := pdef1.AppendNewStatement(aclStatement20)
	if err != nil {
		t.Errorf("Error while creating new statement %v", err)
	}
	actions1 := stmt1.GetOrCreateActions()
	actions1.GetOrCreateBgpActions().SetMed = oc.UnionUint32(bgpMED100)

	pdef2 := rp.GetOrCreatePolicyDefinition(setMEDPolicy50)
	stmt2, err := pdef2.AppendNewStatement(aclStatement20)
	if err != nil {
		t.Errorf("Error while creating new statement %v", err)
	}
	actions2 := stmt2.GetOrCreateActions()
	actions2.GetOrCreateBgpActions().SetMed = oc.UnionUint32(bgpMED50)

	gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)

	// Apply setMed import policy on eBGP Peer1 - OTG Port2 - with MED 100.
	dutPolicyConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).
		Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp().
		PeerGroup(peerGrpName2).ApplyPolicy().ImportPolicy()
	gnmi.Replace(t, dut, dutPolicyConfPath.Config(), []string{setMEDPolicy100})

	// Apply setMed Import policy on eBGP Peer2 - OTG Port3 -  with MED 50.
	dutPolicyConfPath = gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).
		Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp().
		PeerGroup(peerGrpName3).ApplyPolicy().ImportPolicy()
	gnmi.Replace(t, dut, dutPolicyConfPath.Config(), []string{setMEDPolicy50})
}

// verifySetMed is used to validate MED on received prefixes at OTG Port1.
func verifySetMed(t *testing.T, otg *otg.OTG, config gosnappi.Config, wantMEDValue uint32) {
	t.Helper()
	_, ok := gnmi.WatchAll(t,
		otg,
		gnmi.OTG().BgpPeer(ateSrc.Name+".BGP4.peer").UnicastIpv4PrefixAny().State(),
		2*time.Minute,
		func(v *ygnmi.Value[*otgtelemetry.BgpPeer_UnicastIpv4Prefix]) bool {
			return v.IsPresent()
		}).Await(t)

	if ok {
		bgpPrefixes := gnmi.GetAll(t, otg, gnmi.OTG().BgpPeer(ateSrc.Name+".BGP4.peer").UnicastIpv4PrefixAny().State())
		gotPrefixCount := len(bgpPrefixes)
		if gotPrefixCount < routeCount {
			t.Errorf("Received prefixes on otg are not as expected got prefixes %v, want prefixes %v", gotPrefixCount, routeCount)
		} else {
			t.Logf("Received prefixes on otg are matched, got prefixes %v, want prefixes %v", gotPrefixCount, routeCount)
		}
	}

	// TODO: Below code will be uncommented once MED retrieval is supported in OTG.
	// https://github.com/openconfig/featureprofiles/issues/1837

	/*
		wantSetMED := []uint32{}
		// Build wantSetMED to compare the diff.
		for i := 0; i < routeCount; i++ {
			wantSetMED = append(wantSetMED, uint32(wantMEDValue))
		}

		prefixPathAny := gnmi.OTG().BgpPeer(ateSrc.Name + ".BGP4.peer").UnicastIpv4PrefixAny()
		gnmi.WatchAll(t, otg, prefixPathAny.State(), time.Minute, func(v *ygnmi.Value[string]) bool {
			_, present := v.Val()
			return present
		}).Await(t)
		_, ok := gnmi.WatchAll(t, otg, prefixPathAny.Med().State(), 3*time.Minute, func(v *ygnmi.Value[otgtelemetry.E_UnicastIpv4Prefix_Origin]) bool {
			gotSetMED, present := v.Val()
			return present && cmp.Diff(wantSetMED, gotSetMED) == ""
		}).Await(t)
		if !ok {
			t.Errorf("obtained MED on OTG is not as expected")
		}
	*/
}

// verifyBGPCapabilities is used to Verify BGP capabilities like route refresh as32 and mpbgp.
func verifyBGPCapabilities(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	t.Log("Verifying BGP capabilities.")
	var nbrIP = []string{ateSrc.IPv4, ateDst1.IPv4, ateDst2.IPv4}
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	for _, nbr := range nbrIP {
		nbrPath := statePath.Neighbor(nbr)
		capabilities := map[oc.E_BgpTypes_BGP_CAPABILITY]bool{
			oc.BgpTypes_BGP_CAPABILITY_ROUTE_REFRESH: false,
			oc.BgpTypes_BGP_CAPABILITY_MPBGP:         false,
		}
		for _, cap := range gnmi.Get(t, dut, nbrPath.SupportedCapabilities().State()) {
			capabilities[cap] = true
		}
		for cap, present := range capabilities {
			if !present {
				t.Errorf("Capability not reported: %v", cap)
			}
		}
	}
}

// verifyPrefixesTelemetry confirms that the dut shows the correct numbers of installed,
// sent and received IPv4 prefixes.
func verifyPrefixesTelemetry(t *testing.T, dut *ondatra.DUTDevice, wantInstalled, wantSent uint32) {
	t.Helper()
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	prefixesv4 := statePath.Neighbor(ateSrc.IPv4).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Prefixes()
	if gotInstalled := gnmi.Get(t, dut, prefixesv4.Installed().State()); gotInstalled != wantInstalled {
		t.Errorf("Installed prefixes mismatch: got %v, want %v", gotInstalled, wantInstalled)
	}
	if gotSent := gnmi.Get(t, dut, prefixesv4.Sent().State()); gotSent != wantSent {
		t.Errorf("Sent prefixes mismatch: got %v, want %v", gotSent, wantSent)
	}
}

type bgpNeighbor struct {
	as         uint32
	neighborip string
	isV4       bool
	peerGrp    string
}

// TestRemovePrivateAS is to Validate that private AS numbers are stripped
// before advertisement to the eBGP neighbor.
func TestAlwaysCompareMED(t *testing.T) {
	t.Logf("Start DUT config load.")
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	d := &oc.Root{}

	t.Run("Configure DUT interfaces", func(t *testing.T) {
		t.Logf("Start DUT interface Config.")
		configureDUT(t, dut)
	})

	t.Run("Configure DEFAULT network instance", func(t *testing.T) {
		t.Log("Configure Network Instance type.")
		dutConfNIPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut))
		gnmi.Replace(t, dut, dutConfNIPath.Type().Config(), oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)
	})

	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	t.Run("Configure BGP Neighbors", func(t *testing.T) {
		t.Logf("Start DUT BGP Config.")
		gnmi.Delete(t, dut, dutConfPath.Config())
		dutConf := bgpCreateNbr(dutAS, ateAS1, dut)
		gnmi.Replace(t, dut, dutConfPath.Config(), dutConf)
		fptest.LogQuery(t, "DUT BGP Config", dutConfPath.Config(), gnmi.GetConfig(t, dut, dutConfPath.Config()))
	})

	otg := ate.OTG()
	var otgConfig gosnappi.Config
	t.Run("Configure OTG", func(t *testing.T) {
		otgConfig = configureOTG(t, otg)
	})

	t.Run("Verify port status on DUT", func(t *testing.T) {
		verifyPortsUp(t, dut.Device)
	})

	t.Run("Verify BGP telemetry", func(t *testing.T) {
		verifyBgpTelemetry(t, dut)
		verifyOTGBGPTelemetry(t, otg, otgConfig, "ESTABLISHED")
		verifyBGPCapabilities(t, dut)
	})

	t.Run("Configure SET MED on DUT", func(t *testing.T) {
		setMED(t, dut, d)
	})

	t.Run("Configure always compare med on DUT", func(t *testing.T) {
		gnmi.Replace(t, dut, dutConfPath.Bgp().Global().RouteSelectionOptions().AlwaysCompareMed().Config(), true)
	})

	t.Run("Verify received BGP routes at OTG Port 1 have lowest MED", func(t *testing.T) {
		verifyPrefixesTelemetry(t, dut, 0, routeCount)
		t.Log("Verify best route advertised to atePort1 is Peer with lowest MED 50 - eBGP Peer2.")
		verifySetMed(t, otg, otgConfig, bgpMED50)
	})
	t.Run("Send and validate traffic from OTG Port1", func(t *testing.T) {
		t.Log("Validate traffic flowing to the prefixes received from eBGP neighbor #2 from DUT (lowest MED-50).")
		sendTraffic(t, otg, otgConfig)
		verifyTraffic(t, ate, otgConfig, flow1, wantLoss)
		verifyTraffic(t, ate, otgConfig, flow2, !wantLoss)
	})

	t.Run("Remove MED settings on DUT", func(t *testing.T) {
		dutPolicyConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
		gnmi.Delete(t, dut, dutPolicyConfPath.PeerGroup(peerGrpName2).ApplyPolicy().ImportPolicy().Config())
		gnmi.Delete(t, dut, dutPolicyConfPath.PeerGroup(peerGrpName3).ApplyPolicy().ImportPolicy().Config())
	})

	t.Run("Verify MED on received routes at OTG Port1 after removing MED settings", func(t *testing.T) {
		verifyPrefixesTelemetry(t, dut, 0, routeCount)
		t.Log("Verify best route advertised to atePort1.")
		verifySetMed(t, otg, otgConfig, uint32(0))
	})

	t.Run("Send and verify traffic after removing MED settings on DUT", func(t *testing.T) {
		t.Log("Validate traffic change due to change in MED settings - Best route changes.")
		sendTraffic(t, otg, otgConfig)
		verifyTraffic(t, ate, otgConfig, flow1, !wantLoss)
		verifyTraffic(t, ate, otgConfig, flow2, wantLoss)
	})
}
