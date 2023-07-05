package gribi_mpls_compliance_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
)

const (
	// defNIName specifies the default network instance name to be used.
	defNIName = "default"
	// baseLabel specifies the lower bound label used within a stack.
	baseLabel = 42
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Test cases to write:
//	* push(N) labels, N = 1-20.
//	* pop(1) - terminating action
//	* pop(1) + push(N)
//	* pop(all) + push(N)

// TestMPLSLabelPushDepth validates the gRIBI actions that are used to push N labels onto
// a packet as part of routing towards a next-hop. Note that this test does not
// validate against the dataplane, but solely the gRIBI control-plane support.
func TestMPLSLabelPushDepth(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	gribic := dut.RawAPIs().GRIBI().Default(t)
	c := fluent.NewClient()
	c.Connection().WithStub(gribic)

	_ = PushBaseConfigs(t, ondatra.DUT(t, "dut"), ondatra.ATE(t, "ate"))
	sleepFn := func(_ *testing.T, _ []uint32) { time.Sleep(30 * time.Second) }

	baseLabel := 42
	for i := 1; i <= 20; i++ {
		t.Run(fmt.Sprintf("push %d labels", i), func(t *testing.T) {
			EgressLabelStack(t, c, defNIName, baseLabel, i, sleepFn)
		})
	}
}

// TestMPLSPushToIP validates the gRIBI actions that are used to push N labels onto
// an IP packet. Note that this test does not validate against the dataplane.
func TestMPLSPushToIP(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	gribic := dut.RawAPIs().GRIBI().Default(t)
	c := fluent.NewClient()
	c.Connection().WithStub(gribic)

	baseLabel := 42
	numLabels := 20
	for i := 1; i <= numLabels; i++ {
		t.Run(fmt.Sprintf("push %d labels to IP", i), func(t *testing.T) {
			PushToIPPacket(t, c, defNIName, baseLabel, i, nil)
		})
	}
}

// TestPopTopLabel validates the gRIBI actions that are used to pop the top label
// when specified in a next-hop.
func TestPopTopLabel(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	gribic := dut.RawAPIs().GRIBI().Default(t)
	c := fluent.NewClient()
	c.Connection().WithStub(gribic)

	PopTopLabel(t, c, defNIName, nil)
}

// TestPopNLabels validates the gRIBI actions that are used to pop N labels from a
// label stack when specified in a next-hop.
func TestPopNLabels(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	gribic := dut.RawAPIs().GRIBI().Default(t)
	c := fluent.NewClient()
	c.Connection().WithStub(gribic)

	for _, stack := range [][]uint32{{100}, {100, 42}, {100, 42, 43, 44, 45}} {
		t.Run(fmt.Sprintf("pop N labels, stack %v", stack), func(t *testing.T) {
			PopNLabels(t, c, defNIName, stack, nil)
		})
	}
}

// TestPopOnePushN validates the gRIBI actions that are used to pop 1 label and then
// push N when specified in a next-hop.
func TestPopOnePushN(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	gribic := dut.RawAPIs().GRIBI().Default(t)
	c := fluent.NewClient()
	c.Connection().WithStub(gribic)

	stacks := [][]uint32{
		{100}, // swap for label 100, pop+push for label 200
		{100, 200, 300, 400},
		{100, 200, 300, 400, 500, 600},
	}
	for _, stack := range stacks {
		t.Run(fmt.Sprintf("pop one, push N, stack: %v", stack), func(t *testing.T) {
			PopOnePushN(t, c, defNIName, stack, nil)
		})
	}
}
