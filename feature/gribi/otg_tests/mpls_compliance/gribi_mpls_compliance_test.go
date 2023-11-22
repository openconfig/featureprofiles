// Package gribi_mpls_compliance_test implements test that validate the gRIBI
// server behaviour for MPLS programming. No traffic validation is performed
// such that the validation is that the API for MPLS is complied with.
package gribi_mpls_compliance_test

import (
	"fmt"
	"testing"

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

	for numLabels := 1; numLabels <= maxLabelDepth; numLabels++ {
		t.Run(fmt.Sprintf("TE-9.1: Push MPLS labels to MPLS payload: %d labels", numLabels), func(t *testing.T) {
			labels := []uint32{}
			for i := 0; i < numLabels; i++ {
				labels = append(labels, uint32(baseLabel+i))
			}

			mplsT := mplsutil.New(c, mplsutil.PushToMPLS, deviations.DefaultNetworkInstance(dut), &mplsutil.Args{
				LabelsToPush: labels,
			})

			mplsT.ConfigureDevices(t, ondatra.DUT(t, "dut"), ondatra.ATE(t, "ate"))
			mplsT.ProgramGRIBI(t)
			mplsT.ValidateProgramming(t)
			mplsT.Cleanup(t)
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

	baseLabel := 42
	numLabels := 20
	for i := 1; i <= numLabels; i++ {
		t.Run(fmt.Sprintf("TE-9.2: Push MPLS labels to IP packet: %d labels", i), func(t *testing.T) {
			labels := []uint32{}
			for i := 0; i < numLabels; i++ {
				labels = append(labels, uint32(baseLabel+i))
			}
			mplsT := mplsutil.New(c, mplsutil.PushToIP, deviations.DefaultNetworkInstance(dut), &mplsutil.Args{
				LabelsToPush: labels,
			})
			mplsT.ConfigureDevices(t, ondatra.DUT(t, "dut"), ondatra.ATE(t, "ate"))
			mplsT.ProgramGRIBI(t)
			mplsT.ValidateProgramming(t)
			mplsT.Cleanup(t)
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

	t.Run("TE-9.3: Pop top MPLS label", func(t *testing.T) {
		mplsT := mplsutil.New(c, mplsutil.PopTopLabel, deviations.DefaultNetworkInstance(dut), nil)
		mplsT.ConfigureDevices(t, ondatra.DUT(t, "dut"), ondatra.ATE(t, "ate"))
		mplsT.ProgramGRIBI(t)
		mplsT.ValidateProgramming(t)
	})
}

// TestPopNLabels validates the gRIBI actions that are used to pop N labels from a
// label stack when specified in a next-hop.
func TestPopNLabels(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	gribic := dut.RawAPIs().GRIBI(t)
	c := fluent.NewClient()
	c.Connection().WithStub(gribic)

	for _, stack := range [][]uint32{{100}, {100, 42}, {100, 42, 43, 44, 45}} {
		t.Run(fmt.Sprintf("TE-9.4: Pop N Labels From Stack: stack %v", stack), func(t *testing.T) {
			mplsT := mplsutil.New(c, mplsutil.PopNLabels, deviations.DefaultNetworkInstance(dut), &mplsutil.Args{
				LabelsToPop: stack,
			})
			mplsT.ConfigureDevices(t, ondatra.DUT(t, "dut"), ondatra.ATE(t, "ate"))
			mplsT.ProgramGRIBI(t)
			mplsT.ValidateProgramming(t)
			mplsT.Cleanup(t)
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

	stacks := [][]uint32{
		{100}, // swap for label 100, pop+push for label 200
		{100, 200, 300, 400},
		{100, 200, 300, 400, 500, 600},
	}
	for _, stack := range stacks {
		t.Run(fmt.Sprintf("TE-9.5: Pop 1 Push N labels: stack: %v", stack), func(t *testing.T) {
			mplsT := mplsutil.New(c, mplsutil.PopOnePushN, deviations.DefaultNetworkInstance(dut), &mplsutil.Args{
				LabelsToPop: stack,
			})
			mplsT.ConfigureDevices(t, ondatra.DUT(t, "dut"), ondatra.ATE(t, "ate"))
			mplsT.ProgramGRIBI(t)
			mplsT.ValidateProgramming(t)
			mplsT.Cleanup(t)
		})
	}

}
