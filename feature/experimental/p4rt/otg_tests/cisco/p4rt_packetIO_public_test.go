package cisco_p4rt_test

import (
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	p4rt_client "github.com/cisco-open/go-p4/p4rt_client"
	"github.com/google/gopacket/layers"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra/gnmi"
	p4_v1 "github.com/p4lang/p4runtime/go/p4/v1"
)

var (
	PublicGDPTestcases = []Testcase{
		{
			name: "TE-3.1 Program GDP Match Entry and Check PacketIn",
			desc: "TE 3.1",
			fn:   testPublicPacketIn,
		},
		{
			name: "TE-3.2 Program GDP Match Entry and Check PacketOut",
			desc: "TE 3.2",
			fn:   testPublicPacketOut,
		},
	}
)

var (
	PublicLLDPDisableTestcases = []Testcase{
		{
			name: "TE-7.1 Program LLDP Match Entry and Check PacketIn",
			desc: "TE 7.1",
			fn:   testPublicPacketIn,
		},
		{
			name: "TE-7.2 Program LLDP Match Entry and Check PacketOut",
			desc: "TE 7.2",
			fn:   testPublicPacketOut,
		},
	}
)

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
			t.Logf("There is error seen when receving packets. %v, %s", err, err)
			break
		}
	}
	return packets
}

func testPublicPacketIn(ctx context.Context, t *testing.T, args *testArgs) {
	leader := args.p4rtClientA
	follower := args.p4rtClientB

	// Program the entry
	if err := programmTableEntry(ctx, t, leader, args.packetIO, false); err != nil {
		t.Fatalf("There is error when inserting the GDP entry")
	}
	defer programmTableEntry(ctx, t, leader, args.packetIO, true)

	// Send Packet
	// srcEndPoint := args.top.Interfaces()[atePort1.Name]
	testP4RTTraffic(t, args.ate, args.top, args.packetIO.GetTrafficFlow(t, args.ate, 300, 2), 10, args.packetIO.IsIPv6(t))
	otgutils.LogPortMetrics(t, args.ate.OTG(), args.top)
	otgutils.LogFlowMetrics(t, args.ate.OTG(), args.top)

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
			// Check PacketIn on P4Client
			packets := fetchPackets(ctx, t, test.client, 40)

			// t.Logf("Captured packets: %v", len(packets))
			if !test.expectPass {
				// t.Logf("Captured packets: %v", len(packets))
				if len(packets) > 0 {
					t.Fatalf("Unexpected packets received.")
				}
			} else {
				if len(packets) == 0 {
					t.Fatalf("There is no packets received.")
				}
				t.Logf("Start to decode packet.")
				wantPacket := args.packetIO.GetPacketTemplate(t)
				for _, packet := range packets {
					// t.Logf("Packet: %v", packet)
					if packet != nil {
						// t.Logf("Decoded Ether Type: %v; Decoded DST MAC: %v", etherType, dstMac)
						if wantPacket.DstMAC != nil && wantPacket.EthernetType != nil {
							dstMac, etherType := decodePacket(t, packet.Pkt.GetPayload())
							if dstMac != *wantPacket.DstMAC || etherType != layers.EthernetType(*wantPacket.EthernetType) {
								t.Fatalf("Packet is not matching wanted packet.")
							}
						}

						metaData := packet.Pkt.GetMetadata()
						for _, data := range metaData {
							if data.GetMetadataId() == METADATA_INGRESS_PORT {
								if string(data.GetValue()) != fmt.Sprint(portID) {
									t.Fatalf("Ingress Port Id is not matching expectation...")
								}
							}
						}
					}
				}
			}
		})
	}
}

func testPublicPacketOut(ctx context.Context, t *testing.T, args *testArgs) {
	leader := args.p4rtClientA
	follower := args.p4rtClientB

	// Program the entry
	// if err := programmTableEntry(ctx, t, leader, args.packetIO, false); err != nil {
	// 	t.Fatalf("There is error when inserting the GDP entry")
	// }
	// defer programmTableEntry(ctx, t, leader, args.packetIO, true)

	packetOutTests := []struct {
		desc       string
		client     *p4rt_client.P4RTClient
		expectPass bool
	}{{
		desc:       "PacketOut from Primary Controller",
		client:     leader,
		expectPass: true,
	}, {
		desc:       "PacketOut from Secondary Controller",
		client:     follower,
		expectPass: false,
	}}

	for _, test := range packetOutTests {
		t.Run(test.desc, func(t *testing.T) {
			// Check initial packet counters
			port := sortPorts(args.dut.Ports())[0].Name()
			counter_0 := gnmi.Get(t, args.dut, gnmi.OC().Interface(port).Counters().OutPkts().State())

			packet := args.packetIO.GetPacketOut(t, portID, false)

			packet_count := 100
			for i := 0; i < packet_count; i++ {
				if err := test.client.StreamChannelSendMsg(
					&streamName, &p4_v1.StreamMessageRequest{
						Update: &p4_v1.StreamMessageRequest_Packet{
							Packet: packet[0],
						},
					}); err != nil {
					t.Fatalf("There is error seen in Packet Out. %v, %s", err, err)
				}
			}

			// Wait for ate stats to be populated
			time.Sleep(60 * time.Second)
			// testTraffic(t, args.ate, gdpMAC, gdpEtherType, srcEndPoint, 10, args)

			// Check packet counters after packet out
			// counter_1 := args.ate.Telemetry().Interface(port).Counters().InPkts().Get(t)
			counter_1 := gnmi.Get(t, args.dut, gnmi.OC().Interface(port).Counters().OutPkts().State())

			// Verify InPkts stats to check P4RT stream
			// fmt.Println(counter_0)
			// fmt.Println(counter_1)

			t.Logf("Sends out %v packets on interface %s", counter_1-counter_0, port)

			if test.expectPass {
				if counter_1-counter_0 < uint64(float64(packet_count)*0.95) {
					t.Fatalf("Not all the packets are received.")
				}
			} else {
				if counter_1-counter_0 > uint64(float64(packet_count)*0.10) {
					t.Fatalf("Unexpected packets are received.")
				}
			}
		})
	}
}
