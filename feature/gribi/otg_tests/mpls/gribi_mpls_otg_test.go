package gribi_mpls_dataplane_test

import (
	"fmt"
	"testing"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	mplscompliance "github.com/openconfig/featureprofiles/feature/gribi/tests/mpls"
)

const (
	defNIName = "default"
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
// as part of routing towards a next-hop. Note that this test does not validate against the
// dataplane, but solely the gRIBI control-plane support.
func TestMPLSLabelPushDepth(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	gribic := dut.RawAPIs().GRIBI().Default(t)
	c := fluent.NewClient()
	c.Connection().WithStub(gribic)

	baseLabel := 42
	for i := 1; i <= 20; i++ {
		t.Run(fmt.Sprintf("push %d labels", i), func(t *testing.T) {
			// TODO(robjs): define the traffic validation function here.
			mplscompliance.EgressLabelStack(t, c, baseLabel, defNIName, i, nil)
		})
	}
}
