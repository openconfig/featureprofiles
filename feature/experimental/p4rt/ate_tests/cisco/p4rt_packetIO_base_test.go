package cisco_p4rt_test

import (
	"context"
	"encoding/binary"
	"flag"
	"fmt"
	"sort"
	"strings"
	"testing"

	p4rt_client "github.com/cisco-open/go-p4/p4rt_client"
	"github.com/cisco-open/go-p4/utils"
	"github.com/openconfig/featureprofiles/internal/attrs"
	ciscoFlags "github.com/openconfig/featureprofiles/internal/cisco/flags"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ygot/ygot"
	p4_v1 "github.com/p4lang/p4runtime/go/p4/v1"
)

var (
	p4InfoFile = flag.String("p4info_file_location", "./wbb.p4info.pb.txt",
		"Path to the p4info file.")
	electionID            = uint64(100)
	streamName            = "Primary"
	deviceID              = uint64(1)
	portID                = uint32(10)
	gdpMAC                = "00:0a:da:f0:f0:f0"
	gdpEtherType          = uint32(24583)
	lldpMAC               = "01:80:c2:00:00:0e"
	lldpEtherType         = uint32(35020)
	METADATA_INGRESS_PORT = uint32(1)
	METADATA_EGRESS_PORT  = uint32(2)
	SUBMIT_TO_INGRESS     = uint32(1)
	SUBMIT_TO_EGRESS      = uint32(0)
	forusIP               = "10.10.10.10"
	maxPortID             = uint32(0xFFFFFEFF)
)

// Testcase defines testcase structure
type Testcase struct {
	name string
	desc string
	fn   func(ctx context.Context, t *testing.T, args *testArgs)
	skip bool
}

// testArgs holds the objects needed by a test case.
type testArgs struct {
	ctx         context.Context
	p4rtClientA *p4rt_client.P4RTClient
	p4rtClientB *p4rt_client.P4RTClient
	p4rtClientC *p4rt_client.P4RTClient
	p4rtClientD *p4rt_client.P4RTClient
	npus        []string
	dut         *ondatra.DUTDevice
	ate         *ondatra.ATEDevice
	top         *ondatra.ATETopology
	interfaces  *interfaces
	usecase     int
	prefix      *gribiPrefix
	packetIO    PacketIO
}

type interfaces struct {
	in  []string
	out []string
}

type gribiPrefix struct {
	scale int

	host string

	vrfName         string
	vipPrefixLength string

	vip1Ip string
	vip2Ip string

	vip1NhIndex  uint64
	vip1NhgIndex uint64

	vip2NhIndex  uint64
	vip2NhgIndex uint64

	vrfNhIndex  uint64
	vrfNhgIndex uint64
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func sortPorts(ports []*ondatra.Port) []*ondatra.Port {
	sort.Slice(ports, func(i, j int) bool {
		idi, idj := ports[i].ID(), ports[j].ID()
		li, lj := len(idi), len(idj)
		if li == lj {
			return idi < idj
		}
		return li < lj // "port2" < "port10"
	})
	return ports
}

func getGDPParameter(t *testing.T) PacketIO {
	return &GDPPacketIO{
		PacketIOPacket: PacketIOPacket{
			SrcMAC:       ygot.String("00:01:00:02:00:00"),
			DstMAC:       &gdpMAC,
			EthernetType: &gdpEtherType,
		},
		IngressPort: fmt.Sprint(portID),
	}
}

func getLLDPParameter(t *testing.T) PacketIO {
	return &LLDPPacketIO{
		PacketIOPacket: PacketIOPacket{
			SrcMAC:       ygot.String("00:01:00:02:00:00"),
			DstMAC:       &lldpMAC,
			EthernetType: &lldpEtherType,
		},
		NeedConfig:  ygot.Bool(false),
		IngressPort: fmt.Sprint(portID),
	}
}

func getTTLParameter(t *testing.T, ipv4, ipv6, ttlTwo bool) PacketIO {
	return &TTLPacketIO{
		PacketIOPacket: PacketIOPacket{
			SrcMAC:  ygot.String("00:01:00:01:00:01"),
			DstMAC:  ygot.String("00:01:00:02:00:03"),
			SrcIPv4: ygot.String("100.120.1.1"),
			DstIPv4: ygot.String("100.121.1.2"),
			SrcIPv6: ygot.String("100:120:1::1"),
			DstIPv6: ygot.String("100:121:1::2"),
			TTL:     ygot.Uint32(1),
		},
		NeedConfig:  ygot.Bool(false),
		IPv4:        ipv4,
		IPv6:        ipv6,
		TtlTwo:      ttlTwo,
		IngressPort: fmt.Sprint(portID),
		EgressPorts: []string{fmt.Sprint(portID + 1)},
		PacketOutObj: &PacketIOPacket{
			SrcIPv4: ygot.String("100.120.1.2"),
			DstIPv4: ygot.String("100.121.1.1"),
			SrcIPv6: ygot.String("100:120:1::2"),
			DstIPv6: ygot.String("100:121:1::1"),
			TTL:     ygot.Uint32(64),
		},
	}
}

func TestP4RTPacketIO(t *testing.T) {
	if !*ciscoFlags.PacketIOTests {
		t.Skip()
	}
	dut := ondatra.DUT(t, "dut")
	configureDUT(t, dut)

	// Dial gRIBI
	ctx := context.Background()

	// Configure the ATE
	ate := ondatra.ATE(t, "ate")
	top := configureATE(t, ate)
	top.Push(t).StartProtocols(t)

	configureDeviceID(ctx, t, dut)
	configurePortID(ctx, t, dut)

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
		ate:         ate,
		top:         top,
		usecase:     0,
		interfaces:  &interfaces,
		prefix: &gribiPrefix{
			scale:           1,
			host:            "11.11.11.0",
			vrfName:         "TE",
			vipPrefixLength: "32",

			vip1Ip: "192.0.2.40",
			vip2Ip: "192.0.2.42",

			vip1NhIndex:  uint64(100),
			vip1NhgIndex: uint64(100),

			vip2NhIndex:  uint64(200),
			vip2NhgIndex: uint64(200),

			vrfNhIndex:  uint64(1000),
			vrfNhgIndex: uint64(1000),
		},
	}

	if err := setupP4RTClient(ctx, t, args); err != nil {
		t.Fatalf("Could not setup p4rt client: %v", err)
	}

	if *ciscoFlags.GDPTests {
		args.packetIO = getGDPParameter(t)

		for _, tt := range PublicGDPTestcases {
			t.Run(tt.name, func(t *testing.T) {
				t.Logf("Name: %s", tt.name)
				t.Logf("Description: %s", tt.desc)
				tt.fn(ctx, t, args)
			})
		}

		for _, tt := range OODGDPTestcases {
			// Each case will run with its own gRIBI fluent client.
			t.Run(tt.name, func(t *testing.T) {
				t.Logf("Name: %s", tt.name)
				t.Logf("Description: %s", tt.desc)
				tt.fn(ctx, t, args)
			})
		}
	}
	if *ciscoFlags.LLDPTests {
		args.packetIO = getLLDPParameter(t)

		for _, tt := range PublicLLDPDisableTestcases {
			t.Run(tt.name, func(t *testing.T) {
				t.Logf("Name: %s", tt.name)
				t.Logf("Description: %s", tt.desc)
				tt.fn(ctx, t, args)
			})
		}

		for _, tt := range OODLLDPDisabledTestcases {
			// Each case will run with its own gRIBI fluent client.
			t.Run(tt.name, func(t *testing.T) {
				t.Logf("Name: %s", tt.name)
				t.Logf("Description: %s", tt.desc)
				tt.fn(ctx, t, args)
			})
		}

		for _, tt := range LLDPEndabledTestcases {
			gnmi.Update(t, dut, gnmi.OC().Lldp().Enabled().Config(), *ygot.Bool(true))
			// Each case will run with its own gRIBI fluent client.
			t.Run(tt.name, func(t *testing.T) {
				t.Logf("Name: %s", tt.name)
				t.Logf("Description: %s", tt.desc)

				tt.fn(ctx, t, args)
			})
			gnmi.Update(t, dut, gnmi.OC().Lldp().Enabled().Config(), *ygot.Bool(false))
		}
	}
	if *ciscoFlags.TTLTests {
		ttlTestcases := map[string]func(){}
		if *ciscoFlags.TTL1v4 {
			ttlTestcases["IPv4 TTL1 Only"] = func() { args.packetIO = getTTLParameter(t, true, false, false) }
		}
		if *ciscoFlags.TTL1n2v4 {
			ttlTestcases["IPv4 TTL1 and TTL2"] = func() { args.packetIO = getTTLParameter(t, true, false, true) }
		}
		if *ciscoFlags.TTL1v6 {
			ttlTestcases["IPv6 TTL1 Only"] = func() { args.packetIO = getTTLParameter(t, false, true, false) }
		}
		if *ciscoFlags.TTL1n2v6 {
			ttlTestcases["IPv6 TTL1 and TTL2"] = func() { args.packetIO = getTTLParameter(t, false, true, true) }
		}
		if *ciscoFlags.TTL1v4n6 {
			ttlTestcases["IPv4 TTL1 and IPv6 TTL1"] = func() { args.packetIO = getTTLParameter(t, true, true, false) }
		}
		if *ciscoFlags.TTL1n2v4n6 {
			ttlTestcases["IPv4 TTL1 and TTL2 and IPv6 TTL1 and TTL2"] =
				func() {
					args.packetIO = getTTLParameter(t, true, true, true)
				}
		}

		for key, val := range ttlTestcases {
			val()
			for _, tt := range OODTTLTestcases {
				// Each case will run with its own gRIBI fluent client.
				t.Run(key+" "+tt.name, func(t *testing.T) {
					t.Logf("Name: %s %s", key, tt.name)
					t.Logf("Description: %s %s", key, tt.desc)

					tt.fn(ctx, t, args)
				})
			}
		}
	}
}

func setupP4RTClient(ctx context.Context, t *testing.T, args *testArgs) error {
	// Configure device-id and port-id
	deviceID := uint64(1)

	// Setup P4RT ClientA
	streamParameter := p4rt_client.P4RTStreamParameters{
		Name:        streamName,
		DeviceId:    deviceID,
		ElectionIdH: uint64(0),
		ElectionIdL: electionID,
	}

	// Send client arbitration message
	clients := []*p4rt_client.P4RTClient{args.p4rtClientA, args.p4rtClientB, args.p4rtClientC, args.p4rtClientD}
	for index, client := range clients {
		if client != nil {
			client.StreamChannelCreate(&streamParameter)
			if err := client.StreamChannelSendMsg(&streamName, &p4_v1.StreamMessageRequest{
				Update: &p4_v1.StreamMessageRequest_Arbitration{
					Arbitration: &p4_v1.MasterArbitrationUpdate{
						DeviceId: streamParameter.DeviceId,
						ElectionId: &p4_v1.Uint128{
							High: streamParameter.ElectionIdH,
							Low:  streamParameter.ElectionIdL - uint64(index),
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
		}
	}

	p4Info, err := utils.P4InfoLoad(p4InfoFile)
	if err != nil {
		t.Logf("There is error when loading p4info file")
		return err
	}

	// SetForwardingPipeline for p4rtClientA which is Primary Client
	err = args.p4rtClientA.SetForwardingPipelineConfig(&p4_v1.SetForwardingPipelineConfigRequest{
		DeviceId:   deviceID,
		ElectionId: &p4_v1.Uint128{High: uint64(0), Low: electionID},
		Action:     p4_v1.SetForwardingPipelineConfigRequest_VERIFY_AND_COMMIT,
		Config: &p4_v1.ForwardingPipelineConfig{
			P4Info: &p4Info,
			Cookie: &p4_v1.ForwardingPipelineConfig_Cookie{
				Cookie: 159,
			},
		},
	})
	if err != nil {
		t.Logf("There is error seen when setting SetForwardingPipelineConfig")
		return err
	}
	return nil
}

func setupPrimaryP4RTClient(ctx context.Context, t *testing.T, client *p4rt_client.P4RTClient, deviceID, electionID uint64, streamName string) error {
	return p4rtClientSetup(ctx, t, client, deviceID, electionID, streamName, true)
}

func setupBackupP4RTClient(ctx context.Context, t *testing.T, client *p4rt_client.P4RTClient, deviceID, electionID uint64, streamName string) error {
	return p4rtClientSetup(ctx, t, client, deviceID, electionID, streamName, false)
}

func p4rtClientSetup(ctx context.Context, t *testing.T, client *p4rt_client.P4RTClient, deviceID, electionID uint64, streamName string, primary bool) error {
	// Setup P4RT ClientA
	streamParameter := p4rt_client.P4RTStreamParameters{
		Name:        streamName,
		DeviceId:    deviceID,
		ElectionIdH: uint64(0),
		ElectionIdL: electionID,
	}

	// Send client arbitration message
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

	if primary {
		p4Info, err := utils.P4InfoLoad(p4InfoFile)
		if err != nil {
			t.Logf("There is error when loading p4info file")
			return err
		}

		// SetForwardingPipeline for p4rtClientA which is Primary Client
		err = client.SetForwardingPipelineConfig(&p4_v1.SetForwardingPipelineConfigRequest{
			DeviceId:   deviceID,
			ElectionId: &p4_v1.Uint128{High: uint64(0), Low: electionID},
			Action:     p4_v1.SetForwardingPipelineConfigRequest_VERIFY_AND_COMMIT,
			Config: &p4_v1.ForwardingPipelineConfig{
				P4Info: &p4Info,
				Cookie: &p4_v1.ForwardingPipelineConfig_Cookie{
					Cookie: 159,
				},
			},
		})
		if err != nil {
			t.Logf("There is error seen when setting SetForwardingPipelineConfig")
			return err
		}
	}
	return nil
}

const (
	ipv4PrefixLen = 24
	ipv6PrefixLen = 126
	instance      = "default"
	vlanMTU       = 1518
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "dutPort1",
		IPv4:    "100.120.1.1",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "100:120:1::1",
		IPv6Len: ipv6PrefixLen,
		MAC:     "00:01:00:02:00:03",
	}

	atePort1 = attrs.Attributes{
		Name:    "atePort1",
		IPv4:    "100.120.1.2",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "100:120:1::2",
		IPv6Len: ipv6PrefixLen,
	}

	dutPort2 = attrs.Attributes{
		Desc:    "dutPort2",
		IPv4:    "100.121.1.1",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "100:121:1::1",
		IPv6Len: ipv6PrefixLen,
	}

	atePort2 = attrs.Attributes{
		Name:    "atePort2",
		IPv4:    "100.121.1.2",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "100:121:1::2",
		IPv6Len: ipv6PrefixLen,
	}

	dutPort3 = attrs.Attributes{
		Desc:    "dutPort3",
		IPv4:    "100.122.1.1",
		IPv4Len: ipv4PrefixLen,
	}

	atePort3 = attrs.Attributes{
		Name:    "atePort3",
		IPv4:    "100.122.1.2",
		IPv4Len: ipv4PrefixLen,
	}

	dutPort4 = attrs.Attributes{
		Desc:    "dutPort4",
		IPv4:    "100.123.1.1",
		IPv4Len: ipv4PrefixLen,
	}

	atePort4 = attrs.Attributes{
		Name:    "atePort4",
		IPv4:    "100.123.1.2",
		IPv4Len: ipv4PrefixLen,
	}
	dutPort5 = attrs.Attributes{
		Desc:    "dutPort5",
		IPv4:    "100.124.1.1",
		IPv4Len: ipv4PrefixLen,
	}

	atePort5 = attrs.Attributes{
		Name:    "atePort5",
		IPv4:    "100.124.1.2",
		IPv4Len: ipv4PrefixLen,
	}
	dutPort6 = attrs.Attributes{
		Desc:    "dutPort6",
		IPv4:    "100.125.1.1",
		IPv4Len: ipv4PrefixLen,
	}

	atePort6 = attrs.Attributes{
		Name:    "atePort6",
		IPv4:    "100.125.1.2",
		IPv4Len: ipv4PrefixLen,
	}
	dutPort7 = attrs.Attributes{
		Desc:    "dutPort7",
		IPv4:    "100.126.1.1",
		IPv4Len: ipv4PrefixLen,
	}

	atePort7 = attrs.Attributes{
		Name:    "atePort7",
		IPv4:    "100.126.1.2",
		IPv4Len: ipv4PrefixLen,
	}
	dutPort8 = attrs.Attributes{
		Desc:    "dutPort8",
		IPv4:    "100.127.1.1",
		IPv4Len: ipv4PrefixLen,
	}
	atePort8 = attrs.Attributes{
		Name:    "atePort8",
		IPv4:    "100.127.1.2",
		IPv4Len: ipv4PrefixLen,
	}

	dutPort2Vlan10 = attrs.Attributes{
		Desc:    "dutPort2Vlan10",
		IPv4:    "100.121.10.1",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2000::100:121:10:1",
		IPv6Len: ipv6PrefixLen,
		MTU:     vlanMTU,
	}

	atePort2Vlan10 = attrs.Attributes{
		Name:    "atePort2Vlan10",
		IPv4:    "100.121.10.2",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2000::100:121:10:2",
		IPv6Len: ipv6PrefixLen,
		MTU:     vlanMTU,
	}

	dutPort2Vlan20 = attrs.Attributes{
		Desc:    "dutPort2Vlan20",
		IPv4:    "100.121.20.1",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2000::100:121:20:1",
		IPv6Len: ipv6PrefixLen,
		MTU:     vlanMTU,
	}

	atePort2Vlan20 = attrs.Attributes{
		Name:    "atePort2Vlan20",
		IPv4:    "100.121.20.2",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2000::100:121:20:2",
		IPv6Len: ipv6PrefixLen,
		MTU:     vlanMTU,
	}

	dutPort2Vlan30 = attrs.Attributes{
		Desc:    "dutPort2Vlan30",
		IPv4:    "100.121.30.1",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2000::100:121:30:1",
		IPv6Len: ipv6PrefixLen,
		MTU:     vlanMTU,
	}

	atePort2Vlan30 = attrs.Attributes{
		Name:    "atePort2Vlan20",
		IPv4:    "100.121.30.2",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2000::100:121:30:2",
		IPv6Len: ipv6PrefixLen,
		MTU:     vlanMTU,
	}
)

// configureATE configures port1, port2 and port3 on the ATE.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) *ondatra.ATETopology {
	top := ate.Topology().New()

	p1 := ate.Port(t, "port1")
	atePort1.AddToATE(top, p1, &dutPort1)

	p2 := ate.Port(t, "port2")
	atePort2.AddToATE(top, p2, &dutPort2)

	// p3 := ate.Port(t, "port3")
	// i3 := top.AddInterface(atePort3.Name).WithPort(p3)
	// i3.IPv4().
	// 	WithAddress(atePort3.IPv4CIDR()).
	// 	WithDefaultGateway(dutPort3.IPv4)

	return top
}

// configureDUT configures port1 and port2 on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	d := gnmi.OC()

	p1 := dut.Port(t, "port1").Name()
	if strings.Contains(p1, "Bundle") {
		// i1.Type = oc.IETFInterfaces_InterfaceType_ieee8023adLag
		// rely on base config
	} else {
		i1 := dutPort1.NewOCInterface(p1)
		gnmi.Replace(t, dut, d.Interface(p1).Config(), i1)
	}

	p2 := dut.Port(t, "port2").Name()
	if strings.Contains(p2, "Bundle") {
		// i1.Type = oc.IETFInterfaces_InterfaceType_ieee8023adLag
		// rely on base config
	} else {
		i2 := dutPort2.NewOCInterface(p2)
		gnmi.Replace(t, dut, d.Interface(p2).Config(), i2)
	}

}

func TestDecode(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	fmt.Println("**********************", dut.Name())
	fmt.Println(binary.BigEndian.Uint16([]byte("\000\001\000\002\000\003\000\001\000\001\000\001\206\335`\000\000\000\000\362;\001\001\000\001 \000\001\000\000\000\000\000\000\000\000\000\001\001\000\001!\000\001\000\000\000\000\000\000\000\000\000\0029\236\004\373yH\361`Ixia\000\000\000\000\020\021\022\023\020d\337\311\030\031\032\033\034\035\036\037 !\"#$%&'()*+,-./0123456789:;<=>?@ABCDEFGHIJKLMNOPQRSTUVWXYZ[\\]^_`abcdefghijklmnopqrstuvwxyz{|}~\177\200\201\202\203\204\205\206\207\210\211\212\213\214\215\216\217\220\221\222\223\224\225\226\227\230\231\232\233\234\235\236\237\240\241\242\243\244\245\246\247\250\251\252\253\254\255\256\257\260\261\262\263\264\265\266\267\270\271\272\273\274\275\276\277\300\301\302\303\304\305\306\307\310\311\312\313\314\315\316\317\320\321\322\323\324\325\326\327\330\331\332\333\334\335\336\337\340\341\342\343\344\345\346\347\350\351\352\353\354\355\356\357\360\361")))
}
