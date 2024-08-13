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
	"github.com/openconfig/featureprofiles/internal/system"
	hpb "github.com/openconfig/gnoi/healthz"
	spb "github.com/openconfig/gnoi/system"
	tpb "github.com/openconfig/gnoi/types"
	"github.com/openconfig/ondatra"
)

var (
	processName = map[ondatra.Vendor]string{
		ondatra.NOKIA:   "sr_bgp_mgr",
		ondatra.ARISTA:  "IpRib",
		ondatra.JUNIPER: "rpd",
	}
	components = map[ondatra.Vendor]string{
		ondatra.ARISTA:  "Chassis",
		ondatra.CISCO:   "Chassis",
		ondatra.JUNIPER: "CHASSIS0",
		ondatra.NOKIA:   "Chassis",
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
// Note: Initiating checkin to experimental
//  - KillProcess system call is used to kill a process.
//  - The healthz call needs to be modified to reflect the right component and its path.
//
//  - gnoi operation commands can be sent and tested using CLI command grpcurl.
//    https://github.com/fullstorydev/grpcurl
//

func TestCopyingDebugFiles(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	gnoiClient := dut.RawAPIs().GNOI(t)
	if _, ok := processName[dut.Vendor()]; !ok {
		t.Fatalf("Please add support for vendor %v in var processName", dut.Vendor())
	}
	pID := system.FindProcessIDByName(t, dut, processName[dut.Vendor()])
	if pID == 0 {
		t.Fatalf("process %v not found on device", processName[dut.Vendor()])
	}
	killProcessRequest := &spb.KillProcessRequest{
		Signal:  spb.KillProcessRequest_SIGNAL_KILL,
		Name:    processName[dut.Vendor()],
		Pid:     uint32(pID),
		Restart: true,
	}
	processKillResponse, err := gnoiClient.System().KillProcess(context.Background(), killProcessRequest)
	if err != nil {
		t.Fatalf("Failed to restart process %v with unexpected err: %v", processName[dut.Vendor()], err)
	}

	t.Logf("gnoiClient.System().KillProcess() response: %v, err: %v", processKillResponse, err)
	t.Logf("Wait 60 seconds for process to restart ...")
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
	switch dut.Vendor() {
	case ondatra.ARISTA:
		t.Log("Skip logging validResponse for Arista")
	default:
		t.Logf("Response: %v", (validResponse))
	}
	if err != nil {
		t.Fatalf("Unexpected error on healthz get response after restart of %v: %v", processName[dut.Vendor()], err)
	}
}

func TestChassisComponentArtifacts(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	gnoiClient := dut.RawAPIs().GNOI(t)
	componentName := map[string]string{"name": components[dut.Vendor()]}
	// Execute Healthz Check RPC for the chassis component.
	chkReq := &hpb.CheckRequest{
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
	t.Logf("Executing Healthz Check RPC for component %v", componentName["name"])
	chkRes, err := gnoiClient.Healthz().Check(context.Background(), chkReq)
	if err != nil {
		t.Fatalf("Unexpected error on executing Healthz Check RPC: %v", err)
	}
	// Fetch artifact related metadata that was returned in the Check Response.
	artifacts := chkRes.GetStatus().GetArtifacts()
	if len(artifacts) == 0 {
		t.Fatalf("Artifacts received for component %v after executing Healthz Check RPC - want non-nil, got nil", componentName["name"])
	}
	t.Logf("Artifacts received for component %v: %v", componentName["name"], artifacts)
	// Fetch artifact details by executing ArtifactRequest and passing the artifact ID along.
	for _, artifact := range artifacts {
		artID := artifact.GetId()
		t.Logf("Executing ArtifactRequest for artifact ID %v", artID)
		artReq := &hpb.ArtifactRequest{
			Id: artID,
		}
		// Verify that a valid response is received.
		artRes, err := gnoiClient.Healthz().Artifact(context.Background(), artReq)
		if err != nil {
			t.Fatalf("Unexpected error on executing Healthz Artifact RPC: %v", err)
		}
		h1, err := artRes.Header()
		t.Logf("Header of artifact %v: %v", artID, h1)
		if err != nil {
			t.Fatalf("Unexpected error when fetching the header of artifact %v: %v", artID, err)
		}
	}
}
