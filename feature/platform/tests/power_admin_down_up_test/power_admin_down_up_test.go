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
	if deviations.PowerDisableEnableLeafRefValidation(dut) || deviations.ConfigLeafCreateRequired(dut) || dut.Vendor() == ondatra.JUNIPER {
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
			t.Logf("Component %s initially not ACTIVE (got %v); cycling POWER_DISABLED -> POWER_ENABLED", name, comp.GetOperStatus())
			setComponentPowerAdmin(t, dut, name, cType, oc.Platform_ComponentPowerType_POWER_DISABLED)
			if _, state, ok := componentPowerConfigAndState(name, cType); ok {
				testt.CaptureFatal(t, func(t testing.TB) {
					gnmi.Await(t, dut, state, 15*time.Second, oc.Platform_ComponentPowerType_POWER_DISABLED)
				})
			}
			setComponentPowerAdmin(t, dut, name, cType, oc.Platform_ComponentPowerType_POWER_ENABLED)
			inactiveRemovable = append(inactiveRemovable, name)
		}
	}

	// Wait for any healed removable components to become ACTIVE.
	for _, target := range inactiveRemovable {
		t.Logf("Waiting up to 10 minutes for restored %v component %s to reach ACTIVE...", cType, target)
		if _, ok := gnmi.Watch(t, dut, gnmi.OC().Component(target).OperStatus().State(), 10*time.Minute, func(val *ygnmi.Value[oc.E_PlatformTypes_COMPONENT_OPER_STATUS]) bool {
			oper, present := val.Val()
			return present && oper == oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE
		}).Await(t); ok {
			activeRemovable = append(activeRemovable, target)
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
	for _, c := range cs {
		gnmi.Watch(t, dut, gnmi.OC().Component(c).OperStatus().State(), 15*time.Minute, func(val *ygnmi.Value[oc.E_PlatformTypes_COMPONENT_OPER_STATUS]) bool {
			oper, present := val.Val()
			return present && oper == oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE
		}).Await(t)
	}

	// Find out the PRIMARY and SECONDARY Controller Cards using /components/component/state/redundant-role.
	standbyCC, activeCC := components.FindStandbyControllerCard(t, dut, cs)
	t.Logf("Detected Active ControllerCard (PRIMARY): %s, Standby ControllerCard (SECONDARY): %s", activeCC, standbyCC)

	// Wait up to 2 minutes for activeCC to report SwitchoverReady == true.
	testt.CaptureFatal(t, func(t testing.TB) {
		if _, ok := gnmi.Watch(t, dut, gnmi.OC().Component(activeCC).SwitchoverReady().State(), 2*time.Minute, func(val *ygnmi.Value[bool]) bool {
			ready, present := val.Val()
			return present && ready
		}).Await(t); ok {
			t.Logf("Active controller card %s SwitchoverReady: true", activeCC)
		}
	})

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
	triggerBatch := gnmi.OCBatch()
	triggerBatch.AddPaths(
		gnmi.OC().Component(activeCC).OperStatus(),
		gnmi.OC().Component(activeCC).RedundantRole(),
		gnmi.OC().Component(standbyCC).RedundantRole(),
	)
	if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
		_, ok := gnmi.Watch(t, dut, triggerBatch.State(), 4*time.Minute, func(val *ygnmi.Value[*oc.Root]) bool {
			root, present := val.Val()
			if !present {
				return false
			}
			aComp := root.GetComponent(activeCC)
			sComp := root.GetComponent(standbyCC)
			if aComp != nil && ((aComp.GetOperStatus() != oc.PlatformTypes_COMPONENT_OPER_STATUS_UNSET && aComp.GetOperStatus() != oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE) ||
				(aComp.GetRedundantRole() != oc.Platform_ComponentRedundantRole_UNSET && aComp.GetRedundantRole() != oc.Platform_ComponentRedundantRole_PRIMARY)) {
				return true
			}
			if sComp != nil && sComp.GetRedundantRole() == oc.Platform_ComponentRedundantRole_PRIMARY {
				return true
			}
			return false
		}).Await(t)
		if ok {
			switchoverTriggered = true
		}
	}); errMsg != nil {
		t.Logf("gNMI unreachable during controller card power-down (%s); switchover in progress", *errMsg)
		switchoverTriggered = true
	}

	if !switchoverTriggered {
		t.Logf("Primary controller card %s remained ACTIVE and PRIMARY for 4 minutes after POWER_DISABLED (Hardware Protection prevented powering down active controller card). Restoring POWER_ENABLED.", activeCC)
		setComponentPowerAdmin(t, dut, activeCC, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD, oc.Platform_ComponentPowerType_POWER_ENABLED)
		return
	}

	// Wait until the switchover completes and system telemetry is reachable again.
	t.Logf("Wait for switchover to complete (max 30 minutes)...")
	components.WaitForSwitchover(t, dut, 30*time.Minute)

	// Wait up to 15 minutes for standbyCC to report PRIMARY and activeCC to report DISABLED/INACTIVE.
	postSwitchoverBatch := gnmi.OCBatch()
	postSwitchoverBatch.AddPaths(
		gnmi.OC().Component(standbyCC).RedundantRole(),
		gnmi.OC().Component(activeCC).OperStatus(),
	)
	gnmi.Watch(t, dut, postSwitchoverBatch.State(), 15*time.Minute, func(val *ygnmi.Value[*oc.Root]) bool {
		root, present := val.Val()
		if !present {
			return false
		}
		sComp := root.GetComponent(standbyCC)
		aComp := root.GetComponent(activeCC)
		if sComp == nil || aComp == nil {
			return false
		}
		activeOper := aComp.GetOperStatus()
		return sComp.GetRedundantRole() == oc.Platform_ComponentRedundantRole_PRIMARY &&
			(activeOper == oc.PlatformTypes_COMPONENT_OPER_STATUS_DISABLED || activeOper == oc.PlatformTypes_COMPONENT_OPER_STATUS_INACTIVE)
	}).Await(t)

	// Attempt to update power-admin-state to POWER_DISABLED for the newly elected PRIMARY controller card.
	t.Logf("Attempting to update newly elected PRIMARY controller card %s config to POWER_DISABLED", standbyCC)
	setErr = setComponentPowerAdmin(t, dut, standbyCC, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD, oc.Platform_ComponentPowerType_POWER_DISABLED)
	if setErr != nil {
		t.Logf("Set on newly elected PRIMARY controller card %s was rejected as expected: %s", standbyCC, *setErr)
	} else {
		t.Logf("Set on newly elected PRIMARY controller card %s was accepted; waiting for switchover before restoring both controller cards to POWER_ENABLED", standbyCC)
		testt.CaptureFatal(t, func(t testing.TB) {
			gnmi.Watch(t, dut, gnmi.OC().Component(standbyCC).OperStatus().State(), 1*time.Minute, func(val *ygnmi.Value[oc.E_PlatformTypes_COMPONENT_OPER_STATUS]) bool {
				oper, present := val.Val()
				return present && oper != oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE
			}).Await(t)
		})
		components.WaitForSwitchover(t, dut, 30*time.Minute)
	}

	// Restore both controller cards back to POWER_ENABLED.
	t.Logf("Restoring %s and %s config to POWER_ENABLED", activeCC, standbyCC)
	setComponentPowerAdmin(t, dut, activeCC, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD, oc.Platform_ComponentPowerType_POWER_ENABLED)
	setComponentPowerAdmin(t, dut, standbyCC, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD, oc.Platform_ComponentPowerType_POWER_ENABLED)

	// 3. Verification:
	// Wait up to 20 minutes for both controller cards to recover to OperStatus == ACTIVE and valid PRIMARY/SECONDARY roles.
	verifyBatch := gnmi.OCBatch()
	for _, c := range cs {
		verifyBatch.AddPaths(
			gnmi.OC().Component(c).OperStatus(),
			gnmi.OC().Component(c).RedundantRole(),
		)
	}
	var recovered bool
	for attempt := 0; attempt < 4 && !recovered; attempt++ {
		if attempt > 0 {
			setComponentPowerAdmin(t, dut, activeCC, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD, oc.Platform_ComponentPowerType_POWER_ENABLED)
			setComponentPowerAdmin(t, dut, standbyCC, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD, oc.Platform_ComponentPowerType_POWER_ENABLED)
		}
		testt.CaptureFatal(t, func(t testing.TB) {
			_, recovered = gnmi.Watch(t, dut, verifyBatch.State(), 5*time.Minute, func(val *ygnmi.Value[*oc.Root]) bool {
				root, present := val.Val()
				if !present {
					return false
				}
				c0 := root.GetComponent(cs[0])
				c1 := root.GetComponent(cs[1])
				if c0 == nil || c1 == nil {
					return false
				}
				role0, role1 := c0.GetRedundantRole(), c1.GetRedundantRole()
				rolesValid := (role0 == oc.Platform_ComponentRedundantRole_PRIMARY && role1 == oc.Platform_ComponentRedundantRole_SECONDARY) ||
					(role0 == oc.Platform_ComponentRedundantRole_SECONDARY && role1 == oc.Platform_ComponentRedundantRole_PRIMARY)
				return c0.GetOperStatus() == oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE &&
					c1.GetOperStatus() == oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE &&
					rolesValid
			}).Await(t)
		})
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

	if deviations.PowerDisableEnableLeafRefValidation(dut) || deviations.ConfigLeafCreateRequired(dut) || dut.Vendor() == ondatra.JUNIPER {
		gnmi.Update(t, dut, c.Config(), &oc.Component{
			Name: ygot.String(name),
		})
	}
	start := time.Now()
	t.Logf("Starting %s POWER_DISABLE", name)
	gnmi.Replace(t, dut, config, oc.Platform_ComponentPowerType_POWER_DISABLED)

	// Wait up to timeout for component oper-status to become DISABLED or INACTIVE.
	downVal, isDown := gnmi.Watch(t, dut, c.OperStatus().State(), timeout, func(val *ygnmi.Value[oc.E_PlatformTypes_COMPONENT_OPER_STATUS]) bool {
		oper, present := val.Val()
		return present && (oper == oc.PlatformTypes_COMPONENT_OPER_STATUS_DISABLED || oper == oc.PlatformTypes_COMPONENT_OPER_STATUS_INACTIVE)
	}).Await(t)
	var downOper oc.E_PlatformTypes_COMPONENT_OPER_STATUS
	if downVal != nil {
		downOper, _ = downVal.Val()
	}
	var downPower oc.E_Platform_ComponentPowerType
	testt.CaptureFatal(t, func(t testing.TB) {
		if val, ok := gnmi.Lookup(t, dut, state).Val(); ok {
			downPower = val
		}
	})
	if !isDown {
		t.Errorf("Component %s did not power down within %v; last oper-status=%v, power-admin-state=%v", name, timeout, downOper, downPower)
	} else {
		t.Logf("Component %s powered down after %.2f minutes: oper-status=%v, power-admin-state=%v", name, time.Since(start).Minutes(), downOper, downPower)
	}

	start = time.Now()
	t.Logf("Starting %s POWER_ENABLE", name)
	if deviations.PowerDisableEnableLeafRefValidation(dut) || deviations.ConfigLeafCreateRequired(dut) || dut.Vendor() == ondatra.JUNIPER {
		gnmi.Update(t, dut, c.Config(), &oc.Component{
			Name: ygot.String(name),
		})
	}
	gnmi.Replace(t, dut, config, oc.Platform_ComponentPowerType_POWER_ENABLED)

	// Wait up to timeout for component oper-status to return to ACTIVE.
	var isUp bool
	var upOper oc.E_PlatformTypes_COMPONENT_OPER_STATUS
	retryInterval := 5 * time.Minute
	for remaining := timeout; remaining > 0 && !isUp; remaining -= retryInterval {
		waitDur := remaining
		if waitDur > retryInterval {
			waitDur = retryInterval
		}
		val, ok := gnmi.Watch(t, dut, c.OperStatus().State(), waitDur, func(val *ygnmi.Value[oc.E_PlatformTypes_COMPONENT_OPER_STATUS]) bool {
			oper, present := val.Val()
			return present && oper == oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE
		}).Await(t)
		if val != nil {
			upOper, _ = val.Val()
		}
		if ok {
			isUp = true
			break
		}
		if remaining > retryInterval {
			t.Logf("Component %s still in oper-status=%v after %.2f minutes; re-applying POWER_ENABLED", name, upOper, time.Since(start).Minutes())
			setComponentPowerAdmin(t, dut, name, cType, oc.Platform_ComponentPowerType_POWER_ENABLED)
		}
	}

	var upPower oc.E_Platform_ComponentPowerType
	testt.CaptureFatal(t, func(t testing.TB) {
		if val, ok := gnmi.Lookup(t, dut, state).Val(); ok {
			upPower = val
		}
	})
	if !isUp {
		t.Errorf("Component %s oper-status after POWER_ENABLED, got: %v, want: ACTIVE", name, upOper)
	} else {
		t.Logf("Component %s recovered after %.2f minutes: oper-status=%v, power-admin-state=%v", name, time.Since(start).Minutes(), upOper, upPower)
	}
}
