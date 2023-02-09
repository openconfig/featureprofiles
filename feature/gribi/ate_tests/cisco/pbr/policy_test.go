package policy_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/openconfig/featureprofiles/internal/attrs"
	ciscoFlags "github.com/openconfig/featureprofiles/internal/cisco/flags"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/ondatra"
)

const (
	ipv4PrefixLen = 24
	ipv6PrefixLen = 126
	vlanMTU       = 1518
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "dutPort1",
		IPv4:    "100.120.1.1",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2000::100:120:1:1",
		IPv6Len: ipv6PrefixLen,
	}

	atePort1 = attrs.Attributes{
		Name:    "atePort1",
		IPv4:    "100.120.1.2",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2000::100:120:1:2",
		IPv6Len: ipv6PrefixLen,
	}

	dutPort2 = attrs.Attributes{
		Desc:    "dutPort2",
		IPv4:    "100.121.1.1",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2000::100:121:1:1",
		IPv6Len: ipv6PrefixLen,
	}

	atePort2 = attrs.Attributes{
		Name:    "atePort2",
		IPv4:    "100.121.1.2",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2000::100:121:1:2",
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
			name: "Test Acl And PBR Under Same Interface",
			desc: "Configure ACL and PBR under same interface and verify functionality",
			fn:   testAclAndPBRUnderSameInterface,
		},
		{
			name: "Test Src-Ip match",
			desc: "Verify PBR policy works with match Src-ip and action VRF redirect",
			fn:   testSrcIp,
		},
		{
			name: "Test Src-Ip match- Negative case ",
			desc: "Verify PBR policy works with missmatched Src-ip and action VRF redirect",
			fn:   testSrcIpNegative,
		},
		{
			name: "Test Src-Ip and Dscp match",
			desc: "Verify PBR policy works with match Src-ip,Dscp and action VRF redirect",
			fn:   testPBRSrcIpWithDscp,
		},
		{
			name: "Test Attach a Different SrcIp",
			desc: "Verify PBR policy after attaching a different SrcIp and action VRF redirect",
			fn:   testDettachAndAttachDifferentSrcIp,
		},
		{
			name: "Test Attach a Wrong SrcIp",
			desc: "Verify PBR policy after attaching a Wrong SrcIp and action VRF redirect",
			fn:   testDettachAndAttachWrongSrcIp,
		},
		{
			name: "Test Replace Policies",
			desc: "Replaces policy with non-matchable policies and expects the traffic to fail",
			fn:   testPolicesReplace,
		},
		{
			name: "Test Replace Policy",
			desc: "Replaces policy with non-matchable policy and expects the traffic to fail",
			fn:   testPolicyReplace,
		},
		{
			name: "Commit replace with PBR config changes",
			desc: "Unconfig/config with PBR and verify traffic fails/passes",
			fn:   testRemAddPBRWithGNMIReplace,
		},
		{
			name: "Test Update SrcIp",
			desc: "Verify PBR policy after Updating SrcIp and action VRF redirect",
			fn:   testUpdateSrcIp,
		},
		{
			name: "Test Update Wrong Scr Ip",
			desc: "Verify PBR policy after updating Wrong Src-ip and action VRF redirect",
			fn:   testUpdateWrongSrcIp,
		},
		{
			name: "Test Replace Src-Ip at Leaf Level",
			desc: "Verify PBR policy works with match Src-ip after replace at src-ip and action VRF redirect",
			fn:   testReplaceAtSrcIpLeaf,
		},
		{
			name: "Test Replace Src-Ip at Leaf Level-Negative",
			desc: "Verify PBR policy works with match Src-ip after update at src-ip - Negative case and action VRF redirect",
			fn:   testUpdateAtSrcIpLeafNegative,
		},
		{
			name: "Test Replace Src-Ip at Rules Level",
			desc: "Verify PBR policy works with match Src-ip after replace at rule level and action VRF redirect",
			fn:   testReplaceSrcIpRule,
		},
		{
			name: "Test Replace Src-Ip at Leaf Level",
			desc: "Verify PBR policy works with match Src-ip after replace at src-ip and action VRF redirect",
			fn:   testReplaceSrcIpEntirePolicy,
		},
		{
			name: "Test Update Src-Ip at Leaf Level",
			desc: "Verify PBR policy works with match Src-ip after update at src-ip and action VRF redirect",
			fn:   testUpdateAtSrcIpLeaf,
		},
		{
			name: "Test Src-Ip with many rules",
			desc: "Verify PBR policy works with match Src-ip after update at src-ip and action VRF redirect",
			fn:   testSrcIpMoreRules,
		},
		{
			name: "Test Src-Ip with Dscp value",
			desc: "Verify PBR policy works with match Src-ip and Dscp after update at src-ip and action VRF redirect",
			fn:   testSrcIpWithDscp,
		},
		{
			name: "Test Src-Ip with protocol 41-negative",
			desc: "Verify Src-ip with prtotvol 41-negative",
			fn:   testProtocolV6Negative,
		},
		{
			name: "Test Src-Ip with protocol 41",
			desc: "Verify Src-ip with prtotvol 41",
			fn:   testProtocolV6,
		},
		{
			name: "Test Src-Ip with 41-update",
			desc: "Verify Src-ip with prtotvol 41 and then update with protocl 4",
			fn:   testProtocolV6updateV4,
		},
		{
			name: "Test Src-Ip with 41-replace",
			desc: "Verify Src-ip with prtotvol 41 and then replace with protocl 4",
			fn:   testProtocolV6replaceV4,
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
	}
)

func TestCD5PBR(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	// Dial gRIBI
	ctx := context.Background()

	//Configure IPv6 addresses and VLANS on DUT
	configureIpv6AndVlans(t, dut)

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
				DUT:         ondatra.DUT(t, "dut"),
				FIBACK:      false,
				Persistence: true,
			}
			defer clientA.Close(t)
			if err := clientA.Start(t); err != nil {
				t.Logf("gRIBI Connection could not be established: %v\nRetrying...", err)
				if err = clientA.Start(t); err != nil {
					t.Fatalf("gRIBI Connection could not be established: %v", err)
				}
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
					vrfName:         *ciscoFlags.NonDefaultNetworkInstance,
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
