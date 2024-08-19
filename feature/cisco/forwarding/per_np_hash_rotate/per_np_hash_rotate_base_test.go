// Copyright 2022 Google LLC
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

// Package setup is scoped only to be used for scripts in path
// feature/experimental/system/gnmi/benchmarking/ate_tests/
// Do not use elsewhere.
package per_np_hash_rotate_test

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/openconfig/featureprofiles/internal/cisco/config"
	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi/oc"
)

const (
	linecardType           = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_LINECARD
	lbHashMaxNodeID        = 36
	lbHashSeparationOffset = 7
)

// setperNPHashConfig configures per NP hash-rotate valie for a LC with
// cef platform load-balancing algorithm adjust <> instance <> location <>.
func setPerNPHashConfig(t *testing.T, dut *ondatra.DUTDevice, hashVal, npVal int, lcLoc string) {
	t.Helper()
	configCli := fmt.Sprintf("cef platform load-balancing algorithm adjust %v instance %v location %v", hashVal, npVal, lcLoc)
	config.TextWithGNMI(context.Background(), t, dut, configCli)
}

// verifyPerNPHashAutoValCalculation verifies the value programmed as per https://gh-xr.scm.engit.cisco.com/xr/iosxr/blob/main/platforms/common/leaba/ofa/include/la_loadbalance.h#L126
// PER_NP_HASH_ROTATE = (CFG_RTR_ID+LC_ID+(NPU_ID*LB_HASH_SEPARATION_OFFSET)) % (LB_HASH_MAX_NODE_ID+1)
// CFG_RTR_ID = show ofa objects global location <> | i router_id
// LC_ID = LC Slot# + 1 (due to CSCwk77444);  LB_HASH_SEPARATION_OFFSET = 7 ; LB_HASH_MAX_NODE_ID = 35
// Programmed Hash rotate value in SDK = PER_NP_HASH_ROTATE + 1.
// TODO : Remove adding 1 in lcSlot after CSCwk77444 is fixed.
func verifyPerNPHashAutoValCalculation(t *testing.T, lcSlot, npuID, rtrID uint32) int {
	t.Helper()
	var hashVal int
	hashCalc := ((rtrID + lcSlot + npuID*lbHashSeparationOffset) % lbHashMaxNodeID)
	if hashCalc == uint32(35) {
		hashVal = int(0) //After 35 , SDK val rollovers to 0.
	} else {
		hashVal = int(hashCalc + 1) //SDK val is +1; After 35 , SDK val rollovers to 0.
	}
	return hashVal
}

func verifyPerNPHashCLIVal(cliHashVal int) int {
	var hashVal int
	if cliHashVal == 35 {
		hashVal = int(0) //After 35 , SDK val rollovers to 0.
	} else {
		hashVal = int(cliHashVal + 1) //SDK val is +1; After 35 , SDK val rollovers to 0.
	}
	return hashVal
}

// getPerLCPerNPHashValTable returns a map of LC and corresponding per NP hash rotate value using
// show controllers npu debugshell 0 "script device_hash_rotate_info get_val_all_npu" location <LC#> CLI.
func getPerLCPerNPHashTable(t *testing.T, dut *ondatra.DUTDevice) map[string][]int {
	t.Helper()
	hashValMap := make(map[string][]int)
	//get per LC per NP hash-rotate value from the device
	for _, lc := range lcList {
		debugCLI := fmt.Sprintf("show controllers npu debugshell 0 'script device_hash_rotate_info get_val_all_npu' location %v", lc)
		cliResp := config.CMDViaGNMI(context.Background(), t, dut, debugCLI)
		npList := parseDebugCLIOutput(t, cliResp)
		hashValMap[lc] = npList
	}
	return hashValMap
}

// getPerLCPerNPHashVal returns int val for a given NP and LC using
// show controllers npu debugshell 0 "script device_hash_rotate_info get_val" location <LC#> CLI.
func getPerLCPerNPHashVal(t *testing.T, dut *ondatra.DUTDevice, np int, lc string) int {
	t.Helper()
	var hashVal int
	//get per LC per NP hash-rotate value from the device
	debugCLI := fmt.Sprintf("show controllers npu debugshell %v 'script device_hash_rotate_info get_val' location %v", np, lc)
	cliResp := config.CMDViaGNMI(context.Background(), t, dut, debugCLI)
	npList := parseDebugCLIOutput(t, cliResp)
	hashVal = npList[0]
	return hashVal
}

// parseDebugCLIOutput parses show controllers npu debugshell 0 "script device_hash_rotate_info get_val_all_npu" location <LC#> CLI & returns list of hash rotate int values per LC,
// where list index corresponds to the NPU_ID.
func parseDebugCLIOutput(t *testing.T, cliOut string) []int {
	npList := []int{}
	cliSplit := strings.Split(cliOut, "Hash Rotate Value and seed value in HW for NPU:")
	re := regexp.MustCompile("[0-9]+")
	for i, v := range cliSplit {
		if i == 0 {
			continue
		}
		intList := re.FindAllString(v, -1)
		npValStr := intList[1]
		//string to int
		npValInt := util.StringToInt(t, npValStr)
		npList = append(npList, npValInt)
	}
	return npList
}

// getOFARouterID returns uint32 OFA router-id using show ofa objects global location <> CLI.
func getOFARouterID(t *testing.T, dut *ondatra.DUTDevice, lcloc string) uint32 {
	var rtr string
	ofaGlObj := fmt.Sprintf("show ofa objects global location %v | include router", lcloc)
	cliResp := config.CMDViaGNMI(context.Background(), t, dut, ofaGlObj)
	cliSplit := strings.Split(cliResp, "=> ")
	rtr = strings.ReplaceAll(cliSplit[1], "\n", "")
	t.Log("OFA Router-ID is", rtr)
	rtrID, err := strconv.ParseUint(rtr, 0, 32)
	if err != nil {
		t.Fatalf("error in int conversion %v", err)
	}
	return uint32(rtrID)
}

// getPIRouterID parses the IPv4 address used as router-id as string with show router-id ipv4 CLI.
func getPIRouterID(t *testing.T, dut *ondatra.DUTDevice) string {
	var id string
	rtrIDCli := "show router-id ipv4"
	cliResp := config.CMDViaGNMI(context.Background(), t, dut, rtrIDCli)
	cliSplit := strings.Split(cliResp, "Interface")
	cliRep := strings.ReplaceAll(cliSplit[1], "\n", "")
	cliSplit2 := strings.Split(cliRep, "Loopback")
	cliRep2 := strings.ReplaceAll(cliSplit2[0], " ", "")
	id = cliRep2
	return id
}
