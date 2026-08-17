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

package otgutils

import (
	"testing"
	"time"

	"github.com/openconfig/ondatra/gnmi"
	otgtelemetry "github.com/openconfig/ondatra/gnmi/otg"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygnmi/ygnmi"
)

// GetFlowStats checks to see if all the flows are completely stopped and returns tx and rx packets for the given flow
func GetFlowStats(t testing.TB, otg *otg.OTG, flowName string, timeout time.Duration) (txPackets, rxPackets uint64) {
	flow := gnmi.OTG().Flow(flowName)

	_, watcher := gnmi.Watch(t, otg, flow.Transmit().State(), timeout, func(val *ygnmi.Value[bool]) bool {
		transmitState, ok := val.Val()
		return ok && !transmitState
	}).Await(t)
	if !watcher {
		t.Logf("Flow still not stopped after %v. Stats may be inconsistent", timeout)
	}
	txPkts := gnmi.Get(t, otg, gnmi.OTG().Flow(flowName).Counters().OutPkts().State())

	rxPkts, _ := gnmi.Watch(t, otg, gnmi.OTG().Flow(flowName).Counters().InPkts().State(), timeout, func(val *ygnmi.Value[uint64]) bool {
		rxPackets, present := val.Val()
		return present && rxPackets == txPkts
	}).Await(t)
	if rxPkts == nil {
		return txPkts, 0
	}
	rx, _ := rxPkts.Val()

	return txPkts, rx

}

// GetFlowLossPct checks to see if all the flows are completely stopped and
// returns the loss percentage for the given flow
func GetFlowLossPct(t testing.TB, otg *otg.OTG, flowName string, timeout time.Duration) (lossPct float64) {
	tx, rx := GetFlowStats(t, otg, flowName, timeout)
	return (float64(tx) - float64(rx)) * 100 / float64(tx)
}

// VerifyNoPacketLoss verifies that each of the given flows has a loss
// percentage below 5% and reports an error otherwise.
func VerifyNoPacketLoss(t testing.TB, otg *otg.OTG, allFlows []string) {
	t.Helper()
	LogFlowMetrics(t, otg, otg.FetchConfig(t))
	for _, flow := range allFlows {
		_, ok := gnmi.Watch(t, otg, gnmi.OTG().Flow(flow).State(), 15*time.Second, func(val *ygnmi.Value[*otgtelemetry.Flow]) bool {
			flowState, present := val.Val()
			if !present {
				return false
			}
			txPackets := float64(flowState.GetCounters().GetOutPkts())
			if txPackets == 0 {
				return false
			}
			rxPackets := float64(flowState.GetCounters().GetInPkts())
			lossPct := (txPackets - rxPackets) * 100 / txPackets
			return lossPct < 5.0
		}).Await(t)
		if !ok {
			t.Errorf("Traffic Loss Pct for Flow %s: expected loss < 5%%", flow)
		} else {
			t.Logf("Traffic Test Passed for flow %s!", flow)
		}
	}
}

// ConfirmPacketLoss verifies that each of the given flows has a loss
// percentage above 99% and reports an error otherwise.
func ConfirmPacketLoss(t testing.TB, otg *otg.OTG, allFlows []string) {
	t.Helper()
	LogFlowMetrics(t, otg, otg.FetchConfig(t))
	for _, flow := range allFlows {
		_, ok := gnmi.Watch(t, otg, gnmi.OTG().Flow(flow).State(), 15*time.Second, func(val *ygnmi.Value[*otgtelemetry.Flow]) bool {
			flowState, present := val.Val()
			if !present {
				return false
			}
			txPackets := float64(flowState.GetCounters().GetOutPkts())
			if txPackets == 0 {
				return false
			}
			rxPackets := float64(flowState.GetCounters().GetInPkts())
			lossPct := (txPackets - rxPackets) * 100 / txPackets
			return lossPct > 99.0
		}).Await(t)
		if !ok {
			t.Errorf("Traffic %s is expected to fail: want > 99%% failure", flow)
		} else {
			t.Logf("Traffic Test Passed for flow %s! Loss seen as expected", flow)
		}
	}
}
