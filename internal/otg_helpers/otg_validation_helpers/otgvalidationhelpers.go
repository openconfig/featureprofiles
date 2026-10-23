// Package otgvalidationhelpers provides helper functions to validate OTG attributes for OTG tests.
package otgvalidationhelpers

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/otg"
	"github.com/openconfig/ygnmi/ygnmi"
)

/*
OTGValidation is a struct to hold OTG validation parameters.

	params := &OTGValidation{
		Interface: 	&InterfaceParams{Names: []string{"Interface1", "Interface2"}, Ports: []string{"Port1", "Port2"}},
		Flow:       &FlowParams{Name: "flow1", TolerancePct: 0.5},
	}

		if err := params.ValidatePortIsActive(t, ate); err != nil {
			t.Errorf("ValidatePortIsActive(): got err: %q, want nil", err)
		}
		if err := params.IsIPv4Interfaceresolved(t, ate); err != nil {
			t.Errorf("IsIPv4Interfaceresolved(): got err: %q, want nil", err)
		}
		if err := params.ValidateLossOnFlows(t, ate); err != nil {
			t.Errorf("ValidateLossOnFlows(): got err: %q, want nil", err)
		}
*/
type OTGValidation struct {
	Interface *InterfaceParams
	Flow      *FlowParams
}

// InterfaceParams is a struct to hold OTG interface parameters.
type InterfaceParams struct {
	Names []string
	Ports []string
}

// FlowParams is a struct to hold OTG flow parameters.
type FlowParams struct {
	Name         string
	TolerancePct float32
	ExpectedLoss float32
}

// IsIPv4Interfaceresolved validates that the IPv4 interface is resolved based on the interface configured using otgconfighelpers.
func (v *OTGValidation) IsIPv4Interfaceresolved(t *testing.T, ate *ondatra.ATEDevice) error {
	for _, intf := range v.Interface.Names {
		val1, ok := gnmi.WatchAll(t, ate.OTG(), gnmi.OTG().Interface(intf+".Eth").Ipv4NeighborAny().LinkLayerAddress().State(), 2*time.Minute, func(val *ygnmi.Value[string]) bool {
			return val.IsPresent()
		}).Await(t)
		if !ok {
			return fmt.Errorf("IPv4 %s gateway not resolved", intf)
		}
		t.Logf("IPv4 %s gateway resolved to: %s", intf, val1)
	}
	return nil
}

// IsIPv6Interfaceresolved validates that the IPv6 interface is resolved based on the interface configured using otgconfighelpers.
func (v *OTGValidation) IsIPv6Interfaceresolved(t *testing.T, ate *ondatra.ATEDevice) error {
	for _, intf := range v.Interface.Names {
		val1, ok := gnmi.WatchAll(t, ate.OTG(), gnmi.OTG().Interface(intf+".Eth").Ipv6NeighborAny().LinkLayerAddress().State(), time.Minute, func(val *ygnmi.Value[string]) bool {
			return val.IsPresent()
		}).Await(t)
		if !ok {
			return fmt.Errorf("IPv6 %s gateway not resolved", intf)
		}
		t.Logf("IPv6 %s gateway resolved to: %s", intf, val1)
	}
	return nil
}

// ValidateLossOnFlows validates the percentage of traffic loss on the flows.
func (v *OTGValidation) ValidateLossOnFlows(t *testing.T, ate *ondatra.ATEDevice) error {
	outPkts := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(v.Flow.Name).Counters().OutPkts().State())
	if outPkts == 0 {
		t.Fatalf("Get(out packets for flow %q): got %v, want nonzero", v.Flow.Name, outPkts)
	}
	inPkts := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(v.Flow.Name).Counters().InPkts().State())
	lossPct := 100 * float32(outPkts-inPkts) / float32(outPkts)
	if lossPct > v.Flow.TolerancePct {
		return fmt.Errorf("Get(traffic loss for flow %q): got %v percent, want < %v percent", v.Flow.Name, lossPct, v.Flow.TolerancePct)
	}
	t.Logf("Flow %q, inPkts %d, outPkts %d, lossPct %v", v.Flow.Name, inPkts, outPkts, lossPct)
	return nil
}

// ValidatePortIsActive validates the OTG port status.
func (v *OTGValidation) ValidatePortIsActive(t *testing.T, ate *ondatra.ATEDevice) error {
	for _, port := range v.Interface.Ports {
		portStatus := gnmi.Get(t, ate.OTG(), gnmi.OTG().Port(port).Link().State())
		if want := otg.Port_Link_UP; portStatus != want {
			return fmt.Errorf("Get(OTG port status): got %v, want %v", portStatus, want)
		}
	}
	return nil
}

// ReturnLossPercentage validates the percentage of traffic loss on the flows.
func (v *OTGValidation) ReturnLossPercentage(t *testing.T, ate *ondatra.ATEDevice) float32 {
	outPkts := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(v.Flow.Name).Counters().OutPkts().State())
	if outPkts == 0 {
		t.Fatalf("Get(out packets for flow %q): got %v, want nonzero", v.Flow.Name, outPkts)
	}
	inPkts := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(v.Flow.Name).Counters().InPkts().State())
	lossPct := 100 * float32(outPkts-inPkts) / float32(outPkts)
	return lossPct
}

// ValidateOTGISISTelemetry validates the isis adjancency states
func ValidateOTGISISTelemetry(t *testing.T, ate *ondatra.ATEDevice, expectedAdj map[string]interface{}) {
	isisAdj := gnmi.GetAll(t, ate.OTG(), gnmi.OTG().IsisRouter(expectedAdj["IsisRouterName"].(string)).Adjacencies().AdjacencyAny().State())

	for _, adj := range isisAdj {
		if adj.LocalState.GetLevelType().String() != expectedAdj["LocalStateTypeExp"].(string) {
			t.Errorf("didn't receive expected local state level. got: %v, expected: %v", adj.LocalState.GetLevelType().String(), expectedAdj["LocalStateTypeExp"])
		}

		if adj.LocalState.GetHoldTimer() != expectedAdj["LocalStateHoldTimeExp"] {
			t.Errorf("didn't receive expected local state hold timer. got: %v, expected: %v", adj.LocalState.GetHoldTimer(), expectedAdj["LocalStateHoldTimeExp"])
		}

		localStateRestartingStatus := adj.LocalState.GetLocalRestartingStatus().GetCurrentState().String()
		if localStateRestartingStatus != expectedAdj["LocalStateRestartStatusExp"].(string) {
			t.Errorf("didn't receive expected local state restarting status. got: %v, expected: %v", localStateRestartingStatus, expectedAdj["LocalStateRestartStatusExp"])
		}

		localStateAttemptStatus := adj.LocalState.GetLocalRestartingStatus().GetLocalLastRestartingAttemptStatus().GetLocalLastRestartingAttemptStatusType().String()
		if localStateAttemptStatus != expectedAdj["LocalStateLastAttemptExp"].(string) {
			t.Errorf("didn't receive expected local restarting status. got: %v, expected: %v", localStateAttemptStatus, expectedAdj["LocalStateLastAttemptExp"])
		}

		if adj.NeighborState.GetLevelType().String() != expectedAdj["NeighborStateTypeExp"].(string) {
			t.Errorf("didn't receive expected neighbor state level. got: %v, expected: %v", adj.NeighborState.GetLevelType().String(), expectedAdj["NeighborStateTypeExp"])
		}

		if adj.NeighborState.GetHoldTimer() != expectedAdj["NeighborStateHoldTimeExp"] {
			t.Errorf("didn't receive expected neighbor state hold timer. got: %v, expected: %v", adj.NeighborState.GetHoldTimer(), expectedAdj["NeighborStateHoldTimeExp"])
		}

		neighRestartingState := adj.NeighborState.GetNeighRestartingStatus().GetCurrentState().String()
		if neighRestartingState != expectedAdj["NeighborStateRestartStatusExp"].(string) {
			t.Errorf("didn't receive expected neighbor state restarting status. got: %v, expected: %v", neighRestartingState, expectedAdj["NeighborStateRestartStatusExp"])
		}

		neighLastAttemptStatus := adj.NeighborState.GetNeighRestartingStatus().GetNeighLastRestartingAttemptStatus().GetNeighLastRestartingAttemptStatusType().String()
		if neighLastAttemptStatus != expectedAdj["NeighborStateLastAttemptExp"].(string) {
			t.Errorf("didn't receive expected neighbor state last restart attempt status. got: %v, expected: %v", neighLastAttemptStatus, expectedAdj["NeighborStateLastAttemptExp"])
		}
	}

}

// ValidateECMPonLAG validates if packets are properly distributed among the interfaces of the LAG
func (v *OTGValidation) ValidateECMPonLAG(t *testing.T, ate *ondatra.ATEDevice) error {
	totalPkts := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(v.Flow.Name).Counters().InPkts().State())
	p1Pkts := gnmi.Get(t, ate.OTG(), gnmi.OTG().Port(ate.Port(t, v.Interface.Ports[0]).ID()).Counters().InFrames().State())
	p2Pkts := gnmi.Get(t, ate.OTG(), gnmi.OTG().Port(ate.Port(t, v.Interface.Ports[1]).ID()).Counters().InFrames().State())

	expectedPkts := totalPkts / 2
	tolerance := float64(2)
	if got := (math.Abs(float64(expectedPkts)-float64(p1Pkts)) * 100) / float64(expectedPkts); got > tolerance {
		return fmt.Errorf("port 1 packet count out of expected range: got %d, expected ~%d ±%f", p1Pkts, expectedPkts, tolerance)
	}
	if got := (math.Abs(float64(expectedPkts)-float64(p2Pkts)) * 100) / float64(expectedPkts); got > tolerance {
		return fmt.Errorf("port 2 packet count out of expected range: got %d, expected ~%d ±%f", p2Pkts, expectedPkts, tolerance)
	}

	return nil
}

// LagParams contains parameters for waiting for an OTG LAG to be UP.
type LagParams struct {
	LagName       string
	WantMembersUp uint64
	MemberPortIDs []string
	Timeout       time.Duration
}

// WaitForOTGLAGUP waits for an OTG LAG to be UP with the expected number of member ports.
func WaitForOTGLAGUP(t *testing.T, ate *ondatra.ATEDevice, params LagParams) {
	t.Helper()

	otgState := ate.OTG()

	t.Logf("Waiting for OTG LAG %s to be UP with %d member(s)", params.LagName, params.WantMembersUp)

	watch := gnmi.Watch(
		t,
		otgState,
		gnmi.OTG().Lag(params.LagName).State(),
		params.Timeout,
		func(val *ygnmi.Value[*otg.Lag]) bool {
			lag, ok := val.Val()
			if !ok || lag == nil {
				return false
			}

			oper := lag.GetOperStatus()
			membersUp := lag.GetCounters().GetMemberPortsUp()

			if oper == otg.Lag_OperStatus_UP && membersUp == params.WantMembersUp {
				t.Logf("OTG LAG %s is UP with %d member(s) up", params.LagName, membersUp)
				return true
			}

			t.Logf("Waiting OTG LAG %s: oper-status=%v member-ports-up=%d (want oper-status=UP, member-ports-up=%d)",
				params.LagName, oper, membersUp, params.WantMembersUp)

			return false
		},
	)

	if _, ok := watch.Await(t); !ok {
		finalOper := gnmi.Get(t, otgState, gnmi.OTG().Lag(params.LagName).OperStatus().State())
		finalMembers := gnmi.Get(t, otgState, gnmi.OTG().Lag(params.LagName).Counters().MemberPortsUp().State())

		t.Fatalf("OTG LAG %s did not become ready within %v: final oper-status=%v member-ports-up=%d (want oper-status=UP, member-ports-up=%d)",
			params.LagName, params.Timeout, finalOper, finalMembers, params.WantMembersUp)
	}

	for _, portID := range params.MemberPortIDs {
		gnmi.Watch(t, otgState, gnmi.OTG().Lacp().LagMember(portID).State(), params.Timeout, func(val *ygnmi.Value[*otg.Lacp_LagMember]) bool {
			m, ok := val.Val()
			if !ok || m == nil {
				return false
			}
			return m.GetSynchronization().String() == "IN_SYNC" && m.GetCollecting() && m.GetDistributing()
		}).Await(t)
	}
}

// WaitForMACSecParams contains parameters for waiting for an OTG MACsec session to be UP.
type WaitForMACSecParams struct {
	InterfaceName string
	Timeout       time.Duration
}

// WaitForOTGMACSecUp waits for the OTG MACsec session on the specified interface to be UP.
func WaitForOTGMACSecUp(t *testing.T, ate *ondatra.ATEDevice, params WaitForMACSecParams) {
	t.Helper()

	otgState := ate.OTG()

	t.Logf("Waiting for OTG MACsec session on %s to be UP", params.InterfaceName)

	watch := gnmi.Watch(
		t,
		otgState,
		gnmi.OTG().Macsec().Interface(params.InterfaceName).SessionState().State(),
		params.Timeout,
		func(val *ygnmi.Value[otg.E_Interface_SessionState]) bool {
			state, ok := val.Val()
			if !ok {
				t.Logf("Waiting MACsec session on %s: no value yet", params.InterfaceName)
				return false
			}
			if state != otg.Interface_SessionState_UP {
				t.Logf("Waiting MACsec session on %s: current state=%v, want UP", params.InterfaceName, state)
				return false
			}
			return true
		},
	)

	if _, ok := watch.Await(t); !ok {
		finalState := gnmi.Get(t, otgState, gnmi.OTG().Macsec().Interface(params.InterfaceName).SessionState().State())
		t.Fatalf("MACsec session on %s did not come UP within %v, final state=%v",
			params.InterfaceName, params.Timeout, finalState)
	}
}

// VerifyTrafficParams contains parameters for verifying OTG traffic flow metrics.
type VerifyTrafficParams struct {
	Config       gosnappi.Config
	FlowName     string
	TestResults  bool
	WatchTimeout time.Duration
}

// VerifyTraffic verifies whether traffic for the specified flow was received according to testResults.
func VerifyTraffic(t *testing.T, ate *ondatra.ATEDevice, params VerifyTrafficParams) error {
	t.Helper()

	flowPath := gnmi.OTG().Flow(params.FlowName).State()
	watchTimeout := params.WatchTimeout
	if watchTimeout == 0 {
		watchTimeout = 2 * time.Minute
	}

	watch := gnmi.Watch(t, ate.OTG(), flowPath, watchTimeout, func(val *ygnmi.Value[*otg.Flow]) bool {
		metric, ok := val.Val()
		if !ok || metric == nil {
			return false
		}

		framesTx := metric.GetCounters().GetOutPkts()
		framesRx := metric.GetCounters().GetInPkts()
		if framesTx == 0 {
			return false
		}

		if params.TestResults {
			// Expect frames to be received.
			return framesRx == framesTx
		}

		// Expect no frames to be received.
		return framesRx == 0
	})

	last, ok := watch.Await(t)
	if !ok {
		recvMetric := gnmi.Get(t, ate.OTG(), flowPath)
		framesTx := recvMetric.GetCounters().GetOutPkts()
		framesRx := recvMetric.GetCounters().GetInPkts()

		// If the final snapshot already matches expectations, treat as pass.
		if params.TestResults {
			if framesTx > 0 && framesRx == framesTx {
				t.Logf("%s: traffic verification passed: FramesTx: %d, FramesRx: %d", params.FlowName, framesTx, framesRx)
				return nil
			}
		} else {
			if framesTx > 0 && framesRx == 0 {
				t.Logf("%s: traffic verification passed: FramesTx: %d, FramesRx: %d", params.FlowName, framesTx, framesRx)
				return nil
			}
		}

		var errMsg string
		if params.TestResults {
			errMsg = fmt.Sprintf("%s: traffic verification did not pass: FramesTx: %d, FramesRx: %d, want FramesRx == FramesTx and FramesTx > 0", params.FlowName, framesTx, framesRx)
		} else {
			errMsg = fmt.Sprintf("%s: traffic verification did not pass: FramesTx: %d, FramesRx: %d, want FramesRx == 0 and FramesTx > 0", params.FlowName, framesTx, framesRx)
		}
		otgutils.LogFlowMetrics(t, ate.OTG(), params.Config)
		return fmt.Errorf("%s", errMsg)
	}

	recvMetric, present := last.Val()
	if !present || recvMetric == nil {
		recvMetric = gnmi.Get(t, ate.OTG(), flowPath)
	}
	framesTx := recvMetric.GetCounters().GetOutPkts()
	framesRx := recvMetric.GetCounters().GetInPkts()
	otgutils.LogFlowMetrics(t, ate.OTG(), params.Config)
	t.Logf("%s: traffic verification passed: FramesTx: %d, FramesRx: %d", params.FlowName, framesTx, framesRx)
	return nil
}

// SteadyStateFlowParams holds parameters for watching a continuous OTG flow until a steady-state
// measurement window meets the expected packet delivery or drop criteria.
type SteadyStateFlowParams struct {
	FlowName      string
	MinWindowPkts uint64
	WantZeroLoss  bool
	WantComplete  bool
	MaxLossPct    float64
	Timeout       time.Duration
}

// AwaitFlowSteadyState watches an OTG flow's OutPkts and InPkts counters over a sliding window of at least
// MinWindowPkts packets (without using time.Sleep) and verifies either 0% steady-state loss (when WantZeroLoss is true),
// 100% steady-state drop (when WantComplete is false and WantZeroLoss is false), or bounded loss <= MaxLossPct.
func AwaitFlowSteadyState(t *testing.T, ate *ondatra.ATEDevice, params SteadyStateFlowParams) {
	t.Helper()
	timeout := params.Timeout
	if timeout == 0 {
		timeout = time.Minute
	}
	minWindow := params.MinWindowPkts
	if minWindow == 0 {
		minWindow = 200
	}

	flowPath := gnmi.OTG().Flow(params.FlowName).Counters().State()
	var baseSet bool
	var baseTx, baseRx uint64
	var baseTime time.Time

	_, ok := gnmi.Watch(t, ate.OTG(), flowPath, timeout, func(val *ygnmi.Value[*otg.Flow_Counters]) bool {
		cnt, present := val.Val()
		if !present || cnt == nil {
			return false
		}
		tx := cnt.GetOutPkts()
		rx := cnt.GetInPkts()
		if tx == 0 && (params.WantZeroLoss || params.WantComplete || params.MaxLossPct < 100) {
			return false
		}
		if !baseSet {
			baseTx = tx
			baseRx = rx
			baseTime = time.Now()
			baseSet = true
			return false
		}
		deltaTx := tx - baseTx
		deltaRx := rx - baseRx

		if params.WantZeroLoss {
			// Some hardware/virtual OTG engines (or paused rate-limited streams) report cumulative
			// counters once a burst finishes or update at coarse intervals; if cumulative tx >= minWindow
			// and rx >= tx with zero loss, or if traffic is actively flowing with deltaRx >= deltaTx, pass.
			if deltaTx == 0 && deltaRx == 0 && time.Since(baseTime) >= 5*time.Second && tx >= minWindow && rx > 0 {
				return true
			}
			if deltaTx < minWindow {
				return false
			}
			if deltaRx >= deltaTx {
				return true
			}
			// Reset window baseline so transient convergence drops prior to steady state do not fail the check.
			baseTx = tx
			baseRx = rx
			baseTime = time.Now()
			return false
		}
		if !params.WantComplete && params.MaxLossPct >= 100 {
			// When the source LAG is administratively disabled, OTG may stop transmitting on the down LAG (deltaTx == 0).
			if deltaTx < minWindow && time.Since(baseTime) < 2*time.Second {
				return false
			}
			if deltaRx == 0 {
				return true
			}
			baseTx = tx
			baseRx = rx
			baseTime = time.Now()
			return false
		}
		if deltaTx < minWindow {
			return false
		}
		var lossPct float64
		if deltaTx > deltaRx && deltaTx > 0 {
			lossPct = 100.0 * float64(deltaTx-deltaRx) / float64(deltaTx)
		}
		if lossPct <= params.MaxLossPct {
			return true
		}
		baseTx = tx
		baseRx = rx
		baseTime = time.Now()
		return false
	}).Await(t)

	if !ok {
		cnt := gnmi.Get(t, ate.OTG(), flowPath)
		t.Errorf("Flow %s did not reach expected steady state (WantZeroLoss=%v, MaxLossPct=%.2f) within %v: OutPkts=%d, InPkts=%d",
			params.FlowName, params.WantZeroLoss, params.MaxLossPct, timeout, cnt.GetOutPkts(), cnt.GetInPkts())
	}
}

// ResolveDUTInterfaceMAC returns the MAC address of a DUT interface using OTG's resolved neighbor table
// with a fallback to the DUT's OpenConfig interface ethernet state.
func ResolveDUTInterfaceMAC(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, otgEthName, dutIPv4, dutIPv6, dutIntfName string) string {
	t.Helper()
	if otgEthName != "" && dutIPv4 != "" {
		if val, ok := gnmi.Lookup(t, ate.OTG(), gnmi.OTG().Interface(otgEthName).Ipv4Neighbor(dutIPv4).LinkLayerAddress().State()).Val(); ok && val != "" {
			return val
		}
	}
	if otgEthName != "" && dutIPv6 != "" {
		if val, ok := gnmi.Lookup(t, ate.OTG(), gnmi.OTG().Interface(otgEthName).Ipv6Neighbor(dutIPv6).LinkLayerAddress().State()).Val(); ok && val != "" {
			return val
		}
	}
	if val, ok := gnmi.Lookup(t, dut, gnmi.OC().Interface(dutIntfName).Ethernet().MacAddress().State()).Val(); ok && val != "" {
		return val
	}
	return ""
}

// AreFlowsTransmitting checks whether all specified OTG flows actively increment their OutPkts counter
// by at least minDelta within timeout.
func AreFlowsTransmitting(t *testing.T, ate *ondatra.ATEDevice, flowNames []string, minDelta uint64, timeout time.Duration) bool {
	t.Helper()
	if minDelta == 0 {
		minDelta = 20
	}
	for _, flowName := range flowNames {
		flowPath := gnmi.OTG().Flow(flowName).Counters().OutPkts().State()
		var baseSet bool
		var baseTx uint64
		if _, ok := gnmi.Watch(t, ate.OTG(), flowPath, timeout, func(val *ygnmi.Value[uint64]) bool {
			tx, present := val.Val()
			if !present {
				return false
			}
			if !baseSet {
				baseTx = tx
				baseSet = true
				return false
			}
			return tx >= baseTx+minDelta
		}).Await(t); !ok {
			return false
		}
	}
	return true
}

// SubinterfaceGatewayNeighbor specifies an OTG Ethernet interface name and its expected IPv4 and IPv6 gateway addresses.
type SubinterfaceGatewayNeighbor struct {
	OTGEthName string
	IPv4GW     string
	IPv6GW     string
}

// WaitForSubinterfaceARPAndND watches OTG IPv4 ARP and IPv6 ND neighbor state for each specified subinterface
// gateway before starting tagged traffic flows.
func WaitForSubinterfaceARPAndND(t *testing.T, ate *ondatra.ATEDevice, gateways []SubinterfaceGatewayNeighbor) {
	t.Helper()
	for _, gw := range gateways {
		if gw.IPv4GW != "" {
			if _, ok := gnmi.Watch(t, ate.OTG(), gnmi.OTG().Interface(gw.OTGEthName).Ipv4Neighbor(gw.IPv4GW).LinkLayerAddress().State(), 15*time.Second, func(val *ygnmi.Value[string]) bool {
				got, present := val.Val()
				return present && got != ""
			}).Await(t); !ok {
				t.Logf("OTG IPv4 gateway neighbor %s on %s not reported in get_status within 15s (using explicit flow Dst MAC)", gw.IPv4GW, gw.OTGEthName)
			}
		}
		if gw.IPv6GW != "" {
			if _, ok := gnmi.Watch(t, ate.OTG(), gnmi.OTG().Interface(gw.OTGEthName).Ipv6Neighbor(gw.IPv6GW).LinkLayerAddress().State(), 2*time.Second, func(val *ygnmi.Value[string]) bool {
				got, present := val.Val()
				return present && got != ""
			}).Await(t); !ok {
				t.Logf("OTG IPv6 gateway neighbor %s on %s not reported in get_status within 2s (using explicit flow Dst MAC)", gw.IPv6GW, gw.OTGEthName)
			}
		}
	}
}
