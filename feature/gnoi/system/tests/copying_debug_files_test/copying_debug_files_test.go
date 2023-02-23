// Copyright 2022 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package copying_debug_files_test

import (
	"context"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/fptest"
	hpb "github.com/openconfig/gnoi/healthz"
	gnps "github.com/openconfig/gnoi/system"
	tpb "github.com/openconfig/gnoi/types"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
)

// Name of the process to be killed
var processNames = map[ondatra.Vendor]string{
	ondatra.ARISTA:  "bgp",
	ondatra.JUNIPER: "rpd",
	ondatra.NOKIA:   "sr_bgp_mgr",
}

// Name of the component to check the health of
var components = map[ondatra.Vendor]string{
	ondatra.ARISTA:  "Chassis",
	ondatra.CISCO:   "Chassis",
	ondatra.JUNIPER: "CHASSIS0",
	ondatra.NOKIA:   "Chassis",
}

// testArgs holds the objects needed by the test case.
type testArgs struct {
	ctx context.Context
	dut *ondatra.DUTDevice
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Test cases:
//  0) Crash a process in the software using some specific command.
//  1) Issue gnoi healthz grpc call to chassis.
//  2) Validate the device returns relevant information with STATUS_HEALTHY
//  3) Validate the healthz return contains information to debug offline like core file
// Topology:
//   DUT
//
// Test notes:
//. Note: Initiating checkin to experimental
//  - KillProcess system call is used to kill bgp process.
//  - The healthz call needs to be modified to reflect the right component and its path.
//
//  - gnoi operation commands can be sent and tested using CLI command grpcurl.
//    https://github.com/fullstorydev/grpcurl
//

// gNOIKillProcess kills a daemon on the DUT, given its name and pid.
func gNOIKillProcess(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, pName string, pID uint32) {
	gnoiClient := dut.RawAPIs().GNOI().Default(t)
	killRequest := &gnps.KillProcessRequest{Name: pName, Pid: pID, Signal: gnps.KillProcessRequest_SIGNAL_TERM,
		Restart: true}
	killResponse, err := gnoiClient.System().KillProcess(ctx, killRequest)
	t.Logf("Got kill process response: %v\n\n", killResponse)
	if err != nil {
		t.Fatalf("Failed to execute gNOI Kill Process, error received: %v", err)
	}
}

// findProcessByName uses telemetry to find out the PID of a process
func findProcessByName(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, pName string) uint64 {
	pList := gnmi.GetAll(t, dut, gnmi.OC().System().ProcessAny().State())
	var pID uint64
	for _, proc := range pList {
		if proc.GetName() == pName {
			pID = proc.GetPid()
			break
		}
	}
	return pID
}

func TestCopyingDebugFiles(t *testing.T) {

	dut := ondatra.DUT(t, "dut")
	ctx := context.Background()
	gnoiClient := dut.RawAPIs().GNOI().New(t)

	args := &testArgs{
		ctx: ctx,
		dut: dut,
	}

	if _, ok := processNames[dut.Vendor()]; !ok {
		t.Fatalf("Please add support for vendor %v in var processNames", dut.Vendor())
	}

	process := processNames[dut.Vendor()]

	pId := findProcessByName(ctx, t, dut, process)
	if pId == 0 {
		t.Fatalf("Couldn't find pid for process '%s'", process)
	} else {
		t.Logf("Pid of process '%s' is '%d'", process, pId)
	}

	gNOIKillProcess(ctx, t, args.dut, process, uint32(pId))

	// Wait for a bit for gRIBI daemon on the DUT to restart.
	time.Sleep(60 * time.Second)

	componentName := map[string]string{"name": components[dut.Vendor()]}
	req := &hpb.GetRequest{
		Path: &tpb.Path{
			Elem: []*tpb.PathElem{
				{
					Name: "components",
				},
				{
					Name: "component",
					Key:  componentName,
				},
			},
		},
	}
	validResponse, err := gnoiClient.Healthz().Get(context.Background(), req)
	t.Logf("Error: %v", err)
	t.Logf("Response: %v", (validResponse))
	if err != nil {
		t.Fatalf("Unexpected error on healthz get response after restart of %v: %v", process, err)
	}
}
