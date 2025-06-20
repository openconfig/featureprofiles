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
	gnoiClient := dut.RawAPIs().GNOI(t)

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
	if !deviations.GNOIStatusWithEmptySubcomponent(dut) {
		statusReq.Subcomponents = append(statusReq.Subcomponents, getSubCompPath(t, dut))
	}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			if tc.rebootRequest != nil {
				t.Logf("Send reboot request: %v", tc.rebootRequest)
				rebootResponse, err := gnoiClient.System().Reboot(context.Background(), tc.rebootRequest)
				defer gnoiClient.System().CancelReboot(context.Background(), &spb.CancelRebootRequest{})
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
	gnoiClient := dut.RawAPIs().GNOI(t)

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
	defer gnoiClient.System().CancelReboot(context.Background(), &spb.CancelRebootRequest{})
	t.Logf("Got reboot response: %v, err: %v", rebootResponse, err)
	if err != nil {
		t.Fatalf("Failed to request reboot with unexpected err: %v", err)
	}
	statusReq := &spb.RebootStatusRequest{Subcomponents: []*tpb.Path{}}
	if !deviations.GNOIStatusWithEmptySubcomponent(dut) {
		statusReq.Subcomponents = append(statusReq.Subcomponents, getSubCompPath(t, dut))
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

func TestRebootPlusConfigPush(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	gnoiClient := dut.RawAPIs().GNOI(t)

	cases := []struct {
		desc          string
		rebootRequest *spb.RebootRequest
		rebootActive  bool
		cancelReboot  bool
	}{
		{
			desc: "reboot requested without delay",
			rebootRequest: &spb.RebootRequest{
				Method:  spb.RebootMethod_COLD,
				Delay:   0,
				Message: "Reboot chassis without delay",
				Force:   true,
			},
			rebootActive: true,
		},
	}

	statusReq := &spb.RebootStatusRequest{Subcomponents: []*tpb.Path{}}
	if !deviations.GNOIStatusWithEmptySubcomponent(dut) {
		statusReq.Subcomponents = append(statusReq.Subcomponents, getSubCompPath(t, dut))
	}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			if tc.rebootRequest != nil {
				t.Logf("Send reboot request: %v", tc.rebootRequest)
				rebootResponse, err := gnoiClient.System().Reboot(context.Background(), tc.rebootRequest)
				defer gnoiClient.System().CancelReboot(context.Background(), &spb.CancelRebootRequest{})
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

func TestControllerCardLargeConfigPushAndPull(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	// Get the number of ports on the DUT
	numPorts := len(dut.Ports())
	t.Logf("Number of ports on DUT: %d", numPorts)
	// Not assuming that oc base config is loaded.
	// Config the hostname to prevent the test failure when oc base config is not loaded
	gnmi.Replace(t, dut, gnmi.OC().System().Hostname().Config(), "ondatraHost")
	// Configuring the network instance as some devices only populate OC after configuration.
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	// Get Controller Card list that are inserted in the DUT.
	controllerCards := components.FindComponentsByType(t, dut, controlcardType)
	t.Logf("Found controller card list: %v", controllerCards)
	if got, want := len(controllerCards), 2; got < want {
		t.Fatalf("Not enough controller cards for the test on %v: got %v, want at least %v", dut.Model(), got, want)
	}

	params := configParams{
		NumLAGInterfaces:            numPorts,
		NumEthernetInterfacesPerLAG: 1,
		NumBGPNeighbors:             15,
	}

	gnoiClient := dut.RawAPIs().GNOI(t)
	ctx := context.Background()
	t.Run("testLargeConfigSetRequest", func(t *testing.T) {
		testLargeConfigSetRequest(ctx, t, dut, gnoiClient, &controllerCards)
	})
}

func testLargeConfigSetRequest(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, gnoiClient gnoigo.Clients, controllerCards *[]string) {
	activeStandbyCC := fetchActiveStandbyControllerCards(t, dut, controllerCards)
	switchoverControllerCards(ctx, t, dut, &switchoverControllerCardsConfig{&activeStandbyCC, gnoiClient, controllerCardSwitchoverTimeout})
	switchoverResponseTime := time.Now()

	var setResponseTime time.Time
	var setErr error
	for attempt := 1; attempt <= 4; attempt++ {
		if attempt > 1 {
			time.Sleep(sleepTimeBtwAttempts)
		}
		setErr = sendSetRequest(ctx, t, dut, setConfig)
		setResponseTime = time.Now()
		if setErr != nil {
			t.Logf("Error during set request on attempt %d: %v", attempt, setErr)
			if setResponseTime.Sub(switchoverResponseTime) > lastRequestTime {
				t.Fatalf("gNMI Set response after switchover time: %v, got non-zero status code", setResponseTime.Sub(switchoverResponseTime))
			}
			t.Logf("gNMI Set response after switchover time: %v, got non-zero grpc status code, retrying", setResponseTime.Sub(switchoverResponseTime))
			continue
		}
		if setResponseTime.Sub(switchoverResponseTime) > maxResponseTime {
			t.Fatalf("gNMI Set response after switchover time: %v, got SUCCESS, but exceeded max response time: %v", setResponseTime.Sub(switchoverResponseTime), maxResponseTime)
		}
		t.Logf("gNMI Set response after switchover time: %v, got SUCCESS", setResponseTime.Sub(switchoverResponseTime))
		break
	}
	if setErr != nil {
		t.Fatalf("Failed to send gNMI Set request after all attempts: %v", setErr)
	}
	// Retrieve configuration from DUT DUT using gNMI `GetRequest`.
	gnmiClient := dut.RawAPIs().GNMI(t)
	getRequest := buildGetRequest(t)

	ctxWithTimeout, cancelWithTimeout := context.WithTimeout(context.Background(), getRequestTimeout)
	defer cancelWithTimeout()
	fullConfig, err := gnmiClient.Get(ctxWithTimeout, getRequest)
	if err != nil {
		t.Fatalf("Error getting config: %v", err)
	}

	verifyConfiguredElements(t, dut, fullConfig)
}

func getSubCompPath(t *testing.T, dut *ondatra.DUTDevice) *tpb.Path {
	t.Helper()
	controllerCards := components.FindComponentsByType(t, dut, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD)
	if len(controllerCards) == 0 {
		t.Fatal("No controller card components found in DUT.")
	}
	activeRP := controllerCards[0]
	if len(controllerCards) == 2 {
		_, activeRP = components.FindStandbyControllerCard(t, dut, controllerCards)
	}
	useNameOnly := deviations.GNOISubcomponentPath(dut)
	return components.GetSubcomponentPath(activeRP, useNameOnly)
}
