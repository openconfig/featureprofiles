package cisco_p4rt_test

import (
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	p4rt_client "github.com/cisco-open/go-p4/p4rt_client"
	"github.com/google/gopacket/layers"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/ygot/ygot"
	p4_v1 "github.com/p4lang/p4runtime/go/p4/v1"
	"wwwin-github.cisco.com/rehaddad/go-wbb/p4info/wbb"
)

var (
	GDPTestcases = []Testcase{
		{
			name: "Program GDP Match Entry and Check PacketIn",
			desc: "Packet I/O-GDP-PacketIn:001 After programm match EtherType 0x6007 entry, send EtherTye 0x6007 packets with google MAC from tgen and verify packet is sent to controller via PacketIn",
			fn:   testGDPEntryProgrammingPacketIn,
		},
		{
			name: "Program GDP Match Entry and Send Traffic without gdp MAC",
			desc: "Packet I/O-GDP-PacketIn:002 After programm match EtherType 0x6007 entry, send EtherTye 0x6007 packets without google MAC from tgen and verify packet is NOT sent to controller via PacketIn",
			fn:   testGDPEntryProgrammingPacketInWithoutGoogleMAC,
		},
		{
			name: "Send Traffic with gdp MAC without programming GDP entry ",
			desc: "Packet I/O-GDP-PacketIn:004 Without programm match EtherType 0x6007 entry, inject EtherTye 0x6007 packets with google MAC from tgen and verify packet is dropped",
			fn:   testGDPWithoutEntryProgrammingPacketInWithGoogleMAC,
		},
		{
			name: "Send Traffic without gdp MAC without programming GDP entry ",
			desc: "Packet I/O-GDP-PacketIn:005 Without programm match EtherType 0x6007 entry, inject EtherTye 0x6007 packets without google MAC from tgen and verify packet is dropped",
			fn:   testGDPWithoutEntryProgrammingPacketInWithoutGoogleMAC,
		},
		{
			name: "Program GDP Match Entry and remove GDP Match and check PacketIn",
			desc: "Packet I/O-GDP-PacketIn:006 Programm match EtherType 0x6007 entry, send traffic with 0x6007 packets with google MAC and remove the entry and verify traffic is not sent to controller any more",
			fn:   testGDPEntryProgrammingPacketInThenRemoveEntry,
		},
		{
			name: "Program GDP Match Entry and check PackIn with invalid src MAC",
			desc: "Packet I/O-GDP-PacketIn:007 Programm match EtherType 0x6007 entry, send traffic with 0x6007 packets with google MAC + invalid src MAC and verify traffic is not sent to controller",
			fn:   testGDPProgrammingPacketInWithInvalidSrcMAC,
		},
		{
			name: "Program GDP Match Entry and Use GDP MAC as interface MAC and check PacketIn",
			desc: "Packet I/O-GDP-PacketIn:008 Programm match EtherType 0x6007 entry, send traffic with 0x6007 packets with google MAC and configure google MAC on the interface and verify packets are still sent to controller",
			fn:   testGDPProgrammingPacketInWithInterfaceMACAsGDPMac,
		},
		{
			name: "Program GDP Match Entry and change port-id and check PacketIn",
			desc: "Packet I/O-GDP-PacketIn:009 Programm match EtherType 0x6007 entry, then change the related interface id and verify the field is changed accordingly in the PacketIn msg",
			fn:   testGDPProgrammingPacketInAndChangePortID,
		},
		{
			name: "Program GDP Match Entry and downgrade primary controller and check PacketIn",
			desc: "Packet I/O-GDP-PacketIn:011 Programm match EtherType 0x6007 entry, send traffic with 0x6007 packets with google MAC from tgen, downgrade/fail primary controller in case of there is standby controller, verify GDP packets sends to the new primary controller",
			fn:   testGDPEntryProgrammingPacketInDowngradePrimaryController,
		},
		{
			name: "Program GDP Match Entry and Recover previous primary controller and check PacketIn",
			desc: "Packet I/O-GDP-PacketIn:013 Programm match EtherType 0x6007 entry, send traffic with 0x6007 packets with google MAC from tgen, downgrade/fail primary controller then recover the controller, verify GDP packets sends to the same controller",
			fn:   testGDPEntryProgrammingPacketInRecoverPrimaryController,
		},
		{
			name: "Program GDP Match Entry and Check PacketOut(submit_to_ingress)",
			desc: "Packet I/O-GDP-PacketOut:001 Ingress:  Inject EtherType 0x6007 packets and verify traffic behavior in case of EtherType 0x6007 entry programmed",
			fn:   testGDPEntryProgrammingPacketOutWithGDP,
		},
		{
			name: "Check PacketOut(submit_to_ingress)",
			desc: "Packet I/O-GDP-PacketOut:002 Ingress:  Inject EtherType 0x6007 packets and verify traffic sends to related port in case of EtherType 0x6007 entry NOT programmed",
			fn:   testGDPEntryProgrammingPacketOut,
		},
		{
			name: "Program GDP Match Entry and Check PacketOut(submit_to_egress)",
			desc: "Packet I/O-GDP-PacketOut:003 Egress: Inject EtherType 0x6007 packets and verify traffic behavior in case of EtherType 0x6007 entry programmed",
			fn:   testGDPEntryProgrammingPacketOutWithGDPEgress,
		},
		{
			name: "Check PacketOut(submit_to_egress)",
			desc: "Packet I/O-GDP-PacketOut:004 Egress: Inject EtherType 0x6007 packets and verify traffic sends to related port in case of EtherType 0x6007 entry NOT programmed",
			fn:   testGDPEntryProgrammingPacketOutEgress,
		},
		{
			name: "Check PacketOut(submit_to_egress) With Invalid port-id",
			desc: "Packet I/O-GDP-PacketOut:005 Egress: Inject EtherType 0x6007 packets with invalid port-id and verify packet is dropped",
			fn:   testGDPEntryProgrammingPacketOutWithInvalidPortId,
		},
		{
			name: "Change port-id and Check PacketOut(submit_to_egress)",
			desc: "Packet I/O-GDP-PacketOut:009 Egress: Inject EtherType 0x6007 packets on existing port-id and then change related port-id and verify device behavior",
			fn:   testGDPEntryProgrammingPacketOutWithInvalidPortId,
		},
	}
)

func readAllPackets(ctx context.Context, t *testing.T, client p4rt_client.P4RTClient) []*p4rt_client.P4RTPacketInfo {
	t.Helper()
	// Check PacketIn on P4Client
	packets := []*p4rt_client.P4RTPacketInfo{}
	for {
		_, packet, err := client.StreamChannelGetPacket(&streamName, 0)
		if err == io.EOF {
			t.Logf("EOF error is seen in PacketIn.")
			break
		} else if err == nil {
			packets = append(packets, packet)
		} else {
			t.Logf("There is error seen when receving packets. %v, %s", err, err)
			break
		}
	}
	return packets
}

func testTraffic(t *testing.T, ate *ondatra.ATEDevice, dstMac string, etherType uint32, srcEndPoint *ondatra.Interface, duration int, args *testArgs) {
	testTrafficWithSrcMac(t, ate, dstMac, "00:11:00:22:00:33", etherType, srcEndPoint, duration, args)
}

func testTrafficWithSrcMac(t *testing.T, ate *ondatra.ATEDevice, dstMac, srcMac string, etherType uint32, srcEndPoint *ondatra.Interface, duration int, args *testArgs) {
	ethHeader := ondatra.NewEthernetHeader()
	ethHeader.WithSrcAddress(srcMac)
	ethHeader.WithDstAddress(dstMac)
	ethHeader.WithEtherType(etherType)

	flow := ate.Traffic().NewFlow("GDP").
		WithSrcEndpoints(srcEndPoint).
		WithDstEndpoints(srcEndPoint)

	flow.WithFrameSize(300).WithFrameRateFPS(2).WithHeaders(ethHeader)

	ate.Traffic().Start(t, flow)
	time.Sleep(time.Duration(duration) * time.Second)

	ate.Traffic().Stop(t)
}

// programmGDPMatchEntry programms or deletes GDP entry
func programmGDPMatchEntry(ctx context.Context, t *testing.T, client *p4rt_client.P4RTClient, delete bool) error {
	actionType := p4_v1.Update_INSERT
	if delete {
		actionType = p4_v1.Update_DELETE
	}
	err := client.Write(&p4_v1.WriteRequest{
		DeviceId:   deviceID,
		ElectionId: &p4_v1.Uint128{High: uint64(0), Low: uint64(100)},
		Updates: wbb.AclWbbIngressTableEntryGet([]*wbb.AclWbbIngressTableEntryInfo{
			&wbb.AclWbbIngressTableEntryInfo{
				Type:          actionType,
				EtherType:     0x6007,
				EtherTypeMask: 0xFFFF,
			},
		}),
		Atomicity: p4_v1.WriteRequest_CONTINUE_ON_ERROR,
	})
	if err != nil {
		return err
	}
	return nil
}

func testGDPEntryProgrammingPacketIn(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA
	// Program the GDP entry
	if err := programmGDPMatchEntry(ctx, t, client, false); err != nil {
		t.Errorf("There is error when inserting the GDP entry")
	}
	defer programmGDPMatchEntry(ctx, t, client, true)

	// Send GDP Packet
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	testTraffic(t, args.ate, gdpMAC, gdpEtherType, srcEndPoint, 10, args)

	// Check PacketIn on P4Client
	packets := []*p4rt_client.P4RTPacketInfo{}
	for i := 0; i < 40; i++ {
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

	// t.Logf("Captured packets: %v", len(packets))
	if len(packets) == 0 {
		t.Errorf("There is no packets received.")
	}

	t.Logf("Start to decode packet.")
	for _, packet := range packets {
		// t.Logf("Packet: %v", packet)
		if packet != nil {
			dstMac, etherType := decodePacket(t, packet.Pkt.GetPayload())
			// t.Logf("Decoded Ether Type: %v; Decoded DST MAC: %v", etherType, dstMac)
			if dstMac != gdpMAC || etherType != layers.EthernetType(gdpEtherType) {
				t.Errorf("Packet is not matching GDP packet.")
			}
			// TODO: Check Port-id in MetaData
			metaData := packet.Pkt.GetMetadata()
			for _, data := range metaData {
				if data.GetMetadataId() == METADATA_INGRESS_PORT {
					if string(data.GetValue()) != fmt.Sprint(portID) {
						t.Errorf("Ingress Port Id is not matching expectation...")
					}
				}
			}
		}
	}
}

func testGDPEntryProgrammingPacketInWithoutGoogleMAC(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA
	// Program the GDP entry
	if err := programmGDPMatchEntry(ctx, t, client, false); err != nil {
		t.Errorf("There is error when inserting the GDP entry")
	}
	defer programmGDPMatchEntry(ctx, t, client, true)

	// Send GDP Packet
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	testTraffic(t, args.ate, "00:12:34:56:78:9A", gdpEtherType, srcEndPoint, 10, args)

	// Check PacketIn on P4Client
	packets := []*p4rt_client.P4RTPacketInfo{}
	for i := 0; i < 40; i++ {
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

	// t.Logf("Captured packets: %v", len(packets))
	if len(packets) > 0 {
		t.Errorf("There is unexpected packets received.")
	}
}

func testGDPWithoutEntryProgrammingPacketInWithGoogleMAC(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA

	// Send GDP Packet
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	testTraffic(t, args.ate, gdpMAC, gdpEtherType, srcEndPoint, 10, args)

	// Check PacketIn on P4Client
	packets := []*p4rt_client.P4RTPacketInfo{}
	for i := 0; i < 40; i++ {
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

	// t.Logf("Captured packets: %v", len(packets))
	if len(packets) > 0 {
		t.Errorf("There is unexpected packets received.")
	}
}

func testGDPWithoutEntryProgrammingPacketInWithoutGoogleMAC(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA

	// Send GDP Packet
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	testTraffic(t, args.ate, "00:12:34:56:78:9A", gdpEtherType, srcEndPoint, 10, args)

	// Check PacketIn on P4Client
	packets := []*p4rt_client.P4RTPacketInfo{}
	for i := 0; i < 40; i++ {
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

	// t.Logf("Captured packets: %v", len(packets))
	if len(packets) > 0 {
		t.Errorf("There is unexpected packets received.")
	}
}

func testGDPEntryProgrammingPacketInThenRemoveEntry(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA
	// Program the GDP entry
	if err := programmGDPMatchEntry(ctx, t, client, false); err != nil {
		t.Errorf("There is error when inserting the GDP entry")
	}

	// Send GDP Packet
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	testTraffic(t, args.ate, gdpMAC, gdpEtherType, srcEndPoint, 10, args)

	// Check PacketIn on P4Client
	packets := []*p4rt_client.P4RTPacketInfo{}
	for i := 0; i < 40; i++ {
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

	// t.Logf("Captured packets: %v", len(packets))
	if len(packets) == 0 {
		t.Errorf("There is no packets received.")
	}

	t.Logf("Start to decode packet.")
	for _, packet := range packets {
		// t.Logf("Packet: %v", packet)
		if packet != nil {
			dstMac, etherType := decodePacket(t, packet.Pkt.GetPayload())
			// t.Logf("Decoded Ether Type: %v; Decoded DST MAC: %v", etherType, dstMac)
			if dstMac != gdpMAC || etherType != layers.EthernetType(gdpEtherType) {
				t.Errorf("Packet is not matching GDP packet.")
			}
			// TODO: Check Port-id in MetaData
			metaData := packet.Pkt.GetMetadata()
			for _, data := range metaData {
				if data.GetMetadataId() == METADATA_INGRESS_PORT {
					if string(data.GetValue()) != fmt.Sprint(portID) {
						t.Errorf("Ingress Port Id is not matching expectation...")
					}
				}
			}
		}
	}

	programmGDPMatchEntry(ctx, t, client, true)

	// Check PacketIn on P4Client
	packets = []*p4rt_client.P4RTPacketInfo{}
	for i := 0; i < 40; i++ {
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

	// t.Logf("Captured packets: %v", len(packets))
	if len(packets) > 0 {
		t.Errorf("Unexpected packets received.")
	}
}

func testGDPProgrammingPacketInWithInvalidSrcMAC(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA

	// Send GDP Packet
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	testTrafficWithSrcMac(t, args.ate, gdpMAC, "FF:FF:FF:FF:FF:FF", gdpEtherType, srcEndPoint, 10, args)

	// Check PacketIn on P4Client
	packets := []*p4rt_client.P4RTPacketInfo{}
	for i := 0; i < 40; i++ {
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

	// t.Logf("Captured packets: %v", len(packets))
	if len(packets) > 0 {
		t.Errorf("There is unexpected packets received.")
	}
}

func testGDPProgrammingPacketInWithInterfaceMACAsGDPMac(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA
	// Program the GDP entry
	if err := programmGDPMatchEntry(ctx, t, client, false); err != nil {
		t.Errorf("There is error when inserting the GDP entry")
	}
	defer programmGDPMatchEntry(ctx, t, client, true)

	// Change mac address for the related bundle interface MAC
	newMAC := gdpMAC
	currentMAC := args.dut.Telemetry().Interface(args.interfaces.in[0]).Ethernet().MacAddress().Get(t)
	args.dut.Config().Interface(args.interfaces.in[0]).Ethernet().MacAddress().Replace(t, newMAC)
	defer args.dut.Config().Interface(args.interfaces.in[0]).Ethernet().MacAddress().Replace(t, currentMAC)

	// Send GDP Packet
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	testTraffic(t, args.ate, gdpMAC, gdpEtherType, srcEndPoint, 10, args)

	// Check PacketIn on P4Client
	packets := []*p4rt_client.P4RTPacketInfo{}
	for i := 0; i < 40; i++ {
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

	// t.Logf("Captured packets: %v", len(packets))
	if len(packets) == 0 {
		t.Errorf("There is no packets received.")
	}

	t.Logf("Start to decode packet.")
	for _, packet := range packets {
		// t.Logf("Packet: %v", packet)
		if packet != nil {
			dstMac, etherType := decodePacket(t, packet.Pkt.GetPayload())
			// t.Logf("Decoded Ether Type: %v; Decoded DST MAC: %v", etherType, dstMac)
			if dstMac != gdpMAC || etherType != layers.EthernetType(gdpEtherType) {
				t.Errorf("Packet is not matching GDP packet.")
			}
			// TODO: Check Port-id in MetaData
			metaData := packet.Pkt.GetMetadata()
			for _, data := range metaData {
				if data.GetMetadataId() == METADATA_INGRESS_PORT {
					if string(data.GetValue()) != fmt.Sprint(portID) {
						t.Errorf("Ingress Port Id is not matching expectation...")
					}
				}
			}
		}
	}
}

func testGDPProgrammingPacketInAndChangePortID(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA
	// Program the GDP entry
	if err := programmGDPMatchEntry(ctx, t, client, false); err != nil {
		t.Errorf("There is error when inserting the GDP entry")
	}
	defer programmGDPMatchEntry(ctx, t, client, true)

	// Send GDP Packet
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	testTraffic(t, args.ate, gdpMAC, gdpEtherType, srcEndPoint, 10, args)

	// Check PacketIn on P4Client
	packets := []*p4rt_client.P4RTPacketInfo{}
	for i := 0; i < 40; i++ {
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

	// t.Logf("Captured packets: %v", len(packets))
	if len(packets) == 0 {
		t.Errorf("There is no packets received.")
	}

	t.Logf("Start to decode packet.")
	for _, packet := range packets {
		// t.Logf("Packet: %v", packet)
		if packet != nil {
			dstMac, etherType := decodePacket(t, packet.Pkt.GetPayload())
			// t.Logf("Decoded Ether Type: %v; Decoded DST MAC: %v", etherType, dstMac)
			if dstMac != gdpMAC || etherType != layers.EthernetType(gdpEtherType) {
				t.Errorf("Packet is not matching GDP packet.")
			}
			// TODO: Check Port-id in MetaData
			metaData := packet.Pkt.GetMetadata()
			for _, data := range metaData {
				if data.GetMetadataId() == METADATA_INGRESS_PORT {
					if string(data.GetValue()) != fmt.Sprint(portID) {
						t.Errorf("Ingress Port Id is not matching expectation...")
					}
				}
			}
		}
	}

	newPortID := ^portID
	portName := sortPorts(args.dut.Ports())[0].Name()
	args.dut.Config().Interface(portName).Update(t, &telemetry.Interface{
		Name: ygot.String(portName),
		Id:   ygot.Uint32(newPortID),
	})

	defer args.dut.Config().Interface(portName).Update(t, &telemetry.Interface{
		Name: ygot.String(portName),
		Id:   ygot.Uint32(portID),
	})

	testTraffic(t, args.ate, gdpMAC, gdpEtherType, srcEndPoint, 10, args)

	// Check PacketIn on P4Client
	packets = []*p4rt_client.P4RTPacketInfo{}
	for i := 0; i < 40; i++ {
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

	// t.Logf("Captured packets: %v", len(packets))
	if len(packets) == 0 {
		t.Errorf("There is no packets received.")
	}

	t.Logf("Start to decode packet.")
	for _, packet := range packets {
		// t.Logf("Packet: %v", packet)
		if packet != nil {
			dstMac, etherType := decodePacket(t, packet.Pkt.GetPayload())
			// t.Logf("Decoded Ether Type: %v; Decoded DST MAC: %v", etherType, dstMac)
			if dstMac != gdpMAC || etherType != layers.EthernetType(gdpEtherType) {
				t.Errorf("Packet is not matching GDP packet.")
			}
			// TODO: Check Port-id in MetaData
			metaData := packet.Pkt.GetMetadata()
			for _, data := range metaData {
				if data.GetMetadataId() == METADATA_INGRESS_PORT {
					if string(data.GetValue()) != fmt.Sprint(newPortID) {
						t.Errorf("Ingress Port Id is not matching expectation...")
					}
				}
			}
		}
	}
}

func testGDPEntryProgrammingPacketInDowngradePrimaryController(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA
	// Program the GDP entry
	if err := programmGDPMatchEntry(ctx, t, client, false); err != nil {
		t.Errorf("There is error when inserting the GDP entry")
	}
	defer programmGDPMatchEntry(ctx, t, client, true)

	// Send GDP Packet
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	testTraffic(t, args.ate, gdpMAC, gdpEtherType, srcEndPoint, 10, args)

	// Check PacketIn on P4Client
	packets := []*p4rt_client.P4RTPacketInfo{}
	for i := 0; i < 40; i++ {
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

	// t.Logf("Captured packets: %v", len(packets))
	if len(packets) == 0 {
		t.Errorf("There is no packets received.")
	}

	t.Logf("Start to decode packet.")
	for _, packet := range packets {
		// t.Logf("Packet: %v", packet)
		if packet != nil {
			dstMac, etherType := decodePacket(t, packet.Pkt.GetPayload())
			// t.Logf("Decoded Ether Type: %v; Decoded DST MAC: %v", etherType, dstMac)
			if dstMac != gdpMAC || etherType != layers.EthernetType(gdpEtherType) {
				t.Errorf("Packet is not matching GDP packet.")
			}
			// TODO: Check Port-id in MetaData
			metaData := packet.Pkt.GetMetadata()
			for _, data := range metaData {
				if data.GetMetadataId() == METADATA_INGRESS_PORT {
					if string(data.GetValue()) != fmt.Sprint(portID) {
						t.Errorf("Ingress Port Id is not matching expectation...")
					}
				}
			}
		}
	}

	// Downgrade Primary Controller
	if err := client.StreamChannelSendMsg(&streamName, &p4_v1.StreamMessageRequest{
		Update: &p4_v1.StreamMessageRequest_Arbitration{
			Arbitration: &p4_v1.MasterArbitrationUpdate{
				DeviceId: deviceID,
				ElectionId: &p4_v1.Uint128{
					High: uint64(0),
					Low:  uint64(1),
				},
			},
		},
	}); err != nil {
		t.Errorf("There is error when sending arbitration message, %s", err)
	}
	if _, _, arbErr := client.StreamChannelGetArbitrationResp(&streamName, 1); arbErr != nil {
		t.Errorf("There is error when downgrading the client, %s", arbErr)
	}

	defer client.StreamChannelSendMsg(&streamName, &p4_v1.StreamMessageRequest{
		Update: &p4_v1.StreamMessageRequest_Arbitration{
			Arbitration: &p4_v1.MasterArbitrationUpdate{
				DeviceId: deviceID,
				ElectionId: &p4_v1.Uint128{
					High: uint64(0),
					Low:  electionID,
				},
			},
		},
	})

	newClient := args.p4rtClientB
	testTraffic(t, args.ate, gdpMAC, gdpEtherType, srcEndPoint, 10, args)
	// Check PacketIn on P4Client
	packets = []*p4rt_client.P4RTPacketInfo{}
	for i := 0; i < 40; i++ {
		_, packet, err := newClient.StreamChannelGetPacket(&streamName, 0)
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

	// t.Logf("Captured packets: %v", len(packets))
	if len(packets) == 0 {
		t.Errorf("There is no packets received.")
	}

	t.Logf("Start to decode packet.")
	for _, packet := range packets {
		// t.Logf("Packet: %v", packet)
		if packet != nil {
			dstMac, etherType := decodePacket(t, packet.Pkt.GetPayload())
			// t.Logf("Decoded Ether Type: %v; Decoded DST MAC: %v", etherType, dstMac)
			if dstMac != gdpMAC || etherType != layers.EthernetType(gdpEtherType) {
				t.Errorf("Packet is not matching GDP packet.")
			}
			// TODO: Check Port-id in MetaData
			metaData := packet.Pkt.GetMetadata()
			for _, data := range metaData {
				if data.GetMetadataId() == METADATA_INGRESS_PORT {
					if string(data.GetValue()) != fmt.Sprint(portID) {
						t.Errorf("Ingress Port Id is not matching expectation...")
					}
				}
			}
		}
	}
}

func testGDPEntryProgrammingPacketInRecoverPrimaryController(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA
	// Program the GDP entry
	if err := programmGDPMatchEntry(ctx, t, client, false); err != nil {
		t.Errorf("There is error when inserting the GDP entry")
	}
	defer programmGDPMatchEntry(ctx, t, client, true)

	// Send GDP Packet
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	testTraffic(t, args.ate, gdpMAC, gdpEtherType, srcEndPoint, 10, args)

	// Check PacketIn on P4Client
	packets := []*p4rt_client.P4RTPacketInfo{}
	for i := 0; i < 40; i++ {
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

	// t.Logf("Captured packets: %v", len(packets))
	if len(packets) == 0 {
		t.Errorf("There is no packets received.")
	}

	t.Logf("Start to decode packet.")
	for _, packet := range packets {
		// t.Logf("Packet: %v", packet)
		if packet != nil {
			dstMac, etherType := decodePacket(t, packet.Pkt.GetPayload())
			// t.Logf("Decoded Ether Type: %v; Decoded DST MAC: %v", etherType, dstMac)
			if dstMac != gdpMAC || etherType != layers.EthernetType(gdpEtherType) {
				t.Errorf("Packet is not matching GDP packet.")
			}
			// TODO: Check Port-id in MetaData
			metaData := packet.Pkt.GetMetadata()
			for _, data := range metaData {
				if data.GetMetadataId() == METADATA_INGRESS_PORT {
					if string(data.GetValue()) != fmt.Sprint(portID) {
						t.Errorf("Ingress Port Id is not matching expectation...")
					}
				}
			}
		}
	}

	// Downgrade Primary Controller
	if err := client.StreamChannelSendMsg(&streamName, &p4_v1.StreamMessageRequest{
		Update: &p4_v1.StreamMessageRequest_Arbitration{
			Arbitration: &p4_v1.MasterArbitrationUpdate{
				DeviceId: deviceID,
				ElectionId: &p4_v1.Uint128{
					High: uint64(0),
					Low:  uint64(1),
				},
			},
		},
	}); err != nil {
		t.Errorf("There is error when sending arbitration message, %s", err)
	}
	if _, _, arbErr := client.StreamChannelGetArbitrationResp(&streamName, 1); arbErr != nil {
		t.Errorf("There is error when downgrading the client, %s", arbErr)
	}

	testTraffic(t, args.ate, gdpMAC, gdpEtherType, srcEndPoint, 10, args)

	// Recover Primary Controller
	client.StreamChannelSendMsg(&streamName, &p4_v1.StreamMessageRequest{
		Update: &p4_v1.StreamMessageRequest_Arbitration{
			Arbitration: &p4_v1.MasterArbitrationUpdate{
				DeviceId: deviceID,
				ElectionId: &p4_v1.Uint128{
					High: uint64(0),
					Low:  electionID,
				},
			},
		},
	})
	if _, _, arbErr := client.StreamChannelGetArbitrationResp(&streamName, 1); arbErr != nil {
		t.Errorf("There is error when downgrading the client, %s", arbErr)
	}

	testTraffic(t, args.ate, gdpMAC, gdpEtherType, srcEndPoint, 10, args)
	// Check PacketIn on P4Client
	packets = []*p4rt_client.P4RTPacketInfo{}
	for i := 0; i < 40; i++ {
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

	// t.Logf("Captured packets: %v", len(packets))
	if len(packets) == 0 {
		t.Errorf("There is no packets received.")
	}

	t.Logf("Start to decode packet.")
	for _, packet := range packets {
		// t.Logf("Packet: %v", packet)
		if packet != nil {
			dstMac, etherType := decodePacket(t, packet.Pkt.GetPayload())
			// t.Logf("Decoded Ether Type: %v; Decoded DST MAC: %v", etherType, dstMac)
			if dstMac != gdpMAC || etherType != layers.EthernetType(gdpEtherType) {
				t.Errorf("Packet is not matching GDP packet.")
			}
			// TODO: Check Port-id in MetaData
			metaData := packet.Pkt.GetMetadata()
			for _, data := range metaData {
				if data.GetMetadataId() == METADATA_INGRESS_PORT {
					if string(data.GetValue()) != fmt.Sprint(portID) {
						t.Errorf("Ingress Port Id is not matching expectation...")
					}
				}
			}
		}
	}

}

func testGDPEntryProgrammingPacketOutWithGDP(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA

	if err := programmGDPMatchEntry(ctx, t, client, false); err != nil {
		t.Errorf("There is error when inserting the GDP entry")
	}
	defer programmGDPMatchEntry(ctx, t, client, true)
	// srcEndPoint := args.top.Interfaces()[atePort1.Name]

	// Check initial packet counters
	// port := sortPorts(args.ate.Ports())[0].Name()
	// counter_0 := args.ate.Telemetry().Interface(port).Counters().InPkts().Get(t)
	port := sortPorts(args.dut.Ports())[0].Name()
	counter_0 := args.dut.Telemetry().Interface(port).Counters().OutPkts().Get(t)

	packet := p4_v1.PacketOut{
		Payload: packetGDPRequestGet(t),
		Metadata: []*p4_v1.PacketMetadata{
			&p4_v1.PacketMetadata{
				MetadataId: SUBMIT_TO_INGRESS, // "submit_to_egress"
				Value:      []byte(fmt.Sprint(portID)),
			},
		},
	}
	packet_count := 100
	for i := 0; i < packet_count; i++ {
		if err := client.StreamChannelSendMsg(
			&streamName, &p4_v1.StreamMessageRequest{
				Update: &p4_v1.StreamMessageRequest_Packet{
					Packet: &packet,
				},
			}); err != nil {
			t.Errorf("There is error seen in Packet Out. %v, %s", err, err)
		}
	}

	// Wait for ate stats to be populated
	time.Sleep(60 * time.Second)
	// testTraffic(t, args.ate, gdpMAC, gdpEtherType, srcEndPoint, 10, args)

	// Check packet counters after packet out
	// counter_1 := args.ate.Telemetry().Interface(port).Counters().InPkts().Get(t)
	counter_1 := args.dut.Telemetry().Interface(port).Counters().OutPkts().Get(t)

	// Verify InPkts stats to check P4RT stream
	// fmt.Println(counter_0)
	// fmt.Println(counter_1)

	t.Logf("Sends out %v packets on interface %s", counter_1-counter_0, port)
	if counter_1-counter_0 > uint64(float64(packet_count)*0.10) {
		t.Errorf("Unexpected packets are received.")
	}
}

func testGDPEntryProgrammingPacketOut(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA

	// srcEndPoint := args.top.Interfaces()[atePort1.Name]

	// Check initial packet counters
	// port := sortPorts(args.ate.Ports())[0].Name()
	// counter_0 := args.ate.Telemetry().Interface(port).Counters().InPkts().Get(t)
	port := sortPorts(args.dut.Ports())[0].Name()
	counter_0 := args.dut.Telemetry().Interface(port).Counters().OutPkts().Get(t)

	packet := p4_v1.PacketOut{
		Payload: packetGDPRequestGet(t),
		Metadata: []*p4_v1.PacketMetadata{
			&p4_v1.PacketMetadata{
				MetadataId: SUBMIT_TO_INGRESS, // "submit_to_egress"
				Value:      []byte(fmt.Sprint(portID)),
			},
		},
	}
	packet_count := 100
	for i := 0; i < packet_count; i++ {
		if err := client.StreamChannelSendMsg(
			&streamName, &p4_v1.StreamMessageRequest{
				Update: &p4_v1.StreamMessageRequest_Packet{
					Packet: &packet,
				},
			}); err != nil {
			t.Errorf("There is error seen in Packet Out. %v, %s", err, err)
		}
	}

	// Wait for ate stats to be populated
	time.Sleep(60 * time.Second)
	// testTraffic(t, args.ate, gdpMAC, gdpEtherType, srcEndPoint, 10, args)

	// Check packet counters after packet out
	// counter_1 := args.ate.Telemetry().Interface(port).Counters().InPkts().Get(t)
	counter_1 := args.dut.Telemetry().Interface(port).Counters().OutPkts().Get(t)

	// Verify InPkts stats to check P4RT stream
	// fmt.Println(counter_0)
	// fmt.Println(counter_1)

	t.Logf("Sends out %v packets on interface %s", counter_1-counter_0, port)
	if counter_1-counter_0 > uint64(float64(packet_count)*0.10) {
		t.Errorf("Unexpected packets are received.")
	}
}

func testGDPEntryProgrammingPacketOutWithGDPEgress(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA

	if err := programmGDPMatchEntry(ctx, t, client, false); err != nil {
		t.Errorf("There is error when inserting the GDP entry")
	}
	defer programmGDPMatchEntry(ctx, t, client, true)
	// srcEndPoint := args.top.Interfaces()[atePort1.Name]

	// Check initial packet counters
	// port := sortPorts(args.ate.Ports())[0].Name()
	// counter_0 := args.ate.Telemetry().Interface(port).Counters().InPkts().Get(t)
	port := sortPorts(args.dut.Ports())[0].Name()
	counter_0 := args.dut.Telemetry().Interface(port).Counters().OutPkts().Get(t)

	packet := p4_v1.PacketOut{
		Payload: packetGDPRequestGet(t),
		Metadata: []*p4_v1.PacketMetadata{
			&p4_v1.PacketMetadata{
				MetadataId: SUBMIT_TO_EGRESS, // "submit_to_egress"
				Value:      []byte(fmt.Sprint(portID)),
			},
		},
	}
	packet_count := 100
	for i := 0; i < packet_count; i++ {
		if err := client.StreamChannelSendMsg(
			&streamName, &p4_v1.StreamMessageRequest{
				Update: &p4_v1.StreamMessageRequest_Packet{
					Packet: &packet,
				},
			}); err != nil {
			t.Errorf("There is error seen in Packet Out. %v, %s", err, err)
		}
	}

	// Wait for ate stats to be populated
	time.Sleep(60 * time.Second)
	// testTraffic(t, args.ate, gdpMAC, gdpEtherType, srcEndPoint, 10, args)

	// Check packet counters after packet out
	// counter_1 := args.ate.Telemetry().Interface(port).Counters().InPkts().Get(t)
	counter_1 := args.dut.Telemetry().Interface(port).Counters().OutPkts().Get(t)

	// Verify InPkts stats to check P4RT stream
	// fmt.Println(counter_0)
	// fmt.Println(counter_1)

	t.Logf("Sends out %v packets on interface %s", counter_1-counter_0, port)
	if counter_1-counter_0 < uint64(float64(packet_count)*0.90) {
		t.Errorf("There are no enough packets received.")
	}
}

func testGDPEntryProgrammingPacketOutEgress(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA

	// srcEndPoint := args.top.Interfaces()[atePort1.Name]

	// Check initial packet counters
	// port := sortPorts(args.ate.Ports())[0].Name()
	// counter_0 := args.ate.Telemetry().Interface(port).Counters().InPkts().Get(t)
	port := sortPorts(args.dut.Ports())[0].Name()
	counter_0 := args.dut.Telemetry().Interface(port).Counters().OutPkts().Get(t)

	packet := p4_v1.PacketOut{
		Payload: packetGDPRequestGet(t),
		Metadata: []*p4_v1.PacketMetadata{
			&p4_v1.PacketMetadata{
				MetadataId: SUBMIT_TO_EGRESS, // "submit_to_egress"
				Value:      []byte(fmt.Sprint(portID)),
			},
		},
	}
	packet_count := 100
	for i := 0; i < packet_count; i++ {
		if err := client.StreamChannelSendMsg(
			&streamName, &p4_v1.StreamMessageRequest{
				Update: &p4_v1.StreamMessageRequest_Packet{
					Packet: &packet,
				},
			}); err != nil {
			t.Errorf("There is error seen in Packet Out. %v, %s", err, err)
		}
	}

	// Wait for ate stats to be populated
	time.Sleep(60 * time.Second)
	// testTraffic(t, args.ate, gdpMAC, gdpEtherType, srcEndPoint, 10, args)

	// Check packet counters after packet out
	// counter_1 := args.ate.Telemetry().Interface(port).Counters().InPkts().Get(t)
	counter_1 := args.dut.Telemetry().Interface(port).Counters().OutPkts().Get(t)

	// Verify InPkts stats to check P4RT stream
	// fmt.Println(counter_0)
	// fmt.Println(counter_1)

	t.Logf("Sends out %v packets on interface %s", counter_1-counter_0, port)
	if counter_1-counter_0 < uint64(float64(packet_count)*0.90) {
		t.Errorf("There are no enough packets received.")
	}
}

func testGDPEntryProgrammingPacketOutWithInvalidPortId(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA
	// srcEndPoint := args.top.Interfaces()[atePort1.Name]

	// Check initial packet counters
	// port := sortPorts(args.ate.Ports())[0].Name()
	// counter_0 := args.ate.Telemetry().Interface(port).Counters().InPkts().Get(t)
	port := sortPorts(args.dut.Ports())[0].Name()
	counter_0 := args.dut.Telemetry().Interface(port).Counters().OutPkts().Get(t)

	packet := p4_v1.PacketOut{
		Payload: packetGDPRequestGet(t),
		Metadata: []*p4_v1.PacketMetadata{
			&p4_v1.PacketMetadata{
				MetadataId: SUBMIT_TO_EGRESS, // "submit_to_egress"
				Value:      []byte(fmt.Sprint(^portID)),
			},
		},
	}
	packet_count := 100
	for i := 0; i < packet_count; i++ {
		if err := client.StreamChannelSendMsg(
			&streamName, &p4_v1.StreamMessageRequest{
				Update: &p4_v1.StreamMessageRequest_Packet{
					Packet: &packet,
				},
			}); err != nil {
			t.Errorf("There is error seen in Packet Out. %v, %s", err, err)
		}
	}

	// Wait for ate stats to be populated
	time.Sleep(60 * time.Second)
	// testTraffic(t, args.ate, gdpMAC, gdpEtherType, srcEndPoint, 10, args)

	// Check packet counters after packet out
	// counter_1 := args.ate.Telemetry().Interface(port).Counters().InPkts().Get(t)
	counter_1 := args.dut.Telemetry().Interface(port).Counters().OutPkts().Get(t)

	// Verify InPkts stats to check P4RT stream
	// fmt.Println(counter_0)
	// fmt.Println(counter_1)

	t.Logf("Sends out %v packets on interface %s", counter_1-counter_0, port)
	if counter_1-counter_0 > uint64(float64(packet_count)*0.10) {
		t.Errorf("Unexpected packets are received.")
	}
}

func testGDPEntryProgrammingPacketOutAndChangePortId(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA
	// srcEndPoint := args.top.Interfaces()[atePort1.Name]

	newPortID := ^portID
	portName := sortPorts(args.dut.Ports())[0].Name()
	args.dut.Config().Interface(portName).Update(t, &telemetry.Interface{
		Name: ygot.String(portName),
		Id:   ygot.Uint32(newPortID),
	})

	defer args.dut.Config().Interface(portName).Update(t, &telemetry.Interface{
		Name: ygot.String(portName),
		Id:   ygot.Uint32(portID),
	})

	// Check initial packet counters
	// port := sortPorts(args.ate.Ports())[0].Name()
	// counter_0 := args.ate.Telemetry().Interface(port).Counters().InPkts().Get(t)
	port := sortPorts(args.dut.Ports())[0].Name()
	counter_0 := args.dut.Telemetry().Interface(port).Counters().OutPkts().Get(t)

	packet := p4_v1.PacketOut{
		Payload: packetGDPRequestGet(t),
		Metadata: []*p4_v1.PacketMetadata{
			&p4_v1.PacketMetadata{
				MetadataId: SUBMIT_TO_EGRESS, // "submit_to_egress"
				Value:      []byte(fmt.Sprint(newPortID)),
			},
		},
	}
	packet_count := 100
	for i := 0; i < packet_count; i++ {
		if err := client.StreamChannelSendMsg(
			&streamName, &p4_v1.StreamMessageRequest{
				Update: &p4_v1.StreamMessageRequest_Packet{
					Packet: &packet,
				},
			}); err != nil {
			t.Errorf("There is error seen in Packet Out. %v, %s", err, err)
		}
	}

	// Wait for ate stats to be populated
	time.Sleep(60 * time.Second)
	// testTraffic(t, args.ate, gdpMAC, gdpEtherType, srcEndPoint, 10, args)

	// Check packet counters after packet out
	// counter_1 := args.ate.Telemetry().Interface(port).Counters().InPkts().Get(t)
	counter_1 := args.dut.Telemetry().Interface(port).Counters().OutPkts().Get(t)

	// Verify InPkts stats to check P4RT stream
	// fmt.Println(counter_0)
	// fmt.Println(counter_1)

	t.Logf("Sends out %v packets on interface %s", counter_1-counter_0, port)
	if counter_1-counter_0 < uint64(float64(packet_count)*0.90) {
		t.Errorf("There are not enought packets recived.")
	}
}
