// Package gRIBI MPLS forwarding implements tests of the MPLS dataplane that
// use gRIBI as the programming mechanism.
package gribi_mpls_forwarding_test

import (
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
	// baseLabel indicates the minimum label to use on a packet.
	baseLabel = 42
	// maximumStackDepth is the maximum number of labels to be pushed onto the packet.
	maximumStackDepth = 20
	// lossTolerance is the number of packets that can be lost within a flow before the
	// test fails.
	lossTolerance = 1
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// TestMPLSLabelPushDepth validates the gRIBI actions that are used to push N labels onto
// as part of routing towards a next-hop. Note that this test does not validate against the
// dataplane, but solely the gRIBI control-plane support.
func TestMPLSLabelPushDepth(t *testing.T) {
	gribic := ondatra.DUT(t, "dut").RawAPIs().GRIBI(t)
	c := fluent.NewClient()
	c.Connection().WithStub(gribic)

	for numLabels := 1; numLabels <= maximumStackDepth; numLabels++ {
		t.Run(fmt.Sprintf("TE-10.1: Push MPLS labels to MPLS: sh %d labels", numLabels), func(t *testing.T) {
			labels := []uint32{}
			for i := 0; i < numLabels; i++ {
				labels = append(labels, uint32(baseLabel+i))
			}

			mplsT := mplsutil.New(c, mplsutil.PushToMPLS, deviations.DefaultNetworkInstance(ondatra.DUT(t, "dut")), &mplsutil.Args{
				LabelsToPush: labels,
			})

			mplsT.ConfigureDevices(t, ondatra.DUT(t, "dut"), ondatra.ATE(t, "ate"))
			mplsT.ProgramGRIBI(t)
			mplsT.ValidateProgramming(t)
			mplsT.ConfigureFlows(t, ondatra.ATE(t, "ate"))
			mplsT.RunFlows(t, ondatra.ATE(t, "ate"), 10*time.Second, lossTolerance)
			mplsT.Cleanup(t)
		})
	}
}
