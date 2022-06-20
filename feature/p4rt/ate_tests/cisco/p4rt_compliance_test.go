package cisco_p4rt_test

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"

	p4rt_client "github.com/cisco-open/go-p4/p4rt_client"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/ygot/ygot"
	p4_v1 "github.com/p4lang/p4runtime/go/p4/v1"
	"wwwin-github.cisco.com/rehaddad/go-p4/utils"
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

func configureDeviceID(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice) {
	res := dut.Telemetry().ComponentAny().Get(t)
	component := telemetry.Component{}
	component.IntegratedCircuit = &telemetry.Component_IntegratedCircuit{}
	i := uint64(0)
	for _, c := range res {
		name := c.GetName()
		if match, _ := regexp.MatchString(".*-NPU\\d+", name); match && !strings.Contains(name, "FC") {
			component.Name = ygot.String(name)
			component.IntegratedCircuit.NodeId = ygot.Uint64(deviceID + i)
			dut.Config().Component(name).Replace(t, &component)
			i += 1
		}
	}
}

func TestP4RTCompliance(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	// Dial gRIBI
	ctx := context.Background()

	// Configure the ATE
	// ate := ondatra.ATE(t, "ate")
	// top := configureATE(t, ate)
	// top.Push(t).StartProtocols(t)

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
		// ate:         ate,
		// top:         top,
		usecase:    0,
		interfaces: &interfaces,
	}

	configureDeviceID(ctx, t, dut)

	P4RTComplianceTestcases := []Testcase{}
	P4RTComplianceTestcases = append(P4RTComplianceTestcases, P4RTComplianceWriteRPC...)

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
