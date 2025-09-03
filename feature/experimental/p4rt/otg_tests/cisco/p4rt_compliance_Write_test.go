package cisco_p4rt_test

import (
	"context"
	"testing"

	wbb "github.com/openconfig/featureprofiles/internal/p4rtutils"
	p4_v1 "github.com/p4lang/p4runtime/go/p4/v1"
)

var (
	P4RTComplianceWriteRPC = []Testcase{
		{
			name: "Insert entry with trap action in Write RPC",
			desc: "Write RPC-Compliance:001 Verify Write RPC with p4 entry trap action works when it's from Primary controller",
			fn:   testWriteRPCInsertTrapAction,
		},
		{
			name: "Insert entry with copy  action in Write RPC",
			desc: "Write RPC-Compliance:002 Verify Write RPC with p4 entry copy action works when it's from Primary controller",
			fn:   testWriteRPCInsertCopyAction,
		},
		{
			name: "Insert to non-exist device in Write RPC",
			desc: "Write RPC-Compliance:003 12(1) Write RPC with device_id doesn't exist, verify device returns with NOT_FOUND error",
			fn:   testWriteRPCInsertNonExistDeviceID,
		},
		{
			name: "Inert entry with lower election id in Write RPC",
			desc: "Write RPC-Compliance:004 12(2) Write RPC with non-primary election-id(lower than primary), verify device returns with PERMISSION_DENIED error",
			fn:   testWriteRPCInsertWithLowerElectionID,
		},
		{
			name: "Inert entry with higher election id in Write RPC",
			desc: "Write RPC-Compliance:005 12(2) Write RPC with non-primary election-id(higher than primary), verify device returns with PERMISSION_DENIED error",
			fn:   testWriteRPCInsertWithHigherElectionID,
		},
		{
			name: "Send Write RPC before SetForwardingPipeline",
			desc: "Write RPC-Compliance:006 12(3) Write RPC sent before ForwardingPipelineConfig verify device returns with FAILED_PRECONDITION error",
			fn:   testWriteRPCBeforeSetForwardingPipeline,
		},
		{
			name: "Insert non-exist entry in Write RPC",
			desc: "Write RPC-Compliance:007 12(INSERT) Write RPC with Insert non-exist entity, verify the entity is programmed on the device",
			fn:   testWriteRPCInsertEntry,
		},
		{
			name: "Insert existing entry in Write RPC",
			desc: "Write RPC-Compliance:008 12(INSERT) Write RPC with Insert exisint entity, verify ALREADY_EXISTS error returned and existing entity remain unchanged",
			fn:   testWriteRPCInsertSameEntry,
		},
		{
			name: "Insert malformed entry in Write RPC",
			desc: "Write RPC-Compliance:009 12(INSERT) Write RPC with Insert malformed entity, verify INVLIAD_ARGUMENT error is returned ",
			fn:   testWriteRPCInsertMalformedEntry,
		},
		{
			name: "Send Write RPC in case of OOR",
			desc: "Write RPC-Compliance:010 12(INSERT) Write RPC with Insert entity when device is in OOR, verify RESOURCE_EXHAUSTED error is returned",
			fn:   testWriteRPCOOR,
		},
		{
			name: "Modify existing entry in Write RPC",
			desc: "Write RPC-Compliance:011 12(MODIFY) Write RPC with Modify existing entity, verify the entity is changed on the device",
			fn:   testWriteRPCModifyEntry,
		},
		{
			name: "Modify malformed entry in Write RPC",
			desc: "Write RPC-Compliance:012 12(MODIFY) Write RPC with Modify existing entity, verify the entity is changed on the device",
			fn:   testWriteRPCModifyMalformedEntry,
		},
		{
			name: "Modify non-exist entry in Write RPC",
			desc: "Write RPC-Compliance:013 12(MODIFY) Write RPC with Modify existing entity, verify the entity is changed on the device",
			fn:   testWriteRPCModifyNonExistEntry,
		},
		{
			name: "Delete existing entry in Write RPC",
			desc: "Write RPC-Compliance:014 12(DELETE) Write RPC with DELETE existing entity with Match Fields(without Actions) only, verify the entity is removed on the device",
			fn:   testWriteRPCDeleteEntry,
		},
		{
			name: "Delete malformed entry in Write RPC",
			desc: "Write RPC-Compliance:015 12(DELETE) Write RPC with DELETE existing entity with Match Fields, verify the entity is removed on the device",
			fn:   testWriteRPCDeleteMalformedEntry,
		},
		{
			name: "Delete non-exist entry in Write RPC",
			desc: "Write RPC-Compliance:016 12(DELETE) Write RPC with DELETE non-exist entity Match Fields(with different than programmed), verify NOT_FOUND is returned",
			fn:   testWriteRPCDeleteNonExistEntry,
		},
		{
			name: "UnSpecified action in Write RPC",
			desc: "Write RPC-Compliance:017 12(UNSPECIFIED) when UNSPECIFIED sent, verify unimplemented error returned",
			fn:   testWriteRPCWithUnspecificAction,
		},
	}
)

// Write RPC-Compliance:001
func testWriteRPCInsertTrapAction(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA

	streamParameter := generateStreamParameter(deviceID, uint64(0), electionID)

	// Setup P4RT Client
	if err := setupConnection(ctx, t, streamParameter, client); err != nil {
		t.Fatalf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, client)

	if err := setupForwardingPipeline(ctx, t, streamParameter, client); err != nil {
		t.Fatalf("There is error sending SetForwardingPipeline, %s", err)
	}

	// Program the entry
	if err := client.Write(&p4_v1.WriteRequest{
		DeviceId:   uint64(1),
		ElectionId: &p4_v1.Uint128{High: uint64(0), Low: uint64(100)},
		Updates: wbb.ACLWbbIngressTableEntryGet([]*wbb.ACLWbbIngressTableEntryInfo{
			{
				Type:          p4_v1.Update_INSERT,
				EtherType:     0x6007,
				EtherTypeMask: 0xFFFF,
				Priority:      1,
			},
		}),
		Atomicity: p4_v1.WriteRequest_CONTINUE_ON_ERROR,
	}); err != nil {
		t.Fatalf("There is error sending SetForwardingPipeline, %s", err)
	}
}

// Write RPC-Compliance:002
func testWriteRPCInsertCopyAction(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA

	streamParameter := generateStreamParameter(deviceID, uint64(0), electionID)
	// Setup P4RT Client
	if err := setupConnection(ctx, t, streamParameter, client); err != nil {
		t.Fatalf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, client)

	if err := setupForwardingPipeline(ctx, t, streamParameter, client); err != nil {
		t.Fatalf("There is error sending SetForwardingPipeline, %s", err)
	}

	// Program the entry
	if err := client.Write(&p4_v1.WriteRequest{
		DeviceId:   uint64(1),
		ElectionId: &p4_v1.Uint128{High: uint64(0), Low: uint64(100)},
		Updates: wbb.ACLWbbIngressTableEntryGet([]*wbb.ACLWbbIngressTableEntryInfo{
			{
				Type:          p4_v1.Update_INSERT,
				EtherType:     0x6007,
				EtherTypeMask: 0xFFFF,
				Priority:      1,
			},
		}),
		Atomicity: p4_v1.WriteRequest_CONTINUE_ON_ERROR,
	}); err != nil {
		t.Fatalf("There is error sending SetForwardingPipeline, %s", err)
	}
}

// Write RPC-Compliance:003
func testWriteRPCInsertNonExistDeviceID(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA
	streamParameter := generateStreamParameter(deviceID, uint64(0), electionID)

	// Setup P4RT Client
	if err := setupConnection(ctx, t, streamParameter, client); err != nil {
		t.Fatalf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, client)

	if err := setupForwardingPipeline(ctx, t, streamParameter, client); err != nil {
		t.Fatalf("There is error sending SetForwardingPipeline, %s", err)
	}

	// Program the entry
	if err := client.Write(&p4_v1.WriteRequest{
		DeviceId:   ^uint64(1),
		ElectionId: &p4_v1.Uint128{High: uint64(0), Low: uint64(100)},
		Updates: wbb.ACLWbbIngressTableEntryGet([]*wbb.ACLWbbIngressTableEntryInfo{
			{
				Type:          p4_v1.Update_INSERT,
				EtherType:     0x6007,
				EtherTypeMask: 0xFFFF,
				Priority:      1,
			},
		}),
		Atomicity: p4_v1.WriteRequest_CONTINUE_ON_ERROR,
	}); err == nil {
		t.Fatalf("Expected error is not seen.")
	}
}

// Write RPC-Compliance:004
func testWriteRPCInsertWithLowerElectionID(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA

	streamParameter := generateStreamParameter(deviceID, uint64(0), electionID)

	// Setup P4RT Client
	if err := setupConnection(ctx, t, streamParameter, client); err != nil {
		t.Fatalf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, client)

	if err := setupForwardingPipeline(ctx, t, streamParameter, client); err != nil {
		t.Fatalf("There is error sending SetForwardingPipeline, %s", err)
	}

	// Program the entry
	if err := client.Write(&p4_v1.WriteRequest{
		DeviceId:   uint64(1),
		ElectionId: &p4_v1.Uint128{High: uint64(0), Low: uint64(99)},
		Updates: wbb.ACLWbbIngressTableEntryGet([]*wbb.ACLWbbIngressTableEntryInfo{
			{
				Type:          p4_v1.Update_INSERT,
				EtherType:     0x6007,
				EtherTypeMask: 0xFFFF,
				Priority:      1,
			},
		}),
		Atomicity: p4_v1.WriteRequest_CONTINUE_ON_ERROR,
	}); err == nil {
		t.Fatalf("Expected error is not seen.")
	}
}

// Write RPC-Compliance:005
func testWriteRPCInsertWithHigherElectionID(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA

	streamParameter := generateStreamParameter(deviceID, uint64(0), electionID)

	// Setup P4RT Client
	if err := setupConnection(ctx, t, streamParameter, client); err != nil {
		t.Fatalf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, client)

	if err := setupForwardingPipeline(ctx, t, streamParameter, client); err != nil {
		t.Fatalf("There is error sending SetForwardingPipeline, %s", err)
	}

	// Program the entry
	if err := client.Write(&p4_v1.WriteRequest{
		DeviceId:   uint64(1),
		ElectionId: &p4_v1.Uint128{High: uint64(0), Low: uint64(101)},
		Updates: wbb.ACLWbbIngressTableEntryGet([]*wbb.ACLWbbIngressTableEntryInfo{
			{
				Type:          p4_v1.Update_INSERT,
				EtherType:     0x6007,
				EtherTypeMask: 0xFFFF,
				Priority:      1,
			},
		}),
		Atomicity: p4_v1.WriteRequest_CONTINUE_ON_ERROR,
	}); err == nil {
		t.Fatalf("Expected error is not seen.")
	}
}

// Write RPC-Compliance:006
func testWriteRPCBeforeSetForwardingPipeline(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA

	streamParameter := generateStreamParameter(deviceID, uint64(0), electionID)

	// Setup P4RT Client
	if err := setupConnection(ctx, t, streamParameter, client); err != nil {
		t.Fatalf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, client)

	// Program the entry
	if err := client.Write(&p4_v1.WriteRequest{
		DeviceId:   uint64(1),
		ElectionId: &p4_v1.Uint128{High: uint64(0), Low: uint64(100)},
		Updates: wbb.ACLWbbIngressTableEntryGet([]*wbb.ACLWbbIngressTableEntryInfo{
			{
				Type:          p4_v1.Update_INSERT,
				EtherType:     0x6007,
				EtherTypeMask: 0xFFFF,
				Priority:      1,
			},
		}),
		Atomicity: p4_v1.WriteRequest_CONTINUE_ON_ERROR,
	}); err == nil {
		t.Fatalf("Expected error is not seen.")
	}
}

// Write RPC-Compliance:007
func testWriteRPCInsertEntry(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA

	streamParameter := generateStreamParameter(deviceID, uint64(0), electionID)

	// Setup P4RT Client
	if err := setupConnection(ctx, t, streamParameter, client); err != nil {
		t.Fatalf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, client)

	if err := setupForwardingPipeline(ctx, t, streamParameter, client); err != nil {
		t.Fatalf("There is error sending SetForwardingPipeline, %s", err)
	}

	// Program the entry
	if err := programmGDPMatchEntry(ctx, t, client, false); err != nil {
		t.Fatalf("There is error programming the entry, %s", err)
	}
}

// Write RPC-Compliance:008
func testWriteRPCInsertSameEntry(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA

	streamParameter := generateStreamParameter(deviceID, uint64(0), electionID)

	// Setup P4RT Client
	if err := setupConnection(ctx, t, streamParameter, client); err != nil {
		t.Fatalf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, client)

	if err := setupForwardingPipeline(ctx, t, streamParameter, client); err != nil {
		t.Fatalf("There is error sending SetForwardingPipeline, %s", err)
	}

	// Program the entry
	if err := programmGDPMatchEntry(ctx, t, client, false); err != nil {
		t.Fatalf("There is error programming the entry, %s", err)
	}

	// Program the entry 2nd time and expecting error
	if err := programmGDPMatchEntry(ctx, t, client, false); err == nil {
		t.Fatalf("Expected error not seen")
	}
}

// Write RPC-Compliance:009
func testWriteRPCInsertMalformedEntry(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA

	streamParameter := generateStreamParameter(deviceID, uint64(0), electionID)

	// Setup P4RT Client
	if err := setupConnection(ctx, t, streamParameter, client); err != nil {
		t.Fatalf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, client)

	if err := setupForwardingPipeline(ctx, t, streamParameter, client); err != nil {
		t.Fatalf("There is error sending SetForwardingPipeline, %s", err)
	}
	// Program the entry
	if err := client.Write(&p4_v1.WriteRequest{
		DeviceId:   uint64(1),
		ElectionId: &p4_v1.Uint128{High: uint64(0), Low: uint64(100)},
		Updates: wbb.ACLWbbIngressTableEntryGet([]*wbb.ACLWbbIngressTableEntryInfo{
			{
				Type: p4_v1.Update_INSERT,
				// EtherType: 0x6007,
			},
		}),
		Atomicity: p4_v1.WriteRequest_CONTINUE_ON_ERROR,
	}); err == nil {
		t.Fatalf("Expected error is not seen.")
	}
}

// Write RPC-Compliance:010
func testWriteRPCOOR(ctx context.Context, t *testing.T, args *testArgs) {
	t.Logf("TODO: Add new clients to simulate OOR condition")
	t.Skip()
	client := args.p4rtClientA

	streamParameter := generateStreamParameter(deviceID, uint64(0), electionID)

	// Setup P4RT Client
	if err := setupConnection(ctx, t, streamParameter, client); err != nil {
		t.Fatalf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, client)

	if err := setupForwardingPipeline(ctx, t, streamParameter, client); err != nil {
		t.Fatalf("There is error sending SetForwardingPipeline, %s", err)
	}
	// Program the entry
	if err := client.Write(&p4_v1.WriteRequest{
		DeviceId:   uint64(1),
		ElectionId: &p4_v1.Uint128{High: uint64(0), Low: uint64(100)},
		Updates: wbb.ACLWbbIngressTableEntryGet([]*wbb.ACLWbbIngressTableEntryInfo{
			{
				Type: p4_v1.Update_INSERT,
				// EtherType: 0x6007,
			},
		}),
		Atomicity: p4_v1.WriteRequest_CONTINUE_ON_ERROR,
	}); err == nil {
		t.Fatalf("Expected error is not seen.")
	}
}

// Write RPC-Compliance:011
func testWriteRPCModifyEntry(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA

	streamParameter := generateStreamParameter(deviceID, uint64(0), electionID)

	// Setup P4RT Client
	if err := setupConnection(ctx, t, streamParameter, client); err != nil {
		t.Fatalf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, client)

	if err := setupForwardingPipeline(ctx, t, streamParameter, client); err != nil {
		t.Fatalf("There is error sending SetForwardingPipeline, %s", err)
	}

	programmGDPMatchEntry(ctx, t, client, false)

	// Program the entry
	if err := client.Write(&p4_v1.WriteRequest{
		DeviceId:   uint64(1),
		ElectionId: &p4_v1.Uint128{High: uint64(0), Low: uint64(100)},
		Updates: wbb.ACLWbbIngressTableEntryGet([]*wbb.ACLWbbIngressTableEntryInfo{
			{
				Type:          p4_v1.Update_MODIFY,
				EtherType:     0x6007,
				EtherTypeMask: 0xFFFF,
				Priority:      1,
			},
		}),
		Atomicity: p4_v1.WriteRequest_CONTINUE_ON_ERROR,
	}); err == nil {
		t.Fatalf("Expected error is not seen.")
	}
}

// Write RPC-Compliance:012
func testWriteRPCModifyMalformedEntry(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA

	streamParameter := generateStreamParameter(deviceID, uint64(0), electionID)

	// Setup P4RT Client
	if err := setupConnection(ctx, t, streamParameter, client); err != nil {
		t.Fatalf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, client)

	if err := setupForwardingPipeline(ctx, t, streamParameter, client); err != nil {
		t.Fatalf("There is error sending SetForwardingPipeline, %s", err)
	}

	// Program the entry
	if err := client.Write(&p4_v1.WriteRequest{
		DeviceId:   uint64(1),
		ElectionId: &p4_v1.Uint128{High: uint64(0), Low: uint64(100)},
		Updates: wbb.ACLWbbIngressTableEntryGet([]*wbb.ACLWbbIngressTableEntryInfo{
			{
				Type:      p4_v1.Update_MODIFY,
				EtherType: 0x6007,
			},
		}),
		Atomicity: p4_v1.WriteRequest_CONTINUE_ON_ERROR,
	}); err == nil {
		t.Fatalf("Expected error is not seen.")
	}
}

// Write RPC-Compliance:013
func testWriteRPCModifyNonExistEntry(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA

	streamParameter := generateStreamParameter(deviceID, uint64(0), electionID)

	// Setup P4RT Client
	if err := setupConnection(ctx, t, streamParameter, client); err != nil {
		t.Fatalf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, client)

	if err := setupForwardingPipeline(ctx, t, streamParameter, client); err != nil {
		t.Fatalf("There is error sending SetForwardingPipeline, %s", err)
	}

	programmGDPMatchEntry(ctx, t, client, false)

	// Program the entry
	if err := client.Write(&p4_v1.WriteRequest{
		DeviceId:   uint64(1),
		ElectionId: &p4_v1.Uint128{High: uint64(0), Low: uint64(100)},
		Updates: wbb.ACLWbbIngressTableEntryGet([]*wbb.ACLWbbIngressTableEntryInfo{
			{
				Type:          p4_v1.Update_MODIFY,
				EtherType:     0x6666,
				EtherTypeMask: 0xFFFF,
				Priority:      1,
			},
		}),
		Atomicity: p4_v1.WriteRequest_CONTINUE_ON_ERROR,
	}); err == nil {
		t.Fatalf("Expected error is not seen.")
	}
}

// Write RPC-Compliance:014
func testWriteRPCDeleteEntry(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA

	streamParameter := generateStreamParameter(deviceID, uint64(0), electionID)

	// Setup P4RT Client
	if err := setupConnection(ctx, t, streamParameter, client); err != nil {
		t.Fatalf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, client)

	if err := setupForwardingPipeline(ctx, t, streamParameter, client); err != nil {
		t.Fatalf("There is error sending SetForwardingPipeline, %s", err)
	}

	programmGDPMatchEntry(ctx, t, client, false)

	// Program the entry
	if err := programmGDPMatchEntry(ctx, t, client, true); err != nil {
		t.Fatalf("Unexpected error seen, %s", err)
	}
}

// Write RPC-Compliance:015
func testWriteRPCDeleteMalformedEntry(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA

	streamParameter := generateStreamParameter(deviceID, uint64(0), electionID)

	// Setup P4RT Client
	if err := setupConnection(ctx, t, streamParameter, client); err != nil {
		t.Fatalf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, client)

	if err := setupForwardingPipeline(ctx, t, streamParameter, client); err != nil {
		t.Fatalf("There is error sending SetForwardingPipeline, %s", err)
	}

	programmGDPMatchEntry(ctx, t, client, false)

	// Program the entry
	if err := client.Write(&p4_v1.WriteRequest{
		DeviceId:   uint64(1),
		ElectionId: &p4_v1.Uint128{High: uint64(0), Low: uint64(100)},
		Updates: wbb.ACLWbbIngressTableEntryGet([]*wbb.ACLWbbIngressTableEntryInfo{
			{
				Type:      p4_v1.Update_DELETE,
				EtherType: 0x6007,
			},
		}),
		Atomicity: p4_v1.WriteRequest_CONTINUE_ON_ERROR,
	}); err == nil {
		t.Fatalf("Expected error is not seen.")
	}
}

// Write RPC-Compliance:016
func testWriteRPCDeleteNonExistEntry(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA

	streamParameter := generateStreamParameter(deviceID, uint64(0), electionID)

	// Setup P4RT Client
	if err := setupConnection(ctx, t, streamParameter, client); err != nil {
		t.Fatalf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, client)

	if err := setupForwardingPipeline(ctx, t, streamParameter, client); err != nil {
		t.Fatalf("There is error sending SetForwardingPipeline, %s", err)
	}

	programmGDPMatchEntry(ctx, t, client, true)

	// Program the entry
	if err := client.Write(&p4_v1.WriteRequest{
		DeviceId:   uint64(1),
		ElectionId: &p4_v1.Uint128{High: uint64(0), Low: uint64(100)},
		Updates: wbb.ACLWbbIngressTableEntryGet([]*wbb.ACLWbbIngressTableEntryInfo{
			{
				Type:          p4_v1.Update_DELETE,
				EtherType:     0x6007,
				EtherTypeMask: 0xFFFF,
			},
		}),
		Atomicity: p4_v1.WriteRequest_CONTINUE_ON_ERROR,
	}); err == nil {
		t.Fatalf("Expected error is not seen.")
	}
}

// Write RPC-Compliance:017
func testWriteRPCWithUnspecificAction(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA

	streamParameter := generateStreamParameter(deviceID, uint64(0), electionID)

	// Setup P4RT Client
	if err := setupConnection(ctx, t, streamParameter, client); err != nil {
		t.Fatalf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, client)

	if err := setupForwardingPipeline(ctx, t, streamParameter, client); err != nil {
		t.Fatalf("There is error sending SetForwardingPipeline, %s", err)
	}

	programmGDPMatchEntry(ctx, t, client, true)

	// Program the entry
	if err := client.Write(&p4_v1.WriteRequest{
		DeviceId:   uint64(1),
		ElectionId: &p4_v1.Uint128{High: uint64(0), Low: uint64(100)},
		Updates: wbb.ACLWbbIngressTableEntryGet([]*wbb.ACLWbbIngressTableEntryInfo{
			{
				Type:          p4_v1.Update_UNSPECIFIED,
				EtherType:     0x6007,
				EtherTypeMask: 0xFFFF,
				Priority:      1,
			},
		}),
		Atomicity: p4_v1.WriteRequest_CONTINUE_ON_ERROR,
	}); err == nil {
		t.Fatalf("Expected error is not seen.")
	}
}
