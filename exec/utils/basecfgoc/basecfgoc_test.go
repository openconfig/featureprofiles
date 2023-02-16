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
	"fmt"
	"strings"
	"testing"

	"github.com/openconfig/featureprofiles/internal/fptest"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	ognmi "github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
	"google.golang.org/protobuf/encoding/prototext"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestGenConfig(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ocRoot := &oc.Root{}
	for _, p := range dut.Ports() {
		intf := ocRoot.GetOrCreateInterface(p.Name())
		intf.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
		intf.GetOrCreateSubinterface(0)
	}

	fmt.Printf("%s", prettySetRequest(t, ognmi.OC(), ocRoot))

}

// prettySetRequest returns a string version of a gNMI SetRequest for human
// consumption and ignores errors. Note that the output is subject to change.
// See documentation for prototext.Format.
func prettySetRequest(t *testing.T, pathStruct ygnmi.PathStruct, ocVal interface{}) string {
	path, _, errs := ygnmi.ResolvePath(pathStruct)
	if errs != nil {
		t.Fatalf("Could not resolve the path; %v", errs)
	}
	path.Target = ""
	path.Origin = "openconfig"

	ocJSONVal, err := ygot.Marshal7951(ocVal, ygot.JSONIndent("  "), &ygot.RFC7951JSONConfig{AppendModuleName: true, PreferShadowPath: true})
	if err != nil {
		t.Fatalf("Could not encode value (ocVal) into JSON format; %v", err)
	}
	ocReplaceReq := &gpb.Update{
		Path: path,
		Val: &gpb.TypedValue{
			Value: &gpb.TypedValue_JsonIetfVal{
				JsonIetfVal: ocJSONVal,
			},
		},
	}

	setRequest := &gpb.SetRequest{
		Update: []*gpb.Update{ocReplaceReq},
	}

	var buf strings.Builder
	fmt.Fprintf(&buf, "SetRequest:\n%s\n", prototext.Format(setRequest))

	writePath := func(path *gpb.Path) {
		pathStr, err := ygot.PathToString(path)
		if err != nil {
			pathStr = prototext.Format(path)
		}
		fmt.Fprintf(&buf, "%s\n", pathStr)
	}

	writeVal := func(val *gpb.TypedValue) {
		switch v := val.Value.(type) {
		case *gpb.TypedValue_JsonIetfVal:
			fmt.Fprintf(&buf, "%s\n", v.JsonIetfVal)
		default:
			fmt.Fprintf(&buf, "%s\n", prototext.Format(val))
		}
	}

	for i, path := range setRequest.Delete {
		fmt.Fprintf(&buf, "-------delete path #%d------\n", i)
		writePath(path)
	}
	for i, update := range setRequest.Replace {
		fmt.Fprintf(&buf, "-------replace path/value pair #%d------\n", i)
		writePath(update.Path)
		writeVal(update.Val)
	}
	for i, update := range setRequest.Update {
		fmt.Fprintf(&buf, "-------update path/value pair #%d------\n", i)
		writePath(update.Path)
		writeVal(update.Val)
	}
	return buf.String()
}
