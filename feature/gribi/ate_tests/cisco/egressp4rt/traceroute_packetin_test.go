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
// package to test P4RT with traceroute traffic of IPV4 and IPV6 with TTL/HopLimit as 0&1.
// go test -v . -testbed /root/ondatra/featureprofiles/topologies/atedut_2.testbed -binding /root/ondatra/featureprofiles/topologies/atedut_2.binding -outputs_dir logs

package egressp4rt_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"flag"

	"github.com/cisco-open/go-p4/p4rt_client"
	//"github.com/openconfig/featureprofiles/internal/cisco/ha/utils"

	"github.com/cisco-open/go-p4/utils"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/p4rtutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
	p4_v1 "github.com/p4lang/p4runtime/go/p4/v1"
)

var (
	p4InfoFile = flag.String("p4info_file_location", "../../wbb.p4info.pb.txt", "Path to the p4info file.")
	//streamName       = "p4rt"
	stream           = "p4rt"
	stream2          = "p4rt2"
	tracerouteSrcMAC = "00:01:00:02:00:03"
	deviceId         = uint64(1)
	deviceId2        = uint64(2)

	portId                = uint32(10)
	electionId            = uint64(100)
	METADATA_INGRESS_PORT = uint32(1)
	METADATA_EGRESS_PORT  = uint32(2)
	TTL1                  = uint8(1)
	HopLimit1             = uint8(1)
	TTL0                  = uint8(0)
	HopLimit0             = uint8(0)
	ipv4PrefixLen         = uint8(30)
	ipv6PrefixLen         = uint8(126)
)

var (
	P4rtNode, P4rtNode2 string
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// setupP4RTClient sends client arbitration message for both leader and follower clients,
// then sends setforwordingpipelineconfig with leader client.
func setupP4RTClient(args *testArgs, deviceID uint64, streamName string) error {
	// Setup p4rt-client stream parameters
	streamParameter := p4rt_client.P4RTStreamParameters{
		Name:        streamName,
		DeviceId:    deviceID,
		ElectionIdH: uint64(0),
		ElectionIdL: electionId,
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
		ElectionId: &p4_v1.Uint128{High: uint64(0), Low: electionId},
		Action:     p4_v1.SetForwardingPipelineConfigRequest_VERIFY_AND_COMMIT,
		Config: &p4_v1.ForwardingPipelineConfig{
			P4Info: p4Info,
			Cookie: &p4_v1.ForwardingPipelineConfig_Cookie{
				Cookie: 159,
			},
		},
	}); err != nil {
		return errors.New("Errors seen when sending SetForwardingPipelineConfig.")
	}
	return nil
}

// getTracerouteParameter returns Traceroute related parameters for testPacketIn testcase.
func getTracerouteParameter() PacketIO {
	return &TraceroutePacketIO{
		PacketIOPacket: PacketIOPacket{
			TTL:      &TTL1,
			HopLimit: &HopLimit1,
		},
		IngressPort: fmt.Sprint(portId),
		EgressPort:  fmt.Sprint(portId + 1),
	}
}

func TestEgressp4rt(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ctx := context.Background()

	// Configure the DUT
	baseconfig(t)

	nodes := p4rtutils.P4RTNodesByPort(t, dut)
	P4rtNode, ok := nodes["port1"]
	if !ok {
		t.Fatal("Couldn't find P4RT Node for port: port1")
	}
	t.Logf("Configuring P4RT Node: %s", P4rtNode)
	c := oc.Component{}
	c.Name = ygot.String(P4rtNode)
	c.IntegratedCircuit = &oc.Component_IntegratedCircuit{}
	c.IntegratedCircuit.NodeId = ygot.Uint64(deviceId)
	gnmi.Replace(t, dut, gnmi.OC().Component(P4rtNode).Config(), &c)

	///deviceid2
	P4rtNode2, ok := nodes["port3"]
	fmt.Println("P4rt nodes")
	fmt.Println(P4rtNode)
	fmt.Println(P4rtNode2)
	if !ok {
		t.Fatal("Couldn't find P4RT Node for port: port3")
	}
	if P4rtNode != P4rtNode2 {
		t.Logf("Configuring P4RT Node: %s", P4rtNode2)
		c2 := oc.Component{}
		c2.Name = ygot.String(P4rtNode2)
		c2.IntegratedCircuit = &oc.Component_IntegratedCircuit{}
		c2.IntegratedCircuit.NodeId = ygot.Uint64(deviceId2)
		gnmi.Replace(t, dut, gnmi.OC().Component(P4rtNode2).Config(), &c2)
	}

	// Configure the ATE
	ate := ondatra.ATE(t, "ate")
	top := configureATE(t, ate)
	top.Push(t).StartProtocols(t)

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

	if err := setupP4RTClient(args, deviceId, stream); err != nil {
		t.Fatalf("Could not setup p4rt client: %v", err)
	}
	var deviceSet bool
	if P4rtNode != P4rtNode2 {
		if err := setupP4RTClient(args, deviceId2, stream2); err != nil {
			t.Fatalf("Could not setup p4rt client: %v", err)
		}
		deviceSet = true
	} else {
		deviceSet = false
	}

	args.packetIO = getTracerouteParameter()
	var srcport string
	for j := 1; j <= 2; j++ {
		if j == 1 || (j == 2 && deviceSet) {
			if j == 1 {
				srcport = "port1"
			} else if j == 2 && deviceSet {
				args.interfaceaction(t, "port1", false)
				srcport = "port3"

			}
			if j == 1 {
				testWithDCUnoptimized(ctx, t, args, true, false, "", "IpinIpDC", deviceSet, srcport)
				testWithDCUnoptimized(ctx, t, args, false, false, "", "Ipv6inIpDC", deviceSet, srcport)

				testWithDCUnoptimized(ctx, t, args, true, false, "", "IpinIpTcpDC", deviceSet, srcport, &TOptions{ptcp: 4})
				testWithDCUnoptimized(ctx, t, args, true, false, "", "IpinIpUdpDC", deviceSet, srcport, &TOptions{pudp: 8})

				testWithDCUnoptimized(ctx, t, args, false, false, "", "Ipv6inIpUDPDC", deviceSet, srcport, &TOptions{ptcp: 4})
				testWithDCUnoptimized(ctx, t, args, false, false, "", "Ipv6inIpUDPDC", deviceSet, srcport, &TOptions{pudp: 8})

				testWithDCUnoptimized(ctx, t, args, true, false, "flap1", "IpinIpDCprimarychange", deviceSet, srcport)
				args.interfaceaction(t, "port2", true)

				testWithDCUnoptimized(ctx, t, args, false, false, "flap1", "Ipv6inIpDCprimarychange", deviceSet, srcport)
				args.interfaceaction(t, "port2", true)

				testWithDCUnoptimized(ctx, t, args, true, false, "flap1", "IpinIpTcpDCprimarychange", deviceSet, srcport, &TOptions{ptcp: 4})
				args.interfaceaction(t, "port2", true)

				testWithDCUnoptimized(ctx, t, args, true, false, "flap1", "IpinIpUdpDCprimarychange", deviceSet, srcport, &TOptions{pudp: 8})
				args.interfaceaction(t, "port2", true)

				testWithDCUnoptimized(ctx, t, args, false, false, "flap1", "Ipv6inIpTcpprimarychange", deviceSet, srcport, &TOptions{ptcp: 4})
				args.interfaceaction(t, "port2", true)

				testWithDCUnoptimized(ctx, t, args, false, false, "flap1", "Ipv6inIpUDPprimarychange", deviceSet, srcport, &TOptions{pudp: 8})
				args.interfaceaction(t, "port2", true)

				testWithDCUnoptimized(ctx, t, args, true, false, "flap", "IpinIpDCfrr", deviceSet, srcport)
				args.interfaceaction(t, "port2", true)
				args.interfaceaction(t, "port4", true)
				args.interfaceaction(t, "port6", true)
				args.interfaceaction(t, "port8", true)

				testWithDCUnoptimized(ctx, t, args, false, false, "flap", "Ipv6inIpDCfrr", deviceSet, srcport)
				args.interfaceaction(t, "port2", true)
				args.interfaceaction(t, "port4", true)
				args.interfaceaction(t, "port6", true)
				args.interfaceaction(t, "port8", true)
				testWithDCUnoptimized(ctx, t, args, true, false, "flap", "IpinIpTcpDCfrr", deviceSet, srcport, &TOptions{ptcp: 4})
				args.interfaceaction(t, "port2", true)
				args.interfaceaction(t, "port4", true)
				args.interfaceaction(t, "port6", true)
				args.interfaceaction(t, "port8", true)
				testWithDCUnoptimized(ctx, t, args, true, false, "flap", "IpinIpUdpDCfrr", deviceSet, srcport, &TOptions{pudp: 8})
				args.interfaceaction(t, "port2", true)
				args.interfaceaction(t, "port4", true)
				args.interfaceaction(t, "port6", true)
				args.interfaceaction(t, "port8", true)
				testWithDCUnoptimized(ctx, t, args, false, false, "flap", "Ipv6inIpTcpDCfrr", deviceSet, srcport, &TOptions{ptcp: 4})
				args.interfaceaction(t, "port2", true)
				args.interfaceaction(t, "port4", true)
				args.interfaceaction(t, "port6", true)
				args.interfaceaction(t, "port8", true)
				testWithDCUnoptimized(ctx, t, args, false, false, "flap", "Ipv6inIpUDPDCfrr", deviceSet, srcport, &TOptions{pudp: 8})
				args.interfaceaction(t, "port2", true)
				args.interfaceaction(t, "port4", true)
				args.interfaceaction(t, "port6", true)
				args.interfaceaction(t, "port8", true)

				//Encap
				testWithDCUnoptimized(ctx, t, args, true, true, "", "Ip", deviceSet, srcport)

				testWithDCUnoptimized(ctx, t, args, false, true, "", "Ipv6", deviceSet, srcport)

				testWithDCUnoptimized(ctx, t, args, true, true, "", "Iptcp", deviceSet, srcport, &TOptions{ptcp: 4})

				testWithDCUnoptimized(ctx, t, args, true, true, "", "Ipudp", deviceSet, srcport, &TOptions{pudp: 8})

				testWithDCUnoptimized(ctx, t, args, false, true, "", "Ipv6tcp", deviceSet, srcport, &TOptions{ptcp: 4})
				testWithDCUnoptimized(ctx, t, args, false, true, "", "Ipv6udp", deviceSet, srcport, &TOptions{pudp: 8})
				testWithDCUnoptimized(ctx, t, args, true, true, "flap1", "IpPrimaryChange", deviceSet, srcport)
				args.interfaceaction(t, "port2", true)

				testWithDCUnoptimized(ctx, t, args, false, true, "flap1", "Ipv6PrimaryChange", deviceSet, srcport)
				args.interfaceaction(t, "port2", true)

				testWithDCUnoptimized(ctx, t, args, true, true, "flap1", "IptcpPrimaryChange", deviceSet, srcport, &TOptions{ptcp: 4})
				args.interfaceaction(t, "port2", true)

				testWithDCUnoptimized(ctx, t, args, true, true, "flap1", "IpudpPrimaryChange", deviceSet, srcport, &TOptions{pudp: 8})
				args.interfaceaction(t, "port2", true)

				testWithDCUnoptimized(ctx, t, args, false, true, "flap1", "Ipv6tcpPrimaryChange", deviceSet, srcport, &TOptions{ptcp: 4})
				args.interfaceaction(t, "port2", true)

				testWithDCUnoptimized(ctx, t, args, false, true, "flap1", "Ipv6udpPrimaryChange", deviceSet, srcport, &TOptions{pudp: 8})
				args.interfaceaction(t, "port2", true)

				testWithDCUnoptimized(ctx, t, args, true, true, "flap", "Ipfrr", deviceSet, srcport)
				args.interfaceaction(t, "port2", true)
				args.interfaceaction(t, "port4", true)
				args.interfaceaction(t, "port6", true)
				args.interfaceaction(t, "port8", true)

				testWithDCUnoptimized(ctx, t, args, false, true, "flap", "Ipv6frr", deviceSet, srcport)
				args.interfaceaction(t, "port2", true)
				args.interfaceaction(t, "port4", true)
				args.interfaceaction(t, "port6", true)
				args.interfaceaction(t, "port8", true)

				testWithDCUnoptimized(ctx, t, args, true, true, "flap", "Iptcpfrr", deviceSet, srcport, &TOptions{ptcp: 4})
				args.interfaceaction(t, "port2", true)
				args.interfaceaction(t, "port4", true)
				args.interfaceaction(t, "port6", true)
				args.interfaceaction(t, "port8", true)
				testWithDCUnoptimized(ctx, t, args, true, true, "flap", "Ipudpfrr", deviceSet, srcport, &TOptions{pudp: 8})
				args.interfaceaction(t, "port2", true)
				args.interfaceaction(t, "port4", true)
				args.interfaceaction(t, "port6", true)
				args.interfaceaction(t, "port8", true)

				testWithDCUnoptimized(ctx, t, args, false, true, "flap", "Ipv6tcpfrr", deviceSet, srcport, &TOptions{ptcp: 4})
				args.interfaceaction(t, "port2", true)
				args.interfaceaction(t, "port4", true)
				args.interfaceaction(t, "port6", true)
				args.interfaceaction(t, "port8", true)
				testWithDCUnoptimized(ctx, t, args, false, true, "flap", "Ipv6udpfrr", deviceSet, srcport, &TOptions{pudp: 8})
				args.interfaceaction(t, "port2", true)
				args.interfaceaction(t, "port4", true)
				args.interfaceaction(t, "port6", true)
				args.interfaceaction(t, "port8", true)
				t.Logf("Encap frrss done")
				//pop gate
				testWithPoPUnoptimized(ctx, t, args, true, 5, "", "IpinIppop", deviceSet, srcport)
				testWithPoPUnoptimized(ctx, t, args, true, 0, "", "IpinIptcppop", deviceSet, srcport, &TOptions{ptcp: 4})
				testWithPoPUnoptimized(ctx, t, args, true, 0, "", "IpinIpudppop", deviceSet, srcport, &TOptions{pudp: 8})

				testWithPoPUnoptimized(ctx, t, args, false, 0, "", "Ipv6inIppop", deviceSet, srcport)
				testWithPoPUnoptimized(ctx, t, args, false, 0, "", "Ipv6inIptcppop", deviceSet, srcport, &TOptions{ptcp: 4})
				testWithPoPUnoptimized(ctx, t, args, false, 0, "", "Ipv6inIpv6udppop", deviceSet, srcport, &TOptions{pudp: 8})

				testWithPoPUnoptimized(ctx, t, args, true, 0, "flap1", "IpinIppopPrimaryChange", deviceSet, srcport)
				args.interfaceaction(t, "port2", true)

				testWithPoPUnoptimized(ctx, t, args, true, 0, "flap1", "IpinIptcppopPrimaryChange", deviceSet, srcport, &TOptions{ptcp: 4})
				args.interfaceaction(t, "port2", true)

				testWithPoPUnoptimized(ctx, t, args, true, 0, "flap1", "IpinIpudppopPrimaryChange", deviceSet, srcport, &TOptions{pudp: 8})
				args.interfaceaction(t, "port2", true)

				testWithPoPUnoptimized(ctx, t, args, false, 0, "flap1", "Ipv6inIppopPrimaryChange", deviceSet, srcport)
				args.interfaceaction(t, "port2", true)

				testWithPoPUnoptimized(ctx, t, args, false, 0, "flap1", "Ipv6inIptcppopPrimaryChange", deviceSet, srcport, &TOptions{ptcp: 4})
				args.interfaceaction(t, "port2", true)

				testWithPoPUnoptimized(ctx, t, args, false, 0, "flap1", "Ipv6inIpudppopPrimaryChange", deviceSet, srcport, &TOptions{pudp: 8})
				args.interfaceaction(t, "port2", true)

				testWithPoPUnoptimized(ctx, t, args, true, 0, "flap", "IpinIppopfrr", deviceSet, srcport)
				args.interfaceaction(t, "port2", true)
				args.interfaceaction(t, "port4", true)
				args.interfaceaction(t, "port6", true)
				args.interfaceaction(t, "port8", true)
				testWithPoPUnoptimized(ctx, t, args, true, 0, "flap", "IpinIptcppopfrr", deviceSet, srcport, &TOptions{ptcp: 4})
				args.interfaceaction(t, "port2", true)
				args.interfaceaction(t, "port4", true)
				args.interfaceaction(t, "port6", true)
				args.interfaceaction(t, "port8", true)
				testWithPoPUnoptimized(ctx, t, args, true, 0, "flap", "IpinIpudppopfrr", deviceSet, srcport, &TOptions{pudp: 8})
				args.interfaceaction(t, "port2", true)
				args.interfaceaction(t, "port4", true)
				args.interfaceaction(t, "port6", true)
				args.interfaceaction(t, "port8", true)

				testWithPoPUnoptimized(ctx, t, args, false, 0, "flap", "Ipv6inIppopfrr", deviceSet, srcport)
				args.interfaceaction(t, "port2", true)
				args.interfaceaction(t, "port4", true)
				args.interfaceaction(t, "port6", true)
				args.interfaceaction(t, "port8", true)
				testWithPoPUnoptimized(ctx, t, args, false, 0, "flap", "Ipv6inIptcppopfrr", deviceSet, srcport, &TOptions{ptcp: 4})
				args.interfaceaction(t, "port2", true)
				args.interfaceaction(t, "port4", true)
				args.interfaceaction(t, "port6", true)
				args.interfaceaction(t, "port8", true)
				testWithPoPUnoptimized(ctx, t, args, false, 6, "flap", "Ipv6inIpudppopfrr", deviceSet, srcport, &TOptions{pudp: 8})
				args.interfaceaction(t, "port2", true)
				args.interfaceaction(t, "port4", true)
				args.interfaceaction(t, "port6", true)
				args.interfaceaction(t, "port8", true)

				//regionalization
				testWithregionalization(ctx, t, args, true, false, "", "IpinIpDCRegionalization", deviceSet, srcport)
				testWithregionalization(ctx, t, args, false, false, "", "Ipv6inIpDCRegionalization", deviceSet, srcport)

				testWithregionalization(ctx, t, args, true, false, "", "IpinIpTcpDCRegionalization", deviceSet, srcport)
				testWithregionalization(ctx, t, args, true, false, "", "IpinIpUdpDCRegionalization", deviceSet, srcport)

				testWithregionalization(ctx, t, args, false, false, "", "Ipv6inIpTcpRegionalization", deviceSet, srcport)
				testWithregionalization(ctx, t, args, false, false, "", "Ipv6inIpUDPRegionalization", deviceSet, srcport)
				args.interfaceaction(t, "port1", true)
			} else if j == 2 {
				testWithDCUnoptimized(ctx, t, args, true, false, "", "IpinIpDC", deviceSet, srcport)
				testWithDCUnoptimized(ctx, t, args, false, false, "", "Ipv6inIpDC", deviceSet, srcport)

				testWithDCUnoptimized(ctx, t, args, true, false, "", "IpinIpTcpDC", deviceSet, srcport, &TOptions{ptcp: 4})
				testWithDCUnoptimized(ctx, t, args, true, false, "", "IpinIpUdpDC", deviceSet, srcport, &TOptions{pudp: 8})

				testWithDCUnoptimized(ctx, t, args, false, false, "", "Ipv6inIpUDPDC", deviceSet, srcport, &TOptions{ptcp: 4})
				testWithDCUnoptimized(ctx, t, args, false, false, "", "Ipv6inIpUDPDC", deviceSet, srcport, &TOptions{pudp: 8})

				testWithDCUnoptimized(ctx, t, args, true, false, "flap1", "IpinIpTcpDCprimarychange", deviceSet, srcport, &TOptions{ptcp: 4})
				args.interfaceaction(t, "port2", true)

				testWithDCUnoptimized(ctx, t, args, false, false, "flap1", "Ipv6inIpUDPprimarychange", deviceSet, srcport, &TOptions{pudp: 8})
				args.interfaceaction(t, "port2", true)

				testWithDCUnoptimized(ctx, t, args, true, false, "flap", "IpinIpUdpDCfrr", deviceSet, srcport, &TOptions{pudp: 8})
				args.interfaceaction(t, "port2", true)
				args.interfaceaction(t, "port4", true)
				args.interfaceaction(t, "port6", true)
				args.interfaceaction(t, "port8", true)
				testWithDCUnoptimized(ctx, t, args, false, false, "flap", "Ipv6inIpUDPfrr", deviceSet, srcport, &TOptions{ptcp: 4})
				args.interfaceaction(t, "port2", true)
				args.interfaceaction(t, "port4", true)
				args.interfaceaction(t, "port6", true)
				args.interfaceaction(t, "port8", true)

				//pop gate
				testWithPoPUnoptimized(ctx, t, args, true, 5, "", "IpinIppop", deviceSet, srcport)
				testWithPoPUnoptimized(ctx, t, args, true, 0, "", "IpinIptcppop", deviceSet, srcport, &TOptions{ptcp: 4})
				testWithPoPUnoptimized(ctx, t, args, true, 0, "", "IpinIpudppop", deviceSet, srcport, &TOptions{pudp: 8})

				testWithPoPUnoptimized(ctx, t, args, false, 0, "", "Ipv6inIppop", deviceSet, srcport)
				testWithPoPUnoptimized(ctx, t, args, false, 0, "", "Ipv6inIptcppop", deviceSet, srcport, &TOptions{ptcp: 4})
				testWithPoPUnoptimized(ctx, t, args, false, 0, "", "Ipv6inIpv6udppop", deviceSet, srcport, &TOptions{pudp: 8})

				testWithPoPUnoptimized(ctx, t, args, true, 0, "flap", "IpinIpudppopfrr", deviceSet, srcport, &TOptions{pudp: 8})
				args.interfaceaction(t, "port2", true)
				args.interfaceaction(t, "port4", true)
				args.interfaceaction(t, "port6", true)
				args.interfaceaction(t, "port8", true)

				testWithPoPUnoptimized(ctx, t, args, false, 0, "flap", "Ipv6inIptcppopfrr", deviceSet, srcport, &TOptions{ptcp: 4})
				args.interfaceaction(t, "port2", true)
				args.interfaceaction(t, "port4", true)
				args.interfaceaction(t, "port6", true)
				args.interfaceaction(t, "port8", true)

				//regionalization

				testWithregionalization(ctx, t, args, true, false, "", "IpinIpTcpDCRegionalization", deviceSet, srcport)
				testWithregionalization(ctx, t, args, true, false, "", "IpinIpUdpDCRegionalization", deviceSet, srcport)

				testWithregionalization(ctx, t, args, false, false, "", "Ipv6inIpTcpRegionalization", deviceSet, srcport)
				testWithregionalization(ctx, t, args, false, false, "", "Ipv6inIpUDPRegionalization", deviceSet, srcport)

			}
		}
	}
}

type TraceroutePacketIO struct {
	PacketIOPacket
	IngressPort string
	EgressPort  string
}

// GetTableEntry creates p4rtutils acl entry related to Traceroute protocol.
func (traceroute *TraceroutePacketIO) GetTableEntry(delete bool, IsIpv4 bool) []*p4rtutils.ACLWbbIngressTableEntryInfo {
	if IsIpv4 {
		actionType := p4_v1.Update_INSERT
		if delete {
			actionType = p4_v1.Update_DELETE
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
	} else {
		actionType := p4_v1.Update_INSERT
		if delete {
			actionType = p4_v1.Update_DELETE
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
}

// GetPacketTemplate returns expected packets in PacketIn.
func (traceroute *TraceroutePacketIO) GetPacketTemplate() *PacketIOPacket {
	return &traceroute.PacketIOPacket
}

func (traceroute *TraceroutePacketIO) GetTrafficFlow(ate *ondatra.ATEDevice, isIpv4 bool, TTL uint8, frameSize uint32, frameRate uint64) []*ondatra.Flow {
	ethHeader := ondatra.NewEthernetHeader().WithSrcAddress(tracerouteSrcMAC)
	ipv4Header := ondatra.NewIPv4Header().WithSrcAddress(atePort1.IPv4).WithDstAddress(atePort2.IPv4).WithTTL(uint8(TTL)) //ttl=1 is traceroute traffic
	ipv6Header := ondatra.NewIPv6Header().WithSrcAddress(atePort1.IPv6).WithDstAddress(atePort2.IPv6).WithHopLimit(uint8(TTL))
	if isIpv4 {
		flow := ate.Traffic().NewFlow("IP4").WithFrameSize(frameSize).WithFrameRateFPS(frameRate).WithHeaders(ethHeader, ipv4Header)
		return []*ondatra.Flow{flow}
	} else {
		flow := ate.Traffic().NewFlow("IP6").WithFrameSize(frameSize).WithFrameRateFPS(frameRate).WithHeaders(ethHeader, ipv6Header)
		return []*ondatra.Flow{flow}
	}
}

// GetEgressPort returns expected egress port info in Packetin.
func (traceroute *TraceroutePacketIO) GetEgressPort() string {
	return traceroute.EgressPort
}

// GetIngressPort return expected ingress port info in Packetin.
func (traceroute *TraceroutePacketIO) GetIngressPort() string {
	return traceroute.IngressPort
}
