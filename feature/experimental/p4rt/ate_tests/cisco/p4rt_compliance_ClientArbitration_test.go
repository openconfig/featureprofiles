package cisco_p4rt_test

import (
	"context"
	"testing"
	"time"

	p4rt_client "github.com/cisco-open/go-p4/p4rt_client"
	"github.com/golang/protobuf/ptypes/any"
	p4_v1 "github.com/p4lang/p4runtime/go/p4/v1"
)

var (
	P4RTComplianceClientArbitration = []Testcase{
		{
			name: "Send default role info in Client Arbitration",
			desc: "client arbitration-Compliance:001 Controller sends default role info in streamchannel rpc MasterArbitrationUpdate msg",
			fn:   testDefaultRoleInfoInClientArbitration,
		},
		{
			name: "Send non-default role info in Client Arbitration",
			desc: "client arbitration-Compliance:002 Controller sends non-default role info in streamchannel rpc MasterArbitrationUpdate msg",
			fn:   testNonDefaultRoleInfoInClientArbitration,
		},
		{
			name: "Use same Primary for difference device in Client Arbitration",
			desc: "client arbitration-Compliance:003 Same primary controller for dfferent unit(npu) on the same box",
			fn:   testSamePrimaryForDifferentNPU,
		},
		{
			name: "Use different Primary for difference device with same grpc stub in Client Arbitration",
			desc: "client arbitration-Compliance:004 Different primary for differnet units(npu) on the same box via same grpc stub",
			fn:   testDifferentPrimaryForDifferentNPUWithSameGRPC,
		},
		{
			name: "Use different Primary for difference device with different grpc stub in Client Arbitration",
			desc: "client arbitration-Compliance:005 Different primary for differnet units(npu) on the same box via different grpc stub",
			fn:   testDifferentPrimaryForDifferentNPUWithDifferentGRPC,
		},
		{
			name: "Usene Primary controller as Backup Controller on another device Id in Client Arbitration",
			desc: "client arbitration-Compliance:006 Primary controller in one unit works as backup controller in another unit",
			fn:   testPrimaryControllerWorksAsBackupOnDifferentNPU,
		},
		{
			name: "Setup One Primary with Multiple Backup for the same device Id in Client Arbitration",
			desc: "client arbitration-Compliance:007 One priamry controller with more than 1 backup controller for the same unit",
			fn:   testOnePrimaryWithMultipleBackup,
		},
		{
			name: "Use same Election Id for differnet device Id in Client Arbitration",
			desc: "client arbitration-Compliance:008 Controller sends same election-id for the different device-id on the same box",
			fn:   testSameElectionIDForDifferentNPU,
		},
		{
			name: "Verify advisory message from backup controller on same device Id in Client Arbitration",
			desc: "client arbitration-Compliance:009 Verify the advisory message is sent out to other standby controllers on the same device-id when stream channel is broken for primary controller",
			fn:   testAdvisoryMessageOnSameNPU,
		},
		{
			name: "Verify advisory message from controller on different device Id in Client Arbitration",
			desc: "client arbitration-Compliance:010 Verify the advisory message is NOT sent out to other controllers on the different device-id when stream channel is broken for primary controller",
			fn:   testAdvisoryMessageOnDifferentNPU,
		},
		{
			name: "Send arbitration message with non-exist device Id in Client Arbitration",
			desc: "client arbitration-Compliance:011 5.3.1(a) device_id not found on device, terminate streamchannel and return NOT_FOUND error",
			fn:   testNonExistDeviceIdInClientArbitration,
			skip: true,
		},
		{
			name: "Send same election Id as primary controller from other device controller in Client Arbitration",
			desc: "client arbitration-Compliance:012 5.3.1(b) same election-id already used for other controller(primary) on same device-id, terminate stream channel and return INVALID_ARGUMENT",
			fn:   testSameElectionIdAsPrimary,
			skip: true,
		},
		{
			name: "Send same election Id as primary controller from same device backup controller in Client Arbitration",
			desc: "client arbitration-Compliance:013 5.3.1(b) same election-id already used for other controller(standby) on same device-id, terminate stream channel and return INVALID_ARGUMENT",
			fn:   testSameElectionIdAsPrimaryFromBackup,
		},
		{
			name: "Send role info from new controller in Client Arbitration",
			desc: "client arbitration-Compliance:014 5.3.1(c) role.config doesn't match, return INVALID_ARUGMENT. This would be no-op on xr.",
			fn:   testNewControllerWithDifferentRole,
		},
		{
			name: "Trigger OOR with new primary controller in Client Arbitration",
			desc: "client arbitration-Compliance:015 5.3.1(d) # of open stream exceeds the support limit, terminate stream and returning RESOURCE_EXHAUSTED error by initiating a new Primary.",
			fn:   testClientArbitrationOORWithNewPrimary,
		},
		{
			name: "Trigger OOR with new backup controller in Client Arbitration",
			desc: "client arbitration-Compliance:016 5.3.1(d) # of open stream exceeds the support limit, terminate stream and returning RESOURCE_EXHAUSTED error by initiating a new Standby",
			fn:   testClientArbitrationOORWithNewBackup,
		},
		{
			name: "Send different device Id from existing connected controller in Client Arbitration",
			desc: "client arbitration-Compliance:017 5.3.2(a) MasterArbitrationUpdate received on already connected controller, if devide_id doesn't match existing, terminate stream and return FAILED_PRECONDITION",
			fn:   testDiffernetDeviceIdOnExistingConnectedController,
		},
		{
			name: "Send different role from existing connected controller in Client Arbitration",
			desc: "client arbitration-Compliance:018 5.3.2(b) MasterArbitrationUpdate received on already connected controller, if role doesn't match exsiting, terminate stream and return FAILED_PRECONDITION",
			fn:   testDiffernetRoleOnExistingConnectedController,
		},
		{
			name: "Send different role config from existing connected controller in Client Arbitration",
			desc: "client arbitration-Compliance:019 5.3.2(c) MasterArbitrationUpdate received on already connected controller, if role.config doesn't match exsiting, return INVALID_ARGUMENT",
			fn:   testDiffernetRoleConfigOnExistingConnectedController,
		},
		{
			name: "New controller sends same election Id as existing connected primary controller in Client Arbitration",
			desc: "client arbitration-Compliance:020 5.3.2(d) MasterArbitrationUpdate received on already connected controller, if election id is the same as existing, terminate stream and return INVALID_ARUGMENT. Same election-id sent from the different controller (for different sessions)",
			fn:   testSameElectionIdFromDifferentController,
			skip: true,
		},
		{
			name: "New controller sends different role and same election Id as existing connected primary controller in Client Arbitration",
			desc: "client arbitration-Compliance:021 5.3.2(e)(i)MasterArbitrationUpdate received on already connected controller, if election id is the same as existing related connected primary controller, verify role.config is updated and advisory message is sent to all controller related to that device-id + role",
			fn:   testDiffernetRoleAndSameElectionIdFromConnectedController,
		},
		{
			name: "Existing controller sends same election Id as existing connected backup controller in Client Arbitration",
			desc: "client arbitration-Compliance:022 5.3.2(e)(ii) MasterArbitrationUpdate received on already connected controller, if election id is the same as existing related connected backup controller, verify it's no-op on the server and role.config is ignored. No response is sent to any controller",
			fn:   testSameElectionIdAsBackupController,
			skip: true,
		},
		{
			name: "Existing controller sends different election Id with same config in Client Arbitration",
			desc: "client arbitration-Compliance:023 5.3.2(f) MasterArbitrationUpdate received on already connected controller, device_id, role, role-config matches and election-id is different. the server updates it's election_id for the related controller.",
			fn:   testDifferentElectionIdFromController,
		},
		{
			name: "New Primary without ongoing write in Client Arbitration",
			desc: "client arbitration-Compliance:024 5.3.2(f)(1a) MasterArbitrationUpdate is accepted and processed, the controller will be primary based on the election-id and role. Also there are not previous primary or NO on-going Write message from previous primary, server immedidately sends advisory notification",
			fn:   testNewPrimary,
			skip: true,
		},
		{
			name: "Write from previous Primary fails after new Primary takes over in Client Arbitration",
			desc: "client arbitration-Compliance:025 5.3.2(f)(1b)(i) MasterArbitrationUpdate is accepted and processed, the controller will be primary based on the election-id and role. Also there is previous primary oron-going Write message from previous primary. Verify Server stop accepting new writes and reject new Write request with PERMISSION_DENIED",
			fn:   testNewPrimaryWithWriteFromPreviousPrimary,
		},
		{
			name: "New Primary notify non-primary controller in Client Arbitration",
			desc: "client arbitration-Compliance:026 5.3.2(f)(1b)(ii) MasterArbitrationUpdate is accepted and processed, the controller will be primary based on the election-id and role. Also there is previous primary oron-going Write message from previous primary.Verify Server notific other controllers of the new primary via advisory message",
			fn:   testNewPrimaryNotifyOtherController,
			skip: true,
		},
		{
			name: "New Primary connects with existing on-going write from prevoius controller in Client Arbitration",
			desc: "client arbitration-Compliance:027 5.3.2(f)(1b)(iii) MasterArbitrationUpdate is accepted and processed, the controller will be primary based on the election-id and role. Also there is previous primary oron-going Write message from previous primary. Verify Server continue processing on-going writes and return error to previous primary controller if there is any",
			fn:   testNewPrimaryWithOngoingWrite,
		},
		{
			name: "Write from new Primary passes controller in Client Arbitration",
			desc: "client arbitration-Compliance:028 5.3.2(f)(1b)(iv) MasterArbitrationUpdate is accepted and processed, the controller will be primary based on the election-id and role. Also there is previous primary oron-going Write message from previous primary.Verify Server updates its stroed election-id and also accept write from new primary",
			fn:   testNewPrimaryWithWriteFromNewPrimary,
		},
		{
			name: "New Primary connects and server notifies new Primary in Client Arbitration",
			desc: "client arbitration-Compliance:029 5.3.2(f)(1b)(v) MasterArbitrationUpdate is accepted and processed, the controller will be primary based on the election-id and role. Also there is previous primary or on-going Write message from previous primary. Verify server notifies new priamry by sending advisory message",
			fn:   testSeverNotifyNewPrimary,
		},
		{
			name: "Primary controller downgrade to backup in Client Arbitration",
			desc: "client arbitration-Compliance:030 5.3.2(f)(2) MasterArbitrationUpdate is accepted and processed, the controller will be backup based on the election-id and role. If the previous primary controller downgrade, an advisory message is sent to all controllers for that device_id and role",
			fn:   testPrimaryDowngrade,
		},
		{
			name: "New contorller as backup controller in Client Arbitration",
			desc: "client arbitration-Compliance:031 5.3.2(f)(2) MasterArbitrationUpdate is accepted and processed, the controller will be backup based on the election-id and role. It's just a new backup controller and server send an advisory message to that controller only",
			fn:   testNewBackup,
			skip: true,
		},
		{
			name: "ElectionID is left empty in StreamMessageResponse when there was no Primary Controller",
			desc: "client arbitration-Compliance:032 5.4 Server notifiy controllers via StreamMessageResponse. when there was not primary, verify election-id is field is left unset",
			fn:   testNoPrimary,
		},
		{
			name: "ElectionID is set as highest in StreamMessageResponse when there was Primary Controller",
			desc: "client arbitration-Compliance:033 5.4 Server notifiy controllers via StreamMessageResponse. when there were primary controllers, verify election-id is field is set to the higheset value",
			fn:   testElectionIDinStreamMessageResponse,
			skip: true,
		},
		{
			name: "Status code is set as non-OK for backup and is set as OK for primary after new connection",
			desc: "client arbitration-Compliance:034 5.4 Server notifiy controllers via StreamMessageResponse. When there is primary controller current,  Verify non priamry controllers receives with status code set to non-OK and primary controller receive status OK",
			fn:   testStatusinStreamMessageResponse,
			skip: true,
		},
		{
			name: "Status code is set as non-OK for backup controller without primary controller after new connection",
			desc: "client arbitration-Compliance:035 5.4 Server notifiy controllers via StreamMessageResponse. When is no primary currently, Verify non priamry controllers receives with status code set to non-OK",
			fn:   testStatusinStreamMessageResponseWithoutPrimary,
		},
		{
			name: "ClientArbitration without electionID is treated as Backup Controller",
			desc: "client arbitration-Compliance:036 16.2 controller initiates without sending election-id and verify it's treated as backup controller",
			fn:   testBackupOnly,
		},
	}
)

// client arbitration-Compliance:001
func testDefaultRoleInfoInClientArbitration(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA

	// Setup P4RT Client
	if err := setupConnection(ctx, t, generateStreamParameter(deviceID, uint64(0), electionID), client); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, client)
}

// client arbitration-Compliance:002
func testNonDefaultRoleInfoInClientArbitration(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA

	// Setup P4RT Client
	streamParameter := generateStreamParameter(deviceID, uint64(0), electionID)
	client.StreamChannelCreate(&streamParameter)
	if err := client.StreamChannelSendMsg(&streamName, &p4_v1.StreamMessageRequest{
		Update: &p4_v1.StreamMessageRequest_Arbitration{
			Arbitration: &p4_v1.MasterArbitrationUpdate{
				DeviceId: streamParameter.DeviceId,
				ElectionId: &p4_v1.Uint128{
					High: streamParameter.ElectionIdH,
					Low:  streamParameter.ElectionIdL,
				},
				Role: &p4_v1.Role{
					Id: uint64(deviceID),
					Config: &any.Any{
						TypeUrl: "test",
					},
				},
			},
		},
	}); err != nil {
		t.Errorf("There is error when setting up client with Role info")
	}
	_, _, arbErr := client.StreamChannelGetArbitrationResp(&streamParameter.Name, 1)

	if arbErr != nil {
		t.Errorf("There is error at Arbitration time: %v", arbErr)
	}

	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, client)
}

// client arbitration-Compliance:003
func testSamePrimaryForDifferentNPU(ctx context.Context, t *testing.T, args *testArgs) {
	t.Skip()
	client := args.p4rtClientA

	// Setup P4RT Client
	streamParameter := generateStreamParameter(deviceID, uint64(0), electionID)

	// Setup connection for default deviceID
	if err := setupConnection(ctx, t, streamParameter, client); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}

	// Setup connnection for deviceID+1
	streamParameter.DeviceId += 1
	streamParameter.Name += "_new"
	if err := setupConnection(ctx, t, streamParameter, client); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}

	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, client)
}

// client arbitration-Compliance:004
func testDifferentPrimaryForDifferentNPUWithSameGRPC(ctx context.Context, t *testing.T, args *testArgs) {
	if identifiedNPUs <= 1 {
		t.Skip("Not enough NPUs to run this test")
	}
	client := args.p4rtClientA

	// Setup P4RT Client
	streamParameter := generateStreamParameter(deviceID, uint64(0), electionID)

	// Setup connection for default deviceID
	if err := setupConnection(ctx, t, streamParameter, client); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}

	// Setup connnection for deviceID+1
	streamParameter.DeviceId += 1
	streamParameter.ElectionIdL += 1
	if err := setupConnection(ctx, t, streamParameter, client); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}

	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, client)
}

// client arbitration-Compliance:005
func testDifferentPrimaryForDifferentNPUWithDifferentGRPC(ctx context.Context, t *testing.T, args *testArgs) {
	if identifiedNPUs <= 1 {
		t.Skip("Not enough NPUs to run this test")
	}
	clientA := args.p4rtClientA
	clientB := args.p4rtClientB

	// Setup P4RT Client
	streamParameter := generateStreamParameter(deviceID, uint64(0), electionID)

	// Setup connection for default deviceID
	if err := setupConnection(ctx, t, streamParameter, clientA); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, clientA)

	// Setup connnection for deviceID+1
	streamParameter.DeviceId += 1
	streamParameter.ElectionIdL += 1
	if err := setupConnection(ctx, t, streamParameter, clientB); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}

	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, clientB)
}

// client arbitration-Compliance:006
func testPrimaryControllerWorksAsBackupOnDifferentNPU(ctx context.Context, t *testing.T, args *testArgs) {
	if identifiedNPUs <= 1 {
		t.Skip("Not enough NPUs to run this test")
	}
	clientA := args.p4rtClientA
	clientB := args.p4rtClientB

	// Setup P4RT Client
	streamParameter := generateStreamParameter(deviceID, uint64(0), electionID)

	// Setup connection for default deviceID
	if err := setupConnection(ctx, t, streamParameter, clientA); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, clientA)

	// Setup connnection for deviceID+1
	streamParameter.DeviceId += 1
	streamParameter.ElectionIdL += 1
	if err := setupConnection(ctx, t, streamParameter, clientB); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}

	// Setup backup controller with clientA on deviceID+1
	streamParameter.ElectionIdL -= 1
	streamParameter.Name = "Backup"
	if err := setupConnection(ctx, t, streamParameter, clientA); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}

	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, clientB)
}

// client arbitration-Compliance:007
func testOnePrimaryWithMultipleBackup(ctx context.Context, t *testing.T, args *testArgs) {
	clients := []*p4rt_client.P4RTClient{
		args.p4rtClientA,
		args.p4rtClientB,
		args.p4rtClientC,
		args.p4rtClientD,
	}

	// Setup P4RT Client
	streamParameter := generateStreamParameter(deviceID, uint64(0), electionID)

	for index, client := range clients {
		// Setup connection for default deviceID
		streamParameter.ElectionIdL -= uint64(index)

		if err := setupConnection(ctx, t, streamParameter, client); err != nil {
			t.Errorf("There is error setting up connection, %s", err)
		}

		// Destroy P4RT Client
		defer teardownConnection(ctx, t, deviceID, client)
	}
}

// client arbitration-Compliance:008
func testSameElectionIDForDifferentNPU(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA

	// Setup P4RT Client
	streamParameter := generateStreamParameter(deviceID, uint64(0), electionID)

	// Setup connection for default deviceID
	if err := setupConnection(ctx, t, streamParameter, client); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}

	// Setup connnection for deviceID+1
	streamParameter.DeviceId += 1
	if err := setupConnection(ctx, t, streamParameter, client); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}

	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, client)
}

// client arbitration-Compliance:009
func testAdvisoryMessageOnSameNPU(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA

	// Setup P4RT Client
	streamParameter := generateStreamParameter(deviceID, uint64(0), electionID)

	// Setup connection for default deviceID
	if err := setupConnection(ctx, t, streamParameter, client); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}

	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, client)

	// Setup backup connnection
	streamParameter.Name = "Backup"
	streamParameter.ElectionIdL -= 1
	if err := setupConnection(ctx, t, streamParameter, client); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}

	// Disconnect stream channel
	client.StreamChannelDestroy(&streamName)

	// Verify message received
	_, resp, err := client.StreamChannelGetArbitrationResp(&streamParameter.Name, 1)
	if err != nil {
		t.Errorf("There is error when getting Arbitration Response, %v", err)
	}
	if resp == nil || len(resp.Arb.String()) == 0 {
		t.Errorf("Advisory Messge is not received")
	}
	// if len(resp.Arb.String()) == 0 {
	// 	t.Errorf("Advisory Messge is not received")
	// }
}

// client arbitration-Compliance:010
func testAdvisoryMessageOnDifferentNPU(ctx context.Context, t *testing.T, args *testArgs) {
	if identifiedNPUs <= 1 {
		t.Skip("Not enough NPUs to run this test")
	}
	client := args.p4rtClientA

	// Setup P4RT Client
	streamParameter := generateStreamParameter(deviceID, uint64(0), electionID)

	// Setup connection for default deviceID
	if err := setupConnection(ctx, t, streamParameter, client); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}

	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, client)

	// Setup connnection for deviceID+1
	streamParameter.DeviceId += 1
	streamParameter.Name += "_new"
	if err := setupConnection(ctx, t, streamParameter, client); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}

	// Disconnect stream channel
	client.StreamChannelDestroy(&streamName)

	// Verify message received
	_, resp, err := client.StreamChannelGetArbitrationResp(&streamParameter.Name, 1)
	if err != nil {
		t.Errorf("There is error when getting Arbitration Response, %v", err)
	}

	if resp == nil || len(resp.Arb.String()) == 0 {
		t.Errorf("Advisory Message is not received")
	}
}

// client arbitration-Compliance:011
func testNonExistDeviceIdInClientArbitration(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA

	// Setup P4RT Client
	if err := setupConnection(ctx, t, generateStreamParameter(uint64(1), uint64(0), electionID), client); err == nil {
		t.Errorf("Expected error is not seen.")
	}

	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, client)
}

// client arbitration-Compliance:012
func testSameElectionIdAsPrimary(ctx context.Context, t *testing.T, args *testArgs) {
	clientA := args.p4rtClientA
	clientB := args.p4rtClientB

	// Setup P4RT Client
	streamParameter := generateStreamParameter(deviceID, uint64(0), electionID)

	// Setup connection for default deviceID
	if err := setupConnection(ctx, t, streamParameter, clientA); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, clientA)

	// Setup connnection for deviceID
	if err := setupConnection(ctx, t, streamParameter, clientB); err == nil {
		t.Errorf("Expected error is not seen.")
	}

	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, clientB)
}

// client arbitration-Compliance:013
func testSameElectionIdAsPrimaryFromBackup(ctx context.Context, t *testing.T, args *testArgs) {
	clientA := args.p4rtClientA
	clientB := args.p4rtClientB

	// Setup P4RT Client
	streamParameter := generateStreamParameter(deviceID, uint64(0), electionID)

	// Setup connection for default deviceID
	if err := setupConnection(ctx, t, streamParameter, clientA); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, clientA)

	// Setup connnection for deviceID with electionID-1
	streamParameter.ElectionIdL -= 1
	if err := setupConnection(ctx, t, streamParameter, clientB); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}

	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, clientB)

	// Update electionID
	streamParameter.ElectionIdL += 1
	// if err := setupConnection(ctx, t, streamParameter, clientB); err == nil {
	// 	t.Errorf("Expected error is not seen.")
	// }
	if err := clientB.StreamChannelSendMsg(&streamName, &p4_v1.StreamMessageRequest{
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
		t.Errorf("There is error when setting up p4rtClientA")
	}
	time.Sleep(10 * time.Second)

	_, _, arbErr := clientB.StreamChannelGetArbitrationResp(&streamParameter.Name, 1)

	t.Logf("Seeing error %s.", arbErr)
	if arbErr == nil {
		t.Errorf("Expected error is not seen.")
	}
}

// client arbitration-Compliance:014
func testNewControllerWithDifferentRole(ctx context.Context, t *testing.T, args *testArgs) {
	clientA := args.p4rtClientA
	clientB := args.p4rtClientB

	streamParameter := generateStreamParameter(deviceID, uint64(0), electionID)

	// Setup connection for default deviceID
	if err := setupConnection(ctx, t, streamParameter, clientA); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, clientA)

	// Setup P4RT Client
	clientB.StreamChannelCreate(&streamParameter)
	if err := clientB.StreamChannelSendMsg(&streamName, &p4_v1.StreamMessageRequest{
		Update: &p4_v1.StreamMessageRequest_Arbitration{
			Arbitration: &p4_v1.MasterArbitrationUpdate{
				DeviceId: streamParameter.DeviceId,
				ElectionId: &p4_v1.Uint128{
					High: streamParameter.ElectionIdH,
					Low:  streamParameter.ElectionIdL + 1,
				},
				Role: &p4_v1.Role{
					Id: uint64(deviceID),
					Config: &any.Any{
						TypeUrl: "test",
					},
				},
			},
		},
	}); err != nil {
		t.Errorf("There is error when setting up client with Role info")
	}
	_, _, arbErr := clientB.StreamChannelGetArbitrationResp(&streamParameter.Name, 1)

	if arbErr != nil {
		t.Errorf("There is error at Arbitration time: %v", arbErr)
	}

	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, clientB)
}

// client arbitration-Compliance:015
func testClientArbitrationOORWithNewPrimary(ctx context.Context, t *testing.T, args *testArgs) {
	t.Logf("TODO: Add new clients to simulate OOR condition")
	t.Skip()
}

// client arbitration-Compliance:016
func testClientArbitrationOORWithNewBackup(ctx context.Context, t *testing.T, args *testArgs) {
	t.Logf("TODO: Add new clients to simulate OOR condition")
	t.Skip()
}

// client arbitration-Compliance:017
func testDiffernetDeviceIdOnExistingConnectedController(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA

	streamParameter := generateStreamParameter(deviceID, uint64(0), electionID)

	// Setup connection for default deviceID
	if err := setupConnection(ctx, t, streamParameter, client); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, client)

	if err := client.StreamChannelSendMsg(&streamName, &p4_v1.StreamMessageRequest{
		Update: &p4_v1.StreamMessageRequest_Arbitration{
			Arbitration: &p4_v1.MasterArbitrationUpdate{
				DeviceId: streamParameter.DeviceId + 1,
				ElectionId: &p4_v1.Uint128{
					High: streamParameter.ElectionIdH,
					Low:  streamParameter.ElectionIdL + 1,
				},
			},
		},
	}); err != nil {
		t.Errorf("There is error when setting up client with Role info")
	}
	_, _, arbErr := client.StreamChannelGetArbitrationResp(&streamParameter.Name, 1)

	if arbErr == nil {
		t.Errorf("Expected error is not seen.")
	}
}

// client arbitration-Compliance:018
func testDiffernetRoleOnExistingConnectedController(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA

	streamParameter := generateStreamParameter(deviceID, uint64(0), electionID)

	// Setup connection for default deviceID
	if err := setupConnection(ctx, t, streamParameter, client); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, client)

	// Setup P4RT Client
	if err := client.StreamChannelSendMsg(&streamName, &p4_v1.StreamMessageRequest{
		Update: &p4_v1.StreamMessageRequest_Arbitration{
			Arbitration: &p4_v1.MasterArbitrationUpdate{
				DeviceId: streamParameter.DeviceId,
				ElectionId: &p4_v1.Uint128{
					High: streamParameter.ElectionIdH,
					Low:  streamParameter.ElectionIdL + 1,
				},
				Role: &p4_v1.Role{
					Id: uint64(deviceID),
					Config: &any.Any{
						TypeUrl: "test",
					},
				},
			},
		},
	}); err != nil {
		t.Errorf("There is error when setting up client with Role info")
	}
	_, _, arbErr := client.StreamChannelGetArbitrationResp(&streamParameter.Name, 1)

	if arbErr == nil {
		t.Errorf("Expected error is not seen.")
	}
}

// client arbitration-Compliance:019
func testDiffernetRoleConfigOnExistingConnectedController(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA

	streamParameter := generateStreamParameter(deviceID, uint64(0), electionID)

	// Setup connection for default deviceID
	// Setup P4RT Client
	if err := client.StreamChannelSendMsg(&streamName, &p4_v1.StreamMessageRequest{
		Update: &p4_v1.StreamMessageRequest_Arbitration{
			Arbitration: &p4_v1.MasterArbitrationUpdate{
				DeviceId: streamParameter.DeviceId,
				ElectionId: &p4_v1.Uint128{
					High: streamParameter.ElectionIdH,
					Low:  streamParameter.ElectionIdL,
				},
				Role: &p4_v1.Role{
					Id: uint64(deviceID),
					Config: &any.Any{
						TypeUrl: "test",
					},
				},
			},
		},
	}); err != nil {
		t.Errorf("There is error when setting up client with Role info")
	}
	_, _, arbErr := client.StreamChannelGetArbitrationResp(&streamParameter.Name, 1)

	if arbErr != nil {
		t.Errorf("There is error seen when setting up connection, %v", arbErr)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, client)

	// Setup P4RT Client
	if err := client.StreamChannelSendMsg(&streamName, &p4_v1.StreamMessageRequest{
		Update: &p4_v1.StreamMessageRequest_Arbitration{
			Arbitration: &p4_v1.MasterArbitrationUpdate{
				DeviceId: streamParameter.DeviceId,
				ElectionId: &p4_v1.Uint128{
					High: streamParameter.ElectionIdH,
					Low:  streamParameter.ElectionIdL + 1,
				},
				Role: &p4_v1.Role{
					Id: uint64(deviceID),
					Config: &any.Any{
						TypeUrl: "test1",
					},
				},
			},
		},
	}); err != nil {
		t.Errorf("There is error when setting up client with Role info")
	}
	_, _, arbErr = client.StreamChannelGetArbitrationResp(&streamParameter.Name, 1)

	if arbErr == nil {
		t.Errorf("Expected error is not seen.")
	}
}

// client arbitration-Compliance:020
func testSameElectionIdFromDifferentController(ctx context.Context, t *testing.T, args *testArgs) {
	clientA := args.p4rtClientA
	clientB := args.p4rtClientB

	// Setup P4RT Client
	streamParameter := generateStreamParameter(deviceID, uint64(0), electionID)

	// Setup connection for default deviceID
	if err := setupConnection(ctx, t, streamParameter, clientA); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, clientA)

	// Setup connnection for same deviceID with same electionId
	if err := setupConnection(ctx, t, streamParameter, clientB); err == nil {
		t.Errorf("Expected error is nost seen.")
	}

	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, clientB)
}

// client arbitration-Compliance:021
func testDiffernetRoleAndSameElectionIdFromConnectedController(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA

	streamParameter := generateStreamParameter(deviceID, uint64(0), electionID)

	// Setup connection for default deviceID
	if err := setupConnection(ctx, t, streamParameter, client); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, client)

	// Setup P4RT Client
	if err := client.StreamChannelSendMsg(&streamName, &p4_v1.StreamMessageRequest{
		Update: &p4_v1.StreamMessageRequest_Arbitration{
			Arbitration: &p4_v1.MasterArbitrationUpdate{
				DeviceId: streamParameter.DeviceId,
				ElectionId: &p4_v1.Uint128{
					High: streamParameter.ElectionIdH,
					Low:  streamParameter.ElectionIdL,
				},
				Role: &p4_v1.Role{
					Id: uint64(deviceID),
					Config: &any.Any{
						TypeUrl: "test",
					},
				},
			},
		},
	}); err != nil {
		t.Errorf("There is error when setting up client with Role info")
	}
	_, _, arbErr := client.StreamChannelGetArbitrationResp(&streamParameter.Name, 1)

	if arbErr != nil {
		t.Errorf("There is error seen in the Client Arbitration ")
	}
}

// client arbitration-Compliance:022
func testSameElectionIdAsBackupController(ctx context.Context, t *testing.T, args *testArgs) {
	clientA := args.p4rtClientA
	clientB := args.p4rtClientB

	streamParameter := generateStreamParameter(deviceID, uint64(0), electionID)

	// Setup connection for default deviceID
	if err := setupConnection(ctx, t, streamParameter, clientA); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, clientA)

	// Setup connection for default deviceID
	streamParameter.ElectionIdL -= 1
	if err := setupConnection(ctx, t, streamParameter, clientB); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, clientB)

	if err := setupConnection(ctx, t, streamParameter, clientA); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}
}

// client arbitration-Compliance:023
func testDifferentElectionIdFromController(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA

	streamParameter := generateStreamParameter(deviceID, uint64(0), electionID)

	// Setup connection for default deviceID
	if err := setupConnection(ctx, t, streamParameter, client); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, client)

	// Setup connection for default deviceID
	streamParameter.ElectionIdL += 1
	if err := setupConnection(ctx, t, streamParameter, client); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}
}

// client arbitration-Compliance:024
func testNewPrimary(ctx context.Context, t *testing.T, args *testArgs) {
	clientA := args.p4rtClientA
	clientB := args.p4rtClientB

	streamParameter := generateStreamParameter(deviceID, uint64(0), electionID)

	// Setup connection for default deviceID
	if err := setupConnection(ctx, t, streamParameter, clientA); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, clientA)

	// Setup connection for default deviceID
	streamParameter.ElectionIdL += 1
	if err := setupConnection(ctx, t, streamParameter, clientB); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}

	// Verify clientA gets advisory message
	_, resp, arbErr := clientA.StreamChannelGetArbitrationResp(&streamParameter.Name, 1)

	if arbErr != nil {
		t.Errorf("There is error at Arbitration time: %v", arbErr)
	}

	// TODO: check Advisory message properly
	if len(resp.Arb.String()) == 0 {
		t.Errorf("Expected Advisory message is not seen.")
	}
}

// client arbitration-Compliance:025
func testNewPrimaryWithWriteFromPreviousPrimary(ctx context.Context, t *testing.T, args *testArgs) {
	clientA := args.p4rtClientA
	clientB := args.p4rtClientB

	streamParameter := generateStreamParameter(deviceID, uint64(0), electionID)

	// Setup connection for default deviceID
	if err := setupConnection(ctx, t, streamParameter, clientA); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, clientA)

	if err := setupForwardingPipeline(ctx, t, streamParameter, clientA); err != nil {
		t.Errorf("There is error sending SetForwardingPipeline, %s", err)
	}

	// Setup connection for default deviceID
	streamParameter.ElectionIdL += 1
	if err := setupConnection(ctx, t, streamParameter, clientB); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}

	// Verify Write from clientA fails
	if err := programmGDPMatchEntry(ctx, t, clientA, false); err == nil {
		t.Errorf("Expected error is not seen.")
	}
}

// client arbitration-Compliance:026
func testNewPrimaryNotifyOtherController(ctx context.Context, t *testing.T, args *testArgs) {
	clientA := args.p4rtClientA
	clientB := args.p4rtClientB
	clientC := args.p4rtClientC

	streamParameter := generateStreamParameter(deviceID, uint64(0), electionID)

	// Setup connection for default deviceID
	if err := setupConnection(ctx, t, streamParameter, clientA); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, clientA)

	// Setup connection for default deviceID
	streamParameter.ElectionIdL -= 1
	if err := setupConnection(ctx, t, streamParameter, clientC); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, clientC)

	// Setup connection for default deviceID
	streamParameter.ElectionIdL += 2
	if err := setupConnection(ctx, t, streamParameter, clientB); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}
	// Verify clientC gets advisory message
	_, resp, arbErr := clientC.StreamChannelGetArbitrationResp(&streamParameter.Name, 1)

	if arbErr != nil {
		t.Errorf("There is error at Arbitration time: %v", arbErr)
	}

	// TODO: check Advisory message properly
	if len(resp.Arb.String()) == 0 {
		t.Errorf("Expected Advisory message is not seen.")
	}
}

// client arbitration-Compliance:027
func testNewPrimaryWithOngoingWrite(ctx context.Context, t *testing.T, args *testArgs) {
	t.Logf("TODO: Need to find a way to simulate this scale on-going write")
	t.Skip()
	clientA := args.p4rtClientA
	clientB := args.p4rtClientB

	streamParameter := generateStreamParameter(deviceID, uint64(0), electionID)

	// Setup connection for default deviceID
	if err := setupConnection(ctx, t, streamParameter, clientA); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, clientA)

	// Setup connection for default deviceID
	streamParameter.ElectionIdL += 2
	if err := setupConnection(ctx, t, streamParameter, clientB); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}
}

// client arbitration-Compliance:028
func testNewPrimaryWithWriteFromNewPrimary(ctx context.Context, t *testing.T, args *testArgs) {
	clientA := args.p4rtClientA
	clientB := args.p4rtClientB

	streamParameter := generateStreamParameter(deviceID, uint64(0), electionID)

	// Setup connection for default deviceID
	if err := setupConnection(ctx, t, streamParameter, clientA); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, clientA)

	if err := setupForwardingPipeline(ctx, t, streamParameter, clientA); err != nil {
		t.Errorf("There is error sending SetForwardingPipeline, %s", err)
	}

	// Setup connection for default deviceID
	streamParameter.ElectionIdL += 1
	if err := setupConnection(ctx, t, streamParameter, clientB); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}

	// Verify Write from clientA fails
	if err := programmGDPMatchEntry(ctx, t, clientB, false); err != nil {
		t.Errorf("There is error when programming entry. %v", err)
	}
	defer programmGDPMatchEntry(ctx, t, clientB, true)
}

// client arbitration-Compliance:029
func testSeverNotifyNewPrimary(ctx context.Context, t *testing.T, args *testArgs) {
	clientA := args.p4rtClientA
	clientB := args.p4rtClientB

	streamParameter := generateStreamParameter(deviceID, uint64(0), electionID)

	// Setup connection for default deviceID
	if err := setupConnection(ctx, t, streamParameter, clientA); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, clientA)

	// Setup connection for default deviceID
	streamParameter.ElectionIdL += 1
	if err := setupConnection(ctx, t, streamParameter, clientB); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}
	// Verify clientC gets advisory message
	_, resp, arbErr := clientB.StreamChannelGetArbitrationResp(&streamParameter.Name, 1)

	if arbErr != nil {
		t.Errorf("There is error at Arbitration time: %v", arbErr)
	}

	// TODO: check Advisory message properly
	if resp == nil || len(resp.Arb.String()) == 0 {
		t.Errorf("Expected Advisory message is not seen.")
	}
}

// client arbitration-Compliance:030
func testPrimaryDowngrade(ctx context.Context, t *testing.T, args *testArgs) {
	clientA := args.p4rtClientA
	clientB := args.p4rtClientB
	clientC := args.p4rtClientC

	streamParameter := generateStreamParameter(deviceID, uint64(0), electionID)

	// Setup connection for default deviceID
	if err := setupConnection(ctx, t, streamParameter, clientA); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, clientA)

	// Setup connection for default deviceID
	streamParameter.ElectionIdL -= 1
	if err := setupConnection(ctx, t, streamParameter, clientB); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}

	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, clientB)

	// Setup connection for default deviceID
	streamParameter.ElectionIdL -= 1
	if err := setupConnection(ctx, t, streamParameter, clientC); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}

	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, clientC)

	// Downgrade primary
	streamParameter.ElectionIdL -= 1
	if err := setupConnection(ctx, t, streamParameter, clientB); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}

	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, clientB)

	for _, client := range []*p4rt_client.P4RTClient{clientA, clientB, clientC} {
		// Verify clientC gets advisory message
		_, resp, arbErr := client.StreamChannelGetArbitrationResp(&streamParameter.Name, 1)

		if arbErr != nil {
			t.Errorf("There is error at Arbitration time: %v", arbErr)
		}

		// TODO: check Advisory message properly
		if resp == nil || len(resp.Arb.String()) == 0 {
			t.Errorf("Expected Advisory message is not seen.")
		}
	}
}

// client arbitration-Compliance:031
func testNewBackup(ctx context.Context, t *testing.T, args *testArgs) {
	clientA := args.p4rtClientA
	clientB := args.p4rtClientB

	streamParameter := generateStreamParameter(deviceID, uint64(0), electionID)

	// Setup connection for default deviceID
	if err := setupConnection(ctx, t, streamParameter, clientA); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, clientA)

	// Setup connection for default deviceID
	streamParameter.ElectionIdL -= 1
	if err := setupConnection(ctx, t, streamParameter, clientB); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}

	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, clientB)

	// Downgrade primary
	streamParameter.ElectionIdL -= 1
	if err := setupConnection(ctx, t, streamParameter, clientB); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}

	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, clientB)

	// Verify clientC gets advisory message
	_, resp, arbErr := clientB.StreamChannelGetArbitrationResp(&streamParameter.Name, 1)

	if arbErr != nil {
		t.Errorf("There is error at Arbitration time: %v", arbErr)
	}

	// TODO: check Advisory message properly
	if resp == nil || len(resp.Arb.String()) == 0 {
		t.Errorf("Expected Advisory message is not seen.")
	}
}

// client arbitration-Compliance:032
func testNoPrimary(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA

	streamParameter := generateStreamParameter(deviceID, uint64(0), electionID)

	// Setup P4RT Client
	client.StreamChannelCreate(&streamParameter)
	if err := client.StreamChannelSendMsg(&streamName, &p4_v1.StreamMessageRequest{
		Update: &p4_v1.StreamMessageRequest_Arbitration{
			Arbitration: &p4_v1.MasterArbitrationUpdate{
				DeviceId: streamParameter.DeviceId,
			},
		},
	}); err != nil {
		t.Errorf("There is error when setting up client with Role info")
	}

	_, resp, arbErr := client.StreamChannelGetArbitrationResp(&streamParameter.Name, 1)

	if arbErr != nil {
		t.Errorf("There is error seen when setting up connection, %v", arbErr)
	}

	// TODO: check Advisory message properly
	if resp == nil || len(resp.Arb.String()) == 0 {
		t.Errorf("Expected Advisory message is not seen.")
	}
}

// client arbitration-Compliance:033
func testElectionIDinStreamMessageResponse(ctx context.Context, t *testing.T, args *testArgs) {
	clientA := args.p4rtClientA
	clientB := args.p4rtClientB

	streamParameter := generateStreamParameter(deviceID, uint64(0), electionID)

	// Setup connection for default deviceID
	if err := setupConnection(ctx, t, streamParameter, clientA); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, clientA)

	// Setup connection for default deviceID
	streamParameter.ElectionIdL -= 1
	if err := setupConnection(ctx, t, streamParameter, clientB); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}

	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, clientB)

	// Verify clientC gets advisory message
	_, resp, arbErr := clientB.StreamChannelGetArbitrationResp(&streamParameter.Name, 1)

	if arbErr != nil {
		t.Errorf("There is error at Arbitration time: %v", arbErr)
	}

	// TODO: check Advisory message properly
	if resp == nil || len(resp.Arb.String()) == 0 {
		t.Errorf("Expected Advisory message is not seen.")
	}
}

// client arbitration-Compliance:034
func testStatusinStreamMessageResponse(ctx context.Context, t *testing.T, args *testArgs) {
	clientA := args.p4rtClientA
	clientB := args.p4rtClientB
	clientC := args.p4rtClientC

	streamParameter := generateStreamParameter(deviceID, uint64(0), electionID)

	// Setup connection for default deviceID
	if err := setupConnection(ctx, t, streamParameter, clientA); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, clientA)

	// Setup connection for default deviceID
	streamParameter.ElectionIdL -= 1
	if err := setupConnection(ctx, t, streamParameter, clientB); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}

	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, clientB)

	// Setup connection for default deviceID
	streamParameter.ElectionIdL -= 1
	if err := setupConnection(ctx, t, streamParameter, clientC); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}

	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, clientC)

	// Verify clientA gets advisory message
	_, respA, arbErrA := clientA.StreamChannelGetArbitrationResp(&streamParameter.Name, 1)

	if arbErrA != nil {
		t.Errorf("There is error at Arbitration time: %v", arbErrA)
	}

	// TODO: check Advisory message properly
	if respA == nil || len(respA.Arb.String()) == 0 {
		t.Errorf("Expected Advisory message is not seen.")
	}

	// Verify clientB gets advisory message
	_, respB, arbErrB := clientB.StreamChannelGetArbitrationResp(&streamParameter.Name, 1)

	if arbErrB != nil {
		t.Errorf("There is error at Arbitration time: %v", arbErrB)
	}

	// TODO: check Advisory message properly
	if respB == nil || len(respB.Arb.String()) == 0 {
		t.Errorf("Expected Advisory message is not seen.")
	}
}

// client arbitration-Compliance:035
func testStatusinStreamMessageResponseWithoutPrimary(ctx context.Context, t *testing.T, args *testArgs) {
	clientA := args.p4rtClientA
	clientB := args.p4rtClientB

	streamParameter := generateStreamParameter(deviceID, uint64(0), electionID)

	// Setup P4RT Client
	clientA.StreamChannelCreate(&streamParameter)
	if err := clientA.StreamChannelSendMsg(&streamName, &p4_v1.StreamMessageRequest{
		Update: &p4_v1.StreamMessageRequest_Arbitration{
			Arbitration: &p4_v1.MasterArbitrationUpdate{
				DeviceId: streamParameter.DeviceId,
			},
		},
	}); err != nil {
		t.Errorf("There is error when setting up client with Role info")
	}

	// Setup connection for default deviceID
	if err := setupConnection(ctx, t, streamParameter, clientB); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}

	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, clientB)

	// Verify clientC gets advisory message
	_, resp, arbErr := clientA.StreamChannelGetArbitrationResp(&streamParameter.Name, 1)

	if arbErr != nil {
		t.Errorf("There is error at Arbitration time: %v", arbErr)
	}

	// TODO: check Advisory message properly
	if resp == nil || len(resp.Arb.String()) == 0 {
		t.Errorf("Expected Advisory message is not seen.")
	}
}

// client arbitration-Compliance:036
func testBackupOnly(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA

	streamParameter := generateStreamParameter(deviceID, uint64(0), electionID)

	// Setup P4RT Client
	client.StreamChannelCreate(&streamParameter)
	if err := client.StreamChannelSendMsg(&streamName, &p4_v1.StreamMessageRequest{
		Update: &p4_v1.StreamMessageRequest_Arbitration{
			Arbitration: &p4_v1.MasterArbitrationUpdate{
				DeviceId: streamParameter.DeviceId,
				// ElectionId: &p4_v1.Uint128{
				// 	High: 0,
				// 	Low:  0,
				// },
			},
		},
	}); err != nil {
		t.Errorf("There is error when setting up client with Role info")
	}

	_, _, arbErr := client.StreamChannelGetArbitrationResp(&streamParameter.Name, 1)

	if arbErr != nil {
		t.Errorf("There is error seen when setting up connection, %v", arbErr)
	}
}
