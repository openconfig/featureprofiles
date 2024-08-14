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
	"testing"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
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
