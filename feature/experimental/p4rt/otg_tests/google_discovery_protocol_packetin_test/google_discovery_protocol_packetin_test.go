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
	"strings"
	"testing"
	"time"

	"github.com/cisco-open/go-p4/p4rt_client"
	"github.com/cisco-open/go-p4/utils"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/feature/experimental/p4rt/internal/p4rtutils"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"

	p4v1pb "github.com/p4lang/p4runtime/go/p4/v1"
)

const (
	ipv4PrefixLen = 30
)

var (
	p4InfoFile          = flag.String("p4info_file_location", "../../wbb.p4info.pb.txt", "Path to the p4info file.")
	gdpSrcMAC           = flag.String("gdp_src_MAC", "00:01:00:02:00:03", "source MAC address for PacketIn")
	streamName          = "p4rt"
	gdpDstMAC           = "00:0a:da:f0:f0:f0"
	myStationMAC        = "00:1A:11:00:00:01"
	gdpEtherType        = uint32(0x6007)
	deviceID            = uint64(1)
	portID              = uint32(10)
	electionID          = uint64(100)
	metadataIngressPort = uint32(1)
	metadataEgressPort  = uint32(2)
	vlanID              = uint16(4000)
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
	GetPacketTemplate() *PacketIOPacket
	GetTrafficFlows(ate *ondatra.ATEDevice, frameSize uint32, frameRate uint64) []gosnappi.Flow
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

// decodePacket decodes L2 header in the packet and returns source and destination MAC
// vlanID, and ethernet type.
func decodePacket(t *testing.T, packetData []byte) (string, string, uint16, layers.EthernetType) {
	t.Helper()
	packet := gopacket.NewPacket(packetData, layers.LayerTypeEthernet, gopacket.Default)
	etherHeader := packet.Layer(layers.LayerTypeEthernet)
	d1qHeader := packet.Layer(layers.LayerTypeDot1Q)

	srcMAC, dstMAC := "", ""
	vlanID := uint16(0)
	etherType := layers.EthernetType(0)
	if etherHeader != nil {
		header, decoded := etherHeader.(*layers.Ethernet)
		if decoded {
			srcMAC, dstMAC = header.SrcMAC.String(), header.DstMAC.String()
			if header.EthernetType != layers.EthernetTypeDot1Q {
				return srcMAC, dstMAC, vlanID, header.EthernetType
			}
		}
	}
	if d1qHeader != nil {
		header, decoded := d1qHeader.(*layers.Dot1Q)
		if decoded {
			vlanID = header.VLANIdentifier
			etherType = header.Type
		}
	}
	return srcMAC, dstMAC, vlanID, etherType
}

// testTraffic sends traffic flow for duration seconds and returns the
// number of packets sent out.
func testTraffic(t *testing.T, top gosnappi.Config, ate *ondatra.ATEDevice, flows []gosnappi.Flow, duration int) map[string]uint64 {
	t.Helper()
	for _, flow := range flows {
		flow.TxRx().Port().SetTxName("port1").SetRxName("port2")
		flow.Metrics().SetEnable(true)
		top.Flows().Append(flow)
	}
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")

	ate.OTG().StartTraffic(t)
	time.Sleep(time.Duration(duration) * time.Second)

	ate.OTG().StopTraffic(t)
	fNames := []string{"GDPWithVlan", "GDPWithoutVlan", "NonGDP"}
	total := map[string]uint64{}
	for _, fName := range fNames {
		total[fName] = gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(fName).Counters().OutPkts().State())
		if total[fName] == 0 {
			t.Errorf("Coundn't send packets out for flow: %s", fName)
		}
	}

	return total
}

func validateTrafficAtATE(t *testing.T, ate *ondatra.ATEDevice, pktOut map[string]uint64) {
	wantPkts := map[string]uint64{"GDPWithVlan": 0, "GDPWithoutVlan": 0, "NonGDP": pktOut["NonGDP"]}
	for fName, want := range wantPkts {
		got := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(fName).Counters().InPkts().State())
		if got != want {
			t.Errorf("Number of PacketIN at ATE-Port2 for flow: %s, got: %d, want: %d", fName, got, want)
		}
	}
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
	pktOut := testTraffic(t, args.top, args.ate, args.packetIO.GetTrafficFlows(args.ate, 300, 2), 20)
	validateTrafficAtATE(t, args.ate, pktOut)

	packetInTests := []struct {
		desc     string
		client   *p4rt_client.P4RTClient
		wantPkts bool
	}{{
		desc:     "PacketIn to Primary Controller",
		client:   leader,
		wantPkts: true,
	}, {
		desc:     "PacketIn to Secondary Controller",
		client:   follower,
		wantPkts: false,
	}}

	for _, test := range packetInTests {
		t.Run(test.desc, func(t *testing.T) {
			pktCount := pktOut["GDPWithVlan"] + pktOut["GDPWithoutVlan"]
			if !test.wantPkts {
				pktCount = 0
			}
			// Extract packets from PacketIn message sent to p4rt client
			_, packets, err := test.client.StreamChannelGetPackets(&streamName, pktCount, 30*time.Second)
			if err != nil {
				t.Errorf("Unexpected error on StreamChannelGetPackets: %v", err)
			}

			gdpWithoutVlanPkts := uint64(0)
			gdpWithVlanPkts := uint64(0)
			gotVlanID := uint16(0)
			t.Logf("Start to decode packet and compare with expected packets.")
			wantPacket := args.packetIO.GetPacketTemplate()
			for _, packet := range packets {
				if packet != nil {
					if wantPacket.DstMAC != nil && wantPacket.EthernetType != nil {
						srcMAC, dstMac, vID, etherType := decodePacket(t, packet.Pkt.GetPayload())
						if dstMac != *wantPacket.DstMAC || etherType != layers.EthernetType(*wantPacket.EthernetType) {
							continue
						}
						if !strings.EqualFold(srcMAC, *gdpSrcMAC) {
							continue
						}
						gotVlanID = vID
					}
					if gotVlanID == 0 {
						gdpWithoutVlanPkts++
					} else {
						if got, want := gotVlanID, vlanID; got != want {
							t.Errorf("VLAN ID mismatch, got: %d, want: %d", got, want)
						}
						gdpWithVlanPkts++
					}

					metaData := packet.Pkt.GetMetadata()
					for _, data := range metaData {
						switch data.GetMetadataId() {
						case metadataIngressPort:
							if string(data.GetValue()) != args.packetIO.GetIngressPort() {
								t.Fatalf("Ingress Port Id is not matching expectation.")
							}
						case metadataEgressPort:
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
			if !test.wantPkts {
				if gdpWithoutVlanPkts+gdpWithVlanPkts != 0 {
					t.Fatalf("Number of GDP packets got: %d, want: 0", gdpWithoutVlanPkts+gdpWithVlanPkts)
				}
				return
			}
			if got, want := gdpWithoutVlanPkts, pktOut["GDPWithoutVlan"]; got != want {
				t.Errorf("Number of GDP without Vlan PacketIn, got: %d, want: %d", got, want)
			}
			if got, want := gdpWithVlanPkts, pktOut["GDPWithVlan"]; got != want {
				t.Errorf("Number of GDP with Vlan PacketIn, got: %d, want: %d", got, want)
			}
		})
	}
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// configInterfaceDUT configures the interface with the Addrs.
func configInterfaceDUT(i *oc.Interface, a *attrs.Attributes, dut *ondatra.DUTDevice, hasVlan bool) *oc.Interface {
	i.Description = ygot.String(a.Desc)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	if deviations.InterfaceEnabled(dut) {
		i.Enabled = ygot.Bool(true)
	}
	s := i.GetOrCreateSubinterface(0)
	s4 := s.GetOrCreateIpv4()
	if deviations.InterfaceEnabled(dut) && !deviations.IPv4MissingEnabled(dut) {
		s4.Enabled = ygot.Bool(true)
	}
	s4a := s4.GetOrCreateAddress(a.IPv4)
	s4a.PrefixLength = ygot.Uint8(ipv4PrefixLen)

	if hasVlan && deviations.P4RTGdpRequiresDot1QSubinterface(dut) {
		s1 := i.GetOrCreateSubinterface(1)
		s1.GetOrCreateVlan().GetOrCreateMatch().GetOrCreateSingleTagged().SetVlanId(vlanID)
		if deviations.NoMixOfTaggedAndUntaggedSubinterfaces(dut) {
			s.GetOrCreateVlan().GetOrCreateMatch().GetOrCreateSingleTagged().SetVlanId(10)
			i.GetOrCreateAggregation().GetOrCreateSwitchedVlan().SetNativeVlan(10)
		}
	}
	return i
}

// configureDUT configures port1 and port2 on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	d := gnmi.OC()

	p1 := dut.Port(t, "port1")
	i1 := &oc.Interface{Name: ygot.String(p1.Name()), Id: ygot.Uint32(portID)}
	gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), configInterfaceDUT(i1, &dutPort1, dut, true /*hasVlan*/))
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, p1.Name(), deviations.DefaultNetworkInstance(dut), 0)
	}

	p2 := dut.Port(t, "port2")
	i2 := &oc.Interface{Name: ygot.String(p2.Name()), Id: ygot.Uint32(portID + 1)}
	gnmi.Replace(t, dut, d.Interface(p2.Name()).Config(), configInterfaceDUT(i2, &dutPort2, dut, false))
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, p2.Name(), deviations.DefaultNetworkInstance(dut), 0)
	}
	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, p1)
		fptest.SetPortSpeed(t, p2)
	}
	gnmi.Replace(t, dut, gnmi.OC().Lldp().Enabled().Config(), false)
	gnmi.Replace(t, dut, gnmi.OC().System().MacAddress().RoutingMac().Config(), myStationMAC)
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

			client.StreamChannelGet(&streamName).SetPacketQSize(10000)
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

// getGDPParameter returns GDP related parameters for testPacketIn testcase.
func getGDPParameter(t *testing.T) PacketIO {
	return &GDPPacketIO{
		PacketIOPacket: PacketIOPacket{
			SrcMAC:       gdpSrcMAC,
			DstMAC:       &gdpDstMAC,
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
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)

	// Configure P4RT device-id
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
	testPacketIn(ctx, t, args)
}

type GDPPacketIO struct {
	PacketIOPacket
	IngressPort string
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

// GetPacketTemplate returns expected packets in PacketIn.
func (gdp *GDPPacketIO) GetPacketTemplate() *PacketIOPacket {
	return &gdp.PacketIOPacket
}

// GetTrafficFlows generates OTG traffic flows for GDP.
func (gdp *GDPPacketIO) GetTrafficFlows(ate *ondatra.ATEDevice, frameSize uint32, frameRate uint64) []gosnappi.Flow {

	f1 := gosnappi.NewFlow()
	f1.SetName("GDPWithVlan")
	eth1 := f1.Packet().Add().Ethernet()
	eth1.Src().SetValue(*gdp.SrcMAC)
	eth1.Dst().SetValue(*gdp.DstMAC)
	eth1.EtherType().SetValue(uint32(0x8100))

	vlan := f1.Packet().Add().Vlan()
	vlan.Id().SetValue(uint32(vlanID))
	vlan.Tpid().SetValue(uint32(*gdp.EthernetType))

	f1.Size().SetFixed(uint32(frameSize))
	f1.Rate().SetPps(uint64(frameRate))

	f2 := gosnappi.NewFlow()
	f2.SetName("GDPWithoutVlan")
	eth2 := f2.Packet().Add().Ethernet()
	eth2.Src().SetValue(*gdp.SrcMAC)
	eth2.Dst().SetValue(*gdp.DstMAC)
	eth2.EtherType().SetValue(uint32(*gdp.EthernetType))

	f2.Size().SetFixed(uint32(frameSize))
	f2.Rate().SetPps(uint64(frameRate))

	f3 := gosnappi.NewFlow()
	f3.SetName("NonGDP")
	eth3 := f3.Packet().Add().Ethernet()
	eth3.Src().SetValue(*gdp.SrcMAC)
	eth3.Dst().SetValue(myStationMAC)
	ip3 := f3.Packet().Add().Ipv4()
	ip3.Src().SetValue(atePort1.IPv4)
	ip3.Dst().SetValue(atePort2.IPv4)
	ip3.TimeToLive().SetValue(3)

	f3.Size().SetFixed(uint32(frameSize))
	f3.Rate().SetPps(uint64(frameRate))
	return []gosnappi.Flow{f1, f2, f3}
}

// GetEgressPort returns expected egress port info in PacketIn.
func (gdp *GDPPacketIO) GetEgressPort() []string {
	return []string{"0"}
}

// GetIngressPort return expected ingress port info in PacketIn.
func (gdp *GDPPacketIO) GetIngressPort() string {
	return gdp.IngressPort
}
