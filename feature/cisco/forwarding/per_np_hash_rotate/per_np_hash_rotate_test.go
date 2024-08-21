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
	"testing"
	"text/tabwriter"
	"time"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/featureprofiles/internal/fptest"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
)

var (
	lcList = []string{}
	rtrID  uint32
	npList = []int{0, 1, 2}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// TestPerNPHashRotateVerifyAutoVal verifies hash-rotate calculation for each NP/LC on the device.
func TestPerNPHashRotateVerifyAutoVal(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	lcList = util.GetLCList(t, dut)
	hashMap := getPerLCPerNPHashTable(t, dut)
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
		fmt.Fprintf(w, " %v\t %v\t  %v\t  %v\t  %v\t\n", rtrID, lck, npv[0], npv[1], npv[2])
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
					setPerNPHashConfig(t, dut, tc.hashval, np, lc, false)
					defer setPerNPHashConfig(t, dut, tc.hashval, np, lc, true)
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

func TestAutoHashValPostLCReload(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	lcList = util.GetLCList(t, dut)
	util.ReloadLinecards(t, lcList)
	t.Log("Verify hash-rotate calculation across all LCs after reloading all linecards")
	TestPerNPHashRotateVerifyAutoVal(t)
}

func TestAutoHashValPostRouterReload(t *testing.T) {
	util.RebootDevice(t)
	t.Log("Verify hash-rotate calculation across all LCs after rebooting router")
	TestPerNPHashRotateVerifyAutoVal(t)
}

func FetchUniqueItems(t *testing.T, s []string) []string {
	itemExisted := make(map[string]bool)
	var uniqueList []string
	for _, item := range s {
		if _, ok := itemExisted[item]; !ok {
			itemExisted[item] = true
			uniqueList = append(uniqueList, item)
		} else {
			t.Logf("Detected duplicated item: %v", item)
		}
	}
	return uniqueList
}

func TestBulkNPHashRotateConfig(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Logf("Get list of LCs")
	lcList = util.GetLCList(t, dut)

	hashConfig := setBulkPerNPHashConfig(t, dut, lcList, false)
	defer setBulkPerNPHashConfig(t, dut, lcList, true)
	for _, lc := range lcList {
		for _, np := range npList {
			if got, want := getPerLCPerNPHashVal(t, dut, np, lc), verifyPerNPHashCLIVal(hashConfig[lc][np]); got != want {
				t.Errorf("per-NP hash rotate value for LC %v NP%v is not per calculation got %v, want %v", lc, np, got, want)
			}
		}
	}
}

func TestBulkNPHashConfigPersistenceRouterReload(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Logf("Get list of LCs")
	lcList = util.GetLCList(t, dut)

	hashConfig := setBulkPerNPHashConfig(t, dut, lcList, false)
	defer setBulkPerNPHashConfig(t, dut, lcList, true)
	util.RebootDevice(t)
	//wait for grpc to be ready
	time.Sleep(1 * time.Minute)
	for _, lc := range lcList {
		for _, np := range npList {
			if got, want := getPerLCPerNPHashVal(t, dut, np, lc), verifyPerNPHashCLIVal(hashConfig[lc][np]); got != want {
				t.Errorf("per-NP hash rotate value for LC %v NP%v is not per calculation got %v, want %v", lc, np, got, want)
			}
		}
	}
}

func TestBulkNPHashConfigPersistenceLCReload(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Logf("Get list of LCs")
	lcList = util.GetLCList(t, dut)

	hashConfig := setBulkPerNPHashConfig(t, dut, lcList, false)
	defer setBulkPerNPHashConfig(t, dut, lcList, true)
	util.ReloadLinecards(t, lcList)
	// wait for LCs to be ready
	time.Sleep(3 * time.Minute)
	for _, lc := range lcList {
		for _, np := range npList {
			if got, want := getPerLCPerNPHashVal(t, dut, np, lc), verifyPerNPHashCLIVal(hashConfig[lc][np]); got != want {
				t.Errorf("per-NP hash rotate value for LC %v NP%v is not per calculation got %v, want %v", lc, np, got, want)
			}
		}
	}
}
