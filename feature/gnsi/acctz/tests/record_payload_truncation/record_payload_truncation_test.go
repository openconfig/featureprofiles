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

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/helpers"
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
		staticProtocol := ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
		nhAddress := fmt.Sprintf("192.%d.2.1", i)
		nstatRoutes := 0
		switch dut.Vendor() {
		case ondatra.JUNIPER:
			nstatRoutes = 1
		default:
			nstatRoutes = 254
		}
		for j := 0; j < nstatRoutes; j++ {
			sr1 := staticProtocol.GetOrCreateStatic(fmt.Sprintf("10.%d.0.0/24", j))
			nh1 := sr1.GetOrCreateNextHop("0")
			nh1.NextHop = oc.UnionString(nhAddress)
		}
	}
	gnmi.Update(t, dut, gnmi.OC().Config(), ocRoot)
}

func TestAccountzRecordPayloadTruncation(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	const queueSize = 100
	const historyMemory = 100

	switch dut.Vendor() {
	case ondatra.CISCO:
		communitySetCLIConfig := fmt.Sprintf("grpc \n aaa accounting queue-size %d\n aaa accounting history-memory %d \n!", queueSize, historyMemory)
		helpers.GnmiCLIConfig(t, dut, communitySetCLIConfig)
	}

	startTime := time.Now()

	acctzClient := dut.RawAPIs().GNSI(t).AcctzStream()
	acctzSubClient, err := acctzClient.RecordSubscribe(context.Background(), &acctzpb.RecordRequest{
		Timestamp: timestamppb.New(startTime),
	})
	if err != nil {
		t.Fatalf("Failed to subscribe to acctz records: %v", err)
	}

	sendOversizedPayload(t, dut)

	for {
		r := make(chan recordRequestResult)
		go func(r chan recordRequestResult) {
			resp, err := acctzSubClient.Recv()
			r <- recordRequestResult{
				record: resp,
				err:    err,
			}
		}(r)
		var done bool
		var resp recordRequestResult

		select {
		case rr := <-r:
			resp = rr
		case <-time.After(60 * time.Second):
			done = true
		}

		if done {
			t.Fatal("Done receiving records and did not find our record...")
		}

		if resp.err != nil {
			t.Fatalf("Failed receiving record response, error: %s", resp.err)
		}

		t.Logf("Received record: %v", resp.record)

		grpcServiceRecord := resp.record.GetGrpcService()
		if grpcServiceRecord == nil {
			continue
		}

		if grpcServiceRecord.GetServiceType() != acctzpb.GrpcService_GRPC_SERVICE_TYPE_GNMI {
			t.Logf("Not our gnmi set, service type: %v", grpcServiceRecord.GetServiceType())
			continue
		}

		if grpcServiceRecord.GetRpcName() != "/gnmi.gNMI/Set" {
			t.Logf("Not our gnmi set, rpc name: %v", grpcServiceRecord.GetRpcName())
			continue
		}

		if !grpcServiceRecord.GetPayloadIstruncated() {
			t.Log("Found our record, but it is not truncated...")
			continue
		}

		t.Logf("Found truncated record: %v", resp.record)
		break
	}
}
