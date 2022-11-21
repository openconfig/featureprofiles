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

package complete_chassis_reboot_test

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/internal/fptest"
	spb "github.com/openconfig/gnoi/system"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/testt"
)

const (
	oneMinuteInNanoSecond = 6e10
	oneSecondInNanoSecond = 1e9
	rebootDelay           = 120
	// Maximum reboot time is 900 seconds (15 minutes).
	maxRebootTime = 900
	// Maximum wait time for all components to be in responsive state
	maxCompWaitTime = 600
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Test cases:
//  1) Send gNOI reboot request using the method COLD with the delay of N seconds.
//     - method: Only the COLD method is required to be supported by all targets.
//     - Delay: In nanoseconds before issuing reboot.
//     - message: Informational reason for the reboot.
//     - force: Force reboot if basic checks fail. (ex. uncommitted configuration).
//   - Verify the following items.
//     - DUT remains reachable for N seconds by checking DUT current time is updated.
//     - DUT boot time is updated after reboot.
//     - DUT software version is the same after the reboot.
//  2) Send gNOI reboot request using the method COLD without delay.
//     - method: Only the COLD method is required to be supported by all targets.
//     - Delay: 0 - no delay.
//     - message: Informational reason for the reboot.
//     - force: Force reboot if basic checks fail. (ex. uncommitted configuration).
//   - Verify the following items.
//     - DUT boot time is updated after reboot.
//     - DUT software version is the same after the reboot.
//
// Topology:
//   dut:port1 <--> ate:port1
//
// Test notes:
//  - A RebootRequest requests the specified target be rebooted using the specified
//    method after the specified delay.  Only the DEFAULT method with a delay of 0
//    is guaranteed to be accepted for all target types.
//  - A RebootMethod determines what should be done with a target when a Reboot is
//    requested.  Only the COLD method is required to be supported by all
//    targets.  Methods the target does not support should result in failure.
//
//  - gnoi operation commands can be sent and tested using CLI command grpcurl.
//    https://github.com/fullstorydev/grpcurl
//

func TestChassisReboot(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	cases := []struct {
		desc          string
		rebootRequest *spb.RebootRequest
	}{
		{
			desc: "with delay",
			rebootRequest: &spb.RebootRequest{
				Method:  spb.RebootMethod_COLD,
				Delay:   rebootDelay * oneSecondInNanoSecond,
				Message: "Reboot chassis with delay",
				Force:   true,
			}},
		{
			desc: "without delay",
			rebootRequest: &spb.RebootRequest{
				Method:  spb.RebootMethod_COLD,
				Delay:   0,
				Message: "Reboot chassis without delay",
				Force:   true,
			}},
	}

	versions := gnmi.GetAll(t, dut, gnmi.OC().ComponentAny().SoftwareVersion().State())
	expectedVersion := FetchUniqueItems(t, versions)
	sort.Strings(expectedVersion)
	t.Logf("DUT software version: %v", expectedVersion)

	preRebootCompStatus := gnmi.GetAll(t, dut, gnmi.OC().ComponentAny().OperStatus().State())
	t.Logf("DUT components status pre reboot: %v", preRebootCompStatus)

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			gnoiClient := dut.RawAPIs().GNOI().New(t)
			bootTimeBeforeReboot := gnmi.Get(t, dut, gnmi.OC().System().BootTime().State())
			t.Logf("DUT boot time before reboot: %v", bootTimeBeforeReboot)
			prevTime, err := time.Parse(time.RFC3339, gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State()))
			if err != nil {
				t.Fatalf("Failed parsing current-datetime: %s", err)
			}
			start := time.Now()

			t.Logf("Send reboot request: %v", tc.rebootRequest)
			rebootResponse, err := gnoiClient.System().Reboot(context.Background(), tc.rebootRequest)
			t.Logf("Got reboot response: %v, err: %v", rebootResponse, err)
			if err != nil {
				t.Fatalf("Failed to reboot chassis with unexpected err: %v", err)
			}

			if tc.rebootRequest.GetDelay() > 1 {
				t.Logf("Validating DUT remains reachable for at least %d seconds", rebootDelay)
				for {
					time.Sleep(10 * time.Second)
					t.Logf("Time elapsed %.2f seconds since reboot was requested.", time.Since(start).Seconds())
					if uint64(time.Since(start).Seconds()) > rebootDelay {
						t.Logf("Time elapsed %.2f seconds > %d reboot delay", time.Since(start).Seconds(), rebootDelay)
						break
					}
					latestTime, err := time.Parse(time.RFC3339, gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State()))
					if err != nil {
						t.Fatalf("Failed parsing current-datetime: %s", err)
					}
					if latestTime.Before(prevTime) || latestTime.Equal(prevTime) {
						t.Errorf("Get latest system time: got %v, want newer time than %v", latestTime, prevTime)
					}
					prevTime = latestTime
				}
			}

			startReboot := time.Now()
			t.Logf("Wait for DUT to boot up by polling the telemetry output.")
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

			bootTimeAfterReboot := gnmi.Get(t, dut, gnmi.OC().System().BootTime().State())
			t.Logf("DUT boot time after reboot: %v", bootTimeAfterReboot)
			if bootTimeAfterReboot <= bootTimeBeforeReboot {
				t.Errorf("Get boot time: got %v, want > %v", bootTimeAfterReboot, bootTimeBeforeReboot)
			}

			startComp := time.Now()
			t.Logf("Wait for all the components on DUT to come up")

			for {
				postRebootCompStatus := gnmi.GetAll(t, dut, gnmi.OC().ComponentAny().OperStatus().State())

				if len(preRebootCompStatus) == len(postRebootCompStatus) {
					t.Logf("All components on the DUT are in responsive state")
					time.Sleep(10 * time.Second)
					break
				}

				if uint64(time.Since(startComp).Seconds()) > maxCompWaitTime {
					t.Logf("DUT components status post reboot: %v", postRebootCompStatus)
					t.Fatalf("All the components are not in responsive state post reboot")
				}
				time.Sleep(10 * time.Second)
			}

			versions = gnmi.GetAll(t, dut, gnmi.OC().ComponentAny().SoftwareVersion().State())
			swVersion := FetchUniqueItems(t, versions)
			sort.Strings(swVersion)
			t.Logf("DUT software version after reboot: %v", swVersion)
			if diff := cmp.Diff(expectedVersion, swVersion); diff != "" {
				t.Errorf("Software version differed (-want +got):\n%v", diff)
			}
		})
	}
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
