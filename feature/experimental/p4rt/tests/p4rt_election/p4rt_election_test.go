package p4rt_election_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"flag"

	"github.com/cisco-open/go-p4/p4rt_client"
	"github.com/cisco-open/go-p4/utils"
	"github.com/openconfig/featureprofiles/feature/experimental/p4rt/internal/p4rtutils"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
	p4_v1 "github.com/p4lang/p4runtime/go/p4/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Flag variable definitions
var (
	p4InfoFile = flag.String("p4info_file_location", "../../wbb.p4info.pb.txt", "Path to the p4info file.")
)

// Variable definitions
var (
	pDesc        = "Primary Connection"
	sDesc        = "Secondary Connection"
	npDesc       = "New Primary"
	gdpEtherType = *ygot.Uint32(0x6007)
	portId       = *ygot.Uint32(10)
	deviceId     = *ygot.Uint64(1)
	inId0        = *ygot.Uint64(0)
	inId90       = *ygot.Uint64(90)
	inId100      = *ygot.Uint64(100)
	inId101      = *ygot.Uint64(101)
	inId102      = *ygot.Uint64(102)
	streamName   = "p4rt"
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
	name  string
	Items [2]*testArgs
}

// Define Test Args Structure
type testArgs struct {
	desc            string
	lowID           uint64
	highID          uint64
	deviceID        uint64
	handle          *p4rt_client.P4RTClient
	wantStatus      int32
	wantFinalStatus int32
	wantFail        bool
	wantWrite       bool
	wantRead        bool
}

// configureDeviceId configures p4rt device-id on the DUT.
func configureDeviceId(t *testing.T, dut *ondatra.DUTDevice) {
	nodes := p4rtutils.P4RTNodesByPort(t, dut)
	p4rtNode, ok := nodes["port1"]
	if !ok {
		t.Fatal("Couldn't find P4RT Node for port: port1")
	}
	t.Logf("Configuring P4RT Node: %s", p4rtNode)
	c := oc.Component{}
	c.Name = ygot.String(p4rtNode)
	c.IntegratedCircuit = &oc.Component_IntegratedCircuit{}
	c.IntegratedCircuit.NodeId = ygot.Uint64(deviceId)
	gnmi.Replace(t, dut, gnmi.OC().Component(p4rtNode).Config(), &c)
}

// configurePortId configures p4rt port-id and interface type on the DUT.
func configurePortId(t *testing.T, dut *ondatra.DUTDevice) {
	d := gnmi.OC()
	portName := dut.Port(t, "port1").Name()
	currIntf := &oc.Interface{
		Name: ygot.String(portName),
		Type: oc.IETFInterfaces_InterfaceType_ethernetCsmacd,
		Id:   &portId,
	}
	gnmi.Replace(t, dut, d.Interface(portName).Config(), currIntf)

}

// Create client connection
func clientConnection(t *testing.T, dut *ondatra.DUTDevice) *p4rt_client.P4RTClient {
	clientHandle := p4rt_client.NewP4RTClient(&p4rt_client.P4RTClientParameters{})
	if err := clientHandle.P4rtClientSet(dut.RawAPIs().P4RT().Default(t)); err != nil {
		t.Fatalf("Could not initialize p4rt client: %v", err)
	}
	return clientHandle
}

// Setup P4RT Arbitration Stream which will return a GRPC status code
// in line with the client arbitration notifications. An error can also
// be returned.
func streamP4RTArb(args *testArgs) (int32, error) {
	streamParameter := p4rt_client.P4RTStreamParameters{
		Name:        streamName,
		DeviceId:    args.deviceID,
		ElectionIdH: args.highID,
		ElectionIdL: args.lowID,
	}
	// Send ClientArbitration message for a given handle.
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
			return 0, fmt.Errorf("errors seen when sending ClientArbitration message: %v", err)
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
			if err := p4rtutils.StreamTermErr(args.handle.StreamTermErr); err != nil {
				if err != nil {
					return int32(status.Code(err)), err
				}
				return 0, err
			}
			return 0, fmt.Errorf("Errors seen in ClientArbitration response: %v", arbErr)
		}
		// Handle exception when GetCode is empty.
		respCode := arbResp.Arb.GetStatus().GetCode()
		return respCode, nil
	}
	return 0, errors.New("Missing Client")
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

	if err := args.handle.Write(&p4_v1.WriteRequest{
		DeviceId:   args.deviceID,
		ElectionId: &p4_v1.Uint128{High: args.highID, Low: args.lowID},
		Updates: p4rtutils.ACLWbbIngressTableEntryGet(
			packetIO.GetTableEntry(delete),
		),
		Atomicity: p4_v1.WriteRequest_CONTINUE_ON_ERROR,
	}); err != nil {
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
func canWrite(t *testing.T, args *testArgs, includeDeletes bool) (bool, error) {
	pktIO := &GDPPacketIO{
		PacketIOPacket: PacketIOPacket{
			EthernetType: &gdpEtherType,
		},
		IngressPort: fmt.Sprint(portId),
	}
	writeErr := writeTableEntry(args, t, pktIO, false)
	if writeErr != nil {
		if args.wantWrite == false {
			t.Logf("Expected P4Client error: Client write permission (highID %d, lowID %d): want %v, got %v", args.highID, args.lowID, args.wantWrite, false)
		}
		return false, writeErr
	}
	if includeDeletes {
		if writeErr = writeTableEntry(args, t, pktIO, true); writeErr != nil {
			t.Errorf("Error deleting table entry (highID %d, lowID %d): %v", args.highID, args.lowID, writeErr)
		}
	}
	return true, nil
}

// This function handles the values passed through the test case
// and compares them to values obtained in canRead()/canWrite().
func validateRWResp(t *testing.T, args *testArgs, includeDeletes bool) bool {
	returnReadVal := true
	returnWriteVal := true
	// Validate write permissions
	writeOp, writeErr := canWrite(t, args, includeDeletes)
	if args.wantWrite != writeOp {
		t.Errorf("Client write permission error: (highID %d, lowID %d): want %v, got %v", args.highID, args.lowID, args.wantWrite, writeOp)
		if writeErr != nil {
			t.Errorf("Write Error %v", writeErr)
		}
		returnWriteVal = false
	}

	// Validate read permissions
	readOp, readErr := canRead(t, args)
	if args.wantRead != readOp {
		t.Errorf("Client read permission error: (highID %d, lowID %d): want %v, got %v", args.highID, args.lowID, args.wantRead, readOp)
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
	// Give some time for the client to disconnect
	time.Sleep(2 * time.Second)
}

// Test client with unset electionId
func TestUnsetElectionid(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	configureDeviceId(t, dut)
	configurePortId(t, dut)
	clients := []testArgs{
		{
			desc:       pDesc,
			handle:     clientConnection(t, dut),
			deviceID:   deviceId,
			wantStatus: int32(codes.NotFound),
		}, {
			desc:       sDesc,
			handle:     clientConnection(t, dut),
			deviceID:   deviceId,
			wantStatus: int32(codes.NotFound),
		},
	}
	if deviations.P4rtUnsetElectionIDPrimaryAllowed(dut) {
		// For P4Runtime server implementations that allow 0 election id update the
		// expected status to OK for primary and INVALID_ARGUMENT for the secondary
		// connection that connected with the same accepted election id as the
		// primary
		clients[0].wantStatus = int32(codes.OK)
		clients[1].wantStatus = int32(codes.InvalidArgument)
	}
	// Connect 2 clients to same deviceId with unset electionId.
	for _, test := range clients {
		t.Run(test.desc, func(t *testing.T) {
			streamParameter := p4rt_client.P4RTStreamParameters{
				Name:     streamName,
				DeviceId: test.deviceID,
			}
			if test.handle != nil {
				test.handle.StreamChannelCreate(&streamParameter)
				if err := test.handle.StreamChannelSendMsg(&streamName, &p4_v1.StreamMessageRequest{
					Update: &p4_v1.StreamMessageRequest_Arbitration{
						Arbitration: &p4_v1.MasterArbitrationUpdate{
							DeviceId: test.deviceID,
						},
					},
				}); err != nil {
					t.Fatalf("Errors while sending Arbitration Request with unset Election ID: %v", err)
				}
			}
			time.Sleep(1 * time.Second)
			// Validate the return status code for MasterArbitration.
			resp, err := getRespCode(&test)
			if resp != test.wantStatus {
				t.Fatalf("Incorrect status code received: want %d, got %d", test.wantStatus, resp)
			}
			if err != nil {
				if deviations.P4rtUnsetElectionIDPrimaryAllowed(dut) && test.desc != sDesc {
					t.Errorf("Errors seen when sending Master Arbitration for unset ElectionID: %v", err)
				}
			}
			// Verify GetForwardingPipeline for unset electionId.
			_, err = test.handle.GetForwardingPipelineConfig(&p4_v1.GetForwardingPipelineConfigRequest{
				DeviceId:     deviceId,
				ResponseType: p4_v1.GetForwardingPipelineConfigRequest_P4INFO_AND_COOKIE,
			})
			if err != nil {
				t.Errorf("Errors seen when sending GetForwardingPipelineConfig: %v", err)
			}
			p4Info, err := utils.P4InfoLoad(p4InfoFile)
			if err != nil {
				t.Errorf("Errors seen when loading p4info file: %v", err)
			}
			//  Verify SetForwardingPipeline fails for unset electionId.
			if err = test.handle.SetForwardingPipelineConfig(&p4_v1.SetForwardingPipelineConfigRequest{
				DeviceId: deviceId,
				Action:   p4_v1.SetForwardingPipelineConfigRequest_VERIFY_AND_COMMIT,
				Config: &p4_v1.ForwardingPipelineConfig{
					P4Info: &p4Info,
					Cookie: &p4_v1.ForwardingPipelineConfig_Cookie{
						Cookie: 159,
					},
				},
			}); err == nil {
				if !deviations.P4rtUnsetElectionIDPrimaryAllowed(dut) {
					// Verify that SetForwardingPipelineConfig is rejected for implementations that do
					// not allow 0 election id
					t.Errorf("SetForwardingPipelineConfig accepted for unset Election ID: %v", err)
				}
			} else {
				if deviations.P4rtUnsetElectionIDPrimaryAllowed(dut) {
					// Verify that SetForwardingPipelineConfig is allowed for implementations that
					// allow 0 election id
					t.Errorf("SetForwardingPipelineConfig unexpectedly failed for Election ID: %v", err)
				}
			}
		})
	}
	// Disconnect all clients.
	for _, test := range clients {
		removeClient(test.handle)
	}
}

// Test Primary Reconnect
func TestPrimaryReconnect(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	configureDeviceId(t, dut)
	configurePortId(t, dut)
	testCases := []testArgs{
		{
			desc:       pDesc,
			lowID:      inId100,
			highID:     inId0,
			handle:     clientConnection(t, dut),
			deviceID:   deviceId,
			wantFail:   false,
			wantWrite:  true,
			wantRead:   true,
			wantStatus: int32(codes.OK),
		}, {
			desc:       sDesc,
			lowID:      inId90,
			highID:     inId0,
			handle:     clientConnection(t, dut),
			deviceID:   deviceId,
			wantFail:   false,
			wantWrite:  false,
			wantRead:   true,
			wantStatus: int32(codes.NotFound),
		}, {
			desc:       pDesc,
			lowID:      inId100,
			highID:     inId0,
			handle:     clientConnection(t, dut),
			deviceID:   deviceId,
			wantFail:   false,
			wantWrite:  true,
			wantRead:   true,
			wantStatus: int32(codes.OK),
		},
	}
	if deviations.P4rtBackupArbitrationResponseCode(dut) {
		// Change the expected status code to ALREADY_EXISTS for deviant implementations
		// that send ALREADY_EXISTS instead of NOT_FOUND to secondary clients when there
		// is no primary
		testCases[1].wantStatus = int32(codes.AlreadyExists)
	}
	for _, test := range testCases {
		t.Run(test.desc, func(t *testing.T) {
			resp, err := streamP4RTArb(&test)
			if err != nil {
				t.Errorf("Failed to setup P4RT Client: %v", err)
			}
			// Validate status code
			if resp != test.wantStatus {
				t.Errorf("Incorrect status code received: want %d, got %d", test.wantStatus, resp)
			}
			// Validate the response for Read/Write for each client
			validateRWResp(t, &test, !deviations.P4RTMissingDelete(dut))
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
			desc:       pDesc,
			lowID:      inId100,
			highID:     inId0,
			handle:     clientConnection(t, dut),
			deviceID:   deviceId,
			wantFail:   false,
			wantWrite:  true,
			wantRead:   true,
			wantStatus: int32(codes.OK),
		}, {
			desc:       pDesc,
			lowID:      inId90,
			highID:     inId0,
			handle:     clientConnection(t, dut),
			deviceID:   deviceId,
			wantFail:   false,
			wantWrite:  false,
			wantRead:   true,
			wantStatus: int32(codes.AlreadyExists),
		},
	}
	var p4Clients []*p4rt_client.P4RTClient
	for _, test := range testCases {
		t.Run(test.desc, func(t *testing.T) {
			resp, err := streamP4RTArb(&test)
			if err != nil {
				t.Errorf("Failed to setup P4RT Client: %v", err)
			}
			// Validate status code
			if resp != test.wantStatus {
				t.Errorf("Incorrect status code received: want %d, got %d", test.wantStatus, resp)
			}
			// Validate the response for Read/Write for each client
			validateRWResp(t, &test, !deviations.P4RTMissingDelete(dut))
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
			desc:       pDesc,
			lowID:      inId100,
			highID:     inId0,
			handle:     clientConnection(t, dut),
			deviceID:   deviceId,
			wantFail:   false,
			wantStatus: int32(codes.OK),
		}, {
			desc:       sDesc,
			lowID:      inId100,
			highID:     inId0,
			handle:     clientConnection(t, dut),
			deviceID:   deviceId,
			wantFail:   true,
			wantStatus: 3,
		},
	}
	var p4Clients []*p4rt_client.P4RTClient
	for _, test := range testCases {
		t.Run(test.desc, func(t *testing.T) {
			resp, err := streamP4RTArb(&test)
			if err != nil && !test.wantFail {
				t.Errorf("Failed to setup P4RT Client: %v", err)
			}
			if test.wantFail {
				if err != nil {
					t.Logf("Setup of P4RT Client %v failed as expected: %v", sDesc, err)
				} else {
					t.Errorf("Setup of P4RT Client %v did not fail as expected", sDesc)
				}
			}

			// Validate status code
			if resp != test.wantStatus {
				t.Errorf("Incorrect status code received: want %d, got %d", test.wantStatus, resp)
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
	includeDeletes := !deviations.P4RTMissingDelete(dut)
	configureDeviceId(t, dut)
	configurePortId(t, dut)
	testCases := []mcTestArgs{
		{
			name: "Primary_secondary_OK",
			Items: [2]*testArgs{
				{
					desc:            pDesc,
					lowID:           inId101,
					highID:          inId0,
					handle:          clientConnection(t, dut),
					deviceID:        deviceId,
					wantFail:        false,
					wantWrite:       true,
					wantRead:        true,
					wantStatus:      0,
					wantFinalStatus: 6,
				},
				{
					desc:            npDesc,
					lowID:           inId102,
					highID:          inId0,
					handle:          clientConnection(t, dut),
					deviceID:        deviceId,
					wantFail:        false,
					wantWrite:       true,
					wantRead:        true,
					wantStatus:      0,
					wantFinalStatus: 0,
				},
			},
		},
	}
	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			resp, err := streamP4RTArb(test.Items[0])
			if err != nil {
				t.Errorf("Failed to setup P4RT Client: %v", err)
			}
			// Validate status code
			if resp != test.Items[0].wantStatus {
				t.Errorf("Incorrect status code received: want %d, got %d", test.Items[0].wantStatus, resp)
			}
			// Validates that client0 can read/write
			validateRWResp(t, test.Items[0], includeDeletes)
			// Create the stream for client1 (new client)
			newResp, err := streamP4RTArb(test.Items[1])
			if err != nil {
				t.Errorf("Failed to setup P4RT Client: %v", err)
			}
			if newResp != test.Items[1].wantStatus {
				t.Errorf("Incorrect status code received: want %d, got %d", test.Items[1].wantStatus, newResp)
			}
			// Validates that client1 can read/write
			validateRWResp(t, test.Items[1], includeDeletes)
			time.Sleep(1 * time.Second)
			// get first client new response code
			arbResp, err := getRespCode(test.Items[0])
			if err != nil {
				t.Errorf("Failed get response code from primary: %v", err)
			}
			if arbResp != test.Items[0].wantFinalStatus {
				t.Errorf("Incorrect status code received: want %d, got %d", test.Items[0].wantFinalStatus, arbResp)
			}
			// Since client1 has connected with higher election ID
			// the wantWrite flag is moved to false
			test.Items[0].wantWrite = false
			// Validates that client0 can only read, cannot write anymore
			validateRWResp(t, test.Items[0], includeDeletes)
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
	includeDeletes := !deviations.P4RTMissingDelete(dut)
	configureDeviceId(t, dut)
	configurePortId(t, dut)
	test := testArgs{
		desc:       pDesc,
		lowID:      inId102,
		highID:     inId0,
		handle:     clientConnection(t, dut),
		deviceID:   deviceId,
		wantFail:   false,
		wantWrite:  true,
		wantRead:   true,
		wantStatus: int32(codes.OK),
	}
	var p4Clients []*p4rt_client.P4RTClient

	t.Run(test.desc, func(t *testing.T) {
		resp, err := streamP4RTArb(&test)
		if err != nil {
			t.Errorf("Failed to setup P4RT Client: %v", err)
		}
		// Validate status code
		if resp != test.wantStatus {
			t.Errorf("Incorrect status code received: want %d, got %d", test.wantStatus, resp)
		}
		// Validates that client can read/write
		validateRWResp(t, &test, includeDeletes)
		// Updating ElectionID to lower value
		test.lowID = 99
		// After updating electionID, statusCode also changes
		// to secondary without a primary
		test.wantStatus = int32(codes.NotFound)
		if deviations.P4rtBackupArbitrationResponseCode(dut) {
			// Change the expected status code to ALREADY_EXISTS for deviant implementations
			// that send ALREADY_EXISTS instead of NOT_FOUND to secondary clients when there
			// is no primary
			test.wantStatus = int32(codes.AlreadyExists)
		} else {
			test.wantStatus = int32(codes.NotFound)
		}
		// After updating election, wantWrite is false
		// as this client is no longer primary
		test.wantWrite = false
		resp, err = streamP4RTArb(&test)
		if err != nil {
			t.Errorf("Failed to setup P4RT Client: %v", err)
		}
		// Validate status code
		if resp != test.wantStatus {
			t.Errorf("Incorrect status code received: want %d, got %d", test.wantStatus, resp)
		}
		validateRWResp(t, &test, includeDeletes)
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
