package cisco_p4rt_test

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	p4rt_client "github.com/cisco-open/go-p4/p4rt_client"
	"github.com/cisco-open/go-p4/utils"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	wbb "github.com/openconfig/featureprofiles/feature/experimental/p4rt/internal/p4rtutils"
	ciscoFlags "github.com/openconfig/featureprofiles/internal/cisco/flags"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
	p4_v1 "github.com/p4lang/p4runtime/go/p4/v1"
	"google.golang.org/protobuf/testing/protocmp"
)

var (
	identifiedNPUs = 0
)

func getComponentID(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice) string {
	resp := gnmi.GetAll(t, dut, gnmi.OC().ComponentAny().State())
	component := oc.Component{}
	component.IntegratedCircuit = &oc.Component_IntegratedCircuit{}
	names := []string{}
	pattern, _ := regexp.Compile(`.*-NPU\d+`)
	for _, c := range resp {
		name := c.GetName()
		if match := pattern.MatchString(name); match && !strings.Contains(name, "FC") {
			names = append(names, name)
		}
	}
	sort.Slice(names, func(i, j int) bool {
		return names[i] < names[j]
	})
	return names[0]
}

func configureDeviceID(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice) {
	resp := gnmi.GetAll(t, dut, gnmi.OC().ComponentAny().State())
	component := oc.Component{}
	component.IntegratedCircuit = &oc.Component_IntegratedCircuit{}
	pattern, _ := regexp.Compile(`.*-NPU\d+$`)

	i := uint64(0)
	for _, c := range resp {
		name := c.GetName()
		match := false
		for _, lc := range exclude_LC {
			if strings.Contains(name, lc) {
				match = true
				break
			}
		}
		if match {
			continue
		}
		if match := pattern.MatchString(name); match && !strings.Contains(name, "FC") {
			component.Name = ygot.String(name)
			component.IntegratedCircuit.NodeId = ygot.Uint64(deviceID + i)
			gnmi.Update(t, dut, gnmi.OC().Component(name).Config(), &component)
			i += 1
			identifiedNPUs += 1
		}
	}
}

func configurePortID(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice) {
	ports := sortPorts(dut.Ports())
	for index, port := range ports {
		// dut.Config().Interface(port.Name()).Id().Update(t, uint32(index)+portID)
		conf := &oc.Interface{
			Name: ygot.String(port.Name()),
			Id:   ygot.Uint32(uint32(index) + portID),
		}
		if strings.Contains(port.Name(), "Bundle") {
			conf.Type = oc.IETFInterfaces_InterfaceType_ieee8023adLag
		} else {
			conf.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
		}
		gnmi.Update(t, dut, gnmi.OC().Interface(port.Name()).Config(), conf)
		// dut.Config().Interface(port.Name()).Update(t, conf)
	}
}

func programmGDPMatchEntry(ctx context.Context, t *testing.T, client *p4rt_client.P4RTClient, delete bool) error {
	psp := p4rt_client.P4RTStreamParameters{
		DeviceId:    deviceID,
		ElectionIdH: uint64(0),
		ElectionIdL: electionID,
	}
	return programmGDPMatchEntryWithStreamParameter(ctx, t, psp, client, delete)
}

// programmGDPMatchEntryWithStreamParameter programms or deletes GDP entry
func programmGDPMatchEntryWithStreamParameter(ctx context.Context, t *testing.T, streamParameter p4rt_client.P4RTStreamParameters, client *p4rt_client.P4RTClient, delete bool) error {
	actionType := p4_v1.Update_INSERT
	if delete {
		actionType = p4_v1.Update_DELETE
	}
	err := client.Write(&p4_v1.WriteRequest{
		DeviceId:   streamParameter.DeviceId,
		ElectionId: &p4_v1.Uint128{High: streamParameter.ElectionIdH, Low: streamParameter.ElectionIdL},
		Updates: wbb.ACLWbbIngressTableEntryGet([]*wbb.ACLWbbIngressTableEntryInfo{
			{
				Type:          actionType,
				EtherType:     0x6007,
				EtherTypeMask: 0xFFFF,
				Priority:      1,
			},
		}),
		Atomicity: p4_v1.WriteRequest_CONTINUE_ON_ERROR,
	})
	if err != nil {
		return err
	}
	return nil
}

func TestP4RTCompliance(t *testing.T) {
	if !*ciscoFlags.ComplianceTests {
		t.Skip()
	}
	dut := ondatra.DUT(t, "dut")

	// Dial gRIBI
	ctx := context.Background()

	// Configure the ATE
	// ate := ondatra.ATE(t, "ate")
	// top := configureATE(t, ate)
	// top.Push(t).StartProtocols(t)

	p4rtClientA := p4rt_client.P4RTClient{}
	if err := p4rtClientA.P4rtClientSet(dut.RawAPIs().P4RT().Default(t)); err != nil {
		t.Fatalf("Could not initialize p4rt client: %v", err)
	}

	p4rtClientB := p4rt_client.P4RTClient{}
	if err := p4rtClientB.P4rtClientSet(dut.RawAPIs().P4RT().Default(t)); err != nil {
		t.Fatalf("Could not initialize p4rt client: %v", err)
	}

	p4rtClientC := p4rt_client.P4RTClient{}
	if err := p4rtClientC.P4rtClientSet(dut.RawAPIs().P4RT().Default(t)); err != nil {
		t.Fatalf("Could not initialize p4rt client: %v", err)
	}

	p4rtClientD := p4rt_client.P4RTClient{}
	if err := p4rtClientD.P4rtClientSet(dut.RawAPIs().P4RT().Default(t)); err != nil {
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
	configurePortID(ctx, t, dut)

	P4RTComplianceTestcases := []Testcase{}
	P4RTComplianceTestcases = append(P4RTComplianceTestcases, P4RTComplianceWriteRPC...)
	P4RTComplianceTestcases = append(P4RTComplianceTestcases, P4RTComplianceReadRPC...)
	P4RTComplianceTestcases = append(P4RTComplianceTestcases, P4RTComplianceClientArbitration...)
	P4RTComplianceTestcases = append(P4RTComplianceTestcases, P4RTComplianceSetForwardingPipelineConfig...)
	P4RTComplianceTestcases = append(P4RTComplianceTestcases, P4RTComplianceGetForwardingPipelineConfig...)

	for _, tt := range P4RTComplianceTestcases {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Name: %s", tt.name)
			t.Logf("Description: %s", tt.desc)
			if tt.skip {
				t.Skip("testcase marked for skip")
			}

			tt.fn(ctx, t, args)

			time.Sleep(5 * time.Second)
		})
	}
}

func generateStreamParameter(device_ID, election_ID_High, election_ID_Low uint64) p4rt_client.P4RTStreamParameters {
	streamParameter := p4rt_client.P4RTStreamParameters{
		Name:        streamName,
		DeviceId:    device_ID,
		ElectionIdH: election_ID_High,
		ElectionIdL: election_ID_Low,
	}
	return streamParameter
}

func setupConnection(ctx context.Context, t *testing.T, streamParameter p4rt_client.P4RTStreamParameters, client *p4rt_client.P4RTClient) error {
	client.StreamChannelCreate(&streamParameter)
	if err := client.StreamChannelSendMsg(&streamParameter.Name, &p4_v1.StreamMessageRequest{
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

func getForwardingPipeline(ctx context.Context, t *testing.T, streamParameter p4rt_client.P4RTStreamParameters, client *p4rt_client.P4RTClient, responseType p4_v1.GetForwardingPipelineConfigRequest_ResponseType) (*p4_v1.GetForwardingPipelineConfigResponse, error) {
	// Get Forwarding pipeline (for now, we just log it)
	resp, err := client.GetForwardingPipelineConfig(&p4_v1.GetForwardingPipelineConfigRequest{
		DeviceId:     streamParameter.DeviceId,
		ResponseType: responseType,
	})
	if err != nil {
		t.Logf("There is error in case of GetForwardingPipeline ...%s", err)
		return nil, err
	}
	return resp, nil
}

func readProgrammedEntry(ctx context.Context, t *testing.T, device_id uint64, client *p4rt_client.P4RTClient) ([]*p4_v1.TableEntry, error) {
	stream, err := client.Read(&p4_v1.ReadRequest{
		DeviceId: device_id,
		Entities: []*p4_v1.Entity{
			{
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
			break
		} else if respErr != nil {
			t.Logf("There is error when Reading response...%v", respErr)
			return entities, respErr
		} else {
			for _, entry := range readResp.Entities {
				// t.Logf("Read Response with entry: %v", entry)
				entities = append(entities, entry.GetTableEntry())
			}
		}
	}
	return entities, nil
}

func checkEntryExist(ctx context.Context, t *testing.T, want *p4_v1.TableEntry, response []*p4_v1.TableEntry) {
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

func compareEntry(ctx context.Context, t *testing.T, want, got *p4_v1.TableEntry) bool {
	ignoreFields := []string{"Match", "CounterData"}
	opts := []cmp.Option{
		cmpopts.IgnoreFields(p4_v1.TableEntry{}, ignoreFields...),
	}
	if match := cmp.Equal(*want, *got, opts...); !match {
		return false
	}
	found := false
	for _, gotEntry := range got.GetMatch() {
		for _, wantEntry := range want.GetMatch() {
			if cmp.Equal(*gotEntry, *wantEntry) {
				found = true
			}
		}
		if found {
			break
		}
	}
	return found
}
