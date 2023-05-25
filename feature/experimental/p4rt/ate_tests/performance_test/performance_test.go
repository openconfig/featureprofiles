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
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"sort"
	"sync"
	"testing"
	"time"

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
	p4_v1 "github.com/p4lang/p4runtime/go/p4/v1"
)

var (
	p4InfoFile                              = flag.String("p4info_file_location", "../../wbb.p4info.pb.txt", "Path to the p4info file.")
	streamName                              = "p4rt"
	gdpInLayers         layers.EthernetType = 0x6007
	lldpInLayers        layers.EthernetType = 0x88cc
	lldpEthertype                           = uint32(0x88cc)
	gdpEtherType                            = uint32(0x6007)
	deviceID                                = *ygot.Uint64(1)
	portID                                  = *ygot.Uint32(10)
	electionID                              = *ygot.Uint64(100)
	metadataIngressPort                     = *ygot.Uint32(1)
	metadataEgressPort                      = *ygot.Uint32(2)

	duration        = uint32(20)
	gdpBitRate      = uint64(200000)
	lldpBitRate     = uint64(100000)
	trPacketRate    = uint64(324)
	packetInPktsize = uint32(300)
	srcMac          = flag.String("srcMac", "00:01:00:02:00:03", "source MAC address for PacketIn")
	lldpDstmac      = flag.String("lldpDstmac", "01:80:c2:00:00:0e", "source MAC address for PacketIn")
	gdpDstmac       = flag.String("gdpDstmac", "00:0a:da:f0:f0:f0", "source MAC address for PacketIn")
	ttl1            = uint8(1)
	hopLimit1       = uint8(1)
	packetOutWait   = uint32(77) // Wait for the ATE traffic start, so both packetin and packetout hits DUT simultaneously.
	ipv4PrefixLen   = uint8(30)
	ipv6PrefixLen   = uint8(126)
	packetCount     = 2000
)

var (
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
	return client.Write(&p4_v1.WriteRequest{
		DeviceId:   deviceID,
		ElectionId: &p4_v1.Uint128{High: uint64(0), Low: electionID},
		Updates: p4rtutils.ACLWbbIngressTableEntryGet(
			getTableEntry(delete),
		),
		Atomicity: p4_v1.WriteRequest_CONTINUE_ON_ERROR,
	})
}

// testTraffic sends ATE traffic, stop and collect total packet tx from ATE source port.
func testTraffic(t *testing.T, ate *ondatra.ATEDevice, flows []*ondatra.Flow, srcEndPoint *ondatra.Interface, duration int) int {
	t.Helper()
	for _, flow := range flows {
		flow.WithSrcEndpoints(srcEndPoint).WithDstEndpoints(srcEndPoint)
	}
	ate.Traffic().Start(t, flows...)
	time.Sleep(time.Duration(duration) * time.Second)
	ate.Traffic().Stop(t)

	outPkts := gnmi.GetAll(t, ate, gnmi.OC().FlowAny().Counters().OutPkts().State())
	total := 0
	for _, count := range outPkts {
		total += int(count)
	}
	return total
}

// sendPackets GDP/LLDP/Traceroute sends out packets via PacketOut message in StreamChannel.
func sendPackets(t *testing.T, client *p4rt_client.P4RTClient, packet *p4_v1.PacketOut, packetCount int, timer float64) {

	for i := 0; i < packetCount; i++ {
		if err := client.StreamChannelSendMsg(
			&streamName, &p4_v1.StreamMessageRequest{
				Update: &p4_v1.StreamMessageRequest_Packet{
					Packet: packet,
				},
			}); err != nil {
			t.Errorf("There is error seen in Packet Out. %v, %s", err, err)
		}
		time.Sleep(time.Duration(timer) * time.Millisecond)
	}
}

// decodeIPPacketTTL decodes L2 header in the packet and returns TTL, packetData[14:0] to remove first 14 bytes of Ethernet header.
func decodeIPPacketTTL(t *testing.T, packetData []byte) uint8 {
	t.Helper()
	packet := gopacket.NewPacket(packetData, layers.LayerTypeEthernet, gopacket.Default)
	etherHeader := packet.Layer(layers.LayerTypeEthernet)
	if etherHeader != nil {
		header := etherHeader.(*layers.Ethernet)
		if header.EthernetType == layers.EthernetType(0x0800) {
			packet1 := gopacket.NewPacket(packetData[14:], layers.LayerTypeIPv4, gopacket.Default)
			IPv4 := packet1.Layer(layers.LayerTypeIPv4)
			if IPv4 != nil {
				ipv4 := IPv4.(*layers.IPv4)
				IPv4 := ipv4.TTL
				return IPv4
			}
		}
	}
	return 7 // Return 7 if not a valid packet and no ethernet header present.
}

// decodeL2Packet decodes L2 header in the packet and returns destination MAC and ethernet type.
func decodeL2Packet(t *testing.T, packetData []byte) (string, layers.EthernetType) {
	t.Helper()
	packet := gopacket.NewPacket(packetData, layers.LayerTypeEthernet, gopacket.Default)
	etherHeader := packet.Layer(layers.LayerTypeEthernet)
	if etherHeader != nil {
		header, ok := etherHeader.(*layers.Ethernet)
		if header.EthernetType == layers.EthernetType(0x88cc) || header.EthernetType == layers.EthernetType(0x6007) {
			if ok {
				return header.DstMAC.String(), header.EthernetType
			}
		}
	}
	return "", layers.EthernetType(0)
}

// testPktinPktout sends out packetout and packetin traffic together simultaneously.
func testPktInPktOut(ctx context.Context, t *testing.T, args *testArgs) {
	leader := args.leader

	// Insert wbb acl entry on the DUT
	if err := programTableEntry(leader, false); err != nil {
		t.Fatalf("There is error when programming entry")
	}
	// Delete wbb acl entry on the device
	defer programTableEntry(leader, true)

	perfTests := []struct {
		desc       string
		client     *p4rt_client.P4RTClient
		expectPass bool
		wantPkts   int
	}{{
		desc:       "PacketOut and Packetin traffic tests",
		client:     leader,
		expectPass: true,
	}}

	for _, test := range perfTests {
		t.Run(test.desc, func(t *testing.T) {
			// Check initial packet counters
			port := sortPorts(args.ate.Ports())[0].Name()
			counter0 := gnmi.Get(t, args.ate, gnmi.OC().Interface(port).Counters().InPkts().State())

			packets := getPacketOut(portID, false)
			srcEndPoint := args.top.Interfaces()[atePort1.Name]
			streamChan := leader.StreamChannelGet(&streamName)

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
			flows := getTrafficFlow(args, args.ate)
			pktIn := 0
			// Run Packetin and packetout traffic in parallel.
			var wg sync.WaitGroup
			wg.Add(4)

			go func() {
				defer wg.Done()
				pktIn = testTraffic(t, args.ate, flows, srcEndPoint, 20)
				t.Logf("Total Packetin packets sent from ATE %v", pktIn)
			}()

			go func() {
				defer wg.Done()
				time.Sleep(time.Duration(packetOutWait) * time.Second)
				sendPackets(t, test.client, packets[0], packetCount, 2.6) // GDP packetout with 2.6ms timer.
			}()

			go func() {
				defer wg.Done()
				time.Sleep(time.Duration(packetOutWait) * time.Second)
				sendPackets(t, test.client, packets[1], packetCount, 3.1) // Traceroute packetout with 3.1ms timer.
			}()

			go func() {
				defer wg.Done()
				time.Sleep(time.Duration(packetOutWait) * time.Second)
				sendPackets(t, test.client, packets[2], packetCount, 5.1) // LLDP packetout with 5.1ms timer.
			}()

			wg.Wait() // Wait for all four goroutines to finish before exiting.

			test.wantPkts = pktIn
			// Check packet counters after packet out
			counter1 := gnmi.Get(t, args.ate, gnmi.OC().Interface(port).Counters().InPkts().State())

			// Verify Packetout stats to check P4RT stream
			t.Logf("Received %v packetout on ATE port %s", counter1-counter0, port)

			if test.expectPass {
				if float32(counter1-counter0) < (float32(3*packetCount) * 0.85) {
					t.Errorf("Number of Packetout packets, got: %d, want: %d", counter1-counter0, (3 * packetCount))
				}
			} else {
				if float32(counter1-counter0) > (float32(3*packetCount) * 0.10) {
					t.Errorf("Unexpected packets are received.")
				}
			}
			_, packetin_pkts, err := test.client.StreamChannelGetPackets(&streamName, uint64(test.wantPkts), 30*time.Second)
			if err != nil {
				t.Errorf("Unexpected error on StreamChannelGetPackets: %v", err)
			}

			if got, want := len(packetin_pkts), test.wantPkts; got != want {
				t.Errorf("Number of PacketIn, got: %d, want: %d", got, want)
			}
			if test.wantPkts == 0 {
				return
			}
			t.Logf("Start to decode packetin and compare with expected packets.")
			wantgdpPacket := args.gdpPacketIO.GetPacketTemplate()
			wantlldpPacket := args.lldpPacketIO.GetPacketTemplate()
			wanttrPacket := args.trPacketIO.GetPacketTemplate()

			gdpIncount := 0
			lldpIncount := 0
			trIncount := 0

			for _, packet := range packetin_pkts {
				if packet != nil {
					dstMac, etherType := decodeL2Packet(t, packet.Pkt.GetPayload())
					ttl := decodeIPPacketTTL(t, packet.Pkt.GetPayload())
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
										found := false
										portData := args.gdpPacketIO.GetEgressPort()
										if string(data.GetValue()) == portData {
											found = true
											gdpIncount += 1
										}
										if !found {
											t.Fatalf("Egress Port Id is not matching expectation for GDP.")
										}
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
										found := false
										portData := args.lldpPacketIO.GetEgressPort()
										if string(data.GetValue()) == portData {
											found = true
											lldpIncount += 1
										}
										if !found {
											t.Fatalf("Egress Port Id is not matching expectation for LLDP.")
										}
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
							trIncount += 1
						}	
					}
				}
			}
			if total_pktout := (gdpIncount + lldpIncount + trIncount); float32(total_pktout) < (0.95 * float32(test.wantPkts)) {
				t.Fatalf("Not all Packetin Packets are received by P4RT client")
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

// setupP4RTClient sends client arbitration message for the leader client,
// then sends setforwordingpipelineconfig config.
func setupP4RTClient(ctx context.Context, args *testArgs) error {
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
func getGDPParameter() PacketIO {
	return &GDPPacketIO{
		PacketIOPacket: PacketIOPacket{
			SrcMAC:       srcMac,
			DstMAC:       gdpDstmac,
			EthernetType: &gdpEtherType,
		},
		IngressPort: fmt.Sprint(portID),
	}
}

// getTracerouteParameter returns Traceroute related parameters for testPacketIn testcase.
func getTracerouteParameter() PacketIO {
	return &TraceroutePacketIO{
		PacketIOPacket: PacketIOPacket{
			TTL:      &ttl1,
			HopLimit: &hopLimit1,
		},
		IngressPort: fmt.Sprint(portID),
		EgressPort:  fmt.Sprint(portID + 1),
	}
}

// getLLDPParameter returns LLDP related parameters for testPacketIn testcase.
func getLLDPParameter() PacketIO {
	return &LLDPPacketIO{
		PacketIOPacket: PacketIOPacket{
			SrcMAC:       srcMac,
			DstMAC:       lldpDstmac,
			EthernetType: &lldpEthertype,
		},
		IngressPort: fmt.Sprint(portID),
	}
}

func TestP4rtPerformance(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ctx := context.Background()

	configureDUT(t, dut)

	// Configure the ATE
	ate := ondatra.ATE(t, "ate")
	top := configureATE(t, ate)
	top.Push(t).StartProtocols(t)

	// Configure P4RT device-id
	configureDeviceID(ctx, t, dut)

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

	if err := setupP4RTClient(ctx, args); err != nil {
		t.Fatalf("Could not setup p4rt client: %v", err)
	}
	args.gdpPacketIO = getGDPParameter()
	args.lldpPacketIO = getLLDPParameter()
	args.trPacketIO = getTracerouteParameter()
	testPktInPktOut(ctx, t, args)
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

// packetTracerouteRequestGet generates PacketOut payload for Traceroute packets.
func packetTracerouteRequestGet(srcMAC, dstMAC net.HardwareAddr, isIPv4 bool, ttl uint8, seq int) ([]byte, error) {
	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{
		FixLengths:       true,
		ComputeChecksums: true,
	}
	payload := []byte{}
	payLoadLen := 32

	ethType := layers.EthernetTypeIPv4
	if !isIPv4 {
		ethType = layers.EthernetTypeIPv6
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
	if isIPv4 {
		if err := gopacket.SerializeLayers(buf, opts,
			pktEth, pktIpv4, pktICMP4, gopacket.Payload(payload),
		); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	}
	if err := gopacket.SerializeLayers(buf, opts,
		pktEth, pktIpv6, pktICMP6, gopacket.Payload(payload),
	); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
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

// GetTableEntry creates wbb acl entry related to GDP,LLDP and traceroute.
func getTableEntry(delete bool) []*p4rtutils.ACLWbbIngressTableEntryInfo {
	actionType := p4_v1.Update_INSERT
	if delete {
		actionType = p4_v1.Update_DELETE
	}
	return []*p4rtutils.ACLWbbIngressTableEntryInfo{{
		Type:          actionType,
		EtherType:     0x6007,
		EtherTypeMask: 0xFFFF,
		Priority:      1,
	},
		{
			Type:          actionType,
			EtherType:     0x88cc,
			EtherTypeMask: 0xFFFF,
			Priority:      1,
		},
		{
			Type:     actionType,
			IsIpv4:   0x1,
			TTL:      0x2,
			TTLMask:  0xFF,
			Priority: 1,
		},
		{
			Type:     actionType,
			IsIpv6:   0x1,
			TTL:      0x2,
			TTLMask:  0xFF,
			Priority: 1,
		},
	}
}

// GetPacketOut generates 3 PacketOut messages with payload as GDP,LLDP and traceroute.
func getPacketOut(portID uint32, submitIngress bool) []*p4_v1.PacketOut {
	packets := []*p4_v1.PacketOut{}
	packet1 := &p4_v1.PacketOut{
		Payload: packetGDPRequestGet(),
		Metadata: []*p4_v1.PacketMetadata{
			{
				MetadataId: uint32(1), // "egress_port"
				Value:      []byte(fmt.Sprint(portID)),
			},
		},
	}
	packets = append(packets, packet1)

	srcMAC := net.HardwareAddr{0x00, 0x12, 0x8a, 0x00, 0x00, 0x01}
	dstMAC := net.HardwareAddr{0x02, 0xF6, 0x65, 0x64, 0x00, 0x08}
	isIPv4 := true
	pkt, _ := packetTracerouteRequestGet(srcMAC, dstMAC, isIPv4, 2, 1)

	packet2 := &p4_v1.PacketOut{
		Payload: pkt,
		Metadata: []*p4_v1.PacketMetadata{
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
	packets = append(packets, packet2)
	packet3 := &p4_v1.PacketOut{
		Payload: packetLLDPRequestGet(),
		Metadata: []*p4_v1.PacketMetadata{
			{
				MetadataId: uint32(1), // "egress_port"
				Value:      []byte(fmt.Sprint(portID)),
			},
		},
	}
	packets = append(packets, packet3)
	return packets
}

// GetTrafficFlow generates ATE traffic flows for LLDP.
func getTrafficFlow(args *testArgs, ate *ondatra.ATEDevice) []*ondatra.Flow {
	ethHeader1 := ondatra.NewEthernetHeader()
	ethHeader1.WithSrcAddress(*srcMac)
	ethHeader1.WithDstAddress(*lldpDstmac)
	ethHeader1.WithEtherType(lldpEthertype)

	// flow1 for LLDP traffic.
	flow1 := ate.Traffic().NewFlow("LLDP").WithFrameSize(packetInPktsize).WithFrameRateBPS(lldpBitRate).WithHeaders(ethHeader1)
	flow1.Transmission().WithPatternFixedDuration(duration)

	ethHeader2 := ondatra.NewEthernetHeader()
	ethHeader2.WithSrcAddress(*srcMac)
	ethHeader2.WithDstAddress(*gdpDstmac)
	ethHeader2.WithEtherType(uint32(gdpInLayers))

	// flow2 for GDP traffic.
	flow2 := ate.Traffic().NewFlow("GDP").WithFrameSize(packetInPktsize).WithFrameRateBPS(gdpBitRate).WithHeaders(ethHeader2)
	flow2.Transmission().WithPatternFixedDuration(duration)

	ethHeader := ondatra.NewEthernetHeader()
	ipv4Header := ondatra.NewIPv4Header().WithSrcAddress(atePort1.IPv4).WithDstAddress(atePort2.IPv4).WithTTL(ttl1) //ttl=1 is traceroute traffic

	// flow3 for Traceroute traffic.
	flow3 := ate.Traffic().NewFlow("IPv4").WithFrameSize(packetInPktsize).WithFrameRateFPS(trPacketRate).WithHeaders(ethHeader, ipv4Header)
	flow3.Transmission().WithPatternFixedDuration(duration)

	var flows []*ondatra.Flow
	flows = append(flows, flow1)
	flows = append(flows, flow2)
	flows = append(flows, flow3)

	return flows
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
