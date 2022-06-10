package cisco_p4rt_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	p4_v1 "github.com/p4lang/p4runtime/go/p4/v1"
	p4rt_client "wwwin-github.cisco.com/rehaddad/go-p4/p4rt_client"
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
	dut         *ondatra.DUTDevice
	ate         *ondatra.ATEDevice
	top         *ondatra.ATETopology
	interfaces  *interfaces
	usecase     int
	prefix      *gribiPrefix
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
	P4RTTestcases = []Testcase{
		{
			name: "Move physical interface to bundle with same policy",
			desc: "Remove the policy under physical interface and add the related physical interface under bundle interface which use the same PBR policy",
			fn:   testGDPEntryProgramming,
		},
	}
)

func TestP4RT(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	// Dial gRIBI
	ctx := context.Background()

	// Configure the ATE
	ate := ondatra.ATE(t, "ate")
	top := configureATE(t, ate)
	// top.Push(t).StartProtocols(t)

	for _, tt := range P4RTTestcases {
		// Each case will run with its own gRIBI fluent client.
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Name: %s", tt.name)
			t.Logf("Description: %s", tt.desc)

			p4rtClientA := p4rt_client.P4RTClient{}
			if err := p4rtClientA.P4rtClientSet(dut.RawAPIs().P4RT(t)); err != nil {
				t.Fatalf("Could not initialize p4rt client: %v", err)
			}

			p4rtClientB := p4rt_client.P4RTClient{}
			if err := p4rtClientB.P4rtClientSet(dut.RawAPIs().P4RT(t)); err != nil {
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

			tt.fn(ctx, t, args)
		})
	}
}

func setupP4RTClient(ctx context.Context, t *testing.T, args *testArgs) error {
	// Configure device-id and port-id
	deviceID := uint64(1)

	// Setup P4RT ClientA
	streamName := "Primary"
	electionID := uint64(100)
	streamParameter := p4rt_client.P4RTStreamParameters{
		Name:        streamName,
		DeviceId:    deviceID,
		ElectionIdH: uint64(0),
		ElectionIdL: electionID,
	}
	if client := args.p4rtClientA; client != nil {
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
		}
		streamParameter.ElectionIdL -= 1
	}

	if client := args.p4rtClientB; client != nil {
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
			t.Logf("There is error when setting up p4rtClientB")
			return err
		}
		streamParameter.ElectionIdL -= 1
	}

	return nil
}

func configureATE(t *testing.T, ate *ondatra.ATEDevice) *ondatra.ATETopology {
	top := ate.Topology().New()
	return top
}
