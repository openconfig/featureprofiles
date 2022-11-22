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

package chassis_reboot_status_and_cancel_test

import (
	"context"
	"testing"

	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	spb "github.com/openconfig/gnoi/system"
	tpb "github.com/openconfig/gnoi/types"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi/oc"
)

const (
	oneMinuteInNanoSecond = 6e10
	rebootDelay           = 120
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Test cases:
//  1) Send gNOI reboot status request.
//   - Check the reboot status before sending reboot request.
//     - Verify the reboot status is not active.
//   - Check the reboot status after sending reboot request.
//     - Verify the reboot status is active.
//     - Verify the reason from reboot status response matches reboot message.
//     - Verify the wait time from reboot status response matches reboot delay.
//  2) Cancel gNOI reboot request.
//   - Cancel reboot request before the test
//     - Verify that there is no response error returned.
//   - Send reboot request with delay.
//     - Verify the reboot status is active.
//   - Send reboot cancel request.
//     - Verify the reboot status is not active.
//
// Topology:
//   dut:port1 <--> ate:port1
//
// Test notes:
//  - gnoi operation commands can be sent and tested using CLI command grpcurl.
//    https://github.com/fullstorydev/grpcurl
//

func TestRebootStatus(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	gnoiClient := dut.RawAPIs().GNOI().Default(t)

	cases := []struct {
		desc          string
		rebootRequest *spb.RebootRequest
		rebootActive  bool
		cancelReboot  bool
	}{
		{
			desc:          "no reboot requested",
			rebootRequest: nil,
			rebootActive:  false,
		},
		{
			desc: "reboot requested with delay",
			rebootRequest: &spb.RebootRequest{
				Method:  spb.RebootMethod_COLD,
				Delay:   rebootDelay * oneMinuteInNanoSecond,
				Message: "Reboot chassis with delay",
				Force:   true,
			},
			rebootActive: true,
		},
	}

	statusReq := &spb.RebootStatusRequest{Subcomponents: []*tpb.Path{}}
	if !*deviations.GNOIStatusWithEmptySubcomponent {
		supervisors := components.FindComponentsByType(t, dut, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD)
		// the test reboots the chasis, so any subcomponent should be ok to check the status
		statusReq = &spb.RebootStatusRequest{
			Subcomponents: []*tpb.Path{
				components.GetSubcomponentPath(supervisors[0]),
			},
		}
	}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			if tc.rebootRequest != nil {
				t.Logf("Send reboot request: %v", tc.rebootRequest)
				rebootResponse, err := gnoiClient.System().Reboot(context.Background(), tc.rebootRequest)
				t.Logf("Got reboot response: %v, err: %v", rebootResponse, err)
				if err != nil {
					t.Fatalf("Failed to request reboot with unexpected err: %v", err)
				}
			}
			resp, err := gnoiClient.System().RebootStatus(context.Background(), statusReq)
			t.Logf("DUT rebootStatus: %v, err: %v", resp, err)
			if err != nil {
				t.Fatalf("Failed to get reboot status with unexpected err: %v", err)
			}
			if resp.GetActive() != tc.rebootActive {
				t.Errorf("resp.GetActive(): got %v, want %v", resp.GetActive(), tc.rebootActive)
			}

			if tc.rebootRequest != nil {
				if resp.GetReason() != tc.rebootRequest.GetMessage() {
					t.Errorf("resp.GetReason(): got %v, want %v", resp.GetReason(), tc.rebootRequest.GetMessage())
				}
				if resp.GetWait() > tc.rebootRequest.GetDelay() {
					t.Errorf("resp.GetWait(): got %v, want <= %v", resp.GetWait(), tc.rebootRequest.GetDelay())
				}
				if resp.GetWhen() == 0 {
					t.Errorf("resp.GetWhen(): got %v, want > 0", resp.GetWhen())
				}
			}
		})

		t.Logf("Cancel reboot request after the test")

		rebootCancel, err := gnoiClient.System().CancelReboot(context.Background(), &spb.CancelRebootRequest{})
		if err != nil {
			t.Fatalf("Failed to cancel reboot with unexpected err: %v", err)
		}
		t.Logf("DUT CancelReboot response: %v, err: %v", rebootCancel, err)
	}
}

func TestCancelReboot(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	gnoiClient := dut.RawAPIs().GNOI().Default(t)

	rebootRequest := &spb.RebootRequest{
		Method:  spb.RebootMethod_COLD,
		Delay:   rebootDelay * oneMinuteInNanoSecond,
		Message: "Reboot chassis with delay",
		Force:   true,
	}

	t.Logf("Cancel reboot request before the test")
	rebootCancel, err := gnoiClient.System().CancelReboot(context.Background(), &spb.CancelRebootRequest{})
	if err != nil {
		t.Fatalf("Failed to cancel reboot with unexpected err: %v", err)
	}
	t.Logf("DUT CancelReboot response: %v, err: %v", rebootCancel, err)

	t.Logf("Send reboot request: %v", rebootRequest)
	rebootResponse, err := gnoiClient.System().Reboot(context.Background(), rebootRequest)
	t.Logf("Got reboot response: %v, err: %v", rebootResponse, err)
	if err != nil {
		t.Fatalf("Failed to request reboot with unexpected err: %v", err)
	}
	statusReq := &spb.RebootStatusRequest{Subcomponents: []*tpb.Path{}}
	if !*deviations.GNOIStatusWithEmptySubcomponent {
		supervisors := components.FindComponentsByType(t, dut, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD)
		// the test reboots the chasis, so any subcomponent should be ok to check the status
		statusReq = &spb.RebootStatusRequest{
			Subcomponents: []*tpb.Path{
				components.GetSubcomponentPath(supervisors[0]),
			},
		}
	}
	rebootStatus, err := gnoiClient.System().RebootStatus(context.Background(), statusReq)
	t.Logf("DUT rebootStatus: %v, err: %v", rebootStatus, err)
	if err != nil {
		t.Fatalf("Failed to get reboot status with unexpected err: %v", err)
	}
	if !rebootStatus.GetActive() {
		t.Errorf("rebootStatus.GetActive(): got %v, want true", rebootStatus.GetActive())
	}

	t.Logf("Cancel reboot request: %v", rebootRequest)
	rebootCancel, err = gnoiClient.System().CancelReboot(context.Background(), &spb.CancelRebootRequest{})
	t.Logf("DUT CancelReboot response: %v, err: %v", rebootCancel, err)
	if err != nil {
		t.Fatalf("Failed to cancel reboot with unexpected err: %v", err)
	}

	rebootStatus, err = gnoiClient.System().RebootStatus(context.Background(), statusReq)
	t.Logf("DUT rebootStatus: %v, err: %v", rebootStatus, err)
	if err != nil {
		t.Fatalf("Failed to get reboot status with unexpected err: %v", err)
	}
	if rebootStatus.GetActive() {
		t.Errorf("rebootStatus.GetActive(): got %v, want false", rebootStatus.GetActive())
	}
}
