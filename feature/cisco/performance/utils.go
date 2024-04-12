package performance

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
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

// Takes a GNMI path and retrieves all elements from the DUT
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
				Elem:   pathelems,
			},
		},
		Type:     gnmipb.GetRequest_ALL,
		Encoding: gnmipb.Encoding_JSON_IETF,
	}
	var responseRawObj any
	restartResp, err := dut.RawAPIs().GNMI(t).Get(context.Background(), req)
	if err != nil {
		return nil, fmt.Errorf("failed GNMI GET request on native model: \n%v", req)
	} else {
		jsonIetfData := restartResp.GetNotification()[0].GetUpdate()[0].GetVal().GetJsonIetfVal()
		err = json.Unmarshal(jsonIetfData, &responseRawObj)
		if err != nil {
			return nil, fmt.Errorf("could not unmarshal native model GET json")
		}
	}
	return responseRawObj, nil
}

type MemData struct {
	FreeMemory     uint32 `json:"free-memory,string"`
	MemoryState    string `json:"memory-state"`
	PhysicalMemory uint32 `json:"physical-memory"`
}

func DeserializeMemData(t testing.TB, dut *ondatra.DUTDevice) (*MemData, error) {
	req := &gnmipb.GetRequest{
		Path: []*gnmipb.Path{
			{
				Origin: "Cisco-IOS-XR-wd-oper", Elem: []*gnmipb.PathElem{
					{Name: "watchdog"},
					{Name: "nodes"},
					{Name: "node"},
					{Name: "memory-state"},
				},
			},
		},
		Type:     gnmipb.GetRequest_ALL,
		Encoding: gnmipb.Encoding_JSON_IETF,
	}

	var responseRawObj MemData
	restartResp, err := dut.RawAPIs().GNMI(t).Get(context.Background(), req)
	if err != nil {
		return nil, fmt.Errorf("failed GNMI GET request on native model: \n%v", req)
	} else {
		jsonIetfData := restartResp.GetNotification()[0].GetUpdate()[0].GetVal().GetJsonIetfVal()
		err = json.Unmarshal(jsonIetfData, &responseRawObj)
		if err != nil {
			return nil, fmt.Errorf("could not unmarshal native model GET json")
		}
	}
	return &responseRawObj, nil
}

// CLI Parser that runs the top linux command on the DUT
func TopCpuMemoryUtilization(t *testing.T, dut *ondatra.DUTDevice) (float64, float64, float64, float64, float64, error) {
	command := "run top -b | head -n 30"

	gnmiClient := dut.RawAPIs().CLI(t)
	cliOutput, err := gnmiClient.RunCommand(context.Background(), command)
	if err != nil {
		return 0, 0, 0, 0, 0, err
	}

	lines := strings.Split(cliOutput.Output(), "\n")
	cpuRe := regexp.MustCompile(`^\s*\d+\s+\w+\s+\d+\s+-?\d+\s+\S+\s+\S+\s+\S+\s+\S+\s+(\d+\.\d+)\s+(\d+\.\d+)`)
	memRe := regexp.MustCompile(`MiB Mem :\s+(\d+\.\d+) total,\s+(\d+\.\d+) free,\s+(\d+\.\d+) used`)

	var totalCpu, totalMemUsage, totalMem, freeMem, usedMem float64

	for _, line := range lines {
		t.Log(line)
		// Check for CPU and MEM usage
		if cpuMatches := cpuRe.FindStringSubmatch(line); len(cpuMatches) > 2 {
			cpuUsage, err := strconv.ParseFloat(cpuMatches[1], 64)
			if err != nil {
				continue
			}
			memUsage, err := strconv.ParseFloat(cpuMatches[2], 64)
			if err != nil {
				continue
			}
			totalCpu += cpuUsage
			totalMemUsage += memUsage
		}

		// Check for total, free, and used memory
		if memMatches := memRe.FindStringSubmatch(line); len(memMatches) > 3 {
			totalMem, err = strconv.ParseFloat(memMatches[1], 64)
			if err != nil {
				return 0, 0, 0, 0, 0, err
			}
			freeMem, err = strconv.ParseFloat(memMatches[2], 64)
			if err != nil {
				return 0, 0, 0, 0, 0, err
			}
			usedMem, err = strconv.ParseFloat(memMatches[3], 64)
			if err != nil {
				return 0, 0, 0, 0, 0, err
			}
		}
	}

	return totalCpu, totalMemUsage, totalMem, freeMem, usedMem, nil
}

type TopCpuMemData struct {
	TotalCPU      float64
	TotalMemUsage float64
	TotalMem      float64
	FreeMem       float64
	UsedMem       float64
}

type LineCardCpuMemData struct {
	SlotNum int
	TopCpuMemData
}

// CLI Parser that runs the top linux command on each line card in the DUT
func TopLineCardCpuMemoryUtilization(t *testing.T, dut *ondatra.DUTDevice) ([]LineCardCpuMemData, error) {
	gnmiClient := dut.RawAPIs().CLI(t)

	//Get nodes

	getCommand := "show platform"

	cliShowOutput, err := gnmiClient.RunCommand(context.Background(), getCommand)
	if err != nil {
		return nil, err
	}
	nodeLines := strings.Split(cliShowOutput.Output(), "\n")
	nodesRe := regexp.MustCompile(`^\d+\/(\d+)\/CPU\d+\s+\d+-LC-\w+\s+IOS XR RUN\s+\w+`)
	var nodeList []int

	for _, line := range nodeLines {
		t.Log(line)
		if nodeStr := nodesRe.FindStringSubmatch(line); len(nodeStr) > 1 {
			node, err := strconv.ParseFloat(nodeStr[1], 64)
			if err != nil {
				continue
			}
			nodeList = append(nodeList, int(node))
		}
	}

	if len(nodeList) < 1 {
		return nil, fmt.Errorf("no line cards found")
	}

	t.Logf("LC node list: %v", nodeList)

	var lineCardsTopData []LineCardCpuMemData
	commandFormat := "run ssh 172.0.%d.1 top -b | head -n 30"

	for _, node := range nodeList {
		t.Logf("Top of LC%d", node)

		command := fmt.Sprintf(commandFormat, node)
		t.Logf("Command running: %s", command)
		var totalCpu, totalMemUsage, totalMem, freeMem, usedMem float64
		cliOutput, err := gnmiClient.RunCommand(context.Background(), command)
		if err != nil {
			return nil, err
		}

		lines := strings.Split(cliOutput.Output(), "\n")
		cpuRe := regexp.MustCompile(`^\s*\d+\s+\w+\s+\d+\s+-?\d+\s+\S+\s+\S+\s+\S+\s+\S+\s+(\d+\.\d+)\s+(\d+\.\d+)`)
		memRe := regexp.MustCompile(`MiB Mem :\s+(\d+\.\d+) total,\s+(\d+\.\d+) free,\s+(\d+\.\d+) used`)

		for _, line := range lines {
			t.Log(line)
			// Check for CPU and MEM usage
			if cpuMatches := cpuRe.FindStringSubmatch(line); len(cpuMatches) > 2 {
				cpuUsage, err := strconv.ParseFloat(cpuMatches[1], 64)
				if err != nil {
					continue
				}
				memUsage, err := strconv.ParseFloat(cpuMatches[2], 64)
				if err != nil {
					continue
				}
				totalCpu += cpuUsage
				totalMemUsage += memUsage
			}

			// Check for total, free, and used memory
			if memMatches := memRe.FindStringSubmatch(line); len(memMatches) > 3 {
				totalMem, err = strconv.ParseFloat(memMatches[1], 64)
				if err != nil {
					return nil, err
				}
				freeMem, err = strconv.ParseFloat(memMatches[2], 64)
				if err != nil {
					return nil, err
				}
				usedMem, err = strconv.ParseFloat(memMatches[3], 64)
				if err != nil {
					return nil, err
				}
			}
		}

		lineCardsTopData = append(lineCardsTopData,
			LineCardCpuMemData{
				SlotNum: node,
				TopCpuMemData: TopCpuMemData{
					TotalCPU:      totalCpu,
					TotalMemUsage: totalMemUsage,
					TotalMem:      totalMem,
					FreeMem:       freeMem,
					UsedMem:       usedMem,
				},
			},
		)
	}
	//Get top data

	return lineCardsTopData, nil

}

// Queries the OC GNMI model that returns all DUT process (top equivalent) data
func TopCpuMemoryUtilOC(t *testing.T, dut *ondatra.DUTDevice) []*oc.System_Process {
	topArray := gnmi.GetAll(t, dut, gnmi.OC().System().ProcessAny().State())
	return topArray
}

// Queries the OC GNMI model that returns DUT process (top equivalent) data of a given PID
func TopCpuMemoryFromPID(t *testing.T, dut *ondatra.DUTDevice, processName uint64) *oc.System_Process {
	topArray := gnmi.Get(t, dut, gnmi.OC().System().Process(processName).State())
	return topArray
}
