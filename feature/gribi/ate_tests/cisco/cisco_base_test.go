package cisco_gribi

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
	clientA    *gribi.GRIBIHandler
	dut        *ondatra.DUTDevice
	ate        *ondatra.ATEDevice
	top        *ondatra.ATETopology
	interfaces []string
	usecase    int
	prefix     *gribiPrefix
}

type gribiPrefix struct {
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
		// {
		// 	name: "Change VIP1 to from UCMP to ECMP",
		// 	desc: "Programm double recursion transit with WCMP and change VIP1 to ECMP",
		// 	fn:   testChangeVip1UCMP,
		// },
	}
)

func TestCD2(t *testing.T) {
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

			clientA := gribi.NewGRIBIFluent(t, dut, true, false)
			defer clientA.Close(t)

			interfaceList := []string{}
			for i := 121; i < 128; i++ {
				interfaceList = append(interfaceList, fmt.Sprintf("Bundle-Ether%d", i))
			}

			args := &testArgs{
				ctx:        ctx,
				clientA:    clientA,
				dut:        dut,
				ate:        ate,
				top:        top,
				usecase:    0,
				interfaces: interfaceList,
				prefix: &gribiPrefix{
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
