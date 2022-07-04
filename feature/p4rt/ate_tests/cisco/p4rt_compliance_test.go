package cisco_p4rt_test

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"
	"testing"
	"time"

	p4rt_client "github.com/cisco-open/go-p4/p4rt_client"
	"github.com/cisco-open/go-p4/utils"
	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/ygot/ygot"
	p4_v1 "github.com/p4lang/p4runtime/go/p4/v1"
	"google.golang.org/protobuf/testing/protocmp"
	"wwwin-github.cisco.com/rehaddad/go-wbb/p4info/wbb"
)

func configureDeviceID(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice) {
	resp := dut.Telemetry().ComponentAny().Get(t)
	component := telemetry.Component{}
	component.IntegratedCircuit = &telemetry.Component_IntegratedCircuit{}
	i := uint64(0)
	for _, c := range resp {
		name := c.GetName()
		if match, _ := regexp.MatchString(".*-NPU\\d+", name); match && !strings.Contains(name, "FC") {
			component.Name = ygot.String(name)
			component.IntegratedCircuit.NodeId = ygot.Uint64(deviceID + i)
			dut.Config().Component(name).Replace(t, &component)
			i += 1
		}
	}
}

func configurePortID(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice) {
	ports := fptest.SortPorts(dut.Ports())
	for index, port := range ports {
		// dut.Config().Interface(port.Name()).Id().Update(t, uint32(index)+portID)
		dut.Config().Interface(port.Name()).Update(t, &telemetry.Interface{
			Name: ygot.String(port.Name()),
			Id:   ygot.Uint32(uint32(index) + portID),
		})
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
	P4RTComplianceTestcases = append(P4RTComplianceTestcases, P4RTComplianceReadRPC...)

	for _, tt := range P4RTComplianceTestcases {
		// Each case will run with its own gRIBI fluent client.
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Name: %s", tt.name)
			t.Logf("Description: %s", tt.desc)

			tt.fn(ctx, t, args)

			time.Sleep(5 * time.Second)
		})
	}
}

func generateStreamParameter(DeviceId, electionIDH, electionIDL uint64) p4rt_client.P4RTStreamParameters {
	streamParameter := p4rt_client.P4RTStreamParameters{
		Name:        streamName,
		DeviceId:    deviceID,
		ElectionIdH: electionIDH,
		ElectionIdL: electionIDL,
	}
	return streamParameter
}

func setupConnection(ctx context.Context, t *testing.T, streamParameter p4rt_client.P4RTStreamParameters, client *p4rt_client.P4RTClient) error {
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
	_, _, arbErr := client.StreamChannelGetArbitrationResp(&streamParameter.Name, 1)

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

func setupForwardingPipeline(ctx context.Context, t *testing.T, streamParameter p4rt_client.P4RTStreamParameters, client *p4rt_client.P4RTClient) error {
	p4Info, err := utils.P4InfoLoad(p4InfoFile)
	if err != nil {
		t.Logf("There is error when loading p4info file")
		return err
	}

	if err := client.SetForwardingPipelineConfig(&p4_v1.SetForwardingPipelineConfigRequest{
		DeviceId:   streamParameter.DeviceId,
		ElectionId: &p4_v1.Uint128{High: streamParameter.ElectionIdH, Low: streamParameter.ElectionIdL},
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

func readProgrammedEntry(ctx context.Context, t *testing.T, device_id uint64, client *p4rt_client.P4RTClient) ([]*p4_v1.TableEntry, error) {
	stream, err := client.Read(&p4_v1.ReadRequest{
		DeviceId: device_id,
		Entities: []*p4_v1.Entity{
			&p4_v1.Entity{
				Entity: &p4_v1.Entity_TableEntry{},
			},
		},
	})
	if err != nil {
		t.Logf("There is error when Reading entries...%v", err)
		return nil, err
	}
	entities := []*p4_v1.TableEntry{}

	for {
		readResp, respErr := stream.Recv()
		if respErr == io.EOF {
			t.Logf("Response read done!")
			break
		} else if respErr != nil {
			t.Logf("There is error when Reading response...%v", respErr)
			return entities, respErr
		} else {
			for _, entry := range readResp.Entities {
				t.Logf("Read Response with entry: %v", entry)
				entities = append(entities, entry.GetTableEntry())
			}
		}
	}
	return entities, nil
}

func checkEntryExist(ctx context.Context, t *testing.T, want wbb.AclWbbIngressTableEntryInfo, response []*p4_v1.TableEntry) {
	t.Helper()

	// TODO: Need to fill the right verification logic
	found := false
	opts := []cmp.Option{
		protocmp.Transform(),
	}

	for _, r := range response {
		if cmp.Equal(r, want, opts...) {
			found = true
		}
	}
	if !found {
		t.Errorf("Entry is not found!")
	}
}
