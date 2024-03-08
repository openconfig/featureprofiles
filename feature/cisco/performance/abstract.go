// Abstract Trigger Space to have Routines that could be re-usable for any Test Suite
package main

import (
	"context"
	"encoding/json"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/components"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	gnoisys "github.com/openconfig/gnoi/system"
	gnoitype "github.com/openconfig/gnoi/types"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygot/ytypes"
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

func RestartEmsd(t *testing.T, dut *ondatra.DUTDevice) error {
	return RestartProcess(t, dut, "emsd")
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
		t.Error("")
	}

	psFinal := getProcessState(t, dut, processName)

	if psFinal.RespawnCount != psInit.RespawnCount+1 {
		t.Errorf("process %s respawn count increment failed: %d -> %d", processName, psInit.RespawnCount, psInit.RespawnCount)
	}

	t.Logf("Process State Response: %v", PrettyPrint(psFinal))

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
	const maxRebootTime = 30
	t.Logf("Wait for DUT to boot up by polling the telemetry output.")
	for {
		var currentTime string
		t.Logf("Time elapsed %.2f minutes since reboot started.", time.Since(startReboot).Minutes())

		time.Sleep(15 * time.Second)
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

// TODO: Not a trigger, move to utils
func getProcessState(t *testing.T, dut *ondatra.DUTDevice, processName string) *ProcessState {

	timeout := time.Second * 30
	req := &gnmipb.GetRequest{
		Path: []*gnmipb.Path{
			{
				Origin: "Cisco-IOS-XR-sysmgr-oper", Elem: []*gnmipb.PathElem{
					{Name: "system-process"},
					{Name: "node-table"},
					{Name: "node", Key: map[string]string{"node-name": "*"}},
					{Name: "processes"},
					{Name: "process", Key: map[string]string{"name": processName}},
				},
			},
		},
		Type:     gnmipb.GetRequest_STATE,
		Encoding: gnmipb.Encoding_JSON_IETF,
	}

	var responseRawObj ProcessState

	for stay, timeout := true, time.After(timeout); stay; {
		restartResp, err := dut.RawAPIs().GNMI(t).Get(context.Background(), req)
		select {
		case <-timeout:
			if err != nil {
				t.Errorf("Raw GNMI Query failed, timeout with response error: %s", err)
			}
			stay = false
		default:
			if err != nil {
				time.Sleep(time.Second)
				t.Logf("Raw GNMI Query failed, retrying")
				continue
			}
			jsonIetfData := restartResp.GetNotification()[0].GetUpdate()[0].GetVal().GetJsonIetfVal()
			err = json.Unmarshal(jsonIetfData, &responseRawObj)
			if err != nil {
				t.Errorf("Process %s state response serialization failed. Yang model may have non-backward compatible changes.", processName)
			}
			t.Logf("ProcessState %s response received: state: %s, respawn-count: %d", processName, responseRawObj.State, responseRawObj.RespawnCount)

			return &responseRawObj
		}
	}

	return nil
}

// TODO: Move these functions are they are not triggers

func BatchSet(t *testing.T, dut *ondatra.DUTDevice, batchSet *gnmi.SetBatch, leavesCnt int) {
	startTime := time.Now()
	t.Logf("Started GNMI Replace for %d leaves at %s\n", leavesCnt, time.Now().String())
	resp := batchSet.Set(t, dut)
	t.Logf("Batch Set result: %v\n", resp)
	t.Logf("Finished GNMI Replace for %d leaves at %s, (%v)\n", leavesCnt, time.Now(), time.Since(startTime))
}

// load oc from a file
func LoadJSONOC(t *testing.T, path string) *oc.Root {
	var ocRoot oc.Root
	jsonConfig, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Cannot load base config: %v", err)
	}
	opts := []ytypes.UnmarshalOpt{
		&ytypes.PreferShadowPath{},
	}
	if err := oc.Unmarshal(jsonConfig, &ocRoot, opts...); err != nil {
		t.Fatalf("Cannot unmarshal base config: %v", err)
	}
	return &ocRoot
}

func CreateInterfaceSetFromOCRoot(ocRoot *oc.Root, replace bool) *gnmi.SetBatch {
	batchRep := &gnmi.SetBatch{}
	for _, intf := range ocRoot.Interface {
		if replace {
			gnmi.BatchReplace(batchRep, gnmi.OC().Interface(intf.GetName()).Config(), intf)
		} else {
			gnmi.BatchUpdate(batchRep, gnmi.OC().Interface(intf.GetName()).Config(), intf)
		}
	}
	return batchRep
}
