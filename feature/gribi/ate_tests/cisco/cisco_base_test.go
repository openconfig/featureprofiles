package cisco_gribi_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/ondatra"
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
	CD2Testcases = []Testcase{
		{
			name: "Transit with Double Recursion",
			desc: "Programm double recursion transit with WCMP",
			fn:   testDoubleRecursionWithUCMP,
		},
		{
			name: "Change VRF to non-UCMP and change back",
			desc: "Programm double recursion transit with WCMP and change VIP1 to ECMP",
			fn:   testDeleteAndAddUCMP,
		},
		{
			name: "Change VRF to non-recursive and change back",
			desc: "Programm double recursion transit with WCMP and change VRF prefix to non-recursion and change it back",
			fn:   testVRFnonRecursion,
		},
	}
)

var (
	CD5Testcases = []Testcase{
		{
			name: "Move physical interface to bundle with same policy",
			desc: "Remove the policy under physical interface and add the related physical interface under bundle interface which use the same PBR policy",
			fn:   testMovePhysicalToBundleWithSamePolicy,
		},
		{
			name: "Move physical interface to bundle with different policy",
			desc: "Remove the policy under physical interface and add the related physical interface under bundle interface which use the same PBR policy",
			fn:   testMovePhysicalToBundleWithDifferentPolicy,
		},
		{
			name: "Change policy under interface",
			desc: "Change existing policy under the interface to a new one and verify gribi traffic",
			fn:   testChangePBRUnderInterface,
		},
		{
			name: "Match IPinIP with IPv6inIPv4 traffic",
			desc: "Configure with policy matching protocol IPinIP and send IPv6 in IPv4 and verify traffic drop",
			fn:   testIPv6InIPv4Traffic,
		},
		{
			name: "Remove existing class-map",
			desc: "Remove existing class-map which is not related to matching protocol IPinIP and verify traffic",
			fn:   testRemoveClassMap,
		},
		{
			name: "Change existing action",
			desc: "Change existing action to a new VRF with existing class-map and verify traffic",
			fn:   testChangeAction,
		},
		{
			name: "Add new class-map",
			desc: "Add new class-map to existing policy and verify traffic",
			fn:   testAddClassMap,
		},
	}
)

func TestTransitWCMPFlush(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	// Dial gRIBI
	ctx := context.Background()

	// Configure the ATE
	ate := ondatra.ATE(t, "ate")
	top := configureATE(t, ate)
	top.Push(t).StartProtocols(t)

	for _, tt := range CD2Testcases {
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

func TestCD5PBR(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	// Dial gRIBI
	ctx := context.Background()

	// Disable Flowspec and Enable PBR
	convertFlowspecToPBR(ctx, t, dut)

	// Configure the ATE
	ate := ondatra.ATE(t, "ate")
	top := configureATE(t, ate)
	top.Push(t).StartProtocols(t)

	for _, tt := range CD5Testcases {
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
