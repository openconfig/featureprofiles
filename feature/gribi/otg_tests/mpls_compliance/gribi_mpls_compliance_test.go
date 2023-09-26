// Package gribi_mpls_compliance_test implements test that validate the gRIBI
// server behaviour for MPLS programming. No traffic validation is performed
// such that the validation is that the API for MPLS is complied with.
package gribi_mpls_compliance_test

import (
	"flag"
	"fmt"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/feature/gribi/mplsutil"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
)

const (
	// baseLabel specifies the lower bound label used within a stack.
	baseLabel = 42
	// maxLabelDepth is the maximum number of labels that should be pushed on the stack.
	maxLabelDepth = 20
)

var (
	sleep = flag.Int("sleep", 0, "seconds to sleep within test before exiting")
)

// sleepFn is a function that is called by default to pause the test in a specific state before exiting.
// It reads the sleep duration from the provided flag.
func sleepFn(_ *testing.T, _ []uint32) { time.Sleep(time.Duration(*sleep) * time.Second) }

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// TestMPLSLabelPushDepth validates the gRIBI actions that are used to push N labels onto
// a packet as part of routing towards a next-hop. Note that this test does not
// validate against the dataplane, but solely the gRIBI control-plane support.
func TestMPLSLabelPushDepth(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	gribic := dut.RawAPIs().GRIBI(t)
	c := fluent.NewClient()
	c.Connection().WithStub(gribic)

	_ = mplsutil.PushBaseConfigs(t, ondatra.DUT(t, "dut"), ondatra.ATE(t, "ate"))

	for i := 1; i <= maxLabelDepth; i++ {
		t.Run(fmt.Sprintf("push %d labels", i), func(t *testing.T) {
			mplsutil.PushLabelStack(t, c, deviations.DefaultNetworkInstance(dut), baseLabel, i, sleepFn)
		})
	}
}
