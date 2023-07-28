// Copyright 2023 Google LLC
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

package performance_test

import (
	"errors"
	"fmt"
	"net"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"flag"

	"github.com/cisco-open/go-p4/p4rt_client"
	"github.com/cisco-open/go-p4/utils"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/openconfig/featureprofiles/feature/experimental/p4rt/internal/p4rtutils"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
	p4v1pb "github.com/p4lang/p4runtime/go/p4/v1"
)

const (
	deviceID            = uint64(1)
	portID              = uint32(10)
	electionID          = uint64(100)
	metadataIngressPort = uint32(1)
	metadataEgressPort  = uint32(2)
	duration            = uint32(20) // Sleep duration after starting ATE traffic.
	gdpBitRate          = uint64(200000)
	lldpBitRate         = uint64(100000)
	trPacketRate        = uint64(324)
	packetInPktsize     = uint32(300)
	ipv4PrefixLen       = uint8(30)
	ipv6PrefixLen       = uint8(126)
	packetOutWait       = time.Duration(77 * time.Second) // Wait for the ATE traffic start, so both packetin and packetout hits DUT simultaneously.
	packetCount         = int(2000)
)

var (
	p4InfoFile                              = flag.String("p4info_file_location", "../../wbb.p4info.pb.txt", "Path to the p4info file.")
	streamName                              = "p4rt"
	pktInSrcMAC                             = "00:02:00:03:00:04"
	pktOutSrcMAC                            = "00:01:00:02:00:03"
	lldpInDstMAC                            = "01:80:c2:00:00:0e"
	lldpOutDstMAC                           = "01:80:c3:00:00:0e"
	gdpInDstMAC                             = "00:0a:da:f0:f0:f0"
	gdpOutDstMAC                            = "00:0a:db:f0:f0:f0"
	tracerouteOutDstMAC                     = "02:F6:65:64:00:08"
	ttl1                                    = uint8(1)
	hopLimit1                               = uint8(1)
	gdpEthType          layers.EthernetType = 0x6007
	lldpEthType         layers.EthernetType = layers.EthernetTypeLinkLayerDiscovery
	tracerouteEthType   layers.EthernetType = layers.EthernetTypeIPv4

	dutPort1 = attrs.Attributes{
		Desc:    "dutPort1",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::1",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}

	atePort1 = attrs.Attributes{
		Name:    "atePort1",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::2",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}

	dutPort2 = attrs.Attributes{
		Desc:    "dutPort2",
		IPv4:    "192.0.2.5",
		IPv6:    "2001:db8::5",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}

	atePort2 = attrs.Attributes{
		Name:    "atePort2",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::6",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
)

type testArgs struct {
	leader       *p4rt_client.P4RTClient
	dut          *ondatra.DUTDevice
	ate          *ondatra.ATEDevice
	top          *ondatra.ATETopology
	gdpPacketIO  PacketIO
	lldpPacketIO PacketIO
	trPacketIO   PacketIO
}

type PacketIOPacket struct {
	TTL            *uint8
	SrcMAC, DstMAC *string
	EthernetType   *uint32
	HopLimit       *uint8
}

type PacketIO interface {
	GetPacketTemplate() *PacketIOPacket
	GetEgressPort() string
	GetIngressPort() string
}

type LLDPPacketIO struct {
	PacketIOPacket
	IngressPort string
}

type GDPPacketIO struct {
	PacketIOPacket
	IngressPort string
}

type TraceroutePacketIO struct {
	PacketIOPacket
	IngressPort string
	EgressPort  string
}

// programTableEntry programs or deletes p4rt table entry based on delete flag.
func programTableEntry(client *p4rt_client.P4RTClient, delete bool) error {
	updateType := p4v1pb.Update_INSERT
	if delete {
		updateType = p4v1pb.Update_DELETE
	}
	return client.Write(&p4v1pb.WriteRequest{
		DeviceId:   deviceID,
		ElectionId: &p4v1pb.Uint128{High: uint64(0), Low: electionID},
		Updates: p4rtutils.ACLWbbIngressTableEntryGet(
			newTableEntry(updateType),
		),
		Atomicity: p4v1pb.WriteRequest_CONTINUE_ON_ERROR,
	})
}

// testTraffic sends ATE traffic, stop and collect total packet tx from ATE source port.
func testTraffic(t *testing.T, ate *ondatra.ATEDevice, flows []*ondatra.Flow, srcEndPoint *ondatra.Interface, duration time.Duration) int {
	t.Helper()
	for _, flow := range flows {
		flow.WithSrcEndpoints(srcEndPoint).WithDstEndpoints(srcEndPoint)
	}
	ate.Traffic().Start(t, flows...)
	time.Sleep(duration)
	ate.Traffic().Stop(t)

	outPkts := gnmi.GetAll(t, ate, gnmi.OC().FlowAny().Counters().OutPkts().State())
	total := 0
	for _, count := range outPkts {
		total += int(count)
	}
	return total
}

// sendPackets GDP/LLDP/Traceroute sends out packets via PacketOut message in StreamChannel.
func sendPackets(t *testing.T, client *p4rt_client.P4RTClient, packet *p4v1pb.PacketOut, packetCount int, delay time.Duration) {
	for i := 0; i < packetCount; i++ {
		if err := client.StreamChannelSendMsg(
			&streamName, &p4v1pb.StreamMessageRequest{
				Update: &p4v1pb.StreamMessageRequest_Packet{
					Packet: packet,
				},
			}); err != nil {
			t.Errorf("There is error seen in Packet Out. %v, %s", err, err)
		}
		time.Sleep(delay)
	}
}

// decodeIPPacketTTL decodes L2 header in the packet and returns TTL, packetData[14:0] to remove first 14 bytes of Ethernet header.
func decodeIPPacketTTL(packetData []byte) (uint8, error) {
	packet := gopacket.NewPacket(packetData, layers.LayerTypeEthernet, gopacket.Default)
	etherHeader := packet.Layer(layers.LayerTypeEthernet)
	if etherHeader == nil {
		return 0, errors.New("not an Ethernet packet")
	}
	header, ok := etherHeader.(*layers.Ethernet)
	if !ok || header.EthernetType != tracerouteEthType {
		return 0, errors.New("not a Traceroute packet")
	}
	packet1 := gopacket.NewPacket(packetData[14:], layers.LayerTypeIPv4, gopacket.Default)
	if ipv4 := packet1.Layer(layers.LayerTypeIPv4); ipv4 != nil {
		return ipv4.(*layers.IPv4).TTL, nil
	}
	return 0, errors.New("not an Traceroute IPv4 packet")
}

// decodeL2Packet decodes L2 header in the packet and returns source MAC, destination MAC and ethernet type.
func decodeL2Packet(packetData []byte) (string, string, layers.EthernetType) {
	packet := gopacket.NewPacket(packetData, layers.LayerTypeEthernet, gopacket.Default)
	etherHeader := packet.Layer(layers.LayerTypeEthernet)
	if etherHeader != nil {
		header, ok := etherHeader.(*layers.Ethernet)
		if header.EthernetType == lldpEthType || header.EthernetType == gdpEthType || header.EthernetType == tracerouteEthType {
			if ok {
				return header.SrcMAC.String(), header.DstMAC.String(), header.EthernetType
			}
		}
	}
	return "", "", layers.EthernetType(0)
}

// testPktinPktout sends out packetout and packetin traffic together simultaneously.
func testPktInPktOut(t *testing.T, args *testArgs) {

	// Insert wbb acl entry on the DUT
	if err := programTableEntry(args.leader, false); err != nil {
		t.Fatalf("There is error when programming entry")
	}
	// Delete wbb acl entry on the device
	defer programTableEntry(args.leader, true)

	t.Run("PacketOut and Packetin traffic tests", func(t *testing.T) {
		// Check initial packet counters
		port := sortPorts(args.ate.Ports())[0].Name()
		counter0 := gnmi.Get(t, args.ate, gnmi.OC().Interface(port).Counters().InPkts().State())

		packets, err := newPacketOut(portID, false)
		if err != nil {
			t.Fatalf("Unexpected error creating packet out packets: %v", err)
		}
		srcEndPoint := args.top.Interfaces()[atePort1.Name]
		streamChan := args.leader.StreamChannelGet(&streamName)

		qSize := 12000
		streamChan.SetArbQSize(qSize)
		qSizeRead := streamChan.GetArbQSize()
		if qSize != qSizeRead {
			t.Errorf("Stream '%s' expecting Arbitration qSize(%d) Got (%d)",
				streamName, qSize, qSizeRead)
		}

		streamChan.SetPacketQSize(qSize)
		qSizeRead = streamChan.GetPacketQSize()
		if qSize != qSizeRead {
			t.Errorf("Stream '%s' expecting Packet qSize(%d) Got (%d)",
				streamName, qSize, qSizeRead)
		}
		// Create the flows for Packetin.
		flows := newTrafficFlow(args, args.ate)
		pktIn := 0
		// Run Packetin and packetout traffic in parallel.
		var wg sync.WaitGroup
		wg.Add(4)

		go func() {
			defer wg.Done()
			pktIn = testTraffic(t, args.ate, flows, srcEndPoint, 20*time.Second)
			t.Logf("Total Packetin packets sent from ATE %v", pktIn)
		}()

		go func() {
			defer wg.Done()
			time.Sleep(packetOutWait)
			// GDP packetout with 2.6ms timer.
			sendPackets(t, args.leader, packets[0], packetCount, 2600*time.Microsecond)
		}()

		go func() {
			defer wg.Done()
			time.Sleep(packetOutWait)
			// Traceroute packetout with 3.1ms timer.
			sendPackets(t, args.leader, packets[1], packetCount, 3100*time.Microsecond)
		}()

		go func() {
			defer wg.Done()
			time.Sleep(packetOutWait)
			// LLDP packetout with 5.1ms timer.
			sendPackets(t, args.leader, packets[2], packetCount, 5100*time.Microsecond)
		}()

		wg.Wait() // Wait for all four goroutines to finish before exiting.

		// Wait for the packetOut requests to be completed on the server side
		time.Sleep(1 * time.Minute)

		// Check packet counters after packet out
		counter1 := gnmi.Get(t, args.ate, gnmi.OC().Interface(port).Counters().InPkts().State())

		// Verify Packetout stats to check P4RT stream
		t.Logf("Received %v packetout on ATE port %s", counter1-counter0, port)

		if (counter1 - counter0) < uint64(float32(3*packetCount)*0.95) {
			t.Errorf("Number of Packetout packets, got: %d, want: %d", counter1-counter0, (3 * packetCount))
		}
		_, packetinPackets, err := args.leader.StreamChannelGetPackets(&streamName, uint64(pktIn), 30*time.Second)
		if err != nil {
			t.Errorf("Unexpected error on StreamChannelGetPackets: %v", err)
		}

		t.Logf("Start to decode packetin and compare with expected packets.")
		wantgdpPacket := args.gdpPacketIO.GetPacketTemplate()
		wantlldpPacket := args.lldpPacketIO.GetPacketTemplate()
		wanttrPacket := args.trPacketIO.GetPacketTemplate()

		gdpIncount := 0
		lldpIncount := 0
		trIncount := 0

		for _, packet := range packetinPackets {
			if packet != nil {
				srcMAC, dstMac, etherType := decodeL2Packet(packet.Pkt.GetPayload())
				if !strings.EqualFold(srcMAC, pktInSrcMAC) {
					continue
				}
				ttl, _ := decodeIPPacketTTL(packet.Pkt.GetPayload())

				metaData := packet.Pkt.GetMetadata()
				if wantgdpPacket.EthernetType != nil {
					if etherType == layers.EthernetType(*wantgdpPacket.EthernetType) {
						if dstMac == *wantgdpPacket.DstMAC {
							for _, data := range metaData {
								switch data.GetMetadataId() {
								case metadataIngressPort:
									if string(data.GetValue()) != args.gdpPacketIO.GetIngressPort() {
										t.Fatalf("Ingress Port Id is not matching expectation for GDP.")
									}
								case metadataEgressPort:
									portData := args.gdpPacketIO.GetEgressPort()
									if string(data.GetValue()) != portData {
										t.Fatalf("Egress Port Id is not matching expectation for GDP.")
									}
									gdpIncount++
								}
							}
						}
					}
				}
				if wantlldpPacket.EthernetType != nil {
					if etherType == layers.EthernetType(*wantlldpPacket.EthernetType) {
						if dstMac == *wantlldpPacket.DstMAC {
							for _, data := range metaData {
								switch data.GetMetadataId() {
								case metadataIngressPort:
									if string(data.GetValue()) != args.lldpPacketIO.GetIngressPort() {
										t.Fatalf("Ingress Port Id is not matching expectation for LLDP.")
									}
								case metadataEgressPort:
									portData := args.lldpPacketIO.GetEgressPort()
									if string(data.GetValue()) != portData {
										t.Fatalf("Egress Port Id is not matching expectation for LLDP.")
									}
									lldpIncount++
								}
							}
						}
					}
				}
				if wanttrPacket.TTL != nil {
					if ttl == 1 {
						//Metadata comparision
						if metaData := packet.Pkt.GetMetadata(); metaData != nil {
							if got := metaData[0].GetMetadataId(); got == metadataIngressPort {
								if gotPortID := string(metaData[0].GetValue()); gotPortID != args.trPacketIO.GetIngressPort() {
									t.Fatalf("Ingress Port Id mismatch: want %s, got %s", args.trPacketIO.GetIngressPort(), gotPortID)
								}
							} else {
								t.Fatalf("Metadata ingress port mismatch: want %d, got %d", metadataIngressPort, got)
							}
							if got := metaData[1].GetMetadataId(); got == metadataEgressPort {
								if gotPortID := string(metaData[1].GetValue()); gotPortID != args.trPacketIO.GetEgressPort() {
									t.Fatalf("Egress Port Id mismatch: want %s, got %s", args.trPacketIO.GetEgressPort(), gotPortID)
								}
							} else {
								t.Fatalf("Metadata egress port mismatch: want %d, got %d", metadataEgressPort, got)
							}
						} else {
							t.Fatalf("Packet missing metadata information for traceroute")
						}
						trIncount++
					}
				}
			}
		}
		if got, want := (gdpIncount + lldpIncount + trIncount), pktIn; float32(got) < (0.95 * float32(want)) {
			t.Errorf("Number of PacketIn, got: %d, want: %d", got, want)
		}
	})
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

// configInterfaceDUT configures the interface with the IP Addresses.
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

	s6 := s.GetOrCreateIpv6()
	if deviations.InterfaceEnabled(dut) {
		s6.Enabled = ygot.Bool(true)
	}
	s6.GetOrCreateAddress(a.IPv6).PrefixLength = ygot.Uint8(ipv6PrefixLen)

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
	gnmi.Replace(t, dut, gnmi.OC().Lldp().Enabled().Config(), false)

	if deviations.GRIBIMACOverrideStaticARPStaticRoute(dut) {
		gnmi.Replace(t, dut, gnmi.OC().System().MacAddress().RoutingMac().Config(), tracerouteOutDstMAC)
	}
}

// configureATE configures port1 and port2 on the ATE.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) *ondatra.ATETopology {
	top := ate.Topology().New()

	p1 := ate.Port(t, "port1")
	i1 := top.AddInterface(atePort1.Name).WithPort(p1)
	i1.IPv4().
		WithAddress(atePort1.IPv4CIDR()).
		WithDefaultGateway(dutPort1.IPv4)
	i1.IPv6().
		WithAddress(atePort1.IPv6CIDR()).
		WithDefaultGateway(dutPort1.IPv6)

	p2 := ate.Port(t, "port2")
	i2 := top.AddInterface(atePort2.Name).WithPort(p2)
	i2.IPv4().
		WithAddress(atePort2.IPv4CIDR()).
		WithDefaultGateway(dutPort2.IPv4)
	i2.IPv6().
		WithAddress(atePort2.IPv6CIDR()).
		WithDefaultGateway(dutPort2.IPv6)

	return top
}

// configureDeviceIDs configures p4rt device-id on the DUT.
func configureDeviceID(t *testing.T, dut *ondatra.DUTDevice) {
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

// setupP4RTClient sends client arbitration message for the leader client,
// then sends setforwordingpipelineconfig config.
func setupP4RTClient(args *testArgs) error {
	// Setup p4rt-client stream parameters.
	streamParameter := p4rt_client.P4RTStreamParameters{
		Name:        streamName,
		DeviceId:    deviceID,
		ElectionIdH: uint64(0),
		ElectionIdL: electionID,
	}

	// Send ClientArbitration message from the leader client.
	clients := []*p4rt_client.P4RTClient{args.leader}
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

// gdpParameter returns GDP related parameters for testPacketOut testcase.
func gdpParameter() PacketIO {
	ethType := uint32(gdpEthType)
	return &GDPPacketIO{
		PacketIOPacket: PacketIOPacket{
			SrcMAC:       &pktInSrcMAC,
			DstMAC:       &gdpInDstMAC,
			EthernetType: &ethType,
		},
		IngressPort: fmt.Sprint(portID),
	}
}

// tracerouteParameter returns Traceroute related parameters for testPacketIn testcase.
func tracerouteParameter() PacketIO {
	return &TraceroutePacketIO{
		PacketIOPacket: PacketIOPacket{
			TTL:      &ttl1,
			HopLimit: &hopLimit1,
		},
		IngressPort: fmt.Sprint(portID),
		EgressPort:  fmt.Sprint(portID + 1),
	}
}

// lldpParameter returns LLDP related parameters for testPacketIn testcase.
func lldpParameter() PacketIO {
	ethType := uint32(lldpEthType)
	return &LLDPPacketIO{
		PacketIOPacket: PacketIOPacket{
			SrcMAC:       &pktInSrcMAC,
			DstMAC:       &lldpInDstMAC,
			EthernetType: &ethType,
		},
		IngressPort: fmt.Sprint(portID),
	}
}

func TestP4rtPerformance(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	configureDUT(t, dut)

	// Configure the ATE
	ate := ondatra.ATE(t, "ate")
	top := configureATE(t, ate)
	top.Push(t).StartProtocols(t)

	// Configure P4RT device-id
	configureDeviceID(t, dut)

	leader := p4rt_client.NewP4RTClient(&p4rt_client.P4RTClientParameters{})
	if err := leader.P4rtClientSet(dut.RawAPIs().P4RT().Default(t)); err != nil {
		t.Fatalf("Could not initialize p4rt client: %v", err)
	}

	args := &testArgs{
		leader: leader,
		dut:    dut,
		ate:    ate,
		top:    top,
	}

	if err := setupP4RTClient(args); err != nil {
		t.Fatalf("Could not setup p4rt client: %v", err)
	}
	args.gdpPacketIO = gdpParameter()
	args.lldpPacketIO = lldpParameter()
	args.trPacketIO = tracerouteParameter()
	testPktInPktOut(t, args)
}

// packetGDPRequestGet generates PacketOut payload for GDP packets.
func packetGDPRequestGet() ([]byte, error) {
	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{
		FixLengths:       true,
		ComputeChecksums: true,
	}
	srcMAC, err := net.ParseMAC(pktOutSrcMAC)
	if err != nil {
		return nil, err
	}
	dstMAC, err := net.ParseMAC(gdpOutDstMAC)
	if err != nil {
		return nil, err
	}
	pktEth := &layers.Ethernet{
		SrcMAC:       srcMAC,
		DstMAC:       dstMAC,
		EthernetType: gdpEthType,
	}
	var payload []byte
	payLoadLen := 64
	for i := 0; i < payLoadLen; i++ {
		payload = append(payload, byte(i))
	}
	gopacket.SerializeLayers(buf, opts,
		pktEth, gopacket.Payload(payload),
	)
	return buf.Bytes(), nil
}

// packetTracerouteRequestGet generates PacketOut payload for Traceroute packets.
func packetTracerouteRequestGet(ttl uint8, seq int) ([]byte, error) {
	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{
		FixLengths:       true,
		ComputeChecksums: true,
	}
	var payload []byte
	payLoadLen := 32

	ethType := layers.EthernetTypeIPv4

	srcMAC, err := net.ParseMAC(pktOutSrcMAC)
	if err != nil {
		return nil, err
	}
	dstMAC, err := net.ParseMAC(tracerouteOutDstMAC)
	if err != nil {
		return nil, err
	}
	pktEth := &layers.Ethernet{
		SrcMAC:       srcMAC,
		DstMAC:       dstMAC,
		EthernetType: ethType,
	}

	pktIpv4 := &layers.IPv4{
		Version:  4,
		TTL:      ttl,
		SrcIP:    net.ParseIP(dutPort1.IPv4).To4(),
		DstIP:    net.ParseIP(atePort1.IPv4).To4(),
		Protocol: layers.IPProtocolICMPv4,
		Flags:    layers.IPv4DontFragment,
	}
	pktICMP4 := &layers.ICMPv4{
		TypeCode: layers.CreateICMPv4TypeCode(layers.ICMPv4TypeEchoRequest, 0),
		Seq:      uint16(seq),
	}

	pktIpv6 := &layers.IPv6{
		Version:    6,
		HopLimit:   ttl,
		NextHeader: layers.IPProtocolICMPv6,
		SrcIP:      net.ParseIP(dutPort1.IPv6).To16(),
		DstIP:      net.ParseIP(atePort1.IPv6).To16(),
	}
	pktICMP6 := &layers.ICMPv6{
		TypeCode: layers.CreateICMPv6TypeCode(layers.ICMPv6TypeEchoRequest, 0),
	}
	pktICMP6.SetNetworkLayerForChecksum(pktIpv6)

	for i := 0; i < payLoadLen; i++ {
		payload = append(payload, byte(i))
	}
	if err := gopacket.SerializeLayers(buf, opts,
		pktEth, pktIpv4, pktICMP4, gopacket.Payload(payload),
	); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// packetLLDPRequestGet generates PacketOut payload for LLDP packets.
func packetLLDPRequestGet() ([]byte, error) {
	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{
		FixLengths:       true,
		ComputeChecksums: true,
	}
	srcMAC, err := net.ParseMAC(pktOutSrcMAC)
	if err != nil {
		return nil, err
	}
	dstMAC, err := net.ParseMAC(lldpOutDstMAC)
	if err != nil {
		return nil, err
	}
	pktEth := &layers.Ethernet{
		SrcMAC:       srcMAC,
		DstMAC:       dstMAC,
		EthernetType: lldpEthType,
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
	return buf.Bytes(), nil
}

// newTableEntry creates wbb acl entry related to GDP,LLDP and traceroute.
func newTableEntry(actionType p4v1pb.Update_Type) []*p4rtutils.ACLWbbIngressTableEntryInfo {
	return []*p4rtutils.ACLWbbIngressTableEntryInfo{
		{
			Type:          actionType,
			EtherType:     uint16(gdpEthType),
			EtherTypeMask: 0xFFFF,
			Priority:      1,
		},
		{
			Type:          actionType,
			EtherType:     uint16(lldpEthType),
			EtherTypeMask: 0xFFFF,
			Priority:      1,
		},
		{
			Type:     actionType,
			IsIpv4:   0x1,
			TTL:      0x1,
			TTLMask:  0xFF,
			Priority: 1,
		},
		{
			Type:     actionType,
			IsIpv6:   0x1,
			TTL:      0x1,
			TTLMask:  0xFF,
			Priority: 1,
		},
	}
}

// newPacketOut generates 3 PacketOut messages with payload as GDP, LLDP and, traceroute.
func newPacketOut(portID uint32, submitIngress bool) ([]*p4v1pb.PacketOut, error) {
	p, err := packetGDPRequestGet()
	if err != nil {
		return nil, err
	}
	packet1 := &p4v1pb.PacketOut{
		Payload: p,
		Metadata: []*p4v1pb.PacketMetadata{
			{
				MetadataId: uint32(1), // "egress_port"
				Value:      []byte(fmt.Sprint(portID)),
			},
		},
	}

	p, err = packetTracerouteRequestGet(2, 1)
	if err != nil {
		return nil, err
	}

	packet2 := &p4v1pb.PacketOut{
		Payload: p,
		Metadata: []*p4v1pb.PacketMetadata{
			{
				MetadataId: uint32(1), // "egress_port"
				Value:      []byte("0"),
			},
			{
				MetadataId: uint32(2), // "submit_to_ingress"
				Value:      []byte{1},
			},
			{
				MetadataId: uint32(3), // "unused_pad"
				Value:      []byte{0},
			},
		},
	}

	p, err = packetLLDPRequestGet()
	if err != nil {
		return nil, err
	}
	packet3 := &p4v1pb.PacketOut{
		Payload: p,
		Metadata: []*p4v1pb.PacketMetadata{
			{
				MetadataId: uint32(1), // "egress_port"
				Value:      []byte(fmt.Sprint(portID)),
			},
		},
	}
	return []*p4v1pb.PacketOut{packet1, packet2, packet3}, nil
}

// newTrafficFlow generates ATE traffic flows for LLDP.
func newTrafficFlow(args *testArgs, ate *ondatra.ATEDevice) []*ondatra.Flow {
	ethHeader1 := ondatra.NewEthernetHeader()
	ethHeader1.WithSrcAddress(pktInSrcMAC)
	ethHeader1.WithDstAddress(lldpInDstMAC)
	ethHeader1.WithEtherType(uint32(lldpEthType))

	// flow1 for LLDP traffic.
	flow1 := ate.Traffic().NewFlow("LLDP").WithFrameSize(packetInPktsize).WithFrameRateBPS(lldpBitRate).WithHeaders(ethHeader1)
	flow1.Transmission().WithPatternFixedDuration(duration)

	ethHeader2 := ondatra.NewEthernetHeader()
	ethHeader2.WithSrcAddress(pktInSrcMAC)
	ethHeader2.WithDstAddress(gdpInDstMAC)
	ethHeader2.WithEtherType(uint32(gdpEthType))

	// flow2 for GDP traffic.
	flow2 := ate.Traffic().NewFlow("GDP").WithFrameSize(packetInPktsize).WithFrameRateBPS(gdpBitRate).WithHeaders(ethHeader2)
	flow2.Transmission().WithPatternFixedDuration(duration)

	ethHeader := ondatra.NewEthernetHeader().WithSrcAddress(pktInSrcMAC)
	ipv4Header := ondatra.NewIPv4Header().WithSrcAddress(atePort1.IPv4).WithDstAddress(atePort2.IPv4).WithTTL(ttl1) //ttl=1 is traceroute traffic

	// flow3 for Traceroute traffic.
	flow3 := ate.Traffic().NewFlow("IPv4").WithFrameSize(packetInPktsize).WithFrameRateFPS(trPacketRate).WithHeaders(ethHeader, ipv4Header)
	flow3.Transmission().WithPatternFixedDuration(duration)

	return []*ondatra.Flow{flow1, flow2, flow3}
}

// GetEgressPort returns expected egress port info in PacketIn.
func (lldp *LLDPPacketIO) GetEgressPort() string {
	return string("0")
}

// GetIngressPort return expected ingress port info in PacketIn.
func (lldp *LLDPPacketIO) GetIngressPort() string {
	return lldp.IngressPort
}

// GetEgressPort returns expected egress port info in PacketIn.
func (gdp *GDPPacketIO) GetEgressPort() string {
	return string("0")
}

// GetIngressPort return expected ingress port info in PacketIn.
func (gdp *GDPPacketIO) GetIngressPort() string {
	return gdp.IngressPort
}

// GetEgressPort returns expected egress port info in Packetin.
func (traceroute *TraceroutePacketIO) GetEgressPort() string {
	return traceroute.EgressPort
}

// GetIngressPort return expected ingress port info in Packetin.
func (traceroute *TraceroutePacketIO) GetIngressPort() string {
	return traceroute.IngressPort
}

// GetPacketTemplate returns the expected PacketIOPacket for LLDP traffic type.
func (lldp *LLDPPacketIO) GetPacketTemplate() *PacketIOPacket {
	return &lldp.PacketIOPacket
}

// GetPacketTemplate returns the expected PacketIOPacket for GDP traffic type.
func (gdp *GDPPacketIO) GetPacketTemplate() *PacketIOPacket {
	return &gdp.PacketIOPacket
}

// GetPacketTemplate returns the expected  PacketIOPacket for traceroute type.
func (traceroute *TraceroutePacketIO) GetPacketTemplate() *PacketIOPacket {
	return &traceroute.PacketIOPacket
}
