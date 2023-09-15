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

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygnmi/ygnmi"
)

// IsTrafficStopped checks to see if all the flows are completely stopped
func IsTrafficStopped(t testing.TB, otg *otg.OTG, c gosnappi.Config) {

	for _, f := range c.Flows().Items() {
		flow := gnmi.OTG().Flow(f.Name())

		gnmi.Watch(t, otg, flow.Transmit().State(), 30*time.Second, func(val *ygnmi.Value[bool]) bool {
			transmitState, ok := val.Val()
			return ok && !transmitState
		}).Await(t)
	}
}
