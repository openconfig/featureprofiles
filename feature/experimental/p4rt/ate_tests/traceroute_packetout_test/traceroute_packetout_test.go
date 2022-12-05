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
	"github.com/openconfig/featureprofiles/internal/attrs"
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
	deviceId      = uint64(100)
	portId        = uint32(2100)
	electionId    = uint64(100)
)

var (
	p4InfoFile                                 = flag.String("p4info_file_location", "/root/ondatra/featureprofiles/feature/experimental/p4rt/wbb.p4info.pb.txt", "Path to the p4info file.")
	p4rtNodeName                               = flag.String("p4rt_node_name", "FPC0:NPU0", "component name for P4RT Node")
	streamName                                 = "p4rt"
	tracerouteipv4InLayers layers.EthernetType = 0x0800
	checksum                                   = uint16(200)
)

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
	gnmi.Replace(t, dut, d.Interface(p1).Config(), i1)

	p2 := dut.Port(t, "port2").Name()
	i2 := dutPort2.NewOCInterface(p2)
	gnmi.Replace(t, dut, d.Interface(p2).Config(), i2)
}

// configureATE configures port1 and port2 on the ATE.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) *ondatra.ATETopology {
	top := ate.Topology().New()

	p1 := ate.Port(t, "port1")
	atePort1.AddToATE(top, p1, &dutPort1)

	p2 := ate.Port(t, "port2")
	atePort2.AddToATE(top, p2, &dutPort2)
	return top
}

// configureDeviceId configures p4rt device-id on the DUT.
func configureDeviceId(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice) {
	component := oc.Component{
		IntegratedCircuit: &oc.Component_IntegratedCircuit{
			NodeId: ygot.Uint64(deviceId),
		},
		Name: ygot.String(*p4rtNodeName),
	}
	gnmi.Replace(t, dut, gnmi.OC().Component(*p4rtNodeName).Config(), &component)
}

// configurePortId configures p4rt port-id on the DUT.
func configurePortId(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice) {
	ports := sortPorts(dut.Ports())
	for i, port := range ports {
		gnmi.Replace(t, dut, gnmi.OC().Interface(port.Name()).Id().Config(), uint32(i*1000)+portId)
	}
}

// setupP4RTClient sends client arbitration message for both leader and follower clients,
// then sends setforwordingpipelineconfig with leader client.
func setupP4RTClient(ctx context.Context, args *testArgs) error {
	// Setup p4rt-client stream parameters
	streamParameter := p4rt_client.P4RTStreamParameters{
		Name:        streamName,
		DeviceId:    deviceId,
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
			return errors.New("Errors seen when sending ClientArbitration message.")
		}
		if _, _, arbErr := client.StreamChannelGetArbitrationResp(&streamName, 1); arbErr != nil {
			return errors.New("Errors seen in ClientArbitration response.")
		}
	}
	// Load p4info file.
	p4Info, err := utils.P4InfoLoad(p4InfoFile)
	if err != nil {
		return errors.New("Errors seen when loading p4info file.")
	}
	// Send SetForwardingPipelineConfig for p4rt leader client.
	if err := args.leader.SetForwardingPipelineConfig(&p4v1.SetForwardingPipelineConfigRequest{
		DeviceId:   deviceId,
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
	top.Push(t).StartProtocols(t)

	// Configure P4RT device-id and port-id on the DUT
	configureDeviceId(ctx, t, dut)
	configurePortId(ctx, t, dut)

	leader := p4rt_client.P4RTClient{}
	if err := leader.P4rtClientSet(dut.RawAPIs().P4RT().Default(t)); err != nil {
		t.Fatalf("Could not initialize p4rt client: %v", err)
	}

	args := &testArgs{
		ctx:    ctx,
		leader: &leader,
		dut:    dut,
		ate:    ate,
		top:    top,
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
func packetTracerouteRequestGet(isIPv4 bool, ttl uint8) []byte {
	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{
		FixLengths:       true,
		ComputeChecksums: true,
	}
	payload := []byte{}
	payLoadLen := 64

	pktEthipv4 := &layers.Ethernet{
		EthernetType: tracerouteipv4InLayers,
		SrcMAC:       net.HardwareAddr{0x00, 0xAA, 0x00, 0xAA, 0x00, 0xAA},
		// traceroute MAC is 68:F3:8E:64:F3:FC
		DstMAC: net.HardwareAddr{0x02, 0xF6, 0x65, 0x64, 0x00, 0x08},
	}

	pktICMP4 := &layers.ICMPv4{
		TypeCode: layers.ICMPv4TypeTimeExceeded,
		Checksum: checksum,
	}

	pktIpv4 := &layers.IPv4{
		Version:  4,
		TTL:      ttl,
		SrcIP:    net.IP{192, 0, 2, 1},
		DstIP:    net.IP{192, 0, 2, 2},
		Protocol: layers.IPProtocolICMPv4,
	}

	//for ipv6
	pktEthipv6 := &layers.Ethernet{
		EthernetType: 0x86DD,
		SrcMAC:       net.HardwareAddr{0x00, 0xAA, 0x00, 0xAA, 0x00, 0xAA},
		// traceroute MAC is 68:F3:8E:64:F3:FC (HW)
		//traceroute MAC is 02:f6:65:64:00:08 (Virtual)
		DstMAC: net.HardwareAddr{0x68, 0xF3, 0x8E, 0x64, 0xF3, 0xFC},
	}
	pktIpv6 := &layers.IPv6{
		Version:    6,
		HopLimit:   ttl,
		NextHeader: layers.IPProtocolICMPv6,
		SrcIP:      net.ParseIP("2001:db8::192:0:2:1"),
		DstIP:      net.ParseIP("2001:db8::192:0:2:2"),
	}

	for i := 0; i < payLoadLen; i++ {
		payload = append(payload, byte(i))
	}
	if isIPv4 {
		gopacket.SerializeLayers(buf, opts,
			pktEthipv4, pktIpv4, pktICMP4, gopacket.Payload(payload),
		)
		return buf.Bytes()
	} else {
		gopacket.SerializeLayers(buf, opts,
			pktEthipv6, pktIpv6, gopacket.Payload(payload),
		)
		return buf.Bytes()
	}
}

// GetPacketOut generates PacketOut message with payload as Traceroute IPv6 and IPv6 packets.
// isIPv4==true refers to the ipv4 packets and if false we are sending ipv6 packet
func (traceroute *TraceroutePacketIO) GetPacketOut(portID uint32, isIPv4 bool, ttl uint8) []*p4v1.PacketOut {
	packets := []*p4v1.PacketOut{}
	packet := &p4v1.PacketOut{
		Payload: packetTracerouteRequestGet(isIPv4, ttl),
		Metadata: []*p4v1.PacketMetadata{
			{
				MetadataId: uint32(2), // "egress_port"
				Value:      []byte{1},
			},
		},
	}
	packets = append(packets, packet)
	return packets
}
