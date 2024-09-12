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
	"fmt"
	"math/rand"
	"os"
	"strings"
	"testing"
	"text/tabwriter"
	"time"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/fptest"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
)

const (
	cardTypeRp = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD
)

var (
	lcList = []string{}
	rtrID  uint32
	npList = []int{0, 1, 2}
	h      = NpuHash{npList: []int{0, 1, 2}}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// TestPerNPHashRotateVerifyAutoVal verifies hash-rotate calculation for each NP/LC on the device.
func TestPerNPHashRotateVerifyAutoVal(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	lcList = util.GetLCList(t, dut)
	if len(lcList) == 0 {
		// attempt to get RP
		lcList = components.FindComponentsByType(t, dut, cardTypeRp)
		h.npList = []int{0}
	}
	hashMap := getPerLCPerNPHashTable(t, dut, lcList)
	for lck, npv := range hashMap {
		lcSlot := uint32(util.GetLCSlotID(t, lck))
		rtrID = getOFARouterID(t, dut, lck)
		for npuID, gotVal := range npv {
			if want := verifyPerNPHashAutoValCalculation(t, lcSlot, uint32(npuID), rtrID); want != gotVal {
				t.Errorf("per-NP hash rotate value for LC %v NP%v is not per calculation got %v, want %v", lck, npuID, gotVal, want)
			}
		}
	}
	t.Log("Print a table of per-NP hash-rotate values for each LC on the device.")
	w := tabwriter.NewWriter(os.Stdout, 10, 1, 1, ' ', tabwriter.Debug)
	fmt.Fprintln(w, " RTR_ID\tLC_SLOT_ID\t  NP0\t  NP1\t  NP2\t")
	for lck, npv := range hashMap {
		if strings.Contains(lck, "RP") {
			fmt.Fprintf(w, " %v\t  %v\t  %v\t\n", rtrID, lck, npv[0])
		} else if len(npv) != 3 {
			t.Errorf("For linecard %v, expected 3 NPs got %v", lck, npv)
			continue
		} else {
			fmt.Fprintf(w, " %v\t %v\t  %v\t  %v\t  %v\t\n", rtrID, lck, npv[0], npv[1], npv[2])
		}
	}
	w.Flush()
}

// TestChangeRouterIDVerifyAutoVal verifies for each NP/LC on the device after router-ID change.
// PI router-ID is influenced by Highest IP on device.
func TestChangeRouterIDVerifyAutoVal(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ridB := getPIRouterID(t, dut)
	t.Logf("IPv4 router-ID before change %v", ridB)
	t.Log("Change router-id by configuring highest IPv4 address on existing lowest loopback0") //Hardcoded to lo0, since tests will run on sim.
	dutLoopback := attrs.Attributes{
		IPv4:    "254.254.254.254",
		IPv4Len: 32,
	}
	lb := netutil.LoopbackInterface(t, dut, 0)
	lo0 := dutLoopback.NewOCInterface(lb, dut)
	lo0.Type = oc.IETFInterfaces_InterfaceType_softwareLoopback
	gnmi.Update(t, dut, gnmi.OC().Interface(lb).Config(), lo0)
	ridA := getPIRouterID(t, dut)
	t.Logf("IPv4 router-ID after change %v", ridA)
	if ridB == ridA {
		t.Errorf("IPv4 Router-id did not change want %v, got %v", dutLoopback.IPv4, ridA)
	}
	t.Log("Verify hash-rotate calculation across all LCs after router-id change")
	TestPerNPHashRotateVerifyAutoVal(t)
}

type testCase struct {
	name    string
	desc    string
	npval   []int
	hashval int
	lcloc   []string
}

func TestExplicitNPHashRotateConfig(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Logf("Get list of LCs")
	lcList = util.GetLCList(t, dut)
	cases := []testCase{
		{
			name:    "Test perNP CLI config for 1 NP",
			desc:    "Configure per-NP hash value on single NP0 for all LCs",
			npval:   npList[:1],
			hashval: rand.Intn(34) + 1, // valid range (1-35]
			lcloc:   lcList,
		},
		{
			name:    "Test perNP CLI config for All NPs",
			desc:    "Configure per-NP hash value on All the NPs for all LCs",
			npval:   npList,
			hashval: rand.Intn(34) + 1,
			lcloc:   lcList,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			for _, lc := range tc.lcloc {
				for _, np := range tc.npval {
					setPerNPHashConfig(t, dut, tc.hashval, np, lc, true)
					defer setPerNPHashConfig(t, dut, tc.hashval, np, lc, false)
					if got, want := getPerLCPerNPHashVal(t, dut, np, lc), verifyPerNPHashCLIVal(tc.hashval); got != want {
						t.Errorf("per-NP hash rotate value for LC %v NP%v is not per calculation got %v, want %v", lc, tc.npval, got, want)
					}
				}
			}
		})
	}
}

// TestAutoHashValRandomize verifies the auto set per-NP hash values are randomized such that
// a given LC's 3 NPs dont have same values & LCn & LCn+1 values are not same.
func TestAutoHashValRandomize(t *testing.T) {
	//TODO
	t.Skip()
}

// TestAutoHashValPostRouterReload verifies the auto set per-NP hash values are retained after linecard reload.
func TestAutoHashValPostLCReload(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	lcList = util.GetLCList(t, dut)
	if len(lcList) == 0 {
		t.Skip("No linecards found")
	}
	util.ReloadLinecards(t, lcList)
	t.Log("Verify hash-rotate calculation across all LCs after reloading all linecards")
	TestPerNPHashRotateVerifyAutoVal(t)
}

// TestAutoHashValPostRouterReload verifies the auto set per-NP hash values are retained after router reload.
func TestAutoHashValPostRouterReload(t *testing.T) {
	util.RebootDevice(t)
	time.Sleep(1 * time.Minute)
	t.Log("Verify hash-rotate calculation across all LCs after rebooting router")
	TestPerNPHashRotateVerifyAutoVal(t)
}

// TestGlobalHashRotateConfig verifies the global hash values can be configured for multiple linecards at the same time.
func TestBulkNPHashRotateConfig(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Logf("Get list of LCs")
	lcList = util.GetLCList(t, dut)
	if len(lcList) == 0 {
		// attempt to get RP
		lcList = components.FindComponentsByType(t, dut, cardTypeRp)
		h.npList = []int{0}
	}

	h.setBulkPerNPHashConfig(t, dut, lcList, true)
	defer h.setBulkPerNPHashConfig(t, dut, lcList, false)
	for _, lc := range lcList {
		for _, np := range h.npList {
			if got, want := getPerLCPerNPHashVal(t, dut, np, lc), verifyPerNPHashCLIVal(h.hashValMap[lc][np]); got != want {
				t.Errorf("per-NP hash rotate value for LC %v NP%v is not per calculation got %v, want %v", lc, np, got, want)
			}
		}
	}
}

// TestGlobalHashRotateConfig verifies the hash values can be configured for multiple linecards at the same time ard are retained after router reload.
func TestBulkNPHashConfigPersistenceRouterReload(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Logf("Get list of LCs")
	lcList = util.GetLCList(t, dut)
	if len(lcList) == 0 {
		// attempt to get RP
		lcList = components.FindComponentsByType(t, dut, cardTypeRp)
		h.npList = []int{0}
	}
	h.setBulkPerNPHashConfig(t, dut, lcList, true)
	defer h.setBulkPerNPHashConfig(t, dut, lcList, false)
	util.RebootDevice(t)
	//wait for grpc to be ready
	time.Sleep(2 * time.Minute)
	for _, lc := range lcList {
		for _, np := range h.npList {
			if got, want := getPerLCPerNPHashVal(t, dut, np, lc), verifyPerNPHashCLIVal(h.hashValMap[lc][np]); got != want {
				t.Errorf("per-NP hash rotate value for LC %v NP%v is not per calculation got %v, want %v", lc, np, got, want)
			}
		}
	}
}

// TestGlobalHashRotateConfig verifies the hash values can be configured for multiple linecards at the same time and are retained after linecard reload.
func TestBulkNPHashConfigPersistenceLCReload(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Logf("Get list of LCs")
	lcList = util.GetLCList(t, dut)
	if len(lcList) == 0 {
		t.Skip("No linecards found")
	}

	h.setBulkPerNPHashConfig(t, dut, lcList, true)
	defer h.setBulkPerNPHashConfig(t, dut, lcList, false)
	util.ReloadLinecards(t, lcList)
	// wait for LCs to be ready
	time.Sleep(3 * time.Minute)
	for _, lc := range lcList {
		for _, np := range npList {
			if got, want := getPerLCPerNPHashVal(t, dut, np, lc), verifyPerNPHashCLIVal(h.hashValMap[lc][np]); got != want {
				t.Errorf("per-NP hash rotate value for LC %v NP%v is not per calculation got %v, want %v", lc, np, got, want)
			}
		}
	}
}

// TestGlobalAndPerNPHashCoExistence verifies that per-NP hash value is preferred when global value is configured at the same time.
func TestGlobalAndPerNPHashCoExistence(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Logf("Get list of LCs")
	lcList = util.GetLCList(t, dut)
	if len(lcList) == 0 {
		// attempt to get RP
		lcList = components.FindComponentsByType(t, dut, cardTypeRp)
		h.npList = []int{0}
	}
	h.setBulkPerNPHashConfig(t, dut, lcList, true)
	defer h.setBulkPerNPHashConfig(t, dut, lcList, false)
	setGlobalHashConfig(t, dut, 10, true)
	defer setGlobalHashConfig(t, dut, 10, false)
	t.Logf("Verify LCs are using per-NP hash value \n, %v", h.hashValMap)
	for _, lc := range lcList {
		for _, np := range h.npList {
			if got, want := getPerLCPerNPHashVal(t, dut, np, lc), verifyPerNPHashCLIVal(h.hashValMap[lc][np]); got != want {
				t.Errorf("per-NP hash rotate value for LC %v NP%v is not per calculation got %v, want %v", lc, np, got, want)
			}
		}
	}
}

// TestGlobalTakesOverAfterPerNPHashDeleted verifies that global hash value is preferred when per-NP hash value is deleted.
func TestGlobalTakesOverAfterPerNPHashDeleted(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Logf("Get list of LCs")
	lcList = util.GetLCList(t, dut)
	if len(lcList) == 0 {
		// attempt to get RP
		lcList = components.FindComponentsByType(t, dut, cardTypeRp)
		h.npList = []int{0}
	}

	globalHash := rand.Intn(34) + 1

	// configure per np hash value
	h.setBulkPerNPHashConfig(t, dut, lcList, true)
	// configure global hash value
	setGlobalHashConfig(t, dut, globalHash, true)
	defer setGlobalHashConfig(t, dut, globalHash, false)
	// delete per np hash value
	h.setBulkPerNPHashConfig(t, dut, lcList, false)

	t.Logf("Verify LCs are using global hash value \n, %v", globalHash)
	for _, lc := range lcList {
		for _, np := range h.npList {
			if got, want := getPerLCPerNPHashVal(t, dut, np, lc), verifyPerNPHashCLIVal(globalHash); got != want {
				t.Errorf("Global hash value for LC %v NP%v is not per calculation got %v, want %v", lc, np, got, want)
			}
		}
	}
}

// TestGlobalTakesOverAfterPerNPHashDeletedLcReload verifies that global hash value is preferred when pbr profile is deleted and a specific linecards is reloaded.
func TestGlobalTakesOverAfterPbrProfileDeletedLcReload(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Logf("Get list of LCs")
	lcList = util.GetLCList(t, dut)
	if len(lcList) == 0 {
		t.Skip("No linecards found")
	}

	globalHash := rand.Intn(34) + 1
	// test with one lc
	lcList = []string{lcList[rand.Intn(len(lcList))]}

	// configure per np hash value
	h.setBulkPerNPHashConfig(t, dut, lcList, true)
	defer h.setBulkPerNPHashConfig(t, dut, lcList, false)
	// configure global hash value
	setGlobalHashConfig(t, dut, globalHash, true)
	defer setGlobalHashConfig(t, dut, globalHash, false)

	// unconfigure pbr profile
	setHwProfilePbrVrfRedirect(t, dut, false)
	// reload linecards after pbr has been configured back
	defer util.ReloadLinecards(t, lcList)
	defer setHwProfilePbrVrfRedirect(t, dut, true)

	util.ReloadLinecards(t, lcList)

	t.Logf("Verify LC %v are using global hash value \n, %v", lcList[0], globalHash)
	for _, np := range npList {
		if got, want := getPerLCPerNPHashVal(t, dut, np, lcList[0]), verifyPerNPHashCLIVal(globalHash); got != want {
			t.Errorf("Global hash value for LC %v NP%v is not per calculation got %v, want %v", lcList[0], np, got, want)
		}
	}

}

// TestGlobalTakesOverAfterPerNPHashDeletedAllLcReload verifies that global hash value is preferred when pbr profile is deleted and all linecards are reloaded.
func TestGlobalTakesOverAfterPbrProfileDeletedAllLcReload(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Logf("Get list of LCs")
	lcList = util.GetLCList(t, dut)
	if len(lcList) == 0 {
		t.Skip("No linecards found")
	}
	globalHash := rand.Intn(34) + 1
	// configure per np hash value
	h.setBulkPerNPHashConfig(t, dut, lcList, true)
	defer h.setBulkPerNPHashConfig(t, dut, lcList, false)
	// configure global hash value
	setGlobalHashConfig(t, dut, globalHash, true)
	defer setGlobalHashConfig(t, dut, globalHash, false)

	// unconfigure pbr profile
	setHwProfilePbrVrfRedirect(t, dut, false)
	// reload router after pbr has been configured back
	defer util.ReloadLinecards(t, lcList)
	defer setHwProfilePbrVrfRedirect(t, dut, true)

	util.ReloadLinecards(t, lcList)

	t.Logf("Verify LC %v are using global hash value \n, %v", lcList, globalHash)
	for _, lc := range lcList {
		for _, np := range npList {
			if got, want := getPerLCPerNPHashVal(t, dut, np, lc), verifyPerNPHashCLIVal(globalHash); got != want {
				t.Errorf("Global hash value for LC %v NP%v is not per calculation got %v, want %v", lc, np, got, want)
			}
		}
	}
}

// TestNonAutomaticHashWithoutPbrAndGlobalOrPerNpHashAllLcReload verifies that non-automatic hash value is preferred when pbr profile is deleted and all linecards are reloaded.
func TestNonAutomaticHashWithoutPbrAndGlobalOrPerNpHashAllLcReload(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Logf("Get list of LCs")
	lcList = util.GetLCList(t, dut)
	if len(lcList) == 0 {
		t.Skip("No linecards found")
	}
	// config/unconfig per np hash value and global hash to clear any existing values
	globalHash := rand.Intn(34) + 1
	h.setBulkPerNPHashConfig(t, dut, lcList, true)
	setGlobalHashConfig(t, dut, globalHash, true)
	time.Sleep(5 * time.Second)
	h.setBulkPerNPHashConfig(t, dut, lcList, false)
	setGlobalHashConfig(t, dut, globalHash, false)

	// non-automatic hash value without pbr policy and global or per-np hash
	// debug shell output will show this hash value as 1
	nonAutoHash := 0
	// unconfigure pbr profile
	setHwProfilePbrVrfRedirect(t, dut, false)
	// reload router after pbr has been configured back
	util.ReloadLinecards(t, lcList)
	time.Sleep(30 * time.Second)

	t.Logf("Verify LC %v are using global hash value \n, %v", lcList, nonAutoHash)
	for _, lc := range lcList {
		for _, np := range npList {
			if got, want := getPerLCPerNPHashVal(t, dut, np, lc), verifyPerNPHashCLIVal(nonAutoHash); got != want {
				t.Errorf("Global hash value for LC %v NP%v is not per calculation got %v, want %v", lc, np, got, want)
			}
		}
	}

	// restrore pbr profile
	setHwProfilePbrVrfRedirect(t, dut, true)
	util.ReloadLinecards(t, lcList)
}
