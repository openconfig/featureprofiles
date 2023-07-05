package gribi_mpls_compliance_test

import (
	"flag"
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

var (
	sleep = flag.Int("sleep", 0, "seconds to sleep within test before exiting")
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// TestMPLSLabelPushDepth validates the gRIBI actions that are used to push N labels onto
// a packet as part of routing towards a next-hop. Note that this test does not
// validate against the dataplane, but solely the gRIBI control-plane support.
func TestMPLSLabelPushDepth(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	gribic := dut.RawAPIs().GRIBI().Default(t)
	c := fluent.NewClient()
	c.Connection().WithStub(gribic)

	_ = PushBaseConfigs(t, ondatra.DUT(t, "dut"), ondatra.ATE(t, "ate"))
	sleepFn := func(_ *testing.T, _ []uint32) { time.Sleep(*sleep * time.Second) }

	baseLabel := 42
	for i := 1; i <= 20; i++ {
		t.Run(fmt.Sprintf("push %d labels", i), func(t *testing.T) {
			EgressLabelStack(t, c, defNIName, baseLabel, i, sleepFn)
		})
	}
}
