// Package gRIBI MPLS Dataplane Test implements tests of the MPLS dataplane that
// use gRIBI as the programming mechanism.
package gribi_mpls_dataplane_test

import (
	"flag"
	"fmt"
	"testing"
	"time"

	mplsc "github.com/openconfig/featureprofiles/feature/gribi/otg_tests/mpls_compliance"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ygnmi/ygnmi"
)

const (
	defNIName         = "default"
	baseLabel         = 42
	destinationLabel  = 100
	innerLabel        = 5000
	maximumStackDepth = 20
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
	otgCfg := mplsc.PushBaseConfigs(t, ondatra.DUT(t, "dut"), ondatra.ATE(t, "ate"))

	gribic := ondatra.DUT(t, "dut").RawAPIs().GRIBI().Default(t)
	c := fluent.NewClient()
	c.Connection().WithStub(gribic)

	otg := ondatra.ATE(t, "ate").OTG()

	testMPLSFlow := func(t *testing.T, _ []uint32) {
		// We configure a traffic flow from mplsc.ATESrc -> mplsc.ATEDst (passes through
		// mplsc.ATESrc -> [ mplsc.DUTSrc -- mplsc.DUTDst ] --> mplsc.ATEDst.
		//
		// Since EgressLabelStack pushes N labels but has a label forwarding
		// entry of 100 that points at that next-hop, we only need this value
		// to check whether traffic is forwarded.
		//
		// TODO(robjs): in the future, extend this test to check that the
		// received label stack is as we expected.

		// wait for ARP to resolve.
		t.Logf("looking on interface %s_ETH for %s", mplsc.ATESrc.Name, mplsc.DUTSrc.IPv4)
		var dstMAC string
		gnmi.WatchAll(t, otg, gnmi.OTG().Interface(mplsc.ATESrc.Name+"_ETH").Ipv4NeighborAny().LinkLayerAddress().State(), time.Minute, func(val *ygnmi.Value[string]) bool {
			dstMAC, _ = val.Val()
			return val.IsPresent()
		}).Await(t)
		t.Logf("MAC discovered was %s", dstMAC)

		// TODO(robjs): MPLS is currently not supported in OTG.
		otgCfg.Flows().Clear().Items()
		mplsFlow := otgCfg.Flows().Add().SetName("MPLS_FLOW")
		mplsFlow.Metrics().SetEnable(true)
		mplsFlow.TxRx().Port().SetTxName(mplsc.ATESrc.Name).SetRxName(mplsc.ATEDst.Name)

		mplsFlow.Rate().SetChoice("pps").SetPps(1)

		// Set up ethernet layer.
		eth := mplsFlow.Packet().Add().Ethernet()
		eth.Src().SetChoice("value").SetValue(mplsc.ATESrc.MAC)
		eth.Dst().SetChoice("value").SetValue(dstMAC)

		// Set up MPLS layer with destination label 100.
		mpls := mplsFlow.Packet().Add().Mpls()
		mpls.Label().SetChoice("value").SetValue(destinationLabel)
		mpls.BottomOfStack().SetChoice("value").SetValue(0)

		mplsInner := mplsFlow.Packet().Add().Mpls()
		mplsInner.Label().SetChoice("value").SetValue(innerLabel)
		mplsInner.BottomOfStack().SetChoice("value").SetValue(1)

		ip4 := mplsFlow.Packet().Add().Ipv4()
		ip4.Src().SetChoice("value").SetValue("100.64.1.1")
		ip4.Dst().SetChoice("value").SetValue("100.64.2.1")
		ip4.Version().SetChoice("value").SetValue(4)

		otg.PushConfig(t, otgCfg)

		t.Logf("Starting MPLS traffic...")
		otg.StartTraffic(t)
		t.Logf("Sleeping for %s...", *sleepTime)
		time.Sleep(*sleepTime)
		t.Logf("Stopping MPLS traffic...")
		otg.StopTraffic(t)

		// TODO(robjs): validate traffic counters and received headers.
		metrics := gnmi.Get(t, otg, gnmi.OTG().Flow("MPLS_FLOW").State())
		got, want := metrics.GetCounters().GetInPkts(), metrics.GetCounters().GetOutPkts()
		lossPackets := want - got
		if lossPackets != 0 {
			t.Fatalf("did not get expected number of packets, got: %d, want: %d", got, want)
		}
	}

	baseLabel := 42
	for i := 1; i <= maximumStackDepth; i++ {
		t.Run(fmt.Sprintf("push %d labels", i), func(t *testing.T) {
			t.Logf("running MPLS compliance test with %d labels.", i)
			mplsc.EgressLabelStack(t, c, defNIName, baseLabel, i, testMPLSFlow)
		})
	}
}

// TestMPLSPushToIP - inject IP flow from OTG, packet capture to validate that MPLS label stack was applied
//		      as expected.
// TestPopTopLabel - inject MPLS flow with multiple labels and validate that top-most label was removed.
//		   - inject MPLS flow with 1 label, and validate IP packet was exposed.
// TestPopNLabels  - inject MPLS flow with multiple labels and validate that the popped labels were removed.
// TestPopOnePushN - inject MPLS flow with multiple labels, and validate that one label was popped and N were pushed.
//		   - inject MPLS flow with 1 label, and validate that one label was popped and N were pushed.

func TestMPLSPushToIP(t *testing.T) {
	otgCfg := pushBaseConfigs(t, ondatra.DUT(t, "dut"), ondatra.ATE(t, "ate"))
	gribic := ondatra.DUT(t, "dut").RawAPIs().GRIBI().Default(t)
	c := fluent.NewClient()
	c.Connection().WithStub(gribic)

	testIPFlow := func(t *testing.T, _ []uint32) {
		// We configure a traffic flow from mplsc.ATESrc -> mplsc.ATEDst (passes through
		// mplsc.ATESrc -> [ mplsc.DUTSrc -- mplsc.DUTDst ] --> mplsc.ATEDst.
		//
		// Since EgressLabelStack pushes N labels but has a label forwarding
		// entry of 100 that points at that next-hop, we only need this value
		// to check whether traffic is forwarded.
		//
		// TODO(robjs): in the future, extend this test to check that the
		// received label stack is as we expected.

		// wait for ARP to resolve.
		otg := ondatra.ATE(t, "ate").OTG()
		var dstMAC string
		gnmi.WatchAll(t, otg, gnmi.OTG().Interface(mplsc.ATESrc.Name+"_ETH").Ipv4NeighborAny().LinkLayerAddress().State(), time.Minute, func(val *ygnmi.Value[string]) bool {
			dstMAC, _ = val.Val()
			return val.IsPresent()
		}).Await(t)

		// Remove any stale flows.
		otgCfg.Flows().Clear().Items()
		ipFlow := otgCfg.Flows().Add().SetName("MPLS_FLOW")
		ipFlow.Metrics().SetEnable(true)
		ipFlow.TxRx().Port().SetTxName(mplsc.ATESrc.Name).SetRxName(mplsc.ATEDst.Name)

		// Set up ethernet layer.
		eth := ipFlow.Packet().Add().Ethernet()
		eth.Src().SetValue(mplsc.ATESrc.MAC)
		eth.Dst().SetChoice("value").SetValue(dstMAC)

		ip4 := ipFlow.Packet().Add().Ipv4()
		ip4.Src().SetValue(mplsc.ATESrc.IPv4)
		ip4.Dst().SetValue("10.0.0.1") // this must be in 10/8.

		otg.PushConfig(t, otgCfg)

		t.Logf("Starting IP traffic...")
		otg.StartTraffic(t)
		time.Sleep(120 * time.Second)
		t.Logf("Stopping IP traffic...")
		otg.StopTraffic(t)

		// TODO(robjs): validate traffic counters and received headers.
	}

	baseLabel := 42
	numLabels := 20
	for i := 1; i <= numLabels; i++ {
		t.Run(fmt.Sprintf("push %d labels to IP", i), func(t *testing.T) {
			mplscompliance.PushToIPPacket(t, c, defNIName, baseLabel, i, testIPFlow)
		})
	}
}

func TestPopTopLabelMPLSInner(t *testing.T) {
	otgCfg := pushBaseConfigs(t, ondatra.DUT(t, "dut"), ondatra.ATE(t, "ate"))
	gribic := ondatra.DUT(t, "dut").RawAPIs().GRIBI().Default(t)
	c := fluent.NewClient()
	c.Connection().WithStub(gribic)
	_ = otgCfg

	// TODO(robjs): define trafficFunc
	mplscompliance.PopTopLabel(t, c, defNIName, nil)
}

func TestPopTopLabelIPInner(t *testing.T) {
	otgCfg := pushBaseConfigs(t, ondatra.DUT(t, "dut"), ondatra.ATE(t, "ate"))
	gribic := ondatra.DUT(t, "dut").RawAPIs().GRIBI().Default(t)
	c := fluent.NewClient()
	c.Connection().WithStub(gribic)
	_ = otgCfg

	// TODO(robjs): define trafficFunc
	mplscompliance.PopTopLabel(t, c, defNIName, nil)
}

func TestPopNLabels(t *testing.T) {
	otgCfg := pushBaseConfigs(t, ondatra.DUT(t, "dut"), ondatra.ATE(t, "ate"))
	gribic := ondatra.DUT(t, "dut").RawAPIs().GRIBI().Default(t)
	c := fluent.NewClient()
	c.Connection().WithStub(gribic)
	_ = otgCfg

	for _, stack := range [][]uint32{{100}, {100, 42}, {100, 42, 43, 44, 45}} {
		// TODO(robjs): define trafficFunc

		t.Run(fmt.Sprintf("pop N labels, stack %v", stack), func(t *testing.T) {
			mplscompliance.PopNLabels(t, c, defNIName, stack, nil)
		})
	}
}

func TestPopOnePushN(t *testing.T) {
	otgCfg := pushBaseConfigs(t, ondatra.DUT(t, "dut"), ondatra.ATE(t, "ate"))
	gribic := ondatra.DUT(t, "dut").RawAPIs().GRIBI().Default(t)
	c := fluent.NewClient()
	c.Connection().WithStub(gribic)
	_ = otgCfg

	stacks := [][]uint32{
		{100}, // swap for label 100, pop+push for label 200
		{100, 200, 300, 400},
		{100, 200, 300, 400, 500, 600},
	}
	for _, stack := range stacks {
		// TODO(robjs): define trafficFunc
		t.Run(fmt.Sprintf("pop one, push N, stack: %v", stack), func(t *testing.T) {
			mplscompliance.PopOnePushN(t, c, defNIName, stack, nil)
		})
	}
}
