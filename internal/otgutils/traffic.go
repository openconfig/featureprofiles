// Copyright 2025 Google LLC
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

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ygnmi/ygnmi"

	otgtelemetry "github.com/openconfig/ondatra/gnmi/otg"
)

const (
	timeout = 1 * time.Minute
)

// TrafficTestParams encapsulates the parameters required for traffic tests, including the OTG configuration and the ATE device.
type TrafficTestParams struct {
	Config gosnappi.Config
	Ate    *ondatra.ATEDevice
}

// WaitForTxPacketsReceived waits for the sum of transmitted and received packet counts across all flows to match, indicating that traffic has converged.
func WaitForTxPacketsReceived(t *testing.T, params TrafficTestParams) {
	t.Helper()
	otg := params.Ate.OTG()
	t.Log("Waiting for the TxPackets to arrive at Rx")

	// Use a single wildcard watch on all flows' counters to check all flows in parallel
	countersPath := gnmi.OTG().FlowAny().Counters().State()
	gnmi.WatchAll(t, otg, countersPath, timeout, func(v *ygnmi.Value[*otgtelemetry.Flow_Counters]) bool {
		// Efficiently query all flows' counter states using LookupAll
		allFlowCounters := gnmi.LookupAll(t, otg, gnmi.OTG().FlowAny().Counters().State())

		if len(allFlowCounters) == 0 {
			return false
		}

		var totalTxPkts, totalRxPkts uint64
		for _, flowCounters := range allFlowCounters {
			counters, ok := flowCounters.Val()
			if !ok {
				return false
			}

			txPkts := counters.GetOutPkts()
			rxPkts := counters.GetInPkts()

			totalTxPkts += txPkts
			totalRxPkts += rxPkts
		}

		// Sum of all flows: total tx > 0 and total tx == total rx
		return totalTxPkts > 0 && totalTxPkts == totalRxPkts
	}).Await(t)
}
