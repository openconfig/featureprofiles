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

package google_discovery_protocol_packetout_lag_test

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
	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/featureprofiles/internal/p4rtutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ygot/ygot"
	p4v1pb "github.com/p4lang/p4runtime/go/p4/v1"
)

const (
	ipv4PLen    = 30
	packetCount = 300
)

var (
	p4InfoFile                       = flag.String("p4info_file_location", "../../wbb.p4info.pb.txt", "Path to the p4info file.")
	streamName                       = "p4rt"
	gdpInLayers  layers.EthernetType = 0x6007
	deviceID                         = uint64(1)
	portID                           = uint32(10)
	electionID                       = uint64(100)
	vlanID                           = uint16(4000)
	pktOutDstMAC                     = "02:F6:65:64:00:08"
)

type aggPortData struct {
	dutIPv4    string
	ateIPv4    string
	ateAggName string
	ateAggMAC  string
	atePortMAC string
	aggPortID  uint32
	hasVlan    bool
}

var (
	agg1 = &aggPortData{
		dutIPv4:    "192.0.2.1",
		ateIPv4:    "192.0.2.2",
		ateAggName: "lag1",
		ateAggMAC:  "02:00:01:01:01:01",
		atePortMAC: "02:00:01:01:01:02",
		aggPortID:  10,
		hasVlan:    true,
	}
	agg2 = &aggPortData{
		dutIPv4:    "192.0.2.5",
		ateIPv4:    "192.0.2.6",
		ateAggName: "lag2",
		ateAggMAC:  "02:00:01:01:01:04",
		atePortMAC: "02:00:01:01:01:05",
		aggPortID:  11,
		hasVlan:    false,
	}
)

type PacketIO interface {
	GetTableEntry(delete bool) []*p4rtutils.ACLWbbIngressTableEntryInfo
	GetPacketOut(portID uint32) []*p4v1pb.PacketOut
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
	err := client.Write(&p4v1pb.WriteRequest{
		DeviceId:   deviceID,
		ElectionId: &p4v1pb.Uint128{High: uint64(0), Low: electionID},
		Updates: p4rtutils.ACLWbbIngressTableEntryGet(
			packetIO.GetTableEntry(delete),
		),
		Atomicity: p4v1pb.WriteRequest_CONTINUE_ON_ERROR,
	})
	if err != nil {
		return err
	}
	return nil
}

// sendPackets sends out packets via PacketOut message in StreamChannel.
func sendPackets(t *testing.T, client *p4rt_client.P4RTClient, packets []*p4v1pb.PacketOut, packetCount int) {
	count := packetCount / len(packets)
	for _, packet := range packets {
		for i := 0; i < count; i++ {
			if err := client.StreamChannelSendMsg(
				&streamName, &p4v1pb.StreamMessageRequest{
					Update: &p4v1pb.StreamMessageRequest_Packet{
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
			port1 := sortPorts(args.ate.Ports())[0].ID()
			counter0 := gnmi.Get(t, args.ate.OTG(), gnmi.OTG().Port(port1).Counters().InFrames().State())
			packets := args.packetIO.GetPacketOut(portID)
			sendPackets(t, test.client, packets, packetCount)

			// Wait for ate stats to be populated
			time.Sleep(4 * time.Minute)
			otgutils.LogFlowMetrics(t, args.ate.OTG(), args.top)
			otgutils.LogPortMetrics(t, args.ate.OTG(), args.top)
			// Check packet counters after packet out
			counter1 := gnmi.Get(t, args.ate.OTG(), gnmi.OTG().Port(port1).Counters().InFrames().State())
			// Verify InPkts stats to check P4RT stream
			t.Logf("Received %v packets on ATE port %s", counter1-counter0, port1)

			if test.expectPass {
				if counter1-counter0 < uint64(packetCount*0.95) {
					t.Fatalf("Not all the packets are received.")
				}
			} else {
				if counter1-counter0 > uint64(packetCount*0.10) {
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

// configureDUT configures agg1 and agg2 on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) []string {
	t.Helper()
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	var aggIDs []string
	for aggIdx, a := range []*aggPortData{agg1, agg2} {
		b := &gnmi.SetBatch{}
		d := &oc.Root{}

		aggID := netutil.NextAggregateInterface(t, dut)
		aggIDs = append(aggIDs, aggID)

		agg := d.GetOrCreateInterface(aggID)
		agg.GetOrCreateAggregation().LagType = oc.IfAggregate_AggregationType_STATIC
		agg.Type = oc.IETFInterfaces_InterfaceType_ieee8023adLag
		agg.Description = ygot.String(a.ateAggName)
		if deviations.InterfaceEnabled(dut) {
			agg.Enabled = ygot.Bool(true)
		}
		s := agg.GetOrCreateSubinterface(0)
		s4 := s.GetOrCreateIpv4()
		if deviations.InterfaceEnabled(dut) {
			s4.Enabled = ygot.Bool(true)
		}
		a4 := s4.GetOrCreateAddress(a.dutIPv4)
		a4.PrefixLength = ygot.Uint8(ipv4PLen)

		gnmi.BatchDelete(b, gnmi.OC().Interface(aggID).Aggregation().MinLinks().Config())
		gnmi.BatchReplace(b, gnmi.OC().Interface(aggID).Config(), agg)

		p1 := dut.Port(t, fmt.Sprintf("port%d", (aggIdx*1)+1))
		// p2 := dut.Port(t, fmt.Sprintf("port%d", (aggIdx*2)+2))
		for _, port := range []*ondatra.Port{p1} {
			gnmi.BatchDelete(b, gnmi.OC().Interface(port.Name()).Ethernet().AggregateId().Config())
			i := d.GetOrCreateInterface(port.Name())
			// i := &oc.Interface{Name: ygot.String(p1.Name()), Id: ygot.Uint32(a.aggPortID)}
			// i := d.Interface{Name: ygot.String(port.Name()), Id: ygot.Uint32(a.aggPortID)}
			i.Id = ygot.Uint32(a.aggPortID)
			i.Description = ygot.String(fmt.Sprintf("LAG - Member -%s", port.Name()))
			e := i.GetOrCreateEthernet()
			e.AggregateId = ygot.String(aggID)
			i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
			if a.hasVlan && deviations.P4RTGdpRequiresDot1QSubinterface(dut) {
				s1 := i.GetOrCreateSubinterface(1)
				s1.GetOrCreateVlan().GetOrCreateMatch().GetOrCreateSingleTagged().SetVlanId(vlanID)
				if deviations.NoMixOfTaggedAndUntaggedSubinterfaces(dut) {
					s.GetOrCreateVlan().GetOrCreateMatch().GetOrCreateSingleTagged().SetVlanId(10)
					i.GetOrCreateAggregation().GetOrCreateSwitchedVlan().SetNativeVlan(10)
				}
			}
			if deviations.InterfaceEnabled(dut) {
				i.Enabled = ygot.Bool(true)
			}
			gnmi.BatchReplace(b, gnmi.OC().Interface(port.Name()).Config(), i)
		}

		b.Set(t, dut)
	}
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		for _, aggID := range aggIDs {
			fptest.AssignToNetworkInstance(t, dut, aggID, deviations.DefaultNetworkInstance(dut), 0)
		}
	}
	// Wait for LAG interfaces to be UP
	for _, aggID := range aggIDs {
		gnmi.Await(t, dut, gnmi.OC().Interface(aggID).AdminStatus().State(), 60*time.Second, oc.Interface_AdminStatus_UP)
	}
	gnmi.Replace(t, dut, gnmi.OC().System().MacAddress().RoutingMac().Config(), pktOutDstMAC)
	return aggIDs
}

// configureATE configures agg1 and agg2 on the ATE.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	t.Helper()
	top := gosnappi.NewConfig()

	for aggIdx, a := range []*aggPortData{agg1, agg2} {
		p1 := ate.Port(t, fmt.Sprintf("port%d", (aggIdx*1)+1))
		// p2 := ate.Port(t, fmt.Sprintf("port%d", (aggIdx*2)+2))
		top.Ports().Add().SetName(p1.ID())
		agg := top.Lags().Add().SetName(a.ateAggName)
		agg.Protocol().Static().SetLagId(uint32(aggIdx + 1))

		lagDev := top.Devices().Add().SetName(agg.Name() + ".Dev")
		lagEth := lagDev.Ethernets().Add().SetName(agg.Name() + ".Eth").SetMac(a.ateAggMAC)
		lagEth.Connection().SetLagName(agg.Name())
		lagEth.Ipv4Addresses().Add().SetName(agg.Name() + ".IPv4").SetAddress(a.ateIPv4).SetGateway(a.dutIPv4).SetPrefix(ipv4PLen)
		agg.Ports().Add().SetPortName(p1.ID()).Ethernet().SetMac(a.atePortMAC).SetName(a.ateAggName + ".1")
		// agg.Ports().Add().SetPortName(p2.ID()).Ethernet().SetMac(a.atePort2MAC).SetName(a.ateAggName + ".2")
	}
	return top
}

// configureDeviceIDs configures p4rt device-id on the DUT.
func configureDeviceID(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice) {
	nodes := p4rtutils.P4RTNodesByPort(t, dut)
	p4rtNode, ok := nodes["port1"]
	if !ok {
		t.Fatal("Couldn't find P4RT Node for port: port1")
	}
	t.Logf("Configuring P4RT Node: %s", p4rtNode)
	c := oc.Component{}
	c.Name = ygot.String(p4rtNode)
	c.IntegratedCircuit = &oc.Component_IntegratedCircuit{}
	c.IntegratedCircuit.NodeId = ygot.Uint64(deviceID)
	gnmi.Replace(t, dut, gnmi.OC().Component(p4rtNode).Config(), &c)
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
			if err := client.StreamChannelSendMsg(&streamName, &p4v1pb.StreamMessageRequest{
				Update: &p4v1pb.StreamMessageRequest_Arbitration{
					Arbitration: &p4v1pb.MasterArbitrationUpdate{
						DeviceId: streamParameter.DeviceId,
						ElectionId: &p4v1pb.Uint128{
							High: streamParameter.ElectionIdH,
							Low:  streamParameter.ElectionIdL - uint64(index),
						},
					},
				},
			}); err != nil {
				return fmt.Errorf("errors seen when sending ClientArbitration message: %v", err)
			}
			if _, _, arbErr := client.StreamChannelGetArbitrationResp(&streamName, 1); arbErr != nil {
				if err := p4rtutils.StreamTermErr(client.StreamTermErr); err != nil {
					return err
				}
				return fmt.Errorf("errors seen in ClientArbitration response: %v", arbErr)
			}
		}
	}

	// Load p4info file.
	p4Info, err := utils.P4InfoLoad(p4InfoFile)
	if err != nil {
		return errors.New("errors seen when loading p4info file")
	}

	// Send SetForwardingPipelineConfig for p4rt leader client.
	if err := args.leader.SetForwardingPipelineConfig(&p4v1pb.SetForwardingPipelineConfigRequest{
		DeviceId:   deviceID,
		ElectionId: &p4v1pb.Uint128{High: uint64(0), Low: electionID},
		Action:     p4v1pb.SetForwardingPipelineConfigRequest_VERIFY_AND_COMMIT,
		Config: &p4v1pb.ForwardingPipelineConfig{
			P4Info: p4Info,
			Cookie: &p4v1pb.ForwardingPipelineConfig_Cookie{
				Cookie: 159,
			},
		},
	}); err != nil {
		return errors.New("errors seen when sending SetForwardingPipelineConfig")
	}
	return nil
}

// getGDPParameter returns GDP related parameters for testPacketOut testcase.
func getGDPParameter(t *testing.T) PacketIO {
	mac, err := net.ParseMAC(pktOutDstMAC)
	if err != nil {
		t.Fatalf("Could not parse MAC: %v", err)
	}
	return &GDPPacketIO{
		IngressPort: fmt.Sprint(portID),
		DstMAC:      mac,
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

	// Configure P4RT device-id and port-id on the DUT
	configureDeviceID(ctx, t, dut)

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

	args.packetIO = getGDPParameter(t)
	testPacketOut(ctx, t, args)
}

type GDPPacketIO struct {
	PacketIO
	IngressPort string
	DstMAC      net.HardwareAddr
}

// packetGDPRequestGet generates PacketOut payload for GDP packets.
func packetGDPRequestGet(vlan bool) []byte {
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
	if vlan {
		pktEth.EthernetType = layers.EthernetTypeDot1Q
		d1q := &layers.Dot1Q{
			VLANIdentifier: vlanID,
			Type:           gdpInLayers,
		}
		gopacket.SerializeLayers(buf, opts,
			pktEth, d1q, gopacket.Payload(payload),
		)
	} else {
		gopacket.SerializeLayers(buf, opts,
			pktEth, gopacket.Payload(payload),
		)
	}
	return buf.Bytes()
}

func ipPacketToATEPort1(dstMAC net.HardwareAddr) []byte {
	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{
		FixLengths:       true,
		ComputeChecksums: true,
	}
	eth := &layers.Ethernet{
		SrcMAC: net.HardwareAddr{0x00, 0xAA, 0x00, 0xAA, 0x00, 0xAA},
		// GDP MAC is 00:0A:DA:F0:F0:F0
		DstMAC:       dstMAC,
		EthernetType: layers.EthernetTypeIPv4,
	}
	ip := &layers.IPv4{
		SrcIP:    net.ParseIP(agg2.ateIPv4),
		DstIP:    net.ParseIP(agg1.ateIPv4),
		TTL:      2,
		Version:  4,
		Protocol: layers.IPProtocolIPv4,
	}
	tcp := &layers.TCP{
		SrcPort: 10000,
		DstPort: 20000,
		Seq:     11050,
	}

	// Required for checksum computation.
	tcp.SetNetworkLayerForChecksum(ip)

	payload := []byte{}
	payLoadLen := 64
	for i := 0; i < payLoadLen; i++ {
		payload = append(payload, byte(i))
	}
	gopacket.SerializeLayers(buf, opts,
		eth, ip, tcp, gopacket.Payload(payload),
	)
	return buf.Bytes()
}

// GetTableEntry creates wbb acl entry related to GDP.
func (gdp *GDPPacketIO) GetTableEntry(delete bool) []*p4rtutils.ACLWbbIngressTableEntryInfo {
	actionType := p4v1pb.Update_INSERT
	if delete {
		actionType = p4v1pb.Update_DELETE
	}
	return []*p4rtutils.ACLWbbIngressTableEntryInfo{{
		Type:          actionType,
		EtherType:     0x6007,
		EtherTypeMask: 0xFFFF,
		Priority:      1,
	}}
}

// GetPacketOut generates PacketOut message with payload as GDP.
func (gdp *GDPPacketIO) GetPacketOut(portID uint32) []*p4v1pb.PacketOut {
	gdpWithVlan := &p4v1pb.PacketOut{
		Payload: packetGDPRequestGet(true),
		Metadata: []*p4v1pb.PacketMetadata{
			{
				MetadataId: uint32(1), // "egress_port"
				Value:      []byte(fmt.Sprint(portID)),
			},
		},
	}
	gdpWithoutVlan := &p4v1pb.PacketOut{
		Payload: packetGDPRequestGet(false),
		Metadata: []*p4v1pb.PacketMetadata{
			{
				MetadataId: uint32(1), // "egress_port"
				Value:      []byte(fmt.Sprint(portID)),
			},
		},
	}

	nonGDP := &p4v1pb.PacketOut{
		Payload: ipPacketToATEPort1(gdp.DstMAC),
		Metadata: []*p4v1pb.PacketMetadata{
			{
				MetadataId: uint32(2), // "submit_to_ingress"
				Value:      []byte{1},
			},
		},
	}
	return []*p4v1pb.PacketOut{gdpWithoutVlan, gdpWithVlan, nonGDP}
}
