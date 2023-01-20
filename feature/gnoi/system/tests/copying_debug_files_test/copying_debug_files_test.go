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
	spb "github.com/openconfig/gnoi/system"
	tpb "github.com/openconfig/gnoi/types"
	"github.com/openconfig/ondatra"
)

var (
	bgpProcName = map[ondatra.Vendor]string{
		ondatra.NOKIA:  "sr_bgp_mgr",
		ondatra.ARISTA: "bgp",
	}
)

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

func TestCopyingDebugFiles(t *testing.T) {

	dut := ondatra.DUT(t, "dut")
	gnoiClient := dut.RawAPIs().GNOI().New(t)
	if _, ok := bgpProcName[dut.Vendor()]; !ok {
		t.Fatalf("Please add support for vendor %v in var bgpProcName", dut.Vendor())
	}
	killProcessRequest := &spb.KillProcessRequest{
		Signal:  spb.KillProcessRequest_SIGNAL_KILL,
		Name:    bgpProcName[dut.Vendor()],
		Restart: true,
	}
	processKillResponse, err := gnoiClient.System().KillProcess(context.Background(), killProcessRequest)
	if err != nil {
		t.Fatalf("Failed to restart process %v with unexpected err: %v", bgpProcName[dut.Vendor()], err)
	}

	t.Logf("gnoiClient.System().KillProcess() response: %v, err: %v", processKillResponse, err)
	t.Logf("Wait 60 seconds for process to restart ...")
	time.Sleep(60 * time.Second)

	componentName := map[string]string{"name": "Chassis"}
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
		t.Fatalf("Unexpected error on healthz get response after restart of %v: %v", bgpProcName[dut.Vendor()], err)
	}
}
