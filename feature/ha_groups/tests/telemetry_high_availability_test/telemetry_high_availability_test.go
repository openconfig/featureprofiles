// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
// Package telemetry_high_availability_test implements HA-1.0
package telemetry_high_availability_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
)

var (
	haGroupID         = uint8(1)
	haPriority1Change = uint8(200)
	haPriority1       = uint8(90)
	numOfPorts        = int(4)
)

// TestMain sets up the test environment.
func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func interfaceStatusVerification(t *testing.T, dut1, dut2 *ondatra.DUTDevice) {
	t.Helper()
	p := gnmi.OC()
	expectedStatus := oc.Interface_AdminStatus_UP

	// Define a slice of DUT devices.
	duts := []*ondatra.DUTDevice{dut1, dut2}

	for _, dut := range duts {
		for i := 1; i <= numOfPorts; i++ {
			portName := fmt.Sprintf("port%d", i)
			port := strings.ToLower(dut.Port(t, portName).Name())
			status := gnmi.Get(t, dut, p.Interface(port).AdminStatus().State())
			if status != expectedStatus {
				t.Errorf("Interface %s on %s Status: got %v, want %v", port, dut.Name(), status, expectedStatus)
			}
			t.Logf("%s: %s: %v", dut.Name(), port, status)
		}
	}
}

func eventHaStateChange(t *testing.T, dut1, dut2 *ondatra.DUTDevice, haPriority uint8, preempt bool) {
	t.Helper()
	d := gnmi.OC()

	gnmi.Update(t, dut1, d.HaGroup(haGroupID).Priority().Config(), haPriority)

	t.Logf("Waiting for commit to complete")
	time.Sleep(2 * time.Minute)

	gnmi.Update(t, dut1, d.HaGroup(haGroupID).Preempt().Config(), preempt)
	gnmi.Update(t, dut2, d.HaGroup(haGroupID).Preempt().Config(), preempt)

	t.Logf("Waiting for commit to complete")
	time.Sleep(2 * time.Minute)

}

func verifyHAState(t *testing.T, dut1, dut2 *ondatra.DUTDevice, expectedHAState1 oc.E_HaGroup_HaState,
	expectedHAState2 oc.E_HaGroup_HaState) {
	t.Helper()

	p := gnmi.OC()
	haGroupState1 := gnmi.Get(t, dut1, p.HaGroup(haGroupID).State())
	haState1 := haGroupState1.GetHaState()
	if haState1 != expectedHAState1 {
		t.Errorf("DUT1 HA state: got %v, want %v", haState1, expectedHAState1)
	} else {
		t.Logf("%s: HA state: %v: is as expected", dut1.Name(), haState1)
	}

	haGroupState2 := gnmi.Get(t, dut2, p.HaGroup(haGroupID).State())
	haState2 := haGroupState2.GetHaState()
	if haState2 != expectedHAState2 {
		t.Errorf("DUT2 HA state: got %v, want %v", haState2, expectedHAState2)
	} else {
		t.Logf("%s: HA state: %v: is as expected", dut2.Name(), haState2)
	}
}

func eventHaStateChangeVerification(t *testing.T, dut1, dut2 *ondatra.DUTDevice) {
	t.Helper()
	t.Logf("Changing HA state on %s to passive and %s to active", dut1.Name(), dut2.Name())
	eventHaStateChange(t, dut1, dut2, haPriority1Change, true)
	verifyHAState(t, dut1, dut2, oc.HaGroup_HaState_PASSIVE, oc.HaGroup_HaState_ACTIVE)

	t.Logf("Changing HA state on %s to active and %s to passive", dut1.Name(), dut2.Name())
	eventHaStateChange(t, dut1, dut2, haPriority1, false)
	verifyHAState(t, dut1, dut2, oc.HaGroup_HaState_ACTIVE, oc.HaGroup_HaState_PASSIVE)

	t.Logf("HA state verification after event ha state change completed successfully")
}

func configureAndVerifyHaEnabled(t *testing.T, dut1, dut2 *ondatra.DUTDevice) {
	t.Helper()
	d := gnmi.OC()

	gnmi.Update(t, dut1, d.HaGroup(haGroupID).HaEnabled().Config(), true)
	gnmi.Update(t, dut2, d.HaGroup(haGroupID).HaEnabled().Config(), true)
	t.Logf("Waiting for commit to complete")
	time.Sleep(2 * time.Minute)

	haEnabled1 := gnmi.Get(t, dut1, d.HaGroup(haGroupID).HaEnabled().Config())
	if haEnabled1 != true {
		t.Errorf("dut1 HaEnabled: got %v, want %v", haEnabled1, true)
	} else {
		t.Logf("dut1 HaEnabled: got %v is as expected", haEnabled1)
	}

	haEnabled2 := gnmi.Get(t, dut2, d.HaGroup(haGroupID).HaEnabled().Config())
	if haEnabled2 != true {
		t.Errorf("dut2 HaEnabled: got %v, want %v", haEnabled2, true)
	} else {
		t.Logf("dut2 HaEnabled: got %v is as expected", haEnabled2)
	}
}

func configureAndVerifyHaMode(t *testing.T, dut1, dut2 *ondatra.DUTDevice) {
	t.Helper()
	d := gnmi.OC()

	gnmi.Update(t, dut1, d.HaGroup(haGroupID).HaMode().Config(), oc.HaGroup_HaMode_ACTIVE_PASSIVE)
	gnmi.Update(t, dut2, d.HaGroup(haGroupID).HaMode().Config(), oc.HaGroup_HaMode_ACTIVE_PASSIVE)
	t.Logf("Waiting for commit to complete")
	time.Sleep(2 * time.Minute)

	haMode1 := gnmi.Get(t, dut1, d.HaGroup(haGroupID).HaMode().Config())
	if haMode1 != oc.HaGroup_HaMode_ACTIVE_PASSIVE {
		t.Errorf("dut1 HaMode: got %v, want %v", haMode1, oc.HaGroup_HaMode_ACTIVE_PASSIVE)
	} else {
		t.Logf("dut1 HaMode: got %v is as expected", haMode1)
	}

	haMode2 := gnmi.Get(t, dut2, d.HaGroup(haGroupID).HaMode().Config())
	if haMode2 != oc.HaGroup_HaMode_ACTIVE_PASSIVE {
		t.Errorf("dut2 HaMode: got %v, want %v", haMode2, oc.HaGroup_HaMode_ACTIVE_PASSIVE)
	} else {
		t.Logf("dut2 HaMode: got %v is as expected", haMode2)
	}
}

// TestFirewallHighAvailability tests the firewall high availability oc paths.
func TestFirewallHighAvailability(t *testing.T) {
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")

	t.Run("ha_state_active_passive_interface_status_before_event", func(t *testing.T) {
		verifyHAState(t, dut1, dut2, oc.HaGroup_HaState_ACTIVE, oc.HaGroup_HaState_PASSIVE)
		interfaceStatusVerification(t, dut1, dut2)
	})

	t.Run("event_ha_state_change_verification", func(t *testing.T) {
		eventHaStateChangeVerification(t, dut1, dut2)
	})

	t.Run("ha_state_active_passive_interface_status_after_event", func(t *testing.T) {
		verifyHAState(t, dut1, dut2, oc.HaGroup_HaState_PASSIVE, oc.HaGroup_HaState_ACTIVE)
		interfaceStatusVerification(t, dut1, dut2)
	})

	t.Run("configure_and_verify_ha_enabled", func(t *testing.T) {
		configureAndVerifyHaEnabled(t, dut1, dut2)
	})

	t.Run("configure_and_verify_ha_mode", func(t *testing.T) {
		configureAndVerifyHaMode(t, dut1, dut2)
	})
}
