// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package power_admin_down_up_test implements FP-1.1: Power admin DOWN/UP Test.
//
// It validates toggling /components/component/{linecard|fabric|controller-card}/config/power-admin-state:
//   - FP-1.1.1: Powering down a Linecard and a Fabric Card.
//   - FP-1.1.2: Attempting to power down both controller cards.
//
// Every README step is referenced in a comment of the form "FP-1.1.x <section>: <step>".
package power_admin_down_up_test

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/helpers"
	spb "github.com/openconfig/gnoi/system"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygnmi/ygnmi"
)

const (
	// linecardPowerTimeout bounds a linecard oper-status transition after a
	// power-admin-state change.
	linecardPowerTimeout = 20 * time.Minute
	// fabricPowerTimeout bounds a fabric card oper-status transition after a
	// power-admin-state change.
	fabricPowerTimeout = 15 * time.Minute
	// setupActiveTimeout bounds the Test Setup check that cards are ACTIVE.
	setupActiveTimeout = 5 * time.Minute
	// powerAdminStateTimeout bounds state/power-admin-state convergence to the
	// configured value.
	powerAdminStateTimeout = 5 * time.Minute
	// intfRecoveryTimeout bounds DUT port recovery after a linecard or fabric
	// card is powered back up.
	intfRecoveryTimeout = 20 * time.Minute
	// stuckCardCleanupTimeout bounds the cleanup wait for oper-status ACTIVE
	// when the test logic already waited the full linecard/fabric timeout after
	// POWER_ENABLED without the card becoming ACTIVE. Waiting the full timeout
	// again rarely helps and only delays the failure report.
	stuckCardCleanupTimeout = 5 * time.Minute

	// ccActiveTimeout bounds controller card oper-status transitions.
	ccActiveTimeout = 15 * time.Minute
	// switchoverReadyTimeout is the base timeout for switchover-ready.
	switchoverReadyTimeout = 10 * time.Minute
	// switchoverReadyRecoveryTimeout is the base timeout for switchover-ready
	// after a controller card was powered off and on again, which requires the
	// SECONDARY to fully resynchronize with the PRIMARY.
	switchoverReadyRecoveryTimeout = 30 * time.Minute
	// switchoverTimeout is the base timeout for gNMI to be served again by the
	// new PRIMARY controller card after a switchover.
	switchoverTimeout = 30 * time.Minute
	// roleChangeTimeout is the base timeout for a redundant-role change.
	roleChangeTimeout = 15 * time.Minute
	// ccRecoveryTimeout is the base timeout for both controller cards to
	// recover to ACTIVE with PRIMARY/SECONDARY roles.
	ccRecoveryTimeout = 20 * time.Minute
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// TestFP111PowerAdminLinecardFabric implements FP-1.1.1: Powering down
// Linecard and Fabric Card. Each card type is exercised independently and
// sequentially (table driven, not parallel).
func TestFP111PowerAdminLinecardFabric(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	cases := []struct {
		desc    string
		cType   oc.E_PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT
		timeout time.Duration
	}{
		{desc: "Linecard", cType: oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_LINECARD, timeout: linecardPowerTimeout},
		{desc: "Fabric Card", cType: oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_FABRIC, timeout: fabricPowerTimeout},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("FP-1.1.1 %s", tc.desc), func(t *testing.T) {
			testLinecardFabricPowerDownUp(t, dut, tc.desc, tc.cType, tc.timeout)
		})
	}
}

// testLinecardFabricPowerDownUp runs the FP-1.1.1 Test Setup and Test Logic for
// one card type.
func testLinecardFabricPowerDownUp(t *testing.T, dut *ondatra.DUTDevice, desc string, cType oc.E_PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT, timeout time.Duration) {
	var installed []string

	// FP-1.1.1 Test Setup.
	if !t.Run("Test Setup", func(t *testing.T) {
		// FP-1.1.1 Test Setup: Installed cards are taken to be those with
		// state/empty != true and state/removable == true (the README does not define
		// "installed").
		installed = components.FindInstalledRemovableComponents(t, dut, cType)
		if len(installed) == 0 {
			t.Fatalf("FP-1.1.1 Test Setup: no installed removable %s components found on DUT", desc)
		}
		t.Logf("FP-1.1.1 Test Setup: installed %s components: %v", desc, installed)

		// FP-1.1.1 Test Setup: Verify /components/component/{linecard|fabric}/state/oper-status
		// is ACTIVE for all installed Linecards and Fabric Cards.
		if notActive, ok := components.AwaitComponentsOperStatus(t, dut, installed, setupActiveTimeout, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE); !ok {
			t.Fatalf("FP-1.1.1 Test Setup: %s components %v are not ACTIVE: %s", desc, notActive, describeComponentsPower(t, dut, notActive, cType))
		}
		t.Logf("FP-1.1.1 Test Setup: all installed %s components %v are ACTIVE", desc, installed)
	}) {
		t.FailNow()
	}

	// FP-1.1.1 Test Logic: Select one linecard and one fabric card. The last
	// installed card of this type is selected.
	target := installed[len(installed)-1]
	t.Logf("FP-1.1.1 Test Logic: selected %s %s", desc, target)

	t.Run(fmt.Sprintf("Test Logic %s", target), func(t *testing.T) {
		// FP-1.1.1 Cleanup: Restore the original configuration of
		// /components/component/{linecard|fabric}/config/power-admin-state. This
		// cleanup is registered first, so it runs last (after the card has been
		// powered back up below).
		snapshot := snapshotPowerAdminConfig(t, dut, target, cType)
		t.Cleanup(func() {
			if err := snapshot.restore(t, dut); err != nil {
				t.Errorf("Cleanup: %v", err)
			}
		})

		// FP-1.1.1 Cleanup: Regardless of the test result, update power-admin-state
		// to POWER_ENABLED and verify oper-status is ACTIVE. Skipped if the test
		// logic already restored the card. If the test logic already waited the
		// full timeout after POWER_ENABLED without the card becoming ACTIVE, the
		// cleanup waits only stuckCardCleanupTimeout so a stuck card is reported
		// sooner.
		restored := false
		enableTimedOut := false
		t.Cleanup(func() {
			if restored {
				return
			}
			cleanupTimeout := timeout
			if enableTimedOut {
				cleanupTimeout = stuckCardCleanupTimeout
			}
			restorePowerEnabled(t, dut, target, cType, cleanupTimeout)
		})

		// Additional check (not a README step): record the DUT ports that are UP
		// before the card is powered down, so that data plane recovery can be
		// validated at the end of the test.
		upIntfs := helpers.FetchOperStatusUPIntfs(t, dut, true)

		// FP-1.1.1 Test Logic: Update /components/component/{linecard|fabric}/config/power-admin-state
		// to POWER_DISABLED for the selected card.
		components.SetPowerAdminState(t, dut, target, cType, oc.Platform_ComponentPowerType_POWER_DISABLED)

		// FP-1.1.1 Test Logic: Verify /components/component/{linecard|fabric}/state/power-admin-state
		// changes to POWER_DISABLED for the selected card.
		if got, ok := components.AwaitPowerAdminState(t, dut, target, cType, oc.Platform_ComponentPowerType_POWER_DISABLED, powerAdminStateTimeout); !ok {
			t.Errorf("FP-1.1.1: %s %s state/power-admin-state got %v, want %v", desc, target, got, oc.Platform_ComponentPowerType_POWER_DISABLED)
		} else {
			t.Logf("FP-1.1.1: %s %s state/power-admin-state is %v", desc, target, got)
		}

		// FP-1.1.1 Test Logic: Verify /components/component/{linecard|fabric}/state/oper-status
		// is DISABLED for the selected card.
		if got, ok := components.AwaitOperStatus(t, dut, target, timeout, oc.PlatformTypes_COMPONENT_OPER_STATUS_DISABLED); !ok {
			t.Errorf("FP-1.1.1: %s %s state/oper-status after POWER_DISABLED got %v, want %v", desc, target, got, oc.PlatformTypes_COMPONENT_OPER_STATUS_DISABLED)
		} else {
			t.Logf("FP-1.1.1: %s %s state/oper-status after POWER_DISABLED is %v", desc, target, got)
		}

		// FP-1.1.1 Test Logic: Update /components/component/{linecard|fabric}/config/power-admin-state
		// to POWER_ENABLED for the selected card.
		components.SetPowerAdminState(t, dut, target, cType, oc.Platform_ComponentPowerType_POWER_ENABLED)

		// Additional check (README path coverage of
		// /components/component/{linecard|fabric}/state/power-admin-state): verify
		// it changes to POWER_ENABLED for the selected card.
		if got, ok := components.AwaitPowerAdminState(t, dut, target, cType, oc.Platform_ComponentPowerType_POWER_ENABLED, powerAdminStateTimeout); !ok {
			t.Errorf("FP-1.1.1: %s %s state/power-admin-state got %v, want %v", desc, target, got, oc.Platform_ComponentPowerType_POWER_ENABLED)
		} else {
			t.Logf("FP-1.1.1: %s %s state/power-admin-state is %v", desc, target, got)
		}

		// FP-1.1.1 Test Logic: Verify /components/component/{linecard|fabric}/state/oper-status
		// returns to ACTIVE for the selected card.
		got, ok := components.AwaitOperStatus(t, dut, target, timeout, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE)
		if !ok {
			enableTimedOut = true
			t.Fatalf("FP-1.1.1: %s %s state/oper-status after POWER_ENABLED got %v, want %v (%s)", desc, target, got, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE, describeComponentsPower(t, dut, []string{target}, cType))
		}
		t.Logf("FP-1.1.1: %s %s state/oper-status after POWER_ENABLED is %v", desc, target, got)
		restored = true

		// Additional check (not a README step): verify all DUT ports that were UP
		// before the card was powered down are UP again.
		helpers.ValidateOperStatusUPIntfs(t, dut, upIntfs, intfRecoveryTimeout)
	})
}

// restorePowerEnabled sets power-admin-state of component name back to
// POWER_ENABLED and waits for oper-status ACTIVE. Failures are reported with
// t.Errorf so it is safe to use from t.Cleanup.
func restorePowerEnabled(t *testing.T, dut *ondatra.DUTDevice, name string, cType oc.E_PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT, timeout time.Duration) {
	t.Helper()
	t.Logf("Cleanup: restoring %v component %s to POWER_ENABLED", cType, name)
	if err := components.TrySetPowerAdminState(t, dut, name, cType, oc.Platform_ComponentPowerType_POWER_ENABLED); err != nil {
		t.Errorf("Cleanup: %v", err)
		return
	}
	if got, ok := components.AwaitOperStatus(t, dut, name, timeout, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE); !ok {
		t.Errorf("Cleanup: component %s oper-status got %v, want %v (%s)", name, got, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE, describeComponentsPower(t, dut, []string{name}, cType))
	}
}

// TestFP112ControllerCardPowerAdmin implements FP-1.1.2: Attempting to power
// down both controller cards.
func TestFP112ControllerCardPowerAdmin(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	const cType = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD

	// The device does not implement
	// /components/component/controller-card/config/power-admin-state (e.g. Nokia
	// rejects the path with "unknown element 'power-admin-state'. Options are
	// []"), so controller card power-admin-state cannot be configured or
	// validated at all.
	if deviations.SkipControllerCardPowerAdmin(dut) {
		t.Skip("FP-1.1.2: skip_controller_card_power_admin deviation is set; device does not support /components/component/controller-card/config/power-admin-state")
	}

	var cards []string
	var primary, secondary string

	// FP-1.1.2 Test Setup.
	if !t.Run("FP-1.1.2 Test Setup", func(t *testing.T) {
		// FP-1.1.2 Test Setup: Retrieve the list of all available controller cards
		// (components of type CONTROLLER_CARD whose slot is not empty).
		cards = components.FindInstalledComponents(t, dut, cType)

		// FP-1.1.2 Test Setup: Ensure there are at least two controller cards.
		if len(cards) < 2 {
			t.Fatalf("FP-1.1.2 Test Setup: found %d installed controller cards %v, want at least 2", len(cards), cards)
		}
		t.Logf("FP-1.1.2 Test Setup: installed controller cards: %v", cards)

		// FP-1.1.2 Test Setup: Verify /components/component/state/oper-status is
		// ACTIVE for both controller cards.
		if notActive, ok := components.AwaitComponentsOperStatus(t, dut, cards, ccActiveTimeout, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE); !ok {
			t.Fatalf("FP-1.1.2 Test Setup: controller cards %v are not ACTIVE: %s", notActive, describeComponentsPower(t, dut, notActive, cType))
		}
		t.Logf("FP-1.1.2 Test Setup: controller cards %v are ACTIVE", cards)

		// FP-1.1.2 Test Setup: Verify that /components/component/state/switchover-ready
		// is true for both the controller cards.
		if notReady, ok := components.AwaitComponentsSwitchoverReady(t, dut, cards, components.SwitchoverTimeout(dut, switchoverReadyTimeout)); !ok {
			t.Fatalf("FP-1.1.2 Test Setup: controller cards %v are not switchover-ready", notReady)
		}
		t.Logf("FP-1.1.2 Test Setup: controller cards %v are switchover-ready", cards)
	}) {
		t.FailNow()
	}

	// FP-1.1.2 Test Logic.
	t.Run("FP-1.1.2 Test Logic", func(t *testing.T) {
		// FP-1.1.2 Test Logic: Find out the PRIMARY Controller Card using
		// /components/component/state/redundant-role.
		secondary, primary = components.FindStandbyControllerCard(t, dut, cards)
		t.Logf("FP-1.1.2: PRIMARY controller card %s, SECONDARY controller card %s", primary, secondary)

		// FP-1.1.2 Cleanup: Restore the original configuration of
		// /components/component/controller-card/config/power-admin-state for both
		// controller cards. This cleanup is registered first, so it runs last
		// (after both controller cards have been powered back up below).
		snapshots := []*powerAdminConfigSnapshot{
			snapshotPowerAdminConfig(t, dut, primary, cType),
			snapshotPowerAdminConfig(t, dut, secondary, cType),
		}
		t.Cleanup(func() {
			for _, s := range snapshots {
				if err := s.restore(t, dut); err != nil {
					t.Errorf("Cleanup: %v", err)
				}
			}
		})

		// FP-1.1.2 Cleanup: Regardless of the test result, update power-admin-state
		// to POWER_ENABLED for both controller cards and verify they are ACTIVE with
		// one PRIMARY and one SECONDARY. Skipped if the test logic already restored
		// them.
		restored := false
		t.Cleanup(func() {
			if !restored {
				restoreControllerCards(t, dut, []string{primary, secondary})
			}
		})

		// FP-1.1.2 Test Logic: Using gNMI Set, attempt to update
		// /components/component/controller-card/config/power-admin-state to
		// POWER_DISABLED for the PRIMARY controller card. The Set RPC may return an
		// error if the PRIMARY tears down the gNMI session while powering off, so
		// a Set error alone is not treated as a rejection.
		setErr := components.TrySetPowerAdminState(t, dut, primary, cType, oc.Platform_ComponentPowerType_POWER_DISABLED)
		if setErr != nil {
			t.Logf("FP-1.1.2: gNMI Set of POWER_DISABLED on PRIMARY %s returned: %v", primary, setErr)
		}

		// The device rejects powering down the PRIMARY controller card by design
		// and only powers down a SECONDARY controller card. The rejection is
		// validated, the switchover is triggered via gNOI, and the former PRIMARY
		// is then powered down, so the remaining README steps run against the
		// same controller cards and roles.
		if setErr != nil && deviations.ControllerCardPrimaryPowerDisableUnsupported(dut) && !powerDisabledConfigured(t, dut, primary, cType) {
			powerDisablePrimaryViaSwitchover(t, dut, primary, secondary, cards, setErr)
			setErr = nil
		}

		// FP-1.1.2 Test Logic: Wait until the switchover happens and the SECONDARY
		// becomes PRIMARY. The DUT must serve gNMI again within the switchover window.
		components.WaitForSwitchover(t, dut, components.SwitchoverTimeout(dut, switchoverTimeout))
		if setErr != nil && !powerDisabledConfigured(t, dut, primary, cType) {
			t.Fatalf("FP-1.1.2: device rejected POWER_DISABLED on PRIMARY controller card %s, want switchover to %s: %v", primary, secondary, setErr)
		}
		if _, ok := components.AwaitRedundantRole(t, dut, secondary, oc.Platform_ComponentRedundantRole_PRIMARY, components.SwitchoverTimeout(dut, roleChangeTimeout)); !ok {
			t.Fatalf("FP-1.1.2: SECONDARY %s did not become PRIMARY after POWER_DISABLED on PRIMARY %s (gNMI Set error: %v)", secondary, primary, setErr)
		}
		newPrimary, disabledCC := secondary, primary
		t.Logf("FP-1.1.2: switchover complete, new PRIMARY is %s", newPrimary)

		// FP-1.1.2 Test Logic: Verify that /components/component/state/oper-status of
		// the disabled controller card is now DISABLED.
		if got, ok := components.AwaitOperStatusWithRetry(t, dut, disabledCC, components.SwitchoverTimeout(dut, ccActiveTimeout), oc.PlatformTypes_COMPONENT_OPER_STATUS_DISABLED); !ok {
			t.Errorf("FP-1.1.2: disabled controller card %s oper-status got %v, want %v", disabledCC, got, oc.PlatformTypes_COMPONENT_OPER_STATUS_DISABLED)
		} else {
			t.Logf("FP-1.1.2: disabled controller card %s oper-status is %v", disabledCC, got)
		}

		// Additional check (README path coverage of
		// /components/component/controller-card/state/power-admin-state): verify it
		// is POWER_DISABLED for the disabled controller card.
		if got, ok := components.AwaitPowerAdminState(t, dut, disabledCC, cType, oc.Platform_ComponentPowerType_POWER_DISABLED, powerAdminStateTimeout); !ok {
			t.Errorf("FP-1.1.2: disabled controller card %s state/power-admin-state got %v, want %v", disabledCC, got, oc.Platform_ComponentPowerType_POWER_DISABLED)
		} else {
			t.Logf("FP-1.1.2: disabled controller card %s state/power-admin-state is %v", disabledCC, got)
		}

		// FP-1.1.2 Test Logic: Using gNMI Set, attempt to update
		// /components/component/controller-card/config/power-admin-state to
		// POWER_DISABLED for the newly elected PRIMARY controller card.
		setErr = components.TrySetPowerAdminState(t, dut, newPrimary, cType, oc.Platform_ComponentPowerType_POWER_DISABLED)

		// FP-1.1.2 Test Logic: In any case the device should not become
		// unresponsive, unreachable or lose connectivity. Implemented by requiring
		// gNMI /system/state/current-datetime to be served within the switchover
		// window.
		if !components.AwaitGNMIReachable(t, dut, components.SwitchoverTimeout(dut, switchoverTimeout)) {
			t.Fatalf("FP-1.1.2: DUT is not reachable over gNMI after the POWER_DISABLED attempt on new PRIMARY %s", newPrimary)
		}
		t.Logf("FP-1.1.2: DUT is reachable over gNMI after the POWER_DISABLED attempt on new PRIMARY %s", newPrimary)

		// A Set error caused by a session teardown is not a rejection, so the
		// configured value is also checked.
		if setErr != nil && !powerDisabledConfigured(t, dut, newPrimary, cType) {
			// FP-1.1.2 Test Logic: The device should either: Reject the configuration to
			// POWER_DISABLED the controller card. To confirm the rejection, the new
			// PRIMARY must remain ACTIVE and PRIMARY.
			t.Logf("FP-1.1.2: device rejected POWER_DISABLED on new PRIMARY %s: %v", newPrimary, setErr)
			if got, ok := components.AwaitRedundantRole(t, dut, newPrimary, oc.Platform_ComponentRedundantRole_PRIMARY, components.SwitchoverTimeout(dut, roleChangeTimeout)); !ok {
				t.Errorf("FP-1.1.2: controller card %s redundant-role got %v, want %v", newPrimary, got, oc.Platform_ComponentRedundantRole_PRIMARY)
			} else {
				t.Logf("FP-1.1.2: controller card %s redundant-role is %v", newPrimary, got)
			}
			if got, ok := components.AwaitOperStatusWithRetry(t, dut, newPrimary, components.SwitchoverTimeout(dut, ccActiveTimeout), oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE); !ok {
				t.Errorf("FP-1.1.2: controller card %s oper-status got %v, want %v", newPrimary, got, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE)
			} else {
				t.Logf("FP-1.1.2: controller card %s oper-status is %v", newPrimary, got)
			}
		} else {
			// FP-1.1.2 Test Logic: OR Automatically enable the previously POWER_DISABLED
			// Controller Card and then POWER_DISABLED the requested Controller Card. To
			// confirm this, the previously disabled card must become ACTIVE and
			// PRIMARY and the requested card must become DISABLED.
			t.Logf("FP-1.1.2: device accepted POWER_DISABLED on new PRIMARY %s; expecting %s to be re-enabled and become PRIMARY", newPrimary, disabledCC)
			if got, ok := components.AwaitRedundantRole(t, dut, disabledCC, oc.Platform_ComponentRedundantRole_PRIMARY, components.SwitchoverTimeout(dut, roleChangeTimeout)); !ok {
				t.Errorf("FP-1.1.2: previously disabled controller card %s redundant-role got %v, want %v", disabledCC, got, oc.Platform_ComponentRedundantRole_PRIMARY)
			} else {
				t.Logf("FP-1.1.2: previously disabled controller card %s redundant-role is %v", disabledCC, got)
			}
			if got, ok := components.AwaitOperStatusWithRetry(t, dut, disabledCC, components.SwitchoverTimeout(dut, ccActiveTimeout), oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE); !ok {
				t.Errorf("FP-1.1.2: previously disabled controller card %s oper-status got %v, want %v", disabledCC, got, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE)
			} else {
				t.Logf("FP-1.1.2: previously disabled controller card %s oper-status is %v", disabledCC, got)
			}
			if got, ok := components.AwaitOperStatusWithRetry(t, dut, newPrimary, components.SwitchoverTimeout(dut, ccActiveTimeout), oc.PlatformTypes_COMPONENT_OPER_STATUS_DISABLED); !ok {
				t.Errorf("FP-1.1.2: requested controller card %s oper-status got %v, want %v", newPrimary, got, oc.PlatformTypes_COMPONENT_OPER_STATUS_DISABLED)
			} else {
				t.Logf("FP-1.1.2: requested controller card %s oper-status is %v", newPrimary, got)
			}
		}

		// FP-1.1.2 Test Logic: Restore
		// /components/component/controller-card/config/power-admin-state for the
		// disabled controller card(s) back to POWER_ENABLED via gNMI Set.
		restored = restoreControllerCards(t, dut, []string{disabledCC, newPrimary})
	})

	// FP-1.1.2 Verification.
	t.Run("FP-1.1.2 Verification", func(t *testing.T) {
		// FP-1.1.2 Verification: Verify the operational status and redundant roles of
		// the controller cards post-recovery: all ACTIVE, one PRIMARY, others
		// SECONDARY.
		primaryAfter, ok := awaitControllerCardsRedundant(t, dut, cards, components.SwitchoverTimeout(dut, ccRecoveryTimeout))
		if !ok {
			t.Fatalf("FP-1.1.2 Verification: controller cards %v did not recover to ACTIVE with PRIMARY/SECONDARY roles", cards)
		}
		t.Logf("FP-1.1.2 Verification: controller cards recovered, PRIMARY is %s", primaryAfter)

		// Additional check (not a README step): verify the PRIMARY reports
		// switchover-ready true, i.e. redundancy is fully restored.
		if _, ok := components.AwaitComponentsSwitchoverReady(t, dut, []string{primaryAfter}, components.SwitchoverTimeout(dut, switchoverReadyRecoveryTimeout)); !ok {
			got, present := gnmi.Lookup(t, dut, gnmi.OC().Component(primaryAfter).SwitchoverReady().State()).Val()
			t.Errorf("FP-1.1.2 Verification: PRIMARY controller card %s state/switchover-ready got %v (present=%v), want true", primaryAfter, got, present)
		} else {
			t.Logf("FP-1.1.2 Verification: PRIMARY controller card %s state/switchover-ready is true", primaryAfter)
		}
	})
}

// maxGNMIAttempts bounds the number of attempts of a gNMI operation that may
// fail transiently while a controller card switchover is in progress. Each
// failed attempt is followed by a watch-based wait until the DUT serves gNMI
// again (components.AwaitGNMIReachable).
const maxGNMIAttempts = 5

// powerDisablePrimaryViaSwitchover is used when the
// ControllerCardPrimaryPowerDisableUnsupported deviation is set and the device
// rejected POWER_DISABLED on the PRIMARY controller card with rejectErr. It
// verifies that the rejection left the DUT reachable with unchanged controller
// card roles, triggers a switchover to secondary via gNOI
// System.SwitchControlProcessor, waits until both controller cards are ACTIVE
// with secondary as PRIMARY and primary as SECONDARY, and then sets
// power-admin-state of primary (now SECONDARY) to POWER_DISABLED via gNMI Set.
// On return the DUT is in the state the README expects after powering down
// the PRIMARY: secondary is PRIMARY and primary is configured POWER_DISABLED.
func powerDisablePrimaryViaSwitchover(t *testing.T, dut *ondatra.DUTDevice, primary, secondary string, cards []string, rejectErr error) {
	t.Helper()
	const cType = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD
	t.Logf("FP-1.1.2: device rejected POWER_DISABLED on PRIMARY %s (controller_card_primary_power_disable_unsupported deviation): %v", primary, rejectErr)

	// The rejected Set must not affect the DUT: it stays reachable and the
	// PRIMARY stays ACTIVE and PRIMARY.
	if !components.AwaitGNMIReachable(t, dut, components.SwitchoverTimeout(dut, switchoverTimeout)) {
		t.Fatalf("FP-1.1.2: DUT is not reachable over gNMI after the rejected POWER_DISABLED on PRIMARY %s", primary)
	}
	if got, ok := components.AwaitRedundantRole(t, dut, primary, oc.Platform_ComponentRedundantRole_PRIMARY, components.SwitchoverTimeout(dut, roleChangeTimeout)); !ok {
		t.Fatalf("FP-1.1.2: after rejected POWER_DISABLED, controller card %s redundant-role got %v, want %v", primary, got, oc.Platform_ComponentRedundantRole_PRIMARY)
	}
	if got, ok := components.AwaitOperStatusWithRetry(t, dut, primary, components.SwitchoverTimeout(dut, ccActiveTimeout), oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE); !ok {
		t.Fatalf("FP-1.1.2: after rejected POWER_DISABLED, controller card %s oper-status got %v, want %v", primary, got, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE)
	}
	t.Logf("FP-1.1.2: after the rejected POWER_DISABLED, %s is still ACTIVE and PRIMARY and the DUT is reachable", primary)

	// Trigger the switchover to the SECONDARY via gNOI.
	req := &spb.SwitchControlProcessorRequest{
		ControlProcessor: components.GetSubcomponentPath(secondary, deviations.GNOISubcomponentPath(dut)),
	}
	t.Logf("FP-1.1.2: triggering switchover to %s via gNOI SwitchControlProcessor: %v", secondary, req)
	resp, err := dut.RawAPIs().GNOI(t).System().SwitchControlProcessor(context.Background(), req)
	if err != nil {
		t.Fatalf("FP-1.1.2: gNOI SwitchControlProcessor to %s failed: %v", secondary, err)
	}
	t.Logf("FP-1.1.2: gNOI SwitchControlProcessor response: %v", resp)

	// The gNOI RPC may return before the switchover starts, so wait until the
	// SECONDARY actually reports redundant-role PRIMARY rather than checking
	// the controller cards immediately, which would observe the roles from
	// before the switchover.
	t.Logf("FP-1.1.2: waiting for %s to become PRIMARY after gNOI switchover", secondary)
	if got, ok := components.AwaitRedundantRole(t, dut, secondary, oc.Platform_ComponentRedundantRole_PRIMARY, components.SwitchoverTimeout(dut, roleChangeTimeout)); !ok {
		t.Fatalf("FP-1.1.2: after gNOI switchover, controller card %s redundant-role got %v, want %v", secondary, got, oc.Platform_ComponentRedundantRole_PRIMARY)
	}
	t.Logf("FP-1.1.2: %s is PRIMARY after gNOI switchover", secondary)

	// Wait until the former PRIMARY has rebooted as SECONDARY and both
	// controller cards are ACTIVE, since the device only accepts
	// POWER_DISABLED on the SECONDARY.
	newPrimary, ok := awaitControllerCardsRedundant(t, dut, cards, components.SwitchoverTimeout(dut, ccRecoveryTimeout))
	if !ok {
		t.Fatalf("FP-1.1.2: controller cards %v did not recover to ACTIVE with PRIMARY/SECONDARY roles after gNOI switchover to %s", cards, secondary)
	}
	if newPrimary != secondary {
		t.Fatalf("FP-1.1.2: PRIMARY after gNOI switchover got %s, want %s", newPrimary, secondary)
	}
	t.Logf("FP-1.1.2: gNOI switchover complete, %s is PRIMARY and %s is SECONDARY", secondary, primary)

	// Set power-admin-state of the former PRIMARY (now SECONDARY) to
	// POWER_DISABLED, retrying transient gNMI errors.
	for attempt := 1; attempt <= maxGNMIAttempts; attempt++ {
		if err = components.TrySetPowerAdminState(t, dut, primary, cType, oc.Platform_ComponentPowerType_POWER_DISABLED); err == nil {
			return
		}
		t.Logf("Attempt %d: setting %s to POWER_DISABLED failed: %v", attempt, primary, err)
		if !components.AwaitGNMIReachable(t, dut, components.SwitchoverTimeout(dut, switchoverTimeout)) {
			t.Logf("DUT not reachable over gNMI while setting %s to POWER_DISABLED", primary)
		}
	}
	t.Fatalf("FP-1.1.2: setting former PRIMARY %s (now SECONDARY) to POWER_DISABLED failed after %d attempts: %v", primary, maxGNMIAttempts, err)
}

// powerDisabledConfigured reports whether
// /components/component/controller-card/config/power-admin-state of component
// name is POWER_DISABLED in the device configuration. It distinguishes a
// rejected gNMI Set from an accepted Set whose RPC failed due to a gNMI session
// teardown. Transient gNMI errors are retried once the DUT is reachable again.
func powerDisabledConfigured(t *testing.T, dut *ondatra.DUTDevice, name string, cType oc.E_PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT) bool {
	t.Helper()
	config, _, err := components.PowerAdminStateQueries(name, cType)
	if err != nil {
		t.Fatalf("powerDisabledConfigured: %v", err)
	}
	var lastErr string
	for attempt := 1; attempt <= maxGNMIAttempts; attempt++ {
		var got oc.E_Platform_ComponentPowerType
		var present bool
		msg := testt.CaptureFatal(t, func(t testing.TB) {
			got, present = gnmi.LookupConfig(t, dut, config).Val()
		})
		if msg == nil {
			return present && got == oc.Platform_ComponentPowerType_POWER_DISABLED
		}
		lastErr = *msg
		t.Logf("Attempt %d: reading %s config/power-admin-state failed: %s", attempt, name, lastErr)
		if !components.AwaitGNMIReachable(t, dut, components.SwitchoverTimeout(dut, switchoverTimeout)) {
			t.Logf("DUT not reachable over gNMI while reading %s config/power-admin-state", name)
		}
	}
	t.Fatalf("Reading %s config/power-admin-state failed after %d attempts: %s", name, maxGNMIAttempts, lastErr)
	return false
}

// restoreControllerCards sets power-admin-state of every controller card in
// cards back to POWER_ENABLED, retrying transient gNMI errors once the DUT is
// reachable again, and then waits for all controller cards to recover to
// ACTIVE with PRIMARY/SECONDARY roles and to report state/power-admin-state
// POWER_ENABLED. The state leaf is verified here, while config/power-admin-state
// is still POWER_ENABLED, because after the original configuration is restored
// (e.g. the leaf is deleted) some devices no longer report the default value.
// Failures are reported with t.Errorf so it is safe to use from t.Cleanup. It
// returns whether every Set succeeded.
func restoreControllerCards(t *testing.T, dut *ondatra.DUTDevice, cards []string) bool {
	t.Helper()
	const cType = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD
	allSet := true
	for _, c := range cards {
		var err error
		for attempt := 1; attempt <= maxGNMIAttempts; attempt++ {
			if err = components.TrySetPowerAdminState(t, dut, c, cType, oc.Platform_ComponentPowerType_POWER_ENABLED); err == nil {
				break
			}
			t.Logf("Attempt %d: restoring %s to POWER_ENABLED failed: %v", attempt, c, err)
			if !components.AwaitGNMIReachable(t, dut, components.SwitchoverTimeout(dut, switchoverTimeout)) {
				t.Logf("DUT not reachable over gNMI while restoring %s", c)
			}
		}
		if err != nil {
			t.Errorf("Restoring controller card %s to POWER_ENABLED failed after %d attempts: %v", c, maxGNMIAttempts, err)
			allSet = false
		}
	}
	if _, ok := awaitControllerCardsRedundant(t, dut, cards, components.SwitchoverTimeout(dut, ccRecoveryTimeout)); !ok {
		t.Errorf("Controller cards %v did not recover to ACTIVE with PRIMARY/SECONDARY roles after restoring POWER_ENABLED", cards)
	}
	// Additional check (README path coverage of
	// /components/component/controller-card/state/power-admin-state): verify it
	// is POWER_ENABLED for all controller cards.
	if notEnabled, ok := components.AwaitComponentsPowerAdminState(t, dut, cards, cType, oc.Platform_ComponentPowerType_POWER_ENABLED, powerAdminStateTimeout); !ok {
		t.Errorf("Controller cards %v state/power-admin-state is not %v after restoring POWER_ENABLED: %s", notEnabled, oc.Platform_ComponentPowerType_POWER_ENABLED, describeComponentsPower(t, dut, notEnabled, cType))
	}
	return allSet
}

// redundantRoleRetryInterval is the interval at which redundant-role watches
// are re-established after a gNMI disconnect during a switchover.
const redundantRoleRetryInterval = 30 * time.Second

// minPerCardWait is the minimum time awaitRedundantCandidates waits for each
// controller card, so that every card is checked at least once even if earlier
// cards used up the shared timeout.
const minPerCardWait = 30 * time.Second

// waitAtLeast returns the time left until deadline, but at least
// minPerCardWait.
func waitAtLeast(deadline time.Time) time.Duration {
	return max(time.Until(deadline), minPerCardWait)
}

// awaitRedundantRoleIn waits up to timeout, using a single-leaf watch that
// tolerates gNMI disconnects, until the state/redundant-role of controller card
// name is one of want. It returns the observed role (UNSET if none of want was
// observed) and whether one of want was observed.
func awaitRedundantRoleIn(t *testing.T, dut *ondatra.DUTDevice, name string, timeout time.Duration, want ...oc.E_Platform_ComponentRedundantRole) (oc.E_Platform_ComponentRedundantRole, bool) {
	t.Helper()
	val, ok := components.WatchWithRetry(t, dut, gnmi.OC().Component(name).RedundantRole().State(), timeout, redundantRoleRetryInterval, func(v *ygnmi.Value[oc.E_Platform_ComponentRedundantRole]) bool {
		role, present := v.Val()
		return present && slices.Contains(want, role)
	})
	if !ok || val == nil {
		return oc.Platform_ComponentRedundantRole_UNSET, false
	}
	role, _ := val.Val()
	return role, true
}

// maxRedundancyChecks bounds the number of times awaitControllerCardsRedundant
// re-checks the controller cards. A re-check is only needed if a controller
// card changed state between its watch and the confirmation read (e.g. during a
// switchover), and the next round of watches blocks until the state is
// consistent again, so this bound only stops a device that reports
// inconsistent values from being re-checked in a tight loop.
const maxRedundancyChecks = 10

// awaitControllerCardsRedundant waits up to timeout until every controller card
// in cards reports oper-status ACTIVE, exactly one of them reports
// redundant-role PRIMARY and all others report redundant-role SECONDARY. Each
// controller card is waited for with single-leaf watches that tolerate gNMI
// disconnects caused by a controller card switchover, and the combined state is
// then confirmed with one read per leaf. If the redundant state is not reached,
// the last observed oper-status and redundant-role of every controller card
// are logged. It returns the PRIMARY controller card and whether the redundant
// state was reached.
func awaitControllerCardsRedundant(t *testing.T, dut *ondatra.DUTDevice, cards []string, timeout time.Duration) (string, bool) {
	t.Helper()
	if len(cards) < 2 {
		t.Fatalf("awaitControllerCardsRedundant requires at least 2 controller cards, got %v", cards)
	}
	deadline := time.Now().Add(timeout)
	for check := 1; check <= maxRedundancyChecks && time.Now().Before(deadline); check++ {
		if !awaitRedundantCandidates(t, dut, cards, deadline) {
			break
		}
		states := lookupControllerCards(t, dut, cards)
		if primary, ok := redundantPrimary(cards, states); ok {
			return primary, true
		}
		t.Logf("Check %d: controller cards changed state while being checked: %s", check, describeControllerCards(cards, states))
	}
	t.Logf("Controller cards not redundant after %v; last observed: %s", timeout, describeControllerCards(cards, lookupControllerCards(t, dut, cards)))
	return "", false
}

// awaitRedundantCandidates waits, with single-leaf watches, until every
// controller card in cards is ACTIVE, the first controller card reports
// redundant-role PRIMARY or SECONDARY, and every other controller card reports
// a redundant-role compatible with it (see peerRoles). It returns false if a
// watch did not succeed before deadline.
func awaitRedundantCandidates(t *testing.T, dut *ondatra.DUTDevice, cards []string, deadline time.Time) bool {
	t.Helper()
	if _, ok := components.AwaitComponentsOperStatus(t, dut, cards, time.Until(deadline), oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE); !ok {
		return false
	}
	first, ok := awaitRedundantRoleIn(t, dut, cards[0], waitAtLeast(deadline), oc.Platform_ComponentRedundantRole_PRIMARY, oc.Platform_ComponentRedundantRole_SECONDARY)
	if !ok {
		return false
	}
	for _, c := range cards[1:] {
		if _, ok := awaitRedundantRoleIn(t, dut, c, waitAtLeast(deadline), peerRoles(first, len(cards))...); !ok {
			return false
		}
	}
	return true
}

// peerRoles returns the redundant-roles that the other numCards-1 controller
// cards may report when the first controller card reports first, such that
// exactly one controller card is PRIMARY.
func peerRoles(first oc.E_Platform_ComponentRedundantRole, numCards int) []oc.E_Platform_ComponentRedundantRole {
	switch {
	case first == oc.Platform_ComponentRedundantRole_PRIMARY:
		return []oc.E_Platform_ComponentRedundantRole{oc.Platform_ComponentRedundantRole_SECONDARY}
	case numCards == 2:
		return []oc.E_Platform_ComponentRedundantRole{oc.Platform_ComponentRedundantRole_PRIMARY}
	default:
		return []oc.E_Platform_ComponentRedundantRole{oc.Platform_ComponentRedundantRole_PRIMARY, oc.Platform_ComponentRedundantRole_SECONDARY}
	}
}

// controllerCardState is the oper-status and redundant-role reported by a
// controller card.
type controllerCardState struct {
	oper oc.E_PlatformTypes_COMPONENT_OPER_STATUS
	role oc.E_Platform_ComponentRedundantRole
}

// lookupControllerCards reads the oper-status and redundant-role of every
// controller card in cards. Leaves that cannot be read are returned as UNSET.
func lookupControllerCards(t *testing.T, dut *ondatra.DUTDevice, cards []string) []controllerCardState {
	t.Helper()
	states := make([]controllerCardState, len(cards))
	for i, c := range cards {
		testt.CaptureFatal(t, func(t testing.TB) {
			states[i].oper, _ = gnmi.Lookup(t, dut, gnmi.OC().Component(c).OperStatus().State()).Val()
			states[i].role, _ = gnmi.Lookup(t, dut, gnmi.OC().Component(c).RedundantRole().State()).Val()
		})
	}
	return states
}

// redundantPrimary returns the PRIMARY controller card and true if every
// controller card in states is ACTIVE, exactly one is PRIMARY and all others
// are SECONDARY.
func redundantPrimary(cards []string, states []controllerCardState) (string, bool) {
	primary := ""
	for i, s := range states {
		if s.oper != oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE {
			return "", false
		}
		switch s.role {
		case oc.Platform_ComponentRedundantRole_PRIMARY:
			if primary != "" {
				return "", false
			}
			primary = cards[i]
		case oc.Platform_ComponentRedundantRole_SECONDARY:
		default:
			return "", false
		}
	}
	return primary, primary != ""
}

// describeControllerCards formats the oper-status and redundant-role of every
// controller card for logging.
func describeControllerCards(cards []string, states []controllerCardState) string {
	parts := make([]string, len(cards))
	for i, c := range cards {
		parts[i] = fmt.Sprintf("%s: oper-status=%v, redundant-role=%v", c, states[i].oper, states[i].role)
	}
	return strings.Join(parts, "; ")
}

// powerAdminConfigSnapshot records the configuration of the power-admin-state
// leaf of a component, and whether the component config node existed, so that
// the exact original configuration can be restored after a test.
type powerAdminConfigSnapshot struct {
	name                   string
	cType                  oc.E_PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT
	leafPresent            bool
	leafValue              oc.E_Platform_ComponentPowerType
	componentConfigPresent bool
}

// snapshotPowerAdminConfig records the current configuration of
// config/power-admin-state of component name, which is of hardware type cType,
// and whether /components/component[name]/config exists.
func snapshotPowerAdminConfig(t *testing.T, dut *ondatra.DUTDevice, name string, cType oc.E_PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT) *powerAdminConfigSnapshot {
	t.Helper()
	config, _, err := components.PowerAdminStateQueries(name, cType)
	if err != nil {
		t.Fatalf("snapshotPowerAdminConfig: %v", err)
	}
	s := &powerAdminConfigSnapshot{name: name, cType: cType}
	s.leafValue, s.leafPresent = gnmi.LookupConfig(t, dut, config).Val()
	_, s.componentConfigPresent = gnmi.LookupConfig(t, dut, gnmi.OC().Component(name).Name().Config()).Val()
	t.Logf("Snapshot of %s power-admin-state config: leaf present=%v value=%v, component config present=%v", name, s.leafPresent, s.leafValue, s.componentConfigPresent)
	return s
}

// restore reverts the configuration recorded by snapshotPowerAdminConfig:
//   - if the component config node did not exist, it is deleted (this also
//     removes a node created for the leafref deviations);
//   - otherwise, if the power-admin-state leaf existed, its original value is
//     replaced;
//   - otherwise, the power-admin-state leaf is deleted.
//
// Errors are returned rather than failing the test so that restore is safe to
// use from t.Cleanup.
func (s *powerAdminConfigSnapshot) restore(t *testing.T, dut *ondatra.DUTDevice) error {
	t.Helper()
	config, _, err := components.PowerAdminStateQueries(s.name, s.cType)
	if err != nil {
		return err
	}
	var msg *string
	switch {
	case !s.componentConfigPresent:
		t.Logf("Restoring %s: deleting component config created by the test", s.name)
		msg = testt.CaptureFatal(t, func(t testing.TB) {
			gnmi.Delete(t, dut, gnmi.OC().Component(s.name).Config())
		})
	case s.leafPresent:
		t.Logf("Restoring %s: config/power-admin-state to original value %v", s.name, s.leafValue)
		msg = testt.CaptureFatal(t, func(t testing.TB) {
			gnmi.Replace(t, dut, config, s.leafValue)
		})
	default:
		t.Logf("Restoring %s: deleting config/power-admin-state set by the test", s.name)
		msg = testt.CaptureFatal(t, func(t testing.TB) {
			gnmi.Delete(t, dut, config)
		})
	}
	if msg != nil {
		return fmt.Errorf("restoring original power-admin-state config of %s: %s", s.name, *msg)
	}
	return nil
}

// describeComponentsPower returns a human-readable summary of the
// state/oper-status, state/power-admin-state and config/power-admin-state of
// every component in names, which are of hardware type cType, for use in
// failure messages. Values that cannot be read are reported as "unavailable"
// and absent values as "not reported"; it never fails the test.
//
// Each leaf is looked up individually (rather than in one batch) so that an
// unsupported path does not hide the other values. It is intended for failure
// paths only, where the number of components is small.
func describeComponentsPower(t *testing.T, dut *ondatra.DUTDevice, names []string, cType oc.E_PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT) string {
	t.Helper()
	var parts []string
	for _, name := range names {
		oper, state, config := "unavailable", "unavailable", "unavailable"
		testt.CaptureFatal(t, func(t testing.TB) {
			oper = "not reported"
			if v, ok := gnmi.Lookup(t, dut, gnmi.OC().Component(name).OperStatus().State()).Val(); ok {
				oper = v.String()
			}
		})
		if cq, sq, err := components.PowerAdminStateQueries(name, cType); err == nil {
			testt.CaptureFatal(t, func(t testing.TB) {
				state = "not reported"
				if v, ok := gnmi.Lookup(t, dut, sq).Val(); ok {
					state = v.String()
				}
			})
			testt.CaptureFatal(t, func(t testing.TB) {
				config = "not configured"
				if v, ok := gnmi.LookupConfig(t, dut, cq).Val(); ok {
					config = v.String()
				}
			})
		}
		parts = append(parts, fmt.Sprintf("%s: oper-status=%s, state/power-admin-state=%s, config/power-admin-state=%s", name, oper, state, config))
	}
	return strings.Join(parts, "; ")
}
