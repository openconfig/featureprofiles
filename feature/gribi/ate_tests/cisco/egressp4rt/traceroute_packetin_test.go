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
	"time"

	"flag"

	"github.com/cisco-open/go-p4/p4rt_client"
	//"github.com/openconfig/featureprofiles/internal/cisco/ha/utils"

	"github.com/cisco-open/go-p4/utils"
	ciscoFlags "github.com/openconfig/featureprofiles/internal/cisco/flags"
	"github.com/openconfig/featureprofiles/internal/cisco/gribi"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/p4rtutils"
	gnps "github.com/openconfig/gnoi/system"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/testt"
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

// configInterfaceDUT configures the interface with the Addrs.
// func configInterfaceDUT(i *oc.Interface, a *attrs.Attributes, dut *ondatra.DUTDevice) *oc.Interface {
// 	i.Description = ygot.String(a.Desc)
// 	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
// 	if deviations.InterfaceEnabled(dut) {
// 		i.Enabled = ygot.Bool(true)
// 	}

// 	s := i.GetOrCreateSubinterface(0)
// 	s4 := s.GetOrCreateIpv4()
// 	if deviations.InterfaceEnabled(dut) {
// 		s4.Enabled = ygot.Bool(true)
// 	}
// 	s4a := s4.GetOrCreateAddress(a.IPv4)
// 	s4a.PrefixLength = ygot.Uint8(ipv4PrefixLen)

// 	s6 := s.GetOrCreateIpv6()
// 	if deviations.InterfaceEnabled(dut) {
// 		s6.Enabled = ygot.Bool(true)
// 	}
// 	s6.GetOrCreateAddress(a.IPv6).PrefixLength = ygot.Uint8(ipv6PrefixLen)

// 	return i
// }

// // configureDUT configures port1 and port2 on the DUT.
// func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
// 	d := gnmi.OC()

// 	p1 := dut.Port(t, "port1")
// 	i1 := &oc.Interface{Name: ygot.String(p1.Name()), Id: ygot.Uint32(portId)}
// 	gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), configInterfaceDUT(i1, &dutPort1, dut))

// 	p2 := dut.Port(t, "port2")
// 	i2 := &oc.Interface{Name: ygot.String(p2.Name()), Id: ygot.Uint32(portId + 1)}
// 	gnmi.Replace(t, dut, d.Interface(p2.Name()).Config(), configInterfaceDUT(i2, &dutPort2, dut))

// 	if deviations.ExplicitPortSpeed(dut) {
// 		fptest.SetPortSpeed(t, p1)
// 		fptest.SetPortSpeed(t, p2)
// 	}
// 	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
// 		fptest.AssignToNetworkInstance(t, dut, p1.Name(), deviations.DefaultNetworkInstance(dut), 0)
// 		fptest.AssignToNetworkInstance(t, dut, p2.Name(), deviations.DefaultNetworkInstance(dut), 0)
// 	}
// 	gnmi.Replace(t, dut, gnmi.OC().Lldp().Enabled().Config(), false)
// }

// // configureATE configures port1 and port2 on the ATE.
// func configureATE(t *testing.T, ate *ondatra.ATEDevice) *ondatra.ATETopology {
// 	top := ate.Topology().New()

// 	p1 := ate.Port(t, "port1")
// 	i1 := top.AddInterface(atePort1.Name).WithPort(p1)
// 	i1.IPv4().
// 		WithAddress(atePort1.IPv4CIDR()).
// 		WithDefaultGateway(dutPort1.IPv4)
// 	i1.IPv6().
// 		WithAddress(atePort1.IPv6CIDR()).
// 		WithDefaultGateway(dutPort1.IPv6)

// 	p2 := ate.Port(t, "port2")
// 	i2 := top.AddInterface(atePort2.Name).WithPort(p2)
// 	i2.IPv4().
// 		WithAddress(atePort2.IPv4CIDR()).
// 		WithDefaultGateway(dutPort2.IPv4)
// 	i2.IPv6().
// 		WithAddress(atePort2.IPv6CIDR()).
// 		WithDefaultGateway(dutPort2.IPv6)

// 	return top
// }

// configureDeviceIDs configures p4rt device-id on the DUT.
// func configureDeviceID(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice) {
// 	nodes := p4rtutils.P4RTNodesByPort(t, dut)
// 	P4rtNode, ok := nodes["port1"]
// 	if !ok {
// 		t.Fatal("Couldn't find P4RT Node for port: port1")
// 	}
// 	t.Logf("Configuring P4RT Node: %s", P4rtNode)
// 	c := oc.Component{}
// 	c.Name = ygot.String(P4rtNode)
// 	c.IntegratedCircuit = &oc.Component_IntegratedCircuit{}
// 	c.IntegratedCircuit.NodeId = ygot.Uint64(deviceId)
// 	gnmi.Replace(t, dut, gnmi.OC().Component(P4rtNode).Config(), &c)

// 	///deviceid2
// 	P4rtNode2, ok := nodes["port3"]
// 	fmt.Println("P4rt nodes")
// 	fmt.Println(P4rtNode)
// 	fmt.Println(P4rtNode2)
// 	if !ok {
// 		t.Fatal("Couldn't find P4RT Node for port: port3")
// 	}
// 	if P4rtNode != P4rtNode2 {
// 		t.Logf("Configuring P4RT Node: %s", P4rtNode2)
// 		c2 := oc.Component{}
// 		c2.Name = ygot.String(P4rtNode2)
// 		c2.IntegratedCircuit = &oc.Component_IntegratedCircuit{}
// 		c2.IntegratedCircuit.NodeId = ygot.Uint64(deviceId2)
// 		gnmi.Replace(t, dut, gnmi.OC().Component(P4rtNode2).Config(), &c2)
// 	}
// }

// setupP4RTClient sends client arbitration message for both leader and follower clients,
// then sends setforwordingpipelineconfig with leader client.
func setupP4RTClient(ctx context.Context, args *testArgs, deviceID uint64, streamName string) error {
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
func getTracerouteParameter(t *testing.T) PacketIO {
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
	//configureDUT(t, dut)
	baseconfig(t)

	// Configure P4RT device-id
	//configureDeviceID(ctx, t, dut)

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

	// args := &testArgs{
	// 	ctx:      ctx,
	// 	leader:   leader,
	// 	follower: follower,
	// 	dut:      dut,
	// 	ate:      ate,
	// 	top:      top,
	// }

	// type testArgs struct {
	// 	ctx      context.Context
	// 	leader   *p4rt_client.P4RTClient
	// 	follower *p4rt_client.P4RTClient
	// 	dut      *ondatra.DUTDevice
	// 	ate      *ondatra.ATEDevice
	// 	packetIO PacketIO
	// 	clientg  *gribi.Client
	// 	top      *ondatra.ATETopology
	// }

	args := &testArgs{
		ctx:      ctx,
		leader:   leader,
		follower: follower,
		dut:      dut,
		ate:      ate,
		top:      top,
	}

	if err := setupP4RTClient(ctx, args, deviceId, stream); err != nil {
		t.Fatalf("Could not setup p4rt client: %v", err)
	}
	var deviceSet bool
	if P4rtNode != P4rtNode2 {
		fmt.Println(P4rtNode)
		fmt.Println("2nd deviceid setup")
		if err := setupP4RTClient(ctx, args, deviceId2, stream2); err != nil {
			t.Fatalf("Could not setup p4rt client: %v", err)
		}
		deviceSet = true
	} else {
		deviceSet = false
	}

	args.packetIO = getTracerouteParameter(t)
	fmt.Println("ooooo")
	fmt.Println(args.packetIO)
	for i := 1; i <= 2; i++ {
		if i == 2 {

			performrpfo(args.ctx, t, true)
			clientr := gribi.Client{
				DUT:                   args.dut,
				FibACK:                *ciscoFlags.GRIBIFIBCheck,
				Persistence:           true,
				InitialElectionIDLow:  1,
				InitialElectionIDHigh: 0,
			}
			if err := clientr.Start(t); err != nil {
				t.Logf("gRIBI Connection could not be established: %v\nRetrying...", err)
				if err = clientr.Start(t); err != nil {
					t.Fatalf("gRIBI Connection could not be established: %v", err)
				}
			}
			args.client = &clientr
		}
		var srcport string
		for j := 1; j <= 2; j++ {
			if j == 1 || (j == 2 && deviceSet) {
				if j == 1 {
					srcport = "port1"
				} else if j == 2 && deviceSet {
					args.interfaceaction(t, "port1", false)
					srcport = "port3"

				}
				fmt.Println("srcporttt")
				fmt.Println(srcport)
				// testWithDCUnoptimized(ctx, t, args, true, false, "", "IpinIpDC", deviceSet, srcport)
				testWithDCUnoptimized(ctx, t, args, false, false, "", "Ipv6inIpDC", deviceSet, srcport)

				testWithDCUnoptimized(ctx, t, args, true, false, "", "IpinIpTcpDC", deviceSet, srcport, &TOptions{ptcp: 4})
				testWithDCUnoptimized(ctx, t, args, true, false, "", "IpinIpUdpDC", deviceSet, srcport, &TOptions{pudp: 8})

				testWithDCUnoptimized(ctx, t, args, false, false, "", "Ipv6inIpUDP", deviceSet, srcport, &TOptions{ptcp: 4})
				testWithDCUnoptimized(ctx, t, args, false, false, "", "Ipv6inIpUDP", deviceSet, srcport, &TOptions{pudp: 8})

				//flap
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

				// //flap1

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
				testWithDCUnoptimized(ctx, t, args, false, false, "flap", "Ipv6inIpUDPfrr", deviceSet, srcport, &TOptions{ptcp: 4})
				args.interfaceaction(t, "port2", true)
				args.interfaceaction(t, "port4", true)
				args.interfaceaction(t, "port6", true)
				args.interfaceaction(t, "port8", true)
				testWithDCUnoptimized(ctx, t, args, false, false, "flap", "Ipv6inIpUDPfrr", deviceSet, srcport, &TOptions{pudp: 8})
				args.interfaceaction(t, "port2", true)
				args.interfaceaction(t, "port4", true)
				args.interfaceaction(t, "port6", true)
				args.interfaceaction(t, "port8", true)

				t.Logf("frrrss sone")
				//Encap
				testWithDCUnoptimized(ctx, t, args, true, true, "", "Ip", deviceSet, srcport)

				testWithDCUnoptimized(ctx, t, args, false, true, "", "Ipv6", deviceSet, srcport)

				testWithDCUnoptimized(ctx, t, args, true, true, "", "Iptcp", deviceSet, srcport, &TOptions{ptcp: 4})
				testWithDCUnoptimized(ctx, t, args, true, true, "", "Ipudp", deviceSet, srcport, &TOptions{pudp: 8})

				testWithDCUnoptimized(ctx, t, args, false, true, "", "Ipv6tcp", deviceSet, srcport, &TOptions{ptcp: 4})
				testWithDCUnoptimized(ctx, t, args, false, true, "", "Ipv6udp", deviceSet, srcport, &TOptions{pudp: 8})
				//flap1
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

				//flap2
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

				testWithregionalization(ctx, t, args, false, false, "", "Ipv6inIpUDPRegionalization", deviceSet, srcport)
				testWithregionalization(ctx, t, args, false, false, "", "Ipv6inIpUDPRegionalization", deviceSet, srcport)
				args.interfaceaction(t, "port1", true)
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

func performrpfo(ctx context.Context, t *testing.T, gribi_reconnect bool) {
	t.Helper()
	dut := ondatra.DUT(t, "dut")
	// supervisor info
	var supervisors []string
	active_state := gnmi.OC().Component(active_rp).Name().State()
	active := gnmi.Get(t, dut, active_state)
	standby_state := gnmi.OC().Component(standby_rp).Name().State()
	standby := gnmi.Get(t, dut, standby_state)
	supervisors = append(supervisors, active, standby)

	// find active and standby RP
	rpStandbyBeforeSwitch, rpActiveBeforeSwitch := components.FindStandbyRP(t, dut, supervisors)
	t.Logf("Detected activeRP: %v, standbyRP: %v", rpActiveBeforeSwitch, rpStandbyBeforeSwitch)

	// make sure standby RP is reach
	switchoverReady := gnmi.OC().Component(rpActiveBeforeSwitch).SwitchoverReady()
	gnmi.Await(t, dut, switchoverReady.State(), 30*time.Minute, true)
	t.Logf("SwitchoverReady().Get(t): %v", gnmi.Get(t, dut, switchoverReady.State()))
	if got, want := gnmi.Get(t, dut, switchoverReady.State()), true; got != want {
		t.Errorf("switchoverReady.Get(t): got %v, want %v", got, want)
	}
	// gnoiClient := dut.RawAPIs().GNOI(t)
	gnoiClient, err := dut.RawAPIs().BindingDUT().DialGNOI(context.Background())
	if err != nil {
		t.Fatalf("Error dialing gNOI: %v", err)
	}
	useNameOnly := deviations.GNOISubcomponentPath(dut)
	switchoverRequest := &gnps.SwitchControlProcessorRequest{
		ControlProcessor: components.GetSubcomponentPath(rpStandbyBeforeSwitch, useNameOnly),
	}
	t.Logf("switchoverRequest: %v", switchoverRequest)
	switchoverResponse, err := gnoiClient.System().SwitchControlProcessor(context.Background(), switchoverRequest)
	if err != nil {
		t.Fatalf("Failed to perform control processor switchover with unexpected err: %v", err)
	}
	t.Logf("gnoiClient.System().SwitchControlProcessor() response: %v, err: %v", switchoverResponse, err)

	want := rpStandbyBeforeSwitch
	got := ""
	if useNameOnly {
		got = switchoverResponse.GetControlProcessor().GetElem()[0].GetName()
	} else {
		got = switchoverResponse.GetControlProcessor().GetElem()[1].GetKey()["name"]
	}
	if got != want {
		t.Fatalf("switchoverResponse.GetControlProcessor().GetElem()[0].GetName(): got %v, want %v", got, want)
	}

	startSwitchover := time.Now()
	t.Logf("Wait for new active RP to boot up by polling the telemetry output.")
	for {
		var currentTime string
		t.Logf("Time elapsed %.2f seconds since switchover started.", time.Since(startSwitchover).Seconds())
		time.Sleep(30 * time.Second)
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			currentTime = gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State())
		}); errMsg != nil {
			t.Logf("Got testt.CaptureFatal errMsg: %s, keep polling ...", *errMsg)
		} else {
			t.Logf("RP switchover has completed successfully with received time: %v", currentTime)
			break
		}
		if got, want := uint64(time.Since(startSwitchover).Seconds()), uint64(900); got >= want {
			t.Fatalf("time.Since(startSwitchover): got %v, want < %v", got, want)
		}
	}
	t.Logf("RP switchover time: %.2f seconds", time.Since(startSwitchover).Seconds())

	rpStandbyAfterSwitch, rpActiveAfterSwitch := components.FindStandbyRP(t, dut, supervisors)
	t.Logf("Found standbyRP after switchover: %v, activeRP: %v", rpStandbyAfterSwitch, rpActiveAfterSwitch)

	if got, want := rpActiveAfterSwitch, rpStandbyBeforeSwitch; got != want {
		t.Errorf("Get rpActiveAfterSwitch: got %v, want %v", got, want)
	}
	if got, want := rpStandbyAfterSwitch, rpActiveBeforeSwitch; got != want {
		t.Errorf("Get rpStandbyAfterSwitch: got %v, want %v", got, want)
	}

	t.Log("Validate OC Switchover time/reason.")
	activeRP := gnmi.OC().Component(rpActiveAfterSwitch)
	if got, want := gnmi.Lookup(t, dut, activeRP.LastSwitchoverTime().State()).IsPresent(), true; got != want {
		t.Errorf("activeRP.LastSwitchoverTime().Lookup(t).IsPresent(): got %v, want %v", got, want)
	} else {
		t.Logf("Found activeRP.LastSwitchoverTime(): %v", gnmi.Get(t, dut, activeRP.LastSwitchoverTime().State()))
	}

	if got, want := gnmi.Lookup(t, dut, activeRP.LastSwitchoverReason().State()).IsPresent(), true; got != want {
		t.Errorf("activeRP.LastSwitchoverReason().Lookup(t).IsPresent(): got %v, want %v", got, want)
	} else {
		lastSwitchoverReason := gnmi.Get(t, dut, activeRP.LastSwitchoverReason().State())
		t.Logf("Found lastSwitchoverReason.GetDetails(): %v", lastSwitchoverReason.GetDetails())
		t.Logf("Found lastSwitchoverReason.GetTrigger().String(): %v", lastSwitchoverReason.GetTrigger().String())
	}
}
