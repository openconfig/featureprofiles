// Copyright 2023 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package presession_test

import (
	"context"
	"testing"

	"github.com/openconfig/featureprofiles/internal/fptest"
	gpb "github.com/openconfig/gribi/v1/proto/service"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/replayer"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestReplay(t *testing.T) {
	const logFile = "https://github.com/openconfig/featureprofiles/raw/main/feature/experimental/replay/tests/presession_test/grpclog.pb"
	t.Logf("Parsing log file: %v", logFile)
	rec, err := replayer.ParseURL(logFile)
	if err != nil {
		t.Fatalf("Parse(): cannot parse log file: %v", err)
	}

	dut := ondatra.DUT(t, "dut")
	portMap := map[string]string{}
	intfs, err := rec.Interfaces()
	if err != nil {
		t.Fatalf("Interfaces(): cannot get interfaces: %v", err)
	}

	// This test only needs Port-Channel4, so remap only those ports to the dut's reserved ports.
	available := dut.Ports()
	t.Logf("PortChannel4: %v", intfs["Port-Channel4"])
	for _, member := range intfs["Port-Channel4"] {
		if len(available) == 0 {
			t.Fatalf("Ports(): not enough ports to satisfy Port-Channel4 remapping, members: %v", intfs["Port-Channel4"])
		}
		portMap[member.Name] = available[0].Name()
		available = available[1:]
	}

	if err := rec.SetInterfaceMap(portMap); err != nil {
		t.Fatalf("Transform(%v): cannot transform log: %v", portMap, err)
	}

	t.Logf("Creating gRPC clients to dut")
	clients := &replayer.Clients{
		GNMI:  dut.RawAPIs().GNMI(t),
		GRIBI: dut.RawAPIs().GRIBI(t),
	}

	t.Logf("Replaying parsed log to device %v", dut.Name())
	ctx := context.Background()
	results, err := replayer.Replay(ctx, rec, clients)
	if err != nil {
		t.Fatalf("Replay(): got error replaying record: %v", err)
	}

	// Validate that all gRIBI requests were programmed successfully.
	for _, result := range results.GRIBI() {
		if result.OperationID > 0 && result.ProgrammingResult != gpb.AFTResult_FIB_PROGRAMMED && result.ProgrammingResult != gpb.AFTResult_RIB_PROGRAMMED {
			t.Errorf("Replay(): gRIBI Result failed: %v", result)
		}
	}

	// Validate that resulting gRIBI state matches the recorded one.
	if diff := results.GRIBIDiff(rec); diff != "" {
		t.Errorf("Replay(): unexpected diff in final gRIBI state (-want,+got): %v", diff)
	}

	for i, result := range results.GNMI() {
		t.Logf("Result [%v]: %v", i, result)
	}
}
