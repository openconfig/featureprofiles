package p4rt_election_test

import (
	"errors"
	"flag"
	"fmt"
	"testing"
	"time"

	"github.com/cisco-open/go-p4/p4rt_client"
	"github.com/cisco-open/go-p4/utils"
	"github.com/openconfig/featureprofiles/feature/experimental/p4rt/internal/p4rtutils"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
	p4_v1 "github.com/p4lang/p4runtime/go/p4/v1"
)

// Variable definitions
var (
	streamName   = "p4rt"
	p4InfoFile   = flag.String("p4info_file_location", "../../wbb.p4info.pb.txt", "Path to the p4info file.")
	p4rtNodeName = flag.String("p4rt_node_name", "SwitchChip1/0", "component name for P4RT Node")
	gdpEtherType = *ygot.Uint32(0x6007)
	portID       = *ygot.Uint32(10)
	deviceID     = *ygot.Uint64(1)
	highID       = *ygot.Uint64(0)
	pDesc        = "Primary Connection"
	sDesc        = "Secondary Connection"
	npDesc       = "New Primary"
)

// Define PacketIO Interface
type PacketIO interface {
	GetTableEntry(delete bool) []*p4rtutils.ACLWbbIngressTableEntryInfo
}

// Define GDP Packet io structure
type GDPPacketIO struct {
	PacketIOPacket
	IngressPort string
}

// Define PacketIO Structure
type PacketIOPacket struct {
	EthernetType *uint32
}

// Define Multiple Client Args Structure
type mcTestArgs struct {
	name       string
	Items      [2]*testArgs
	expectPass bool
}

// Define Test Args Structure
type testArgs struct {
	desc              string
	lowID             uint64
	highID            uint64
	deviceID          uint64
	handle            *p4rt_client.P4RTClient
	expectStatus      int32
	expectFinalStatus int32
	expectPass        bool
	expectWrite       bool
	expectRead        bool
}

// configureDeviceId configures p4rt device-id on the DUT.
func configureDeviceId(t *testing.T, dut *ondatra.DUTDevice) {
	component := oc.Component{}
	component.IntegratedCircuit = &oc.Component_IntegratedCircuit{}
	component.Name = ygot.String(*p4rtNodeName)
	component.IntegratedCircuit.NodeId = ygot.Uint64(deviceID)
	gnmi.Replace(t, dut, gnmi.OC().Component(*p4rtNodeName).Config(), &component)
}

// configurePortId configures p4rt port-id on the DUT.
func configurePortId(t *testing.T, dut *ondatra.DUTDevice) {
	portName := dut.Port(t, "port1").Name()
	gnmi.Replace(t, dut, gnmi.OC().Interface(portName).Id().Config(), portID)
}

// Create client connection
func clientConnection(t *testing.T, dut *ondatra.DUTDevice) *p4rt_client.P4RTClient {
	clientHandle := p4rt_client.P4RTClient{}
	if err := clientHandle.P4rtClientSet(dut.RawAPIs().P4RT().Default(t)); err != nil {
		t.Fatalf("Could not initialize p4rt client: %v", err)
	}
	return &clientHandle
}

// Setup P4RT Arbitration Stream
func streamP4RTArb(args *testArgs) (int32, error) {
	streamParameter := p4rt_client.P4RTStreamParameters{
		Name:        streamName,
		DeviceId:    args.deviceID,
		ElectionIdH: args.highID,
		ElectionIdL: args.lowID,
	}
	// Send arb msg
	if args.handle != nil {
		args.handle.StreamChannelCreate(&streamParameter)
		if err := args.handle.StreamChannelSendMsg(&streamName, &p4_v1.StreamMessageRequest{
			Update: &p4_v1.StreamMessageRequest_Arbitration{
				Arbitration: &p4_v1.MasterArbitrationUpdate{
					DeviceId: streamParameter.DeviceId,
					ElectionId: &p4_v1.Uint128{
						High: streamParameter.ElectionIdH,
						Low:  streamParameter.ElectionIdL,
					},
				},
			},
		}); err != nil {
			return 0, errors.New("Errors seen when sending ClientArbitration message.")
		}
		time.Sleep(1 * time.Second)
		return getRespCode(args)
	}
	return 0, errors.New("Missing Client")
}

// Obtain status Code from Arbitration
func getRespCode(args *testArgs) (int32, error) {
	handle := args.handle
	if handle != nil {
		// Grab Arb Response to look at status code
		_, arbResp, arbErr := handle.StreamChannelGetArbitrationResp(&streamName, 1)
		if arbErr != nil {
			return 0, errors.New("Errors seen in ClientArbitration response")
		}
		// Handle exception when GetCode is empty.
		respCode := arbResp.Arb.GetStatus().GetCode()
		return respCode, nil
	}
	return 0, errors.New("Missing Client")
}

// getGDPParameter returns GDP related parameters for testPacketIn testcase.
func getGDPParameter(t *testing.T) PacketIO {
	return &GDPPacketIO{
		PacketIOPacket: PacketIOPacket{
			EthernetType: &gdpEtherType,
		},
		IngressPort: fmt.Sprint(portID),
	}
}

// GetTableEntry creates wbb acl entry related to GDP.
func (gdp *GDPPacketIO) GetTableEntry(delete bool) []*p4rtutils.ACLWbbIngressTableEntryInfo {
	actionType := p4_v1.Update_INSERT
	if delete {
		actionType = p4_v1.Update_DELETE
	}
	return []*p4rtutils.ACLWbbIngressTableEntryInfo{{
		Type:          actionType,
		EtherType:     0x6007,
		EtherTypeMask: 0xFFFF,
		Priority:      1,
	}}
}

// writeTableEntry programs or deletes p4rt table entry
func writeTableEntry(args *testArgs, t *testing.T, packetIO PacketIO, delete bool) error {
	t.Helper()

	p4Info, err := utils.P4InfoLoad(p4InfoFile)
	if err != nil {
		return errors.New("Errors seen when loading p4info file.")
	}

	if err := args.handle.SetForwardingPipelineConfig(&p4_v1.SetForwardingPipelineConfigRequest{
		DeviceId:   args.deviceID,
		ElectionId: &p4_v1.Uint128{High: args.highID, Low: args.lowID},
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

	err = args.handle.Write(&p4_v1.WriteRequest{
		DeviceId:   args.deviceID,
		ElectionId: &p4_v1.Uint128{High: args.highID, Low: args.lowID},
		Updates: p4rtutils.ACLWbbIngressTableEntryGet(
			packetIO.GetTableEntry(delete),
		),
		Atomicity: p4_v1.WriteRequest_CONTINUE_ON_ERROR,
	})
	if err != nil {
		t.Logf("Write Error: %v", err)
		return err
	}
	return nil
}

// Function to read a table entry returns response/error
func readTableEntry(args *testArgs, t *testing.T) (*p4_v1.ReadResponse, error) {
	t.Helper()
	readMsg := &p4_v1.ReadRequest{
		DeviceId: args.deviceID,
		Entities: []*p4_v1.Entity{
			{
				Entity: &p4_v1.Entity_TableEntry{
					TableEntry: &p4_v1.TableEntry{
						TableId: p4rtutils.WbbTableMap["acl_wbb_ingress_table"],
					},
				},
			},
		},
	}
	readClient, err := args.handle.Read(readMsg)
	if err != nil {
		t.Logf("ReadClient Error: %v", err)
		return nil, err
	}
	readResp, err := readClient.Recv()
	if err != nil {
		t.Logf("readResp Error: %v", err)
		return nil, err
	}
	return readResp, nil
}

// Every client/controller should be able to read
func canRead(t *testing.T, args *testArgs) (bool, error) {
	_, readErr := readTableEntry(args, t)
	if readErr != nil {
		return false, readErr
	}
	return true, nil
}

// Only Primary should be able to write
func canWrite(t *testing.T, args *testArgs) (bool, error) {
	pktIO := getGDPParameter(t)
	writeErr := writeTableEntry(args, t, pktIO, false)
	if writeErr != nil {
		if args.expectWrite == false {
			t.Logf("Expected P4Client error: Client write permission (highID %d, lowID %d): want %v, got %v", args.highID, args.lowID, args.expectWrite, false)
		}
		return false, writeErr
	}
	return true, nil
}

// This function handles the values passed through the test case
// and compares them to values obtained in canRead()/canWrite().
func validateRWResp(t *testing.T, args *testArgs) bool {
	returnReadVal := true
	returnWriteVal := true
	// Validate write permissions
	writeOp, writeErr := canWrite(t, args)
	if args.expectWrite != writeOp {
		t.Errorf("Client write permission error: (highID %d, lowID %d): want %v, got %v", args.highID, args.lowID, args.expectWrite, writeOp)
		if writeErr != nil {
			t.Errorf("Write Error %v", writeErr)
		}
		returnWriteVal = false
	}

	// Validate read permissions
	readOp, readErr := canRead(t, args)
	if args.expectRead != readOp {
		t.Errorf("Client read permission error: (highID %d, lowID %d): want %v, got %v", args.highID, args.lowID, args.expectRead, readOp)
		t.Errorf("Read Error: %v", readErr)
		returnReadVal = false
	}
	if returnWriteVal && returnReadVal {
		return true
	}
	return false
}

func removeClient(handle *p4rt_client.P4RTClient) {
	handle.StreamChannelDestroy(&streamName)
	handle.ServerDisconnect()
}

// Test Zero client with 0 electionID
func TestZeroMaster(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	configureDeviceId(t, dut)
	configurePortId(t, dut)
	test := testArgs{

		desc:         pDesc,
		lowID:        *ygot.Uint64(0),
		highID:       highID,
		handle:       clientConnection(t, dut),
		deviceID:     deviceID,
		expectPass:   false,
		expectStatus: 0,
	}
	t.Run(test.desc, func(t *testing.T) {
		resp, err := streamP4RTArb(&test)
		if err == nil && test.expectPass {
			t.Errorf("Zero ElectionID (0,0) is allowed: %v", err)
			removeClient(test.handle)
		} else {
			t.Logf("Zero ElectionID (0,0) connection failed as expected: %v", err)
		}
		// Validate status code
		if resp != test.expectStatus {
			t.Errorf("Incorrect status code received: want %d, got %d", test.expectStatus, resp)
		}
	})

}

// Test Primary Reconnect
func TestPrimaryReconnect(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	configureDeviceId(t, dut)
	configurePortId(t, dut)
	testCases := []testArgs{
		{
			desc:         pDesc,
			lowID:        *ygot.Uint64(100),
			highID:       highID,
			handle:       clientConnection(t, dut),
			deviceID:     deviceID,
			expectPass:   true,
			expectWrite:  true,
			expectRead:   true,
			expectStatus: 0,
		}, {
			desc:         sDesc,
			lowID:        *ygot.Uint64(90),
			highID:       highID,
			handle:       clientConnection(t, dut),
			deviceID:     deviceID,
			expectPass:   true,
			expectWrite:  false,
			expectRead:   true,
			expectStatus: 5,
		}, {
			desc:         pDesc,
			lowID:        *ygot.Uint64(100),
			highID:       highID,
			handle:       clientConnection(t, dut),
			deviceID:     deviceID,
			expectPass:   true,
			expectWrite:  true,
			expectRead:   true,
			expectStatus: 0,
		},
	}
	for _, test := range testCases {
		t.Run(test.desc, func(t *testing.T) {
			resp, err := streamP4RTArb(&test)
			if err != nil && test.expectPass {
				t.Errorf("Failed to setup P4RT Client: %v", err)
			}
			// Validate status code
			if resp != test.expectStatus {
				t.Errorf("Incorrect status code received: want %d, got %d", test.expectStatus, resp)
			}
			// Validate the response for Read/Write for each client
			validateRWResp(t, &test)
			// Disconnect Primary
			removeClient(test.handle)
		})
	}
}

// Test Primary/Secondary clients
func TestPrimarySecondary(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	configureDeviceId(t, dut)
	configurePortId(t, dut)
	testCases := []testArgs{
		{
			desc:         pDesc,
			lowID:        *ygot.Uint64(100),
			highID:       highID,
			handle:       clientConnection(t, dut),
			deviceID:     deviceID,
			expectPass:   true,
			expectWrite:  true,
			expectRead:   true,
			expectStatus: 0,
		}, {
			desc:         pDesc,
			lowID:        *ygot.Uint64(90),
			highID:       highID,
			handle:       clientConnection(t, dut),
			deviceID:     deviceID,
			expectPass:   true,
			expectWrite:  false,
			expectRead:   true,
			expectStatus: 6,
		},
	}
	var p4Clients []*p4rt_client.P4RTClient
	for _, test := range testCases {
		t.Run(test.desc, func(t *testing.T) {
			resp, err := streamP4RTArb(&test)
			if err != nil && test.expectPass {
				t.Errorf("Failed to setup P4RT Client: %v", err)
			}
			// Validate status code
			if resp != test.expectStatus {
				t.Errorf("Incorrect status code received: want %d, got %d", test.expectStatus, resp)
			}
			// Validate the response for Read/Write for each client
			validateRWResp(t, &test)
			// Keep track of connections for teardown
			p4Clients = append(p4Clients, test.handle)
		})
	}
	// Teardown clients
	for _, p4Client := range p4Clients {
		removeClient(p4Client)
	}
}

// Test Duplicate ElectionID
func TestDuplicateElectionID(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	configureDeviceId(t, dut)
	configurePortId(t, dut)
	testCases := []testArgs{
		{
			desc:         pDesc,
			lowID:        *ygot.Uint64(100),
			highID:       highID,
			handle:       clientConnection(t, dut),
			deviceID:     deviceID,
			expectPass:   true,
			expectStatus: 0,
		}, {
			desc:         sDesc,
			lowID:        *ygot.Uint64(100),
			highID:       highID,
			handle:       clientConnection(t, dut),
			deviceID:     deviceID,
			expectPass:   false,
			expectStatus: 0,
		},
	}
	var p4Clients []*p4rt_client.P4RTClient
	for _, test := range testCases {
		t.Run(test.desc, func(t *testing.T) {
			resp, err := streamP4RTArb(&test)
			if test.expectPass {
				if err != nil {
					t.Errorf("Failed to setup P4RT Client: %v", err)
				}
			} else {
				if err != nil {
					t.Logf("As expected failed to setup P4RT Client: %v", err)
				}
			}

			// Validate status code
			if resp != test.expectStatus {
				t.Errorf("Incorrect status code received: want %d, got %d", test.expectStatus, resp)
			}
			// Keep track of connections for teardown
			p4Clients = append(p4Clients, test.handle)
		})
	}
	// Teardown clients
	for _, p4Client := range p4Clients {
		removeClient(p4Client)
	}
}

func TestReplacePrimary(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	configureDeviceId(t, dut)
	configurePortId(t, dut)
	testCases := []mcTestArgs{
		{
			name:       "Primary_secondary_OK",
			expectPass: true,
			Items: [2]*testArgs{
				{
					desc:              pDesc,
					lowID:             *ygot.Uint64(101),
					highID:            highID,
					handle:            clientConnection(t, dut),
					deviceID:          deviceID,
					expectPass:        true,
					expectWrite:       true,
					expectRead:        true,
					expectStatus:      0,
					expectFinalStatus: 6,
				},
				{
					desc:              npDesc,
					lowID:             *ygot.Uint64(102),
					highID:            highID,
					handle:            clientConnection(t, dut),
					deviceID:          deviceID,
					expectPass:        true,
					expectWrite:       true,
					expectRead:        true,
					expectStatus:      0,
					expectFinalStatus: 0,
				},
			},
		},
	}
	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			resp, err := streamP4RTArb(test.Items[0])
			if err != nil && test.expectPass {
				t.Errorf("Failed to setup P4RT Client: %v", err)
			}
			// Validate status code
			if resp != test.Items[0].expectStatus {
				t.Errorf("Incorrect status code received: want %d, got %d", test.Items[0].expectStatus, resp)
			}
			// Validates that client0 can read/write
			validateRWResp(t, test.Items[0])
			// Create the stream for client1 (new client)
			newResp, err := streamP4RTArb(test.Items[1])
			if err != nil {
				t.Errorf("Failed to setup P4RT Client: %v", err)
			}
			if newResp != test.Items[1].expectStatus {
				t.Errorf("Incorrect status code received: want %d, got %d", test.Items[1].expectStatus, newResp)
			}
			// Validates that client1 can read/write
			validateRWResp(t, test.Items[1])
			time.Sleep(1 * time.Second)
			// get first client new response code
			arbResp, err := getRespCode(test.Items[0])
			if err != nil {
				t.Errorf("Failed get response code from primary: %v", err)
			}
			if arbResp != test.Items[0].expectFinalStatus {
				t.Errorf("Incorrect status code received: want %d, got %d", test.Items[0].expectFinalStatus, arbResp)
			}
			// Since client1 has connected with higher election ID
			// the expectWrite flag is moved to false
			test.Items[0].expectWrite = false
			// Validates that client0 can only read, cannot write anymore
			validateRWResp(t, test.Items[0])
		})
		// Teardown clients
		for _, item := range test.Items {
			item.handle.StreamChannelDestroy(&streamName)
			item.handle.ServerDisconnect()
		}
	}
}

// Test arbitration update from same client
func TestArbitrationUpdate(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	configureDeviceId(t, dut)
	configurePortId(t, dut)
	test := testArgs{

		desc:         pDesc,
		lowID:        *ygot.Uint64(102),
		highID:       highID,
		handle:       clientConnection(t, dut),
		deviceID:     deviceID,
		expectPass:   false,
		expectWrite:  true,
		expectRead:   true,
		expectStatus: 0,
	}
	var p4Clients []*p4rt_client.P4RTClient

	t.Run(test.desc, func(t *testing.T) {
		resp, err := streamP4RTArb(&test)
		if err != nil && test.expectPass {
			t.Errorf("Failed to setup P4RT Client: %v", err)
		}
		// Validate status code
		if resp != test.expectStatus {
			t.Errorf("Incorrect status code received: want %d, got %d", test.expectStatus, resp)
		}
		// Validates that client can read/write
		validateRWResp(t, &test)
		// Updating ElectionID to lower value
		test.lowID = 99
		// After updating electionID, statusCode also changes
		// to secondary without a primary
		test.expectStatus = 5
		// After updating election, expectWrite is false
		// as this client is no longer primary
		test.expectWrite = false
		resp, err = streamP4RTArb(&test)
		if err != nil && test.expectPass {
			t.Errorf("Failed to setup P4RT Client: %v", err)
		}
		// Validate status code
		if resp != test.expectStatus {
			t.Errorf("Incorrect status code received: want %d, got %d", test.expectStatus, resp)
		}
		validateRWResp(t, &test)
		// Keep track of connections for teardown
		p4Clients = append(p4Clients, test.handle)
	})

	// Teardown clients
	for _, p4Client := range p4Clients {
		removeClient(p4Client)
	}
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}
