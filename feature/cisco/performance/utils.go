package performance

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ytypes"
)

func PrettyPrint(i interface{}) string {
	s, _ := json.MarshalIndent(i, "", "\t")
	return string(s)
}

func BenchMark(ygnmiCli *ygnmi.Client) {
	ctx := context.Background()
	data, err := ygnmi.CollectAll(ctx, ygnmiCli, gnmi.OC().System().CpuAny().State()).Await()
	if err != nil {
		fmt.Printf("Error %v /n", err)
	}
	for _, memUse := range data {
		usedMem, _ := memUse.Val()
		fmt.Printf("Cpu info at %v : %v\n", memUse.Timestamp, PrettyPrint(usedMem))
	}
}

func ControlPlaneVerification(ygnmiCli *ygnmi.Client) {
	// TODO1:Check for Crash
	// TODO2: Check for Traces
	// TODO3: Check for Memory Usage
	BenchMark(ygnmiCli)
}

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
