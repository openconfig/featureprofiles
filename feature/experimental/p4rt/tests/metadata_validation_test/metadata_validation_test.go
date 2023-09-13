package metadata_validation_test

import (
	"flag"
	"fmt"
	"strings"
	"testing"

	"github.com/cisco-open/go-p4/p4rt_client"
	"github.com/cisco-open/go-p4/utils"
	"github.com/openconfig/featureprofiles/feature/experimental/p4rt/internal/p4rtutils"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"

	p4v1pb "github.com/p4lang/p4runtime/go/p4/v1"
)

var (
	p4InfoFile = flag.String("p4info_file_location", "../../wbb.p4info.pb.txt", "Path to the p4info file.")
)

var (
	portID     = uint32(10)
	deviceID   = uint64(1)
	streamName = "p4rt"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestP4RTMetadata(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	configureDeviceID(t, dut)
	configurePortID(t, dut)

	c := p4rt_client.NewP4RTClient(&p4rt_client.P4RTClientParameters{})
	if err := c.P4rtClientSet(dut.RawAPIs().P4RT(t)); err != nil {
		t.Fatalf("Could not initialize p4rt client: %v", err)
	}

	if err := clientArbitration(c); err != nil {
		t.Fatalf("Error in P4RT Client Arbitration: %v", err)
	}

	if err := forwardingPipeline(c); err != nil {
		t.Fatalf("Error in P4RT Forwarding Pipeline config: %v", err)
	}

	metadata := "test metadata"

	t.Run("Insert and Read Table with Metadata", func(t *testing.T) {
		got, found, err := writeReadTableEntry(c, metadata, p4v1pb.Update_INSERT)
		if err != nil {
			t.Fatalf("Error in Write+Read Table Entry with Type %v: %v", p4v1pb.Update_INSERT, err)
		}
		if !found {
			t.Error("Write+Read TableEntry not found in metadata")
		}
		if got, want := got, metadata; got != want {
			t.Errorf("Incorrect metadata received in ReadTable, got: %q, want: %q", got, want)
		}
	})

	t.Run("Modify and Read Table with Metadata", func(t *testing.T) {
		if deviations.P4RTModifyTableEntryUnsupported(dut) {
			t.Skip("Skipping Modify Table Entry")
		}

		// Update Metadata to a different value.
		metadata := "test metadata modified"

		got, found, err := writeReadTableEntry(c, metadata, p4v1pb.Update_MODIFY)
		if err != nil {
			t.Fatalf("Error in Write+Read Table Entry with Type %v: %v", p4v1pb.Update_MODIFY, err)
		}
		if !found {
			t.Error("Write+Read TableEntry not found in metadata")
		}
		if got, want := got, metadata; got != want {
			t.Errorf("Incorrect metadata received in ReadTable, got: %q, want: %q", got, want)
		}
	})

	t.Run("Delete Table with Metadata", func(t *testing.T) {
		got, found, err := writeReadTableEntry(c, metadata, p4v1pb.Update_DELETE)
		if err != nil {
			t.Fatalf("Error in Write+Read Table Entry with Type %v: %v", p4v1pb.Update_DELETE, err)
		}
		if found {
			t.Errorf("Write+Read TableEntry found in metadata, got: %q, want: %q", got, "")
		}
	})
}

func writeReadTableEntry(c *p4rt_client.P4RTClient, metadata string, action p4v1pb.Update_Type) (string, bool, error) {
	if err := writeTableEntry(c, metadata, action); err != nil {
		return "", false, fmt.Errorf("error in writing Table Entry: %v", err)
	}
	resp, err := readTableEntry(c)
	if err != nil {
		return "", false, fmt.Errorf("error in reading Table Entry: %v", err)
	}

	for _, e := range resp.GetEntities() {
		if e.GetTableEntry() != nil {
			m := strings.Builder{}
			m.Write(e.GetTableEntry().GetMetadata())
			return m.String(), true, nil
		}
	}

	return "", false, nil
}

func clientArbitration(c *p4rt_client.P4RTClient) error {
	sp := p4rt_client.P4RTStreamParameters{
		Name:        streamName,
		DeviceId:    deviceID,
		ElectionIdH: 0,
		ElectionIdL: 1,
	}

	if err := c.StreamChannelCreate(&sp); err != nil {
		return fmt.Errorf("Error creating stream channel: %v", err)
	}
	if err := c.StreamChannelSendMsg(&streamName, &p4v1pb.StreamMessageRequest{
		Update: &p4v1pb.StreamMessageRequest_Arbitration{
			Arbitration: &p4v1pb.MasterArbitrationUpdate{
				DeviceId: sp.DeviceId,
				ElectionId: &p4v1pb.Uint128{
					High: sp.ElectionIdH,
					Low:  sp.ElectionIdL,
				},
			},
		},
	}); err != nil {
		return fmt.Errorf("Error sending ClientArbitration request: %v", err)
	}
	if _, _, arbErr := c.StreamChannelGetArbitrationResp(&streamName, 1); arbErr != nil {
		if err := p4rtutils.StreamTermErr(c.StreamTermErr); err != nil {
			return fmt.Errorf("Stream terminated on sending ClientArbitration: %v", err)
		}
		return fmt.Errorf("Error getting ClientArbitration response: %v", arbErr)
	}
	return nil
}

func forwardingPipeline(c *p4rt_client.P4RTClient) error {
	p4Info, err := utils.P4InfoLoad(p4InfoFile)
	if err != nil {
		return fmt.Errorf("Error loading P4 Info file: %v", err)
	}

	if err := c.SetForwardingPipelineConfig(&p4v1pb.SetForwardingPipelineConfigRequest{
		DeviceId:   deviceID,
		ElectionId: &p4v1pb.Uint128{High: 0, Low: 1},
		Action:     p4v1pb.SetForwardingPipelineConfigRequest_VERIFY_AND_COMMIT,
		Config: &p4v1pb.ForwardingPipelineConfig{
			P4Info: p4Info,
			Cookie: &p4v1pb.ForwardingPipelineConfig_Cookie{
				Cookie: 159,
			},
		},
	}); err != nil {
		return fmt.Errorf("Error in SetForwardingPipelineConfig: %v", err)
	}
	if _, err := c.GetForwardingPipelineConfig(&p4v1pb.GetForwardingPipelineConfigRequest{
		DeviceId:     deviceID,
		ResponseType: p4v1pb.GetForwardingPipelineConfigRequest_P4INFO_AND_COOKIE,
	}); err != nil {
		return fmt.Errorf("Error in GetForwardingPipelineConfig: %v", err)
	}
	return nil
}

func writeTableEntry(c *p4rt_client.P4RTClient, metadata string, action p4v1pb.Update_Type) error {
	te := []*p4rtutils.ACLWbbIngressTableEntryInfo{{
		Type:          action,
		EtherType:     0x6007,
		EtherTypeMask: 0xFFFF,
		Priority:      1,
		Metadata:      metadata,
	}}

	writeReq := &p4v1pb.WriteRequest{
		DeviceId:   deviceID,
		ElectionId: &p4v1pb.Uint128{High: 0, Low: 1},
		Updates:    p4rtutils.ACLWbbIngressTableEntryGet(te),
		Atomicity:  p4v1pb.WriteRequest_CONTINUE_ON_ERROR,
	}
	if err := c.Write(writeReq); err != nil {
		return fmt.Errorf("Error in Write Table: %v", err)
	}
	return nil
}

func readTableEntry(c *p4rt_client.P4RTClient) (*p4v1pb.ReadResponse, error) {
	readReq := &p4v1pb.ReadRequest{
		DeviceId: deviceID,
		Entities: []*p4v1pb.Entity{
			{
				Entity: &p4v1pb.Entity_TableEntry{
					TableEntry: &p4v1pb.TableEntry{
						TableId: p4rtutils.WbbTableMap["acl_wbb_ingress_table"],
					},
				},
			},
		},
	}
	rc, err := c.Read(readReq)
	if err != nil {
		return nil, fmt.Errorf("Error in Read Table: %v", err)
	}
	resp, err := rc.Recv()
	if err != nil {
		return nil, fmt.Errorf("Error in receiving ReadTable response: %v", err)
	}
	return resp, nil
}

// configureDeviceID configures p4rt device-id on the DUT.
func configureDeviceID(t *testing.T, dut *ondatra.DUTDevice) {
	nodes := p4rtutils.P4RTNodesByPort(t, dut)
	p4rtNode, ok := nodes["port1"]
	if !ok {
		t.Fatal("Couldn't find P4RT Node for port: port1")
	}
	t.Logf("Configuring P4RT Node: %s", p4rtNode)
	c := oc.Component{}
	c.Name = ygot.String(p4rtNode)
	c.IntegratedCircuit = &oc.Component_IntegratedCircuit{}
	c.IntegratedCircuit.NodeId = ygot.Uint64(deviceID)
	gnmi.Replace(t, dut, gnmi.OC().Component(p4rtNode).Config(), &c)
}

// configurePortID configures p4rt port-id and interface type on the DUT.
func configurePortID(t *testing.T, dut *ondatra.DUTDevice) {
	d := gnmi.OC()
	portName := dut.Port(t, "port1").Name()
	currIntf := &oc.Interface{
		Name: ygot.String(portName),
		Type: oc.IETFInterfaces_InterfaceType_ethernetCsmacd,
		Id:   &portID,
	}
	gnmi.Replace(t, dut, d.Interface(portName).Config(), currIntf)
}
