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
	"errors"
	"flag"
	"fmt"
	"sort"
	"testing"

	"github.com/cisco-open/go-p4/p4rt_client"
	"github.com/cisco-open/go-p4/utils"
	"github.com/golang/glog"
	"github.com/google/go-cmp/cmp"
	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/feature/experimental/p4rt/internal/p4rtutils"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
	p4_v1 "github.com/p4lang/p4runtime/go/p4/v1"
	"google.golang.org/protobuf/testing/protocmp"
)

type testArgs struct {
	ctx     context.Context
	client1 *p4rt_client.P4RTClient
	client2 *p4rt_client.P4RTClient
	dut     *ondatra.DUTDevice
	ate     *ondatra.ATEDevice
	top     gosnappi.Config
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

	electionId = uint64(100)
	//Enter the P4RT openconfig node-id and P4RT port-id to be configured in DUT and for client connection
	deviceId1 = uint64(100)
	deviceId2 = uint64(200)

	portId = uint32(20)

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

// configureDeviceIDs configures p4rt device-id on the DUT.
func configureDeviceIDs(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	nodes := p4rtutils.P4RTNodesByPort(t, dut)
	deviceIDs := []uint64{deviceId1, deviceId2}

	for idx, p := range []string{"port1", "port2"} {
		if _, ok := nodes[p]; !ok {
			t.Fatalf("Couldn't find P4RT Node for port: %s", p)
		}
		t.Logf("Configuring P4RT Node: %s", nodes[p])
		c := oc.Component{}
		c.Name = ygot.String(nodes[p])
		c.IntegratedCircuit = &oc.Component_IntegratedCircuit{}
		c.IntegratedCircuit.NodeId = ygot.Uint64(deviceIDs[idx])
		gnmi.Replace(t, dut, gnmi.OC().Component(nodes[p]).Config(), &c)
	}
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

// configurePortId configures p4rt port-id on the DUT.
func configurePortId(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	ports := sortPorts(dut.Ports())
	for i, port := range ports {
		gnmi.Replace(t, dut, gnmi.OC().Interface(port.Name()).Type().Config(), oc.IETFInterfaces_InterfaceType_ethernetCsmacd)
		gnmi.Replace(t, dut, gnmi.OC().Interface(port.Name()).Id().Config(), uint32(i)+portId)
	}
}

// configureDUT configures port1 and port2 on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	d := gnmi.OC()

	p1 := dut.Port(t, "port1")
	i1 := &oc.Interface{Name: ygot.String(p1.Name())}
	gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), configInterfaceDUT(i1, &dutPort1))

	p2 := dut.Port(t, "port2")
	i2 := &oc.Interface{Name: ygot.String(p2.Name())}
	gnmi.Replace(t, dut, d.Interface(p2.Name()).Config(), configInterfaceDUT(i2, &dutPort2))

	if *deviations.ExplicitPortSpeed {
		fptest.SetPortSpeed(t, p1)
		fptest.SetPortSpeed(t, p2)
	}
	if *deviations.ExplicitInterfaceInDefaultVRF {
		fptest.AssignToNetworkInstance(t, dut, p1.Name(), *deviations.DefaultNetworkInstance, 0)
		fptest.AssignToNetworkInstance(t, dut, p2.Name(), *deviations.DefaultNetworkInstance, 0)
	}
}

// ATE configuration with IP address
func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	t.Helper()
	otg := ate.OTG()
	top := otg.NewConfig(t)

	p1 := ate.Port(t, "port1")
	atePort1.AddToOTG(top, p1, &dutPort1)

	p2 := ate.Port(t, "port2")
	atePort2.AddToOTG(top, p2, &dutPort2)

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
				return fmt.Errorf("errors seen when sending ClientArbitration message: %v", err)
			}
			if _, _, arbErr := client.StreamChannelGetArbitrationResp(&streamlist[index].Name, 1); arbErr != nil {
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
		if diff := cmp.Diff(&p4Info, resp.Config.P4Info, protocmp.Transform()); diff != "" {
			return fmt.Errorf("P4info diff (-want +got): \n%s", diff)
		}
	}
	return nil
}

// Function to compare and check if the expected table is present in RPC ReadResponse
func verifyReadReceiveMatch(expected_update []*p4_v1.Update, received_entry *p4_v1.ReadResponse) error {

	matches := 0
	for _, table := range received_entry.Entities {
		if diff := cmp.Diff(expected_update[0].Entity.Entity, table.Entity, protocmp.Transform()); diff != "" {
			glog.Errorf("Table entry diff (-want +got): \n%s", diff)
			continue
		}
		matches++
	}
	if matches == 0 {
		return errors.New("match unsuccesful")
	}
	return nil
}

// TestP4rtConnect connects to the P4Runtime server over grpc
// It then calls setupP4RTClient which sets the arbitration request and sends SetForwardingPipelineConfig with P4Info
func TestP4rtConnect(t *testing.T) {

	dut := ondatra.DUT(t, "dut")
	ctx := context.Background()
	ate := ondatra.ATE(t, "ate")

	// configure DUT with P4RT node-id and ids on different FAPs
	configureDUT(t, dut)
	configureDeviceIDs(ctx, t, dut)

	configurePortId(ctx, t, dut)
	top := configureATE(t, ate)
	ate.OTG().PushConfig(t, top)

	// Setup two different clients for different FAPs
	client1 := p4rt_client.NewP4RTClient(&p4rt_client.P4RTClientParameters{})
	if err := client1.P4rtClientSet(dut.RawAPIs().P4RT().Default(t)); err != nil {
		t.Fatalf("Could not initialize p4rt client: %v", err)
	}

	client2 := p4rt_client.NewP4RTClient(&p4rt_client.P4RTClientParameters{})
	if err := client2.P4rtClientSet(dut.RawAPIs().P4RT().Default(t)); err != nil {
		t.Fatalf("Could not initialize p4rt client: %v", err)
	}

	args := &testArgs{
		ctx:     ctx,
		client1: client1,
		client2: client2,
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
			Updates: p4rtutils.ACLWbbIngressTableEntryGet([]*p4rtutils.ACLWbbIngressTableEntryInfo{
				{
					Type:          p4_v1.Update_INSERT,
					EtherType:     0x6007,
					EtherTypeMask: 0xFFFF,
					Priority:      1,
				},
				{
					Type:          p4_v1.Update_INSERT,
					EtherType:     0x88cc,
					EtherTypeMask: 0xFFFF,
					Priority:      1,
				},
				{
					Type:     p4_v1.Update_INSERT,
					IsIpv4:   0x1,
					TTL:      0x1,
					TTLMask:  0xFF,
					Priority: 1,
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
				{
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
		expected_update := p4rtutils.ACLWbbIngressTableEntryGet([]*p4rtutils.ACLWbbIngressTableEntryInfo{
			{
				Type:          p4_v1.Update_INSERT,
				EtherType:     0x6007,
				EtherTypeMask: 0xFFFF,
				Priority:      1,
			},
		})

		if err := verifyReadReceiveMatch(expected_update, readResp); err != nil {
			t.Errorf("Table entry for GDP %s", err)
			nomatch += 1
		}

		// Construct expected table for LLDP to match with received table entry
		expected_update = p4rtutils.ACLWbbIngressTableEntryGet([]*p4rtutils.ACLWbbIngressTableEntryInfo{
			{
				Type:          p4_v1.Update_INSERT,
				EtherType:     0x88cc,
				EtherTypeMask: 0xFFFF,
				Priority:      1,
			},
		})
		if err := verifyReadReceiveMatch(expected_update, readResp); err != nil {
			t.Errorf("Table entry for LLDP %s", err)
			nomatch += 1
		}

		// Construct expected table for traceroute to match with received table entry
		expected_update = p4rtutils.ACLWbbIngressTableEntryGet([]*p4rtutils.ACLWbbIngressTableEntryInfo{
			{
				Type:     p4_v1.Update_INSERT,
				IsIpv4:   0x1,
				TTL:      0x1,
				TTLMask:  0xFF,
				Priority: 1,
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
