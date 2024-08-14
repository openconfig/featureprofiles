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
	"math/rand"
	"os"
	"testing"
	"text/tabwriter"
	"time"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/fptest"
	spb "github.com/openconfig/gnoi/system"
	tpb "github.com/openconfig/gnoi/types"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/testt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
			hashval: rand.Intn(34) + 1, // 0 is not a valid value for hash-rotate
			lcloc:   lcList,
		},
		{
			name:    "Test perNP CLI config for All NPs",
			desc:    "Configure per-NP hash value on All the NPs for all LCs",
			npval:   npList,
			hashval: rand.Intn(34) + 1, // 0 is not a valid value for hash-rotate
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
	const linecardBoottime = 5 * time.Minute
	dut := ondatra.DUT(t, "dut")
	lcList = util.GetLCList(t, dut)
	ReloadLinecards(t, lcList)
	t.Log("Verify hash-rotate calculation across all LCs after reloading all linecards")
	TestPerNPHashRotateVerifyAutoVal(t)
}

func TestAutoHashValPostRouterReload(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	startTime := time.Now()
	lcList = util.GetLCList(t, dut)

	RebootDevice(t)
	t.Logf("It took %v minutes to reboot router.", time.Now().Sub(startTime).Minutes())

	t.Log("Verify hash-rotate calculation across all LCs after rebooting router")
	TestPerNPHashRotateVerifyAutoVal(t)
}

func RebootDevice(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	const (
		oneMinuteInNanoSecond = 6e10
		oneSecondInNanoSecond = 1e9
		pollingDelay          = 180 * time.Second
		// Maximum reboot time is 900 seconds (15 minutes).
		maxRebootTime = 900
	)

	rebootRequest := &spb.RebootRequest{
		Method:  spb.RebootMethod_COLD,
		Message: "Reboot chassis with cold method gracefully",
		Force:   false,
	}

	gnoiClient, err := dut.RawAPIs().BindingDUT().DialGNOI(context.Background())
	if err != nil {
		t.Fatalf("Error dialing gNOI: %v", err)
	}
	bootTimeBeforeReboot := gnmi.Get(t, dut, gnmi.OC().System().BootTime().State())
	t.Logf("DUT boot time before reboot: %v", bootTimeBeforeReboot)
	if err != nil {
		t.Fatalf("Failed parsing current-datetime: %s", err)
	}

	t.Logf("Send reboot request: %v", rebootRequest)
	startReboot := time.Now()
	rebootResponse, err := gnoiClient.System().Reboot(context.Background(), rebootRequest)
	defer gnoiClient.System().CancelReboot(context.Background(), &spb.CancelRebootRequest{})
	t.Logf("Got reboot response: %v, err: %v", rebootResponse, err)
	if err != nil {
		t.Fatalf("Failed to reboot chassis with unexpected err: %v", err)
	}

	t.Logf("Wait for the device to gracefully complete system backup and start rebooting.")
	time.Sleep(pollingDelay)

	t.Logf("Check if router has booted by polling the telemetry output.")
	for {
		var currentTime string
		t.Logf("Time elapsed %.2f seconds since reboot started.", time.Since(startReboot).Seconds())
		time.Sleep(30 * time.Second)
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			currentTime = gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State())
		}); errMsg != nil {
			t.Logf("Got testt.CaptureFatal errMsg: %s, keep polling ...", *errMsg)
		} else {
			t.Logf("Device rebooted successfully with received time: %v", currentTime)
			break
		}

		if uint64(time.Since(startReboot).Seconds()) > maxRebootTime {
			t.Errorf("Check boot time: got %v, want < %v", time.Since(startReboot), maxRebootTime)
		}
	}
	t.Logf("Device boot time: %.2f seconds", time.Since(startReboot).Seconds())
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

func ReloadLinecards(t *testing.T, lcList []string) {
	const linecardBoottime = 5 * time.Minute
	dut := ondatra.DUT(t, "dut")
	gnoiClient := dut.RawAPIs().GNOI(t)
	rebootSubComponentRequest := &spb.RebootRequest{
		Method:        spb.RebootMethod_COLD,
		Subcomponents: []*tpb.Path{},
	}

	req := &spb.RebootStatusRequest{
		Subcomponents: []*tpb.Path{},
	}

	for _, lc := range lcList {
		rebootSubComponentRequest.Subcomponents = append(rebootSubComponentRequest.Subcomponents, components.GetSubcomponentPath(lc, false))
		req.Subcomponents = append(req.Subcomponents, components.GetSubcomponentPath(lc, false))
	}

	t.Logf("Reloading linecards: %v", lcList)
	startTime := time.Now()
	_, err := gnoiClient.System().Reboot(context.Background(), rebootSubComponentRequest)
	if err != nil {
		t.Fatalf("Failed to perform line card reboot with unexpected err: %v", err)
	}

	rebootDeadline := startTime.Add(linecardBoottime)
	for retry := true; retry; {
		t.Log("Waiting for 10 seconds before checking linecard status.")
		time.Sleep(10 * time.Second)
		if time.Now().After(rebootDeadline) {
			retry = false
			break
		}
		resp, err := gnoiClient.System().RebootStatus(context.Background(), req)
		switch {
		case status.Code(err) == codes.Unimplemented:
			t.Fatalf("Unimplemented RebootStatus RPC: %v", err)
		case err == nil:
			retry = resp.GetActive()
		default:
			// any other error just sleep.
		}
	}
	t.Logf("It took %v minutes to reboot linecards.", time.Now().Sub(startTime).Minutes())
}
