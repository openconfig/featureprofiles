// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package gnoi provides utilities for interacting with the gNOI API.
package gnoi

import (
	"context"
	"fmt"
	"testing"
	"time"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
)

var (
	ocAgentTerminationCmd = map[ondatra.Vendor]string{
		ondatra.ARISTA: "agent Octa terminate",
	}
	ocAgentDaemon = map[ondatra.Vendor]string{
		ondatra.ARISTA: "Octa",
	}
)

// TerminateOCAgent terminates the OpenConfig agent on the DUT.
func TerminateOCAgent(t *testing.T, dut *ondatra.DUTDevice, waitForRestart bool) error {
	t.Helper()

	ctx := context.Background()
	cli := dut.RawAPIs().CLI(t)

	cmd, ok := ocAgentTerminationCmd[dut.Vendor()]
	if !ok {
		t.Errorf("No command found for vendor %v", dut.Vendor())
	}
	res, err := cli.RunCommand(ctx, cmd)
	if err != nil {
		return fmt.Errorf("error executing command %q: %v", cmd, err)
	}
	if res.Error() != "" {
		return fmt.Errorf("error executing command %q: %v", cmd, res.Error())
	}

	if ocAgent, ok := ocAgentDaemon[dut.Vendor()]; ok && waitForRestart {
		gnmi.WatchAll(
			t,
			dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_ON_CHANGE)),
			gnmi.OC().System().ProcessAny().State(),
			time.Minute,
			func(p *ygnmi.Value[*oc.System_Process]) bool {
				val, ok := p.Val()
				if !ok {
					return false
				}
				return val.GetName() == ocAgent
			},
		)
	}
	return nil
}
