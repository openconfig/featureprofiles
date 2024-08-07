package record_payload_truncation_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/gnsi/acctz"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

type recordRequestResult struct {
	record *acctz.RecordResponse
	err    error
}

func sendOversizedPayload(t *testing.T, dut *ondatra.DUTDevice) {
	// perhaps other vendors will need a different payload/size/etc., for now we'll just send a
	// giant set of network instances + static routes that should hopefully work for everyone!

	ocRoot := &oc.Root{}

	for i := 0; i < 50; i++ {
		ni := ocRoot.GetOrCreateNetworkInstance(fmt.Sprintf("acctz-test-ni-%d", i))

		ni.SetDescription("this is a pointlessly long description in order to make enbiggen the payload")
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
		t.Fatalf("failed getting accountz record subscribe client, error: %s", err)
	}

	err = acctzSubClient.Send(&acctz.RecordRequest{
		Timestamp: timestamppb.New(startTime),
	})
	if err != nil {
		t.Fatalf("failed sending record request, error: %s", err)
	}

	for {
		r := make(chan recordRequestResult)

		go func(r chan recordRequestResult) {
			var response *acctz.RecordResponse

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
			t.Fatal("done receiving records and did not find our record...")
		}

		if resp.err != nil {
			t.Fatalf("failed receiving record response, error: %s", resp.err)
		}

		grpcServiceRecord := resp.record.GetGrpcService()

		if grpcServiceRecord.GetServiceType() != acctz.GrpcService_GRPC_SERVICE_TYPE_GNMI {
			// not our gnmi set, nothin to see here
			continue
		}

		if grpcServiceRecord.RpcName != "/gnmi.gNMI/Set" {
			continue
		}

		if grpcServiceRecord.GetPayloadIstruncated() {
			t.Log("found truncated payload of gnmi.Set after start timestamp, success!")

			break
		}
	}
}
