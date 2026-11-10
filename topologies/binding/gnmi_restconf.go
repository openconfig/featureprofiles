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
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra/binding/introspect"
	"google.golang.org/grpc"
)

// restconfGNMI rewrites gNMI JSON for SONiC translib.
//
// Translib unmarshals an Update or Replace as RESTCONF: the payload is a
// map keyed by the last path element, and a list node is a JSON array.
// ygnmi sends the bare value, which translib rejects with errors such as
// "got type string inside container" or "JSON contains unexpected field".
// Other NOS accept the bare value, so the rewrite is opt-in. A SONiC binding
// sets gnmi.restconf_json, which covers every test that uses that binding.
// -restconf-gnmi forces the same rewrite for a single run.
var restconfGNMI = flag.Bool("restconf-gnmi", false, "rewrite gNMI JSON into the RESTCONF shape SONiC translib unmarshals")

func (d *staticDUT) useRestconfGNMI() bool {
	if *restconfGNMI {
		return true
	}
	params := dutSvcParams[introspect.GNMI]
	if params == nil || d.dev == nil {
		return false
	}
	return d.r.grpc(d.dev, params).GetRestconfJson()
}

type restconfGNMIClient struct {
	gpb.GNMIClient
}

func (c *restconfGNMIClient) Set(ctx context.Context, in *gpb.SetRequest, opts ...grpc.CallOption) (*gpb.SetResponse, error) {
	for _, up := range in.GetReplace() {
		if err := wrapRestconfUpdate(up); err != nil {
			return nil, err
		}
	}
	for _, up := range in.GetUpdate() {
		if err := wrapRestconfUpdate(up); err != nil {
			return nil, err
		}
	}
	return c.GNMIClient.Set(ctx, in, opts...)
}

func (c *restconfGNMIClient) Get(ctx context.Context, in *gpb.GetRequest, opts ...grpc.CallOption) (*gpb.GetResponse, error) {
	if in.GetEncoding() == gpb.Encoding_PROTO || in.GetEncoding() == gpb.Encoding_JSON {
		in.Encoding = gpb.Encoding_JSON_IETF
	}
	resp, err := c.GNMIClient.Get(ctx, in, opts...)
	if resp != nil {
		for _, note := range resp.GetNotification() {
			normalizeRestconfNotification(note)
		}
	}
	return resp, err
}

func (c *restconfGNMIClient) Subscribe(ctx context.Context, opts ...grpc.CallOption) (gpb.GNMI_SubscribeClient, error) {
	sub, err := c.GNMIClient.Subscribe(ctx, opts...)
	if err != nil {
		return nil, err
	}
	return &restconfSubscribeClient{GNMI_SubscribeClient: sub, client: c}, nil
}

// wrapRestconfUpdate wraps a ygnmi JSON value in the node named by the last
// path element. A list node becomes a one-element JSON array. Values that
// are not JSON-IETF, such as a CLI ascii push, are left unchanged.
//
// When every key in an object value shares one module prefix, that prefix
// is added to the wrapper key. ygnmi path elements are unqualified
// ("interface"), and SONiC matches the qualified RESTCONF name
// ("openconfig-interfaces:interface").
func wrapRestconfUpdate(up *gpb.Update) error {
	raw := up.GetVal().GetJsonIetfVal()
	if len(raw) == 0 {
		return nil
	}
	elems := up.GetPath().GetElem()
	if len(elems) == 0 {
		return nil
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return fmt.Errorf("unable to unmarshal config: %v", err)
	}
	name := restconfKey(elems[len(elems)-1].GetName(), v)
	if len(elems[len(elems)-1].GetKey()) > 0 {
		v = []any{v}
	}
	js, err := json.MarshalIndent(map[string]any{name: v}, "", "  ")
	if err != nil {
		return fmt.Errorf("unable to marshal config with wrapping container: %v", err)
	}
	up.GetVal().Value = &gpb.TypedValue_JsonIetfVal{JsonIetfVal: js}
	return nil
}

func restconfKey(name string, v any) string {
	if strings.Contains(name, ":") {
		return name
	}
	if mod := sharedModule(v); mod != "" {
		return mod + ":" + name
	}
	return name
}

func sharedModule(v any) string {
	obj, ok := v.(map[string]any)
	if !ok || len(obj) == 0 {
		return ""
	}
	var mod string
	for k := range obj {
		i := strings.LastIndex(k, ":")
		if i < 0 {
			return ""
		}
		m := k[:i]
		if mod == "" {
			mod = m
			continue
		}
		if m != mod {
			return ""
		}
	}
	return mod
}

type restconfSubscribeClient struct {
	gpb.GNMI_SubscribeClient
	client   *restconfGNMIClient
	once     []*gpb.SubscribeResponse
	onceMode bool
}

// Send asks for JSON_IETF. ygnmi defaults to PROTO, and SONiC returns leaf
// values as JSON objects that PROTO decoding drops. A ONCE subscription on
// this server returns sync and no data, so that request is served from Get.
func (sc *restconfSubscribeClient) Send(req *gpb.SubscribeRequest) error {
	sub := req.GetSubscribe()
	if sub != nil && sub.GetMode() == gpb.SubscriptionList_ONCE {
		getReq := &gpb.GetRequest{
			Prefix:   sub.GetPrefix(),
			Encoding: gpb.Encoding_JSON_IETF,
			Type:     gpb.GetRequest_ALL,
		}
		for _, s := range sub.GetSubscription() {
			if s.GetPath() != nil {
				getReq.Path = append(getReq.Path, s.GetPath())
			}
		}
		resp, err := sc.client.Get(context.Background(), getReq)
		if err != nil {
			return err
		}
		for _, note := range resp.GetNotification() {
			sc.once = append(sc.once, &gpb.SubscribeResponse{
				Response: &gpb.SubscribeResponse_Update{Update: note},
			})
		}
		sc.once = append(sc.once, &gpb.SubscribeResponse{
			Response: &gpb.SubscribeResponse_SyncResponse{SyncResponse: true},
		})
		sc.onceMode = true
		return nil
	}
	if sub != nil && (sub.Encoding == gpb.Encoding_PROTO || sub.Encoding == gpb.Encoding_JSON) {
		sub.Encoding = gpb.Encoding_JSON_IETF
	}
	return sc.GNMI_SubscribeClient.Send(req)
}

// Recv rewrites module-prefixed paths and single-key JSON objects into the
// scalar leaf values ygnmi unmarshals.
func (sc *restconfSubscribeClient) Recv() (*gpb.SubscribeResponse, error) {
	if sc.onceMode {
		if len(sc.once) == 0 {
			return nil, io.EOF
		}
		resp := sc.once[0]
		sc.once = sc.once[1:]
		return resp, nil
	}
	resp, err := sc.GNMI_SubscribeClient.Recv()
	if err != nil || resp == nil {
		return resp, err
	}
	if note := resp.GetUpdate(); note != nil {
		normalizeRestconfNotification(note)
	}
	return resp, nil
}

func normalizeRestconfNotification(n *gpb.Notification) {
	if n == nil {
		return
	}
	normalizeRestconfPath(n.GetPrefix())
	for _, up := range n.GetUpdate() {
		normalizeRestconfPath(up.GetPath())
		unwrapRestconfLeaf(up)
	}
}

func normalizeRestconfPath(p *gpb.Path) {
	if p == nil {
		return
	}
	for _, elem := range p.GetElem() {
		if i := strings.LastIndex(elem.GetName(), ":"); i >= 0 {
			elem.Name = elem.GetName()[i+1:]
		}
	}
}

// unwrapRestconfLeaf turns {"openconfig-interfaces:name":"Ethernet46"} into
// "Ethernet46" when that key is the last path element.
func unwrapRestconfLeaf(up *gpb.Update) {
	raw := up.GetVal().GetJsonIetfVal()
	elems := up.GetPath().GetElem()
	if len(raw) == 0 || len(elems) == 0 || up.GetVal() == nil {
		return
	}
	var parsed any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return
	}
	obj, ok := parsed.(map[string]any)
	if !ok || len(obj) != 1 {
		return
	}
	var key string
	var val any
	for k, v := range obj {
		key, val = k, v
	}
	local := key
	if i := strings.LastIndex(key, ":"); i >= 0 {
		local = key[i+1:]
	}
	if local != elems[len(elems)-1].GetName() {
		return
	}
	scalar, err := json.Marshal(val)
	if err != nil {
		return
	}
	up.Val.Value = &gpb.TypedValue_JsonIetfVal{JsonIetfVal: scalar}
}
