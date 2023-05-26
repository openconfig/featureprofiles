package cisco_p4rt_test

import (
	"fmt"
	"net"
	"testing"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	wbb "github.com/openconfig/featureprofiles/feature/experimental/p4rt/internal/p4rtutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	p4_v1 "github.com/p4lang/p4runtime/go/p4/v1"
)

// debug lpts platform pifib all

var (
	lldpInLayers layers.EthernetType = 0x88cc
)

var (
	OODLLDPDisabledTestcases = []Testcase{
		{
			name: "(LLDP_Disable)Program LLDP Match Entry and Check PacketIn",
			desc: "Packet I/O-LLDP-PacketIn:001 LLDP disabled:After programm match EtherType 0x88cc entry, send EtherTye 0x88cc packets from tgen and verify packet is sent to controller via PacketIn",
			fn:   testEntryProgrammingPacketIn,
		},
		{
			name: "(LLDP_Disable)Program LLDP Match Entry and Check PacketIn With Unicast MAC",
			desc: "Packet I/O-LLDP-PacketIn:002 LLDP disabled:After programm match EtherType 0x88cc entry, send EtherTye 0x88cc packets with unicast MAC from tgen and verify packet is NOT sent to controller via PacketIn",
			fn:   testEntryProgrammingPacketInWithUnicastMAC,
		},
		{
			name: "(LLDP_Disable)Program LLDP Match Entry and Check PacketIn With Broadcast MAC",
			desc: "Packet I/O-LLDP-PacketIn:003 LLDP disabled:After programm match EtherType 0x88cc entry, send EtherTye 0x88cc packets with broadcast MAC from tgen and verify packet is NOT sent to controller via PacketIn",
			fn:   testEntryProgrammingPacketInWithBroadcastMAC,
		},
		{
			name: "(LLDP_Disable)Check PacketIn Without Program LLDP Match Entry",
			desc: "Packet I/O-LLDP-PacketIn:004 LLDP disabled:Without programm match EtherType 0x88cc entry, inject EtherTye 0x88cc packets from tgen and verify packet is dropped",
			fn:   testPacketInWithoutEntryProgramming,
		},
		{
			name: "(LLDP_Disable)Check PacketIn With Program LLDP Match Entry Then Remove LLDP Match Entry",
			desc: "Packet I/O-LLDP-PacketIn:005 LLDP disabled:Programm match EtherType 0x88cc entry, send traffic with 0x88cc packets and remove the entry and verify traffic is not sent to controller any more",
			fn:   testEntryProgrammingPacketInThenRemoveEntry,
		},
		{
			name: "(LLDP_Disable)Check PacketIn With Program LLDP Match Entry Then Change Port-Id",
			desc: "Packet I/O-LLDP-PacketIn:006 LLDP disabled:Programm match EtherType 0x88cc entry, then change the related interface id and verify the field is changed accordingly in the PacketIn msg",
			fn:   testEntryProgrammingPacketInAndChangePortID,
		},
		{
			name: "(LLDP_Disable)Check PacketIn With Program LLDP Match Entry Then Change Device-Id",
			desc: "Packet I/O-LLDP-PacketIn:007 LLDP disabled:Programm match EtherType 0x88cc entry, then change the related device-id and verify the field is changed accordingly in the PacketIn msg and packets are sent to the new controller",
			fn:   testEntryProgrammingPacketInAndChangeDeviceID,
		},
		{
			name: "(LLDP_Disable)Program LLDP Match Entry and Downgrade primary controller and Check PacketIn",
			desc: "Packet I/O-LLDP-PacketIn:008 LLDP disabled:Programm match EtherType 0x88cc entry, send traffic with 0x88cc packets from tgen, downgrade/fail primary controller in case of there is standby controller, verify LLDP packets sends to the new primary controller",
			fn:   testEntryProgrammingPacketInDowngradePrimaryController,
		},
		{
			name: "(LLDP_Disable)Program LLDP Match Entry and Downgrade primary controller without backup controller and Check PacketIn",
			desc: "Packet I/O-LLDP-PacketIn:009 LLDP disabled:Programm match EtherType 0x88cc entry, send traffic with 0x88cc packets from tgen, downgrade/fail primary controller in case of there is NO standby controller, verify LLDP packets are not sent out and no impact on the device",
			fn:   testEntryProgrammingPacketInDowngradePrimaryControllerWithoutStandby,
		},
		{
			name: "(LLDP_Disable)Program LLDP Match Entry and Recover previous primary controller and Check PacketIn",
			desc: "Packet I/O-GDP-PacketIn:010 LLDP disabled:Programm match EtherType 0x88cc entry, send traffic with 0x88cc packets from tgen, downgrade/fail primary controller then recover the controller, verify LLDP packets sends to the same controller",
			fn:   testEntryProgrammingPacketInRecoverPrimaryController,
		},
		{
			name: "(LLDP_Disable)Program LLDP Match Entry and other match fields and Check PacketIn",
			desc: "Packet I/O-GDP-PacketIn:011 LLDP disabled:Programm match EtherType 0x88cc with other match fields, send traffic with EtherType 0x88cc and verify device behavior",
			fn:   testEntryProgrammingPacketInWithMoreMatchingField,
		},
		{
			name: "(LLDP_Disable)Program LLDP Match Entry and Send traffic to port not in P4RT and Check PacketIn",
			desc: "Packet I/O-LLDP-PacketIn:013 LLDP disabled:Programm match EtherType 0x88cc, send traffic with EtherType 0x88cc on port which is not part of P4RT, verify the packets are not sent to the controller, packets should be dropped on the ingress port",
			fn:   testEntryProgrammingPacketInWithouthPortID,
		},
		{
			name: "(LLDP_Disable)Program LLDP Match Entry and Send scale LLDP traffic",
			desc: "Packet I/O-LLDP-PacketIn:014 LLDP disabled:Programm match EtherType 0x88cc, send traffic with EtherType 0x88cc on port which is not part of P4RT, verify the packets are not sent to the controller, packets should be dropped on the ingress port",
			fn:   testEntryProgrammingPacketInScaleRate,
		},
		{
			name: "(LLDP_Disable)Program LLDP Match Entry and Check PacketOut",
			desc: "Packet I/O-LLDP-PacketOut:001 LLDP disabled: Ingress: Inject EtherType 0x88cc packets and verify traffic behavior in case of EtherType 0x88cc entry programmed",
			fn:   testPacketOut,
			// skip: true,
		},
		{
			name: "(LLDP_Disable)Check PacketOut Without Programming LLDP Match Entry",
			desc: "Packet I/O-LLDP-PacketOut:002 LLDP disabled: Ingress:  Inject EtherType 0x88cc packets and verify traffic behavior in case of EtherType 0x88cc entry NOT programmed",
			fn:   testPacketOutWithoutMatchEntry,
		},
		{
			name: "(LLDP_Disable)Program LLDP Match Entry and Check PacketOut(submit_to_egress)",
			desc: "Packet I/O-LLDP-PacketOut:003 LLDP disabled: Egress: Inject EtherType 0x88cc packets and verify traffic behavior port in case of EtherType 0x88cc entry programmed",
			fn:   testPacketOutEgress,
			// skip: true,
		},
		{
			name: "(LLDP_Disable)Check PacketOut Without Programming LLDP Match Entry(submit_to_egress)",
			desc: "Packet I/O-LLDP-PacketOut:004 LLDP disabled: Egress: Inject EtherType 0x88cc packets and verify traffic behavior in case of EtherType 0x88cc entry NOT programmed",
			fn:   testPacketOutEgressWithoutMatchEntry,
		},
		{
			name: "(LLDP_Disable)Check PacketOut Scale(submit_to_egress)",
			desc: "Packet I/O-LLDP-PacketOut:005 LLDP disabled: Verify scale rate of LLDP packets injecting to the device",
			fn:   testPacketOutEgressScale,
		},
		{
			name: "(LLDP_Disable)Flap Interface and Check PacketOut(submit_to_egress)",
			desc: "Packet I/O-LLDP-PacketOut:011 LLDP disabled: Egress: Verify behavior when port flap",
			fn:   testPacketOutEgressWithInterfaceFlap,
			// skip: true,
		},
	}
	LLDPEndabledTestcases = []Testcase{
		{
			name: "(LLDP Enable)Program LLDP Match Entry and Check PacketIn",
			desc: "Packet I/O-LLDP-PacketIn:015 LLDP enabled:After programm match EtherType 0x88cc entry, send EtherTye 0x88cc packets from tgen and verify packet is sent to controller via PacketIn",
			fn:   testEntryProgrammingPacketIn,
		},
		{
			name: "(LLDP Enable)Check PacketIn Without Program LLDP Match Entry",
			desc: "Packet I/O-LLDP-PacketIn:016 LLDP enabled:Programm match EtherType 0x88cc entry, send traffic with 0x88cc packets and remove the entry and verify traffic is not sent to controller any more",
			fn:   testPacketInWithoutEntryProgramming,
		},
		{
			name: "(LLDP Enable)Check PacketIn With Program LLDP Match Entry Then Remove LLDP Match Entry",
			desc: "Packet I/O-LLDP-PacketIn:017 LLDP enabled:Programm match EtherType 0x88cc entry, send traffic with 0x88cc packets and remove the entry and verify traffic is not sent to controller any more",
			fn:   testEntryProgrammingPacketInThenRemoveEntry,
		},
		{
			name: "(LLDP Enable)Check PacketIn With Program LLDP Match Entry Then Change Port-Id",
			desc: "Packet I/O-LLDP-PacketIn:018 LLDP enabled:Programm match EtherType 0x88cc entry, then change the related interface id and verify the field is changed accordingly in the PacketIn msg",
			fn:   testEntryProgrammingPacketInAndChangePortID,
		},
		{
			name: "(LLDP_Enable)Check PacketIn With Program LLDP Match Entry Then Change Device-Id",
			desc: "Packet I/O-LLDP-PacketIn:019 LLDP enabled:Programm match EtherType 0x88cc entry, then change the related device-id and verify the field is changed accordingly in the PacketIn msg and packets are sent to the new controller",
			fn:   testEntryProgrammingPacketInAndChangeDeviceID,
		},
		{
			name: "(LLDP_Enable)Program LLDP Match Entry and Downgrade primary controller and Check PacketIn",
			desc: "Packet I/O-LLDP-PacketIn:020 LLDP enabled:Programm match EtherType 0x88cc entry, send traffic with 0x88cc packets from tgen, downgrade/fail primary controller in case of there is standby controller, verify LLDP packets sends to the new primary controller",
			fn:   testEntryProgrammingPacketInDowngradePrimaryController,
			// skip: true,
		},
		{
			name: "(LLDP_Enable)Program LLDP Match Entry and Downgrade primary controller without backup controller and Check PacketIn",
			desc: "Packet I/O-LLDP-PacketIn:021 LLDP enabled:Programm match EtherType 0x88cc entry, send traffic with 0x88cc packets from tgen, downgrade/fail primary controller in case of there is NO standby controller, verify LLDP packets are not sent out and no impact on the device",
			fn:   testEntryProgrammingPacketInDowngradePrimaryControllerWithoutStandby,
			// skip: true,
		},
		{
			name: "(LLDP_Enable)Program LLDP Match Entry and Recover previous primary controller and Check PacketIn",
			desc: "Packet I/O-GDP-PacketIn:022 LLDP enabled:Programm match EtherType 0x88cc entry, send traffic with 0x88cc packets from tgen, downgrade/fail primary controller then recover the controller, verify LLDP packets sends to the same controller",
			fn:   testEntryProgrammingPacketInRecoverPrimaryController,
			// skip: true,
		},
		{
			name: "(LLDP_Enable)Program LLDP Match Entry and other match fields and Check PacketIn",
			desc: "Packet I/O-GDP-PacketIn:023 LLDP enabled:Programm match EtherType 0x88cc with other match fields, send traffic with EtherType 0x88cc and verify device behavior",
			fn:   testEntryProgrammingPacketInWithMoreMatchingField,
		},
		{
			name: "(LLDP_Enable)Program LLDP Match Entry and Send traffic to port not in P4RT and Check PacketIn",
			desc: "Packet I/O-LLDP-PacketIn:025 LLDP enabled:Programm match EtherType 0x88cc, send traffic with EtherType 0x88cc on port which is not part of P4RT, verify the packets are not sent to the controller, packets should be dropped on the ingress port",
			fn:   testEntryProgrammingPacketInWithouthPortID,
			// skip: true,
		},
		{
			name: "(LLDP_Enable)Program LLDP Match Entry and Send scale LLDP traffic",
			desc: "Packet I/O-LLDP-PacketIn:027 LLDP enabled:Verify scale rate of LLDP packets",
			fn:   testEntryProgrammingPacketInScaleRate,
		},
		{
			name: "(LLDP_Enable)Program LLDP Match Entry and Check PacketOut",
			desc: "Packet I/O-LLDP-PacketOut:006 LLDP enabled: Ingress: Inject EtherType 0x88cc packets and verify traffic behavior in case of EtherType 0x88cc entry programmed",
			fn:   testPacketOut,
		},
		{
			name: "(LLDP_Enable)Check PacketOut Without Programming LLDP Match Entry",
			desc: "Packet I/O-LLDP-PacketOut:007 LLDP enabled: Ingress:  Inject EtherType 0x88cc packets and verify traffic behavior in case of EtherType 0x88cc entry NOT programmed",
			fn:   testPacketOutWithoutMatchEntry,
		},
		{
			name: "(LLDP_Enable)Program LLDP Match Entry and Check PacketOut(submit_to_egress)",
			desc: "Packet I/O-LLDP-PacketOut:008 LLDP enabled: Egress: Inject EtherType 0x88cc packets and verify traffic behavior port in case of EtherType 0x88cc entry programmed",
			fn:   testPacketOutEgress,
		},
		{
			name: "(LLDP_Enable)Check PacketOut Without Programming LLDP Match Entry(submit_to_egress)",
			desc: "Packet I/O-LLDP-PacketOut:009 LLDP enabled: Egress: Inject EtherType 0x88cc packets and verify traffic behavior in case of EtherType 0x88cc entry NOT programmed",
			fn:   testPacketOutEgressWithoutMatchEntry,
			// skip: true,
		},
		{
			name: "(LLDP_Enable)Check PacketOut Scale(submit_to_egress)",
			desc: "Packet I/O-LLDP-PacketOut:010 LLDP enabled: Verify scale rate of LLDP packets injecting to the device",
			fn:   testPacketOutEgressScale,
		},
		{
			name: "(LLDP_Enable)Flap Interface and Check PacketOut(submit_to_egress)",
			desc: "Packet I/O-LLDP-PacketOut:011 LLDP enabled: Egress: Verify behavior when port flap",
			fn:   testPacketOutEgressWithInterfaceFlap,
			// skip: true,
		},
	}
)

func packetLLDPRequestGet(t *testing.T) []byte {
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

type LLDPPacketIO struct {
	PacketIOPacket
	NeedConfig  *bool
	IngressPort string
}

func (lldp *LLDPPacketIO) GetTableEntry(t *testing.T, delete bool) []*wbb.ACLWbbIngressTableEntryInfo {
	actionType := p4_v1.Update_INSERT
	if delete {
		actionType = p4_v1.Update_DELETE
	}
	return []*wbb.ACLWbbIngressTableEntryInfo{{
		Type:          actionType,
		EtherType:     0x88cc,
		EtherTypeMask: 0xFFFF,
		Priority:      1,
	}}
}

func (lldp *LLDPPacketIO) ApplyConfig(t *testing.T, dut *ondatra.DUTDevice, delete bool) {
	if *lldp.NeedConfig {
		config := gnmi.OC().Lldp().Enabled()
		gnmi.Replace(t, dut, config.Config(), !delete)
	}
}

func (lldp *LLDPPacketIO) GetPacketOut(t *testing.T, portID uint32, submitIngress bool) []*p4_v1.PacketOut {
	packets := []*p4_v1.PacketOut{}
	packet := &p4_v1.PacketOut{
		Payload: packetLLDPRequestGet(t),
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

func (lldp *LLDPPacketIO) GetPacketTemplate(t *testing.T) *PacketIOPacket {
	return &lldp.PacketIOPacket
}

func (lldp *LLDPPacketIO) GetTrafficFlow(t *testing.T, ate *ondatra.ATEDevice, frameSize uint32, frameRate uint64) []*ondatra.Flow {
	ethHeader := ondatra.NewEthernetHeader()
	ethHeader.WithSrcAddress(*lldp.SrcMAC)
	ethHeader.WithDstAddress(*lldp.DstMAC)
	ethHeader.WithEtherType(*lldp.EthernetType)

	flow := ate.Traffic().NewFlow("LLDP").WithFrameSize(frameSize).WithFrameRateFPS(frameRate).WithHeaders(ethHeader)
	return []*ondatra.Flow{flow}
}

func (lldp *LLDPPacketIO) GetEgressPort(t *testing.T) []string {
	return []string{"0"}
}

func (lldp *LLDPPacketIO) SetEgressPorts(t *testing.T, portIDs []string) {

}

func (lldp *LLDPPacketIO) GetIngressPort(t *testing.T) string {
	return lldp.IngressPort
}

func (lldp *LLDPPacketIO) SetIngressPorts(t *testing.T, portID string) {
	lldp.IngressPort = portID
}

func (lldp *LLDPPacketIO) GetPacketIOPacket(t *testing.T) *PacketIOPacket {
	return &lldp.PacketIOPacket
}

func (lldp *LLDPPacketIO) GetPacketOutObj(t *testing.T) *PacketIOPacket {
	return &lldp.PacketIOPacket
}

func (lldp *LLDPPacketIO) GetPacketOutExpectation(t *testing.T, submit_to_ingress bool) bool {
	return !submit_to_ingress
}
