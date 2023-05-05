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
//go test -v  -testbed /root/ondatra/featureprofiles/topologies/atedut_2.testbed -binding /root/ondatra/featureprofiles/topologies/atedut_2.binding -outputs_dir logs

package traceroute_packetout_test

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"sort"
	"testing"

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
	p4v1 "github.com/p4lang/p4runtime/go/p4/v1"
)

const (
	ipv4PrefixLen = 30
	ipv6PrefixLen = 126
	deviceID      = uint64(100)
	portId        = uint32(2100)
	electionId    = uint64(100)
	dstMAC        = "00:1A:11:00:00:01"
)

var (
	p4InfoFile = flag.String("p4info_file_location", "../../wbb.p4info.pb.txt", "Path to the p4info file.")
	streamName = "p4rt"
)

func dMAC(t *testing.T, dut *ondatra.DUTDevice) string {
	if !deviations.GRIBIMACOverrideWithStaticARP(dut) {
		return dstMAC
	}
	gnmi.Replace(t, dut, gnmi.OC().System().MacAddress().RoutingMac().Config(), dstMAC)
	return dstMAC
}

var (
	dutPort1 = attrs.Attributes{
		Desc:    "dutPort1",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::192:0:2:1",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}

	atePort1 = attrs.Attributes{
		Name:    "atePort1",
		MAC:     "02:11:01:00:00:01",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::192:0:2:2",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}

	dutPort2 = attrs.Attributes{
		Desc:    "dutPort2",
		IPv4:    "192.0.2.5",
		IPv6:    "2001:db8::192:0:2:5",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}

	atePort2 = attrs.Attributes{
		Name:    "atePort2",
		MAC:     "02:12:01:00:00:01",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::192:0:2:6",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
)

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

// configureDUT configures port1 and port2 on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	d := gnmi.OC()

	p1 := dut.Port(t, "port1").Name()
	i1 := dutPort1.NewOCInterface(p1)
	i1.Id = ygot.Uint32(portId)
	gnmi.Replace(t, dut, d.Interface(p1).Config(), i1)

	p2 := dut.Port(t, "port2").Name()
	i2 := dutPort2.NewOCInterface(p2)
	i2.Id = ygot.Uint32(portId + 1)
	gnmi.Replace(t, dut, d.Interface(p2).Config(), i2)

	if *deviations.ExplicitPortSpeed {
		fptest.SetPortSpeed(t, dut.Port(t, "port1"))
		fptest.SetPortSpeed(t, dut.Port(t, "port2"))
	}
	if *deviations.ExplicitInterfaceInDefaultVRF {
		fptest.AssignToNetworkInstance(t, dut, p1, *deviations.DefaultNetworkInstance, 0)
		fptest.AssignToNetworkInstance(t, dut, p2, *deviations.DefaultNetworkInstance, 0)
	}
}

// configureATE configures port1 and port2 on the ATE.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	otg := ate.OTG()
	top := otg.NewConfig(t)

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
		ElectionIdL: electionId,
	}

	// Send ClientArbitration message on both p4rt leader and backup clients.
	client := args.leader

	if client != nil {
		client.StreamChannelCreate(&streamParameter)
		if err := client.StreamChannelSendMsg(&streamName, &p4v1.StreamMessageRequest{
			Update: &p4v1.StreamMessageRequest_Arbitration{
				Arbitration: &p4v1.MasterArbitrationUpdate{
					DeviceId: streamParameter.DeviceId,
					ElectionId: &p4v1.Uint128{
						High: streamParameter.ElectionIdH,
						Low:  streamParameter.ElectionIdL - uint64(0),
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
	// Load p4info file.
	p4Info, err := utils.P4InfoLoad(p4InfoFile)
	if err != nil {
		return errors.New("Errors seen when loading p4info file.")
	}
	// Send SetForwardingPipelineConfig for p4rt leader client.
	if err := args.leader.SetForwardingPipelineConfig(&p4v1.SetForwardingPipelineConfigRequest{
		DeviceId:   deviceID,
		ElectionId: &p4v1.Uint128{High: uint64(0), Low: electionId},
		Action:     p4v1.SetForwardingPipelineConfigRequest_VERIFY_AND_COMMIT,
		Config: &p4v1.ForwardingPipelineConfig{
			P4Info: &p4Info,
			Cookie: &p4v1.ForwardingPipelineConfig_Cookie{
				Cookie: 159,
			},
		},
	}); err != nil {
		return errors.New("errors seen when sending SetForwardingPipelineConfig.")
	}
	return nil
}

// getTracerouteParameter returns Traceroute related parameters for testPacketOut testcase.
func getTracerouteParameter(t *testing.T) PacketIO {
	return &TraceroutePacketIO{
		IngressPort: fmt.Sprint(portId),
	}
}
func TestPacketOut(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ctx := context.Background()
	// Configure the DUT
	configureDUT(t, dut)
	// Configure the ATE
	ate := ondatra.ATE(t, "ate")
	top := configureATE(t, ate)

	otg := ate.OTG()
	otg.PushConfig(t, top)
	otg.StartProtocols(t)

	// Configure P4RT device-id and port-id on the DUT
	configureDeviceID(ctx, t, dut)

	leader := p4rt_client.NewP4RTClient(&p4rt_client.P4RTClientParameters{})
	if err := leader.P4rtClientSet(dut.RawAPIs().P4RT().Default(t)); err != nil {
		t.Fatalf("Could not initialize p4rt client: %v", err)
	}

	sm := gnmi.Get(t, dut, gnmi.OC().Interface(dut.Port(t, "port1").Name()).Ethernet().MacAddress().State())
	srcMAC, err := net.ParseMAC(sm)
	if err != nil {
		t.Fatalf("Couldn't parse Source MAC: %v", err)
	}

	dstMAC, err := net.ParseMAC(dMAC(t, dut))
	if err != nil {
		t.Fatalf("Couldn't parse router MAC: %v", err)
	}

	args := &testArgs{
		ctx:    ctx,
		leader: leader,
		dut:    dut,
		ate:    ate,
		top:    top,
		srcMAC: srcMAC,
		dstMAC: dstMAC,
	}

	if err := setupP4RTClient(ctx, args); err != nil {
		t.Fatalf("Could not setup p4rt client: %v", err)
	}

	args.packetIO = getTracerouteParameter(t)
	testPacketOut(ctx, t, args)
}

type TraceroutePacketIO struct {
	PacketIO
	IngressPort string
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

// GetPacketOut generates PacketOut message with payload as Traceroute IPv6 and IPv6 packets.
// isIPv4==true refers to the ipv4 packets and if false we are sending ipv6 packet
func (traceroute *TraceroutePacketIO) GetPacketOut(srcMAC, dstMAC net.HardwareAddr, portID uint32, isIPv4 bool, ttl uint8, numPkts int) ([]*p4v1.PacketOut, error) {
	packets := []*p4v1.PacketOut{}
	for i := 1; i <= numPkts; i++ {
		pkt, err := packetTracerouteRequestGet(srcMAC, dstMAC, isIPv4, ttl, i)
		if err != nil {
			return nil, err
		}
		packet := &p4v1.PacketOut{
			Payload: pkt,
			Metadata: []*p4v1.PacketMetadata{
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
		packets = append(packets, packet)
	}
	return packets, nil
}
