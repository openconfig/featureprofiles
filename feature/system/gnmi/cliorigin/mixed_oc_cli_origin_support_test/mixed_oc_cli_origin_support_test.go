/*
Copyright 2022 Google Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package gnmi_1_12_mixed_oc_cli_origin_support_test implements GNMI 1.12 from go/wbb:vendor-testplan
package gnmi_1_12_mixed_oc_cli_origin_support_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/openconfig/ygot/ygot"
	"github.com/openconfig/featureprofiles/internal/fptest"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
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
	}
	return ""
}

func buildOCUpdate(path *gpb.Path, value string) *gpb.Update {
	if len(path.GetElem()) == 0 || path.GetElem()[0].GetName() != "meta" {
		path.Origin = "openconfig"
	}
	update := &gpb.Update{
		Path: path,
		Val:  &gpb.TypedValue{Value: &gpb.TypedValue_StringVal{StringVal: value}},
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

func TestOrderDependence(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	dp := dut.Port(t, "port1")
	intfConfig := interfaceDescriptionCLI(dp, "want")
	if intfConfig == "" {
		t.Skip("Vendor is not supported.")
	}
	gnmiClient := dut.RawAPIs().GNMI().Default(t)
	t.Logf("Building the CLI config:\n%s", intfConfig)
	resolvedPath := dut.Config().Interface(dp.Name()).Description()
	path, _, errs := ygot.ResolvePath(resolvedPath)

	gpbSetRequest := &gpb.SetRequest{
		Update: []*gpb.Update{
			buildOCUpdate(path, "not want"),
			buildCLIUpdate(intfConfig),
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

	got := dut.Telemetry().Interface(dp.Name()).Description().Get(t)
	if got != "want" {
		t.Errorf("Get(DUT port description): got %v, want %v", got, "want")
	}
}

func TestMixedOriginOCCLIConfig(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")
	gnmiClient := dut.RawAPIs().GNMI().Default(t)
	intf1Config := interfaceDescriptionCLI(dp1, "foo1")
	t.Logf("Building the CLI config:\n%s", intf1Config)
	resolvedPath := dut.Config().Interface(dp2.Name()).Description()
	path, _, errs := ygot.ResolvePath(resolvedPath)

	gpbSetRequest := &gpb.SetRequest{
		Update: []*gpb.Update{
			buildOCUpdate(path, "foo2"),
			buildCLIUpdate(intf1Config),
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

	if got := dut.Telemetry().Interface(dp1.Name()).Description().Get(t); got != "foo1" {
		t.Errorf("Get(DUT port description): got %v, want %v", got, "foo1")
	}
	if got := dut.Telemetry().Interface(dp2.Name()).Description().Get(t); got != "foo2" {
		t.Errorf("Get(DUT port description): got %v, want %v", got, "foo2")
	}

}
