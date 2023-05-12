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
			gnmi.Replace(t, dut, gnmi.OC().Component(f).Fabric().PowerAdminState().Config(), oc.Platform_ComponentPowerType_POWER_DISABLED)

			pw := gnmi.Watch(t, dut, gnmi.OC().Component(f).Fabric().PowerAdminState().State(), time.Minute, func(val *ygnmi.Value[oc.E_Platform_ComponentPowerType]) bool {
				pt, ok := val.Val()
				return ok && pt == oc.Platform_ComponentPowerType_POWER_DISABLED
			})

			p, ok := pw.Await(t)
			power, _ := p.Val()
			if !ok {
				t.Errorf("Fabric Component %s, power-admin-state got: %v, want: %v", f, power, oc.Platform_ComponentPowerType_POWER_DISABLED)
			}

			opw := gnmi.Watch(t, dut, gnmi.OC().Component(f).OperStatus().State(), time.Minute, func(val *ygnmi.Value[oc.E_PlatformTypes_COMPONENT_OPER_STATUS]) bool {
				pt, ok := val.Val()
				return ok && pt != oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE
			})

			o, ok := opw.Await(t)
			oper, _ = o.Val()
			if got, want := oper, oc.PlatformTypes_COMPONENT_OPER_STATUS_DISABLED; got != want {
				t.Errorf("Linecard Component %s oper-status, got: %v, want: %v", f, got, want)
			}

			gnmi.Replace(t, dut, gnmi.OC().Component(f).Fabric().PowerAdminState().Config(), oc.Platform_ComponentPowerType_POWER_ENABLED)

			op := gnmi.Watch(t, dut, gnmi.OC().Component(f).OperStatus().State(), 5*time.Minute, func(val *ygnmi.Value[oc.E_PlatformTypes_COMPONENT_OPER_STATUS]) bool {
				s, ok := val.Val()
				return ok && s == oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE
			})
			s, ok := op.Await(t)
			status, _ := s.Val()
			if !ok {
				t.Errorf("Fabric Component %s oper-status after POWER_ENABLED, got: %v, want: %v", f, status, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE)
			}
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
			gnmi.Replace(t, dut, gnmi.OC().Component(l).Linecard().PowerAdminState().Config(), oc.Platform_ComponentPowerType_POWER_DISABLED)

			pw := gnmi.Watch(t, dut, gnmi.OC().Component(l).Linecard().PowerAdminState().State(), time.Minute, func(val *ygnmi.Value[oc.E_Platform_ComponentPowerType]) bool {
				pt, ok := val.Val()
				return ok && pt == oc.Platform_ComponentPowerType_POWER_DISABLED
			})

			p, ok := pw.Await(t)
			power, _ := p.Val()
			if !ok {
				t.Errorf("Linecard Component %s, power-admin-state got: %v, want: %v", l, power, oc.Platform_ComponentPowerType_POWER_DISABLED)
			}

			opw := gnmi.Watch(t, dut, gnmi.OC().Component(l).OperStatus().State(), time.Minute, func(val *ygnmi.Value[oc.E_PlatformTypes_COMPONENT_OPER_STATUS]) bool {
				pt, ok := val.Val()
				return ok && pt != oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE
			})

			o, ok := opw.Await(t)
			oper, _ = o.Val()
			if got, want := oper, oc.PlatformTypes_COMPONENT_OPER_STATUS_DISABLED; got != want {
				t.Errorf("Linecard Component %s oper-status, got: %v, want: %v", l, got, want)
			}

			gnmi.Replace(t, dut, gnmi.OC().Component(l).Linecard().PowerAdminState().Config(), oc.Platform_ComponentPowerType_POWER_ENABLED)

			op := gnmi.Watch(t, dut, gnmi.OC().Component(l).OperStatus().State(), 5*time.Minute, func(val *ygnmi.Value[oc.E_PlatformTypes_COMPONENT_OPER_STATUS]) bool {
				s, ok := val.Val()
				return ok && s == oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE
			})
			s, ok := op.Await(t)
			status, _ := s.Val()
			if !ok {
				t.Errorf("Linecard Component %s oper-status after POWER_ENABLED, got: %v, want: %v", l, status, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE)
			}
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

			gnmi.Replace(t, dut, gnmi.OC().Component(c).ControllerCard().PowerAdminState().Config(), oc.Platform_ComponentPowerType_POWER_DISABLED)

			pw := gnmi.Watch(t, dut, gnmi.OC().Component(c).ControllerCard().PowerAdminState().State(), time.Minute, func(val *ygnmi.Value[oc.E_Platform_ComponentPowerType]) bool {
				pt, ok := val.Val()
				return ok && pt == oc.Platform_ComponentPowerType_POWER_DISABLED
			})

			p, ok := pw.Await(t)
			power, _ := p.Val()
			if !ok {
				t.Errorf("ControllerCard Component %s, power-admin-state got: %v, want: %v", c, power, oc.Platform_ComponentPowerType_POWER_DISABLED)
			}

			opw := gnmi.Watch(t, dut, gnmi.OC().Component(c).OperStatus().State(), time.Minute, func(val *ygnmi.Value[oc.E_PlatformTypes_COMPONENT_OPER_STATUS]) bool {
				pt, ok := val.Val()
				return ok && pt != oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE
			})

			o, ok := opw.Await(t)
			oper, _ = o.Val()
			if got, want := oper, oc.PlatformTypes_COMPONENT_OPER_STATUS_DISABLED; got != want {
				t.Errorf("Controllercard Component %s oper-status, got: %v, want: %v", c, got, want)
			}

			gnmi.Replace(t, dut, gnmi.OC().Component(c).ControllerCard().PowerAdminState().Config(), oc.Platform_ComponentPowerType_POWER_ENABLED)

			op := gnmi.Watch(t, dut, gnmi.OC().Component(c).OperStatus().State(), 30*time.Minute, func(val *ygnmi.Value[oc.E_PlatformTypes_COMPONENT_OPER_STATUS]) bool {
				s, ok := val.Val()
				return ok && s == oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE
			})
			s, ok := op.Await(t)
			status, _ := s.Val()
			if !ok {
				t.Errorf("ControllerCard Component %s oper-status after POWER_ENABLED, got: %v, want: %v", c, status, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE)
			}
		})
	}
}
