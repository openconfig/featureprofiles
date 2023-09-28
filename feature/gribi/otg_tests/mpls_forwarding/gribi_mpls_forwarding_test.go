// Package gRIBI MPLS forwarding implements tests of the MPLS dataplane that
// use gRIBI as the programming mechanism.
package gribi_mpls_forwarding_test

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
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ygnmi/ygnmi"
)

const (
	// baseLabel indicates the minimum label to use on a packet.
	baseLabel = 42
	// destinationLabel is a label that is programmed as a forwarding entry.
	destinationLabel = 100
	// innerLabel is a label used within the label stack (to ensure that the egress device's
	// payload is MPLS).
	innerLabel = 5000
	// maximumStackDepth is the maximum number of labels to be pushed onto the packet.
	maximumStackDepth = 20
	// lossTolerance is the number of packets that can be lost within a flow before the
	// test fails.
	lossTolerance = 1

	// flowName is a string used to uniquely identify the flow used within a test, where
	// only one flow is required.
	flowName = "MPLS_FLOW"
)

var (
	// sleepTime allows a user to specify that the test should sleep after setting
	// up all elements (configuration, gRIBI forwarding entries, traffic flows etc.).
	sleepTime = flag.Duration("sleep", 10*time.Second, "duration for which the test should sleep after setup")
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// TestMPLSLabelPushDepth validates the gRIBI actions that are used to push N labels onto
// as part of routing towards a next-hop. Note that this test does not validate against the
// dataplane, but solely the gRIBI control-plane support.
func TestMPLSLabelPushDepth(t *testing.T) {
	otgCfg := mplsutil.PushBaseConfigs(t, ondatra.DUT(t, "dut"), ondatra.ATE(t, "ate"))

	gribic := ondatra.DUT(t, "dut").RawAPIs().GRIBI(t)
	c := fluent.NewClient()
	c.Connection().WithStub(gribic)

	otg := ondatra.ATE(t, "ate").OTG()

	testMPLSFlow := func(t *testing.T, _ []uint32) {
		// We configure a traffic flow from mplsutil.ATESrc -> mplsutil.ATEDst (passes through
		// mplsutil.ATESrc -> [ mplsutil.DUTSrc -- mplsutil.DUTDst ] --> mplsutil.ATEDst.
		//
		// Since EgressLabelStack pushes N labels but has a label forwarding
		// entry of 100 that points at that next-hop, we only need this value
		// to check whether traffic is forwarded.
		//
		// TODO(robjs): in the future, extend this test to check that the
		// received label stack is as we expected.

		// wait for ARP to resolve.
		t.Logf("looking on interface %s_ETH for %s", mplsutil.ATESrc.Name, mplsutil.DUTSrc.IPv4)
		var dstMAC string
		gnmi.WatchAll(t, otg, gnmi.OTG().Interface(mplsutil.ATESrc.Name+"_ETH").Ipv4NeighborAny().LinkLayerAddress().State(), time.Minute, func(val *ygnmi.Value[string]) bool {
			dstMAC, _ = val.Val()
			return val.IsPresent()
		}).Await(t)
		t.Logf("MAC discovered was %s", dstMAC)

		otgCfg.Flows().Clear().Items()
		mplsFlow := otgCfg.Flows().Add().SetName(flowName)
		mplsFlow.Metrics().SetEnable(true)
		mplsFlow.TxRx().Port().SetTxName(mplsutil.ATESrc.Name).SetRxName(mplsutil.ATEDst.Name)

		mplsFlow.Rate().SetChoice("pps").SetPps(1)

		// Set up ethernet layer.
		eth := mplsFlow.Packet().Add().Ethernet()
		eth.Src().SetChoice("value").SetValue(mplsutil.ATESrc.MAC)
		eth.Dst().SetChoice("value").SetValue(dstMAC)

		// Set up MPLS layer with destination label 100.
		mpls := mplsFlow.Packet().Add().Mpls()
		mpls.Label().SetChoice("value").SetValue(destinationLabel)
		mpls.BottomOfStack().SetChoice("value").SetValue(0)

		mplsInner := mplsFlow.Packet().Add().Mpls()
		mplsInner.Label().SetChoice("value").SetValue(innerLabel)
		mplsInner.BottomOfStack().SetChoice("value").SetValue(1)

		ip4 := mplsFlow.Packet().Add().Ipv4()
		ip4.Src().SetChoice("value").SetValue("198.18.1.1")
		ip4.Dst().SetChoice("value").SetValue("198.18.2.1")
		ip4.Version().SetChoice("value").SetValue(4)

		otg.PushConfig(t, otgCfg)

		t.Logf("Starting MPLS traffic...")
		otg.StartTraffic(t)
		t.Logf("Sleeping for %s...", *sleepTime)
		time.Sleep(*sleepTime)
		t.Logf("Stopping MPLS traffic...")
		otg.StopTraffic(t)

		metrics := gnmi.Get(t, otg, gnmi.OTG().Flow(flowName).State())
		got, want := metrics.GetCounters().GetInPkts(), metrics.GetCounters().GetOutPkts()
		lossPackets := want - got
		if lossPackets > lossTolerance {
			t.Fatalf("did not get expected number of packets, got: %d, want: >= %d", got, (want - lossTolerance))
		}
	}

	for i := 1; i <= maximumStackDepth; i++ {
		t.Run(fmt.Sprintf("push %d labels", i), func(t *testing.T) {
			t.Logf("running MPLS compliance test with %d labels.", i)
			mplsutil.PushLabelStack(t, c, deviations.DefaultNetworkInstance(ondatra.DUT(t, "dut")), baseLabel, i, testMPLSFlow)
		})
	}
}
