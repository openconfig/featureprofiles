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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	p4v1pb "github.com/p4lang/p4runtime/go/p4/v1"
)

// Flag variable definitions
var (
	p4InfoFile = flag.String("p4info_file_location", "../../wbb.p4info.pb.txt", "Path to the p4info file.")
)

// Variable definitions
var (
	gdpEtherType = uint32(0x6007)
	portID       = uint32(10)
	deviceID     = uint64(1)
	streamName   = "p4rt"
)

// PacketIO Interface
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
	wantStatus      codes.Code
	wantFinalStatus codes.Code
	wantFail        bool
}

// configureDeviceID configures p4rt device-id on the DUT.
func configureDeviceID(t *testing.T, dut *ondatra.DUTDevice) {
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

// configurePortID configures p4rt port-id and interface type on the DUT.
func configurePortID(t *testing.T, dut *ondatra.DUTDevice) {
	d := gnmi.OC()
	portName := dut.Port(t, "port1").Name()
	currIntf := &oc.Interface{
		Name: ygot.String(portName),
		Type: oc.IETFInterfaces_InterfaceType_ethernetCsmacd,
		Id:   &portID,
	}
	gnmi.Replace(t, dut, d.Interface(portName).Config(), currIntf)

}

// Create client connection
func clientConnection(t *testing.T, dut *ondatra.DUTDevice) *p4rt_client.P4RTClient {
	clientHandle := p4rt_client.NewP4RTClient(&p4rt_client.P4RTClientParameters{})
	if err := clientHandle.P4rtClientSet(dut.RawAPIs().P4RT(t)); err != nil {
		t.Fatalf("Could not initialize p4rt client: %v", err)
	}
	return clientHandle
}

// Setup P4RT Arbitration Stream which will return a GRPC status code
// in line with the client arbitration notifications, and whether the
// stream was terminated. An error would be returned if request/response
// were unsuccessful.
func streamP4RTArb(args *testArgs) (codes.Code, bool, error) {
	streamParameter := p4rt_client.P4RTStreamParameters{
		Name:        streamName,
		DeviceId:    args.deviceID,
		ElectionIdH: args.highID,
		ElectionIdL: args.lowID,
	}
	if args.handle == nil {
		return codes.OK, false, errors.New("missing client")
	}
	args.handle.StreamChannelCreate(&streamParameter)
	if err := args.handle.StreamChannelSendMsg(&streamName, &p4v1pb.StreamMessageRequest{
		Update: &p4v1pb.StreamMessageRequest_Arbitration{
			Arbitration: &p4v1pb.MasterArbitrationUpdate{
				DeviceId: streamParameter.DeviceId,
				ElectionId: &p4v1pb.Uint128{
					High: streamParameter.ElectionIdH,
					Low:  streamParameter.ElectionIdL,
				},
			},
		},
	}); err != nil {
		return codes.OK, false, fmt.Errorf("errors seen when sending ClientArbitration message: %v", err)
	}
	time.Sleep(1 * time.Second)
	return arbitrationResponseStatus(args)
}

// arbitrationResponseStatus returns the status code received and
// whether the stream was terminated.
// Error returned is nil if a valid arbitration response is received or
// the stream is terminated.
func arbitrationResponseStatus(args *testArgs) (codes.Code, bool, error) {
	handle := args.handle
	if handle == nil {
		return codes.OK, false, errors.New("Missing Client")
	}
	// Grab Arb Response to look at status code
	_, arbResp, arbErr := handle.StreamChannelGetArbitrationResp(&streamName, 1)
	if err := p4rtutils.StreamTermErr(args.handle.StreamTermErr); err != nil {
		return status.Code(err), true, nil
	}
	if arbErr != nil {
		return codes.OK, false, fmt.Errorf("errors seen in ClientArbitration response: %v", arbErr)
	}

	if arbResp == nil {
		return codes.OK, false, errors.New("Missing ClientArbitration response")
	}
	if arbResp.Arb == nil {
		return codes.OK, false, errors.New("Missing MasterArbitrationUpdate response")
	}
	if arbResp.Arb.GetStatus() == nil {
		return codes.OK, false, errors.New("Missing MasterArbitrationUpdate Status in response")
	}
	return codes.Code(arbResp.Arb.GetStatus().GetCode()), false, nil
}

// GetTableEntry creates wbb acl entry related to GDP.
func (gdp *GDPPacketIO) GetTableEntry(delete bool) []*p4rtutils.ACLWbbIngressTableEntryInfo {
	actionType := p4v1pb.Update_INSERT
	if delete {
		actionType = p4v1pb.Update_DELETE
	}
	return []*p4rtutils.ACLWbbIngressTableEntryInfo{{
		Type:          actionType,
		EtherType:     0x6007,
		EtherTypeMask: 0xFFFF,
		Priority:      1,
	}}
}

// writeTableEntry programs or deletes p4rt table entry
func writeTableEntry(t *testing.T, args *testArgs, packetIO PacketIO, delete bool) error {
	t.Helper()

	req := &p4v1pb.WriteRequest{
		DeviceId:   args.deviceID,
		ElectionId: &p4v1pb.Uint128{High: args.highID, Low: args.lowID},
		Updates: p4rtutils.ACLWbbIngressTableEntryGet(
			packetIO.GetTableEntry(delete),
		),
		Atomicity: p4v1pb.WriteRequest_CONTINUE_ON_ERROR,
	}
	if err := args.handle.Write(req); err != nil {
		return err
	}
	return nil
}

// Function to read a table entry returns response/error
func readTableEntry(args *testArgs, t *testing.T) (*p4v1pb.ReadResponse, error) {
	t.Helper()
	readMsg := &p4v1pb.ReadRequest{
		DeviceId: args.deviceID,
		Entities: []*p4v1pb.Entity{
			{
				Entity: &p4v1pb.Entity_TableEntry{
					TableEntry: &p4v1pb.TableEntry{
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
	_, rErr := args.handle.GetForwardingPipelineConfig(&p4v1pb.GetForwardingPipelineConfigRequest{
		DeviceId:     deviceID,
		ResponseType: p4v1pb.GetForwardingPipelineConfigRequest_P4INFO_AND_COOKIE,
	})
	if rErr != nil {
		return false, rErr
	}

	p4Info, err := utils.P4InfoLoad(p4InfoFile)
	if err != nil {
		t.Fatalf("Errors seen when loading p4info file: %v", err)
	}
	//  Verify SetForwardingPipeline fails for unset electionId.
	err = args.handle.SetForwardingPipelineConfig(&p4v1pb.SetForwardingPipelineConfigRequest{
		DeviceId:   deviceID,
		ElectionId: &p4v1pb.Uint128{High: args.highID, Low: args.lowID},
		Action:     p4v1pb.SetForwardingPipelineConfigRequest_VERIFY_AND_COMMIT,
		Config: &p4v1pb.ForwardingPipelineConfig{
			P4Info: p4Info,
			Cookie: &p4v1pb.ForwardingPipelineConfig_Cookie{
				Cookie: 159,
			},
		},
	})
	if err != nil {
		return true, nil
	}
	// If error is nil, client should also be able to read table entry
	_, readErr := readTableEntry(args, t)
	if readErr != nil {
		return false, readErr
	}
	return true, nil
}

// Only Primary should be able to write
func canWrite(t *testing.T, args *testArgs) (bool, error) {
	pktIO := &GDPPacketIO{
		PacketIOPacket: PacketIOPacket{
			EthernetType: &gdpEtherType,
		},
		IngressPort: fmt.Sprint(portID),
	}
	writeErr := writeTableEntry(t, args, pktIO, false)
	if writeErr != nil {
		return false, fmt.Errorf("error writing table entry (highID %d, lowID %d): %v", args.highID, args.lowID, writeErr)
	}
	if writeErr = writeTableEntry(t, args, pktIO, true); writeErr != nil {
		return false, fmt.Errorf("error deleting table entry (highID %d, lowID %d): %v", args.highID, args.lowID, writeErr)
	}
	return true, nil
}

func readWriteTableEntryForStatus(t *testing.T, dut *ondatra.DUTDevice, args *testArgs, status codes.Code) error {
	r, rErr := canRead(t, args)
	w, wErr := canWrite(t, args)
	switch status {
	case codes.InvalidArgument:
		if r {
			return fmt.Errorf("Read allowed for status %v, got: true, want: false", status)
		}
		if w {
			return fmt.Errorf("Write allowed for status %v, got: false, want: true", status)
		}
		return nil
	case codes.AlreadyExists:
		fallthrough
	case codes.NotFound:
		if !r {
			return fmt.Errorf("Read allowed for status %v, got: error(%v) , want: true", status, rErr)
		}
		if w {
			return fmt.Errorf("Write allowed for status %v, got: true, want: false", status)
		}
		return nil
	case codes.OK:
		if !r {
			return fmt.Errorf("Read allowed for status %v, got: error(%v) , want: true", status, rErr)
		}
		if !w {
			return fmt.Errorf("Write allowed for status %v, got: error(%v) , want: true", status, wErr)
		}
		return nil
	default:
		return fmt.Errorf("Unknown status code returned: %v", status)
	}
}

func removeClient(handle *p4rt_client.P4RTClient) {
	handle.StreamChannelDestroy(&streamName)
	handle.ServerDisconnect()
	// Give some time for the client to disconnect
	time.Sleep(2 * time.Second)
}

// Test client with unset electionId
// Both clients should be able to connect as secondary
func TestUnsetElectionid(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	configureDeviceID(t, dut)
	configurePortID(t, dut)
	clients := []testArgs{
		{
			desc:       "Primary",
			handle:     clientConnection(t, dut),
			deviceID:   deviceID,
			wantStatus: codes.NotFound,
		}, {
			desc:       "Secondary",
			handle:     clientConnection(t, dut),
			deviceID:   deviceID,
			wantStatus: codes.NotFound,
		},
	}
	if deviations.P4rtUnsetElectionIDPrimaryAllowed(dut) {
		// For P4 Runtime server implementations that allow unset election id update the
		// expected status to OK for primary and INVALID_ARGUMENT for the secondary
		// connection that connected with the same accepted election id as the
		// primary
		clients[0].wantStatus = codes.OK
		clients[1].wantStatus = codes.InvalidArgument
	}
	// Connect 2 clients to same deviceID with unset electionId.
	for _, test := range clients {
		t.Run(test.desc, func(t *testing.T) {
			streamParameter := p4rt_client.P4RTStreamParameters{
				Name:     streamName,
				DeviceId: test.deviceID,
			}
			if test.handle == nil {
				t.Fatal("p4rt client not found")
			}
			test.handle.StreamChannelCreate(&streamParameter)
			if err := test.handle.StreamChannelSendMsg(&streamName, &p4v1pb.StreamMessageRequest{
				Update: &p4v1pb.StreamMessageRequest_Arbitration{
					Arbitration: &p4v1pb.MasterArbitrationUpdate{
						DeviceId: test.deviceID,
					},
				},
			}); err != nil {
				t.Fatalf("Errors while sending Arbitration Request with unset Election ID: %v", err)
			}
			time.Sleep(1 * time.Second)
			resp, terminated, err := arbitrationResponseStatus(&test)

			// If error in response, the MasterArbitration was not successful.
			if err != nil {
				t.Fatalf("errors seen when sending Master Arbitration for unset ElectionID: %v", err)
			}
			// If InvalidArgument status is not wanted and stream terminates,
			// then it is an error.
			if test.wantStatus != codes.InvalidArgument && terminated {
				t.Fatalf("Stream Terminated for status: %v, want non-termination with status: %v", resp, test.wantStatus)
			}
			if test.wantStatus == codes.InvalidArgument && !terminated {
				t.Fatalf("Stream not Terminated for status: %v, want termination with status: %v", resp, test.wantStatus)
			}
			if resp != test.wantStatus {
				t.Errorf("Incorrect status code received: got %v, want %v", resp, test.wantStatus)
			}
			if terminated {
				return
			}

			// Verify GetForwardingPipeline for unset electionId.
			_, rErr := test.handle.GetForwardingPipelineConfig(&p4v1pb.GetForwardingPipelineConfigRequest{
				DeviceId:     deviceID,
				ResponseType: p4v1pb.GetForwardingPipelineConfigRequest_P4INFO_AND_COOKIE,
			})
			if rErr != nil {
				t.Errorf("Errors seen when sending GetForwardingPipelineConfig: %v", err)
			}
			p4Info, err := utils.P4InfoLoad(p4InfoFile)
			if err != nil {
				t.Errorf("Errors seen when loading p4info file: %v", err)
			}
			//  Verify SetForwardingPipeline fails for unset electionId.
			err = test.handle.SetForwardingPipelineConfig(&p4v1pb.SetForwardingPipelineConfigRequest{
				DeviceId: deviceID,
				Action:   p4v1pb.SetForwardingPipelineConfigRequest_VERIFY_AND_COMMIT,
				Config: &p4v1pb.ForwardingPipelineConfig{
					P4Info: p4Info,
					Cookie: &p4v1pb.ForwardingPipelineConfig_Cookie{
						Cookie: 159,
					},
				},
			})
			if err == nil && test.wantStatus != codes.OK {
				t.Errorf("SetForwardingPipelineConfig accepted for unset Election ID: %v", err)
			}
			if err != nil && test.wantStatus == codes.OK {
				t.Errorf("SetForwardingPipelineConfig unexpectedly failed for Election ID: %v", err)
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
	configureDeviceID(t, dut)
	configurePortID(t, dut)
	testCases := []testArgs{
		{
			desc:       "Primary",
			lowID:      100,
			highID:     0,
			handle:     clientConnection(t, dut),
			deviceID:   deviceID,
			wantFail:   false,
			wantStatus: codes.OK,
		}, {
			desc:       "Secondary",
			lowID:      90,
			highID:     0,
			handle:     clientConnection(t, dut),
			deviceID:   deviceID,
			wantFail:   false,
			wantStatus: codes.NotFound,
		}, {
			desc:       "Primary Reconnect",
			lowID:      100,
			highID:     0,
			handle:     clientConnection(t, dut),
			deviceID:   deviceID,
			wantFail:   false,
			wantStatus: codes.OK,
		},
	}
	if deviations.P4rtBackupArbitrationResponseCode(dut) {
		// Change the expected status code to ALREADY_EXISTS for deviant implementations
		// that send ALREADY_EXISTS instead of NOT_FOUND to secondary clients when there
		// is no primary
		testCases[1].wantStatus = codes.AlreadyExists
	}
	for _, test := range testCases {
		t.Run(test.desc, func(t *testing.T) {
			resp, terminated, err := streamP4RTArb(&test)
			if err != nil {
				t.Fatalf("Failed to setup P4RT Client: %v", err)
			}
			if terminated {
				t.Fatalf("Stream terminated. got status: %v, want: non-termination with status: %v", resp, test.wantStatus)
			}
			// Validate status code
			if resp != test.wantStatus {
				t.Fatalf("Incorrect status code received: got %v, want %v", resp, test.wantStatus)
			}
			// Validate the response for Read/Write for each client
			if err := readWriteTableEntryForStatus(t, dut, &test, test.wantStatus); err != nil {
				t.Fatalf("Error on Read Write Table entry for status %v: %v", test.wantStatus, err)
			}
			// Disconnect Primary
			removeClient(test.handle)
		})
	}
}

// // Test Primary/Secondary clients
func TestPrimarySecondary(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	configureDeviceID(t, dut)
	configurePortID(t, dut)
	testCases := []testArgs{
		{
			desc:       "Primary",
			lowID:      100,
			highID:     0,
			handle:     clientConnection(t, dut),
			deviceID:   deviceID,
			wantFail:   false,
			wantStatus: codes.OK,
		}, {
			desc:       "Secondary",
			lowID:      90,
			highID:     0,
			handle:     clientConnection(t, dut),
			deviceID:   deviceID,
			wantFail:   false,
			wantStatus: codes.AlreadyExists,
		},
	}
	var p4Clients []*p4rt_client.P4RTClient
	for _, test := range testCases {
		t.Run(test.desc, func(t *testing.T) {
			resp, terminated, err := streamP4RTArb(&test)
			if err != nil {
				t.Fatalf("Failed to setup P4RT Client: %v", err)
			}
			if terminated {
				t.Fatalf("Stream terminated. got status: %v, want: non-termination with status: %v", resp, test.wantStatus)
			}
			// Validate status code
			if resp != test.wantStatus {
				t.Errorf("Incorrect status code received: got %v, want %v", resp, test.wantStatus)
			}
			// Validate the response for Read/Write for each client
			if err := readWriteTableEntryForStatus(t, dut, &test, test.wantStatus); err != nil {
				t.Fatalf("Error on Read Write Table entry for status %v: %v", test.wantStatus, err)
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

// // Test Duplicate ElectionID
func TestDuplicateElectionID(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	configureDeviceID(t, dut)
	configurePortID(t, dut)
	testCases := []testArgs{
		{
			desc:       "Primary",
			lowID:      100,
			highID:     0,
			handle:     clientConnection(t, dut),
			deviceID:   deviceID,
			wantFail:   false,
			wantStatus: codes.OK,
		}, {
			desc:       "Duplicate ElectionID",
			lowID:      100,
			highID:     0,
			handle:     clientConnection(t, dut),
			deviceID:   deviceID,
			wantFail:   true,
			wantStatus: codes.InvalidArgument,
		},
	}
	var p4Clients []*p4rt_client.P4RTClient
	for _, test := range testCases {
		t.Run(test.desc, func(t *testing.T) {
			resp, terminated, err := streamP4RTArb(&test)
			if err != nil {
				t.Fatalf("Failed to setup P4RT Client: %v", err)
			}
			if !test.wantFail && terminated {
				t.Fatalf("Stream Terminated for status: %v, want non-termination with status: %v", resp, test.wantStatus)
			}
			if test.wantFail && !terminated {
				t.Fatalf("Stream not Terminated for status: %v, want termination with status: %v", resp, test.wantStatus)
			}

			// Validate status code
			if resp != test.wantStatus {
				t.Errorf("Incorrect status code received: got %v, want %v", resp, test.wantStatus)
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
	configureDeviceID(t, dut)
	configurePortID(t, dut)
	testCases := []mcTestArgs{
		{
			name: "Primary_secondary_OK",
			Items: [2]*testArgs{
				{
					desc:            "Primary",
					lowID:           101,
					highID:          0,
					handle:          clientConnection(t, dut),
					deviceID:        deviceID,
					wantFail:        false,
					wantStatus:      codes.OK,
					wantFinalStatus: codes.AlreadyExists,
				},
				{
					desc:            "New Primary",
					lowID:           102,
					highID:          0,
					handle:          clientConnection(t, dut),
					deviceID:        deviceID,
					wantFail:        false,
					wantStatus:      codes.OK,
					wantFinalStatus: codes.OK,
				},
			},
		},
	}
	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			resp, terminated, err := streamP4RTArb(test.Items[0])
			if err != nil {
				t.Fatalf("Failed to setup P4RT Client: %v", err)
			}
			if terminated {
				t.Fatalf("Stream Terminated for status: %v, want non-termination with status: %v", resp, test.Items[0].wantStatus)
			}
			// Validate status code
			if resp != test.Items[0].wantStatus {
				t.Errorf("Incorrect status code received: want %d, got %d", test.Items[0].wantStatus, resp)
			}
			// Validates that client0 can read/write
			if err := readWriteTableEntryForStatus(t, dut, test.Items[0], test.Items[0].wantStatus); err != nil {
				t.Errorf("Error on Read Write Table entry for status %v: %v", test.Items[0].wantStatus, err)
			}
			// Create the stream for client1 (new client)
			newResp, terminated, err := streamP4RTArb(test.Items[1])
			if err != nil {
				t.Errorf("Failed to setup P4RT Client: %v", err)
			}
			if terminated {
				t.Fatalf("Stream Terminated for status: %v, want non-termination with status: %v", newResp, test.Items[1].wantStatus)
			}
			if newResp != test.Items[1].wantStatus {
				t.Errorf("Incorrect status code received: got %d, want %d", newResp, test.Items[1].wantStatus)
			}
			// Validates that client1 can read/write
			if err := readWriteTableEntryForStatus(t, dut, test.Items[1], test.Items[1].wantStatus); err != nil {
				t.Errorf("Error on Read Write Table entry for status %v: %v", test.Items[1].wantStatus, err)
			}
			time.Sleep(1 * time.Second)
			// get first client new response code
			arbResp, terminated, err := arbitrationResponseStatus(test.Items[0])
			if err != nil {
				t.Errorf("Failed get response code from primary: %v", err)
			}
			if terminated {
				t.Fatalf("Stream Terminated for old primary with status: %v, want non-termination with status: %v", arbResp, test.Items[0].wantFinalStatus)
			}
			if arbResp != test.Items[0].wantFinalStatus {
				t.Errorf("Incorrect status code received: want %d, got %d", test.Items[0].wantFinalStatus, arbResp)
			}

			// Validates that client0 can only read, cannot write anymore
			if err := readWriteTableEntryForStatus(t, dut, test.Items[0], test.Items[0].wantFinalStatus); err != nil {
				t.Errorf("Error on Read Write Table entry for status %v: %v", test.Items[0].wantFinalStatus, err)
			}
		})
		// Teardown clients
		for _, item := range test.Items {
			item.handle.StreamChannelDestroy(&streamName)
			item.handle.ServerDisconnect()
		}
	}
}

// // Test arbitration update from same client
func TestArbitrationUpdate(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	configureDeviceID(t, dut)
	configurePortID(t, dut)
	test := testArgs{
		desc:       "Primary to Secondary fallback on ElectionID update",
		lowID:      102,
		highID:     0,
		handle:     clientConnection(t, dut),
		deviceID:   deviceID,
		wantFail:   false,
		wantStatus: codes.OK,
	}
	var p4Clients []*p4rt_client.P4RTClient

	t.Run(test.desc, func(t *testing.T) {
		resp, terminated, err := streamP4RTArb(&test)
		if err != nil {
			t.Fatalf("Failed to setup P4RT Client: %v", err)
		}
		if terminated {
			t.Fatalf("Stream Terminated for status: %v, want non-termination with status: %v", resp, test.wantStatus)
		}
		// Validate status code
		if resp != test.wantStatus {
			t.Errorf("Incorrect status code received: want %d, got %d", test.wantStatus, resp)
		}
		// Validates that client can read/write
		if err := readWriteTableEntryForStatus(t, dut, &test, test.wantStatus); err != nil {
			t.Fatalf("Error on Read Write Table entry for status %v: %v", test.wantStatus, err)
		}
		// Updating ElectionID to lower value
		test.lowID = 99
		// After updating electionID, statusCode also changes
		// to secondary without a primary
		test.wantStatus = codes.NotFound
		if deviations.P4rtBackupArbitrationResponseCode(dut) {
			// Change the expected status code to ALREADY_EXISTS for deviant implementations
			// that send ALREADY_EXISTS instead of NOT_FOUND to secondary clients when there
			// is no primary.
			test.wantStatus = codes.AlreadyExists
		}

		resp, terminated, err = streamP4RTArb(&test)
		if err != nil {
			t.Errorf("Failed to setup P4RT Client: %v", err)
		}
		if terminated {
			t.Fatalf("Stream Terminated for status: %v, want non-termination with status: %v", resp, test.wantStatus)
		}
		// Validate status code
		if resp != test.wantStatus {
			t.Errorf("Incorrect status code received: want %d, got %d", test.wantStatus, resp)
		}

		// After updating election, wantWrite is false as this client is no longer primary.
		if err := readWriteTableEntryForStatus(t, dut, &test, test.wantStatus); err != nil {
			t.Errorf("Error on Read Write Table entry for status %v: %v", test.wantStatus, err)
		}
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
