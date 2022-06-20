package cisco_p4rt_test

import (
	"context"
	"testing"

	p4_v1 "github.com/p4lang/p4runtime/go/p4/v1"
	"wwwin-github.cisco.com/rehaddad/go-p4/p4info/wbb"
)

// Write RPC-Compliance:001
func testWriteRPCInsertTrapAction(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA
	// Setup P4RT Client
	if err := setupConnection(ctx, t, deviceID, client); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, client)

	if err := setupForwardingPipeline(ctx, t, deviceID, client); err != nil {
		t.Errorf("There is error sending SetForwardingPipeline, %s", err)
	}

	// Program the entry
	if err := client.Write(&p4_v1.WriteRequest{
		DeviceId:   uint64(1),
		ElectionId: &p4_v1.Uint128{High: uint64(0), Low: uint64(100)},
		Updates: wbb.AclWbbIngressTableEntryGet([]*wbb.AclWbbIngressTableEntryInfo{
			&wbb.AclWbbIngressTableEntryInfo{
				Type:          p4_v1.Update_INSERT,
				EtherType:     0x6007,
				EtherTypeMask: 0xFFFF,
			},
		}),
		Atomicity: p4_v1.WriteRequest_CONTINUE_ON_ERROR,
	}); err != nil {
		t.Errorf("There is error sending SetForwardingPipeline, %s", err)
	}
}

// Write RPC-Compliance:002
func testWriteRPCInsertCopyAction(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA
	// Setup P4RT Client
	if err := setupConnection(ctx, t, deviceID, client); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, client)

	if err := setupForwardingPipeline(ctx, t, deviceID, client); err != nil {
		t.Errorf("There is error sending SetForwardingPipeline, %s", err)
	}

	// Program the entry
	if err := client.Write(&p4_v1.WriteRequest{
		DeviceId:   uint64(1),
		ElectionId: &p4_v1.Uint128{High: uint64(0), Low: uint64(100)},
		Updates: wbb.AclWbbIngressTableEntryGet([]*wbb.AclWbbIngressTableEntryInfo{
			&wbb.AclWbbIngressTableEntryInfo{
				Type:          p4_v1.Update_INSERT,
				EtherType:     0x6007,
				EtherTypeMask: 0xFFFF,
			},
		}),
		Atomicity: p4_v1.WriteRequest_CONTINUE_ON_ERROR,
	}); err != nil {
		t.Errorf("There is error sending SetForwardingPipeline, %s", err)
	}
}

// Write RPC-Compliance:003
func testWriteRPCInsertNonExistDeviceID(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA
	// Setup P4RT Client
	if err := setupConnection(ctx, t, deviceID, client); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, client)

	if err := setupForwardingPipeline(ctx, t, deviceID, client); err != nil {
		t.Errorf("There is error sending SetForwardingPipeline, %s", err)
	}

	// Program the entry
	if err := client.Write(&p4_v1.WriteRequest{
		DeviceId:   ^uint64(1),
		ElectionId: &p4_v1.Uint128{High: uint64(0), Low: uint64(100)},
		Updates: wbb.AclWbbIngressTableEntryGet([]*wbb.AclWbbIngressTableEntryInfo{
			&wbb.AclWbbIngressTableEntryInfo{
				Type:          p4_v1.Update_INSERT,
				EtherType:     0x6007,
				EtherTypeMask: 0xFFFF,
			},
		}),
		Atomicity: p4_v1.WriteRequest_CONTINUE_ON_ERROR,
	}); err == nil {
		t.Errorf("Expected error is not seen.")
	}
}

// Write RPC-Compliance:004
func testWriteRPCInsertWithLowerElectionID(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA
	// Setup P4RT Client
	if err := setupConnection(ctx, t, deviceID, client); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, client)

	if err := setupForwardingPipeline(ctx, t, deviceID, client); err != nil {
		t.Errorf("There is error sending SetForwardingPipeline, %s", err)
	}

	// Program the entry
	if err := client.Write(&p4_v1.WriteRequest{
		DeviceId:   uint64(1),
		ElectionId: &p4_v1.Uint128{High: uint64(0), Low: uint64(99)},
		Updates: wbb.AclWbbIngressTableEntryGet([]*wbb.AclWbbIngressTableEntryInfo{
			&wbb.AclWbbIngressTableEntryInfo{
				Type:          p4_v1.Update_INSERT,
				EtherType:     0x6007,
				EtherTypeMask: 0xFFFF,
			},
		}),
		Atomicity: p4_v1.WriteRequest_CONTINUE_ON_ERROR,
	}); err == nil {
		t.Errorf("Expected error is not seen.")
	}
}

// Write RPC-Compliance:005
func testWriteRPCInsertWithHigherElectionID(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA
	// Setup P4RT Client
	if err := setupConnection(ctx, t, deviceID, client); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, client)

	if err := setupForwardingPipeline(ctx, t, deviceID, client); err != nil {
		t.Errorf("There is error sending SetForwardingPipeline, %s", err)
	}

	// Program the entry
	if err := client.Write(&p4_v1.WriteRequest{
		DeviceId:   uint64(1),
		ElectionId: &p4_v1.Uint128{High: uint64(0), Low: uint64(101)},
		Updates: wbb.AclWbbIngressTableEntryGet([]*wbb.AclWbbIngressTableEntryInfo{
			&wbb.AclWbbIngressTableEntryInfo{
				Type:          p4_v1.Update_INSERT,
				EtherType:     0x6007,
				EtherTypeMask: 0xFFFF,
			},
		}),
		Atomicity: p4_v1.WriteRequest_CONTINUE_ON_ERROR,
	}); err == nil {
		t.Errorf("Expected error is not seen.")
	}
}

// Write RPC-Compliance:006
func testWriteRPCBeforeSetForwardingPipeline(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA
	// Setup P4RT Client
	if err := setupConnection(ctx, t, deviceID, client); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, client)

	// Program the entry
	if err := client.Write(&p4_v1.WriteRequest{
		DeviceId:   uint64(1),
		ElectionId: &p4_v1.Uint128{High: uint64(0), Low: uint64(100)},
		Updates: wbb.AclWbbIngressTableEntryGet([]*wbb.AclWbbIngressTableEntryInfo{
			&wbb.AclWbbIngressTableEntryInfo{
				Type:          p4_v1.Update_INSERT,
				EtherType:     0x6007,
				EtherTypeMask: 0xFFFF,
			},
		}),
		Atomicity: p4_v1.WriteRequest_CONTINUE_ON_ERROR,
	}); err == nil {
		t.Errorf("Expected error is not seen.")
	}
}

// Write RPC-Compliance:007
func testWriteRPCInsertEntry(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA
	// Setup P4RT Client
	if err := setupConnection(ctx, t, deviceID, client); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, client)

	if err := setupForwardingPipeline(ctx, t, deviceID, client); err != nil {
		t.Errorf("There is error sending SetForwardingPipeline, %s", err)
	}

	// Program the entry
	if err := programmGDPMatchEntry(ctx, t, client, false); err != nil {
		t.Errorf("There is error programming the entry, %s", err)
	}
}

// Write RPC-Compliance:008
func testWriteRPCInsertSameEntry(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA
	// Setup P4RT Client
	if err := setupConnection(ctx, t, deviceID, client); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, client)

	if err := setupForwardingPipeline(ctx, t, deviceID, client); err != nil {
		t.Errorf("There is error sending SetForwardingPipeline, %s", err)
	}

	// Program the entry
	if err := programmGDPMatchEntry(ctx, t, client, false); err != nil {
		t.Errorf("There is error programming the entry, %s", err)
	}

	// Program the entry 2nd time and expecting error
	if err := programmGDPMatchEntry(ctx, t, client, false); err == nil {
		t.Errorf("Expected error not seen")
	}
}

// Write RPC-Compliance:009
func testWriteRPCInsertMalformedEntry(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA
	// Setup P4RT Client
	if err := setupConnection(ctx, t, deviceID, client); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, client)

	if err := setupForwardingPipeline(ctx, t, deviceID, client); err != nil {
		t.Errorf("There is error sending SetForwardingPipeline, %s", err)
	}
	// Program the entry
	if err := client.Write(&p4_v1.WriteRequest{
		DeviceId:   uint64(1),
		ElectionId: &p4_v1.Uint128{High: uint64(0), Low: uint64(100)},
		Updates: wbb.AclWbbIngressTableEntryGet([]*wbb.AclWbbIngressTableEntryInfo{
			&wbb.AclWbbIngressTableEntryInfo{
				Type: p4_v1.Update_INSERT,
				// EtherType: 0x6007,
			},
		}),
		Atomicity: p4_v1.WriteRequest_CONTINUE_ON_ERROR,
	}); err == nil {
		t.Errorf("Expected error is not seen.")
	}
}

// Write RPC-Compliance:010
func testWriteRPCOOR(ctx context.Context, t *testing.T, args *testArgs) {
	t.Logf("TODO: Add new clients to simulate OOR condition")
	t.Skip()
	client := args.p4rtClientA
	// Setup P4RT Client
	if err := setupConnection(ctx, t, deviceID, client); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, client)

	if err := setupForwardingPipeline(ctx, t, deviceID, client); err != nil {
		t.Errorf("There is error sending SetForwardingPipeline, %s", err)
	}
	// Program the entry
	if err := client.Write(&p4_v1.WriteRequest{
		DeviceId:   uint64(1),
		ElectionId: &p4_v1.Uint128{High: uint64(0), Low: uint64(100)},
		Updates: wbb.AclWbbIngressTableEntryGet([]*wbb.AclWbbIngressTableEntryInfo{
			&wbb.AclWbbIngressTableEntryInfo{
				Type: p4_v1.Update_INSERT,
				// EtherType: 0x6007,
			},
		}),
		Atomicity: p4_v1.WriteRequest_CONTINUE_ON_ERROR,
	}); err == nil {
		t.Errorf("Expected error is not seen.")
	}
}

// Write RPC-Compliance:011
func testWriteRPCModifyEntry(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA
	// Setup P4RT Client
	if err := setupConnection(ctx, t, deviceID, client); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, client)

	if err := setupForwardingPipeline(ctx, t, deviceID, client); err != nil {
		t.Errorf("There is error sending SetForwardingPipeline, %s", err)
	}

	programmGDPMatchEntry(ctx, t, client, false)

	// Program the entry
	if err := client.Write(&p4_v1.WriteRequest{
		DeviceId:   uint64(1),
		ElectionId: &p4_v1.Uint128{High: uint64(0), Low: uint64(100)},
		Updates: wbb.AclWbbIngressTableEntryGet([]*wbb.AclWbbIngressTableEntryInfo{
			&wbb.AclWbbIngressTableEntryInfo{
				Type:          p4_v1.Update_MODIFY,
				EtherType:     0x6007,
				EtherTypeMask: 0xFFFF,
			},
		}),
		Atomicity: p4_v1.WriteRequest_CONTINUE_ON_ERROR,
	}); err == nil {
		t.Errorf("Expected error is not seen.")
	}
}

// Write RPC-Compliance:012
func testWriteRPCModifyMalformedEntry(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA
	// Setup P4RT Client
	if err := setupConnection(ctx, t, deviceID, client); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, client)

	if err := setupForwardingPipeline(ctx, t, deviceID, client); err != nil {
		t.Errorf("There is error sending SetForwardingPipeline, %s", err)
	}

	// Program the entry
	if err := client.Write(&p4_v1.WriteRequest{
		DeviceId:   uint64(1),
		ElectionId: &p4_v1.Uint128{High: uint64(0), Low: uint64(100)},
		Updates: wbb.AclWbbIngressTableEntryGet([]*wbb.AclWbbIngressTableEntryInfo{
			&wbb.AclWbbIngressTableEntryInfo{
				Type:      p4_v1.Update_MODIFY,
				EtherType: 0x6007,
			},
		}),
		Atomicity: p4_v1.WriteRequest_CONTINUE_ON_ERROR,
	}); err == nil {
		t.Errorf("Expected error is not seen.")
	}
}

// Write RPC-Compliance:013
func testWriteRPCModifyNonExistEntry(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA
	// Setup P4RT Client
	if err := setupConnection(ctx, t, deviceID, client); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, client)

	if err := setupForwardingPipeline(ctx, t, deviceID, client); err != nil {
		t.Errorf("There is error sending SetForwardingPipeline, %s", err)
	}

	programmGDPMatchEntry(ctx, t, client, false)

	// Program the entry
	if err := client.Write(&p4_v1.WriteRequest{
		DeviceId:   uint64(1),
		ElectionId: &p4_v1.Uint128{High: uint64(0), Low: uint64(100)},
		Updates: wbb.AclWbbIngressTableEntryGet([]*wbb.AclWbbIngressTableEntryInfo{
			&wbb.AclWbbIngressTableEntryInfo{
				Type:          p4_v1.Update_MODIFY,
				EtherType:     0x6666,
				EtherTypeMask: 0xFFFF,
			},
		}),
		Atomicity: p4_v1.WriteRequest_CONTINUE_ON_ERROR,
	}); err == nil {
		t.Errorf("Expected error is not seen.")
	}
}

// Write RPC-Compliance:014
func testWriteRPCDeleteEntry(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA
	// Setup P4RT Client
	if err := setupConnection(ctx, t, deviceID, client); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, client)

	if err := setupForwardingPipeline(ctx, t, deviceID, client); err != nil {
		t.Errorf("There is error sending SetForwardingPipeline, %s", err)
	}

	programmGDPMatchEntry(ctx, t, client, false)

	// Program the entry
	if err := programmGDPMatchEntry(ctx, t, client, true); err != nil {
		t.Errorf("Unexpected error seen, %s", err)
	}
}

// Write RPC-Compliance:015
func testWriteRPCDeleteMalformedEntry(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA
	// Setup P4RT Client
	if err := setupConnection(ctx, t, deviceID, client); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, client)

	if err := setupForwardingPipeline(ctx, t, deviceID, client); err != nil {
		t.Errorf("There is error sending SetForwardingPipeline, %s", err)
	}

	programmGDPMatchEntry(ctx, t, client, false)

	// Program the entry
	if err := client.Write(&p4_v1.WriteRequest{
		DeviceId:   uint64(1),
		ElectionId: &p4_v1.Uint128{High: uint64(0), Low: uint64(100)},
		Updates: wbb.AclWbbIngressTableEntryGet([]*wbb.AclWbbIngressTableEntryInfo{
			&wbb.AclWbbIngressTableEntryInfo{
				Type:      p4_v1.Update_DELETE,
				EtherType: 0x6007,
			},
		}),
		Atomicity: p4_v1.WriteRequest_CONTINUE_ON_ERROR,
	}); err == nil {
		t.Errorf("Expected error is not seen.")
	}
}

// Write RPC-Compliance:016
func testWriteRPCDeleteNonExistEntry(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA
	// Setup P4RT Client
	if err := setupConnection(ctx, t, deviceID, client); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, client)

	if err := setupForwardingPipeline(ctx, t, deviceID, client); err != nil {
		t.Errorf("There is error sending SetForwardingPipeline, %s", err)
	}

	programmGDPMatchEntry(ctx, t, client, true)

	// Program the entry
	if err := client.Write(&p4_v1.WriteRequest{
		DeviceId:   uint64(1),
		ElectionId: &p4_v1.Uint128{High: uint64(0), Low: uint64(100)},
		Updates: wbb.AclWbbIngressTableEntryGet([]*wbb.AclWbbIngressTableEntryInfo{
			&wbb.AclWbbIngressTableEntryInfo{
				Type:          p4_v1.Update_DELETE,
				EtherType:     0x6007,
				EtherTypeMask: 0xFFFF,
			},
		}),
		Atomicity: p4_v1.WriteRequest_CONTINUE_ON_ERROR,
	}); err == nil {
		t.Errorf("Expected error is not seen.")
	}
}

// Write RPC-Compliance:017
func testWriteRPCWithUnspecificAction(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA
	// Setup P4RT Client
	if err := setupConnection(ctx, t, deviceID, client); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, client)

	if err := setupForwardingPipeline(ctx, t, deviceID, client); err != nil {
		t.Errorf("There is error sending SetForwardingPipeline, %s", err)
	}

	programmGDPMatchEntry(ctx, t, client, true)

	// Program the entry
	if err := client.Write(&p4_v1.WriteRequest{
		DeviceId:   uint64(1),
		ElectionId: &p4_v1.Uint128{High: uint64(0), Low: uint64(100)},
		Updates: wbb.AclWbbIngressTableEntryGet([]*wbb.AclWbbIngressTableEntryInfo{
			&wbb.AclWbbIngressTableEntryInfo{
				Type:          p4_v1.Update_UNSPECIFIED,
				EtherType:     0x6007,
				EtherTypeMask: 0xFFFF,
			},
		}),
		Atomicity: p4_v1.WriteRequest_CONTINUE_ON_ERROR,
	}); err == nil {
		t.Errorf("Expected error is not seen.")
	}
}
