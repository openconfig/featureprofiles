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
	runPowerAdminTest(t, dut, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_FABRIC, 6*time.Minute)
}

func TestLinecardPowerAdmin(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	runPowerAdminTest(t, dut, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_LINECARD, 20*time.Minute)
}

func runPowerAdminTest(t *testing.T, dut *ondatra.DUTDevice, cType oc.E_PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT, timeout time.Duration) {
	t.Helper()
	cs := components.FindComponentsByType(t, dut, cType)

	// Test Setup: Verify state/oper-status is ACTIVE for all installed cards of cType.
	batch := gnmi.OCBatch()
	for _, name := range cs {
		batch.AddPaths(
			gnmi.OC().Component(name).Empty(),
			gnmi.OC().Component(name).Removable(),
			gnmi.OC().Component(name).OperStatus(),
		)
	}
	results := gnmi.Get(t, dut, batch.State())
	for _, name := range cs {
		comp := results.GetComponent(name)
		if comp == nil || comp.GetEmpty() || !comp.GetRemovable() {
			continue
		}
		if comp.GetOperStatus() != oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE {
			t.Errorf("Component %s initially not ACTIVE, got: %s", name, comp.GetOperStatus())
		}
	}

	for _, name := range cs {
		t.Run(name, func(t *testing.T) {
			empty, ok := gnmi.Lookup(t, dut, gnmi.OC().Component(name).Empty().State()).Val()
			if ok && empty {
				t.Skipf("Component %s is empty, hence skipping", name)
			}
			if !gnmi.Get(t, dut, gnmi.OC().Component(name).Removable().State()) {
				t.Skipf("Skip the test on non-removable component.")
			}
			oper := gnmi.Get(t, dut, gnmi.OC().Component(name).OperStatus().State())
			if got, want := oper, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE; got != want {
				t.Skipf("Component %s is already INACTIVE, hence skipping", name)
			}

			// Only track connected DUT ports rather than all chassis ports
			before := helpers.FetchOperStatusUPIntfs(t, dut, true)
			powerDownUp(t, dut, name, cType, timeout)

			// Await the component to finish booting and reach ACTIVE before validating interfaces
			gnmi.Await(t, dut, gnmi.OC().Component(name).OperStatus().State(), 10*time.Minute, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE)

			// Validate interfaces with a 20-minute timeout for Cisco 8808 fabric convergence
			helpers.ValidateOperStatusUPIntfs(t, dut, before, 20*time.Minute)
		})
	}
}

func TestControllerCardPowerAdmin(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	if deviations.SkipControllerCardPowerAdmin(dut) {
		t.Skipf("Power-admin-state config on controller card is not supported.")
	}

	cs := components.FindComponentsByType(t, dut, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD)
	if len(cs) < 2 {
		t.Skipf("Number of controller cards is less than 2. Skipping test for controller-card power-admin-state.")
	}

	// 1. Test Setup:
	// Verify that /components/component/state/oper-status is ACTIVE and switchover-ready is true for both controller cards.
	setupBatch := gnmi.OCBatch()
	for _, c := range cs {
		comp := gnmi.OC().Component(c)
		setupBatch.AddPaths(
			comp.OperStatus(),
			comp.SwitchoverReady(),
		)
	}

	gnmi.Watch(t, dut, setupBatch.State(), 2*time.Minute, func(val *ygnmi.Value[*oc.Root]) bool {
		root, present := val.Val()
		if !present || root == nil {
			return false
		}
		for _, c := range cs {
			comp := root.GetComponent(c)
			if comp == nil ||
				comp.GetOperStatus() != oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE ||
				!comp.GetSwitchoverReady() {
				return false
			}
		}
		return true
	}).Await(t)

	// 2. Test Logic:
	// Find out the PRIMARY Controller Card using /components/component/state/redundant-role.
	standbyCC, activeCC := components.FindStandbyControllerCard(t, dut, cs)
	t.Logf("Detected Active ControllerCard (PRIMARY): %s, Standby ControllerCard (SECONDARY): %s", activeCC, standbyCC)

	// Using gNMI Set, attempt to update /components/component/controller-card/config/power-admin-state to POWER_DISABLED for the PRIMARY controller card.
	primaryConfig := gnmi.OC().Component(activeCC).ControllerCard().PowerAdminState().Config()
	primaryState := gnmi.OC().Component(activeCC).ControllerCard().PowerAdminState().State()
	newActiveConfig := gnmi.OC().Component(standbyCC).ControllerCard().PowerAdminState().Config()

	// Register a safety net configuration cleanup to restore primary & standby controllers to POWER_ENABLED.
	t.Cleanup(func() {
		t.Logf("Cleaning up: Restoring controller cards back to POWER_ENABLED...")
		if deviations.PowerDisableEnableLeafRefValidation(dut) {
			gnmi.Update(t, dut, gnmi.OC().Component(activeCC).Config(), &oc.Component{
				Name: ygot.String(activeCC),
			})
			gnmi.Update(t, dut, gnmi.OC().Component(standbyCC).Config(), &oc.Component{
				Name: ygot.String(standbyCC),
			})
		}
		gnmi.Replace(t, dut, primaryConfig, oc.Platform_ComponentPowerType_POWER_ENABLED)
		gnmi.Replace(t, dut, newActiveConfig, oc.Platform_ComponentPowerType_POWER_ENABLED)
	})

	if deviations.PowerDisableEnableLeafRefValidation(dut) {
		gnmi.Update(t, dut, gnmi.OC().Component(activeCC).Config(), &oc.Component{
			Name: ygot.String(activeCC),
		})
	}
	t.Logf("Updating config to POWER_DISABLED for primary controller card: %s", activeCC)
	gnmi.Replace(t, dut, primaryConfig, oc.Platform_ComponentPowerType_POWER_DISABLED)

	// Wait until the switchover happens and the SECONDARY becomes PRIMARY.
	t.Logf("Wait for switchover to complete (max 30 minutes)...")
	components.WaitForSwitchover(t, dut, 30*time.Minute)

	// Verify that the /components/component/state/oper-status of the disabled Controller Card is now DISABLED.
	oper, ok := gnmi.Await(t, dut, gnmi.OC().Component(activeCC).OperStatus().State(), 20*time.Minute, oc.PlatformTypes_COMPONENT_OPER_STATUS_DISABLED).Val()
	if !ok {
		t.Errorf("Component %s (old primary) oper-status, got: %v, want: DISABLED", activeCC, oper)
	}

	if !deviations.MissingValueForDefaults(dut) {
		power, ok := gnmi.Await(t, dut, primaryState, 10*time.Minute, oc.Platform_ComponentPowerType_POWER_DISABLED).Val()
		if !ok {
			t.Errorf("Component %s, power-admin-state got: %v, want: POWER_DISABLED", activeCC, power)
		}
	}

	// Verify the newly elected PRIMARY controller card (old secondary/standbyCC) redundant role is PRIMARY.
	newActiveRole := gnmi.Get(t, dut, gnmi.OC().Component(standbyCC).RedundantRole().State())
	if newActiveRole != oc.Platform_ComponentRedundantRole_PRIMARY {
		t.Errorf("Newly elected PRIMARY controller card %s redundant role, got: %v, want: PRIMARY", standbyCC, newActiveRole)
	}

	// Using gNMI Set, attempt to update /components/component/controller-card/config/power-admin-state to POWER_DISABLED for the newly elected PRIMARY controller card.
	if deviations.PowerDisableEnableLeafRefValidation(dut) {
		gnmi.Update(t, dut, gnmi.OC().Component(standbyCC).Config(), &oc.Component{
			Name: ygot.String(standbyCC),
		})
	}
	t.Logf("Attempting to update newly elected PRIMARY controller card %s config to POWER_DISABLED", standbyCC)

	var setErr *string
	setErr = testt.CaptureFatal(t, func(t testing.TB) {
		gnmi.Replace(t, dut, newActiveConfig, oc.Platform_ComponentPowerType_POWER_DISABLED)
	})

	if setErr == nil {
		t.Logf("Set was accepted. Waiting for the automatic reactivation / switchover back to %s", activeCC)
		components.WaitForSwitchover(t, dut, 30*time.Minute)

		// Wait for standbyCC to become DISABLED, and activeCC to become ACTIVE concurrently.
		switchBatch := gnmi.OCBatch()
		switchBatch.AddPaths(
			gnmi.OC().Component(standbyCC).OperStatus(),
			gnmi.OC().Component(activeCC).OperStatus(),
		)
		gnmi.Watch(t, dut, switchBatch.State(), 20*time.Minute, func(val *ygnmi.Value[*oc.Root]) bool {
			root, present := val.Val()
			if !present || root == nil {
				return false
			}
			sComp := root.GetComponent(standbyCC)
			aComp := root.GetComponent(activeCC)
			return sComp != nil && sComp.GetOperStatus() == oc.PlatformTypes_COMPONENT_OPER_STATUS_DISABLED &&
				aComp != nil && aComp.GetOperStatus() == oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE
		}).Await(t)

		// Restore the config for the disabled card back to POWER_ENABLED
		t.Logf("Restoring %s config to POWER_ENABLED", standbyCC)
		gnmi.Replace(t, dut, newActiveConfig, oc.Platform_ComponentPowerType_POWER_ENABLED)
		gnmi.Await(t, dut, gnmi.OC().Component(standbyCC).OperStatus().State(), 20*time.Minute, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE)
	} else {
		t.Logf("Set was rejected as expected: %v", *setErr)

		// Restore the config for the disabled card back to POWER_ENABLED
		t.Logf("Restoring %s config to POWER_ENABLED", activeCC)
		gnmi.Replace(t, dut, primaryConfig, oc.Platform_ComponentPowerType_POWER_ENABLED)
		gnmi.Await(t, dut, gnmi.OC().Component(activeCC).OperStatus().State(), 20*time.Minute, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE)
	}

	// 3. Verification:
	// Verify the operational status and switchover readiness of the controller cards post-recovery concurrently.
	verifyBatch := gnmi.OCBatch()
	for _, c := range cs {
		comp := gnmi.OC().Component(c)
		verifyBatch.AddPaths(
			comp.OperStatus(),
			comp.SwitchoverReady(),
		)
	}

	gnmi.Watch(t, dut, verifyBatch.State(), 20*time.Minute, func(val *ygnmi.Value[*oc.Root]) bool {
		root, present := val.Val()
		if !present || root == nil {
			return false
		}
		for _, c := range cs {
			comp := root.GetComponent(c)
			if comp == nil ||
				comp.GetOperStatus() != oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE ||
				!comp.GetSwitchoverReady() {
				return false
			}
		}
		return true
	}).Await(t)

	// Fetch both redundant roles concurrently in a single batched RPC
	roleBatch := gnmi.OCBatch()
	rolePath1 := gnmi.OC().Component(cs[0]).RedundantRole().State()
	rolePath2 := gnmi.OC().Component(cs[1]).RedundantRole().State()

	roleBatch.AddPaths(rolePath1, rolePath2)
	roleRes := roleBatch.Get(t, dut)

	role1 := gnmi.Lookup(t, roleRes, rolePath1).Val(t)
	role2 := gnmi.Lookup(t, roleRes, rolePath2).Val(t)

	if (role1 == oc.Platform_ComponentRedundantRole_PRIMARY && role2 == oc.Platform_ComponentRedundantRole_SECONDARY) ||
		(role1 == oc.Platform_ComponentRedundantRole_SECONDARY && role2 == oc.Platform_ComponentRedundantRole_PRIMARY) {
		t.Logf("Operational status and redundant roles of controller cards have recovered successfully. %s role: %v, %s role: %v", cs[0], role1, cs[1], role2)
	} else {
		t.Errorf("Unexpected redundant roles post-recovery. %s: %v, %s: %v", cs[0], role1, cs[1], role2)
	}
}
func powerDownUp(t *testing.T, dut *ondatra.DUTDevice, name string, cType oc.E_PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT, timeout time.Duration) {
	c := gnmi.OC().Component(name)
	var config ygnmi.ConfigQuery[oc.E_Platform_ComponentPowerType]
	var state ygnmi.SingletonQuery[oc.E_Platform_ComponentPowerType]

	switch cType {
	case oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD:
		config = c.ControllerCard().PowerAdminState().Config()
		state = c.ControllerCard().PowerAdminState().State()
	case oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_LINECARD:
		config = c.Linecard().PowerAdminState().Config()
		state = c.Linecard().PowerAdminState().State()
	case oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_FABRIC:
		config = c.Fabric().PowerAdminState().Config()
		state = c.Fabric().PowerAdminState().State()
	default:
		t.Fatalf("Unknown component type: %s", cType.String())
	}
	if deviations.PowerDisableEnableLeafRefValidation(dut) {
		gnmi.Update(t, dut, c.Config(), &oc.Component{
			Name: ygot.String(name),
		})
	}
	start := time.Now()
	t.Logf("Starting %s POWER_DISABLE", name)
	gnmi.Replace(t, dut, config, oc.Platform_ComponentPowerType_POWER_DISABLED)

	// Wait time for control plan to stabilize and redial grpc connection
	gnmi.Await(t, dut, gnmi.OC().Component(name).OperStatus().State(), 5*time.Minute, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE)

	power, ok := gnmi.Await(t, dut, state, timeout, oc.Platform_ComponentPowerType_POWER_DISABLED).Val()
	if !ok {
		t.Errorf("Component %s, power-admin-state got: %v, want: %v", name, power, oc.Platform_ComponentPowerType_POWER_DISABLED)
	}
	t.Logf("Component %s, power-admin-state after %f minutes: %v", name, time.Since(start).Minutes(), power)

	oper, ok := gnmi.Await(t, dut, c.OperStatus().State(), timeout, oc.PlatformTypes_COMPONENT_OPER_STATUS_DISABLED).Val()
	if !ok {
		t.Errorf("Component %s oper-status, got: %v, want: %v", name, oper, oc.PlatformTypes_COMPONENT_OPER_STATUS_DISABLED)
	}
	t.Logf("Component %s, oper-status after %f minutes: %v", name, time.Since(start).Minutes(), oper)
	start = time.Now()
	t.Logf("Starting %s POWER_ENABLE", name)
	gnmi.Replace(t, dut, config, oc.Platform_ComponentPowerType_POWER_ENABLED)

	if !deviations.MissingValueForDefaults(dut) {
		power, ok = gnmi.Await(t, dut, state, timeout, oc.Platform_ComponentPowerType_POWER_ENABLED).Val()
		if !ok {
			t.Errorf("Component %s, power-admin-state got: %v, want: %v", name, power, oc.Platform_ComponentPowerType_POWER_ENABLED)
		}
		t.Logf("Component %s, power-admin-state after %f minutes: %v", name, time.Since(start).Minutes(), power)
	}

	oper, ok = gnmi.Await(t, dut, c.OperStatus().State(), timeout, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE).Val()
	if !ok {
		t.Errorf("Component %s oper-status after POWER_ENABLED, got: %v, want: %v", name, oper, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE)
	}
	t.Logf("Component %s, oper-status after %f minutes: %v", name, time.Since(start).Minutes(), oper)
}
