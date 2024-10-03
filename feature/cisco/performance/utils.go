package performance

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/cisco/config"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
)

func getProcessState(t *testing.T, dut *ondatra.DUTDevice, processName string) *ProcessState {
	// Fetch platform details
	cliCmd := "show platform"
	resp := config.CMDViaGNMI(context.Background(), t, dut, cliCmd)
	t.Logf("Platform response: %s", resp)

	var activeRP string

	lines := strings.Split(resp, "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		node, nodeType := fields[0], fields[1]

		// Check for standby RP and determine activeRP
		if strings.HasPrefix(node, "0/RP") || strings.HasPrefix(node, "0/RSP") {
			if strings.Contains(nodeType, "(Active)") {
				activeRP = node
			}
		}
	}
	t.Logf("Detected activeRP: %v", activeRP)

	timeout := time.Second * 90
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
	maxRetries := 3

	for stay, timeout := true, time.After(timeout); stay; {
		for i := 0; i < maxRetries; i++ {
			restartResp, err := dut.RawAPIs().GNMI(t).Get(context.Background(), req)
			if err == nil {
				select {
				case <-timeout:
					if err != nil {
						t.Errorf("Raw GNMI Query failed, timeout with response error: %s", err)
					}
					stay = false
				default:
					if err != nil {
						time.Sleep(time.Second * 10)
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
			} else {
				t.Logf("Raw GNMI Query failed, retrying %d/%d: %v", i+1, maxRetries, err)
				time.Sleep(time.Second * 10)
			}
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

// // Takes a GNMI path and retrieves all elements from the DUT
// func GetAllNativeModel(t testing.TB, dut *ondatra.DUTDevice, str string) (any, error) {
//
// 	split := strings.Split(str, ":")
// 	origin := split[0]
// 	paths := strings.Split(split[1], "/")
// 	pathelems := []*gnmipb.PathElem{}
// 	for _, path := range paths {
// 		pathelems = append(pathelems, &gnmipb.PathElem{Name: path})
// 	}
//
// 	req := &gnmipb.GetRequest{
// 		Path: []*gnmipb.Path{
// 			{
// 				Origin: origin,
// 				Elem:   pathelems,
// 			},
// 		},
// 		Type:     gnmipb.GetRequest_ALL,
// 		Encoding: gnmipb.Encoding_JSON_IETF,
// 	}
// 	var responseRawObj any
// 	restartResp, err := dut.RawAPIs().GNMI(t).Get(context.Background(), req)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed GNMI GET request on native model: \n%v", req)
// 	} else {
// 		jsonIetfData := restartResp.GetNotification()[0].GetUpdate()[0].GetVal().GetJsonIetfVal()
// 		err = json.Unmarshal(jsonIetfData, &responseRawObj)
// 		if err != nil {
// 			return nil, fmt.Errorf("could not unmarshal native model GET json")
// 		}
// 	}
// 	return responseRawObj, nil
// }
//
// type MemData struct {
// 	FreeMemory     uint32 `json:"free-memory,string"`
// 	MemoryState    string `json:"memory-state"`
// 	PhysicalMemory uint32 `json:"physical-memory"`
// }
//
// func DeserializeMemData(t testing.TB, dut *ondatra.DUTDevice) (*MemData, error) {
// 	req := &gnmipb.GetRequest{
// 		Path: []*gnmipb.Path{
// 			{
// 				Origin: "Cisco-IOS-XR-wd-oper", Elem: []*gnmipb.PathElem{
// 					{Name: "watchdog"},
// 					{Name: "nodes"},
// 					{Name: "node"},
// 					{Name: "memory-state"},
// 				},
// 			},
// 		},
// 		Type:     gnmipb.GetRequest_ALL,
// 		Encoding: gnmipb.Encoding_JSON_IETF,
// 	}
//
// 	var responseRawObj MemData
// 	restartResp, err := dut.RawAPIs().GNMI(t).Get(context.Background(), req)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed GNMI GET request on native model: \n%v", req)
// 	} else {
// 		jsonIetfData := restartResp.GetNotification()[0].GetUpdate()[0].GetVal().GetJsonIetfVal()
// 		err = json.Unmarshal(jsonIetfData, &responseRawObj)
// 		if err != nil {
// 			return nil, fmt.Errorf("could not unmarshal native model GET json")
// 		}
// 	}
// 	return &responseRawObj, nil
// }
//
// // Queries the OC GNMI model that returns all DUT process (top equivalent) data
// func TopCpuMemoryUtilOC(t *testing.T, dut *ondatra.DUTDevice) []*oc.System_Process {
// 	topArray := gnmi.GetAll(t, dut, gnmi.OC().System().ProcessAny().State())
// 	return topArray
// }
//
// // Queries the OC GNMI model that returns DUT process (top equivalent) data of a given PID
// func TopCpuMemoryFromPID(t *testing.T, dut *ondatra.DUTDevice, processName uint64) *oc.System_Process {
// 	topArray := gnmi.Get(t, dut, gnmi.OC().System().Process(processName).State())
// 	return topArray
// }
