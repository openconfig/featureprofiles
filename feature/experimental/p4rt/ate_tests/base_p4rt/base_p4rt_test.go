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

package base_p4rt_test

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"strconv"
	"testing"

	"github.com/cisco-open/go-p4/p4rt_client"
	"github.com/cisco-open/go-p4/utils"
	"github.com/openconfig/featureprofiles/feature/experimental/p4rt/wbb"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/ygot/ygot"
	p4_v1 "github.com/p4lang/p4runtime/go/p4/v1"
)

type testArgs struct {
	ctx     context.Context
	client1 *p4rt_client.P4RTClient
	client2 *p4rt_client.P4RTClient
	dut     *ondatra.DUTDevice
	ate     *ondatra.ATEDevice
	top     *ondatra.ATETopology
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	ipv4PrefixLen = 30
)

var (
	// Path to the p4Info file for sending it with SetFwdPipelineConfig
	p4InfoFile  = flag.String("p4info_file_location", "../../wbb.p4info.pb.txt", "Path to the p4info file.")
	streamName1 = "p4rt1"
	streamName2 = "p4rt2"

	//Enter Component name used as string type
	comp1name = flag.String("p4rt_node_name1", "FPC0:NPU0", "component name for P4RT Node1")
	comp2name = flag.String("p4rt_node_name2", "FPC1:NPU0", "component name for P4RT Node2")

	electionId = uint64(100)
	//Enter the P4RT openconfig node-id and P4RT port-id to be configured in DUT and for client connection
	deviceId1 = uint64(100)
	deviceId2 = uint64(200)

	portId1 = uint32(2100)
	portId2 = uint32(3100)

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

// configInterfaceDUT configures the interface with the Addrs.
func configInterfaceDUT(i *telemetry.Interface, a *attrs.Attributes, p4rtid uint32) *telemetry.Interface {
	i.Description = ygot.String(a.Desc)
	i.Type = telemetry.IETFInterfaces_InterfaceType_ethernetCsmacd
	i.Id = ygot.Uint32(p4rtid)
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

// configComponentDUT configures the component with NodeId
func configComponentDUT(c *telemetry.Component, nodeid uint64) *telemetry.Component {
	ic := c.GetOrCreateIntegratedCircuit()
	ic.NodeId = ygot.Uint64(nodeid)

	c.IntegratedCircuit = ic
	return c

}

// configureDUT configures ports on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice, ports []attrs.Attributes, components []string, nodeids []uint64, p4rtids []uint32) {
	//create a for loop to configure for the given number of ports
	d := dut.Config()
	for n, port := range ports {
		p_name := "port" + strconv.Itoa(n+1)
		p := dut.Port(t, p_name)
		i := &telemetry.Interface{Name: ygot.String(p.Name())}
		d.Interface(p.Name()).Replace(t, configInterfaceDUT(i, &port, p4rtids[n]))
		fptest.LogYgot(t, fmt.Sprintf("%s to Replace()", p), d.Interface(p.Name()), configInterfaceDUT(i, &port, p4rtids[n]))
	}

	for n, comp := range components {
		c := &telemetry.Component{Name: ygot.String(comp)}
		d.Component(comp).Replace(t, configComponentDUT(c, nodeids[n]))
		fptest.LogYgot(t, fmt.Sprintf("%s to Replace()", comp), d.Component(comp), configComponentDUT(c, nodeids[n]))
	}

}

// ATE configuration with IP address
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

// setupP4RTClient sends client arbitration message for both clients.
// then sends setforwordingpipelineconfig for both clients, compare the P4Info
func setupP4RTClient(ctx context.Context, args *testArgs) error {
	// Setup p4rt-client stream parameters for both clients
	streamParameter1 := p4rt_client.P4RTStreamParameters{
		Name:        streamName1,
		DeviceId:    deviceId1,
		ElectionIdH: uint64(0),
		ElectionIdL: electionId,
	}
	streamParameter2 := p4rt_client.P4RTStreamParameters{
		Name:        streamName2,
		DeviceId:    deviceId2,
		ElectionIdH: uint64(0),
		ElectionIdL: electionId,
	}
	streamlist := []p4rt_client.P4RTStreamParameters{streamParameter1, streamParameter2}

	// Send ClientArbitration message on both clients.
	clients := []*p4rt_client.P4RTClient{args.client1, args.client2}
	for index, client := range clients {
		if client != nil {
			client.StreamChannelCreate(&streamlist[index])
			if err := client.StreamChannelSendMsg(&streamlist[index].Name, &p4_v1.StreamMessageRequest{
				Update: &p4_v1.StreamMessageRequest_Arbitration{
					Arbitration: &p4_v1.MasterArbitrationUpdate{
						DeviceId: streamlist[index].DeviceId,
						ElectionId: &p4_v1.Uint128{
							High: streamlist[index].ElectionIdH,
							Low:  streamlist[index].ElectionIdL,
						},
					},
				},
			}); err != nil {
				return errors.New("Errors seen when sending ClientArbitration message.")
			}
			if _, _, arbErr := client.StreamChannelGetArbitrationResp(&streamlist[index].Name, 1); arbErr != nil {
				return errors.New("Errors seen in ClientArbitration response.")
			}
		}
	}

	// Load p4info file.
	p4Info, err := utils.P4InfoLoad(p4InfoFile)
	if err != nil {
		return errors.New("Errors seen when loading p4info file.")
	}
	setp4Info, _ := json.Marshal(p4Info)

	// Send SetForwardingPipelineConfig for p4rt leader client.
	fmt.Println("Sending SetForwardingPipelineConfig for both clients")
	deviceid_list := []uint64{deviceId1, deviceId2}
	for index, client := range clients {
		if err := client.SetForwardingPipelineConfig(&p4_v1.SetForwardingPipelineConfigRequest{
			DeviceId:   deviceid_list[index],
			ElectionId: &p4_v1.Uint128{High: uint64(0), Low: electionId},
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
	}

	// Receive GetForwardingPipelineConfig
	for index, client := range clients {
		resp, err := client.GetForwardingPipelineConfig(&p4_v1.GetForwardingPipelineConfigRequest{
			DeviceId:     deviceid_list[index],
			ResponseType: p4_v1.GetForwardingPipelineConfigRequest_P4INFO_AND_COOKIE,
		})
		if err != nil {
			return errors.New("Errors seen when sending SetForwardingPipelineConfig.")
		}
		// Compare P4Info from GetForwardingPipelineConfig and SetForwardingPipelineConfig
		getp4Info, _ := json.Marshal(resp.Config.P4Info)
		if string(setp4Info) == string(getp4Info) {
			fmt.Println("P4info matches fine for client", index)
		} else {
			err := fmt.Errorf("P4info does not match for client %d", index)
			fmt.Println(err.Error())
		}
	}
	return nil
}

// Function to compare and check if the expected table is present in RPC ReadResponse
func verifyReadReceiveMatch(expected_update []*p4_v1.Update, received_entry *p4_v1.ReadResponse) error {

	matches := 0
	for i, table := range received_entry.Entities {
		if table.Entity == expected_update[i].Entity.Entity {
			matches += 1
		}
	}
	if matches == len(expected_update) {
		fmt.Println("Table match succesful")
		return nil
	} else {
		err := fmt.Errorf("P4info does not match")
		fmt.Println(err.Error())
		return errors.New("match unsuccesful")
	}
}

// TestP4rtConnect connects to the P4Runtime server over grpc
// It then calls setupP4RTClient which sets the arbitration request and sends SetForwardingPipelineConfig with P4Info
func TestP4rtConnect(t *testing.T) {

	dut := ondatra.DUT(t, "dut")
	ctx := context.Background()
	ate := ondatra.ATE(t, "ate")

	// configure DUT with P4RT node-id and ids on different FAPs
	configureDUT(t, dut, []attrs.Attributes{dutPort1, dutPort2}, []string{*comp1name, *comp2name}, []uint64{deviceId1, deviceId2}, []uint32{portId1, portId2})

	top := configureATE(t, ate)

	// Setup two different clients for different FAPs
	client1 := p4rt_client.P4RTClient{}
	if err := client1.P4rtClientSet(dut.RawAPIs().P4RT(t)); err != nil {
		t.Fatalf("Could not initialize p4rt client: %v", err)
	}

	client2 := p4rt_client.P4RTClient{}
	if err := client2.P4rtClientSet(dut.RawAPIs().P4RT(t)); err != nil {
		t.Fatalf("Could not initialize p4rt client: %v", err)
	}

	args := &testArgs{
		ctx:     ctx,
		client1: &client1,
		client2: &client2,
		dut:     dut,
		ate:     ate,
		top:     top,
	}

	if err := setupP4RTClient(ctx, args); err != nil {
		t.Fatalf("Could not setup p4rt client: %v", err)
	}

	// RPC Write to write the table entries to the P4RT Server
	deviceid_list := []uint64{deviceId1, deviceId2}
	clients := []*p4rt_client.P4RTClient{args.client1, args.client2}
	for index, client := range clients {
		err := client.Write(&p4_v1.WriteRequest{
			DeviceId:   deviceid_list[index],
			ElectionId: &p4_v1.Uint128{High: uint64(0), Low: electionId},
			Updates: wbb.ACLWbbIngressTableEntryGet([]*wbb.ACLWbbIngressTableEntryInfo{
				{
					Type:          p4_v1.Update_INSERT,
					EtherType:     0x6007,
					EtherTypeMask: 0xFFFF,
				},
				{
					Type:          p4_v1.Update_INSERT,
					EtherType:     0x88cc,
					EtherTypeMask: 0xFFFF,
				},
				{
					Type:    p4_v1.Update_INSERT,
					IsIpv4:  0x1,
					TTL:     0x1,
					TTLMask: 0xFF,
				},
			}),
			Atomicity: p4_v1.WriteRequest_CONTINUE_ON_ERROR,
		})
		if err != nil {
			countOK, countNotOK, errDetails := p4rt_client.P4RTWriteErrParse(err)
			t.Errorf("Write Partial Errors %d/%d: %s", countOK, countNotOK, errDetails)
		} else {
			t.Logf("RPC Write Success for client %d", index)
		}
	}

	nomatch := 0 // To count no matches for Table entries
	// Receive read response
	for index, client := range clients {
		rStream, rErr := client.Read(&p4_v1.ReadRequest{
			DeviceId: deviceid_list[index],
			Entities: []*p4_v1.Entity{
				&p4_v1.Entity{
					Entity: &p4_v1.Entity_TableEntry{},
				},
			},
		})
		if rErr != nil {
			t.Fatalf("Received error")
		}

		readResp, respErr := rStream.Recv()
		if respErr != nil {
			t.Fatalf("Read Response Err: %s", respErr)
		} else {
			t.Logf("Read Response success")
		}
		t.Logf("Verify Read response for client%d", index)

		// Construct expected table for GDP to match with received table entry
		expected_update := wbb.ACLWbbIngressTableEntryGet([]*wbb.ACLWbbIngressTableEntryInfo{
			{
				Type:          p4_v1.Update_INSERT,
				EtherType:     0x6007,
				EtherTypeMask: 0xFFFF,
			},
		})

		if err := verifyReadReceiveMatch(expected_update, readResp); err != nil {
			t.Errorf("Table entry for GDP %s", err)
			nomatch += 1
		}

		// Construct expected table for LLDP to match with received table entry
		expected_update = wbb.ACLWbbIngressTableEntryGet([]*wbb.ACLWbbIngressTableEntryInfo{
			{
				Type:          p4_v1.Update_INSERT,
				EtherType:     0x88cc,
				EtherTypeMask: 0xFFFF,
			},
		})
		if err := verifyReadReceiveMatch(expected_update, readResp); err != nil {
			t.Errorf("Table entry for LLDP %s", err)
			nomatch += 1
		}

		// Construct expected table for traceroute to match with received table entry
		expected_update = wbb.ACLWbbIngressTableEntryGet([]*wbb.ACLWbbIngressTableEntryInfo{
			{
				Type:    p4_v1.Update_INSERT,
				IsIpv4:  0x1,
				TTL:     0x1,
				TTLMask: 0xFF,
			},
		})
		if err := verifyReadReceiveMatch(expected_update, readResp); err != nil {
			t.Errorf("Table entry for traceroute %s", err)
			nomatch += 1
		}
	}
	if nomatch > 0 {
		t.Fatalf("Table entry matches failed")
	}
}
