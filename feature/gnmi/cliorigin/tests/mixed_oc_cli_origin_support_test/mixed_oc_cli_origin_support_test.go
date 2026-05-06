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
	"testing"

	"github.com/openconfig/ygnmi/ygnmi"

	"github.com/openconfig/ondatra/gnmi/oc"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ygnmi/schemaless"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

type testCase struct {
	addNonOcBatchUpdateF func(t *testing.T, mixedQuery *gnmi.SetBatch)
	queueName            string
	forwardGroupName     string
}

func prepareDut(t *testing.T, dut *ondatra.DUTDevice, queueName string) {
	t.Logf("Step 1: Delete and make sure QoS queue under test is not already set.")

	qosPath := gnmi.OC().Qos()

	gnmi.Delete(t, dut, qosPath.Queue(queueName).Config())

	if existingQueue := gnmi.LookupConfig(t, dut, qosPath.Queue(queueName).Config()); existingQueue.IsPresent() {
		t.Fatalf("Detected an existing %v queue. This is unexpected.", queueName)
	}
}

func replaceAsNoOp(t *testing.T, dut *ondatra.DUTDevice, subTreeReplace bool) *oc.Root {
	t.Logf("Step 2: Retrieve current root OC config")

	ocConfig := gnmi.Get[*oc.Root](t, dut, gnmi.OC().Config())

	qosPath := gnmi.OC().Qos()
	qosConfig := ocConfig.GetOrCreateQos()

	fptest.LogQuery(t, "QoS update for the OC config:", qosPath.Config(), qosConfig)

	t.Logf("Step 3: Test that replacing device with current config is accepted and is a no-op.")
	var result *ygnmi.Result
	if subTreeReplace {
		result = gnmi.Replace(t, dut, qosPath.Config(), qosConfig)
	} else {
		result = gnmi.Replace(t, dut, gnmi.OC().Config(), ocConfig)
	}

	t.Logf("gnmi.Replace on root response: %+v", result.RawResponse)

	return ocConfig
}

func sendMixedOriginRequest(
	t *testing.T,
	dut *ondatra.DUTDevice,
	ocConfig *oc.Root,
	tCase testCase,
	subTreeReplace bool,
) {
	t.Logf("Step 4: Construct and send mixed-origin SetRequest")

	// Create OC addition to the config.
	qosConfig := ocConfig.GetOrCreateQos()

	qosConfig.GetOrCreateQueue(tCase.queueName)
	qosConfig.GetOrCreateForwardingGroup(tCase.forwardGroupName).SetOutputQueue(tCase.queueName)

	qosPath := gnmi.OC().Qos()

	fptest.LogQuery(t, "QoS update for the OC config:", qosPath.Config(), qosConfig)

	mixedQuery := &gnmi.SetBatch{}
	if subTreeReplace {
		gnmi.BatchReplace(mixedQuery, qosPath.Config(), qosConfig)
	} else {
		gnmi.BatchReplace(mixedQuery, gnmi.OC().Config(), ocConfig)
	}

	tCase.addNonOcBatchUpdateF(t, mixedQuery)

	result := mixedQuery.Set(t, dut)

	t.Logf("gnmiClient.Set() response: %+v", result.RawResponse)
}

func verifyChanges(t *testing.T, dut *ondatra.DUTDevice, tCase testCase) {
	t.Logf("Step 5: Verify QoS queue configuration has been accepted by the target")

	qosPath := gnmi.OC().Qos()

	qosPath.Queue(tCase.queueName).Config().PathStruct()

	// Validate new OC config has been accepted.
	gotQueue := gnmi.Get[*oc.Qos_Queue](t, dut, qosPath.Queue(tCase.queueName).Config())
	if got := gotQueue.GetName(); got != tCase.queueName {
		t.Errorf("Get(DUT queue name): got %v, want %v", got, tCase.queueName)
	}
	gotFG := gnmi.Get[*oc.Qos_ForwardingGroup](t, dut, qosPath.ForwardingGroup(tCase.forwardGroupName).Config())
	if got := gotFG.GetName(); got != tCase.forwardGroupName {
		t.Errorf("Get(DUT forwarding group name): got %v, want %v", got, tCase.forwardGroupName)
	}
	if got := gotFG.GetOutputQueue(); got != tCase.queueName {
		t.Errorf("Get(DUT forwarding group output queue): got %v, want %v", got, tCase.queueName)
	}
}

// testQoSWithCLIAndOCUpdates carries out a mixed-origin test for a QoS test case.
//
// TODO: If a truly permanent example of a mutual dependency between CLI and OC
// exists that must be modelled partially in CLI even as OC modelling continues
// to mature, then consider changing this test to that instead.
func testQoSWithCLIAndOCUpdates(
	t *testing.T,
	dut *ondatra.DUTDevice,
	tCase testCase,
	subTreeReplace bool,
) {
	prepareDut(t, dut, tCase.queueName)

	ocConfig := replaceAsNoOp(t, dut, subTreeReplace)

	sendMixedOriginRequest(t, dut, ocConfig, tCase, subTreeReplace)

	verifyChanges(t, dut, tCase)
}

func TestQoSDependentCLIFullReplace(t *testing.T) {
	// TODO: Skipping this test case because it is not required to pass right now.
	t.Skip()
	dut := ondatra.DUT(t, "dut")

	testQoSWithCLIAndOCUpdates(t, dut, getTestcase(t, dut), false)
}

func TestQoSDependentCLISubtreeReplace(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	testQoSWithCLIAndOCUpdates(t, dut, getTestcase(t, dut), true)
}

func getTestcase(t *testing.T, dut *ondatra.DUTDevice) testCase {
	tc := testCase{
		queueName:        "TEST",
		forwardGroupName: "target-group-TEST",
	}

	switch vendor := dut.Vendor(); vendor {
	case ondatra.ARISTA:
		tc.addNonOcBatchUpdateF = func(t *testing.T, mixedQuery *gnmi.SetBatch) {
			nonOCConfigPath, err := schemaless.NewConfig[string]("", "cli")
			if err != nil {
				t.Fatalf("Failed to create CLI ygnmi query: %v", err)
			}

			nonOCConfig := `qos traffic-class 0 name target-group-TEST
qos tx-queue 0 name TEST`

			gnmi.BatchUpdate(mixedQuery, nonOCConfigPath, nonOCConfig)
		}
	case ondatra.NOKIA:
		// nokia deviates and uses native yang model rather than cli since its all the same thing
		// for srlinux
		tc.addNonOcBatchUpdateF = func(t *testing.T, mixedQuery *gnmi.SetBatch) {
			nonOCConfigPath, err := schemaless.NewConfig[map[string]interface{}]("/qos", "srl_nokia")
			if err != nil {
				t.Fatalf("Failed to create CLI ygnmi query: %v", err)
			}

			nonOCConfigJSON := map[string]interface{}{
				"queues": map[string]interface{}{
					"queue": []map[string]interface{}{
						{
							"name":        "TEST",
							"queue-index": 0,
						},
					},
				},
			}
			gnmi.BatchUpdate(mixedQuery, nonOCConfigPath, nonOCConfigJSON)
		}
	default:
		t.Skipf("Unsupported vendor device: %v", vendor)
	}

	return tc
}
