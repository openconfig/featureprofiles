package qos_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	ciscoFlags "github.com/openconfig/featureprofiles/internal/cisco/flags"
	"github.com/openconfig/featureprofiles/internal/cisco/gribi"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
)

// Testcase defines testcase structure
type Testcase struct {
	name string
	desc string
	fn   func(ctx context.Context, t *testing.T, args *testArgs)
}

const (
	inint1 = "Bundle-Ether122"
	inint2 = "Bundle-Ether123"
	mac1   = "00:01:00:03:00:00"
	mac2   = "00:01:00:04:00:00"
)

// testArgs holds the objects needed by a test case.
type testArgs struct {
	ctx        context.Context
	clientA    *gribi.Client
	dut        *ondatra.DUTDevice
	ate        *ondatra.ATEDevice
	top        *ondatra.ATETopology
	interfaces *interfaces
	usecase    int
	prefix     *gribiPrefix
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

var (
	QoSTrafficTestcases = []Testcase{
		{
			name: "Test QOS counters with Traffic",
			desc: "Program gribi with wucmp and verify qos counters",
			fn:   testQosCounter,
		},
		{
			name: "test clear counters with traffic",
			desc: "Clear qos counters and send traffic again",
			fn:   ClearQosCounter,
		},
		{
			name: "test queue delete and verify qos stats",
			desc: "Delete and Add indvidual queue and verify qos stats",
			fn:   QueueDelete,
		},
		{
			name: "test ipv6 Qos policy with traffic",
			desc: "Test ipv6 traffic with Qos and verify stats",
			fn:   testQosCounteripv6,
		},
	}
)

var (
	QosSchedulerTestcases = []Testcase{
		{
			name: "testing scheduling functionality",
			desc: "create congestion on egress interface and test scheduling for queue7",
			fn:   testScheduler,
		},
		{
			name: "testing scheduling functionality for queue6",
			desc: "create congestion on egress interface and test scheduling",
			fn:   testScheduler2,
		},
		{
			name: "testing scheduling functionality with wrr and ecn",
			desc: "create congestion on egress interface and test scheduling interfaces",
			fn:   testSchedulerwrr,
		},
	}
)
var (
	QoSWrrTrafficTestcases = []Testcase{

		{
			name: "Test QOS counters with Traffic with wrr configs",
			desc: "Program gribi with wucmp and verify qos counters",
			fn:   testQoswrrCounter,
		},
		{
			name: "Test QOS counters streaming with Traffic with wrr configs",
			desc: "Program gribi with wucmp and verify qos streaming",
			fn:   testQoswrrStreaming,
		},
		{
			name: "Test Delete of sequence and add back sequence",
			desc: "testing gribi tests with sequence del and add",
			fn:   testQoswrrdeladdseq,
		},
	}
)

func TestTrafficQos(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	cliHandle := dut.RawAPIs().CLI(t)
	defer cliHandle.Close()
	resp, err := cliHandle.SendCommand(context.Background(), "show version")
	t.Logf(resp)
	if err != nil {
		t.Error(err)
	}
	if strings.Contains(resp, "VXR") {
		t.Logf("Skipping since platfrom is VXR")
		t.Skip()
	}

	//Configure IPv6 addresses and VLANS on DUT
	configureIpv6AndVlans(t, dut)

	// Dial gRIBI
	ctx := context.Background()

	// Configure the ATE
	ate := ondatra.ATE(t, "ate")
	top := configureATE(t, ate)
	top.Push(t).StartProtocols(t)

	for _, tt := range QoSTrafficTestcases {
		// Each case will run with its own gRIBI fluent client.
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Name: %s", tt.name)
			t.Logf("Description: %s", tt.desc)

			clientA := gribi.Client{
				DUT:                   dut,
				FibACK:                *ciscoFlags.GRIBIFIBCheck,
				Persistence:           true,
				InitialElectionIDLow:  10,
				InitialElectionIDHigh: 0,
			}
			defer clientA.Close(t)
			if err := clientA.Start(t); err != nil {
				t.Fatalf("Could not initialize gRIBI: %v", err)
			}
			//clientA.BecomeLeader(t)

			interfaceList := []string{}
			for i := 121; i < 128; i++ {
				interfaceList = append(interfaceList, fmt.Sprintf("Bundle-Ether%d", i))
			}

			interfaces := interfaces{
				in:  []string{"Bundle-Ether120"},
				out: interfaceList,
			}

			args := &testArgs{
				ctx:        ctx,
				clientA:    &clientA,
				dut:        dut,
				ate:        ate,
				top:        top,
				usecase:    0,
				interfaces: &interfaces,
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

			tt.fn(ctx, t, args)
		})
	}
}

func TestScheduler(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	time.Sleep(time.Minute)
	cliHandle := dut.RawAPIs().CLI(t)
	defer cliHandle.Close()
	resp, err := cliHandle.SendCommand(context.Background(), "show version")
	t.Logf(resp)
	if err != nil {
		t.Error(err)
	}
	if strings.Contains(resp, "VXR") {
		t.Logf("Skipping since platfrom is VXR")
		t.Skip()
	}

	// Dial gRIBI
	ctx := context.Background()

	//Configure IPv6 addresses and VLANS on DUT
	configureIpv6AndVlans(t, dut)
	gnmi.Update(t, dut, gnmi.OC().Interface(inint1).Ethernet().MacAddress().Config(), mac1)
	gnmi.Update(t, dut, gnmi.OC().Interface(inint2).Ethernet().MacAddress().Config(), mac2)

	// Disable Flowspec and Enable PBR

	// Configure the ATE
	ate := ondatra.ATE(t, "ate")
	top := configureATE(t, ate)
	top.Push(t).StartProtocols(t)

	for _, tt := range QosSchedulerTestcases {
		// Each case will run with its own gRIBI fluent client.
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Name: %s", tt.name)
			t.Logf("Description: %s", tt.desc)

			clientA := gribi.Client{

				DUT:                   dut,
				FibACK:                *ciscoFlags.GRIBIFIBCheck,
				Persistence:           true,
				InitialElectionIDLow:  10,
				InitialElectionIDHigh: 0,
			}
			defer clientA.Close(t)
			if err := clientA.Start(t); err != nil {
				t.Fatalf("Could not initialize gRIBI: %v", err)
			}
			//clientA.BecomeLeader(t)

			interfaceList := []string{}
			for i := 121; i < 128; i++ {
				interfaceList = append(interfaceList, fmt.Sprintf("Bundle-Ether%d", i))
			}

			interfaces := interfaces{
				in:  []string{"Bundle-Ether120"},
				out: interfaceList,
			}

			args := &testArgs{
				ctx:        ctx,
				clientA:    &clientA,
				dut:        dut,
				ate:        ate,
				top:        top,
				usecase:    0,
				interfaces: &interfaces,
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

			tt.fn(ctx, t, args)
		})
	}
}

func TestWrrTrafficQos(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	time.Sleep(time.Minute)
	cliHandle := dut.RawAPIs().CLI(t)
	defer cliHandle.Close()
	resp, err := cliHandle.SendCommand(context.Background(), "show version")
	t.Logf(resp)
	if err != nil {
		t.Error(err)
	}
	if strings.Contains(resp, "VXR") {
		t.Logf("Skipping since platfrom is VXR")
		t.Skip()
	}
	//Configure IPv6 addresses and VLANS on DUT
	configureIpv6AndVlans(t, dut)

	// Dial gRIBI
	ctx := context.Background()

	// Configure the ATE
	ate := ondatra.ATE(t, "ate")
	top := configureATE(t, ate)
	top.Push(t).StartProtocols(t)

	for _, tt := range QoSWrrTrafficTestcases {
		// Each case will run with its own gRIBI fluent client.
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Name: %s", tt.name)
			t.Logf("Description: %s", tt.desc)

			clientA := gribi.Client{
				DUT:                   dut,
				FibACK:                *ciscoFlags.GRIBIFIBCheck,
				Persistence:           true,
				InitialElectionIDLow:  10,
				InitialElectionIDHigh: 0,
			}
			defer clientA.Close(t)
			if err := clientA.Start(t); err != nil {
				t.Fatalf("Could not initialize gRIBI: %v", err)
			}
			//clientA.BecomeLeader(t)

			interfaceList := []string{}
			for i := 121; i < 128; i++ {
				interfaceList = append(interfaceList, fmt.Sprintf("Bundle-Ether%d", i))
			}

			interfaces := interfaces{
				in:  []string{"Bundle-Ether120"},
				out: interfaceList,
			}

			args := &testArgs{
				ctx:        ctx,
				clientA:    &clientA,
				dut:        dut,
				ate:        ate,
				top:        top,
				usecase:    0,
				interfaces: &interfaces,
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

			tt.fn(ctx, t, args)
		})
	}
}

// func TestDelQos(t *testing.T) {
// 	dut := ondatra.DUT(t, "dut")
// 	dut.Config().Qos().Delete(t)

// }
