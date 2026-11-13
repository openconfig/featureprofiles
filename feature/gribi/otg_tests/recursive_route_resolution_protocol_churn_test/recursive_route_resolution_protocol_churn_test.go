// Copyright 2026 Google LLC
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

package recursive_route_resolution_protocol_churn_test

import (
	"context"
	"fmt"
	"net/netip"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	aftspb "github.com/openconfig/gribi/v1/proto/service"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	otgtelemetry "github.com/openconfig/ondatra/gnmi/otg"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	isisName                = "DEFAULT"
	dutAS                   = uint32(64500)
	dutLoopback             = "192.0.2.1"
	prefixCount             = 1000
	flowName                = "Flow-1"
	controlFlow             = "Flow-2"
	flow1PPS                = 10000
	flow2PPS                = 1000
	frameBytes              = 512
	peer2Addr               = "192.0.2.2"
	peer3Addr               = "192.0.2.3"
	port2Routes             = "port2.routes"
	port3Routes             = "port3.routes"
	port3ISISRte            = "port3.loopback.route"
	bgpBaseSecond           = 64
	gribiBase               = 68
	steadyWindow            = 10 * time.Second
	settleSamples           = 5
	maxOutage               = 1.0
	minimumEgressFraction   = 0.99
	egressSampleAttempts    = 3
	maxUnexpectedFraction   = 0.01
	steadyLossTolerancePct  = 0.01
	totalLossTolerancePct   = 0.1
	maxRxExcessFraction     = 0.01
	churnConvergenceTimeout = 2 * time.Minute
	bgpEstablishTimeout     = 2 * time.Minute
	bgpHoldTime             = 9
	bgpKeepaliveInterval    = 3
	minimumLSPCount         = 3
	churnIterations         = 5
)

var (
	dutPorts = []attrs.Attributes{
		{Desc: "Ingress", IPv4: "198.51.100.1", IPv4Len: 30},
		{Desc: "Path 1", IPv4: "198.51.100.5", IPv4Len: 30},
		{Desc: "Path 2", IPv4: "198.51.100.9", IPv4Len: 30},
		{Desc: "Negative control", IPv4: "203.0.113.1", IPv4Len: 30},
	}
	atePorts = []attrs.Attributes{
		{Name: "port1", MAC: "02:00:01:01:01:01", IPv4: "198.51.100.2", IPv4Len: 30},
		{Name: "port2", MAC: "02:00:02:01:01:01", IPv4: "198.51.100.6", IPv4Len: 30},
		{Name: "port3", MAC: "02:00:03:01:01:01", IPv4: "198.51.100.10", IPv4Len: 30},
		{Name: "port4", MAC: "02:00:04:01:01:01", IPv4: "203.0.113.2", IPv4Len: 30},
	}
)

// TestMain configures the feature test harness before running tests.
func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// TestRecursiveRouteResolutionProtocolChurn verifies recursive gRIBI route
// resolution across BGP route flaps, IS-IS underlay failure, and route churn.
func TestRecursiveRouteResolutionProtocolChurn(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	otgC := ate.OTG()

	t.Cleanup(func() {
		cleanupT, cancel := cleanupTestContext(t)
		defer cancel()
		cleanupDUTConfig(cleanupT, dut)
	})
	configureDUT(t, dut)
	top := configureATE(t, ate)
	otgC.PushConfig(t, top)
	otgC.StartProtocols(t)
	setRouteState(t, ate, gosnappi.StateProtocolRouteState.WITHDRAW, port3Routes)
	otgutils.WaitForARP(t, otgC, top, "IPv4")

	ctx, cancelClient := context.WithCancel(context.WithoutCancel(t.Context()))
	t.Cleanup(cancelClient)
	client := fluent.NewClient()
	client.Connection().WithStub(dut.RawAPIs().GRIBI(t)).WithPersistence().WithInitialElectionID(1, 0).
		WithRedundancyMode(fluent.ElectedPrimaryClient).WithFIBACK()
	client.Start(ctx, t)
	t.Cleanup(func() {
		cleanupT, cancel := cleanupTestContext(t)
		defer cancel()
		client.Stop(cleanupT)
	})
	client.StartSending(ctx, t)
	gribi.BecomeLeader(t, client)
	flushDefault(t, dut, client)
	t.Cleanup(func() {
		cleanupT, cancel := cleanupTestContext(t)
		defer cancel()
		flushDefault(cleanupT, dut, client)
	})

	trafficStarted := false
	t.Cleanup(func() {
		if trafficStarted {
			cleanupT, cancel := cleanupTestContext(t)
			defer cancel()
			otgC.StopTraffic(cleanupT)
		}
	})

	if !t.Run("TE_1_4_1_BaseRecursiveResolution", func(t *testing.T) {
		t.Log("step 1: validate IS-IS underlay and BGP peering")
		verifyISISAdjacencies(t, dut)
		verifyISISLSDB(t, dut)
		verifyAFTRoute(t, dut, peer2Addr+"/32")
		verifyAFTRoute(t, dut, peer3Addr+"/32")
		verifyBGPNeighbor(t, dut, peer2Addr)
		verifyBGPNeighbor(t, dut, peer3Addr)

		t.Log("step 2: BGP routes from ATE port-2 are in the Loc-RIB")
		awaitLocRIBCount(t, dut, peer2Addr, prefixCount, 2*time.Minute)
		awaitLocRIBCount(t, dut, peer3Addr, 0, time.Minute)
		verifyBGPRoute(t, dut, peer2Addr, "100.64.0.0/24")

		t.Log("step 3: install gRIBI recursive routes")
		programRecursiveRoutes(t, dut, client)
		verifyFIBProgrammed(t, client, prefixCount*3)
		verifyGRIBIRoutes(t, dut)
		verifyGRIBIGet(t, dut, client)
		verifyGRIBIHierarchy(t, dut, "100.68.0.0/24", atePorts[1].IPv4)
		verifyGRIBIHierarchy(t, dut, "100.71.231.0/24", atePorts[1].IPv4)

		t.Log("step 4: baseline traffic")
		otgC.StartTraffic(t)
		trafficStarted = true
		verifyFlow1Egress(t, ate, "port2", "port3")
		assertSteadyLoss(t, otgC, flowName)
		assertSteadyLoss(t, otgC, controlFlow)
	}) {
		return
	}

	if !t.Run("TE_1_4_2_BGPFlapAndTrafficConvergence", func(t *testing.T) {
		t.Log("step 1: flap the BGP routes (port-2 withdraws, port-3 advertises)")
		transitions := map[string]uint64{
			peer2Addr: bgpEstablishedTransitions(t, dut, peer2Addr),
			peer3Addr: bgpEstablishedTransitions(t, dut, peer3Addr),
		}
		txBefore, rxBefore := flowCounters(t, otgC, flowName)
		flapStart := time.Now()
		setRouteState(t, ate, gosnappi.StateProtocolRouteState.ADVERTISE, port3Routes)
		setRouteState(t, ate, gosnappi.StateProtocolRouteState.WITHDRAW, port2Routes)

		t.Log("step 2: validate traffic convergence to ATE port-3")
		awaitLocRIBCount(t, dut, peer3Addr, prefixCount, time.Minute)
		awaitLocRIBCount(t, dut, peer2Addr, 0, time.Minute)
		verifyBGPRoute(t, dut, peer3Addr, "100.64.0.0/24")
		txAfter, rxAfter := flowCounters(t, otgC, flowName)
		assertConvergenceOutage(t, flapStart, txBefore, rxBefore, txAfter, rxAfter)

		verifyGRIBIRoutes(t, dut)
		verifyGRIBIHierarchy(t, dut, "100.68.0.0/24", atePorts[2].IPv4)
		for peer, baseline := range transitions {
			requireBGPEstablished(t, dut, peer, baseline)
		}
		verifyFlow1Egress(t, ate, "port3", "port2")
		assertSteadyLoss(t, otgC, flowName)
		assertSteadyLoss(t, otgC, controlFlow)
	}) {
		return
	}

	if !t.Run("TE_1_4_3_ISISUnderlayFailure", func(t *testing.T) {
		t.Log("step 1: withdraw the active IS-IS underlay route (192.0.2.3/32)")
		setRouteState(t, ate, gosnappi.StateProtocolRouteState.WITHDRAW, port3ISISRte)

		t.Log("step 2: validate route unreachability (100% loss on Flow-1)")
		verifyAFTRouteAbsent(t, dut, peer3Addr+"/32")
		verifyBGPRoutesWithdrawn(t, dut, peer3Addr)
		assertTotalLoss(t, otgC, flowName)
		assertSteadyLoss(t, otgC, controlFlow)
	}) {
		return
	}

	t.Run("TE_1_4_4_BGPRouteChurn", func(t *testing.T) {
		t.Log("step 1: restore the underlay and induce BGP route churn")
		setRouteState(t, ate, gosnappi.StateProtocolRouteState.ADVERTISE, port3ISISRte)
		verifyAFTRoute(t, dut, peer3Addr+"/32")
		verifyBGPNeighbor(t, dut, peer2Addr)
		verifyBGPNeighbor(t, dut, peer3Addr)
		awaitLocRIBCount(t, dut, peer3Addr, prefixCount, 2*time.Minute)
		transitions := map[string]uint64{
			peer2Addr: bgpEstablishedTransitions(t, dut, peer2Addr),
			peer3Addr: bgpEstablishedTransitions(t, dut, peer3Addr),
		}

		for iteration := 1; iteration <= churnIterations; iteration++ {
			t.Logf("churn iteration %d/%d, phase A (port-2 active)", iteration, churnIterations)
			setRouteState(t, ate, gosnappi.StateProtocolRouteState.ADVERTISE, port2Routes)
			setRouteState(t, ate, gosnappi.StateProtocolRouteState.WITHDRAW, port3Routes)
			awaitLocRIBCount(t, dut, peer2Addr, prefixCount, churnConvergenceTimeout)
			awaitLocRIBCount(t, dut, peer3Addr, 0, churnConvergenceTimeout)
			requireBGPEstablished(t, dut, peer2Addr, transitions[peer2Addr])
			requireBGPEstablished(t, dut, peer3Addr, transitions[peer3Addr])

			t.Logf("churn iteration %d/%d, phase B (port-3 active)", iteration, churnIterations)
			setRouteState(t, ate, gosnappi.StateProtocolRouteState.ADVERTISE, port3Routes)
			setRouteState(t, ate, gosnappi.StateProtocolRouteState.WITHDRAW, port2Routes)
			awaitLocRIBCount(t, dut, peer3Addr, prefixCount, churnConvergenceTimeout)
			awaitLocRIBCount(t, dut, peer2Addr, 0, churnConvergenceTimeout)
			requireBGPEstablished(t, dut, peer2Addr, transitions[peer2Addr])
			requireBGPEstablished(t, dut, peer3Addr, transitions[peer3Addr])
		}
		t.Log("step 2: validate stability (sessions ESTABLISHED, Flow-1 on port-3)")
		verifyBGPNeighbor(t, dut, peer2Addr)
		verifyBGPNeighbor(t, dut, peer3Addr)
		awaitLocRIBCount(t, dut, peer3Addr, prefixCount, time.Minute)
		awaitLocRIBCount(t, dut, peer2Addr, 0, time.Minute)
		verifyGRIBIRoutes(t, dut)
		verifyGRIBIGet(t, dut, client)
		verifyGRIBIHierarchy(t, dut, "100.68.0.0/24", atePorts[2].IPv4)
		verifyGRIBIHierarchy(t, dut, "100.71.231.0/24", atePorts[2].IPv4)
		verifyFlow1Egress(t, ate, "port3", "port2")
		assertSteadyLoss(t, otgC, flowName)
		assertSteadyLoss(t, otgC, controlFlow)
		otgC.StopTraffic(t)
		trafficStarted = false
		assertCumulativeControlFlowClean(t, otgC)
		for peer, baseline := range transitions {
			requireBGPEstablished(t, dut, peer, baseline)
		}
	})
}

// configureDUT installs the interfaces, loopback, IS-IS, and BGP configuration.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	for i, port := range dutPorts {
		p := dut.Port(t, fmt.Sprintf("port%d", i+1))
		intf := port.NewOCInterface(p.Name(), dut)
		gnmi.Replace(t, dut, gnmi.OC().Interface(p.Name()).Config(), intf)
		if deviations.ExplicitInterfaceInDefaultVRF(dut) {
			fptest.AssignToNetworkInstance(t, dut, p.Name(), deviations.DefaultNetworkInstance(dut), 0)
		}
	}
	if deviations.ExplicitPortSpeed(dut) {
		for i := range dutPorts {
			fptest.SetPortSpeed(t, dut.Port(t, fmt.Sprintf("port%d", i+1)))
		}
	}

	root := &oc.Root{}
	lo := root.GetOrCreateInterface("Loopback0")
	lo.Type = oc.IETFInterfaces_InterfaceType_softwareLoopback
	if deviations.InterfaceEnabled(dut) {
		lo.Enabled = ygot.Bool(true)
	}
	lo.GetOrCreateSubinterface(0).GetOrCreateIpv4().GetOrCreateAddress(dutLoopback).PrefixLength = ygot.Uint8(32)
	gnmi.Replace(t, dut, gnmi.OC().Interface("Loopback0").Config(), lo)

	configureISIS(t, dut)
	configureBGP(t, dut)
}

// configureISIS configures the DUT's Level-2 IPv4 IS-IS underlay.
func configureISIS(t *testing.T, dut *ondatra.DUTDevice) {
	root := &oc.Root{}
	ni := root.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	protocol := ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisName)
	protocol.Enabled = ygot.Bool(true)
	isis := protocol.GetOrCreateIsis()
	global := isis.GetOrCreateGlobal()
	global.LevelCapability = oc.Isis_LevelType_LEVEL_2
	global.Net = []string{"49.0001.1920.0000.2001.00"}
	global.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	if deviations.ISISInstanceEnabledRequired(dut) {
		global.Instance = ygot.String(isisName)
	}
	level := isis.GetOrCreateLevel(2)
	level.MetricStyle = oc.Isis_MetricStyle_WIDE_METRIC
	if deviations.ISISLevelEnabled(dut) {
		level.Enabled = ygot.Bool(true)
	}
	for _, name := range []string{dut.Port(t, "port2").Name(), dut.Port(t, "port3").Name(), "Loopback0"} {
		if deviations.ExplicitInterfaceInDefaultVRF(dut) || deviations.InterfaceRefInterfaceIDFormat(dut) {
			if name != "Loopback0" {
				name += ".0"
			}
		}
		intf := isis.GetOrCreateInterface(name)
		intf.Enabled = ygot.Bool(true)
		if name == "Loopback0" {
			intf.Passive = ygot.Bool(true)
			continue
		}
		intf.CircuitType = oc.Isis_CircuitType_POINT_TO_POINT
		intfLevel := intf.GetOrCreateLevel(2)
		intfLevel.Enabled = ygot.Bool(true)
		af := intfLevel.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST)
		af.Enabled = ygot.Bool(true)
		af.Metric = ygot.Uint32(10)
		if deviations.ISISInterfaceAfiUnsupported(dut) {
			intf.Af = nil
		}
		if deviations.MissingIsisInterfaceAfiSafiEnable(dut) {
			af.Enabled = nil
		}
	}
	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisName).Config(), protocol)
}

// configureBGP configures the DUT's iBGP peers to both ATE loopbacks.
func configureBGP(t *testing.T, dut *ondatra.DUTDevice) {
	root := &oc.Root{}
	protocol := root.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut)).GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, deviations.DefaultBgpInstanceName(dut))
	protocol.Enabled = ygot.Bool(true)
	bgp := protocol.GetOrCreateBgp()
	global := bgp.GetOrCreateGlobal()
	global.As = ygot.Uint32(dutAS)
	global.RouterId = ygot.String(dutLoopback)
	global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
	for _, peer := range []string{peer2Addr, peer3Addr} {
		n := bgp.GetOrCreateNeighbor(peer)
		n.PeerAs = ygot.Uint32(dutAS)
		n.Enabled = ygot.Bool(true)
		n.GetOrCreateTransport().LocalAddress = ygot.String(dutLoopback)
		n.GetOrCreateTimers().HoldTime = ygot.Uint16(bgpHoldTime)
		n.GetOrCreateTimers().KeepaliveInterval = ygot.Uint16(bgpKeepaliveInterval)
		n.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
	}
	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, deviations.DefaultBgpInstanceName(dut)).Config(), protocol)
}

// configureATE builds the ATE interfaces, routing protocols, route ranges, and traffic.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	top := gosnappi.NewConfig()
	devices := make([]gosnappi.Device, len(atePorts))
	for i, port := range atePorts {
		ap := ate.Port(t, port.Name)
		top.Ports().Add().SetName(ap.ID())
		dev := top.Devices().Add().SetName(port.Name)
		devices[i] = dev
		eth := dev.Ethernets().Add().SetName(port.Name + ".Eth").SetMac(port.MAC)
		eth.Connection().SetPortName(ap.ID())
		eth.Ipv4Addresses().Add().SetName(port.Name + ".IPv4").SetAddress(port.IPv4).
			SetGateway(dutPorts[i].IPv4).SetPrefix(uint32(port.IPv4Len))
	}

	for i := 1; i <= 2; i++ {
		dev := devices[i]
		loopIP := fmt.Sprintf("192.0.2.%d", i+1)
		loop := dev.Ipv4Loopbacks().Add().SetName(fmt.Sprintf("port%d.loopback", i+1)).
			SetEthName(dev.Ethernets().Items()[0].Name()).SetAddress(loopIP)
		isis := dev.Isis().SetName(fmt.Sprintf("port%d.isis", i+1)).SetSystemId(fmt.Sprintf("19200000200%d", i+1))
		isis.Basic().SetIpv4TeRouterId(atePorts[i].IPv4).SetHostname(isis.Name()).SetLearnedLspFilter(true).SetEnableWideMetric(true)
		isis.Advanced().SetAreaAddresses([]string{"490001"})
		isis.Interfaces().Add().SetEthName(dev.Ethernets().Items()[0].Name()).SetName(fmt.Sprintf("port%d.isis.interface", i+1)).
			SetLevelType(gosnappi.IsisInterfaceLevelType.LEVEL_2).SetNetworkType(gosnappi.IsisInterfaceNetworkType.POINT_TO_POINT).SetMetric(10)
		isis.V4Routes().Add().SetName(fmt.Sprintf("port%d.loopback.route", i+1)).SetLinkMetric(10).
			Addresses().Add().SetAddress(loopIP).SetPrefix(32)
		bgp := dev.Bgp().SetRouterId(loopIP)
		peer := bgp.Ipv4Interfaces().Add().SetIpv4Name(loop.Name()).Peers().Add().SetName(fmt.Sprintf("port%d.bgp", i+1))
		peer.SetPeerAddress(dutLoopback).SetAsNumber(dutAS).SetAsType(gosnappi.BgpV4PeerAsType.IBGP)
		// Underlay withdrawal can break only the DUT-to-ATE path; the ATE must
		// also expire the session promptly instead of using its 90-second default.
		peer.Advanced().SetHoldTimeInterval(bgpHoldTime).SetKeepAliveInterval(bgpKeepaliveInterval)
		peer.Capability().SetIpv4Unicast(true)
		peer.GracefulRestart().SetEnableGr(false)
		peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true)
		addBGPRoutes(peer, fmt.Sprintf("port%d.routes", i+1), loopIP)
	}
	configureTraffic(top)
	return top
}

// addBGPRoutes configures the 1,000 /24 prefixes 100.<64+i/256>.<i%256>.0/24
// with the given next hop. Both peers carry the range; which one is currently
// advertised is controlled at runtime with route withdraw/advertise actions.
func addBGPRoutes(peer gosnappi.BgpV4Peer, name, nextHop string) {
	routes := peer.V4Routes().Add().SetName(name).
		SetNextHopIpv4Address(nextHop).
		SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).
		SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)
	routes.Addresses().Add().SetAddress(fmt.Sprintf("100.%d.0.0", bgpBaseSecond)).SetPrefix(24).SetCount(prefixCount)
}

// configureTraffic adds the recursive-route and negative-control traffic flows.
func configureTraffic(top gosnappi.Config) {
	destinations := make([]string, 0, prefixCount)
	for i := range prefixCount {
		destinations = append(destinations, fmt.Sprintf("100.%d.%d.10", gribiBase+i/256, i%256))
	}
	for _, tc := range []struct {
		name    string
		rxNames []string
		pps     uint64
		dsts    []string
	}{
		{name: flowName, rxNames: []string{"port2.IPv4", "port3.IPv4"}, pps: flow1PPS, dsts: destinations},
		{name: controlFlow, rxNames: []string{"port4.IPv4"}, pps: flow2PPS, dsts: []string{atePorts[3].IPv4}},
	} {
		flow := top.Flows().Add().SetName(tc.name)
		flow.Metrics().SetEnable(true)
		flow.Size().SetFixed(frameBytes)
		flow.Rate().SetPps(tc.pps)
		flow.Duration().Continuous()
		flow.TxRx().Device().SetTxNames([]string{"port1.IPv4"}).SetRxNames(tc.rxNames)
		flow.Packet().Add().Ethernet().Src().SetValue(atePorts[0].MAC)
		ip := flow.Packet().Add().Ipv4()
		ip.Src().SetValue(atePorts[0].IPv4)
		ip.Dst().SetValues(tc.dsts)
	}
}

// setRouteState withdraws/advertises named OTG route ranges (BGP or IS-IS)
// without touching the sessions or re-pushing the configuration.
func setRouteState(t *testing.T, ate *ondatra.ATEDevice, state gosnappi.StateProtocolRouteStateEnum, names ...string) {
	t.Helper()
	if len(names) == 0 {
		t.Fatal("setRouteState requires at least one route range name")
	}
	cs := gosnappi.NewControlState()
	cs.Protocol().Route().SetNames(names).SetState(state)
	ate.OTG().SetControlState(t, cs)
}

// flushDefault removes all gRIBI entries from the DUT's default network instance.
func flushDefault(t testing.TB, dut *ondatra.DUTDevice, client *fluent.GRIBIClient) {
	t.Helper()
	ni := deviations.DefaultNetworkInstance(dut)
	if _, err := client.Flush().WithElectionOverride().WithNetworkInstance(ni).Send(); err != nil {
		t.Fatalf("gRIBI Flush on %s failed: %v", ni, err)
	}
}

type cleanupContextTB struct {
	testing.TB
	ctx context.Context
}

// Context returns the bounded context used for cleanup operations.
func (t cleanupContextTB) Context() context.Context {
	return t.ctx
}

// cleanupTestContext creates a one-minute context that remains active after test cancellation.
func cleanupTestContext(t testing.TB) (testing.TB, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(t.Context()), time.Minute)
	return cleanupContextTB{TB: t, ctx: ctx}, cancel
}

// cleanupDUTConfig removes test protocols, the loopback, and subinterfaces without deleting physical interfaces.
func cleanupDUTConfig(t testing.TB, dut *ondatra.DUTDevice) {
	ni := deviations.DefaultNetworkInstance(dut)
	bgpConfig := gnmi.OC().NetworkInstance(ni).
		Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, deviations.DefaultBgpInstanceName(dut)).Config()
	isisConfig := gnmi.OC().NetworkInstance(ni).
		Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisName).Config()
	gnmi.Delete(t, dut, bgpConfig)
	gnmi.Delete(t, dut, isisConfig)
	gnmi.Delete(t, dut, gnmi.OC().Interface("Loopback0").Config())
	for i := range dutPorts {
		port := dut.Port(t, fmt.Sprintf("port%d", i+1))
		if deviations.ExplicitInterfaceInDefaultVRF(dut) {
			gnmi.Delete(t, dut, gnmi.OC().NetworkInstance(ni).Interface(port.Name()).Config())
		}
		gnmi.Delete(t, dut, gnmi.OC().Interface(port.Name()).Subinterface(0).Config())
	}
}

// programRecursiveRoutes installs the 1,000 recursive gRIBI routes and waits for programming.
func programRecursiveRoutes(t *testing.T, dut *ondatra.DUTDevice, client *fluent.GRIBIClient) {
	t.Helper()
	ni := deviations.DefaultNetworkInstance(dut)
	entries := make([]fluent.GRIBIEntry, 0, prefixCount*3)
	for i := range prefixCount {
		entries = append(entries,
			fluent.NextHopEntry().WithNetworkInstance(ni).WithIndex(uint64(i+1)).
				WithIPAddress(fmt.Sprintf("100.%d.%d.10", bgpBaseSecond+i/256, i%256)),
			fluent.NextHopGroupEntry().WithNetworkInstance(ni).WithID(uint64(i+1)).AddNextHop(uint64(i+1), 1),
			fluent.IPv4Entry().WithNetworkInstance(ni).WithPrefix(fmt.Sprintf("100.%d.%d.0/24", gribiBase+i/256, i%256)).
				WithNextHopGroup(uint64(i+1)).WithNextHopGroupNetworkInstance(ni),
		)
	}
	client.Modify().AddEntry(t, entries...)
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	defer cancel()
	if err := client.Await(ctx, t); err != nil {
		t.Fatalf("Timed out waiting for recursive routes to be FIB programmed: %v", err)
	}
}

// verifyFIBProgrammed checks that every one of the AFTOperations was ACKed
// with FIB_PROGRAMMED and that none failed.
func verifyFIBProgrammed(t *testing.T, client *fluent.GRIBIClient, want int) {
	t.Helper()
	fib := 0
	for _, r := range client.Results(t) {
		switch r.ProgrammingResult {
		case aftspb.AFTResult_FIB_PROGRAMMED:
			fib++
		case aftspb.AFTResult_FIB_FAILED:
			t.Errorf("operation %d failed FIB programming: %v", r.OperationID, r.ClientError)
		}
	}
	if fib != want {
		t.Fatalf("got %d FIB_PROGRAMMED ACKs, want %d", fib, want)
	}
}

// verifyISISAdjacencies waits for IS-IS adjacencies on both ATE-facing links to reach UP.
func verifyISISAdjacencies(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	for _, port := range []string{dut.Port(t, "port2").Name(), dut.Port(t, "port3").Name()} {
		if deviations.ExplicitInterfaceInDefaultVRF(dut) || deviations.InterfaceRefInterfaceIDFormat(dut) {
			port += ".0"
		}
		query := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisName).
			Isis().Interface(port).LevelAny().AdjacencyAny().AdjacencyState().State()
		_, ok := gnmi.WatchAll(
			t, dut, query, time.Minute,
			func(v *ygnmi.Value[oc.E_Isis_IsisInterfaceAdjState]) bool {
				state, present := v.Val()
				return present && state == oc.Isis_IsisInterfaceAdjState_UP
			},
		).Await(t)
		if !ok {
			t.Fatalf("IS-IS adjacency on %s did not reach UP", port)
		}
	}
}

// verifyISISLSDB validates that the ATE loopbacks are present in the IS-IS
// link-state database, or, when individual LSP TLVs are unavailable, that the
// expected number of Level-2 LSPs have been originated.
func verifyISISLSDB(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	if deviations.ISISLSPTlvsOCUnsupported(dut) {
		query := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).
			Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisName).
			Isis().Level(2).SystemLevelCounters().TotalLsps().State()
		value, ok := gnmi.Watch(t, dut, query, time.Minute, func(v *ygnmi.Value[uint32]) bool {
			count, present := v.Val()
			return present && count >= minimumLSPCount
		}).Await(t)
		if !ok {
			t.Fatalf("IS-IS total LSP count did not reach at least %d: %v", minimumLSPCount, value)
		}
		t.Log("Validated IS-IS total LSP count instead of individual LSDB prefixes; loopback installation is checked separately in AFT")
		return
	}
	for _, addr := range []string{peer2Addr, peer3Addr} {
		verifyISISLSDBPrefix(t, dut, addr)
	}
}

// verifyISISLSDBPrefix waits for an address to appear in an IS-IS extended IPv4 reachability TLV.
func verifyISISLSDBPrefix(t *testing.T, dut *ondatra.DUTDevice, addr string) {
	t.Helper()
	prefix := addr + "/32"
	for _, key := range []string{prefix, addr} {
		query := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisName).
			Isis().Level(2).LspAny().TlvAny().ExtendedIpv4Reachability().Prefix(key).Prefix().State()
		if _, ok := gnmi.WatchAll(t, dut, query, time.Minute, func(v *ygnmi.Value[string]) bool {
			got, present := v.Val()
			return present && got == key
		}).Await(t); ok {
			return
		}
	}
	t.Fatalf("IS-IS LSDB has no extended-ipv4-reachability entry for %s", prefix)
}

// verifyAFTRoute waits for the specified IPv4 prefix to appear in the DUT AFT.
func verifyAFTRoute(t *testing.T, dut *ondatra.DUTDevice, prefix string) {
	t.Helper()
	path := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Afts().Ipv4Entry(prefix)
	_, ok := gnmi.Watch(t, dut, path.State(), time.Minute, func(v *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv4Entry]) bool {
		entry, present := v.Val()
		return present && entry != nil && entry.GetPrefix() == prefix
	}).Await(t)
	if !ok {
		t.Fatalf("AFT route %s did not become present", prefix)
	}
}

// verifyAFTRouteAbsent waits for the specified IPv4 prefix to disappear from the DUT AFT.
func verifyAFTRouteAbsent(t *testing.T, dut *ondatra.DUTDevice, prefix string) {
	t.Helper()
	path := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Afts().Ipv4Entry(prefix)
	_, ok := gnmi.Watch(t, dut, path.State(), 30*time.Second, func(v *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv4Entry]) bool {
		return !v.IsPresent()
	}).Await(t)
	if !ok {
		t.Fatalf("AFT route %s remained present after its IS-IS advertisement was withdrawn", prefix)
	}
}

// verifyBGPNeighbor waits for the specified BGP peer to reach ESTABLISHED.
func verifyBGPNeighbor(t *testing.T, dut *ondatra.DUTDevice, address string) {
	t.Helper()
	path := gnmi.OC().
		NetworkInstance(deviations.DefaultNetworkInstance(dut)).
		Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, deviations.DefaultBgpInstanceName(dut)).
		Bgp().Neighbor(address)
	_, ok := gnmi.Watch(
		t, dut, path.SessionState().State(), bgpEstablishTimeout,
		func(v *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
			state, present := v.Val()
			return present && state == oc.Bgp_Neighbor_SessionState_ESTABLISHED
		},
	).Await(t)
	if !ok {
		diag := "neighbor state not present on the DUT"
		if neighbor, present := gnmi.Lookup(t, dut, path.State()).Val(); present {
			diag = fmt.Sprintf("session-state %v, established-transitions %d, last-established %d, NOTIFICATIONs received %d sent %d",
				neighbor.GetSessionState(), neighbor.GetEstablishedTransitions(), neighbor.GetLastEstablished(),
				neighbor.GetMessages().GetReceived().GetNOTIFICATION(), neighbor.GetMessages().GetSent().GetNOTIFICATION())
		}
		t.Fatalf("BGP neighbor %s did not reach ESTABLISHED within %v: %s", address, bgpEstablishTimeout, diag)
	}
}

// bgpNeighborState reads the current state of the specified BGP neighbor.
func bgpNeighborState(t *testing.T, dut *ondatra.DUTDevice, address string) *oc.NetworkInstance_Protocol_Bgp_Neighbor {
	t.Helper()
	path := gnmi.OC().
		NetworkInstance(deviations.DefaultNetworkInstance(dut)).
		Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, deviations.DefaultBgpInstanceName(dut)).
		Bgp().Neighbor(address).State()
	return gnmi.Get(t, dut, path)
}

// bgpEstablishedTransitions returns the peer's established-transition counter after validating its state.
func bgpEstablishedTransitions(t *testing.T, dut *ondatra.DUTDevice, address string) uint64 {
	t.Helper()
	neighbor := bgpNeighborState(t, dut, address)
	if got := neighbor.GetSessionState(); got != oc.Bgp_Neighbor_SessionState_ESTABLISHED {
		t.Fatalf("BGP neighbor %s session-state = %v, want ESTABLISHED", address, got)
	}
	if neighbor.EstablishedTransitions == nil {
		t.Fatalf("BGP neighbor %s has no established-transitions counter", address)
	}
	return neighbor.GetEstablishedTransitions()
}

// requireBGPEstablished fails if the session flapped since baseline. The
// failure message carries the neighbor's message counters so a hold-timer
// expiry (no NOTIFICATIONs received) can be told apart from a peer-initiated
// reset.
func requireBGPEstablished(t *testing.T, dut *ondatra.DUTDevice, address string, baseline uint64) {
	t.Helper()
	neighbor := bgpNeighborState(t, dut, address)
	if neighbor.GetSessionState() == oc.Bgp_Neighbor_SessionState_ESTABLISHED && neighbor.GetEstablishedTransitions() == baseline {
		return
	}
	t.Fatalf("BGP neighbor %s flapped during route churn: session-state %v, established-transitions %d (baseline %d), last-established %d, NOTIFICATIONs received %d sent %d",
		address, neighbor.GetSessionState(), neighbor.GetEstablishedTransitions(), baseline, neighbor.GetLastEstablished(),
		neighbor.GetMessages().GetReceived().GetNOTIFICATION(), neighbor.GetMessages().GetSent().GetNOTIFICATION())
}

// verifyBGPRoutesWithdrawn waits until the DUT stops installing the peer's BGP
// routes for forwarding after the IS-IS route to the peer's loopback (the iBGP
// next hop) is withdrawn. Vendors differ in how an underlay failure manifests:
// some tear the iBGP session down and clear the Loc-RIB, while others keep the
// session ESTABLISHED via next-hop tracking and retain the routes in the
// Loc-RIB but withdraw them from the FIB. The neighbor's installed
// IPv4-unicast prefix count dropping to zero (or the afi-safi state being
// removed) is the vendor-neutral proof that the overlay routes are no longer
// usable for forwarding.
func verifyBGPRoutesWithdrawn(t *testing.T, dut *ondatra.DUTDevice, address string) {
	t.Helper()
	ni := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut))
	neighborPath := ni.Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, deviations.DefaultBgpInstanceName(dut)).
		Bgp().Neighbor(address)
	installed := neighborPath.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Prefixes().Installed().State()
	withdrawn := func(v *ygnmi.Value[uint32]) bool {
		count, present := v.Val()
		return !present || count == 0
	}
	if _, ok := gnmi.Watch(t, dut, installed, churnConvergenceTimeout, withdrawn).Await(t); ok {
		return
	}
	if withdrawn(gnmi.Lookup(t, dut, installed)) {
		t.Logf("streamed installed-prefix count for %s did not update; a fresh read confirms the routes are withdrawn from forwarding", address)
		return
	}
	ribPath := ni.Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, deviations.DefaultBgpInstanceName(dut)).
		Bgp().Rib().AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Ipv4Unicast().LocRib().State()
	rib, present := gnmi.Lookup(t, dut, ribPath).Val()
	t.Logf("fresh Loc-RIB lookup: present %t, test prefixes from %s: %d, test prefixes from all peers: %d",
		present, address, locRIBCount(rib, address), locRIBCount(rib, ""))
	count, _ := gnmi.Lookup(t, dut, installed).Val()
	t.Fatalf("BGP neighbor %s still installs %d IPv4-unicast routes after its IS-IS loopback route was withdrawn, want 0", address, count)
}

// verifyBGPRoute waits for prefix, learned from neighbor, to appear in the
// Loc-RIB. It scans the route list instead of addressing the route by its
// (prefix, origin, path-id) list key, because platforms differ in the key they
// use (e.g. Cisco did not accept origin=<peer>, path-id=0), while the origin
// leaf value is consistent and already relied on by locRIBCount.
func verifyBGPRoute(t *testing.T, dut *ondatra.DUTDevice, neighbor, prefix string) {
	t.Helper()
	query := gnmi.OC().
		NetworkInstance(deviations.DefaultNetworkInstance(dut)).
		Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, deviations.DefaultBgpInstanceName(dut)).
		Bgp().Rib().AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).
		Ipv4Unicast().LocRib().State()
	_, ok := gnmi.Watch(
		t, dut, query, time.Minute,
		func(v *ygnmi.Value[*oc.NetworkInstance_Protocol_Bgp_Rib_AfiSafi_Ipv4Unicast_LocRib]) bool {
			rib, present := v.Val()
			if !present || rib == nil {
				return false
			}
			for _, route := range rib.Route {
				if route.GetPrefix() == prefix {
					origin, ok := bgpOriginAddress(route.GetOrigin())
					if ok && origin == neighbor {
						return true
					}
				}
			}
			return false
		},
	).Await(t)
	if !ok {
		t.Fatalf("BGP route %s from peer %s not found in Loc-RIB", prefix, neighbor)
	}
}

// bgpOriginAddress extracts a string-valued peer address from a BGP route origin.
func bgpOriginAddress(value oc.NetworkInstance_Protocol_Bgp_Rib_AfiSafi_Ipv4Unicast_LocRib_Route_Origin_Union) (string, bool) {
	switch origin := value.(type) {
	case oc.UnionString:
		return string(origin), true
	default:
		return "", false
	}
}

// prefixIndex maps 100.<base+i/256>.<i%256>.0/24 back to i, reporting whether
// the prefix belongs to the 1,000-prefix range starting at second octet base.
func prefixIndex(prefix string, base int) (int, bool) {
	p, err := netip.ParsePrefix(prefix)
	if err != nil || p.Bits() != 24 || !p.Addr().Is4() {
		return 0, false
	}
	a := p.Addr().As4()
	if a[0] != 100 || int(a[1]) < base || int(a[1]) >= base+4 {
		return 0, false
	}
	i := (int(a[1])-base)*256 + int(a[2])
	return i, i < prefixCount
}

// locRIBCount counts Loc-RIB routes in the 1,000-prefix BGP range learned from
// origin (the advertising peer). An empty origin counts routes from any peer.
func locRIBCount(rib *oc.NetworkInstance_Protocol_Bgp_Rib_AfiSafi_Ipv4Unicast_LocRib, origin string) int {
	if rib == nil {
		return 0
	}
	seen := map[int]bool{}
	for _, r := range rib.Route {
		i, inRange := prefixIndex(r.GetPrefix(), bgpBaseSecond)
		if !inRange {
			continue
		}
		if origin != "" {
			routeOrigin, ok := bgpOriginAddress(r.GetOrigin())
			if !ok || routeOrigin != origin {
				continue
			}
		}
		seen[i] = true
	}
	return len(seen)
}

// awaitLocRIBCount waits for the expected number of test prefixes from a peer in the Loc-RIB.
func awaitLocRIBCount(t *testing.T, dut *ondatra.DUTDevice, origin string, want int, timeout time.Duration) {
	t.Helper()
	query := gnmi.OC().
		NetworkInstance(deviations.DefaultNetworkInstance(dut)).
		Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, deviations.DefaultBgpInstanceName(dut)).
		Bgp().Rib().AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).
		Ipv4Unicast().LocRib().State()
	got := 0
	_, ok := gnmi.Watch(
		t, dut, query, timeout,
		func(v *ygnmi.Value[*oc.NetworkInstance_Protocol_Bgp_Rib_AfiSafi_Ipv4Unicast_LocRib]) bool {
			rib, _ := v.Val()
			got = locRIBCount(rib, origin)
			return got == want
		},
	).Await(t)
	if ok {
		return
	}
	rib, _ := gnmi.Lookup(t, dut, query).Val()
	fresh := locRIBCount(rib, origin)
	if fresh != want {
		t.Fatalf("Loc-RIB has %d BGP routes from %q (streamed %d), want %d within %v", fresh, origin, got, want, timeout)
	}
	t.Logf("streamed Loc-RIB still showed %d BGP routes from %q, but a fresh Get shows the expected %d", got, origin, want)
}

// verifyGRIBIRoutes waits for all programmed test prefixes to appear in the DUT AFT.
func verifyGRIBIRoutes(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	query := gnmi.OC().
		NetworkInstance(deviations.DefaultNetworkInstance(dut)).
		Afts().State()
	got := 0
	_, ok := gnmi.Watch(t, dut, query, time.Minute, func(v *ygnmi.Value[*oc.NetworkInstance_Afts]) bool {
		afts, _ := v.Val()
		seen := map[int]bool{}
		if afts != nil {
			for _, e := range afts.Ipv4Entry {
				if i, ok := prefixIndex(e.GetPrefix(), gribiBase); ok {
					seen[i] = true
				}
			}
		}
		got = len(seen)
		return got == prefixCount
	}).Await(t)
	if !ok {
		t.Fatalf("AFT has %d of %d gRIBI prefixes programmed", got, prefixCount)
	}
}

// verifyGRIBIGet checks that gRIBI Get returns all programmed test prefixes.
func verifyGRIBIGet(t *testing.T, dut *ondatra.DUTDevice, client *fluent.GRIBIClient) {
	t.Helper()
	ni := deviations.DefaultNetworkInstance(dut)
	resp, err := client.Get().WithNetworkInstance(ni).WithAFT(fluent.IPv4).Send()
	if err != nil {
		t.Fatalf("gRIBI Get on %s failed: %v", ni, err)
	}
	seen := map[int]bool{}
	for _, e := range resp.GetEntry() {
		if i, ok := prefixIndex(e.GetIpv4().GetPrefix(), gribiBase); ok {
			seen[i] = true
		}
	}
	if len(seen) != prefixCount {
		t.Fatalf("gRIBI Get returned %d of %d programmed prefixes", len(seen), prefixCount)
	}
}

// verifyGRIBIHierarchy watches the entire hierarchy so route, NHG, and NH changes
// trigger revalidation even when the gRIBI IPv4 entry itself is unchanged.
func verifyGRIBIHierarchy(t *testing.T, dut *ondatra.DUTDevice, prefix, wantNHIP string) {
	t.Helper()
	path := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Afts()
	opaqueRecursiveNHG := deviations.GRIBIOpaqueRecursiveNHG(dut)
	lastErr := error(nil)
	_, ok := gnmi.Watch(t, dut, path.State(), time.Minute, func(v *ygnmi.Value[*oc.NetworkInstance_Afts]) bool {
		afts, _ := v.Val()
		lastErr = checkGRIBIHierarchy(afts, prefix, wantNHIP, opaqueRecursiveNHG)
		return lastErr == nil
	}).Await(t)
	if !ok {
		t.Fatalf("%s did not resolve to %s: %v", prefix, wantNHIP, lastErr)
	}
}

// checkGRIBIHierarchy follows AFT next-hop references until the expected resolved next hop is found.
func checkGRIBIHierarchy(afts *oc.NetworkInstance_Afts, prefix, wantNHIP string, opaqueRecursiveNHG bool) error {
	if afts == nil {
		return fmt.Errorf("AFT state not present")
	}
	visited := map[string]bool{}
	for {
		if visited[prefix] {
			return fmt.Errorf("recursive AFT resolution loop at %s", prefix)
		}
		visited[prefix] = true
		entry := afts.GetIpv4Entry(prefix)
		if entry == nil {
			return fmt.Errorf("ipv4-entry %s not present in AFT", prefix)
		}
		nhgID := entry.GetNextHopGroup()
		nhg := afts.GetNextHopGroup(nhgID)
		if nhg == nil || nhg.GetId() != nhgID {
			return fmt.Errorf("NHG %d missing or has mismatched id", nhgID)
		}
		nextHop := ""
		if len(nhg.NextHop) == 0 && opaqueRecursiveNHG && len(visited) == 1 {
			i, ok := prefixIndex(prefix, gribiBase)
			if !ok {
				return fmt.Errorf("prefix %s is not a gRIBI test prefix", prefix)
			}
			nextHop = fmt.Sprintf("100.%d.%d.10", bgpBaseSecond+i/256, i%256)
		} else {
			if len(nhg.NextHop) != 1 {
				return fmt.Errorf("NHG %d has %d next hops, want 1", nhgID, len(nhg.NextHop))
			}
			for idx := range nhg.NextHop {
				nh := afts.GetNextHop(idx)
				if nh == nil || nh.GetIndex() != idx {
					return fmt.Errorf("NH %d missing or has mismatched index", idx)
				}
				nextHop = nh.GetIpAddress()
			}
		}
		if nextHop == wantNHIP {
			return nil
		}
		addr, err := netip.ParseAddr(nextHop)
		if err != nil {
			return fmt.Errorf("invalid recursive next hop %q: %w", nextHop, err)
		}
		bestBits := -1
		for candidate := range afts.Ipv4Entry {
			p, err := netip.ParsePrefix(candidate)
			if err == nil && p.Contains(addr) && p.Bits() > bestBits {
				prefix, bestBits = candidate, p.Bits()
			}
		}
		if bestBits < 0 {
			return fmt.Errorf("NH %s has no resolving AFT route, want %s", nextHop, wantNHIP)
		}
	}
}

// flowCounters reads the current transmitted and received packet counts for a flow.
func flowCounters(t *testing.T, o *otg.OTG, name string) (uint64, uint64) {
	t.Helper()
	flow := gnmi.Get(t, o, gnmi.OTG().Flow(name).State())
	if flow.GetCounters() == nil {
		t.Fatalf("flow %s has no OTG counters", name)
	}
	return flow.GetCounters().GetOutPkts(), flow.GetCounters().GetInPkts()
}

// awaitFlowPackets waits until the flow has transmitted a steady-window's worth of packets.
func awaitFlowPackets(t *testing.T, o *otg.OTG, name string, tx0, rx0 uint64) (uint64, uint64) {
	t.Helper()
	rate := uint64(0)
	switch name {
	case flowName:
		rate = flow1PPS
	case controlFlow:
		rate = flow2PPS
	default:
		t.Fatalf("unknown flow %q", name)
	}
	minPackets := rate * uint64(steadyWindow/time.Second)
	tx1, rx1 := tx0, rx0
	decreased := false
	_, ok := gnmi.Watch(t, o, gnmi.OTG().Flow(name).State(), time.Minute, func(v *ygnmi.Value[*otgtelemetry.Flow]) bool {
		flow, present := v.Val()
		if !present || flow == nil || flow.GetCounters() == nil {
			return false
		}
		tx1, rx1 = flow.GetCounters().GetOutPkts(), flow.GetCounters().GetInPkts()
		if tx1 < tx0 || rx1 < rx0 {
			decreased = true
			return true
		}
		return tx1-tx0 >= minPackets
	}).Await(t)
	if decreased {
		t.Fatalf("flow %s counters decreased during measurement: Tx %d -> %d, Rx %d -> %d", name, tx0, tx1, rx0, rx1)
	}
	if !ok {
		t.Fatalf("flow %s transmitted %d packets during measurement, want >= %d", name, tx1-tx0, minPackets)
	}
	return tx1, rx1
}

// packetLossPct computes the loss percentage between two counter snapshots.
func packetLossPct(tx0, rx0, tx1, rx1 uint64) (float64, error) {
	if tx1 < tx0 || rx1 < rx0 {
		return 0, fmt.Errorf("counters decreased: Tx %d -> %d, Rx %d -> %d", tx0, tx1, rx0, rx1)
	}
	dtx, drx := tx1-tx0, rx1-rx0
	if dtx == 0 {
		return 0, fmt.Errorf("no packets transmitted")
	}
	if drx >= dtx {
		if float64(drx-dtx) > float64(dtx)*maxRxExcessFraction {
			return 0, fmt.Errorf("received %d packets for %d transmitted packets", drx, dtx)
		}
		return 0, nil
	}
	return float64(dtx-drx) / float64(dtx) * 100, nil
}

// settledFlowCounters samples the flow until settleSamples arrive and returns
// the Tx/Rx pair from the sample with the smallest in-flight gap (Rx closest to
// Tx). Telemetry can report InPkts lagging OutPkts by a variable amount in any
// single sample; taking the minimum-gap sample yields a coherent snapshot and
// removes that per-sample skew. It never pauses the flow.
func settledFlowCounters(t *testing.T, o *otg.OTG, name string) (uint64, uint64) {
	t.Helper()
	bestTx, bestRx, bestGap, seen := uint64(0), uint64(0), int64(-1), 0
	_, ok := gnmi.Watch(t, o, gnmi.OTG().Flow(name).State(), time.Minute, func(v *ygnmi.Value[*otgtelemetry.Flow]) bool {
		flow, present := v.Val()
		if !present || flow == nil || flow.GetCounters() == nil {
			return false
		}
		tx, rx := flow.GetCounters().GetOutPkts(), flow.GetCounters().GetInPkts()
		gap := int64(tx) - int64(rx)
		if gap < 0 {
			gap = 0
		}
		if bestGap < 0 || gap < bestGap {
			bestTx, bestRx, bestGap = tx, rx, gap
		}
		seen++
		return seen >= settleSamples
	}).Await(t)
	if !ok {
		t.Fatalf("flow %s did not settle: collected %d of %d counter samples before timeout", name, seen, settleSamples)
	}
	if bestGap < 0 {
		t.Fatalf("flow %s produced no counter samples while settling", name)
	}
	return bestTx, bestRx
}

// measureLossPct measures loss over steadyWindow from two settled snapshots of
// the running flow's counters; the flow is never paused or stopped, so Flow-2's
// whole-test counters are preserved. Churn must be finished beforehand. Each
// boundary is settled (minimum Tx-Rx gap) so only a gap that grows across the
// window - genuine loss - is reported, not a one-off counter skew.
func measureLossPct(t *testing.T, o *otg.OTG, name string) float64 {
	t.Helper()
	tx0, rx0 := settledFlowCounters(t, o, name)
	awaitFlowPackets(t, o, name, tx0, rx0)
	tx1, rx1 := settledFlowCounters(t, o, name)
	loss, err := packetLossPct(tx0, rx0, tx1, rx1)
	if err != nil {
		t.Fatalf("flow %s invalid measurement: %v", name, err)
	}
	t.Logf("flow %s measurement: Tx %d, Rx %d, loss %.3f%%", name, tx1-tx0, rx1-rx0, loss)
	return loss
}

// assertSteadyLoss fails if a flow exceeds the permitted loss in a steady-state measurement window.
func assertSteadyLoss(t *testing.T, o *otg.OTG, name string) {
	t.Helper()
	if loss := measureLossPct(t, o, name); loss > steadyLossTolerancePct {
		t.Fatalf("flow %s steady-state loss %.3f%%, want <= %.2f%%", name, loss, steadyLossTolerancePct)
	}
}

// assertTotalLoss fails unless the flow is fully dropped in a steady-state measurement window.
func assertTotalLoss(t *testing.T, o *otg.OTG, name string) {
	t.Helper()
	if loss := measureLossPct(t, o, name); loss < 100-totalLossTolerancePct {
		t.Fatalf("flow %s loss %.3f%%, want 100%%", name, loss)
	}
}

// assertCumulativeControlFlowClean proves Flow-2 saw no collateral loss for
// the whole test, not just in the sampled windows.
func assertCumulativeControlFlowClean(t *testing.T, o *otg.OTG) {
	t.Helper()
	_, stopped := gnmi.Watch(t, o, gnmi.OTG().Flow(controlFlow).Transmit().State(), time.Minute, func(v *ygnmi.Value[bool]) bool {
		transmitting, present := v.Val()
		return present && !transmitting
	}).Await(t)
	if !stopped {
		t.Fatalf("control flow %s did not stop transmitting", controlFlow)
	}
	tx, rx := uint64(0), uint64(0)
	_, ok := gnmi.Watch(t, o, gnmi.OTG().Flow(controlFlow).State(), time.Minute, func(v *ygnmi.Value[*otgtelemetry.Flow]) bool {
		flow, present := v.Val()
		if !present || flow == nil || flow.GetCounters() == nil || flow.GetTransmit() {
			return false
		}
		tx, rx = flow.GetCounters().GetOutPkts(), flow.GetCounters().GetInPkts()
		return tx > 0 && rx >= tx
	}).Await(t)
	if ok {
		return
	}
	if tx == 0 {
		t.Fatalf("control flow %s sent no packets", controlFlow)
	}
	t.Fatalf("control flow %s cumulative loss %.3f%% after traffic stopped, want 0%% (Tx %d, Rx %d)", controlFlow, float64(tx-rx)/float64(tx)*100, tx, rx)
}

// portInFrames returns frames received on an ATE port.
func portInFrames(t *testing.T, ate *ondatra.ATEDevice, port string) uint64 {
	t.Helper()
	return gnmi.Get(t, ate.OTG(), gnmi.OTG().Port(ate.Port(t, port).ID()).Counters().InFrames().State())
}

// verifyFlow1Egress checks that Flow-1 is landing on wantPort and not on
// otherPort (Flow-1's receive set is both ports, so flow loss alone cannot
// tell which path is in use).
func verifyFlow1Egress(t *testing.T, ate *ondatra.ATEDevice, wantPort, otherPort string) {
	t.Helper()
	for attempt := 1; attempt <= egressSampleAttempts; attempt++ {
		w0, o0 := portInFrames(t, ate, wantPort), portInFrames(t, ate, otherPort)
		tx0, rx0 := flowCounters(t, ate.OTG(), flowName)
		tx1, _ := awaitFlowPackets(t, ate.OTG(), flowName, tx0, rx0)
		w1, o1 := portInFrames(t, ate, wantPort), portInFrames(t, ate, otherPort)
		if w1 < w0 || o1 < o0 {
			t.Logf("port counters were reset during sample %d/%d, resampling", attempt, egressSampleAttempts)
			continue
		}
		windowFrames := float64(tx1 - tx0)
		wantMin := uint64(windowFrames * minimumEgressFraction)
		if w1-w0 < wantMin {
			t.Fatalf("Flow-1 egress on %s: %d received frames for %d transmitted packets, want >= %d", wantPort, w1-w0, tx1-tx0, wantMin)
		}
		if o1-o0 > uint64(windowFrames*maxUnexpectedFraction) {
			t.Fatalf("Flow-1 unexpectedly present on %s: %d frames", otherPort, o1-o0)
		}
		return
	}
	t.Fatalf("port counters on %s/%s kept decreasing across %d samples", wantPort, otherPort, egressSampleAttempts)
}

// assertConvergenceOutage computes (Tx - Rx) / frame rate across the flap.
func assertConvergenceOutage(t *testing.T, started time.Time, txBefore, rxBefore, txAfter, rxAfter uint64) {
	t.Helper()
	lostFrames := int64(txAfter-txBefore) - int64(rxAfter-rxBefore)
	if lostFrames < 0 {
		lostFrames = 0
	}
	outage := float64(lostFrames) / float64(flow1PPS)
	t.Logf("BGP recursive-resolution switchover observed over %v: %d lost frames, estimated outage %.3fs", time.Since(started), lostFrames, outage)
	if outage >= maxOutage {
		t.Fatalf("BGP recursive-resolution outage %.3fs, want < %.0fs", outage, maxOutage)
	}
}
