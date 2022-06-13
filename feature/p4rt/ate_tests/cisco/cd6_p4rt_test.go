package cisco_p4rt_test

import (
	"context"
	"testing"
	"time"

	"github.com/openconfig/ondatra"
	p4_v1 "github.com/p4lang/p4runtime/go/p4/v1"
	p4rt_client "wwwin-github.cisco.com/rehaddad/go-p4/p4rt_client"
	"wwwin-github.cisco.com/rehaddad/go-wbb/p4info/wbb"
)

func testTraffic(t *testing.T, ate *ondatra.ATEDevice, srcEndPoint *ondatra.Interface, duration int, args *testArgs) {
	ethHeader := ondatra.NewEthernetHeader()
	ethHeader.WithSrcAddress("00:11:01:00:00:01")
	ethHeader.WithDstAddress("00:22:01:00:00:01")
	ethHeader.WithEthernetType("0x6007")

	flow := ate.Traffic().NewFlow("GDP").
		WithSrcEndpoints(srcEndPoint).
		WithDstEndpoints(srcEndPoint)

	flow.WithFrameSize(300).WithFrameRateFPS(2).WithHeaders(ethHeader)

	ate.Traffic().Start(t, flow)
	time.Sleep(time.Duration(duration) * time.Second)

	ate.Traffic().Stop(t)
}

// programmGDPMatchEntry programms or deletes GDP entry
func programmGDPMatchEntry(ctx context.Context, t *testing.T, client *p4rt_client.P4RTClient, delete bool) error {
	actionType := p4_v1.Update_INSERT
	if delete {
		actionType = p4_v1.Update_DELETE
	}
	err := client.Write(&p4_v1.WriteRequest{
		DeviceId:   uint64(1),
		ElectionId: &p4_v1.Uint128{High: uint64(0), Low: uint64(100)},
		Updates: wbb.AclWbbIngressTableEntryGet([]*wbb.AclWbbIngressTableEntryInfo{
			&wbb.AclWbbIngressTableEntryInfo{
				Type:          actionType,
				EtherType:     0x6007,
				EtherTypeMask: 0xFFFF,
			},
		}),
		Atomicity: p4_v1.WriteRequest_CONTINUE_ON_ERROR,
	})
	if err != nil {
		return err
	}
	return nil
}

func testGDPEntryProgramming(ctx context.Context, t *testing.T, args *testArgs) {
	client := args.p4rtClientA
	// Program the GDP entry
	if err := programmGDPMatchEntry(ctx, t, client, false); err != nil {
		t.Errorf("There is error when inserting the GDP entry")
	}
	defer programmGDPMatchEntry(ctx, t, client, true)

	// Send GDP Packet
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	testTraffic(t, args.ate, srcEndPoint, 10, args)

	// Check PacketIn on P4Client
	_, packets, err := client.StreamChannelGetPacket(&streamName, 0)
	if err != nil {
		t.Errorf("There is error when checking packets in PacketIn ")
	}

	if packets != nil {
		t.Logf("PacketIn: %s", packets.Pkt.String())
	} else {
		t.Logf("There is no packets received")
	}

	//TODO check detail packets
}
