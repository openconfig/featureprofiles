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
		rxPackets, ok := val.Val()
		return ok && rxPackets == txPkts
	}).Await(t)
	rx, _ := rxPkts.Val()

	return txPkts, rx

}
