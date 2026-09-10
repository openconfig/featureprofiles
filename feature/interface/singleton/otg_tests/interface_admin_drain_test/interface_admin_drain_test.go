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

// Package interface_admin_drain_test implements RT-5.17: Physical Interface
// Drain via Admin Down. It validates that administratively disabling a
// physical interface tears down its eBGP sessions and IS-IS Level-2
// adjacencies, stops forwarding across that interface, leaves pass-through
// traffic on the other ports unaffected, and that re-enabling the interface
// restores the protocols and traffic. See README.md for the full test plan.
package interface_admin_drain_test

import (
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	otgconfighelpers "github.com/openconfig/featureprofiles/internal/otg_helpers/otg_config_helpers"
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

const (
	dutArea              = "49.0001"
	dutSysID             = "0000.0000.0001"
	ateSysIDPort1        = "0000.0000.0002"
	ateSysIDPort3        = "0000.0000.0003"
	streamOneFlowV4      = "stream1-port1-v4"
	streamOneFlowV6      = "stream1-port1-v6"
	streamTwoFlowV4      = "stream2-port3-v4"
	streamTwoFlowV6      = "stream2-port3-v6"
	port1V4Prefix        = "203.0.113.0/25"
	port1V6Prefix        = "2001:db8:2::/64"
	port3V4Prefix        = "203.0.113.128/25"
	port3V6Prefix        = "2001:db8:3::/64"
	bgpConvergeTimeout   = 90 * time.Second
	isisConvergeTimeout  = 60 * time.Second
	ifaceStatusTimeout   = 30 * time.Second
	ifaceUpTimeout       = 60 * time.Second
	outPktsSettleTimeout = 15 * time.Second
	flowTxTimeout        = 60 * time.Second
	minFlowTxPkts        = 100
)

var allFlows = []string{streamOneFlowV4, streamOneFlowV6, streamTwoFlowV4, streamTwoFlowV6}

// configureATEISIS adds an IS-IS L2 P2P instance to the ATE device at
// bs.ATEIntfs[idx] via the shared otgconfighelpers.ConfigureISIS plugin.
func configureATEISIS(t *testing.T, bs *cfgplugins.BGPSession, idx int, sysID string) {
	t.Helper()
	dev := bs.ATEIntfs[idx]
	ethName := dev.Ethernets().Items()[0].Name()

	otgconfighelpers.ConfigureISIS(t, dev, &otgconfighelpers.ISISAttrs{
		Name:                "isis-" + dev.Name(),
		SystemID:            strings.ReplaceAll(sysID, ".", ""),
		Hostname:            dev.Name(),
		AreaAddresses:       []string{strings.ReplaceAll(dutArea, ".", "")},
		SetLearnedLspFilter: true,
		Interfaces: []*otgconfighelpers.ISISInterfaceAttrs{{
			Name:        "isisInt-" + dev.Name(),
			EthName:     ethName,
			NetworkType: gosnappi.IsisInterfaceNetworkType.POINT_TO_POINT,
			LevelType:   gosnappi.IsisInterfaceLevelType.LEVEL_2,
			Metric:      10,
		}},
	})
}

// advertiseBGPRoutes attaches the given v4/v6 prefixes to the BGP peer
// that cfgplugins.WithEBGP already created on bs.ATEIntfs[idx], using
// otgconfighelpers.AddBGPV4Routes/AddBGPV6Routes.
func advertiseBGPRoutes(t *testing.T, bs *cfgplugins.BGPSession, idx int, v4Prefix, v6Prefix string) {
	t.Helper()
	dev := bs.ATEIntfs[idx]

	v4Peer := dev.Bgp().Ipv4Interfaces().Items()[0].Peers().Items()[0]
	otgconfighelpers.AddBGPV4Routes(v4Peer, dev.Name()+"-v4-routes", []string{v4Prefix})

	v6Peer := dev.Bgp().Ipv6Interfaces().Items()[0].Peers().Items()[0]
	otgconfighelpers.AddBGPV6Routes(v6Peer, dev.Name()+"-v6-routes", []string{v6Prefix})
}

// configureTrafficStreams builds the four unidirectional flows from ATE
// Port 2 (a v4 and v6 flow toward each of port1 and port3) described in the
// README, using otgconfighelpers.Flow.
func configureTrafficStreams(t *testing.T, bs *cfgplugins.BGPSession) {
	t.Helper()
	srcDev := bs.ATEIntfs[1] // port2, ingress traffic generator

	newFlow := func(name, dstDev, dst string, isV6 bool) {
		// OTG device flows bind to IPv4/IPv6 endpoint names, not bare device names.
		suffix := ".IPv4"
		if isV6 {
			suffix = ".IPv6"
		}
		f := &otgconfighelpers.Flow{
			FlowName:  name,
			TxNames:   []string{srcDev.Name() + suffix},
			RxNames:   []string{dstDev + suffix},
			FrameSize: 512,
			Flowrate:  10,
		}
		f.CreateFlow(bs.ATETop)
		f.EthFlow = &otgconfighelpers.EthFlowParams{SrcMAC: bs.ATEPorts[1].MAC}
		f.AddEthHeader()
		if isV6 {
			f.IPv6Flow = &otgconfighelpers.IPv6FlowParams{IPv6Src: bs.ATEPorts[1].IPv6, IPv6Dst: dst}
			f.AddIPv6Header()
		} else {
			f.IPv4Flow = &otgconfighelpers.IPv4FlowParams{IPv4Src: bs.ATEPorts[1].IPv4, IPv4Dst: dst}
			f.AddIPv4Header()
		}
	}

	newFlow(streamOneFlowV4, bs.ATEIntfs[0].Name(), "203.0.113.10", false)
	newFlow(streamOneFlowV6, bs.ATEIntfs[0].Name(), "2001:db8:2::10", true)
	newFlow(streamTwoFlowV4, bs.ATEIntfs[2].Name(), "203.0.113.130", false)
	newFlow(streamTwoFlowV6, bs.ATEIntfs[2].Name(), "2001:db8:3::10", true)
}

// waitForFlowTx blocks until the named OTG flow has transmitted at least
// minPkts packets, so loss is only measured once real traffic has flowed.
func waitForFlowTx(t *testing.T, ate *ondatra.ATEDevice, flowName string, minPkts uint64, timeout time.Duration) {
	t.Helper()
	_, ok := gnmi.Watch(t, ate.OTG(), gnmi.OTG().Flow(flowName).Counters().OutPkts().State(), timeout, func(val *ygnmi.Value[uint64]) bool {
		got, present := val.Val()
		return present && got >= minPkts
	}).Await(t)
	if !ok {
		t.Fatalf("Flow %s did not transmit at least %d packets within %v", flowName, minPkts, timeout)
	}
}

// verifyProtocolsUp confirms the pre-conditions from the README's "Test
// environment setup": both eBGP sessions ESTABLISHED and both IS-IS L2
// adjacencies UP.
func verifyProtocolsUp(t *testing.T, dut *ondatra.DUTDevice, bs *cfgplugins.BGPSession) {
	t.Helper()
	cfgplugins.VerifyDUTBGPEstablished(t, dut, bgpConvergeTimeout)
	cfgplugins.VerifyOTGBGPEstablished(t, bs.ATE, bgpConvergeTimeout)
	cfgplugins.VerifyISISAdjacencyState(t, dut, bs.OndatraDUTPorts[0].Name(), true, isisConvergeTimeout)
	cfgplugins.VerifyISISAdjacencyState(t, dut, bs.OndatraDUTPorts[2].Name(), true, isisConvergeTimeout)
}

// startTraffic resolves ARP/ND on all OTG interfaces and then starts the flows.
func startTraffic(t *testing.T, bs *cfgplugins.BGPSession) {
	t.Helper()
	otg := bs.ATE.OTG()
	otgutils.WaitForARP(t, otg, bs.ATETop, "IPv4")
	otgutils.WaitForARP(t, otg, bs.ATETop, "IPv6")
	otg.StartTraffic(t)
}

// verifyBaseline confirms baseline forwarding: 0% loss on all four flows once
// traffic is running (README "Test environment setup" step 6).
func verifyBaseline(t *testing.T, bs *cfgplugins.BGPSession) {
	t.Helper()
	otg := bs.ATE.OTG()
	for _, flow := range allFlows {
		waitForFlowTx(t, bs.ATE, flow, minFlowTxPkts, flowTxTimeout)
		otgutils.ExpectedTrafficLoss(t, otg, flow, 0, 0)
	}
}

// drainPort1 administratively disables DUT Port 1 (config/enabled=false,
// preserving the rest of the interface config per the README canonical OC)
// and waits for the interface to report admin/oper DOWN. It is shared by
// RT-5.17.1 (the drain under test) and RT-5.17.2 (its drained precondition)
// so each subtest can run independently.
func drainPort1(t *testing.T, dut *ondatra.DUTDevice, bs *cfgplugins.BGPSession) {
	t.Helper()
	p1 := bs.OndatraDUTPorts[0]

	gnmi.Update(t, dut, gnmi.OC().Interface(p1.Name()).Enabled().Config(), false)

	gnmi.Await(t, dut, gnmi.OC().Interface(p1.Name()).Enabled().State(), ifaceStatusTimeout, false)
	waitForAdminStatus(t, dut, p1.Name(), oc.Interface_AdminStatus_DOWN, ifaceStatusTimeout)
	waitForOperStatus(t, dut, p1.Name(), oc.Interface_OperStatus_DOWN, ifaceStatusTimeout)
}

// waitForAdminStatus and waitForOperStatus are small, one-off gnmi.Watch
// calls kept inline rather than factored into a shared helper: they're
// simple, single-path watches specific to this test's own orchestration,
// unlike the BGP/ISIS per-neighbor/per-adjacency watches (which are
// genuinely reusable across similar failure-injection tests and so were
// added to cfgplugins instead).
func waitForAdminStatus(t *testing.T, dut *ondatra.DUTDevice, portName string, want oc.E_Interface_AdminStatus, timeout time.Duration) {
	t.Helper()
	watch := gnmi.Watch(t, dut, gnmi.OC().Interface(portName).AdminStatus().State(), timeout, func(val *ygnmi.Value[oc.E_Interface_AdminStatus]) bool {
		got, ok := val.Val()
		return ok && got == want
	})
	if val, ok := watch.Await(t); !ok {
		t.Fatalf("Interface %s admin-status: got %v, want %v within %v", portName, val, want, timeout)
	}
}

// waitForOperStatus watches the operational status of the specified interface until it matches the desired state or
// the timeout is reached.
func waitForOperStatus(t *testing.T, dut *ondatra.DUTDevice, portName string, want oc.E_Interface_OperStatus, timeout time.Duration) {
	t.Helper()
	watch := gnmi.Watch(t, dut, gnmi.OC().Interface(portName).OperStatus().State(), timeout, func(val *ygnmi.Value[oc.E_Interface_OperStatus]) bool {
		got, ok := val.Val()
		return ok && got == want
	})
	if val, ok := watch.Await(t); !ok {
		t.Fatalf("Interface %s oper-status: got %v, want %v within %v", portName, val, want, timeout)
	}
}

// verifyOutPktsStopped confirms DUT egress on the drained port has ceased:
// after admin-down the out-pkts counter must stop advancing. A frozen counter
// emits no further on-change telemetry, so we watch for any *increase* over the
// settle window and treat the absence of one (watch timeout) as "settled". By
// the time this runs the BGP/IS-IS teardown checks have already elapsed, so the
// post-drain stats flush is complete. The ATE loss check is the authoritative
// drain signal; this confirms it from the DUT side.
func verifyOutPktsStopped(t *testing.T, dut *ondatra.DUTDevice, portName string, timeout time.Duration) {
	t.Helper()
	base := gnmi.Get(t, dut, gnmi.OC().Interface(portName).Counters().OutPkts().State())
	val, ok := gnmi.Watch(t, dut, gnmi.OC().Interface(portName).Counters().OutPkts().State(), timeout, func(val *ygnmi.Value[uint64]) bool {
		got, present := val.Val()
		return present && got > base
	}).Await(t)
	if ok {
		got, _ := val.Val()
		t.Fatalf("Interface %s out-pkts still advancing after admin-down: %d -> %d within %v", portName, base, got, timeout)
	}
	t.Logf("Interface %s out-pkts settled after admin-down at %d", portName, base)
}

// verifyFIBInstalled confirms the un-drained port1 prefixes are re-installed
// into the DUT FIB via AFT telemetry (README RT-5.17.2 step 3).
func verifyFIBInstalled(t *testing.T, dut *ondatra.DUTDevice, v4Prefix, v6Prefix string, timeout time.Duration) {
	t.Helper()
	dni := deviations.DefaultNetworkInstance(dut)

	v4Path := gnmi.OC().NetworkInstance(dni).Afts().Ipv4Entry(v4Prefix)
	if _, ok := gnmi.Watch(t, dut, v4Path.State(), timeout, func(val *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv4Entry]) bool {
		e, present := val.Val()
		return present && e.GetPrefix() == v4Prefix
	}).Await(t); !ok {
		t.Fatalf("Prefix %s not re-installed into FIB within %v", v4Prefix, timeout)
	}
	t.Logf("Prefix %s re-installed into FIB", v4Prefix)

	v6Path := gnmi.OC().NetworkInstance(dni).Afts().Ipv6Entry(v6Prefix)
	if _, ok := gnmi.Watch(t, dut, v6Path.State(), timeout, func(val *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv6Entry]) bool {
		e, present := val.Val()
		return present && e.GetPrefix() == v6Prefix
	}).Await(t); !ok {
		t.Fatalf("Prefix %s not re-installed into FIB within %v", v6Prefix, timeout)
	}
	t.Logf("Prefix %s re-installed into FIB", v6Prefix)
}

// captureBGPLastEstablished returns the neighbor's last-established timestamp,
// or ok=false when the DUT does not report it.
func captureBGPLastEstablished(t *testing.T, dut *ondatra.DUTDevice, neighbor string) (uint64, bool) {
	t.Helper()
	dni := deviations.DefaultNetworkInstance(dut)
	return gnmi.Lookup(t, dut, gnmi.OC().NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, cfgplugins.BGPName).Bgp().Neighbor(neighbor).LastEstablished().State()).Val()
}

// verifyNoBGPFlap fails if the neighbor's last-established timestamp changed
// from the captured baseline, which indicates the session flapped.
func verifyNoBGPFlap(t *testing.T, dut *ondatra.DUTDevice, neighbor string, baseline uint64, have bool) {
	t.Helper()
	if !have {
		t.Logf("BGP neighbor %s last-established unavailable at baseline; skipping no-flap check", neighbor)
		return
	}
	now, ok := captureBGPLastEstablished(t, dut, neighbor)
	if !ok {
		t.Errorf("BGP neighbor %s last-established became unavailable during drain; cannot confirm the session did not flap", neighbor)
		return
	}
	if now != baseline {
		t.Errorf("BGP neighbor %s flapped during drain: last-established %d -> %d", neighbor, baseline, now)
		return
	}
	t.Logf("BGP neighbor %s did not flap during drain", neighbor)
}

// captureISISUpTimestamp returns the up-timestamp of the IS-IS L2 adjacency on
// ifaceName, or ok=false when the DUT does not report it. A change in this
// value across the drain window means the adjacency flapped (README RT-5.17.1).
func captureISISUpTimestamp(t *testing.T, dut *ondatra.DUTDevice, ifaceName string) (uint64, bool) {
	t.Helper()
	if (deviations.ExplicitInterfaceInDefaultVRF(dut) || deviations.InterfaceRefInterfaceIDFormat(dut)) && !strings.Contains(ifaceName, ".") {
		ifaceName += ".0"
	}
	dni := deviations.DefaultNetworkInstance(dut)
	upPath := gnmi.OC().NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, dni).Isis().Interface(ifaceName).Level(2).AdjacencyAny().UpTimestamp().State()
	for _, v := range gnmi.LookupAll(t, dut, upPath) {
		if ts, ok := v.Val(); ok {
			return ts, true
		}
	}
	return 0, false
}

// verifyNoISISFlap fails if the IS-IS adjacency's up-timestamp changed from the
// captured baseline, which indicates the adjacency flapped.
func verifyNoISISFlap(t *testing.T, dut *ondatra.DUTDevice, ifaceName string, baseline uint64, have bool) {
	t.Helper()
	if !have {
		t.Logf("IS-IS adjacency on %s up-timestamp unavailable at baseline; skipping no-flap check", ifaceName)
		return
	}
	now, ok := captureISISUpTimestamp(t, dut, ifaceName)
	if !ok {
		t.Errorf("IS-IS adjacency on %s up-timestamp became unavailable during drain; cannot confirm the adjacency did not flap", ifaceName)
		return
	}
	if now != baseline {
		t.Errorf("IS-IS adjacency on %s flapped during drain: up-timestamp %d -> %d", ifaceName, baseline, now)
		return
	}
	t.Logf("IS-IS adjacency on %s did not flap during drain", ifaceName)
}

// verifyNoRateDegradation confirms the flow is not losing throughput: its
// received frame rate must track its transmitted frame rate (README RT-5.17.1
// step 4, "no rate degradation").
func verifyNoRateDegradation(t *testing.T, ate *ondatra.ATEDevice, flowName string) {
	t.Helper()
	m := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(flowName).State())
	txRate := ygot.BinaryToFloat32(m.GetOutFrameRate())
	rxRate := ygot.BinaryToFloat32(m.GetInFrameRate())
	if txRate == 0 {
		t.Errorf("Flow %s not transmitting; cannot assess rate degradation", flowName)
		return
	}
	if rxRate < txRate*0.99 {
		t.Errorf("Flow %s rate degraded: in-frame-rate %.0f < out-frame-rate %.0f", flowName, rxRate, txRate)
		return
	}
	t.Logf("Flow %s no rate degradation: in-frame-rate %.0f ~ out-frame-rate %.0f", flowName, rxRate, txRate)
}

// waitForFlowRestart waits until the flow's Tx counter has been cleared after
// a stop/start (dropped below its pre-restart value) and then climbed back to
// at least minPkts, so loss is measured over the new window and never the old
// one.
func waitForFlowRestart(t *testing.T, ate *ondatra.ATEDevice, flowName string, priorTx, minPkts uint64, timeout time.Duration) {
	t.Helper()
	sawReset := priorTx == 0
	_, ok := gnmi.Watch(t, ate.OTG(), gnmi.OTG().Flow(flowName).Counters().OutPkts().State(), timeout, func(val *ygnmi.Value[uint64]) bool {
		got, present := val.Val()
		if !present {
			return false
		}
		if !sawReset {
			if got >= priorTx {
				return false
			}
			sawReset = true
		}
		return got >= minPkts
	}).Await(t)
	if !ok {
		t.Fatalf("Flow %s did not reset and re-transmit at least %d packets within %v", flowName, minPkts, timeout)
	}
}

// resetTrafficCounters stops and restarts the OTG flows so their cumulative
// Tx/Rx counters zero out, letting each phase measure loss over its own window
// rather than the whole test run. When resolveNeighbors is true (all ports
// expected up) it re-resolves ARP/ND before restarting; it then blocks until
// each flow's counter has cleared and carried fresh traffic.
func resetTrafficCounters(t *testing.T, bs *cfgplugins.BGPSession, flows []string, resolveNeighbors bool) {
	t.Helper()
	otg := bs.ATE.OTG()

	prior := make(map[string]uint64, len(flows))
	for _, flow := range flows {
		if v, ok := gnmi.Lookup(t, otg, gnmi.OTG().Flow(flow).Counters().OutPkts().State()).Val(); ok {
			prior[flow] = v
		}
	}

	otg.StopTraffic(t)
	if resolveNeighbors {
		otgutils.WaitForARP(t, otg, bs.ATETop, "IPv4")
		otgutils.WaitForARP(t, otg, bs.ATETop, "IPv6")
	}
	otg.StartTraffic(t)

	for _, flow := range flows {
		waitForFlowRestart(t, bs.ATE, flow, prior[flow], minFlowTxPkts, flowTxTimeout)
	}
}

// testAdminDownDrain implements RT-5.17.1: administratively disable Port 1
// and validate that routing protocols tear down, forwarding on Port 1
// ceases, and Port 3 remains unaffected.
func testAdminDownDrain(t *testing.T, dut *ondatra.DUTDevice, bs *cfgplugins.BGPSession) {
	p1 := bs.OndatraDUTPorts[0]
	p3V4Est, p3V4Have := captureBGPLastEstablished(t, dut, bs.ATEPorts[2].IPv4)
	p3V6Est, p3V6Have := captureBGPLastEstablished(t, dut, bs.ATEPorts[2].IPv6)
	p3ISISUp, p3ISISHave := captureISISUpTimestamp(t, dut, bs.OndatraDUTPorts[2].Name())
	drainPort1(t, dut, bs)
	cfgplugins.VerifyBGPNeighborSessionState(t, dut, bs.ATEPorts[0].IPv4, false, bgpConvergeTimeout)
	cfgplugins.VerifyBGPNeighborSessionState(t, dut, bs.ATEPorts[0].IPv6, false, bgpConvergeTimeout)
	cfgplugins.VerifyISISAdjacencyState(t, dut, p1.Name(), false, isisConvergeTimeout)
	cfgplugins.VerifyBGPNeighborSessionState(t, dut, bs.ATEPorts[2].IPv4, true, bgpConvergeTimeout)
	cfgplugins.VerifyBGPNeighborSessionState(t, dut, bs.ATEPorts[2].IPv6, true, bgpConvergeTimeout)
	cfgplugins.VerifyISISAdjacencyState(t, dut, bs.OndatraDUTPorts[2].Name(), true, isisConvergeTimeout)
	verifyNoBGPFlap(t, dut, bs.ATEPorts[2].IPv4, p3V4Est, p3V4Have)
	verifyNoBGPFlap(t, dut, bs.ATEPorts[2].IPv6, p3V6Est, p3V6Have)
	verifyNoISISFlap(t, dut, bs.OndatraDUTPorts[2].Name(), p3ISISUp, p3ISISHave)
	verifyOutPktsStopped(t, dut, p1.Name(), outPktsSettleTimeout)
	resetTrafficCounters(t, bs, allFlows, false)
	otgutils.ExpectedTrafficLoss(t, bs.ATE.OTG(), streamOneFlowV4, 100, 100)
	otgutils.ExpectedTrafficLoss(t, bs.ATE.OTG(), streamOneFlowV6, 100, 100)
	otgutils.ExpectedTrafficLoss(t, bs.ATE.OTG(), streamTwoFlowV4, 0, 0)
	otgutils.ExpectedTrafficLoss(t, bs.ATE.OTG(), streamTwoFlowV6, 0, 0)
	verifyNoRateDegradation(t, bs.ATE, streamTwoFlowV4)
	verifyNoRateDegradation(t, bs.ATE, streamTwoFlowV6)
}

// testUnDrainRestore implements RT-5.17.2: re-enable Port 1 and validate
// full protocol and traffic restoration.
func testUnDrainRestore(t *testing.T, dut *ondatra.DUTDevice, bs *cfgplugins.BGPSession) {
	p1 := bs.OndatraDUTPorts[0]
	drainPort1(t, dut, bs)
	gnmi.Update(t, dut, gnmi.OC().Interface(p1.Name()).Enabled().Config(), true)
	if !deviations.MissingValueForDefaults(dut) {
		gnmi.Await(t, dut, gnmi.OC().Interface(p1.Name()).Enabled().State(), ifaceUpTimeout, true)
	}
	waitForAdminStatus(t, dut, p1.Name(), oc.Interface_AdminStatus_UP, ifaceUpTimeout)
	waitForOperStatus(t, dut, p1.Name(), oc.Interface_OperStatus_UP, ifaceUpTimeout)
	cfgplugins.VerifyBGPNeighborSessionState(t, dut, bs.ATEPorts[0].IPv4, true, bgpConvergeTimeout)
	cfgplugins.VerifyBGPNeighborSessionState(t, dut, bs.ATEPorts[0].IPv6, true, bgpConvergeTimeout)
	cfgplugins.VerifyISISAdjacencyState(t, dut, p1.Name(), true, isisConvergeTimeout)
	verifyFIBInstalled(t, dut, port1V4Prefix, port1V6Prefix, bgpConvergeTimeout)
	resetTrafficCounters(t, bs, allFlows, true)
	for _, flow := range allFlows {
		otgutils.ExpectedTrafficLoss(t, bs.ATE.OTG(), flow, 0, 0)
	}
}

// TestPhysicalInterfaceDrain implements RT-5.17.1 and RT-5.17.2.
func TestPhysicalInterfaceDrain(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	bs := cfgplugins.NewBGPSession(t, cfgplugins.PortCount4, nil)
	bs.WithEBGP(
		t,
		[]oc.E_BgpTypes_AFI_SAFI_TYPE{oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST, oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST},
		[]string{bs.ATEPorts[0].Name, bs.ATEPorts[2].Name},
		false, false,
	)

	isisBatch := &gnmi.SetBatch{}
	cfgplugins.NewISIS(t, dut, &cfgplugins.ISISGlobalParams{
		DUTArea:             dutArea,
		DUTSysID:            dutSysID,
		NetworkInstanceName: deviations.DefaultNetworkInstance(dut),
		ISISInterfaceNames:  []string{bs.OndatraDUTPorts[0].Name(), bs.OndatraDUTPorts[2].Name()},
	}, isisBatch)

	configureATEISIS(t, bs, 0, ateSysIDPort1)
	configureATEISIS(t, bs, 2, ateSysIDPort3)

	advertiseBGPRoutes(t, bs, 0, port1V4Prefix, port1V6Prefix)
	advertiseBGPRoutes(t, bs, 2, port3V4Prefix, port3V6Prefix)

	configureTrafficStreams(t, bs)

	t.Cleanup(func() {
		dni := deviations.DefaultNetworkInstance(dut)
		teardown := &gnmi.SetBatch{}
		gnmi.BatchDelete(teardown, gnmi.OC().NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, cfgplugins.BGPName).Config())
		gnmi.BatchDelete(teardown, gnmi.OC().NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, dni).Config())
		for _, p := range bs.OndatraDUTPorts {
			gnmi.BatchDelete(teardown, gnmi.OC().Interface(p.Name()).Subinterface(0).Config())
			gnmi.BatchUpdate(teardown, gnmi.OC().Interface(p.Name()).Enabled().Config(), true)
			if deviations.ExplicitInterfaceInDefaultVRF(dut) {
				gnmi.BatchDelete(teardown, gnmi.OC().NetworkInstance(dni).Interface(p.Name()+".0").Config())
			}
		}
		teardown.Set(t, dut)
	})

	if err := bs.PushDUT(t); err != nil {
		t.Fatalf("Failed to push DUT config: %v", err)
	}
	isisBatch.Set(t, dut)
	bs.PushAndStartATE(t)

	t.Cleanup(func() {
		bs.ATE.OTG().StopTraffic(t)
		bs.ATE.OTG().StopProtocols(t)
	})

	verifyProtocolsUp(t, dut, bs)
	startTraffic(t, bs)
	verifyBaseline(t, bs)

	t.Run("RT-5.17.1: Physical Interface Admin Down Drain", func(t *testing.T) {
		testAdminDownDrain(t, dut, bs)
	})

	t.Run("RT-5.17.2: Physical Interface Un-drain and Traffic Restoration", func(t *testing.T) {
		testUnDrainRestore(t, dut, bs)
	})
}
