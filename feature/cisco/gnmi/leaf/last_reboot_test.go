package last_reboot_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/cisco/ha/utils"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/fptest"
	gnoisys "github.com/openconfig/gnoi/system"
	gnoitype "github.com/openconfig/gnoi/types"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/testt"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func reloadRouterWithRebootMethod(t *testing.T, dut *ondatra.DUTDevice, rebootMethod gnoisys.RebootMethod) error {
	gnoiClient, err := dut.RawAPIs().BindingDUT().DialGNOI(context.Background())
	if err != nil {
		t.Fatalf("Error dialing gNOI: %v", err)
	}
	Resp, err := gnoiClient.System().Reboot(context.Background(), &gnoisys.RebootRequest{
		Method:  rebootMethod,
		Delay:   0,
		Message: "Reboot chassis without delay",
		Force:   true,
	})
	if err != nil {
		t.Fatalf("Reboot failed %v", err)
	}
	t.Logf("Reload Response %v ", Resp)

	startReboot := time.Now()
	time.Sleep(5 * time.Second)
	const maxRebootTime = 30
	t.Logf("Wait for DUT to boot up by polling the telemetry output.")
	for {
		var currentTime string
		t.Logf("Time elapsed %.2f minutes since reboot started.", time.Since(startReboot).Minutes())

		time.Sleep(90 * time.Second)
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			currentTime = gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State())
		}); errMsg != nil {
			t.Logf("Got testt.CaptureFatal errMsg: %s, keep polling ...", *errMsg)
		} else {
			t.Logf("Device rebooted successfully with received time: %v", currentTime)
			break
		}

		if uint64(time.Since(startReboot).Minutes()) > maxRebootTime {
			t.Fatalf("Check boot time: got %v, want < %v", time.Since(startReboot), maxRebootTime)
		}
	}
	// gnmi.Await(t, dut, gnmi.OC().Component(dut.Device.Name()).OperStatus().State(), time.Minute*30, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE)

	t.Logf("Device boot time: %.2f minutes", time.Since(startReboot).Minutes())
	return nil
}

func reloadLineCardsWithRebootMethod(t *testing.T, dut *ondatra.DUTDevice, rebootMethod gnoisys.RebootMethod) error {
	gnoiClient := dut.RawAPIs().GNOI(t)
	lcs := components.FindComponentsByType(t, dut, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_LINECARD)

	wg := sync.WaitGroup{}

	relaunched := make([]string, 0)

	for _, lc := range lcs {
		t.Logf("Restarting LC %v\n", lc)
		if empty := gnmi.Get(t, dut, gnmi.OC().Component(lc).Empty().State()); empty {
			t.Logf("Linecard Component %s is empty, skipping", lc)
		}
		if removable := gnmi.Get(t, dut, gnmi.OC().Component(lc).Removable().State()); !removable {
			t.Logf("Linecard Component %s is non-removable, skipping", lc)
		}
		oper := gnmi.Get(t, dut, gnmi.OC().Component(lc).OperStatus().State())

		if got, want := oper, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE; got != want {
			t.Logf("Linecard Component %s is already INACTIVE, skipping", lc)
		}

		lineCardPath := components.GetSubcomponentPath(lc, false)

		resp, err := gnoiClient.System().Reboot(context.Background(), &gnoisys.RebootRequest{
			Method:  gnoisys.RebootMethod_COLD,
			Delay:   0,
			Message: "Reboot line card without delay",
			Subcomponents: []*gnoitype.Path{
				lineCardPath,
			},
			Force: true,
		})
		if err == nil {
			wg.Add(1)
			relaunched = append(relaunched, lc)
		} else {
			t.Fatalf("Reboot failed %v", err)
		}
		t.Logf("Reboot response: \n%v\n", resp)
	}

	// wait for all line cards to be back up
	for _, lc := range relaunched {
		go func(lc string) {
			defer wg.Done()
			timeout := time.Minute * 30
			t.Logf("Awaiting relaunch of linecard: %s", lc)
			oper := gnmi.Await[oc.E_PlatformTypes_COMPONENT_OPER_STATUS](
				t, dut,
				gnmi.OC().Component(lc).OperStatus().State(),
				timeout,
				oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE,
			)
			if val, ok := oper.Val(); !ok {
				t.Errorf("Reboot timed out, received status: %s", val)
				// check status if failed
			}
		}(lc)
	}

	wg.Wait()
	t.Log("All linecards successfully relaunched")

	return nil
}

func TestInitialBootTime(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	var controllerCard string

	controllerCards := components.FindComponentsByType(t, dut, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD)
	if len(controllerCards) > 0 {
		controllerCard = controllerCards[0]
	} else {
		t.Error("could not find controller card")
	}

	if !(gnmi.Lookup(t, dut, gnmi.OC().Component(controllerCard).LastRebootTime().State()).IsPresent()) {
		t.Error("value for LastRebootTime is not present")
	}

}

func TestRouterLastRebootTime(t *testing.T) {

	dut := ondatra.DUT(t, "dut")

	rebootMethods := []gnoisys.RebootMethod{
		gnoisys.RebootMethod_UNKNOWN,
		gnoisys.RebootMethod_COLD,
		gnoisys.RebootMethod_POWERDOWN,
		gnoisys.RebootMethod_HALT,
		gnoisys.RebootMethod_WARM,
		gnoisys.RebootMethod_NSF,
		gnoisys.RebootMethod_POWERUP,
	}

	var controllerCard string

	controllerCards := components.FindComponentsByType(t, dut, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD)
	if len(controllerCards) > 0 {
		controllerCard = controllerCards[0]
	} else {
		t.Error("could not find controller card")
	}

	//reload with each reboot method listed above
	for _, rebootMethod := range rebootMethods {
		t.Run(gnoisys.RebootMethod_name[int32(rebootMethod)], func(t *testing.T) {
			lastRebootTimeBefore := gnmi.Get(t, dut, gnmi.OC().Component(controllerCard).LastRebootTime().State())

			reloadRouterWithRebootMethod(t, dut, rebootMethod)

			time.Sleep(time.Minute * 3)

			lastRebootTimeAfter := gnmi.Get(t, dut, gnmi.OC().Component(controllerCard).LastRebootTime().State())

			if lastRebootTimeAfter < lastRebootTimeBefore {
				t.Errorf("LastRebootTime().Get(t): got %v, want > %v", lastRebootTimeAfter, lastRebootTimeBefore)
			}
		})
	}
}

func TestLCReloadLastRebootTime(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	rebootMethods := []gnoisys.RebootMethod{
		gnoisys.RebootMethod_UNKNOWN,
		gnoisys.RebootMethod_COLD,
		gnoisys.RebootMethod_POWERDOWN,
		gnoisys.RebootMethod_HALT,
		gnoisys.RebootMethod_WARM,
		gnoisys.RebootMethod_NSF,
		gnoisys.RebootMethod_POWERUP,
	}

	lcs := components.FindComponentsByType(t, dut, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_LINECARD)

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

			reloadLineCardsWithRebootMethod(t, dut, rebootMethod)

			time.Sleep(time.Minute * 3)

			for _, lc := range linecards {
				lastRebootTimeAfter := gnmi.Get(t, dut, gnmi.OC().Component(lc).LastRebootTime().State())
				if lastRebootTimeAfter < rebootTimes[lc] {
					t.Errorf("LastRebootTime().Get(t): got %v, want > %v", lastRebootTimeAfter, rebootTimes[lc])
				}
			}
		})
	}

}

func TestRPFOLastRebootTime(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	rps := components.FindComponentsByType(t, dut, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD)
	_, rpActive := components.FindStandbyControllerCard(t, dut, rps)
	t.Logf("RPs: %v", rps)

	lastRebootTimeBefore := gnmi.Get(t, dut, gnmi.OC().Component(rpActive).LastRebootTime().State())

	utils.Dorpfo(context.Background(), t, false)

	time.Sleep(time.Minute * 3)

	lastRebootTimeAfter := gnmi.Get(t, dut, gnmi.OC().Component(rpActive).LastRebootTime().State())

	if lastRebootTimeAfter < lastRebootTimeBefore {
		t.Errorf("LastRebootTime().Get(t): got %v, want > %v", lastRebootTimeAfter, lastRebootTimeBefore)
	}

}
