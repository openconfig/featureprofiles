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
// package to test P4RT with traceroute traffic of IPV4 and IPV6 with TTL/HopLimit as 0&1.
// go test -v . -testbed /root/ondatra/featureprofiles/topologies/atedut_2.testbed -binding /root/ondatra/featureprofiles/topologies/atedut_2.binding -outputs_dir logs

package traceroute_packetin_with_vrf_selection_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/cisco-open/go-p4/p4rt_client"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/p4rtutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ygnmi/ygnmi"
	pb "github.com/p4lang/p4runtime/go/p4/v1"
)

type PacketIO interface {
	GetTableEntry(delete bool, isIPv4 bool) []*p4rtutils.ACLWbbIngressTableEntryInfo
	GetPacketTemplate() *PacketIOPacket
	GetTrafficFlow(ate *ondatra.ATEDevice, dstMac string, isIpv4 bool,
		TTL uint8, frameSize uint32, frameRate uint64, dstIP string, flowValues *flowArgs) gosnappi.Flow
	GetIngressPort() string
}

type PacketIOPacket struct {
	TTL            *uint8
	SrcMAC, DstMAC *string
	EthernetType   *uint32
	HopLimit       *uint8
}

// programmTableEntry programs or deletes p4rt table entry based on delete flag.
func programmTableEntry(client *p4rt_client.P4RTClient, packetIO PacketIO, delete bool, isIPv4 bool) error {
	err := client.Write(&pb.WriteRequest{
		DeviceId:   deviceID,
		ElectionId: &pb.Uint128{High: uint64(0), Low: electionID},
		Updates: p4rtutils.ACLWbbIngressTableEntryGet(
			packetIO.GetTableEntry(delete, isIPv4),
		),
		Atomicity: pb.WriteRequest_CONTINUE_ON_ERROR,
	})
	return err
}

// decodePacket decodes L2 header in the packet and returns source and destination MAC and ethernet type.
func decodePacket(t *testing.T, packetData []byte) (string, string, layers.EthernetType) {
	t.Helper()
	packet := gopacket.NewPacket(packetData, layers.LayerTypeEthernet, gopacket.Default)
	etherHeader := packet.Layer(layers.LayerTypeEthernet)
	if etherHeader != nil {
		header, decoded := etherHeader.(*layers.Ethernet)
		if decoded {
			return header.SrcMAC.String(), header.DstMAC.String(), header.EthernetType
		}
	}
	return "", "", layers.EthernetType(0)
}

// decodePacket decodes L2 header in the packet and returns TTL. packetData[14:0] to remove first 14 bytes of Ethernet header.
func decodePacket4(t *testing.T, packetData []byte) uint8 {
	t.Helper()
	packet := gopacket.NewPacket(packetData[14:], layers.LayerTypeIPv4, gopacket.Default)
	if IPv4 := packet.Layer(layers.LayerTypeIPv4); IPv4 != nil {
		ipv4, _ := IPv4.(*layers.IPv4)
		IPv4 := ipv4.TTL
		return IPv4
	}
	return 7
}

// decodePacket decodes IPV6 L2 header in the packet and returns HopLimit. packetData[14:] to remove first 14 bytes of Ethernet header.
func decodePacket6(t *testing.T, packetData []byte) uint8 {
	t.Helper()
	packet := gopacket.NewPacket(packetData[14:], layers.LayerTypeIPv6, gopacket.Default)
	if IPv6 := packet.Layer(layers.LayerTypeIPv6); IPv6 != nil {
		ipv6, _ := IPv6.(*layers.IPv6)
		IPv6 := ipv6.HopLimit
		return IPv6
	}
	return 7
}

// testTraffic sends traffic flow for duration seconds and returns the
// number of packets sent out.
func testTraffic(t *testing.T, top gosnappi.Config, ate *ondatra.ATEDevice, flows []gosnappi.Flow, srcEndPoint gosnappi.Port, duration int, cs gosnappi.ControlState) int {
	t.Helper()
	top.Flows().Clear()
	for _, flow := range flows {
		flow.TxRx().Port().SetTxName(srcEndPoint.Name()).SetRxName(srcEndPoint.Name())
		flow.Metrics().SetEnable(true)
		top.Flows().Append(flow)
	}
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	time.Sleep(30 * time.Second)
	ate.OTG().StartTraffic(t)
	time.Sleep(time.Duration(duration) * time.Second)
	ate.OTG().StopTraffic(t)

	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.STOP)
	ate.OTG().SetControlState(t, cs)

	outPkts := gnmi.GetAll(t, ate.OTG(), gnmi.OTG().FlowAny().Counters().OutPkts().State())
	total := 0
	for _, count := range outPkts {
		total += int(count)
	}
	return total
}

// testPacketIn programs p4rt table entry and sends traffic related to Traceroute,
// then validates packetin message metadata and payload.
func testPacketIn(ctx context.Context, t *testing.T, args *testArgs, isIPv4 bool, cs gosnappi.ControlState, flowValues []*flowArgs, EgressPortMap map[string]bool) []float64 {
	leader := args.leader
	if isIPv4 {
		// Insert p4rtutils acl entry on the DUT
		if err := programmTableEntry(leader, args.packetIO, false, isIPv4); err != nil {
			t.Fatalf("There is error when programming entry")
		}
		// Delete p4rtutils acl entry on the device
		defer programmTableEntry(leader, args.packetIO, true, isIPv4)
	} else {
		// Insert p4rtutils acl entry on the DUT
		if err := programmTableEntry(leader, args.packetIO, false, false); err != nil {
			t.Fatalf("There is error when programming entry")
		}
		// Delete p4rtutils acl entry on the device
		defer programmTableEntry(leader, args.packetIO, true, false)
	}
	streamChan := args.leader.StreamChannelGet(&streamName)
	qSize := 12000
	streamChan.SetArbQSize(qSize)
	qSizeRead := streamChan.GetArbQSize()
	if qSize != qSizeRead {
		t.Errorf("Stream '%s' expecting Arbitration qSize(%d) Got (%d)",
			streamName, qSize, qSizeRead)
	}

	streamChan.SetPacketQSize(qSize)
	qSizeRead = streamChan.GetPacketQSize()
	if qSize != qSizeRead {
		t.Errorf("Stream '%s' expecting Packet qSize(%d) Got (%d)",
			streamName, qSize, qSizeRead)
	}

	// Send Traceroute traffic from ATE
	srcEndPoint := ateInterface(t, args.top, "port1")
	llAddress, found := gnmi.Watch(t, args.ate.OTG(), gnmi.OTG().Interface("atePort1"+".Eth").Ipv4Neighbor(portsIPv4["dut:port1"]).LinkLayerAddress().State(), time.Minute, func(val *ygnmi.Value[string]) bool {
		return val.IsPresent()
	}).Await(t)
	if !found {
		t.Fatalf("Could not get the LinkLayerAddress %s", llAddress)
	}
	dstMac, _ := llAddress.Val()
	var flow []gosnappi.Flow
	for _, flowValue := range flowValues {
		flow = append(flow, args.packetIO.GetTrafficFlow(args.ate, dstMac, isIPv4, 1, 300, 50, ipv4InnerDst, flowValue))
	}
	pktOut := testTraffic(t, args.top, args.ate, flow, srcEndPoint, 60, cs)
	var countPkts = map[string]int{"11": 0, "12": 0, "13": 0, "14": 0, "15": 0, "16": 0, "17": 0}

	packetInTests := []struct {
		desc     string
		client   *p4rt_client.P4RTClient
		wantPkts int
	}{{
		desc:     "PacketIn to Primary Controller",
		client:   leader,
		wantPkts: pktOut,
	}}

	t.Log("TTL/HopLimit 1")
	for _, test := range packetInTests {
		t.Run(test.desc, func(t *testing.T) {
			// Extract packets from PacketIn message sent to p4rt client
			_, packets, err := test.client.StreamChannelGetPackets(&streamName, uint64(test.wantPkts), 30*time.Second)
			if err != nil {
				t.Errorf("Unexpected error on fetchPackets: %v", err)
			}

			if test.wantPkts == 0 {
				return
			}

			gotPkts := 0
			t.Logf("Start to decode packet and compare with expected packets.")
			wantPacket := args.packetIO.GetPacketTemplate()

			for _, packet := range packets {
				if packet != nil {
					srcMAC, _, etherType := decodePacket(t, packet.Pkt.GetPayload())
					if etherType != layers.EthernetTypeIPv4 && etherType != layers.EthernetTypeIPv6 {
						continue
					}
					if !strings.EqualFold(srcMAC, tracerouteSrcMAC) {
						continue
					}
					if wantPacket.TTL != nil {
						// TTL/HopLimit comparison for IPV4 & IPV6
						if isIPv4 {
							captureTTL := decodePacket4(t, packet.Pkt.GetPayload())
							if captureTTL != TTL1 {
								t.Fatalf("Packet in PacketIn message is not matching wanted packet=IPV4 TTL1")
							}

						} else {
							captureHopLimit := decodePacket6(t, packet.Pkt.GetPayload())
							if captureHopLimit != HopLimit1 {
								t.Fatalf("Packet in PacketIn message is not matching wanted packet=IPV6 HopLimit1")
							}
						}
					}

					// Metadata comparision
					if metaData := packet.Pkt.GetMetadata(); metaData != nil {
						if got := metaData[0].GetMetadataId(); got == MetadataIngressPort {
							if gotPortID := string(metaData[0].GetValue()); gotPortID != args.packetIO.GetIngressPort() {
								t.Fatalf("Ingress Port Id mismatch: want %s, got %s", args.packetIO.GetIngressPort(), gotPortID)
							}
						} else {
							t.Fatalf("Metadata ingress port mismatch: want %d, got %d", MetadataIngressPort, got)
						}
						if got := metaData[1].GetMetadataId(); got == MetadataEgressPort {
							countPkts[string(metaData[1].GetValue())]++
							if gotPortID := string(metaData[1].GetValue()); !EgressPortMap[gotPortID] {
								t.Fatalf("Egress Port Id mismatch: got %s", gotPortID)
							}
						} else {
							t.Fatalf("Metadata egress port mismatch: want %d, got %d", MetadataEgressPort, got)
						}
					} else {
						t.Fatalf("Packet missing metadata information.")
					}
					gotPkts++
				}
			}
			if got, want := gotPkts, test.wantPkts; got != want {
				t.Errorf("Number of PacketIn, got: %d, want: %d", got, want)
			}
		})
	}
	loadBalancePercent := []float64{float64(countPkts["11"]) / float64(pktOut), float64(countPkts["12"]) / float64(pktOut),
		float64(countPkts["13"]) / float64(pktOut), float64(countPkts["14"]) / float64(pktOut), float64(countPkts["15"]) / float64(pktOut),
		float64(countPkts["16"]) / float64(pktOut), float64(countPkts["17"]) / float64(pktOut)}

	return loadBalancePercent
}

func ateInterface(t *testing.T, topo gosnappi.Config, portID string) gosnappi.Port {
	for _, p := range topo.Ports().Items() {
		if p.Name() == portID {
			return p
		}
	}
	return nil
}
