// Copyright 2026 Google LLC
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

package aft_binding_contract_test

import (
	"context"
	"encoding/json"
	"testing"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type captureGNMI struct {
	gpb.GNMIClient
	sets     []*gpb.SetRequest
	gets     []*gpb.GetRequest
	setError error
}

func (c *captureGNMI) Set(_ context.Context, request *gpb.SetRequest, _ ...grpc.CallOption) (*gpb.SetResponse, error) {
	c.sets = append(c.sets, request)
	return &gpb.SetResponse{}, c.setError
}

func (c *captureGNMI) Get(_ context.Context, request *gpb.GetRequest, _ ...grpc.CallOption) (*gpb.GetResponse, error) {
	c.gets = append(c.gets, request)
	return &gpb.GetResponse{Notification: []*gpb.Notification{{
		Prefix: request.GetPrefix(),
		Update: []*gpb.Update{{Path: request.GetPath()[0], Val: &gpb.TypedValue{
			Value: &gpb.TypedValue_JsonIetfVal{JsonIetfVal: []byte(`"POLICY-TEST"`)}}}},
	}}}, nil
}

func TestTypedGlobalFilterWireContract(t *testing.T) {
	fake := &captureGNMI{}
	client, err := ygnmi.NewClient(fake, ygnmi.WithTarget("test-dut"))
	if err != nil {
		t.Fatal(err)
	}
	filter := gnmi.OC().NetworkInstance("VRF-A").Afts().GlobalFilter()
	batch := &ygnmi.SetBatch{}
	ygnmi.BatchUpdate(batch, filter.Ipv4Policy().Config(), "POLICY-TEST")
	ygnmi.BatchUpdate(batch, filter.Ipv6Policy().Config(), "POLICY-TEST")
	if _, err := batch.Set(context.Background(), client, ygnmi.WithEncoding(gpb.Encoding_JSON_IETF)); err != nil {
		t.Fatal(err)
	}
	if len(fake.sets) != 1 || len(fake.sets[0].GetUpdate()) != 2 {
		t.Fatalf("dual-stack update must use one RPC with two leaves: %v", fake.sets)
	}
	for i, update := range fake.sets[0].GetUpdate() {
		path, err := ygot.PathToString(update.GetPath())
		if err != nil {
			t.Fatal(err)
		}
		leaf := []string{"ipv4-policy", "ipv6-policy"}[i]
		want := "/network-instances/network-instance[name=VRF-A]/afts/global-filter/config/" + leaf
		if path != want {
			t.Fatalf("path=%q, want %q", path, want)
		}
		var policy string
		if err := json.Unmarshal(update.GetVal().GetJsonIetfVal(), &policy); err != nil || policy != "POLICY-TEST" {
			t.Fatalf("JSON_IETF string value not preserved: %v, %v", update.GetVal(), err)
		}
	}
	if fake.sets[0].GetPrefix().GetTarget() != "test-dut" {
		t.Fatal("DUT target was not preserved")
	}
	for _, check := range []struct {
		query    ygnmi.SingletonQuery[string]
		datatype gpb.GetRequest_DataType
	}{
		{filter.Ipv4Policy().Config(), gpb.GetRequest_CONFIG},
		{filter.Ipv6Policy().State(), gpb.GetRequest_STATE},
	} {
		value, err := ygnmi.Get(context.Background(), client, check.query, ygnmi.WithUseGet())
		if err != nil || value != "POLICY-TEST" {
			t.Fatalf("typed Get value=%q, error=%v", value, err)
		}
		if fake.gets[len(fake.gets)-1].GetType() != check.datatype {
			t.Fatalf("Get datatype not preserved: %v", fake.gets)
		}
	}
	deletion := &ygnmi.SetBatch{}
	ygnmi.BatchDelete(deletion, filter.Ipv4Policy().Config())
	ygnmi.BatchDelete(deletion, filter.Ipv6Policy().Config())
	ygnmi.BatchDelete(deletion, gnmi.OC().RoutingPolicy().PolicyDefinition("POLICY-TEST").Config())
	if _, err := deletion.Set(context.Background(), client); err != nil {
		t.Fatal(err)
	}
	if len(fake.sets) != 2 || len(fake.sets[1].GetDelete()) != 3 {
		t.Fatalf("filter + policy deletion must use one RPC: %v", fake.sets)
	}
	fake.setError = status.Error(codes.FailedPrecondition, "expected validation failure")
	_, err = batch.Set(context.Background(), client)
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("negative-check status changed: %v", err)
	}
}
