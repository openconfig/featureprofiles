// Copyright 2024 Google LLC
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
// package to test P4RT with traceroute traffic of IPV4 and IPV6 with TTL/HopLimit as 0&1.
// go test -v . -testbed /root/ondatra/featureprofiles/topologies/atedut_2.testbed -binding /root/ondatra/featureprofiles/topologies/atedut_2.binding -outputs_dir logs

package traceroute_packetin_with_vrf_selection_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/cisco-open/go-p4/p4rt_client"
	"github.com/cisco-open/go-p4/utils"
	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/p4rtutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
	pb "github.com/p4lang/p4runtime/go/p4/v1"
)

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
			if err := client.StreamChannelSendMsg(&streamName, &pb.StreamMessageRequest{
				Update: &pb.StreamMessageRequest_Arbitration{
					Arbitration: &pb.MasterArbitrationUpdate{
						DeviceId: streamParameter.DeviceId,
						ElectionId: &pb.Uint128{
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
	if err := args.leader.SetForwardingPipelineConfig(&pb.SetForwardingPipelineConfigRequest{
		DeviceId:   deviceID,
		ElectionId: &pb.Uint128{High: uint64(0), Low: electionID},
		Action:     pb.SetForwardingPipelineConfigRequest_VERIFY_AND_COMMIT,
		Config: &pb.ForwardingPipelineConfig{
			P4Info: p4Info,
			Cookie: &pb.ForwardingPipelineConfig_Cookie{
				Cookie: 159,
			},
		},
	}); err != nil {
		return errors.New("errors seen when sending SetForwardingPipelineConfig")
	}
	return nil
}

// getTracerouteParameter returns Traceroute related parameters for testPacketIn testcase.
func getTracerouteParameter(t *testing.T) PacketIO {
	return &TraceroutePacketIO{
		PacketIOPacket: PacketIOPacket{
			TTL:      &TTL1,
			HopLimit: &HopLimit1,
		},
		IngressPort: fmt.Sprint(portID),
		EgressPort:  fmt.Sprint(portID + 7),
	}
}

// testPacket setup p4RT client, table entry and send traffic and returns the
// percentage of packets sent out of each egress port
func testPacket(t *testing.T, args *testArgs, cs gosnappi.ControlState, flowValues []*flowArgs, EgressPortMap map[string]bool) []float64 {
	dut := ondatra.DUT(t, "dut")
	ctx := context.Background()
	ate := ondatra.ATE(t, "ate")
	top := args.otgConfig
	top.Flows().Clear()

	configureDeviceID(ctx, t, dut)

	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	time.Sleep(30 * time.Second)

	args = &testArgs{
		ctx:      ctx,
		leader:   args.leader,
		follower: args.follower,
		dut:      dut,
		ate:      ate,
		top:      top,
	}

	if err := setupP4RTClient(ctx, args); err != nil {
		t.Fatalf("Could not setup p4rt client: %v", err)
	}
	args.packetIO = getTracerouteParameter(t)
	return testPacketIn(ctx, t, args, true, cs, flowValues, EgressPortMap)
}

type TraceroutePacketIO struct {
	PacketIOPacket
	IngressPort string
	EgressPort  string
}

// GetTableEntry creates p4rtutils acl entry which is used to get the configured p4rt trap rules.
// A packet is sent to controller based on the trap rules
func (traceroute *TraceroutePacketIO) GetTableEntry(delete bool, IsIpv4 bool) []*p4rtutils.ACLWbbIngressTableEntryInfo {
	if IsIpv4 {
		actionType := pb.Update_INSERT
		if delete {
			actionType = pb.Update_DELETE
		}
		return []*p4rtutils.ACLWbbIngressTableEntryInfo{{
			Type:     actionType,
			IsIpv4:   0x1,
			TTL:      0x1,
			TTLMask:  0xFF,
			Priority: 1,
		},
			{
				Type:     actionType,
				IsIpv4:   0x1,
				TTL:      0x0,
				TTLMask:  0xFF,
				Priority: 1,
			}}
	}
	actionType := pb.Update_INSERT
	if delete {
		actionType = pb.Update_DELETE
	}
	return []*p4rtutils.ACLWbbIngressTableEntryInfo{{
		Type:     actionType,
		IsIpv6:   0x1,
		TTL:      0x1,
		TTLMask:  0xFF,
		Priority: 1,
	},
		{
			Type:     actionType,
			IsIpv6:   0x1,
			TTL:      0x0,
			TTLMask:  0xFF,
			Priority: 1,
		}}
}

// GetPacketTemplate returns expected packets in PacketIn.
func (traceroute *TraceroutePacketIO) GetPacketTemplate() *PacketIOPacket {
	return &traceroute.PacketIOPacket
}

func (traceroute *TraceroutePacketIO) GetTrafficFlow(ate *ondatra.ATEDevice, dstMac string, isIpv4 bool,
	TTL uint8, frameSize uint32, frameRate uint64, dstIP string, flowValues *flowArgs) gosnappi.Flow {

	flow := gosnappi.NewFlow()
	flow.SetName(flowValues.flowName)
	ethHeader := flow.Packet().Add().Ethernet()
	ethHeader.Src().SetValue(tracerouteSrcMAC)
	ethHeader.Dst().SetValue(dstMac)

	ipHeader := flow.Packet().Add().Ipv4()
	ipHeader.Src().SetValue(flowValues.outHdrSrcIP)
	ipHeader.Dst().SetValue(flowValues.outHdrDstIP)
	ipHeader.TimeToLive().SetValue(uint32(TTL))
	if len(flowValues.outHdrDscp) != 0 {
		ipHeader.Priority().Dscp().Phb().SetValues(flowValues.outHdrDscp)
	}
	if flowValues.udp {
		UDPHeader := flow.Packet().Add().Udp()
		UDPHeader.DstPort().Increment().SetStart(1).SetCount(50000).SetStep(1)
		UDPHeader.SrcPort().Increment().SetStart(1).SetCount(50000).SetStep(1)
	}

	if flowValues.proto != 0 {
		innerIPHdr := flow.Packet().Add().Ipv4()
		innerIPHdr.Protocol().SetValue(flowValues.proto)
		innerIPHdr.Src().SetValue(flowValues.InnHdrSrcIP)
		innerIPHdr.Dst().SetValue(flowValues.InnHdrDstIP)
		innerIPHdr.TimeToLive().SetValue(uint32(TTL))
	} else {
		if flowValues.isInnHdrV4 {
			innerIPHdr := flow.Packet().Add().Ipv4()
			innerIPHdr.Src().SetValue(flowValues.InnHdrSrcIP)
			innerIPHdr.Dst().SetValue(flowValues.InnHdrDstIP)
			UDPHeader := flow.Packet().Add().Udp()
			UDPHeader.DstPort().Increment().SetStart(1).SetCount(50000).SetStep(1)
			UDPHeader.SrcPort().Increment().SetStart(1).SetCount(50000).SetStep(1)
		} else {
			innerIpv6Hdr := flow.Packet().Add().Ipv6()
			innerIpv6Hdr.Src().SetValue(flowValues.InnHdrSrcIPv6)
			innerIpv6Hdr.Dst().SetValue(flowValues.InnHdrDstIPv6)
			UDPHeader := flow.Packet().Add().Udp()
			UDPHeader.DstPort().Increment().SetStart(1).SetCount(50000).SetStep(1)
			UDPHeader.SrcPort().Increment().SetStart(1).SetCount(50000).SetStep(1)
		}
	}

	flow.Size().SetFixed(uint32(frameSize))
	flow.Rate().SetPps(uint64(frameRate))
	return flow
}

// GetIngressPort return expected ingress port info in Packetin.
func (traceroute *TraceroutePacketIO) GetIngressPort() string {
	return traceroute.IngressPort
}
