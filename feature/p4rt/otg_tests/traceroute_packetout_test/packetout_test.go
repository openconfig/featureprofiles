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
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/cisco-open/go-p4/p4rt_client"
	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	p4v1 "github.com/p4lang/p4runtime/go/p4/v1"
)

type PacketIO interface {
	GetPacketOut(srcMAC, dstMAC net.HardwareAddr, isIPv4 bool, ttl uint8, numPkts int, metadata []*p4v1.PacketMetadata) ([]*p4v1.PacketOut, error)
}

type testArgs struct {
	ctx         context.Context
	client      *p4rt_client.P4RTClient
	dut         *ondatra.DUTDevice
	ate         *ondatra.ATEDevice
	top         gosnappi.Config
	srcMAC      net.HardwareAddr
	dstMAC      net.HardwareAddr
	useIpv4     bool
	metadata    []*p4v1.PacketMetadata
	trafficPort string
	packetIO    PacketIO
}

func (t *testArgs) testName() string {
	var egressPort, submitIngress, padding string
	for _, m := range t.metadata {
		switch m.MetadataId {
		case 1:
			egressPort = string(m.Value)
		case 2:
			submitIngress = string(m.Value)
		case 3:
			padding = string(m.Value)
		}
	}
	return fmt.Sprintf("egressPort:%s,submitToIngress:%s,padding:%s,ipv4:%v", egressPort, submitIngress, padding, t.useIpv4)
}

// sendPackets sends out packets via PacketOut message in StreamChannel.
func sendPackets(t *testing.T, client *p4rt_client.P4RTClient, packets []*p4v1.PacketOut) {
	for _, packet := range packets {
		if err := client.StreamChannelSendMsg(
			&streamName, &p4v1.StreamMessageRequest{
				Update: &p4v1.StreamMessageRequest_Packet{
					Packet: packet,
				},
			},
		); err != nil {
			t.Errorf("There is error seen in Packet Out. %v", err)
		}
	}
}

// testPacketOut sends out PacketOut with payload on p4rt client,
// then verify DUT interface statistics
func testPacketOut(ctx context.Context, t *testing.T, args *testArgs) {
	ttl := 2

	counter0p1 := gnmi.Get(t, args.ate.OTG(), gnmi.OTG().Port("port1").Counters().InFrames().State())
	t.Logf("Initial number of packets on ATE port %s: %d", "port1", counter0p1)
	counter0p2 := gnmi.Get(t, args.ate.OTG(), gnmi.OTG().Port("port2").Counters().InFrames().State())
	t.Logf("Initial number of packets on ATE port %s: %d", "port2", counter0p2)

	packets, err := args.packetIO.GetPacketOut(args.srcMAC, args.dstMAC, args.useIpv4, uint8(ttl), 100, args.metadata)
	if err != nil {
		t.Fatalf("GetPacketOut returned unexpected error: %v", err)
	}

	packetCount := len(packets)
	t.Logf("Sending %d ipv4 packets with ttl %d", packetCount, ttl)
	sendPackets(t, args.client, packets)

	// Wait for ate stats to be populated
	time.Sleep(60 * time.Second)

	// Check packet counters after packet out
	counter1p1 := gnmi.Get(t, args.ate.OTG(), gnmi.OTG().Port("port1").Counters().InFrames().State())
	t.Logf("Final number of packets on ATE port %s: %d", "port1", counter1p1)
	counter1p2 := gnmi.Get(t, args.ate.OTG(), gnmi.OTG().Port("port2").Counters().InFrames().State())
	t.Logf("Final number of packets on ATE port %s: %d", "port2", counter1p2)

	// Verify InPkts stats to check P4RT stream
	t.Logf("Received %v packets on ATE port %s", counter1p1-counter0p1, "port1")
	t.Logf("Received %v packets on ATE port %s", counter1p2-counter0p2, "port2")

	switch args.trafficPort {
	case "port1":
		// should receive all packets on port1
		if !gotAllPackets(counter1p1, counter0p1, packetCount) {
			t.Fatalf("Not all the packets are received on ATE port %s", "port1")
		}
		// should receive no packets on port2
		if gotAllPackets(counter1p2, counter0p2, packetCount) {
			t.Fatalf("Unexpected packets received on ATE port %s", "port2")
		}
	case "port2":
		// should receive all packets on port2
		if !gotAllPackets(counter1p2, counter0p2, packetCount) {
			t.Fatalf("Not all the packets are received on ATE port %s", "port2")
		}
		// should receive no packets on port1
		if gotAllPackets(counter1p1, counter0p1, packetCount) {
			t.Fatalf("Unexpected packets received on ATE port %s", "port1")
		}
	default:
		// should not receive packets on any port
		if gotAllPackets(counter1p1, counter0p1, packetCount) {
			t.Fatalf("Unexpected packets received on ATE port %s", "port1")
		}
		if gotAllPackets(counter1p2, counter0p2, packetCount) {
			t.Fatalf("Unexpected packets received on ATE port %s", "port2")
		}
	}
}

func gotAllPackets(counter1, counter0 uint64, expected int) bool {
	return counter1-counter0 >= uint64(float64(expected)*0.95)
}
