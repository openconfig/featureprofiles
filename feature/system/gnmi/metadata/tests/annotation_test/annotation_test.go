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

package gnmi_1_8_metadata_annotation_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ygnmi/ygnmi"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ygot/util"
	"github.com/openconfig/ygot/ygot"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Test cases:
//  - Set and get metadata annotation and verify annotation is configured.
//  - Set metadata annotation with different one and verify new annotation is configured.
//
// Test notes:
//   - https://github.com/openconfig/public/blob/master/release/models/extensions/openconfig-metadata.yang
//   - Documentation on the @ path: https://datatracker.ietf.org/doc/html/rfc7952#section-5.2.2
//   - Metadata is not supported by telemgen generated telemetry APIs, so we need to use
//     RawAPIs() to set request and get response data.
//   - Metadata annotation is a proto message with base64 encoded.
//   - Although RFC 7952 allows metadata annotation on any path and leaves,
//     this test only configures it at root.
//   - Steps to create SetRequest and get response data.
//     1) Marshal proto message and encode it with base64.
//        - Arista does not accept SetRequest paths with "@" in it - b/216674592.
//     2) Create a json format config for metadata annotation.
//     3) Marshal the metadata annotation config.
//     4) Use gnmiClient to send SetRequest with Update operation.
//     5) For GetResponse data, it is a reverse process to SetRequest.
//
//  Sample metadata annotation in json and flat format:
//   1) json format
//        {
//          "@": {
//             "openconfig-metadata:protobuf-metadata": "CNyR0gk="
//          }
//        }
//   2) flat format
//        /@/openconfig-metadata:protobuf-metadata: CNyR0gk=
//

func TestGNMIMetadataAnnotation(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	cases := []struct {
		desc        string
		protoMsg    proto.Message
		receivedMsg proto.Message
	}{
		{
			// This case is designed to set an empty field.
			desc:        "Set and get metadata annotation and verify annotation",
			protoMsg:    &timestamppb.Timestamp{Seconds: 1643261234},
			receivedMsg: &timestamppb.Timestamp{},
		},
		{
			// This case is designed to reset an existing field configured from the previous test.
			// It should also pass if it is run independently
			desc:        "Set metadata annotation with different value and verify new annotation",
			protoMsg:    &gpb.ModelData{Name: "Test model data", Organization: "Google", Version: "1.0"},
			receivedMsg: &gpb.ModelData{},
		},
	}

	for _, tc := range cases {
		t.Log(tc.desc)
		gnmiClient := dut.RawAPIs().GNMI(t)
		//Not assuming that hostname is already configured
		hostnameConfigPath := gnmi.OC().System().Hostname()
		gnmi.Replace(t, dut, hostnameConfigPath.Config(), string("ondatraHost"))

		t.Log("Build an annotated gNMI SetRequest from proto message")
		gpbSetRequest, err := buildMetadataAnnotation(t, tc.protoMsg)
		if err != nil {
			t.Errorf("Cannot build a gNMI SetRequest from proto message: %v", err)
		}

		t.Log("Appending an update to the annotated gNMI SetRequest")
		// accompaniedPath and accompaniedUpdateVal can be any valid oc path and value
		accompaniedPath := gnmi.OC().System().Hostname().Config().PathStruct()
		accompaniedUpdateVal := gnmi.Get[string](t, dut, gnmi.OC().System().Hostname().Config())
		gpbSetRequest.Update = append(gpbSetRequest.Update, buildGNMIUpdate(t, accompaniedPath, &accompaniedUpdateVal))

		t.Log("gnmiClient Set metadata annotation")
		if _, err = gnmiClient.Set(context.Background(), gpbSetRequest); err != nil {
			t.Errorf("gnmi.Set unexpected error: %v", err)
		}

		t.Log("gnmiClient Get metadata annotation")
		getResponse, err := gnmiClient.Get(context.Background(), &gpb.GetRequest{
			Path: []*gpb.Path{{
				Elem: []*gpb.PathElem{},
			}},
			Type:     gpb.GetRequest_CONFIG,
			Encoding: gpb.Encoding_JSON_IETF,
		})
		if err != nil {
			t.Fatalf("Cannot fetch metadata annotation from the DUT: %v", err)
		}

		receivedProtoMsg := tc.receivedMsg
		if err := extractMetadataAnnotation(t, getResponse, receivedProtoMsg); err != nil {
			t.Errorf("Extracts metadata protobuf message from getResponse with error: %v", err)
		}
		if diff := cmp.Diff(tc.protoMsg, receivedProtoMsg, protocmp.Transform()); diff != "" {
			t.Errorf("MyFunction(%s) diff (-want +got):\n%s", tc.protoMsg, diff)
		}
	}
}

// buildMetadataAnnotation builds a gNMI SetRequest by encoding a protobuf message as metadata.
func buildMetadataAnnotation(t *testing.T, m proto.Message) (*gpb.SetRequest, error) {
	t.Helper()
	b, err := proto.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("cannot marshal proto msg - error: %v", err)
	}
	anyMsg := &anypb.Any{TypeUrl: "example.com", Value: b}
	a, err := proto.Marshal(anyMsg)
	if err != nil {
		return nil, fmt.Errorf("cannot marshal proto any msg - error: %v", err)
	}

	b64 := base64.StdEncoding.EncodeToString(a)
	t.Logf("Encoded proto msg: %s\n", b64)

	j := map[string]interface{}{
		"@": map[string]interface{}{
			"openconfig-metadata:protobuf-metadata": b64,
		},
	}
	v, err := json.Marshal(j)
	if err != nil {
		return nil, fmt.Errorf("marshal metadata annotation failed with unexpected error: %v", err)
	}

	// TODO: extract the GetRequest construction to a buildGetRequest
	gpbSetRequest := &gpb.SetRequest{
		Update: []*gpb.Update{{
			Path: &gpb.Path{
				Elem: []*gpb.PathElem{},
			},
			Val: &gpb.TypedValue{
				Value: &gpb.TypedValue_JsonIetfVal{JsonIetfVal: v},
			},
		}},
	}
	return gpbSetRequest, nil
}

// extractMetadataAnnotation extracts the metadata protobuf message from a gNMI GetResponse.
func extractMetadataAnnotation(t *testing.T, getResponse *gpb.GetResponse, m proto.Message) error {
	t.Helper()
	if got := len(getResponse.GetNotification()); got != 1 {
		return fmt.Errorf("number of notifications got %d, want 1", got)
	}

	var encoded string
	n := getResponse.GetNotification()[0]
	u := n.Update[0]
	path, err := util.JoinPaths(n.GetPrefix(), u.GetPath())
	if err != nil {
		return err
	}
	if len(path.GetElem()) > 0 {
		return fmt.Errorf("update path, got: %v, want root", prototext.Format(path))
	}
	var v map[string]interface{}
	if err = json.Unmarshal(u.GetVal().GetJsonIetfVal(), &v); err != nil {
		return fmt.Errorf("cannot unmarshal the json content, err: %v", err)
	}
	metav, ok := v["@"]
	if !ok {
		return fmt.Errorf("did not receive metadata value from root data: %#v", v)
	}
	metaObj, ok := metav.(map[string]interface{})
	if !ok {
		return fmt.Errorf("received metadata object is not a JSON object: %v", metav)
	}
	annotation, ok := metaObj["openconfig-metadata:protobuf-metadata"]
	if !ok {
		return fmt.Errorf("received metadata object does not contain expected annotation: %#v", metav)
	}
	if encoded, ok = annotation.(string); !ok {
		return fmt.Errorf("got %T type for annotation, expected string type", annotation)
	}
	t.Logf("Got the encoded annotation %s", encoded)

	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return fmt.Errorf("cannot decode base64 string, err: %v", err)
	}

	anyMsg := &anypb.Any{}
	if err := proto.Unmarshal(decoded, anyMsg); err != nil {
		return fmt.Errorf("cannot unmarshal received proto any msg, err: %v", err)
	}
	if err := proto.Unmarshal(anyMsg.GetValue(), m); err != nil {
		return fmt.Errorf("cannot unmarshal received proto msg, err: %v", err)
	}
	return nil
}

// buildGNMIUpdate builds a gnmi update for a given ygot path and value.
func buildGNMIUpdate(t *testing.T, yPath ygnmi.PathStruct, val interface{}) *gpb.Update {
	t.Helper()
	path, _, errs := ygnmi.ResolvePath(yPath)
	if errs != nil {
		t.Fatalf("Could not resolve the ygot path; %v", errs)
	}
	js, err := ygot.Marshal7951(val, ygot.JSONIndent("  "), &ygot.RFC7951JSONConfig{AppendModuleName: true, PreferShadowPath: true})
	if err != nil {
		t.Fatalf("Could not encode value into JSON format: %v", err)
	}
	return &gpb.Update{
		Path: path,
		Val: &gpb.TypedValue{
			Value: &gpb.TypedValue_JsonIetfVal{
				JsonIetfVal: js,
			},
		},
	}

}
