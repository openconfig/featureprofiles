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

package traceroute_packetout_test

import (
	"context"
	"testing"
	"time"

	"github.com/cisco-open/go-p4/p4rt_client"
	"github.com/openconfig/ondatra"
	p4_v1 "github.com/p4lang/p4runtime/go/p4/v1"
)

type PacketIO interface {
	GetPacketOut(portID uint32, isIPv4 bool, ttl uint8) []*p4_v1.PacketOut
}

type testArgs struct {
	ctx    context.Context
	leader *p4rt_client.P4RTClient
	//follower *p4rt_client.P4RTClient
	dut      *ondatra.DUTDevice
	ate      *ondatra.ATEDevice
	top      *ondatra.ATETopology
	packetIO PacketIO
}

// sendPackets sends out packets via PacketOut message in StreamChannel.
func sendPackets(t *testing.T, client *p4rt_client.P4RTClient, packets []*p4_v1.PacketOut, packetCount int) {
	count := packetCount / len(packets)
	for _, packet := range packets {
		for i := 0; i < count; i++ {
			if err := client.StreamChannelSendMsg(
				&streamName, &p4_v1.StreamMessageRequest{
					Update: &p4_v1.StreamMessageRequest_Packet{
						Packet: packet,
					},
				}); err != nil {
				t.Errorf("There is error seen in Packet Out. %v, %s", err, err)

			}
		}
	}
}

// testPacketOut sends out PacketOut with GDP payload on p4rt leader or
// follower client, then verify DUT interface statistics
func testPacketOut(ctx context.Context, t *testing.T, args *testArgs) {
	leader := args.leader

	packetOutTests := []struct {
		desc       string
		client     *p4rt_client.P4RTClient
		expectPass bool
	}{{
		desc:       "PacketOut from Primary Controller",
		client:     leader,
		expectPass: true,
	}}

	//for ipv4
	for _, test := range packetOutTests {
		t.Run(test.desc, func(t *testing.T) {
			// Check initial packet counters
			port := sortPorts(args.ate.Ports())[0].Name()
			//for ipv4 packets
			counter_0 := args.ate.Telemetry().Interface(port).Counters().InPkts().Get(t)

			packets := args.packetIO.GetPacketOut(portId, true, 1)
			packet_count := 100
			sendPackets(t, test.client, packets, packet_count)

			// Wait for ate stats to be populated
			time.Sleep(60 * time.Second)

			// Check packet counters after packet out
			counter_1 := args.ate.Telemetry().Interface(port).Counters().InPkts().Get(t)

			// Verify InPkts stats to check P4RT stream
			t.Logf("Received %v packets on ATE port %s", counter_1-counter_0, port)

			if test.expectPass {
				if counter_1-counter_0 < uint64(float64(packet_count)*0.95) {
					t.Fatalf("Not all the packets are received.")
				}
			} else {
				if counter_1-counter_0 > uint64(float64(packet_count)*0.10) {
					t.Fatalf("Unexpected packets are received.")
				}
			}
			t.Logf("Done sending ipv4 packets")
			time.Sleep(300 * time.Second)

			//repeat for ttl=0
			packets_2 := args.packetIO.GetPacketOut(portId, true, 0)
			packet_count = 100
			sendPackets(t, test.client, packets_2, packet_count)

			// Wait for ate stats to be populated
			time.Sleep(60 * time.Second)

			// Check packet counters after packet out
			counter_2 := args.ate.Telemetry().Interface(port).Counters().InPkts().Get(t)

			// Verify InPkts stats to check P4RT stream
			t.Logf("Received %v packets on ATE port %s", counter_2-counter_1, port)

			if test.expectPass {
				if counter_2-counter_1 < uint64(float64(packet_count)*0.95) {
					t.Fatalf("Not all the packets are received.")
				}
			} else {
				if counter_2-counter_1 > uint64(float64(packet_count)*0.10) {
					t.Fatalf("Unexpected packets are received.")
				}
			}

		})

	}

	//for ipv6
	for _, test := range packetOutTests {
		t.Run(test.desc, func(t *testing.T) {
			// Check initial packet counters
			port := sortPorts(args.ate.Ports())[0].Name()
			//for ipv4 packets

			counter_0 := args.ate.Telemetry().Interface(port).Counters().InPkts().Get(t)

			packets := args.packetIO.GetPacketOut(portId, false, 1)
			packet_count := 100
			t.Logf("Sending packets now")

			sendPackets(t, test.client, packets, packet_count)

			// Wait for ate stats to be populated
			time.Sleep(60 * time.Second)

			// Check packet counters after packet out
			counter_1 := args.ate.Telemetry().Interface(port).Counters().InPkts().Get(t)

			// Verify InPkts stats to check P4RT stream
			t.Logf("Received %v packets on ATE port %s", counter_1-counter_0, port)

			if test.expectPass {
				if counter_1-counter_0 < uint64(float64(packet_count)*0.95) {
					t.Fatalf("Not all the packets are received.")
				}
			} else {
				if counter_1-counter_0 > uint64(float64(packet_count)*0.10) {
					t.Fatalf("Unexpected packets are received.")
				}
			}
			t.Logf("Done sending ipv6 packets")
			time.Sleep(900 * time.Second)
			//repeat for ttl=0

			packets_2 := args.packetIO.GetPacketOut(portId, false, 0)
			packet_count = 10
			sendPackets(t, test.client, packets_2, packet_count)

			// Wait for ate stats to be populated
			time.Sleep(20 * time.Second)

			// Check packet counters after packet out
			counter_3 := args.ate.Telemetry().Interface(port).Counters().InPkts().Get(t)

			// Verify InPkts stats to check P4RT stream
			t.Logf("Received %v packets on ATE port %s", counter_3-counter_1, port)

			if test.expectPass {
				if counter_3-counter_1 < uint64(float64(packet_count)*0.95) {
					t.Fatalf("Not all the packets are received.")
				}
			} else {
				if counter_3-counter_1 > uint64(float64(packet_count)*0.10) {
					t.Fatalf("Unexpected packets are received.")
				}
			}

		})
	}

}
