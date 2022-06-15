package cisco_p4rt_test

import (
	"context"
	"fmt"
	"testing"

	p4rt_client "github.com/cisco-open/go-p4/p4rt_client"
	"github.com/openconfig/ondatra"
	p4_v1 "github.com/p4lang/p4runtime/go/p4/v1"
	"wwwin-github.cisco.com/rehaddad/go-p4/utils"
)

var (
	P4RTComplianceTestcases = []Testcase{
		{
			name: "Insert existing entry in Write RPC",
			desc: "Write RPC-Compliance:008 12(INSERT) Write RPC with Insert exisint entity, verify ALREADY_EXISTS error returned and existing entity remain unchanged",
			fn:   testWriteRPCInsertSameEntry,
		},
		// {
		// 	name: "Insert existing entry in Write RPC",
		// 	desc: "Write RPC-Compliance:008 12(INSERT) Write RPC with Insert exisint entity, verify ALREADY_EXISTS error returned and existing entity remain unchanged",
		// 	fn:   testWriteRPCInsertSameEntry,
		// },
	}
)

func TestP4RTCompliance(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	// Dial gRIBI
	ctx := context.Background()

	// Configure the ATE
	ate := ondatra.ATE(t, "ate")
	top := configureATE(t, ate)
	top.Push(t).StartProtocols(t)

	p4rtClientA := p4rt_client.P4RTClient{}
	if err := p4rtClientA.P4rtClientSet(dut.RawAPIs().P4RT(t)); err != nil {
		t.Fatalf("Could not initialize p4rt client: %v", err)
	}

	p4rtClientB := p4rt_client.P4RTClient{}
	if err := p4rtClientB.P4rtClientSet(dut.RawAPIs().P4RT(t)); err != nil {
		t.Fatalf("Could not initialize p4rt client: %v", err)
	}

	p4rtClientC := p4rt_client.P4RTClient{}
	if err := p4rtClientC.P4rtClientSet(dut.RawAPIs().P4RT(t)); err != nil {
		t.Fatalf("Could not initialize p4rt client: %v", err)
	}

	p4rtClientD := p4rt_client.P4RTClient{}
	if err := p4rtClientD.P4rtClientSet(dut.RawAPIs().P4RT(t)); err != nil {
		t.Fatalf("Could not initialize p4rt client: %v", err)
	}

	interfaceList := []string{}
	for i := 121; i < 128; i++ {
		interfaceList = append(interfaceList, fmt.Sprintf("Bundle-Ether%d", i))
	}

	interfaces := interfaces{
		in:  []string{"Bundle-Ether120"},
		out: interfaceList,
	}

	args := &testArgs{
		ctx:         ctx,
		p4rtClientA: &p4rtClientA,
		p4rtClientB: &p4rtClientB,
		p4rtClientC: &p4rtClientC,
		p4rtClientD: &p4rtClientD,
		dut:         dut,
		ate:         ate,
		top:         top,
		usecase:     0,
		interfaces:  &interfaces,
	}

	for _, tt := range P4RTComplianceTestcases {
		// Each case will run with its own gRIBI fluent client.
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Name: %s", tt.name)
			t.Logf("Description: %s", tt.desc)

			tt.fn(ctx, t, args)
		})
	}
}

func setupConnection(ctx context.Context, t *testing.T, deviceID uint64, client *p4rt_client.P4RTClient) error {
	streamParameter := p4rt_client.P4RTStreamParameters{
		Name:        streamName,
		DeviceId:    deviceID,
		ElectionIdH: uint64(0),
		ElectionIdL: electionID,
	}
	client.StreamChannelCreate(&streamParameter)
	if err := client.StreamChannelSendMsg(&streamName, &p4_v1.StreamMessageRequest{
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
		t.Logf("There is error when setting up p4rtClientA")
		return err
	}
	_, _, arbErr := client.StreamChannelGetArbitrationResp(&streamName, 1)

	if arbErr != nil {
		t.Logf("There is error at Arbitration time: %v", arbErr)
		return arbErr
	}
	return nil
}

func teardownConnection(ctx context.Context, t *testing.T, deviceID uint64, client *p4rt_client.P4RTClient) error {
	if err := client.StreamChannelDestroy(&streamName); err != nil {
		return err
	}
	return nil
}

func setupForwardingPipeline(ctx context.Context, t *testing.T, deviceID uint64, client *p4rt_client.P4RTClient) error {
	p4Info, err := utils.P4InfoLoad(p4InfoFile)
	if err != nil {
		t.Logf("There is error when loading p4info file")
		return err
	}

	if err := client.SetForwardingPipelineConfig(&p4_v1.SetForwardingPipelineConfigRequest{
		DeviceId:   deviceID,
		ElectionId: &p4_v1.Uint128{High: uint64(0), Low: electionID},
		Action:     p4_v1.SetForwardingPipelineConfigRequest_VERIFY_AND_COMMIT,
		Config: &p4_v1.ForwardingPipelineConfig{
			P4Info: &p4Info,
			Cookie: &p4_v1.ForwardingPipelineConfig_Cookie{
				Cookie: 159,
			},
		},
	}); err != nil {
		return err
	}

	return nil
}

func testWriteRPCInsertSameEntry(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA
	deviceID := uint64(1)
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
