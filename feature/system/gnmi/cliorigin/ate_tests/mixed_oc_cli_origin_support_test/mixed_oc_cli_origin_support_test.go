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
	resolvedPath := dut.Config().Interface(dp.Name()).Description()
	path, _, errs := ygot.ResolvePath(resolvedPath)
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

	// Validate that DUT port-1 description is `"from oc"`
	got := dut.Telemetry().Interface(dp.Name()).Description().Get(t)
	want := "from oc"
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
	resolvedPath := dut.Config().Interface(dp.Name()).Description()
	path, _, errs := ygot.ResolvePath(resolvedPath)
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
	got := dut.Telemetry().Interface(dp.Name()).Description().Get(t)
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
	resolvedPath := dut.Config().Interface(dp2.Name()).Description()
	path, _, errs := ygot.ResolvePath(resolvedPath)

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
	if got := dut.Telemetry().Interface(dp1.Name()).Description().Get(t); got != "foo1" {
		t.Errorf("Get(DUT port description): got %v, want %v", got, "foo1")
	}
	if got := dut.Telemetry().Interface(dp2.Name()).Description().Get(t); got != "foo2" {
		t.Errorf("Get(DUT port description): got %v, want %v", got, "foo2")
	}

}
