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

package google_discovery_protocol_packetin_test

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/cisco-open/go-p4/p4rt_client"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/openconfig/featureprofiles/feature/experimental/p4rt/wbb"
	"github.com/openconfig/ondatra"
	p4_v1 "github.com/p4lang/p4runtime/go/p4/v1"
)

type PacketIO interface {
	GetTableEntry(delete bool) []*wbb.AclWbbIngressTableEntryInfo
	GetPacketTemplate() *PacketIOPacket
	GetTrafficFlow(ate *ondatra.ATEDevice, frameSize uint32, frameRate uint64) []*ondatra.Flow
	GetEgressPort() []string
	GetIngressPort() string
}

type PacketIOPacket struct {
	SrcMAC, DstMAC *string
	EthernetType   *uint32
}

type testArgs struct {
	ctx      context.Context
	leader   *p4rt_client.P4RTClient
	follower *p4rt_client.P4RTClient
	dut      *ondatra.DUTDevice
	ate      *ondatra.ATEDevice
	top      *ondatra.ATETopology
	packetIO PacketIO
}

// programmTableEntry programs or deletes p4rt table entry based on delete flag.
func programmTableEntry(ctx context.Context, t *testing.T, client *p4rt_client.P4RTClient, packetIO PacketIO, delete bool) error {
	t.Helper()
	err := client.Write(&p4_v1.WriteRequest{
		DeviceId:   deviceId,
		ElectionId: &p4_v1.Uint128{High: uint64(0), Low: electionId},
		Updates: wbb.AclWbbIngressTableEntryGet(
			packetIO.GetTableEntry(delete),
		),
		Atomicity: p4_v1.WriteRequest_CONTINUE_ON_ERROR,
	})
	if err != nil {
		return err
	}
	return nil
}

// decodePacket decodes L2 header in the packet and returns destination MAC and ethernet type.
func decodePacket(t *testing.T, packetData []byte) (string, layers.EthernetType) {
	t.Helper()
	packet := gopacket.NewPacket(packetData, layers.LayerTypeEthernet, gopacket.Default)
	etherHeader := packet.Layer(layers.LayerTypeEthernet)
	if etherHeader != nil {
		header, decoded := etherHeader.(*layers.Ethernet)
		if decoded {
			return header.DstMAC.String(), header.EthernetType
		}
	}
	return "", layers.EthernetType(0)
}

// testTraffic sends traffic flow for duration seconds.
func testTraffic(t *testing.T, ate *ondatra.ATEDevice, flows []*ondatra.Flow, srcEndPoint *ondatra.Interface, duration int) {
	t.Helper()
	for _, flow := range flows {
		flow.WithSrcEndpoints(srcEndPoint).WithDstEndpoints(srcEndPoint)
	}
	ate.Traffic().Start(t, flows...)
	time.Sleep(time.Duration(duration) * time.Second)

	ate.Traffic().Stop(t)
}

// fetchPackets reads p4rt packets sent to p4rt client.
func fetchPackets(ctx context.Context, t *testing.T, client *p4rt_client.P4RTClient, expectNumber int) []*p4rt_client.P4RTPacketInfo {
	t.Helper()
	packets := []*p4rt_client.P4RTPacketInfo{}
	for i := 0; i < expectNumber; i++ {
		_, packet, err := client.StreamChannelGetPacket(&streamName, 0)
		if err == io.EOF {
			t.Logf("EOF error is seen in PacketIn.")
			break
		} else if err == nil {
			if packet != nil {
				packets = append(packets, packet)
			}
		} else {
			t.Fatalf("There is error seen when receving packets. %v, %s", err, err)
			break
		}
	}
	return packets
}

// testPacketIn programs p4rt table entry and sends traffic related to GDP,
// then validates packetin message metadata and payload.
func testPacketIn(ctx context.Context, t *testing.T, args *testArgs) {
	leader := args.leader
	follower := args.follower

	// Insert wbb acl entry on the DUT
	if err := programmTableEntry(ctx, t, leader, args.packetIO, false); err != nil {
		t.Fatalf("There is error when programming entry")
	}
	// Delete wbb acl entry on the device
	defer programmTableEntry(ctx, t, leader, args.packetIO, true)

	// Send GDP traffic from ATE
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	testTraffic(t, args.ate, args.packetIO.GetTrafficFlow(args.ate, 300, 2), srcEndPoint, 10)

	packetInTests := []struct {
		desc       string
		client     *p4rt_client.P4RTClient
		expectPass bool
	}{{
		desc:       "PacketIn to Primary Controller",
		client:     leader,
		expectPass: true,
	}, {
		desc:       "PacketIn to Secondary Controller",
		client:     follower,
		expectPass: false,
	}}

	for _, test := range packetInTests {
		t.Run(test.desc, func(t *testing.T) {
			// Extract packets from PacketIn message sent to p4rt client
			packets := fetchPackets(ctx, t, test.client, 40)

			if !test.expectPass {
				if len(packets) > 0 {
					t.Fatalf("Unexpected packets received.")
				}
			} else {
				if len(packets) == 0 {
					t.Fatalf("There are no packets received.")
				}
				t.Logf("Start to decode packet and compare with expected packets.")
				wantPacket := args.packetIO.GetPacketTemplate()
				for _, packet := range packets {
					if packet != nil {
						if wantPacket.DstMAC != nil && wantPacket.EthernetType != nil {
							dstMac, etherType := decodePacket(t, packet.Pkt.GetPayload())
							if dstMac != *wantPacket.DstMAC || etherType != layers.EthernetType(*wantPacket.EthernetType) {
								t.Fatalf("Packet in PacketIn message is not matching wanted packet.")
							}
						}

						metaData := packet.Pkt.GetMetadata()
						for _, data := range metaData {
							if data.GetMetadataId() == METADATA_INGRESS_PORT {
								if string(data.GetValue()) != args.packetIO.GetIngressPort() {
									t.Fatalf("Ingress Port Id is not matching expectation.")
								}
							}
							if data.GetMetadataId() == METADATA_EGRESS_PORT {
								found := false
								for _, portData := range args.packetIO.GetEgressPort() {
									if string(data.GetValue()) == portData {
										found = true
									}
								}
								if !found {
									t.Fatalf("Egress Port Id is not matching expectation.")
								}

							}
						}
					}
				}
			}
		})
	}
}
