// Copyright 2026 Cisco Systems, Inc.
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

package binding

import (
	"encoding/json"
	"testing"

	bindpb "github.com/openconfig/featureprofiles/topologies/proto/binding"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/protobuf/encoding/prototext"
)

func TestRestconfJSONBindingOption(t *testing.T) {
	b := &bindpb.Binding{}
	const text = `
duts {
  id: "dut"
  name: "example"
  gnmi { restconf_json: true }
}
`
	if err := prototext.Unmarshal([]byte(text), b); err != nil {
		t.Fatalf("prototext.Unmarshal() = %v", err)
	}
	if got := b.GetDuts(); len(got) != 1 || !got[0].GetGnmi().GetRestconfJson() {
		t.Fatalf("gnmi.restconf_json = %v, want true", got)
	}
}

func TestWrapRestconfLeaf(t *testing.T) {
	up := &gpb.Update{
		Path: pathOf("network-instances", "network-instance", "config", "type"),
		Val:  jsonIETF(`"openconfig-network-instance-types:DEFAULT_INSTANCE"`),
	}
	if err := wrapRestconfUpdate(up); err != nil {
		t.Fatalf("wrapRestconfUpdate() = %v", err)
	}
	got := unmarshalJSON(t, up.GetVal().GetJsonIetfVal())
	want := map[string]any{"type": "openconfig-network-instance-types:DEFAULT_INSTANCE"}
	if diff := jsonDiff(got, want); diff != "" {
		t.Fatalf("leaf payload mismatch: %s", diff)
	}
}

func TestWrapRestconfList(t *testing.T) {
	body := `{
	  "openconfig-interfaces:config": {
	    "description": "To ATE",
	    "name": "Ethernet46",
	    "type": "iana-if-type:ethernetCsmacd"
	  },
	  "openconfig-interfaces:name": "Ethernet46",
	  "openconfig-interfaces:subinterfaces": {
	    "subinterface": [{"index": 0}]
	  }
	}`
	up := &gpb.Update{
		Path: &gpb.Path{Elem: []*gpb.PathElem{{
			Name: "interface",
			Key:  map[string]string{"name": "Ethernet46"},
		}}},
		Val: jsonIETF(body),
	}
	if err := wrapRestconfUpdate(up); err != nil {
		t.Fatalf("wrapRestconfUpdate() = %v", err)
	}
	got := unmarshalJSON(t, up.GetVal().GetJsonIetfVal())
	obj := got.(map[string]any)
	list, ok := obj["openconfig-interfaces:interface"].([]any)
	if !ok || len(list) != 1 {
		t.Fatalf("wrapper key = %#v, want one-element openconfig-interfaces:interface", obj)
	}
}

func TestWrapRestconfSkipsNonJSON(t *testing.T) {
	up := &gpb.Update{
		Path: pathOf("interfaces"),
		Val:  &gpb.TypedValue{Value: &gpb.TypedValue_AsciiVal{AsciiVal: "interface Ethernet46"}},
	}
	if err := wrapRestconfUpdate(up); err != nil {
		t.Fatalf("wrapRestconfUpdate() = %v", err)
	}
	if up.GetVal().GetAsciiVal() != "interface Ethernet46" {
		t.Fatalf("ascii value changed: %#v", up.GetVal())
	}
}

func TestUnwrapRestconfLeaf(t *testing.T) {
	up := &gpb.Update{
		Path: pathOf("interfaces", "interface", "state", "name"),
		Val:  jsonIETF(`{"openconfig-interfaces:name":"Ethernet46"}`),
	}
	unwrapRestconfLeaf(up)
	if got := string(up.GetVal().GetJsonIetfVal()); got != `"Ethernet46"` {
		t.Fatalf("unwrapped leaf = %s, want %q", got, `"Ethernet46"`)
	}
}

func pathOf(names ...string) *gpb.Path {
	p := &gpb.Path{}
	for _, n := range names {
		p.Elem = append(p.Elem, &gpb.PathElem{Name: n})
	}
	return p
}

func jsonIETF(s string) *gpb.TypedValue {
	return &gpb.TypedValue{Value: &gpb.TypedValue_JsonIetfVal{JsonIetfVal: []byte(s)}}
}

func unmarshalJSON(t *testing.T, raw []byte) any {
	t.Helper()
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("json.Unmarshal(%s) = %v", raw, err)
	}
	return v
}

func jsonDiff(got, want any) string {
	gj, _ := json.Marshal(got)
	wj, _ := json.Marshal(want)
	if string(gj) == string(wj) {
		return ""
	}
	return "got " + string(gj) + " want " + string(wj)
}
