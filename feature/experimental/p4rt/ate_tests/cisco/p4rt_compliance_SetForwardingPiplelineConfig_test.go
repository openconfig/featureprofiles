package cisco_p4rt_test

import (
	"context"
	"testing"

	"github.com/cisco-open/go-p4/utils"
	p4_v1 "github.com/p4lang/p4runtime/go/p4/v1"
)

var (
	P4RTComplianceSetForwardingPipelineConfig = []Testcase{
		{
			name: "SetForwardingPipelineCfg from Primary Controller",
			desc: "SetForwardingPipelineConfig-Compliance:001 Verify primary controller with fwdpipeline config is accepted",
			fn:   testSetForwardingPipelineFromPrimary,
		},
		{
			name: "SetForwardingPipelineCfg from Backup Controller",
			desc: "SetForwardingPipelineConfig-Compliance:002 Verify standby controller with fwdpipeline config is rejected",
			fn:   testSetForwardingPipelineFromBackup,
			skip: true,
		},
		{
			name: "SetForwardingPipelineCfg to non-exist device id",
			desc: "SetForwardingPipelineConfig-Compliance:010 14.1 send non-exist device-id in SetForwardingPipelineConfig and verify device returns NOT_FOUND error",
			fn:   testSetForwardingPipelineOnNonExistDeviceID,
		},
		{
			name: "SetForwardingPipelineCfg from non-primary and verify error",
			desc: "SetForwardingPipelineConfig-Compliance:011 14.2 send SetForwardingPipelineConfig from non-primary client and verify device return PERMISSION_DENIED error",
			fn:   testSetForwardingPipelineFromNonPrimary,
		},
		{
			name: "SetForwardingPipelineCfg from primary with differnet election id",
			desc: "SetForwardingPipelineConfig-Compliance:012 14.2 send SetForwardingPipelineConfig from primary with differnet election id value, verify the device behavior, verify xr returns with PERMISSION_DENIED",
			fn:   testSetForwardingPipelineFromPrimaryWithDifferentElectionID,
		},
		{
			name: "SetForwardingPipelineCfg with VERIFY action with google p4info",
			desc: "SetForwardingPipelineConfig-Compliance:013 14(VERIFY) send google p4info with VERIFY action, verify device is able to realize it and doesn’t change current config",
			fn:   testVERIFYWithGoogleP4Info,
		},
		{
			name: "SetForwardingPipelineCfg with VERIFY action with other p4info",
			desc: "SetForwardingPipelineConfig-Compliance:014 14(VERIFY) send other p4info with VERIFY action, verify device is NOT able to realize it and return INVALID_ARGUMENT error",
			fn:   testVERIFYWithOtherP4Info,
		},
		{
			name: "SetForwardingPipelineCfg with VERIFY_AND_SAVE action with google p4info",
			desc: "SetForwardingPipelineConfig-Compliance:015 14(VERIFY_AND_SAVE) send google p4info with VERIFY_AND_SAVE action, verify device is able to realize it and doesn’t change current config. Also subsequenct Read/Write must refer to the new config",
			fn:   testVERIFYANDSAVEWithGoogleP4Info,
		},
		{
			name: "SetForwardingPipelineCfg with VERIFY_AND_SAVE action with other p4info",
			desc: "SetForwardingPipelineConfig-Compliance:016 14(VERIFY_AND_SAVE) send other p4info with VERIFY_AND_SAVE action, verify device is NOT able to realize it and return with INVALID_AGRUMENT",
			fn:   testVERIFYANDSAVEWithOtherP4Info,
		},
		{
			name: "SetForwardingPipelineCfg with VERIFY_AND_COMMIT action with google p4info",
			desc: "SetForwardingPipelineConfig-Compliance:017 14(VERIFY_AND_COMMIT) send google p4info with VERIFY_AND_COMMIT action, verify device is able to realize it, verify the forwarding state in the target is cleared",
			fn:   testVERIFYANDCOMMITWithGoogleP4Info,
		},
		{
			name: "SetForwardingPipelineCfg with VERIFY_AND_COMMIT action with other p4info",
			desc: "SetForwardingPipelineConfig-Compliance:018 14(VERIFY_AND_COMMIT) send other p4info with VERIFY_AND_COMMIT action, verify device is NOT able to realize it and return with INVALID_AGRUMENT",
			fn:   testVERIFYANDCOMMITWithOtherP4Info,
		},
		{
			name: "SetForwardingPipelineCfg with COMMIT action with google p4info",
			desc: "SetForwardingPipelineConfig-Compliance:019 14(COMMIT) send google p4info with VERIFY_AND_SAVE action first and then send B91ify device is able to realize it, verify device replays write request since last save action",
			fn:   testCOMMITWithGoogleP4Info,
		},
		{
			name: "SetForwardingPipelineCfg with Cookie",
			desc: "SetForwardingPipelineConfig-Compliance:024 14. add Cookie in the ForwardingPipeline msg and verify device behavior",
			fn:   testSetForwardingPipelineWithCookie,
		},
	}
)

// SetForwardingPipelineConfig-Compliance:001
func testSetForwardingPipelineFromPrimary(ctx context.Context, t *testing.T, args *testArgs) {
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
}

// SetForwardingPipelineConfig-Compliance:002
func testSetForwardingPipelineFromBackup(ctx context.Context, t *testing.T, args *testArgs) {
	clientA := args.p4rtClientA
	clientB := args.p4rtClientB

	streamParameter := generateStreamParameter(deviceID, uint64(0), electionID)
	// Setup P4RT Client
	if err := setupConnection(ctx, t, streamParameter, clientA); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, clientA)

	// Setup P4RT Client
	streamParameter.ElectionIdL -= 1
	if err := setupConnection(ctx, t, streamParameter, clientB); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, clientB)

	if err := setupForwardingPipeline(ctx, t, streamParameter, args.p4rtClientB); err == nil {
		t.Errorf("Expected error is not seen.")
	}
}

// SetForwardingPipelineConfig-Compliance:010
func testSetForwardingPipelineOnNonExistDeviceID(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA
	streamParameter := generateStreamParameter(deviceID, uint64(0), electionID)
	// Setup P4RT Client
	if err := setupConnection(ctx, t, streamParameter, client); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, client)

	streamParameter.DeviceId = ^streamParameter.DeviceId
	if err := setupForwardingPipeline(ctx, t, streamParameter, client); err == nil {
		t.Errorf("Expected error is not seen.")
	}
}

// SetForwardingPipelineConfig-Compliance:011
func testSetForwardingPipelineFromNonPrimary(ctx context.Context, t *testing.T, args *testArgs) {
	clientA := args.p4rtClientA
	clientB := args.p4rtClientB

	streamParameter := generateStreamParameter(deviceID, uint64(0), electionID)
	// Setup P4RT Client
	if err := setupConnection(ctx, t, streamParameter, clientA); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, clientA)

	// Setup P4RT Client
	streamParameter.ElectionIdL -= 1
	if err := setupConnection(ctx, t, streamParameter, clientB); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, clientB)

	err := setupForwardingPipeline(ctx, t, streamParameter, args.p4rtClientB)
	if err == nil {
		t.Errorf("Expected error is not seen.")
	}

	// TODO: check error details
}

// SetForwardingPipelineConfig-Compliance:012
func testSetForwardingPipelineFromPrimaryWithDifferentElectionID(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA

	streamParameter := generateStreamParameter(deviceID, uint64(0), electionID)
	// Setup P4RT Client
	if err := setupConnection(ctx, t, streamParameter, client); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, client)

	streamParameter.ElectionIdL += 1
	err := setupForwardingPipeline(ctx, t, streamParameter, client)
	if err == nil {
		t.Errorf("Expected error is not seen.")
	}

	streamParameter.ElectionIdL -= 2
	err = setupForwardingPipeline(ctx, t, streamParameter, client)
	if err == nil {
		t.Errorf("Expected error is not seen.")
	}

	// TODO: check error details
}

func testSetForwardingPipelineAction(ctx context.Context, t *testing.T, args *testArgs, action p4_v1.SetForwardingPipelineConfigRequest_Action, otherP4InfoFile bool) {
	client := args.p4rtClientA

	streamParameter := generateStreamParameter(deviceID, uint64(0), electionID)
	// Setup P4RT Client
	if err := setupConnection(ctx, t, streamParameter, client); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, client)

	p4Info, err := utils.P4InfoLoad(p4InfoFile)
	if err != nil {
		t.Logf("There is error when loading p4info file")
	}
	// remove acl_wbb_ingress.acl_wbb_ingress_table
	if otherP4InfoFile {
		tables := p4Info.GetTables()
		for _, table := range tables {
			if table.Preamble.Name == "ingress.acl_wbb_ingress.acl_wbb_ingress_table" {
				table = nil
			}
		}
	}

	err = client.SetForwardingPipelineConfig(&p4_v1.SetForwardingPipelineConfigRequest{
		DeviceId:   streamParameter.DeviceId,
		ElectionId: &p4_v1.Uint128{High: streamParameter.ElectionIdH, Low: streamParameter.ElectionIdL},
		Action:     action,
		Config: &p4_v1.ForwardingPipelineConfig{
			P4Info: p4Info,
		},
	})
	if otherP4InfoFile && err == nil {
		t.Errorf("There is error when loading p4info file, %s", err)

	} else if err != nil {
		t.Errorf("There is error when loading p4info file, %s", err)
	}
}

// SetForwardingPipelineConfig-Compliance:013
func testVERIFYWithGoogleP4Info(ctx context.Context, t *testing.T, args *testArgs) {
	testSetForwardingPipelineAction(ctx, t, args, p4_v1.SetForwardingPipelineConfigRequest_VERIFY, false)
}

// SetForwardingPipelineConfig-Compliance:014
func testVERIFYWithOtherP4Info(ctx context.Context, t *testing.T, args *testArgs) {
	testSetForwardingPipelineAction(ctx, t, args, p4_v1.SetForwardingPipelineConfigRequest_VERIFY, true)
}

// SetForwardingPipelineConfig-Compliance:015
func testVERIFYANDSAVEWithGoogleP4Info(ctx context.Context, t *testing.T, args *testArgs) {
	testSetForwardingPipelineAction(ctx, t, args, p4_v1.SetForwardingPipelineConfigRequest_VERIFY_AND_SAVE, false)
}

// SetForwardingPipelineConfig-Compliance:016
func testVERIFYANDSAVEWithOtherP4Info(ctx context.Context, t *testing.T, args *testArgs) {
	testSetForwardingPipelineAction(ctx, t, args, p4_v1.SetForwardingPipelineConfigRequest_VERIFY_AND_SAVE, true)
}

// SetForwardingPipelineConfig-Compliance:017
func testVERIFYANDCOMMITWithGoogleP4Info(ctx context.Context, t *testing.T, args *testArgs) {
	testSetForwardingPipelineAction(ctx, t, args, p4_v1.SetForwardingPipelineConfigRequest_VERIFY_AND_COMMIT, false)
}

// SetForwardingPipelineConfig-Compliance:018
func testVERIFYANDCOMMITWithOtherP4Info(ctx context.Context, t *testing.T, args *testArgs) {
	testSetForwardingPipelineAction(ctx, t, args, p4_v1.SetForwardingPipelineConfigRequest_VERIFY_AND_COMMIT, true)
}

// SetForwardingPipelineConfig-Compliance:019
func testCOMMITWithGoogleP4Info(ctx context.Context, t *testing.T, args *testArgs) {
	testSetForwardingPipelineAction(ctx, t, args, p4_v1.SetForwardingPipelineConfigRequest_COMMIT, false)
}

// SetForwardingPipelineConfig-Compliance:024
func testSetForwardingPipelineWithCookie(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA
	streamParameter := generateStreamParameter(deviceID, uint64(0), electionID)
	// Setup P4RT Client
	if err := setupConnection(ctx, t, streamParameter, client); err != nil {
		t.Errorf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, client)

	p4Info, err := utils.P4InfoLoad(p4InfoFile)
	if err != nil {
		t.Errorf("There is error when loading p4info file")
	}

	if err := client.SetForwardingPipelineConfig(&p4_v1.SetForwardingPipelineConfigRequest{
		DeviceId:   streamParameter.DeviceId,
		ElectionId: &p4_v1.Uint128{High: streamParameter.ElectionIdH, Low: streamParameter.ElectionIdL},
		Action:     p4_v1.SetForwardingPipelineConfigRequest_VERIFY_AND_COMMIT,
		Config: &p4_v1.ForwardingPipelineConfig{
			P4Info: p4Info,
			Cookie: &p4_v1.ForwardingPipelineConfig_Cookie{
				Cookie: 159,
			},
		},
	}); err != nil {
		t.Errorf("There is error seen when SetForwardingPipelineConfig, %s", err)
	}
}
