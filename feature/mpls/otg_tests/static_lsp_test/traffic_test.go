// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package static_lsp_test

import (
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	otgtelemetry "github.com/openconfig/ondatra/gnmi/otg"
	"github.com/openconfig/ygnmi/ygnmi"
)

// runFlows measures a fresh fixed-packet batch for only the requested flows.
// The caller must push a fresh OTG config before every run.
func runFlows(t *testing.T, ate *ondatra.ATEDevice, flowNames []string) {
	t.Helper()
	runFlowsDuringChange(t, ate, flowNames, nil)
}

// runFlowsDuringChange establishes forwarding before invoking change while
// continuous traffic runs. It measures at least 10,000 packets and at least
// 1,000 additional packets after the change; loss validation uses the whole run.
// With a nil change, the configured fixed-packet duration is retained.
func runFlowsDuringChange(t *testing.T, ate *ondatra.ATEDevice, flowNames []string, change func()) {
	t.Helper()
	if len(flowNames) == 0 {
		t.Fatal("No traffic flows selected")
	}
	for _, name := range flowNames {
		// A zero sample before START separates this run from the preceding batch.
		// In particular, an old 10,000-packet sample must not end a new run at 179.
		awaitFlowCounters(t, ate, name, "fresh counters before START", func(c *otgtelemetry.Flow_Counters) bool {
			return c.GetOutPkts() == 0 && c.GetInPkts() == 0
		})
	}
	setFlowTransmit(t, ate, flowNames, gosnappi.StateTrafficFlowTransmitState.START)
	stopped := false
	defer func() {
		if !stopped {
			setFlowTransmit(t, ate, flowNames, gosnappi.StateTrafficFlowTransmitState.STOP)
		}
	}()
	targets := make(map[string]uint64)
	for _, name := range flowNames {
		targets[name] = trafficPackets
	}
	if change != nil {
		beforeChange := make(map[string]uint64)
		for _, name := range flowNames {
			baseline := awaitFlowCounters(t, ate, name, "forwarding baseline before churn", func(c *otgtelemetry.Flow_Counters) bool {
				return c.GetOutPkts() >= 1000 && c.GetInPkts() >= 1000
			})
			beforeChange[name] = baseline.GetOutPkts()
		}
		change()
		for _, name := range flowNames {
			// Some OTG servers report transmit=false even while counters advance.
			// Check packet progress across the change and require a further
			// post-change batch instead of relying on that boolean.
			counters := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(name).Counters().State())
			if counters.GetOutPkts() <= beforeChange[name] {
				t.Fatalf("Flow %s made no transmit progress during churn: before=%d after=%d", name, beforeChange[name], counters.GetOutPkts())
			}
			targets[name] = max(uint64(trafficPackets), counters.GetOutPkts()+1000)
		}
	}
	for _, name := range flowNames {
		awaitFlowCounters(t, ate, name, "complete measurement", func(c *otgtelemetry.Flow_Counters) bool {
			return c.GetOutPkts() >= targets[name]
		})
	}
	setFlowTransmit(t, ate, flowNames, gosnappi.StateTrafficFlowTransmitState.STOP)
	stopped = true
	for _, name := range flowNames {
		counters := awaitStoppedFlowCounters(t, ate, name)
		if counters.GetOutPkts() < targets[name] {
			t.Fatalf("Flow %s sent only %d packets, want at least %d", name, counters.GetOutPkts(), targets[name])
		}
		t.Logf("Fresh flow %s completed: Tx=%d Rx=%d (minimum Tx=%d)", name, counters.GetOutPkts(), counters.GetInPkts(), targets[name])
	}
}

func setFlowTransmit(t *testing.T, ate *ondatra.ATEDevice, flowNames []string, state gosnappi.StateTrafficFlowTransmitStateEnum) {
	t.Helper()
	control := gosnappi.NewControlState()
	control.Traffic().FlowTransmit().SetFlowNames(flowNames).SetState(state)
	ate.OTG().SetControlState(t, control)
}

func awaitFlowCounters(t *testing.T, ate *ondatra.ATEDevice, name, phase string, ready func(*otgtelemetry.Flow_Counters) bool) *otgtelemetry.Flow_Counters {
	t.Helper()
	last, ok := gnmi.Watch(t, ate.OTG(), gnmi.OTG().Flow(name).Counters().State(), time.Minute, func(v *ygnmi.Value[*otgtelemetry.Flow_Counters]) bool {
		c, present := v.Val()
		return present && c != nil && c.OutPkts != nil && c.InPkts != nil && ready(c)
	}).Await(t)
	if !ok {
		t.Fatalf("Flow %s did not reach %s; last counters: %v", name, phase, last)
	}
	counters, _ := last.Val()
	return counters
}

// awaitStoppedFlowCounters waits for the counters to settle after STOP. The
// transmit state alone cannot acknowledge STOP on servers that always report false.
func awaitStoppedFlowCounters(t *testing.T, ate *ondatra.ATEDevice, name string) *otgtelemetry.Flow_Counters {
	t.Helper()
	path := gnmi.OTG().Flow(name).Counters().State()
	previous := gnmi.Get(t, ate.OTG(), path)
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(time.Second)
		current := gnmi.Get(t, ate.OTG(), path)
		if current.GetOutPkts() == previous.GetOutPkts() && current.GetInPkts() == previous.GetInPkts() {
			return current
		}
		previous = current
	}
	t.Fatalf("Flow %s counters kept advancing after STOP: %v", name, previous)
	return nil
}
