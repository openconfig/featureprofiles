package cisco_gribi

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/gribigo/fluent"
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
	clientA    *fluent.GRIBIClient
	dut        *ondatra.DUTDevice
	ate        *ondatra.ATEDevice
	top        *ondatra.ATETopology
	interfaces []string
	usecase    int
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
			name: "Change VIP1 to from UCMP to ECMP",
			desc: "Programm double recursion transit with WCMP and change VIP1 to ECMP",
			fn:   testChangeVip1UCMP,
		},
	}
)

func TestCD2(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	// Dial gRIBI
	ctx := context.Background()
	gribic := dut.RawAPIs().GRIBI().Default(t)

	// Configure the ATE
	ate := ondatra.ATE(t, "ate")
	top := configureATE(t, ate)
	top.Push(t).StartProtocols(t)

	for _, tt := range CD2Testcases {
		// Each case will run with its own gRIBI fluent client.
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Name: %s", tt.name)
			t.Logf("Description: %s", tt.desc)

			electionID := GetNextElectionIdviaStub(gribic, t)

			// Configure the gRIBI client clientA with election ID of 10.
			clientA := fluent.NewClient()

			clientA.Connection().WithStub(gribic).WithInitialElectionID(electionID, 0).
				WithRedundancyMode(fluent.ElectedPrimaryClient).WithPersistence()

			clientA.Start(ctx, t)
			defer clientA.Stop(t)
			clientA.StartSending(ctx, t)
			if err := awaitTimeout(ctx, clientA, t, time.Minute); err != nil {
				t.Fatalf("Await got error during session negotiation for clientA: %v", err)
			}

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
			}

			tt.fn(ctx, t, args)
		})
	}
}
