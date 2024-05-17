package gnmi_subscriptionlist_test

import (
	"context"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/args"
	"github.com/openconfig/featureprofiles/internal/fptest"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ygnmi/ygnmi"
)

const (
	syncResponseWaitTimeOut = 300 * time.Second
)

var (
	returnChangedMode = map[gpb.SubscriptionMode]gpb.SubscriptionMode{
		gpb.SubscriptionMode_TARGET_DEFINED: gpb.SubscriptionMode_SAMPLE,
		gpb.SubscriptionMode_ON_CHANGE:      gpb.SubscriptionMode_TARGET_DEFINED,
	}
	backplaneFacingCapacityPaths = map[gpb.SubscriptionMode][]ygnmi.PathStruct{
		gpb.SubscriptionMode_ON_CHANGE: {
			gnmi.OC().ComponentAny().IntegratedCircuit().BackplaneFacingCapacity().TotalOperationalCapacity().State().PathStruct(),
		},
		gpb.SubscriptionMode_TARGET_DEFINED: {
			gnmi.OC().ComponentAny().IntegratedCircuit().BackplaneFacingCapacity().AvailablePct().State().PathStruct(),
			gnmi.OC().ComponentAny().IntegratedCircuit().BackplaneFacingCapacity().ConsumedCapacity().State().PathStruct(),
			gnmi.OC().ComponentAny().IntegratedCircuit().BackplaneFacingCapacity().Total().State().PathStruct(),
		},
	}

	telemetryPaths = map[gpb.SubscriptionMode][]ygnmi.PathStruct{
		gpb.SubscriptionMode_ON_CHANGE: {
			gnmi.OC().InterfaceAny().AdminStatus().State().PathStruct(),
			gnmi.OC().Lacp().InterfaceAny().MemberAny().Interface().State().PathStruct(),
			gnmi.OC().InterfaceAny().Ethernet().MacAddress().State().PathStruct(),
			gnmi.OC().InterfaceAny().HardwarePort().State().PathStruct(),
			gnmi.OC().InterfaceAny().Id().State().PathStruct(),
			gnmi.OC().InterfaceAny().OperStatus().State().PathStruct(),
			gnmi.OC().InterfaceAny().Ethernet().PortSpeed().State().PathStruct(),
			gnmi.OC().ComponentAny().IntegratedCircuit().NodeId().State().PathStruct(),
			gnmi.OC().ComponentAny().Parent().State().PathStruct(),
			gnmi.OC().ComponentAny().OperStatus().State().PathStruct(),
			gnmi.OC().InterfaceAny().ForwardingViable().State().PathStruct(),
		}, gpb.SubscriptionMode_TARGET_DEFINED: {
			gnmi.OC().InterfaceAny().Counters().InUnicastPkts().State().PathStruct(),
			gnmi.OC().InterfaceAny().Counters().InBroadcastPkts().State().PathStruct(),
			gnmi.OC().InterfaceAny().Counters().InMulticastPkts().State().PathStruct(),
			gnmi.OC().InterfaceAny().Counters().OutUnicastPkts().State().PathStruct(),
			gnmi.OC().InterfaceAny().Counters().OutBroadcastPkts().State().PathStruct(),
			gnmi.OC().InterfaceAny().Counters().OutMulticastPkts().State().PathStruct(),
			gnmi.OC().InterfaceAny().Counters().InOctets().State().PathStruct(),
			gnmi.OC().InterfaceAny().Counters().OutOctets().State().PathStruct(),
			gnmi.OC().InterfaceAny().Counters().InDiscards().State().PathStruct(),
			gnmi.OC().InterfaceAny().Counters().OutDiscards().State().PathStruct(),
			gnmi.OC().InterfaceAny().Counters().InErrors().State().PathStruct(),
			gnmi.OC().InterfaceAny().Counters().OutErrors().State().PathStruct(),
			gnmi.OC().InterfaceAny().Counters().InFcsErrors().State().PathStruct(),
			gnmi.OC().Qos().InterfaceAny().Output().QueueAny().TransmitOctets().State().PathStruct(),
			gnmi.OC().Qos().InterfaceAny().Output().QueueAny().TransmitPkts().State().PathStruct(),
			gnmi.OC().Qos().InterfaceAny().Output().QueueAny().DroppedPkts().State().PathStruct(),
		},
	}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func updateTelemetryPaths() {
	if *args.NumControllerCards > 0 {
		for mode, paths := range backplaneFacingCapacityPaths {
			telemetryPaths[mode] = append(telemetryPaths[mode], paths...)
		}
	}
}

func createSubscriptionList(t *testing.T, telemetryData map[gpb.SubscriptionMode][]ygnmi.PathStruct, changeSubscriptionModes bool) *gpb.SubscriptionList {
	subscriptions := make([]*gpb.Subscription, 0)
	for mode, paths := range telemetryData {
		currMode := mode
		if changeSubscriptionModes == true {
			currMode = returnChangedMode[mode]
		}
		for _, path := range paths {
			gnmiPath, _, err := ygnmi.ResolvePath(path)

			if err != nil {
				t.Errorf("[Error]:Error in resolving gnmi path =%v", path)
			}

			gnmiRequest := &gpb.Subscription{
				Path: gnmiPath,
				Mode: currMode,
			}
			if currMode == gpb.SubscriptionMode_SAMPLE {
				gnmiRequest.SampleInterval = uint64(time.Second * 10)
			}

			subscriptions = append(subscriptions, gnmiRequest)
		}
	}

	return &gpb.SubscriptionList{
		Subscription: subscriptions,
		Mode:         gpb.SubscriptionList_STREAM,
	}
}

func TestSingleSubscription(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ctx := context.Background()
	updateTelemetryPaths()
	testCases := []struct {
		desc       string
		changeMode bool
	}{
		{desc: "GNMI-2.1: Verify single subscription request with a Subscriptionlist and different SubscriptionModes", changeMode: false},
		{desc: "GNMI-2.2: Change SubscriptionModes in the subscription list and verify receipt of sync_response:", changeMode: true},
		{desc: "GNMI-2.2: Swithcing Modes, back to previous modes and verifying the receipt of sync_response ", changeMode: false},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			t.Log(tc.desc)
			subscribeList := createSubscriptionList(t, telemetryPaths, tc.changeMode)
			subscribeRequest := &gpb.SubscribeRequest{
				Request: &gpb.SubscribeRequest_Subscribe{
					Subscribe: subscribeList,
				},
			}
			stream, err := dut.RawAPIs().GNMI(t).Subscribe(ctx)
			defer stream.CloseSend()
			defer ctx.Done()

			if err != nil {
				t.Fatalf("[Fail]:Failed to create subscribe stream: %v", err)
			}

			if err := stream.Send(subscribeRequest); err != nil {
				t.Fatalf("[Fail]:Failed to send subscribe request: %v", err)
			}

			startTime := time.Now()
			for {
				resp, err := stream.Recv()
				if resp.GetSyncResponse() == true {
					t.Logf("Received sync_response!")
					break
				}
				if err != nil {
					t.Errorf("[Error]: While receieving the subcription response %v", err)
				}

				if time.Since(startTime).Seconds() > float64(syncResponseWaitTimeOut) {
					t.Fatalf("[Fail]:Didn't receive sync_response. Time limit = %v  exceeded", syncResponseWaitTimeOut)
				}
			}
		})
	}
}
