// Package power_admin_down_up_test tests the power-admin-state leaf configuration
// on fabrics, controllers and linecards.
package power_admin_down_up_test

import (
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestFabricPowerAdmin(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	fs := components.FindComponentsByType(t, dut, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_FABRIC)

	for _, f := range fs {
		t.Run(f, func(t *testing.T) {
			oper := gnmi.Get(t, dut, gnmi.OC().Component(f).OperStatus().State())

			if got, want := oper, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE; got != want {
				t.Skipf("Fabric Component %s is already INACTIVE, hence skipping", f)
			}

			powerDownUp(t, dut, f, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_FABRIC, 3*time.Minute)
		})
	}
}

func TestLinecardPowerAdmin(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ls := components.FindComponentsByType(t, dut, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_LINECARD)

	for _, l := range ls {
		t.Run(l, func(t *testing.T) {
			empty, ok := gnmi.Lookup(t, dut, gnmi.OC().Component(l).Empty().State()).Val()
			if ok && empty {
				t.Skipf("Linecard Component %s is empty, hence skipping", l)
			}

			oper := gnmi.Get(t, dut, gnmi.OC().Component(l).OperStatus().State())

			if got, want := oper, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE; got != want {
				t.Skipf("Linecard Component %s is already INACTIVE, hence skipping", l)
			}

			powerDownUp(t, dut, l, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_LINECARD, 3*time.Minute)
		})
	}
}

func TestControllerCardPowerAdmin(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	cs := components.FindComponentsByType(t, dut, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD)

	for _, c := range cs {
		t.Run(c, func(t *testing.T) {
			role := gnmi.Get(t, dut, gnmi.OC().Component(c).RedundantRole().State())
			if got, want := role, oc.Platform_ComponentRedundantRole_PRIMARY; got == want {
				t.Skipf("ControllerCard Component %s is PRIMARY, hence skipping", c)
			}

			oper := gnmi.Get(t, dut, gnmi.OC().Component(c).OperStatus().State())
			if got, want := oper, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE; got != want {
				t.Skipf("ControllerCard Component %s is already INACTIVE, hence skipping", c)
			}

			powerDownUp(t, dut, c, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD, 3*time.Minute)
		})
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

	start := time.Now()
	t.Logf("Starting %s POWER_DISABLE", name)
	gnmi.Replace(t, dut, config, oc.Platform_ComponentPowerType_POWER_DISABLED)

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

	power, ok = gnmi.Await(t, dut, state, timeout, oc.Platform_ComponentPowerType_POWER_ENABLED).Val()
	if !ok {
		t.Errorf("Component %s, power-admin-state got: %v, want: %v", name, power, oc.Platform_ComponentPowerType_POWER_ENABLED)
	}
	t.Logf("Component %s, power-admin-state after %f minutes: %v", name, time.Since(start).Minutes(), power)

	oper, ok = gnmi.Await(t, dut, c.OperStatus().State(), timeout, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE).Val()
	if !ok {
		t.Errorf("Component %s oper-status after POWER_ENABLED, got: %v, want: %v", name, oper, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE)
	}
	t.Logf("Component %s, oper-status after %f minutes: %v", name, time.Since(start).Minutes(), oper)
}
