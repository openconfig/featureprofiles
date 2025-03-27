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
// package to test P4RT with traceroute traffic of IPV4 and IPV6 with TTL/HopLimit as 0&1.
// go test -v . -testbed /root/ondatra/featureprofiles/topologies/atedut_2.testbed -binding /root/ondatra/featureprofiles/topologies/atedut_2.binding -outputs_dir logs

package egressp4rt_test

import (
	"context"
	"testing"

	"github.com/cisco-open/go-p4/p4rt_client"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/openconfig/featureprofiles/internal/cisco/gribi"
	"github.com/openconfig/featureprofiles/internal/p4rtutils"
	"github.com/openconfig/ondatra"
	p4_v1 "github.com/p4lang/p4runtime/go/p4/v1"
)

type PacketIO interface {
	GetTableEntry(delete bool, isIPv4 bool) []*p4rtutils.ACLWbbIngressTableEntryInfo
	GetPacketTemplate() *PacketIOPacket
	GetTrafficFlow(ate *ondatra.ATEDevice, isIPv4 bool, TTL uint8, frameSize uint32, frameRate uint64) []*ondatra.Flow
	GetEgressPort() string
	GetIngressPort() string
}

type PacketIOPacket struct {
	TTL            *uint8
	SrcMAC, DstMAC *string
	EthernetType   *uint32
	HopLimit       *uint8
}

type testArgs struct {
	ctx      context.Context
	leader   *p4rt_client.P4RTClient
	follower *p4rt_client.P4RTClient
	dut      *ondatra.DUTDevice
	ate      *ondatra.ATEDevice
	packetIO PacketIO
	client   *gribi.Client
	top      *ondatra.ATETopology
}

// programmTableEntry programs or deletes p4rt table entry based on delete flag.
func programmTableEntry(client *p4rt_client.P4RTClient, packetIO PacketIO, delete bool, isIPv4 bool, deviceID uint64) error {
	err := client.Write(&p4_v1.WriteRequest{
		DeviceId:   deviceID,
		ElectionId: &p4_v1.Uint128{High: uint64(0), Low: electionId},
		Updates: p4rtutils.ACLWbbIngressTableEntryGet(
			packetIO.GetTableEntry(delete, isIPv4),
		),
		Atomicity: p4_v1.WriteRequest_CONTINUE_ON_ERROR,
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
