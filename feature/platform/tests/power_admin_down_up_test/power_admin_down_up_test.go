// Package power_admin_down_up_test tests the power-admin-state leaf configuration
// on fabrics, controllers and linecards.
package power_admin_down_up_test

import (
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestFabricPowerAdmin(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	runPowerAdminTest(t, dut, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_FABRIC, 15*time.Minute)
}

func TestLinecardPowerAdmin(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	runPowerAdminTest(t, dut, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_LINECARD, 20*time.Minute)
}

func componentPowerConfigAndState(name string, cType oc.E_PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT) (ygnmi.ConfigQuery[oc.E_Platform_ComponentPowerType], ygnmi.SingletonQuery[oc.E_Platform_ComponentPowerType], bool) {
	c := gnmi.OC().Component(name)
	switch cType {
	case oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD:
		return c.ControllerCard().PowerAdminState().Config(), c.ControllerCard().PowerAdminState().State(), true
	case oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_LINECARD:
		return c.Linecard().PowerAdminState().Config(), c.Linecard().PowerAdminState().State(), true
	case oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_FABRIC:
		return c.Fabric().PowerAdminState().Config(), c.Fabric().PowerAdminState().State(), true
	default:
		return nil, nil, false
	}
}

func setComponentPowerAdmin(t *testing.T, dut *ondatra.DUTDevice, name string, cType oc.E_PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT, power oc.E_Platform_ComponentPowerType) *string {
	t.Helper()
	config, _, ok := componentPowerConfigAndState(name, cType)
	if !ok {
		t.Fatalf("Unknown component type: %s", cType.String())
	}
	if deviations.PowerDisableEnableLeafRefValidation(dut) {
		testt.CaptureFatal(t, func(t testing.TB) {
			gnmi.Update(t, dut, gnmi.OC().Component(name).Config(), &oc.Component{
				Name: ygot.String(name),
			})
		})
	}
	return testt.CaptureFatal(t, func(t testing.TB) {
		gnmi.Replace(t, dut, config, power)
	})
}

func runPowerAdminTest(t *testing.T, dut *ondatra.DUTDevice, cType oc.E_PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT, timeout time.Duration) {
	t.Helper()
	cs := components.FindComponentsByType(t, dut, cType)
	if len(cs) == 0 {
		t.Skipf("No components found for type %v", cType)
	}

	// Test Setup: Verify state/oper-status is ACTIVE for installed cards of cType,
	// and heal any removable card left powered down from a prior interrupted run.
	batch := gnmi.OCBatch()
	for _, name := range cs {
		batch.AddPaths(
			gnmi.OC().Component(name).Empty(),
			gnmi.OC().Component(name).Removable(),
			gnmi.OC().Component(name).OperStatus(),
		)
	}
	results := gnmi.Get(t, dut, batch.State())

	var inactiveRemovable []string
	var activeRemovable []string
	for _, name := range cs {
		comp := results.GetComponent(name)
		if comp == nil || comp.GetEmpty() || !comp.GetRemovable() {
			continue
		}
		if comp.GetOperStatus() == oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE {
			activeRemovable = append(activeRemovable, name)
		} else {
			t.Logf("Component %s initially not ACTIVE (got %v); restoring POWER_ENABLED", name, comp.GetOperStatus())
			setComponentPowerAdmin(t, dut, name, cType, oc.Platform_ComponentPowerType_POWER_ENABLED)
			inactiveRemovable = append(inactiveRemovable, name)
		}
	}

	// If no removable component was ACTIVE initially, wait for one of the healed components to become ACTIVE.
	if len(activeRemovable) == 0 && len(inactiveRemovable) > 0 {
		t.Logf("Waiting up to 10 minutes for restored %v component(s) %v to reach ACTIVE...", cType, inactiveRemovable)
		deadline := time.Now().Add(10 * time.Minute)
		for time.Now().Before(deadline) && len(activeRemovable) == 0 {
			time.Sleep(10 * time.Second)
			for _, name := range inactiveRemovable {
				var oper oc.E_PlatformTypes_COMPONENT_OPER_STATUS
				if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
					oper = gnmi.Get(t, dut, gnmi.OC().Component(name).OperStatus().State())
				}); errMsg == nil && oper == oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE {
					activeRemovable = append(activeRemovable, name)
				}
			}
		}
	}

	if len(activeRemovable) == 0 {
		t.Skipf("No active removable components found for type %v", cType)
	}

	// Per FP-1.1.1 README ("Select one Linecard and one Fabric Card"), pick one healthy active removable component.
	// Pick the last active removable component so we prefer a fresh slot untouched by any earlier interrupted run.
	targetName := activeRemovable[len(activeRemovable)-1]
	t.Logf("Selected %v component %s (from %d active removable components: %v)", cType, targetName, len(activeRemovable), activeRemovable)

	t.Run(targetName, func(t *testing.T) {
		before := helpers.FetchOperStatusUPIntfs(t, dut, true)
		powerDownUp(t, dut, targetName, cType, timeout)

		// Verify the component is ACTIVE before validating interfaces.
		gnmi.Await(t, dut, gnmi.OC().Component(targetName).OperStatus().State(), 10*time.Minute, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE)

		// Validate interfaces with a 20-minute timeout for fabric/linecard convergence.
		helpers.ValidateOperStatusUPIntfs(t, dut, before, 20*time.Minute)
	})
}

func TestControllerCardPowerAdmin(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	if deviations.SkipControllerCardPowerAdmin(dut) {
		t.Skipf("Power-admin-state config on controller card is not supported.")
	}

	rawCS := components.FindComponentsByType(t, dut, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD)
	var cs []string
	for _, c := range rawCS {
		var empty bool
		testt.CaptureFatal(t, func(t testing.TB) {
			if val, ok := gnmi.Lookup(t, dut, gnmi.OC().Component(c).Empty().State()).Val(); ok {
				empty = val
			}
		})
		if !empty {
			cs = append(cs, c)
		}
	}
	if len(cs) < 2 {
		t.Skipf("Number of installed controller cards (%v) is less than 2. Skipping test for controller-card power-admin-state.", cs)
	}

	// Ensure both controller cards are powered on and ACTIVE before starting setup.
	for _, c := range cs {
		var oper oc.E_PlatformTypes_COMPONENT_OPER_STATUS
		testt.CaptureFatal(t, func(t testing.TB) {
			if val, ok := gnmi.Lookup(t, dut, gnmi.OC().Component(c).OperStatus().State()).Val(); ok {
				oper = val
			}
		})
		if oper != oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE {
			t.Logf("Controller card %s is initially %v; restoring POWER_ENABLED before setup", c, oper)
			setComponentPowerAdmin(t, dut, c, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD, oc.Platform_ComponentPowerType_POWER_ENABLED)
		}
	}

	// 1. Test Setup:
	// Wait for both controller cards to be ACTIVE and redundant roles to be settled.
	setupDeadline := time.Now().Add(15 * time.Minute)
	for time.Now().Before(setupDeadline) {
		allActive := true
		for _, c := range cs {
			var oper oc.E_PlatformTypes_COMPONENT_OPER_STATUS
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				oper = gnmi.Get(t, dut, gnmi.OC().Component(c).OperStatus().State())
			}); errMsg != nil || oper != oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE {
				allActive = false
				break
			}
		}
		if allActive {
			break
		}
		time.Sleep(10 * time.Second)
	}

	// Find out the PRIMARY and SECONDARY Controller Cards using /components/component/state/redundant-role.
	standbyCC, activeCC := components.FindStandbyControllerCard(t, dut, cs)
	t.Logf("Detected Active ControllerCard (PRIMARY): %s, Standby ControllerCard (SECONDARY): %s", activeCC, standbyCC)

	// Wait up to 2 minutes for activeCC to report SwitchoverReady == true.
	readyDeadline := time.Now().Add(2 * time.Minute)
	for time.Now().Before(readyDeadline) {
		var ready bool
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			if val, ok := gnmi.Lookup(t, dut, gnmi.OC().Component(activeCC).SwitchoverReady().State()).Val(); ok {
				ready = val
			}
		}); errMsg == nil && ready {
			t.Logf("Active controller card %s SwitchoverReady: true", activeCC)
			break
		}
		time.Sleep(5 * time.Second)
	}

	// Register a safety net configuration cleanup to restore primary & standby controllers to POWER_ENABLED.
	// Use testt.CaptureFatal so cleanup does not fail on platforms that disallow setting power-admin-state on the active controller.
	t.Cleanup(func() {
		t.Logf("Cleaning up: Restoring controller cards back to POWER_ENABLED...")
		setComponentPowerAdmin(t, dut, activeCC, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD, oc.Platform_ComponentPowerType_POWER_ENABLED)
		setComponentPowerAdmin(t, dut, standbyCC, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD, oc.Platform_ComponentPowerType_POWER_ENABLED)
	})

	// 2. Test Logic:
	// Attempt to update /components/component/controller-card/config/power-admin-state to POWER_DISABLED for the PRIMARY controller card.
	t.Logf("Updating config to POWER_DISABLED for primary controller card: %s", activeCC)
	setErr := setComponentPowerAdmin(t, dut, activeCC, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD, oc.Platform_ComponentPowerType_POWER_DISABLED)
	if setErr != nil {
		t.Logf("Device rejected POWER_DISABLED on primary controller card (Hardware Protection): %s", *setErr)
		return // Successful early exit, as the platform blocks this operation at step 1 instead of step 2.
	}

	// Wait up to 4 minutes to see if setting POWER_DISABLED on activeCC triggers a switchover or state change.
	t.Logf("Checking if setting POWER_DISABLED on %s triggers a controller switchover...", activeCC)
	switchoverTriggered := false
	triggerDeadline := time.Now().Add(4 * time.Minute)
	for time.Now().Before(triggerDeadline) {
		time.Sleep(10 * time.Second)
		var activeOper oc.E_PlatformTypes_COMPONENT_OPER_STATUS
		var activeRole, standbyRole oc.E_Platform_ComponentRedundantRole
		errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			activeOper = gnmi.Get(t, dut, gnmi.OC().Component(activeCC).OperStatus().State())
			activeRole = gnmi.Get(t, dut, gnmi.OC().Component(activeCC).RedundantRole().State())
			standbyRole = gnmi.Get(t, dut, gnmi.OC().Component(standbyCC).RedundantRole().State())
		})
		if errMsg != nil {
			t.Logf("gNMI unreachable during controller card power-down (%s); switchover in progress", *errMsg)
			switchoverTriggered = true
			break
		}
		if activeOper != oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE ||
			activeRole != oc.Platform_ComponentRedundantRole_PRIMARY ||
			standbyRole == oc.Platform_ComponentRedundantRole_PRIMARY {
			t.Logf("Switchover observed: %s oper=%v role=%v, %s role=%v", activeCC, activeOper, activeRole, standbyCC, standbyRole)
			switchoverTriggered = true
			break
		}
	}

	if !switchoverTriggered {
		t.Logf("Primary controller card %s remained ACTIVE and PRIMARY for 4 minutes after POWER_DISABLED (Hardware Protection prevented powering down active controller card). Restoring POWER_ENABLED.", activeCC)
		setComponentPowerAdmin(t, dut, activeCC, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD, oc.Platform_ComponentPowerType_POWER_ENABLED)
		return
	}

	// Wait until the switchover completes and system telemetry is reachable again.
	t.Logf("Wait for switchover to complete (max 30 minutes)...")
	components.WaitForSwitchover(t, dut, 30*time.Minute)

	// Poll up to 15 minutes for standbyCC to report PRIMARY and activeCC to report DISABLED/INACTIVE.
	roleDeadline := time.Now().Add(15 * time.Minute)
	for time.Now().Before(roleDeadline) {
		var standbyRole oc.E_Platform_ComponentRedundantRole
		var activeOper oc.E_PlatformTypes_COMPONENT_OPER_STATUS
		errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			standbyRole = gnmi.Get(t, dut, gnmi.OC().Component(standbyCC).RedundantRole().State())
			if val, ok := gnmi.Lookup(t, dut, gnmi.OC().Component(activeCC).OperStatus().State()).Val(); ok {
				activeOper = val
			}
		})
		if errMsg == nil && standbyRole == oc.Platform_ComponentRedundantRole_PRIMARY &&
			(activeOper == oc.PlatformTypes_COMPONENT_OPER_STATUS_DISABLED || activeOper == oc.PlatformTypes_COMPONENT_OPER_STATUS_INACTIVE) {
			t.Logf("Post-switchover state verified: %s role=%v, %s oper-status=%v", standbyCC, standbyRole, activeCC, activeOper)
			break
		}
		time.Sleep(10 * time.Second)
	}

	// Attempt to update power-admin-state to POWER_DISABLED for the newly elected PRIMARY controller card.
	t.Logf("Attempting to update newly elected PRIMARY controller card %s config to POWER_DISABLED", standbyCC)
	setErr = setComponentPowerAdmin(t, dut, standbyCC, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD, oc.Platform_ComponentPowerType_POWER_DISABLED)
	if setErr != nil {
		t.Logf("Set on newly elected PRIMARY controller card %s was rejected as expected: %s", standbyCC, *setErr)
	} else {
		t.Logf("Set on newly elected PRIMARY controller card %s was accepted; waiting 1 minute before restoring both controller cards to POWER_ENABLED", standbyCC)
		time.Sleep(1 * time.Minute)
		components.WaitForSwitchover(t, dut, 30*time.Minute)
	}

	// Restore both controller cards back to POWER_ENABLED.
	t.Logf("Restoring %s and %s config to POWER_ENABLED", activeCC, standbyCC)
	setComponentPowerAdmin(t, dut, activeCC, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD, oc.Platform_ComponentPowerType_POWER_ENABLED)
	setComponentPowerAdmin(t, dut, standbyCC, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD, oc.Platform_ComponentPowerType_POWER_ENABLED)

	// 3. Verification:
	// Poll up to 20 minutes for both controller cards to recover to OperStatus == ACTIVE and valid PRIMARY/SECONDARY roles.
	verifyDeadline := time.Now().Add(20 * time.Minute)
	var recovered bool
	for time.Now().Before(verifyDeadline) {
		var oper0, oper1 oc.E_PlatformTypes_COMPONENT_OPER_STATUS
		var role0, role1 oc.E_Platform_ComponentRedundantRole
		errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			oper0 = gnmi.Get(t, dut, gnmi.OC().Component(cs[0]).OperStatus().State())
			oper1 = gnmi.Get(t, dut, gnmi.OC().Component(cs[1]).OperStatus().State())
			role0 = gnmi.Get(t, dut, gnmi.OC().Component(cs[0]).RedundantRole().State())
			role1 = gnmi.Get(t, dut, gnmi.OC().Component(cs[1]).RedundantRole().State())
		})
		rolesValid := (role0 == oc.Platform_ComponentRedundantRole_PRIMARY && role1 == oc.Platform_ComponentRedundantRole_SECONDARY) ||
			(role0 == oc.Platform_ComponentRedundantRole_SECONDARY && role1 == oc.Platform_ComponentRedundantRole_PRIMARY)
		if errMsg == nil &&
			oper0 == oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE &&
			oper1 == oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE &&
			rolesValid {
			t.Logf("Operational status and redundant roles of controller cards have recovered successfully. %s: oper=%v role=%v, %s: oper=%v role=%v",
				cs[0], oper0, role0, cs[1], oper1, role1)
			recovered = true
			break
		}
		// Periodically re-apply POWER_ENABLED in case a controller card was still rebooting when the first Replace was sent.
		setComponentPowerAdmin(t, dut, activeCC, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD, oc.Platform_ComponentPowerType_POWER_ENABLED)
		setComponentPowerAdmin(t, dut, standbyCC, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD, oc.Platform_ComponentPowerType_POWER_ENABLED)
		time.Sleep(15 * time.Second)
	}
	if !recovered {
		t.Errorf("Controller cards %v did not recover to ACTIVE with PRIMARY/SECONDARY roles within 20 minutes", cs)
	}
}

func powerDownUp(t *testing.T, dut *ondatra.DUTDevice, name string, cType oc.E_PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT, timeout time.Duration) {
	c := gnmi.OC().Component(name)
	config, state, ok := componentPowerConfigAndState(name, cType)
	if !ok {
		t.Fatalf("Unknown component type: %s", cType.String())
	}

	// Ensure component is always restored to POWER_ENABLED on exit.
	t.Cleanup(func() {
		setComponentPowerAdmin(t, dut, name, cType, oc.Platform_ComponentPowerType_POWER_ENABLED)
	})

	if deviations.PowerDisableEnableLeafRefValidation(dut) {
		gnmi.Update(t, dut, c.Config(), &oc.Component{
			Name: ygot.String(name),
		})
	}
	start := time.Now()
	t.Logf("Starting %s POWER_DISABLE", name)
	gnmi.Replace(t, dut, config, oc.Platform_ComponentPowerType_POWER_DISABLED)

	// Allow hardware to initiate power-down before polling telemetry.
	time.Sleep(30 * time.Second)

	// Poll up to timeout for component oper-status to become DISABLED or INACTIVE.
	downDeadline := time.Now().Add(timeout)
	var downOper oc.E_PlatformTypes_COMPONENT_OPER_STATUS
	var downPower oc.E_Platform_ComponentPowerType
	var isDown bool
	for time.Now().Before(downDeadline) {
		testt.CaptureFatal(t, func(t testing.TB) {
			if val, ok := gnmi.Lookup(t, dut, state).Val(); ok {
				downPower = val
			}
			if val, ok := gnmi.Lookup(t, dut, c.OperStatus().State()).Val(); ok {
				downOper = val
			}
		})
		if downOper == oc.PlatformTypes_COMPONENT_OPER_STATUS_DISABLED ||
			downOper == oc.PlatformTypes_COMPONENT_OPER_STATUS_INACTIVE {
			isDown = true
			break
		}
		time.Sleep(5 * time.Second)
	}
	if !isDown {
		t.Errorf("Component %s did not power down within %v; last oper-status=%v, power-admin-state=%v", name, timeout, downOper, downPower)
	} else {
		t.Logf("Component %s powered down after %.2f minutes: oper-status=%v, power-admin-state=%v", name, time.Since(start).Minutes(), downOper, downPower)
	}

	// Let hardware power-down settle completely before re-enabling so POWER_ENABLED does not race physical shutdown.
	time.Sleep(15 * time.Second)

	start = time.Now()
	t.Logf("Starting %s POWER_ENABLE", name)
	gnmi.Replace(t, dut, config, oc.Platform_ComponentPowerType_POWER_ENABLED)

	// Poll up to timeout for component oper-status to return to ACTIVE.
	upDeadline := time.Now().Add(timeout)
	lastRetry := time.Now()
	var upOper oc.E_PlatformTypes_COMPONENT_OPER_STATUS
	var upPower oc.E_Platform_ComponentPowerType
	var isUp bool
	for time.Now().Before(upDeadline) {
		testt.CaptureFatal(t, func(t testing.TB) {
			if val, ok := gnmi.Lookup(t, dut, state).Val(); ok {
				upPower = val
			}
			if val, ok := gnmi.Lookup(t, dut, c.OperStatus().State()).Val(); ok {
				upOper = val
			}
		})
		if upOper == oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE {
			isUp = true
			break
		}
		// If the component is still DISABLED after 5 minutes, re-apply POWER_ENABLED.
		if upOper == oc.PlatformTypes_COMPONENT_OPER_STATUS_DISABLED && time.Since(lastRetry) >= 5*time.Minute {
			t.Logf("Component %s still in oper-status=%v after %.2f minutes; re-applying POWER_ENABLED", name, upOper, time.Since(start).Minutes())
			setComponentPowerAdmin(t, dut, name, cType, oc.Platform_ComponentPowerType_POWER_ENABLED)
			lastRetry = time.Now()
		}
		time.Sleep(10 * time.Second)
	}

	if !isUp {
		t.Errorf("Component %s oper-status after POWER_ENABLED, got: %v, want: ACTIVE", name, upOper)
	} else {
		t.Logf("Component %s recovered after %.2f minutes: oper-status=%v, power-admin-state=%v", name, time.Since(start).Minutes(), upOper, upPower)
	}
}
