// Copyright 2024 Google Inc. All Rights Reserved.
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

package p4rt_replay_test

import (
	"context"
	"flag"
	"testing"

	"github.com/cisco-open/go-p4/utils"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/p4rtutils"
	gpb "github.com/openconfig/gribi/v1/proto/service"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/replayer"
	"github.com/openconfig/ygot/ygot"
)

// Flag variable definitions
var (
	p4InfoFile = flag.String("p4info_file_location", "../../../p4rt/wbb.p4info.pb.txt", "Path to the p4info file.")
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// configureDeviceID configures p4rt device-id on the DUT.
func configureDeviceID(t *testing.T, dut *ondatra.DUTDevice) {
	nodes := p4rtutils.P4RTNodesByPort(t, dut)
	p4rtNode, ok := nodes["port1"]
	if !ok {
		t.Fatal("Couldn't find P4RT Node for port: port1")
	}
	c := oc.Component{}
	c.Name = ygot.String(p4rtNode)
	c.IntegratedCircuit = &oc.Component_IntegratedCircuit{}
	c.IntegratedCircuit.NodeId = ygot.Uint64(1)
	t.Logf("Configuring P4RT Node %s", p4rtNode)
	gnmi.Replace(t, dut, gnmi.OC().Component(p4rtNode).Config(), &c)
}

func TestReplay(t *testing.T) {
	const logFile = "https://storage.googleapis.com/featureprofiles-binarylogs/p4rt_replay.pb"
	t.Logf("Parsing log file: %v", logFile)
	rec := replayer.ParseURL(t, logFile)

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

	configureDeviceID(t, dut)

	p4Info, err := utils.P4InfoLoad(p4InfoFile)
	if err != nil {
		t.Fatalf("wbbp4info.Get(): failed to get P4 Info file: %v", err)
	}

	t.Logf("Creating gRPC clients to dut")
	cfg := &replayer.Config{
		GNMI:   dut.RawAPIs().GNMI(t),
		GRIBI:  dut.RawAPIs().GRIBI(t),
		P4RT:   dut.RawAPIs().P4RT(t),
		P4Info: p4Info,
	}

	t.Logf("Replaying parsed log to device %v", dut.Name())
	ctx := context.Background()
	results := replayer.Replay(ctx, t, rec, cfg)

	// Validate that all gRIBI requests were programmed successfully.
	for _, result := range results.GRIBI() {
		if result.OperationID > 0 && result.ProgrammingResult != gpb.AFTResult_FIB_PROGRAMMED && result.ProgrammingResult != gpb.AFTResult_RIB_PROGRAMMED {
			t.Errorf("Replay(): gRIBI Result failed: %v", result)
		}
	}

	// Validate that resulting gRIBI state matches the recorded one.
	if diff := replayer.GRIBIDiff(rec, results); diff != "" {
		t.Errorf("Replay(): unexpected diff in final gRIBI state (-want,+got): %v", diff)
	}

	for i, result := range results.GNMI() {
		t.Logf("Result [%v]: %v", i, result)
	}
}
