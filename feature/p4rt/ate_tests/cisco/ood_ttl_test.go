package cisco_p4rt_test

import (
	"fmt"
	"net"
	"testing"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/openconfig/ondatra"
	p4_v1 "github.com/p4lang/p4runtime/go/p4/v1"
	"wwwin-github.cisco.com/rehaddad/go-wbb/p4info/wbb"
)

var (
	OODTTLTestcases = []Testcase{
		{
			name: "Check PacketIn Without Program TTL Match Entry",
			desc: "Packet I/O-Traceroute-PacketIn:001 Verify TTL=[0,1,2] packet from tgen is fwd/drop on the device and not sent to the controller",
			fn:   testPacketInWithoutEntryProgramming,
		},
		{
			name: "Program TTL Match Entry and Check PacketIn",
			desc: "Packet I/O-Traceroute-PacketIn:002-003-004-005-012-025-026 Programm match TTL=[1,2], send outter TTL=[0,1,2] packet from tgen and verify packet(only TTL=1) is sent to primary controller",
			fn:   testEntryProgrammingPacketIn,
		},
		{
			name: "Program TTL Match Entry and check PacketIn and verify no ICMP reply",
			desc: "Packet I/O-Traceroute-PacketIn:006 Programm match TTL=[1,2], send outter TTL=[0,1,2] packet from tgen and verify router doesn't sent ICMP back ",
			fn:   testEntryProgrammingPacketInAndNoReply,
		},
		{
			name: "Program TTL Match Entry and Sent traffic with inner TTL1 and check PacketIn",
			desc: "Packet I/O-Traceroute-PacketIn:008 Programm match TTL=[1,2], send inner TTL=[0,1,2] packet from tgen and verify packet is NOT sent to primary controller",
			fn:   testEntryProgrammingPacketInWithInnerTTL,
		},
		{
			name: "Program TTL Match Entry and Sent traffic to forus IP and check PacketIn",
			desc: "Packet I/O-Traceroute-PacketIn:009 Programm match TTL=[1,2], send outter TTL=[0,1,2] with destnation IP as local IP(for us IP) packet from tgen and verify traffic is not sent to controller",
			fn:   testEntryProgrammingPacketInWithForUsIP,
		},
		{
			name: "Program TTL Match Entry and Sent traffic to non-exist IP and check PacketIn",
			desc: "Packet I/O-Traceroute-PacketIn:010 Programm match TTL=[1,2], send outter TTL=[0,1,2] with destnation IP not in lpm packet from tgen and verify traffic is not sent to controller",
			fn:   testEntryProgrammingPacketInWithNonExistIP,
		},
		{
			name: "Program TTL Match Entry and Sent traffic to physical interface and check PacketIn",
			desc: "Packet I/O-Traceroute-PacketIn:011 Programm match TTL=[1,2], send outter TTL=[0,1,2] packet from tgen to the physcial interface and verify packet(only TTL=1) is sent to primary controller",
			fn:   testEntryProgrammingPacketInWithPhysicalInterface,
		},
		{
			name: "Program TTL Match Entry and Sent Traffic to bundle subinterface and check PacketIn",
			desc: "Packet I/O-Traceroute-PacketIn:014 Programm match TTL=[1,2], send outter TTL=[0,1,2] packet from tgen to both main interface and sub interface and verify packet(only TTL=1) received on main interface is sent to primary controller",
			fn:   testEntryProgrammingPacketInWithSubInterface,
		},
		{
			name: "Program TTL Match Entry and Sent TTL3 traffic and check PacketIn",
			desc: "Packet I/O-Traceroute-PacketIn:015 Programm match TTL=[1,2], send outter TTL≠[0,1,2] packet from tgen and those packets are not sent to controller",
			fn:   testEntryProgrammingPacketInWithOutterTTL3,
		},
		{
			name: "Program TTL Match Entry and Trigger Traceroute and check PacketIn",
			desc: "Packet I/O-Traceroute-PacketIn:018 Programm match TTL=[1,2], and initiate Ping/Traceroute with TTL=[0,1,2] from DUT, verify the packet is NOT sent to controller",
			fn:   testEntryProgrammingPacketInWithGNOI,
		},
		{
			name: "Program TTL Match Entry and Sent malformed traffic and check PacketIn",
			desc: "Packet I/O-Traceroute-PacketIn:019 Programm match TTL=[1,2], and sent malformed packet with TTL=[0,1,2] from tgen, verify the packet is not sent to controller",
			fn:   testEntryProgrammingPacketInWithMalformedPacket,
		},
		{
			name: "Program TTL Match Entry and remove TTL Match and check PacketIn",
			desc: "Packet I/O-Traceroute-PacketIn:020 Programm match TTL=[1,2], send outter TTL=[0,1,2] packet from tgen, delete/remove the table entry on the same NP, verify the packet is not sent to controller",
			fn:   testEntryProgrammingPacketInThenRemoveEntry,
		},
		{
			name: "Program TTL Match Entry and Send ICMP Traceroute Packet and Check PacketIn",
			desc: "Packet I/O-Traceroute-PacketIn:023 Programm match TTL=[1,2], simulate ICMP/Traceroute packets with TTL=1,2 from tgen, verify the packet is sent to the controller, verify ICMP packet not sent out",
			fn:   testEntryProgrammingPacketInWithUDP,
		},
		{
			name: "Program TTL Match Entry and other match fields and Check PacketIn",
			desc: "Packet I/O-Traceroute-PacketIn:027 Programm match TTL=[1,2] with other field matched in the entry, send TTL=[0,1,2] packets from tgen, and verify the packet sent to the controller",
			fn:   testEntryProgrammingPacketInWithMoreMatchingField,
		},
		{
			name: "Program Match Entry and Send traffic to non-configured port in P4RT and Check PacketIn",
			desc: "Packet I/O-Traceroute-PacketIn:029 Programm match TTL=[1,2], non-configured port-id on the configured npu, TTL=1 to the non-configured port will be punted but dropped",
			fn:   testEntryProgrammingPacketInWithouthPortID,
		},
		{
			name: "Program Match Entry and Send traffic to non-configured port in P4RT and then configure port-id and Check PacketIn",
			desc: "Packet I/O-Traceroute-PacketIn:030 Programm match TTL=[1,2], non-configured port-id on the configured npu, TTL=1 to the non-configured port will be punted but dropped, then verify it starts working after adding the port-id",
			fn:   testEntryProgrammingPacketInWithouthPortIDThenAddPortID,
		},
		{
			name: "Program TTL Match Entry and Downgrade primary controller and Check PacketIn",
			desc: "Packet I/O-Traceroute-PacketIn:032 Programm match TTL=[1,2], send packets with TTL=[0,1,2] from tgen, downgrade/fail the primary controller, verify the packets are sent to the backup controller if there is backup controller",
			fn:   testEntryProgrammingPacketInDowngradePrimaryController,
		},
		{
			name: "Program TTL Match Entry and Downgrade primary controller without backup controller and Check PacketIn",
			desc: "Packet I/O-Traceroute-PacketIn:033 Programm match TTL=[1,2], send packets with TTL=[0,1,2] from tgen, downgrade/fail the primary controller, verify the packets are not sent out if there is no backup controller",
			fn:   testEntryProgrammingPacketInDowngradePrimaryControllerWithoutStandby,
		},
		{
			name: "Program TTL Match Entry and Recover previous primary controller and Check PacketIn",
			desc: "Packet I/O-Traceroute-PacketIn:034 Programm match TTL=[1,2], send packets with TTL=[0,1,2] from tgen, fail the primary controller and then recover the controller, verify the packets are sent to the primary controller",
			fn:   testEntryProgrammingPacketInRecoverPrimaryController,
		},
		{
			name: "Program TTL Match Entry and Send traffic with Flowlabel and Check PacketIn",
			desc: "Packet I/O-Traceroute-PacketIn:036 Programm match TTL=[1,2], send IPv6 packets with TTL=[0,1,2] with flow-label/SRH from tgen, verify those packets sent to controller",
			fn:   testEntryProgrammingPacketInWithFlowLabel,
		},
		{
			name: "Program TTL Match Entry and Send scale TTL traffic",
			desc: "Packet I/O-Traceroute-PacketIn:038 Verify scale rate of TTL=[1,2] packets (198 pps)",
			fn:   testEntryProgrammingPacketInScaleRate,
		},
	}
)

func packetTTLRequestGet(t *testing.T) []byte {
	t.Helper()
	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{
		FixLengths:       true,
		ComputeChecksums: true,
	}
	pktEth := &layers.Ethernet{
		SrcMAC: net.HardwareAddr{0x00, 0xAA, 0x00, 0xAA, 0x00, 0xAA},
		//01:80:C2:00:00:0E
		DstMAC:       net.HardwareAddr{0x01, 0x80, 0xC2, 0x00, 0x00, 0x0E},
		EthernetType: lldpInLayers,
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

type TTLPacketIO struct {
	PacketIOPacket
	NeedConfig  *bool
	IPv4        bool
	IPv6        bool
	TtlTwo      bool
	EgressPorts []string
	IngressPort string
}

func (ttl *TTLPacketIO) GetTableEntry(t *testing.T, delete bool) []*wbb.AclWbbIngressTableEntryInfo {
	actionType := p4_v1.Update_INSERT
	if delete {
		actionType = p4_v1.Update_DELETE
	}
	entries := []*wbb.AclWbbIngressTableEntryInfo{}
	if ttl.IPv4 {
		entries = append(entries, &wbb.AclWbbIngressTableEntryInfo{
			Type:   actionType,
			IsIpv4: uint8(1),
			Ttl:    uint8(1),
		})
		if ttl.TtlTwo {
			entries = append(entries, &wbb.AclWbbIngressTableEntryInfo{
				Type:   actionType,
				IsIpv4: uint8(1),
				Ttl:    uint8(2),
			})
		}
	}
	if ttl.IPv6 {
		entries = append(entries, &wbb.AclWbbIngressTableEntryInfo{
			Type:   actionType,
			IsIpv6: uint8(1),
			Ttl:    uint8(1),
		})
		if ttl.TtlTwo {
			entries = append(entries, &wbb.AclWbbIngressTableEntryInfo{
				Type:   actionType,
				IsIpv4: uint8(1),
				Ttl:    uint8(2),
			})
		}
	}

	return entries
}

func (ttl *TTLPacketIO) ApplyConfig(t *testing.T, dut *ondatra.DUTDevice, delete bool) {
	t.Logf("There is no configuration required")
}

func (ttl *TTLPacketIO) GetPacketOut(t *testing.T, portID uint32, submitIngress bool) *p4_v1.PacketOut {
	packet := &p4_v1.PacketOut{
		Payload: packetTTLRequestGet(t),
		Metadata: []*p4_v1.PacketMetadata{
			&p4_v1.PacketMetadata{
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
				// Value:      []byte(fmt.Sprint(0)),
			})
	}
	// else {
	// 	packet.Metadata = append(packet.Metadata,
	// 		&p4_v1.PacketMetadata{
	// 			MetadataId: uint32(2), // "submit_to_ingress"
	// 			Value:      []byte(fmt.Sprint(0)),
	// 		})
	// }
	return packet
}

func (ttl *TTLPacketIO) GetPacketTemplate(t *testing.T) *PacketIOPacket {
	return &ttl.PacketIOPacket
}

func (ttl *TTLPacketIO) GetTrafficFlow(t *testing.T, ate *ondatra.ATEDevice, frameSize uint32, frameRate uint64) []*ondatra.Flow {

	flows := []*ondatra.Flow{}

	ethHeader := ondatra.NewEthernetHeader()
	ethHeader.WithSrcAddress(*ttl.SrcMAC)
	ethHeader.WithDstAddress(*ttl.DstMAC)

	if ttl.IPv4 {
		ipv4 := ondatra.NewIPv4Header()
		ipv4.WithSrcAddress(*ttl.SrcIPv4)
		ipv4.WithDstAddress(*ttl.DstIPv4)
		ipv4.WithTTL(uint8(*ttl.TTL))
		flow := ate.Traffic().NewFlow("TTL-IPv4").WithFrameSize(frameSize).WithFrameRateFPS(frameRate).WithHeaders(ethHeader, ipv4)
		flows = append(flows, flow)
	}

	if ttl.IPv6 {
		ipv6 := ondatra.NewIPv6Header()
		ipv6.WithSrcAddress(*ttl.SrcIPv6)
		ipv6.WithDstAddress(*ttl.DstIPv6)
		ipv6.WithHopLimit(uint8(*ttl.TTL))
		flow := ate.Traffic().NewFlow("TTL-IPv6").WithFrameSize(frameSize).WithFrameRateFPS(frameRate).WithHeaders(ethHeader, ipv6)
		flows = append(flows, flow)
	}

	return flows
}

func (ttl *TTLPacketIO) GetEgressPort(t *testing.T) []string {
	return ttl.EgressPorts
}

func (ttl *TTLPacketIO) SetEgressPorts(t *testing.T, portIDs []string) {
	ttl.EgressPorts = portIDs
}

func (ttl *TTLPacketIO) GetIngressPort(t *testing.T) string {
	return ttl.IngressPort
}

func (ttl *TTLPacketIO) SetIngressPorts(t *testing.T, portID string) {
	ttl.IngressPort = portID
}

func (ttl *TTLPacketIO) GetPacketIOPacket(t *testing.T) *PacketIOPacket {
	return &ttl.PacketIOPacket
}
