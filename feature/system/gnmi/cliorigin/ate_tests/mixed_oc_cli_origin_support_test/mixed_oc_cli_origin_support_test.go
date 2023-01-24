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

	"github.com/openconfig/featureprofiles/internal/fptest"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func buildOCUpdate(t *testing.T, ps ygnmi.PathStruct, goStruct ygot.ValidatedGoStruct, configPath bool) *gpb.Update {
	t.Helper()
	path, _, err := ygnmi.ResolvePath(ps)
	if err != nil {
		t.Fatalf("Could not resolve QoS path: %v", err)
	}

	if len(path.GetElem()) == 0 || path.GetElem()[0].GetName() != "meta" {
		path.Origin = "openconfig"
	}
	jsonVal, err := ygot.Marshal7951(goStruct, ygot.JSONIndent("  "), &ygot.RFC7951JSONConfig{AppendModuleName: true, PreferShadowPath: configPath})
	if err != nil {
		t.Fatalf("Error while marshalling a goStruct: %v", err)
	}
	update := &gpb.Update{
		Path: path,
		Val:  &gpb.TypedValue{Value: &gpb.TypedValue_JsonIetfVal{JsonIetfVal: jsonVal}},
	}
	return update
}

func buildCLIUpdate(value string) *gpb.Update {
	update := &gpb.Update{
		Path: &gpb.Path{
			Origin: "cli",
			Elem:   []*gpb.PathElem{},
		},
		Val: &gpb.TypedValue{
			Value: &gpb.TypedValue_AsciiVal{
				AsciiVal: value,
			},
		},
	}
	return update
}

type testCase struct {
	cliConfig        string
	queueName        string
	forwardGroupName string
	// reorderFunc orders the input updates a certain way for the test case.
	reorderFunc func(input []*gpb.Update) []*gpb.Update
	okToFail    bool
}

func testQoSWithCLIAndOCUpdates(t *testing.T, tCase testCase) {
	dut := ondatra.DUT(t, "dut")

	// We use ARISTA-specific command-line commands here. Other vendor devices won't accept it.
	if dut.Vendor() != ondatra.ARISTA {
		t.Skipf("Unsupported vendor device: got %v, expected %v", dut.Vendor(), ondatra.ARISTA)
	}

	t.Logf("Generated an update for the CLI config:\n%s", tCase.cliConfig)

	qosPath := gnmi.OC().Qos()

	// Make sure the current config does not contain new data already.
	if existingQueue := gnmi.LookupConfig(t, dut, qosPath.Queue(tCase.queueName).Config()); existingQueue.IsPresent() {
		t.Errorf("Detected an existing %v queue. This is unexpected.", tCase.queueName)
	}

	// Creating OC addition to the config.
	r := &oc.Root{}
	qos := r.GetOrCreateQos()
	qos.GetOrCreateQueue(tCase.queueName)
	qos.GetOrCreateForwardingGroup(tCase.forwardGroupName).SetOutputQueue(tCase.queueName)

	fptest.LogQuery(t, "QoS update for the OC config:", qosPath.Config(), qos)

	update1 := buildCLIUpdate(tCase.cliConfig)
	update2 := buildOCUpdate(t, qosPath.Config().PathStruct(), qos, true)
	gpbSetRequest := &gpb.SetRequest{Update: tCase.reorderFunc([]*gpb.Update{update1, update2})}

	t.Log("Calling gnmiClient.Set() CLI + OpenConfig config:")
	t.Log(gpbSetRequest)

	gnmiClient := dut.RawAPIs().GNMI().Default(t)
	response, err := gnmiClient.Set(context.Background(), gpbSetRequest)
	if err != nil {
		if tCase.okToFail {
			t.Skipf("gnmiClient.Set() with unexpected error: %v\nSkipping as this test has okToFail: %v", err, tCase.okToFail)
		} else {
			t.Fatalf("gnmiClient.Set() with unexpected error: %v", err)
		}
	}
	t.Log("gnmiClient.Set() response:\n", response)

	// Validate
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

var (
	passthru = func(input []*gpb.Update) []*gpb.Update {
		return []*gpb.Update{input[0], input[1]}
	}
	swap = func(input []*gpb.Update) []*gpb.Update {
		return []*gpb.Update{input[1], input[0]}
	}
)

func TestQoSDependentCLIThenOC(t *testing.T) {
	testQoSWithCLIAndOCUpdates(t, testCase{
		cliConfig: `
qos traffic-class 0 name target-group-BE0
qos tx-queue 0 name BE0
	`,
		queueName:        "BE0",
		forwardGroupName: "target-group-BE0",
		reorderFunc:      passthru,
		okToFail:         false,
	})
}

// This test (dependent OC preceding CLI) is not required to succeed.
func TestQoSDependentOCThenCLI(t *testing.T) {
	testQoSWithCLIAndOCUpdates(t, testCase{
		cliConfig: `
qos traffic-class 1 name target-group-BE1
qos tx-queue 1 name BE1
	`,
		queueName:        "BE1",
		forwardGroupName: "target-group-BE1",
		reorderFunc:      swap,
		okToFail:         true,
	})
}
