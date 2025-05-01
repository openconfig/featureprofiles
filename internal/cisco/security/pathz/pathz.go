// Copyright 2023 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package authz provides helper APIs to simplify writing authz test cases.
// It also packs authz rotate and get operations with the corresponding verifications to
// prevent code duplications and increase the test code readability.
package pathz

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math"
	"net"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/cisco/config"
	"github.com/openconfig/featureprofiles/internal/cisco/util"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	pathzpb "github.com/openconfig/gnsi/pathz"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygot/ygot"
)

// Spiffe is an struct to save an Spiffe id and its svid.
type Spiffe struct {
	// ID store Spiffe id.
	ID string
	// TlsConf stores the svid of Spiffe id.
	TLSConf *tls.Config
}

var (
	record                   = flag.Bool("record", true, "record the query before gnmi set")
	recordPath               = flag.String("record_path", "testdata/set_config.json", "path where the gnmi set will be recorded")
	useGNMIReplace           = flag.Bool("use_replace", true, "use gnmi replace to push the config, by default gnmi update will be used.")
	maxLeavesCount           = flag.Int("max_leaves_count", 100000, "the max number of  config leaves to be be generated, the script will break the generation when the config leaves exceed the max value")
	subinterfacePerInterface = flag.Int("subinterface_per_interface", 100, "the number of subinterface to be created per interface, note there is a limit for the number of subinterface per linecard")
	ipPerSubinterface        = flag.Int("ip_per_subinterface", 100, "the number of ip will be created per subinterface")
)

type EMSDPid struct {
	EmsdPid uint64 `json:"process-id"`
}

func prettyPrint(i interface{}) string {
	s, _ := json.MarshalIndent(i, "", "\t")
	return string(s)
}

const (
	ActionDeny        = pathzpb.Action_ACTION_DENY
	ActionPermit      = pathzpb.Action_ACTION_PERMIT
	ModeRead          = pathzpb.Mode_MODE_READ
	activeController  = oc.Platform_ComponentRedundantRole_PRIMARY
	standbyController = oc.Platform_ComponentRedundantRole_SECONDARY
)

func ConfigAndVerifyISIS(t testing.TB, d *ondatra.DUTDevice, i string, ni string, si uint32) {
	t.Helper()
	if ni == "" {
		t.Fatalf("Network instance not provided for interface assignment")
	}
	netInst := &oc.NetworkInstance{Name: ygot.String(ni)}
	intf := &oc.Interface{Name: ygot.String(i)}
	netInstIntf, err := netInst.NewInterface(intf.GetName())
	if err != nil {
		t.Errorf("Error fetching NewInterface for %s", intf.GetName())
	}
	netInstIntf.Interface = ygot.String(intf.GetName())
	netInstIntf.Subinterface = ygot.Uint32(si)
	netInstIntf.Id = ygot.String(intf.GetName() + "." + fmt.Sprint(si))
	if intf.GetOrCreateSubinterface(si) != nil {
		gnmi.Update(t, d, gnmi.OC().NetworkInstance(ni).Config(), netInst)
	}
}

func countLeaves(v any) int {
	return rvCountFields(reflect.ValueOf(v))
}

func rvCountFields(rv reflect.Value) (count int) {
	if rv.Kind() != reflect.Struct {
		return
	}

	fs := rv.NumField()
	count += fs
	for i := 0; i < fs; i++ {
		f := rv.Field(i)
		if f.Kind() == reflect.Ptr {
			if f.IsNil() {
				count--
				continue
			}
			if f.Elem().Kind() == reflect.Struct {
				// do not count the containers
				count--
				count += rvCountFields(f.Elem())
			}
		}
		if f.Kind() == reflect.Map {
			for _, e := range f.MapKeys() {
				v := f.MapIndex(e)
				// do not account the containers
				count--
				if v.Kind() == reflect.Ptr {
					count += rvCountFields(v.Elem())
				}
			}
		}
	}
	return
}

func EmsdMemoryCheck(t *testing.T, dut *ondatra.DUTDevice) uint64 {

	var responseRawObj EMSDPid
	pidreq := &gpb.GetRequest{
		Path: []*gpb.Path{
			{
				Origin: "Cisco-IOS-XR-sysmgr-oper", Elem: []*gpb.PathElem{
					{Name: "system-process"},
					{Name: "node-table"},
					{Name: "node", Key: map[string]string{"node-name": "*"}},
					{Name: "name"},
					{Name: "process-name-infos"},
					{Name: "process-name-info", Key: map[string]string{"proc-name": "emsd"}},
					{Name: "proc-basic-info-val"},
				},
			},
		},
		Type:     gpb.GetRequest_STATE,
		Encoding: gpb.Encoding_JSON_IETF,
	}

	emsdPid, err := dut.RawAPIs().GNMI(t).Get(context.Background(), pidreq)
	if err != nil {
		t.Logf("Error: %v", err)
	}
	t.Logf("Process Response: %v", emsdPid)

	jsonIetfData := emsdPid.GetNotification()[0].GetUpdate()[0].GetVal().GetJsonIetfVal()
	err = json.Unmarshal(jsonIetfData, &responseRawObj)
	if err != nil {
		t.Errorf("Process emsd pid state response serialization failed. Yang model may have non-backward compatible changes.")
	}
	t.Logf("Process emsd PID value: %v", responseRawObj.EmsdPid)

	cliCmd := fmt.Sprintf("show processes memory %v", responseRawObj.EmsdPid)
	resp := config.CMDViaGNMI(context.Background(), t, dut, cliCmd)
	t.Log(resp)

	var dynamicMemory uint64
	// Split the response into lines
	lines := strings.Split(resp, "\n")
	// Iterate over each line to find the one containing "emsd"
	for _, line := range lines {
		if strings.Contains(line, "emsd") {
			// Split the line into fields
			fields := strings.Fields(line)
			if len(fields) > 4 {
				// Assuming the 5th field is Dynamic(KB)
				fmt.Sscanf(fields[4], "%d", &dynamicMemory)
			}
		}
	}
	return dynamicMemory
}

func CheckPlatformStatus(t *testing.T, dut *ondatra.DUTDevice) error {
	maxRetries := 10                   // Maximum number of retries
	retryInterval := 120 * time.Second // Interval between retries

	cliCmd := "show platform"

	checkCPU0State := func(output string) error {
		lines := strings.Split(output, "\n")
		for _, line := range lines {
			// Check if the line contains CPU0 and extract its state
			if strings.Contains(line, "CPU0") {
				fields := strings.Fields(line)
				if len(fields) < 5 {
					return errors.New("unexpected output format")
				}
				state := fields[2] + " " + fields[3] + " " + fields[4] // Correct field for State
				t.Logf("CPU0 state: %s", state)
				if state != "IOS XR RUN" {
					return fmt.Errorf("%s not in 'IOS XR RUN' state: %s", line, state)
				}
			}
		}
		return nil
	}

	var err error
	for i := 0; i < maxRetries; i++ {
		resp := config.CMDViaGNMI(context.Background(), t, dut, cliCmd)
		t.Log(resp)

		err = checkCPU0State(resp)
		if err == nil {
			t.Logf("All CPU0 entries are in 'IOS XR RUN' state.")
			return nil
		}
		t.Logf("Retry %d/%d failed: %v\n", i+1, maxRetries, err)
		time.Sleep(retryInterval)
	}
	return fmt.Errorf("all retries failed: %v", err)
}

func CleanUPInterface(t *testing.T, dut *ondatra.DUTDevice) {
	// ocRoot is the key to create all oc struct
	ocRoot := &oc.Root{}

	// read interface config save them in json file for edit and push
	interfaces := gnmi.GetAll(t, dut, gnmi.OC().InterfaceAny().State())
	ocRoot.Interface = make(map[string]*oc.Interface)
	for _, intf := range interfaces {
		ygot.PruneConfigFalse(oc.SchemaTree["Interface"], intf)
		if intf.GetName() == "Null0" || strings.HasPrefix(intf.GetName(), "Loopback") ||
			strings.HasPrefix(intf.GetName(), "PTP") || strings.HasPrefix(intf.GetName(), "MgmtEth") { // skip nul
			continue
		}
		intf.ForwardingViable = nil
		intf.Mtu = nil
		intf.HoldTime = nil
		if intf.Subinterface != nil {
			intf.Subinterface = nil
		}
		if intf.Ethernet != nil {
			intf.Ethernet = nil
		}
		ocRoot.Interface[intf.GetName()] = intf
	}

	// cleanup interfaces
	fmt.Println("Cleaning subinterface configs")
	leavesCnt := 0
	batchSet := &gnmi.SetBatch{}
	for _, intf := range ocRoot.Interface {
		gnmi.BatchReplace(batchSet, gnmi.OC().Interface(intf.GetName()).Config(), intf)
		leavesCnt += countLeaves(*intf)
	}

	startTime := time.Now()
	fmt.Printf("Started GNMI Replace for %d leaves at %s\n", leavesCnt, time.Now().String())
	batchSet.Set(t, dut)
	fmt.Printf("Finished GNMI Replace for %d leaves at %s, (%v)\n", leavesCnt, time.Now(), time.Since(startTime))
}

func GenerateSubInterfaceConfig(t *testing.T, dut *ondatra.DUTDevice) (*gnmi.SetBatch, int) {
	ocRoot := &oc.Root{}
	interfaces := gnmi.GetAll(t, dut, gnmi.OC().InterfaceAny().State())
	ocRoot.Interface = make(map[string]*oc.Interface)
	for _, intf := range interfaces {
		ygot.PruneConfigFalse(oc.SchemaTree["Interface"], intf)
		if intf.GetName() == "Null0" || strings.HasPrefix(intf.GetName(), "Loopback") ||
			strings.HasPrefix(intf.GetName(), "PTP") || strings.HasPrefix(intf.GetName(), "MgmtEth") { // skip nul
			continue
		}
		intf.ForwardingViable = nil
		intf.Mtu = nil
		intf.HoldTime = nil
		if intf.Subinterface != nil {
			intf.Subinterface = nil
		}
		if intf.Ethernet != nil {
			intf.Ethernet = nil
		}
		ocRoot.Interface[intf.GetName()] = intf
	}

	maxSubInterface := 3500 / len(ocRoot.Interface)
	if *subinterfacePerInterface >= maxSubInterface {
		*subinterfacePerInterface = maxSubInterface
	}

	// Create a set batch to store configuration changes
	batchSet := &gnmi.SetBatch{}
	ip := net.IPv4(192, 168, 1, 1)
	// Count the total number of leaves added
	leavesCount := 0

	for _, intf := range ocRoot.Interface {
		intf.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd

		// Iterate over subinterfaces
		for i := 2; i <= maxSubInterface; i++ {
			subintf := intf.GetOrCreateSubinterface(uint32(i))
			subintf.Enabled = ygot.Bool(false)
			subintf.Description = ygot.String("test_")

			// Iterate over IP addresses per subinterface
			for j := 0; j <= *ipPerSubinterface; j++ {
				ipv4 := subintf.GetOrCreateIpv4()
				add := ipv4.GetOrCreateAddress(ip.String())
				add.PrefixLength = ygot.Uint8(24)

				// Set type based on whether it's the first IP address
				if j == 0 {
					add.Type = oc.IfIp_Ipv4AddressType_PRIMARY
				} else {
					add.Type = oc.IfIp_Ipv4AddressType_SECONDARY
				}

				// Move to the next IP address
				ip = nextIP(ip, 1)
			}

			// Break if we've reached the maximum number of leaves
			if leavesCount >= *maxLeavesCount {
				break
			}
		}

		// Add configuration changes to the batch
		if *useGNMIReplace {
			gnmi.BatchReplace(batchSet, gnmi.OC().Interface(intf.GetName()).Config(), intf)
		} else {
			gnmi.BatchUpdate(batchSet, gnmi.OC().Interface(intf.GetName()).Config(), intf)
		}

		// Increment the leaves count
		leavesCount += countLeaves(*intf)

		// Break if we've reached the maximum number of leaves
		if leavesCount >= *maxLeavesCount {
			break
		}
	}
	fmt.Printf("BatchSet %v\n", batchSet)
	fmt.Printf("LeavesCount %v\n", leavesCount)

	if *record {
		fmt.Println("Recording the gnmi set")
		saveOCJSON(ocRoot, *recordPath)
		fmt.Printf("File %v :", prettyPrint(recordPath))
	}

	return batchSet, leavesCount
}

func nextIP(ip net.IP, inc uint) net.IP {
	i := ip.To4()
	v := uint(i[0])<<24 + uint(i[1])<<16 + uint(i[2])<<8 + uint(i[3])
	v += inc
	v3 := byte(v & 0xFF)
	v2 := byte((v >> 8) & 0xFF)
	v1 := byte((v >> 16) & 0xFF)
	v0 := byte((v >> 24) & 0xFF)
	for {
		if v3 != 0 && v3 != 255 {
			break
		}
		v += inc
		v3 = byte(v & 0xFF)
		v2 = byte((v >> 8) & 0xFF)
		v1 = byte((v >> 16) & 0xFF)
		v0 = byte((v >> 24) & 0xFF)
	}

	//return net.IPv4(v0, v1, v2, v3).String()
	return net.IPv4(v0, v1, v2, v3)
}

// load oc from a file
func saveOCJSON(val *oc.Root, path string) {
	var jsonConfig []byte
	marshalCFG := ygot.RFC7951JSONConfig{
		AppendModuleName:             true,
		PrependModuleNameIdentityref: true,
		PreferShadowPath:             true,
	}
	marshalCFG.AppendModuleName = true
	mapVal, err := ygot.ConstructIETFJSON(val, &marshalCFG)
	if err != nil {
		panic(fmt.Sprintf("Cannot Construct IETF JSON from oc struct : %v\n", err))
	}
	jsonConfig, err = json.MarshalIndent(mapVal, "", "	")
	if err != nil {
		panic(fmt.Sprintf("Cannot marshal oc struct in to json : %v", err))
	}
	err = os.WriteFile(path, jsonConfig, 0644)
	if err != nil {
		panic(fmt.Sprintf("Cannot write to file %s : %v", path, err))
	}
}

// Delete Authz policy file.
func DeletePolicyData(t *testing.T, dut *ondatra.DUTDevice, file string) {
	time.Sleep(5 * time.Second)
	cliHandle := dut.RawAPIs().CLI(t)
	resp, err := cliHandle.RunCommand(context.Background(), "run rm /mnt/rdsfs/ems/gnsi/"+file)
	time.Sleep(10 * time.Second)
	if err != nil {
		t.Error(err)
	}
	t.Logf("delete authz/pathz policy file  %v, %s", resp, file)
}

func VerifyPolicyInfo(t *testing.T, dut *ondatra.DUTDevice, expectedTimestamp uint64, expectedVersion string, emptyPolicy bool) {
	// Retrieve timestamp from the device
	timestamp := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").GnmiPathzPolicyCreatedOn().State())
	t.Logf("Got the expected Policy timestamp: %v", timestamp)

	// Verify timestamp
	if timestamp != expectedTimestamp {
		t.Fatalf("Unexpected value for Policy timestamp. Expected: %d, Got: %d", expectedTimestamp, timestamp)
	}

	// Verify version if expectedVersion is provided
	if expectedVersion != "" {
		version := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").GnmiPathzPolicyVersion().State())
		t.Logf("Got the expected Policy version: %s", version)

		if version != expectedVersion {
			t.Fatalf("Unexpected value for Policy version. Expected: %s, Got: %s", expectedVersion, version)
		}
	} else {
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").GnmiPathzPolicyVersion().State())
		}); errMsg != nil {
			t.Logf("Got testt.CaptureFatal errMsg: %s, ", *errMsg)
		} else {
			t.Fatalf("This gNMI GET operation should have failed, expecting empty gnmi get response")
		}
	}

	if emptyPolicy {
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").GnmiPathzPolicyCounters().State())
		}); errMsg != nil {
			t.Logf("Got testt.CaptureFatal errMsg: %s, ", *errMsg)
		} else {
			t.Fatalf("This gNMI GET operation should have failed, expecting empty gnmi get response")
		}
	}
}

func VerifyReadPolicyCounters(t *testing.T, dut *ondatra.DUTDevice, path string, isLastReject, isLastAccept bool, readRejects, readAccepts int) {
	// Get policy counters
	counters := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").GnmiPathzPolicyCounters().Path(path).State())
	t.Logf("Received response for path %q: %v", path, counters)

	// Verify read counters
	if counters.Reads == nil {
		t.Fatalf("Received nil value for read counters at path %q", path)
	}

	if counters.Reads.GetAccessAccepts() == uint64(readAccepts) {
		t.Logf("Policy read accept counters match expected value for path %q", path)
	} else {
		t.Fatalf("Unexpected value for Policy read accept counter at path %q. Expected: %d, Got: %d", path, readAccepts, counters.Reads.GetAccessAccepts())
	}
	if isLastAccept {
		if counters.Reads.GetLastAccessAccept() > uint64(0) {
			t.Logf("Last read accept time for path %q: %d", path, counters.Reads.GetLastAccessAccept())
		} else {
			t.Fatalf("Unexpected value for last read accept time at path %q. Got: %d", path, counters.Reads.GetLastAccessAccept())
		}
	} else {
		if counters.Reads.GetLastAccessAccept() == uint64(0) {
			t.Logf("Last read accept time for path %q: %d", path, counters.Reads.GetLastAccessAccept())
		} else {
			t.Fatalf("Unexpected value for last read accept time at path %q. Got: %d", path, counters.Reads.GetLastAccessAccept())

		}
	}
	if counters.Reads.GetAccessRejects() == uint64(readRejects) {
		t.Logf("Policy read reject counters match expected value for path %q", path)
	} else {
		t.Fatalf("Unexpected value for Policy read reject counter at path %q. Expected: %d, Got: %d", path, readRejects, counters.Reads.GetAccessRejects())
	}

	if isLastReject {
		if counters.Reads.GetLastAccessReject() > uint64(0) {
			t.Logf("Last read reject time for path %q: %d", path, counters.Reads.GetLastAccessReject())
		} else {
			t.Fatalf("Unexpected value for last read reject time at path %q. Got: %d", path, counters.Reads.GetLastAccessReject())
		}

	} else {
		if counters.Reads.GetLastAccessReject() == uint64(0) {
			t.Logf("Last read reject time for path %q: %d", path, counters.Reads.GetLastAccessReject())
		} else {
			t.Fatalf("Unexpected value for last read reject time at path %q. Got: %d", path, counters.Reads.GetLastAccessReject())
		}
	}

}

func VerifyWritePolicyCounters(t *testing.T, dut *ondatra.DUTDevice, path string, isLastReject, isLastAccept bool, writeRejects, writeAccepts int) {
	// Get policy counters
	counters := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").GnmiPathzPolicyCounters().Path(path).State())
	t.Logf("Received response for path %q: %v", path, counters)

	// Verify write counters
	if counters.Writes == nil {
		t.Fatalf("Received nil value for write counters at path %q", path)
	}

	if counters.Writes.GetAccessAccepts() != uint64(writeAccepts) {
		t.Fatalf("Unexpected value for Policy write accept counter at path %q. Expected: %d, Got: %d", path, writeAccepts, counters.Writes.GetAccessAccepts())
	} else {
		t.Logf("Policy write accept counters match expected value for path %q", path)
	}

	if isLastAccept {
		if counters.Writes.GetLastAccessAccept() > uint64(0) {
			t.Logf("Last write accept time for path %q: %d", path, counters.Writes.GetLastAccessAccept())
		} else {
			t.Fatalf("Unexpected value for last write accept time at path %q. Got: %d", path, counters.Writes.GetLastAccessAccept())
		}
	} else {
		if counters.Writes.GetLastAccessAccept() == uint64(0) {
			t.Logf("Last write accept time for path %q: %d", path, counters.Writes.GetLastAccessAccept())
		} else {
			t.Fatalf("Unexpected value for last write accept time at path %q. Got: %d", path, counters.Writes.GetLastAccessAccept())
		}
	}

	if counters.Writes.GetAccessRejects() != uint64(writeRejects) {
		t.Fatalf("Unexpected value for Policy write reject counter at path %q. Expected: %d, Got: %d", path, writeRejects, counters.Writes.GetAccessRejects())
	} else {
		t.Logf("Policy write reject counters match expected value for path %q", path)
	}

	if isLastReject {
		if counters.Writes.GetLastAccessReject() > uint64(0) {
			t.Logf("Last write reject time for path %q: %d", path, counters.Writes.GetLastAccessReject())
		} else {
			t.Fatalf("Unexpected value for last write reject time at path %q. Got: %d", path, counters.Writes.GetLastAccessReject())
		}
	} else {
		if counters.Writes.GetLastAccessReject() == uint64(0) {
			t.Logf("Last write reject time for path %q: %d", path, counters.Writes.GetLastAccessReject())
		} else {
			t.Fatalf("Unexpected value for last write reject time at path %q. Got: %d", path, counters.Writes.GetLastAccessReject())
		}
	}
}

type MemVerifier struct {
	freeMemoryAvgBefore int
	usedMemoryAvgBefore int
	freeMemoryAvgAfter  int
	usedMemoryAvgAfter  int
	memoryStateAfter    string
}

func NewVerifier() *MemVerifier {
	return &MemVerifier{}
}

func (v *MemVerifier) SampleBefore(t *testing.T, dut *ondatra.DUTDevice) {
	c := CollectMemData(t, dut, time.Second, 5*time.Second)
	c.Wait()
	totalFree := 0
	totalUsed := 0
	for _, mem := range c.MemLogs {
		totalFree += int(mem.FreeMemory)
		totalUsed += int(mem.PhysicalMemory - mem.FreeMemory)
	}
	// Integer floor divison
	// susceptible to skew from missing data
	v.freeMemoryAvgBefore = totalFree / (len(c.MemLogs))
	v.usedMemoryAvgBefore = totalUsed / (len(c.MemLogs))
}

func (v *MemVerifier) SampleAfter(t *testing.T, dut *ondatra.DUTDevice) {
	c := CollectMemData(t, dut, time.Second, 5*time.Second)
	c.Wait()
	totalFree := 0
	totalUsed := 0
	for _, mem := range c.MemLogs {
		totalFree += int(mem.FreeMemory)
		totalUsed += int(mem.PhysicalMemory - mem.FreeMemory)
		v.memoryStateAfter = mem.MemoryState
	}
	// Integer floor divison
	// susceptible to skew from missing data
	v.freeMemoryAvgAfter = totalFree / (len(c.MemLogs))
	v.usedMemoryAvgAfter = totalUsed / (len(c.MemLogs))
}

func (v *MemVerifier) Verify(t *testing.T) bool {
	percentDiff := func(before, after int) float64 {
		if after > before {
			//1.25
			return float64(after)/float64(before) - 1
		} else {
			//0.75
			return (1 - float64(after)/float64(before)) * -1
		}
	}

	diffFreeMem := percentDiff(v.freeMemoryAvgBefore, v.freeMemoryAvgAfter)
	diffUsedMem := percentDiff(v.usedMemoryAvgBefore, v.usedMemoryAvgAfter)

	t.Logf("Free memory avg\nbefore:\t%d\nafter:\t%d\ndelta:\t%+.2f%%\n", v.freeMemoryAvgBefore, v.freeMemoryAvgAfter, math.Round(diffFreeMem*10000)/100)
	t.Logf("Used memory avg\nbefore:\t%d\nafter:\t%d\ndelta:\t%+.2f%%\n", v.usedMemoryAvgBefore, v.usedMemoryAvgAfter, math.Round(diffUsedMem*10000)/100)
	t.Logf("Memory state: %s", v.memoryStateAfter)

	if math.Abs(diffFreeMem) > 0.25 || math.Abs(diffUsedMem) > 0.25 || v.memoryStateAfter != "normal" {
		return false
	}
	return true
}

type MemData struct {
	FreeMemory     uint32 `json:"free-memory,string"`
	MemoryState    string `json:"memory-state"`
	PhysicalMemory uint32 `json:"physical-memory"`
}
type Collector struct {
	sync.WaitGroup
	CpuLogs [][]*oc.System_Cpu
	// MemLogs []*oc.System_Memory
	MemLogs []MemData
}

func CollectMemData(t *testing.T, dut *ondatra.DUTDevice, frequency time.Duration, duration time.Duration) *Collector {
	t.Helper()
	collector := &Collector{
		MemLogs: make([]MemData, 0),
	}
	collector.Add(1)
	go receiveMemData(t, getMemData(t, dut, frequency, duration), collector)
	return collector
}

func receiveMemData(t *testing.T, memChan chan MemData, collector *Collector) {
	t.Helper()
	defer collector.Done()
	for memData := range memChan {
		// TODO: change from log to capture
		t.Logf("\nMemory INFO:, t: %s\n%s\n", time.Now(), util.PrettyPrintJson(memData))
		collector.MemLogs = append(collector.MemLogs, memData)
	}
}

func getMemData(t *testing.T, dut *ondatra.DUTDevice, freq time.Duration, dur time.Duration) chan MemData {
	// oc leaves for memory do not work!! and cpu information require extra analysis, commenting this code for now
	t.Helper()
	// memChan := make(chan *oc.System_Memory, 100)
	memChan := make(chan MemData, 100)

	go func() {
		ticker := time.NewTicker(freq)
		timer := time.NewTimer(dur)
		done := false
		defer close(memChan)
		for !done {
			select {
			case <-ticker.C:
				if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
					// Cisco-IOS-XR-wd-oper:watchdog/nodes/node/memory-state
					// var data MemData
					// data, err := GetAllNativeModel(t, dut, "Cisco-IOS-XR-wd-oper:watchdog/nodes/node/memory-state")
					data, err := DeserializeMemData(t, dut)
					if err != nil {
						t.Logf("Memory collector failed: %s", err)
					}

					// Cisco-IOS-XR-procmem-oper:processes-memory/nodes/node/process-ids/process-id
					// nativeModelObj2, err := GetAllNativeModel(t, dut, "Cisco-IOS-XR-procmem-oper:processes-memory/nodes/node/process-ids/process-id")
					// if err != nil {
					// 	t.Logf("Memory collector failed: %s", err)
					// } else {
					// 	t.Logf("Mem Data: \n %s\n", util.PrettyPrintJson(nativeModelObj2))
					// }
					memChan <- *data
				}); errMsg != nil {
					t.Logf("Memory collector failed: %s", *errMsg)
					continue
				}
			case <-timer.C:
				done = true
			}
		}
	}()

	return memChan
}

func DeserializeMemData(t testing.TB, dut *ondatra.DUTDevice) (*MemData, error) {
	req := &gpb.GetRequest{
		Path: []*gpb.Path{
			{
				Origin: "Cisco-IOS-XR-wd-oper", Elem: []*gpb.PathElem{
					{Name: "watchdog"},
					{Name: "nodes"},
					{Name: "node"},
					{Name: "memory-state"},
				},
			},
		},
		Type:     gpb.GetRequest_ALL,
		Encoding: gpb.Encoding_JSON_IETF,
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

type PathElem struct {
	Name string
	Key  map[string]string
}

type Path struct {
	Origin string
	Elem   []*PathElem
}

type AuthorizationRule struct {
	Id        string
	Path      *Path
	Principal *pathzpb.AuthorizationRule_User
	Mode      pathzpb.Mode
	Action    pathzpb.Action
}

type Group struct {
	Name  string
	Users []*pathzpb.User
}

type AuthorizationPolicy struct {
	Groups []*Group
	Rules  []*AuthorizationRule
}

type UploadRequest struct {
	Version   string
	CreatedOn uint64
	Policy    *AuthorizationPolicy
}

type RotateRequest struct {
	RotateRequest *pathzpb.RotateRequest_UploadRequest
}

func GenerateRules(filePath string, origin string, user string, createdTime uint64) *pathzpb.RotateRequest {
	ruleNo := 1
	var rules []*pathzpb.AuthorizationRule

	// Read paths from file
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Println("Error reading paths from file:", err)
		return nil
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		path := scanner.Text()
		var elems []*gpb.PathElem

		if ruleNo == 5803 {
			// Special case for rule number 5803
			elems = []*gpb.PathElem{
				{Name: "interfaces"},
				{Name: "interface", Key: map[string]string{"name": "*"}},
			}
		} else {
			// Regular path processing
			pathElems := strings.Split(path, "/")
			for _, elem := range pathElems {
				// Trim quotes and commas if it's the last element
				elem = strings.Trim(elem, `",`)
				elems = append(elems, &gpb.PathElem{Name: elem})
			}
		}

		var action pathzpb.Action
		if ruleNo <= 2400 {
			action = pathzpb.Action_ACTION_DENY
		} else {
			action = pathzpb.Action_ACTION_PERMIT
		}

		var mode pathzpb.Mode
		var principal *pathzpb.AuthorizationRule_User
		if ruleNo == 5803 {
			mode = pathzpb.Mode_MODE_WRITE
			principal = &pathzpb.AuthorizationRule_User{User: user}
		} else {
			mode = pathzpb.Mode_MODE_READ
			principal = &pathzpb.AuthorizationRule_User{User: user + strconv.Itoa(ruleNo)}
		}

		rule := &pathzpb.AuthorizationRule{
			Id:        "Rule" + strconv.Itoa(ruleNo),
			Path:      &gpb.Path{Origin: origin, Elem: elems},
			Principal: principal,
			Mode:      mode,
			Action:    action,
		}

		rules = append(rules, rule)
		ruleNo++
	}

	if err := scanner.Err(); err != nil {
		fmt.Println("Error scanning file:", err)
		return nil
	}

	req := &pathzpb.RotateRequest{
		RotateRequest: &pathzpb.RotateRequest_UploadRequest{
			UploadRequest: &pathzpb.UploadRequest{
				Version:   "5800-Rules",
				CreatedOn: createdTime,
				Policy: &pathzpb.AuthorizationPolicy{
					Groups: []*pathzpb.Group{
						{Name: "pathz", Users: []*pathzpb.User{{Name: user}}},
						{Name: "admin", Users: []*pathzpb.User{{Name: user}}},
					},
					Rules: rules,
				},
			},
		},
	}
	return req
}

// GnsiPathAuthStats represents the parsed statistics from the output.
type GnsiPathAuthStats struct {
	RotationsInProgressCount int
	PolicyRotations          int
	PolicyRotationErrors     int
	PolicyUploadRequests     int
	PolicyUploadErrors       int
	PolicyFinalize           int
	PolicyFinalizeErrors     int
	ProbeRequests            int
	ProbeErrors              int
	GetRequests              int
	GetErrors                int
	PolicyUnmarshallErrors   int
	SandboxPolicyErrors      int
	NoPolicyAuthRequests     int
	GnmiPathLeaves           int
	GnmiAuthorizations       int
	GnmiSetPathPermit        int
	GnmiSetPathDeny          int
	GnmiGetPathPermit        int
	GnmiGetPathDeny          int
	PathToStringErrors       int
	OriginTypeErrors         int
	BadModeErrors            int
	BadActionErrors          int
	JsonFlattenErrors        int
	StringToPathErrors       int
	JoinPathsErrors          int
	NilPathErrors            int
	NilSetRequestErrors      int
	EmptyUserErrors          int
	ProbeInternalErrors      int
	PathCountersIncrement    int
	PathCountersFind         int
	PathCountersClear        int
	PathCountersWalk         int
}

// parseGnsiPathAuthStats parses the given output string into GnsiPathAuthStats.
func parseGnsiPathAuthStats(output string) (*GnsiPathAuthStats, error) {
	stats := &GnsiPathAuthStats{}
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		keyValue := strings.Split(line, ":")
		if len(keyValue) != 2 {
			continue
		}

		key := strings.TrimSpace(keyValue[0])
		valueStr := strings.TrimSpace(keyValue[1])
		value, err := strconv.Atoi(valueStr)
		if err != nil {
			value = 0 // Default to zero for non-integer values
		}

		switch key {
		case "Rotations in Progress Count":
			stats.RotationsInProgressCount = value
		case "Policy Rotations":
			stats.PolicyRotations = value
		case "Policy Rotation Errors":
			stats.PolicyRotationErrors = value
		case "Policy Upload Requests":
			stats.PolicyUploadRequests = value
		case "Policy Upload Errors":
			stats.PolicyUploadErrors = value
		case "Policy Finalize":
			stats.PolicyFinalize = value
		case "Policy Finalize Errors":
			stats.PolicyFinalizeErrors = value
		case "Probe Requests":
			stats.ProbeRequests = value
		case "Probe Errors":
			stats.ProbeErrors = value
		case "Get Requests":
			stats.GetRequests = value
		case "Get Errors":
			stats.GetErrors = value
		case "Policy Unmarshall Errors":
			stats.PolicyUnmarshallErrors = value
		case "Sandbox Policy Errors":
			stats.SandboxPolicyErrors = value
		case "No Policy Auth Requests":
			stats.NoPolicyAuthRequests = value
		case "gNMI Path Leaves":
			stats.GnmiPathLeaves = value
		case "gNMI Authorizations":
			stats.GnmiAuthorizations = value
		case "gNMI Set Path Permit":
			stats.GnmiSetPathPermit = value
		case "gNMI Set Path Deny":
			stats.GnmiSetPathDeny = value
		case "gNMI Get Path Permit":
			stats.GnmiGetPathPermit = value
		case "gNMI Get Path Deny":
			stats.GnmiGetPathDeny = value
		case "Path To String":
			stats.PathToStringErrors = value
		case "Origin Type":
			stats.OriginTypeErrors = value
		case "Bad Mode":
			stats.BadModeErrors = value
		case "Bad Action":
			stats.BadActionErrors = value
		case "JSON Flatten":
			stats.JsonFlattenErrors = value
		case "String To Path":
			stats.StringToPathErrors = value
		case "Join Paths":
			stats.JoinPathsErrors = value
		case "Nil Path":
			stats.NilPathErrors = value
		case "Nil SetRequest":
			stats.NilSetRequestErrors = value
		case "Empty User":
			stats.EmptyUserErrors = value
		case "Probe Internal":
			stats.ProbeInternalErrors = value
		case "Increment":
			stats.PathCountersIncrement = value
		case "Find":
			stats.PathCountersFind = value
		case "Clear":
			stats.PathCountersClear = value
		case "Walk":
			stats.PathCountersWalk = value
		}
	}
	return stats, nil
}

// validateGnsiPathAuthStats validates multiple statistics against their expected values.
func ValidateGnsiPathAuthStats(t *testing.T, dut *ondatra.DUTDevice, expectedStats map[string]int) {

	cliCmd := "show gnsi path authorization statistics"
	output := config.CMDViaGNMI(context.Background(), t, dut, cliCmd)
	t.Log(output)

	stats, err := parseGnsiPathAuthStats(output)
	t.Logf("Parsed stats: %+v", stats)
	if err != nil {
		t.Fatalf("Failed to parse stats: %v", err)
	}

	for statName, expectedValue := range expectedStats {
		var actualValue int
		switch statName {
		case "RotationsInProgressCount":
			actualValue = stats.RotationsInProgressCount
		case "PolicyRotations":
			actualValue = stats.PolicyRotations
		case "PolicyRotationErrors":
			actualValue = stats.PolicyRotationErrors
		case "PolicyUploadRequests":
			actualValue = stats.PolicyUploadRequests
		case "PolicyUploadErrors":
			actualValue = stats.PolicyUploadErrors
		case "PolicyFinalize":
			actualValue = stats.PolicyFinalize
		case "PolicyFinalizeErrors":
			actualValue = stats.PolicyFinalizeErrors
		case "ProbeRequests":
			actualValue = stats.ProbeRequests
		case "ProbeErrors":
			actualValue = stats.ProbeErrors
		case "GetRequests":
			actualValue = stats.GetRequests
		case "GetErrors":
			actualValue = stats.GetErrors
		case "PolicyUnmarshallErrors":
			actualValue = stats.PolicyUnmarshallErrors
		case "SandboxPolicyErrors":
			actualValue = stats.SandboxPolicyErrors
		case "NoPolicyAuthRequests":
			actualValue = stats.NoPolicyAuthRequests
		case "GnmiPathLeaves":
			actualValue = stats.GnmiPathLeaves
		case "GnmiAuthorizations":
			actualValue = stats.GnmiAuthorizations
		case "GnmiSetPathPermit":
			actualValue = stats.GnmiSetPathPermit
		case "GnmiSetPathDeny":
			actualValue = stats.GnmiSetPathDeny
		case "GnmiGetPathPermit":
			actualValue = stats.GnmiGetPathPermit
		case "GnmiGetPathDeny":
			actualValue = stats.GnmiGetPathDeny
		case "PathToStringErrors":
			actualValue = stats.PathToStringErrors
		case "OriginTypeErrors":
			actualValue = stats.OriginTypeErrors
		case "BadModeErrors":
			actualValue = stats.BadModeErrors
		case "BadActionErrors":
			actualValue = stats.BadActionErrors
		case "JsonFlattenErrors":
			actualValue = stats.JsonFlattenErrors
		case "StringToPathErrors":
			actualValue = stats.StringToPathErrors
		case "JoinPathsErrors":
			actualValue = stats.JoinPathsErrors
		case "NilPathErrors":
			actualValue = stats.NilPathErrors
		case "NilSetRequestErrors":
			actualValue = stats.NilSetRequestErrors
		case "EmptyUserErrors":
			actualValue = stats.EmptyUserErrors
		case "ProbeInternalErrors":
			actualValue = stats.ProbeInternalErrors
		case "PathCountersIncrement":
			actualValue = stats.PathCountersIncrement
		case "PathCountersFind":
			actualValue = stats.PathCountersFind
		case "PathCountersClear":
			actualValue = stats.PathCountersClear
		case "PathCountersWalk":
			actualValue = stats.PathCountersWalk
		default:
			t.Fatalf("Unknown stat name: %s", statName)
		}

		if actualValue != expectedValue {
			t.Errorf("Validation failed for %s: expected %d, got %d", statName, expectedValue, actualValue)
		} else {
			t.Logf("Validation succeeded for %s: expected %d, got %d", statName, expectedValue, actualValue)
		}
	}
}
