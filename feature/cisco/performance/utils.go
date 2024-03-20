package performance

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
)

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
			t.Logf("emsd restart collector")
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

func GetAllNativeModel(t testing.TB, dut *ondatra.DUTDevice, str string) (any, error) {

	split := strings.Split(str, ":")
	origin := split[0]
	paths := strings.Split(split[1], "/")
	pathelems := []*gnmipb.PathElem{}
	for _, path := range paths {
		pathelems = append(pathelems, &gnmipb.PathElem{Name: path})
	}
	
	req := &gnmipb.GetRequest{
		Path: []*gnmipb.Path{
			{
				Origin: origin,
				Elem: pathelems,
			},
		},
		Type:     gnmipb.GetRequest_ALL,
		Encoding: gnmipb.Encoding_JSON_IETF,
	}
	var responseRawObj any
	restartResp, err := dut.RawAPIs().GNMI(t).Get(context.Background(), req)
	if err != nil {
		return nil, fmt.Errorf("Failed GNMI GET request on native model: \n%v\n", req)
	} else {
		jsonIetfData := restartResp.GetNotification()[0].GetUpdate()[0].GetVal().GetJsonIetfVal()
		err = json.Unmarshal(jsonIetfData, &responseRawObj)
		if err != nil {
			return nil, fmt.Errorf("Could not unmarshal native model GET json")
		}
	}
	return responseRawObj, nil
}
