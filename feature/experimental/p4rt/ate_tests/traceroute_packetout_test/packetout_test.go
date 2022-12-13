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
	"github.com/openconfig/ondatra/gnmi"
	p4v1 "github.com/p4lang/p4runtime/go/p4/v1"
)

type PacketIO interface {
	GetPacketOut(portID uint32, isIPv4 bool, ttl uint8) []*p4v1.PacketOut
}

type testArgs struct {
	ctx      context.Context
	leader   *p4rt_client.P4RTClient
	dut      *ondatra.DUTDevice
	ate      *ondatra.ATEDevice
	top      *ondatra.ATETopology
	packetIO PacketIO
}

// sendPackets sends out packets via PacketOut message in StreamChannel.
func sendPackets(t *testing.T, client *p4rt_client.P4RTClient, packets []*p4v1.PacketOut, packetCount int) {
	count := packetCount / len(packets)
	for _, packet := range packets {
		for i := 0; i < count; i++ {
			if err := client.StreamChannelSendMsg(
				&streamName, &p4v1.StreamMessageRequest{
					Update: &p4v1.StreamMessageRequest_Packet{
						Packet: packet,
					},
				},
			); err != nil {
				t.Errorf("There is error seen in Packet Out. %v, %s", err, err)

			}
		}
	}
}

// testPacketOut sends out PacketOut with payload on p4rt leader or
// follower client, then verify DUT interface statistics
func testPacketOut(ctx context.Context, t *testing.T, args *testArgs) {
	leader := args.leader
	desc := "PacketOut from Primary Controller"
	ttls := []int{0, 1}
	//for ipv4
	t.Run(desc+" ipv4 ", func(t *testing.T) {
		// Check initial packet counters
		port := sortPorts(args.ate.Ports())[0].Name()

		for _, ttl := range ttls {
			t.Logf("Sending ipv4 pakcets with ttl %d", ttl)
			counter0 := gnmi.Get(t, args.ate, gnmi.OC().Interface(port).Counters().InPkts().State())
			t.Logf("Initial number of packets: %d", counter0)

			packets := args.packetIO.GetPacketOut(portId, true, uint8(ttl))
			packetCounter := 100
			t.Logf("Sending packets now")

			sendPackets(t, leader, packets, packetCounter)

			// Wait for ate stats to be populated
			time.Sleep(60 * time.Second)

			// Check packet counters after packet out
			counter1 := gnmi.Get(t, args.ate, gnmi.OC().Interface(port).Counters().InPkts().State())
			t.Logf("Final number of packets: %d", counter1)

			// Verify InPkts stats to check P4RT stream
			t.Logf("Received %v packets on ATE port %s", counter1-counter0, port)

			if counter1-counter0 < uint64(float64(packetCounter)*0.95) {
				t.Fatalf("Not all the packets are received.")
			}
			time.Sleep(20 * time.Second)
		}

	},
	)
	//for ipv6
	t.Run(desc+" ipv6", func(t *testing.T) {
		// Check initial packet counters
		port := sortPorts(args.ate.Ports())[0].Name()

		for _, ttl := range ttls {
			t.Logf("Sending ipv6 packets with ttl = %d", ttl)
			counter0 := gnmi.Get(t, args.ate, gnmi.OC().Interface(port).Counters().InPkts().State())
			t.Logf("Initial number of packets: %d", counter0)

			packets := args.packetIO.GetPacketOut(portId, false, uint8(ttl))
			packetCounter := 100
			t.Logf("Sending packets now")

			sendPackets(t, leader, packets, packetCounter)

			// Wait for ate stats to be populated
			time.Sleep(60 * time.Second)

			// Check packet counters after packet out
			counter1 := gnmi.Get(t, args.ate, gnmi.OC().Interface(port).Counters().InPkts().State())
			t.Logf("Final number of packets: %d", counter1)

			// Verify InPkts stats to check P4RT stream
			t.Logf("Received %v packets on ATE port %s", counter1-counter0, port)

			if counter1-counter0 < uint64(float64(packetCounter)*0.95) {
				t.Fatalf("Not all the packets are received.")
			}

			time.Sleep(20 * time.Second)
		}

	},
	)

}
