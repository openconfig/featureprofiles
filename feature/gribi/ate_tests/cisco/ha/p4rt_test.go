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
)

var (
	p4InfoFile                      = flag.String("p4info_file_location", "wbb.p4info.pb.txt", "Path to the p4info file.")
	streamName                      = "p4rt"
	gdpInLayers layers.EthernetType = 0x6007
	deviceId                        = *ygot.Uint64(1)
	portId                          = *ygot.Uint32(10)
	electionId                      = *ygot.Uint64(100)
)

const (
	packetOutDuration = 60 * time.Second
)

type GDPPacketIO struct {
	PacketIO
	IngressPort string
}

type PacketIO interface {
	GetPacketOut(portID uint32, submitIngress bool) []*p4_v1.PacketOut
}

// configureDeviceId configures p4rt device-id on the DUT.
func configureDeviceId(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice) {
	component := oc.Component{}
	component.IntegratedCircuit = &oc.Component_IntegratedCircuit{}
	component.Name = ygot.String(*p4rtNodeName)
	component.IntegratedCircuit.NodeId = ygot.Uint64(deviceId)
	gnmi.Replace(t, dut, gnmi.OC().Component(*p4rtNodeName).Config(), &component)
}

// configurePortId configures p4rt port-id on the DUT.
func configurePortId(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice) {
	gnmi.Update(t, dut, gnmi.OC().Interface(dut.Port(t, "port1").Name()).Id().Config(), portId)
}

func setupP4RTClients(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice) ([]*p4rt_client.P4RTClient, error) {
	leader := p4rt_client.P4RTClient{}
	if err := leader.P4rtClientSet(dut.RawAPIs().P4RT().Default(t)); err != nil {
		t.Fatalf("Could not initialize p4rt client: %v", err)
	}

	follower := p4rt_client.P4RTClient{}
	if err := follower.P4rtClientSet(dut.RawAPIs().P4RT().Default(t)); err != nil {
		t.Fatalf("Could not initialize p4rt client: %v", err)
	}

	// Setup p4rt-client stream parameters
	streamParameter := p4rt_client.P4RTStreamParameters{
		Name:        streamName,
		DeviceId:    deviceId,
		ElectionIdH: uint64(0),
		ElectionIdL: electionId,
	}

	// Send ClientArbitration message on both p4rt leader and follower clients.
	clients := []*p4rt_client.P4RTClient{&leader, &follower}
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
				return clients, errors.New("Errors seen when sending ClientArbitration message.")
			}
			if _, _, arbErr := client.StreamChannelGetArbitrationResp(&streamName, 1); arbErr != nil {
				return clients, errors.New("Errors seen in ClientArbitration response.")
			}
		}
	}

	// Load p4info file.
	p4Info, err := utils.P4InfoLoad(p4InfoFile)
	if err != nil {
		return clients, errors.New("Errors seen when loading p4info file.")
	}

	// Send SetForwardingPipelineConfig for p4rt leader client.
	if err := leader.SetForwardingPipelineConfig(&p4_v1.SetForwardingPipelineConfigRequest{
		DeviceId:   deviceId,
		ElectionId: &p4_v1.Uint128{High: uint64(0), Low: electionId},
		Action:     p4_v1.SetForwardingPipelineConfigRequest_VERIFY_AND_COMMIT,
		Config: &p4_v1.ForwardingPipelineConfig{
			P4Info: p4Info,
			Cookie: &p4_v1.ForwardingPipelineConfig_Cookie{
				Cookie: 159,
			},
		},
	}); err != nil {
		return clients, errors.New("Errors seen when sending SetForwardingPipelineConfig.")
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

func getGDPParameter(t *testing.T) PacketIO {
	return &GDPPacketIO{
		IngressPort: fmt.Sprint(portId),
	}
}

func p4rtPacketOut(t *testing.T, events *monitor.CachedConsumer, args ...interface{}) {
	arg := args[0].(*testArgs)
	startTime := time.Now()
	t.Logf("P4RTPacketOut test started at: %v", startTime)

	dut := arg.dut
	ate := arg.ate
	ctx := arg.ctx

	clients, err := setupP4RTClients(ctx, t, dut)
	if err != nil {
		t.Errorf("There is error setting up p4rt clients. %v, %s", err, err)
	}

	leader := clients[0]
	follower := clients[1]

	port := ate.Port(t, "port1").Name()
	packetIO := getGDPParameter(t)

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
			counter0 := gnmi.Get(t, arg.ate, gnmi.OC().Interface(port).Counters().InPkts().State())

			packets := packetIO.GetPacketOut(portId, false)

			packetCount := 0
			for start := time.Now(); time.Since(start) < packetOutDuration; {
				for _, packet := range packets {
					if err := test.client.StreamChannelSendMsg(
						&streamName, &p4_v1.StreamMessageRequest{
							Update: &p4_v1.StreamMessageRequest_Packet{
								Packet: packet,
							},
						}); err != nil {
						t.Errorf("There is error seen in Packet Out. %v, %s", err, err)
					}
					packetCount += len(packets)
				}
			}

			// Wait for ate stats to be populated
			time.Sleep(60 * time.Second)

			// Check packet counters after packet out
			counter1 := gnmi.Get(t, arg.ate, gnmi.OC().Interface(port).Counters().InPkts().State())

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

	endTime := time.Now()
	t.Logf("P4RTPacketOut test completed at %v (Total: %v)", endTime, time.Since(startTime))
}
