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

package google_discovery_protocol_packetout_test

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"sort"
	"testing"
	"time"

	"github.com/cisco-open/go-p4/p4rt_client"
	"github.com/cisco-open/go-p4/utils"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/openconfig/featureprofiles/feature/experimental/p4rt/wbb"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/ygot/ygot"
	p4_v1 "github.com/p4lang/p4runtime/go/p4/v1"
)

const (
	ipv4PrefixLen = 30
	packetCount   = 100
)

var (
	p4InfoFile                                = flag.String("p4info_file_location", "../../wbb.p4info.pb.txt", "Path to the p4info file.")
	p4rtNodeName                              = flag.String("p4rt_node_name", "0/1/CPU0-NPU1", "component name for P4RT Node")
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

var (
	dutPort1 = attrs.Attributes{
		Desc:    "dutPort1",
		IPv4:    "192.0.2.1",
		IPv4Len: ipv4PrefixLen,
	}

	atePort1 = attrs.Attributes{
		Name:    "atePort1",
		IPv4:    "192.0.2.2",
		IPv4Len: ipv4PrefixLen,
	}

	dutPort2 = attrs.Attributes{
		Desc:    "dutPort2",
		IPv4:    "192.0.2.5",
		IPv4Len: ipv4PrefixLen,
	}

	atePort2 = attrs.Attributes{
		Name:    "atePort2",
		IPv4:    "192.0.2.6",
		IPv4Len: ipv4PrefixLen,
	}
)

type PacketIO interface {
	GetTableEntry(delete bool) []*wbb.ACLWbbIngressTableEntryInfo
	GetPacketOut(portID uint32, submitIngress bool) []*p4_v1.PacketOut
}

type testArgs struct {
	ctx      context.Context
	leader   *p4rt_client.P4RTClient
	follower *p4rt_client.P4RTClient
	dut      *ondatra.DUTDevice
	ate      *ondatra.ATEDevice
	top      *ondatra.ATETopology
	packetIO PacketIO
}

// programmTableEntry programs or deletes p4rt table entry based on delete flag.
func programmTableEntry(ctx context.Context, t *testing.T, client *p4rt_client.P4RTClient, packetIO PacketIO, delete bool) error {
	t.Helper()
	err := client.Write(&p4_v1.WriteRequest{
		DeviceId:   deviceID,
		ElectionId: &p4_v1.Uint128{High: uint64(0), Low: electionID},
		Updates: wbb.ACLWbbIngressTableEntryGet(
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

// testPacketOut sends out PacketOut with GDP payload on p4rt leader or
// follower client, then verify DUT interface statistics
func testPacketOut(ctx context.Context, t *testing.T, args *testArgs) {
	leader := args.leader
	follower := args.follower

	// Insert wbb acl entry on the DUT
	if err := programmTableEntry(ctx, t, leader, args.packetIO, false); err != nil {
		t.Fatalf("There is error when programming entry")
	}
	// Delete wbb acl entry on the device
	defer programmTableEntry(ctx, t, leader, args.packetIO, true)

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
			port := sortPorts(args.ate.Ports())[0].Name()
			counter0 := args.ate.Telemetry().Interface(port).Counters().InPkts().Get(t)

			packets := args.packetIO.GetPacketOut(portID, false)
			sendPackets(t, test.client, packets, packetCount)

			// Wait for ate stats to be populated
			time.Sleep(60 * time.Second)

			// Check packet counters after packet out
			counter1 := args.ate.Telemetry().Interface(port).Counters().InPkts().Get(t)

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

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// sortPorts sorts the ports by the testbed port ID.
func sortPorts(ports []*ondatra.Port) []*ondatra.Port {
	sort.Slice(ports, func(i, j int) bool {
		idi, idj := ports[i].ID(), ports[j].ID()
		li, lj := len(idi), len(idj)
		if li == lj {
			return idi < idj
		}
		return li < lj // "port2" < "port10"
	})
	return ports
}

// configInterfaceDUT configures the interface with the Addrs.
func configInterfaceDUT(i *telemetry.Interface, a *attrs.Attributes) *telemetry.Interface {
	i.Description = ygot.String(a.Desc)
	i.Type = telemetry.IETFInterfaces_InterfaceType_ethernetCsmacd
	if *deviations.InterfaceEnabled {
		i.Enabled = ygot.Bool(true)
	}

	s := i.GetOrCreateSubinterface(0)
	s4 := s.GetOrCreateIpv4()
	if *deviations.InterfaceEnabled {
		s4.Enabled = ygot.Bool(true)
	}
	s4a := s4.GetOrCreateAddress(a.IPv4)
	s4a.PrefixLength = ygot.Uint8(ipv4PrefixLen)

	return i
}

// configureDUT configures port1 and port2 on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	d := dut.Config()

	p1 := dut.Port(t, "port1")
	i1 := &telemetry.Interface{Name: ygot.String(p1.Name())}
	d.Interface(p1.Name()).Replace(t, configInterfaceDUT(i1, &dutPort1))

	p2 := dut.Port(t, "port2")
	i2 := &telemetry.Interface{Name: ygot.String(p2.Name())}
	d.Interface(p2.Name()).Replace(t, configInterfaceDUT(i2, &dutPort2))
}

// configureATE configures port1 and port2 on the ATE.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) *ondatra.ATETopology {
	top := ate.Topology().New()

	p1 := ate.Port(t, "port1")
	i1 := top.AddInterface(atePort1.Name).WithPort(p1)
	i1.IPv4().
		WithAddress(atePort1.IPv4CIDR()).
		WithDefaultGateway(dutPort1.IPv4)

	p2 := ate.Port(t, "port2")
	i2 := top.AddInterface(atePort2.Name).WithPort(p2)
	i2.IPv4().
		WithAddress(atePort2.IPv4CIDR()).
		WithDefaultGateway(dutPort2.IPv4)

	return top
}

// configureDeviceId configures p4rt device-id on the DUT.
func configureDeviceId(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice) {
	component := telemetry.Component{}
	component.IntegratedCircuit = &telemetry.Component_IntegratedCircuit{}
	component.Name = ygot.String(*p4rtNodeName)
	component.IntegratedCircuit.NodeId = ygot.Uint64(deviceID)
	dut.Config().Component(*p4rtNodeName).Replace(t, &component)
}

// configurePortId configures p4rt port-id on the DUT.
func configurePortId(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice) {
	ports := sortPorts(dut.Ports())
	for i, port := range ports {
		dut.Config().Interface(port.Name()).Id().Replace(t, uint32(i)+portID)
	}
}

// setupP4RTClient sends client arbitration message for both leader and follower clients,
// then sends setforwordingpipelineconfig with leader client.
func setupP4RTClient(ctx context.Context, args *testArgs) error {
	// Setup p4rt-client stream parameters
	streamParameter := p4rt_client.P4RTStreamParameters{
		Name:        streamName,
		DeviceId:    deviceID,
		ElectionIdH: uint64(0),
		ElectionIdL: electionID,
	}

	// Send ClientArbitration message on both p4rt leader and follower clients.
	clients := []*p4rt_client.P4RTClient{args.leader, args.follower}
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
				return errors.New("Errors seen when sending ClientArbitration message.")
			}
			if _, _, arbErr := client.StreamChannelGetArbitrationResp(&streamName, 1); arbErr != nil {
				return errors.New("Errors seen in ClientArbitration response.")
			}
		}
	}

	// Load p4info file.
	p4Info, err := utils.P4InfoLoad(p4InfoFile)
	if err != nil {
		return errors.New("Errors seen when loading p4info file.")
	}

	// Send SetForwardingPipelineConfig for p4rt leader client.
	if err := args.leader.SetForwardingPipelineConfig(&p4_v1.SetForwardingPipelineConfigRequest{
		DeviceId:   deviceID,
		ElectionId: &p4_v1.Uint128{High: uint64(0), Low: electionID},
		Action:     p4_v1.SetForwardingPipelineConfigRequest_VERIFY_AND_COMMIT,
		Config: &p4_v1.ForwardingPipelineConfig{
			P4Info: &p4Info,
			Cookie: &p4_v1.ForwardingPipelineConfig_Cookie{
				Cookie: 159,
			},
		},
	}); err != nil {
		return errors.New("Errors seen when sending SetForwardingPipelineConfig.")
	}
	return nil
}

// getGDPParameter returns GDP related parameters for testPacketOut testcase.
func getGDPParameter(t *testing.T) PacketIO {
	return &GDPPacketIO{
		IngressPort: fmt.Sprint(portID),
	}
}

func TestPacketOut(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ctx := context.Background()

	configureDUT(t, dut)

	// Configure the ATE
	ate := ondatra.ATE(t, "ate")
	top := configureATE(t, ate)
	top.Push(t).StartProtocols(t)

	// Configure P4RT device-id and port-id on the DUT
	configureDeviceId(ctx, t, dut)
	configurePortId(ctx, t, dut)

	leader := p4rt_client.P4RTClient{}
	if err := leader.P4rtClientSet(dut.RawAPIs().P4RT(t)); err != nil {
		t.Fatalf("Could not initialize p4rt client: %v", err)
	}

	follower := p4rt_client.P4RTClient{}
	if err := follower.P4rtClientSet(dut.RawAPIs().P4RT(t)); err != nil {
		t.Fatalf("Could not initialize p4rt client: %v", err)
	}

	args := &testArgs{
		ctx:      ctx,
		leader:   &leader,
		follower: &follower,
		dut:      dut,
		ate:      ate,
		top:      top,
	}

	if err := setupP4RTClient(ctx, args); err != nil {
		t.Fatalf("Could not setup p4rt client: %v", err)
	}

	args.packetIO = getGDPParameter(t)
	testPacketOut(ctx, t, args)
}

type GDPPacketIO struct {
	PacketIO
	IngressPort string
}

// packetGDPRequestGet generates PacketOut payload for GDP packets.
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
func (gdp *GDPPacketIO) GetTableEntry(delete bool) []*wbb.ACLWbbIngressTableEntryInfo {
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
