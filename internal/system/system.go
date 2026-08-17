// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package system provides helper functions for gNMI system related operations.
package system

import (
	"fmt"
	"testing"
	"time"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/testt"
)

// FindProcessIDByName uses telemetry to find out the PID of a process.
func FindProcessIDByName(t *testing.T, dut *ondatra.DUTDevice, pName string) uint64 {
	t.Helper()

	var pid uint64
	pList := gnmi.GetAll[*oc.System_Process](t, dut, gnmi.OC().System().ProcessAny().State())
	for _, proc := range pList {
		if proc.GetName() == pName {
			pid = proc.GetPid()
			break
		}
	}
	return pid
}

// ProcessInfo holds PID, Start Time, and Memory Usage of a system process.
type ProcessInfo struct {
	Pid         uint64
	StartTime   uint64
	MemoryUsage uint64
}

// GetProcessInfo returns the PID, Start Time, and Memory Usage of the named processes.
func GetProcessInfo(t *testing.T, dut *ondatra.DUTDevice, pNames []string) (map[string]*ProcessInfo, error) {
	t.Helper()
	pList := gnmi.GetAll[*oc.System_Process](t, dut, gnmi.OC().System().ProcessAny().State())
	results := make(map[string]*ProcessInfo)

	nameMap := make(map[string]bool)
	for _, name := range pNames {
		nameMap[name] = true
	}

	for _, proc := range pList {
		pName := proc.GetName()
		if nameMap[pName] {
			if _, ok := results[pName]; !ok {
				results[pName] = &ProcessInfo{
					Pid:         proc.GetPid(),
					StartTime:   proc.GetStartTime(),
					MemoryUsage: proc.GetMemoryUsage(),
				}
			}
		}
	}

	for _, name := range pNames {
		if _, ok := results[name]; !ok {
			return nil, fmt.Errorf("process %q not found", name)
		}
	}
	return results, nil
}

// AwaitDeviceReachable waits for the device to become reachable via gNMI after an event like a reboot or switchover.
// It continuously polls a basic state leaf (like current-datetime) until it succeeds or times out.
func AwaitDeviceReachable(t *testing.T, dut *ondatra.DUTDevice, timeout time.Duration) {
	t.Helper()
	start := time.Now()
	t.Logf("Waiting for device %v to become reachable via gNMI (timeout: %v)...", dut.Name(), timeout)

	// Fast initial polling, backing off to larger intervals.
	pollInterval := 5 * time.Second
	for {
		if time.Since(start) > timeout {
			t.Fatalf("Device %v failed to become reachable within %v timeout", dut.Name(), timeout)
		}

		var currentTime string
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			currentTime = gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State())
		}); errMsg != nil {
			time.Sleep(pollInterval)
			if pollInterval < 30*time.Second {
				pollInterval += 5 * time.Second
			}
			continue
		}

		t.Logf("Device %v is reachable. Current system datetime: %v", dut.Name(), currentTime)
		return
	}
}
