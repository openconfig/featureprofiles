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

package p4rt_test

import (
	"flag"
	"fmt"
	"log"
	"net"
	"testing"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	p4_v1 "github.com/p4lang/p4runtime/go/p4/v1"
	"wwwin-github.cisco.com/rehaddad/go-p4/p4info/wbb"
	p4rt "wwwin-github.cisco.com/rehaddad/go-p4/p4rt_client"
	"wwwin-github.cisco.com/rehaddad/go-p4/utils"
)

var (
	p4InfoFile = flag.String("p4info_file_location", "./wbb.p4info.pb.txt",
		"Path to the p4info file.")
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestP4RTClientIntegration(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	// ctx := context.Background()

	fmt.Println("Get p4rt client from ondatra...")
	ondatra_client := dut.RawAPIs().P4RT(t)

	fmt.Println("Get p4rt clinet from cisco...")
	p4rt_client := p4rt.P4RTClient{}
	p4rt_client.P4rtClientSet(ondatra_client)
	fmt.Println(p4rt_client)

	// Create Stream
	streamName := "Primary"
	streamParameter := p4rt.P4RTStreamParameters{
		Name:        streamName,
		DeviceId:    uint64(1),
		ElectionIdH: uint64(1),
		ElectionIdL: uint64(1),
	}
	p4rt_client.StreamChannelCreate(&streamParameter)
	err := p4rt_client.StreamChannelSendMsg(&streamName, &p4_v1.StreamMessageRequest{
		Update: &p4_v1.StreamMessageRequest_Arbitration{
			Arbitration: &p4_v1.MasterArbitrationUpdate{
				DeviceId: streamParameter.DeviceId,
				ElectionId: &p4_v1.Uint128{
					High: streamParameter.ElectionIdH,
					Low:  streamParameter.ElectionIdL,
				},
			},
		},
	})
	if err != nil {
		fmt.Println("There is error when sending the 1st msg!")

	}

	fmt.Println("checking primary state...")
	lastSeqNum0, arbMsg0, arbErr0 := p4rt_client.StreamChannelGetArbitrationResp(&streamName, 1)

	if arbErr0 != nil {
		log.Fatal(arbErr0)
	}
	fmt.Println(lastSeqNum0, arbMsg0)

	p4Info, _ := utils.P4InfoLoad(p4InfoFile)
	fmt.Println(p4Info)

	// Get Capbilities (for now, we just log it)
	_, err = p4rt_client.Capabilities(&p4_v1.CapabilitiesRequest{})
	if err != nil {
		fmt.Println("Capabilities err:", err)
	}

	// Set Forwarding pipeline
	// Not associated with any streams, but we have to use the primary's
	// Note, both arbMsg and arbMsg2 have the primary's Election Id
	err = p4rt_client.SetForwardingPipelineConfig(&p4_v1.SetForwardingPipelineConfigRequest{
		DeviceId:   arbMsg0.Arb.DeviceId,
		ElectionId: arbMsg0.Arb.ElectionId,
		Action:     p4_v1.SetForwardingPipelineConfigRequest_VERIFY_AND_COMMIT,
		Config: &p4_v1.ForwardingPipelineConfig{
			P4Info: &p4Info,
			Cookie: &p4_v1.ForwardingPipelineConfig_Cookie{
				Cookie: 159,
			},
		},
	})
	if err != nil {
		fmt.Println("There is error seen when setting SetForwardingPipelineConfig")
	}

	// Get Forwarding pipeline (for now, we just log it)
	get_response, err := p4rt_client.GetForwardingPipelineConfig(&p4_v1.GetForwardingPipelineConfigRequest{
		DeviceId:     arbMsg0.Arb.DeviceId,
		ResponseType: p4_v1.GetForwardingPipelineConfigRequest_ALL,
	})
	if err != nil {
		fmt.Println("There is error seen when setting GetForwardingPipelineConfig")
	}
	fmt.Println("GetForwardingPipelineConfig Response: ", get_response)

	// Write is not associated with any streams, but we have to use the primary's
	err = p4rt_client.Write(&p4_v1.WriteRequest{
		DeviceId:   arbMsg0.Arb.DeviceId,
		ElectionId: arbMsg0.Arb.ElectionId,
		Updates: wbb.AclWbbIngressTableEntryGet([]*wbb.AclWbbIngressTableEntryInfo{
			&wbb.AclWbbIngressTableEntryInfo{
				Type:          p4_v1.Update_INSERT,
				EtherType:     0x6007,
				EtherTypeMask: 0xFFFF,
			},
			&wbb.AclWbbIngressTableEntryInfo{
				Type:    p4_v1.Update_INSERT,
				IsIpv4:  0x1,
				Ttl:     0x1,
				TtlMask: 0xFF,
			},
		}),
		Atomicity: p4_v1.WriteRequest_CONTINUE_ON_ERROR,
	})
	if err != nil {
		fmt.Println("There is error when writing entries...", err)
	}

	fmt.Println("Reading Entries: ")
	// Read ALL and log
	rStream, rErr := p4rt_client.Read(&p4_v1.ReadRequest{
		DeviceId: arbMsg0.Arb.DeviceId,
		Entities: []*p4_v1.Entity{
			&p4_v1.Entity{
				Entity: &p4_v1.Entity_TableEntry{},
			},
		},
	})
	if rErr != nil {
		fmt.Println("There is error when Reading entries...", rErr)
	}
	// fmt.Println("Length:", len())
	for {
		readResp, respErr := rStream.Recv()
		if respErr != nil {
			fmt.Println("Read Response Err: ", respErr)
			break
		} else {
			fmt.Println("Read Response: ", readResp)
		}
	}

	// Send L3 packet to ingress (on Primary channel)
	err = p4rt_client.StreamChannelSendMsg(
		&streamName, &p4_v1.StreamMessageRequest{
			Update: &p4_v1.StreamMessageRequest_Packet{
				Packet: &p4_v1.PacketOut{
					Payload: utils.PacketICMPEchoRequestGet(false,
						net.HardwareAddr{0xFF, 0xAA, 0xFA, 0xAA, 0xFF, 0xAA},
						net.HardwareAddr{0xBD, 0xBD, 0xBD, 0xBD, 0xBD, 0xBD},
						net.IP{10, 0, 0, 1},
						net.IP{10, 0, 0, 2},
						64),
					Metadata: []*p4_v1.PacketMetadata{
						&p4_v1.PacketMetadata{
							MetadataId: 2, // "submit_to_ingress"
							Value:      []byte{0x1},
						},
					},
				},
			},
		})
	if err != nil {
		log.Fatal(err)
	}
}
