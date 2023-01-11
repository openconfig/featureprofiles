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
	"fmt"
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

func interfaceDescriptionCLI(dp *ondatra.Port, desc string) string {
	switch dp.Device().Vendor() {
	case ondatra.ARISTA, ondatra.CISCO:
		const tmpl = `
interface %s
  description %s
`
		return fmt.Sprintf(tmpl, dp.Name(), desc)
	case ondatra.JUNIPER:
		const tmpl = `
interfaces {
    %s {
        description "%s"
    }
}
`
		return fmt.Sprintf(tmpl, dp.Name(), desc)
	}
	return ""
}

func buildOCUpdate(path *gpb.Path, value string) *gpb.Update {
	if len(path.GetElem()) == 0 || path.GetElem()[0].GetName() != "meta" {
		path.Origin = "openconfig"
	}
	jsonVal, _ := ygot.Marshal7951(ygot.String(value))
	update := &gpb.Update{
		Path: path,
		Val:  &gpb.TypedValue{Value: &gpb.TypedValue_JsonIetfVal{JsonIetfVal: jsonVal}},
	}
	return update
}

func buildOCUpdateFromGo(t *testing.T, path *gpb.Path, goStruct ygot.ValidatedGoStruct) *gpb.Update {
	if len(path.GetElem()) == 0 || path.GetElem()[0].GetName() != "meta" {
		path.Origin = "openconfig"
	}
	jsonVal, err := ygot.Marshal7951(goStruct, ygot.JSONIndent("  "), &ygot.RFC7951JSONConfig{AppendModuleName: true, PreferShadowPath: true})
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

// TestCLIBeforeOpenConfig pushes overlapping mixed SetRequest specifying CLI before OpenConfig for DUT port-1.
func TestCLIBeforeOpenConfig(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	dp := dut.Port(t, "port1")

	// `origin: "cli"` - containing vendor configuration.
	intfConfig := interfaceDescriptionCLI(dp, "from cli")
	if intfConfig == "" {
		t.Fatalf("Please add vendor support for %v", dut.Vendor())
	}
	t.Logf("Building the CLI config:\n%s", intfConfig)

	// `origin: ""` (openconfig, default origin) setting the DUT port-1
	//  string value at `/interfaces/interface/config/description` to `"from oc"`.
	resolvedPath := gnmi.OC().Interface(dp.Name()).Description().Config().PathStruct()
	path, _, errs := ygnmi.ResolvePath(resolvedPath)
	if errs != nil {
		t.Fatalf("Could not resolve path: %v", errs)
	}

	gpbSetRequest := &gpb.SetRequest{
		Update: []*gpb.Update{
			buildCLIUpdate(intfConfig),
			buildOCUpdate(path, "from oc"),
		},
	}
	t.Log("gnmiClient Set both CLI and OpenConfig modelled config")
	t.Log(gpbSetRequest)

	gnmiClient := dut.RawAPIs().GNMI().Default(t)
	response, err := gnmiClient.Set(context.Background(), gpbSetRequest)
	if err != nil {
		t.Fatalf("gnmiClient.Set() with unexpected error: %v", err)
	}
	t.Log("gnmiClient Set Response for CLI and OpenConfig modelled config")
	t.Log(response)

	// Validate that DUT port-1 description is `"from cli"`
	got := gnmi.Get(t, dut, gnmi.OC().Interface(dp.Name()).Description().State())
	want := "from cli"
	if got != want {
		t.Errorf("Get(DUT port description): got %v, want %v", got, want)
	}
}

// TestOpenConfigBeforeCLI pushes overlapping mixed SetRequest specifying OpenConfig before CLI for DUT port-1.
func TestOpenConfigBeforeCLI(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	dp := dut.Port(t, "port1")

	// `origin: ""` (openconfig, default origin) setting the DUT port-1
	//  string value at `/interfaces/interface/config/description` to `"from oc"`.
	resolvedPath := gnmi.OC().Interface(dp.Name()).Description().Config().PathStruct()
	path, _, errs := ygnmi.ResolvePath(resolvedPath)
	if errs != nil {
		t.Fatalf("Could not resolve path: %v", errs)
	}

	// `origin: "cli"` - containing vendor configuration.
	intfConfig := interfaceDescriptionCLI(dp, "from cli")
	if intfConfig == "" {
		t.Fatalf("Please add vendor support for %v", dut.Vendor())
	}
	t.Logf("Building the CLI config:\n%s", intfConfig)

	gpbSetRequest := &gpb.SetRequest{
		Update: []*gpb.Update{
			buildOCUpdate(path, "from oc"),
			buildCLIUpdate(intfConfig),
		},
	}
	t.Log("gnmiClient Set both CLI and OpenConfig modelled config")
	t.Log(gpbSetRequest)

	gnmiClient := dut.RawAPIs().GNMI().Default(t)
	response, err := gnmiClient.Set(context.Background(), gpbSetRequest)
	if err != nil {
		t.Fatalf("gnmiClient.Set() with unexpected error: %v", err)
	}
	t.Log("gnmiClient Set Response for CLI and OpenConfig modelled config")
	t.Log(response)

	// Validate that DUT port-1 description is `"from cli"`
	got := gnmi.Get(t, dut, gnmi.OC().Interface(dp.Name()).Description().State())
	want := "from cli"
	if got != want {
		t.Errorf("Get(DUT port description): got %v, want %v", got, want)
	}
}

// Push non-overlapping mixed SetRequest specifying CLI for DUT port-1 and
// OpenConfig for DUT port-2
func TestMixedOriginOCCLIConfig(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")
	gnmiClient := dut.RawAPIs().GNMI().Default(t)
	// `origin: "cli"` - containing vendor configuration.
	intf1Config := interfaceDescriptionCLI(dp1, "foo1")
	if intf1Config == "" {
		t.Fatalf("Please add vendor support for %v", dut.Vendor())
	}
	t.Logf("Building the CLI config:\n%s", intf1Config)

	// `origin: ""` (openconfig, default origin) setting the DUT port-2
	//  string value at `/interfaces/interface/config/description` to `"foo2"`.
	resolvedPath := gnmi.OC().Interface(dp2.Name()).Description().Config().PathStruct()
	path, _, errs := ygnmi.ResolvePath(resolvedPath)

	gpbSetRequest := &gpb.SetRequest{
		Update: []*gpb.Update{
			buildCLIUpdate(intf1Config),
			buildOCUpdate(path, "foo2"),
		},
	}
	if errs != nil {
		t.Fatalf("Could not resolve path: %v", errs)
	}
	t.Log("gnmiClient Set both CLI and OpenConfig modelled config")
	t.Log(gpbSetRequest)

	response, err := gnmiClient.Set(context.Background(), gpbSetRequest)
	if err != nil {
		t.Fatalf("gnmiClient.Set() with unexpected error: %v", err)
	}
	t.Log("gnmiClient Set Response for CLI and OpenConfig modelled config")
	t.Log(response)

	// Validate that DUT port-1 and DUT port-2 description through telemetry.
	if got := gnmi.Get(t, dut, gnmi.OC().Interface(dp1.Name()).Description().State()); got != "foo1" {
		t.Errorf("Get(DUT port description): got %v, want %v", got, "foo1")
	}
	if got := gnmi.Get(t, dut, gnmi.OC().Interface(dp2.Name()).Description().State()); got != "foo2" {
		t.Errorf("Get(DUT port description): got %v, want %v", got, "foo2")
	}
}

type testCase struct {
	cmdCfg           string
	queueName        string
	forwardGroupName string
	reorderFunc      func(input []*gpb.Update) []*gpb.Update
	okToFail         bool
}

func testQoSWithCLIAndOCUpdates(t *testing.T, tCase testCase) {
	dut := ondatra.DUT(t, "dut")

	// We use ARISTA-specific command-line commands here. Other vendor devices won't accept it.
	if dut.Vendor() != ondatra.ARISTA {
		t.Skipf("Unsupported vendor device: got %v, expected %v", dut.Vendor(), ondatra.ARISTA)
	}

	t.Logf("Generated an update for the CLI config:\n%s", tCase.cmdCfg)

	qosPath := gnmi.OC().Qos().Config().PathStruct()
	resolvedQosPath, _, err := ygnmi.ResolvePath(qosPath)
	if err != nil {
		t.Fatalf("Could not resolve QoS path: %v", err)
	}

	// Make sure the current config does not contain new data already.
	if existingQueue := gnmi.LookupConfig(t, dut, gnmi.OC().Qos().Queue(tCase.queueName).Config()); existingQueue.IsPresent() {
		t.Errorf("Detected an existing %v queue. This is unexpected.", tCase.queueName)
	}

	// Creating OC addition to the config.
	r := &oc.Root{}
	qos := r.GetOrCreateQos()
	qos.GetOrCreateQueue(tCase.queueName)
	fg := qos.GetOrCreateForwardingGroup(tCase.forwardGroupName)
	fg.SetOutputQueue(tCase.queueName)

	fptest.LogQuery(t, "QoS update for the OC config:", gnmi.OC().Qos().Config(), qos)

	update1 := buildCLIUpdate(tCase.cmdCfg)
	update2 := buildOCUpdateFromGo(t, resolvedQosPath, qos)
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
	gotQueue := gnmi.GetConfig(t, dut, gnmi.OC().Qos().Queue(tCase.queueName).Config())
	if got := gotQueue.GetName(); got != tCase.queueName {
		t.Errorf("Get(DUT queue name): got %v, want %v", got, tCase.queueName)
	}
	gotFG := gnmi.GetConfig(t, dut, gnmi.OC().Qos().ForwardingGroup(tCase.forwardGroupName).Config())
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
		cmdCfg: `
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
		cmdCfg: `
qos traffic-class 1 name target-group-BE1
qos tx-queue 1 name BE1
	`,
		queueName:        "BE1",
		forwardGroupName: "target-group-BE1",
		reorderFunc:      swap,
		okToFail:         true,
	})
}
