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
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/testt"
)

const (
	// rebootDelay is the time to wait before the DUT starts the reboot process.
	rebootDelay = 120 * time.Second
	// maxRebootTime is the maximum time allowed for the DUT to complete the reboot.
	maxRebootTime = 15 * time.Minute
	// maxCompWaitTime is the maximum wait time for all components to be in a responsive state after reboot.
	maxCompWaitTime = 10 * time.Minute
	// componentPollInterval is the interval at which component status is polled.
	componentPollInterval = 5 * time.Second
	// rebootPollInterval is the interval at which the DUT's reachability is polled during reboot.
	rebootPollInterval = 5 * time.Second
	// contextTimeout is the overall timeout for the gNOI reboot operation.
	contextTimeout = 20 * time.Minute
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
			desc: "with_delay",
			rebootRequest: &spb.RebootRequest{
				Method:  spb.RebootMethod_COLD,
				Delay:   uint64(rebootDelay.Nanoseconds()),
				Message: "Reboot chassis with delay",
				Force:   true,
			}},
		{
			desc: "without_delay",
			rebootRequest: &spb.RebootRequest{
				Method:  spb.RebootMethod_COLD,
				Delay:   0,
				Message: "Reboot chassis without delay",
				Force:   true,
			}},
	}

	versions := gnmi.GetAll(t, dut, gnmi.OC().ComponentAny().SoftwareVersion().State())
	expectedVersion := uniqueSortedStrings(t, versions)
	t.Logf("DUT software version: %v", expectedVersion)

	preRebootCompStatus := gnmi.GetAll(t, dut, gnmi.OC().ComponentAny().OperStatus().State())
	preRebootCompDebug := gnmi.GetAll(t, dut, gnmi.OC().ComponentAny().State())
	var preCompMatrix []string
	for _, preComp := range preRebootCompDebug {
		if preComp.GetOperStatus() != oc.PlatformTypes_COMPONENT_OPER_STATUS_UNSET {
			preCompMatrix = append(preCompMatrix, preComp.GetName()+":"+preComp.GetOperStatus().String())
		}
	}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			gnoiClient, err := dut.RawAPIs().BindingDUT().DialGNOI(t.Context())
			if err != nil {
				t.Fatalf("Error dialing gNOI: %v", err)
			}
			bootTimeBeforeReboot := gnmi.Get(t, dut, gnmi.OC().System().BootTime().State())
			t.Logf("DUT boot time before reboot: %v", bootTimeBeforeReboot)
			prevTime, err := time.Parse(time.RFC3339, gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State()))
			if err != nil {
				t.Fatalf("Failed parsing current-datetime: %s", err)
			}
			start := time.Now()

			t.Logf("Send reboot request: %v.", tc.rebootRequest)
			ctxWithTimeout, cancel := context.WithTimeout(t.Context(), contextTimeout)
			defer cancel()
			_, err = gnoiClient.System().Reboot(ctxWithTimeout, tc.rebootRequest)
			defer gnoiClient.System().CancelReboot(t.Context(), &spb.CancelRebootRequest{})
			if err != nil {
				t.Fatalf("Failed to reboot chassis with unexpected err: %v", err)
			}

			if tc.rebootRequest.GetDelay() > 1 {
				t.Logf("Validating DUT remains reachable for at least %d seconds.", rebootDelay)
				for {
					time.Sleep(10 * time.Second)
					t.Logf("Time elapsed %.2f seconds since reboot was requested.", time.Since(start).Seconds())
					if time.Since(start).Seconds() > rebootDelay.Seconds() {
						t.Logf("Time elapsed (%.2f seconds) has exceeded the reboot delay of %d seconds.", time.Since(start).Seconds(), rebootDelay)
						break
					}
					var latestTime time.Time
					var err error
					if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
						latestTime, err = time.Parse(time.RFC3339, gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State()))
						if err != nil {
							t.Fatalf("Failed parsing current-datetime: %s", err)
						}
					}); errMsg != nil && time.Since(start).Seconds() < rebootDelay.Seconds() {
						t.Fatalf("Get request failed before the reboot delay: %s.", *errMsg)
					}

					if err != nil && time.Since(start).Seconds() < rebootDelay.Seconds() {
						t.Fatalf("Failed parsing current-datetime: %s.", err)
					}
					if latestTime.Before(prevTime) || latestTime.Equal(prevTime) {
						t.Errorf("Get latest system time: got %v, want newer time than %v.", latestTime, prevTime)
					}
					prevTime = latestTime
				}
			}

			startReboot := time.Now()
			t.Logf("Wait for DUT to boot up by polling the telemetry output.")
			{
				ticker := time.NewTicker(rebootPollInterval)
				defer ticker.Stop()
				timeout := time.After(maxRebootTime)

			rebootLoop:
				for {
					select {
					case <-timeout:
						t.Fatalf("Timeout exceeded: DUT did not reboot within %v seconds.", maxRebootTime)
					case <-ticker.C:
						var currentTime string
						if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
							currentTime = gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State())
						}); errMsg != nil {
							t.Logf("Time elapsed %.2f seconds, DUT not reachable yet: %s.", time.Since(startReboot).Seconds(), *errMsg)
						} else {
							t.Logf("Device rebooted successfully with received time: %v.", currentTime)
							break rebootLoop
						}
					}
				}
			}
			t.Logf("Device boot time: %.2f seconds.", time.Since(startReboot).Seconds())

			bootTimeAfterReboot := gnmi.Get(t, dut, gnmi.OC().System().BootTime().State())
			t.Logf("DUT boot time after reboot: %v", bootTimeAfterReboot)
			if bootTimeAfterReboot <= bootTimeBeforeReboot {
				t.Errorf("Get boot time: got %v, want > %v.", bootTimeAfterReboot, bootTimeBeforeReboot)
			}

			startComp := time.Now()
			t.Logf("Wait for all the components on DUT to come up.")
			{
				ticker := time.NewTicker(componentPollInterval)
				defer ticker.Stop()
				timeout := time.After(maxCompWaitTime)

			compLoop:
				for {
					select {
					case <-timeout:
						postRebootCompDebug := gnmi.GetAll(t, dut, gnmi.OC().ComponentAny().State())
						var postCompMatrix []string
						for _, postComp := range postRebootCompDebug {
							if postComp.GetOperStatus() != oc.PlatformTypes_COMPONENT_OPER_STATUS_UNSET {
								postCompMatrix = append(postCompMatrix, postComp.GetName()+":"+postComp.GetOperStatus().String())
							}
						}
						if rebootDiff := cmp.Diff(preCompMatrix, postCompMatrix); rebootDiff != "" {
							t.Logf("[DEBUG] Unexpected diff after reboot (-component missing from pre reboot, +component added from pre reboot): %v.", rebootDiff)
						}
						postRebootCompStatus := gnmi.GetAll(t, dut, gnmi.OC().ComponentAny().OperStatus().State())
						t.Fatalf("Timeout exceeded: There's a difference in components obtained in pre reboot: %v and post reboot: %v.", len(preRebootCompStatus), len(postRebootCompStatus))
					case <-ticker.C:
						postRebootCompStatus := gnmi.GetAll(t, dut, gnmi.OC().ComponentAny().OperStatus().State())
						if len(preRebootCompStatus) == len(postRebootCompStatus) {
							t.Logf("All components on the DUT are in responsive state after %.2f seconds.", time.Since(startComp).Seconds())
							break compLoop
						}
					}
				}
			}

			versions = gnmi.GetAll(t, dut, gnmi.OC().ComponentAny().SoftwareVersion().State())
			swVersion := uniqueSortedStrings(t, versions)
			t.Logf("DUT software version after reboot: %v", swVersion)
			if diff := cmp.Diff(expectedVersion, swVersion); diff != "" {
				t.Errorf("Software version differed (-want +got):\n%v.", diff)
			}
		})
	}
}

func uniqueSortedStrings(t *testing.T, s []string) []string {
	t.Helper()
	itemExisted := make(map[string]bool)
	var uniqueList []string
	for _, item := range s {
		if _, ok := itemExisted[item]; ok {
			continue
		}
		itemExisted[item] = true
		uniqueList = append(uniqueList, item)
	}
	sort.Strings(uniqueList)
	return uniqueList
}
