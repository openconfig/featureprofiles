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

// Package mixed_oc_cli_origin_support_test implements GNMI 1.12 from go/wbb:vendor-testplan
package mixed_oc_cli_origin_support_test

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ygnmi/schemaless"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

type testCase struct {
	cliConfig        string
	queueName        string
	forwardGroupName string
}

// showRunningConfig returns the output of 'show running-config' on the device.
func showRunningConfig(t *testing.T, dut *ondatra.DUTDevice) string {
	t.Helper()
	runningConfig, err := dut.RawAPIs().CLI(t).SendCommand(context.Background(), "show running-config")
	if err != nil {
		t.Fatalf("'show running-config' failed: %v", err)
	}
	return runningConfig
}

func testQoSWithCLIAndOCUpdates(t *testing.T, dut *ondatra.DUTDevice, tCase testCase) {
	qosPath := gnmi.OC().Qos()

	t.Logf("Step 1: Make sure QoS queue under test is not already set.")
	// Make sure the current config does not contain new data already.
	if existingQueue := gnmi.LookupConfig(t, dut, qosPath.Queue(tCase.queueName).Config()); existingQueue.IsPresent() {
		t.Fatalf("Detected an existing %v queue. This is unexpected.", tCase.queueName)
	}

	t.Logf("Step 2: Retrieve current root OC config")
	runningConfig := showRunningConfig(t, dut)
	r := gnmi.GetConfig(t, dut, gnmi.OC().Config())

	t.Logf("Step 3: Test that replacing device with current config is accepted and is a no-op.")
	result := gnmi.Replace(t, dut, gnmi.OC().Config(), r)
	t.Logf("gnmi.Replace on root response: %+v", result.RawResponse)

	// Create OC addition to the config.
	qos := r.GetOrCreateQos()
	qos.GetOrCreateQueue(tCase.queueName)
	qos.GetOrCreateForwardingGroup(tCase.forwardGroupName).SetOutputQueue(tCase.queueName)

	fptest.LogQuery(t, "QoS update for the OC config:", qosPath.Config(), qos)

	// Create and apply mixed CLI+OC SetRequest.
	cliPath, err := schemaless.NewConfig[string]("", "cli")
	if err != nil {
		t.Fatalf("Failed to create CLI ygnmi query: %v", err)
	}

	t.Logf("Step 4: Send mixed-origin SetRequest")
	mixedQuery := &gnmi.SetBatch{}
	gnmi.BatchReplace(mixedQuery, gnmi.OC().Config(), r)
	gnmi.BatchUpdate(mixedQuery, cliPath, tCase.cliConfig)
	result = mixedQuery.Set(t, dut)

	t.Logf("gnmiClient.Set() response: %+v", result.RawResponse)

	t.Logf("Step 5: Verify QoS queue configuration has been accepted by the target")

	// Validate CLI has changed
	newRunningConfig := showRunningConfig(t, dut)
	diff := cmp.Diff(runningConfig, newRunningConfig)
	t.Logf("running config (-old, +new):\n%s", cmp.Diff(runningConfig, newRunningConfig))
	if diff == "" {
		t.Errorf("CLI running-config did not change as expected after mixed-origin SetRequest.")
	}

	// Validate new OC config has been accepted.
	gotQueue := gnmi.GetConfig(t, dut, qosPath.Queue(tCase.queueName).Config())
	if got := gotQueue.GetName(); got != tCase.queueName {
		t.Errorf("Get(DUT queue name): got %v, want %v", got, tCase.queueName)
	}
	gotFG := gnmi.GetConfig(t, dut, qosPath.ForwardingGroup(tCase.forwardGroupName).Config())
	if got := gotFG.GetName(); got != tCase.forwardGroupName {
		t.Errorf("Get(DUT forwarding group name): got %v, want %v", got, tCase.forwardGroupName)
	}
	if got := gotFG.GetOutputQueue(); got != tCase.queueName {
		t.Errorf("Get(DUT forwarding group output queue): got %v, want %v", got, tCase.queueName)
	}
}

func TestQoSDependentCLI(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	var cliConfig string
	// TODO: additional vendor CLI to be added if and when necessary for compatibility with the OC QoS configuration.
	switch vendor := dut.Vendor(); vendor {
	case ondatra.ARISTA:
		cliConfig = `qos traffic-class 0 name target-group-BE0
qos tx-queue 0 name BE0`
	default:
		t.Skipf("Unsupported vendor device: %v", vendor)
	}

	testQoSWithCLIAndOCUpdates(t, dut, testCase{
		cliConfig:        cliConfig,
		queueName:        "BE0",
		forwardGroupName: "target-group-BE0",
	})
}
