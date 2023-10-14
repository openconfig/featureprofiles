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

package lldp_packetout_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sort"
	"testing"
	"time"

	"flag"

	"github.com/cisco-open/go-p4/p4rt_client"
	"github.com/cisco-open/go-p4/utils"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/feature/experimental/p4rt/internal/p4rtutils"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
	p4pb "github.com/p4lang/p4runtime/go/p4/v1"
)

const (
	ipv4PrefixLen = 30
	packetCount   = 100
)

var (
	p4InfoFile                       = flag.String("p4info_file_location", "../../wbb.p4info.pb.txt", "Path to the p4info file.")
	streamName                       = "p4rt"
	lldpInLayers layers.EthernetType = 0x88cc
	deviceID                         = *ygot.Uint64(1)
	portID                           = *ygot.Uint32(10)
	electionID                       = *ygot.Uint64(100)
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "dutPort1",
		IPv4:    "192.0.2.1",
		IPv4Len: ipv4PrefixLen,
	}

	atePort1 = attrs.Attributes{
		Name:    "atePort1",
		MAC:     "02:11:01:00:00:01",
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
		MAC:     "02:12:01:00:00:01",
		IPv4:    "192.0.2.6",
		IPv4Len: ipv4PrefixLen,
	}
)

type PacketIO interface {
	GetTableEntry(delete bool) []*p4rtutils.ACLWbbIngressTableEntryInfo
	GetPacketOut(portID uint32, submitIngress bool) []*p4pb.PacketOut
}

type testArgs struct {
	ctx      context.Context
	leader   *p4rt_client.P4RTClient
	follower *p4rt_client.P4RTClient
	dut      *ondatra.DUTDevice
	ate      *ondatra.ATEDevice
	top      gosnappi.Config
	packetIO PacketIO
}

// programmTableEntry programs or deletes p4rt table entry based on delete flag.
func programmTableEntry(ctx context.Context, t *testing.T, client *p4rt_client.P4RTClient, packetIO PacketIO, delete bool) error {
	t.Helper()
	err := client.Write(&p4pb.WriteRequest{
		DeviceId:   deviceID,
		ElectionId: &p4pb.Uint128{High: uint64(0), Low: electionID},
		Updates: p4rtutils.ACLWbbIngressTableEntryGet(
			packetIO.GetTableEntry(delete),
		),
		Atomicity: p4pb.WriteRequest_CONTINUE_ON_ERROR,
	})
	if err != nil {
		return err
	}
	return nil
}

// sendPackets sends out packets via PacketOut message in StreamChannel.
func sendPackets(t *testing.T, client *p4rt_client.P4RTClient, packets []*p4pb.PacketOut, packetCount int) {
	count := packetCount / len(packets)
	for _, packet := range packets {
		for i := 0; i < count; i++ {
			if err := client.StreamChannelSendMsg(
				&streamName, &p4pb.StreamMessageRequest{
					Update: &p4pb.StreamMessageRequest_Packet{
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
	}}

	for _, test := range packetOutTests {
		t.Run(test.desc, func(t *testing.T) {
			// Check initial packet counters
			port := sortPorts(args.ate.Ports())[0].ID()
			counter0 := gnmi.Get(t, args.ate.OTG(), gnmi.OTG().Port(port).Counters().InFrames().State())

			packets := args.packetIO.GetPacketOut(portID, false)
			sendPackets(t, test.client, packets, packetCount)

			// Wait for ate stats to be populated
			time.Sleep(60 * time.Second)

			// Check packet counters after packet out
			counter1 := gnmi.Get(t, args.ate.OTG(), gnmi.OTG().Port(port).Counters().InFrames().State())

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
func configInterfaceDUT(i *oc.Interface, a *attrs.Attributes, dut *ondatra.DUTDevice) *oc.Interface {
	i.Description = ygot.String(a.Desc)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	if deviations.InterfaceEnabled(dut) {
		i.Enabled = ygot.Bool(true)
	}

	s := i.GetOrCreateSubinterface(0)
	s4 := s.GetOrCreateIpv4()
	if deviations.InterfaceEnabled(dut) {
		s4.Enabled = ygot.Bool(true)
	}
	s4a := s4.GetOrCreateAddress(a.IPv4)
	s4a.PrefixLength = ygot.Uint8(ipv4PrefixLen)

	return i
}

// configureDUT configures port1 and port2 on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	d := gnmi.OC()

	p1 := dut.Port(t, "port1")
	i1 := &oc.Interface{Name: ygot.String(p1.Name()), Id: ygot.Uint32(portID)}
	gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), configInterfaceDUT(i1, &dutPort1, dut))

	p2 := dut.Port(t, "port2")
	i2 := &oc.Interface{Name: ygot.String(p2.Name()), Id: ygot.Uint32(portID + 1)}
	gnmi.Replace(t, dut, d.Interface(p2.Name()).Config(), configInterfaceDUT(i2, &dutPort2, dut))

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, p1)
		fptest.SetPortSpeed(t, p2)
	}

	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, p1.Name(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, p2.Name(), deviations.DefaultNetworkInstance(dut), 0)
	}
}

// configureATE configures port1 and port2 on the ATE.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	top := gosnappi.NewConfig()

	p1 := ate.Port(t, "port1")
	atePort1.AddToOTG(top, p1, &dutPort1)

	p2 := ate.Port(t, "port2")
	atePort2.AddToOTG(top, p2, &dutPort2)

	return top
}

// configureDeviceID configures p4rt device-id on the DUT.
func configureDeviceID(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice) {
	nodes := p4rtutils.P4RTNodesByPort(t, dut)
	p4rtNode, ok := nodes["port1"]
	if !ok {
		t.Fatal("Couldn't find P4RT Node for port: port1")
	}
	t.Logf("Configuring P4RT Node: %s", p4rtNode)
	component := oc.Component{}
	component.IntegratedCircuit = &oc.Component_IntegratedCircuit{}
	component.Name = ygot.String(p4rtNode)
	component.IntegratedCircuit.NodeId = ygot.Uint64(deviceID)
	gnmi.Replace(t, dut, gnmi.OC().Component(p4rtNode).Config(), &component)
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
			if err := client.StreamChannelSendMsg(&streamName, &p4pb.StreamMessageRequest{
				Update: &p4pb.StreamMessageRequest_Arbitration{
					Arbitration: &p4pb.MasterArbitrationUpdate{
						DeviceId: streamParameter.DeviceId,
						ElectionId: &p4pb.Uint128{
							High: streamParameter.ElectionIdH,
							Low:  streamParameter.ElectionIdL - uint64(index),
						},
					},
				},
			}); err != nil {
				return errors.New("errors seen when sending ClientArbitration message")
			}
			if _, _, arbErr := client.StreamChannelGetArbitrationResp(&streamName, 1); arbErr != nil {
				if err := p4rtutils.StreamTermErr(client.StreamTermErr); err != nil {
					return err
				}
				return errors.New("errors seen in ClientArbitration response")
			}
		}
	}

	// Load p4info file.
	p4Info, err := utils.P4InfoLoad(p4InfoFile)
	if err != nil {
		return errors.New("errors seen when loading p4info file")
	}

	// Send SetForwardingPipelineConfig for p4rt leader client.
	if err := args.leader.SetForwardingPipelineConfig(&p4pb.SetForwardingPipelineConfigRequest{
		DeviceId:   deviceID,
		ElectionId: &p4pb.Uint128{High: uint64(0), Low: electionID},
		Action:     p4pb.SetForwardingPipelineConfigRequest_VERIFY_AND_COMMIT,
		Config: &p4pb.ForwardingPipelineConfig{
			P4Info: p4Info,
			Cookie: &p4pb.ForwardingPipelineConfig_Cookie{
				Cookie: 159,
			},
		},
	}); err != nil {
		return errors.New("errors seen when sending SetForwardingPipelineConfig")
	}
	return nil
}

// getLLDPParameter returns LLDP related parameters for testPacketOut testcase.
func getLLDPParameter(t *testing.T) PacketIO {
	return &LLDPPacketIO{
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
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)

	// Configure P4RT device-id
	configureDeviceID(ctx, t, dut)

	t.Logf("Disable LLDP config")
	gnmi.Replace(t, dut, gnmi.OC().Lldp().Enabled().Config(), false)

	leader := p4rt_client.NewP4RTClient(&p4rt_client.P4RTClientParameters{})
	if err := leader.P4rtClientSet(dut.RawAPIs().P4RT(t)); err != nil {
		t.Fatalf("Could not initialize p4rt client: %v", err)
	}

	follower := p4rt_client.NewP4RTClient(&p4rt_client.P4RTClientParameters{})
	if err := follower.P4rtClientSet(dut.RawAPIs().P4RT(t)); err != nil {
		t.Fatalf("Could not initialize p4rt client: %v", err)
	}

	args := &testArgs{
		ctx:      ctx,
		leader:   leader,
		follower: follower,
		dut:      dut,
		ate:      ate,
		top:      top,
	}

	if err := setupP4RTClient(ctx, args); err != nil {
		t.Fatalf("Could not setup p4rt client: %v", err)
	}

	args.packetIO = getLLDPParameter(t)
	testPacketOut(ctx, t, args)
}

type LLDPPacketIO struct {
	PacketIO
	IngressPort string
}

// packetLLDPRequestGet generates PacketOut payload for LLDP packets.
func packetLLDPRequestGet() []byte {
	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{
		FixLengths:       true,
		ComputeChecksums: true,
	}
	pktEth := &layers.Ethernet{
		SrcMAC: net.HardwareAddr{0x00, 0xAA, 0x00, 0xAA, 0x00, 0xAA},
		// LLDP MAC is 01:80:C2:00:00:0E
		DstMAC:       net.HardwareAddr{0x01, 0x80, 0xC2, 0x00, 0x00, 0x0E},
		EthernetType: lldpInLayers,
	}

	pktLLDP := &layers.LinkLayerDiscovery{
		ChassisID: layers.LLDPChassisID{
			Subtype: layers.LLDPChassisIDSubTypeMACAddr,
			ID:      []byte{0x01, 0x01, 0x01, 0x01, 0x01, 0x01},
		},
		PortID: layers.LLDPPortID{
			Subtype: layers.LLDPPortIDSubtypeIfaceName,
			ID:      []byte("port1"),
		},
		TTL: 100,
	}

	gopacket.SerializeLayers(buf, opts, pktEth, pktLLDP)
	return buf.Bytes()
}

// GetTableEntry creates wbb acl entry related to LLDP.
func (lldp *LLDPPacketIO) GetTableEntry(delete bool) []*p4rtutils.ACLWbbIngressTableEntryInfo {
	actionType := p4pb.Update_INSERT
	if delete {
		actionType = p4pb.Update_DELETE
	}
	return []*p4rtutils.ACLWbbIngressTableEntryInfo{{
		Type:          actionType,
		EtherType:     0x88cc,
		EtherTypeMask: 0xFFFF,
		Priority:      1,
	}}
}

// GetPacketOut generates PacketOut message with payload as LLDP.
func (lldp *LLDPPacketIO) GetPacketOut(portID uint32, submitIngress bool) []*p4pb.PacketOut {
	packets := []*p4pb.PacketOut{}
	packet := &p4pb.PacketOut{
		Payload: packetLLDPRequestGet(),
		Metadata: []*p4pb.PacketMetadata{
			{
				MetadataId: uint32(1), // "egress_port"
				Value:      []byte(fmt.Sprint(portID)),
			},
		},
	}
	if submitIngress {
		packet.Metadata = append(packet.Metadata,
			&p4pb.PacketMetadata{
				MetadataId: uint32(2), // "submit_to_ingress"
				Value:      []byte{1},
			})
	}
	packets = append(packets, packet)
	return packets
}
