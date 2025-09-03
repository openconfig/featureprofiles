package cisco_p4rt_test

import (
	"context"
	"testing"

	"github.com/cisco-open/go-p4/utils"
	"github.com/google/go-cmp/cmp"
	v1 "github.com/p4lang/p4runtime/go/p4/config/v1"
	p4_v1 "github.com/p4lang/p4runtime/go/p4/v1"
)

var (
	P4RTComplianceGetForwardingPipelineConfig = []Testcase{
		{
			name: "GetForwardingPipelineConfig with p4info programmed on the router",
			desc: "GetForwardingPipelineConfig-Compliance:001 Verify device with ForwardPipelineConfig set returns google p4info when send GetForwardingPipelineConfig",
			fn:   testGetForwardingPipelineFromPrimary,
		},
		{
			name: "GetForwardingPipelineConfig without p4info programmed on the router",
			desc: "GetForwardingPipelineConfig-Compliance:002 Verify device without ForwardPipelineConfig set returns empty when send GetForwardingPipelineConfig",
			fn:   testGetForwardingPipelineFromEmpty,
		},
		{
			name: "GetForwardingPipelineConfig from backup controller",
			desc: "GetForwardingPipelineConfig-Compliance:003 Verify GetForwardingPipelineConfig from backup controller works fine",
			fn:   testGetForwardingPipelineFromBackup,
		},
		{
			name: "GetForwardingPipelineConfig from previous primary controller",
			desc: "GetForwardingPipelineConfig-Compliance:004 Verify GetForwardingPipelineConfig from previous primary controller works fine",
			fn:   testGetForwardingPipelineFromPreviousPrimary,
		},
		{
			name: "GetForwardingPipelineConfig from downgraded controller",
			desc: "GetForwardingPipelineConfig-Compliance:005 Verify GetForwardingPipelineConfig from downgraded primary controller works fine",
			fn:   testGetForwardingPipelineFromDowngradedPrimary,
		},
		{
			name: "GetForwardingPipelineConfig on non-exist device id",
			desc: "GetForwardingPipelineConfig-Compliance:008 15 GetForwardingPipelineConfig send with non-exisit device-id, verify server respond with NOT_FOUND error",
			fn:   testGetForwardingPipelineOnNonExistDeviceID,
		},
		{
			name: "GetForwardingPipelineConfig with ALL response type",
			desc: "GetForwardingPipelineConfig-Compliance:009 15(ALL) Default behavior. Verify device returns the ForwardingPipelineConfig running on the device",
			fn:   testGetForwardingPipelineWithALL,
		},
		{
			name: "GetForwardingPipelineConfig with Cookie_Only response type",
			desc: "GetForwardingPipelineConfig-Compliance:010 15(COOKIE_ONLY) With COOKE_ONLY set in the GetForwardingPipelineConfig, Verify device returns with Cookie only",
			fn:   testGetForwardingPipelineWithCookieOnly,
		},
		{
			name: "GetForwardingPipelineConfig with P4info_And_Cookie response type",
			desc: "GetForwardingPipelineConfig-Compliance:011 15(P4INFO_AND_COOKIE) With P4INFO_AND_COOKIE set in GetForwardingPipelineConfig, verify device respond with P4info and cookie",
			fn:   testGetForwardingPipelineWithP4InfoAndCookie,
		},
		{
			name: "GetForwardingPipelineConfig with Device_Config_And_Cookie response type",
			desc: "GetForwardingPipelineConfig-Compliance:012 15(DEVICE_CONIFG_AND_COOKIE) with DEVICE_CONFIG_AND_COOKIE set in GetForwardingPipelineConfig, verify device returns with device config and cookie",
			fn:   testGetForwardingPipelineWithDeviceConfigAndCookie,
		},
	}
)

// GetForwardingPipelineConfig-Compliance:001
func testGetForwardingPipelineFromPrimary(ctx context.Context, t *testing.T, args *testArgs) {
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

	resp, err := getForwardingPipeline(ctx, t, streamParameter, client, p4_v1.GetForwardingPipelineConfigRequest_ALL)
	if err != nil {
		t.Fatalf("There is error in GetForwardingPipelineConfig. %s", err)
	}

	wantP4Info, _ := utils.P4InfoLoad(p4InfoFile)
	gotP4Info := resp.Config.GetP4Info()
	if !cmp.Equal(wantP4Info, *gotP4Info) {
		t.Fatalf("There is error in GetForwardingPipelineConfig.")
	}
}

// GetForwardingPipelineConfig-Compliance:002
func testGetForwardingPipelineFromEmpty(ctx context.Context, t *testing.T, args *testArgs) {
	// TODO: clean up current ForwardingPipleConfig on the device

	client := args.p4rtClientA
	streamParameter := generateStreamParameter(deviceID, uint64(0), electionID)
	// Setup P4RT Client
	if err := setupConnection(ctx, t, streamParameter, client); err != nil {
		t.Fatalf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, client)

	resp, err := getForwardingPipeline(ctx, t, streamParameter, client, p4_v1.GetForwardingPipelineConfigRequest_ALL)
	if err != nil {
		t.Fatalf("There is error in GetForwardingPipelineConfig. %s", err)
	}

	wantP4Info := v1.P4Info{}
	gotP4Info := resp.Config.GetP4Info()
	if !cmp.Equal(*gotP4Info, wantP4Info) {
		t.Fatalf("There is error in GetForwardingPipelineConfig.")
	}
}

// GetForwardingPipelineConfig-Compliance:003
func testGetForwardingPipelineFromBackup(ctx context.Context, t *testing.T, args *testArgs) {
	clientA := args.p4rtClientA
	clientB := args.p4rtClientB

	streamParameter := generateStreamParameter(deviceID, uint64(0), electionID)
	// Setup P4RT Client
	if err := setupConnection(ctx, t, streamParameter, clientA); err != nil {
		t.Fatalf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, clientA)

	if err := setupForwardingPipeline(ctx, t, streamParameter, clientA); err != nil {
		t.Fatalf("There is error sending SetForwardingPipeline, %s", err)
	}

	// Setup P4RT Client
	streamParameter.ElectionIdL -= 1
	if err := setupConnection(ctx, t, streamParameter, clientB); err != nil {
		t.Fatalf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, clientB)

	resp, err := getForwardingPipeline(ctx, t, streamParameter, clientB, p4_v1.GetForwardingPipelineConfigRequest_ALL)
	if err != nil {
		t.Fatalf("There is error in GetForwardingPipelineConfig. %s", err)
	}

	wantP4Info, _ := utils.P4InfoLoad(p4InfoFile)
	gotP4Info := resp.Config.GetP4Info()
	if !cmp.Equal(wantP4Info, *gotP4Info) {
		t.Fatalf("There is error in GetForwardingPipelineConfig.")
	}
}

// GetForwardingPipelineConfig-Compliance:004
func testGetForwardingPipelineFromPreviousPrimary(ctx context.Context, t *testing.T, args *testArgs) {
	clientA := args.p4rtClientA
	clientB := args.p4rtClientB

	streamParameter := generateStreamParameter(deviceID, uint64(0), electionID)
	// Setup P4RT Client
	if err := setupConnection(ctx, t, streamParameter, clientA); err != nil {
		t.Fatalf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, clientA)

	if err := setupForwardingPipeline(ctx, t, streamParameter, clientA); err != nil {
		t.Fatalf("There is error sending SetForwardingPipeline, %s", err)
	}

	// Setup P4RT Client
	streamParameter.ElectionIdL += 1
	if err := setupConnection(ctx, t, streamParameter, clientB); err != nil {
		t.Fatalf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, clientB)

	if err := setupForwardingPipeline(ctx, t, streamParameter, clientB); err != nil {
		t.Fatalf("There is error sending SetForwardingPipeline, %s", err)
	}

	resp, err := getForwardingPipeline(ctx, t, streamParameter, clientA, p4_v1.GetForwardingPipelineConfigRequest_ALL)
	if err != nil {
		t.Fatalf("There is error in GetForwardingPipelineConfig. %s", err)
	}

	wantP4Info, _ := utils.P4InfoLoad(p4InfoFile)
	gotP4Info := resp.Config.GetP4Info()
	if !cmp.Equal(wantP4Info, *gotP4Info) {
		t.Fatalf("There is error in GetForwardingPipelineConfig.")
	}
}

// GetForwardingPipelineConfig-Compliance:005
func testGetForwardingPipelineFromDowngradedPrimary(ctx context.Context, t *testing.T, args *testArgs) {
	clientA := args.p4rtClientA
	clientB := args.p4rtClientB

	streamParameter := generateStreamParameter(deviceID, uint64(0), electionID)
	// Setup P4RT Client
	if err := setupConnection(ctx, t, streamParameter, clientA); err != nil {
		t.Fatalf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, clientA)

	if err := setupForwardingPipeline(ctx, t, streamParameter, clientA); err != nil {
		t.Fatalf("There is error sending SetForwardingPipeline, %s", err)
	}

	// Setup P4RT Client
	streamParameter.ElectionIdL -= 1
	if err := setupConnection(ctx, t, streamParameter, clientB); err != nil {
		t.Fatalf("There is error setting up connection, %s", err)
	}
	// Destroy P4RT Client
	defer teardownConnection(ctx, t, deviceID, clientB)

	streamParameter.ElectionIdL -= 1
	if err := setupConnection(ctx, t, streamParameter, clientA); err != nil {
		t.Fatalf("There is error setting up connection, %s", err)
	}

	resp, err := getForwardingPipeline(ctx, t, streamParameter, clientA, p4_v1.GetForwardingPipelineConfigRequest_ALL)
	if err != nil {
		t.Fatalf("There is error in GetForwardingPipelineConfig. %s", err)
	}

	wantP4Info, _ := utils.P4InfoLoad(p4InfoFile)
	gotP4Info := resp.Config.GetP4Info()
	if !cmp.Equal(wantP4Info, *gotP4Info) {
		t.Fatalf("There is error in GetForwardingPipelineConfig.")
	}
}

// GetForwardingPipelineConfig-Compliance:008
func testGetForwardingPipelineOnNonExistDeviceID(ctx context.Context, t *testing.T, args *testArgs) {
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

	streamParameter.DeviceId = ^deviceID
	_, err := getForwardingPipeline(ctx, t, streamParameter, client, p4_v1.GetForwardingPipelineConfigRequest_ALL)
	if err == nil {
		t.Fatalf("Expected error is not seen.")
	}
}

// GetForwardingPipelineConfig-Compliance:009
func testGetForwardingPipelineWithALL(ctx context.Context, t *testing.T, args *testArgs) {
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

	resp, err := getForwardingPipeline(ctx, t, streamParameter, client, p4_v1.GetForwardingPipelineConfigRequest_ALL)
	if err != nil {
		t.Fatalf("There is error in GetForwardingPipelineConfig. %s", err)
	}

	wantP4Info, _ := utils.P4InfoLoad(p4InfoFile)
	gotP4Info := resp.Config.GetP4Info()
	if !cmp.Equal(wantP4Info, *gotP4Info) {
		t.Fatalf("There is error in GetForwardingPipelineConfig.")
	}
}

// GetForwardingPipelineConfig-Compliance:010
func testGetForwardingPipelineWithCookieOnly(ctx context.Context, t *testing.T, args *testArgs) {
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

	resp, err := getForwardingPipeline(ctx, t, streamParameter, client, p4_v1.GetForwardingPipelineConfigRequest_COOKIE_ONLY)
	if err != nil {
		t.Fatalf("There is error in GetForwardingPipelineConfig. %s", err)
	}

	want := uint64(159)
	got := resp.Config.GetCookie()
	if !cmp.Equal(want, *got) {
		t.Fatalf("There is error in GetForwardingPipelineConfig.")
	}
}

// GetForwardingPipelineConfig-Compliance:011
func testGetForwardingPipelineWithP4InfoAndCookie(ctx context.Context, t *testing.T, args *testArgs) {
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

	resp, err := getForwardingPipeline(ctx, t, streamParameter, client, p4_v1.GetForwardingPipelineConfigRequest_P4INFO_AND_COOKIE)
	if err != nil {
		t.Fatalf("There is error in GetForwardingPipelineConfig. %s", err)
	}

	wantP4Info, _ := utils.P4InfoLoad(p4InfoFile)
	gotP4Info := resp.Config.GetP4Info()
	if !cmp.Equal(wantP4Info, *gotP4Info) {
		t.Fatalf("There is error in GetForwardingPipelineConfig.")
	}
}

// GetForwardingPipelineConfig-Compliance:012
func testGetForwardingPipelineWithDeviceConfigAndCookie(ctx context.Context, t *testing.T, args *testArgs) {
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

	resp, err := getForwardingPipeline(ctx, t, streamParameter, client, p4_v1.GetForwardingPipelineConfigRequest_DEVICE_CONFIG_AND_COOKIE)
	if err != nil {
		t.Fatalf("There is error in GetForwardingPipelineConfig. %s", err)
	}

	want := uint64(159)
	got := resp.Config.GetCookie()
	if !cmp.Equal(want, *got) {
		t.Fatalf("There is error in GetForwardingPipelineConfig.")
	}
}
