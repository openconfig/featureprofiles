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

// Package zr_800_operational_mode_inventory_recovery_test implements TRANSCEIVER-109:
// 400ZR / 800ZR Operational Mode Transition and Inventory Persistence.
package zr_800_operational_mode_inventory_recovery_test

import (
	"flag"
	"fmt"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/featureprofiles/internal/telemetry/transceiver"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/gnmi/otg"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygnmi/ygnmi"
)

const (
	defaultBaselineOpMode    = uint16(1)
	defaultCustomOpMode      = uint16(2)
	unsupportedOpMode        = uint16(65535)
	defaultFrequencyMHz      = uint64(196100000)
	mismatchedFrequencyMHz   = uint64(193100000)
	defaultTargetPowerdBm    = float64(-7.0)
	laserOffOutputPowerdBm   = float64(-30.0)
	convergenceTimeout       = 5 * time.Minute
	negativeCheckTimeout     = 2 * time.Minute
	stressDeleteCycles       = 5
	staticRouteScalePerPort  = 16
	trafficFrameSizeBytes    = 512
	trafficRatePPS           = 10000
	trafficFixedPackets      = 100000
	expectedUnicastDeltaPort = uint64(200000)
	ipv4PrefixLen            = 30
	ipv6PrefixLen            = 126
	flow1AName               = "Flow_1A_IPv4_Port1_to_Port2"
	flow1BName               = "Flow_1B_IPv4_Port2_to_Port1"
	flow2AName               = "Flow_2A_IPv6_Port1_to_Port2"
	flow2BName               = "Flow_2B_IPv6_Port2_to_Port1"
)

var (
	operationalModes    cfgplugins.OperationalModeList
	frequencies         cfgplugins.FrequencyList
	targetOpticalPowers cfgplugins.TargetOpticalPowerList

	dutPort1 = attrs.Attributes{
		Desc:    "dutPort1",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::1",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
	atePort1 = attrs.Attributes{
		Name:    "atePort1",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::2",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
	dutPort2 = attrs.Attributes{
		Desc:    "dutPort2",
		IPv4:    "192.0.2.5",
		IPv6:    "2001:db8::5",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
	atePort2 = attrs.Attributes{
		Name:    "atePort2",
		MAC:     "02:00:02:01:01:01",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::6",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
)

// init registers CLI flags for optical operational modes, frequency, and target output power.
// README.md Section: Procedure - Test environment setup (used across TRANSCEIVER-109.1, 109.2, 109.3).
// Technical Summary: Allows overriding default 400ZR/800ZR optical-channel parameters at runtime.
func init() {
	flag.Var(&operationalModes, "operational_mode", "Operational modes (baseline, custom) for the optical channel.")
	flag.Var(&frequencies, "frequency", "Frequency in MHz for the optical channel.")
	flag.Var(&targetOpticalPowers, "target_optical_power", "Target optical power in dBm for the optical channel.")
}

// setupEnvironment initializes the Ondatra test framework and reserves the testbed topology.
// README.md Section: Testbed type & Procedure - Test environment setup.
// Technical Summary: Calls fptest.RunTests(m) to bind DUT/ATE devices and gNMI clients.
func setupEnvironment(m *testing.M) {
	fptest.RunTests(m)
}

// TestMain is the entry point for executing the TRANSCEIVER-109 test suite.
// README.md Section: Plan ID TRANSCEIVER-109 (TRANSCEIVER-109.1, TRANSCEIVER-109.2, TRANSCEIVER-109.3).
// Technical Summary: Starts the test runner via setupEnvironment(m).
func TestMain(m *testing.M) {
	setupEnvironment(m)
}

// portCountersSnapshot stores a snapshot of interface packet and error counters plus OTN FEC uncorrectable blocks.
type portCountersSnapshot struct {
	inUnicastPkts  uint64
	outUnicastPkts uint64
	inPkts         uint64
	outPkts        uint64
	inErrors       uint64
	outErrors      uint64
	inFCSErrors    uint64
	fecUncorrect   uint64
}

// resolveTestParameters selects the baseline/custom operational modes, frequency, and target power.
// README.md Section: Procedure - Test environment setup ("Hardware & Coherent Optics Discovery").
// Technical Summary: Uses CLI flags or cfgplugins defaults (fallback: mode 1 -> 2, 196100000 MHz, -7.0 dBm).
func resolveTestParameters(t *testing.T, dut *ondatra.DUTDevice) (baselineMode, customMode uint16, freq uint64, targetPower float64) {
	t.Helper()
	baselineMode = defaultBaselineOpMode
	customMode = defaultCustomOpMode
	if len(operationalModes) >= 2 {
		baselineMode = operationalModes[0]
		customMode = operationalModes[1]
	} else if len(operationalModes) == 1 {
		customMode = operationalModes[0]
	}

	freq = defaultFrequencyMHz
	if len(frequencies) > 0 {
		freq = frequencies[0]
	} else if defFreqs := frequencies.Default(t, dut); len(defFreqs) > 0 {
		freq = defFreqs[0]
	}

	targetPower = defaultTargetPowerdBm
	if len(targetOpticalPowers) > 0 {
		targetPower = targetOpticalPowers[0]
	} else if defPowers := targetOpticalPowers.Default(t, dut); len(defPowers) > 0 {
		targetPower = defPowers[0]
	}
	return baselineMode, customMode, freq, targetPower
}

// ipv4StaticPrefixes builds 16 IPv4 /24 static route prefixes and destination IPs for a port direction.
// README.md Section: Procedure - Test environment setup ("L3 Interface, Static Routing, and Traffic Flow Configuration").
// Technical Summary: Generates 198.51.100.0/24..115.0/24 (to Port 2) or 203.0.113.0/24..128.0/24 (to Port 1).
func ipv4StaticPrefixes(towardsPort2 bool) (prefixes []string, dstIPs []string) {
	for i := 0; i < staticRouteScalePerPort; i++ {
		if towardsPort2 {
			// 198.51.100.0/24 + i (16 prefixes)
			prefixes = append(prefixes, fmt.Sprintf("198.51.%d.0/24", 100+i))
			dstIPs = append(dstIPs, fmt.Sprintf("198.51.%d.1", 100+i))
		} else {
			// 203.0.113.0/24 + i (16 prefixes)
			prefixes = append(prefixes, fmt.Sprintf("203.0.%d.0/24", 113+i))
			dstIPs = append(dstIPs, fmt.Sprintf("203.0.%d.1", 113+i))
		}
	}
	return prefixes, dstIPs
}

// ipv6StaticPrefixes builds 16 IPv6 /64 static route prefixes and destination IPs for a port direction.
// README.md Section: Procedure - Test environment setup ("L3 Interface, Static Routing, and Traffic Flow Configuration").
// Technical Summary: Generates 2001:db8:2::/64..2:f::/64 (to Port 2) or 2001:db8:1::/64..1:f::/64 (to Port 1).
func ipv6StaticPrefixes(towardsPort2 bool) (prefixes []string, dstIPs []string) {
	subnetGroup := 1
	if towardsPort2 {
		subnetGroup = 2
	}
	for i := 0; i < staticRouteScalePerPort; i++ {
		if i == 0 {
			prefixes = append(prefixes, fmt.Sprintf("2001:db8:%d::/64", subnetGroup))
			dstIPs = append(dstIPs, fmt.Sprintf("2001:db8:%d::1", subnetGroup))
		} else {
			prefixes = append(prefixes, fmt.Sprintf("2001:db8:%d:%x::/64", subnetGroup, i))
			dstIPs = append(dstIPs, fmt.Sprintf("2001:db8:%d:%x::1", subnetGroup, i))
		}
	}
	return prefixes, dstIPs
}

// configureL3AndStaticRoutes configures DUT L3 subinterface IPs and 64 static routes in one gNMI batch.
// README.md Section: Test environment setup, TRANSCEIVER-109.1 (Step 4), 109.2 (Step 3), and 109.3 (Steps 2 & 3).
// Technical Summary: Applies dual-stack IPs on port1/port2 and installs 32 IPv4 + 32 IPv6 static routes in DEFAULT VRF.
func configureL3AndStaticRoutes(t *testing.T, dut *ondatra.DUTDevice, p1, p2 *ondatra.Port, portAttrs map[string]*attrs.Attributes) {
	t.Helper()
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	v4ToP2, _ := ipv4StaticPrefixes(true)
	v4ToP1, _ := ipv4StaticPrefixes(false)
	v6ToP2, _ := ipv6StaticPrefixes(true)
	v6ToP1, _ := ipv6StaticPrefixes(false)

	batch := &gnmi.SetBatch{}
	cfgplugins.BatchConfigureL3AndStaticRoutes(t, dut, batch, cfgplugins.L3StaticRouteConfig{
		Ports:     []*ondatra.Port{p1, p2},
		PortAttrs: portAttrs,
		RouteGroups: []cfgplugins.StaticRouteGroup{
			{Prefixes: v4ToP2, NextHopIP: atePort2.IPv4},
			{Prefixes: v4ToP1, NextHopIP: atePort1.IPv4},
			{Prefixes: v6ToP2, NextHopIP: atePort2.IPv6},
			{Prefixes: v6ToP1, NextHopIP: atePort1.IPv6},
		},
	})
	batch.Set(t, dut)

	cfgplugins.AssignPortsToDefaultNetworkInstance(t, dut, []*ondatra.Port{p1, p2})
}

// configureOTG configures ATE ports, dual-stack IPs, and the 4 bidirectional IPv4/IPv6 traffic flows.
// README.md Section: Test environment setup ("L3 Interface, Static Routing, and Traffic Flow Configuration") & TRANSCEIVER-109.1 (Step 4).
// Technical Summary: Pushes OTG config for Flow 1A/1B (IPv4) and Flow 2A/2B (IPv6) and starts ARP/ND protocols.
func configureOTG(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	t.Helper()
	top := gosnappi.NewConfig()

	ap1 := ate.Port(t, "port1")
	ap2 := ate.Port(t, "port2")
	atePort1.AddToOTG(top, ap1, &dutPort1)
	atePort2.AddToOTG(top, ap2, &dutPort2)

	_, v4DstToP2 := ipv4StaticPrefixes(true)
	_, v4DstToP1 := ipv4StaticPrefixes(false)
	_, v6DstToP2 := ipv6StaticPrefixes(true)
	_, v6DstToP1 := ipv6StaticPrefixes(false)

	addIPv4Flow(top, flow1AName, fmt.Sprintf("%s.IPv4", atePort1.Name), fmt.Sprintf("%s.IPv4", atePort2.Name), atePort1.MAC, atePort1.IPv4, v4DstToP2)
	addIPv4Flow(top, flow1BName, fmt.Sprintf("%s.IPv4", atePort2.Name), fmt.Sprintf("%s.IPv4", atePort1.Name), atePort2.MAC, atePort2.IPv4, v4DstToP1)
	addIPv6Flow(top, flow2AName, fmt.Sprintf("%s.IPv6", atePort1.Name), fmt.Sprintf("%s.IPv6", atePort2.Name), atePort1.MAC, atePort1.IPv6, v6DstToP2)
	addIPv6Flow(top, flow2BName, fmt.Sprintf("%s.IPv6", atePort2.Name), fmt.Sprintf("%s.IPv6", atePort1.Name), atePort2.MAC, atePort2.IPv6, v6DstToP1)

	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	return top
}

// addIPv4Flow adds an IPv4 traffic flow to the OTG configuration.
// README.md Section: Test environment setup (Flow 1A & Flow 1B), TRANSCEIVER-109.1 (Step 4), 109.2 (Step 3), 109.3 (Steps 2 & 3).
// Technical Summary: Configures a 512-byte, 10,000 pps, 100,000-packet IPv4 stream across 16 destination prefixes.
func addIPv4Flow(top gosnappi.Config, name, txEndpoint, rxEndpoint, srcMAC, srcIP string, dstIPs []string) {
	f := top.Flows().Add().SetName(name)
	f.Metrics().SetEnable(true)
	f.TxRx().Device().SetTxNames([]string{txEndpoint}).SetRxNames([]string{rxEndpoint})
	f.Size().SetFixed(trafficFrameSizeBytes)
	f.Rate().SetPps(trafficRatePPS)
	f.Duration().FixedPackets().SetPackets(trafficFixedPackets)

	eth := f.Packet().Add().Ethernet()
	eth.Src().SetValue(srcMAC)

	ip := f.Packet().Add().Ipv4()
	ip.Src().SetValue(srcIP)
	ip.Dst().SetValues(dstIPs)
}

// addIPv6Flow adds an IPv6 traffic flow to the OTG configuration.
// README.md Section: Test environment setup (Flow 2A & Flow 2B), TRANSCEIVER-109.1 (Step 4), 109.2 (Step 3), 109.3 (Steps 2 & 3).
// Technical Summary: Configures a 512-byte, 10,000 pps, 100,000-packet IPv6 stream across 16 destination prefixes.
func addIPv6Flow(top gosnappi.Config, name, txEndpoint, rxEndpoint, srcMAC, srcIP string, dstIPs []string) {
	f := top.Flows().Add().SetName(name)
	f.Metrics().SetEnable(true)
	f.TxRx().Device().SetTxNames([]string{txEndpoint}).SetRxNames([]string{rxEndpoint})
	f.Size().SetFixed(trafficFrameSizeBytes)
	f.Rate().SetPps(trafficRatePPS)
	f.Duration().FixedPackets().SetPackets(trafficFixedPackets)

	eth := f.Packet().Add().Ethernet()
	eth.Src().SetValue(srcMAC)

	ip := f.Packet().Add().Ipv6()
	ip.Src().SetValue(srcIP)
	ip.Dst().SetValues(dstIPs)
}

// verifyControlPlaneConvergenceGate verifies L3 subinterface IPs, ARP/ND neighbors, and AFT routes without sleep timers.
// README.md Section: Test environment setup ("Event-Driven Control-Plane & AFT Convergence Gate"), TRANSCEIVER-109.1 (Step 4), 109.2 (Step 3), 109.3 (Steps 2 & 3).
// Technical Summary: Uses gnmi.Watch().Await() to confirm IP state, neighbor resolution, and all 64 AFT prefixes before traffic.
func verifyControlPlaneConvergenceGate(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, top gosnappi.Config, p1, p2 *ondatra.Port) {
	t.Helper()
	// 1. Verify subinterface IPv4 and IPv6 addresses are active in state via ON_CHANGE watch.
	for _, tc := range []struct {
		port *ondatra.Port
		attr attrs.Attributes
	}{
		{port: p1, attr: dutPort1},
		{port: p2, attr: dutPort2},
	} {
		var ok bool
		for attempt := 0; attempt < 3; attempt++ {
			if msg := testt.CaptureFatal(t, func(tb testing.TB) {
				_, ok = gnmi.Watch(tb, dut, gnmi.OC().Interface(tc.port.Name()).Subinterface(0).Ipv4().Address(tc.attr.IPv4).Ip().State(), convergenceTimeout, func(val *ygnmi.Value[string]) bool {
					v, present := val.Val()
					return present && v == tc.attr.IPv4
				}).Await(tb)
			}); msg == nil {
				break
			}
		}
		if !ok {
			t.Errorf("Port %s IPv4 address %s not active in state", tc.port.Name(), tc.attr.IPv4)
		}
		ok = false
		for attempt := 0; attempt < 3; attempt++ {
			if msg := testt.CaptureFatal(t, func(tb testing.TB) {
				_, ok = gnmi.Watch(tb, dut, gnmi.OC().Interface(tc.port.Name()).Subinterface(0).Ipv6().Address(tc.attr.IPv6).Ip().State(), convergenceTimeout, func(val *ygnmi.Value[string]) bool {
					v, present := val.Val()
					return present && v == tc.attr.IPv6
				}).Await(tb)
			}); msg == nil {
				break
			}
		}
		if !ok {
			t.Errorf("Port %s IPv6 address %s not active in state", tc.port.Name(), tc.attr.IPv6)
		}
	}

	if ate != nil && top != nil {
		// 2. Verify OTG neighbor resolution using standard otgutils.WaitForARP helper for both IPv4 and IPv6 before starting traffic.
		otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")
		otgutils.WaitForARP(t, ate.OTG(), top, "IPv6")

		// 3. Verify DUT ARP and ND neighbor link-layer-address entries via ON_CHANGE watch.
		if !deviations.IPNeighborMissing(dut) {
			for _, tc := range []struct {
				port     *ondatra.Port
				peerIPv4 string
				peerIPv6 string
			}{
				{port: p1, peerIPv4: atePort1.IPv4, peerIPv6: atePort1.IPv6},
				{port: p2, peerIPv4: atePort2.IPv4, peerIPv6: atePort2.IPv6},
			} {
				_, ok := gnmi.Watch(t, dut, gnmi.OC().Interface(tc.port.Name()).Subinterface(0).Ipv4().Neighbor(tc.peerIPv4).LinkLayerAddress().State(), convergenceTimeout, func(val *ygnmi.Value[string]) bool {
					v, present := val.Val()
					return present && v != ""
				}).Await(t)
				if !ok {
					t.Errorf("Port %s IPv4 ARP neighbor %s link-layer-address not resolved", tc.port.Name(), tc.peerIPv4)
				}
				_, ok = gnmi.Watch(t, dut, gnmi.OC().Interface(tc.port.Name()).Subinterface(0).Ipv6().Neighbor(tc.peerIPv6).LinkLayerAddress().State(), convergenceTimeout, func(val *ygnmi.Value[string]) bool {
					v, present := val.Val()
					return present && v != ""
				}).Await(t)
				if !ok {
					t.Errorf("Port %s IPv6 ND neighbor %s link-layer-address not resolved", tc.port.Name(), tc.peerIPv6)
				}
			}
		}
	}

	// 4. Verify all 32 IPv4 and 32 IPv6 static route prefixes are installed in AFT using a single batched watch.
	niName := deviations.DefaultNetworkInstance(dut)
	v4ToP2, _ := ipv4StaticPrefixes(true)
	v4ToP1, _ := ipv4StaticPrefixes(false)
	v6ToP2, _ := ipv6StaticPrefixes(true)
	v6ToP1, _ := ipv6StaticPrefixes(false)
	transceiver.VerifyAFTPrefixesConverged(t, dut, niName, append(v4ToP2, v4ToP1...), append(v6ToP2, v6ToP1...), convergenceTimeout)
}

// snapshotPortCounters captures a point-in-time snapshot of DUT interface packet/error counters and OTN FEC blocks.
// README.md Section: TRANSCEIVER-109.1 (Step 4), TRANSCEIVER-109.2 (Step 3), and TRANSCEIVER-109.3 (Steps 2 & 3).
// Technical Summary: Reads unicast/total packets, interface errors, FCS errors, and fec-uncorrectable-blocks via gnmi.Lookup.
func snapshotPortCounters(t *testing.T, dut *ondatra.DUTDevice, p *ondatra.Port, params *cfgplugins.ConfigParameters) portCountersSnapshot {
	t.Helper()
	cnt, _ := gnmi.Lookup(t, dut, gnmi.OC().Interface(p.Name()).Counters().State()).Val()
	var fcsErr uint64
	if !deviations.InterfaceCountersInFcsErrorsUnsupported(dut) {
		fcsErr, _ = gnmi.Lookup(t, dut, gnmi.OC().Interface(p.Name()).Counters().InFcsErrors().State()).Val()
	}
	otnIdx := params.OTNIndexes[p.Name()]
	fecUncorrect, _ := gnmi.Lookup(t, dut, gnmi.OC().TerminalDevice().Channel(otnIdx).Otn().FecUncorrectableBlocks().State()).Val()

	return portCountersSnapshot{
		inUnicastPkts:  cnt.GetInUnicastPkts(),
		outUnicastPkts: cnt.GetOutUnicastPkts(),
		inPkts:         cnt.GetInPkts(),
		outPkts:        cnt.GetOutPkts(),
		inErrors:       cnt.GetInErrors(),
		outErrors:      cnt.GetOutErrors(),
		inFCSErrors:    fcsErr,
		fecUncorrect:   fecUncorrect,
	}
}

// runAndVerifyBidirectionalTraffic runs the 4 dual-stack traffic flows and checks zero packet loss and zero DUT errors.
// README.md Section: TRANSCEIVER-109.1 (Step 4), TRANSCEIVER-109.2 (Step 3), and TRANSCEIVER-109.3 (Steps 2 & 3).
// Technical Summary: Verifies 100,000 tx/rx packets per flow, >= +200,000 packet deltas per port, and 0 interface/FEC errors.
func runAndVerifyBidirectionalTraffic(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, top gosnappi.Config, p1, p2 *ondatra.Port, params *cfgplugins.ConfigParameters) {
	t.Helper()
	beforeP1 := snapshotPortCounters(t, dut, p1, params)
	beforeP2 := snapshotPortCounters(t, dut, p2, params)

	var trafficStarted bool
	if ate != nil && top != nil {
		ate.OTG().StartTraffic(t)
		trafficStarted = true
		defer func() {
			if trafficStarted {
				ate.OTG().StopTraffic(t)
			}
		}()
		flowNames := []string{flow1AName, flow1BName, flow2AName, flow2BName}
		for _, fn := range flowNames {
			_, ok := gnmi.Watch(t, ate.OTG(), gnmi.OTG().Flow(fn).State(), convergenceTimeout, func(val *ygnmi.Value[*otg.Flow]) bool {
				f, present := val.Val()
				return present && f != nil && !f.GetTransmit() && f.GetCounters().GetOutPkts() == trafficFixedPackets
			}).Await(t)
			if !ok {
				t.Errorf("Traffic flow %s did not finish transmitting %d packets within %v", fn, trafficFixedPackets, convergenceTimeout)
			}
		}
		ate.OTG().StopTraffic(t)
		trafficStarted = false
		otgutils.LogFlowMetrics(t, ate.OTG(), top)
		otgutils.LogPortMetrics(t, ate.OTG(), top)

		for _, fn := range flowNames {
			flowCounters := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(fn).Counters().State())
			txPkts := flowCounters.GetOutPkts()
			rxPkts := flowCounters.GetInPkts()
			if txPkts != trafficFixedPackets || rxPkts != trafficFixedPackets {
				t.Errorf("Flow %s packet count mismatch: got tx=%d, rx=%d, want tx=%d, rx=%d (0%% loss)", fn, txPkts, rxPkts, trafficFixedPackets, trafficFixedPackets)
			}
		}
	}

	for _, tc := range []struct {
		port   *ondatra.Port
		before portCountersSnapshot
	}{
		{port: p1, before: beforeP1},
		{port: p2, before: beforeP2},
	} {
		if ate != nil && top != nil {
			// Wait via gnmi.Watch for interface counters (in-unicast-pkts, out-unicast-pkts, in-pkts, out-pkts) to reflect >= +200,000 packet delta.
			_, ok := gnmi.Watch(t, dut, gnmi.OC().Interface(tc.port.Name()).Counters().State(), negativeCheckTimeout, func(val *ygnmi.Value[*oc.Interface_Counters]) bool {
				c, present := val.Val()
				if !present || c == nil {
					return false
				}
				return (c.GetInUnicastPkts()-tc.before.inUnicastPkts) >= expectedUnicastDeltaPort &&
					(c.GetOutUnicastPkts()-tc.before.outUnicastPkts) >= expectedUnicastDeltaPort &&
					(c.GetInPkts()-tc.before.inPkts) >= expectedUnicastDeltaPort &&
					(c.GetOutPkts()-tc.before.outPkts) >= expectedUnicastDeltaPort
			}).Await(t)
			if !ok {
				t.Errorf("Port %s counter watch timed out waiting for +%d packet delta within %v", tc.port.Name(), expectedUnicastDeltaPort, negativeCheckTimeout)
			}
		}

		after := snapshotPortCounters(t, dut, tc.port, params)
		if ate != nil && top != nil {
			if delta := after.inUnicastPkts - tc.before.inUnicastPkts; delta < expectedUnicastDeltaPort {
				t.Errorf("Port %s in-unicast-pkts delta = %d, want >= %d", tc.port.Name(), delta, expectedUnicastDeltaPort)
			}
			if delta := after.outUnicastPkts - tc.before.outUnicastPkts; delta < expectedUnicastDeltaPort {
				t.Errorf("Port %s out-unicast-pkts delta = %d, want >= %d", tc.port.Name(), delta, expectedUnicastDeltaPort)
			}
			if delta := after.inPkts - tc.before.inPkts; delta < expectedUnicastDeltaPort {
				t.Errorf("Port %s in-pkts delta = %d, want >= %d", tc.port.Name(), delta, expectedUnicastDeltaPort)
			}
			if delta := after.outPkts - tc.before.outPkts; delta < expectedUnicastDeltaPort {
				t.Errorf("Port %s out-pkts delta = %d, want >= %d", tc.port.Name(), delta, expectedUnicastDeltaPort)
			}
		}
		if delta := after.inErrors - tc.before.inErrors; delta != 0 {
			t.Errorf("Port %s in-errors delta = %d, want 0", tc.port.Name(), delta)
		}
		if delta := after.outErrors - tc.before.outErrors; delta != 0 {
			t.Errorf("Port %s out-errors delta = %d, want 0", tc.port.Name(), delta)
		}
		if !deviations.InterfaceCountersInFcsErrorsUnsupported(dut) {
			if delta := after.inFCSErrors - tc.before.inFCSErrors; delta != 0 {
				t.Errorf("Port %s in-fcs-errors delta = %d, want 0", tc.port.Name(), delta)
			}
		}
		if !deviations.NonIntervalFecErrorCounter(dut) {
			if after.fecUncorrect != 0 {
				t.Errorf("Port %s fec-uncorrectable-blocks = %d, want 0", tc.port.Name(), after.fecUncorrect)
			}
		} else if delta := after.fecUncorrect - tc.before.fecUncorrect; delta != 0 {
			t.Errorf("Port %s fec-uncorrectable-blocks delta = %d, want 0", tc.port.Name(), delta)
		}
	}
}

// Test800ZROperationalModeInventoryRecovery runs the end-to-end 400ZR/800ZR operational mode and inventory recovery suite.
// README.md Section: Plan ID TRANSCEIVER-109 (executes TRANSCEIVER-109.1, TRANSCEIVER-109.2, TRANSCEIVER-109.3, and Cleanup).
// Technical Summary: Sets up DUT/ATE ports and parameters, registers cleanup, and executes all three subtests sequentially.
func Test800ZROperationalModeInventoryRecovery(t *testing.T) {
	t.Log("Starting TRANSCEIVER-109: 400ZR / 800ZR Operational Mode Transition and Inventory Persistence")
	dut := ondatra.DUT(t, "dut")
	ate, hasATE := ondatra.ATEs(t)["ate"]

	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")
	ports := []*ondatra.Port{p1, p2}
	portAttrs := map[string]*attrs.Attributes{
		p1.Name(): &dutPort1,
		p2.Name(): &dutPort2,
	}

	baselineOpMode, customOpMode, freqMHz, targetPowerdBm := resolveTestParameters(t, dut)
	params := &cfgplugins.ConfigParameters{
		Enabled:            true,
		Frequency:          freqMHz,
		TargetOpticalPower: targetPowerdBm,
		OperationalMode:    baselineOpMode,
	}

	// t.Cleanup restores baseline optical-channel settings, removes L3 subinterfaces/static routes, and verifies ports recover.
	// README.md Section: Procedure - Cleanup.
	// Technical Summary: Reverts operational-mode/frequency/power via gNMI.SetBatch, bounces transceivers, and awaits OperStatus UP.
	t.Cleanup(func() {
		cleanupBatch := &gnmi.SetBatch{}
		params.Enabled = true
		params.Frequency = freqMHz
		params.TargetOpticalPower = targetPowerdBm
		params.OperationalMode = baselineOpMode
		cfgplugins.NewZRInterfaceConfigAll(t, dut, cleanupBatch, params)
		if !deviations.TransceiverConfigEnableUnsupported(dut) {
			cfgplugins.BatchConfigureTransceiverEnabled(t, dut, cleanupBatch, ports, params, true)
		}
		for _, p := range ports {
			gnmi.BatchDelete(cleanupBatch, gnmi.OC().Interface(p.Name()).Subinterface(0).Config())
		}

		niName := deviations.DefaultNetworkInstance(dut)
		staticProtoName := deviations.StaticProtocolName(dut)
		v4ToP2, _ := ipv4StaticPrefixes(true)
		v4ToP1, _ := ipv4StaticPrefixes(false)
		v6ToP2, _ := ipv6StaticPrefixes(true)
		v6ToP1, _ := ipv6StaticPrefixes(false)
		for _, prefix := range append(append(v4ToP2, v4ToP1...), append(v6ToP2, v6ToP1...)...) {
			gnmi.BatchDelete(cleanupBatch, gnmi.OC().NetworkInstance(niName).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, staticProtoName).Static(prefix).Config())
		}
		for attempt := 0; attempt < 3; attempt++ {
			if msg := testt.CaptureFatal(t, func(tb testing.TB) {
				cleanupBatch.Set(tb, dut)
			}); msg == nil {
				break
			}
		}

		cfgplugins.BounceTransceivers(t, dut, ports, params, negativeCheckTimeout)

		for _, p := range ports {
			trName := params.TransceiverNames[p.Name()]
			var ok bool
			for attempt := 0; attempt < 3; attempt++ {
				if msg := testt.CaptureFatal(t, func(tb testing.TB) {
					_, ok = gnmi.Watch(tb, dut, gnmi.OC().Interface(p.Name()).OperStatus().State(), negativeCheckTimeout, func(val *ygnmi.Value[oc.E_Interface_OperStatus]) bool {
						v, present := val.Val()
						return present && v == oc.Interface_OperStatus_UP
					}).Await(tb)
				}); msg == nil {
					break
				}
			}
			if !ok {
				t.Errorf("Cleanup: Interface %s did not return to OperStatus UP", p.Name())
			}
			var trActive bool
			for attempt := 0; attempt < 3; attempt++ {
				if msg := testt.CaptureFatal(t, func(tb testing.TB) {
					got, present := gnmi.Lookup(tb, dut, gnmi.OC().Component(trName).OperStatus().State()).Val()
					trActive = present && got == oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE
				}); msg == nil {
					break
				}
			}
			if !trActive {
				t.Errorf("Cleanup: Transceiver %s oper-status is not ACTIVE", trName)
			}
		}
	})

	fptest.ConfigureDefaultNetworkInstance(t, dut)

	baselineInventory := make(map[string]*transceiver.InventorySnapshot)
	var otgConfig gosnappi.Config

	// Subtest TRANSCEIVER-109.1 validates 400ZR/800ZR operational-mode transition with transceiver bounce and traffic.
	// README.md Section: Subtest TRANSCEIVER-109.1 (Steps 1 to 4).
	// Technical Summary: Provisions baseline mode, captures baseline CMIS inventory, transitions to custom mode with a bounce,
	// and verifies optical telemetry, inventory persistence, AFT convergence, and 0% loss dual-stack traffic.
	t.Run("TRANSCEIVER-109.1_Validate_400ZR_800ZR_Operational_Mode_Transition_With_Transceiver_Bounce_And_Traffic", func(t *testing.T) {
		t.Log("Starting TRANSCEIVER-109.1: Validate 400ZR / 800ZR operational-mode transition with transceiver bounce and bidirectional dual-stack traffic")
		// TRANSCEIVER-109.1 Step 1: Initial Default Operational Mode Provisioning (Both Ports).
		initBatch := &gnmi.SetBatch{}
		cfgplugins.NewZRInterfaceConfigAll(t, dut, initBatch, params)
		if !deviations.TransceiverConfigEnableUnsupported(dut) {
			cfgplugins.BatchConfigureTransceiverEnabled(t, dut, initBatch, ports, params, true)
		}
		initBatch.Set(t, dut)

		cfgplugins.BounceTransceivers(t, dut, ports, params, negativeCheckTimeout)

		for _, p := range ports {
			var ok bool
			for attempt := 0; attempt < 3; attempt++ {
				if msg := testt.CaptureFatal(t, func(tb testing.TB) {
					_, ok = gnmi.Watch(tb, dut, gnmi.OC().Interface(p.Name()).OperStatus().State(), convergenceTimeout, func(val *ygnmi.Value[oc.E_Interface_OperStatus]) bool {
						v, present := val.Val()
						return present && v == oc.Interface_OperStatus_UP
					}).Await(tb)
				}); msg == nil {
					break
				}
			}
			if !ok {
				t.Errorf("TRANSCEIVER-109.1 Step 1: Interface %s did not reach OperStatus UP within %v", p.Name(), convergenceTimeout)
			}
		}

		// TRANSCEIVER-109.1 Step 2: Baseline Transceiver Inventory Verification (gNMI.Subscribe & gNMI.Get).
		for _, p := range ports {
			baselineInventory[p.Name()] = transceiver.CaptureAndValidateBaselineInventory(t, dut, p, params, negativeCheckTimeout)
		}

		// TRANSCEIVER-109.1 Step 3: Reconfigure to Custom Operational Mode (Appsel) and Execute Event-Driven Transceiver Bounce.
		params.OperationalMode = customOpMode
		opModeBatch := &gnmi.SetBatch{}
		cfgplugins.BatchConfigureOpticalChannel(t, dut, opModeBatch, ports, params)
		opModeBatch.Set(t, dut)

		cfgplugins.BounceTransceivers(t, dut, ports, params, negativeCheckTimeout)

		// Wait via gNMI.Subscribe (ON_CHANGE and SAMPLE) and verify optical-channel operational-mode,
		// frequency, target-output-power, input/output power levels, pre-FEC BER, and zero FEC uncorrectable blocks.
		transceiver.VerifyOpticalAndChannelTelemetry(t, dut, ports, params, customOpMode, freqMHz, convergenceTimeout, negativeCheckTimeout)

		// Verify transceiver inventory persistence after custom operational-mode transition and transceiver bounce.
		for _, p := range ports {
			transceiver.VerifyInventoryMatchesBaseline(t, dut, p, params, baselineInventory[p.Name()], true, true, true)
		}

		// TRANSCEIVER-109.1 Step 4: Event-Driven Control-Plane/AFT Convergence and Bidirectional Dual-Stack Traffic Validation.
		configureL3AndStaticRoutes(t, dut, p1, p2, portAttrs)
		if hasATE {
			otgConfig = configureOTG(t, ate)
		}
		verifyControlPlaneConvergenceGate(t, dut, ate, otgConfig, p1, p2)
		runAndVerifyBidirectionalTraffic(t, dut, ate, otgConfig, p1, p2, params)
	})

	// Subtest TRANSCEIVER-109.2 validates transceiver inventory persistence across 5 interface delete/re-create stress cycles.
	// README.md Section: Subtest TRANSCEIVER-109.2 (Steps 1, 2.1, 2.2, 2.3, and 3).
	// Technical Summary: Runs a continuous ON_CHANGE inventory watcher while deleting and recreating both ZR interfaces 5 times,
	// confirming no inventory leaf is deleted or mutated, followed by post-stress optical and traffic verification.
	t.Run("TRANSCEIVER-109.2_Validate_400ZR_800ZR_Multi_Port_Inventory_Persistence_Across_Interface_Delete_And_Recreate_Cycles", func(t *testing.T) {
		t.Log("Starting TRANSCEIVER-109.2: Validate scaled multi-port 400ZR / 800ZR transceiver inventory persistence and link recovery across repeated interface delete and re-create cycles")
		// TRANSCEIVER-109.2 Step 1: Maintain Continuous ON_CHANGE Inventory Subscription.
		inventoryWatchers := make(map[string]*transceiver.OnChangeInventoryWatcher)
		for _, p := range ports {
			if baselineInventory[p.Name()] == nil {
				baselineInventory[p.Name()] = transceiver.CaptureAndValidateBaselineInventory(t, dut, p, params, negativeCheckTimeout)
			}
			trName := params.TransceiverNames[p.Name()]
			w := transceiver.StartOnChangeInventoryWatcher(t, dut, trName, baselineInventory[p.Name()])
			defer w.Close()
			inventoryWatchers[p.Name()] = w
			transceiver.VerifyInventoryMatchesBaseline(t, dut, p, params, baselineInventory[p.Name()], true, true, true)
			w.VerifyNoViolations(t)
		}

		// TRANSCEIVER-109.2 Step 2: Execute 5 Consecutive Multi-Port Interface Delete and Re-Create Cycles (Iteration i = 1..5).
		for cycle := 1; cycle <= stressDeleteCycles; cycle++ {
			// Step 2.1: Batched Interface Delete via gNMI.Set.
			deleteBatch := &gnmi.SetBatch{}
			cfgplugins.BatchDeleteZRInterfaces(t, dut, deleteBatch, ports, params)
			deleteBatch.Set(t, dut)

			// Step 2.2: Verify Transceiver Inventory Persistence While Interfaces Are Deleted.
			for _, p := range ports {
				inventoryWatchers[p.Name()].VerifyNoViolations(t)
				transceiver.VerifyInventoryMatchesBaseline(t, dut, p, params, baselineInventory[p.Name()], false, true, true)
			}

			// Step 2.3: Batched Interface Re-Creation and Custom Operational Mode Verification.
			recreateBatch := &gnmi.SetBatch{}
			params.Enabled = true
			params.OperationalMode = customOpMode
			cfgplugins.BatchRecreateZRInterfaces(t, dut, recreateBatch, ports, params, portAttrs)
			recreateBatch.Set(t, dut)

			cfgplugins.AssignPortsToDefaultNetworkInstance(t, dut, ports)

			for _, p := range ports {
				ochName := params.OpticalChannelNames[p.Name()]
				var ok bool
				for attempt := 0; attempt < 3; attempt++ {
					if msg := testt.CaptureFatal(t, func(tb testing.TB) {
						_, ok = gnmi.Watch(tb, dut, gnmi.OC().Interface(p.Name()).OperStatus().State(), convergenceTimeout, func(val *ygnmi.Value[oc.E_Interface_OperStatus]) bool {
							v, present := val.Val()
							return present && v == oc.Interface_OperStatus_UP
						}).Await(tb)
					}); msg == nil {
						break
					}
				}
				if !ok {
					t.Fatalf("Cycle %d: Interface %s did not return to OperStatus UP within %v after re-creation", cycle, p.Name(), convergenceTimeout)
				}
				if !deviations.OperationalModeUnsupported(dut) {
					var modeOK bool
					for attempt := 0; attempt < 3; attempt++ {
						if msg := testt.CaptureFatal(t, func(tb testing.TB) {
							_, modeOK = gnmi.Watch(tb, dut, gnmi.OC().Component(ochName).OpticalChannel().OperationalMode().State(), negativeCheckTimeout, func(val *ygnmi.Value[uint16]) bool {
								v, present := val.Val()
								return present && v == customOpMode
							}).Await(tb)
						}); msg == nil {
							break
						}
					}
					if !modeOK {
						var gotMode uint16
						for attempt := 0; attempt < 3; attempt++ {
							if msg := testt.CaptureFatal(t, func(tb testing.TB) {
								gotMode, _ = gnmi.Lookup(tb, dut, gnmi.OC().Component(ochName).OpticalChannel().OperationalMode().State()).Val()
							}); msg == nil {
								break
							}
						}
						t.Errorf("Cycle %d: Port %s /components/component[name=%s]/optical-channel/state/operational-mode = %d, want %d", cycle, p.Name(), ochName, gotMode, customOpMode)
					}
				}
				inventoryWatchers[p.Name()].VerifyNoViolations(t)
				transceiver.VerifyInventoryMatchesBaseline(t, dut, p, params, baselineInventory[p.Name()], true, true, true)
			}
		}

		// TRANSCEIVER-109.2 Step 3: Post-Stress Bidirectional Dual-Stack IPv4 and IPv6 Traffic Validation.
		transceiver.VerifyOpticalAndChannelTelemetry(t, dut, ports, params, customOpMode, freqMHz, convergenceTimeout, negativeCheckTimeout)
		configureL3AndStaticRoutes(t, dut, p1, p2, portAttrs)
		verifyControlPlaneConvergenceGate(t, dut, ate, otgConfig, p1, p2)
		runAndVerifyBidirectionalTraffic(t, dut, ate, otgConfig, p1, p2, params)
	})

	// Subtest TRANSCEIVER-109.3 validates negative fault handling, inventory retention, laser shutoff, and link recovery.
	// README.md Section: Subtest TRANSCEIVER-109.3 (Steps 1, 2, and 3).
	// Technical Summary: Tests (1) unsupported operational-mode 65535, (2) optical frequency mismatch (193.10 THz) and bounce
	// recovery, and (3) transceiver disable laser shutoff (<= -30 dBm) and re-enable recovery while verifying inventory stays intact.
	t.Run("TRANSCEIVER-109.3_Negative_Validation_Unsupported_Mode_Frequency_Mismatch_And_Laser_Shutoff", func(t *testing.T) {
		t.Log("Starting TRANSCEIVER-109.3: Negative Validation — Unsupported Operational Mode, Optical Mismatch Fault Recovery, and Laser Shutoff")
		if baselineInventory[p1.Name()] == nil {
			baselineInventory[p1.Name()] = transceiver.CaptureAndValidateBaselineInventory(t, dut, p1, params, negativeCheckTimeout)
		}
		p1OchName := params.OpticalChannelNames[p1.Name()]
		p1TrName := params.TransceiverNames[p1.Name()]

		// Maintain an active ON_CHANGE inventory watcher on Port 1 across all negative test cases.
		p1Watcher := transceiver.StartOnChangeInventoryWatcher(t, dut, p1TrName, baselineInventory[p1.Name()])
		defer p1Watcher.Close()

		// TRANSCEIVER-109.3 Step 1: Negative Case 1 — Unsupported / Out-of-Range operational-mode (Appsel) Handling.
		fatalMsg := testt.CaptureFatal(t, func(t testing.TB) {
			gnmi.Replace(t, dut, gnmi.OC().Component(p1OchName).OpticalChannel().OperationalMode().Config(), unsupportedOpMode)
		})
		if fatalMsg == nil {
			_, ok := gnmi.Watch(t, dut, gnmi.OC().Interface(p1.Name()).OperStatus().State(), negativeCheckTimeout, func(val *ygnmi.Value[oc.E_Interface_OperStatus]) bool {
				v, present := val.Val()
				return present && (v == oc.Interface_OperStatus_DOWN || v == oc.Interface_OperStatus_LOWER_LAYER_DOWN)
			}).Await(t)
			if !ok {
				t.Errorf("TRANSCEIVER-109.3 Step 1: Interface %s did not transition to DOWN/LOWER_LAYER_DOWN when configured with unsupported operational-mode %d", p1.Name(), unsupportedOpMode)
			}
		}

		// Verify via ON_CHANGE stream and gNMI.Get that transceiver inventory remains populated and identical to baseline.
		p1Watcher.VerifyNoViolations(t)
		transceiver.VerifyInventoryMatchesBaseline(t, dut, p1, params, baselineInventory[p1.Name()], true, false, true)

		// Restore valid custom operational-mode on port1 and ensure link is UP before Step 2.
		gnmi.Replace(t, dut, gnmi.OC().Component(p1OchName).OpticalChannel().OperationalMode().Config(), customOpMode)
		var ok bool
		for attempt := 0; attempt < 3; attempt++ {
			if msg := testt.CaptureFatal(t, func(tb testing.TB) {
				_, ok = gnmi.Watch(tb, dut, gnmi.OC().Interface(p1.Name()).OperStatus().State(), convergenceTimeout, func(val *ygnmi.Value[oc.E_Interface_OperStatus]) bool {
					v, present := val.Val()
					return present && v == oc.Interface_OperStatus_UP
				}).Await(tb)
			}); msg == nil {
				break
			}
		}
		if !ok {
			t.Fatalf("TRANSCEIVER-109.3 Step 1: Interface %s did not recover to OperStatus UP after restoring operational-mode %d", p1.Name(), customOpMode)
		}

		// TRANSCEIVER-109.3 Step 2: Negative Case 2 — Optical Frequency / Mode Mismatch Fault Simulation and Subsequent Bounce Recovery.
		gnmi.Replace(t, dut, gnmi.OC().Component(p1OchName).OpticalChannel().Frequency().Config(), mismatchedFrequencyMHz)

		ok = false
		for attempt := 0; attempt < 3; attempt++ {
			if msg := testt.CaptureFatal(t, func(tb testing.TB) {
				_, ok = gnmi.Watch(tb, dut, gnmi.OC().Interface(p1.Name()).OperStatus().State(), negativeCheckTimeout, func(val *ygnmi.Value[oc.E_Interface_OperStatus]) bool {
					v, present := val.Val()
					return present && (v == oc.Interface_OperStatus_DOWN || v == oc.Interface_OperStatus_LOWER_LAYER_DOWN)
				}).Await(tb)
			}); msg == nil {
				break
			}
		}
		if !ok {
			t.Errorf("TRANSCEIVER-109.3 Step 2: Interface %s did not transition to DOWN/LOWER_LAYER_DOWN under optical frequency mismatch (%d MHz)", p1.Name(), mismatchedFrequencyMHz)
		}

		// Verify transceiver inventory leaves remain intact while optical carrier lock is lost.
		p1Watcher.VerifyNoViolations(t)
		transceiver.VerifyInventoryMatchesBaseline(t, dut, p1, params, baselineInventory[p1.Name()], true, false, true)

		// Verify 100% traffic loss (rx_pkts == 0) on Flow 1A and Flow 2A while Link 1 is down due to frequency mismatch.
		var mismatchTrafficStarted bool
		if hasATE {
			ate.OTG().StartTraffic(t)
			mismatchTrafficStarted = true
			defer func() {
				if mismatchTrafficStarted {
					ate.OTG().StopTraffic(t)
				}
			}()
			for _, fn := range []string{flow1AName, flow2AName} {
				_, ok := gnmi.Watch(t, ate.OTG(), gnmi.OTG().Flow(fn).State(), negativeCheckTimeout, func(val *ygnmi.Value[*otg.Flow]) bool {
					f, present := val.Val()
					return present && f != nil && !f.GetTransmit() && f.GetCounters().GetOutPkts() == trafficFixedPackets
				}).Await(t)
				if !ok {
					t.Errorf("TRANSCEIVER-109.3 Step 2: Flow %s did not finish transmitting %d packets", fn, trafficFixedPackets)
				}
			}
			ate.OTG().StopTraffic(t)
			mismatchTrafficStarted = false
			for _, fn := range []string{flow1AName, flow2AName} {
				rxPkts := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(fn).Counters().InPkts().State())
				if rxPkts != 0 {
					t.Errorf("TRANSCEIVER-109.3 Step 2: Flow %s received %d packets during optical frequency mismatch, want 0 (100%% loss)", fn, rxPkts)
				}
			}
		}

		// Recovery from Optical Fault: restore frequency to 196100000 MHz and operational-mode to customOpMode, then bounce transceiver.
		params.Frequency = freqMHz
		params.OperationalMode = customOpMode
		restoreFreqBatch := &gnmi.SetBatch{}
		cfgplugins.BatchConfigureOpticalChannel(t, dut, restoreFreqBatch, []*ondatra.Port{p1}, params)
		restoreFreqBatch.Set(t, dut)

		if !deviations.OperationalModeUnsupported(dut) {
			for attempt := 0; attempt < 3; attempt++ {
				if msg := testt.CaptureFatal(t, func(tb testing.TB) {
					gnmi.Watch(tb, dut, gnmi.OC().Component(p1OchName).OpticalChannel().Frequency().State(), negativeCheckTimeout, func(val *ygnmi.Value[uint64]) bool {
						v, present := val.Val()
						return present && v == freqMHz
					}).Await(tb)
				}); msg == nil {
					break
				}
			}
		}

		cfgplugins.BounceTransceivers(t, dut, []*ondatra.Port{p1}, params, negativeCheckTimeout)

		ok = false
		for attempt := 0; attempt < 3; attempt++ {
			if msg := testt.CaptureFatal(t, func(tb testing.TB) {
				_, ok = gnmi.Watch(tb, dut, gnmi.OC().Interface(p1.Name()).OperStatus().State(), convergenceTimeout, func(val *ygnmi.Value[oc.E_Interface_OperStatus]) bool {
					v, present := val.Val()
					return present && v == oc.Interface_OperStatus_UP
				}).Await(tb)
			}); msg == nil {
				break
			}
		}
		if !ok {
			cfgplugins.BounceTransceivers(t, dut, ports, params, negativeCheckTimeout)
			for attempt := 0; attempt < 3; attempt++ {
				if msg := testt.CaptureFatal(t, func(tb testing.TB) {
					_, ok = gnmi.Watch(tb, dut, gnmi.OC().Interface(p1.Name()).OperStatus().State(), convergenceTimeout, func(val *ygnmi.Value[oc.E_Interface_OperStatus]) bool {
						v, present := val.Val()
						return present && v == oc.Interface_OperStatus_UP
					}).Await(tb)
				}); msg == nil {
					break
				}
			}
		}
		if !ok {
			t.Fatalf("TRANSCEIVER-109.3 Step 2 Recovery: Interface %s did not return to OperStatus UP within %v", p1.Name(), convergenceTimeout)
		}
		configureL3AndStaticRoutes(t, dut, p1, p2, portAttrs)
		verifyControlPlaneConvergenceGate(t, dut, ate, otgConfig, p1, p2)
		runAndVerifyBidirectionalTraffic(t, dut, ate, otgConfig, p1, p2, params)

		// TRANSCEIVER-109.3 Step 3: Negative Case 3 — Administrative Transceiver Disable Laser Shutoff and Inventory Retention.
		disableTrBatch := &gnmi.SetBatch{}
		cfgplugins.BatchConfigureTransceiverEnabled(t, dut, disableTrBatch, []*ondatra.Port{p1}, params, false)
		disableTrBatch.Set(t, dut)

		if !deviations.TransceiverConfigEnableUnsupported(dut) {
			var trDisabled bool
			for attempt := 0; attempt < 3; attempt++ {
				if msg := testt.CaptureFatal(t, func(tb testing.TB) {
					_, trDisabled = gnmi.Watch(tb, dut, gnmi.OC().Component(p1TrName).Transceiver().Enabled().State(), negativeCheckTimeout, func(val *ygnmi.Value[bool]) bool {
						v, present := val.Val()
						return present && !v
					}).Await(tb)
				}); msg == nil {
					break
				}
			}
			if !trDisabled {
				t.Errorf("TRANSCEIVER-109.3 Step 3: Transceiver %s state/enabled did not transition to false within %v", p1TrName, negativeCheckTimeout)
			}
		}

		ok = false
		for attempt := 0; attempt < 3; attempt++ {
			if msg := testt.CaptureFatal(t, func(tb testing.TB) {
				_, ok = gnmi.Watch(tb, dut, gnmi.OC().Interface(p1.Name()).OperStatus().State(), negativeCheckTimeout, func(val *ygnmi.Value[oc.E_Interface_OperStatus]) bool {
					v, present := val.Val()
					return present && (v == oc.Interface_OperStatus_DOWN || v == oc.Interface_OperStatus_LOWER_LAYER_DOWN)
				}).Await(tb)
			}); msg == nil {
				break
			}
		}
		if !ok {
			t.Errorf("TRANSCEIVER-109.3 Step 3: Interface %s did not transition to DOWN/LOWER_LAYER_DOWN within %v", p1.Name(), negativeCheckTimeout)
		}

		// Verify via gNMI.Subscribe and gNMI.Get that optical-channel output-power/instant drops below or equal to -30.0 dBm.
		transceiver.VerifyLaserShutoffOutputPower(t, dut, p1OchName, laserOffOutputPowerdBm, negativeCheckTimeout)

		// Verify via ON_CHANGE stream and gNMI.Get that all static transceiver inventory leaves remain present and identical to baseline.
		p1Watcher.VerifyNoViolations(t)
		transceiver.VerifyInventoryMatchesBaseline(t, dut, p1, params, baselineInventory[p1.Name()], true, false, false)

		// Final Recovery Verification: Re-enable transceiver and verify full recovery.
		enableTrBatch := &gnmi.SetBatch{}
		cfgplugins.BatchConfigureTransceiverEnabled(t, dut, enableTrBatch, []*ondatra.Port{p1}, params, true)
		enableTrBatch.Set(t, dut)

		transceiver.VerifyOpticalAndChannelTelemetry(t, dut, ports, params, customOpMode, freqMHz, convergenceTimeout, negativeCheckTimeout)
		p1Watcher.VerifyNoViolations(t)
		transceiver.VerifyInventoryMatchesBaseline(t, dut, p1, params, baselineInventory[p1.Name()], true, true, true)
		configureL3AndStaticRoutes(t, dut, p1, p2, portAttrs)
		verifyControlPlaneConvergenceGate(t, dut, ate, otgConfig, p1, p2)
		runAndVerifyBidirectionalTraffic(t, dut, ate, otgConfig, p1, p2, params)
	})
}
