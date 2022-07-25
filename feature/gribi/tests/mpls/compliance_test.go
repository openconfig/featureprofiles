package gribi_mpls_compliance_test

import (
	"fmt"
	"testing"

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

	baseLabel := 42
	for i := 1; i <= 20; i++ {
		t.Run(fmt.Sprintf("push %d labels", i), func(t *testing.T) {
			EgressLabelStack(t, c, defNIName, baseLabel, i, nil)
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
