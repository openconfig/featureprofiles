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
	"os"
	"testing"
	"text/tabwriter"

	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
)

var (
	lcList = []string{}
	rtrID  uint32
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestPerNPhashRotateVerifyVal(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	lcList = util.GetLCList(t, dut)
	hashMap := getPerLCPerNPHashValTable(t, dut)
	for lck, npv := range hashMap {
		lcSlot := uint32(util.GetLCSlotID(lck))
		rtrID = getRouterID(t, dut, lck)
		for npuID, gotVal := range npv {
			if want := verifyPerNPHashRotateValCalculation(t, lcSlot, uint32(npuID), rtrID); want != gotVal {
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
