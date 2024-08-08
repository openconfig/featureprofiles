package performance

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/featureprofiles/internal/components"
	gnoisys "github.com/openconfig/gnoi/system"
	gnoitype "github.com/openconfig/gnoi/types"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/testt"
)

type ProcessState struct {
	IsMaintenance  bool   `json:"is-maintenance"`
	IsMandatory    bool   `json:"is-mandatory"`
	InstanceId     uint64 `json:"instance-id"`
	Jid            uint64 `json:"jid"`
	Name           string `json:"name"`
	State          string `json:"state"`
	RespawnCount   uint64 `json:"respawn-count"`
	LastStarted    string `json:"last-started"`
	PlacementState string `json:"placement-state"`
}

func RestartProcess(t *testing.T, dut *ondatra.DUTDevice, processName string) error {

	psInit := getProcessState(t, dut, processName)

	if psInit == nil {
		t.Fatalf("Could not get process state info for \"%s\"", processName)
	}

	resp, err := dut.RawAPIs().GNOI(t).System().KillProcess(context.Background(), &gnoisys.KillProcessRequest{
		Name:    processName,
		Restart: true,
		Signal:  gnoisys.KillProcessRequest_SIGNAL_TERM,
	})
	if err != nil {
		return err
	}
	if resp == nil {
		t.Error("Response is nil")
	}

	psFinal := getProcessState(t, dut, processName)

	if psFinal.RespawnCount != psInit.RespawnCount+1 {
		t.Errorf("process %s respawn count increment failed: %d -> %d", processName, psInit.RespawnCount, psInit.RespawnCount)
	}

	t.Logf("Process State Response: %v", util.PrettyPrintJson(psFinal))

	return nil
}

func ReloadRouter(t *testing.T, dut *ondatra.DUTDevice) error {
	gnoiClient := dut.RawAPIs().GNOI(t)
	_, err := gnoiClient.System().Reboot(context.Background(), &gnoisys.RebootRequest{
		Method:  gnoisys.RebootMethod_COLD,
		Delay:   0,
		Message: "Reboot chassis without delay",
		Force:   true,
	})
	if err != nil {
		t.Fatalf("Reboot failed %v", err)
	}
	startReboot := time.Now()
	time.Sleep(5 * time.Second)
	const maxRebootTime = 30
	t.Logf("Wait for DUT to boot up by polling the telemetry output.")
	for {
		var currentTime string
		t.Logf("Time elapsed %.2f minutes since reboot started.", time.Since(startReboot).Minutes())

		time.Sleep(1 * time.Second)
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

func ReloadLineCards(t *testing.T, dut *ondatra.DUTDevice) error {
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

func GNMIBigSetRequest(t *testing.T, dut *ondatra.DUTDevice, set *gnmi.SetBatch, numLeaves int) error {
	BatchSet(t, dut, set, numLeaves)
	return nil
}
