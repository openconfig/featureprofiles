package cisco_p4rt_test

import (
	"fmt"
	"net"
	"testing"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	wbb "github.com/openconfig/featureprofiles/feature/experimental/p4rt/internal/p4rtutils"
	"github.com/openconfig/ondatra"
	p4_v1 "github.com/p4lang/p4runtime/go/p4/v1"
)

//-----------------------------//

var (
	gdpInLayers layers.EthernetType = 0x6007
)

var (
	OODGDPTestcases = []Testcase{
		{
			name: "Program GDP Match Entry and Check PacketIn",
			desc: "Packet I/O-GDP-PacketIn:001 After programm match EtherType 0x6007 entry, send EtherTye 0x6007 packets with google MAC from tgen and verify packet is sent to controller via PacketIn",
			fn:   testEntryProgrammingPacketIn,
		},
		{
			name: "Program GDP Match Entry and Check PacketIn With Traffic without GDP MAC",
			desc: "Packet I/O-GDP-PacketIn:002 After programm match EtherType 0x6007 entry, send EtherTye 0x6007 packets without google MAC from tgen and verify packet is NOT sent to controller via PacketIn",
			fn:   testEntryProgrammingPacketInWithUnicastMAC,
		},
		{
			name: "Send Traffic with gdp MAC without programming GDP entry ",
			desc: "Packet I/O-GDP-PacketIn:004 Without programm match EtherType 0x6007 entry, inject EtherTye 0x6007 packets with google MAC from tgen and verify packet is dropped",
			fn:   testPacketInWithoutEntryProgramming,
		},
		{
			name: "Send Traffic without gdp MAC without programming GDP entry ",
			desc: "Packet I/O-GDP-PacketIn:005 Without programm match EtherType 0x6007 entry, inject EtherTye 0x6007 packets without google MAC from tgen and verify packet is dropped",
			fn:   testPacketInWithoutEntryProgrammingWithNewMAC,
		},
		{
			name: "Program GDP Match Entry and remove GDP Match and check PacketIn",
			desc: "Packet I/O-GDP-PacketIn:006 Programm match EtherType 0x6007 entry, send traffic with 0x6007 packets with google MAC and remove the entry and verify traffic is not sent to controller any more",
			fn:   testEntryProgrammingPacketInThenRemoveEntry,
		},
		{
			name: "Program GDP Match Entry and check PackIn with invalid src MAC",
			desc: "Packet I/O-GDP-PacketIn:007 Programm match EtherType 0x6007 entry, send traffic with 0x6007 packets with google MAC + invalid src MAC and verify traffic is not sent to controller",
			fn:   testProgrammingPacketInWithInvalidSrcMAC,
		},
		{
			name: "Program GDP Match Entry and Use GDP MAC as interface MAC and check PacketIn",
			desc: "Packet I/O-GDP-PacketIn:008 Programm match EtherType 0x6007 entry, send traffic with 0x6007 packets with google MAC and configure google MAC on the interface and verify packets are still sent to controller",
			fn:   testProgrammingPacketInWithInterfaceMACAsGDPMac,
			skip: true,
		},
		{
			name: "Program GDP Match Entry and change port-id and check PacketIn",
			desc: "Packet I/O-GDP-PacketIn:009 Programm match EtherType 0x6007 entry, then change the related interface id and verify the field is changed accordingly in the PacketIn msg",
			fn:   testEntryProgrammingPacketInAndChangePortID,
		},
		{
			name: "Program GDP Match Entry and change device-id and check PacketIn",
			desc: "Packet I/O-GDP-PacketIn:010 Programm match EtherType 0x6007 entry, then change the related device-id and verify the field is changed accordingly in the PacketIn msg and packets are sent to the new controller",
			fn:   testEntryProgrammingPacketInAndChangeDeviceID,
		},
		{
			name: "Program GDP Match Entry and downgrade primary controller and check PacketIn",
			desc: "Packet I/O-GDP-PacketIn:011 Programm match EtherType 0x6007 entry, send traffic with 0x6007 packets with google MAC from tgen, downgrade/fail primary controller in case of there is standby controller, verify GDP packets sends to the new primary controller",
			fn:   testEntryProgrammingPacketInDowngradePrimaryController,
			skip: true,
		},
		{
			name: "Program GDP Match Entry and downgrade primary controller without backup controller and check PacketIn",
			desc: "Packet I/O-GDP-PacketIn:012 Programm match EtherType 0x6007 entry, send traffic with 0x6007 packets with google MAC from tgen, downgrade/fail primary controller in case of there is NO standby controller, verify GDP packets are not sent out and no impact on the device",
			fn:   testEntryProgrammingPacketInDowngradePrimaryControllerWithoutStandby,
		},
		{
			name: "Program GDP Match Entry and Recover previous primary controller and check PacketIn",
			desc: "Packet I/O-GDP-PacketIn:013 Programm match EtherType 0x6007 entry, send traffic with 0x6007 packets with google MAC from tgen, downgrade/fail primary controller then recover the controller, verify GDP packets sends to the same controller",
			fn:   testEntryProgrammingPacketInRecoverPrimaryController,
		},
		{
			name: "Program GDP Match Entry and other match fields and Check PacketIn",
			desc: "Packet I/O-GDP-PacketIn:014 Programm match EtherType 0x6007 with other match fields, send traffic with EtherType 0x6007 and verify device behavior",
			fn:   testEntryProgrammingPacketInWithMoreMatchingField,
		},
		{
			name: "Program GDP Match Entry and Send traffic to port not in P4RT and Check PacketIn",
			desc: "Packet I/O-GDP-PacketIn:018 Programm match EtherType 0x6007,  send traffic with EtherType 0x6007 on port which is not part of P4RT, verify the packets are not sent to the controller",
			fn:   testEntryProgrammingPacketInWithouthPortID,
		},
		{
			name: "Program GDP Match Entry and Send scale GDP traffic",
			desc: "Packet I/O-GDP-PacketIn:019 Verify scale rate of GDP packets(200kbps)",
			fn:   testEntryProgrammingPacketInScaleRate,
		},
		{
			name: "Program GDP Match Entry and Check PacketOut(submit_to_ingress)",
			desc: "Packet I/O-GDP-PacketOut:001 Ingress: Inject EtherType 0x6007 packets and verify traffic behavior in case of EtherType 0x6007 entry programmed",
			fn:   testPacketOut,
		},
		{
			name: "Check PacketOut Without Programming GDP Match Entry(submit_to_ingress)",
			desc: "Packet I/O-GDP-PacketOut:002 Ingress: Inject EtherType 0x6007 packets and verify traffic sends to related port in case of EtherType 0x6007 entry NOT programmed",
			fn:   testPacketOutWithoutMatchEntry,
		},
		{
			name: "Program GDP Match Entry and Check PacketOut(submit_to_egress)",
			desc: "Packet I/O-GDP-PacketOut:003 Egress: Inject EtherType 0x6007 packets and verify traffic behavior in case of EtherType 0x6007 entry programmed",
			fn:   testPacketOutEgress,
		},
		{
			name: "Check PacketOut Without Programming GDP Match Entry(submit_to_egress)",
			desc: "Packet I/O-GDP-PacketOut:004 Egress: Inject EtherType 0x6007 packets and verify traffic sends to related port in case of EtherType 0x6007 entry NOT programmed",
			fn:   testPacketOutEgressWithoutMatchEntry,
		},
		{
			name: "Check PacketOut With Invalid Port Id(submit_to_egress)",
			desc: "Packet I/O-GDP-PacketOut:005 Egress: Inject EtherType 0x6007 packets with invalid port-id and verify packet is dropped",
			fn:   testPacketOutEgressWithInvalidPortId,
		},
		{
			name: "Change Port-id and Check PacketOut(submit_to_egress)",
			desc: "Packet I/O-GDP-PacketOut:009 Egress: Inject EtherType 0x6007 packets on existing port-id and then change related port-id and verify device behavior",
			fn:   testPacketOutEgressWithChangePortId,
		},
		{
			name: "Change Metadata and Check PacketOut(submit_to_egress)",
			desc: "Packet I/O-GDP-PacketOut:010 Egress: Inject EtherType 0x6007 packets on existing port-id and then change related port-id and verify device behavior",
			fn:   testPacketOutEgressWithChangeMetadata,
		},
		{
			name: "Flap Interface and Check PacketOut(submit_to_ingress)",
			desc: "Packet I/O-GDP-PacketOut:011 Ingress: Verify bring down port in GDP PacketOut case and verify server behavior",
			fn:   testPacketOutIngressWithInterfaceFlap,
		},
		{
			name: "Flap Interface and Check PacketOut(submit_to_egress)",
			desc: "Packet I/O-GDP-PacketOut:011 Egress: Verify bring down port in GDP PacketOut case and verify server behavior",
			fn:   testPacketOutEgressWithInterfaceFlap,
			skip: true,
		},
		{
			name: "Check PacketOut Scale(submit_to_egress)",
			desc: "Packet I/O-GDP-PacketOut:013 Verify scale rate of GDP packets injecting to the device(200kbps)",
			fn:   testPacketOutEgressScale,
		},
	}
)

func packetGDPRequestGet(t *testing.T) []byte {
	t.Helper()
	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{
		FixLengths:       true,
		ComputeChecksums: true,
	}
	pktEth := &layers.Ethernet{
		SrcMAC: net.HardwareAddr{0x00, 0xAA, 0x00, 0xAA, 0x00, 0xAA},
		//00:0A:DA:F0:F0:F0
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

type GDPPacketIO struct {
	PacketIOPacket
	IngressPort string
}

func (gdp *GDPPacketIO) GetTableEntry(t *testing.T, delete bool) []*wbb.ACLWbbIngressTableEntryInfo {
	actionType := p4_v1.Update_INSERT
	if delete {
		actionType = p4_v1.Update_DELETE
	}
	return []*wbb.ACLWbbIngressTableEntryInfo{{
		Type:          actionType,
		EtherType:     0x6007,
		EtherTypeMask: 0xFFFF,
		Priority:      1,
	}}
}

func (gdp *GDPPacketIO) ApplyConfig(t *testing.T, dut *ondatra.DUTDevice, delete bool) {
	t.Logf("There is no configuration required")
}

func (gdp *GDPPacketIO) GetPacketOut(t *testing.T, portID uint32, submitIngress bool) []*p4_v1.PacketOut {
	packets := []*p4_v1.PacketOut{}
	packet := &p4_v1.PacketOut{
		Payload: packetGDPRequestGet(t),
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

func (gdp *GDPPacketIO) GetPacketTemplate(t *testing.T) *PacketIOPacket {
	return &gdp.PacketIOPacket
}

func (gdp *GDPPacketIO) GetTrafficFlow(t *testing.T, ate *ondatra.ATEDevice, frameSize uint32, frameRate uint64) []*ondatra.Flow {
	ethHeader := ondatra.NewEthernetHeader()
	ethHeader.WithSrcAddress(*gdp.SrcMAC)
	ethHeader.WithDstAddress(*gdp.DstMAC)
	ethHeader.WithEtherType(*gdp.EthernetType)

	flow := ate.Traffic().NewFlow("GDP").WithFrameSize(frameSize).WithFrameRateFPS(frameRate).WithHeaders(ethHeader)
	return []*ondatra.Flow{flow}
}

func (gdp *GDPPacketIO) GetEgressPort(t *testing.T) []string {
	return []string{"0"}
}

func (gdp *GDPPacketIO) SetEgressPorts(t *testing.T, portIDs []string) {

}

func (gdp *GDPPacketIO) GetIngressPort(t *testing.T) string {
	return gdp.IngressPort
}

func (gdp *GDPPacketIO) SetIngressPorts(t *testing.T, portID string) {
	gdp.IngressPort = portID
}

func (gdp *GDPPacketIO) GetPacketIOPacket(t *testing.T) *PacketIOPacket {
	return &gdp.PacketIOPacket
}

func (gdp *GDPPacketIO) GetPacketOutObj(t *testing.T) *PacketIOPacket {
	return &gdp.PacketIOPacket
}

func (gdp *GDPPacketIO) GetPacketOutExpectation(t *testing.T, submit_to_ingress bool) bool {
	return !submit_to_ingress
}
