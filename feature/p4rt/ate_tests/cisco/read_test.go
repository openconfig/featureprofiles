package cisco_p4rt_test

import (
	"context"
	"testing"
)

var (
	P4RTComplianceReadRPC = []Testcase{
		{
			name: "Read entry from Primary Client",
			desc: "Read RPC-Compliance:001 Verify Read RPC works when it's from Primary controller",
			fn:   testReadRPCFromPrimary,
		},
		{
			name: "Read entry from Non-Primary Client",
			desc: "Read RPC-Compliance:002 Verify Read RPC works when it's from Backup controller",
			fn:   testReadRPCFromBackup,
		},
		{
			name: "Read entry on non-exist device ID",
			desc: "Read RPC-Compliance:003 13 Read RPC with non-existing device-id, verify server returns with NOT_FOUND error",
			fn:   testReadRPCNonExistDeviceID,
		},
	}
)

// Read RPC-Compliance:001
func testReadRPCFromPrimary(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA

	// Setup P4RT Client
	if err := setupConnection(ctx, t, generateStreamParameter(deviceID, uint64(0), electionID), client); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, client)

	if err := setupForwardingPipeline(ctx, t, deviceID, client); err != nil {
		t.Errorf("There is error sending SetForwardingPipeline, %s", err)
	}

	// Programm one entry
	programmGDPMatchEntry(ctx, t, client, false)
	defer programmGDPMatchEntry(ctx, t, client, true)

	// Read that entry
	// Read ALL and log
	readProgrammedEntry(ctx, t, deviceID, client)
}

// Read RPC-Compliance:002
func testReadRPCFromBackup(ctx context.Context, t *testing.T, args *testArgs) {
	clientA := args.p4rtClientA
	clientB := args.p4rtClientB

	// Setup P4RT Client
	if err := setupConnection(ctx, t, generateStreamParameter(deviceID, uint64(0), electionID), clientA); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, clientA)

	if err := setupForwardingPipeline(ctx, t, deviceID, clientA); err != nil {
		t.Errorf("There is error sending SetForwardingPipeline, %s", err)
	}

	// Setup P4RT Client
	if err := setupConnection(ctx, t, generateStreamParameter(deviceID, uint64(0), electionID-1), clientB); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, clientA)

	// Programm one entry
	programmGDPMatchEntry(ctx, t, clientA, false)
	defer programmGDPMatchEntry(ctx, t, clientA, true)

	// Read that entry
	// Read ALL and log
	readProgrammedEntry(ctx, t, deviceID, clientB)
}

// Read RPC-Compliance:002
func testReadRPCNonExistDeviceID(ctx context.Context, t *testing.T, args *testArgs) {
	clientA := args.p4rtClientA
	clientB := args.p4rtClientB

	// Setup P4RT Client
	if err := setupConnection(ctx, t, generateStreamParameter(deviceID, uint64(0), electionID), clientA); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, clientA)

	if err := setupForwardingPipeline(ctx, t, deviceID, clientA); err != nil {
		t.Errorf("There is error sending SetForwardingPipeline, %s", err)
	}

	// Setup P4RT Client
	if err := setupConnection(ctx, t, generateStreamParameter(deviceID, uint64(0), electionID-1), clientB); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, clientA)

	// Programm one entry
	programmGDPMatchEntry(ctx, t, clientA, false)
	defer programmGDPMatchEntry(ctx, t, clientA, true)

	// Read that entry
	if err := readProgrammedEntry(ctx, t, ^uint64(1), clientA); err == nil {
		t.Errorf("Expected error is not seen.")
	}
}
