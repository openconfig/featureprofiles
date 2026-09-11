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

package controller_card_redundancy_test

import (
	"context"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/args"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	spb "github.com/openconfig/gnoi/system"
	tpb "github.com/openconfig/gnoi/types"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygnmi/ygnmi"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	controlcardType     = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD
	primaryController   = oc.Platform_ComponentRedundantRole_PRIMARY
	secondaryController = oc.Platform_ComponentRedundantRole_SECONDARY
	switchTrigger       = oc.PlatformTypes_ComponentRedundantRoleSwitchoverReasonTrigger_USER_INITIATED
	maxSwitchoverTime   = 1200
)

// Function for verifying the stability of SUP cards after switchover
func waitForSwitchover(t *testing.T, dut *ondatra.DUTDevice) {
	startSwitchover := time.Now()
	t.Logf("Wait for new active RP to boot up by polling the telemetry output.")
	for {
		var currentTime string
		t.Logf("Time elapsed %.2f seconds since switchover started.", time.Since(startSwitchover).Seconds())
		time.Sleep(30 * time.Second)
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			currentTime = gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State())
		}); errMsg != nil {
			t.Logf("Got testt.CaptureFatal errMsg: %s, keep polling ...", *errMsg)
		} else {
			t.Logf("RP switchover has completed successfully with received time: %v", currentTime)
			break
		}
		if got, want := uint64(time.Since(startSwitchover).Seconds()), uint64(maxSwitchoverTime); got >= want {
			t.Fatalf("time.Since(startSwitchover): got %v, want < %v", got, want)
		}
	}
	t.Logf("RP switchover time: %.2f seconds", time.Since(startSwitchover).Seconds())
}

func testControllerCardSwitchover(t *testing.T, dut *ondatra.DUTDevice, controllerCards []string) {
	// Collect active and standby controller cards before the switchover
	rpStandbyBeforeSwitch, rpActiveBeforeSwitch := components.FindStandbyControllerCard(t, dut, controllerCards)
	t.Logf("Detected rpStandby: %v, rpActive: %v", rpStandbyBeforeSwitch, rpActiveBeforeSwitch)

	// Check if active RP is ready for switchover
	switchoverReady := gnmi.OC().Component(rpActiveBeforeSwitch).SwitchoverReady()
	gnmi.Await(t, dut, switchoverReady.State(), 10*time.Minute, true)
	t.Logf("SwitchoverReady().Get(t): %v", gnmi.Get(t, dut, switchoverReady.State()))
	if got, want := gnmi.Get(t, dut, switchoverReady.State()), true; got != want {
		t.Errorf("switchoverReady.Get(t): got %v, want %v", got, want)
	}

	// Initiate a RP switchover
	gnoiClient := dut.RawAPIs().GNOI(t)
	useNameOnly := deviations.GNOISubcomponentPath(dut)
	switchoverRequest := &spb.SwitchControlProcessorRequest{
		ControlProcessor: components.GetSubcomponentPath(rpStandbyBeforeSwitch, useNameOnly),
	}
	t.Logf("switchoverRequest: %v", switchoverRequest)
	switchoverResponse, err := gnoiClient.System().SwitchControlProcessor(context.Background(), switchoverRequest)
	if err != nil {
		t.Fatalf("Failed to perform control processor switchover with unexpected err: %v", err)
	}
	t.Logf("gnoiClient.System().SwitchControlProcessor() response: %v, err: %v", switchoverResponse, err)

	want := rpStandbyBeforeSwitch
	got := ""
	if deviations.GNOISubcomponentPath(dut) {
		got = switchoverResponse.GetControlProcessor().GetElem()[0].GetName()
	} else {
		got = switchoverResponse.GetControlProcessor().GetElem()[1].GetKey()["name"]
	}
	if got != want {
		t.Fatalf("switchoverResponse.GetControlProcessor().GetElem()[0].GetName(): got %v, want %v", got, want)
	}
	if got, want := switchoverResponse.GetVersion(), ""; got == want {
		t.Errorf("switchoverResponse.GetVersion(): got %v, want non-empty version", got)
	}
	if got := switchoverResponse.GetUptime(); got == 0 {
		t.Errorf("switchoverResponse.GetUptime(): got %v, want > 0", got)
	}
	// Waiting for device to be stable after the switchover.
	waitForSwitchover(t, dut)

	// RP roles after the switchover
	rpStandbyAfterSwitch, rpActiveAfterSwitch := components.FindStandbyControllerCard(t, dut, controllerCards)
	t.Logf("Found standbyRP after switchover: %v, activeRP: %v", rpStandbyAfterSwitch, rpActiveAfterSwitch)

	if got, want := rpActiveAfterSwitch, rpStandbyBeforeSwitch; got != want {
		t.Errorf("Get rpActiveAfterSwitch: got %v, want %v", got, want)
	}
	if got, want := rpStandbyAfterSwitch, rpActiveBeforeSwitch; got != want {
		t.Errorf("Get rpStandbyAfterSwitch: got %v, want %v", got, want)
	}

	// Validate controller card last switchover time
	lastSwitchoverTime := gnmi.OC().Component(rpActiveAfterSwitch).LastSwitchoverTime()
	lastSwitchoverTimeCard := gnmi.Get(t, dut, lastSwitchoverTime.State())
	if !(gnmi.Lookup(t, dut, lastSwitchoverTime.State()).IsPresent()) {
		t.Errorf("Controller card last switchover time is not returning a valid value for %s", lastSwitchoverTime.State())
	}
	t.Logf("The value of last switchover time is %v", lastSwitchoverTimeCard)
	// Validate controller card last switchover reason trigger
	lastSwitchoverReasonTrigger := gnmi.OC().Component(rpActiveAfterSwitch).LastSwitchoverReason().Trigger()
	lastSwitchoverReasonTriggerCard := gnmi.Get(t, dut, lastSwitchoverReasonTrigger.State())
	if !(gnmi.Lookup(t, dut, lastSwitchoverReasonTrigger.State()).IsPresent()) {
		t.Errorf("Controller card last switchover reason trigger is not returning a valid value for %s", lastSwitchoverReasonTrigger.State())
	}
	t.Logf("The value of last switchover reason trigger is %v", lastSwitchoverReasonTriggerCard)
	// Validate controller card last switchover reason details
	lastSwitchoverReasonDetails := gnmi.OC().Component(rpActiveAfterSwitch).LastSwitchoverReason().Details()
	lastSwitchoverReasonDetailsCard := gnmi.Get(t, dut, lastSwitchoverReasonDetails.State())
	if !(gnmi.Lookup(t, dut, lastSwitchoverReasonDetails.State()).IsPresent()) {
		t.Errorf("Controller card last switchover reason details is not returning a valid value for %s", lastSwitchoverReasonDetails.State())
	}
	t.Logf("The value of last switchover reason details is %v", lastSwitchoverReasonDetailsCard)

	// Verify that all controller_cards has switchover-ready=TRUE
	switchoverReadyActiverp := gnmi.OC().Component(rpActiveAfterSwitch).SwitchoverReady()
	switchoverReadyStandbyrp := gnmi.OC().Component(rpStandbyAfterSwitch).SwitchoverReady()
	gnmi.Await(t, dut, switchoverReadyActiverp.State(), 30*time.Minute, true)
	gnmi.Await(t, dut, switchoverReadyStandbyrp.State(), 30*time.Minute, true)
	t.Logf("SwitchoverReady().Get(t): %v", gnmi.Get(t, dut, switchoverReady.State()))
	if got, want := gnmi.Get(t, dut, switchoverReadyActiverp.State()), true; got != want {
		t.Errorf("switchoverReady.Get(t): got %v, want %v", got, want)
	}
	if got, want := gnmi.Get(t, dut, switchoverReadyStandbyrp.State()), true; got != want {
		t.Errorf("switchoverReady.Get(t): got %v, want %v", got, want)
	}
}

func testControllerCardInventory(t *testing.T, dut *ondatra.DUTDevice, controllerCards []string) {
	for _, controllerCard := range controllerCards {
		t.Logf("\n\n VALIDATE %s: \n\n", controllerCard)

		// Get entire component state in one call
		component := gnmi.OC().Component(controllerCard)
		componentState := gnmi.Get(t, dut, component.State())

		// Verify component data is present
		if !gnmi.Lookup(t, dut, component.State()).IsPresent() {
			t.Errorf("Controller card component is not returning a valid value for %s", component.State())
			continue
		}

		// Validate all fields from the fetched component state
		if componentState == nil {
			t.Errorf("Controller card component state is nil for %s", controllerCard)
			continue
		}

		// Log and validate empty slots
		if componentState.Empty != nil {
			t.Logf("The value of empty slots is %v", *componentState.Empty)
		} else {
			t.Errorf("Controller card empty slot is not returning a valid value for %s", controllerCard)
		}

		// Log and validate location
		if componentState.Location != nil {
			t.Logf("The value of location is %v", *componentState.Location)
		} else {
			t.Errorf("Controller card location is not returning a valid value for %s", controllerCard)
		}

		// Log and validate oper status
		if componentState.OperStatus != 0 {
			t.Logf("The value of oper status is %v", componentState.OperStatus)
		} else {
			t.Errorf("Controller card oper status is not returning a valid value for %s", controllerCard)
		}

		// Log and validate switchover ready
		if componentState.SwitchoverReady != nil {
			t.Logf("The value of switchover ready is %v", *componentState.SwitchoverReady)
		} else {
			t.Errorf("Controller card switchover ready is not returning a valid value for %s", controllerCard)
		}

		// Log and validate redundant role
		if componentState.RedundantRole != 0 {
			t.Logf("The value of redundant role is %v", componentState.RedundantRole)
		} else {
			t.Errorf("Controller card redundant role is not returning a valid value for %s", controllerCard)
		}

		// Log and validate last reboot time
		if componentState.LastRebootTime != nil {
			t.Logf("The value of last reboot time is %v", *componentState.LastRebootTime)
		} else {
			t.Errorf("Controller card last reboot time is not returning a valid value for %s", controllerCard)
		}

		// Log and validate last reboot reason
		if componentState.LastRebootReason != 0 {
			t.Logf("The value of last reboot reason is %v", componentState.LastRebootReason)
		} else {
			t.Errorf("Controller card last reboot reason is not returning a valid value for %s", controllerCard)
		}

		// Log and validate hardware version
		if componentState.HardwareVersion != nil {
			t.Logf("The value of hardware version is %v", *componentState.HardwareVersion)
		} else {
			t.Errorf("Controller card hardware version is not returning a valid value for %s", controllerCard)
		}

		// Log and validate description
		if componentState.Description != nil {
			t.Logf("The value of description is %v", *componentState.Description)
		} else {
			t.Errorf("Controller card description is not returning a valid value for %s", controllerCard)
		}

		// Log and validate id
		if componentState.Id != nil {
			t.Logf("The value of id is %v", *componentState.Id)
		} else {
			t.Errorf("Controller card id is not returning a valid value for %s", controllerCard)
		}

		// Log and validate mfg name
		if componentState.MfgName != nil {
			t.Logf("The value of mfg name is %v", *componentState.MfgName)
		} else {
			t.Errorf("Controller card mfg name is not returning a valid value for %s", controllerCard)
		}

		// Log and validate name
		if componentState.Name != nil {
			t.Logf("The value of name is %v", *componentState.Name)
		} else {
			t.Errorf("Controller card name is not returning a valid value for %s", controllerCard)
		}

		// Log and validate parent
		if componentState.Parent != nil {
			t.Logf("The value of parent is %v", *componentState.Parent)
		} else {
			t.Errorf("Controller card parent is not returning a valid value for %s", controllerCard)
		}

		// Log and validate part no
		if componentState.PartNo != nil {
			t.Logf("The value of part no is %v", *componentState.PartNo)
		} else {
			t.Errorf("Controller card part no is not returning a valid value for %s", controllerCard)
		}

		// Log and validate serial no
		if componentState.SerialNo != nil {
			t.Logf("The value of serial no is %v", *componentState.SerialNo)
		} else {
			t.Errorf("Controller card serial no is not returning a valid value for %s", controllerCard)
		}

		// Log and validate type
		if componentState.Type != nil {
			t.Logf("The value of type is %v", componentState.Type)
		} else {
			t.Errorf("Controller card type is not returning a valid value for %s", controllerCard)
		}
	}
}

func testControllerCardRedundancy(t *testing.T, dut *ondatra.DUTDevice, controllerCards []string) {

	if deviations.SkipControllerCardPowerAdmin(dut) {
		t.Skipf("Skipping test for controller card redundancy test as the device does not support controller card power up/down")
	}

	// Collect active and standby controller cards before the switchover
	rpStandbyBeforeSwitch, rpActiveBeforeSwitch := components.FindStandbyControllerCard(t, dut, controllerCards)

	// Check if active RP is ready for switchover
	switchoverReady := gnmi.OC().Component(rpActiveBeforeSwitch).SwitchoverReady()
	gnmi.Await(t, dut, switchoverReady.State(), 10*time.Minute, true)
	t.Logf("SwitchoverReady().Get(t): %v", gnmi.Get(t, dut, switchoverReady.State()))
	if got, want := gnmi.Get(t, dut, switchoverReady.State()), true; got != want {
		t.Errorf("switchoverReady.Get(t): got %v, want %v", got, want)
	}

	// Initiate a RP switchover
	gnoiClient := dut.RawAPIs().GNOI(t)
	useNameOnly := deviations.GNOISubcomponentPath(dut)
	switchoverRequest := &spb.SwitchControlProcessorRequest{
		ControlProcessor: components.GetSubcomponentPath(rpStandbyBeforeSwitch, useNameOnly),
	}
	t.Logf("Wait for 10 seconds before triggering switchover")
	time.Sleep(10 * time.Second)
	t.Logf("switchoverRequest: %v", switchoverRequest)
	switchoverResponse, err := gnoiClient.System().SwitchControlProcessor(context.Background(), switchoverRequest)
	if err != nil {
		t.Fatalf("Failed to perform control processor switchover with unexpected err: %v", err)
	}
	t.Logf("gnoiClient.System().SwitchControlProcessor() response: %v, err: %v", switchoverResponse, err)
	// Polling the device to verify the stability of the device.
	waitForSwitchover(t, dut)
	rpStandbyAfterSwitch, rpActiveAfterSwitch := components.FindStandbyControllerCard(t, dut, controllerCards)
	switchoverReadyActiverp := gnmi.OC().Component(rpActiveAfterSwitch).SwitchoverReady()
	switchoverReadyStandbyrp := gnmi.OC().Component(rpStandbyAfterSwitch).SwitchoverReady()
	gnmi.Await(t, dut, switchoverReadyActiverp.State(), 30*time.Minute, true)
	gnmi.Await(t, dut, switchoverReadyStandbyrp.State(), 30*time.Minute, true)
	t.Logf("SwitchoverReady for active RP: %v, standby RP: %v", gnmi.Get(t, dut, switchoverReadyActiverp.State()), gnmi.Get(t, dut, switchoverReadyStandbyrp.State()))

	want := rpStandbyBeforeSwitch
	got := ""
	if deviations.GNOISubcomponentPath(dut) {
		got = switchoverResponse.GetControlProcessor().GetElem()[0].GetName()
	} else {
		got = switchoverResponse.GetControlProcessor().GetElem()[1].GetKey()["name"]
	}
	if got != want {
		t.Fatalf("switchoverResponse.GetControlProcessor().GetElem()[0].GetName(): got %v, want %v", got, want)
	}
	if got, want := switchoverResponse.GetVersion(), ""; got == want {
		t.Errorf("switchoverResponse.GetVersion(): got %v, want non-empty version", got)
	}
	if got := switchoverResponse.GetUptime(); got == 0 {
		t.Errorf("switchoverResponse.GetUptime(): got %v, want > 0", got)
	}

	// PowerDown the standby RP
	powerDownControllerCardRequest := &spb.RebootRequest{
		Method: spb.RebootMethod_POWERDOWN,
		Subcomponents: []*tpb.Path{
			components.GetSubcomponentPath(rpActiveBeforeSwitch, useNameOnly),
		},
	}
	t.Logf("powerDownControllerCardRequest: %v", powerDownControllerCardRequest)
	powerDownResponse, err := gnoiClient.System().Reboot(context.Background(), powerDownControllerCardRequest)
	if err != nil {
		t.Fatalf("Failed to perform standby RP powerdown with unexpected err: %v", err)
	}
	t.Logf("gnoiClient.System().PowerDown() response: %v, err: %v", powerDownResponse, err)

	t.Logf("Wait for 5 seconds to allow the sub component's reboot process to start")
	time.Sleep(5 * time.Second)

	// Iterate through the controller cards and check if the state is expected after standby RP powerdown
	for _, controllerCard := range controllerCards {
		// Check if the powered_Down controller card has power-admin-status present
		//if controllerCard == rpActiveBeforeSwitch && gnmi.Lookup(t, dut, gnmi.OC().Component(controllerCard).ControllerCard().PowerAdminState().State()).IsPresent() {
		if controllerCard == rpActiveBeforeSwitch {
			t.Logf("[%s] Waiting for Controller Card %s power-admin-state to reach %s", time.Now().Format("15:04:05.000"), controllerCard, oc.Platform_ComponentPowerType_POWER_DISABLED)
			powerStatus := gnmi.OC().Component(controllerCard).ControllerCard().PowerAdminState()
			powerStatusCard := gnmi.Await(t, dut, powerStatus.State(), 10*time.Minute, oc.Platform_ComponentPowerType_POWER_DISABLED)
			t.Logf("[%s] Controller card %s is in the state : %s after standby RP reboot", time.Now().Format("15:04:05.000"), controllerCard, gnmi.Get(t, dut, gnmi.OC().Component(controllerCard).ControllerCard().PowerAdminState().State()))
			t.Logf("[%s] Controller card %s is in the expected state : %s after standby RP reboot", time.Now().Format("15:04:05.000"), controllerCard, powerStatusCard)
		}
		if controllerCard == rpActiveBeforeSwitch && !gnmi.Lookup(t, dut, gnmi.OC().Component(controllerCard).ControllerCard().PowerAdminState().State()).IsPresent() {
			t.Errorf("Controller card %s is not populating the power-admin-state of the component after powerdown", controllerCard)
			continue
		}
		operStatus := gnmi.OC().Component(controllerCard).OperStatus()
		operStatusCard := gnmi.Lookup(t, dut, operStatus.State())
		if controllerCard == rpStandbyBeforeSwitch && !operStatusCard.IsPresent() {
			t.Logf("Controller card %s oper-state is not present / populated", controllerCard)
		} else {
			val, _ := operStatusCard.Val()
			t.Logf("Controller card %s is in the state : %s after standby RP reboot", controllerCard, val)
		}
		redundantRole := gnmi.OC().Component(controllerCard).RedundantRole()
		redundantRoleCard := gnmi.Lookup(t, dut, redundantRole.State())
		if controllerCard == rpStandbyBeforeSwitch && !redundantRoleCard.IsPresent() {
			t.Logf("Controller card %s  redundant-role is not present / populated", controllerCard)
		} else {
			val, _ := redundantRoleCard.Val()
			t.Logf("Controller card %s is in role : %s after standby RP reboot", controllerCard, val)
		}

		//if controllerCard == rpStandbyBeforeSwitch && operStatusCard == oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE && redundantRoleCard == oc.Platform_ComponentRedundantRole_PRIMARY {
		//	t.Logf("Controller card %s is in the expected state %s after standby RP reboot", controllerCard, operStatusCard)
		//}
	}
	t.Logf("Wait for 5 seconds before powerup the standby RP")
	time.Sleep(5 * time.Second)
	// PowerUp the standby RP
	powerUpControllerCardRequest := &spb.RebootRequest{
		Method: spb.RebootMethod_POWERUP,
		Subcomponents: []*tpb.Path{
			components.GetSubcomponentPath(rpActiveBeforeSwitch, useNameOnly),
		},
	}
	t.Logf("powerUpControllerCardRequest: %v", powerUpControllerCardRequest)
	powerUpResponse, err := gnoiClient.System().Reboot(context.Background(), powerUpControllerCardRequest)
	if err != nil {
		t.Fatalf("Failed to perform standby RP powerup with unexpected err: %v", err)
	}
	t.Logf("gnoiClient.System().Reboot() response: %v, err: %v", powerUpResponse, err)

	t.Logf("Wait for 5 seconds to allow the sub component's powerup process to start")
	time.Sleep(5 * time.Second)

	// wait for the RP to come to Power Enabled state
	powerStatus := gnmi.OC().Component(rpActiveBeforeSwitch).ControllerCard().PowerAdminState()
	powerStatusCard := gnmi.Await(t, dut, powerStatus.State(), 10*time.Minute, oc.Platform_ComponentPowerType_POWER_ENABLED)
	t.Logf("Controller card %s  power-status: %s", rpActiveBeforeSwitch, powerStatusCard)
	// Verify that all controller_cards has switchover-ready=TRUE
	switchoverReadyActiverp = gnmi.OC().Component(rpStandbyBeforeSwitch).SwitchoverReady()
	switchoverReadyStandbyrp = gnmi.OC().Component(rpActiveBeforeSwitch).SwitchoverReady()
	gnmi.Await(t, dut, switchoverReadyActiverp.State(), 30*time.Minute, true)
	gnmi.Await(t, dut, switchoverReadyStandbyrp.State(), 30*time.Minute, true)
	t.Logf("SwitchoverReady for active RP (%s): %v, standby RP (%s): %v", rpStandbyBeforeSwitch, gnmi.Get(t, dut, switchoverReadyActiverp.State()), rpActiveBeforeSwitch, gnmi.Get(t, dut, switchoverReadyStandbyrp.State()))
	if got, want := gnmi.Get(t, dut, switchoverReadyActiverp.State()), true; got != want {
		t.Errorf("switchoverReady.Get(t): got %v, want %v", got, want)
	}
	if got, want := gnmi.Get(t, dut, switchoverReadyStandbyrp.State()), true; got != want {
		t.Errorf("switchoverReady.Get(t): got %v, want %v", got, want)
	}
}

func testControllerCardLastRebootTime(t *testing.T, dut *ondatra.DUTDevice, controllerCards []string) {
	// Get the standby and active controller cards
	rpStandby, _ := components.FindStandbyControllerCard(t, dut, controllerCards)
	// Get the last reboot time of the standby controller card before the reboot
	lastRebootTime := gnmi.OC().Component(rpStandby).LastRebootTime()
	lastRebootTimeBefore := gnmi.Get(t, dut, lastRebootTime.State())
	// Reboot the standby controller card
	gnoiClient := dut.RawAPIs().GNOI(t)
	useNameOnly := deviations.GNOISubcomponentPath(dut)
	rebootControllerCardRequest := &spb.RebootRequest{
		Method: spb.RebootMethod_COLD,
		Subcomponents: []*tpb.Path{
			components.GetSubcomponentPath(rpStandby, useNameOnly),
		},
	}
	t.Logf("rebootControllerCardRequest: %v", rebootControllerCardRequest)
	startReboot := time.Now()
	rebootResponse, err := gnoiClient.System().Reboot(context.Background(), rebootControllerCardRequest)
	if err != nil {
		t.Fatalf("Failed to perform component reboot with unexpected err: %v", err)
	}
	t.Logf("gnoiClient.System().Reboot() response: %v, err: %v", rebootResponse, err)

	t.Logf("Wait for a minute to allow the sub component's reboot process to start")
	time.Sleep(1 * time.Minute)

	watch := gnmi.Watch(t, dut, gnmi.OC().Component(rpStandby).RedundantRole().State(), 30*time.Minute, func(val *ygnmi.Value[oc.E_Platform_ComponentRedundantRole]) bool {
		return val.IsPresent()
	})
	if val, ok := watch.Await(t); !ok {
		t.Fatalf("DUT did not reach target state within %v: got %v", 30*time.Minute, val)
	}
	t.Logf("Standby controller boot time: %.2f seconds", time.Since(startReboot).Seconds())

	// Wait for the telemetry agent to populate the last-reboot-time leaf
	watchTime := gnmi.Watch(t, dut, lastRebootTime.State(), 5*time.Minute, func(val *ygnmi.Value[uint64]) bool {
		return val.IsPresent()
	})
	if val, ok := watchTime.Await(t); !ok {
		t.Fatalf("last-reboot-time was not populated within 5 minutes: got %v", val)
	}

	// Get the last reboot time of the standby controller card after the reboot
	lastRebootTimeAfter := gnmi.Get(t, dut, lastRebootTime.State())

	if lastRebootTimeAfter < lastRebootTimeBefore {
		t.Errorf("LastRebootTime().Get(t): got %v, want > %v", lastRebootTimeAfter, lastRebootTimeBefore)
	}

}

func TestControllerCards(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	// Get Controller Card list that are inserted in the DUT.
	controllerCards := components.FindComponentsByType(t, dut, controlcardType)
	t.Logf("Found controller card list: %v", controllerCards)

	if *args.NumControllerCards >= 0 && len(controllerCards) != *args.NumControllerCards {
		t.Errorf("Incorrect number of controller cards: got %v, want exactly %v (specified by flag)", len(controllerCards), *args.NumControllerCards)
	}

	if got, want := len(controllerCards), 2; got < want {
		t.Errorf("Not enough controller cards for the test on %v: got %v, want at least %v", dut.Model(), got, want)
	}

	// Test cases.
	type testCase struct {
		name            string
		controllerCards []string
		testFunc        func(t *testing.T, dut *ondatra.DUTDevice, controllerCards []string)
	}

	testCases := []testCase{
		{
			name:            "TEST 1: Controller Card inventory",
			controllerCards: controllerCards,
			testFunc:        testControllerCardInventory,
		},
		{
			name:            "TEST 2: Controller Card switchover",
			controllerCards: controllerCards,
			testFunc:        testControllerCardSwitchover,
		},
		{
			name:            "TEST 3: Controller Card redundancy",
			controllerCards: controllerCards,
			testFunc:        testControllerCardRedundancy,
		},
		{
			name:            "TEST 4: Controller Card last reboot time",
			controllerCards: controllerCards,
			testFunc:        testControllerCardLastRebootTime,
		},
	}

	// Run the test cases.
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Description: %s", tc.name)
			tc.testFunc(t, dut, tc.controllerCards)
		})
	}
}
