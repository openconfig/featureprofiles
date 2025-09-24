package last_reboot_test

import (
	"context"
	"testing"

	"github.com/openconfig/featureprofiles/internal/cisco/ha/utils"
	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/fptest"
	gnoisys "github.com/openconfig/gnoi/system"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// If test is run on a freshly brought-up sim, then the LastRebootTime should still be populated
// since we are taking the behaviour of the new uprevved "boot-time" leaf
func TestInitialBootTime(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	var controllerCard string

	controllerCards := components.FindComponentsByType(t, dut, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD)
	if len(controllerCards) > 0 {
		controllerCard = controllerCards[0]
	} else {
		t.Fatal("could not find controller card")
	}

	if !(gnmi.Lookup(t, dut, gnmi.OC().Component(controllerCard).LastRebootTime().State()).IsPresent()) {
		t.Fatal("value for LastRebootTime is not present")
	}

}

func TestRouterLastRebootTime(t *testing.T) {

	dut := ondatra.DUT(t, "dut")

	rebootMethods := []gnoisys.RebootMethod{
		gnoisys.RebootMethod_COLD,
		gnoisys.RebootMethod_WARM,
	}

	var controllerCard string

	controllerCards := components.FindComponentsByType(t, dut, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD)
	if len(controllerCards) > 0 {
		controllerCard = controllerCards[0]
	} else {
		t.Fatal("could not find controller card")
	}

	// reload with each reboot method listed above
	for _, rebootMethod := range rebootMethods {
		t.Run(gnoisys.RebootMethod_name[int32(rebootMethod)], func(t *testing.T) {
			lastRebootTimeBefore := gnmi.Get(t, dut, gnmi.OC().Component(controllerCard).LastRebootTime().State())

			util.ReloadRouterWithRebootMethod(t, dut, rebootMethod)

			lastRebootTimeAfter := gnmi.Get(t, dut, gnmi.OC().Component(controllerCard).LastRebootTime().State())

			if lastRebootTimeAfter < lastRebootTimeBefore {
				t.Fatalf("LastRebootTime().Get(t): got %v, want > %v", lastRebootTimeAfter, lastRebootTimeBefore)
			}
		})
	}
}

func TestLCReloadLastRebootTime(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	rebootMethods := []gnoisys.RebootMethod{
		gnoisys.RebootMethod_COLD,
		gnoisys.RebootMethod_WARM,
	}

	lcs := components.FindComponentsByType(t, dut, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_LINECARD)
	if len(lcs) < 2 {
		t.Log("No linecards, skipping")
		t.Skip()
	}

	linecards := []string{}

	for _, lc := range lcs {
		oper := gnmi.Get(t, dut, gnmi.OC().Component(lc).OperStatus().State())

		if got, want := oper, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE; got == want {
			linecards = append(linecards, lc)
		}
	}

	if len(linecards) == 0 {
		t.Log("No linecards found, skipping test")
		t.Skip()
	}

	for _, rebootMethod := range rebootMethods {
		t.Run(gnoisys.RebootMethod_name[int32(rebootMethod)], func(t *testing.T) {
			rebootTimes := make(map[string]uint64)

			for _, lc := range linecards {

				rebootTimes[lc] = gnmi.Get(t, dut, gnmi.OC().Component(lc).LastRebootTime().State())
			}

			util.ReloadLineCardsWithRebootMethod(t, dut, rebootMethod)

			for _, lc := range linecards {
				lastRebootTimeAfter := gnmi.Get(t, dut, gnmi.OC().Component(lc).LastRebootTime().State())
				if lastRebootTimeAfter < rebootTimes[lc] {
					t.Fatalf("LastRebootTime().Get(t): got %v, want > %v", lastRebootTimeAfter, rebootTimes[lc])
				}
			}
		})
	}
}

func TestRPFOLastRebootTime(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	rps := components.FindComponentsByType(t, dut, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD)
	// skip test if there are less than 2 RPs
	if len(rps) < 2 {
		t.Log("RPFO not configured, skipping")
		t.Skip()
	}
	_, rpActive := components.FindStandbyControllerCard(t, dut, rps)
	t.Logf("RPs: %v", rps)

	lastRebootTimeBefore := gnmi.Get(t, dut, gnmi.OC().Component(rpActive).LastRebootTime().State())

	// Dorpfo depends on metadata.textproto existing since it uses one of the fields
	utils.Dorpfo(context.Background(), t, false)

	lastRebootTimeAfter := gnmi.Get(t, dut, gnmi.OC().Component(rpActive).LastRebootTime().State())

	if lastRebootTimeAfter < lastRebootTimeBefore {
		t.Fatalf("LastRebootTime().Get(t): got %v, want > %v", lastRebootTimeAfter, lastRebootTimeBefore)
	}

}
