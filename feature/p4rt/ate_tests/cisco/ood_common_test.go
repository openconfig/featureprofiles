package cisco_p4rt_test

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	p4rt_client "github.com/cisco-open/go-p4/p4rt_client"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/featureprofiles/internal/fptest"
	spb "github.com/openconfig/gnoi/system"
	tpb "github.com/openconfig/gnoi/types"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/ygot/ygot"
	p4_v1 "github.com/p4lang/p4runtime/go/p4/v1"
	"wwwin-github.cisco.com/rehaddad/go-wbb/p4info/wbb"
)

type TrafficFlow interface {
	GetSrcMAC() string
	SetSrcMAC(srcMAC string)
	GetDstMAC() string
	SetDstMAC(dstMAC string)
	GetEtherType() uint32
	SetEtherType(etherType uint32)
	GetSrcIP() string
	SetSrcIP(srcIP string)
	GetDstIP() string
	SetDstIP(dstIP string)
	GetTTL() uint32
	SetTTL(ttl uint32)
	GetTrafficFlow(t *testing.T) *ondatra.Flow
}

type PacketIO interface {
	GetTableEntry(t *testing.T, delete bool) []*wbb.AclWbbIngressTableEntryInfo
	ApplyConfig(t *testing.T, dut *ondatra.DUTDevice, delete bool)
	GetPacketOut(t *testing.T, portID uint32, submitIngress bool) *p4_v1.PacketOut
	GetPacketTemplate(t *testing.T) *PacketIOPacket
	GetTrafficFlow(t *testing.T, ate *ondatra.ATEDevice, frameSize uint32, frameRate uint64) []*ondatra.Flow
	GetEgressPort(t *testing.T) []string
	SetEgressPorts(t *testing.T, portIDs []string)
	GetIngressPort(t *testing.T) string
	SetIngressPorts(t *testing.T, portID string)
	GetPacketIOPacket(t *testing.T) *PacketIOPacket
}

type PacketIOPacket struct {
	SrcMAC, DstMAC   *string
	EthernetType     *uint32
	SrcIPv4, DstIPv4 *string
	SrcIPv6, DstIPv6 *string
	TTL              *uint32
}

func programmTableEntry(ctx context.Context, t *testing.T, client *p4rt_client.P4RTClient, packetIO PacketIO, delete bool) error {

	err := client.Write(&p4_v1.WriteRequest{
		DeviceId:   deviceID,
		ElectionId: &p4_v1.Uint128{High: uint64(0), Low: uint64(100)},
		Updates: wbb.AclWbbIngressTableEntryGet(
			packetIO.GetTableEntry(t, delete),
		),
		Atomicity: p4_v1.WriteRequest_CONTINUE_ON_ERROR,
	})
	if err != nil {
		return err
	}
	return nil
}

func decodePacket(t *testing.T, packetData []byte) (string, layers.EthernetType) {
	t.Helper()
	packet := gopacket.NewPacket(packetData, layers.LayerTypeEthernet, gopacket.Default)
	etherHeader := packet.Layer(layers.LayerTypeEthernet)
	if etherHeader != nil {
		header, decoded := etherHeader.(*layers.Ethernet)
		if decoded {
			return header.DstMAC.String(), header.EthernetType
		}
	}
	return "", layers.EthernetType(0)
}

func decodeIPPacket(t *testing.T, packetData []byte) (string, string) {
	t.Helper()
	packet := gopacket.NewPacket(packetData, layers.IPProtocolIPv4, gopacket.Default)
	ipHeader := packet.Layer(layers.LayerTypeIPv4)
	if ipHeader != nil {
		header, decoded := ipHeader.(*layers.IPv4)
		if decoded {
			return header.SrcIP.String(), header.DstIP.String()
		}
	} else {
		ipHeader = packet.Layer(layers.LayerTypeIPv6)
		header, decoded := ipHeader.(*layers.IPv4)
		if decoded {
			return header.SrcIP.String(), header.DstIP.String()
		}
	}
	// if etherHeader != nil {
	// 	header, decoded := etherHeader.(*layers.Ethernet)
	// 	if decoded {
	// 		return header.DstMAC.String(), header.EthernetType
	// 	}
	// }
	return "", ""
}

func getPackets(t *testing.T, client *p4rt_client.P4RTClient, packetCount int) []*p4rt_client.P4RTPacketInfo {
	packets := []*p4rt_client.P4RTPacketInfo{}
	for i := 0; i < packetCount; i++ {
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
	return packets
}

func validatePackets(t *testing.T, args *testArgs, packets []*p4rt_client.P4RTPacketInfo) {
	t.Logf("Start to decode packet.")
	wantPacket := args.packetIO.GetPacketTemplate(t)
	for _, packet := range packets {
		// t.Logf("Packet: %v", packet)
		if packet != nil {
			if wantPacket.DstMAC != nil && wantPacket.EthernetType != nil {
				dstMac, etherType := decodePacket(t, packet.Pkt.GetPayload())
				t.Logf("Decoded Ether Type: %v; Decoded DST MAC: %v", etherType, dstMac)
				if dstMac != *wantPacket.DstMAC || etherType != layers.EthernetType(*wantPacket.EthernetType) {
					t.Errorf("Packet is not matching wanted packet.")
				}
			}

			if wantPacket.DstIPv4 != nil || wantPacket.DstIPv6 != nil {
				srcIP, dstIP := decodeIPPacket(t, packet.Pkt.GetPayload())
				if *wantPacket.SrcIPv4 != srcIP && *wantPacket.SrcIPv6 != srcIP && *wantPacket.DstIPv4 != dstIP && *wantPacket.DstIPv6 != dstIP {
					t.Errorf("IP header in Packet is not matching wanted packet.")
				}
			}

			// TODO: Check Port-id in MetaData
			metaData := packet.Pkt.GetMetadata()
			for _, data := range metaData {
				t.Logf("Metadata: %d, %s", data.GetMetadataId(), data.GetValue())
				if data.GetMetadataId() == METADATA_INGRESS_PORT {
					if string(data.GetValue()) != args.packetIO.GetIngressPort(t) {
						t.Errorf("Ingress Port Id is not matching expectation...")
					}
				}
				if data.GetMetadataId() == METADATA_EGRESS_PORT {
					found := false
					for _, portData := range args.packetIO.GetEgressPort(t) {
						if string(data.GetValue()) == portData {
							found = true
						}
					}
					if !found {
						t.Errorf("Ingress Port Id is not matching expectation...")
					}

				}
			}
		}
	}
}

func sendPackets(t *testing.T, client *p4rt_client.P4RTClient, packet *p4_v1.PacketOut, packetCount int) {
	for i := 0; i < packetCount; i++ {
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

func testP4RTTraffic(t *testing.T, ate *ondatra.ATEDevice, flows []*ondatra.Flow, srcEndPoint *ondatra.Interface, duration int) {
	for _, flow := range flows {
		flow.WithSrcEndpoints(srcEndPoint).WithDstEndpoints(srcEndPoint)
	}

	ate.Traffic().Start(t, flows...)
	time.Sleep(time.Duration(duration) * time.Second)

	ate.Traffic().Stop(t)
}

func configureInterface(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, interfaceName, ipv4Address string, subInterface uint32) {
	config := telemetry.Interface{
		Name:    ygot.String(interfaceName),
		Enabled: ygot.Bool(true),
		Subinterface: map[uint32]*telemetry.Interface_Subinterface{
			subInterface: &telemetry.Interface_Subinterface{
				Index: ygot.Uint32(subInterface),
				Ipv4: &telemetry.Interface_Subinterface_Ipv4{
					Address: map[string]*telemetry.Interface_Subinterface_Ipv4_Address{
						ipv4Address: &telemetry.Interface_Subinterface_Ipv4_Address{
							Ip:           ygot.String(ipv4Address),
							PrefixLength: ygot.Uint8(24),
						},
					},
				},
				Ipv6: &telemetry.Interface_Subinterface_Ipv6{
					Enabled: ygot.Bool(true),
				},
			},
		},
	}
	if subInterface > 0 {
		config.Subinterface[subInterface].Vlan = &telemetry.Interface_Subinterface_Vlan{
			Match: &telemetry.Interface_Subinterface_Vlan_Match{
				SingleTagged: &telemetry.Interface_Subinterface_Vlan_Match_SingleTagged{
					VlanId: ygot.Uint16(uint16(subInterface)),
				},
			},
		}
		dut.Config().Interface(interfaceName).Subinterface(subInterface).Replace(t, config.Subinterface[subInterface])
	} else {
		dut.Config().Interface(interfaceName).Replace(t, &config)
	}

}

func testEntryProgrammingPacketIn(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA
	// Program the entry
	if err := programmTableEntry(ctx, t, client, args.packetIO, false); err != nil {
		t.Errorf("There is error when inserting the match entry")
	}
	defer programmTableEntry(ctx, t, client, args.packetIO, true)

	// Send Packet
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	testP4RTTraffic(t, args.ate, args.packetIO.GetTrafficFlow(t, args.ate, 300, 2), srcEndPoint, 10)

	// Check PacketIn on P4Client
	packets := getPackets(t, client, 40)

	// t.Logf("Captured packets: %v", len(packets))
	if len(packets) == 0 {
		t.Errorf("There is no packets received.")
	}

	validatePackets(t, args, packets)
}

func testEntryProgrammingPacketInAndNoReply(ctx context.Context, t *testing.T, args *testArgs) {
	portName := fptest.SortPorts(args.dut.Ports())[0].Name()
	count_0 := args.dut.Telemetry().Interface(portName).Counters().OutPkts().Get(t)
	testEntryProgrammingPacketIn(ctx, t, args)
	count_1 := args.dut.Telemetry().Interface(portName).Counters().OutPkts().Get(t)
	if count_1-count_0 > 15 {
		t.Errorf("Unexpected replies are sent from router!")
	}
}

func testEntryProgrammingPacketInWithUnicastMAC(ctx context.Context, t *testing.T, args *testArgs) {
	testEntryProgrammingPacketInWithNewMAC(ctx, t, args, "00:aa:bb:cc:dd:ee", false)
}

func testEntryProgrammingPacketInWithBroadcastMAC(ctx context.Context, t *testing.T, args *testArgs) {
	testEntryProgrammingPacketInWithNewMAC(ctx, t, args, "ff:ff:ff:ff:ff:ff", false)
}

func testProgrammingPacketInWithInvalidSrcMAC(ctx context.Context, t *testing.T, args *testArgs) {
	testEntryProgrammingPacketInWithNewMAC(ctx, t, args, "ff:ff:ff:ff:ff:ff", true)
}

func testEntryProgrammingPacketInWithNewMAC(ctx context.Context, t *testing.T, args *testArgs, macAddress string, src bool) {
	client := args.p4rtClientA
	// Program the entry
	if err := programmTableEntry(ctx, t, client, args.packetIO, false); err != nil {
		t.Errorf("There is error when inserting the match entry")
	}
	defer programmTableEntry(ctx, t, client, args.packetIO, true)

	// Modify Traffic DstMAC to Unicast MAC
	mac := args.packetIO.GetPacketTemplate(t).DstMAC
	if src {
		mac = args.packetIO.GetPacketTemplate(t).SrcMAC
	}
	currentMAC := *mac
	*mac = macAddress
	defer func() { *mac = currentMAC }()

	// Send Packet
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	testP4RTTraffic(t, args.ate, args.packetIO.GetTrafficFlow(t, args.ate, 300, 2), srcEndPoint, 10)

	// Check PacketIn on P4Client
	packets := getPackets(t, client, 40)

	// t.Logf("Captured packets: %v", len(packets))
	if len(packets) > 0 {
		t.Errorf("Unexpected packets received")
	}
}

func testEntryProgrammingPacketInWithForUsIP(ctx context.Context, t *testing.T, args *testArgs) {
	testEntryProgrammingPacketInWithNewIP(ctx, t, args, "100.101.102.103", false)
}

func testEntryProgrammingPacketInWithNonExistIP(ctx context.Context, t *testing.T, args *testArgs) {
	testEntryProgrammingPacketInWithNewIP(ctx, t, args, "200.101.102.103", false)
}

func testEntryProgrammingPacketInWithNewIP(ctx context.Context, t *testing.T, args *testArgs, ipAddress string, isIPv6 bool) {
	client := args.p4rtClientA
	// Program the entry
	if err := programmTableEntry(ctx, t, client, args.packetIO, false); err != nil {
		t.Errorf("There is error when inserting the match entry")
	}
	defer programmTableEntry(ctx, t, client, args.packetIO, true)

	// Modify Traffic DstMAC to Unicast MAC
	dstIP := args.packetIO.GetPacketTemplate(t).DstIPv4
	flows := args.packetIO.GetTrafficFlow(t, args.ate, 300, 2)

	if isIPv6 {
		dstIP = args.packetIO.GetPacketTemplate(t).DstIPv6
		flows = []*ondatra.Flow{flows[1]}
	} else {
		flows = []*ondatra.Flow{flows[0]}
	}
	currentIP := *dstIP
	*dstIP = ipAddress
	defer func() { *dstIP = currentIP }()

	// Send Packet
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	testP4RTTraffic(t, args.ate, flows, srcEndPoint, 10)

	// Check PacketIn on P4Client
	packets := getPackets(t, client, 40)

	// t.Logf("Captured packets: %v", len(packets))
	if len(packets) > 0 {
		t.Errorf("Unexpected packets received")
	}
}

func testEntryProgrammingPacketInWithOutterTTL3(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA
	// Program the entry
	if err := programmTableEntry(ctx, t, client, args.packetIO, false); err != nil {
		t.Errorf("There is error when inserting the match entry")
	}
	defer programmTableEntry(ctx, t, client, args.packetIO, true)

	// Modify Traffic DstMAC to Unicast MAC
	currentTTL := args.packetIO.GetPacketIOPacket(t).TTL
	*currentTTL = 3

	defer func() { *currentTTL = 1 }()

	// Send Packet
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	testP4RTTraffic(t, args.ate, args.packetIO.GetTrafficFlow(t, args.ate, 300, 2), srcEndPoint, 10)

	// Check PacketIn on P4Client
	packets := getPackets(t, client, 40)

	// t.Logf("Captured packets: %v", len(packets))
	if len(packets) > 0 {
		t.Errorf("Unexpected packets received")
	}
}

func testPacketInWithoutEntryProgramming(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA

	// Send Packet
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	testP4RTTraffic(t, args.ate, args.packetIO.GetTrafficFlow(t, args.ate, 300, 2), srcEndPoint, 10)

	// Check PacketIn on P4Client
	packets := getPackets(t, client, 40)

	// t.Logf("Captured packets: %v", len(packets))
	if len(packets) > 0 {
		t.Errorf("Unexpected packets received")
		for _, packet := range packets {
			// t.Logf("Packet: %v", packet)
			if packet != nil {
				// t.Logf("Decoded Ether Type: %v; Decoded DST MAC: %v", etherType, dstMac)
				dstMac, etherType := decodePacket(t, packet.Pkt.GetPayload())
				t.Logf(dstMac, etherType)
				// if dstMac != *wantPacket.DstMAC || etherType != layers.EthernetType(*wantPacket.EthernetType) {
				// 	t.Errorf("Packet is not matching wanted packet.")
				// }

			}
		}
	}
}

func testPacketInWithoutEntryProgrammingWithNewMAC(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA

	macAddress := "00:aa:bb:cc:dd:ee"
	// Modify Traffic DstMAC to Unicast MAC
	mac := args.packetIO.GetPacketTemplate(t).DstMAC
	currentMAC := *mac
	*mac = macAddress
	defer func() { *mac = currentMAC }()

	// Send Packet
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	testP4RTTraffic(t, args.ate, args.packetIO.GetTrafficFlow(t, args.ate, 300, 2), srcEndPoint, 10)

	// Check PacketIn on P4Client
	packets := getPackets(t, client, 40)

	// t.Logf("Captured packets: %v", len(packets))
	if len(packets) > 0 {
		t.Errorf("Unexpected packets received")
	}
}

func testEntryProgrammingPacketInThenRemoveEntry(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA

	if err := programmTableEntry(ctx, t, client, args.packetIO, false); err != nil {
		t.Errorf("There is error when inserting the match entry")
	}

	// Send GDP Packet
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	testP4RTTraffic(t, args.ate, args.packetIO.GetTrafficFlow(t, args.ate, 300, 2), srcEndPoint, 10)

	// Check PacketIn on P4Client
	packets := getPackets(t, client, 40)

	// t.Logf("Captured packets: %v", len(packets))
	if len(packets) == 0 {
		t.Errorf("There is no packets received.")
	}

	validatePackets(t, args, packets)

	programmTableEntry(ctx, t, client, args.packetIO, true)

	testP4RTTraffic(t, args.ate, args.packetIO.GetTrafficFlow(t, args.ate, 300, 2), srcEndPoint, 10)

	// Check PacketIn on P4Client
	packets = getPackets(t, client, 40)

	// t.Logf("Captured packets: %v", len(packets))
	if len(packets) > 0 {
		t.Errorf("Unexpected packets received.")
	}
}

func testEntryProgrammingPacketInAndChangeDeviceID(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA

	// Program the entry
	if err := programmTableEntry(ctx, t, client, args.packetIO, false); err != nil {
		t.Errorf("There is error when inserting the match entry")
	}
	defer programmTableEntry(ctx, t, client, args.packetIO, true)

	componentName := getComponentID(ctx, t, args.dut)
	component := telemetry.Component{}
	component.IntegratedCircuit = &telemetry.Component_IntegratedCircuit{}
	component.Name = ygot.String(componentName)
	component.IntegratedCircuit.NodeId = ygot.Uint64(^deviceID)
	args.dut.Config().Component(componentName).Replace(t, &component)

	// Setup P4RT ClientB to be primary
	newStreamName := "new_primary"
	setupPrimaryP4RTClient(ctx, t, args.p4rtClientB, ^deviceID, electionID, newStreamName)

	defer func() {
		componentName := getComponentID(ctx, t, args.dut)
		component := telemetry.Component{}
		component.IntegratedCircuit = &telemetry.Component_IntegratedCircuit{}
		component.Name = ygot.String(componentName)
		component.IntegratedCircuit.NodeId = ygot.Uint64(deviceID)
		args.dut.Config().Component(componentName).Replace(t, &component)

		args.p4rtClientA.StreamChannelDestroy(&streamName)
		args.p4rtClientB.StreamChannelDestroy(&streamName)
		args.p4rtClientC.StreamChannelDestroy(&streamName)
		args.p4rtClientD.StreamChannelDestroy(&streamName)

		args.p4rtClientB.StreamChannelDestroy(&newStreamName)

		setupP4RTClient(ctx, t, args)
	}()

	// Send Packet
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	testP4RTTraffic(t, args.ate, args.packetIO.GetTrafficFlow(t, args.ate, 300, 2), srcEndPoint, 10)

	// Check PacketIn on P4Client
	packets := getPackets(t, client, 40)

	// t.Logf("Captured packets: %v", len(packets))
	if len(packets) > 0 {
		t.Errorf("Unexpected packets are received.")
	}

	// Capture packets on clientB
	packets = getPackets(t, args.p4rtClientB, 40)

	validatePackets(t, args, packets)
}

func testEntryProgrammingPacketInAndChangePortID(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA

	// Program the entry
	if err := programmTableEntry(ctx, t, client, args.packetIO, false); err != nil {
		t.Errorf("There is error when inserting the match entry")
	}
	defer programmTableEntry(ctx, t, client, args.packetIO, true)

	// Send Packet
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	testP4RTTraffic(t, args.ate, args.packetIO.GetTrafficFlow(t, args.ate, 300, 2), srcEndPoint, 10)

	// Check PacketIn on P4Client
	packets := getPackets(t, client, 40)

	// t.Logf("Captured packets: %v", len(packets))
	if len(packets) == 0 {
		t.Errorf("There is no packets received.")
	}

	validatePackets(t, args, packets)

	newPortID := ^portID
	portName := fptest.SortPorts(args.dut.Ports())[0].Name()
	args.packetIO.SetIngressPorts(t, fmt.Sprint(newPortID))
	args.dut.Config().Interface(portName).Update(t, &telemetry.Interface{
		Name: ygot.String(portName),
		Id:   ygot.Uint32(newPortID),
	})

	defer args.packetIO.SetIngressPorts(t, fmt.Sprint(portID))
	defer args.dut.Config().Interface(portName).Update(t, &telemetry.Interface{
		Name: ygot.String(portName),
		Id:   ygot.Uint32(portID),
	})

	testP4RTTraffic(t, args.ate, args.packetIO.GetTrafficFlow(t, args.ate, 300, 2), srcEndPoint, 10)

	// Check PacketIn on P4Client
	packets = getPackets(t, client, 40)

	// t.Logf("Captured packets: %v", len(packets))
	if len(packets) == 0 {
		t.Errorf("There is no packets received.")
	}

	validatePackets(t, args, packets)
}

func testProgrammingPacketInWithInterfaceMACAsGDPMac(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA
	// Program the entry
	if err := programmTableEntry(ctx, t, client, args.packetIO, false); err != nil {
		t.Errorf("There is error when inserting the match entry")
	}
	defer programmTableEntry(ctx, t, client, args.packetIO, true)

	// Change mac address for the related bundle interface MAC
	newMAC := gdpMAC
	currentMAC := args.dut.Telemetry().Interface(args.interfaces.in[0]).Ethernet().MacAddress().Get(t)
	args.dut.Config().Interface(args.interfaces.in[0]).Ethernet().MacAddress().Replace(t, newMAC)
	defer args.dut.Config().Interface(args.interfaces.in[0]).Ethernet().MacAddress().Replace(t, currentMAC)

	// Send Packet
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	testP4RTTraffic(t, args.ate, args.packetIO.GetTrafficFlow(t, args.ate, 300, 2), srcEndPoint, 10)

	// Check PacketIn on P4Client
	packets := getPackets(t, client, 40)

	// t.Logf("Captured packets: %v", len(packets))
	if len(packets) > 0 {
		t.Errorf("Unexpected packets received.")
	}
}

func testEntryProgrammingPacketInDowngradePrimaryController(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA

	// Program the entry
	if err := programmTableEntry(ctx, t, client, args.packetIO, false); err != nil {
		t.Errorf("There is error when inserting the match entry")
	}
	defer programmTableEntry(ctx, t, client, args.packetIO, true)

	// Send Packet
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	testP4RTTraffic(t, args.ate, args.packetIO.GetTrafficFlow(t, args.ate, 300, 2), srcEndPoint, 10)

	// Check PacketIn on P4Client
	packets := getPackets(t, client, 40)

	// t.Logf("Captured packets: %v", len(packets))
	if len(packets) == 0 {
		t.Errorf("There is no packets received.")
	}

	validatePackets(t, args, packets)

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
	testP4RTTraffic(t, args.ate, args.packetIO.GetTrafficFlow(t, args.ate, 300, 2), srcEndPoint, 10)

	// Check PacketIn on P4Client
	packets = getPackets(t, newClient, 40)

	// t.Logf("Captured packets: %v", len(packets))
	if len(packets) == 0 {
		t.Errorf("There is no packets received.")
	}

	validatePackets(t, args, packets)
}

func testEntryProgrammingPacketInDowngradePrimaryControllerWithoutStandby(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA
	args.p4rtClientB.StreamChannelDestroy(&streamName)
	args.p4rtClientC.StreamChannelDestroy(&streamName)
	args.p4rtClientD.StreamChannelDestroy(&streamName)
	defer func() {
		setupPrimaryP4RTClient(ctx, t, args.p4rtClientA, deviceID, electionID, streamName)
		setupBackupP4RTClient(ctx, t, args.p4rtClientB, deviceID, electionID-1, streamName)
		setupBackupP4RTClient(ctx, t, args.p4rtClientB, deviceID, electionID-2, streamName)
		setupBackupP4RTClient(ctx, t, args.p4rtClientB, deviceID, electionID-3, streamName)
		programmTableEntry(ctx, t, args.p4rtClientA, args.packetIO, true)
	}()

	programmTableEntry(ctx, t, args.p4rtClientA, args.packetIO, true)

	// Program the entry
	if err := programmTableEntry(ctx, t, client, args.packetIO, false); err != nil {
		t.Errorf("There is error when inserting the match entry")
	}

	// Send Packet
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	testP4RTTraffic(t, args.ate, args.packetIO.GetTrafficFlow(t, args.ate, 300, 2), srcEndPoint, 10)

	// Check PacketIn on P4Client
	packets := getPackets(t, client, 40)

	// t.Logf("Captured packets: %v", len(packets))
	if len(packets) == 0 {
		t.Errorf("There is no packets received.")
	}

	validatePackets(t, args, packets)

	client.StreamChannelDestroy(&streamName)

	testP4RTTraffic(t, args.ate, args.packetIO.GetTrafficFlow(t, args.ate, 300, 2), srcEndPoint, 10)
}

func testEntryProgrammingPacketInRecoverPrimaryController(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA

	// Program the entry
	if err := programmTableEntry(ctx, t, client, args.packetIO, false); err != nil {
		t.Errorf("There is error when inserting the match entry")
	}
	defer programmTableEntry(ctx, t, client, args.packetIO, true)

	// Send Packet
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	testP4RTTraffic(t, args.ate, args.packetIO.GetTrafficFlow(t, args.ate, 300, 2), srcEndPoint, 10)

	// Check PacketIn on P4Client
	packets := getPackets(t, client, 40)

	// t.Logf("Captured packets: %v", len(packets))
	if len(packets) == 0 {
		t.Errorf("There is no packets received.")
	}

	validatePackets(t, args, packets)

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

	testP4RTTraffic(t, args.ate, args.packetIO.GetTrafficFlow(t, args.ate, 300, 2), srcEndPoint, 10)

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

	testP4RTTraffic(t, args.ate, args.packetIO.GetTrafficFlow(t, args.ate, 300, 2), srcEndPoint, 10)
	// Check PacketIn on P4Client
	packets = getPackets(t, client, 40)

	// t.Logf("Captured packets: %v", len(packets))
	if len(packets) == 0 {
		t.Errorf("There is no packets received.")
	}

	validatePackets(t, args, packets)
}

func testEntryProgrammingPacketInWithMoreMatchingField(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA

	// Program the entry
	tableEntry := args.packetIO.GetTableEntry(t, false)
	if len(tableEntry) == 1 && tableEntry[0].IsIpv4 == 0 {
		tableEntry[0].IsIpv4 = uint8(1)
		client.Write(&p4_v1.WriteRequest{
			DeviceId:   deviceID,
			ElectionId: &p4_v1.Uint128{High: uint64(0), Low: electionID},
			Updates: wbb.AclWbbIngressTableEntryGet(
				tableEntry,
			),
			Atomicity: p4_v1.WriteRequest_CONTINUE_ON_ERROR,
		})
	} else {
		for _, entry := range tableEntry {
			entry.EtherType = uint16(123)
		}
	}

	defer func() {
		tableEntry[0].Type = p4_v1.Update_DELETE
		client.Write(&p4_v1.WriteRequest{
			DeviceId:   deviceID,
			ElectionId: &p4_v1.Uint128{High: uint64(0), Low: electionID},
			Updates: wbb.AclWbbIngressTableEntryGet(
				tableEntry,
			),
			Atomicity: p4_v1.WriteRequest_CONTINUE_ON_ERROR,
		})
	}()

	// Send Packet
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	testP4RTTraffic(t, args.ate, args.packetIO.GetTrafficFlow(t, args.ate, 300, 2), srcEndPoint, 10)

	// Check PacketIn on P4Client
	packets := getPackets(t, client, 40)

	// t.Logf("Captured packets: %v", len(packets))
	if len(packets) > 0 {
		t.Errorf("Unexpected packets are received.")
	}
}

func testEntryProgrammingPacketInWithouthPortID(ctx context.Context, t *testing.T, args *testArgs) {
	portName := fptest.SortPorts(args.dut.Ports())[0].Name()
	args.dut.Config().Interface(portName).Id().Delete(t)
	defer args.dut.Config().Interface(portName).Update(t, &telemetry.Interface{
		Name: ygot.String(portName),
		Id:   ygot.Uint32(portID),
	})

	client := args.p4rtClientA

	// Program the entry
	if err := programmTableEntry(ctx, t, client, args.packetIO, false); err != nil {
		t.Errorf("There is error when inserting the match entry")
	}
	defer programmTableEntry(ctx, t, client, args.packetIO, true)

	// Send Packet
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	testP4RTTraffic(t, args.ate, args.packetIO.GetTrafficFlow(t, args.ate, 300, 2), srcEndPoint, 10)

	// Check PacketIn on P4Client
	packets := getPackets(t, client, 40)

	// t.Logf("Captured packets: %v", len(packets))
	if len(packets) > 0 {
		t.Errorf("Unexpected packets received.")
	}
}

func testEntryProgrammingPacketInWithouthPortIDThenAddPortID(ctx context.Context, t *testing.T, args *testArgs) {
	portName := fptest.SortPorts(args.dut.Ports())[0].Name()
	args.dut.Config().Interface(portName).Id().Delete(t)
	defer args.dut.Config().Interface(portName).Update(t, &telemetry.Interface{
		Name: ygot.String(portName),
		Id:   ygot.Uint32(portID),
	})

	client := args.p4rtClientA

	// Program the entry
	if err := programmTableEntry(ctx, t, client, args.packetIO, false); err != nil {
		t.Errorf("There is error when inserting the match entry")
	}
	defer programmTableEntry(ctx, t, client, args.packetIO, true)

	// Send Packet
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	testP4RTTraffic(t, args.ate, args.packetIO.GetTrafficFlow(t, args.ate, 300, 2), srcEndPoint, 10)

	// Check PacketIn on P4Client
	packets := getPackets(t, client, 40)

	// t.Logf("Captured packets: %v", len(packets))
	if len(packets) > 0 {
		t.Errorf("Unexpected packets received.")
	}

	args.dut.Config().Interface(portName).Update(t, &telemetry.Interface{
		Name: ygot.String(portName),
		Id:   ygot.Uint32(^portID),
	})

	testP4RTTraffic(t, args.ate, args.packetIO.GetTrafficFlow(t, args.ate, 300, 2), srcEndPoint, 10)
	// Check PacketIn on P4Client
	packets = getPackets(t, client, 40)

	// t.Logf("Captured packets: %v", len(packets))
	if len(packets) == 0 {
		t.Errorf("There is no packets received.")
	}

	args.packetIO.SetIngressPorts(t, fmt.Sprint(^portID))
	defer args.packetIO.SetIngressPorts(t, fmt.Sprint(portID))
	validatePackets(t, args, packets)
}

func testEntryProgrammingPacketInWithInnerTTL(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA

	// Program the entry
	if err := programmTableEntry(ctx, t, client, args.packetIO, false); err != nil {
		t.Errorf("There is error when inserting the match entry")
	}
	defer programmTableEntry(ctx, t, client, args.packetIO, true)

	flows := args.packetIO.GetTrafficFlow(t, args.ate, 300, 2)
	newFlows := []*ondatra.Flow{}
	for _, flow := range flows {
		ipv4Header := false
		headers := flow.Headers()
		for _, header := range headers {
			if v, ok := header.(*ondatra.IPv4Header); ok {
				v.WithTTL(64)
				ipv4Header = true
			}
			if v, ok := header.(*ondatra.IPv6Header); ok {
				v.WithHopLimit(64)
			}
		}
		if ipv4Header {
			innerHeader := ondatra.NewIPv4Header()
			innerHeader.WithSrcAddress("1.1.1.1")
			innerHeader.WithDstAddress("2.2.2.2")
			innerHeader.WithTTL(uint8(1))
			headers = append(headers, innerHeader)
			flow.WithHeaders(headers...)
			newFlows = append(newFlows, flow)
		} else {
			innerHeader := ondatra.NewIPv6Header()
			innerHeader.WithSrcAddress("1::1")
			innerHeader.WithDstAddress("2::2")
			innerHeader.WithHopLimit(uint8(1))
			headers = append(headers, innerHeader)
			flow.WithHeaders(headers...)
			newFlows = append(newFlows, flow)
		}
	}

	// Send Packet
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	testP4RTTraffic(t, args.ate, newFlows, srcEndPoint, 10)

	// Check PacketIn on P4Client
	packets := getPackets(t, client, 40)

	// t.Logf("Captured packets: %v", len(packets))
	if len(packets) > 0 {
		t.Errorf("Unexpected packets received.")
	}
}

func testEntryProgrammingPacketInWithMalformedPacket(ctx context.Context, t *testing.T, args *testArgs) {
	portName := fptest.SortPorts(args.dut.Ports())[0].Name()
	args.dut.Config().Interface(portName).Id().Delete(t)
	defer args.dut.Config().Interface(portName).Update(t, &telemetry.Interface{
		Name: ygot.String(portName),
		Id:   ygot.Uint32(portID),
	})

	client := args.p4rtClientA

	// Program the entry
	if err := programmTableEntry(ctx, t, client, args.packetIO, false); err != nil {
		t.Errorf("There is error when inserting the match entry")
	}
	defer programmTableEntry(ctx, t, client, args.packetIO, true)

	flows := args.packetIO.GetTrafficFlow(t, args.ate, 300, 2)
	newFlows := []*ondatra.Flow{}
	for _, flow := range flows {
		if ethernetHeader, ok := flow.Headers()[0].(*ondatra.EthernetHeader); ok {
			if strings.Contains(flow.Name(), "IPv4") {
				ethernetHeader.WithEtherType(0x0800)
			}
			if strings.Contains(flow.Name(), "IPv6") {
				ethernetHeader.WithEtherType(0x8600)
			}
			flow.WithHeaders(ethernetHeader)
			newFlows = append(newFlows, flow)
		}
	}

	// Send Packet
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	testP4RTTraffic(t, args.ate, newFlows, srcEndPoint, 10)

	// Check PacketIn on P4Client
	packets := getPackets(t, client, 40)

	// t.Logf("Captured packets: %v", len(packets))
	if len(packets) > 0 {
		t.Errorf("Unexpected packets received.")
	}
}

func testEntryProgrammingPacketInWithGNOI(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA

	// Program the entry
	if err := programmTableEntry(ctx, t, client, args.packetIO, false); err != nil {
		t.Errorf("There is error when inserting the match entry")
	}
	defer programmTableEntry(ctx, t, client, args.packetIO, true)

	gnoi := args.dut.RawAPIs().GNOI().Default(t)

	gnoi.System().Traceroute(ctx, &spb.TracerouteRequest{
		Destination: atePort1.IPv4,
		InitialTtl:  1,
		MaxTtl:      1,
		L3Protocol:  tpb.L3Protocol_IPV4,
		L4Protocol:  spb.TracerouteRequest_UDP,
	})

	// Check PacketIn on P4Client
	packets := getPackets(t, client, 40)

	// t.Logf("Captured packets: %v", len(packets))
	if len(packets) > 0 {
		t.Errorf("Unexpected packets received.")
	}
}

func testEntryProgrammingPacketInWithUDP(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA

	portName := fptest.SortPorts(args.dut.Ports())[0].Name()
	count_0 := args.dut.Telemetry().Interface(portName).Counters().OutPkts().Get(t)

	// Program the entry
	if err := programmTableEntry(ctx, t, client, args.packetIO, false); err != nil {
		t.Errorf("There is error when inserting the match entry")
	}
	defer programmTableEntry(ctx, t, client, args.packetIO, true)

	flows := args.packetIO.GetTrafficFlow(t, args.ate, 300, 2)
	udpHeader := ondatra.NewUDPHeader()
	udpHeader.WithSrcPort(11111)
	udpHeader.WithDstPort(33433)
	for _, flow := range flows {
		headers := flow.Headers()
		headers = append(headers)
		flow.WithHeaders(headers...)
	}

	// Send Packet
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	testP4RTTraffic(t, args.ate, flows, srcEndPoint, 10)

	// Check PacketIn on P4Client
	packets := getPackets(t, client, 40)

	// t.Logf("Captured packets: %v", len(packets))
	if len(packets) == 0 {
		t.Errorf("There is no packets received.")
	}

	validatePackets(t, args, packets)

	count_1 := args.dut.Telemetry().Interface(portName).Counters().OutPkts().Get(t)
	if count_1-count_0 > 15 {
		t.Errorf("Unexpected replies are sent from router!")
	}
}

func testEntryProgrammingPacketInWithFlowLabel(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA

	// Program the entry
	if err := programmTableEntry(ctx, t, client, args.packetIO, false); err != nil {
		t.Errorf("There is error when inserting the match entry")
	}
	defer programmTableEntry(ctx, t, client, args.packetIO, true)

	skipFlag := true
	flows := args.packetIO.GetTrafficFlow(t, args.ate, 300, 2)
	for _, flow := range flows {
		headers := flow.Headers()
		for _, header := range headers {
			if v, ok := header.(*ondatra.IPv6Header); ok {
				skipFlag = false
				v.WithFlowLabel(11111).FlowLabelRange().WithCount(1000)
			}
		}
	}

	if skipFlag {
		t.Skip()
	}

	// Send Packet
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	testP4RTTraffic(t, args.ate, flows, srcEndPoint, 10)

	// Check PacketIn on P4Client
	packets := getPackets(t, client, 40)

	// t.Logf("Captured packets: %v", len(packets))
	if len(packets) == 0 {
		t.Errorf("There is no packets received.")
	}

	validatePackets(t, args, packets)
}

func testEntryProgrammingPacketInWithPhysicalInterface(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA

	portName := fptest.SortPorts(args.dut.Ports())[0].Name()
	existingConfig := args.dut.Config().Interface(portName).Get(t)
	configureInterface(ctx, t, args.dut, portName, "103.102.101.100", 0)
	defer args.dut.Config().Interface(portName).Replace(t, existingConfig)

	// Program the entry
	if err := programmTableEntry(ctx, t, client, args.packetIO, false); err != nil {
		t.Errorf("There is error when inserting the match entry")
	}
	defer programmTableEntry(ctx, t, client, args.packetIO, true)

	mac := args.packetIO.GetPacketIOPacket(t).DstMAC
	existingMAC := *mac
	*mac = args.dut.Telemetry().Interface(portName).Ethernet().MacAddress().Get(t)
	defer func() { *mac = existingMAC }()

	// Send Packet
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	testP4RTTraffic(t, args.ate, args.packetIO.GetTrafficFlow(t, args.ate, 300, 2), srcEndPoint, 10)

	// Check PacketIn on P4Client
	packets := getPackets(t, client, 40)

	// t.Logf("Captured packets: %v", len(packets))
	if len(packets) == 0 {
		t.Errorf("There is no packets received.")
	}

	validatePackets(t, args, packets)
}

func testEntryProgrammingPacketInWithSubInterface(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA

	configureInterface(ctx, t, args.dut, args.interfaces.in[0], "103.102.101.100", 1)
	defer args.dut.Config().Interface(args.interfaces.in[0]).Subinterface(1).Delete(t)

	// Program the entry
	if err := programmTableEntry(ctx, t, client, args.packetIO, false); err != nil {
		t.Errorf("There is error when inserting the match entry")
	}
	defer programmTableEntry(ctx, t, client, args.packetIO, true)

	flows := args.packetIO.GetTrafficFlow(t, args.ate, 300, 2)

	// Send Packet
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	testP4RTTraffic(t, args.ate, flows, srcEndPoint, 10)

	// Check PacketIn on P4Client
	packets := getPackets(t, client, 40)

	// t.Logf("Captured packets: %v", len(packets))
	if len(packets) == 0 {
		t.Errorf("There is no packets received.")
	}

	validatePackets(t, args, packets)

	for _, flow := range flows {
		headers := flow.Headers()
		if v, ok := headers[0].(*ondatra.EthernetHeader); ok {
			v.WithVLANID(1)
		}
	}

	testP4RTTraffic(t, args.ate, flows, srcEndPoint, 10)
	// Check PacketIn on P4Client
	packets = getPackets(t, client, 40)

	// t.Logf("Captured packets: %v", len(packets))
	if len(packets) > 0 {
		t.Errorf("Unexpected packets received.")
	}
}

func testEntryProgrammingPacketInWithAcl(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA

	flows := args.packetIO.GetTrafficFlow(t, args.ate, 300, 2)

	skipFlag := false

	for _, flow := range flows {
		headers := flow.Headers()
		for _, header := range headers {
			if _, ok := header.(*ondatra.IPv6Header); ok {
				skipFlag = true
			}
		}
	}

	if skipFlag {
		t.Skip()
	}

	// Program the entry
	if err := programmTableEntry(ctx, t, client, args.packetIO, false); err != nil {
		t.Errorf("There is error when inserting the match entry")
	}
	defer programmTableEntry(ctx, t, client, args.packetIO, true)

	// Configure Acl
	acl := (&telemetry.Device{}).GetOrCreateAcl()
	aclSetIPv4 := acl.GetOrCreateAclSet("ttl-ipv4", telemetry.Acl_ACL_TYPE_ACL_IPV4)
	aclEntryIpv4 := aclSetIPv4.GetOrCreateAclEntry(1).GetOrCreateIpv4()
	aclEntryIpv4.HopLimit = ygot.Uint8(1)
	aclEntryAction := aclSetIPv4.GetOrCreateAclEntry(1).GetOrCreateActions()
	aclEntryAction.ForwardingAction = telemetry.Acl_FORWARDING_ACTION_REJECT
	aclEntryActionDefault := aclSetIPv4.GetOrCreateAclEntry(2).GetOrCreateActions()
	aclEntryActionDefault.ForwardingAction = telemetry.Acl_FORWARDING_ACTION_ACCEPT

	// aclSetIPv6 := acl.GetOrCreateAclSet("ttl-ipv6", telemetry.Acl_ACL_TYPE_ACL_IPV6)
	// aclEntryIPv6 := aclSetIPv6.GetOrCreateAclEntry(1).GetOrCreateIpv6()
	// aclEntryIPv6.HopLimit = ygot.Uint8(1)
	// aclEntryActionIPv6 := aclSetIPv6.GetOrCreateAclEntry(1).GetOrCreateActions()
	// aclEntryActionIPv6.ForwardingAction = telemetry.Acl_FORWARDING_ACTION_REJECT

	args.dut.Config().Acl().Update(t, acl)

	args.dut.Config().Acl().Interface(args.interfaces.in[0]).IngressAclSet("ttl-ipv4", telemetry.Acl_ACL_TYPE_ACL_IPV4).SetName().Update(t, "ttl-ipv4")
	// args.dut.Config().Acl().Interface(args.interfaces.in[0]).IngressAclSet("ttl-ipv6", telemetry.Acl_ACL_TYPE_ACL_IPV6).SetName().Update(t, "ttl-ipv6")
	defer func() {
		args.dut.Config().Acl().Delete(t)
		defer args.dut.Config().Acl().Interface(args.interfaces.in[0]).IngressAclSet("ttl-ipv4", telemetry.Acl_ACL_TYPE_ACL_IPV4).Delete(t)
		// defer args.dut.Config().Acl().Interface(args.interfaces.in[0]).IngressAclSet("ttl-ipv6", telemetry.Acl_ACL_TYPE_ACL_IPV6).Delete(t)
	}()

	// Send Packet
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	testP4RTTraffic(t, args.ate, flows, srcEndPoint, 10)

	// Check PacketIn on P4Client
	packets := getPackets(t, client, 40)

	// t.Logf("Captured packets: %v", len(packets))
	if len(packets) > 0 {
		t.Errorf("Unexpected packets received.")
	}
}

func testEntryProgrammingPacketInScaleRate(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA

	// Program the entry
	if err := programmTableEntry(ctx, t, client, args.packetIO, false); err != nil {
		t.Errorf("There is error when inserting the match entry")
	}
	defer programmTableEntry(ctx, t, client, args.packetIO, true)

	// Send Packet
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	testP4RTTraffic(t, args.ate, args.packetIO.GetTrafficFlow(t, args.ate, 300, 1000), srcEndPoint, 5)

	// Check PacketIn on P4Client
	packets := getPackets(t, client, 6000)

	// t.Logf("Captured packets: %v", len(packets))
	validatePackets(t, args, packets)
}

//-----------------------------//

func testPacketOut(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA

	// Program the entry
	if err := programmTableEntry(ctx, t, client, args.packetIO, false); err != nil {
		t.Errorf("There is error when inserting the match entry")
	}
	defer programmTableEntry(ctx, t, client, args.packetIO, true)

	// Check initial packet counters
	port := fptest.SortPorts(args.dut.Ports())[0].Name()
	counter_0 := args.dut.Telemetry().Interface(port).Counters().OutPkts().Get(t)

	packet := args.packetIO.GetPacketOut(t, portID, true)

	packet_count := 100
	sendPackets(t, client, packet, packet_count)

	// Wait for ate stats to be populated
	time.Sleep(60 * time.Second)

	// Check packet counters after packet out
	// counter_1 := args.ate.Telemetry().Interface(port).Counters().InPkts().Get(t)
	counter_1 := args.dut.Telemetry().Interface(port).Counters().OutPkts().Get(t)

	// Verify InPkts stats to check P4RT stream
	// fmt.Println(counter_0)
	// fmt.Println(counter_1)

	t.Logf("Sends out %v packets on interface %s", counter_1-counter_0, port)

	if counter_1-counter_0 > uint64(float64(packet_count)*0.15) {
		t.Errorf("Unexpected packets are received.")
	}
}

func testPacketOutWithoutMatchEntry(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA

	// Check initial packet counters
	port := fptest.SortPorts(args.dut.Ports())[0].Name()
	counter_0 := args.dut.Telemetry().Interface(port).Counters().OutPkts().Get(t)

	packet := args.packetIO.GetPacketOut(t, portID, true)

	packet_count := 100
	sendPackets(t, client, packet, packet_count)

	// Wait for ate stats to be populated
	time.Sleep(60 * time.Second)

	// Check packet counters after packet out
	// counter_1 := args.ate.Telemetry().Interface(port).Counters().InPkts().Get(t)
	counter_1 := args.dut.Telemetry().Interface(port).Counters().OutPkts().Get(t)

	// Verify InPkts stats to check P4RT stream
	// fmt.Println(counter_0)
	// fmt.Println(counter_1)

	t.Logf("Sends out %v packets on interface %s", counter_1-counter_0, port)

	if counter_1-counter_0 > uint64(float64(packet_count)*0.15) {
		t.Errorf("Not all the packets are received.")
	}
}

func testPacketOutEgress(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA

	// Program the entry
	if err := programmTableEntry(ctx, t, client, args.packetIO, false); err != nil {
		t.Errorf("There is error when inserting the match entry")
	}
	defer programmTableEntry(ctx, t, client, args.packetIO, true)

	// Check initial packet counters
	port := fptest.SortPorts(args.dut.Ports())[0].Name()
	counter_0 := args.dut.Telemetry().Interface(port).Counters().OutPkts().Get(t)

	packet := args.packetIO.GetPacketOut(t, portID, false)

	packet_count := 100
	sendPackets(t, client, packet, packet_count)

	// Wait for ate stats to be populated
	time.Sleep(60 * time.Second)

	// Check packet counters after packet out
	// counter_1 := args.ate.Telemetry().Interface(port).Counters().InPkts().Get(t)
	counter_1 := args.dut.Telemetry().Interface(port).Counters().OutPkts().Get(t)

	// Verify InPkts stats to check P4RT stream
	// fmt.Println(counter_0)
	// fmt.Println(counter_1)

	t.Logf("Sends out %v packets on interface %s", counter_1-counter_0, port)

	if counter_1-counter_0 < uint64(float64(packet_count)*0.95) {
		t.Errorf("Not all the packets are received.")
	}
}

func testPacketOutEgressWithoutMatchEntry(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA

	// Check initial packet counters
	port := fptest.SortPorts(args.dut.Ports())[0].Name()
	counter_0 := args.dut.Telemetry().Interface(port).Counters().OutPkts().Get(t)

	packet := args.packetIO.GetPacketOut(t, portID, false)

	packet_count := 100
	sendPackets(t, client, packet, packet_count)

	// Wait for ate stats to be populated
	time.Sleep(60 * time.Second)

	// Check packet counters after packet out
	// counter_1 := args.ate.Telemetry().Interface(port).Counters().InPkts().Get(t)
	counter_1 := args.dut.Telemetry().Interface(port).Counters().OutPkts().Get(t)

	// Verify InPkts stats to check P4RT stream
	// fmt.Println(counter_0)
	// fmt.Println(counter_1)

	t.Logf("Sends out %v packets on interface %s", counter_1-counter_0, port)

	if counter_1-counter_0 < uint64(float64(packet_count)*0.95) {
		t.Errorf("Not all the packets are received.")
	}
}

func testPacketOutEgressWithInvalidPortId(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA
	// srcEndPoint := args.top.Interfaces()[atePort1.Name]

	// Check initial packet counters
	// port := fptest.SortPorts(args.ate.Ports())[0].Name()
	// counter_0 := args.ate.Telemetry().Interface(port).Counters().InPkts().Get(t)
	port := fptest.SortPorts(args.dut.Ports())[0].Name()
	counter_0 := args.dut.Telemetry().Interface(port).Counters().OutPkts().Get(t)

	packet := args.packetIO.GetPacketOut(t, ^portID, false)

	packet_count := 100
	sendPackets(t, client, packet, packet_count)

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

func testPacketOutEgressWithChangePortId(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA
	// srcEndPoint := args.top.Interfaces()[atePort1.Name]

	newPortID := ^portID
	portName := fptest.SortPorts(args.dut.Ports())[0].Name()
	args.dut.Config().Interface(portName).Update(t, &telemetry.Interface{
		Name: ygot.String(portName),
		Id:   ygot.Uint32(newPortID),
	})

	defer args.dut.Config().Interface(portName).Update(t, &telemetry.Interface{
		Name: ygot.String(portName),
		Id:   ygot.Uint32(portID),
	})

	// Check initial packet counters
	// port := fptest.SortPorts(args.ate.Ports())[0].Name()
	// counter_0 := args.ate.Telemetry().Interface(port).Counters().InPkts().Get(t)
	port := fptest.SortPorts(args.dut.Ports())[0].Name()
	counter_0 := args.dut.Telemetry().Interface(port).Counters().OutPkts().Get(t)

	packet := args.packetIO.GetPacketOut(t, newPortID, false)

	packet_count := 100
	sendPackets(t, client, packet, packet_count)

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

func testPacketOutEgressWithChangeMetadata(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA
	// srcEndPoint := args.top.Interfaces()[atePort1.Name]

	newPortID := ^portID
	portName := fptest.SortPorts(args.dut.Ports())[0].Name()
	args.dut.Config().Interface(portName).Update(t, &telemetry.Interface{
		Name: ygot.String(portName),
		Id:   ygot.Uint32(newPortID),
	})

	defer args.dut.Config().Interface(portName).Update(t, &telemetry.Interface{
		Name: ygot.String(portName),
		Id:   ygot.Uint32(portID),
	})

	// Check initial packet counters
	// port := fptest.SortPorts(args.ate.Ports())[0].Name()
	// counter_0 := args.ate.Telemetry().Interface(port).Counters().InPkts().Get(t)
	port := fptest.SortPorts(args.dut.Ports())[0].Name()
	counter_0 := args.dut.Telemetry().Interface(port).Counters().OutPkts().Get(t)

	packet := args.packetIO.GetPacketOut(t, portID, false)

	// Change metadata
	packet.Metadata[0].MetadataId = uint32(10)
	packet.Metadata[0].Value = []byte{1, 2, 3, 4, 5, 6, 7, 8}

	packet_count := 100
	sendPackets(t, client, packet, packet_count)

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

func testPacketOutIngressWithInterfaceFlap(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA

	// Program the entry
	if err := programmTableEntry(ctx, t, client, args.packetIO, false); err != nil {
		t.Errorf("There is error when inserting the match entry")
	}
	defer programmTableEntry(ctx, t, client, args.packetIO, true)

	// Check initial packet counters
	port := fptest.SortPorts(args.dut.Ports())[0].Name()
	counter_0 := args.dut.Telemetry().Interface(port).Counters().OutPkts().Get(t)

	packet := args.packetIO.GetPacketOut(t, portID, true)

	packet_count := 100
	sendPackets(t, client, packet, packet_count)

	// Wait for ate stats to be populated
	time.Sleep(60 * time.Second)

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

	// Flap interface
	util.SetInterfaceState(t, args.dut, port, false)
	defer util.SetInterfaceState(t, args.dut, port, true)

	sendPackets(t, client, packet, packet_count)

	// Wait for ate stats to be populated
	time.Sleep(60 * time.Second)

	// Check packet counters after packet out
	// counter_1 := args.ate.Telemetry().Interface(port).Counters().InPkts().Get(t)
	counter_2 := args.dut.Telemetry().Interface(port).Counters().OutPkts().Get(t)

	// Verify InPkts stats to check P4RT stream
	// fmt.Println(counter_0)
	// fmt.Println(counter_1)

	t.Logf("Sends out %v packets on interface %s", counter_2-counter_1, port)

	if counter_2-counter_1 > uint64(float64(packet_count)*0.10) {
		t.Errorf("Unexpected packets are received.")
	}
}

func testPacketOutEgressWithInterfaceFlap(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA

	// Program the entry
	if err := programmTableEntry(ctx, t, client, args.packetIO, false); err != nil {
		t.Errorf("There is error when inserting the match entry")
	}
	defer programmTableEntry(ctx, t, client, args.packetIO, true)

	// Check initial packet counters
	port := fptest.SortPorts(args.dut.Ports())[0].Name()
	counter_0 := args.dut.Telemetry().Interface(port).Counters().OutPkts().Get(t)

	packet := args.packetIO.GetPacketOut(t, portID, false)

	packet_count := 100
	sendPackets(t, client, packet, packet_count)

	// Wait for ate stats to be populated
	time.Sleep(60 * time.Second)

	// Check packet counters after packet out
	// counter_1 := args.ate.Telemetry().Interface(port).Counters().InPkts().Get(t)
	counter_1 := args.dut.Telemetry().Interface(port).Counters().OutPkts().Get(t)

	// Verify InPkts stats to check P4RT stream
	// fmt.Println(counter_0)
	// fmt.Println(counter_1)

	t.Logf("Sends out %v packets on interface %s", counter_1-counter_0, port)

	if counter_1-counter_0 < uint64(float64(packet_count)*0.95) {
		t.Errorf("Not all the packets are received.")
	}

	// Flap interface
	util.SetInterfaceState(t, args.dut, port, false)
	defer func() {
		util.SetInterfaceState(t, args.dut, port, true)
		time.Sleep(10 * time.Second)
	}()

	sendPackets(t, client, packet, packet_count)

	// Wait for ate stats to be populated
	time.Sleep(60 * time.Second)

	// Check packet counters after packet out
	// counter_1 := args.ate.Telemetry().Interface(port).Counters().InPkts().Get(t)
	counter_2 := args.dut.Telemetry().Interface(port).Counters().OutPkts().Get(t)

	// Verify InPkts stats to check P4RT stream
	// fmt.Println(counter_0)
	// fmt.Println(counter_1)

	t.Logf("Sends out %v packets on interface %s", counter_2-counter_1, port)

	if counter_2-counter_1 > uint64(float64(packet_count)*0.15) {
		t.Errorf("Unexpected packets are received.")
	}
}

// func testPacketOutEgress(ctx context.Context, t *testing.T, args *testArgs) {
// 	client := args.p4rtClientA

// 	// Program the entry
// 	if err := programmTableEntry(ctx, t, client, args.packetIO, false); err != nil {
// 		t.Errorf("There is error when inserting the match entry")
// 	}
// 	defer programmTableEntry(ctx, t, client, args.packetIO, false)

// 	// Check initial packet counters
// 	port := fptest.SortPorts(args.dut.Ports())[0].Name()
// 	counter_0 := args.dut.Telemetry().Interface(port).Counters().OutPkts().Get(t)

// 	packet := args.packetIO.GetPacketOut(t, portID, true)

// 	packet_count := 100
// 	for i := 0; i < packet_count; i++ {
// 		if err := client.StreamChannelSendMsg(
// 			&streamName, &p4_v1.StreamMessageRequest{
// 				Update: &p4_v1.StreamMessageRequest_Packet{
// 					Packet: packet,
// 				},
// 			}); err != nil {
// 			t.Errorf("There is error seen in Packet Out. %v, %s", err, err)
// 		}
// 	}

// 	// Wait for ate stats to be populated
// 	time.Sleep(60 * time.Second)

// 	// Check packet counters after packet out
// 	// counter_1 := args.ate.Telemetry().Interface(port).Counters().InPkts().Get(t)
// 	counter_1 := args.dut.Telemetry().Interface(port).Counters().OutPkts().Get(t)

// 	// Verify InPkts stats to check P4RT stream
// 	// fmt.Println(counter_0)
// 	// fmt.Println(counter_1)

// 	t.Logf("Sends out %v packets on interface %s", counter_1-counter_0, port)

// 	if counter_1-counter_0 < uint64(float64(packet_count)*0.95) {
// 		t.Errorf("Not all the packets are received.")
// 	}
// }

func testPacketOutEgressScale(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA

	// Program the entry
	// if err := programmTableEntry(ctx, t, client, args.packetIO, false); err != nil {
	// 	t.Errorf("There is error when inserting the match entry")
	// }
	// defer programmTableEntry(ctx, t, client, args.packetIO, true)

	// Check initial packet counters
	port := fptest.SortPorts(args.dut.Ports())[0].Name()
	counter_0 := args.dut.Telemetry().Interface(port).Counters().OutPkts().Get(t)

	packet := args.packetIO.GetPacketOut(t, portID, false)

	packet_count := 1000
	thread_number := 200
	for x := 0; x < thread_number; x++ {
		// Spawn a thread for each iteration in the loop.
		// Pass 'i' into the goroutine's function
		//   in order to make sure each goroutine
		//   uses a different value for 'i'.
		go func() {
			// At the end of the goroutine, tell the WaitGroup
			//   that another thread has completed.
			sendPackets(t, client, packet, packet_count)
		}()
	}

	// Wait for ate stats to be populated
	time.Sleep(60 * time.Second)

	// Check packet counters after packet out
	// counter_1 := args.ate.Telemetry().Interface(port).Counters().InPkts().Get(t)
	counter_1 := args.dut.Telemetry().Interface(port).Counters().OutPkts().Get(t)

	// Verify InPkts stats to check P4RT stream
	// fmt.Println(counter_0)
	// fmt.Println(counter_1)

	t.Logf("Sends out %v packets on interface %s", counter_1-counter_0, port)

	if counter_1-counter_0 < uint64(float64(packet_count)*0.95) {
		t.Errorf("Not all the packets are received.")
	}
}
