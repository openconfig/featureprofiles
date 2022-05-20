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
	skip bool
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
			name: "Test DSCP Protocol Based VRF Selection",
			desc: "Test RT3.1 with DSCP, IPv4, IPv6, IPinIP based VRF selection",
			fn:   testDscpProtocolBasedVRFSelection,
		},
		{
			name: "Test Multiple DSCP Protocol Rule Based VRF Selection",
			desc: "Test RT3.2 with multiple DSCP, IPinIP protocol based VRF selection",
			fn:   testMultipleDscpProtocolRuleBasedVRFSelection,
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
		{
			name: "Unconfigure policy under interface",
			desc: "Unconfigure PBR policy under bundle interface and verify traffic drop",
			fn:   testUnconfigPBRUnderBundleInterface,
		},
		{
			name: "Add new match field",
			desc: "Add new match field in existing class-map and verify traffic",
			fn:   testAddMatchField,
		},
		{
			name: "Modify existing match field",
			desc: "Modify existing match filed in the existing class-map and verify traffic",
			fn:   testModifyMatchField,
		},
		{
			name: "Remove existing match field",
			desc: "Remove existing existing match field in existing class-map and verify traffic",
			fn:   testRemoveMatchField,
		},
		{
			name: "Flap Interface",
			desc: "Flap Interface and verify traffic",
			fn:   testTrafficFlapInterface,
		},
		{
			name: "Match DSCP and VRF redirect",
			desc: "Verify PBR policy works with match DSCP and action VRF redirect",
			fn:   testMatchDscpActionVRFRedirect,
		},
		{
			name: "Commit replace with PBR config changes",
			desc: "Unconfig/config with PBR and verify traffic fails/passes",
			fn:   testRemAddPBRWithGNMIReplace,
		},
		{
			name: "Commit replace with HW config along with OC via GNMI",
			desc: "Unconfig/config  PBR using oc and HWModule using text in the same GNMI replace  and verify traffic fails/passes",
			fn:   testRemAddHWWithGNMIReplaceAndPBRwithOC,
		},
		{
			name: "Add remove hw-module CLI",
			desc: "remove/add the pbr policy using hw-module and verify traffic fails/passes",
			fn:   testRemAddHWModule,
		},
		{
			name: "Test Acl And PBR Under Same Interface",
			desc: "Configure ACL and PBR under same interface and verify functionality",
			fn:   testAclAndPBRUnderSameInterface,
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
	t.Skip()
	dut := ondatra.DUT(t, "dut")

	// Dial gRIBI
	ctx := context.Background()

	// Disable Flowspec and Enable PBR
	convertFlowspecToPBR(ctx, t, dut)

	//Configure IPv6 addresses and VLANS on DUT
	configureIpv6AndVlans(t, dut)

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
