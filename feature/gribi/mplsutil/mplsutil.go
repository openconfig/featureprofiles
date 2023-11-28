// Package mplsutil implements a set of helper utility to run common gRIBI
// MPLS test scenarios against an ATE and DUT.
package mplsutil

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/gribigo/chk"
	"github.com/openconfig/gribigo/client"
	"github.com/openconfig/gribigo/compliance"
	"github.com/openconfig/gribigo/constants"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
)

// Mode is an enumerated value fting the type of tests supported by the
// gRIBI MPLS util.
type Mode int64

const (
	_ Mode = iota
	// PushToMPLS defines a test that programs a DUT via gRIBI with a
	// label forwarding entry within defaultNIName, with a label stack with
	// numLabels in it, starting at baseLabel, if trafficFunc is non-nil it is run
	// to validate the dataplane.
	//
	// The DUT is expected to have a next-hop of 192.0.2.2 that is resolvable.
	PushToMPLS
	// PushToIP programs a gRIBI entry for an ingress LER function whereby MPLS labels are
	// pushed to an IP packet. The entries are programmed into defaultNIName, with the stack
	// imposed being a stack of numLabels labels, starting with baseLabel. After the programming
	// has been verified trafficFunc is run to allow validation of the dataplane.
	//
	// The DUT is expected to be within a topology where 192.0.2.2 is a valid next-hop.
	PushToIP
	// PopTopLabel creates a test whereby the top label of an input packet is popped.
	// The DUT is expected to be in a topology where 192.0.2.2 is a valid next-hop. Packets
	// with label 100 will have this label popped from the stack.
	PopTopLabel
	// PopNLabels programs a gRIBI server with a LFIB entry matching label 100
	// that pops the labels specified in popLabels from the stack. If trafficFunc
	// is non-nil it is called after the gRIBI programming is verified.
	//
	// The DUT is expected to be in a topology where 192.0.2.2 resolves to
	// a valid next-hop.
	PopNLabels
	// PopOnePushN implements a test whereby one (the top) label is popped, and N labels as specified
	// by pushLabels are pushed to the stack for an input MPLS packet. Two LFIB entries (100 and 200)
	// are created. If trafficFunc is non-nil it is called after the gRIBI programming has been validated.
	//
	// The DUT is expected to be in a topology where 192.0.2.2 is a resolvable next-hop.
	PopOnePushN
)

// GRIBIMPLSTest is a wrapper around a specific gRIBI MPLS test scenario,
// the test has a specific mode that is used to control its underlying
// functionality. The modes are enumerated by the mplsutil.Mode type
// described in detail above.
type GRIBIMPLSTest struct {
	// mode stores the mode that the test was initialised in.
	mode Mode

	// defaultNIName stores the name of the default network instance for the
	// test.
	defaultNIName string

	// client stores the gRIBI client that should be used to contact the
	// DUT.
	client *fluent.GRIBIClient

	// config is a cache of the OTG configuration that is to be used for
	// the test.
	otgConfig gosnappi.Config

	// args stores the arguments to the test.
	args *Args

	// result caches the last set of results that the test ran.
	result []*client.OpResult
}

// New returns a new GRIBIMPLSTest initialised with the specified client, mode
// and default network instance name (defName). The supplied args are used to
// determine the behaviour of specific tests that require additional configuration.
func New(c *fluent.GRIBIClient, m Mode, defName string, args *Args) *GRIBIMPLSTest {
	return &GRIBIMPLSTest{
		client:        c,
		mode:          m,
		defaultNIName: defName,
		args:          args,
	}
}

// Args specifies a set of arguments that can be handed to specific gRIBI
// tests to control the specific payload that they create.
type Args struct {
	// LabelsToPop specifies the set of labels that should be popped from
	// an incoming MPLS packet.
	LabelsToPop []uint32
	// LabelsToPush specifies the set of labels that should be pushed to
	// an incoming MPLS packet.
	LabelsToPush []uint32
}

// ConfigureDevices configures the DUT and ATE with their base configurations
// such the test topology is set up. This uses a constant topology as described
// in topo.go.
func (g *GRIBIMPLSTest) ConfigureDevices(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice) {
	DUTSrc.Name = dut.Port(t, "port1").Name()
	DUTDst.Name = dut.Port(t, "port2").Name()

	otgCfg, err := configureATEInterfaces(t, ate, ATESrc, DUTSrc, ATEDst, DUTDst)
	if err != nil {
		t.Fatalf("cannot configure ATE interfaces via OTG, %v", err)
	}

	ate.OTG().PushConfig(t, otgCfg)

	d := &oc.Root{}
	// configure ports on the DUT. note that each port maps to two interfaces
	// because we create a LAG.
	for index, i := range []*attrs.Attributes{DUTSrc, DUTDst} {
		cfgs, err := dutIntf(dut, i, index)
		if err != nil {
			t.Fatalf("cannot generate configuration for interface %s, err: %v", i.Name, err)
		}
		for _, intf := range cfgs {
			d.AppendInterface(intf)
		}
	}
	fptest.LogQuery(t, "", gnmi.OC().Config(), d)
	gnmi.Update(t, dut, gnmi.OC().Config(), d)

	// Sleep for 1 second to ensure that OTG has absorbed configuration.
	time.Sleep(1 * time.Second)
	ate.OTG().StartProtocols(t)

	g.otgConfig = otgCfg
}

// ProgramGRIBI performs the programming operations specified by the mode of the test.
func (g *GRIBIMPLSTest) ProgramGRIBI(t *testing.T) {
	switch g.mode {
	case PushToMPLS:
		if len(g.args.LabelsToPush) == 0 {
			t.Fatalf("invalid number of labels to push, got: %d, want: >0", len(g.args.LabelsToPush))
		}

		g.result = g.modify(t, []fluent.GRIBIEntry{
			fluent.NextHopEntry().
				WithNetworkInstance(g.defaultNIName).
				WithIndex(1).
				WithIPAddress(ATEDst.IPv4).
				WithPushedLabelStack(g.args.LabelsToPush...),
			fluent.NextHopGroupEntry().
				WithNetworkInstance(g.defaultNIName).
				WithID(1).
				AddNextHop(1, 1),
			fluent.LabelEntry().
				WithLabel(100).
				WithNetworkInstance(g.defaultNIName).
				WithNextHopGroup(1),
		})
	case PushToIP:
		if len(g.args.LabelsToPush) == 0 {
			t.Fatalf("invalid number of labels to push, got: %d, want: >0", len(g.args.LabelsToPush))
		}
		g.result = g.modify(t, []fluent.GRIBIEntry{
			fluent.NextHopEntry().
				WithNetworkInstance(g.defaultNIName).
				WithIndex(1).
				WithPushedLabelStack(g.args.LabelsToPush...).
				WithIPAddress(ATEDst.IPv4),
			fluent.NextHopGroupEntry().
				WithNetworkInstance(g.defaultNIName).
				WithID(1).
				AddNextHop(1, 1),
			fluent.IPv4Entry().
				WithPrefix(dutRoutedIPv4Prefix).
				WithNetworkInstance(g.defaultNIName).
				WithNextHopGroupNetworkInstance(g.defaultNIName).
				WithNextHopGroup(1),
		})
	case PopTopLabel:
		g.result = g.modify(t, []fluent.GRIBIEntry{
			fluent.NextHopEntry().
				WithNetworkInstance(g.defaultNIName).
				WithIndex(1).
				WithIPAddress(ATEDst.IPv4).
				WithPopTopLabel(),
			fluent.NextHopGroupEntry().
				WithNetworkInstance(g.defaultNIName).
				WithID(1).
				AddNextHop(1, 1),
			fluent.LabelEntry().
				WithLabel(staticMPLSToATE).
				WithNetworkInstance(g.defaultNIName).
				WithNextHopGroupNetworkInstance(g.defaultNIName).
				WithNextHopGroup(1),
		})
	case PopNLabels:
		if len(g.args.LabelsToPop) == 0 {
			t.Fatalf("invalid number of labels to pop, got: %d, want: >0", len(g.args.LabelsToPop))
		}
		g.result = g.modify(t, []fluent.GRIBIEntry{
			fluent.NextHopEntry().
				WithNetworkInstance(g.defaultNIName).
				WithIndex(1).
				WithIPAddress(ATEDst.IPv4),
			fluent.NextHopGroupEntry().
				WithNetworkInstance(g.defaultNIName).
				WithID(1).
				AddNextHop(1, 1),
			fluent.LabelEntry().
				WithLabel(100).
				WithPoppedLabelStack(g.args.LabelsToPop...).
				WithNetworkInstance(g.defaultNIName).
				WithNextHopGroup(1),
		})
	case PopOnePushN:
		if len(g.args.LabelsToPush) == 0 {
			t.Fatalf("invalid number of labels to push, got: %d, want: >0", len(g.args.LabelsToPush))
		}
		g.result = g.modify(t, []fluent.GRIBIEntry{
			fluent.NextHopEntry().
				WithNetworkInstance(g.defaultNIName).
				WithIndex(1).
				WithIPAddress("192.0.2.2").
				WithPopTopLabel().
				WithPushedLabelStack(g.args.LabelsToPush...),
			fluent.NextHopGroupEntry().
				WithNetworkInstance(g.defaultNIName).
				WithID(1).
				AddNextHop(1, 1),
			fluent.LabelEntry().
				WithLabel(100).
				WithNetworkInstance(g.defaultNIName).
				WithNextHopGroup(1),

			fluent.LabelEntry().
				WithLabel(200).
				WithNetworkInstance(g.defaultNIName).
				WithNextHopGroup(1),
		})
	default:
		t.Fatalf("invalid test mode specified")
	}
}

// ValidateProgramming validates whether the programming for the specific
// scenario was accepted at the DUT.
func (g *GRIBIMPLSTest) ValidateProgramming(t *testing.T) {
	t.Helper()
	switch g.mode {
	case PushToMPLS:
		chk.HasResult(t, g.result,
			fluent.OperationResult().
				WithMPLSOperation(100).
				WithProgrammingResult(fluent.InstalledInRIB).
				WithOperationType(constants.Add).
				AsResult(),
			chk.IgnoreOperationID(),
		)

		chk.HasResult(t, g.result,
			fluent.OperationResult().
				WithNextHopGroupOperation(1).
				WithProgrammingResult(fluent.InstalledInRIB).
				WithOperationType(constants.Add).
				AsResult(),
			chk.IgnoreOperationID(),
		)

		chk.HasResult(t, g.result,
			fluent.OperationResult().
				WithNextHopOperation(1).
				WithProgrammingResult(fluent.InstalledInRIB).
				WithOperationType(constants.Add).
				AsResult(),
			chk.IgnoreOperationID(),
		)
	case PushToIP:
		chk.HasResult(t, g.result,
			fluent.OperationResult().
				WithIPv4Operation(dutRoutedIPv4Prefix).
				WithProgrammingResult(fluent.InstalledInRIB).
				WithOperationType(constants.Add).
				AsResult(),
			chk.IgnoreOperationID())

		chk.HasResult(t, g.result,
			fluent.OperationResult().
				WithNextHopOperation(1).
				WithProgrammingResult(fluent.InstalledInRIB).
				WithOperationType(constants.Add).
				AsResult(),
			chk.IgnoreOperationID())

		chk.HasResult(t, g.result,
			fluent.OperationResult().
				WithNextHopGroupOperation(1).
				WithProgrammingResult(fluent.InstalledInRIB).
				WithOperationType(constants.Add).
				AsResult(),
			chk.IgnoreOperationID())
	case PopTopLabel:
		chk.HasResult(t, g.result,
			fluent.OperationResult().
				WithMPLSOperation(100).
				WithProgrammingResult(fluent.InstalledInRIB).
				WithOperationType(constants.Add).
				AsResult(),
			chk.IgnoreOperationID())

		chk.HasResult(t, g.result,
			fluent.OperationResult().
				WithNextHopGroupOperation(1).
				WithProgrammingResult(fluent.InstalledInRIB).
				WithOperationType(constants.Add).
				AsResult(),
			chk.IgnoreOperationID())

		chk.HasResult(t, g.result,
			fluent.OperationResult().
				WithNextHopOperation(1).
				WithProgrammingResult(fluent.InstalledInRIB).
				WithOperationType(constants.Add).
				AsResult(),
			chk.IgnoreOperationID())
	case PopOnePushN:
		chk.HasResult(t, g.result,
			fluent.OperationResult().
				WithNextHopGroupOperation(1).
				WithProgrammingResult(fluent.InstalledInRIB).
				WithOperationType(constants.Add).
				AsResult(),
			chk.IgnoreOperationID())

		chk.HasResult(t, g.result,
			fluent.OperationResult().
				WithNextHopOperation(1).
				WithProgrammingResult(fluent.InstalledInRIB).
				WithOperationType(constants.Add).
				AsResult(),
			chk.IgnoreOperationID())

		for _, label := range []uint64{100, 200} {
			chk.HasResult(t, g.result,
				fluent.OperationResult().
					WithMPLSOperation(label).
					WithProgrammingResult(fluent.InstalledInRIB).
					WithOperationType(constants.Add).
					AsResult(),
				chk.IgnoreOperationID())
		}
	}
}

const (
	// flowName is the name of the flow created within the ATE configuration.
	flowName = "TEST_MPLS"
	// innerLabel is an MPLS label used to ensure that payloads are MPLS
	// when labels are added or removed.
	innerLabel = 42
)

// ConfigureFlows sets up the flows that are required for the specified test
// type on the ATE device provided.
func (g *GRIBIMPLSTest) ConfigureFlows(t *testing.T, ate *ondatra.ATEDevice) {
	t.Helper()
	switch g.mode {
	case PushToMPLS:
		t.Logf("looking on interface %s_ETH for %s", ATESrc.Name, DUTSrc.IPv4)
		var dstMAC string
		gnmi.WatchAll(t, ate, gnmi.OTG().Interface(ATESrc.Name+"_ETH").Ipv4NeighborAny().LinkLayerAddress().State(), time.Minute, func(val *ygnmi.Value[string]) bool {
			dstMAC, _ = val.Val()
			return val.IsPresent()
		}).Await(t)
		t.Logf("MAC discovered was %s", dstMAC)

		g.otgConfig.Flows().Clear().Items()
		mplsFlow := g.otgConfig.Flows().Add().SetName(flowName)
		mplsFlow.Metrics().SetEnable(true)
		mplsFlow.TxRx().Port().SetTxName(ATESrc.Name).SetRxName(ATEDst.Name)

		mplsFlow.Rate().SetChoice("pps").SetPps(1)

		// Set up ethernet layer.
		eth := mplsFlow.Packet().Add().Ethernet()
		eth.Src().SetChoice("value").SetValue(ATESrc.MAC)
		eth.Dst().SetChoice("value").SetValue(dstMAC)

		// Set up MPLS layer with destination label 100.
		mpls := mplsFlow.Packet().Add().Mpls()
		mpls.Label().SetChoice("value").SetValue(staticMPLSToATE)
		mpls.BottomOfStack().SetChoice("value").SetValue(0)

		mplsInner := mplsFlow.Packet().Add().Mpls()
		mplsInner.Label().SetChoice("value").SetValue(innerLabel)
		mplsInner.BottomOfStack().SetChoice("value").SetValue(1)

		ip4 := mplsFlow.Packet().Add().Ipv4()
		ip4.Src().SetChoice("value").SetValue("198.18.1.1")
		ip4.Dst().SetChoice("value").SetValue("198.18.2.1")
		ip4.Version().SetChoice("value").SetValue(4)

		ate.OTG().PushConfig(t, g.otgConfig)
	default:
		t.Fatalf("unspecified flow for test type %v", g.mode)
	}
}

// RunFlows validates that traffic is forwarded by the DUT by running the
// flows required by the specified test.
func (g *GRIBIMPLSTest) RunFlows(t *testing.T, ate *ondatra.ATEDevice, runtime time.Duration, tolerableLostPackets uint64) {
	t.Helper()
	switch g.mode {
	case PushToMPLS:
		t.Logf("Starting MPLS traffic...")
		ate.OTG().StartTraffic(t)
		t.Logf("Sleeping for %s...", runtime)
		time.Sleep(runtime)
		t.Logf("Stopping MPLS traffic...")
		ate.OTG().StopTraffic(t)

		metrics := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(flowName).State())
		got, want := metrics.GetCounters().GetInPkts(), metrics.GetCounters().GetOutPkts()
		lossPackets := want - got
		if lossPackets > tolerableLostPackets {
			t.Fatalf("did not get expected number of packets, got: %d, want: >= %d", got, (want - tolerableLostPackets))
		}
	default:
		t.Fatalf("traffic validation invalid for test type %v", g.mode)
	}
}

// Cleanup cleans up the underlying gRIBI server to remove any entries.
func (g *GRIBIMPLSTest) Cleanup(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	g.flushServer(ctx, t)
}

// modify performs a set of operations (in ops) on the supplied gRIBI client,
// reporting errors via t.
func (g *GRIBIMPLSTest) modify(t *testing.T, ops []fluent.GRIBIEntry) []*client.OpResult {
	g.client.Connection().
		WithRedundancyMode(fluent.ElectedPrimaryClient).
		WithInitialElectionID(electionID.Load(), 0)

	opFn := []func(){
		func() {
			g.client.Modify().AddEntry(t, ops...)
		},
	}

	return compliance.DoModifyOps(g.client, t, opFn, fluent.InstalledInRIB, false)
}

var (
	// electionID is the global election ID used between test cases.
	electionID = func() *atomic.Uint64 {
		eid := new(atomic.Uint64)
		eid.Store(1)
		return eid
	}()
)

// flushServer removes all entries from the server and can be called between
// test cases in order to remove the server's RIB contents.
func (g *GRIBIMPLSTest) flushServer(ctx context.Context, t *testing.T) {
	g.client.Start(ctx, t)
	defer g.client.Stop(t)

	if _, err := g.client.Flush().
		WithElectionOverride().
		WithAllNetworkInstances().
		Send(); err != nil {
		t.Fatalf("Could not remove all entries from server, got: %v", err)
	}
}
