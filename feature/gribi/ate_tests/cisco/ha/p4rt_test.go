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

package ha_test

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/cisco-open/go-p4/p4rt_client"
	"github.com/cisco-open/go-p4/utils"
	"github.com/google/gopacket/layers"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"

	p4_v1 "github.com/p4lang/p4runtime/go/p4/v1"

	"github.com/google/gopacket"
	"github.com/openconfig/featureprofiles/internal/cisco/ha/monitor"
	"github.com/openconfig/featureprofiles/internal/p4rtutils"
)

const (
	packetCount = 100
)

var (
	p4InfoFile                                = flag.String("p4info_file_location", "./wbb.p4info.pb.txt", "Path to the p4info file.")
	streamName                                = "p4rt"
	gdpInLayers           layers.EthernetType = 0x6007
	deviceID                                  = *ygot.Uint64(1)
	portID                                    = *ygot.Uint32(10)
	electionID                                = *ygot.Uint64(100)
	METADATA_INGRESS_PORT                     = *ygot.Uint32(1)
	METADATA_EGRESS_PORT                      = *ygot.Uint32(2)
	SUBMIT_TO_INGRESS                         = *ygot.Uint32(1)
	SUBMIT_TO_EGRESS                          = *ygot.Uint32(0)
)

type GDPPacketIO struct {
	PacketIO
	IngressPort string
}

type PacketIO interface {
	GetTableEntry(delete bool) []*p4rtutils.ACLWbbIngressTableEntryInfo
	GetPacketOut(portID uint32, submitIngress bool) []*p4_v1.PacketOut
}

// programmTableEntry programs or deletes p4rt table entry based on delete flag.
func programmTableEntry(ctx context.Context, t *testing.T, client *p4rt_client.P4RTClient, packetIO PacketIO, delete bool) error {
	t.Helper()
	err := client.Write(&p4_v1.WriteRequest{
		DeviceId:   deviceID,
		ElectionId: &p4_v1.Uint128{High: uint64(0), Low: electionID},
		Updates: p4rtutils.ACLWbbIngressTableEntryGet(
			packetIO.GetTableEntry(delete),
		),
		Atomicity: p4_v1.WriteRequest_CONTINUE_ON_ERROR,
	})
	if err != nil {
		return err
	}
	return nil
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

// configureDeviceId configures p4rt device-id on the DUT.
func configureDeviceId(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice) {
	nodes := p4rtutils.P4RTNodesByPort(t, dut)
	p4rtNode, ok := nodes["port1"]
	if !ok {
		t.Fatal("Couldn't find P4RT Node for port: port1")
	}
	t.Logf("Configuring P4RT Node: %s", p4rtNode)
	c := oc.Component{}
	c.Name = ygot.String(p4rtNode)
	c.IntegratedCircuit = &oc.Component_IntegratedCircuit{}
	c.IntegratedCircuit.NodeId = ygot.Uint64(deviceID)
	gnmi.Replace(t, dut, gnmi.OC().Component(p4rtNode).Config(), &c)
}

func setupP4RTClients(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice) ([]*p4rt_client.P4RTClient, error) {
	// Setup p4rt-client stream parameters
	streamParameter := p4rt_client.P4RTStreamParameters{
		Name:        streamName,
		DeviceId:    deviceID,
		ElectionIdH: uint64(0),
		ElectionIdL: electionID,
	}

	leader := p4rt_client.NewP4RTClient(&p4rt_client.P4RTClientParameters{})
	if err := leader.P4rtClientSet(dut.RawAPIs().P4RT(t)); err != nil {
		t.Fatalf("Could not initialize p4rt client: %v", err)
	}

	follower := p4rt_client.NewP4RTClient(&p4rt_client.P4RTClientParameters{})
	if err := follower.P4rtClientSet(dut.RawAPIs().P4RT(t)); err != nil {
		t.Fatalf("Could not initialize p4rt client: %v", err)
	}

	// Send ClientArbitration message on both p4rt leader and follower clients.
	clients := []*p4rt_client.P4RTClient{leader, follower}
	for index, client := range clients {
		if client != nil {
			client.StreamChannelCreate(&streamParameter)
			if err := client.StreamChannelSendMsg(&streamName, &p4_v1.StreamMessageRequest{
				Update: &p4_v1.StreamMessageRequest_Arbitration{
					Arbitration: &p4_v1.MasterArbitrationUpdate{
						DeviceId: streamParameter.DeviceId,
						ElectionId: &p4_v1.Uint128{
							High: streamParameter.ElectionIdH,
							Low:  streamParameter.ElectionIdL - uint64(index),
						},
					},
				},
			}); err != nil {
				return clients, fmt.Errorf("errors seen when sending ClientArbitration message: %v", err)
			}
			if _, _, arbErr := client.StreamChannelGetArbitrationResp(&streamName, 1); arbErr != nil {
				return clients, fmt.Errorf("errors seen in ClientArbitration response: %v", arbErr)
			}
		}
	}

	// Load p4info file.
	p4Info, err := utils.P4InfoLoad(p4InfoFile)
	if err != nil {
		return clients, errors.New("errors seen when loading p4info file")
	}

	// Send SetForwardingPipelineConfig for p4rt leader client.
	if err := leader.SetForwardingPipelineConfig(&p4_v1.SetForwardingPipelineConfigRequest{
		DeviceId:   deviceID,
		ElectionId: &p4_v1.Uint128{High: uint64(0), Low: electionID},
		Action:     p4_v1.SetForwardingPipelineConfigRequest_VERIFY_AND_COMMIT,
		Config: &p4_v1.ForwardingPipelineConfig{
			P4Info: p4Info,
			Cookie: &p4_v1.ForwardingPipelineConfig_Cookie{
				Cookie: 159,
			},
		},
	}); err != nil {
		return clients, errors.New("errors seen when sending SetForwardingPipelineConfig")
	}
	return clients, nil
}

func packetGDPRequestGet() []byte {
	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{
		FixLengths:       true,
		ComputeChecksums: true,
	}
	pktEth := &layers.Ethernet{
		SrcMAC: net.HardwareAddr{0x00, 0xAA, 0x00, 0xAA, 0x00, 0xAA},
		// GDP MAC is 00:0A:DA:F0:F0:F0
		DstMAC:       net.HardwareAddr{0x00, 0x0A, 0xDA, 0xF0, 0xF0, 0xF0},
		EthernetType: gdpInLayers,
	}
	payload := []byte{}
	payLoadLen := 64
	for i := 0; i < payLoadLen; i++ {
		payload = append(payload, byte(i))
	}
	gopacket.SerializeLayers(buf, opts,
		pktEth, gopacket.Payload(payload),
	)
	return buf.Bytes()
}

// GetTableEntry creates wbb acl entry related to GDP.
func (gdp *GDPPacketIO) GetTableEntry(delete bool) []*p4rtutils.ACLWbbIngressTableEntryInfo {
	actionType := p4_v1.Update_INSERT
	if delete {
		actionType = p4_v1.Update_DELETE
	}
	return []*p4rtutils.ACLWbbIngressTableEntryInfo{{
		Type:          actionType,
		EtherType:     0x6007,
		EtherTypeMask: 0xFFFF,
		Priority:      1,
	}}
}

// GetPacketOut generates PacketOut message with payload as GDP.
func (gdp *GDPPacketIO) GetPacketOut(portID uint32, submitIngress bool) []*p4_v1.PacketOut {
	packets := []*p4_v1.PacketOut{}
	packet := &p4_v1.PacketOut{
		Payload: packetGDPRequestGet(),
		Metadata: []*p4_v1.PacketMetadata{
			{
				MetadataId: uint32(1), // "egress_port"
				Value:      []byte(fmt.Sprint(portID)),
			},
		},
	}
	if submitIngress {
		packet.Metadata = append(packet.Metadata,
			&p4_v1.PacketMetadata{
				MetadataId: uint32(2), // "submit_to_ingress"
				Value:      []byte{1},
			})
	}
	packets = append(packets, packet)
	return packets
}

// getGDPParameter returns GDP related parameters for testPacketOut testcase.
func getGDPParameter(t *testing.T) PacketIO {
	return &GDPPacketIO{
		IngressPort: fmt.Sprint(portID),
	}
}

func p4rtPacketOut(t *testing.T, events *monitor.CachedConsumer, args ...interface{}) {
	arg := args[0].(*testArgs)
	startTime := time.Now()
	t.Logf("P4RTPacketOut test started at: %v", startTime)

	dut := arg.dut
	ate := arg.ate
	ctx := arg.ctx

	p1 := dut.Port(t, "port1")
	i1 := &oc.Interface{Name: ygot.String(p1.Name()), Id: ygot.Uint32(portID)}
	gnmi.Update(t, dut, gnmi.OC().Interface(p1.Name()).Config(), i1)

	p2 := dut.Port(t, "port2")
	i2 := &oc.Interface{Name: ygot.String(p2.Name()), Id: ygot.Uint32(portID + 1)}
	gnmi.Update(t, dut, gnmi.OC().Interface(p2.Name()).Config(), i2)

	// Configure P4RT device-id
	configureDeviceId(ctx, t, dut)

	clients, err := setupP4RTClients(ctx, t, dut)
	if err != nil {
		t.Errorf("There is error setting up p4rt clients. %v, %s", err, err)
	}

	leader := clients[0]
	follower := clients[1]

	packetIO := getGDPParameter(t)

	// Insert wbb acl entry on the DUT
	if err := programmTableEntry(ctx, t, leader, packetIO, false); err != nil {
		t.Fatalf("There is error when programming entry")
	}
	// Delete wbb acl entry on the device
	defer programmTableEntry(ctx, t, leader, packetIO, true)

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
			port := sortPorts(ate.Ports())[0].Name()
			counter0 := gnmi.Get(t, ate, gnmi.OC().Interface(port).Counters().InPkts().State())

			packets := packetIO.GetPacketOut(portID, false)
			sendPackets(t, test.client, packets, packetCount)

			// Wait for ate stats to be populated
			time.Sleep(60 * time.Second)

			// Check packet counters after packet out
			counter1 := gnmi.Get(t, ate, gnmi.OC().Interface(port).Counters().InPkts().State())

			// Verify InPkts stats to check P4RT stream
			t.Logf("Received %v packets on ATE port %s", counter1-counter0, port)

			if test.expectPass {
				if counter1-counter0 < uint64(float64(packetCount)*0.95) {
					t.Fatalf("Not all the packets are received.")
				}
			} else {
				if counter1-counter0 > uint64(float64(packetCount)*0.10) {
					t.Fatalf("Unexpected packets are received.")
				}
			}
		})
	}
}
