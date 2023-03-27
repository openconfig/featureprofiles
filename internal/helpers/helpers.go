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

// Package helpers provides helper APIs to simplify writing FP test cases.
package helpers

import (
	"sort"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/args"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"

)

// FetchOperStatusUPIntfs function uses telemetry to generate a list of all up interfaces.
// When CheckInterfacesInBinding is set to true, all interfaces that are not defined in binding file are excluded.
func FetchOperStatusUPIntfs(t *testing.T, dut *ondatra.DUTDevice) []string {
	t.Helper()
	intfsOperStatusUP := []string{}
	intfs := gnmi.GetAll(t, dut, gnmi.OC().InterfaceAny().Name().State())
	for _, intf := range intfs {
		if *args.CheckInterfacesInBinding {
			checkPort := false
			for _, port := range dut.Ports() {
				if port.Name() == intf {
					checkPort = true
					break
				}
			}
			if !checkPort {
				continue
			}
		}
		operStatus, present := gnmi.Lookup(t, dut, gnmi.OC().Interface(intf).OperStatus().State()).Val()
		if present && operStatus == oc.Interface_OperStatus_UP {
			intfsOperStatusUP = append(intfsOperStatusUP, intf)
		}
	}
	sort.Strings(intfsOperStatusUP)
	return intfsOperStatusUP
}


// ValidateOperStatusUPIntfs function takes a list of interfaces and validates if they are up.
// if any of the given interfaces is not up, it fails the test and logs the failed interfaces.  
func ValidateOperStatusUPIntfs(t *testing.T, dut *ondatra.DUTDevice, upIntfs []string,  timeout time.Duration)  {
	t.Helper()
	t.Logf("Validate interface OperStatus.")
	batch := gnmi.OCBatch()
	upInterfaces := make(map[string]bool)
	for _, port := range upIntfs {
		batch.AddPaths(gnmi.OC().Interface(port).OperStatus())
		upInterfaces[port] = true
	}
	watch := gnmi.Watch(t, dut, batch.State(), timeout, func(val *ygnmi.Value[*oc.Root]) bool {
		root, present := val.Val()
		if !present {
			return false
		}
		for _, port := range upIntfs {
			if root.GetInterface(port).GetOperStatus() != oc.Interface_OperStatus_UP {
				upInterfaces[port] = false
				return false
			} else {
				upInterfaces[port] = true
			}
		}
		return true
	})
	if val, ok := watch.Await(t); !ok {
		for intf, up := range upInterfaces {
			if !up {
				gnmi.Get(t, dut, gnmi.OC().Interface(intf).State())
				t.Logf("Interface %s is not up", intf)
			}
		}
		t.Fatalf("DUT did not reach target state: got %v", val)
	}
}