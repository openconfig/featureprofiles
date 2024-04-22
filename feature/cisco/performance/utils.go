package performance

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
)

const (
	activeController  = oc.Platform_ComponentRedundantRole_PRIMARY
	standbyController = oc.Platform_ComponentRedundantRole_SECONDARY
)

func getProcessState(t *testing.T, dut *ondatra.DUTDevice, processName string) *ProcessState {

	activeRP := "0/RP0/CPU0"
	standbyRP := "0/RP1/CPU0"
	role := gnmi.Get(t, dut, gnmi.OC().Component(activeRP).RedundantRole().State())
	t.Logf("Component(%s).RedundantRole().Get(t): Role: %s", activeRP, role)

	switch role {
	case standbyController:
		standbyRP, activeRP = activeRP, standbyRP
	case activeController:
		// No need to change activeRP and standbyRP
	default:
		t.Fatalf("Expected controller to be active or standby, got %v", role)
	}
	t.Logf("Detected activeRP: %v, standbyRP: %v", activeRP, standbyRP)

	timeout := time.Second * 30
	req := &gnmipb.GetRequest{
		Path: []*gnmipb.Path{
			{
				Origin: "Cisco-IOS-XR-sysmgr-oper", Elem: []*gnmipb.PathElem{
					{Name: "system-process"},
					{Name: "node-table"},
					{Name: "node", Key: map[string]string{"node-name": activeRP}},
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
		t.Logf("Error: %v", err)
		t.Logf("Process Response: %v", restartResp)
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

func BatchSet(t *testing.T, dut *ondatra.DUTDevice, batchSet *gnmi.SetBatch, leavesCnt int) {
	startTime := time.Now()
	t.Logf("Started GNMI Replace for %d leaves at %s\n", leavesCnt, time.Now().String())
	resp := batchSet.Set(t, dut)
	t.Logf("Batch Set result: %v\n", resp)
	t.Logf("Finished GNMI Replace for %d leaves at %s, (%v)\n", leavesCnt, time.Now(), time.Since(startTime))
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
