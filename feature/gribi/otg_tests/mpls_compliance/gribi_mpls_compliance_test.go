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

// TestMPLSPushToIP validates the gRIBI actions that are used to push N labels onto
// an IP packet. Note that this test does not validate against the dataplane.
func TestMPLSPushToIP(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	gribic := dut.RawAPIs().GRIBI(t)
	c := fluent.NewClient()
	c.Connection().WithStub(gribic)

	_ = mplsutil.PushBaseConfigs(t, ondatra.DUT(t, "dut"), ondatra.ATE(t, "ate"))

	baseLabel := 42
	numLabels := 20
	for i := 1; i <= numLabels; i++ {
		t.Run(fmt.Sprintf("push %d labels to IP", i), func(t *testing.T) {
			mplsutil.PushToIPPacket(t, c, deviations.DefaultNetworkInstance(dut), baseLabel, i, sleepFn)
		})
	}
}

// TestPopTopLabel validates the gRIBI actions that are used to pop the top label
// when specified in a next-hop.
func TestPopTopLabel(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	gribic := dut.RawAPIs().GRIBI(t)
	c := fluent.NewClient()
	c.Connection().WithStub(gribic)

	_ = mplsutil.PushBaseConfigs(t, ondatra.DUT(t, "dut"), ondatra.ATE(t, "ate"))

	mplsutil.PopTopLabel(t, c, deviations.DefaultNetworkInstance(dut), sleepFn)
}
<<<<<<< HEAD

// TestPopNLabels validates the gRIBI actions that are used to pop N labels from a
// label stack when specified in a next-hop.
func TestPopNLabels(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	gribic := dut.RawAPIs().GRIBI(t)
	c := fluent.NewClient()
	c.Connection().WithStub(gribic)

	_ = mplsutil.PushBaseConfigs(t, ondatra.DUT(t, "dut"), ondatra.ATE(t, "ate"))

	for _, stack := range [][]uint32{{100}, {100, 42}, {100, 42, 43, 44, 45}} {
		t.Run(fmt.Sprintf("pop N labels, stack %v", stack), func(t *testing.T) {
			mplsutil.PopNLabels(t, c, deviations.DefaultNetworkInstance(dut), stack, sleepFn)
		})
	}
}

// TestPopOnePushN validates the gRIBI actions that are used to pop 1 label and then
// push N when specified in a next-hop.
func TestPopOnePushN(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	gribic := dut.RawAPIs().GRIBI(t)
	c := fluent.NewClient()
	c.Connection().WithStub(gribic)

	_ = mplsutil.PushBaseConfigs(t, ondatra.DUT(t, "dut"), ondatra.ATE(t, "ate"))

	stacks := [][]uint32{
		{100}, // swap for label 100, pop+push for label 200
		{100, 200, 300, 400},
		{100, 200, 300, 400, 500, 600},
	}
	for _, stack := range stacks {
		t.Run(fmt.Sprintf("pop one, push N, stack: %v", stack), func(t *testing.T) {
			mplsutil.PopOnePushN(t, c, deviations.DefaultNetworkInstance(dut), stack, sleepFn)
		})
	}
}
=======
>>>>>>> 81bb985c (Add gRIBI control plane only pop top MPLS label test.)
