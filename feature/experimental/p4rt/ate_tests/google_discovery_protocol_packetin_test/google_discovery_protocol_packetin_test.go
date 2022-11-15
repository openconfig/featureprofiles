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

package google_discovery_protocol_packetin_test

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
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
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
	p4_v1 "github.com/p4lang/p4runtime/go/p4/v1"
)

const (
	ipv4PrefixLen = 30
)

var (
	p4InfoFile            = flag.String("p4info_file_location", "../../wbb.p4info.pb.txt", "Path to the p4info file.")
	p4rtNodeName          = flag.String("p4rt_node_name", "0/1/CPU0-NPU1", "component name for P4RT Node")
	gdpSrcMAC             = flag.String("gdp_src_MAC", "00:01:00:02:00:03", "source MAC address for PacketIn")
	streamName            = "p4rt"
	gdpMAC                = "00:0a:da:f0:f0:f0"
	gdpEtherType          = *ygot.Uint32(0x6007)
	deviceID              = *ygot.Uint64(1)
	portID                = *ygot.Uint32(10)
	electionID            = *ygot.Uint64(100)
	METADATA_INGRESS_PORT = *ygot.Uint32(1)
	METADATA_EGRESS_PORT  = *ygot.Uint32(2)
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
	GetPacketTemplate() *PacketIOPacket
	GetTrafficFlow(ate *ondatra.ATEDevice, frameSize uint32, frameRate uint64) []*ondatra.Flow
	GetEgressPort() []string
	GetIngressPort() string
}

type PacketIOPacket struct {
	SrcMAC, DstMAC *string
	EthernetType   *uint32
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

// decodePacket decodes L2 header in the packet and returns destination MAC and ethernet type.
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

// testTraffic sends traffic flow for duration seconds.
func testTraffic(t *testing.T, ate *ondatra.ATEDevice, flows []*ondatra.Flow, srcEndPoint *ondatra.Interface, duration int) {
	t.Helper()
	for _, flow := range flows {
		flow.WithSrcEndpoints(srcEndPoint).WithDstEndpoints(srcEndPoint)
	}
	ate.Traffic().Start(t, flows...)
	time.Sleep(time.Duration(duration) * time.Second)

	ate.Traffic().Stop(t)
}

// fetchPackets reads p4rt packets sent to p4rt client.
func fetchPackets(ctx context.Context, t *testing.T, client *p4rt_client.P4RTClient, expectNumber int) []*p4rt_client.P4RTPacketInfo {
	t.Helper()
	packets := []*p4rt_client.P4RTPacketInfo{}
	for i := 0; i < expectNumber; i++ {
		_, packet, err := client.StreamChannelGetPacket(&streamName, 0)
		if err == io.EOF {
			t.Logf("EOF error is seen in PacketIn.")
			break
		} else if err == nil {
			if packet != nil {
				packets = append(packets, packet)
			}
		} else {
			t.Fatalf("There is error seen when receving packets. %v, %s", err, err)
			break
		}
	}
	return packets
}

// testPacketIn programs p4rt table entry and sends traffic related to GDP,
// then validates packetin message metadata and payload.
func testPacketIn(ctx context.Context, t *testing.T, args *testArgs) {
	leader := args.leader
	follower := args.follower

	// Insert wbb acl entry on the DUT
	if err := programmTableEntry(ctx, t, leader, args.packetIO, false); err != nil {
		t.Fatalf("There is error when programming entry")
	}
	// Delete wbb acl entry on the device
	defer programmTableEntry(ctx, t, leader, args.packetIO, true)

	// Send GDP traffic from ATE
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	testTraffic(t, args.ate, args.packetIO.GetTrafficFlow(args.ate, 300, 2), srcEndPoint, 10)

	packetInTests := []struct {
		desc       string
		client     *p4rt_client.P4RTClient
		expectPass bool
	}{{
		desc:       "PacketIn to Primary Controller",
		client:     leader,
		expectPass: true,
	}, {
		desc:       "PacketIn to Secondary Controller",
		client:     follower,
		expectPass: false,
	}}

	for _, test := range packetInTests {
		t.Run(test.desc, func(t *testing.T) {
			// Extract packets from PacketIn message sent to p4rt client
			packets := fetchPackets(ctx, t, test.client, 40)

			if !test.expectPass {
				if len(packets) > 0 {
					t.Fatalf("Unexpected packets received.")
				}
			} else {
				if len(packets) == 0 {
					t.Fatalf("There are no packets received.")
				}
				t.Logf("Start to decode packet and compare with expected packets.")
				wantPacket := args.packetIO.GetPacketTemplate()
				for _, packet := range packets {
					if packet != nil {
						if wantPacket.DstMAC != nil && wantPacket.EthernetType != nil {
							dstMac, etherType := decodePacket(t, packet.Pkt.GetPayload())
							if dstMac != *wantPacket.DstMAC || etherType != layers.EthernetType(*wantPacket.EthernetType) {
								t.Fatalf("Packet in PacketIn message is not matching wanted packet.")
							}
						}

						metaData := packet.Pkt.GetMetadata()
						for _, data := range metaData {
							if data.GetMetadataId() == METADATA_INGRESS_PORT {
								if string(data.GetValue()) != args.packetIO.GetIngressPort() {
									t.Fatalf("Ingress Port Id is not matching expectation.")
								}
							}
							if data.GetMetadataId() == METADATA_EGRESS_PORT {
								found := false
								for _, portData := range args.packetIO.GetEgressPort() {
									if string(data.GetValue()) == portData {
										found = true
									}
								}
								if !found {
									t.Fatalf("Egress Port Id is not matching expectation.")
								}

							}
						}
					}
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
func configInterfaceDUT(i *oc.Interface, a *attrs.Attributes) *oc.Interface {
	i.Description = ygot.String(a.Desc)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
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
	d := gnmi.OC()

	p1 := dut.Port(t, "port1")
	i1 := &oc.Interface{Name: ygot.String(p1.Name())}
	gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), configInterfaceDUT(i1, &dutPort1))

	p2 := dut.Port(t, "port2")
	i2 := &oc.Interface{Name: ygot.String(p2.Name())}
	gnmi.Replace(t, dut, d.Interface(p2.Name()).Config(), configInterfaceDUT(i2, &dutPort2))
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
	component := oc.Component{}
	component.IntegratedCircuit = &oc.Component_IntegratedCircuit{}
	component.Name = ygot.String(*p4rtNodeName)
	component.IntegratedCircuit.NodeId = ygot.Uint64(deviceID)
	gnmi.Replace(t, dut, gnmi.OC().Component(*p4rtNodeName).Config(), &component)
}

// configurePortId configures p4rt port-id on the DUT.
func configurePortId(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice) {
	ports := sortPorts(dut.Ports())
	for i, port := range ports {
		gnmi.Replace(t, dut, gnmi.OC().Interface(port.Name()).Id().Config(), uint32(i)+portID)
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

// getGDPParameter returns GDP related parameters for testPacketIn testcase.
func getGDPParameter(t *testing.T) PacketIO {
	return &GDPPacketIO{
		PacketIOPacket: PacketIOPacket{
			SrcMAC:       gdpSrcMAC,
			DstMAC:       &gdpMAC,
			EthernetType: &gdpEtherType,
		},
		IngressPort: fmt.Sprint(portID),
	}
}

func TestPacketIn(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ctx := context.Background()

	configureDUT(t, dut)

	// Configure the ATE
	ate := ondatra.ATE(t, "ate")
	top := configureATE(t, ate)
	top.Push(t).StartProtocols(t)

	// Configure P4RT device-id and port-id
	configureDeviceId(ctx, t, dut)
	configurePortId(ctx, t, dut)

	leader := p4rt_client.P4RTClient{}
	if err := leader.P4rtClientSet(dut.RawAPIs().P4RT().Default(t)); err != nil {
		t.Fatalf("Could not initialize p4rt client: %v", err)
	}

	follower := p4rt_client.P4RTClient{}
	if err := follower.P4rtClientSet(dut.RawAPIs().P4RT().Default(t)); err != nil {
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
	testPacketIn(ctx, t, args)
}

type GDPPacketIO struct {
	PacketIOPacket
	IngressPort string
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

// GetPacketTemplate returns expected packets in PacketIn.
func (gdp *GDPPacketIO) GetPacketTemplate() *PacketIOPacket {
	return &gdp.PacketIOPacket
}

// GetTrafficFlow generates ATE traffic flows for GDP.
func (gdp *GDPPacketIO) GetTrafficFlow(ate *ondatra.ATEDevice, frameSize uint32, frameRate uint64) []*ondatra.Flow {
	ethHeader := ondatra.NewEthernetHeader()
	ethHeader.WithSrcAddress(*gdp.SrcMAC)
	ethHeader.WithDstAddress(*gdp.DstMAC)
	ethHeader.WithEtherType(*gdp.EthernetType)

	flow := ate.Traffic().NewFlow("GDP").WithFrameSize(frameSize).WithFrameRateFPS(frameRate).WithHeaders(ethHeader)
	return []*ondatra.Flow{flow}
}

// GetEgressPort returns expected egress port info in PacketIn.
func (gdp *GDPPacketIO) GetEgressPort() []string {
	return []string{"0"}
}

// GetIngressPort return expected ingress port info in PacketIn.
func (gdp *GDPPacketIO) GetIngressPort() string {
	return gdp.IngressPort
}
