// Copyright 2024 Google LLC
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

	fperrorspb "github.com/openconfig/featureprofiles/internal/fperrors/fperrors_go_proto"
	otgtelemetry "github.com/openconfig/ondatra/gnmi/otg"
)

// ExpectedTrafficLoss checks if traffic loss for a given flow is within the expected range (minLossPct to maxLossPct).
// It waits up to a configurable waitTime with a default of 45 seconds for the traffic loss percentage to be within
// the expected range, then fails the test with a standard error message if the validation fails.
func ExpectedTrafficLoss(t testing.TB, otg *otg.OTG, flowName string, minLossPct, maxLossPct float64, waitTime ...time.Duration) {
	var waitT time.Duration = 45
	if len(waitTime) > 0 {
		waitT = waitTime[0]
	}
	t.Helper()
	_, ok := gnmi.Watch(t, otg, gnmi.OTG().Flow(flowName).State(), waitT*time.Second, func(val *ygnmi.Value[*otgtelemetry.Flow]) bool {
		recvMetric, present := val.Val()
		if !present || recvMetric == nil || recvMetric.GetCounters() == nil {
			return false
		}
		txPackets := float64(recvMetric.GetCounters().GetOutPkts())
		rxPackets := float64(recvMetric.GetCounters().GetInPkts())
		if txPackets == 0 || rxPackets > txPackets {
			return false
		}
		lossPct := (txPackets - rxPackets) * 100.0 / txPackets
		return lossPct >= minLossPct-0.01 && lossPct <= maxLossPct+0.01
	}).Await(t)

	if ok {
		t.Logf("Traffic validation successful for flow %s", flowName)
		return
	}

	recvMetric := gnmi.Get(t, otg, gnmi.OTG().Flow(flowName).State())
	if recvMetric == nil || recvMetric.GetCounters() == nil {
		t.Fatalf("[%s] OTG traffic generation failed: missing metrics for flow %s", fperrorspb.ErrorCategory_ERROR_CATEGORY_TRAFFIC_GENERATION_FAILED.String(), flowName)
	}

	txPackets := float64(recvMetric.GetCounters().GetOutPkts())
	rxPackets := float64(recvMetric.GetCounters().GetInPkts())
	if txPackets == 0 {
		t.Fatalf("[%s] OTG traffic generation failed: TxPkts = 0 for flow %s", fperrorspb.ErrorCategory_ERROR_CATEGORY_TRAFFIC_GENERATION_FAILED.String(), flowName)
	}
	if rxPackets > txPackets {
		t.Fatalf("[%s] OTG traffic validation anomaly: RxPkts (%v) > TxPkts (%v)", fperrorspb.ErrorCategory_ERROR_CATEGORY_TRAFFIC_VALIDATION_ANOMALY.String(), rxPackets, txPackets)
	}
	lossPct := (txPackets - rxPackets) * 100.0 / txPackets

	t.Fatalf("[%s] Generic Test Assertion Failure: Flow %s: got %v, want between %v and %v", fperrorspb.ErrorCategory_ERROR_CATEGORY_TEST_ASSERTION_FAILURE.String(), flowName, lossPct, minLossPct, maxLossPct)
}
