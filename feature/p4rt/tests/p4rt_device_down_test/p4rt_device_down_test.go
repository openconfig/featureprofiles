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

package p4rt_device_down_test

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"testing"
	"time"

	"github.com/cisco-open/go-p4/p4rt_client"
	"github.com/cisco-open/go-p4/utils"
	"github.com/google/go-cmp/cmp"
	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/p4rtutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
	p4pb "github.com/p4lang/p4runtime/go/p4/v1"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/openconfig/featureprofiles/internal/components"

	spb "github.com/openconfig/gnoi/system"
	tpb "github.com/openconfig/gnoi/types"
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

	electionID       = uint64(100)
	p4rtNode1Name    string
	p4rtNode2Name    string
	lc2ComponentName string

	// P4RT openconfig node-id and P4RT port-id to be configured in DUT and for client connection
	deviceID1 = uint64(111)
	deviceID2 = uint64(222)
	portID1   = uint32(20)
	portID2   = uint32(21)

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

	return i
}

// configureDeviceIDs configures p4rt device-id on the DUT.
func configureDeviceIDs(t *testing.T, dut *ondatra.DUTDevice, nodes map[string]string) {
	t.Helper()
	// Configure Node 1
	t.Logf("Configuring P4RT Node component: %s with NodeId: %d", p4rtNode1Name, deviceID1)
	c1 := &oc.Component{
		Name: ygot.String(p4rtNode1Name),
		IntegratedCircuit: &oc.Component_IntegratedCircuit{
			NodeId: ygot.Uint64(deviceID1),
		},
	}
	gnmi.Replace(t, dut, gnmi.OC().Component(p4rtNode1Name).Config(), c1)

	// Configure Node 2
	t.Logf("Configuring P4RT Node component: %s with NodeId: %d", p4rtNode2Name, deviceID2)
	c2 := &oc.Component{
		Name: ygot.String(p4rtNode2Name),
		IntegratedCircuit: &oc.Component_IntegratedCircuit{
			NodeId: ygot.Uint64(deviceID2),
		},
	}
	gnmi.Replace(t, dut, gnmi.OC().Component(p4rtNode2Name).Config(), c2)
}

// configureDUT configures two ports on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice, port1Name, port2Name string) {
	t.Helper()
	d := gnmi.OC()

	p1 := dut.Port(t, port1Name)
	i1 := &oc.Interface{Name: ygot.String(p1.Name()), Id: ygot.Uint32(portID1)}
	gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), configInterfaceDUT(i1, &dutPort1, dut))

	p2 := dut.Port(t, port2Name)
	i2 := &oc.Interface{Name: ygot.String(p2.Name()), Id: ygot.Uint32(portID2)}
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

// findP4RTNodes returns a map[string]string where keys are unique P4RT Device IDs
// and values represent ONDATRA DUT port IDs from the devices
func findP4RTNodes(t *testing.T, dut *ondatra.DUTDevice) map[string]string {
	t.Helper()
	nodesFound := make(map[string]string)
	p4NodeMapFromUtil := p4rtutils.P4RTNodesByPort(t, dut)
	t.Logf("p4rtutils.P4RTNodesByPort(t, dut) returned: %+v", p4NodeMapFromUtil)

	for portName, p4rtNodeName := range p4NodeMapFromUtil {
		if p4rtNodeName == "" {
			t.Logf("Skipping port '%s' because its P4RT Node Name is empty.", portName)
			continue
		}

		if _, exists := nodesFound[p4rtNodeName]; !exists {
			nodesFound[p4rtNodeName] = portName
			t.Logf("Found P4RT Node '%s' with port '%s'. Current nodes map: %+v", p4rtNodeName, portName, nodesFound)
		}

		// quit when found two unique P4RT Node Names
		if len(nodesFound) == 2 {
			t.Logf("Found 2 distinct P4RT Node Names and their corresponding ports: %+v", nodesFound)
			return nodesFound
		}
	}
	t.Fatalf("The test requires two DUT ports located on different P4RT Nodes. Found %d unique P4RT Nodes) (%+v) from the initial map (%+v). Cannot proceed.", len(nodesFound), nodesFound, p4NodeMapFromUtil)
	return nil
}

// ATE configuration with IP address
func configureATE(t *testing.T, ate *ondatra.ATEDevice, port1Name, port2Name string) gosnappi.Config {
	t.Helper()
	top := gosnappi.NewConfig()

	p1 := ate.Port(t, port1Name)
	atePort1.AddToOTG(top, p1, &dutPort1)

	p2 := ate.Port(t, port2Name)
	atePort2.AddToOTG(top, p2, &dutPort2)

	return top
}

// disableLinecard powers down the linecard associated with port2
func disableLinecard(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	t.Logf("Attempting to disable Linecard component associated with port2 (presumed device_id %d)", deviceID2)
	nodes := p4rtutils.P4RTNodesByPort(t, dut)
	lc2ComponentName, ok := nodes["port2"]
	t.Logf("The state of LC before powering down is: %v", gnmi.Get(t, dut, gnmi.OC().Component(lc2ComponentName).State()))
	if !ok {
		t.Fatalf("Couldn't find P4RT Node/Component Name for port: port2 via p4rtutils")
	}
	t.Logf("Found component name '%s' associated with port2. Proceeding to disable.", lc2ComponentName)

	gnoiClient := dut.RawAPIs().GNOI(t)
	useNameOnly := deviations.GNOISubcomponentPath(dut)
	subCompPath := components.GetSubcomponentPath(lc2ComponentName, useNameOnly)
	subCompPath.Origin = ""
	powerDownSubComponentRequest := &spb.RebootRequest{
		Method: spb.RebootMethod_POWERDOWN,
		Subcomponents: []*tpb.Path{
			subCompPath,
		},
	}
	powerDownResponse, err := gnoiClient.System().Reboot(context.Background(), powerDownSubComponentRequest)
	if err != nil {
		t.Fatalf("Failed to perform line card reboot with unexpected err: %v", err)
	}
	t.Logf("gnoiClient power down response: %v, err: %v", powerDownResponse, err)
}

func enableLinecard(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	expectedUpStatus := oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE
	t.Logf("Attempting to enable Linecard component associated with port2 (presumed device_id %d)", deviceID2)
	gnoiClient := dut.RawAPIs().GNOI(t)
	useNameOnly := deviations.GNOISubcomponentPath(dut)
	subCompPath := components.GetSubcomponentPath(lc2ComponentName, useNameOnly)
	subCompPath.Origin = ""
	powerUpSubComponentRequest := &spb.RebootRequest{
		Method: spb.RebootMethod_POWERUP,
		Subcomponents: []*tpb.Path{
			subCompPath,
		},
	}
	powerUpResponse, err := gnoiClient.System().Reboot(context.Background(), powerUpSubComponentRequest)
	if err != nil {
		t.Fatalf("Failed to perform line card reboot with unexpected err: %v", err)
	}
	gnmi.Await(t, dut, gnmi.OC().Component(lc2ComponentName).OperStatus().State(), 10*time.Minute, expectedUpStatus)
	t.Logf("gnoiClient PowerUp response: %v, err: %v", powerUpResponse, err)
	finalState := gnmi.Get(t, dut, gnmi.OC().Component(lc2ComponentName).OperStatus().State())
	t.Logf("Component '%s' state AFTER power up command: %v", lc2ComponentName, finalState)
}

// setupP4RTClient sends client arbitration message for both clients.
// then sends setforwordingpipelineconfig for both clients, compare the P4Info
func setupP4RTClient(ctx context.Context, args *testArgs) error {
	// Setup p4rt-client stream parameters for both clients
	streamParameter1 := p4rt_client.P4RTStreamParameters{
		Name:        streamName1,
		DeviceId:    deviceID1,
		ElectionIdH: uint64(0),
		ElectionIdL: electionID,
	}
	streamParameter2 := p4rt_client.P4RTStreamParameters{
		Name:        streamName2,
		DeviceId:    deviceID2,
		ElectionIdH: uint64(0),
		ElectionIdL: electionID,
	}
	streamlist := []p4rt_client.P4RTStreamParameters{streamParameter1, streamParameter2}

	// Send ClientArbitration message on both clients.
	clients := []*p4rt_client.P4RTClient{args.client1, args.client2}
	for index, client := range clients {
		if client != nil {
			client.StreamChannelCreate(&streamlist[index])
			if err := client.StreamChannelSendMsg(&streamlist[index].Name, &p4pb.StreamMessageRequest{
				Update: &p4pb.StreamMessageRequest_Arbitration{
					Arbitration: &p4pb.MasterArbitrationUpdate{
						DeviceId: streamlist[index].DeviceId,
						ElectionId: &p4pb.Uint128{
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
		return errors.New("errors seen when loading p4info file")
	}

	// Send SetForwardingPipelineConfig for p4rt leader client.
	fmt.Println("Sending SetForwardingPipelineConfig for both clients")
	deviceidList := []uint64{deviceID1, deviceID2}
	for index, client := range clients {
		if err := client.SetForwardingPipelineConfig(&p4pb.SetForwardingPipelineConfigRequest{
			DeviceId:   deviceidList[index],
			ElectionId: &p4pb.Uint128{High: uint64(0), Low: electionID},
			Action:     p4pb.SetForwardingPipelineConfigRequest_VERIFY_AND_COMMIT,
			Config: &p4pb.ForwardingPipelineConfig{
				P4Info: p4Info,
				Cookie: &p4pb.ForwardingPipelineConfig_Cookie{
					Cookie: 159,
				},
			},
		}); err != nil {
			return errors.New("errors seen when sending set forwarding pipeline config")
		}
	}

	// Receive GetForwardingPipelineConfig
	for index, client := range clients {
		resp, err := client.GetForwardingPipelineConfig(&p4pb.GetForwardingPipelineConfigRequest{
			DeviceId:     deviceidList[index],
			ResponseType: p4pb.GetForwardingPipelineConfigRequest_P4INFO_AND_COOKIE,
		})
		if err != nil {
			return errors.New("errors seen when sending SetForwardingPipelineConfig")
		}
		// Compare P4Info from GetForwardingPipelineConfig and SetForwardingPipelineConfig
		if diff := cmp.Diff(p4Info, resp.Config.P4Info, protocmp.Transform()); diff != "" {
			return fmt.Errorf("P4info diff (-want +got): \n%s", diff)
		}
	}
	return nil
}

// Function to compare and check if the expected table is present in RPC ReadResponse
func verifyReadReceiveMatch(t *testing.T, expectedTable *p4pb.Update, receivedEntry *p4pb.ReadResponse) error {
	for _, table := range receivedEntry.Entities {
		t.Logf("Received Table: %v", table)
		if cmp.Equal(table, expectedTable.Entity, protocmp.Transform(), protocmp.IgnoreFields(&p4pb.TableEntry{}, "meter_config", "counter_data")) {
			return nil
		}
	}
	return fmt.Errorf("no matches found: \ngot %+v, \nwant: %+v", receivedEntry, expectedTable)
}

// TestP4rtConnect connects to the P4Runtime server over grpc
// It then calls setupP4RTClient which sets the arbitration request and sends SetForwardingPipelineConfig with P4Info
func TestP4rtConnect(t *testing.T) {

	dut := ondatra.DUT(t, "dut")
	ctx := context.Background()
	ate := ondatra.ATE(t, "ate")

	// configure DUT with P4RT node-id and ids on different FAPs
	portToNodeMap := findP4RTNodes(t, dut)
	if len(portToNodeMap) < 2 {
		t.Fatalf("findP4RTNodes was expected to find 2 P4RT nodes, but found %d: %+v", len(portToNodeMap), portToNodeMap)
	}

	var dutPort1Name, dutPort2Name string
	// var discoveredP4RTNode1, discoveredP4RTNode2 string

	i := 0
	for p4rtNode, portName := range portToNodeMap {
		if i == 0 {
			p4rtNode1Name = p4rtNode
			dutPort1Name = portName
		} else if i == 1 {
			p4rtNode2Name = p4rtNode
			dutPort2Name = portName
			break
		}
		i++
	}

	lc2ComponentName = p4rtNode2Name

	t.Logf("DUT Port for Node 1 (%s, ID %d): %s", p4rtNode1Name, deviceID1, dutPort1Name)
	t.Logf("DUT Port for Node 2 (%s, ID %d): %s", p4rtNode2Name, deviceID2, dutPort2Name)

	configureDeviceIDs(t, dut, portToNodeMap)

	configureDUT(t, dut, dutPort1Name, dutPort2Name)

	top := configureATE(t, ate, dutPort1Name, dutPort2Name)
	ate.OTG().PushConfig(t, top)

	// Setup two different clients for different FAPs
	client1 := p4rt_client.NewP4RTClient(&p4rt_client.P4RTClientParameters{})
	if err := client1.P4rtClientSet(dut.RawAPIs().P4RT(t)); err != nil {
		t.Fatalf("Could not initialize p4rt client 1: %v", err)
	}

	client2 := p4rt_client.NewP4RTClient(&p4rt_client.P4RTClientParameters{})
	if err := client2.P4rtClientSet(dut.RawAPIs().P4RT(t)); err != nil {
		t.Fatalf("Could not initialize p4rt client 2: %v", err)
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

	// Disable line card associated with port2
	disableLinecard(t, dut)

	lldpACLEntry := p4rtutils.ACLWbbIngressTableEntryGet([]*p4rtutils.ACLWbbIngressTableEntryInfo{
		{
			Type:          p4pb.Update_INSERT,
			EtherType:     0x88cc,
			EtherTypeMask: 0xFFFF,
			Priority:      1,
		},
	})
	if len(lldpACLEntry) != 1 {
		t.Fatalf("Expected exactly one Update from ACLWbbIngressTableEntryGet, got %d", len(lldpACLEntry))
	}

	lldpUpdate := lldpACLEntry[0]

	// RPC Write to write the table entries to the P4RT Server
	writeErrors := make(map[uint64]error)
	clients := map[uint64]*p4rt_client.P4RTClient{deviceID1: args.client1, deviceID2: args.client2}
	deviceIDs := []uint64{deviceID1, deviceID2}

	for _, deviceID := range deviceIDs {
		client := clients[deviceID]
		if client == nil {
			t.Logf("Client for Device ID %d is nil, skipping Write.", deviceID)
			writeErrors[deviceID] = fmt.Errorf("client is nil")
			continue
		}
		writeErrors[deviceID] = client.Write(&p4pb.WriteRequest{
			DeviceId:   deviceID,
			ElectionId: &p4pb.Uint128{High: uint64(0), Low: electionID},
			Updates: p4rtutils.ACLWbbIngressTableEntryGet([]*p4rtutils.ACLWbbIngressTableEntryInfo{
				{
					Type:          p4pb.Update_INSERT,
					EtherType:     0x88cc,
					EtherTypeMask: 0xFFFF,
					Priority:      1,
				},
			}),
			Atomicity: p4pb.WriteRequest_CONTINUE_ON_ERROR,
		})
	}

	// Verify Device 1 (Active) Read Results - Should succeed
	if writeErrors[deviceID1] != nil {
		countOK, countNotOK, errDetails := p4rt_client.P4RTWriteErrParse(writeErrors[deviceID1])
		t.Errorf("P4RT Write for ACTIVE node %d (Node: %s) failed unexpectedly: %v", deviceID1, p4rtNode1Name, writeErrors[deviceID1])
		t.Errorf("Write Partial Errors %d/%d: %s", countOK, countNotOK, errDetails)
	} else {
		t.Logf("P4RT Write for ACTIVE node %d (Node: %s) succeeded as expected.", deviceID1, p4rtNode1Name)
	}
	// Verify Device 2 (Inactive) Read Results- Should fail with specific error
	if writeErrors[deviceID2] == nil {
		t.Errorf("P4RT Write for INACTIVE node %d (Node: %s) succeeded unexpectedly", deviceID2, p4rtNode2Name)
	} else {
		if countOK, countNotOK, errDetails := p4rt_client.P4RTWriteErrParse(writeErrors[deviceID2]); countNotOK > 0 {
			t.Logf("Write to INACTIVE node with Device ID %d failed as expected (%d OK / %d Not OK): %s", deviceID2, countOK, countNotOK, errDetails)
		}
	}

	// Receive Read Response and Verify Device 1 (Active) - Should find the entry
	if args.client1 != nil {
		rStream1, rErr1 := args.client1.Read(&p4pb.ReadRequest{
			DeviceId: deviceID1,
			Entities: []*p4pb.Entity{
				{
					Entity: &p4pb.Entity_TableEntry{},
				},
			},
		})
		if rErr1 != nil {
			t.Errorf("P4RT Read request failed for ACTIVE node %d: %v", deviceID1, rErr1)
		} else {
			readResp1, respErr1 := rStream1.Recv()
			if respErr1 != nil {
				t.Errorf("P4RT Read Recv failed for ACTIVE node %d: %v", deviceID1, respErr1)
			} else {
				t.Logf("Read Response received from ACTIVE node %d.", deviceID1)
				if err := verifyReadReceiveMatch(t, lldpUpdate, readResp1); err != nil {
					t.Errorf("LLDP table entry verification failed for ACTIVE node %d: %s", deviceID1, err)
				} else {
					t.Logf("LLDP table entry verification successful for ACTIVE node %d.", deviceID1)
				}
			}
		}
	} else {
		t.Errorf("Cannot perform Read verification for ACTIVE node %d, client is nil", deviceID1)
	}

	// Attempt to Receive Read Response for Device 2 (Inactive) - Should fail
	t.Logf("Sending P4RT Read to INACTIVE node %d (Node: %s) (expected to fail)", deviceID2, p4rtNode2Name)
	readFailedAsExpected := false
	if args.client2 != nil {
		rStream2, rErr2 := args.client2.Read(&p4pb.ReadRequest{
			DeviceId: deviceID2,
			Entities: []*p4pb.Entity{{Entity: &p4pb.Entity_TableEntry{}}},
		})
		if rErr2 != nil {
			t.Errorf("P4RT Read request for INACTIVE node %d failed: %v", deviceID2, rErr2)
			readFailedAsExpected = true
		} else {
			// Read request succeeded, try Recv
			readResp2, respErr2 := rStream2.Recv()
			if respErr2 != nil {
				t.Logf("P4RT Read Recv for INACTIVE node %d failed: %v", deviceID2, respErr2)
				readFailedAsExpected = true
			} else {
				// Read and Recv succeeded, verify entry is NOT present
				if err := verifyReadReceiveMatch(t, lldpUpdate, readResp2); err == nil {
					t.Errorf("P4RT Read for INACTIVE node %d unexpectedly succeeded AND found the LLDP entry.", deviceID2)
				} else {
					t.Errorf("P4RT Read for INACTIVE node %d unexpectedly succeeded but did not find the LLDP entry, as expected.", deviceID2)
					readFailedAsExpected = true
				}
			}
		}
	} else {
		t.Logf("Skipping Read verification for INACTIVE node %d, client is nil (considered expected failure).", deviceID2)
		readFailedAsExpected = true
	}
	if !readFailedAsExpected {
		t.Errorf("Read verification for INACTIVE node %d did not fail as expected.", deviceID2)
	}

	// Re-enable line card associated with port2
	t.Logf("Re-enabling Linecard %s", lc2ComponentName)
	enableLinecard(t, dut)

}
