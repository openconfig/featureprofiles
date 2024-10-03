package cisco_p4rt_test

import (
	"context"
	"testing"

	wbb "github.com/openconfig/featureprofiles/internal/p4rtutils"
)

var (
	gdpTableEntry = wbb.ACLWbbIngressTableEntryGet([]*wbb.ACLWbbIngressTableEntryInfo{{
		EtherType:     0x6007,
		EtherTypeMask: 0xFFFF,
	},
	})[0].Entity.GetTableEntry()
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

	streamParameter := generateStreamParameter(deviceID, uint64(0), electionID)
	// Setup P4RT Client
	if err := setupConnection(ctx, t, streamParameter, client); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, client)

	if err := setupForwardingPipeline(ctx, t, streamParameter, client); err != nil {
		t.Errorf("There is error sending SetForwardingPipeline, %s", err)
	}

	// Programm one entry
	programmGDPMatchEntry(ctx, t, client, false)
	defer programmGDPMatchEntry(ctx, t, client, true)

	// Read that entry
	// Read ALL and log
	res, err := readProgrammedEntry(ctx, t, deviceID, client)
	if err != nil {
		t.Errorf("There is error seen when reading entries, %v", err)
	}

	checkEntryExist(ctx, t, gdpTableEntry, res)
}

// Read RPC-Compliance:002
func testReadRPCFromBackup(ctx context.Context, t *testing.T, args *testArgs) {
	clientA := args.p4rtClientA
	clientB := args.p4rtClientB

	streamParameter := generateStreamParameter(deviceID, uint64(0), electionID)
	// Setup P4RT Client
	if err := setupConnection(ctx, t, streamParameter, clientA); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, clientA)

	if err := setupForwardingPipeline(ctx, t, streamParameter, clientA); err != nil {
		t.Errorf("There is error sending SetForwardingPipeline, %s", err)
	}

	// Setup P4RT Client
	streamParameter.ElectionIdL -= 1
	if err := setupConnection(ctx, t, streamParameter, clientB); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, clientB)

	// Programm one entry
	programmGDPMatchEntry(ctx, t, clientA, false)
	defer programmGDPMatchEntry(ctx, t, clientA, true)

	// Read that entry
	// Read ALL and log
	res, err := readProgrammedEntry(ctx, t, deviceID, clientB)
	if err != nil {
		t.Errorf("There is error seen when reading entries, %v", err)
	}

	checkEntryExist(ctx, t, gdpTableEntry, res)
}

// Read RPC-Compliance:002
func testReadRPCNonExistDeviceID(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA

	streamParameter := generateStreamParameter(deviceID, uint64(0), electionID)

	// Setup P4RT Client
	if err := setupConnection(ctx, t, streamParameter, client); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, client)

	if err := setupForwardingPipeline(ctx, t, streamParameter, client); err != nil {
		t.Errorf("There is error sending SetForwardingPipeline, %s", err)
	}

	// Programm one entry
	programmGDPMatchEntry(ctx, t, client, false)
	defer programmGDPMatchEntry(ctx, t, client, true)

	// Read that entry
	if _, err := readProgrammedEntry(ctx, t, ^uint64(1), client); err == nil {
		t.Errorf("Expected error is not seen.")
	}
}
