package cisco_p4rt_test

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"testing"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	wbb "github.com/openconfig/featureprofiles/feature/experimental/p4rt/internal/p4rtutils"
	"github.com/openconfig/ondatra"
	p4_v1 "github.com/p4lang/p4runtime/go/p4/v1"
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
			// skip: true,
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
			// skip: true,
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
			// skip: true,
		},
		{
			name: "Program TTL Match Entry and Downgrade primary controller and Check PacketIn",
			desc: "Packet I/O-Traceroute-PacketIn:032 Programm match TTL=[1,2], send packets with TTL=[0,1,2] from tgen, downgrade/fail the primary controller, verify the packets are sent to the backup controller if there is backup controller",
			fn:   testEntryProgrammingPacketInDowngradePrimaryController,
			// skip: true,
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
			// skip: true,
		},
		{
			name: "Program TTL Match Entry and Configure ACL and Check PacketIn",
			desc: "Packet I/O-Traceroute-PacketIn:037 Programm match TTL=[1,2], configured IPv4 ACL to match TTL=1 and set action to drop, verify packets dropped and not sent to controller",
			fn:   testEntryProgrammingPacketInWithAcl,
			// skip: true,
		},
		{
			name: "Program TTL Match Entry and Send scale TTL traffic",
			desc: "Packet I/O-Traceroute-PacketIn:038 Verify scale rate of TTL=[1,2] packets (198 pps)",
			fn:   testEntryProgrammingPacketInScaleRate,
		},
		{
			name: "Check PacketOut Without Programming TTL Match Entry(submit_to_ingress)",
			desc: "Packet I/O-Traceroute-PacketOut:001-002 Ingress: Inject EtherType 0x6007 packets and verify traffic sends to related port in case of EtherType 0x6007 entry NOT programmed",
			fn:   testPacketOutWithoutMatchEntry,
			// skip: true,
		},
		{
			name: "Program TTL Match Entry and Check PacketOut(submit_to_ingress)",
			desc: "Packet I/O-Traceroute-PacketOut:004 Ingress: Programm match TTL=[1,2], inject packets with TTL>3, and verify packet fwding based fwding chain on the router side",
			fn:   testPacketOut,
			// skip: true,
		},
		{
			name: "Program TTL Match Entry and Check PacketOut With TTL1 (submit_to_ingress)",
			desc: "Packet I/O-Traceroute-PacketOut:005-024 Ingress: Programm match TTL=[1,2], inject packets with TTL>3, and verify packet fwding based fwding chain on the router side",
			fn:   testPacketOutTTLOneWithoutMatchEntry,
			// skip: true,
		},
		{
			name: "Program TTL Match Entry and Check PacketOut With TTL1 With For-Us-IP(submit_to_ingress)",
			desc: "Packet I/O-Traceroute-PacketOut:006 Ingress: Programm match TTL=[1,2], inject ICMP/Traceroute packets with TTL=[0,1,2], verify if the packets go out for 1/2, 0 case not sent out",
			fn:   testPacketOutTTLOneWithUDP,
			// skip: true,
		},
		{
			name: "Program TTL Match Entry and Check PacketOut With TTL1 With For-Us-IP submit to ingress",
			desc: "Packet I/O-Traceroute-PacketOut:007 Ingress: dst IP is for us for the incoming packet, packet goes through lpts",
			fn:   testPacketOutWithForUsIP,
			// skip: true,
		},
		{
			name: "Program TTL Match Entry and Check PacketOut With TTL1 Traffic With For-Us-IP(submit_to_ingress)",
			desc: "Packet I/O-Traceroute-PacketOut:007 Ingress: dst IP is for us for the incoming packet, packet goes through lpts",
			fn:   testPacketOutTTLOneWithForUsIP,
			// skip: true,
		},
		{
			name: "Check PacketOut Without Programming TTL Match Entry(submit_to_egress)",
			desc: "Packet I/O-Traceroute-PacketOut:012-013 Egress: Without any match entries, Injecting IP packet with any TTL, verify packets sent out on those egress interfaces",
			fn:   testPacketOutEgressWithoutMatchEntry,
		},
		{
			name: "Check PacketOut Without Programming TTL Match Entry with Static Route(submit_to_egress)",
			desc: "Packet I/O-Traceroute-PacketOut:014-015 Egress: Without any match entries, Injecting IPv4/IPv6 packet with any TTL and configure null0 static router for the packet destination, verify packets sent out on those egress interfaces",
			fn:   testPacketOutTTLOneWithStaticroute,
			// skip: true,
		},
		{
			name: "Program TTL Match Entry and Check PacketOut(submit_to_egress)",
			desc: "Packet I/O-Traceroute-PacketOut:016 Egress: Programm match TTL=[1,2], inject packets with TTL>3, and verify packet fwding based fwding chain on the router side",
			fn:   testPacketOutEgress,
			// skip: true,
		},
		{
			name: "Program TTL Match Entry and Check PacketOut With TTL1 (submit_to_egress)",
			desc: "Packet I/O-Traceroute-PacketOut:017 Egress: Programm match TTL=[1,2], inject packets with TTL=[0,1,2], and verify packet sent back to controller",
			fn:   testPacketOutEgress,
		},
		{
			name: "Program TTL Match Entry and Check PacketOut With TTL more than 2 with Static Route (submit_to_egress)",
			desc: "Packet I/O-Traceroute-PacketOut:018 Egress: Programm match TTL=[1,2], inject packets with TTL>3 and configure null0 static router for the packet destination, verify packets sent out on those egress interfaces",
			fn:   testPacketOutEgressWithStaticroute,
		},
		{
			name: "Program TTL Match Entry and Check PacketOut With TTL1 With Static Route(submit_to_egress)",
			desc: "Packet I/O-Traceroute-PacketOut:019 Egress: Programm match TTL=[1,2], inject packets with TTL=[0,1,2], configure null0 static router for the packet destination, verify packets sent out on those egress interfaces",
			fn:   testPacketOutEgressTTLOneWithStaticroute,
		},
		{
			name: "Program TTL Match Entry and Check PacketOut With TTL1 With ICMP or Traceroute(submit_to_egress)",
			desc: "Packet I/O-Traceroute-PacketOut:020 Egress: Programm match TTL=[1,2], inject ICMP/Traceroute packets with TTL=[0,1,2],verify packets sent out on those egress interfaces",
			fn:   testPacketOutEgressTTLOneWithUDP,
		},
		{
			name: "Program TTL Match Entry and Check PacketOut With TTL1 With ICMP or Traceroute With Static Route(submit_to_egress)",
			desc: "Packet I/O-Traceroute-PacketOut:021 Egress: Programm match TTL=[1,2], inject ICMP/Traceroute packets with TTL=[0,1,2],configure null0 static router for the packet destination, verify packets sent out on those egress interfaces",
			fn:   testPacketOutEgressTTLOneWithUDPAndStaticRoute,
		},
		{
			name: "Flap Interface and Check PacketOut(submit_to_egress)",
			desc: "Packet I/O-Traceroute-PacketOut:023 Flap egress ports and verify the packets sent/dropped as port up/down",
			fn:   testPacketOutEgressWithInterfaceFlap,
			// skip: true,
		},
		{
			name: "Flap Interface and Check PacketOut(submit_to_ingress)",
			desc: "Packet I/O-Traceroute-PacketOut:023 Flap egress ports and verify the packets sent/dropped as port up/down",
			fn:   testPacketOutIngressWithInterfaceFlap,
		},
		{
			name: "Check PacketOut Scale(submit_to_egress)",
			desc: "Packet I/O-Traceroute-PacketOut:025 Verify scale rate of TTL=[1,2] packets",
			fn:   testPacketOutEgressScale,
		},
	}
)

type TTLPacketIO struct {
	PacketIOPacket
	NeedConfig   *bool
	IPv4         bool
	IPv6         bool
	TtlTwo       bool
	EgressPorts  []string
	IngressPort  string
	PacketOutObj *PacketIOPacket
}

func (ttl *TTLPacketIO) GetTableEntry(t *testing.T, delete bool) []*wbb.ACLWbbIngressTableEntryInfo {
	actionType := p4_v1.Update_INSERT
	if delete {
		actionType = p4_v1.Update_DELETE
	}
	entries := []*wbb.ACLWbbIngressTableEntryInfo{}
	if ttl.IPv4 {
		entries = append(entries, &wbb.ACLWbbIngressTableEntryInfo{
			Type:     actionType,
			IsIpv4:   uint8(1),
			TTL:      uint8(1),
			TTLMask:  uint8(255),
			Priority: 1,
		})
		if ttl.TtlTwo {
			entries = append(entries, &wbb.ACLWbbIngressTableEntryInfo{
				Type:     actionType,
				IsIpv4:   uint8(1),
				TTL:      uint8(2),
				TTLMask:  uint8(255),
				Priority: 1,
			})
		}
	}
	if ttl.IPv6 {
		entries = append(entries, &wbb.ACLWbbIngressTableEntryInfo{
			Type:     actionType,
			IsIpv6:   uint8(1),
			TTL:      uint8(1),
			TTLMask:  uint8(255),
			Priority: 1,
		})
		if ttl.TtlTwo {
			entries = append(entries, &wbb.ACLWbbIngressTableEntryInfo{
				Type:     actionType,
				IsIpv4:   uint8(1),
				TTL:      uint8(2),
				TTLMask:  uint8(255),
				Priority: 1,
			})
		}
	}

	return entries
}

func (ttl *TTLPacketIO) ApplyConfig(t *testing.T, dut *ondatra.DUTDevice, delete bool) {
	t.Logf("There is no configuration required")
}

func (ttl *TTLPacketIO) GetPacketOut(t *testing.T, portID uint32, submitIngress bool) []*p4_v1.PacketOut {
	packets := []*p4_v1.PacketOut{}
	if ttl.IPv4 {
		packet := &p4_v1.PacketOut{
			Payload: ttl.packetTTLRequestGet(t, submitIngress, true),
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
	}

	if ttl.IPv6 {
		packet := &p4_v1.PacketOut{
			Payload: ttl.packetTTLRequestGet(t, submitIngress, false),
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
	}

	return packets
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

func (ttl *TTLPacketIO) GetPacketOutObj(t *testing.T) *PacketIOPacket {
	return ttl.PacketOutObj
}

func (ttl *TTLPacketIO) packetTTLRequestGet(t *testing.T, submitIngress, ipv4 bool) []byte {
	t.Helper()
	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{
		FixLengths:       true,
		ComputeChecksums: true,
	}

	packetLayers := []gopacket.SerializableLayer{}

	if !submitIngress {
		pktEth := &layers.Ethernet{
			SrcMAC: net.HardwareAddr{0x00, 0xAA, 0x00, 0xAA, 0x00, 0xAA},
			DstMAC: net.HardwareAddr{0x00, 0x01, 0x00, 0x02, 0x00, 0x03},
		}
		if ipv4 {
			pktEth.EthernetType = 0x0800
		} else {
			pktEth.EthernetType = 0x86dd
		}
		packetLayers = append(packetLayers, pktEth)
	} else {
		pktEth := &layers.Ethernet{
			SrcMAC: net.HardwareAddr{0x00, 0x01, 0x00, 0x02, 0x00, 0x03},
			DstMAC: net.HardwareAddr{0x00, 0x1A, 0x11, 0x00, 0x00, 0x01},
		}
		if ipv4 {
			pktEth.EthernetType = 0x0800
		} else {
			pktEth.EthernetType = 0x86dd
		}
		packetLayers = append(packetLayers, pktEth)
	}

	// strings.Split(atePort1.IPv4, ".")

	if ipv4 {
		t.Logf("SOURCE IP %v ", net.IP(convertIPv4Address(t, *ttl.PacketOutObj.SrcIPv4)).To4().String())
		t.Logf("DEST IP %v ", net.IP(convertIPv4Address(t, *ttl.PacketOutObj.DstIPv4)).To4().String())
		// for PacketOut submit_to_ingress/submit_to_egress the flow is DUT to ATE
		pktIP := &layers.IPv4{
			Version:  4,
			SrcIP:    net.IP(convertIPv4Address(t, *ttl.PacketOutObj.DstIPv4)),
			DstIP:    net.IP(convertIPv4Address(t, *ttl.PacketOutObj.SrcIPv4)),
			TTL:      uint8(*ttl.PacketOutObj.TTL),
			Protocol: layers.IPProtocol(61),
		}
		packetLayers = append(packetLayers, pktIP)

	} else {

		pktIP := &layers.IPv6{
			Version:  6,
			SrcIP:    net.IP(convertIPv6Address(t, *ttl.PacketOutObj.DstIPv6)),
			DstIP:    net.IP(convertIPv6Address(t, *ttl.PacketOutObj.SrcIPv6)),
			HopLimit: uint8(*ttl.PacketOutObj.TTL),
		}
		packetLayers = append(packetLayers, pktIP)

	}
	// add UDP layer if udp is true
	if ttl.PacketOutObj.udp {
		udp := &layers.UDP{
			SrcPort: 11111,
			DstPort: 22222,
		}
		packetLayers = append(packetLayers, udp)
	}
	payload := []byte{}
	payLoadLen := 64
	for i := 0; i < payLoadLen; i++ {
		payload = append(payload, byte(i))
	}
	packetLayers = append(packetLayers, gopacket.Payload(payload))

	gopacket.SerializeLayers(buf, opts, packetLayers...)
	return buf.Bytes()
}

func (ttl *TTLPacketIO) GetPacketOutExpectation(t *testing.T, submit_to_ingress bool) bool {
	return true
}

func convertIPv4Address(t *testing.T, ip string) []byte {
	ss := strings.Split(ip, ".")
	bs := []byte{}
	for _, s := range ss {
		if num, err := strconv.ParseUint(s, 0, 8); err == nil {
			bs = append(bs, byte(num))
		} else {
			t.Logf("there is error when converting IPv4 address, %s", err)
		}
	}
	return bs
}

func convertIPv6Address(t *testing.T, ip string) []byte {
	ss := strings.Split(ip, ":")
	bs := []byte{}
	for _, s := range ss {
		if len(s) == 0 {
			// Handle :: case
			paddingLen := 8 - len(ss) + 1
			for i := 0; i < paddingLen; i++ {
				bs = append(bs, 0x00)
			}
		} else {
			if num, err := strconv.ParseUint(s, 16, 16); err == nil {
				bs = append(bs, byte(num))
			} else {
				t.Logf("there is error when converting IPv6 address, %s", err)
			}
		}
	}
	return bs
}
