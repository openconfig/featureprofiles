// Copyright 2024 Google LLC
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

package record_payload_truncation_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/fptest"
	acctzpb "github.com/openconfig/gnsi/acctz"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

type recordRequestResult struct {
	record *acctzpb.RecordResponse
	err    error
}

func sendOversizedPayload(t *testing.T, dut *ondatra.DUTDevice) {
	// Perhaps other vendors will need a different payload/size/etc., for now we'll just send a
	// giant set of network instances + static routes which should hopefully work for everyone.
	ocRoot := &oc.Root{}

	for i := 0; i < 50; i++ {
		ni := ocRoot.GetOrCreateNetworkInstance(fmt.Sprintf("acctz-test-ni-%d", i))
		ni.SetDescription("This is a pointlessly long description in order to make the payload bigger.")
		ni.SetType(oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF)
		staticProtocol := ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, "static")
		for j := 0; j < 254; j++ {
			staticProtocol.GetOrCreateStatic(fmt.Sprintf("10.%d.0.0/24", j))
		}
	}

	gnmi.Update(t, dut, gnmi.OC().Config(), ocRoot)
}

func TestAccountzRecordPayloadTruncation(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	startTime := time.Now()
	sendOversizedPayload(t, dut)
	acctzClient := dut.RawAPIs().GNSI(t).Acctz()

	acctzSubClient, err := acctzClient.RecordSubscribe(context.Background())
	if err != nil {
		t.Fatalf("Failed getting accountz record subscribe client, error: %s", err)
	}

	err = acctzSubClient.Send(&acctzpb.RecordRequest{
		Timestamp: timestamppb.New(startTime),
	})
	if err != nil {
		t.Fatalf("Failed sending record request, error: %s", err)
	}

	for {
		r := make(chan recordRequestResult)

		go func(r chan recordRequestResult) {
			var response *acctzpb.RecordResponse
			response, err = acctzSubClient.Recv()
			r <- recordRequestResult{
				record: response,
				err:    err,
			}
		}(r)

		var done bool
		var resp recordRequestResult

		select {
		case rr := <-r:
			resp = rr
		case <-time.After(10 * time.Second):
			done = true
		}

		if done {
			t.Fatal("Done receiving records and did not find our record...")
		}

		if resp.err != nil {
			t.Fatalf("Failed receiving record response, error: %s", resp.err)
		}

		grpcServiceRecord := resp.record.GetGrpcService()

		if grpcServiceRecord.GetServiceType() != acctzpn.GrpcService_GRPC_SERVICE_TYPE_GNMI {
			// Not our gnmi set, nothing to see here.
			continue
		}

		if grpcServiceRecord.RpcName != "/gnmi.gNMI/Set" {
			continue
		}

		if grpcServiceRecord.GetPayloadIstruncated() {
			t.Log("Found truncated payload of gnmi.Set after start timestamp, success!")
			break
		}
	}
}
