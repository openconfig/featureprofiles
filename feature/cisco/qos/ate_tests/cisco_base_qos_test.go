package qos_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/ondatra"
	//"github.com/openconfig/featureprofiles/internal/cisco/config"
	//"github.com/openconfig/ygot/ygot"
)

// Testcase defines testcase structure
type Testcase struct {
	name string
	desc string
	fn   func(ctx context.Context, t *testing.T, args *testArgs)
}

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
			name: "Transit with Double Recursion",
			desc: "Programm double recursion transit with WCMP",
			fn:   testQosCounter,
		},
		{
			name: "test clear counters with traffic",
			desc: "Clear qod counters and sen traffic again",
			fn:   ClearQosCounter,
		},
		{
			name: "test clear counters with traffic",
			desc: "Clear qod counters and sen traffic again",
			fn:   QueueDelete,
		},
		{
			name: "test clear counters with traffic",
			desc: "Clear qod counters and sen traffic again",
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
	}
)

func TestTrafficQos(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
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
				DUT:                  ondatra.DUT(t, "dut"),
				FibACK:               false,
				Persistence:          true,
				InitialElectionIDLow: 1,
			}
			defer clientA.Close(t)
			if err := clientA.Start(t); err != nil {
				t.Fatalf("Could not initialize gRIBI: %v", err)
			}
			clientA.BecomeLeader(t)

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

	// Dial gRIBI
	ctx := context.Background()

	//Configure IPv6 addresses and VLANS on DUT
	configureIpv6AndVlans(t, dut)
	dut.Config().Interface("Bundle-Ether122").Ethernet().MacAddress().Update(t, "00:01:00:03:00:00")
	dut.Config().Interface("Bundle-Ether123").Ethernet().MacAddress().Update(t, "00:01:00:04:00:00")

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
				DUT:                  ondatra.DUT(t, "dut"),
				FibACK:               false,
				Persistence:          true,
				InitialElectionIDLow: 10,
			}
			defer clientA.Close(t)
			if err := clientA.Start(t); err != nil {
				t.Fatalf("Could not initialize gRIBI: %v", err)
			}
			clientA.BecomeLeader(t)

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
